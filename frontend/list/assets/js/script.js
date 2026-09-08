const PUBLIC_ORIGIN = 'https://cdp-dmp.ru';
const API = PUBLIC_ORIGIN;

function apiData(body) {
    if (body && typeof body === 'object' && 'data' in body && 'success' in body) {
        return body.data;
    }
    return body;
}

function apiError(body, fallback) {
    if (body && body.data && body.data.error) return body.data.error;
    if (body && body.error) return body.error;
    return fallback;
}

const STAGE_OPTIONS = [
    ['1', 'принята'],
    ['2', 'обогащена'],
    ['3', 'в СК'],
    ['4', 'из СК'],
    ['5', 'в Б24'],
    ['6', 'отклонена'],
    ['7', 'ожидание'],
];

const ZARYA_OPTIONS = [
    ['Чистый', 'Чистый'],
    ['Дубль', 'Дубль'],
    ['Ошибка проверки', 'Ошибка проверки'],
];

const FIELD_CATALOG = [
    { id: 'dateRange', label: 'Дата создания', group: 'Основные', type: 'daterange', pinned: true },
    { id: 'source', label: 'Источник', group: 'Основные', type: 'multiselect', pinned: true, options: [] },
    { id: 'childSource', label: 'Подисточник', group: 'Основные', type: 'multiselect', pinned: true, options: [] },
    { id: 'baza', label: 'Холодная база', group: 'Источник (JOIN)', type: 'text', pinned: true },
    { id: 'region', label: 'Регион', group: 'lead_row', type: 'multiselect', pinned: true, options: [] },
    { id: 'operator', label: 'Оператор связи', group: 'lead_row', type: 'multiselect', pinned: true, options: [] },
    { id: 'zarya', label: 'Проверка Зари', group: 'Основные', type: 'multiselect', pinned: true, options: ZARYA_OPTIONS },
    { id: 'phone', label: 'Телефон', group: 'Основные', type: 'text', pinned: true },
    { id: 'stage', label: 'Стадия', group: 'lead_row', type: 'multiselect', pinned: true, options: STAGE_OPTIONS },
    { id: 'viaUs', label: 'Из скорозвона', group: 'lead_row', type: 'select', pinned: true, options: [['false', 'да'], ['true', 'нет']] },
    { id: 'city', label: 'Город', group: 'lead_row', type: 'text' },
    { id: 'teg', label: 'Тег (вход)', group: 'lead_row', type: 'text' },
    { id: 'callResult', label: 'Результат звонка', group: 'lead_row', type: 'text' },
];
const DEFAULT_ACTIVE = FIELD_CATALOG.filter(f => f.pinned).map(f => f.id);
let activeFields = DEFAULT_ACTIVE.slice();
const multiState = {};

const COLUMN_CATALOG = [
    { id: 'date', label: 'Дата создания', get: r => r.date_create || '—' },
    { id: 'source', label: 'Источник', get: r => esc(r.source || '—') },
    { id: 'childSource', label: 'Подисточник', get: r => esc(r.child_source || '—') },
    { id: 'baza', label: 'Холодная база', get: r => esc(r.cold_label || '—') },
    { id: 'kakaya', label: 'Какая база', get: r => esc(r.base_label || '—') },
    { id: 'sourceDesc', label: 'Source description', get: r => esc(r.source_description || '—') },
    { id: 'region', label: 'Регион', get: r => esc(r.region || '—') },
    { id: 'city', label: 'Город', get: r => esc(r.city || '—') },
    { id: 'operator', label: 'Оператор', get: r => esc(r.operator || '—') },
    { id: 'stage', label: 'Стадия', get: r => `<span class="stage"><span class="dot dot-${r.status_id || 0}"></span>${esc(r.stage || '—')}</span>` },
    { id: 'viaUs', label: 'Из скорозвона', get: r => r.via_us === false ? 'да' : 'нет' },
    { id: 'zarya', label: 'Проверка Зари', get: r => r.zarya ? `<span class="badge ${zaryaBadgeClass(r.zarya)}">${esc(r.zarya)}</span>` : '—' },
    { id: 'callResult', label: 'Результат звонка', get: r => esc(r.call_result_name || (r.call_result != null ? String(r.call_result) : '') || '—') },
    { id: 'upk', label: 'ЮПК', get: r => esc(r.upk || '—') },
    { id: 'phone', label: 'Телефон', get: r => esc(r.phone || '—') },
    { id: 'teg', label: 'Тег (вход)', get: r => esc(r.tag || '—') },
    { id: 'webId', label: 'Web ID', get: r => esc(r.web_id || '—') },
    { id: 'projectNum', label: '№ проекта', get: r => esc(r.project_num || '—') },
];
const DEFAULT_COLUMNS = ['date', 'source', 'childSource', 'baza', 'kakaya', 'sourceDesc', 'region', 'operator', 'stage', 'viaUs', 'zarya', 'upk', 'phone'];
const COL_PREFS_KEY = 'sdmp.list.colprefs';
const CARD_PREFS_KEY = 'sdmp.list.cardfields';
const MIN_COL_WIDTH = 64;
const DEFAULT_COL_WIDTHS = {
    chk: 36, uuid: 280, date: 150, source: 140, childSource: 160, baza: 140, kakaya: 140,
    sourceDesc: 180, region: 200, city: 140, operator: 120, stage: 120, viaUs: 130, zarya: 140,
    callResult: 130, upk: 110, phone: 130, teg: 120, webId: 120, projectNum: 110,
};
let activeColumns = DEFAULT_COLUMNS.slice();
let columnWidths = {};
let colMove = null;

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
    { id: 'viaUs', label: 'Из скорозвона', get: r => r.via_us === false ? 'да' : 'нет' },
    { id: 'source', label: 'Источник', get: r => esc(r.source || '—') },
    { id: 'childSource', label: 'Подисточник', get: r => esc(r.child_source || '—') },
    { id: 'baza', label: 'Холодная база', get: r => esc(r.cold_label || '—') },
    { id: 'kakaya', label: 'Какая база', get: r => esc(r.base_label || '—') },
    { id: 'sourceDesc', label: 'Source description', get: r => esc(r.source_description || '—') },
    {
        id: 'region', label: 'Регион', get: r => {
            const src = r.region_source == null ? '' : ` <span style="color:var(--text-muted); font-size:11px;">(${esc(REGION_SOURCE[r.region_source] || r.region_source)})</span>`;
            return esc(r.region || '—') + src;
        }
    },
    { id: 'city', label: 'Город', get: r => esc(r.city || '—') },
    {
        id: 'operator', label: 'Оператор связи', get: r => {
            const src = r.operator_source == null ? '' : ` <span style="color:var(--text-muted); font-size:11px;">(${esc(OPERATOR_SOURCE[r.operator_source] || r.operator_source)})</span>`;
            return esc(r.operator || '—') + src;
        }
    },
    { id: 'teg', label: 'Тег (вход)', get: r => esc(r.tag || '—') },
    { id: 'zarya', label: 'Проверка по Заре', get: r => r.zarya ? `<span class="badge ${zaryaBadgeClass(r.zarya)}">${esc(r.zarya)}</span>` : '—' },
    { id: 'callResult', label: 'Результат звонка', get: r => esc(r.call_result_name || (r.call_result != null ? String(r.call_result) : '') || '—') },
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
const DEFAULT_CARD_FIELDS = ['stage', 'viaUs', 'source', 'childSource', 'baza', 'kakaya', 'sourceDesc', 'region', 'operator', 'zarya', 'callResult', 'upk', 'uuid'];
let cardVisibleFieldIds = DEFAULT_CARD_FIELDS.slice();
let currentCardId = null;
let currentCard = null;
let dragCardFieldId = null;

