package main

import (
	"context"
	"embed"
	"sort"
	"strings"
	"sync"
	"time"

	"cachecleaner/internal/applog"
	"cachecleaner/internal/clean"
	"cachecleaner/internal/config"
	"cachecleaner/internal/dismclean"
	"cachecleaner/internal/filelock"
	"cachecleaner/internal/model"
	"cachecleaner/internal/regclean"
	"cachecleaner/internal/scan"
	"cachecleaner/internal/update"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

//go:embed all:frontend/dist
var assets embed.FS

// EntryDTO 是传给前端、可 JSON 序列化的缓存项。
type EntryDTO struct {
	Path      string `json:"path"`
	ShortPath string `json:"shortPath"`
	Category  string `json:"category"`
	Risk      string `json:"risk"`
	Size      int64  `json:"size"`
	FileCount int64  `json:"fileCount"`
}

// HistoryDTO 是清理历史的可序列化结构。
type HistoryDTO struct {
	Time string `json:"time"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// CleanFailure 是一条清理失败明细（路径 + 错误），便于前端精确保留失败项。
type CleanFailure struct {
	Path  string `json:"path"`
	Error string `json:"error"`
}

// LockerDTO 是占用文件的进程信息（传给前端展示）。
type LockerDTO struct {
	Pid  uint32 `json:"pid"`
	Name string `json:"name"`
	Path string `json:"path"`
	Safe bool   `json:"safe"`
}

// CleanResult 是清理结果。Cleaned 为成功清理的路径，前端据此移除列表项；
// Failed 保留失败明细，失败项仍留在列表中供用户重试。
// ScheduledReboot 为"被占用但已登记重启删除"的路径（视同成功，重启后生效）。
type CleanResult struct {
	Freed           int64          `json:"freed"`
	Count           int64          `json:"count"`
	Cleaned         []string       `json:"cleaned"`
	Failed          []CleanFailure `json:"failed"`
	ScheduledReboot []string       `json:"scheduledReboot"`
}

// RegEntryDTO 是传给前端的注册表垃圾项。
type RegEntryDTO struct {
	Key      string `json:"key"`
	Category string `json:"category"`
	Risk     string `json:"risk"`
	Desc     string `json:"desc"`
	Values   int    `json:"values"`
	Size     int64  `json:"size"`
}

// DismReportDTO 是组件存储分析报告的可序列化结构。
type DismReportDTO struct {
	ReportedSize    string `json:"reportedSize"`
	ActualSize      string `json:"actualSize"`
	ReclaimablePkgs string `json:"reclaimablePkgs"`
	LastCleanup     string `json:"lastCleanup"`
	Recommended     bool   `json:"recommended"`
	Raw             string `json:"raw"`
}

// DismInfoDTO 是「系统工具」页初始化信息：工具可用性 + 当前提权状态 + 最近报告。
type DismInfoDTO struct {
	Available bool           `json:"available"`
	Elevated  bool           `json:"elevated"`
	Report    *DismReportDTO `json:"report"`
}

// App 是绑定到前端（window.go.main.App）的结构，承载所有 UI 操作。
// 业务逻辑全部复用内部包，这里只做编排与序列化。
//
// 并发约定：Wails 的每个绑定调用都在独立 goroutine 执行，因此所有对 cfg、
// lastEntries、lastRegEntries 的读写都必须持有 mu。cfg 指针本身在 startup 后不再替换。
type App struct {
	ctx            context.Context
	cfg            *config.Config
	mu             sync.Mutex
	lastEntries    map[string]model.CacheEntry
	lastRegEntries map[string]regclean.Entry
	scanning       bool // 扫描重入保护（前端连点按钮会产生并发扫描）
	cleaning       bool // 清理重入保护
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cfg, err := config.Load()
	if err != nil {
		// 配置加载失败也要保证 cfg 非 nil，否则后续所有绑定方法都会空指针 panic。
		runtime.LogError(a.ctx, "配置加载失败，已回退默认配置: "+err.Error())
		applog.Error("配置加载失败，已回退默认配置: %v", err)
		cfg = config.Default()
	}
	a.mu.Lock()
	a.cfg = cfg
	a.lastEntries = map[string]model.CacheEntry{}
	a.lastRegEntries = map[string]regclean.Entry{}
	a.mu.Unlock()
	applog.Info("启动完成: OS=%s Home=%s Documents=%s Windows=%s",
		cfg.OS, cfg.Home, cfg.Documents, cfg.Windows)

	// 清理上次更新残留（CacheCleaner.old.exe）：更新中断时兜底
	update.CleanupOld()

	// 启动后延迟自动检查更新：走后台 goroutine，失败或处于静默期都不打扰用户。
	// 发现新版本且未跳过时自动开始后台下载（商业软件式静默下载），
	// 下载完成后前端弹提示，用户点「更新并重启」即可完成更新。
	go func() {
		time.Sleep(5 * time.Second)
		info, err := update.Check(false)
		if err != nil {
			applog.Error("启动检查更新失败: %v", err)
			return
		}
		if info.HasUpdate {
			applog.Info("发现新版本: %s -> %s", info.Current, info.Latest)
			if !info.Skipped && !info.Snoozed {
				if derr := update.StartDownload(info); derr != nil {
					applog.Error("自动下载启动失败: %v", derr)
				} else {
					applog.Info("已开始后台自动下载 %s", info.Latest)
				}
			}
		}
		a.emit("update:info", toUpdateDTO(info))
	}()
}

func (a *App) domReady(ctx context.Context) {}

// 关闭前不拦截：直接允许退出。
func (a *App) beforeClose(ctx context.Context) bool { return false }

// emit 向前端广播事件。ctx 未就绪时（启动未完成或单测环境）静默跳过，
// 避免 Wails runtime 因无效 context 打印错误日志。
func (a *App) emit(event string, data ...interface{}) {
	if a.ctx == nil {
		return
	}
	runtime.EventsEmit(a.ctx, event, data...)
}

func toDTO(e model.CacheEntry) EntryDTO {
	return EntryDTO{
		Path:      e.Path,
		ShortPath: e.ShortPath,
		Category:  e.Category,
		Risk:      e.Risk.Label(),
		Size:      e.Size,
		FileCount: e.FileCount,
	}
}

func toRegDTO(e regclean.Entry) RegEntryDTO {
	return RegEntryDTO{
		Key:      e.Key,
		Category: e.Category,
		Risk:     e.Risk.Label(),
		Desc:     e.Desc,
		Values:   e.Values,
		Size:     e.Size,
	}
}

// Scan 在后台执行扫描，过程通过 "scan:progress" 实时回传进度，结果通过 "scan:done" 回传（避免界面假死）。
// 已有扫描在跑时直接忽略重复请求，避免两次扫描的进度/结果事件交错。
func (a *App) Scan(mode string) {
	a.mu.Lock()
	if a.scanning {
		a.mu.Unlock()
		a.emit("scan:busy", nil)
		return
	}
	a.scanning = true
	cfg := a.cfg
	a.mu.Unlock()

	applog.Info("扫描开始: mode=%s", mode)
	go func() {
		defer func() {
			a.mu.Lock()
			a.scanning = false
			a.mu.Unlock()
		}()
		start := time.Now()

		progress := func(phase string, done, total, step, stepTotal int) {
			var pct float64
			if total > 0 {
				pct = (float64(step-1) + float64(done)/float64(total)) / float64(stepTotal) * 100
			} else {
				pct = float64(step-1) / float64(stepTotal) * 100
			}
			a.emit("scan:progress", map[string]interface{}{
				"phase":     phase,
				"done":      done,
				"total":     total,
				"step":      step,
				"stepTotal": stepTotal,
				"pct":       pct,
			})
		}

		var entries []model.CacheEntry
		switch mode {
		case "ai":
			entries = scan.DeepAIScan(cfg, progress)
		default:
			entries = scan.SmartScan(cfg, progress)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Size > entries[j].Size })
		var totalSize int64
		for _, e := range entries {
			totalSize += e.Size
		}
		applog.Info("扫描完成: mode=%s 共 %d 项 / %d 字节, 耗时 %s", mode, len(entries), totalSize, time.Since(start).Round(time.Second))

		dtos := make([]EntryDTO, 0, len(entries))
		for _, e := range entries {
			dtos = append(dtos, toDTO(e))
		}

		a.mu.Lock()
		a.lastEntries = make(map[string]model.CacheEntry, len(entries))
		for _, e := range entries {
			a.lastEntries[e.Path] = e
		}
		a.mu.Unlock()

		a.emit("scan:done", dtos)
	}()
}

// CleanSelected 按路径清理选中的缓存项。
// 只清理上一次扫描结果中的路径（不接受任意路径，避免越权删除）。
func (a *App) CleanSelected(paths []string) CleanResult {
	a.mu.Lock()
	if a.cleaning {
		a.mu.Unlock()
		return CleanResult{}
	}
	a.cleaning = true
	// 快照待清理项后立即释放锁：删除是耗时 IO，持锁会阻塞界面其他调用。
	var chosen []model.CacheEntry
	for _, p := range paths {
		if e, ok := a.lastEntries[p]; ok {
			chosen = append(chosen, e)
		}
	}
	cfg := a.cfg
	a.mu.Unlock()

	defer func() {
		a.mu.Lock()
		a.cleaning = false
		a.mu.Unlock()
	}()

	if len(chosen) == 0 {
		return CleanResult{}
	}
	// 清理可能耗时数分钟（删除几十 GB 的缓存目录），逐项回传进度避免界面"假死"
	emitCleanProgress := func(p clean.Progress) {
		a.emit("clean:progress", map[string]interface{}{
			"done":       p.Done,
			"total":      p.Total,
			"freed":      p.Freed,
			"totalBytes": p.TotalBytes,
			"path":       p.Path,
			"pct":        p.Pct,
		})
	}
	emitCleanProgress(clean.Progress{Total: len(chosen), Path: "准备清理…"})
	applog.Info("清理开始: %d 项", len(chosen))
	freed, count, cleaned, failed, scheduled := clean.Clean(chosen, cfg, emitCleanProgress)
	applog.Info("清理结束: 成功 %d 项 / 释放 %d 字节, 失败 %d 项, 重启删除 %d 项",
		count, freed, len(failed), len(scheduled))
	for _, f := range failed {
		applog.Error("清理失败: %s", f)
	}
	result := CleanResult{Freed: freed, Count: count}
	cleanedSet := make(map[string]bool, len(cleaned))
	for _, e := range cleaned {
		cleanedSet[e.Path] = true
		result.Cleaned = append(result.Cleaned, e.Path)
	}
	for _, e := range scheduled {
		result.ScheduledReboot = append(result.ScheduledReboot, e.Path)
	}

	// 只有真正删掉的项才从扫描结果中移除：失败项与被排除项保留在列表里供重试；
	// 登记重启删除的项也移除（已处理，重启后生效）。
	a.mu.Lock()
	for _, e := range cleaned {
		delete(a.lastEntries, e.Path)
	}
	for _, e := range scheduled {
		delete(a.lastEntries, e.Path)
	}
	a.mu.Unlock()

	for _, f := range failed {
		result.Failed = append(result.Failed, splitFailure(f))
	}
	return result
}

// splitFailure 把 "路径: 错误" 拆为结构化字段（Windows 路径含盘符冒号，只按首个 ": " 切分）。
func splitFailure(msg string) CleanFailure {
	const sep = ": "
	if i := strings.Index(msg, sep); i >= 0 {
		return CleanFailure{Path: msg[:i], Error: msg[i+len(sep):]}
	}
	return CleanFailure{Path: msg, Error: msg}
}

// ── 文件占用进程（Restart Manager）──

// FindLockers 查找占用指定路径的进程（前端清理失败后调用，告诉用户"谁占着"）。
// 返回按进程名排序去重的列表；查不到或无权限时返回空。
func (a *App) FindLockers(paths []string) []LockerDTO {
	lockers := filelock.FindLockers(paths)
	out := make([]LockerDTO, 0, len(lockers))
	for _, l := range lockers {
		out = append(out, LockerDTO{Pid: l.Pid, Name: l.Name, Path: l.Path, Safe: l.Safe})
	}
	applog.Info("占用查询: %d 个路径 -> %d 个进程", len(paths), len(out))
	return out
}

// KillProcess 强制结束占用文件的进程。安全校验：
// 用 paths 重新查出占用名单，目标 pid 必须真实出现在名单中且 Safe=true
// （服务/系统关键进程由后端判定拒绝），防止前端传任意 pid 乱杀进程。
// 返回空串表示成功，否则为错误信息。
func (a *App) KillProcess(paths []string, pid uint32) string {
	lockers := filelock.FindLockers(paths)
	for _, l := range lockers {
		if l.Pid != pid {
			continue
		}
		if !l.Safe {
			return "该进程为系统关键进程，已拒绝结束"
		}
		if err := filelock.Kill(pid); err != nil {
			applog.Error("结束进程 %s(pid=%d) 失败: %v", l.Name, pid, err)
			return "结束进程失败: " + err.Error()
		}
		applog.Info("已结束占用进程 %s (pid=%d)", l.Name, pid)
		return ""
	}
	return "该进程未占用所选路径，已拒绝结束"
}

// ── 自定义目录 / 排除目录（全部在锁内读写，避免与扫描 goroutine 数据竞争）──

func (a *App) GetCustomDirs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.cfg.CustomDirs...)
}

func (a *App) GetExcludeDirs() []string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return append([]string(nil), a.cfg.ExcludeDirs...)
}

func (a *App) AddCustomDir(p string) {
	if p == "" {
		return
	}
	a.mu.Lock()
	a.cfg.CustomDirs = appendUnique(a.cfg.CustomDirs, p)
	a.saveLocked("保存自定义目录失败")
	a.mu.Unlock()
}

func (a *App) AddExcludeDir(p string) {
	if p == "" {
		return
	}
	a.mu.Lock()
	a.cfg.ExcludeDirs = appendUnique(a.cfg.ExcludeDirs, p)
	a.saveLocked("保存排除目录失败")
	a.mu.Unlock()
}

func (a *App) RemoveCustomDir(p string) {
	a.mu.Lock()
	a.cfg.CustomDirs = removeStr(a.cfg.CustomDirs, p)
	a.saveLocked("移除自定义目录失败")
	a.mu.Unlock()
}

func (a *App) RemoveExcludeDir(p string) {
	a.mu.Lock()
	a.cfg.ExcludeDirs = removeStr(a.cfg.ExcludeDirs, p)
	a.saveLocked("移除排除目录失败")
	a.mu.Unlock()
}

// saveLocked 在持锁状态下保存配置并记录错误（调用方必须已持有 mu）。
func (a *App) saveLocked(msg string) {
	if err := a.cfg.Save(); err != nil && a.ctx != nil {
		runtime.LogError(a.ctx, msg+": "+err.Error())
	}
}

func (a *App) GetHistory() []HistoryDTO {
	a.mu.Lock()
	cfg := a.cfg
	a.mu.Unlock() // 释放锁后再读文件，避免大文件读取阻塞界面

	recs := clean.LoadHistory(cfg)
	out := make([]HistoryDTO, 0, len(recs))
	for _, r := range recs {
		out = append(out, HistoryDTO{Time: r.Time, Path: r.Path, Size: r.Size})
	}
	return out
}

// ── 软件更新 ──

// UpdateInfoDTO 是传给前端的更新信息（字段与 internal/update.Info 一一对应）。
type UpdateInfoDTO struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	HasUpdate   bool   `json:"hasUpdate"`
	URL         string `json:"url"`
	Notes       string `json:"notes"`
	Size        int64  `json:"size"`
	PublishedAt string `json:"publishedAt"`
	FromCache   bool   `json:"fromCache"`
	Snoozed     bool   `json:"snoozed"`
	Skipped     bool   `json:"skipped"`
}

func toUpdateDTO(i *update.Info) UpdateInfoDTO {
	if i == nil {
		return UpdateInfoDTO{Current: update.CurrentVersion()}
	}
	return UpdateInfoDTO{
		Current:     i.Current,
		Latest:      i.Latest,
		HasUpdate:   i.HasUpdate,
		URL:         i.URL,
		Notes:       i.Notes,
		Size:        i.Size,
		PublishedAt: i.PublishedAt,
		FromCache:   i.FromCache,
		Snoozed:     i.Snoozed,
		Skipped:     i.Skipped,
	}
}

// GetVersion 返回当前程序版本，供"关于"页面展示。
func (a *App) GetVersion() string { return update.CurrentVersion() }

// CheckUpdate 检查更新。force=true 为设置页手动点击（立即联网、忽略静默/跳过）；
// force=false 为启动时的自动检查（受检查间隔与静默策略约束，失败也不打扰用户）。
func (a *App) CheckUpdate(force bool) UpdateInfoDTO {
	info, err := update.Check(force)
	if err != nil {
		applog.Error("检查更新失败: %v", err)
	} else if info != nil && info.HasUpdate {
		applog.Info("发现新版本: %s -> %s", info.Current, info.Latest)
	}
	return toUpdateDTO(info)
}

// SnoozeUpdate 推迟更新提醒 hours 小时（<=0 时按默认 24 小时）。
func (a *App) SnoozeUpdate(hours int) {
	if err := update.Snooze(time.Duration(hours) * time.Hour); err != nil {
		applog.Error("推迟更新提醒失败: %v", err)
	}
}

// SkipUpdate 跳过指定版本（空串表示跳过当前最新版本）。
func (a *App) SkipUpdate(v string) {
	if err := update.SkipVersion(v); err != nil {
		applog.Error("跳过版本失败: %v", err)
	}
}

// ClearUpdateSkip 清除跳过/静默标记，让提醒重新生效。
func (a *App) ClearUpdateSkip() {
	if err := update.ClearSkipped(); err != nil {
		applog.Error("清除跳过标记失败: %v", err)
	}
}

// OpenDownloadPage 用系统默认浏览器打开 Release 下载页。
// 不直接下载二进制：github.com 的下载域名在国内实测不可达，交给用户浏览器更可靠。
func (a *App) OpenDownloadPage(url string) {
	target := url
	if target == "" {
		target = update.DownloadPageURL()
	}
	// 只接受本项目 Release 页，避免被传入任意地址
	if !strings.HasPrefix(target, "https://github.com/") {
		target = update.DownloadPageURL()
	}
	runtime.BrowserOpenURL(a.ctx, target)
	applog.Info("已打开下载页: %s", target)
}

// ── 自动更新（后台下载 + 替换重启）──

// UpdateDownloadDTO 是传给前端的下载进度。
type UpdateDownloadDTO struct {
	State    string  `json:"state"`
	Pct      float64 `json:"pct"`
	Received int64   `json:"received"`
	Total    int64   `json:"total"`
	Message  string  `json:"message"`
}

// StartUpdateDownload 后台下载最新版本（异步，进度经 GetUpdateDownload 轮询）。
// 返回空串表示已启动，否则为错误信息。
func (a *App) StartUpdateDownload() string {
	// 先查一次拿最新信息（含真实下载 URL 与大小）
	info, err := update.Check(true)
	if err != nil {
		applog.Error("下载前检查更新失败: %v", err)
		return "检查更新失败: " + err.Error()
	}
	if !info.HasUpdate {
		return "当前已是最新版本"
	}
	if err := update.StartDownload(info); err != nil {
		applog.Error("启动下载失败: %v", err)
		return err.Error()
	}
	applog.Info("开始后台下载 %s", info.Latest)
	return ""
}

// GetUpdateDownload 返回当前下载进度。
func (a *App) GetUpdateDownload() UpdateDownloadDTO {
	d := update.GetDownloadInfo()
	return UpdateDownloadDTO{State: d.State, Pct: d.Pct, Received: d.Received, Total: d.Total, Message: d.Message}
}

// ApplyUpdateAndRestart 用已下载的新版本替换当前 exe 并重启。
// 调用成功后本进程应尽快退出（前端随即调 window.close 由 Wails 退出）。
// 返回空串表示已启动替换脚本，否则为错误信息。
func (a *App) ApplyUpdateAndRestart() string {
	if err := update.ApplyAndRestart(); err != nil {
		applog.Error("应用更新失败: %v", err)
		return err.Error()
	}
	applog.Info("更新替换脚本已启动，程序即将退出")
	return ""
}

// ── 注册表清理 ──

// ScanRegistry 扫描注册表垃圾项（同步调用，注册表扫描耗时在毫秒级）。
func (a *App) ScanRegistry() []RegEntryDTO {
	entries := regclean.Scan()
	sort.Slice(entries, func(i, j int) bool { return entries[i].Size > entries[j].Size })

	a.mu.Lock()
	a.lastRegEntries = make(map[string]regclean.Entry, len(entries))
	for _, e := range entries {
		a.lastRegEntries[e.Key] = e
	}
	a.mu.Unlock()

	dtos := make([]RegEntryDTO, 0, len(entries))
	for _, e := range entries {
		dtos = append(dtos, toRegDTO(e))
	}
	return dtos
}

// CleanRegistry 清理选中的注册表项。只接受上一次扫描结果中的键（白名单 + 扫描结果双重校验）。
func (a *App) CleanRegistry(keys []string) CleanResult {
	a.mu.Lock()
	var chosen []regclean.Entry
	for _, k := range keys {
		if e, ok := a.lastRegEntries[k]; ok {
			chosen = append(chosen, e)
		}
	}
	cfg := a.cfg
	a.mu.Unlock()

	if len(chosen) == 0 {
		return CleanResult{}
	}
	names := make([]string, 0, len(chosen))
	for _, e := range chosen {
		names = append(names, e.Key)
	}
	// 注册表清理很快，但仍回传首尾两个进度事件，让界面进度条表现一致（起 → 满）
	a.emit("clean:progress", map[string]interface{}{
		"done": 0, "total": len(names), "freed": int64(0), "totalBytes": int64(0), "path": "准备清理…", "pct": float64(0),
	})
	cleaned, failed := regclean.Clean(names)
	a.emit("clean:progress", map[string]interface{}{
		"done": len(names), "total": len(names), "freed": int64(0), "totalBytes": int64(0), "path": "清理完成", "pct": float64(100),
	})

	var result CleanResult
	now := time.Now().Format("2006-01-02 15:04:05")
	recs := make([]clean.HistoryRecord, 0, len(cleaned))
	for _, e := range cleaned {
		result.Freed += e.Size
		result.Count++
		result.Cleaned = append(result.Cleaned, e.Key)
		recs = append(recs, clean.HistoryRecord{Time: now, Path: e.Key, Size: e.Size})
	}
	for _, f := range failed {
		result.Failed = append(result.Failed, splitFailure(f))
	}

	a.mu.Lock()
	for _, e := range cleaned {
		delete(a.lastRegEntries, e.Key)
	}
	a.mu.Unlock()

	clean.AppendHistory(cfg, recs)
	return result
}

// ── 组件存储清理（WinSxS / DISM，需提权）──

// GetDismInfo 返回系统工具页初始化信息（可用性 / 提权状态 / 最近分析报告）。
func (a *App) GetDismInfo() DismInfoDTO {
	return DismInfoDTO{
		Available: dismclean.Available(),
		Elevated:  dismclean.IsElevated(),
		Report:    toDismReportDTO(dismclean.AnalyzeReport()),
	}

}

func toDismReportDTO(r *dismclean.Report) *DismReportDTO {
	if r == nil {
		return nil
	}
	return &DismReportDTO{
		ReportedSize:    r.ReportedSize,
		ActualSize:      r.ActualSize,
		ReclaimablePkgs: r.ReclaimablePkgs,
		LastCleanup:     r.LastCleanup,
		Recommended:     r.Recommended,
		Raw:             r.Raw,
	}
}

// AnalyzeComponentStore 启动组件存储分析（非管理员进程会弹 UAC 授权框）。
// 返回空串表示已启动（进度经 dism:progress 事件推送），否则为错误信息。
func (a *App) AnalyzeComponentStore() string {
	applog.Info("DISM: 请求启动组件存储分析 (elevated=%v)", dismclean.IsElevated())
	if err := dismclean.StartAnalyze(); err != nil {
		applog.Error("DISM: 分析启动失败: %v", err)
		return err.Error()
	}
	a.watchDismJob("analyze")
	return ""
}

// StartComponentCleanup 启动组件存储清理（不带 /ResetBase，清理后仍可卸载已装更新）。
// 返回空串表示已启动（进度经 dism:progress 事件推送），否则为错误信息。
func (a *App) StartComponentCleanup() string {
	applog.Info("DISM: 请求启动组件存储清理 (elevated=%v)", dismclean.IsElevated())
	if err := dismclean.StartCleanup(); err != nil {
		applog.Error("DISM: 清理启动失败: %v", err)
		return err.Error()
	}
	a.watchDismJob("cleanup")
	return ""
}

// StartComponentRepair 启动组件存储修复（RestoreHealth）。
// 针对"清理反复报 0x80070005 / STATUS_CANNOT_DELETE"的组件存储不一致场景。
// 返回空串表示已启动（进度经 dism:progress 事件推送），否则为错误信息。
func (a *App) StartComponentRepair() string {
	applog.Info("DISM: 请求启动组件存储修复 (elevated=%v)", dismclean.IsElevated())
	if err := dismclean.StartRepair(); err != nil {
		applog.Error("DISM: 修复启动失败: %v", err)
		return err.Error()
	}
	a.watchDismJob("repair")
	return ""
}

// LogFrontend 接收前端 JS 错误（window.onerror / unhandledrejection）并落盘，
// 前端报错从此不再不可见。
func (a *App) LogFrontend(msg string) {
	applog.Error("前端: %s", msg)
}

// watchDismJob 后台轮询提权作业进度并广播 dism:progress 事件，结束时附带分析报告。
func (a *App) watchDismJob(kind string) {
	a.emit("dism:progress", map[string]interface{}{
		"kind": kind, "running": true, "pct": float64(0), "line": "等待提权授权…", "done": false,
	})
	go func() {
		ticker := time.NewTicker(700 * time.Millisecond)
		defer ticker.Stop()
		for range ticker.C {
			st := dismclean.Poll()
			if !st.Running && !st.Done {
				continue // 作业刚启动，输出文件尚未产生
			}
			payload := map[string]interface{}{
				"kind": kind, "running": st.Running, "pct": st.Pct, "line": st.Line,
				"done": st.Done, "ok": st.OK, "message": st.Message,
			}
			if st.Done {
				if kind == "analyze" && st.OK {
					payload["report"] = toDismReportDTO(dismclean.AnalyzeReport())
				}
				if st.OK {
					applog.Info("DISM: %s 作业完成", kind)
				} else {
					applog.Error("DISM: %s 作业失败: %s", kind, st.Message)
				}
				a.emit("dism:progress", payload)
				return
			}
			a.emit("dism:progress", payload)
		}
	}()
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func removeStr(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}
