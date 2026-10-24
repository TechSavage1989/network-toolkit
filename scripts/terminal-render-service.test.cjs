'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
global.self=global;const {Terminal}=require('../web/assets/xterm.js');
const renderRowFor=require('./terminal-render-fixture.cjs'),{RenderService}=renderRowFor;
const noEvent=()=>({dispose(){}});

test('real render service keeps overlapping scrollback destinations colored through full refreshes and settled timers',async t=>{
 const term=new Terminal({cols:150,rows:52,scrollback:2000,allowProposedApi:true});
 Object.defineProperty(term,'element',{value:{getClientRects:()=>[{}]}});
 const timers=new Map(),frames=new Map(),painted=new Map(),renders=[];let next=0,clock=0;
 const renderRow=renderRowFor(term);
 // Keep the bundled RenderService and RenderDebouncer, including automatic
 // full refreshes on decoration insertion/removal. Only RAF delivery and DOM
 // element allocation/measurement are substituted for the Node environment.
 const service=new RenderService(term.rows,null,term._core.optionsService,
  {onCharSizeChange:noEvent,hasValidSize:true,measure(){}},term._core.coreService,
  term._core._decorationService,term._core._bufferService,
  {window:{requestAnimationFrame(fn){const id=++next;frames.set(id,fn);return id;},cancelAnimationFrame(id){frames.delete(id);}},onDprChange:noEvent,onWindowChange:noEvent},
  {onChangeColors:noEvent});
 service.onRenderedViewportChange(event=>term._core._onRender.fire(event));
 service.setRenderer({dimensions:{css:{canvas:{width:100,height:100}}},
  renderRows(start,end){renders.push({start,end});for(let row=start;row<=end;row++)painted.set(row,renderRow(term.buffer.active.viewportY+row));},
  onRequestRedraw:noEvent,handleSelectionChanged(){},clear(){},dispose(){}});
 term.refresh=(start,end)=>service.refreshRows(start,end);
 term.onScroll(()=>term.refresh(0,term.rows-1));term.onWriteParsed(()=>term.refresh(0,term.rows-1));
 const context={setTimeout(fn){const id=++next;timers.set(id,fn);return id;},clearTimeout(id){timers.delete(id);},performance:{now:()=>clock+=7}};
 vm.runInNewContext(fs.readFileSync(require.resolve('../web/assets/terminal-highlight.js'),'utf8'),context);
 const controller=context.NTTerminalHighlight.attach(term,{underlines:false});
 t.after(()=>{controller.dispose();service.dispose();term.dispose();});
 const frame=()=>{const jobs=[...frames.values()];frames.clear();for(const fn of jobs)fn();};
 const timer=()=>{const jobs=[...timers.values()];timers.clear();for(const fn of jobs)fn();};
 function settle(){
  // Browser paints can precede the 40 ms highlight timer. Running the frame
  // first also exercises RenderService's own decoration-driven full redraws.
  for(let iteration=0;(frames.size||timers.size)&&iteration<1024;iteration++){frame();timer();}
  assert.equal(frames.size,0,'animation frames did not settle');assert.equal(timers.size,0,'highlight timers did not settle');
 }
 function missingColors(){
  return Array.from({length:term.rows},(_,row)=>row).filter(row=>{
   const spans=painted.get(row)||[];
   return !spans.some(span=>span.color==='#f0a3d0')||!spans.some(span=>span.color==='#bddf86');
  });
 }
 await new Promise(resolve=>term.write(Array.from({length:1800},(_,i)=>'        set snmp-index '+i).join('\r\n'),resolve));
 await new Promise(resolve=>setTimeout(resolve,20));
 frame();assert.deepEqual(renders.at(-1),{start:0,end:51},'decoration changes must use the real full-viewport refresh');
 settle();assert.deepEqual(missingColors(),[],'initial visible output did not finish coloring');
 const failures=[];
 for(const destination of [1500,1478,1490,1450,1460,1400,1380,1420,1400,1360]){
  term.scrollToLine(destination);frame();
  const first=missingColors();if(first.length)failures.push({destination,stage:'first paint',rows:first});
  settle();const settled=missingColors();if(settled.length)failures.push({destination,stage:'settled',rows:settled});
 }
 assert.deepEqual(failures,[],'live decorations must remain discoverable after overlapping scroll moves');
 const index=term._core._decorationService._decorations;
 assert.equal([...index.values()].filter(decoration=>decoration.marker.isDisposed).length,0,'disposed markers remained in the native decoration index');
});
