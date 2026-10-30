'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
global.NetworkToolkitResultTime=require('../web/assets/result-time.js');
const h=require('../web/assets/graph-history.js');
const stamp=n=>new Date(Date.parse('2026-10-29T00:00:00Z')+n*1000).toISOString(),rows=n=>Array.from({length:n},(_,i)=>({time:stamp(i),sequence:i+1}));
test('curve preferences validate separately and round-trip without changing old settings',()=>{const p=h.normalize({durationMs:10000,recentMs:120000,compression:{count:71,strength:.7,curve:.2}});assert.deepEqual(p.compression,{count:71,strength:.7,curve:.2});h.mergePreferences({graphTimeRange:JSON.parse(JSON.stringify(p))});assert.deepEqual(h.preferences(),p);assert.deepEqual(h.normalizeCompression({count:1,strength:9,curve:-1}),{count:2,strength:1,curve:0});assert.deepEqual(h.normalizeCompression({count:'3',strength:NaN}),h.defaults.compression);h.mergePreferences({});});
test('every curve is monotonic and invertible; recorded instants map through the same axis transform',()=>{for(const strength of [0,.5,1])for(const curve of [0,.5,1]){const items=rows(5000),p=h.sampleProjection(items,{}, {mode:'condensed',compression:{count:12,strength,curve},width:744}),s=h.projectScope({},p);let previous=-Infinity;for(let i=0;i<=200;i++){const x=i/200,t=h.at(x,s,'condensed'),again=h.fraction(t,s,'condensed');assert(Math.abs(again-x)<1e-8);assert(h.instant(t)>=previous);previous=h.instant(t);}assert.equal(h.fraction(s.end,s,'condensed'),1);assert(Math.abs(h.fraction(s.start,s,'condensed'))<1e-8);}});
test('recent spacing never depends on accumulated old history; width limits preserve saved count',()=>{let ref;for(const n of [2,12,55,500,10000]){const items=rows(n),p=h.sampleProjection(items,{}, {mode:'condensed',width:744,compression:{count:12,strength:.55,curve:.5}}),s=h.projectScope({},p),gap=1-h.fraction(stamp(n-2),s,'condensed');if(ref)assert(Math.abs(gap-ref)<1e-10);ref=gap;if(n===2)assert(h.fraction(items[0].time,s,'condensed')>0);if(n===10000)assert(p.filled);}const value={count:500,strength:.8,curve:.1},copy=JSON.stringify(value),p=h.sampleProjection(rows(100),{}, {mode:'condensed',width:180,compression:value});assert(p.widthLimited);assert(p.count<p.requestedCount);assert.equal(JSON.stringify(value),copy);assert.equal(p.requestedCount,500);});
test('linear startup fills naturally and full history pins exact recorded endpoints',()=>{const items=rows(3),p=h.sampleProjection(items,{}, {filled:false,referenceMs:30000}),s=h.projectScope({},p);assert.equal(h.fraction(items.at(-1).time,s),1);assert(h.fraction(items[0].time,s)>.9);const full=h.sampleProjection(rows(91),{}, {filled:true}),scope=h.projectScope({},full);assert.equal(h.fraction(stamp(0),scope),0);assert.equal(h.fraction(stamp(90),scope),1);});
test('curve control is compact and has a real draggable/keyboard-accessible point and meaningful direction',()=>{assert.match(h.compressionControlHTML(),/data-graph-compression-open/);assert.match(h.compressionControlHTML(7),/aria-label="Adjust compression curve"/);const html=h.compressionPreview(h.defaults.compression);assert.match(html,/data-graph-curve-handle/);assert.match(html,/role="slider"/);assert.match(html,/Older \/ compressed/);assert.match(html,/Newest \/ normal/);});
test('irregular cadence counts actual newest points and huge gaps reduce effective count without reversal',()=>{for(const values of [[0,1,2,3,15,16],[0,10,20],[0,0,0,1,1,2]])for(const referenceMs of [1000,30000]){const items=values.map(n=>({time:stamp(n)})),p=h.sampleProjection(items,{}, {mode:'condensed',width:300,referenceMs,compression:{count:4,strength:.5,curve:.5}});assert(p.recentShare<=.65);assert(Math.abs(h.projectionDistance(p.recent,p)-p.recentShare)<1e-12);assert(h.projectionDistance(p.recent+1,p)>=p.recentShare);const end=h.instant(p.end),linear=items.filter(r=>Number(end-h.instant(r.time))<=p.recent);assert(linear.length>=Math.min(p.count,items.length),'coincident timestamps stay coincident');if(p.count<=items.length)assert.equal(p.recent,Number(end-h.instant(items[items.length-p.count].time)));let last=-Infinity;for(let n=0;n<=200;n++){const d=h.projectionDistance(n*p.horizon/200,p);assert(d>=last);last=d;}}});
test('configured curve wheel/drag inverse uses its actual mapping rather than the retired logarithmic curve',()=>{const p=h.sampleProjection(rows(500),{}, {mode:'condensed',compression:{count:17,strength:.9,curve:.1}}),scope=h.projectScope({},p);for(const share of [.1,.4,.8]){const anchor=h.instant(h.at(share,scope,'condensed'));for(const zoom of [.5,2,8]){const end=h.anchoredEnd(scope,1,zoom,share,'condensed'),next=h.viewport(scope,zoom,end),t=h.instant(h.at(share,next,'condensed'));assert(t>=anchor-1n&&t<=anchor+1n);}const pan=h.panScope(scope,share,share+.05,'condensed');assert.equal(h.instant(h.at(share+.05,pan,'condensed')),anchor);}});

