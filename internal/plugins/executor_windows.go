//go:build windows

package plugins

import (
	"os/exec"
	"syscall"
)

// hideConsoleWindow 隐藏子进程的控制台窗口。
// 插件脚本通过 cc.exec 执行系统命令时，若不加此设置，Windows 会为 cmd 弹出一个
// 黑框一闪而过（用户看到的「类似窗口的东西闪一下」）。设置 HideWindow 后，
// 子进程在后台静默运行，不再弹出控制台窗口。
func hideConsoleWindow(c *exec.Cmd) {
	if c.SysProcAttr == nil {
		c.SysProcAttr = &syscall.SysProcAttr{}
	}
	c.SysProcAttr.HideWindow = true
}
