package db

import (
	"os"
	"path/filepath"
	"time"

	"cachecleaner/internal/config"
)

// ScanDir 递归统计目录大小、文件数与最近访问时间。
func ScanDir(path string) (size int64, fc int64, la time.Time) {
	info, err := os.Lstat(path)
	if err != nil {
		return
	}
	la = info.ModTime()
	_ = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if fi.IsDir() {
			if fi.ModTime().After(la) {
				la = fi.ModTime()
			}
			return nil
		}
		size += fi.Size()
		fc++
		if fi.ModTime().After(la) {
			la = fi.ModTime()
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
