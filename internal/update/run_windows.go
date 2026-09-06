//go:build windows

package update

import (
	"unsafe"

	"golang.org/x/sys/windows"
)

// startDetached 用 CreateProcess 以独立进程方式启动命令（无窗口、不随父进程退出）。
//
// 关键坑：lpApplicationName 必须传 nil。若传相对路径（如 "cmd.exe"），
// Windows 不做 PATH 搜索，只按字面路径找文件 → 找不到就报
// ERROR_FILE_NOT_FOUND（The system cannot find the file specified）。
// 传 nil 时系统从命令行第一个 token 解析，自动搜索 PATH——
// 所以 cmdline 必须以可执行文件名开头（如 "cmd.exe /c ..."）。
func startDetached(cmdline string) error {
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
	err = windows.CreateProcess(nil, cmdPtr, nil, nil, false,
		0x8|0x08000000, nil, nil, &si, &pi)
	if err != nil {
		return err
	}
	windows.CloseHandle(pi.Thread)
	windows.CloseHandle(pi.Process)
	return nil
}
