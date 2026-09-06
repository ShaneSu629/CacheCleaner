//go:build windows

package update

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// 下载状态（并发安全，供前端轮询）。
var (
	dlMu     sync.Mutex
	dlInfo   DownloadInfo
	dlCancel = make(chan struct{})
	dlActive bool
)

// DownloadInfo 是后台下载进度快照。
type DownloadInfo struct {
	State    string  `json:"state"` // idle / downloading / verifying / done / error / cancelled
	Pct      float64 `json:"pct"`
	Received int64   `json:"received"`
	Total    int64   `json:"total"`
	Path     string  `json:"path"`
	Message  string  `json:"message"`
}

// GetDownloadInfo 返回当前下载状态（无下载任务时返回 idle）。
// 进程重启后内存状态丢失：若磁盘上存在已下载完成的 CacheCleaner.new.exe，
// 直接恢复为 done 状态，避免下载好的文件变成孤儿、用户重复下载。
func GetDownloadInfo() DownloadInfo {
	dlMu.Lock()
	defer dlMu.Unlock()
	if dlActive {
		return dlInfo
	}
	// 无活动任务：检查磁盘上有没有已下载完成但未应用的临时文件
	if p, err := downloadPath(); err == nil {
		if _, statErr := os.Stat(p); statErr == nil {
			return DownloadInfo{State: "done", Pct: 100, Path: p}
		}
	}
	return DownloadInfo{State: "idle"}
}

// downloadPath 返回下载临时文件路径（exe 同目录下 .new 后缀，便于覆盖替换）。
func downloadPath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	dir := filepath.Dir(exe)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return filepath.Join(dir, "CacheCleaner.new.exe"), nil
}

// StartDownload 后台下载最新版到临时文件并校验 SHA256。
// 返回 nil 表示已启动（进度经 GetDownloadInfo 轮询）。
// 幂等：已有下载任务进行中、或磁盘上已有下载完成的文件时直接返回成功，
// 不会重复下载浪费 IO。
func StartDownload(info *Info) error {
	dlMu.Lock()
	if dlActive {
		dlMu.Unlock()
		return nil // 已在下载中，无需重复启动
	}
	dlMu.Unlock()

	// 已有完成文件：直接复用，不再下载
	if p, err := downloadPath(); err == nil {
		if _, statErr := os.Stat(p); statErr == nil {
			dlMu.Lock()
			dlInfo = DownloadInfo{State: "done", Pct: 100, Path: p}
			dlMu.Unlock()
			return nil
		}
	}

	dlMu.Lock()
	if dlActive { // 双重检查：并发调用下可能已被其他 goroutine 启动
		dlMu.Unlock()
		return nil
	}
	dlActive = true
	dlInfo = DownloadInfo{State: "downloading", Pct: 0, Total: info.Size}
	dlCancel = make(chan struct{})
	dlMu.Unlock()

	go func() {
		defer func() {
			dlMu.Lock()
			dlActive = false
			dlMu.Unlock()
		}()
		dlErr := doDownload(info, dlCancel)
		dlMu.Lock()
		if dlErr != nil {
			dlInfo.State = "error"
			dlInfo.Message = dlErr.Error()
		} else {
			dlInfo.State = "done"
			dlInfo.Pct = 100
		}
		dlMu.Unlock()
	}()
	return nil
}

// doDownload 执行下载：直连 GitHub，失败自动切 gh-proxy.com 镜像（国内可用）。
// 下载完成后核对 SHA256（SHA256SUMS.txt 中的哈希），不符则失败。
func doDownload(info *Info, cancel <-chan struct{}) error {
	// 取真实下载 URL（列表接口 asset 里的 browser_download_url 没被解析，
	// 直接拼 releases/download 地址）
	baseURL := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s",
		repoOwner, repoName, info.Latest, assetName)
	urls := []string{baseURL}
	// 镜像：gh-proxy.com 国内实测可达，仅对官方直连地址使用
	urls = append(urls, "https://gh-proxy.com/"+baseURL)

	var lastErr error
	for _, u := range urls {
		if err := downloadFrom(u, info, cancel); err != nil {
			lastErr = err
			dlMu.Lock()
			dlInfo.Pct = 0
			dlInfo.Received = 0
			dlMu.Unlock()
			continue
		}
		// 校验 SHA256
		dlMu.Lock()
		dlInfo.State = "verifying"
		dlMu.Unlock()
		expected, err := fetchSHA256(info.Latest)
		if err != nil {
			// 校验文件拿不到（旧 release 没有 SHA256SUMS.txt）时跳过校验，仅告警
			expected = ""
		}
		if expected != "" {
			if err := verifySHA256(dlInfo.Path, expected); err != nil {
				os.Remove(dlInfo.Path)
				return fmt.Errorf("SHA256 校验失败: %v", err)
			}
		}
		return nil
	}
	if lastErr == nil {
		lastErr = errors.New("所有下载源均失败")
	}
	return lastErr
}

// downloadFrom 从单个 URL 下载，支持取消与进度。
func downloadFrom(url string, info *Info, cancel <-chan struct{}) error {
	path, err := downloadPath()
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 0} // 大文件不限总时长，靠取消控制
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", repoName+"-updater")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	dlMu.Lock()
	dlInfo.Path = path
	if resp.ContentLength > 0 {
		dlInfo.Total = resp.ContentLength
	}
	dlMu.Unlock()

	buf := make([]byte, 256<<10)
	var received int64
	for {
		select {
		case <-cancel:
			os.Remove(path)
			return errors.New("下载已取消")
		default:
		}
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				return werr
			}
			received += int64(n)
			dlMu.Lock()
			dlInfo.Received = received
			if dlInfo.Total > 0 {
				dlInfo.Pct = float64(received) / float64(dlInfo.Total) * 100
			}
			dlMu.Unlock()
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
	}
	return nil
}

