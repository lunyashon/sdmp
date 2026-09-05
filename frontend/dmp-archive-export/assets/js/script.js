const API = '';

const STAGE_OPTIONS = [
  ['1', 'принята'],
  ['2', 'обогащена'],
  ['3', 'в СК'],
  ['4', 'из СК'],
  ['5', 'в Б24'],
  ['6', 'отклонена'],
  ['7', 'ожидание'],
];

const FIELD_CATALOG = [
  { id: 'dateRange', label: 'Дата создания', group: 'Основные', type: 'daterange', pinned: true },
  { id: 'source', label: 'Источник', group: 'Основные', type: 'select', pinned: true, options: [] },
  { id: 'baza', label: 'Холодная база', group: 'Источник (JOIN)', type: 'select', pinned: true, options: [] },
  { id: 'region', label: 'Регион', group: 'lead_row', type: 'select', pinned: true, options: [] },
  { id: 'operator', label: 'Оператор связи', group: 'lead_row', type: 'select', pinned: true, options: [] },
  { id: 'phone', label: 'Телефон', group: 'Основные', type: 'text', pinned: true },
  { id: 'stage', label: 'Стадия', group: 'lead_row', type: 'select', options: STAGE_OPTIONS },
  { id: 'city', label: 'Город', group: 'lead_row', type: 'text' },
  { id: 'teg', label: 'Тег (вход)', group: 'lead_row', type: 'text' },
  { id: 'callResult', label: 'Результат звонка', group: 'lead_row', type: 'text' },
];
const DEFAULT_ACTIVE = FIELD_CATALOG.filter(f => f.pinned).map(f => f.id);
let activeFields = DEFAULT_ACTIVE.slice();

const COLUMN_CATALOG = [
  { id: 'date', label: 'Дата создания', get: r => r.date_create || '—' },
  { id: 'source', label: 'Источник', get: r => esc(r.source || '—') },
  { id: 'baza', label: 'Холодная база', get: r => esc(r.cold_label || '—') },
  { id: 'region', label: 'Регион', get: r => esc(r.region || '—') },
  { id: 'city', label: 'Город', get: r => esc(r.city || '—') },
  { id: 'operator', label: 'Оператор', get: r => esc(r.operator || '—') },
  { id: 'stage', label: 'Стадия', get: r => `<span class="stage"><span class="dot dot-${r.status_id || 0}"></span>${esc(r.stage || '—')}</span>` },
  { id: 'zarya', label: 'Проверка Зари', get: r => r.zarya ? `<span class="badge ${zaryaBadgeClass(r.zarya)}">${esc(r.zarya)}</span>` : '—' },
  { id: 'callResult', label: 'Результат звонка', get: r => r.call_result != null ? esc(String(r.call_result)) : '—' },
  { id: 'upk', label: 'ЮПК', get: r => esc(r.upk || '—') },
  { id: 'phone', label: 'Телефон', get: r => esc(r.phone || '—') },
  { id: 'teg', label: 'Тег (вход)', get: r => esc(r.tag || '—') },
  { id: 'webId', label: 'Web ID', get: r => esc(r.web_id || '—') },
  { id: 'projectNum', label: '№ проекта', get: r => esc(r.project_num || '—') },
];
const DEFAULT_COLUMNS = ['date', 'source', 'baza', 'region', 'operator', 'stage', 'zarya', 'upk', 'phone'];
let activeColumns = DEFAULT_COLUMNS.slice();

const CALL_FORM_LABELS = {
  imya: 'Имя',
  summa_dolga: 'Сумма долга',
  vid_dolga: 'Вид долга',
  dohod: 'Доход',
  zalog: 'Залог',
  imuschestvo: 'Имущество',
  bankrotstvo: 'Банкротство ранее',
  gorod: 'Город (из анкеты)',
};
const REGION_SOURCE = { 0: 'не определён', 1: 'от поставщика', 2: 'Росреестр', 3: 'оператор' };
const OPERATOR_SOURCE = { 0: 'не определён', 1: 'от поставщика', 2: 'по номеру' };
const EVENT_SOURCE = { 1: 'Worker', 2: 'Скорозвон', 3: 'API', 4: 'внешнее' };

