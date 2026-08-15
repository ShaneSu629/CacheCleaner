package main

import (
	"context"
	"embed"
	"sort"
	"sync"

	"cachecleaner/internal/clean"
	"cachecleaner/internal/config"
	"cachecleaner/internal/model"
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

// CleanResult 是清理结果。
type CleanResult struct {
	Freed  int64    `json:"freed"`
	Count  int64    `json:"count"`
	Failed []string `json:"failed"`
}

// App 是绑定到前端（window.go.main.App）的结构，承载所有 UI 操作。
// 业务逻辑全部复用内部包，这里只做编排与序列化。
type App struct {
	ctx         context.Context
	cfg         *config.Config
	mu          sync.Mutex
	lastEntries map[string]model.CacheEntry
}

func NewApp() *App { return &App{} }

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	cfg, err := config.Load()
	if err != nil {
		runtime.LogError(a.ctx, "配置加载失败: "+err.Error())
		return
	}
	a.cfg = cfg
	a.lastEntries = map[string]model.CacheEntry{}
}

func (a *App) domReady(ctx context.Context) {}

// 禁止关闭前拦截：直接允许退出。
func (a *App) beforeClose(ctx context.Context) bool { return false }

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

// Scan 在后台执行扫描，过程通过 "scan:progress" 实时回传进度，结果通过 "scan:done" 回传（避免界面假死）。
func (a *App) Scan(mode string) {
	go func() {
		progress := func(phase string, done, total, step, stepTotal int) {
			var pct float64
			if total > 0 {
				pct = (float64(step-1) + float64(done)/float64(total)) / float64(stepTotal) * 100
			} else {
				pct = float64(step-1) / float64(stepTotal) * 100
			}
			runtime.EventsEmit(a.ctx, "scan:progress", map[string]interface{}{
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
			entries = scan.DeepAIScan(a.cfg, progress)
		default:
			entries = scan.SmartScan(a.cfg, progress)
		}
		sort.Slice(entries, func(i, j int) bool { return entries[i].Size > entries[j].Size })

		a.mu.Lock()
		a.lastEntries = make(map[string]model.CacheEntry, len(entries))
		dtos := make([]EntryDTO, 0, len(entries))
		for _, e := range entries {
			a.lastEntries[e.Path] = e
			dtos = append(dtos, toDTO(e))
		}
		a.mu.Unlock()

		runtime.EventsEmit(a.ctx, "scan:done", dtos)
	}()
}

// CleanSelected 按路径清理选中的缓存项。
func (a *App) CleanSelected(paths []string) CleanResult {
	a.mu.Lock()
	defer a.mu.Unlock()

	var chosen []model.CacheEntry
	for _, p := range paths {
		if e, ok := a.lastEntries[p]; ok {
			chosen = append(chosen, e)
		}
	}
	if len(chosen) == 0 {
		return CleanResult{}
	}
	freed, count, failed := clean.Clean(chosen, a.cfg)
	for _, e := range chosen {
		delete(a.lastEntries, e.Path)
	}
	return CleanResult{Freed: freed, Count: count, Failed: failed}
}

func (a *App) GetCustomDirs() []string  { return a.cfg.CustomDirs }
func (a *App) GetExcludeDirs() []string { return a.cfg.ExcludeDirs }

func (a *App) AddCustomDir(p string) {
	if p == "" {
		return
	}
	a.cfg.CustomDirs = appendUnique(a.cfg.CustomDirs, p)
	_ = a.cfg.Save()
}

func (a *App) AddExcludeDir(p string) {
	if p == "" {
		return
	}
	a.cfg.ExcludeDirs = appendUnique(a.cfg.ExcludeDirs, p)
	_ = a.cfg.Save()
}

func (a *App) RemoveCustomDir(p string) {
	a.cfg.CustomDirs = removeStr(a.cfg.CustomDirs, p)
	_ = a.cfg.Save()
}

func (a *App) RemoveExcludeDir(p string) {
	a.cfg.ExcludeDirs = removeStr(a.cfg.ExcludeDirs, p)
	_ = a.cfg.Save()
}

func (a *App) GetHistory() []HistoryDTO {
	recs := clean.LoadHistory(a.cfg)
	out := make([]HistoryDTO, 0, len(recs))
	for _, r := range recs {
		out = append(out, HistoryDTO{Time: r.Time, Path: r.Path, Size: r.Size})
	}
	return out
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

