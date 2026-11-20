'use strict';

// Settings page shared by the Windows and Linux apps. Laid out like the macOS
// app: profiles in a sidebar, and for the selected profile a status dashboard,
// quick templates and one card per knob gesture. Every label is written inline
// as L('tiếng Việt', 'English'), like the Go side.

let lang = 'en';
const L = (vi, en) => (lang === 'en' ? en : vi);

const GESTURES = [
  { id: 'rotate_left', vi: 'Xoay trái', en: 'Rotate left', icon: '↺' },
  { id: 'rotate_right', vi: 'Xoay phải', en: 'Rotate right', icon: '↻' },
  { id: 'press', vi: 'Nhấn', en: 'Press', icon: '◉' },
  { id: 'press_rotate_left', vi: 'Giữ + xoay trái', en: 'Hold + rotate left', icon: '↺' },
  { id: 'press_rotate_right', vi: 'Giữ + xoay phải', en: 'Hold + rotate right', icon: '↻' },
];

const TYPES = [
  { id: '', vi: '(theo mặc định)', en: '(use default)' },
  { id: 'passthrough', vi: 'Hệ điều hành xử lý (âm lượng)', en: 'System handles it (volume)' },
  { id: 'shortcut', vi: 'Tổ hợp phím', en: 'Shortcut' },
  { id: 'scroll', vi: 'Cuộn chuột', en: 'Scroll' },
  { id: 'mouse', vi: 'Click chuột', en: 'Mouse click' },
  { id: 'shell', vi: 'Chạy lệnh', en: 'Run command' },
  { id: 'noop', vi: 'Không làm gì', en: 'Do nothing' },
];

const keys = (...k) => ({ type: 'shortcut', keys: k });
const scroll = (dy, dx, ...k) => {
  const a = { type: 'scroll' };
  if (dy) a.dy = dy;
  if (dx) a.dx = dx;
  if (k.length) a.keys = k;
  return a;
};

const QUICK = [
  { group: ['Âm thanh', 'Media'], items: [
    ['Âm lượng +', 'Volume +', keys('volume_up')],
    ['Âm lượng −', 'Volume −', keys('volume_down')],
    ['Tắt tiếng', 'Mute', keys('mute')],
    ['Play / Pause', 'Play / Pause', keys('play_pause')],
    ['Bài sau', 'Next track', keys('next_track')],
    ['Bài trước', 'Previous track', keys('previous_track')],
  ] },
  { group: ['Màn hình', 'Display'], items: [
    ['Độ sáng +', 'Brightness +', keys('brightness_up')],
    ['Độ sáng −', 'Brightness −', keys('brightness_down')],
  ] },
  { group: ['Cuộn và zoom', 'Scroll and zoom'], items: [
    ['Zoom to (Ctrl + cuộn)', 'Zoom in (Ctrl + scroll)', scroll(1, 0, 'ctrl')],
    ['Zoom nhỏ (Ctrl + cuộn)', 'Zoom out (Ctrl + scroll)', scroll(-1, 0, 'ctrl')],
    ['Cuộn lên', 'Scroll up', scroll(1, 0)],
    ['Cuộn xuống', 'Scroll down', scroll(-1, 0)],
    ['Cuộn trái', 'Scroll left', scroll(0, -1)],
    ['Cuộn phải', 'Scroll right', scroll(0, 1)],
  ] },
  { group: ['Trình duyệt', 'Browser'], items: [
    ['Tab sau', 'Next tab', keys('ctrl', 'tab')],
    ['Tab trước', 'Previous tab', keys('ctrl', 'shift', 'tab')],
    ['Trang trước (Alt + ←)', 'Back (Alt + ←)', keys('alt', 'left')],
    ['Trang sau (Alt + →)', 'Forward (Alt + →)', keys('alt', 'right')],
    ['Tải lại (F5)', 'Reload (F5)', keys('f5')],
  ] },
  { group: ['Soạn thảo', 'Editing'], items: [
    ['Hoàn tác (Ctrl + Z)', 'Undo (Ctrl + Z)', keys('ctrl', 'z')],
    ['Làm lại (Ctrl + Y)', 'Redo (Ctrl + Y)', keys('ctrl', 'y')],
    ['Sao chép (Ctrl + C)', 'Copy (Ctrl + C)', keys('ctrl', 'c')],
    ['Dán (Ctrl + V)', 'Paste (Ctrl + V)', keys('ctrl', 'v')],
    ['Lưu (Ctrl + S)', 'Save (Ctrl + S)', keys('ctrl', 's')],
    ['Tìm (Ctrl + F)', 'Find (Ctrl + F)', keys('ctrl', 'f')],
  ] },
  { group: ['Phím đơn', 'Single keys'], items: [
    ['Mũi tên trái', 'Left arrow', keys('left')],
    ['Mũi tên phải', 'Right arrow', keys('right')],
    ['Enter', 'Enter', keys('enter')],
    ['Esc', 'Esc', keys('esc')],
    ['Space', 'Space', keys('space')],
  ] },
  { group: ['Hệ thống', 'System'], items: [
    ['Chuyển cửa sổ (Alt + Tab)', 'Switch window (Alt + Tab)', keys('alt', 'tab')],
    ['Hiện desktop (Win + D)', 'Show desktop (Win + D)', keys('win', 'd')],
    ['Click trái', 'Left click', { type: 'mouse', button: 'left' }],
    ['Hệ điều hành xử lý (như núm gốc)', 'System handles it (knob default)', { type: 'passthrough' }],
    ['Không làm gì', 'Do nothing', { type: 'noop' }],
  ] },
];

