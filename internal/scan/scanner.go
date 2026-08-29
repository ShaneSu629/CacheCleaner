package scan

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cachecleaner/internal/config"
	"cachecleaner/internal/db"
	"cachecleaner/internal/model"
)

// ProgressFunc 是可选的扫描进度回调，供 UI 实时展示进度。
//   - phase: 当前阶段名（如 "自动发现缓存目录"）
//   - done/total: 该阶段内已处理 / 总项数；total<=0 表示总量未知（前端用不确定态）
//   - step/stepTotal: 整体阶段的序号与总数（用于跨阶段连续显示进度）
type ProgressFunc func(phase string, done, total int, step, stepTotal int)

// 扫描阶段总数：已知缓存(1) / 自动发现(2) / 通用识别(3) / 社交缓存(4) / 自定义目录(5)
const stepTotalAll = 5

// report 安全调用进度回调（p 为 nil 时直接跳过）。
func report(p ProgressFunc, phase string, done, total, step, stepTotal int) {
	if p != nil {
		p(phase, done, total, step, stepTotal)
	}
}

// FindKnownCaches 返回当前系统下真实存在的已知缓存项。progress 可选，用于实时回传进度。
func FindKnownCaches(cfg *config.Config, progress ProgressFunc) []model.CacheEntry {
	report(progress, "扫描已知缓存", 0, 1, 1, stepTotalAll)
	res := db.BuildKnownCaches(cfg)
	report(progress, "扫描已知缓存", 1, 1, 1, stepTotalAll)
	return res
}

// AutoDiscover 通过关键词自动发现常见缓存目录（npm/pip/maven/gradle/jetbrains 等非 Electron 类）。
// 扫描根：AppData 三目录 + ProgramData（系统级厂商缓存）+ 文档目录（社交/应用缓存）+ 用户根隐藏目录。
// 关键词除经典缓存名外，还覆盖升级/补丁/崩溃类暂存目录（update/upgrade/patch/crashpad 等）。
func AutoDiscover(cfg *config.Config, progress ProgressFunc) []model.CacheEntry {
	keywords := []string{
		"cache", "caches", "tmp", "temp", "logs", "log", "downloads",
		"update", "upgrade", "patch", "crashpad", "crashinfo", "crash reports",
	}

	// 每个扫描根有独立的深度预算：文档目录树相对浅且是社交应用缓存重灾区，给 3 层。
	type rootDepth struct {
		path  string
		depth int
	}
	roots := []rootDepth{}
	for _, b := range []config.Base{config.BaseLocal, config.BaseRoaming, config.BaseLocalLow, config.BaseProgramData} {
		if r := cfg.Resolve(b); r != "" {
			roots = append(roots, rootDepth{r, 2})
		}
	}
	if r := cfg.Resolve(config.BaseDocuments); r != "" {
		roots = append(roots, rootDepth{r, 3})
	}
	// 用户根下的隐藏目录也纳入（如 .npm/.gradle/.m2）
	if entries, err := os.ReadDir(cfg.Home); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), ".") {
				roots = append(roots, rootDepth{filepath.Join(cfg.Home, e.Name()), 2})
			}
		}
	}

	var out []model.CacheEntry
	seen := map[string]bool{}
	report(progress, "自动发现缓存目录", 0, len(roots), 2, stepTotalAll)
	for i, rd := range roots {
		discoverFrom(rd.path, 0, rd.depth, keywords, cfg, seen, &out)
		report(progress, "自动发现缓存目录", i+1, len(roots), 2, stepTotalAll)
	}
	return out
}

func discoverFrom(root string, depth, maxDepth int, keywords []string, cfg *config.Config, seen map[string]bool, out *[]model.CacheEntry) {
	if depth > maxDepth {
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		full := filepath.Join(root, name)

		// 跳过 Electron 标准缓存（交给通用结构识别处理，避免重复）
		if db.IsElectronSafe(name) || db.IsElectronExcluded(name) {
			discoverFrom(full, depth+1, maxDepth, keywords, cfg, seen, out)
			continue
		}
		hit := false
		for _, kw := range keywords {
			if strings.EqualFold(name, kw) {
				hit = true
				break
			}
		}
		if hit && !seen[full] && !cfg.IsExcluded(full) {
			seen[full] = true
			size, fc, la := db.ScanDir(full)
			if size > 0 {
				*out = append(*out, model.CacheEntry{
					Path:       full,
					ShortPath:  db.ShortOf(cfg, full),
					Category:   "自动发现",
					Risk:       model.RiskCaution,
					Desc:       "关键词匹配的缓存目录: " + name,
					Size:       size,
					FileCount:  fc,
					LastAccess: la,
				})
			}
		}
		discoverFrom(full, depth+1, maxDepth, keywords, cfg, seen, out)
	}
}

