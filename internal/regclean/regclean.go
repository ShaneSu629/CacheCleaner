// Package regclean 提供注册表垃圾项的扫描与清理。
//
// 设计原则与文件缓存一致：只处理"已知位置 + 风险分级"，绝不做开放式注册表搜索，
// 清理仅限白名单键（defs 中定义），且只删值不删键——Windows 会在需要时自动重建。
// 仅支持 Windows；其他平台 Scan 返回空、Clean 返回失败提示。
package regclean

import "cachecleaner/internal/model"

// Entry 是一条可清理的注册表垃圾项。
type Entry struct {
	Key      string     // 完整键路径，如 HKCU\Software\Microsoft\Windows\CurrentVersion\Explorer\RunMRU
	Category string     // 分类（注册表历史 / 注册表缓存）
	Risk     model.Risk // 风险等级
	Desc     string     // 描述
	Values   int        // 键下的值数量
	Size     int64      // 值数据总字节数（近似，用于展示与统计）
}
