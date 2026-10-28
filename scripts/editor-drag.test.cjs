'use strict';
const test=require('node:test'),assert=require('node:assert/strict');
const {create,MIME}=require('../web/assets/editor-drag.js');
const flush=()=>new Promise(resolve=>setImmediate(resolve));
function fixture({native=true,pending=false,reject=false}={}){
 const calls=[],messages=[],nodes=[],classes=()=>({values:new Set(),add(v){this.values.add(v)},remove(v){this.values.delete(v)}});
 const node=(kind,uid)=>{const n={kind,dataset:uid?{edUid:uid}:{},classList:classes(),hidden:false,setAttribute(k,v){this[k]=v},removeAttribute(k){delete this[k]},contains(x){return nodes.includes(x)},append(x){nodes.push(x)},getBoundingClientRect(){return {left:uid==='a'?0:100,width:100}},closest(selector){if(selector==='[data-ed-uid]')return kind==='tab'?this:null;if(selector==='[data-ed-tabs]')return ['tab','strip'].includes(kind)?strip:null;if(selector==='[data-editor-drag-detach]')return kind==='zone'?this:null;if(selector==='.nt-editor-window')return kind==='outside'?null:host;return null;}};nodes.push(n);return n;};
 const host=node('host'),strip=node('strip'),a=node('tab','a'),b=node('tab','b'),outside=node('outside');a.nextElementSibling=b;b.nextElementSibling=null;
 host.querySelector=selector=>selector.includes('"a"')?a:selector.includes('"b"')?b:null;
 const document={querySelector:selector=>selector==='.nt-editor-window'?host:null,querySelectorAll:selector=>selector==='[data-editor-drop-before]'?nodes.filter(x=>Object.hasOwn(x,selector.slice(1,-1))):[host],createElement:()=>node('zone'),addEventListener(){}};
 let reason='',resolve;
 const coordinator={state:{enabled:native},beginDrag:(uid,ticket)=>{calls.push(['begin',uid,ticket]);return reject?Promise.reject(Error('ownership unavailable')):pending?new Promise(r=>resolve=r):Promise.resolve();},cancelDrag:ticket=>{calls.push(['cancel',ticket]);return Promise.resolve({state:'cancelled'});},claimDrag:ticket=>{calls.push(['claim',ticket]);return Promise.resolve({state:'claimed'});},detachDrag:ticket=>{calls.push(['detach',ticket]);return Promise.resolve({state:'claimed'});}};
 const editor={active:{uid:'b'},canReorderDocuments:()=>reason,reorderDocument:(uid,before)=>{calls.push(['reorder',uid,before]);return true;}};
 const drag=create({document,windows:()=>coordinator,editor:()=>editor,crypto:{getRandomValues:bytes=>bytes.fill(0xab)},message:m=>messages.push(m),setTimeout:fn=>fn()});
 const data=()=>({values:new Map(),types:[],effectAllowed:'',dropEffect:'none',clearData(){this.values.clear();this.types=[]},setData(type,value){this.values.set(type,value);this.types=[...this.values.keys()]},getData(type){return this.values.get(type)||''}});
 const event=(target,dt=data(),extra={})=>({target,dataTransfer:dt,button:0,isTrusted:true,clientX:10,prevented:false,preventDefault(){this.prevented=true},stopPropagation(){this.stopped=true},...extra});
 return {drag,calls,messages,a,b,host,strip,outside,data,event,get zone(){return nodes.find(x=>x.kind==='zone')},setReason:r=>reason=r,resolve:()=>resolve?.()};
}
test('native gesture carries only an opaque ticket, never active-tab identity, paths, text or capabilities',async()=>{
 const f=fixture(),dt=f.data();f.drag.begin(f.event(f.a));await flush();f.drag.start(f.event(f.a,dt));await flush();
 assert.equal(f.drag.state.uid,'a');assert.deepEqual([...dt.values.keys()],[MIME]);assert.match(dt.getData(MIME),/^[a-f0-9]{32}$/);assert.equal(dt.effectAllowed,'move');assert.deepEqual(f.calls[0].slice(0,2),['begin','a']);
});
test('fast native drag carries only its ticket while registration is pending; local drop waits safely',async()=>{
 const f=fixture({pending:true}),dt=f.data();f.drag.begin(f.event(f.a));assert.deepEqual(f.calls,[]);const e=f.event(f.a,dt);f.drag.start(e);assert.equal(e.prevented,false);assert.match(dt.getData(MIME),/^[a-f0-9]{32}$/);await flush();
 const dropped=f.drag.drop(f.event(f.b,dt,{clientX:180}));await flush();assert(!f.calls.some(x=>x[0]==='reorder'));f.resolve();await dropped;assert(f.calls.some(x=>x[0]==='reorder'&&x[1]==='a'));
});
test('failed native registration alone cannot move the original or claim a destination',async()=>{
 const f=fixture({reject:true}),dt=f.data();f.drag.begin(f.event(f.a));f.drag.start(f.event(f.a,dt));await flush();f.drag.end(f.event(f.a,dt));assert(!f.calls.some(x=>['claim','detach','reorder'].includes(x[0])));assert(f.messages.length);
});
test('same-window reorder uses gesture UID, not the active document, and works without native windows',async()=>{
 const f=fixture({native:false}),dt=f.data();f.drag.begin(f.event(f.a));f.drag.start(f.event(f.a,dt));await f.drag.drop(f.event(f.b,dt,{clientX:180}));assert.deepEqual(f.calls,[['reorder','a',null]]);assert.equal(dt.dropEffect,'move');assert.equal(f.drag.state.armed,false,'tab redraw cannot leave a stale gesture if removed source misses bubbling dragend');
});
test('native local reorder cancels its unused broker ticket without offering another window',async()=>{
 const f=fixture(),dt=f.data();f.drag.begin(f.event(f.a));await flush();f.drag.start(f.event(f.a,dt));await f.drag.drop(f.event(f.b,dt,{clientX:110}));assert(f.calls.some(x=>x[0]==='reorder'&&x[1]==='a'&&x[2]==='b'));assert(f.calls.some(x=>x[0]==='cancel'));assert(!f.calls.some(x=>['claim','detach'].includes(x[0])));
});
test('foreign text, malformed and foreign MIME data cannot claim or detach a document',async()=>{
 for(const [type,value] of [['text/plain','ab'.repeat(16)],[MIME,'path/to/file'],[MIME,'ab'.repeat(32)]]){const f=fixture(),dt=f.data();dt.setData(type,value);await f.drag.drop(f.event(f.host,dt));assert.deepEqual(f.calls,[]);}
});
test('cross-window drop submits only its opaque ticket to authenticated receiver coordinator',async()=>{
 const f=fixture(),dt=f.data();dt.setData(MIME,'cd'.repeat(16));const e=f.event(f.host,dt);f.drag.over(e);assert.equal(dt.dropEffect,'move');await f.drag.drop(e);assert.deepEqual(f.calls,[['claim','cd'.repeat(16)]]);
});
test('explicit in-app detach target claims new destination; dragend cannot cancel that seen drop',async()=>{
 const f=fixture(),dt=f.data();f.drag.begin(f.event(f.a));await flush();f.drag.start(f.event(f.a,dt));assert.equal(f.zone.hidden,false);await f.drag.drop(f.event(f.zone,dt));f.drag.end(f.event(f.a,dt));assert(f.calls.some(x=>x[0]==='detach'));assert(!f.calls.some(x=>x[0]==='cancel'));
});
test('bare-desktop, foreign target and Escape never infer a new window or release original ownership',async()=>{
 for(const escape of [false,true]){const f=fixture(),dt=f.data();f.drag.begin(f.event(f.a));await flush();f.drag.start(f.event(f.a,dt));if(escape)f.drag.escape({key:'Escape'});await f.drag.drop(f.event(f.outside,dt));f.drag.end(f.event(f.a,dt));assert(f.calls.some(x=>x[0]==='cancel'));assert(!f.calls.some(x=>['claim','detach','reorder'].includes(x[0])));}
});
test('ordinary tab clicks make no broker or checkpoint request',async()=>{
 const f=fixture();for(let i=0;i<100;i++){f.drag.begin(f.event(f.a));f.drag.up();}await flush();assert.deepEqual(f.calls,[]);assert.equal(f.drag.state.armed,false);
});
test('real drag cancellation before registration settles cancels again after late completion',async()=>{
 const f=fixture({pending:true}),dt=f.data();f.drag.begin(f.event(f.a));f.drag.start(f.event(f.a,dt));await flush();f.drag.end(f.event(f.a,dt));f.resolve();await flush();assert(f.calls.filter(x=>x[0]==='cancel').length>=1);assert(!f.calls.some(x=>['claim','detach','reorder'].includes(x[0])));assert.equal(f.drag.state.armed,false);
});
test('protected documents and manufactured dragstart cannot move or reorder',async()=>{
 const f=fixture();f.setReason('Move pending');f.drag.begin(f.event(f.a));assert.deepEqual(f.calls,[]);const e=f.event(f.a,f.data(),{isTrusted:false});f.drag.start(e);assert.equal(e.prevented,true);assert.deepEqual(f.calls,[]);
});

test('pending registration detach is submitted at the actual drop, not after registration',async()=>{
 const f=fixture({pending:true}),dt=f.data();f.drag.begin(f.event(f.a));f.drag.start(f.event(f.a,dt));await flush();await f.drag.drop(f.event(f.zone,dt));assert(f.calls.some(x=>x[0]==='detach'));f.resolve();await flush();
});
test('leaving a prospective receiver clears only its drop highlight',async()=>{
 const f=fixture(),dt=f.data();dt.setData(MIME,'ab'.repeat(16));f.drag.over(f.event(f.host,dt));assert(f.host.classList.values.has('nt-editor-drag-over'));f.drag.leave(f.event(f.outside,dt,{relatedTarget:null}));assert(!f.host.classList.values.has('nt-editor-drag-over'));assert.deepEqual(f.calls,[]);
});
