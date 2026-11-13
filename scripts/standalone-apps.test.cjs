'use strict';
// Unit checks for the standalone Text Editor and Terminal apps: what each one's command palette and
// help contain (standalone-apps.js), how a page talks to its own engine for peer send (peer-send.js),
// and that the main window and pages without a peer-send engine are left alone.
const fs=require('node:fs'),vm=require('node:vm'),path=require('node:path'),assert=require('node:assert/strict'),{test}=require('node:test'),{document:makeDocument}=require('./terminal-dom-fixture.cjs');
const asset=name=>fs.readFileSync(path.join(__dirname,'..','web','assets',name),'utf8');
const sleep=ms=>new Promise(resolve=>setTimeout(resolve,ms));

function standalone(){
 const document=makeDocument('<div id="networkToolkitFileEditor"><div class="nt-editor-status"><span data-ed-message></span></div></div>');document.body=document;
 const started=[],window={document,NetworkToolkitPeerSend:{start:mode=>started.push(mode),peerName:()=>window.peerName||''}};
 vm.runInNewContext(asset('standalone-apps.js'),{window,icon:name=>'<svg data-icon="'+name+'"></svg>',console});
 return {api:window.NetworkToolkitStandalone,document,started,window};
}
const GLOBAL=[['help','Keyboard shortcuts and help'],['welcome','Welcome and quick starts'],['palette','Search tools and commands'],['settings','App settings'],['restore-tab','Reopen recently closed tab'],['updates','Update settings / Check for updates'],['theme','Switch light/dark theme'],['navigation','Show or hide navigation']].map(([id,label])=>({id,label,group:'Application',run(){}}));
const HELP=[['Application','Ctrl+Shift+P','Search tools and commands'],['Application','F1','Open this help sheet'],['Text Editor','Ctrl+N','New document'],['Terminal','Ctrl+Shift+F','Show or hide Find in terminal output'],['File browser','F2','Rename selected file'],['Panels','Arrow keys on divider','Resize a panel']];

test('the main window and child windows keep every command, help row and footer',()=>{
 const h=standalone();
 assert.equal(h.api.configure({editor:false,terminal:false}),'');assert.equal(h.api.mode(),'');
 assert.equal(h.document.className,'');assert.deepEqual(h.started,[],'peer send is not started');
 assert.equal(h.api.commands(GLOBAL),GLOBAL);assert.equal(h.api.helpRows(HELP),HELP);assert.equal(h.api.helpIntro('intro'),'intro');
 assert.equal(h.api.decorateEditor(h.document.getElementById('networkToolkitFileEditor')),false);assert.equal(h.document.querySelector('.standalone-app-tools'),null);
});

test('the standalone Text Editor lists only its own commands and help, and gains status bar buttons',()=>{
 const h=standalone();h.window.peerName='Network Toolkit Terminal';
 assert.equal(h.api.configure({editor:true,terminal:false}),'editor');
 for(const name of ['standalone-app','standalone-editor'])assert(h.document.classList.contains(name),name);assert.equal(h.document.classList.contains('standalone-terminal'),false);
 assert.deepEqual(h.started,['editor'],'the peer inbox starts with the app');
 const commands=h.api.commands(GLOBAL);
 assert.deepEqual(commands.map(item=>item.label),['Keyboard shortcuts and help','Search tools and commands','Window settings','Reopen recently closed tab','Switch light/dark theme','Send to terminal','Open Network Toolkit Terminal']);
 assert.equal(commands.find(item=>item.id==='send-terminal').shortcut,'Ctrl+Shift+Enter');
 assert.equal(GLOBAL.find(item=>item.id==='settings').label,'App settings','the shared list is not changed in place');
 const help=h.api.helpRows(HELP).map(row=>row.join('|'));
 assert.deepEqual(help,['Application|Ctrl+Shift+P|Search tools and commands','Application|F1|Open this help sheet','Text Editor|Ctrl+N|New document','Text Editor|Ctrl+Shift+Enter|Send the selection or the document to Network Toolkit Terminal','Files sidebar|F2|Rename the selected file','Files sidebar|Ctrl / Shift+click|Select several files','Panels|Arrow keys on divider|Resize a panel']);
 assert.equal(/Terminal keystrokes/.test(h.api.helpIntro('Use buttons. Terminal keystrokes are reserved.')),false);
 // configure() already decorated the editor that existed; a second call adds nothing.
 const tools=h.document.querySelectorAll('.standalone-app-tools');assert.equal(tools.length,1);
 assert.deepEqual(tools[0].querySelectorAll('button').map(b=>b.getAttribute('aria-label')),['Switch color theme','Window settings','Keyboard shortcuts and help']);
 assert.deepEqual(tools[0].querySelectorAll('button').map(b=>b.getAttribute('data-action')||b.id),['theme','settings','uxEditorHelpLauncher']);
 assert.equal(h.api.decorateEditor(h.document.getElementById('networkToolkitFileEditor')),false);assert.equal(h.document.querySelectorAll('.standalone-app-tools').length,1);
});

