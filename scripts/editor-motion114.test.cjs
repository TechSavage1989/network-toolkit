'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const source=fs.readFileSync(require.resolve('./editor-motion114.browser.cjs'),'utf8');
test('temporal motion sampler is a non-mutating DOM read, never a native layout flush',()=>{
 const sample=source.slice(source.indexOf('function beginCapture('),source.indexOf('async function finishCapture('));
 const code=sample.replace(/\/\/[^\n]*/g,'');
 assert(!/elementAtHeight|lineBlockAt|requestMeasure|EditorView|\.dispatch\(/.test(code));
 assert(code.includes('getBoundingClientRect()')&&code.includes('s.scrollTop'));
});
test('motion acceptance measures actual per-frame pixels and catches sustained physical peer lag',()=>{
 const report={violations:[]},context={assert,report,console:{error(){}}};vm.createContext(context);
 vm.runInContext(source.slice(source.indexOf('function summarize('),source.indexOf('async function captureGesture('))+source.slice(source.indexOf('function validateMotion('),source.indexOf('async function converged(')),context);
 const frames=Array.from({length:41},(_,i)=>({at:i*16.7,panes:[{top:i*36,position:i},{top:i*36,position:i-1}],delta:.5}));
 const inputs=frames.slice(1).map((f,i)=>({at:f.at-1,frame:i+1}));const good=context.summarize({label:'left-map-0-1',frames,inputs},'left');assert.equal(good.together,40);context.validateMotion(good);assert.equal(report.violations.length,0,'stale cached block identity must not create a false physical-motion failure');
 const lag=frames.map((f,i)=>({...f,panes:[f.panes[0],{top:Math.floor(i/8)*8*36}],delta:(i%8)*36}));context.validateMotion(context.summarize({label:'right-map-0-1',frames:lag,inputs},'left'));assert.equal(report.violations.length,1,'actual stalled peer remains a failure');
 report.violations=[];const stopped=frames.map((f,i)=>({...f,panes:f.panes.map(p=>({...p,top:i<30?0:1440}))}));context.validateMotion(context.summarize({label:'left-map-0-1',frames:stopped,inputs},'left'));assert.equal(report.violations.length,1,'both panes frozen then jumping together must fail the temporal bound');
 report.violations=[];const coalesced=Array.from({length:91},(_,i)=>({at:i*16.7,panes:[{top:Math.floor(i/4)*36},{top:Math.floor(i/4)*36}],delta:.5}));const continuous=context.summarize({label:'left-map-0-1',frames:coalesced,inputs:coalesced.slice(1,89).map((f,i)=>({at:f.at-1,frame:i+1}))},'left');assert(continuous.moving<24);context.validateMotion(continuous);assert.equal(report.violations.length,0,'continuous bounded coalesced motion must not require an arbitrary number of sampled positions');
 report.violations=[];context.validateMotion({...good,unansweredInputs:1});assert.equal(report.violations.length,1,'the final delivered input cannot be omitted from response acceptance');
 report.violations=[];context.validateMotion({...good,maxSourcePauseMs:1000,maxInputResponseFrames:1,maxInputResponseMs:10});assert.equal(report.violations.length,0,'a transport pause before input delivery is not an application freeze');
 report.violations=[];context.validateMotion({...good,maxInputResponseMs:589});assert.equal(report.violations.length,1,'a589ms frozen source fails even if few RAF samples occurred');
 report.violations=[];context.validateMotion({...good,maxInputResponseFrames:5});assert.equal(report.violations.length,1,'more than4 opportunities without source response must fail despite quick wall time');
 report.violations=[];context.validateMotion({...good,p95PeerOffset:432});assert.equal(report.violations.length,1,'actual visible-row divergence remains a failure even when both panes move');
});

test('the first delivered input is compared to a synchronous pre-input baseline, not the next gesture step',()=>{
 let now=0;const scheduled=[],listeners={},scroll={scrollTop:0,getBoundingClientRect:()=>({top:0,bottom:200})};
 const line={dataset:{sourceLine:'1'},getBoundingClientRect:()=>({top:60-scroll.scrollTop,bottom:80-scroll.scrollTop,height:20})};
 const track={getAttribute:()=> '0',addEventListener:(type,fn)=>listeners[type]=fn,removeEventListener(){}};
 const c={result:{rows:[{left:0,right:0}]},sourceView:()=>({inputElement:{querySelectorAll:()=>[line]},scrollElement:scroll})};
 const context={window:{},NetworkToolkitFileEditor:{comparison:c},document:{querySelector:()=>track},performance:{now:()=>now},requestAnimationFrame:fn=>scheduled.push(fn)};vm.createContext(context);
 vm.runInContext(source.slice(source.indexOf('function beginCapture('),source.indexOf('async function finishCapture('))+source.slice(source.indexOf('function summarize('),source.indexOf('async function captureGesture(')),context);
 context.beginCapture({label:'left-map-0-1',side:'left',kind:'map'});const record=context.window.motion114;
 assert.equal(record.frames.length,1,'baseline must exist before page.evaluate returns and the first real input is sent');
 now=1;listeners.pointermove({buttons:1});scroll.scrollTop=36;now=16.7;scheduled.shift()();
 const summary=context.summarize(record,'left');assert.equal(summary.deliveredInputs,1);assert.equal(summary.unansweredInputs,0);assert.equal(summary.maxInputResponseFrames,1);assert.equal(summary.maxInputResponseMs,15.7);
});
