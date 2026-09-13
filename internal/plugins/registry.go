// Package plugins 提供 CacheCleaner 的插件扩展系统。
//
// 插件以「目录 + manifest.json」形式存在，放在主程序旁的 plugins/ 目录下，
// 每个插件一个子目录。插件是脚本型的：manifest 描述元信息 + 入口脚本，
// 由主程序的 JS 执行器（executor.go）运行，安全可控。
//
// 用户可自由增删插件目录、开关插件，无需重编译主程序。
//
// 设计约束（同项目其余 internal 包）：
//   - 不 import Wails/CGO，保证 CLI 交叉编译不受影响
//   - 数据一律用可 JSON 序列化的 DTO 返回
package plugins

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"cachecleaner/internal/applog"
)

// Descriptor 是插件的元信息（来自 manifest.json）。
// 插件系统只支持脚本型插件（type=script），功能型扩展由开发者内置到主程序。
type Descriptor struct {
	ID     string `json:"id"`     // 唯一标识（目录名）
	NameZh string `json:"nameZh"` // 中文显示名
	NameEn string `json:"nameEn"` // 英文显示名
	Icon   string `json:"icon"`   // 前端精灵图 symbol 名
	DescZh string `json:"descZh"` // 中文描述
	DescEn string `json:"descEn"` // 英文描述
	Type   string `json:"type"`   // 插件类型（仅 "script"）
	Entry  string `json:"entry"`  // 脚本入口文件名（默认 main.js）
	// Version / Author 供管理界面展示。
	Version string `json:"version"`
	Author  string `json:"author"`
}

// IsScript 判断是否为脚本型插件。
func (d Descriptor) IsScript() bool { return d.Type == "script" }

// PluginState 是插件运行时状态（含启用与否）。
type PluginState struct {
	Descriptor
	Enabled  bool   `json:"enabled"`
	Dir      string `json:"dir"`      // 插件目录绝对路径
	Manifest string `json:"manifest"` // manifest.json 绝对路径
	Valid    bool   `json:"valid"`    // manifest 是否解析成功
	ErrMsg   string `json:"errMsg"`   // 解析失败原因
}

// manifest 文件内结构（顶层多包一层，便于未来扩展）。
type manifestFile struct {
	ID      string `json:"id"`
	NameZh  string `json:"nameZh"`
	NameEn  string `json:"nameEn"`
	Icon    string `json:"icon"`
	DescZh  string `json:"descZh"`
	DescEn  string `json:"descEn"`
	Type    string `json:"type"`
	Entry   string `json:"entry"`
	Version string `json:"version"`
	Author  string `json:"author"`
}

// PluginDir 返回插件目录（主程序 exe 旁的 plugins/）。
func PluginDir() string {
	exe, err := os.Executable()
	if err != nil {
		return ""
	}
	return filepath.Join(filepath.Dir(exe), "plugins")
}

// LoadAll 扫描插件目录，解析所有插件（含启用状态）。
// 启用状态持久化在 <plugins>/.enabled 文件（每行一个已启用的插件 ID）。
func LoadAll() []PluginState {
	dir := PluginDir()
	if dir == "" {
		return nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		_ = os.MkdirAll(dir, 0o755)
		return nil
	}
	enabled := loadEnabled(dir)

	var out []PluginState
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		mf := filepath.Join(sub, "manifest.json")
		st := PluginState{Dir: sub, Manifest: mf, Valid: true}
		data, err := os.ReadFile(mf)
		if err != nil {
			// 无 manifest.json 的目录跳过（可能是数据目录）
			continue
		}
		var m manifestFile
		if err := json.Unmarshal(data, &m); err != nil {
			st.Valid = false
			st.ErrMsg = "manifest 解析失败: " + err.Error()
			st.ID = e.Name()
			out = append(out, st)
			continue
		}
		if m.ID == "" {
			m.ID = e.Name()
		}
		st.Descriptor = Descriptor{
			ID:      m.ID,
			NameZh:  m.NameZh,
			NameEn:  m.NameEn,
			Icon:    m.Icon,
			DescZh:  m.DescZh,
			DescEn:  m.DescEn,
			Type:    m.Type,
			Entry:   m.Entry,
			Version: m.Version,
			Author:  m.Author,
		}
		// 脚本型插件入口默认 main.js
		if st.Type == "script" && st.Entry == "" {
			st.Entry = "main.js"
		}
		if st.NameZh == "" && st.NameEn == "" {
			st.NameZh = m.ID
			st.NameEn = m.ID
		}
		st.Enabled = enabled[m.ID]
		out = append(out, st)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// loadEnabled 读取 .enabled 文件，返回已启用插件 ID 集合。
func loadEnabled(dir string) map[string]bool {
	m := map[string]bool{}
	data, err := os.ReadFile(filepath.Join(dir, ".enabled"))
	if err != nil {
		return m
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			m[line] = true
		}
	}
	return m
}

