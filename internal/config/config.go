package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
)

// Base 表示缓存路径所基于的根目录锚点。
type Base int

const (
	BaseHome Base = iota // 用户主目录 ~
	BaseLocal            // 本地数据：Win %LocalAppData% / mac ~/Library/Caches / linux ~/.cache
	BaseRoaming          // 配置数据：Win %AppData% / mac ~/Library/Application Support / linux ~/.config
	BaseLocalLow         // Win %AppData%\LocalLow（仅 Windows 有效）
)

// Config 保存运行期配置与各系统基路径。
type Config struct {
	Home       string
	Local      string
	Roaming    string
	LocalLow   string
	OS         string
	ExcludeDirs []string
	CustomDirs  []string
	ConfigFile  string
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
	}
	return c.Home
}
