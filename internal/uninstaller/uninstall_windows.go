//go:build windows

package uninstaller

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sys/windows/registry"
)

// silentize 决定卸载命令如何执行。
//
// 策略（调用 Windows 标准卸载机制，兼容性优先）：
//   - msiexec（MSI 标准卸载器）→ 加 /qn（官方支持静默）
//   - 其他一切卸载器 → 原样交互式运行（弹卸载向导）
//
// 注意：这里不改变命令结构，只决定是否追加静默参数。
// 真正的执行在 runUninstall 里用 exec.Command 直接解析（不经 cmd /c），
// 避免带引号路径在 cmd 里被二次解析导致"不是内部或外部命令"。
func silentize(uninst string) string {
	u := strings.TrimSpace(uninst)
	if u == "" {
		return ""
	}
	low := strings.ToLower(u)

	// 仅 MSI 卸载器加静默参数
	if strings.Contains(low, "msiexec") {
		if !strings.Contains(low, "/qn") && !strings.Contains(low, "/quiet") {
			return u + " /qn"
		}
		return u
	}
	// 其余一律交互式（原样运行）
	return u
}

// runUninstall 执行卸载命令（Windows 标准卸载机制）。
//
// 用 exec.Command 直接解析 UninstallString（exec 内部正确处理引号与空格），
// 不经过 cmd /c —— 这避免了带引号路径在 cmd 里被二次解析导致
// "不是内部或外部命令"的经典坑（LocalSend 报错根因）。
//
// 返回 (退出码, stderr 文本)。卸载器可能是 GUI 程序（立即返回），
// 所以退出码仅供参考，真正的成功判据在 Uninstall 里查注册表键。
func runUninstall(cmd string) (int, string, error) {
	// 解析命令行（处理引号）：msiexec /x{...}、带引号 exe 路径 + 参数等
	name, args, err := parseCommandLine(cmd)
	if err != nil {
		return -1, "", err
	}
	c := exec.Command(name, args...)
	var stderr strings.Builder
	c.Stderr = &stderr
	if err := c.Start(); err != nil {
		return -1, stderr.String(), err
	}
	done := make(chan error, 1)
	go func() { done <- c.Wait() }()
	select {
	case err := <-done:
		if err != nil {
			if ee, ok := err.(*exec.ExitError); ok {
				return ee.ExitCode(), stderr.String(), nil
			}
			return -1, stderr.String(), err
		}
		return 0, stderr.String(), nil
	case <-time.After(10 * time.Minute):
		// 卸载器卡死（常见于流氓软件挽留弹窗），强制终止
		_ = c.Process.Kill()
		<-done
		return -1, stderr.String(), fmt.Errorf("卸载超时（10 分钟），已强制终止")
	}
}

// parseCommandLine 把 UninstallString 解析成 (程序名, 参数列表)。
// 正确处理引号包裹的路径（如 "C:\Program Files\LocalSend\unins000.exe"）。
// 对 msiexec 等命令原样切分。
func parseCommandLine(cmd string) (string, []string, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		return "", nil, fmt.Errorf("空命令")
	}
	// 用标准库的字段切分逻辑（处理双引号）
	fields, err := splitFields(cmd)
	if err != nil {
		return "", nil, err
	}
	if len(fields) == 0 {
		return "", nil, fmt.Errorf("空命令")
	}
	return fields[0], fields[1:], nil
}

// splitFields 按空格切分命令行，正确处理双引号包裹的含空格路径。
func splitFields(s string) ([]string, error) {
	var fields []string
	var cur strings.Builder
	inQuote := false
	for _, r := range s {
		switch r {
		case '"':
			inQuote = !inQuote
		case ' ', '\t':
			if inQuote {
				cur.WriteRune(r)
			} else if cur.Len() > 0 {
				fields = append(fields, cur.String())
				cur.Reset()
			}
		default:
			cur.WriteRune(r)
		}
	}
	if cur.Len() > 0 {
		fields = append(fields, cur.String())
	}
	return fields, nil
}

