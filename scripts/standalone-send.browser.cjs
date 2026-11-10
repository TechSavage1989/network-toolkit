'use strict';
// Owner request: in the installed edition the standalone Text Editor sends to the standalone
// Terminal, and the standalone Terminal sends to the standalone Text Editor, with their existing
// "Send to terminal" and "To editor" commands (web/assets/peer-send.js).
// The bundled application and xterm run in Chromium. Each page is ONE standalone app; the other
// app, every API response, every session and every document is synthetic. This proves the page
// behaviour: what each dialog lists, what is sent to the app's own engine, how results and "not
// running" are shown, and that received text takes the same paste and insert paths as a send
// inside one window. The two real engines are covered by the Go test
// TestPeerSendBetweenTwoStandaloneAppsOfOneDataRoot. No native Windows claim.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT,origin='https://standalone-send.toolkit.test';
const report={passed:false,checks:[],errors:[],screenshots:[],scope:'Synthetic production-asset browser acceptance in Chromium with an intercepted API. Not native Windows WebView2 acceptance and not a test of two real processes.'};
const pass=text=>{report.checks.push(text);console.log('PASS '+text);};
const TERMINAL='Network Toolkit Terminal',EDITOR='Network Toolkit Text Editor';
const sessions=[{id:'s1',label:'lab-switch-01',type:'ssh',host:'192.0.2.10',port:22,username:'operator',state:'connected',cols:100,rows:28},{id:'s2',label:'lab-router-02',type:'telnet',host:'192.0.2.11',port:23,state:'connected',cols:100,rows:28},{id:'s3',label:'lab-closed-03',type:'ssh',host:'192.0.2.12',port:22,username:'operator',state:'closed',cols:100,rows:28}];
let browser,page,fixture;
async function shot(name){if(!output)return;const file='standalone-send-'+name+'.png';await page.screenshot({path:path.join(output,file)});report.screenshots.push(file);}
// Wait until a predicate over the fixture holds (the pages poll once a second).
async function until(check,what,ms=8000){const end=Date.now()+ms;for(;;){const value=check();if(value)return value;if(Date.now()>end)throw new Error('Timed out waiting for '+what);await page.waitForTimeout(40);}}
const toasts=()=>page.evaluate(()=>[...document.querySelectorAll('#toastStack .toast')].map(t=>t.textContent.trim()));
const toast=text=>page.waitForFunction(t=>[...document.querySelectorAll('#toastStack .toast')].some(n=>n.textContent.includes(t)),text);
const clearToasts=()=>page.evaluate(()=>{document.querySelector('#toastStack').innerHTML='';});

