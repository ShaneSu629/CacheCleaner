package model

import "time"

// Risk 表示缓存项的清理风险等级。
type Risk string

const (
	RiskSafe    Risk = "Safe"    // 纯缓存，可无脑清理，重建不影响登录/数据
	RiskCaution Risk = "Caution" // 含对话历史/索引/项目状态，清理会丢失但可重建，需用户确认
	RiskReview  Risk = "Review"  // 含登录态/凭据/核心配置/大模型权重，默认排除，需人工复核
)

// CacheEntry 表示一条可被清理的缓存记录。
type CacheEntry struct {
	Path       string
	ShortPath  string
	Category   string
	Risk       Risk
	Desc       string
	Size       int64
	FileCount  int64
	LastAccess time.Time
}

// RiskColor 返回该风险等级对应的中文标签与颜色（用于终端展示）。
func (r Risk) Label() string {
	switch r {
	case RiskSafe:
		return "安全"
	case RiskCaution:
		return "谨慎"
	case RiskReview:
		return "复核"
	default:
		return "未知"
	}
}
