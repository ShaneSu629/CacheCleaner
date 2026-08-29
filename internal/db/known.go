package db

import (
	"os"
	"path/filepath"
	"strings"

	"cachecleaner/internal/config"
	"cachecleaner/internal/model"
)

// knownDef 是已知缓存路径的定义。
type knownDef struct {
	applyOS  string // "windows" | "darwin" | "linux" | "all"
	base     config.Base
	rel      string // 相对路径，允许 "*" 段匹配该层任意单个目录（如 profiles/*/Cache）
	category string
	risk     model.Risk
	desc     string
}

// BuildKnownCaches 根据当前系统解析已知缓存库，只返回真实存在的项。
func BuildKnownCaches(cfg *config.Config) []model.CacheEntry {
	defs := knownWindows()
	defs = append(defs, knownDarwin()...)
	defs = append(defs, knownLinux()...)
	defs = append(defs, knownCrossPlatform()...)

	var out []model.CacheEntry
	for _, d := range defs {
		if d.applyOS != "all" && d.applyOS != cfg.OS {
			continue
		}
		root := cfg.Resolve(d.base)
		if root == "" {
			continue
		}
		for _, full := range expandPath(root, d.rel) {
			if cfg.IsExcluded(full) {
				continue
			}
			size, fc, la := ScanDir(full)
			if size <= 0 {
				continue
			}
			out = append(out, model.CacheEntry{
				Path:       full,
				ShortPath:  ShortOf(cfg, full),
				Category:   d.category,
				Risk:       d.risk,
				Desc:       d.desc,
				Size:       size,
				FileCount:  fc,
				LastAccess: la,
			})
		}
	}
	return out
}

// expandPath 将含 "*" 段的相对路径展开为真实存在的目录列表。
// "*" 只匹配一个路径段（不递归、不跨层），避免误伤深层目录。
func expandPath(root, rel string) []string {
	segs := strings.Split(filepath.ToSlash(rel), "/")
	cur := []string{root}
	for _, seg := range segs {
		if seg == "" || seg == "." {
			continue
		}
		var next []string
		if seg == "*" {
			for _, base := range cur {
				entries, err := os.ReadDir(base)
				if err != nil {
					continue
				}
				for _, e := range entries {
					if e.IsDir() {
						next = append(next, filepath.Join(base, e.Name()))
					}
				}
			}
		} else {
			for _, base := range cur {
				p := filepath.Join(base, filepath.FromSlash(seg))
				if fi, err := os.Stat(p); err == nil && fi.IsDir() {
					next = append(next, p)
				}
			}
		}
		cur = next
		if len(cur) == 0 {
			return nil
		}
	}
	return cur
}

