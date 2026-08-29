/* CacheCleaner · Wails 前端交互层
   约定：业务逻辑全部在 Go 侧（window.go.main.App），这里只做编排、渲染与交互。 */

const $ = (s) => document.querySelector(s);
const $$ = (s) => document.querySelectorAll(s);

let entries = [];        // 当前扫描结果（DTO 数组）
let regEntries = [];     // 当前注册表扫描结果
let selected = new Set();
let regSelected = new Set();

/* ── 工具 ───────────────────────────────────────────────────────────── */

// 目录名/注册表项可包含引号与尖括号，直接拼 innerHTML 会破坏 DOM（属性注入）
const esc = (s) => String(s === undefined || s === null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;').replace(/'/g, '&#39;');

const fmtSize = (b) => {
  if (!b || b < 1024) return (b || 0) + ' B';
  const u = ['KB', 'MB', 'GB', 'TB'];
  let i = -1, n = b;
  do { n /= 1024; i++; } while (n >= 1024 && i < u.length - 1);
  return n.toFixed(1) + ' ' + u[i];
};

const riskClass = (r) => ({ '安全': 'ok', '谨慎': 'warn', '复核': 'risk' }[r] || '');

const show = (el, on) => { if (el) el.hidden = !on; };
const toggleEmpty = (el, on) => { if (el) el.classList.toggle('is-shown', !!on); };

async function call(method, ...args) {
  return await window.go.main.App[method](...args);
}

/* ── 前端错误上报：JS 异常统一落盘到 exe 旁的 CacheCleaner.log ──────── */
// Wails 运行时注入前 window.go 可能不存在，等可用后再挂接，避免错误丢失。
function reportFrontend(msg) {
  try {
    if (window.go && window.go.main && window.go.main.App) {
      window.go.main.App.LogFrontend(String(msg).slice(0, 500));
    }
  } catch (e) { /* 上报失败不影响页面 */ }
}
window.addEventListener('error', (ev) => {
  reportFrontend((ev.message || 'unknown') + ' @ ' + (ev.filename || '') + ':' + (ev.lineno || 0));
});
window.addEventListener('unhandledrejection', (ev) => {
  const r = ev.reason;
  reportFrontend('unhandledrejection: ' + ((r && r.message) || r || 'unknown'));
});

/* ── 主题（浅色 / 深色，跟随系统 + 手动覆盖 + 本地记忆） ────────────── */

const THEME_KEY = 'cachecleaner.theme';

function systemTheme() {
  return window.matchMedia && window.matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light';
}

function applyTheme(theme) {
  document.documentElement.dataset.theme = theme;
  const label = $('#theme-label');
  if (label) label.textContent = theme === 'dark' ? '浅色' : '深色';
}

function initTheme() {
  let saved = null;
  try { saved = localStorage.getItem(THEME_KEY); } catch (e) { saved = null; }
  applyTheme(saved || systemTheme());

  $('#btn-theme').onclick = () => {
    const next = document.documentElement.dataset.theme === 'dark' ? 'light' : 'dark';
    applyTheme(next);
    try { localStorage.setItem(THEME_KEY, next); } catch (e) { /* 忽略隐私模式写入失败 */ }
  };

  // 用户未手动选择时，跟随系统切换
  if (window.matchMedia) {
    window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', (e) => {
      let pref = null;
      try { pref = localStorage.getItem(THEME_KEY); } catch (err) { pref = null; }
      if (!pref) applyTheme(e.matches ? 'dark' : 'light');
    });
  }
}

/* ── Toast / 确认对话框 ─────────────────────────────────────────────── */

let toastTimer = null;
function toast(msg) {
  const t = $('#toast');
  t.textContent = msg;
  show(t, true);
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => show(t, false), 2400);
}

let lastFocused = null;

function confirmModal(text, title = '确认清理') {
  return new Promise((resolve) => {
    lastFocused = document.activeElement;
    $('#modal-title').textContent = title;
    $('#modal-text').textContent = text;
    show($('#modal'), true);

    const done = (v) => {
      show($('#modal'), false);
      $('#modal-ok').onclick = null;
      $('#modal-cancel').onclick = null;
      if (lastFocused && lastFocused.focus) lastFocused.focus();
      resolve(v);
    };
    $('#modal-ok').onclick = () => done(true);
    $('#modal-cancel').onclick = () => done(false);
    $('#modal-ok').focus();
  });
}