async function open(mode){
 fixture={mode,saved:{activePage:mode==='terminal'?'terminal':'editor',appUpdatePreferences:{enabled:false},theme:'light',terminalInteraction:{pasteReview:true,copyOnSelect:false,rightClick:'menu'},terminalSidebarCollapsed:true},
  targets:{app:mode==='terminal'?'editor':'terminal',name:mode==='terminal'?EDITOR:TERMINAL,running:false,ready:false,targets:[]},peerCalls:[],polls:0,reports:[],inbox:[],acks:[],sends:[],opens:0,results:[],sendError:null,inputs:[],unexpected:[]};
 const context=await browser.newContext({viewport:{width:1280,height:800},serviceWorkers:'block'});page=await context.newPage();page.setDefaultTimeout(15000);
 page.on('pageerror',error=>report.errors.push(mode+': '+error.message));
 const f=fixture;
 await page.route('**/*',async route=>{if(await handleEditorActivityMetadata(route,origin,report))return;
  const req=route.request(),url=new URL(req.url()),p=url.pathname,method=req.method();assert.equal(url.origin,origin,'No external requests are allowed');
  if(p.startsWith('/api/')){let data={},status=200;const body=()=>{try{return req.postDataJSON();}catch{return null;}};
   if(p.startsWith('/api/peer-send/')){f.peerCalls.push(method+' '+p);
    assert.equal(req.headers()['x-session-token'],'standalone-send','peer-send calls carry this page\'s own session token');
    if(p==='/api/peer-send/targets'&&method==='GET')data=f.targets;
    else if(p==='/api/peer-send/open'&&method==='POST'){f.opens++;f.targets={...f.targets,running:true};data={app:f.targets.app,name:f.targets.name,running:true,ready:f.targets.ready};}
    else if(p==='/api/peer-send/send'&&method==='POST'){f.sends.push(body());if(f.sendError){status=f.sendError.status;data={error:f.sendError.error};}else data={requestId:String(f.sends.length).padStart(32,'0'),state:'queued',name:f.targets.name};}
    else if(p==='/api/peer-send/result'&&method==='GET'){assert.match(url.searchParams.get('requestId')||'',/^[0-9]{32}$/);data={requestId:url.searchParams.get('requestId'),...(f.results.length>1?f.results.shift():f.results[0]||{state:'queued'})};}
    else if(p==='/api/peer-send/inbox'&&method==='GET'){f.polls++;data={request:f.inbox.shift()||null,report:!f.reports.length,from:f.targets.name};}
    else if(p==='/api/peer-send/report'&&method==='POST'){f.reports.push(body());data={ok:true};}
    else if(p==='/api/peer-send/ack'&&method==='POST'){f.acks.push(body());data={ok:true};}
    else{f.unexpected.push(method+' '+p);status=404;data={error:'unexpected peer-send request'};}
    return route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});}
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',windowMode:mode,storageWritable:true,localShellAvailable:true,terminalWindowAvailable:mode==='terminal',editorLaunchAvailable:mode==='editor-standalone',editorLaunchReusable:mode==='editor-standalone',system:{hostname:'Standalone fixture',interfaces:[]}};
   else if(p==='/api/state'){if(method==='PUT')f.saved=body();else data=f.saved;}
   else if(p==='/api/system')data={hostname:'Standalone fixture',interfaces:[]};
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/terminals')data={sessions:mode==='editor-standalone'?[]:sessions};
   else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   else if(p==='/api/terminals/events'){await new Promise(r=>setTimeout(r,80));data={sessions:mode==='editor-standalone'?[]:sessions,events:[],after:0};}
   else if(p==='/api/terminal-window/requests')data={requests:[],latest:0,oldest:0};
   else if(p==='/api/editor-launch')data={requests:[]};
   else if(p==='/api/activity')data={tools:{}};
   else if(p==='/api/remote-browser'&&method==='GET')data={sessions:[]};
   else if(p==='/api/remote-browser')data={supported:false};
   else if(p==='/api/credential-vault')data={available:false,state:'unavailable'};
   else if(p==='/api/credentials')data={vault:{available:false,state:'unavailable'},entries:[]};
   else if(/\/api\/terminals\/[^/]+\/input$/.test(p)){f.inputs.push({id:p.split('/')[3],text:Buffer.from(String(body()?.data||''),'base64').toString('utf8')});data={accepted:true};}
   else if(/\/api\/terminals\/[^/]+\/resize$/.test(p))data={};
   else if(method!=='GET')return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture: not available'})});
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});}
  let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);if(!fs.existsSync(file))return route.fulfill({status:404,body:''});let bytes=fs.readFileSync(file);
  if(p==='/')bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','standalone-send').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__','null').replaceAll('__EDITOR_VIEW__','null');
  return route.fulfill({body:bytes,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});});
 await page.goto(origin+'/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);
 return context;
}

