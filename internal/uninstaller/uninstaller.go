// Package uninstaller 实现主程序内置功能「软件卸载」：
// 枚举已安装软件、Geek 式深度卸载 + 残留扫描 + 国内流氓软件（PUP）识别。
//
// 这是内置功能（非插件），由 app.go 直接绑定为 Wails 方法。
package uninstaller

import "sort"

// App 是一条已安装软件记录（JSON 序列化给前端）。
type App struct {
	Key         string `json:"key"`         // 注册表键路径（卸载时精确定位）
	Name        string `json:"name"`        // 显示名
	Version     string `json:"version"`     // 版本号
	Publisher   string `json:"publisher"`   // 发布者
	InstallDate string `json:"installDate"` // 安装日期
	InstallDir  string `json:"installDir"`  // 安装目录（用于残留扫描）
	Size        int64  `json:"size"`        // 安装目录体积
	Uninstall   string `json:"uninstall"`   // 卸载命令（静默化后执行）
	PUP         bool   `json:"pup"`         // 是否流氓软件
	PUPReason   string `json:"pupReason"`   // 流氓特征描述
	Is64        bool   `json:"is64"`        // 注册表视图
	HKCU        bool   `json:"hcu"`         // 是否 HKCU 项
}

// UninstallResult 是卸载执行结果。
type UninstallResult struct {
	Success    bool     `json:"success"`
	Message    string   `json:"message"`
	Residues   []string `json:"residues"`   // 残留文件/目录
	RegResidue []string `json:"regResidue"` // 残留注册表项
}

// Service 是软件卸载能力实现（方法直接暴露给 Wails 绑定）。
type Service struct{}

// sortApps 按 PUP 优先 + 名称排序（流氓软件置顶）。
func sortApps(apps []App) {
	sort.SliceStable(apps, func(i, j int) bool {
		if apps[i].PUP != apps[j].PUP {
			return apps[i].PUP
		}
		return apps[i].Name < apps[j].Name
	})
}
