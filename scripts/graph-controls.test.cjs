'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
const time=require('../web/assets/result-time.js');global.NetworkToolkitResultTime=time;const graph=require('../web/assets/graph-history.js');
const start='2026-10-27T00:00:00.123456789Z',end='2026-10-27T00:00:10.123456789Z',scope={start,end,condensedMs:180000};
test('visible timeframe presets include common durations and fit without erasing custom historical dates',()=>{
 graph.mergePreferences({graphTimeRange:{anchor:'range',start,end,recentMs:17000}});const html=graph.controlHTML();assert.match(html,/Timeframe/);assert.match(html,/data-graph-time-preset/);assert.match(html,/Custom/);assert.deepEqual(graph.presets.map(p=>p[0]),[10000,30000,60000,300000,900000,3600000,21600000,86400000,0]);assert(graph.setDuration(30000));assert.deepEqual(graph.preferences(),{anchor:'end',start,end,recentMs:17000,durationMs:30000,invertWheelZoom:false,compression:{count:12,strength:.55,curve:.5}});assert.equal(graph.setDuration(17),false);graph.mergePreferences({});
});
test('Y-axis fits a narrow positive range and does not require a zero baseline',()=>{const scale=graph.valueScale([42,43,48,44]);assert.equal(scale.minimum,42);assert.equal(scale.maximum,48);assert.equal(scale.lower,41.52);assert.equal(scale.upper,48.48);assert.equal(scale.empty,false);});
test('equal measurements, valid zero and very small values have stable distinct bounds and readable labels',()=>{
 for(const values of [[5,5],[0,0],[.000000001,.000000002]]){const scale=graph.valueScale(values);assert(scale.lower<=Math.min(...values)&&scale.upper>=Math.max(...values));assert(scale.upper>scale.lower);assert.notEqual(graph.scaleLabel(scale.lower),graph.scaleLabel(scale.upper));assert.equal(scale.empty,false);}
 assert.equal(graph.valueScale([0]).minimum,0);assert.equal(graph.valueScale([5]).lower,4.75);assert.equal(graph.valueScale([5]).upper,5.25);
});
test('missing and unsuccessful measurements do not fabricate scale values',()=>{const scale=graph.valueScale([null,undefined,NaN,Infinity,-1,'0']);assert.deepEqual(scale,{minimum:null,maximum:null,lower:0,upper:1,empty:true});const count=graph.valueScale([2,7],{whole:true});assert.equal(count.lower,1);assert.equal(count.upper,8);});
test('only original contributors inside the time window determine visible extrema',()=>{const items=[{time:'2026-10-26T23:59:59.123456789Z',value:9999},{time:start,value:42},{time:end,value:48},{time:'2026-10-27T00:00:10.123456790Z',value:5000}],before=JSON.stringify(items),visible=graph.select(items,scope);assert.equal(graph.valueScale(visible.map(r=>r.value)).maximum,48);assert.equal(JSON.stringify(items),before);});
test('wheel normalization supports trackpad pixels and wheel lines while respecting shortcuts and zoom limits',()=>{
 assert.equal(graph.wheelLevel({deltaY:16,deltaMode:0},1),graph.wheelLevel({deltaY:1,deltaMode:1},1));assert(graph.wheelLevel({deltaY:-1},1)<1);assert(graph.wheelLevel({deltaY:1},1)>1);for(const key of ['ctrlKey','metaKey','altKey'])assert.equal(graph.wheelLevel({deltaY:-100,[key]:true},1),1);assert.equal(graph.wheelLevel({deltaY:200},16),16);assert.equal(graph.wheelLevel({deltaY:-200},.25),.25);assert.equal(graph.wheelLevel({deltaY:0},1),1);
});
for(const mode of ['linear','condensed'])test(mode+' zoom preserves the exact pointer instant at both ends and inside the time domain',()=>{
 for(const share of [0,.1,.4,.8,1]){const anchor=graph.instant(graph.at(share,scope,mode));const finish=graph.anchoredEnd(scope,1,2,share,mode),next=graph.viewport(scope,2,finish);const result=graph.instant(graph.at(share,next,mode));assert(result>=anchor-1n&&result<=anchor+1n,mode+' '+share);assert.equal(graph.instant(next.end)-graph.instant(next.start),5000000000n);}
});
test('continuous age grouping gradually widens intervals without power-of-two tier jumps or lost records',()=>{
 const recent=60000,base=Date.parse(end),items=Array.from({length:1801},(_,i)=>({id:i,time:new Date(base-i*1000).toISOString(),value:i===900?5000:i===800?0:42})).reverse(),before=JSON.stringify(items),groups=graph.condense(items,{end,recentMs:recent}),members=groups.flatMap(g=>g.items);assert.equal(members.length,items.length);assert.equal(new Set(members).size,items.length);assert.equal(JSON.stringify(items),before);assert(groups.filter(g=>!g.aggregate).length>=60);const older=groups.filter(g=>g.aggregate).map(g=>g.items.length).reverse(),smoothed=older.slice(1,-1).map((n,i)=>(older[i]+n+older[i+2])/3);for(let i=2;i<smoothed.length-1;i++)assert(smoothed[i]<=smoothed[i-1]*1.4+1,'Resolution grows gradually after integer sample-boundary rounding: '+smoothed.slice(i-1,i+1));assert(groups.find(g=>g.items.some(p=>p.id===900)).items.some(p=>p.value===5000));assert(groups.find(g=>g.items.some(p=>p.id===800)).items.some(p=>p.value===0));
});