test('default curve matches a 10-second normal timeframe at each width and cadence without growth decay',()=>{
 for(const width of [220,744,1400])for(const cadence of [.1,.5,1,2]){
  let gap;
  for(const count of [120,720,7201]){
   const items=Array.from({length:count},(_,i)=>({time:stamp(i*cadence)})),p=h.sampleProjection(items,{}, {mode:'condensed',width}),scope=h.projectScope({},p),last=items.at(-1),previous=items.at(-2),distance=(h.fraction(last.time,scope,'condensed')-h.fraction(previous.time,scope,'condensed'))*width;
   assert(Math.abs(distance-width*cadence/10)<1e-6,'Recent spacing equals 10-second linear reference');if(gap!==undefined)assert(Math.abs(distance-gap)<1e-7);gap=distance;
   assert.equal(h.fraction(last.time,scope,'condensed'),1);assert(p.widthLimited===false||p.count<p.requestedCount);
  }
 }
});


test('120 selected durations never compress same-age Fit spacing and short spans only expand',()=>{
 for(const width of [220,744,1262])for(const strength of [0,.55,1])for(const curve of [0,.5,1]){
  const items=rows(303),base=h.sampleProjection(items,{}, {mode:'condensed',width,compression:{count:3,strength,curve}}),end=h.instant(base.end),before=JSON.stringify(base);
  for(const seconds of [4,10,30,300,900]){
   const scope={start:h.timestamp(end-BigInt(seconds)*1000000000n),end:base.end},p=h.scaleProjection(base,scope),s=h.projectScope(scope,p);
   assert.equal(s.start,scope.start);assert.equal(s.end,scope.end);assert.equal(p.recent,base.recent);assert.equal(p.tail,base.tail);assert.equal(p.slope,base.slope);assert.equal(p.count,base.count);assert.equal(p.curve,base.curve);
   const gap=(1-h.fraction(h.timestamp(end-1000000000n),s,'condensed'))*width,baseline=width/10;assert(gap>=baseline-1e-7);
   if(seconds*1e9>=base.horizon)assert(Math.abs(gap-baseline)<1e-7);else assert(Math.abs(h.fraction(scope.start,s,'condensed'))<1e-9,'Short window fills the left edge');
   let previous=-Infinity;for(let i=0;i<=200;i++){const x=i/200,t=h.at(x,s,'condensed'),again=h.fraction(t,s,'condensed');assert(Math.abs(again-x)<1e-8);assert(h.instant(t)>=previous);previous=h.instant(t);}
   for(const zoom of [.5,2])for(const share of [.2,.8]){const anchor=h.instant(h.at(share,s,'condensed')),next=h.viewport(s,zoom,h.anchoredEnd(s,1,zoom,share,'condensed'));assert(h.instant(h.at(share,next,'condensed'))>=anchor-1n&&h.instant(h.at(share,next,'condensed'))<=anchor+1n);}
  }
  assert.equal(JSON.stringify(base),before);
 }
 const items=rows(303),base=h.sampleProjection(items,{}, {mode:'condensed',width:1262,compression:{count:3,strength:.55,curve:.5}}),p=h.scaleProjection(base,{start:stamp(2),end:stamp(302)});assert(Math.abs(h.projectionDistance(1e9,p)*1262-126.2)<1e-7,'Regression: 5m must not reduce the 126.2px Fit gap to8.6px');
});
test('120 selected startup retains natural baseline fill until sufficient actual history exists',()=>{
 const items=rows(3),base=h.sampleProjection(items,{}, {mode:'condensed',width:1262}),p=h.scaleProjection(base,{start:stamp(0),end:stamp(2)},{fill:false}),s=h.projectScope({},p);assert.equal(p.distanceScale,1);assert(h.fraction(items[0].time,s,'condensed')>0);assert.equal(h.fraction(items.at(-1).time,s,'condensed'),1);
});

