/* A native text control for one bounded document page. Selections and the
   scroll map use document offsets even when they extend outside that page. */
(function (root) {
'use strict';

const RANGE = 1000000;
const BUFFER_LINES = 40;
const PREVIEW_UNITS = 512;
const clamp = (value, minimum, maximum) => Math.min(maximum, Math.max(minimum, value));

function create({textarea, gutter, container, session, changed = () => {}, message = () => {}, readOnly = false}) {
 const pageUnits = root.NTKEditorDiskSession.PAGE_UNITS;
 const abort = new AbortController();
 const listeners = {signal: abort.signal};
 const original = {
  readOnly: textarea.readOnly,
  wrap: textarea.wrap,
  width: gutter.parentElement.style.width,
  basis: gutter.parentElement.style.flexBasis,
  padding: gutter.style.paddingTop,
  transform: gutter.style.transform
 };
 const vertical = scrollBar('nt-editor-disk-scroll', 'Document position', 'vertical');
 const horizontal = scrollBar('nt-editor-disk-horizontal', 'Position in long line', 'horizontal');
 container.classList.add('nt-editor-disk');
 textarea.wrap = 'off';

 let page = session.state().page;
 let starts = lineStarts(page.text);
 let selection = {start: page.start, end: page.start, direction: 'none'};
 let virtual = false, composing = false, disposed = false, editing = false;
 let navigation = 0, navigating = false, frame = 0, pointer = false;
 let top = page.line, barTop = 0, barLeft = 0, lastScrollTop = 0, lastScrollLeft = 0;
 let gutterStamp = '', lastAnnouncement = '', lastPrefetch = '';
 let rowHeight = 20, characterWidth = 8, clientRows = 1, clientColumns = 1, font = '';
 let verticalFrame = 0, horizontalFrame = 0;
 textarea.value = page.text;
 textarea.setSelectionRange(0, 0);
 updateReadOnly();

 function scrollBar(className, label, orientation) {
  const bar = document.createElement('div');
  const spacer = document.createElement('div');
  bar.className = className;
  bar.tabIndex = 0;
  bar.setAttribute('role', 'scrollbar');
  bar.setAttribute('aria-label', label);
  bar.setAttribute('aria-orientation', orientation);
  if (orientation === 'vertical') spacer.style.height = RANGE + 'px';
  else {
   spacer.style.width = RANGE + 'px';
   spacer.style.height = '1px';
  }
  bar.append(spacer);
  container.append(bar);
  return bar;
 }

 function lineStarts(text) {
  const offsets = [0];
  for (let index = -1; (index = text.indexOf('\n', index + 1)) >= 0;) offsets.push(index + 1);
  return offsets;
 }

 function localLine(offset) {
  let low = 0, high = starts.length;
  while (low + 1 < high) {
   const mid = Math.floor((low + high) / 2);
   if (starts[mid] <= offset) low = mid;
   else high = mid;
  }
  return low;
 }

 function updateReadOnly() {
  textarea.readOnly = readOnly || editing || session.state().blocked;
 }

 function report(error) {
  if (!disposed && error?.name !== 'AbortError') message(error.message, true);
 }

 function run(promise) { void promise.catch(report); }

 function capture() {
  if (!virtual) {
   selection = {
    start: page.start + textarea.selectionStart,
    end: page.start + textarea.selectionEnd,
    direction: textarea.selectionDirection || 'none'
   };
  }
  return {...selection, top, left: textarea.scrollLeft + page.column * characterWidth};
 }

 function announce() {
  const view = capture();
  const stamp = JSON.stringify(view);
  if (stamp !== lastAnnouncement) {
   lastAnnouncement = stamp;
   changed(view);
  }
 }

 function select(start, end = start, direction = 'none', focus = false) {
  const size = session.state().metadata.units;
  start = clamp(start, 0, size);
  end = clamp(end, start, size);
  selection = {start, end, direction};
  virtual = start < page.start || end > page.start + page.text.length;
  textarea.setSelectionRange(
   clamp(start - page.start, 0, page.text.length),
   clamp(end - page.start, 0, page.text.length),
   direction
  );
  if (focus) textarea.focus({preventScroll: true});
  announce();
 }

 function metrics() {
  const style = root.getComputedStyle(textarea);
  rowHeight = parseFloat(style.lineHeight) || 20;
  const fontStyle = style.font || style.fontSize + ' ' + style.fontFamily;
  if (fontStyle !== font) {
   const context = document.createElement('canvas').getContext('2d');
   if (context) {
    context.font = fontStyle;
    characterWidth = context.measureText('0').width || 8;
   }
   font = fontStyle;
  }
  clientRows = Math.max(1, Math.floor(textarea.clientHeight / rowHeight));
  clientColumns = Math.max(1, Math.floor(textarea.clientWidth / characterWidth));
 }

 function paint() {
  if (disposed) return;
  frame = 0;
  const {lines} = session.state().metadata;
  const row = clamp(Math.floor(textarea.scrollTop / rowHeight), 0, starts.length - 1);
  const first = Math.max(0, row - 3);
  const last = Math.min(starts.length, first + clientRows + 8);
  const stamp = [page.line, first, last, rowHeight, String(lines).length].join(':');
  if (stamp !== gutterStamp) {
   gutterStamp = stamp;
   const rows = [];
   for (let line = first; line < last; line++) {
    const element = document.createElement('div');
    element.style.height = rowHeight + 'px';
    element.style.lineHeight = rowHeight + 'px';
    element.textContent = String(page.line + line + 1);
    rows.push(element);
   }
   gutter.style.paddingTop = first * rowHeight + 'px';
   gutter.replaceChildren(...rows);
   const width = String(lines).length + 3 + 'ch';
   gutter.parentElement.style.width = width;
   gutter.parentElement.style.flexBasis = width;
  }
  gutter.style.transform = 'translateY(' + (-textarea.scrollTop) + 'px)';
  top = page.line + row;
  vertical.hidden = lines <= clientRows;
  const lastTop = Math.max(1, lines - clientRows);
  barTop = clamp(top / lastTop, 0, 1) * (RANGE - vertical.clientHeight);
  if (Math.abs(vertical.scrollTop - barTop) > 1) vertical.scrollTop = barTop;
  scrollbarValue(vertical, lines, top + 1);
  const longLine = page.lineUnits > pageUnits;
  horizontal.hidden = !longLine;
  container.classList.toggle('nt-editor-disk-long-line', longLine);
  if (longLine) {
   const column = page.column + textarea.scrollLeft / characterWidth;
   const lastColumn = Math.max(1, page.lineUnits - clientColumns);
   barLeft = clamp(column / lastColumn, 0, 1) * (RANGE - horizontal.clientWidth);
   if (Math.abs(horizontal.scrollLeft - barLeft) > 1) horizontal.scrollLeft = barLeft;
   scrollbarValue(horizontal, page.lineUnits + 1, Math.floor(column) + 1);
  }
  lastScrollTop = textarea.scrollTop;
  lastScrollLeft = textarea.scrollLeft;
  announce();
 }

 function scrollbarValue(element, maximum, value) {
  element.setAttribute('aria-valuemin', '1');
  element.setAttribute('aria-valuemax', String(maximum));
  element.setAttribute('aria-valuenow', String(Math.min(maximum, value)));
 }

 function schedulePaint() {
  if (!frame && !disposed) frame = root.requestAnimationFrame(paint);
 }

 function install(value) {
  if (disposed || !value) return;
  const view = capture();
  page = value;
  starts = lineStarts(page.text);
  if (textarea.value !== page.text) textarea.value = page.text;
  select(view.start, view.end, view.direction);
  updateReadOnly();
 }

 async function navigate(operation) {
  if (disposed || composing || editing) return false;
  const ticket = ++navigation;
  navigating = true;
  const active = () => !disposed && !composing && ticket === navigation;
  try {
   return await operation(active);
  } finally {
   if (ticket === navigation) navigating = false;
  }
 }

 async function loadPage(target, active) {
  const result = await session.load(target);
  if (!result || !active()) return false;
  install(result);
  return true;
 }

 function showLine(line) {
  textarea.scrollTop = Math.max(0, (line - page.line) * rowHeight);
  paint();
 }

 function alignCursor(offset) {
  const at = offset - page.start;
  if (at > 0 && at < page.text.length &&
      /[\uD800-\uDBFF]/.test(page.text[at - 1]) &&
      /[\uDC00-\uDFFF]/.test(page.text[at])) return offset - 1;
  return offset;
 }

 async function load(target, options = {}) {
  return navigate(async active => {
   if (!await loadPage(target, active)) return false;
   if (options.caret !== undefined) select(options.caret, options.caret, 'none', options.focus);
   else if (options.keepSelection === false) select(page.start);
   if (options.left !== undefined) textarea.scrollLeft = options.left;
   showLine(options.line ?? page.line);
   return true;
  });
 }

 async function openLine(line, active, force = false) {
  const count = session.state().metadata.lines;
  line = clamp(Math.floor(line), 0, count - 1);
  const row = line - page.line;
  const lastPage = page.start + page.text.length >= session.state().metadata.units;
  if (!force && row >= 0 && (row !== 0 || page.column === 0) &&
      row < starts.length && (lastPage || row + clientRows < starts.length)) return true;
  const from = Math.max(0, line - BUFFER_LINES);
  if (!await loadPage({line: from}, active)) return false;
  if (line >= page.line + starts.length || (line === page.line && page.column)) {
   // A very long line may consume the look-behind page before the target line.
   return loadPage({line}, active);
  }
  if (line > page.line && line === page.line + starts.length - 1 &&
      page.start + page.text.length < session.state().metadata.units) {
   // Obtain the complete logical length for a last row that may span pages.
   return loadPage({line}, active);
  }
  return true;
 }

 async function goLine(line, focus = false, force = false) {
  line = clamp(Math.floor(line), 0, session.state().metadata.lines - 1);
  return navigate(async active => {
   if (!await openLine(line, active, force)) return false;
   if (focus) select(page.start + starts[line - page.line], undefined, 'none', true);
   showLine(line);
   return true;
  });
 }

 async function reveal(offset, active) {
  if (offset < page.start || offset > page.start + page.text.length) {
   if (!await loadPage({start: Math.max(0, offset - PREVIEW_UNITS)}, active)) return false;
  }
  return true;
 }

 function showOffset(offset) {
  const local = clamp(offset - page.start, 0, page.text.length);
  const row = localLine(local);
  const column = local - starts[row];
  const right = textarea.scrollLeft + textarea.clientWidth - characterWidth * 4;
  if (column * characterWidth > right || column * characterWidth < textarea.scrollLeft) {
   textarea.scrollLeft = Math.max(0, (column - Math.floor(clientColumns / 2)) * characterWidth);
  }
  showLine(page.line + Math.max(0, row - 2));
 }

 async function position(start, end = start, focus = true) {
  const size = session.state().metadata.units;
  start = clamp(start, 0, size);
  end = clamp(end, start, size);
  return navigate(async active => {
   if (!await reveal(start, active)) return false;
   select(alignCursor(start), alignCursor(end), 'none', focus);
   showOffset(start);
   return true;
  });
 }

 async function restore(view, focus = false) {
  if (!view) return;
  return navigate(async active => {
   const line = clamp(view.top ?? 0, 0, session.state().metadata.lines - 1);
   if (!await openLine(line, active)) return false;
   if (line === page.line && page.lineUnits > pageUnits && view.left > pageUnits * characterWidth) {
    const column = Math.min(page.lineUnits, Math.floor(view.left / characterWidth));
    if (!await loadPage({start: page.start - page.column + column}, active)) return false;
   }
   select(view.start ?? 0, view.end ?? view.start ?? 0, view.direction, focus);
   textarea.scrollLeft = Math.max(0, (view.left || 0) - page.column * characterWidth);
   showLine(line);
   return true;
  });
 }

 async function applyEdit(operation, offset) {
  if (readOnly || editing || disposed) return false;
  if (composing) throw Error('Finish text composition before changing the selection.');
  editing = true;
  navigation++;
  navigating = false;
  session.cancelRead?.();
  updateReadOnly();
  try {
   await operation();
   if (disposed) return false;
   install(session.state().page);
  } finally {
   editing = false;
   if (!disposed) updateReadOnly();
  }
  const at = Math.min(offset, session.state().metadata.units);
  await position(at);
  return true;
 }

 function replace(text, range = capture()) {
  const value = String(text).replace(/\r\n|\r/g, '\n');
  return applyEdit(() => session.replace(range.start, range.end, value), range.start + value.length);
 }

 function history(redo = false) {
  const view = capture();
  return applyEdit(() => session.history(redo), view.start);
 }

 function prefetch(target) {
  const stamp = JSON.stringify([session.state().metadata.revision, target]);
  if (stamp === lastPrefetch) return;
  lastPrefetch = stamp;
  void session.prefetch(target).catch(() => {});
 }

 function panLongLine(column) {
  const start = page.start - page.column;
  const last = Math.max(0, page.lineUnits - clientColumns);
  column = clamp(column, 0, last);
  return load({start: start + column}, {left: 0, keepSelection: true});
 }

 function rollForward(line, origin) {
  return navigate(async active => {
   if (!await loadPage({line: origin}, active)) return false;
   if (line >= page.line + starts.length && !await openLine(line, active)) return false;
   showLine(line);
   return true;
  });
 }

 function scroll() {
  schedulePaint();
  if (composing || navigating || editing || pointer) return;
  const {units, lines} = session.state().metadata;
  const row = clamp(Math.floor(textarea.scrollTop / rowHeight), 0, starts.length - 1);
  const line = page.line + row;
  const movedDown = textarea.scrollTop > lastScrollTop;
  const movedUp = textarea.scrollTop < lastScrollTop;
  if (movedUp && row < BUFFER_LINES / 2 && page.line > 0) {
   const target = Math.max(0, line - BUFFER_LINES);
   if (target < page.line) run(goLine(line, false, true));
  } else if (movedDown && page.start + page.text.length < units) {
   const remaining = starts.length - row - clientRows;
   const origin = page.line + starts.length - clientRows - Math.floor(1.5 * BUFFER_LINES);
   if (origin > page.line) {
    if (remaining < BUFFER_LINES / 2) run(rollForward(line, origin));
    else if (remaining < BUFFER_LINES) prefetch({line: origin});
   }
  }
  if (page.lineUnits > pageUnits && textarea.scrollLeft !== lastScrollLeft && !navigating) {
   const column = page.column + Math.floor(textarea.scrollLeft / characterWidth);
   const remaining = page.text.length - column + page.column - clientColumns;
   if (remaining < 256 && page.column + page.text.length < page.lineUnits) run(panLongLine(column));
   else if (textarea.scrollLeft < characterWidth * 64 && page.column > 0 && textarea.scrollLeft < lastScrollLeft) {
    const rewind = Math.min(page.column, 8192);
    run(load({start: page.start - rewind}, {left: (rewind + column - page.column) * characterWidth}));
   }
  } else if (!navigating && textarea.scrollLeft > lastScrollLeft &&
      textarea.scrollLeft + textarea.clientWidth >= textarea.scrollWidth - 8 &&
      page.start + page.text.length < units && row + clientRows >= starts.length) {
   const lastRow = starts.at(-1);
   const column = Math.floor(textarea.scrollLeft / characterWidth);
   if (column > 0 && column < page.text.length - lastRow - clientColumns / 2) {
    run(load({start: page.start + lastRow + column}, {left: 0}));
   }
  } else if (movedDown && line + clientRows >= lines) {
   lastPrefetch = '';
  }
 }

 function wheel(event) {
  if (composing || navigating || editing || event.defaultPrevented) return;
  const current = session.state().metadata;
  const delta = event.deltaY;
  const row = Math.floor(textarea.scrollTop / rowHeight);
  const bottom = textarea.scrollTop + textarea.clientHeight >= textarea.scrollHeight - 2;
  if (delta < 0 && textarea.scrollTop < 2 && page.line > 0) {
   event.preventDefault();
   run(goLine(page.line - 1, false, true));
  } else if (delta > 0 && bottom && page.line + starts.length < current.lines) {
   event.preventDefault();
   const next = Math.min(page.line + starts.length, page.line + row + Math.max(1, clientRows - 1));
   if (next > page.line) {
    // Load at the next visible line itself if the context page cannot advance.
    run(load({line: next}, {line: next}));
   }
  }
 }

 function changedText() {
  const text = textarea.value;
  if (page.text === text) return;
  session.change(text);
  page = {...page, text};
  starts = lineStarts(text);
  virtual = false;
  schedulePaint();
  if ((text.length > pageUnits * 1.25 || starts.length > root.NTKEditorDiskSession.PAGE_LINES + 128) && !navigating) {
   const cursor = page.start + textarea.selectionStart;
   run(load({start: Math.max(0, cursor - PREVIEW_UNITS)}).then(loaded => {
    if (loaded) showOffset(cursorOffset(capture()));
   }));
  }
 }

 function cursorOffset(view) {
  return view.direction === 'backward' ? view.start : view.end;
 }

 function cursorAnchor(view) {
  return view.direction === 'backward' ? view.end : view.start;
 }

 function moveSelection(anchor, offset, extend) {
  if (!extend) select(offset, offset, 'none', true);
  else select(Math.min(anchor, offset), Math.max(anchor, offset), offset < anchor ? 'backward' : 'forward', true);
  showOffset(offset);
 }

 async function move(key, extend, control) {
  const view = capture();
  const anchor = cursorAnchor(view);
  const cursor = cursorOffset(view);
  return navigate(async active => {
   const {lines, units} = session.state().metadata;
   if (control && (key === 'Home' || key === 'End')) {
    const offset = key === 'Home' ? 0 : units;
    if (!await reveal(offset, active)) return false;
    moveSelection(anchor, offset, extend);
    return true;
   }
   if (!await reveal(cursor, active)) return false;
   const leftEdge = key === 'ArrowLeft' && cursor === page.start && cursor > 0;
   const rightEdge = key === 'ArrowRight' && cursor === page.start + page.text.length && cursor < units;
   if ((leftEdge || rightEdge) && !await loadPage({start: Math.max(0, cursor - PREVIEW_UNITS)}, active)) return false;
   const at = cursor - page.start;
   if (key === 'ArrowLeft' || key === 'ArrowRight') {
    if (!extend && view.end > view.start) {
     const edge = key === 'ArrowLeft' ? view.start : view.end;
     if (!await reveal(edge, active)) return false;
     moveSelection(anchor, edge, false);
     return true;
    }
    const right = key === 'ArrowRight';
    const code = right ? page.text.charCodeAt(at) : page.text.charCodeAt(at - 1);
    const pair = right ? code >= 0xd800 && code <= 0xdbff : code >= 0xdc00 && code <= 0xdfff;
    const offset = clamp(cursor + (right ? 1 : -1) * (pair ? 2 : 1), 0, units);
    if (!await reveal(offset, active)) return false;
    moveSelection(anchor, alignCursor(offset), extend);
    return true;
   }
   const row = localLine(clamp(at, 0, page.text.length));
   const line = page.line + row;
   const column = at - starts[row] + (row === 0 ? page.column : 0);
   const targets = {ArrowUp: line - 1, ArrowDown: line + 1, PageUp: line - clientRows, PageDown: line + clientRows};
   const next = clamp(targets[key] ?? line, 0, lines - 1);
   if (!await openLine(next, active)) return false;
   let nextRow = next - page.line;
   if (key === 'End' && nextRow === starts.length - 1 && page.start + page.text.length < units) {
    if (!await loadPage({line: next}, active)) return false;
    nextRow = 0;
   }
   const begin = starts[nextRow];
   const end = starts[nextRow + 1] !== undefined ? starts[nextRow + 1] - 1 : page.text.length;
   let offset = page.start + clamp(begin + column, begin, end);
   if (key === 'Home') offset = page.start + begin;
   if (key === 'End') {
    offset = page.start + end;
    if (nextRow === 0 && page.lineUnits) offset = page.start - page.column + page.lineUnits;
   }
   if (!await reveal(offset, active)) return false;
   moveSelection(anchor, alignCursor(offset), extend);
   return true;
  });
 }

 async function erase(backwards) {
  const view = capture();
  if (view.start !== view.end) return replace('', view);
  const cursor = view.start;
  const at = cursor - page.start;
  if (at <= 0 || at >= page.text.length) {
   const found = await load({start: Math.max(0, cursor - PREVIEW_UNITS)}, {caret: cursor});
   if (!found) return;
  }
  const index = cursor - page.start;
  const code = backwards ? page.text.charCodeAt(index - 1) : page.text.charCodeAt(index);
  const pair = backwards ? code >= 0xdc00 && code <= 0xdfff : code >= 0xd800 && code <= 0xdbff;
  const units = session.state().metadata.units;
  return replace('', {
   start: backwards ? Math.max(0, cursor - (pair ? 2 : 1)) : cursor,
   end: backwards ? cursor : Math.min(units, cursor + (pair ? 2 : 1))
  });
 }

 function selectedText(limit = Infinity) {
  const {start, end} = capture();
  if (end - start > limit) return Promise.reject(Error('The selection is too large for this command.'));
  if (start >= page.start && end <= page.start + page.text.length) {
   return Promise.resolve(page.text.slice(start - page.start, end - page.start));
  }
  return session.readText(start, end, limit, abort.signal);
 }

 async function copy(cut = false) {
  const view = capture();
  if (view.start === view.end || cut && readOnly) return false;
  const clipboard = root.navigator?.clipboard;
  if (!clipboard) throw Error('Clipboard access is unavailable.');
  if (cut && (composing || editing)) throw Error('Finish the current edit before cutting text.');
  // Lock only this control during a cut so the chosen range cannot be changed
  // while the clipboard receives it. Navigation and the rest of the app remain available.
  const stamp = session.state().editVersion;
  if (cut) {
   editing = true;
   updateReadOnly();
  }
  try {
   if (!virtual) {
    await clipboard.writeText(page.text.slice(view.start - page.start, view.end - page.start));
   } else {
    if (typeof root.ClipboardItem !== 'function' || typeof clipboard.write !== 'function') {
     throw Error('This browser cannot copy a selection spanning pages.');
    }
    const transfer = new AbortController();
    const cancel = () => transfer.abort();
    abort.signal.addEventListener('abort', cancel, {once: true});
    const blob = session.readBlob(view.start, view.end, transfer.signal);
    // A browser can refuse the clipboard before it consumes the promised data.
    void blob.catch(() => {});
    // Start the clipboard write during the user gesture; resolve its data as
    // chunks arrive. The browser, not a JS document string, holds the payload.
    try { await clipboard.write([new root.ClipboardItem({'text/plain': blob})]); }
    finally { transfer.abort(); abort.signal.removeEventListener('abort', cancel); }
   }
  } finally {
   if (cut) {
    editing = false;
    if (!disposed) updateReadOnly();
   }
  }
  if (cut && !disposed) {
   const now = capture();
   if (now.start !== view.start || now.end !== view.end || session.state().editVersion !== stamp) {
    throw Error('The selection changed. Text was copied without deleting it.');
   }
   await replace('', view);
  }
  return true;
 }

 function handleKey(event) {
  if (event.isComposing || composing) return;
  const control = event.ctrlKey || event.metaKey;
  const key = event.key;
  const command = key.toLowerCase();
  if (control && (key === 'PageUp' || key === 'PageDown')) return;
  const stop = () => { event.preventDefault(); event.stopImmediatePropagation(); };
  if (control && command === 'a') {
   stop();
   select(0, session.state().metadata.units, 'none', true);
   return;
  }
  if (control && (command === 'z' || command === 'y')) {
   stop();
   if (!readOnly) run(history(command === 'y' || event.shiftKey));
   return;
  }
  const at = textarea.selectionDirection === 'backward' ? textarea.selectionStart : textarea.selectionEnd;
  const row = localLine(at);
  const before = page.start > 0;
  const after = page.start + page.text.length < session.state().metadata.units;
  const left = at === 0 && before;
  const right = at === page.text.length && after;
  if (!readOnly && (key === 'Backspace' && left || key === 'Delete' && right)) {
   stop();
   run(erase(key === 'Backspace'));
   return;
  }
  const atBoundary = {
   ArrowLeft: left, ArrowRight: right,
   ArrowUp: row === 0 && before, ArrowDown: row === starts.length - 1 && after,
   PageUp: row < clientRows && before, PageDown: row >= starts.length - clientRows && after,
   Home: (control || row === 0 && page.column > 0), End: (control || row === starts.length - 1 && after)
  };
  if (Object.hasOwn(atBoundary, key) && (virtual || atBoundary[key])) {
   stop();
   run(move(key, event.shiftKey, control));
  }
 }

 textarea.addEventListener('pointerdown', event => {
  pointer = true;
  if (!event.shiftKey) virtual = false;
 }, listeners);
 root.addEventListener('pointerup', () => {
  if (!pointer) return;
  pointer = false;
  schedulePaint();
 }, listeners);
 root.addEventListener('pointercancel', () => { pointer = false; }, listeners);
 textarea.addEventListener('select', schedulePaint, listeners);
 textarea.addEventListener('keyup', schedulePaint, listeners);
 textarea.addEventListener('compositionstart', () => {
  composing = true;
  navigation++;
  navigating = false;
  session.cancelRead?.();
 }, listeners);
 textarea.addEventListener('compositionend', () => {
  composing = false;
  try { changedText(); } catch (error) { report(error); }
 }, listeners);
 textarea.addEventListener('input', event => {
  event.stopImmediatePropagation();
  if (!composing) {
   try { changedText(); } catch (error) { report(error); }
  }
  schedulePaint();
 }, listeners);
 textarea.addEventListener('beforeinput', event => {
  if (readOnly || editing || session.state().blocked) {
   event.preventDefault();
   return;
  }
  const type = event.inputType;
  if (type === 'historyUndo' || type === 'historyRedo') {
   event.preventDefault();
   event.stopImmediatePropagation();
   run(history(type === 'historyRedo'));
   return;
  }
  if (!virtual) return;
  event.preventDefault();
  event.stopImmediatePropagation();
  if (event.isComposing) {
   message('Place the cursor in the document before composing text.', true);
  } else if (type.startsWith('delete') && type !== 'deleteByCut') {
   run(erase(type.endsWith('Backward')));
  } else if (type === 'insertText' || type === 'insertReplacementText') {
   run(replace(event.data || ''));
  } else if (type === 'insertLineBreak' || type === 'insertParagraph') {
   run(replace('\n'));
  }
 }, listeners);
 textarea.addEventListener('paste', event => {
  event.preventDefault();
  event.stopImmediatePropagation();
  const data = event.clipboardData;
  if (!readOnly && data && Array.from(data.types).includes('text/plain')) run(replace(data.getData('text/plain')));
 }, listeners);
 textarea.addEventListener('drop', event => {
  if (!event.dataTransfer || Array.from(event.dataTransfer.types).includes('Files')) return;
  if (!Array.from(event.dataTransfer.types).includes('text/plain')) return;
  event.preventDefault();
  event.stopImmediatePropagation();
  if (!readOnly) run(replace(event.dataTransfer.getData('text/plain')));
 }, listeners);
 for (const type of ['copy', 'cut']) textarea.addEventListener(type, event => {
  if (!virtual) return;
  event.preventDefault();
  event.stopImmediatePropagation();
  run(copy(type === 'cut'));
 }, listeners);
 textarea.addEventListener('keydown', handleKey, listeners);
 textarea.addEventListener('scroll', scroll, listeners);
 textarea.addEventListener('wheel', wheel, {...listeners, passive: false});

 vertical.addEventListener('scroll', () => {
  if (Math.abs(vertical.scrollTop - barTop) <= 1 || verticalFrame) return;
  verticalFrame = root.requestAnimationFrame(() => {
   verticalFrame = 0;
   if (disposed || Math.abs(vertical.scrollTop - barTop) <= 1) return;
   const last = Math.max(0, session.state().metadata.lines - clientRows);
   const line = Math.round(vertical.scrollTop / Math.max(1, RANGE - vertical.clientHeight) * last);
   run(goLine(line));
  });
 }, listeners);
 vertical.addEventListener('keydown', event => {
  const count = session.state().metadata.lines;
  const targets = {
   Home: 0, End: count - 1, ArrowUp: top - 1, ArrowDown: top + 1,
   PageUp: top - clientRows, PageDown: top + clientRows
  };
  if (!Object.hasOwn(targets, event.key)) return;
  event.preventDefault();
  run(goLine(targets[event.key]));
 }, listeners);
 horizontal.addEventListener('scroll', () => {
  if (Math.abs(horizontal.scrollLeft - barLeft) <= 1 || horizontalFrame) return;
  horizontalFrame = root.requestAnimationFrame(() => {
   horizontalFrame = 0;
   if (disposed || Math.abs(horizontal.scrollLeft - barLeft) <= 1) return;
   const last = Math.max(0, page.lineUnits - clientColumns);
   run(panLongLine(Math.round(horizontal.scrollLeft / Math.max(1, RANGE - horizontal.clientWidth) * last)));
  });
 }, listeners);
 horizontal.addEventListener('keydown', event => {
  const column = Math.round(page.column + textarea.scrollLeft / characterWidth);
  const targets = {
   Home: 0, End: page.lineUnits, ArrowLeft: column - 1, ArrowRight: column + 1,
   PageUp: column - clientColumns, PageDown: column + clientColumns
  };
  if (!Object.hasOwn(targets, event.key)) return;
  event.preventDefault();
  run(panLongLine(targets[event.key]));
 }, listeners);

 const resize = typeof root.ResizeObserver === 'function'
  ? new root.ResizeObserver(() => { metrics(); schedulePaint(); }) : null;
 resize?.observe(textarea);
 metrics();
 schedulePaint();

 return Object.freeze({
  load, goLine, position, restore, replace, history, capture, select, selectedText, copy,
  selectAll() { select(0, session.state().metadata.units, 'none', true); },
  setReadOnly(value) { readOnly = !!value; updateReadOnly(); },
  refresh() {
   const value = session.state().page;
   if (value.start !== page.start || value.text !== page.text || value.line !== page.line ||
       value.lineUnits !== page.lineUnits) install(value);
   else updateReadOnly();
   metrics();
   schedulePaint();
  },
  selection: capture,
  get composing() { return composing; },
  get cursor() {
   const offset = clamp(cursorOffset(capture()) - page.start, 0, page.text.length);
   const row = localLine(offset);
   return {line: page.line + row, column: offset - starts[row] + (row === 0 ? page.column : 0)};
  },
  dispose() {
   disposed = true;
   navigation++;
   session.cancelRead?.();
   abort.abort();
   resize?.disconnect();
   for (const job of [frame, verticalFrame, horizontalFrame]) if (job) root.cancelAnimationFrame(job);
   vertical.remove();
   horizontal.remove();
   textarea.readOnly = original.readOnly;
   textarea.wrap = original.wrap;
   gutter.parentElement.style.width = original.width;
   gutter.parentElement.style.flexBasis = original.basis;
   gutter.style.paddingTop = original.padding;
   gutter.style.transform = original.transform;
   container.classList.remove('nt-editor-disk', 'nt-editor-disk-long-line');
  }
 });
}
root.NTKEditorDiskView = Object.freeze({create});
if (typeof module === 'object' && module.exports) module.exports = {create};
})(globalThis);
