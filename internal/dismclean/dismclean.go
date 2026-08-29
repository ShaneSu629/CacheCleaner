// Package dismclean 提供 WinSxS 组件存储的分析与清理（对齐 Dism++ 的核心能力）。
//
// 与普通缓存清理的本质区别：组件存储清理不是删目录，而是执行
//   Dism /Online /Cleanup-Image /AnalyzeComponentStore   （分析）
//   Dism /Online /Cleanup-Image /StartComponentCleanup   （清理）
// DISM 的任何 /Online 操作都要求管理员权限（错误 740），因此本包的所有执行
// 都走"提权作业"模型（见 dismclean_windows.go）：非管理员进程通过
// ShellExecute runas 弹 UAC 授权启动子进程，dism 输出重定向到临时文件，
// 调用方轮询 Poll() 获取进度。不使用 /ResetBase（清理后仍可卸载已装更新）。
package dismclean

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Report 是 AnalyzeComponentStore 的解析结果（字段为空表示解析不到，用 Raw 兜底展示）。
type Report struct {
	ReportedSize    string // 资源管理器报告的组件存储大小（含硬链接虚高）
	ActualSize      string // 组件存储实际大小
	ReclaimablePkgs string // 可回收包数
	LastCleanup     string // 上次清理日期
	Recommended     bool   // DISM 是否建议执行清理
	Raw             string // 原始输出尾部（解析失败时展示给用户）
}

// Status 是一次提权作业的进度快照。
type Status struct {
	Running bool    // 作业进行中
	Pct     float64 // 解析到的最近百分比（0-100，未解析到时为 0）
	Line    string  // 最近一行输出（截断展示用）
	Done    bool    // 作业已结束
	OK      bool    // 结束时的成败（Done 为 true 时有意义）
	Message string  // 结束后的汇总信息 / 出错原因
}

// buildJobCommand 构造提权作业命令：执行 dism 并把输出重定向到 outFile，
// 结束后向 doneFile 写入 "0"（成功）或 "1"（失败）。
// 关键细节：echo 的数字与 > 之间必须有空格——"echo 0>file" 中 0> 会被 cmd
// 解析成重定向句柄 0（而不是输出文本 0），done 文件内容变成垃圾导致误判失败。
func buildJobCommand(dismArgs, outFile, doneFile string) string {
	return fmt.Sprintf(`dism %s >"%s" 2>&1 && (echo 0 > "%s") || (echo 1 > "%s")`,
		dismArgs, outFile, doneFile, doneFile)
}

// pctRe 匹配 DISM 进度行中的百分比，如 [======                    20.0%      ]
var pctRe = regexp.MustCompile(`(\d+(?:\.\d+)?)\s*%`)

// parseProgress 从 DISM 输出中解析最近一次出现的百分比与最后一行有效文本。
func parseProgress(output string) (pct float64, lastLine string) {
	var v float64
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimRight(line, "\r \t")
		if line == "" {
			continue
		}
		lastLine = line
		if m := pctRe.FindStringSubmatch(line); m != nil {
			if f, err := strconv.ParseFloat(m[1], 64); err == nil {
				v = f
			}
		}
	}
	return v, lastLine
}

// 分析报告关键词（中英文 Windows 都覆盖；值取冒号后内容）。
var reportKeys = []struct {
	zh, en string
	set    func(*Report, string)
}{
	{"资源管理器报告的组件存储大小", "Size of Component Store reported by Windows Explorer", func(r *Report, v string) { r.ReportedSize = v }},
	{"组件存储的实际大小", "Actual Size of Component Store", func(r *Report, v string) { r.ActualSize = v }},
	{"可回收包数", "Reclaimable Packages", func(r *Report, v string) { r.ReclaimablePkgs = v }},
	{"上次清理日期", "Date of Last Cleanup", func(r *Report, v string) { r.LastCleanup = v }},
}

// parseReport 解析 AnalyzeComponentStore 的输出（中文或英文系统）。
// 解析不到任何字段时返回 false，调用方应展示 Raw。
func parseReport(output string) (*Report, bool) {
	r := &Report{}
	matched := 0
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line == "" {
			continue
		}
		for _, k := range reportKeys {
			if idx := strings.Index(line, k.zh); idx >= 0 {
				k.set(r, valueAfterColon(line[idx:]))
				matched++
			} else if idx := strings.Index(line, k.en); idx >= 0 {
				k.set(r, valueAfterColon(line[idx:]))
				matched++
			}
		}
		// 建议清理标记：中文"是"、英文 Yes
		if strings.Contains(line, "建议组件存储清理") || strings.Contains(line, "Component Store Cleanup Recommended") {
			v := valueAfterColon(line)
			if strings.Contains(v, "是") || strings.EqualFold(strings.TrimSpace(v), "yes") {
				r.Recommended = true
			}
			matched++
		}
	}
	// 保留输出尾部作为兜底展示（进度行噪音较多，从后往前找有效行）
	r.Raw = tailLines(output, 12)
	return r, matched > 0
}

// valueAfterColon 取中英文冒号后的值并去除空白。
// 注意按 rune 切分：全角冒号"："占 3 字节，按字节 +1 会切进字符中间。
func valueAfterColon(s string) string {
	for i, r := range s {
		if r == ':' || r == '：' {
			return strings.TrimSpace(s[i+len(string(r)):])
		}
	}
	return strings.TrimSpace(s)
}

// tailLines 取输出的最后 n 个非空行。
func tailLines(s string, n int) string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimRight(line, "\r \t")
		if line != "" {
			lines = append(lines, line)
		}
	}
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