const TEMPLATES = [
  ['Âm lượng', 'Volume', { type: 'passthrough' }, { type: 'passthrough' }],
  ['Độ sáng màn hình', 'Screen brightness', keys('brightness_down'), keys('brightness_up')],
  ['Zoom (Ctrl + cuộn)', 'Zoom (Ctrl + scroll)', scroll(-1, 0, 'ctrl'), scroll(1, 0, 'ctrl')],
  ['Zoom (Ctrl − / Ctrl =)', 'Zoom (Ctrl − / Ctrl =)', keys('ctrl', '-'), keys('ctrl', '=')],
  ['Cuộn trang', 'Scroll page', scroll(1, 0), scroll(-1, 0)],
  ['Cuộn ngang', 'Scroll sideways', scroll(0, -1), scroll(0, 1)],
  ['Chuyển tab', 'Switch tabs', keys('ctrl', 'shift', 'tab'), keys('ctrl', 'tab')],
  ['Tua video (← →)', 'Seek video (← →)', keys('left'), keys('right')],
  ['Hoàn tác / làm lại', 'Undo / redo', keys('ctrl', 'z'), keys('ctrl', 'y')],
  ['Cỡ brush ([ ])', 'Brush size ([ ])', keys('['), keys(']')],
  ['Bài trước / bài sau', 'Previous / next track', keys('previous_track'), keys('next_track')],
];

// ---- state ------------------------------------------------------------------

let state = null; // live status from the core, polled
let config = null; // the configuration being edited
let apps = []; // running apps that can be given a profile
let selected = ''; // '' is the global profile, anything else an apps key
let dirty = false;
let knownRev = null; // config revision the editor is based on
let warnedRev = null;
let query = '';
let tpl = 0;
let tplTarget = 'rotate';
let statusMsg = { text: '', error: false };
const live = {}; // nodes updated in place on every poll

const token = (() => {
  const m = location.hash.match(/t=([0-9a-f]+)/);
  if (m) {
    try { sessionStorage.setItem('knobToken', m[1]); } catch (e) { /* private mode */ }
    history.replaceState(null, '', location.pathname);
    return m[1];
  }
  try { return sessionStorage.getItem('knobToken') || ''; } catch (e) { return ''; }
})();

async function api(method, path, body) {
  const init = { method, cache: 'no-store', headers: { 'X-Knob-Token': token } };
  if (body !== undefined) {
    init.headers['Content-Type'] = 'application/json';
    init.body = JSON.stringify(body);
  }
  const res = await fetch('/api/' + path, init);
  let data = null;
  try { data = await res.json(); } catch (e) { /* empty body */ }
  if (!res.ok) {
    const err = new Error((data && data.error) || res.statusText);
    err.status = res.status;
    throw err;
  }
  return data;
}

// el builds a DOM node. Text always goes through textContent: app names and
// window titles come from other programs and must never be parsed as HTML.
function el(tag, props, ...kids) {
  const n = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v == null || v === false) continue;
    if (k === 'class') n.className = v;
    else if (k === 'text') n.textContent = v;
    else if (k.startsWith('on')) n.addEventListener(k.slice(2), v);
    else if (k in n) n[k] = v;
    else n.setAttribute(k, v);
  }
  n.append(...clean(kids));
  return n;
}

// clean flattens nested child lists and drops the empty slots conditionals leave.
const clean = (kids) => kids.flat(Infinity).filter((k) => k != null && k !== false);
const setKids = (n, ...kids) => n.replaceChildren(...clean(kids));

// ---- profiles ---------------------------------------------------------------

const profileMap = (key) => (key === '' ? config.global : config.apps[key]);
const splitKey = (key) => {
  const i = key.indexOf('|');
  return i < 0 ? [key, ''] : [key.slice(0, i), key.slice(i + 1)];
};
const sameName = (a, b) => a.toLowerCase() === b.toLowerCase();

function profileTitle(key) {
  return key === '' ? L('Mặc định chung', 'Global default') : splitKey(key)[0];
}

function markDirty() {
  dirty = true;
  updateLive();
}

function setStatus(text, error) {
  statusMsg = { text, error: !!error };
  if (live.status) {
    live.status.textContent = text;
    live.status.className = 'status' + (error ? ' error' : '');
  }
}

function select(key) {
  selected = key;
  renderList();
  renderHeader();
  renderEditor();
  updateLive();
}

function addApp(name) {
  name = name.trim();
  if (!name) return;
  const existing = Object.keys(config.apps).find((k) => sameName(k, name));
  if (!existing) {
    config.apps[name] = {};
    markDirty();
    setStatus(L(`Đã thêm ${name} — bấm Lưu cấu hình để áp dụng.`, `Added ${name} — click Save config to apply.`));
  }
  select(existing || name);
}

function deleteSelected() {
  if (selected === '') return;
  delete config.apps[selected];
  markDirty();
  setStatus(L(`Đã xoá ${selected} — bấm Lưu cấu hình để áp dụng.`, `Deleted ${selected} — click Save config to apply.`));
  select('');
}

