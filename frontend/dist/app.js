const $ = (s) => document.querySelector(s);
const $$ = (s) => document.querySelectorAll(s);

let entries = [];          // 当前扫描结果（DTO 数组）
let regEntries = [];       // 当前注册表扫描结果
let selected = new Set();  // 选中项索引
let regSelected = new Set();

const riskClass = (r) => ({ '安全': 'ok', '谨慎': 'warn', '复核': 'risk' }[r] || '');
const fmtSize = (b) => {
  if (b < 1024) return b + ' B';
  const u = ['KB', 'MB', 'GB', 'TB'];
  let i = -1, n = b;
  do { n /= 1024; i++; } while (n >= 1024 && i < u.length - 1);
  return n.toFixed(1) + ' ' + u[i];
};

// 目录名/注册表项可包含引号与尖括号，直接拼 innerHTML 会破坏 DOM（属性注入）。
const esc = (s) => String(s === undefined || s === null ? '' : s)
  .replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;')
  .replace(/"/g, '&quot;').replace(/'/g, '&#39;');

// 调用后端绑定方法：window.go.main.App.<Method>
async function call(method, ...args) {
  return await window.go.main.App[method](...args);
}

function show(el, on) { el.classList.toggle('hidden', !on); }

function toast(msg) {
  const t = $('#toast');
  t.textContent = msg;
  show(t, true);
  clearTimeout(toast._t);
  toast._t = setTimeout(() => show(t, false), 2200);
}

// 自定义确认弹窗（避免依赖 webview 原生 confirm）
function confirmModal(text) {
  return new Promise((resolve) => {
    $('#modal-text').textContent = text;
    show($('#modal'), true);
    const done = (v) => {
      show($('#modal'), false);
      $('#modal-ok').onclick = null;
      $('#modal-cancel').onclick = null;
      resolve(v);
    };
    $('#modal-ok').onclick = () => done(true);
    $('#modal-cancel').onclick = () => done(false);
  });
}

// ── 扫描 ──
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
  show($('#results'), false);
  await call('Scan', mode); // 进度经 scan:progress 回传，结果经 scan:done 回传
}

// 实时进度条：扫描包按阶段(已知/自动发现/通用识别/自定义)回传 done/total，这里换算为整体百分比
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

// 扫描已在跑时后端会忽略重复请求，这里恢复按钮可用状态
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
  renderList();
  renderChart();
  $('#status-text').textContent = `扫描完成：共 ${entries.length} 项`;
  setTimeout(() => show($('#status'), false), 1400);
  show($('#results'), true);
  setScanning(false);
});

function renderList() {
  const list = $('#list');
  list.innerHTML = '';
  let total = 0;
  entries.forEach((e, i) => {
    total += e.size;
    const row = document.createElement('div');
    row.className = 'row';
    row.innerHTML =
      `<input type="checkbox" data-i="${i}" ${selected.has(i) ? 'checked' : ''}/>` +
      `<span class="size">${fmtSize(e.size)}</span>` +
      `<span class="badge ${riskClass(e.risk)}">${esc(e.risk)}</span>` +
      `<span class="cat" title="${esc(e.category)}">${esc(e.category)}</span>` +
      `<span class="path" title="${esc(e.path)}">${esc(e.shortPath)}</span>`;
    list.appendChild(row);
  });
  $('#stats').textContent = `共 ${entries.length} 项 · 可释放 ${fmtSize(total)}`;
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
  const ok = await confirmModal(`确认清理 ${paths.length} 项？将释放约 ${fmtSize(selectedSize())}。`);
  if (!ok) return;
  const res = await call('CleanSelected', paths);
  toast(`已清理 ${res.count} 项，释放 ${fmtSize(res.freed)}` +
    (res.failed && res.failed.length ? `（失败 ${res.failed.length} 项）` : ''));
  // 只移除真正清理成功的项，失败/被排除的项保留在列表中供重试
  const cleaned = new Set(res.cleaned || []);
  entries = entries.filter((e) => !cleaned.has(e.path));
  selected = new Set();
  renderList();
  renderChart();
};

