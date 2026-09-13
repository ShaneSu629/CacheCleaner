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

// GitHub 的 ISO 时间（2026-09-06T05:53:16Z）转本地时间 "2026-09-06 13:53"
const fmtTime = (iso) => {
  if (!iso) return '—';
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  const p = (n) => String(n).padStart(2, '0');
  return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}`;
};

const riskClass = (r) => {
  // 后端传中文标签，前端按当前语言翻译显示
  const map = { '安全': 'risk.safe', '谨慎': 'risk.caution', '复核': 'risk.review' };
  const key = map[r];
  if (!key) return '';
  const label = t(key);
  return { '安全': 'ok', '谨慎': 'warn', '复核': 'risk' }[r] || '';
};

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

/* ── 语言切换（i18n） ──────────────────────────────────────────────── */

// 切换语言后重新渲染当前面板（动态内容）
function rerenderCurrentPanel() {
  const active = $('.panel.is-active');
  if (!active) return;
  switch (active.id) {
    case 'tab-scan':
      renderList(); renderChart(); renderKpis();
      break;
    case 'tab-registry':
      renderRegList();
      break;
    case 'tab-custom':
      refreshDirs();
      break;
    case 'tab-history':
      refreshHistory();
      break;
    case 'tab-update':
      if (lastUpdateInfo) renderUpdate(lastUpdateInfo);
      break;
    case 'tab-uninstaller':
      renderUninstaller();
      break;
    default:
      break;
  }
}

function initLang() {
  applyI18n();
  $('#btn-lang').onclick = () => { toggleLang(); };
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
  if (!$('#modal').hidden && $('#modal').dataset.mode !== 'alert') $('#modal-cancel').click();
  else if (!$('#about').hidden) show($('#about'), false);
  else if (!$('#update-alert').hidden) hideUpdateAlert();
});

// 顶部醒目确认框：清理失败等需要用户确认的结果，顶在窗口最上方，
// 遮罩更深、层级最高，必须点「知道了」才关闭（点遮罩/Esc 不关）。
function alertModal(text, title = '清理结果') {
  const scrim = $('#modal');
  scrim.classList.add('top');
  scrim.dataset.mode = 'alert';
  $('#modal-title').textContent = title;
  $('#modal-text').textContent = text;
  $('#modal-ok').textContent = '知道了';
  $('#modal-ok').className = 'btn btn-accent';
  show($('#modal-cancel'), false); // 单项提醒不需要取消按钮
  show(scrim, true);
  $('#modal-ok').onclick = () => {
    scrim.classList.remove('top');
    delete scrim.dataset.mode;
    show(scrim, false);
    $('#modal-ok').onclick = null;
    $('#modal-ok').className = 'btn btn-danger';
    $('#modal-ok').textContent = '确认清理';
    show($('#modal-cancel'), true);
    if (lastFocused && lastFocused.focus) lastFocused.focus();
  };
  $('#modal-ok').focus();
}

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
  const failed = res.failed || [];
  const sched = res.scheduledReboot || [];
  if (failed.length) {
    const busyPaths = failed
      .filter((f) => f.error && /Access is denied|denied|being used|另一个程序|占用/.test(f.error))
      .map((f) => f.path);
    // 查占用进程（Restart Manager），弹窗里列出占用者并支持一键结束
    let lockers = [];
    if (busyPaths.length) {
      try { lockers = await call('FindLockers', busyPaths) || []; } catch (e) { lockers = []; }
    }
    if (lockers.length) {
      showLockerModal(res, failed, busyPaths, lockers, sched);
    } else {
      let detail = '';
      if (busyPaths.length) {
        detail = `已清理 ${res.count} 项，释放 ${fmtSize(res.freed)}。\n有 ${failed.length} 项因文件被占用而失败（未识别出占用进程），退出对应软件后重试。`;
      } else {
        detail = `已清理 ${res.count} 项，释放 ${fmtSize(res.freed)}。\n有 ${failed.length} 项清理失败，详情见日志。`;
      }
      if (sched.length) {
        detail += `\n\n另有 ${sched.length} 项被占用，已登记为「下次开机时自动删除」。`;
      }
      alertModal(detail, '清理完成（部分失败）');
    }
  } else if (sched.length) {
    alertModal(
      `已清理 ${res.count} 项，释放 ${fmtSize(res.freed)}。\n\n其中 ${sched.length} 项正被运行中的程序占用，已登记为「下次开机时自动删除」，重启电脑后生效。`,
      '清理完成（含延迟删除）');
  } else {
    toast(`已清理 ${res.count} 项，释放 ${fmtSize(res.freed)}`);
  }
  // 只移除真正清理成功的项，失败/被排除的项保留供重试
  const cleaned = new Set(res.cleaned || []);
  entries = entries.filter((e) => !cleaned.has(e.path));
  selected = new Set();
  renderList();
  renderChart();
  renderKpis();
};

// 占用进程弹窗：列出占用者，用户可以一键结束（后端校验安全后才杀）。
async function showLockerModal(res, failed, busyPaths, lockers, sched) {
  const scrim = $('#modal');
  scrim.classList.add('top');
  scrim.dataset.mode = 'alert';
  $('#modal-title').textContent = '文件被占用，清理未完全成功';
  const safeNames = lockers.map((l) => `${l.name}（${l.safe ? '可结束' : '系统进程'}）`).join('、');
  const itemList = lockers.slice(0, 8).map((l) =>
    `<span class="locker-chip ${l.safe ? '' : 'protected'}">${esc(l.name)}</span>`).join(' ');
  const schedNote = sched && sched.length
    ? `<br/>另有 ${sched.length} 项已登记为「下次开机自动删除」。`
    : '';
  $('#modal-text').innerHTML =
    `已清理 ${res.count} 项，释放 ${fmtSize(res.freed)}。<br/>` +
    `${failed.length} 项清理失败。<br/><br/>` +
    `占用文件进程：${itemList || '（未识别）'}<br/><br/>` +
    `<small>结束进程会关闭对应软件（未保存的数据可能丢失）。</small>` + schedNote;
  // 底部动作：结束进程按钮 + 知道了
  $('#modal-ok').textContent = '知道了';
  $('#modal-ok').className = 'btn btn-accent';
  $('#modal-ok').onclick = () => closeLockerModal();
  show($('#modal-cancel'), false);
  show(scrim, true);
  $('#modal-ok').focus();

  // 结束进程按钮（新增在 modal-ok 前）
  let btnKill = $('#btn-kill-locker');
  if (!btnKill) {
    btnKill = document.createElement('button');
    btnKill.id = 'btn-kill-locker';
    btnKill.className = 'btn btn-danger';
    btnKill.textContent = '结束占用进程';
    $('#modal-ok').parentNode.insertBefore(btnKill, $('#modal-ok'));
  }
  show(btnKill, lockers.some((l) => l.safe));
  btnKill.onclick = async () => {
    btnKill.disabled = true;
    btnKill.textContent = '正在结束…';
    for (const l of lockers) {
      if (!l.safe) continue;
      try {
        const err = await call('KillProcess', busyPaths, l.pid);
        if (err) toast(err);
      } catch (e) { toast('结束进程失败'); }
    }
    closeLockerModal();
    toast('占用进程已结束，可重新扫描清理');
  };
}

function closeLockerModal() {
  const scrim = $('#modal');
  scrim.classList.remove('top');
  delete scrim.dataset.mode;
  show(scrim, false);
  $('#modal-ok').onclick = null;
  $('#modal-ok').className = 'btn btn-danger';
  $('#modal-ok').textContent = '确认清理';
  $('#modal-text').textContent = '';
  show($('#modal-cancel'), true);
  const btnKill = $('#btn-kill-locker');
  if (btnKill) { btnKill.onclick = null; btnKill.remove(); }
}

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
    $('#btn-dism-repair').disabled = true;
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
  $('#btn-dism-repair').disabled = on;
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

$('#btn-dism-repair').onclick = async () => {
  if (dismBusy) { toast('已有组件存储任务进行中'); return; }
  const ok = await confirmModal(
    '修复组件存储将以管理员身份执行 DISM RestoreHealth（最长约 30 分钟，从 Windows Update 拉取修复源），期间请勿关闭本程序。适用于清理反复报「拒绝访问」的情况。确认执行？',
    '修复组件存储');
  if (!ok) return;
  const err = await call('StartComponentRepair');
  if (err) toast(err);
};

window.runtime.EventsOn('dism:progress', (p) => {
  if (!p.running && !p.done) return;
  if (p.running) {
    const kindLabel = p.kind === 'analyze' ? '正在分析组件存储…'
      : p.kind === 'repair' ? '正在修复组件存储…'
      : '正在清理组件存储…';
    setDismBusy(true, kindLabel);
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

const TAB_LOADERS = { custom: refreshDirs, history: refreshHistory, registry: () => { if (!regEntries.length) scanRegistry(); }, tools: refreshDismInfo, update: refreshUpdate };

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
  const p = await call('ChooseDirectory', '选择要扫描的自定义目录');
  if (p) { await call('AddCustomDir', p); refreshDirs(); }
};
$('#btn-add-exclude').onclick = async () => {
  const p = await call('ChooseDirectory', '选择要排除的目录');
  if (p) { await call('AddExcludeDir', p); refreshDirs(); }
};

/* ── 清理历史 ───────────────────────────────────────────────────────── */

// 从路径推断分类标签
function inferCategory(path) {
  const p = path.toLowerCase();
  const rules = [
    // AI / 开发工具
    [/workbuddy|trae|codebuddy|codex|cursor|cline|windsurf|aider|devika|swe-agent|open.?debate/i, 'AI 工具'],
    [/通义灵码|tongyi|lingma|doubao|豆包/i, 'AI 工具'],
    [/claude|gpt|chatgpt|openai|copilot/i, 'AI 工具'],
    [/vscode|code.*extension|cachedextension|code.?cache/i, 'IDE'],
    [/intellij|jetbrains|idea|webstorm|pycharm/i, 'IDE'],
    // 浏览器
    [/chrome|google/i, '浏览器'],
    [/edge|microsoft.*edg/i, '浏览器'],
    [/firefox|mozilla/i, '浏览器'],
    // IM / 社交
    [/微信|wechat|xwechat|weixin/i, '微信'],
    [/qq|tencent.*qq/i, 'QQ'],
    [/telegram/i, 'Telegram'],
    [/discord/i, 'Discord'],
    // 包管理器 / 语言运行时
    [/npm-cache|yarn[/\\]cache|pnpm-store|uv[/\\]cache|go[/\\]pkg[/\\]mod/i, '包管理器'],
    [/node_modules|\.nuget|\.gradle|\.m2|\.cargo/i, '包管理器'],
    // 系统级
    [/windows[/\\]upgrade|windows10upgrade|win[sx]s/i, '系统升级'],
    [/packages[/\\].*localcache|component_crx_cache/i, '系统组件'],
    [/nvidia|amd[/\\]vkcache|dxcache|glcache|shader/i, '显卡着色器'],
    // 下载/网盘
    [/baidunetdisk|adrive|aliyundrive|115[/\\]cache/i, '网盘'],
    // 其他应用
    [/postman|insomnia/i, 'API 工具'],
    [/electron|code.?cache|gpucache|dawn|blob_storage|crashpad/i, 'Electron'],
  ];
  for (const [re, label] of rules) {
    if (re.test(p)) return label;
  }
  // 注册表项（HKEY_ 开头）
  if (/^hkey_|^hk[clm]_/i.test(p)) return '注册表';
  return '其他';
}

// 分类标签颜色
const catColors = {
  'AI 工具':  '#8b5cf6',
  'IDE':      '#6366f1',
  '浏览器':   '#3b82f6',
  '微信':     '#22c55e',
  'QQ':       '#06b6d4',
  'Telegram': '#0ea5e9',
  'Discord':  '#5865f2',
  '包管理器': '#f59e0b',
  '系统升级': '#ef4444',
  '系统组件': '#f97316',
  '显卡着色器': '#ec4899',
  '网盘':     '#14b8a6',
  'API 工具': '#a855f7',
  'Electron': '#64748b',
  '注册表':   '#dc2626',
  '其他':     '#94a3b8',
};

function catBadge(cat) {
  const c = catColors[cat] || '#94a3b8';
  return `<span class="hist-cat" style="--cat-c:${c}">${esc(cat)}</span>`;
}

async function refreshHistory() {
  const h = await call('GetHistory');
  const container = $('#history-groups');
  if (!container) return;
  if (!h || !h.length) {
    container.innerHTML = '';
    toggleEmpty($('#empty-history'), true);
    return;
  }
  toggleEmpty($('#empty-history'), false);

  // 按时间分组（同一秒的记录归为一次清理批次）
  const groups = [];
  let cur = null;
  for (const r of h) {
    if (!cur || cur.time !== r.time) {
      cur = { time: r.time, items: [], total: 0, cats: {} };
      groups.push(cur);
    }
    cur.items.push(r);
    cur.total += r.size;
    const cat = inferCategory(r.path);
    cur.cats[cat] = (cur.cats[cat] || 0) + r.size;
  }

  // 渲染（最新的在前）
  container.innerHTML = groups.reverse().map((g, gi) => {
    // 分类汇总条
    const catSummary = Object.entries(g.cats)
      .sort((a, b) => b[1] - a[1])
      .map(([cat, size]) => `${catBadge(cat)} ${fmtSize(size)}`)
      .join(' ');

    return `<div class="hist-group" data-gi="${gi}">` +
      `<button class="hist-header" aria-expanded="false" aria-label="展开清理批次 ${esc(g.time)}">` +
        `<span class="hist-time">${esc(g.time)}</span>` +
        `<span class="hist-summary">${fmtSize(g.total)} · ${g.items.length} 项</span>` +
        `<span class="hist-cats">${catSummary}</span>` +
        `<svg class="icon hist-arrow"><use href="#i-scan"/></svg>` +
      `</button>` +
      `<div class="hist-detail" hidden>` +
        `<table class="hist"><tbody>` +
        g.items.map((r) => {
          const cat = inferCategory(r.path);
          return `<tr><td>${catBadge(cat)}</td><td>${fmtSize(r.size)}</td><td title="${esc(r.path)}">${esc(r.path)}</td></tr>`;
        }).join('') +
        `</tbody></table>` +
      `</div>` +
    `</div>`;
  }).join('');

  // 绑定展开/?收起
  container.querySelectorAll('.hist-header').forEach((btn) => {
    btn.onclick = () => {
      const detail = btn.nextElementSibling;
      const expanded = !detail.hidden;
      detail.hidden = expanded;
      btn.setAttribute('aria-expanded', !expanded);
      btn.classList.toggle('is-open', !expanded);
    };
  });
}

/* ── 关于 ───────────────────────────────────────────────────────────── */

$('#btn-about').onclick = () => {
  lastFocused = document.activeElement;
  show($('#about'), true);
  // 填充真实版本号（后端注入）
  call('GetVersion').then((v) => {
    const el = $('#about-version');
    if (el && v) el.textContent = v;
  }).catch(() => {});
  $('#about-close').focus();
};
$('#about-close').onclick = () => {
  show($('#about'), false);
  if (lastFocused && lastFocused.focus) lastFocused.focus();
};

/* ── 软件更新 ──────────────────────────────────────────────────────── */

let lastUpdateInfo = null; // 缓存最近一次更新信息，供弹窗/页面共享
let dlStatus = { state: 'idle', pct: 0 }; // 下载状态（前端轮询 GetUpdateDownload）
let dlTimer = null; // 下载进度轮询定时器

// 轮询下载进度（1 秒一次，下载完成/出错后自动停止）
function startDlPolling() {
  if (dlTimer) return;
  dlTimer = setInterval(async () => {
    let d;
    try { d = await call('GetUpdateDownload'); } catch (e) { return; }
    dlStatus = d || { state: 'idle', pct: 0 };
    renderUpdateDl(dlStatus.state, dlStatus.pct);
    // 状态变化后重新渲染按钮（下载完成 → 显示「更新并重启」）
    if (lastUpdateInfo) renderUpdate(lastUpdateInfo);
    if (dlStatus.state === 'done' || dlStatus.state === 'error' || dlStatus.state === 'idle') {
      clearInterval(dlTimer);
      dlTimer = null;
      if (dlStatus.state === 'done') {
        promptApplyUpdate();
      } else if (dlStatus.state === 'error') {
        toast('下载失败：' + (dlStatus.message || '未知错误'));
      }
    }
  }, 1000);
}

// 下载完成：立即弹顶部确认框，让用户马上更新
let applyPromptShown = false;
function promptApplyUpdate() {
  if (applyPromptShown) return; // 只弹一次，避免重复骚扰
  applyPromptShown = true;
  const scrim = $('#modal');
  scrim.classList.add('top');
  scrim.dataset.mode = 'alert';
  $('#modal-title').textContent = '新版本已就绪';
  $('#modal-text').textContent = '新版本已下载完成，是否立即更新？\n更新将关闭程序、替换为新版本并自动重新打开，全程约几秒钟。';
  $('#modal-ok').textContent = '立即更新';
  $('#modal-ok').className = 'btn btn-accent';
  show($('#modal-cancel'), true);
  $('#modal-cancel').textContent = '稍后再说';
  show(scrim, true);
  $('#modal-ok').focus();

  const doApply = async () => {
    const err = await call('ApplyUpdateAndRestart');
    if (err) { toast(err); return; }
    try { window.runtime.Quit(); } catch (e) { /* 退出失败由 bat 脚本兜底等待 */ }
  };

  $('#modal-ok').onclick = doApply;
  $('#modal-cancel').onclick = () => {
    scrim.classList.remove('top');
    delete scrim.dataset.mode;
    show(scrim, false);
    $('#modal-ok').onclick = null;
    $('#modal-cancel').onclick = null;
    $('#modal-ok').className = 'btn btn-danger';
    $('#modal-ok').textContent = '确认清理';
    $('#modal-cancel').textContent = '取消';
    applyPromptShown = false; // 取消后如果用户再点下载完成还能再弹
    toast('可在「软件更新」页随时点击「更新并重启」');
  };
}

// 渲染设置页「软件更新」tab 的版本信息
function renderUpdate(info) {
  if (!info) return;
  lastUpdateInfo = info;

  // 版本信息表
  const cur = $('#upd-current');
  const lat = $('#upd-latest');
  const pub = $('#upd-published');
  const sz  = $('#upd-size');
  if (cur) cur.textContent = info.current || '—';
  if (lat) lat.textContent = info.latest || '—';
  if (pub) pub.textContent = fmtTime(info.publishedAt);
  if (sz)  sz.textContent = info.size ? fmtSize(info.size) : '—';

  // 状态标签
  const status = $('#update-status');
  if (status) {
    if (info.hasUpdate) {
      status.textContent = info.snoozed ? '有更新（已推迟提醒）' : info.skipped ? '有更新（已跳过）' : '有更新可用';
      status.className = 'card-hint ' + (info.snoozed || info.skipped ? '' : 'has-update');
    } else {
      status.textContent = info.current ? '已是最新' : '未检查';
      status.className = 'card-hint';
    }
  }

  // 更新说明
  const notesWrap = $('#update-notes');
  const notesText = $('#update-notes-text');
  if (notesWrap && notesText) {
    const hasNotes = info.notes && info.notes.trim();
    show(notesWrap, hasNotes);
    if (hasNotes) notesText.textContent = info.notes;
  }

  // 按钮可见性 / 可用性
  const btnNow   = $('#btn-update-now');       // 更新并重启
  const btnDl    = $('#btn-update-download');  // 后台下载
  const btnLater = $('#btn-update-later');
  const btnSkip  = $('#btn-update-skip');

  const hasUpd = info.hasUpdate && !info.skipped;
  if (btnNow)   show(btnNow,   hasUpd && dlStatus.state === 'done'); // 只有下载完成后才能更新并重启
  if (btnDl)    show(btnDl,    hasUpd && dlStatus.state !== 'done'); // 下载完成前可点后台下载
  if (btnLater) btnLater.disabled = !info.hasUpdate || info.snoozed || info.skipped;
  if (btnSkip)  btnSkip.disabled  = !info.hasUpdate || info.skipped;

  // 下载进度条
  renderUpdateDl(dlStatus.state, dlStatus.pct);

  // 侧栏版本号
  const rv = $('#rail-version');
  if (rv && info.current) rv.textContent = info.current;
}

// 渲染下载进度
function renderUpdateDl(state, pct) {
  const bar = $('#update-dl-progress');
  if (!bar) return;
  if (!state || state === 'idle') { show(bar, false); return; }
  show(bar, true);
  const fill = $('#update-dl-fill');
  const txt = $('#update-dl-text');
  const pctEl = $('#update-dl-pct');
  switch (state) {
    case 'downloading':
      txt.textContent = '正在后台下载…';
      fill.classList.remove('indeterminate');
      fill.style.width = (pct || 0).toFixed(1) + '%';
      pctEl.textContent = Math.round(pct || 0) + '%';
      break;
    case 'verifying':
      txt.textContent = '正在校验完整性…';
      pctEl.textContent = '100%';
      break;
    case 'done':
      txt.textContent = '下载完成，可更新并重启';
      fill.style.width = '100%';
      pctEl.textContent = '100%';
      break;
    case 'error':
      txt.textContent = '下载失败';
      fill.classList.add('indeterminate');
      pctEl.textContent = '';
      break;
    default:
      break;
  }
}

// 启动更新提醒弹窗（仅在有更新 && 未推迟 && 未跳过时自动弹出）
function showUpdateAlert(info) {
  if (!info || !info.hasUpdate || info.snoozed || info.skipped) return;
  const alertEl = $('#update-alert');
  const text = $('#update-alert-text');
  if (!alertEl || !text) return;
  let msg = `当前 ${info.current}，最新 ${info.latest}。可前往下载页获取新版本。`;
  if (info.notes) {
    // Release Notes 只显示前 3 行，避免弹窗过长
    const lines = info.notes.split('\n').filter((l) => l.trim()).slice(0, 3);
    if (lines.length) msg += '\n\n更新内容：\n' + lines.map((l) => '· ' + l.trim()).join('\n');
  }
  text.textContent = msg;
  show(alertEl, true);
  $('#btn-alert-now').focus();
}

// 关闭更新提醒弹窗
function hideUpdateAlert() {
  show($('#update-alert'), false);
}

// 初始化更新功能：绑定按钮 + 注册后端事件
function initUpdate() {
  // 后端 startup 5s 后会推送 update:info
  window.runtime.EventsOn('update:info', (info) => {
    renderUpdate(info);
    showUpdateAlert(info);
    // 后端可能已自动开始后台下载（无静默/跳过时），开始轮询进度
    call('GetUpdateDownload').then((d) => {
      dlStatus = d || { state: 'idle', pct: 0 };
      if (dlStatus.state === 'downloading' || dlStatus.state === 'verifying') {
        startDlPolling();
      } else if (dlStatus.state === 'done') {
        renderUpdateDl('done', 100);
        renderUpdate(info);
        // 上次已下载完成（重启后恢复）：立即提示更新
        promptApplyUpdate();
      }
    }).catch(() => {});
  });

  // 设置页「检查更新」按钮（force=true，忽略静默/跳过策略）
  const btnCheck = $('#btn-check-update');
  if (btnCheck) {
    btnCheck.onclick = async () => {
      btnCheck.disabled = true;
      try {
        const info = await call('CheckUpdate', true);
        renderUpdate(info);
        if (info.hasUpdate) {
          toast(`发现新版本 ${info.latest}`);
        } else {
          toast('当前已是最新版本');
        }
      } catch (e) {
        toast('检查更新失败');
      } finally {
        btnCheck.disabled = false;
      }
    };
  }

  // 设置页按钮：更新并重启（下载完成后出现）
  const btnNow = $('#btn-update-now');
  if (btnNow) {
    btnNow.onclick = async () => {
      if (btnNow.disabled) return; // 防连点
      btnNow.disabled = true;
      const ok = await confirmModal(
        '将关闭程序、替换为新版本并自动重新打开。确定现在更新吗？',
        '更新并重启');
      if (!ok) { btnNow.disabled = false; return; }
      const err = await call('ApplyUpdateAndRestart');
      if (err) { toast(err); btnNow.disabled = false; return; }
      // 替换脚本已在后台启动，退出本程序让它接管
      try { window.runtime.Quit(); } catch (e) { /* 退出失败由脚本兜底等待 */ }
    };
  }

  // 设置页按钮：后台下载
  const btnDl = $('#btn-update-download');
  if (btnDl) {
    btnDl.onclick = async () => {
      if (btnDl.disabled) return; // 防连点
      btnDl.disabled = true;
      applyPromptShown = false; // 新一轮下载，重置"已弹过"标记
      const err = await call('StartUpdateDownload');
      if (err) { toast(err); btnDl.disabled = false; return; }
      hideUpdateAlert();
      toast('已开始后台下载，完成后会提示');
      startDlPolling();
    };
  }

  const btnLater = $('#btn-update-later');
  if (btnLater) {
    btnLater.onclick = async () => {
      await call('SnoozeUpdate', 24);
      toast('已推迟提醒，24 小时后再提醒');
      hideUpdateAlert();
      // 重新渲染状态
      const info = await call('CheckUpdate', false);
      renderUpdate(info);
    };
  }

  const btnSkip = $('#btn-update-skip');
  if (btnSkip) {
    btnSkip.onclick = async () => {
      const v = lastUpdateInfo && lastUpdateInfo.latest ? lastUpdateInfo.latest : '';
      await call('SkipUpdate', v);
      toast('已跳过此版本');
      hideUpdateAlert();
      const info = await call('CheckUpdate', false);
      renderUpdate(info);
    };
  }

  // 启动提醒弹窗按钮：后台下载
  const alertDl = $('#btn-alert-download');
  if (alertDl) {
    alertDl.onclick = async () => {
      const err = await call('StartUpdateDownload');
      if (err) { toast(err); return; }
      hideUpdateAlert();
      toast('已开始后台下载，完成后会提示');
      startDlPolling();
    };
  }

  // 启动提醒弹窗按钮：前往下载页（保留手动更新入口）
  const alertNow = $('#btn-alert-now');
  if (alertNow) {
    alertNow.onclick = () => {
      const url = lastUpdateInfo && lastUpdateInfo.url ? lastUpdateInfo.url : '';
      call('OpenDownloadPage', url);
      hideUpdateAlert();
    };
  }

  const alertLater = $('#btn-alert-later');
  if (alertLater) {
    alertLater.onclick = async () => {
      await call('SnoozeUpdate', 24);
      hideUpdateAlert();
      const info = await call('CheckUpdate', false);
      renderUpdate(info);
    };
  }

  const alertSkip = $('#btn-alert-skip');
  if (alertSkip) {
    alertSkip.onclick = async () => {
      const v = lastUpdateInfo && lastUpdateInfo.latest ? lastUpdateInfo.latest : '';
      await call('SkipUpdate', v);
      hideUpdateAlert();
      const info = await call('CheckUpdate', false);
      renderUpdate(info);
    };
  }

  // 弹窗点击遮罩关闭
  const alertEl = $('#update-alert');
  if (alertEl) {
    alertEl.addEventListener('click', (e) => {
      if (e.target === alertEl) hideUpdateAlert();
    });
  }

  // 初始填充侧栏版本号
  call('GetVersion').then((v) => {
    const rv = $('#rail-version');
    if (rv && v) rv.textContent = v;
  }).catch(() => {});
}

// 打开「软件更新」tab 时刷新数据
async function refreshUpdate() {
  try {
    // 先拿当前版本，再静默检查（受静默/跳过策略约束）
    const v = await call('GetVersion');
    const info = await call('CheckUpdate', false);
    renderUpdate(Object.assign({}, info, { current: info.current || v }));
    // 恢复下载状态（如果上次下载已完成但未应用）
    const d = await call('GetUpdateDownload');
    dlStatus = d || { state: 'idle', pct: 0 };
    renderUpdateDl(dlStatus.state, dlStatus.pct);
    if (dlStatus.state === 'downloading' || dlStatus.state === 'verifying') startDlPolling();
    renderUpdate(Object.assign({}, info, { current: info.current || v }));
  } catch (e) { /* 静默 */ }
}

/* ── 插件系统 ─────────────────────────────────────────────────────── */

let uninstallApps = []; // 卸载插件：软件列表缓存

// 初始化插件：查询已启用插件，注入侧边栏入口 + 管理界面
async function initPlugins() {
  await renderPluginRail();
  await renderPluginMgr();
}

// 注入侧边栏「插件」分组的脚本插件入口（仅已启用且有效的脚本型插件）
async function renderPluginRail() {
  let list = [];
  try { list = await call('ListPlugins') || []; } catch (e) { return; }
  // 只注入脚本型插件（能力型插件已移除，功能型扩展由开发者内置到主程序）
  const enabled = list.filter((p) => p.valid && p.enabled && p.type === 'script');

  const slot = $('#rail-plugin-slot');
  if (!slot) return;
  slot.innerHTML = '';
  if (!enabled.length) return;

  for (const p of enabled) {
    const name = currentLang === 'zh-CN' ? p.nameZh : p.nameEn;
    const tabId = 'script-' + p.id;
    const btn = document.createElement('button');
    btn.className = 'rail-item';
    btn.dataset.tab = tabId;
    btn.dataset.pluginId = p.id;
    btn.innerHTML = `<svg class="icon"><use href="#${p.icon || 'i-layers'}"/></svg><span>${esc(name)}</span>`;
    btn.onclick = async () => {
      // 关键：先加载内容（脚本执行完、内容已填好），再切换面板。
      // 这样切换时新面板内容已经是完整的，绝不会出现「旧面板已隐藏、
      // 新面板还是空白」的闪屏帧。脚本通常只要几十毫秒，用户无感。
      await loadScriptPlugin(p.id);

      $$('.rail-item').forEach((x) => { x.classList.remove('is-active'); x.removeAttribute('aria-current'); });
      btn.classList.add('is-active');
      btn.setAttribute('aria-current', 'page');
      // 内容已就绪，再切换面板（此时 ensureScriptPanel 已确保面板存在）
      ensureScriptPanel(p.id);
      $$('.panel').forEach((pl) => pl.classList.toggle('is-active', pl.id === 'tab-' + tabId));
    };
    slot.appendChild(btn);

    // 预热：预先创建面板 DOM（不执行脚本、不激活），让 WebView2 提前完成
    // 首次光栅化/合成层缓存。这样用户点击时面板已存在，只是 toggle 显示，
    // 避免「动态插入新元素触发整窗重绘」的闪屏（重启后首次点击才闪的根因）。
    ensureScriptPanel(p.id);
  }
}

// 确保脚本插件面板已创建（同步，幂等）。返回面板 DOM。
// 分离「创建面板」与「加载内容」：创建必须同步完成，保证切换 tab 时面板已存在，
// 不会出现「旧面板已隐藏、新面板未创建」的空白帧闪屏。
function ensureScriptPanel(id) {
  let panel = $('#tab-script-' + id);
  if (panel) return panel;
  panel = document.createElement('section');
  panel.className = 'panel';
  panel.id = 'tab-script-' + id;
  panel.setAttribute('role', 'tabpanel');
  panel.innerHTML = `
    <div class="panel-head reveal" style="--d:0ms">
      <div class="panel-title">
        <h2 id="script-title-${id}"></h2>
        <p class="sub" id="script-sub-${id}"></p>
      </div>
      <div class="actions">
        <button class="btn btn-subtle" id="script-reload-${id}">
          <svg class="icon"><use href="#i-scan"/></svg><span data-i18n="plugin.mgr.reload">重新扫描</span>
        </button>
      </div>
    </div>
    <section class="card reveal" style="--d:40ms">
      <div id="script-body-${id}"></div>
      <div class="empty" id="script-empty-${id}" hidden>
        <svg class="icon"><use href="#i-empty"/></svg>
        <p></p>
      </div>
    </section>
  `;
  document.querySelector('main .container, main').appendChild(panel);
  const reloadBtn = panel.querySelector(`#script-reload-${id}`);
  reloadBtn.onclick = () => { panel.dataset.loaded = '0'; loadScriptPlugin(id); };
  return panel;
}