// 点击遮罩空白处或按 Esc 关闭弹窗
$('#modal').addEventListener('click', (e) => { if (e.target === $('#modal')) $('#modal-cancel').click(); });
$('#about').addEventListener('click', (e) => { if (e.target === $('#about')) show($('#about'), false); });
document.addEventListener('keydown', (e) => {
  if (e.key !== 'Escape') return;
  if (!$('#modal').hidden) $('#modal-cancel').click();
  else if (!$('#about').hidden) show($('#about'), false);
});

/* ── 清理进度（删除大目录可能持续数分钟，必须让用户看到进展） ───────── */

let cleanHideTimer = null;

function setCleaning(on) {
  $('#btn-clean').disabled = on;
  $('#btn-reg-clean').disabled = on;
  $('#btn-select-all').disabled = on;
  $('#btn-select-safe').disabled = on;
  $('#btn-reg-select-all').disabled = on;
  $('#btn-reg-select-safe').disabled = on;
  if (!on) return;
  // 起手先显示进度条，等后端第一个事件再更新数值
  const bar = $('#clean-bar');
  show(bar, true);
  $('#clean-text').textContent = '准备清理…';
  $('#clean-pct').textContent = '0%';
  $('#clean-fill').style.width = '0%';
  $('#clean-sub').textContent = '';
}

window.runtime.EventsOn('clean:progress', (p) => {
  const bar = $('#clean-bar');
  show(bar, true);
  const total = p.total || 0;
  $('#clean-text').textContent = total ? `清理中 ${p.done}/${total} 项` : '清理中…';
  const pct = p.pct || 0;
  $('#clean-pct').textContent = Math.round(pct) + '%';
  $('#clean-fill').style.width = pct.toFixed(1) + '%';

  const parts = [];
  if (p.path) parts.push('正在处理：' + p.path);
  if (p.freed) parts.push('已释放 ' + fmtSize(p.freed));
  $('#clean-sub').textContent = parts.join(' · ');

  clearTimeout(cleanHideTimer);
  if (total && p.done >= total) {
    $('#clean-text').textContent = '清理完成';
    cleanHideTimer = setTimeout(() => show(bar, false), 1400);
  }
});

/* ── 文件缓存扫描 ───────────────────────────────────────────────────── */

function setScanning(on) {
  $('#btn-ai').disabled = on;
  $('#btn-all').disabled = on;
}

async function startScan(mode) {
  setScanning(true);
  show($('#status'), true);
  const fill = $('#progress-fill');
  fill.classList.remove('indeterminate');
  fill.style.width = '0%';
  $('#status-text').textContent = '扫描中…（首次可能较慢）';
  $('#status-pct').textContent = '0%';
  await call('Scan', mode); // 进度经 scan:progress 回传，结果经 scan:done 回传
}

window.runtime.EventsOn('scan:progress', (d) => {
  $('#status-text').textContent = '扫描中：' + (d.phase || '');
  const fill = $('#progress-fill');
  if (d.total > 0) {
    fill.classList.remove('indeterminate');
    fill.style.width = (d.pct || 0).toFixed(1) + '%';
    $('#status-pct').textContent = Math.round(d.pct || 0) + '%';
  } else {
    // 该阶段总量未知：用流动条表示"进行中"
    fill.classList.add('indeterminate');
    $('#status-pct').textContent = '';
  }
});

// 扫描已在跑时后端会忽略重复请求
window.runtime.EventsOn('scan:busy', () => {
  toast('扫描正在进行中，请稍候');
});

window.runtime.EventsOn('scan:done', (data) => {
  entries = data || [];
  selected = new Set();
  const fill = $('#progress-fill');
  fill.classList.remove('indeterminate');
  fill.style.width = '100%';
  $('#status-pct').textContent = '100%';
  $('#status-text').textContent = `扫描完成：共 ${entries.length} 项`;
  renderList();
  renderChart();
  renderKpis();
  setTimeout(() => show($('#status'), false), 1200);
  setScanning(false);
});

function renderKpis() {
  let total = 0, safe = 0;
  entries.forEach((e) => { total += e.size; if (e.risk === '安全') safe++; });
  $('#kpi-count').textContent = entries.length;
  $('#kpi-size').textContent = fmtSize(total);
  $('#kpi-safe').textContent = safe;
}