// ── Windows 专有（AppData 体系） ──
func knownWindows() []knownDef {
	return []knownDef{
		// Trae（国内版真实路径）
		{"windows", config.BaseRoaming, "Trae CN/Cache", "AI缓存", model.RiskSafe, "Trae AI 编辑器缓存"},
		{"windows", config.BaseRoaming, "Trae CN/CachedData", "AI缓存", model.RiskSafe, "Trae AI 缓存数据"},
		{"windows", config.BaseRoaming, "Trae CN/Code Cache", "AI缓存", model.RiskSafe, "Trae AI 代码缓存"},
		{"windows", config.BaseRoaming, "Trae CN/GPUCache", "AI缓存", model.RiskSafe, "Trae AI GPU 缓存"},
		{"windows", config.BaseRoaming, "Trae CN/logs", "AI缓存", model.RiskSafe, "Trae AI 日志"},
		// WorkBuddy / CodeBuddy
		{"windows", config.BaseLocal, "@genieworkbuddy-desktop-updater", "AI缓存", model.RiskSafe, "WorkBuddy 桌面更新器缓存"},
		{"windows", config.BaseLocal, "WorkBuddy", "AI缓存", model.RiskCaution, "WorkBuddy 本地运行时"},
		{"windows", config.BaseLocal, "CodeBuddyExtension/Data", "AI缓存", model.RiskCaution, "CodeBuddy 扩展数据"},
		// Claude Desktop（Windows）
		{"windows", config.BaseRoaming, "Claude/Cache", "AI缓存", model.RiskSafe, "Claude 桌面版缓存"},
		{"windows", config.BaseRoaming, "Claude/CachedData", "AI缓存", model.RiskSafe, "Claude 桌面版缓存数据"},
		{"windows", config.BaseRoaming, "Claude/Code Cache", "AI缓存", model.RiskSafe, "Claude 桌面版代码缓存"},
		{"windows", config.BaseRoaming, "Claude/GPUCache", "AI缓存", model.RiskSafe, "Claude 桌面版 GPU 缓存"},
		{"windows", config.BaseRoaming, "Claude/logs", "AI缓存", model.RiskSafe, "Claude 桌面版日志"},
		{"windows", config.BaseRoaming, "Claude/IndexedDB", "AI缓存", model.RiskReview, "Claude 桌面版索引(含登录态)"},
		// Windsurf / Codeium
		{"windows", config.BaseRoaming, "Windsurf/Cache", "AI缓存", model.RiskSafe, "Windsurf 缓存"},
		{"windows", config.BaseRoaming, "Windsurf/CachedData", "AI缓存", model.RiskSafe, "Windsurf 缓存数据"},
		{"windows", config.BaseRoaming, "Windsurf/Code Cache", "AI缓存", model.RiskSafe, "Windsurf 代码缓存"},
		{"windows", config.BaseRoaming, "Windsurf/GPUCache", "AI缓存", model.RiskSafe, "Windsurf GPU 缓存"},
		{"windows", config.BaseRoaming, "Windsurf/logs", "AI缓存", model.RiskSafe, "Windsurf 日志"},
		{"windows", config.BaseLocal, "Windsurf", "AI缓存", model.RiskCaution, "Windsurf 运行时数据"},
		{"windows", config.BaseRoaming, "Code/User/globalStorage/codeium-codeium", "AI缓存", model.RiskCaution, "Codeium 扩展数据"},
		// 通义灵码 / Qoder
		{"windows", config.BaseLocal, ".lingma", "AI缓存", model.RiskCaution, "通义灵码索引/配置"},
		// 文心快码 Comate
		{"windows", config.BaseRoaming, "com.baidu.wenxin-code", "AI缓存", model.RiskCaution, "文心快码配置"},
		{"windows", config.BaseLocal, "com.baidu.wenxin-code", "AI缓存", model.RiskSafe, "文心快码缓存"},
		// Cline / Continue（VS Code 扩展）
		{"windows", config.BaseRoaming, "Code/User/globalStorage/saoudrizwan.claude-dev", "AI缓存", model.RiskCaution, "Cline 扩展数据(含对话历史)"},
		// Cursor
		{"windows", config.BaseRoaming, "Cursor/Cache", "AI缓存", model.RiskSafe, "Cursor 缓存"},
		{"windows", config.BaseRoaming, "Cursor/CachedData", "AI缓存", model.RiskSafe, "Cursor 缓存数据"},
		{"windows", config.BaseRoaming, "Cursor/Code Cache", "AI缓存", model.RiskSafe, "Cursor 代码缓存"},
		{"windows", config.BaseRoaming, "Cursor/GPUCache", "AI缓存", model.RiskSafe, "Cursor GPU 缓存"},
		{"windows", config.BaseRoaming, "Cursor/User/workspaceStorage", "AI缓存", model.RiskCaution, "Cursor 工作区存储(重新打开项目会恢复)"},
		// VS Code（AI 扩展宿主）
		{"windows", config.BaseRoaming, "Code/Cache", "AI缓存", model.RiskSafe, "VS Code 缓存(含AI扩展)"},
		{"windows", config.BaseRoaming, "Code/CachedData", "AI缓存", model.RiskSafe, "VS Code 缓存数据(含AI扩展)"},
		{"windows", config.BaseRoaming, "Code/Code Cache", "AI缓存", model.RiskSafe, "VS Code 代码缓存"},
		{"windows", config.BaseRoaming, "Code/User/workspaceStorage", "AI缓存", model.RiskCaution, "VS Code 工作区存储"},
		// Ollama / LM Studio
		{"windows", config.BaseLocal, "ollama/models", "AI模型", model.RiskReview, "Ollama 本地模型(下载大模型,谨慎清理)"},
		{"windows", config.BaseLocal, "LM Studio/models", "AI模型", model.RiskReview, "LM Studio 模型(谨慎清理)"},
		{"windows", config.BaseLocal, "lm-studio/models", "AI模型", model.RiskReview, "LM Studio 模型(谨慎清理)"},
		// 豆包 Doubao（Electron/Chromium 结构：登录态在 User Data 根与 Default 下的
		// Cookies/Local State/IndexedDB 等文件，只清理纯缓存子目录，绝不碰整目录）
		{"windows", config.BaseLocal, "Doubao/User Data/Default/Cache", "AI缓存", model.RiskSafe, "豆包网页缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/Default/Code Cache", "AI缓存", model.RiskSafe, "豆包代码缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/Default/GPUCache", "AI缓存", model.RiskSafe, "豆包 GPU 缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/Default/DawnGraphiteCache", "AI缓存", model.RiskSafe, "豆包图形缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/Default/DawnWebGPUCache", "AI缓存", model.RiskSafe, "豆包 WebGPU 缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/Default/blob_storage", "AI缓存", model.RiskSafe, "豆包 blob 存储"},
		{"windows", config.BaseLocal, "Doubao/User Data/GrShaderCache", "AI缓存", model.RiskSafe, "豆包着色器缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/GraphiteDawnCache", "AI缓存", model.RiskSafe, "豆包图形缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/ShaderCache", "AI缓存", model.RiskSafe, "豆对着色器缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/Crashpad", "AI缓存", model.RiskSafe, "豆包崩溃报告缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/component_crx_cache", "AI缓存", model.RiskSafe, "豆包组件缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/extensions_crx_cache", "AI缓存", model.RiskSafe, "豆包扩展缓存"},
		{"windows", config.BaseLocal, "Doubao/User Data/update_downloads", "升级缓存", model.RiskSafe, "豆包升级下载缓存(重新下载即可)"},
		{"windows", config.BaseLocal, "Doubao/User Data/update_data", "升级缓存", model.RiskCaution, "豆包升级元数据"},
		// DeepSeek
		{"windows", config.BaseRoaming, "DeepSeek/Cache", "AI缓存", model.RiskSafe, "DeepSeek 桌面版缓存"},
		{"windows", config.BaseRoaming, "DeepSeek", "AI缓存", model.RiskCaution, "DeepSeek 桌面版数据"},
		// 常见系统/应用缓存
		{"windows", config.BaseLocal, "Microsoft/Windows/INetCache", "系统缓存", model.RiskSafe, "IE/WinINet 缓存"},
		{"windows", config.BaseLocal, "Microsoft/Edge/User Data/Default/Cache", "浏览器缓存", model.RiskSafe, "Edge 浏览器缓存"},
		{"windows", config.BaseLocal, "Google/Chrome/User Data/Default/Cache", "浏览器缓存", model.RiskSafe, "Chrome 浏览器缓存"},
		{"windows", config.BaseLocal, "Temp", "系统缓存", model.RiskSafe, "系统临时文件"},

		// ── 系统软件升级缓存（本机取证：微信 4.x 升级缓存可达 GB 级，此前全部漏扫）──
		// 腾讯系升级缓存
		{"windows", config.BaseRoaming, "Tencent/xwechat/update", "升级缓存", model.RiskSafe, "微信 4.x 升级下载缓存(清后升级包重新下载)"},
		// 注意：企业微信升级目录实测叫 upgrade（不是 Update），写错名字会永远匹配不到（本机实测 2.0GB）
		{"windows", config.BaseRoaming, "Tencent/WXWork/upgrade", "升级缓存", model.RiskSafe, "企业微信升级暂存包(本机实测2.0GB,清后升级包重新下载)"},
		{"windows", config.BaseRoaming, "Tencent/WXWork/patch", "升级缓存", model.RiskSafe, "企业微信热更新补丁包缓存"},
		{"windows", config.BaseRoaming, "Tencent/WeMail/downloading", "升级缓存", model.RiskCaution, "QQ邮箱下载缓存(清理中邮件需重新下载)"},
		{"windows", config.BaseRoaming, "Tencent/WeChat/XPlugin", "升级缓存", model.RiskCaution, "微信 3.x 插件/升级缓存"},

		// ── 社交应用缓存（本机取证：xwechat 1.8G / WXWork 3.2G 此前几乎全部漏扫）──
		// 微信 4.x（xwechat）：日志/崩溃信息
		{"windows", config.BaseRoaming, "Tencent/xwechat/log", "社交缓存(微信)", model.RiskSafe, "微信 4.x 运行日志(xlog)"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/crashinfo", "社交缓存(微信)", model.RiskSafe, "微信 4.x 崩溃信息"},
		// 微信 4.x Radium 小程序网页运行时：纯缓存子目录（profiles 下是标准 Chromium 缓存簇）
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/cache", "社交缓存(微信)", model.RiskSafe, "微信 4.x 小程序运行时缓存"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/crashpad", "社交缓存(微信)", model.RiskSafe, "微信 4.x 崩溃报告缓存"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/web/profiles_to_delete", "社交缓存(微信)", model.RiskSafe, "微信 4.x 待删除网页档案"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/web/profiles/*/Cache", "社交缓存(微信)", model.RiskSafe, "微信 4.x 小程序网页缓存"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/web/profiles/*/Code Cache", "社交缓存(微信)", model.RiskSafe, "微信 4.x 小程序代码缓存"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/web/profiles/*/GPUCache", "社交缓存(微信)", model.RiskSafe, "微信 4.x 小程序 GPU 缓存"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/web/profiles/*/DawnGraphiteCache", "社交缓存(微信)", model.RiskSafe, "微信 4.x 图形缓存"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/web/profiles/*/DawnWebGPUCache", "社交缓存(微信)", model.RiskSafe, "微信 4.x WebGPU 缓存"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/web/profiles/*/blob_storage", "社交缓存(微信)", model.RiskSafe, "微信 4.x 小程序 blob 存储"},
		// 微信 4.x 小程序用户级缓存（users/<hash>/ 下，只取明确的缓存子目录，不碰 mmkv/applet data 等会话数据）
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/users/*/applet/codecache", "社交缓存(微信)", model.RiskSafe, "微信小程序代码缓存(重新打开自动重建)"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/users/*/applet/publicLib", "社交缓存(微信)", model.RiskSafe, "微信小程序公共库(按需重新下载)"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/users/*/xworker/mpxworker", "社交缓存(微信)", model.RiskCaution, "微信小程序工作线程缓存(删除后按需重建,不影响聊天记录)"},
		{"windows", config.BaseRoaming, "Tencent/xwechat/radium/users/*/xworker/liteapp", "社交缓存(微信)", model.RiskCaution, "微信轻应用缓存(删除后按需重建)"},
		// 微信 4.x 插件组件（RadiumWMPF 框架/播放器/OCR 等，本机实测 637M）
		{"windows", config.BaseRoaming, "Tencent/xwechat/XPlugin/Plugins", "社交缓存(微信)", model.RiskCaution, "微信 4.x 插件组件(删除后按需重新下载,不影响聊天记录)"},
		// 微信 3.x / 共享（部分版本 4.x 也写 Roaming\Tencent\WeChat）
		{"windows", config.BaseRoaming, "Tencent/WeChat/log", "社交缓存(微信)", model.RiskSafe, "微信运行日志"},
		{"windows", config.BaseRoaming, "Tencent/WeChat/crash", "社交缓存(微信)", model.RiskSafe, "微信崩溃日志"},
		// 企业微信：日志与临时转换产物（cef/wmpf_Applet/WeChatOCR/WXDrive_x64 等是组件运行时，勿动）
		{"windows", config.BaseRoaming, "Tencent/WXWork/Log", "社交缓存(企业微信)", model.RiskSafe, "企业微信运行日志"},
		// Electron/Squirrel 安装器升级临时目录
		{"windows", config.BaseLocal, "SquirrelTemp", "升级缓存", model.RiskSafe, "Electron 应用安装器升级临时目录"},
		// 系统级升级缓存（ProgramData / Windows 目录，部分需管理员权限，删除失败会列入失败项）
		{"windows", config.BaseProgramData, "Package Cache", "升级缓存", model.RiskReview, "应用安装器引导缓存(VS/VC++ 运行库等,删除影响卸载与修复,谨慎)"},
		{"windows", config.BaseProgramData, "NVIDIA Corporation/Downloader", "升级缓存", model.RiskSafe, "NVIDIA 驱动下载缓存"},
		{"windows", config.BaseProgramData, "NVIDIA Corporation/Installer2", "升级缓存", model.RiskReview, "NVIDIA 安装器缓存(删除影响驱动卸载,谨慎)"},
		{"windows", config.BaseWindows, "SoftwareDistribution/Download", "升级缓存", model.RiskReview, "Windows 更新下载缓存(需管理员权限)"},
		{"windows", config.BaseWindows, "ServiceProfiles/NetworkService/AppData/Local/Microsoft/Windows/DeliveryOptimization/Cache", "升级缓存", model.RiskReview, "Windows 传递优化缓存(需管理员权限)"},
		{"windows", config.BaseWindows, "Temp", "系统缓存", model.RiskCaution, "系统级临时目录(需管理员权限)"},

		// ── 系统空间回收（对齐 Dism++「空间回收」的文件级项目）──
		// 注：C:\Windows 下的目录需要管理员权限才能删除，权限不足时列入失败项，属预期行为。
		{"windows", config.BaseWindows, "Logs/CBS", "系统空间回收", model.RiskSafe, "组件服务 CBS 日志(可膨胀至 GB 级,需管理员权限)"},
		{"windows", config.BaseWindows, "Logs/DISM", "系统空间回收", model.RiskSafe, "DISM 日志(需管理员权限)"},
		{"windows", config.BaseWindows, "Minidump", "系统空间回收", model.RiskSafe, "蓝屏小型内存转储(需管理员权限)"},
		{"windows", config.BaseWindows, "LiveKernelReports", "系统空间回收", model.RiskSafe, "内核实时报告转储(需管理员权限)"},
		{"windows", config.BaseWindows, "Prefetch", "系统空间回收", model.RiskCaution, "启动预取文件(删除后短暂影响启动加速,自动重建,需管理员权限)"},
		{"windows", config.BaseProgramData, "Microsoft/Windows/WER", "系统空间回收", model.RiskSafe, "Windows 错误报告(系统级)"},
		{"windows", config.BaseLocal, "Microsoft/Windows/WER", "系统空间回收", model.RiskSafe, "Windows 错误报告(用户级)"},
		{"windows", config.BaseLocal, "D3DSCache", "系统空间回收", model.RiskSafe, "DirectX 着色器缓存(游戏/图形程序自动重建)"},
		{"windows", config.BaseSystemDrive, "$RECYCLE.BIN", "系统空间回收", model.RiskCaution, "回收站(删除后无法找回其中文件,系统盘)"},
		{"windows", config.BaseSystemDrive, "Windows.old", "系统空间回收", model.RiskReview, "旧系统备份(删除后无法回退到升级前的 Windows)"},
	}
}

