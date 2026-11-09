/* The large-file workspace renders the same editable pages used by the
   single editor. Difference metadata is requested for visible lines only. */
(function(root) {
'use strict';

const sides = ['left', 'right'];
const peerOf = side => side === 'left' ? 'right' : 'left';
const number = value => Number(value || 0).toLocaleString();
const clamp = (value, min, max) => Math.min(max, Math.max(min, value));
const pageSize = 64;
const differenceSymbols = {added:'+', removed:'−', changed:'~'};
const mapScrollRange = 1000000;

function selectorMarkup(side) {
 return '<label class="nt-compare-selector">' + (side === 'left' ? 'Left' : 'Right') +
  ' <select data-compare-select="' + side + '" aria-label="' + side + ' comparison document"></select></label>';
}

function paneMarkup(side) {
 return '<section class="nt-compare-pane" data-compare-pane="' + side + '">' +
  '<div class="nt-editor-tabs" data-ed-tabs data-ed-view="' + side + '" role="tablist" aria-label="' + side + ' view document tabs"></div>' +
  '<div class="nt-disk-compare-code">' +
  '<div class="nt-editor-gutter" aria-hidden="true"><div data-disk-compare-gutter="' + side + '"></div></div>' +
  '<div class="nt-editor-surface"><div class="nt-disk-compare-colors" aria-hidden="true"></div>' +
  '<textarea id="ntCompare-' + side + '" data-compare-text="' + side + '" data-input-clear="off" wrap="off" spellcheck="false" aria-label="Edit ' + side + ' document"></textarea></div></div>' +
  '<div class="nt-compare-document-state" data-compare-state="' + side + '"></div></section>';
}

function mapMarkup(side) {
 return '<div class="nt-compare-track" data-compare-track="' + side + '" tabindex="0" role="scrollbar" aria-label="' +
  side + ' document map" aria-orientation="vertical" aria-controls="ntCompare-' + side + '" title="Scroll the map. Click a line to navigate.">' +
  '<svg data-compare-map-marks="' + side + '" aria-hidden="true" preserveAspectRatio="none"></svg>' +
  '<div data-compare-map-viewport="' + side + '" aria-hidden="true"></div></div>';
}

function create(options) {
 const host = options.container;
 const comparing = options.comparing !== false;
 const viewField = comparing ? 'compareView' : 'view';
 host.className = 'nt-editor-compare nt-editor-disk-compare' + (comparing ? '' : ' is-split-view');
 host.innerHTML = '<div class="nt-compare-tools">' + sides.map(selectorMarkup).join('') +
  '<button type="button" class="btn small primary" data-compare="compute">Compare</button>' +
  '<button type="button" class="btn small ghost" data-compare="swap">Swap sides</button>' +
  '<button type="button" class="btn small ghost" data-compare="previous" title="Previous difference">Previous</button>' +
  '<button type="button" class="btn small ghost" data-compare="next" title="Next difference">Next</button>' +
  '<details class="nt-compare-settings"><summary class="btn small ghost">Options</summary><div>' +
  '<label><input type="checkbox" data-compare-sync checked> Sync scroll</label>' +
  '<label><input type="checkbox" data-compare-horizontal-sync> Sync horizontal scroll</label>' +
  '<label><input type="checkbox" data-compare-underline> Underline changes</label>' +
  '<label><input type="checkbox" data-compare-intraline> Highlight changed characters</label>' +
  '<label>Map zoom <select data-compare-scale aria-label="Map zoom"><option value="0">Individual lines</option>' +
  '<option value="-2">Compact (3 lines per pixel)</option><option value="-3">Dynamic fit (1 pixel marks)</option>' +
  '<option value="0.5">1.5×</option><option value="1">2×</option><option value="2">3×</option><option value="4">5×</option></select></label>' +
  '</div></details>' +
  '<button type="button" class="btn small ghost" data-compare="close">Close comparison</button></div>' +
  '<div class="nt-compare-summary"><span data-compare-summary role="status"></span>' +
  '<span class="nt-compare-legend" aria-label="Difference key"><span class="added">+ Added</span><span class="removed">− Removed</span>' +
  '<span class="changed">~ Modified</span></span></div><div class="nt-compare-grid">' +
  sides.map(paneMarkup).join('<div class="nt-editor-pane-resize" data-compare-divider></div>') +
  '<div class="nt-compare-map-panel"><div class="nt-compare-map-labels" aria-hidden="true"><span>Left</span><span>Right</span></div>' +
  '<div class="nt-compare-map is-line-map" data-compare-map><div data-compare-map-content><div class="nt-disk-map-window">' +
  sides.map(mapMarkup).join('') + '</div></div></div></div></div>';

 const q = selector => host.querySelector(selector);
 const summary = q('[data-compare-summary]');
 const map = q('[data-compare-map]');
 const mapContent = q('[data-compare-map-content]');
 const prefs = options.comparisonPreferences?.() || {};
 q('[data-compare-horizontal-sync]').checked = prefs.compareHorizontalSync === true;
 q('[data-compare-underline]').checked = prefs.compareUnderline !== false;
 q('[data-compare-intraline]').checked = prefs.compareIntralineHighlight !== false;
 root.NTKEditorCompare.mountSettings(host);

 const panes = Object.fromEntries(sides.map(side => {
  const section = q('[data-compare-pane="' + side + '"]');
  return [side, {
   section, container: section.querySelector('.nt-disk-compare-code'),
   textarea: q('[data-compare-text="' + side + '"]'),
   gutter: q('[data-disk-compare-gutter="' + side + '"]'),
   colors: section.querySelector('.nt-disk-compare-colors'),
   track: q('[data-compare-track="' + side + '"]'),
   select: q('[data-compare-select="' + side + '"]'),
   markers: q('[data-compare-map-marks="' + side + '"]'),
   marker: q('[data-compare-map-viewport="' + side + '"]'),
   view: null, ready: false, doc: null, lines: [0], text: '', top: 0, ranges: [],
   composing: false, creating: false, requested: null, pending: null, locateTimer: null
  }];
 }));
 let ids = {left:'', right:''};
 let lastSide = options.activeSide?.() || 'left';
 let signature = '', listSignature = '', disposed = false, syncing = 0, scale = 0;
 let job = null, startTimer = null, selected = -1, drag = null;
 let mapFrame = 0, gestureSide = null;
 let mapTop = 0, mapExtent = 0, mapScrollMax = 0, mapScrollPosition = 0;
 let mapData = null, mapRequest = null, mapPaintKey = '';
 let lineHighlight = null;
 const preview = comparing && root.NTKEditorCompare.mountDifferencePreview(host, readDifferencePreview, () =>
  job?.status && {count:job.status.count, index:Math.max(0, selected)});
 const mapLineCount = () => Math.max(...sides.map(side => totalLines(panes[side])));
 const mapPitch = () => scale === -3 ? Math.max(1, map.clientHeight) / mapLineCount() :
  scale === -2 ? 1 / 3 : Math.max(2, Math.ceil(map.clientHeight / 3500)) * (1 + scale);
 const splitter = root.NTKEditorCompare.createDivider(host, q('.nt-compare-grid'), comparing, () => {
  for (const side of sides) changed(side);
 });
 const activeSide = () => options.activeSide?.() || lastSide;
 const composing = () => sides.some(side => panes[side].composing || panes[side].view?.composing);
 const lineHeight = pane => parseFloat(root.getComputedStyle(pane.textarea).lineHeight) || 20;
 const linesInView = pane => Math.ceil(pane.textarea.clientHeight / lineHeight(pane)) + 6;
 const totalLines = pane => pane.doc?.disk ? pane.doc.diskMeta?.lines || 1 : pane.lines.length;
 const go = promise => { void promise.catch(error => setSummary(error.message)); };

 function setSummary(message) {
  if (!disposed && summary.textContent !== message) summary.textContent = message;
 }

 function indexInline(pane, text) {
  if (pane.text === text) return;
  pane.text = text;
  pane.lines = [0];
  for (let end = text.indexOf('\n'); end !== -1; end = text.indexOf('\n', end + 1)) pane.lines.push(end + 1);
 }

 function remember(side) {
  const {doc, view, textarea} = panes[side];
  if (!doc) return;
  doc[viewField] = view ? view.capture() : {
   start: textarea.selectionStart, end: textarea.selectionEnd, direction: textarea.selectionDirection,
   top: textarea.scrollTop, left: textarea.scrollLeft
  };
 }

 function updatePane(side, doc) {
  const pane = panes[side], ta = pane.textarea;
  const changedDocument = pane.doc !== doc || (!!doc?.disk && !pane.view && !doc.staged);
  if (changedDocument) {
   remember(side);
   pane.view?.dispose();
   pane.view = null;
   pane.ready = false;
   pane.doc = doc;
   pane.top = 0;
   pane.ranges = [];
   pane.mapping = null;
   pane.requested = null;
   pane.gutter.replaceChildren();
   pane.colors.replaceChildren();
   pane.gutter.style.paddingTop = '';
   pane.gutter.style.transform = '';
   ta.value = '';
   pane.text = '';
   pane.lines = [0];
   if (doc?.disk && !doc.staged) {
    pane.view = options.mountDiskView(doc, {
     textarea:ta, gutter:pane.gutter, container:pane.container, savedView:doc[viewField] || doc.view,
     changed: () => changed(side)
    });
    const view = pane.view;
    void view.ready.then(() => { if (pane.view === view && !disposed) pane.ready = true; }).catch(() => {});
   }
  }
  const blocked = !!(doc?.loading || doc?.frozen || doc?.reloading || doc?.promoting || doc?.staged);
  if (!doc?.disk) {
   ta.disabled = blocked || (!doc && (comparing || options.documents().length >= 12));
   ta.placeholder = doc ? '' : comparing ? 'Choose a document to compare.' : 'Type here or move a tab to this view.';
   ta.wrap = options.wordWrap?.() ? 'soft' : 'off';
   ta.classList.toggle('is-wrapped', ta.wrap === 'soft');
   const value = doc?.content || '';
   if (ta.value !== value) ta.value = value;
   indexInline(pane, value);
   if (changedDocument && doc) {
    const saved = doc[viewField] || doc.view;
    if (saved) {
     ta.setSelectionRange(saved.start, saved.end, saved.direction);
     ta.scrollTop = saved.top;
     ta.scrollLeft = saved.left;
    }
   }
  } else if (doc.staged || blocked) {
   ta.disabled = true;
  } else if (pane.ready) {
   ta.disabled = false;
  }
  pane.view?.refreshSyntax();
  const state = q('[data-compare-state="' + side + '"]');
  state.textContent = doc ? doc.source + ' · ' + doc.path + (doc.dirty ? ' · Unsaved' : ' · Saved') +
   (doc.saving ? ' · Saving…' : '') : 'Open or create a second document.';
  pane.section.classList.toggle('is-active-view', side === activeSide());
 }

 function paintInlineGutter(pane) {
  const height = lineHeight(pane), top = Math.floor(pane.textarea.scrollTop / height);
  const from = Math.max(0, top - 3), end = Math.min(pane.lines.length, top + linesInView(pane));
  pane.top = top;
  pane.gutter.style.paddingTop = from * height + 'px';
  pane.gutter.style.transform = 'translateY(' + (-pane.textarea.scrollTop) + 'px)';
  const rows = [];
  for (let line = from; line < end; line++) {
   const row = document.createElement('div');
   row.style.height = height + 'px';
   row.style.lineHeight = height + 'px';
   row.textContent = String(line + 1);
   rows.push(row);
  }
  pane.gutter.replaceChildren(...rows);
 }

 function differenceKind(side, line, ranges) {
  for (const diff of ranges) {
   const start = diff[side], count = diff[side + 'Count'];
   if (line >= start && line < start + count) {
    const peerCount = diff[peerOf(side) + 'Count'];
    return line - start < peerCount ? 'changed' : side === 'left' ? 'removed' : 'added';
   }
  }
  return '';
 }

 function paintDifferences(side) {
  const pane = panes[side], rows = pane.gutter.children, colors = [];
  const first = rows[0];
  const bounds = first && (pane.ranges.length || lineHighlight?.lines[side] != null) ? first.getBoundingClientRect() : null;
  const top = bounds ? bounds.top - pane.colors.getBoundingClientRect().top : 0;
  let index = 0;
  const intraline = q('[data-compare-intraline]').checked;
  const budget = {remaining:8192};
  for (const row of rows) {
   const line = Number(row.textContent) - 1;
   const kind = differenceKind(side, line, pane.ranges);
   const className = kind ? 'nt-compare-line ' + kind : '';
   if (row.className !== className) row.className = className;
   if (kind) row.dataset.differenceSymbol = differenceSymbols[kind];
   else delete row.dataset.differenceSymbol;
   const offset = index++;
   const current = lineHighlight?.lines[side] === line;
   if ((!kind && !current) || !bounds) continue;
   const color = document.createElement('span');
   color.className = 'nt-disk-compare-color ' + kind + (current ? ' is-current' : '');
   color.style.top = top + offset * bounds.height + 'px';
   color.style.height = bounds.height + 'px';
   if (intraline && kind === 'changed') paintCharacters(side, line, color, budget);
   colors.push(color);
  }
  if (colors.length || pane.colors.childElementCount) pane.colors.replaceChildren(...colors);
 }

 function visibleLine(pane, line) {
  const page = pane.view?.comparisonPage() || {line:0, text:pane.text, column:0, complete:true};
  if (pane.characterText?.text !== page.text) pane.characterText = {text:page.text, lines:page.text.split('\n')};
  const lines = pane.characterText.lines, local = line - page.line;
  if ((local === 0 && page.column) || (local === lines.length - 1 && !page.complete)) return null;
  return lines[local] ?? null;
 }

 function paintCharacters(side, line, row, budget) {
  const peer = peerOf(side), pane = panes[side];
  const diff = pane.ranges.find(value => line >= value[side] && line - value[side] < value[side + 'Count']);
  if (!diff) return;
  const text = visibleLine(pane, line), other = visibleLine(panes[peer], diff[peer] + line - diff[side]);
  if (text == null || other == null || Math.max(text.length, other.length) > 4096) return;
  const ranges = root.NTKEditorAlignmentModel?.intralineRanges(text, other, budget).left;
  if (!ranges?.length) return;
  const overlay = document.createElement('span');
  overlay.className = 'nt-disk-compare-intraline';
  overlay.style.transform = 'translateX(' + (-pane.textarea.scrollLeft) + 'px)';
  let from = 0;
  for (const range of ranges) {
   overlay.append(text.slice(from, range.from));
   const mark = document.createElement('span');
   mark.textContent = text.slice(range.from, range.to);
   overlay.append(mark);
   from = range.to;
  }
  row.append(overlay);
 }

 function highlightLine(side) {
  if (!comparing || disposed || composing()) return;
  const pane = panes[side];
  if (!pane.doc) return;
  const cursor = pane.textarea.selectionDirection === 'backward' ? pane.textarea.selectionStart : pane.textarea.selectionEnd;
  let line = pane.view?.cursor.line;
  if (line === undefined) {
   let first = 0, end = pane.lines.length;
   while (first + 1 < end) {
    const mid = Math.floor((first + end) / 2);
    if (pane.lines[mid] <= cursor) first = mid;
    else end = mid;
   }
   line = first;
  }
  const current = job, peer = peerOf(side);
  if (lineHighlight?.side === side && lineHighlight.lines[side] === line && lineHighlight.job === current) return;
  const highlight = {side, lines:{[side]:line}, job:current};
  lineHighlight = highlight;
  for (const name of sides) paintDifferences(name);
  if (!current?.status || line >= current.status[side + 'Lines']) return;
  void options.compareRequest({action:'locate', id:current.id, side, line, end:line, limit:1}, current.abort.signal).then(value => {
   if (disposed || highlight !== lineHighlight || current !== job || !value.aligned) return;
   const diff = value.differences[0], relative = line - (diff?.[side] ?? line);
   if (diff && relative >= diff[peer + 'Count'] && relative < diff[side + 'Count']) return;
   highlight.lines[peer] = value.peerLine;
   paintDifferences(peer);
  }).catch(() => {});
 }

 function paintMap() {
  if (!comparing || disposed) return;
  readMapScroll();
  const atEnd = mapExtent > 0 && mapTop === mapExtent;
  const status = job?.status;
  const height = Math.max(1, map.clientHeight), pitch = mapPitch();
  const fit = scale === -3, step = scale === -2 ? 3 : 1;
  const contentHeight = fit ? height : Math.max(height, Math.ceil(pitch * mapLineCount()));
  // Keep the native scrollbar within browser layout limits. The wheel and
  // visible marks still use the full, unscaled line coordinates.
  mapExtent = contentHeight - height;
  mapContent.style.height = Math.min(contentHeight, Math.max(mapScrollRange, height)) + 'px';
  mapContent.style.setProperty('--nt-disk-map-height', height + 'px');
  mapScrollMax = map.scrollHeight - map.clientHeight;
  setMapTop(atEnd ? mapExtent : mapTop);
  const first = Math.floor(mapTop / (pitch * step)) * step;
  const end = Math.ceil((mapTop + height) / (pitch * step)) * step;
  const revision = status?.count ? job.id + ':' + status.count : '';
  const data = mapData?.revision === revision ? mapData : null;
  const paintKey = [revision, data?.start, first, end, pitch, height].join(':');
  for (const side of sides) {
   const pane = panes[side], count = totalLines(pane);
   pane.track.setAttribute('aria-valuemin', '1');
   pane.track.setAttribute('aria-valuemax', String(count));
   pane.track.setAttribute('aria-valuenow', String(clamp(pane.top + 1, 1, count)));
   pane.track.setAttribute('aria-disabled', String(!pane.doc));
   pane.marker.style.top = pane.top * pitch - mapTop + 'px';
   pane.marker.style.height = Math.max(1, Math.min(linesInView(pane), count - pane.top) * pitch) + 'px';
   pane.markers.style.transform = fit ? '' : 'translateY(' + (first * pitch - mapTop) + 'px)';
   if (paintKey !== mapPaintKey) {
    pane.markers.setAttribute('viewBox', '0 0 1 ' + height);
    const marks = new Map();
    if (fit) {
     const bins = status?.[side + 'Overview'] || [];
     for (let i = 0; i < bins.length; i++) {
      const y = Math.floor(i / bins.length * height);
      if (bins[i]) marks.set(y, Math.max(marks.get(y) || 0, bins[i]));
     }
    } else if (data) {
     for (let line = first; line < end; line += step) {
      let kind = 0;
      for (let row = line; row < line + step; row++) kind = Math.max(kind, data[side][row - data.start] || 0);
      if (kind) marks.set((line - first) * pitch, kind);
     }
    }
    pane.markers.innerHTML = Array.from(marks, ([y, kind]) => '<rect width="1" height="' +
     (fit || step > 1 ? 1 : pitch - 1) + '" y="' + y + '" class="' + ['', 'removed', 'added', 'changed'][kind] + '"/>').join('');
   }
  }
  mapPaintKey = paintKey;
  if (!fit && revision && !mapRequest && (!data || first < data.start || end > data.end)) {
   const start = Math.max(0, first - 256), stop = Math.min(start + 32768, end + 256);
   const current = job;
   const request = options.compareRequest({action:'map', id:current.id, start, end:stop}, current.abort.signal);
   mapRequest = request;
   void request.then(values => {
    if (job === current) {
     mapData = {...values, start, end:stop, revision};
     scheduleMap();
    }
   }).catch(() => {}).finally(() => {
    if (mapRequest === request) mapRequest = null;
   });
  }
 }

 function setMapTop(top) {
  mapTop = clamp(top, 0, mapExtent);
  const position = mapExtent ? mapTop / mapExtent * mapScrollMax : 0;
  if (Math.abs(map.scrollTop - position) >= .5) map.scrollTop = position;
  mapScrollPosition = map.scrollTop;
 }

 function readMapScroll() {
  const position = map.scrollTop;
  if (position === mapScrollPosition) return false;
  // A resize can clamp the native scroll position before the map is repainted.
  const currentMax = map.scrollHeight - map.clientHeight;
  mapTop = currentMax ? clamp(position / currentMax, 0, 1) * mapExtent : 0;
  mapScrollPosition = position;
  return true;
 }

 function revealMap(side, line = panes[side].top) {
  const y = line * mapPitch();
  if (y < mapTop || y >= mapTop + map.clientHeight) {
   setMapTop(y - map.clientHeight / 2);
   scheduleMap();
  }
 }

 function scheduleMap() {
  if (comparing && !mapFrame) mapFrame = root.requestAnimationFrame(() => {
   mapFrame = 0;
   paintMap();
  });
 }

 function changed(side) {
  if (disposed) return;
  const pane = panes[side];
  const previousTop = pane.top;
  if (pane.view) {
   const view = pane.view.capture();
   pane.top = Math.max(0, view.top || 0);
   pane.fraction = view.fraction || 0;
  } else {
   paintInlineGutter(pane);
   pane.fraction = pane.textarea.scrollTop / lineHeight(pane) - pane.top;
  }
  remember(side);
  paintDifferences(side);
  scheduleMap();
  if (pane.top !== previousTop && gestureSide === side && drag === null) revealMap(side);
  const sync = !syncing && gestureSide === side && q('[data-compare-sync]').checked;
  if (sync) syncFromCache(side);
  if (pane.top !== previousTop && job?.status && !coversViewport(side)) queueLocate(side, sync);
  if (activeSide() === side) options.selectionChanged?.();
 }

 async function goLine(side, line, fraction = 0) {
  const pane = panes[side];
  if (!pane.doc) return;
  const target = clamp(Math.floor(line), 0, totalLines(pane) - 1);
  if (pane.view) {
   const view = pane.view;
   await view.ready;
   if (pane.view === view && !disposed) await view.goLine(target, false, false, fraction);
  } else {
   pane.textarea.scrollTop = (target + fraction) * lineHeight(pane);
   changed(side);
  }
 }

 function coversViewport(side) {
  const pane = panes[side], mapping = pane.mapping;
  return !!mapping && mapping.job === job && mapping.count === job.status.count &&
   pane.top >= mapping.top && Math.min(totalLines(pane) - 1, pane.top + linesInView(pane)) <= mapping.end;
 }

 function syncFromCache(side) {
  const pane = panes[side], mapping = pane.mapping, peer = peerOf(side);
  if (!coversViewport(side) || pane.top >= job.status[side + 'Lines']) return;
  let line = mapping.peerLine + pane.top - mapping.top;
  for (const diff of pane.ranges) {
   const start = diff[side];
   if (start > pane.top) break;
   const relative = pane.top - start, count = diff[side + 'Count'], peerCount = diff[peer + 'Count'];
   line = diff[peer] + (relative < count ? Math.min(relative, Math.max(0, peerCount - 1)) : peerCount + relative - count);
  }
  line = clamp(line, 0, totalLines(panes[peer]) - 1);
  const fraction = pane.fraction || 0;
  if (panes[peer].top === line && Math.abs((panes[peer].fraction || 0) - fraction) < 0.03) return;
  const topAtSync = pane.top;
  syncing++;
  go(goLine(peer, line, fraction).finally(() => {
   syncing--;
   if (!disposed && gestureSide === side && q('[data-compare-sync]').checked &&
       (pane.top !== topAtSync || pane.fraction !== fraction)) {
    syncFromCache(side);
   }
  }));
 }

 function queueLocate(side, sync = false) {
  const pane = panes[side], current = job;
  if (!current?.status || !pane.doc || current.status.error) return;
  // Reuse the mapping while scrolling within a small band around the viewport.
  const top = Math.max(0, pane.top - 16), end = pane.top + linesInView(pane) + 48;
  const key = [current.id, current.status.count, current.status[side + 'Lines'] > top, top, end].join(':');
  if (pane.requested?.key === key) {
   pane.requested.sync ||= sync;
   return;
  }
  pane.requested = {key, top, end, sync, job:current, count:current.status.count};
  if (pane.locateTimer !== null || pane.pending) return;
  pane.locateTimer = root.setTimeout(() => {
   pane.locateTimer = null;
   void locate(side);
  }, 0);
 }

 async function locate(side) {
  const pane = panes[side], request = pane.requested;
  if (!request || request.job !== job || disposed) return;
  pane.pending = request;
  try {
   const result = await options.compareRequest({
    action:'locate', id:request.job.id, side, line:request.top, end:request.end, limit:pageSize
   }, request.job.abort.signal);
   if (job !== request.job) return;
   const end = result.differences.length === pageSize ? result.differences.at(-1)[side] : request.end;
   if (pane.requested !== request && (pane.top < request.top || pane.top > end)) return;
   pane.ranges = result.differences;
   pane.mapping = result.aligned ? {...request, peerLine:result.peerLine, end} : null;
   paintDifferences(side);
   if (result.aligned && (request.sync || pane.requested?.sync) && gestureSide === side && !syncing && q('[data-compare-sync]').checked) {
    syncFromCache(side);
   }
  } catch (error) {
   if (job === request.job && !request.job.abort.signal.aborted) failJob(request.job, error);
  } finally {
   pane.pending = null;
   if (pane.requested !== request && pane.requested?.job === job && !disposed && !coversViewport(side)) void locate(side);
  }
 }

 function clearJob() {
  root.clearTimeout(startTimer);
  startTimer = null;
  const previous = job;
  job = null;
  selected = -1;
  delete summary.dataset.compareDone;
  delete summary.dataset.compareCount;
  delete summary.dataset.compareSelected;
  if (previous) {
   previous.abort.abort();
   root.clearTimeout(previous.timer);
   if (previous.sent) void options.compareRequest({action:'cancel', id:previous.id}).catch(() => {});
  }
  mapData = mapRequest = null;
  lineHighlight = null;
  preview?.refresh?.();
  for (const side of sides) {
   const pane = panes[side];
   pane.ranges = [];
   pane.mapping = null;
   pane.requested = null;
   root.clearTimeout(pane.locateTimer);
   pane.locateTimer = null;
   paintDifferences(side);
  }
  paintMap();
  buttons();
 }

 function buttons() {
  const enabled = !!job?.status?.count && !job.status.error;
  for (const button of host.querySelectorAll(root.NTKEditorCompare.differenceControls)) button.disabled = !enabled;
  q('[data-compare="compute"]').textContent = job && !job.status?.done ? 'Cancel comparison' : 'Compare';
 }

 function failJob(current, error) {
  if (job !== current || disposed) return;
  clearJob();
  setSummary(error.message);
 }

 function applyStatus(current, status) {
  if (job !== current || disposed) return;
  const justFinished = status.done && !current.status?.done;
  const countChanged = current.status?.count !== status.count;
  current.status = status;
  if (status.error) {
   failJob(current, Error(status.error));
   return;
  }
  summary.dataset.compareDone = String(status.done);
  summary.dataset.compareCount = String(status.count);
  if (selected < 0) {
   setSummary(status.done ?
    (status.count ? number(status.count) + ' differences. Use Previous and Next to navigate.' : 'The documents are identical.') :
    'Comparing… ' + number(Math.floor(100 * (status.leftBytes + status.rightBytes) / Math.max(1, status.leftSize + status.rightSize))) +
    '% · ' + number(status.count) + ' differences found.');
  }
  buttons();
  if (justFinished || countChanged) preview.refresh();
  if (justFinished && q('[data-compare-sync]').checked) gestureSide ||= activeSide();
  for (const side of sides) queueLocate(side, justFinished && gestureSide === side);
  scheduleMap();
  root.clearTimeout(current.timer);
  current.timer = root.setTimeout(() => void poll(current), status.done ? 30000 : 250);
 }

 async function poll(current) {
  if (job !== current || disposed) return;
  try {
   const status = await options.compareRequest({action:'status', id:current.id}, current.abort.signal);
   applyStatus(current, status);
  } catch (error) {
   failJob(current, error);
  }
 }

 function readyDocuments() {
  return sides.every(side => {
   const d = panes[side].doc;
   return d && !d.loading && !d.staged && !d.frozen && !d.reloading && !d.promoting;
  }) && panes.left.doc !== panes.right.doc;
 }

 async function compute() {
  clearJob();
  if (!comparing || !readyDocuments() || composing()) {
   setSummary('Choose two open documents to compare.');
   return;
  }
  const id = Array.from(root.crypto.getRandomValues(new Uint8Array(16)), value => value.toString(16).padStart(2, '0')).join('');
  const current = {id, abort:new AbortController(), timer:null, status:null, pages:new Map(), sent:false};
  const documents = sides.map(side => panes[side].doc);
  job = current;
  buttons();
  setSummary('Preparing comparison…');
  try {
   const sources = await Promise.all(documents.map(async (d, i) => {
    if (!d.disk) return {text:d.content};
    await panes[sides[i]].view?.ready;
    const session = options.ensureDiskSession(d);
    await session.flush();
    return {uid:d.uid, revision:session.state().metadata.revision};
   }));
   if (job !== current || disposed) return;
   current.sent = true;
   const status = await options.compareRequest({action:'start', id, left:sources[0], right:sources[1], spaces:q('[data-compare-spaces]').value}, current.abort.signal);
   applyStatus(current, status);
  } catch (error) {
   failJob(current, error);
  }
 }

 async function getDifference(current, index, signal = current.abort.signal) {
  const start = Math.floor(index / pageSize) * pageSize;
  let page = current.pages.get(start);
  if (!page || !page[index - start]) {
   const value = await options.compareRequest({action:'page', id:current.id, start, limit:pageSize}, signal);
   if (job !== current || disposed) return;
   page = value.differences;
   current.pages.set(start, page);
   if (current.pages.size > 3) current.pages.delete(current.pages.keys().next().value);
  }
  return page[index - start];
 }

 async function readDifferencePreview(index, offset, limit, signal) {
  const current = job, diff = await getDifference(current, index, signal);
  if (!diff || job !== current) return;
  const read = async side => {
   const d = panes[side].doc, count = Math.max(0, Math.min(limit, diff[side + 'Count'] - offset));
   const line = diff[side] + offset, end = line + count;
   if (!count) return [];
   const extract = root.NTKEditorCompare.previewSourceLines;
   if (!d.disk) return extract(d.content, line, count);
   const session = options.ensureDiskSession(d), values = [];
   for (let first = line; first < end;) {
    signal.throwIfAborted();
    const page = await session.previewWindow(first, signal);
    const rows = extract(page.text, first, end - first, page.line);
    const last = rows.at(-1);
    if (!last) throw Error('Could not read the changed lines.');
    if (last.line === page.line + page.text.split('\n').length - 1 &&
        page.start + page.text.length < page.units) {
     if (rows.length > 1) rows.pop();
     else last.text += ' … (open the editable comparison to view the full line)';
    }
    values.push(...rows);
    first = rows.at(-1).line + 1;
   }
   return values;
  };
  const [left, right] = await Promise.all(sides.map(read));
  return {left, right, length:Math.max(diff.leftCount, diff.rightCount)};
 }

 async function difference(step) {
  const current = job;
  const count = current?.status?.count || 0;
  if (!count) return;
  const index = root.NTKEditorCompare.differenceIndex(selected, step, count);
  const diff = await getDifference(current, index);
  if (!diff || job !== current) return;
  syncing++;
  try { await Promise.all(sides.map(side => goLine(side, diff[side]))); }
  finally { syncing--; }
  if (job !== current || disposed) return;
  revealMap(activeSide());
  selected = index;
  preview.refresh();
  summary.dataset.compareSelected = String(index);
  setSummary('Difference ' + number(index + 1) + ' of ' + number(count) + ' · ' +
   number(diff.leftCount) + ' left / ' + number(diff.rightCount) + ' right lines.');
  for (const side of sides) {
   panes[side].ranges = [diff];
   paintDifferences(side);
   panes[side].requested = null;
   queueLocate(side);
  }
 }

 function refresh() {
  if (disposed) return;
  const docs = options.documents();
  ids = options.selection?.() || ids;
  const listKey = JSON.stringify(docs.map(d => [d.id, d.path, d.source, d.dirty]));
  for (const side of sides) {
   const pane = panes[side];
   if (listKey !== listSignature) {
    const list = [new Option('Choose document', '')];
    for (const d of docs) list.push(new Option((d.dirty ? '* ' : '') + d.path + ' · ' + d.source, d.id));
    pane.select.replaceChildren(...list);
   }
   pane.select.value = ids[side];
   updatePane(side, docs.find(d => d.id === ids[side]) || null);
   changed(side);
  }
  listSignature = listKey;
  const key = JSON.stringify(sides.map(side => {
   const d = panes[side].doc;
   return d ? [d.uid, d.editRevision, d.loading, d.frozen, d.reloading, d.promoting] : null;
  }));
  if (comparing && key !== signature) {
   signature = key;
   clearJob();
   if (readyDocuments()) {
    setSummary('Preparing comparison…');
    startTimer = root.setTimeout(() => void compute(), 300);
   } else setSummary('Choose two open documents to compare.');
  }
 }

 function showDocument(id) {
  const doc = options.documents().find(d => d.id === id);
  if (!doc || composing()) return false;
  lastSide = options.sideOf?.(doc.uid) || lastSide;
  ids[lastSide] = id;
  refresh();
  const pane = panes[lastSide];
  if (pane.view) void pane.view.ready.then(() => {
   if (pane.doc === doc && options.active()?.id === id) pane.textarea.focus({preventScroll:true});
  }).catch(() => {});
  else pane.textarea.focus({preventScroll:true});
  return true;
 }

 function undo(redo = false) {
  const pane = panes[activeSide()];
  if (!pane.doc || pane.textarea.disabled) return;
  if (pane.view) go(pane.view.history(redo));
  else {
   options.undo(pane.doc, redo);
   refresh();
  }
 }

 async function inlineInput(side) {
  const pane = panes[side];
  if (pane.view || pane.composing || pane.creating) return;
  const ta = pane.textarea;
  const text = ta.value;
  let d = pane.doc;
  if (!d && !comparing) {
   pane.creating = true;
   try {
    d = await options.createInView?.(side);
    if (!d || disposed) return;
    refresh();
    if (pane.view) {
     await pane.view.ready;
     if (!disposed && pane.doc === d) await pane.view.replace(text);
     return;
    }
   } finally { pane.creating = false; }
  }
  if (d && options.edit(d, text, ta) === false && !d.promoting) ta.value = d.content;
  refresh();
 }

 function pointOnMap(event, side) {
  const pane = panes[side];
  const line = clamp(Math.floor((event.clientY - map.getBoundingClientRect().top + mapTop) / mapPitch()), 0, totalLines(pane) - 1);
  const halfView = Math.floor(pane.textarea.clientHeight / lineHeight(pane) / 2);
  gestureSide = side;
  go(goLine(side, line - halfView));
  return line;
 }

 map.addEventListener('scroll', () => { if (readMapScroll()) scheduleMap(); });
 map.addEventListener('wheel', event => {
  if (!event.deltaY || event.ctrlKey) return;
  const unit = event.deltaMode === 1 ? 20 : event.deltaMode === 2 ? map.clientHeight : 1;
  event.preventDefault();
  if (!mapExtent) {
   const side = event.target.closest('[data-compare-track]')?.dataset.compareTrack || activeSide();
   const pane = panes[side];
   const line = Math.max(0, pane.top + (pane.fraction || 0) + event.deltaY * unit / lineHeight(pane));
   gestureSide = side;
   go(goLine(side, line, line % 1));
   return;
  }
  setMapTop(mapTop + event.deltaY * unit);
  scheduleMap();
 }, {passive:false});

 const horizontal = root.NTKEditorCompare.createHorizontalSync(side => {
  const ta = panes[side].textarea;
  return {left:ta.scrollLeft, width:ta.scrollWidth, client:ta.clientWidth};
 }, (side, left) => { panes[side].textarea.scrollLeft = left; }, () => comparing && q('[data-compare-horizontal-sync]').checked);

 for (const side of sides) {
  const pane = panes[side], ta = pane.textarea, track = pane.track;
  pane.select.addEventListener('change', () => {
   if (composing()) { pane.select.value = ids[side]; return; }
   if (pane.select.value) options.choose?.(side, pane.select.value);
   refresh();
  });
  ta.addEventListener('focus', () => {
   if (!pane.doc) return;
   lastSide = side;
   options.activate(pane.doc, side);
   for (const name of sides) panes[name].section.classList.toggle('is-active-view', name === side);
  });
  const gesture = () => { gestureSide = side; horizontal.input(side); };
  pane.container.addEventListener('pointerdown', gesture);
  pane.container.addEventListener('wheel', gesture, {passive:true});
  pane.container.addEventListener('keydown', gesture, true);
  ta.addEventListener('scroll', () => { horizontal.scroll(side); if (!pane.view) changed(side); });
  ta.addEventListener('select', () => { if (!pane.view) changed(side); });
  for (const event of ['click', 'keyup']) ta.addEventListener(event, () => highlightLine(side));
  ta.addEventListener('compositionstart', () => { if (!pane.view) pane.composing = true; });
  ta.addEventListener('compositionend', () => {
   if (!pane.view) { pane.composing = false; go(inlineInput(side)); }
  });
  ta.addEventListener('input', () => go(inlineInput(side)));
  ta.addEventListener('keydown', event => {
   if (event.defaultPrevented || event.isComposing || !comparing) return;
   const key = event.key.toLowerCase(), ctrl = event.ctrlKey || event.metaKey;
   if (ctrl && key === 's' && pane.doc) {
    event.preventDefault();
    event.stopPropagation();
    go(Promise.resolve(options.save(pane.doc, event.shiftKey)));
   } else if (ctrl && (key === 'z' || key === 'y')) {
    event.preventDefault();
    event.stopPropagation();
    undo(key === 'y' || event.shiftKey);
   } else if (key === 'tab' && !ta.disabled && pane.doc) {
    event.preventDefault();
    if (pane.view) go(pane.view.replace('\t'));
    else {
     ta.setRangeText('\t', ta.selectionStart, ta.selectionEnd, 'end');
     go(inlineInput(side));
    }
   }
  });
  track.addEventListener('pointerdown', event => {
   if (!pane.doc || event.button !== 0) return;
   event.preventDefault();
   drag = {id:event.pointerId, line:0, y:event.clientY};
   track.setPointerCapture?.(event.pointerId);
   track.focus({preventScroll:true});
   drag.line = pointOnMap(event, side);
  });
  track.addEventListener('pointermove', event => { if (drag?.id === event.pointerId) pointOnMap(event, side); });
  track.addEventListener('pointerup', event => {
   if (drag?.id !== event.pointerId || Math.abs(event.clientY - drag.y) > 3) return;
   setMapTop((drag.line + .5) * mapPitch() - map.clientHeight / 2);
   scheduleMap();
  });
  track.addEventListener('lostpointercapture', () => { drag = null; });
  track.addEventListener('keydown', event => {
   const targets = {Home:0, End:totalLines(pane) - 1, ArrowUp:pane.top - 1, ArrowDown:pane.top + 1,
    PageUp:pane.top - linesInView(pane), PageDown:pane.top + linesInView(pane)};
   if (!Object.hasOwn(targets, event.key)) return;
   event.preventDefault();
   gestureSide = side;
   revealMap(side, clamp(targets[event.key], 0, totalLines(pane) - 1));
   go(goLine(side, targets[event.key]));
  });
 }

 host.addEventListener('click', event => {
  const action = event.target.closest('[data-compare]')?.dataset.compare;
  if (action === 'compute') {
   if (job && !job.status?.done) { clearJob(); setSummary('Comparison cancelled.'); }
   else void compute();
  }
  if (action === 'swap' && !composing()) { options.swap?.(); refresh(); }
  if (action === 'previous' || action === 'next') go(difference(action === 'next' ? 1 : -1));
  if (action === 'first' || action === 'last') go(difference(action));
  if (action === 'close') options.close?.();
 });
 host.addEventListener('change', event => {
  const control = event.target;
  if (control.matches('[data-compare-spaces]')) void compute();
  if (control.matches('[data-compare-scale]')) {
   const topLine = mapTop / mapPitch();
   scale = Number(control.value) || 0;
   mapTop = topLine * mapPitch();
   paintMap();
  }
  if (control.matches('[data-compare-sync]') && control.checked) {
   panes[activeSide()].requested = null;
   gestureSide = activeSide();
   queueLocate(activeSide(), true);
  }
  if (control.matches('[data-compare-horizontal-sync]')) {
   options.setComparisonPreference?.('compareHorizontalSync', control.checked);
   horizontal.sync(activeSide());
  }
  if (control.matches('[data-compare-underline]')) {
   options.setComparisonPreference?.('compareUnderline', control.checked);
   host.classList.toggle('nt-disk-compare-underline', control.checked);
  }
  if (control.matches('[data-compare-intraline]')) {
   options.setComparisonPreference?.('compareIntralineHighlight', control.checked);
   for (const side of sides) paintDifferences(side);
  }
 });
 host.classList.toggle('nt-disk-compare-underline', q('[data-compare-underline]').checked);
 const observer = typeof ResizeObserver === 'function' ? new ResizeObserver(() => {
  splitter.apply();
  for (const side of sides) changed(side);
 }) : null;
 observer?.observe(q('.nt-compare-grid'));
 if (comparing) observer?.observe(map);
 splitter.apply();
 refresh();

 return {
  refresh, showDocument, undo, difference,
  get textarea() { return panes[activeSide()].textarea; },
  get activeSide() { return lastSide; },
  get composing() { return composing(); },
  get result() { return job?.status || null; },
  get selection() { return {...ids}; },
  diskView: side => panes[side || activeSide()].view,
  scrollActive() { changed(activeSide()); },
  positionAt(offset, focus = true) {
   const pane = panes[activeSide()];
   if (pane.view) go(pane.view.position(offset, offset, focus));
   else {
    pane.textarea.setSelectionRange(offset, offset);
    if (focus) pane.textarea.focus({preventScroll:true});
    let low = 0, high = pane.lines.length;
    while (low + 1 < high) {
     const mid = Math.floor((low + high) / 2);
     if (pane.lines[mid] <= offset) low = mid;
     else high = mid;
    }
    pane.textarea.scrollTop = low * lineHeight(pane);
    changed(activeSide());
   }
  },
  snapshot() {
   for (const side of sides) remember(side);
   return {...ids, scale, sync:q('[data-compare-sync]').checked};
  },
  restore(value) {
   if (!value || composing()) return;
   const active = options.active();
   ids = {left:String(value.left || ''), right:String(value.right || '')};
   for (const side of sides) if (ids[side]) options.choose?.(side, ids[side]);
   if (active && sides.some(side => ids[side] === active.id)) options.activate(active, options.sideOf?.(active.uid));
   scale = [-3, -2, 0, .5, 1, 2, 4].includes(value.scale) ? value.scale : 0;
   q('[data-compare-scale]').value = String(scale);
   q('[data-compare-sync]').checked = value.sync !== false;
   refresh();
  },
  dispose() {
   if (disposed) return;
   clearJob();
   disposed = true;
   preview?.dispose?.();
   splitter.dispose();
   observer?.disconnect();
   if (mapFrame) root.cancelAnimationFrame(mapFrame);
   for (const side of sides) {
    remember(side);
    panes[side].view?.dispose();
   }
   host.replaceChildren();
  }
 };
}

root.NTKEditorDiskCompare = Object.freeze({create});
})(globalThis);
