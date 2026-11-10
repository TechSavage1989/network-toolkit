'use strict';
// Owner request: in the installed edition the standalone Text Editor sends to the standalone
// Terminal, and the standalone Terminal sends to the standalone Text Editor, with their existing
// "Send to terminal" and "To editor" commands (web/assets/peer-send.js).
// Owner request (receiver order): the engine sends to the other standalone app if it runs, else to
// the main Toolkit window, else it starts the other standalone app and delivers the text itself;
// the user is never asked to open anything and never has to send twice. Here the engine is the
// fixture, so these checks prove what the pages do with each of those answers, and that the main
// window's page receives text through the same paste and insert paths.
// The bundled application and xterm run in Chromium. Each page is ONE standalone app; the other
// app, every API response, every session and every document is synthetic. This proves the page
// behaviour: what each dialog lists, what is sent to the app's own engine, how results and "not
// running" are shown, and that received text takes the same paste and insert paths as a send
// inside one window. The real engines are covered by the Go tests in peer_send_test.go and, with
// these pages, by TestStandalonePeerSendBrowser. No native Windows claim.
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

async function open(mode,options={}){
 fixture={mode,openError:null,saved:{activePage:mode==='terminal'?'terminal':'editor',appUpdatePreferences:{enabled:false},theme:'light',terminalInteraction:{pasteReview:true,copyOnSelect:false,rightClick:'menu'},terminalSidebarCollapsed:true},
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
    else if(p==='/api/peer-send/open'&&method==='POST'){f.opens++;if(f.openError){status=f.openError.status;data={error:f.openError.error};}else{f.targets={...f.targets,receiver:'standalone',running:true,ready:true};data={app:f.targets.app,name:f.targets.name,receiver:'standalone',running:true,ready:true};}}
    else if(p==='/api/peer-send/send'&&method==='POST'){f.sends.push(body());if(f.sendError){status=f.sendError.status;data={error:f.sendError.error};}else data={requestId:String(f.sends.length).padStart(32,'0'),state:f.targets.running?'queued':'starting',name:f.targets.name,receiver:f.targets.receiver||'standalone'};}
    else if(p==='/api/peer-send/result'&&method==='GET'){assert.match(url.searchParams.get('requestId')||'',/^[0-9]{32}$/);data={requestId:url.searchParams.get('requestId'),...(f.results.length>1?f.results.shift():f.results[0]||{state:'queued'})};}
    else if(p==='/api/peer-send/inbox'&&method==='GET'){f.polls++;const request=f.inbox.shift()||null;data={request,report:!f.reports.length,from:f.targets.name};if(request)data.kind=request.session?'terminal':'editor';}
    else if(p==='/api/peer-send/report'&&method==='POST'){f.reports.push(body());data={ok:true};}
    else if(p==='/api/peer-send/ack'&&method==='POST'){f.acks.push(body());data={ok:true};}
    else{f.unexpected.push(method+' '+p);status=404;data={error:'unexpected peer-send request'};}
    return route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});}
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',windowMode:mode,storageWritable:true,localShellAvailable:true,terminalWindowAvailable:mode==='terminal',editorLaunchAvailable:mode==='editor-standalone',editorLaunchReusable:mode==='editor-standalone',peerReceive:options.peerReceive===true,system:{hostname:'Standalone fixture',interfaces:[]}};
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

 // ---- Nothing is running: the Terminal app is opened without a question ----
 // (Changed on the owner's request: this used to show "Network Toolkit Terminal is not open." with an
 // Open Terminal button the user had to press.)
 await send.click();await toast('Opening '+TERMINAL+'…');await page.locator('[data-editor-send-peer-missing]').waitFor();
 assert.equal(f.opens,1,'Send to terminal asks this app\'s engine to start the Terminal app itself');
 assert.equal(f.peerCalls.filter(c=>c==='GET /api/peer-send/targets').length,2,'the sessions are listed again once the Terminal app is ready');
 // A new Terminal has no session, and nothing connects one for the user: the same answer as inside one window.
 assert.match(await page.locator('#modalRoot .modal-body').innerText(),/No terminal session is connected in Network Toolkit Terminal\.\s*Connect one there, then choose Send to terminal again\./);assert.equal(await page.locator('#editorSendTerminalForm').count(),0);
 assert.equal((await page.locator('[data-editor-send-open-peer]').textContent()).trim(),'Show Terminal');assert.equal(await page.locator('#modalRoot').getByRole('button',{name:'Open Terminal'}).count(),0,'there is no question about opening the Terminal');assert.equal(await page.locator('#modalRoot').getByRole('button',{name:'OK'}).count(),1);
 assert.deepEqual(f.sends,[],'nothing is sent without a session');assert.equal(await text.inputValue(),doc,'the document is unchanged');
 await shot('editor-terminal-opened-no-session');
 await page.locator('[data-editor-send-open-peer]').click();await toast(TERMINAL+' is open.');
 assert.equal(f.opens,2,'Show Terminal brings the Terminal app to the front');await page.locator('#modalRoot .modal').waitFor({state:'detached'});assert.deepEqual(f.sends,[]);
 pass('Text Editor: with no receiver running, Send to terminal opens Network Toolkit Terminal by itself; with no session connected it says so, as inside one window');
 // The Terminal app does not come up: the reason is shown and the document is untouched.
 await clearToasts();f.targets={...f.targets,running:false,ready:false};f.openError={status:503,error:TERMINAL+' did not become ready. If it is open from an older version, close it and open it again.'};
 await send.click();await toast(TERMINAL+' did not become ready. If it is open from an older version, close it and open it again. Nothing was sent; your document is unchanged.');
 assert.equal(f.opens,3);assert.equal(await page.locator('#modalRoot .modal').count(),0,'no dialog is left behind');assert.equal(await text.inputValue(),doc);assert.deepEqual(f.sends,[]);f.openError=null;
 pass('Text Editor: when the Terminal app does not come up in time the user is told plainly and the text stays in the document');
 // ---- Open, but nothing connected yet ----
 f.targets={...f.targets,receiver:'standalone',running:true,ready:true,targets:[]};await clearToasts();
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
 // ---- The Terminal app is not running, the main Toolkit window is: its sessions are listed ----
 {const standaloneTargets=f.targets,opens=f.opens;await clearToasts();
  f.targets={app:'terminal',name:'Network Toolkit',receiver:'main',running:true,ready:true,targets:[{id:'m1',name:'lab-core-01',type:'ssh',connected:true,active:true}]};
  await text.evaluate(el=>{el.focus();el.setSelectionRange(0,12);});await send.click();await page.locator('#editorSendTerminalForm').waitFor();
  assert.equal(await page.locator('#editorSendTarget optgroup').getAttribute('label'),'Network Toolkit','the sessions are listed under the main window');
  assert.deepEqual(await page.locator('#editorSendTarget option').allTextContents(),['lab-core-01 · SSH']);
  assert.match(await page.locator('#editorSendTerminalForm').innerText(),/Network Toolkit opens on this session and the text is entered as a paste, using its paste settings\. Text with line breaks is shown there for review first/);
  await shot('editor-send-to-main-window');
  f.results=[{state:'queued'},{state:'done',outcome:'review'}];const sentBefore=f.sends.length;
  await page.locator('#modalRoot button[type="submit"][form="editorSendTerminalForm"]').click();
  await toast('Network Toolkit is showing the text for review before it goes to lab-core-01.');
  assert.deepEqual(f.sends.slice(sentBefore),[{session:'m1',text:'show version'}]);assert.equal(f.opens,opens,'nothing is started while the main window runs');
  // The main window has no connected session: the same answer, and no button that would start something else.
  await clearToasts();f.targets={...f.targets,targets:[]};await send.click();await page.locator('[data-editor-send-peer-missing]').waitFor();
  assert.match(await page.locator('#modalRoot .modal-body').innerText(),/No terminal session is connected in Network Toolkit\.\s*Connect one there, then choose Send to terminal again\./);assert.equal(await page.locator('[data-editor-send-open-peer]').count(),0);
  await page.locator('#modalRoot [data-action="close-modal"]').first().click();
  // An older app that was passed over is named.
  await clearToasts();f.targets={...f.targets,targets:[{id:'m1',name:'lab-core-01',type:'ssh',connected:true}],note:'Network Toolkit Terminal was skipped: it is open from an older version, or its window record cannot be read, so it cannot receive text. Close it and open it again to send to it.'};
  await send.click();await page.locator('#editorSendTerminalForm').waitFor();await toast('Network Toolkit Terminal was skipped: it is open from an older version');
  await page.locator('#modalRoot [data-action="close-modal"]').first().click();assert.equal(f.opens,opens);
  f.targets=standaloneTargets;}
 pass('Text Editor: with only the main Toolkit window running its Terminal sessions are listed and receive the text; a skipped older app is named');

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
 // Nothing is running: only a new document is offered. Send is pressed ONCE; this app's engine
 // starts the Text Editor app and delivers the text itself (the fixture answers "starting").
 // (Changed on the owner's request: the page used to call open and then send.)
 await toEditor.click();await page.locator('#terminalSendEditorForm').waitFor();
 assert.equal(await page.locator('#terminalSendEditorForm fieldset legend').nth(1).textContent(),'To '+EDITOR);
 assert.match(await page.locator('[data-terminal-send-peer-missing]').innerText(),/Network Toolkit Text Editor is not open\. It opens when you choose Send to editor\./);
 assert.equal(await page.locator('[name="terminalSendTarget"][value="new"]').isChecked(),true);assert.equal(await page.locator('[name="terminalSendTarget"][value="existing"]').isDisabled(),true);
 await shot('terminal-editor-not-open');
 f.results=[{state:'starting'},{state:'starting'},{state:'queued'},{state:'taken'},{state:'done',outcome:'inserted'}];
 await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
 await toast('Opening Network Toolkit Text Editor… The text is sent when it is ready.');
 await toast('Sent to a new document in Network Toolkit Text Editor.');
 assert.deepEqual(f.sends,[{target:'new',text:copied,label:'Last command output from lab-switch-01'}],'the text is sent once, and nothing but the text, its label and "new"');
 assert.equal(f.opens,0,'the page asks for nothing else: the engine starts the Text Editor app');assert.equal(f.peerCalls.filter(c=>c==='POST /api/peer-send/send').length,1,'no second send');
 assert.equal(await page.evaluate(()=>state.activePage),'terminal');
 pass('Terminal: with no receiver running, one Send to editor starts Network Toolkit Text Editor and delivers the text to a new document');
 // The Text Editor app does not come up: the engine's explanation is shown as it is.
 await clearToasts();await toEditor.click();await page.locator('#terminalSendEditorForm').waitFor();
 f.results=[{state:'starting'},{state:'failed',local:true,message:EDITOR+' did not become ready. If it is open from an older version, close it and open it again. Nothing was sent.'}];
 await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
 await toast(EDITOR+' did not become ready. If it is open from an older version, close it and open it again. Nothing was sent.');
 assert.equal(await page.evaluate(()=>[...document.querySelectorAll('#toastStack .toast')].some(n=>n.textContent.startsWith('Network Toolkit Text Editor: '))),false);assert.equal(f.sends.length,2);f.sends.pop();
 pass('Terminal: when the Text Editor app does not come up in time the user is told plainly');
 // The Text Editor is open with documents.
 await clearToasts();f.targets={...f.targets,receiver:'standalone',running:true,ready:true,targets:[{id:'0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11',name:'switch notes.txt',unsaved:true},{id:'1c7a6e4f-9b66-4c1f-8a4d-3e2a1b7d8f22',name:'running-config.cfg',active:true},{id:'2d8b7f50-ac77-4d20-9b5e-4f3b2c8e9a33',name:'loading.log',busy:true}]};
 await toEditor.click();await page.locator('#terminalSendEditorForm').waitFor();
 assert.equal(await page.locator('[data-terminal-send-peer-missing]').count(),0);
 assert.deepEqual(await page.locator('#terminalSendDocument option').allTextContents(),['1. switch notes.txt (unsaved)','2. running-config.cfg','3. loading.log · unavailable']);
 assert.equal(await page.locator('#terminalSendDocument').inputValue(),'1c7a6e4f-9b66-4c1f-8a4d-3e2a1b7d8f22','the document shown in the Text Editor app is preselected');
 assert.equal(await page.locator('#terminalSendDocument option').nth(2).isDisabled(),true);assert.equal(await page.locator('#terminalSendWhere').inputValue(),'pick');
 await shot('terminal-to-editor-dialog');
 f.results=[{state:'done',outcome:'pick'}];
 await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();
 await toast('Network Toolkit Text Editor is asking where the text goes in running-config.cfg.');
 assert.deepEqual(f.sends[1],{target:'1c7a6e4f-9b66-4c1f-8a4d-3e2a1b7d8f22',where:'pick',text:copied,label:'Last command output from lab-switch-01'});assert.equal(f.opens,0,'an open Text Editor is not started again');
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
 // ---- The Text Editor app is not running, the main Toolkit window is: its documents are listed ----
 {const standaloneTargets=f.targets,sentBefore=f.sends.length;await clearToasts();
  f.targets={app:'editor',name:'Network Toolkit',receiver:'main',running:true,ready:true,targets:[{id:'3e9c8061-bd88-4e31-8c6f-5a4c3d9fab44',name:'main notes.txt',active:true}],note:'Network Toolkit Text Editor was skipped: it is open from an older version, or its window record cannot be read, so it cannot receive text. Close it and open it again to send to it.'};
  await toEditor.click();await page.locator('#terminalSendEditorForm').waitFor();
  assert.equal(await page.locator('#terminalSendEditorForm fieldset legend').nth(1).textContent(),'To Network Toolkit');assert.equal(await page.locator('[data-terminal-send-peer-missing]').count(),0);
  assert.deepEqual(await page.locator('#terminalSendDocument option').allTextContents(),['1. main notes.txt']);
  assert.match(await page.locator('[data-terminal-send-peer-note]').innerText(),/Network Toolkit Text Editor was skipped: it is open from an older version/);
  assert.match(await page.locator('#terminalSendEditorForm').innerText(),/Network Toolkit opens with the text added as an ordinary edit/);
  await shot('terminal-to-main-window-dialog');
  await page.locator('#terminalSendWhere').selectOption('end');f.results=[{state:'done',outcome:'inserted'}];
  await page.locator('#modalRoot button[type="submit"][form="terminalSendEditorForm"]').click();await toast('Sent to main notes.txt in Network Toolkit.');
  assert.deepEqual(f.sends.slice(sentBefore),[{target:'3e9c8061-bd88-4e31-8c6f-5a4c3d9fab44',where:'end',text:copied,label:'Last command output from lab-switch-01'}]);assert.equal(f.opens,0,'nothing is started while the main window runs');
  f.targets=standaloneTargets;f.sends.length=sentBefore;}
 pass('Terminal: with only the main Toolkit window running its Text Editor documents are listed and receive the text; a skipped older app is named');

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

