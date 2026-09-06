package singleflight

import (
	"syscall"
	"unsafe"
)

var (
	kernel32                 = syscall.NewLazyDLL("kernel32.dll")
	procCreateMutexW         = kernel32.NewProc("CreateMutexW")
	user32                   = syscall.NewLazyDLL("user32.dll")
	procFindWindowW          = user32.NewProc("FindWindowW")
	procIsIconic             = user32.NewProc("IsIconic")
	procShowWindow           = user32.NewProc("ShowWindow")
	procSetForegroundWindow  = user32.NewProc("SetForegroundWindow")
	procBringWindowToTop     = user32.NewProc("BringWindowToTop")
)

const (
	errorAlreadyExists = 183 // ERROR_ALREADY_EXISTS
	swRestore          = 9   // SW_RESTORE：恢复最小化窗口
	swShow             = 5   // SW_SHOW
)

// Acquire 获取单实例互斥体。返回 false 表示已有实例在运行。
// 此时静默定位已运行实例的窗口（按 windowTitle 查找），恢复并前置到前台，
// 然后本进程直接退出——用户双击 exe 的体验是"界面跳到已开的软件上"，无任何弹窗。
// 互斥体句柄随进程生命周期持有，不需要显式释放（进程退出时系统自动回收）。
// 命名用 Local 命名空间：同用户同会话内互斥，管理员与普通用户会话互不影响。
//
// 注意：LazyProc.Call 的第三个返回值就是 GetLastError 的系统错误，
// 直接用它判断 ERROR_ALREADY_EXISTS，不要再单独调 GetLastError（有竞争）。
func Acquire(name, windowTitle string) bool {
	namePtr, _ := syscall.UTF16PtrFromString("Local\\" + name)
	h, _, err := procCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(namePtr)))
	if h == 0 {
		// 创建失败（极少见）：不阻塞启动，宁可多开也不让用户打不开程序
		return true
	}
	if err != nil && err == syscall.Errno(errorAlreadyExists) {
		focusExistingWindow(windowTitle)
		return false
	}
	return true
}

// focusExistingWindow 找到已运行实例的主窗口，恢复并前置到前台。
// 查找失败或前置失败都静默（多开本来也只是重复打开，不影响使用）。
func focusExistingWindow(windowTitle string) {
	title, _ := syscall.UTF16PtrFromString(windowTitle)
	// 窗口类名传 nil：Wails 窗口的类名由框架生成，按标题查找更可靠。
	hwnd, _, _ := procFindWindowW.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		return
	}
	// 最小化时先恢复，否则 SetForegroundWindow 只会让任务栏图标闪烁
	if r, _, _ := procIsIconic.Call(hwnd); r != 0 {
		procShowWindow.Call(hwnd, swRestore)
	} else {
		procShowWindow.Call(hwnd, swShow)
	}
	// 双重保险：SetForegroundWindow 受前台锁定限制（后台进程抢焦点会被拒绝），
	// 失败时用 BringWindowToTop 兜底（能把窗口拉到 Z 序顶部）。
	if r, _, _ := procSetForegroundWindow.Call(hwnd); r == 0 {
		procBringWindowToTop.Call(hwnd)
		procSetForegroundWindow.Call(hwnd)
	}
}
