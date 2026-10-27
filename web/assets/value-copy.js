/* Copy individual result values without changing text selection or row actions. */
(function (root) {
  'use strict';
  const selector = 'td,.data-item .val,.result-record-grid dd,.device-data-grid dd,.adapter-detail .value,.stat .value,.overview-info-card .overview-card-value,.calc-value,.packet-detail .value,.packet-details .value,.device-details .value,.result-detail .value,.detail-value,.kv-value,.wifi-detail .value,.data-grid .value,[data-copy-value]';
  const nonDataLabel = /^(?:tools?|actions?|result|status|state|type|record|hop|asc|sort|controls?|options?|elapsed|duration|count|records?|rtt)$/i;
  const informationalValue = /^(?:timeout|ttl_expired|destination|completed|failed|success|error|warning|info|pending|running|stopped|not applicable|lookup timed out)$/i;
  const excluded = 'button,a,summary,[role="button"],.rb-app,.fm-explorer,.transfer-file-scroller,.transfer-log-panel,.nt-editor-page,.nt-editor-overlay,.session-library,.terminal-sidebar-sftp,.xterm,.terminal-workspace,.terminal-pane,.terminal-view,.terminal-output,.public-ip-value,[data-terminal-secret],[data-wifi-secret],#terminalWorkspace,#terminalPanes,#terminalTabs,[data-terminal-pane],input,textarea,select,[contenteditable="true"],[data-no-value-copy],[data-secret],[data-sensitive],[data-value-copy-control]';
  const interactive = 'button,a,input,textarea,select,summary,[contenteditable="true"],[role="button"],[role="checkbox"],[role="switch"]';
  const sensitive = /password|passphrase|private[\s_-]*key|secret|credential|(?:access|refresh|api|auth|session)[\s_-]*token|api[\s_-]*key|^key$/i;
  const placeholder = /^(?:[-\u2013\u2014]|true|false|yes|no|n\/a|null|undefined|not reported|not measured|not available|not loaded|not checked|unavailable|loading|checking|no data|unknown|none)$/i;
  const copyIcon = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true"><rect x="8" y="8" width="12" height="12" rx="2"/><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"/></svg>';
  let initialized = false, observer, scheduled = false, contextSequence = 0;
  const pending = new Set(), contexts = new Map();

  function labelFor(element) {
    if (element.hasAttribute('data-copy-label')) return element.getAttribute('data-copy-label').trim() || 'Value';
    if (element.tagName === 'TD') {
      const row = element.parentElement, table = element.closest('table'), cells = [...(row?.children || [])].filter(cell => ['TD','TH'].includes(cell.tagName));
      const headers = table?.querySelectorAll('thead th') || [], header = headers[cells.indexOf(element)];
      return (header?.getAttribute('data-column-name') || header?.textContent || '').trim().replace(/\s+/g,' ') || 'Value';
    }
    const group = element.closest('.data-item') || element.closest('.overview-info-card') || element.parentElement;
    return (group?.querySelector('.key,dt,.label,.overview-card-title,.section-eyebrow')?.textContent || '').trim().replace(/\s+/g,' ') || 'Value';
  }

  function copyCompatibleControl(control, element) {
    if (control.matches('.network-address-details[data-select-device]')) return true;
    // Discovery's explicit numeric aggregate still needs its separate copy
    // action now that each port is a connection button. Do not enable copying
    // arbitrary controls, their labels, editable fields or credential groups.
    if (!element.matches('.network-open-ports[data-copy-value]') || element.getAttribute('data-copy-label') !== 'Open TCP ports' || !control.matches('button.network-port-connect[data-network-port][data-network-port-key]')) return false;
    const ports = element.getAttribute('data-copy-value').split(', ');
    return ports.length > 0 && ports.every(port => /^[1-9]\d{0,4}$/.test(port) && Number(port) <= 65535);
  }

  function captureContext(target) {
    const element = target?.closest?.(selector);
    if (!element || element.closest(excluded) || element.querySelector('input,textarea,select,[contenteditable="true"],[data-secret],[data-sensitive],[data-terminal-secret],[data-wifi-secret]')) return null;
    if (element.tagName === 'TD' && (Number(element.getAttribute('colspan') || 1) > 1 || element.classList.contains('list-empty'))) return null;
    // A group of individual values must not also get an aggregate copy action.
    if (element.querySelector(selector)) return null;
    const controls = [...element.querySelectorAll(interactive)].filter(control => !control.hasAttribute('data-value-copy-control'));
    if (controls.some(control => !copyCompatibleControl(control, element))) return null;
    const label = labelFor(element);
    if (sensitive.test(label) || sensitive.test(element.getAttribute('data-copy-field') || '') || nonDataLabel.test(label)) return null;
    if (element.closest('[data-tools-menu],.tools-cell,.status-pill,.badge,.pill')) return null;
    if (controls.some(control => !copyCompatibleControl(control, element))) return null;
    const address = element.querySelector('.network-address-details .mono'), record = element.querySelector('.record-value'), explicit = element.getAttribute('data-copy-value');
    const value = String(explicit !== null ? explicit : address?.textContent ?? record?.textContent ?? element.textContent ?? ''), eligible = value.trim();
    if (!eligible || placeholder.test(eligible) || informationalValue.test(eligible) || !/[\p{L}\p{N}]/u.test(eligible)) return null;
    return {element,value,label};
  }

  function controlsFor(element) { return [...element.children].filter(child => child.hasAttribute?.('data-value-copy-control')); }
  function prepare(container = document) {
    const values = container.matches?.(selector) ? [container,...container.querySelectorAll(selector)] : [...container.querySelectorAll(selector)];
    for (const element of values) {
      const value = captureContext(element), controls = controlsFor(element);
      if (!value) { for (const control of controls) control.remove(); element.classList.remove('value-copy-cell'); continue; }
      element.classList.add('value-copy-cell');
      let button = controls.shift(); for (const duplicate of controls) duplicate.remove();
      if (!button) { button = document.createElement('button'); button.type = 'button'; button.className = 'value-copy-control'; button.setAttribute('data-value-copy-control',''); button.innerHTML = copyIcon; element.append(button); }
      const caption = value.label === 'Value' ? 'Copy value' : 'Copy ' + value.label + ' value';
      button.setAttribute('aria-label',caption); button.title = caption;
    }
    return container;
  }

  function copy(value) {
    if (!value) return;
    if (typeof root.copyText === 'function') return root.copyText(value);
    if (typeof root.NetworkToolkitTerminalInteraction?.copyValue === 'function') return root.NetworkToolkitTerminalInteraction.copyValue(value);
  }

  function contextActionHTML(target, options = {}) {
    const value = captureContext(target); if (!value) return '';
    const token = String(++contextSequence); contexts.set(token,{...value,close:options.close});
    if (contexts.size > 32) contexts.delete(contexts.keys().next().value);
    return '<button type="button" role="menuitem" data-value-copy-context="' + token + '">Copy value</button>';
  }

  function enqueue(container) {
    if (!container?.querySelectorAll) container = container?.parentElement;
    if (!container || container.closest?.(excluded)) return; pending.add(container);
    if (scheduled) return; scheduled = true;
    const run = () => { scheduled = false; const scopes = [...pending]; pending.clear(); for (const scope of scopes) if (scope.isConnected || scope === document) prepare(scope); };
    if (typeof root.requestAnimationFrame === 'function') root.requestAnimationFrame(run); else root.setTimeout(run,0);
  }

  function init() {
    if (initialized) return; initialized = true; prepare(document);
    document.addEventListener('pointerdown', event => {
      if (event.button === 0 && event.target.closest?.('[data-value-copy-control]')) { event.preventDefault(); event.stopImmediatePropagation(); }
    }, true);
    document.addEventListener('click', event => {
      const button = event.target.closest?.('[data-value-copy-control],[data-value-copy-context]'); if (!button) return;
      event.preventDefault(); event.stopImmediatePropagation();
      if (button.hasAttribute('data-value-copy-context')) {
        const value = contexts.get(button.getAttribute('data-value-copy-context')); if (!value) return;
        contexts.delete(button.getAttribute('data-value-copy-context')); value.close?.();
        controlsFor(value.element)[0]?.focus?.({preventScroll:true}); void copy(value.value);
      } else { const value = captureContext(button.parentElement); if (value) void copy(value.value); }
    }, true);
    document.addEventListener('keydown', event => {
      const button = event.target.closest?.('[data-value-copy-control]'); if (!button) return;
      if (['Enter',' '].includes(event.key)) event.stopImmediatePropagation();
    }, true);
    if (typeof root.MutationObserver === 'function') {
      observer = new root.MutationObserver(records => {
        for (const record of records) {
          if (record.target.closest?.(excluded) || record.target.parentElement?.closest?.(excluded)) continue;
          if (record.type === 'characterData') { enqueue(record.target.parentElement); continue; }
          let changed = !!record.removedNodes?.length;
          for (const node of record.addedNodes || []) { if (node.nodeType === 1 && !node.hasAttribute('data-value-copy-control')) { enqueue(node); changed = true; } else if (node.nodeType === 3) changed = true; }
          if (changed) enqueue(record.target);
        }
      }); observer.observe(document.body,{childList:true,subtree:true,characterData:true});
    }
  }
  root.NetworkToolkitValueCopy = Object.freeze({init,prepare,captureContext,contextActionHTML});
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded',init,{once:true}); else init();
})(globalThis);
