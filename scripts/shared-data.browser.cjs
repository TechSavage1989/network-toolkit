'use strict';
// Driven by TestSharedDataBrowser (shared_data_browser_test.go): the shipped
// page of the main window, the standalone Terminal and the standalone Text
// Editor, each against its own real engine, all three on one temporary data
// folder. Saved connections are synthetic; no connection is opened.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const urls={main:process.env.NTK_SHARED_MAIN_URL,terminal:process.env.NTK_SHARED_TERMINAL_URL,editor:process.env.NTK_SHARED_EDITOR_URL};
const tokens={main:process.env.NTK_SHARED_MAIN_TOKEN,terminal:process.env.NTK_SHARED_TERMINAL_TOKEN,editor:process.env.NTK_SHARED_EDITOR_TOKEN};
const report={passed:false,checks:[],errors:[]};
const check=text=>{report.checks.push(text);console.log('ok - '+text);};
(async()=>{
 for(const [name,url] of Object.entries(urls))assert(url,'missing engine URL for '+name);
 if(output)fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 const pages={};
 try{
  for(const name of ['main','terminal','editor']){
   const context=await browser.newContext({viewport:{width:1280,height:800}}),page=await context.newPage();
   page.setDefaultTimeout(20000);
   page.on('pageerror',error=>report.errors.push(name+': '+error.message));
   if(tokens[name])await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},tokens[name]);
   await page.goto(urls[name]);
   await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online&&stateLoaded);
   pages[name]=page;
  }
  const {main,terminal,editor}=pages;
  assert.equal(await main.evaluate(()=>WINDOW_MODE),'main');
  assert.equal(await terminal.evaluate(()=>WINDOW_MODE),'terminal');
  assert.equal(await editor.evaluate(()=>WINDOW_MODE),'editor-standalone');
  const showSaved=page=>page.evaluate(()=>{if(state.activePage!=='terminal')navigate('terminal');NetworkToolkitSessionSidebar.activateTab('saved',{expand:true});});
  await showSaved(main);await showSaved(terminal);
  const rows=page=>page.evaluate(()=>[...document.querySelectorAll('#sessionSavedList [data-session-kind="saved"]')].map(row=>row.dataset.sessionId).sort());
  const profiles=page=>page.evaluate(()=>state.terminalProfiles.map(p=>p.id).sort());
  const hasRow=(page,id)=>page.waitForFunction(id=>!!document.querySelector('#sessionSavedList [data-session-id="'+id+'"]'),id);
  const noRow=(page,id)=>page.waitForFunction(id=>!document.querySelector('#sessionSavedList [data-session-id="'+id+'"]'),id);
  // What a user does in the Save session dialog, without typing in it.
  const addProfile=(page,id,name,host)=>page.evaluate(([id,name,host])=>{state.terminalProfiles=[{id,name,nameExplicit:true,nameRevision:0,_nameNew:true,folder:'',...safeTerminalConfig({type:'ssh',host,port:22,username:'operator'})},...state.terminalProfiles];saveState();NetworkToolkitSessionSidebar.render();},[id,name,host]);
  const settled=page=>page.waitForFunction(()=>saveTimer===null&&stateRevision===(savedStateRevision<0?0:savedStateRevision)&&!NetworkToolkitSharedData.busy());

  // The page's refresh uses the same clean-up as loading settings.
  const same=await main.evaluate(async()=>{const served=await api('/state'),fresh=NetworkToolkitSharedData.normalize(served);return NetworkToolkitSharedData.LIVE.every(key=>JSON.stringify(fresh[key])===JSON.stringify(state[key]));});
  assert(same,'a refreshed list must look exactly like a loaded one');check('Refreshing shared values gives the same result as loading settings');

  // 1. A connection saved in the standalone Terminal appears in the main
  //    window's Saved sidebar while that window is idle.
  await addProfile(terminal,'from-terminal','Core switch','192.0.2.10');
  await hasRow(terminal,'from-terminal');await settled(terminal);
  await hasRow(main,'from-terminal');
  assert.deepEqual(await rows(main),['from-terminal']);check('A connection saved in the Terminal appears in the main window');

  // 2. The other direction.
  await addProfile(main,'from-main','Edge router','192.0.2.20');
  await settled(main);await hasRow(terminal,'from-main');
  assert.deepEqual(await rows(terminal),['from-main','from-terminal']);check('A connection saved in the main window appears in the Terminal');

  // 3. A save that changes nothing shared does not change the revision, so
  //    no other app refreshes for it.
  const before=await terminal.evaluate(()=>NetworkToolkitSharedData.revision);
  await terminal.evaluate(()=>{state.terminalComposerCollapsed=!state.terminalComposerCollapsed;saveState();});
  await settled(terminal);
  assert.equal(await terminal.evaluate(()=>NetworkToolkitSharedData.revision),before);
  assert.deepEqual(await profiles(terminal),['from-main','from-terminal']);check('A window preference does not disturb the shared lists');

  // 4. A page that has not refreshed saves its whole state: nothing another
  //    app added is removed, and the page receives what it was missing.
  await main.evaluate(()=>NetworkToolkitSharedData.stop());
  await addProfile(terminal,'while-stale','Access point','192.0.2.30');
  await settled(terminal);
  await main.waitForTimeout(600);
  assert.deepEqual(await profiles(main),['from-main','from-terminal'],'the main page is stale now');
  await main.evaluate(()=>{setTheme(state.theme==='dark'?'light':'dark');});
  await hasRow(main,'while-stale');
  assert.deepEqual(await rows(main),['from-main','from-terminal','while-stale']);
  await main.evaluate(()=>NetworkToolkitSharedData.start());
  await terminal.waitForTimeout(3200);
  assert.deepEqual(await rows(terminal),['from-main','from-terminal','while-stale'],'the stale save removed nothing');check('A save from a page that had not refreshed removes nothing and receives the merged list');

  // 5. An open dialog is not disturbed: the change waits until it closes.
  await main.evaluate(()=>openModal('Review','<p>Dialog under test</p>','<button class="btn primary" data-action="close-modal">Done</button>'));
  await addProfile(terminal,'during-dialog','Temporary','192.0.2.40');
  await settled(terminal);
  await main.waitForTimeout(3600);
  assert.deepEqual(await rows(main),['from-main','from-terminal','while-stale'],'no refresh while a dialog is open');
  assert.equal(await main.locator('#modalRoot .modal').count(),1,'the dialog stays open');
  await main.evaluate(()=>closeModal());
  await hasRow(main,'during-dialog');check('A change made elsewhere waits for an open dialog and is applied after it closes');

  // 6. A removal reaches the other apps too.
  await terminal.evaluate(()=>{state.terminalProfiles=state.terminalProfiles.filter(p=>p.id!=='during-dialog');saveState();NetworkToolkitSessionSidebar.render();});
  await settled(terminal);await noRow(main,'during-dialog');
  assert.deepEqual(await rows(main),['from-main','from-terminal','while-stale']);check('A removed connection disappears from the other app');

  // 7. The standalone Text Editor lists the saved connections.
  const listed=await editor.evaluate(async()=>(await NetworkToolkitEditorSavedConnections.list()).profiles.map(p=>p.id).sort());
  assert.deepEqual(listed,['ssh:from-main','ssh:from-terminal','ssh:while-stale']);
  await editor.waitForFunction(()=>state.terminalProfiles.length===3);
  assert.deepEqual(await profiles(editor),['from-main','from-terminal','while-stale'],'the editor page holds the shared list');check('The standalone Text Editor lists the saved connections');

  if(output){await main.screenshot({path:path.join(output,'shared-data-main.png')});await terminal.screenshot({path:path.join(output,'shared-data-terminal.png')});}
  assert.deepEqual(report.errors,[]);report.passed=true;
 }finally{
  if(output)fs.writeFileSync(path.join(output,'shared-data-report.json'),JSON.stringify(report,null,2));
  await browser.close();
 }
})().catch(error=>{console.error(error);console.error(JSON.stringify(report.errors));process.exit(1);});