test('the standalone Terminal lists only its own commands and help',()=>{
 const h=standalone();h.window.peerName='Network Toolkit Text Editor';
 assert.equal(h.api.configure({editor:false,terminal:true}),'terminal');
 assert(h.document.classList.contains('standalone-terminal'));assert.equal(h.document.classList.contains('standalone-editor'),false);assert.deepEqual(h.started,['terminal']);
 assert.deepEqual(h.api.commands(GLOBAL).map(item=>item.label),['Keyboard shortcuts and help','Search tools and commands','Window settings','Reopen recently closed tab','Switch light/dark theme','Send output to Text Editor','Open Network Toolkit Text Editor']);
 assert.deepEqual(h.api.helpRows(HELP).map(row=>row[0]+'|'+row[1]),['Application|Ctrl+Shift+P','Application|F1','Terminal|Ctrl+Shift+F','Terminal|Send to Text Editor','Panels|Arrow keys on divider']);
 assert.equal(h.api.helpIntro('intro'),'intro');
 assert.equal(h.document.querySelector('.standalone-app-tools'),null,'the Terminal keeps its app buttons in its sidebar rail');
});

// peer-send.js against a recording stand-in for the page's own engine.
function peer(options={}){
 const document=makeDocument('<div id="modalRoot"></div><div id="toastStack"></div>');document.body=document;
 const calls=[],toasts=[],timers=[],window={document,setTimeout,toast:(message,kind)=>toasts.push([message,kind||''])};
 const engine=options.engine||(()=>({}));
 window.api=async(route,method='GET',body)=>{calls.push([method,route,body]);const reply=await engine(route,method,body,calls);if(reply instanceof Error)throw reply;return reply;};
 const context={window,console,setTimeout,setInterval:(fn,ms)=>{timers.push([fn,ms]);return timers.length;},TextEncoder,online:true,shutDown:false,OFFLINE:false,terminalActive:'s1',...options.context};
 vm.runInNewContext(asset('peer-send.js'),context);
 return {api:window.NetworkToolkitPeerSend,window,calls,toasts,timers,context,document};
}

test('a page without a standalone mode never contacts a peer-send engine',async()=>{
 const h=peer();
 assert.equal(h.api.sender(),'');assert.equal(h.api.peerName(),'');assert.equal(h.api.start('main'),false);assert.equal(h.api.start(''),false);
 await h.api.poll();assert.deepEqual(JSON.parse(JSON.stringify(await h.api.targets())),{running:false,ready:false,targets:[]});assert.equal(await h.api.open(),false);
 assert.equal((await h.api.send({session:'s1',text:'x'})).ok,false);
 assert.deepEqual(h.calls,[]);assert.deepEqual(h.timers,[]);
});