const CARD_FIELD_CATALOG = [
  { id: 'stage', label: 'Стадия', get: r => esc(r.stage || '—') },
  { id: 'source', label: 'Источник', get: r => esc(r.source || '—') },
  { id: 'childSource', label: 'Под-источник', get: r => esc(r.child_source || '—') },
  { id: 'baza', label: 'Холодная база', get: r => esc(r.cold_label || '—') },
  { id: 'kakaya', label: 'Какая база', get: r => esc(r.base_label || '—') },
  { id: 'region', label: 'Регион', get: r => {
    const src = r.region_source == null ? '' : ` <span style="color:var(--text-muted); font-size:11px;">(${esc(REGION_SOURCE[r.region_source] || r.region_source)})</span>`;
    return esc(r.region || '—') + src;
  } },
  { id: 'city', label: 'Город', get: r => esc(r.city || '—') },
  { id: 'operator', label: 'Оператор связи', get: r => {
    const src = r.operator_source == null ? '' : ` <span style="color:var(--text-muted); font-size:11px;">(${esc(OPERATOR_SOURCE[r.operator_source] || r.operator_source)})</span>`;
    return esc(r.operator || '—') + src;
  } },
  { id: 'teg', label: 'Тег (вход)', get: r => esc(r.tag || '—') },
  { id: 'zarya', label: 'Проверка по Заре', get: r => r.zarya ? `<span class="badge ${zaryaBadgeClass(r.zarya)}">${esc(r.zarya)}</span>` : '—' },
  { id: 'callResult', label: 'Результат звонка', get: r => r.call_result != null ? esc(String(r.call_result)) : '—' },
  { id: 'upk', label: 'ЮПК', get: r => esc(r.upk || '—') },
  { id: 'webId', label: 'Web ID', get: r => esc(r.web_id || '—') },
  { id: 'projectNum', label: '№ проекта', get: r => esc(r.project_num || '—') },
  { id: 'uuid', label: 'UUID', get: r => esc(r.uuid || '—') },
  { id: 'phoneRaw', label: 'Телефон', get: r => esc(r.phone || '—') },
  { id: 'phoneValid', label: 'Телефон валиден', get: r => r.phone_valid ? 'да' : 'нет' },
  { id: 'syryeKey', label: 's3_key', get: r => esc(r.s3_key || '—') },
  { id: 'bitrixLead', label: 'Bitrix lead id', get: r => esc(r.bitrix_lead_id || '—') },
  { id: 'createdAt', label: 'Дата создания', get: r => esc(r.created_at || '—') },
  { id: 'updatedAt', label: 'Дата обновления', get: r => esc(r.updated_at || '—') },
];
let cardVisibleFieldIds = ['stage', 'source', 'region', 'operator', 'zarya', 'callResult', 'upk', 'uuid'];
let currentCardId = null;
let currentCard = null;
let dragCardFieldId = null;

let pageRows = [];
let total = 0;
let page = 1;
let pageSize = 15;
let selected = new Set();
let loading = false;
let savedFilters = [];
let dragColId = null;

function esc(s) {
  return String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}

function ymd(d) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
}

function defaultDateFrom() {
  const d = new Date();
  d.setDate(d.getDate() - 30);
  return ymd(d);
}

function defaultDateTo() {
  return ymd(new Date());
}

function toApiDate(value, end) {
  if (!value) return '';
  const [y, m, d] = value.split('-');
  return `${d}.${m}.${y} ${end ? '23:59:59' : '00:00:00'}`;
}

function fieldValueId(fieldId) { return 'f-' + fieldId; }

function setFieldOptions(id, options) {
  const f = FIELD_CATALOG.find(x => x.id === id);
  if (f) f.options = options;
}

