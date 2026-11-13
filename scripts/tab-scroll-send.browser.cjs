'use strict';
// Owner requests (v1.20.13): Terminal and Text Editor tab strips scroll with the mouse wheel and
// show left/right scroll buttons when tabs overflow; the Text Editor sends its selection or whole
// document to a chosen Terminal session through the normal paste rules.
// Actual bundled application and xterm in Chromium; every API response and SSH session is synthetic.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const {handleTextTools}=require('./editor-text-tools-fixture.cjs');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT,origin='https://tabscroll.toolkit.test';
const report={passed:false,checks:[],errors:[],scope:'Synthetic sessions and API in Chromium; no live device or native Windows claim.'};
const sessions=Array.from({length:14},(_,i)=>({id:'s'+(i+1),label:'lab-switch-'+String(i+1).padStart(2,'0'),type:'ssh',host:'192.0.2.'+(i+10),port:22,username:'operator',state:'connected',cols:100,rows:28}));
(async()=>{if(output)fs.mkdirSync(output,{recursive:true});const browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 try{
  const context=await browser.newContext({viewport:{width:1280,height:800}});await context.grantPermissions(['clipboard-read','clipboard-write'],{origin});const page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));
  let saved={activePage:'terminal',appUpdatePreferences:{enabled:false},terminalInteraction:{pasteReview:true,copyOnSelect:false,rightClick:'menu'},terminalSidebarCollapsed:true};const inputs=[];
  await page.route(origin+'/**',async route=>{if(await handleEditorActivityMetadata(route,origin,report))return;if(await handleTextTools(route))return;const req=route.request(),p=new URL(req.url()).pathname,method=req.method();
   if(p.startsWith('/api/')){let data={};
    if(p==='/api/status')data={version,os:'windows',arch:'amd64'};
    else if(p==='/api/state'){if(method==='PUT')saved=req.postDataJSON();else data=saved;}
    else if(p==='/api/jobs')data=[];
    else if(p==='/api/terminals')data={sessions};
    else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
    else if(p==='/api/terminals/events'){await new Promise(r=>setTimeout(r,80));data={sessions,events:[],after:0};}
    else if(p==='/api/activity')data={tools:{}};
    else if(/\/api\/terminals\/[^/]+\/input$/.test(p)){inputs.push({id:p.split('/')[3],body:req.postDataJSON()});data={accepted:true};}
    else if(/\/api\/terminals\/[^/]+\/resize$/.test(p))data={};
    else if(method!=='GET')return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture: not available'})});
    return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});}
   let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);if(!fs.existsSync(file))return route.fulfill({status:404,body:''});let bytes=fs.readFileSync(file);if(p==='/')bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','tabscroll').replaceAll('__WINDOW_MODE__','main').replaceAll('__TERMINAL_VIEW__','null').replaceAll('__EDITOR_VIEW__','null');
   return route.fulfill({body:bytes,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});});
  await page.goto(origin);await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('terminal'));
  await page.waitForFunction(n=>terminalSessions.size===n&&document.querySelectorAll('#terminalTabs [data-terminal-tab]').length===n,sessions.length);

  // ---- Terminal strip ----
  const strip='#terminalTabs',controls=strip+'>[data-tab-scroll-controls]';
  const scroll=sel=>page.evaluate(s=>document.querySelector(s).scrollLeft,sel);
  await page.waitForFunction(s=>{const c=document.querySelector(s);return c&&!c.hidden;},controls);
  assert.equal(await page.evaluate(s=>document.querySelector(s).parentElement.lastElementChild===document.querySelector(s),controls),true,'controls are the last item in the strip');
  await page.evaluate(s=>{document.querySelector(s).scrollLeft=0;},strip);await page.waitForTimeout(100);
  assert.equal(await page.locator(controls+' [data-tab-scroll="left"]').isDisabled(),true,'left disabled at the start');
  assert.equal(await page.locator(controls+' [data-tab-scroll="right"]').isDisabled(),false);
  {const box=await page.locator(controls).boundingBox(),area=await page.locator(strip).boundingBox();assert(Math.abs(box.x+box.width-(area.x+area.width))<=12,'controls pinned to the right edge: '+JSON.stringify({box,area}));}
  report.checks.push('Terminal: scroll buttons appear at the right end of an overflowing strip, left disabled at the start');
  const tabBox=await page.locator('#terminalTabs [data-terminal-tab="s2"]').boundingBox();await page.mouse.move(tabBox.x+20,tabBox.y+tabBox.height/2);await page.mouse.wheel(0,240);
  await page.waitForFunction(s=>document.querySelector(s).scrollLeft>100,strip);const afterWheel=await scroll(strip);
  await page.mouse.wheel(0,-240);await page.waitForFunction(([s,v])=>document.querySelector(s).scrollLeft<v,[strip,afterWheel]);
  report.checks.push('Terminal: the mouse wheel over the tab row scrolls it right and back left');
  await page.evaluate(s=>{document.querySelector(s).scrollLeft=0;},strip);await page.waitForTimeout(80);
  const activeBefore=await page.evaluate(()=>terminalActive);
  await page.locator(controls+' [data-tab-scroll="right"]').click();await page.waitForFunction(s=>document.querySelector(s).scrollLeft>150,strip);
  assert.equal(await page.evaluate(()=>terminalActive),activeBefore,'a scroll button does not switch tabs');
  assert.equal(await page.locator(controls+' [data-tab-scroll="left"]').isDisabled(),false,'left enabled once scrolled');
  await page.evaluate(s=>{const e=document.querySelector(s);e.scrollLeft=e.scrollWidth;},strip);await page.waitForTimeout(150);
  assert.equal(await page.locator(controls+' [data-tab-scroll="right"]').isDisabled(),true,'right disabled at the end');
  await page.locator(controls+' [data-tab-scroll="left"]').click();await page.waitForTimeout(500);assert(await scroll(strip)<await page.evaluate(s=>{const e=document.querySelector(s);return e.scrollWidth-e.clientWidth;},strip)-5,'left button scrolls back');
  report.checks.push('Terminal: scroll buttons scroll by a page, keep the active tab, and disable at each end');
  if(output){await page.evaluate(s=>{document.querySelector(s).scrollLeft=200;},strip);await page.waitForTimeout(150);const b=await page.locator(strip).boundingBox();await page.screenshot({path:path.join(output,'terminal-tab-scroll.png'),clip:{x:b.x,y:Math.max(0,b.y-10),width:b.width,height:b.height+20}});}
  // Few tabs: no controls.
  const wide=await page.evaluate(s=>{const e=document.querySelector(s);return e.scrollWidth>e.clientWidth;},strip);assert(wide);

  // ---- Text Editor strip ----
  await page.evaluate(()=>navigate('editor'));await page.locator('#networkToolkitFileEditor').waitFor();
  for(let i=0;i<10;i++){await page.locator('#networkToolkitFileEditor [data-ed="new"]').first().click();}
  const edStrip='#networkToolkitFileEditor [data-ed-tabs]:not([hidden])';
  await page.waitForFunction(s=>{const e=document.querySelector(s);return e&&e.querySelectorAll('.nt-editor-tab').length>=10;},edStrip);
  await page.waitForFunction(s=>{const c=document.querySelector(s)?.querySelector(':scope>[data-tab-scroll-controls]');return c&&!c.hidden;},edStrip);
  {const sel=await page.evaluate(s=>{const e=document.querySelector(s),tab=e.querySelector('[aria-selected="true"]').closest('.nt-editor-tab').getBoundingClientRect(),c=e.querySelector(':scope>[data-tab-scroll-controls]').getBoundingClientRect();return {tabRight:tab.right,controlsLeft:c.left};},edStrip);
   assert(sel.tabRight<=sel.controlsLeft+1,'the selected document tab is not hidden under the scroll buttons: '+JSON.stringify(sel));}
  const edBox=await page.locator(edStrip).boundingBox();await page.evaluate(s=>{document.querySelector(s).scrollLeft=0;},edStrip);await page.mouse.move(edBox.x+60,edBox.y+edBox.height/2);await page.mouse.wheel(0,200);
  await page.waitForFunction(s=>document.querySelector(s).scrollLeft>80,edStrip);
  report.checks.push('Text Editor: overflowing document tabs show the scroll buttons, the wheel scrolls, and the selected tab stays clear of the buttons');
  if(output)await page.screenshot({path:path.join(output,'editor-tab-scroll.png'),clip:{x:0,y:0,width:1280,height:260}});

  // ---- Send to terminal ----
  const openSend=async()=>{await page.locator('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="sendTerminal"]').click();await page.locator('#editorSendTerminalForm').waitFor();};
  assert.equal(await page.locator('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="sendTerminal"]').isVisible(),true,'Send to terminal is always visible on the toolbar');assert.equal(await page.locator('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="sendTerminal"] svg').count(),1,'it is an icon button');report.checks.push('Send to terminal is an always-visible toolbar icon button');
  const doc='show version\nshow ip interface brief\n';
  await page.locator('#networkToolkitFileEditor [data-ed-text]').fill(doc);
  await page.evaluate(()=>{const t=document.querySelector('#networkToolkitFileEditor [data-ed-text]');t.focus();t.setSelectionRange(0,12);});
  await openSend();
  assert.equal(await page.locator('[name="editorSendWhat"][value="selection"]').isChecked(),true,'selection is chosen when text is selected');
  assert.equal(await page.locator('#editorSendTarget option').count(),sessions.length,'every connected session is offered');
  if(output)await page.screenshot({path:path.join(output,'editor-send-dialog.png')});
  await page.locator('#editorSendTarget').selectOption('s9');assert.deepEqual(inputs,[],'nothing is sent before Send');
  await page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  await page.waitForFunction(()=>state.activePage==='terminal'&&terminalActive==='s9');
  for(let i=0;i<100&&!inputs.length;i++)await page.waitForTimeout(30);
  assert.equal(inputs.length>=1,true,'single-line selection is entered without review');assert.equal(inputs[0].id,'s9');
  const sent=inputs.map(x=>Buffer.from(String(x.body.data||''),'base64').toString('utf8')).join('');assert.match(sent,/show version/);assert.doesNotMatch(sent,/interface/,'only the selection is sent');assert.doesNotMatch(sent,/[\r\n]/,'no line break is added');
  report.checks.push('Send selection: switches to the chosen terminal session and enters only the selected text, with nothing added');
  // Whole document with line breaks goes through the multi-line paste review.
  await page.evaluate(()=>navigate('editor'));await page.locator('#networkToolkitFileEditor [data-ed-text]').waitFor();
  await page.evaluate(()=>{const t=document.querySelector('#networkToolkitFileEditor [data-ed-text]');t.focus();t.setSelectionRange(0,0);});
  await page.keyboard.press('Control+Shift+Enter');await page.locator('#editorSendTerminalForm').waitFor();
  assert.equal(await page.locator('[name="editorSendWhat"][value="document"]').isChecked(),true,'whole document when nothing is selected');
  assert.equal(await page.locator('#editorSendTarget').inputValue(),'s9','the last session is remembered');
  const before=inputs.length;await page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  await page.locator('#terminalPasteForm').waitFor();assert.equal(await page.evaluate(()=>terminalActive),'s9');assert.equal(inputs.length,before,'multi-line text waits for paste review');
  assert.equal(await page.locator('#terminalPasteText').inputValue(),doc,'the review shows the whole document');
  report.checks.push('Send document (Ctrl+Shift+Enter): multi-line text opens the Terminal paste review (configured rule) before anything is sent');
  if(output)await page.screenshot({path:path.join(output,'editor-send-review.png')});
  await page.locator('#terminalPasteDismiss').click();
  // Editor documents (unsaved) are still open.
  await page.evaluate(()=>navigate('editor'));await page.waitForFunction(d=>document.querySelector('#networkToolkitFileEditor [data-ed-text]')?.value===d,doc);
  report.checks.push('Editor documents and unsaved text stay open after sending');
  // Changed document after opening the dialog: asks to reopen.
  await openSend();
  await page.evaluate(()=>{const t=document.querySelector('#networkToolkitFileEditor [data-ed-text]');t.value+='x';t.dispatchEvent(new Event('input',{bubbles:true}));});
  await page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();await page.waitForFunction(()=>/changed after this dialog opened/.test(document.querySelector('#editorSendError')?.textContent||''));
  assert.equal(await page.evaluate(()=>state.activePage),'editor');report.checks.push('A document changed after opening the dialog is not sent');
  // ---- Terminal output to the Text Editor ----
  await page.locator('#modalRoot [data-action="close-modal"]').first().click().catch(()=>{});
  await page.evaluate(()=>{navigate('terminal');selectTerminal('s9',true);});await page.waitForFunction(()=>currentTerminal()?.info.id==='s9'&&terminalEntryAcceptsInput(currentTerminal()));
  await page.evaluate(()=>{window.receive=async text=>{const e=currentTerminal(),api=NetworkToolkitTerminalExtras,position=api.beforeOutput(e);api.observeOutput(e,text);await new Promise(r=>e.xterm.write(text,r));api.afterOutput(e,position);await api.flush(e);};currentTerminal().xterm.reset();});
  const receive=text=>page.evaluate(t=>receive(t),text);
  await receive('lab-switch-09# ');await page.evaluate(()=>currentTerminal().xterm.focus());await page.keyboard.type('show clock');await receive('show clock');await page.keyboard.press('Enter');await receive('\r\n10:00:00.000 UTC Thu Nov 5 2026\r\nlab-switch-09# ');
  await page.waitForFunction(()=>/10:00:00/.test(NetworkToolkitTerminalExtras.output(currentTerminal(),NetworkToolkitTerminalExtras.current(currentTerminal()))?.text||''));
  // The same text Copy output gives (Include commands as set).
  const copied=await page.evaluate(()=>{const e=currentTerminal(),x=NetworkToolkitTerminalExtras;return x.output(e,x.current(e),x.prefs(e).pasteIncludeCommands===true)?.text;});assert.match(copied,/10:00:00/);
  const sendButton=page.locator('[data-terminal-extra="send-editor"]');
  const openToEditor=async()=>{if(await sendButton.isVisible())await sendButton.click();else await page.evaluate(()=>NetworkToolkitTerminalSendEditor.open(currentTerminal()));await page.locator('#terminalSendEditorForm').waitFor();};
  await openToEditor();
  assert.equal(await page.locator('[name="terminalSendSource"][value="output"]').isChecked(),true,'last command output is the default source');
  assert.equal(await page.locator('[name="terminalSendTarget"][value="existing"]').isChecked(),true,'an open document is offered');
  assert.equal(await page.locator('#terminalSendWhere').inputValue(),'pick','choosing the position is the default');
  if(output)await page.screenshot({path:path.join(output,'terminal-to-editor-dialog.png')});
  await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
  await page.waitForFunction(()=>state.activePage==='editor'&&!!document.querySelector('[data-ed-insert-bar]'));
  const editorText=()=>page.evaluate(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value);
  const beforeInsert=await editorText();assert.equal(beforeInsert.includes('10:00:00'),false,'nothing is inserted until the position is confirmed');
  await page.evaluate(()=>{const t=document.querySelector('#networkToolkitFileEditor [data-ed-text]');t.focus();const at=t.value.indexOf('show ip');t.setSelectionRange(at,at);});
  if(output)await page.screenshot({path:path.join(output,'terminal-to-editor-pick.png')});
  await page.locator('[data-ed-insert-here]').click();await page.waitForFunction(()=>!document.querySelector('[data-ed-insert-bar]'));
  const afterInsert=await editorText(),at=beforeInsert.indexOf('show ip');
  assert.equal(afterInsert.slice(0,at),beforeInsert.slice(0,at),'text before the cursor is unchanged');assert.equal(afterInsert.slice(at,at+copied.length),copied,'output inserted at the chosen cursor: '+JSON.stringify(afterInsert));
  assert(afterInsert.endsWith(beforeInsert.slice(at)),'text after the cursor is kept');
  report.checks.push('Terminal output to an open document: the editor asks for the position, and Insert here puts it at the placed cursor');
  // Cancel adds nothing.
  await page.evaluate(()=>navigate('terminal'));await openToEditor();await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();await page.locator('[data-ed-insert-cancel]').waitFor();
  const kept=await editorText();await page.locator('[data-ed-insert-cancel]').click();assert.equal(await editorText(),kept);report.checks.push('Cancel at the position step adds nothing');
  // Line number.
  await page.evaluate(()=>navigate('terminal'));await openToEditor();await page.locator('#terminalSendWhere').selectOption('line');await page.locator('#terminalSendLine').fill('2');
  await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();await page.waitForFunction(()=>state.activePage==='editor');
  await page.waitForFunction(c=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value.split('\n')[1]?.startsWith(c.split('\n')[0]),copied);report.checks.push('Insert at a line number');
  // New document with the selected terminal text.
  await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(()=>currentTerminal()?.info.id==='s9');
  await page.evaluate(()=>{const x=currentTerminal().xterm,b=x.buffer.active;for(let y=0;y<b.length;y++){const line=b.getLine(y)?.translateToString(true)||'';if(line.startsWith('10:00:00')){x.select(0,y,8);break;}}});
  await openToEditor();assert.equal(await page.locator('[name="terminalSendSource"][value="selection"]').isDisabled(),false);await page.locator('[name="terminalSendSource"][value="selection"]').check();await page.locator('[name="terminalSendTarget"][value="new"]').check();
  const tabsBefore=await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length);
  await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
  await page.waitForFunction(n=>NetworkToolkitFileEditor.listDocuments().length===n+1,tabsBefore);await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value==='10:00:00');
  assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments().find(d=>d.active)?.dirty),true,'the new document is unsaved, so closing asks first');
  report.checks.push('Selected terminal text to a new document (marked unsaved)');
  if(output)await page.screenshot({path:path.join(output,'terminal-to-editor-new.png')});
  // ---- Text Editor right-click menu ----
  await page.evaluate(()=>{const t=document.querySelector('#networkToolkitFileEditor [data-ed-text]');t.value='alpha beta\ngamma';t.dispatchEvent(new Event('input',{bubbles:true}));t.focus();t.setSelectionRange(0,5);});
  const text=page.locator('#networkToolkitFileEditor [data-ed-text]');
  const menu=page.locator('#editorContextMenu');
  const submenu=name=>menu.locator(`[data-editor-submenu="${name}"]`);
  const submenuPanel=name=>menu.locator(`[data-editor-submenu-panel="${name}"]`);
  await page.evaluate(()=>{const t=document.querySelector('#networkToolkitFileEditor [data-ed-text]');t.focus();t.setSelectionRange(0,5);});
  {const b=await text.boundingBox();await page.mouse.click(b.x+28,b.y+12,{button:'right'});}await menu.waitFor();
  assert.equal(await page.evaluate(()=>{const t=document.querySelector('#networkToolkitFileEditor [data-ed-text]');return [t.selectionStart,t.selectionEnd].join();}),'0,5','right-clicking inside the selection keeps it');
  {const r=await menu.boundingBox();assert(r.y>=0&&r.y+r.height<=800&&r.x>=0&&r.x+r.width<=1280,'menu inside the window: '+JSON.stringify(r));}
  for(const label of ['Undo','Redo','Cut','Copy','Paste','Select all','Send selection to terminal…','Find','Replace','Go to line']) {
    assert.equal(await menu.getByRole('menuitem',{name:new RegExp('^'+label+'(?:\\s|$)')}).count(),1,label);
  }
  assert.deepEqual(await menu.locator('.nt-editor-menu-root [data-editor-submenu]').evaluateAll(list=>list.map(el=>el.dataset.editorSubmenu)),['tools-lines','tools-encode','tools-format','tools-device']);
  await submenuPanel('tools-lines').waitFor({state:'hidden'});
  await submenuPanel('tools-encode').waitFor({state:'hidden'});
  await submenu('tools-lines').hover();
  await submenuPanel('tools-lines').waitFor();
  assert.equal(await submenu('tools-lines').getAttribute('aria-expanded'),'true');
  for(const action of ['comment','duplicate','sort-asc','trim-trailing']) {
    assert.equal(await submenuPanel('tools-lines').locator(`[data-editor-menu-action="${action}"]`).isVisible(),true,action);
  }
  await submenu('tools-format').hover();
  await submenuPanel('tools-format').waitFor();
  for(const action of ['format','upper','lower','json-pretty']) {
    assert.equal(await submenuPanel('tools-format').locator(`[data-editor-menu-action="${action}"]`).isVisible(),true,action);
  }
  if(output)await page.screenshot({path:path.join(output,'editor-context-menu.png')});
  await page.locator('#editorContextMenu [data-editor-menu-action="cut"]').click();await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value===' beta\ngamma');
  assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),'alpha','Cut puts the selection on the clipboard');
  await page.evaluate(()=>{const t=document.querySelector('#networkToolkitFileEditor [data-ed-text]');t.focus();t.setSelectionRange(t.value.length,t.value.length);});
  await page.locator('#networkToolkitFileEditor [data-ed-text]').dispatchEvent('contextmenu',{clientX:400,clientY:300,button:2});await page.locator('#editorContextMenu [data-editor-menu-action="paste"]').click();
  await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value===' beta\ngammaalpha');
  await page.locator('#networkToolkitFileEditor [data-ed-text]').dispatchEvent('contextmenu',{clientX:400,clientY:300,button:2});await page.locator('#editorContextMenu [data-editor-menu-action="undo"]').click();
  await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value===' beta\ngamma');
  report.checks.push('Text Editor right-click menu groups text tools and keeps Cut, Paste and Undo directly available');
  await text.evaluate(el=>{el.focus();el.setSelectionRange(1,5);});
  await text.evaluate(el=>el.dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true,clientX:innerWidth-4,clientY:innerHeight-4,button:2})));
  await submenu('tools-encode').hover();
  await submenuPanel('tools-encode').waitFor();
  const submenuBounds=await submenuPanel('tools-encode').boundingBox(), menuBounds=await menu.boundingBox();
  assert(submenuBounds.x>=0&&submenuBounds.y>=0&&submenuBounds.x+submenuBounds.width<=1280&&submenuBounds.y+submenuBounds.height<=800,'submenu fits at the lower-right edge');
  assert(submenuBounds.x+submenuBounds.width/2<menuBounds.x,'submenu opens on the left where there is room: '+JSON.stringify({submenuBounds,menuBounds}));
  const triggerBounds=await submenu('tools-encode').boundingBox();
  await page.mouse.move(submenuBounds.x+submenuBounds.width-12,triggerBounds.y+triggerBounds.height/2,{steps:6});
  assert.equal(await submenuPanel('tools-encode').isVisible(),true,'moving across to the submenu keeps it open');
  await submenuPanel('tools-encode').locator('[data-editor-menu-action="base64-encode"]').click();
  await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value===' YmV0YQ==\ngamma');
  await text.press('Shift+F10');
  await submenu('tools-encode').focus();
  await page.keyboard.press('ArrowRight');
  await submenuPanel('tools-encode').waitFor();
  assert.equal(await submenuPanel('tools-encode').locator(':focus').count(),1,'opening by keyboard focuses a conversion');
  await page.keyboard.press('ArrowLeft');
  await submenuPanel('tools-encode').waitFor({state:'hidden'});
  assert.equal(await submenu('tools-encode').evaluate(el=>el===document.activeElement),true);
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('Escape');
  await submenuPanel('tools-encode').waitFor({state:'hidden'});
  assert.equal(await submenu('tools-encode').evaluate(el=>el===document.activeElement),true);
  await page.keyboard.press('ArrowRight');
  await page.keyboard.press('Home');
  assert.equal(await page.evaluate(()=>document.activeElement?.dataset.editorMenuAction),'base64-decode');
  await page.keyboard.press('Enter');
  await menu.waitFor({state:'hidden'});
  await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value===' beta\ngamma');
  // The toolbar's Tools button opens the same groups.
  const toolsButton=page.locator('#networkToolkitFileEditor [data-ed="tools"]');
  await toolsButton.click();await menu.waitFor();
  await submenu('tools-encode').hover();await submenuPanel('tools-encode').waitFor();
  await submenuPanel('tools-encode').locator('[data-editor-menu-action="hex-encode"]').click();
  await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value===' 62657461\ngamma');
  await toolsButton.click();await menu.waitFor();
  await submenu('tools-encode').hover();await submenuPanel('tools-encode').waitFor();
  await submenuPanel('tools-encode').locator('[data-editor-menu-action="hex-decode"]').click();
  await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value===' beta\ngamma');
  report.checks.push('Encode / decode submenu fits at the window edge, supports keyboard navigation, and replaces only the selection; the Tools button opens the same tools');
  await page.keyboard.press('Escape');
  await page.evaluate(()=>{const t=document.querySelector('#networkToolkitFileEditor [data-ed-text]');t.focus();t.setSelectionRange(1,5);});await page.keyboard.press('Shift+F10');await page.locator('#editorContextMenu').waitFor();
  assert.equal(await page.evaluate(()=>document.activeElement?.closest('#editorContextMenu')!==null),true,'keyboard opens the menu with focus inside');
  await page.locator('#editorContextMenu [data-editor-menu-action="sendTerminal"]').click();await page.locator('#editorSendTerminalForm').waitFor();
  assert.equal(await page.locator('[name="editorSendWhat"][value="selection"]').isChecked(),true);await page.locator('#modalRoot [data-action="close-modal"]').first().click();
  report.checks.push('Shift+F10 opens the menu; Send selection to terminal opens the send dialog');
  // ---- Terminal right-click menu ----
  await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(()=>currentTerminal()?.info.id==='s9'&&terminalEntryAcceptsInput(currentTerminal()));
  {const b=await page.locator('.terminal-pane:not(.hidden) .xterm-screen').first().boundingBox();await page.mouse.click(b.x+80,b.y+40,{button:'right'});}
  await page.locator('#terminalContextMenu [data-terminal-menu-action="send-editor"]').waitFor();
  if(output)await page.screenshot({path:path.join(output,'terminal-context-menu.png')});
  await page.locator('#terminalContextMenu [data-terminal-menu-action="send-editor"]').click();await page.locator('#terminalSendEditorForm').waitFor();await page.locator('#modalRoot [data-action="close-modal"]').first().click();
  assert.deepEqual(await page.evaluate(()=>NetworkToolkitTerminalInteraction.safePreferences({menuItems:['copy']}).menuItems),['copy','send-editor'],'a menu customized before this version gains the item once');
  assert.deepEqual(await page.evaluate(()=>NetworkToolkitTerminalInteraction.safePreferences({menuItems:['copy'],menuRevision:2}).menuItems),['copy'],'unticking it later is kept');
  report.checks.push('Terminal right-click menu has Send to Text Editor (added once to customized menus)');
  report.errors=report.errors.filter(e=>!/Synthetic fixture/.test(e));assert.deepEqual(report.errors,[]);report.passed=true;
 }finally{if(output)fs.writeFileSync(path.join(output,'tab-scroll-send-report.json'),JSON.stringify(report,null,2));await browser.close();}
 console.log(report.checks.join('\n'));
})().catch(error=>{console.error(error);console.error(JSON.stringify(report.errors));process.exit(1);});
