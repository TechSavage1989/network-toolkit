'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const timeline=require('../web/assets/result-timeline.js');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const stamp=n=>'2026-10-26T16:00:'+String(n).padStart(2,'0')+'.123Z';
const reply=(sequence,rttMs,time=stamp(sequence))=>({sequence,rttMs,time,success:true,status:'Reply',address:'192.0.2.20'});
const samples=()=>[reply(1,10),reply(2,14),{...reply(3,999),success:false,status:'Timed out'},reply(4,30),reply(5,20)];

function harness(){
 const document=makeDocument('');document.body=document;document.activeElement=document;
 let observer;
 const context={document,console,Intl,Date,NetworkToolkitResultTime:{format:value=>value==null||value===''?'Not recorded':'LOCAL '+value},MutationObserver:class{constructor(fn){observer=fn;}observe(){}}};
 context.window=context;vm.createContext(context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/result-timeline.js'),'utf8'),context);
 const api=context.NetworkToolkitResultTimeline;
 function render(rows=samples(),id='job'){
  for(const child of [...document.children])child.remove();
  document.innerHTML=api.latency(rows,{id});
  for(const surface of document.querySelectorAll('[data-rt-surface]'))surface.focus=()=>{document.activeElement=surface;document.emit('focusin',surface);};
  return document.querySelector('.rt-timeline');
 }
 return {context,document,api,render,observe:()=>observer?.(),detail:()=>document.querySelector('.rt-detail').textContent,mark:index=>document.querySelector('[data-rt-surface="latency"] [data-rt-point="'+index+'"]')};
}

test('window statistics retain only measured RTTs and never bridge losses for jitter',()=>{
 const points=timeline.normalize(samples()),stats=timeline.summary(points);
 assert.deepEqual(points.map(point=>point.rtt),[10,14,null,30,20]);
 assert.deepEqual(points.map(point=>point.jitter),[null,4,null,null,10]);
 assert.equal(stats.count,5);assert.equal(stats.received,4);assert.equal(stats.lost,1);assert.equal(stats.lossPercent,20);
 assert.equal(stats.min,10);assert.equal(stats.max,30);assert.equal(stats.average,18.5);assert.equal(stats.jitter,7);assert.equal(stats.jitterPairs,2);
 const empty=timeline.summary(timeline.normalize([{success:true,rttMs:null},{success:true,rttMs:''},{success:false,rttMs:0},{success:true,rttMs:-5},{success:true,rttMs:Infinity}]));
 assert.equal(empty.average,null);assert.equal(empty.jitter,null);
 assert.equal(timeline.normalize([reply(1,0),reply(2,0)])[1].jitter,0,'A measured zero remains a legitimate measurement');
});

test('latest 180 bounds preserve source identities and explicitly label window-only statistics',()=>{
 const rows=Array.from({length:220},(_,i)=>reply(i+1,i+1,'2026-10-26T16:00:00Z'));
 const points=timeline.normalize(rows);
 assert.equal(points.length,180);assert.equal(points[0].sequence,41);assert.equal(points.at(-1).sequence,220);
 assert.equal(points[0].jitter,null,'No jitter pair is implied outside the displayed window');
 assert.equal(timeline.summary(points).min,41);
 const html=timeline.latency(rows,{id:'retained'});
 assert.match(html,/Latest 180 retained samples/);assert.match(html,/Statistics for displayed samples/);
 assert.equal(rows[0].rttMs,1,'Rendering never changes source rows');
});

test('missing time is unknown, epoch zero survives, and untrusted point fields are escaped',()=>{
 const points=timeline.normalize([{sequence:1,time:'',timestamp:stamp(1),rttMs:1,success:true},{sequence:2,time:0,rttMs:2,success:true},{sequence:3,rttMs:3,success:true,address:'<img src=x onerror=alert(1)>',status:'<script>bad</script>'}]);
 assert.equal(points[0].time,stamp(1));assert.equal(points[1].time,0);assert.equal(points[2].time,'');
 const html=timeline.pointDetails(points[2]);
 assert.match(html,/Not recorded/);assert.match(html,/&lt;img/);assert.match(html,/&lt;script&gt;/);assert.doesNotMatch(html,/<img|<script/);
 assert.match(timeline.latency([reply(1,1)],{id:'" onclick="bad',title:'<bad>'}),/data-rt-id="&quot; onclick=&quot;bad"/);
});

test('hover details use recorded local time, click pins, pointer exit restores the pinned sample',()=>{
 const h=harness();h.render();
 h.document.emit('pointermove',h.mark(1));assert.match(h.detail(),/LOCAL 2026-10-26T16:00:02.123Z/);assert.match(h.detail(),/14 ms/);
 h.render([...samples(),reply(6,19)]);assert.match(h.detail(),/14 ms/,'Stationary hover details do not disappear on a live render');
 h.document.emit('click',h.mark(1));assert.equal(h.document.querySelector('[data-rt-clear]').disabled,false);
 h.document.emit('pointermove',h.mark(2));assert.match(h.detail(),/14 ms/,'Pinned details remain selected while another sample is hovered');assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Timed out/);assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Not measured/);assert.doesNotMatch(h.document.querySelector('[data-rt-tooltip]').textContent,/999 ms/);assert(h.mark(1).classList.contains('rt-point-selected'));assert(h.mark(2).classList.contains('rt-point-hovered'));
 h.document.emit('pointerout',h.mark(2),{relatedTarget:null});assert.match(h.detail(),/14 ms/);
 h.render([...samples(),reply(6,19)]);assert.match(h.detail(),/LOCAL 2026-10-26T16:00:02.123Z/,'Pinned details survive new live samples');
});

