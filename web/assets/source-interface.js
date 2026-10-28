/* Source selection and recorded source labels. Original project code, no runtime dependencies. */
(function (root, factory) {
  'use strict';
  const api = factory();
  if (typeof module === 'object' && module.exports) module.exports = api;
  if (root) root.NetworkToolkitSource = api;
})(typeof globalThis === 'object' ? globalThis : this, function () {
  'use strict';
  const tools = new Set(['quick', 'ping', 'trace', 'tcp', 'monitor', 'http', 'tls', 'discover', 'identify', 'dns']);
  const supports = tool => tools.has(tool);
  const record = value => value !== null && typeof value === 'object' && !Array.isArray(value);
  const text = value => typeof value === 'string' ? value.trim() : '';
  const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;'}[c]));
  let initialized = false;

  function addressInfo(value) {
    let address = text(value).split('/')[0];
    if (address.startsWith('[') && address.endsWith(']')) address = address.slice(1, -1);
    if (!address || address.length > 200) return null;
    const ipv4 = address.split('.');
    if (ipv4.length === 4 && ipv4.every(part => /^\d{1,3}$/.test(part) && Number(part) <= 255)) {
      const octets = ipv4.map(Number);
      if (octets.every(n => n === 0) || octets[0] >= 224) return null;
      return {address: octets.join('.'), family: 'ipv4'};
    }
    const parts = address.split('%');
    if (parts.length > 2 || parts.length === 2 && !parts[1]) return null;
    try {
      const parsed = new URL('http://[' + parts[0] + ']/');
      const bare = parsed.hostname.slice(1, -1).toLowerCase();
      if (!bare.includes(':') || bare === '::' || bare.startsWith('ff')) return null;
      return {address: bare + (parts.length === 2 ? '%' + parts[1] : ''), family: 'ipv6'};
    } catch (_) { return null; }
  }

  function normalize(value) {
    if (!record(value)) return null;
    const index = value.interfaceIndex, info = addressInfo(value.address);
    if (!Number.isInteger(index) || index <= 0 || index > 0xffffffff || !info) return null;
    return {interfaceIndex: index, interfaceName: text(value.interfaceName).slice(0, 512),
      interfaceDescription: text(value.interfaceDescription).slice(0, 1024), ...info};
  }

  const key = source => source ? JSON.stringify([source.interfaceIndex, source.interfaceName || '', source.address]) : '';
  function adapterLabel(source) {
    const name = text(source?.interfaceName) || 'Interface ' + source?.interfaceIndex;
    const description = text(source?.interfaceDescription);
    return name + (description && description.toLowerCase() !== name.toLowerCase() ? ' - ' + description : '');
  }
  const choiceLabel = source => adapterLabel(source) + ' | ' + source.address + ' (' + (source.family === 'ipv6' ? 'IPv6' : 'IPv4') + ')';

  function choices(adapters) {
    const out = [], seen = new Set();
    for (const adapter of Array.isArray(adapters) ? adapters : []) {
      if (!record(adapter) || adapter.up !== true || !Number.isInteger(adapter.index) || adapter.index <= 0) continue;
      const details = Array.isArray(adapter.addressDetails) ? adapter.addressDetails : [];
      for (const entry of Array.isArray(adapter.addresses) ? adapter.addresses : []) {
        const raw = typeof entry === 'string' ? entry : entry?.address || entry?.ip || entry?.cidr;
        const info = addressInfo(raw);
        if (!info) continue;
        const detail = details.find(row => addressInfo(row.address || row.ip || row.cidr)?.address.split('%')[0] === info.address.split('%')[0]);
        const dad = String(detail?.dadState ?? detail?.state ?? '').toLowerCase();
        if (['invalid', 'tentative', 'duplicate', '0', '1', '2'].includes(dad)) continue;
        const source = normalize({interfaceIndex: adapter.index, interfaceName: adapter.name,
          interfaceDescription: adapter.description, address: info.address});
        if (!source || seen.has(key(source))) continue;
        seen.add(key(source)); out.push(source);
      }
    }
    return out.sort((a, b) => a.interfaceName.localeCompare(b.interfaceName, undefined, {numeric: true}) ||
      a.interfaceIndex - b.interfaceIndex || a.family.localeCompare(b.family) || a.address.localeCompare(b.address, undefined, {numeric: true}));
  }

  function availability(source, adapters, tool, subnet) {
    if (!source) return {ok: true, selected: null, message: ''};
    const selected = choices(adapters).find(item => key(item) === key(source));
    if (!selected) return {ok: false, selected: source, message: 'The selected source is unavailable. Refresh adapters, choose an available source, or select Automatic routing.'};
    if ((['discover', 'identify'].includes(tool) || subnet) && selected.family !== 'ipv4') {
      return {ok: false, selected, message: 'This IPv4 scan needs an IPv4 source. Choose an IPv4 address or Automatic routing.'};
    }
    return {ok: true, selected, message: ''};
  }

  function requestOptions(tool, source) {
    if (!supports(tool) || !source) return {};
    return {sourceInterfaceIndex: source.interfaceIndex, sourceInterfaceName: source.interfaceName || '', sourceAddress: source.address};
  }

  function jobSource(job) {
    if (record(job?.result?.data?.source)) return job.result.data.source;
    const events = Array.isArray(job?.events) ? job.events : [];
    for (let index = events.length - 1; index >= 0; index--) {
      if (events[index]?.data?.type === 'source' && record(events[index].data.value)) return events[index].data.value;
    }
    if (job?.options?.sourceAddress !== undefined || job?.options?.sourceInterfaceIndex !== undefined) {
      return {mode: 'selected', status: 'pending', interfaceIndex: job.options.sourceInterfaceIndex,
        interfaceName: job.options.sourceInterfaceName, address: job.options.sourceAddress,
        note: 'Requested source. Interface validation has not been recorded.'};
    }
    return supports(job?.tool) ? {mode: 'automatic', status: 'automatic', nameResolution: 'system',
      note: 'The operating system chooses the route and source address. No explicit source was selected.'} : null;
  }

  function summary(source, includeDescription) {
    if (!source || source.mode === 'automatic') return 'Automatic routing';
    const name = includeDescription ? adapterLabel(source) : text(source.interfaceName) || 'Interface ' + source.interfaceIndex;
    return name + (source.address ? ' / ' + source.address : '') + (source.status === 'invalid' ? ' (unavailable)' : '');
  }

  function sourceFields(job) {
    const source = jobSource(job);
    if (!source) return [];
    if (source.mode === 'automatic') return [['Source selection', 'Automatic routing'], ['Source details', source.note || 'The operating system chooses the route and source address.']];
    const fields = [['Source interface', adapterLabel(source)], ['Source IP', source.address || 'Not recorded'],
      ['Source validation', ({validated: 'Validated before this run', invalid: 'Invalid; no fallback to Automatic', pending: 'Requested; validation not recorded'})[source.status] || source.status || 'Not recorded']];
    if (source.family) fields.push(['Source address family', String(source.family).toUpperCase()]);
    if (source.interfaceIndex) fields.push(['Interface index', String(source.interfaceIndex)]);
    if (source.dnsServer) fields.push(['DNS server', source.dnsServer]);
    if (source.nameResolution) fields.push(['Name resolution', ({bound_dns: 'DNS queries bound to the selected source', unavailable: 'No compatible DNS server is available on this source', system: 'System resolver'})[source.nameResolution] || source.nameResolution]);
    if (source.note) fields.push(['Source details', source.note]);
    if (source.error) fields.push(['Source error', source.error]);
    return fields;
  }

  function resultHTML(job) {
    const source = jobSource(job);
    if (!source) return '';
    // Keep the immutable run context available without duplicating the sidebar
    // selection in the shared results header. This disclosure is read-only.
    return '<details class="recorded-diagnostic-source" data-detail="recorded-source"><summary>Recorded source: ' +
      esc(summary(source)) + '</summary><div class="data-grid">' + sourceFields(job).map(([name, value]) =>
        '<div class="data-item"><div class="key">' + esc(name) + '</div><div class="val">' + esc(value) + '</div></div>').join('') +
      '</div><p class="help-line">Recorded for this diagnostic. The sidebar source selection applies to future runs.</p></details>';
  }

  function controlHTML(source, adapters, tool, subnet, id) {
    id = id || 'diagnosticSource';
    const available = choices(adapters), selectedKey = key(source), found = !source || available.some(item => key(item) === selectedKey);
    const checked = availability(source, adapters, tool, subnet);
    const hint = !checked.ok ? checked.message : source
      ? 'Shared across diagnostics. Probes and DNS queries use this source; edits apply to the next run.'
      : 'Uses the system routing table. Select an adapter and IP to choose the source for diagnostic probes.';
    return '<div class="diagnostic-source-control' + (!checked.ok ? ' source-unavailable' : '') + '"><div class="source-control-heading">' +
      '<label for="' + esc(id) + '">Source interface</label><button type="button" class="btn small ghost source-refresh" data-source-refresh title="Refresh source interfaces" aria-label="Refresh source interfaces">Refresh</button></div>' +
      '<select id="' + esc(id) + '" data-diagnostic-source aria-describedby="' + esc(id) + '-hint" title="' + esc(source ? choiceLabel(checked.selected) : 'Automatic routing') + '">' +
      '<option value=""' + (!source ? ' selected' : '') + '>Automatic routing</option>' +
      (!found ? '<option value="' + esc(selectedKey) + '" selected>Unavailable: ' + esc(choiceLabel(source)) + '</option>' : '') +
      available.map(item => '<option value="' + esc(key(item)) + '"' + (key(item) === selectedKey ? ' selected' : '') + '>' + esc(choiceLabel(item)) + '</option>').join('') +
      '</select><p id="' + esc(id) + '-hint" class="source-hint"' + (!checked.ok ? ' role="status"' : '') + '>' + esc(hint) + '</p></div>';
  }

  function render() {
    const host = document.getElementById('diagnosticSourceHost');
    if (!host) return;
    const tool = activeTool();
    host.hidden = !supports(tool) || !needsTarget();
    if (host.hidden) { host.innerHTML = ''; return; }
    const focus = document.activeElement?.id;
    host.innerHTML = controlHTML(state.diagnosticSource, interfaces(), tool, networkMode(tool));
    if (focus === 'diagnosticSource') document.getElementById(focus)?.focus({preventScroll: true});
  }

  function validation(tool) {
    return supports(tool) ? availability(state.diagnosticSource, interfaces(), tool, networkMode(tool)).message : '';
  }

  function commandSummary() {
    const element = document.getElementById('toolTargetMeta');
    if (!element || !supports(activeTool()) || !splitToolPage()) return;
    const chosen = state.diagnosticSource;
    element.textContent = 'Source: ' + summary(chosen) + ' | ' + element.textContent;
    element.title = (chosen ? choiceLabel(chosen) + '. ' : '') + element.textContent;
  }

  function deviceControlHTML() {
    return '<section class="device-diagnostic-source"><h3>Diagnostic source</h3>' +
      controlHTML(state.diagnosticSource, interfaces(), 'quick', false, 'deviceDiagnosticSource') +
      '<p class="help-line">This setting applies to Run actions below. Terminal sessions and external applications use their own connection routing.</p></section>';
  }

  function init() {
    if (initialized || typeof document === 'undefined') return;
    initialized = true;
    document.addEventListener('change', event => {
      if (!event.target.matches('[data-diagnostic-source]')) return;
      const value = event.target.value;
      if (!value) state.diagnosticSource = null;
      else {
        const choice = choices(interfaces()).find(item => key(item) === value);
        if (!choice) return;
        state.diagnosticSource = choice;
      }
      render();
      const device = document.getElementById('deviceDiagnosticSource');
      if (device) {
        const focused = document.activeElement === device;
        device.closest('.diagnostic-source-control').outerHTML = controlHTML(state.diagnosticSource, interfaces(), 'quick', false, 'deviceDiagnosticSource');
        if (focused) document.getElementById('deviceDiagnosticSource')?.focus({preventScroll: true});
      }
      updateRunButtons(); saveState();
    });
    document.addEventListener('click', async event => {
      const refresh = event.target.closest('[data-source-refresh]');
      if (refresh) {
        event.preventDefault();
        if (refresh.disabled) return;
        refresh.disabled = true;
        try {
          await refreshSystem();
          const device = document.getElementById('deviceDiagnosticSource');
          if (device) device.closest('.diagnostic-source-control').outerHTML = controlHTML(state.diagnosticSource, interfaces(), 'quick', false, 'deviceDiagnosticSource');
        } finally { refresh.disabled = false; }
        return;
      }
      const button = event.target.closest('[data-source-job]');
      if (!button) return;
      const job = [...(imported?.jobs || []), ...jobs].find(item => item.id === button.dataset.sourceJob);
      if (!job) return;
      openModal('Source for this run', '<div class="data-grid">' + sourceFields(job).map(([name, value]) => kv(name, value)).join('') +
        '</div><p class="help-line">This is the source recorded for this diagnostic. Changing the current selection affects future runs.</p>');
    });
  }

  return Object.freeze({supports, normalize, choices, availability, requestOptions, jobSource, summary, sourceFields,
    resultHTML, controlHTML, render, validation, commandSummary, deviceControlHTML, init});
});
