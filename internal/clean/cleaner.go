package clean

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	"cachecleaner/internal/config"
	"cachecleaner/internal/model"
)

// HistoryRecord 是一条清理历史记录。
type HistoryRecord struct {
	Time string `json:"time"`
	Path string `json:"path"`
	Size int64  `json:"size"`
}

// Progress 是清理过程中的进度快照。
type Progress struct {
	Done       int     // 已处理的条目数（含跳过与失败的项）
	Total      int     // 待处理条目总数
	Freed      int64   // 已释放字节数
	TotalBytes int64   // 待清理总字节数（扫描时统计的大小之和）
	Path       string  // 当前正在处理的路径
	Pct        float64 // 百分比：优先按字节数，无大小数据时按条目数
}

// ProgressFunc 是可选的清理进度回调，供 UI 实时展示进度（删除大目录时耗时可达分钟级）。
type ProgressFunc func(Progress)

func report(p ProgressFunc, pr Progress) {
	// 优先按字节算百分比（更符合"释放了多少"的直觉），没有大小数据时退化为按条目数
	switch {
	case pr.TotalBytes > 0:
		pr.Pct = float64(pr.Freed) / float64(pr.TotalBytes) * 100
	case pr.Total > 0:
		pr.Pct = float64(pr.Done) / float64(pr.Total) * 100
	}
	if p != nil {
		p(pr)
	}
}

// Clean 执行清理：跳过排除目录，尝试删除。
// 返回值：释放字节数、成功项数、实际被清理的条目（供调用方更新界面与历史）、失败列表（"路径: 错误"）。
// 释放字节数取扫描时统计的 e.Size（与界面展示一致；清理前不再重复遍历大目录，清理耗时约减半）。
// progress 可选，每处理完一项回调一次；传 nil 表示不需要进度（如 CLI 静默模式）。
func Clean(entries []model.CacheEntry, cfg *config.Config, progress ProgressFunc) (freed int64, count int64, cleaned []model.CacheEntry, failed []string) {
	var totalBytes int64
	for _, e := range entries {
		totalBytes += e.Size
	}
	for i, e := range entries {
		// 先报"正在处理"，再执行删除：删除单个大目录可能耗时很久，用户需要知道当前卡在哪一项
		report(progress, Progress{Done: i, Total: len(entries), Freed: freed, TotalBytes: totalBytes, Path: e.Path})

		if cfg.IsExcluded(e.Path) {
			continue
		}
		if err := os.RemoveAll(e.Path); err != nil {
			msg := err.Error()
			// Windows 上"Access is denied"多半是文件正被运行中的程序占用（共享冲突），
			// 把提示翻译成用户能懂的指引，而不是原样抛英文错误码。
			if strings.Contains(msg, "Access is denied") || strings.Contains(msg, "denied") {
				msg += "（文件可能正被运行中的程序占用，退出对应软件后重试）"
			}
			failed = append(failed, e.Path+": "+msg)
			continue
		}
		freed += e.Size
		count++
		cleaned = append(cleaned, e)
	}
	report(progress, Progress{Done: len(entries), Total: len(entries), Freed: freed, TotalBytes: totalBytes})
	recs := make([]HistoryRecord, 0, len(cleaned))
	now := time.Now().Format("2006-01-02 15:04:05")
	for _, e := range cleaned {
		recs = append(recs, HistoryRecord{Time: now, Path: e.ShortPath, Size: e.Size})
	}
	// 历史一次性批量写入（此前逐条读写整个文件，N 项清理产生 N 次全量 IO）
	appendHistory(cfg, recs)
	return
}

// AppendHistory 供外部（如注册表清理）批量追加清理历史，复用同一套原子写逻辑。
func AppendHistory(cfg *config.Config, recs []HistoryRecord) {
	appendHistory(cfg, recs)
}

func historyPath(cfg *config.Config) string {
	if cfg.ConfigFile == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(cfg.ConfigFile), "history.json")
}

func appendHistory(cfg *config.Config, recs []HistoryRecord) {
	if len(recs) == 0 {
		return
	}
	histPath := historyPath(cfg)
	if histPath == "" {
		return
	}
	var existing []HistoryRecord
	if data, err := os.ReadFile(histPath); err == nil {
		_ = json.Unmarshal(data, &existing)
	}
	recs = append(existing, recs...)
	if len(recs) > 200 {
		recs = recs[len(recs)-200:]
	}
	data, err := json.MarshalIndent(recs, "", "  ")
	if err != nil {
		return
	}
	// 原子写：先写临时文件再改名，避免并发读取（历史页面刷新）读到半个文件
	dir := filepath.Dir(histPath)
	_ = os.MkdirAll(dir, 0o755)
	tmp := histPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return
	}
	_ = os.Rename(tmp, histPath)
}

// LoadHistory 读取清理历史。
func LoadHistory(cfg *config.Config) []HistoryRecord {
	histPath := historyPath(cfg)
	if histPath == "" {
		return nil
	}
	data, err := os.ReadFile(histPath)
	if err != nil {
		return nil
	}
	var recs []HistoryRecord
	_ = json.Unmarshal(data, &recs)
	return recs
}