// The main Toolkit window as a receiver (owner request): only a main window whose engine says it
// can be found runs the inbox. Text for a terminal takes the Terminal page's paste path, text for
// the editor the editor's insert path, and the window shows the right page.
async function mainWindowReceives(){
 const context=await open('main',{peerReceive:true}),f=fixture;
 await page.waitForFunction(()=>terminalSessions.size===3);
 assert.equal(await page.evaluate(()=>NetworkToolkitPeerSend.sender()),'','the main window is never a sender');
 await page.evaluate(()=>navigate('editor'));await page.locator('#networkToolkitFileEditor [data-ed="new"]').first().click();
 const text=page.locator('#networkToolkitFileEditor [data-ed-text]');await text.fill('main window notes\n');
 const reported=await until(()=>f.reports.find(r=>r.targets?.length===2&&r.documents?.length===1),'the main window summary');
 assert.deepEqual(Object.keys(reported).sort(),['documents','targets']);
 assert.deepEqual(reported.targets.map(t=>[t.id,t.name,t.type,t.connected]),[['s1','lab-switch-01','ssh',true],['s2','lab-router-02','telnet',true]],'connected sessions shown in this window');
 assert.deepEqual(Object.keys(reported.documents[0]).sort(),['active','busy','id','name','unsaved']);assert.equal(reported.documents[0].name,'Untitled.txt');assert.equal(JSON.stringify(reported).includes('main window notes'),false,'document text is never reported');
 pass('Main window as receiver: reports its connected sessions and its open document names to its own engine');
 // Text for a terminal: the window shows the Terminal page and the text waits in the paste review.
 const commands='show version\nshow ip interface brief\n';f.inputs.length=0;
 f.inbox.push({requestId:'a'.repeat(32),session:'s1',text:commands});
 await until(()=>f.acks.length===1,'the first acknowledgement');assert.deepEqual(f.acks[0],{requestId:'a'.repeat(32),ok:true,outcome:'review'});
 await page.locator('#terminalPasteForm').waitFor();assert.equal(await page.locator('#terminalPasteText').inputValue(),commands);
 assert.equal(await page.evaluate(()=>state.activePage),'terminal','the main window shows its Terminal page');assert.equal(await page.evaluate(()=>terminalActive),'s1');
 assert.equal(f.inputs.length,0,'nothing reaches the device before the review is confirmed');
 await shot('main-receive-review');
 // While the review is open, more text is refused instead of replacing it.
 f.inbox.push({requestId:'b'.repeat(32),target:'new',where:'start',text:'must wait'});
 await until(()=>f.acks.length===2,'the second acknowledgement');assert.deepEqual(f.acks[1],{requestId:'b'.repeat(32),ok:false,error:'A dialog is open in Network Toolkit. Finish or close it there, then send again.'});
 assert.equal(await page.locator('#terminalPasteText').inputValue(),commands,'the open review is untouched');
 await page.locator('#terminalPasteDismiss').click();await page.locator('#terminalPasteForm').waitFor({state:'detached'});assert.equal(f.inputs.length,0,'cancelling the review sends nothing');
 // One line needs no review: it is entered in the chosen session, with no Enter added.
 f.inbox.push({requestId:'c'.repeat(32),session:'s2',text:'show clock'});
 await until(()=>f.acks.length===3,'the third acknowledgement');assert.deepEqual(f.acks[2],{requestId:'c'.repeat(32),ok:true,outcome:'sent'});
 await until(()=>f.inputs.length>=1,'terminal input');assert.equal(f.inputs.every(x=>x.id==='s2'),true);
 {const typed=f.inputs.map(x=>x.text).join('');assert.match(typed,/show clock/);assert.doesNotMatch(typed,/[\r\n]/,'no line break is added');}
 pass('Main window as receiver: terminal text takes the paste path (review for line breaks, no Enter added) on its Terminal page, and an open dialog is never replaced');
 // Text for the editor: the window shows the Text Editor.
 const before=await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length),uid=await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments()[0].uid);
 f.inbox.push({requestId:'d'.repeat(32),target:'new',where:'start',text:'10:00:00.000 UTC Thu Nov 5 2026\n',label:'Last command output from lab-switch-01'});
 await until(()=>f.acks.length===4,'the fourth acknowledgement');assert.deepEqual(f.acks[3],{requestId:'d'.repeat(32),ok:true,outcome:'inserted'});
 await page.waitForFunction(n=>NetworkToolkitFileEditor.listDocuments().length===n+1,before);assert.equal(await page.evaluate(()=>state.activePage),'editor','the main window shows its Text Editor');
 assert.equal(await text.inputValue(),'10:00:00.000 UTC Thu Nov 5 2026\n');assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments().find(d=>d.active)?.dirty),true);
 await page.evaluate(()=>navigate('terminal'));
 f.inbox.push({requestId:'e'.repeat(32),target:uid,where:'pick',text:'interface Vlan10\n',label:'Selected terminal text from lab-switch-01'});
 await until(()=>f.acks.length===5,'the fifth acknowledgement');assert.deepEqual(f.acks[4],{requestId:'e'.repeat(32),ok:true,outcome:'pick'});
 await page.locator('[data-ed-insert-bar]').waitFor();assert.equal(await page.evaluate(()=>state.activePage),'editor');
 assert.equal((await text.inputValue()).includes('Vlan10'),false,'nothing is inserted until the position is confirmed');
 await shot('main-receive-pick');
 await text.evaluate(el=>{el.focus();el.setSelectionRange(0,0);});await page.locator('[data-ed-insert-here]').click();await page.waitForFunction(()=>!document.querySelector('[data-ed-insert-bar]'));
 assert.equal(await text.inputValue(),'interface Vlan10\nmain window notes\n');
 pass('Main window as receiver: editor text arrives in a new unsaved document or through the Insert here bar in its Text Editor');
 // Its own Send to terminal is unchanged: inside the window, and no sending route is ever used.
 await text.evaluate(el=>{el.focus();el.setSelectionRange(0,16);});
 await page.locator('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="sendTerminal"]').click();await page.locator('#editorSendTerminalForm').waitFor();
 assert.equal(await page.locator('#editorSendTarget optgroup').count(),0,'the main window lists its own sessions');
 await page.locator('#modalRoot [data-action="close-modal"]').first().click();
 assert.deepEqual([...new Set(f.peerCalls)].sort(),['GET /api/peer-send/inbox','POST /api/peer-send/ack','POST /api/peer-send/report'],'the main window only receives');
 assert.deepEqual(f.unexpected,[]);
 pass('Main window as receiver: its own Send to terminal stays inside the window and it never uses a sending route');
 await context.close();
}

(async()=>{
 if(output)fs.mkdirSync(output,{recursive:true});
 browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 await standaloneEditor();await standaloneTerminal();await mainWindow();await mainWindowReceives();
 report.errors=report.errors.filter(e=>!/Synthetic fixture/.test(e));assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.errors.push(String(error?.stack||error));console.error(error);process.exitCode=1;}).finally(async()=>{if(output)fs.writeFileSync(path.join(output,'standalone-send-results.json'),JSON.stringify(report,null,2));await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,errors:report.errors}));});
