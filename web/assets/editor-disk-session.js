/* A disk document keeps a bounded viewport and any unacknowledged edit here.
   The authoritative document and undo history stay in the local engine. */
(function (root) {
'use strict';

const PAGE_UNITS = 49152;
const PAGE_LINES = 1024;
const CHUNK_UNITS = 128 * 1024;
const metadataKeys = ['revision', 'units', 'bytes', 'lines', 'dirty', 'canUndo', 'canRedo'];

function countLines(text) {
 let count = 0;
 for (let at = -1; (at = text.indexOf('\n', at + 1)) >= 0;) count++;
 return count;
}

function splitsPair(text, at) {
 if (at <= 0 || at >= text.length) return false;
 const before = text.charCodeAt(at - 1);
 const after = text.charCodeAt(at);
 return before >= 0xd800 && before <= 0xdbff && after >= 0xdc00 && after <= 0xdfff;
}

function fragment(before, after) {
 let from = 0, to = before.length, end = after.length;
 while (from < to && from < end && before.charCodeAt(from) === after.charCodeAt(from)) from++;
 if (splitsPair(before, from) || splitsPair(after, from)) from--;
 while (to > from && end > from && before.charCodeAt(to - 1) === after.charCodeAt(end - 1)) {
  to--;
  end--;
 }
 if (splitsPair(before, to) || splitsPair(after, end)) {
  to++;
  end++;
 }
 return {from, to, text: after.slice(from, end)};
}

const emptyPage = () => ({start: 0, line: 0, column: 0, lineUnits: 0, text: ''});
const createIdentity = () => Array.from(
 root.crypto.getRandomValues(new Uint8Array(16)),
 n => n.toString(16).padStart(2, '0')
).join('');

function create({request, exportRange, changed = () => {}, failed = () => {}, identity = createIdentity}) {
 let metadata = {revision: 0, units: 0, bytes: 0, lines: 1, dirty: false, canUndo: false, canRedo: false};
 let page = emptyPage();
 let acknowledged = '', pageLines = 0, acknowledgedLines = 0;
 let flushing = null, failure = null, closed = false, epoch = 0, editVersion = 0, readTicket = 0;
 let retry = null, external = null, scheduled = false, reloading = false;
 let reading = null;
 const cache = new Map();
 const prefetching = new Map();

 function hasChanges() {
  return !!retry || !!external || page.text !== acknowledged;
 }

 function state() {
  return {
   metadata: {
    ...metadata,
    units: metadata.units + page.text.length - acknowledged.length,
    lines: metadata.lines + pageLines - acknowledgedLines,
    dirty: metadata.dirty || hasChanges(),
    canUndo: metadata.canUndo || page.text !== acknowledged
   },
   page: {...page},
   editVersion,
   pending: !!flushing || hasChanges(),
   blocked: !!external || reloading,
   error: failure
  };
 }

 function notify() {
  if (!closed) changed(state());
 }

 function ensure() {
  if (closed) throw Error('The document buffer is no longer attached to this editor.');
 }

 function receive(value) {
  if (!value || !Number.isSafeInteger(value.revision) || !Number.isSafeInteger(value.units) ||
      !Number.isSafeInteger(value.lines)) throw Error('The document engine returned invalid metadata.');
  metadata = Object.fromEntries(metadataKeys.map(key => [key, value[key]]));
 }

 function clearCache() {
  cache.clear();
  for (const job of prefetching.values()) job.controller.abort();
  prefetching.clear();
 }

 function viewport(value) {
  if (value.loading) throw Error('The file is still opening.');
  if (typeof value.text !== 'string' || !Number.isSafeInteger(value.start) ||
      !Number.isSafeInteger(value.line)) throw Error('The document engine returned an invalid viewport.');
  let end = -1;
  for (let line = 0; line < PAGE_LINES; line++) {
   end = value.text.indexOf('\n', end + 1);
   if (end < 0) break;
  }
  return {
   start: value.start, line: value.line, column: value.column || 0,
   lineUnits: value.lineUnits || 0, text: end < 0 ? value.text : value.text.slice(0, end)
  };
 }

 function install(value, next = viewport(value)) {
  receive(value);
  page = next;
  acknowledged = page.text;
  pageLines = acknowledgedLines = countLines(page.text);
  failure = null;
  epoch++;
 }

 async function mutate(action, payload) {
  // Reuse the exact request identity and payload after an uncertain response.
  let error;
  for (let attempt = 0; attempt < 2; attempt++) {
   try {
    return await request(action, payload);
   } catch (e) {
    error = e;
    if (e.status && e.status < 500) break;
   }
  }
  throw error;
 }

 async function applyOperation(operation) {
  const {payload} = operation;
  if (payload.action !== 'edit' || payload.text.length <= CHUNK_UNITS) {
   return mutate(payload.action, payload);
  }

  // Paste remains one undo operation. The live tree changes only on commit.
  const {requestId, revision, from, to, text} = payload;
  if (!operation.begun) {
   await mutate('edit-begin', {requestId, revision, from, to});
   operation.begun = true;
   operation.offset = 0;
  }
  while (operation.offset < text.length) {
   ensure();
   const offset = operation.offset;
   let end = Math.min(text.length, offset + CHUNK_UNITS);
   if (splitsPair(text, end)) end--;
   const result = await mutate('edit-chunk', {requestId, offset, text: text.slice(offset, end)});
   if (result.received !== end) throw Error('The app could not confirm the inserted text.');
   operation.offset = end;
  }
  return mutate('edit-commit', {requestId});
 }

 function scheduleFlush() {
  if (scheduled || failure || closed) return;
  scheduled = true;
  queueMicrotask(() => {
   scheduled = false;
   if (hasChanges() && !failure && !closed) void flush().catch(() => {});
  });
 }

 function change(text) {
  ensure();
  if (external || reloading) throw Error('Confirm the pending document change before typing.');
  if (typeof text !== 'string') throw TypeError('Expected page text.');
  if (text === page.text) return;
  page = {...page, text};
  pageLines = countLines(text);
  epoch++;
  editVersion++;
  clearCache();
  notify();
  scheduleFlush();
 }

 async function drain() {
  if (external) {
   const operation = external;
   if (!operation.applied) {
    const result = await applyOperation(operation);
    ensure();
    receive(result);
    operation.applied = true;
    retry = null;
    clearCache();
    epoch++;
   }
   const value = await request('window', {
    start: Math.min(operation.target, metadata.units), limit: PAGE_UNITS, align: true
   });
   ensure();
   install(value);
   external = null;
   notify();
  }
  while (retry || page.text !== acknowledged) {
   ensure();
   if (!retry) {
    const edit = fragment(acknowledged, page.text);
    retry = {
     sent: page.text,
     lines: pageLines,
     payload: {
      action: 'edit', requestId: identity(), revision: metadata.revision,
      from: page.start + edit.from, to: page.start + edit.to, text: edit.text
     }
    };
   }
   const sent = retry;
   const result = await applyOperation(sent);
   ensure();
   receive(result);
   acknowledged = sent.sent;
   acknowledgedLines = sent.lines;
   retry = null;
   failure = null;
   clearCache();
   notify();
  }
 }

 function flush() {
  ensure();
  if (flushing) return flushing;
  if (!hasChanges()) return Promise.resolve();
  flushing = drain().catch(error => {
   failure = error;
   if (!closed) failed(error);
   throw error;
  }).finally(() => {
   flushing = null;
   notify();
   if (hasChanges()) scheduleFlush();
  });
  return flushing;
 }

 function cacheKey(target, limit) {
  return JSON.stringify([metadata.revision, target.line ?? null, target.start ?? null, limit]);
 }

 async function load(target = {start: 0}, limit = PAGE_UNITS, prepare) {
  ensure();
  const ticket = ++readTicket;
  reading?.abort();
  const controller = new AbortController();
  reading = controller;
  try {
   for (;;) {
    await flush();
    if (controller.signal.aborted || closed) return null;
    const stamp = epoch;
    const key = cacheKey(target, limit);
    const value = cache.get(key) || await request('window', {...target, limit, align: true}, controller.signal);
    if (ticket !== readTicket || closed) return null;
    // Typing may continue while a viewport is read. Read again after edits settle.
    if (epoch !== stamp || hasChanges()) continue;
    const next = viewport(value);
    if (prepare) {
     await prepare(next, value.units, controller.signal);
     if (ticket !== readTicket || controller.signal.aborted || closed) return null;
     if (epoch !== stamp || hasChanges()) continue;
    }
    install(value, next);
    notify();
    return {...page};
   }
  } catch (error) {
   if (controller.signal.aborted) return null;
   throw error;
  } finally {
   if (reading === controller) reading = null;
  }
 }

 async function prefetch(target, limit = PAGE_UNITS) {
  ensure();
  if (hasChanges() || reloading) return;
  const revision = metadata.revision;
  const key = cacheKey(target, limit);
  if (cache.has(key)) return;
  const previous = prefetching.get(key);
  if (previous) return previous.promise;
  if (prefetching.size >= 2) return;
  const controller = new AbortController();
  const promise = (async () => {
   try {
    const value = await request('window', {...target, limit, align: true}, controller.signal);
    if (!closed && !controller.signal.aborted && !value.loading && value.revision === revision &&
        metadata.revision === revision && !hasChanges()) {
     cache.set(key, value);
     while (cache.size > 3) cache.delete(cache.keys().next().value);
    }
   } catch (error) {
    if (!controller.signal.aborted) throw error;
   } finally {
    if (prefetching.get(key)?.controller === controller) prefetching.delete(key);
   }
  })();
  prefetching.set(key, {controller, promise});
  return promise;
 }

 // Read a window without showing it, for example to choose where the next
 // page starts. Returns null when the document changed during the read.
 async function peek(target, limit = PAGE_UNITS, signal) {
  await flush();
  ensure();
  const revision = metadata.revision;
  const value = await request('window', {...target, limit, align: true}, signal);
  ensure();
  if (value.loading || hasChanges() || value.revision !== revision || metadata.revision !== revision ||
      typeof value.text !== 'string' || !Number.isSafeInteger(value.start)) return null;
  return {start: value.start, line: value.line, column: value.column || 0, lineUnits: value.lineUnits || 0, text: value.text};
 }

 async function previewWindow(line, signal) {
  await flush();
  ensure();
  const revision = metadata.revision;
  const value = await request('window', {line, limit:PAGE_UNITS, align:true}, signal);
  if (closed || hasChanges() || value.revision !== revision || metadata.revision !== revision) {
   throw Error('The comparison source changed. Run Compare again.');
  }
  return value;
 }

 async function replace(from, to, text) {
  await flush();
  ensure();
  if (external || reloading) throw Error('Finish the current document change first.');
  if (!Number.isSafeInteger(from) || !Number.isSafeInteger(to) ||
      from < 0 || to < from || to > metadata.units) throw Error('The selected range is no longer available.');
  if (typeof text !== 'string' || text.isWellFormed?.() === false) throw Error('The inserted text contains invalid Unicode.');
  if (from >= page.start && to <= page.start + page.text.length &&
      page.text.length - (to - from) + text.length <= PAGE_UNITS * 2) {
   change(page.text.slice(0, from - page.start) + text + page.text.slice(to - page.start));
   await flush();
   return;
  }

  // Offscreen selections and long inserts use document offsets. The viewport
  // never receives the entire selection just to delete or replace it.
  external = {
   payload: {action: 'edit', requestId: identity(), revision: metadata.revision, from, to, text},
   target: text.length > PAGE_UNITS ? from + text.length - 4096 : from
  };
  editVersion++;
  notify();
  await flush();
 }

 async function history(redo = false) {
  await flush();
  ensure();
  if (external || reloading) throw Error('Finish the current document change first.');
  external = {
   payload: {action: redo ? 'redo' : 'undo', requestId: identity(), revision: metadata.revision},
   target: page.start
  };
  editVersion++;
  notify();
  await flush();
  return {...page};
 }

 async function* readParts(from, to, signal) {
  await flush();
  ensure();
  if (!Number.isSafeInteger(from) || !Number.isSafeInteger(to) ||
      from < 0 || to < from || to > metadata.units) throw Error('The selected text is no longer available.');
  const revision = metadata.revision;
  const stamp = editVersion;
  for (let at = from; at < to;) {
   signal?.throwIfAborted();
   const value = await request('window', {start: at, limit: Math.min(PAGE_UNITS, to - at)}, signal);
   ensure();
   if (value.revision !== revision || metadata.revision !== revision || editVersion !== stamp) {
    throw Error('The document changed while reading the selection. Select the text again.');
   }
   if (value.start !== at || !value.text || value.text.length > to - at) {
    throw Error('The document engine returned an incomplete selection.');
   }
   at += value.text.length;
   yield value.text;
  }
 }

 async function readText(from, to, limit = Infinity, signal) {
  if (to - from > limit) throw Error('The selection is too large for this command.');
  const parts = [];
  for await (const text of readParts(from, to, signal)) parts.push(text);
  return parts.join('');
 }

 async function readBlob(from, to, signal) {
  if (exportRange) {
   await flush();
   ensure();
   signal?.throwIfAborted();
   const response = await exportRange({
    revision: metadata.revision, from, to, format: {encoding: 'UTF-8', bom: false, eol: 'LF'}
   }, signal);
   const blob = await response.blob();
   return blob.slice(0, blob.size, 'text/plain');
  }
  const parts = readParts(from, to, signal);
  const encoder = new TextEncoder();
  const stream = new ReadableStream({
   async pull(controller) {
    const {done, value} = await parts.next();
    if (done) controller.close();
    else controller.enqueue(encoder.encode(value));
   },
   cancel() { return parts.return(); }
  });
  return new Response(stream, {headers: {'Content-Type': 'text/plain'}}).blob();
 }

 async function find(query, start, options = {}, signal) {
  await flush();
  ensure();
  const revision = metadata.revision;
  const result = await request('find', {query, start, ...options}, signal);
  if (result.revision !== revision || metadata.revision !== revision || hasChanges()) {
   throw Error('The document changed during the search. Search again.');
  }
  return result;
 }

 async function save(path, format, overwrite = false, signal) {
  await flush();
  ensure();
  const result = await request('save', {revision: metadata.revision, path, format, overwrite}, signal);
  ensure();
  // Acknowledging an earlier saved revision cannot clear newer edits.
  if (result.metadata.revision === metadata.revision && !retry) receive(result.metadata);
  notify();
  return result;
 }

 async function reload(signal, importer) {
  await flush();
  ensure();
  reloading = true;
  notify();
  try {
   const result = importer ? await importer() : await request('reload', {}, signal);
   ensure();
   receive(result.metadata);
   clearCache();
   acknowledged = '';
   page = emptyPage();
   pageLines = acknowledgedLines = 0;
   retry = null;
   failure = null;
   epoch++;
   editVersion++;
   await load();
   return result;
  } finally {
   reloading = false;
   notify();
  }
 }

 function cancelRead() {
  readTicket++;
  reading?.abort();
 }

 return Object.freeze({
  change, flush, load, prefetch, peek, previewWindow, replace, history, readParts, readText, readBlob, find, save, reload,
  cancelRead,
  initialize(value) { ensure(); receive(value); notify(); },
  state,
  close() { closed = true; cancelRead(); clearCache(); },
  get pending() { return !!flushing || hasChanges(); }
 });
}

const api = Object.freeze({create, fragment, PAGE_UNITS, PAGE_LINES});
root.NTKEditorDiskSession = api;
if (typeof module === 'object' && module.exports) module.exports = api;
})(globalThis);
