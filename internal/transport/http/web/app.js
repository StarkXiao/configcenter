const state = { token: sessionStorage.getItem('adminToken') || '', operator: sessionStorage.getItem('operator') || 'admin', apps: [], app: null, environments: [], environment: null, draft: null, releases: [], stream: null, mode: null };
const byId = id => document.getElementById(id);

byId('admin-token').value = state.token;
byId('operator').value = state.operator;

async function api(path, options = {}) {
  const headers = { Authorization: `Bearer ${state.token}`, 'X-Operator': byId('operator').value.trim(), ...options.headers };
  if (options.body) headers['Content-Type'] = 'application/json';
  const response = await fetch(path, { ...options, headers });
  if (response.status === 204) return null;
  const data = await response.json().catch(() => ({}));
  if (!response.ok) throw new Error(data.error?.message || `${response.status} ${response.statusText}`);
  return data;
}

function toast(message, error = false) {
  const element = byId('toast');
  element.textContent = message;
  element.classList.toggle('error', error);
  element.hidden = false;
  clearTimeout(toast.timer);
  toast.timer = setTimeout(() => { element.hidden = true; }, 3500);
}

async function connect() {
  state.token = byId('admin-token').value.trim();
  state.operator = byId('operator').value.trim() || 'admin';
  sessionStorage.setItem('adminToken', state.token);
  sessionStorage.setItem('operator', state.operator);
  try {
    await loadApps();
    byId('health').textContent = '服务已连接';
    byId('health').className = 'status online';
  } catch (error) {
    byId('health').textContent = '连接失败';
    byId('health').className = 'status offline';
    toast(error.message, true);
  }
}

async function loadApps(selectSlug = '') {
  const data = await api('/api/v1/apps');
  state.apps = data.items || [];
  renderApps();
  if (selectSlug) {
    const target = state.apps.find(item => item.slug === selectSlug);
    if (target) await selectApp(target);
  }
}

function renderApps() {
  const host = byId('apps');
  host.replaceChildren();
  if (!state.apps.length) {
    host.innerHTML = '<p class="empty">暂无应用</p>';
    return;
  }
  state.apps.forEach(item => {
    const button = document.createElement('button');
    button.className = `app-button ${state.app?.slug === item.slug ? 'active' : ''}`;
    button.innerHTML = `<strong>${escapeHTML(item.name)}</strong><span>${escapeHTML(item.slug)}</span>`;
    button.addEventListener('click', () => selectApp(item));
    host.append(button);
  });
}

async function selectApp(application) {
  closeStream();
  state.app = application;
  state.environment = null;
  state.draft = null;
  renderApps();
  byId('current-app').textContent = application.name;
  byId('app-description').textContent = application.description || application.slug;
  byId('add-env').disabled = false;
  const data = await api(`/api/v1/apps/${application.slug}/envs`);
  state.environments = data.items || [];
  renderEnvironments();
  if (state.environments.length) await selectEnvironment(state.environments[0]);
  else showEmpty('该应用还没有环境', '创建环境后即可维护配置。');
}

function renderEnvironments() {
  const host = byId('environments');
  host.replaceChildren();
  state.environments.forEach(item => {
    const button = document.createElement('button');
    button.className = `tab ${state.environment?.code === item.code ? 'active' : ''}`;
    button.textContent = `${item.name} · v${item.current_version}`;
    button.addEventListener('click', () => selectEnvironment(item));
    host.append(button);
  });
}

async function selectEnvironment(environment) {
  closeStream();
  state.environment = environment;
  renderEnvironments();
  byId('empty-state').hidden = true;
  byId('environment-view').hidden = false;
  byId('env-name').textContent = `${environment.name} (${environment.code})`;
  await refreshEnvironment();
  openStream();
}

