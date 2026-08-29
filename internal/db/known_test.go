package db

import (
	"os"
	"path/filepath"
	"sort"
	"testing"
)

// expandPath：固定段必须存在，"*" 只匹配一层任意目录，不存在的段返回空。
func TestExpandPath(t *testing.T) {
	root := t.TempDir()
	mk := func(rel string) {
		if err := os.MkdirAll(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	mk("profiles/game/Cache")
	mk("profiles/multitab/Cache")
	mk("profiles/web_shell/Cache")
	mk("profiles/game/GPUCache")
	mk("other/x/Cache")
	// 文件不应被 "*" 当作目录匹配
	if err := os.WriteFile(filepath.Join(root, "profiles", "afile.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got := expandPath(root, "profiles/*/Cache")
	if len(got) != 3 {
		t.Fatalf("profiles/*/Cache 应命中 3 个目录，实际 %d: %v", len(got), got)
	}
	sort.Strings(got)
	for _, p := range got {
		if filepath.Base(p) != "Cache" {
			t.Errorf("展开的应是 Cache 目录: %s", p)
		}
	}

	// 多段通配
	got2 := expandPath(root, "*/*/Cache")
	if len(got2) != 4 { // profiles 下 3 个 + other/x 下 1 个
		t.Fatalf("*/*/Cache 应命中 4 个目录，实际 %d: %v", len(got2), got2)
	}

	// 不存在的固定段
	if got3 := expandPath(root, "nope/*/Cache"); len(got3) != 0 {
		t.Errorf("不存在的段应返回空: %v", got3)
	}
	// "*" 不匹配文件
	for _, p := range expandPath(root, "profiles/*/Cache") {
		if filepath.Base(filepath.Dir(p)) == "afile.txt" {
			t.Errorf("通配符不应匹配文件: %s", p)
		}
	}
	// 无通配符时等价于原来的 Stat 判断
	got4 := expandPath(root, "profiles/game/GPUCache")
	if len(got4) != 1 || filepath.Base(got4[0]) != "GPUCache" {
		t.Errorf("固定路径展开不正确: %v", got4)
	}
}
