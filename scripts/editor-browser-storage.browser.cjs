'use strict';

const assert = require('node:assert/strict');
const {createHash} = require('node:crypto');
const {chromium} = require(process.env.PLAYWRIGHT_MODULE || 'playwright');

const {
 NTK_EDITOR_FIXTURE_URL: url,
 NTK_EDITOR_FIXTURE_TOKEN: token,
 NTK_EDITOR_DRAFT_DESTINATION: destination
} = process.env;
const upload = 'browser fixture row\n'.repeat(65536) + 'UPLOAD-EOF';
const draft = 'seed\n' + 'draft fixture row\n'.repeat(65536) + 'DRAFT-EOF';
const suffix = 'EDITED';

async function run() {
 const browser = await chromium.launch({
  headless: true,
  ...(process.env.TOOLKIT_BROWSER_EXECUTABLE ? {executablePath: process.env.TOOLKIT_BROWSER_EXECUTABLE} : {})
 });
 try {
  const page = await browser.newPage({viewport: {width: 1280, height: 800}});
  const pageErrors = [];
  page.on('pageerror', error => pageErrors.push(error.message));
  await page.addInitScript(value => { window.__networkToolkitNativeSession = value; }, token);
  await page.goto(url);
  await page.waitForFunction(() => window.NetworkToolkitEditorWindows?.bufferSupported());
  await page.evaluate(() => navigate('editor'));
  const text = page.locator('.nt-editor-code textarea[data-ed-text]').first();
  const waitForEditableUnits = units => page.waitForFunction(n => {
   const d = NetworkToolkitFileEditor.active;
   const editor = document.querySelector('.nt-editor-code textarea');
   return d?.diskMeta?.units === n && !d.promoting && !d.diskSession.pending && !editor.disabled && !editor.readOnly;
  }, units);

  await page.evaluate(async content => {
   let file = new File([content], 'browser-fixture.txt', {lastModified: 1});
   let writes = 0, changeWhileWriting = false;
   window.storageFixture = {
    get writes() { return writes; },
    async hash() {
     const digest = await crypto.subtle.digest('SHA-256', await file.arrayBuffer());
     return Array.from(new Uint8Array(digest), n => n.toString(16).padStart(2, '0')).join('');
    },
    changeDuringSave() { changeWhileWriting = true; },
    changeExternally() { file = new File(['external change\n', file], file.name, {lastModified: file.lastModified + 1}); }
   };
   const handle = {
    getFile: async () => file,
    async createWritable() {
     const chunks = [];
     writes++;
     return new WritableStream({
      write(chunk) {
       chunks.push(chunk);
       if (changeWhileWriting) { changeWhileWriting = false; storageFixture.changeExternally(); }
      },
      close() { file = new File(chunks, file.name, {lastModified: file.lastModified + 1}); }
     });
    }
   };
   await NetworkToolkitFileEditor.importFiles([file], [handle]);
  }, upload);
  await waitForEditableUnits(upload.length);
  await text.press('Control+End');
  await page.waitForFunction(n => {
   const d = NetworkToolkitFileEditor.active;
   return d.diskSession.state().page.start + document.querySelector('.nt-editor-code textarea').selectionEnd === n;
  }, upload.length);
  await text.pressSequentially(suffix);
  await waitForEditableUnits(upload.length + suffix.length);
  await page.locator('[data-ed="save"]').click();
  await page.waitForFunction(() => storageFixture.writes === 1 && !NetworkToolkitFileEditor.active.saving &&
   !NetworkToolkitFileEditor.active.dirty);
  assert.equal(await page.evaluate(() => storageFixture.hash()), createHash('sha256').update(upload + suffix).digest('hex'));

  await page.evaluate(() => storageFixture.changeDuringSave());
  await text.pressSequentially('X');
  await page.locator('[data-ed="save"]').click();
  await page.waitForFunction(() => NetworkToolkitFileEditor.active.saveFailed && !NetworkToolkitFileEditor.active.saving);
  assert.equal(await page.evaluate(() => storageFixture.writes), 2);
  assert.equal(await page.evaluate(() => storageFixture.hash()), createHash('sha256').update('external change\n' + upload + suffix).digest('hex'));
  await page.locator('[data-ed="save"]').click();
  await page.waitForFunction(() => NetworkToolkitFileEditor.active.saveFailed && !NetworkToolkitFileEditor.active.saving);
  assert.equal(await page.evaluate(() => storageFixture.writes), 2);
  assert.ok(await page.evaluate(() => NetworkToolkitFileEditor.active.dirty));
  await page.locator('[data-ed="reload"]').click();
  await page.locator('.nt-editor-dialog [data-ed-dialog-action="accept"]').click();
  await page.waitForFunction(n => {
   const d = NetworkToolkitFileEditor.active;
   return !d.reloading && !d.dirty && d.diskMeta.units === n;
  }, 'external change\n'.length + upload.length + suffix.length);
  const uploadedUID = await page.evaluate(() => NetworkToolkitFileEditor.active.uid);
  await page.locator('[data-ed-close-tab]').first().click();
  await page.waitForFunction(uid => !NetworkToolkitFileEditor.documents.size &&
   !NetworkToolkitEditorWindows.state.owned[uid], uploadedUID);

  await page.evaluate(() => NetworkToolkitFileEditor.newDocument());
  await text.fill('seed\n');
  await page.locator('[data-ed-eol]').selectOption('CRLF');
  // Seed the generated paste payload in the same input entry point without
  // sending a megabyte as an automation key gesture.
  await text.evaluate((editor, value) => {
   editor.value = value;
   editor.setSelectionRange(value.length, value.length);
   editor.dispatchEvent(new InputEvent('input', {bubbles: true, inputType: 'insertFromPaste'}));
  }, draft);
  await waitForEditableUnits(draft.length);
  assert.ok((await text.inputValue()).includes('DRAFT-EOF'), 'promotion keeps the insertion visible');
  await text.press('Control+z');
  await waitForEditableUnits('seed\n'.length);
  await text.press('Control+z');
  await waitForEditableUnits(0);
  assert.ok(await page.evaluate(() => NetworkToolkitFileEditor.active.dirty), 'undo retains an unsaved CRLF format change');
  await text.press('Control+y');
  await waitForEditableUnits('seed\n'.length);
  await text.press('Control+y');
  await waitForEditableUnits(draft.length);

  await text.press('Control+Home');
  await page.waitForFunction(() => document.querySelector('.nt-editor-code textarea').value.startsWith('seed\n'));
  for (let i = 0; i < 4; i++) await text.press('Shift+ArrowRight');
  const draftUID = await page.evaluate(() => NetworkToolkitFileEditor.active.uid);
  const move = async (source, target) => {
   await source.evaluate(async ({uid, target}) => {
    const windows = NetworkToolkitEditorWindows;
    const deadline = performance.now() + 5000;
    while (windows.state.running) {
     if (performance.now() >= deadline) throw Error('Editor ownership check did not finish.');
     await new Promise(resolve => setTimeout(resolve, 20));
    }
    await windows.move([uid], target);
    if (!windows.state.outgoing && NetworkToolkitFileEditor.active?.uid === uid) throw Error(windows.state.error || 'No move offer for the disk-backed draft.');
   }, {uid: draftUID, target});
  };
  await move(page, 'new');
  const launch = await page.evaluate(() => api('/editor-fixture/child'));
  const child = await browser.newPage({viewport: {width: 960, height: 600}});
  child.on('pageerror', error => pageErrors.push(error.message));
  await child.addInitScript(value => { window.__networkToolkitNativeSession = value; }, launch.token);
  await child.goto(new URL(launch.path, url).href);
  await child.waitForFunction(({uid, units}) => {
   const d = window.NetworkToolkitFileEditor?.active;
   const ta = document.querySelector('.nt-editor-code textarea');
   return d?.uid === uid && !d.frozen && d.diskMeta?.units === units && !ta?.disabled && ta?.selectionEnd === 4;
  }, {uid: draftUID, units: draft.length});
  await page.waitForFunction(uid => ![...NetworkToolkitFileEditor.documents.values()].some(d => d.uid === uid), draftUID);
  await move(child, 'main');
  await child.waitForFunction(() => !NetworkToolkitFileEditor.documents.size);
  await child.close();
  await waitForEditableUnits(draft.length);
  assert.equal(await text.evaluate(editor => editor.value.slice(editor.selectionStart, editor.selectionEnd)), 'seed');

  await page.locator('[data-ed="saveAs"]').click();
  await page.locator('[data-ed-dialog-input]').fill(destination);
  await page.locator('.nt-editor-dialog [data-ed-dialog-action="accept"]').click();
  await page.waitForFunction(() => {
   const d = NetworkToolkitFileEditor.active;
   return !d.saving && !d.dirty && d.source === 'Local computer';
  });
  const savedID = await page.evaluate(() => NetworkToolkitFileEditor.active.id);
  await page.evaluate(() => NetworkToolkitFileEditor.newDocument());
  await text.fill('clipboard fixture');
  const clipboardID = await page.evaluate(() => {
   Object.defineProperty(navigator, 'clipboard', {configurable: true, value: {
    readText: () => new Promise(resolve => { window.completeClipboardRead = resolve; })
   }});
   return NetworkToolkitFileEditor.active.id;
  });
  await text.press('Shift+F10');
  await page.locator('[data-editor-menu-action="paste"]').click();
  await page.waitForFunction(() => typeof window.completeClipboardRead === 'function');
  await page.evaluate(id => {
   NetworkToolkitFileEditor.activateDocument(id);
   completeClipboardRead('unexpected content');
  }, savedID);
  await waitForEditableUnits(draft.length);
  assert.equal(await page.evaluate(id => NetworkToolkitFileEditor.documents.get(id).content, clipboardID), 'clipboard fixture');
  assert.deepEqual(pageErrors, []);
  console.log('Verified browser file upload, save, conflict and reload, plus draft promotion, undo and exact Save As.');
 } finally {
  await browser.close();
 }
}
run().catch(error => { console.error(error); process.exitCode = 1; });
