const PUBLIC_ORIGIN = 'https://cdp-dmp.ru';
const API = PUBLIC_ORIGIN;

function receiveLeadUrl(token) {
  return PUBLIC_ORIGIN + '/rest/1/' + encodeURIComponent(token) + '/crm.lead.add.json';
}

function mapSource(s) {
  return {
    id: s.id,
    nazvanie: s.name,
    bitrix_code: s.bitrix_code || '',
    token: s.token || null,
    roditel_id: s.parent_id ?? null,
    metka_holodnaya: s.cold_label || '',
    metka_kakaya: s.base_label || '',
    aktiven: s.is_active !== false,
  };
}

async function apiJSON(path, opts) {
  const res = await fetch(API + path, opts);
  const body = await res.json().catch(() => ({}));
  if (!res.ok || body.success === false) {
    const err = (body.data && body.data.error) || body.error || res.statusText || 'request failed';
    throw new Error(err);
  }
  if (body && typeof body === 'object' && 'data' in body && 'success' in body) {
    return body.data;
  }
  return body;
}

function errorHost() {
  const order = ['custom-prompt-overlay', 'google-overlay', 'svyazka-overlay', 'partner-overlay', 'drob-overlay'];
  for (const id of order) {
    const overlay = document.getElementById(id);
    if (overlay && overlay.classList.contains('show')) {
      return overlay.querySelector('.form-error') || document.getElementById('page-error');
    }
  }
  return document.getElementById('page-error');
}

function clearError() {
  document.querySelectorAll('.form-error').forEach(el => {
    el.classList.remove('show');
    el.textContent = '';
  });
}

function showError(msg) {
  clearError();
  const box = errorHost();
  if (!box) return;
  box.textContent = msg;
  box.classList.add('show');
}

async function loadSources() {
  const data = await apiJSON('/rest/sources');
  DATA.sources = (data.sources || []).map(mapSource);
}

async function loadOperators() {
  const data = await apiJSON('/rest/operators');
  DATA.operatorDict = (data.items || []).map(o => ({
    id: o.id,
    group: o.name,
    raw: o.aliases || [],
    unmapped: o.unmapped || [],
  }));
  DATA.operatorDict.sort((a, b) => (a.group === 'ДРУГИЕ') - (b.group === 'ДРУГИЕ'));
}

async function loadBitrixMap() {
  DATA.bitrixMap = await apiJSON('/rest/bitrix/lead-map');
}

const EXTRA_SDMP_BITRIX_FIELDS = [
  { key: 'web_id', title: 'Web ID', group: 'CDP' },
  { key: 'zarya', title: 'Проверка по Заре', group: 'CDP' },
  { key: 'zarya_old', title: 'Старая проверка на дубль', group: 'CDP' },
  { key: 'source_description', title: 'Дополнительно об источнике', group: 'CDP' },
  { key: 'call_url', title: 'Ссылка на запись звонка', group: 'CDP' },
];

function bitrixMapState() {
  const state = DATA.bitrixMap || { sdmp_fields: [], skorozvon_fields: [], bitrix_fields: [], statuses: [], sources: [], maps: [] };
  if (!state.sdmp_fields) state.sdmp_fields = [];
  const have = new Set(state.sdmp_fields.map(f => f.key));
  EXTRA_SDMP_BITRIX_FIELDS.forEach(f => {
    if (!have.has(f.key)) state.sdmp_fields.push(f);
  });
  return state;
}

function bitrixMapCode(key) {
  const row = (bitrixMapState().maps || []).find(m => m.source_key === key);
  return row ? (row.bitrix_code || '') : '';
}

function bitrixMapFixed(key) {
  const row = (bitrixMapState().maps || []).find(m => m.source_key === key);
  return row ? (row.fixed_value || '') : '';
}

function ensureBitrixMapRow(key) {
  const state = bitrixMapState();
  if (!state.maps) state.maps = [];
  let row = state.maps.find(m => m.source_key === key);
  if (!row) {
    row = { source_key: key, bitrix_code: '', fixed_value: '' };
    state.maps.push(row);
  }
  return row;
}

function bitrixFieldOptions() {
  const fields = (bitrixMapState().bitrix_fields || []).filter(f => f.writable);
  return [{ value: '', label: '— не отправлять' }].concat(fields.map(f => ({
    value: f.code,
    label: (f.title || f.code) + ' · ' + f.code,
  })));
}

function bitrixStatusOptions() {
  return [{ value: '', label: '— не задана' }].concat((bitrixMapState().statuses || []).map(s => ({
    value: s.status_id,
    label: (s.name || s.status_id) + ' · ' + s.status_id,
  })));
}

function bitrixFieldLabel(code) {
  if (!code) return '';
  const f = (bitrixMapState().bitrix_fields || []).find(x => x.code === code);
  return f ? ((f.title || f.code) + ' · ' + f.code) : code;
}

function bitrixStatusLabel(id) {
  if (!id) return '';
  const s = (bitrixMapState().statuses || []).find(x => x.status_id === id);
  return s ? ((s.name || s.status_id) + ' · ' + s.status_id) : id;
}

function bitrixFieldsRenderer() {
  const state = bitrixMapState();
  const emptyCatalog = !(state.bitrix_fields || []).length;
  const sdmpRows = (state.sdmp_fields || []).map((f, i) => {
    const comboId = 'bx-sdmp-' + f.key;
    const extra = f.key === 'status_id'
      ? `<div style="margin-top:6px;">${comboHTML('bx-status-value', bitrixStatusLabel(bitrixMapFixed('status_id')), 'Стадия crm.status.list', 420)}</div>`
      : '';
    return `<tr>
      <td>
        <div style="font-weight:600;">${escAttr(f.title)}</div>
        <div style="font-size:11px; color:var(--text-muted);">${escAttr(f.group)} · ${escAttr(f.key)}</div>
      </td>
      <td>${comboHTML(comboId, bitrixFieldLabel(bitrixMapCode(f.key)), 'Поле Битрикс', 420)}${extra}</td>
    </tr>`;
  }).join('');
  const skRows = (state.skorozvon_fields || []).map(f => {
    const comboId = 'bx-sk-' + f.field_id;
    return `<tr>
      <td>
        <div style="font-weight:600;">${escAttr(f.name || ('FIELD_' + f.field_id))}</div>
        <div style="font-size:11px; color:var(--text-muted);">Скорозвон · лид · FIELD_${escAttr(f.field_id)}${f.field_type ? ' · ' + escAttr(f.field_type) : ''}</div>
      </td>
      <td>${comboHTML(comboId, bitrixFieldLabel(f.bitrix_code || ''), 'Поле Битрикс', 420)}</td>
    </tr>`;
  }).join('');
  return {
    customHTML: `
      <div class="incoming-toolbar" style="margin-bottom:12px; display:flex; gap:8px; flex-wrap:wrap;">
        <button class="btn" onclick="refreshBitrixMap()">Обновить из Битрикс</button>
        <button class="btn btn-primary" onclick="saveBitrixMap()">Сохранить связку</button>
      </div>
      ${emptyCatalog ? '<p style="font-size:13px; color:var(--text-muted);">Справочник полей пуст. Нажмите «Обновить из Битрикс» — подтянутся crm.lead.fields, crm.status.list и crm.source.list.</p>' : ''}
      <table>
        <thead><tr><th>Поле CDP / Скорозвон</th><th>Поле лида в Битрикс</th></tr></thead>
        <tbody>
          ${sdmpRows}
          ${skRows || ''}
        </tbody>
      </table>
    `,
    addFn: null,
    note: 'Worker при создании лида в Б24 берёт эту таблицу. Поля Скорозвона пишутся в skorozvon_maps.bitrix_code, системные — в bitrix_lead_map. Стадия — фиксированное значение STATUS_ID.'
  };
}

function bindBitrixMapCombos() {
  const state = bitrixMapState();
  const fieldOpts = bitrixFieldOptions();
  (state.sdmp_fields || []).forEach(f => {
    registerCombo('bx-sdmp-' + f.key, fieldOpts, value => {
      ensureBitrixMapRow(f.key).bitrix_code = value;
      renderPanels();
    });
  });
  registerCombo('bx-status-value', bitrixStatusOptions(), value => {
    ensureBitrixMapRow('status_id').fixed_value = value;
    if (!bitrixMapCode('status_id')) ensureBitrixMapRow('status_id').bitrix_code = 'STATUS_ID';
    renderPanels();
  });
  (state.skorozvon_fields || []).forEach(f => {
    registerCombo('bx-sk-' + f.field_id, fieldOpts, value => {
      f.bitrix_code = value;
      renderPanels();
    });
  });
}

async function refreshBitrixMap() {
  try {
    DATA.bitrixMap = await apiJSON('/rest/bitrix/lead-map/refresh', { method: 'POST' });
    fetched.bitrixMap = true;
    renderPanels();
    showToast(activeTab);
  } catch (e) {
    showError(e.message || e);
  }
}

async function saveBitrixMap() {
  const state = bitrixMapState();
  try {
    DATA.bitrixMap = await apiJSON('/rest/bitrix/lead-map', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        maps: state.maps || [],
        skorozvon_fields: (state.skorozvon_fields || []).map(f => ({
          field_id: f.field_id,
          bitrix_code: f.bitrix_code || '',
        })),
      }),
    });
    fetched.bitrixMap = true;
    renderPanels();
    showToast(activeTab);
  } catch (e) {
    showError(e.message || e);
  }
}

function mapPartner(p) {
  return {
    id: p.id,
    nazvanie: p.name,
    webhook_url: p.webhook_url || '',
    aktiven: p.is_active !== false,
    zarya_results: p.zarya_results || [],
    region_ids: p.region_ids || [],
    stop_source_ids: p.stop_source_ids || [],
    last_date_request: p.last_date_request || null,
    last_request: p.last_request,
    last_http_status: p.last_http_status,
    last_response: p.last_response || '',
    body: p.body && typeof p.body === 'object' ? p.body : {},
    content_type: p.content_type || 'application/x-www-form-urlencoded',
  };
}

function partnerPayload(u) {
  return {
    name: u.nazvanie,
    webhook_url: u.webhook_url || '',
    is_active: !!u.aktiven,
    zarya_results: u.zarya_results || [],
    region_ids: u.region_ids || [],
    stop_source_ids: u.stop_source_ids || [],
    body: u.body || {},
    content_type: u.content_type || 'application/x-www-form-urlencoded',
  };
}

async function loadPartners() {
  const data = await apiJSON('/rest/partners');
  DATA.upk = (data.items || []).map(mapPartner);
}

async function loadRegions() {
  const data = await apiJSON('/rest/regions');
  DATA.regions = (data.items || []).map(r => ({ id: r.id, name: r.name }));
}

const SKOROZVON_PROJECT_PAGE = 5;
const hiddenSkorozvonProjectIDs = new Set();
let skorozvonProjectOffset = 0;
let skorozvonProjectName = '';
let skorozvonProjectTotal = 0;
let skorozvonProjectNameTimer = 0;
let skorozvonProjectChecking = false;

function mapSkorozvonProject(p) {
  const id = p.uuid;
  return {
    id,
    uuid: id,
    nazvanie: p.name,
    project_id: p.project_id,
    visible: !hiddenSkorozvonProjectIDs.has(id),
    created_at: p.created_at,
    last_date_request: p.last_date_request,
    last_request: p.last_request,
  };
}

async function fetchSkorozvonProjectPage(offset, name) {
  const params = new URLSearchParams();
  params.set('offset', String(offset));
  if (name) params.set('name', name);
  return apiJSON('/rest/skorozvon/projects/list?' + params.toString());
}

async function loadSkorozvonProjects() {
  const data = await fetchSkorozvonProjectPage(skorozvonProjectOffset, skorozvonProjectName.trim());
  DATA.skorozvonProjects = (data.items || []).map(mapSkorozvonProject);
  skorozvonProjectTotal = data.total || 0;
  if (skorozvonProjectOffset > 0 && DATA.skorozvonProjects.length === 0 && skorozvonProjectTotal > 0) {
    skorozvonProjectOffset = Math.max(0, Math.floor((skorozvonProjectTotal - 1) / SKOROZVON_PROJECT_PAGE) * SKOROZVON_PROJECT_PAGE);
    return loadSkorozvonProjects();
  }
}

async function loadAllSkorozvonProjects() {
  const all = [];
  for (let offset = 0, guard = 0; guard < 200; offset += SKOROZVON_PROJECT_PAGE, guard++) {
    const data = await fetchSkorozvonProjectPage(offset, '');
    const items = data.items || [];
    all.push(...items.map(mapSkorozvonProject));
    if (all.length >= (data.total || 0) || items.length === 0) break;
  }
  DATA.skorozvonProjectsAll = all;
}

function skorozvonProjectCatalog() {
  return (DATA.skorozvonProjectsAll && DATA.skorozvonProjectsAll.length)
    ? DATA.skorozvonProjectsAll
    : (DATA.skorozvonProjects || []);
}

function mapSvyazka(s, provider) {
  return {
    id: s.id,
    provider: provider || s.provider || 'skorozvon',
    nazvanie: s.name,
    pod_source_ids: s.child_source_ids || [],
    region_ids: s.region_ids || [],
    project_id: s.project_id,
    type: s.type,
  };
}

function svyazkaProvider(s) {
  return (s && s.provider) === 'crmcalls' ? 'crmcalls' : 'skorozvon';
}

function svyazkaProviderLabel(s) {
  return svyazkaProvider(s) === 'crmcalls' ? 'CRMCalls' : 'Скорозвон';
}

function svyazkaApiPath(s) {
  return svyazkaProvider(s) === 'crmcalls' ? '/rest/crmcalls/links' : '/rest/skorozvon/links';
}

function svyazkaPayload(s) {
  const payload = {
    name: s.nazvanie,
    type: s.type,
    child_source_ids: s.pod_source_ids || [],
    region_ids: s.region_ids || [],
  };
  if (svyazkaProvider(s) === 'crmcalls') {
    payload.project_id = String(s.project_id || '');
  } else {
    payload.project_id = Number(s.project_id);
  }
  return payload;
}

function findSvyazka(provider, id) {
  return DATA.svyazki.find(x => svyazkaProvider(x) === (provider || 'skorozvon') && Number(x.id) === Number(id));
}

async function loadSvyazki() {
  const [sk, cc] = await Promise.all([
    apiJSON('/rest/skorozvon/links'),
    apiJSON('/rest/crmcalls/links').catch(() => ({ items: [] })),
  ]);
  DATA.svyazki = (sk.items || []).map(s => mapSvyazka(s, 'skorozvon'))
    .concat((cc.items || []).map(s => mapSvyazka(s, 'crmcalls')));
}

async function loadCrmcallsProjects() {
  try {
    const data = await apiJSON('/rest/crmcalls/projects');
    DATA.crmcallsProjects = data.items || data || [];
    if (!Array.isArray(DATA.crmcallsProjects)) DATA.crmcallsProjects = [];
  } catch (_) {
    DATA.crmcallsProjects = [];
  }
}

function crmcallsProjectCatalog() {
  return DATA.crmcallsProjects || [];
}

let crmcallsKeyDraft = '';
let crmcallsBaseDraft = '';
let crmcallsProjectName = '';
let crmcallsConnecting = false;

async function loadCrmcallsSettings() {
  try {
    DATA.crmcallsSettings = await apiJSON('/rest/crmcalls/settings');
  } catch (_) {
    DATA.crmcallsSettings = { has_key: false, base_url: 'https://cc.crmcalls.ru', api_key_masked: '', project_count: 0 };
  }
  if (!DATA.crmcallsSettings) DATA.crmcallsSettings = { has_key: false, base_url: 'https://cc.crmcalls.ru', api_key_masked: '', project_count: 0 };
  if (!crmcallsBaseDraft) crmcallsBaseDraft = DATA.crmcallsSettings.base_url || 'https://cc.crmcalls.ru';
}

function crmcallsConnection() {
  return DATA.crmcallsSettings || { has_key: false, base_url: 'https://cc.crmcalls.ru', api_key_masked: '', project_count: 0 };
}

async function reloadCrmcallsCatalog() {
  fetched.crmcallsProjects = false;
  fetched.crmcallsSettings = false;
  await Promise.all([loadCrmcallsProjects(), loadCrmcallsSettings()]);
  fetched.crmcallsProjects = true;
  fetched.crmcallsSettings = true;
}

async function connectCrmcalls() {
  const apiKey = (crmcallsKeyDraft || '').trim();
  const baseUrl = (crmcallsBaseDraft || '').trim();
  crmcallsConnecting = true;
  if (activeTab === 'outgoingTraffic') renderPanels();
  try {
    await apiJSON('/rest/crmcalls/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ api_key: apiKey, base_url: baseUrl }),
    });
    crmcallsKeyDraft = '';
    await reloadCrmcallsCatalog();
    showToast(activeTab);
    await refreshCrmcallsProjectsCheck();
  } catch (err) {
    showError(err.message || 'Не удалось подключить CRMCalls');
  } finally {
    crmcallsConnecting = false;
    if (activeTab === 'outgoingTraffic') renderPanels();
  }
}

async function syncCrmcallsProjects() {
  try {
    await apiJSON('/rest/crmcalls/projects/sync', { method: 'POST' });
    await reloadCrmcallsCatalog();
    renderPanels();
    showToast(activeTab);
    await refreshCrmcallsProjectsCheck();
  } catch (err) {
    showError(err.message || 'Не удалось синхронизировать проекты CRMCalls');
  }
}

let crmcallsProjectChecking = false;

function crmcallsCheckLabel(p) {
  if (crmcallsProjectChecking && !p.last_date_request) {
    return '<span class="status-pill">проверка\u2026</span>';
  }
  const status = String(p.status || '').toLowerCase();
  const cabinetActive = status === 'active' || status === 'running' || status === 'enabled' || status === 'online';
  if (!p.last_date_request) {
    if (!status) return '\u2014';
    return `<span class="status-pill ${cabinetActive ? 'on' : 'off'}">${cabinetActive ? 'активен' : 'не активен'}</span>`;
  }
  const ok = p.last_request === true;
  const when = fmtCheckTime(p.last_date_request);
  return `<span class="status-pill ${ok ? 'on' : 'off'}">${ok ? 'активен' : 'не активен'}</span><div style="font-size:11px; color:var(--text-muted); margin-top:3px;">${when}</div>`;
}

async function refreshCrmcallsProjectsCheck() {
  if (!(DATA.crmcallsProjects || []).length) return;
  crmcallsProjectChecking = true;
  if (activeTab === 'outgoingTraffic') renderPanels();
  try {
    const data = await apiJSON('/rest/crmcalls/projects/refresh', { method: 'POST' });
    DATA.crmcallsProjects = data.items || data || [];
    if (!Array.isArray(DATA.crmcallsProjects)) DATA.crmcallsProjects = [];
  } catch (err) {
    showError(err.message || 'Не удалось проверить проекты CRMCalls');
  } finally {
    crmcallsProjectChecking = false;
    if (activeTab === 'outgoingTraffic') renderPanels();
  }
}

async function sendCrmcallsTest(projectId) {
  const p = (DATA.crmcallsProjects || []).find(x => String(x.id) === String(projectId));
  const name = p ? (p.name || p.id) : projectId;
  showTestToast('Отправляем тестовую строку в \u00ab' + name + '\u00bb\u2026');
  try {
    const data = await apiJSON('/rest/crmcalls/projects/' + encodeURIComponent(projectId) + '/test', { method: 'POST' });
    const extra = data.contact_id ? ' (id ' + data.contact_id + ')' : '';
    showTestToast('Тест успешно отправлен \u2014 ' + name + extra);
  } catch (err) {
    showTestToast('Ошибка тестирования \u2014 ' + name + ': ' + (err.message || ''), true);
  }
}

function svyazkaProjectTitle(s) {
  const id = s && s.project_id;
  if (!id) return '';
  if (svyazkaProvider(s) === 'crmcalls') {
    const pr = crmcallsProjectCatalog().find(p => String(p.id) === String(id));
    return pr ? ((pr.name || pr.nazvanie || id) + ' (' + id + ')') : String(id);
  }
  const pr = skorozvonProjectCatalog().find(p => String(p.project_id) === String(id));
  return pr ? (pr.nazvanie + ' (' + id + ')') : String(id);
}

async function loadGoogleSheets() {
  const data = await apiJSON('/rest/google/sheets');
  DATA.googleSheets = data.items || [];
}

function googleSheetURL(spreadsheetId) {
  return 'https://docs.google.com/spreadsheets/d/' + encodeURIComponent(spreadsheetId) + '/edit';
}

function googleSheetCardHTML(s) {
  const stops = (s.stop_source_ids || []).map(sourceName);
  const regions = s.region_ids || [];
  const table = s.spreadsheet_id || '';
  return `
  <div class="partner-card">
    <div class="partner-card-head">
      <span style="font-size:15px; font-weight:600; display:flex; align-items:center; gap:6px;">${escAttr(s.name)} <span style="font-size:11px; font-weight:500; color:var(--text-muted);">Google Sheets</span></span>
      <span class="status-pill ${s.is_active !== false ? 'on' : 'off'}">${s.is_active !== false ? 'активен' : 'выключен'}</span>
    </div>
    <div class="partner-summary-row"><span class="k">Таблица</span><span class="v" style="font-family:monospace; font-size:11.5px;">${escAttr(table ? table.replace(/^https?:\/\//,'').slice(0,32)+'…' : '—')}</span></div>
    <div class="partner-summary-row"><span class="k">Принимает</span><span class="v">${zaryaSummaryLabel(s.zarya_results)}</span></div>
    <div class="partner-summary-row"><span class="k">Регион</span><span class="v">${regions.length ? regions.length + ' в списке' : 'не проверяется'}</span></div>
    <div class="partner-summary-row"><span class="k">Стоп-источники</span><span class="v">${stops.length ? stops.length : 'нет'}</span></div>
    <div class="partner-summary-row"><span class="k">Лист</span><span class="v">${escAttr(s.sheet_name || '—')}</span></div>
    <button class="btn btn-sm partner-card-action" onclick="openGoogleSheetModal(${s.id})">Настроить</button>
  </div>`;
}