function renameSelected(exe, title) {
  exe = exe.trim();
  title = title.trim();
  if (selected === '' || !exe) return renderHeader();
  const key = title ? exe + '|' + title : exe;
  if (key === selected) return;
  if (config.apps[key]) {
    setStatus(L(`Đã có mục ${key} — giữ tên cũ.`, `${key} already exists — kept the old name.`), true);
    return renderHeader();
  }
  config.apps[key] = config.apps[selected];
  delete config.apps[selected];
  markDirty();
  select(key);
}

function applyTemplate(key) {
  const t = TEMPLATES[tpl];
  const [left, right] = tplTarget === 'hold' ? ['press_rotate_left', 'press_rotate_right'] : ['rotate_left', 'rotate_right'];
  const m = profileMap(key);
  m[left] = structuredClone(t[2]);
  m[right] = structuredClone(t[3]);
  markDirty();
  setStatus(L(`Đã điền mẫu "${t[0]}" — bấm Lưu cấu hình để áp dụng.`, `Template "${t[1]}" filled in — click Save config to apply.`));
}

// ---- server calls -----------------------------------------------------------

async function loadConfig() {
  config = await api('GET', 'config');
  config.global = config.global || {};
  config.apps = config.apps || {};
  dirty = false;
  if (selected !== '' && !config.apps[selected]) selected = '';
}

async function syncRev() {
  state = await api('GET', 'state');
  knownRev = state.configRev;
}

async function save() {
  try {
    await api('POST', 'config', config);
    dirty = false;
    await syncRev();
    setStatus(L('Đã lưu và áp dụng lúc ', 'Saved and applied at ') + new Date().toLocaleTimeString() + '.');
  } catch (e) {
    setStatus(L('Chưa lưu được: ', 'Could not save: ') + e.message, true);
  }
  updateLive();
}

async function reload() {
  try {
    await api('POST', 'command', { name: 'reload' });
    await loadConfig();
    await syncRev();
    build();
    setStatus(L('Đã nạp lại cấu hình từ file.', 'Config reloaded from the file.'));
  } catch (e) {
    setStatus(L('Không nạp lại được: ', 'Could not reload: ') + e.message, true);
  }
}

async function command(name, arg) {
  try {
    await api('POST', 'command', { name, arg: arg || '' });
    return true;
  } catch (e) {
    setStatus(e.message, true);
    return false;
  }
}

async function setLanguage(code) {
  if (!(await command('language', code))) return;
  config.language = code; // the core wrote it to the file; keep unsaved edits in step
  await syncRev();
  lang = state.language;
  build();
}

async function loadApps() {
  try {
    apps = (await api('GET', 'apps')) || [];
  } catch (e) {
    apps = [];
  }
  renderList();
}

async function poll() {
  let s;
  try {
    s = await api('GET', 'state');
  } catch (e) {
    showBanner(e.status === 401
      ? L('Phiên này đã cũ. Mở lại Cài đặt từ biểu tượng Anticater Knob ở khay hệ thống.', 'This window is out of date. Reopen Settings from the Anticater Knob tray icon.')
      : L('Mất kết nối với Anticater Knob. App có đang chạy không?', 'Lost connection to Anticater Knob. Is the app running?'));
    return;
  }
  showBanner('');
  const langChanged = s.language !== lang;
  state = s;
  lang = s.language;
  if (knownRev === null || (s.configRev !== knownRev && !dirty)) {
    await loadConfig();
    knownRev = s.configRev;
    build();
  } else if (langChanged) {
    build();
  } else {
    if (s.configRev !== knownRev && warnedRev !== s.configRev) {
      warnedRev = s.configRev;
      setStatus(L('File cấu hình đã đổi trên đĩa. Bấm Nạp lại để lấy bản mới (mất phần chưa lưu).',
        'The config file changed on disk. Click Reload to pick it up (unsaved edits are lost).'), true);
    }
    updateLive();
  }
}

function showBanner(text) {
  if (!live.banner) {
    live.banner = el('div', { class: 'banner' });
    document.body.append(live.banner);
  }
  live.banner.textContent = text;
  live.banner.hidden = !text;
}

// ---- layout -----------------------------------------------------------------

function build() {
  document.documentElement.lang = lang;
  live.list = el('div', { class: 'list' });
  live.header = el('header', { class: 'header' });
  live.dash = el('section', { class: 'dash' });
  live.editor = el('section', { class: 'editor' });
  live.status = el('span', { class: 'status' });
  live.configFile = el('span', { class: 'config-file' });
  setKids(document.getElementById('app'),
    el('aside', { class: 'sidebar' }, live.list, sidebarTools()),
    el('main', { class: 'detail' }, live.header, el('div', { class: 'scroll' }, live.dash, live.editor),
      el('footer', { class: 'footer' }, live.status, live.configFile)),
  );
  renderList();
  renderHeader();
  renderEditor();
  setStatus(statusMsg.text, statusMsg.error);
  updateLive();
}

function avatar(name, glyph) {
  let hash = 0;
  for (const ch of name) hash = (hash * 31 + ch.codePointAt(0)) % 360;
  const a = el('span', { class: 'avatar', text: glyph || (name.replace(/[^\p{L}\p{N}]/gu, '')[0] || '?').toUpperCase() });
  if (!glyph) a.style.background = `hsl(${hash} 55% 48%)`;
  return a;
}

