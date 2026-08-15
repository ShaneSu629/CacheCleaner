package ui

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"cachecleaner/internal/model"
)

var (
	disableColor bool
	cReset       = "\033[0m"
	cRed         = "\033[31m"
	cYellow      = "\033[33m"
	cGreen       = "\033[32m"
	cCyan        = "\033[36m"
	cDim         = "\033[2m"
)

func init() {
	if os.Getenv("NO_COLOR") != "" {
		disableColor = true
		return
	}
	fi, err := os.Stdout.Stat()
	if err == nil && fi.Mode()&os.ModeCharDevice == 0 {
		disableColor = true
	}
}

func col(s, c string) string {
	if disableColor {
		return s
	}
	return c + s + cReset
}

func riskColor(r model.Risk) string {
	switch r {
	case model.RiskSafe:
		return cGreen
	case model.RiskCaution:
		return cYellow
	case model.RiskReview:
		return cRed
	}
	return cReset
}

// FormatSize 把字节数格式化为人类可读字符串。
func FormatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// Head 返回带青色高亮标题（用于菜单/分区标题）。
func Head(s string) string {
	return col(s, cCyan)
}

// RenderEntries 按大小降序渲染缓存列表。
func RenderEntries(entries []model.CacheEntry) {
	sort.Slice(entries, func(i, j int) bool { return entries[i].Size > entries[j].Size })
	fmt.Println()
	header := fmt.Sprintf("%-4s %-10s %-8s %-20s %s", "#", "大小", "风险", "分类", "路径")
	fmt.Println(col(header, cCyan))
	fmt.Println(strings.Repeat("-", 78))
	for i, e := range entries {
		rc := riskColor(e.Risk)
		fmt.Printf("%-4d %-10s %s%-6s%s %-20s %s\n",
			i+1, FormatSize(e.Size), rc, e.Risk.Label(), cReset, e.Category, e.ShortPath)
	}
}

// RenderSummary 打印汇总行。
func RenderSummary(entries []model.CacheEntry) {
	var total int64
	for _, e := range entries {
		total += e.Size
	}
	fmt.Printf("\n共 %s 项, 合计可释放 %s\n", col(fmt.Sprintf("%d", len(entries)), cCyan), col(FormatSize(total), cGreen))
}
