//go:build windows

package main

import (
	"os/exec"
)

// openExplorer 用系统资源管理器打开目录（Windows）。
func openExplorer(dir string) {
	_ = exec.Command("explorer", dir).Start()
}