let googleModalId = null;
let googleDraft = null;
let googleEditing = false;

function openGoogleSheetModal(id) {
  googleModalId = (typeof id === 'number') ? id : null;
  googleEditing = false;
  googleDraft = null;
  document.getElementById('google-overlay').classList.add('show');
  if (googleModalId === null) {
    document.getElementById('google-modal-title').textContent = 'Новый инстанс Google Sheets';
    enterGoogleSheetEditMode();
  } else {
    const s = DATA.googleSheets.find(x => x.id === googleModalId);
    document.getElementById('google-modal-title').textContent = s ? s.name : 'Google Sheets';
    renderGoogleSheetModal();
  }
}
function closeGoogleSheetModal() {
  document.getElementById('google-overlay').classList.remove('show');
  googleModalId = null; googleDraft = null; googleEditing = false;
}
let googleRegionSearch = '';
function enterGoogleSheetEditMode() {
  googleEditing = true;
  googleRegionSearch = '';
  if (googleModalId === null) {
    googleDraft = { name: '', spreadsheet_id: '', sheet_name: 'Телефоны', is_active: true, test_mode: true, zarya_results: [], regions: [], stops: [], credentials_text: '' };
  } else {
    const s = DATA.googleSheets.find(x => x.id === googleModalId);
    googleDraft = {
      name: s.name,
      spreadsheet_id: s.spreadsheet_id,
      sheet_name: s.sheet_name,
      is_active: s.is_active !== false,
      test_mode: !!s.test_mode,
      zarya_results: (s.zarya_results || []).slice(),
      regions: (s.region_ids || []).slice(),
      stops: (s.stop_source_ids || []).slice(),
      credentials_text: ''
    };
  }
  document.getElementById('google-apply-btn').textContent = googleModalId === null ? 'Создать инстанс' : 'Применить изменения';
  renderGoogleSheetModal();
}
function draftGoogleField(field, value) { googleDraft[field] = value; }
function draftGoogleZaryaToggle(value, checked) {
  if (checked) { if (!googleDraft.zarya_results.includes(value)) googleDraft.zarya_results.push(value); }
  else { googleDraft.zarya_results = googleDraft.zarya_results.filter(v => v !== value); }
  renderGoogleSheetModal();
}
function draftGoogleRegionToggle(regionId, checked) {
  regionId = parseInt(regionId, 10);
  if (checked) { if (!googleDraft.regions.includes(regionId)) googleDraft.regions.push(regionId); }
  else { googleDraft.regions = googleDraft.regions.filter(r => r !== regionId); }
  const summary = document.getElementById('google-region-summary');
  if (summary) summary.innerHTML = googleRegionSummaryHTML();
}
function draftGoogleStopAdd(sourceId) { if (!googleDraft.stops.includes(sourceId)) { googleDraft.stops.push(sourceId); renderGoogleSheetModal(); } }
function draftGoogleStopRemove(sourceId) { googleDraft.stops = googleDraft.stops.filter(s => s !== sourceId); renderGoogleSheetModal(); }
function googleRegionSummaryHTML() {
  return googleDraft.regions.length ? 'Выбрано: ' + googleDraft.regions.map(regionName).join(', ') : 'Ничего не отмечено — регион не проверяется';
}
function renderGoogleRegionListHTML() {
  const filtered = (DATA.regions || []).filter(r => r.name.toLowerCase().includes(googleRegionSearch.toLowerCase()));
  return filtered.map(r => `
    <label style="display:flex; align-items:center; gap:8px; padding:4px 0; font-size:13px; cursor:pointer;">
      <input type="checkbox" ${googleDraft.regions.includes(r.id)?'checked':''} onchange="draftGoogleRegionToggle(${r.id}, this.checked)">
      ${r.name}
    </label>`).join('') || '<div style="color:var(--text-muted); font-size:12px; padding:4px 0;">Ничего не найдено</div>';
}
function setGoogleRegionSearch(v) {
  googleRegionSearch = v;
  const list = document.getElementById('google-region-list');
  if (list) list.innerHTML = renderGoogleRegionListHTML();
}

function renderGoogleSheetModal() {
  const box = document.getElementById('google-modal-body-content');
  document.getElementById('google-modal-sub').textContent = googleEditing
    ? (googleModalId === null ? 'Создание инстанса' : 'Режим редактирования — черновик')
    : 'Просмотр';
  document.getElementById('google-edit-banner').classList.toggle('show', googleEditing && googleModalId !== null);
  document.getElementById('google-btn-edit').style.display = (!googleEditing && googleModalId !== null) ? 'inline-block' : 'none';
  document.getElementById('google-edit-actions').style.display = googleEditing ? 'flex' : 'none';
  const delBtn = document.getElementById('google-btn-delete');
  if (delBtn) delBtn.style.display = googleModalId !== null ? 'inline-block' : 'none';

  if (!googleEditing) {
    const s = DATA.googleSheets.find(x => x.id === googleModalId);
    const href = s.spreadsheet_id ? googleSheetURL(s.spreadsheet_id) : '';
    const regions = (s.region_ids || []).map(regionName);
    const stops = (s.stop_source_ids || []).map(sourceName);
    box.innerHTML = `
      <div class="partner-field"><label>Активен</label><span class="cell-static">${s.is_active !== false ? 'да' : 'нет — полностью отключён'}</span></div>
      <div class="partner-field"><label>Таблица</label>
        <span style="display:flex; align-items:center; gap:6px;">
          <span class="cell-static" style="font-family:monospace; font-size:12px; word-break:break-all;">${escAttr(s.spreadsheet_id)}</span>
          ${href ? copyIconBtn(href, 'Ссылка на таблицу') : ''}
        </span>
      </div>
      <div class="partner-field"><label>Лист</label><span class="cell-static">${escAttr(s.sheet_name)}</span></div>
      <div class="partner-field"><label>Что принимает</label><span class="cell-static">${zaryaSummaryLabel(s.zarya_results)}</span></div>
      <div class="partner-field"><label>Регион</label><span class="cell-static">${regions.length ? regions.join(', ') : 'не проверяется'}</span></div>
      <div class="partner-field"><label>Стоп-источники</label><span class="cell-static">${stops.length ? stops.join(', ') : 'нет'}</span></div>
      <div class="partner-field"><label>Сервисный аккаунт</label><span class="cell-static">${escAttr(s.client_email || '—')}</span></div>
      <div class="partner-field"><label>Ключ</label><span class="cell-static">${s.has_key ? 'загружен, в интерфейсе не показывается' : 'нет'}</span></div>
    `;
    return;
  }

  const cAddStop = 'google-add-stop';
  registerCombo(cAddStop, allSourceNamesFlat().filter(s => !googleDraft.stops.includes(s.id)).map(s => ({ value:s.id, label:s.label })), v => draftGoogleStopAdd(parseInt(v)));

  box.innerHTML = `
    <div class="partner-field">
      <label>Активен ${helpIcon('Полное отключение инстанса — строки не уходят в таблицу, независимо от региона и стоп-источников.')}</label>
      <label class="toggle"><input type="checkbox" ${googleDraft.is_active ? 'checked' : ''} onchange="draftGoogleField('is_active', this.checked)"><span class="toggle-slider"></span></label>
    </div>
    <div class="partner-field">
      <label>Название</label>
      <input class="cell-input" style="border:1px solid var(--border-strong);" value="${escAttr(googleDraft.name)}" placeholder="Например: Телефоны" onchange="draftGoogleField('name', this.value)">
    </div>
    <div class="partner-field">
      <label>Spreadsheet ID ${helpIcon('Инстанс Google: id таблицы вместо webhook покупателя.')}</label>
      <input class="cell-input" style="border:1px solid var(--border-strong); font-family:monospace; font-size:12px;" value="${escAttr(googleDraft.spreadsheet_id)}" placeholder="id из URL таблицы" onchange="draftGoogleField('spreadsheet_id', this.value)">
    </div>
    <div class="partner-field">
      <label>Имя листа</label>
      <input class="cell-input" style="border:1px solid var(--border-strong);" value="${escAttr(googleDraft.sheet_name)}" onchange="draftGoogleField('sheet_name', this.value)">
    </div>
    <div class="partner-field">
      <label>Что принимает ${helpIcon('Результат проверки по Заре. Можно отметить несколько вариантов сразу.')}</label>
      <div class="tag-list">
        ${ZARYA_OPTIONS.map(([v,label]) => `
          <label style="display:inline-flex; align-items:center; gap:4px; font-size:12.5px; background:#f0efeb; border:1px solid var(--border-strong); border-radius:14px; padding:3px 10px 3px 6px;">
            <input type="checkbox" ${googleDraft.zarya_results.includes(v)?'checked':''} onchange="draftGoogleZaryaToggle('${v}', this.checked)">
            ${label}
          </label>`).join('')}
      </div>
      ${googleDraft.zarya_results.length === 0 ? '<div style="font-size:11px; color:var(--danger); margin-top:4px;">Не выбрано ни одного значения — в эту таблицу ничего не будет уходить.</div>' : ''}
    </div>
    <div class="partner-field">
      <label>Регион ${helpIcon('Сужает список разрешённых регионов. Пустой список = регион не проверяется.')}</label>
      <input type="text" class="cell-input" id="google-region-search-input" style="border:1px solid var(--border-strong); margin-bottom:6px;" placeholder="Поиск региона..." value="${googleRegionSearch}" oninput="setGoogleRegionSearch(this.value)">
      <div id="google-region-list" style="max-height:170px; overflow-y:auto; border:1px solid var(--border); border-radius:6px; padding:6px 10px;">${renderGoogleRegionListHTML()}</div>
      <div id="google-region-summary" style="font-size:11.5px; color:var(--text-muted); margin-top:5px;">${googleRegionSummaryHTML()}</div>
    </div>
    <div class="partner-field">
      <label>Стоп-источники ${helpIcon('Строки этого источника не уходят в таблицу, даже если они дубль и регион подходит.')}</label>
      <div class="tag-list">
        ${googleDraft.stops.map(sid => `<span class="tag">${sourceName(sid)}<span class="tag-x" onclick="draftGoogleStopRemove(${sid})">&times;</span></span>`).join('') || '<span style="color:var(--text-muted); font-size:12px;">нет</span>'}
      </div>
      <div style="margin-top:6px; width:220px;">${comboHTML(cAddStop, '', '+ добавить стоп-источник')}</div>
    </div>
    <div class="partner-field">
      <label>JSON сервисного аккаунта ${googleModalId === null ? '' : '(оставьте пустым, чтобы не менять ключ)'}</label>
      <textarea class="cell-textarea" placeholder='{"type":"service_account","client_email":"...","private_key":"..."}' onchange="draftGoogleField('credentials_text', this.value)">${escAttr(googleDraft.credentials_text || '')}</textarea>
    </div>
  `;
}

function googleSheetPayload(draft) {
  const body = {
    name: draft.name,
    spreadsheet_id: draft.spreadsheet_id,
    sheet_name: draft.sheet_name,
    is_active: draft.is_active !== false,
    test_mode: !!draft.test_mode,
    zarya_results: draft.zarya_results || [],
    region_ids: draft.regions || [],
    stop_source_ids: draft.stops || [],
  };
  const raw = String(draft.credentials_text || '').trim();
  if (raw) body.credentials = JSON.parse(raw);
  return body;
}

function applyGoogleSheetDraft() {
  if (!googleDraft.name.trim()) { showError('Укажите название инстанса.'); return; }
  if (!googleDraft.spreadsheet_id.trim()) { showError('Укажите spreadsheet_id.'); return; }
  if (!googleDraft.sheet_name.trim()) { showError('Укажите имя листа.'); return; }
  if (googleModalId === null && !String(googleDraft.credentials_text || '').trim()) {
    showError('Вставьте JSON сервисного аккаунта.');
    return;
  }
  let body;
  try {
    body = googleSheetPayload(googleDraft);
  } catch (err) {
    showError('JSON ключа не разбирается: ' + (err.message || err));
    return;
  }
  const isNew = googleModalId === null;
  customConfirm(isNew ? `Создать инстанс «${googleDraft.name}»?` : `Сохранить инстанс «${googleDraft.name}»?`, async (ok) => {
    if (!ok) return;
    try {
      if (isNew) {
        await apiJSON('/rest/google/sheets', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
      } else {
        await apiJSON('/rest/google/sheets/' + googleModalId, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(body),
        });
      }
      fetched.googleSheets = false;
      await loadGoogleSheets();
      fetched.googleSheets = true;
      closeGoogleSheetModal();
      renderPanels();
      showToast(activeTab);
    } catch (err) {
      showError(err.message || 'Не удалось сохранить инстанс');
    }
  });
}

function confirmDeleteGoogleSheet(id) {
  const s = DATA.googleSheets.find(x => x.id === id);
  if (!s) return;
  confirmChange(
    `Удалить инстанс «${s.name}»? Ключ и привязка к таблице пропадут из CDP.`,
    async () => {
      await apiJSON('/rest/google/sheets/' + id, { method: 'DELETE' });
      DATA.googleSheets = DATA.googleSheets.filter(x => x.id !== id);
      if (googleModalId === id) closeGoogleSheetModal();
    }
  );
}

function operatorName(id) {
  const o = DATA.operatorDict.find(x => x.id === id);
  return o ? o.group : '#' + id;
}

function mapSplitRule(cfg, r) {
  return {
    id: r.id,
    source_id: cfg.source_id,
    operator_id: r.operator_id,
    operator_group: operatorName(r.operator_id),
    pod_source_id: r.child_source_id,
    test_pod_source_id: r.test_child_source_id || null,
    metka_holodnaya: r.cold_label || '',
    metka_kakaya: r.base_label || '',
  };
}

async function loadSplits() {
  const data = await apiJSON('/rest/splits');
  DATA.droblenie = [];
  (data.items || []).forEach(cfg => {
    const src = DATA.sources.find(s => s.id === cfg.source_id);
    if (src) {
      src.teg_active = !!cfg.teg_active;
      src.teg_expected_value = cfg.teg_expected_value || '';
    }
    (cfg.rules || []).forEach(r => {
      DATA.droblenie.push(mapSplitRule(cfg, r));
      const ch = DATA.sources.find(s => s.id === r.child_source_id);
      if (ch) {
        ch.metka_holodnaya = r.cold_label || '';
        ch.metka_kakaya = r.base_label || '';
      }
    });
  });
}

function regionName(id) {
  const r = (DATA.regions || []).find(x => x.id === id);
  return r ? r.name : '#' + id;
}

async function createChildSource(parentId, name) {
  return apiJSON('/rest/sources/' + parentId + '/children', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
}

async function createParentSource(name) {
  return apiJSON('/rest/sources', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ name }),
  });
}

/* ===== СПРАВОЧНИКИ (редактируемые, с маппингом сырых значений) ===== */
let DATA = {
  sources: [],
  droblenie: [],
  sourceUpk: [
    { id:1, source_id:1, upk:'РТ', razreshen:true },
    { id:2, source_id:1, upk:'КР', razreshen:true },
    { id:3, source_id:1, upk:'ББ', razreshen:true },
    { id:4, source_id:9, upk:'КР', razreshen:false },
    { id:5, source_id:5, upk:'ББ', razreshen:true },
    { id:6, source_id:9, upk:'ББ', razreshen:false },
  ],
  svyazki: [],
  sourceSkorozvon: [
    { id:1, source_id:1, project_id:'proj-2001' },
    { id:2, source_id:2, project_id:'proj-2010' },
    { id:3, source_id:3, project_id:'proj-2011' },
    { id:4, source_id:5, project_id:'proj-2020' },
  ],
  upk: [],
  regions: [],
  skorozvonProjects: [],
  skorozvonProjectsAll: [],
  crmcallsProjects: [],
  crmcallsSettings: { has_key: false, base_url: 'https://cc.crmcalls.ru', api_key_masked: '', project_count: 0 },
  mediaplans: [
    { id:1, upk:'КР', region:'Московская область' },
    { id:2, upk:'КР', region:'Краснодарский край' },
    { id:3, upk:'ББ', region:'Свердловская область' },
    { id:4, upk:'ББ', region:'Республика Татарстан' },
  ],
  operatorDict: [],
  bitrixMap: null,
  googleSheets: [],
  regionDict: [
    { id:1, canonical:'Московская область', raw:['Москва','Москва и Московская область','Московская обл.','Московская область * Москва'] },
    { id:2, canonical:'Кемеровская область', raw:['Кемеровская обл.','Кемеровская область - Кузбасс'] },
    { id:3, canonical:'Нижегородская область', raw:['Нижегородская обл.','Нижегородская область'] },
    { id:4, canonical:'Свердловская область', raw:['Свердловская обл.','Свердловская область'] },
    { id:5, canonical:'Ленинградская область', raw:['г. Санкт-Петербург','г. Санкт-Петербург и Ленинградская область'] },
    { id:6, canonical:'Краснодарский край', raw:[] },
    { id:7, canonical:'Республика Татарстан', raw:[] },
    { id:8, canonical:'Пермский край', raw:[] },
  ],
};
const UPK_LIST = ['РТ','КР','ББ'];
const ZARYA_RESULTS_ALL = ['ДЮ','ДС менее 90','ДС более 90','Ошибка проверки'];
const ZARYA_OPTIONS = [['dup','Дубль'], ['unique','Уникальный']];
function zaryaSummaryLabel(values) {
  values = values || [];
  if (values.length === 2) return 'дубль и уник';
  return values.map(v => (ZARYA_OPTIONS.find(o => o[0]===v)||[,v])[1]).join(', ') || 'ничего';
}
let nextId = { sources:100, droblenie:100, sourceUpk:100, sourceSkorozvon:100, upk:100, mediaplans:100, operatorDict:100, regionDict:100, svyazki:100, skorozvonProjects:100 };
let skorozvonHookActive = true;
function setSkorozvonHookActive(checked) { skorozvonHookActive = checked; showToast(activeTab); }
function skorozvonProjectPagerInfo() {
  if (!skorozvonProjectTotal) return '0 из 0';
  const from = skorozvonProjectOffset + 1;
  const to = Math.min(skorozvonProjectOffset + SKOROZVON_PROJECT_PAGE, skorozvonProjectTotal);
  return from + '\u2013' + to + ' из ' + skorozvonProjectTotal;
}

async function reloadSkorozvonProjectPage() {
  fetched.skorozvonProjects = false;
  await loadSkorozvonProjects();
  fetched.skorozvonProjects = true;
}

async function checkCurrentSkorozvonProjects() {
  const uuids = (DATA.skorozvonProjects || []).map(p => p.uuid || p.id).filter(Boolean);
  if (!uuids.length) return;
  skorozvonProjectChecking = true;
  if (activeTab === 'outgoingTraffic') renderPanels();
  try {
    await apiJSON('/rest/skorozvon/projects/refresh', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ uuids }),
    });
    await reloadSkorozvonProjectPage();
  } finally {
    skorozvonProjectChecking = false;
    if (activeTab === 'outgoingTraffic') renderPanels();
  }
}

async function shiftSkorozvonProjectPage(delta) {
  const next = Math.max(0, skorozvonProjectOffset + delta);
  if (next === skorozvonProjectOffset) return;
  skorozvonProjectOffset = next;
  try {
    await reloadSkorozvonProjectPage();
    renderPanels();
    await checkCurrentSkorozvonProjects();
  } catch (err) {
    showError(err.message || 'Не удалось загрузить проекты');
  }
}

async function applySkorozvonProjectSearch() {
  clearTimeout(skorozvonProjectNameTimer);
  const input = document.getElementById('skorozvon-project-search');
  if (input) skorozvonProjectName = input.value;
  skorozvonProjectOffset = 0;
  try {
    await reloadSkorozvonProjectPage();
    renderPanels();
    await checkCurrentSkorozvonProjects();
  } catch (err) {
    showError(err.message || 'Не удалось найти проекты');
  }
}

function onSkorozvonProjectNameInput(value) {
  skorozvonProjectName = value;
  clearTimeout(skorozvonProjectNameTimer);
  skorozvonProjectNameTimer = setTimeout(() => { applySkorozvonProjectSearch(); }, 300);
}

async function syncSkorozvonProjects() {
  try {
    await apiJSON('/rest/skorozvon/projects/sync', { method: 'POST' });
    fetched.skorozvonProjectsAll = false;
    await reloadSkorozvonProjectPage();
        renderPanels();
        showToast(activeTab);
      } catch (err) {
    showError(err.message || 'Не удалось синхронизировать проекты');
      }
}

function rootSources() { return DATA.sources.filter(s => s.roditel_id === null); }
function childrenOf(sourceId) { return DATA.sources.filter(s => s.roditel_id === sourceId); }
function childBinding(id) {
  const inSplit = (DATA.droblenie || []).some(d => d.pod_source_id === id || d.test_pod_source_id === id);
  if (inSplit) return { cls: 'on', text: 'в дроблении' };
  const inLink = (DATA.svyazki || []).some(s => (s.pod_source_ids || []).includes(id));
  if (inLink) return { cls: 'on', text: 'в связке' };
  return { cls: 'off', text: 'без правил' };
}
function sourceName(id) { const s = DATA.sources.find(x => x.id === id); return s ? s.nazvanie : '—'; }
function allSourceNamesFlat() { return DATA.sources.map(s => ({ id:s.id, label: s.roditel_id ? '↳ ' + s.nazvanie : s.nazvanie })); }
function operatorGroups() { return DATA.operatorDict.map(d => d.group); }
function helpIcon(text) { return `<span class="help-icon" title="${text.replace(/"/g,'&quot;')}">?</span>`; }
function regionCanonicalList() { return DATA.regionDict.map(d => d.canonical); }