test('the standalone page polls with a GET and reports its targets only when asked or changed',async()=>{
 const documents=[{id:'local:C:\\Users\\operator\\notes.txt',uid:'0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11',name:'notes.txt',path:'C:\\Users\\operator\\notes.txt',source:'This computer',dirty:true,active:true,blocked:''}];
 let asked=true;
 const h=peer({engine:(route,method)=>route==='/peer-send/inbox'?{request:null,report:asked}:method==='POST'?{ok:true}:{}});
 h.window.NetworkToolkitFileEditor={listDocuments:()=>documents};
 assert.equal(h.api.start('editor'),true);assert.equal(h.api.sender(),'editor');assert.equal(h.api.peerKind(),'terminal');assert.equal(h.api.peerName(),'Network Toolkit Terminal');
 assert.equal(h.timers.length,1);assert.equal(h.timers[0][1],1000,'one poll a second');assert.equal(h.api.start('editor'),true);assert.equal(h.timers.length,1,'started once');
 await h.api.poll();
 assert.deepEqual(h.calls.map(c=>c[0]+' '+c[1]),['GET /peer-send/inbox','POST /peer-send/report']);
 // Names only: no document text, path, source or internal id leaves the page.
 assert.deepEqual(JSON.parse(JSON.stringify(h.calls[1][2])),{targets:[{id:'0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11',name:'notes.txt',unsaved:true,busy:false,active:true}]});
 asked=false;await h.api.poll();await h.api.poll();
 assert.deepEqual(h.calls.slice(2).map(c=>c[0]+' '+c[1]),['GET /peer-send/inbox','GET /peer-send/inbox'],'an unchanged summary is not sent again');
 documents[0].dirty=false;documents.push({id:'x',uid:'bad uid with spaces',name:'skipped.txt'},{id:'y',uid:'1c7a6e4f-9b66-4c1f-8a4d-3e2a1b7d8f22',name:'second\u0007.cfg',blocked:'That document is still loading.'});
 await h.api.poll();
 assert.deepEqual(JSON.parse(JSON.stringify(h.calls.at(-1))),['POST','/peer-send/report',{targets:[{id:'0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11',name:'notes.txt',unsaved:false,busy:false,active:true},{id:'1c7a6e4f-9b66-4c1f-8a4d-3e2a1b7d8f22',name:'second .cfg',unsaved:false,busy:true,active:false}]}]);
 // While the app is not connected to its engine, or is closing, nothing is polled.
 const before=h.calls.length;h.context.online=false;await h.api.poll();h.context.online=true;h.context.shutDown=true;await h.api.poll();assert.equal(h.calls.length,before);
});

test('a fixture or older engine that does not answer with a report flag is never sent a report',async()=>{
 const h=peer({engine:()=>({})});h.window.NetworkToolkitFileEditor={listDocuments:()=>[{id:'a',uid:'a1',name:'a.txt'}]};
 h.api.start('editor');await h.api.poll();await h.api.poll();
 assert.deepEqual(h.calls.map(c=>c[0]+' '+c[1]),['GET /peer-send/inbox','GET /peer-send/inbox']);
});

test('the Terminal page reports connected sessions by tab name and delivers received text through the paste path',async()=>{
 const entries=[{info:{id:'s1',type:'ssh',label:'lab-switch-01',state:'connected'}},{info:{id:'s2',type:'TELNET?',label:'',host:'192.0.2.11',state:'connected'}}];
 const inbox=[],acks=[],delivered=[];
 const h=peer({engine:(route,method,body)=>{if(route==='/peer-send/inbox')return {request:inbox.shift()||null,report:false};if(route==='/peer-send/ack'){acks.push(body);return {ok:true};}return {ok:true};}});
 h.window.NetworkToolkitEditorSendTerminal={targets:()=>entries,label:entry=>entry.info.label||entry.info.host||'Terminal',deliver:async(entry,text,options)=>{delivered.push([entry.info.id,text,options]);return text.includes('\n')?{ok:true,outcome:'review'}:{ok:true,outcome:'sent'};}};
 h.api.start('terminal');await h.api.poll();
 assert.deepEqual(JSON.parse(JSON.stringify(h.calls.find(c=>c[1]==='/peer-send/report')[2])),{targets:[{id:'s1',name:'lab-switch-01',type:'ssh',connected:true,active:true},{id:'s2',name:'192.0.2.11',type:'',connected:true,active:false}]});
 inbox.push({requestId:'a'.repeat(32),session:'s2',text:'show version'});await h.api.poll();
 assert.deepEqual(JSON.parse(JSON.stringify(delivered)),[['s2','show version',{report:true}]]);assert.deepEqual(JSON.parse(JSON.stringify(acks)),[{requestId:'a'.repeat(32),ok:true,outcome:'sent'}]);
 inbox.push({requestId:'b'.repeat(32),session:'s1',text:'a\nb\n'});await h.api.poll();assert.deepEqual(JSON.parse(JSON.stringify(acks.at(-1))),{requestId:'b'.repeat(32),ok:true,outcome:'review'});
 // A session that is gone, and an open dialog, are refused without touching the paste path.
 inbox.push({requestId:'c'.repeat(32),session:'s9',text:'x'});await h.api.poll();assert.deepEqual(JSON.parse(JSON.stringify(acks.at(-1))),{requestId:'c'.repeat(32),ok:false,error:'That terminal session is no longer connected. Nothing was sent.'});
 h.document.querySelector('#modalRoot').innerHTML='<section class="modal"></section>';
 inbox.push({requestId:'d'.repeat(32),session:'s1',text:'x'});await h.api.poll();assert.deepEqual(JSON.parse(JSON.stringify(acks.at(-1))),{requestId:'d'.repeat(32),ok:false,error:'A dialog is open in Network Toolkit Terminal. Finish or close it there, then send again.'});
 assert.equal(delivered.length,2);
});

