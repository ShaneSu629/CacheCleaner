package plugins

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
	"unicode/utf8"

	"cachecleaner/internal/applog"

	"github.com/dop251/goja"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// ScriptResult 是脚本插件执行后返回给前端的结果。
// 脚本通过 cc.render({...}) 填充 Title/Text/HTML，通过 cc.log 填充 Logs。
type ScriptResult struct {
	Title string   `json:"title"`
	Text  string   `json:"text"`
	HTML  string   `json:"html"`
	Logs  []string `json:"logs"`
}

// RunScript 执行脚本型插件的入口脚本（默认 main.js），返回渲染结果。
// 每次调用创建独立 goja 运行时，无共享状态，可并发安全。
func RunScript(d Descriptor) (*ScriptResult, error) {
	applog.Info("执行脚本插件: %s (entry=%s)", d.ID, d.Entry)
	entry := d.Entry
	if entry == "" {
		entry = "main.js"
	}
	// 脚本路径：插件目录/<entry>。插件目录从 PluginDir/<id> 推导。
	dir := filepath.Join(PluginDir(), d.ID)
	srcPath := filepath.Join(dir, entry)
	data, err := os.ReadFile(srcPath)
	if err != nil {
		applog.Error("读取脚本失败: %s -> %v", srcPath, err)
		return nil, fmt.Errorf("读取脚本失败: %v", err)
	}

	vm := goja.New()
	res := &ScriptResult{}

	// cc API：脚本里可用的全局对象。
	cc := map[string]interface{}{
		"log": func(msg string) {
			applog.Info("[插件:%s] %s", d.ID, msg)
			res.Logs = append(res.Logs, msg)
		},
		"render": func(v map[string]interface{}) {
			if t, ok := v["title"].(string); ok {
				res.Title = t
			}
			if t, ok := v["text"].(string); ok {
				res.Text = t
			}
			if h, ok := v["html"].(string); ok {
				res.HTML = h
			}
		},
		"exec": func(cmd string, timeoutMs int) map[string]interface{} {
			return ccExec(cmd, timeoutMs)
		},
		"readFile": func(p string) string {
			full := resolvePath(dir, p)
			b, err := os.ReadFile(full)
			if err != nil {
				applog.Error("[插件:%s] 读文件失败 %s -> %v", d.ID, full, err)
				return ""
			}
			return string(b)
		},
		"writeFile": func(p, content string) bool {
			full := resolvePath(dir, p)
			if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
				applog.Error("[插件:%s] 建目录失败 %v", d.ID, err)
				return false
			}
			if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
				applog.Error("[插件:%s] 写文件失败 %s -> %v", d.ID, full, err)
				return false
			}
			return true
		},
		"env": func(name string) string { return os.Getenv(name) },
		"cwd": func() string { return dir },
		"dataDir": func() string {
			dd := filepath.Join(dir, "data")
			_ = os.MkdirAll(dd, 0o755)
			return dd
		},
		"home":     func() string { h, _ := os.UserHomeDir(); return h },
		"platform": func() string { return runtime.GOOS },
	}
	_ = vm.Set("cc", cc)

	// 执行脚本；脚本抛异常则整体失败。
	if _, err := vm.RunString(string(data)); err != nil {
		applog.Error("脚本执行失败: %s -> %v", d.ID, err)
		return res, fmt.Errorf("脚本执行失败: %v", err)
	}
	if res.Title == "" {
		res.Title = d.NameZh
	}
	applog.Info("脚本执行完成: %s", d.ID)
	return res, nil
}

// resolvePath 解析脚本里传入的路径：绝对路径原样返回，相对路径相对插件目录。
func resolvePath(pluginDir, p string) string {
	if filepath.IsAbs(p) {
		return p
	}
	return filepath.Join(pluginDir, p)
}

// ccExec 执行系统命令，返回 {code, stdout, stderr}。
// Windows 用 cmd /c，其余用 sh -c。timeoutMs<=0 时默认 60s。
func ccExec(cmd string, timeoutMs int) map[string]interface{} {
	if timeoutMs <= 0 {
		timeoutMs = 60000
	}
	var c *exec.Cmd
	if runtime.GOOS == "windows" {
		c = exec.Command("cmd", "/c", cmd)
		// 隐藏 cmd 控制台窗口：否则每次插件执行系统命令都会闪出一个黑框
		hideConsoleWindow(c)
	} else {
		c = exec.Command("sh", "-c", cmd)
	}
	var stdout, stderr strings.Builder
	c.Stdout = &stdout
	c.Stderr = &stderr

	if err := c.Start(); err != nil {
		return map[string]interface{}{
			"code": -1, "stdout": "", "stderr": "启动失败: " + err.Error(),
		}
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	code := 0
	select {
	case err := <-done:
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				code = ee.ExitCode()
			} else {
				code = -1
			}
		}
	case <-time.After(time.Duration(timeoutMs) * time.Millisecond):
		_ = c.Process.Kill()
		<-done
		return map[string]interface{}{
			"code": -1, "stdout": stdout.String(), "stderr": "命令超时（" + fmt.Sprint(timeoutMs) + "ms）",
		}
	}
	return map[string]interface{}{
		"code": code, "stdout": toUTF8(stdout.String()), "stderr": toUTF8(stderr.String()),
	}
}

// toUTF8 把命令输出转为 UTF-8。中文 Windows 下 cmd 输出常是 GBK（代码页 936），
// 直接按 UTF-8 读会乱码（如"版本"→"�汾"）。这里检测：若已是合法 UTF-8 则原样返回，
// 否则按 GBK 解码。保证插件的 cc.exec 拿到正确中文。
func toUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	if out, err := simplifiedchinese.GBK.NewDecoder().String(s); err == nil {
		return out
	}
	return s
}
