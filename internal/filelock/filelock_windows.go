//go:build windows

// Package filelock 通过 Windows Restart Manager API 查找占用指定文件的进程，
// 并支持结束这些进程。Restart Manager 是微软官方的"谁锁了文件"查询机制
// （rm.exe / 进程管理器"占用"列同源），比按文件名猜进程可靠。
package filelock

import (
	"sort"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Locker 是占用目标文件的进程信息。
type Locker struct {
	Pid  uint32
	Name string // 进程名（如 WorkBuddy.exe）
	Path string // 可执行文件完整路径
	Safe bool   // 是否可安全结束（系统关键进程为 false）
}

// 不可结束的系统关键进程（按进程名，小写比较）。
var protectedNames = map[string]bool{
	"system": true, "smss.exe": true, "csrss.exe": true, "wininit.exe": true,
	"services.exe": true, "lsass.exe": true, "winlogon.exe": true,
	"svchost.exe": true, "explorer.exe": true, "dwm.exe": true,
	"fontdrvhost.exe": true, "msmpeng.exe": true, "searchindexer.exe": true,
	"runtimebroker.exe": true, "sihost.exe": true, "taskhostw.exe": true,
	"ctfmon.exe": true, "audiodg.exe": true, "spoolsv.exe": true,
	"conhost.exe": true, "cmd.exe": true,
}

// Restart Manager 应用类型（RM_APP_TYPE）。
const (
	rmUnknownApp  = 0
	rmMainWindow  = 1
	rmOtherWindow = 2
	rmService     = 3
	rmExplorer    = 4
	rmConsole     = 5
	rmCritical    = 1000
)

// CCH_RM_MAX_APP_NAME / CCH_RM_MAX_SVC_NAME（Windows SDK 定义）。
const (
	cchRmMaxAppName = 255
	cchRmMaxSvcName = 63
)

var (
	rmdll              = windows.NewLazySystemDLL("rstrtmgr.dll")
	rmStartSession     = rmdll.NewProc("RmStartSession")
	rmEndSession       = rmdll.NewProc("RmEndSession")
	rmRegisterResource = rmdll.NewProc("RmRegisterResources")
	rmGetList          = rmdll.NewProc("RmGetList")
)

type rmUniqueProcess struct {
	ProcessId        uint32
	ProcessStartTime windows.Filetime
}

type rmProcessInfo struct {
	Process          rmUniqueProcess
	AppName          [cchRmMaxAppName + 1]uint16
	ServiceShortName [cchRmMaxSvcName + 1]uint16
	ApplicationType  uint32
	AppStatus        uint32
	TSSessionId      uint32
	Restartable      int32
}

func startSession() (uint32, error) {
	var session uint32
	var key [cchRmMaxSvcName + 1]uint16
	ret, _, _ := rmStartSession.Call(
		uintptr(unsafe.Pointer(&session)),
		0, // dwSessionFlags = 0
		uintptr(unsafe.Pointer(&key[0])),
	)
	if ret != 0 {
		return 0, syscall.Errno(ret)
	}
	return session, nil
}

// registerResources 注册要查询占用的文件路径。
// 微软文档要求每个路径缓冲区至少 CCH_RM_MAX_SVC_NAME+1 个宽字符。
func registerResources(session uint32, paths []string) error {
	total := 0
	for _, p := range paths {
		n := len(p) + 1 // 字节数 >= 宽字符数，安全上界
		if n < cchRmMaxSvcName+1 {
			n = cchRmMaxSvcName + 1
		}
		total += n
	}
	flat := make([]uint16, total)
	ptrs := make([]unsafe.Pointer, len(paths))
	off := 0
	for i, p := range paths {
		u, err := windows.UTF16FromString(p)
		if err != nil {
			return err
		}
		copy(flat[off:off+len(u)], u)
		ptrs[i] = unsafe.Pointer(&flat[off])
		n := len(p) + 1
		if n < cchRmMaxSvcName+1 {
			n = cchRmMaxSvcName + 1
		}
		off += n
	}
	ret, _, _ := rmRegisterResource.Call(
		uintptr(session),
		uintptr(len(paths)),
		uintptr(unsafe.Pointer(&ptrs[0])),
		0, 0, 0, // 不注册应用与服务
	)
	if ret != 0 {
		return syscall.Errno(ret)
	}
	return nil
}

func getProcessList(session uint32) ([]rmProcessInfo, error) {
	var needed, count, reasons uint32
	// 第一次调用拿所需数量（标准 Restart Manager 两段式查询）
	rmGetList.Call(
		uintptr(session),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&count)),
		0,
		uintptr(unsafe.Pointer(&reasons)),
	)
	if needed == 0 {
		return nil, nil
	}
	buf := make([]rmProcessInfo, needed)
	count = needed
	ret, _, _ := rmGetList.Call(
		uintptr(session),
		uintptr(unsafe.Pointer(&needed)),
		uintptr(unsafe.Pointer(&count)),
		uintptr(unsafe.Pointer(&buf[0])),
		uintptr(unsafe.Pointer(&reasons)),
	)
	if ret != 0 {
		return nil, syscall.Errno(ret)
	}
	return buf[:count], nil
}

// processPath 查询进程可执行文件完整路径（PROCESS_QUERY_LIMITED_INFORMATION，
// 对受保护进程也能拿到，且不需要高权限）。
func processPath(pid uint32) string {
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return ""
	}
	defer windows.CloseHandle(h)
	buf := make([]uint16, 1024)
	n := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &n); err != nil {
		return ""
	}
	return windows.UTF16ToString(buf[:n])
}

// FindLockers 查找占用 paths 中任意文件的进程，按进程名排序去重。
// 返回空切片表示没查到（或无权限，Restart Manager 失败时静默降级）。
func FindLockers(paths []string) []Locker {
	if len(paths) == 0 {
		return nil
	}
	session, err := startSession()
	if err != nil {
		return nil
	}
	defer rmEndSession.Call(uintptr(session))
	if err := registerResources(session, paths); err != nil {
		return nil
	}
	infos, err := getProcessList(session)
	if err != nil {
		return nil
	}

	myPid := uint32(windows.GetCurrentProcessId())
	seen := map[uint32]bool{}
	var out []Locker
	for _, info := range infos {
		pid := info.Process.ProcessId
		if pid == 0 || pid == 4 || pid == myPid || seen[pid] {
			continue // 跳过系统 Idle、自身与重复项
		}
		seen[pid] = true
		name := windows.UTF16ToString(info.AppName[:])
		name = strings.TrimRight(name, "\x00")
		if name == "" {
			name = windows.UTF16ToString(info.ServiceShortName[:])
			name = strings.TrimRight(name, "\x00")
		}
		safe := info.ApplicationType != rmService && info.ApplicationType != rmCritical
		if safe && protectedNames[strings.ToLower(name)] {
			safe = false
		}
		out = append(out, Locker{Pid: pid, Name: name, Path: processPath(pid), Safe: safe})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Kill 强制结束指定进程（TerminateProcess）。受保护进程调用方应自行过滤。
func Kill(pid uint32) error {
	h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, pid)
	if err != nil {
		return err
	}
	defer windows.CloseHandle(h)
	return windows.TerminateProcess(h, 1)
}