test('sending shows the result that the other app reports',async()=>{
 const results=[{state:'queued'},{state:'taken'},{state:'done',outcome:'review'}];
 const h=peer({engine:(route,method)=>route==='/peer-send/send'?{requestId:'1'.repeat(32),state:'queued'}:route.startsWith('/peer-send/result?requestId='+'1'.repeat(32))?results.shift():new Error('unexpected '+method+' '+route)});
 h.api.start('editor');
 const sent=await h.api.send({session:'s1',text:'show version\n'},{target:'lab-switch-01',interval:1});
 assert.equal(sent.ok,true);assert.equal(sent.outcome,'review');
 assert.deepEqual(h.toasts,[['Network Toolkit Terminal is showing the text for review before it goes to lab-switch-01.','success']]);
 assert.deepEqual(JSON.parse(JSON.stringify(h.calls[0])),['POST','/peer-send/send',{session:'s1',text:'show version\n'}]);
 // An error from the other app, and a refusal by this app's own engine, are shown in plain words.
 const failed=peer({engine:route=>route==='/peer-send/send'?{requestId:'2'.repeat(32),state:'queued'}:{state:'failed',message:'Close the document comparison first.'}});
 failed.api.start('terminal');assert.equal((await failed.api.send({target:'new',text:'x'},{interval:1})).ok,false);
 assert.deepEqual(failed.toasts,[['Network Toolkit Text Editor: Close the document comparison first.','error']]);
 const refused=peer({engine:()=>new Error('Network Toolkit Terminal is not open. Nothing was sent.')});
 refused.api.start('editor');assert.equal((await refused.api.send({session:'s1',text:'x'})).ok,false);
 assert.deepEqual(refused.toasts,[['Network Toolkit Terminal is not open. Nothing was sent.','error']]);assert.equal(refused.calls.length,1,'no result is polled after a refusal');
 // Results for each destination.
 for(const [mode,outcome,target,text] of [['editor','sent','COM3','Sent to COM3 in Network Toolkit Terminal.'],['terminal','inserted','notes.txt','Sent to notes.txt in Network Toolkit Text Editor.'],['terminal','pick','notes.txt','Network Toolkit Text Editor is asking where the text goes in notes.txt.'],['terminal','inserted','a new document','Sent to a new document in Network Toolkit Text Editor.']]){
  const ok=peer({engine:route=>route==='/peer-send/send'?{requestId:'3'.repeat(32),state:'queued'}:{state:'done',outcome}});ok.api.start(mode);await ok.api.send({text:'x'},{target,interval:1});assert.deepEqual(ok.toasts,[[text,'success']]);}
 await sleep(5);
});

test('listing the other app tolerates a missing engine and keeps only safe fields',async()=>{
 const h=peer({engine:route=>route==='/peer-send/targets'?{running:true,ready:true,targets:[{id:'s1',name:'lab\u0000-switch-01',type:'ssh',connected:true,token:'never shown'},{id:'bad id',name:'x',connected:true},null,{id:'s2',name:'',connected:true}]}:{}});
 h.api.start('editor');
 assert.deepEqual(JSON.parse(JSON.stringify(await h.api.targets())),{running:true,ready:true,targets:[{id:'s1',name:'lab -switch-01',type:'ssh',connected:true,unsaved:false,busy:false,active:false},{id:'s2',name:'Untitled',type:'',connected:true,unsaved:false,busy:false,active:false}]});
 const down=peer({engine:()=>new Error('Request failed (404).')});down.api.start('terminal');
 assert.deepEqual(JSON.parse(JSON.stringify(await down.api.targets())),{running:false,ready:false,targets:[],error:'Request failed (404).'});
});