async function loadCatalogs() {
  const [sourcesRes, regionsRes, operatorsRes] = await Promise.all([
    fetch(API + '/rest/sources').then(r => r.json()),
    fetch(API + '/rest/regions').then(r => r.json()),
    fetch(API + '/rest/operators').then(r => r.json()),
  ]);
  const sources = (sourcesRes.sources || []).filter(s =>
    s.cold_label || s.parent_id || s.bitrix_code === 'UC_L26K09' || s.bitrix_code === 'UC_VEAX7V'
  );
  sources.sort((a, b) => a.name.localeCompare(b.name, 'ru'));
  setFieldOptions('source', sources.map(s => [String(s.id), s.name]));
  const labels = [...new Set(sources.map(s => s.cold_label).filter(Boolean))].sort((a, b) => a.localeCompare(b, 'ru'));
  setFieldOptions('baza', labels);
  setFieldOptions('region', (regionsRes.items || []).map(x => [String(x.id), x.name]));
  setFieldOptions('operator', (operatorsRes.items || []).map(x => [String(x.id), x.name]));
}

function renderFiltersGrid() {
  const grid = document.getElementById('filters-grid');
  grid.innerHTML = '';
  activeFields.forEach(fid => {
    const f = FIELD_CATALOG.find(x => x.id === fid);
    if (!f) return;
    const wrap = document.createElement('div');
    wrap.className = 'field' + (f.pinned ? ' pinned' : '') + (f.type === 'daterange' ? ' span-2' : '');
    const removeBtn = f.pinned ? '' : `<span class="remove-x" onclick="removeField('${fid}')">&times;</span>`;
    if (f.type === 'daterange') {
      wrap.innerHTML = `<label>${f.label}${removeBtn}</label>
        <div class="field-dates">
          <input type="date" id="${fieldValueId(fid)}-from" value="${defaultDateFrom()}">
          <input type="date" id="${fieldValueId(fid)}-to" value="${defaultDateTo()}">
        </div>`;
    } else if (f.type === 'select') {
      const opts = (f.options || []).map(o => {
        const [val, txt] = Array.isArray(o) ? o : [o, o];
        return `<option value="${esc(val)}">${esc(txt)}</option>`;
      }).join('');
      wrap.innerHTML = `<label>${f.label}${removeBtn}</label><select id="${fieldValueId(fid)}"><option value="">Любое</option>${opts}</select>`;
    } else {
      wrap.innerHTML = `<label>${f.label}${removeBtn}</label><input type="text" id="${fieldValueId(fid)}" placeholder="${f.label}...">`;
    }
    grid.appendChild(wrap);
  });
}

function removeField(fid) { activeFields = activeFields.filter(x => x !== fid); renderFiltersGrid(); applyFilters(); }

function toggleFieldPicker(e) {
  e.stopPropagation();
  const picker = document.getElementById('field-picker');
  picker.classList.toggle('show');
  if (picker.classList.contains('show')) renderFieldPicker('');
}
document.addEventListener('click', (e) => {
  const picker = document.getElementById('field-picker');
  if (picker && !picker.contains(e.target) && !e.target.closest('.add-field-wrap')) picker.classList.remove('show');
});

function renderFieldPicker(query) {
  const list = document.getElementById('field-picker-list');
  const q = (query || '').toLowerCase();
  const groups = {};
  FIELD_CATALOG.forEach(f => {
    if (q && !f.label.toLowerCase().includes(q)) return;
    if (!groups[f.group]) groups[f.group] = [];
    groups[f.group].push(f);
  });
  let html = '';
  Object.keys(groups).forEach(g => {
    html += `<div class="field-picker-group">${g}</div>`;
    groups[g].forEach(f => {
      const isActive = activeFields.includes(f.id);
      html += `<div class="field-picker-item ${isActive ? 'disabled' : ''}" onclick="${isActive ? '' : `addField('${f.id}')`}">${f.label}${isActive ? ' \u2713' : ''}</div>`;
    });
  });
  list.innerHTML = html || '<div class="field-picker-item disabled">Ничего не найдено</div>';
}

function addField(fid) {
  if (!activeFields.includes(fid)) activeFields.push(fid);
  renderFiltersGrid();
  document.getElementById('field-picker').classList.remove('show');
}

