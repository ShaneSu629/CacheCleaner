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

// Clean 执行清理：跳过排除目录，尝试删除。
// 返回值：释放字节数、成功项数、实际被清理的条目（供调用方更新界面与历史）、失败列表（"路径: 错误"）。
// 释放字节数取扫描时统计的 e.Size（与界面展示一致；清理前不再重复遍历大目录，清理耗时约减半）。
func Clean(entries []model.CacheEntry, cfg *config.Config) (freed int64, count int64, cleaned []model.CacheEntry, failed []string) {
	for _, e := range entries {
		if cfg.IsExcluded(e.Path) {
			continue
		}
		if err := os.RemoveAll(e.Path); err != nil {
			failed = append(failed, e.Path+": "+err.Error())
			continue
		}
		freed += e.Size
		count++
		cleaned = append(cleaned, e)
	}
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
