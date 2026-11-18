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

function formatPhone(raw) {
    const s = String(raw == null ? '' : raw).trim();
    if (!s) return '—';
    let d = s.replace(/\D/g, '');
    if (d.length === 10) d = '7' + d;
    if (d.length === 11 && (d[0] === '7' || d[0] === '8')) {
        d = '7' + d.slice(1);
        return '+7 (' + d.slice(1, 4) + ') ' + d.slice(4, 7) + '-' + d.slice(7, 9) + '-' + d.slice(9, 11);
    }
    return s;
}

const STAGE_OPTIONS = [
    ['1', 'принята'],
    ['2', 'обогащена'],
    ['3', 'в СК'],
    ['4', 'из звонилки'],
    ['5', 'в Б24'],
    ['6', 'отклонена'],
    ['7', 'ожидание'],
];

const ZARYA_OPTIONS = [
    ['Чистый', 'Чистый'],
    ['Дубль', 'Дубль'],
    ['Ошибка проверки', 'Ошибка проверки'],
];
const ZARYA_OLD_OPTIONS = [
    ['Чистый', 'Чистый'],
    ['дубль', 'дубль'],
];

const SK_SEND_OPTIONS = [
    ['ok', 'Успех'],
    ['error', 'Ошибка'],
    ['none', 'Нет отправки'],
];

const FIELD_CATALOG = [
    { id: 'dateRange', label: 'Дата создания', group: 'Основные', type: 'daterange', pinned: true },
    { id: 'source', label: 'Источник', group: 'Основные', type: 'multiselect', pinned: true, options: [] },
    { id: 'childSource', label: 'Подисточник', group: 'Основные', type: 'multiselect', pinned: true, options: [] },
    { id: 'baza', label: 'Холодная база', group: 'Источник (JOIN)', type: 'text', pinned: true },
    { id: 'region', label: 'Регион', group: 'lead_row', type: 'multiselect', pinned: true, options: [] },
    { id: 'operator', label: 'Оператор связи', group: 'lead_row', type: 'multiselect', pinned: true, options: [] },
    { id: 'zarya', label: 'Проверка Зари', group: 'Основные', type: 'multiselect', pinned: true, options: ZARYA_OPTIONS },
    { id: 'id', label: 'ID', group: 'Основные', type: 'text', pinned: true },
    { id: 'phone', label: 'Телефон', group: 'Основные', type: 'text', pinned: true },
    { id: 'stage', label: 'Стадия', group: 'lead_row', type: 'multiselect', pinned: true, options: STAGE_OPTIONS },
    { id: 'viaUs', label: 'Из скорозвона строка', group: 'lead_row', type: 'multiselect', pinned: true, options: [['false', 'Создалась'], ['true', 'Обновилось']] },
    { id: 'skSend', label: 'Отправка в СК', group: 'dispatch', type: 'multiselect', pinned: true, options: SK_SEND_OPTIONS },
    { id: 'city', label: 'Город', group: 'lead_row', type: 'text' },
    { id: 'teg', label: 'Тег (вход)', group: 'lead_row', type: 'text' },
    { id: 'callResult', label: 'Результат звонка', group: 'lead_row', type: 'text' },
];
const DEFAULT_ACTIVE = FIELD_CATALOG.filter(f => f.pinned).map(f => f.id);
let activeFields = DEFAULT_ACTIVE.slice();
const multiState = {};