function getFieldValue(fid) {
  const f = FIELD_CATALOG.find(x => x.id === fid);
  if (!f) return null;
  if (f.type === 'daterange') {
    const fromEl = document.getElementById(fieldValueId(fid) + '-from');
    const toEl = document.getElementById(fieldValueId(fid) + '-to');
    return { from: fromEl ? fromEl.value : '', to: toEl ? toEl.value : '' };
  }
  const el = document.getElementById(fieldValueId(fid));
  return el ? el.value : '';
}

function buildQuery() {
  const params = new URLSearchParams();
  const dates = getFieldValue('dateRange') || {};
  params.set('date_from', toApiDate(dates.from || defaultDateFrom(), false));
  params.set('date_to', toApiDate(dates.to || defaultDateTo(), true));
  const source = getFieldValue('source');
  if (source) params.append('sources', source);
  const baza = getFieldValue('baza');
  if (baza) params.append('cold_label', baza);
  const region = getFieldValue('region');
  if (region) params.append('region', region);
  const operator = getFieldValue('operator');
  if (operator) params.append('operator', operator);
  const phone = getFieldValue('phone');
  if (phone) params.set('phone', phone);
  const stage = getFieldValue('stage');
  if (stage) params.append('status', stage);
  const city = getFieldValue('city');
  if (city) params.set('city', city);
  const teg = getFieldValue('teg');
  if (teg) params.set('tag', teg);
  const callResult = getFieldValue('callResult');
  if (callResult) params.set('call_result', callResult);
  params.set('limit', String(pageSize));
  params.set('offset', String((page - 1) * pageSize));
  return params;
}

async function fetchLeads() {
  loading = true;
  document.getElementById('total-count').textContent = '…';
  try {
    const res = await fetch(API + '/rest/leads?' + buildQuery().toString());
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || res.statusText);
    pageRows = data.leads || [];
    total = data.total || 0;
    pageSize = data.limit || pageSize;
  } catch (err) {
    pageRows = [];
    total = 0;
    showToast('Не удалось загрузить строки: ' + err.message, true);
  } finally {
    loading = false;
    render();
  }
}

function applyFilters() {
  page = 1;
  selected.clear();
  fetchLeads();
}

function resetFilters() {
  activeFields = DEFAULT_ACTIVE.slice();
  renderFiltersGrid();
  applyFilters();
}

function renderSavedFilters() {
  const wrap = document.getElementById('saved-filters-chips');
  wrap.innerHTML = savedFilters.map((f, idx) => `
    <span class="filter-chip" onclick="applySavedFilter(${idx})">${esc(f.name)}<span class="chip-x" onclick="event.stopPropagation(); removeSavedFilter(${idx})">&times;</span></span>
  `).join('');
}
function saveCurrentFilter() {
  const name = prompt('Название фильтра:');
  if (!name) return;
  const values = {};
  activeFields.forEach(fid => { values[fid] = getFieldValue(fid); });
  savedFilters.push({ name, fields: activeFields.slice(), values });
  renderSavedFilters();
  showToast(`Фильтр «${name}» сохранён.`);
}
function applySavedFilter(idx) {
  const sf = savedFilters[idx];
  activeFields = Array.from(new Set([...DEFAULT_ACTIVE, ...sf.fields]));
  renderFiltersGrid();
  Object.keys(sf.values).forEach(fid => {
    const f = FIELD_CATALOG.find(x => x.id === fid);
    if (!f) return;
    if (f.type === 'daterange') {
      const v = sf.values[fid];
      if (v.from) document.getElementById(fieldValueId(fid) + '-from').value = v.from;
      if (v.to) document.getElementById(fieldValueId(fid) + '-to').value = v.to;
    } else {
      const el = document.getElementById(fieldValueId(fid));
      if (el) el.value = sf.values[fid];
    }
  });
  applyFilters();
}
function removeSavedFilter(idx) { savedFilters.splice(idx, 1); renderSavedFilters(); }

function changePageSize(v) { pageSize = parseInt(v, 10); page = 1; fetchLeads(); }
function gotoPage() {
  const v = parseInt(document.getElementById('goto-page').value, 10);
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  if (!v || v < 1 || v > totalPages) return;
  page = v;
  fetchLeads();
}