/* ===== ВКЛАДКИ ===== */
const TABS = [
  { id:'droblenie', label:'Дробление', section:'routing', impact:'Настройка по каждому источнику — в отдельном окне (кнопка «Настроить»). Пока идёт редактирование, в проде продолжают действовать старые правила; новые вступают в силу только после «Применить изменения». Влияет на определение под-источника (шаг 3 Worker) и метки (шаг 5).' },
  { id:'partners', label:'Отправка партнёрам', section:'routing', impact:'Каждый блок — самостоятельный набор условий одного покупателя (webhook, регион, стоп-источники). Google Sheets здесь же: отдельная карточка таблицы. Ошибка ответа Зари никуда не уходит. Изменения влияют на шаг 6 Worker.' },
  { id:'dupWork', label:'Работа с дублями', section:'routing', impact:'Изменения влияют на: дополнительную отправку в проект Скорозвона для строк-дублей, по региону. Работает параллельно с обычным обзвоном и с отправкой партнёрам.' },
  { id:'uniqWork', label:'Работа с униками', section:'routing', impact:'Изменения влияют на: дополнительную отправку в проект Скорозвона для уникальных строк, по региону. Работает параллельно с обычным обзвоном.' },
  { id:'sources', label:'Источники', section:'dict', impact:'Создание и удаление источников и под-источников (привязанных к родителю). Токен, вкл/выкл приёма и статистика — на вкладке «Интеграции → Входящий трафик». Метки — на вкладке «Дробление».' },
  { id:'operatorDict', label:'Операторы', section:'dict', impact:'Изменения влияют на: как сырое значение оператора от поставщика группируется при обработке строки (шаг 2 Worker), и на выбор группы во вкладке «Дробление». Значение, не найденное ни в одной группе, автоматически попадает в «ДРУГИЕ».', warn:'Изменение состава группы задним числом не пересчитывает уже принятые строки — только новые.' },
  { id:'bitrixFields', label:'Связка полей (Битрикс)', section:'dict', impact:'Какое поле CDP или Скорозвона уходит в какое поле лида Битрикс при crm.lead.add (шаг 11 Worker). Стадия берётся из справочника crm.status.list. После «Сохранить» применяется к новым отправкам.' },
  { id:'incomingTraffic', label:'Входящий трафик', section:'integrations', impact:'Токен привязывается к корневому источнику и передаётся поставщику для настройки интеграции. Изменение или отключение сразу влияет на приём строк (Метод 1) — отключённый источник отвечает 403 Forbidden.' },
  { id:'outgoingTraffic', label:'Исходящий трафик', section:'integrations', impact:'Журнал того, что реально ушло партнёрам, в Скорозвон и CRMCalls. Проекты CRMCalls подключаются и подтягиваются здесь. Маршруты — на вкладках «Отправка партнёрам», «Работа с дублями», «Работа с униками».' },
  { id:'skorozvonBitrix', label:'Скорозвон → Битрикс', section:'integrations', impact:'Сколько строк обновилось после webhook от Скорозвона (шаг 10 Worker, статус 4) и сколько из них успешно передано в Битрикс (шаг 11 Worker, статус 5). Только просмотр.' },
  { id:'regionControl', label:'Регион', section:'enrichment', impact:'Контроль качества определения региона по источникам: как часто регион определяется от поставщика / Россети / не определяется. Только просмотр.' },
  { id:'operatorControl', label:'Оператор', section:'enrichment', impact:'Контроль качества определения оператора по источникам: от поставщика / Россети / не определён (ушло в «Другие»). Только просмотр.' },
  { id:'zaryaControl', label:'Дедупликация (Заря)', section:'enrichment', impact:'Контроль доли дублей и уников по источникам (результат проверки по Заре, свёрнут до 2 значений). Только просмотр.' },
];
const SECTIONS = [
  { id:'dict', label:'Справочники', path:'guide' },
  { id:'integrations', label:'Интеграции', path:'integrations' },
  { id:'enrichment', label:'Обогащение', path:'analytics' },
  { id:'routing', label:'Маршрутизация', path:'routing' },
];
const TAB_PATH = {
  sources: 'sources',
  operatorDict: 'operators',
  bitrixFields: 'bitrix',
  incomingTraffic: 'in',
  outgoingTraffic: 'out',
  skorozvonBitrix: 'mapping',
  regionControl: 'regions',
  operatorControl: 'operators',
  zaryaControl: 'dedup',
  droblenie: 'crushing',
  partners: 'partners',
  dupWork: 'duplicate',
  uniqWork: 'unique',
};
const SECTION_BY_PATH = Object.fromEntries(SECTIONS.map(s => [s.path, s.id]));
const DEFAULT_TAB = 'sources';
const SETTINGS_BASE = '/settings';

let activeSection = 'dict';
let activeTab = DEFAULT_TAB;
let syncingUrl = false;

function sectionPath(sectionId) {
  const s = SECTIONS.find(x => x.id === sectionId);
  return s ? s.path : 'guide';
}
function tabPath(tabId) {
  return TAB_PATH[tabId] || tabId;
}
function settingsHref(sectionId, tabId) {
  return SETTINGS_BASE + '/' + sectionPath(sectionId) + '/' + tabPath(tabId);
}
function firstTabInSection(sectionId) {
  return TABS.find(t => t.section === sectionId) || TABS.find(t => t.id === DEFAULT_TAB);
}
function resolveRoute(pathname) {
  const parts = String(pathname || '').replace(/\/+$/, '').split('/').filter(Boolean);
  // expected: settings / {section} / {tab}
  if (parts[0] !== 'settings') {
    return { sectionId: 'dict', tabId: DEFAULT_TAB, canonical: settingsHref('dict', DEFAULT_TAB) };
  }
  if (parts.length === 1) {
    return { sectionId: 'dict', tabId: DEFAULT_TAB, canonical: settingsHref('dict', DEFAULT_TAB) };
  }
  const sectionId = SECTION_BY_PATH[parts[1]] || null;
  if (!sectionId) {
    return { sectionId: 'dict', tabId: DEFAULT_TAB, canonical: settingsHref('dict', DEFAULT_TAB) };
  }
  if (parts.length === 2) {
    const tab = firstTabInSection(sectionId);
    return { sectionId, tabId: tab.id, canonical: settingsHref(sectionId, tab.id) };
  }
  const tabSlug = parts[2];
  const tab = TABS.find(t => t.section === sectionId && TAB_PATH[t.id] === tabSlug);
  if (!tab) {
    const fallback = firstTabInSection(sectionId);
    return { sectionId, tabId: fallback.id, canonical: settingsHref(sectionId, fallback.id) };
  }
  return { sectionId, tabId: tab.id, canonical: settingsHref(sectionId, tab.id) };
}
function syncSettingsUrl(tabId, { replace = false } = {}) {
  const tab = TABS.find(t => t.id === tabId);
  if (!tab) return;
  const href = settingsHref(tab.section, tab.id);
  const current = location.pathname.replace(/\/+$/, '') || '/';
  const target = href.replace(/\/+$/, '');
  if (current === target) {
    if (replace) history.replaceState({ tabId }, '', href);
    return;
  }
  if (replace) history.replaceState({ tabId }, '', href);
  else history.pushState({ tabId }, '', href);
}

function switchSection(id) {
  const tab = firstTabInSection(id);
  showTab(tab.id, { pushUrl: true });
}

function renderTabs() {
  const tabsInSection = TABS.filter(t => t.section === activeSection);
  document.getElementById('section-switch').innerHTML = SECTIONS.map(s => {
    const tab = firstTabInSection(s.id);
    return `<a class="section-btn ${s.id===activeSection?'active':''}" href="${settingsHref(s.id, tab.id)}" data-section="${s.id}">${s.label}</a>`;
  }).join('');
  document.getElementById('tabs').innerHTML = tabsInSection.map(t =>
    `<a class="tab ${t.id===activeTab?'active':''}" href="${settingsHref(t.section, t.id)}" data-tab="${t.id}">${t.label}</a>`
  ).join('');
}
function switchTab(id) { showTab(id, { pushUrl: true }); }

const TAB_LOAD = {
  sources: ['sources'],
  incomingTraffic: ['sources'],
  operatorDict: ['operators'],
  bitrixFields: ['bitrixMap'],
  partners: ['sources', 'regions', 'partners', 'googleSheets'],
  droblenie: ['sources', 'operators', 'splits'],
  outgoingTraffic: ['skorozvonProjects', 'partners', 'crmcallsProjects', 'crmcallsSettings'],
  dupWork: ['sources', 'regions', 'skorozvonProjectsAll', 'crmcallsProjects', 'svyazki'],
  uniqWork: ['sources', 'regions', 'skorozvonProjectsAll', 'crmcallsProjects', 'svyazki'],
  skorozvonBitrix: ['sources'],
  regionControl: ['sources'],
  operatorControl: ['sources'],
  zaryaControl: ['sources'],
};
const TAB_PREFETCH = {
  sources: ['operators', 'splits', 'svyazki'],
};
const LOADERS = {
  sources: loadSources,
  operators: loadOperators,
  partners: loadPartners,
  regions: loadRegions,
  skorozvonProjects: loadSkorozvonProjects,
  skorozvonProjectsAll: loadAllSkorozvonProjects,
  crmcallsProjects: loadCrmcallsProjects,
  crmcallsSettings: loadCrmcallsSettings,
  svyazki: loadSvyazki,
  splits: loadSplits,
  bitrixMap: loadBitrixMap,
  googleSheets: loadGoogleSheets,
};
const fetched = {};
const inflight = {};
let tabLoadGen = 0;
let tabLoading = false;

function loadNamed(name) {
  if (fetched[name]) return Promise.resolve();
  if (inflight[name]) return inflight[name];
  inflight[name] = LOADERS[name]()
    .then(() => { fetched[name] = true; })
    .finally(() => { delete inflight[name]; });
  return inflight[name];
}

async function ensureTabData(tabId) {
  await Promise.all((TAB_LOAD[tabId] || []).map(loadNamed));
}

function prefetchTabData(tabId) {
  (TAB_PREFETCH[tabId] || []).forEach(name => {
    loadNamed(name).then(() => {
      if (activeTab === tabId && !tabLoading) renderPanels();
    }).catch(() => {});
  });
}

function tabDataReady(tabId) {
  return (TAB_LOAD[tabId] || []).every(name => fetched[name]);
}

async function showTab(id, opts = {}) {
  const tab = TABS.find(t => t.id === id);
  if (!tab) return;
  const gen = ++tabLoadGen;
  activeTab = id;
  activeSection = tab.section;
  clearError();
  tabLoading = !tabDataReady(id);
  if (!syncingUrl) {
    if (opts.pushUrl) syncSettingsUrl(id, { replace: false });
    else if (opts.replaceUrl !== false) syncSettingsUrl(id, { replace: true });
  }
  renderTabs();
  renderPanels();
  prefetchTabData(id);
  try {
    await ensureTabData(id);
  } catch (err) {
    if (gen !== tabLoadGen) return;
    tabLoading = false;
    showError(err.message || 'Не удалось загрузить данные');
    renderPanels();
    return;
  }
  if (gen !== tabLoadGen) return;
  tabLoading = false;
  renderPanels();
  if (id === 'incomingTraffic') loadIncomingCounts();
  if (id === 'outgoingTraffic') {
    loadOutgoingStats();
    checkCurrentSkorozvonProjects();
    refreshCrmcallsProjectsCheck();
  }
  if (id === 'skorozvonBitrix') loadSkorozvonBitrixStats();
  if (id === 'regionControl') loadRegionStats();
  if (id === 'operatorControl') loadOperatorStats();
  if (id === 'zaryaControl') loadZaryaStats();
}
function showToast(id) {
  clearError();
  const t = document.getElementById('toast-' + id);
  if (!t) return;
  t.classList.add('show');
  clearTimeout(t._timer);
  t._timer = setTimeout(() => t.classList.remove('show'), 1800);
}

/* ===== КОМБОБОКС С ПОИСКОМ ===== */
let COMBO_REGISTRY = {};
function registerCombo(id, options, onSelect) { COMBO_REGISTRY[id] = { options, onSelect }; }
function comboHTML(id, currentLabel, placeholder, width) {
  return `<div class="combo" style="${width?`width:${width}px`:''}">
    <input type="text" class="cell-input" value="${currentLabel || ''}" placeholder="${placeholder||'Выбрать...'}"
      oninput="comboFilter('${id}', this.value)" onfocus="this.select(); comboOpen('${id}', this)" autocomplete="off">
    <div class="combo-list" id="combolist-${id}"></div>
  </div>`;
}
function comboRenderList(id, query) {
  const reg = COMBO_REGISTRY[id];
  const list = document.getElementById('combolist-' + id);
  if (!reg || !list) return;
  const q = (query || '').toLowerCase();
  const filtered = reg.options.filter(o => o.label.toLowerCase().includes(q));
  list.innerHTML = filtered.length
    ? filtered.map(o => `<div class="combo-item" onmousedown="comboSelect('${id}', '${String(o.value).replace(/'/g,"\\'")}')">${o.label}</div>`).join('')
    : `<div class="combo-item empty">Ничего не найдено</div>`;
}
function positionComboList(id, inputEl) {
  const list = document.getElementById('combolist-' + id);
  if (!list || !inputEl) return;
  const rect = inputEl.getBoundingClientRect();
  list.style.top = (rect.bottom + 4) + 'px';
  list.style.left = rect.left + 'px';
  list.style.width = rect.width + 'px';
}
function comboOpen(id, inputEl) {
  document.querySelectorAll('.combo-list.show').forEach(l => l.classList.remove('show'));
  comboRenderList(id, '');
  positionComboList(id, inputEl);
  document.getElementById('combolist-' + id).classList.add('show');
}
function comboFilter(id, query) {
  comboRenderList(id, query);
  const inputEl = event && event.target;
  if (inputEl) positionComboList(id, inputEl);
  document.getElementById('combolist-' + id).classList.add('show');
}
function comboSelect(id, value) {
  const reg = COMBO_REGISTRY[id];
  if (!reg) return;
  const opt = reg.options.find(o => String(o.value) === String(value));
  const list = document.getElementById('combolist-' + id);
  const input = list && list.parentElement && list.parentElement.querySelector('input');
  if (input && opt) input.value = opt.label;
  if (reg.onSelect) reg.onSelect(value);
  if (list) list.classList.remove('show');
}
document.addEventListener('click', (e) => {
  if (!e.target.closest('.combo')) document.querySelectorAll('.combo-list.show').forEach(l => l.classList.remove('show'));
  const tokenBtn = e.target.closest('[data-token-action]');
  if (tokenBtn) {
    const id = Number(tokenBtn.getAttribute('data-source-id'));
    const action = tokenBtn.getAttribute('data-token-action');
    if (action === 'generate') confirmGenerateToken(id);
    else if (action === 'refresh') confirmRegenerateToken(id);
    else if (action === 'delete') confirmDeleteToken(id);
    else if (action === 'copy') {
      const row = DATA.sources.find(s => s.id === id);
      copyText(row && row.token, 'Токен');
    }
    return;
  }
  const copyBtn = e.target.closest('[data-copy-text]');
  if (copyBtn) {
    copyText(copyBtn.getAttribute('data-copy-text'), copyBtn.getAttribute('data-copy-label') || 'Значение');
  }
});

