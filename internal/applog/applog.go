// Package applog 提供落盘日志：在 exe 同目录写 CacheCleaner.log，
// 记录启动、扫描、清理、DISM 提权作业、前端 JS 错误与 panic，
// 解决"GUI 报错了看不到任何信息"的问题。
//
// 设计要点：
//   - 日志文件在 exe 同目录（便携分发时跟着程序走）；目录不可写时回退到系统临时目录。
//   - 单文件 + 简单轮转：超过 2MB 时改名为 CacheCleaner.old.log（只保留一份历史），
//     避免长期使用后文件无限膨胀。
//   - 全部写操作持锁，Init 未调用时所有函数静默降级为空操作（单测/CLI 环境安全）。
package applog

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

const (
	logName    = "CacheCleaner.log"
	oldName    = "CacheCleaner.old.log"
	maxSize    = 2 << 20 // 2MB
	timeLayout = "2006-01-02 15:04:05.000"
)

var (
	mu     sync.Mutex
	lg     *log.Logger
	file   *os.File
	path   string
	inited bool
)

// Init 打开日志文件（exe 同目录优先，失败回退临时目录）。重复调用幂等。
func Init() string {
	mu.Lock()
	defer mu.Unlock()
	if inited {
		return path
	}
	inited = true

	exe, err := os.Executable()
	if err != nil {
		exe = ""
	}
	candidates := []string{}
	if exe != "" {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), logName))
	}
	candidates = append(candidates, filepath.Join(os.TempDir(), logName))

	for _, p := range candidates {
		rotate(p)
		f, err := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
		if err != nil {
			continue
		}
		file = f
		path = p
		break
	}
	if file == nil {
		return ""
	}
	lg = log.New(file, "", 0)
	write("==== 启动 %s ====", time.Now().Format("2006-01-02 15:04:05"))
	return path
}

// rotate 在日志超过 maxSize 时将其改名为 .old（只留一份历史）。
func rotate(p string) {
	fi, err := os.Stat(p)
	if err != nil || fi.Size() < maxSize {
		return
	}
	old := filepath.Join(filepath.Dir(p), oldName)
	os.Remove(old)
	os.Rename(p, old)
}

// Path 返回当前日志文件路径（未初始化时为空串）。
func Path() string {
	mu.Lock()
	defer mu.Unlock()
	return path
}

// write 内部写日志（调用方需未持有 mu）。
func write(format string, args ...interface{}) {
	mu.Lock()
	defer mu.Unlock()
	if lg == nil {
		return
	}
	line := fmt.Sprintf(format, args...)
	lg.Printf("%s %s", time.Now().Format(timeLayout), line)
}

// Info 记录一般事件（启动、扫描/清理开始与结束等）。
func Info(format string, args ...interface{}) { write("INFO  "+format, args...) }

// Error 记录错误（操作失败、异常返回等）。
func Error(format string, args ...interface{}) { write("ERROR "+format, args...) }

// Recover 捕获 panic 并落盘，用于 defer。调用后 panic 被吞掉，
// 仅适用于 main 顶层兜底（程序随后退出）。
func Recover() {
	if r := recover(); r != nil {
		write("PANIC %v", r)
		Close()
	}
}

// Close 关闭日志文件（程序退出时调用，幂等）。
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if file != nil {
		lg.Printf("%s ==== 退出 ====", time.Now().Format(timeLayout))
		file.Close()
		file = nil
		lg = nil
	}
}