function positionPicker(picker, btnEl) {
  const rect = btnEl.getBoundingClientRect();
  const pw = 260;
  const estHeight = 360;
  let left = rect.right - pw;
  if (left < 8) left = 8;
  const spaceBelow = window.innerHeight - rect.bottom;
  if (spaceBelow < estHeight && rect.top > estHeight) {
    picker.style.top = 'auto';
    picker.style.bottom = (window.innerHeight - rect.top + 4) + 'px';
  } else {
    picker.style.bottom = 'auto';
    picker.style.top = (rect.bottom + 4) + 'px';
  }
  picker.style.left = left + 'px';
}
function toggleColPicker(e) {
  e.stopPropagation();
  const picker = document.getElementById('col-picker');
  picker.classList.toggle('show');
  if (picker.classList.contains('show')) { renderColPicker(); positionPicker(picker, e.target); }
}
document.addEventListener('click', (e) => {
  const picker = document.getElementById('col-picker');
  if (picker && !picker.contains(e.target) && !e.target.closest('.col-picker-wrap')) picker.classList.remove('show');
});

function renderColPicker() {
  const list = document.getElementById('col-picker-list');
  const ordered = [
    ...activeColumns.map(id => COLUMN_CATALOG.find(c => c.id === id)).filter(Boolean),
    ...COLUMN_CATALOG.filter(c => !activeColumns.includes(c.id)),
  ];
  list.innerHTML = ordered.map(c => `
    <div class="col-picker-item" draggable="true" data-col="${c.id}"
         ondragstart="dragColId='${c.id}'; this.classList.add('dragging')"
         ondragend="this.classList.remove('dragging')"
         ondragover="event.preventDefault()"
         ondrop="dropColumn(event, '${c.id}')">
      <span class="drag-handle">⠿</span>
      <input type="checkbox" id="colchk-${c.id}" ${activeColumns.includes(c.id) ? 'checked' : ''} onchange="toggleColumn('${c.id}', this.checked)">
      <label for="colchk-${c.id}">${c.label}</label>
    </div>
  `).join('');
}

function toggleColumn(id, checked) {
  if (checked && !activeColumns.includes(id)) activeColumns.push(id);
  if (!checked) activeColumns = activeColumns.filter(x => x !== id);
  render();
}

function dropColumn(e, targetId) {
  e.preventDefault();
  if (!dragColId || dragColId === targetId) return;
  if (!activeColumns.includes(dragColId) || !activeColumns.includes(targetId)) { renderColPicker(); return; }
  const from = activeColumns.indexOf(dragColId);
  const to = activeColumns.indexOf(targetId);
  activeColumns.splice(from, 1);
  activeColumns.splice(to, 0, dragColId);
  dragColId = null;
  renderColPicker();
  render();
}

function resetColumns() {
  activeColumns = DEFAULT_COLUMNS.slice();
  renderColPicker();
  render();
}

function zaryaBadgeClass(v) {
  if (v === 'Чистый') return 'badge-clean';
  if (v === 'Ошибка проверки') return 'badge-err';
  return 'badge-dup';
}