/* ===== РЕНДЕР ВКЛАДОК ===== */
const PANEL_RENDERERS = {

  sources: () => {
    function cardHTML(root) {
      const children = childrenOf(root.id);
      const open = expandedSources.has(root.id);
      const childRows = children.map(ch => {
        const bound = childBinding(ch.id);
        return `<div class="source-child-row">
          <span class="indent-guide">↳</span>
          <span class="source-child-name">${ch.nazvanie}</span>
          <span class="status-pill ${bound.cls}" style="margin-left:auto;">${bound.text}</span>
          <button class="del-btn" onclick="confirmDeleteSource(${ch.id}, '${ch.nazvanie.replace(/'/g,"\\'")}')" title="Удалить">&times;</button>
        </div>`;
      }).join('');
      const body = children.length
        ? `<div class="source-children">${childRows}</div>`
        : `<div class="source-children-empty">Под-источников пока нет</div>`;
      return `
      <div class="source-card ${open ? 'open' : 'collapsed'}">
        <div class="source-card-head" onclick="toggleSourceExpand(${root.id})">
          <span class="source-card-title">
            <span class="source-card-chevron">${open ? '▾' : '▸'}</span>
            ${root.nazvanie}
            ${children.length ? `<span class="source-card-count">${children.length}</span>` : ''}
          </span>
          <div style="display:flex; gap:6px;" onclick="event.stopPropagation()">
            <button class="btn btn-sm" onclick="addSubSourceConfirm(${root.id})">+ под-источник</button>
            <button class="del-btn" onclick="confirmDeleteSource(${root.id}, '${root.nazvanie.replace(/'/g,"\\'")}')" title="Удалить источник">&times;</button>
          </div>
        </div>
        ${open ? body : ''}
      </div>`;
    }
    return {
      customHTML: `<div class="source-cards">${rootSources().map(cardHTML).join('')}</div>`,
      addFn:'addSourceForm', addLabel:'Добавить источник',
      note:'Под-источник создаётся кнопкой «+ под-источник» у нужного родителя и сразу привязывается к нему. Дальше он выбирается на вкладке «Дробление» — какой группе оператора соответствует. Токен и вкл/выкл приёма — на вкладке «Интеграции → Входящий трафик».'
    };
  },

  droblenie: () => {
    let rows = '';
    rootSources().forEach(root => {
      const rules = DATA.droblenie.filter(d => d.source_id === root.id);
      const children = childrenOf(root.id);
      rows += `<tr>
        <td style="font-weight:600;">${root.nazvanie}</td>
        <td>${children.length}</td>
        <td>${rules.length ? rules.map(r => `${r.operator_group} → ${sourceName(r.pod_source_id)}`).join('; ') : '<span style="color:var(--text-muted);">правил нет — без разбивки</span>'}</td>
        <td><button class="btn btn-sm" onclick="openDroblenieModal(${root.id})">Настроить</button></td>
      </tr>`;
    });
    return {
      headers:['Источник','Под-источников','Текущие правила',''], rows, addFn:null,
      note:'Настройка ведётся по каждому источнику отдельно, в режиме редактирования. Новый под-источник рождается автоматически вместе с правилом — при добавлении правила указывается только название.'
    };
  },

  partners: () => {
    function summaryCardHTML(u) {
      const stops = (u.stop_source_ids || []).map(sourceName);
      const regions = u.region_ids || [];
      return `
      <div class="partner-card">
        <div class="partner-card-head">
          <span style="font-size:15px; font-weight:600; display:flex; align-items:center; gap:6px;">${u.nazvanie} <button class="edit-pencil-btn" title="Переименовать" onclick="renamePartner(${u.id})">✎</button></span>
          <span class="status-pill ${u.aktiven ? 'on' : 'off'}">${u.aktiven ? 'активен' : 'выключен'}</span>
        </div>
        <div class="partner-summary-row"><span class="k">Webhook</span><span class="v" style="font-family:monospace; font-size:11.5px;">${u.webhook_url ? u.webhook_url.replace(/^https?:\/\//,'').slice(0,32)+'…' : '—'}</span></div>
        <div class="partner-summary-row"><span class="k">Принимает</span><span class="v">${zaryaSummaryLabel(u.zarya_results)}</span></div>
        <div class="partner-summary-row"><span class="k">Регион</span><span class="v">${regions.length ? regions.length + ' в списке' : 'не проверяется'}</span></div>
        <div class="partner-summary-row"><span class="k">Стоп-источники</span><span class="v">${stops.length ? stops.length : 'нет'}</span></div>
        <div class="partner-summary-row"><span class="k">Параметры</span><span class="v">${Object.keys(u.body || {}).length || 'нет'}</span></div>
        <div class="partner-summary-row"><span class="k">Content-Type</span><span class="v" style="font-family:monospace; font-size:11.5px;">${u.content_type || 'application/x-www-form-urlencoded'}</span></div>
        <button class="btn btn-sm partner-card-action" onclick="openPartnerModal(${u.id})">Настроить</button>
      </div>`;
    }
    const cardsHTML = DATA.upk.map(summaryCardHTML).join('');
    const googleHTML = (DATA.googleSheets || []).map(googleSheetCardHTML).join('');
    return {
      customHTML: `
        <div class="partners-lead-note">
          Все покупатели равноправны — приоритета между ними нет. Если строка подходит нескольким сразу, уходит всем одновременно (параллельно). Что принимать — дубль или уникальную строку — настраивается у каждого покупателя. Ошибка ответа Зари никуда не уходит.
        </div>
        <div class="partner-cards">${cardsHTML}${googleHTML}</div>
        <div style="margin-top:12px;">
          <button class="btn btn-sm" type="button" onclick="openGoogleSheetModal()">+ Google Sheets</button>
        </div>`,
      addFn: 'addUpk'
    };
  },

  dupWork: () => svyazkiTabRenderer('dup'),
  uniqWork: () => svyazkiTabRenderer('uniq'),

  operatorDict: () => {
    const rows = DATA.operatorDict.map(r => {
      const isFallback = r.group === 'ДРУГИЕ';
      return `
      <tr${isFallback ? ' style="background:#fdf8ee;"' : ''}>
        <td><input class="cell-input" value="${r.group}" disabled title="Внутренняя группа из справочника"></td>
        <td>
          ${isFallback ? unmappedOperatorHTML(r) : `<div class="tag-list">
                ${r.raw.map((v,i) => `<span class="tag">${escAttr(v)}<span class="tag-x" onclick="removeOperatorAlias(${r.id}, ${i})">&times;</span></span>`).join('')}
                <input class="tag-add-input" placeholder="+ добавить" onkeydown="if(event.key==='Enter'){addOperatorAlias(${r.id}, this.value).catch(e=>showError(e.message||e)); this.value='';}">
              </div>`}
        </td>
        <td class="col-actions"></td>
      </tr>`;
    }).join('');

    return {
      customHTML: `
        <table>
          <thead><tr><th>Внутренняя группа (дробление)</th><th>Верхний маппинг — сырые значения поставщика и определителя</th><th></th></tr></thead>
          <tbody>${rows}</tbody>
        </table>
      `,
      addFn:null,
      note:'Слева — внутренние группы из БД. Справа — верхний маппинг: сырые названия поставщика и определителя. Не найденное значение уходит в «ДРУГИЕ»; такие сырые строки показываются там, чтобы добавить их в нужную группу.'
    };
  },

  bitrixFields: () => bitrixFieldsRenderer(),

  regionDict: () => {
    const rows = DATA.regionDict.map(r => `
      <tr>
        <td><input class="cell-input" value="${r.canonical}" onchange="updateField('regionDict', ${r.id}, 'canonical', this.value)"></td>
        <td>
          <div class="tag-list">
            ${r.raw.map((v,i) => `<span class="tag">${v}<span class="tag-x" onclick="removeTag('regionDict', ${r.id}, ${i})">&times;</span></span>`).join('')}
            <input class="tag-add-input" placeholder="+ добавить" onkeydown="if(event.key==='Enter'){addTag('regionDict', ${r.id}, this.value); this.value='';}">
          </div>
        </td>
        <td class="col-actions"><button class="del-btn" onclick="deleteRow('regionDict', ${r.id})">&times;</button></td>
      </tr>`).join('');
    return { headers:['Регион (стандарт)','Варианты написания от источников',''], rows, addFn:'addRegion' };
  },

  incomingTraffic: () => {
    const rows = rootSources().map(root => {
      const tokenCell = root.token
        ? `<div class="token-cell">
            <code class="token-value" title="${escAttr(receiveLeadUrl(root.token))}">${escAttr(receiveLeadUrl(root.token))}</code>
            ${iconBtn('copy', 'Скопировать URL приёма', `onTokenAction('copy', ${root.id})`)}
          </div>`
        : `<div class="token-cell">
            <span class="token-empty">нет токена</span>
            ${iconBtn('plus', 'Сгенерировать', `onTokenAction('generate', ${root.id})`, 'accent')}
          </div>`;
      return `<tr>
        <td style="font-weight:600;">${root.nazvanie}</td>
        <td style="text-align:left;">${tokenCell}</td>
        <td style="text-align:center; font-weight:600;">${incomingCountCell(root.id)}</td>
        <td><label class="toggle" title="Отключение немедленно останавливает приём — 403 Forbidden"><input type="checkbox" ${root.aktiven?'checked':''} onchange="confirmAktivenChange(${root.id}, '${root.nazvanie.replace(/'/g,"\\'")}', this.checked, this)"><span class="toggle-slider"></span></label></td>
      </tr>`;
    }).join('');
    return {
      headers:['Источник','Токен приёма','Строк за период','Активен'],
      rows, addFn:null,
      customTop: `
        <div class="incoming-toolbar">
          <div class="incoming-dates">
          <div class="field"><label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">С</label><input type="date" class="cell-input" style="border:1px solid var(--border-strong);" value="${incomingRange.from}" onchange="setIncomingRange('from', this.value)"></div>
          <div class="field"><label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">По</label><input type="date" class="cell-input" style="border:1px solid var(--border-strong);" value="${incomingRange.to}" onchange="setIncomingRange('to', this.value)"></div>
          </div>
          ${hintIconBtn('download', 'Документация', 'downloadCRMLeadDocs()')}
        </div>`,
      note:'Токен передаётся поставщику для настройки интеграции. Раздел будет дорабатываться — появятся дополнительные функции управления интеграциями.'
    };
  },

  outgoingTraffic: () => {
    const partners = DATA.upk || [];
    const projects = DATA.skorozvonProjects || [];

    const rows = partners.map(u => `
      <tr>
        <td>${fmtCheckTime(u.last_date_request)}</td>
        <td>${escAttr(u.nazvanie)}</td>
        <td>${outgoingPartnerCount(u.id)}</td>
        <td>
          ${partnerCheckLabel(u)}
          <div style="margin-top:6px;"><button class="btn btn-sm" onclick="sendPartnerTest(${u.id})">Отправить тест</button></div>
        </td>
      </tr>`).join('');

    const visibleProjects = projects.filter(p => p.visible);
    const projectRows = visibleProjects.map(p => `
      <tr>
        <td style="font-weight:600; text-align:left;">${p.nazvanie} <span style="color:var(--text-muted); font-family:monospace; font-size:11px;">(${p.project_id})</span></td>
        <td style="font-weight:600;">${outgoingSkorozvonCount(p.project_id)}</td>
        <td>${skorozvonCheckLabel(p)}</td>
        <td><button class="btn btn-sm" onclick="sendSkorozvonTest('${String(p.uuid || p.id).replace(/'/g, "\\'")}')">Отправить тест</button></td>
      </tr>`).join('');

    const cc = crmcallsConnection();
    const ccQuery = (crmcallsProjectName || '').toLowerCase();
    const ccProjects = (DATA.crmcallsProjects || []).filter(p => {
      if (!ccQuery) return true;
      return String(p.name || '').toLowerCase().includes(ccQuery) || String(p.id || '').toLowerCase().includes(ccQuery);
    });
    const ccRows = ccProjects.map(p => `
      <tr>
        <td style="font-weight:600; text-align:left;">${escAttr(p.name || p.id)} <span style="color:var(--text-muted); font-family:monospace; font-size:11px;">(${escAttr(p.id)})</span></td>
        <td style="font-weight:600;">${outgoingCrmcallsCount(p.id)}</td>
        <td>${crmcallsCheckLabel(p)}</td>
        <td><button class="btn btn-sm" onclick="sendCrmcallsTest('${String(p.id).replace(/'/g, "\\'")}')">Отправить тест</button></td>
      </tr>`).join('');
    const ccKeyPlaceholder = cc.has_key ? (cc.api_key_masked || 'ключ сохранён') : 'crmc_live_\u2026';

    return {
      headers:['Время проверки активности','Куда','Строк','Статус'],
      rows, addFn:null,
      customTop: `
        <div style="display:flex; align-items:flex-end; gap:8px; margin-bottom:16px;">
          <div class="field"><label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">С</label><input type="date" class="cell-input" style="border:1px solid var(--border-strong);" value="${outgoingRange.from}" onchange="setOutgoingRange('from', this.value)"></div>
          <div class="field"><label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">По</label><input type="date" class="cell-input" style="border:1px solid var(--border-strong);" value="${outgoingRange.to}" onchange="setOutgoingRange('to', this.value)"></div>
        </div>

        <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:6px;">
          <div style="font-size:13px; font-weight:600;">Проверка активности</div>
          ${hintIconBtn('refresh', 'Обновить статус активности', 'refreshOutgoingTraffic()')}
            </div>
        <p style="font-size:12px; color:var(--text-muted); margin:0 0 10px;">Кнопка шлёт проверку во все проекты Скорозвона и CRMCalls и тестовую заявку на webhook каждого партнёра. В таблице \u2014 последний ответ.</p>`,
      customAfter: `
        <div style="margin-top:22px;">
          <div style="font-size:13px; font-weight:600; margin-bottom:8px;">Строк по проектам Скорозвона за период</div>
          <div class="proj-toolbar">
            <input type="search" class="cell-input proj-search" id="skorozvon-project-search" placeholder="Поиск по названию" value="${escAttr(skorozvonProjectName)}" oninput="onSkorozvonProjectNameInput(this.value)" onkeydown="if(event.key==='Enter'){event.preventDefault();applySkorozvonProjectSearch();}">
            ${iconBtn('search', 'Найти', 'applySkorozvonProjectSearch()')}
            <div class="hint-icon-row">
              ${hintIconBtn('eye', 'Видимость', 'openProjectVisibilityModal()')}
              ${hintIconBtn('cloud', 'Синхронизировать проекты', 'syncSkorozvonProjects()')}
          </div>
          </div>
          ${projectRows ? `<table><thead><tr><th>Проект</th><th>Строк</th><th>Проверка</th><th></th></tr></thead><tbody>${projectRows}</tbody></table>` : '<p style="color:var(--text-muted); font-size:13px;">Нет проектов \u2014 измените поиск или синхронизируйте проекты.</p>'}
          <div class="proj-pager">
            <span>${skorozvonProjectPagerInfo()}</span>
            <button class="btn btn-sm" ${skorozvonProjectOffset <= 0 ? 'disabled' : ''} onclick="shiftSkorozvonProjectPage(-${SKOROZVON_PROJECT_PAGE})" aria-label="Предыдущая страница">&lt;</button>
            <button class="btn btn-sm" ${skorozvonProjectOffset + SKOROZVON_PROJECT_PAGE >= skorozvonProjectTotal ? 'disabled' : ''} onclick="shiftSkorozvonProjectPage(${SKOROZVON_PROJECT_PAGE})" aria-label="Следующая страница">&gt;</button>
          </div>
        </div>

        <div style="margin-top:28px;">
          <div style="display:flex; justify-content:space-between; align-items:center; margin-bottom:8px; gap:12px;">
            <div style="font-size:13px; font-weight:600;">Подключение CRMCalls</div>
            <span class="status-pill ${cc.has_key ? 'on' : 'off'}">${cc.has_key ? 'подключено' : 'нет ключа'}</span>
        </div>
          <p style="font-size:12px; color:var(--text-muted); margin:0 0 10px;">Ключ из кабинета CRMCalls. После «Подключить» подтянем проекты \u2014 они появятся в селекте связок дубли/уники.</p>
          <div style="display:flex; flex-wrap:wrap; gap:10px; align-items:flex-end;">
            <div class="field" style="flex:1; min-width:240px;">
              <label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">API-ключ</label>
              <input type="password" class="cell-input" style="border:1px solid var(--border-strong); font-family:monospace;" value="${escAttr(crmcallsKeyDraft)}" placeholder="${escAttr(ccKeyPlaceholder)}" oninput="crmcallsKeyDraft=this.value" autocomplete="off">
            </div>
            <div class="field" style="width:260px;">
              <label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">Host</label>
              <input class="cell-input" style="border:1px solid var(--border-strong);" value="${escAttr(crmcallsBaseDraft || cc.base_url || 'https://cc.crmcalls.ru')}" oninput="crmcallsBaseDraft=this.value">
            </div>
            <button class="btn btn-primary" ${crmcallsConnecting ? 'disabled' : ''} onclick="connectCrmcalls()">${crmcallsConnecting ? 'Подключаем\u2026' : (cc.has_key ? 'Обновить и подтянуть' : 'Подключить')}</button>
          </div>
        </div>

        <div style="margin-top:22px;">
          <div style="font-size:13px; font-weight:600; margin-bottom:8px;">Проекты CRMCalls</div>
          <div class="proj-toolbar">
            <input type="search" class="cell-input proj-search" placeholder="Поиск по названию или UUID" value="${escAttr(crmcallsProjectName)}" oninput="crmcallsProjectName=this.value" onkeydown="if(event.key==='Enter'){event.preventDefault();renderPanels();}">
            ${iconBtn('search', 'Найти', 'renderPanels()')}
            <div class="hint-icon-row">
              ${hintIconBtn('cloud', 'Синхронизировать проекты CRMCalls', 'syncCrmcallsProjects()')}
            </div>
          </div>
          ${ccRows ? `<table><thead><tr><th>Проект</th><th>Строк</th><th>Проверка</th><th></th></tr></thead><tbody>${ccRows}</tbody></table>` : '<p style="color:var(--text-muted); font-size:13px;">Нет проектов \u2014 подключите CRMCalls или синхронизируйте.</p>'}
        </div>`,
      note:'Число строк — успешные отправки из dispatch за выбранный период. Проекты CRMCalls подключаются здесь и сразу попадают в селект связок. Настройка маршрутов \u2014 на вкладках «Отправка партнёрам», «Работа с дублями», «Работа с униками».'
    };
  },

  skorozvonBitrix: () => {
    const sources = rootSources().map(root => ({ id: root.id, nazvanie: root.nazvanie }));
    const none = skorozvonBitrixStats.byId[0];
    if (none && (none.created || none.updated)) {
      sources.push({ id: 0, nazvanie: 'Без источника' });
    }
    const rows = sources.map(root => `<tr>
        <td style="font-weight:600;">${escAttr(root.nazvanie)}</td>
        ${skorozvonBitrixRow(root.id)}
      </tr>`).join('');
    return {
      headers:['Источник','Обновилось','Отправлено (обновл.)','Конв.','Создалось со Скорозвона','Отправлено (новые)','Конв.'],
      rows, addFn:null,
      customTop: `
        <div style="display:flex; align-items:flex-end; gap:8px; margin-bottom:12px;">
          <div class="field"><label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">С</label><input type="date" class="cell-input" style="border:1px solid var(--border-strong);" value="${skorozvonBitrixRange.from}" onchange="setSkorozvonBitrixRange('from', this.value)"></div>
          <div class="field"><label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">По</label><input type="date" class="cell-input" style="border:1px solid var(--border-strong);" value="${skorozvonBitrixRange.to}" onchange="setSkorozvonBitrixRange('to', this.value)"></div>
        </div>`,
      note:'Левый блок — наши строки (via_us = true) с id в Скорозвоне: сколько обновилось и сколько из них ушло в Битрикс (есть bitrix_lead_id). Правый блок — строки, которых не было у нас (via_us = false): сколько создано из вебхука Скорозвона и сколько из них уже в Битрикс. Низкая конверсия (выделено) — сигнал проверить передачу в Б24.'
    };
  },

  regionControl: () => enrichmentPanel({
    tab: 'regionControl',
    range: regionRange,
    setFrom: 'setRegionRange',
    setTo: 'setRegionRange',
    headers: ['Источник', 'От поставщика', 'Россети', 'Не определён'],
    cellsFn: id => geoStatCells(regionStats, regionRange, id, 'region'),
    note: 'Доли по тому, как воркер проставил регион: от поставщика, через Россети или не определил. Высокая доля «Не определён» (выделено) — сигнал проверить, шлёт ли источник регион и работает ли Россети.',
  }),

  operatorControl: () => enrichmentPanel({
    tab: 'operatorControl',
    range: operatorRange,
    setFrom: 'setOperatorRange',
    setTo: 'setOperatorRange',
    headers: ['Источник', 'От поставщика', 'Россети', 'Не определён (в «Другие»)'],
    cellsFn: id => geoStatCells(operatorStats, operatorRange, id, 'operator'),
    note: 'Доли по тому, как воркер проставил оператора. «Не определён» — пустой или несмапленный оператор, он уходит в «Другие». Высокая доля (выделено) — проверить справочник «Операторы».',
  }),

  zaryaControl: () => enrichmentPanel({
    tab: 'zaryaControl',
    range: zaryaRange,
    setFrom: 'setZaryaRange',
    setTo: 'setZaryaRange',
    headers: ['Источник', 'Уник', 'Дубль'],
    cellsFn: id => zaryaStatCells(id),
    note: 'Последний результат Зари по строке: уник = «Чистый», дубль = всё остальное. Проценты считаются только среди проверенных строк.',
  }),
};

let skorozvonBitrixRange = defaultIncomingRange();
let skorozvonBitrixStats = { key: '', loading: true, error: '', byId: {} };
let skorozvonBitrixStatsGen = 0;
function skorozvonBitrixRangeKey() { return skorozvonBitrixRange.from + '|' + skorozvonBitrixRange.to; }
function setSkorozvonBitrixRange(field, value) { skorozvonBitrixRange[field] = value; loadSkorozvonBitrixStats(); }
function skorozvonBitrixRow(sourceId) {
  if (skorozvonBitrixStats.loading || skorozvonBitrixStats.key !== skorozvonBitrixRangeKey()) {
    return statsSpinnerCell() + statsSpinnerCell() + statsSpinnerCell() + statsSpinnerCell() + statsSpinnerCell() + statsSpinnerCell();
  }
  if (skorozvonBitrixStats.error) {
    return statsDashCell(skorozvonBitrixStats.error) + statsDashCell('') + statsDashCell('') + statsDashCell('') + statsDashCell('') + statsDashCell('');
  }
  const s = skorozvonBitrixStats.byId[sourceId] || { updated: 0, updated_sent: 0, created: 0, created_sent: 0 };
  const convUpd = statPct(s.updated_sent, s.updated);
  const convNew = statPct(s.created_sent, s.created);
  return `<td>${(s.updated || 0).toLocaleString('ru-RU')}</td>
        <td>${(s.updated_sent || 0).toLocaleString('ru-RU')}</td>
        <td style="color:${convUpd < 50 && s.updated ? 'var(--danger)' : 'var(--text-2)'};">${convUpd}%</td>
        <td style="border-left:1px solid var(--border);">${(s.created || 0).toLocaleString('ru-RU')}</td>
        <td>${(s.created_sent || 0).toLocaleString('ru-RU')}</td>
        <td style="color:${convNew < 50 && s.created ? 'var(--danger)' : 'var(--text-2)'};">${convNew}%</td>`;
}
async function loadSkorozvonBitrixStats() {
  const box = { get gen() { return skorozvonBitrixStatsGen; }, set gen(v) { skorozvonBitrixStatsGen = v; }, get state() { return skorozvonBitrixStats; }, set state(v) { skorozvonBitrixStats = v; } };
  await fetchRangeStats({
    tab: 'skorozvonBitrix', range: skorozvonBitrixRange, path: '/rest/leads/skorozvon-bitrix-stats', box,
    map: item => ({
      updated: item.updated || 0,
      updated_sent: item.updated_sent || 0,
      created: item.created || 0,
      created_sent: item.created_sent || 0,
    }),
  });
}

const enrichSourceIds = { regionControl: [], operatorControl: [], zaryaControl: [] };

function visibleEnrichSources(tab) {
  const roots = rootSources();
  const ids = enrichSourceIds[tab] || [];
  if (!ids.length) return roots;
  const set = new Set(ids.map(Number));
  return roots.filter(s => set.has(s.id));
}

function enrichmentPanel(opts) {
  const sources = visibleEnrichSources(opts.tab);
  const rows = sources.map(root => `
      <tr>
        <td style="font-weight:600; text-align:left;">${escAttr(root.nazvanie)}</td>
        ${opts.cellsFn(root.id)}
      </tr>`).join('');
    return {
    headers: opts.headers,
    rows,
    addFn: null,
    customTop: enrichmentFiltersHTML(opts),
    note: opts.note,
  };
}

function enrichmentFiltersHTML(opts) {
  const selected = enrichSourceIds[opts.tab] || [];
  const sel = new Set(selected.map(String));
  const roots = rootSources();
  let label = 'Все источники';
  if (sel.size === 1) {
    const one = roots.find(s => String(s.id) === [...sel][0]);
    label = one ? one.nazvanie : '1 источник';
  } else if (sel.size > 1) {
    label = sel.size + ' выбрано';
  }
  const optsHtml = roots.map(s => `
    <label class="ms-opt" data-text="${escAttr(String(s.nazvanie).toLowerCase())}">
      <span>${escAttr(s.nazvanie)}</span>
      <input type="checkbox" value="${s.id}" ${sel.has(String(s.id)) ? 'checked' : ''} onchange="toggleEnrichSource('${opts.tab}', ${s.id}, this.checked)">
    </label>`).join('');
  return `
    <div class="enrich-filters">
      <div class="field"><label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">С</label><input type="date" class="cell-input" style="border:1px solid var(--border-strong);" value="${opts.range.from}" onchange="${opts.setFrom}('from', this.value)"></div>
      <div class="field"><label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">По</label><input type="date" class="cell-input" style="border:1px solid var(--border-strong);" value="${opts.range.to}" onchange="${opts.setTo}('to', this.value)"></div>
      <div class="field" style="min-width:240px; flex:1;">
        <label style="font-size:11px; color:var(--text-muted); display:block; margin-bottom:2px;">Источник</label>
        <div class="ms" id="enrich-src-${opts.tab}">
          <button type="button" class="ms-trigger" onclick="toggleEnrichSourceMs('${opts.tab}', event)"><span class="ms-label">${escAttr(label)}</span></button>
          <span class="ms-caret">▾</span>
          <div class="ms-drop" onclick="event.stopPropagation()">
            <input type="text" class="ms-search" placeholder="Поиск..." oninput="filterEnrichSourceMs('${opts.tab}', this.value)">
            <div class="ms-opts">${optsHtml || '<div class="ms-empty">Нет источников</div>'}</div>
            <div class="ms-foot"><button type="button" class="btn" onclick="clearEnrichSource('${opts.tab}')">Все источники</button></div>
          </div>
        </div>
      </div>
    </div>`;
}

function toggleEnrichSourceMs(tab, e) {
  e.stopPropagation();
  const el = document.getElementById('enrich-src-' + tab);
  if (!el) return;
  const open = el.classList.contains('open');
  document.querySelectorAll('.ms.open').forEach(x => x.classList.remove('open'));
  if (!open) el.classList.add('open');
}

function filterEnrichSourceMs(tab, query) {
  const el = document.getElementById('enrich-src-' + tab);
  if (!el) return;
  const q = (query || '').toLowerCase();
  el.querySelectorAll('.ms-opt').forEach(opt => {
    opt.classList.toggle('hidden', Boolean(q) && !(opt.dataset.text || '').includes(q));
  });
}

function toggleEnrichSource(tab, id, checked) {
  const cur = (enrichSourceIds[tab] || []).slice();
  const i = cur.indexOf(id);
  if (checked && i < 0) cur.push(id);
  if (!checked && i >= 0) cur.splice(i, 1);
  enrichSourceIds[tab] = cur;
  renderPanels();
  requestAnimationFrame(() => {
    const el = document.getElementById('enrich-src-' + tab);
    if (el) el.classList.add('open');
  });
}

function clearEnrichSource(tab) {
  enrichSourceIds[tab] = [];
  renderPanels();
}

document.addEventListener('click', () => {
  document.querySelectorAll('.ms.open').forEach(x => x.classList.remove('open'));
});

