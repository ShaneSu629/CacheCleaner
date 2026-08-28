package scan

import (
	"os"
	"path/filepath"
	"testing"

	"cachecleaner/internal/config"
	"cachecleaner/internal/model"
)

func TestDedupeParents(t *testing.T) {
	root := string(os.PathSeparator)
	entries := []model.CacheEntry{
		{Path: root + "a" + root + "App" + root + "Cache" + root + "Cache_Data", Size: 100},
		{Path: root + "a" + root + "App" + root + "Cache", Size: 100},
		{Path: root + "b" + root + "Other" + root + "GPUCache", Size: 50},
	}
	got := dedupeParents(entries)
	if len(got) != 2 {
		t.Fatalf("应保留 2 项（祖先合并后），实际 %d: %+v", len(got), got)
	}
	for _, g := range got {
		if filepath.Base(g.Path) == "Cache_Data" {
			t.Errorf("子孙目录 Cache_Data 应被祖先 Cache 覆盖: %s", g.Path)
		}
	}
}

func TestDedupePrefersKnown(t *testing.T) {
	root := string(os.PathSeparator)
	p := root + "app" + root + "Cache"
	entries := []model.CacheEntry{
		{Path: p, Category: "Electron缓存(app)", Risk: model.RiskSafe},
		{Path: p, Category: "AI缓存", Risk: model.RiskCaution},
	}
	got := dedupe(entries)
	if len(got) != 1 {
		t.Fatalf("同路径应合并为 1 项，实际 %d", len(got))
	}
	if got[0].Category != "AI缓存" {
		t.Errorf("已知项应优先保留，实际: %s", got[0].Category)
	}
}

// FindCustomCaches 修复前：用户添加的自定义目录从未被扫描（死功能）。
func TestFindCustomCaches(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, "mycache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "a.bin"), make([]byte, 2048), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := &config.Config{Home: dir, CustomDirs: []string{cacheDir, filepath.Join(dir, "not-exist")}}

	got := FindCustomCaches(cfg, nil)
	if len(got) != 1 {
		t.Fatalf("只应返回存在的自定义目录，实际 %d 项: %+v", len(got), got)
	}
	if got[0].Path != cacheDir || got[0].Size != 2048 || got[0].Category != "自定义目录" {
		t.Errorf("自定义目录项不正确: %+v", got[0])
	}

	// 被排除的自定义目录不应返回
	cfg.ExcludeDirs = []string{filepath.Base(cacheDir)}
	if got := FindCustomCaches(cfg, nil); len(got) != 0 {
		t.Errorf("被排除的自定义目录不应返回，实际: %+v", got)
	}
}
