package update

import (
	"testing"
	"time"
)

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"v1.0.7", "v1.0.7", 0},
		{"1.0.7", "v1.0.7", 0},
		{"v1.0.8", "v1.0.7", 1},
		{"v1.0.7", "v1.0.8", -1},
		{"v1.1.0", "v1.0.9", 1},
		{"v2.0.0", "v1.9.9", 1},
		{"v0.9.0", "v1.0.0", -1},
		// 预发布版本低于正式版
		{"v1.0.7-beta", "v1.0.7", -1},
		{"v1.0.7", "v1.0.7-beta", 1},
		// 缺段按 0 处理
		{"v1.0", "v1.0.0", 0},
		{"v1", "v1.0.0", 0},
		// 异常输入不 panic，按 0 处理
		{"v1.0.x", "v1.0.0", 0},
		{"abc", "v1.0.0", -1},
		{"", "v1.0.0", -1},
	}
	for _, c := range cases {
		if got := Compare(c.a, c.b); got != c.want {
			t.Errorf("Compare(%q,%q)=%d want %d", c.a, c.b, got, c.want)
		}
	}
}

func TestParse(t *testing.T) {
	nums, pre := parse("v1.2.3-beta")
	if nums != [3]int{1, 2, 3} || pre != "beta" {
		t.Errorf("parse(v1.2.3-beta)=%v,%q", nums, pre)
	}
	nums, pre = parse("2.0")
	if nums != [3]int{2, 0, 0} || pre != "" {
		t.Errorf("parse(2.0)=%v,%q", nums, pre)
	}
}

func TestApplyLocalState(t *testing.T) {
	st := State{}
	info := &Info{Current: "v1.0.0", Latest: "v1.0.7"}
	applyLocalState(info, st)
	if !info.HasUpdate {
		t.Error("新版本应判定为有更新")
	}
	if info.Snoozed || info.Skipped {
		t.Error("默认状态不应是静默或跳过")
	}

	// 无更新
	info2 := &Info{Current: "v1.0.7", Latest: "v1.0.7"}
	applyLocalState(info2, st)
	if info2.HasUpdate {
		t.Error("同版本不应判定为有更新")
	}

	// 静默期内
	st.SnoozeUntil = time.Now().Add(time.Hour)
	info3 := &Info{Current: "v1.0.0", Latest: "v1.0.7"}
	applyLocalState(info3, st)
	if !info3.Snoozed {
		t.Error("静默期内应标记 Snoozed")
	}

	// 跳过此版本
	st.SnoozeUntil = time.Time{}
	st.SkippedVersion = "v1.0.7"
	info4 := &Info{Current: "v1.0.0", Latest: "v1.0.7"}
	applyLocalState(info4, st)
	if !info4.Skipped {
		t.Error("已跳过该版本时应标记 Skipped")
	}
	if info4.Snoozed {
		t.Error("跳过时不应同时标记 Snoozed")
	}

	// 跳过的版本与最新不一致 → 不生效
	st.SkippedVersion = "v1.0.5"
	info5 := &Info{Current: "v1.0.0", Latest: "v1.0.7"}
	applyLocalState(info5, st)
	if info5.Skipped {
		t.Error("跳过的是旧版本时，新版本不应被标记 Skipped")
	}
}

func TestDevVersionNeverUpdates(t *testing.T) {
	old := version
	defer func() { version = old }()
	version = "dev"
	if IsRelease() {
		t.Error("dev 版本不应视为正式发布")
	}
	info, err := Check(true)
	if err != nil {
		t.Fatalf("dev 版本检查不应报错: %v", err)
	}
	if info.HasUpdate {
		t.Error("dev 版本不应提示更新")
	}
}

func TestCheckNoNetworkUsesCache(t *testing.T) {
	// 断网或 API 不可用时，Check 必须返回缓存结果且不 panic。
	// 这里不断言具体内容，只保证不 panic 且返回非 nil。
	info, _ := Check(false)
	if info == nil {
		t.Fatal("Check 不应返回 nil")
	}
	if info.Current != CurrentVersion() {
		t.Errorf("Current 字段应为当前版本, got %q", info.Current)
	}
}

func TestPickNewestRelease(t *testing.T) {
	// 模拟 CI 并行发布的乱序 release 列表（旧版本晚发布），
	// 验证按版本号选最大而不是按发布时间。
	rels := []release{
		{TagName: "v1.0.7", Prerelease: false, Draft: false},
		{TagName: "v1.1.1", Prerelease: false, Draft: false},
		{TagName: "v1.1.0", Prerelease: false, Draft: false},
		{TagName: "v1.2.0-rc1", Prerelease: true, Draft: false},
		{TagName: "v1.3.0", Prerelease: false, Draft: true},
		{TagName: "bad-tag", Prerelease: false, Draft: false},
		{TagName: "", Prerelease: false, Draft: false},
	}
	best := pickNewest(rels)
	if best == nil || best.TagName != "v1.1.1" {
		t.Errorf("应选出 v1.1.1（按版本号最大），实际 %+v", best)
	}

	// 全是垃圾数据时返回 nil
	if pickNewest([]release{{TagName: ""}}) != nil {
		t.Error("无有效 release 时应返回 nil")
	}
}

func TestDownloadPageURL(t *testing.T) {
	u := DownloadPageURL()
	if u == "" {
		t.Fatal("下载页地址不应为空")
	}
	for _, want := range []string{"github.com", repoOwner, repoName, "releases/latest"} {
		if !contains(u, want) {
			t.Errorf("下载页地址 %q 应包含 %q", u, want)
		}
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