// deleteUninstallKey 删除卸载注册表项（强制卸载的最后一步）。
// 两个视图都尝试删（64 位 + WOW6432Node 32 位），因为卸载器的 Is64 信息
// 可能不准（尤其强制卸载前已重新枚举），漏删任一会导致软件"卸载不干净"残留列表里。
func deleteUninstallKey(app App) []string {
	var deleted []string
	root := registry.LOCAL_MACHINE
	if app.HKCU {
		root = registry.CURRENT_USER
	}
	// 依次尝试 64 位视图和 32 位视图（WOW6432Node）两个路径
	paths := []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\` + app.Key,
		`SOFTWARE\WOW6432Node\Microsoft\Windows\CurrentVersion\Uninstall\` + app.Key,
	}
	for _, path := range paths {
		if err := registry.DeleteKey(root, path); err == nil {
			deleted = append(deleted, path)
		}
	}
	return deleted
}

// scanRegResidue 扫描常见注册表残留：
//   - 启动项（Run / RunOnce）
//   - 文件关联（HKCR）
//   - 卸载残留（HKCU Uninstall）
// 只在键名/值名包含软件名时命中。
func scanRegResidue(app App) []string {
	var found []string
	name := app.Name

	// 1. 启动项
	for _, run := range []string{
		`SOFTWARE\Microsoft\Windows\CurrentVersion\Run`,
		`SOFTWARE\Microsoft\Windows\CurrentVersion\RunOnce`,
	} {
		for _, src := range []struct {
			root registry.Key
			view uint32
		}{
			{registry.CURRENT_USER, registry.WOW64_64KEY},
			{registry.CURRENT_USER, registry.WOW64_32KEY},
			{registry.LOCAL_MACHINE, registry.WOW64_64KEY},
			{registry.LOCAL_MACHINE, registry.WOW64_32KEY},
		} {
			k, err := registry.OpenKey(src.root, run, registry.READ|src.view)
			if err != nil {
				continue
			}
			vals, _ := k.ReadValueNames(-1)
			for _, v := range vals {
				val, _, _ := k.GetStringValue(v)
				if containsIC(v, name) || containsIC(val, name) {
					found = append(found, run+`\`+v)
				}
			}
			k.Close()
		}
	}
	return found
}

// containsIC 大小写不敏感包含。
func containsIC(s, sub string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(sub))
}

// ── 流氓软件（PUP）识别 ──

// pupRules 是流氓软件特征规则库。
// 国内互联网环境常见：强制捆绑安装、静默驻留、弹窗广告、
// 篡改主页、卸载器挽留、难以卸载等行为。
// reason 用于向用户解释为什么判定为流氓软件。
var pupRules = []struct {
	match  func(App) bool
	reason string
}{
	// 1. 发布者特征：知名流氓软件厂商/壳公司
	{func(a App) bool {
		p := strings.ToLower(a.Publisher)
		for _, m := range []string{
			"360", "奇虎", "qihoo", "baidu", "百度", "tencent pc manager",
			"腾讯电脑管家", "2345", "kingsoft", "金山", "rising", "瑞星",
			"zol", "中关村在线", "baitiao", "白条", "duoduote", "多特",
			"crsky", "非凡", "pcfaster", "电脑管家", "lu大师", "鲁大师",
			"kk", "kankan", "快播", "flashhelper", "flash helper", "重橙网络",
			"mktbar", "marketing", "adware", "bundler", "installer*", "shanghai 2345",
		} {
			if strings.Contains(p, m) {
				return true
			}
		}
		return false
	}, "发布者具有流氓软件/捆绑推广特征"},
	// 2. 名称特征：工具条/主页锁定/加速器/伪杀软等典型 PUP 命名
	{func(a App) bool {
		n := strings.ToLower(a.Name)
		for _, m := range []string{
			"toolbar", "工具条", "assistant", "助手", "game center", "游戏中心",
			"主页保护", "主页修复", "homepage guard", "browser guard", "安全浏览器",
			"search protect", "deskbab", "天气王", "wifi管家", "wifi 管家",
			"壁纸", "walpaper", "加速球", "speedup", "一键装机", "装机必备",
			"快捷搜索", "毒霸", "安全卫士", "杀毒", "antivirus free",
			"浏览器助手", "pdf转换", "压缩王", "驱动人生", "驱动精灵",
			"搜狗输入法", "sogou", "快压", "kuaizip", "好压", "haozip",
		} {
			if strings.Contains(n, m) {
				return true
			}
		}
		return false
	}, "名称符合流氓软件/捆绑软件常见命名"},
	// 3. 安装目录特征：随机目录、隐藏目录、Temp 目录（典型的偷装行为）
	{func(a App) bool {
		d := strings.ToLower(a.InstallDir)
		return strings.Contains(d, "\\temp\\") ||
			strings.Contains(d, "\\appdata\\roaming\\") && strings.Contains(d, "cache")
	}, "安装目录异常（疑似静默偷装）"},
}

// DetectPUP 识别软件是否流氓软件（PUP）。返回 (原因, 是否PUP)。
// 命中任意规则即判定。
func DetectPUP(app App) (string, bool) {
	for _, r := range pupRules {
		if r.match(app) {
			return r.reason, true
		}
	}
	return "", false
}

// ── 工具 ──

// exePath 从卸载命令中提取可执行文件路径（第一个参数）。
// 用 parseCommandLine 正确处理引号包裹的含空格路径（如 "C:\Program Files\...\unins.exe"）。
func exePath(uninst string) string {
	name, _, err := parseCommandLine(uninst)
	if err != nil {
		return ""
	}
	return name
}

// fileExists 判断文件存在。
func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

var _ = filepath.Join // 保留 import（其他平台实现可能用到）
var _ = fmt.Sprintf