// ── macOS 专有（~/Library 体系） ──
func knownDarwin() []knownDef {
	return []knownDef{
		// Trae
		{"darwin", config.BaseRoaming, "Trae", "AI缓存", model.RiskCaution, "Trae 应用数据"},
		{"darwin", config.BaseLocal, "Trae", "AI缓存", model.RiskSafe, "Trae 系统缓存"},
		// WorkBuddy / CodeBuddy
		{"darwin", config.BaseRoaming, "WorkBuddy", "AI缓存", model.RiskCaution, "WorkBuddy 应用数据"},
		{"darwin", config.BaseLocal, "com.tencent.workbuddy", "AI缓存", model.RiskSafe, "WorkBuddy 系统缓存"},
		// Claude Desktop
		{"darwin", config.BaseRoaming, "Claude", "AI缓存", model.RiskCaution, "Claude 桌面版数据"},
		{"darwin", config.BaseLocal, "io.anthropic.Claude", "AI缓存", model.RiskSafe, "Claude 桌面版系统缓存"},
		// Windsurf / Codeium
		{"darwin", config.BaseRoaming, "Windsurf", "AI缓存", model.RiskCaution, "Windsurf 应用数据"},
		{"darwin", config.BaseLocal, "Windsurf", "AI缓存", model.RiskSafe, "Windsurf 系统缓存"},
		// Cursor
		{"darwin", config.BaseRoaming, "Cursor", "AI缓存", model.RiskCaution, "Cursor 应用数据"},
		{"darwin", config.BaseLocal, "Cursor", "AI缓存", model.RiskSafe, "Cursor 系统缓存"},
		// VS Code
		{"darwin", config.BaseRoaming, "Code", "AI缓存", model.RiskCaution, "VS Code 数据(含AI扩展)"},
		{"darwin", config.BaseLocal, "com.microsoft.VSCode", "AI缓存", model.RiskSafe, "VS Code 系统缓存"},
		// Cherry Studio
		{"darwin", config.BaseRoaming, "CherryStudio", "AI缓存", model.RiskCaution, "Cherry Studio 数据"},
		// Ollama / LM Studio
		{"darwin", config.BaseRoaming, "Ollama/models", "AI模型", model.RiskReview, "Ollama 本地模型"},
		{"darwin", config.BaseRoaming, "LMStudio", "AI模型", model.RiskReview, "LM Studio 数据"},
		// 豆包（只清缓存子目录，整目录含登录/配置，不清）
		{"darwin", config.BaseRoaming, "Doubao/cache", "AI缓存", model.RiskSafe, "豆包缓存"},
		// 常见浏览器/应用
		{"darwin", config.BaseLocal, "com.google.Chrome", "浏览器缓存", model.RiskSafe, "Chrome 系统缓存"},
		{"darwin", config.BaseLocal, "com.microsoft.Edge", "浏览器缓存", model.RiskSafe, "Edge 系统缓存"},
		{"darwin", config.BaseLocal, "com.apple.Safari", "浏览器缓存", model.RiskSafe, "Safari 缓存"},
	}
}

