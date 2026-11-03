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
  assert.equal(h.api.output(e,h.api.current(e,'last-paste'),true).text,'show one\nFIRST\n\nshow two\nSECOND');
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

test('macro runs link a batch block per target and complete it from the verified capture without touching macro shortcuts',async()=>{
 const entry={info:{id:'A'},_ntOutputHistory:[]},other={info:{id:'B'},_ntOutputHistory:[]};
 const noted=[],discarded=[];
 const root={NetworkToolkitTerminalExtras:{
  notePaste(e,text){const block={kind:'paste',text};e._ntOutputHistory.push(block);noted.push({id:e.info.id,text});return block;},
  discardPacedPaste(e,block){discarded.push(e.info.id);e._ntOutputHistory.splice(e._ntOutputHistory.indexOf(block),1);return true;},
  completePacedPaste(e,i,v){i.pacedCapture={steps:v.steps};return true;},
  current(e,kind){return e._ntOutputHistory.filter(i=>i.kind===(kind==='last-paste'?'paste':'command')).at(-1)||null;}
 }};
 const fetches=[],S={last:new Map()},pacedPending=new Set(),terminalSessions=new Map([['A',entry],['B',other]]);
 const capture={runId:'macro-1',sessionId:'A',state:'completed',steps:[{index:1,command:'show one',output:'one'},{index:2,command:'show two',output:'two'}]};
 const c={S,pacedPending,terminalSessions,root,running:r=>['running','paused'].includes(r.state),call:async uri=>{fetches.push(uri);return {capture:{...capture,sessionId:uri.endsWith('=A')?'A':'B'}}},redact:s=>s,encodeURIComponent};
 vm.createContext(c);for(const name of ['noteRunBlocks','linkRunBlocks','macroCommands','trackPacedPaste','collectPacedCaptures'])vm.runInContext(fn(macros,name),c);
 const macro={steps:[{kind:'command',text:'show one'},{kind:'delay',delayMs:5},{kind:'command',text:'skip me',disabled:true},{kind:'command',text:'show two'}]};
 assert.deepEqual(c.macroCommands(macro),['show one','show two'],'Only enabled command steps form the batch');
 const blocks=c.noteRunBlocks([{id:'A',generation:9},{id:'B',generation:2},{id:'missing',generation:1}],c.macroCommands(macro));
 assert.equal(blocks.length,2);assert.deepEqual(noted,[{id:'A',text:'show one\nshow two\n'},{id:'B',text:'show one\nshow two\n'}]);
 const run={id:'macro-1',macroId:'fixture',state:'running',targets:[{id:'A',generation:9},{id:'B',generation:2}]};
 c.linkRunBlocks(blocks,run);assert.deepEqual(discarded,[]);assert.equal(blocks[0].block.pacedRunId,'macro-1');assert.equal(blocks[1].block.pacedGeneration,2);
 await tick();assert.deepEqual(fetches,[],'No capture is fetched while the run is active');
 await c.collectPacedCaptures([{...run,state:'completed'}]);
 assert.equal(fetches.length,2);assert(blocks[0].block.pacedCapture&&blocks[1].block.pacedCapture,'Every target console receives the verified capture');
 assert.equal(S.last.size,0,'Macro runs do not rewrite the Last command / Last batch macro shortcuts');
 // A start that fails discards the provisional blocks instead of leaving empty batches behind.
 const failed=c.noteRunBlocks([{id:'A',generation:9}],['show three']);c.linkRunBlocks(failed,{id:'macro-2',state:'failed',targets:[]});assert.deepEqual(discarded,['A']);
 c.linkRunBlocks(c.noteRunBlocks([{id:'B',generation:2}],['show four']),null);assert.deepEqual(discarded,['A','B']);
 assert.equal(c.noteRunBlocks([{id:'A',generation:9}],['  ','']).length,0,'Blank command lists create no block');
});

