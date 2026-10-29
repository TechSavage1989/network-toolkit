'use strict';
const test=require('node:test'),assert=require('node:assert/strict');
const {create,MIME}=require('../web/assets/editor-drag.js');
const flush=()=>new Promise(resolve=>setImmediate(resolve));
function fixture({native=true,pending=false,reject=false}={}){
 const calls=[],messages=[],nodes=[],classes=()=>({values:new Set(),add(v){this.values.add(v)},remove(v){this.values.delete(v)}});
 const node=(kind,uid)=>{const n={kind,style:{},dataset:uid?{edUid:uid}:{},classList:classes(),hidden:false,setAttribute(k,v){this[k]=v},removeAttribute(k){delete this[k]},contains(x){return nodes.includes(x)},append(x){nodes.push(x)},getBoundingClientRect(){return {left:uid==='a'?0:100,width:100}},closest(selector){if(selector==='[data-ed-uid]')return kind==='tab'?this:null;if(selector==='[data-ed-tabs]')return ['tab','strip'].includes(kind)?strip:null;if(selector==='[data-editor-drag-detach]')return kind==='zone'?this:null;if(selector==='.nt-editor-window')return kind==='outside'?null:host;return null;}};nodes.push(n);return n;};
 const host=node('host'),strip=node('strip'),a=node('tab','a'),b=node('tab','b'),outside=node('outside');a.nextElementSibling=b;b.nextElementSibling=null;
 host.querySelector=selector=>selector.includes('"a"')?a:selector.includes('"b"')?b:null;
 const document={querySelector:selector=>selector==='.nt-editor-window'?host:null,querySelectorAll:selector=>selector==='[data-editor-drop-before]'?nodes.filter(x=>Object.hasOwn(x,selector.slice(1,-1))):[host],createElement:()=>node('zone'),addEventListener(){}};
 let reason='',resolve;
 const coordinator={state:{enabled:native},beginDrag:(uid,ticket)=>{calls.push(['begin',uid,ticket]);return reject?Promise.reject(Error('ownership unavailable')):pending?new Promise(r=>resolve=r):Promise.resolve();},cancelDrag:ticket=>{calls.push(['cancel',ticket]);return Promise.resolve({state:'cancelled'});},claimDrag:ticket=>{calls.push(['claim',ticket]);return Promise.resolve({state:'claimed'});},detachDrag:ticket=>{calls.push(['detach',ticket]);return Promise.resolve({state:'claimed'});}};
 const editor={active:{uid:'b'},canReorderDocuments:()=>reason,reorderDocument:(uid,before)=>{calls.push(['reorder',uid,before]);return true;}};
 const drag=create({document,windows:()=>coordinator,editor:()=>editor,crypto:{getRandomValues:bytes=>bytes.fill(0xab)},message:m=>messages.push(m),setTimeout:fn=>fn()});
 const data=()=>({values:new Map(),types:[],effectAllowed:'',dropEffect:'none',clearData(){this.values.clear();this.types=[]},setData(type,value){this.values.set(type,value);this.types=[...this.values.keys()]},getData(type){return this.values.get(type)||''}});
 const event=(target,dt=data(),extra={})=>({target,dataTransfer:dt,button:0,isTrusted:true,altKey:true,clientX:10,prevented:false,preventDefault(){this.prevented=true},stopPropagation(){this.stopped=true},...extra});
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


test('the detach target follows visible tab rows and cannot cover a compact view tab strip',()=>{
 const f=fixture();let bottom=116;f.host.getBoundingClientRect=()=>({top:10,height:600});f.host.querySelectorAll=()=>[{setAttribute(){},getBoundingClientRect:()=>({width:500,height:36,bottom})},{setAttribute(){},getBoundingClientRect:()=>({width:0,height:0,bottom:0})}];
 f.drag.mount();assert.equal(f.zone.style.top,'118px');bottom=220;f.drag.mount();assert.equal(f.zone.style.top,'222px');
});

test('tab detach and cancelled native gestures preserve pane scroll despite browser edge autoscroll',async()=>{
 for(const cancel of [false,true]){
  const f=fixture(),ta={scrollTop:100,scrollLeft:17,isConnected:true};f.host.querySelectorAll=selector=>selector==='[data-ed-text],[data-compare-text],.cm-scroller'?[ta]:[];
  const dt=f.data();f.drag.begin(f.event(f.a));f.drag.start(f.event(f.a,dt));await flush();ta.scrollTop=82;ta.scrollLeft=0;
  if(cancel)f.drag.escape({key:'Escape'});else await f.drag.drop(f.event(f.zone,dt));
  assert.equal(ta.scrollTop,100);assert.equal(ta.scrollLeft,17);ta.scrollTop=80;f.drag.end(f.event(f.a,dt));assert.equal(ta.scrollTop,100);
 }
});

function pointerFixture(){
 const calls=[],timers=[],classes=()=>({add(){},remove(){}}),all=[];let blocked='',capture=null;
 const docs=new Map([['a',{uid:'a',content:'DIRTY_A',dirty:true,undo:['BEFORE_A']}],['b',{uid:'b',content:'DIRTY_B',dirty:true,undo:['BEFORE_B']}] ]),owned={a:1,b:2};
 const host={contains:()=>true,querySelectorAll:()=>[],classList:classes()},strip={isConnected:true,classList:classes(),setAttribute(){},getBoundingClientRect:()=>({left:0,right:200,top:30,bottom:60,width:200,height:30}),setPointerCapture:id=>capture=id,hasPointerCapture:id=>capture===id,releasePointerCapture:()=>capture=null,closest:s=>s==='.nt-editor-window'?host:s==='[data-ed-view]'?{dataset:{edView:'left'}}:null};
 const tab=(uid,x)=>({isConnected:true,draggable:true,dataset:{edUid:uid},classList:classes(),setAttribute(k,v){this[k]=v},removeAttribute(k){delete this[k]},getBoundingClientRect:()=>({left:x,right:x+100,width:100,top:30,bottom:60}),closest(s){return s==='[data-ed-uid]'?this:s==='[data-ed-tabs]'?strip:s==='[data-ed-view]'?{dataset:{edView:'left'}}:s==='.nt-editor-window'?host:null;}});
 const a=tab('a',0),b=tab('b',100);a.nextElementSibling=b;b.nextElementSibling=null;all.push(a,b);strip.querySelectorAll=()=>all;
 let hit=strip;strip.closest=s=>s==='[data-ed-tabs]'?strip:s==='.nt-editor-window'?host:s==='[data-ed-view]'?{dataset:{edView:'left'}}:null;
 const listeners={},body={append(){}},document={body,querySelector:s=>s==='.nt-editor-window'?host:null,querySelectorAll:()=>all,elementFromPoint:()=>hit,createElement:()=>({style:{},dataset:{},setAttribute(){},remove(){}}),addEventListener(name,callback){listeners[name]=callback;}};host.append=()=>{};
 const coordinator={state:{enabled:true,owned},move:async(ids,dest)=>calls.push(['move',ids,dest]),beginDrag:async()=>calls.push(['begin']),cancelDrag:async()=>calls.push(['cancel'])};
 const editor={documents:docs,active:docs.get('b'),canReorderDocuments:()=>blocked,reorderDocument:(uid,before)=>calls.push(['reorder',uid,before]),moveDocumentToView:(uid,view)=>calls.push(['view',uid,view])};
 const drag=create({document,windows:()=>coordinator,editor:()=>editor,setTimeout:fn=>timers.push(fn),message:msg=>calls.push(['message',msg]),crypto:{getRandomValues:b=>b.fill(1)}});
 const event=(target=a,x=10,y=40,extra={})=>({target,pointerId:1,pointerType:'mouse',button:0,isPrimary:true,isTrusted:true,clientX:x,clientY:y,preventDefault(){this.prevented=true},stopPropagation(){this.stopped=true},...extra});
 return {drag,a,b,strip,host,docs,owned,calls,event,coordinator,listeners,setBlocked:v=>blocked=v,setHit:v=>hit=v,get capture(){return capture},timers};
}
test('ordinary captured-pointer clicks never register, move, or reorder',()=>{const f=pointerFixture();f.drag.begin(f.event());assert.equal(f.capture,1);assert.equal(f.a.draggable,false);f.drag.up(f.event());assert.equal(f.capture,null);assert.equal(f.a.draggable,true);assert.deepEqual(f.calls,[]);});
test('same-view inactive-tab pointer reorder uses fixed UID and never activates or rebuilds its view',()=>{const f=pointerFixture();f.drag.begin(f.event());f.drag.pointerMove(f.event(f.b,180,40));assert.deepEqual(f.calls,[]);f.drag.up(f.event(f.b,180,40));assert.deepEqual(f.calls,[['reorder','a',null]]);assert.equal(f.drag.state.armed,false);assert.equal(f.capture,null);});
test('ordinary drag-away needs moved genuine pointer release; no broker or drop box',async()=>{const f=pointerFixture();const before=JSON.stringify([...f.docs]);f.drag.begin(f.event());f.drag.pointerMove(f.event(f.a,350,150));assert.deepEqual(f.calls,[]);f.drag.up(f.event(f.a,350,150));await flush();assert.deepEqual(f.calls,[['move',['a'],'new']]);assert.equal(JSON.stringify([...f.docs]),before);});
test('Escape, pointer cancellation, lost capture and wrong pointer never detach',async()=>{
 for(const mode of ['escape','cancel','lostcapture','wrong-pointer']){const f=pointerFixture();f.drag.begin(f.event());f.drag.pointerMove(f.event(f.a,350,150));if(mode==='escape')f.drag.escape({key:'Escape'});else if(mode==='cancel')f.listeners.pointercancel(f.event());else if(mode==='lostcapture')f.listeners.lostpointercapture(f.event());else f.drag.up(f.event(f.a,350,150,{pointerId:2}));if(mode==='wrong-pointer')f.drag.abortPointer();f.drag.up(f.event(f.a,350,150));await flush();assert(!f.calls.some(c=>c[0]==='move'));assert.equal(f.capture,null);}
});
test('document replacement, lease change or busy guard during drag retain original',async()=>{
 for(const change of [f=>f.docs.set('a',{...f.docs.get('a')}),f=>f.owned.a++,f=>f.setBlocked('Move pending')]){const f=pointerFixture();f.drag.begin(f.event());f.drag.pointerMove(f.event(f.a,350,150));change(f);f.drag.up(f.event(f.a,350,150));await flush();assert(!f.calls.some(c=>['move','reorder'].includes(c[0])));}
});
test('microtask ownership change after release cannot move a replacement document',async()=>{const f=pointerFixture();f.drag.begin(f.event());f.drag.pointerMove(f.event(f.a,350,150));f.drag.up(f.event(f.a,350,150));f.docs.set('a',{...f.docs.get('a')});await flush();assert(!f.calls.some(c=>c[0]==='move'));assert(f.calls.some(c=>c[0]==='message'));});
test('ordinary pointer mode prevents HTML5 dragstart and keeps captured release semantics',async()=>{const f=pointerFixture();f.drag.begin(f.event());const e=f.event(f.a,20,40,{dataTransfer:{}});f.drag.start(e);assert.equal(e.prevented,true);f.drag.pointerMove(f.event(f.a,350,150));f.drag.up(f.event(f.a,350,150));await flush();assert.deepEqual(f.calls,[['move',['a'],'new']]);});
test('touch, manufactured and control/meta-modified gestures do not arm or make API calls',()=>{for(const extra of [{pointerType:'touch'},{isTrusted:false},{ctrlKey:true},{metaKey:true},{isPrimary:false}]){const f=pointerFixture();f.drag.begin(f.event(f.a,10,40,extra));assert.equal(f.drag.state.armed,false);assert.deepEqual(f.calls,[]);}});
test('no native capability means ordinary reorder works and detach is safely explained',async()=>{const f=pointerFixture();f.coordinator.state.enabled=false;f.drag.begin(f.event());f.drag.pointerMove(f.event(f.a,350,150));f.drag.up(f.event(f.a,350,150));await flush();assert(!f.calls.some(c=>c[0]==='move'));assert(f.calls.some(c=>c[0]==='message'));});

test('pointer drop into a different editor view invokes the existing view move once',()=>{const f=pointerFixture(),other={getBoundingClientRect:()=>({left:250,right:450,top:30,bottom:60,width:200,height:30}),querySelectorAll:()=>[],closest(s){return s==='[data-ed-tabs]'?this:s==='[data-ed-view]'?{dataset:{edView:'right'}}:s==='.nt-editor-window'?f.host:null;}};f.setHit(other);f.drag.begin(f.event());f.drag.pointerMove(f.event(f.a,270,40));f.drag.up(f.event(f.a,270,40));assert.deepEqual(f.calls,[['view','a','right'],['reorder','a',null]]);});
