'use strict';
// Text Editor session restore in the shipped page with synthetic engine APIs:
// open and edit a local file, add an unsaved draft, keep the session, reload,
// and check tabs, order, the active tab, unsaved markers and text. A second
// reload covers a file that changed on disk and one that was deleted.
// The last part opens files from outside (Explorer, a second start: the
// launch request inbox) with a kept session of several tabs: at the start of
// the main window, handed over to the running window, and at the start of
// the standalone Text Editor. The file from outside must be the active tab.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const digest=c=>c.repeat(64);
const NOTES='C:\\fixture\\notes.txt',PLAN='C:\\fixture\\plan.txt',CLEAN='C:\\fixture\\clean.txt',OUTSIDE='C:\\fixture\\from-explorer.txt',SECOND='C:\\fixture\\second.txt',THIRD='C:\\fixture\\third.txt';
(async()=>{
 const files=new Map([[NOTES,{content:'First saved line\nSecond saved line\n',hash:digest('1')}],[PLAN,{content:'Plan v1\n',hash:digest('2')}],[CLEAN,{content:'Nothing to change here.\n',hash:digest('3')}]]);
 for(const [file,c] of [[OUTSIDE,'5'],[SECOND,'6'],[THIRD,'7']])files.set(file,{content:'Opened from outside: '+path.win32.basename(file)+'\n',hash:digest(c)});
 // launch: the engine's inbox of files to open (POST /api/editor-launch takes them). It is offered only in the last part.
 let mode='main',launch=null,sessionDelay=0;
 let index={version:1,active:-1,tabs:[]},copyCount=0;const copies=new Map(),writes=[],errors=[],unloadPrompts=[];
 const browser=await chromium.launch({headless:true,args:['--no-sandbox'],...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 const page=await browser.newPage({viewport:{width:1280,height:860}});
 page.on('pageerror',error=>errors.push(error.message));
 page.on('dialog',dialog=>{if(dialog.type()==='beforeunload')unloadPrompts.push(dialog.message());void dialog.accept();});
 await page.route('https://toolkit.local/**',async route=>{
  const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method();
  if(p.startsWith('/api/')){
   const body=request.postData()?request.postDataJSON():null;let data={},status=200;
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',windowMode:mode,editorDiskAvailable:false,editorSessionAvailable:true,...(launch?{editorLaunchAvailable:true,editorLaunchReusable:true}:{})};
   else if(p==='/api/editor-launch'&&launch){data={requests:launch.splice(0).map(file=>({path:file}))};}
   else if(p==='/api/state')data=method==='GET'?{appUpdatePreferences:{enabled:false}}:{};
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/terminals')data={sessions:[]};
   else if(p==='/api/file-editor/local'){
    if(body.action==='read-text'){const file=files.get(body.path);if(file)data={content:file.content,encoding:'UTF-8',bom:false,hash:file.hash};else{status=400;data={error:'The system cannot find the file specified.'};}}
    else{writes.push(body);data={hash:digest('9')};}
   }else if(p==='/api/editor-session'){
    if(method==='GET'&&sessionDelay)await new Promise(resolve=>setTimeout(resolve,sessionDelay));
    if(method==='GET')data={...index,tabs:index.tabs.map(tab=>tab.kind==='local'?{...tab,exists:files.has(tab.path)}:tab),warning:''};
    else if(method==='PUT'){assert(!JSON.stringify(body).match(/binding|connectionId|password/),'the session index holds no bindings or credentials');index=body;data={ok:true};}
    else if(method==='DELETE'){index={version:1,active:-1,tabs:[]};copies.clear();data={ok:true};}
   }else if(p==='/api/editor-session/backup'){
    if(method==='POST'){assert.equal(body.source,'text');const id=String(++copyCount).padStart(32,'0');copies.set(id,body.text);data={id,size:body.text.length+3,sha256:digest('e'),revision:0,elapsedMs:0};}
    else{const text=copies.get(url.searchParams.get('id'));if(text===undefined){status=404;data={error:'The kept copy of this document is no longer available.'};}else data={text};}
   }else if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
   else data={sessions:[],entries:[]};
   return route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
  }
  try{
   const file=path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));
   if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','editor-session-fixture').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__','null').replaceAll('__EDITOR_VIEW__','null');
   return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 const input=page.locator('.nt-editor-code [data-ed-text]');
 async function openEditor(){
  await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  await page.evaluate(()=>navigate('editor'));
  await page.waitForFunction(()=>NetworkToolkitFileEditor.session.phase==='ready');
 }
 const tabs=()=>page.evaluate(()=>[...document.querySelectorAll('[data-ed-tabs] [data-ed-open]')].filter(tab=>tab.offsetParent).map(tab=>({name:tab.querySelector('.nt-editor-tab-name').textContent,dirty:tab.querySelector('.nt-editor-tab-status').textContent==='●',selected:tab.getAttribute('aria-selected')==='true'})));
 const texts=()=>page.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].map(d=>d.content));
 async function type(text){await input.focus();await page.keyboard.press('Control+End');await page.keyboard.type(text);}
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  await page.goto('https://toolkit.local/');await openEditor();
  assert.equal(await page.locator('[data-ed-restore-session]').isChecked(),true,'restore is on by default');
  // The menu panel is shown by the details element's toggle event, which fires
  // just after the click; wait for it instead of checking in the same instant.
  await page.locator('.nt-editor-more>summary').click();await page.locator('[data-ed-restore-session]').waitFor({state:'visible'});assert.match(await page.locator('#ntEditorRestoreHint').textContent(),/kept in this app’s data folder until it is saved or its tab is closed/);
  await page.locator('#ntEditorRestoreHint').scrollIntoViewIfNeeded();
  if(output)await page.screenshot({path:path.join(output,'editor-session-preference.png')});
  await page.locator('.nt-editor-more>summary').click();await page.locator('[data-ed-restore-session]').waitFor({state:'hidden'});
  await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),NOTES);await type('Unsaved third line');
  await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),PLAN);await type('Plan v2 (unsaved)');
  await page.locator('[data-ed-tabs] [data-ed="new"]').first().click();await type('An untitled draft\nwith two lines');
  await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file),CLEAN);
  await page.locator('[data-ed-open="local:'+NOTES.replaceAll('\\','\\\\')+'"]').click();
  await page.evaluate(()=>NetworkToolkitFileEditor.session.save());
  assert.deepEqual(index.tabs.map(tab=>[tab.kind,tab.path||tab.name,tab.dirty]),[['local',NOTES,true],['local',PLAN,true],['untitled','Untitled.txt',true],['local',CLEAN,false]]);
  assert.equal(index.active,0);assert.equal(index.tabs[0].hash,digest('1'));
  assert.deepEqual([...copies.values()].sort(),['An untitled draft\nwith two lines','First saved line\nSecond saved line\nUnsaved third line','Plan v1\nPlan v2 (unsaved)'].sort());
  const before=await texts();
  // Every unsaved change is kept, so closing the app would not ask about it.
  assert.equal(await page.evaluate(async()=>{const closing=NetworkToolkitFileEditor.windowAdapter.prepareWindowClose(),result=await closing,dialog=!!document.querySelector('.nt-editor-dialog');NetworkToolkitFileEditor.windowAdapter.cancelWindowClose();return result&&!dialog;}),true);

  await page.reload();await openEditor();
  await page.waitForFunction(()=>NetworkToolkitFileEditor.documents.size===4);
  assert.deepEqual(await tabs(),[{name:'notes.txt',dirty:true,selected:true},{name:'plan.txt',dirty:true,selected:false},{name:'Untitled.txt',dirty:true,selected:false},{name:'clean.txt',dirty:false,selected:false}]);
  assert.deepEqual(await texts(),before);
  assert.equal(await input.inputValue(),before[0],'the active restored tab shows its unsaved text');
  assert.doesNotMatch(await page.locator('[data-ed-message]').textContent(),/changed on disk/);
  assert.deepEqual(unloadPrompts,[],'a reload with every change kept does not warn');
  if(output)await page.screenshot({path:path.join(output,'editor-session-restore.png')});

  // Outside changes since the last session: notes.txt changed, plan.txt deleted.
  await page.evaluate(()=>NetworkToolkitFileEditor.session.save());
  files.set(NOTES,{content:'Changed by another program\n',hash:digest('4')});files.delete(PLAN);
  await page.reload();await openEditor();
  await page.waitForFunction(()=>NetworkToolkitFileEditor.documents.size===4);
  const message=await page.locator('[data-ed-message]').textContent();
  assert.match(message,/notes\.txt changed on disk since your last session\. Your unsaved version is shown\./);
  assert.match(message,/plan\.txt no longer exists\. Your unsaved text was restored into a new untitled tab/);
  const after=await page.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].map(d=>({path:d.path,dirty:!!d.dirty,untitled:!!d.untitled,hash:d.hash,content:d.content})));
  assert.deepEqual(after.map(d=>[d.path,d.dirty,d.untitled]),[[NOTES,true,false],['plan.txt',true,true],['Untitled.txt',true,true],[CLEAN,false,false]]);
  assert.equal(after[0].hash,digest('1'),'saving notes.txt still checks the revision it had in the last session');
  assert.deepEqual(after.map(d=>d.content),before);
  assert.equal(writes.length,0,'restoring never writes the original files');
  assert.deepEqual(errors,[]);
  if(output)await page.screenshot({path:path.join(output,'editor-session-restore-changed.png')});

  // ---- Files opened from outside ----
  const shown=()=>page.evaluate(()=>({page:state.activePage,active:NetworkToolkitFileEditor.active?.path||'',paths:[...NetworkToolkitFileEditor.documents.values()].map(d=>d.path),text:document.querySelector('.nt-editor-code [data-ed-text]').value}));
  const settledTabs=count=>page.waitForFunction(count=>NetworkToolkitFileEditor.session.phase==='ready'&&NetworkToolkitFileEditor.documents.size===count&&[...NetworkToolkitFileEditor.documents.values()].every(d=>!d.loading),count);
  const selected=async()=>(await tabs()).filter(tab=>tab.selected).map(tab=>tab.name);
  const outsideText=file=>'Opened from outside: '+path.win32.basename(file)+'\n';
  const KEPT=[NOTES,'plan.txt','Untitled.txt',CLEAN];
  await page.evaluate(()=>NetworkToolkitFileEditor.session.save());
  assert.equal(index.active,0,'the kept session has notes.txt as its active tab');assert.equal(index.tabs.length,4);
  // 1. The main window starts with a file (the Text Editor shortcut or Explorer): its page is shown and the file is the active tab.
  launch=[OUTSIDE];
  await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await settledTabs(5);
  let now=await shown();
  assert.equal(now.page,'editor','the Text Editor page is shown for a file from outside');
  assert.deepEqual(now.paths,[...KEPT,OUTSIDE],'every kept tab is reopened in order; the file from outside follows');
  assert.equal(now.active,OUTSIDE,'the file opened from outside is the active tab at the start of the main window');
  assert.deepEqual(await selected(),['from-explorer.txt']);assert.equal(now.text,outsideText(OUTSIDE));
  assert.deepEqual((await texts()).slice(0,4),before,'kept unsaved text is all there');
  assert.deepEqual((await tabs()).map(tab=>tab.dirty),[true,true,true,false,false]);
  if(output)await page.screenshot({path:path.join(output,'editor-session-open-file-active.png')});
  // 2. Handed over to the running window (the page asks the inbox once a second).
  await page.locator('[data-ed-open="local:'+NOTES.replaceAll('\\','\\\\')+'"]').click();assert.deepEqual(await selected(),['notes.txt']);
  launch.push(CLEAN);// already open in a tab
  await page.waitForFunction(file=>NetworkToolkitFileEditor.active?.path===file,CLEAN);
  now=await shown();assert.equal(now.paths.length,5,'a file that is already open is not opened twice');assert.deepEqual(await selected(),['clean.txt']);
  await page.evaluate(()=>navigate('overview'));
  launch.push(SECOND);
  await page.waitForFunction(file=>NetworkToolkitFileEditor.active?.path===file&&state.activePage==='editor',SECOND);await settledTabs(6);
  assert.deepEqual(await selected(),['second.txt']);assert.equal((await shown()).text,outsideText(SECOND));
  // Several files handed over together: each once, the last one is active.
  await page.locator('[data-ed-open="local:'+NOTES.replaceAll('\\','\\\\')+'"]').click();
  launch.push(THIRD,OUTSIDE,SECOND);
  await page.waitForFunction(file=>NetworkToolkitFileEditor.active?.path===file,SECOND);await settledTabs(7);
  now=await shown();assert.deepEqual(now.paths,[...KEPT,OUTSIDE,SECOND,THIRD]);assert.deepEqual(await selected(),['second.txt']);
  // 3. The standalone Text Editor starts with a file that is one of the kept tabs, but not the one that was active.
  await page.locator('[data-ed-open="local:'+NOTES.replaceAll('\\','\\\\')+'"]').click();
  await page.evaluate(()=>NetworkToolkitFileEditor.session.save());
  assert.equal(index.active,0);assert.equal(index.tabs.length,7);
  for(const delay of [0,1500]){
   // delay 0: the session is read before the launch file is asked for; 1500: after (a slow read, or tabs taken over from the other editor).
   mode='editor-standalone';launch=[THIRD];sessionDelay=delay;
   await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await settledTabs(7);
   now=await shown();
   assert.deepEqual(now.paths,[...KEPT,OUTSIDE,SECOND,THIRD],'the standalone editor reopens every kept tab once');
   assert.equal(now.active,THIRD,'the file opened from outside is the active tab at the start of the standalone editor (session read delayed '+delay+' ms)');
   assert.deepEqual(await selected(),['third.txt']);assert.equal(now.text,outsideText(THIRD));
   assert.deepEqual((await texts()).slice(0,4),before);
   // Without a file from outside the stored active tab applies, as before.
   await page.locator('[data-ed-open="local:'+NOTES.replaceAll('\\','\\\\')+'"]').click();
   await page.evaluate(()=>NetworkToolkitFileEditor.session.save());assert.equal(index.active,0);
  }
  launch=[];sessionDelay=0;
  await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await settledTabs(7);
  assert.deepEqual(await selected(),['notes.txt'],'nothing was asked to be opened: the stored active tab');
  if(output)await page.screenshot({path:path.join(output,'editor-session-open-file-standalone.png')});
  assert.equal(writes.length,0,'opening files from outside never writes a file');
  assert.deepEqual(errors,[]);
  console.log(JSON.stringify({passed:true,restoredTabs:4,openedFromOutsideIsActive:{mainWindowStart:true,runningWindow:true,alreadyOpen:true,severalFiles:true,standaloneStart:true},orderAndActiveTab:true,unsavedMarkersAndText:true,closeWithoutPrompt:true,changedOnDiskMessage:true,missingFileFallback:true,noFileWrites:true}));
 }catch(error){
  if(output)await page.screenshot({path:path.join(output,'editor-session-restore-failure.png')}).catch(()=>{});
  console.error(error);process.exitCode=1;
 }finally{await browser.close();}
})();
