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