// Owner request: the receiver is the other standalone app, else the main Toolkit window, else the
// other standalone app is started. The engine decides; the page shows which app answered.
test('the main Toolkit window only receives: sessions and documents reported, text delivered by kind, nothing sent',async()=>{
 const entries=[{info:{id:'m1',type:'ssh',label:'lab-core-01',state:'connected'}}],documents=[{id:'local:1',uid:'0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11',name:'main notes.txt',path:'C:\\Users\\operator\\main notes.txt',dirty:true,active:true,blocked:''}];
 const inbox=[],acks=[],pasted=[],inserted=[];
 const h=peer({context:{terminalActive:'m1'},engine:(route,method,body)=>{if(route==='/peer-send/inbox'){const next=inbox.shift();return next?{request:next.request,kind:next.kind,report:false}:{request:null,report:false};}if(route==='/peer-send/ack'){acks.push(body);return {ok:true};}return {ok:true};}});
 h.window.NetworkToolkitEditorSendTerminal={targets:()=>entries,label:entry=>entry.info.label,deliver:async(entry,text,options)=>{pasted.push([entry.info.id,text,options]);return {ok:true,outcome:'review'};}};
 h.window.NetworkToolkitFileEditor={listDocuments:()=>documents};
 h.window.NetworkToolkitTerminalSendEditor={deliver:async request=>{inserted.push(request);return {ok:true,outcome:request.where==='pick'?'pick':'inserted'};}};
 assert.equal(h.api.start('main'),false,'start() is for the standalone apps');assert.deepEqual(h.timers,[]);
 assert.equal(h.api.startMain(),true);assert.equal(h.timers.length,1);assert.equal(h.api.startMain(),true);assert.equal(h.timers.length,1,'started once');
 assert.equal(h.api.sender(),'','the main window is never a sender');assert.equal(h.api.peerName(),'');assert.equal(h.api.peerKind(),'');
 // Its own Send to commands stay inside the window: no engine call is made for them.
 assert.deepEqual(JSON.parse(JSON.stringify(await h.api.targets())),{running:false,ready:false,targets:[]});assert.equal(await h.api.open(),false);assert.equal((await h.api.send({session:'m1',text:'x'})).ok,false);
 assert.deepEqual(h.calls,[]);
 await h.api.poll();
 assert.deepEqual(h.calls.map(c=>c[0]+' '+c[1]),['GET /peer-send/inbox','POST /peer-send/report']);
 assert.deepEqual(JSON.parse(JSON.stringify(h.calls[1][2])),{targets:[{id:'m1',name:'lab-core-01',type:'ssh',connected:true,active:true}],documents:[{id:'0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11',name:'main notes.txt',unsaved:true,busy:false,active:true}]},'names only: no path, no text');
 // Terminal text takes the paste path; editor text the insert path.
 inbox.push({kind:'terminal',request:{requestId:'a'.repeat(32),session:'m1',text:'show version\nshow clock\n'}});await h.api.poll();
 assert.deepEqual(JSON.parse(JSON.stringify(pasted)),[['m1','show version\nshow clock\n',{report:true}]]);assert.deepEqual(JSON.parse(JSON.stringify(acks.at(-1))),{requestId:'a'.repeat(32),ok:true,outcome:'review'});
 inbox.push({kind:'editor',request:{requestId:'b'.repeat(32),target:'0b6f5d3e-8a55-4b0e-9f3c-2d1f0a6c7e11',where:'pick',text:'output\n',label:'Last command output'}});await h.api.poll();
 assert.deepEqual(JSON.parse(JSON.stringify(inserted)),[{docId:'local:1',where:'pick',text:'output\n',label:'Last command output',report:true}]);assert.deepEqual(JSON.parse(JSON.stringify(acks.at(-1))),{requestId:'b'.repeat(32),ok:true,outcome:'pick'});
 inbox.push({kind:'editor',request:{requestId:'c'.repeat(32),target:'new',where:'start',text:'new\n'}});await h.api.poll();
 assert.deepEqual(JSON.parse(JSON.stringify(inserted.at(-1))),{docId:'',where:'start',text:'new\n',label:'Terminal output',report:true});
 // A request without a known kind, and an open dialog, are refused; nothing is pasted or inserted.
 inbox.push({kind:'',request:{requestId:'d'.repeat(32),session:'m1',text:'x'}});await h.api.poll();assert.deepEqual(JSON.parse(JSON.stringify(acks.at(-1))),{requestId:'d'.repeat(32),ok:false,error:'Network Toolkit cannot take the text right now.'});
 h.document.querySelector('#modalRoot').innerHTML='<section class="modal"></section>';
 for(const [id,kind,request] of [['e','terminal',{session:'m1',text:'x'}],['f','editor',{target:'new',where:'start',text:'x'}]]){inbox.push({kind,request:{requestId:id.repeat(32),...request}});await h.api.poll();assert.deepEqual(JSON.parse(JSON.stringify(acks.at(-1))),{requestId:id.repeat(32),ok:false,error:'A dialog is open in Network Toolkit. Finish or close it there, then send again.'});}
 assert.equal(pasted.length,1);assert.equal(inserted.length,2);
 assert.equal(h.calls.some(c=>/^\/peer-send\/(targets|open|send|result)/.test(c[1])),false,'the main window page never uses a sending route');
});