// 加载脚本插件内容（异步）：执行脚本并渲染结果。
async function loadScriptPlugin(id) {
  const panel = ensureScriptPanel(id);
  const body = panel.querySelector(`#script-body-${id}`);
  const empty = panel.querySelector(`#script-empty-${id}`);
  const titleEl = panel.querySelector(`#script-title-${id}`);

  // 复用已有面板时不重新执行脚本、不闪屏；仅首次（或显式"重新扫描"）才加载。
  if (panel.dataset.loaded === '1') {
    return;
  }
  panel.dataset.loaded = '1';
  body.innerHTML = '';
  empty.hidden = true;

  let res;
  try {
    res = await call('RunPluginScript', id);
  } catch (e) {
    empty.hidden = false;
    empty.querySelector('p').textContent = '插件执行失败: ' + e;
    return;
  }
  titleEl.textContent = res.title || id;
  // 直接渲染内容（无动画、无骨架屏，避免 WebView2 整窗重绘闪烁）
  if (res.html) {
    body.innerHTML = res.html;
  } else if (res.text) {
    body.textContent = res.text;
  }
  // 日志（若有）追加到正文下方
  if (res.logs && res.logs.length) {
    const logBox = document.createElement('pre');
    logBox.className = 'plugin-logs';
    logBox.textContent = res.logs.join('\n');
    body.appendChild(logBox);
  }
  if (!res.html && !res.text && !(res.logs && res.logs.length)) {
    empty.hidden = false;
    empty.querySelector('p').textContent = '插件未输出任何内容';
  }
}