function sidebarTools() {
  const manual = el('input', {
    type: 'text', spellcheck: false,
    placeholder: L('Nhập tên exe, vd. notepad.exe', 'Enter an exe name, e.g. notepad.exe'),
    onkeydown: (e) => { if (e.key === 'Enter') addManual(); },
  });
  const addManual = () => {
    addApp(manual.value);
    manual.value = '';
  };
  live.del = el('button', { class: 'danger', text: L('Xoá app đang chọn', 'Delete selected app'), onclick: deleteSelected });
  live.appsStatus = el('span', { class: 'muted small' });
  return el('div', { class: 'side-tools' },
    el('input', {
      type: 'search', value: query, placeholder: L('Tìm app', 'Search apps'),
      oninput: (e) => { query = e.target.value; renderList(); },
    }),
    el('div', { class: 'row' }, el('button', { text: L('Làm mới', 'Refresh'), onclick: loadApps }), live.appsStatus),
    manual,
    el('div', { class: 'row' },
      el('button', { text: L('Thêm', 'Add'), onclick: addManual }),
      el('button', {
        text: L('Thêm app hiện tại', 'Add current app'),
        onclick: () => { if (state.frontApp) addApp(state.frontApp); },
      })),
    live.del,
  );
}

function renderList() {
  if (!live.list) return;
  const item = (key, title, sub, glyph) => el('button', {
    class: 'item' + (selected === key ? ' selected' : ''), onclick: () => select(key),
  }, avatar(title, glyph), el('span', { class: 'item-text' },
    el('span', { class: 'item-title', text: title }),
    sub ? el('span', { class: 'item-sub', text: sub }) : null));

  const configured = Object.keys(config.apps).sort((a, b) => a.toLowerCase().localeCompare(b.toLowerCase()));
  const q = query.trim().toLowerCase();
  const found = apps.filter((a) => !q || a.name.toLowerCase().includes(q) || a.title.toLowerCase().includes(q));

  setKids(live.list,
    el('div', { class: 'section', text: L('Mặc định', 'Default') }),
    item('', L('Mặc định chung', 'Global default'), L('Mọi app không có hồ sơ riêng', 'Every app without its own profile'), '◎'),
    el('div', { class: 'section', text: L('App', 'Apps') }),
    configured.length ? configured.map((key) => {
      const [exe, title] = splitKey(key);
      return item(key, exe, title ? L('Tiêu đề chứa: ', 'Title has: ') + title : '');
    }) : el('div', { class: 'empty', text: L('Chưa có app nào', 'No configured apps') }),
    el('div', { class: 'section', text: L('App đang chạy', 'Discovered apps') }),
    found.length ? found.map((a) => {
      const added = configured.some((k) => sameName(splitKey(k)[0], a.name));
      return el('button', { class: 'item', title: a.title, onclick: () => addApp(a.name) },
        avatar(a.name), el('span', { class: 'item-text' },
          el('span', { class: 'item-title', text: a.name }),
          el('span', { class: 'item-sub', text: a.title })),
        el('span', { class: 'badge' + (added ? ' on' : ''), text: added ? L('Đã thêm', 'Added') : L('Đang chạy', 'Running') }));
    }) : el('div', { class: 'empty', text: L('Không thấy app nào', 'No apps found') }),
  );
  if (live.appsStatus) live.appsStatus.textContent = L(`${apps.length} app đang chạy`, `${apps.length} running apps`);
}

function tile(label, name) {
  live[name] = el('div', { class: 'tile-value' });
  live[name + 'Sub'] = el('div', { class: 'tile-sub' });
  return el('div', { class: 'tile' }, el('div', { class: 'tile-label', text: label }), live[name], live[name + 'Sub']);
}