async function refreshEnvironment() {
  if (!state.app || !state.environment) return;
  try {
    const base = `/api/v1/apps/${state.app.slug}/envs/${state.environment.code}`;
    const [draft, releases] = await Promise.all([api(`${base}/draft`), api(`${base}/releases`)]);
    state.draft = draft;
    state.releases = releases.items || [];
    state.environment.current_version = draft.current_version;
    byId('current-version').textContent = draft.current_version;
    byId('draft-revision').textContent = draft.revision;
    renderDraft();
    renderReleases();
    renderEnvironments();
  } catch (error) { toast(error.message, true); }
}

function renderDraft() {
  const host = byId('config-rows');
  host.replaceChildren();
  (state.draft?.items || []).forEach(item => host.append(createRow(item)));
  byId('config-empty').hidden = host.children.length > 0;
}

function createRow(item = { key: '', type: 'string', value: '', sensitive: false, description: '' }) {
  const row = document.createElement('tr');
  row.innerHTML = `<td><input data-field="key" maxlength="128"></td>
    <td><select data-field="type"><option>string</option><option>number</option><option>boolean</option><option>json</option></select></td>
    <td><input data-field="value"></td><td><input data-field="sensitive" type="checkbox"></td>
    <td><input data-field="description" maxlength="300"></td><td><button class="delete-row" title="删除">×</button></td>`;
  row.querySelector('[data-field=key]').value = item.key;
  row.querySelector('[data-field=type]').value = item.type;
  row.querySelector('[data-field=value]').value = item.value;
  row.querySelector('[data-field=sensitive]').checked = item.sensitive;
  row.querySelector('[data-field=description]').value = item.description;
  row.querySelector('.delete-row').addEventListener('click', () => { row.remove(); byId('config-empty').hidden = byId('config-rows').children.length > 0; });
  return row;
}

function collectItems() {
  return [...byId('config-rows').children].map(row => ({
    key: row.querySelector('[data-field=key]').value.trim(), type: row.querySelector('[data-field=type]').value,
    value: row.querySelector('[data-field=value]').value, sensitive: row.querySelector('[data-field=sensitive]').checked,
    description: row.querySelector('[data-field=description]').value.trim()
  }));
}

async function saveDraft() {
  try {
    state.draft = await api(spacePath('/draft'), { method: 'PUT', body: JSON.stringify({ revision: state.draft.revision, items: collectItems() }) });
    byId('draft-revision').textContent = state.draft.revision;
    toast('草稿已保存');
  } catch (error) { toast(error.message, true); }
}

async function showDiff() {
  try {
    const diff = await api(spacePath('/diff'));
    const groups = [['新增', diff.added], ['修改', diff.modified], ['删除', diff.deleted]];
    byId('diff-content').innerHTML = groups.map(([label, items]) => `<div class="diff-group"><h3>${label} (${items.length})</h3><ul>${items.map(item => `<li>${escapeHTML(item.key)}</li>`).join('') || '<li>无</li>'}</ul></div>`).join('');
    byId('diff-dialog').showModal();
  } catch (error) { toast(error.message, true); }
}

function renderReleases() {
  const host = byId('release-rows');
  host.replaceChildren();
  state.releases.forEach(item => {
    const row = document.createElement('tr');
    const rollback = item.version !== state.environment.current_version ? `<button data-version="${item.version}">回滚</button>` : '<span>当前</span>';
    row.innerHTML = `<td>v${item.version}</td><td>${item.operation === 'rollback' ? '回滚' : '发布'}</td><td>${escapeHTML(item.change_summary)}</td><td>${escapeHTML(item.operator)}</td><td>${new Date(item.created_at).toLocaleString()}</td><td>${rollback}</td>`;
    row.querySelector('button')?.addEventListener('click', () => openAction('rollback', item.version));
    host.append(row);
  });
  byId('history-empty').hidden = state.releases.length > 0;
}

function openEntity(mode) {
  state.mode = mode;
  byId('form-title').textContent = mode === 'app' ? '新增应用' : '新增环境';
  byId('code-field').firstChild.textContent = mode === 'app' ? '应用标识' : '环境代码';
  byId('entity-name').value = '';
  byId('entity-code').value = '';
  byId('entity-description').value = '';
  byId('form-dialog').showModal();
}