// ── Linux 专有（~/.cache ~/.config 体系） ──
func knownLinux() []knownDef {
	return []knownDef{
		// Trae
		{"linux", config.BaseRoaming, "trae", "AI缓存", model.RiskCaution, "Trae 配置数据"},
		{"linux", config.BaseLocal, "trae", "AI缓存", model.RiskSafe, "Trae 缓存"},
		// WorkBuddy / CodeBuddy
		{"linux", config.BaseRoaming, "WorkBuddy", "AI缓存", model.RiskCaution, "WorkBuddy 配置数据"},
		{"linux", config.BaseLocal, "com.tencent.workbuddy", "AI缓存", model.RiskSafe, "WorkBuddy 缓存"},
		// Claude Desktop
		{"linux", config.BaseRoaming, "Claude", "AI缓存", model.RiskCaution, "Claude 桌面版数据"},
		{"linux", config.BaseLocal, "Claude", "AI缓存", model.RiskSafe, "Claude 桌面版缓存"},
		// Windsurf / Codeium
		{"linux", config.BaseRoaming, "Windsurf", "AI缓存", model.RiskCaution, "Windsurf 配置数据"},
		{"linux", config.BaseLocal, "Windsurf", "AI缓存", model.RiskSafe, "Windsurf 缓存"},
		// Cursor
		{"linux", config.BaseRoaming, "Cursor", "AI缓存", model.RiskCaution, "Cursor 配置数据"},
		{"linux", config.BaseLocal, "Cursor", "AI缓存", model.RiskSafe, "Cursor 缓存"},
		// VS Code
		{"linux", config.BaseRoaming, "Code", "AI缓存", model.RiskCaution, "VS Code 数据(含AI扩展)"},
		{"linux", config.BaseLocal, "Code", "AI缓存", model.RiskSafe, "VS Code 缓存"},
		// Cherry Studio
		{"linux", config.BaseRoaming, "CherryStudio", "AI缓存", model.RiskCaution, "Cherry Studio 数据"},
		// Ollama / LM Studio
		{"linux", config.BaseRoaming, "ollama/models", "AI模型", model.RiskReview, "Ollama 本地模型"},
		{"linux", config.BaseLocal, "lm-studio/models", "AI模型", model.RiskReview, "LM Studio 模型"},
		// 豆包（只清缓存子目录，整目录含登录/配置，不清）
		{"linux", config.BaseRoaming, "Doubao/cache", "AI缓存", model.RiskSafe, "豆包缓存"},
		// 常见浏览器
		{"linux", config.BaseLocal, "google-chrome", "浏览器缓存", model.RiskSafe, "Chrome 缓存"},
		{"linux", config.BaseLocal, "microsoft-edge", "浏览器缓存", model.RiskSafe, "Edge 缓存"},
	}
}

