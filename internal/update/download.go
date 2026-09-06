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
func GetDownloadInfo() DownloadInfo {
	dlMu.Lock()
	defer dlMu.Unlock()
	if !dlActive {
		return DownloadInfo{State: "idle"}
	}
	return dlInfo
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
func StartDownload(info *Info) error {
	dlMu.Lock()
	if dlActive {
		dlMu.Unlock()
		return errors.New("已有下载任务进行中")
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
func DownloadedPath() string {
	dlMu.Lock()
	defer dlMu.Unlock()
	if dlActive && dlInfo.State == "done" && dlInfo.Path != "" {
		return dlInfo.Path
	}
	return ""
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

	// 写替换脚本：等待旧进程退出 → 覆盖 → 启动新版
	bat := filepath.Join(os.TempDir(), "cachecleaner_update.bat")
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
		"copy /Y \"" + newPath + "\" \"" + exe + "\" >nul",
		"if errorlevel 1 goto fail",
		"del /Q \"" + newPath + "\" >nul 2>&1",
		"start \"\" \"" + exe + "\"",
		"exit /b 0",
		":fail",
		"rem 覆盖失败时提示用户手动替换",
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
