'use strict';
// Driven by TestEditorSessionBrowser (editor_session_browser_test.go): the
// shipped page against the production editor and session handlers. A large
// local file is disk-backed, so its kept copy is written by the engine; a
// small local file and a draft send their text. After an app close and a
// simulated restart, the tabs come back with their unsaved text.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {NTK_EDITOR_FIXTURE_URL:url,NTK_EDITOR_FIXTURE_TOKEN:token,NTK_EDITOR_SESSION_LARGE:large,NTK_EDITOR_SESSION_SMALL:small}=process.env;
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const HEAD='EDITED-HEAD ';
(async()=>{
 const browser=await chromium.launch({headless:true,...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 const errors=[];let page;
 async function launch(){
  const context=await browser.newContext({viewport:{width:1280,height:820}});page=await context.newPage();
  page.on('pageerror',error=>errors.push(error.message));
  await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},token);
  await page.goto(url);
  await page.waitForFunction(()=>window.NetworkToolkitEditorWindows?.bufferSupported()&&window.__networkToolkitStartupState===1);
  await page.evaluate(()=>navigate('editor'));
  await page.waitForFunction(()=>NetworkToolkitFileEditor.session.phase==='ready');
 }
 const text=()=>page.locator('.nt-editor-code textarea[data-ed-text]').first();
 const settled=()=>page.waitForFunction(()=>{const d=NetworkToolkitFileEditor.active;return d&&!d.loading&&!d.saving&&!d.diskSession?.pending&&!document.querySelector('.nt-editor-code textarea').disabled;});
 const activate=file=>page.evaluate(id=>NetworkToolkitFileEditor.activateDocument(id),'local:'+file);
 const head=uid=>page.evaluate(uid=>NetworkToolkitEditorWindows.buffer(uid,'window',{start:0,limit:64}).then(page=>page.text),uid);
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  await launch();
  await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),large);await settled();
  const before=await page.evaluate(()=>({disk:NetworkToolkitFileEditor.active.disk,units:NetworkToolkitFileEditor.active.diskMeta.units}));
  assert.equal(before.disk,true,'the large fixture opens disk-backed');
  await text().focus();await page.keyboard.press('Control+Home');await text().pressSequentially(HEAD,{delay:10});
  await page.waitForFunction(units=>NetworkToolkitFileEditor.active.diskMeta.units===units&&!NetworkToolkitFileEditor.active.diskSession.pending,before.units+HEAD.length);
  await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),small);await settled();
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.disk),undefined,'the small fixture is an ordinary memory document');
  await text().focus();await page.keyboard.press('Control+End');await page.keyboard.type('unsaved line');
  await page.locator('[data-ed-tabs] [data-ed="new"]').first().click();await text().focus();await page.keyboard.type('draft kept for later');
  await activate(large);await settled();
  // Closing the app keeps every unsaved change and therefore does not ask.
  assert.equal(await page.evaluate(()=>window.__networkToolkitClose()),true);
  assert.equal(await page.locator('.nt-editor-dialog').count(),0);
  const kept=await page.evaluate(()=>api('/editor-session'));
  assert.deepEqual(kept.tabs.map(tab=>[tab.kind,tab.dirty,!!tab.backup]),[['local',true,true],['local',true,true],['untitled',true,true]]);
  assert.equal(kept.active,0);
  assert.equal((await page.evaluate(()=>fetch('/api/editor-fixture/restart',{method:'POST',headers:{'X-Session-Token':window.__networkToolkitNativeSession}}).then(r=>r.status))),200);
  await page.context().close();

  await launch();
  await page.waitForFunction(()=>NetworkToolkitFileEditor.documents.size===3&&[...NetworkToolkitFileEditor.documents.values()].every(d=>!d.loading));
  const restored=await page.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].map(d=>({path:d.path,uid:d.uid,dirty:!!d.dirty,disk:!!d.disk,untitled:!!d.untitled,content:d.content})));
  assert.deepEqual(restored.map(d=>[path.basename(d.path),d.dirty,d.disk,d.untitled]),[['large-log.txt',true,true,false],['small notes.txt',true,true,false],['Untitled.txt',true,false,true]]);
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.active.path),large,'the stored active tab is selected');
  assert((await head(restored[0].uid)).startsWith(HEAD+'synthetic log record \n'),'the large file shows its unsaved edit');
  assert.equal(await head(restored[1].uid),'first saved line\nunsaved line');
  assert.equal(restored[2].content,'draft kept for later');
  assert.match(await page.locator('[data-ed-message]').textContent(),/small notes\.txt changed on disk since your last session\. Your unsaved version is shown\./);
  if(output)await page.screenshot({path:path.join(output,'editor-session-disk-restored.png')});
  // The unchanged large file saves; the changed small file refuses to be replaced.
  await activate(large);await settled();await page.locator('[data-ed="save"]').click();
  await page.waitForFunction(()=>!NetworkToolkitFileEditor.active.saving&&!NetworkToolkitFileEditor.active.dirty);
  await activate(small);await settled();await page.locator('[data-ed="save"]').click();
  await page.waitForFunction(()=>NetworkToolkitFileEditor.active.saveFailed===true);
  assert.match(await page.locator('[data-ed-message]').textContent(),/Save failed: .*changed/);
  assert.equal(await page.evaluate(()=>window.__networkToolkitClose()),true,'a kept document whose save failed does not block closing');
  assert.equal(await page.locator('.nt-editor-dialog').count(),0);
  assert.deepEqual(errors,[]);
  console.log(JSON.stringify({passed:true,diskBackedCopyByEngine:true,closeWithoutPrompt:true,restoredOrderAndActive:true,changedOnDisk:true,saveBaselineChecked:true}));
 }catch(error){
  if(output&&page)await page.screenshot({path:path.join(output,'editor-session-disk-failure.png')}).catch(()=>{});
  console.error(error);console.error(errors);process.exitCode=1;
 }finally{await browser.close();}
})();