// 渲染插件管理界面（列表 + 启用/禁用开关 + 详情）。
// 只展示脚本型插件（type=script）；功能性扩展由开发者内置到主程序，不在插件系统出现。
async function renderPluginMgr() {
  let list = [];
  try { list = await call('ListPlugins') || []; } catch (e) { list = []; }
  // 只保留脚本型插件
  list = list.filter((p) => p.type === 'script');

  const wrap = $('#plugin-mgr-list');
  const empty = $('#empty-plugins');
  if (!wrap) return;

  toggleEmpty(empty, list.length === 0);
  if (!list.length) { wrap.innerHTML = ''; return; }

  wrap.innerHTML = list.map((p) => {
    const name = currentLang === 'zh-CN' ? p.nameZh : p.nameEn;
    const desc = currentLang === 'zh-CN' ? p.descZh : p.descEn;
    const bad = !p.valid
      ? `<em class="badge risk">${esc(p.errMsg || '无效')}</em>`
      : '';
    const ver = p.version ? `<span class="card-hint">v${esc(p.version)}</span>` : '';
    const author = p.author ? `<span class="card-hint">${esc(p.author)}</span>` : '';
    return `<div class="plugin-row" data-id="${esc(p.id)}">
      <div class="plugin-info">
        <div class="plugin-name">${esc(name)} ${ver} ${author} ${bad}</div>
        ${desc ? `<div class="plugin-desc">${esc(desc)}</div>` : ''}
      </div>
      <div class="plugin-actions">
        <button class="btn btn-sm btn-danger" data-plugin-remove>${t('plugin.mgr.uninstall')}</button>
        <label class="plugin-switch">
          <input type="checkbox" data-plugin-toggle ${p.enabled ? 'checked' : ''} ${p.valid ? '' : 'disabled'}>
          <span class="switch-track"></span>
        </label>
      </div>
    </div>`;
  }).join('');
}