function renderList() {
  const list = $('#list');
  list.innerHTML = '';
  entries.forEach((e, i) => {
    const row = document.createElement('div');
    row.className = 'row';
    row.style.animationDelay = Math.min(i, 12) * 12 + 'ms';
    row.innerHTML =
      `<input type="checkbox" data-i="${i}" ${selected.has(i) ? 'checked' : ''} aria-label="选择 ${esc(e.shortPath || e.path)}"/>` +
      `<span class="badge ${riskClass(e.risk)}">${esc(e.risk)}</span>` +
      `<span class="size">${fmtSize(e.size)}</span>` +
      `<span class="cat" title="${esc(e.category)}">${esc(e.category)}</span>` +
      `<span class="path" title="${esc(e.path)}">${esc(e.shortPath)}</span>`;
    list.appendChild(row);
  });
  toggleEmpty($('#empty-scan'), entries.length === 0);
  $('#stats').textContent = `共 ${entries.length} 项 · 可释放 ${fmtSize(entries.reduce((s, e) => s + e.size, 0))}`;
}

$('#list').addEventListener('change', (ev) => {
  const i = +ev.target.dataset.i;
  if (ev.target.checked) selected.add(i); else selected.delete(i);
});

function selectedPaths() { return [...selected].map((i) => entries[i].path); }
function selectedSize() { return [...selected].reduce((s, i) => s + (entries[i] ? entries[i].size : 0), 0); }

$('#btn-select-all').onclick = () => { selected = new Set(entries.map((_, i) => i)); renderList(); };
$('#btn-select-safe').onclick = () => {
  selected = new Set();
  entries.forEach((e, i) => { if (e.risk === '安全') selected.add(i); });
  renderList();
};

$('#btn-clean').onclick = async () => {
  const paths = selectedPaths();
  if (!paths.length) { toast('请先勾选要清理的项'); return; }
  const ok = await confirmModal(`确认清理 ${paths.length} 项？将释放约 ${fmtSize(selectedSize())}。`, '确认清理缓存');
  if (!ok) return;
  setCleaning(true);
  let res;
  try {
    res = await call('CleanSelected', paths);
  } finally {
    setCleaning(false);
  }
  toast(`已清理 ${res.count} 项，释放 ${fmtSize(res.freed)}` +
    (res.failed && res.failed.length ? `（失败 ${res.failed.length} 项）` : ''));
  // 只移除真正清理成功的项，失败/被排除的项保留供重试
  const cleaned = new Set(res.cleaned || []);
  entries = entries.filter((e) => !cleaned.has(e.path));
  selected = new Set();
  renderList();
  renderChart();
  renderKpis();
};

function renderChart() {
  const map = {};
  entries.forEach((e) => { map[e.category] = (map[e.category] || 0) + e.size; });
  const arr = Object.entries(map).sort((a, b) => b[1] - a[1]).slice(0, 6);
  const max = arr.length ? arr[0][1] : 1;
  $('#chart').innerHTML = arr.map(([k, v], i) =>
    `<div class="bar-row" style="animation-delay:${i * 45}ms">` +
    `<span class="bar-label" title="${esc(k)}">${esc(k)}</span>` +
    `<div class="bar-track"><div class="bar-fill" style="width:${Math.max(2, (v / max) * 100)}%;animation-delay:${i * 45}ms"></div></div>` +
    `<span class="bar-val">${fmtSize(v)}</span></div>`).join('');
  toggleEmpty($('#empty-chart'), arr.length === 0);
  $('#chart-hint').textContent = arr.length ? `共 ${arr.length} 类` : '扫描后显示';
}

/* ── 注册表垃圾清理 ─────────────────────────────────────────────────── */

async function scanRegistry() {
  regEntries = (await call('ScanRegistry')) || [];
  regSelected = new Set();
  renderRegList();
}

function renderRegList() {
  const list = $('#reg-list');
  list.innerHTML = '';
  regEntries.forEach((e, i) => {
    const row = document.createElement('div');
    row.className = 'row';
    row.style.animationDelay = Math.min(i, 10) * 14 + 'ms';
    row.innerHTML =
      `<input type="checkbox" data-i="${i}" ${regSelected.has(i) ? 'checked' : ''} aria-label="选择 ${esc(e.desc)}"/>` +
      `<span class="badge ${riskClass(e.risk)}">${esc(e.risk)}</span>` +
      `<span class="size">${fmtSize(e.size)}</span>` +
      `<span class="cat" title="${esc(e.category)}">${esc(e.category)}</span>` +
      `<span class="desc" title="${esc(e.key)}">${esc(e.desc)}</span>`;
    list.appendChild(row);
  });
  const total = regEntries.reduce((s, e) => s + e.size, 0);
  $('#reg-stats').textContent = regEntries.length ? `共 ${regEntries.length} 项 · 约 ${fmtSize(total)}` : '未扫描';
  toggleEmpty($('#empty-reg'), regEntries.length === 0);
}

$('#reg-list').addEventListener('change', (ev) => {
  const i = +ev.target.dataset.i;
  if (ev.target.checked) regSelected.add(i); else regSelected.delete(i);
});

