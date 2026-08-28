# Smart Cache Cleaner - 智能缓存清理工具（跨平台版）

自动扫描并清理 **AI 编程/聊天工具**、浏览器、开发工具、系统产生的缓存，支持
**Windows / macOS / Linux** 三平台。内置「通用结构识别」——不认工具名，只认
Electron/Chromium 标准缓存目录结构，因此**任何新出的 Electron 套壳 AI 工具装了即被自动识别**，
无需手工维护清单。

> 本仓库提供两种构建：
> - **命令行版（CLI）**：纯标准库、`CGO_ENABLED=0` 静态编译，源码位于 `cmd/cli/`，无外部依赖。
> - **图形界面版（GUI，Wails）**：基于 `github.com/wailsapp/wails/v2`，源码位于 `main.go` / `app.go` + `frontend/`，
>   需要 CGO 与系统 WebView 运行时，详见下方「GUI 图形界面（Wails）」一节。
>
> 旧版 Windows 专用 PowerShell 脚本 `CacheCleaner.ps1` / `CacheCleaner.bat` 仍保留作参考，
> 但已不再维护。

---

## 功能特性

- **深度扫描 AI 缓存**（聚焦 AI/IDE 工具）：WorkBuddy/CodeBuddy、Cursor、Trae、Claude、
  Windsurf/Codeium、VS Code、通义灵码/Qoder、文心快码、Cline、Continue、OpenAI Codex、
  Ollama、LM Studio、HuggingFace、Cherry Studio、豆包、DeepSeek、Kimi 等。
- **通用结构识别（核心）**：遍历 `AppData` 三目录 + 用户根隐藏目录，命中
  `Cache / GPUCache / Code Cache / Dawn*Cache / blob_storage / Crashpad / CachedData` 等
  标准缓存名即判定为可清理缓存簇，**自动覆盖未知的未来工具**。
- **一键扫描所有缓存**：含浏览器、IM、网盘等一切 Electron 应用缓存。
- **系统软件升级缓存识别**：微信 4.x（`xwechat\update`，实测可达 GB 级）、企业微信、QQ 邮箱、
  Electron 安装器 `SquirrelTemp`、`C:\ProgramData\Package Cache`、NVIDIA 驱动下载缓存、
  Windows 更新下载缓存与传递优化缓存、`C:\Windows\Temp` 等（`ProgramData` 已纳入扫描根）。
- **注册表垃圾清理（Windows）**：扫描并清理 HKCU 下「运行/最近文档/地址栏/打开保存对话框 MRU」、
  程序名缓存、托盘图标通知历史等**系统可自动重建的衍生数据**；**只删值不删键**，
  且仅允许清理内置白名单中的键，清理前二次确认。
- **Windows 11 风格界面**：Mica 桌面底纹 + Acrylic 半透明毛玻璃、圆角卡片、统一 Fluent 图标；
  **浅色 / 深色双主题**（默认跟随系统，顶栏可一键切换并记忆）；响应式三档：
  桌面完整侧栏 / 平板图标栏 / 手机底部应用栏。
- **风险三级分级**：`Safe` 安全（绿）/ `Caution` 谨慎（黄）/ `Review` 复核（红）。
- **应用图标**：扫帚 + 清洁闪光的 Fluent 风格蓝色磁贴图标（16–256px 多尺寸）。
- **交互选择清理**：按编号选择，`safe` 仅清安全项，`all` 全部，`q` 取消；清理前二次确认。
- **自定义目录管理 / 排除目录 / 清理历史**：自定义目录会作为独立缓存项纳入扫描；
  排除项按「完整路径段 + 大小写不敏感」匹配，不会误伤 `Catalogs` 这类同名子串路径。
- **GUI 实时进度条**：Wails 图形界面扫描时按阶段（已知缓存 → 自动发现 → 通用识别 → 自定义目录）
  实时回传进度与百分比，并在「关于」弹窗中说明版本与引擎。

---

## 环境要求

- **无需安装运行时**：发布的二进制为静态可执行文件，下载即可运行。
- Windows 10/11、macOS 11+、主流 Linux 发行版均可。
- 推荐在终端（Windows Terminal / PowerShell 7 / iTerm / 系统终端）中运行；
  若在老版 `cmd` 出现中文乱码，先执行 `chcp 65001`。

---

## 使用方法

### 方式一：从 Release 下载（推荐）

前往仓库 **Releases** 页面，按系统下载对应二进制：

| 系统 | 文件 |
|------|------|
| Windows | `cachecleaner-windows-amd64.exe` |
| Linux | `cachecleaner-linux-amd64` |
| macOS Intel | `cachecleaner-darwin-amd64` |
| macOS Apple Silicon | `cachecleaner-darwin-arm64` |