// ── 跨平台通用的用户根隐藏目录（.claude/.workbuddy 等，三系统路径一致） ──
func knownCrossPlatform() []knownDef {
	return []knownDef{
		// Claude Code（CLI）
		{"all", config.BaseHome, ".claude/projects", "AI缓存", model.RiskReview, "Claude Code 项目会话历史(清理可强制重载清爽上下文)"},
		{"all", config.BaseHome, ".claude/tasks", "AI缓存", model.RiskReview, "Claude Code 任务上下文"},
		{"all", config.BaseHome, ".claude/logs", "AI缓存", model.RiskSafe, "Claude Code 日志"},
		{"all", config.BaseHome, ".claude/file-history", "AI缓存", model.RiskCaution, "Claude Code 文件历史"},
		// Cursor
		{"all", config.BaseHome, ".cursor", "AI缓存", model.RiskSafe, "Cursor 编辑器缓存(代码补全历史)"},
		{"all", config.BaseHome, ".cursor/extensions", "AI缓存", model.RiskSafe, "Cursor 扩展缓存"},
		// Trae 根隐藏目录
		{"all", config.BaseHome, ".trae-cn", "AI缓存", model.RiskCaution, "Trae 国内版根目录(含配置与缓存)"},
		{"all", config.BaseHome, ".trae", "AI缓存", model.RiskCaution, "Trae 根目录(含配置与缓存)"},
		// WorkBuddy / CodeBuddy
		{"all", config.BaseHome, ".workbuddy/tmp", "AI缓存", model.RiskSafe, "WorkBuddy 临时文件"},
		{"all", config.BaseHome, ".workbuddy/logs", "AI缓存", model.RiskSafe, "WorkBuddy 日志"},
		{"all", config.BaseHome, ".workbuddy/traces", "AI缓存", model.RiskSafe, "WorkBuddy 跟踪数据"},
		{"all", config.BaseHome, ".workbuddy/plugins", "AI缓存", model.RiskSafe, "WorkBuddy 插件缓存"},
		{"all", config.BaseHome, ".workbuddy/binaries", "AI缓存", model.RiskCaution, "WorkBuddy 运行时(清理后重新下载)"},
		{"all", config.BaseHome, ".workbuddy/projects", "AI缓存", model.RiskReview, "WorkBuddy 项目上下文(可能导致输出不准)"},
		{"all", config.BaseHome, ".workbuddy/sessions", "AI缓存", model.RiskReview, "WorkBuddy 会话上下文"},
		{"all", config.BaseHome, ".workbuddy/tasks", "AI缓存", model.RiskReview, "WorkBuddy 任务上下文"},
		{"all", config.BaseHome, "WorkBuddy", "AI缓存", model.RiskCaution, "WorkBuddy 根目录"},
		// Codeium / Windsurf
		{"all", config.BaseHome, ".codeium/windsurf", "AI缓存", model.RiskCaution, "Windsurf/Codeium 会话与记忆"},
		// 通义灵码 / Qoder
		{"all", config.BaseHome, ".lingma/cache", "AI缓存", model.RiskSafe, "通义灵码缓存"},
		{"all", config.BaseHome, ".lingma/index", "AI缓存", model.RiskCaution, "通义灵码索引(可膨胀)"},
		{"all", config.BaseHome, ".lingma", "AI缓存", model.RiskCaution, "通义灵码根目录"},
		// 文心快码
		{"all", config.BaseHome, ".comate", "AI缓存", model.RiskCaution, "文心快码记忆目录"},
		// Cline / Continue
		{"all", config.BaseHome, ".cline", "AI缓存", model.RiskCaution, "Cline 数据"},
		{"all", config.BaseHome, ".continue/cache", "AI缓存", model.RiskSafe, "Continue 缓存"},
		{"all", config.BaseHome, ".continue/models", "AI缓存", model.RiskSafe, "Continue 模型缓存"},
		{"all", config.BaseHome, ".continue/sessions", "AI缓存", model.RiskCaution, "Continue 会话历史"},
		// Codex
		{"all", config.BaseHome, ".codex/cache", "AI缓存", model.RiskSafe, "OpenAI Codex 缓存"},
		{"all", config.BaseHome, ".codex/logs", "AI缓存", model.RiskSafe, "OpenAI Codex 日志"},
		{"all", config.BaseHome, ".codex/sessions", "AI缓存", model.RiskCaution, "OpenAI Codex 会话"},
		{"all", config.BaseHome, ".codex", "AI缓存", model.RiskCaution, "OpenAI Codex 根目录"},
		// Ollama / LM Studio
		{"all", config.BaseHome, ".ollama/models", "AI模型", model.RiskReview, "Ollama 本地模型"},
		{"all", config.BaseHome, ".lmstudio", "AI模型", model.RiskReview, "LM Studio 数据"},
		// Hugging Face
		{"all", config.BaseLocal, "huggingface", "AI模型", model.RiskCaution, "HuggingFace 模型缓存(重建费带宽)"},
		// Kimi / DeepSeek 会话
		{"all", config.BaseHome, ".kimi/sessions", "AI缓存", model.RiskCaution, "Kimi 会话历史"},
		{"all", config.BaseHome, ".deepseek/sessions", "AI缓存", model.RiskCaution, "DeepSeek 会话历史"},
		// VS Code 扩展缓存
		{"all", config.BaseHome, ".vscode/extensions/.cache", "AI缓存", model.RiskSafe, "VS Code 扩展缓存(含AI扩展)"},
	}
}