async function standaloneEditor(){
 const context=await open('editor-standalone'),f=fixture;
 const editor='#networkToolkitFileEditor',send=page.locator(editor+' .nt-editor-toolbar [data-ed="sendTerminal"]'),text=page.locator(editor+' [data-ed-text]');
 await send.waitFor();assert.equal(await page.evaluate(()=>NetworkToolkitPeerSend.sender()),'editor');assert.equal(await page.evaluate(()=>NetworkToolkitEditorSendTerminal.available()),true,'Send to terminal is available in the standalone Text Editor');
 await page.locator(editor+' [data-ed="new"]').first().click();
 const doc='show version\nshow ip interface brief\n';await text.fill(doc);
 // The page tells its own engine which documents can take text: names only.
 const reported=await until(()=>f.reports.find(r=>r.targets.length===1),'the document summary');
 assert.deepEqual(Object.keys(reported).sort(),['targets']);assert.deepEqual(Object.keys(reported.targets[0]).sort(),['active','busy','id','name','unsaved']);
 assert.match(reported.targets[0].id,/^[0-9a-f-]{36}$/);assert.equal(reported.targets[0].name,'Untitled.txt');assert.equal(reported.targets[0].unsaved,true);assert.equal(JSON.stringify(reported).includes('show version'),false,'document text is never reported');
 // Nothing changed: the page keeps polling but does not report again.
 {const reports=f.reports.length,polls=f.polls;await until(()=>f.polls>=polls+2,'two more polls');assert.equal(f.reports.length,reports,'an unchanged summary is not sent again');}
 pass('Text Editor: reports open document names (no text, no folders) to its own engine when asked and when they change');

 // ---- The Terminal is not open ----
 await send.click();await page.locator('[data-editor-send-peer-missing]').waitFor();
 assert.match(await page.locator('#modalRoot .modal-body').innerText(),/Network Toolkit Terminal is not open\./);assert.equal(await page.locator('#editorSendTerminalForm').count(),0);
 await shot('editor-terminal-not-open');
 await page.locator('[data-editor-send-open-peer]').click();await toast(TERMINAL+' is open.');
 assert.equal(f.opens,1,'Open Terminal asks this app\'s engine to start the Terminal app');await page.locator('#modalRoot .modal').waitFor({state:'detached'});assert.deepEqual(f.sends,[],'nothing is sent by opening the Terminal');
 pass('Text Editor: when Network Toolkit Terminal is not open the dialog says so and Open Terminal starts it');
 // ---- Open, but nothing connected yet ----
 f.targets={...f.targets,running:true,ready:true,targets:[]};await clearToasts();
 await send.click();await page.locator('[data-editor-send-peer-missing]').waitFor();assert.match(await page.locator('#modalRoot .modal-body').innerText(),/No terminal session is connected in Network Toolkit Terminal\./);
 await page.locator('#modalRoot [data-action="close-modal"]').first().click();
 pass('Text Editor: with no connected session in the Terminal app nothing can be sent');

 // ---- Sessions of the Terminal app ----
 f.targets={...f.targets,targets:[{id:'s1',name:'lab-switch-01',type:'ssh',connected:true},{id:'s2',name:'lab-router-02',type:'telnet',connected:true,active:true}]};
 await text.evaluate(el=>{el.focus();el.setSelectionRange(0,12);});
 await send.click();await page.locator('#editorSendTerminalForm').waitFor();
 assert.equal(await page.locator('#editorSendTarget optgroup').getAttribute('label'),TERMINAL,'sessions are listed under the Terminal app');
 assert.deepEqual(await page.locator('#editorSendTarget option').allTextContents(),['lab-switch-01 · SSH','lab-router-02 · Telnet']);
 assert.equal(await page.locator('#editorSendTarget').inputValue(),'s2','the session selected in the Terminal app is preselected');
 assert.equal(await page.locator('[name="editorSendWhat"][value="selection"]').isChecked(),true);
 assert.match(await page.locator('#editorSendTerminalForm').innerText(),/Network Toolkit Terminal opens on this session and the text is entered as a paste, using its paste settings\./);
 await shot('editor-send-dialog');
 await page.locator('#editorSendTarget').selectOption('s1');assert.deepEqual(f.sends,[],'nothing is sent before Send');
 f.results=[{state:'queued'},{state:'taken'},{state:'done',outcome:'sent'}];
 await page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
 await toast('Sent to lab-switch-01 in Network Toolkit Terminal.');
 assert.deepEqual(f.sends,[{session:'s1',text:'show version'}],'only the selection, the chosen session and nothing else is sent');
 assert.equal(await text.inputValue(),doc,'the document stays open and unchanged');assert.equal(await page.evaluate(()=>state.activePage),'editor');
 pass('Text Editor: the dialog lists the sessions of Network Toolkit Terminal, sends the selection to the chosen one and shows the result');
 await shot('editor-sent');
 // Whole document by keyboard: the other app reviews multi-line text.
 await clearToasts();await text.evaluate(el=>{el.focus();el.setSelectionRange(0,0);});
 await page.keyboard.press('Control+Shift+Enter');await page.locator('#editorSendTerminalForm').waitFor();
 assert.equal(await page.locator('[name="editorSendWhat"][value="document"]').isChecked(),true);assert.equal(await page.locator('#editorSendTarget').inputValue(),'s1','the last session is remembered');
 f.results=[{state:'queued'},{state:'done',outcome:'review'}];
 await page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
 await toast('Network Toolkit Terminal is showing the text for review before it goes to lab-switch-01.');
 assert.deepEqual(f.sends[1],{session:'s1',text:doc});
 pass('Text Editor: Ctrl+Shift+Enter sends the whole document; a review in the Terminal app is reported');
 // The right-click menu opens the same dialog.
 await clearToasts();await text.evaluate(el=>{el.focus();el.setSelectionRange(0,4);});await page.keyboard.press('Shift+F10');
 await page.locator('#editorContextMenu [data-editor-menu-action="sendTerminal"]').click();await page.locator('#editorSendTerminalForm').waitFor();
 assert.equal(await page.locator('#editorSendTarget optgroup').getAttribute('label'),TERMINAL);
 // An error from the other app is shown as it is.
 f.results=[{state:'failed',message:'That terminal session is no longer connected. Nothing was sent.'}];
 await page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
 await toast('Network Toolkit Terminal: That terminal session is no longer connected. Nothing was sent.');
 pass('Text Editor: the right-click menu uses the same dialog, and an error from the Terminal app is shown');
 // The engine refuses (for example the Terminal closed in the meantime).
 await clearToasts();f.sendError={status:503,error:'Network Toolkit Terminal is not open. Nothing was sent.'};
 await send.click();await page.locator('#editorSendTerminalForm').waitFor();await page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
 await toast('Network Toolkit Terminal is not open. Nothing was sent.');f.sendError=null;
 // A document changed after the dialog opened is not sent.
 const sent=f.sends.length;await send.click();await page.locator('#editorSendTerminalForm').waitFor();
 await text.evaluate(el=>{el.value+='x';el.dispatchEvent(new Event('input',{bubbles:true}));});
 await page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();await page.waitForFunction(()=>/changed after this dialog opened/.test(document.querySelector('#editorSendError')?.textContent||''));
 assert.equal(f.sends.length,sent);await page.locator('#modalRoot [data-action="close-modal"]').first().click();
 pass('Text Editor: a refusal by the engine is shown, and a document changed after the dialog opened is not sent');

 // ---- Receiving text from the Terminal app ----
 await clearToasts();const before=await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length);
 f.inbox.push({requestId:'a'.repeat(32),target:'new',where:'start',text:'10:00:00.000 UTC Thu Nov 5 2026\n',label:'Last command output from lab-switch-01'});
 await until(()=>f.acks.length===1,'the first acknowledgement');assert.deepEqual(f.acks[0],{requestId:'a'.repeat(32),ok:true,outcome:'inserted'});
 await page.waitForFunction(n=>NetworkToolkitFileEditor.listDocuments().length===n+1,before);assert.equal(await text.inputValue(),'10:00:00.000 UTC Thu Nov 5 2026\n');
 assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments().find(d=>d.active)?.dirty),true,'the new document is unsaved, so closing asks first');
 pass('Text Editor: text from the Terminal app arrives in a new, unsaved document');
 const uid=await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments()[0].uid),first=await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments()[0].id);
 f.inbox.push({requestId:'b'.repeat(32),target:uid,where:'pick',text:'interface Vlan10\n',label:'Selected terminal text from lab-switch-01'});
 await until(()=>f.acks.length===2,'the second acknowledgement');assert.deepEqual(f.acks[1],{requestId:'b'.repeat(32),ok:true,outcome:'pick'});
 await page.locator('[data-ed-insert-bar]').waitFor();assert.match(await page.locator('[data-ed-insert-bar]').innerText(),/Selected terminal text from lab-switch-01/);
 assert.equal(await page.evaluate(id=>NetworkToolkitFileEditor.active?.id===id,first),true,'the chosen document is shown');
 const beforeInsert=await text.inputValue();assert.equal(beforeInsert.includes('Vlan10'),false,'nothing is inserted until the position is confirmed');
 await shot('editor-receive-pick');
 await text.evaluate(el=>{el.focus();el.setSelectionRange(0,0);});await page.locator('[data-ed-insert-here]').click();await page.waitForFunction(()=>!document.querySelector('[data-ed-insert-bar]'));
 assert.equal(await text.inputValue(),'interface Vlan10\n'+beforeInsert);
 pass('Text Editor: text for an open document asks for the position with the usual Insert here bar');
 f.inbox.push({requestId:'c'.repeat(32),target:uid,where:'line',line:2,text:'! from terminal\n',label:'Last command output'});
 await until(()=>f.acks.length===3,'the third acknowledgement');assert.deepEqual(f.acks[2],{requestId:'c'.repeat(32),ok:true,outcome:'inserted'});
 assert.equal((await text.inputValue()).split('\n')[1],'! from terminal');
 f.inbox.push({requestId:'d'.repeat(32),target:'00000000-0000-4000-8000-000000000000',where:'end',text:'x'});
 await until(()=>f.acks.length===4,'the fourth acknowledgement');assert.deepEqual(f.acks[3],{requestId:'d'.repeat(32),ok:false,error:'That document is no longer open. Nothing was added.'});
 // A dialog open in this app is never replaced by arriving text.
 await page.locator('.standalone-app-tools [data-action="settings"]').click();await page.locator('#modalTitle').filter({hasText:'Text Editor window settings'}).waitFor();
 const docs=await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length);
 f.inbox.push({requestId:'e'.repeat(32),target:'new',where:'start',text:'must wait'});
 await until(()=>f.acks.length===5,'the fifth acknowledgement');assert.deepEqual(f.acks[4],{requestId:'e'.repeat(32),ok:false,error:'A dialog is open in Network Toolkit Text Editor. Finish or close it there, then send again.'});
 assert.equal(await page.locator('#modalTitle').innerText(),'Text Editor window settings');assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length),docs);
 await page.keyboard.press('Escape');
 pass('Text Editor: insert at a line works, a closed document is refused, and an open dialog is never replaced');
 assert.deepEqual(f.unexpected,[]);
 await context.close();
}