// 插件管理：启用/禁用开关（事件委托）
$('#plugin-mgr-list').addEventListener('change', async (ev) => {
  const cb = ev.target.closest('input[data-plugin-toggle]');
  if (!cb) return;
  const row = cb.closest('.plugin-row');
  const id = row.dataset.id;
  await call('SetPluginEnabled', id, cb.checked);
  toast(cb.checked ? t('plugin.mgr.enabled') : t('plugin.mgr.disabled'));
  await renderPluginRail(); // 刷新侧边栏入口
});

// 插件管理：卸载按钮（事件委托）
$('#plugin-mgr-list').addEventListener('click', async (ev) => {
  const btn = ev.target.closest('button[data-plugin-remove]');
  if (!btn) return;
  const row = btn.closest('.plugin-row');
  const id = row.dataset.id;
  const ok = await confirmModal(
    t('plugin.mgr.uninstall.confirm', { id }),
    t('plugin.mgr.uninstall'));
  if (!ok) return;
  const err = await call('UninstallPlugin', id);
  if (err) { toast(err); return; }
  toast(t('plugin.mgr.uninstalled'));
  await renderPluginMgr();
  await renderPluginRail();
});

// 安装插件：选择源目录（含 manifest.json）复制到插件目录
$('#btn-plugin-install').onclick = async () => {
  const dir = await call('ChooseDirectory', t('plugin.mgr.install.title'));
  if (!dir) return;
  const err = await call('InstallPlugin', dir);
  if (err) { toast(err); return; }
  toast(t('plugin.mgr.installed'));
  await renderPluginMgr();
  await renderPluginRail();
};