const COLUMN_CATALOG = [
    { id: 'id', label: 'ID', get: r => r.id != null ? esc(r.id) : '—' },
    { id: 'date', label: 'Дата создания', get: r => r.date_create || '—' },
    { id: 'source', label: 'Источник', get: r => esc(r.source || '—') },
    { id: 'childSource', label: 'Подисточник', get: r => esc(r.child_source || '—') },
    { id: 'baza', label: 'Холодная база', get: r => esc(r.cold_label || '—') },
    { id: 'kakaya', label: 'Какая база', get: r => esc(r.base_label || '—') },
    { id: 'sourceDesc', label: 'Дополнительно об источнике', get: r => esc(r.source_description || '—') },
    { id: 'region', label: 'Регион', get: r => esc(r.region || '—') },
    { id: 'city', label: 'Город', get: r => esc(r.city || '—') },
    { id: 'operator', label: 'Оператор', get: r => esc(r.operator || '—') },
    { id: 'stage', label: 'Стадия', get: r => `<span class="stage"><span class="dot dot-${r.status_id || 0}"></span>${esc(r.stage || '—')}</span>` },
    { id: 'viaUs', label: 'Из скорозвона строка', get: r => r.via_us === false ? 'Создалась' : 'Обновилось' },
    { id: 'skSend', label: 'Отправка в СК', get: r => skSendCell(r) },
    { id: 'zarya', label: 'Проверка Зари', get: r => r.zarya ? `<span class="badge ${zaryaBadgeClass(r.zarya)}">${esc(r.zarya)}</span>` : '—' },
    { id: 'zaryaOld', label: 'Старая проверка на дубль', get: r => r.zarya_old ? `<span class="badge ${zaryaBadgeClass(r.zarya_old)}">${esc(r.zarya_old)}</span>` : '—' },
    { id: 'callResult', label: 'Результат звонка', get: r => esc(r.call_result_name || (r.call_result != null ? String(r.call_result) : '') || '—') },
    { id: 'upk', label: 'ЮПК', get: r => esc(r.upk || '—') },
    { id: 'phone', label: 'Телефон', get: r => esc(formatPhone(r.phone)) },
    { id: 'teg', label: 'Тег (вход)', get: r => esc(r.tag || '—') },
    { id: 'webId', label: 'Web ID', get: r => esc(r.web_id || '—') },
    { id: 'projectNum', label: '№ проекта', get: r => esc(r.project_num || '—') },
];
const DEFAULT_COLUMNS = ['id', 'date', 'source', 'childSource', 'baza', 'kakaya', 'sourceDesc', 'region', 'operator', 'stage', 'viaUs', 'skSend', 'zarya', 'upk', 'phone'];
const COL_PREFS_KEY = 'sdmp.list.colprefs';
const CARD_PREFS_KEY = 'sdmp.list.cardfields';
const MIN_COL_WIDTH = 64;
const DEFAULT_COL_WIDTHS = {
    chk: 36, uuid: 280, id: 90, date: 150, source: 140, childSource: 160, baza: 140, kakaya: 140,
    sourceDesc: 180, region: 200, city: 140, operator: 120, stage: 120, viaUs: 170, skSend: 150, zarya: 140, zaryaOld: 180,
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
    { id: 'viaUs', label: 'Из скорозвона строка', get: r => r.via_us === false ? 'Создалась' : 'Обновилось' },
    { id: 'source', label: 'Источник', get: r => esc(r.source || '—') },
    { id: 'childSource', label: 'Подисточник', get: r => esc(r.child_source || '—') },
    { id: 'baza', label: 'Холодная база', get: r => esc(r.cold_label || '—') },
    { id: 'kakaya', label: 'Какая база', get: r => esc(r.base_label || '—') },
    { id: 'sourceDesc', label: 'Дополнительно об источнике', get: r => esc(r.source_description || '—') },
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
    { id: 'zaryaOld', label: 'Старая проверка на дубль', get: r => r.zarya_old ? `<span class="badge ${zaryaBadgeClass(r.zarya_old)}">${esc(r.zarya_old)}</span>` : '—' },
    { id: 'callResult', label: 'Результат звонка', get: r => esc(r.call_result_name || (r.call_result != null ? String(r.call_result) : '') || '—') },
    { id: 'upk', label: 'ЮПК', get: r => esc(r.upk || '—') },
    { id: 'id', label: 'ID', get: r => r.id != null ? esc(r.id) : '—' },
    { id: 'webId', label: 'Web ID', get: r => esc(r.web_id || '—') },
    { id: 'projectNum', label: '№ проекта', get: r => esc(r.project_num || '—') },
    { id: 'uuid', label: 'UUID', get: r => esc(r.uuid || '—') },
    { id: 'phoneRaw', label: 'Телефон', get: r => esc(formatPhone(r.phone)) },
    { id: 'phoneValid', label: 'Телефон валиден', get: r => r.phone_valid ? 'да' : 'нет' },
    { id: 'syryeKey', label: 's3_key', get: r => esc(r.s3_key || '—') },
    { id: 'bitrixLead', label: 'Bitrix lead id', get: r => esc(r.bitrix_lead_id || '—') },
    { id: 'createdAt', label: 'Дата создания', get: r => esc(r.created_at || '—') },
    { id: 'updatedAt', label: 'Дата обновления', get: r => esc(r.updated_at || '—') },
];
const DEFAULT_CARD_FIELDS = ['stage', 'viaUs', 'source', 'childSource', 'baza', 'kakaya', 'sourceDesc', 'region', 'operator', 'zarya', 'zaryaOld', 'callResult', 'upk', 'uuid'];
let cardVisibleFieldIds = DEFAULT_CARD_FIELDS.slice();
let currentCardId = null;
let currentCard = null;
let dragCardFieldId = null;

