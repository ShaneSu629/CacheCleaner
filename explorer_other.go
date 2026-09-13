//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

// openExplorer 用系统文件管理器打开目录（macOS/Linux）。
func openExplorer(dir string) {
	switch runtime.GOOS {
	case "darwin":
		_ = exec.Command("open", dir).Start()
	default:
		_ = exec.Command("xdg-open", dir).Start()
	}
}