let regionRange = defaultIncomingRange();
let operatorRange = defaultIncomingRange();
let zaryaRange = defaultIncomingRange();
let regionStats = { key: '', loading: true, error: '', byId: {} };
let operatorStats = { key: '', loading: true, error: '', byId: {} };
let zaryaStats = { key: '', loading: true, error: '', byId: {} };
let regionStatsGen = 0;
let operatorStatsGen = 0;
let zaryaStatsGen = 0;
function rangeKey(range) { return range.from + '|' + range.to; }
function statPct(part, total) { return total ? Math.round(part * 100 / total) : 0; }
function statsSpinnerCell() {
  return '<td><span class="count-spinner" title="Загрузка"></span></td>';
}
function statsDashCell(err) {
  return '<td><span style="color:var(--text-muted); font-weight:500;" title="' + escAttr(err || '') + '">\u2014</span></td>';
}
function geoStatCells(state, range, sourceId, kind) {
  if (state.loading || state.key !== rangeKey(range)) {
    return statsSpinnerCell() + statsSpinnerCell() + statsSpinnerCell();
  }
  if (state.error) {
    return statsDashCell(state.error) + statsDashCell('') + statsDashCell('');
  }
  const row = state.byId[sourceId] || { total: 0, supplier: 0, provider: 0, unknown: 0 };
  const unknown = statPct(row.unknown, row.total);
  const danger = unknown > 15 ? ' style="color:var(--danger);"' : '';
  return `<td>${statPct(row.supplier, row.total)}%</td>
        <td>${statPct(row.provider, row.total)}%</td>
        <td${danger}>${unknown}%</td>`;
}
function zaryaStatCells(sourceId) {
  if (zaryaStats.loading || zaryaStats.key !== rangeKey(zaryaRange)) {
    return statsSpinnerCell() + statsSpinnerCell();
  }
  if (zaryaStats.error) {
    return statsDashCell(zaryaStats.error) + statsDashCell('');
  }
  const row = zaryaStats.byId[sourceId] || { total: 0, unique: 0, dup: 0 };
  return `<td>${statPct(row.unique, row.total)}%</td>
        <td>${statPct(row.dup, row.total)}%</td>`;
}
async function fetchRangeStats(opts) {
  const key = rangeKey(opts.range);
  const gen = ++opts.box.gen;
  opts.box.state = { key, loading: true, error: '', byId: {} };
  if (activeTab === opts.tab) renderPanels();
  try {
    const q = 'date_from=' + encodeURIComponent(opts.range.from) + '&date_to=' + encodeURIComponent(opts.range.to);
    const data = await apiJSON(opts.path + '?' + q);
    if (gen !== opts.box.gen) return;
    const byId = {};
    (data.items || []).forEach(item => { byId[item.source_id] = opts.map(item); });
    opts.box.state = { key, loading: false, error: '', byId };
  } catch (err) {
    if (gen !== opts.box.gen) return;
    opts.box.state = { key, loading: false, error: err.message || 'Не удалось загрузить статистику', byId: {} };
  }
  if (activeTab === opts.tab) renderPanels();
}
function mapGeoShare(item, kind) {
  const share = (item && item[kind]) || {};
  return {
    total: item.total || 0,
    supplier: share.supplier || 0,
    provider: share.provider || 0,
    unknown: share.unknown || 0,
  };
}
async function loadRegionStats() {
  const box = { get gen() { return regionStatsGen; }, set gen(v) { regionStatsGen = v; }, get state() { return regionStats; }, set state(v) { regionStats = v; } };
  await fetchRangeStats({ tab: 'regionControl', range: regionRange, path: '/rest/leads/enrichment-stats', box, map: item => mapGeoShare(item, 'region') });
}
async function loadOperatorStats() {
  const box = { get gen() { return operatorStatsGen; }, set gen(v) { operatorStatsGen = v; }, get state() { return operatorStats; }, set state(v) { operatorStats = v; } };
  await fetchRangeStats({ tab: 'operatorControl', range: operatorRange, path: '/rest/leads/enrichment-stats', box, map: item => mapGeoShare(item, 'operator') });
}
async function loadZaryaStats() {
  const box = { get gen() { return zaryaStatsGen; }, set gen(v) { zaryaStatsGen = v; }, get state() { return zaryaStats; }, set state(v) { zaryaStats = v; } };
  await fetchRangeStats({
    tab: 'zaryaControl', range: zaryaRange, path: '/rest/leads/zarya-stats', box,
    map: item => ({ total: item.total || 0, unique: item.unique || 0, dup: item.dup || 0 }),
  });
}
function setRegionRange(field, value) { regionRange[field] = value; loadRegionStats(); }
function setOperatorRange(field, value) { operatorRange[field] = value; loadOperatorStats(); }
function setZaryaRange(field, value) { zaryaRange[field] = value; loadZaryaStats(); }

/* ===== Моковая статистика трафика ===== */
function ymdLocal(d) {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return y + '-' + m + '-' + day;
}
function defaultIncomingRange() {
  const to = new Date();
  const from = new Date(to.getFullYear(), to.getMonth(), to.getDate() - 6);
  return { from: ymdLocal(from), to: ymdLocal(to) };
}
let incomingRange = defaultIncomingRange();
let incomingCounts = { key: '', loading: true, error: '', byId: {} };
let incomingCountsGen = 0;
function incomingRangeKey() { return incomingRange.from + '|' + incomingRange.to; }
function incomingCountCell(sourceId) {
  if (incomingCounts.loading || incomingCounts.key !== incomingRangeKey()) {
    return '<span class="count-spinner" title="Загрузка"></span>';
  }
  if (incomingCounts.error) {
    return '<span style="color:var(--text-muted); font-weight:500;" title="' + escAttr(incomingCounts.error) + '">\u2014</span>';
  }
  return (incomingCounts.byId[sourceId] || 0).toLocaleString('ru-RU');
}
async function loadIncomingCounts() {
  const key = incomingRangeKey();
  const gen = ++incomingCountsGen;
  incomingCounts = { key, loading: true, error: '', byId: {} };
  if (activeTab === 'incomingTraffic') renderPanels();
  try {
    const q = 'date_from=' + encodeURIComponent(incomingRange.from) + '&date_to=' + encodeURIComponent(incomingRange.to);
    const data = await apiJSON('/rest/leads/incoming-stats?' + q);
    if (gen !== incomingCountsGen) return;
    const byId = {};
    (data.items || []).forEach(item => { byId[item.source_id] = item.count || 0; });
    incomingCounts = { key, loading: false, error: '', byId };
  } catch (err) {
    if (gen !== incomingCountsGen) return;
    incomingCounts = { key, loading: false, error: err.message || 'Не удалось загрузить строки', byId: {} };
  }
  if (activeTab === 'incomingTraffic') renderPanels();
}
function setIncomingRange(field, value) {
  incomingRange[field] = value;
  loadIncomingCounts();
}
function setIncomingPreset(days) {
  const to = new Date('2026-08-20');
  const from = new Date(to.getTime() - (days-1) * 86400000);
  incomingRange = { from: from.toISOString().slice(0,10), to: to.toISOString().slice(0,10) };
  renderPanels();
}
function trafficStatsForRange(sourceId, from, to) {
  const days = Math.max(1, Math.round((new Date(to) - new Date(from)) / 86400000) + 1);
  const seed = sourceId * 37;
  const perDay = (seed % 40) + 5;
  return perDay * days;
}
let outgoingRange = defaultIncomingRange();
let outgoingStats = { key: '', loading: true, error: '', partners: {}, skorozvon: {}, crmcalls: {} };
let outgoingStatsGen = 0;
function outgoingRangeKey() { return outgoingRange.from + '|' + outgoingRange.to; }
function outgoingCountCell(map, id) {
  if (outgoingStats.loading || outgoingStats.key !== outgoingRangeKey()) {
    return '<span class="count-spinner" title="Загрузка"></span>';
  }
  if (outgoingStats.error) {
    return '<span style="color:var(--text-muted); font-weight:500;" title="' + escAttr(outgoingStats.error) + '">\u2014</span>';
  }
  return (map[id] || 0).toLocaleString('ru-RU');
}
function outgoingPartnerCount(id) { return outgoingCountCell(outgoingStats.partners, id); }
function outgoingSkorozvonCount(projectId) { return outgoingCountCell(outgoingStats.skorozvon, String(projectId)); }
function outgoingCrmcallsCount(projectId) { return outgoingCountCell(outgoingStats.crmcalls, String(projectId)); }
async function loadOutgoingStats() {
  const key = outgoingRangeKey();
  const gen = ++outgoingStatsGen;
  outgoingStats = { key, loading: true, error: '', partners: {}, skorozvon: {}, crmcalls: {} };
  if (activeTab === 'outgoingTraffic') renderPanels();
  try {
    const q = 'date_from=' + encodeURIComponent(outgoingRange.from) + '&date_to=' + encodeURIComponent(outgoingRange.to);
    const data = await apiJSON('/rest/leads/outgoing-stats?' + q);
    if (gen !== outgoingStatsGen) return;
    const partners = {};
    (data.partners || []).forEach(item => { partners[item.partner_id] = item.count || 0; });
    const skorozvon = {};
    (data.skorozvon || []).forEach(item => { skorozvon[String(item.project_id)] = item.count || 0; });
    const crmcalls = {};
    (data.crmcalls || []).forEach(item => { crmcalls[String(item.project_id)] = item.count || 0; });
    outgoingStats = { key, loading: false, error: '', partners, skorozvon, crmcalls };
  } catch (err) {
    if (gen !== outgoingStatsGen) return;
    outgoingStats = { key, loading: false, error: err.message || 'Не удалось загрузить строки', partners: {}, skorozvon: {}, crmcalls: {} };
  }
  if (activeTab === 'outgoingTraffic') renderPanels();
}
function setOutgoingRange(field, value) {
  outgoingRange[field] = value;
  loadOutgoingStats();
}
function fmtCheckTime(raw) {
  if (!raw) return '\u2014';
  return String(raw).replace('T', ' ').replace(/\.\d+Z?$/, '').replace('Z', '');
}
function skorozvonCheckLabel(p) {
  if (skorozvonProjectChecking && !p.last_date_request) {
    return '<span class="status-pill">проверка…</span>';
  }
  if (!p.last_date_request) return '\u2014';
  const ok = p.last_request === true;
  const when = fmtCheckTime(p.last_date_request);
  return `<span class="status-pill ${ok ? 'on' : 'off'}">${ok ? '200 OK' : 'не 200'}</span><div style="font-size:11px; color:var(--text-muted); margin-top:3px;">${when}</div>`;
}
function partnerReply(u) {
  const httpOk = u.last_request === true || (u.last_http_status >= 200 && u.last_http_status < 300);
  const parsed = parsePartnerReply(u.last_response);
  const ok = httpOk && parsed.ok !== false;
  return {
    ok,
    pill: ok ? (u.last_http_status && u.last_http_status !== 200 ? String(u.last_http_status) : 'принято') : (u.last_http_status && u.last_http_status >= 400 ? String(u.last_http_status) : 'ошибка'),
    text: parsed.text || (ok ? 'Партнёр ответил без замечаний' : 'Партнёр не принял заявку'),
  };
}
function parsePartnerReply(raw) {
  const src = String(raw || '').trim();
  if (!src) return { ok: true, text: '' };
  let data = null;
  if (src.charAt(0) === '{' || src.charAt(0) === '[') {
    try { data = JSON.parse(src); } catch (_) { data = null; }
  }
  if (data && typeof data === 'object' && !Array.isArray(data)) {
    const err = firstReplyText(data.error, data.errors, data.err);
    const msg = firstReplyText(data.message, data.msg, data.detail, data.description);
    const status = String(data.status || data.result || '').toLowerCase();
    const failed = !!(err || status === 'error' || status === 'fail' || status === 'failed');
    return { ok: !failed, text: humanPartnerReply(err || msg || (failed ? 'Партнёр вернул ошибку' : '')) };
  }
  return { ok: !/error|fail|ошиб/i.test(src), text: humanPartnerReply(src) };
}
function firstReplyText() {
  for (let i = 0; i < arguments.length; i++) {
    const v = arguments[i];
    if (v == null || v === '') continue;
    if (typeof v === 'string') return v.trim();
    if (Array.isArray(v) && v.length) return firstReplyText.apply(null, v);
    if (typeof v === 'object') {
      const inner = firstReplyText(v.message, v.error, v.msg, v.detail);
      if (inner) return inner;
    }
  }
  return '';
}
function humanPartnerReply(text) {
  const src = String(text || '').trim();
  if (!src) return '';
  const key = src.replace(/\s+/g, ' ');
  const map = {
    'Missing parameters': 'Не хватает параметров',
    "Bad Request: 'phone' parameter is missing": 'Не передан телефон',
    'Processed. Mode: Saved to inbound buffer. Queued for morning (Non-work hours)': 'Принято, в очереди до рабочего времени',
    'Processed. Mode: No new leads processed (possible duplicates)': 'Дубль — новых заявок нет',
  };
  if (map[key]) return map[key];
  const lower = key.toLowerCase();
  if (lower.includes('missing parameters')) return 'Не хватает параметров';
  if (lower.includes('phone') && (lower.includes('missing') || lower.includes('required'))) return 'Не передан телефон';
  if (lower.includes('duplicate')) return 'Дубль — новых заявок нет';
  if (lower.includes('queued') || lower.includes('non-work')) return 'Принято, в очереди до рабочего времени';
  if (lower === 'success' || lower === 'ok' || lower === 'accepted') return 'Принято партнёром';
  if (/^[{[]/.test(src)) return 'Партнёр вернул ответ';
  return src.length > 140 ? src.slice(0, 137) + '\u2026' : src;
}
function partnerCheckLabel(u) {
  if (!u.last_date_request) return '\u2014';
  const reply = partnerReply(u);
  return `<span class="status-pill ${reply.ok ? 'on' : 'off'}">${escAttr(reply.pill)}</span>
    <div style="font-size:12px; color:var(--text-2); margin-top:4px; line-height:1.35;">${escAttr(reply.text)}</div>`;
}
async function refreshOutgoingTraffic() {
  try {
    showTestToast('Проверяем Скорозвон, CRMCalls и партнёров\u2026');
    const results = await Promise.allSettled([
      apiJSON('/rest/skorozvon/projects/refresh', { method: 'POST' }),
      apiJSON('/rest/partners/refresh', { method: 'POST' }),
      apiJSON('/rest/crmcalls/projects/refresh', { method: 'POST' }),
    ]);
    fetched.skorozvonProjects = false;
    fetched.crmcallsProjects = false;
    await Promise.all([loadSkorozvonProjects(), loadPartners(), loadCrmcallsProjects()]);
    fetched.skorozvonProjects = true;
    fetched.crmcallsProjects = true;
    renderPanels();
    const failed = results.filter(r => r.status === 'rejected');
    if (failed.length === results.length) {
      showError(failed[0].reason && failed[0].reason.message || 'Не удалось проверить активность');
      return;
    }
    if (failed.length) {
      showTestToast('Часть проверок не прошла: ' + ((failed[0].reason && failed[0].reason.message) || ''), true);
    } else {
      showTestToast('Статус активности обновлён');
    }
    showToast(activeTab);
  } catch (err) {
    showError(err.message || 'Не удалось проверить активность');
  }
}
async function sendPartnerTest(id) {
  const u = (DATA.upk || []).find(x => x.id === id);
  const name = u ? u.nazvanie : ('#' + id);
  showTestToast('Отправляем тест в «' + name + '»\u2026');
  try {
    const data = await apiJSON('/rest/partners/' + id + '/test', { method: 'POST' });
    const reply = partnerReply({
      last_date_request: data.checked_at || true,
      last_request: data.ok,
      last_http_status: data.http_status,
      last_response: data.response || '',
    });
    if (reply.ok) {
      showTestToast('«' + name + '»: ' + reply.text);
    } else {
      showTestToast('«' + name + '»: ' + reply.text, true);
    }
    await loadPartners();
    renderPanels();
  } catch (err) {
    showTestToast('Ошибка тестирования \u2014 ' + name + ': ' + (err.message || ''), true);
  }
}
async function sendSkorozvonTest(projectId) {
  const p = DATA.skorozvonProjects.find(x => String(x.id) === String(projectId) || String(x.uuid) === String(projectId));
  if (!p) {
    showTestToast('Проект не найден', true);
    return;
  }
  showTestToast(`Отправляем тестовую строку в «${p.nazvanie}»\u2026`);
  try {
    const data = await apiJSON('/rest/skorozvon/projects/' + encodeURIComponent(p.uuid || p.id) + '/test', { method: 'POST' });
    const extra = data.lead_id ? ` (id ${data.lead_id})` : '';
    showTestToast(`Тест успешно отправлен \u2014 ${p.nazvanie}${extra}`);
  } catch (err) {
    showTestToast(`Ошибка тестирования \u2014 ${p.nazvanie}: ${err.message || 'проверьте project_id'}`, true);
  }
}
function showTestToast(msg, isError) {
  let t = document.getElementById('test-toast');
  if (!t) {
    t = document.createElement('div');
    t.id = 'test-toast';
    t.style.cssText = 'position:fixed; bottom:20px; right:20px; z-index:999; padding:10px 16px; border-radius:6px; font-size:13px; box-shadow:0 4px 14px rgba(0,0,0,.15); transition:opacity .2s;';
    document.body.appendChild(t);
  }
  t.textContent = msg;
  t.style.background = isError ? '#fbece8' : '#eaf1e8';
  t.style.color = isError ? '#a23b2e' : '#2f5233';
  t.style.border = '1px solid ' + (isError ? '#e0b3a8' : '#c3d9c3');
  t.style.opacity = '1';
  clearTimeout(t._timer);
  t._timer = setTimeout(() => { t.style.opacity = '0'; }, 2600);
}

/* ===== Модалка видимости проектов (как выбор колонок в Bitrix) ===== */
function openProjectVisibilityModal() {
  const existing = document.getElementById('projvis-overlay');
  if (existing) existing.remove();
  const overlay = document.createElement('div');
  overlay.id = 'projvis-overlay';
  overlay.className = 'overlay show';
  overlay.onclick = (e) => { if (e.target === overlay) overlay.remove(); };
  overlay.innerHTML = `
    <div class="modal-card" style="width:380px;">
      <div class="modal-head"><p class="modal-title">Видимость проектов</p></div>
      <div class="modal-body">
        <p style="font-size:12px; color:var(--text-muted); margin:0 0 10px;">Отметьте, какие проекты показывать в сводке. Остальные останутся в системе, просто не будут отображаться.</p>
        <div id="projvis-list"></div>
      </div>
      <div class="modal-foot">
        <button class="btn" onclick="document.getElementById('projvis-overlay').remove()">Отмена</button>
        <button class="btn btn-primary" onclick="applyProjectVisibility()">Применить</button>
      </div>
    </div>`;
  document.body.appendChild(overlay);
  const box = document.getElementById('projvis-list');
  box.innerHTML = '<div class="page-loading"><span class="page-spinner" aria-hidden="true"></span>Загрузка…</div>';
  loadAllSkorozvonProjects().then(() => {
    fetched.skorozvonProjectsAll = true;
  renderProjectVisibilityList();
  }).catch(err => {
    box.textContent = err.message || 'Не удалось загрузить проекты';
  });
}
function renderProjectVisibilityList() {
  const box = document.getElementById('projvis-list');
  if (!box) return;
  const items = skorozvonProjectCatalog();
  box.innerHTML = items.map(p => `
    <label style="display:flex; align-items:center; gap:8px; padding:6px 0; border-bottom:1px dashed var(--border); font-size:13px;">
      <input type="checkbox" data-proj="${p.id}" ${p.visible?'checked':''}>
      <span>${p.nazvanie} <span style="color:var(--text-muted); font-family:monospace; font-size:11px;">(${p.project_id})</span></span>
    </label>`).join('') || '<p style="color:var(--text-muted); font-size:13px;">Нет проектов</p>';
}
function applyProjectVisibility() {
  hiddenSkorozvonProjectIDs.clear();
  document.querySelectorAll('#projvis-list input[type=checkbox]').forEach(cb => {
    if (!cb.checked) hiddenSkorozvonProjectIDs.add(cb.dataset.proj);
  });
  (DATA.skorozvonProjects || []).forEach(p => { p.visible = !hiddenSkorozvonProjectIDs.has(p.id); });
  (DATA.skorozvonProjectsAll || []).forEach(p => { p.visible = !hiddenSkorozvonProjectIDs.has(p.id); });
  document.getElementById('projvis-overlay').remove();
  renderPanels();
  showToast(activeTab);
}

function renderPanels() {
  const wrap = document.getElementById('panels');
  const t = TABS.find(x => x.id === activeTab);
  if (tabLoading) {
    wrap.innerHTML = `
      <div class="panel">
        <div class="panel-head"><p class="panel-title">${t.label}</p></div>
        <div class="page-loading"><span class="page-spinner" aria-hidden="true"></span>Загрузка…</div>
      </div>`;
    return;
  }
  const res = PANEL_RENDERERS[activeTab]();
  const bodyHTML = (res.customTop || '') + (res.customHTML
    ? res.customHTML
    : `<table>
        <thead><tr>${res.headers.map(h => `<th>${h}</th>`).join('')}</tr></thead>
        <tbody>${res.rows || `<tr class="empty-row"><td colspan="${res.headers.length}">Список пуст</td></tr>`}</tbody>
      </table>`);
  wrap.innerHTML = `
    <div class="panel">
      <div class="panel-head">
        <p class="panel-title">${t.label}</p>
        ${res.addFn ? `<button class="btn btn-primary" onclick="${res.addFn}()">+ ${res.addLabel || 'Добавить'}</button>` : ''}
      </div>
      ${bodyHTML}
      ${res.customAfter || ''}
      ${res.note ? `<p style="font-size:12px; color:var(--text-muted); margin:8px 0 0;">${res.note}</p>` : ''}
      <div class="impact-note"><b>Что это меняет:</b>&nbsp;${t.impact}</div>
      ${t.warn ? `<div class="impact-note warn"><b>Важно:</b>&nbsp;${t.warn}</div>` : ''}
      <div class="save-toast" id="toast-${activeTab}">Сохранено — применяется к новым строкам сразу.</div>
    </div>
  `;
  if (activeTab === 'bitrixFields') bindBitrixMapCombos();
}

/* ===== МУТАЦИИ ===== */
function updateField(entity, id, field, value) {
  const row = DATA[entity].find(r => r.id === id);
  if (row) row[field] = value;
  renderPanels();
  showToast(activeTab);
}
function updateFieldByIndex(entity, idx, field, value) {
  DATA[entity][idx][field] = value;
  renderPanels();
  showToast(activeTab);
}
function deleteRow(entity, id) { DATA[entity] = DATA[entity].filter(r => r.id !== id); renderPanels(); }
function deleteRowByIndex(entity, idx) { DATA[entity].splice(idx, 1); renderPanels(); }
/* ===== СОБСТВЕННЫЕ МОДАЛКИ ВМЕСТО prompt()/confirm() (надёжнее в песочнице/превью) ===== */
let _promptCallback = null;
function customPrompt(title, callback) {
  clearError();
  _promptCallback = callback;
  document.getElementById('custom-prompt-title').textContent = title;
  const input = document.getElementById('custom-prompt-input');
  input.value = '';
  document.getElementById('custom-prompt-overlay').classList.add('show');
  setTimeout(() => input.focus(), 30);
}
function customPromptConfirm() {
  const val = document.getElementById('custom-prompt-input').value.trim();
  document.getElementById('custom-prompt-overlay').classList.remove('show');
  const cb = _promptCallback; _promptCallback = null;
  if (cb) cb(val || null);
}
function customPromptCancel() {
  document.getElementById('custom-prompt-overlay').classList.remove('show');
  _promptCallback = null;
}

let _confirmCallback = null;
function customConfirm(text, callback) {
  _confirmCallback = callback;
  document.getElementById('custom-confirm-text').textContent = text;
  document.getElementById('custom-confirm-overlay').classList.add('show');
}
function customConfirmAnswer(ok) {
  document.getElementById('custom-confirm-overlay').classList.remove('show');
  const cb = _confirmCallback; _confirmCallback = null;
  if (cb) cb(ok);
}

let expandedSources = new Set();
let expandedSvyazkaGroups = new Set(['svgroup-dup-1', 'svgroup-uniq-5']);
function toggleSvyazkaGroup(key) {
  if (expandedSvyazkaGroups.has(key)) expandedSvyazkaGroups.delete(key); else expandedSvyazkaGroups.add(key);
  renderPanels();
}
function toggleSourceExpand(id) {
  if (expandedSources.has(id)) expandedSources.delete(id); else expandedSources.add(id);
  renderPanels();
}

function confirmChange(message, applyFn, revertFn) {
  customConfirm(message + '\n\nПрименится сразу к новым строкам. Продолжить?', async (ok) => {
    if (ok) {
      try {
        await applyFn();
        renderPanels();
        showToast(activeTab);
      } catch (err) {
        showError(err.message || 'Не удалось сохранить');
      }
    } else if (revertFn) {
      revertFn();
    }
  });
}
function confirmAktivenChange(id, name, checked, el) {
  const action = checked ? 'включить' : 'выключить';
  const warnText = checked ? '' : ' Приём новых строк от этого источника немедленно остановится (403 Forbidden).';
  confirmChange(
    `Вы хотите ${action} источник «${name}».${warnText}`,
    async () => {
      const data = await apiJSON('/rest/sources/' + id, {
        method: 'PATCH',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ is_active: checked }),
      });
      const row = DATA.sources.find(r => r.id === id);
      if (row) row.aktiven = data.is_active !== false;
    },
    () => { el.checked = !checked; }
  );
}
async function issueSourceToken(id) {
  const data = await apiJSON('/rest/sources/' + id + '/token', { method: 'POST' });
  if (!data.token) {
    throw new Error('Сервер не вернул токен');
  }
  const row = DATA.sources.find(s => s.id === id);
  if (row) row.token = data.token;
}

