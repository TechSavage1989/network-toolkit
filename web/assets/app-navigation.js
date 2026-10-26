(function (root) {
  'use strict';

  let initialized = false, compactQuery = null, sidebar = null, toggleButton = null, toggleAnchor = null, brandHeader = null, backdrop = null, drawerClose = null, main = null;
  let lastRailClick = null;
  let previousDrawerOpen = false, layoutMode = '', layoutPending = false, ownsMainInert = false, previousMainInert = false;

  function terminalOnly() { return typeof TERMINAL_ONLY !== 'undefined' && TERMINAL_ONLY; }
  function compact() { return compactQuery?.matches === true; }
  function drawerOpen() { return compact() && sidebar?.classList.contains('open'); }

  function layoutChanged() {
    if (layoutPending) return;
    layoutPending = true;
    requestAnimationFrame(() => {
      layoutPending = false;
      if (typeof applyToolLayout === 'function') applyToolLayout();
      root.NetworkToolkitWorkspace?.layoutDetails();
      if (typeof scheduleTerminalFit === 'function') scheduleTerminalFit();
    });
  }

  function sync({restoreFocus = true} = {}) {
    if (!initialized || !sidebar || terminalOnly()) return;
    const small = compact();
    if (small && toggleButton.parentElement !== toggleAnchor.parentElement) toggleAnchor.before(toggleButton);
    const leading = small ? drawerClose : toggleButton;
    if (brandHeader.firstElementChild !== leading) brandHeader.firstElementChild.before(leading);
    if (!small && sidebar.classList.contains('open')) sidebar.classList.remove('open');
    const open = drawerOpen(), collapsed = !small && state.navigationCollapsed === true;
    const divider=document.getElementById('appNavigationDivider');if(divider){divider.setAttribute('aria-valuemin','0');divider.setAttribute('aria-valuemax','1');divider.setAttribute('aria-valuenow',collapsed?'0':'1');divider.setAttribute('aria-valuetext',collapsed?'Collapsed':'Expanded');}
    const returnFocus = restoreFocus && previousDrawerOpen && !open && sidebar.contains(document.activeElement);
    document.body.classList.toggle('app-nav-collapsed', collapsed);
    document.body.classList.toggle('app-nav-drawer-open', open);
    sidebar.inert = small && !open;
    sidebar.setAttribute('aria-hidden', String(small && !open));
    backdrop.hidden = !open;
    if (open && !ownsMainInert) { previousMainInert = main.inert; main.inert = true; ownsMainInert = true; }
    else if (!open && ownsMainInert) { main.inert = previousMainInert; ownsMainInert = false; }
    root.NetworkToolkitPanelControls.set(toggleButton, {name: 'navigation', edge: 'left', expanded: small ? open : !collapsed});
    root.NetworkToolkitPanelControls.set(drawerClose, {name: 'navigation', edge: 'left', expanded: true});
    // The collapsed rail stays usable for navigation; revealing the panel is an
    // explicit arrow, divider or double-click action. Single clicks navigate.
    root.NetworkToolkitPanelControls.bindReveal(sidebar, {collapsed: () => !compact() && state.navigationCollapsed === true, button: () => toggleButton, clickToReveal:false});
    for (const button of sidebar.querySelectorAll('.nav-item[data-page]')) {
      const text = [...button.children].find(element => element.tagName === 'SPAN' && !element.classList.contains('badge'));
      text?.classList.add('nav-item-text');
      const name = text?.textContent.trim() || titleFor(button.dataset.page), badge = button.querySelector('.badge')?.textContent.trim();
      button.setAttribute('aria-label', name + (badge ? ', ' + badge + ' diagnostics' : ''));
      button.title = name + (badge ? ' (' + badge + ')' : '') + (small ? '' : '. Double-click to ' + (collapsed ? 'expand' : 'collapse') + ' navigation.');
    }
    const status = sidebar.querySelector('#engineLabel'), version = sidebar.querySelector('#engineSub');
    if (status) {
      const description = [status.textContent.trim(),version?.textContent.trim()].filter(Boolean).join('. ');
      status.title = description;
      status.setAttribute('role', 'img');
      status.setAttribute('aria-label', description);
      status.tabIndex = collapsed ? 0 : -1;
    }
    previousDrawerOpen = open;
    const nextMode = small ? 'drawer' : collapsed ? 'rail' : 'full';
    if (nextMode !== layoutMode) { layoutMode = nextMode; layoutChanged(); }
    if (returnFocus) toggleButton.focus({preventScroll:true});
  }

  function closeDrawer(restoreFocus = true) {
    if (!sidebar || terminalOnly()) return;
    const wasOpen = drawerOpen();
    sidebar.classList.remove('open');
    sync({restoreFocus:false});
    if (wasOpen && restoreFocus) toggleButton.focus({preventScroll:true});
  }

  function toggle() {
    if (!initialized || !sidebar || terminalOnly()) return;
    lastRailClick = null;
    closeEntryContextMenu(false);
    if (compact()) {
      if (drawerOpen()) { closeDrawer(); return; }
      sidebar.classList.add('open');
      sync();
      const selected = sidebar.querySelector('.nav-item[aria-current="page"]') || sidebar.querySelector('.nav-item') || drawerClose;
      selected.focus({preventScroll:true});
      selected.scrollIntoView({block:'nearest',inline:'nearest'});
      return;
    }
    state.navigationCollapsed = state.navigationCollapsed !== true;
    sync();
    saveState();
  }

  function init() {
    if (initialized) return;
    initialized = true;
    if (terminalOnly()) return;
    sidebar = document.getElementById('sidebar');
    toggleButton = document.getElementById('mobileMenu');
    main = document.querySelector('.main');
    if (!sidebar || !toggleButton || !main) return;
    const divider=document.createElement('div');divider.id='appNavigationDivider';divider.className='app-navigation-divider';divider.setAttribute('role','separator');divider.setAttribute('aria-orientation','vertical');divider.setAttribute('aria-label','Expand or collapse tool navigation');divider.tabIndex=0;divider.title='Double-click or press Enter or Space to expand or collapse tool navigation.';sidebar.append(divider);
    divider.addEventListener('dblclick',event=>{if(compact())return;event.preventDefault();event.stopPropagation();toggle();});
    sidebar.addEventListener('dblclick',event=>{if(compact()||state.navigationCollapsed!==true||!event.target.closest?.('.brand-mark,#engineLabel'))return;event.preventDefault();event.stopPropagation();toggle();});
    divider.addEventListener('keydown',event=>{if(compact()||!['Enter',' '].includes(event.key))return;event.preventDefault();event.stopPropagation();toggle();});
    document.body.classList.add('app-navigation-ready');
    toggleButton.type = 'button';
    toggleButton.setAttribute('aria-controls', 'sidebar');
    toggleAnchor = document.createElement('span');
    toggleAnchor.hidden = true;
    toggleAnchor.setAttribute('aria-hidden', 'true');
    toggleButton.before(toggleAnchor);
    brandHeader = sidebar.querySelector('.brand');
    const brand = sidebar.querySelector('.brand>div:not(.brand-mark)');
    brand?.classList.add('nav-brand-copy');
    sidebar.querySelector('.brand')?.setAttribute('title', 'Network Toolkit');
    drawerClose = document.createElement('button');
    drawerClose.id = 'sidebarDrawerClose';
    drawerClose.type = 'button';
    drawerClose.className = 'iconbtn sidebar-drawer-close';
    root.NetworkToolkitPanelControls.set(drawerClose, {name: 'navigation', edge: 'left', expanded: true});
    drawerClose.addEventListener('click', () => closeDrawer());
    brandHeader.firstElementChild.before(drawerClose);
    backdrop = document.createElement('button');
    backdrop.id = 'appNavigationBackdrop';
    backdrop.type = 'button';
    backdrop.className = 'app-navigation-backdrop';
    backdrop.hidden = true;
    backdrop.tabIndex = -1;
    backdrop.setAttribute('aria-hidden', 'true');
    backdrop.addEventListener('click', () => closeDrawer());
    document.body.append(backdrop);
    compactQuery = matchMedia('(max-width:1100px)');
    compactQuery.addEventListener('change', () => {
      lastRailClick = null;
      const focused = document.activeElement, returnFocus = sidebar.contains(focused) && (compact() || focused === drawerClose);
      closeDrawer(false);
      if (returnFocus) toggleButton.focus({preventScroll:true});
    });
    new MutationObserver(() => sync()).observe(sidebar, {attributes:true,attributeFilter:['class']});
    document.addEventListener('keydown', event => {
      if (!drawerOpen() || document.querySelector('.modal')) return;
      if (event.key === 'Escape') { event.preventDefault(); event.stopImmediatePropagation(); closeDrawer(); return; }
      if (event.key !== 'Tab') return;
      const controls = [...sidebar.querySelectorAll('button:not(:disabled),a[href],input:not(:disabled),select:not(:disabled),[tabindex="0"]')].filter(element => element.getClientRects().length);
      const first = controls[0], last = controls.at(-1);
      if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last?.focus(); }
      else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first?.focus(); }
    }, true);
    document.addEventListener('click', event => {
      const button = event.target.closest?.('#sidebar .nav-item[data-page]');
      if (!button || event.button !== 0 || button.disabled) { lastRailClick = null; return; }
      const small = compact(), page = button.dataset.page;
      // Navigation redraws its buttons after the first click; recognize the
      // second click by page identity rather than relying on a surviving node.
      // Handle tool-tab pairs here in both panel states. A native dblclick
      // following the second click must not toggle the sidebar a second time.
      if(!small){const now=Date.now();if(lastRailClick?.page===page&&now-lastRailClick.at<500){lastRailClick=null;toggle();}else lastRailClick={page,at:now};}else lastRailClick=null;
      if (small) closeDrawer(false);
      requestAnimationFrame(() => {
        if (state.activePage !== page) return;
        const target = small ? document.getElementById('workspaceScroll') : [...sidebar.querySelectorAll('.nav-item[data-page]')].find(item => item.dataset.page === page);
        target?.focus({preventScroll:true});
      });
    }, true);
    sync();
  }

  root.NetworkToolkitNavigation = Object.freeze({init,sync,toggle,closeDrawer});
})(globalThis);
