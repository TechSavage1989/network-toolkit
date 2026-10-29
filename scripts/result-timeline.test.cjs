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
 context.window=context;context.CustomEvent=class{constructor(type,{detail}){this.type=type;this.detail=detail;}};document.dispatchEvent=event=>document.emit(event.type,document,{detail:event.detail});vm.createContext(context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/graph-history.js'),'utf8'),context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/graph-navigator.js'),'utf8'),context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/result-timeline.js'),'utf8'),context);
 const api=context.NetworkToolkitResultTimeline;
 context.NetworkToolkitResultDetails={showGraphSelection:value=>{selected=value;let box=document.querySelector('.mock-entry-details');if(!box){box=document.createElement('aside');box.className='mock-entry-details';box.innerHTML='<div class="mock-point-details"></div><p class="mock-selection-context"></p>';document.append(box);}const content=api.pointDetails(value.point),detail=box.querySelector('.mock-point-details'),note=box.querySelector('.mock-selection-context');if(detail._source!==content){detail.innerHTML=content;detail._source=content;}if(note.textContent!==value.message)note.textContent=value.message;},clearGraphSelection:()=>{selected=null;document.querySelector('.mock-entry-details')?.remove();}};

 function render(rows=samples(),id='job',options={}){
  for(const child of [...document.children])child.remove();
  document.innerHTML=api.latency(rows,{id,...options});
  for(const surface of document.querySelectorAll('[data-rt-surface]'))surface.focus=()=>{document.activeElement=surface;document.emit('focusin',surface);};
  api.restore();return document.querySelector('.rt-timeline');
 }
 return {context,document,api,render,observe:()=>observer?.(),detail:()=>selected?.id===document.querySelector('.rt-timeline')?.dataset.rtId?document.querySelector('.mock-point-details')?.textContent||'':document.querySelector('[data-rt-tooltip]')&&!document.querySelector('[data-rt-tooltip]').hidden?document.querySelector('[data-rt-tooltip]').textContent:'Hover over a point',mark:index=>document.querySelector('[data-rt-surface="latency"] [data-rt-point="'+index+'"]')};
}

function clusterSize(h,rows,prefix=0){const svg=h.document.querySelector('[data-rt-surface]'),history=h.context.NetworkToolkitGraphHistory,end=history.instant(h.document.querySelector('.rt-timeline').dataset.rtScopeEnd),cutoff=end-BigInt(Math.round(Number(svg.dataset.rtHorizonMs)*1000000));return prefix+rows.slice(-10000).filter(row=>history.instant(row.time)<=cutoff).length;}
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

test('repeated restoration assigns Boolean hover state instead of toggling unrelated ordinary points',()=>{
 for(const selected of [false,true])for(const hovered of [false,true]){
  const h=harness();h.render();
  if(selected)h.document.emit('click',h.mark(0));
  if(hovered)h.document.emit('pointermove',h.mark(3));
  const original=[...h.document.querySelectorAll('[data-rt-point]')];
  for(const mark of original){const toggle=mark.classList.toggle;mark.classList.toggle=(name,force)=>{if(name==='rt-point-hovered')assert.equal(typeof force,'boolean','Every hover assignment has an explicit Boolean force');return toggle(name,force);};}
  for(let cycle=0;cycle<8;cycle++){
   h.observe();const marks=[...h.document.querySelectorAll('[data-rt-point]')];
   assert(marks.every((mark,index)=>mark===original[index]),'Class updates preserve the same point nodes');
   assert.deepEqual(marks.filter(mark=>mark.classList.contains('rt-point-hovered')).map(mark=>mark.dataset.rtKey),hovered?[original[3].dataset.rtKey]:[]);
   assert.deepEqual(marks.filter(mark=>mark.classList.contains('rt-point-selected')).map(mark=>mark.dataset.rtKey),selected?[original[0].dataset.rtKey]:[]);
  }
  if(hovered)h.document.emit('pointerout',original[3],{relatedTarget:null});
  for(let cycle=0;cycle<8;cycle++){h.api.restore();assert.equal(h.document.querySelectorAll('.rt-point-hovered').length,0,'Pointer leave remains clear across later restores');}
 }
});

test('condensed aggregate and ordinary membership stay stable on restore, including row-origin hover',()=>{
 const h=harness(),rows=Array.from({length:301},(_,i)=>reply(i+1,10,stamp(i)));
 h.api.setPreferences({mode:'condensed'},{persist:false});h.render(rows);
 const cluster=h.document.querySelector('[data-rt-aggregate]'),recent=h.document.querySelector('[data-rt-sequence="301"]');assert(cluster);assert(recent);
 const check=(hovered,selected)=>{const original=[...h.document.querySelectorAll('[data-rt-point]')];for(let cycle=0;cycle<8;cycle++){h.observe();const marks=[...h.document.querySelectorAll('[data-rt-point]')];assert(marks.every((mark,index)=>mark===original[index]));assert.deepEqual(marks.filter(mark=>mark.classList.contains('rt-point-hovered')).map(mark=>mark.dataset.rtKey),hovered.map(mark=>mark.dataset.rtKey));assert.deepEqual(marks.filter(mark=>mark.classList.contains('rt-point-selected')).map(mark=>mark.dataset.rtKey),selected.map(mark=>mark.dataset.rtKey));}};
 check([],[]);h.document.emit('pointermove',cluster);check([cluster],[]);
 h.document.emit('click',cluster);check([cluster],[cluster]);
 h.document.emit('pointermove',recent);check([recent],[cluster]);
 h.document.emit('pointerout',recent,{relatedTarget:null});check([],[cluster]);
 const row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(rows[0],0);h.document.append(row);
 h.document.emit('pointermove',row);check([cluster],[cluster]);assert(row.classList.contains('result-graph-hovered'));
 h.document.emit('pointerout',row,{relatedTarget:null});check([],[cluster]);assert(!row.classList.contains('result-graph-hovered'));
});

