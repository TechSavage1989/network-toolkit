/* Test-only DOM automation, inserted by editor_native_windows_test.go. This file
 * is not embedded in the application and creates no production debug bridge. */
(async function () {
  'use strict';
  const endpoint = '/__native_editor_fixture';
  const sleep = ms => new Promise(resolve => setTimeout(resolve, ms));
  const errors = [];
  let rejectSyntheticSave = null;
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
  const fields = ['id', 'uid', 'path', 'source', 'binding', 'hash', 'content', 'original', 'saved', 'savedEol', 'savedBOM', 'savedEncoding', 'dirty', 'saving', 'saveFailed', 'undo', 'redo', 'view', 'compareView', 'eol', 'encoding', 'bom', 'language', 'untitled', 'editRevision', 'frozen', 'staged'];
  const rect = el => { if (!el) return null; const r=el.getBoundingClientRect(); return {left:r.left,top:r.top,width:r.width,height:r.height}; };
  function detachPoint() {
    // Test-only pre-drag geometry: do not display a hidden target or synthesize
    // events. An actual drag must still show it and hit-test its real drop zone.
    globalThis.NetworkToolkitEditorDrag?.mount();
    const host=document.querySelector('.nt-editor-window'),zone=host?.querySelector('[data-editor-drag-detach]');
    if(!host||!zone)return null;
    const r=host.getBoundingClientRect(),style=getComputedStyle(zone),left=parseFloat(style.left),right=parseFloat(style.right),top=parseFloat(style.top),height=parseFloat(style.minHeight);
    const x=r.left+left+(r.width-left-right)/2,y=r.top+top+height/2;
    if(![x,y,left,right,top,height].every(Number.isFinite)||height<=0||x<=r.left||x>=r.right||y<=r.top||y>=r.bottom||x<0||y<0||x>=innerWidth||y>=innerHeight)return null;
    return {left:x,top:y};
  }
  function snapshot() {
    const docs = [...editor().documents.values()];
    // Let the real adapter capture the live selection without manufacturing it.
    if (docs.length && !editor().windowAdapter.canTransfer(docs.map(doc => doc.uid))) {
      editor().windowAdapter.checkpoint();
    }
    const ta = textarea();
    return {
      documents: docs.map(doc => Object.fromEntries(fields.map(key => [key, doc[key]]))),
      active: editor().active?.uid || '',
      window: windows().state,
      textarea: ta && {value: ta.value, disabled: ta.disabled, start: ta.selectionStart, end: ta.selectionEnd, direction: ta.selectionDirection, top: ta.scrollTop, left: ta.scrollLeft},
      closeDialog: !!document.querySelector('.nt-editor-dialog'),
      askSaveBeforeClose: document.querySelector('[data-ed-ask-save-close]')?.checked !== false,
      message: document.querySelector('[data-ed-message]')?.textContent || '',
      startup: globalThis.__networkToolkitStartupState,
      nativeCapability: /^[a-f0-9]{64}$/.test(globalThis.__networkToolkitNativeSession || ''),
      location: location.pathname,
      geometry: {width:innerWidth,height:innerHeight,detachPoint:detachPoint(),main:rect(document.querySelector('.main')),editor:rect(document.querySelector('.nt-editor-window')),tabs:[...document.querySelectorAll('[data-ed-uid]')].map(tab=>({uid:tab.dataset.edUid,...rect(tab.querySelector('[data-ed-open]'))}))},
      drag: globalThis.NetworkToolkitEditorDrag?.state,
      diagnostic: globalThis.__nativeEditorTrace128?.(),
      errors: [...errors]
    };
  }
  function performanceState() {
    const state=snapshot();
    for(const doc of state.documents){for(const key of ['content','original','saved'])doc[key]='length:'+doc[key].length;doc.undo=doc.undo.map(text=>'length:'+text.length);doc.redo=doc.redo.map(text=>'length:'+text.length);}
    if(state.textarea)state.textarea.value='length:'+state.textarea.value.length;
    return state;
  }
  async function performanceDigest() {
    const state=snapshot(),cache=new Map(),digest=async text=>{if(!cache.has(text)){const bytes=await crypto.subtle.digest('SHA-256',new TextEncoder().encode(text));cache.set(text,[...new Uint8Array(bytes)].map(n=>n.toString(16).padStart(2,'0')).join(''));}return cache.get(text);};
    for(const doc of state.documents){for(const key of ['content','original','saved'])doc[key]=await digest(doc[key]);doc.undo=await Promise.all(doc.undo.map(digest));doc.redo=await Promise.all(doc.redo.map(digest));}
    if(state.textarea)state.textarea.value=await digest(state.textarea.value);
    return state;
  }
  async function execute(command) {
    if (command.action === 'snapshot') return snapshot();
    if (command.action === 'performance-state') return performanceState();
    if (command.action === 'performance-digest') return performanceDigest();
    if (command.action === 'seed' || command.action === 'performance-seed') {
      const texts=command.action==='performance-seed'?[('SYNTHETIC PERFORMANCE114 ONLY\n'+'x'.repeat(1000)+'\n').repeat(Math.ceil(command.size/1030)).slice(0,command.size)]:command.texts;
      for (const text of texts) {
        const count = editor().documents.size;
        document.querySelector('.nt-editor-tabs [data-ed="new"]').click();
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
      return command.action==='performance-seed'?performanceState():snapshot();
    }
    if (command.action === 'move' || command.action === 'performance-move') {
      const trigger = await wait(() => document.querySelector('[data-editor-window-toggle]'), 'compact move menu');
      trigger.click();
      const box = await wait(() => { const menu = document.querySelector('[data-editor-window-controls]'); return menu && !menu.hidden && menu.getBoundingClientRect().height > 0 && menu; }, 'visible native move controls');
      await wait(() => !box.querySelector('[data-editor-window-move="' + command.mode + '"]').disabled, 'move button enabled');
      const select = box.querySelector('select');
      if (![...select.options].some(option => option.value === command.destination)) throw Error('Destination not offered by real coordinator');
      select.value = command.destination;
      select.dispatchEvent(new Event('change', {bubbles: true}));
      box.querySelector('[data-editor-window-move="' + command.mode + '"]').click();
      return command.action==='performance-move'?performanceState():snapshot();
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
    if (command.action === 'close-preference') {
      const more = document.querySelector('.nt-editor-more');
      if (!more.open) more.querySelector('summary').click();
      const control = await wait(() => document.querySelector('[data-ed-ask-save-close]'), 'close preference');
      if (control.disabled) throw Error('Main close preference is unexpectedly read-only');
      if (control.checked !== command.enabled) control.click();
      // The user explicitly changed this preference, so no secondary confirmation is shown.
      await wait(() => control.checked === command.enabled && !document.querySelector('.nt-editor-dialog'), 'preference committed');
      more.querySelector('summary').click();
      return snapshot();
    }
    if (command.action === 'close-tab') {
      const d = editor().active, count = editor().documents.size;
      const button = [...document.querySelectorAll('[data-ed-close-tab]')].find(el => el.dataset.edCloseTab === d.id);
      if (!button) throw Error('Active tab close control missing');
      button.click();
      await wait(() => editor().documents.size < count || document.querySelector('.nt-editor-dialog') || /Finish|pending|saving|move/i.test(document.querySelector('[data-ed-message]')?.textContent || ''), 'tab close outcome');
      return snapshot();
    }
    if (command.action === 'answer-close') {
      const button = await wait(() => document.querySelector('.nt-editor-dialog [data-ed-dialog-action="' + command.mode + '"]'), 'explicit close answer');
      button.click();
      await wait(() => !document.querySelector('.nt-editor-dialog') && !windows().state.closing, 'close answer completed without shutdown');
      return snapshot();
    }
    if (command.action === 'start-synthetic-save') {
      // In-memory test binding only. The actual Save button and pending-save
      // lifecycle execute; no host file or remote connection is accessed.
      await editor().open({id:'native-close113-save', source:'Synthetic test binding', path:'synthetic-save113.txt',
        read:async()=>({content:'SYNTHETIC ORIGINAL\n'+Array.from({length:60},(_,i)=>'Synthetic retained save line '+(i+1)).join('\n'),hash:'synthetic',encoding:'UTF-8'}),
        write:()=>new Promise((resolve,reject)=>{rejectSyntheticSave=reject;})});
      const ta=textarea();ta.value+='\nSYNTHETIC UNSAVED SAVE FAILURE';ta.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:'synthetic'}));
      // This intentionally nontransferable in-memory binding cannot use the
      // adapter checkpoint to initialize view metadata. Capture real selection
      // and scroll via the ordinary active-tab action before entering Save.
      ta.setSelectionRange(9,31,'backward');ta.scrollTop=Math.min(120,ta.scrollHeight-ta.clientHeight);
      ta.dispatchEvent(new Event('select',{bubbles:true}));ta.dispatchEvent(new Event('scroll'));
      const uid=editor().active.uid;
      document.querySelector('[data-ed-uid="'+uid+'"] [data-ed-open]').click();
      await wait(()=>{const d=editor().active,t=textarea(),v=d.view;return d.uid===uid&&v&&v.start===9&&v.end===31&&v.direction==='backward'&&v.top>0&&v.start===t.selectionStart&&v.end===t.selectionEnd&&v.direction===t.selectionDirection&&v.top===t.scrollTop&&v.left===t.scrollLeft;},'ordinary tab action captures exact nontrivial selection and scroll before Save');
      const observed=snapshot(),captured=observed.documents.find(d=>d.uid===uid);
      if(!captured?.view||['start','end','direction','top','left'].some(key=>captured.view[key]!==observed.textarea[key]))throw Error('Synthetic Save precondition did not retain the actual editor view');
      document.querySelector('[data-ed="save"]').click();
      await wait(() => editor().active.saving && rejectSyntheticSave, 'actual Save waits on synthetic binding');
      return snapshot();
    }
    if (command.action === 'fail-synthetic-save') {
      if (!rejectSyntheticSave) throw Error('No synthetic pending save');
      rejectSyntheticSave(Error('Synthetic native fixture write failure'));rejectSyntheticSave=null;
      await wait(() => !editor().active.saving && /Save failed: Synthetic native fixture write failure/.test(document.querySelector('[data-ed-message]')?.textContent || ''), 'visible failed save');
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
