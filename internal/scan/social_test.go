package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cachecleaner/internal/config"
	"cachecleaner/internal/model"
)

// mkfile 在 dir 下创建指定相对路径的文件（自动建父目录）。
func mkfile(t *testing.T, dir, rel string, size int) {
	t.Helper()
	full := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, make([]byte, size), 0o644); err != nil {
		t.Fatal(err)
	}
}

// 社交发现：微信 3.x 只应产出 FileStorage 纯缓存子目录，绝不产出含聊天记录的账号整目录。
func TestFindSocialCaches_WeChat3(t *testing.T) {
	docs := t.TempDir()
	base := filepath.Join(docs, "WeChat Files", "wxid_test123")
	mkfile(t, base, "FileStorage/Cache/a.bin", 4096)
	mkfile(t, base, "FileStorage/Thumb/t.bin", 1024)
	mkfile(t, base, "Temp/x.tmp", 512)
	// 用户数据：图片/视频/聊天记录数据库，绝不能成为缓存项
	mkfile(t, base, "FileStorage/Image/2026-08/pic.dat", 99999)
	mkfile(t, base, "FileStorage/Video/2026-08/v.dat", 99999)
	mkfile(t, base, "Msg/MicroMsg.db", 99999)
	mkfile(t, base, "config/cfg.json", 128)

	cfg := &config.Config{Home: docs, Documents: docs, OS: "windows"}
	got := FindSocialCaches(cfg)

	if len(got) != 3 {
		t.Fatalf("应发现 3 个纯缓存子目录，实际 %d: %+v", len(got), got)
	}
	for _, e := range got {
		if !strings.Contains(e.Category, "微信") {
			t.Errorf("分类应标注微信: %s", e.Category)
		}
		low := strings.ToLower(e.Path)
		for _, banned := range []string{"image", "video", "micromsg", "msg", "config"} {
			if strings.Contains(low, banned) && !strings.HasSuffix(low, "cache") {
				t.Errorf("用户数据目录不应成为缓存项: %s", e.Path)
			}
		}
		if filepath.Base(e.Path) == "wxid_test123" {
			t.Errorf("账号整目录不应成为缓存项: %s", e.Path)
		}
	}
}

// 社交发现：QQ NT 的日志/图片/视频/表情缓存能被发现，global(登录列表) 与 Ptt(语音) 不碰。
func TestFindSocialCaches_QQNT(t *testing.T) {
	docs := t.TempDir()
	base := filepath.Join(docs, "Tencent Files", "123456")
	mkfile(t, base, "nt_qq/nt_data/log/run.log", 2048)
	mkfile(t, base, "nt_qq/nt_data/Pic/2026-08/a.jpg", 8192)
	mkfile(t, base, "nt_qq/nt_data/Video/2026-08/b.mp4", 16384)
	mkfile(t, base, "nt_qq/nt_data/Emoji/2026-08/e.gif", 1024)
	mkfile(t, base, "nt_qq/nt_data/Ptt/2026-08/v.amr", 7777)  // 语音：用户数据
	mkfile(t, base, "nt_qq/global/setting.ini", 128)          // 登录列表：敏感
	mkfile(t, base, "nt_qq/nt_data/File_Recv/doc.zip", 55555) // 接收文件：用户数据

	cfg := &config.Config{Home: docs, Documents: docs, OS: "windows"}
	got := FindSocialCaches(cfg)

	if len(got) != 4 {
		t.Fatalf("应发现 4 个 QQ 缓存目录，实际 %d: %+v", len(got), got)
	}
	riskByBase := map[string]model.Risk{}
	for _, e := range got {
		b := filepath.Base(e.Path)
		riskByBase[b] = e.Risk
		if !strings.Contains(e.Category, "QQ") {
			t.Errorf("分类应标注 QQ: %s", e.Category)
		}
		low := strings.ToLower(e.Path)
		if strings.Contains(low, "ptt") || strings.Contains(low, "global") || strings.Contains(low, "file_recv") {
			t.Errorf("用户数据/敏感目录不应成为缓存项: %s", e.Path)
		}
	}
	if riskByBase["log"] != model.RiskSafe {
		t.Errorf("运行日志应为安全级: %+v", riskByBase)
	}
	if riskByBase["Pic"] != model.RiskCaution || riskByBase["Video"] != model.RiskCaution {
		t.Errorf("聊天图片/视频缓存应为谨慎级: %+v", riskByBase)
	}
}