test('linear defaults retain all samples up to the existing retention bound without changing source identities',()=>{
 const rows=Array.from({length:220},(_,i)=>reply(i+1,i+1,'2026-10-26T16:00:00Z'));
 const points=timeline.normalize(rows);
 assert.equal(points.length,220);assert.equal(points[0].sequence,1);assert.equal(points.at(-1).sequence,220);
 assert.equal(points[0].jitter,null,'No jitter pair is implied outside the displayed window');
 assert.equal(timeline.summary(points).min,1);
 for(const mode of ['linear','window','progressive','condensed']){const retained=timeline.normalize(Array.from({length:10020},(_,i)=>reply(i+1,i+1)),{mode});assert.equal(retained.length,10000);assert.equal(retained[0].sequence,21);assert.equal(retained.at(-1).sequence,10020);}
 const html=timeline.latency(rows,{id:'retained'});
 assert.equal((html.match(/data-rt-sequence=/g)||[]).length,220);assert.doesNotMatch(html,/Moving window|Recent 180|Full history/);
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

test('pinned selections are job-scoped and retain recorded details outside the chosen timeframe',()=>{
 const h=harness();h.render();h.document.emit('click',h.mark(0));
 h.render(samples(),'other-job');assert.match(h.detail(),/Hover over a point/);
 h.render(samples(),'job');assert.match(h.detail(),/Sample1/);
 h.context.NetworkToolkitGraphHistory.setPreferences({anchor:'range',start:stamp(100),end:stamp(180)},{persist:false});h.render(Array.from({length:181},(_,i)=>reply(i+1,10)),'job');
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
 h.document.emit('pointermove',surface,{clientX:74});assert.match(h.detail(),/Sample1/);
 h.document.emit('pointermove',surface,{clientX:818});assert.match(h.detail(),/Sample5/);
 h.api.clearSelection('job');assert.match(h.detail(),/Hover over a point/);
});

test('persistent selection and transient hover have distinct marks, details and popup',()=>{
 const h=harness();h.render();h.document.emit('click',h.mark(0));h.document.emit('pointermove',h.mark(3),{clientX:970,clientY:730});
 assert(h.mark(0).classList.contains('rt-point-selected'));assert(!h.mark(3).classList.contains('rt-point-selected'));assert(h.mark(3).classList.contains('rt-point-hovered'));
 assert.match(h.detail(),/Sample1/);const tooltip=h.document.querySelector('[data-rt-tooltip]');assert.match(tooltip.textContent,/Sample 4/);assert.match(tooltip.textContent,/30 ms/);assert.equal(tooltip.hidden,false);
 h.document.emit('pointerout',h.mark(3),{relatedTarget:null});assert.equal(tooltip.hidden,true);assert(h.mark(0).classList.contains('rt-point-selected'));assert(!h.mark(3).classList.contains('rt-point-hovered'));
 h.render([...samples(),reply(6,12)]);assert.match(h.detail(),/Sample1/);assert(h.mark(0).classList.contains('rt-point-selected'));
});

test('linear mode fits retained timestamped history and keeps a manually panned viewport through live updates',()=>{
 const h=harness(),rows=Array.from({length:1000},(_,i)=>reply(i+1,10,stamp(i)));h.api.setPreferences({mode:'linear'},{persist:false});h.render(rows);assert.equal(h.document.querySelectorAll('[data-rt-point]').length,1000);assert.equal(h.document.querySelector('[data-rt-scroll]'),null);
 h.api.zoom('job',2);h.api.pan('job',-.5);const axis=h.document.querySelector('.rt-time-axis').innerHTML;h.render([...rows,reply(1001,12,stamp(1000))]);assert.equal(h.document.querySelector('.rt-time-axis').innerHTML,axis);assert.equal(h.api.tracking('job').following,false);h.api.jumpLatest('job');assert.equal(h.api.tracking('job').following,true);assert.equal(h.api.normalize([...rows,...rows,...rows,...rows,...rows,...rows,...rows,...rows,...rows,...rows,...rows],{mode:'linear'}).length,10000);
});

test('saved graph mode is normalized, persisted through host state and merged without losing a pin',()=>{
 const h=harness();let saves=0;h.context.state={};h.context.saveState=()=>saves++;h.api.setPreferences({mode:'linear'});assert.equal(saves,1);assert.equal(h.context.state.graphPreferences.mode,'linear');
 h.render();h.document.emit('click',h.mark(1));h.api.mergePreferences({graphPreferences:{mode:'linear'}});h.render();assert.equal(h.api.preferences().mode,'linear');assert.match(h.detail(),/Sample2/);
 h.api.mergePreferences({graphPreferences:{mode:'invalid'}});assert.equal(h.api.preferences().mode,'linear');
});

test('sample identities support sorted and paginated result rows and reciprocal hover without changing the pin',()=>{
 const h=harness();h.render();const row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(samples()[3],3);h.document.append(row);
 h.document.emit('click',h.mark(0));h.document.emit('pointermove',row);assert(row.classList.contains('result-graph-hovered'));assert(h.mark(3).classList.contains('rt-point-hovered'));assert(h.mark(0).classList.contains('rt-point-selected'));assert.match(h.detail(),/Sample1/);assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 4/);
 h.document.emit('pointerout',row,{relatedTarget:null});assert(!row.classList.contains('result-graph-hovered'));h.document.emit('pointermove',h.mark(3));assert(row.classList.contains('result-graph-hovered'));
 assert.equal(h.api.sampleKey(samples()[3],3),h.api.sampleKey(samples()[3],0),'Sequence/time identity ignores sorted row position');
});

test('retained row hover reveals and highlights the original source in condensed and panned linear views',()=>{
 for(const view of ['condensed','zoomed-linear']){
  const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,i%20,stamp(i)));h.render(rows);let row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(rows[0],0);h.document.append(row);
  h.document.emit('pointermove',row);assert(h.document.querySelector('[data-rt-point][data-rt-sequence="1"]'));assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 1/);h.document.emit('pointerout',row,{relatedTarget:null});
  if(view==='condensed')h.api.setPreferences({mode:'condensed'},{persist:false});else h.api.zoom('job',2);
  h.render(rows);row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(rows[0],0);h.document.append(row);h.observe();h.document.emit('pointermove',row);
  assert(h.document.querySelector(view==='condensed'?'[data-rt-surface="latency"] [data-rt-aggregate]':'[data-rt-surface="latency"] [data-rt-sequence="1"]').classList.contains('rt-point-hovered'),view+' highlights the exact revealed source');assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 1/);assert.equal(h.document.querySelector('[data-rt-scroll]'),null);
 }
});

test('original pinned timestamp stays available after source retention removes the sample',()=>{
 const h=harness(),rows=Array.from({length:180},(_,i)=>reply(i+1,10,'2026-10-26T16:00:00Z'));h.render(rows);h.document.emit('click',h.mark(0));h.render(Array.from({length:10000},(_,i)=>reply(i+2,20,'2026-10-26T16:00:01Z')));
 assert.match(h.detail(),/Sample1/);assert.match(h.detail(),/LOCAL 2026-10-26T16:00:00Z/);assert.match(h.document.querySelector('.mock-selection-context').textContent,/retained as a snapshot/);assert.equal(h.api.isHeld('job'),true);
});

test('linear axis follows the selected time scope and data panning rather than a DOM scrollbar',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i)));h.api.setPreferences({mode:'linear'},{persist:false});h.context.NetworkToolkitGraphHistory.setPreferences({durationMs:10000},{persist:false});h.render(rows);
 const times=()=>h.document.querySelector('.rt-time-axis').querySelectorAll('time'),ends=()=>[times()[0].getAttribute('datetime'),times().at(-1).getAttribute('datetime')];assert.equal(Date.parse(ends()[1])-Date.parse(ends()[0]),10000);assert.equal(Date.parse(ends()[1]),Date.parse(rows.at(-1).time));h.api.pan('job',-.5);assert.equal(Date.parse(ends()[1]),Date.parse(rows.at(-1).time)-5000);
});

test('time zoom changes range, hit testing and axis accurately and resets without changing recorded rows',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i))),original=JSON.stringify(rows);h.render(rows);assert.equal(h.document.querySelectorAll('[data-rt-point]').length,400);h.api.zoom('job','in');assert.equal(h.document.querySelectorAll('[data-rt-point]').length,200);
 let surface=h.document.querySelector('[data-rt-surface]');surface.getBoundingClientRect=()=>({left:10,width:820});h.document.emit('pointermove',surface,{clientX:74});assert.match(h.document.querySelector('[data-rt-tooltip]').textContent,/Sample 201/);h.api.zoom('job','out');assert.equal(h.document.querySelectorAll('[data-rt-point]').length,400);h.api.zoom('job','out');assert.equal(h.document.querySelectorAll('[data-rt-point]').length,400);h.api.zoom('job','reset');assert.equal(h.document.querySelectorAll('[data-rt-point]').length,400);assert.equal(JSON.stringify(rows),original);
});

test('linear zoom uses time domains at bounded scales without a wide scrolling SVG',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i)));h.api.setPreferences({mode:'linear'},{persist:false});h.render(rows);h.api.zoom('job',2);assert.equal(h.document.querySelectorAll('[data-rt-point]').length,200);assert.equal(h.api.zoom('job',999),16);assert(h.document.querySelectorAll('[data-rt-point]').length<30);assert.equal(h.document.querySelector('[data-rt-scroll]'),null);assert.equal(h.api.zoom('job','reset'),1);assert.equal(h.document.querySelectorAll('[data-rt-point]').length,400);
});

