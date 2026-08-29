package applog

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TestInitWritesStartupLine 回归测试：Init 必须能写完启动行。
// 历史 bug：Init 持锁后又调用会再次加锁的 write()，导致启动即死锁
// （日志文件 0 字节、程序卡死）。若死锁复现，本测试会因超时被 go test 杀掉。
func TestInitWritesStartupLine(t *testing.T) {
	done := make(chan string, 1)
	go func() { done <- Init() }()
	select {
	case p := <-done:
		if p == "" {
			t.Skip("当前环境无可用日志目录")
		}
		Info("单测写入测试")
		data, err := os.ReadFile(p)
		if err != nil {
			t.Fatalf("日志文件应可读: %v", err)
		}
		if !strings.Contains(string(data), "启动") {
			t.Errorf("日志应包含启动行，实际内容: %q", string(data))
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Init 超过 10 秒未返回，疑似死锁")
	}
}

// TestWriteWithoutInit 未初始化时写入应静默降级（不 panic、不阻塞）。
func TestWriteWithoutInit(t *testing.T) {
	mu.Lock()
	lg = nil
	if file != nil {
		file.Close()
		file = nil
	}
	inited = false
	mu.Unlock()
	Info("不应 panic")
	Error("不应 panic")
	Recover() // 无 panic 时应原样返回
}