function render() {
  const tbody = document.getElementById('table-body');
  document.getElementById('total-count').textContent = total.toLocaleString('ru-RU');
  const totalPages = Math.max(1, Math.ceil(total / pageSize));
  if (page > totalPages) page = totalPages;
  const startIdx = total === 0 ? 0 : (page - 1) * pageSize;
  document.getElementById('empty-state').style.display = !loading && total === 0 ? 'block' : 'none';

  const headRow = document.getElementById('table-head-row');
  const headCells = activeColumns.map(id => {
    const c = COLUMN_CATALOG.find(x => x.id === id);
    return c ? `<th>${c.label}</th>` : '';
  }).join('');
  headRow.innerHTML = `<th class="chk"><input type="checkbox" id="chk-all" onchange="toggleAll(this)"></th><th>UUID</th>${headCells}`;

  tbody.innerHTML = '';
  pageRows.forEach(r => {
    const tr = document.createElement('tr');
    const cells = activeColumns.map(id => {
      const c = COLUMN_CATALOG.find(x => x.id === id);
      return c ? `<td>${c.get(r)}</td>` : '';
    }).join('');
    const uuid = r.uuid || ('#' + r.id);
    tr.innerHTML = `<td class="chk"><input type="checkbox" data-id="${r.id}" ${selected.has(r.id) ? 'checked' : ''} onchange="toggleRow(${r.id}, this)"></td><td><a href="javascript:void(0)" class="uuid-link" onclick="openCard(${r.id})">${esc(uuid)}</a></td>${cells}`;
    tbody.appendChild(tr);
  });
  document.getElementById('pager-info').textContent = total === 0
    ? (loading ? 'Загрузка…' : 'Нет данных')
    : `${startIdx + 1}\u2013${Math.min(startIdx + pageSize, total)} из ${total.toLocaleString('ru-RU')}`;
  const btnsWrap = document.getElementById('pager-buttons');
  btnsWrap.innerHTML = '';
  const mkBtn = (label, disabled, onclick, active) => { const b = document.createElement('button'); b.className = 'page-btn' + (active ? ' active' : ''); b.textContent = label; b.disabled = disabled; b.onclick = onclick; return b; };
  btnsWrap.appendChild(mkBtn('\u2039', page <= 1, () => { page--; fetchLeads(); }));
  const pagesToShow = [];
  for (let p = Math.max(1, page - 2); p <= Math.min(totalPages, page + 2); p++) pagesToShow.push(p);
  pagesToShow.forEach(p => btnsWrap.appendChild(mkBtn(String(p), false, () => { page = p; fetchLeads(); }, p === page)));
  btnsWrap.appendChild(mkBtn('\u203a', page >= totalPages, () => { page++; fetchLeads(); }));
  updateSelectedInfo();
}

function toggleRow(id, el) { if (el.checked) selected.add(id); else selected.delete(id); updateSelectedInfo(); }
function toggleAll(el) {
  pageRows.forEach(r => { if (el.checked) selected.add(r.id); else selected.delete(r.id); });
  render();
}
function updateSelectedInfo() {
  const info = document.getElementById('selected-info');
  document.getElementById('selected-count').textContent = selected.size;
  info.style.display = selected.size > 0 ? 'inline' : 'none';
}

function exportData(mode) {
  const count = mode === 'selected' ? selected.size : total;
  if (count === 0) { showToast('Нечего экспортировать — список пуст.', true); return; }
  showToast('Экспорт через API ещё не сделан. Сейчас можно смотреть таблицу и карточку строки.');
}
function showToast(msg, isError) {
  const t = document.getElementById('export-toast');
  t.textContent = msg;
  t.style.borderColor = isError ? '#e0b3a8' : '#c3d9c3';
  t.style.background = isError ? '#fbece8' : '#eaf1e8';
  t.style.color = isError ? '#a23b2e' : '#2f5233';
  t.classList.add('show');
}

