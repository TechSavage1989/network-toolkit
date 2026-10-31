'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const assert=require('node:assert/strict'),{test}=require('node:test');
const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
function appFunction(name){
 const start=html.search(new RegExp('^function '+name+'\\(','m'));
 assert(start>=0,'missing '+name);
 const tail=html.slice(start),next=tail.slice(1).search(/\n(?:async )?function /);
 assert(next>=0,'missing next function after '+name);
 return tail.slice(0,next+1);
}
function harness(){
 const ids=['ssh','telnet','serial','powershell'];
 const infos=ids.map(id=>({id,type:id,state:'connected',label:id,nameRevision:1,bytesReceived:0}));
 const entries=new Map(infos.map(info=>[info.id,{info,lastOutputSeq:0}]));
 const timers=new Map();let timer=0,renders=0,outputs=0,seq=0;
 const ctx={
  window:{requestAnimationFrame(){}},state:{activePage:'terminal'},terminalSessions:entries,
  terminalInitialized:true,terminalInitialPanesFilled:true,terminalFeedAfter:0,
  terminalOutputChromeTimer:null,DETACHED_WORKSPACE:false,shutDown:false,
  upsertTerminalSession(info){const entry=entries.get(info.id);if(entry)entry.info={...info};return entry;},
  terminalEntryShown:()=>true,appendTerminalOutput:()=>{outputs++;},flushTerminalOutputBatch:()=>{},base64ToBytes:value=>value,
  setTimeout(fn,delay){const key=++timer;timers.set(key,{fn,delay});return key;},
  clearTimeout:key=>timers.delete(key),
  renderTerminalChrome(){
   if(ctx.terminalOutputChromeTimer!==null){ctx.clearTimeout(ctx.terminalOutputChromeTimer);ctx.terminalOutputChromeTimer=null;}
   renders++;
  }
 };
 vm.createContext(ctx);
 for(const name of ['scheduleTerminalOutputChrome','processTerminalBatch'])vm.runInContext(appFunction(name),ctx);
 function outputBatch(change){
  const sessions=infos.map(info=>({...info,bytesReceived:++info.bytesReceived,...(change?.[info.id]||{})}));
  const events=ids.map(id=>({type:'output',sessionId:id,seq:++seq,data:'eA=='}));
  ctx.processTerminalBatch({sessions,events,next:seq});
 }
 return {ctx,infos,timers,outputBatch,get renders(){return renders;},get outputs(){return outputs;},
  advance(){const tasks=[...timers.values()];timers.clear();tasks.forEach(t=>t.fn());}};
}
test('four streaming terminals incur one deferred chrome render per output burst',()=>{
 const h=harness();
 for(let i=0;i<40;i++)h.outputBatch();
 assert.equal(h.outputs,160);
 assert.equal(h.renders,0,'terminal output should not redraw the entire app synchronously');
 assert.equal(h.timers.size,1,'one pending redraw is sufficient for forty batches');
 assert.equal([...h.timers.values()][0].delay,75);
 h.advance();assert.equal(h.renders,1);
 h.outputBatch();assert.equal(h.timers.size,1);h.advance();
 assert.equal(h.renders,2,'future output remains visible');
});
test('important terminal status transitions redraw immediately and cancel stale refreshes',()=>{
 const h=harness();h.outputBatch();assert.equal(h.timers.size,1);
 h.outputBatch({ssh:{state:'awaiting_input',prompt:{id:'verification'}}});
 assert.equal(h.renders,1);
 assert.equal(h.timers.size,0,'an immediate status change supersedes a delayed cosmetic refresh');
 h.advance();assert.equal(h.renders,1);
});
test('background diagnostic pages skip terminal chrome output work',()=>{
 const h=harness();h.ctx.state.activePage='overview';
 for(let i=0;i<10;i++)h.outputBatch();
 assert.equal(h.outputs,40);
 assert.equal(h.timers.size,0);
 assert.equal(h.renders,0);
});