async function confirmGenerateToken(id) {
  try {
    await issueSourceToken(id);
    renderPanels();
    showToast(activeTab);
  } catch (err) {
    showError(err.message || 'Не удалось сгенерировать токен');
  }
}
function confirmRegenerateToken(id) {
  const row = DATA.sources.find(s => s.id === id);
  confirmChange(
    `Перегенерировать токен источника «${row ? row.nazvanie : ''}»? Старый токен перестанет работать — поставщику нужно будет передать новый.`,
    () => issueSourceToken(id)
  );
}
function confirmDeleteToken(id) {
  const row = DATA.sources.find(s => s.id === id);
  confirmChange(
    `Удалить токен источника «${row ? row.nazvanie : ''}»? Приём по этому токену остановится.`,
    async () => {
      await apiJSON('/rest/sources/' + id + '/token', { method: 'DELETE' });
      if (row) row.token = null;
    }
  );
}
function copyText(text, label) {
  if (!text) { showError((label||'Значение') + ' ещё не задано.'); return; }
  const ok = () => showCopyToast(label || 'Значение');
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(text).then(ok).catch(() => copyTextFallback(text, ok));
    return;
  }
  copyTextFallback(text, ok);
}
function copyTextFallback(text, done) {
  const ta = document.createElement('textarea');
  ta.value = text;
  ta.setAttribute('readonly', '');
  ta.style.position = 'fixed';
  ta.style.left = '-9999px';
  document.body.appendChild(ta);
  ta.select();
  try {
    document.execCommand('copy');
    if (done) done();
  } catch (err) {
    showError('Не удалось скопировать');
  }
  ta.remove();
}
function copyToken(token) { copyText(token, 'Токен'); }
function showCopyToast(label) {
  const t = document.getElementById('export-toast') || document.querySelector('.save-toast');
  // мини-уведомление прямо у курсора не делаем — используем общий toast текущей вкладки
  const el = document.getElementById('toast-' + activeTab);
  if (el) { el.textContent = label + ' скопирован(о) в буфер обмена.'; el.classList.add('show'); clearTimeout(el._timer); el._timer = setTimeout(()=>el.classList.remove('show'), 1800); }
}
const ICON_SVG = {
  copy: '<rect x="7" y="7" width="10" height="10" rx="1.5" stroke="currentColor" stroke-width="1.4"/><path d="M4 13V4a1 1 0 0 1 1-1h9" stroke="currentColor" stroke-width="1.4"/>',
  plus: '<path d="M10 4v12M4 10h12" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>',
  refresh: '<path d="M4.2 10a5.8 5.8 0 0 1 9.7-4.3L15.5 7.2M15.8 10a5.8 5.8 0 0 1-9.7 4.3L4.5 12.8" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/><path d="M11.8 7.2h3.7V3.6M8.2 12.8H4.5v3.6" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round"/>',
  trash: '<path d="M4 6h12M8 6V4h4v2M6 6l.7 10h6.6L14 6" stroke="currentColor" stroke-width="1.4" stroke-linecap="round" stroke-linejoin="round"/>',
  eye: '<path d="M2.2 10S5.4 4.5 10 4.5 17.8 10 17.8 10 14.6 15.5 10 15.5 2.2 10 2.2 10z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/><circle cx="10" cy="10" r="2.3" stroke="currentColor" stroke-width="1.5"/>',
  cloud: '<path d="M6.1 14.6h7.7a3.2 3.2 0 0 0 .35-6.38 4.15 4.15 0 0 0-8.05-1.15A3.15 3.15 0 0 0 6.1 14.6z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/>',
  search: '<circle cx="8.6" cy="8.6" r="4.4" stroke="currentColor" stroke-width="1.6"/><path d="M12 12.2L16.4 16.6" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>',
  download: '<path d="M10 3.4v9.2M6.6 9.4L10 12.8l3.4-3.4" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/><path d="M4 16.4h12" stroke="currentColor" stroke-width="1.6" stroke-linecap="round"/>',
  layers: '<rect x="5.5" y="8.2" width="11" height="7.5" rx="1.2" stroke="currentColor" stroke-width="1.4"/><path d="M3.5 6.2h11M7.5 3.8h11" stroke="currentColor" stroke-width="1.4" stroke-linecap="round"/>',
  hash: '<path d="M7.2 3.5L5.5 16.5M14.5 3.5L12.8 16.5M3.8 7.8h12.4M3.8 12.2h12.4" stroke="currentColor" stroke-width="1.5" stroke-linecap="round"/>',
  pin: '<path d="M10 17.2s-5.2-4.1-5.2-8a5.2 5.2 0 1 1 10.4 0c0 3.9-5.2 8-5.2 8z" stroke="currentColor" stroke-width="1.5" stroke-linejoin="round"/><circle cx="10" cy="9" r="1.8" stroke="currentColor" stroke-width="1.4"/>',
};
function metaIcon(name) {
  return `<svg width="12" height="12" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">${ICON_SVG[name]}</svg>`;
}
function onTokenAction(action, id) {
  id = Number(id);
  if (action === 'generate') confirmGenerateToken(id);
  else if (action === 'refresh') confirmRegenerateToken(id);
  else if (action === 'delete') confirmDeleteToken(id);
  else if (action === 'copy') {
    const row = DATA.sources.find(s => s.id === id);
    copyText(row && row.token ? receiveLeadUrl(row.token) : '', 'URL приёма');
  }
}
function iconBtn(name, title, onclick, kind) {
  const cls = kind ? `copy-icon-btn icon-btn-${kind}` : 'copy-icon-btn';
  return `<button type="button" class="${cls}" title="${title}" onclick="${onclick}">
    <svg width="14" height="14" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg">${ICON_SVG[name]}</svg>
  </button>`;
}
async function downloadCRMLeadDocs() {
  try {
    const res = await fetch(API + '/rest/docs/crm.lead.add.docx');
    const buf = await res.arrayBuffer();
    const bytes = new Uint8Array(buf);
    if (!res.ok || bytes.length < 4 || bytes[0] !== 0x50 || bytes[1] !== 0x4b) {
      showError('Документация недоступна');
      return;
    }
    const blob = new Blob([buf], { type: 'application/vnd.openxmlformats-officedocument.wordprocessingml.document' });
    const url = URL.createObjectURL(blob);
    const a = document.createElement('a');
    a.href = url;
    a.download = 'crm-lead-add.docx';
    document.body.appendChild(a);
    a.click();
    a.remove();
    URL.revokeObjectURL(url);
  } catch (err) {
    showError(err.message || 'Не удалось скачать документацию');
  }
}
function hintIconBtn(name, label, onclick) {
  return `<button type="button" class="hint-icon-btn" aria-label="${escAttr(label)}" onclick="${onclick}">
    <svg width="14" height="14" viewBox="0 0 20 20" fill="none" xmlns="http://www.w3.org/2000/svg" aria-hidden="true">${ICON_SVG[name]}</svg>
    <span class="hint-icon-label">${escAttr(label)}</span>
  </button>`;
}
function copyIconBtn(text, label) {
  const safe = String(text || '').replace(/\\/g, '\\\\').replace(/'/g, "\\'");
  return iconBtn('copy', 'Скопировать ' + (label || ''), `copyText('${safe}', '${label || 'Значение'}')`);
}
function confirmDeleteSource(id, name) {
  const kids = childrenOf(id);
  const extra = kids.length
    ? ` Вместе с ним удалятся ${kids.length} под-источник(а), правила дробления, связки Скорозвона и стоп-листы партнёров.`
    : ' Связанные правила дробления, связки Скорозвона и стоп-листы тоже удалятся.';
  confirmChange(
    `Удалить источник «${name}»?${extra} Строки архива останутся.`,
    async () => {
      await apiJSON('/rest/sources/' + id, { method: 'DELETE' });
      const gone = new Set([id, ...kids.map(k => k.id)]);
      DATA.sources = DATA.sources.filter(r => !gone.has(r.id));
      DATA.droblenie = DATA.droblenie.filter(r => !gone.has(r.source_id) && !gone.has(r.pod_source_id) && !gone.has(r.test_pod_source_id));
      DATA.svyazki = (DATA.svyazki || [])
        .map(s => ({ ...s, pod_source_ids: (s.pod_source_ids || []).filter(pid => !gone.has(pid)) }))
        .filter(s => s.pod_source_ids.length > 0);
      (DATA.upk || []).forEach(u => {
        u.stop_source_ids = (u.stop_source_ids || []).filter(sid => !gone.has(sid));
      });
      fetched.sources = fetched.splits = fetched.svyazki = fetched.partners = false;
    }
  );
}
function addSubSourceConfirm(rootId) {
  customPrompt('Название под-источника:', (name) => {
    if (!name) return;
    confirmChange(
      `Создать под-источник «${name}» у источника «${sourceName(rootId)}»?`,
      async () => {
        const created = await createChildSource(rootId, name);
        DATA.sources.push(mapSource(created));
        expandedSources.add(rootId);
      }
    );
  });
}
/* ===== МОДАЛКА ДРОБЛЕНИЯ: view → edit (draft) → apply/cancel ===== */
let drobModalSourceId = null;
let drobDraft = null; // черновик: массив правил, редактируется независимо от DATA до "Применить"
let drobDraftTeg = null; // { active, expected_value }
let drobEditing = false;

function openDroblenieModal(sourceId) {
  drobModalSourceId = sourceId;
  drobEditing = false;
  drobDraft = null;
  document.getElementById('drob-modal-title').textContent = sourceName(sourceId);
  document.getElementById('drob-overlay').classList.add('show');
  renderDroblenieModal();
}
function closeDroblenieModal(discard) {
  if (discard && drobEditing) {
    // черновик просто выбрасывается — DATA не тронута, старые правила остаются в силе
  }
  document.getElementById('drob-overlay').classList.remove('show');
  drobModalSourceId = null; drobDraft = null; drobEditing = false;
}
function enterDroblenieEditMode() {
  drobEditing = true;
  const root = DATA.sources.find(s => s.id === drobModalSourceId);
  drobDraft = DATA.droblenie.filter(d => d.source_id === drobModalSourceId).map(d => {
    const ch = DATA.sources.find(s => s.id === d.pod_source_id);
    return {
      ...d,
      metka_holodnaya: d.metka_holodnaya || ch?.metka_holodnaya || '',
      metka_kakaya: d.metka_kakaya || ch?.metka_kakaya || '',
    };
  });
  drobDraftTeg = { active: root.teg_active || false, expected_value: root.teg_expected_value || '' };
  renderDroblenieModal();
}
function draftAddRule() {
  const children = childrenOf(drobModalSourceId);
  if (children.length === 0) { showError('У этого источника нет под-источников. Сначала создайте их в справочнике «Источники» (кнопка «+ под-источник»).'); return; }
  const firstOp = DATA.operatorDict[0];
  drobDraft.push({
    id: 'draft-' + Date.now(),
    source_id: drobModalSourceId,
    operator_id: firstOp ? firstOp.id : 0,
    operator_group: firstOp ? firstOp.group : '',
    pod_source_id: children[0].id,
    test_pod_source_id: null,
    metka_holodnaya: children[0].metka_holodnaya || '',
    metka_kakaya: children[0].metka_kakaya || '',
    _new: true,
  });
  renderDroblenieModal();
}
function draftUpdateRule(idx, field, value) {
  drobDraft[idx][field] = value;
  renderDroblenieModal();
}
function draftDeleteRule(idx) {
  drobDraft.splice(idx, 1);
  renderDroblenieModal();
}
function draftUpdateSubProp(idx, field, value) {
  if (!drobDraft[idx]) return;
  drobDraft[idx][field] = value;
}
function draftUpdateTeg(field, value) { drobDraftTeg[field] = value; renderDroblenieModal(); }
function renderDroblenieModal() {
  const source = DATA.sources.find(s => s.id === drobModalSourceId);
  const children = childrenOf(drobModalSourceId);
  const rules = drobEditing ? drobDraft : DATA.droblenie.filter(d => d.source_id === drobModalSourceId);

  document.getElementById('drob-modal-sub').textContent = drobEditing ? 'Режим редактирования — черновик' : 'Просмотр текущих правил';
  document.getElementById('drob-edit-banner').classList.toggle('show', drobEditing);
  document.getElementById('drob-btn-edit').style.display = drobEditing ? 'none' : 'inline-block';
  document.getElementById('drob-edit-actions').style.display = drobEditing ? 'flex' : 'none';
  document.getElementById('drob-add-rule-btn').style.display = drobEditing ? 'inline-block' : 'none';

  const container = document.getElementById('drob-modal-rows');
  const emptyBox = document.getElementById('drob-modal-empty');
  emptyBox.style.display = rules.length ? 'none' : 'block';
  emptyBox.textContent = 'Правил дробления нет — строки останутся в источнике без разбивки по под-источнику. Нажмите «Редактировать», затем «+ Добавить правило».';

  if (!drobEditing) {
    const rows = rules.map(r => {
      const ch = DATA.sources.find(s => s.id === r.pod_source_id);
      return `<tr>
        <td>${r.operator_group || operatorName(r.operator_id)}</td>
        <td>${sourceName(r.pod_source_id)}</td>
        <td style="font-size:12px; color:var(--text-2);">${r.metka_holodnaya || ch?.metka_holodnaya || '—'} / ${r.metka_kakaya || ch?.metka_kakaya || '—'}</td>
        <td style="font-size:12px; color:var(--text-2);">${r.test_pod_source_id ? sourceName(r.test_pod_source_id) : '—'}</td>
      </tr>`;
    }).join('');
    const tegView = source.teg_active
      ? `<div class="impact-note" style="margin-bottom:14px;"><b>Проверка тега активна.</b> Ожидаемое значение: <code>${source.teg_expected_value || '—'}</code>. Если не совпадает — строка уходит в тестовый под-источник этого правила (см. колонку «Тест» ниже).</div>`
      : '';
    container.innerHTML = (tegView + (rules.length ? `
      <table><thead><tr><th>Группа оператора</th><th>Под-источник</th><th>Холодная / Какая база</th><th>Тестовый под-источник</th></tr></thead>
      <tbody>${rows}</tbody></table>` : ''));
    return;
  }

  // редактирование — блок проверки тега + карточки правил
  const cGrpAll = drobDraft.map((r, idx) => idx);
  const tegBlock = `
    <div class="drob-rule-card" style="background:#fdf8ee;">
      <div class="drob-rule-card-head"><span class="drob-rule-num" style="color:#8c5a1f;">Проверка тега (teg)</span></div>
      <div class="drob-rule-row" style="align-items:end;">
        <div class="drob-rule-field">
          <label>Активна ${helpIcon('Если включено — после определения под-источника по оператору дополнительно проверяется поле teg входящей строки. Совпало с ожидаемым значением — строка идёт как обычно. Не совпало или пусто — переключается на тестовый под-источник этого же правила.')}</label>
          <label class="toggle"><input type="checkbox" ${drobDraftTeg.active?'checked':''} onchange="draftUpdateTeg('active', this.checked)"><span class="toggle-slider"></span></label>
        </div>
        <div class="drob-rule-field">
          <label>Ожидаемое значение</label>
          <input class="cell-input" value="${drobDraftTeg.expected_value}" placeholder="например: подтверждён" ${drobDraftTeg.active?'':'disabled'} onchange="draftUpdateTeg('expected_value', this.value)">
        </div>
      </div>
      ${drobDraftTeg.active ? '<p class="drob-rule-hint">Укажите тестовый под-источник в каждом правиле ниже — по одному на каждую группу оператора, как в Bitrix.</p>' : ''}
    </div>`;

  container.innerHTML = tegBlock + rules.map((r, idx) => {
    const cGrp = 'draft-grp-' + idx, cPod = 'draft-pod-' + idx, cTest = 'draft-test-' + idx;
    registerCombo(cGrp, DATA.operatorDict.map(g => ({ value:g.id, label:g.group })), v => {
      const id = parseInt(v, 10);
      drobDraft[idx].operator_id = id;
      drobDraft[idx].operator_group = operatorName(id);
      renderDroblenieModal();
    });
    registerCombo(cPod, children.map(s => ({ value:s.id, label:s.nazvanie })), v => draftUpdateRule(idx, 'pod_source_id', parseInt(v)));
    if (drobDraftTeg.active) registerCombo(cTest, children.map(s => ({ value:s.id, label:s.nazvanie })), v => draftUpdateRule(idx, 'test_pod_source_id', parseInt(v)));
    return `
      <div class="drob-rule-card">
        <div class="drob-rule-card-head">
          <span class="drob-rule-num">Правило ${idx + 1}</span>
          <button class="del-btn" onclick="draftDeleteRule(${idx})" title="Удалить правило">&times;</button>
        </div>
        <div class="drob-rule-section-label">Условие: группа оператора → под-источник</div>
        <div class="drob-rule-row">
          <div class="drob-rule-field">
            <label>Группа оператора</label>
            ${comboHTML(cGrp, r.operator_group || operatorName(r.operator_id), 'Группа оператора')}
          </div>
          <div class="drob-rule-field">
            <label>Под-источник</label>
            ${comboHTML(cPod, sourceName(r.pod_source_id), 'Под-источник')}
          </div>
        </div>
        ${drobDraftTeg.active ? `
        <div class="drob-rule-row" style="padding-top:0;">
          <div class="drob-rule-field" style="grid-column: span 2;">
            <label>Тестовый под-источник для этой группы ${helpIcon('Куда уходит строка, если тег не совпал с ожидаемым значением или его нет.')}</label>
            ${comboHTML(cTest, r.test_pod_source_id ? sourceName(r.test_pod_source_id) : '', 'Выбрать тестовый под-источник')}
          </div>
        </div>` : ''}
        <div class="drob-rule-section-label" style="padding-top:14px;">Свойства этого под-источника</div>
        <div class="drob-rule-metki">
          <div class="drob-rule-field">
            <label>Холодная база</label>
            <input class="cell-input" value="${r.metka_holodnaya||''}" onchange="draftUpdateSubProp(${idx}, 'metka_holodnaya', this.value)">
          </div>
          <div class="drob-rule-field">
            <label>Какая база</label>
            <input class="cell-input" value="${r.metka_kakaya||''}" onchange="draftUpdateSubProp(${idx}, 'metka_kakaya', this.value)">
          </div>
        </div>
      </div>`;
  }).join('');
}
function applyDroblenieDraft() {
  const before = DATA.droblenie.filter(d => d.source_id === drobModalSourceId);
  const beforeText = before.length ? before.map(r => `${r.operator_group} → ${sourceName(r.pod_source_id)}`).join('; ') : 'правил не было';
  const afterText = drobDraft.length ? drobDraft.map(r => `${r.operator_group} → ${sourceName(r.pod_source_id)}`).join('; ') : 'правил не будет';

  if (drobDraftTeg.active) {
    if (!drobDraftTeg.expected_value.trim()) { showError('Укажите ожидаемое значение тега, или выключите проверку.'); return; }
    const missing = drobDraft.filter(r => !r.test_pod_source_id);
    if (missing.length) { showError('Проверка тега активна — у каждого правила должен быть указан тестовый под-источник.'); return; }
  }

  customConfirm(
    `Применить изменения дробления для «${sourceName(drobModalSourceId)}»?\n\n` +
    `Было: ${beforeText}\nСтанет: ${afterText}\n\n` +
    `Метки по под-источникам тоже будут обновлены.` +
    (drobDraftTeg.active ? `\nПроверка тега: активна, ожидаемое значение «${drobDraftTeg.expected_value}».` : '') +
    `\nНовые правила начнут действовать в проде сразу после подтверждения.`,
    async (ok) => {
      if (!ok) return;
      try {
        await apiJSON('/rest/sources/' + drobModalSourceId + '/split', {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            teg_active: !!drobDraftTeg.active,
            teg_expected_value: drobDraftTeg.expected_value || '',
            rules: drobDraft.map(r => ({
                operator_id: r.operator_id,
                child_source_id: r.pod_source_id,
                test_child_source_id: r.test_pod_source_id || null,
                cold_label: r.metka_holodnaya || '',
                base_label: r.metka_kakaya || '',
            })),
          }),
        });
        await loadSources();
        await loadSplits();
        closeDroblenieModal(false);
        renderPanels();
        showToast(activeTab);
      } catch (err) {
        showError(err.message || 'Не удалось сохранить дробление');
      }
    }
  );
}

