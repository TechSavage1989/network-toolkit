'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const read=name=>fs.readFileSync(path.join(__dirname,'../web/assets/',name),'utf8');
function between(source,first,last){const a=source.indexOf(first),b=source.indexOf(last,a+first.length);assert(a>=0&&b>a,first+' not found');return source.slice(a,b);}
test('Editor Files switching to the already-selected panel does not remount controls or steal focus',()=>{
 const src=read('editor-files-sidebar.js'),host={hidden:false},calls=[],ctx={panel:'connections',navigationRevision:5,detached:()=>false,document:{querySelector:()=>host},mount:h=>calls.push(['mount',h]),remember:h=>calls.push(['save',h]),expandedIntent(){throw Error('unexpected reveal')},visibility(){throw Error('unexpected visibility')}};
 vm.createContext(ctx);vm.runInContext(between(src,'function selectPanel(value){','\nfunction mount(host){'),ctx);
 ctx.selectPanel('connections');assert.deepEqual(calls,[]);assert.equal(ctx.navigationRevision,6,'same-tab selection still invalidates stale async requests');
 ctx.selectPanel('files');assert.deepEqual(calls.map(x=>x[0]),['mount','save']);assert.equal(ctx.navigationRevision,7);
});
test('Editor connection settings explicitly focus a current visible field without stale second-frame focus',()=>{
 const src=read('remote-browser.js'),frames=[],selected=[],document={activeElement:null,querySelector:()=>null},side={hidden:false};
 const host={isConnected:true,disabled:false,getClientRects:()=>[{}],scrollIntoView(){},focus(){document.activeElement=this;}};
 const app={querySelectorAll:selector=>selector==='[data-rb-setting="host"]'?[host]:[],querySelector:selector=>selector==='[data-rb-setting="host"]'?host:selector==='.rb-connect'?{contains:el=>el===host}:null,closest:()=>side};
 let current=app;
 const root={NetworkToolkitEditorFiles:{selectPanel:page=>selected.push(page)},requestAnimationFrame:fn=>frames.push(fn)},ctx={root,document,detachedEditor:()=>false,rootNode:()=>current};
 vm.createContext(ctx);vm.runInContext(between(src,'function focusSidebarConnectionSettings(k){','\nfunction options('),ctx);
 ctx.focusSidebarConnectionSettings('ssh');
 assert.deepEqual(selected,['connections']);assert.equal(document.activeElement,host);
 const later={};document.activeElement=later;frames.shift()();assert.equal(document.activeElement,later,'a later user focus must win');
 current=null;ctx.focusSidebarConnectionSettings('ssh');assert.equal(document.activeElement,later);
});
test('comparison map preserves fractional target positions rather than rounding each drag to an aligned row',()=>{
 const src=read('editor-aligned-compare.js'),sent=[],geometry={rowOffsets:[0,20,40,60]},ctx={pixelReady:true,geometry,sourceHeights:{},q:()=>({checked:true}),lastSide:'left',independentReveal:false,scrollSource:'left',scrollReconciler:{request:v=>sent.push(v)},E:{lineAtOffset:(rows,pixel)=>Math.floor(pixel/20)},clamp:(n,a,b)=>Math.min(b,Math.max(a,n))};
 vm.createContext(ctx);vm.runInContext(between(src,'function setScroll(side,pixel,','\n function jump('),ctx);
 ctx.setScroll('right',27.625,true);ctx.setScroll('left',-40,false);ctx.setScroll('left',1000);
 assert.deepEqual(sent.map(x=>x.pixel),[27.625,0,60]);assert.deepEqual(sent.map(x=>x.both),[true,false,true]);assert(sent.every(x=>x.kind==='aligned'));
 assert(src.includes('Number.isFinite(intent.pixel)?clamp(intent.pixel'),'reconciler must use exact pixel before row fallback');
});
function explorerHarness(){
 const src=read('transfer-explorer.js'),listeners=new Map(),rootListeners=new Map(),frames=new Map(),attributes={},labels=[],saved=[],m={split:49,single:false};
 let next=0,held=null;const grip={dataset:{},tabIndex:0,isConnected:true,closest(sel){if(sel==='[data-fm-gutter]')return this;if(sel==='[data-fm-explorer]')return pane;return null;},setAttribute:(k,v)=>{attributes[k]=v;},setPointerCapture:id=>held=id,hasPointerCapture:id=>held===id,releasePointerCapture:id=>{if(held===id)held=null;}};
 const area={isConnected:true,getBoundingClientRect:()=>({left:100,width:400})};grip.parentElement=area;
 const pane={dataset:{fmExplorer:'tftp'},style:{setProperty(k,v){labels.push([k,v])}},classList:{toggle(){}},querySelector:s=>s==='[data-fm-gutter]'?grip:s==='[data-fm-single-label]'?{set textContent(v){labels.push(['tag',v])}}:null};
 const document={addEventListener(type,fn){const list=listeners.get(type)||[];list.push(fn);listeners.set(type,list)},querySelector:s=>s==='[data-fm-explorer="tftp"]'?pane:null};
 const root={addEventListener(type,fn){rootListeners.set(type,fn)},requestAnimationFrame(fn){const id=++next;frames.set(id,fn);return id},cancelAnimationFrame:id=>frames.delete(id)};
 const ctx={root,document,state:{activePage:'tftp'},PROTOCOLS:new Set(['tftp','sftp']),currentPage:p=>p==='tftp',get:p=>m,remember:()=>saved.push(m.split)};
 vm.createContext(ctx);
 vm.runInContext(between(src,'function applyLayout(p){','\nfunction setMessage('),ctx);
 vm.runInContext(between(src,'let paneResize=null;','\nhandlers();')+'\nhandlers();',ctx);
 const event=(type,options={})=>{const e={target:grip,pointerId:7,button:0,clientX:296,key:'',shiftKey:false,preventDefault(){this.prevented=true},stopPropagation(){},...options};for(const fn of listeners.get(type)||[])fn(e);return e};
 return {m,grip,attributes,frames,saved,event,blur:()=>rootListeners.get('blur')(),run(){const list=[...frames.values()];frames.clear();list.forEach(fn=>fn())}};
}
test('server file explorer pointer drag coalesces moves and commits the final position once',()=>{
 const h=explorerHarness();h.event('pointerdown');h.event('pointermove',{clientX:330});h.event('pointermove',{clientX:340});assert.equal(h.frames.size,1);h.run();assert.equal(h.m.split,60);
 h.event('pointerup',{clientX:360});assert.equal(h.m.split,65);assert.deepEqual(h.saved,[65]);assert.equal(h.attributes['aria-valuenow'],'65');
 h.event('lostpointercapture');assert.deepEqual(h.saved,[65]);
});
test('cancelled, blurred and stale-pointer file divider gestures restore the previous ratio',()=>{
 const h=explorerHarness();h.event('pointerdown');h.event('pointermove',{clientX:390});h.event('pointermove',{pointerId:3,clientX:180});h.run();assert.equal(h.m.split,72.5);h.event('pointercancel');assert.equal(h.m.split,49);assert.deepEqual(h.saved,[]);
 h.event('pointerdown',{clientX:296});h.event('pointermove',{clientX:400});h.blur();assert.equal(h.m.split,49);assert.equal(h.frames.size,0);
});
test('file divider keyboard and double click offer consistent accessible limits and reset',()=>{
 const h=explorerHarness(),press=(key,shiftKey=false)=>h.event('keydown',{key,shiftKey});
 press('ArrowRight');assert.equal(h.m.split,51);press('ArrowLeft',true);assert.equal(h.m.split,41);press('End');assert.equal(h.m.split,75);press('Home');assert.equal(h.m.split,25);h.event('dblclick');assert.equal(h.m.split,49);
 assert.equal(h.saved.length,5);assert.equal(h.attributes['aria-valuemin'],undefined);assert.equal(h.attributes['aria-valuenow'],'49');
 assert.match(h.attributes['aria-valuetext'],/49% computer pane/);
 h.m.single=true;h.m.split=49;h.event('keydown',{key:'ArrowLeft'});assert.equal(h.m.split,49);
});
