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
 return {document,api,render,observe:()=>observer?.(),detail:()=>document.querySelector('.rt-detail').textContent,mark:index=>document.querySelector('[data-rt-surface="latency"] [data-rt-point="'+index+'"]')};
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
 h.document.emit('pointermove',h.mark(2));assert.match(h.detail(),/Timed out/);assert.match(h.detail(),/Not measured/);assert.doesNotMatch(h.detail(),/999 ms/);
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

test('pinned selections are job-scoped and expire when outside the displayed window',()=>{
 const h=harness();h.render();h.document.emit('click',h.mark(0));
 h.render(samples(),'other-job');assert.match(h.detail(),/Hover over a point/);
 h.render(samples(),'job');assert.match(h.detail(),/Sample1/);
 h.render(Array.from({length:181},(_,i)=>reply(i+1,10,'2026-10-26T16:00:00Z')),'job');
 assert.match(h.detail(),/outside the displayed window/);
 assert.equal(h.document.querySelector('[data-rt-clear]').hasAttribute('disabled'),true);
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