test('outside controls preserve a held pin and filters until explicit deselection',()=>{
 const h=harness();let filter={active:true,statistic:'maximum'},clears=0;h.context.NetworkToolkitResultFilters={current:()=>filter,graphMatches:(_,raw)=>raw.sequence===4,clearStatistic:()=>clears++};h.render();
 h.document.emit('click',h.mark(1));h.document.emit('pointermove',h.mark(1));h.document.emit('pointerout',h.mark(1),{relatedTarget:null});assert(h.mark(1).classList.contains('rt-point-selected'));
 const statistic=h.document.createElement('button');statistic.dataset.statistic='loss';h.document.append(statistic);h.document.emit('click',statistic);
 assert.equal(h.api.isHeld('job'),true);assert(h.mark(1).classList.contains('rt-point-selected'));assert.equal(clears,0);assert.equal(filter.statistic,'maximum');
 h.document.emit('click',h.mark(2));h.api.zoom('job','in');assert.match(h.detail(),/Sample3/,'Graph controls keep selected measurements');h.document.emit('click',h.document);assert.equal(h.api.isHeld('job'),true);h.api.clearSelection('job');assert.equal(h.api.isHeld('job'),false);
});

test('Follow latest respects matching samples in both modes without resetting zoom or preferences',()=>{
 for(const mode of ['linear','condensed']){const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i)));h.context.NetworkToolkitResultFilters={current:()=>({active:true}),graphMatches:(_,row)=>row.sequence<=350};h.api.setPreferences({mode},{persist:false});h.render(rows);h.document.emit('click',h.mark(0));h.api.zoom('job',2);h.api.jumpLatest('job');assert.equal(h.document.querySelector('.mock-entry-details'),null);assert.equal(h.document.querySelector('.rt-point-selected')?.dataset.rtSequence,'350');assert.equal(h.api.tracking('job').following,true);assert.equal(h.api.isHeld('job'),false);assert.equal(h.api.preferences().mode,mode);assert.equal(h.document.querySelector('[data-rt-zoom-level]').dataset.rtZoomFactor,'2');}
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
 const surface=h.document.querySelector('[data-rt-surface="latency"]');h.document.emit('keydown',surface,{key:'End'});assert.deepEqual(calls.at(-1),['job',true]);h.document.emit('focusout',surface,{relatedTarget:h.document.querySelector('[data-rt-mode]')});assert.deepEqual(calls.at(-1),['job',false]);
 const row=h.document.createElement('tr');row.dataset.resultGraphId='job';row.dataset.resultSampleKey=h.api.sampleKey(samples()[2],2);h.document.append(row);h.document.emit('pointermove',row);assert.deepEqual(calls.at(-1),['job',true]);h.document.emit('pointerout',row,{relatedTarget:null});assert.deepEqual(calls.at(-1),['job',false]);
});

test('sequence gaps never invent jitter pairs or connect across unrecorded probes',()=>{
 const rows=[reply(1,10),reply(3,30),reply(4,25),{...reply(5,50),success:false,status:'Lost'},reply(6,15),reply(7,20)];
 assert.deepEqual(timeline.normalize(rows).map(point=>point.jitter),[null,null,5,null,null,5]);assert.equal(timeline.summary(timeline.normalize(rows)).jitterPairs,2);
 const h=harness();h.render(rows);const lines=h.document.querySelector('[data-rt-surface="latency"]').querySelectorAll('polyline');assert.equal(lines.length,2,'Latency connects only the observed consecutive pairs, with failures breaking the line');
 const legacy=[{rttMs:2,success:true},{rttMs:4,success:true}];assert.equal(timeline.normalize(legacy)[1].jitter,2,'Legacy records without recorded sequence remain usable');
});

test('zoom and panning retain an older entry, and Follow latest releases it while preserving zoom',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i)));h.render(rows);h.document.emit('click',h.mark(240));h.api.zoom('job',4);assert.match(h.detail(),/Sample241/);h.api.pan('job',-.5);assert.match(h.detail(),/Sample241/);h.api.jumpLatest('job');assert.equal(h.document.querySelector('.mock-entry-details'),null);assert.equal(h.document.querySelector('.rt-point-selected')?.dataset.rtSequence,'400');assert.equal(h.api.isHeld('job'),false);assert.equal(h.document.querySelector('[data-rt-zoom-level]').dataset.rtZoomFactor,'4');
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
 for(const mode of ['linear','condensed']){
  const h=harness();h.context.NetworkToolkitResultFilters={current:()=>({active:true,statistic:'average',reference:18.5,referenceKind:'run'}),graphMatches:()=>true};h.api.setPreferences({mode},{persist:false});h.render();h.document.emit('click',h.mark(0));h.observe();
  const selectors=['[data-graph-time-open]','[data-rt-mode]'],controls=selectors.map(selector=>h.document.querySelector(selector));
  let serializedReads=0;for(const node of [h.document.querySelector('.mock-point-details'),h.document.querySelector('.mock-selection-context'),...h.document.querySelectorAll('[data-rt-surface]')]){const descriptor=Object.getOwnPropertyDescriptor(Object.getPrototypeOf(node),'innerHTML');Object.defineProperty(node,'innerHTML',{get(){serializedReads++;return '<browser-canonical-serialization data-empty=""></browser-canonical-serialization>';},set(value){descriptor.set.call(this,value);}});}
  for(let pass=0;pass<32;pass++)h.observe();
  assert.equal(serializedReads,0,'Restore compares prepared semantic content rather than browser-normalized HTML');
  selectors.forEach((selector,index)=>{assert.equal(h.document.querySelector(selector),controls[index],mode+' keeps the exact '+selector+' element attached');assert.equal(controls[index].isConnected,true);});
 }
});

test('aggregate filter explanations remain escaped and accessible beside the graph',()=>{
 const h=harness();let body='';h.context.openModal=(_,html)=>{body=html;};h.context.NetworkToolkitResultFilters={current:()=>({active:true,statistic:'average',reference:18.5,referenceKind:'run',message:'Average is an aggregate <img src=x onerror="unsafe()">'}),graphMatches:()=>true};h.render();body=h.document.querySelector('.rt-filter-note').innerHTML;assert.match(body,/Average is an aggregate &lt;img/);assert.doesNotMatch(body,/<img/);assert.match(body,/Run average RTT reference: 18.5 ms/);assert(h.document.querySelector('[data-rt-stat-reference]'));
});

test('switching plots preserves the panned viewport and cannot silently resume following',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10+i%9,stamp(i)));h.api.setPreferences({mode:'linear'},{persist:false});h.render(rows);h.api.pan('job',-.5);const axis=h.document.querySelector('.rt-time-axis').innerHTML;h.api.setPlot('job','jitter');assert.equal(h.document.querySelector('.rt-time-axis').innerHTML,axis);assert.equal(h.api.tracking('job').following,false);assert.equal(h.document.querySelector('[data-rt-surface="latency"]'),null);
});

test('data-pan navigation survives a source render and releases only on Follow latest',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10+i%9,stamp(i)));h.api.setPreferences({mode:'linear'},{persist:false});h.render(rows);h.api.zoom('job','in');h.api.pan('job',-.5);const axis=h.document.querySelector('.rt-time-axis').innerHTML;h.render([...rows,reply(401,10,stamp(400))]);assert.equal(h.document.querySelector('.rt-time-axis').innerHTML,axis);h.api.jumpLatest('job');assert.equal(h.api.tracking('job').following,true);assert.notEqual(h.document.querySelector('.rt-time-axis').innerHTML,axis);
});