// 重新扫描 + 打开插件目录
$('#btn-plugin-reload').onclick = async () => {
  await renderPluginMgr();
  await renderPluginRail();
  toast(t('plugin.mgr.reloaded'));
};
$('#btn-plugin-openfolder').onclick = () => call('OpenPluginDir');

// 插件管理 tab 加载器
TAB_LOADERS.plugins = () => { renderPluginMgr(); renderPluginRail(); };

// 软件卸载插件面板渲染
async function renderUninstaller() {
  const list = $('#uninstall-list');
  const empty = $('#empty-uninstall');
  if (!list) return;

  try {
    uninstallApps = await call('ListUninstallApps') || [];
  } catch (e) {
    uninstallApps = [];
  }

  if (!uninstallApps.length) {
    list.innerHTML = '';
    toggleEmpty(empty, true);
    return;
  }
  toggleEmpty(empty, false);

  list.innerHTML = uninstallApps.map((a) => {
    const pup = a.pup
      ? `<em class="badge risk" title="${esc(a.pupReason)}">${t('uninstall.pup')}</em>`
      : '';
    const size = a.size ? `<span class="size">${fmtSize(a.size)}</span>` : '<span class="size">—</span>';
    // 版本号：同名软件（如新旧两个微信）靠它区分，附加在名称后用小字显示
    const ver = a.version ? `<span class="ver">${esc(a.version)}</span>` : '';
    return `<div class="row" data-key="${esc(a.key)}">
      <span class="path" title="${esc(a.name)}">${esc(a.name)} ${ver}</span>
      ${size}
      <span class="cat" title="${esc(a.publisher || '')}">${esc(a.publisher || '')}</span>
      ${pup}
      <button class="btn btn-sm btn-subtle" data-act="uninstall">${t('uninstall.uninstall')}</button>
      ${a.pup ? `<button class="btn btn-sm btn-danger" data-act="force">${t('uninstall.force')}</button>` : ''}
    </div>`;
  }).join('');

  // PUP 警告条
  const pups = uninstallApps.filter((a) => a.pup);
  const callout = $('#pup-callout');
  if (callout) {
    show(callout, pups.length > 0);
    if (pups.length) {
      $('#pup-callout-text').textContent =
        t('uninstall.pup.warn', { reason: pups[0].pupReason }) +
        (pups.length > 1 ? `（共 ${pups.length} 个可疑软件，已置顶显示）` : '');
    }
  }
}