赋予执行权限（macOS/Linux）后直接运行：

```bash
chmod +x cachecleaner-darwin-arm64
./cachecleaner-darwin-arm64
```

### 方式二：从源码编译

```bash
# 需要 Go 1.22+
go build -o cachecleaner .

# 交叉编译示例（无需目标平台环境）
GOOS=linux   GOARCH=amd64 CGO_ENABLED=0 go build -o cachecleaner-linux   .
GOOS=darwin  GOARCH=arm64 CGO_ENABLED=0 go build -o cachecleaner-macos   .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -o cachecleaner.exe     .
```

### 方式三：GitHub Actions 自动编译

推送 `v*` 标签即可触发工作流 `.github/workflows/build.yml`，自动为四个目标平台
交叉编译并发布到 Release（静态、无 CGO）。也可在 Actions 页面手动 `Run workflow`。
> 注：CI 当前仅编译静态 CLI（`cmd/cli`），GUI 需在本机使用 `wails build` 出包。

---

## GUI 图形界面（Wails）

除命令行版外，仓库还内置一套 **Wails** 实现的扁平化图形界面（`main.go` + `app.go` 绑定层 +
`frontend/dist` 静态前端）。它**复用同一套扫描/清理引擎，仅替换 UI 层**，不引入任何业务逻辑改动。

### 前置依赖（每个平台不同）

- **Go 1.22+**（需开启 `CGO`）
- **Wails CLI**：`go install github.com/wailsapp/wails/v2/cmd/wails@latest`
- **系统 WebView 运行时**：
  - Windows：WebView2（Win10/11 通常已自带）
  - macOS：系统 WKWebView（无需额外安装）
  - Linux：`sudo apt install gcc libgtk-3-dev libwebkit2gtk-4.0-dev`
    （Ubuntu 24.04+ 用 `libwebkit2gtk-4.1-dev`，并在 `wails build` 加 `-tags webkit2_41`）
- 前端为**纯静态 HTML/CSS/JS**，无需 Node 构建链。

### 构建与运行

```bash
# 首次拉取依赖（需联网）
go get github.com/wailsapp/wails/v2@latest
go mod tidy

# 开发模式（热重载，需系统 WebView）
wails dev

# 编译发布（开启 CGO，输出原生二进制 / .app）
wails build
```

---

## 菜单功能（命令行版）

```
══════════════════════════════════════
       智能缓存清理工具
══════════════════════════════════════
  [1] 深度扫描 AI 缓存（聚焦 AI/IDE 工具）
  [2] 一键扫描所有缓存（含浏览器/IM/网盘/升级缓存）
  [3] 自定义缓存目录管理
  [4] 查看清理历史
  [5] 注册表垃圾清理（仅 Windows）
  [0] 退出
```

选择后输入编号（逗号分隔）/ `safe` / `all` / `q`，再输入 `y` 确认清理。

---

## 项目结构

```
CacheCleaner/
├── go.mod                      # Go 模块（CLI 无外部依赖；GUI 依赖 Wails）
├── main.go                     # GUI 入口（Wails 应用）
├── app.go                      # GUI 绑定层（App 结构 + Scan/Clean/配置方法），复用内部包
├── cmd/cli/main.go             # 命令行版入口（纯标准库、静态编译，无外部依赖）
├── frontend/dist/              # GUI 静态前端（HTML/CSS/JS，扁平化）
├── internal/
│   ├── model/                  # 数据结构与风险等级
│   ├── config/                 # OS 自适应路径与用户配置
│   ├── db/                     # 三系统已知缓存库 + Electron 缓存签名
│   ├── scan/                   # 扫描引擎（已知/自动发现/通用识别/合并）
│   ├── ui/                     # 命令行菜单与配色展示
│   └── clean/                  # 清理引擎与历史记录
├── wails.json                  # Wails 工程配置
├── .github/workflows/build.yml # 跨平台编译 CI（当前仅编译 CLI）
├── 技术文档.md                  # 架构与修改记录
└── CacheCleaner.ps1 / .bat     # 旧版 Windows 专用实现（已弃用）
```

---

## 安全说明

- 清理前会列出全部可清理项及大小，由用户交互选择并二次确认。
- 通用识别**只删除纯缓存目录**（`Cache`/`GPUCache`/`blob_storage`/`Crashpad`/`logs` 等），
  自动排除 `Cookies`/`Local State`/`IndexedDB`/`Preferences` 等登录态与配置。
- **绝不扫描** `Documents`/`Downloads`/`Desktop` 等用户数据目录。
- `Review` 级（如 `.workbuddy/projects/sessions/tasks`、Ollama 模型）默认不参与 `safe` 清理，
  需用户显式选择；清理旧会话上下文正是解决「AI 输出不准」的抓手。

## 许可证

MIT