function regSelectedKeys() { return [...regSelected].map((i) => regEntries[i].key); }

$('#btn-reg-scan').onclick = () => scanRegistry();
$('#btn-reg-select-all').onclick = () => { regSelected = new Set(regEntries.map((_, i) => i)); renderRegList(); };
$('#btn-reg-select-safe').onclick = () => {
  regSelected = new Set();
  regEntries.forEach((e, i) => { if (e.risk === '安全') regSelected.add(i); });
  renderRegList();
};

$('#btn-reg-clean').onclick = async () => {
  const keys = regSelectedKeys();
  if (!keys.length) { toast('请先勾选要清理的注册表项'); return; }
  const ok = await confirmModal(`确认清理 ${keys.length} 个注册表项？注册表操作不可恢复。`, '确认清理注册表');
  if (!ok) return;
  setCleaning(true);
  let res;
  try {
    res = await call('CleanRegistry', keys);
  } finally {
    setCleaning(false);
  }
  toast(`已清理 ${res.count} 个注册表项，释放约 ${fmtSize(res.freed)}` +
    (res.failed && res.failed.length ? `（失败 ${res.failed.length} 项）` : ''));
  const cleaned = new Set(res.cleaned || []);
  regEntries = regEntries.filter((e) => !cleaned.has(e.key));
  regSelected = new Set();
  renderRegList();
};

/* ── 系统工具（组件存储清理，需 UAC 提权） ──────────────────────────── */

let dismBusy = false; // DISM 任务进行中（分析/清理共用一把锁，后端也只允许单任务）

async function refreshDismInfo() {
  const info = await call('GetDismInfo');
  if (!info) return;
  if (!info.available) {
    $('#dism-status').textContent = 'dism.exe 不可用（可能被安全策略禁用）';
    $('#btn-dism-analyze').disabled = true;
    $('#btn-dism-clean').disabled = true;
    return;
  }
  renderDismReport(info.report);
}

function renderDismReport(r) {
  const parsed = r && (r.actualSize || r.reportedSize || r.reclaimablePkgs);
  show($('#dism-report'), !!parsed);
  show($('#dism-raw'), !!(r && r.raw && !parsed));
  if (r && r.raw) $('#dism-raw-text').textContent = r.raw;
  toggleEmpty($('#empty-dism'), !parsed && !(r && r.raw));

  if (parsed) {
    $('#dism-reported').textContent = r.reportedSize || '—';
    $('#dism-actual').textContent = r.actualSize || '—';
    $('#dism-reclaimable').textContent = r.reclaimablePkgs || '0';
    $('#dism-lastclean').textContent = r.lastCleanup || '从未';
    $('#dism-recommended').textContent = r.recommended ? '建议清理' : '无需清理';
    $('#dism-status').textContent = r.recommended
      ? `建议清理 · 可回收 ${r.reclaimablePkgs || '?'} 个包`
      : '状态良好';
  } else if (r && r.raw) {
    $('#dism-status').textContent = '分析完成（未能解析报告，显示原始输出）';
  } else {
    $('#dism-status').textContent = '尚未分析';
  }
}

function setDismBusy(on, label) {
  dismBusy = on;
  $('#btn-dism-analyze').disabled = on;
  $('#btn-dism-clean').disabled = on;
  const bar = $('#dism-progress');
  show(bar, on);
  if (on) {
    $('#dism-progress-text').textContent = label || '等待提权授权…';
    $('#dism-progress-pct').textContent = '';
    $('#dism-progress-fill').style.width = '0%';
    $('#dism-progress-fill').classList.add('indeterminate');
    $('#dism-progress-line').textContent = '';
  }
}

$('#btn-dism-analyze').onclick = async () => {
  if (dismBusy) { toast('已有组件存储任务进行中'); return; }
  const ok = await confirmModal('分析组件存储需要管理员权限，Windows 将弹出 UAC 授权框。继续？', '分析组件存储');
  if (!ok) return;
  const err = await call('AnalyzeComponentStore');
  if (err) toast(err);
};

$('#btn-dism-clean').onclick = async () => {
  if (dismBusy) { toast('已有组件存储任务进行中'); return; }
  const ok = await confirmModal(
    '组件清理将以管理员身份执行 DISM（约 5-20 分钟），期间请勿关闭本程序。不使用 /ResetBase，清理后仍可卸载已装更新。确认执行？',
    '执行组件存储清理');
  if (!ok) return;
  const err = await call('StartComponentCleanup');
  if (err) toast(err);
};

