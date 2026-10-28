(function (root) {
  'use strict';
  let initialized = false, shortWindow = null;

  // Reuse each existing control; no duplicate handlers, state or capabilities.
  function navigationHost() {
    if (document.body.classList.contains('terminal-window') || document.body.classList.contains('editor-window')) return null;
    if (document.body.classList.contains('terminal-page')) return document.querySelector('#terminalCommandBar .terminal-command-start');
    const command = document.getElementById('toolCommandBar');
    if (document.body.classList.contains('diagnostic-page') && command && !command.classList.contains('hidden')) return command;
    if (shortWindow?.matches && document.body.classList.contains('diagnostic-page')) return document.getElementById('appNavigationFooterSlot');
    return document.getElementById('appNavigationSlot');
  }

  function sync() {
    const host = document.getElementById('hostTop'), brand = document.querySelector('#sidebar .brand-mark');
    if (host) {
      const name = host.textContent.trim() || 'Local diagnostics';
      host.title = name;
      host.setAttribute('aria-label', 'System: ' + name);
      if (brand) {
        brand.title = 'Network Toolkit — ' + name;
        brand.setAttribute('aria-label', 'Network Toolkit. System: ' + name);
        brand.setAttribute('role', 'img');
        brand.tabIndex = document.body.classList.contains('app-nav-collapsed') ? 0 : -1;
      }
    }
    const count = document.getElementById('runningCount');
    if (count) {
      const text = count.textContent.trim();
      count.dataset.count = /^\d+/.exec(text)?.[0] || '0';
      count.title = text + ' diagnostic jobs';
      count.setAttribute('aria-label', text + ' diagnostic jobs');
      count.tabIndex = count.hidden ? -1 : 0;
    }
  }

  function init() {
    if (!initialized) {
      initialized = true;
      shortWindow = root.matchMedia?.('(max-height:600px)');
      shortWindow?.addEventListener('change', () => root.NetworkToolkitNavigation?.sync());
    }
    document.body.classList.add('app-chrome-ready'); sync();
  }
  root.NetworkToolkitAppChrome = Object.freeze({init, sync, navigationHost});
})(globalThis);
