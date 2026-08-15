package main

import (
	"fmt"
	"strconv"

	"cachecleaner/internal/clean"
	"cachecleaner/internal/config"
	"cachecleaner/internal/model"
	"cachecleaner/internal/scan"
	"cachecleaner/internal/ui"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Println("配置加载失败:", err)
		return
	}

	for {
		choice := ui.ShowMainMenu()
		switch choice {
		case 1:
			fmt.Println("正在深度扫描 AI 缓存(聚焦 AI/IDE 工具)... 请稍候")
			runClean(cfg, scan.DeepAIScan(cfg))
		case 2:
			fmt.Println("正在一键扫描所有缓存... 请稍候（首次可能较慢）")
			runClean(cfg, scan.SmartScan(cfg))
		case 3:
			manageCustom(cfg)
		case 4:
			showHistory(cfg)
		case 0:
			fmt.Println("再见。")
			return
		default:
			if choice == -1 {
				fmt.Println("输入结束，已退出。")
				return
			}
			fmt.Println("无效选择，请重试。")
		}
	}
}

func runClean(cfg *config.Config, entries []model.CacheEntry) {
	chosen, ok := ui.SelectEntries(entries)
	if !ok {
		return
	}
	freed, count, failed := clean.Clean(chosen, cfg)
	fmt.Println()
	fmt.Printf("已清理 %d 项, 释放 %s\n", count, ui.FormatSize(freed))
	for _, f := range failed {
		fmt.Println("  失败:", f)
	}
}

func manageCustom(cfg *config.Config) {
	for {
		fmt.Println()
		fmt.Println(ui.Head("自定义缓存目录管理"))
		fmt.Println("  [1] 查看当前自定义目录")
		fmt.Println("  [2] 新增目录")
		fmt.Println("  [3] 移除目录")
		fmt.Println("  [4] 查看排除目录")
		fmt.Println("  [5] 新增排除目录")
		fmt.Println("  [6] 返回主菜单")
		input := ui.Prompt("> ")
		if input == "" {
			return
		}
		switch input {
		case "1":
			listDirs("自定义目录", cfg.CustomDirs)
		case "2":
			p := ui.Prompt("输入要添加的目录绝对路径: ")
			if p != "" {
				cfg.CustomDirs = appendUnique(cfg.CustomDirs, p)
				_ = cfg.Save()
				fmt.Println("已添加。")
			}
		case "3":
			listDirs("自定义目录", cfg.CustomDirs)
			idx := ui.Prompt("输入要移除的编号(或 q 取消): ")
			if idx != "q" && idx != "Q" {
				if n, err := strconv.Atoi(idx); err == nil && n >= 1 && n <= len(cfg.CustomDirs) {
					cfg.CustomDirs = append(cfg.CustomDirs[:n-1], cfg.CustomDirs[n:]...)
					_ = cfg.Save()
					fmt.Println("已移除。")
				}
			}
		case "4":
			listDirs("排除目录", cfg.ExcludeDirs)
		case "5":
			p := ui.Prompt("输入要排除的目录名或路径: ")
			if p != "" {
				cfg.ExcludeDirs = appendUnique(cfg.ExcludeDirs, p)
				_ = cfg.Save()
				fmt.Println("已添加排除项。")
			}
		case "6":
			return
		default:
			fmt.Println("无效选择。")
		}
	}
}

func listDirs(title string, dirs []string) {
	fmt.Println()
	if len(dirs) == 0 {
		fmt.Printf("（%s为空）\n", title)
		return
	}
	fmt.Printf("%s:\n", title)
	for i, d := range dirs {
		fmt.Printf("  %d. %s\n", i+1, d)
	}
}

func appendUnique(s []string, v string) []string {
	for _, x := range s {
		if x == v {
			return s
		}
	}
	return append(s, v)
}

func showHistory(cfg *config.Config) {
	recs := clean.LoadHistory(cfg)
	fmt.Println()
	if len(recs) == 0 {
		fmt.Println(ui.Head("（暂无清理历史）"))
		return
	}
	fmt.Println(ui.Head("清理历史"))
	var total int64
	for _, r := range recs {
		fmt.Printf("  %s  %-12s %s\n", r.Time, ui.FormatSize(r.Size), r.Path)
		total += r.Size
	}
	fmt.Printf("合计已释放: %s\n", ui.FormatSize(total))
}