test('arrow keys, Home, End, Enter and Escape expose exact samples without fake loss timing',()=>{
 const h=harness();h.render();let surface=h.document.querySelector('[data-rt-surface="latency"]');
 h.document.emit('keydown',surface,{key:'Home'});assert.match(h.detail(),/Sample1/);
 h.document.emit('keydown',surface,{key:'ArrowRight'});assert.match(h.detail(),/Sample2/);
 h.document.emit('keydown',surface,{key:'ArrowRight'});assert.match(h.detail(),/Sample3/);assert.match(h.detail(),/Not measured/);
 h.document.emit('keydown',surface,{key:'ArrowRight'});assert.match(h.detail(),/Sample4/);assert.match(h.detail(),/No consecutive measured pair/);
 h.document.emit('keydown',surface,{key:'End'});assert.match(h.detail(),/Sample5/);assert.match(h.detail(),/10 ms/);
 h.document.emit('keydown',surface,{key:'Enter'});assert.match(h.detail(),/Sample5/);
 h.document.emit('keydown',surface,{key:'Escape'});assert.match(h.detail(),/Hover over a point/);assert.equal(h.document.querySelector('[data-rt-clear]').disabled,true);
});

test('pinned selections are job-scoped and retain recorded details outside the moving window',()=>{
 const h=harness();h.render();h.document.emit('click',h.mark(0));
 h.render(samples(),'other-job');assert.match(h.detail(),/Hover over a point/);
 h.render(samples(),'job');assert.match(h.detail(),/Sample1/);
 h.render(Array.from({length:181},(_,i)=>reply(i+1,10,'2026-10-26T16:00:00Z')),'job');
 assert.match(h.detail(),/Sample1/);assert.match(h.document.querySelector('.rt-selection-context').textContent,/outside the moving window|no longer in retained history/);
 assert.equal(h.document.querySelector('[data-rt-clear]').hasAttribute('disabled'),false);
});

test('variability expansion and chart keyboard focus survive live rendering',()=>{
 const h=harness();h.render();const details=h.document.querySelector('.rt-variability');details.open=true;h.document.emit('toggle',details);
 const original=h.document.querySelector('[data-rt-surface="jitter"]');original.focus();
 h.render([...samples(),reply(6,18)]);
 assert.equal(h.document.querySelector('.rt-variability').hasAttribute('open'),true);
 h.document.activeElement=h.document;
 h.observe();assert.equal(h.document.activeElement,h.document.querySelector('[data-rt-surface="jitter"]'));
});

test('plot coordinate hover chooses the nearest actual point when no marker is hit',()=>{
 const h=harness();h.render();const surface=h.document.querySelector('[data-rt-surface="latency"]');
 surface.getBoundingClientRect=()=>({left:10,width:820});
 h.document.emit('pointermove',surface,{clientX:10});assert.match(h.detail(),/Sample1/);
 h.document.emit('pointermove',surface,{clientX:830});assert.match(h.detail(),/Sample5/);
 h.document.emit('click',h.document.querySelector('[data-rt-clear]'));assert.match(h.detail(),/Hover over a point/);
});