async function submitEntity(event) {
  event.preventDefault();
  const body = { name: byId('entity-name').value.trim(), description: byId('entity-description').value.trim() };
  if (state.mode === 'app') body.slug = byId('entity-code').value.trim(); else body.code = byId('entity-code').value.trim();
  try {
    if (state.mode === 'app') {
      const result = await api('/api/v1/apps', { method: 'POST', body: JSON.stringify(body) });
      byId('form-dialog').close();
      await loadApps(result.application.slug);
      toast(`应用已创建。客户端令牌：${result.access_token}`);
    } else {
      const result = await api(`/api/v1/apps/${state.app.slug}/envs`, { method: 'POST', body: JSON.stringify(body) });
      byId('form-dialog').close();
      state.environments.push(result);
      await selectEnvironment(result);
      toast('环境已创建');
    }
  } catch (error) { toast(error.message, true); }
}

function openAction(mode, target = 0) {
  state.mode = mode;
  byId('action-title').textContent = mode === 'publish' ? '发布配置' : `回滚到 v${target}`;
  byId('action-copy').textContent = mode === 'publish' ? '发布会生成新版本并通知所有订阅客户端。' : '回滚会复制历史快照并生成新的递增版本。';
  byId('target-field').hidden = true;
  byId('target-version').innerHTML = `<option value="${target}">${target}</option>`;
  byId('action-reason').value = '';
  byId('action-submit').className = mode === 'publish' ? 'primary' : 'danger';
  byId('action-dialog').showModal();
}

async function submitAction(event) {
  event.preventDefault();
  const reason = byId('action-reason').value.trim();
  try {
    if (state.mode === 'publish') {
      await api(spacePath('/releases'), { method: 'POST', body: JSON.stringify({ expected_version: state.environment.current_version, summary: reason }) });
      toast('配置已发布');
    } else {
      await api(spacePath('/rollback'), { method: 'POST', body: JSON.stringify({ expected_version: state.environment.current_version, target_version: Number(byId('target-version').value), reason }) });
      toast('版本已回滚');
    }
    byId('action-dialog').close();
    await refreshEnvironment();
  } catch (error) { toast(error.message, true); }
}

function openStream() {
  byId('stream-state').textContent = '控制台轮询';
  byId('stream-state').className = 'status online';
  state.stream = setInterval(refreshEnvironment, 30000);
}

function closeStream() {
  if (state.stream) clearInterval(state.stream);
  state.stream = null;
  byId('stream-state').textContent = '未订阅';
  byId('stream-state').className = 'status offline';
}

function spacePath(suffix) { return `/api/v1/apps/${state.app.slug}/envs/${state.environment.code}${suffix}`; }
function showEmpty(title, copy) { byId('environment-view').hidden = true; byId('empty-state').hidden = false; byId('empty-state').innerHTML = `<div><h2>${escapeHTML(title)}</h2><p>${escapeHTML(copy)}</p></div>`; }
function escapeHTML(value = '') { return String(value).replace(/[&<>'"]/g, char => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', "'": '&#39;', '"': '&quot;' }[char])); }

byId('connect').addEventListener('click', connect);
byId('add-app').addEventListener('click', () => openEntity('app'));
byId('add-env').addEventListener('click', () => openEntity('env'));
byId('entity-form').addEventListener('submit', submitEntity);
byId('action-form').addEventListener('submit', submitAction);
byId('add-row').addEventListener('click', () => { byId('config-rows').append(createRow()); byId('config-empty').hidden = true; });
byId('save-draft').addEventListener('click', saveDraft);
byId('view-diff').addEventListener('click', showDiff);
byId('publish').addEventListener('click', () => openAction('publish'));
byId('refresh').addEventListener('click', refreshEnvironment);
byId('close-diff').addEventListener('click', () => byId('diff-dialog').close());
window.addEventListener('beforeunload', closeStream);
if (state.token) connect();