function renderHeader() {
  if (!live.header) return;
  live.front = el('div', { class: 'muted' });
  live.pause = el('button', { onclick: async () => { await command('pause'); poll(); } });
  live.save = el('button', { onclick: save });

  let rename = null;
  if (selected !== '') {
    const [exe, title] = splitKey(selected);
    const exeBox = el('input', { type: 'text', value: exe, spellcheck: false });
    const titleBox = el('input', {
      type: 'text', value: title, spellcheck: false,
      placeholder: L('để trống = mọi cửa sổ', 'empty = any window'),
    });
    const commit = () => renameSelected(exeBox.value, titleBox.value);
    exeBox.addEventListener('change', commit);
    titleBox.addEventListener('change', commit);
    rename = el('div', { class: 'bar' },
      el('label', {}, el('span', { class: 'caption', text: L('Tên exe', 'Exe name') }), exeBox),
      el('label', {}, el('span', { class: 'caption', text: L('Tiêu đề cửa sổ chứa', 'Window title has') }), titleBox));
  }

  live.pills = {};
  const pills = GESTURES.map((g) => {
    const count = el('b', { text: '0' });
    live.pills[g.id] = { count, node: el('span', { class: 'pill' }, g.icon + ' ' + L(g.vi, g.en) + ' ', count) };
    return live.pills[g.id].node;
  });
  live.counts = el('span', { class: 'muted small' });
  live.raw = el('span', { class: 'muted small mono' });

  const tools = el('details', { class: 'menu' },
    el('summary', { text: L('Công cụ', 'Listening tools') }),
    el('div', { class: 'menu-pop' },
      menuItem(L('Khôi phục âm lượng cho hồ sơ mặc định', 'Restore global volume default'), () => {
        for (const g of ['rotate_left', 'rotate_right', 'press']) config.global[g] = { type: 'passthrough' };
        markDirty();
        select('');
        setStatus(L('Đã đặt lại xoay và nhấn về âm lượng — bấm Lưu cấu hình để áp dụng.',
          'Rotate and press are back to volume — click Save config to apply.'));
      }),
      menuItem(L('Xoá bộ đếm', 'Clear counters'), async () => { await command('clearCounters'); poll(); }),
      el('hr'),
      menuItem(L('Báo cáo chẩn đoán', 'Diagnostic report'), () => command('diagnostics')),
      menuItem(L('Mở file cấu hình', 'Open config file'), () => command('openConfig')),
      menuItem(L('Mở log', 'Open log'), () => command('openLog')),
      el('hr'),
      live.autostart = menuItem('', async () => { await command('autostart'); poll(); }),
      menuItem(lang === 'en' ? 'Ngôn ngữ: Tiếng Việt' : 'Language: English', () => setLanguage(lang === 'en' ? 'vi' : 'en')),
    ));

  setKids(live.header,
    el('div', { class: 'title-row' },
      el('div', { class: 'title-text' }, el('h1', { text: profileTitle(selected) }), live.front),
      live.pause,
      el('button', { text: L('Nạp lại', 'Reload'), onclick: reload }),
      live.save),
    rename);
  setKids(live.dash,
    el('div', { class: 'tiles' },
      tile(L('Lắng nghe', 'Listening'), 'tListen'),
      tile(L('Núm', 'Knob'), 'tKnob'),
      tile(L('App phía trước', 'Frontmost app'), 'tFront'),
      tile(L('Hồ sơ đang áp dụng', 'Effective mapping'), 'tEffective'),
      tile(L('Chặn phím gốc', 'Native key hook'), 'tHook'),
      tile(L('Tín hiệu gần nhất', 'Last event'), 'tLast')),
    el('div', { class: 'monitor' },
      el('span', { class: 'caption', text: L('Theo dõi tín hiệu', 'Input monitor') }), live.counts, ...pills, live.raw),
    el('div', { class: 'bar' },
      el('span', { class: 'caption', text: L('Mẫu nhanh', 'Quick templates') }),
      el('select', { onchange: (e) => { tpl = +e.target.value; } },
        TEMPLATES.map((t, i) => el('option', { value: String(i), text: L(t[0], t[1]), selected: i === tpl }))),
      el('span', { class: 'segmented' },
        segment(L('Xoay', 'Rotate'), 'rotate'), segment(L('Giữ + xoay', 'Hold + rotate'), 'hold')),
      el('button', {
        text: L('Áp dụng cho hồ sơ này', 'Apply to current profile'),
        onclick: () => { applyTemplate(selected); renderEditor(); },
      }),
      el('button', {
        text: L('Tạo cho app phía trước và áp dụng', 'Create for frontmost app and apply'),
        onclick: () => {
          if (!state.frontApp) return setStatus(L('Chưa biết app phía trước.', 'No frontmost app yet.'), true);
          addApp(state.frontApp);
          applyTemplate(selected);
          renderEditor();
        },
      }),
      el('span', { class: 'grow' }),
      tools),
  );
  updateLive();
}

function segment(label, value) {
  return el('button', {
    class: tplTarget === value ? 'on' : '', text: label,
    onclick: () => { tplTarget = value; renderHeader(); updateLive(); },
  });
}

function menuItem(label, fn) {
  return el('button', {
    class: 'menu-item', text: label,
    onclick: (e) => { e.target.closest('details').open = false; fn(); },
  });
}

// ---- gesture editor ---------------------------------------------------------

const keyText = (a) => (a.keys || []).join('+');
const parseKeys = (s) => s.split(/[+\s]+/).filter(Boolean);

function field(label, control, cls) {
  return el('label', { class: 'field' + (cls ? ' ' + cls : '') }, el('span', { class: 'caption', text: label }), control);
}

function numberBox(a, prop, min) {
  return el('input', {
    type: 'number', value: a[prop] || '', min: min == null ? undefined : String(min),
    oninput: (e) => {
      const v = parseInt(e.target.value, 10);
      if (Number.isFinite(v) && v !== 0) a[prop] = v; else delete a[prop];
      markDirty();
    },
  });
}

function textBox(a, prop, placeholder) {
  return el('input', {
    type: 'text', value: a[prop] || '', placeholder: placeholder || '', spellcheck: false,
    oninput: (e) => {
      if (e.target.value) a[prop] = e.target.value; else delete a[prop];
      markDirty();
    },
  });
}

function keysBox(a, placeholder) {
  return el('input', {
    type: 'text', value: keyText(a), placeholder, spellcheck: false,
    oninput: (e) => {
      const k = parseKeys(e.target.value);
      if (k.length) a.keys = k; else delete a.keys;
      markDirty();
    },
  });
}

