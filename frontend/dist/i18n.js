/* ==========================================================================
   CacheCleaner i18n — 轻量多语言支持
   设计：词条字典 + data-i18n 属性标记 + 占位符 {n}
   语言持久化到 localStorage，默认跟随系统（navigator.language）
   ========================================================================== */

const I18N = {
  'zh-CN': {
    // 通用
    'app.name': '智能缓存清理工具',
    'app.tagline': '跨平台 · 安全分级 · 一键释放空间',
    'theme.label': '深色',
    'theme.label.light': '浅色',
    'lang.label': '语言',

    // 导航
    'nav.clean': '清理',
    'nav.settings': '设置',
    'nav.plugins': '插件',
    'nav.plugins.manage': '插件管理',
    'plugin.mgr.sub': '插件放在程序目录 plugins 文件夹，支持启用 / 禁用',
    'plugin.mgr.reload': '重新扫描',
    'plugin.mgr.openfolder': '打开插件目录',
    'plugin.mgr.empty': '未发现任何插件',
    'plugin.mgr.hint.title': '如何开发插件？',
    'plugin.mgr.hint.msg': '创建一个文件夹，放入 manifest.json 和 main.js（脚本插件）。插件用 JavaScript 编写，通过 cc API 调用系统能力。',
    'plugin.mgr.enabled': '插件已启用',
    'plugin.mgr.disabled': '插件已禁用',
    'plugin.mgr.reloaded': '已重新扫描插件',
    'plugin.mgr.install': '安装插件',
    'plugin.mgr.install.title': '选择要安装的插件文件夹',
    'plugin.mgr.installed': '插件已安装',
    'plugin.mgr.uninstall': '卸载',
    'plugin.mgr.uninstall.confirm': '确定卸载插件「{id}」吗？将删除其文件夹。',
    'plugin.mgr.uninstalled': '插件已卸载',
    'nav.scan': '扫描清理',
    'nav.uninstall': '软件卸载',
    'nav.tools': '系统工具',
    'nav.registry': '注册表',
    'nav.custom': '目录规则',
    'nav.history': '清理历史',
    'nav.update': '软件更新',

    // 首页 Hero
    'hero.title': '为你的电脑腾出空间',
    'hero.sub': '深度扫描 AI 工具缓存，或一键扫描浏览器 / IM / 系统升级缓存，安全分级放心清理',
    'hero.ai': '深度扫描 AI 缓存',
    'hero.all': '一键扫描全部',
    'hero.empty': '暂无数据，先执行一次扫描',
    'hero.noresult': '还没有扫描结果',
    'hero.noresult.sub': '点击上方「深度扫描 AI 缓存」开始',

    // 扫描
    'scan.scanning': '扫描中…',
    'scan.done': '扫描完成',
    'scan.results': '扫描结果',
    'scan.total': '共 {n} 项',
    'scan.cleanable': '可清理项',
    'scan.free': '预计释放',
    'scan.safe': '安全项（可放心清理）',
    'scan.chart': '分类占用',
    'scan.chart.hint': '扫描后显示',
    'scan.safeOnly': '仅安全项',
    'scan.selectAll': '全选',
    'scan.cleanSelected': '清理选中',
    'scan.cleaning': '清理中…',

    // 清理结果
    'clean.done': '已清理 {n} 项，释放 {size}',
    'clean.partial': '清理完成（部分失败）',
    'clean.delayed': '清理完成（含延迟删除）',
    'clean.delayed.note': '另有 {n} 项被占用，已登记为「下次开机时自动删除」。',
    'clean.delayed.only': '其中 {n} 项正被运行中的程序占用，已登记为「下次开机时自动删除」，重启电脑后生效。',
    'clean.failed.note': '已清理 {n} 项，释放 {size}。\n有 {f} 项因文件被占用而失败（未识别出占用进程），退出对应软件后重试。',
    'clean.failed.log': '已清理 {n} 项，释放 {size}。\n有 {f} 项清理失败，详情见日志。',
    'clean.busy.title': '文件被占用，清理未完全成功',
    'clean.busy.msg': '已清理 {n} 项，释放 {size}。\n{f} 项清理失败。\n\n占用文件进程：{list}\n\n结束进程会关闭对应软件（未保存的数据可能丢失）。',

    // 风险等级
    'risk.safe': '安全',
    'risk.caution': '谨慎',
    'risk.review': '复核',

    // 系统工具
    'tools.title': '系统工具',
    'tools.sub': '组件存储（WinSxS）清理等需要管理员权限的系统级维护',
    'tools.warn.title': '需要管理员权限',
    'tools.warn.msg': '本程序已配置为自动请求管理员权限。DISM 在后台执行（组件清理约需 5-20 分钟）。不使用 /ResetBase，清理后仍可卸载已装更新。\n⚠ 若清理反复报「拒绝访问」，说明组件存储可能不一致——先点「修复组件存储」（最长约 30 分钟），完成后再清理。',
    'tools.analyze': '分析组件存储',
    'tools.repair': '修复组件存储',
    'tools.clean': '执行组件清理',
    'tools.notAnalyzed': '尚未分析',
    'tools.analyze.hint': '分析可查看 WinSxS 实际占用与可回收空间（约 1 分钟）',
    'tools.winsxs': '组件存储（WinSxS）',
    'tools.reported': '资源管理器报告大小',
    'tools.actual': '实际大小',
    'tools.reclaimable': '可回收包数',
    'tools.lastclean': '上次清理日期',
    'tools.recommended': '清理建议',
    'tools.waitAuth': '等待提权授权…',

    // 注册表
    'reg.title': '注册表垃圾清理',
    'reg.sub': '仅清理系统可自动重建的衍生数据（MRU 历史 / 程序名缓存）',
    'reg.warn.title': '注册表操作不可恢复',
    'reg.warn.msg': '只删除键值、不删除键本身，且仅限内置白名单位置；建议优先清理「安全」项。',
    'reg.scan': '扫描注册表',
    'reg.cleanable': '可清理项',
    'reg.unscanned': '未扫描',
    'reg.empty': '未发现可清理的注册表项',
    'reg.empty.sub': '打开本页时会自动扫描一次',

    // 目录规则
    'custom.title': '目录规则',
    'custom.sub': '额外纳入扫描的目录，以及需要跳过不清理的目录',
    'custom.add': '添加目录',
    'custom.exclude.add': '添加排除项',
    'custom.exclude.note': '排除项按「完整路径段 + 大小写不敏感」匹配，例如 logs 不会误伤 Catalogs。',

    // 清理历史
    'history.title': '清理历史',
    'history.sub': '按批次分组，点击展开明细',
    'history.empty': '暂无清理记录',
    'history.empty.sub': '清理缓存后这里会记录',

    // 软件更新
    'update.title': '软件更新',
    'update.sub': '检查新版本并前往下载；不想立刻更新可稍后提醒或跳过此版本',
    'update.check': '检查更新',
    'update.version': '版本信息',
    'update.unchecked': '未检查',
    'update.current': '当前版本',
    'update.latest': '最新版本',
    'update.published': '发布时间',
    'update.size': '安装包大小',
    'update.download': '后台下载',
    'update.restart': '更新并重启',
    'update.snooze': '稍后提醒',
    'update.skip': '跳过此版本',
    'update.has': '有更新可用',
    'update.has.snoozed': '有更新（已推迟提醒）',
    'update.has.skipped': '有更新（已跳过）',
    'update.uptodate': '已是最新',
    'update.found': '发现新版本 {v}',
    'update.downloading': '正在后台下载…',
    'update.verifying': '正在校验完整性…',
    'update.done': '下载完成，可更新并重启',
    'update.failed': '下载失败',
    'update.notes': '更新说明',
    'update.ready.title': '新版本已就绪',
    'update.ready.msg': '新版本已下载完成，是否立即更新？\n更新将关闭程序、替换为新版本并自动重新打开，全程约几秒钟。',
    'update.ready.ok': '立即更新',
    'update.ready.cancel': '稍后再说',
    'update.apply.confirm': '将关闭程序、替换为新版本并自动重新打开。确定现在更新吗？',
    'update.snoozed.toast': '已推迟提醒，24 小时后再提醒',
    'update.skipped.toast': '已跳过此版本',
    'update.dl.toast': '已开始后台下载，完成后会提示',
    'update.dl.done.toast': '新版本已下载完成，可更新并重启',
    'update.dl.fail.toast': '下载失败：{msg}',
    'update.manual': '更新方式：覆盖即可，无需卸载',
    'update.manual.msg': '本程序是单文件绿色软件，下载新版本 exe 覆盖旧文件即完成更新。目录规则、排除项与清理历史保存在 %APPDATA%\\CacheCleaner，不会因更新丢失。',
    'update.manual.download': '前往下载',

    // 通用按钮/对话框
    'btn.cancel': '取消',
    'btn.ok': '确认清理',
    'btn.confirm': '确认操作',
    'btn.know': '知道了',
    'dialog.update.title': '发现新版本',
    'dialog.update.msg': '当前 {cur}，最新 {latest}。可前往下载页获取新版本。',

    // 关于
    'about.version': '版本',
    'about.engine': '引擎',
    'about.engine.value': 'Go 原生扫描 + Electron 结构识别',
    'about.support': '支持',
    'about.scope': '清理范围',
    'about.scope.value': 'AI 缓存 · 升级缓存 · 注册表垃圾',
    'about.tip.prefix': '风险分级：',
    'about.tip.safe': ' 可放心清理，',
    'about.tip.suffix': ' 请确认后操作。',
    'about.close': '知道了',

    // 插件：软件卸载
    'uninstall.title': '软件卸载',
    'uninstall.sub': '枚举已安装软件，深度卸载 + 残留清理 + 流氓软件识别',
    'uninstall.refresh': '刷新列表',
    'uninstall.uninstall': '卸载',
    'uninstall.uninstalling': '正在卸载',
    'uninstall.done': '卸载完成',
    'uninstall.failed': '卸载失败',
    'uninstall.empty': '未发现已安装的软件',
    'uninstall.pup': '流氓软件',
    'uninstall.force': '强制卸载',
    'uninstall.residue': '残留扫描',
    'uninstall.confirm': '确定卸载 {name}？卸载后将自动扫描并清理残留文件与注册表项。',
    'uninstall.done.toast': '{name} 已卸载',
    'uninstall.done.detail': '{name} 卸载完成，检测到残留：',
    'uninstall.pup.warn': '检测到流氓软件特征（{reason}），建议立即卸载。',
    'uninstall.notdone': '{name} 可能未完全卸载（{msg}）。是否强制卸载？强制卸载会深度清理残留文件和注册表项。',
    'uninstall.residue.cleaned': '残留已清理',
  },

  'en-US': {
    // General
    'app.name': 'Cache Cleaner',
    'app.tagline': 'Cross-platform · Risk-graded · One-click cleanup',
    'theme.label': 'Dark',
    'theme.label.light': 'Light',
    'lang.label': 'Language',

    // Navigation
    'nav.clean': 'Clean',
    'nav.settings': 'Settings',
    'nav.plugins': 'Plugins',
    'nav.plugins.manage': 'Plugin Manager',
    'plugin.mgr.sub': 'Plugins live in the "plugins" folder next to the app. Enable or disable them.',
    'plugin.mgr.reload': 'Rescan',
    'plugin.mgr.openfolder': 'Open plugin folder',
    'plugin.mgr.empty': 'No plugins found',
    'plugin.mgr.hint.title': 'How to develop a plugin?',
    'plugin.mgr.hint.msg': 'Create a folder with a manifest.json and a main.js (script plugin). Plugins run JavaScript via the cc API.',
    'plugin.mgr.enabled': 'Plugin enabled',
    'plugin.mgr.disabled': 'Plugin disabled',
    'plugin.mgr.reloaded': 'Plugins rescanned',
    'plugin.mgr.install': 'Install plugin',
    'plugin.mgr.install.title': 'Select the plugin folder to install',
    'plugin.mgr.installed': 'Plugin installed',
    'plugin.mgr.uninstall': 'Uninstall',
    'plugin.mgr.uninstall.confirm': 'Uninstall plugin "{id}"? Its folder will be deleted.',
    'plugin.mgr.uninstalled': 'Plugin uninstalled',
    'nav.scan': 'Scan & Clean',
    'nav.uninstall': 'Uninstaller',
    'nav.tools': 'System Tools',
    'nav.registry': 'Registry',
    'nav.custom': 'Directory Rules',
    'nav.history': 'History',
    'nav.update': 'Updates',

    // Home hero
    'hero.title': 'Free up space on your PC',
    'hero.sub': 'Deep-scan AI tool caches, or clean browser / IM / system upgrade caches in one click with risk grading',
    'hero.ai': 'Deep Scan AI Caches',
    'hero.all': 'Scan Everything',
    'hero.empty': 'No data yet. Run a scan first',
    'hero.noresult': 'No scan results yet',
    'hero.noresult.sub': 'Click "Deep Scan AI Caches" above to start',

    // Scan
    'scan.scanning': 'Scanning…',
    'scan.done': 'Scan complete',
    'scan.results': 'Scan Results',
    'scan.total': '{n} items',
    'scan.cleanable': 'Cleanable items',
    'scan.free': 'Space to free',
    'scan.safe': 'Safe items',
    'scan.chart': 'Usage by category',
    'scan.chart.hint': 'Shown after scan',
    'scan.safeOnly': 'Safe only',
    'scan.selectAll': 'Select all',
    'scan.cleanSelected': 'Clean selected',
    'scan.cleaning': 'Cleaning…',

    // Clean results
    'clean.done': 'Cleaned {n} items, freed {size}',
    'clean.partial': 'Cleanup finished (some failed)',
    'clean.delayed': 'Cleanup finished (with delayed deletes)',
    'clean.delayed.note': '{n} items are locked and have been scheduled for deletion at next boot.',
    'clean.delayed.only': '{n} items are locked by running programs and have been scheduled for deletion at next boot.',
    'clean.failed.note': 'Cleaned {n} items, freed {size}.\n{f} items failed because files are in use (no locking process identified). Close the related apps and retry.',
    'clean.failed.log': 'Cleaned {n} items, freed {size}.\n{f} items failed. See log for details.',
    'clean.busy.title': 'Files are in use, cleanup incomplete',
    'clean.busy.msg': 'Cleaned {n} items, freed {size}.\n{f} items failed.\n\nLocking processes: {list}\n\nEnding a process will close its app (unsaved data may be lost).',

    // Risk levels
    'risk.safe': 'Safe',
    'risk.caution': 'Caution',
    'risk.review': 'Review',

    // System tools
    'tools.title': 'System Tools',
    'tools.sub': 'System-level maintenance such as component store (WinSxS) cleanup requiring admin rights',
    'tools.warn.title': 'Administrator rights required',
    'tools.warn.msg': 'This program is configured to request admin rights automatically. DISM runs in the background (component cleanup takes about 5-20 minutes). /ResetBase is not used, so installed updates can still be uninstalled.\n⚠ If cleanup keeps failing with "Access denied", the component store may be inconsistent — run "Repair component store" first (up to 30 minutes), then clean.',
    'tools.analyze': 'Analyze component store',
    'tools.repair': 'Repair component store',
    'tools.clean': 'Run component cleanup',
    'tools.notAnalyzed': 'Not analyzed',
    'tools.analyze.hint': 'Analyze shows WinSxS actual usage and reclaimable space (about 1 minute)',
    'tools.winsxs': 'Component Store (WinSxS)',
    'tools.reported': 'Explorer-reported size',
    'tools.actual': 'Actual size',
    'tools.reclaimable': 'Reclaimable packages',
    'tools.lastclean': 'Last cleanup date',
    'tools.recommended': 'Recommendation',
    'tools.waitAuth': 'Waiting for elevation…',

    // Registry
    'reg.title': 'Registry Junk Cleaner',
    'reg.sub': 'Only cleans system-rebuildable derived data (MRU history / program name caches)',
    'reg.warn.title': 'Registry operations are irreversible',
    'reg.warn.msg': 'Only values are deleted, never keys, and only within the built-in whitelist. Prefer cleaning "Safe" items.',
    'reg.scan': 'Scan registry',
    'reg.cleanable': 'Cleanable items',
    'reg.unscanned': 'Not scanned',
    'reg.empty': 'No cleanable registry items found',
    'reg.empty.sub': 'This page auto-scans when opened',

    // Directory rules
    'custom.title': 'Directory Rules',
    'custom.sub': 'Extra directories to include in scans, and directories to skip',
    'custom.add': 'Add directory',
    'custom.exclude.add': 'Add exclusion',
    'custom.exclude.note': 'Exclusions match full path segments case-insensitively, e.g. "logs" will not affect "Catalogs".',

    // History
    'history.title': 'Cleanup History',
    'history.sub': 'Grouped by batch, click to expand',
    'history.empty': 'No cleanup records yet',
    'history.empty.sub': 'Records appear here after cleaning',

    // Updates
    'update.title': 'Software Update',
    'update.sub': 'Check for new versions and download. Snooze or skip if you prefer',
    'update.check': 'Check for updates',
    'update.version': 'Version info',
    'update.unchecked': 'Not checked',
    'update.current': 'Current version',
    'update.latest': 'Latest version',
    'update.published': 'Published',
    'update.size': 'Package size',
    'update.download': 'Download',
    'update.restart': 'Update & restart',
    'update.snooze': 'Remind later',
    'update.skip': 'Skip this version',
    'update.has': 'Update available',
    'update.has.snoozed': 'Update (snoozed)',
    'update.has.skipped': 'Update (skipped)',
    'update.uptodate': 'Up to date',
    'update.found': 'New version found: {v}',
    'update.downloading': 'Downloading…',
    'update.verifying': 'Verifying integrity…',
    'update.done': 'Downloaded, ready to update',
    'update.failed': 'Download failed',
    'update.notes': 'Release notes',
    'update.ready.title': 'New version ready',
    'update.ready.msg': 'A new version has been downloaded. Update now?\nThe app will close, be replaced, and reopen automatically. It only takes a few seconds.',
    'update.ready.ok': 'Update now',
    'update.ready.cancel': 'Later',
    'update.apply.confirm': 'The app will close, be replaced with the new version, and reopen automatically. Update now?',
    'update.snoozed.toast': 'Reminder snoozed for 24 hours',
    'update.skipped.toast': 'Version skipped',
    'update.dl.toast': 'Download started in background',
    'update.dl.done.toast': 'New version downloaded. Ready to update',
    'update.dl.fail.toast': 'Download failed: {msg}',
    'update.manual': 'How to update: just overwrite, no uninstall needed',
    'update.manual.msg': 'This is a portable single-file app. Download the new exe and overwrite the old one. Directory rules, exclusions and history are stored in %APPDATA%\\CacheCleaner and survive updates.',
    'update.manual.download': 'Open download page',

    // Common buttons/dialogs
    'btn.cancel': 'Cancel',
    'btn.ok': 'Confirm clean',
    'btn.confirm': 'Confirm',
    'btn.know': 'OK',
    'dialog.update.title': 'New version available',
    'dialog.update.msg': 'Current {cur}, latest {latest}. Open the download page to get the new version.',

    // About
    'about.version': 'Version',
    'about.engine': 'Engine',
    'about.engine.value': 'Go native scanner + Electron structure detection',
    'about.support': 'Platforms',
    'about.scope': 'Cleans',
    'about.scope.value': 'AI caches · upgrade caches · registry junk',
    'about.tip.prefix': 'Risk levels: ',
    'about.tip.safe': ' safe to clean, ',
    'about.tip.suffix': ' confirm before acting.',
    'about.close': 'OK',

    // Plugin: uninstaller
    'uninstall.title': 'Software Uninstaller',
    'uninstall.sub': 'List installed software, deep uninstall + residue cleanup + PUP detection',
    'uninstall.refresh': 'Refresh list',
    'uninstall.uninstall': 'Uninstall',
    'uninstall.uninstalling': 'Uninstalling',
    'uninstall.done': 'Uninstalled',
    'uninstall.failed': 'Uninstall failed',
    'uninstall.empty': 'No installed software found',
    'uninstall.pup': 'PUP',
    'uninstall.force': 'Force uninstall',
    'uninstall.residue': 'Residue scan',
    'uninstall.confirm': 'Uninstall {name}? After uninstall, residue files and registry entries will be scanned and cleaned.',
    'uninstall.done.toast': '{name} uninstalled',
    'uninstall.done.detail': '{name} uninstalled. Residues detected:',
    'uninstall.pup.warn': 'Potentially unwanted program detected ({reason}). We recommend uninstalling it.',
    'uninstall.notdone': '{name} may not be fully uninstalled ({msg}). Force uninstall? This deeply cleans residue files and registry entries.',
    'uninstall.residue.cleaned': 'Residues cleaned',
  },
};

