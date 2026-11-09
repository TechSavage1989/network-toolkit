/* A native text control for one bounded document page. Selections and the
   scroll map use document offsets even when they extend outside that page. */
(function (root) {
'use strict';

const RANGE = 1000000;
const BUFFER_LINES = 40;
const PREVIEW_UNITS = 512;
const clamp = (value, minimum, maximum) => Math.min(maximum, Math.max(minimum, value));

function create({textarea, gutter, container, session, syntaxLanguage, changed = () => {}, message = () => {}, readOnly = false}) {
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
 const syntax = syntaxLanguage ? document.createElement('pre') : null;
 const syntaxChannel = textarea.dataset.compareText ? 'compare-' + textarea.dataset.compareText : 'default';
 let syntaxWindow, syntaxTicket = 0, syntaxTimer;
 if (syntax) {
  syntax.className = 'nt-editor-highlight nt-editor-disk-highlight';
  syntax.hidden = true;
  syntax.setAttribute('aria-hidden', 'true');
  textarea.before(syntax);
 }
 container.classList.add('nt-editor-disk');
 textarea.wrap = 'off';

 let page = session.state().page;
 let starts = lineStarts(page.text);
 let alignment = null, projection = null, alignmentTicket = 0;
 let selection = {start: page.start, end: page.start, direction: 'none'};
 let virtual = false, composing = false, disposed = false, editing = false;
 let navigation = 0, navigating = false, frame = 0, pointer = false;
 let top = page.line, barTop = 0, barLeft = 0, lastScrollTop = 0, lastScrollLeft = 0;
 let gutterStamp = '', lastAnnouncement = '', lastPrefetch = '';
 let fraction = 0;
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

 function localLine(offset, offsets = starts) {
  let low = 0, high = offsets.length;
  while (low + 1 < high) {
   const mid = Math.floor((low + high) / 2);
   if (offsets[mid] <= offset) low = mid;
   else high = mid;
  }
  return low;
 }

 function displayOffset(offset) {
  if (!projection) return offset;
  const line = localLine(offset);
  return projection.offsets[projection.sourceRows[line]] + offset - starts[line];
 }

 function sourceOffset(offset) {
  if (!projection) return offset;
  const row = localLine(offset, projection.offsets), value = projection.rows[row];
  if (value.gap) return starts[value.source] ?? page.text.length;
  return starts[value.source] + offset - projection.offsets[row];
 }

 function displayRow() {
  return clamp(Math.floor(textarea.scrollTop / rowHeight), 0, (projection?.rows.length || starts.length) - 1);
 }

 function installProjection(layout, around) {
  const gaps = new Map(layout.gaps.map(gap => [gap.line - page.line, gap.count]));
  const rows = [], sourceRows = [], parts = [];
  let row = layout.row;
  const addGaps = source => {
   const count = gaps.get(source) || 0;
   const size = Math.min(count, Math.max(128, clientRows * 3));
   const first = clamp((around ?? row) - row - clientRows, 0, count - size);
   for (let i = first; i < first + size; i++) rows.push({source, row:row+i, gap:true});
   parts.push('\n'.repeat(size));
   row += count;
  };
  for (let source = 0; source < starts.length; source++) {
   addGaps(source);
   sourceRows.push(rows.length);
   rows.push({source,row:row++});
   parts.push(page.text.slice(starts[source], starts[source+1]));
  }
  addGaps(starts.length);
  const text = parts.join('');
  projection = {rows, sourceRows, offsets:lineStarts(text), text};
  gutterStamp = '';
  if (textarea.value !== text) textarea.value = text;
 }

 async function project(active, around) {
  if (!alignment || page.column) return;
  const current = alignment, source = page;
  const complete = page.start + page.text.length === session.state().metadata.units;
  const end = page.line + starts.length - (complete ? 0 : 1);
  const layout = await current.layout(page.line, end);
  if (!active() || disposed || composing || alignment !== current || page !== source) return;
  const view = capture();
  installProjection(layout, around);
  select(view.start, view.end, view.direction);
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
    start: page.start + sourceOffset(textarea.selectionStart),
    end: page.start + sourceOffset(textarea.selectionEnd),
    direction: textarea.selectionDirection || 'none'
   };
  }
  return {...selection, top, left: textarea.scrollLeft + page.column * characterWidth, fraction,
   alignedTop:projection?.rows[displayRow()]?.row};
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
   displayOffset(clamp(start - page.start, 0, page.text.length)),
   displayOffset(clamp(end - page.start, 0, page.text.length)),
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
  const {lines, units} = session.state().metadata;
  const row = displayRow(), projected = projection?.rows[row];
  const first = Math.max(0, row - 3);
  const last = Math.min(projection?.rows.length || starts.length, first + clientRows + 8);
  const stamp = [page.line, first, last, rowHeight, String(lines).length, alignmentTicket].join(':');
  if (stamp !== gutterStamp) {
   gutterStamp = stamp;
   const rows = [];
   for (let line = first; line < last; line++) {
    const element = document.createElement('div');
    element.style.height = rowHeight + 'px';
    element.style.lineHeight = rowHeight + 'px';
    const value = projection?.rows[line];
    if (value?.gap) element.dataset.alignmentGap = '';
    else element.textContent = String(page.line + (value?.source ?? line) + 1);
    rows.push(element);
   }
   gutter.style.paddingTop = first * rowHeight + 'px';
   gutter.replaceChildren(...rows);
   const width = String(lines).length + 3 + 'ch';
   gutter.parentElement.style.width = width;
   gutter.parentElement.style.flexBasis = width;
  }
  gutter.style.transform = 'translateY(' + (-textarea.scrollTop) + 'px)';
  paintSyntax();
  top = Math.min(lines - 1, page.line + (projected?.source ?? row));
  fraction = clamp(textarea.scrollTop / rowHeight - row, 0, 0.999);
  const scrollLines = alignment?.rows || lines, scrollTop = projected?.row ?? top;
  vertical.hidden = scrollLines <= clientRows;
  const lastTop = Math.max(1, scrollLines - clientRows);
  barTop = clamp(scrollTop / lastTop, 0, 1) * (RANGE - vertical.clientHeight);
  if (Math.abs(vertical.scrollTop - barTop) > 1) vertical.scrollTop = barTop;
  scrollbarValue(vertical, scrollLines, scrollTop + 1);
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
  if (page.start + page.text.length < units) {
   const origin = nextPageOrigin();
   if (origin > page.line) prefetch({line: origin});
  }
  lastScrollTop = textarea.scrollTop;
  lastScrollLeft = textarea.scrollLeft;
  announce();
 }

 function paintSyntax() {
  if (!syntax) return;
  const text = textarea.value, next = syntaxLanguage(text);
  const offsets = projection?.offsets || starts;
  const top = displayRow(), bottom = Math.min(offsets.length, top + clientRows + 2);
  const position = first => {
   syntax.style.transform = 'translate(' + (-textarea.scrollLeft) + 'px,' +
    (first * rowHeight - textarea.scrollTop) + 'px)';
  };
  if (syntaxWindow?.text === text && syntaxWindow.language === next &&
      syntaxWindow.first <= top && syntaxWindow.end >= bottom) {
   position(syntaxWindow.first);
   return;
  }
  const first = Math.max(0, top - 8), end = Math.min(offsets.length, bottom + 12);
  syntaxWindow = {text, language:next, first, end};
  position(first);
  const ticket = ++syntaxTicket, service = root.NTKEditorLanguageService;
  root.clearTimeout(syntaxTimer);
  syntax.hidden = true;
  textarea.classList.remove('highlight-active');
  if (!next || next === 'text' || !text || !service) return;
  const visible = text.slice(offsets[first], offsets[end]);
  syntaxTimer = root.setTimeout(() => void service.highlight(visible, next, {channel:syntaxChannel}).then(value => {
   if (disposed || ticket !== syntaxTicket || textarea.value !== text || syntaxLanguage(text) !== next) return;
   if (value.limited) return;
   syntax.innerHTML = value.html + '\n';
   syntax.hidden = false;
   textarea.classList.add('highlight-active');
  }).catch(() => {}), 140);
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
  projection = null;
  alignmentTicket++;
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
  const {alignedRow, ...source} = target;
  const result = await session.load(source);
  if (!result || !active()) return false;
  install(result);
  await project(active, alignedRow);
  return true;
 }

 function showLine(line) {
  const source = Math.max(0, Math.floor(line - page.line));
  const row = projection?.sourceRows[source] ?? source;
  textarea.scrollTop = (row + (line % 1)) * rowHeight;
  paint();
 }

 function showAligned(row, fraction) {
  const local = projection?.rows.findIndex(value => value.row === row) ?? -1;
  if (local < 0) return false;
  textarea.scrollTop = (local + fraction) * rowHeight;
  paint();
  return true;
 }

 async function goAlignedRow(row, fraction = 0) {
  if (!alignment) return false;
  const current = alignment;
  row = clamp(Math.floor(row), 0, current.rows - 1);
  const local = projection?.rows.findIndex(value => value.row === row) ?? -1;
  if (local >= 0 && (local + clientRows < projection.rows.length ||
      projection.rows.at(-1).row === current.rows - 1)) return showAligned(row, fraction);
  return navigate(async active => {
   const target = await current.source(row);
   if (!active() || current !== alignment) return false;
   const count = session.state().metadata.lines;
   const line = Math.min(count - 1, target.line);
   const origin = Math.max(0, line - Math.min(BUFFER_LINES, clientRows));
   if (!await loadPage({line:origin, alignedRow:row}, active)) return false;
   if (showAligned(row, fraction) && displayRow() >= (projection?.rows.findIndex(value => value.row === row) ?? 0)) return true;
   if (origin !== line && await loadPage({line, alignedRow:row}, active)) return showAligned(row, fraction);
   return false;
  });
 }

 async function setAlignment(value) {
  if (alignment === value || disposed) return;
  if (composing) {
   if (!value) { alignment = null; alignmentTicket++; }
   return;
  }
  const saved = capture();
  alignment = value;
  alignmentTicket++;
  if (!value) {
   projection = null;
   textarea.value = page.text;
   select(saved.start, saved.end, saved.direction);
   showLine(saved.top + saved.fraction);
   return;
  }
  const ticket = alignmentTicket;
  await project(() => ticket === alignmentTicket);
  if (ticket === alignmentTicket) showLine(saved.top + saved.fraction);
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
  if (line - page.line + clientRows >= starts.length &&
      page.start + page.text.length < session.state().metadata.units) {
   return loadPage({line}, active);
  }
  return true;
 }

 async function goLine(line, focus = false, force = false, fraction = 0) {
  line = clamp(Math.floor(line), 0, session.state().metadata.lines - 1);
  return navigate(async active => {
   if (!await openLine(line, active, force)) return false;
   if (focus) select(page.start + starts[line - page.line], undefined, 'none', true);
   showLine(line + fraction);
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
   showLine(line + clamp(view.fraction || 0, 0, 0.999));
   return true;
  });
 }

 async function applyEdit(operation, offset, savedView) {
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
  if (savedView) await restore(savedView);
  else await position(at);
  return true;
 }

 function replace(text, range = capture()) {
  const value = String(text).replace(/\r\n|\r/g, '\n');
  return applyEdit(() => session.replace(range.start, range.end, value), range.start + value.length);
 }

 function history(redo = false) {
  const view = capture();
  return applyEdit(() => session.history(redo), view.start, view);
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

 function rollForward(line, origin, offset) {
  return navigate(async active => {
   if (!await loadPage({line: origin}, active)) return false;
   if (line >= page.line + starts.length && !await openLine(line, active)) return false;
   showLine(line + offset);
   return true;
  });
 }

 function nextPageOrigin() {
  const spare = starts.length - clientRows;
  return page.line + Math.max(0, spare - Math.min(BUFFER_LINES, Math.floor(spare / 2)));
 }

 function scroll() {
  schedulePaint();
  if (composing || navigating || editing || pointer) return;
  const {units, lines} = session.state().metadata;
  const display = displayRow(), projected = projection?.rows[display];
  const row = projected?.source ?? display;
  const line = page.line + row;
  const movedDown = textarea.scrollTop > lastScrollTop;
  const movedUp = textarea.scrollTop < lastScrollTop;
  if (projected) {
   const previous = clamp(Math.floor(lastScrollTop / rowHeight), 0, projection.rows.length - 1);
   const expected = projection.rows[previous].row + display - previous;
   if (expected !== projected.row) {
    run(goAlignedRow(expected, textarea.scrollTop / rowHeight - display));
    return;
   }
   if (movedDown && display + clientRows >= projection.rows.length - 2 && projected.row + clientRows < alignment.rows) {
    run(goAlignedRow(projected.row + 1));
    return;
   }
  }
  if (movedUp && row < BUFFER_LINES / 2 && page.line > 0) {
   const target = Math.max(0, line - BUFFER_LINES);
   if (target < page.line) run(goLine(line, false, true));
  } else if (movedDown && page.start + page.text.length < units) {
   const remaining = starts.length - row - clientRows;
   const origin = nextPageOrigin();
   if (origin > page.line) {
    if (row >= origin - page.line && remaining < BUFFER_LINES / 2 && !projected?.gap) run(rollForward(line, origin, Math.max(0, textarea.scrollTop / rowHeight - display)));
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
  if (projection && (delta < 0 && row <= 0 || delta > 0 && bottom)) {
   const currentRow = projection.rows[displayRow()].row;
   const next = clamp(currentRow + (delta < 0 ? -3 : 3), 0, Math.max(0, alignment.rows - clientRows));
   if (next !== currentRow) {
    event.preventDefault();
    run(goAlignedRow(next));
   }
   return;
  }
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
  if (projection) {
   // Some native input events cannot be cancelled. Never send display rows
   // to the source buffer even if the browser has already changed the control.
   if (text !== projection.text) {
    textarea.value = projection.text;
    select(selection.start, selection.end, selection.direction);
   }
   return;
  }
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

 function editProjected(text) {
  const view = capture();
  const from = view.start - page.start, to = view.end - page.start;
  if (from < 0 || to > page.text.length) { run(replace(text, view)); return; }
  const value = page.text.slice(0, from) + text + page.text.slice(to);
  projection = null;
  alignmentTicket++;
  textarea.value = value;
  textarea.setSelectionRange(from + text.length, from + text.length);
  changedText();
  showOffset(page.start + from + text.length);
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
  const at = sourceOffset(textarea.selectionDirection === 'backward' ? textarea.selectionStart : textarea.selectionEnd);
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
  if (projection) {
   const view = capture();
   projection = null;
   alignmentTicket++;
   textarea.value = page.text;
   select(view.start, view.end, view.direction);
   showLine(view.top + view.fraction);
  }
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
  if (!virtual && !projection) return;
  event.preventDefault();
  event.stopImmediatePropagation();
  if (event.isComposing) {
   message('Place the cursor in the document before composing text.', true);
  } else if (type.startsWith('delete') && type !== 'deleteByCut') {
   run(erase(type.endsWith('Backward')));
  } else if (type === 'insertText' || type === 'insertReplacementText') {
   if (projection) editProjected(event.data || '');
   else run(replace(event.data || ''));
  } else if (type === 'insertLineBreak' || type === 'insertParagraph') {
   if (projection) editProjected('\n');
   else run(replace('\n'));
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
  if (!virtual && !projection) return;
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
   const last = Math.max(0, (alignment?.rows || session.state().metadata.lines) - clientRows);
   const line = Math.round(vertical.scrollTop / Math.max(1, RANGE - vertical.clientHeight) * last);
   run(alignment ? goAlignedRow(line) : goLine(line));
  });
 }, listeners);
 vertical.addEventListener('keydown', event => {
  const count = alignment?.rows || session.state().metadata.lines;
  const position = projection?.rows[displayRow()].row ?? top;
  const targets = {
   Home: 0, End: count - 1, ArrowUp: position - 1, ArrowDown: position + 1,
   PageUp: position - clientRows, PageDown: position + clientRows
  };
  if (!Object.hasOwn(targets, event.key)) return;
  event.preventDefault();
  run(alignment ? goAlignedRow(targets[event.key]) : goLine(targets[event.key]));
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
  load, goLine, goAlignedRow, setAlignment, position, restore, replace, history, capture, select, selectedText, copy,
  comparisonPage: () => ({line:page.line, text:page.text, column:page.column, complete:page.start+page.text.length>=session.state().metadata.units}),
  refreshSyntax: paintSyntax,
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
   if (syntax) {
    root.clearTimeout(syntaxTimer);
    root.NTKEditorLanguageService?.cancelHighlight?.(syntaxChannel);
    syntax.remove();
    textarea.classList.remove('highlight-active');
   }
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
