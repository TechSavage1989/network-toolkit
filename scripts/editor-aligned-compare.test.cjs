'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const api=require('../web/assets/editor-aligned-compare.js');
const context=vm.createContext({console,setTimeout,clearTimeout},{codeGeneration:{strings:false,wasm:false}});
vm.runInContext(fs.readFileSync(require.resolve('../web/assets/vendor/codemirror/codemirror.js'),'utf8'),context);
const C=context.NetworkToolkitCodeMirror;
function harness(text='alpha\n😀 beta\nlast'){
 const document={uid:'synthetic',content:text,dirty:false,undo:[],redo:[],editRevision:0},scroll={scrollTop:0,scrollLeft:0,clientHeight:100,clientWidth:300,scrollHeight:500,scrollWidth:600,style:{},getBoundingClientRect:()=>({width:300,height:100})},input={classList:{toggle(){}},textContent:'not the source'};
 let writable=true,focused=0,reflecting=0;
 function write(value){if(!writable||document.content===value)return;document.undo.push(document.content);document.redo=[];document.content=value;document.dirty=true;document.editRevision++;}
 const view={state:C.EditorState.create({doc:text}),scrollDOM:scroll,contentDOM:input,focus(){focused++},dispatch(spec){const tr=this.state.update(spec);this.state=tr.state;if(tr.docChanged&&!reflecting)write(tr.newDoc.toString());}};
 function reflect(){reflecting++;try{if(view.state.doc.toString()!==document.content)view.dispatch({changes:{from:0,to:view.state.doc.length,insert:document.content}});}finally{reflecting--;}}
 const source=api.createSourceView(view,()=>document,write,reflect,()=>writable);
 return {source,view,document,scroll,input,get focused(){return focused},set writable(value){writable=value},undo(){document.redo.push(document.content);document.content=document.undo.pop();reflect()},redo(){document.undo.push(document.content);document.content=document.redo.pop();reflect()}};
}
test('source facade exposes only exact authoritative content, never rendered decorations or gaps',()=>{const h=harness();h.input.textContent='alpha\nVISUAL GAP\n😀 beta';assert.equal(h.source.value,'alpha\n😀 beta\nlast');assert.equal(h.source.inputElement,h.input);assert.equal(h.source.scrollElement,h.scroll);assert.equal(h.document.undo.length,0);assert.deepEqual(api.starts(h.source.value),[0,6,14]);});
test('source-facing application edit records once and does not create secondary editor history',()=>{const h=harness();h.source.value='new\n😀';assert.equal(h.source.value,'new\n😀');assert.equal(h.view.state.doc.toString(),h.source.value);assert.equal(h.document.undo.length,1);assert.equal(h.document.dirty,true);h.source.value=h.source.value;assert.equal(h.document.undo.length,1);h.undo();assert.equal(h.source.value,'alpha\n😀 beta\nlast');assert.equal(h.view.state.doc.toString(),h.source.value);h.redo();assert.equal(h.source.value,'new\n😀');});
test('native source transaction and application undo retain Unicode UTF16 offsets',()=>{const h=harness();h.view.dispatch({changes:{from:6,to:8,insert:'Ω'},selection:{anchor:7}});assert.equal(h.source.value,'alpha\nΩ beta\nlast');assert.equal(h.source.selectionStart,7);assert.equal(h.document.undo.length,1);h.undo();assert.equal(h.source.value,'alpha\n😀 beta\nlast');});
test('selection facade clamps source offsets and preserves backward direction',()=>{const h=harness();h.source.setSelectionRange(6,13,'backward');assert.equal(h.source.selectionStart,6);assert.equal(h.source.selectionEnd,13);assert.equal(h.source.selectionDirection,'backward');h.source.setSelectionRange(99,4);assert.equal(h.source.selectionStart,4);assert.equal(h.source.selectionEnd,4);h.source.setSelectionRange(-4,999);assert.equal(h.source.selectionStart,0);assert.equal(h.source.selectionEnd,h.source.value.length);});
test('setRangeText select/start/end operate exclusively in source coordinates',()=>{for(const [mode,expected]of[['select',[2,4]],['start',[2,2]],['end',[4,4]]]){const h=harness('abcdef');h.source.setRangeText('XY',2,5,mode);assert.equal(h.source.value,'abXYf');assert.deepEqual([h.source.selectionStart,h.source.selectionEnd],expected);assert.equal(h.document.undo.length,1);}const h=harness();assert.throws(()=>h.source.setRangeText('x',5,2),RangeError);});
test('setRangeText preserve retains selections before, inside, across and after replacement',()=>{for(const [selection,expected]of[[[0,1],[0,1]],[[3,4],[2,4]],[[1,8],[1,7]],[[7,9],[6,8]]]){const h=harness('0123456789');h.source.setSelectionRange(...selection,'backward');h.source.setRangeText('XY',2,5);assert.equal(h.source.value,'01XY56789');assert.deepEqual([h.source.selectionStart,h.source.selectionEnd],expected);assert.equal(h.source.selectionDirection,'backward');}});
test('frozen source rejects mutation while view/scroll/focus remain source-safe',()=>{const h=harness();h.writable=false;assert.equal(h.source.disabled,true);h.source.setRangeText('x');h.source.value='not allowed';assert.equal(h.source.value,'alpha\n😀 beta\nlast');assert.equal(h.document.undo.length,0);h.source.scrollTop=42;h.source.scrollLeft=9;assert.deepEqual([h.scroll.scrollTop,h.scroll.scrollLeft],[42,9]);h.source.focus();assert.equal(h.focused,1);});
function stampFixture(){const docs={left:{uid:'a',content:'same',editRevision:1},right:{uid:'b',content:'other',editRevision:2}},stamp=Object.fromEntries(Object.entries(docs).map(([side,d])=>[side,{d,text:d.content,revision:d.editRevision}])),state={disposed:false,generation:7,composing:false};return {docs,stamp,state,valid:()=>api.presentationCurrent(state,7,stamp,side=>docs[side])};}
test('presentation guard accepts only the exact generation and source identity/revision/text',()=>{const h=stampFixture();assert(h.valid());h.state.generation++;assert(!h.valid());h.state.generation--;h.docs.left.content='edited';assert(!h.valid());h.docs.left.content='same';h.docs.left.editRevision++;assert(!h.valid());h.docs.left.editRevision--;h.docs.left={...h.docs.left};assert(!h.valid());});
test('IME and disposal reject delayed layout/syntax while source snapshots remain untouched',()=>{const h=stampFixture(),before=JSON.stringify(h.docs);h.state.composing=true;assert(!h.valid());h.state.composing=false;assert(h.valid());h.state.disposed=true;assert(!h.valid());assert.equal(JSON.stringify(h.docs),before);});
test('source contract uses no storage/network or shadow textarea and keeps existing diff separate',()=>{const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');assert(!/\b(?:fetch|XMLHttpRequest|WebSocket|localStorage|sessionStorage)\b/.test(source));assert(!/<textarea/i.test(source));assert(source.includes('E.compare(left.content,right.content)'));assert(source.includes('decorationEffect.of(Decoration.set(decorations,true))'));assert(source.includes("block:true"));assert(!source.includes('history()'));});

test('document boundary commands explicitly reveal even an unchanged caret and extend in source offsets',()=>{const h=harness(),effects=[];const reveal=(position,options)=>{effects.push({position,options});return C.StateEffect.define().of(position)};assert.equal(h.source.selectionStart,0);assert.equal(api.moveToDocumentEdge(h.view,false,false,reveal),true);assert.deepEqual(effects.at(-1),{position:0,options:{y:'start',yMargin:0}});assert.equal(h.source.selectionStart,0);api.moveToDocumentEdge(h.view,true,true,reveal);assert.deepEqual([h.source.selectionStart,h.source.selectionEnd],[0,h.source.value.length]);assert.equal(effects.at(-1).options.y,'end');h.source.setSelectionRange(6,6);api.moveToDocumentEdge(h.view,false,true,reveal);assert.deepEqual([h.source.selectionStart,h.source.selectionEnd],[0,6]);assert.equal(h.source.selectionDirection,'backward');api.moveToDocumentEdge(h.view,true,false,reveal);assert.equal(h.source.selectionStart,h.source.value.length);assert.equal(h.document.undo.length,0);assert.equal(h.document.dirty,false);});

test('hunk navigation reveals the source line start rather than trusting estimated virtual pixel height',()=>{const h=harness(),effects=[];const reveal=(position,options)=>{effects.push({position,options});return C.StateEffect.define().of(position)};api.revealSourceRange(h.view,6,13,reveal);assert.deepEqual([h.source.selectionStart,h.source.selectionEnd],[6,13]);assert.deepEqual(effects[0],{position:6,options:{y:'start',yMargin:0}});api.revealSourceRange(h.view,6,13,reveal);assert.equal(effects.length,2,'already selected hunk must still explicitly reveal its start');assert.equal(h.document.undo.length,0);assert.equal(h.source.value,'alpha\n😀 beta\nlast');});

function blockView(middleHeight){const doc=C.EditorState.create({doc:'a\nb\nc'}).doc,gap={alignedStart:20,height:20},blocks=[{from:0,to:1,top:0,height:20},{from:1,to:1,top:20,height:20,widget:gap},{from:2,to:3,top:40,height:middleHeight},{from:4,to:5,top:40+middleHeight,height:20}];for(const block of blocks){block.bottom=block.top+block.height;block.type=block.widget?1:0;}return {state:{doc},elementAtHeight(pixel){return blocks.find(b=>pixel<b.bottom)||blocks.at(-1)},lineBlockAt(position){if(position===doc.line(1).to)return {type:[blocks[0],blocks[1]]};const line=doc.lineAt(position).number;if(line===1)return blocks[0];if(line===2)return blocks[2];return blocks[3];}};}
test('shared source geometry never copies unequal virtual native scroll offsets',()=>{const left=blockView(200),right=blockView(60),offsets=[0,40,100,120],heights=[20,60,20];const aligned=api.alignedPixelFromNative(left,offsets,heights,100);assert.equal(aligned,58);assert.equal(api.nativePixelFromAligned(right,offsets,heights,aligned),58);assert.equal(api.nativePixelFromAligned(left,offsets,heights,aligned),100);assert.equal(api.alignedPixelFromNative(right,offsets,heights,58),58);assert.notEqual(api.nativePixelFromAligned(right,offsets,heights,aligned),100);});
test('internal comparison gaps belong after their source line, not before the next gutter row',()=>{const view=blockView(200),doc=view.state.doc,gapAt=doc.line(1).to,nextAt=doc.line(2).from;assert.equal(gapAt,1);assert.equal(nextAt,2);assert.equal(view.lineBlockAt(gapAt).type[1].widget.alignedStart,20);assert.equal(view.lineBlockAt(nextAt).widget,undefined);for(const pixel of [20.125,25.25,39.875]){const native=api.nativePixelFromAligned(view,[0,40,100,120],[20,60,20],pixel);assert(Math.abs(api.alignedPixelFromNative(view,[0,40,100,120],[20,60,20],native)-pixel)<.001);}const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');assert(source.includes('gapPosition=source.to;'),'map gaps at the preceding real source boundary');assert(source.includes('const pos=to;list.push(Decoration.widget(')&&source.includes('block:true,side:1}).range(pos)'),'attach source-free padding after the line, so the following native gutter row is not a composite block');});
test('source geometry maps actual blank widgets without inventing numbered source lines',()=>{const view=blockView(200),offsets=[0,40,100,120],heights=[20,60,20];for(const pixel of [0,10,20,25,39,40,50,99,100,110,120]){const native=api.nativePixelFromAligned(view,offsets,heights,pixel);assert(Math.abs(api.alignedPixelFromNative(view,offsets,heights,native)-pixel)<.001);}assert.equal(api.nativePixelFromAligned(view,offsets,heights,30),30);assert.equal(view.state.doc.toString(),'a\nb\nc');});
test('leading and trailing source-free gaps retain continuous map coordinates',()=>{const doc=C.EditorState.create({doc:'one'}).doc,blocks=[{from:0,to:0,top:0,height:30,widget:{alignedStart:0,height:30}},{from:0,to:3,top:30,height:80},{from:3,to:3,top:110,height:30,widget:{alignedStart:70,height:30}}];for(const b of blocks){b.bottom=b.top+b.height;b.type=b.widget?1:0;}const view={state:{doc},elementAtHeight:y=>blocks.find(b=>y<b.bottom)||blocks.at(-1),lineBlockAt:()=>({type:blocks})};for(const p of [0,15,29,30,50,69,70,85,100]){const native=api.nativePixelFromAligned(view,[30,100],[40],p);assert(Math.abs(api.alignedPixelFromNative(view,[30,100],[40],native)-p)<.001);}assert.equal(doc.toString(),'one');});

test('focusable map keyboard and sync checkbox never read absent, limited or invalidated geometry',()=>{const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8'),key=source.match(/track.addEventListener\('keydown',event=>\{([\s\S]*?)\}\);\}/)[1],change=source.match(/host.addEventListener\('change',event=>\{([\s\S]*?)\}\);\n for\(const side of sides\)/)[1];for(const state of [{pixelReady:false,geometry:null,sourceHeights:null},{pixelReady:true,geometry:null,sourceHeights:{left:[20]}},{pixelReady:false,geometry:{rowOffsets:[0,20],sourceOffsets:{left:[999]}},sourceHeights:{left:[20]}}]){let reads=0;const context={...state,side:'left',lastSide:'left',views:new Proxy({},{get(){reads++;throw Error('stale native view read')}}),alignedPixelFromNative(){reads++;throw Error('stale mapping')},setScroll(){reads++},isComposing:()=>false,q:()=>({checked:true}),event:{key:'PageDown',preventDefault(){},stopPropagation(){},target:{dataset:{},matches:selector=>selector==='[data-compare-sync]'}}};vm.runInNewContext('(event=>{'+key+'})(event)',context);vm.runInNewContext('(event=>{'+change+'})(event)',context);assert.equal(reads,0);}});

function scrollQueue(read,write){const queued=[],outcomes=[],sync=api.createScrollReconciler(job=>queued.push(job),read,write,done=>outcomes.push(done));return {sync,queued,outcomes,step(){const job=queued.shift();assert(job,'a measurement is queued');const value=job.read();job.write(value)},flush(){let count=0;while(queued.length){assert(++count<=30,'bounded reconciliation');this.step();}return count;}};}
test('one source intent survives unavailable resize geometry and resumes only from the current measurement',()=>{let ready=false,target=60,actual=0,reads=0,writes=0;const q=scrollQueue(intent=>{reads++;return ready?{target:intent.side==='left'?target:90,actual}:null},value=>{writes++;if(value.target===actual)return false;actual=value.target;return true});q.sync.request({side:'left'});q.flush();assert.equal(writes,0);ready=true;q.sync.resume();q.flush();assert.equal(actual,60);assert.deepEqual(q.outcomes,[true]);assert.equal(writes,3);q.sync.pause();ready=false;q.sync.request({side:'right'});q.flush();assert.equal(actual,60);ready=true;q.sync.resume();q.flush();assert.equal(actual,90);assert(reads>=7);});
test('peer source target is remeasured after a virtual wrapped block changes its estimate',()=>{const offsets=[0,40,100,120],heights=[20,60,20],source=blockView(60);let target=blockView(200),actual=0,writes=0;const anchor=58,q=scrollQueue(()=>({target:api.nativePixelFromAligned(target,offsets,heights,anchor),actual}),value=>{if(Math.abs(value.actual-value.target)<=.5)return false;actual=value.target;if(++writes===1)target=blockView(60);return true});q.sync.request({side:'left'});q.flush();assert.equal(writes,2,'the estimated first write is corrected after layout refines');assert.equal(actual,58);assert.equal(api.alignedPixelFromNative(target,offsets,heights,actual),api.alignedPixelFromNative(source,offsets,heights,58));assert.equal(source.state.doc.toString(),'a\nb\nc');assert.equal(target.state.doc.toString(),'a\nb\nc');assert.deepEqual(q.outcomes,[true]);});
test('new intent, geometry invalidation, document replacement and disposal reject queued stale writes',()=>{let actual=0;const q=scrollQueue(intent=>({target:intent.pixel}),value=>{const changed=actual!==value.target;actual=value.target;return changed});q.sync.request({pixel:50});let stale=q.queued.shift(),read=stale.read();q.sync.request({pixel:80});stale.write(read);assert.equal(actual,0);q.flush();assert.equal(actual,80);q.sync.request({pixel:100});stale=q.queued.shift();read=stale.read();q.sync.pause();stale.write(read);assert.equal(actual,80);q.sync.resume();q.flush();assert.equal(actual,100);q.sync.request({pixel:120});stale=q.queued.shift();read=stale.read();q.sync.cancel();stale.write(read);assert.equal(actual,100);q.sync.resume();assert.equal(q.queued.length,0);});
test('reconciliation requires consecutive stable measurements and is bounded for an unstable viewport',()=>{let writes=0;const q=scrollQueue(()=>({}),()=>{writes++;return true});q.sync.request({side:'left'});assert.equal(q.flush(),8);assert.equal(writes,8);assert.deepEqual(q.outcomes,[false]);const stable=scrollQueue(()=>({}),()=>false);stable.sync.request({side:'left'});assert.equal(stable.flush(),2);assert.deepEqual(stable.outcomes,[true]);});
test('source reconciliation preserves only the latest real intent through invalidation without raw pixel copying',()=>{const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');assert(source.includes("scrollReconciler.request({kind:'source',side})"));assert(source.includes('pixelReady=true;drawMap();scrollReconciler.resume()'));assert(source.includes('function invalidatePixels(){scrollReconciler.pause()'));assert(source.includes('if(sourceChanged){scrollReconciler.cancel()'));assert(source.includes('splitter.dispose();scrollReconciler.cancel()'));assert(!source.includes('views[other].scrollDOM.scrollTop='));});
test('map and hunk intents retain aligned row fraction rather than obsolete pixels after wrapping changes',()=>{const E=require('../web/assets/editor-compare.js'),before=[0,20,60,80],after=[0,60,180,200],row=E.lineAtOffset(before,40);assert.equal(row,1.5);assert.equal(E.offsetAtLine(after,row),120);const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');assert(source.includes("row:E.lineAtOffset(geometry.rowOffsets,Math.max(0,pixel))"));assert(source.includes('E.offsetAtLine(geometry.rowOffsets,intent.row)'));});

test('late native height refinement re-arms the retained source intent after initial convergence',()=>{
 const offsets=[0,40,100,120],heights=[20,60,20],anchor=58;let view=blockView(200),actual=0;
 const q=scrollQueue(()=>({target:api.nativePixelFromAligned(view,offsets,heights,anchor),actual}),value=>{const changed=Math.abs(value.actual-value.target)>.5;if(changed)actual=value.target;return changed});
 q.sync.request({kind:'source',side:'left'});q.flush();assert.equal(actual,100);assert.deepEqual(q.outcomes,[true]);
 view=blockView(60);q.sync.refine();q.sync.refine();q.sync.refine();assert.equal(q.queued.length,1,'coalesce repeated native geometry notifications');q.flush();assert.equal(actual,58);assert.equal(api.alignedPixelFromNative(view,offsets,heights,actual),anchor);assert.deepEqual(q.outcomes,[true,true]);
 q.sync.refine();q.flush();assert.equal(actual,58,'already aligned native geometry never produces a new scroll');assert.equal(view.state.doc.toString(),'a\nb\nc');
});
test('late refinements cannot resurrect cancelled intent or overwrite a newer real input',()=>{
 let actual=0;const q=scrollQueue(intent=>({target:intent.pixel}),value=>{const changed=actual!==value.target;actual=value.target;return changed});
 q.sync.request({pixel:40});q.flush();q.sync.refine();const stale=q.queued.shift(),value=stale.read();q.sync.request({pixel:80});stale.write(value);q.flush();assert.equal(actual,80);
 q.sync.cancel();q.sync.refine();assert.equal(q.queued.length,0,'document replacement and disposal remain final');
 const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');assert(source.includes('if(update.geometryChanged&&pixelReady&&!isComposing()){scrollReconciler.refine()'),'wake only on actual native geometry changes while ready, not every viewport scroll');
});

test('native gutter refresh is coalesced and idempotent for unchanged measured viewport blocks',()=>{
 const jobs=[],deferred=[],writes=[];let signature='current',ready=true;const refresh=api.createMeasuredRefresh(job=>jobs.push(job),()=>ready?{signature}:null,value=>writes.push(value.signature),()=>true,fn=>deferred.push(fn));
 const flush=()=>{while(jobs.length){const job=jobs.shift();job.write(job.read());}while(deferred.length)deferred.shift()();};
 refresh.request();refresh.request();assert.equal(jobs.length,1);flush();assert.deepEqual(writes,['current']);refresh.request();flush();assert.deepEqual(writes,['current'],'config refresh must not trigger itself');
 signature='late height refinement';refresh.request();flush();assert.deepEqual(writes,['current','late height refinement']);ready=false;refresh.request();flush();assert.equal(writes.length,2);
});
test('native gutter refresh never dispatches from a measurement write or after stale source/disposal',()=>{
 const jobs=[],deferred=[];let writes=0,valid=true;const refresh=api.createMeasuredRefresh(job=>jobs.push(job),()=>({signature:String(jobs.length)+':geometry'}),()=>writes++,()=>valid,fn=>deferred.push(fn));
 refresh.request();let job=jobs.shift();job.write(job.read());assert.equal(writes,0,'defer public reconfiguration until the native update has completed');valid=false;deferred.shift()();assert.equal(writes,0);
 refresh.cancel();valid=true;refresh.request();job=jobs.shift();job.write(job.read());refresh.cancel();deferred.shift()();assert.equal(writes,0,'disposal cancels an already measured refresh');
 refresh.request();job=jobs.shift();const old=job.read();refresh.cancel();job.write(old);assert.equal(deferred.length,0,'stale measured geometry cannot queue a write');
});
test('native gutter signature tracks current viewport and block geometry, not cached DOM labels',()=>{
 const view={viewport:{from:0,to:8},contentDOM:{clientWidth:120},documentPadding:{top:10},viewportLineBlocks:[{from:0,to:3,top:0,height:20,type:0},{from:4,to:8,top:20,height:40,type:0}]};const before=api.nativeGutterSignature(view);assert.equal(api.nativeGutterSignature(view),before);view.viewportLineBlocks[1].height=60;assert.notEqual(api.nativeGutterSignature(view),before);const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');assert(source.includes('gutterConfig[side].reconfigure(lineNumbers({formatNumber:line=>String(line)}))'));assert(source.includes('gutterRefresh[side].cancel()'));assert(!source.includes("querySelectorAll('.cm-gutterElement')"));
});

test('continuous same-owner input coalesces before measurement without starving every pending frame',()=>{
 let actual=0,writes=0;const q=scrollQueue(intent=>({target:intent.pixel}),value=>{writes++;actual=value.target;return true;});
 for(let frame=1;frame<=60;frame++){
  q.sync.request({kind:'source',side:'right',pixel:frame*10});
  q.sync.request({kind:'source',side:'right',pixel:frame*10+3});
  assert.equal(q.queued.length,1,'only one measurement may be outstanding during a continuous gesture');
  q.step();assert.equal(actual,frame*10+3,'peer progresses during each frame, not only after input ends');
 }
 assert.equal(writes,60);q.sync.cancel();q.flush();
});
test('repeated map targets use the latest fractional aligned intent without accumulated stale jobs',()=>{
 let actual=0;const scheduled=[],q=scrollQueue(intent=>({target:intent.row}),value=>{actual=value.target;return false;});
 for(let frame=0;frame<40;frame++){
  for(let sample=0;sample<8;sample++)q.sync.request({kind:'aligned',side:'left',row:frame+sample/8});
  assert.equal(q.queued.length,1);q.step();assert.equal(actual,frame+7/8);scheduled.push(actual);
 }
 assert(scheduled.every((value,i)=>!i||value>scheduled[i-1]));q.sync.cancel();q.flush();
});
test('real input adopts its pane once, not a cancellation for every wheel tick',()=>{
 const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');
 assert(source.includes('if(scrollSource!==side||independentReveal)scrollReconciler.cancel()'));
 assert(source.includes('views[intent.side].requestMeasure(job)'),'measurement belongs to the active source rather than always the left pane');
 assert(!source.includes('createScrollReconciler(job=>root.requestAnimationFrame'),'do not add a redundant frame before the native scheduled measurement');
});

test('native virtual-gap origin contradiction uses actual visible text and preserves its fractional source anchor',()=>{
 const state=C.EditorState.create({doc:'first\nsecond\nthird'}),rect={top:-2.828125,bottom:17.3125,height:20.140625},view={state,documentTop:-25,scrollDOM:{getBoundingClientRect:()=>({top:0,bottom:100})},contentDOM:{querySelector:()=>({}),querySelectorAll:()=>[{dataset:{sourceLine:'2'},getBoundingClientRect:()=>rect}]},lineBlockAt:()=>({top:19.34375})};
 const mismatch=api.nativeOriginMismatch(view);assert.equal(mismatch.line,2);assert.equal(mismatch.delta,2.828125);assert.equal(mismatch.fraction,2.828125/20.140625);
 rect.top-=.828125;assert.equal(api.nativeOriginMismatch(view),null,'at most 2px follows existing rendered geometry tolerance');
 rect.top=-2.828125;view.contentDOM.querySelector=()=>null;assert.equal(api.nativeOriginMismatch(view),null,'never rebuild a nonvirtual native document');
});
test('native origin recovery is bounded once per side/layout generation and never consumes a busy gesture',()=>{
 const guard=api.createOriginRepairGuard();let resets=0;const reset=()=>resets++;
 for(let i=0;i<40;i++)assert.equal(guard.attempt('right',7,false,reset),false);
 assert.equal(resets,0,'continuous map/wheel/composition/transfer refusal is not a repaint timer');
 assert.equal(guard.attempt('right',7,true,reset),true);assert.equal(resets,1);
 for(let i=0;i<40;i++)assert.equal(guard.attempt('right',7,true,reset),false);
 assert.equal(guard.attempt('left',7,true,reset),true);assert.equal(guard.attempt('right',8,true,reset),true);assert.equal(resets,3);
});
test('public native reset preserves exact immutable source state, selection, external undo and focus',()=>{
 const state=C.EditorState.create({doc:'alpha\n😀 beta\nlast',selection:{anchor:3,head:10}}),undo=['previous text'],document={content:state.doc.toString(),undo,dirty:true,editRevision:12},view={state,hasFocus:true,scrollDOM:{scrollLeft:29},setState(value){assert.equal(value,state);this.state=value;this.scrollDOM.scrollLeft=0;this.hasFocus=false;},focus(){this.hasFocus=true;}};
 assert.equal(api.resetNativeOrigin(view),state);assert.equal(view.state,state);assert.equal(view.state.doc,state.doc);assert.equal(view.state.selection,state.selection);assert.equal(view.scrollDOM.scrollLeft,29);assert.equal(view.hasFocus,true);assert.equal(document.undo,undo);assert.equal(document.content,state.doc.toString());assert.equal(document.editRevision,12);assert.equal(document.dirty,true);
 view.hasFocus=false;let focusCalls=0;view.focus=()=>focusCalls++;api.resetNativeOrigin(view);assert.equal(focusCalls,0,'never steal focus from another pane/control');
 const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');assert(source.includes('mapDrag!==null||sides.some(side=>Object.values(inputBusy[side]).some(Boolean))'),'input on either pane blocks both native repairs');assert(source.includes('!!mismatch&&!interacting(side)&&!bound[side]?.frozen&&!bound[side]?.loading'));assert(source.includes('value=>pixelReady&&stable(value.epoch,value.stamp)'),'same source/revision and no composition checked again at deferred dispatch');assert(source.includes('geometry.sourceOffsets[side][mismatch.line-1]+mismatch.fraction*sourceHeights[side][mismatch.line-1]'),'restore captured logical source/fraction through the existing reconciler');assert(!source.includes("querySelectorAll('.cm-gutterElement')"),'no gutter DOM workaround');
});

test('map wheel, map keyboard and divider gestures share the same deferred-repair exclusion',()=>{
 const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');
 assert(source.includes("if(!pixelReady)return;wheelIdle[side].begin();"));
 assert(source.includes("if(!pixelReady||!geometry||!sourceHeights)return;inputBusy[side].key=true;"));
 assert(source.includes("divider.addEventListener(event,()=>{inputBusy.left[event==='keydown'?'key':'pointer']=true;"));
 assert(!source.includes("addEventListener('scrollend',()=>{inputBusy[side].wheel=false;"));
 assert(source.includes("root.removeEventListener(event,endInteraction)"));
});

test('wheel idle ends without scrollend at a boundary, extends continuous input, and cancels on blur/disposal',()=>{
 const queued=new Map(),events=[];let id=0;const clock={schedule(fn,ms){assert.equal(ms,200);queued.set(++id,fn);return id;},cancel(key){queued.delete(key);}},idle=api.createWheelIdle(value=>events.push(value),clock.schedule,clock.cancel);
 idle.begin();const stale=queued.get(1);for(let i=0;i<40;i++){idle.begin();assert.equal(queued.size,1);assert.equal(events.at(-1),true);}stale();assert.equal(events.at(-1),true,'late cancelled timer never ends a continued gesture');
 {const [key,callback]=queued.entries().next().value;queued.delete(key);callback();}assert.equal(events.at(-1),false,'boundary wheel ends even when no scroll event was produced');
 idle.begin();idle.end();assert.equal(events.at(-1),false,'window blur clears the active wheel immediately');const count=events.length;idle.begin();const late=queued.values().next().value;idle.cancel();late();assert.equal(events.length,count+1,'disposal emits no late callback');
 const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');assert(source.includes("'lostpointercapture','touchend','touchcancel','keyup','blur'"));assert(source.includes("if(event.type==='blur')mapDrag=null"));assert(source.includes('wheelIdle[side].cancel();gutterRefresh[side].cancel()'));
});

test('native rebuild retains selected hunk target instead of adopting a transient following source row',()=>{
 let actual=22,fallbacks=0,resets=0;const q=scrollQueue(intent=>({target:intent.row}),value=>{const changed=actual!==value.target;actual=value.target;return changed;});
 q.sync.request({kind:'aligned',side:'left',row:21,both:true});const stale=q.queued.shift(),measured=stale.read();
 q.sync.rebuild(()=>{resets++;actual=22;},()=>{fallbacks++;q.sync.request({kind:'aligned',row:22});});stale.write(measured);assert.equal(actual,22,'old native measurements are invalidated');q.flush();assert.equal(actual,21,'explicit Next/Previous target wins over the currently visible following line');assert.equal(fallbacks,0);assert.equal(resets,1);
 q.sync.rebuild(()=>{actual=22;},()=>fallbacks++);q.flush();assert.equal(actual,21,'retained aligned target survives even after earlier native convergence');assert.equal(fallbacks,0);
});
test('native rebuild source fallback never resurrects cancelled or replaced navigation',()=>{
 let actual=0,fallbacks=0;const q=scrollQueue(intent=>({target:intent.row}),value=>{const changed=actual!==value.target;actual=value.target;return changed;});
 q.sync.request({kind:'source',side:'right'});q.sync.rebuild(()=>{},()=>{fallbacks++;q.sync.request({kind:'aligned',row:33});});q.flush();assert.equal(actual,33);assert.equal(fallbacks,1,'no aligned intent uses captured logical source/fraction');
 q.sync.rebuild(()=>q.sync.request({kind:'aligned',row:60}),()=>fallbacks++);q.flush();assert.equal(actual,60,'new intent cannot be overwritten by earlier snapshot');
 q.sync.rebuild(()=>q.sync.cancel(),()=>fallbacks++);q.flush();assert.equal(actual,60);assert.equal(fallbacks,1,'document replacement/disposal cancellation remains final');
 q.sync.rebuild(()=>q.sync.cancel(),()=>fallbacks++);q.flush();assert.equal(fallbacks,1,'explicit cancellation with an already-null intent must not invoke fallback');
 q.sync.rebuild(()=>q.sync.pause(),()=>fallbacks++);q.flush();assert.equal(fallbacks,1,'a newer geometry invalidation also stays authoritative');
 const source=fs.readFileSync(require.resolve('../web/assets/editor-aligned-compare.js'),'utf8');assert(source.includes('scrollReconciler.rebuild(()=>resetNativeOrigin(views[side]),()=>setScroll(side,pixel))'));
});


test('alignment measurement retains fractional native width and row height at wrap thresholds',()=>{
 const view={contentDOM:{clientWidth:482,getBoundingClientRect:()=>({width:481.53125})},defaultLineHeight:20.140625};
 const metrics=api.measurementMetrics(view,20.15);assert.deepEqual(metrics,{width:481.53125,height:20.140625});
 assert.equal(metrics.height*13,261.828125,'thirteen native wrapped rows must not accumulate computed-style rounding');
 assert.equal(api.measurementMetrics({...view,defaultLineHeight:0},20.15).height,20.15);
 assert.equal(api.measurementMetrics({...view,contentDOM:{getBoundingClientRect:()=>({width:0})}}).width,1);
});


test('rendered source origins override virtual estimates without resetting source or view',()=>{
 const doc=C.EditorState.create({doc:'wrapped source'}).doc,block={from:0,to:14,top:500,height:1530.6875,bottom:2030.6875};let rendered=true,reads=0;
 const view={state:{doc},documentTop:100,elementAtHeight:()=>block,lineBlockAt:()=>block,contentDOM:{querySelector(selector){reads++;assert.equal(selector,'.cm-line[data-source-line="1"]');return rendered?{getBoundingClientRect:()=>({top:603.96875,height:1530.6875})}:null}}};
 const target=api.nativePixelFromAligned(view,[0,1530.6875],[1530.6875],765.34375);assert.equal(target,1269.3125);assert.equal(api.alignedPixelFromNative(view,[0,1530.6875],[1530.6875],target),765.34375);
 rendered=false;assert.equal(api.nativePixelFromAligned(view,[0,1530.6875],[1530.6875],765.34375),1265.34375,'unrendered target retains native estimate until remeasured');assert.equal(doc.toString(),'wrapped source');assert(reads>=3);
});
test('rendered alignment widget is matched by exact source and aligned offset, including native getter',()=>{
 const widget={side:'left',offset:7,alignedStart:20,height:30},block=Object.assign(Object.create({get widget(){return widget;}}),{from:7,to:7,top:100,height:30,bottom:130});
 const fake=(alignedStart,top)=>({dataset:{compareGap:'left',sourceOffset:'7',alignedStart:String(alignedStart)},getBoundingClientRect:()=>({top,height:30})});
 const view={documentTop:50,contentDOM:{querySelectorAll:()=>[fake(0,999),fake(20,153.5)]}},actual=api.renderedBlockGeometry(view,block);
 assert.equal(actual.top,103.5);assert.equal(actual.bottom,133.5);assert.equal(actual.widget,widget);assert.equal(widget.alignedStart,20);
});
