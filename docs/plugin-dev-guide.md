# CacheCleaner 插件开发指南

> CacheCleaner 支持**脚本型插件**：用 JavaScript 编写，通过全局 `cc` API 调用系统能力，完全脱离主项目开发。用户或 AI 工具（WorkBuddy、TraeCode 等）都能独立开发插件。

## 一、插件是什么

插件 = 一个文件夹，含两份文件：

```
<任意名>/
├── manifest.json   # 元信息 + 声明入口
└── main.js         # 插件逻辑（JavaScript）
```

主程序加载插件后，执行 `main.js`，脚本通过 `cc` API 做系统操作并把结果渲染到面板。

## 二、manifest.json

```json
{
  "id": "my-plugin",          // 必填：唯一标识（也是目录名）
  "nameZh": "中文名",
  "nameEn": "My Plugin",
  "icon": "i-info",           // 图标（i-trash / i-info / i-layers / i-folder 等）
  "descZh": "中文描述",
  "descEn": "English description",
  "type": "script",           // ★必填：script = 脚本型插件
  "entry": "main.js",         // 入口脚本（默认 main.js，可省略）
  "version": "1.0.0",
  "author": "作者"
}
```

## 三、cc API 速查

| 方法 | 说明 | 示例 |
|------|------|------|
| `cc.log(msg)` | 写日志（进 CacheCleaner.log + 面板底部） | `cc.log('start')` |
| `cc.render({title,text,html})` | 渲染面板内容 | `cc.render({title:'X', html:'<b>hi</b>'})` |
| `cc.exec(cmd, timeoutMs)` | 执行命令，返回 `{code,stdout,stderr}` | `cc.exec('dir', 5000)` |
| `cc.readFile(p)` / `cc.writeFile(p, c)` | 读写文件（相对路径相对插件目录） | `cc.writeFile('data/a.txt','x')` |
| `cc.env(name)` | 读环境变量 | `cc.env('APPDATA')` |
| `cc.cwd()` | 插件目录 | — |
| `cc.home()` | 用户主目录 | — |
| `cc.platform()` | `windows`/`darwin`/`linux` | — |
| `cc.dataDir()` | 插件目录下 data/（自动创建） | — |

## 四、完整示例

```js
cc.log('插件启动，平台: ' + cc.platform());

let osInfo = '';
if (cc.platform() === 'windows') {
  osInfo = cc.exec('ver', 5000).stdout.trim();
} else {
  osInfo = cc.exec('uname -a', 5000).stdout.trim();
}

cc.render({
  title: '系统信息',
  html: `<div style="padding:16px;">
    <h3>🖥️ 系统信息</h3>
    <table>
      <tr><td>平台</td><td>${cc.platform()}</td></tr>
      <tr><td>系统版本</td><td>${osInfo}</td></tr>
      <tr><td>用户目录</td><td>${cc.home()}</td></tr>
    </table>
  </div>`,
});
```

## 五、安装与使用

1. 打开 CacheCleaner → 侧边栏「管理」→「插件管理」
2. 点「安装插件」→ 选择插件文件夹
3. 侧边栏「插件」分组出现你的插件入口
4. 点击 → 执行脚本 → 显示结果

## 六、给 AI 工具用的 skill

开发者（或 AI 工具）可加载 `cachecleaner-plugin-dev` skill，按规范生成插件。skill 内置了 cc API 完整说明和安全边界。

## 七、内置能力型插件（旧模式，保留）

早期还支持「能力型」插件（`type` 缺省或 `capability` 字段），引用主程序内置能力（如 `uninstaller` 软件卸载）。这种模式能力有限，新插件请优先用脚本型。
