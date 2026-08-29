package main

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
	"time"

	"cachecleaner/internal/clean"
	"cachecleaner/internal/config"
	"cachecleaner/internal/dismclean"
	"cachecleaner/internal/model"
	"cachecleaner/internal/regclean"
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
			runClean(cfg, scan.DeepAIScan(cfg, nil))
		case 2:
			fmt.Println("正在一键扫描所有缓存... 请稍候（首次可能较慢）")
			runClean(cfg, scan.SmartScan(cfg, nil))
		case 3:
			manageCustom(cfg)
		case 4:
			showHistory(cfg)
		case 5:
			runRegistry()
		case 6:
			runDismClean()
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
	// 终端进度：单行 \r 刷新，避免刷屏
	progress := func(p clean.Progress) {
		fmt.Printf("\r  清理中 [%d/%d] %s   ", p.Done, p.Total, p.Path)
	}
	freed, count, _, failed := clean.Clean(chosen, cfg, progress)
	fmt.Print("\r\033[K") // 清掉进度行
	fmt.Println()
	fmt.Printf("已清理 %d 项, 释放 %s\n", count, ui.FormatSize(freed))
	for _, f := range failed {
		fmt.Println("  失败:", f)
	}
}

// runRegistry 注册表垃圾清理（仅 Windows 支持）。
func runRegistry() {
	if runtime.GOOS != "windows" {
		fmt.Println("注册表清理仅支持 Windows。")
		return
	}
	fmt.Println("正在扫描注册表垃圾项...")
	entries := regclean.Scan()
	if len(entries) == 0 {
		fmt.Println(ui.Head("未发现可清理的注册表垃圾项。"))
		return
	}
	renderRegistry(entries)
	chosen, ok := ui.SelectRegistry(entries)
	if !ok {
		return
	}
	keys := make([]string, 0, len(chosen))
	for _, e := range chosen {
		keys = append(keys, e.Key)
	}
	cleaned, failed := regclean.Clean(keys)
	var freed int64
	for _, e := range cleaned {
		freed += e.Size
	}
	fmt.Printf("\n已清理 %d 个注册表项, 释放约 %s\n", len(cleaned), ui.FormatSize(freed))
	for _, f := range failed {
		fmt.Println("  失败:", f)
	}
	if len(cleaned) > 0 {
		fmt.Println("提示：部分历史设置在资源管理器重启后生效。")
	}
}

// runDismClean 组件存储（WinSxS）分析与清理：需要管理员授权（非管理员进程会弹 UAC）。
func runDismClean() {
	if runtime.GOOS != "windows" {
		fmt.Println("组件存储清理仅支持 Windows。")
		return
	}
	if !dismclean.Available() {
		fmt.Println("未找到 dism.exe（可能被安全策略禁用）。")
		return
	}
	if !dismclean.IsElevated() {
		fmt.Println(ui.Head("提示：当前不是管理员权限，接下来的操作会弹出 UAC 授权框，请点击「是」。"))
	}

	fmt.Println("  [1] 分析组件存储（查看可回收空间，约 1 分钟）")
	fmt.Println("  [2] 执行组件清理（约 5-20 分钟，不使用 /ResetBase）")
	fmt.Println("  [0] 返回")
	choice := ui.Prompt("请选择操作: ")

	var kind string
	switch choice {
	case "1":
		kind = "analyze"
		if err := dismclean.StartAnalyze(); err != nil {
			fmt.Println("启动失败:", err)
			return
		}
	case "2":
		kind = "cleanup"
		confirm := ui.Prompt("组件清理期间请勿关闭本窗口，确认执行? [y/N] ")
		if !strings.EqualFold(strings.TrimSpace(confirm), "y") {
			return
		}
		if err := dismclean.StartCleanup(); err != nil {
			fmt.Println("启动失败:", err)
			return
		}
	default:
		return
	}

	// 轮询进度：单行刷新百分比，直到作业结束
	for {
		time.Sleep(800 * time.Millisecond)
		st := dismclean.Poll()
		if !st.Running && !st.Done {
			continue
		}
		if st.Running {
			fmt.Printf("\r  进行中 %.1f%%   ", st.Pct)
			continue
		}
		fmt.Print("\r\033[K")
		fmt.Println(st.Message)
		if kind == "analyze" && st.OK {
			if r := dismclean.AnalyzeReport(); r != nil {
				fmt.Println()
				fmt.Println(ui.Head("组件存储报告:"))
				if r.ReportedSize != "" {
					fmt.Println("  资源管理器报告大小:", r.ReportedSize)
				}
				if r.ActualSize != "" {
					fmt.Println("  实际大小:", r.ActualSize)
				}
				if r.ReclaimablePkgs != "" {
					fmt.Println("  可回收包数:", r.ReclaimablePkgs)
				}
				if r.LastCleanup != "" {
					fmt.Println("  上次清理日期:", r.LastCleanup)
				}
				if r.Recommended {
					fmt.Println("  清理建议: 建议执行组件清理")
				}
				if r.ReportedSize == "" && r.ActualSize == "" && r.Raw != "" {
					fmt.Println(r.Raw)
				}
			}
		}
		return
	}
}

func renderRegistry(entries []regclean.Entry) {
	var total int64
	for _, e := range entries {
		total += e.Size
	}
	fmt.Println()
	fmt.Println(ui.Head(fmt.Sprintf("共 %d 项注册表明细, 约 %s", len(entries), ui.FormatSize(total))))
	for i, e := range entries {
		fmt.Printf("  %d. [%s] %s (%d 个值, %s)\n",
			i+1, e.Risk.Label(), e.Desc, e.Values, ui.FormatSize(e.Size))
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
