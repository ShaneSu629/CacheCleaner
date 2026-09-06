//go:build windows

package update

import (
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// startDetached 用 CreateProcess 以独立进程方式启动命令（无窗口、不随父进程退出）。
// 命令行原样传递，避免 exec.Command 的转义破坏。
func startDetached(file, cmdline string) error {
	filePtr, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	cmdPtr, err := windows.UTF16PtrFromString(cmdline)
	if err != nil {
		return err
	}
	var si windows.StartupInfo
	var pi windows.ProcessInformation
	si.Cb = uint32(unsafe.Sizeof(si))
	si.ShowWindow = 0
	si.Flags = 0x00000001 // STARTF_USESHOWWINDOW
	// DETACHED_PROCESS (0x8) + CREATE_NO_WINDOW (0x08000000)：
	// 独立进程组，父进程退出不影响脚本继续运行
	err = windows.CreateProcess(filePtr, cmdPtr, nil, nil, false,
		0x8|0x08000000, nil, nil, &si, &pi)
	if err != nil {
		return err
	}
	windows.CloseHandle(pi.Thread)
	windows.CloseHandle(pi.Process)
	return nil
}

var _ = syscall.Errno(0)
