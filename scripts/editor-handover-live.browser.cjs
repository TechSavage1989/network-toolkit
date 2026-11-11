'use strict';
// Driven by TestEditorHandoverBrowser (peer_handover_browser_test.go): the shipped pages of the main
// Toolkit window and of the standalone Text Editor, each against its own real engine of one data
// root. The two engines talk over real loopback HTTP through their protected instance records.
// Nothing here is intercepted. Chromium stands in for the app windows; no native Windows claim.
// Owner decision: popping the main window's Text Editor out while the standalone Text Editor runs
// adds the local and untitled tabs to that editor; a tab that cannot be carried (here: a file that
// changed on disk) still goes to a connected window. A refused hand-over leaves the tabs in place.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {NTK_HANDOVER_MAIN_URL:mainURL,NTK_HANDOVER_MAIN_TOKEN:mainToken,NTK_HANDOVER_EDITOR_URL:editorURL,NTK_HANDOVER_EDITOR_TOKEN:editorToken}=process.env;
const files=JSON.parse(process.env.NTK_HANDOVER_FILES||'null');
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const errors=[];
const pass=text=>console.log('PASS '+text);
(async()=>{
 assert(mainURL&&mainToken&&editorURL&&editorToken&&files?.notes&&files?.clean&&files?.changed,'run through go test (TestEditorHandoverBrowser)');
 if(output)fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,args:['--no-sandbox'],...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 async function app(url,token,name){
  const context=await browser.newContext({viewport:{width:1280,height:800}}),page=await context.newPage();page.setDefaultTimeout(20000);
  page.on('pageerror',error=>errors.push(name+': '+error.message));
  await page.route('**/*',route=>new URL(route.request().url()).origin===url?route.continue():route.abort());
  await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},token);
  await page.goto(url+'/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  return page;
 }
 const shot=async(page,name)=>{if(output)await page.screenshot({path:path.join(output,'editor-handover-'+name+'.png')});};
 const toast=(page,text)=>page.waitForFunction(t=>[...document.querySelectorAll('#toastStack .toast')].some(n=>n.textContent.includes(t)),text);
 const docs=page=>page.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].map(d=>({name:d.path.split(/[\\/]/).pop(),path:d.path,text:d.content,dirty:!!d.dirty,eol:d.eol,encoding:d.encoding,frozen:!!d.frozen,untitled:!!d.untitled,active:d===NetworkToolkitFileEditor.active})));
 let main,editor;
 try{
  main=await app(mainURL,mainToken,'main');editor=await app(editorURL,editorToken,'editor');
  assert.equal(await main.evaluate(()=>WINDOW_MODE),'main');assert.equal(await editor.evaluate(()=>WINDOW_MODE),'editor-standalone');
  await main.evaluate(()=>navigate('editor'));
  for(const page of [main,editor])await page.waitForFunction(()=>NetworkToolkitFileEditor.session.phase==='ready');
  await main.waitForFunction(()=>NetworkToolkitEditorWindows.state.enabled===true);
  const input=main.locator('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');
  const type=async text=>{await input.focus();await main.keyboard.press('Control+End');await main.keyboard.type(text);};

  // ---- The main window: an edited local file (CRLF), a saved file, an untitled draft and a file that changes on disk ----
  await main.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),files.notes);await type('description uplink');
  await main.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),files.clean);
  await main.evaluate(()=>NetworkToolkitFileEditor.newDocument());await type('interface Gi0/2');
  await main.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),files.changed);await type('mine');
  fs.writeFileSync(files.changed,'changed by another program\n');
  const before=await docs(main);
  assert.deepEqual(before.map(d=>[d.name,d.dirty]),[['notes.txt',true],['clean.txt',false],['Untitled.txt',true],['changed.txt',true]]);
  assert.equal(before[0].eol,'CRLF');
  // The standalone Text Editor's page reports within a second or two.
  for(let attempt=0;!await main.evaluate(()=>NetworkToolkitEditorHandover.receiver().then(found=>!!found));attempt++){assert(attempt<40,'the standalone Text Editor never became available');await main.waitForTimeout(250);}

  // ---- Pop the editor out ----
  await main.locator('[data-editor-window-detach]').click();
  const expected='3 tabs moved to the open Network Toolkit Text Editor; 1 tab that changed on disk since it was opened opens in a connected window.';
  await toast(main,expected);
  pass('the main window says where each tab went');
  await editor.waitForFunction(()=>NetworkToolkitFileEditor.documents.size===3);
  const received=await docs(editor);
  assert.deepEqual(received.map(d=>[d.name,d.dirty,d.untitled]),[['notes.txt',true,false],['clean.txt',false,false],['Untitled.txt',true,true]]);
  assert.equal(received[0].path,files.notes);assert.equal(received[0].text,before[0].text,'the unsaved text arrived exactly');assert.equal(received[0].eol,'CRLF','line endings carried');assert.equal(received[0].encoding,'UTF-8');
  assert.equal(received[2].text,'interface Gi0/2');assert.equal(received[1].text,'hostname lab-core-01\n');
  assert.equal(received.find(d=>d.active)?.name,'notes.txt','the first handed-over tab is active');
  assert(received.every(d=>!d.frozen),'the received tabs can be edited');
  // The unsaved text is kept in the editor's own session before the main window let go of it.
  assert.equal(await editor.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].filter(d=>d.dirty).every(d=>NetworkToolkitFileEditor.session.isKept(d))),true);
  pass('the local and untitled tabs arrived in the running Text Editor with their unsaved text, format and title');
  await shot(editor,'received');
  // The changed file goes to a connected window; nothing else is left in the main window.
  await main.waitForFunction(()=>[...NetworkToolkitFileEditor.documents.values()].length===1);
  const left=await docs(main);
  assert.deepEqual(left.map(d=>d.name),['changed.txt']);assert.equal(left[0].text,before[3].text,'its unsaved text is untouched');
  await main.waitForFunction(()=>NetworkToolkitEditorWindows.state.outgoing===1);
  pass('the tab that cannot be carried is moving to a connected window, as before');
  await shot(main,'after');
  // Nothing was saved: the files on disk are as they were.
  assert.equal(fs.readFileSync(files.notes,'utf8'),'hostname lab-switch-01\r\ninterface Gi0/1\r\n');

  // ---- A refusal: a dialog is open in the Text Editor. The tab stays in the main window, unchanged. ----
  await main.evaluate(()=>NetworkToolkitEditorWindows.cancel());
  await main.waitForFunction(()=>NetworkToolkitEditorWindows.state.outgoing===0&&[...NetworkToolkitFileEditor.documents.values()].every(d=>!d.frozen));
  await main.evaluate(()=>NetworkToolkitFileEditor.newDocument());await type('kept here');
  await editor.evaluate(()=>{document.querySelector('#modalRoot').innerHTML='<section class="modal" role="dialog"></section>';});
  const tabMenu=await main.evaluate(()=>NetworkToolkitFileEditor.active.uid);
  // move() refuses while the window's ownership poll runs; ask again, as a second click would.
  const moveTab=async options=>{for(let attempt=0;;attempt++){const error=await main.evaluate(([uid,options])=>NetworkToolkitEditorWindows.move([uid],'new',options).then(()=>'',e=>e.message),[tabMenu,options]);if(!error)return;assert(/Wait for the current/.test(error)&&attempt<40,error);await main.waitForTimeout(100);}};
  // Owner decision: Move to separate window on ONE tab always opens its own connected window,
  // even while the Text Editor runs (the tab menu calls move() without a hand-over).
  await moveTab(undefined);
  await main.waitForFunction(()=>NetworkToolkitEditorWindows.state.outgoing===1);
  assert.equal((await docs(editor)).length,3,'a single tab is never handed to the Text Editor');
  await main.evaluate(()=>NetworkToolkitEditorWindows.cancel());
  await main.waitForFunction(()=>NetworkToolkitEditorWindows.state.outgoing===0&&[...NetworkToolkitFileEditor.documents.values()].every(d=>!d.frozen));
  pass('Move to separate window on a single tab goes to a connected window, not to the running Text Editor');
  // Detach editor and Move all hand over (move(..., {handover:true})); here the editor refuses.
  await moveTab({handover:true});
  await toast(main,'1 tab stayed here: A dialog is open in Network Toolkit Text Editor');
  const kept=(await docs(main)).find(d=>d.text==='kept here');
  assert(kept&&!kept.frozen&&kept.dirty,'the refused tab is still here, editable and unsaved');
  assert.equal((await docs(editor)).length,3,'nothing was added behind the dialog');
  pass('a refused hand-over leaves the tab in the main window, unchanged');
  console.log('editor-handover-live: OK');
 }catch(error){
  for(const [name,page] of [['main',main],['editor',editor]]){try{if(page)console.error(name+': '+JSON.stringify(await docs(page))+' toasts '+JSON.stringify(await page.evaluate(()=>[...document.querySelectorAll('#toastStack .toast')].map(n=>n.textContent))));}catch{}}
  console.error(error);process.exitCode=1;
 }finally{
  if(errors.length){console.error('page errors: '+errors.join(' | '));process.exitCode=1;}
  await browser.close();
 }
})();