test('condensed history renders aggregates with extrema and keeps recent results individual without changing raw samples',()=>{
 const h=harness(),rows=Array.from({length:7201},(_,i)=>reply(i+1,i===1?1000:i%19,new Date(Date.parse('2026-10-26T12:00:00Z')+i*1000).toISOString())),before=JSON.stringify(rows);h.api.setPreferences({mode:'condensed'},{persist:false});h.render(rows);const marks=h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]');assert(marks.length<1600);const group=h.document.querySelector('[data-rt-surface="latency"] [data-rt-aggregate]');assert(group);assert(h.document.querySelector('.rt-aggregate-range'));h.document.emit('click',group);assert.match(h.document.querySelector('.mock-point-details').textContent,/Grouped samples/);assert.equal(h.api.isHeld('job'),true);h.api.selectResult('job',h.api.sampleKey(rows[1]));assert.match(h.detail(),/1,000 ms/);h.document.emit('click',h.document);assert.equal(h.api.isHeld('job'),true);h.api.jumpLatest('job');assert.equal(h.api.isHeld('job'),false);assert.equal(h.document.querySelector('.mock-entry-details'),null);assert.equal(h.document.querySelector('.rt-point-selected')?.dataset.rtSequence,'7201');assert.equal(JSON.stringify(rows),before);assert.equal(h.document.querySelector('.rt-detail'),null);
});
test('fixed time navigation stays anchored during live updates while Latest restores duration-preserving following',()=>{
 const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,new Date(Date.parse('2026-10-26T12:00:00Z')+i*1000).toISOString()));h.context.NetworkToolkitGraphHistory.setPreferences({anchor:'end',end:rows[199].time,durationMs:60000},{persist:false});h.render(rows);assert.equal(h.document.querySelectorAll('[data-rt-surface="latency"] [data-rt-point]').length,61);let axis=h.document.querySelector('.rt-time-axis').querySelectorAll('time');assert.equal(Date.parse(axis.at(-1).getAttribute('datetime')),Date.parse(rows[199].time));h.api.selectResult('job',h.api.sampleKey(rows[180]));h.render([...rows,reply(401,20,'2026-10-26T13:00:00Z')]);assert.match(h.detail(),/Sample181/);h.api.jumpLatest('job');assert.equal(h.context.NetworkToolkitGraphHistory.preferences().durationMs,60000);assert.equal(h.context.NetworkToolkitGraphHistory.preferences().anchor,'latest');assert.equal(h.document.querySelector('.mock-entry-details'),null);assert.equal(h.document.querySelector('.rt-point-selected')?.dataset.rtSequence,'401');
});

test('chosen time range never includes an RTT change whose previous source sample is outside it',()=>{
 const h=harness(),rows=Array.from({length:5},(_,i)=>reply(i+1,[1,5,100,104,108][i],'2026-10-26T16:00:0'+i+'Z'));h.context.NetworkToolkitGraphHistory.setPreferences({anchor:'range',start:rows[2].time,end:rows[4].time},{persist:false});h.render(rows);h.api.setPlot('job','jitter');assert(h.document.querySelector('[data-rt-surface="jitter"]'));h.api.setPlot('job','rtt');h.document.emit('click',h.mark(0));assert.match(h.detail(),/Response time100 ms/);h.document.emit('click',h.mark(1));assert.match(h.detail(),/RTT change4 ms/);
});

test('a held aggregate can open an original contributor after it leaves retained graph history',()=>{
 const h=harness(),rows=Array.from({length:1500},(_,i)=>reply(i+1,5+i%30,new Date(Date.parse('2026-10-26T12:00:00Z')+i*1000).toISOString()));h.api.setPreferences({mode:'condensed'},{persist:false});h.render(rows);const first=h.document.querySelector('[data-rt-surface="latency"] [data-rt-aggregate]');assert(first);h.document.emit('click',first);assert.match(h.detail(),/Grouped samples/);h.render(rows.map(row=>({...row,sequence:row.sequence+1000,time:new Date(Date.parse(row.time)+1000000).toISOString()})));assert.equal(h.api.selectResult('job',h.api.sampleKey(rows[0],0)),true);assert.match(h.detail(),/Sample1/);assert.match(h.document.querySelector('.mock-selection-context').textContent,/snapshot/);
});

test('endpoint gutters contain singleton, paired and selected markers across modes, ranges, zoom and resize',()=>{
 for(const mode of ['linear','condensed'])for(const count of [1,2,5]){
  const h=harness(),rows=Array.from({length:count},(_,i)=>reply(i+1,10+i,stamp(i)));h.api.setPreferences({mode},{persist:false});h.render(rows);
  assert.equal(h.document.querySelector('[data-rt-options]'),null,'No redundant options menu');
  assert(h.document.querySelector('[data-graph-time-open]'),'Range is a direct control');
  for(const size of [180,320,820])for(const zoom of [1,2,.5]){
   h.api.zoom('job',zoom);const svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width:size,height:80});h.api.restore();
   for(const marker of svg.querySelectorAll('[data-rt-marker]')){const x=Number(marker.getAttribute('cx')),radius=Number(marker.getAttribute('r'))+2;assert(x-radius>=0,mode+' left highlighted endpoint');assert(x+radius<=size,mode+' right highlighted endpoint');}
   assert.equal(h.document.querySelector('.rt-time-axis').style.marginRight,'12px','Axis shares the plot gutter');
  }
  const htime=h.context.NetworkToolkitGraphHistory;htime.setPreferences({anchor:'range',start:stamp(1),end:stamp(3)});h.render(rows);
  for(const marker of h.document.querySelectorAll('[data-rt-marker]')){const x=Number(marker.getAttribute('cx')),radius=Number(marker.getAttribute('r'))+2;assert(x-radius>=0&&x+radius<=820);}
 }
 const h=harness();assert.equal(h.api.latency([],{id:'empty'}),'','Empty history does not invent endpoints');
});

test('legacy graph modes migrate to Linear while retaining range, held entries and exactly two choices',()=>{
 for(const mode of ['window','progressive','linear']){
  const h=harness(),rows=Array.from({length:400},(_,i)=>reply(i+1,10,stamp(i)));h.context.state={};h.api.mergePreferences({graphPreferences:{mode}});h.render(rows);
  assert.equal(h.document.querySelectorAll('[data-rt-marker]').length,400);assert.equal(h.api.preferences().mode,'linear');assert.equal(h.context.state.graphPreferences.mode,'linear');
  const select=h.document.querySelector('[data-rt-mode]');assert.deepEqual(select.querySelectorAll('option').map(option=>option.getAttribute('value')),['linear','condensed']);assert.match(select.textContent,/Linear/);assert.doesNotMatch(select.textContent,/180|Full history|Progressive|Window/i);
  h.document.emit('click',h.mark(100));h.context.NetworkToolkitGraphHistory.setPreferences({anchor:'range',start:rows[200].time,end:rows[300].time},{persist:false});const range=JSON.stringify(h.context.NetworkToolkitGraphHistory.preferences());h.api.mergePreferences({graphPreferences:{mode}});h.render(rows);
  assert.equal(JSON.stringify(h.context.NetworkToolkitGraphHistory.preferences()),range);assert.match(h.detail(),/Sample101/);assert.equal(h.api.isHeld('job'),true);assert.equal(h.document.querySelectorAll('[data-rt-marker]').length,101);assert.equal(rows.length,400);
 }
 const h=harness();h.api.mergePreferences({graphPreferences:{mode:'condensed'}});assert.equal(h.api.preferences().mode,'condensed');
});


