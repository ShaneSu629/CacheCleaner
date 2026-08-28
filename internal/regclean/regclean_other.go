//go:build !windows

package regclean

// Scan 在非 Windows 平台返回空（注册表清理仅支持 Windows）。
func Scan() []Entry { return nil }

// Clean 在非 Windows 平台全部返回失败提示。
func Clean(keys []string) (cleaned []Entry, failed []string) {
	for _, k := range keys {
		failed = append(failed, k+": 注册表清理仅支持 Windows")
	}
	return nil, failed
}