test('axis labels distinguish tightly clustered measurements and compact extremely small or large magnitudes',()=>{for(const values of [[42.000001,42.000002],[1e-12,2e-12],[1e10,2e10]]){const labels=graph.scaleLabels(graph.valueScale(values));assert.notEqual(labels[0],labels[1]);assert(labels.every(label=>label.length<=14));}assert.deepEqual(graph.scaleLabels(graph.valueScale([])),['—','—']);});

test('packet condensed displayed point spacing becomes progressively denser toward old history while recent raw samples have the most space',()=>{
 const base=Date.parse(end),items=Array.from({length:7201},(_,i)=>({id:i,time:new Date(base-7200000+i*1000).toISOString()})),groups=graph.condense(items,{end,recentMs:60000}),scope={start:items[0].time,end,legacyCondensed:true},positions=groups.map(g=>graph.fraction(g.start,scope,'condensed')),gaps=positions.slice(1).map((x,i)=>x-positions[i]),average=values=>values.reduce((a,b)=>a+b,0)/values.length;
 assert(groups.length<1600&&groups.length>60);assert(average(gaps.slice(-20))>average(gaps.slice(1,101))*3,'Newest samples have more display space than old aggregates');assert(average(gaps.slice(100,200))>average(gaps.slice(1,101)),'Aggregation spacing widens gradually toward recent time');assert.equal(groups.flatMap(g=>g.items).length,7201);
});


test('optional packet grouping can retain modest raw histories without changing source spacing',()=>{
 const items=Array.from({length:706},(_,i)=>({id:i,time:new Date(Date.parse(start)+i*1000).toISOString(),value:i===17?5000:i===26?0:42})),before=JSON.stringify(items),scope={start:items[0].time,end:items.at(-1).time,legacyCondensed:true},groups=graph.condense(items,{end:scope.end,recentMs:60000,preserveRaw:true});
 assert.equal(groups.length,items.length);assert(groups.every((group,index)=>!group.aggregate&&group.items.length===1&&group.items[0]===items[index]));assert.equal(groups[0].start,scope.start);assert.equal(groups.at(-1).end,scope.end);assert.equal(JSON.stringify(items),before);
 const positions=groups.map(group=>graph.fraction(group.start,scope,'condensed')),gaps=positions.slice(1).map((x,i)=>x-positions[i]);assert.equal(positions[0],0);assert.equal(positions.at(-1),1);for(let i=1;i<gaps.length;i++)assert(gaps[i]>gaps[i-1],'Each equally timed original point spreads farther apart toward newer history');
});

test('wheel inversion is strictly boolean and persists independently of range/curve settings',()=>{
 graph.mergePreferences({graphTimeRange:{durationMs:10000,invertWheelZoom:true,compression:{count:17,strength:.2,curve:.3}}});assert(graph.wheelLevel({deltaY:80},1)<1);assert(graph.wheelLevel({deltaY:-80},1)>1);const saved=graph.preferences();graph.mergePreferences({graphTimeRange:JSON.parse(JSON.stringify(saved))});assert.deepEqual(graph.preferences(),saved);assert.equal(graph.normalize({invertWheelZoom:'true'}).invertWheelZoom,false);graph.mergePreferences({});assert(graph.wheelLevel({deltaY:80},1)>1);
});
test('sample zoom floor uses five real contributors and at least two seconds, including sparse and fast records',()=>{
 const at=n=>new Date(Date.parse(start)+n*1000).toISOString();for(const values of [[0,1,2,3,4,5,6],[0,5,90,100,500,900],[0,.1,.2,.3,.4,.5,2,2.1,2.2]]){const rows=values.map(time=>({time:at(time)})),before=JSON.stringify(rows),bounded=graph.minimumSampleWindow(rows,{start:at(values.at(-1)-.01),end:at(values.at(-1))}),visible=graph.select(rows,bounded);assert(visible.length>=5);assert(graph.instant(visible.at(-1).time)-graph.instant(visible[0].time)>=2000000000n);assert.equal(JSON.stringify(rows),before);assert.deepEqual(graph.minimumSampleWindow(rows,bounded),bounded);}
 const short=[0,.1,.2].map(n=>({time:at(n)})),scope={start:at(0),end:at(.2)};assert.equal(graph.minimumSampleWindow(short,scope),scope);const old={start:at(-100),end:at(-50)};assert.equal(graph.minimumSampleWindow([0,1,2,3,4].map(n=>({time:at(n)})),old),old,'Archived-only ranges do not steal later raw data');
});
