//go:build !windows

package uninstaller

// ListApps 非 Windows 平台无注册表卸载项，返回空。
func (p *Service) ListApps() []App { return nil }

// Uninstall 非 Windows 平台不支持。
func (p *Service) Uninstall(key string) UninstallResult {
	return UninstallResult{Success: false, Message: "软件卸载插件仅支持 Windows"}
}

// ForceUninstall 非 Windows 平台不支持。
func (p *Service) ForceUninstall(key string) UninstallResult {
	return UninstallResult{Success: false, Message: "软件卸载插件仅支持 Windows"}
}
