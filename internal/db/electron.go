package db

// ElectronSafeDirs 是 Electron/Chromium 套壳工具会产生、且可安全删除的纯缓存子目录名。
// 命中这些名字即可判定为可清理缓存，无需知道工具叫什么名字（结构识别核心）。
var ElectronSafeDirs = map[string]bool{
	"Cache":                true,
	"Cache_Data":           true,
	"GPUCache":             true,
	"Code Cache":           true,
	"DawnGraphiteCache":    true,
	"DawnWebGPUCache":      true,
	"GrShaderCache":        true,
	"blob_storage":         true,
	"Crashpad":             true,
	"logs":                 true,
	"CachedData":           true,
	"CachedExtensions":     true,
	"CachedExtensionVSIXs": true,
}

// ElectronExcludeDirs 是同目录下严禁删除的敏感子目录（登录态/凭据/配置/会话）。
var ElectronExcludeDirs = map[string]bool{
	"Cookies":            true,
	"Local State":        true,
	"Preferences":        true,
	"IndexedDB":          true,
	"Local Storage":      true,
	"Session Storage":    true,
	"Service Worker":     true,
	"Network":            true,
	"User Data":          true,
	"Default":            true,
	"databases":          true,
	"History":            true,
	"Bookmarks":          true,
	"Login Data":         true,
	"Web Data":           true,
	"Extensions":         true,
}

// IsElectronSafe 判断目录名是否为可安全清理的 Electron 缓存目录。
func IsElectronSafe(name string) bool {
	return ElectronSafeDirs[name]
}

// IsElectronExcluded 判断目录名是否属于严禁删除的敏感目录。
func IsElectronExcluded(name string) bool {
	return ElectronExcludeDirs[name]
}