function toggleCardFieldPicker(e) {
  e.stopPropagation();
  const picker = document.getElementById('card-field-picker');
  picker.classList.toggle('show');
  if (picker.classList.contains('show')) { renderCardFieldPicker(); positionPicker(picker, e.target); }
}
document.addEventListener('click', (e) => {
  const picker = document.getElementById('card-field-picker');
  if (picker && !picker.contains(e.target) && !e.target.closest('.col-picker-wrap')) picker.classList.remove('show');
});
function renderCardFieldPicker() {
  const list = document.getElementById('card-field-picker-list');
  const ordered = [
    ...cardVisibleFieldIds.map(id => CARD_FIELD_CATALOG.find(f => f.id === id)).filter(Boolean),
    ...CARD_FIELD_CATALOG.filter(f => !cardVisibleFieldIds.includes(f.id)),
  ];
  list.innerHTML = ordered.map(f => `
    <div class="col-picker-item" draggable="true" data-fld="${f.id}"
         ondragstart="dragCardFieldId='${f.id}'; this.classList.add('dragging')"
         ondragend="this.classList.remove('dragging')"
         ondragover="event.preventDefault()"
         ondrop="dropCardField(event, '${f.id}')">
      <span class="drag-handle">⠿</span>
      <input type="checkbox" id="cardfld-${f.id}" ${cardVisibleFieldIds.includes(f.id) ? 'checked' : ''} onchange="toggleCardField('${f.id}', this.checked)">
      <label for="cardfld-${f.id}">${f.label}</label>
    </div>`).join('');
}
function toggleCardField(id, checked) {
  if (checked && !cardVisibleFieldIds.includes(id)) cardVisibleFieldIds.push(id);
  if (!checked) cardVisibleFieldIds = cardVisibleFieldIds.filter(x => x !== id);
  renderCardFieldPicker();
}
function dropCardField(e, targetId) {
  e.preventDefault();
  if (!dragCardFieldId || dragCardFieldId === targetId) return;
  if (!cardVisibleFieldIds.includes(dragCardFieldId) || !cardVisibleFieldIds.includes(targetId)) { renderCardFieldPicker(); return; }
  const from = cardVisibleFieldIds.indexOf(dragCardFieldId);
  const to = cardVisibleFieldIds.indexOf(targetId);
  cardVisibleFieldIds.splice(from, 1);
  cardVisibleFieldIds.splice(to, 0, dragCardFieldId);
  dragCardFieldId = null;
  renderCardFieldPicker();
}
function resetCardFields() {
  cardVisibleFieldIds = ['stage', 'source', 'region', 'operator', 'zarya', 'callResult', 'upk', 'uuid'];
  renderCardFieldPicker();
}

function kvHtml(entries) {
  if (!entries.length) {
    return '<div style="grid-column:1/-1; color:var(--text-muted); font-size:13px;">Нет данных.</div>';
  }
  return entries.map(([k, v]) => `<div class="kv"><span class="k">${esc(k)}</span><span class="v">${v}</span></div>`).join('');
}

function formatValue(v) {
  if (v == null || v === '') return '—';
  if (typeof v === 'object') return esc(JSON.stringify(v));
  return esc(String(v));
}

function objectEntries(value) {
  if (!value) return [];
  if (Array.isArray(value)) {
    return value.map((item, i) => [String(i + 1), formatValue(item)]);
  }
  if (typeof value === 'object') {
    return Object.keys(value).map(k => [k, formatValue(value[k])]);
  }
  return [['значение', formatValue(value)]];
}

