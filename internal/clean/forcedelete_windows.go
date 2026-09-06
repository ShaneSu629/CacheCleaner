//go:build windows

package clean

import (
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/sys/windows"
)

// scheduleDeleteReboot 把被占用的文件登记进 Windows 的
// PendingFileRenameOperations（重启时由系统在早期删除），
// 这是删除"被运行中程序占用文件"的唯一官方机制，不需要杀进程。
//
// 返回 nil 表示登记成功，重启后自动删除；失败原因记录在 err。
func scheduleDeleteReboot(path string) error {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	// MOVEFILE_DELAY_UNTIL_REBOOT = 0x4
	if err := windows.MoveFileEx(p, nil, 0x4); err != nil {
		return err
	}
	return nil
}

// ScheduleRebootDeletes 对删不掉的路径做重启删除登记（Windows 官方延迟删除机制）。
//
// 策略：
//   - 目录 → 递归登记其中所有文件（MoveFileEx 对非空目录不可靠，
//     逐个登记文件后，重启时目录自然变空）
//   - 文件 → 直接登记
//   - 优先删除"纯缓存"子目录内的文件（如 Cache/GPUCache/logs），
//     这些本来就是垃圾，重启删除没有副作用
//
// 返回成功登记的文件数。登记失败的文件静默跳过（保持"能删多少删多少"）。
func ScheduleRebootDeletes(root string) int {
	if !strings.Contains(strings.ToLower(root), "cache") &&
		!strings.Contains(strings.ToLower(root), "logs") &&
		!strings.Contains(strings.ToLower(root), "temp") {
		return 0 // 只对缓存/日志/临时目录做重启删除，避免误伤
	}
	// 收集中所有文件（限制数量避免极端目录卡死）
	var files []string
	filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || len(files) >= 2000 {
			if err != nil {
				return nil // 访问失败跳过继续
			}
			return filepath.SkipAll
		}
		if !d.IsDir() {
			files = append(files, path)
		}
		return nil
	})
	sort.Slice(files, func(i, j int) bool { return len(files[i]) > len(files[j]) })
	scheduled := 0
	for _, f := range files {
		if scheduleDeleteReboot(f) == nil {
			scheduled++
		}
	}
	if scheduled == 0 {
		// 目录内没有可登记的文件：尝试直接登记目录本身（空目录或小目录可能成功）
		if scheduleDeleteReboot(root) == nil {
			return 1
		}
	}
	return scheduled
}