const DEFAULT_PAGE_SIZE = 15;
let catalogSources = [];
let pageRows = [];
let total = 0;
let page = 1;
let pageSize = DEFAULT_PAGE_SIZE;
let syncingURL = false;
let selected = new Set();
let loading = true;
let savedFilters = [];
let dragColId = null;
let leadsFetchSeq = 0;

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

function parseIDs(raw) {
    return String(raw || '').split(/[^\d]+/).filter(Boolean);
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
    catalogSources = allSources;
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
document.addEventListener('scroll', () => {
    const el = document.getElementById('import-source');
    if (el && el.classList.contains('open')) positionImportSourceDrop();
}, true);

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
    parseIDs(getFieldValue('id')).forEach(id => params.append('ids', id));
    const phone = String(getFieldValue('phone') || '').replace(/\D/g, '');
    if (phone) params.set('phone', phone);
    appendMulti(params, 'status', getFieldValue('stage'));
    const city = getFieldValue('city');
    if (city) params.set('city', city);
    const teg = getFieldValue('teg');
    if (teg) params.set('tag', teg);
    const callResult = getFieldValue('callResult');
    if (callResult) params.set('call_result', callResult);
    const viaUsRaw = getFieldValue('viaUs');
    const viaUs = (Array.isArray(viaUsRaw) ? viaUsRaw : [viaUsRaw]).filter(v => v === 'true' || v === 'false');
    if (viaUs.length === 1) params.set('via_us', viaUs[0]);
    appendMulti(params, 'sk_send', skSendFilterValues());
    params.set('limit', String(pageSize));
    params.set('offset', String((page - 1) * pageSize));
    return params;
}

function skSendFilterValues() {
    const raw = getFieldValue('skSend');
    return (Array.isArray(raw) ? raw : [raw]).filter(v => v === 'ok' || v === 'error' || v === 'none');
}

async function fetchLeads() {
    loading = true;
    const seq = ++leadsFetchSeq;
    render();
    try {
        const query = buildQuery();
        const res = await fetch(API + '/rest/leads?' + query.toString());
        const body = await res.json();
        if (!res.ok || body.success === false) throw new Error(apiError(body, res.statusText));
        if (seq !== leadsFetchSeq) return;
        const data = apiData(body);
        pageRows = data.leads || [];
        total = data.total || 0;
        pageSize = data.limit || pageSize;
        const skFiltered = skSendFilterValues().length > 0;
        pageRows.forEach(r => { r.sk_loaded = skFiltered; });
        loading = false;
        render();
        writeListURL();
        if (!skFiltered && activeColumns.includes('skSend') && pageRows.length) {
            attachSkDispatch(pageRows, seq);
        }
    } catch (err) {
        if (seq !== leadsFetchSeq) return;
        pageRows = [];
        total = 0;
        showToast('Не удалось загрузить строки: ' + err.message, true);
        loading = false;
        render();
        writeListURL();
    }
}

