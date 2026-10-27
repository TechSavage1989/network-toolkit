'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const timeline=require('../web/assets/result-timeline.js');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const stamp=n=>new Date(Date.parse('2026-10-26T16:00:00.123Z')+n*1000).toISOString();
const reply=(sequence,rttMs,time=stamp(sequence))=>({sequence,rttMs,time,success:true,status:'Reply',address:'192.0.2.20'});
const samples=()=>[reply(1,10),reply(2,14),{...reply(3,999),success:false,status:'Timed out'},reply(4,30),reply(5,20)];

function harness(){
 const document=makeDocument('');document.body=document;document.activeElement=document;
 let observer,selected=null;
 const context={document,console,Intl,Date,NetworkToolkitResultTime:{format:value=>value==null||value===''?'Not recorded':'LOCAL '+value},MutationObserver:class{constructor(fn){observer=fn;}observe(){}}};
 context.window=context;vm.createContext(context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/graph-history.js'),'utf8'),context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/result-timeline.js'),'utf8'),context);
 const api=context.NetworkToolkitResultTimeline;
 context.NetworkToolkitResultDetails={showGraphSelection:value=>{selected=value;let box=document.querySelector('.mock-entry-details');if(!box){box=document.createElement('aside');box.className='mock-entry-details';box.innerHTML='<div class="mock-point-details"></div><p class="mock-selection-context"></p>';document.append(box);}const content=api.pointDetails(value.point),detail=box.querySelector('.mock-point-details'),note=box.querySelector('.mock-selection-context');if(detail._source!==content){detail.innerHTML=content;detail._source=content;}if(note.textContent!==value.message)note.textContent=value.message;},clearGraphSelection:()=>{selected=null;document.querySelector('.mock-entry-details')?.remove();}};

 function render(rows=samples(),id='job'){
  for(const child of [...document.children])child.remove();
  document.innerHTML=api.latency(rows,{id});
  for(const surface of document.querySelectorAll('[data-rt-surface]'))surface.focus=()=>{document.activeElement=surface;document.emit('focusin',surface);};
  api.restore();return document.querySelector('.rt-timeline');
 }
 return {context,document,api,render,observe:()=>observer?.(),detail:()=>selected?.id===document.querySelector('.rt-timeline')?.dataset.rtId?document.querySelector('.mock-point-details')?.textContent||'':document.querySelector('[data-rt-tooltip]')&&!document.querySelector('[data-rt-tooltip]').hidden?document.querySelector('[data-rt-tooltip]').textContent:'Hover over a point',mark:index=>document.querySelector('[data-rt-surface="latency"] [data-rt-point="'+index+'"]')};
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
 assert.equal((html.match(/data-rt-sequence=/g)||[]).length,180);assert.match(html,/Moving window/);
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
 h.document.emit('click',h.mark(1));assert.equal(h.api.isHeld('job'),true);
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
 h.document.emit('keydown',surface,{key:'Escape'});assert.match(h.detail(),/Hover over a point/);assert.equal(h.api.isHeld('job'),false);
});

test('pinned selections are job-scoped and retain recorded details outside the moving window',()=>{
 const h=harness();h.render();h.document.emit('click',h.mark(0));
 h.render(samples(),'other-job');assert.match(h.detail(),/Hover over a point/);
 h.render(samples(),'job');assert.match(h.detail(),/Sample1/);
 h.render(Array.from({length:181},(_,i)=>reply(i+1,10,'2026-10-26T16:00:00Z')),'job');
 assert.match(h.detail(),/Sample1/);assert.match(h.document.querySelector('.mock-selection-context').textContent,/outside the displayed graph range|retained as a snapshot/);
 assert.equal(!h.api.isHeld('job'),false);
});

test('plot tabs show only the chosen surface and preserve held entry and keyboard focus across live rendering',()=>{
 const h=harness();h.render();h.document.emit('click',h.mark(1));h.api.setPlot('job','jitter');let surface=h.document.querySelector('[data-rt-surface="jitter"]');assert(surface);assert.equal(h.document.querySelector('[data-rt-surface="latency"]'),null);assert.match(h.detail(),/Sample2/);
 surface.focus=()=>{h.document.activeElement=surface;h.document.emit('focusin',surface);};surface.focus();h.render([...samples(),reply(6,18)]);assert(h.document.querySelector('[data-rt-surface="jitter"]'));assert.equal(h.document.querySelectorAll('[data-rt-surface]').length,1);assert.match(h.detail(),/Sample2/);
});