// 社交发现：被排除目录不产出；不存在的容器不报错。
func TestFindSocialCaches_Excluded(t *testing.T) {
	docs := t.TempDir()
	base := filepath.Join(docs, "WeChat Files", "wxid_abc")
	mkfile(t, base, "FileStorage/Cache/a.bin", 2048)

	cfg := &config.Config{Home: docs, Documents: docs, OS: "windows", ExcludeDirs: []string{"FileStorage"}}
	if got := FindSocialCaches(cfg); len(got) != 0 {
		t.Errorf("排除规则应生效，实际: %+v", got)
	}

	cfg2 := &config.Config{Home: t.TempDir(), Documents: t.TempDir(), OS: "windows"}
	if got := FindSocialCaches(cfg2); len(got) != 0 {
		t.Errorf("空环境不应产出任何项: %+v", got)
	}
}

// 微信 4.x：xwechat_files 只取 cache/temp，不碰 msg/db/backup。
func TestFindSocialCaches_WeChat4(t *testing.T) {
	docs := t.TempDir()
	base := filepath.Join(docs, "xwechat_files", "wxid_4x")
	mkfile(t, base, "cache/c.bin", 4096)
	mkfile(t, base, "temp/t.tmp", 1024)
	mkfile(t, base, "msg/video/2026-08/v.mp4", 88888)
	mkfile(t, base, "msg/file/2026-08/f.docx", 44444)
	mkfile(t, base, "db/core.db", 66666)
	mkfile(t, base, "backup/bak.dat", 33333)

	cfg := &config.Config{Home: docs, Documents: docs, OS: "windows"}
	got := FindSocialCaches(cfg)

	if len(got) != 2 {
		t.Fatalf("微信 4.x 应只发现 cache/temp 两项，实际 %d: %+v", len(got), got)
	}
	for _, e := range got {
		low := strings.ToLower(e.Path)
		for _, banned := range []string{"msg", "db", "backup", "video", "file"} {
			if strings.Contains(low, string(filepath.Separator)+banned+string(filepath.Separator)) {
				t.Errorf("用户数据目录不应成为缓存项: %s", e.Path)
			}
		}
	}
}

// 厂商子树 + 容器深度加成：国产应用深层嵌套（Tencent/xwechat/radium/web/
// profiles/<随机名>/Cache，本机取证 6 层以上）中的标准缓存簇必须可达。
// 修复前深度预算固定为 3，walk 在 web 层就被截断，永远够不到 profiles 容器。
func TestCollectElectron_DeepVendorBoost(t *testing.T) {
	root := t.TempDir()
	deep := filepath.Join(root, "Tencent", "xwechat", "radium", "web", "profiles", "randomProfile", "Cache")
	mkfile(t, deep, "c.bin", 2048)

	cfg := &config.Config{Home: root, OS: "windows"}
	var out []model.CacheEntry
	seen := map[string]bool{}
	// 与真实扫描一致的预算 3：Roaming(0) Tencent(1) xwechat(2) radium(3) 就到顶
	collectElectron(root, 0, 3, cfg, seen, &out)

	found := false
	for _, e := range out {
		if e.Path == deep {
			found = true
		}
	}
	if !found {
		t.Errorf("厂商深层嵌套下的 Cache 应通过深度加成被发现，实际: %+v", out)
	}

	// 对照：非厂商目录的同样深度结构不应被加深（遍历成本可控的关键）
	root2 := t.TempDir()
	deep2 := filepath.Join(root2, "RandomApp", "a", "b", "c", "d", "e", "Cache")
	mkfile(t, deep2, "c.bin", 2048)
	cfg2 := &config.Config{Home: root2, OS: "windows"}
	var out2 []model.CacheEntry
	seen2 := map[string]bool{}
	collectElectron(root2, 0, 3, cfg2, seen2, &out2)
	for _, e := range out2 {
		if e.Path == deep2 {
			t.Errorf("非厂商目录不应享受深度加成: %s", e.Path)
		}
	}
}

// AutoDiscover：文档目录应作为扫描根，且新关键词 upgrade/patch 能命中。
func TestAutoDiscover_DocumentsAndNewKeywords(t *testing.T) {
	home := t.TempDir()
	docs := t.TempDir()
	mkfile(t, docs, "SomeApp/upgrade/pkg.bin", 4096)
	mkfile(t, docs, "SomeApp/patch/hotfix.bin", 2048)
	mkfile(t, docs, "work/normal.txt", 128) // 普通目录不应命中

	cfg := &config.Config{Home: home, Documents: docs, OS: "windows"}
	got := AutoDiscover(cfg, nil)

	bases := map[string]bool{}
	for _, e := range got {
		bases[filepath.Base(e.Path)] = true
	}
	if !bases["upgrade"] || !bases["patch"] {
		t.Errorf("upgrade/patch 关键词应命中文档目录下的升级暂存目录: %+v", got)
	}
	if bases["normal"] || bases["work"] {
		t.Errorf("普通目录不应被误报: %+v", got)
	}
}
