'use strict';
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
function source(name){
 const start=html.search(new RegExp('^function '+name+'\\(','m'));
 assert(start>=0,'Missing '+name);
 const tail=html.slice(start),stop=tail.slice(1).search(/\n(?:async )?function /);
 assert(stop>=0,'Function not terminated: '+name);
 return tail.slice(0,stop+1);
}
const functions=['markTerminalOutputLoss','writeTerminalDisplay','flushTerminalOutputBatch','appendTerminalOutput','scheduleTerminalOutputChrome','processTerminalBatch'];
function harness(ids=['ssh','telnet','serial','powershell']){
 const infos=ids.map(id=>({id,type:id,state:'connected',label:id,bytesReceived:0}));
 const writes=new Map(ids.map(id=>[id,[]])),observed=[],macros=[],after=[],prompt=[],timers=new Map();
 let timer=0,renders=0;
 const entries=new Map(infos.map(info=>[info.id,{
  info,lastOutputSeq:0,outputRevision:0,renderPendingBytes:0,missingOutput:false,
  xterm:{
   write(bytes,callback){writes.get(info.id).push({kind:'bytes',bytes:Buffer.from(bytes),callback});},
   writeln(message){writes.get(info.id).push({kind:'notice',message});}
  }
 }]));
 const ctx={
  window:{
   requestAnimationFrame(){},
   NetworkToolkitTerminalMacros:{onOutput:(entry,bytes)=>macros.push([entry.info.id,Buffer.from(bytes)])},
   NetworkToolkitTerminalPromptLabels:{observe:entry=>prompt.push(entry.info.id)}
  },
  NetworkToolkitTerminalExtras:{
   beforeOutput:()=>({epoch:0}),
   afterOutput:(entry,position)=>after.push(entry.info.id),
   observeOutput:(entry,bytes)=>observed.push([entry.info.id,Buffer.from(bytes)])
  },
  state:{activePage:'terminal'},terminalInitialized:true,terminalInitialPanesFilled:true,
  terminalSessions:entries,terminalFeedAfter:0,terminalOutputChromeTimer:null,
  DETACHED_WORKSPACE:false,shutDown:false,
  base64ToBytes:value=>Buffer.from(value,'base64'),
  terminalEntryShown:()=>true,
  upsertTerminalSession(info){const entry=entries.get(info.id);if(entry)entry.info={...info};return entry;},
  setTimeout(fn,delay){const id=++timer;timers.set(id,{fn,delay});return id;},
  clearTimeout:id=>timers.delete(id),
  renderTerminalChrome(){renders++;if(ctx.terminalOutputChromeTimer!==null){ctx.clearTimeout(ctx.terminalOutputChromeTimer);ctx.terminalOutputChromeTimer=null;}}
 };
 vm.createContext(ctx);
 for(const name of functions)vm.runInContext(source(name),ctx);
 let seq=0;
 function batch(chunks,changes={}){
  const events=chunks.map(([id,data])=>({type:'output',sessionId:id,seq:++seq,data:Buffer.from(data).toString('base64')}));
  const sessions=infos.map(info=>({...info,bytesReceived:++info.bytesReceived,...(changes[info.id]||{})}));
  ctx.processTerminalBatch({sessions,events,next:seq});
 }
 const drain=id=>{for(const write of writes.get(id))if(write.kind==='bytes'&&write.callback){const callback=write.callback;write.callback=null;callback();}};
 return {ctx,entries,writes,observed,macros,after,prompt,timers,batch,drain,get renders(){return renders;}};
}
test('a multi-terminal output burst preserves every byte and collapses parser writes by session',()=>{
 const h=harness(),ids=['ssh','telnet','serial','powershell'],chunks=[],expected=new Map(ids.map(id=>[id,[]]));
 for(let i=0;i<80;i++)for(const id of ids){
  // Every chunk must arrive in order even while multiple sessions stream.
  const bytes=Buffer.from((i%2?'m':'\x1b[32')+id+':'+i+':'+'.'.repeat(160));
  chunks.push([id,bytes]);expected.get(id).push(bytes);
 }
 h.batch(chunks);
 assert.equal(h.observed.length,320,'each original output event is still captured');
 assert.equal(h.macros.length,320,'macro recording retains event boundaries');
 for(const id of ids){
  const writes=h.writes.get(id).filter(w=>w.kind==='bytes');
  assert.equal(writes.length,1,'small per-session bursts need just one xterm parser write');
  assert.deepEqual(Buffer.concat(writes.map(w=>w.bytes)),Buffer.concat(expected.get(id)));
  assert.equal(h.entries.get(id).renderPendingBytes,Buffer.concat(expected.get(id)).length);
  h.drain(id);assert.equal(h.entries.get(id).renderPendingBytes,0);
 }
 assert.equal(h.after.length,4,'post-render hooks run after each combined parser write');
 assert.equal(h.prompt.length,4);
 assert.equal(h.renders,0,'cosmetic chrome rendering remains deferred');
 assert.equal(h.timers.size,1);
});
test('large output bursts use bounded 64 KiB writes with exact ordering and no silent clipping',()=>{
 const h=harness(['ssh']),chunks=Array.from({length:160},(_,i)=>['ssh',Buffer.alloc(1024,i%251)]);
 h.batch(chunks);
 const writes=h.writes.get('ssh').filter(w=>w.kind==='bytes');
 assert.equal(writes.length,3);
 assert.deepEqual(writes.map(w=>w.bytes.length),[65536,65536,32768]);
 assert.deepEqual(Buffer.concat(writes.map(w=>w.bytes)),Buffer.concat(chunks.map(c=>c[1])));
 assert.equal(h.observed.length,160);
 assert.equal(h.entries.get('ssh').missingOutput,false);
 h.drain('ssh');assert.equal(h.entries.get('ssh').renderPendingBytes,0);
});
test('over-cap output reports loss after flushing earlier buffered bytes',()=>{
 const h=harness(['ssh']);
 h.batch([['ssh',Buffer.alloc(700000,65)],['ssh',Buffer.alloc(400000,66)]]);
 const writes=h.writes.get('ssh');
 assert.equal(writes[0].kind,'bytes');
 assert.equal(writes[0].bytes.length,700000);
 assert.equal(writes[1].kind,'notice');
 assert.match(writes[1].message,/could not keep up/i);
 assert.equal(h.entries.get('ssh').missingOutput,true);
 assert.equal(h.observed.length,2,'original capture remains independent of xterm display limits');
 h.drain('ssh');assert.equal(h.entries.get('ssh').renderPendingBytes,0);
});
test('status changes retain immediate display writes instead of batching',()=>{
 const h=harness(['ssh']);
 h.batch([['ssh',Buffer.from('first')]],{ssh:{state:'awaiting_input',prompt:{id:'verify'}}});
 assert.equal(h.writes.get('ssh').filter(w=>w.kind==='bytes').length,1);
 assert.equal(h.renders,1);
 assert.equal(h.timers.size,0);
});
test('a retired terminal cannot receive a late parser callback',()=>{
 const h=harness(['ssh']);
 h.batch([['ssh',Buffer.from('payload')]]);
 h.entries.get('ssh').xterm={write(){throw Error('Unexpected call')}};
 h.drain('ssh');
 assert.equal(h.after.length,0);
 assert.equal(h.prompt.length,0);
 assert.equal(h.entries.get('ssh').renderPendingBytes,0);
});