/* ===== МОДАЛКА ПАРТНЁРА: view → edit (draft) → apply/cancel ===== */
let partnerModalId = null;
let partnerDraft = null; // {aktiven, webhook_url, dedup_days, regions:[], stops:[]}
let partnerEditing = false;

function openPartnerModal(upkId) {
  partnerModalId = upkId;
  partnerEditing = false;
  partnerDraft = null;
  const u = DATA.upk.find(x => x.id === upkId);
  document.getElementById('partner-modal-title').textContent = u.nazvanie;
  document.getElementById('partner-overlay').classList.add('show');
  renderPartnerModal();
}
function closePartnerModal(discard) {
  document.getElementById('partner-overlay').classList.remove('show');
  partnerModalId = null; partnerDraft = null; partnerEditing = false;
}
let partnerRegionSearch = '';
function renderPartnerRegionListHTML() {
  const filtered = (DATA.regions || []).filter(r => r.name.toLowerCase().includes(partnerRegionSearch.toLowerCase()));
  return filtered.map(r => `
    <label style="display:flex; align-items:center; gap:8px; padding:4px 0; font-size:13px; cursor:pointer;">
      <input type="checkbox" ${partnerDraft.regions.includes(r.id)?'checked':''} onchange="draftPartnerRegionToggle(${r.id}, this.checked)">
      ${r.name}
    </label>`).join('') || '<div style="color:var(--text-muted); font-size:12px; padding:4px 0;">Ничего не найдено</div>';
}
function partnerRegionSummaryHTML() {
  return partnerDraft.regions.length ? 'Выбрано: ' + partnerDraft.regions.map(regionName).join(', ') : 'Ничего не отмечено — регион не проверяется';
}
function setPartnerRegionSearch(v) {
  partnerRegionSearch = v;
  // точечное обновление — НЕ трогаем сам инпут, иначе слетает фокус после первого символа
  const list = document.getElementById('partner-region-list');
  if (list) list.innerHTML = renderPartnerRegionListHTML();
}
function enterPartnerEditMode() {
  const u = DATA.upk.find(x => x.id === partnerModalId);
  partnerEditing = true;
  partnerRegionSearch = '';
  partnerDraft = {
    aktiven: u.aktiven,
    webhook_url: u.webhook_url,
    regions: (u.region_ids || []).slice(),
    stops: (u.stop_source_ids || []).slice(),
    zarya_results: (u.zarya_results || []).slice(),
    body: partnerBodyPairs(u.body),
    content_type: u.content_type || 'application/x-www-form-urlencoded',
  };
  renderPartnerModal();
}
function draftPartnerField(field, value) { partnerDraft[field] = value; }
function draftPartnerRegionToggle(regionId, checked) {
  regionId = parseInt(regionId, 10);
  if (checked) { if (!partnerDraft.regions.includes(regionId)) partnerDraft.regions.push(regionId); }
  else { partnerDraft.regions = partnerDraft.regions.filter(r => r !== regionId); }
  const summary = document.getElementById('partner-region-summary');
  if (summary) summary.innerHTML = partnerRegionSummaryHTML();
}
function draftPartnerRegionAdd(region) { if (region && !partnerDraft.regions.includes(region)) { partnerDraft.regions.push(region); renderPartnerModal(); } }
function draftPartnerRegionRemove(region) { partnerDraft.regions = partnerDraft.regions.filter(r => r !== region); renderPartnerModal(); }
function draftPartnerStopAdd(sourceId) { if (!partnerDraft.stops.includes(sourceId)) { partnerDraft.stops.push(sourceId); renderPartnerModal(); } }
function draftPartnerStopRemove(sourceId) { partnerDraft.stops = partnerDraft.stops.filter(s => s !== sourceId); renderPartnerModal(); }
function partnerBodyPairs(body) {
  const keys = Object.keys(body || {});
  if (!keys.length) return [{ key: '', value: '' }];
  return keys.map(k => ({ key: k, value: body[k] == null ? '' : String(body[k]) }));
}
function partnerBodyObject(pairs) {
  const out = {};
  (pairs || []).forEach(p => {
    const k = String(p.key || '').trim();
    if (k) out[k] = p.value == null ? '' : String(p.value);
  });
  return out;
}
function draftPartnerBodyField(i, field, value) {
  if (!partnerDraft.body[i]) partnerDraft.body[i] = { key: '', value: '' };
  partnerDraft.body[i][field] = value;
}
function draftPartnerBodyAdd() {
  partnerDraft.body.push({ key: '', value: '' });
  renderPartnerModal();
}
function draftPartnerBodyRemove(i) {
  partnerDraft.body.splice(i, 1);
  if (!partnerDraft.body.length) partnerDraft.body.push({ key: '', value: '' });
  renderPartnerModal();
}
function partnerBodyViewHTML(body) {
  const entries = Object.entries(body || {});
  if (!entries.length) return '<span class="cell-static">нет</span>';
  return `<div class="partner-body-view">${entries.map(([k, v]) =>
    `<div class="partner-body-line"><code>${escAttr(k)}</code><span>=</span><span>${escAttr(String(v))}</span></div>`
  ).join('')}</div>`;
}
function draftPartnerZaryaToggle(value, checked) {
  if (checked) { if (!partnerDraft.zarya_results.includes(value)) partnerDraft.zarya_results.push(value); }
  else { partnerDraft.zarya_results = partnerDraft.zarya_results.filter(v => v !== value); }
  renderPartnerModal();
}

function renderPartnerModal() {
  const u = DATA.upk.find(x => x.id === partnerModalId);
  document.getElementById('partner-modal-sub').textContent = partnerEditing ? 'Режим редактирования — черновик' : 'Просмотр текущих условий';
  document.getElementById('partner-edit-banner').classList.toggle('show', partnerEditing);
  document.getElementById('partner-btn-edit').style.display = partnerEditing ? 'none' : 'inline-block';
  document.getElementById('partner-edit-actions').style.display = partnerEditing ? 'flex' : 'none';

  const box = document.getElementById('partner-modal-body-content');

  if (!partnerEditing) {
    const regions = (u.region_ids || []).map(regionName);
    const stops = (u.stop_source_ids || []).map(sourceName);
    box.innerHTML = `
      <div class="partner-field"><label>Активен</label><span class="cell-static">${u.aktiven ? 'да' : 'нет — полностью отключён'}</span></div>
      <div class="partner-field"><label>Webhook URL</label>
        <span style="display:flex; align-items:center; gap:6px;">
          <span class="cell-static" style="font-family:monospace; font-size:12px; word-break:break-all;">${u.webhook_url || '—'}</span>
          ${copyIconBtn(u.webhook_url, 'Webhook URL')}
        </span>
      </div>
      <div class="partner-field"><label>Что принимает</label><span class="cell-static">${zaryaSummaryLabel(u.zarya_results)}</span></div>
      <div class="partner-field"><label>Регион</label><span class="cell-static">${regions.length ? regions.join(', ') : 'не проверяется'}</span></div>
      <div class="partner-field"><label>Стоп-источники</label><span class="cell-static">${stops.length ? stops.join(', ') : 'нет'}</span></div>
      <div class="partner-field"><label>Параметры (GET + POST)</label>${partnerBodyViewHTML(u.body)}</div>
      <div class="partner-field"><label>Content-Type</label><span class="cell-static" style="font-family:monospace; font-size:12px;">${escAttr(u.content_type || 'application/x-www-form-urlencoded')}</span></div>
    `;
    return;
  }

  const cAddStop = 'draft-add-stop';
  registerCombo(cAddStop, allSourceNamesFlat().filter(s => !partnerDraft.stops.includes(s.id)).map(s => ({ value:s.id, label:s.label })), v => draftPartnerStopAdd(parseInt(v)));

  box.innerHTML = `
    <div class="partner-field">
      <label>Активен ${helpIcon('Полное отключение покупателя — строки не уходят вообще, независимо от региона и стоп-источников.')}</label>
      <label class="toggle"><input type="checkbox" ${partnerDraft.aktiven?'checked':''} onchange="draftPartnerField('aktiven', this.checked)"><span class="toggle-slider"></span></label>
    </div>
    <div class="partner-field">
      <label>Webhook URL ${helpIcon('Куда уходит запрос при отправке строки этому покупателю.')}</label>
      <input class="cell-input" style="border:1px solid var(--border-strong); font-family:monospace; font-size:12px;" value="${escAttr(partnerDraft.webhook_url || '')}" onchange="draftPartnerField('webhook_url', this.value)">
    </div>
    <div class="partner-field">
      <label>Что принимает ${helpIcon('Результат проверки по Заре. Можно отметить несколько вариантов сразу.')}</label>
      <div class="tag-list">
        ${ZARYA_OPTIONS.map(([v,label]) => `
          <label style="display:inline-flex; align-items:center; gap:4px; font-size:12.5px; background:#f0efeb; border:1px solid var(--border-strong); border-radius:14px; padding:3px 10px 3px 6px;">
            <input type="checkbox" ${partnerDraft.zarya_results.includes(v)?'checked':''} onchange="draftPartnerZaryaToggle('${v}', this.checked)">
            ${label}
          </label>`).join('')}
      </div>
      ${partnerDraft.zarya_results.length === 0 ? '<div style="font-size:11px; color:var(--danger); margin-top:4px;">Не выбрано ни одного значения — этому покупателю ничего не будет уходить.</div>' : ''}
    </div>
    <div class="partner-field">
      <label>Регион ${helpIcon('Сужает список разрешённых регионов. Пустой список = регион не проверяется.')}</label>
      <input type="text" class="cell-input" id="partner-region-search-input" style="border:1px solid var(--border-strong); margin-bottom:6px;" placeholder="Поиск региона..." value="${partnerRegionSearch}" oninput="setPartnerRegionSearch(this.value)">
      <div id="partner-region-list" style="max-height:170px; overflow-y:auto; border:1px solid var(--border); border-radius:6px; padding:6px 10px;">${renderPartnerRegionListHTML()}</div>
      <div id="partner-region-summary" style="font-size:11.5px; color:var(--text-muted); margin-top:5px;">${partnerRegionSummaryHTML()}</div>
    </div>
    <div class="partner-field">
      <label>Стоп-источники ${helpIcon('Строки этого источника не уходят покупателю, даже если они дубль и регион подходит.')}</label>
      <div class="tag-list">
        ${partnerDraft.stops.map(sid => `<span class="tag">${sourceName(sid)}<span class="tag-x" onclick="draftPartnerStopRemove(${sid})">&times;</span></span>`).join('') || '<span style="color:var(--text-muted); font-size:12px;">нет</span>'}
      </div>
      <div style="margin-top:6px; width:220px;">${comboHTML(cAddStop, '', '+ добавить стоп-источник')}</div>
    </div>
    <div class="partner-field">
      <label>Параметры (GET + POST) ${helpIcon('Одни и те же пары уходят и в query URL, и в тело POST. Плейсхолдеры как в БД: {{phone}}, {{phone_valid}}, {{region}} (название), {{city}}.')}</label>
      <div class="partner-body-rows">
        ${(partnerDraft.body || []).map((row, i) => `
          <div class="partner-body-row">
            <input class="cell-input" style="border:1px solid var(--border-strong); font-family:monospace; font-size:12px;" placeholder="ключ" value="${escAttr(row.key)}" onchange="draftPartnerBodyField(${i}, 'key', this.value)">
            <input class="cell-input" style="border:1px solid var(--border-strong); font-family:monospace; font-size:12px;" placeholder="значение" value="${escAttr(row.value)}" onchange="draftPartnerBodyField(${i}, 'value', this.value)">
            <button type="button" class="del-btn" title="Удалить" onclick="draftPartnerBodyRemove(${i})">&times;</button>
          </div>`).join('')}
      </div>
      <button type="button" class="btn btn-sm" style="margin-top:6px;" onclick="draftPartnerBodyAdd()">+ параметр</button>
    </div>
    <div class="partner-field">
      <label>Content-Type ${helpIcon('Заголовок тела POST. Для текущих партнёров — application/x-www-form-urlencoded.')}</label>
      <input class="cell-input" style="border:1px solid var(--border-strong); font-family:monospace; font-size:12px;" value="${escAttr(partnerDraft.content_type || 'application/x-www-form-urlencoded')}" onchange="draftPartnerField('content_type', this.value)">
    </div>
  `;
}

function applyPartnerDraft() {
  const u = DATA.upk.find(x => x.id === partnerModalId);
  const beforeRegions = (u.region_ids || []).map(regionName);
  const afterRegions = partnerDraft.regions.map(regionName);
  const beforeStops = (u.stop_source_ids || []).map(sourceName);
  const afterStops = partnerDraft.stops.map(sourceName);

  const changes = [];
  if (u.aktiven !== partnerDraft.aktiven) changes.push(`Активен: ${u.aktiven?'да':'нет'} → ${partnerDraft.aktiven?'да':'нет'}`);
  const beforeZarya = u.zarya_results || [];
  if (JSON.stringify(beforeZarya.slice().sort()) !== JSON.stringify(partnerDraft.zarya_results.slice().sort())) changes.push(`Что принимает: [${zaryaSummaryLabel(beforeZarya)}] → [${zaryaSummaryLabel(partnerDraft.zarya_results)}]`);
  if (JSON.stringify(beforeRegions) !== JSON.stringify(afterRegions)) changes.push(`Регионы: [${beforeRegions.join(', ')||'—'}] → [${afterRegions.join(', ')||'—'}]`);
  if (JSON.stringify(beforeStops.sort()) !== JSON.stringify(afterStops.slice().sort())) changes.push(`Стоп-источники: [${beforeStops.join(', ')||'—'}] → [${afterStops.join(', ')||'—'}]`);
  const beforeBody = JSON.stringify(u.body || {});
  const afterBody = JSON.stringify(partnerBodyObject(partnerDraft.body));
  if (beforeBody !== afterBody) changes.push('Параметры запроса изменены');
  const beforeCT = u.content_type || 'application/x-www-form-urlencoded';
  const afterCT = partnerDraft.content_type || 'application/x-www-form-urlencoded';
  if (beforeCT !== afterCT) changes.push(`Content-Type: ${beforeCT} → ${afterCT}`);

  customConfirm(
    `Применить изменения для покупателя «${u.nazvanie}»?\n\n` +
    (changes.length ? changes.join('\n') : 'Изменений нет.') +
    `\n\nНовые условия начнут действовать в проде сразу после подтверждения.`,
    async (confirmed) => {
      if (!confirmed) return;
      try {
        await apiJSON('/rest/partners/' + u.id, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name: u.nazvanie,
            webhook_url: partnerDraft.webhook_url || '',
            is_active: !!partnerDraft.aktiven,
            zarya_results: partnerDraft.zarya_results || [],
            region_ids: partnerDraft.regions || [],
            stop_source_ids: partnerDraft.stops || [],
            body: partnerBodyObject(partnerDraft.body),
            content_type: partnerDraft.content_type || 'application/x-www-form-urlencoded',
          }),
        });
        await loadPartners();
        closePartnerModal(false);
        renderPanels();
        showToast(activeTab);
      } catch (err) {
        showError(err.message || 'Не удалось сохранить партнёра');
      }
    }
  );
}

function toggleUpkPermission(sourceId, upk, checked) {
  let row = DATA.sourceUpk.find(r => r.source_id === sourceId && r.upk === upk);
  if (row) row.razreshen = checked;
  else DATA.sourceUpk.push({ id: nextId.sourceUpk++, source_id: sourceId, upk, razreshen: checked });
  renderPanels();
  showToast(activeTab);
}
function addStopSource(upkName, sourceId) {
  let row = DATA.sourceUpk.find(r => r.source_id === sourceId && r.upk === upkName);
  if (row) row.razreshen = false;
  else DATA.sourceUpk.push({ id: nextId.sourceUpk++, source_id: sourceId, upk: upkName, razreshen: false });
  renderPanels();
  showToast(activeTab);
}
function removeStopSource(upkName, sourceLabel) {
  DATA.sourceUpk = DATA.sourceUpk.filter(r => !(r.upk === upkName && sourceName(r.source_id) === sourceLabel && r.razreshen === false));
  renderPanels();
  showToast(activeTab);
}
function addMediaplanRegion(upkName, region) {
  if (!region) return;
  DATA.mediaplans.push({ id: nextId.mediaplans++, upk: upkName, region });
  renderPanels();
  showToast(activeTab);
}
function removeMediaplanRegion(upkName, region) {
  DATA.mediaplans = DATA.mediaplans.filter(m => !(m.upk === upkName && m.region === region));
  renderPanels();
  showToast(activeTab);
}
function toggleRegionList(upkId) {
  const el = document.getElementById('regionlist-' + upkId);
  if (el) el.classList.toggle('show');
}
function addTag(entity, id, value) {
  if (!value.trim()) return;
  const row = DATA[entity].find(r => r.id === id);
  if (row) row.raw.push(value.trim());
  renderPanels();
  showToast(activeTab);
}
function removeTag(entity, id, idx) {
  const row = DATA[entity].find(r => r.id === id);
  if (row) row.raw.splice(idx, 1);
  renderPanels();
  showToast(activeTab);
}
function escAttr(s) {
  return String(s).replace(/[&<>"']/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
}
function unmappedOperatorHTML(r) {
  const values = r.unmapped || [];
  const opts = DATA.operatorDict
    .filter(g => g.group !== 'ДРУГИЕ')
    .map(g => `<option value="${g.id}">${escAttr(g.group)}</option>`)
    .join('');
  if (!values.length) {
    return '<span style="color:var(--text-muted); font-size:12px;">\u2014 несмапленных сырых значений пока нет. Сюда автоматически попадает любое значение оператора, не найденное ни в одной другой группе.</span>';
  }
  return `<div class="tag-list">
      ${values.map(v => `<span class="tag tag-unmapped" data-alias="${escAttr(v)}" title="Сырое значение, которое ушло в ДРУГИЕ">${escAttr(v)}<select class="tag-map-select" onchange="mapUnmappedOperator(this)"><option value="">в группу\u2026</option>${opts}</select></span>`).join('')}
    </div>
    <div class="unmapped-hint">Эти значения пришли от поставщика или определителя и не совпали ни с одной группой. Выберите группу — добавим в верхний маппинг. Уже принятые строки не пересчитываются.</div>`;
}
async function mapUnmappedOperator(sel) {
  const id = Number(sel && sel.value);
  const alias = sel && sel.closest && sel.closest('.tag') && sel.closest('.tag').dataset.alias;
  if (!id || !alias) return;
  try {
    await addOperatorAlias(id, alias);
  } catch (e) {
    sel.value = '';
    showError(e.message || e);
  }
}
async function addOperatorAlias(id, value) {
  value = String(value || '').trim();
  if (!value) return;
  await apiJSON('/rest/operators/' + id + '/aliases', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ alias: value }),
  });
  await loadOperators();
  renderPanels();
  showToast(activeTab);
}
async function removeOperatorAlias(id, idx) {
  const row = DATA.operatorDict.find(r => r.id === id);
  const alias = row && row.raw[idx];
  if (!alias) return;
  await apiJSON('/rest/operators/' + id + '/aliases?alias=' + encodeURIComponent(alias), { method: 'DELETE' });
  await loadOperators();
  renderPanels();
  showToast(activeTab);
}

function addSourceForm() {
  customPrompt('Название нового источника:', (name) => {
    if (!name) return;
    confirmChange(
      `Создать источник «${name}»?`,
      async () => {
        const created = await createParentSource(name);
        DATA.sources.push(mapSource(created));
      }
    );
  });
}
function addDroblenie() { const s = rootSources()[0]; DATA.droblenie.push({ id:nextId.droblenie++, source_id:s.id, operator_group:operatorGroups()[0], pod_source_id:(childrenOf(s.id)[0]||s).id }); renderPanels(); }
function addSourceUpk() { DATA.sourceUpk.push({ id:nextId.sourceUpk++, source_id:rootSources()[0].id, upk:UPK_LIST[0], razreshen:true }); renderPanels(); }