function gestureCard(g) {
  const m = profileMap(selected);
  const a = m[g.id];
  const type = !a ? (selected === '' ? 'passthrough' : '') : (a.type === 'key' ? 'shortcut' : a.type || 'passthrough');

  const setAction = (next) => {
    if (next) m[g.id] = next; else delete m[g.id];
    markDirty();
    renderEditor();
  };

  const typeSelect = el('select', {
    onchange: (e) => {
      const t = e.target.value;
      if (t === '') return setAction(null);
      const next = { type: t };
      if (t === 'scroll') next.dy = -1;
      if (t === 'mouse') next.button = 'left';
      if (a && a.description) next.description = a.description;
      setAction(next);
    },
  }, TYPES.filter((t) => t.id !== '' || selected !== '').map((t) =>
    el('option', { value: t.id, text: L(t.vi, t.en), selected: t.id === type })));

  const quick = el('select', {
    class: 'quick',
    onchange: (e) => {
      const [gi, ii] = e.target.value.split('.').map(Number);
      if (Number.isFinite(ii)) {
        const pick = QUICK[gi].items[ii];
        setAction(structuredClone(pick[2]));
        setStatus(L(`Đã gán "${pick[0]}" — bấm Lưu cấu hình để áp dụng.`, `Assigned "${pick[1]}" — click Save config to apply.`));
      }
    },
  }, el('option', { value: '', text: L('Chọn nhanh…', 'Quick pick…') }),
  QUICK.map((group, gi) => el('optgroup', { label: L(group.group[0], group.group[1]) },
    group.items.map((it, ii) => el('option', { value: gi + '.' + ii, text: L(it[0], it[1]) })))));

  let body;
  switch (type) {
    case 'shortcut': {
      const box = keysBox(a, 'ctrl+shift+tab');
      body = [field(L('Phím', 'Keys'), box, 'wide'),
        el('button', { text: L('Ghi phím…', 'Record…'), onclick: () => recordShortcut((k) => { a.keys = k; markDirty(); renderEditor(); }) })];
      break;
    }
    case 'scroll':
      body = [field(L('Dọc (dy)', 'Vertical (dy)'), numberBox(a, 'dy'), 'narrow'),
        field(L('Ngang (dx)', 'Horizontal (dx)'), numberBox(a, 'dx'), 'narrow'),
        field(L('Phím giữ kèm', 'Held keys'), keysBox(a, L('vd. ctrl để zoom', 'e.g. ctrl to zoom')))];
      break;
    case 'mouse':
      body = [field(L('Nút chuột', 'Button'), el('select', {
        onchange: (e) => { a.button = e.target.value; markDirty(); },
      }, [['left', 'Trái', 'Left'], ['right', 'Phải', 'Right'], ['middle', 'Giữa', 'Middle']].map(([v, vi, en]) =>
        el('option', { value: v, text: L(vi, en), selected: (a.button || 'left').toLowerCase() === v }))))];
      break;
    case 'shell':
      body = [field(L('Lệnh', 'Command'), textBox(a, 'command', 'notepad.exe'), 'wide')];
      break;
    case 'noop':
      body = [el('span', { class: 'muted', text: L('Thao tác này bị bỏ qua.', 'This gesture is ignored.') })];
      break;
    case 'passthrough':
      body = [el('span', { class: 'muted', text: L('Hệ điều hành tự xử lý như núm gốc: âm lượng hoặc tắt tiếng.', 'The system handles it like the stock knob: volume or mute.') })];
      break;
    default:
      body = [el('span', { class: 'muted', text: L('Dùng hành động của hồ sơ mặc định.', 'Uses the action of the global default profile.') })];
  }
  if (a && ['shortcut', 'scroll', 'mouse', 'shell'].includes(type)) {
    const every = field(L('Mỗi N nấc', 'Every N ticks'), numberBox(a, 'every', 1), 'narrow');
    every.title = L('Chỉ chạy mỗi N nấc; đặt 2 nếu một khấc núm chạy hành động hai lần.',
      'Act only on every Nth tick; use 2 if one detent fires the action twice.');
    body.push(every);
  }
  if (a) body.push(field(L('Ghi chú', 'Description'), textBox(a, 'description'), 'wide'));

  return el('div', { class: 'card' },
    el('div', { class: 'card-head' },
      el('span', { class: 'gesture-icon' + (g.id.startsWith('press_') ? ' hold' : ''), text: g.icon }),
      el('span', { class: 'gesture-name', text: L(g.vi, g.en) }),
      el('span', { class: 'grow' }),
      field(L('Hành động', 'Type'), typeSelect), quick),
    el('div', { class: 'card-body' }, body));
}

function renderEditor() {
  if (!live.editor) return;
  setKids(live.editor, selected === '' ? brightnessCard() : null, ledCard(), GESTURES.map(gestureCard));
}

// brightnessCard picks which screens the brightness actions move (config.brightness_screens).
function brightnessCard() {
  if (state && state.platform && state.platform !== 'windows') return null;
  const all = el('input', { type: 'checkbox' });
  all.checked = config.brightness_screens === 'all';
  all.addEventListener('change', () => {
    if (all.checked) config.brightness_screens = 'all'; else delete config.brightness_screens;
    markDirty();
  });
  return el('div', { class: 'card' },
    el('div', { class: 'card-head' }, el('strong', { text: L('Độ sáng màn hình', 'Screen brightness') })),
    el('div', { class: 'card-body' }, el('label', {}, all, ' ', L(
      'Chỉnh mọi màn hình cùng lúc (bỏ chọn: chỉ màn hình đang có con trỏ chuột)',
      'Change every screen together (unticked: only the screen under the mouse pointer)'))));
}

