package clean

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"cachecleaner/internal/config"
	"cachecleaner/internal/model"
)

func newTestCfg(t *testing.T) (*config.Config, string) {
	t.Helper()
	dir := t.TempDir()
	return &config.Config{Home: dir, ConfigFile: filepath.Join(dir, "cfg", "config.json")}, dir
}

func TestCleanRemovesAndRecordsHistoryOnce(t *testing.T) {
	cfg, dir := newTestCfg(t)
	cacheDir := filepath.Join(dir, "Cache")
	if err := os.MkdirAll(filepath.Join(cacheDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "sub", "a.bin"), make([]byte, 1024), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []model.CacheEntry{{Path: cacheDir, ShortPath: "Cache", Size: 1024}}

	var got []Progress
	freed, count, cleaned, failed, _ := Clean(entries, cfg, func(p Progress) { got = append(got, p) })
	if len(failed) != 0 {
		t.Fatalf("不应有失败项: %v", failed)
	}
	// 进度回调：最后一次应为 done == total 且 100%
	if len(got) == 0 {
		t.Fatal("应至少回调一次进度")
	}
	last := got[len(got)-1]
	if last.Done != last.Total || last.Total != 1 {
		t.Errorf("末次进度应 done==total==1，实际 done=%d total=%d", last.Done, last.Total)
	}
	if last.Pct != 100 {
		t.Errorf("末次进度应为 100%%，实际 %.1f", last.Pct)
	}
	if count != 1 || freed != 1024 || len(cleaned) != 1 {
		t.Fatalf("清理结果不符: freed=%d count=%d cleaned=%d", freed, count, len(cleaned))
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("目录应已被删除: %v", err)
	}

	// 历史应恰好写入 1 条（修复前每条记录都会全量重写一次文件）
	recs := LoadHistory(cfg)
	if len(recs) != 1 {
		t.Fatalf("历史应只有 1 条，实际 %d: %+v", len(recs), recs)
	}
	if recs[0].Size != 1024 || recs[0].Time == "" {
		t.Errorf("历史记录内容不对: %+v", recs[0])
	}

	// 历史文件本身必须是合法 JSON（原子写：临时文件 + 改名）
	data, err := os.ReadFile(filepath.Join(filepath.Dir(cfg.ConfigFile), "history.json"))
	if err != nil {
		t.Fatalf("读取历史文件失败: %v", err)
	}
	var again []HistoryRecord
	if err := json.Unmarshal(data, &again); err != nil {
		t.Fatalf("历史文件不是合法 JSON: %v", err)
	}
}

func TestCleanSkipsExcluded(t *testing.T) {
	cfg, dir := newTestCfg(t)
	cacheDir := filepath.Join(dir, "logs")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// 排除项 "logs" 按段匹配命中；"og" 这类子串不应命中
	cfg.ExcludeDirs = []string{"logs"}
	entries := []model.CacheEntry{{Path: cacheDir, ShortPath: "logs", Size: 10}}

	freed, count, cleaned, failed, _ := Clean(entries, cfg, nil)
	if count != 0 || freed != 0 || len(cleaned) != 0 || len(failed) != 0 {
		t.Fatalf("被排除项应静默跳过: freed=%d count=%d cleaned=%d failed=%v", freed, count, len(cleaned), failed)
	}
	if _, err := os.Stat(cacheDir); err != nil {
		t.Errorf("被排除的目录不应被删除: %v", err)
	}
	if recs := LoadHistory(cfg); len(recs) != 0 {
		t.Errorf("被排除项不应写入历史: %+v", recs)
	}
}