function renderChart() {
  const map = {};
  entries.forEach((e) => { map[e.category] = (map[e.category] || 0) + e.size; });
  const arr = Object.entries(map).sort((a, b) => b[1] - a[1]).slice(0, 6);
  const max = arr.length ? arr[0][1] : 1;
  $('#chart').innerHTML = arr.map(([k, v]) =>
    `<div class="bar-row"><span class="bar-label" title="${esc(k)}">${esc(k)}</span>` +
    `<div class="bar-track"><div class="bar-fill" style="width:${Math.max(2, (v / max) * 100)}%"></div></div>` +
    `<span class="bar-val">${fmtSize(v)}</span></div>`).join('');
}

// ── 注册表垃圾清理 ──
async function scanRegistry() {
  regEntries = (await call('ScanRegistry')) || [];
  regSelected = new Set();
  renderRegList();
}

function renderRegList() {
  const list = $('#reg-list');
  list.innerHTML = '';
  let total = 0;
  regEntries.forEach((e, i) => {
    total += e.size;
    const row = document.createElement('div');
    row.className = 'row';
    row.innerHTML =
      `<input type="checkbox" data-i="${i}" ${regSelected.has(i) ? 'checked' : ''}/>` +
      `<span class="size">${fmtSize(e.size)}</span>` +
      `<span class="badge ${riskClass(e.risk)}">${esc(e.risk)}</span>` +
      `<span class="cat" title="${esc(e.category)}">${esc(e.category)}</span>` +
      `<span class="path" title="${esc(e.key)}">${esc(e.desc)}</span>`;
    list.appendChild(row);
  });
  $('#reg-stats').textContent = regEntries.length
    ? `共 ${regEntries.length} 项 · 约 ${fmtSize(total)}`
    : '未发现可清理的注册表项';
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
  const ok = await confirmModal(`确认清理 ${keys.length} 个注册表项？注册表操作不可恢复。`);
  if (!ok) return;
  const res = await call('CleanRegistry', keys);
  toast(`已清理 ${res.count} 个注册表项，释放约 ${fmtSize(res.freed)}` +
    (res.failed && res.failed.length ? `（失败 ${res.failed.length} 项）` : ''));
  const cleaned = new Set(res.cleaned || []);
  regEntries = regEntries.filter((e) => !cleaned.has(e.key));
  regSelected = new Set();
  renderRegList();
};

// ── Tabs ──
$$('.tab').forEach((t) => t.onclick = () => {
  $$('.tab').forEach((x) => x.classList.remove('active'));
  t.classList.add('active');
  const tab = t.dataset.tab;
  show($('#results'), tab === 'scan');
  show($('#tab-custom'), tab === 'custom');
  show($('#tab-history'), tab === 'history');
  show($('#tab-registry'), tab === 'registry');
  if (tab === 'custom') refreshDirs();
  if (tab === 'history') refreshHistory();
  if (tab === 'registry' && !regEntries.length) scanRegistry();
});

// ── 自定义 / 排除目录 ──
async function refreshDirs() {
  const custom = await call('GetCustomDirs');
  const exclude = await call('GetExcludeDirs');
  $('#custom-list').innerHTML = (custom && custom.length)
    ? custom.map((p) => `<li>${esc(p)} <button class="x" data-type="custom" data-p="${esc(p)}">✕</button></li>`).join('')
    : '<li class="empty">（空）</li>';
  $('#exclude-list').innerHTML = (exclude && exclude.length)
    ? exclude.map((p) => `<li>${esc(p)} <button class="x" data-type="exclude" data-p="${esc(p)}">✕</button></li>`).join('')
    : '<li class="empty">（空）</li>';
}
$('#custom-list').addEventListener('click', onRemove);
$('#exclude-list').addEventListener('click', onRemove);
async function onRemove(ev) {
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

// ── 历史 ──
async function refreshHistory() {
  const h = await call('GetHistory');
  $('#history-body').innerHTML = (h && h.length)
    ? h.map((r) => `<tr><td>${esc(r.time)}</td><td>${fmtSize(r.size)}</td><td title="${esc(r.path)}">${esc(r.path)}</td></tr>`).join('')
    : '<tr><td colspan="3" class="empty">（暂无记录）</td></tr>';
}

// ── 关于弹窗 ──
$('#btn-about').onclick = () => show($('#about'), true);
$('#about-close').onclick = () => show($('#about'), false);

// ── 入口 ──
$('#btn-ai').onclick = () => startScan('ai');
$('#btn-all').onclick = () => startScan('all');
refreshDirs();
