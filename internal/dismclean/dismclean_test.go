package dismclean

import (
	"strings"
	"testing"
)

// 中文 Windows 的 AnalyzeComponentStore 典型输出。
const sampleZH = `
部署映像服务和管理工具
版本: 10.0.22631.2861

映像版本: 10.0.22631.2861

[==========================100.0%==========================]

组件存储(WinSxS)信息:

Windows 资源管理器报告的组件存储大小 : 8.91 GB

组件存储的实际大小 : 8.45 GB

与 Windows 共享的二进制文件数 : 1512

上次清理日期 : 2026-08-01 10:20:33

可回收包数 : 3

建议组件存储清理 : 是

操作成功完成。
`

// 英文 Windows 的对应输出。
const sampleEN = `
Deployment Image Servicing and Management tool
Version: 10.0.22631.2861

Image Version: 10.0.22631.2861

[==========================100.0%==========================]

Component Store (WinSxS) information:

Windows Explorer Reported Size of Component Store : 8.91 GB

Actual Size of Component Store : 8.45 GB

Shared with Windows : 1512

Date of Last Cleanup : 2026-08-01 10:20:33

Reclaimable Packages : 3

Component Store Cleanup Recommended : Yes

The operation completed successfully.
`

func TestParseReportZH(t *testing.T) {
	r, ok := parseReport(sampleZH)
	if !ok {
		t.Fatal("中文输出应能解析")
	}
	if r.ReportedSize != "8.91 GB" {
		t.Errorf("报告大小解析错误: %q", r.ReportedSize)
	}
	if r.ActualSize != "8.45 GB" {
		t.Errorf("实际大小解析错误: %q", r.ActualSize)
	}
	if r.ReclaimablePkgs != "3" {
		t.Errorf("可回收包数解析错误: %q", r.ReclaimablePkgs)
	}
	if r.LastCleanup != "2026-08-01 10:20:33" {
		t.Errorf("上次清理日期解析错误: %q", r.LastCleanup)
	}
	if !r.Recommended {
		t.Error("应识别出建议清理")
	}
}

func TestParseReportEN(t *testing.T) {
	r, ok := parseReport(sampleEN)
	if !ok {
		t.Fatal("英文输出应能解析")
	}
	if r.ActualSize != "8.45 GB" {
		t.Errorf("实际大小解析错误: %q", r.ActualSize)
	}
	if r.ReclaimablePkgs != "3" {
		t.Errorf("可回收包数解析错误: %q", r.ReclaimablePkgs)
	}
	if !r.Recommended {
		t.Error("应识别出建议清理 (Yes)")
	}
}

func TestParseReportGarbage(t *testing.T) {
	_, ok := parseReport("some random\noutput\n")
	if ok {
		t.Error("无关输出不应解析成功")
	}
}

func TestParseProgress(t *testing.T) {
	out := "映像版本: 10.0.22631.2861\r\n\r\n[=====                     10.0%                    ]\r\n[==========               42.5%                    ]\r"
	pct, line := parseProgress(out)
	if pct != 42.5 {
		t.Errorf("应取最近一次百分比 42.5，实际 %v", pct)
	}
	if !strings.Contains(line, "42.5%") {
		t.Errorf("最后一行应为进度行: %q", line)
	}

	pct2, _ := parseProgress("")
	if pct2 != 0 {
		t.Errorf("空输出百分比应为 0，实际 %v", pct2)
	}
}

func TestTailLines(t *testing.T) {
	s := "a\n\nb\nc\n"
	got := tailLines(s, 2)
	if got != "b\nc" {
		t.Errorf("tailLines 应取最后 2 个非空行: %q", got)
	}
	if tailLines("", 3) != "" {
		t.Error("空输入应返回空串")
	}
}

func TestValueAfterColon(t *testing.T) {
	if got := valueAfterColon("组件存储的实际大小 : 8.45 GB"); got != "8.45 GB" {
		t.Errorf("英文冒号取值错误: %q", got)
	}
	if got := valueAfterColon("实际大小：8.45 GB"); got != "8.45 GB" {
		t.Errorf("中文冒号取值错误: %q", got)
	}
}

// TestBuildJobCommand 回归测试：echo 数字与 > 之间必须有空格。
// 历史 bug："echo 0>file" 中 0> 被 cmd 解析成重定向句柄 0，done 文件
// 内容变成垃圾，DISM 成功却被误判为失败。
func TestBuildJobCommand(t *testing.T) {
	cmd := buildJobCommand("/Online /Cleanup-Image /AnalyzeComponentStore", `C:\t\out.txt`, `C:\t\done.txt`, false)
	if strings.Contains(cmd, "echo 0>") || strings.Contains(cmd, "echo 1>") {
		t.Errorf("echo 与 > 之间缺少空格会被 cmd 解析成句柄重定向: %s", cmd)
	}
	if !strings.Contains(cmd, `echo 0 > "C:\t\done.txt"`) || !strings.Contains(cmd, `echo 1 > "C:\t\done.txt"`) {
		t.Errorf("done 标志命令不正确: %s", cmd)
	}
	if !strings.Contains(cmd, `>"C:\t\out.txt" 2>&1`) {
		t.Errorf("输出重定向不正确: %s", cmd)
	}
	// analyze 不应含 TrustedInstaller / wuauserv
	if strings.Contains(cmd, "TrustedInstaller") || strings.Contains(cmd, "wuauserv") {
		t.Errorf("analyze 不应含服务操作: %s", cmd)
	}

	// cleanup 需要 TrustedInstaller + 暂停/恢复 Windows Update
	cmd2 := buildJobCommand("/Online /Cleanup-Image /StartComponentCleanup", `C:\t\out.txt`, `C:\t\done.txt`, true)
	if !strings.Contains(cmd2, `sc stop wuauserv`) {
		t.Errorf("cleanup 应含 sc stop wuauserv: %s", cmd2)
	}
	if !strings.Contains(cmd2, `sc start TrustedInstaller`) {
		t.Errorf("cleanup 应含 sc start TrustedInstaller: %s", cmd2)
	}
	if !strings.Contains(cmd2, `sc start wuauserv`) {
		t.Errorf("cleanup 结束后应恢复 wuauserv: %s", cmd2)
	}
	// 服务操作必须在 dism 之前
	stopIdx := strings.Index(cmd2, "sc stop wuauserv")
	dismIdx := strings.Index(cmd2, "dism")
	if stopIdx >= dismIdx || stopIdx < 0 {
		t.Errorf("sc stop 应在 dism 之前: %s", cmd2)
	}
	// 恢复 wuauserv 必须在 dism 之后
	startIdx := strings.Index(cmd2, "sc start wuauserv")
	if startIdx < dismIdx {
		t.Errorf("sc start wuauserv 应在 dism 之后: %s", cmd2)
	}
}
