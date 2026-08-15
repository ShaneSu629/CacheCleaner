package ui

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"

	"cachecleaner/internal/model"
)

// Prompt 读取单行用户输入。遇到 EOF（无交互/管道结束）时返回空字符串。
func Prompt(msg string) string {
	fmt.Print(msg)
	r := bufio.NewReader(os.Stdin)
	line, err := r.ReadString('\n')
	if err != nil && len(line) == 0 {
		return ""
	}
	return strings.TrimSpace(line)
}

// ShowMainMenu 展示主菜单并返回用户选择。
func ShowMainMenu() int {
	fmt.Println()
	fmt.Println(col("══════════════════════════════════════", cCyan))
	fmt.Println(col("       智能缓存清理工具", cCyan))
	fmt.Println(col("══════════════════════════════════════", cCyan))
	fmt.Println("  [1] 深度扫描 AI 缓存（聚焦 AI/IDE 工具）")
	fmt.Println("  [2] 一键扫描所有缓存（含浏览器/IM/网盘）")
	fmt.Println("  [3] 自定义缓存目录管理")
	fmt.Println("  [4] 查看清理历史")
	fmt.Println("  [0] 退出")
	fmt.Println()

	input := Prompt("请选择操作: ")
	n, err := strconv.Atoi(input)
	if err != nil {
		return -1
	}
	return n
}

// SelectEntries 展示列表并让用户选择要清理的项，返回选中项与是否确认。
func SelectEntries(entries []model.CacheEntry) ([]model.CacheEntry, bool) {
	if len(entries) == 0 {
		fmt.Println(col("未发现可清理的缓存项。", cYellow))
		return nil, false
	}
	RenderEntries(entries)
	RenderSummary(entries)

	fmt.Println()
	fmt.Println("选择要清理的项: 输入编号(逗号分隔), 或")
	fmt.Println("  safe = 仅清理[安全]项    all = 全部    q = 取消")
	input := Prompt("> ")

	if input == "" || input == "q" || input == "Q" {
		return nil, false
	}

	var chosen []model.CacheEntry
	switch input {
	case "all", "ALL":
		chosen = append(chosen, entries...)
	case "safe", "SAFE":
		for _, e := range entries {
			if e.Risk == model.RiskSafe {
				chosen = append(chosen, e)
			}
		}
	default:
		for _, part := range strings.Split(input, ",") {
			part = strings.TrimSpace(part)
			n, err := strconv.Atoi(part)
			if err != nil || n < 1 || n > len(entries) {
				continue
			}
			chosen = append(chosen, entries[n-1])
		}
	}

	if len(chosen) == 0 {
		fmt.Println(col("未选择任何项。", cYellow))
		return nil, false
	}

	var sz int64
	for _, e := range chosen {
		sz += e.Size
	}
	confirm := Prompt(fmt.Sprintf("确认清理 %s 项 (释放约 %s)? [y/N] ",
		col(strconv.Itoa(len(chosen)), cCyan), col(FormatSize(sz), cGreen)))
	if confirm != "y" && confirm != "Y" {
		fmt.Println("已取消。")
		return nil, false
	}
	return chosen, true
}
