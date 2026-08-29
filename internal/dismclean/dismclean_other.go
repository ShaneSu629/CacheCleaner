//go:build !windows

package dismclean

import "errors"

var errUnsupported = errors.New("组件存储清理仅支持 Windows")

// Available 在非 Windows 平台恒为 false。
func Available() bool { return false }

// IsElevated 在非 Windows 平台恒为 false。
func IsElevated() bool { return false }

// StartAnalyze 在非 Windows 平台返回不支持错误。
func StartAnalyze() error { return errUnsupported }

// StartCleanup 在非 Windows 平台返回不支持错误。
func StartCleanup() error { return errUnsupported }

// Poll 在非 Windows 平台返回零值。
func Poll() Status { return Status{} }

// AnalyzeReport 在非 Windows 平台返回 nil。
func AnalyzeReport() *Report { return nil }
