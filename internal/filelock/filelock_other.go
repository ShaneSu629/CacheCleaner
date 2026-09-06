//go:build !windows

// Package filelock 非 Windows 平台无 Restart Manager，返回空结果。
package filelock

// Locker 是占用目标文件的进程信息（非 Windows 平台无此能力）。
type Locker struct {
	Pid  uint32
	Name string
	Path string
	Safe bool
}

// FindLockers 非 Windows 平台不支持，返回空。
func FindLockers(paths []string) []Locker { return nil }

// Kill 非 Windows 平台不支持。
func Kill(pid uint32) error { return nil }