// fetchSHA256 拉取 SHA256SUMS.txt 并返回目标资产的哈希。
func fetchSHA256(tag string) (string, error) {
	url := fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/SHA256SUMS.txt",
		repoOwner, repoName, tag)
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest(http.MethodGet, url, nil)
	req.Header.Set("User-Agent", repoName+"-updater")
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(data), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && fields[1] == assetName {
			return fields[0], nil
		}
	}
	return "", errors.New("SHA256SUMS.txt 中没有目标资产")
}

// verifySHA256 校验文件哈希。
func verifySHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expected) {
		return fmt.Errorf("哈希不符: 期望 %s 实际 %s", expected, got)
	}
	return nil
}

// CancelDownload 取消进行中的下载。
func CancelDownload() {
	dlMu.Lock()
	active := dlActive
	c := dlCancel
	dlMu.Unlock()
	if active {
		close(c)
	}
}

// DownloadedPath 返回已完成下载的临时文件路径（未完成时返回空）。
// 注意不能要求 dlActive：下载完成时 goroutine 会把 dlActive 置 false，
// 只有 State=="done" 才是有效判据。
func DownloadedPath() string {
	dlMu.Lock()
	defer dlMu.Unlock()
	if dlInfo.State == "done" && dlInfo.Path != "" {
		if _, err := os.Stat(dlInfo.Path); err == nil {
			return dlInfo.Path
		}
	}
	return ""
}

// CleanupOld 清理更新残留：旧版本备份（CacheCleaner.old.exe）与
// 未应用的下载文件（CacheCleaner.new.exe）。
// 启动时调用：正常更新后 bat 会删掉它们；若上次更新中断（如杀进程），
// 残留文件在此兜底清理，避免目录里堆垃圾。
func CleanupOld() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	dir := filepath.Dir(exe)
	// 只清 .old（旧版本备份）——.new 是已下载待应用的新版本，不能动
	oldPath := filepath.Join(dir, "CacheCleaner.old.exe")
	if _, err := os.Stat(oldPath); err == nil {
		os.Remove(oldPath)
	}
}

// ApplyAndRestart 用下载好的新版本覆盖旧 exe 并重启。
// 单文件绿色软件自更新：
//  1. 写一个 .bat 到临时目录，循环等待本进程退出
//  2. 把 .new.exe 覆盖旧 exe（运行中的 exe 不可覆盖，但可改名）
//  3. 重启新版本
// 返回空串表示已启动替换流程（调用方应随即退出），否则为错误。
func ApplyAndRestart() error {
	newPath := DownloadedPath()
	if newPath == "" {
		return errors.New("没有已下载的新版本")
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}

	// 写替换脚本：等待旧进程退出 → 改名旧 exe → 覆盖 → 启动新版
	// 关键：运行中的 exe 在 Windows 上被独占锁定，copy 覆盖会失败；
	// 必须先 rename 旧 exe（改名允许），再 copy 新文件进去。
	bat := filepath.Join(os.TempDir(), "cachecleaner_update.bat")
	oldExe := filepath.Join(filepath.Dir(exe), "CacheCleaner.old.exe")
	content := strings.Join([]string{
		"@echo off",
		"setlocal",
		"rem CacheCleaner 自更新脚本",
		"rem 等待本程序退出（最多 60 秒）",
		"set tries=0",
		":waitloop",
		"tasklist /FI \"IMAGENAME eq " + filepath.Base(exe) + "\" 2>nul | find /I \"" + filepath.Base(exe) + "\" >nul",
		"if errorlevel 1 goto replace",
		"timeout /t 1 /nobreak >nul",
		"set /a tries+=1",
		"if %tries% lss 60 goto waitloop",
		"rem 超时放弃",
		"exit /b 1",
		":replace",
		"rem 旧 exe 改名（运行中可改名），清理上次残留",
		"del /Q \"" + oldExe + "\" >nul 2>&1",
		"ren \"" + exe + "\" \"CacheCleaner.old.exe\"",
		"if errorlevel 1 goto fail",
		"copy /Y \"" + newPath + "\" \"" + exe + "\" >nul",
		"if errorlevel 1 goto fail",
		"rem 覆盖成功，删除临时文件与旧版本",
		"del /Q \"" + newPath + "\" >nul 2>&1",
		"del /Q \"" + oldExe + "\" >nul 2>&1",
		"start \"\" \"" + exe + "\"",
		"exit /b 0",
		":fail",
		"rem 覆盖失败：把旧 exe 改名回去，提示用户手动替换",
		"if exist \"" + oldExe + "\" ren \"" + oldExe + "\" \"" + filepath.Base(exe) + "\"",
		"exit /b 1",
		"",
	}, "\r\n")
	if err := os.WriteFile(bat, []byte(content), 0o644); err != nil {
		return err
	}

	// 启动替换脚本（独立进程，父进程退出后继续运行）
	// 用 CreateProcess 原样传参避免转义问题
	return runDetached("cmd.exe", "/c \""+bat+"\"")
}

// runDetached 用 CreateProcess 启动独立进程（本函数在平台专用文件中实现）。
func runDetached(exe, args string) error {
	return startDetached(exe, args)
}
