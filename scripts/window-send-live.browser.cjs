'use strict';
// Driven by TestWindowSendBrowser (cmd/network-toolkit/window_send_browser_test.go): the shipped
// page as the main Toolkit window, one detached Terminal window and one detached Text Editor window,
// all of ONE real engine, with real Telnet sessions to synthetic loopback devices. Nothing here is
// intercepted. Owner request: popped-out windows keep the single window's features between them, so
// "Send to terminal" in the editor window reaches a session in the Terminal window or the main
// window, and "Send to Text Editor" in the Terminal window reaches the editor window's document or a
// new document in the main window, each through the receiving window's own paste or insert path.
// Chromium tabs stand in for the native windows; no native Windows claim.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const E=process.env,url=E.NTK_WINDOW_URL,output=E.TOOLKIT_TEST_OUTPUT||E.NTK_BROWSER_OUTPUT_DIR;
const mainSession=E.NTK_WINDOW_MAIN_SESSION,viewSession=E.NTK_WINDOW_VIEW_SESSION;
const checks=[],errors=[];
const pass=text=>{checks.push(text);console.log('PASS '+text);};
(async()=>{
 assert(url&&E.NTK_WINDOW_MAIN_TOKEN&&E.NTK_WINDOW_TERMINAL_LAUNCH&&E.NTK_WINDOW_EDITOR_LAUNCH&&mainSession&&viewSession,'run through go test (TestWindowSendBrowser)');
 if(output)fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,args:['--no-sandbox'],...(E.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:E.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 async function window(pathname,token,name,ready){
  const context=await browser.newContext({viewport:{width:1280,height:800}}),page=await context.newPage();page.setDefaultTimeout(20000);
  page.on('pageerror',error=>errors.push(name+': '+error.message));
  await page.route('**/*',route=>new URL(route.request().url()).origin===url?route.continue():route.abort());
  await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},token);
  await page.goto(url+pathname);try{await page.waitForFunction(ready);}catch(error){console.error(name+' did not open: '+JSON.stringify(await page.evaluate(()=>({online:typeof online!=='undefined'&&online,error:typeof terminalViewError!=='undefined'?terminalViewError:'',toasts:[...document.querySelectorAll('#toastStack .toast')].map(n=>n.textContent)}))));throw error;}
  return page;
 }
 const toast=(page,text)=>page.waitForFunction(t=>[...document.querySelectorAll('#toastStack .toast')].some(n=>n.textContent.includes(t)),text);
 const shot=async(page,name)=>{if(output)await page.screenshot({path:path.join(output,'window-send-'+name+'.png')});};
 const screen=(page,pattern)=>page.waitForFunction(source=>{const e=currentTerminal(),b=e?.xterm.buffer.active,re=new RegExp(source);for(let y=0;b&&y<b.length;y++)if(re.test(b.getLine(y)?.translateToString(true)||''))return true;return false;},pattern.source);
 let main,term,ed;
 const describe=async()=>{for(const [name,page] of [['main',main],['terminal',term],['editor',ed]]){if(!page)continue;try{console.error(name+': '+JSON.stringify(await page.evaluate(()=>({page:state.activePage,modal:document.querySelector('#modalTitle')?.textContent||'',toasts:[...document.querySelectorAll('#toastStack .toast')].map(n=>n.textContent),text:document.querySelector('#networkToolkitFileEditor [data-ed-text]')?.value}))));if(output)await page.screenshot({path:path.join(output,'window-send-failure-'+name+'.png')});}catch(error){console.error('Could not describe '+name+': '+error.message);}}};
 try{
  main=await window('/',E.NTK_WINDOW_MAIN_TOKEN,'main window',()=>window.__networkToolkitStartupState===1&&online);
  // The main window pops the second session out with its own code (Move to a new window).
  await main.waitForFunction(id=>terminalSessions.get(id)?.info.state==='connected',viewSession);
  await main.evaluate(id=>detachTerminal(terminalSessions.get(id)),viewSession);
  const launched=async file=>{for(let i=0;i<100;i++){try{return JSON.parse(fs.readFileSync(file,'utf8'));}catch{await new Promise(r=>setTimeout(r,100));}}return null;};
  const launch=await launched(E.NTK_WINDOW_TERMINAL_LAUNCH);
  assert(launch?.view&&launch?.token,'the main window started the Terminal window');
  term=await window('/terminal-view/'+launch.view,launch.token,'Terminal window',()=>typeof terminalDetachedReady!=='undefined'&&terminalDetachedReady&&online);
  // The main window's Text Editor opens a document and pops it out into a new window with its own code.
  await main.evaluate(async()=>{navigate('editor');const ed=NetworkToolkitFileEditor,doc=await ed.newDocument();const r=await ed.insertText(doc.id,'interface Gi0/1\n','start');if(!r?.ok)throw new Error(r?.error||'insert');});
  await main.waitForFunction(()=>NetworkToolkitEditorWindows.state.enabled);
  await main.evaluate(()=>NetworkToolkitEditorWindows.detachEditor());
  const editorLaunch=await launched(E.NTK_WINDOW_EDITOR_LAUNCH);
  assert(editorLaunch?.view&&editorLaunch?.token,'the main window started the Text Editor window');
  await main.evaluate(()=>navigate('terminal'));
  ed=await window('/editor-view/'+editorLaunch.view,editorLaunch.token,'Text Editor window',()=>online&&NetworkToolkitFileEditor.listDocuments().length===1);
  for(const [page,kind] of [[main,'main'],[term,'terminal'],[ed,'editor']])assert.equal(await page.evaluate(()=>NetworkToolkitWindowSend.kind()+':'+NetworkToolkitWindowSend.enabled()),kind+':true');
  await term.waitForFunction(id=>terminalSessions.get(id)?.info.state==='connected'&&currentTerminal()?.info.id===id&&terminalEntryAcceptsInput(currentTerminal()),viewSession);
  await screen(term,/lab-switch-01# $/);
  await main.waitForFunction(id=>terminalSessions.get(id)?.info.state==='connected'&&currentTerminal()?.info.id===id&&terminalEntryAcceptsInput(currentTerminal()),mainSession);
  assert.equal(await main.evaluate(id=>NetworkToolkitEditorSendTerminal.targets().some(e=>e.info.id===id),viewSession),false,'the main window does not show the detached session itself');

  // ---- Text Editor window -> the Terminal window's session: one line, no review, no Enter ----
  const text=ed.locator('#networkToolkitFileEditor [data-ed-text]');
  await text.fill('show version\nshow clock\n');await text.evaluate(el=>{el.focus();el.setSelectionRange(0,12);});
  const send=ed.locator('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="sendTerminal"]');
  const openSend=async()=>{for(let attempt=0;;attempt++){await send.click();await ed.locator('#modalRoot .modal').first().waitFor();if(await ed.locator('#editorSendTerminalForm optgroup').count()===2)return;assert(attempt<20,'the other windows never listed their sessions');await ed.evaluate(()=>closeModal());await ed.waitForTimeout(400);}};
  await openSend();
  assert.deepEqual(await ed.locator('#editorSendTarget optgroup').evaluateAll(groups=>groups.map(g=>g.label+': '+[...g.querySelectorAll('option')].length)),['Main window: 1','Terminal window: 1']);
  await shot(ed,'editor-dialog');
  const option=async(page,group)=>page.locator('#editorSendTarget optgroup[label="'+group+'"] option').first().getAttribute('value');
  await ed.locator('#editorSendTarget').selectOption(await option(ed,'Terminal window'));
  await ed.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  await toast(ed,'in Terminal window.');
  await screen(term,/lab-switch-01# show version$/);
  assert.equal(await term.locator('#terminalPasteForm').count(),0,'one line needs no review');
  pass('Text Editor window to the Terminal window: the selected line reached the detached session through its paste path, with no Enter');

  // ---- A dialog open in the Terminal window is never replaced ----
  await term.evaluate(()=>openModal('Synthetic dialog','<p>Busy</p>','<button class="btn" data-action="close-modal">Close</button>'));
  await openSend();await ed.locator('#editorSendTarget').selectOption(await option(ed,'Terminal window'));
  await ed.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  await toast(ed,'Terminal window: A dialog is open in the Terminal window. Finish or close it there, then send again.');
  assert.equal(await term.locator('#modalTitle').textContent(),'Synthetic dialog');
  await term.evaluate(()=>closeModal());
  pass('Text Editor window to the Terminal window: refused while a dialog is open there, nothing was sent');

  // ---- Text Editor window -> the main window's session: line breaks wait for review there ----
  await text.fill('show clock\n');await text.evaluate(el=>{el.focus();el.setSelectionRange(0,0);});
  await openSend();await ed.locator('[name="editorSendWhat"][value="document"]').check();
  await ed.locator('#editorSendTarget').selectOption(await option(ed,'Main window'));
  await ed.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  await toast(ed,'Main window is showing the text for review');
  await main.locator('#terminalPasteForm').waitFor();assert.equal(await main.locator('#terminalPasteText').inputValue(),'show clock\n');
  await shot(main,'main-review');
  await main.locator('#terminalPasteMode').selectOption('raw');await main.locator('#terminalPasteConfirm').click();await main.locator('#terminalPasteForm').waitFor({state:'detached'});
  await screen(main,/^10:00:00\.000 UTC Thu Nov 5 2026/);
  pass('Text Editor window to the main window: text with a line break waited in the main window\'s paste review and was sent once confirmed');

  // ---- Terminal window -> the Text Editor window's document, and a new document in the main window ----
  await term.evaluate(()=>{const x=currentTerminal().xterm,b=x.buffer.active;for(let y=0;y<b.length;y++){const line=b.getLine(y)?.translateToString(true)||'';if(line.startsWith('lab-switch-01# show version')){x.select(0,y,line.length);break;}}});
  // A single-session Terminal window has no tab strip: its console bar's "To editor" button.
  const toEditor=async()=>{const button=term.locator('#terminalConsoleBar [data-terminal-extra="send-editor"]');await button.waitFor();assert.equal(await button.isEnabled(),true,'Send to Text Editor is offered in a detached Terminal window');await button.click();await term.locator('#terminalSendEditorForm').waitFor();};
  await toEditor();
  assert.deepEqual(await term.locator('#terminalSendDocument option').allTextContents(),['1. Untitled.txt (unsaved) · Text Editor window']);
  assert.match(await term.locator('[name="terminalSendTarget"][value="new"]').locator('xpath=..').textContent(),/New document \(in Main window\)/);
  await term.locator('[name="terminalSendSource"][value="selection"]').check();await term.locator('#terminalSendWhere').selectOption('end');
  await shot(term,'terminal-dialog');
  const before=await text.inputValue();
  await term.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
  await toast(term,'Sent to Untitled.txt in Text Editor window.');
  await ed.waitForFunction(expected=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value===expected,before+'lab-switch-01# show version');
  pass('Terminal window to the Text Editor window: selected terminal text was added at the end of its document');
  const documents=await main.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length);
  await toEditor();await term.locator('[name="terminalSendSource"][value="selection"]').check();await term.locator('[name="terminalSendTarget"][value="new"]').check();
  await term.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
  await toast(term,'Sent to a new document in Main window.');
  await main.waitForFunction(n=>NetworkToolkitFileEditor.listDocuments().length===n+1&&state.activePage==='editor',documents);
  await main.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value==='lab-switch-01# show version');
  await shot(main,'main-received');
  pass('Terminal window to the main window: a new, unsaved document holds the text');

  // ---- The main window lists the detached windows' sessions and documents ----
  await main.evaluate(()=>navigate('terminal'));
  const mainLists=await main.evaluate(async()=>[await NetworkToolkitWindowSend.targets('terminal'),await NetworkToolkitWindowSend.targets('editor')]);
  assert.deepEqual(mainLists[0].windows.map(w=>w.name+':'+w.targets.length),['Terminal window:1']);
  assert.deepEqual(mainLists[1].windows.map(w=>w.name+':'+w.targets.map(t=>t.name).join()),['Text Editor window:Untitled.txt']);
  pass('The main window lists the Terminal window\'s session and the Text Editor window\'s document as targets');
  for(const page of [main,term,ed])assert.equal(await page.evaluate(()=>NetworkToolkitPeerSend.sender()),'','no window of the main engine sends through peer send');
  assert.deepEqual(errors,[]);
  if(output)fs.writeFileSync(path.join(output,'window-send-report.json'),JSON.stringify({passed:true,checks},null,1));
  await browser.close();
 }catch(error){await describe();console.error(error);errors.length&&console.error(errors.join('\n'));await browser.close();process.exit(1);}
})();
