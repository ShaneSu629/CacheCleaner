package scan

import (
	"os"
	"path/filepath"
	"strings"

	"cachecleaner/internal/config"
	"cachecleaner/internal/db"
	"cachecleaner/internal/model"
)

// 本文件实现"社交应用缓存智能发现"。
//
// 背景（本机取证 + 公开资料核对）：
// 微信/QQ 的数据大头根本不在 AppData，而在用户可自选的"文件存储目录"：
//   - 微信 3.x：<文档>\WeChat Files\<wxid>\FileStorage\{Cache,Thumb,...}
//   - 微信 4.x：<盘符根>:\xwechat_files\<wxid>\...（默认已迁出文档目录，可能在任意盘）
//   - QQ NT / TIM：<文档>\Tencent Files\<uin>\nt_qq\nt_data\{log,Pic,Video,Emoji,...}
// 这些位置固定的扫描根（AppData 体系）完全覆盖不到，因此这里单独做容器级发现：
// 先定位容器根（文档目录 + 所有盘符根目录探测），再按每种应用的已知子目录规则取
// "纯缓存子目录"作为缓存项——绝不输出账号整目录（含聊天记录数据库/登录态）。

// socialRule 是某个账号目录下的一条缓存子目录规则。
type socialRule struct {
	rel  string
	risk model.Risk
	desc string
}

// 微信 3.x：FileStorage 下只有 Cache/Thumb/Temp 是纯缓存；
// Image/Video/File/MsgAttach 是用户收发的文件（删了聊天记录里就打不开），坚决不碰。
var wechat3Rules = []socialRule{
	{"FileStorage/Cache", model.RiskSafe, "微信 3.x 缓存(朋友圈/图片临时数据,可重建)"},
	{"FileStorage/Thumb", model.RiskSafe, "微信 3.x 缩略图缓存(自动重建)"},
	{"Temp", model.RiskSafe, "微信 3.x 临时文件"},
}

// 微信 4.x（xwechat_files）：只取明确的临时/缓存目录；
// msg/{video,file} 是聊天收发的视频与文件（用户数据），config/db/backup 是配置与备份，都不碰。
// 注：Windows 文件系统大小写不敏感，"temp" 一条即可命中 temp/Temp/TEMP；
// Linux 下只命中小写，属于可接受的保守行为。
var wechat4Rules = []socialRule{
	{"cache", model.RiskCaution, "微信 4.x 缓存目录"},
	{"temp", model.RiskSafe, "微信 4.x 临时文件"},
}

// QQ NT（QQ9）：nt_data 下 log 是运行日志；Pic/Video/Emoji 是聊天图片/视频/表情缓存，
// 正常消息可从服务器重新下载（被撤回/删除的消息除外），标谨慎。
// Ptt(语音)/File_Recv(接收文件) 是用户数据，不碰；global 是登录列表，不碰。
var qqNtRules = []socialRule{
	{"nt_qq/nt_data/log", model.RiskSafe, "QQ 运行日志"},
	{"nt_qq/nt_data/Pic", model.RiskCaution, "QQ 聊天图片缓存(未撤回消息可重新下载)"},
	{"nt_qq/nt_data/Video", model.RiskCaution, "QQ 聊天视频缓存(未撤回消息可重新下载)"},
	{"nt_qq/nt_data/Emoji", model.RiskCaution, "QQ 表情缓存(按需重新下载)"},
	{"AppWebCache", model.RiskCaution, "旧版 QQ/TIM 网页缓存"},
}

// socialContainer 描述一种社交应用数据容器：容器名 -> 应用名与账号级规则。
type socialContainer struct {
	dirName  string // 容器目录名（如 "xwechat_files"）
	appName  string // 展示用应用名（如 "微信"）
	rules    []socialRule
	driveTop bool // 是否探测盘符根目录（微信 4.x 默认在盘符根）
}

var socialContainers = []socialContainer{
	{"WeChat Files", "微信", wechat3Rules, true},
	{"xwechat_files", "微信", wechat4Rules, true},
	{"Tencent Files", "QQ", qqNtRules, true},
}

// socialRoots 返回真实存在的社交容器根目录列表：
// 文档目录下的容器 + 各盘符根目录下的容器（覆盖用户把存储位置改到 D 盘等情况）。
func socialRoots(cfg *config.Config) []socialContainer {
	type found struct {
		c   socialContainer
		dir string
	}
	var out []socialContainer
	seen := map[string]bool{}
	add := func(c socialContainer, dir string) {
		key := strings.ToLower(dir)
		if seen[key] {
			return
		}
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			seen[key] = true
			c.dirName = dir // 用真实路径替换容器名
			out = append(out, c)
		}
	}
	for _, c := range socialContainers {
		// 文档目录（含 OneDrive 重定向，由 config 解析）
		if cfg.Documents != "" {
			add(c, filepath.Join(cfg.Documents, c.dirName))
		}
		// 盘符根目录探测（Windows 下逐个字母 Stat，代价可忽略；非 Windows 不会命中）
		if c.driveTop {
			for ch := 'C'; ch <= 'Z'; ch++ {
				add(c, string(ch)+":\\"+c.dirName)
			}
		}
	}
	return out
}

// FindSocialCaches 发现微信/QQ 等社交应用的账号级缓存子目录。
// 只输出规则表里的纯缓存子目录；账号整目录（含聊天记录/登录态）永远不会成为缓存项。
func FindSocialCaches(cfg *config.Config) []model.CacheEntry {
	var out []model.CacheEntry
	// 按路径小写去重：Windows 文件系统大小写不敏感，规则里不同写法的条目
	// 可能命中同一个真实目录，不区分大小写地去重避免体积被重复计数。
	seen := map[string]bool{}
	for _, c := range socialRoots(cfg) {
		accounts, err := os.ReadDir(c.dirName)
		if err != nil {
			continue
		}
		for _, acc := range accounts {
			if !acc.IsDir() {
				continue
			}
			accDir := filepath.Join(c.dirName, acc.Name())
			for _, r := range c.rules {
				full := filepath.Join(accDir, filepath.FromSlash(r.rel))
				key := strings.ToLower(full)
				if seen[key] {
					continue
				}
				if fi, err := os.Stat(full); err != nil || !fi.IsDir() {
					continue
				}
				if cfg.IsExcluded(full) {
					continue
				}
				size, fc, la := db.ScanDir(full)
				if size <= 0 {
					continue
				}
				seen[key] = true
				out = append(out, model.CacheEntry{
					Path:       full,
					ShortPath:  db.ShortOf(cfg, full),
					Category:   "社交缓存(" + c.appName + ")",
					Risk:       r.risk,
					Desc:       r.desc,
					Size:       size,
					FileCount:  fc,
					LastAccess: la,
				})
			}
		}
	}
	return out
}