test('persistent selection and transient hover have distinct marks, details and popup',()=>{
 const h=harness();h.render();h.document.emit('click',h.mark(0));h.document.emit('pointermove',h.mark(3),{clientX:970,clientY:730});
 assert(h.mark(0).classList.contains('rt-point-selected'));assert(!h.mark(3).classList.contains('rt-point-selected'));assert(h.mark(3).classList.contains('rt-point-hovered'));
 assert.match(h.detail(),/Sample1/);const tooltip=h.document.querySelector('[data-rt-tooltip]');assert.match(tooltip.textContent,/Sample 4/);assert.match(tooltip.textContent,/30 ms/);assert.equal(tooltip.hidden,false);
 h.document.emit('pointerout',h.mark(3),{relatedTarget:null});assert.equal(tooltip.hidden,true);assert(h.mark(0).classList.contains('rt-point-selected'));assert(!h.mark(3).classList.contains('rt-point-hovered'));
 h.render([...samples(),reply(6,12)]);assert.match(h.detail(),/Sample1/);assert(h.mark(0).classList.contains('rt-point-selected'));
});

test('progressive mode virtualizes retained history and preserves paused horizontal position across new samples',()=>{
 const h=harness(),rows=Array.from({length:10000},(_,i)=>reply(i+1,10+i%13,'2026-10-26T16:00:00Z'));
 h.api.setPreferences({mode:'progressive'},{persist:false});h.render(rows);
 let scroll=h.document.querySelector('[data-rt-scroll="latency"]');scroll.clientWidth=600;scroll.scrollWidth=80010;scroll.scrollLeft=2400;h.document.emit('scroll',scroll);
 assert.equal(scroll.scrollLeft,2400);assert(h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]').length<100,'Only visible progressive markers enter the DOM');
 const first=Number(h.document.querySelector('[data-rt-surface="latency"] [data-rt-sequence]').dataset.rtSequence);assert(first>200&&first<400);
 h.render([...rows.slice(1),reply(10001,14,'2026-10-26T16:00:00Z')]);h.observe();scroll=h.document.querySelector('[data-rt-scroll="latency"]');assert.equal(scroll.scrollLeft,2400,'A live render does not pull an inspecting user to the latest point');
 assert.equal(h.document.querySelector('[data-rt-follow]').getAttribute('aria-pressed'),'false');
 h.document.emit('click',h.document.querySelector('[data-rt-follow]'));assert.equal(h.document.querySelector('[data-rt-follow]').getAttribute('aria-pressed'),'true');assert(scroll.scrollLeft>78000);
 assert.equal(h.api.normalize(rows,{mode:'progressive'}).length,10000);assert.equal(h.api.normalize([...rows,...rows],{mode:'progressive'}).length,10000,'Retained history is bounded');
 h.api.setPreferences({mode:'window'},{persist:false});
});

test('saved graph mode is normalized, persisted through host state and merged without losing a pin',()=>{
 const h=harness();let saves=0;h.context.state={};h.context.saveState=()=>saves++;h.api.setPreferences({mode:'progressive'});assert.equal(saves,1);assert.equal(h.context.state.graphPreferences.mode,'progressive');
 h.render();h.document.emit('click',h.mark(1));h.api.mergePreferences({graphPreferences:{mode:'window'}});h.render();assert.equal(h.api.preferences().mode,'window');assert.match(h.detail(),/Sample2/);
 h.api.mergePreferences({graphPreferences:{mode:'invalid'}});assert.equal(h.api.preferences().mode,'window');
});

test('sample identities support sorted and paginated result rows and reciprocal hover without changing the pin',()=>{
 const h=harness();h.render();const row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(samples()[3],3);h.document.append(row);
 h.document.emit('click',h.mark(0));h.document.emit('pointermove',row);assert(row.classList.contains('result-graph-hovered'));assert(h.mark(3).classList.contains('rt-point-hovered'));assert(h.mark(0).classList.contains('rt-point-selected'));assert.match(h.detail(),/Sample1/);assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 4/);
 h.document.emit('pointerout',row,{relatedTarget:null});assert(!row.classList.contains('result-graph-hovered'));h.document.emit('pointermove',h.mark(3));assert(row.classList.contains('result-graph-hovered'));
 assert.equal(h.api.sampleKey(samples()[3],3),h.api.sampleKey(samples()[3],0),'Sequence/time identity ignores sorted row position');
});

