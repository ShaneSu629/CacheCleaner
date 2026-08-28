package db

import (
	"io/fs"
	"os"
	"path/filepath"
	"time"

	"cachecleaner/internal/config"
)

// ScanDir 递归统计目录大小、文件数与最近修改时间。
// 使用 WalkDir 而非 Walk：WalkDir 直接使用 ReadDir 返回的 DirEntry，
// 无需对每个条目额外调用 Lstat，大目录扫描性能显著更优。
func ScanDir(path string) (size int64, fc int64, la time.Time) {
	info, err := os.Lstat(path)
	if err != nil {
		return
	}
	la = info.ModTime()
	_ = filepath.WalkDir(path, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if mt, e := d.Info(); e == nil && mt.ModTime().After(la) {
				la = mt.ModTime()
			}
			return nil
		}
		if fi, e := d.Info(); e == nil {
			size += fi.Size()
			fc++
			if fi.ModTime().After(la) {
				la = fi.ModTime()
			}
		}
		return nil
	})
	return
}

// ShortOf 将绝对路径转换为相对用户主目录的短路径，便于展示。
func ShortOf(cfg *config.Config, full string) string {
	rel, err := filepath.Rel(cfg.Home, full)
	if err != nil || rel == ".." || filepath.IsAbs(rel) {
		return full
	}
	return rel
}
