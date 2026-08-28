package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"cachecleaner/internal/config"
	"cachecleaner/internal/model"
)

// TestAppConcurrentConfigAccess 验证配置读写并发安全。
// 修复前：AddCustomDir/RemoveCustomDir/GetCustomDirs 完全不加锁，而扫描 goroutine
// 正在并发读 cfg，属于典型数据竞争（go test -race 可复现）。
func TestAppConcurrentConfigAccess(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{
		Home:        dir,
		ConfigFile:  filepath.Join(dir, "config.json"),
		CustomDirs:  []string{},
		ExcludeDirs: []string{},
	}
	a := &App{cfg: cfg, lastEntries: map[string]model.CacheEntry{}}

	// 准备一个真实可删的缓存目录
	cacheDir := filepath.Join(dir, "Cache")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, "a.bin"), make([]byte, 512), 0o644); err != nil {
		t.Fatal(err)
	}
	a.lastEntries[cacheDir] = model.CacheEntry{Path: cacheDir, ShortPath: "Cache", Size: 512}

	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			a.AddCustomDir(fmt.Sprintf("dir-%d", i))
		}(i)
	}
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = a.GetCustomDirs()
			_ = a.GetExcludeDirs()
			_ = a.GetHistory()
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		a.CleanSelected([]string{cacheDir})
	}()
	wg.Wait()

	if got := len(a.GetCustomDirs()); got != 16 {
		t.Errorf("并发写入后应有 16 个自定义目录，实际 %d", got)
	}
	if _, err := os.Stat(cacheDir); !os.IsNotExist(err) {
		t.Errorf("缓存目录应已被清理: %v", err)
	}
}

// TestCleanSelectedKeepsFailedEntries 验证失败项不会被误当作已清理项从结果中移除。
func TestCleanSelectedKeepsFailedEntries(t *testing.T) {
	dir := t.TempDir()
	cfg := &config.Config{Home: dir, ConfigFile: filepath.Join(dir, "config.json")}
	a := &App{cfg: cfg, lastEntries: map[string]model.CacheEntry{}}

	// 一个可删、一个因被排除而跳过的项
	okDir := filepath.Join(dir, "ok")
	skipDir := filepath.Join(dir, "skip")
	if err := os.MkdirAll(okDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(skipDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg.ExcludeDirs = []string{"skip"}
	a.lastEntries[okDir] = model.CacheEntry{Path: okDir, ShortPath: "ok", Size: 1}
	a.lastEntries[skipDir] = model.CacheEntry{Path: skipDir, ShortPath: "skip", Size: 2}

	res := a.CleanSelected([]string{okDir, skipDir})
	if res.Count != 1 {
		t.Fatalf("应只清理 1 项，实际 %d", res.Count)
	}
	if len(res.Cleaned) != 1 || res.Cleaned[0] != okDir {
		t.Fatalf("Cleaned 应只含成功项: %+v", res.Cleaned)
	}
	if _, ok := a.lastEntries[skipDir]; !ok {
		t.Errorf("被跳过的项应仍保留在扫描结果中，供用户重试")
	}
}