test('Condensed Fit keeps both retained edges inside the plot across raw-to-aggregate growth and retention rollover',()=>{
 const h=harness(),rows=Array.from({length:10001},(_,i)=>reply(i+1,i%17,stamp(i))),original=JSON.stringify(rows);h.api.setPreferences({mode:'condensed'},{persist:false});
 for(const count of [1400,1401,10001]){
  h.render(rows.slice(0,count));h.api.zoom('job','reset');
  if(count===1400)h.api.selectResult('job',h.api.sampleKey(rows[699]));
  const container=h.document.querySelector('.rt-timeline'),svg=h.document.querySelector('[data-rt-surface]'),markers=svg.querySelectorAll('[data-rt-marker]'),first=Math.max(0,count-10000),history=h.context.NetworkToolkitGraphHistory;
  assert(history.instant(container.dataset.rtScopeStart)>=history.instant(rows[first].time),'Compressed axis names the history-cluster boundary, not its oldest member');assert.equal(history.instant(container.dataset.rtScopeEnd),history.instant(rows[count-1].time));
  assert.equal(Number(markers[0].getAttribute('cx')),Number(svg.dataset.rtLeft),'Oldest raw or aggregate point stays at the retained left edge');assert.equal(Number(markers.at(-1).getAttribute('cx')),808,'Newest point keeps the12px right gutter');
  for(const marker of [markers[0],markers.at(-1)]){const x=Number(marker.getAttribute('cx')),radius=Number(marker.getAttribute('r'))+2;assert(x-radius>=0&&x+radius<=820,'Endpoints include marker and highlight padding');}
  assert.equal(markers.length,count-first-clusterSize(h,rows.slice(0,count))+1,'One cluster plus every original newer point');assert.equal(svg.querySelectorAll('[data-rt-aggregate]').length,1);assert.match(h.detail(),/Sample700/);assert.equal(h.api.isHeld('job'),true);
 }
 assert.equal(JSON.stringify(rows),original,'Fit, grouping and retention display never rewrite source rows');
});

test('Condensed recent markers keep their pixels as older history grows, across grouping, resize, filters and a held selection',()=>{
 const h=harness(),rows=Array.from({length:10000},(_,i)=>reply(i+1,42,stamp(i))),before=JSON.stringify(rows),references=new Map();h.api.setPreferences({mode:'condensed'},{persist:false});
 for(const count of [706,1400,1401,7201]){
  h.render(rows.slice(-count));h.api.zoom('job','reset');if(count===706)h.api.selectResult('job',h.api.sampleKey(rows[9990]));
  for(const width of [820,320]){
   const svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width,height:100});h.api.restore();const markers=svg.querySelectorAll('[data-rt-marker]'),recent=Array.from({length:5},(_,i)=>Number(svg.querySelector('[data-rt-sequence="'+(9996+i)+'"] [data-rt-marker]').getAttribute('cx')));
   assert.equal(Number(markers[0].getAttribute('cx')),Number(svg.dataset.rtLeft));assert.equal(Number(markers.at(-1).getAttribute('cx')),width-12);
   if(references.has(width))recent.forEach((x,i)=>assert(Math.abs(x-references.get(width)[i])<=.01,'More old history cannot move recent pixels'));else references.set(width,recent);
   for(const marker of [markers[0],markers.at(-1)]){const x=Number(marker.getAttribute('cx')),radius=Number(marker.getAttribute('r'))+2;assert(x-radius>=0&&x+radius<=width);}
  }
  assert.match(h.detail(),/Sample9991/);assert.equal(h.api.isHeld('job'),true);assert(h.context.NetworkToolkitGraphHistory.instant(h.document.querySelector('.rt-timeline').dataset.rtScopeStart)>=h.context.NetworkToolkitGraphHistory.instant(rows.at(-count).time));
 }
 h.context.NetworkToolkitResultFilters={current:()=>({active:true}),graphMatches:(_,row)=>row.sequence%2===0};h.document.emit('networkToolkitResultFilterChange',h.document,{detail:{jobId:'job'}});assert.match(h.detail(),/Sample9991/);assert.equal(Number(h.document.querySelector('[data-rt-marker]').getAttribute('cx')),Number(h.document.querySelector('[data-rt-surface]').dataset.rtLeft));assert.equal(JSON.stringify(rows),before);
 const history=h.context.NetworkToolkitGraphHistory,container=h.document.querySelector('.rt-timeline'),scope={start:container.dataset.rtScopeStart,end:container.dataset.rtScopeEnd},svg=h.document.querySelector('[data-rt-surface]'),left=Number(svg.dataset.rtLeft),plot=320-12-left,from=left+plot*.8,to=left+plot*.9,anchor=history.instant(history.at(.8,scope,'condensed'));svg.getBoundingClientRect=()=>({left:0,top:0,width:320,height:100});
 h.document.emit('pointerdown',svg,{button:0,pointerId:9,clientX:from});h.document.emit('pointerup',svg,{pointerId:9,clientX:to});const moved=h.document.querySelector('.rt-timeline');assert(history.instant(moved.dataset.rtScopeEnd)<history.instant(scope.end),'Dragging toward the right requests older recorded samples');assert.equal(Number(h.document.querySelector('[data-rt-marker]').getAttribute('cx')),Number(h.document.querySelector('[data-rt-surface]').dataset.rtLeft),'Historical range snaps actual oldest displayed point to the boundary');assert.match(h.detail(),/Sample9991/);
});

test('Condensed groups retain duplicate timestamps, gaps, zero, spikes and failures with truthful original details',()=>{
 const h=harness(),rows=Array.from({length:2001},(_,i)=>({...reply(i+1,i===0?0:i===1?9999:42,stamp(i<4?0:i+(i>=1000?600:0))),...(i===2?{success:false,status:'Timed out',rttMs:null}:{})})),before=JSON.stringify(rows);h.api.setPreferences({mode:'condensed'},{persist:false});h.render(rows);const svg=h.document.querySelector('[data-rt-surface]');assert.equal(Number(svg.dataset.rtMinimum),0);assert.equal(Number(svg.dataset.rtMaximum),9999);assert(svg.querySelector('.rt-aggregate-range'));
 const history=h.context.NetworkToolkitGraphHistory,items=h.api.normalize(rows).filter(point=>history.instant(point.time)<=history.instant(rows.at(-1).time)-BigInt(Math.round(Number(svg.dataset.rtHorizonMs)*1000000))),oldest={items,start:items[0].time,end:items.at(-1).time},stats=h.api.summary(items);assert.equal(svg.querySelectorAll('[data-rt-aggregate]').length,1);assert(oldest.items.some(point=>point.sequence===1)&&oldest.items.some(point=>point.sequence===2)&&oldest.items.some(point=>point.sequence===3));h.document.emit('click',svg.querySelector('[data-rt-aggregate]'));
 const detail=h.detail();for(const value of ['Grouped samples'+stats.count,'Unsuccessful'+stats.lost,'Minimum RTT0 ms','Maximum RTT9,999 ms','LOCAL '+oldest.start,'LOCAL '+oldest.end])assert(detail.includes(value),value);
 h.api.selectResult('job',h.api.sampleKey(rows[2]));assert.match(h.detail(),/Sample3/);assert.match(h.detail(),/Timed out/);h.api.zoom('job',2);h.api.setPlot('job','jitter');assert.match(h.detail(),/Sample3/);assert.equal(JSON.stringify(rows),before,'Grouping and navigation preserve every original result');
});


