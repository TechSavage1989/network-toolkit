'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
global.self=global;
const {Terminal}=require('../web/assets/xterm.js');
const extras=fs.readFileSync(path.join(__dirname,'../web/assets/terminal-extras.js'),'utf8');
const macros=fs.readFileSync(path.join(__dirname,'../web/assets/terminal-macros.js'),'utf8');
const tick=async()=>{for(let i=0;i<12;i++)await Promise.resolve();};
function harness(){
 let active=null,clipboard='';
 const window={NetworkToolkitTerminalInteraction:{preferences:()=>({autoPager:false,pasteIncludeCommands:false}),copyValue:async text=>{clipboard=text;return true;}}};
 const ctx={window,document:{querySelector:()=>null},TextDecoder,TextEncoder,setTimeout,clearTimeout,Date,Math,toast(){},terminalEntryAcceptsInput:e=>e.info.state==='connected',currentTerminal:()=>active,openModal(){},icon(){return '';},console};
 vm.runInNewContext(extras,ctx);
 const api=window.NetworkToolkitTerminalExtras;
 function entry(id='one'){
  const e={info:{id,state:'connected'},disposables:[],xterm:new Terminal({cols:100,rows:12,allowProposedApi:true}),inputBytes:0};
  active=e;return e;
 }
 async function write(e,text){api.observeOutput(e,text);await new Promise(resolve=>e.xterm.write(text,resolve));}
 return {api,entry,write,get clipboard(){return clipboard}};
}
test('paced Last paste shows every accepted command output and Last command shows only final command',async()=>{
 const h=harness(),e=h.entry();try{
  await h.write(e,'switch# ');
  const block=h.api.notePaste(e,'show one\n\nshow two\n');
  assert(block&&block.kind==='paste');block.pacedRunId='paced-1';
  const capture={runId:'paced-1',sessionId:'one',state:'completed',steps:[
   {index:1,command:'show one',output:'switch# show one\r\nFIRST\r\nswitch# '},
   {index:2,command:'',output:'\r\nswitch# '},
   {index:3,command:'show two',output:'switch# show two\r\nSECOND\r\nswitch# '}
  ]};
  assert.equal(h.api.completePacedPaste(e,block,capture),true);
  assert.equal(h.api.completePacedPaste(e,block,capture),false);
  assert.equal(h.api.output(e,h.api.current(e,'last-paste'),false).text,'FIRST\nSECOND');
  assert.equal(h.api.output(e,h.api.current(e,'last-command'),false).text,'SECOND');
  assert.match(h.api.output(e,h.api.current(e,'last-paste'),true).text,/switch# show one\nFIRST\nCommand 2: \[Enter\]\nswitch# show two\nSECOND/);
  assert.equal(e._ntOutputHistory.filter(i=>i.kind==='command').length,1);
  assert.equal(await h.api.copy(e,false,'last-paste'),true);
  assert.equal(h.clipboard,'FIRST\nSECOND');
  const state=JSON.parse(JSON.stringify(h.api.snapshot(e))),other=h.entry('other');
  h.api.restore(other,state);
  assert.equal(h.api.output(other,h.api.current(other,'last-paste'),false).text,'FIRST\nSECOND');
  assert.equal(h.api.output(other,h.api.current(other,'last-command'),false).text,'SECOND');
  other.xterm.dispose();
 }finally{e.xterm.dispose();}
});
test('a stopped paced paste cannot promote unsent lines or another session',async()=>{
 const h=harness(),a=h.entry('A'),b=h.entry('B');try{
  await h.write(a,'switch# ');const one=h.api.notePaste(a,'first\nsecond\n');one.pacedRunId='stop-1';
  assert.equal(h.api.completePacedPaste(a,one,{runId:'stop-1',sessionId:'B',state:'stopped',steps:[]}),false);
  assert.equal(h.api.completePacedPaste(a,one,{runId:'stop-1',sessionId:'A',state:'stopped',steps:[{index:1,command:'first',output:'switch# first\r\nDONE\r\nswitch# '}]}),true);
  assert.equal(h.api.output(a,h.api.current(a,'last-paste'),false).text,'DONE');
  assert.equal(h.api.output(a,h.api.current(a,'last-command'),false).text,'DONE');
  assert.equal(h.api.current(b,'last-paste'),null);
  assert(!h.api.output(a,h.api.current(a,'last-paste'),true).text.includes('second'));
  const pending=h.api.notePaste(b,'unconfirmed\nother\n');assert(pending);
  assert.equal(h.api.discardPacedPaste(b,pending),true);
  assert.equal(h.api.current(b,'last-paste'),null);
 }finally{a.xterm.dispose();b.xterm.dispose();}
});
function fn(source,name){
 const start=source.search(new RegExp('^(?:async )?function '+name+'\\(','m'));assert(start>=0,name);
 const tail=source.slice(start),next=tail.slice(1).search(/\n(?:async )?function /);
 return next<0?tail:tail.slice(0,next+1);
}
test('macro Last command and Last paste update from verified run, not initial input',async()=>{
 const entry={info:{id:'A'},_ntOutputHistory:[{kind:'paste'}]},block=entry._ntOutputHistory[0];
 const run={id:'run-1',macroId:'literal-paste',state:'running',targets:[{id:'A',generation:9}]};
 const capture={runId:run.id,sessionId:'A',state:'completed',steps:[{index:1,command:'show one',output:'one'},{index:2,command:'show two',output:'two'}]};
 const fetches=[],S={last:new Map()},pacedPending=new Set(),terminalSessions=new Map([['A',entry]]);
 const root={NetworkToolkitTerminalExtras:{
  completePacedPaste(e,i,v){assert.equal(e,entry);assert.equal(i,block);assert.equal(v,capture);i.pacedCapture={steps:v.steps};e._ntOutputHistory.splice(1,0,{kind:'command',pacedCapture:{steps:[v.steps.at(-1)]}});return true;},
  current(e,kind){return e._ntOutputHistory.filter(i=>i.kind===(kind==='last-paste'?'paste':'command')).at(-1)||null;}
 }};
 const c={S,pacedPending,terminalSessions,root,running:r=>['running','paused'].includes(r.state),
   call:async uri=>{fetches.push(uri);return {capture}},redact:s=>s,encodeURIComponent};
 vm.createContext(c);for(const name of ['trackPacedPaste','collectPacedCaptures'])vm.runInContext(fn(macros,name),c);
 assert.equal(c.trackPacedPaste(entry,block,run),true);await tick();assert.deepEqual(fetches,[]);
 await c.collectPacedCaptures([{...run,state:'completed'}]);
 assert.equal(fetches.length,1);assert.match(fetches[0],/run-1\/capture\?sessionId=A$/);
 assert.equal(S.last.get('A').paste,'show one\nshow two');
 assert.equal(S.last.get('A').commands.at(-1),'show two');
 await c.collectPacedCaptures([{...run,state:'completed'}]);assert.equal(fetches.length,1);
});