// 卸载按钮事件委托
$('#uninstall-list').addEventListener('click', async (ev) => {
  const btn = ev.target.closest('button[data-act]');
  if (!btn) return;
  const row = btn.closest('.row');
  const key = row.dataset.key;
  const app = uninstallApps.find((a) => a.key === key);
  if (!app) return;

  if (btn.dataset.act === 'force') {
    const ok = await confirmModal(
      t('uninstall.confirm', { name: app.name }) + '\n\n' +
      t('uninstall.pup.warn', { reason: app.pupReason }),
      t('uninstall.force'));
    if (!ok) return;
    toast(t('uninstall.uninstalling'));
    const res = await call('ForceUninstallApp', key);
    toast(res.success ? t('uninstall.done.toast', { name: app.name }) : res.message);
    renderUninstaller();
  } else {
    const ok = await confirmModal(
      t('uninstall.confirm', { name: app.name }),
      t('uninstall.uninstall'));
    if (!ok) return;
    toast(t('uninstall.uninstalling'));
    const res = await call('UninstallApp', key);
    if (res.success) {
      toast(t('uninstall.done.toast', { name: app.name }));
      if ((res.residues || []).length || (res.regResidue || []).length) {
        const note = (res.residues || []).concat(res.regResidue || []).slice(0, 6).join('\n');
        // 用确认框而非信息框：残留清理由用户决定，不打断刚完成的卸载
        const clean = await confirmModal(
          t('uninstall.done.detail', { name: app.name }) + '\n\n' + note,
          t('uninstall.done'));
        if (clean) {
          // 卸载成功后注册表键已被删，无法再用 key 定位软件，
          // 这里直接传卸载时扫描出的残留路径删除（不依赖注册表）。
          const fres = await call('CleanResidueApp', app.name, res.residues || [], res.regResidue || []);
          toast(fres.success ? (t('uninstall.residue.cleaned') || '残留已清理') : fres.message);
        }
      }
    } else {
      toast(res.message || t('uninstall.failed'));
      // 卸载未完成：引导用户确认是否强制卸载（深度清理残留）
      const ok = await confirmModal(
        t('uninstall.notdone', { name: app.name, msg: res.message }),
        t('uninstall.force'));
      if (ok) {
        const fres = await call('ForceUninstallApp', key);
        toast(fres.success ? t('uninstall.done.toast', { name: app.name }) : fres.message);
      }
    }
    renderUninstaller();
  }
});

$('#btn-uninstall-refresh').onclick = async () => {
  toast(t('scan.scanning'));
  await renderUninstaller();
};

// 插件 tab 加载器
TAB_LOADERS.uninstaller = () => { if (!uninstallApps.length) renderUninstaller(); };

$('#btn-ai').onclick = () => startScan('ai');
$('#btn-all').onclick = () => startScan('all');

initTheme();
initLang();          // 多语言（必须在渲染前应用一次 data-i18n）
initPlugins();       // 插件系统：注册侧边栏入口 + 面板
renderKpis();
renderChart();
refreshDirs();
initUpdate();