test('single history cluster preserves a million-sample prefix across raw rollover without retaining its raw records',()=>{
 const h=harness(),prefix={count:1000000,received:900000,lost:100000,measured:900000,minMs:0,maxMs:9999,sumMs:9000000,zeroCount:1,firstTime:stamp(-1000000),lastTime:stamp(-1),firstSequence:1,lastSequence:1000000,jitterSumMs:22,jitterPairs:2,jitterMinMs:2,jitterMaxMs:20,lastSuccess:true,lastRttMs:10},rows=Array.from({length:10000},(_,i)=>reply(1000001+i,10,stamp(i))),before=JSON.stringify(prefix);h.api.setPreferences({mode:'condensed'},{persist:false});h.render(rows,'job',{history:prefix});
 const svg=h.document.querySelector('[data-rt-surface]'),cluster=svg.querySelector('[data-rt-aggregate]');assert.equal(svg.querySelectorAll('[data-rt-aggregate]').length,1);const expected=clusterSize(h,rows,1000000);assert.equal(Number(cluster.dataset.rtAggregate),expected);assert.equal(svg.querySelectorAll('[data-rt-marker]').length,10000-(expected-1000000)+1);assert.equal(Number(cluster.querySelector('[data-rt-marker]').getAttribute('cx')),Number(svg.dataset.rtLeft));assert.equal(Number(svg.dataset.rtMinimum),0);assert.equal(Number(svg.dataset.rtMaximum),9999);h.document.emit('click',cluster);assert.match(h.detail(),/1000000 summarized only/);assert.match(h.detail(),/Unsuccessful100000/);assert.match(h.detail(),/Minimum RTT0 ms/);assert.match(h.detail(),/Maximum RTT9,999 ms/);assert.equal(JSON.stringify(prefix),before);
 const next={...prefix,count:1000001,received:900001,measured:900001,sumMs:9000010,lastTime:rows[0].time,lastSequence:1000001};h.render([...rows.slice(1),reply(1010001,10,stamp(10000))],'job',{history:next});assert.equal(Number(h.document.querySelector('[data-rt-aggregate]').dataset.rtAggregate),expected,'Held time range does not silently advance');assert.equal(h.api.isHeld('job'),true,'A held aggregate remains a truthful snapshot');h.api.jumpLatest('job');assert.equal(Number(h.document.querySelector('[data-rt-aggregate]').dataset.rtAggregate),expected+1);
 h.api.setPreferences({mode:'linear'},{persist:false});h.render([...rows.slice(1),reply(1010001,10,stamp(10000))],'job',{history:next});assert.equal(h.document.querySelectorAll('[data-rt-aggregate]').length,0);assert.equal(h.document.querySelectorAll('[data-rt-marker]').length,10000,'Linear remains individual retained data');
 h.api.setPreferences({mode:'condensed'},{persist:false});h.render([reply(1,5,stamp(0))],'new-run');assert.equal(h.document.querySelectorAll('[data-rt-aggregate]').length,0);assert.equal(Number(h.document.querySelector('[data-rt-marker]').getAttribute('cx')),808,'One point anchors latest; no synthetic older cluster');
});
test('sparse startup and changing Y labels never rescale recent X, and partial archived ranges never invent statistics',()=>{
 const h=harness();h.api.setPreferences({mode:'condensed'},{persist:false});const points=[];let gap;
 for(const count of [2,10,60,64,65,66,179,180,181,300]){const rows=Array.from({length:count},(_,i)=>reply(i+1,count%2?1e10:.000002,stamp(i)));h.render(rows);const svg=h.document.querySelector('[data-rt-surface]'),markers=svg.querySelectorAll('[data-rt-marker]'),last=Number(markers.at(-1).getAttribute('cx')),current=last-Number(markers.at(-2).getAttribute('cx'));if(gap!=null)assert(Math.abs(current-gap)<.01);gap=current;assert.equal(Number(svg.dataset.rtLeft),64);assert.equal(last,808);assert.equal(svg.querySelectorAll('[data-rt-aggregate]').length,clusterSize(h,rows)>0?1:0);points.push(current);}
 const prefix={count:500,received:500,measured:500,sumMs:5000,minMs:1,maxMs:9999,firstTime:stamp(-500),lastTime:stamp(-1)},rows=Array.from({length:301},(_,i)=>reply(i+1,10,stamp(i)));h.context.NetworkToolkitGraphHistory.setPreferences({anchor:'range',start:stamp(-250),end:stamp(300)},{persist:false});h.render(rows,'job',{history:prefix});const svg=h.document.querySelector('[data-rt-surface]');assert.equal(Number(svg.dataset.rtMaximum),10,'Partial prefix extrema are unknown, not fabricated');assert.match(h.document.querySelector('.rt-plot-caption').textContent,/Partial archived interval unavailable/);
});

test('legacy omitted history and archived filter matches are explicitly unavailable, not invented',()=>{
 const h=harness(),rows=Array.from({length:301},(_,i)=>reply(i+701,10,stamp(i)));h.api.setPreferences({mode:'condensed'},{persist:false});h.render(rows,'job',{job:{result:{data:{samples:rows,statistics:{sent:1001}}}}});assert.match(h.document.querySelector('.rt-plot-caption').textContent,/700 earlier results have no recoverable summary/);assert.equal(Number(h.document.querySelector('[data-rt-aggregate]').dataset.rtAggregate),clusterSize(h,rows));
 const prefix={count:700,received:700,measured:700,sumMs:7000,minMs:10,maxMs:10,firstTime:stamp(-700),lastTime:stamp(-1)};h.context.NetworkToolkitResultFilters={current:()=>({active:true}),graphMatches:(_,row)=>row.sequence===1001};h.render(rows,'job',{history:prefix});h.document.emit('click',h.document.querySelector('[data-rt-aggregate]'));assert.match(h.document.querySelector('.mock-selection-context').textContent,/exact matches among discarded records are unavailable/);assert(h.detail().includes('Grouped samples'+clusterSize(h,rows,700)));
});

test('duplicate timestamps across prefix rollover preserve counts using distinct sequence identities',()=>{
 const h=harness(),rows=Array.from({length:301},(_,i)=>reply(i+3,10,stamp(i))),prefix={count:2,received:1,lost:1,measured:1,sumMs:0,minMs:0,maxMs:0,firstTime:stamp(-1),lastTime:rows[0].time,firstSequence:1,lastSequence:2};h.api.setPreferences({mode:'condensed'},{persist:false});h.render(rows,'job',{history:prefix});const cluster=h.document.querySelector('[data-rt-aggregate]');assert.equal(Number(cluster.dataset.rtAggregate),clusterSize(h,rows,2));h.document.emit('click',cluster);assert.match(h.detail(),/2 summarized only/);assert.match(h.detail(),/Minimum RTT0 ms/);
});

test('Fit includes whole archive prefix before a long horizon and pairs only the actual live or completed sample source',()=>{
 const h=harness(),rows=Array.from({length:2200},(_,i)=>reply(i+1000001,10,stamp(i))),history={count:1000000,received:1000000,measured:1000000,minMs:5,maxMs:50,sumMs:10000000,firstTime:stamp(-1000000),lastTime:stamp(-1),lastSequence:1000000};h.api.setPreferences({mode:'condensed'},{persist:false});h.context.NetworkToolkitGraphHistory.setPreferences({recentMs:86400000},{persist:false});h.render(rows,'live',{job:{sampleHistory:history}});const cluster=h.document.querySelector('[data-rt-aggregate]');assert.equal(Number(cluster.dataset.rtAggregate),1000000);assert.equal(h.document.querySelectorAll('[data-rt-point]').length,2201);h.document.emit('click',cluster);assert.match(h.detail(),/summarized before the display horizon/);assert.match(h.detail(),/1000000 summarized only/);
 h.render(rows,'complete',{job:{sampleHistory:{...history,count:99},result:{data:{samples:rows,history}}}});assert.equal(Number(h.document.querySelector('[data-rt-aggregate]').dataset.rtAggregate),1000000,'Completed raw rows pair only with completed prefix');
});

