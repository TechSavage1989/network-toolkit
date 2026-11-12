'use strict';
// Driven by TestWindowSendRemoteFileBrowser (cmd/network-toolkit/window_send_remote_browser_test.go):
// the shipped page as the main Toolkit window and as its detached Terminal workspace, one real
// engine, and a real SSH session with SFTP to a synthetic loopback server. Owner decision: Edit in
// the workspace's SFTP browser opens the remote file in the main window's Text Editor on the same
// connection; it is edited and saved there with the main window's own checks. Nothing is
// intercepted. Chromium tabs stand in for the native windows; no native Windows claim.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const E=process.env,url=E.NTK_WINDOW_URL,session=E.NTK_WINDOW_SESSION,output=E.TOOLKIT_TEST_OUTPUT||E.NTK_BROWSER_OUTPUT_DIR;
const errors=[];
const pass=text=>console.log('PASS '+text);
(async()=>{
 assert(url&&E.NTK_WINDOW_MAIN_TOKEN&&E.NTK_WINDOW_TERMINAL_LAUNCH&&session,'run through go test (TestWindowSendRemoteFileBrowser)');
 if(output)fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,args:['--no-sandbox'],...(E.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:E.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 async function window(pathname,token,name,ready){
  const context=await browser.newContext({viewport:{width:1280,height:800}}),page=await context.newPage();page.setDefaultTimeout(20000);
  page.on('pageerror',error=>errors.push(name+': '+error.message));
  await page.route('**/*',route=>new URL(route.request().url()).origin===url?route.continue():route.abort());
  await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},token);
  await page.goto(url+pathname);await page.waitForFunction(ready);
  return page;
 }
 const toast=(page,text)=>page.waitForFunction(t=>[...document.querySelectorAll('#toastStack .toast')].some(n=>n.textContent.includes(t)),text);
 const shot=async(page,name)=>{if(output)await page.screenshot({path:path.join(output,'window-send-remote-'+name+'.png')});};
 let main,term;
 try{
  main=await window('/',E.NTK_WINDOW_MAIN_TOKEN,'main window',()=>window.__networkToolkitStartupState===1&&online);
  await main.waitForFunction(id=>terminalSessions.get(id)?.info.state==='connected',session);
  // The main window pops its Terminal workspace out with its own code.
  await main.evaluate(()=>detachTerminalWorkspace());
  let launch=null;for(let i=0;i<100&&!launch;i++){try{launch=JSON.parse(fs.readFileSync(E.NTK_WINDOW_TERMINAL_LAUNCH,'utf8'));}catch{await new Promise(r=>setTimeout(r,100));}}
  assert(launch?.view&&launch?.token,'the main window started the Terminal workspace window');
  term=await window('/terminal-view/'+launch.view,launch.token,'Terminal workspace',()=>typeof terminalDetachedReady!=='undefined'&&terminalDetachedReady&&online);
  assert.equal(await term.evaluate(()=>DETACHED_WORKSPACE),true);
  await term.waitForFunction(()=>NetworkToolkitWindowSend.enabled());
  // Its SFTP browser: read-only, with Edit.
  const sftpTab=term.locator('[data-session-tab="files"]').first();await sftpTab.waitFor();await sftpTab.click();
  const row=term.locator('#terminalSidebarSFTP [data-tsftp-item$="startup.cfg"]');await row.waitFor();
  assert.deepEqual(await term.locator('#terminalSidebarSFTP [data-tsftp]').evaluateAll(list=>list.map(b=>b.dataset.tsftp).filter(n=>!['up','home','root','refresh'].includes(n))),['edit','preview'],'only Edit and Preview act on files');
  await row.click();
  // The main window's page reports within a few seconds of the workspace opening.
  for(let attempt=0;;attempt++){
   await term.locator('#terminalSidebarSFTP [data-tsftp="edit"]').click();
   const outcome=await term.waitForFunction(()=>{const notes=[...document.querySelectorAll('#toastStack .toast')].map(n=>n.textContent);const opened=notes.find(t=>t.includes('Opened startup.cfg'));const failed=document.querySelector('#terminalSidebarSFTP [data-tsftp-note]:not([hidden])')?.textContent||'';return opened||failed||null;}).then(h=>h.jsonValue());
   if(/Opened startup\.cfg in the Text Editor of Main window\./.test(outcome))break;
   assert(attempt<10&&/not ready|Open the main/.test(outcome),outcome);
   await term.evaluate(()=>{const n=document.querySelector('#terminalSidebarSFTP [data-tsftp-note]');if(n){n.hidden=true;n.textContent='';}});await term.waitForTimeout(500);
  }
  await shot(term,'workspace');
  await main.waitForFunction(()=>state.activePage==='editor'&&NetworkToolkitFileEditor.active?.path?.endsWith('startup.cfg')&&!NetworkToolkitFileEditor.active.loading);
  const doc=await main.evaluate(()=>({source:NetworkToolkitFileEditor.active.source,kind:NetworkToolkitFileEditor.active.binding?.kind,content:NetworkToolkitFileEditor.active.content,eol:NetworkToolkitFileEditor.active.eol}));
  assert.equal(doc.kind,'terminal');assert.equal(doc.source,'SSH terminal');assert.match(doc.content,/^hostname test-switch\n?$/);
  pass('Edit in the detached workspace opened the remote file in the main window\'s Text Editor on the same SSH session');
  // Edit and save it in the main window.
  const text=main.locator('#networkToolkitFileEditor [data-ed-text]');
  await text.focus();await main.keyboard.press('Control+End');await main.keyboard.type('banner edited-in-main\n');
  await main.locator('#networkToolkitFileEditor [data-ed="save"]').first().click();
  await main.waitForFunction(()=>!NetworkToolkitFileEditor.active.dirty&&!NetworkToolkitFileEditor.active.saving);
  await shot(main,'saved');
  pass('the main window saved the remote file with its own save path');
  // The workspace still cannot write anything itself.
  await assert.rejects(term.evaluate(id=>NetworkToolkitTerminalSFTP.request(id,'write-text',{path:'startup.cfg',content:'x'}),session),/read-only/);

  // ---- Owner decision (v1.21.50): the main window is closed; Edit reopens it and opens the file there ----
  await main.context().close();main=null;
  fs.writeFileSync(E.NTK_WINDOW_MAIN_CLOSED,'closed');
  for(let i=0;fs.existsSync(E.NTK_WINDOW_MAIN_CLOSED);i++){assert(i<100,'the engine never saw the main window close');await new Promise(r=>setTimeout(r,100));}
  await term.evaluate(()=>{const stack=document.querySelector('#toastStack');if(stack)stack.innerHTML='';});
  // startup.cfg is still the selected file in the workspace's SFTP browser.
  await term.locator('#terminalSidebarSFTP [data-tsftp="edit"]').click();
  // The engine asks for the main window (the native host reopens it; here the Go test says so).
  let reopened=false;for(let i=0;i<150&&!reopened;i++){reopened=fs.existsSync(E.NTK_WINDOW_MAIN_REOPEN);if(!reopened)await new Promise(r=>setTimeout(r,100));}
  assert(reopened,'Edit did not ask for the closed main window');
  await toast(term,'Opening the main window to edit startup.cfg');
  main=await window('/',E.NTK_WINDOW_MAIN_TOKEN,'reopened main window',()=>window.__networkToolkitStartupState===1&&online);
  await toast(term,'Opened startup.cfg in the Text Editor of Main window.');
  await main.waitForFunction(()=>state.activePage==='editor'&&NetworkToolkitFileEditor.active?.path?.endsWith('startup.cfg')&&!NetworkToolkitFileEditor.active.loading);
  assert.match(await main.evaluate(()=>NetworkToolkitFileEditor.active.content),/banner edited-in-main/,'the reopened main window shows the saved file');
  assert.equal(await main.evaluate(()=>NetworkToolkitFileEditor.active.binding?.kind),'terminal');
  await shot(main,'reopened');
  pass('With the main window closed, Edit in the workspace reopened it and opened the remote file there');
  // The same grant rules: only that path, never a save elsewhere.
  await assert.rejects(main.evaluate(id=>api('/remote-browser','POST',{protocol:'ssh',sessionId:id,action:'read-text',path:'other.cfg'}),session),/./);
  assert.deepEqual(errors,[]);
  console.log('window-send-remote-live: OK');
 }catch(error){
  for(const [name,page] of [['main',main],['terminal',term]]){try{if(page){console.error(name+': '+JSON.stringify(await page.evaluate(()=>({page:state.activePage,toasts:[...document.querySelectorAll('#toastStack .toast')].map(n=>n.textContent),note:document.querySelector('#terminalSidebarSFTP [data-tsftp-note]')?.textContent||''}))));if(output)await page.screenshot({path:path.join(output,'window-send-remote-failure-'+name+'.png')});}}catch{}}
  console.error(error);process.exitCode=1;
 }finally{
  if(errors.length){console.error('page errors: '+errors.join(' | '));process.exitCode=1;}
  await browser.close();
 }
})();