async function attachSkDispatch(rows, seq) {
    const ids = rows.map(r => r.id).filter(id => id != null);
    if (!ids.length) return;
    const params = new URLSearchParams();
    ids.forEach(id => params.append('ids', String(id)));
    try {
        const res = await fetch(API + '/rest/leads/sk-dispatch?' + params.toString());
        const body = await res.json();
        if (!res.ok || body.success === false) throw new Error(apiError(body, res.statusText));
        if (seq !== leadsFetchSeq) return;
        const data = apiData(body);
        const byID = {};
        (data.items || []).forEach(item => { byID[item.id] = item; });
        rows.forEach(r => {
            const d = byID[r.id];
            r.sk_loaded = true;
            if (!d) {
                r.sk_sent_ok = null;
                r.sk_http_status = null;
                r.sk_error = '';
                return;
            }
            r.sk_sent_ok = d.sent_ok;
            r.sk_http_status = d.http_status;
            r.sk_error = d.error || '';
        });
        render();
    } catch (err) {
        if (seq !== leadsFetchSeq) return;
        rows.forEach(r => { r.sk_loaded = true; });
        render();
        showToast('Не удалось загрузить отправку в СК: ' + err.message, true);
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
                if (!activeColumns.includes('skSend')) {
                    const after = activeColumns.indexOf('viaUs');
                    if (after >= 0) activeColumns.splice(after + 1, 0, 'skSend');
                    else activeColumns.push('skSend');
                }
                if (!activeColumns.includes('id')) {
                    activeColumns.unshift('id');
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

function skSendCell(r) {
    if (r.sk_sent_ok === true) {
        const code = r.sk_http_status != null ? ' ' + r.sk_http_status : '';
        return `<span class="badge badge-clean" title="${esc(r.sk_error || '')}">Успех${esc(code)}</span>`;
    }
    if (r.sk_sent_ok === false) {
        const code = r.sk_http_status != null ? ' ' + r.sk_http_status : '';
        return `<span class="badge badge-dup" title="${esc(r.sk_error || '')}">Ошибка${esc(code)}</span>`;
    }
    if (!r.sk_loaded) return '<span class="sk-send-pending">…</span>';
    return '—';
}

function render() {
    const tbody = document.getElementById('table-body');
    document.getElementById('total-count').textContent = loading ? '…' : total.toLocaleString('ru-RU');
    const totalPages = Math.max(1, Math.ceil(total / pageSize));
    if (total > 0 && page > totalPages) page = totalPages;
    const startIdx = total === 0 ? 0 : (page - 1) * pageSize;
    document.getElementById('empty-state').style.display = !loading && total === 0 ? 'block' : 'none';
    const scroll = document.getElementById('table-scroll');
    const loadingEl = document.getElementById('table-loading');
    if (scroll) scroll.classList.toggle('is-loading', loading);
    if (loadingEl) loadingEl.hidden = !loading;

    const headRow = document.getElementById('table-head-row');
    const headCells = activeColumns.map(id => {
        const c = COLUMN_CATALOG.find(x => x.id === id);
        return c ? thHtml(c.id, c.label, true) : '';
    }).join('');
    const allOn = pageRows.length > 0 && pageRows.every(r => selected.has(r.id));
    headRow.innerHTML = `<th class="chk"><input type="checkbox" id="chk-all" ${allOn ? 'checked' : ''} onchange="toggleAll(this)"></th>${thHtml('uuid', 'UUID', false)}${headCells}`;
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
    syncChkAll();
}

function toggleRow(id, el) {
    if (el.checked) selected.add(id);
    else selected.delete(id);
    updateSelectedInfo();
    syncChkAll();
}
function toggleAll(el) {
    pageRows.forEach(r => { if (el.checked) selected.add(r.id); else selected.delete(r.id); });
    render();
}
function syncChkAll() {
    const el = document.getElementById('chk-all');
    if (!el) return;
    const n = pageRows.length;
    const c = pageRows.filter(r => selected.has(r.id)).length;
    el.checked = n > 0 && c === n;
    el.indeterminate = c > 0 && c < n;
}
function updateSelectedInfo() {
    const info = document.getElementById('selected-info');
    document.getElementById('selected-count').textContent = selected.size;
    info.style.display = selected.size > 0 ? 'inline' : 'none';
}

function exportData(mode) {
    const count = mode === 'selected' ? selected.size : total;
    if (count === 0) { showToast('Нечего экспортировать — список пуст.', true); return; }
    if (!activeColumns.length) { showToast('Выберите хотя бы одну колонку.', true); return; }
    const cols = ['uuid', ...activeColumns.filter(id => id !== 'uuid')];
    const params = new URLSearchParams();
    const dates = getFieldValue('dateRange') || {};
    params.set('date_from', toApiDate(dates.from || defaultDateFrom(), false));
    params.set('date_to', toApiDate(dates.to || defaultDateTo(), true));
    cols.forEach(id => params.append('columns', id));
    if (mode === 'selected') {
        [...selected].forEach(id => params.append('ids', String(id)));
    } else {
        const q = buildQuery();
        q.delete('limit');
        q.delete('offset');
        for (const [k, v] of q.entries()) {
            if (k === 'date_from' || k === 'date_to') continue;
            params.append(k, v);
        }
    }
    downloadExport(params, count);
}

async function downloadExport(params, count) {
    try {
        showToast('Готовим файл…');
        const res = await fetch(API + '/rest/leads/export.xlsx?' + params.toString());
        if (!res.ok) {
            let msg = res.statusText;
            try {
                const body = await res.json();
                msg = apiError(body, msg);
            } catch (e) { /* not json */ }
            throw new Error(msg);
        }
        const blob = await res.blob();
        const dispo = res.headers.get('Content-Disposition') || '';
        const m = /filename="([^"]+)"/.exec(dispo);
        const name = m ? m[1] : 'leads.xlsx';
        const url = URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = name;
        document.body.appendChild(a);
        a.click();
        a.remove();
        URL.revokeObjectURL(url);
        showToast('Скачано ' + count.toLocaleString('ru-RU') + ' строк.');
    } catch (err) {
        showToast('Не удалось экспортировать: ' + err.message, true);
    }
}
function showToast(msg, isError) {
    const t = document.getElementById('export-toast');
    t.textContent = msg;
    t.style.borderColor = isError ? '#e0b3a8' : '#c3d9c3';
    t.style.background = isError ? '#fbece8' : '#eaf1e8';
    t.style.color = isError ? '#a23b2e' : '#2f5233';
    t.classList.add('show');
}

function toggleActionMenu(e) {
    e.stopPropagation();
    document.getElementById('action-menu').classList.toggle('show');
}
document.addEventListener('click', (e) => {
    const menu = document.getElementById('action-menu');
    if (menu && !e.target.closest('#action-menu-wrap')) menu.classList.remove('show');
});
function runAction(kind) {
    document.getElementById('action-menu').classList.remove('show');
    if (kind === 'export-selected') exportData('selected');
    else if (kind === 'export-filter') exportData('all');
    else if (kind === 'resend-sk') resendSkorozvon();
    else if (kind === 'import') openImport();
}

async function resendSkorozvon() {
    const ids = [...selected];
    if (!ids.length) {
        showToast('Выберите строки для повторной отправки в СК.', true);
        return;
    }
    try {
        showToast('Ставим в очередь Скорозвона…');
        const res = await fetch(API + '/rest/leads/skorozvon/resend', {
            method: 'POST',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ ids }),
        });
        const body = await res.json();
        if (!res.ok || body.success === false) throw new Error(apiError(body, res.statusText));
        const data = apiData(body);
        const queued = data.queued || 0;
        const missing = data.missing || 0;
        let msg = 'В очередь Скорозвона: ' + queued;
        if (missing) msg += ', не найдены: ' + missing;
        showToast(msg, queued === 0);
        fetchLeads();
    } catch (err) {
        showToast('Не удалось переотправить в СК: ' + err.message, true);
    }
}