test('rendered Condensed newest gap matches a 10-second Linear view at normal and narrow widths',()=>{
 for(const width of [820,320]){const h=harness(),rows=Array.from({length:301},(_,i)=>reply(i+1,42,stamp(i)));h.api.setPreferences({mode:'linear'},{persist:false});h.context.NetworkToolkitGraphHistory.setPreferences({durationMs:10000},{persist:false});h.render(rows);{const svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width,height:100});h.api.restore();}const gap=()=>{const markers=h.document.querySelector('[data-rt-surface]').querySelectorAll('[data-rt-marker]');return Number(markers.at(-1).getAttribute('cx'))-Number(markers.at(-2).getAttribute('cx'));},reference=gap();h.api.setPreferences({mode:'condensed'},{persist:false});h.context.NetworkToolkitGraphHistory.setPreferences({durationMs:0,recentMs:60000},{persist:false});h.render(rows);{const svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width,height:100});h.api.restore();}const actual=gap();assert(Math.abs(actual/reference-1)<.03);assert(actual>reference*.97);}
});

test('highlight-only restoration keeps the same SVG marker nodes instead of rebuilding under the pointer',()=>{
 const h=harness();h.render();const svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width:820,height:110,right:820,bottom:110});h.api.restore();const mark=h.mark(1);
 h.document.emit('pointermove',mark);h.observe();assert.equal(h.mark(1)===mark,true,'Hover must not replace the hit target');
 h.document.emit('click',mark);h.observe();assert.equal(h.mark(1)===mark,true,'Selection must not replace the hit target');
 for(let i=0;i<5;i++)h.observe();assert.equal(h.mark(1)===mark,true,'Unchanged completed graph stays stable through repeated observer passes');
});

test('vertical alignment selects nearest timestamp independent of measured Y, only inside the plot',()=>{
 const h=harness();h.render(Array.from({length:21},(_,i)=>reply(i+1,i%2?0:100)));const svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width:820,height:110,right:820,bottom:110});h.api.restore();const x=Number(h.mark(10).querySelector('[data-rt-marker]').getAttribute('cx'));
 h.document.emit('pointermove',svg,{clientX:x,clientY:98});assert.match(h.detail(),/Sample 11/,'X-aligned high point wins over lower neighboring points');
 h.document.emit('click',svg,{button:0,clientX:x,clientY:98});assert.equal(h.api.isHeld('job'),true);assert.match(h.detail(),/Sample11/);
 h.api.clearSelection('job');h.document.emit('pointermove',h.document.querySelector('[data-rt-surface]'),{clientX:3,clientY:40});assert.equal(h.document.querySelector('[data-rt-tooltip]').hidden,true,'Y-axis gutter is not an implied point');
});

test('a press snapshots the highlighted sample while fresh samples move under the same coordinate',()=>{
 const h=harness();h.render();let svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width:820,height:110,right:820,bottom:110});h.api.restore();const x=Number(h.mark(1).querySelector('[data-rt-marker]').getAttribute('cx'));
 h.document.emit('pointermove',svg,{clientX:x,clientY:95});h.document.emit('pointerdown',svg,{button:0,pointerId:8,clientX:x,clientY:95});
 h.render([...samples(),reply(6,20),reply(7,20)]);svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width:820,height:110,right:820,bottom:110});h.api.restore();
 h.document.emit('pointerup',svg,{pointerId:8,clientX:x,clientY:95});h.document.emit('click',svg,{button:0,clientX:x,clientY:95});assert.match(h.detail(),/Sample2/,'Held selection uses press-time identity, not the next sample at this x');
});

test('axis gutters, modified clicks, cancelled presses and cross-job releases cannot select an unrelated result',()=>{
 const h=harness();h.render();let svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width:820,height:110,right:820,bottom:110});h.api.restore();
 for(const modifiers of [{ctrlKey:true},{metaKey:true},{altKey:true},{shiftKey:true}]){h.document.emit('pointerdown',svg,{button:0,pointerId:5,clientX:400,clientY:60,...modifiers});h.document.emit('pointerup',svg,{pointerId:5,clientX:400,clientY:60,...modifiers});h.document.emit('click',svg,{button:0,clientX:400,clientY:60,...modifiers});assert.equal(h.api.isHeld('job'),false);}
 h.document.emit('pointerdown',svg,{button:0,pointerId:5,clientX:400,clientY:60});h.document.emit('pointercancel',svg,{pointerId:5});h.document.emit('pointerup',svg,{pointerId:5,clientX:400,clientY:60});assert.equal(h.api.isHeld('job'),false);
 h.document.emit('pointerdown',svg,{button:0,pointerId:5,clientX:400,clientY:60});h.render(samples(),'different-job');svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width:820,height:110,right:820,bottom:110});h.api.restore();h.document.emit('pointerup',svg,{pointerId:5,clientX:400,clientY:60});h.document.emit('click',svg,{button:0,clientX:400,clientY:60});assert.equal(h.api.isHeld('different-job'),false);
 h.document.emit('click',svg,{button:0,clientX:2,clientY:60});assert.equal(h.api.isHeld('different-job'),false);
});

test('a vertical drag is not a selection and a real horizontal drag still pans',()=>{
 const h=harness();h.render();let svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width:820,height:110,right:820,bottom:110});h.api.restore();const end=h.document.querySelector('.rt-timeline').dataset.rtScopeEnd;
 h.document.emit('pointerdown',svg,{button:0,pointerId:9,clientX:400,clientY:40});h.document.emit('pointerup',svg,{pointerId:9,clientX:400,clientY:70});h.document.emit('click',svg,{button:0,clientX:400,clientY:70});assert.equal(h.api.isHeld('job'),false);assert.equal(h.document.querySelector('.rt-timeline').dataset.rtScopeEnd,end);
 h.document.emit('pointerdown',svg,{button:0,pointerId:9,clientX:400,clientY:40});h.document.emit('pointerup',svg,{pointerId:9,clientX:450,clientY:40});h.document.emit('click',h.document.querySelector('[data-rt-surface]'),{button:0,clientX:450,clientY:40});assert.equal(h.api.isHeld('job'),false);assert.notEqual(h.document.querySelector('.rt-timeline').dataset.rtScopeEnd,end);
});

test('overlapping transparent hit circles resolve the visible X sample, not DOM stacking order',()=>{
 for(const width of [820,320]){
  const h=harness(),rows=Array.from({length:402},(_,i)=>reply(i+1,10+i%9));h.api.setPreferences({mode:'linear'},{persist:false});h.render(rows);const svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width,height:110,right:width,bottom:110});h.api.restore();
  const marker=h.mark(397).querySelector('[data-rt-marker]'),x=Number(marker.getAttribute('cx')),y=Number(marker.getAttribute('cy')),cover=h.mark(400).querySelector('.rt-point-hit');assert(Math.abs(x-Number(cover.getAttribute('cx')))<9,'A later transparent hit circle covers the intended visible sample');
  h.document.emit('pointermove',cover,{clientX:x,clientY:y});assert.match(h.detail(),/Sample 398/,'Hover follows nearest timestamp despite the later hit target');
  h.document.emit('pointerdown',cover,{button:0,pointerId:1,clientX:x,clientY:y});h.document.emit('pointerup',cover,{pointerId:1,clientX:x,clientY:y});h.document.emit('click',cover,{button:0,clientX:x,clientY:y});assert.match(h.detail(),/Sample398/,'Press/release selects the visible X-aligned identity');
 }
});

