//go:build !windows

package clean

// ScheduleRebootDeletes 非 Windows 平台不支持重启删除，返回 0。
func ScheduleRebootDeletes(root string) int { return 0 }
