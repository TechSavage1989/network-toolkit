/* Test-only observer/control channel injected by terminal_sftp123_windows_test.go.
   File gestures are exclusively real Win32 input, never DOM drag events. */
(async function () {
  'use strict';
  const errors = [], sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
  addEventListener('error', e => errors.push(String(e.message)));
  addEventListener('unhandledrejection', e => errors.push(String(e.reason?.message || e.reason)));
  const gestures = {trustedDown: 0, trustedMoves: 0, htmlDragStarts: 0};
  for (const name of ['pointerdown', 'pointermove', 'dragstart']) document.addEventListener(name, e => {
    if (e.isTrusted && e.target.closest?.('[data-tsftp-item]')) gestures[name === 'pointerdown' ? 'trustedDown' : name === 'pointermove' ? 'trustedMoves' : 'htmlDragStarts']++;
  }, true);
  async function wait(fn, label) {
    const end = Date.now() + 25000;
    while (Date.now() < end) { const value = fn(); if (value) return value; await sleep(30); }
    throw Error('Terminal SFTP123 timeout: ' + label);
  }
  const current = () => currentTerminal();
  const editable = e => !!(e?.xterm && terminalOwnerCanMutate(e) && !e.transferBusy && !e.restoring && !e.awaitingAcceptance && !e.xterm.options.disableStdin && online);
  const rect = node => { const r = node.getBoundingClientRect(); return {left: r.left, top: r.top, width: r.width, height: r.height}; };
  let original = null;
  function snapshot() {
    const e = current();
    return {startup: __networkToolkitStartupState, nativeCapability: /^[a-f0-9]{64}$/.test(globalThis.__networkToolkitNativeSession || ''), location: location.pathname,
      view: DETACHED_TERMINAL ? TERMINAL_VIEW.id : '', session: e?.info.id || '', state: e?.info.state || '', editable: editable(e), owner: e ? terminalSessionOwner(e.info.id) : null,
      transcript: e?.xterm ? terminalBufferText(e.xterm.buffer.normal).trimEnd() : '',
      geometry: {width: innerWidth, height: innerHeight, tabs: [...document.querySelectorAll('#terminalSidebarSFTP [data-tsftp-item]')].filter(n => n.getClientRects().length).map(n => ({uid: n.dataset.tsftpItem, ...rect(n)}))},
      outputBlocks: {workspace: !!DETACHED_WORKSPACE, gestures: {...gestures}, sidebarAvailable: NetworkToolkitTerminalSFTP.available(e), visibleModal: !!document.querySelector('.modal-backdrop:not(.hidden), dialog[open], [data-native-copy-handle]')}, errors: [...errors]};
  }
  async function execute(c) {
    if (c.action === 'snapshot') return snapshot();
    if (c.action === 'setup-sftp') {
      selectTerminal(c.destination);
      await wait(() => current()?.info.id === c.destination && editable(current()) && terminalBufferText(current().xterm.buffer.normal).includes('SYNTHETIC TERMINAL123 READY'), 'retained authenticated SSH terminal');
      await wait(() => { NetworkToolkitTerminalSFTP.sync(current()); return NetworkToolkitTerminalSFTP.available(current()); }, 'real retained-channel SFTP probe');
      NetworkToolkitSessionSidebar.activateTab('files');
      await wait(() => document.querySelector('#terminalSidebarSFTP [data-tsftp-item]'), 'actual SFTP listing');
      await NetworkToolkitTerminalSFTP.load(c.destination, '/');
      await wait(() => document.querySelector('[data-tsftp-item="/one.txt"]'), 'real root file row');
      original = {entry: current(), session: current().info.id, transcript: terminalBufferText(current().xterm.buffer.normal).trimEnd()};
      return snapshot();
    }
    if (c.action.startsWith('row:')) {
      const path = c.action.slice(4), row = await wait(() => document.querySelector('#terminalSidebarSFTP [data-tsftp-item="' + CSS.escape(path) + '"]'), 'requested row');
      row.scrollIntoView({block: 'center', inline: 'nearest'});
      await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
      const r = row.getBoundingClientRect(), clip = {left: Math.max(0, r.left), top: Math.max(0, r.top), right: Math.min(innerWidth, r.right), bottom: Math.min(innerHeight, r.bottom)};
      for (let p = row.parentElement; p; p = p.parentElement) {
        const style = getComputedStyle(p), q = p.getBoundingClientRect();
        if (/auto|scroll|hidden|clip/.test(style.overflowX)) { clip.left = Math.max(clip.left, q.left); clip.right = Math.min(clip.right, q.right); }
        if (/auto|scroll|hidden|clip/.test(style.overflowY)) { clip.top = Math.max(clip.top, q.top); clip.bottom = Math.min(clip.bottom, q.bottom); }
      }
      let point;
      for (let y = clip.top + 2; y < clip.bottom - 1 && !point; y += 2) for (let x = clip.left + 8; x < Math.min(clip.right - 2, clip.left + 120); x += 8) {
        if (document.elementFromPoint(x, y)?.closest('[data-tsftp-item]') === row) { point = {x, y}; break; }
      }
      if (!point) throw Error('Terminal SFTP source has no real hit-testable point');
      const out = snapshot(); out.geometry.tabs = [{uid: path, left: point.x, top: point.y, width: 1, height: 1}]; return out;
    }
    if (c.action === 'verify-retained') {
      if (!original || current() !== original.entry || current().info.id !== original.session || !editable(current()) || current().info.state !== 'connected' || terminalBufferText(current().xterm.buffer.normal).trimEnd() !== original.transcript) throw Error('Native copy changed retained Terminal session/output');
      if (document.querySelector('[data-native-copy-handle], dialog[open], #tsftpOperationForm')) throw Error('Copy unexpectedly opened a Prepare/Save/export dialog');
      return snapshot();
    }
    if (c.action === 'detach-workspace') { if (!editable(current()) || DETACHED_TERMINAL) throw Error('Invalid workspace detach source'); void detachTerminalWorkspace(); return snapshot(); }
    throw Error('Unknown fixed Terminal SFTP123 command ' + c.action);
  }
  const headers = {'Content-Type': 'application/json', 'X-Session-Token': globalThis.__networkToolkitNativeSession};
  async function exchange(body) { const response = await fetch('/__native_terminal_fixture', {method: 'POST', headers, body: JSON.stringify(body)}); if (!response.ok) throw Error('Fixture channel ' + response.status); return response.json(); }
  try {
    await wait(() => globalThis.__networkToolkitStartupState === 1 && typeof terminalSessions !== 'undefined', 'production native bootstrap');
    if (!DETACHED_TERMINAL) document.querySelector('#navigation [data-page="terminal"]').click();
    for (;;) { const c = await exchange({kind: 'next'}); if (!c.id) continue; let result; try { result = {state: await execute(c)}; } catch (e) { result = {error: String(e.stack || e), state: snapshot()}; } await exchange({kind: 'result', id: c.id, ...result}); }
  } catch (e) { await exchange({kind: 'fatal', error: String(e.stack || e)}).catch(() => {}); }
})();
