'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
global.self=global;const {Terminal}=require('../web/assets/xterm.js');
const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8'),interaction=fs.readFileSync(path.join(__dirname,'../web/assets/terminal-interaction.js'),'utf8');
function appFunction(name){const tail=html.slice(html.search(new RegExp('^function '+name+'\\(','m')));return tail.slice(0,tail.slice(1).search(/\n(?:async )?function /)+1);}
function interactionFunction(name){const tail=interaction.slice(interaction.indexOf('  function '+name+'('));return tail.slice(0,tail.slice(1).search(/\n  (?:async )?function /)+1);}
function harness(cols=40){
 let selected=null,current,rev=0,reviews=[],copied=0;
 const listeners=new Map(),screen={getBoundingClientRect:()=>({left:0,top:0,right:term.cols*10,bottom:120,width:term.cols*10,height:120})};
 const element={querySelector:()=>screen,addEventListener:(type,fn)=>listeners.set(type,fn),removeEventListener(){}};
 const term=new Terminal({cols,rows:6,allowProposedApi:true,scrollback:100});term.focus=()=>{};term.getSelectionPosition=()=>selected;term.hasSelection=()=>!!selected;term.clearSelection=()=>{selected=null;};
 const entry={info:{id:'one',type:'ssh',state:'connected'},config:{},element,xterm:term,disposables:[],inputChunks:[],inputBytes:0};current=entry;
 const window={NetworkToolkitTerminalCopy:{suppressesInput:()=>false},NetworkToolkitTerminalInteraction:{suppressesInput:()=>false,noteInput:()=>{},cancelSelectionCopy:()=>{copied++;}},NetworkToolkitTerminalExtras:{notePaste:()=>{},prefs:()=>({pasteReview:true})}};
 const bound=new Map([[entry,{xterm:term,nativePaste:text=>term.input(text,true)}]]);
 const c={window,root:window,TextEncoder,TextDecoder,terminalSessionRevision:()=>rev,terminalEntryAcceptsInput:e=>e===current&&e.info.state==='connected',NetworkToolkitTerminalInteraction:window.NetworkToolkitTerminalInteraction,toast:()=>{},setTimeout:()=>0,clearTimeout(){},bound,validDestination:t=>t.entry===current,byteLength:t=>new TextEncoder().encode(t).length,MAX_PASTE_BYTES:1024*1024,approvedPaste:false,pasteStarts:new Map(),review:null,openReview:(token,text)=>reviews.push({token,text}),changedDestination:()=>{},terminalPaneShortcut:()=>false,preferences:()=>({ctrlC:'send'})};
 vm.createContext(c);vm.runInContext(appFunction('enqueueTerminalInput'),c);vm.runInContext(fs.readFileSync(path.join(__dirname,'../web/assets/terminal-input.js'),'utf8'),c);for(const name of ['keyHandler','sendPaste','acceptText'])vm.runInContext(interactionFunction(name),c);
 const api=window.NetworkToolkitTerminalInput;api.bind(entry);term.onData(data=>c.enqueueTerminalInput(entry,data));
 async function write(text){await new Promise(resolve=>term.write(text,resolve));}
 function drain(){const data=Buffer.concat(entry.inputChunks.map(b=>Buffer.from(b))).toString();entry.inputChunks=[];entry.inputBytes=0;return data;}
 async function type(text){term.input(text,true);const data=drain();await write(text);return data;}
 function event(x,y=0){return {clientX:x*10,clientY:y*20+5,button:0,detail:1,preventDefault(){this.prevented=true;},stopImmediatePropagation(){this.stopped=true;}};}
 function key(name){return c.keyHandler(entry,{type:'keydown',key:name,preventDefault(){},stopPropagation(){}});}
 return {api,c,entry,term,write,type,drain,event,key,listeners,reviews,select:(x1,y1,x2,y2)=>{selected={start:{x:x1,y:y1},end:{x:x2,y:y2}};},revision:()=>rev++,focus:e=>current=e,get selected(){return selected},get copied(){return copied},dispose(){for(const d of entry.disposables)d.dispose();term.dispose();}};
}
test('a click positions the current command cursor without Enter and keeps drag selection',async()=>{
 const h=harness();await h.write('switch# ');await h.type('show version');const click=h.event(13);h.listeners.get('pointerdown')(click);h.listeners.get('click')(click);assert.equal(h.drain(),'\x1b[D'.repeat(7));assert(click.prevented&&click.stopped);
 h.listeners.get('pointerdown')(h.event(10));h.listeners.get('click')(h.event(17));assert.equal(h.drain(),'');h.dispose();
});
test('Backspace or Delete removes an input selection using the current device cursor',async()=>{
 for(const key of ['Backspace','Delete']){const h=harness();await h.write('switch# ');await h.type('show version');h.select(13,0,20,0);assert.equal(h.key(key),false);assert.equal(h.drain(),'\x7f'.repeat(7));assert.equal(h.selected,null);assert.equal(h.copied,1);h.dispose();}
 const h=harness();await h.write('switch# ');await h.type('show version');h.select(8,0,12,0);h.key('Delete');assert.equal(h.drain(),'\x1b[D'.repeat(8)+'\x7f'.repeat(4));h.dispose();
});
test('Backspace and Delete ignore empty cells selected after the current command',async()=>{
 for(const key of ['Backspace','Delete'])for(const left of [0,3]){
  const h=harness();await h.write('switch# ');await h.type('show version');if(left)await h.write('\x1b['+left+'D');h.select(13,0,35,0);
  assert.equal(h.key(key),false);assert.equal(h.drain(),'\x1b[C'.repeat(left)+'\x7f'.repeat(7));assert.equal(h.selected,null);assert.equal(h.copied,1);h.dispose();
 }
});
test('wrapped selections can end in padding or at the next row boundary',async()=>{
 for(const [x,y] of [[18,1],[0,2]]){
  const h=harness(20);await h.write('sw# ');await h.type('show interfaces description');h.select(14,0,x,y);
  assert.equal(h.api.deleteSelection(h.entry),true);assert.equal(h.drain(),'\x7f'.repeat(17));assert.equal(h.selected,null);h.dispose();
 }
});
test('trailing spaces before the device cursor are included but empty padding adds no deletes',async()=>{
 const h=harness();await h.write('sw# ');await h.type('show version   ');h.select(9,0,35,0);
 assert.equal(h.api.deleteSelection(h.entry),true);assert.equal(h.drain(),'\x7f'.repeat(10));assert.equal(h.selected,null);h.dispose();
});
test('space cells left by a device backspace echo are not treated as extra command characters',async()=>{
 const h=harness();await h.write('sw# ');await h.type('show versionx');await h.write('\b \b');h.select(9,0,35,0);
 assert.equal(h.api.deleteSelection(h.entry),true);assert.equal(h.drain(),'\x7f'.repeat(7));assert.equal(h.selected,null);h.dispose();
});
test('typing and accepted paste replace text selected with trailing empty cells',async()=>{
 for(const paste of [false,true]){
  const h=harness();await h.write('sw# ');await h.type('show version');h.select(9,0,35,0);
  if(paste)assert.equal(h.c.acceptText({entry:h.entry,xterm:h.term},'other'),true);
  else{assert.equal(h.key('x'),true);h.term.input('x',true);}
  assert.equal(h.drain(),'\x7f'.repeat(7)+(paste?'other':'x'));assert.equal(h.selected,null);h.dispose();
 }
});
test('padding edits reject prompt, previous output, empty-only and later hard-line selections',async()=>{
 const h=harness();await h.write('OLDER OUTPUT\r\nswitch# ');await h.type('show version');
 for(const selection of [[0,0,35,1],[0,1,35,1],[21,1,35,1],[13,1,5,2]]){
  h.select(...selection);assert.equal(h.api.deleteSelection(h.entry),false);assert.equal(h.drain(),'');assert(h.selected);
 }
 h.select(13,1,35,1);h.entry.inputSending=true;assert.equal(h.api.deleteSelection(h.entry),false);assert.equal(h.drain(),'');assert(h.selected);h.dispose();
});
test('Backspace and Delete accept preceding prompt spaces and trailing padding together',async()=>{
 for(const key of ['Backspace','Delete'])for(const left of [0,3]){
  const h=harness();await h.write('switch#   ');await h.type('show version');if(left)await h.write('\x1b['+left+'D');h.select(7,0,35,0);
  assert.equal(h.key(key),false);assert.equal(h.drain(),'\x1b[C'.repeat(left)+'\x7f'.repeat(12));assert.equal(h.selected,null);assert.equal(h.copied,1);h.dispose();
 }
});
test('preceding prompt whitespace does not add deletes when only a command prefix is selected',async()=>{
 const h=harness();await h.write('switch# ');await h.type('show version');h.select(7,0,12,0);
 assert.equal(h.api.deleteSelection(h.entry),true);assert.equal(h.drain(),'\x1b[D'.repeat(8)+'\x7f'.repeat(4));assert.equal(h.selected,null);h.dispose();
});
test('actual leading input spaces are deleted while selected prompt spacing is excluded',async()=>{
 for(const start of [7,8]){
  const h=harness();await h.write('switch# ');await h.type('  show version');h.select(start,0,35,0);
  assert.equal(h.api.deleteSelection(h.entry),true);assert.equal(h.drain(),'\x7f'.repeat(14));assert.equal(h.selected,null);h.dispose();
 }
});
test('leading whitespace normalization follows a wrapped prompt and wrapped command',async()=>{
 const h=harness(20);await h.write('abcdefghijklmnopqrstuvwx#  ');await h.type('show interfaces description');h.select(5,1,0,3);
 assert.equal(h.api.deleteSelection(h.entry),true);assert.equal(h.drain(),'\x7f'.repeat(27));assert.equal(h.selected,null);h.dispose();
});
test('preceding whitespace preserves wide and combining character deletion counts',async()=>{
 const h=harness();await h.write('sw# ');await h.type('a界e\u0301z');h.select(3,0,30,0);
 assert.equal(h.api.deleteSelection(h.entry),true);assert.equal(h.drain(),'\x7f'.repeat(4));assert.equal(h.selected,null);h.dispose();
});
test('typing and approved paste replace selections that start in preceding prompt whitespace',async()=>{
 for(const paste of [false,true]){
  const h=harness();await h.write('switch# ');await h.type('show version');h.select(7,0,35,0);
  if(paste)assert.equal(h.c.acceptText({entry:h.entry,xterm:h.term},'other'),true);
  else{assert.equal(h.key('x'),true);h.term.input('x',true);}
  assert.equal(h.drain(),'\x7f'.repeat(12)+(paste?'other':'x'));assert.equal(h.selected,null);h.dispose();
 }
});
test('leading padding rejects prompt characters, prior blank lines, glyph continuations and hidden cells',async()=>{
 const h=harness();await h.write('       \r\nswitch# ');await h.type('show version');
 for(const selection of [[7,0,25,1],[6,1,25,1],[0,1,25,1],[7,1,8,1]]){
  h.select(...selection);assert.equal(h.api.deleteSelection(h.entry),false);assert.equal(h.drain(),'');assert(h.selected);
 }
 h.select(7,1,35,1);h.entry.inputSending=true;assert.equal(h.api.deleteSelection(h.entry),false);assert.equal(h.drain(),'');assert(h.selected);h.dispose();
 const wide=harness();await wide.write('sw界 ');await wide.type('abc');wide.select(3,0,20,0);assert.equal(wide.api.deleteSelection(wide.entry),false);assert.equal(wide.drain(),'');wide.dispose();
 const hidden=harness();await hidden.write('switch#\x1b[8m \x1b[0m');await hidden.type('abc');hidden.select(7,0,20,0);assert.equal(hidden.api.deleteSelection(hidden.entry),false);assert.equal(hidden.drain(),'');hidden.dispose();
});
test('wrapped commands permit click positioning and deletion across screen rows',async()=>{
 const h=harness(20);await h.write('sw# ');await h.type('show interfaces description');assert.equal(h.api.move(h.entry,h.event(7)),true);assert.equal(h.drain(),'\x1b[D'.repeat(24));h.select(14,0,8,1);assert.equal(h.api.deleteSelection(h.entry),true);assert.equal(h.drain(),'\x1b[D'.repeat(3)+'\x7f'.repeat(14));h.dispose();
});
test('wide and combining characters count as input characters rather than screen cells',async()=>{
 const h=harness();await h.write('sw# ');await h.type('a界e\u0301z');assert.equal(h.api.move(h.entry,h.event(5)),true);assert.equal(h.drain(),'\x1b[D'.repeat(3));h.select(5,0,9,0);assert.equal(h.api.deleteSelection(h.entry),true);assert.equal(h.drain(),'\x7f'.repeat(3));h.dispose();
});
test('prompt/output selections, hidden passwords and unavailable sessions cannot send edits',async()=>{
 const h=harness();await h.write('OLDER OUTPUT\r\nswitch# ');await h.type('show version');h.select(0,0,4,0);assert.equal(h.api.deleteSelection(h.entry),false);assert.equal(h.api.move(h.entry,h.event(3,1)),false);assert.equal(h.drain(),'');
 h.entry.info.state='closed';h.select(8,1,12,1);assert.equal(h.api.deleteSelection(h.entry),false);h.entry.info.state='connected';h.revision();assert.equal(h.api.move(h.entry,h.event(10,1)),false);assert.equal(h.drain(),'');h.dispose();
 const hidden=harness();await hidden.write('Password: ');hidden.term.input('secret',true);hidden.drain();assert.equal(hidden.api.move(hidden.entry,hidden.event(12)),false);assert(!JSON.stringify(hidden.api.snapshot(hidden.entry)).includes('secret'));hidden.dispose();
 const invisible=harness();await invisible.write('Password: ');invisible.term.input('secret',true);invisible.drain();await invisible.write('\x1b[8msecret');assert.equal(invisible.api.move(invisible.entry,invisible.event(12)),false);invisible.dispose();
});
test('native mouse reporting and alternate-screen applications retain their own input handling',async()=>{
 for(const mode of ['\x1b[?1000h','\x1b[?1049h']){const h=harness();await h.write('sw# ');await h.type('show version');await h.write(mode);assert.equal(h.api.move(h.entry,h.event(7)),false);assert.equal(h.drain(),'');h.dispose();}
});
test('queued input prevents speculative editing and accepted edits remain session-specific',async()=>{
 const h=harness();await h.write('sw# ');await h.type('show version');h.entry.inputSending=true;assert.equal(h.api.move(h.entry,h.event(7)),false);h.entry.inputSending=false;h.focus({info:{id:'other',state:'connected'}});assert.equal(h.api.move(h.entry,h.event(7)),false);assert.equal(h.drain(),'');h.dispose();
});
test('click editing survives normal buffer reflow and respects application cursor keys',async()=>{
 const h=harness();await h.write('sw# ');await h.type('show interfaces description');h.term.resize(20,6);await h.write('\r\x1b[2Ksw# show interfaces description');assert.equal(h.api.move(h.entry,h.event(7)),true);assert.equal(h.drain(),'\x1b[D'.repeat(24));await h.write('\x1b[?1h');assert.equal(h.api.move(h.entry,h.event(7)),true);assert.equal(h.drain(),'\x1bOD'.repeat(24));h.dispose();
});
test('typing replaces selected input while multiline paste cancellation sends nothing',async()=>{
 const h=harness();await h.write('sw# ');await h.type('show version');h.select(9,0,16,0);assert.equal(h.key('x'),true);h.term.input('x',true);assert.equal(h.drain(),'\x7f'.repeat(7)+'x');h.dispose();
 const p=harness();await p.write('sw# ');await p.type('show version');p.select(9,0,16,0);const token={entry:p.entry,xterm:p.term};assert.equal(p.c.acceptText(token,'show one\nshow two'),false);assert.equal(p.reviews.length,1);assert.equal(p.drain(),'');assert(p.selected);
 p.c.review={form:{isConnected:true},destination:token};p.c.window.NetworkToolkitTerminalInteraction.suppressesInput=()=>!!p.c.review&&!p.c.approvedPaste;
 assert.equal(p.c.sendPaste(token,'other'),true);assert.equal(p.drain(),'\x7f'.repeat(7)+'other');assert.equal(p.selected,null);assert.equal(p.c.approvedPaste,false);p.dispose();
});
test('input position transfers without keystrokes and Enter or Clear ends editing',async()=>{
 const h=harness();await h.write('sw# ');await h.type('show secret');const snapshot=JSON.parse(JSON.stringify(h.api.snapshot(h.entry)));assert.deepEqual(snapshot,{line:0,offset:4});assert(!JSON.stringify(snapshot).includes('secret'));const other=harness();await other.write('sw# show secret');other.api.restore(other.entry,snapshot);assert.equal(other.api.move(other.entry,other.event(7)),true);assert.equal(other.drain(),'\x1b[D'.repeat(8));other.dispose();h.term.input('\r',true);h.drain();assert.equal(h.api.move(h.entry,h.event(7)),false);h.api.restore(h.entry,snapshot);h.api.clear(h.entry);assert.equal(h.api.move(h.entry,h.event(7)),false);h.dispose();
});