test('plot coordinate hover chooses the nearest actual point when no marker is hit',()=>{
 const h=harness();h.render();const surface=h.document.querySelector('[data-rt-surface="latency"]');
 surface.getBoundingClientRect=()=>({left:10,width:820});
 h.document.emit('pointermove',surface,{clientX:10});assert.match(h.detail(),/Sample1/);
 h.document.emit('pointermove',surface,{clientX:830});assert.match(h.detail(),/Sample5/);
 h.api.clearSelection('job');assert.match(h.detail(),/Hover over a point/);
});

test('persistent selection and transient hover have distinct marks, details and popup',()=>{
 const h=harness();h.render();h.document.emit('click',h.mark(0));h.document.emit('pointermove',h.mark(3),{clientX:970,clientY:730});
 assert(h.mark(0).classList.contains('rt-point-selected'));assert(!h.mark(3).classList.contains('rt-point-selected'));assert(h.mark(3).classList.contains('rt-point-hovered'));
 assert.match(h.detail(),/Sample1/);const tooltip=h.document.querySelector('[data-rt-tooltip]');assert.match(tooltip.textContent,/Sample 4/);assert.match(tooltip.textContent,/30 ms/);assert.equal(tooltip.hidden,false);
 h.document.emit('pointerout',h.mark(3),{relatedTarget:null});assert.equal(tooltip.hidden,true);assert(h.mark(0).classList.contains('rt-point-selected'));assert(!h.mark(3).classList.contains('rt-point-hovered'));
 h.render([...samples(),reply(6,12)]);assert.match(h.detail(),/Sample1/);assert(h.mark(0).classList.contains('rt-point-selected'));
});

test('progressive mode fits retained timestamped history and keeps a manually panned viewport through live updates',()=>{
 const h=harness(),rows=Array.from({length:1000},(_,i)=>reply(i+1,10,stamp(i)));h.api.setPreferences({mode:'progressive'},{persist:false});h.render(rows);assert.equal(h.document.querySelectorAll('[data-rt-point]').length,1000);assert.equal(h.document.querySelector('[data-rt-scroll]'),null);
 h.api.zoom('job',2);h.api.pan('job',-.5);const axis=h.document.querySelector('.rt-time-axis').innerHTML;h.render([...rows,reply(1001,12,stamp(1000))]);assert.equal(h.document.querySelector('.rt-time-axis').innerHTML,axis);assert.equal(h.api.tracking('job').following,false);h.api.jumpLatest('job');assert.equal(h.api.tracking('job').following,true);assert.equal(h.api.normalize([...rows,...rows,...rows,...rows,...rows,...rows,...rows,...rows,...rows,...rows,...rows],{mode:'progressive'}).length,10000);
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
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,i%20,stamp(i)));h.render(rows);let row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(rows[0],0);h.document.append(row);
 h.document.emit('pointermove',row);assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/outside the displayed graph range/i);assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 1/);
 h.api.setPreferences({mode:'progressive'},{persist:false});h.render(rows);row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(rows[0],0);h.document.append(row);h.observe();h.document.emit('pointermove',row);
 assert(h.document.querySelector('[data-rt-surface="latency"] [data-rt-point="0"]').classList.contains('rt-point-hovered'));assert.equal(h.document.querySelector('[data-rt-scroll]'),null);
 h.api.setPreferences({mode:'window'},{persist:false});
});

test('original pinned timestamp stays available after source retention removes the sample',()=>{
 const h=harness(),rows=Array.from({length:180},(_,i)=>reply(i+1,10,'2026-10-26T16:00:00Z'));h.render(rows);h.document.emit('click',h.mark(0));h.render(Array.from({length:10000},(_,i)=>reply(i+2,20,'2026-10-26T16:00:01Z')));
 assert.match(h.detail(),/Sample1/);assert.match(h.detail(),/LOCAL 2026-10-26T16:00:00Z/);assert.match(h.document.querySelector('.mock-selection-context').textContent,/retained as a snapshot/);assert.equal(h.api.isHeld('job'),true);
});

