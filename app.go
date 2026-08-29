package main

import (
	"context"
	"embed"
	"sort"
	"strings"
	"sync"
	"time"

	"cachecleaner/internal/clean"
	"cachecleaner/internal/config"
	"cachecleaner/internal/dismclean"
	"cachecleaner/internal/model"
	"cachecleaner/internal/regclean"
	"cachecleaner/internal/scan"
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

// CleanResult 是清理结果。Cleaned 为成功清理的路径，前端据此移除列表项；
// Failed 保留失败明细，失败项仍留在列表中供用户重试。
type CleanResult struct {
	Freed   int64          `json:"freed"`
	Count   int64          `json:"count"`
	Cleaned []string       `json:"cleaned"`
	Failed  []CleanFailure `json:"failed"`
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
		cfg = config.Default()
	}
	a.mu.Lock()
	a.cfg = cfg
	a.lastEntries = map[string]model.CacheEntry{}
	a.lastRegEntries = map[string]regclean.Entry{}
	a.mu.Unlock()
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

	go func() {
		defer func() {
			a.mu.Lock()
			a.scanning = false
			a.mu.Unlock()
		}()

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
	freed, count, cleaned, failed := clean.Clean(chosen, cfg, emitCleanProgress)
	result := CleanResult{Freed: freed, Count: count}
	cleanedSet := make(map[string]bool, len(cleaned))
	for _, e := range cleaned {
		cleanedSet[e.Path] = true
		result.Cleaned = append(result.Cleaned, e.Path)
	}

	// 只有真正删掉的项才从扫描结果中移除：失败项与被排除项保留在列表里供重试
	a.mu.Lock()
	for _, e := range cleaned {
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
	if err := dismclean.StartAnalyze(); err != nil {
		return err.Error()
	}
	a.watchDismJob("analyze")
	return ""
}

// StartComponentCleanup 启动组件存储清理（不带 /ResetBase，清理后仍可卸载已装更新）。
// 返回空串表示已启动（进度经 dism:progress 事件推送），否则为错误信息。
func (a *App) StartComponentCleanup() string {
	if err := dismclean.StartCleanup(); err != nil {
		return err.Error()
	}
	a.watchDismJob("cleanup")
	return ""
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
