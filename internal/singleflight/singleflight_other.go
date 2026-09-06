//go:build !windows

package singleflight

// Acquire 非 Windows 平台无单实例锁，直接放行（多开无害）。
func Acquire(name, windowTitle string) bool { return true }
