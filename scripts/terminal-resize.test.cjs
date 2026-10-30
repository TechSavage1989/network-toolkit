'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
function source(name){const start=html.search(new RegExp('^(?:async )?function '+name+'\\(','m'));assert(start>=0,'Missing '+name);const tail=html.slice(start),next=tail.slice(1).search(/\n(?:async )?function /);return next<0?tail:tail.slice(0,next+1);}
function setup(){
 let now=0,sequence=0;const timers=new Map(),requests=[],entry={info:{id:'resize-fixture',state:'connected'},inputEpoch:0,lastResize:'',resizeTimer:null};
 const c={entry,requests,online:true,owned:true,Date:{now:()=>now},terminalOwnerCanMutate:()=>c.owned,renderTerminalChrome(){},toast:()=>assert.fail('Unexpected resize error'),setTimeout(fn,delay){const id=++sequence;timers.set(id,{fn,at:now+delay});return id;},clearTimeout:id=>timers.delete(id)};
 c.api=async(url,method,size)=>{requests.push({at:now,url,method,size:{...size}});return {};};
 c.advance=async(ms)=>{const end=now+ms;for(;;){const next=[...timers].filter(([,timer])=>timer.at<=end).sort((a,b)=>a[1].at-b[1].at||a[0]-b[0])[0];if(!next)break;now=next[1].at;timers.delete(next[0]);next[1].fn();await Promise.resolve();await Promise.resolve();}now=end;};
 // Use the real optional display observer; resize-only entries intentionally have no xterm.
 c.window=c;c.state={terminalPromptNames:[]};vm.createContext(c);vm.runInContext(fs.readFileSync(path.join(__dirname,'../web/assets/terminal-prompt-labels.js'),'utf8'),c);for(const name of ['terminalDimensions','queueTerminalResize','drainTerminalResize'])vm.runInContext(source(name),c);return c;
}
test('Repeated identical fits cannot starve the pending backend resize',async()=>{
 const c=setup(),size={cols:154,rows:45};c.queueTerminalResize(c.entry,size);
 // Faster than the 120 ms resize debounce: continuous output/layout updates
 // must not postpone the same pending geometry indefinitely.
 for(let i=0;i<12;i++){await c.advance(80);c.queueTerminalResize(c.entry,size);}
 assert.equal(c.requests.length,1);assert.equal(c.requests[0].at,120);assert.deepEqual(c.requests[0].size,size);
 await c.advance(120);assert.equal(c.entry.desiredResize,null);assert.equal(c.entry.lastResize,'154x45');assert.equal(c.entry.resizeSending,false);
});
test('A different pending size still debounces and latest dimensions win',async()=>{
 const c=setup();c.queueTerminalResize(c.entry,{cols:80,rows:24});await c.advance(60);c.queueTerminalResize(c.entry,{cols:120,rows:35});await c.advance(40);c.queueTerminalResize(c.entry,{cols:120,rows:35});await c.advance(79);assert.equal(c.requests.length,0);await c.advance(1);
 assert.deepEqual(c.requests,[{at:180,url:'/terminals/resize-fixture/resize',method:'POST',size:{cols:120,rows:35}}]);
});
test('Resize guards remain fail-closed and a later identical fit can resume',async()=>{
 const c=setup();c.entry.transferBusy=true;c.queueTerminalResize(c.entry,{cols:154,rows:45});assert.equal(c.entry.desiredResize,undefined);c.entry.transferBusy=false;c.entry.restoring=true;c.queueTerminalResize(c.entry,{cols:154,rows:45});assert.equal(c.entry.desiredResize,undefined);c.entry.restoring=false;
 c.queueTerminalResize(c.entry,{cols:154,rows:45});c.owned=false;await c.advance(200);assert.equal(c.requests.length,0);assert(c.entry.desiredResize);c.owned=true;c.queueTerminalResize(c.entry,{cols:154,rows:45});await c.advance(120);assert.equal(c.requests.length,1);assert.equal(c.entry.lastResize,'154x45');
});
test('Clearing an outgoing timer does not strand a later identical resize',async()=>{
 const c=setup();c.queueTerminalResize(c.entry,{cols:90,rows:30});await c.advance(40);c.clearTimeout(c.entry.resizeTimer);await c.advance(160);c.queueTerminalResize(c.entry,{cols:90,rows:30});await c.advance(120);assert.equal(c.requests.length,1);assert.equal(c.entry.desiredResize,null);
});
test('A new size queued during an in-flight request is sent after its acknowledgement',async()=>{
 const c=setup();let accept;c.api=async(url,method,size)=>{c.requests.push({url,method,size:{...size}});if(c.requests.length===1)await new Promise(resolve=>accept=resolve);return {};};
 c.queueTerminalResize(c.entry,{cols:80,rows:24});await c.advance(120);assert.equal(c.entry.resizeSending,true);c.queueTerminalResize(c.entry,{cols:100,rows:32});await c.advance(80);c.queueTerminalResize(c.entry,{cols:100,rows:32});await c.advance(40);assert.equal(c.requests.length,1);accept();await new Promise(setImmediate);assert.deepEqual(c.requests.map(r=>r.size),[{cols:80,rows:24},{cols:100,rows:32}]);assert.equal(c.entry.lastResize,'100x32');assert.equal(c.entry.desiredResize,null);
});
