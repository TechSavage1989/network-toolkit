'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const source=fs.readFileSync(path.join(__dirname,'credential-security.browser.cjs'),'utf8');
const begin=source.indexOf('async function stableCredentialLayout(page){'),end=source.indexOf('\n(async()=>{',begin);
assert(begin>=0&&end>begin,'browser layout helper must remain testable');

async function measure(widths){
 let frame=0;
 const width=()=>widths[Math.min(frame-1,widths.length-1)];
 const body={clientWidth:364,get scrollWidth(){return width();},getBoundingClientRect:()=>({left:13,right:377}),querySelectorAll:()=>[{tagName:'DIV',id:'fixture-overflow',className:'fixture',clientWidth:width(),scrollWidth:width(),getBoundingClientRect:()=>({left:13,right:13+width(),width:width()})}]};
 const modal={querySelector:selector=>{assert.equal(selector,'.modal-body');return body;},getBoundingClientRect:()=>({left:12,right:378})};
 const context=vm.createContext({innerWidth:390,document:{fonts:{ready:Promise.resolve()}},requestAnimationFrame:callback=>{frame++;queueMicrotask(callback);}});
 vm.runInContext(source.slice(begin,end),context);
 const page={locator:selector=>{assert.equal(selector,'.modal');return {evaluate:callback=>callback(modal)};}};
 return JSON.parse(JSON.stringify(await context.stableCredentialLayout(page)));
}

test('credential layout measurement waits for consecutive stable frames after resize',async()=>{
 const result=await measure([735,364,364,364]);
 assert.equal(result.settled,true);assert.equal(result.frames,4);assert.equal(result.overflow,0);
 assert.equal(result.history[0].overflow,371);
});

test('stable credential overflow is returned as a failure candidate, not waited away',async()=>{
 const result=await measure([735,735,735,364,364,364]);
 assert.equal(result.settled,true);assert.equal(result.frames,3);assert.equal(result.overflow,371);
 assert.equal(result.overflowing[0].id,'fixture-overflow');
});

test('credential layout measurement fails bounded settling when geometry keeps changing',async()=>{
 const result=await measure(Array.from({length:90},(_,i)=>364+i));
 assert.equal(result.settled,false);assert.equal(result.frames,90);assert.equal(result.overflow,89);
});
