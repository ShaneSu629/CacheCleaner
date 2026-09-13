//go:build windows

package uninstaller

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"cachecleaner/internal/applog"
	"cachecleaner/internal/db"

	"golang.org/x/sys/windows/registry"
)

// uninstallRoots 是四路注册表卸载项枚举源：
//   - HKLM 64 位视图（64 位系统上多数程序）
//   - HKLM 32 位视图（WOW6432Node）
//   - HKCU 64/32（当前用户安装的程序，免提权）
var uninstallRoots = []struct {
	key   registry.Key
	view  uint32
	hcu   bool
	label string
}{
	{registry.LOCAL_MACHINE, registry.WOW64_64KEY, false, "HKLM64"},
	{registry.LOCAL_MACHINE, registry.WOW64_32KEY, false, "HKLM32"},
	{registry.CURRENT_USER, registry.WOW64_64KEY, true, "HKCU64"},
	{registry.CURRENT_USER, registry.WOW64_32KEY, true, "HKCU32"},
}

// ListApps 枚举全部已安装软件（Windows 注册表 Uninstall 四路）。
func (p *Service) ListApps() []App {
	var apps []App
	seen := map[string]bool{}

	for _, src := range uninstallRoots {
		k, err := registry.OpenKey(src.key,
			`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall`, registry.ENUMERATE_SUB_KEYS|src.view)
		if err != nil {
			continue
		}
		subs, err := k.ReadSubKeyNames(-1)
		k.Close()
		if err != nil {
			continue
		}
		for _, sub := range subs {
			app := readUninstallEntry(src.key, src.view, src.hcu, sub)
			if app.Name == "" || seen[app.Key] {
				continue
			}
			seen[app.Key] = true
			// 跳过系统组件与更新补丁（无法也不应卸载）
			if isSystemComponent(app.Name) {
				continue
			}
			size, _, _ := db.ScanDir(app.InstallDir)
			app.Size = size
			// 流氓软件识别
			if reason, isPUP := DetectPUP(app); isPUP {
				app.PUP = true
				app.PUPReason = reason
			}
			apps = append(apps, app)
		}
	}
	sortApps(apps)
	applog.Info("枚举已安装软件: 共 %d 项（含流氓软件 %d 项）", len(apps), countPUP(apps))
	return apps
}

// countPUP 统计流氓软件数量（供日志）。
func countPUP(apps []App) int {
	n := 0
	for _, a := range apps {
		if a.PUP {
			n++
		}
	}
	return n
}

