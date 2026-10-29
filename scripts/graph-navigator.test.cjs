'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
const h=require('../web/assets/graph-history.js'),nav=require('../web/assets/graph-navigator.js');
const stamp=n=>h.timestamp(h.instant('2026-10-27T00:00:00.123456789Z')+BigInt(n));
const full={start:stamp(0),end:stamp(86400000000000)};
test('a six-hour window occupies exactly one quarter of a day with timestamp-based position',()=>{const c=nav.coverage(full,{start:stamp(21600000000000),end:stamp(43200000000000)});assert.equal(c.left,.25);assert.equal(c.width,.25);assert.equal(c.before,false);assert.equal(c.after,false);});
test('dragging preserves exact duration and clamps movement at the retained extent',()=>{const view={start:stamp(21600000000000),end:stamp(43200000000000)},next=nav.move(full,view,'window',.5);assert.equal(h.instant(next.end)-h.instant(next.start),21600000000000n);assert.equal(next.end,full.end);assert.equal(nav.move(full,view,'window',-5).start,full.start);assert.equal(JSON.stringify(view),JSON.stringify({start:stamp(21600000000000),end:stamp(43200000000000)}));});
test('independent handle resizing retains the opposite boundary and one-nanosecond intervals',()=>{const view={start:stamp(21600000000000),end:stamp(43200000000000)},a=nav.move(full,view,'start',.1),b=nav.move(full,view,'end',-.1);assert.equal(a.end,view.end);assert.equal(b.start,view.start);assert.equal(h.instant(a.start),h.instant(view.start)+8640000000000n);const tiny=nav.move(full,view,'start',1);assert.equal(h.instant(tiny.end)-h.instant(tiny.start),1n);assert(nav.coverage(full,tiny).width<1e-12);});
test('outside-history coverage is clipped visually without altering requested timestamps',()=>{const view={start:stamp(-21600000000000),end:stamp(21600000000000)},before=JSON.stringify(view),c=nav.coverage(full,view);assert.equal(c.left,0);assert.equal(c.width,.25);assert.equal(c.before,true);assert.equal(JSON.stringify(view),before);const wide=nav.move(full,{start:stamp(-86400000000000),end:stamp(86400000000000)},'window',-.1);assert.equal(h.instant(wide.end)-h.instant(wide.start),172800000000000n);});
test('retained extent ignores unknown timestamps and preserves nanosecond bounds, duplicate instants and empty history',()=>{assert.equal(nav.extent([{time:stamp(1)},{time:stamp(0)},{time:'0001-01-01T00:00:00Z'},{}]).end,stamp(1));assert.equal(nav.coverage(nav.extent([]),full).valid,false);const one=nav.extent([{time:stamp(0)},{time:stamp(0)}]);assert(nav.coverage(one,one).single);assert.equal(nav.coverage(one,one).width,1);});
test('Fit selects all retained time and clears chosen dates while preserving compression preference',()=>{h.mergePreferences({graphTimeRange:{durationMs:10000,anchor:'range',start:stamp(0),end:stamp(50),recentMs:17000}});nav.fit();assert.deepEqual(h.preferences(),{durationMs:0,anchor:'latest',start:'',end:'',recentMs:17000,allHistory:true,invertWheelZoom:false,compression:{count:12,strength:.55,curve:.5}});h.mergePreferences({});});
test('bounded overview retains genuine extrema, valid zeros and missing/status information without mutating records',()=>{const rows=Array.from({length:10000},(_,i)=>({time:stamp(i*1000000),value:i===440?9000:i===441?0:i%19===0?null:42,level:i%19===0?'bad':'good'})),before=JSON.stringify(rows),preview=nav.geometry(nav.extent(rows),rows);assert.equal(preview.scale.minimum,0);assert.equal(preview.scale.maximum,9000);assert(preview.html.includes('gn-unmeasured'));assert(preview.html.match(/gn-bad/));assert(preview.html.length<230000);assert.equal(JSON.stringify(rows),before);assert(nav.geometry(nav.extent([]),[]).scale.empty);});

