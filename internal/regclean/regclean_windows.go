//go:build windows

package regclean

import (
	"strings"

	"cachecleaner/internal/model"

	"golang.org/x/sys/windows/registry"
)

// HivePrefix 是本包支持的唯一根键前缀（全部目标均为当前用户 HKCU，不需要管理员权限）。
const HivePrefix = `HKCU\`

// def 定义一个已知的注册表垃圾位置（白名单，清理范围严格受限于此表）。
type def struct {
	sub      string // HKCU 下的相对键路径
	category string
	risk     model.Risk
	desc     string
}

// defs 已知注册表垃圾位置清单：
// MRU（最近使用记录）/ MuiCache / 托盘通知历史等，均为 Windows 会自动重建的衍生数据。
var defs = []def{
	{`Software\Microsoft\Windows\CurrentVersion\Explorer\RunMRU`, "注册表历史", model.RiskSafe, "运行对话框(Win+R)输入历史"},
	{`Software\Microsoft\Windows\CurrentVersion\Explorer\RecentDocs`, "注册表历史", model.RiskSafe, "最近打开文档历史"},
	{`Software\Microsoft\Windows\CurrentVersion\Explorer\TypedPaths`, "注册表历史", model.RiskSafe, "资源管理器地址栏输入历史"},
	{`Software\Microsoft\Windows\CurrentVersion\Explorer\ComDlg32\OpenSavePidlMRU`, "注册表历史", model.RiskSafe, "打开/保存对话框文件名历史"},
	{`Software\Microsoft\Windows\CurrentVersion\Explorer\ComDlg32\LastVisitedPidlMRU`, "注册表历史", model.RiskSafe, "打开/保存对话框位置历史"},
	{`Software\Microsoft\Windows\CurrentVersion\Explorer\Map Network Drive MRU`, "注册表历史", model.RiskSafe, "映射网络驱动器历史"},
	{`Software\Classes\Local Settings\Software\Microsoft\Windows\Shell\MuiCache`, "注册表缓存", model.RiskSafe, "程序显示名称缓存"},
	{`Software\Classes\Local Settings\Software\Microsoft\Windows\CurrentVersion\TrayNotify`, "注册表缓存", model.RiskCaution, "托盘图标通知历史(清理后已隐藏图标设置会重置)"},
}

// Scan 扫描所有已知注册表垃圾位置，返回存在且有值的项。
func Scan() []Entry {
	var out []Entry
	for _, d := range defs {
		k, err := registry.OpenKey(registry.CURRENT_USER, d.sub, registry.READ)
		if err != nil {
			continue // 键不存在 = 无垃圾，跳过
		}
		names, err := k.ReadValueNames(-1)
		if err != nil {
			k.Close()
			continue
		}
		var size int64
		for _, name := range names {
			// buf 传 nil 时 GetValue 返回值所需字节数（附带 ErrShortBuffer，忽略即可）
			if n, _, err := k.GetValue(name, nil); err == nil || n > 0 {
				size += int64(n)
			}
		}
		k.Close()
		if len(names) > 0 {
			out = append(out, Entry{
				Key:      HivePrefix + d.sub,
				Category: d.category,
				Risk:     d.risk,
				Desc:     d.desc,
				Values:   len(names),
				Size:     size,
			})
		}
	}
	return out
}

// Clean 清理选中的注册表项。
// 安全约束：只接受白名单（defs）中的键；仅删除键下的值，保留键本身，Windows 会自动重建。
// 返回实际清理成功的条目（供调用方写清理历史）与失败列表（键 + 错误信息）。
func Clean(keys []string) (cleaned []Entry, failed []string) {
	defBySub := make(map[string]def, len(defs))
	for _, d := range defs {
		defBySub[d.sub] = d
	}
	for _, key := range keys {
		sub := strings.TrimPrefix(key, HivePrefix)
		d, ok := defBySub[sub]
		if !ok {
			failed = append(failed, key+": 非白名单键，拒绝清理")
			continue
		}
		k, err := registry.OpenKey(registry.CURRENT_USER, sub, registry.READ|registry.SET_VALUE)
		if err != nil {
			failed = append(failed, key+": "+err.Error())
			continue
		}
		names, err := k.ReadValueNames(-1)
		if err != nil {
			k.Close()
			failed = append(failed, key+": "+err.Error())
			continue
		}
		var size int64
		var firstErr error
		for _, name := range names {
			if n, _, err := k.GetValue(name, nil); err == nil || n > 0 {
				size += int64(n)
			}
			if err := k.DeleteValue(name); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		k.Close()
		if firstErr != nil {
			failed = append(failed, key+": "+firstErr.Error())
			continue
		}
		cleaned = append(cleaned, Entry{
			Key:      HivePrefix + sub,
			Category: d.category,
			Risk:     d.risk,
			Desc:     d.desc,
			Values:   len(names),
			Size:     size,
		})
	}
	return
}