test('pager prompt residue (blank and space-only lines after --More--) leaves no gaps in batch output',async()=>{
 const h=harness(),e=h.entry('aoscx');try{
  await h.write(e,'switch# ');
  const block=h.api.notePaste(e,'show run\nshow vlan\n');block.pacedRunId='run-gaps';
  const pad=' '.repeat(63);
  assert(h.api.completePacedPaste(e,block,{runId:'run-gaps',sessionId:'aoscx',state:'completed',steps:[
   {index:1,command:'show run',output:'switch# show run\r\nCurrent configuration:\r\n!\r\n    qos trust cos\r\n--More--\r\n\r'+pad+'\r    spanning-tree bpdu-guard\r\ninterface 1/1/7\r\n--More--\r'+pad+'\r\n    no shutdown\r\n\r\nhttps-server vrf mgmt\r\nswitch# '},
   {index:2,command:'show vlan',output:'switch# show vlan\r\nVLAN  Name\r\n\r\n1     DEFAULT\r\nswitch# '}
  ]}));
  assert.equal(h.api.output(e,h.api.current(e,'last-paste'),false).text,'Current configuration:\n!\n    qos trust cos\n    spanning-tree bpdu-guard\ninterface 1/1/7\n    no shutdown\n\nhttps-server vrf mgmt\nVLAN  Name\n\n1     DEFAULT');
 }finally{e.xterm.dispose();}
});

test('paced output uses submitted commands without numbering or duplicate device echoes',async()=>{
 const h=harness(),e=h.entry('paced');try{
  await h.write(e,'switch# ');
  const block=h.api.notePaste(e,'  vlan trunk native 10\nexit\nshow version\n');
  block.pacedRunId='run-formatted';
  assert(h.api.completePacedPaste(e,block,{
   runId:'run-formatted',sessionId:'paced',state:'completed',steps:[
    {index:439,command:'  vlan trunk native 10',output:'  vlan trunk native 10\r\nApplied trunk settings\r\nswitch(config-if)# '},
    {index:440,command:'exit',output:'switch(config-if)# exit\r\nswitch# '},
    {index:441,command:'show version',output:'switch# show version\r\nSystem version 10.14\r\nshow version\r\nswitch# '}
   ]
  }));
  const paste=h.api.current(e,'last-paste'),last=h.api.current(e,'last-command');
  assert.equal(h.api.output(e,paste,true).text,'  vlan trunk native 10\nApplied trunk settings\nexit\nshow version\nSystem version 10.14\nshow version');
  assert.equal(h.api.output(e,paste,false).text,'Applied trunk settings\nSystem version 10.14\nshow version');
  assert.equal(h.api.output(e,last,true).text,'show version\nSystem version 10.14\nshow version');
  assert.equal(h.api.output(e,last,false).text,'System version 10.14\nshow version');
  assert(!h.api.output(e,paste,true).text.includes('Command 439:'));
  assert.equal(await h.api.copy(e,false,'last-paste'),true);
  assert.equal(h.clipboard,'Applied trunk settings\nSystem version 10.14\nshow version');
  assert.equal(await h.api.copy(e,true,'last-paste'),true);
  assert.equal(h.clipboard,'  vlan trunk native 10\nApplied trunk settings\nexit\nshow version\nSystem version 10.14\nshow version');
  assert.equal(await h.api.copy(e,false,'last-command'),true);
  assert.equal(h.clipboard,'System version 10.14\nshow version');
 }finally{e.xterm.dispose();}
});
test('paced output preserves every device response across 458 commands without echo or headings',async()=>{
 const h=harness(),e=h.entry('large');try{
  await h.write(e,'switch# ');
  const block=h.api.notePaste(e,'fixture command list\n');block.pacedRunId='large-run';
  const steps=Array.from({length:458},(_,i)=>{
   const command=i%4===0?'  vlan trunk allowed 4,10':i%4===1?'qos trust cos':i%4===2?'spanning-tree bpdu-guard':'exit';
   const echo=i%3===0?command:'switch(config-if)# '+command;
   return {index:i+1,command,output:echo+'\r\nRESULT-'+(i+1)+'\r\nextra data '+(i+1)+'\r\nswitch(config-if)# '};
  });
  assert(h.api.completePacedPaste(e,block,{runId:'large-run',sessionId:'large',state:'completed',steps}));
  const on=h.api.output(e,h.api.current(e,'last-paste'),true).text;
  const off=h.api.output(e,h.api.current(e,'last-paste'),false).text;
  assert(!on.includes('Command '));assert(!on.includes('switch(config-if)#'));
  assert(!off.includes('vlan trunk allowed'));assert(!off.includes('qos trust cos'));
  assert(!off.includes('spanning-tree bpdu-guard'));assert(!off.includes('\nexit\n'));
  for(let i=1;i<=458;i++){
   assert(on.includes('RESULT-'+i+'\nextra data '+i), 'missing included output '+i);
   assert(off.includes('RESULT-'+i+'\nextra data '+i), 'missing output '+i);
  }
  assert.equal(off.split('\n').filter(line=>/^RESULT-\d+$/.test(line)).length,458);
  assert.equal(on.split('\n').filter(line=>/^RESULT-\d+$/.test(line)).length,458);
  assert.equal(on.split('\n').filter(line=>line==='  vlan trunk allowed 4,10').length,115);
 }finally{e.xterm.dispose();}
});
test('paced output keeps non-echo lines, internal blanks, and truncated capture status',()=>{
 const output={pacedCapture:{truncated:true,steps:[
  {index:1,command:'show something',output:'Result line\n\nshow something\nEnd\nswitch# '},
  {index:2,command:'  description TEST',output:'\n  description TEST\nError: invalid command\nswitch# '},
  {index:3,command:'  no shutdown',output:'\nConfiguration accepted\r\nswitch# '}
 ]}};
 const r=harness().api;
 const off=r.output(null,output,false),on=r.output(null,output,true);
 assert.equal(off.truncated,true);
 assert.equal(off.text,'Result line\n\nshow something\nEnd\nError: invalid command\nConfiguration accepted');
 assert.equal(on.text,'show something\nResult line\n\nshow something\nEnd\n  description TEST\nError: invalid command\n  no shutdown\nConfiguration accepted');
});

