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
 c.window=c;c.state={terminalPromptNames:[]};vm.createContext(c);vm.runInContext(fs.readFileSync(path.join(__dirname,'../web/assets/terminal-prompt-labels.js'),'utf8'),c);for(const name of ['terminalDimensions','queueTerminalResize','drainTerminalResize','cancelTerminalFit','cancelTerminalResizeTimer'])vm.runInContext(source(name),c);return c;
}
test('Repeated identical fits cannot starve the pending backend resize',async()=>{
 const c=setup(),size={cols:154,rows:45};c.queueTerminalResize(c.entry,size);
 // Faster than the 120 ms resize debounce: continuous output/layout updates
 // must not postpone the same pending geometry indefinitely.
 for(let i=0;i<12;i++){await c.advance(80);c.queueTerminalResize(c.entry,size);}
 assert.equal(c.requests.length,1);assert.equal(c.requests[0].at,120);assert.deepEqual(c.requests[0].size,size);
 await c.advance(120);assert.equal(c.entry.desiredResize,null);assert.equal(c.entry.lastResize,'154x45');assert.equal(c.entry.resizeSending,false);
});
test('A different pending size retains the first bounded deadline and latest dimensions win',async()=>{
 const c=setup();c.queueTerminalResize(c.entry,{cols:80,rows:24});await c.advance(60);c.queueTerminalResize(c.entry,{cols:120,rows:35});await c.advance(40);c.queueTerminalResize(c.entry,{cols:120,rows:35});await c.advance(19);assert.equal(c.requests.length,0);await c.advance(1);
 assert.deepEqual(c.requests,[{at:120,url:'/terminals/resize-fixture/resize',method:'POST',size:{cols:120,rows:35}}]);
});
test('Resize guards remain fail-closed and a later identical fit can resume',async()=>{
 const c=setup();c.entry.transferBusy=true;c.queueTerminalResize(c.entry,{cols:154,rows:45});assert.equal(c.entry.desiredResize,undefined);c.entry.transferBusy=false;c.entry.restoring=true;c.queueTerminalResize(c.entry,{cols:154,rows:45});assert.equal(c.entry.desiredResize,undefined);c.entry.restoring=false;
 c.queueTerminalResize(c.entry,{cols:154,rows:45});c.owned=false;await c.advance(200);assert.equal(c.requests.length,0);assert(c.entry.desiredResize);c.owned=true;c.queueTerminalResize(c.entry,{cols:154,rows:45});await c.advance(120);assert.equal(c.requests.length,1);assert.equal(c.entry.lastResize,'154x45');
});
test('Clearing an outgoing timer does not strand a later identical resize',async()=>{
 const c=setup();c.queueTerminalResize(c.entry,{cols:90,rows:30});await c.advance(40);c.cancelTerminalResizeTimer(c.entry);await c.advance(160);c.queueTerminalResize(c.entry,{cols:90,rows:30});await c.advance(120);assert.equal(c.requests.length,1);assert.equal(c.entry.desiredResize,null);
});
test('A new size queued during an in-flight request is sent after its acknowledgement',async()=>{
 const c=setup();let accept;c.api=async(url,method,size)=>{c.requests.push({url,method,size:{...size}});if(c.requests.length===1)await new Promise(resolve=>accept=resolve);return {};};
 c.queueTerminalResize(c.entry,{cols:80,rows:24});await c.advance(120);assert.equal(c.entry.resizeSending,true);c.queueTerminalResize(c.entry,{cols:100,rows:32});await c.advance(80);c.queueTerminalResize(c.entry,{cols:100,rows:32});await c.advance(40);assert.equal(c.requests.length,1);accept();await new Promise(setImmediate);assert.deepEqual(c.requests.map(r=>r.size),[{cols:80,rows:24},{cols:100,rows:32}]);assert.equal(c.entry.lastResize,'100x32');assert.equal(c.entry.desiredResize,null);
});