test('124 mature selected curves pin actual irregular endpoints without changing the requested interval',()=>{
 for(const width of [220,744,1262])for(const seconds of [10,30,300]){
  const items=[0,1,2,3,4.15,5,6.4,7,8,9.3,10.2].map((n,i)=>({time:stamp(n),sequence:i+1})),first=items[1].time,last=items.at(-1).time,scope={start:stamp(10.25-seconds),end:stamp(10.25)},selected=items.filter(r=>h.contains(r.time,scope)),base=h.sampleProjection(selected,scope,{mode:'condensed',width}),p=h.scaleProjection(base,scope,{first:selected[0].time,last}),s=h.projectScope(scope,p);
  assert.equal(s.start,scope.start);assert.equal(s.end,scope.end);assert.equal(h.fraction(last,s,'condensed'),1);assert(Math.abs(h.fraction(selected[0].time,s,'condensed'))<1e-9);
  assert.equal(p.curve,base.curve);assert.equal(p.recent,base.recent);assert.equal(p.requestedCount,base.requestedCount);assert(p.distanceScale>=1);
  assert.equal(h.instant(h.at(0,s,'condensed')),h.instant(selected[0].time));assert.equal(h.instant(h.at(1,s,'condensed')),h.instant(last));
  for(const r of selected){const x=h.fraction(r.time,s,'condensed');assert(x>=-1e-9&&x<=1);assert(h.instant(h.at(x,s,'condensed'))>=h.instant(r.time)-1n&&h.instant(h.at(x,s,'condensed'))<=h.instant(r.time)+1n);}
  const startup=h.scaleProjection(base,scope,{fill:false,first,last});assert.equal(startup.distanceScale,1);assert.equal(startup.end,scope.end);assert.equal(startup.recordedStart,'');
 }
 const items=rows(400),scope={start:stamp(90.25),end:stamp(399.4)},base=h.sampleProjection(items,scope,{mode:'condensed',compression:{count:3,strength:.55,curve:.5}}),p=h.scaleProjection(base,scope,{first:stamp(91),last:stamp(399)});assert.equal(p.distanceScale,1,'Long populated windows must not rescale recent density');assert.equal(h.projectionDistance(1e9,p),h.projectionDistance(1e9,base));
 const linear=h.sampleProjection(items,scope,{mode:'linear'}),selected=h.scaleProjection(linear,scope,{first:stamp(91),last:stamp(399)});assert.equal(selected.start,scope.start);assert.equal(selected.end,scope.end,'Linear remains time proportional');
});
