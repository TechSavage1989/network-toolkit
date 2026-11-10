'use strict';
// Text Editor session restore in the shipped page with synthetic engine APIs:
// open and edit a local file, add an unsaved draft, keep the session, reload,
// and check tabs, order, the active tab, unsaved markers and text. A second
// reload covers a file that changed on disk and one that was deleted.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const digest=c=>c.repeat(64);
const NOTES='C:\\fixture\\notes.txt',PLAN='C:\\fixture\\plan.txt',CLEAN='C:\\fixture\\clean.txt';
(async()=>{
 const files=new Map([[NOTES,{content:'First saved line\nSecond saved line\n',hash:digest('1')}],[PLAN,{content:'Plan v1\n',hash:digest('2')}],[CLEAN,{content:'Nothing to change here.\n',hash:digest('3')}]]);
 let index={version:1,active:-1,tabs:[]},copyCount=0;const copies=new Map(),writes=[],errors=[],unloadPrompts=[];
 const browser=await chromium.launch({headless:true,args:['--no-sandbox'],...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 const page=await browser.newPage({viewport:{width:1280,height:860}});
 page.on('pageerror',error=>errors.push(error.message));
 page.on('dialog',dialog=>{if(dialog.type()==='beforeunload')unloadPrompts.push(dialog.message());void dialog.accept();});
 await page.route('https://toolkit.local/**',async route=>{
  const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method();
  if(p.startsWith('/api/')){
   const body=request.postData()?request.postDataJSON():null;let data={},status=200;
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',editorDiskAvailable:false,editorSessionAvailable:true};
   else if(p==='/api/state')data=method==='GET'?{appUpdatePreferences:{enabled:false}}:{};
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/terminals')data={sessions:[]};
   else if(p==='/api/file-editor/local'){
    if(body.action==='read-text'){const file=files.get(body.path);if(file)data={content:file.content,encoding:'UTF-8',bom:false,hash:file.hash};else{status=400;data={error:'The system cannot find the file specified.'};}}
    else{writes.push(body);data={hash:digest('9')};}
   }else if(p==='/api/editor-session'){
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
   if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','editor-session-fixture').replaceAll('__WINDOW_MODE__','main');
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
  console.log(JSON.stringify({passed:true,restoredTabs:4,orderAndActiveTab:true,unsavedMarkersAndText:true,closeWithoutPrompt:true,changedOnDiskMessage:true,missingFileFallback:true,noFileWrites:true}));
 }catch(error){
  if(output)await page.screenshot({path:path.join(output,'editor-session-restore-failure.png')}).catch(()=>{});
  console.error(error);process.exitCode=1;
 }finally{await browser.close();}
})();
