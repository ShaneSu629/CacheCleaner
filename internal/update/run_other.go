//go:build !windows

package update

import "errors"

// startDetached 非 Windows 平台不支持自更新替换，返回错误。
func startDetached(file, cmdline string) error {
	return errors.New("当前平台不支持自动替换更新，请手动下载新版本")
}