async function standaloneTerminal(){
 const context=await open('terminal'),f=fixture;
 await page.waitForFunction(()=>terminalSessions.size===3&&document.querySelectorAll('#terminalTabs [data-terminal-tab]').length===3);
 assert.equal(await page.evaluate(()=>NetworkToolkitPeerSend.sender()),'terminal');assert.equal(await page.evaluate(()=>NetworkToolkitTerminalSendEditor.available()),true,'To editor is available in the standalone Terminal');
 await page.evaluate(()=>selectTerminal('s1',true));await page.waitForFunction(()=>currentTerminal()?.info.id==='s1'&&terminalEntryAcceptsInput(currentTerminal()));
 // The page tells its own engine which sessions can take input.
 const reported=await until(()=>f.reports.find(r=>r.targets.length===2),'the session summary');
 assert.deepEqual(reported.targets,[{id:'s1',name:'lab-switch-01',type:'ssh',connected:true,active:true},{id:'s2',name:'lab-router-02',type:'telnet',connected:true,active:false}],'connected sessions only, as their tabs name them');
 {const reports=f.reports.length,polls=f.polls;await until(()=>f.polls>=polls+2,'two more polls');assert.equal(f.reports.length,reports,'an unchanged summary is not sent again');}
 await page.evaluate(()=>selectTerminal('s2',true));await until(()=>f.reports.at(-1).targets.find(t=>t.id==='s2')?.active===true,'the changed summary');await page.evaluate(()=>selectTerminal('s1',true));
 await until(()=>f.reports.at(-1).targets.find(t=>t.id==='s1')?.active===true,'the summary after switching back');
 pass('Terminal: reports the connected sessions (tab name and type only) to its own engine when asked and when they change');

 // ---- Output to the Text Editor app ----
 await page.evaluate(()=>{window.receive=async text=>{const e=currentTerminal(),api=NetworkToolkitTerminalExtras,position=api.beforeOutput(e);api.observeOutput(e,text);await new Promise(r=>e.xterm.write(text,r));api.afterOutput(e,position);await api.flush(e);};currentTerminal().xterm.reset();});
 const receive=text=>page.evaluate(t=>receive(t),text);
 await receive('lab-switch-01# ');await page.evaluate(()=>currentTerminal().xterm.focus());await page.keyboard.type('show clock');await receive('show clock');await page.keyboard.press('Enter');await receive('\r\n10:00:00.000 UTC Thu Nov 5 2026\r\nlab-switch-01# ');
 await page.waitForFunction(()=>/10:00:00/.test(NetworkToolkitTerminalExtras.output(currentTerminal(),NetworkToolkitTerminalExtras.current(currentTerminal()))?.text||''));
 const copied=await page.evaluate(()=>{const e=currentTerminal(),x=NetworkToolkitTerminalExtras;return x.output(e,x.current(e),x.prefs(e).pasteIncludeCommands===true)?.text;});
 const toEditor=page.locator('[data-terminal-extra="send-editor"]');await toEditor.waitFor();
 await page.waitForFunction(()=>document.querySelector('[data-terminal-extra="send-editor"]')?.disabled===false);
 // The Text Editor is not open: only a new document is offered, and Send opens it first.
 await toEditor.click();await page.locator('#terminalSendEditorForm').waitFor();
 assert.equal(await page.locator('#terminalSendEditorForm fieldset legend').nth(1).textContent(),'To '+EDITOR);
 assert.match(await page.locator('[data-terminal-send-peer-missing]').innerText(),/Network Toolkit Text Editor is not open\. It opens when you choose Send to editor\./);
 assert.equal(await page.locator('[name="terminalSendTarget"][value="new"]').isChecked(),true);assert.equal(await page.locator('[name="terminalSendTarget"][value="existing"]').isDisabled(),true);
 await shot('terminal-editor-not-open');
 f.results=[{state:'queued'},{state:'queued'},{state:'taken'},{state:'done',outcome:'inserted'}];
 await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
 await toast('Sent to a new document in Network Toolkit Text Editor.');
 assert.equal(f.opens,1,'the Text Editor app is opened first');assert.deepEqual(f.sends,[{target:'new',text:copied,label:'Last command output from lab-switch-01'}]);
 assert.equal(f.peerCalls.indexOf('POST /api/peer-send/open')<f.peerCalls.indexOf('POST /api/peer-send/send'),true,'open comes before send');
 assert.equal(await page.evaluate(()=>state.activePage),'terminal');
 pass('Terminal: when Network Toolkit Text Editor is not open, To editor opens it and delivers to a new document');
 // The Text Editor is open with documents.
 await clearToasts();f.targets={...f.targets,running:true,ready:true,targets:[{id:'0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11',name:'switch notes.txt',unsaved:true},{id:'1c7a6e4f-9b66-4c1f-8a4d-3e2a1b7d8f22',name:'running-config.cfg',active:true},{id:'2d8b7f50-ac77-4d20-9b5e-4f3b2c8e9a33',name:'loading.log',busy:true}]};
 await toEditor.click();await page.locator('#terminalSendEditorForm').waitFor();
 assert.equal(await page.locator('[data-terminal-send-peer-missing]').count(),0);
 assert.deepEqual(await page.locator('#terminalSendDocument option').allTextContents(),['1. switch notes.txt (unsaved)','2. running-config.cfg','3. loading.log · unavailable']);
 assert.equal(await page.locator('#terminalSendDocument').inputValue(),'1c7a6e4f-9b66-4c1f-8a4d-3e2a1b7d8f22','the document shown in the Text Editor app is preselected');
 assert.equal(await page.locator('#terminalSendDocument option').nth(2).isDisabled(),true);assert.equal(await page.locator('#terminalSendWhere').inputValue(),'pick');
 await shot('terminal-to-editor-dialog');
 f.results=[{state:'done',outcome:'pick'}];
 await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
 await toast('Network Toolkit Text Editor is asking where the text goes in running-config.cfg.');
 assert.deepEqual(f.sends[1],{target:'1c7a6e4f-9b66-4c1f-8a4d-3e2a1b7d8f22',where:'pick',text:copied,label:'Last command output from lab-switch-01'});assert.equal(f.opens,1,'an open Text Editor is not started again');
 // A line number, from the tab menu.
 await clearToasts();await page.locator('#terminalTabs [data-terminal-tab="s1"]').click({button:'right'});
 const tabMenu=page.locator('#terminalTabContextMenu');await tabMenu.waitFor();
 assert.equal(await tabMenu.locator('[data-terminal-tab-action="send-editor"]').isEnabled(),true,'the tab menu item works in the standalone Terminal');
 await tabMenu.locator('[data-terminal-tab-action="send-editor"]').click();await page.locator('#terminalSendEditorForm').waitFor();
 await page.locator('#terminalSendDocument').selectOption('0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11');await page.locator('#terminalSendWhere').selectOption('line');await page.locator('#terminalSendLine').fill('12');
 f.results=[{state:'done',outcome:'inserted'}];
 await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();await toast('Sent to switch notes.txt in Network Toolkit Text Editor.');
 assert.deepEqual(f.sends[2],{target:'0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11',where:'line',text:copied,label:'Last command output from lab-switch-01',line:12});
 // The right-click menu, and an error from the Text Editor app.
 await clearToasts();{const b=await page.locator('.terminal-pane:not(.hidden) .xterm-screen').first().boundingBox();await page.mouse.click(b.x+80,b.y+40,{button:'right'});}
 await page.locator('#terminalContextMenu [data-terminal-menu-action="send-editor"]').click();await page.locator('#terminalSendEditorForm').waitFor();
 await page.locator('#terminalSendWhere').selectOption('end');f.results=[{state:'failed',message:'Close the document comparison first.'}];
 await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();await toast('Network Toolkit Text Editor: Close the document comparison first.');
 assert.deepEqual(f.sends[3],{target:'1c7a6e4f-9b66-4c1f-8a4d-3e2a1b7d8f22',where:'end',text:copied,label:'Last command output from lab-switch-01'});
 pass('Terminal: To editor lists the documents of Network Toolkit Text Editor with the usual position choices, from the console bar, the tab menu and the right-click menu, and shows results and errors');

 // ---- Receiving text from the Text Editor app ----
 await clearToasts();f.inputs.length=0;
 f.inbox.push({requestId:'a'.repeat(32),session:'s2',text:'show version'});
 await until(()=>f.acks.length===1,'the first acknowledgement');assert.deepEqual(f.acks[0],{requestId:'a'.repeat(32),ok:true,outcome:'sent'});
 await until(()=>f.inputs.length>=1,'terminal input');assert.equal(f.inputs.every(x=>x.id==='s2'),true,'the text goes to the chosen session only');
 const typed=f.inputs.map(x=>x.text).join('');assert.match(typed,/show version/);assert.doesNotMatch(typed,/[\r\n]/,'no line break is added');
 assert.equal(await page.evaluate(()=>terminalActive),'s2','the Terminal shows the session that received the text');
 pass('Terminal: single-line text from the Text Editor app is entered in the chosen session through the paste path');
 // Text with line breaks waits in the Terminal's own paste review.
 const commands='show version\nshow ip interface brief\n',inputsBefore=f.inputs.length;
 f.inbox.push({requestId:'b'.repeat(32),session:'s1',text:commands});
 await until(()=>f.acks.length===2,'the second acknowledgement');assert.deepEqual(f.acks[1],{requestId:'b'.repeat(32),ok:true,outcome:'review'});
 await page.locator('#terminalPasteForm').waitFor();assert.equal(await page.locator('#terminalPasteText').inputValue(),commands);assert.equal(await page.evaluate(()=>terminalActive),'s1');
 assert.equal(f.inputs.length,inputsBefore,'nothing reaches the device before the review is confirmed');
 await shot('terminal-receive-review');
 // While that review is open, more text is refused instead of replacing it.
 f.inbox.push({requestId:'c'.repeat(32),session:'s2',text:'reload'});
 await until(()=>f.acks.length===3,'the third acknowledgement');assert.deepEqual(f.acks[2],{requestId:'c'.repeat(32),ok:false,error:'A dialog is open in Network Toolkit Terminal. Finish or close it there, then send again.'});
 assert.equal(await page.locator('#terminalPasteText').inputValue(),commands,'the open review is untouched');assert.equal(f.inputs.length,inputsBefore);
 await page.locator('#terminalPasteDismiss').click();await page.locator('#terminalPasteForm').waitFor({state:'detached'});assert.equal(f.inputs.length,inputsBefore,'cancelling the review sends nothing');
 pass('Terminal: multi-line text opens the Terminal\'s paste review, and an open dialog is never replaced');
 // A session that is not connected, or unknown, is refused.
 for(const [id,session] of [['d'.repeat(32),'s3'],['e'.repeat(32),'s9']]){f.inbox.push({requestId:id,session,text:'show clock'});const n=f.acks.length;await until(()=>f.acks.length===n+1,'a refusal');assert.deepEqual(f.acks.at(-1),{requestId:id,ok:false,error:'That terminal session is no longer connected. Nothing was sent.'});}
 assert.equal(f.inputs.length,inputsBefore);
 pass('Terminal: text for a closed or unknown session is refused and nothing is sent');
 assert.deepEqual(f.unexpected,[]);
 await context.close();
}