let importPreview = null;
const importMap = {};
let importStep = 1;
let importSourceId = '';
let importBusy = false;
let importAbort = false;

function openImport() {
    importPreview = null;
    importStep = 1;
    importSourceId = '';
    importBusy = false;
    importAbort = false;
    Object.keys(importMap).forEach(k => delete importMap[k]);
    const file = document.getElementById('import-file');
    if (file) file.value = '';
    setImportError('');
    document.getElementById('import-loading').hidden = true;
    document.getElementById('import-map-wrap').hidden = true;
    document.getElementById('import-project-wrap').hidden = true;
    document.getElementById('import-upload-wrap').hidden = true;
    document.getElementById('import-foot').hidden = true;
    document.getElementById('import-drop').hidden = false;
    document.getElementById('import-sub').textContent = 'xlsx или csv. Сопоставление колонок, источник, затем очередь по связкам.';
    document.getElementById('import-overlay').classList.add('show');
}
function closeImport() {
    importAbort = true;
    importBusy = false;
    document.getElementById('import-overlay').classList.remove('show');
}
function setImportError(msg) {
    const el = document.getElementById('import-error');
    if (!msg) { el.hidden = true; el.textContent = ''; return; }
    el.hidden = false;
    el.textContent = msg;
}
async function uploadImportFile(file) {
    if (!file || importBusy) return;
    const name = String(file.name || '').toLowerCase();
    if (!/\.(xlsx|csv|txt)$/.test(name)) {
        setImportError('Нужен файл xlsx или csv.');
        return;
    }
    setImportError('');
    document.getElementById('import-loading').hidden = false;
    document.getElementById('import-map-wrap').hidden = true;
    document.getElementById('import-foot').hidden = true;
    const body = new FormData();
    body.append('file', file);
    try {
        const res = await fetch(API + '/rest/leads/import/preview', { method: 'POST', body });
        const json = await res.json().catch(() => null);
        if (!res.ok) throw new Error(apiError(json, res.statusText));
        importPreview = apiData(json);
        importStep = 1;
        importSourceId = '';
        Object.keys(importMap).forEach(k => delete importMap[k]);
        const suggested = importPreview.suggested || {};
        Object.keys(suggested).forEach(idx => { importMap[idx] = suggested[idx]; });
        renderImportMapping();
        showImportStep(1);
    } catch (err) {
        setImportError('Не удалось прочитать файл: ' + err.message);
    } finally {
        document.getElementById('import-loading').hidden = true;
        const input = document.getElementById('import-file');
        if (input) input.value = '';
    }
}
function importRowCount() {
    if (!importPreview) return 0;
    if (Array.isArray(importPreview.rows)) return importPreview.rows.length;
    return Number(importPreview.row_count) || 0;
}
function importHasPhone() {
    return Object.values(importMap).includes('phone');
}
function showImportStep(step) {
    importStep = step;
    const loaded = !!importPreview;
    document.getElementById('import-drop').hidden = step !== 1;
    document.getElementById('import-map-wrap').hidden = !(loaded && step === 1);
    document.getElementById('import-project-wrap').hidden = step !== 2;
    document.getElementById('import-upload-wrap').hidden = step !== 3;
    document.getElementById('import-foot').hidden = !loaded;
    document.getElementById('import-step-n').textContent = String(step);
    document.getElementById('import-back').hidden = step === 1 || importBusy;
    const next = document.getElementById('import-next');
    next.hidden = false;
    next.disabled = importBusy;
    if (step === 1) {
        next.textContent = 'Далее';
        const n = importRowCount();
        document.getElementById('import-sub').textContent =
            (importPreview.filename || 'файл') + ' · ' + n.toLocaleString('ru-RU') + ' строк' +
            (importPreview.truncated ? ' (обрезано)' : '');
    } else if (step === 2) {
        next.textContent = 'Далее';
        document.getElementById('import-sub').textContent = 'Источник. Подисточник и проект назначатся по связкам.';
        renderImportSources();
    } else {
        next.textContent = importBusy ? 'Загрузка…' : 'Закрыть';
    }
}
function importBack() {
    if (importBusy) return;
    setImportError('');
    if (importStep === 2) showImportStep(1);
    else if (importStep === 3) showImportStep(2);
}
function importNext() {
    if (importStep === 1) {
        if (!importHasPhone()) { setImportError('Сопоставьте колонку с полем «Телефон» (phone).'); return; }
        if (importRowCount() === 0) { setImportError('В файле нет строк данных.'); return; }
        setImportError('');
        showImportStep(2);
        return;
    }
    if (importStep === 2) {
        if (!importSourceId) { setImportError('Выберите источник.'); return; }
        setImportError('');
        showImportStep(3);
        startImportUpload();
        return;
    }
    if (importStep === 3 && !importBusy) closeImport();
}
function importParentSources() {
    return (catalogSources || []).filter(s => !s.parent_id).sort((a, b) => (a.name || '').localeCompare(b.name || '', 'ru'));
}
function importSourceLabel() {
    if (!importSourceId) return '';
    const s = importParentSources().find(x => String(x.id) === String(importSourceId));
    return s ? (s.name || String(s.id)) : String(importSourceId);
}
function renderImportSources() {
    const wrap = document.getElementById('import-source');
    const opts = document.getElementById('import-source-opts');
    const label = wrap && wrap.querySelector('.ms-label');
    if (!wrap || !opts) return;
    if (!catalogSources.length) {
        loadCatalogs().then(() => { if (importStep === 2) renderImportSources(); }).catch(() => {});
    }
    const parents = importParentSources();
    opts.innerHTML = parents.length
        ? parents.map(s => {
            const sel = String(s.id) === String(importSourceId) ? ' is-selected' : '';
            return `<div class="ms-opt${sel}" data-id="${esc(String(s.id))}" data-text="${esc(String(s.name || s.id).toLowerCase())}" onclick="selectImportSource(this.dataset.id)">${esc(s.name || s.id)}</div>`;
        }).join('')
        : '<div class="ms-empty">Нет источников</div>';
    if (label) label.textContent = importSourceLabel() || 'Выберите источник';
}
function toggleImportSourceMs(e) {
    e.stopPropagation();
    const el = document.getElementById('import-source');
    if (!el) return;
    const open = el.classList.contains('open');
    document.querySelectorAll('.ms.open').forEach(x => x.classList.remove('open'));
    if (open) return;
    el.classList.add('open');
    positionImportSourceDrop();
    const search = el.querySelector('.ms-search');
    if (search) {
        search.value = '';
        filterImportSourceMs('');
        search.focus();
    }
}
function positionImportSourceDrop() {
    const el = document.getElementById('import-source');
    const drop = el && el.querySelector('.ms-drop');
    const trigger = el && el.querySelector('.ms-trigger');
    if (!drop || !trigger) return;
    const rect = trigger.getBoundingClientRect();
    drop.style.top = (rect.bottom + 4) + 'px';
    drop.style.left = rect.left + 'px';
    drop.style.width = Math.max(rect.width, 240) + 'px';
}
function filterImportSourceMs(query) {
    const el = document.getElementById('import-source');
    if (!el) return;
    const q = (query || '').toLowerCase();
    el.querySelectorAll('.ms-opt').forEach(opt => {
        opt.classList.toggle('hidden', Boolean(q) && !(opt.dataset.text || '').includes(q));
    });
}
function selectImportSource(id) {
    importSourceId = id || '';
    setImportError('');
    const wrap = document.getElementById('import-source');
    if (wrap) wrap.classList.remove('open');
    renderImportSources();
}
function importSelectLabel() {
    return importSourceLabel() || String(importSourceId || '');
}
function setImportProgress(done, total, extra) {
    const pct = total ? Math.min(100, Math.round(done * 100 / total)) : 0;
    document.getElementById('import-progress-fill').style.width = pct + '%';
    document.getElementById('import-progress-text').textContent =
        done.toLocaleString('ru-RU') + ' / ' + total.toLocaleString('ru-RU') + ' в очередь' + (extra ? ' · ' + extra : '');
}
async function startImportUpload() {
    const rows = (importPreview && importPreview.rows) || [];
    const total = rows.length;
    importBusy = true;
    importAbort = false;
    showImportStep(3);
    setImportProgress(0, total, '');
    document.getElementById('import-upload-hint').textContent =
        'Источник: ' + importSelectLabel() +
        '. Подисточник и проект — по связкам. Пачки по 500.';
    let queued = 0;
    let failed = 0;
    const mapping = {};
    Object.keys(importMap).forEach(k => { mapping[k] = importMap[k]; });
    try {
        for (let i = 0; i < rows.length; i += 500) {
            if (importAbort) throw new Error('остановлено');
            const chunk = rows.slice(i, i + 500);
            const res = await fetch(API + '/rest/leads/import', {
                method: 'POST',
                headers: { 'Content-Type': 'application/json' },
                body: JSON.stringify({
                    source_id: Number(importSourceId),
                    mapping,
                    rows: chunk,
                }),
            });
            const json = await res.json().catch(() => null);
            if (!res.ok) throw new Error(apiError(json, res.statusText));
            const data = apiData(json) || {};
            queued += Number(data.queued) || 0;
            failed += Number(data.failed) || 0;
            setImportProgress(Math.min(queued + failed, total), total, failed ? ('ошибок ' + failed) : '');
        }
        document.getElementById('import-sub').textContent =
            'В очередь: ' + queued.toLocaleString('ru-RU') +
            (failed ? ', не принято ' + failed.toLocaleString('ru-RU') : '') +
            '. Дальше воркер обогатит и отправит по связкам.';
        document.getElementById('import-upload-hint').textContent = 'Готово. В очереди outbox — стандартный пайплайн воркера.';
    } catch (err) {
        setImportError('Импорт прерван: ' + err.message);
        document.getElementById('import-sub').textContent = 'Загрузка не завершена.';
    } finally {
        importBusy = false;
        const next = document.getElementById('import-next');
        next.disabled = false;
        next.textContent = 'Закрыть';
        document.getElementById('import-back').hidden = true;
    }
}
function renderImportMapping() {
    const wrap = document.getElementById('import-map');
    const cols = (importPreview && importPreview.columns) || [];
    const fields = (importPreview && importPreview.fields) || [];
    const used = new Set(Object.values(importMap).filter(Boolean));
    wrap.innerHTML = cols.map(col => {
        const cur = importMap[String(col.index)] || '';
        const samples = (col.samples || []).join(' · ');
        const opts = ['<option value="">— не маппить —</option>'].concat(fields.map(f => {
            const taken = used.has(f.id) && f.id !== cur;
            return `<option value="${esc(f.id)}" ${f.id === cur ? 'selected' : ''} ${taken ? 'disabled' : ''}>${esc(f.label)} (${esc(f.column)})</option>`;
        }));
        return `<div class="import-map-row">
          <div class="import-map-col">
            <div class="import-map-name" title="${esc(col.name)}">${esc(col.name)}</div>
            <div class="import-map-samples">${samples ? esc(samples) : 'пусто'}</div>
          </div>
          <div class="import-map-arrow">→</div>
          <select onchange="setImportField(${col.index}, this.value)">${opts.join('')}</select>
        </div>`;
    }).join('');
    renderImportPreview();
}
function setImportField(index, fieldId) {
    const key = String(index);
    if (!fieldId) delete importMap[key];
    else {
        Object.keys(importMap).forEach(k => { if (importMap[k] === fieldId) delete importMap[k]; });
        importMap[key] = fieldId;
    }
    renderImportMapping();
}
function renderImportPreview() {
    const table = document.getElementById('import-preview-table');
    const hint = document.getElementById('import-preview-hint');
    const fields = (importPreview && importPreview.fields) || [];
    const rows = (importPreview && importPreview.rows) || [];
    const byId = {};
    fields.forEach(f => { byId[f.id] = f; });
    const mapped = Object.keys(importMap)
        .map(idx => ({ index: Number(idx), fieldId: importMap[idx] }))
        .filter(x => x.fieldId && byId[x.fieldId])
        .sort((a, b) => a.index - b.index);
    const n = Number(importPreview && importPreview.row_count) || rows.length;
    const previewRows = rows.slice(0, 8);
    if (!mapped.length) {
        hint.textContent = 'Выберите хотя бы одно поле, чтобы увидеть превью.';
        table.innerHTML = '<tbody><tr><td class="import-preview-empty">Нет сопоставленных полей</td></tr></tbody>';
        return;
    }
    hint.textContent = 'Первые ' + previewRows.length + ' из ' + n.toLocaleString('ru-RU') + ' строк.';
    const head = mapped.map(m => `<th>${esc(byId[m.fieldId].label)}<div style="font-weight:400;color:var(--text-muted);text-transform:none;">${esc(byId[m.fieldId].column)}</div></th>`).join('');
    const body = previewRows.map(row => {
        const cells = mapped.map(m => {
            const v = row[m.index] == null || row[m.index] === '' ? '—' : String(row[m.index]);
            return `<td title="${esc(v)}">${esc(v)}</td>`;
        }).join('');
        return `<tr>${cells}</tr>`;
    }).join('');
    table.innerHTML = `<thead><tr>${head}</tr></thead><tbody>${body || '<tr><td class="import-preview-empty" colspan="' + mapped.length + '">В файле только заголовок</td></tr>'}</tbody>`;
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
function queueCardRows(r) {
    return (r.queues || []).map(q => {
        const bits = [q.status || 'в очереди на отправку'];
        if (q.uuid) bits.push(q.uuid);
        if (q.count_retry > 0) bits.push('повтор ' + q.count_retry);
        return `<div class="kv">
          <span class="k">Очередь: ${esc(q.channel || '')}</span>
          <span class="v">${esc(bits.join(' · '))}</span>
        </div>`;
    }).join('');
}
function renderCardMain(r) {
    const visibleFields = cardVisibleFieldIds.map(id => CARD_FIELD_CATALOG.find(f => f.id === id)).filter(Boolean);
    const fields = visibleFields.map(f => `
        <div class="kv kv-drag" draggable="true" data-fld="${f.id}"
             ondragstart="onCardKvDragStart(event, '${f.id}')"
             ondragend="onCardKvDragEnd(event)"
             ondragover="onCardKvDragOver(event)"
             ondrop="dropCardField(event, '${f.id}')">
          <span class="k"><span class="drag-handle">⠿</span>${f.label}</span>
          <span class="v">${f.get(r)}</span>
        </div>`).join('')
        || '<div style="grid-column:1/-1; color:var(--text-muted); font-size:13px;">Все поля скрыты — настройте через ⚙️ Поля.</div>';
    document.getElementById('card-main').innerHTML = fields + queueCardRows(r);
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
    document.getElementById('card-phone').textContent = formatPhone(r.phone);
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
    if (r.call_url || r.call_id) {
        recSection.style.display = '';
        const recURL = API + '/rest/leads/' + encodeURIComponent(r.id || r.uuid) + '/recording';
        const player = document.getElementById('card-recording-player');
        player.innerHTML = `
      <div class="call-player">
        <audio class="call-player-audio" controls preload="metadata" src="${esc(recURL)}"></audio>
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
    (r.queues || []).forEach(q => {
        const extra = [q.uuid || '', q.count_retry > 0 ? 'повтор ' + q.count_retry : ''].filter(Boolean).join(' · ');
        items.push({
            at: q.last_retry || r.updated_at || r.created_at || '',
            type: 'queue',
            label: 'Отправка: ' + (q.channel || ''),
            detail: extra,
            status: q.status || 'в очереди на отправку',
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
            if (e.type === 'queue') {
                return `<div class="tl-item tl-queue">
        <div class="tl-label">${esc(e.label)}<span class="tl-badge wait">${esc(e.status || 'в очереди на отправку')}</span></div>
        ${e.detail ? `<div class="tl-detail">${esc(e.detail)}</div>` : ''}
        <div class="tl-time">${esc(e.at || '')}</div>
      </div>`;
            }
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
