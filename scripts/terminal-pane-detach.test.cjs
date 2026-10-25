'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test'),{document:makeDocument,Element}=require('./terminal-dom-fixture.cjs');
const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
function appFunction(name){const start=html.search(new RegExp('^(?:async )?function '+name+'\\(','m'));assert(start>=0,name);const tail=html.slice(start),next=tail.slice(1).search(/\n(?:async )?function /);return tail.slice(0,next+1);}
function harness(detached=false){
 const document=makeDocument('<div id="terminalStage"><div id="terminalPaneGrid"></div></div><div id="terminalParking"></div><select id="terminalLayoutSelect"></select><span id="terminalFocusLabel"></span><button id="terminalRestoreLayout"></button>');
 const calls=[],sessions=new Map(),revisions=new Map();
 for(const id of ['one','two','three','four']){sessions.set(id,{info:{id,label:id,type:'ssh',state:'connected'},config:{type:'ssh'},element:new Element(),xterm:{options:{}},disposables:[]});revisions.set(id,0);}
 document.querySelector('#terminalPaneGrid').style.setProperty=()=>{};
 const c={document,terminalSessions:sessions,terminalDraftTabs:new Map(),terminalPanes:['one','two','three','four'],terminalPaneFocus:0,terminalActive:'one',terminalPaneMaximized:-1,state:{terminalLayout:{kind:'grid',columnRatio:.5,rowRatio:.5}},terminalViewRegistry:{available:true,views:detached?[{id:'source',workspace:true,state:'detached',sessionIds:[...sessions.keys()]}]:[]},terminalConnectBusy:false,terminalWorkspaceTransferBusy:false,terminalDetachedReturning:false,terminalDetachedReady:true,terminalWorkspaceCheckpointSignature:'old',terminalViewError:'',DETACHED_TERMINAL:detached,DETACHED_WORKSPACE:detached,online:true,clearTimeout(){},terminalPaneCount:()=>4,safeTerminalLayout:x=>x,normalizeTerminalPanes(){},terminalWelcomeHTML:()=>'<h3>Empty pane</h3>',terminalStatusClass:()=>'',terminalStatusLabel:i=>i.state,beginTerminalPaneResize(){},resizeTerminalPaneByKey(){},focusTerminalPane(){},renderTerminalConnectionState(){},esc:String,icon:()=>'',terminalSessionRevision:id=>revisions.get(id),terminalOwnerCanMutate:e=>!!e&&!e.unowned,terminalEntryAcceptsInput:e=>e?.info.id===c.terminalActive&&!e.unowned,terminalVisibleEntries:()=>[...sessions.values()].filter(e=>c.terminalEntryShown(e)),terminalEntryShown:e=>!c.terminalViewRegistry.views.some(v=>!v.workspace&&v.sessionId===e.info.id),terminalViewForSession:id=>c.terminalViewRegistry.views.find(v=>v.sessionId===id),currentTerminal:()=>sessions.get(c.terminalActive),renderTerminalChrome:()=>c.renderTerminalPanes(),scheduleTerminalFit(){},flushState:async()=>{calls.push(['flush',c.state.terminalSidebarSide]);return true;},settleTerminalOutgoing:async e=>calls.push(['settle',e.info.id]),captureTerminalSnapshot:async e=>({data:'retained '+e.info.id,after:100,config:{type:'ssh'}}),withTerminalFeedPaused:async fn=>fn(),receiveTerminalViewState:s=>{c.terminalViewRegistry=s;},dropTerminalDisplay:e=>{calls.push(['drop',e.info.id]);sessions.delete(e.info.id);c.terminalPanes=c.terminalPanes.map(id=>id===e.info.id?'':id);},requestTerminalReattach:async e=>calls.push(['reattach',e.info.id]),toast:(text,kind)=>calls.push(['toast',kind,text]),api:async(route,method,body)=>{calls.push(['api',route,body.sessionId,body.snapshot.data]);return {view:{id:'view-'+body.sessionId,sessionId:body.sessionId,state:'opening'},...(detached?{viewState:{available:true,views:[{id:'source',workspace:true,state:'detached',sessionIds:[...sessions.keys()].filter(id=>id!==body.sessionId)}]}}:{})};}};
 c.$=(selector,root=document)=>root.querySelector(selector);
 vm.createContext(c);for(const name of ['terminalPaneIndex','terminalSingleDetachReason','terminalPaneMoveState','syncTerminalPaneMove','moveTerminalPaneSession','initializeTerminalPanes','renderTerminalPanes','detachTerminal'])vm.runInContext(appFunction(name),c);
 c.renderTerminalPanes();return {c,calls,sessions,revisions,document,button:index=>document.querySelector('[data-terminal-pane-move="'+index+'"]')};
}
test('each actual split pane detaches its own session while sibling pane DOM remains in place',async()=>{
 const h=harness(),bodies=[0,1,2,3].map(i=>h.document.querySelector('[data-terminal-pane-body="'+i+'"]')),elements=[...h.sessions.values()].map(e=>e.element);
 assert.equal(h.document.querySelectorAll('[data-terminal-pane-move]').length,4);
 await h.c.moveTerminalPaneSession(h.button(2));
 assert.deepEqual(h.calls.filter(c=>c[0]==='api'),[['api','/terminal-views','three','retained three']]);assert.equal(h.c.terminalActive,'one');assert.deepEqual(Array.from(h.c.terminalPanes),['one','two','three','four']);
 for(const i of [0,1,3])assert.equal(elements[i].parentElement,bodies[i]);
 assert.equal(h.button(0).disabled,false);assert.equal(h.button(1).disabled,false);assert.equal(h.button(2).textContent,'Opening...');
 h.c.terminalViewRegistry.views[0].state='detached';h.c.renderTerminalPanes();assert.equal(h.button(2).textContent,'Reattach');await h.c.moveTerminalPaneSession(h.button(2));assert.deepEqual(h.calls.filter(c=>c[0]==='reattach'),[['reattach','three']]);
 await h.c.moveTerminalPaneSession(h.button(1));assert.equal(h.calls.filter(c=>c[0]==='api').at(-1)[2],'two');assert.equal(h.c.terminalViewRegistry.views.length,2);
});
test('pane detach rejects changed sessions, stale ownership, pending prompts and repeated clicks',async()=>{
 const h=harness(),button=h.button(1);h.revisions.set('two',1);await h.c.moveTerminalPaneSession(button);assert.equal(h.calls.length,0);
 h.c.renderTerminalPanes();button.dataset.terminalPaneSession='one';await h.c.moveTerminalPaneSession(button);assert.equal(h.calls.length,0);
 h.sessions.get('two').info.prompt={id:'password'};h.c.renderTerminalPanes();assert.equal(button.disabled,true);await h.c.moveTerminalPaneSession(button);assert.equal(h.calls.length,0);
 h.sessions.get('two').info.prompt=null;h.c.renderTerminalPanes();const first=h.c.moveTerminalPaneSession(button),second=h.c.moveTerminalPaneSession(button);await Promise.all([first,second]);assert.equal(h.calls.filter(c=>c[0]==='api').length,1);
});
test('a pane inside a linked workspace moves independently and only its display leaves that workspace',async()=>{
 const h=harness(true),one=h.sessions.get('one').element,four=h.sessions.get('four').element;
 await h.c.moveTerminalPaneSession(h.button(1));assert.deepEqual(h.calls.filter(c=>c[0]==='api'),[['api','/terminal-views/current/detach-session','two','retained two']]);
 assert.deepEqual(h.calls.filter(c=>c[0]==='drop'),[['drop','two']]);assert.deepEqual(Array.from(h.c.terminalPanes),['one','','three','four']);assert.equal(h.sessions.size,3);assert.equal(one.parentElement,h.document.querySelector('[data-terminal-pane-body="0"]'));assert.equal(four.parentElement,h.document.querySelector('[data-terminal-pane-body="3"]'));assert.equal(h.c.terminalWorkspaceTransferBusy,false);
});