test('older retained row hover reports an honest moving-window limit and progressive hover reveals its point',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,i%20,'2026-10-26T16:00:00Z'));h.render(rows);let row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(rows[0],0);h.document.append(row);
 h.document.emit('pointermove',row);assert.match(h.document.querySelector('.rt-hover-context').textContent,/Sample 1 is outside the moving window/);assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 1/);
 h.api.setPreferences({mode:'progressive'},{persist:false});h.render(rows);row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(rows[0],0);h.document.append(row);h.observe();h.document.emit('pointermove',row);
 assert(h.document.querySelector('[data-rt-surface="latency"] [data-rt-point="0"]').classList.contains('rt-point-hovered'));assert.equal(h.document.querySelector('[data-rt-scroll="latency"]').scrollLeft,0);
 h.api.setPreferences({mode:'window'},{persist:false});
});

test('original pinned timestamp stays available after source retention removes the sample',()=>{
 const h=harness(),rows=Array.from({length:180},(_,i)=>reply(i+1,10,'2026-10-26T16:00:00Z'));h.render(rows);h.document.emit('click',h.mark(0));h.render(Array.from({length:10000},(_,i)=>reply(i+2,20,'2026-10-26T16:00:01Z')));
 assert.match(h.detail(),/Sample1/);assert.match(h.detail(),/LOCAL 2026-10-26T16:00:00Z/);assert.match(h.document.querySelector('.rt-selection-context').textContent,/no longer in retained history/);assert.equal(h.document.querySelector('[data-rt-clear]').disabled,false);
});

test('progressive time-axis endpoints describe the visible scroll viewport',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,new Date(Date.parse('2026-10-26T16:00:00Z')+i*1000).toISOString()));h.api.setPreferences({mode:'progressive'},{persist:false});h.render(rows);
 const scroll=h.document.querySelector('[data-rt-scroll="latency"]');scroll.clientWidth=600;scroll.scrollWidth=3200;scroll.scrollLeft=0;h.document.emit('scroll',scroll);
 let axis=h.document.querySelector('.rt-time-axis').querySelectorAll('time');assert.equal(axis[0].textContent,'LOCAL '+rows[0].time);assert.equal(axis[1].textContent,'LOCAL '+rows[73].time);
 scroll.scrollLeft=1600;h.document.emit('scroll',scroll);axis=h.document.querySelector('.rt-time-axis').querySelectorAll('time');assert.equal(axis[0].textContent,'LOCAL '+rows[199].time);assert.equal(axis[1].textContent,'LOCAL '+rows[273].time);h.api.setPreferences({mode:'window'},{persist:false});
});

test('moving-window zoom changes sample range, hit testing and axis accurately, then resets',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,new Date(Date.parse('2026-10-26T16:00:00Z')+i*1000).toISOString()));h.render(rows);
 assert.equal(h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]').length,180);
 h.document.emit('click',h.document.querySelector('[data-rt-zoom="in"]'));
 assert.equal(h.document.querySelector('[data-rt-zoom-level]').textContent,'2x');assert.equal(h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]').length,90);
 let axis=h.document.querySelector('.rt-time-axis').querySelectorAll('time');assert.equal(axis[0].textContent,'LOCAL '+rows[310].time);assert.equal(axis[1].textContent,'LOCAL '+rows[399].time);
 let surface=h.document.querySelector('[data-rt-surface="latency"]');surface.getBoundingClientRect=()=>({left:10,width:820});h.document.emit('pointermove',surface,{clientX:19});assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 311/);
 h.document.emit('pointermove',surface,{clientX:821});assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 400/);
 h.document.emit('click',h.document.querySelector('[data-rt-zoom="out"]'));assert.equal(h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]').length,180);
 h.api.zoom('job','out');assert.equal(h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]').length,360);
 h.document.emit('click',h.document.querySelector('[data-rt-zoom="reset"]'));assert.equal(h.document.querySelector('[data-rt-zoom-level]').textContent,'1x');assert.equal(h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]').length,180);
 assert.deepEqual(rows.map(row=>row.sequence),Array.from({length:400},(_,i)=>i+1),'Zoom never mutates recorded rows');
});