function renderCard(r) {
  currentCard = r;
  currentCardId = r.id;
  document.getElementById('card-phone').textContent = r.phone || '—';
  document.getElementById('card-sub').textContent = `${r.source || '—'} · создана ${r.created_at || '—'} · ${r.uuid || ('id ' + r.id)}`;

  const visibleFields = cardVisibleFieldIds.map(id => CARD_FIELD_CATALOG.find(f => f.id === id)).filter(Boolean);
  document.getElementById('card-main').innerHTML = visibleFields.map(f => `<div class="kv"><span class="k">${f.label}</span><span class="v">${f.get(r)}</span></div>`).join('')
    || '<div style="grid-column:1/-1; color:var(--text-muted); font-size:13px;">Все поля скрыты — настройте через ⚙️ Поля.</div>';

  const form = r.call_form;
  if (form && typeof form === 'object' && !Array.isArray(form)) {
    const entries = Object.keys(form).map(k => [CALL_FORM_LABELS[k] || k, formatValue(form[k])]);
    document.getElementById('card-anketa').innerHTML = kvHtml(entries);
  } else if (form) {
    document.getElementById('card-anketa').innerHTML = kvHtml([['call_form', formatValue(form)]]);
  } else {
    document.getElementById('card-anketa').innerHTML = '<div style="grid-column:1/-1; color:var(--text-muted); font-size:13px;">Анкета появится после исхода звонка от Скорозвона.</div>';
  }

  document.getElementById('card-legacy').innerHTML = kvHtml(objectEntries(r.legacy_fields));

  const recSection = document.getElementById('card-recording-section');
  if (r.call_url) {
    recSection.style.display = '';
    const label = r.call_id ? `Прослушать запись (call_id: ${esc(r.call_id)})` : 'Прослушать запись';
    document.getElementById('card-recording-link').innerHTML = `<a class="raw-link" href="${esc(r.call_url)}" target="_blank" rel="noopener">${label} →</a>`;
  } else {
    recSection.style.display = 'none';
  }

  const history = r.phone_history || [];
  const histSection = document.getElementById('card-history-section');
  if (history.length) {
    histSection.style.display = '';
    document.getElementById('card-history').innerHTML = history.map(x => `
      <div class="dispatch-row" style="cursor:pointer;" onclick="openCard(${x.id})">
        <span>${esc(x.date_create || '—')} · ${esc(x.source || '—')} · <span class="uuid-link" style="pointer-events:none;">${esc((x.uuid || '').slice(0, 8))}…</span></span>
        <span class="stage"><span class="dot dot-${x.status_id || 0}"></span>${esc(x.stage || '—')}</span>
      </div>`).join('');
  } else {
    histSection.style.display = 'none';
  }

  const items = [];
  (r.events || []).forEach(e => {
    items.push({
      at: e.occurred_at,
      type: 'event',
      label: e.type || 'событие',
      detail: [EVENT_SOURCE[e.source] || '', e.details || ''].filter(Boolean).join(' · '),
      ok: true,
    });
  });
  (r.dispatches || []).forEach(d => {
    const who = d.channel === 'вебхук-партнёр' && d.upk ? `покупатель ${d.upk}` : d.channel;
    const extra = [who, d.project_id, d.external_id ? 'ext ' + d.external_id : ''].filter(Boolean).join(' · ');
    items.push({
      at: d.sent_at,
      type: 'dispatch',
      label: 'Отправка: ' + (d.channel || 'канал'),
      detail: extra,
      ok: d.sent_ok,
    });
  });
  items.sort((a, b) => String(a.at).localeCompare(String(b.at)));
  document.getElementById('card-timeline').innerHTML = items.length
    ? items.map(e => `<div class="tl-item ${e.type === 'dispatch' ? 'tl-dispatch' : ''}">
        <div class="tl-label">${esc(e.label)}<span class="tl-badge ${e.ok ? 'ok' : 'fail'}">${e.ok ? 'ok' : 'ошибка'}</span></div>
        ${e.detail ? `<div class="tl-detail">${esc(e.detail)}</div>` : ''}
        <div class="tl-time">${esc(e.at || '')}</div>
      </div>`).join('')
    : '<div style="color:var(--text-muted); font-size:13px;">Событий пока нет — Worker ещё не писал в event/dispatch.</div>';

  const rawSection = document.querySelector('#card-overlay .card-section:last-child');
  if (rawSection) {
    if (r.s3_key) {
      rawSection.style.display = '';
      rawSection.innerHTML = `<h3>Сырьё</h3><span class="raw-link">${esc(r.s3_key)}</span>`;
    } else {
      rawSection.innerHTML = '<h3>Сырьё</h3><div style="color:var(--text-muted); font-size:13px;">s3_key пустой — у исторических строк сырьё может быть в legacy_fields.</div>';
    }
  }

  document.getElementById('card-overlay').classList.add('show');
}

async function openCard(id) {
  currentCardId = id;
  document.getElementById('card-phone').textContent = 'Загрузка…';
  document.getElementById('card-sub').textContent = 'GET /rest/leads/' + id;
  document.getElementById('card-overlay').classList.add('show');
  try {
    const res = await fetch(API + '/rest/leads/' + id);
    const data = await res.json();
    if (!res.ok) throw new Error(data.error || res.statusText);
    renderCard(data.lead);
  } catch (err) {
    document.getElementById('card-phone').textContent = 'Ошибка';
    document.getElementById('card-sub').textContent = err.message;
    showToast('Не удалось открыть строку: ' + err.message, true);
  }
}
function closeCard() { document.getElementById('card-overlay').classList.remove('show'); }

(async function init() {
  renderFiltersGrid();
  render();
  renderSavedFilters();
  try {
    await loadCatalogs();
    renderFiltersGrid();
  } catch (err) {
    showToast('Не загрузились справочники: ' + err.message, true);
  }
  await fetchLeads();
})();