/* ===== МОДАЛКА СВЯЗКИ: создание (сразу edit) / просмотр → редактирование (draft) → apply/cancel ===== */
function svyazkaKind(s) { return s.pod_source_ids.length <= 1 ? 'standard' : 'custom'; }
function svyazkaRootId(s) {
  // источник для группировки — родитель первого под-источника
  if (!s.pod_source_ids.length) return null;
  const ps = DATA.sources.find(x => x.id === s.pod_source_ids[0]);
  return ps ? ps.roditel_id : null;
}
function svyazkaCardHTML(s) {
  const podNames = s.pod_source_ids.map(sourceName).join(', ') || '\u2014';
  const regionLabel = (s.region_ids || []).length ? s.region_ids.length + ' в списке' : 'все';
  return `
  <div class="partner-card">
    <div class="partner-card-head">
      <span style="font-size:15px; font-weight:600; display:flex; align-items:center; gap:6px; min-width:0; word-break:break-word;">${s.nazvanie} <span style="font-size:11px; font-weight:500; color:var(--text-muted);">${svyazkaProviderLabel(s)}</span> <button class="edit-pencil-btn" title="Переименовать" onclick="renameSvyazka('${svyazkaProvider(s)}', ${s.id})">\u270E</button></span>
      <span class="status-pill ${svyazkaKind(s)==='standard' ? 'on' : 'off'}">${svyazkaKind(s)==='standard' ? 'Стандартная' : 'Кастомная'}</span>
    </div>
    <div class="partner-meta">
      <div class="partner-meta-item">
        <span class="partner-meta-label">${metaIcon('layers')} Под-источники</span>
        <span class="partner-meta-value">${podNames}</span>
      </div>
      <div class="partner-meta-item">
        <span class="partner-meta-label">${metaIcon('hash')} Проект</span>
        <span class="partner-meta-value" style="font-family:monospace; font-size:11.5px;">${s.project_id || '\u2014'}</span>
      </div>
      <div class="partner-meta-item">
        <span class="partner-meta-label">${metaIcon('pin')} Регион</span>
        <span class="partner-meta-value">${regionLabel}</span>
      </div>
    </div>
    <div class="partner-card-actions">
      <button class="btn btn-sm partner-card-action" onclick="openSvyazkaModal('${svyazkaProvider(s)}', ${s.id})">Настроить</button>
      <button class="btn btn-sm btn-danger partner-card-action" onclick="confirmDeleteSvyazka('${svyazkaProvider(s)}', ${s.id})">Удалить</button>
    </div>
  </div>`;
}
function svyazkiTabRenderer(type) {
  const list = DATA.svyazki.filter(s => s.type === type);
  const standard = list.filter(s => svyazkaKind(s) === 'standard');
  const custom = list.filter(s => svyazkaKind(s) === 'custom');

  const groupedByRoot = {};
  standard.forEach(s => {
    const rootId = svyazkaRootId(s);
    if (!groupedByRoot[rootId]) groupedByRoot[rootId] = [];
    groupedByRoot[rootId].push(s);
  });

  const standardHTML = Object.keys(groupedByRoot).map(rootId => {
    const root = DATA.sources.find(x => x.id === parseInt(rootId));
    const items = groupedByRoot[rootId];
    const key = 'svgroup-' + type + '-' + rootId;
    const isOpen = expandedSvyazkaGroups.has(key);
    return `
    <div class="accordion-item">
      <div class="accordion-head" onclick="toggleSvyazkaGroup('${key}')">
        <span class="accordion-arrow ${isOpen?'open':''}">\u203a</span>
        <span style="font-weight:600;">${root ? root.nazvanie : '\u2014'}</span>
        <span style="color:var(--text-muted); font-size:12px; margin-left:8px;">${items.length} связ${items.length===1?'ка':(items.length>=2&&items.length<=4?'ки':'ок')}</span>
      </div>
      <div class="accordion-body ${isOpen?'show':''}">
        <div class="partner-cards">${items.map(svyazkaCardHTML).join('')}</div>
      </div>
    </div>`;
  }).join('');

  const label = type === 'dup' ? 'дублей' : 'уникальных строк';
  return {
    customHTML: `
      <div class="partners-lead-note">
        Связка = набор под-источников (1 \u2014 стандартная, 2+ \u2014 кастомная) + необязательный фильтр по региону (пусто = подходят все) \u2192 один проект Скорозвона или CRMCalls для ${label}. Площадку выбираете при создании связки. Если строка подходит под условия нескольких связок сразу \u2014 уходит во все параллельно. Если не подошла ни под одну \u2014 просто остаётся в БД, без доп. отправки.
      </div>
      <div style="font-size:13px; font-weight:600; margin-bottom:8px;">Стандартные (по источникам)</div>
      ${standardHTML || '<p style="color:var(--text-muted); font-size:13px; margin-bottom:16px;">Стандартных связок пока нет.</p>'}
      <div style="font-size:13px; font-weight:600; margin:18px 0 8px; padding-top:14px; border-top:1px solid var(--border);">Кастомные</div>
      <div class="partner-cards">${custom.map(svyazkaCardHTML).join('') || '<p style="color:var(--text-muted); font-size:13px;">Кастомных связок пока нет.</p>'}</div>
    `,
    addFn: 'openSvyazkaModal', addLabel: 'Добавить связку'
  };
}

let svyazkaModalId = null; // null = создание новой
let svyazkaModalProvider = 'skorozvon';
let svyazkaDraft = null;
let svyazkaEditing = false;

function openSvyazkaModal(provider, id) {
  if (arguments.length === 1 && typeof provider !== 'string') {
    id = provider;
    provider = 'skorozvon';
  }
  if (id === undefined || id === null || id === '') {
    svyazkaModalId = null;
    svyazkaModalProvider = 'skorozvon';
  } else {
    svyazkaModalId = Number(id);
    svyazkaModalProvider = provider === 'crmcalls' ? 'crmcalls' : 'skorozvon';
  }
  svyazkaEditing = false;
  svyazkaDraft = null;
  document.getElementById('svyazka-overlay').classList.add('show');
  if (svyazkaModalId === null) {
    document.getElementById('svyazka-modal-title').textContent = activeTab === 'dupWork' ? 'Новая связка (дубли)' : 'Новая связка (уники)';
    enterSvyazkaEditMode();
  } else {
    const s = findSvyazka(svyazkaModalProvider, svyazkaModalId);
    document.getElementById('svyazka-modal-title').textContent = s ? s.nazvanie : 'Связка';
    renderSvyazkaModal();
  }
}
function closeSvyazkaModal(discard) {
  document.getElementById('svyazka-overlay').classList.remove('show');
  svyazkaModalId = null; svyazkaModalProvider = 'skorozvon'; svyazkaDraft = null; svyazkaEditing = false;
}
function enterSvyazkaEditMode() {
  svyazkaEditing = true;
  if (svyazkaModalId === null) {
    svyazkaDraft = { nazvanie: '', provider: 'skorozvon', pod_source_ids: [], project_id: '', region_ids: [], type: (activeTab === 'uniqWork' ? 'uniq' : 'dup') };
  } else {
    const s = findSvyazka(svyazkaModalProvider, svyazkaModalId);
    svyazkaDraft = { nazvanie: s.nazvanie, provider: svyazkaProvider(s), pod_source_ids: s.pod_source_ids.slice(), project_id: s.project_id, region_ids: (s.region_ids || []).slice(), type: s.type };
  }
  document.getElementById('svyazka-apply-btn').textContent = svyazkaModalId === null ? 'Создать связку' : 'Применить изменения';
  renderSvyazkaModal();
}
function draftSvyazkaProvider(value) {
  const next = value === 'crmcalls' ? 'crmcalls' : 'skorozvon';
  if (svyazkaDraft.provider === next) return;
  svyazkaDraft.provider = next;
  svyazkaDraft.project_id = '';
  renderSvyazkaModal();
}
function draftSvyazkaField(field, value) { svyazkaDraft[field] = value; }
function draftSvyazkaPodSourceAdd(id) { if (!svyazkaDraft.pod_source_ids.includes(id)) { svyazkaDraft.pod_source_ids.push(id); renderSvyazkaModal(); } }
function draftSvyazkaPodSourceRemove(id) { svyazkaDraft.pod_source_ids = svyazkaDraft.pod_source_ids.filter(x => x !== id); renderSvyazkaModal(); }
function draftSvyazkaRegionAdd(regionId) { if (regionId && !svyazkaDraft.region_ids.includes(regionId)) { svyazkaDraft.region_ids.push(regionId); renderSvyazkaModal(); } }
function draftSvyazkaRegionRemove(regionId) { svyazkaDraft.region_ids = svyazkaDraft.region_ids.filter(r => r !== regionId); renderSvyazkaModal(); }

function renderSvyazkaModal() {
  const box = document.getElementById('svyazka-modal-body-content');
  document.getElementById('svyazka-modal-sub').textContent = svyazkaEditing ? (svyazkaModalId===null ? 'Создание новой связки' : 'Режим редактирования — черновик') : 'Просмотр';
  document.getElementById('svyazka-edit-banner').classList.toggle('show', svyazkaEditing && svyazkaModalId !== null);
  document.getElementById('svyazka-btn-edit').style.display = (!svyazkaEditing && svyazkaModalId !== null) ? 'inline-block' : 'none';
  document.getElementById('svyazka-edit-actions').style.display = svyazkaEditing ? 'flex' : 'none';
  const delBtn = document.getElementById('svyazka-btn-delete');
  if (delBtn) delBtn.style.display = svyazkaModalId !== null ? 'inline-block' : 'none';

  if (!svyazkaEditing) {
    const s = findSvyazka(svyazkaModalProvider, svyazkaModalId);
    box.innerHTML = `
      <div class="partner-field"><label>Название</label><span class="cell-static">${s.nazvanie}</span></div>
      <div class="partner-field"><label>Площадка</label><span class="cell-static">${svyazkaProviderLabel(s)}</span></div>
      <div class="partner-field"><label>Тип связки</label><span class="cell-static">${svyazkaKind(s)==='standard' ? 'Стандартная (1 под-источник)' : 'Кастомная (' + s.pod_source_ids.length + ' под-источников)'}</span></div>
      <div class="partner-field"><label>Под-источники</label><span class="cell-static">${s.pod_source_ids.map(sourceName).join(', ') || '\u2014'}</span></div>
      <div class="partner-field"><label>Проект ${svyazkaProviderLabel(s)}</label><span class="cell-static">${svyazkaProjectTitle(s) || '\u2014'}</span></div>
      <div class="partner-field"><label>Регионы</label><span class="cell-static">${(s.region_ids || []).map(regionName).join(', ') || 'все (фильтр не задан)'}</span></div>
      <div class="partner-field"><label>Поток</label><span class="cell-static">${s.type === 'dup' ? 'Работа с дублями' : 'Работа с униками'}</span></div>
    `;
    return;
  }

  const isCrmcalls = svyazkaProvider(svyazkaDraft) === 'crmcalls';
  const projectOptions = isCrmcalls
    ? crmcallsProjectCatalog().map(pr => ({ value: pr.id, label: `${pr.name || pr.id} (${pr.id})` }))
    : skorozvonProjectCatalog().map(pr => ({ value: pr.project_id, label: `${pr.nazvanie} (${pr.project_id})` }));

  const cAddRegion = 'svyazka-add-reg';
  registerCombo(cAddRegion, (DATA.regions || []).filter(r => !(svyazkaDraft.region_ids || []).includes(r.id)).map(r => ({ value: r.id, label: r.name })), v => draftSvyazkaRegionAdd(parseInt(v, 10)));

  const cProject = 'svyazka-project';
  registerCombo(cProject, projectOptions, v => {
    draftSvyazkaField('project_id', v);
    renderSvyazkaModal();
  });

  const cAddPodSource = 'svyazka-add-pod';
  const allPodSources = DATA.sources.filter(s => s.roditel_id !== null).map(s => ({ id:s.id, label: sourceName(s.roditel_id) + ' \u2192 ' + s.nazvanie }));
  registerCombo(cAddPodSource, allPodSources.filter(s => !svyazkaDraft.pod_source_ids.includes(s.id)).map(s => ({ value:s.id, label:s.label })), v => draftSvyazkaPodSourceAdd(parseInt(v)));

  const kindLabel = svyazkaDraft.pod_source_ids.length <= 1 ? 'Стандартная' : 'Кастомная';

  box.innerHTML = `
    <div class="partner-field">
      <label>Название связки</label>
      <input class="cell-input" style="border:1px solid var(--border-strong);" value="${svyazkaDraft.nazvanie}" placeholder="Например: ВР Билайн" onchange="draftSvyazkaField('nazvanie', this.value)">
    </div>
    <div class="partner-field">
      <label>Площадка ${helpIcon('Скорозвон и CRMCalls \u2014 отдельные очереди. После создания связки площадку сменить нельзя, только удалить и завести новую.')}</label>
      ${svyazkaModalId === null ? `
      <div style="display:flex; gap:8px;">
        <button type="button" class="btn ${!isCrmcalls ? 'btn-primary' : ''}" onclick="draftSvyazkaProvider('skorozvon')">Скорозвон</button>
        <button type="button" class="btn ${isCrmcalls ? 'btn-primary' : ''}" onclick="draftSvyazkaProvider('crmcalls')">CRMCalls</button>
      </div>` : `<span class="cell-static">${svyazkaProviderLabel(svyazkaDraft)}</span>`}
    </div>
    <div class="partner-field">
      <label>Под-источники ${helpIcon('Один под-источник \u2014 стандартная связка. Два и более \u2014 кастомная (объединяет несколько под-источников под один проект).')} <span class="status-pill ${kindLabel==='Стандартная'?'on':'off'}" style="margin-left:6px;">${kindLabel}</span></label>
      <div class="tag-list">
        ${svyazkaDraft.pod_source_ids.map(id => `<span class="tag">${sourceName(id)}<span class="tag-x" onclick="draftSvyazkaPodSourceRemove(${id})">&times;</span></span>`).join('') || '<span style="color:var(--text-muted); font-size:12px;">не выбрано ни одного</span>'}
      </div>
      <div style="margin-top:6px; width:220px;">${comboHTML(cAddPodSource, '', '+ добавить под-источник')}</div>
    </div>
    <div class="partner-field">
      <label>Проект ${isCrmcalls ? 'CRMCalls' : 'Скорозвона'} ${helpIcon(isCrmcalls ? 'Список проектов \u2014 из «Интеграции \u2192 Исходящий трафик»: подключите CRMCalls и подтяните проекты. Можно вставить UUID вручную. Worker создаст/обновит контакт POST /api/v1/contacts.' : 'Worker вызовет assignToProject с этим project_id для строк, попавших в эту связку. Список проектов \u2014 из «Интеграции \u2192 Исходящий трафик», единый реестр.')}</label>
      ${comboHTML(cProject, svyazkaDraft.project_id ? svyazkaProjectTitle(svyazkaDraft) : '', isCrmcalls ? 'Выбрать проект CRMCalls' : 'Выбрать проект из реестра')}
      ${isCrmcalls ? `<input class="cell-input" style="margin-top:6px; border:1px solid var(--border-strong); font-family:monospace; font-size:12px;" value="${escAttr(svyazkaDraft.project_id || '')}" placeholder="UUID проекта CRMCalls" onchange="draftSvyazkaField('project_id', this.value.trim()); renderSvyazkaModal();">` : ''}
    </div>
    <div class="partner-field">
      <label>Регионы ${helpIcon('Включающий фильтр: если список пуст \u2014 подходят строки из любого региона. Если не пуст \u2014 только перечисленные.')}</label>
      <div class="tag-list">
        ${(svyazkaDraft.region_ids || []).map(id => `<span class="tag">${regionName(id)}<span class="tag-x" onclick="draftSvyazkaRegionRemove(${id})">&times;</span></span>`).join('') || '<span style="color:var(--text-muted); font-size:12px;">не задано \u2014 подходят все регионы</span>'}
      </div>
      <div style="margin-top:6px; width:220px;">${comboHTML(cAddRegion, '', '+ добавить регион')}</div>
    </div>
    <div class="partner-field">
      <label>Поток</label>
      <span class="cell-static">${svyazkaDraft.type === 'dup' ? 'Работа с дублями' : 'Работа с униками'} — определяется вкладкой, где создана связка</span>
    </div>
  `;
}
function confirmDeleteSvyazka(provider, id) {
  if (arguments.length === 1) { id = provider; provider = svyazkaModalProvider || 'skorozvon'; }
  const s = findSvyazka(provider, id);
  if (!s) return;
  const flow = s.type === 'dup' ? 'дублей' : 'уников';
  confirmChange(
    `Удалить связку «${s.nazvanie}»? Новые ${flow} по этим под-источникам больше не уйдут в ${svyazkaProviderLabel(s)} (${s.project_id || 'проект'}).`,
    async () => {
      await apiJSON(svyazkaApiPath(s) + '/' + s.id, { method: 'DELETE' });
      DATA.svyazki = DATA.svyazki.filter(x => !(svyazkaProvider(x) === svyazkaProvider(s) && Number(x.id) === Number(s.id)));
      if (svyazkaModalId === s.id && svyazkaModalProvider === svyazkaProvider(s)) closeSvyazkaModal(false);
    }
  );
}
function renameSvyazka(provider, id) {
  if (arguments.length === 1) { id = provider; provider = 'skorozvon'; }
  const s = findSvyazka(provider, id);
  if (!s) return;
  customPrompt('Новое название связки:', async (name) => {
    if (!name) return;
    try {
      await apiJSON(svyazkaApiPath(s) + '/' + s.id, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(svyazkaPayload({ ...s, nazvanie: name })),
      });
      await loadSvyazki();
      renderPanels();
      showToast(activeTab);
    } catch (err) {
      showError(err.message || 'Не удалось переименовать');
    }
  });
}
function applySvyazkaDraft() {
  if (!svyazkaDraft.nazvanie.trim()) { showError('Укажите название связки.'); return; }
  if (svyazkaDraft.pod_source_ids.length === 0) { showError('Выберите хотя бы один под-источник.'); return; }
  if (!svyazkaDraft.project_id) { showError('Укажите проект ' + svyazkaProviderLabel(svyazkaDraft) + '.'); return; }

  const isNew = svyazkaModalId === null;
  const podNames = svyazkaDraft.pod_source_ids.map(sourceName).join(', ');
  const regionNames = (svyazkaDraft.region_ids || []).map(regionName).join(', ');
  const msg = isNew
    ? `Создать связку «${svyazkaDraft.nazvanie}» (${svyazkaProviderLabel(svyazkaDraft)}: ${podNames} \u2192 ${svyazkaDraft.project_id}, регионы: ${regionNames || 'все'})?`
    : `Применить изменения связки «${svyazkaDraft.nazvanie}»?\n\nНовые условия начнут действовать в проде сразу после подтверждения.`;

  customConfirm(msg, async (ok) => {
    if (!ok) return;
    try {
      const path = svyazkaApiPath(svyazkaDraft);
      if (isNew) {
        await apiJSON(path, {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(svyazkaPayload(svyazkaDraft)),
        });
      } else {
        await apiJSON(path + '/' + svyazkaModalId, {
          method: 'PUT',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify(svyazkaPayload(svyazkaDraft)),
        });
      }
      await loadSvyazki();
      closeSvyazkaModal(false);
      renderPanels();
      showToast(activeTab);
    } catch (err) {
      showError(err.message || 'Не удалось сохранить связку');
    }
  });
}

let ppStatsRange = { from: '2026-08-14', to: '2026-08-20' };
function setPpStatsRange(field, value) { ppStatsRange[field] = value; renderPanels(); }
function ppProjectStatsForRange(projectId, from, to) {
  const days = Math.max(1, Math.round((new Date(to) - new Date(from)) / 86400000) + 1);
  let seed = 0; for (let i=0;i<projectId.length;i++) seed += projectId.charCodeAt(i);
  const perDay = (seed % 15) + 3;
  return perDay * days;
}
function addSourceSkorozvon() { DATA.sourceSkorozvon.push({ id:nextId.sourceSkorozvon++, source_id:rootSources()[0].id, project_id:'proj-new' }); renderPanels(); }
function addUpk() {
  customPrompt('Название партнёра:', (name) => {
    if (!name) return;
    customPrompt('Webhook URL:', async (url) => {
      try {
        await apiJSON('/rest/partners', {
          method: 'POST',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({
            name,
            webhook_url: url || '',
            is_active: true,
            zarya_results: ['dup'],
            region_ids: [],
            stop_source_ids: [],
          }),
        });
        await loadPartners();
        renderPanels();
        showToast(activeTab);
      } catch (err) {
        showError(err.message || 'Не удалось создать партнёра');
      }
    });
  });
}
function renamePartner(id) {
  const u = DATA.upk.find(x => x.id === id);
  customPrompt('Новое название покупателя:', async (name) => {
    if (!name) return;
    try {
      await apiJSON('/rest/partners/' + u.id, {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ ...partnerPayload(u), name }),
      });
      await loadPartners();
      renderPanels();
      showToast(activeTab);
    } catch (err) {
      showError(err.message || 'Не удалось переименовать');
    }
  });
}
function addMediaplan() { DATA.mediaplans.push({ id:nextId.mediaplans++, upk:UPK_LIST[1], region:regionCanonicalList()[0] }); renderPanels(); }
function addRegion() { DATA.regionDict.push({ id:nextId.regionDict++, canonical:'Новый регион', raw:[] }); renderPanels(); }

/* ===== ИНИЦИАЛИЗАЦИЯ ===== */
(function initSettingsRouter() {
  document.getElementById('section-switch').addEventListener('click', (e) => {
    const a = e.target.closest('a[data-section]');
    if (!a) return;
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
    e.preventDefault();
    switchSection(a.dataset.section);
  });
  document.getElementById('tabs').addEventListener('click', (e) => {
    const a = e.target.closest('a[data-tab]');
    if (!a) return;
    if (e.metaKey || e.ctrlKey || e.shiftKey || e.altKey || e.button !== 0) return;
    e.preventDefault();
    switchTab(a.dataset.tab);
  });
  window.addEventListener('popstate', () => {
    const route = resolveRoute(location.pathname);
    syncingUrl = true;
    showTab(route.tabId, { replaceUrl: false }).finally(() => { syncingUrl = false; });
  });

  const route = resolveRoute(location.pathname);
  activeSection = route.sectionId;
  activeTab = route.tabId;
  if (location.pathname.replace(/\/+$/, '') !== route.canonical.replace(/\/+$/, '')) {
    history.replaceState({ tabId: route.tabId }, '', route.canonical);
  }
  showTab(route.tabId, { replaceUrl: true });
})();