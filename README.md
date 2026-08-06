# Smart Cache Cleaner - 智能缓存清理工具

自动扫描 Windows 用户目录下的 AI 缓存、软件缓存、临时文件，支持智能识别、风险分级、交互选择、安全清理。

## 功能特性

### AI 缓存清理（重点）
- **Cursor AI** — 编辑器缓存、扩展缓存、代码缓存、机器标识
- **Trae AI** — 缓存数据、代码缓存
- **VS Code** — 缓存、扩展缓存、工作区存储
- **GitHub Copilot** — AI 补全缓存
- **AI 模型** — HuggingFace、PyTorch、Ollama、LM Studio 等
- 深度扫描 AI 特征文件（`.aicache`、`.completions`、`.copilot` 等）

### 分类清理
| 功能 | 说明 |
|------|------|
| 一键扫描所有缓存 | 全面扫描所有已知和自动发现的缓存目录 |
| 清理 AI 缓存 | 仅扫描 AI 相关缓存 |
| 清理浏览器缓存 | Chrome、Edge、Internet 临时文件 |
| 清理系统临时文件 | Windows Temp、缩略图、D3D 缓存等 |
| 清理开发工具缓存 | npm、pip、Gradle、Maven、Yarn 等 |
| 快速清理（仅安全项） | 一键清理所有 Safe 级别的缓存 |

### 高级功能
- **管理自定义目录** — 添加/删除自定义清理目录，添加排除目录
- **扫描大文件** — 发现 >50MB 的大文件
- **查看清理历史** — 累计清理统计

### 智能识别
- **已知缓存数据库** — 内置 60+ 已知缓存路径，覆盖 AI、浏览器、开发工具、IDE 等
- **自动发现** — 通过关键词匹配和文件类型分析，自动发现未知缓存目录
- **风险分级** — Safe（安全）/ Caution（注意）/ Review（确认）

## 环境要求

- Windows 7/10/11
- PowerShell 5.1 或更高版本

## 使用方法

### 方式一：双击运行

1. 下载 `CacheCleaner.bat` 和 `CacheCleaner.ps1` 到同一目录
2. 双击 `CacheCleaner.bat` 启动
3. 推荐选择 **「1. 深度扫描 AI 缓存」**

### 方式二：管理员运行（推荐）

右键 `CacheCleaner.bat` → **以管理员身份运行**，可获得最佳清理效果。

### 方式三：直接运行 PowerShell 脚本

```powershell
powershell -NoProfile -ExecutionPolicy Bypass -File "CacheCleaner.ps1"
```

## 菜单功能

```
┌───── AI 缓存清理（重点） ─────
│ 1. 🔥 深度扫描 AI 缓存（推荐）
│ 2. 一键扫描所有缓存
├───── 分类清理 ─────
│ 3. 清理 AI 缓存
│ 4. 清理浏览器缓存
│ 5. 清理系统临时文件
│ 6. 清理开发工具缓存
│ 7. 快速清理（仅安全项）
├───── 高级功能 ─────
│ 8. 管理自定义目录
│ 9. 扫描大文件(>50MB)
│ 0. 查看历史清理记录
└─────
q. 退出
```

## 文件结构

```
CacheCleaner/
├── CacheCleaner.ps1          # 主脚本
├── CacheCleaner.bat          # 双击启动入口
├── .gitignore                # Git 忽略规则
└── config/
    ├── CacheCleaner.config.json  # 用户自定义配置（已忽略）
    └── clean_history.json        # 清理历史记录（已忽略）
```

## 安全说明

- 清理前会列出所有可清理项及大小，由用户交互选择确认
- 所有清理操作可撤销（仅删除缓存文件，不影响系统和用户数据）
- 配置文件（`config/` 目录）包含用户路径信息，已加入 `.gitignore`
- 如需扫描整个 C 盘，修改脚本第 16 行：`UserHome = "C:\"`

## 自定义配置

编辑 `config/CacheCleaner.config.json`：

```json
{
    "CustomDirs": [
        "%USERPROFILE%\\AppData\\Local\\CustomCache"
    ],
    "ExcludedDirs": [
        "node_modules",
        ".git"
    ]
}
```

## 许可证

MIT