// FindElectronCaches 通用结构识别：不认工具名，只认 Electron/Chromium 标准缓存目录结构。
// 遍历 AppData 三目录 + ProgramData + 文档目录 + 用户根隐藏目录，命中标准缓存名即判定为可清理缓存簇。
func FindElectronCaches(cfg *config.Config, progress ProgressFunc) []model.CacheEntry {
	var out []model.CacheEntry
	seen := map[string]bool{}

	// 收集所有待扫描根及其递归深度，便于按根回传进度
	type rootDepth struct {
		path  string
		depth int
	}
	roots := []rootDepth{}
	// AppData 体系：深度 3（足以覆盖 App\UserData\Cache 这类嵌套）
	for _, b := range []config.Base{config.BaseLocal, config.BaseRoaming, config.BaseLocalLow} {
		if r := cfg.Resolve(b); r != "" {
			roots = append(roots, rootDepth{r, 3})
		}
	}
	// ProgramData（系统级 Electron 应用缓存，如全局安装的套壳应用）：深度 2
	if r := cfg.Resolve(config.BaseProgramData); r != "" {
		roots = append(roots, rootDepth{r, 2})
	}
	// 文档目录（QQ NT 等社交应用的 Chromium 缓存可能落在文档下）：深度 3
	if r := cfg.Resolve(config.BaseDocuments); r != "" {
		roots = append(roots, rootDepth{r, 3})
	}
	// 用户根隐藏目录（.workbuddy/.trae-cn/.claude 等）：深度 5，覆盖更深缓存
	if entries, err := os.ReadDir(cfg.Home); err == nil {
		for _, e := range entries {
			if e.IsDir() && strings.HasPrefix(e.Name(), ".") {
				roots = append(roots, rootDepth{filepath.Join(cfg.Home, e.Name()), 5})
			}
		}
	}

	report(progress, "通用识别 Electron 缓存", 0, len(roots), 3, stepTotalAll)
	for i, rd := range roots {
		collectElectron(rd.path, 0, rd.depth, cfg, seen, &out)
		report(progress, "通用识别 Electron 缓存", i+1, len(roots), 3, stepTotalAll)
	}
	return out
}

// electronContainerDirs 是 Chromium 应用常见的"档案容器"目录名。
// 这类目录本身不是缓存，但它的下一层才是各个 profile（如微信 4.x 的
// radium/web/profiles/<随机名>/Cache），层级深度不可预知，因此进入容器时
// 额外放宽深度预算，让标准缓存簇始终可达。
var electronContainerDirs = map[string]bool{
	"profiles":  true,
	"user data": true,
	"userdata":  true,
}

// deepVendorDirs 是国产应用常见的"厂商总目录"（位于 AppData 各根下）。
// 这些厂商的应用普遍采用 <厂商>/<应用>/<运行时>/<版本或随机名>/... 的深层嵌套
// （如 Tencent/xwechat/radium/web/profiles/<随机名>/Cache，可达 6 层以上），
// 固定深度预算根本够不到；进入厂商目录时把预算加深，让其子树内的标准缓存簇可达。
// 只针对这些子树加深（而非全局加深），遍历成本可控。
var deepVendorDirs = map[string]bool{
	"tencent": true, "kingsoft": true, "sogou": true, "baidu": true,
	"netease": true, "alibaba": true, "alipay": true, "bytedance": true,
	"360safe": true, "thunder": true, "bilibili": true, "dingtalk": true,
	"feishu": true, "lark": true, "iqiyi": true, "youku": true,
	"kugou": true, "kuwo": true, "huya": true, "douyu": true, "mozilla": true,
}

// collectElectronBoost 给容器目录额外增加的深度预算。
const collectElectronBoost = 3

