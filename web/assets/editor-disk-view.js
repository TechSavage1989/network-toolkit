/* A native text control for one bounded document page. Selections and the
   scroll map use document offsets even when they extend outside that page.

   Word wrap ("flow" below, when no comparison alignment is active): the page
   is a window that slides through the document as the reader scrolls, keeping
   text above and below the view. The vertical bar maps to a character offset
   in the whole document instead of a line number, because one line can hold
   most of a file (a Base64 export is a single line) and counting the wrapped
   rows of a huge file exactly would mean laying out all of it. The thumb size
   uses an estimate of the wrapped rows (lines plus characters per row); a
   document that fits in one page maps exactly. Moving the window never moves
   the text the reader sees: the visual row at the top of the view is put back
   at the same height. A page moving forward starts at a visual row start of
   the current page, so every following row wraps the same way; scrolling back
   returns to the earlier pages of that chain (`trail`); any other page inside
   a long line starts on the wrap grid (see startBefore). */
(function (root) {
'use strict';

// Chromium 151 (fixed by 153): once a click on a button has taken the focus
// from a textarea, the page's selection stays at that textarea, and each later
// click on a button puts back the selection the textarea had then, undoing
// one set by script since (Find Next's match before Replace). Dropping that
// stale page selection keeps the one set here (as in file-editor.js).
function keepScriptSelection(ta) {
 const doc = ta?.ownerDocument;
 if (!doc || doc.activeElement === ta) return;
 const s = doc.getSelection?.();
 if (!s?.rangeCount) return;
 const at = (node, offset) => node === ta || node === ta.parentNode && (node.childNodes[offset] === ta || node.childNodes[offset - 1] === ta);
 if (at(s.anchorNode, s.anchorOffset) || at(s.focusNode, s.focusOffset)) s.removeAllRanges();
}

const RANGE = 1000000;
const BUFFER_LINES = 40;
const PREVIEW_UNITS = 512;
const clamp = (value, minimum, maximum) => Math.min(maximum, Math.max(minimum, value));
const highSurrogate = code => code >= 0xd800 && code <= 0xdbff;
const lowSurrogate = code => code >= 0xdc00 && code <= 0xdfff;

function measurementStyle(textarea) {
 const style = root.getComputedStyle(textarea);
 const width = Math.max(1, textarea.clientWidth - parseFloat(style.paddingLeft || 0) - parseFloat(style.paddingRight || 0));
 return {width, font:style.font, height:parseFloat(style.lineHeight) || 20, letterSpacing:style.letterSpacing, tabSize:style.tabSize};
}

function createMeasure(container) {
 const measure = document.createElement('div');
 measure.className = 'nt-editor-wrap-measure';
 measure.setAttribute('aria-hidden', 'true');
 container.append(measure);
 return measure;
}

function layoutRows(measure, text, {width, height, letterSpacing, tabSize}, previous) {
 Object.assign(measure.style, {width:width + 'px', font:'inherit', letterSpacing, tabSize});
 const lines = text.split('\n');
 const oldLines = previous?.lines || [];
 let first = 0, end = lines.length, oldEnd = oldLines.length;
 while (first < end && first < oldEnd && lines[first] === oldLines[first]) first++;
 while (end > first && oldEnd > first && lines[end - 1] === oldLines[oldEnd - 1]) {
  end--;
  oldEnd--;
 }
 const changed = lines.slice(first, end).map(line => {
  const element = document.createElement('div');
  element.textContent = line || '\u200b';
  return element;
 });
 const oldRows = previous?.rows || [];
 for (let i = first; i < oldEnd; i++) oldRows[i].remove();
 if (changed.length) {
  const fragment = document.createDocumentFragment();
  for (const element of changed) fragment.append(element);
  measure.insertBefore(fragment, oldRows[oldEnd] || null);
 }
 const rows = [...oldRows.slice(0, first), ...changed, ...oldRows.slice(oldEnd)];
 const oldHeights = previous?.heights || [];
 const heights = [...oldHeights.slice(0, first),
  ...changed.map(element => element.getBoundingClientRect().height || height), ...oldHeights.slice(oldEnd)];
 const counts = heights.map(value => Math.max(1, Math.round(value / height)));
 return {counts, heights, rows, lines};
}

function measureWrapRows(text, textarea) {
 const measure = createMeasure(textarea.parentElement);
 try { return layoutRows(measure, text, measurementStyle(textarea)).counts; }
 finally { measure.remove(); }
}

function create({textarea, gutter, container, session, syntaxLanguage, wordWrap = () => false,
 changed = () => {}, message = () => {}, readOnly = false, locked = () => false, marks = null, bookmarks = null, painted = () => {}}) {
 const pageUnits = root.NTKEditorDiskSession.PAGE_UNITS;
 const abort = new AbortController();
 const listeners = {signal: abort.signal};
 const original = {
  readOnly: textarea.readOnly,
  wrap: textarea.wrap,
  wrapped: textarea.classList.contains('is-wrapped'),
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
 // Marks of the Find window (editor-find.js): marks(from, to) gives the
 // marked ranges of that part of the document, or null until it knows them;
 // bookmarks() the bookmarked lines.
 const markLayer = marks ? document.createElement('div') : null;
 let markStamp = 0, markKey = '', markRanges = null;
 if (markLayer) {
  markLayer.className = 'nt-editor-marks nt-editor-disk-highlight';
  markLayer.hidden = true;
  markLayer.setAttribute('aria-hidden', 'true');
  (syntax || textarea).before(markLayer);
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
 let fraction = 0, alignedTop;
 let rowHeight = 20, characterWidth = 8, clientRows = 1, clientColumns = 1, font = '';
 let wrapped = false, wrapStyle = '', measure, measured, wrapGeometry, gutterGeometry;
 let wrapFrame = 0, wrapRead;
 let verticalFrame = 0, horizontalFrame = 0;
 // Where the view last put each bar. A bar that is elsewhere was moved by the
 // user (a click on the track, a drag) and its scroll event is still on its
 // way: painting must not move it back, or the user's move is lost (CI caught
 // a track click undone by a paint that ran before the bar's scroll event).
 let barSet = 0, barLeftSet = 0;
 const barMoved = () => Math.abs(vertical.scrollTop - barSet) > 1;
 const barLeftMoved = () => Math.abs(horizontal.scrollLeft - barLeftSet) > 1;
 // A bar that is or was just hidden has no position the user chose (the
 // browser may reset it while hidden): take it as the view's own.
 function showBar(bar, hidden) {
  const was = bar.hidden;
  bar.hidden = hidden;
  if (was || hidden) {
   if (bar === vertical) barSet = vertical.scrollTop;
   else barLeftSet = horizontal.scrollLeft;
  }
 }
 function placeBar(moved) {
  if (!verticalFrame && !moved && Math.abs(vertical.scrollTop - barTop) > 1) vertical.scrollTop = barTop;
  if (!moved) barSet = vertical.scrollTop;
 }
 // A pointer selection in progress: its anchor may leave the page while the
 // page slides under the pointer.
 let drag = null, pressed = null, clicks = 0;
 // Word-wrap flow state: the last painted view position, a slide that could
 // not make progress, scrolling that waits for text, and the wrap grid.
 let flowView = null, flowStuck = '', pendingScroll = 0, flowFrame = 0, grid = null, padding = 30;
 // Earlier page starts that the current page continues, in order: each page
 // started at a visual row start of the one before, so they wrap a long line
 // the same way and scrolling back to one of them changes no row.
 let trail = [];
 const recenter = Math.floor(pageUnits * 0.4);
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
  return Math.min(page.text.length, starts[value.source] + offset - projection.offsets[row]);
 }

 // Only the loaded page is laid out. Logical positions stay in the source
 // document, while the native textarea handles caret movement inside the page.
 function measureText(text) {
  if (!measure) measure = createMeasure(container);
  const style = measurementStyle(textarea);
  const key = Object.values(style).join(':');
  if (measured?.text === text && measured.key === key) return measured;
  const previous = measured?.key === key ? measured : null;
  if (!previous) measure.replaceChildren();
  measured = {text, key, ...layoutRows(measure, text, style, previous)};
  return measured;
 }

 function measureCurrentWrapRows(text) {
  return measureWrapRows(text, textarea);
 }

 function ensureWrapGeometry() {
  if (!wrapped) return null;
  const text = textarea.value;
  if (wrapGeometry?.text === text && wrapGeometry.style === wrapStyle) return wrapGeometry.pixels;
  const {heights} = measureText(text), pixels = new Float64Array(heights.length + 1);
  for (let i = 0; i < heights.length; i++) pixels[i + 1] = pixels[i] + heights[i];
  wrapGeometry = {text, style:wrapStyle, pixels};
  return pixels;
 }

 function pixelAt(row) {
  const pixels = ensureWrapGeometry();
  return pixels?.[row] ?? row * rowHeight;
 }

 function rowAt(pixel) {
  const count = projection?.rows.length || starts.length;
  const pixels = ensureWrapGeometry();
  const row = pixels ? localLine(Math.max(0, pixel), pixels) : Math.floor(pixel / rowHeight);
  return clamp(row, 0, count - 1);
 }

 function displayRow() { return rowAt(textarea.scrollTop); }

 function groupStart(row) { return projection?.rows[row]?.wrapHead ?? row; }
 function groupEnd(row) { return projection?.rows[row]?.wrapEnd ?? row + 1; }
 function rowFraction(row) {
  const start = groupStart(row);
  return clamp((textarea.scrollTop - pixelAt(start)) / (pixelAt(groupEnd(start)) - pixelAt(start)), 0, 0.999);
 }

 function rowScrollOffset(row, fraction) {
  row = groupStart(row);
  const pixel = pixelAt(row);
  return pixel + fraction * (pixelAt(groupEnd(row)) - pixel);
 }

 function scrollToRow(row, fraction = 0) {
  textarea.scrollTop = rowScrollOffset(row, fraction);
 }

 // Word-wrap flow: visual rows of the page, found from character boxes in the
 // hidden measure, which wraps the same text with the textarea's width, font
 // and wrapping rules. Offsets here are page-local (no projection in flow).
 function flowing() { return wrapped && !alignment; }

 function charBox(data, line, column) {
  const element = data.rows[line], node = element?.firstChild, text = data.lines[line] || '';
  if (!node || !text.length) return null;
  let at = clamp(column, 0, text.length - 1);
  if (at > 0 && lowSurrogate(text.charCodeAt(at)) && highSurrogate(text.charCodeAt(at - 1))) at--;
  const range = document.createRange();
  range.setStart(node, at);
  range.setEnd(node, Math.min(text.length, at + (highSurrogate(text.charCodeAt(at)) ? 2 : 1)));
  const box = range.getBoundingClientRect(), base = element.getBoundingClientRect();
  return {row: Math.max(0, Math.round((box.top - base.top) / rowHeight)), left: box.left - base.left, right: box.right - base.left};
 }

 // The first column of visual row `row` in display line `line`.
 function rowStartColumn(data, line, row) {
  const text = data.lines[line] || '';
  if (row <= 0 || !text.length) return 0;
  let low = 0, high = text.length;
  while (low < high) {
   const mid = (low + high) >> 1;
   if (charBox(data, line, mid).row >= row) high = mid;
   else low = mid + 1;
  }
  return low;
 }

 function describeRow(data, line, row) {
  const text = data.lines[line] || '', count = data.counts[line] || 1;
  row = clamp(row, 0, count - 1);
  const start = rowStartColumn(data, line, row);
  // The line break belongs to the last row of its line.
  const end = row + 1 < count ? Math.max(start, rowStartColumn(data, line, row + 1)) : text.length + 1;
  return {line, row, last: row + 1 >= count, top: pixelAt(line) + row * rowHeight,
   start: starts[line] + start, end: starts[line] + end};
 }

 function visualRow(data, y) {
  const line = rowAt(y);
  return describeRow(data, line, Math.floor((y - pixelAt(line)) / rowHeight + 1e-6));
 }

 function rowOf(data, offset) {
  offset = clamp(offset, 0, page.text.length);
  const line = localLine(offset), text = data.lines[line] || '';
  const column = offset - starts[line];
  const box = text.length ? charBox(data, line, Math.min(column, text.length - 1)) : null;
  return describeRow(data, line, box?.row || 0);
 }

 function caretX(data, offset) {
  const line = localLine(offset), text = data.lines[line] || '', column = offset - starts[line];
  if (!text.length) return 0;
  return column < text.length ? charBox(data, line, column).left : charBox(data, line, text.length - 1).right;
 }

 // The page offset nearest to x on the visual row at y.
 function offsetAtPoint(data, x, y) {
  const row = visualRow(data, y), text = data.lines[row.line] || '';
  const from = row.start - starts[row.line], to = Math.min(text.length, row.end - starts[row.line]);
  let low = from, high = to;
  while (low < high) {
   const mid = (low + high) >> 1, box = charBox(data, row.line, mid);
   if ((box.left + box.right) / 2 > x) high = mid;
   else low = mid + 1;
  }
  // The end of a wrapped row is the start of the next one; stay on this row.
  if (!row.last && low >= to && low > from) low = to - 1;
  return starts[row.line] + low;
 }

 function viewHeight() { return Math.max(rowHeight, textarea.clientHeight - padding); }

 // Characters per row for text without break opportunities, measured once per
 // width and font. Long-line pages start on this grid (see startBefore).
 function wrapGrid() {
  const style = measurementStyle(textarea), key = Object.values(style).join(':');
  if (grid?.key === key) return grid.columns;
  const probe = createMeasure(container);
  try {
   Object.assign(probe.style, {width:style.width + 'px', font:'inherit', letterSpacing:style.letterSpacing, tabSize:style.tabSize});
   const element = document.createElement('div');
   element.textContent = '0'.repeat(Math.ceil(style.width / Math.max(1, characterWidth)) * 2 + 8);
   probe.append(element);
   const data = {rows:[element], lines:[element.textContent]};
   const columns = rowStartColumn(data, 0, 1);
   grid = {key, columns: Math.max(1, columns || element.textContent.length)};
  } finally { probe.remove(); }
  return grid.columns;
 }

 // The position at the top of the view as a fractional document offset, and
 // how many characters the view shows.
 function flowPosition() {
  const data = measureText(textarea.value);
  const point = y => {
   const row = visualRow(data, y);
   const part = clamp((y - row.top) / rowHeight, 0, 1);
   return page.start + Math.min(page.text.length, row.start + part * Math.max(0, row.end - row.start));
  };
  const offset = point(textarea.scrollTop);
  return {offset, visible: Math.max(1, point(textarea.scrollTop + viewHeight()) - offset)};
 }

 function flowAnchor() {
  const row = visualRow(measureText(textarea.value), textarea.scrollTop);
  return {offset: page.start + Math.min(row.start, page.text.length), dy: textarea.scrollTop - row.top};
 }

 // Put the visual row that holds the anchor back at the same height.
 function flowRestore(anchor) {
  const local = anchor.offset - page.start;
  if (local < 0 || local > page.text.length) return false;
  textarea.scrollTop = rowOf(measureText(textarea.value), local).top + anchor.dy;
  return true;
 }

 // Show a fractional document offset at the top of the view (the inverse of
 // flowPosition, so dragging the bar inside a page is smooth).
 function flowPlace(target) {
  const local = clamp(target - page.start, 0, page.text.length);
  const row = rowOf(measureText(textarea.value), Math.floor(local));
  textarea.scrollTop = row.top + clamp((local - row.start) / Math.max(1, row.end - row.start), 0, 0.999) * rowHeight;
 }

 function projectionRows(layout, around, source, offsets) {
  const gaps = new Map(layout.gaps.map(gap => [gap.line - source.line, gap.count]));
  const rows = [];
  let row = layout.row;
  const addGaps = source => {
   const count = gaps.get(source) || 0;
   const size = Math.min(count, Math.max(128, clientRows * 3));
   const first = clamp((around ?? row) - row - clientRows, 0, count - size);
   for (let i = first; i < first + size; i++) rows.push({source, row:row+i, gap:true});
   row += count;
  };
  for (let source = 0; source < offsets.length; source++) {
   addGaps(source);
   rows.push({source,row:row++});
  }
  addGaps(offsets.length);
  return rows;
 }

 function buildProjection(baseRows, peerCounts, source, offsets, units) {
  const sourceRows = [], rows = [], parts = [];
  const ownCounts = peerCounts?.size ? measureCurrentWrapRows(source.text) : [];
  const partialTail = source.start + source.text.length < units;
  for (const value of baseRows) {
   const head = rows.length;
   if (!value.gap) sourceRows[value.source] = head;
   rows.push(value);
   const text = value.gap ? '' : source.text.slice(offsets[value.source], offsets[value.source + 1]).replace(/\n$/, '');
   parts.push(text);
   const own = value.gap ? 1 : ownCounts[value.source] || 1;
   const pad = !value.gap && partialTail && value.source === offsets.length - 1
    ? 0 : Math.max(0, (peerCounts?.get(value.row) || 1) - own);
   for (let i = 0; i < pad; i++) {
    rows.push({source:value.source + (value.gap ? 0 : 1), row:value.row, gap:true, wrapHead:head});
    parts.push('');
   }
   if (pad) value.wrapEnd = rows.length;
  }
  const text = parts.join('\n');
  return {rows, sourceRows, offsets:lineStarts(text), text};
 }

 async function planProjection(source, offsets, units, active, around, signal) {
  if (!alignment || source.column) return;
  const current = alignment;
  const complete = source.start + source.text.length === units;
  const end = source.line + offsets.length - (complete ? 0 : 1);
  const layout = await current.layout(source.line, end, signal);
  const valid = () => active() && !disposed && !composing && !signal.aborted && alignment === current;
  if (!valid()) return;
  const viewRow = alignedTop ?? layout.row + Math.max(0, top - source.line);
  const rows = projectionRows(layout, around ?? viewRow, source, offsets);
  const style = wrapStyle;
  const counts = wrapped && current.peerWrapRows ? await current.peerWrapRows(rows.map(row => row.row), signal) : null;
  if (!valid()) return;
  if (style !== wrapStyle) return planProjection(source, offsets, units, active, around, signal);
  return buildProjection(rows, counts, source, offsets, units);
 }

 async function project(active, around, preserveView = false) {
  if (!alignment || page.column) return;
  const source = page;
  const valid = () => active() && page === source;
  wrapRead?.abort();
  const request = wrapRead = new AbortController();
  let planned;
  try {
   planned = await planProjection(source, starts, session.state().metadata.units, valid, around, request.signal);
  } catch (error) {
   if (!request.signal.aborted) throw error;
   return;
  } finally {
   if (wrapRead === request) wrapRead = null;
  }
  if (request.signal.aborted || !valid() || !planned) return;
  const view = preserveView ? viewAnchor() : capture();
  projection = planned;
  gutterStamp = '';
  if (textarea.value !== projection.text) textarea.value = projection.text;
  select(view.start, view.end, view.direction);
  if (preserveView && (view.alignedTop == null || !showAligned(view.alignedTop, view.fraction))) showLine(view.top + view.fraction);
 }

 function updateReadOnly() {
  textarea.readOnly = readOnly || editing || session.state().blocked;
 }

 function report(error) {
  if (!disposed && error?.name !== 'AbortError') message(error.message, true);
 }

 function run(promise) { void promise.catch(report); }

 function nativeSelection() {
  return {
   start: page.start + sourceOffset(textarea.selectionStart),
   end: page.start + sourceOffset(textarea.selectionEnd),
   direction: textarea.selectionDirection || 'none'
  };
 }

 function capture() {
  if (drag) {
   // A pointer selection: the browser moves the focus inside the page. Once
   // the page slides away from the anchor, the native anchor rests on the
   // page edge and only the focus comes from the browser.
   const native = nativeSelection(), backward = native.direction === 'backward';
   const base = backward ? native.end : native.start, focus = backward ? native.start : native.end;
   drag.anchor ??= base;
   const inside = drag.anchor >= page.start && drag.anchor <= page.start + page.text.length;
   if (inside) {
    drag.anchor = base;
    selection = native;
   } else {
    selection = focus < drag.anchor ? {start: focus, end: drag.anchor, direction: 'backward'}
     : {start: drag.anchor, end: focus, direction: 'forward'};
   }
   virtual = !inside;
  } else if (!virtual) selection = nativeSelection();
  const view = {...selection, top, left: textarea.scrollLeft + page.column * characterWidth, fraction, alignedTop};
  if (flowing() && flowView) view.offset = flowView.offset;
  return view;
 }

 function viewAnchor() {
  const row = displayRow(), anchor = projection?.rows[groupStart(row)];
  return {...capture(), top:page.line + (anchor?.source ?? row), fraction:rowFraction(row), alignedTop:anchor?.row};
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
  else keepScriptSelection(textarea);
  announce();
 }

 function metrics() {
  const style = root.getComputedStyle(textarea);
  const fontStyle = style.font || style.fontSize + ' ' + style.fontFamily;
  const metricKey = fontStyle + '/' + style.lineHeight;
  const changedMetrics = metricKey !== font;
  if (changedMetrics) {
   const context = document.createElement('canvas').getContext('2d');
   if (context) {
    context.font = fontStyle;
    characterWidth = context.measureText('0').width || 8;
   }
   // CSSOM rounds line-height differently from the native editor's line
   // boxes. Measure one box so hundreds of rows cannot accumulate drift.
   const probe = createMeasure(container);
   probe.style.font = 'inherit';
   probe.textContent = '0';
   const height = probe.getBoundingClientRect().height;
   probe.remove();
   rowHeight = height || parseFloat(style.lineHeight) || 20;
   if (height) font = metricKey;
  }
  clientRows = Math.max(1, Math.floor(textarea.clientHeight / rowHeight));
  clientColumns = Math.max(1, Math.floor(textarea.clientWidth / characterWidth));
  padding = (parseFloat(style.paddingTop) || 0) + (parseFloat(style.paddingBottom) || 0);
  return changedMetrics;
 }

 function refreshWrap(force = false) {
  if (disposed) return;
  const style = root.getComputedStyle(textarea);
  const next = wordWrap() === true;
  const key = next ? [textarea.clientWidth, style.font, style.lineHeight, style.tabSize, style.letterSpacing].join(':') : '';
  if (!force && next === wrapped && key === wrapStyle) {
   if (metrics()) schedulePaint();
   return;
  }
  const saved = viewAnchor();
  wrapped = next;
  wrapStyle = key;
  textarea.wrap = next ? 'soft' : 'off';
  textarea.classList.toggle('is-wrapped', next);
  container.classList.toggle('nt-editor-disk-wrapped', next);
  syntax?.classList.toggle('is-wrapped', next);
  markLayer?.classList.toggle('is-wrapped', next);
  gutterStamp = '';
  wrapGeometry = null;
  flowView = null;
  flowStuck = '';
  metrics();
  if (next) textarea.scrollLeft = 0;
  const ticket = ++alignmentTicket;
  if (wrapFrame) root.cancelAnimationFrame(wrapFrame);
  if (!navigating && !editing && !composing) {
   const anchor = saved.alignedTop == null ? -1 : projection?.rows.findIndex(value => value.row === saved.alignedTop) ?? -1;
   if (anchor < 0) positionLine(saved.top + saved.fraction);
   else scrollToRow(anchor, saved.fraction);
   schedulePaint();
  }
  if (!alignment) {
   wrapFrame = 0;
   // The text wraps anew at this width: a page inside a long line moves to
   // the wrap grid (see startBefore) without moving the view.
   trail = [];
   if (next && !navigating && !editing && !composing && page.column % wrapGrid()) {
    run(navigate(async active => {
     const start = await startBefore(page.start + 1, 1);
     return start !== null && active() && (start === page.start || flowShift(start, active));
    }));
   } else scheduleFlow();
   return;
  }
  wrapFrame = root.requestAnimationFrame(() => {
   wrapFrame = 0;
   if (disposed || composing || !alignment || navigating || editing || ticket !== alignmentTicket) return;
   run(project(() => ticket === alignmentTicket, undefined, true));
  });
 }

 function paint() {
  if (disposed) return;
  frame = 0;
  const {lines, units} = session.state().metadata;
  const row = displayRow(), projected = projection?.rows[row];
  const first = Math.max(0, row - 3);
  const count = projection?.rows.length || starts.length;
  const last = Math.min(count, wrapped ? rowAt(textarea.scrollTop + textarea.clientHeight) + 6 : first + clientRows + 8);
  const stamp = [page.line, first, last, rowHeight, String(lines).length, alignmentTicket, markStamp].join(':');
  const geometry = ensureWrapGeometry();
  if (stamp !== gutterStamp || geometry !== gutterGeometry) {
   gutterStamp = stamp;
   gutterGeometry = geometry;
   const marked = bookmarks?.();
   const rows = [];
   for (let line = first; line < last; line++) {
    const element = document.createElement('div');
    element.style.height = (pixelAt(line + 1) - pixelAt(line)) + 'px';
    element.style.lineHeight = rowHeight + 'px';
    const value = projection?.rows[line];
    if (value?.gap) element.dataset.alignmentGap = '';
    if (value?.wrapHead !== undefined) element.dataset.wrapGap = '';
    if (!value?.gap) {
     const source = value?.source ?? line;
     element.dataset.sourceLine = String(page.line + source + 1);
     if (marked?.has(page.line + source) && (source > 0 || page.column === 0)) element.classList.add('is-bookmarked');
     if (source > 0 || page.column === 0 || !wrapped) element.textContent = element.dataset.sourceLine;
    }
    rows.push(element);
   }
   gutter.style.paddingTop = pixelAt(first) + 'px';
   gutter.replaceChildren(...rows);
   lastAnnouncement = '';
   const width = String(lines).length + 3 + 'ch';
   gutter.parentElement.style.width = width;
   gutter.parentElement.style.flexBasis = width;
  }
  gutter.style.transform = 'translateY(' + (-textarea.scrollTop) + 'px)';
  paintSyntax();
  paintMarks();
  top = Math.min(lines - 1, page.line + (projection?.rows[groupStart(row)]?.source ?? row));
  fraction = rowFraction(row);
  alignedTop = projected?.row;
  const flow = flowing();
  if (flow) paintFlowBar(lines, units);
  else {
   flowView = null;
   const scrollLines = alignment?.rows || lines, scrollTop = projected?.row ?? top;
   showBar(vertical, scrollLines <= clientRows);
   const lastTop = Math.max(1, scrollLines - clientRows), moved = barMoved();
   barTop = clamp(scrollTop / lastTop, 0, 1) * thumb(vertical, clientRows / scrollLines);
   placeBar(moved);
   scrollbarValue(vertical, scrollLines, scrollTop + 1);
  }
  // With word wrap the vertical bar covers long lines too.
  const longLine = page.lineUnits > pageUnits && !flow;
  showBar(horizontal, !longLine);
  horizontal.setAttribute('aria-label', wrapped ? 'Section of long source line' : 'Position in long line');
  container.classList.toggle('nt-editor-disk-long-line', longLine);
  if (longLine) {
   const surface = textarea.parentElement;
   horizontal.style.left = surface.offsetLeft + 'px';
   horizontal.style.width = surface.offsetWidth + 'px';
   const column = page.column + textarea.scrollLeft / characterWidth;
   const lastColumn = Math.max(1, page.lineUnits - (wrapped ? 1 : clientColumns)), moved = barLeftMoved();
   barLeft = clamp(column / lastColumn, 0, 1) * thumb(horizontal, wrapped ? 1 / page.lineUnits : clientColumns / page.lineUnits);
   if (!horizontalFrame && !moved && Math.abs(horizontal.scrollLeft - barLeft) > 1) horizontal.scrollLeft = barLeft;
   if (!moved) barLeftSet = horizontal.scrollLeft;
   scrollbarValue(horizontal, page.lineUnits + 1, Math.floor(column) + 1);
  }
  if (!flow && page.start + page.text.length < units) {
   const origin = nextPageOrigin();
   if (origin > page.line) prefetch({line: origin});
  }
  lastScrollTop = textarea.scrollTop;
  lastScrollLeft = textarea.scrollLeft;
  announce();
  painted();
 }

 function paintSyntax() {
  if (!syntax) return;
  const text = textarea.value, next = syntaxLanguage(text);
  const offsets = projection?.offsets || starts;
  const top = displayRow(), bottom = Math.min(offsets.length,
   wrapped ? rowAt(textarea.scrollTop + textarea.clientHeight) + 3 : top + clientRows + 2);
  const position = first => {
   syntax.style.transform = 'translate(' + (-textarea.scrollLeft) + 'px,' +
    (pixelAt(first) - textarea.scrollTop) + 'px)';
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

 // The marks of the rows in view, in a layer behind the text placed like
 // the syntax colours. A comparison's aligned rows show none.
 function paintMarks() {
  if (!markLayer) return;
  const text = textarea.value;
  if (projection || text !== page.text) {
   markLayer.hidden = true;
   return;
  }
  const top = displayRow(), bottom = Math.min(starts.length,
   wrapped ? rowAt(textarea.scrollTop + textarea.clientHeight) + 3 : top + clientRows + 2);
  const first = Math.max(0, top - 8), end = Math.min(starts.length, bottom + 12);
  const from = starts[first] || 0, to = end < starts.length ? starts[end] : text.length;
  const ranges = marks(page.start + from, page.start + to);
  const key = [page.start, from, to, markStamp, text.length].join(':');
  if (!ranges?.length) {
   // Nothing marked here, or not known yet: no marks of another part show.
   if (ranges || key !== markKey) markLayer.hidden = true;
   if (ranges) {
    markRanges = ranges;
    markKey = key;
   }
   return;
  }
  if (ranges !== markRanges || key !== markKey) {
   markRanges = ranges;
   markKey = key;
   let html = '', at = from;
   for (const [a, b, kind] of ranges) {
    const start = Math.max(a - page.start, at), stop = Math.min(b - page.start, to);
    if (stop <= start) continue;
    html += escapeText(text.slice(at, start)) + (kind ? '<mark class="is-current">' : '<mark>') + escapeText(text.slice(start, stop)) + '</mark>';
    at = stop;
   }
   markLayer.innerHTML = html + escapeText(text.slice(at, to)) + '\n';
   markLayer.dataset.first = String(first);
  }
  markLayer.hidden = false;
  markLayer.style.transform = 'translate(' + (-textarea.scrollLeft) + 'px,' +
   (pixelAt(Number(markLayer.dataset.first) || 0) - textarea.scrollTop) + 'px)';
 }

 function escapeText(value) {
  return value.replace(/[&<>]/g, c => c === '&' ? '&amp;' : c === '<' ? '&lt;' : '&gt;');
 }

 function scrollbarValue(element, maximum, value, text = '', minimum = 1) {
  element.setAttribute('aria-valuemin', String(minimum));
  element.setAttribute('aria-valuemax', String(maximum));
  element.setAttribute('aria-valuenow', String(Math.min(maximum, value)));
  if (text) element.setAttribute('aria-valuetext', text);
  else element.removeAttribute('aria-valuetext');
 }

 // Size a bar's spacer so its thumb shows the visible share of the content.
 // Returns the bar's scroll range. Painting moves a bar to the view's
 // position only while no scroll of that bar by the user waits to be read.
 function thumb(bar, share) {
  const across = bar === horizontal, track = across ? bar.clientWidth : bar.clientHeight;
  const size = Math.round(clamp(track / Math.max(share, 1e-9), track + 1, RANGE)) + 'px';
  const spacer = bar.firstChild;
  if (across ? spacer.style.width !== size : spacer.style.height !== size) spacer.style[across ? 'width' : 'height'] = size;
  return Math.max(1, parseFloat(size) - track);
 }

 // The document bar in word-wrap flow: the position is the character offset
 // at the top of the view; the thumb is the share of the estimated wrapped rows
 // that fit the view. A document that fits in one page maps exactly.
 function paintFlowBar(lines, units) {
  const end = page.start + page.text.length, whole = page.start === 0 && end >= units;
  const range = Math.max(0, textarea.scrollHeight - textarea.clientHeight);
  const view = flowView = flowPosition();
  showBar(vertical, whole && range <= 1);
  let position, share;
  if (whole) {
   position = range ? textarea.scrollTop / range : 0;
   share = textarea.clientHeight / Math.max(1, textarea.scrollHeight);
  } else {
   const atEnd = end >= units && textarea.scrollTop >= range - 1;
   position = atEnd ? 1 : page.start === 0 && textarea.scrollTop < 1 ? 0 :
    clamp(view.offset / Math.max(1, units - view.visible), 0, 1);
   const rows = lines + Math.max(0, units - lines + 1) / wrapGrid();
   share = Math.min(1, viewHeight() / rowHeight / Math.max(1, rows));
  }
  const moved = barMoved();
  barTop = position * thumb(vertical, share);
  placeBar(moved);
  const percent = Math.round(position * 1000) / 10;
  scrollbarValue(vertical, 100, percent, 'Line ' + (top + 1).toLocaleString() + ' of ' + lines.toLocaleString() + ', ' + percent + '%', 0);
 }

 function schedulePaint() {
  if (!frame && !disposed) frame = root.requestAnimationFrame(paint);
 }

 function install(value, planned = null) {
  if (disposed || !value) return;
  const view = capture();
  projection = planned;
  alignmentTicket++;
  trail = [];
  page = value;
  starts = lineStarts(page.text);
  const text = projection?.text ?? page.text;
  if (textarea.value !== text) textarea.value = text;
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
   if (ticket === navigation) {
    navigating = false;
    // Scrolling that waited for text continues; otherwise keep text around
    // the view so the reader can keep scrolling without waiting.
    if (flowing() && !disposed) {
     if (pendingScroll) run(navigate(flowDrain));
     else scheduleFlow();
    }
   }
  }
 }

 // Word-wrap flow: keep at least this much text above and below the view.
 function flowMargin() {
  const range = Math.max(0, textarea.scrollHeight - textarea.clientHeight);
  return clamp(range / 4, rowHeight * 2, Math.max(rowHeight * 2, textarea.clientHeight));
 }

 function flowNeed() {
  const units = session.state().metadata.units, margin = flowMargin();
  const below = textarea.scrollHeight - textarea.clientHeight - textarea.scrollTop;
  if (page.start + page.text.length < units && below < margin) return 1;
  if (page.start > 0 && textarea.scrollTop < margin) return -1;
  return 0;
 }

 function scheduleFlow() {
  if (flowFrame || disposed) return;
  flowFrame = root.requestAnimationFrame(() => {
   flowFrame = 0;
   if (disposed || !flowing() || composing || editing || navigating) return;
   const direction = flowNeed();
   const stamp = [page.start, page.text.length, direction, session.state().metadata.revision].join(':');
   if (direction && stamp !== flowStuck) {
    run(navigate(async active => {
     const moved = await flowSlide(active, direction);
     if (!moved && active()) flowStuck = stamp;
     return moved;
    }));
   }
  });
 }

 // A start that leaves about `before` characters, and at most half a page of
 // lines, ahead of `target`. Only the text before the target is read. Inside
 // a long line the start is on the wrap grid: a whole number of rows of the
 // width's character count after the line start, so text that can break
 // anywhere (Base64, hex) wraps into the same rows on every page. Word wrap
 // from different starts need not agree (with a fixed-width font it often
 // does not for hundreds of rows), so a page with such text reached this way
 // can wrap differently from the one before; see `trail` for scrolling back.
 async function startBefore(target, before) {
  target = Math.floor(target);
  if (target <= 0) return 0;
  const from = Math.max(0, target - before);
  const probe = await session.peek({start: from}, target - from);
  if (!probe) return null;
  const text = probe.text, cap = root.NTKEditorDiskSession.PAGE_LINES >> 1;
  let lines = 0;
  for (let at = text.length; at > 0 && (at = text.lastIndexOf('\n', at - 1)) >= 0;) {
   if (++lines > cap) return probe.start + at + 1;
  }
  return probe.start - probe.column % wrapGrid();
 }

 // Replace the page without moving the text the reader sees. `chain` is the
 // trail that the new page continues.
 async function flowShift(start, active, chain = []) {
  const value = await session.load({start}, pageUnits);
  if (!value || !active()) return false;
  const anchor = flowAnchor(), end = page.start + page.text.length;
  install(value);
  trail = chain;
  if (!flowRestore(anchor)) textarea.scrollTop = anchor.offset >= end ? textarea.scrollHeight : 0;
  paint();
  return true;
 }

 function flowSlide(active, direction) {
  const units = session.state().metadata.units, end = page.start + page.text.length;
  if (direction > 0 ? end >= units : page.start <= 0) return false;
  const margin = flowMargin();
  if (direction > 0 && textarea.scrollTop > margin * 2) {
   const row = visualRow(measureText(textarea.value), textarea.scrollTop - margin * 2);
   if (row.start > 0 && row.start < page.text.length) return flowForward(active, page.start + row.start);
  }
  return flowRecenter(active, flowAnchor().offset, direction);
 }

 // A visual row start of this page keeps every following row exactly as it
 // is; this page joins the trail.
 function flowForward(active, start) {
  return flowShift(start, active, [...trail, page.start].slice(-64));
 }

 // Load a page around `offset` (near the view, around the view itself).
 async function flowRecenter(active, offset, direction) {
  const anchor = flowAnchor().offset;
  const target = Math.abs(offset - anchor) < recenter / 2 ? anchor : offset;
  if (direction > 0 && target - recenter > page.start) {
   const row = rowOf(measureText(textarea.value), target - recenter - page.start);
   if (row.start > 0) return flowForward(active, page.start + row.start);
  }
  if (direction < 0) {
   // Back to text this chain of pages already wrapped: its rows stay as they were.
   for (let i = trail.length - 1; i >= 0 && target - trail[i] <= pageUnits - recenter / 2; i--) {
    if (trail[i] < page.start) return flowShift(trail[i], active, trail.slice(0, i));
   }
  }
  const start = await startBefore(target, recenter);
  if (start === null || !active() || start === page.start) return false;
  return flowShift(start, active);
 }

 // Scroll by `delta` pixels, bringing more text into the page as needed.
 function flowScroll(delta) {
  const limit = textarea.clientHeight * 4;
  pendingScroll = clamp(pendingScroll + delta, -limit, limit);
  if (!navigating) run(navigate(flowDrain));
 }

 async function flowDrain(active) {
  for (let step = 0; step < 8 && pendingScroll && active(); step++) {
   const before = textarea.scrollTop;
   textarea.scrollTop = before + pendingScroll;
   pendingScroll -= textarea.scrollTop - before;
   if (Math.abs(pendingScroll) < 1 || !await flowSlide(active, Math.sign(pendingScroll))) break;
  }
  pendingScroll = 0;
  paint();
  return true;
 }

 // Show a document offset at the top of the view, or the end of the document.
 async function flowGo(target, active, bottom = false) {
  const units = session.state().metadata.units;
  target = clamp(target, 0, units);
  const at = Math.floor(target), end = page.start + page.text.length;
  if (at < page.start || at > end || at === end && end < units) {
   const start = await startBefore(at, recenter);
   if (start === null || !active() || !await loadPage({start, gridded: true}, active)) return false;
  }
  if (bottom) textarea.scrollTop = textarea.scrollHeight;
  else flowPlace(target);
  paint();
  return true;
 }

 // Keyboard moves by visual rows when the selection reaches past the page.
 async function flowMove(key, anchor, cursor, extend, active) {
  const units = session.state().metadata.units;
  let data = measureText(textarea.value);
  let row = rowOf(data, cursor - page.start);
  const reach = viewHeight() + rowHeight * 2;
  const up = row.top < reach && page.start > 0;
  if (up || row.top + reach > pixelAt(starts.length) && page.start + page.text.length < units) {
   await flowRecenter(active, cursor, up ? -1 : 1);
   if (!active()) return false;
   data = measureText(textarea.value);
   row = rowOf(data, cursor - page.start);
  }
  let offset;
  if (key === 'Home') offset = row.start;
  else if (key === 'End') offset = row.last ? row.end - 1 : extend ? row.end : row.end - 1;
  else {
   const screen = viewHeight() - rowHeight;
   const step = {ArrowUp: -rowHeight, ArrowDown: rowHeight, PageUp: -screen, PageDown: screen}[key];
   const y = row.top + step + rowHeight / 2, content = pixelAt(starts.length);
   if (y < 0) offset = 0;
   else if (y >= content) offset = page.text.length;
   else offset = offsetAtPoint(data, caretX(data, cursor - page.start), y);
   if (key === 'PageUp' || key === 'PageDown') textarea.scrollTop += step;
  }
  moveSelection(anchor, alignCursor(page.start + clamp(offset, 0, page.text.length)), extend);
  return true;
 }

 async function loadPage(target, active) {
  const {alignedRow, gridded, ...source} = target;
  // In word wrap a page inside a long line starts on the wrap grid (see startBefore).
  if (flowing() && source.start > 0 && !gridded) {
   const start = await startBefore(source.start + 1, 1);
   if (start === null || !active()) return false;
   source.start = Math.min(source.start, start);
  }
  let planned, current;
  // Keep the current projected page visible until the new source text and
  // its spacer layout can be installed together.
  const prepare = alignment ? async (next, units, signal) => {
   const offsets = lineStarts(next.text);
   do {
    current = alignment;
    planned = await planProjection(next, offsets, units, active, alignedRow, signal);
   } while (active() && !signal.aborted && current !== alignment);
  } : undefined;
  const result = await session.load(source, pageUnits, prepare);
  if (!result || !active()) return false;
  install(result, current === alignment ? planned : null);
  if (alignment && !projection && !page.column) await project(active, alignedRow);
  return true;
 }

 function positionLine(line) {
  const source = clamp(Math.floor(line - page.line), 0, starts.length - 1);
  const row = projection?.sourceRows[source] ?? source;
  scrollToRow(row, line % 1);
 }

 function showLine(line) {
  positionLine(line);
  paint();
 }

 function showAligned(row, fraction) {
  const local = projection?.rows.findIndex(value => value.row === row) ?? -1;
  if (local < 0) return false;
  scrollToRow(local, fraction);
  paint();
  return true;
 }

 async function goAlignedRow(row, fraction = 0) {
  if (!alignment) return false;
  const current = alignment;
  row = clamp(Math.floor(row), 0, current.rows - 1);
  const local = projection?.rows.findIndex(value => value.row === row) ?? -1;
  if (local >= 0 && (projection.rows.at(-1).row === current.rows - 1 ||
      rowScrollOffset(local, fraction) + textarea.clientHeight <= pixelAt(projection.rows.length))) {
   if (navigating) {
    navigation++;
    navigating = false;
    session.cancelRead?.();
   }
   return showAligned(row, fraction);
  }
  return navigate(async active => {
   const target = await current.source(row);
   if (!active() || current !== alignment) return false;
   const count = session.state().metadata.lines;
   const line = Math.min(count - 1, target.line);
   const origin = Math.max(0, line - Math.min(BUFFER_LINES, clientRows));
   if (!await loadPage({line:origin, alignedRow:row}, active) || current !== alignment) return false;
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
  const saved = viewAnchor();
  wrapRead?.abort();
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
  await project(() => ticket === alignmentTicket, undefined, true);
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
   if (options.bottom) {
    textarea.scrollTop = textarea.scrollHeight;
    paint();
   } else showLine(options.line ?? page.line);
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
   // In word wrap, also keep text above the target so the reader can
   // scroll back without waiting.
   const start = flowing() ? await startBefore(offset, recenter) : Math.max(0, offset - PREVIEW_UNITS);
   if (start === null || !active() || !await loadPage({start, gridded: true}, active)) return false;
  }
  return true;
 }

 function showOffset(offset) {
  const local = clamp(offset - page.start, 0, page.text.length);
  const row = localLine(local);
  if (wrapped) {
   const visual = caretPixel(displayOffset(local));
   if (visual < textarea.scrollTop || visual + rowHeight > textarea.scrollTop + textarea.clientHeight) {
    textarea.scrollTop = Math.max(0, visual - rowHeight * 2);
   }
   paint();
   return;
  }
  const column = local - starts[row];
  const right = textarea.scrollLeft + textarea.clientWidth - characterWidth * 4;
  if (column * characterWidth > right || column * characterWidth < textarea.scrollLeft) {
   textarea.scrollLeft = Math.max(0, (column - Math.floor(clientColumns / 2)) * characterWidth);
  }
  showLine(page.line + Math.max(0, row - 2));
 }

 function caretPixel(offset) {
  const data = measureText(textarea.value);
  const offsets = projection?.offsets || starts;
  const row = localLine(offset, offsets);
  const text = data.rows[row]?.firstChild;
  if (!text || !text.length) return pixelAt(row);
  const column = clamp(offset - offsets[row], 0, text.length);
  const range = document.createRange();
  range.setStart(text, column);
  range.setEnd(text, column);
  const rect = range.getClientRects()[0];
  return rect ? pixelAt(row) + Math.max(0, Math.round((rect.top - data.rows[row].getBoundingClientRect().top) / rowHeight)) * rowHeight : pixelAt(row);
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
  if (flowing() && Number.isFinite(view.offset)) {
   return navigate(async active => {
    if (!await flowGo(view.offset, active)) return false;
    select(view.start ?? 0, view.end ?? view.start ?? 0, view.direction, focus);
    return true;
   });
  }
  return navigate(async active => {
   const line = clamp(view.top ?? 0, 0, session.state().metadata.lines - 1);
   if (!await openLine(line, active)) return false;
   if (line === page.line && page.lineUnits > pageUnits && view.left > 0 && (wrapped || view.left > pageUnits * characterWidth)) {
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

 function panLongLine(column, bottom = false) {
  const start = page.start - page.column;
  const last = Math.max(0, page.lineUnits - (wrapped ? 1 : clientColumns));
  column = clamp(column, 0, last);
  return load({start: start + column}, {left: 0, keepSelection: true, bottom});
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
  if (flowing()) {
   if (!composing && !editing) scheduleFlow();
   return;
  }
  // A pointer on the textarea's own scroll bar must not have its page
  // replaced; a selection drag may move on to the next page (see capture).
  if (composing || navigating || editing || pointer && !drag) return;
  const {units, lines} = session.state().metadata;
  const display = displayRow(), projected = projection?.rows[display];
  const row = projection?.rows[groupStart(display)]?.source ?? display;
  const line = page.line + row;
  const movedDown = textarea.scrollTop > lastScrollTop;
  const movedUp = textarea.scrollTop < lastScrollTop;
  if (projected) {
   const previous = groupStart(rowAt(lastScrollTop)), head = groupStart(display);
   const before = head < previous, end = Math.max(previous, head);
   for (let index = Math.min(previous, head); index < end; index = groupEnd(index)) {
    const next = groupEnd(index), a = projection.rows[index].row, b = projection.rows[next].row;
    if (b <= a + 1) continue;
    run(goAlignedRow(before ? b - 1 : a + 1, rowFraction(display)));
    return;
   }
   const nearBottom = wrapped
    ? pixelAt(projection.rows.length) - textarea.scrollTop <= textarea.clientHeight + rowHeight * 2
    : display + clientRows >= projection.rows.length - 2;
   if (movedDown && nearBottom && projected.row + clientRows < alignment.rows) {
    run(goAlignedRow(projected.row + 1));
    return;
   }
  }
  if (!projected && movedUp && row < BUFFER_LINES / 2 && page.line > 0) {
   const target = Math.max(0, line - BUFFER_LINES);
   if (target < page.line) run(goLine(line, false, true));
  } else if (!projected && movedDown && page.start + page.text.length < units) {
   const remaining = starts.length - row - clientRows;
   const origin = nextPageOrigin();
   if (origin > page.line) {
    if (row >= origin - page.line && remaining < BUFFER_LINES / 2 && !projected?.gap) run(rollForward(line, origin, rowFraction(display)));
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
  if (flowing()) {
   if (composing || editing || event.defaultPrevented || event.ctrlKey || !event.deltaY) return;
   const delta = event.deltaY * (event.deltaMode === 1 ? rowHeight : event.deltaMode === 2 ? viewHeight() : 1);
   const range = textarea.scrollHeight - textarea.clientHeight;
   const units = session.state().metadata.units;
   const blocked = delta > 0 ? textarea.scrollTop + delta > range - 1 && page.start + page.text.length < units
    : textarea.scrollTop + delta < 0 && page.start > 0;
   // The native scroll handles everything the page holds.
   if (blocked || pendingScroll) {
    event.preventDefault();
    flowScroll(delta);
   }
   return;
  }
  if (composing || navigating || editing || event.defaultPrevented) return;
  const current = session.state().metadata;
  const delta = event.deltaY;
  const row = displayRow();
  const bottom = textarea.scrollTop + textarea.clientHeight >= textarea.scrollHeight - 2;
  if (wrapped && !event.ctrlKey && !event.shiftKey) {
   const forward = delta > 0 && bottom && page.start + page.text.length < current.units;
   const backward = delta < 0 && textarea.scrollTop < 2 && page.column > 0;
   if (backward || forward && starts.length === 1 && page.column + page.text.length < page.lineUnits) {
    event.preventDefault();
    const overlap = Math.min(pageUnits / 2, clientColumns * Math.max(1, clientRows - 2));
    const step = Math.max(1, (forward ? page.text.length : pageUnits) - overlap);
    run(panLongLine(page.column + (forward ? step : -step), backward));
    return;
   }
   if (!projection && forward && starts.length > 1 && starts.at(-1) < page.text.length) {
    // Continue a partially loaded line before moving to the next logical line.
    const line = page.line + starts.length - 1;
    event.preventDefault();
    run(load({line}, {line}));
    return;
   }
  }
  if (projection && (delta < 0 && textarea.scrollTop < 2 || delta > 0 && bottom)) {
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
   if (leftEdge || rightEdge) {
    const start = flowing() ? await startBefore(cursor, recenter) : Math.max(0, cursor - PREVIEW_UNITS);
    if (start === null || !active() || !await loadPage({start, gridded: true}, active)) return false;
   }
   if (flowing() && ['ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End'].includes(key)) {
    return flowMove(key, anchor, cursor, extend, active);
   }
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
  if (wrapped && !virtual && atBoundary[key] && !control) {
   const caret = caretPixel(displayOffset(at));
   if (key === 'ArrowUp') atBoundary[key] = caret < rowHeight / 2;
   if (key === 'ArrowDown') atBoundary[key] = caret + rowHeight * 1.5 >= pixelAt(projection?.rows.length || starts.length);
   if (key === 'PageUp') atBoundary[key] = textarea.scrollTop < 2;
   if (key === 'PageDown') atBoundary[key] = textarea.scrollTop + textarea.clientHeight >= textarea.scrollHeight - 2;
   if (key === 'Home' || key === 'End') atBoundary[key] = false;
  }
  if (Object.hasOwn(atBoundary, key) && (virtual || atBoundary[key])) {
   stop();
   run(move(key, event.shiftKey, control));
  }
 }

 textarea.addEventListener('pointerdown', event => {
  // Another button (the context menu) keeps a selection that spans pages.
  if (event.button !== 0) return;
  pointer = true;
  drag = null;
  pressed = null;
  // The textarea's own scroll bar is not text.
  if (event.offsetX >= textarea.clientWidth || event.offsetY >= textarea.clientHeight) return;
  // Shift+click extends the whole selection, also when it reaches past the page.
  if (event.shiftKey) drag = {anchor: cursorAnchor(capture())};
  else {
   virtual = false;
   // A plain click, double-click (word) or triple-click (line) is not a drag:
   // the page must stay put until the button is released, or replacing it
   // would drop the selection the browser just made (CI found a lost
   // double-click word). It becomes a drag once the pointer moves.
   pressed = {x: event.clientX, y: event.clientY};
  }
 }, listeners);
 textarea.addEventListener('mousedown', event => { clicks = event.detail; }, listeners);
 textarea.addEventListener('pointermove', event => {
  if (!pointer || drag || !pressed || !(event.buttons & 1) || clicks > 1) return;
  if (Math.abs(event.clientX - pressed.x) + Math.abs(event.clientY - pressed.y) < 4) return;
  pressed = null;
  drag = {anchor: null};
 }, listeners);
 const release = () => {
  if (!pointer) return;
  pointer = false;
  if (drag && !disposed) {
   const view = capture();
   drag = null;
   select(view.start, view.end, view.direction);
  }
  drag = null;
  pressed = null;
  schedulePaint();
 };
 root.addEventListener('pointerup', release, listeners);
 root.addEventListener('pointercancel', release, listeners);
 textarea.addEventListener('select', schedulePaint, listeners);
 textarea.addEventListener('selectionchange', schedulePaint, listeners);
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
  if (!barMoved() || verticalFrame) return;
  verticalFrame = root.requestAnimationFrame(() => {
   verticalFrame = 0;
   if (disposed || !barMoved()) return;
   barSet = vertical.scrollTop;
   seek(clamp(vertical.scrollTop / Math.max(1, vertical.scrollHeight - vertical.clientHeight), 0, 1));
  });
 }, listeners);

 // Show a position of the document bar (0 to 1) at the top of the view.
 function seek(position) {
  if (flowing()) {
   const {units} = session.state().metadata;
   if (page.start === 0 && page.start + page.text.length >= units) {
    textarea.scrollTop = position * (textarea.scrollHeight - textarea.clientHeight);
    paint();
   } else {
    const target = position * Math.max(0, units - (flowView?.visible || 0));
    run(navigate(active => flowGo(position >= 1 ? units : target, active, position >= 1)));
   }
   return;
  }
  const last = Math.max(0, (alignment?.rows || session.state().metadata.lines) - clientRows);
  const line = Math.round(position * last);
  run(alignment ? goAlignedRow(line) : goLine(line));
 }
 vertical.addEventListener('keydown', event => {
  if (flowing()) {
   const step = {ArrowUp: -rowHeight, ArrowDown: rowHeight, PageUp: rowHeight - viewHeight(), PageDown: viewHeight() - rowHeight};
   if (event.key === 'Home' || event.key === 'End') {
    event.preventDefault();
    const end = event.key === 'End';
    run(navigate(active => flowGo(end ? session.state().metadata.units : 0, active, end)));
   } else if (Object.hasOwn(step, event.key)) {
    event.preventDefault();
    flowScroll(step[event.key]);
   }
   return;
  }
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
  if (!barLeftMoved() || horizontalFrame) return;
  horizontalFrame = root.requestAnimationFrame(() => {
   horizontalFrame = 0;
   if (disposed || !barLeftMoved()) return;
   barLeftSet = horizontal.scrollLeft;
   const last = Math.max(0, page.lineUnits - (wrapped ? 1 : clientColumns));
   run(panLongLine(Math.round(horizontal.scrollLeft / Math.max(1, horizontal.scrollWidth - horizontal.clientWidth) * last)));
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
  ? new root.ResizeObserver(() => { refreshWrap(); metrics(); schedulePaint(); }) : null;
 resize?.observe(textarea);
 metrics();
 refreshWrap();
 schedulePaint();

 // The Document Map (editor-document-map.js) maps a document that is not all
 // in the page by position, like the document bar: by line without word wrap,
 // by character offset with it. It reads what the last paint used.
 function mapState() {
  const {lines, units, revision} = session.state().metadata;
  const flow = flowing(), end = page.start + page.text.length;
  let estimatedRows = lines, wrapColumns = clientColumns;
  if (flow) {
   wrapColumns = wrapGrid();
   estimatedRows = lines + Math.max(0, units - lines + 1) / wrapColumns;
  }
  return {lines, units, revision, flow, aligned: !!alignment, whole: page.start === 0 && end >= units,
   top, fraction, rows: viewHeight() / rowHeight, offset: flowView?.offset ?? page.start, visible: flowView?.visible ?? 0,
   estimatedRows, wrapColumns, rowHeight, characterWidth,
   page: {start: page.start, line: page.line, column: page.column, text: page.text, lineUnits: page.lineUnits}};
 }

 // Put a line (without word wrap) or a character offset (with it) at the top
 // of the view, as the document bar does.
 function mapSeek({line, offset}) {
  if (disposed || alignment) return;
  const {lines, units} = session.state().metadata;
  if (flowing()) {
   const visible = flowView?.visible || 0, target = clamp(Number(offset) || 0, 0, units);
   const last = target >= units - visible;
   if (page.start === 0 && page.start + page.text.length >= units) {
    seek(clamp(target / Math.max(1, units - visible), 0, 1));
    return;
   }
   run(navigate(active => flowGo(last ? units : target, active, last)));
   return;
  }
  const value = clamp(Number(line) || 0, 0, Math.max(0, lines - clientRows));
  run(goLine(Math.floor(value), false, false, value - Math.floor(value)));
 }

 // Scroll by pixels as the mouse wheel over the text does, loading pages.
 function wheelBy(delta) {
  let prevented = false;
  wheel({deltaY: delta, deltaMode: 0, ctrlKey: false, shiftKey: false,
   get defaultPrevented() { return prevented; }, preventDefault() { prevented = true; }});
  if (!prevented) textarea.scrollTop += delta;
 }

 return Object.freeze({
  load, goLine, goAlignedRow, setAlignment, position, restore, replace, history, capture, select, selectedText, copy,
  mapState, mapSeek, wheelBy,
  comparisonPage: () => ({line:page.line, text:page.text, column:page.column, complete:page.start+page.text.length>=session.state().metadata.units}),
  measureWrapRows:measureCurrentWrapRows, refreshWrap, refreshSyntax: paintSyntax,
  // The Find window's marks or bookmarks changed: paint them again.
  refreshMarks() { markStamp++; markKey = ''; gutterStamp = ''; schedulePaint(); },
  selectAll() { select(0, session.state().metadata.units, 'none', true); },
  // locked() is a document that can never be edited (a file that is not text).
  setReadOnly(value) { readOnly = !!value || !!locked(); updateReadOnly(); },
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
   const focus = cursorOffset(capture()), lineStart = page.start - page.column;
   // A focus beyond the page on the page's first line (a long line).
   if ((focus < page.start || focus > page.start + page.text.length) && focus >= lineStart &&
       focus <= lineStart + page.lineUnits) return {line: page.line, column: focus - lineStart};
   const offset = clamp(focus - page.start, 0, page.text.length);
   const row = localLine(offset);
   return {line: page.line + row, column: offset - starts[row] + (row === 0 ? page.column : 0)};
  },
  dispose() {
   disposed = true;
   navigation++;
   session.cancelRead?.();
   wrapRead?.abort();
   abort.abort();
   resize?.disconnect();
   for (const job of [frame, verticalFrame, horizontalFrame, wrapFrame, flowFrame]) if (job) root.cancelAnimationFrame(job);
   vertical.remove();
   horizontal.remove();
   markLayer?.remove();
   if (syntax) {
    root.clearTimeout(syntaxTimer);
    root.NTKEditorLanguageService?.cancelHighlight?.(syntaxChannel);
    syntax.remove();
    textarea.classList.remove('highlight-active');
   }
   textarea.readOnly = original.readOnly;
   textarea.wrap = original.wrap;
   textarea.classList.toggle('is-wrapped', original.wrapped);
   measure?.remove();
   gutter.parentElement.style.width = original.width;
   gutter.parentElement.style.flexBasis = original.basis;
   gutter.style.paddingTop = original.padding;
   gutter.style.transform = original.transform;
   container.classList.remove('nt-editor-disk', 'nt-editor-disk-long-line', 'nt-editor-disk-wrapped');
  }
 });
}
root.NTKEditorDiskView = Object.freeze({create, measureWrapRows});
if (typeof module === 'object' && module.exports) module.exports = {create, measureWrapRows};
})(globalThis);
