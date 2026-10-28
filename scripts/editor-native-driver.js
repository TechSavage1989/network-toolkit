/* Test-only DOM automation, inserted by editor_native_windows_test.go. This file
 * is not embedded in the application and creates no production debug bridge. */
(async function () {
  'use strict';
  const endpoint = '/__native_editor_fixture';
  const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
  const errors = [];
  addEventListener('error', event => errors.push(String(event.message)));
  addEventListener('unhandledrejection', event => errors.push(String(event.reason?.message || event.reason)));
  async function wait(predicate, description, timeout = 15000) {
    const deadline = Date.now() + timeout;
    while (Date.now() < deadline) {
      const value = predicate();
      if (value) return value;
      await sleep(40);
    }
    throw Error('Native editor fixture timed out: ' + description);
  }
  const editor = () => globalThis.NetworkToolkitFileEditor;
  const windows = () => globalThis.NetworkToolkitEditorWindows;
  const textarea = () => document.querySelector('[data-ed-text]');
  const fields = ['id', 'uid', 'path', 'source', 'binding', 'hash', 'content', 'original', 'saved', 'savedEol', 'savedBOM', 'savedEncoding', 'dirty', 'undo', 'redo', 'view', 'compareView', 'eol', 'encoding', 'bom', 'language', 'untitled', 'editRevision', 'frozen', 'staged'];
  function snapshot() {
    const docs = [...editor().documents.values()];
    // Let the real adapter capture the live selection without manufacturing it.
    if (docs.length && docs.every(doc => !doc.frozen)) {
      editor().windowAdapter.checkpoint();
    }
    const ta = textarea();
    return {
      documents: docs.map(doc => Object.fromEntries(fields.map(key => [key, doc[key]]))),
      active: editor().active?.uid || '',
      window: windows().state,
      textarea: ta && {value: ta.value, disabled: ta.disabled, start: ta.selectionStart, end: ta.selectionEnd, direction: ta.selectionDirection, top: ta.scrollTop, left: ta.scrollLeft},
      startup: globalThis.__networkToolkitStartupState,
      nativeCapability: /^[a-f0-9]{64}$/.test(globalThis.__networkToolkitNativeSession || ''),
      location: location.pathname,
      errors: [...errors]
    };
  }
  async function execute(command) {
    if (command.action === 'snapshot') return snapshot();
    if (command.action === 'seed') {
      for (const text of command.texts) {
        const count = editor().documents.size;
        document.querySelector('.nt-editor-toolbar [data-ed="new"]').click();
        await wait(() => editor().documents.size === count + 1, 'New document creates exactly one tab');
        const ta = textarea();
        ta.focus();
        ta.value = text;
        ta.dispatchEvent(new InputEvent('input', {bubbles: true, inputType: 'insertText', data: text}));
        ta.value += '\nSYNTHETIC SECOND EDIT';
        ta.dispatchEvent(new InputEvent('input', {bubbles: true, inputType: 'insertText', data: '\nSYNTHETIC SECOND EDIT'}));
      }
      textarea().setSelectionRange(11, 29, 'backward');
      textarea().scrollTop = 120;
      textarea().dispatchEvent(new Event('scroll', {bubbles: true}));
      return snapshot();
    }
    if (command.action === 'move') {
      const trigger = await wait(() => document.querySelector('[data-editor-window-toggle]'), 'compact move menu');
      trigger.click();
      const box = await wait(() => { const menu = document.querySelector('[data-editor-window-controls]'); return menu && !menu.hidden && menu.getBoundingClientRect().height > 0 && menu; }, 'visible native move controls');
      await wait(() => !box.querySelector('[data-editor-window-move="' + command.mode + '"]').disabled, 'move button enabled');
      const select = box.querySelector('select');
      if (![...select.options].some(option => option.value === command.destination)) throw Error('Destination not offered by real coordinator');
      select.value = command.destination;
      select.dispatchEvent(new Event('change', {bubbles: true}));
      box.querySelector('[data-editor-window-move="' + command.mode + '"]').click();
      return snapshot();
    }
    if (command.action === 'cancel') {
      const trigger = await wait(() => document.querySelector('[data-editor-window-toggle]'), 'compact cancel access');
      trigger.click();
      const button = await wait(() => {
        const el = document.querySelector('[data-editor-window-cancel]');
        return el && !el.hidden && !el.disabled && el;
      }, 'cancel pending move enabled');
      button.click();
      await wait(() => windows().state.outgoing === 0 && !windows().state.running, 'authoritative cancel response');
      return snapshot();
    }
    if (command.action === 'edit-history') {
      const ta = textarea();
      if (ta.disabled || editor().active?.frozen) throw Error('Destination is not editable');
      const before = ta.value;
      ta.value = before + '\nSYNTHETIC CHILD EDIT';
      ta.dispatchEvent(new InputEvent('input', {bubbles: true, inputType: 'insertText', data: '\nSYNTHETIC CHILD EDIT'}));
      document.querySelector('[data-ed="undo"]').click();
      if (textarea().value !== before) throw Error('Transferred undo history is unusable');
      document.querySelector('[data-ed="redo"]').click();
      if (textarea().value !== before + '\nSYNTHETIC CHILD EDIT') throw Error('Transferred redo history is unusable');
      textarea().setSelectionRange(13, 35, 'backward');
      textarea().scrollTop = 100;
      textarea().dispatchEvent(new Event('scroll', {bubbles: true}));
      return snapshot();
    }
    if (command.action === 'release-empty') {
      if (editor().documents.size) throw Error('Refusing fixture close with owned documents');
      await wait(() => !windows().state.running, 'idle before empty native release');
      await windows().beforeClose();
      return snapshot();
    }
    throw Error('Unknown fixed native fixture action');
  }
  const headers = {'Content-Type': 'application/json', 'X-Session-Token': globalThis.__networkToolkitNativeSession};
  async function exchange(body) {
    const response = await fetch(endpoint, {method: 'POST', headers, body: JSON.stringify(body)});
    if (!response.ok) throw Error('Native fixture report channel HTTP ' + response.status);
    return response.json();
  }
  try {
    await wait(() => globalThis.__networkToolkitStartupState === 1 && editor() && windows(), 'real app startup', 35000);
    if (!location.pathname.startsWith('/editor-view/')) {
      const button = document.querySelector('#navigation [data-page="editor"]');
      if (!button) throw Error('Main editor navigation control absent');
      button.click();
    }
    await wait(() => textarea() && windows().state.enabled, 'editor and native coordinator mounted');
    for (;;) {
      const command = await exchange({kind: 'next'});
      if (!command.id) continue;
      let result;
      try { result = {id: command.id, state: await execute(command)}; }
      catch (error) { result = {id: command.id, error: String(error.stack || error), state: snapshot()}; }
      await exchange({kind: 'result', ...result});
    }
  } catch (error) {
    try { await exchange({kind: 'fatal', error: String(error.stack || error)}); } catch {}
  }
})();