test('Acknowledged identical 80ms fits preserve real xterm prompt stability without extra requests',async()=>{
 global.self=global;const {Terminal}=require('../web/assets/xterm.js');
 const c=setup(),entry=c.entry;entry.info={...entry.info,type:'ssh',host:'192.0.2.5',port:22,username:'operator'};entry.config={...entry.info};entry.xterm=new Terminal({cols:90,rows:12,allowProposedApi:true});entry.outputRevision=1;entry.lastResize='90x12';c.saveState=()=>{};
 try{await new Promise(resolve=>entry.xterm.write('EDGE-SW01(config-if)# ',resolve));c.NetworkToolkitTerminalPromptLabels.observe(entry);
 for(let i=0;i<12;i++){c.queueTerminalResize(entry,{cols:90,rows:12});await c.advance(80);}
 assert.equal(entry._ntPromptName,'EDGE-SW01');assert.equal(entry.outputRevision,1);assert.equal(c.requests.length,0);assert.equal(entry.desiredResize,undefined);
 }finally{entry.xterm.dispose();}
});
test('Returning to an acknowledged size while another resize is in flight must still be sent',async()=>{
 const c=setup();c.entry.lastResize='80x24';let accept;
 c.api=async(url,method,size)=>{c.requests.push({url,method,size:{...size}});if(c.requests.length===1)await new Promise(resolve=>accept=resolve);return {};};
 c.queueTerminalResize(c.entry,{cols:100,rows:32});await c.advance(120);assert.equal(c.entry.resizeSending,true);
 c.queueTerminalResize(c.entry,{cols:80,rows:24});assert.equal(c.entry.desiredResize.cols,80);assert.equal(c.entry.desiredResize.rows,24);accept();await new Promise(setImmediate);
 assert.deepEqual(c.requests.map(r=>r.size),[{cols:100,rows:32},{cols:80,rows:24}]);assert.equal(c.entry.lastResize,'80x24');
});
test('continuously changing pane geometry reaches the backend before dragging stops',async()=>{
 const c=setup();for(let i=0;i<24;i++){c.queueTerminalResize(c.entry,{cols:80+i,rows:24+i%3});await c.advance(20);}
 assert(c.requests.length>=3,'continuous changes must not starve remote PTY resize');assert.equal(c.requests[0].at,120);assert.deepEqual(c.requests[0].size,{cols:85,rows:26});await c.advance(120);assert.equal(c.entry.lastResize,'103x26');assert.equal(c.entry.desiredResize,null);
});
test('continuous changes during a slow acknowledgement remain single-flight and send only the latest pending size',async()=>{
 const c=setup();let accept,active=0,maxActive=0;c.api=async(url,method,size)=>{active++;maxActive=Math.max(maxActive,active);c.requests.push({size:{...size}});if(c.requests.length===1)await new Promise(resolve=>accept=resolve);active--;return {};};
 c.queueTerminalResize(c.entry,{cols:80,rows:24});await c.advance(120);for(let i=0;i<20;i++){c.queueTerminalResize(c.entry,{cols:90+i,rows:30});await c.advance(20);}assert.equal(c.requests.length,1);accept();await new Promise(setImmediate);assert.equal(maxActive,1);assert.deepEqual(c.requests.map(r=>r.size),[{cols:80,rows:24},{cols:109,rows:30}]);assert.equal(c.entry.lastResize,'109x30');
});
test('old input epoch acknowledgement cannot overwrite new epoch geometry or send while ownership is lost',async()=>{
 const c=setup();let accept;c.api=async(url,method,size)=>{c.requests.push({size:{...size}});if(c.requests.length===1)await new Promise(resolve=>accept=resolve);return {};};c.queueTerminalResize(c.entry,{cols:80,rows:24});await c.advance(120);
 c.entry.inputEpoch++;c.owned=false;c.entry.lastResize='';accept();await new Promise(setImmediate);assert.equal(c.entry.lastResize,'');assert.equal(c.entry.info.cols,undefined);c.queueTerminalResize(c.entry,{cols:100,rows:30});await c.advance(200);assert.equal(c.requests.length,1);c.owned=true;c.queueTerminalResize(c.entry,{cols:100,rows:30});await c.advance(120);assert.equal(c.requests.length,2);assert.equal(c.entry.lastResize,'100x30');
});

test('backwards wall clock cannot push a pending resize beyond its transport budget',async()=>{const c=setup();c.queueTerminalResize(c.entry,{cols:80,rows:24});c.Date.now=()=>-3600000;c.queueTerminalResize(c.entry,{cols:90,rows:30});await c.advance(120);assert.equal(c.requests.length,1);assert.equal(c.entry.lastResize,'90x30');});
test('four panes retain pending transport timer identity through reentrant acknowledgements',async()=>{
 const c=setup(),entries=Array.from({length:4},(_,i)=>({...c.entry,info:{id:'pane-'+i,state:'connected'}}));let reentered=false,handles;
 c.api=async(url,method,size)=>{c.requests.push({url,size:{...size}});if(!reentered){reentered=true;for(let i=1;i<4;i++){c.queueTerminalResize(entries[i],{cols:100+i,rows:30});assert.equal(entries[i].resizeTimer,handles[i],'ready pending timer must not be replaced by another pane callback');}}return {};};
 for(const entry of entries)c.queueTerminalResize(entry,{cols:80,rows:24});handles=entries.map(e=>e.resizeTimer);await c.advance(120);assert.equal(c.requests.length,4);assert.deepEqual(c.requests.map(r=>r.size.cols),[80,101,102,103]);assert(entries.every(e=>e.resizeTimer===null&&!e.desiredResize&&!e.resizeSending));
});
test('explicit lifecycle cancellation clears all pending timer identities and permits fresh work',async()=>{const c=setup();c.queueTerminalResize(c.entry,{cols:80,rows:24});c.cancelTerminalResizeTimer(c.entry);assert.equal(c.entry.resizeTimer,null);assert.equal(c.entry.resizeDueAt,null);await c.advance(200);assert.equal(c.requests.length,0);c.queueTerminalResize(c.entry,{cols:90,rows:30});await c.advance(120);assert.equal(c.requests.length,1);assert.equal(c.entry.lastResize,'90x30');});