// 检测系统语言：中文环境（zh-*）→ 中文，其余 → 英文。
// 只在首次启动（无手动设置）时用于决定默认语言。
function detectSystemLang() {
  const lang = (navigator.language || navigator.userLanguage || navigator.browserLanguage || 'zh-CN').toLowerCase();
  return lang.startsWith('zh') ? 'zh-CN' : 'en-US';
}

// 语言优先级：用户手动切换过 → 用 localStorage；否则跟随系统语言。
let currentLang = localStorage.getItem('cachecleaner.lang') || detectSystemLang();

function t(key, vars) {
  const dict = I18N[currentLang] || I18N['zh-CN'];
  let s = dict[key] !== undefined ? dict[key] : key;
  if (vars) {
    Object.keys(vars).forEach((k) => {
      s = s.split('{' + k + '}').join(vars[k]);
    });
  }
  return s;
}

function applyI18n() {
  document.documentElement.lang = currentLang === 'zh-CN' ? 'zh-CN' : 'en-US';
  // data-i18n 属性元素
  document.querySelectorAll('[data-i18n]').forEach((el) => {
    const v = t(el.getAttribute('data-i18n'));
    if (v) el.textContent = v;
  });
  // data-i18n-placeholder
  document.querySelectorAll('[data-i18n-placeholder]').forEach((el) => {
    const v = t(el.getAttribute('data-i18n-placeholder'));
    if (v) el.placeholder = v;
  });
  // 语言按钮文字
  const langBtn = $('#lang-label');
  if (langBtn) langBtn.textContent = currentLang === 'zh-CN' ? 'English' : '中文';
  // 主题按钮
  const themeLabel = $('#theme-label');
  if (themeLabel) {
    const dark = document.documentElement.dataset.theme === 'dark';
    themeLabel.textContent = dark ? t('theme.label.light') : t('theme.label');
  }
}

function toggleLang() {
  currentLang = currentLang === 'zh-CN' ? 'en-US' : 'zh-CN';
  localStorage.setItem('cachecleaner.lang', currentLang);
  applyI18n();
  // 重新渲染当前面板（动态内容也要刷新语言）
  if (typeof rerenderCurrentPanel === 'function') rerenderCurrentPanel();
}
