//go:build windows

package dismclean

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// job 是一次提权作业的运行态：dism 输出写入 outFile，结束后写 doneFile（内容 "0"/"1"）。
type job struct {
	kind     string // "analyze" | "cleanup"
	outFile  string
	doneFile string
	started  time.Time
}

var (
	mu      sync.Mutex
	current *job
	// 作业结束后缓存的最终输出（供 AnalyzeReport / 结果展示），下次 start 时清除。
	lastKind   string
	lastOutput string
)

// Available 检测 dism.exe 是否存在（公司策略可能禁用系统工具，reg.exe 即被禁）。
func Available() bool {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = os.Getenv("windir")
	}
	if root == "" {
		return false
	}
	_, err := os.Stat(filepath.Join(root, "System32", "dism.exe"))
	return err == nil
}

// IsElevated 判断当前进程是否已拥有管理员权限（TokenElevation）。
func IsElevated() bool {
	var token windows.Token
	if err := windows.OpenProcessToken(windows.CurrentProcess(), windows.TOKEN_QUERY, &token); err != nil {
		return false
	}
	defer token.Close()
	var elevation uint32
	var outLen uint32
	const tokenElevation = 20
	err := windows.GetTokenInformation(token, tokenElevation, (*byte)(unsafe.Pointer(&elevation)), 4, &outLen)
	return err == nil && elevation != 0
}

// StartAnalyze 启动组件存储分析（AnalyzeComponentStore）。
func StartAnalyze() error { return start("analyze") }

// StartCleanup 启动组件存储清理（StartComponentCleanup，不带 /ResetBase，清理后仍可卸载已装更新）。
func StartCleanup() error { return start("cleanup") }

// start 启动一次提权作业。同一时刻只允许一个作业。
func start(kind string) error {
	if !Available() {
		return errors.New("未找到 dism.exe（可能被安全策略禁用）")
	}
	mu.Lock()
	if current != nil {
		mu.Unlock()
		return errors.New("已有组件存储任务进行中")
	}
	dir, err := os.MkdirTemp("", "ccdism_")
	if err != nil {
		mu.Unlock()
		return err
	}
	j := &job{
		kind:     kind,
		outFile:  filepath.Join(dir, "out.txt"),
		doneFile: filepath.Join(dir, "done.txt"),
		started:  time.Now(),
	}
	current = j
	lastKind = ""
	lastOutput = ""
	mu.Unlock()

	dismArgs := "/Online /Cleanup-Image /StartComponentCleanup"
	if kind == "analyze" {
		dismArgs = "/Online /Cleanup-Image /AnalyzeComponentStore"
	}
	// 输出重定向到文件（供进度轮询），结束后写 done 标志。
	// 用 && / || 而不是 %ERRORLEVEL%，避免依赖延迟展开；命令行整体经 Unicode
	// 传递（不写 .bat，规避中文用户名路径在 ANSI .bat 中乱码的问题）。
	inner := fmt.Sprintf(`dism %s >"%s" 2>&1 && (echo 0>"%s") || (echo 1>"%s")`,
		dismArgs, j.outFile, j.doneFile, j.doneFile)

	var runErr error
	if IsElevated() {
		// 已是管理员：直接执行，无需再弹 UAC。
		c := exec.Command("cmd.exe", "/c", inner)
		c.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000} // CREATE_NO_WINDOW
		runErr = c.Start()
		if runErr == nil {
			go c.Wait()
		}
	} else {
		runErr = shellExecuteRunAs("cmd.exe", "/c "+inner)
	}
	if runErr != nil {
		mu.Lock()
		current = nil
		mu.Unlock()
		os.RemoveAll(dir)
	}
	return runErr
}

// shellExecuteRunAs 通过 ShellExecuteW(runas) 以管理员身份启动命令（触发 UAC 授权框）。
// 用户拒绝授权时返回明确错误。
func shellExecuteRunAs(file, params string) error {
	shell32 := windows.NewLazySystemDLL("shell32.dll")
	proc := shell32.NewProc("ShellExecuteW")

	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	filePtr, err := windows.UTF16PtrFromString(file)
	if err != nil {
		return err
	}
	paramsPtr, err := windows.UTF16PtrFromString(params)
	if err != nil {
		return err
	}
	const swHide = 0
	ret, _, _ := proc.Call(0,
		uintptr(unsafe.Pointer(verb)),
		uintptr(unsafe.Pointer(filePtr)),
		uintptr(unsafe.Pointer(paramsPtr)),
		0, swHide)
	// ShellExecuteW 返回值 > 32 表示成功，否则为错误码。
	if ret > 32 {
		return nil
	}
	switch ret {
	case 5: // SE_ERR_ACCESSDENIED：用户在 UAC 授权框点了"否"
		return errors.New("提权被取消（UAC 授权被拒绝），组件存储操作需要管理员权限")
	case 2:
		return errors.New("找不到可执行文件: " + file)
	default:
		return fmt.Errorf("提权启动失败（ShellExecute 错误码 %d）", ret)
	}
}

// Poll 读取当前作业的进度快照。无作业时返回零值（Running=false）。
func Poll() Status {
	mu.Lock()
	j := current
	mu.Unlock()
	if j == nil {
		return Status{}
	}
	out := readTextFile(j.outFile)
	pct, line := parseProgress(out)
	st := Status{Running: true, Pct: pct, Line: line}

	if code, ok := readDone(j.doneFile); ok {
		st.Running = false
		st.Done = true
		st.OK = code == "0"
		if st.OK {
			if j.kind == "analyze" {
				st.Message = "组件存储分析完成"
			} else {
				st.Message = "组件存储清理完成"
			}
		} else {
			st.Message = "DISM 执行失败：" + tailLines(out, 3)
		}
		finish(j, out)
	} else if time.Since(j.started) > 30*time.Minute {
		// 兜底：组件清理正常十几分钟，超过 30 分钟认为异常终止
		st.Running = false
		st.Done = true
		st.Message = "任务超时（30 分钟无结果），已停止跟踪"
		finish(j, out)
	}
	return st
}

// AnalyzeReport 返回最近一次完成的分析作业解析结果；无完成记录或解析失败时返回 nil。
func AnalyzeReport() *Report {
	mu.Lock()
	kind, out := lastKind, lastOutput
	mu.Unlock()
	if kind != "analyze" || out == "" {
		return nil
	}
	r, ok := parseReport(out)
	if !ok {
		// 解析失败也把原始输出尾部带回去展示
		return &Report{Raw: tailLines(out, 12)}
	}
	return r
}

// finish 结束作业：缓存最终输出、清除运行态、延时删除临时目录。
func finish(j *job, finalOutput string) {
	mu.Lock()
	if current == j {
		current = nil
		lastKind = j.kind
		lastOutput = finalOutput
	}
	mu.Unlock()
	go func() {
		time.Sleep(30 * time.Second)
		os.RemoveAll(filepath.Dir(j.outFile))
	}()
}

// readDone 读取 done 标志文件，返回内容（"0"/"1"）与是否存在。
func readDone(path string) (string, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(data)), true
}

// readTextFile 读取 DISM 输出文件。DISM 重定向输出使用系统 OEM 代码页
// （中文 Windows 为 GBK），统一按 GBK 解码（ASCII 内容不受损）。
func readTextFile(path string) string {
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return ""
	}
	decoded, err := simplifiedchinese.GBK.NewDecoder().Bytes(data)
	if err != nil {
		return string(data)
	}
	return string(decoded)
}