window.runtime.EventsOn('dism:progress', (p) => {
  if (!p.running && !p.done) return;
  if (p.running) {
    setDismBusy(true, p.kind === 'analyze' ? '正在分析组件存储…' : '正在清理组件存储…');
    if (p.pct > 0) {
      $('#dism-progress-fill').classList.remove('indeterminate');
      $('#dism-progress-fill').style.width = p.pct.toFixed(1) + '%';
      $('#dism-progress-pct').textContent = Math.round(p.pct) + '%';
    }
    $('#dism-progress-line').textContent = p.line || '';
    return;
  }
  // 任务结束
  setDismBusy(false);
  $('#dism-progress-fill').classList.remove('indeterminate');
  $('#dism-progress-fill').style.width = '100%';
  if (p.ok && p.report) renderDismReport(p.report);
  if (!p.ok && p.message) $('#dism-status').textContent = '执行失败';
  toast(p.message || (p.ok ? '任务完成' : '任务失败'));
});

/* ── 导航 ───────────────────────────────────────────────────────────── */

const TAB_LOADERS = { custom: refreshDirs, history: refreshHistory, registry: () => { if (!regEntries.length) scanRegistry(); }, tools: refreshDismInfo };

$$('.rail-item').forEach((btn) => btn.onclick = () => {
  $$('.rail-item').forEach((x) => { x.classList.remove('is-active'); x.removeAttribute('aria-current'); });
  btn.classList.add('is-active');
  btn.setAttribute('aria-current', 'page');

  const tab = btn.dataset.tab;
  $$('.panel').forEach((p) => p.classList.toggle('is-active', p.id === 'tab-' + tab));
  const loader = TAB_LOADERS[tab];
  if (loader) loader();
});

/* ── 自定义 / 排除目录 ──────────────────────────────────────────────── */

async function refreshDirs() {
  const custom = await call('GetCustomDirs');
  const exclude = await call('GetExcludeDirs');
  $('#custom-list').innerHTML = (custom && custom.length)
    ? custom.map((p) => `<li><span class="p" title="${esc(p)}">${esc(p)}</span><button class="x" data-type="custom" data-p="${esc(p)}" aria-label="移除">✕</button></li>`).join('')
    : '<li class="empty">（空）还没有额外纳入扫描的目录</li>';
  $('#exclude-list').innerHTML = (exclude && exclude.length)
    ? exclude.map((p) => `<li><span class="p" title="${esc(p)}">${esc(p)}</span><button class="x" data-type="exclude" data-p="${esc(p)}" aria-label="移除">✕</button></li>`).join('')
    : '<li class="empty">（空）还没有排除项</li>';
}

$('#custom-list').addEventListener('click', onRemoveDir);
$('#exclude-list').addEventListener('click', onRemoveDir);
async function onRemoveDir(ev) {
  if (!ev.target.classList.contains('x')) return;
  const { type, p } = ev.target.dataset;
  if (type === 'custom') await call('RemoveCustomDir', p);
  else await call('RemoveExcludeDir', p);
  refreshDirs();
}

$('#btn-add-custom').onclick = async () => {
  const p = await window.runtime.OpenDirectoryDialog({ Title: '选择要扫描的自定义目录' });
  if (p) { await call('AddCustomDir', p); refreshDirs(); }
};
$('#btn-add-exclude').onclick = async () => {
  const p = await window.runtime.OpenDirectoryDialog({ Title: '选择要排除的目录' });
  if (p) { await call('AddExcludeDir', p); refreshDirs(); }
};

/* ── 清理历史 ───────────────────────────────────────────────────────── */

async function refreshHistory() {
  const h = await call('GetHistory');
  $('#history-body').innerHTML = (h && h.length)
    ? h.map((r) => `<tr><td>${esc(r.time)}</td><td>${fmtSize(r.size)}</td><td title="${esc(r.path)}">${esc(r.path)}</td></tr>`).join('')
    : '<tr><td colspan="3" class="empty">（暂无记录）</td></tr>';
}

/* ── 关于 ───────────────────────────────────────────────────────────── */

$('#btn-about').onclick = () => { lastFocused = document.activeElement; show($('#about'), true); $('#about-close').focus(); };
$('#about-close').onclick = () => {
  show($('#about'), false);
  if (lastFocused && lastFocused.focus) lastFocused.focus();
};

/* ── 入口 ───────────────────────────────────────────────────────────── */

$('#btn-ai').onclick = () => startScan('ai');
$('#btn-all').onclick = () => startScan('all');

initTheme();
renderKpis();
renderChart();
refreshDirs();