const DEFAULT_PAGE_SIZE = 15;
let pageRows = [];
let total = 0;
let page = 1;
let pageSize = DEFAULT_PAGE_SIZE;
let syncingURL = false;
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
    const [sourcesRes, regionsRes, operatorsRes, statusesRes] = await Promise.all([
        fetch(API + '/rest/sources').then(r => r.json()).then(apiData),
        fetch(API + '/rest/regions').then(r => r.json()).then(apiData),
        fetch(API + '/rest/operators').then(r => r.json()).then(apiData),
        fetch(API + '/rest/statuses').then(r => r.json()).then(apiData),
    ]);
    const allSources = sourcesRes.sources || [];
    const parentById = {};
    allSources.forEach(s => { parentById[s.id] = s; });
    const parents = allSources.filter(s => !s.parent_id);
    parents.sort((a, b) => a.name.localeCompare(b.name, 'ru'));
    const children = allSources.filter(s => s.parent_id);
    children.sort((a, b) => {
        const pa = (parentById[a.parent_id] || {}).name || '';
        const pb = (parentById[b.parent_id] || {}).name || '';
        return pa.localeCompare(pb, 'ru') || a.name.localeCompare(b.name, 'ru');
    });
    setFieldOptions('source', parents.map(s => [String(s.id), s.name]));
    setFieldOptions('childSource', children.map(s => {
        const p = parentById[s.parent_id];
        return [String(s.id), p ? (p.name + ' → ' + s.name) : s.name];
    }));
    setFieldOptions('region', (regionsRes.items || []).map(x => [String(x.id), x.name]));
    setFieldOptions('operator', (operatorsRes.items || []).map(x => [String(x.id), x.name]));
    const statuses = (statusesRes.items || []).map(x => [String(x.id), x.name]);
    if (statuses.length) setFieldOptions('stage', statuses);
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
        } else if (f.type === 'multiselect') {
            wrap.innerHTML = `<label>${f.label}${removeBtn}</label>${msHtml(f)}`;
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
    activeFields.forEach(fid => {
        const f = FIELD_CATALOG.find(x => x.id === fid);
        if (f && f.type === 'multiselect') updateMsTrigger(fid);
    });
}

function msSelected(fid) {
    return Array.isArray(multiState[fid]) ? multiState[fid] : [];
}

function msHtml(f) {
    const selected = new Set(msSelected(f.id));
    const opts = (f.options || []).map(o => {
        const [val, txt] = Array.isArray(o) ? o : [o, o];
        const checked = selected.has(String(val)) ? 'checked' : '';
        return `<label class="ms-opt" data-text="${esc(String(txt).toLowerCase())}">
      <span>${esc(txt)}</span>
      <input type="checkbox" value="${esc(val)}" ${checked} onchange="toggleMsOpt('${f.id}', this.value, this.checked)">
    </label>`;
    }).join('');
    return `<div class="ms" id="${fieldValueId(f.id)}">
    <button type="button" class="ms-trigger" onclick="toggleMs('${f.id}', event)"><span class="ms-label">Любое</span></button>
    <span class="ms-caret">▾</span>
    <div class="ms-drop" onclick="event.stopPropagation()">
      <input type="text" class="ms-search" placeholder="Поиск..." oninput="filterMs('${f.id}', this.value)">
      <div class="ms-opts">${opts || '<div class="ms-empty">Нет вариантов</div>'}</div>
      <div class="ms-foot"><button type="button" class="btn btn-ghost" onclick="clearMs('${f.id}')">Сбросить</button></div>
    </div>
  </div>`;
}

