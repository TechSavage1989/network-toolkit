'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const app=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
const extras=fs.readFileSync(path.join(__dirname,'../web/assets/terminal-extras.js'),'utf8');
function extract(name){
 const start=app.search(new RegExp('^function '+name+'\\(','m'));
 assert(start>=0,'Missing function '+name);
 const tail=app.slice(start);
 const stop=tail.slice(1).search(/\n(?:async )?function /);
 return stop<0?tail:tail.slice(0,stop+1);
}
function dragHarness(){
 let rect={left:0,top:0,width:1000,height:800},serial=0,scheduled=0,canceled=0,saves=0,fits=0;
 const frames=new Map(),events=new Map(),windowEvents=new Map(),updates=[];
 const divider={dataset:{splitAxis:'column'},classList:{values:new Set(),add(v){this.values.add(v);},remove(v){this.values.delete(v);}},pointerId:null,
  setPointerCapture(id){this.pointerId=id;},hasPointerCapture(id){return this.pointerId===id;},releasePointerCapture(id){if(id===this.pointerId)this.pointerId=null;},
  addEventListener(type,fn){events.set(type,fn);},removeEventListener(type,fn){if(events.get(type)===fn)events.delete(type);}
 };
 const grid={getBoundingClientRect:()=>({...rect})};
 const c={terminalPaneContext:1,terminalPaneDrag:null,state:{terminalLayout:{kind:'columns',columnRatio:.5,rowRatio:.5}},
  window:null,$:sel=>sel==='#terminalPaneGrid'?grid:null,safeTerminalLayout:val=>val,
  updateTerminalPaneRatio(axis,ratio,persist){updates.push({axis,ratio,persist:!!persist});c.state.terminalLayout[axis+'Ratio']=ratio;},
  scheduleTerminalFit(){fits++;},saveState(){saves++;}
 };
 c.window={requestAnimationFrame(fn){const id=++serial;frames.set(id,fn);scheduled++;return id;},
  cancelAnimationFrame(id){frames.delete(id);canceled++;},
  addEventListener(type,fn){windowEvents.set(type,fn);},
  removeEventListener(type,fn){if(windowEvents.get(type)===fn)windowEvents.delete(type);}};
 vm.createContext(c);vm.runInContext(extract('beginTerminalPaneResize'),c);
 function send(type,x,y=300,id=7){const fn=events.get(type);assert(fn,'No '+type+' listener');fn({pointerId:id,clientX:x,clientY:y});}
 return {c,divider,grid,events,frames,updates,windowEvents,send,
  begin(){c.beginTerminalPaneResize({button:0,currentTarget:divider,pointerId:7,preventDefault(){}});},
  setRect(value){rect={...rect,...value};},
  runFrame(){const list=[...frames.values()];frames.clear();for(const fn of list)fn();},
  get scheduled(){return scheduled;},get canceled(){return canceled;},get saves(){return saves;},get fits(){return fits;}
 };
}

test('Beta 2 splitter drags are frame coalesced, honor live geometry and commit last coordinates',()=>{
 const h=dragHarness();h.begin();
 for(let x=510;x<710;x++)h.send('pointermove',x);
 assert.equal(h.frames.size,1,'pointer bursts must queue one frame');
 assert.equal(h.updates.length,0,'motion must not force synchronous CSS layout writes');
 h.runFrame();assert.equal(h.updates.length,1);
 assert.equal(h.updates[0].ratio,.709);
 h.setRect({left:100,width:800}); // Sidebar resized while dragging.
 h.send('pointermove',540);h.runFrame();
 assert.equal(h.updates.at(-1).ratio,.55,'use current splitter geometry, not the pointerdown rect');
 h.send('pointermove',600);assert.equal(h.frames.size,1);
 h.send('pointerup',740);
 assert.equal(h.frames.size,0);
 assert.equal(h.updates.at(-1).ratio,.8,'pointerup position is authoritative and clamped upstream');
 assert.equal(h.saves,1);assert.equal(h.fits,1);
 assert.equal(h.events.size,0);assert.equal(h.windowEvents.size,0);
 assert.equal(h.divider.classList.values.has('dragging'),false);
 assert.equal(h.c.terminalPaneDrag,null);
 h.runFrame();assert.equal(h.updates.at(-1).ratio,.8,'cancelled frame cannot reapply stale ratio');
});

test('Beta 2 pointercancel, capture loss and blur clean up without stale work',()=>{
 for(const stop of ['pointercancel','lostpointercapture','blur']){
  const h=dragHarness();h.begin();h.send('pointermove',700);h.runFrame();assert.equal(h.updates.at(-1).ratio,.7);
  h.send('pointermove',750);
  if(stop==='blur')h.windowEvents.get('blur')();else h.send(stop,760);
  assert.equal(h.updates.at(-1).ratio,.5,'cancel must restore the pre-drag ratio');
  assert.equal(h.saves,0,'persistence belongs to updateTerminalPaneRatio, not an extra save');
  assert.equal(h.frames.size,0);assert.equal(h.events.size,0);assert.equal(h.windowEvents.size,0);
  assert.equal(h.c.terminalPaneDrag,null);
  h.runFrame();assert.equal(h.updates.at(-1).ratio,.5);
 }
});

