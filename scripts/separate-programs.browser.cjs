'use strict';
// Driven by TestSeparateProgramsBrowser (separate_programs_browser_test.go): the shipped pages
// against the real separate programs (docs/SEPARATE_EXECUTABLES.md), built from this checkout.
// The separate Terminal program holds a real Telnet session to a synthetic loopback device; the
// Text Editor runs in its separate program, opened through Network_Toolkit.exe --editor-standalone.
// 1. Text Editor -> Terminal: a selected line reaches the device through the Terminal program.
// 2. The Text Editor is closed; one Send to editor in the Terminal starts the separate Text Editor
//    program itself (no launch hook), and its page, opened here when its record appears, receives
//    the text in a new document.
// Chromium stands in for the app windows; no native Windows claim. Synthetic data only.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {NTK_SEPARATE_TERMINAL_URL:terminalURL,NTK_SEPARATE_EDITOR_URL:editorURL,NTK_SEPARATE_SESSION:session,NTK_SEPARATE_EDITOR_RECORD:editorRecord}=process.env;
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const errors=[];
const pass=text=>console.log('PASS '+text);
(async()=>{
 assert(terminalURL&&editorURL&&session&&editorRecord,'run through go test (TestSeparateProgramsBrowser)');
 if(output)fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,args:['--no-sandbox'],...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 // The page of a headless program carries its own session token.
 async function app(url,name){
  const context=await browser.newContext({viewport:{width:1280,height:800}}),page=await context.newPage();page.setDefaultTimeout(20000);
  page.on('pageerror',error=>errors.push(name+': '+error.message));
  await page.route('**/*',route=>new URL(route.request().url()).origin===url?route.continue():route.abort());
  await page.goto(url+'/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  return page;
 }
 const toast=(page,text)=>page.waitForFunction(t=>[...document.querySelectorAll('#toastStack .toast')].some(n=>n.textContent.includes(t)),text);
 const shot=async(page,name)=>{if(output)await page.screenshot({path:path.join(output,'separate-programs-'+name+'.png')});};
 const record=()=>{try{return JSON.parse(fs.readFileSync(editorRecord,'utf8'));}catch{return null;}};
 let terminal,editor;
 try{
  terminal=await app(terminalURL,'terminal');editor=await app(editorURL,'editor');
  assert.equal(await terminal.evaluate(()=>WINDOW_MODE),'terminal');assert.equal(await editor.evaluate(()=>WINDOW_MODE),'editor-standalone');
  await terminal.waitForFunction(id=>terminalSessions.get(id)?.info.state==='connected'&&currentTerminal()?.info.id===id&&terminalEntryAcceptsInput(currentTerminal()),session);
  await terminal.waitForFunction(()=>/lab-switch-01# $/.test(currentTerminal().xterm.buffer.active.getLine(currentTerminal().xterm.buffer.active.cursorY+currentTerminal().xterm.buffer.active.baseY)?.translateToString(true)||''));
  const name=await terminal.evaluate(id=>NetworkToolkitEditorSendTerminal.label(terminalSessions.get(id)),session);

  // ---- 1. Text Editor program -> Terminal program ----
  const text=editor.locator('#networkToolkitFileEditor [data-ed-text]');
  await editor.locator('#networkToolkitFileEditor [data-ed="new"]').first().click();
  await text.fill('show version\nshow clock\n');await text.evaluate(el=>{el.focus();el.setSelectionRange(0,12);});
  const send=editor.locator('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="sendTerminal"]');
  for(let attempt=0;;attempt++){await send.click();await editor.locator('#editorSendTerminalForm,[data-editor-send-peer-missing]').first().waitFor();if(await editor.locator('#editorSendTerminalForm').count())break;assert(attempt<20,'the Terminal program never listed its session');await editor.evaluate(()=>closeModal());await editor.waitForTimeout(300);}
  assert.equal(await editor.locator('#editorSendTarget optgroup').getAttribute('label'),'Network Toolkit Terminal');
  assert.deepEqual(await editor.locator('#editorSendTarget option').evaluateAll(options=>options.map(o=>o.value)),[session]);
  await editor.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  await toast(editor,'Sent to '+name+' in Network Toolkit Terminal.');
  await terminal.waitForFunction(()=>{const b=currentTerminal().xterm.buffer.active;for(let y=0;y<b.length;y++)if(/lab-switch-01# show version$/.test(b.getLine(y)?.translateToString(true)||''))return true;return false;});
  await shot(terminal,'terminal-received');
  pass('Separate Text Editor program to separate Terminal program: the selected line reached the real session');

  // ---- 2. The Text Editor closes; the Terminal program starts the separate Text Editor program ----
  const before=record();assert(before?.instanceId,'the Text Editor program registered its record');
  // Close as the window does: the page's own shutdown request (no unsaved work is kept: restore is off).
  await editor.evaluate(()=>fetch('/api/shutdown',{method:'POST',headers:{'X-Session-Token':SESSION}}));
  await editor.context().close();editor=null;
  for(let i=0;record()?.instanceId===before.instanceId;i++){assert(i<150,'the Text Editor program did not close');await new Promise(r=>setTimeout(r,100));}
  await terminal.evaluate(()=>{const x=currentTerminal().xterm,b=x.buffer.active;for(let y=0;y<b.length;y++){const line=b.getLine(y)?.translateToString(true)||'';if(line.startsWith('Synthetic Telnet device')){x.select(0,y,line.length);break;}}});
  assert.equal(await terminal.evaluate(()=>currentTerminal().xterm.getSelection()),'Synthetic Telnet device');
  await terminal.locator('#terminalTabs [data-terminal-tab="'+session+'"]').click({button:'right'});
  const item=terminal.locator('#terminalTabContextMenu [data-terminal-tab-action="send-editor"]');await item.waitFor();await item.click();
  await terminal.locator('#terminalSendEditorForm').waitFor();
  assert.match(await terminal.locator('[data-terminal-send-peer-missing]').innerText(),/Network Toolkit Text Editor is not open\. It opens when you choose Send to editor\./);
  await terminal.locator('[name="terminalSendSource"][value="selection"]').check();
  await terminal.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
  await toast(terminal,'Opening Network Toolkit Text Editor… The text is sent when it is ready.');
  // The started program is headless here: open its page as its window would, once it registered.
  let started=null;for(let i=0;!(started=record())||started.instanceId===before.instanceId;i++){assert(i<200,'the separate Text Editor program was not started');await new Promise(r=>setTimeout(r,100));}
  editor=await app('http://'+started.host,'started editor');
  await toast(terminal,'Sent to a new document in Network Toolkit Text Editor.');
  await editor.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]')?.value==='Synthetic Telnet device');
  assert.equal(await editor.evaluate(()=>NetworkToolkitFileEditor.listDocuments().find(d=>d.active)?.dirty),true);
  await shot(editor,'started-editor-received');
  pass('Nothing open: Send to editor in the Terminal program started the separate Text Editor program, and the text arrived in a new document');
  assert.deepEqual(errors,[]);
 }catch(error){
  try{for(const [name,page] of [['terminal',terminal],['editor',editor]]){if(!page)continue;console.error(name+': '+JSON.stringify(await page.evaluate(()=>({page:state.activePage,modal:document.querySelector('#modalTitle')?.textContent||'',toasts:[...document.querySelectorAll('#toastStack .toast')].map(n=>n.textContent)}))));if(output)await page.screenshot({path:path.join(output,'separate-programs-failure-'+name+'.png')});}}catch{}
  throw error;
 }finally{await browser.close();}
 console.log(JSON.stringify({passed:true}));
})().catch(error=>{console.error(error);console.error(JSON.stringify(errors));process.exit(1);});