// ledCard edits the colour of the knob's lights for the selected profile
// (config.led, see windows/led_windows.go). The global profile also holds the
// desktop colour and the "auto" switch for apps without a colour.
function ledCard() {
  if (state && state.platform && state.platform !== 'windows') return null;
  const led = config.led || null;
  const key = selected === '' ? 'desktop' : selected;
  const colour = led && led.apps && led.apps[key];
  const hex = (c) => '#' + (c && /^[0-9a-f]{6}$/i.test(c) ? c : 'FFFFFF');
  const ensure = () => {
    if (!config.led) config.led = { brightness: 45, auto: true, apps: {} };
    config.led.apps = config.led.apps || {};
    return config.led;
  };
  // The colour in force now: the one set here, or the automatic one the app reports.
  const shown = colour || (state && state.ledColours && state.ledColours[key]) || '';
  const pick = el('input', { type: 'color', value: hex(shown) });
  pick.addEventListener('input', () => {
    ensure().apps[key] = pick.value.slice(1).toUpperCase();
    markDirty();
    renderEditor();
  });
  const clear = el('button', {
    text: selected === '' ? L('Không đổi màu ở màn hình nền', 'No colour on the desktop') : L('Dùng màu tự động', 'Use the automatic colour'),
    disabled: !colour,
    onclick: () => { delete ensure().apps[key]; markDirty(); renderEditor(); },
  });
  const rows = [
    el('div', { class: 'row' },
      el('span', { text: selected === '' ? L('Màu ở màn hình nền', 'Colour on the desktop') : L('Màu khi dùng app này', 'Colour in this app') }),
      pick,
      el('span', { class: 'muted small', text: colour ? '#' + colour.toUpperCase()
        : shown && /^[0-9a-f]{6}$/i.test(shown) ? '#' + shown.toUpperCase() + L(' (tự động)', ' (automatic)') : L('(tự động)', '(automatic)') }),
      clear),
  ];
  if (selected === '') {
    const auto = el('input', { type: 'checkbox' });
    auto.checked = !led || !!led.auto;
    auto.addEventListener('change', () => { ensure().auto = auto.checked; markDirty(); });
    const bright = el('input', { type: 'number', min: 1, max: 100, value: (led && led.brightness) || 45, style: 'width:5em' });
    bright.addEventListener('change', () => { ensure().brightness = Math.max(1, Math.min(100, +bright.value || 45)); markDirty(); });
    rows.push(el('div', { class: 'row' },
      el('label', {}, auto, ' ', L('App chưa có màu: lấy màu đặc trưng của app, hoặc một màu cố định theo tên', 'Apps without a colour: their brand colour, or a fixed colour from their name')),
    ));
    rows.push(el('div', { class: 'row' }, el('span', { text: L('Độ sáng đèn (1–100)', 'Light brightness (1–100)') }), bright));
    const green = el('input', { type: 'number', min: 30, max: 100, step: 5, value: Math.round(((led && led.green) || 0.75) * 100), style: 'width:5em' });
    green.addEventListener('change', () => { ensure().green = Math.max(30, Math.min(100, +green.value || 75)) / 100; markDirty(); });
    rows.push(el('div', { class: 'row' }, el('span', { text: L('Giảm xanh lá (%) — bù màu xanh của mạch trong núm', 'Green level (%) — offsets the green board inside the knob') }), green));
  }
  rows.push(el('p', { class: 'muted small', text: L(
    'Đèn đổi màu qua Bluetooth khi bạn chuyển app. Đóng phần mềm ANTICATER của hãng, vì nó giữ đèn của núm.',
    'The lights change over Bluetooth when you switch apps. Close the vendor ANTICATER app: it holds the lights.') }));
  return el('div', { class: 'card' },
    el('div', { class: 'card-head' }, el('strong', { text: L('Đèn của núm', 'Knob lights') })),
    el('div', { class: 'card-body', style: 'flex-direction:column;align-items:stretch' }, rows));
}

// ---- shortcut recorder ------------------------------------------------------

const CODE_KEYS = {
  Enter: 'enter', NumpadEnter: 'enter', Escape: 'esc', Space: 'space', Tab: 'tab', Backspace: 'backspace',
  Delete: 'delete', Insert: 'insert', Home: 'home', End: 'end', PageUp: 'pageup', PageDown: 'pagedown',
  ArrowLeft: 'left', ArrowUp: 'up', ArrowRight: 'right', ArrowDown: 'down',
  BracketLeft: '[', BracketRight: ']', Minus: '-', Equal: '=', Comma: ',', Period: '.', Slash: '/',
  Semicolon: ';', Quote: "'", Backquote: '`', Backslash: '\\',
};
// Names that are valid in the Keys box but cannot be typed as a key press.
const MEDIA_KEYS = {
  AudioVolumeUp: 'volume_up', AudioVolumeDown: 'volume_down', AudioVolumeMute: 'mute',
  MediaPlayPause: 'play_pause', MediaTrackNext: 'next_track', MediaTrackPrevious: 'previous_track',
};

function keyName(e) {
  if (MEDIA_KEYS[e.key]) return MEDIA_KEYS[e.key];
  if (CODE_KEYS[e.code]) return CODE_KEYS[e.code];
  let m = /^Key([A-Z])$/.exec(e.code);
  if (m) return m[1].toLowerCase();
  m = /^(?:Digit|Numpad)(\d)$/.exec(e.code);
  if (m) return m[1];
  m = /^F(\d{1,2})$/.exec(e.code);
  if (m && +m[1] >= 1 && +m[1] <= 24) return 'f' + m[1];
  return '';
}