test('progressive axis follows the selected time scope and data panning rather than a DOM scrollbar',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i)));h.api.setPreferences({mode:'progressive'},{persist:false});h.context.NetworkToolkitGraphHistory.setPreferences({durationMs:10000},{persist:false});h.render(rows);
 const times=()=>h.document.querySelector('.rt-time-axis').querySelectorAll('time'),ends=()=>[times()[0].getAttribute('datetime'),times().at(-1).getAttribute('datetime')];assert.equal(Date.parse(ends()[1])-Date.parse(ends()[0]),10000);assert.equal(Date.parse(ends()[1]),Date.parse(rows.at(-1).time));h.api.pan('job',-.5);assert.equal(Date.parse(ends()[1]),Date.parse(rows.at(-1).time)-5000);
});

test('time zoom changes range, hit testing and axis accurately and resets without changing recorded rows',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i))),original=JSON.stringify(rows);h.render(rows);assert.equal(h.document.querySelectorAll('[data-rt-point]').length,180);h.api.zoom('job','in');assert.equal(h.document.querySelectorAll('[data-rt-point]').length,90);
 let surface=h.document.querySelector('[data-rt-surface]');surface.getBoundingClientRect=()=>({left:10,width:820});h.document.emit('pointermove',surface,{clientX:19});assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 311/);h.api.zoom('job','out');assert.equal(h.document.querySelectorAll('[data-rt-point]').length,180);h.api.zoom('job','out');assert.equal(h.document.querySelectorAll('[data-rt-point]').length,359);h.api.zoom('job','reset');assert.equal(h.document.querySelectorAll('[data-rt-point]').length,180);assert.equal(JSON.stringify(rows),original);
});

test('progressive zoom uses time domains at bounded scales without a wide scrolling SVG',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i)));h.api.setPreferences({mode:'progressive'},{persist:false});h.render(rows);h.api.zoom('job',2);assert.equal(h.document.querySelectorAll('[data-rt-point]').length,200);assert.equal(h.api.zoom('job',999),16);assert(h.document.querySelectorAll('[data-rt-point]').length<30);assert.equal(h.document.querySelector('[data-rt-scroll]'),null);assert.equal(h.api.zoom('job','reset'),1);assert.equal(h.document.querySelectorAll('[data-rt-point]').length,400);
});

test('outside controls preserve a held pin and filters until explicit deselection',()=>{
 const h=harness();let filter={active:true,statistic:'maximum'},clears=0;h.context.NetworkToolkitResultFilters={current:()=>filter,graphMatches:(_,raw)=>raw.sequence===4,clearStatistic:()=>clears++};h.render();
 h.document.emit('click',h.mark(1));h.document.emit('pointermove',h.mark(1));h.document.emit('pointerout',h.mark(1),{relatedTarget:null});assert(h.mark(1).classList.contains('rt-point-selected'));
 const statistic=h.document.createElement('button');statistic.dataset.statistic='loss';h.document.append(statistic);h.document.emit('click',statistic);
 assert.equal(h.api.isHeld('job'),true);assert(h.mark(1).classList.contains('rt-point-selected'));assert.equal(clears,0);assert.equal(filter.statistic,'maximum');
 h.document.emit('click',h.mark(2));h.api.zoom('job','in');assert.match(h.detail(),/Sample3/,'Graph controls keep selected measurements');h.document.emit('click',h.document);assert.equal(h.api.isHeld('job'),true);h.api.clearSelection('job');assert.equal(h.api.isHeld('job'),false);
});

test('Follow latest respects matching samples in both modes without resetting zoom or preferences',()=>{
 for(const mode of ['window','progressive','condensed']){const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i)));h.context.NetworkToolkitResultFilters={current:()=>({active:true}),graphMatches:(_,row)=>row.sequence<=350};h.api.setPreferences({mode},{persist:false});h.render(rows);h.document.emit('click',h.mark(0));h.api.zoom('job',2);h.api.jumpLatest('job');assert.match(h.detail(),/Sample350/);assert.equal(h.api.tracking('job').following,true);assert.equal(h.api.isHeld('job'),false);assert.equal(h.api.preferences().mode,mode);assert.match(h.document.querySelector('[data-rt-options]').textContent,/2×/);}
});

