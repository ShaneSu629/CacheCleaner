//go:build !windows

package plugins

import "os/exec"

// hideConsoleWindow 非 Windows 平台无控制台窗口概念，空实现。
func hideConsoleWindow(c *exec.Cmd) {}
