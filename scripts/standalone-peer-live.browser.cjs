'use strict';
// Driven by TestStandalonePeerSendBrowser (peer_send_browser_test.go): the shipped pages of the
// standalone Terminal and the standalone Text Editor, each against its own real engine. The two
// engines talk to each other over real loopback HTTP through their instance records, and the
// Terminal holds a real Telnet session to a synthetic loopback device started by the Go test.
// Nothing here is intercepted. Chromium stands in for the app windows; no native Windows claim.
// Receiver order (owner request), each with real engines and real pages:
//   both running  - the two standalone apps send to each other (the first part below);
//   only main     - a standalone Text Editor sends to the main Toolkit window's Terminal page and a
//                   standalone Terminal to its Text Editor;
//   nothing runs  - one Send starts the other standalone app and the text that was sent arrives.
//                   The Go test "starts" an app by registering an engine whose page this script
//                   already loaded; a process start itself is not exercised here.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {NTK_PEER_TERMINAL_URL:terminalURL,NTK_PEER_TERMINAL_TOKEN:terminalToken,NTK_PEER_EDITOR_URL:editorURL,NTK_PEER_EDITOR_TOKEN:editorToken,NTK_PEER_SESSION:session}=process.env;
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR;
const more=JSON.parse(process.env.NTK_PEER_MORE||'null');
const checks=[],errors=[];
const pass=text=>{checks.push(text);console.log('PASS '+text);};
(async()=>{
 assert(terminalURL&&terminalToken&&editorURL&&editorToken&&session&&more?.main&&more?.start,'run through go test (TestStandalonePeerSendBrowser)');
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

  const screen=(page,pattern)=>page.waitForFunction(source=>{const e=currentTerminal(),b=e?.xterm.buffer.active,re=new RegExp(source);for(let y=0;b&&y<b.length;y++)if(re.test(b.getLine(y)?.translateToString(true)||''))return true;return false;},pattern.source);
  const selectLine=async(page,start)=>{await page.evaluate(prefix=>{const x=currentTerminal().xterm,b=x.buffer.active;for(let y=0;y<b.length;y++){const line=b.getLine(y)?.translateToString(true)||'';if(line.startsWith(prefix)){x.select(0,y,line.length);break;}}},start);return page.evaluate(()=>currentTerminal().xterm.getSelection());};
  const toEditorDialog=async(page,id)=>{await page.locator('#terminalTabs [data-terminal-tab="'+id+'"]').click({button:'right'});const item=page.locator('#terminalTabContextMenu [data-terminal-tab-action="send-editor"]');await item.waitFor();assert.equal(await item.isEnabled(),true);await item.click();await page.locator('#terminalSendEditorForm').waitFor();};
  const editorText=page=>page.locator('#networkToolkitFileEditor [data-ed-text]');
  const sendButton=page=>page.locator('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="sendTerminal"]');
  const submitTerminal=page=>page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  const submitEditor=page=>page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
  const ready=(page,id)=>page.waitForFunction(id=>terminalSessions.get(id)?.info.state==='connected'&&currentTerminal()?.info.id===id&&terminalEntryAcceptsInput(currentTerminal()),id);

  // ---- Only the main Toolkit window runs (the standalone apps here are senders, not registered) ----
  {
   const M=more.main,main=await app(M.main.url,M.main.token,'main window'),ed=await app(M.editor.url,M.editor.token,'editor (main only)'),term=await app(M.terminal.url,M.terminal.token,'terminal (main only)');
   try{
    assert.equal(await main.evaluate(()=>WINDOW_MODE),'main');assert.equal(await main.evaluate(()=>NetworkToolkitPeerSend.sender()),'','the main window never sends through peer send');
    await ready(main,M.main.session);await screen(main,/lab-switch-01# $/);
    const mainName=await main.evaluate(id=>NetworkToolkitEditorSendTerminal.label(terminalSessions.get(id)),M.main.session);
    // The main window is on another page: the text must bring its Terminal page forward.
    await main.evaluate(()=>navigate('overview'));await main.waitForFunction(()=>state.activePage==='overview');
    // Text Editor app -> the main window's Terminal page: one selected line, no review, no Enter.
    const text=editorText(ed);await ed.locator('#networkToolkitFileEditor [data-ed="new"]').first().click();
    await text.fill('show version\nshow clock\n');await text.evaluate(el=>{el.focus();el.setSelectionRange(0,12);});
    // The main window's page reports its sessions within a second or two of starting.
    for(let attempt=0;;attempt++){await sendButton(ed).click();await ed.locator('#editorSendTerminalForm,[data-editor-send-peer-missing]').first().waitFor();if(await ed.locator('#editorSendTerminalForm').count())break;assert(attempt<20,'the main window never listed its session');await ed.evaluate(()=>closeModal());await ed.waitForTimeout(300);}
    assert.equal(await ed.locator('#editorSendTarget optgroup').getAttribute('label'),'Network Toolkit','the sessions are those of the main Toolkit window');
    assert.deepEqual(await ed.locator('#editorSendTarget option').evaluateAll(options=>options.map(o=>o.value)),[M.main.session]);
    assert.equal(await ed.locator('#editorSendTarget option').first().textContent(),mainName+' · Telnet');
    await shot(ed,'main-only-editor-dialog');
    await submitTerminal(ed);await toast(ed,'Sent to '+mainName+' in Network Toolkit.');
    await main.waitForFunction(()=>state.activePage==='terminal');await screen(main,/lab-switch-01# show version$/);
    assert.equal(await main.locator('#terminalPasteForm').count(),0,'one line needs no review');
    pass('Only the main window runs: the Text Editor app\'s selected line reached the main window\'s real session, on its Terminal page, with no Enter added');
    // Text with a line break waits in the main window's own paste review.
    await main.evaluate(()=>navigate('overview'));
    await text.fill('show clock\n');await text.evaluate(el=>{el.focus();el.setSelectionRange(0,0);});
    await ed.keyboard.press('Control+Shift+Enter');await ed.locator('#editorSendTerminalForm').waitFor();
    assert.equal(await ed.locator('[name="editorSendWhat"][value="document"]').isChecked(),true);
    await submitTerminal(ed);await toast(ed,'Network Toolkit is showing the text for review before it goes to '+mainName+'.');
    await main.locator('#terminalPasteForm').waitFor();assert.equal(await main.locator('#terminalPasteText').inputValue(),'show clock\n','the main window reviews exactly the text that was sent');
    assert.equal(await main.evaluate(()=>state.activePage),'terminal');
    await shot(main,'main-only-review');
    // A second send while the review is open is refused by the main window, and the review stays.
    await text.evaluate(el=>{el.focus();el.setSelectionRange(0,4);});await sendButton(ed).click();await ed.locator('#editorSendTerminalForm').waitFor();await submitTerminal(ed);
    await toast(ed,'Network Toolkit: A dialog is open in Network Toolkit. Finish or close it there, then send again.');
    assert.equal(await main.locator('#terminalPasteText').inputValue(),'show clock\n');
    await main.locator('#terminalPasteMode').selectOption('raw');await main.locator('#terminalPasteConfirm').click();await main.locator('#terminalPasteForm').waitFor({state:'detached'});
    await screen(main,/synthetic reply to show versionshow clock/);
    pass('Only the main window runs: text with a line break waited in the main window\'s paste review, a second send was refused meanwhile, and confirming sent it');
    // Terminal app -> the main window's Text Editor.
    await ready(term,M.terminal.session);await screen(term,/^Synthetic Telnet device/);
    assert.equal(await selectLine(term,'Synthetic Telnet device'),'Synthetic Telnet device');
    await main.evaluate(()=>navigate('terminal'));
    await toEditorDialog(term,M.terminal.session);
    assert.equal(await term.locator('#terminalSendEditorForm fieldset legend').nth(1).textContent(),'To Network Toolkit');assert.equal(await term.locator('[data-terminal-send-peer-missing]').count(),0);
    await term.locator('[name="terminalSendSource"][value="selection"]').check();await term.locator('[name="terminalSendTarget"][value="new"]').check();
    await shot(term,'main-only-terminal-dialog');
    await submitEditor(term);await toast(term,'Sent to a new document in Network Toolkit.');
    await main.waitForFunction(()=>state.activePage==='editor'&&document.querySelector('#networkToolkitFileEditor [data-ed-text]')?.value==='Synthetic Telnet device');
    assert.equal(await main.evaluate(()=>NetworkToolkitFileEditor.listDocuments().find(d=>d.active)?.dirty),true);
    // An open document of the main window, at a position chosen there.
    await main.evaluate(()=>navigate('terminal'));
    let listed='';for(let attempt=0;;attempt++){await toEditorDialog(term,M.terminal.session);if(await term.locator('#terminalSendDocument option').count()){listed=await term.locator('#terminalSendDocument option').first().textContent();break;}assert(attempt<20,'the main window never listed its document');await term.evaluate(()=>closeModal());await term.waitForTimeout(300);}
    assert.match(listed,/^1\. .+ \(unsaved\)$/,'the main window\'s document is listed by name');
    await term.locator('[name="terminalSendSource"][value="selection"]').check();assert.equal(await term.locator('#terminalSendWhere').inputValue(),'pick');
    await submitEditor(term);await toast(term,'Network Toolkit is asking where the text goes in ');
    await main.locator('[data-ed-insert-bar]').waitFor();assert.equal(await main.evaluate(()=>state.activePage),'editor');
    const mainText=editorText(main);assert.equal(await mainText.inputValue(),'Synthetic Telnet device','nothing is inserted until the position is confirmed');
    await mainText.evaluate(el=>{el.focus();el.setSelectionRange(0,0);});await main.locator('[data-ed-insert-here]').click();
    await main.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value==='Synthetic Telnet deviceSynthetic Telnet device');
    await shot(main,'main-only-editor-received');
    pass('Only the main window runs: the Terminal app\'s selected text arrived in a new document and through the Insert here bar of the main window\'s Text Editor');
    for(const [page,tokens] of [[main,[M.editor.token,M.terminal.token]],[ed,[M.main.token,M.terminal.token]],[term,[M.main.token,M.editor.token]]])for(const token of tokens)assert.equal(await page.evaluate(token=>document.documentElement.outerHTML.includes(token)||JSON.stringify(state).includes(token),token),false);
   }catch(error){try{for(const [name,page] of [['main',main],['editor',ed],['terminal',term]]){console.error(name+' window (main only): '+JSON.stringify(await page.evaluate(()=>({page:state.activePage,modal:document.querySelector('#modalTitle')?.textContent||'',toasts:[...document.querySelectorAll('#toastStack .toast')].map(n=>n.textContent)}))));if(output)await page.screenshot({path:path.join(output,'standalone-peer-live-failure-main-only-'+name+'.png')});}}catch{}throw error;}
  }

  // ---- Nothing runs: one Send starts the other standalone app and the text arrives ----
  {
   const S=more.start,term=await app(S.terminal.url,S.terminal.token,'terminal (start)'),ed=await app(S.editor.url,S.editor.token,'editor (start)');
   try{
    await ready(term,S.terminal.session);await screen(term,/^Synthetic Telnet device/);
    const name=await term.evaluate(id=>NetworkToolkitEditorSendTerminal.label(terminalSessions.get(id)),S.terminal.session);
    assert.equal(await selectLine(term,'Synthetic Telnet device'),'Synthetic Telnet device');
    // Terminal -> Text Editor: nothing is running, so only a new document is offered.
    await toEditorDialog(term,S.terminal.session);
    assert.equal(await term.locator('#terminalSendEditorForm fieldset legend').nth(1).textContent(),'To Network Toolkit Text Editor');
    assert.match(await term.locator('[data-terminal-send-peer-missing]').innerText(),/Network Toolkit Text Editor is not open\. It opens when you choose Send to editor\./);
    await term.locator('[name="terminalSendSource"][value="selection"]').check();assert.equal(await term.locator('[name="terminalSendTarget"][value="new"]').isChecked(),true);
    const documents=await ed.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length);
    // ONE click on Send. The engine starts the Text Editor app and delivers this text itself.
    await submitEditor(term);
    await toast(term,'Opening Network Toolkit Text Editor… The text is sent when it is ready.');
    await shot(term,'start-terminal-opening');
    await toast(term,'Sent to a new document in Network Toolkit Text Editor.');
    await ed.waitForFunction(n=>NetworkToolkitFileEditor.listDocuments().length===n+1,documents);
    assert.equal(await editorText(ed).inputValue(),'Synthetic Telnet device','the text that was sent arrived, with no second click');
    await ed.waitForTimeout(1500);assert.equal(await ed.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length),documents+1,'the text arrived once');
    assert.equal(await term.locator('#modalRoot .modal').count(),0,'the Terminal asked nothing');
    await shot(ed,'start-editor-received');
    pass('Nothing runs: one Send to editor started the Text Editor app and the selected terminal text arrived in a new document, once');
    // Text Editor -> Terminal: the Terminal app is not registered yet, so ONE click on Send to
    // terminal starts it, with no question; its sessions are then listed in the usual dialog.
    const text=editorText(ed);await ed.locator('#networkToolkitFileEditor [data-ed="new"]').first().click();
    await text.fill('show version\nshow clock\n');await text.evaluate(el=>{el.focus();el.setSelectionRange(0,12);});
    await sendButton(ed).click();
    await toast(ed,'Opening Network Toolkit Terminal…');
    await ed.locator('#editorSendTerminalForm').waitFor();
    assert.equal(await ed.locator('[data-editor-send-peer-missing]').count(),0,'no "not open" question was shown');
    assert.equal(await ed.locator('#editorSendTarget optgroup').getAttribute('label'),'Network Toolkit Terminal');
    assert.deepEqual(await ed.locator('#editorSendTarget option').evaluateAll(options=>options.map(o=>o.value)),[S.terminal.session],'the started Terminal app\'s session is listed');
    assert.equal(await text.inputValue(),'show version\nshow clock\n','the document is unchanged');
    await shot(ed,'start-editor-dialog');
    await submitTerminal(ed);await toast(ed,'Sent to '+name+' in Network Toolkit Terminal.');
    await screen(term,/lab-switch-01# show version$/);assert.equal(await term.locator('#terminalPasteForm').count(),0);
    pass('Nothing runs: one Send to terminal started the Terminal app without a question, and the selected line reached its real session');
    for(const [page,token] of [[term,S.editor.token],[ed,S.terminal.token]])assert.equal(await page.evaluate(token=>document.documentElement.outerHTML.includes(token)||JSON.stringify(state).includes(token),token),false);
   }catch(error){try{for(const [name,page] of [['terminal',term],['editor',ed]]){console.error(name+' window (start): '+JSON.stringify(await page.evaluate(()=>({page:state.activePage,modal:document.querySelector('#modalTitle')?.textContent||'',toasts:[...document.querySelectorAll('#toastStack .toast')].map(n=>n.textContent)}))));if(output)await page.screenshot({path:path.join(output,'standalone-peer-live-failure-start-'+name+'.png')});}}catch{}throw error;}
  }
  assert.deepEqual(errors,[]);
 }catch(error){await describe();throw error;}finally{await browser.close();}
 console.log(JSON.stringify({passed:true,checks:checks.length}));
})().catch(error=>{console.error(error);console.error(JSON.stringify(errors));process.exit(1);});
