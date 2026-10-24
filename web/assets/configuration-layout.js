/* Compact configuration surfaces. All diagnostic values still use the existing form state. */
(function (root) {
  'use strict';
  let initialized = false, queued = false;
  const networkTabs = ['local', 'routed', 'selected'];
  const tcpPresets = [
    ['common', 'Common services', '22,53,80,443,445,3389'],
    ['windows', 'Windows / AD', '53,88,135,139,389,445,464,636,3268,3269,3389,5985,5986'],
    ['remote', 'Remote access', '22,23,443,3389,5900,5985,5986'],
    ['web', 'Web services', '80,443,8000,8080,8443'],
    ['extended', 'Expanded common', '20-25,53,67-69,80,110,123,135,139,143,161,389,443,445,465,587,636,993,995,1433,1521,3306,3389,5432,5900,8080,8443']
  ];
  const isRecord = value => value && typeof value === 'object' && !Array.isArray(value);
  function normalizeSections(value) {
    const result = {};
    if (!isRecord(value)) return result;
    for (const [key, open] of Object.entries(value).slice(0, 100)) {
      if (/^[a-z][a-z0-9-]{0,40}\.[a-z][a-z0-9-]{0,40}$/.test(key) && typeof open === 'boolean') result[key] = open;
    }
    return result;
  }
  function mergePreferences(saved) {
    state.configurationSections = normalizeSections(saved.configurationSections);
    state.configurationNetworkTab = networkTabs.includes(saved.configurationNetworkTab) ? saved.configurationNetworkTab : 'local';
  }
  function sectionOpen(key, fallback = false) {
    return typeof state.configurationSections?.[key] === 'boolean' ? state.configurationSections[key] : fallback;
  }
  function section(key, title, body, summary = '', symbol = 'settings', open = false) {
    return '<details class="configuration-section" data-config-section="' + esc(key) + '" data-config-default="' + open + '"' + (sectionOpen(key, open) ? ' open' : '') +
      '><summary><span class="configuration-section-title">' + icon(symbol) + esc(title) + '</span><span class="configuration-section-summary" data-config-section-summary>' + esc(summary) +
      '</span><span class="configuration-modified" hidden>Modified</span>' + icon('chevron', 'configuration-chevron') + '</summary><div class="configuration-section-body">' + body + '</div></details>';
  }
  function fieldsFor(tool, config) {
    const primary = [], secondary = [...config.advanced];
    const secondaryIDs = tool === 'ping' ? (networkMode(tool) ? ['timeoutMs', 'concurrency'] : ['timeoutMs'])
      : tool === 'tcp' && networkMode(tool) ? ['timeoutMs', 'concurrency'] : [];
    for (const field of config.primary) (secondaryIDs.includes(field.id) ? secondary : primary).push(field);
    return {primary, secondary};
  }
  function field(tool, item) {
    let html = fieldHTML(tool, item);
    if (item.type === 'checkbox' && item.desc) {
      const id = 'opt-' + tool + '-' + item.id + '-description';
      html = html.replace('<label class="toggle-row">', '<label class="toggle-row" title="' + esc(item.desc) + '">')
        .replace(' data-tool=', ' aria-describedby="' + id + '" data-tool=')
        .replace('<p>', '<p id="' + id + '" class="configuration-field-description">');
    }
    return html;
  }
  function signature(value) { try { return scanPorts(String(value), 1024).join(','); } catch (_) { return null; } }
  function tcpPreset() {
    const form = state.forms.tcp || {}, value = signature(form.ports);
    if (form.portPreset === 'custom') return 'custom';
    return tcpPresets.find(item => signature(item[2]) === value)?.[0] || 'custom';
  }
  function tcpPresetHTML() {
    const chosen = tcpPreset();
    return '<div class="field wide configuration-port-preset"><label for="tcpPortPreset">Port list</label><select id="tcpPortPreset">' +
      tcpPresets.map(([id, name, ports]) => '<option value="' + id + '"' + (chosen === id ? ' selected' : '') + '>' + esc(name) + ' (' + scanPorts(ports, 1024).length + ' ports)</option>').join('') +
      '<option value="custom"' + (chosen === 'custom' ? ' selected' : '') + '>Custom ports</option></select></div>';
  }
  function selectTCPPreset(id) {
    const preset = tcpPresets.find(item => item[0] === id), form = state.forms.tcp;
    if (!preset && id !== 'custom') return;
    if (tcpPreset() === 'custom') form.customPorts = String(form.ports ?? '');
    if (preset) form.ports = preset[2];
    else if (typeof form.customPorts === 'string') form.ports = form.customPorts;
    form.portPreset = id;
    const input = document.getElementById('opt-tcp-ports');
    if (input) input.value = form.ports;
    updateNetworkRangeStatus(); updateRunButtons(); saveState(); update();
  }
  function valueFor(tool, option) { return state.forms[tool]?.[option.id] ?? option.value; }
  function fieldSummary(tool, option) {
    const value = valueFor(tool, option), name = option.id;
    if (name === 'timeoutMs') return String(value) + ' ms timeout';
    if (name === 'intervalMs') return String(value) + ' ms interval';
    if (name === 'concurrency') return String(value) + (networkMode(tool) ? ' concurrent hosts' : ' concurrent connections');
    if (name === 'serverName') return value ? 'SNI: ' + value : 'SNI: target name';
    if (name === 'redirects') return 'Redirects ' + (value ? 'on' : 'off');
    if (name === 'verifyTLS') return 'TLS verification ' + (value ? 'on' : 'OFF');
    return option.name + ': ' + (typeof value === 'boolean' ? value ? 'on' : 'off' : String(value));
  }
  function optionsHTML(tool, config) {
    const grouped = fieldsFor(tool, config), name = ['http', 'tls'].includes(tool) ? 'options' : 'timing';
    let html = '<div class="configuration-primary tool-options">' + (tool === 'discover' ? discoveryPortPresetHTML() : tool === 'tcp' ? tcpPresetHTML() : '') +
      grouped.primary.map(item => field(tool, item)).join('') + '</div>';
    if (tool === 'dns') html += '<div class="preset-row configuration-resolver-presets"><span class="sr-only">Resolver presets</span>' +
      '<button type="button" class="chip" data-dns-server="">' + (state.diagnosticSource ? 'Adapter DNS' : 'System DNS') + '</button>' +
      '<button type="button" class="chip" data-dns-server="1.1.1.1">1.1.1.1</button><button type="button" class="chip" data-dns-server="8.8.8.8">8.8.8.8</button></div>';
    if (grouped.secondary.length) html += section(tool + '.' + name, tool === 'http' ? 'Request options' : tool === 'tls' ? 'TLS options' : 'Timing and limits',
      '<div class="configuration-secondary tool-options">' + grouped.secondary.map(item => field(tool, item)).join('') + '</div>', grouped.secondary.map(item => fieldSummary(tool, item)).join(' | '));
    const notes = [config.note || ''];
    if (tool === 'dns' && state.diagnosticSource) notes[0] = 'Leave DNS server empty to use a compatible resolver configured on the selected source adapter. No compatible resolver means hostname queries cannot run; literal-address probes remain available. PTR accepts an IP address. SRV accepts a service name.';
    if (['quick', 'tcp', 'discover'].includes(tool)) notes.push('TCP ports accept individual numbers and ranges, for example 22,443,8000-8010. Separate entries with commas, spaces or new lines. You can edit the port list directly.');
    if (tool === 'discover') notes.push(discoveryPortPresetDescription(discoveryPortPresetSelection()));
    for (const item of [...config.primary, ...config.advanced]) if (item.desc) notes.push(item.name + ': ' + item.desc);
    if (NetworkToolkitSource.supports(tool)) notes.push('Source interface is shared across diagnostic tools. Automatic uses the system routing table. An explicit source binds supported probes and DNS queries to that adapter and address. Changes made during a run apply to the next run.');
    if (notes.some(Boolean)) html += section(tool + '.help', 'Help and formats', notes.filter(Boolean).map(note => '<p>' + esc(note) + '</p>').join(''), '', 'info');
    return html;
  }
  function setText(element, value) { if (element && element.textContent !== value) element.textContent = value; }
  function syncTarget() {
    const container = document.getElementById('availableNetworks');
    if (!container) return;
    const enabled = networkMode() && needsTarget(), key = activeTool() + '.networks';
    container.hidden = !enabled;
    if (container.dataset.configSection !== key) {
      container.dataset.configSection = key;
      container.open = sectionOpen(key);
    }
    selectNetworkTab(state.configurationNetworkTab || 'local', false);
    update();
  }
  function selectNetworkTab(tab, persist = true, focus = false) {
    if (!networkTabs.includes(tab)) return;
    state.configurationNetworkTab = tab;
    for (const button of document.querySelectorAll('[data-config-network-tab]')) {
      const selected = button.dataset.configNetworkTab === tab;
      button.setAttribute('aria-selected', String(selected)); button.tabIndex = selected ? 0 : -1;
      const panel = document.getElementById(button.getAttribute('aria-controls'));
      if (panel) panel.hidden = !selected;
      if (selected && focus) button.focus({preventScroll: true});
    }
    if (persist) saveState();
  }
  function detailsFor(tool) {
    const opts = toolOptions(tool), network = networkMode(tool), parts = [];
    let warning = NetworkToolkitSource.validation(tool) || '', critical = false;
    if (network) {
      const plan = scanPlan(tool);
      if (plan.ok) {
        parts.push(fmt(plan.hosts, 0) + (plan.hosts === 1 ? ' host' : ' hosts'));
        if (plan.ports.length) parts.push(plan.ports.length + ' TCP ports');
        parts.push(fmt(plan.requestedProbes, 0) + ' checks');
      } else warning = [warning, plan.error].filter(Boolean).join(' ');
      if (tool === 'discover') parts.push('Names ' + (opts.resolveHostnames ? 'on' : 'off'), 'Model ' + (opts.identify ? 'on' : 'off'));
      else if (tool === 'tcp') parts.push('Model ' + (opts.identify ? 'on' : 'off'));
    } else if (tool === 'ping') parts.push(Number(opts.count)===0?'Until stopped':opts.count + ' packets', opts.intervalMs + ' ms interval', opts.timeoutMs + ' ms timeout');
    else if (tool === 'trace') parts.push(opts.maxHops + ' hops', opts.probes + (Number(opts.probes) === 1 ? ' probe/hop' : ' probes/hop'), 'Hostnames ' + (opts.resolveHostnames ? 'on' : 'off'));
    else if (['quick', 'tcp'].includes(tool)) {
      try { const ports = scanPorts(opts.ports, 1024); parts.push(ports.length + ' TCP ports', tool === 'quick' ? 'ICMP + DNS' : opts.timeoutMs + ' ms timeout'); }
      catch (error) { warning = [warning, error.message].filter(Boolean).join(' '); }
    } else if (tool === 'dns') parts.push(opts.recordType + ' records', opts.server ? 'DNS ' + opts.server : state.diagnosticSource ? 'Adapter DNS' : 'System DNS');
    else if (tool === 'http') {
      parts.push(opts.method, opts.timeoutMs + ' ms timeout', 'Redirects ' + (opts.redirects ? 'on' : 'off'));
      if (opts.verifyTLS === false) { parts.push('TLS verification OFF'); critical = true; }
    } else if (tool === 'tls') parts.push('Port ' + opts.port, opts.serverName ? 'SNI: ' + opts.serverName : 'SNI: target name', opts.timeoutMs + ' ms timeout');
    else if (tool === 'identify') parts.push('Hostname + MAC/vendor + UPnP', opts.timeoutMs + ' ms timeout');
    else if (tool === 'monitor') parts.push(opts.mode === 'tcp' ? 'TCP ' + opts.port : 'ICMP', Number(opts.count)===0?'Until stopped':opts.count + ' samples', opts.intervalMs + ' ms interval');
    else if (tool === 'wol') parts.push('Broadcast ' + opts.broadcast, 'UDP ' + opts.port);
    return {text: parts.join(' | '), warning, critical};
  }
  function update() {
    if (typeof state === 'undefined') return;
    const tool = activeTool(), host = document.getElementById('configurationSummary');
    if (host) {
      const visible = splitToolPage() && needsTarget();
      host.hidden = !visible;
      if (visible) {
        const info = detailsFor(tool);
        const main = host.querySelector('[data-config-overview]'), warning = host.querySelector('[data-config-warning]');
        setText(main, info.text); main?.classList.toggle('warn', info.critical);
        setText(warning, info.warning); if (warning) warning.hidden = !info.warning;
        host.dataset.invalid = String(!!info.warning);
      }
    }
    if (TOOLS[tool]) {
      const grouped = fieldsFor(tool, {...TOOLS[tool], ...toolFields(tool)});
      for (const el of document.querySelectorAll('#toolOptions [data-config-section]')) {
        if (el.dataset.configSection.endsWith('.help')) continue;
        const summary = grouped.secondary.map(item => fieldSummary(tool, item)).join(' | ');
        setText(el.querySelector('[data-config-section-summary]'), summary);
        const changed = grouped.secondary.some(item => String(valueFor(tool, item)) !== String(item.value));
        const badge = el.querySelector('.configuration-modified'); if (badge) badge.hidden = !changed;
        el.classList.toggle('configuration-critical', tool === 'http' && state.forms.http.verifyTLS === false);
      }
    }
    const selector = document.getElementById('tcpPortPreset');
    if (selector) selector.value = tcpPreset();
    const resolver = document.getElementById('opt-dns-server');
    if (resolver) {
      resolver.placeholder = state.diagnosticSource ? 'Selected adapter DNS' : 'System resolver';
      setText(document.querySelector('#toolOptions [data-dns-server=""]'), state.diagnosticSource ? 'Adapter DNS' : 'System DNS');
    }
    const networks = document.getElementById('availableNetworks');
    if (networks && !networks.hidden) {
      const local = localIPv4Choices().length, route = root.NetworkToolkitScanRoutes?.info?.() || {}, scopes = NetworkToolkitScanScopes.tokens(state.networkRange).length;
      const summary = local + ' local | ' + (route.loading ? 'reading routes' : (route.available || 0) + ' routed') + ' | ' + scopes + (scopes === 1 ? ' scope' : ' scopes');
      setText(networks.querySelector('[data-config-section-summary]'), summary);
      setText(document.querySelector('[data-config-network-count="local"]'), String(local));
      setText(document.querySelector('[data-config-network-count="routed"]'), route.loading ? '...' : String(route.available || 0));
      setText(document.querySelector('[data-config-network-count="selected"]'), String(scopes));
      networks.classList.toggle('configuration-critical', !!route.error);
      networks.title = route.error || 'Choose local or routed networks, or review selected scopes and supported formats.';
    }
    const all = document.getElementById('configurationExpandAll');
    if (all) {
      const sections = [...document.querySelectorAll('#toolConfiguration details[data-config-section]')].filter(el => !el.hidden);
      all.hidden = sections.length < 2;
      const expand = sections.some(el => !el.open);
      setText(all, expand ? 'Expand options' : 'Collapse options'); all.dataset.expand = String(expand);
    }
  }
  function schedule() { if (!queued) { queued = true; requestAnimationFrame(() => { queued = false; update(); }); } }
  function revealField(element) {
    if (!element) return;
    for (let parent = element.parentElement; parent; parent = parent.parentElement) if (parent.matches('details')) parent.open = true;
    element.focus(); element.scrollIntoView({block: 'nearest', inline: 'nearest'});
  }
  // The control follows the panel it controls. A narrow rail remains in the
  // grid while collapsed so both the button and the divider can reopen it.
  function syncSidebarControl() {
    const workspace = document.getElementById('toolWorkspace'), heading = document.getElementById('toolConfigHeading'), toggle = document.getElementById('toolConfigurationToggle'), divider = document.getElementById('toolDivider');
    if (!workspace || !heading || !toggle || !divider) return;
    let rail = document.getElementById('toolConfigurationRail');
    if (!rail) { rail = document.createElement('div'); rail.id = 'toolConfigurationRail'; rail.setAttribute('aria-label', 'Configuration sidebar controls'); workspace.append(rail); }
    const collapsed = workspace.classList.contains('configuration-hidden'), owner = collapsed ? rail : heading;
    if (toggle.parentElement !== owner || !collapsed && owner.firstElementChild !== toggle) {if(collapsed)owner.append(toggle);else heading.firstElementChild.before(toggle);}
    root.NetworkToolkitPanelControls.set(toggle, {name: 'configuration', edge: 'left', expanded: !collapsed});
    root.NetworkToolkitPanelControls.bindReveal(rail, {collapsed: () => workspace.classList.contains('configuration-hidden'), button: () => toggle});
    divider.setAttribute('aria-label', collapsed ? 'Show configuration sidebar' : 'Resize or hide configuration sidebar');
    divider.setAttribute('aria-controls', 'toolConfiguration');
    divider.title = collapsed ? 'Double-click or press Enter or Space to show configuration.' : 'Drag to resize. Double-click or press Enter or Space to hide configuration. Arrow keys resize; Home resets.';
    if (collapsed) { divider.setAttribute('aria-valuenow', '0'); divider.setAttribute('aria-valuemin', '0'); divider.setAttribute('aria-valuetext', 'Collapsed'); }
  }
  function toggleSidebar() {
    if (!splitToolPage()) return;
    if (compactToolWorkspace()) setToolPanel(document.getElementById('toolWorkspace').classList.contains('configuration-hidden') ? 'configuration' : 'results');
    else { state.toolLayout.collapsed = !state.toolLayout.collapsed; applyToolLayout(); saveState(); }
    syncSidebarControl();
  }
  function initSidebarControl() {
    syncSidebarControl();
    const workspace = document.getElementById('toolWorkspace');
    if (workspace && typeof MutationObserver === 'function') {
      const observer = new MutationObserver(syncSidebarControl);
      observer.observe(workspace, {attributes: true, attributeFilter: ['class', 'style']});
      // Existing layout updates also refresh the button label; keep its
      // shorter panel-heading label after those updates settle.
      observer.observe(document.getElementById('toolConfigurationToggle'), {childList: true});
    }
    document.addEventListener('click', event => {
      const toggle = event.target.closest('#toolConfigurationToggle');
      if (!toggle) return;
      event.preventDefault(); event.stopImmediatePropagation(); closeEntryContextMenu(false); toggleSidebar(); toggle.focus({preventScroll: true});
    }, true);
    document.addEventListener('dblclick', event => {
      if (event.target.id !== 'toolDivider') return;
      event.preventDefault(); event.stopImmediatePropagation(); toggleSidebar();
    }, true);
    document.addEventListener('pointerdown', event => {
      // A click on the collapsed edge must not resize from a hidden panel's
      // zero-sized bounding box and overwrite the last useful width.
      if (event.target.id !== 'toolDivider' || !state.toolLayout.collapsed) return;
      event.preventDefault(); event.stopImmediatePropagation();
    }, true);
    document.addEventListener('keydown', event => {
      if (event.target.id !== 'toolDivider') return;
      if (['Enter', ' '].includes(event.key)) { event.preventDefault(); event.stopImmediatePropagation(); toggleSidebar(); }
      else if (state.toolLayout.collapsed && ['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) { state.toolLayout.collapsed = false; applyToolLayout(); syncSidebarControl(); }
    }, true);
  }
  function init() {
    if (initialized || typeof document === 'undefined') return; initialized = true;
    initSidebarControl();
    document.addEventListener('toggle', event => {
      const el = event.target;
      if (!el.matches?.('[data-config-section]') || !el.isConnected) return;
      const key = el.dataset.configSection;
      if (!key || !/^[a-z][a-z0-9-]{0,40}\.[a-z][a-z0-9-]{0,40}$/.test(key)) return;
      if (sectionOpen(key, el.dataset.configDefault === 'true') !== el.open) {
        state.configurationSections ||= {}; state.configurationSections[key] = el.open; saveState();
      }
      schedule();
    }, true);
    document.addEventListener('click', event => {
      const summary = event.target.closest('summary'), details = summary?.parentElement;
      if (details?.matches('details[data-config-section]') && !event.defaultPrevented && !event.target.closest('a,button,input,select,textarea')) {
        // Native toggle events are queued. Remember a click before navigation
        // can remove its details element and discard that queued event.
        event.preventDefault();
        const open = !details.open;
        state.configurationSections ||= {};
        state.configurationSections[details.dataset.configSection] = open;
        details.open = open;
        saveState(); update(); return;
      }
      const tab = event.target.closest('[data-config-network-tab]');
      if (tab) { selectNetworkTab(tab.dataset.configNetworkTab); return; }
      const all = event.target.closest('#configurationExpandAll');
      if (all) {
        const open = all.dataset.expand === 'true'; state.configurationSections ||= {};
        for (const el of document.querySelectorAll('#toolConfiguration details[data-config-section]')) {
          if (el.hidden) continue;
          state.configurationSections[el.dataset.configSection] = open; el.open = open;
        }
        saveState(); update(); return;
      }
      if (event.target.closest('[data-dns-server]')) schedule();
    });
    document.addEventListener('keydown', event => {
      const tab = event.target.closest?.('[data-config-network-tab]');
      if (!tab || !['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
      event.preventDefault();
      const index = networkTabs.indexOf(tab.dataset.configNetworkTab);
      selectNetworkTab(networkTabs[event.key === 'Home' ? 0 : event.key === 'End' ? 2 : (index + (event.key === 'ArrowRight' ? 1 : 2)) % 3], true, true);
    });
    document.addEventListener('change', event => { if (event.target.id === 'tcpPortPreset') selectTCPPreset(event.target.value); });
    document.addEventListener('input', event => {
      if (event.target.id === 'opt-tcp-ports') {
        const form = state.forms.tcp;
        form.portPreset = tcpPresets.find(item => signature(item[2]) === signature(form.ports))?.[0] || 'custom';
        if (form.portPreset === 'custom') form.customPorts = String(form.ports ?? '');
      }
      if (event.target.closest('#toolConfiguration')) schedule();
    });
  }
  root.NetworkToolkitConfiguration = Object.freeze({init, mergePreferences, normalizeSections, section, optionsHTML, syncTarget, update, revealField, detailsFor});
})(globalThis);
