// Package update 提供基于 GitHub Releases 的版本检查与更新提醒。
//
// 设计要点：
//   - 只做"检查 + 提醒 + 引导下载"，不自动下载替换二进制。
//     原因：github.com/.../releases/download 会 302 到 objects.githubusercontent.com，
//     该域名在国内实测不可达（HTTP=000 超时），而 api.github.com 正常。
//     因此查版本走 API（通），实际下载交给用户浏览器打开 Release 页。
//   - 支持"稍后提醒"与"跳过此版本"，避免在用户不想更新时反复打扰。
//   - 所有网络失败都降级为"使用上次缓存的结果 + 返回错误"，绝不阻断启动。
package update

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// version 由构建时注入：
//
//	wails build -ldflags "-X cachecleaner/internal/update.version=v1.0.7"
//
// 未注入时保持 "dev"，此时不参与版本比较（永远不提示更新）。
var version = "dev"

const (
	repoOwner = "ShaneSu629"
	repoName  = "CacheCleaner"
	apiURL    = "https://api.github.com/repos/" + repoOwner + "/" + repoName + "/releases/latest"

	// checkInterval 自动检查的最小间隔，避免每次启动都打 API（未鉴权限流 60 次/小时/IP）。
	checkInterval = 6 * time.Hour
	// defaultSnooze 用户点"稍后提醒"后的静默时长。
	defaultSnooze = 24 * time.Hour

	// assetName 当前平台对应的发布资产名（Windows GUI 产物）。
	assetName = "CacheCleaner-GUI-windows-amd64.exe"
)

// Info 是返回给前端的更新信息。
type Info struct {
	Current     string `json:"current"`
	Latest      string `json:"latest"`
	HasUpdate   bool   `json:"hasUpdate"`
	URL         string `json:"url"`
	Notes       string `json:"notes"`
	Size        int64  `json:"size"`
	PublishedAt string `json:"publishedAt"`
	// FromCache 表示本次结果来自本地缓存（网络不可用或处于检查间隔内）。
	FromCache bool `json:"fromCache"`
	// Snoozed 表示该版本正处于"稍后提醒"静默期内，前端不应弹窗。
	Snoozed bool `json:"snoozed"`
	// Skipped 表示用户已选择跳过此版本，前端不应弹窗。
	Skipped bool `json:"skipped"`
}

// State 是持久化到本地的更新状态（延后、跳过、上次检查结果）。
type State struct {
	LastCheck      time.Time `json:"lastCheck"`
	LatestVersion  string    `json:"latestVersion"`
	LatestURL      string    `json:"latestURL"`
	LatestNotes    string    `json:"latestNotes"`
	LatestSize     int64     `json:"latestSize"`
	LatestPubAt    string    `json:"latestPublishedAt"`
	SnoozeUntil    time.Time `json:"snoozeUntil"`
	SkippedVersion string    `json:"skippedVersion"`
}

// CurrentVersion 返回当前程序版本（构建时注入，未注入则为 "dev"）。
func CurrentVersion() string { return version }

// IsRelease 判断当前程序是否为正式发布版本（dev 版本不提示更新）。
func IsRelease() bool { return isReleaseVersion(version) }

// isReleaseVersion 判断某个版本串是否为正式发布版本。
// 逻辑放在这里（而非直接用 IsRelease）是为了让 applyLocalState 可独立测试。
func isReleaseVersion(v string) bool { return v != "" && v != "dev" }

// ── 状态持久化 ──

// statePath 返回状态文件路径（与 config.json 同目录）。
func statePath() string {
	conf, err := os.UserConfigDir()
	if err != nil || conf == "" {
		return ""
	}
	return filepath.Join(conf, "CacheCleaner", "update.json")
}

func loadState() State {
	var st State
	p := statePath()
	if p == "" {
		return st
	}
	data, err := os.ReadFile(p)
	if err != nil {
		return st
	}
	_ = json.Unmarshal(data, &st)
	return st
}

