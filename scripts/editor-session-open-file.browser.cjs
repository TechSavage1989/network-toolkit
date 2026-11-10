'use strict';
// Driven by TestEditorSessionBrowserOpenedFileIsActive
// (editor_session_open_file_browser_test.go): the shipped page against two
// production engines on one data folder. The main window keeps a session of
// three tabs with unsaved text (one of them a large, disk-backed file) and is
// closed. The standalone Text Editor then starts with a file from Explorer:
// it takes over the main window's tabs, reopens all of them, and the file
// from Explorer must be the active tab. Further files are handed over to the
// running window by real second starts, and a last start opens a file that is
// already one of the kept tabs.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {NTK_EDITOR_MAIN_URL:mainURL,NTK_EDITOR_MAIN_TOKEN:mainToken,NTK_EDITOR_STANDALONE_URL:editorURL,NTK_EDITOR_STANDALONE_TOKEN:editorToken,NTK_EDITOR_SESSION_LARGE:large,NTK_EDITOR_SESSION_SMALL:small,NTK_EDITOR_SESSION_OPENED:opened,NTK_EDITOR_SESSION_SECOND:second,NTK_EDITOR_SESSION_THIRD:third}=process.env;
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const HEAD='EDITED-HEAD ';
(async()=>{
 const browser=await chromium.launch({headless:true,...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 const errors=[];let page;
 async function launch(url,token,standalone){
  const context=await browser.newContext({viewport:{width:1280,height:820}});page=await context.newPage();
  page.on('pageerror',error=>errors.push(error.message));
  await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},token);
  await page.goto(url);
  await page.waitForFunction(()=>window.NetworkToolkitEditorWindows?.bufferSupported()&&window.__networkToolkitStartupState===1);
  assert.equal(await page.evaluate(()=>EDITOR_STANDALONE),standalone);
  if(!standalone)await page.evaluate(()=>navigate('editor'));
  await page.waitForFunction(()=>NetworkToolkitFileEditor.session.phase==='ready');
 }
 const text=()=>page.locator('.nt-editor-code textarea[data-ed-text]').first();
 const settled=()=>page.waitForFunction(()=>{const d=NetworkToolkitFileEditor.active;return d&&!d.loading&&!d.saving&&!d.diskSession?.pending&&!document.querySelector('.nt-editor-code textarea').disabled;});
 const allOpen=count=>page.waitForFunction(count=>NetworkToolkitFileEditor.documents.size===count&&[...NetworkToolkitFileEditor.documents.values()].every(d=>!d.loading),count);
 const activate=file=>page.evaluate(id=>NetworkToolkitFileEditor.activateDocument(id),'local:'+file);
 const head=uid=>page.evaluate(uid=>NetworkToolkitEditorWindows.buffer(uid,'window',{start:0,limit:64}).then(page=>page.text),uid);
 const fixture=(action,body={})=>page.evaluate(([action,body])=>fetch('/api/editor-fixture/'+action,{method:'POST',headers:{'X-Session-Token':window.__networkToolkitNativeSession,'Content-Type':'application/json'},body:JSON.stringify(body)}).then(async r=>r.status===200?200:r.status+' '+await r.text()),[action,body]);
 const docs=()=>page.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].map(d=>({path:d.path,uid:d.uid,dirty:!!d.dirty,disk:!!d.disk,untitled:!!d.untitled,content:d.content})));
 // The active tab as the editor holds it and as the tab strip and the text area show it.
 const shown=()=>page.evaluate(async()=>{const d=NetworkToolkitFileEditor.active,selected=[...document.querySelectorAll('[data-ed-tabs] [data-ed-open]')].filter(tab=>tab.offsetParent&&tab.getAttribute('aria-selected')==='true').map(tab=>tab.querySelector('.nt-editor-tab-name').textContent);
  return {path:d?.path,selected,text:d?.disk?(await NetworkToolkitEditorWindows.buffer(d.uid,'window',{start:0,limit:64})).text:document.querySelector('.nt-editor-code textarea[data-ed-text]').value};});
 const names=list=>list.map(d=>path.basename(d.path));
 const handOver=file=>fixture('second-start',{open:file});
 const active=file=>page.waitForFunction(file=>NetworkToolkitFileEditor.active?.path===file,file);
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  // The main window's editor: a large file, a small file and a draft, all with unsaved text.
  await launch(mainURL,mainToken,false);
  await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),large);await settled();
  const before=await page.evaluate(()=>({disk:NetworkToolkitFileEditor.active.disk,units:NetworkToolkitFileEditor.active.diskMeta.units}));
  assert.equal(before.disk,true,'the large fixture opens disk-backed');
  await text().focus();await page.keyboard.press('Control+Home');await text().pressSequentially(HEAD,{delay:10});
  await page.waitForFunction(units=>NetworkToolkitFileEditor.active.diskMeta.units===units&&!NetworkToolkitFileEditor.active.diskSession.pending,before.units+HEAD.length);
  await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),small);await settled();
  await text().focus();await page.keyboard.press('Control+End');await page.keyboard.type('unsaved line');
  await page.locator('[data-ed-tabs] [data-ed="new"]').first().click();await text().focus();await page.keyboard.type('draft kept for later');
  await activate(large);await settled();
  assert.equal(await page.evaluate(()=>window.__networkToolkitClose()),true);
  const kept=await page.evaluate(()=>api('/editor-session'));
  assert.deepEqual(kept.tabs.map(tab=>[tab.kind,tab.dirty,!!tab.backup]),[['local',true,true],['local',true,true],['untitled',true,true]]);
  assert.equal(kept.active,0,'the kept session has the large file as its active tab');
  assert.equal(await fixture('exit'),200);
  await page.context().close();

  // 1. The standalone Text Editor starts with a file from Explorer and takes over the main window's tabs.
  await launch(editorURL,editorToken,true);await allOpen(4);
  let list=await docs();
  assert.deepEqual(list.map(d=>[path.basename(d.path),d.dirty,d.untitled]),[['large-log.txt',true,false],['small notes.txt',true,false],['Untitled.txt',true,true],['opened from explorer.txt',false,false]],'every kept tab is reopened in order; the file from Explorer follows');
  assert.deepEqual(await shown(),{path:opened,selected:['opened from explorer.txt'],text:'opened from outside\n'},'the file opened from Explorer is the active tab at the start');
  if(output)await page.screenshot({path:path.join(output,'editor-session-open-file-start.png')});
  // The kept tabs that reopened behind it show their unsaved text.
  assert((await head(list[0].uid)).startsWith(HEAD+'synthetic log record \n'),'the large file has its unsaved edit');
  assert.equal(await head(list[1].uid),'first saved line\nunsaved line');
  assert.equal(list[2].content,'draft kept for later');
  await activate(large);await settled();
  assert((await shown()).text.startsWith(HEAD+'synthetic log record \n'),'the large file is shown with its unsaved edit when its tab is chosen');

  // 2. Files handed over to the running Text Editor by further starts.
  assert.equal(await handOver(second),200);await active(second);await allOpen(5);
  assert.deepEqual(await shown(),{path:second,selected:['second file.txt'],text:'second file from outside\n'},'a file handed over to the running editor is the active tab');
  assert.equal(await handOver(large),200);await active(large);await settled();
  list=await docs();assert.equal(list.length,5,'a file that is already open is not opened twice');
  assert.deepEqual((await shown()).selected,['large-log.txt']);assert.equal(list[0].dirty,true,'and keeps its unsaved text');
  assert((await head(list[0].uid)).startsWith(HEAD));
  // Two starts in quick succession: each file once, the last one is active.
  assert.equal(await handOver(third),200);assert.equal(await handOver(opened),200);
  await active(opened);await allOpen(6);
  list=await docs();
  assert.deepEqual(names(list),['large-log.txt','small notes.txt','Untitled.txt','opened from explorer.txt','second file.txt','third file.txt']);
  assert.deepEqual((await shown()).selected,['opened from explorer.txt']);
  if(output)await page.screenshot({path:path.join(output,'editor-session-open-file-handed-over.png')});

  // 3. The next start opens a file that is one of the kept tabs, but not the one that was active.
  await activate(large);await settled();
  assert.equal(await page.evaluate(()=>window.__networkToolkitClose()),true);
  assert.equal(await page.locator('.nt-editor-dialog').count(),0);
  const again=await page.evaluate(()=>api('/editor-session'));
  assert.equal(again.tabs.length,6);assert.equal(again.active,0);
  assert.equal(await fixture('restart',{open:second}),200);
  await page.context().close();
  await launch(editorURL,editorToken,true);await allOpen(6);
  list=await docs();
  assert.deepEqual(names(list),['large-log.txt','small notes.txt','Untitled.txt','opened from explorer.txt','second file.txt','third file.txt'],'every kept tab once');
  assert.deepEqual(list.map(d=>d.dirty),[true,true,true,false,false,false]);
  assert.deepEqual(await shown(),{path:second,selected:['second file.txt'],text:'second file from outside\n'},'the file opened from Explorer is active, not the tab that was active last time');
  assert((await head(list[0].uid)).startsWith(HEAD));assert.equal(await head(list[1].uid),'first saved line\nunsaved line');assert.equal(list[2].content,'draft kept for later');

  // 4. A start without a file: the tab that was active last time, as before.
  await activate(small);await settled();
  assert.equal(await page.evaluate(()=>window.__networkToolkitClose()),true);
  assert.equal(await fixture('restart'),200);
  await page.context().close();
  await launch(editorURL,editorToken,true);await allOpen(6);
  assert.equal((await shown()).path,small,'nothing was asked to be opened: the stored active tab');
  assert.deepEqual((await docs()).map(d=>d.dirty),[true,true,true,false,false,false]);
  assert.deepEqual(errors,[]);
  console.log(JSON.stringify({passed:true,startWithFileAndTakenOverTabs:true,handedOverToRunningEditor:true,alreadyOpenFile:true,severalFiles:true,startWithKeptFile:true,storedActiveWithoutFile:true,keptUnsavedTextReopened:true}));
 }catch(error){
  if(output&&page)await page.screenshot({path:path.join(output,'editor-session-open-file-failure.png')}).catch(()=>{});
  console.error(error);console.error(errors);process.exitCode=1;
 }finally{await browser.close();}
})();