// readUninstallEntry 读取单个卸载键并组装 App。
func readUninstallEntry(root registry.Key, view uint32, hcu bool, sub string) App {
	k, err := registry.OpenKey(root,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\`+sub, registry.QUERY_VALUE|view)
	if err != nil {
		return App{}
	}
	defer k.Close()

	get := func(name string) string {
		v, _, err := k.GetStringValue(name)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(v)
	}
	name := get("DisplayName")
	if name == "" {
		return App{}
	}
	uninst := get("UninstallString")
	dir := get("InstallLocation")
	if dir == "" {
		dir = inferInstallDir(uninst)
	}
	return App{
		Key:         sub,
		Name:        name,
		Version:     get("DisplayVersion"),
		Publisher:   get("Publisher"),
		InstallDate: get("InstallDate"),
		InstallDir:  dir,
		Uninstall:   uninst,
		Is64:        view == registry.WOW64_64KEY,
		HKCU:        hcu,
	}
}

// inferInstallDir 从卸载命令推断安装目录（无 InstallLocation 值时）。
func inferInstallDir(uninst string) string {
	if uninst == "" {
		return ""
	}
	low := strings.ToLower(uninst)
	for _, marker := range []string{"uninstall", "uninst", "setup"} {
		i := strings.Index(low, marker)
		if i > 0 {
			return uninst[:i]
		}
	}
	return ""
}

// isSystemComponent 过滤系统组件与更新补丁（不可卸载）。
func isSystemComponent(name string) bool {
	low := strings.ToLower(name)
	markers := []string{
		"hotfix", "security update", "update for", "kb", "service pack",
		"windows software development kit", "windows sdk",
		"microsoft visual c++", "microsoft edge webview2 runtime",
	}
	for _, m := range markers {
		if strings.Contains(low, m) {
			return true
		}
	}
	return false
}

// Uninstall 执行卸载：识别卸载器类型，交互式（默认）或静默（msiexec/Inno），
// 卸载后以「卸载注册表键是否消失」作为成功判据（不依赖返回码）。
func (p *Service) Uninstall(key string) UninstallResult {
	app := p.findByKey(key)
	if app.Name == "" {
		applog.Error("卸载失败：未找到软件 key=%s", key)
		return UninstallResult{Success: false, Message: "未找到该软件"}
	}
	applog.Info("开始卸载: %s (key=%s, 目录=%s)", app.Name, app.Key, app.InstallDir)
	cmd := silentize(app.Uninstall)
	if cmd == "" {
		applog.Error("卸载失败：%s 无卸载程序，原命令=%q", app.Name, app.Uninstall)
		return UninstallResult{Success: false, Message: "该软件没有卸载程序，可尝试强制卸载"}
	}
	// 卸载器文件不存在（如曾被强制卸载删掉目录但注册表键残留）→ 直接判定"已卸载"，
	// 引导清理残留注册表，而不是去执行一个不存在的 exe 报"找不到文件"。
	// 仅当提取到绝对路径（含盘符）时才做存在性检查；msiexec 等系统命令跳过。
	if exe := exePath(cmd); exe != "" && filepath.IsAbs(exe) && !fileExists(exe) {
		applog.Info("卸载器已不存在（可能已卸载）: %s -> %s", app.Name, exe)
		res := UninstallResult{Success: true, Message: "卸载器已不存在，视为已卸载"}
		res.RegResidue = scanRegResidue(app)
		res.Residues = scanDirResidue(app)
		// 卸载键本身可能仍在，标记为可清理
		if uninstallKeyExists(app) {
			res.Message = "卸载器已不存在，注册表项残留，建议强制卸载清理"
			res.Success = false
		}
		return res
	}
	applog.Info("执行卸载命令: %s", cmd)
	code, stderr, err := runUninstall(cmd)
	if err != nil {
		applog.Error("卸载执行异常: %s -> %v", app.Name, err)
		return UninstallResult{Success: false, Message: fmt.Sprintf("卸载执行失败: %v", err)}
	}
	if stderr != "" {
		applog.Info("卸载器输出: %s", strings.TrimSpace(stderr))
	}
	if code != 0 {
		applog.Info("卸载器返回码 %d（%s）", code, app.Name)
	}
	// 先等卸载器主进程返回后，再等真正卸载结束（见 waitUninstallSettled 说明），
	// 避免在卸载向导还在跑的时候就弹残留清理窗口。
	waitUninstallSettled(app)
	// 卸载器返回码不可靠，以"卸载注册表键是否消失"作为成功判据。
	stillRegistered := uninstallKeyExists(app)
	res := UninstallResult{Success: !stillRegistered}
	if stillRegistered {
		res.Message = fmt.Sprintf("卸载程序返回码 %d（注册表项仍存在，可能未完全卸载）", code)
		applog.Error("卸载未完成：%s 卸载键仍存在，返回码=%d", app.Name, code)
	} else {
		res.Message = "卸载完成"
		applog.Info("卸载完成: %s（返回码 %d）", app.Name, code)
	}
	// Geek 式残留扫描
	if res.Success {
		res.RegResidue = scanRegResidue(app)
		res.Residues = scanDirResidue(app)
		if len(res.Residues) > 0 || len(res.RegResidue) > 0 {
			applog.Info("残留扫描: %s 目录残留 %d 项, 注册表残留 %d 项",
				app.Name, len(res.Residues), len(res.RegResidue))
		}
	}
	return res
}

// uninstallKeyExists 判断软件的卸载注册表键是否仍存在（卸载成功的核心判据）。
func uninstallKeyExists(app App) bool {
	root := registry.LOCAL_MACHINE
	if app.HKCU {
		root = registry.CURRENT_USER
	}
	view := uint32(registry.WOW64_64KEY)
	if !app.Is64 {
		view = registry.WOW64_32KEY
	}
	_, err := registry.OpenKey(root,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\`+app.Key,
		registry.QUERY_VALUE|view)
	return err == nil
}

// waitUninstallSettled 等待卸载真正结束（安装目录消失或大小稳定）。
//
// 为什么需要：Inno 等卸载器常是「引导器」——主进程启动后立即返回（返回码 0），
// 真正的卸载由后台副本执行；且删除顺序常是「先删注册表键、再删文件」。
// 因此「注册表键消失」不等于卸载结束：此时卸载向导还在跑、文件还在删，
// 若立刻做残留扫描并弹窗，会打断用户（LocalSend 遇到的现象）。
//
// 判据：安装目录消失 → 立即返回；目录大小连续多次不变 → 视为结束（用户可能取消了）；
// 超时（30 秒）→ 放弃等待。无安装目录信息时直接返回。
func waitUninstallSettled(app App) {
	dir := strings.TrimSpace(app.InstallDir)
	if dir == "" || !filepath.IsAbs(dir) {
		return // 无安装目录信息，无法判断，跳过
	}
	deadline := time.Now().Add(30 * time.Second)
	var last int64 = -1
	stable := 0
	for time.Now().Before(deadline) {
		if _, err := os.Stat(dir); err != nil {
			applog.Info("卸载已结束（安装目录已删除）: %s", app.Name)
			return // 目录没了 = 卸载完成
		}
		size, _, _ := db.ScanDir(dir)
		if size == last {
			stable++
			if stable >= 4 { // 连续 4 次（约 2 秒）大小不变，认为卸载已停
				applog.Info("卸载已结束（安装目录大小稳定）: %s", app.Name)
				return
			}
		} else {
			stable = 0
		}
		last = size
		time.Sleep(500 * time.Millisecond)
	}
	applog.Info("等待卸载结束超时（30 秒），继续残留扫描: %s", app.Name)
}

// ForceUninstall 强制卸载：直接清理注册表卸载项 + 安装目录 + 常见残留位置。
// 仅建议对确认的流氓软件使用（它们卸载器常带挽留/诱导，且会留后门）。
func (p *Service) ForceUninstall(key string) UninstallResult {
	app := p.findByKey(key)
	if app.Name == "" {
		applog.Error("强制卸载失败：未找到软件 key=%s", key)
		return UninstallResult{Success: false, Message: "未找到该软件"}
	}
	applog.Info("开始强制卸载: %s (key=%s, 目录=%s)", app.Name, app.Key, app.InstallDir)
	// 1. 先尝试正常卸载（尊重用户选择；卸载器有自清理逻辑）
	res := p.Uninstall(key)
	// 2. 无论卸载器结果如何，都做深度残留清理（流氓软件卸载器不可信）
	regs := scanRegResidue(app)
	// 卸载项本身的键最后删
	regs = append(regs, deleteUninstallKey(app)...)
	dirs := scanDirResidue(app)
	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			applog.Error("强制卸载：删除残留目录失败 %s -> %v", d, err)
		}
	}
	res.Residues = append(res.Residues, dirs...)
	res.RegResidue = append(res.RegResidue, regs...)
	res.Success = true
	applog.Info("强制卸载完成: %s，删除目录 %d 项，注册表 %d 项",
		app.Name, len(dirs), len(regs))
	return res
}

// findByKey 按注册表键路径反查 App（枚举一次）。
func (p *Service) findByKey(key string) App {
	for _, a := range p.ListApps() {
		if a.Key == key {
			return a
		}
	}
	return App{}
}

// CleanResidue 直接删除给定的残留路径（不依赖注册表键定位软件）。
// 用于「卸载成功后清理残留」场景——此时注册表键已被卸载器删除，
// 无法再 findByKey，但残留路径已在卸载时扫描出来，直接删即可。
func (p *Service) CleanResidue(name string, dirs []string, regs []string) UninstallResult {
	res := UninstallResult{Success: true, Message: "残留已清理"}
	for _, d := range dirs {
		if err := os.RemoveAll(d); err != nil {
			applog.Error("清理残留：删除目录失败 %s -> %v", d, err)
			res.Success = false
		} else {
			applog.Info("清理残留：已删除目录 %s", d)
		}
	}
	for _, r := range regs {
		if err := deleteRegPath(r); err != nil {
			applog.Error("清理残留：删除注册表失败 %s -> %v", r, err)
		} else {
			applog.Info("清理残留：已删除注册表 %s", r)
		}
	}
	res.Residues = dirs
	res.RegResidue = regs
	applog.Info("清理残留完成: %s，目录 %d 项，注册表 %d 项", name, len(dirs), len(regs))
	return res
}

// deleteRegPath 删除指定注册表路径（按 HKCU/HKLM 前缀识别根键）。
func deleteRegPath(path string) error {
	lower := strings.ToLower(path)
	switch {
	case strings.HasPrefix(lower, "hkcu\\"), strings.HasPrefix(lower, "hkey_current_user\\"):
		return registry.DeleteKey(registry.CURRENT_USER, trimRoot(path))
	case strings.HasPrefix(lower, "hklm\\"), strings.HasPrefix(lower, "hkey_local_machine\\"):
		return registry.DeleteKey(registry.LOCAL_MACHINE, trimRoot(path))
	default:
		return fmt.Errorf("无法识别的注册表路径: %s", path)
	}
}

// trimRoot 去掉注册表路径的根键前缀（HKCU\ / HKLM\ 等）。
func trimRoot(path string) string {
	i := strings.Index(path, "\\")
	if i < 0 {
		return path
	}
	return path[i+1:]
}

// scanDirResidue 扫描安装目录残留（卸载后目录仍存在时返回）。
func scanDirResidue(app App) []string {
	var out []string
	dir := strings.TrimSpace(app.InstallDir)
	if dir == "" || !filepath.IsAbs(dir) {
		dir = ""
	}
	// 安装目录本身残留
	if dir != "" {
		if st, err := os.Stat(dir); err == nil && st.IsDir() {
			out = append(out, dir)
		}
	}
	// 常见残留：AppData 下同名目录。用多个候选名匹配，
	// 因为注册表 DisplayName 常带"版本 x.x.x"后缀（如 "LocalSend 版本 1.17.0"），
	// 而实际缓存目录叫纯名字（如 "LocalSend"）。
	candidates := residueNames(app, dir)
	for _, base := range []string{os.Getenv("APPDATA"), os.Getenv("LOCALAPPDATA")} {
		if base == "" {
			continue
		}
		for _, name := range candidates {
			cand := filepath.Join(base, name)
			if st, err := os.Stat(cand); err == nil && st.IsDir() {
				out = append(out, cand)
			}
		}
	}
	return out
}

// residueNames 生成用于残留目录匹配的候选名。
// 优先用安装目录的最后一节（最准确），再去掉 DisplayName 的版本后缀。
func residueNames(app App, installDir string) []string {
	seen := map[string]bool{}
	var out []string
	add := func(n string) {
		n = strings.TrimSpace(n)
		if n == "" || seen[n] {
			return
		}
		seen[n] = true
		out = append(out, n)
	}
	// 1. 安装目录最后一节（如 C:\Program Files\LocalSend → LocalSend）
	if installDir != "" {
		add(filepath.Base(installDir))
	}
	// 2. 去掉版本后缀的 DisplayName（"LocalSend 版本 1.17.0" → "LocalSend"）
	add(stripVersionSuffix(app.Name))
	// 3. 原始 DisplayName（兜底）
	add(app.Name)
	return out
}

// stripVersionSuffix 去掉显示名里的版本后缀：
//   - "LocalSend 版本 1.17.0" → "LocalSend"
//   - "Foo v2.3" → "Foo"
//   - "Bar 1.0.0.1" → "Bar"
func stripVersionSuffix(name string) string {
	re := versionSuffixRe
	return strings.TrimSpace(re.ReplaceAllString(name, ""))
}

// versionSuffixRe 匹配"版本 x.y.z" / " v1.2" / " x.y.z.w" 等版本后缀。
var versionSuffixRe = regexp.MustCompile(`(?i)\s*(版本|version)?\s*v?\d+(\.\d+)+.*$`)
