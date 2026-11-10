'use strict';
// Driven by TestStandalonePeerSendBrowser (peer_send_browser_test.go): the shipped pages of the
// standalone Terminal and the standalone Text Editor, each against its own real engine. The two
// engines talk to each other over real loopback HTTP through their instance records, and the
// Terminal holds a real Telnet session to a synthetic loopback device started by the Go test.
// Nothing here is intercepted. Chromium stands in for the app windows; no native Windows claim.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {NTK_PEER_TERMINAL_URL:terminalURL,NTK_PEER_TERMINAL_TOKEN:terminalToken,NTK_PEER_EDITOR_URL:editorURL,NTK_PEER_EDITOR_TOKEN:editorToken,NTK_PEER_SESSION:session}=process.env;
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const checks=[],errors=[];
const pass=text=>{checks.push(text);console.log('PASS '+text);};
(async()=>{
 assert(terminalURL&&terminalToken&&editorURL&&editorToken&&session,'run through go test (TestStandalonePeerSendBrowser)');
 if(output)fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,args:['--no-sandbox'],...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 async function app(url,token,name){
  const context=await browser.newContext({viewport:{width:1280,height:800}}),page=await context.newPage();page.setDefaultTimeout(20000);
  page.on('pageerror',error=>errors.push(name+': '+error.message));
  // Only loopback requests to this app's own engine are allowed.
  await page.route('**/*',route=>new URL(route.request().url()).origin===url?route.continue():route.abort());
  await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},token);
  await page.goto(url+'/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
  return page;
 }
 const toast=(page,text)=>page.waitForFunction(t=>[...document.querySelectorAll('#toastStack .toast')].some(n=>n.textContent.includes(t)),text);
 const shot=async(page,name)=>{if(output)await page.screenshot({path:path.join(output,'standalone-peer-live-'+name+'.png')});};
 let terminal,editor;
 // On failure, say what each window showed (synthetic data only).
 const describe=async()=>{try{console.error('Terminal screen: '+JSON.stringify(await terminal.evaluate(()=>{const e=currentTerminal(),b=e?.xterm.buffer.active,lines=[];for(let y=0;b&&y<b.length;y++){const line=b.getLine(y)?.translateToString(true);if(line)lines.push(line);}return {active:e?.info.id,state:e?.info.state,lines,modal:document.querySelector('#modalTitle')?.textContent||'',toasts:[...document.querySelectorAll('#toastStack .toast')].map(n=>n.textContent)};})));console.error('Editor window: '+JSON.stringify(await editor.evaluate(()=>({text:document.querySelector('#networkToolkitFileEditor [data-ed-text]')?.value,modal:document.querySelector('#modalTitle')?.textContent||'',toasts:[...document.querySelectorAll('#toastStack .toast')].map(n=>n.textContent)}))));if(output){await terminal.screenshot({path:path.join(output,'standalone-peer-live-failure-terminal.png')});await editor.screenshot({path:path.join(output,'standalone-peer-live-failure-editor.png')});}}catch(error){console.error('Could not describe the windows: '+error.message);}};
 try{
  terminal=await app(terminalURL,terminalToken,'terminal');editor=await app(editorURL,editorToken,'editor');
  assert.equal(await terminal.evaluate(()=>WINDOW_MODE),'terminal');assert.equal(await editor.evaluate(()=>WINDOW_MODE),'editor-standalone');
  await terminal.waitForFunction(id=>terminalSessions.get(id)?.info.state==='connected'&&currentTerminal()?.info.id===id&&terminalEntryAcceptsInput(currentTerminal()),session);
  await terminal.waitForFunction(()=>/lab-switch-01# $/.test(currentTerminal().xterm.buffer.active.getLine(currentTerminal().xterm.buffer.active.cursorY+currentTerminal().xterm.buffer.active.baseY)?.translateToString(true)||''));
  const name=await terminal.evaluate(id=>NetworkToolkitEditorSendTerminal.label(terminalSessions.get(id)),session);

  // ---- Text Editor -> Terminal: one selected line, entered without review ----
  const text=editor.locator('#networkToolkitFileEditor [data-ed-text]');
  await editor.locator('#networkToolkitFileEditor [data-ed="new"]').first().click();
  await text.fill('show version\nshow clock\n');await text.evaluate(el=>{el.focus();el.setSelectionRange(0,12);});
  const send=editor.locator('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="sendTerminal"]');
  // The Terminal page reports its sessions within a second or two of starting.
  for(let attempt=0;;attempt++){await send.click();await editor.locator('#editorSendTerminalForm,[data-editor-send-peer-missing]').first().waitFor();if(await editor.locator('#editorSendTerminalForm').count())break;assert(attempt<20,'the Terminal app never listed its session');await editor.evaluate(()=>closeModal());await editor.waitForTimeout(300);}
  assert.equal(await editor.locator('#editorSendTarget optgroup').getAttribute('label'),'Network Toolkit Terminal');
  assert.deepEqual(await editor.locator('#editorSendTarget option').evaluateAll(options=>options.map(o=>o.value)),[session],'the real session of the Terminal app is listed');
  assert.equal(await editor.locator('#editorSendTarget option').first().textContent(),name+' · Telnet');
  await shot(editor,'editor-dialog');
  await editor.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  await toast(editor,'Sent to '+name+' in Network Toolkit Terminal.');
  await terminal.waitForFunction(()=>{const b=currentTerminal().xterm.buffer.active;for(let y=0;y<b.length;y++)if(/lab-switch-01# show version$/.test(b.getLine(y)?.translateToString(true)||''))return true;return false;});
  assert.equal(await terminal.locator('#terminalPasteForm').count(),0,'one line needs no review');
  pass('Text Editor to Terminal: the selected line reached the real session through the Terminal app, with no Enter added');
  await shot(terminal,'terminal-after-line');
  // Finish that command in the Terminal itself, then send two lines: they wait for review there.
  await terminal.evaluate(()=>currentTerminal().xterm.focus());await terminal.keyboard.press('Enter');
  await terminal.waitForFunction(()=>{const b=currentTerminal().xterm.buffer.active;for(let y=0;y<b.length;y++)if(/synthetic reply to show version/.test(b.getLine(y)?.translateToString(true)||''))return true;return false;});
  await text.fill('show clock\n\n');await text.evaluate(el=>{el.focus();el.setSelectionRange(0,0);});
  await editor.keyboard.press('Control+Shift+Enter');await editor.locator('#editorSendTerminalForm').waitFor();
  assert.equal(await editor.locator('[name="editorSendWhat"][value="document"]').isChecked(),true);
  await editor.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  await toast(editor,'Network Toolkit Terminal is showing the text for review before it goes to '+name+'.');
  await terminal.locator('#terminalPasteForm').waitFor();assert.equal(await terminal.locator('#terminalPasteText').inputValue(),'show clock\n\n','the Terminal reviews exactly the text that was sent');
  await shot(terminal,'terminal-review');
  // A second send while the review is open is refused by the Terminal app.
  await text.evaluate(el=>{el.focus();el.setSelectionRange(0,4);});await send.click();await editor.locator('#editorSendTerminalForm').waitFor();await editor.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  await toast(editor,'Network Toolkit Terminal: A dialog is open in Network Toolkit Terminal. Finish or close it there, then send again.');
  assert.equal(await terminal.locator('#terminalPasteText').inputValue(),'show clock\n\n');
  await terminal.locator('#terminalPasteMode').selectOption('raw');await terminal.locator('#terminalPasteConfirm').click();await terminal.locator('#terminalPasteForm').waitFor({state:'detached'});
  await terminal.waitForFunction(()=>{const b=currentTerminal().xterm.buffer.active;for(let y=0;y<b.length;y++)if(/^10:00:00\.000 UTC Thu Nov 5 2026/.test(b.getLine(y)?.translateToString(true)||''))return true;return false;});
  pass('Text Editor to Terminal: text with line breaks waited in the Terminal\'s paste review, a second send was refused meanwhile, and confirming sent it to the device');

  // ---- Terminal -> Text Editor ----
  await terminal.evaluate(()=>{const x=currentTerminal().xterm,b=x.buffer.active;for(let y=0;y<b.length;y++){const line=b.getLine(y)?.translateToString(true)||'';if(line.startsWith('10:00:00.000')){x.select(0,y,line.length);break;}}});
  assert.equal(await terminal.evaluate(()=>currentTerminal().xterm.getSelection()),'10:00:00.000 UTC Thu Nov 5 2026');
  const openToEditor=async()=>{await terminal.locator('#terminalTabs [data-terminal-tab="'+session+'"]').click({button:'right'});const item=terminal.locator('#terminalTabContextMenu [data-terminal-tab-action="send-editor"]');await item.waitFor();assert.equal(await item.isEnabled(),true);await item.click();await terminal.locator('#terminalSendEditorForm').waitFor();};
  await openToEditor();
  assert.equal(await terminal.locator('#terminalSendEditorForm fieldset legend').nth(1).textContent(),'To Network Toolkit Text Editor');
  assert.deepEqual(await terminal.locator('#terminalSendDocument option').allTextContents(),['1. Untitled.txt (unsaved)'],'the real document of the Text Editor app is listed by name');
  await terminal.locator('[name="terminalSendSource"][value="selection"]').check();await terminal.locator('#terminalSendWhere').selectOption('end');
  await shot(terminal,'terminal-dialog');
  const before=await text.inputValue();
  await terminal.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
  await toast(terminal,'Sent to Untitled.txt in Network Toolkit Text Editor.');
  await editor.waitForFunction(expected=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value===expected,before+'10:00:00.000 UTC Thu Nov 5 2026');
  pass('Terminal to Text Editor: selected terminal text was added at the end of the chosen open document');
  // A position the user chooses in the Text Editor app.
  await openToEditor();await terminal.locator('[name="terminalSendSource"][value="selection"]').check();assert.equal(await terminal.locator('#terminalSendWhere').inputValue(),'pick');
  await terminal.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
  await toast(terminal,'Network Toolkit Text Editor is asking where the text goes in Untitled.txt.');
  await editor.locator('[data-ed-insert-bar]').waitFor();const held=await text.inputValue();
  await text.evaluate(el=>{el.focus();el.setSelectionRange(0,0);});await editor.locator('[data-ed-insert-here]').click();
  await editor.waitForFunction(expected=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value===expected,'10:00:00.000 UTC Thu Nov 5 2026'+held);
  // A new document.
  const documents=await editor.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length);
  await openToEditor();await terminal.locator('[name="terminalSendSource"][value="selection"]').check();await terminal.locator('[name="terminalSendTarget"][value="new"]').check();
  await terminal.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
  await toast(terminal,'Sent to a new document in Network Toolkit Text Editor.');
  await editor.waitForFunction(n=>NetworkToolkitFileEditor.listDocuments().length===n+1,documents);
  await editor.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value==='10:00:00.000 UTC Thu Nov 5 2026');
  assert.equal(await editor.evaluate(()=>NetworkToolkitFileEditor.listDocuments().find(d=>d.active)?.dirty),true);
  await shot(editor,'editor-received');
  pass('Terminal to Text Editor: the Insert here bar and a new, unsaved document both work across the two apps');
  // Neither page ever holds the other app's tokens.
  for(const [page,other] of [[terminal,editorToken],[editor,terminalToken]])assert.equal(await page.evaluate(token=>document.documentElement.outerHTML.includes(token)||JSON.stringify(state).includes(token),other),false);
  assert.deepEqual(errors,[]);
 }catch(error){await describe();throw error;}finally{await browser.close();}
 console.log(JSON.stringify({passed:true,checks:checks.length}));
})().catch(error=>{console.error(error);console.error(JSON.stringify(errors));process.exit(1);});
