/* Application-only discovery layout and table interactions. Reports use their own renderer. */
(function (root) {
  'use strict';
  const selectedByRun = new Map();
  const clamp = (value, min, max) => Math.max(min, Math.min(max, Number(value) || min));
  const runKey = job => String(job.id) + '|' + String(job.started || '');
  let resizeObserver, observedWorkspace, queued = false, started = false;
  const defaults = {width: 330, height: 260, hidden: false, dock: 'auto'};
  function preferences() { return state.discoveryDetails || (state.discoveryDetails = {...defaults}); }
  function mergePreferences(saved) {
    const source = saved.discoveryDetails || {};
    state.discoveryDetails = {width: clamp(source.width || defaults.width, 260, 700), height: clamp(source.height || defaults.height, 80, 650), hidden: source.hidden === true, dock: ['auto', 'right', 'bottom'].includes(source.dock) ? source.dock : 'auto'};
    state.tableWidths = {};
    if (saved.tableWidths && typeof saved.tableWidths === 'object' && !Array.isArray(saved.tableWidths)) {
      for (const [view, columns] of Object.entries(saved.tableWidths).slice(0, 150)) {
        if (!view || view.length > 240 || ['__proto__', 'constructor', 'prototype'].includes(view) || !columns || typeof columns !== 'object' || Array.isArray(columns)) continue;
        const safe = {};
        for (const [id, width] of Object.entries(columns).slice(0, 60)) if (id.length <= 120 && !['__proto__', 'constructor', 'prototype'].includes(id) && Number.isFinite(width)) safe[id] = Math.round(clamp(width, 64, 1200));
        state.tableWidths[view] = safe;
      }
    }
  }

  function scopeEntries() {
    return NetworkToolkitScanScopes.tokens(state.networkRange).map(raw => {
      try { return NetworkToolkitScanScopes.parseEntry(raw); } catch (_) { return {raw, canonical: raw, invalid: true}; }
    });
  }
  function selectedScopes() { return new Set(scopeEntries().filter(entry => !entry.invalid).map(entry => entry.canonical)); }
  function adapterCards(choices) {
    const selected = selectedScopes();
    return '<div class="network-picker-heading"><div><strong>Local networks</strong><span id="networkSelectionCount">' + choices.filter(choice => selected.has(choice.network)).length + ' of ' + choices.length + ' selected</span></div><button type="button" class="btn small ghost" data-action="refresh-system" ' + (!online ? 'disabled' : '') + ' title="Refresh local adapters">' + icon('refresh') + '<span>Refresh</span></button></div><div class="local-network-cards" role="group" aria-label="Add local networks to scan scopes">' + (choices.length ? choices.map(choice => {
      const added = selected.has(choice.network), label = adapterLabel(choice);
      return '<button type="button" class="local-network-card' + (added ? ' selected' : '') + '" data-network-toggle="' + esc(choice.key) + '" data-network-cidr="' + esc(choice.network) + '" aria-pressed="' + added + '" title="' + esc((added ? 'Remove ' : 'Add ') + choice.network + ' from scan scopes. ' + label) + '"><span class="local-network-mark">' + icon(/wi-?fi|wireless|802\.11/i.test(label) ? 'wifi' : 'network') + '</span><span class="local-network-text"><strong class="mono">' + esc(choice.network) + '</strong><span class="adapter-label">' + esc(label) + '</span><small>Local IP ' + esc(choice.address) + ' · ' + fmt(choice.hosts, 0) + ' hosts' + (choice.gateway ? ' · Gateway' : '') + '</small></span><span class="network-card-action">' + icon(added ? 'check' : 'plus') + '<span>' + (added ? 'Added' : 'Add') + '</span></span></button>';
    }).join('') : '<p class="help-line">No active IPv4 networks were reported. Enter a scope below or refresh the adapters.</p>') + '</div><p class="help-line network-picker-note">Click to add or remove a target network. Use Source interface below to choose where probes are sent from.</p>';
  }
  function updateAdapterCards() {
    const selected = selectedScopes(), cards = $$('[data-network-toggle]');
    for (const card of cards) {
      const added = selected.has(card.dataset.networkCidr);
      if (card.getAttribute('aria-pressed') !== String(added)) {
        card.setAttribute('aria-pressed', String(added)); card.classList.toggle('selected', added);
        const action = card.querySelector('.network-card-action');
        if (action) action.innerHTML = icon(added ? 'check' : 'plus') + '<span>' + (added ? 'Added' : 'Add') + '</span>';
      }
      const choice = localIPv4Choices().find(item => item.key === card.dataset.networkToggle);
      card.title = (added ? 'Remove ' : 'Add ') + card.dataset.networkCidr + ' from scan scopes.' + (choice ? ' ' + adapterLabel(choice) : '');
    }
    const count = $('#networkSelectionCount');
    if (count) count.textContent = cards.filter(card => selected.has(card.dataset.networkCidr)).length + ' of ' + cards.length + ' selected';
  }
  function updateScope(value) {
    setCurrentTargetValue(value); const input = $('#globalTarget'); if (input) input.value = value;
    updateNetworkRangeStatus(); updateRunButtons(); saveState();
  }
  function toggleAdapter(key) {
    const choice = localIPv4Choices().find(item => item.key === key); if (!choice) return;
    const entries = scopeEntries(), exists = entries.some(entry => entry.canonical === choice.network && !entry.invalid);
    const next = exists ? entries.filter(entry => entry.canonical !== choice.network || entry.invalid).map(entry => entry.raw) : [...entries.map(entry => entry.raw), choice.network];
    state.networkAdapter = choice.key;
    updateScope(next.join('\n'));
  }
  function scopeSummary(plan, tool) {
    const entries = scopeEntries(), unique = [...new Map(entries.map(entry => [entry.canonical, entry])).values()];
    const chips = unique.length ? '<div class="scope-chips" aria-label="Entered scan scopes">' + unique.slice(0, 100).map(entry => '<span class="scope-chip' + (entry.invalid ? ' invalid' : '') + '"><span class="mono" title="' + esc(entry.raw) + '">' + esc(entry.canonical) + '</span><button type="button" data-scope-remove="' + esc(entry.canonical) + '" aria-label="Remove scope ' + esc(entry.canonical) + '" title="Remove scope">' + icon('close') + '</button></span>').join('') + (unique.length > 100 ? '<span class="help-line">' + (unique.length - 100) + ' additional scopes in the input.</span>' : '') + '</div>' : '';
    const examples = '<details class="scope-examples" data-detail="scan-scope-examples"><summary>Scope formats and limits</summary><dl><dt>Network</dt><dd class="mono">192.168.1.0/24</dd><dt>Subnet mask</dt><dd class="mono">192.168.1.10/255.255.255.0</dd><dt>Individual host</dt><dd class="mono">192.168.2.25</dd><dt>Address range</dt><dd class="mono">192.168.1.1-254</dd><dt>Cross-network range</dt><dd class="mono">192.168.1.250-192.168.2.10</dd><dt>Wildcard network</dt><dd class="mono">192.168.1.*</dd></dl><p>Separate entries with new lines, commas, semicolons or spaces. Overlapping addresses are checked once. Network and wildcard scopes exclude ordinary network and broadcast addresses; /31 and /32 include all addresses. Explicit hosts and ranges retain their endpoints.</p><p>Maximum 1,024 unique hosts and 16,384 TCP connections per run' + (tool === 'discover' ? ', with up to 64 TCP ports' : '') + '.</p></details>';
    let summary;
    if (!plan.ok) summary = '<div class="notice ' + (String(state.networkRange).trim() ? 'warn' : '') + '">' + icon('info') + '<span>' + esc(plan.error) + '</span></div>';
    else {
      const checks = [(tool === 'ping' || tool === 'discover') ? '1 ICMP' : '', plan.ports.length ? plan.ports.length + (plan.ports.length === 1 ? ' TCP port' : ' TCP ports') : '', plan.hostnameLookups ? '1 hostname lookup' : '', plan.identityLookups ? '1 UPnP request' : ''].filter(Boolean).join(' + ');
      summary = '<div class="scope-plan" id="scanScopeSummary"><div class="scope-plan-counts"><span><strong>' + fmt(plan.scopes.length, 0) + '</strong>' + (plan.scopes.length === 1 ? 'scope' : 'scopes') + '</span><span><strong>' + fmt(plan.hosts, 0) + '</strong>' + (plan.hosts === 1 ? 'unique host' : 'unique hosts') + '</span><span><strong>' + fmt(plan.requestedProbes, 0) + '</strong>' + (plan.requestedProbes === 1 ? 'check' : 'checks') + '</span></div><div class="help-line">' + esc(checks) + ' per host</div>' + (plan.connections ? '<div class="help-line">' + fmt(plan.connections, 0) + ' TCP connections</div>' : '') + (plan.overlaps ? '<div class="help-line">' + fmt(plan.overlaps, 0) + ' overlapping addresses counted once.</div>' : '') + '</div>';
    }
    return chips + summary + examples;
  }
  function targetInput(network) {
    const old = $('#globalTarget'), desired = network ? 'TEXTAREA' : 'INPUT';
    if (!old || old.tagName === desired) return;
    const replacement = document.createElement(network ? 'textarea' : 'input');
    for (const attribute of old.attributes) if (attribute.name !== 'type') replacement.setAttribute(attribute.name, attribute.value);
    if (network) { replacement.rows = 2; replacement.maxLength = 16384; replacement.setAttribute('aria-describedby', 'scanScopeHelp targetHint'); }
    else { replacement.type = 'text'; replacement.removeAttribute('rows'); replacement.removeAttribute('maxlength'); replacement.setAttribute('aria-describedby', 'targetHint'); }
    replacement.value = old.value; old.replaceWith(replacement);
    $('#targetForm')?.classList.toggle('multiple-scopes', network);
  }

  function deviceIcon(row, neighbor) {
    const ports = new Set((row.openPorts || []).map(Number));
    let symbol = neighbor ? 'network' : 'server', title = neighbor ? 'Cached network neighbor' : 'Discovered address';
    if ([515, 631, 9100].some(port => ports.has(port))) { symbol = 'printer'; title = 'Printing-related TCP port is open. Device type is not verified.'; }
    else if ([3389, 5900].some(port => ports.has(port))) { symbol = 'monitor'; title = 'Remote desktop TCP port is open. Device type is not verified.'; }
    else if ([22, 23, 830].some(port => ports.has(port))) { symbol = 'terminal'; title = 'Remote administration TCP port is open. Service identity is not verified.'; }
    else if ([139, 445, 548, 2049].some(port => ports.has(port))) { symbol = 'harddrive'; title = 'File service TCP port is open. Device type is not verified.'; }
    else if ([80, 443, 8000, 8080, 8443].some(port => ports.has(port))) { symbol = 'globe'; title = 'Web-related TCP port is open. Service identity is not verified.'; }
    else if (!neighbor && row.responded === false) { symbol = 'server'; title = row.status === 'error' ? 'Probe error recorded' : 'No response evidence recorded'; }
    const graphic = symbol === 'printer' ? '<svg viewBox="0 0 24 24" aria-hidden="true"><path d="M6 9V3h12v6 M6 17H3V9h18v8h-3 M6 14h12v7H6z M17 12h1"/></svg>' : icon(symbol);
    return '<span class="device-kind-icon' + (!neighbor && row.responded === false ? ' muted' : '') + '" title="' + esc(title) + '" aria-label="' + esc(title) + '">' + graphic + '</span>';
  }
  function selectedRow(job) {
    const key = selectedByRun.get(runKey(job)); if (!key) return null;
    const snapshot = networkSnapshot(job), row = snapshot.rows.find(value => deviceRecordKey(job, value) === key);
    if (!row) return null;
    const record = {...row, address: usableDeviceAddress(row.address), host: usableDeviceAddress(row.address), sourceTool: job.tool, ...(!snapshot.neighbor ? {_recordedProbes: true, _probeFormat: snapshot.legacyDiscovery ? 'legacy' : 'full', _probeRecord: row} : {})};
    deviceRecords.set(key, record); return {key, row: record, neighbor: snapshot.neighbor};
  }
  function detailField(name, value, mono) { return '<div><dt>' + esc(name) + '</dt><dd' + (mono ? ' class="mono"' : '') + '>' + esc(value || 'Not reported') + '</dd></div>'; }
  function detailContent(job) {
    const selected = selectedRow(job);
    if (!selected) return '<div class="device-pane-empty">' + icon('server') + '<strong>Select a device</strong><p>Choose an address in the list to review its identity, response evidence and connection tools.</p><span>Use Enter on a focused row. Right-click a row for quick tools.</span></div>';
    const {key, row, neighbor} = selected, address = deviceTarget(row), mac = rowMAC(row), hostname = deviceHostname(row), samples = Array.isArray(row.samples) ? row.samples : [], statistics = row.statistics || {}, ports = Array.isArray(row.ports) ? row.ports : [], evidence = Array.isArray(row.evidence) ? row.evidence : [];
    const ssh = deviceApplicability(row, 'ssh'), telnet = deviceApplicability(row, 'telnet');
    const identity = detailField('IP address', address, true) + detailField('Hostname', hostname || 'Not resolved') + detailField('MAC address', mac || 'Not available', true) + detailField('MAC vendor assignment', macVendorLabel(row)) + (row.manufacturer ? detailField('Reported manufacturer', row.manufacturer) : '') + (row.model || row.modelName || row.modelNumber ? detailField('Reported model', deviceModel(row)) : '') + (row.interface || row.interfaceIndex || row.interfaceName || row.interfaceDescription ? detailField('Adapter', recordedInterfaceLabel(row)) : '');
    const sent = statistics.sent ?? samples.length, received = statistics.received ?? samples.filter(sample => sample.success).length;
    const icmp = neighbor ? '<p class="help-line">A neighbor cache entry does not confirm a current response.</p>' : row.sourceTool === 'tcp' ? '<p class="help-line">ICMP was not part of this TCP scan.</p>' : sent ? '<dl class="device-data-grid compact">' + detailField('ICMP replies', received + ' / ' + sent) + detailField('Packet loss', statistics.lossPercent != null ? fmt(statistics.lossPercent, 1) + '%' : fmt((sent - received) / sent * 100, 1) + '%') + detailField('Average RTT', received && statistics.avgMs != null ? fmt(statistics.avgMs) + ' ms' : received && row.latencyMs != null ? fmt(row.latencyMs) + ' ms' : 'Not measured') + (received && statistics.minMs != null ? detailField('RTT range', fmt(statistics.minMs) + ' to ' + fmt(statistics.maxMs) + ' ms') : '') + '</dl>' : '<p class="help-line">No ICMP attempts were recorded for this address.</p>';
    const tcp = ports.length ? '<div class="table-wrap device-port-table"><table data-column-view="selected-device:ports"><thead><tr><th data-column-id="port">Port</th><th data-column-id="service">Service label</th><th data-column-id="outcome">Outcome</th><th data-column-id="elapsed">Elapsed</th></tr></thead><tbody>' + ports.map(port => '<tr><td class="mono">' + esc(port.port) + '</td><td>' + esc(port.service || 'Unlabelled') + '</td><td>' + pill(port.state || 'unknown') + (port.error ? '<span class="device-port-error" title="' + esc(port.error) + '">' + esc(port.error) + '</span>' : '') + '</td><td>' + (port.latencyMs != null ? fmt(port.latencyMs) + ' ms' : 'Not recorded') + '</td></tr>').join('') + '</tbody></table></div><p class="help-line">Service labels are common port names, not verified identities.</p>' : '<p class="help-line">' + (row.openPorts?.length ? 'Open TCP ports: ' + esc(row.openPorts.join(', ')) + '. Individual attempts were not retained.' : row.sourceTool === 'ping' || neighbor ? 'TCP ports were not checked in this snapshot.' : 'No individual TCP attempts were recorded.') + '</p>';
    const diagnostics = [['ping', 'Ping'], ['trace', 'Trace route'], ['identify', 'Read device details']].map(([tool, name]) => {
      const allowed = deviceApplicability(row, tool);
      return '<button type="button" class="btn small ghost" data-run-device="' + tool + '" data-device-record="' + esc(key) + '" ' + (!allowed.ok || !online ? 'disabled' : '') + ' title="' + esc(allowed.reason || 'Run ' + name.toLowerCase() + ' for ' + address) + '">' + icon(TOOLS[tool].icon) + esc(name) + '</button>';
    }).join('');
    return '<div class="device-selected-heading">' + deviceIcon(row, neighbor) + '<div><strong>' + esc(hostname || address) + '</strong>' + (hostname ? '<span class="mono">' + esc(address) + '</span>' : '') + '</div>' + (!neighbor ? networkColumnCell('response', row, job) : pill('info', row.state || 'Cached neighbor')) + '</div><p class="selection-filter-note hidden" id="selectedDeviceFilterNote">Selected device is outside the current filter or page.</p><div class="device-copy-actions"><button type="button" class="btn small ghost" data-device-copy-details="' + esc(key) + '">' + icon('copy') + 'Copy details</button><button class="btn small ghost" data-entry-action="copy-ip" data-entry-record="' + esc(key) + '">Copy IP</button><button class="btn small ghost" data-entry-action="copy-mac" data-entry-record="' + esc(key) + '" ' + (!mac ? 'disabled' : '') + '>Copy MAC</button></div><dl class="device-data-grid">' + identity + '</dl><section class="device-pane-section"><h4>Connect</h4><div class="device-connect-actions"><button class="btn small" data-device-connect="ssh" data-device-record="' + esc(key) + '" ' + (!ssh.ok || !online ? 'disabled' : '') + ' title="' + esc(ssh.reason || 'Review SSH connection') + '">' + icon('lock') + 'SSH</button><button class="btn small" data-device-connect="telnet" data-device-record="' + esc(key) + '" ' + (!telnet.ok || !online ? 'disabled' : '') + ' title="' + esc(telnet.reason || 'Review Telnet connection') + '">' + icon('terminal') + 'Telnet</button><button class="btn small ghost" data-device-key="' + esc(key) + '">' + icon('settings') + 'All tools</button></div><div class="device-open-actions">' + deviceConnectionButtons(key) + '</div></section><section class="device-pane-section"><h4>Diagnostics</h4><div class="device-connect-actions">' + diagnostics + '</div></section><section class="device-pane-section"><h4>Response evidence</h4><p class="help-line">' + esc(neighbor ? 'Local neighbor cache. ' + (row.state || '') : evidence.length ? evidence.join(', ') : row.responded ? 'Response recorded. Individual evidence is not available.' : 'No response evidence recorded. An unanswered probe does not prove that a device is offline.') + '</p>' + icmp + (row.icmpError ? '<p class="help-line warn">' + esc(row.icmpError) + '</p>' : '') + '</section><section class="device-pane-section"><h4>TCP checks' + (ports.length ? ' (' + ports.length + ')' : '') + '</h4>' + tcp + '</section>' + (row._recordedProbes ? '<button class="btn small ghost" data-entry-action="probe-details" data-entry-record="' + esc(key) + '">' + icon('report') + 'Complete recorded probes</button>' : '') + '<div class="device-source-notes">' + [row.hostnameNote, row.vendorSource ? 'Vendor source: ' + row.vendorSource : '', row.modelSource ? 'Model source: ' + row.modelSource : '', row.identityError, row.identityNote, row.neighborNote].filter(Boolean).map(note => '<p>' + esc(note) + '</p>').join('') + '<p>MAC vendor assignments can identify a component supplier. Device icons describe observed ports or cache entries.</p></div>';
  }
  function detailPane(job) {
    return '<aside id="selectedDevicePane" aria-label="Selected device details"><div class="device-pane-heading"><button type="button" class="iconbtn" data-device-pane-hide aria-controls="selectedDevicePane" aria-label="Hide device details" title="Hide device details">' + root.NetworkToolkitPanelControls.icon('right', true) + '</button><strong>' + icon('info') + 'Device details</strong><div><label class="sr-only" for="deviceDetailsDock">Details position</label><select id="deviceDetailsDock" title="Details position"><option value="auto"' + (preferences().dock === 'auto' ? ' selected' : '') + '>Auto</option><option value="right"' + (preferences().dock === 'right' ? ' selected' : '') + '>Right</option><option value="bottom"' + (preferences().dock === 'bottom' ? ' selected' : '') + '>Bottom</option></select></div></div><div id="selectedDeviceContent" tabindex="0" role="region" aria-label="Selected device information">' + detailContent(job) + '</div></aside>';
  }
  function prepareResult(template, job) {
    const body = template.content.querySelector('#toolResultsScroll'), group = body?.querySelector('[data-network-results]');
    if (!job || !group) { prepareTables(template.content); return; }
    const workspace = document.createElement('div'); workspace.id = 'networkResultsWorkspace'; workspace.dataset.networkJob = job.id; workspace.dataset.runKey = runKey(job);
    workspace.className = 'network-results-workspace' + (preferences().hidden ? ' details-hidden' : '');
    body.before(workspace); workspace.append(body);
    workspace.insertAdjacentHTML('beforeend', '<div id="deviceDetailsDivider" role="separator" tabindex="0" aria-label="Resize or hide device details" aria-controls="selectedDevicePane" aria-orientation="vertical" aria-valuemin="260" aria-valuemax="700" aria-valuenow="' + Math.round(preferences().width) + '" title="Drag to resize. Double-click or press Enter or Space to hide details. Arrow keys resize; Home resets."></div>' + detailPane(job));
    const key = selectedByRun.get(runKey(job));
    for (const row of workspace.querySelectorAll('[data-entry-key]')) if (row.dataset.entryKey === key) { row.classList.add('device-selected'); row.setAttribute('aria-current', 'true'); }
    for (const button of workspace.querySelectorAll('[data-select-device]')) button.setAttribute('aria-pressed', String(button.dataset.selectDevice === key));
    prepareTables(template.content);
  }
  function layoutDetails() {
    const workspace = $('#networkResultsWorkspace'); if (!workspace) return;
    const pref = preferences(), bounds = workspace.getBoundingClientRect();
    const toolbar = workspace.querySelector('.network-list-toolbar'), minimumListHeight = (toolbar?.getBoundingClientRect().height || 90) + 138;
    const short = bounds.height < minimumListHeight + 170;
    const bottom = pref.dock === 'bottom' || bounds.width < 900;
    const drawer = bottom && short, emptyDrawer = drawer && !selectedByRun.get(workspace.dataset.runKey);
    workspace.classList.toggle('details-bottom', bottom); workspace.classList.toggle('details-right', !bottom); workspace.classList.toggle('details-hidden', pref.hidden);
    workspace.classList.toggle('details-drawer', drawer); workspace.classList.toggle('details-drawer-idle', emptyDrawer);
    const maxWidth = Math.max(260, Math.min(700, bounds.width - 320)), width = clamp(pref.width, 260, maxWidth), maxHeight = Math.max(80, Math.min(650, bounds.height * 0.6, bounds.height - minimumListHeight)), height = clamp(pref.height, 80, maxHeight);
    workspace.style.setProperty('--device-details-width', Math.round(width) + 'px'); workspace.style.setProperty('--device-details-height', Math.round(height) + 'px');
    const divider = $('#deviceDetailsDivider');
    if (divider) {
      divider.setAttribute('aria-orientation', bottom ? 'horizontal' : 'vertical'); divider.setAttribute('aria-valuenow', pref.hidden ? '0' : String(Math.round(bottom ? height : width))); divider.setAttribute('aria-valuemin', pref.hidden ? '0' : bottom ? '80' : '260'); divider.setAttribute('aria-valuemax', String(Math.round(bottom ? maxHeight : maxWidth)));
      divider.setAttribute('aria-label', pref.hidden ? 'Show device details' : 'Resize or hide device details'); divider.setAttribute('aria-valuetext', pref.hidden ? 'Collapsed' : Math.round(bottom ? height : width) + ' pixels');
      divider.title = pref.hidden ? 'Double-click or press Enter or Space to show device details.' : 'Drag to resize. Double-click or press Enter or Space to hide details. Arrow keys resize; Home resets.';
    }
    let rail = $('#deviceDetailsRail');
    if (!rail) { rail = document.createElement('div'); rail.id = 'deviceDetailsRail'; rail.className = 'device-details-rail'; rail.innerHTML = '<button type="button" class="iconbtn" data-device-pane-toggle aria-controls="selectedDevicePane"></button>'; workspace.append(rail); }
    for (const button of $$('[data-device-pane-toggle],[data-device-pane-hide]')) root.NetworkToolkitPanelControls.set(button, {name: 'device details', edge: bottom ? 'bottom' : 'right', expanded: !pref.hidden});
    root.NetworkToolkitPanelControls.bindReveal(rail, {collapsed: () => preferences().hidden, button: () => rail.querySelector('[data-device-pane-toggle]')});
    if (resizeObserver && observedWorkspace !== workspace) { if (observedWorkspace) resizeObserver.unobserve(observedWorkspace); observedWorkspace = workspace; resizeObserver.observe(workspace); }
    requestAnimationFrame(updateToolResultSticky);
  }
  function syncSelection() {
    const workspace = $('#networkResultsWorkspace'); if (!workspace) return;
    const key = selectedByRun.get(workspace.dataset.runKey); let visible = false;
    for (const row of $$('#toolResultsScroll [data-entry-key]', workspace)) {
      const selected = row.dataset.entryKey === key; row.classList.toggle('device-selected', selected);
      if (selected) { row.setAttribute('aria-current', 'true'); visible = true; } else row.removeAttribute('aria-current');
    }
    for (const button of $$('[data-select-device]', workspace)) button.setAttribute('aria-pressed', String(button.dataset.selectDevice === key));
    $('#selectedDeviceFilterNote')?.classList.toggle('hidden', visible);
    layoutDetails();
  }
  function refreshDetails(job, resetScroll) {
    const content = $('#selectedDeviceContent'), workspace = $('#networkResultsWorkspace');
    if (!content || !workspace || workspace.dataset.networkJob !== String(job.id)) return;
    const top = resetScroll ? 0 : content.scrollTop;
    patchResultHTML(content, detailContent(job)); content.scrollTop = top; syncSelection();
  }
  function selectDevice(key, jobId) {
    const job = networkResultJobs.get(jobId); if (!job) return;
    selectedByRun.set(runKey(job), key); while (selectedByRun.size > 220) selectedByRun.delete(selectedByRun.keys().next().value);
    if (preferences().hidden) { preferences().hidden = false; saveState(); }
    refreshDetails(job, true);
  }
  function copyDetails(key) {
    const row = deviceRecords.get(key); if (!row) return;
    const lines = ['Device: ' + deviceTarget(row), 'Hostname: ' + (deviceHostname(row) || 'Not resolved'), 'MAC address: ' + (rowMAC(row) || 'Not available'), 'MAC vendor assignment: ' + macVendorLabel(row), 'Reported manufacturer: ' + (row.manufacturer || 'Not reported'), 'Reported model: ' + deviceModel(row), 'Outcome: ' + (row.status || row.state || (row.responded ? 'Responded' : 'Not reported')), 'Response evidence: ' + (row.evidence?.join(', ') || 'Not recorded'), 'Open TCP ports: ' + (row.openPorts?.join(', ') || 'None recorded')];
    if (row.statistics) for (const [name, value] of Object.entries(row.statistics)) lines.push('ICMP ' + label(name) + ': ' + value);
    for (const port of row.ports || []) lines.push('TCP ' + port.port + ': ' + (port.state || 'Unknown') + (port.service ? ' (' + port.service + ')' : '') + (port.latencyMs != null ? ', ' + port.latencyMs + ' ms' : '') + (port.error ? ', ' + port.error : ''));
    void copyText(lines.join('\n'));
  }

  function columnId(header, index) { return header.dataset.columnId || header.querySelector('[data-network-sort]')?.dataset.networkSort || header.querySelector('[data-list-sort]')?.dataset.listSort || header.textContent.trim().replace(/\b(?:Sort|Asc|Desc)\s*$/, '').toLowerCase().replace(/[^a-z0-9]+/g, '-').slice(0, 80) || 'column-' + index; }
  function defaultWidth(id, name, view = '') {
    if (view === 'result:dns:records' && /^(?:name|value)$/.test(id)) return 200;
    if (/^(?:tools|actions)$/.test(id)) return 90;
    if (/^(?:address|ip|ip-address|ipv4-address)$/.test(id)) return 166;
    if (/mac/i.test(id)) return 165;
    if (/hostname|adapter|interface|vendor|manufacturer/i.test(id)) return 180;
    if (/error|detail|description|note|subject|issuer|endpoint|path|command|destination/i.test(id)) return 240;
    if (/^ports?$/.test(id)) return id === 'port' ? 76 : 140;
    if (/latency|elapsed|rtt|ttl|count|sequence|metric|loss|size|process|pid|prefix/i.test(id)) return 100;
    return Math.min(220, Math.max(112, name.length * 7 + 40));
  }
  function viewKey(table, headers) {
    if (table.dataset.columnView) return table.dataset.columnView;
    const list = table.closest('[data-list-view]'), config = list && dataListViews.get(list.dataset.listView);
    if (config) return config.view;
    const network = table.closest('[data-network-results]'), job = network && networkResultJobs.get(network.dataset.networkResults);
    if (job) return 'network:' + job.tool;
    return ('table:' + state.activePage + ':' + (state.activePage === 'local' ? state.localTab + ':' : '') + headers.map((header, index) => columnId(header, index)).join('|')).slice(0, 240);
  }
  function widthFor(view, id, name) { return Math.round(clamp(state.tableWidths?.[view]?.[id] || defaultWidth(id, name, view), 64, 1200)); }
  const widthsForRow = (table, index, name, id) => {
    // Estimate natural text width from a bounded number of visible rows.
    // Leave large text fields wrappable instead of consuming the whole view.
    const rows = table.tBodies[0]?.rows || [];
    const base = /^(tools|actions)$/.test(id) ? 110 : Math.max(70, name.length * 7 + 30);
    let best = base;
    for (let j = 0; j < Math.min(rows.length, 36); j++) {
      const cell = rows[j].cells[index]; if (!cell || cell.colSpan > 1) continue;
      const button = cell.querySelector('button:not([data-value-copy-control]):not(.ntk-inline-copy)');
      if (button) {
        best = Math.max(best, Math.min(330, (button.textContent || '').trim().length * 7 + 54));
        continue;
      }
      const text = (cell.innerText || cell.textContent || '').trim().replace(/\s+/g, ' ');
      const estimate = text.length * 6.6 + (cell.querySelector('[data-value-copy-control],.ntk-inline-copy') ? 36 : 20);
      best = Math.max(best, Math.min(estimate, /value|detail|description|hostname|name|path|server|error|info/i.test(id) ? 440 : 230));
    }
    return Math.ceil(clamp(best, /^(tools|actions)$/.test(id) ? 108 : 72, 500));
  };
  function prepareTables(scope) {
    if (!scope?.querySelectorAll) return;
    const tables = [...(scope.matches?.('table') ? [scope] : []), ...scope.querySelectorAll('table')];
    for (const table of tables) {
      if (table.closest('.xterm') || table.dataset.noColumnResize !== undefined || table.classList.contains('fm-table') || table.classList.contains('packet-table')) continue;
      const headerRow = table.tHead?.rows[table.tHead.rows.length - 1] || table.querySelector('tr');
      const headers = headerRow ? [...headerRow.cells] : [];
      if (!headers.length || headers.some(header => header.tagName !== 'TH' || header.colSpan > 1 || header.rowSpan > 1)) continue;
      const view = viewKey(table, headers); table.dataset.columnView = view; table.classList.add('resizable-data-table');
      let group = table.querySelector('colgroup[data-column-sizes]');
      if (!group) { group = document.createElement('colgroup'); group.dataset.columnSizes = ''; table.prepend(group); }
      const definitions = headers.map((header,index) => {
        const id = columnId(header,index), name = header.dataset.columnName || header.querySelector('.table-sort')?.childNodes[0]?.textContent?.trim() || header.textContent.trim().replace(/\b(?:Sort|Asc|Desc)\s*$/, '') || 'Column ' + (index+1);
        header.dataset.columnId = id; header.dataset.columnName = name; header.scope = 'col';
        const manual = state.tableWidths?.[view]?.[id];
        const width = Number.isFinite(manual) ? widthFor(view,id,name) : Math.max(72, widthsForRow(table,index,name,id));
        return {id,name,width,manual:Number.isFinite(manual),header};
      });
      const wrapper = table.closest('.table-wrap') || table.parentElement;
      const available = Math.floor(wrapper?.clientWidth || 0);
      let total = definitions.reduce((sum,item)=>sum+item.width,0);
      // Distribute unused width; keep pinned/manual widths stable and put the
      // extra room where longer strings benefit from fewer wrapped lines.
      if (available>total && total>0) {
        const surplus=available-total;
        let flexible=definitions.filter(item=>!item.manual);
        if (!flexible.length) flexible=definitions;
        const weight=item=>/(?:value|name|description|hostname|detail|address|server|path|info)/i.test(item.id)?3: /^(?:tools|actions)$/.test(item.id)?0.12:1;
        const weights=flexible.reduce((sum,item)=>sum+weight(item),0);
        for (const item of flexible) item.width+=Math.round(surplus*weight(item)/weights);
        total=definitions.reduce((sum,item)=>sum+item.width,0);
      }
      definitions.forEach(({header,id,name,width},index) => {
        let column = group.children[index]; if (!column) { column=document.createElement('col'); group.append(column); }
        column.dataset.columnId=id; column.style.width=width+'px';
        let handle=header.querySelector('[data-column-resize]');
        if (!handle) { handle=document.createElement('span'); handle.className='column-resize-handle'; handle.dataset.columnResize=id; handle.tabIndex=0; handle.setAttribute('role','separator'); handle.setAttribute('aria-orientation','vertical'); header.append(handle); }
        handle.setAttribute('aria-label','Resize '+name+' column'); handle.setAttribute('aria-valuemin','64'); handle.setAttribute('aria-valuemax','1200'); handle.setAttribute('aria-valuenow',String(Math.round(width))); handle.title='Drag to resize. Double-click or Enter fits contents. Arrow keys resize; Home resets.';
      });
      while (group.children.length>headers.length) group.lastElementChild.remove();
      table.style.width=Math.max(total,available)+'px';
      let toolbar=table.closest('[data-list-view]')?.querySelector(':scope > .data-list-tools');
      if (!toolbar && view.startsWith('network:')) toolbar=table.closest('[data-network-results]')?.querySelector(':scope > .network-list-toolbar > .network-table-tools');
      if (!toolbar && !view.startsWith('network:') && table.dataset.columnOwner!=='list') { const wrapper=table.closest('.table-wrap'); if(wrapper){toolbar=[...wrapper.children].find(child=>child.classList.contains('table-size-tools'));if(!toolbar){toolbar=document.createElement('div');toolbar.className='table-size-tools';wrapper.insertBefore(toolbar,table);}}}
      if (toolbar && ![...toolbar.querySelectorAll('[data-column-reset]')].some(button=>button.dataset.columnReset===view)) {
        const button=document.createElement('button');button.type='button';button.className='btn small ghost table-width-reset';button.dataset.columnReset=view;button.title='Restore responsive column widths for this table';button.innerHTML=icon('refresh')+'<span>Reset widths</span>';toolbar.append(button);
      }
    }
  }
  function storeWidth(table, id, width, persist) {
    const view = table.dataset.columnView; state.tableWidths ||= {}; state.tableWidths[view] ||= {}; state.tableWidths[view][id] = Math.round(clamp(width, 64, 1200));
    for (const candidate of $$('table[data-column-view]')) if (candidate.dataset.columnView === view) prepareTables(candidate);
    if (persist) saveState();
    requestAnimationFrame(updateToolResultSticky);
  }
  function autoFit(table, id) {
    const headers = [...table.querySelectorAll('thead tr:last-child th')], index = headers.findIndex(header => header.dataset.columnId === id); if (index < 0) return;
    const canvas = document.createElement('canvas'), context = canvas.getContext('2d'); let width = 64;
    const cells = [headers[index], ...[...table.tBodies].flatMap(body => [...body.rows].slice(0, 200).map(row => row.cells[index]).filter(Boolean))];
    for (const cell of cells) {
      const style = getComputedStyle(cell); context.font = style.font || (style.fontSize + ' ' + style.fontFamily);
      const lines = (cell === headers[index] ? cell.dataset.columnName : cell.innerText || cell.textContent).split(/\n/);
      const textWidth = Math.max(...lines.map(line => context.measureText(line.trim()).width), 0);
      const padding = parseFloat(style.paddingLeft) + parseFloat(style.paddingRight) + (cell === headers[index] ? 42 : cell.querySelector('svg') ? 30 : 8);
      width = Math.max(width, Math.ceil(textWidth + padding));
    }
    storeWidth(table, id, width, true);
  }
  function resetWidths(view) {
    if (state.tableWidths) delete state.tableWidths[view];
    for (const table of $$('table[data-column-view]')) if (table.dataset.columnView === view) prepareTables(table);
    saveState(); requestAnimationFrame(updateToolResultSticky);
  }
  function afterRender() {
    prepareTables(document); syncSelection();
  }
  function schedule() { if (queued) return; queued = true; requestAnimationFrame(() => { queued = false; afterRender(); }); }
  function resizePane(event, divider) {
    if (preferences().hidden) { event.preventDefault(); event.stopPropagation(); return; }
    const workspace = $('#networkResultsWorkspace'), bottom = workspace.classList.contains('details-bottom'), start = bottom ? event.clientY : event.clientX, pref = preferences(), initial = bottom ? $('#selectedDevicePane').getBoundingClientRect().height : $('#selectedDevicePane').getBoundingClientRect().width;
    event.preventDefault(); event.stopPropagation(); divider.classList.add('dragging'); document.body.classList.add(bottom ? 'resizing-rows' : 'resizing-columns');
    const move = next => { const amount = initial - ((bottom ? next.clientY : next.clientX) - start); pref[bottom ? 'height' : 'width'] = clamp(amount, Number(divider.getAttribute('aria-valuemin')), Number(divider.getAttribute('aria-valuemax'))); layoutDetails(); };
    const finish = () => { window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', finish); window.removeEventListener('pointercancel', finish); divider.classList.remove('dragging'); document.body.classList.remove('resizing-rows', 'resizing-columns'); saveState(); };
    window.addEventListener('pointermove', move); window.addEventListener('pointerup', finish); window.addEventListener('pointercancel', finish);
  }
  let lastWorkspaceTableWidth=0;
  function init() {
    if (started) return; started = true;
    if (typeof ResizeObserver === 'function') resizeObserver = new ResizeObserver(layoutDetails);
    const observer = new MutationObserver(records => { if (records.some(record => record.type === 'childList' && [...record.addedNodes].some(node => node.nodeType === 1 && (node.matches?.('table,thead,tbody,tr,th,#networkResultsWorkspace') || node.querySelector?.('table,#networkResultsWorkspace'))))) schedule(); });
    observer.observe(document.body, {subtree: true, childList: true});
    document.addEventListener('click', event => {
      const target = event.target;
      if (target.closest('[data-column-resize]')) { event.stopImmediatePropagation(); event.preventDefault(); return; }
      const toggle = target.closest('[data-network-toggle]'); if (toggle) { event.stopImmediatePropagation(); toggleAdapter(toggle.dataset.networkToggle); return; }
      const remove = target.closest('[data-scope-remove]'); if (remove) { event.stopImmediatePropagation(); updateScope(scopeEntries().filter(entry => entry.canonical !== remove.dataset.scopeRemove).map(entry => entry.raw).join('\n')); return; }
      const reset = target.closest('[data-column-reset]'); if (reset) { event.stopImmediatePropagation(); resetWidths(reset.dataset.columnReset); return; }
      const paneToggle = target.closest('[data-device-pane-toggle], [data-device-pane-hide]'); if (paneToggle) { event.stopImmediatePropagation(); preferences().hidden = paneToggle.hasAttribute('data-device-pane-hide') || !preferences().hidden; layoutDetails(); saveState(); (preferences().hidden ? $('#deviceDetailsRail button') : $('#selectedDevicePane [data-device-pane-hide]'))?.focus({preventScroll: true}); return; }
      const copy = target.closest('[data-device-copy-details]'); if (copy) { event.stopImmediatePropagation(); copyDetails(copy.dataset.deviceCopyDetails); return; }
      const row = target.closest('#toolResultsScroll [data-network-results] tr[data-entry-key]');
      if (row && !row.hasAttribute('data-result-row') && (!target.closest('button,input,select,textarea,a,[role="separator"]') || target.closest('[data-select-device]')) && window.getSelection()?.isCollapsed !== false) { event.stopImmediatePropagation(); selectDevice(row.dataset.entryKey, row.closest('[data-network-results]').dataset.networkResults); }
      else if (!row) { const entry = target.closest('[data-select-device]'); if (entry) { event.stopImmediatePropagation(); const record = deviceRecords.get(entry.dataset.selectDevice); if (record?._recordedProbes) openProbeDetails(entry.dataset.selectDevice); else openDeviceTools(entry.dataset.selectDevice); } }
    }, true);
    document.addEventListener('change', event => { if (event.target.id === 'deviceDetailsDock') { preferences().dock = event.target.value; layoutDetails(); saveState(); } });
    document.addEventListener('dblclick', event => { const handle = event.target.closest('[data-column-resize]'); if (handle) { event.preventDefault(); event.stopPropagation(); autoFit(handle.closest('table'), handle.dataset.columnResize); } else if (event.target.id === 'deviceDetailsDivider') { event.preventDefault(); event.stopPropagation(); preferences().hidden = !preferences().hidden; layoutDetails(); saveState(); } });
    document.addEventListener('pointerdown', event => {
      if (event.button !== 0) return;
      const handle = event.target.closest('[data-column-resize]');
      if (handle) {
        event.preventDefault(); event.stopPropagation(); handle.focus({preventScroll: true});
        const table = handle.closest('table'), id = handle.dataset.columnResize, start = event.clientX, initial = Number(handle.getAttribute('aria-valuenow'));
        handle.classList.add('dragging'); document.body.classList.add('resizing-columns');
        const move = next => storeWidth(table, id, initial + next.clientX - start, false);
        const finish = () => { window.removeEventListener('pointermove', move); window.removeEventListener('pointerup', finish); window.removeEventListener('pointercancel', finish); handle.classList.remove('dragging'); document.body.classList.remove('resizing-columns'); saveState(); };
        window.addEventListener('pointermove', move); window.addEventListener('pointerup', finish); window.addEventListener('pointercancel', finish);
      } else if (event.target.id === 'deviceDetailsDivider') resizePane(event, event.target);
    }, true);
    document.addEventListener('keydown', event => {
      const handle = event.target.closest('[data-column-resize]');
      if (handle && ['ArrowLeft', 'ArrowRight', 'Home', 'Enter'].includes(event.key)) {
        event.preventDefault(); event.stopImmediatePropagation(); const table = handle.closest('table'), id = handle.dataset.columnResize;
        if (event.key === 'Enter') autoFit(table, id);
        else storeWidth(table, id, event.key === 'Home' ? defaultWidth(id, handle.closest('th').dataset.columnName, table.dataset.columnView) : Number(handle.getAttribute('aria-valuenow')) + (event.key === 'ArrowRight' ? 1 : -1) * (event.shiftKey ? 25 : 10), true);
        return;
      }
      if (event.target.id === 'deviceDetailsDivider' && ['Enter', ' ', 'ArrowLeft', 'ArrowRight', 'ArrowUp', 'ArrowDown', 'Home'].includes(event.key)) {
        event.preventDefault(); event.stopImmediatePropagation();
        if (['Enter', ' '].includes(event.key)) { preferences().hidden = !preferences().hidden; layoutDetails(); saveState(); return; }
        if (preferences().hidden) { preferences().hidden = false; layoutDetails(); }
        const bottom = $('#networkResultsWorkspace').classList.contains('details-bottom'), name = bottom ? 'height' : 'width', sign = ['ArrowLeft', 'ArrowUp'].includes(event.key) ? 1 : -1;
        preferences()[name] = event.key === 'Home' ? defaults[name] : clamp(Number(event.target.getAttribute('aria-valuenow')) + sign * (event.shiftKey ? 30 : 10), Number(event.target.getAttribute('aria-valuemin')), Number(event.target.getAttribute('aria-valuemax'))); layoutDetails(); saveState(); return;
      }
      if (event.target.id === 'globalTarget' && event.target.tagName === 'TEXTAREA' && event.key === 'Enter' && (event.ctrlKey || event.metaKey)) { event.preventDefault(); event.stopImmediatePropagation(); void startJob(); return; }
      const row = event.target.closest('#toolResultsScroll [data-network-results] tr[data-entry-key]');
      if (row && !row.hasAttribute('data-result-row') && event.target === row && ['Enter', ' ', 'ArrowUp', 'ArrowDown'].includes(event.key)) {
        event.preventDefault(); event.stopImmediatePropagation(); let next = row;
        if (event.key === 'ArrowUp' || event.key === 'ArrowDown') { const rows = [...row.parentElement.children]; next = rows[clamp(rows.indexOf(row) + (event.key === 'ArrowDown' ? 1 : -1), 0, rows.length - 1)]; next.focus({preventScroll: true}); next.scrollIntoView({block: 'nearest', inline: 'nearest'}); }
        selectDevice(next.dataset.entryKey, next.closest('[data-network-results]').dataset.networkResults);
      }
    }, true);
    window.addEventListener('resize', ()=>{layoutDetails();schedule();});
    if(typeof ResizeObserver==='function'){const owner=document.getElementById('toolWorkspace');if(owner){const tableObserver=new ResizeObserver(entries=>{const width=Math.round(entries[0]?.contentRect?.width||0);if(width&&width!==lastWorkspaceTableWidth){lastWorkspaceTableWidth=width;schedule();}});tableObserver.observe(owner);}}
    document.addEventListener('scroll', event => { if (event.target.id === 'toolResultsScroll') event.target.style.setProperty('--result-scroll-x', event.target.scrollLeft + 'px'); }, {capture: true, passive: true});
  }
  root.NetworkToolkitWorkspace = Object.freeze({init, mergePreferences, adapterCards, updateAdapterCards, scopeSummary, targetInput, deviceIcon, prepareResult, prepareTables, afterRender, syncSelection, refreshDetails, layoutDetails});
})(globalThis);