test('progressive zoom scales coordinates and axes while bounding the SVG at every scale',()=>{
 const h=harness(),rows=Array.from({length:10000},(_,i)=>reply(i+1,10,new Date(Date.parse('2026-10-26T16:00:00Z')+i*1000).toISOString()));h.api.setPreferences({mode:'progressive'},{persist:false});h.render(rows);
 let scroll=h.document.querySelector('[data-rt-scroll="latency"]');scroll.clientWidth=600;scroll.scrollWidth=80010;scroll.scrollLeft=1600;h.document.emit('scroll',scroll);
 h.api.zoom('job',2);scroll=h.document.querySelector('[data-rt-scroll="latency"]');scroll.clientWidth=600;scroll.scrollWidth=160010;scroll.scrollLeft=1600;h.document.emit('scroll',scroll);
 let surface=h.document.querySelector('[data-rt-surface="latency"]');surface.getBoundingClientRect=()=>({left:0,width:600});h.document.emit('pointermove',surface,{clientX:9});assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 101/);
 let axis=h.document.querySelector('.rt-time-axis').querySelectorAll('time');assert.equal(axis[0].textContent,'LOCAL '+rows[100].time);assert.equal(axis[1].textContent,'LOCAL '+rows[136].time);
 assert(h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]').length<45);
 h.api.zoom('job',.25);assert(h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]').length<420,'Zoom out virtualizes the 10k history');
 h.api.zoom('job',999);assert.equal(h.document.querySelector('[data-rt-zoom-level]').textContent,'16x');assert(h.document.querySelector('[data-rt-zoom="in"]').hasAttribute('disabled'));
 h.api.zoom('job','reset');assert.equal(h.document.querySelector('[data-rt-zoom-level]').textContent,'1x');assert.equal(h.api.preferences().mode,'progressive');
});

test('outside clicks clear a persistent pin and keep filters, while pointer leave alone preserves it',()=>{
 const h=harness();let filter={active:true,statistic:'maximum'},clears=0;h.context.NetworkToolkitResultFilters={current:()=>filter,graphMatches:(_,raw)=>raw.sequence===4,clearStatistic:()=>clears++};h.render();
 h.document.emit('click',h.mark(1));h.document.emit('pointermove',h.mark(1));h.document.emit('pointerout',h.mark(1),{relatedTarget:null});assert(h.mark(1).classList.contains('rt-point-selected'));
 const statistic=h.document.createElement('button');statistic.dataset.statistic='loss';h.document.append(statistic);h.document.emit('click',statistic);
 assert.equal(h.document.querySelector('[data-rt-clear]').disabled,true);assert(!h.mark(1).classList.contains('rt-point-selected'));assert.equal(clears,0);assert.equal(filter.statistic,'maximum');
 h.document.emit('click',h.mark(2));h.document.emit('click',h.document.querySelector('[data-rt-zoom="in"]'));assert.match(h.detail(),/Sample3/,'Graph controls keep selected measurements');h.document.emit('click',h.document);assert.equal(h.document.querySelector('[data-rt-clear]').disabled,true);
});

test('latest graph control respects active matching samples in both modes without resetting zoom or preferences',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,new Date(Date.parse('2026-10-26T16:00:00Z')+i*1000).toISOString()));let active=false;
 h.context.NetworkToolkitResultFilters={current:()=>({active}),graphMatches:(_,raw)=>raw.sequence<=200};h.render(rows);h.api.zoom('job',2);active=true;h.document.emit('click',h.document.querySelector('[data-rt-latest]'));
 assert.equal(h.document.querySelector('[data-rt-zoom-level]').textContent,'2x');assert.equal(h.document.querySelector('.rt-time-axis').querySelectorAll('time')[1].textContent,'LOCAL '+rows[199].time);assert.equal(active,true);
 h.api.setPreferences({mode:'progressive'},{persist:false});h.render(rows);h.observe();let scroll=h.document.querySelector('[data-rt-scroll="latency"]');assert(scroll.scrollLeft<3200,'Latest locates matching sample instead of blindly using the end of the history');
 scroll.clientWidth=600;scroll.scrollWidth=6400;scroll.scrollLeft=0;h.document.emit('scroll',scroll);h.document.emit('click',h.document.querySelector('[data-rt-latest]'));assert(scroll.scrollLeft>2500&&scroll.scrollLeft<3300);assert.equal(h.document.querySelector('[data-rt-zoom-level]').textContent,'2x');assert.equal(h.api.preferences().mode,'progressive');
});

