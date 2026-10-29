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
 const good=context.summarize({label:'left-map-0-1',frames},'left');assert.equal(good.together,40);context.validateMotion(good);assert.equal(report.violations.length,0,'stale cached block identity must not create a false physical-motion failure');
 const lag=frames.map((f,i)=>({...f,panes:[f.panes[0],{top:Math.floor(i/8)*8*36}],delta:(i%8)*36}));context.validateMotion(context.summarize({label:'right-map-0-1',frames:lag},'left'));assert.equal(report.violations.length,1,'actual stalled peer remains a failure');
 report.violations=[];context.validateMotion({...good,p95PeerOffset:432});assert.equal(report.violations.length,1,'actual visible-row divergence remains a failure even when both panes move');
});