func saveState(st State) error {
	p := statePath()
	if p == "" {
		return errors.New("无法确定配置目录")
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}

// ── 版本比较 ──

// Compare 比较两个版本号字符串，返回 -1/0/1（a<b / a==b / a>b）。
// 支持可选的 "v" 前缀与预发布后缀（如 v1.0.7-beta 视为低于 v1.0.7）。
// 只比较前三段数字，非数字段按 0 处理，保证不会因格式异常 panic。
func Compare(a, b string) int {
	an, apre := parse(a)
	bn, bpre := parse(b)
	for i := 0; i < 3; i++ {
		if an[i] != bn[i] {
			if an[i] < bn[i] {
				return -1
			}
			return 1
		}
	}
	// 数值相同：带预发布后缀的更小（1.0.7-beta < 1.0.7）
	if apre != bpre {
		if apre == "" {
			return 1
		}
		if bpre == "" {
			return -1
		}
		return strings.Compare(apre, bpre)
	}
	return 0
}

// parse 拆解版本串为三个数字段与预发布后缀。
func parse(v string) ([3]int, string) {
	var nums [3]int
	s := strings.TrimPrefix(strings.TrimSpace(v), "v")
	pre := ""
	if i := strings.IndexAny(s, "-+"); i >= 0 {
		pre = s[i+1:]
		s = s[:i]
	}
	parts := strings.Split(s, ".")
	for i := 0; i < 3 && i < len(parts); i++ {
		n, err := strconv.Atoi(parts[i])
		if err != nil {
			// 非数字片段（如 "1.0.x"）按 0 处理，不 panic
			n = 0
		}
		nums[i] = n
	}
	return nums, pre
}

// ── 检查更新 ──

// release 是 GitHub Releases API 响应中我们关心的字段。
type release struct {
	TagName     string `json:"tag_name"`
	HTMLURL     string `json:"html_url"`
	Body        string `json:"body"`
	PublishedAt string `json:"published_at"`
	Assets      []struct {
		Name string `json:"name"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// Check 检查更新。
//
// force=true 时忽略检查间隔、静默期与跳过标记，立即联网查询（用户手动点击时使用）。
// force=false 时（启动自动检查）会命中缓存/静默策略，避免频繁打 API 与打扰用户。
// 网络或解析失败时，若本地有缓存结果则降级返回缓存（FromCache=true）。
func Check(force bool) (*Info, error) {
	st := loadState()
	cur := CurrentVersion()

	info := &Info{
		Current:     cur,
		Latest:      st.LatestVersion,
		URL:         st.LatestURL,
		Notes:       st.LatestNotes,
		Size:        st.LatestSize,
		PublishedAt: st.LatestPubAt,
		FromCache:   true,
	}

	// dev 版本（未注入版本号）不提示更新，也不联网
	if !IsRelease() {
		return info, nil
	}

	// 沿用缓存结论时，重新计算静默/跳过标记与 HasUpdate
	applyLocalState(info, st)

	needNet := force || time.Since(st.LastCheck) > checkInterval || st.LatestVersion == ""
	if !needNet {
		return info, nil
	}

	rel, err := fetchLatest()
	if err != nil {
		// 联网失败：有缓存就用缓存，没有就把错误抛给调用方（前端静默处理）
		if st.LatestVersion == "" {
			return info, err
		}
		return info, nil
	}

	// 更新缓存
	st.LastCheck = time.Now()
	st.LatestVersion = rel.TagName
	st.LatestNotes = rel.Body
	st.LatestPubAt = rel.PublishedAt
	st.LatestSize = 0
	if rel.HTMLURL != "" {
		st.LatestURL = rel.HTMLURL
	} else {
		st.LatestURL = fmt.Sprintf("https://github.com/%s/%s/releases/latest", repoOwner, repoName)
	}
	for _, a := range rel.Assets {
		if a.Name == assetName {
			st.LatestSize = a.Size
			break
		}
	}
	// 版本变了 → 之前的"跳过此版本"失效
	if st.SkippedVersion != "" && Compare(st.SkippedVersion, st.LatestVersion) != 0 {
		st.SkippedVersion = ""
	}
	_ = saveState(st)

	info.Latest = st.LatestVersion
	info.URL = st.LatestURL
	info.Notes = st.LatestNotes
	info.Size = st.LatestSize
	info.PublishedAt = st.LatestPubAt
	info.FromCache = false
	applyLocalState(info, st)
	return info, nil
}

// applyLocalState 根据本地状态计算 HasUpdate / Snoozed / Skipped。
func applyLocalState(info *Info, st State) {
	info.HasUpdate = info.Latest != "" && isReleaseVersion(info.Current) &&
		Compare(info.Latest, info.Current) > 0
	info.Snoozed = info.HasUpdate && time.Now().Before(st.SnoozeUntil)
	info.Skipped = info.HasUpdate && st.SkippedVersion != "" &&
		Compare(st.SkippedVersion, info.Latest) == 0
}

// fetchLatest 拉取最新 release 信息。
func fetchLatest() (*release, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, err
	}
	// 必须带 UA，否则 GitHub API 返回 403
	req.Header.Set("User-Agent", repoName+"-updater")
	req.Header.Set("Accept", "application/vnd.github+json")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GitHub API 返回 %d", resp.StatusCode)
	}
	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	if rel.TagName == "" {
		return nil, errors.New("release 响应缺少 tag_name")
	}
	return &rel, nil
}

// ── 用户操作 ──

// Snooze 将更新提醒推迟 d（d<=0 时用默认 24 小时）。
func Snooze(d time.Duration) error {
	if d <= 0 {
		d = defaultSnooze
	}
	st := loadState()
	st.SnoozeUntil = time.Now().Add(d)
	return saveState(st)
}

// SkipVersion 跳过指定版本（传空串表示跳过当前已知的最新版本）。
func SkipVersion(v string) error {
	st := loadState()
	if v == "" {
		v = st.LatestVersion
	}
	if v == "" {
		return errors.New("没有可跳过的版本")
	}
	st.SkippedVersion = v
	st.SnoozeUntil = time.Time{} // 跳过优先级更高，清掉静默期
	return saveState(st)
}

// ClearSkipped 清除跳过标记，让提醒重新生效（用户改主意时使用）。
func ClearSkipped() error {
	st := loadState()
	st.SkippedVersion = ""
	st.SnoozeUntil = time.Time{}
	return saveState(st)
}

// DownloadPageURL 返回供浏览器打开的下载页地址。
func DownloadPageURL() string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/latest", repoOwner, repoName)
}
