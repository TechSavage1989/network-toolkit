/* Read-only routing-table suggestions for the existing bounded IPv4 scanner. */
(function (root) {
  'use strict';
  let loaded = false, loading = false, error = '', checkedAt = '', networks = [], query = '', page = 0, initialized = false;
  const pageSize = 6;
  function selectedScopes() {
    return new Set(NetworkToolkitScanScopes.tokens(state.networkRange).map(value => {
      try { return NetworkToolkitScanScopes.parseEntry(value).canonical; } catch (_) { return value; }
    }));
  }
  function pathLabel(path) {
    const name = path.interfaceName || (path.interfaceIndex ? 'Interface ' + path.interfaceIndex : path.interfaceAddress || 'Unidentified adapter');
    const adapter = name + (path.interfaceDescription && path.interfaceDescription !== name ? ' - ' + path.interfaceDescription : '');
    return (path.onLink || !path.gateway || path.gateway === '0.0.0.0' ? 'On-link' : 'Via ' + path.gateway) + ' | ' + adapter;
  }
  function filtered() {
    const needle = query.trim().toLowerCase();
    return networks.filter(item => !needle || [item.cidr, ...(item.paths || []).map(pathLabel)].join(' ').toLowerCase().includes(needle));
  }
  function listHTML() {
    const items = filtered(), selected = selectedScopes();
    page = Math.max(0, Math.min(page, Math.ceil(items.length / pageSize) - 1));
    const start = page * pageSize, visible = items.slice(start, start + pageSize);
    return (visible.length ? '<div class="route-scope-list" role="group" aria-label="Add routed networks to scan scopes">' + visible.map(item => {
      const added = selected.has(item.cidr), paths = (item.paths || []).map(pathLabel), large = Number(item.hostCount) > 1024;
      return '<button type="button" class="route-scope-card' + (added ? ' selected' : '') + '" data-route-toggle="' + esc(item.cidr) + '" aria-pressed="' + added +
        '" title="' + esc((added ? 'Remove ' : 'Add ') + item.cidr + ' from scan scopes.\n' + paths.join('\n')) + '"><span class="route-scope-icon">' + icon('route') +
        '</span><span class="route-scope-text"><strong class="mono">' + esc(item.cidr) + '</strong><span>' + esc(paths[0] || 'Routing-table destination') + '</span>' +
        '<small>' + fmt(item.hostCount, 0) + (item.hostCount === 1 ? ' host' : ' hosts') + (paths.length > 1 ? ' | ' + paths.length + ' route paths' : '') +
        (large ? ' | Narrow this range before scanning' : '') + '</small></span><span class="route-scope-mark">' + icon(added ? 'check' : 'plus') + '</span></button>';
    }).join('') + '</div>' : '<p class="help-line">' + (query ? 'No routed networks match this filter.' : 'No additional routed IPv4 networks are available.') + '</p>') +
      (items.length > pageSize ? '<div class="route-scope-pages"><span>' + (start + 1) + '-' + Math.min(start + pageSize, items.length) + ' of ' + items.length + '</span>' +
        '<button type="button" class="btn small ghost" data-route-page="-1"' + (!page ? ' disabled' : '') + '>Previous</button>' +
        '<button type="button" class="btn small ghost" data-route-page="1"' + (start + pageSize >= items.length ? ' disabled' : '') + '>Next</button></div>' : '');
  }
  function refreshList() {
    const list = document.getElementById('routeScopeList');
    if (list) list.innerHTML = listHTML();
  }
  function render() {
    const host = document.getElementById('routeScopePicker');
    if (!host) return;
    host.hidden = !networkMode() || !needsTarget();
    if (host.hidden) return;
    const focus = document.activeElement?.id === 'routeScopeFilter', selection = focus ? [document.activeElement.selectionStart, document.activeElement.selectionEnd] : null;
    host.innerHTML = '<section class="route-scope-picker" aria-label="Routed networks"><div class="route-scope-body"><div class="route-scope-tools">' +
      '<input type="search" id="routeScopeFilter" aria-label="Filter routed networks" placeholder="Filter network, gateway or adapter" value="' + esc(query) + '">' +
      '<button type="button" class="btn small ghost" data-route-refresh' + (!online || loading ? ' disabled' : '') + ' title="Refresh the local routing table">' + icon('refresh') + 'Refresh</button></div>' +
      (error ? '<p class="source-hint warn" role="status">' + esc(error) + '</p>' : '') + '<div id="routeScopeList">' + (loading && !loaded ? '<p class="help-line">Reading local routes...</p>' : listHTML()) + '</div>' +
      '<p class="help-line route-scope-note">Default and standard system routes are excluded. These cards add target scopes; Source interface chooses where probes are sent from.</p>' +
      (checkedAt ? '<p class="route-scope-time">Updated ' + esc(time(checkedAt)) + '</p>' : '') + '</div></section>';
    if (focus) { const input = document.getElementById('routeScopeFilter'); input.focus({preventScroll: true}); input.setSelectionRange?.(...selection); }
    if (!loaded && !loading && online && !OFFLINE) void load();
    root.NetworkToolkitConfiguration?.update();
  }
  async function load(force) {
    if (loading || !online || OFFLINE || shutDown || loaded && !force) return;
    loading = true; render();
    try {
      const result = await api('/scan-networks');
      networks = (Array.isArray(result.networks) ? result.networks : []).filter(item => item && typeof item.cidr === 'string' && Number.isFinite(item.hostCount));
      checkedAt = typeof result.checkedAt === 'string' ? result.checkedAt : '';
      error = typeof result.error === 'string' ? result.error : '';
      loaded = true;
    } catch (failure) { error = 'The local routing table could not be read: ' + failure.message; loaded = true; }
    finally { loading = false; render(); }
  }
  function toggle(cidr) {
    if (!networks.some(item => item.cidr === cidr)) return;
    const entries = NetworkToolkitScanScopes.tokens(state.networkRange), selected = selectedScopes().has(cidr);
    const next = selected ? entries.filter(value => { try { return NetworkToolkitScanScopes.parseEntry(value).canonical !== cidr; } catch (_) { return value !== cidr; } }) : [...entries, cidr];
    setCurrentTargetValue(next.join('\n')); renderTargetControls(); updateRunButtons(); saveState();
    const button = [...document.querySelectorAll('[data-route-toggle]')].find(item => item.dataset.routeToggle === cidr);
    button?.focus({preventScroll: true});
  }
  function init() {
    if (initialized) return; initialized = true;
    document.addEventListener('click', event => {
      const toggleButton = event.target.closest('[data-route-toggle]');
      if (toggleButton) { event.preventDefault(); toggle(toggleButton.dataset.routeToggle); return; }
      const refresh = event.target.closest('[data-route-refresh]');
      if (refresh && !refresh.disabled) { event.preventDefault(); void load(true); return; }
      const step = event.target.closest('[data-route-page]');
      if (step && !step.disabled) { event.preventDefault(); page += Number(step.dataset.routePage); refreshList(); }
    });
    document.addEventListener('input', event => { if (event.target.id === 'routeScopeFilter') { query = event.target.value.slice(0, 300); page = 0; refreshList(); } });
  }
  root.NetworkToolkitScanRoutes = Object.freeze({init, render, refresh: () => load(true), refreshList, info: () => ({available: networks.length, loading, loaded, error})});
})(globalThis);