test('graph marks use actual status palette categories and combined filters retain raw source values',()=>{
 const h=harness();let filter={active:true,statistic:'average',reference:35,referenceKind:'run'},seen=[];const rows=[reply(1,1000),{...reply(2,5),success:false,status:'Timed out'},{...reply(3,5),success:false,status:'Cancelled'},reply(4,30)];
 h.context.NetworkToolkitResultFilters={current:()=>filter,graphMatches:(_,raw)=>{seen.push(raw);return raw.sequence===1||raw.sequence===4;},severity:raw=>raw.success?'good':raw.status==='Cancelled'?'warn':'bad'};h.render(rows);
 assert(h.mark(0).classList.contains('rt-severity-good'),'High measured RTT is successful rather than falsely negative');assert(h.mark(1).classList.contains('rt-severity-bad'));assert(h.mark(2).classList.contains('rt-severity-warn'));assert(h.mark(0).classList.contains('rt-point-filtered'));assert(h.mark(1).classList.contains('rt-point-filter-dimmed'));assert(seen.every(raw=>rows.includes(raw)));
 assert(h.document.querySelector('[data-rt-stat-reference]'));assert.match(h.document.querySelector('[data-rt-stat-reference]').textContent,/Run average RTT: 35 ms/);
 h.document.emit('click',h.mark(1));h.document.emit('pointermove',h.mark(2));assert(h.mark(1).classList.contains('rt-severity-bad'));assert(h.mark(1).classList.contains('rt-point-selected'));assert(h.mark(2).classList.contains('rt-severity-warn'));assert(h.mark(2).classList.contains('rt-point-hovered'));
 filter={active:false};h.document.emit('networkToolkitResultFilterChange',h.document,{detail:{jobId:'job'}});assert(!h.document.querySelector('[data-rt-stat-reference]'));assert(!h.mark(0).classList.contains('rt-point-filtered'));assert(!h.mark(1).classList.contains('rt-point-filter-dimmed'));assert.deepEqual(rows.map(row=>row.rttMs),[1000,5,5,30]);
});

test('hover and keyboard inspection pause list tracking only while a measurement is being inspected',()=>{
 const h=harness(),calls=[];h.context.NetworkToolkitGraphWorkspace={inspection:(id,on)=>calls.push([id,on]),revealSample:()=>true};h.render();
 h.document.emit('pointermove',h.mark(0));assert.deepEqual(calls.at(-1),['job',true]);h.document.emit('click',h.mark(0));h.document.emit('pointerout',h.mark(0),{relatedTarget:null});assert.deepEqual(calls.at(-1),['job',false]);assert(h.mark(0).classList.contains('rt-point-selected'),'Pinned point survives ending transient inspection; the workspace hold remains separate');
 const surface=h.document.querySelector('[data-rt-surface="latency"]');h.document.emit('keydown',surface,{key:'End'});assert.deepEqual(calls.at(-1),['job',true]);h.document.emit('focusout',surface,{relatedTarget:h.document.querySelector('[data-rt-options]')});assert.deepEqual(calls.at(-1),['job',false]);
 const row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(samples()[2],2);h.document.append(row);h.document.emit('pointermove',row);assert.deepEqual(calls.at(-1),['job',true]);h.document.emit('pointerout',row,{relatedTarget:null});assert.deepEqual(calls.at(-1),['job',false]);
});

test('sequence gaps never invent jitter pairs or connect across unrecorded probes',()=>{
 const rows=[reply(1,10),reply(3,30),reply(4,25),{...reply(5,50),success:false,status:'Lost'},reply(6,15),reply(7,20)];
 assert.deepEqual(timeline.normalize(rows).map(point=>point.jitter),[null,null,5,null,null,5]);assert.equal(timeline.summary(timeline.normalize(rows)).jitterPairs,2);
 const h=harness();h.render(rows);const lines=h.document.querySelector('[data-rt-surface="latency"]').querySelectorAll('polyline');assert.equal(lines.length,2,'Latency connects only the observed consecutive pairs, with failures breaking the line');
 const legacy=[{rttMs:2,success:true},{rttMs:4,success:true}];assert.equal(timeline.normalize(legacy)[1].jitter,2,'Legacy records without recorded sequence remain usable');
});