function toggleMs(fid, e) {
    e.stopPropagation();
    const el = document.getElementById(fieldValueId(fid));
    if (!el) return;
    const open = el.classList.contains('open');
    document.querySelectorAll('.ms.open').forEach(x => x.classList.remove('open'));
    if (!open) el.classList.add('open');
}

document.addEventListener('click', () => {
    document.querySelectorAll('.ms.open').forEach(x => x.classList.remove('open'));
});

function filterMs(fid, query) {
    const el = document.getElementById(fieldValueId(fid));
    if (!el) return;
    const q = (query || '').toLowerCase();
    el.querySelectorAll('.ms-opt').forEach(opt => {
        opt.classList.toggle('hidden', Boolean(q) && !(opt.dataset.text || '').includes(q));
    });
}

function toggleMsOpt(fid, value, checked) {
    const cur = msSelected(fid).slice();
    const i = cur.indexOf(value);
    if (checked && i < 0) cur.push(value);
    if (!checked && i >= 0) cur.splice(i, 1);
    multiState[fid] = cur;
    updateMsTrigger(fid);
}

function clearMs(fid) {
    multiState[fid] = [];
    const el = document.getElementById(fieldValueId(fid));
    if (el) el.querySelectorAll('input[type="checkbox"]').forEach(c => { c.checked = false; });
    updateMsTrigger(fid);
}

function updateMsTrigger(fid) {
    const el = document.getElementById(fieldValueId(fid));
    if (!el) return;
    const f = FIELD_CATALOG.find(x => x.id === fid);
    const selected = msSelected(fid);
    const label = el.querySelector('.ms-label');
    if (!label) return;
    if (!selected.length) {
        label.textContent = 'Любое';
        return;
    }
    if (selected.length === 1 && f) {
        const opt = (f.options || []).find(o => {
            const [val] = Array.isArray(o) ? o : [o, o];
            return String(val) === selected[0];
        });
        label.textContent = opt ? (Array.isArray(opt) ? opt[1] : opt) : selected[0];
        return;
    }
    label.textContent = selected.length + ' выбрано';
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
    if (f.type === 'multiselect') return msSelected(fid);
    const el = document.getElementById(fieldValueId(fid));
    return el ? el.value : '';
}

function appendMulti(params, key, values) {
    (Array.isArray(values) ? values : (values ? [values] : [])).forEach(v => {
        if (v !== '' && v != null) params.append(key, v);
    });
}

function buildQuery() {
    const params = new URLSearchParams();
    const dates = getFieldValue('dateRange') || {};
    params.set('date_from', toApiDate(dates.from || defaultDateFrom(), false));
    params.set('date_to', toApiDate(dates.to || defaultDateTo(), true));
    appendMulti(params, 'sources', getFieldValue('source'));
    appendMulti(params, 'child_sources', getFieldValue('childSource'));
    const baza = getFieldValue('baza');
    if (baza) params.append('cold_label', baza);
    appendMulti(params, 'region', getFieldValue('region'));
    appendMulti(params, 'operator', getFieldValue('operator'));
    appendMulti(params, 'zarya', getFieldValue('zarya'));
    const phone = getFieldValue('phone');
    if (phone) params.set('phone', phone);
    appendMulti(params, 'status', getFieldValue('stage'));
    const city = getFieldValue('city');
    if (city) params.set('city', city);
    const teg = getFieldValue('teg');
    if (teg) params.set('tag', teg);
    const callResult = getFieldValue('callResult');
    if (callResult) params.set('call_result', callResult);
    const viaUs = getFieldValue('viaUs');
    if (viaUs === 'true' || viaUs === 'false') params.set('via_us', viaUs);
    params.set('limit', String(pageSize));
    params.set('offset', String((page - 1) * pageSize));
    return params;
}

async function fetchLeads() {
    loading = true;
    document.getElementById('total-count').textContent = '…';
    try {
        const res = await fetch(API + '/rest/leads?' + buildQuery().toString());
        const body = await res.json();
        if (!res.ok || body.success === false) throw new Error(apiError(body, res.statusText));
        const data = apiData(body);
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
        writeListURL();
    }
}

function applyFilters() {
    page = 1;
    selected.clear();
    fetchLeads();
}

function resetFilters() {
    activeFields = DEFAULT_ACTIVE.slice();
    Object.keys(multiState).forEach(k => { delete multiState[k]; });
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
            if (v && v.from) document.getElementById(fieldValueId(fid) + '-from').value = v.from;
            if (v && v.to) document.getElementById(fieldValueId(fid) + '-to').value = v.to;
        } else if (f.type === 'multiselect') {
            const v = sf.values[fid];
            multiState[fid] = Array.isArray(v) ? v.slice() : (v ? [v] : []);
            const el = document.getElementById(fieldValueId(fid));
            if (el) {
                const sel = new Set(multiState[fid]);
                el.querySelectorAll('input[type="checkbox"]').forEach(c => { c.checked = sel.has(c.value); });
                updateMsTrigger(fid);
            }
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
    saveColumnPrefs();
    render();
}

function moveColumn(srcId, targetId, after) {
    if (!srcId || srcId === targetId) return false;
    if (!activeColumns.includes(srcId) || !activeColumns.includes(targetId)) return false;
    const from = activeColumns.indexOf(srcId);
    let to = activeColumns.indexOf(targetId);
    activeColumns.splice(from, 1);
    if (from < to) to -= 1;
    if (after) to += 1;
    activeColumns.splice(to, 0, srcId);
    return true;
}

function dropColumn(e, targetId) {
    e.preventDefault();
    if (!moveColumn(dragColId, targetId, false)) { renderColPicker(); return; }
    dragColId = null;
    saveColumnPrefs();
    renderColPicker();
    render();
}

