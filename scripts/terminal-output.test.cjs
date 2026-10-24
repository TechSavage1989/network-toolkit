'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
global.self=global;
const {Terminal}=require('../web/assets/xterm.js');
const source=fs.readFileSync(path.join(__dirname,'../web/assets/terminal-extras.js'),'utf8');
function harness(){
 let active,clipboard='',messages=[],sent=[],modals=[];
 const window={NetworkToolkitTerminalInteraction:{preferences:()=>({autoPager:false,pagerKey:'auto',pasteReview:true,pasteIncludeCommands:false}),copyValue:async text=>{clipboard=text;return true;},suppressesInput:()=>false}};
 const context={window,document:{querySelector:()=>null},TextDecoder,TextEncoder,Date,Math,toast:text=>messages.push(text),terminalEntryAcceptsInput:e=>e.info.state==='connected',currentTerminal:()=>active,enqueueTerminalInput:(e,data)=>sent.push([e.info.id,data]),openModal:(...args)=>modals.push(args),esc:String,icon:()=>'',console};
 vm.runInNewContext(source,context);const api=window.NetworkToolkitTerminalExtras;
 function entry(id='one',cols=80,rows=12,scrollback=10000){const e={info:{id,state:'connected'},disposables:[],xterm:new Terminal({cols,rows,scrollback,allowProposedApi:true}),inputBytes:0};e.xterm.selectLines=(start,end)=>{e.selection={start,end};};e.xterm.onData(data=>api.noteManualInput(e,data));active=e;return e;}
 async function write(e,text){const pos=api.beforeOutput(e);api.observeOutput(e,text);await new Promise(resolve=>e.xterm.write(text,resolve));api.afterOutput(e,pos);}
 async function command(e,text,output){e.xterm.input(text,true);await write(e,text);e.xterm.input('\r',true);await write(e,'\r\n'+output+'\r\nswitch# ');}
 return {api,entry,write,command,active:e=>active=e,text:(e,include=false,choice)=>api.output(e,api.current(e,choice),include)?.text,get clipboard(){return clipboard},messages,sent,modals};
}
test('typed commands copy complete output and optional command echo',async()=>{
 const h=harness(),e=h.entry();await h.write(e,'switch# ');const output=Array.from({length:260},(_,i)=>'configuration '+i).join('\r\n');await h.command(e,'show running-config',output);assert.equal(h.text(e),output.replaceAll('\r',''));assert.equal(e._ntOutputHistory.length,1);assert.equal(e._ntOutputHistory[0].kind,'command');await h.api.copy(e);assert.equal(h.clipboard,output.replaceAll('\r',''));assert.match(h.text(e,true),/^switch# show running-config\nconfiguration 0/);e.xterm.dispose();
});
test('next command bounds previous output before the next echo',async()=>{
 const h=harness(),e=h.entry();await h.write(e,'switch# ');await h.command(e,'show one','FIRST');const id=e._ntOutputHistory[0].id;await h.command(e,'show two','SECOND');assert.equal(h.text(e,false,id),'FIRST');assert.equal(h.text(e),'SECOND');assert.equal(h.text(e,true,id),'switch# show one\nFIRST');e.xterm.dispose();
});
test('paste commands interleaved with output are excluded in sequence',async()=>{
 const h=harness(),e=h.entry();await h.write(e,'switch# ');h.api.notePaste(e,'show one\nshow two\n');await h.write(e,'show one\r\nFIRST\r\nswitch# show two\r\nSECOND\r\noutput mentions show one\r\nswitch# ');assert.equal(h.text(e),'FIRST\nSECOND\noutput mentions show one');assert.equal(h.text(e,true),'switch# show one\nFIRST\nswitch# show two\nSECOND\noutput mentions show one');e.xterm.dispose();
});
test('single line paste and Enter stay associated with the same paste',async()=>{
 const h=harness(),e=h.entry();await h.write(e,'switch# ');h.api.notePaste(e,'show version');await h.write(e,'show version');e.xterm.input('\r',true);await h.write(e,'\r\nVERSION 1\r\nswitch# ');assert.equal(e._ntOutputHistory.length,1);assert.equal(e._ntOutputHistory[0].kind,'paste');assert.equal(h.text(e),'VERSION 1');e.xterm.dispose();
});
test('wrapped typed commands and trailing spaces in wrapped output are preserved',async()=>{
 const h=harness(),e=h.entry('narrow',30);await h.write(e,'switch# ');const cmd='show configuration '+'interface-description '.repeat(5),out='A '.repeat(90)+'END';await h.command(e,cmd,out);assert.equal(h.text(e),out);assert.match(h.text(e,true),/^switch# show configuration/);e.xterm.dispose();
});
test('manual and automatic pager responses do not create command blocks',async()=>{
 const h=harness(),e=h.entry();await h.write(e,'switch# ');await h.command(e,'show long','FIRST');e._ntSessionPrefs={autoPager:true};await h.write(e,'\r\n--More--');assert.equal(h.sent.length,1);assert.equal(h.sent[0][1],' ');e._ntSessionPrefs.autoPager=false;await h.write(e,'\r\nSECOND\r\n--More--');e.xterm.input(' ',true);assert.equal(e._ntOutputHistory.length,1);e._ntSessionPrefs={autoPager:true,pagerKey:'enter'};await h.write(e,'\r\nTHIRD\r\nPress Enter to continue');assert.equal(h.sent.at(-1)[1],'\r');e.xterm.dispose();
});
test('different sessions retain isolated output and pager settings',async()=>{
 const h=harness(),a=h.entry('A');await h.write(a,'switch# ');await h.command(a,'show a','OUTPUT A');a._ntSessionPrefs={autoPager:true,pasteReview:false};const b=h.entry('B');await h.write(b,'switch# ');await h.command(b,'show b','OUTPUT B');assert.equal(h.text(a),'OUTPUT A');assert.equal(h.text(b),'OUTPUT B');assert.equal(h.api.prefs(a).autoPager,true);assert.equal(h.api.prefs(b).autoPager,false);assert.equal(h.api.prefs(a).pasteReview,false);assert.equal(h.api.prefs(b).pasteReview,true);a.xterm.dispose();b.xterm.dispose();
});
test('transferred history contains positions and session settings without raw input',async()=>{
 const h=harness(),a=h.entry('A');await h.write(a,'switch# ');await h.command(a,'show transfer','RETAIN THIS');a._ntSessionPrefs={autoPager:true,pasteReview:false};const value=JSON.parse(JSON.stringify(h.api.snapshot(a)));assert(!JSON.stringify(value).includes('show transfer'));assert(!JSON.stringify(value).includes('RETAIN THIS'));const b=h.entry('B');await h.write(b,'switch# show transfer\r\nRETAIN THIS\r\nswitch# ');h.api.restore(b,value);assert.equal(h.text(b),'RETAIN THIS');assert.equal(h.api.prefs(b).autoPager,true);assert.equal(h.api.prefs(b).pasteReview,false);a.xterm.dispose();b.xterm.dispose();
});
test('trimmed history cannot copy unrelated retained lines',async()=>{
 const h=harness(),e=h.entry('trim',80,5,10);await h.write(e,'switch# ');await h.command(e,'show old','OLD');await h.write(e,Array.from({length:80},(_,i)=>'new line '+i).join('\r\n'));assert.equal(h.text(e),undefined);assert.equal(await h.api.copy(e),false);assert.equal(h.clipboard,'');e.xterm.dispose();
});
test('history is bounded and clear removes all block positions',async()=>{
 const h=harness(),e=h.entry();await h.write(e,'switch# ');for(let i=0;i<55;i++)await h.command(e,'show '+i,'OUTPUT '+i);assert.equal(e._ntOutputHistory.length,50);assert.equal(h.text(e),'OUTPUT 54');h.api.clear(e);assert.equal(e._ntOutputHistory.length,0);assert.equal(h.api.current(e),null);e.xterm.dispose();
});
test('jumping to earlier output pauses follow without affecting other sessions',async()=>{
 const h=harness(),e=h.entry('A'),other=h.entry('B');await h.write(e,'switch# ');await h.command(e,'show all',Array.from({length:100},(_,i)=>'ROW '+i).join('\r\n'));assert.equal(e.xterm.buffer.normal.viewportY,e.xterm.buffer.normal.baseY);h.api.jump(e);assert.equal(h.api.prefs(e).autoScroll,false);const at=e.xterm.buffer.normal.viewportY;await h.write(e,'MORE OUTPUT\r\n');assert.equal(e.xterm.buffer.normal.viewportY,at);assert.equal(h.api.prefs(other).autoScroll,true);e.xterm.dispose();other.xterm.dispose();
});
test('alternate screen and terminal replies are not tracked as commands',async()=>{
 const h=harness(),e=h.entry();await h.write(e,'switch# ');e.xterm.input('\x1b[I',true);e.xterm.input('\x1b[O',true);e.xterm.input('\x1b[1;1R',true);assert.equal(e._ntOutputHistory,undefined);await h.write(e,'\x1b[?1049h');e.xterm.input('editor input\r',true);assert.equal(e._ntOutputHistory,undefined);e.xterm.dispose();
});

test('a command echo arriving after Enter still excludes every wrapped part',async()=>{
 const h=harness(),e=h.entry('delayed',30);await h.write(e,'switch# ');const cmd='show configuration '+ 'long-section '.repeat(6);e.xterm.input(cmd,true);e.xterm.input('\r',true);await h.write(e,cmd+'\r\nLATE ECHO OUTPUT\r\nswitch# ');assert.equal(h.text(e),'LATE ECHO OUTPUT');e.xterm.dispose();
});
test('editing a pasted command keeps its complete displayed input optional',async()=>{
 const h=harness(),e=h.entry();await h.write(e,'switch# ');e.xterm.input('show ',true);await h.write(e,'show ');h.api.notePaste(e,'version');await h.write(e,'version');e.xterm.input(' detail',true);await h.write(e,' detail');e.xterm.input('\r',true);await h.write(e,'\r\nDETAILED VERSION\r\nswitch# ');assert.equal(h.text(e),'DETAILED VERSION');assert.equal(h.text(e,true),'switch# show version detail\nDETAILED VERSION');e.xterm.dispose();
});