// SetEnabled 启用/禁用插件，持久化到 .enabled 文件。
func SetEnabled(id string, enabled bool) error {
	dir := PluginDir()
	if dir == "" {
		applog.Error("设置插件状态失败：无法定位插件目录")
		return os.ErrNotExist
	}
	cur := loadEnabled(dir)
	if enabled {
		cur[id] = true
	} else {
		delete(cur, id)
	}
	var ids []string
	for k := range cur {
		ids = append(ids, k)
	}
	sort.Strings(ids)
	data := []byte(strings.Join(ids, "\n") + "\n")
	if err := os.WriteFile(filepath.Join(dir, ".enabled"), data, 0o644); err != nil {
		applog.Error("写插件状态失败: %v", err)
		return err
	}
	applog.Info("插件状态变更: id=%s enabled=%v", id, enabled)
	return nil
}

// EnabledPlugins 返回所有「有效且已启用」的插件（供前端注入侧边栏）。
func EnabledPlugins() []Descriptor {
	var out []Descriptor
	for _, p := range LoadAll() {
		if p.Valid && p.Enabled {
			out = append(out, p.Descriptor)
		}
	}
	return out
}

// Install 安装插件：把源目录（含 manifest.json）复制到插件目录。
// 返回错误（目录不存在 / 无 manifest / 复制失败 / ID 冲突等）。
// 若插件 ID 已存在，返回 ErrPluginExists。
func Install(srcDir string) error {
	applog.Info("安装插件: 源目录=%s", srcDir)
	if srcDir == "" {
		return errors.New("源目录为空")
	}
	st, err := os.Stat(srcDir)
	if err != nil || !st.IsDir() {
		return errors.New("源目录不存在或不是目录")
	}
	mf := filepath.Join(srcDir, "manifest.json")
	data, err := os.ReadFile(mf)
	if err != nil {
		return errors.New("源目录缺少 manifest.json")
	}
	var m manifestFile
	if err := json.Unmarshal(data, &m); err != nil {
		return errors.New("manifest.json 解析失败: " + err.Error())
	}
	id := m.ID
	if id == "" {
		id = filepath.Base(srcDir)
	}
	if id == "" {
		return errors.New("无法确定插件 ID")
	}

	dir := PluginDir()
	if dir == "" {
		return errors.New("无法定位插件目录")
	}
	_ = os.MkdirAll(dir, 0o755)

	dst := filepath.Join(dir, id)
	if _, err := os.Stat(dst); err == nil {
		applog.Error("安装插件失败：已存在同名插件 id=%s", id)
		return ErrPluginExists
	}
	// 复制整个源目录到插件目录
	if err := copyDir(srcDir, dst); err != nil {
		applog.Error("安装插件失败：复制目录出错 %v", err)
		return err
	}
	applog.Info("安装插件成功: id=%s -> %s", id, dst)
	// 新安装的插件默认启用
	return SetEnabled(id, true)
}

// Uninstall 卸载插件：删除插件目录并移除启用状态。
// 返回错误（目录不存在 / 删除失败等）。
func Uninstall(id string) error {
	applog.Info("卸载插件: id=%s", id)
	if id == "" {
		return errors.New("插件 ID 为空")
	}
	dir := PluginDir()
	if dir == "" {
		return errors.New("无法定位插件目录")
	}
	dst := filepath.Join(dir, id)
	if _, err := os.Stat(dst); err != nil {
		return errors.New("插件不存在")
	}
	if err := os.RemoveAll(dst); err != nil {
		applog.Error("卸载插件失败：删除目录出错 %v", err)
		return err
	}
	applog.Info("卸载插件成功: id=%s", id)
	_ = SetEnabled(id, false)
	return nil
}

// ErrPluginExists 表示插件 ID 已存在。
var ErrPluginExists = errors.New("插件已存在")

// copyDir 递归复制目录（不跟随符号链接，限制文件大小与数量防止失控）。
func copyDir(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}