function resetColumns() {
    activeColumns = DEFAULT_COLUMNS.slice();
    columnWidths = {};
    saveColumnPrefs();
    renderColPicker();
    render();
}

function colWidth(id) {
    const w = Number(columnWidths[id]);
    if (w >= MIN_COL_WIDTH) return Math.round(w);
    return DEFAULT_COL_WIDTHS[id] || 140;
}

function loadColumnPrefs() {
    try {
        const raw = JSON.parse(localStorage.getItem(COL_PREFS_KEY) || '{}');
        const known = new Set(COLUMN_CATALOG.map(c => c.id));
        if (Array.isArray(raw.visible)) {
            const next = raw.visible.filter(id => known.has(id));
            if (next.length) {
                activeColumns = next;
                if (!activeColumns.includes('viaUs')) {
                    const after = activeColumns.indexOf('stage');
                    if (after >= 0) activeColumns.splice(after + 1, 0, 'viaUs');
                    else activeColumns.push('viaUs');
                }
            }
        }
        if (raw.widths && typeof raw.widths === 'object') {
            Object.keys(raw.widths).forEach(id => {
                const w = Number(raw.widths[id]);
                if (w >= MIN_COL_WIDTH) columnWidths[id] = w;
            });
        }
    } catch (err) { /* ignore broken prefs */ }
}

function saveColumnPrefs() {
    try {
        localStorage.setItem(COL_PREFS_KEY, JSON.stringify({ visible: activeColumns, widths: columnWidths }));
    } catch (err) { /* quota / private mode */ }
}

function applyColumnWidths() {
    const group = document.getElementById('table-cols');
    if (!group) return;
    const ids = ['chk', 'uuid', ...activeColumns];
    group.innerHTML = ids.map(id => `<col data-col="${id}" style="width:${colWidth(id)}px">`).join('');
}

function setColWidth(id, width) {
    const w = Math.max(MIN_COL_WIDTH, Math.round(width));
    columnWidths[id] = w;
    const col = document.querySelector(`#table-cols col[data-col="${id}"]`);
    if (col) col.style.width = w + 'px';
}

function headCells() {
    return [...document.getElementById('table-head-row').children];
}

function colIndexById(id) {
    return headCells().findIndex(th => th.getAttribute('data-col') === id);
}

function snapshotColLefts() {
    const map = {};
    headCells().forEach((th, i) => {
        const key = th.getAttribute('data-col') || ('#' + i);
        map[key] = th.getBoundingClientRect().left;
    });
    return map;
}

function playColFlip(first) {
    const heads = headCells();
    const rows = [...document.querySelectorAll('#table-body tr')];
    const all = [];
    heads.forEach((th, i) => {
        all.push(th);
        rows.forEach(row => {
            if (row.children[i]) all.push(row.children[i]);
        });
    });
    all.forEach(el => {
        el.style.transition = 'none';
        el.style.transform = '';
    });
    void document.getElementById('table-head-row').offsetWidth;
    const moving = [];
    heads.forEach((th, i) => {
        const key = th.getAttribute('data-col') || ('#' + i);
        if (first[key] == null) return;
        const dx = first[key] - th.getBoundingClientRect().left;
        if (Math.abs(dx) < 0.5) return;
        const els = [th];
        rows.forEach(row => {
            if (row.children[i]) els.push(row.children[i]);
        });
        els.forEach(el => {
            el.style.transform = 'translateX(' + dx + 'px)';
        });
        moving.push.apply(moving, els);
    });
    if (!moving.length) return;
    requestAnimationFrame(() => {
        moving.forEach(el => {
            el.style.transition = 'transform .22s ease';
            el.style.transform = '';
        });
    });
    clearTimeout(playColFlip._tid);
    playColFlip._tid = setTimeout(() => {
        moving.forEach(el => { el.style.transition = ''; });
    }, 260);
}

function moveTableColumn(fromIdx, toIdx) {
    if (fromIdx === toIdx || fromIdx < 0 || toIdx < 0) return;
    const first = snapshotColLefts();
    const shift = (parent) => {
        if (!parent || fromIdx >= parent.children.length || toIdx >= parent.children.length) return;
        const el = parent.children[fromIdx];
        parent.removeChild(el);
        if (toIdx >= parent.children.length) parent.appendChild(el);
        else parent.insertBefore(el, parent.children[toIdx]);
    };
    shift(document.getElementById('table-cols'));
    shift(document.getElementById('table-head-row'));
    document.querySelectorAll('#table-body tr').forEach(shift);
    playColFlip(first);
}

function syncOrderFromDom() {
    const ids = headCells()
        .map(th => th.getAttribute('data-col'))
        .filter(id => id && id !== 'chk' && id !== 'uuid');
    if (ids.length) activeColumns = ids;
}

function markOriginColumn(id, on) {
    document.querySelectorAll(`#table-head-row th[data-col="${id}"], #table-body td[data-col="${id}"]`).forEach(el => {
        el.classList.toggle('col-origin', on);
    });
}

function buildColGhost(id) {
    const idx = colIndexById(id);
    const th = headCells()[idx];
    if (!th) return null;
    const width = th.getBoundingClientRect().width;
    const table = document.querySelector('.table-scroll table');
    const tableRect = table.getBoundingClientRect();
    const ghost = document.createElement('div');
    ghost.className = 'col-ghost';
    ghost.style.width = width + 'px';
    ghost.style.height = tableRect.height + 'px';
    const inner = document.createElement('table');
    inner.innerHTML = `<colgroup><col style="width:${width}px"></colgroup><thead><tr></tr></thead><tbody></tbody>`;
    const headClone = th.cloneNode(true);
    inner.querySelector('thead tr').appendChild(headClone);
    const body = inner.querySelector('tbody');
    document.querySelectorAll('#table-body tr').forEach(row => {
        const cell = row.children[idx];
        if (!cell) return;
        const tr = document.createElement('tr');
        tr.appendChild(cell.cloneNode(true));
        body.appendChild(tr);
    });
    ghost.appendChild(inner);
    document.body.appendChild(ghost);
    return ghost;
}