test('same-timestamp marker ties retain explicit marker identity and stable off-marker hover',()=>{
 const h=harness();h.render([reply(1,10,stamp(1)),reply(2,100,stamp(1)),reply(3,20,stamp(2))]);const svg=h.document.querySelector('[data-rt-surface]');svg.getBoundingClientRect=()=>({left:0,top:0,width:820,height:110,right:820,bottom:110});h.api.restore();const marker=h.mark(1).querySelector('[data-rt-marker]'),x=Number(marker.getAttribute('cx')),y=Number(marker.getAttribute('cy'));
 h.document.emit('pointermove',h.mark(1).querySelector('.rt-point-hit'),{clientX:x,clientY:y});assert.match(h.detail(),/Sample 2/,'An explicit same-X marker is not replaced by array order');h.document.emit('pointermove',svg,{clientX:x,clientY:98});assert.match(h.detail(),/Sample 2/,'Moving vertically at the same timestamp retains the highlighted identity');
});
test('default linear startup integration leaves honest space then binds both actual endpoints',()=>{const h=harness();for(const count of [2,3,10]){h.render(Array.from({length:count},(_,i)=>reply(i+1,5,stamp(i))));const svg=h.document.querySelector('[data-rt-surface]'),m=svg.querySelectorAll('[data-rt-marker]');assert(Number(m[0].getAttribute('cx'))>Number(svg.dataset.rtLeft));assert.equal(Number(m.at(-1).getAttribute('cx')),Number(svg.dataset.rtRight));}h.render(Array.from({length:91},(_,i)=>reply(i+1,5,stamp(i))));const svg=h.document.querySelector('[data-rt-surface]');assert.equal(Number(svg.querySelector('[data-rt-marker]').getAttribute('cx')),Number(svg.dataset.rtLeft));});
test('all actual endpoint paint is clipped and a genuine time gap is not bridged',()=>{const h=harness(),rows=Array.from({length:12},(_,i)=>reply(i+1,5,stamp(i+(i>=6?300:0))));h.render(rows);const svg=h.document.querySelector('[data-rt-surface]');assert.match(svg.querySelector('.rt-points').getAttribute('clip-path'),/^url\(#/);assert.equal(svg.querySelector('.rt-series').querySelectorAll('polyline').length,2);assert.equal(JSON.stringify(rows),JSON.stringify(Array.from({length:12},(_,i)=>reply(i+1,5,stamp(i+(i>=6?300:0))))));});
test('mature graphs never regain startup gaps on wider time scales or zoom out and growth does not squeeze recent points',()=>{for(const mode of ['linear','condensed']){const h=harness();h.api.setPreferences({mode},{persist:false});h.render(Array.from({length:91},(_,i)=>reply(i+1,5,stamp(i))));h.context.NetworkToolkitGraphHistory.setPreferences({durationMs:3600000},{persist:false});h.api.zoom('job',.25);let gap;for(const count of [91,100,200]){h.render(Array.from({length:count},(_,i)=>reply(i+1,5,stamp(i))));const svg=h.document.querySelector('[data-rt-surface]'),marks=svg.querySelectorAll('[data-rt-marker]');assert.equal(Number(marks[0].getAttribute('cx')),Number(svg.dataset.rtLeft),mode+' mature left edge');assert.equal(Number(marks.at(-1).getAttribute('cx')),Number(svg.dataset.rtRight));if(mode==='condensed'){const current=Number(marks.at(-1).getAttribute('cx'))-Number(marks.at(-2).getAttribute('cx'));if(gap!==undefined)assert(Math.abs(current-gap)<.02,'New history cannot shrink normal recent spacing');gap=current;}}}});


test('navigator preserves both archived bounds when a range contains only the historical cluster',()=>{
 const h=harness(),prefix={count:500,received:500,measured:500,sumMs:5000,minMs:10,maxMs:10,firstTime:stamp(-500),lastTime:stamp(-1)},rows=Array.from({length:31},(_,i)=>reply(i+501,10,stamp(i)));
 h.api.setPreferences({mode:'condensed'},{persist:false});h.context.NetworkToolkitGraphHistory.setPreferences({anchor:'range',start:stamp(-600),end:stamp(-.5)},{persist:false});h.render(rows,'prefix-only',{history:prefix});
 const nav=h.document.querySelector('[data-graph-navigator]');const instant=h.context.NetworkToolkitGraphHistory.instant;assert.equal(instant(nav.dataset.viewStart),instant(prefix.firstTime));assert.equal(instant(nav.dataset.viewEnd),instant(prefix.lastTime));assert.equal(h.document.querySelectorAll('[data-rt-point]').length,1);assert.equal(Number(h.document.querySelector('[data-rt-aggregate]').dataset.rtAggregate),500);
});


test('condensed curve labels are idempotent under child-list observer restoration',()=>{
 const h=harness();h.api.setPreferences({mode:'condensed'},{persist:false});h.render(Array.from({length:301},(_,i)=>reply(i+1,10,stamp(i))));const button=h.document.querySelector('[data-graph-compression-open]');assert(button);let writes=0;const descriptor=Object.getOwnPropertyDescriptor(Object.getPrototypeOf(button),'textContent');Object.defineProperty(button,'textContent',{get(){return descriptor.get.call(this);},set(value){writes++;descriptor.set.call(this,value);}});
 for(let i=0;i<12;i++)h.observe();assert.equal(writes,0,'Unchanged restore must not create another childList mutation');assert.equal(h.document.querySelector('[data-graph-compression-open]'),button);
});

test('Follow latest never calls Details while releasing held selection or during later updates',()=>{
 const h=harness(),rows=Array.from({length:80},(_,i)=>reply(i+1,10,stamp(i)));let calls=0;const original=h.context.NetworkToolkitResultDetails.showGraphSelection;h.context.NetworkToolkitResultDetails.showGraphSelection=value=>{calls++;return original(value);};
 h.render(rows);h.api.selectResult('job',h.api.sampleKey(rows[20]));assert(calls>0);calls=0;
 h.api.jumpLatest('job');assert.equal(calls,0);assert.equal(h.document.querySelector('.mock-entry-details'),null);
 for(let i=0;i<4;i++){h.render([...rows,reply(81+i,12,stamp(80+i))]);h.api.restore();assert.equal(calls,0);assert.equal(h.document.querySelector('.rt-point-selected')?.dataset.rtSequence,String(81+i));}
 h.api.selectResult('job',h.api.sampleKey(rows[21]));assert(calls>0,'Explicit sample selection still opens Details');
});

test('visible span readout shows actual elapsed scope and includes the full condensed history cluster',()=>{
 const h=harness(),rows=Array.from({length:7201},(_,i)=>reply(i+1,10,stamp(i)));h.api.setPreferences({mode:'condensed'},{persist:false});h.render(rows);const label=h.document.querySelector('[data-rt-zoom-level]');assert.equal(label.textContent,'2 h');assert.match(label.title,/nonlinear.*history cluster/);assert.doesNotMatch(label.textContent,/×/);
 h.api.setPreferences({mode:'linear'},{persist:false});h.context.NetworkToolkitGraphHistory.setPreferences({durationMs:10000},{persist:false,fit:true});h.render(rows);assert.equal(h.document.querySelector('[data-rt-zoom-level]').textContent,'10 s');h.api.zoom('job',2);assert.equal(h.document.querySelector('[data-rt-zoom-level]').textContent,'5 s');
});
