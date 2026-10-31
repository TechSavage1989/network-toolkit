'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
const interaction=fs.readFileSync(path.join(__dirname,'../web/assets/terminal-interaction.js'),'utf8');
function source(name){const start=html.search(new RegExp('^(?:async )?function '+name+'\\(','m'));assert(start>=0,'Missing '+name);const tail=html.slice(start);const next=tail.slice(1).search(/\n(?:async )?function /);return next<0?tail:tail.slice(0,next+1);}
function setup(){
 const entry={info:{id:'session-one',state:'closed',type:'ssh',host:'192.0.2.10',port:2222,updatedAt:'2026-10-22T00:00:00Z'},config:{type:'ssh',host:'192.0.2.10',port:2222,username:'operator',authMethod:'password'},inputChunks:[Uint8Array.from([65,66])],inputBytes:2,inputError:'Old input failure',inputUncertain:true,answeredPrompts:new Set(['old-prompt']),xterm:{options:{},focus(){},hasSelection:()=>false,clearSelection(){this.cleared=true;}}};
 const pane=['session-one','other','',''],requests=[],notices=[],status={dataset:{},innerHTML:'',classList:{toggle(name,value){this[name]=value;}}},outputBar={dataset:{},innerHTML:'',classList:{toggle(name,value){this[name]=value;}}};
 const c={entry,pane,requests,notices,status,online:true,OFFLINE:false,terminalSessions:new Map([[entry.info.id,entry]]),terminalWorkspaceTransferBusy:false,terminalDetachedReturning:false,terminalAssetsReady:()=>true,terminalEntryShown:()=>true,terminalOwnerCanMutate:()=>true,currentTerminal:()=>c.focus||entry,renderTerminalChrome:()=>{},scheduleTerminalFit:()=>{},startTerminalFeed:()=>{},receiveTerminalViewState:s=>{c.viewState=s;},upsertTerminalSession:s=>{entry.info={...s};return entry;},NetworkToolkitTerminalExtras:{finalize:e=>{e.finalized=true;},scrollBottom:e=>{e.followResumed=true;}},toast:(...m)=>notices.push(m),setTimeout,clearTimeout,Uint8Array,TextEncoder,bytesToBase64:bytes=>Buffer.from(bytes).toString('base64'),icon:()=>'',esc:s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),$:()=>status,openTerminalConnect:()=>assert.fail('Reconnect opened a connection form')};
 c.api=async(...args)=>{requests.push(args);return {session:{...entry.info,state:'connecting',updatedAt:'2026-10-22T00:01:00Z'},viewState:{revision:1}};};
 c.document={querySelector:()=>null};c.safeTerminalConfig=value=>{const clean={};for(const field of ['type','host','port','username','authMethod','credentialId'])if(value[field]!==undefined)clean[field]=value[field];return clean;};c.outputBar=outputBar;c.$=selector=>selector==='[data-terminal-connection-output]'?outputBar:status;c.DETACHED_TERMINAL=false;c.DETACHED_WORKSPACE=false;c.window={};
 vm.createContext(c);for(const name of ['cancelTerminalResizeTimer','terminalConnectionButtonsHTML','terminalReconnectAllowed','renderTerminalConnectionState','setTerminalConnectionScreen','notifyTerminalMacroConnected','reconnectTerminal','drainTerminalInput'])vm.runInContext(source(name),c);
 return c;
}
test('Reconnect of an active session reuses the same tab, pane and settings without opening a connection form',async()=>{
 const c=setup(),entry=c.entry,term=entry.xterm,originalConfig=entry.config,queued=entry.inputChunks[0];entry.info.state='connected';await c.reconnectTerminal(entry);
 assert.equal(c.terminalSessions.size,1);assert.equal(c.terminalSessions.get(entry.info.id),entry);assert.equal(entry.xterm,term);assert.equal(entry.config,originalConfig);assert.equal(c.pane[0],entry.info.id);assert.equal(c.requests.length,1);assert.equal(c.requests[0][0],'/terminals/session-one/reconnect');assert.equal(c.requests[0][1],'POST');assert.deepEqual(JSON.parse(JSON.stringify(c.requests[0][2])),{});assert.deepEqual([...queued],[0,0]);assert.equal(entry.inputBytes,0);assert.equal(entry.inputError,'');assert.equal(entry.finalized,true);assert.equal(entry.followResumed,true);assert.equal(entry.reconnectBusy,false);assert.equal(c.viewState.revision,1);
});
test('Repeated clicks and transfer states cannot start another reconnect',async()=>{
 const c=setup();let resolve;c.api=(...args)=>{c.requests.push(args);return new Promise(r=>resolve=r);};const running=c.reconnectTerminal(c.entry);await c.reconnectTerminal(c.entry);assert.equal(c.requests.length,1);resolve({session:{...c.entry.info,state:'connecting'},viewState:{revision:1}});await running;c.entry.info.state='closed';c.entry.transferBusy=true;await c.reconnectTerminal(c.entry);assert.equal(c.requests.length,1);
});
test('Disconnected panes display their own status and reconnect control',()=>{
 const c=setup(),pane={dataset:{terminalPane:'2'}};c.renderTerminalConnectionState(pane,c.entry,false);assert.equal(c.status.classList.hidden,false);assert.equal(c.outputBar.classList.hidden,true);assert.match(c.status.innerHTML,/>Disconnected</);assert.match(c.status.innerHTML,/data-terminal-id="session-one"/);assert.match(c.status.innerHTML,/>Reconnect</);
 for(const type of ['ssh','telnet','serial'])assert.match(c.status.innerHTML,new RegExp('data-terminal-new-type="'+type+'" data-terminal-new-pane="2"'));
 assert.match(c.status.innerHTML,/View output/);const term=c.entry.xterm;c.setTerminalConnectionScreen(c.entry,true);c.renderTerminalConnectionState(pane,c.entry,false);assert.equal(c.status.classList.hidden,true);assert.equal(c.outputBar.classList.hidden,false);assert.match(c.outputBar.innerHTML,/Connection options/);assert.equal(c.entry.xterm,term);c.setTerminalConnectionScreen(c.entry,false);c.renderTerminalConnectionState(pane,c.entry,false);assert.equal(c.status.classList.hidden,false);
 c.entry.info.state='connected';c.renderTerminalConnectionState(pane,c.entry,false);assert.equal(c.status.classList.hidden,true);assert.equal(c.outputBar.classList.hidden,true);c.entry.info.state='closed';c.renderTerminalConnectionState(pane,c.entry,true);assert.equal(c.status.classList.hidden,true);assert.equal(c.outputBar.classList.hidden,true);
});
test('Delayed input acknowledgements from the old connection cannot corrupt the new input queue',async()=>{
 const c=setup();c.entry.info.state='connected';c.entry.inputError='';let acceptInput;
 c.api=async(url,...args)=>{c.requests.push([url,...args]);if(url.endsWith('/input'))return new Promise(r=>acceptInput=r);return {session:{...c.entry.info,state:'connecting'},viewState:{revision:1}};};
 const sending=c.drainTerminalInput(c.entry);await c.reconnectTerminal(c.entry);acceptInput({ok:true});await sending;assert.equal(c.entry.inputBytes,0);assert.equal(c.entry.inputChunks.length,0);assert.equal(c.entry.inputError,'');assert.equal(c.entry.inputSending,false);
});
test('Ctrl+C and standard control keys are terminal input and suppress browser actions',async()=>{
 const c=setup(),sent=[];let copied=0;c.preferences=()=>({ctrlC:'send'});c.terminalPaneShortcut=()=>false;c.enqueueTerminalInput=(entry,data)=>sent.push([entry.info.id,data]);c.noteInput=()=>{};c.copy=async()=>{copied++;return true;};
 const start=interaction.indexOf('  function keyHandler('),end=interaction.indexOf('\n  function settingsHTML(',start);vm.runInContext(interaction.slice(start,end),c);
 for(const [key,code] of [['c',3],['d',4],['w',23],['z',26],['l',12],['r',18],['t',20],['q',17],['[',27]]){
  const event={type:'keydown',key,ctrlKey:true,metaKey:false,altKey:false,shiftKey:false,preventDefault(){this.prevented=true;},stopPropagation(){this.stopped=true;}};
  assert.equal(c.keyHandler(c.entry,event),false);assert(event.prevented&&event.stopped);assert.equal(sent.at(-1)[1].charCodeAt(0),code);assert.equal(c.terminalSessions.size,1);
 }
 const event={type:'keydown',key:'C',ctrlKey:true,shiftKey:true,preventDefault(){},stopPropagation(){}};c.keyHandler(c.entry,event);await Promise.resolve();assert.equal(copied,1);assert.equal(c.entry.xterm.cleared,true);assert.equal(sent.length,9);
});
test('Reconnect resets terminal modes without clearing retained console output',async()=>{
 global.self=global;const {Terminal}=require('../web/assets/xterm.js');const term=new Terminal({allowProposedApi:true,cols:80,rows:12});const write=data=>new Promise(resolve=>term.write(data,resolve));
 await write('RETAIN THIS OUTPUT\r\n\x1b[?1049h\x1b[?1000h\x1b[?2004h');
 const manager=fs.readFileSync(path.join(__dirname,'../internal/terminal/manager.go'),'utf8');const literal=manager.match(/Data: \[\]byte\(("[^\n]+Reconnecting using previous settings\.[^\n]+")\)/)[1];await write(vm.runInNewContext(literal));assert.equal(term.buffer.active,term.buffer.normal);assert.equal(term.modes.mouseTrackingMode,'none');assert.equal(term.modes.bracketedPasteMode,false);assert.match(Array.from({length:term.buffer.normal.length},(_,n)=>term.buffer.normal.getLine(n).translateToString(true)).join('\n'),/RETAIN THIS OUTPUT/);term.dispose();
});
test('Reconnect appends below the last entry without restoring an old saved cursor or overwriting history',async()=>{
 global.self=global;const {Terminal}=require('../web/assets/xterm.js');
 const manager=fs.readFileSync(path.join(__dirname,'../internal/terminal/manager.go'),'utf8'),literal=manager.match(/Data: \[\]byte\(("[^\n]+Reconnecting using previous settings\.[^\n]+")\)/)[1],reconnect=vm.runInNewContext(literal);
 for(const count of [3,120])for(const alternate of [false,true]){
  const term=new Terminal({allowProposedApi:true,cols:80,rows:12,scrollback:1000}),write=data=>new Promise(resolve=>term.write(data,resolve));
  await write('first line\r\n\x1b7'+Array.from({length:count},(_,i)=>'previous output '+i+'\r\n').join('')+'router# unfinished input');
  const b=term.buffer.normal,last=b.baseY+b.cursorY,old=Array.from({length:last+1},(_,i)=>b.getLine(i).translateToString(true));
  if(alternate)await write('\x1b[?1049h\x1b[H\x1b[?1000h\x1b[?2004hFULL SCREEN');
  term.scrollToTop();await write(reconnect);await write('router# show version\r\nNEW VERSION OUTPUT\r\nrouter# ');
  const buffer=term.buffer.normal,lines=Array.from({length:buffer.length},(_,i)=>buffer.getLine(i).translateToString(true));
  assert.deepEqual(lines.slice(0,last+1),old,'reconnect overwrote previous console output');
  assert.equal(lines[last+1],'[Network Toolkit: Reconnecting using previous settings.]');
  assert.equal(lines[last+2],'router# show version');assert.equal(lines[last+3],'NEW VERSION OUTPUT');assert.equal(lines[last+4],'router# ');
  assert.equal(buffer.baseY+buffer.cursorY,last+4);assert.equal(term.buffer.active,buffer);assert.equal(term.modes.mouseTrackingMode,'none');assert.equal(term.modes.bracketedPasteMode,false);
  // Repeated reconnects append after the most recent command as well.
  const again=buffer.baseY+buffer.cursorY;await write(reconnect+'router# ');assert.equal(buffer.getLine(again).translateToString(true),'router# ');assert.equal(buffer.getLine(again+1).translateToString(true),'[Network Toolkit: Reconnecting using previous settings.]');assert.equal(buffer.getLine(again+2).translateToString(true),'router# ');term.dispose();
 }
});

test('manual reconnect requests fresh authentication using only safe connection fields',async()=>{
 const c=setup(),forms=[];c.entry.config.password='must-not-survive';c.entry.config.privateKey='must-not-survive';c.openTerminalConnect=(...args)=>forms.push(args);
 c.api=async()=>{const error=Error('Authentication required');error.status=409;error.code='authentication_required';throw error;};
 await c.reconnectTerminal(c.entry);assert.equal(forms.length,1);assert.equal(forms[0][0].host,'192.0.2.10');assert(!JSON.stringify(forms).includes('must-not-survive'));assert.equal(c.entry.reconnectBusy,false);assert.match(c.entry.reconnectError,/Sign in again/);
});
test('late reauthentication requirement does not replace a newer connection dialog',async()=>{
 const c=setup();c.document.querySelector=()=>({});c.api=async()=>{throw Object.assign(Error('Sign in'),{code:'authentication_required',status:409});};await c.reconnectTerminal(c.entry);assert.equal(c.entry.reconnectBusy,false);
});
