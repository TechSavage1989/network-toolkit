'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8'),start=source.indexOf('function scrollDriveStrip('),end=source.indexOf('function paintDrives(',start);assert(start>0&&end>start);const ctx={};vm.runInNewContext(source.slice(start,end),ctx);
function wheel(strip,values={}){const e={target:{closest:selector=>selector==='[data-rb-drives]'?strip:null},deltaY:40,deltaX:0,deltaMode:0,cancelable:true,defaultPrevented:false,preventDefault(){this.defaultPrevented=true;},...values};ctx.scrollDriveStrip(e);return e;}
test('vertical mousewheel pans only overflowing drive strip with pixels, lines and pages',()=>{
 const s={clientWidth:200,scrollWidth:900,scrollLeft:0};assert(wheel(s).defaultPrevented);assert.equal(s.scrollLeft,40);wheel(s,{deltaMode:1,deltaY:3});assert.equal(s.scrollLeft,88);wheel(s,{deltaMode:2,deltaY:1});assert.equal(s.scrollLeft,288);wheel(s,{deltaY:-50});assert.equal(s.scrollLeft,238);wheel(s,{deltaY:900});assert.equal(s.scrollLeft,700);
});
test('edges, nonoverflow, unrelated targets and uncancelable/consumed events are not hijacked',()=>{
 for(const [s,e] of [[null,{}],[{clientWidth:200,scrollWidth:200,scrollLeft:0},{}],[{clientWidth:200,scrollWidth:900,scrollLeft:0},{deltaY:-1}],[{clientWidth:200,scrollWidth:900,scrollLeft:700},{}],[{clientWidth:200,scrollWidth:900,scrollLeft:10},{cancelable:false}],[{clientWidth:200,scrollWidth:900,scrollLeft:10},{defaultPrevented:true}]]){const before=s?.scrollLeft,result=wheel(s,e);assert.equal(result.defaultPrevented,e.defaultPrevented===true);assert.equal(s?.scrollLeft,before);}
});
test('horizontal touchpad, Shift-wheel and zoom modifiers retain native behavior',()=>{
 for(const e of [{deltaX:60,deltaY:20},{deltaX:30,deltaY:30},{deltaX:25,deltaY:0},{ctrlKey:true},{metaKey:true},{shiftKey:true},{deltaY:0}]){const s={clientWidth:200,scrollWidth:900,scrollLeft:20};assert(!wheel(s,e).defaultPrevented);assert.equal(s.scrollLeft,20);}
 assert(source.includes("document.addEventListener('wheel',scrollDriveStrip,{passive:false})"));
});