function placeColGhost(ghost, clientX) {
    if (!ghost || !colMove) return;
    const scroll = document.querySelector('.table-scroll');
    const table = scroll.querySelector('table');
    const tableRect = table.getBoundingClientRect();
    const scrollRect = scroll.getBoundingClientRect();
    let left = clientX - colMove.grabX;
    const maxLeft = scrollRect.right - ghost.offsetWidth;
    left = Math.min(Math.max(left, scrollRect.left), maxLeft);
    ghost.style.left = left + 'px';
    ghost.style.top = tableRect.top + 'px';
    ghost.style.height = tableRect.height + 'px';
}

function layoutRect(el) {
    const r = el.getBoundingClientRect();
    let tx = 0;
    const t = getComputedStyle(el).transform;
    if (t && t !== 'none') {
        const m = t.match(/matrix\(([^)]+)\)/);
        if (m) tx = parseFloat(m[1].split(',')[4]) || 0;
    }
    return { left: r.left - tx, width: el.offsetWidth };
}

function dropIndexFromX(clientX, dragIdx) {
    const cells = headCells();
    const minIdx = 2;
    const lastX = colMove.lastX == null ? clientX : colMove.lastX;
    const goingRight = clientX > lastX + 1;
    const goingLeft = clientX < lastX - 1;
    colMove.lastX = clientX;
    if (!goingRight && !goingLeft) return dragIdx;

    const ghost = colMove.ghost;
    const edge = ghost ? ghost.getBoundingClientRect() : { left: clientX, right: clientX };

    if (goingRight && dragIdx + 1 < cells.length) {
        const box = layoutRect(cells[dragIdx + 1]);
        if (edge.right > box.left + 4) return dragIdx + 1;
    }
    if (goingLeft && dragIdx - 1 >= minIdx) {
        const box = layoutRect(cells[dragIdx - 1]);
        if (edge.left < box.left + box.width - 4) return dragIdx - 1;
    }
    return dragIdx;
}

function autoScrollCols(clientX) {
    const scroll = document.querySelector('.table-scroll');
    const box = scroll.getBoundingClientRect();
    const edge = 48;
    if (clientX < box.left + edge) scroll.scrollLeft -= 24;
    else if (clientX > box.right - edge) scroll.scrollLeft += 24;
}

function finishColMove() {
    if (!colMove) return;
    document.removeEventListener('mousemove', onColMoveDrag);
    document.removeEventListener('mouseup', onColMoveUp);
    if (colMove.ghost) colMove.ghost.remove();
    if (colMove.id) markOriginColumn(colMove.id, false);
    document.querySelectorAll('#table-head-row th, #table-body td').forEach(el => {
        el.style.transform = '';
        el.style.transition = '';
    });
    document.body.classList.remove('col-moving');
    if (colMove.moved) {
        syncOrderFromDom();
        saveColumnPrefs();
    }
    colMove = null;
}

function onColMoveDrag(e) {
    if (!colMove) return;
    if (!colMove.active) {
        if (Math.abs(e.clientX - colMove.startX) < 5) return;
        colMove.active = true;
        colMove.moved = true;
        colMove.ghost = buildColGhost(colMove.id);
        markOriginColumn(colMove.id, true);
        document.body.classList.add('col-moving');
    }
    autoScrollCols(e.clientX);
    placeColGhost(colMove.ghost, e.clientX);
    const dragIdx = colIndexById(colMove.id);
    const target = dropIndexFromX(e.clientX, dragIdx);
    if (target !== dragIdx) moveTableColumn(dragIdx, target);
}

function onColMoveUp() {
    finishColMove();
}

function startColMove(e, id) {
    if (e.button !== 0 || e.target.closest('.th-resize')) return;
    e.preventDefault();
    const th = e.currentTarget;
    const rect = th.getBoundingClientRect();
    colMove = {
        id,
        startX: e.clientX,
        lastX: e.clientX,
        grabX: e.clientX - rect.left,
        active: false,
        moved: false,
        ghost: null,
    };
    document.addEventListener('mousemove', onColMoveDrag);
    document.addEventListener('mouseup', onColMoveUp);
}

function startColResize(e, id) {
    e.preventDefault();
    e.stopPropagation();
    const startX = e.clientX;
    const startW = colWidth(id);
    document.body.classList.add('col-resizing');
    function move(ev) {
        setColWidth(id, startW + ev.clientX - startX);
    }
    function up() {
        document.body.classList.remove('col-resizing');
        document.removeEventListener('mousemove', move);
        document.removeEventListener('mouseup', up);
        saveColumnPrefs();
    }
    document.addEventListener('mousemove', move);
    document.addEventListener('mouseup', up);
}