test('graph marks use actual status palette categories and combined filters retain raw source values',()=>{
 const h=harness();let filter={active:true,statistic:'average',reference:35,referenceKind:'run'},seen=[];const rows=[reply(1,1000),{...reply(2,5),success:false,status:'Timed out'},{...reply(3,5),success:false,status:'Cancelled'},reply(4,30)];
 h.context.NetworkToolkitResultFilters={current:()=>filter,graphMatches:(_,raw)=>{seen.push(raw);return raw.sequence===1||raw.sequence===4;},severity:raw=>raw.success?'good':raw.status==='Cancelled'?'warn':'bad'};h.render(rows);
 assert(h.mark(0).classList.contains('rt-severity-good'),'High measured RTT is successful rather than falsely negative');assert(h.mark(1).classList.contains('rt-severity-bad'));assert(h.mark(2).classList.contains('rt-severity-warn'));assert(h.mark(0).classList.contains('rt-point-filtered'));assert(h.mark(1).classList.contains('rt-point-filter-dimmed'));assert(seen.every(raw=>rows.includes(raw)));
 assert(h.document.querySelector('[data-rt-stat-reference]'));assert.match(h.document.querySelector('.rt-filter-note').textContent,/Run average RTT reference: 35 ms/);
 h.document.emit('click',h.mark(1));h.document.emit('pointermove',h.mark(2));assert(h.mark(1).classList.contains('rt-severity-bad'));assert(h.mark(1).classList.contains('rt-point-selected'));assert(h.mark(2).classList.contains('rt-severity-warn'));assert(h.mark(2).classList.contains('rt-point-hovered'));
 filter={active:false};h.document.emit('networkToolkitResultFilterChange',h.document,{detail:{jobId:'job'}});assert(!h.document.querySelector('[data-rt-stat-reference]'));assert(!h.mark(0).classList.contains('rt-point-filtered'));assert(!h.mark(1).classList.contains('rt-point-filter-dimmed'));assert.deepEqual(rows.map(row=>row.rttMs),[1000,5,5,30]);
});

test('hover and keyboard inspection pause list tracking only while a measurement is being inspected',()=>{
 const h=harness(),calls=[];h.context.NetworkToolkitGraphWorkspace={inspection:(id,on)=>calls.push([id,on]),revealSample:()=>true};h.render();
 h.document.emit('pointermove',h.mark(0));assert.deepEqual(calls.at(-1),['job',true]);h.document.emit('click',h.mark(0));h.document.emit('pointerout',h.mark(0),{relatedTarget:null});assert.deepEqual(calls.at(-1),['job',false]);assert(h.mark(0).classList.contains('rt-point-selected'),'Pinned point survives resuming list tracking');
 const surface=h.document.querySelector('[data-rt-surface="latency"]');h.document.emit('keydown',surface,{key:'End'});assert.deepEqual(calls.at(-1),['job',true]);h.document.emit('focusout',surface,{relatedTarget:h.document.querySelector('[data-rt-latest]')});assert.deepEqual(calls.at(-1),['job',false]);
 const row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(samples()[2],2);h.document.append(row);h.document.emit('pointermove',row);assert.deepEqual(calls.at(-1),['job',true]);h.document.emit('pointerout',row,{relatedTarget:null});assert.deepEqual(calls.at(-1),['job',false]);
});

test('sequence gaps never invent jitter pairs or connect across unrecorded probes',()=>{
 const rows=[reply(1,10),reply(3,30),reply(4,25),{...reply(5,50),success:false,status:'Lost'},reply(6,15),reply(7,20)];
 assert.deepEqual(timeline.normalize(rows).map(point=>point.jitter),[null,null,5,null,null,5]);assert.equal(timeline.summary(timeline.normalize(rows)).jitterPairs,2);
 const h=harness();h.render(rows);const lines=h.document.querySelector('[data-rt-surface="latency"]').querySelectorAll('polyline');assert.equal(lines.length,2,'Latency connects only the observed consecutive pairs, with failures breaking the line');
 const legacy=[{rttMs:2,success:true},{rttMs:4,success:true}];assert.equal(timeline.normalize(legacy)[1].jitter,2,'Legacy records without recorded sequence remain usable');
});

