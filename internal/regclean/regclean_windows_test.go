//go:build windows

package regclean

import (
	"strings"
	"testing"

	"cachecleaner/internal/model"

	"golang.org/x/sys/windows/registry"
)

// 白名单键定义必须完整：所有 def 的键路径非空、风险等级合法。
func TestDefsValid(t *testing.T) {
	if len(defs) == 0 {
		t.Fatal("defs 不应为空")
	}
	seen := map[string]bool{}
	for _, d := range defs {
		if d.sub == "" {
			t.Errorf("存在空键路径的 def: %+v", d)
		}
		if seen[d.sub] {
			t.Errorf("重复的键路径: %s", d.sub)
		}
		seen[d.sub] = true
		switch d.risk {
		case model.RiskSafe, model.RiskCaution, model.RiskReview:
		default:
			t.Errorf("非法风险等级 %q: %s", d.risk, d.sub)
		}
	}
}

// Scan 返回的每项都必须来自白名单且带 HKCU 前缀。
func TestScanWhitelist(t *testing.T) {
	allowed := map[string]bool{}
	for _, d := range defs {
		allowed[HivePrefix+d.sub] = true
	}
	for _, e := range Scan() {
		if !allowed[e.Key] {
			t.Errorf("Scan 返回了非白名单键: %s", e.Key)
		}
		if !strings.HasPrefix(e.Key, HivePrefix) {
			t.Errorf("键缺少 HKCU 前缀: %s", e.Key)
		}
	}
}

// TestCleanRemovesValues 在 HKCU\Software 下自建临时键验证真实删除路径：
// 写入值 → 临时加入白名单 → 清理 → 断言值已清空 → 删除临时键。
// 不触碰用户真实的 MRU 数据。
func TestCleanRemovesValues(t *testing.T) {
	const testSub = `Software\CacheCleanerTest`

	k, _, err := registry.CreateKey(registry.CURRENT_USER, testSub, registry.SET_VALUE)
	if err != nil {
		t.Fatalf("创建临时键失败: %v", err)
	}
	if err := k.SetStringValue("mrutest", "hello"); err != nil {
		k.Close()
		t.Fatalf("写入测试值失败: %v", err)
	}
	k.Close()
	defer func() { _ = registry.DeleteKey(registry.CURRENT_USER, testSub) }()

	// 仅本次测试把临时键纳入白名单
	origDefs := defs
	defs = append(defs, def{sub: testSub, category: "测试", risk: model.RiskSafe, desc: "单元测试临时键"})
	defer func() { defs = origDefs }()

	cleaned, failed := Clean([]string{HivePrefix + testSub})
	if len(failed) != 0 {
		t.Fatalf("清理失败: %v", failed)
	}
	if len(cleaned) != 1 {
		t.Fatalf("应清理 1 项，实际 %d", len(cleaned))
	}

	verify, err := registry.OpenKey(registry.CURRENT_USER, testSub, registry.READ)
	if err != nil {
		t.Fatalf("重新打开临时键失败: %v", err)
	}
	defer verify.Close()
	names, err := verify.ReadValueNames(-1)
	if err != nil {
		t.Fatalf("读取值列表失败: %v", err)
	}
	if len(names) != 0 {
		t.Errorf("键值应已被清空，实际剩余: %v", names)
	}
}

// Clean 必须拒绝白名单之外的键（防止任意注册表删除）。
func TestCleanRejectsUnknownKeys(t *testing.T) {
	evil := []string{
		HivePrefix + `Software\Microsoft\Windows\CurrentVersion\Run`, // 开机自启动项，绝不能删
		`HKLM\Software\Microsoft\Windows\CurrentVersion\Run`,
	}
	cleaned, failed := Clean(evil)
	if len(cleaned) != 0 {
		t.Errorf("白名单外的键不应被清理，实际清理了 %d 项", len(cleaned))
	}
	if len(failed) != len(evil) {
		t.Errorf("白名单外的键应全部失败，want %d, got %d", len(evil), len(failed))
	}
}