async function mainWindow(){
 const context=await open('main'),f=fixture;
 await page.evaluate(()=>navigate('editor'));await page.locator('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="sendTerminal"]').waitFor();
 assert.equal(await page.evaluate(()=>NetworkToolkitPeerSend.sender()),'');assert.equal(await page.evaluate(()=>NetworkToolkitStandalone.mode()),'');
 await page.locator('#networkToolkitFileEditor [data-ed="new"]').first().click();await page.locator('#networkToolkitFileEditor [data-ed-text]').fill('show version');
 await page.locator('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="sendTerminal"]').click();await page.locator('#editorSendTerminalForm').waitFor();
 assert.equal(await page.locator('#editorSendTarget optgroup').count(),0,'the main window lists its own sessions');assert.deepEqual(await page.locator('#editorSendTarget option').allTextContents(),['lab-switch-01 · SSH','lab-router-02 · Telnet']);
 await page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();await page.waitForFunction(()=>state.activePage==='terminal');
 await until(()=>f.inputs.some(x=>/show version/.test(x.text)),'in-window terminal input');
 await page.waitForTimeout(2300);
 assert.deepEqual(f.peerCalls,[],'the main window never uses peer send');
 pass('Main window: Send to terminal still works inside the window and never contacts another app');
 await context.close();
}

(async()=>{
 if(output)fs.mkdirSync(output,{recursive:true});
 browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 await standaloneEditor();await standaloneTerminal();await mainWindow();
 report.errors=report.errors.filter(e=>!/Synthetic fixture/.test(e));assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.errors.push(String(error?.stack||error));console.error(error);process.exitCode=1;}).finally(async()=>{if(output)fs.writeFileSync(path.join(output,'standalone-send-results.json'),JSON.stringify(report,null,2));await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,errors:report.errors}));});