test('Beta 2 layout changes during a drag discard stale pointer events',()=>{
 const h=dragHarness();h.begin();h.send('pointermove',700);
 h.c.terminalPaneContext++; // View or layout moved.
 h.send('pointerup',750);
 assert.equal(h.updates.length,0);
 assert.equal(h.saves,0);
 assert.equal(h.c.terminalPaneDrag,null);
 assert.equal(h.frames.size,0);
});

test('Beta 2 pane body clips a still-unfitted xterm display to its assigned pane',()=>{
 assert.match(app,/\.terminal-pane-body\{[^}]*overflow:hidden;isolation:isolate\}/);
 assert.match(app,/\.terminal-pane-body \.terminal-view\{[^}]*overflow:hidden\}/);
 assert.match(app,/\.terminal-pane\{[^}]*overflow:hidden/);
});
function scrollHarness(){
 const calls=[];
 const root={NetworkToolkitTerminalInteraction:{preferences:()=>({autoScroll:true})},
  NetworkToolkitTerminalState:{restoreViewport(term,row){calls.push([term.name,row]);}}};
 const c={window:root,TextDecoder,TextEncoder,Date,Math,setTimeout,clearTimeout};
 vm.runInNewContext(extras,c);
 let marks=0,scrolls=0;
 function entry(id,scroll=120,base=220){
  const normal={viewportY:scroll,baseY:base,cursorY:0},alternate={viewportY:0,baseY:0,cursorY:0};
  const term={name:id,buffer:{normal,active:normal},_core:{_bufferService:{isUserScrolling:false}},
   registerMarker(){marks++;return {line:normal.viewportY,isDisposed:false,disposed:false,dispose(){this.disposed=true;}};},
   scrollToLine(row){calls.push([id,row]);},scrollToBottom(){scrolls++;}
  };
  return {info:{id},_ntSessionPrefs:{autoScroll:false},_ntScrollEpoch:0,xterm:term};
 }
 return {api:root.NetworkToolkitTerminalExtras,calls,entry,get marks(){return marks;},get scrolls(){return scrolls;}};
}
test('Beta 2 scrolled-up panes retain their own viewport when reflow disposes an anchor',()=>{
 const h=scrollHarness(),a=h.entry('A',130,240),b=h.entry('B',40,100);
 const posA=h.api.beforeFit(a),posB=h.api.beforeFit(b);
 posA.anchor.isDisposed=true;posB.anchor.line=36;
 a.xterm.buffer.normal.baseY=160;b.xterm.buffer.normal.baseY=80;
 h.api.afterFit(a,posA);h.api.afterFit(b,posB);
 assert.deepEqual(h.calls,[['A',130],['B',36]]);
 assert.equal(posA.anchor.disposed,true);assert.equal(posB.anchor.disposed,true);
 assert.equal(h.scrolls,0);
 assert.equal(a.xterm._core._bufferService.isUserScrolling,true);
 assert.equal(b.xterm._core._bufferService.isUserScrolling,true);
});
test('Beta 2 viewport fallback clamps after scrollback trimming and tolerates missing markers',()=>{
 const h=scrollHarness(),a=h.entry('A',160,200);
 const pos=h.api.beforeFit(a);pos.anchor.isDisposed=true;
 a.xterm.buffer.normal.baseY=50;
 h.api.afterFit(a,pos);
 assert.deepEqual(h.calls,[['A',50]]);
});
test('Beta 2 Follow Latest and user scroll changes override stale resize anchors',()=>{
 const h=scrollHarness(),a=h.entry('A',75,190);
 const anchor=h.api.beforeFit(a);
 a._ntScrollEpoch++;a._ntSessionPrefs.autoScroll=true;
 h.api.afterFit(a,anchor);
 assert.deepEqual(h.calls,[]);
 assert.equal(h.scrolls,0,'scroll epoch changed, resize must not scroll on its behalf');
 assert.equal(a.xterm._core._bufferService.isUserScrolling,false);
 const following=h.api.beforeFit(a);
 h.api.afterFit(a,following);
 assert.equal(h.scrolls,1);
});
test('Beta 2 alternate-screen contents do not reuse normal scrollback anchors',()=>{
 const h=scrollHarness(),a=h.entry('A',60,100);
 a.xterm.buffer.active={viewportY:0,baseY:0};
 const pos=h.api.beforeFit(a);assert.equal(pos.anchor,null);
 h.api.afterFit(a,pos);assert.deepEqual(h.calls,[]);
});
