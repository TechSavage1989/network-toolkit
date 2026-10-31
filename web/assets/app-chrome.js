(function (root) {
  'use strict';
  let initialized = false, shortWindow = null;

  function visibleHost(element) {
    if (!element || element.classList.contains('hidden')) return false;
    const style = root.getComputedStyle(element), box = element.getBoundingClientRect();
    return style.display !== 'none' && style.visibility !== 'hidden' && box.width > 0 && box.height > 0;
  }

  // Reuse each existing control; no duplicate handlers, state or capabilities.
  const attribute=(node,name,value)=>{if(node.getAttribute(name)!==value)node.setAttribute(name,value);};
  const title=(node,value)=>{if(node.title!==value)node.title=value;};
  const tabIndex=(node,value)=>{if(node.tabIndex!==value)node.tabIndex=value;};
  function navigationHost() {
    if (document.body.classList.contains('terminal-window') || document.body.classList.contains('editor-window')) return null;
    if (document.body.classList.contains('terminal-page')) return document.querySelector('#terminalCommandBar .terminal-command-start');
    const command = document.getElementById('toolCommandBar');
    // Transfer server/client layouts suppress this shared bar in CSS even when
    // split-tool configuration leaves its class unchanged. Never park the only
    // navigation opener inside a non-rendered host.
    if (document.body.classList.contains('diagnostic-page') && visibleHost(command)) return command;
    if (shortWindow?.matches && document.body.classList.contains('diagnostic-page')) return document.getElementById('appNavigationFooterSlot');
    return document.getElementById('appNavigationSlot');
  }

  function sync() {
    const host = document.getElementById('hostTop'), brand = document.querySelector('#sidebar .brand-mark');
    if (host) {
      const name = host.textContent.trim() || 'Local diagnostics';
      title(host,name);
      attribute(host,'aria-label', 'System: ' + name);
      if (brand) {
        title(brand,'Network Toolkit — ' + name);
        attribute(brand,'aria-label', 'Network Toolkit. System: ' + name);
        attribute(brand,'role', 'img');
        tabIndex(brand,document.body.classList.contains('app-nav-collapsed') ? 0 : -1);
      }
    }
    const count = document.getElementById('runningCount');
    if (count) {
      const text = count.textContent.trim();
      const total=/^\d+/.exec(text)?.[0] || '0';
      if(count.dataset.count!==total)count.dataset.count=total;
      title(count,text + ' diagnostic jobs');
      attribute(count,'aria-label',text + ' diagnostic jobs');
      tabIndex(count,count.hidden ? -1 : 0);
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