test('zoom can inspect a pinned older measurement, while graph Latest returns to the newest viewport',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,new Date(Date.parse('2026-10-26T16:00:00Z')+i*1000).toISOString()));h.render(rows);h.document.emit('click',h.mark(20));h.api.zoom('job',4);
 assert.match(h.detail(),/Sample241/);assert(h.document.querySelector('[data-rt-sequence="241"]').classList.contains('rt-point-selected'));
 let last=h.document.querySelector('.rt-time-axis').querySelectorAll('time')[1].textContent;assert.notEqual(last,'LOCAL '+rows[399].time);
 h.document.emit('click',h.document.querySelector('[data-rt-latest]'));last=h.document.querySelector('.rt-time-axis').querySelectorAll('time')[1].textContent;assert.equal(last,'LOCAL '+rows[399].time);assert.match(h.detail(),/Sample241/,'Latest moves the viewport and retains the recorded pin');assert.equal(h.document.querySelector('[data-rt-zoom-level]').textContent,'4x');
});

test('restoration settles without repeatedly replacing unchanged detail nodes',()=>{
 const h=harness();h.render();h.document.emit('click',h.mark(0));h.observe();
 const details=h.document.querySelector('.rt-detail').firstElementChild,context=h.document.querySelector('.rt-selection-context').firstElementChild;
 for(let pass=0;pass<4;pass++)h.observe();
 assert.equal(h.document.querySelector('.rt-detail').firstElementChild,details);assert.equal(h.document.querySelector('.rt-selection-context').firstElementChild,context,'Mutation restoration must settle without child-list feedback');
});

test('removed hovered rows, hidden graphs and tool switches release temporary tracking pauses',()=>{
 const h=harness(),calls=[];h.context.NetworkToolkitGraphWorkspace={inspection:(id,on)=>calls.push([id,on]),revealSample:()=>true};h.render();
 const row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(samples()[2],2);h.document.append(row);h.document.emit('pointermove',row);assert.deepEqual(calls.at(-1),['job',true]);row.remove();h.observe();assert.deepEqual(calls.at(-1),['job',false]);assert.equal(h.document.querySelector('[data-rt-tooltip]').hidden,true);
 h.document.emit('pointermove',h.mark(0));h.document.querySelector('.rt-timeline').setAttribute('hidden','');h.observe();assert.deepEqual(calls.at(-1),['job',false]);h.document.querySelector('.rt-timeline').removeAttribute('hidden');
 const surface=h.document.querySelector('[data-rt-surface="latency"]');h.document.emit('keydown',surface,{key:'Home'});assert.deepEqual(calls.at(-1),['job',true]);h.render(samples(),'other');h.observe();assert(calls.some(([id,on])=>id==='job'&&on===false),'A detached tool must not preserve its keyboard inspection pause');
});

test('repeated restoration retains actual timeline control nodes despite browser serialization differences',()=>{
 for(const mode of ['window','progressive']){
  const h=harness();h.context.NetworkToolkitResultFilters={current:()=>({active:true,statistic:'average',reference:18.5,referenceKind:'run'}),graphMatches:()=>true};h.api.setPreferences({mode},{persist:false});h.render();h.document.emit('click',h.mark(0));h.observe();
  const selectors=['[data-rt-clear]','[data-rt-zoom="in"]','[data-rt-zoom="reset"]','[data-rt-latest]','[data-rt-mode]',...(mode==='progressive'?['[data-rt-follow]']:[])],controls=selectors.map(selector=>h.document.querySelector(selector));
  let serializedReads=0;for(const node of [h.document.querySelector('.rt-detail'),h.document.querySelector('.rt-selection-context'),...h.document.querySelectorAll('[data-rt-surface]')]){const descriptor=Object.getOwnPropertyDescriptor(Object.getPrototypeOf(node),'innerHTML');Object.defineProperty(node,'innerHTML',{get(){serializedReads++;return '<browser-canonical-serialization data-empty=""></browser-canonical-serialization>';},set(value){descriptor.set.call(this,value);}});}
  for(let pass=0;pass<32;pass++)h.observe();
  assert.equal(serializedReads,0,'Restore compares prepared semantic content rather than browser-normalized HTML');
  selectors.forEach((selector,index)=>{assert.equal(h.document.querySelector(selector),controls[index],mode+' keeps the exact '+selector+' element attached');assert.equal(controls[index].isConnected,true);});
 }
});