function recordShortcut(done) {
  let combo = [];
  const shown = el('div', { class: 'combo', text: L('Bấm tổ hợp phím…', 'Press the shortcut…') });
  const use = el('button', { class: 'primary', text: L('Dùng', 'Use'), disabled: true });
  const onKey = (e) => {
    e.preventDefault();
    e.stopPropagation();
    const mods = [];
    if (e.ctrlKey) mods.push('ctrl');
    if (e.shiftKey) mods.push('shift');
    if (e.altKey) mods.push('alt');
    if (e.metaKey) mods.push('win');
    const key = keyName(e);
    if (!key) {
      shown.textContent = mods.length ? mods.join(' + ') + ' + …' : L('Bấm tổ hợp phím…', 'Press the shortcut…');
      return;
    }
    combo = [...mods, key];
    shown.textContent = combo.join(' + ');
    use.disabled = false;
  };
  const close = () => {
    window.removeEventListener('keydown', onKey, true);
    sheet.remove();
  };
  use.addEventListener('click', () => { close(); done(combo); });
  const sheet = el('div', { class: 'sheet-back' },
    el('div', { class: 'sheet' },
      el('h2', { text: L('Ghi tổ hợp phím', 'Record shortcut') }),
      shown,
      el('p', { class: 'muted small', text: L(
        'Tổ hợp mà hệ điều hành hoặc trình duyệt giữ riêng (Alt + Tab, Win + D…) không ghi được ở đây — hãy gõ tên phím vào ô Phím.',
        'Shortcuts reserved by the system or the browser (Alt + Tab, Win + D…) cannot be recorded here — type the key names in the Keys box.') }),
      el('div', { class: 'row end' }, el('button', { text: L('Huỷ', 'Cancel'), onclick: close }), use)));
  document.body.append(sheet);
  window.addEventListener('keydown', onKey, true);
}

// ---- live status ------------------------------------------------------------

function setTile(name, value, sub, tone) {
  if (!live[name]) return;
  live[name].textContent = value;
  live[name].className = 'tile-value' + (tone ? ' ' + tone : '');
  live[name + 'Sub'].textContent = sub || '';
  live[name + 'Sub'].title = sub || '';
}

function updateLive() {
  if (!state || !live.front) return;
  const s = state;
  live.front.textContent = L('App phía trước: ', 'Frontmost app: ') + (s.frontApp || '—');
  live.pause.textContent = s.paused ? L('Tiếp tục lắng nghe', 'Start listening') : L('Dừng lắng nghe', 'Stop listening');
  live.save.textContent = L('Lưu cấu hình', 'Save config');
  live.save.className = dirty ? 'primary' : '';
  live.del.disabled = selected === '';
  live.autostart.textContent = (s.autostart ? '✓ ' : '') + L('Khởi động cùng hệ thống', 'Start with the system');

  setTile('tListen', s.paused ? L('Đang tạm dừng', 'Paused') : L('Đang chạy', 'Running'), '', s.paused ? 'warn' : 'ok');
  const ago = s.knobAgoMs;
  setTile('tKnob',
    ago < 0 ? L('Chưa có tín hiệu', 'No input yet') : L('Đã nhận tín hiệu', 'Receiving input'),
    ago < 0 ? L('Đang chờ: ', 'Waiting for: ') + (s.devices || []).join(', ')
      : (ago < 2000 ? L('vừa xong', 'just now') : L(`${Math.round(ago / 1000)} giây trước`, `${Math.round(ago / 1000)} s ago`)),
    ago < 0 ? 'warn' : 'ok');
  setTile('tFront', s.frontApp || '—', s.frontTitle);
  setTile('tEffective', s.effective ? splitKey(s.effective)[0] : L('Mặc định chung', 'Global default'),
    s.effective && splitKey(s.effective)[1] ? L('Tiêu đề chứa: ', 'Title has: ') + splitKey(s.effective)[1] : '');
  setTile('tHook', s.hookInstalled ? L('Đang bật', 'Installed') : L('Không cài được', 'Failed'),
    L(`Đã chặn ${s.swallowed} · gửi lại ${s.resent}`, `${s.swallowed} swallowed · ${s.resent} re-sent`), s.hookInstalled ? 'ok' : 'bad');
  setTile('tLast', s.lastEvent || '—', '');
  live.tLast.title = s.lastEvent || '';

  live.counts.textContent = L(`Thô ${s.raw} · Nhận diện ${s.recognized}`, `Raw ${s.raw} · Recognized ${s.recognized}`);
  live.raw.textContent = s.lastRaw || '';
  for (const g of GESTURES) {
    const pill = live.pills[g.id];
    const n = (s.gestures && s.gestures[g.id]) || 0;
    if (pill.shown !== undefined && n > pill.shown) {
      pill.node.classList.remove('flash');
      void pill.node.offsetWidth; // restart the animation
      pill.node.classList.add('flash');
    }
    pill.shown = n;
    pill.count.textContent = String(n);
  }
  live.configFile.textContent = L('Cấu hình: ', 'Config: ') + (s.configPath || '').split(/[\\/]/).pop() + (dirty ? L(' · chưa lưu', ' · unsaved') : '');
  live.configFile.title = s.configPath || '';
}

// ---- start ------------------------------------------------------------------

if (!token) {
  lang = (navigator.language || 'en').startsWith('vi') ? 'vi' : 'en';
  showBanner(L('Mở Cài đặt từ biểu tượng Anticater Knob ở khay hệ thống.', 'Open Settings from the Anticater Knob tray icon.'));
} else {
  poll().then(loadApps);
  setInterval(poll, 600);
  window.addEventListener('focus', loadApps);
}