test('sub-nanosecond magnification limits and wide windows retain their actual temporal span',()=>{const tiny=h.viewport(full,86400000000000);assert.equal(h.instant(tiny.end)-h.instant(tiny.start),1n);const base={start:stamp(0),end:stamp(10000000000)},wide=h.viewport(base,.01);assert.equal(h.instant(wide.end)-h.instant(wide.start),1000000000000n);});

test('timeline wheel pans actual time from either axis with pixel/line/page normalization and clamped boundaries',()=>{
 const view={start:stamp(21600000000000),end:stamp(43200000000000)};
 const down=nav.wheel(full,view,{deltaY:120}),up=nav.wheel(full,down,{deltaY:-120});
 assert(h.instant(down.start)>h.instant(view.start));assert.equal(up.start,view.start);assert.equal(up.end,view.end);
 assert.deepEqual(nav.wheel(full,view,{deltaX:120,deltaY:2}),down);
 assert.deepEqual(nav.wheel(full,view,{deltaY:1,deltaMode:1}),nav.wheel(full,view,{deltaY:16}));
 assert.deepEqual(nav.wheel(full,view,{deltaY:1,deltaMode:2}),nav.wheel(full,view,{deltaY:600}));
 assert.equal(nav.wheel(full,{start:full.start,end:stamp(1)},{deltaY:-600}).start,full.start);
 assert.equal(nav.wheel(full,{start:stamp(43200000000000),end:full.end},{deltaY:600}).end,full.end);
 for(const key of ['ctrlKey','metaKey','altKey'])assert.equal(nav.wheel(full,view,{deltaY:120,[key]:true}),null);
 for(const deltaY of [0,NaN,Infinity])assert.equal(nav.wheel(full,view,{deltaY}),null);
});

test('visible elapsed labels use seconds, minutes and hours, including exact subsecond and empty cases',()=>{
 const start=stamp(0),label=ns=>h.spanLabel({start,end:stamp(ns)});
 assert.equal(label(10000000000),'10 s');assert.equal(label(90000000000),'1.5 min');assert.equal(label(5400000000000),'1.5 h');assert.equal(label(86400000000000),'24 h');assert.equal(label(1000000),'1 ms');assert.equal(label(1),'1 ns');assert.equal(label(0),'0 s');assert.equal(h.spanLabel({start:'',end:start}),'—');
});

test('wheel integration coalesces input in one frame without losing accumulated distance or gesture boundaries',()=>{
 const fs=require('node:fs'),vm=require('node:vm'),{document:makeDocument}=require('./terminal-dom-fixture.cjs'),document=makeDocument('');let queued,begin=0,end=0,changes=[];
 const context={document,NetworkToolkitGraphHistory:h,requestAnimationFrame:fn=>{queued=fn;return 1;},cancelAnimationFrame:()=>{},addEventListener:()=>{}};context.window=context;vm.createContext(context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/graph-navigator.js'),'utf8'),context);const nav=context.NetworkToolkitGraphNavigator,view={start:stamp(21600000000000),end:stamp(43200000000000)};
 document.innerHTML=nav.html('wheel',full,view,{html:''});const node=document.querySelector('[data-graph-navigator]'),config={full,view,begin:()=>begin++,end:()=>end++,change:scope=>changes.push(scope)};nav.attach(node,config);
 for(let i=0;i<4;i++)document.emit('wheel',node,{deltaY:30,deltaMode:0});assert.equal(changes.length,0);assert.equal(begin,1);queued();assert.equal(end,1);assert.equal(changes.length,1);assert.equal(changes[0].start,nav.wheel(full,view,{deltaY:120}).start);assert.equal(changes[0].end,nav.wheel(full,view,{deltaY:120}).end);
 nav.attach(node,{...config,view:changes[0]});document.emit('wheel',node,{deltaY:-120});queued();assert.equal(changes[1].start,view.start);assert.equal(begin,2);assert.equal(end,2);
});