// deepVendorBoost 给厂商子树额外增加的深度预算。
const deepVendorBoost = 6

func collectElectron(root string, depth, maxDepth int, cfg *config.Config, seen map[string]bool, out *[]model.CacheEntry) {
	if depth > maxDepth {
		return
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		full := filepath.Join(root, name)

		if db.IsElectronExcluded(name) {
			// 排除目录（如 User Data）仍需下钻找其中的 Cache
			collectElectron(full, depth+1, maxDepth, cfg, seen, out)
			continue
		}
		// 深度加成：档案容器（profiles/userdata 等）与国产厂商总目录的子树
		// 层级不可预知，放宽深度预算让深处的标准缓存簇可达。
		nextMax := maxDepth
		lower := strings.ToLower(name)
		if electronContainerDirs[lower] && nextMax < depth+collectElectronBoost {
			nextMax = depth + collectElectronBoost
		}
		if deepVendorDirs[lower] && nextMax < depth+deepVendorBoost {
			nextMax = depth + deepVendorBoost
		}
		if db.IsElectronSafe(name) {
			if !seen[full] && !cfg.IsExcluded(full) {
				seen[full] = true
				size, fc, la := db.ScanDir(full)
				if size > 0 {
					app := appName(cfg, full)
					*out = append(*out, model.CacheEntry{
						Path:       full,
						ShortPath:  db.ShortOf(cfg, full),
						Category:   "Electron缓存(" + app + ")",
						Risk:       model.RiskSafe,
						Desc:       "通用识别的 " + name + " 缓存(归属应用: " + app + ")",
						Size:       size,
						FileCount:  fc,
						LastAccess: la,
					})
				}
			}
		}
		collectElectron(full, depth+1, nextMax, cfg, seen, out)
	}
}

// appName 从缓存项路径中提取"所属应用名"。
func appName(cfg *config.Config, full string) string {
	rel := db.ShortOf(cfg, full)
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) >= 3 && parts[0] == "AppData" {
		return parts[2]
	}
	if len(parts) >= 1 {
		return parts[0]
	}
	return "未知"
}

// ── 合并入口 ──

var aiKeywords = []string{
	"cursor", "trae", "claude", "workbuddy", "lingma", "qoder", "windsurf", "codeium",
	"copilot", "aider", "continue", "kimi", "doubao", "cherry", "codex", "comate",
	"cline", "ollama", "hugging", "vscode", "github", "gpt", "deepseek", "yuanbao", "tongyi",
	"anthropic", "openai", "qwen", "gemini", "chatgpt", "moonshot", "zhipu",
	"minimax", "lmstudio", "lm studio",
}

// appOf 从缓存项的分类/路径中提取"所属应用名"，用于 AI 聚焦判断。
func appOf(category, short string) string {
	const prefix = "Electron缓存("
	if strings.HasPrefix(category, prefix) && strings.HasSuffix(category, ")") {
		return category[len(prefix) : len(category)-1]
	}
	parts := strings.Split(short, string(os.PathSeparator))
	if len(parts) > 0 {
		return parts[0]
	}
	return short
}