test('a send names the app that received it, and waits without a second click while the other app is started',async()=>{
 // The main Toolkit window answered.
 const main=peer({engine:route=>route==='/peer-send/send'?{requestId:'4'.repeat(32),state:'queued',name:'Network Toolkit',receiver:'main'}:{state:'done',outcome:'review',name:'Network Toolkit'}});
 main.api.start('editor');assert.equal((await main.api.send({session:'m1',text:'a\nb\n'},{target:'lab-core-01',interval:1})).ok,true);
 assert.deepEqual(main.toasts,[['Network Toolkit is showing the text for review before it goes to lab-core-01.','success']]);
 // Nothing was running: the engine starts the other app and delivers the same text itself.
 const results=[{state:'starting'},{state:'starting'},{state:'queued'},{state:'done',outcome:'inserted'}];
 const started=peer({engine:route=>route==='/peer-send/send'?{requestId:'5'.repeat(32),state:'starting',name:'Network Toolkit Text Editor',receiver:'standalone'}:results.shift()});
 started.api.start('terminal');const sent=await started.api.send({target:'new',text:'output\n'},{target:'a new document',interval:1});
 assert.equal(sent.ok,true);assert.deepEqual(started.toasts,[['Opening Network Toolkit Text Editor… The text is sent when it is ready.',''],['Sent to a new document in Network Toolkit Text Editor.','success']]);
 assert.equal(started.calls.filter(c=>c[1]==='/peer-send/send').length,1,'the text is sent once');assert.equal(started.calls.some(c=>c[1]==='/peer-send/open'),false,'the page does not open the other app itself');
 // The other app did not come up: the engine's own explanation is shown as it is.
 const late=peer({engine:route=>route==='/peer-send/send'?{requestId:'6'.repeat(32),state:'starting',name:'Network Toolkit Text Editor'}:{state:'failed',local:true,message:'Network Toolkit Text Editor did not become ready. If it is open from an older version, close it and open it again. Nothing was sent.'}});
 late.api.start('terminal');assert.equal((await late.api.send({target:'new',text:'x'},{interval:1})).ok,false);
 assert.deepEqual(late.toasts.at(-1),['Network Toolkit Text Editor did not become ready. If it is open from an older version, close it and open it again. Nothing was sent.','error']);
 // An app that was passed over is named.
 const noted=peer({engine:route=>route==='/peer-send/send'?{requestId:'7'.repeat(32),state:'queued',name:'Network Toolkit',note:'Network Toolkit Terminal was skipped: it is open from an older version.'}:{state:'done',outcome:'sent'}});
 noted.api.start('editor');await noted.api.send({session:'m1',text:'x'},{target:'lab-core-01',interval:1});
 assert.deepEqual(noted.toasts,[['Network Toolkit Terminal was skipped: it is open from an older version.',''],['Sent to lab-core-01 in Network Toolkit.','success']]);
 const listed=peer({engine:()=>({running:true,ready:true,name:'Network Toolkit',receiver:'main',note:'Network Toolkit Terminal was skipped.',token:'never shown',targets:[{id:'m1',name:'lab-core-01',type:'ssh',connected:true}]})});
 listed.api.start('editor');
 assert.deepEqual(JSON.parse(JSON.stringify(await listed.api.targets())),{running:true,ready:true,targets:[{id:'m1',name:'lab-core-01',type:'ssh',connected:true,unsaved:false,busy:false,active:false}],name:'Network Toolkit',receiver:'main',note:'Network Toolkit Terminal was skipped.'});
 // Opening failed: the reason, plus what happened to the user's text.
 const closed=peer({engine:()=>new Error('Network Toolkit Terminal did not become ready.')});closed.api.start('editor');
 assert.equal(await closed.api.open({quiet:true,failure:' Nothing was sent; your document is unchanged.'}),false);
 assert.deepEqual(closed.toasts,[['Network Toolkit Terminal did not become ready. Nothing was sent; your document is unchanged.','error']]);
 await sleep(5);
});
