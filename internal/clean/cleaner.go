package clean

import (
	"encoding/json"
	"os"
	"path/filepath"
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

// Clean 执行清理：跳过排除目录，尝试删除，返回释放字节数、成功项数、失败列表。
func Clean(entries []model.CacheEntry, cfg *config.Config) (freed int64, count int64, failed []string) {
	for _, e := range entries {
		if isExcludedName(cfg, e.Path) {
			continue
		}
		pre := dirSize(e.Path)
		if err := os.RemoveAll(e.Path); err != nil {
			failed = append(failed, e.Path+": "+err.Error())
			continue
		}
		freed += pre
		count++
		appendHistory(cfg, e)
	}
	return
}

func isExcludedName(cfg *config.Config, path string) bool {
	base := filepath.Base(path)
	for _, ex := range cfg.ExcludeDirs {
		if ex != "" && (base == ex || filepath.Base(filepath.Dir(path)) == ex) {
			return true
		}
	}
	return false
}

func dirSize(path string) int64 {
	var s int64
	_ = filepath.Walk(path, func(_ string, fi os.FileInfo, err error) error {
		if err == nil && fi != nil && !fi.IsDir() {
			s += fi.Size()
		}
		return nil
	})
	return s
}

func appendHistory(cfg *config.Config, e model.CacheEntry) {
	if cfg.ConfigFile == "" {
		return
	}
	dir := filepath.Dir(cfg.ConfigFile)
	histPath := filepath.Join(dir, "history.json")
	var recs []HistoryRecord
	if data, err := os.ReadFile(histPath); err == nil {
		_ = json.Unmarshal(data, &recs)
	}
	recs = append(recs, HistoryRecord{
		Time: time.Now().Format("2006-01-02 15:04:05"),
		Path: e.ShortPath,
		Size: e.Size,
	})
	if len(recs) > 200 {
		recs = recs[len(recs)-200:]
	}
	data, _ := json.MarshalIndent(recs, "", "  ")
	_ = os.MkdirAll(dir, 0o755)
	_ = os.WriteFile(histPath, data, 0o644)
}

// LoadHistory 读取清理历史。
func LoadHistory(cfg *config.Config) []HistoryRecord {
	if cfg.ConfigFile == "" {
		return nil
	}
	histPath := filepath.Join(filepath.Dir(cfg.ConfigFile), "history.json")
	data, err := os.ReadFile(histPath)
	if err != nil {
		return nil
	}
	var recs []HistoryRecord
	_ = json.Unmarshal(data, &recs)
	return recs
}
