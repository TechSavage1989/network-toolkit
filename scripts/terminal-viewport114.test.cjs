'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
global.self=global;
const {Terminal}=require('../web/assets/xterm.js');require('../web/assets/terminal-state.js');
const state=global.NetworkToolkitTerminalState;
// Extract the exact bundled viewport class, not an independently rewritten
// scrolling algorithm. Only the browser's pixel storage/refresh queue is modeled.
const source=fs.readFileSync(require.resolve('../web/assets/xterm.js'),'utf8'),needle='t.Viewport=u,t.Viewport=u=s(';
assert.equal(source.split(needle).length,2);
const context={self:{},module:{exports:{}},exports:{},console,process,performance,setTimeout,clearTimeout,queueMicrotask};context.self=context;
vm.runInNewContext(source.replace(needle,'globalThis.TestViewport=u,'+needle),context);
const Viewport=context.TestViewport;assert.equal(typeof Viewport,'function');
async function fixture(){
 const term=new Terminal({cols:89,rows:12,scrollback:1000,allowProposedApi:true});await new Promise(r=>term.write(Array.from({length:80},(_,i)=>'SYNTHETIC '+String(i).padStart(3,'0')).join('\r\n'),r));
 const viewport=Object.create(Viewport.prototype),queue=[];let scrollTop=0,dimensions={height:204,scrollHeight:1360};
 Object.assign(viewport,{_bufferService:term._core._bufferService,_isSyncing:false,_isHandlingScroll:false,_suppressOnScrollHandler:false,_renderService:{dimensions:{css:{cell:{height:17},canvas:{height:204}}},addRefreshCallback(fn){queue.push(fn);return queue.length;}},_onRequestScrollLines:{fire(n){term._core._bufferService.scrollLines(n,false);}},_scrollableElement:{getScrollPosition:()=>({scrollTop}),setScrollDimensions(d){dimensions=d;},setScrollPosition(p){const old=scrollTop;scrollTop=Math.max(0,Math.min(p.scrollTop,dimensions.scrollHeight-dimensions.height));if(scrollTop!==old)viewport._handleScroll({scrollTop});}}});
 term._core._viewport=viewport;
 return {term,viewport,queue,get row(){return scrollTop/17;},set row(n){scrollTop=n*17;},flush(){while(queue.length)queue.shift()();},close(){term._core._viewport=undefined;term.dispose();}};
}
test('actual bundled viewport reproduces relative buffer/scrollbar divergence and absolute restore closes it',async()=>{const f=await fixture();try{f.term._core._bufferService.buffer.ydisp=10;f.row=15;f.term.scrollToLine(10);assert.equal(f.row,15,'old public delta is zero despite wrong displayed row');f.viewport._handleScroll({scrollTop:15*17});assert.equal(f.term.buffer.active.viewportY,15,'stale browser scroll can overwrite model');state.restoreViewport(f.term,10);f.flush();assert.equal(f.row,10);assert.equal(f.term.buffer.active.viewportY,10);assert.equal(f.viewport._latestYDisp,10);}finally{f.close();}});
test('absolute viewport restore survives the actual12/17-row native geometry changes and queued refreshes',async()=>{const f=await fixture();try{state.restoreViewport(f.term,10);for(const rows of [17,12,17,12]){f.term.resize(rows===17?94:89,rows);f.viewport._renderService.dimensions.css.canvas.height=rows*17;state.restoreViewport(f.term,10);f.flush();assert.equal(f.row,10);assert.equal(f.term.buffer.active.viewportY,10);assert.equal(f.term.buffer.normal.getLine(10).translateToString(true),'SYNTHETIC 010');}}finally{f.close();}});
test('viewport restoration rejects invalid targets or unrecognized browser layout without changing the current row',async()=>{const f=await fixture();try{state.restoreViewport(f.term,10);for(const row of [-1,NaN,1.5,10000])assert.throws(()=>state.restoreViewport(f.term,row),/viewport restore row/);f.viewport.queueSync=undefined;assert.throws(()=>state.restoreViewport(f.term,0),/pinned viewport API/);assert.equal(f.row,10);}finally{f.close();}});
test('headless fallback preserves validated public scroll behavior',async()=>{const t=new Terminal({cols:20,rows:4,allowProposedApi:true});try{await new Promise(r=>t.write('one\r\ntwo\r\nthree\r\nfour\r\nfive\r\nsix',r));state.restoreViewport(t,1);assert.equal(t.buffer.active.viewportY,1);}finally{t.dispose();}});