test('zoom and panning retain an older entry, and Follow latest releases it while preserving zoom',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i)));h.render(rows);h.document.emit('click',h.mark(20));h.api.zoom('job',4);assert.match(h.detail(),/Sample241/);h.api.pan('job',-.5);assert.match(h.detail(),/Sample241/);h.api.jumpLatest('job');assert.match(h.detail(),/Sample400/);assert.equal(h.api.isHeld('job'),false);assert.match(h.document.querySelector('[data-rt-options]').textContent,/4×/);
});

test('restoration settles without repeatedly replacing unchanged detail nodes',()=>{
 const h=harness();h.render();h.document.emit('click',h.mark(0));h.observe();
 const details=h.document.querySelector('.mock-point-details').firstElementChild,context=h.document.querySelector('.mock-selection-context').firstElementChild;
 for(let pass=0;pass<4;pass++)h.observe();
 assert.equal(h.document.querySelector('.mock-point-details').firstElementChild,details);assert.equal(h.document.querySelector('.mock-selection-context').firstElementChild,context,'Mutation restoration must settle without child-list feedback');
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
  const selectors=['[data-rt-options]','[data-rt-mode]'],controls=selectors.map(selector=>h.document.querySelector(selector));
  let serializedReads=0;for(const node of [h.document.querySelector('.mock-point-details'),h.document.querySelector('.mock-selection-context'),...h.document.querySelectorAll('[data-rt-surface]')]){const descriptor=Object.getOwnPropertyDescriptor(Object.getPrototypeOf(node),'innerHTML');Object.defineProperty(node,'innerHTML',{get(){serializedReads++;return '<browser-canonical-serialization data-empty=""></browser-canonical-serialization>';},set(value){descriptor.set.call(this,value);}});}
  for(let pass=0;pass<32;pass++)h.observe();
  assert.equal(serializedReads,0,'Restore compares prepared semantic content rather than browser-normalized HTML');
  selectors.forEach((selector,index)=>{assert.equal(h.document.querySelector(selector),controls[index],mode+' keeps the exact '+selector+' element attached');assert.equal(controls[index].isConnected,true);});
 }
});

test('aggregate filter explanations remain escaped and accessible in compact graph options',()=>{
 const h=harness();let body='';h.context.openModal=(_,html)=>{body=html;};h.context.NetworkToolkitResultFilters={current:()=>({active:true,statistic:'average',reference:18.5,referenceKind:'run',message:'Average is an aggregate <img src=x onerror="unsafe()">'}),graphMatches:()=>true};h.render();h.document.emit('click',h.document.querySelector('[data-rt-options]'));assert.match(body,/Average is an aggregate &lt;img/);assert.doesNotMatch(body,/<img/);assert.match(body,/Run average RTT reference: 18.5 ms/);assert(h.document.querySelector('[data-rt-stat-reference]'));
});

test('switching plots preserves the panned viewport and cannot silently resume following',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10+i%9,stamp(i)));h.api.setPreferences({mode:'progressive'},{persist:false});h.render(rows);h.api.pan('job',-.5);const axis=h.document.querySelector('.rt-time-axis').innerHTML;h.api.setPlot('job','jitter');assert.equal(h.document.querySelector('.rt-time-axis').innerHTML,axis);assert.equal(h.api.tracking('job').following,false);assert.equal(h.document.querySelector('[data-rt-surface="latency"]'),null);
});

test('data-pan navigation survives a source render and releases only on Follow latest',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10+i%9,stamp(i)));h.api.setPreferences({mode:'progressive'},{persist:false});h.render(rows);h.api.zoom('job','in');h.api.pan('job',-.5);const axis=h.document.querySelector('.rt-time-axis').innerHTML;h.render([...rows,reply(401,10,stamp(400))]);assert.equal(h.document.querySelector('.rt-time-axis').innerHTML,axis);h.api.jumpLatest('job');assert.equal(h.api.tracking('job').following,true);assert.notEqual(h.document.querySelector('.rt-time-axis').innerHTML,axis);
});