test('Ctrl+A selects the current echoed input only and sends no control byte',async()=>{
 for(const [prompt,cols,command,column,row] of [
  ['switch# ',40,'show version',8,0],
  ['sw# ',20,'show interfaces description',4,0],
  ['abcdefghijklmnopqrstuvwx#  ',20,'show interfaces description',7,1]
 ]){
  const h=harness(cols),calls=[];await h.write(prompt);await h.type(command);
  h.term.select=(x,y,length)=>{calls.push({x,y,length});h.select(x,y,(x+length)%cols,y+Math.floor((x+length)/cols));};
  const e={type:'keydown',key:'a',ctrlKey:true,shiftKey:false,altKey:false,metaKey:false,preventDefault(){this.prevented=true;},stopPropagation(){this.stopped=true;}};
  assert.equal(h.c.keyHandler(h.entry,e),false);
  assert.deepEqual(calls,[{x:column,y:row,length:command.length}]);
  assert.deepEqual(h.selected.start,{x:column,y:row});
  assert.equal(h.drain(),'');
  assert(e.prevented&&e.stopped);
  assert.equal(h.copied,1,'programmatic selection must cancel an old automatic copy');
  h.dispose();
 }
});
test('Ctrl+A leaves unsafe, unknown and full-screen terminal input to the device',async()=>{
 for(const scenario of ['no-input','pending-input','alternate-screen','input-suppressed','closed']){
  const h=harness(),calls=[];
  h.term.select=(...args)=>calls.push(args);
  await h.write('switch# ');
  if(scenario!=='no-input')await h.type('show version');
  if(scenario==='pending-input')h.entry.inputBytes=1;
  if(scenario==='alternate-screen')await h.write('\x1b[?1049h');
  if(scenario==='input-suppressed')h.c.window.NetworkToolkitTerminalInteraction.suppressesInput=()=>true;
  if(scenario==='closed')h.entry.info.state='closed';
  const e={type:'keydown',key:'a',ctrlKey:true,shiftKey:false,altKey:false,metaKey:false,preventDefault(){this.prevented=true;},stopPropagation(){this.stopped=true;}};
  assert.equal(h.c.keyHandler(h.entry,e),false);
  assert.deepEqual(calls,[],scenario);
  assert.equal(h.drain(),['input-suppressed','closed'].includes(scenario)?'':'\x01',scenario);
  h.dispose();
 }
});
test('Ctrl+Shift+A preserves the existing control-key behavior',async()=>{
 const h=harness();await h.write('switch# ');await h.type('show version');
 let selected=false;h.term.select=()=>{selected=true;};
 const e={type:'keydown',key:'A',ctrlKey:true,shiftKey:true,altKey:false,metaKey:false,preventDefault(){},stopPropagation(){}};
 assert.equal(h.c.keyHandler(h.entry,e),false);
 assert.equal(selected,false);
 assert.equal(h.drain(),'\x01');
 h.dispose();
});
