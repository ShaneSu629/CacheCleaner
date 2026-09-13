# 插件示例（Examples）

本目录存放 **CacheCleaner 插件的开发示例**，供开发者参考。这些示例不会被编译进主程序，也不会自动安装。

## 如何使用示例

1. 打开 CacheCleaner
2. 侧边栏「管理」→「插件管理」→「安装插件」
3. 选择本目录下的某个示例文件夹（如 `demo-system-info`）
4. 插件会自动复制到 exe 旁的 `plugins/` 目录并启用

## 示例列表

### demo-system-info — 系统信息
演示脚本型插件的完整写法：
- `manifest.json`：声明 `type: "script"` + 入口 `main.js`
- `main.js`：使用 `cc` API（`cc.platform` / `cc.exec` / `cc.home` / `cc.render` / `cc.log`）

## 开发自己的插件

见 `docs/plugin-dev-guide.md`，或加载 `cachecleaner-plugin-dev` skill 让 AI 助手生成插件。