test('condensed history renders aggregates with extrema and keeps recent results individual without changing raw samples',()=>{
 const h=harness(),rows=Array.from({length:7201},(_,i)=>reply(i+1,i===1?1000:i%19,new Date(Date.parse('2026-10-26T12:00:00Z')+i*1000).toISOString())),before=JSON.stringify(rows);h.api.setPreferences({mode:'condensed'},{persist:false});h.render(rows);const marks=h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]');assert(marks.length<250);const group=h.document.querySelector('[data-rt-surface="latency"] [data-rt-aggregate]');assert(group);assert(h.document.querySelector('.rt-aggregate-range'));h.document.emit('click',group);assert.match(h.document.querySelector('.mock-point-details').textContent,/Grouped samples/);assert.equal(h.api.isHeld('job'),true);h.api.selectResult('job',h.api.sampleKey(rows[1]));assert.match(h.detail(),/1,000 ms/);h.document.emit('click',h.document);assert.equal(h.api.isHeld('job'),true);h.api.jumpLatest('job');assert.equal(h.api.isHeld('job'),false);assert.match(h.detail(),/Sample7201/);assert.equal(JSON.stringify(rows),before);assert.equal(h.document.querySelector('.rt-detail'),null);
});
test('fixed time navigation stays anchored during live updates while Latest restores duration-preserving following',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,new Date(Date.parse('2026-10-26T12:00:00Z')+i*1000).toISOString()));h.context.NetworkToolkitGraphHistory.setPreferences({anchor:'end',end:rows[199].time,durationMs:60000},{persist:false});h.render(rows);assert.equal(h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]').length,61);let axis=h.document.querySelector('.rt-time-axis').querySelectorAll('time');assert.equal(Date.parse(axis.at(-1).getAttribute('datetime')),Date.parse(rows[199].time));h.api.selectResult('job',h.api.sampleKey(rows[180]));h.render([...rows,reply(401,20,'2026-10-26T13:00:00Z')]);assert.match(h.detail(),/Sample181/);h.api.jumpLatest('job');assert.equal(h.context.NetworkToolkitGraphHistory.preferences().durationMs,60000);assert.equal(h.context.NetworkToolkitGraphHistory.preferences().anchor,'latest');assert.match(h.detail(),/Sample401/);
});

test('chosen time range never includes an RTT change whose previous source sample is outside it',()=>{
 const h=harness(),rows=Array.from({length:5},(_,i)=>reply(i+1,[1,5,100,104,108][i],'2026-10-26T16:00:0'+i+'Z'));h.context.NetworkToolkitGraphHistory.setPreferences({anchor:'range',start:rows[2].time,end:rows[4].time},{persist:false});h.render(rows);h.api.setPlot('job','jitter');assert(h.document.querySelector('[data-rt-surface="jitter"]'));h.api.setPlot('job','rtt');h.document.emit('click',h.mark(0));assert.match(h.detail(),/Response time100 ms/);h.document.emit('click',h.mark(1));assert.match(h.detail(),/RTT change4 ms/);
});

test('a held aggregate can open an original contributor after it leaves retained graph history',()=>{
 const h=harness(),rows=Array.from({length:300},(_,i)=>reply(i+1,5+i%30,new Date(Date.parse('2026-10-26T12:00:00Z')+i*1000).toISOString()));h.api.setPreferences({mode:'condensed'},{persist:false});h.render(rows);const first=h.document.querySelector('[data-rt-surface="latency"] [data-rt-aggregate]');assert(first);h.document.emit('click',first);assert.match(h.detail(),/Grouped samples/);h.render(rows.map(row=>({...row,sequence:row.sequence+1000,time:new Date(Date.parse(row.time)+1000000).toISOString()})));assert.equal(h.api.selectResult('job',h.api.sampleKey(rows[0],0)),true);assert.match(h.detail(),/Sample1/);assert.match(h.document.querySelector('.mock-selection-context').textContent,/snapshot/);
});