test('detaching a session waits for its latest sidebar side to reach the source display state',async()=>{
 for(const detached of [false,true]){
  const h=harness(detached);h.c.state.terminalSidebarSide='left';let finish;
  h.c.flushState=()=>{h.calls.push(['flush',h.c.state.terminalSidebarSide]);return new Promise(resolve=>{finish=resolve;});};
  const move=h.c.moveTerminalPaneSession(h.button(1));await new Promise(resolve=>setImmediate(resolve));
  assert.deepEqual(h.calls.filter(row=>row[0]==='flush'),[['flush','left']]);assert.equal(h.calls.filter(row=>row[0]==='api').length,0);
  finish(true);await move;assert.equal(h.calls.filter(row=>row[0]==='api').length,1);
  assert(h.calls.findIndex(row=>row[0]==='flush')<h.calls.findIndex(row=>row[0]==='api'));
 }
});
test('a failed display save leaves the original session available instead of detaching stale preferences',async()=>{
 const h=harness();h.c.flushState=async()=>false;await h.c.moveTerminalPaneSession(h.button(1));
 assert.equal(h.calls.filter(row=>row[0]==='api').length,0);assert.equal(h.sessions.size,4);assert.equal(h.sessions.get('two').transferBusy,false);
 assert(h.calls.some(row=>row[0]==='toast'&&row[1]==='error'));
});