test('Select output uses the same include-commands filter for last paste and last command',()=>{
 const selected={value:'',focused:false,selected:false,focus(){this.focused=true;},select(){this.selected=true;}};
 const document={querySelector:selector=>selector==='#terminalSelectedOutput'?selected:null};
 const window={NetworkToolkitTerminalInteraction:{preferences:()=>({pasteIncludeCommands:false})}};
 const modals=[];
 const ctx={window,document,TextDecoder,TextEncoder,Date,Math,setTimeout,clearTimeout,toast(){},
   terminalEntryAcceptsInput:()=>true,currentTerminal:()=>null,
   openModal:(...args)=>modals.push(args),icon:()=>'',console};
 vm.runInNewContext(extras,ctx);const api=window.NetworkToolkitTerminalExtras;
 const steps=[
  {index:439,command:'  vlan trunk native 10',output:'  vlan trunk native 10\r\ntrunk updated\r\nswitch# '},
  {index:440,command:'show interfaces',output:'switch# show interfaces\r\nPORT STATUS\r\nswitch# '}
 ];
 const item={kind:'paste',echo:[],pacedCapture:{steps}},last={kind:'command',echo:[],pacedCapture:{steps:[steps[1]]}};
 const entry={info:{id:'test',state:'connected'},_ntOutputHistory:[item,last],
   xterm:new Terminal({cols:80,rows:12,allowProposedApi:true})};
 try{
  assert.equal(api.select(entry,false,'last-paste'),true);
  assert.equal(selected.value,'trunk updated\nPORT STATUS');
  assert(selected.selected&&selected.focused);
  assert.equal(api.select(entry,true,'last-paste'),true);
  assert.equal(selected.value,'  vlan trunk native 10\ntrunk updated\nshow interfaces\nPORT STATUS');
  assert.equal(api.select(entry,false,'last-command'),true);
  assert.equal(selected.value,'PORT STATUS');
  assert.equal(api.select(entry,true,'last-command'),true);
  assert.equal(selected.value,'show interfaces\nPORT STATUS');
  assert.equal(modals.length,4);
  assert(!selected.value.includes('Command 440:'));
 }finally{entry.xterm.dispose();}
});