function thHtml(id, label, draggable) {
    const dragAttrs = draggable
        ? `class="th-col th-drag" data-col="${id}" onmousedown="startColMove(event, '${id}')"`
        : `class="th-col" data-col="${id}"`;
    return `<th ${dragAttrs}><span class="th-label">${esc(label)}</span><span class="th-resize" onmousedown="startColResize(event, '${id}')"></span></th>`;
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
    if (total > 0 && page > totalPages) page = totalPages;
    const startIdx = total === 0 ? 0 : (page - 1) * pageSize;
    document.getElementById('empty-state').style.display = !loading && total === 0 ? 'block' : 'none';

    const headRow = document.getElementById('table-head-row');
    const headCells = activeColumns.map(id => {
        const c = COLUMN_CATALOG.find(x => x.id === id);
        return c ? thHtml(c.id, c.label, true) : '';
    }).join('');
    headRow.innerHTML = `<th class="chk"><input type="checkbox" id="chk-all" onchange="toggleAll(this)"></th>${thHtml('uuid', 'UUID', false)}${headCells}`;
    applyColumnWidths();

    tbody.innerHTML = '';
    pageRows.forEach(r => {
        const tr = document.createElement('tr');
        const cells = activeColumns.map(id => {
            const c = COLUMN_CATALOG.find(x => x.id === id);
            return c ? `<td data-col="${c.id}">${c.get(r)}</td>` : '';
        }).join('');
        const uuid = r.uuid || ('#' + r.id);
        tr.innerHTML = `<td class="chk"><input type="checkbox" data-id="${r.id}" ${selected.has(r.id) ? 'checked' : ''} onchange="toggleRow(${r.id}, this)"></td><td data-col="uuid"><a href="javascript:void(0)" class="uuid-link" onclick="openCard(${r.id})">${esc(uuid)}</a></td>${cells}`;
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
    saveCardFieldPrefs();
    renderCardFieldPicker();
    refreshCardMain();
}
function moveCardField(srcId, targetId) {
    if (!srcId || srcId === targetId) return false;
    if (!cardVisibleFieldIds.includes(srcId) || !cardVisibleFieldIds.includes(targetId)) return false;
    const from = cardVisibleFieldIds.indexOf(srcId);
    const to = cardVisibleFieldIds.indexOf(targetId);
    cardVisibleFieldIds.splice(from, 1);
    cardVisibleFieldIds.splice(to, 0, srcId);
    return true;
}
function dropCardField(e, targetId) {
    e.preventDefault();
    if (!moveCardField(dragCardFieldId, targetId)) { renderCardFieldPicker(); return; }
    dragCardFieldId = null;
    saveCardFieldPrefs();
    renderCardFieldPicker();
    refreshCardMain();
}
function resetCardFields() {
    cardVisibleFieldIds = DEFAULT_CARD_FIELDS.slice();
    saveCardFieldPrefs();
    renderCardFieldPicker();
    refreshCardMain();
}
function loadCardFieldPrefs() {
    try {
        const raw = JSON.parse(localStorage.getItem(CARD_PREFS_KEY) || '{}');
        const known = new Set(CARD_FIELD_CATALOG.map(f => f.id));
        if (Array.isArray(raw.visible)) {
            const next = raw.visible.filter(id => known.has(id));
            if (next.length) cardVisibleFieldIds = next;
        }
    } catch (err) { /* ignore broken prefs */ }
}
function saveCardFieldPrefs() {
    try {
        localStorage.setItem(CARD_PREFS_KEY, JSON.stringify({ visible: cardVisibleFieldIds }));
    } catch (err) { /* quota / private mode */ }
}
function refreshCardMain() {
    if (currentCard) renderCardMain(currentCard);
}
function renderCardMain(r) {
    const visibleFields = cardVisibleFieldIds.map(id => CARD_FIELD_CATALOG.find(f => f.id === id)).filter(Boolean);
    document.getElementById('card-main').innerHTML = visibleFields.map(f => `
        <div class="kv kv-drag" draggable="true" data-fld="${f.id}"
             ondragstart="onCardKvDragStart(event, '${f.id}')"
             ondragend="onCardKvDragEnd(event)"
             ondragover="onCardKvDragOver(event)"
             ondrop="dropCardField(event, '${f.id}')">
          <span class="k"><span class="drag-handle">⠿</span>${f.label}</span>
          <span class="v">${f.get(r)}</span>
        </div>`).join('')
        || '<div style="grid-column:1/-1; color:var(--text-muted); font-size:13px;">Все поля скрыты — настройте через ⚙️ Поля.</div>';
}
function onCardKvDragStart(e, id) {
    dragCardFieldId = id;
    if (e.currentTarget) e.currentTarget.classList.add('dragging');
    if (e.dataTransfer) e.dataTransfer.setData('text/plain', id);
}
function onCardKvDragEnd(e) {
    if (e.currentTarget) e.currentTarget.classList.remove('dragging');
    document.querySelectorAll('#card-main .kv-drag.drop-target').forEach(el => el.classList.remove('drop-target'));
}
function onCardKvDragOver(e) {
    e.preventDefault();
    const el = e.currentTarget;
    if (!el || el.dataset.fld === dragCardFieldId) return;
    document.querySelectorAll('#card-main .kv-drag.drop-target').forEach(x => {
        if (x !== el) x.classList.remove('drop-target');
    });
    el.classList.add('drop-target');
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

    renderCardMain(r);

    const fields = r.skorozvon_anketa_fields;
    const anketa = r.skorozvon_anketa;
    const form = r.call_form;
    if (Array.isArray(fields) && fields.length) {
        const entries = fields.map(f => [
            f.name || ('FIELD_' + (f.field_id || '')),
            (f.values || []).filter(Boolean).join(', ') || '—',
        ]);
        document.getElementById('card-anketa').innerHTML = kvHtml(entries);
    } else if (anketa && Array.isArray(anketa.answers) && anketa.answers.length) {
        const entries = anketa.answers.map(a => [
            a.question_name || ('вопрос ' + (a.question_id || '')),
            (a.answer_values || []).filter(Boolean).join(', ') || '—',
        ]);
        document.getElementById('card-anketa').innerHTML = kvHtml(entries);
    } else if (form && typeof form === 'object' && !Array.isArray(form)) {
        const entries = Object.keys(form).map(k => [CALL_FORM_LABELS[k] || k, formatValue(form[k])]);
        document.getElementById('card-anketa').innerHTML = kvHtml(entries);
    } else if (form) {
        document.getElementById('card-anketa').innerHTML = kvHtml([['call_form', formatValue(form)]]);
    } else {
        document.getElementById('card-anketa').innerHTML = kvHtml([]);
    }

    document.getElementById('card-legacy').innerHTML = kvHtml(objectEntries(r.legacy_fields));

    const recSection = document.getElementById('card-recording-section');
    if (r.call_url) {
        recSection.style.display = '';
        const player = document.getElementById('card-recording-player');
        player.innerHTML = `
      <div class="call-player">
        <audio class="call-player-audio" controls preload="metadata" src="${esc(r.call_url)}"></audio>
        <p class="call-player-error" hidden>Не удалось загрузить запись.</p>
      </div>`;
        const audio = player.querySelector('audio');
        const errEl = player.querySelector('.call-player-error');
        audio.addEventListener('error', () => { errEl.hidden = false; });
    } else {
        recSection.style.display = 'none';
        document.getElementById('card-recording-player').innerHTML = '';
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
    (r.dispatches || []).forEach((d, i) => {
        const isPartner = d.channel_id === 2 || d.channel === 'вебхук-партнёр';
        const who = isPartner && d.upk ? `покупатель ${d.upk}` : (d.channel || '');
        const extra = [who, d.project_id, d.external_id ? 'ext ' + d.external_id : ''].filter(Boolean).join(' · ');
        items.push({
            at: d.sent_at,
            type: 'dispatch',
            label: 'Отправка: ' + (d.channel || 'канал'),
            detail: extra,
            ok: d.sent_ok,
            isPartner,
            partnerId: d.upk_id,
            raw: d.raw_response || '',
            httpStatus: d.http_status,
            idx: i,
        });
    });
    items.sort((a, b) => String(a.at).localeCompare(String(b.at)));
    document.getElementById('card-timeline').innerHTML = items.length
        ? items.map(e => {
            if (e.type !== 'dispatch') {
                return `<div class="tl-item">
        <div class="tl-label">${esc(e.label)}<span class="tl-badge ${e.ok ? 'ok' : 'fail'}">${e.ok ? 'ok' : 'ошибка'}</span></div>
        ${e.detail ? `<div class="tl-detail">${esc(e.detail)}</div>` : ''}
        <div class="tl-time">${esc(e.at || '')}</div>
      </div>`;
            }
            const canResend = e.isPartner && e.partnerId;
            const body = [e.httpStatus ? 'HTTP ' + e.httpStatus : '', e.raw].filter(Boolean).join('\n') || 'тело ответа пустое';
            return `<div class="tl-item tl-dispatch">
        <div class="tl-label">${esc(e.label)}<span class="tl-badge ${e.ok ? 'ok' : 'fail'}">${e.ok ? 'ok' : 'ошибка'}</span></div>
        <div class="tl-detail">
          ${e.isPartner ? `<button type="button" class="tl-chevron" id="tl-chevron-${e.idx}" onclick="toggleDispatchBody(${e.idx})" title="Показать ответ партнёра" aria-expanded="false">▸</button>` : ''}
          <span>${esc(e.detail || '')}</span>
          ${canResend ? `<button type="button" class="tl-resend" id="tl-resend-${e.idx}" title="Повторная отправка" aria-label="Повторная отправка" onclick="resendPartner(${e.partnerId}, ${e.idx})"><svg width="14" height="14" viewBox="0 0 20 20" fill="none" aria-hidden="true"><path d="M4.2 10a5.8 5.8 0 0 1 9.7-4.3L15.5 7.2M15.8 10a5.8 5.8 0 0 1-9.7 4.3L4.5 12.8" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/><path d="M11.8 7.2h3.7V3.6M8.2 12.8H4.5v3.6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/></svg></button>` : ''}
        </div>
        ${e.isPartner ? `<pre class="tl-body" id="tl-body-${e.idx}" hidden>${esc(body)}</pre>` : ''}
        <div class="tl-time">${esc(e.at || '')}</div>
      </div>`;
        }).join('')
        : '<div style="color:var(--text-muted); font-size:13px;">Записей нет</div>';

    const rawSection = document.getElementById('card-raw-section');
    if (rawSection) {
        if (r.s3_key) {
            rawSection.style.display = '';
            rawSection.innerHTML = `<h3>Сырьё</h3><a class="raw-link" href="javascript:void(0)" onclick="downloadLeadRaw()">${esc(r.s3_key)}</a>`;
        } else {
            rawSection.innerHTML = '<h3>Сырьё</h3><div style="color:var(--text-muted); font-size:13px;">s3_key пустой — у исторических строк сырьё может быть в legacy_fields.</div>';
        }
    }

    document.getElementById('card-overlay').classList.add('show');
}

function toggleDispatchBody(idx) {
    const el = document.getElementById('tl-body-' + idx);
    const btn = document.getElementById('tl-chevron-' + idx);
    if (!el) return;
    const open = el.hidden;
    el.hidden = !open;
    if (btn) {
        btn.classList.toggle('open', open);
        btn.setAttribute('aria-expanded', open ? 'true' : 'false');
        btn.textContent = open ? '▾' : '▸';
    }
}

async function resendPartner(partnerId, idx) {
    const lead = currentCard;
    if (!lead || (!lead.id && !lead.uuid) || !partnerId) return;
    const btn = document.getElementById('tl-resend-' + idx);
    if (btn) btn.disabled = true;
    const ref = lead.id || lead.uuid;
    try {
        const res = await fetch(API + '/rest/leads/' + encodeURIComponent(ref) + '/partners/' + partnerId + '/resend', { method: 'POST' });
        const body = await res.json();
        if (!res.ok || body.success === false) throw new Error(apiError(body, res.statusText));
        const data = apiData(body) || {};
        showToast(data.sent_ok ? 'Повторно отправлено партнёру' : 'Партнёр ответил ошибкой');
        if (lead.uuid) await openCardByUUID(lead.uuid);
        else await openCard(lead.id);
    } catch (err) {
        if (btn) btn.disabled = false;
        showToast('Не удалось отправить: ' + err.message, true);
    }
}

async function downloadLeadRaw() {
    const lead = currentCard;
    if (!lead || (!lead.id && !lead.uuid)) {
        showToast('Нет идентификатора строки для скачивания сырья.', true);
        return;
    }
    const ref = lead.uuid || lead.id;
    try {
        const res = await fetch(API + '/rest/leads/' + encodeURIComponent(ref) + '/raw');
        if (!res.ok) {
            let message = res.statusText;
            try {
                const body = await res.json();
                message = apiError(body, message);
            } catch (err) { /* not json */ }
            throw new Error(message);
        }
        const blob = await res.blob();
        const name = filenameFromDisposition(res.headers.get('Content-Disposition'), (lead.s3_key || 'raw.json').split('/').pop());
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = name;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
    } catch (err) {
        showToast('Не удалось скачать сырьё: ' + err.message, true);
    }
}

function filenameFromDisposition(header, fallback) {
    const m = String(header || '').match(/filename="([^"]+)"/i);
    return (m && m[1]) || fallback || 'raw.json';
}

function knownLeadUUID(id) {
    const row = pageRows.find(r => r.id === id);
    if (row && row.uuid) return row.uuid;
    const hist = (currentCard && currentCard.phone_history) || [];
    const item = hist.find(x => x.id === id);
    return item && item.uuid ? item.uuid : '';
}

async function openCard(id) {
    stopCardRecording();
    currentCardId = id;
    document.getElementById('card-phone').textContent = 'Загрузка…';
    document.getElementById('card-sub').textContent = 'GET /rest/leads/' + id;
    document.getElementById('card-overlay').classList.add('show');
    const known = knownLeadUUID(id);
    if (known) writeListURL(known);
    try {
        const res = await fetch(API + '/rest/leads/' + id);
        const body = await res.json();
        if (!res.ok || body.success === false) throw new Error(apiError(body, res.statusText));
        const data = apiData(body);
        renderCard(data.lead);
        writeListURL(data.lead.uuid);
    } catch (err) {
        document.getElementById('card-phone').textContent = 'Ошибка';
        document.getElementById('card-sub').textContent = err.message;
        showToast('Не удалось открыть строку: ' + err.message, true);
    }
}

async function openCardByUUID(uuid) {
    stopCardRecording();
    document.getElementById('card-phone').textContent = 'Загрузка…';
    document.getElementById('card-sub').textContent = 'GET /rest/leads/' + uuid;
    document.getElementById('card-overlay').classList.add('show');
    writeListURL(uuid);
    try {
        const res = await fetch(API + '/rest/leads/' + encodeURIComponent(uuid));
        const body = await res.json();
        if (!res.ok || body.success === false) throw new Error(apiError(body, res.statusText));
        const data = apiData(body);
        renderCard(data.lead);
        writeListURL(data.lead.uuid);
    } catch (err) {
        document.getElementById('card-phone').textContent = 'Ошибка';
        document.getElementById('card-sub').textContent = err.message;
        showToast('Не удалось открыть строку: ' + err.message, true);
    }
}

function stopCardRecording() {
    const audio = document.querySelector('#card-recording-player audio');
    if (!audio) return;
    audio.pause();
}

function closeCard() {
    stopCardRecording();
    document.getElementById('card-overlay').classList.remove('show');
    currentCardId = null;
    currentCard = null;
    writeListURL('');
}

function leadHash() {
    return decodeURIComponent((location.hash || '').replace(/^#/, '')).trim();
}

function readListURL() {
    const q = new URLSearchParams(location.search);
    const p = parseInt(q.get('page'), 10);
    if (p >= 1) page = p;
    const size = parseInt(q.get('page_size'), 10);
    if (size >= 1) pageSize = size;
}

function writeListURL(uuid) {
    const q = new URLSearchParams(location.search);
    q.set('page', String(Math.max(1, page)));
    if (pageSize !== DEFAULT_PAGE_SIZE) q.set('page_size', String(pageSize));
    else q.delete('page_size');
    const search = q.toString();
    const hashUUID = uuid === undefined ? leadHash() : String(uuid || '').trim();
    const hash = hashUUID ? '#' + hashUUID : '';
    const next = location.pathname + (search ? '?' + search : '') + hash;
    if (location.pathname + location.search + location.hash === next) return;
    syncingURL = true;
    history.replaceState(null, '', next);
    syncingURL = false;
}

window.addEventListener('hashchange', () => {
    if (syncingURL) return;
    const uuid = leadHash();
    if (uuid) openCardByUUID(uuid);
    else if (document.getElementById('card-overlay').classList.contains('show')) closeCard();
});

(async function init() {
    readListURL();
    const sizeSel = document.getElementById('page-size-sel');
    if (sizeSel) sizeSel.value = String(pageSize);
    loadColumnPrefs();
    loadCardFieldPrefs();
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
    const uuid = leadHash();
    if (uuid) await openCardByUUID(uuid);
})();