func isAIFocus(name string) bool {
	s := strings.ToLower(name)
	for _, kw := range aiKeywords {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// FindCustomCaches 扫描用户自定义目录：目录本身作为缓存项整体纳入（存在且未被排除才返回）。
func FindCustomCaches(cfg *config.Config, progress ProgressFunc) []model.CacheEntry {
	report(progress, "扫描自定义目录", 0, len(cfg.CustomDirs), 5, stepTotalAll)
	var out []model.CacheEntry
	for i, dir := range cfg.CustomDirs {
		if dir == "" {
			continue
		}
		full := filepath.Clean(dir)
		if fi, err := os.Stat(full); err == nil && fi.IsDir() && !cfg.IsExcluded(full) {
			size, fc, la := db.ScanDir(full)
			out = append(out, model.CacheEntry{
				Path:       full,
				ShortPath:  db.ShortOf(cfg, full),
				Category:   "自定义目录",
				Risk:       model.RiskCaution,
				Desc:       "用户指定的缓存目录",
				Size:       size,
				FileCount:  fc,
				LastAccess: la,
			})
		}
		report(progress, "扫描自定义目录", i+1, len(cfg.CustomDirs), 5, stepTotalAll)
	}
	return out
}

// findSocial 带进度回传地执行社交应用缓存发现（容器数事先未知，用起止两点表示）。
func findSocial(cfg *config.Config, progress ProgressFunc) []model.CacheEntry {
	report(progress, "扫描社交应用缓存", 0, 1, 4, stepTotalAll)
	res := FindSocialCaches(cfg)
	report(progress, "扫描社交应用缓存", 1, 1, 4, stepTotalAll)
	return res
}

// DeepAIScan 深度 AI 扫描：聚焦 AI/IDE 类缓存，合并已知 + 自动发现 + 通用识别 + 自定义目录。
func DeepAIScan(cfg *config.Config, progress ProgressFunc) []model.CacheEntry {
	known := FindKnownCaches(cfg, progress)
	discovered := AutoDiscover(cfg, progress)
	electron := FindElectronCaches(cfg, progress)
	findSocial(cfg, progress) // 社交发现不纳入 AI 聚焦，但保持阶段进度一致
	custom := FindCustomCaches(cfg, progress)

	var pool []model.CacheEntry
	for _, e := range known {
		if strings.Contains(e.Category, "AI") {
			pool = append(pool, e)
		}
	}
	for _, e := range discovered {
		if isAIFocus(appOf(e.Category, e.ShortPath)) {
			pool = append(pool, e)
		}
	}
	for _, e := range electron {
		if isAIFocus(appOf(e.Category, e.ShortPath)) {
			pool = append(pool, e)
		}
	}
	// 用户显式添加的自定义目录始终纳入
	pool = append(pool, custom...)
	return dedupe(pool)
}

// SmartScan 一键扫描：合并所有已知 + 自动发现 + 通用识别 + 社交缓存 + 自定义目录（含浏览器/IM/网盘等）。
func SmartScan(cfg *config.Config, progress ProgressFunc) []model.CacheEntry {
	known := FindKnownCaches(cfg, progress)
	discovered := AutoDiscover(cfg, progress)
	electron := FindElectronCaches(cfg, progress)
	social := findSocial(cfg, progress)
	custom := FindCustomCaches(cfg, progress)
	pool := append(append(known, discovered...), electron...)
	pool = append(pool, social...)
	pool = append(pool, custom...)
	return dedupe(pool)
}

// dedupe 按路径去重，已知项优先（风险/描述更准确）。
func dedupe(entries []model.CacheEntry) []model.CacheEntry {
	seen := map[string]int{}
	var out []model.CacheEntry
	for _, e := range entries {
		if idx, ok := seen[e.Path]; ok {
			// 已知项（分类非 Electron/自动发现）优先保留
			if strings.Contains(out[idx].Category, "Electron") || strings.Contains(out[idx].Category, "自动发现") {
				if !strings.Contains(e.Category, "Electron") && !strings.Contains(e.Category, "自动发现") {
					out[idx] = e
				}
			}
			continue
		}
		seen[e.Path] = len(out)
		out = append(out, e)
	}
	return dedupeParents(out)
}

// dedupeParents 移除被其他缓存项完整包含的子孙目录，避免父子目录重复计数。
// 例如 Electron 的 X\Cache（递归大小已含其全部内容）与其子目录 X\Cache\Cache_data
// 大小相同却都被列出；仅保留更靠上的祖先即可，既避免合计虚高，也避免清理父目录时
// 子目录被重复计入。按路径长度升序处理，优先保留祖先。
func dedupeParents(entries []model.CacheEntry) []model.CacheEntry {
	sorted := make([]model.CacheEntry, len(entries))
	copy(sorted, entries)
	sort.SliceStable(sorted, func(i, j int) bool {
		return len(sorted[i].Path) < len(sorted[j].Path)
	})
	kept := make([]model.CacheEntry, 0, len(sorted))
	for _, e := range sorted {
		covered := false
		ek := strings.ToLower(e.Path)
		for _, k := range kept {
			kk := strings.ToLower(k.Path)
			if strings.HasPrefix(ek, kk+string(os.PathSeparator)) {
				covered = true
				break
			}
		}
		if !covered {
			kept = append(kept, e)
		}
	}
	return kept
}
