/* Network Toolkit: explicit copying from parsed terminal output.
 * Uses xterm's public buffer/cell APIs, never the original ANSI/OSC stream.
 * No terminal input, selection rewriting during a drag, or clipboard writes
 * from output events. The host must check suppressesInput(entry) before
 * queuing xterm onData/onBinary events, including focus/mouse reports.
 */
(function (global) {
  'use strict';

  const controllers = new WeakMap();
  const gestures = new Set();
  let synchronousGuards = 0;
  const LIMITS = Object.freeze({ cells: 1048576, text: 1048576, bytes: 1048576, drag: 5 });
  const LABELS = Object.freeze({ address: 'IP / CIDR', mac: 'MAC address', interface: 'Interface label',
    warning: 'Warning', error: 'Error', success: 'Status' });
  const COPY_ICON = '<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.7" aria-hidden="true"><rect x="8" y="8" width="12" height="12" rx="2"></rect><path d="M16 8V5a2 2 0 0 0-2-2H5a2 2 0 0 0-2 2v9a2 2 0 0 0 2 2h3"></path></svg>';

  function clean(value) { return String(value || '').replace(/[\x00-\x1f\x7f-\x9f]/g, ''); }
  function readLine(term, row) {
    const buffer = term?.buffer?.active;
    if (!buffer || !Number.isInteger(row) || row < 0 || row >= buffer.length) return null;
    let first = row, last = row, scanned = 0;
    while (first > 0 && buffer.getLine(first)?.isWrapped) {
      if (++scanned > LIMITS.cells) return null;
      first--;
    }
    while (last + 1 < buffer.length && buffer.getLine(last + 1)?.isWrapped) {
      if (++scanned > LIMITS.cells) return null;
      last++;
    }
    const parts = [], cell = buffer.getNullCell();
    let cells = 0, length = 0;
    for (let y = first; y <= last; y++) {
      const line = buffer.getLine(y);
      if (!line) return null;
      let columns = Math.min(term.cols, line.length);
      // Null padding is layout, including a gap before a wrapped wide glyph.
      // Actual spaces sent by the peer are retained, even at the line end.
      while (columns > 0) {
        const tail = line.getCell(columns - 1, cell);
        if (tail?.getChars()) break;
        columns--;
      }
      cells += columns;
      if (cells > LIMITS.cells) return null;
      let text = '';
      for (let column = 0; column < columns; column++) {
        const value = line.getCell(column, cell);
        if (!value || value.getWidth() === 0) continue;
        // Hidden SGR cells are copied as the spaces the user can see.
        const chars = value.isInvisible?.() ? ' '.repeat(Math.max(1, value.getWidth())) : clean(value.getChars()) || ' ';
        length += chars.length;
        if (length > LIMITS.text) return null;
        text += chars;
      }
      parts.push(text);
    }
    const text = parts.join('');
    if (new TextEncoder().encode(text).length > LIMITS.bytes) return null;
    return { text, row, first, last, buffer };
  }
  function preferences() {
    try { return global.NetworkToolkitTerminalInteraction?.preferences?.() || {}; } catch (_) { return {}; }
  }
  function suppressesInput(entry) {
    const controller = controllers.get(entry);
    return synchronousGuards > 0 || gestures.size > 0 || !!controller && controller.depth > 0;
  }
  function withInputSuppressed(entry, callback) {
    const controller = controllers.get(entry);
    synchronousGuards++;
    if (controller) controller.depth++;
    try { return callback(); } finally { synchronousGuards--; if (controller) controller.depth--; }
  }
  function tell(message, error) {
    if (typeof global.toast === 'function') global.toast(message, error ? 'error' : 'success');
  }
  async function write(entry, text) {
    const interaction = global.NetworkToolkitTerminalInteraction;
    if (typeof interaction?.copyValue === 'function') return interaction.copyValue(text, { entry, silent: true });
    try {
      if (global.navigator?.clipboard?.writeText) { await global.navigator.clipboard.writeText(text); return true; }
    } catch (_) {}
    return withInputSuppressed(entry, () => {
      const active = global.document.activeElement, input = global.document.createElement('textarea');
      input.value = text;
      input.setAttribute('aria-label', 'Copy terminal output');
      input.style.cssText = 'position:fixed;left:-10000px;top:0;opacity:0';
      global.document.body.append(input);
      let copied = false;
      try { input.focus({ preventScroll: true }); input.select(); copied = global.document.execCommand('copy'); }
      catch (_) {}
      finally { input.remove(); active?.focus?.({ preventScroll: true }); }
      return copied;
    });
  }
  function valid(entry, snapshot) {
    const controller = controllers.get(entry);
    if (!controller || controller.disposed || !snapshot || snapshot.controller !== controller ||
        entry.xterm !== controller.term || controller.term.buffer.active !== snapshot.buffer || !controller.visible()) return false;
    const line = readLine(controller.term, snapshot.row);
    return !!line && line.text === snapshot.lineText && line.first === snapshot.first && line.last === snapshot.last;
  }
  async function copy(entry, snapshot, label) {
    if (!valid(entry, snapshot)) { tell('The terminal output changed. Choose the line or value again.', true); return false; }
    if (snapshot.text === '') { controllers.get(entry)?.feedback('This line is blank'); return false; }
    const copied = await write(entry, snapshot.text);
    const controller = controllers.get(entry);
    if (copied) controller?.feedback(label + ' copied');
    else tell('Copy is unavailable. Select the text and use Ctrl+Shift+C.', true);
    return !!copied;
  }
  function context(entry, event) {
    const controller = controllers.get(entry);
    return controller && !controller.disposed ? controller.context(event) : { line: null, token: null };
  }
  function copyLine(entry, snapshot) { return copy(entry, snapshot || context(entry).line, 'Line'); }
  function copyToken(entry, snapshot) {
    const token = snapshot || context(entry).token;
    return copy(entry, token, LABELS[token?.kind] || 'Value');
  }

  function bind(entry, options) {
    if (controllers.has(entry)) return controllers.get(entry).public;
    const term = entry?.xterm, root = term?.element, opts = options || {};
    if (!root || !term.buffer?.active) return { dispose() {} };
    const document = root.ownerDocument, subscriptions = [], disposers = [];
    const overlay = document.createElement('div');
    overlay.className = 'nt-terminal-copy-tools';
    overlay.inert = true; // Line copy remains in the context menu, without overlaying output.
    overlay.setAttribute('role', 'group');
    overlay.setAttribute('aria-label', 'Copy terminal output');
    overlay.innerHTML = '<button type="button" class="nt-terminal-copy-line" data-terminal-copy-action="line">' + COPY_ICON + '<span>Copy line</span></button>' +
      '<button type="button" class="nt-terminal-copy-value" data-terminal-copy-action="token" hidden>' + COPY_ICON + '<span>Copy value</span></button>';
    const lineButton = overlay.firstElementChild, tokenButton = overlay.lastElementChild;
    lineButton.title = 'Copy this complete line. Wrapped rows are joined; no Enter is added. Use Up/Down to choose a row and Left/Right to choose a highlighted value.';
    const hint = document.createElement('div');
    hint.className = 'nt-terminal-copy-hint';
    hint.setAttribute('aria-hidden', 'true');
    const rowHighlight = document.createElement('div');
    rowHighlight.className = 'nt-terminal-copy-row-highlight';
    rowHighlight.setAttribute('aria-hidden','true');
    const status = document.createElement('span');
    status.className = 'nt-terminal-copy-status';
    status.setAttribute('role', 'status');
    status.setAttribute('aria-live', 'polite');
    root.append(rowHighlight, overlay, hint, status);
    root.classList.add('nt-terminal-copy-ready');
    const controller = { term, root, disposed: false, revision: 0, depth: 0, gesture: null, public: null };
    controllers.set(entry, controller);
    let hover = null, keyboard = false, valueIndex = 0, feedbackTimer = null, hoverFrame = null, pendingPoint = null;

    function listen(node, type, handler, capture) {
      node.addEventListener(type, handler, capture);
      disposers.push(() => node.removeEventListener(type, handler, capture));
    }
    function subscribe(event, callback) { if (typeof event === 'function') subscriptions.push(event(callback)); }
    function geometry() {
      const screen = root.querySelector('.xterm-screen'), rect = screen?.getBoundingClientRect(), parent = root.getBoundingClientRect();
      if (!rect || rect.width <= 0 || rect.height <= 0 || !term.cols || !term.rows) return null;
      return { rect, parent, width: rect.width / term.cols, height: rect.height / term.rows };
    }
    function visible() { return !controller.disposed && root.isConnected && root.getClientRects().length > 0 && !!geometry(); }
    controller.visible = visible;
    function point(event) {
      const g = geometry();
      if (!g || !Number.isFinite(event?.clientX) || !Number.isFinite(event?.clientY) || event.clientX < g.rect.left || event.clientX >= g.rect.right || event.clientY < g.rect.top || event.clientY >= g.rect.bottom) return null;
      return { column: Math.max(0, Math.min(term.cols - 1, Math.floor((event.clientX - g.rect.left) / g.width))),
        row: term.buffer.active.viewportY + Math.max(0, Math.min(term.rows - 1, Math.floor((event.clientY - g.rect.top) / g.height))) };
    }
    // Syntax colors must not turn every command/setting word into a new
    // click-to-copy target, which would interfere with editing terminal input.
    function tokens(row) { return (entry.highlighter?.tokensAtRow?.(row) || []).filter(token => Object.hasOwn(LABELS, token.kind)); }
    function tokenAt(column, row) { const token = entry.highlighter?.tokenAt?.(column, row); return token && Object.hasOwn(LABELS, token.kind) ? token : null; }
    function snapshot(row, token) {
      const line = readLine(term, row);
      if (!line) return null;
      return { controller, revision: controller.revision, buffer: line.buffer, row, first: line.first, last: line.last,
        lineText: line.text, text: token ? clean(token.text) : line.text, kind: token?.kind || '', token: token || null };
    }
    function cursorPoint() {
      const buffer = term.buffer.active;
      return { row: Math.max(buffer.viewportY, Math.min(buffer.viewportY + term.rows - 1, buffer.baseY + buffer.cursorY)), column: Math.min(term.cols - 1, buffer.cursorX) };
    }
    controller.context = event => {
      const location = event && (event.clientX !== 0 || event.clientY !== 0) ? point(event) : null;
      const selected = location || (hover?.revision === controller.revision ? hover : cursorPoint());
      const token = tokenAt(selected.column || 0, selected.row);
      return { line: snapshot(selected.row), token: token ? snapshot(selected.row, token) : null };
    };
    function allowed() { return preferences().copyOnClick !== false; }
    function resetVisual() {
      root.classList.remove('nt-terminal-copy-hover', 'nt-terminal-copy-token-hover');
      hint.classList.remove('visible');
      rowHighlight.classList.remove('visible');
      hover = null;
      root.removeAttribute('title');
      lineButton.disabled = false;
      tokenButton.hidden = true;
    }
    function show(row, column, token, fromKeyboard) {
      if (!allowed() || !visible()) { resetVisual(); return; }
      const g = geometry(), buffer = term.buffer.active;
      if (row < buffer.viewportY || row >= buffer.viewportY + term.rows) { resetVisual(); return; }
      hover = { row, column, token, revision: controller.revision, line: snapshot(row), value: token ? snapshot(row, token) : null };
      const y = (row - buffer.viewportY) * g.height + g.rect.top - g.parent.top;
      overlay.style.top = Math.max(g.rect.top - g.parent.top, Math.min(g.rect.bottom - g.parent.top - 24, y)) + 'px';
      overlay.style.left = Math.max(0, g.rect.left - g.parent.left + 2) + 'px';
      overlay.style.right = 'auto';
      overlay.style.maxWidth = Math.max(80, g.rect.width - 4) + 'px';
      rowHighlight.style.left = (g.rect.left-g.parent.left)+'px';
      rowHighlight.style.top = y+'px';
      rowHighlight.style.width = g.rect.width+'px';
      rowHighlight.style.height = g.height+'px';
      rowHighlight.classList.add('visible');
      root.classList.add('nt-terminal-copy-hover');
      lineButton.disabled = !hover.line;
      lineButton.setAttribute('aria-label', 'Copy line ' + (row + 1) + (hover.line?.first !== hover.line?.last ? ', including its wrapped rows' : '') + '. Up and Down choose a row. Left and Right choose a highlighted value.');
      tokenButton.hidden = !fromKeyboard || !hover.value;
      if (hover.value) {
        tokenButton.querySelector('span').textContent = 'Copy ' + (LABELS[token.kind] || 'value');
        tokenButton.title = token.text;
        tokenButton.setAttribute('aria-label', 'Copy ' + (LABELS[token.kind] || 'highlighted value') + ': ' + token.text);
      }
      const clickable = !!token && term.modes.mouseTrackingMode === 'none';
      root.classList.toggle('nt-terminal-copy-token-hover', clickable && !fromKeyboard);
      hint.classList.toggle('visible', clickable && !fromKeyboard);
      if (clickable && !fromKeyboard) {
        root.title = 'Click to copy ' + (LABELS[token.kind] || 'value') + '. Use the terminal context menu to copy a whole line.';
        hint.textContent = 'Click to copy ' + (LABELS[token.kind] || 'value');
        hint.style.left = Math.max(g.rect.left - g.parent.left + 2, Math.min(g.rect.width - 170, (column + 1) * g.width + g.rect.left - g.parent.left)) + 'px';
        hint.style.top = Math.max(g.rect.top - g.parent.top, Math.min(g.rect.bottom - g.parent.top - 22, y + g.height + 2)) + 'px';
      }
    }
    function invalidate(preserveUnchanged) {
      controller.revision++;
      const focused = overlay.contains(document.activeElement);
      const previous = hover;
      if (preserveUnchanged === true && previous?.line && valid(entry, previous.line) && allowed()) {
        show(previous.row, previous.column, previous.token, keyboard);
        return;
      }
      resetVisual();
      if (focused && allowed()) {
        const position = cursorPoint();
        keyboard = true;
        show(position.row, position.column, tokenAt(position.column, position.row), true);
      }
    }
    controller.feedback = message => {
      global.clearTimeout(feedbackTimer);
      status.textContent = message;
      root.classList.add('nt-terminal-copy-confirmed');
      feedbackTimer = global.setTimeout(() => { status.textContent = ''; root.classList.remove('nt-terminal-copy-confirmed'); }, 1600);
    };
    function activate() { if (typeof opts.activate === 'function') opts.activate(); }
    function restoreSelection(gesture) {
      const selection = gesture?.selection;
      if (!selection || gesture.revision !== controller.revision) return;
      const length = (selection.end.y - selection.start.y) * term.cols + selection.end.x - selection.start.x;
      if (length > 0) term.select(selection.start.x, selection.start.y, length);
    }
    function endGesture(restore) {
      const gesture = controller.gesture;
      if (!gesture) return;
      if (restore) {
        restoreSelection(gesture);
        if (gesture.focus?.isConnected) gesture.focus.focus?.({ preventScroll: true });
      }
      controller.gesture = null;
      gestures.delete(controller);
    }
    function controlTarget(event) { return event.target?.closest?.('[data-terminal-copy-action]'); }
    function stop(event) { event.preventDefault(); event.stopImmediatePropagation(); }

    listen(root, 'pointermove', event => {
      const gesture = controller.gesture;
      if (gesture && Math.hypot(event.clientX - gesture.x, event.clientY - gesture.y) > LIMITS.drag) {
        gesture.dragged = true;
        activate();
        // Native xterm selection owns the drag; no clipboard action follows.
        endGesture(false);
      }
      if (controlTarget(event) || event.buttons) return;
      if (global.NetworkToolkitTerminalInput?.contains(entry,event)) { resetVisual(); return; }
      pendingPoint = { clientX: event.clientX, clientY: event.clientY };
      if (hoverFrame !== null) return;
      hoverFrame = global.requestAnimationFrame(() => {
        hoverFrame = null;
        if (!pendingPoint || controller.disposed || keyboard) return;
        const location = point(pendingPoint);
        if (!location) { resetVisual(); return; }
        if (hover?.revision === controller.revision && hover.row === location.row && hover.column === location.column) return;
        const token = tokenAt(location.column, location.row);
        show(location.row, location.column, token, false);
      });
    }, true);
    listen(root, 'pointerleave', () => { if (!overlay.contains(document.activeElement)) { keyboard = false; resetVisual(); } });
    listen(root, 'pointerdown', event => {
      if (controlTarget(event)) { stop(event); return; }
      keyboard = false;
      if (global.NetworkToolkitTerminalInput?.contains(entry,event)) return;
      if (!allowed() || event.button !== 0 || event.ctrlKey || event.metaKey || event.altKey || event.shiftKey || term.modes.mouseTrackingMode !== 'none') return;
      const location = point(event), token = location && tokenAt(location.column, location.row);
      if (!token) return;
      const value = snapshot(location.row, token);
      if (!value) return;
      controller.gesture = { x: event.clientX, y: event.clientY, pointer: event.pointerId, value, revision: controller.revision,
        selection: term.getSelectionPosition?.(), focus: document.activeElement, dragged: false, second: false };
      gestures.add(controller);
      // xterm uses compatibility mousedown for selection. Preserve that event,
      // while an eventual copy click must not change the workspace's pane.
      event.stopPropagation();
    }, true);
    listen(root, 'mousedown', event => {
      if (controlTarget(event)) { stop(event); return; }
      if (controller.gesture && event.detail > 1) {
        controller.gesture.second = true;
        activate();
        endGesture(false);
      }
    }, true);
    listen(root, 'focusin', event => { if (controller.gesture) event.stopPropagation(); }, true);
    listen(root, 'mouseup', event => {
      if (controlTarget(event)) stop(event);
    }, true);
    // xterm can report mouse motion in a remote full-screen application.
    // Explicit overlay controls must never generate those reports.
    for (const type of ['mousemove', 'dblclick', 'contextmenu']) listen(root, type, event => {
      if (controlTarget(event)) stop(event);
    }, true);
    listen(document, 'pointerup', event => {
      const gesture = controller.gesture;
      if (!gesture || event.pointerId !== gesture.pointer) return;
      if (event.button !== 0 || Math.hypot(event.clientX - gesture.x, event.clientY - gesture.y) > LIMITS.drag) { endGesture(false); return; }
      const location = point(event), token = location && tokenAt(location.column, location.row);
      if (!token || token.text !== gesture.value.text || !valid(entry, gesture.value)) { endGesture(false); return; }
      // Keep the guard through compatibility mouseup/click: xterm can emit
      // cursor/focus reports there. Copy only after a complete no-drag click.
      gesture.ready = true;
      gesture.timer = global.setTimeout(() => { if (controller.gesture === gesture) endGesture(false); }, 100);
    }, true);
    listen(root, 'click', event => {
      const target = controlTarget(event);
      if (target) {
        stop(event);
        const value = target.dataset.terminalCopyAction === 'token' ? hover?.value : hover?.line;
        if (value) void (target.dataset.terminalCopyAction === 'token' ? copyToken(entry, value) : copyLine(entry, value));
        return;
      }
      const gesture = controller.gesture;
      if (!gesture?.ready || event.button !== 0) return;
      stop(event);
      global.clearTimeout(gesture.timer);
      void copyToken(entry, gesture.value);
      endGesture(true);
    }, true);
    listen(document, 'pointercancel', () => endGesture(false), true);
    listen(global, 'blur', () => { endGesture(false); resetVisual(); });
    listen(global, 'network-toolkit-terminal-preferences', () => {
      if (!allowed()) { endGesture(false); resetVisual(); lineButton.tabIndex = -1; }
      else lineButton.tabIndex = 0;
    });
    listen(overlay, 'focusin', event => {
      if (!allowed()) return;
      keyboard = true;
      if (!hover || hover.revision !== controller.revision) {
        const position = cursorPoint();
        show(position.row, position.column, tokenAt(position.column, position.row), true);
      } else show(hover.row, hover.column, hover.token, true);
      event.stopPropagation();
    });
    listen(overlay, 'focusout', () => global.setTimeout(() => {
      if (!overlay.contains(document.activeElement)) { keyboard = false; resetVisual(); }
    }, 0));
    listen(overlay, 'keydown', event => {
      if (!allowed()) return;
      if (['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(event.key)) {
        stop(event);
        const buffer = term.buffer.active, first = buffer.viewportY, last = Math.min(buffer.length - 1, first + term.rows - 1);
        const row = event.key === 'Home' ? first : event.key === 'End' ? last : Math.max(first, Math.min(last, (hover?.row ?? cursorPoint().row) + (event.key === 'ArrowUp' ? -1 : 1)));
        valueIndex = 0;
        const values = tokens(row), token = values[0] || null;
        show(row, token?.spans.find(span => span.row === row)?.x || 0, token, true);
      } else if (['ArrowLeft', 'ArrowRight'].includes(event.key)) {
        stop(event);
        const row = hover?.row ?? cursorPoint().row, values = tokens(row);
        if (!values.length) return;
        valueIndex = (valueIndex + (event.key === 'ArrowLeft' ? -1 : 1) + values.length) % values.length;
        const token = values[valueIndex];
        show(row, token.spans.find(span => span.row === row)?.x || 0, token, true);
        status.textContent = (LABELS[token.kind] || 'Value') + ': ' + token.text + '. Tab to Copy value.';
      } else if (event.key === 'Escape') {
        stop(event);
        withInputSuppressed(entry, () => term.focus());
        resetVisual();
      } else if (event.key === 'Enter' || event.key === ' ') {
        stop(event);
        const value = event.target === tokenButton ? hover?.value : hover?.line;
        if (value) void (event.target === tokenButton ? copyToken(entry, value) : copyLine(entry, value));
      }
    }, true);
    subscribe(term.onWriteParsed, () => invalidate(true));
    subscribe(term.onScroll, () => invalidate());
    subscribe(term.onResize, () => invalidate());
    subscribe(term.buffer.onBufferChange, () => invalidate());
    resetVisual();
    lineButton.disabled = false; // A tab stop even before the first hover.
    lineButton.tabIndex = allowed() ? 0 : -1;
    lineButton.setAttribute('aria-label', 'Copy terminal line. Up and Down choose a row. Left and Right choose a highlighted value.');
    controller.public = {
      dispose() {
        if (controller.disposed) return;
        controller.disposed = true;
        global.clearTimeout(feedbackTimer);
        if (hoverFrame !== null) global.cancelAnimationFrame(hoverFrame);
        endGesture(false);
        for (const dispose of disposers) dispose();
        for (const subscription of subscriptions) subscription?.dispose?.();
        overlay.remove(); hint.remove(); rowHighlight.remove(); status.remove();
        root.classList.remove('nt-terminal-copy-ready', 'nt-terminal-copy-hover', 'nt-terminal-copy-token-hover', 'nt-terminal-copy-confirmed');
        controllers.delete(entry);
      }
    };
    return controller.public;
  }
  global.NetworkToolkitTerminalCopy = Object.freeze({ bind, context, copyLine, copyToken, suppressesInput, withInputSuppressed, readLine, limits: LIMITS });
})(typeof globalThis !== 'undefined' ? globalThis : window);
