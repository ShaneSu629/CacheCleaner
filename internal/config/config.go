package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// Base 表示缓存路径所基于的根目录锚点。
type Base int

const (
	BaseHome Base = iota // 用户主目录 ~
	BaseLocal            // 本地数据：Win %LocalAppData% / mac ~/Library/Caches / linux ~/.cache
	BaseRoaming          // 配置数据：Win %AppData% / mac ~/Library/Application Support / linux ~/.config
	BaseLocalLow         // Win %AppData%\LocalLow（仅 Windows 有效）
	BaseProgramData      // Win C:\ProgramData（全系统共享应用数据，仅 Windows 有效）
	BaseWindows          // Win 系统目录（如 C:\Windows，仅 Windows 有效）
)

// Config 保存运行期配置与各系统基路径。
type Config struct {
	Home        string
	Local       string
	Roaming     string
	LocalLow    string
	ProgramData string
	Windows     string
	OS          string

	ExcludeDirs []string
	CustomDirs  []string
	ConfigFile  string
}

// Default 返回基于系统环境的默认配置（Load 失败时的兜底，保证 cfg 永不为 nil）。
func Default() *Config {
	cfg, _ := Load()
	return cfg
}

// Load 初始化配置，并尝试从用户配置目录加载自定义排除/目录。
func Load() (*Config, error) {
	home, _ := os.UserHomeDir()
	cache, _ := os.UserCacheDir()
	conf, _ := os.UserConfigDir()

	cfg := &Config{
		Home:    home,
		Local:   cache,
		Roaming: conf,
		OS:      runtime.GOOS,
	}
	if runtime.GOOS == "windows" {
		cfg.LocalLow = filepath.Join(home, "AppData", "LocalLow")
		if pd := os.Getenv("ProgramData"); pd != "" {
			cfg.ProgramData = pd
		}
		if w := os.Getenv("SystemRoot"); w != "" {
			cfg.Windows = w
		} else if w := os.Getenv("windir"); w != "" {
			cfg.Windows = w
		}
	}
	if conf != "" {
		cfg.ConfigFile = filepath.Join(conf, "CacheCleaner", "config.json")
	}

	if cfg.ConfigFile != "" {
		if data, err := os.ReadFile(cfg.ConfigFile); err == nil {
			var u struct {
				ExcludeDirs []string `json:"excludeDirs"`
				CustomDirs  []string `json:"customDirs"`
			}
			if json.Unmarshal(data, &u) == nil {
				cfg.ExcludeDirs = u.ExcludeDirs
				cfg.CustomDirs = u.CustomDirs
			}
		}
	}
	if cfg.ExcludeDirs == nil {
		cfg.ExcludeDirs = []string{}
	}
	if cfg.CustomDirs == nil {
		cfg.CustomDirs = []string{}
	}
	return cfg, nil
}

// Save 持久化用户自定义排除/目录。
func (c *Config) Save() error {
	if c.ConfigFile == "" {
		return nil
	}
	dir := filepath.Dir(c.ConfigFile)
	_ = os.MkdirAll(dir, 0o755)
	u := struct {
		ExcludeDirs []string `json:"excludeDirs"`
		CustomDirs  []string `json:"customDirs"`
	}{c.ExcludeDirs, c.CustomDirs}
	data, err := json.MarshalIndent(u, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(c.ConfigFile, data, 0o644)
}

// Resolve 返回指定基锚点的绝对路径。
func (c *Config) Resolve(base Base) string {
	switch base {
	case BaseHome:
		return c.Home
	case BaseLocal:
		return c.Local
	case BaseRoaming:
		return c.Roaming
	case BaseLocalLow:
		return c.LocalLow
	case BaseProgramData:
		return c.ProgramData
	case BaseWindows:
		return c.Windows
	}
	return c.Home
}

// IsExcluded 判断路径是否被用户排除项命中（扫描与清理共用同一语义）。
// 排除项支持两种形式（均大小写不敏感、正反斜杠通用）：
//  1. 名称形式（如 "logs"、"User Data"）：需与目标路径的某个完整路径段/段后缀相等才命中，
//     不做子串匹配，避免 "logs" 误伤 "Catalogs"、"Temp" 误伤 "Temporary" 这类路径；
//  2. 绝对路径形式（如 C:\Users\me\.ollama）：目标位于该目录之下（含自身）才命中。
func (c *Config) IsExcluded(path string) bool {
	if len(c.ExcludeDirs) == 0 || path == "" {
		return false
	}
	pl := normalize(path)
	// 前后各补一个分隔符：使匹配始终以"完整路径段"为单位，
	// 既能命中首段（C:\logs\a）、中间段（…\user data\Default\Cache）与尾段，
	// 又不会把 "logs" 误匹配到 "Catalogs"、"temp" 误匹配到 "Temporary"。
	padded := "/" + pl + "/"
	for _, ex := range c.ExcludeDirs {
		ex = strings.TrimSpace(ex)
		if ex == "" {
			continue
		}
		exc := normalize(ex)
		if strings.Contains(padded, "/"+exc+"/") {
			return true
		}
	}
	return false
}

// normalize 统一路径：小写 + 正斜杠 + Clean，用于大小写不敏感比较。
func normalize(p string) string {
	return strings.ToLower(filepath.ToSlash(filepath.Clean(p)))
}
