'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
const time=require('../web/assets/result-time.js');global.NetworkToolkitResultTime=time;
const history=require('../web/assets/graph-history.js');
const scope={start:'2026-10-27T00:00:00.000000001Z',end:'2026-10-27T00:00:10.000000001Z'};
test('each mode projects the complete 10-second domain to the viewport and preserves genuine gaps',()=>{
 for(const mode of ['linear','window','progressive']){assert.equal(history.fraction(scope.start,scope,mode),0);assert.equal(history.fraction(scope.end,scope,mode),1);assert.equal(history.fraction('2026-10-27T00:00:01.000000001Z',scope,mode),.1);assert.equal(history.fraction('2026-10-27T00:00:08.000000001Z',scope,mode),.8);}
});
test('zoom scales actual time while preserving raw nanosecond endpoints and selected interval settings',()=>{
 const before=JSON.stringify(scope),zoomed=history.viewport(scope,2);assert.equal(zoomed.start,'2026-10-27T00:00:05.000000001Z');assert.equal(zoomed.end,scope.end);assert.equal(JSON.stringify(scope),before);const panned=history.viewport(scope,2,'2026-10-27T00:00:07.000000001Z');assert.equal(panned.start,'2026-10-27T00:00:02.000000001Z');assert.equal(panned.end,'2026-10-27T00:00:07.000000001Z');
});
test('condensed projection labels its nonlinear distribution by the inverse mapping',()=>{
 const s={start:'2026-10-20T00:00:00Z',end:'2026-10-27T00:00:00Z'};for(const share of [0,.1,.25,.5,.75,.9,1]){const stamp=history.at(share,s,'condensed',60000);assert(Math.abs(history.fraction(stamp,s,'condensed',60000)-share)<1e-10);}assert.equal(history.at(0,s,'condensed'), '2026-10-20T00:00:00.000000000Z');assert.equal(history.at(1,s,'condensed'),'2026-10-27T00:00:00.000000000Z');
});
test('sub-millisecond capture intervals project distinct adjacent nanoseconds without rounding source instants',()=>{
 const s={start:'2026-10-27T00:00:00.123456780Z',end:'2026-10-27T00:00:00.123456790Z'};assert.equal(history.fraction('2026-10-27T00:00:00.123456781Z',s),.1);assert.equal(history.at(.1,s),'2026-10-27T00:00:00.123456781Z');assert.equal(history.viewport(s,2).start,'2026-10-27T00:00:00.123456785Z');assert.match(history.axisHTML(s,400),/123456780/);assert.match(history.axisHTML(s,400),/123456790/);
});
test('adaptive ticks respect configured 12/24-hour clock and named timezone and retain exact instants',()=>{
 time.configure({hourFormat:'12',zoneMode:'named',timeZone:'America/New_York'});const narrow=history.axisHTML(scope,300);assert.match(narrow,/08:00:00 PM/);assert.match(narrow,/2026-10-27T00:00:00.000000001Z/);const wide=history.axisHTML(scope,1200);assert((wide.match(/<time/g)||[]).length>(narrow.match(/<time/g)||[]).length);time.configure({hourFormat:'24',zoneMode:'utc'});assert.match(history.axisHTML(scope,600),/00:00:00/);assert.doesNotMatch(history.axisHTML(scope,600),/AM|PM/);
});
test('unknown times are not projected or invented and equal instants get an honest nonzero view domain',()=>{
 assert(Number.isNaN(history.fraction('0001-01-01T00:00:00Z',scope)));assert.match(history.axisHTML({start:'',end:''},400),/Recorded time unavailable/);const s=history.viewport({start:scope.end,end:scope.end});assert.equal(history.instant(s.end)-history.instant(s.start),0n);assert.equal(s.end,scope.end);
});

test('compressed time ticks increase displayed precision instead of repeating indistinguishable second labels',()=>{
 time.configure({hourFormat:'24',zoneMode:'utc'});history.setPreferences({recentMs:1000},{persist:false});const html=history.axisHTML(scope,820,'condensed'),labels=[...html.matchAll(/>([^<]+)<\/time>/g)].map(match=>match[1]);assert.equal(new Set(labels).size,labels.length);assert(labels.length>=2);history.setPreferences({recentMs:60000},{persist:false});
});

test('Condensed Fit has visibly increasing equal-time spacing even when retained history is shorter than the recent interval',()=>{
 for(const seconds of [1,10,60,7200]){
  const start=history.instant(scope.start),span=BigInt(seconds)*1000000000n,s={start:scope.start,end:history.timestamp(start+span)},times=Array.from({length:21},(_,i)=>history.timestamp(start+span*BigInt(i)/20n)),positions=times.map(stamp=>history.fraction(stamp,s,'condensed',60000)),gaps=positions.slice(1).map((x,i)=>x-positions[i]);
  assert.equal(positions[0],0);assert.equal(positions.at(-1),1);
  for(let i=1;i<gaps.length;i++)assert(gaps[i]>gaps[i-1],seconds+'s progressively spreads equal-time samples toward the right');
  assert(gaps.at(-1)>gaps[0]*2,seconds+'s must not look uniformly compressed');
  for(const [i,stamp] of times.entries()){
   assert(Math.abs(history.fraction(stamp,s,'linear')-i/20)<1e-12,'Linear preserves timestamp-proportional spacing');
   const reconstructed=history.instant(history.at(positions[i],s,'condensed',60000));assert(reconstructed>=history.instant(stamp)-1n&&reconstructed<=history.instant(stamp)+1n,'Condensed inverse preserves the actual source instant');
  }
 }
});

test('recent Condensed pixel spacing is independent of older history after the Fit warm-up',()=>{
 const end=history.instant(scope.end),recent=60000,ages=[0,.25,1,5,15,30,59,60],atAge=seconds=>history.timestamp(end-BigInt(Math.round(seconds*1e9)));let reference;
 for(const seconds of [180,706,1399,1400,1401,7201,1000000]){
  const s={start:atAge(seconds),end:scope.end},positions=ages.map(age=>history.fraction(atAge(age),s,'condensed',recent));
  assert.equal(history.fraction(s.start,s,'condensed',recent),0);assert.equal(positions[0],1);assert.equal(positions.at(-1),.5,'Configured recent interval owns the right half');
  if(reference)for(const width of [280,760])positions.forEach((x,i)=>assert(Math.abs((x-reference[i])*width)<1e-9,'Older history cannot squeeze recent pixels'));else reference=positions;
  for(const [i,stamp] of ages.map(atAge).entries())assert.equal(history.at(positions[i],s,'condensed',recent),stamp,'Recent inverse returns the precise original instant');
 }
});

test('Condensed old/recent join is continuous in position and slope with steadily increasing age compression',()=>{
 const end=history.instant(scope.end),atAge=seconds=>history.timestamp(end-BigInt(Math.round(seconds*1e9))),eps=.00001;
 for(const seconds of [180,181,706,1401,7201,1000000]){
  const s={start:atAge(seconds),end:scope.end},x=age=>history.fraction(atAge(age),s,'condensed',60000),recentSlope=(x(60-eps)-x(60))/eps,olderSlope=(x(60)-x(60+eps))/eps;
  assert(Math.abs(olderSlope/recentSlope-1)<.0001,'No two-speed join at '+seconds+'s');
  const slopes=[0,1,30,59,60,61,120,seconds-.01].map(age=>(x(age)-x(age+.001))/.001);for(let i=1;i<slopes.length;i++)assert(slopes[i]<=slopes[i-1]+1e-10,'Compression increases with age');
 }
 for(const age of [1,30,60,100,179]){const stamp=atAge(age),before=history.fraction(stamp,{start:atAge(179.999),end:scope.end},'condensed'),after=history.fraction(stamp,{start:atAge(180.001),end:scope.end},'condensed');assert(Math.abs(after-before)<.00001,'Warm-up enters stable resolution without a jump');}
});

test('inverse-based plot dragging and zoom preserve the grabbed instant on both sides of the recent join',()=>{
 const end=history.instant(scope.end);
 for(const mode of ['linear','condensed'])for(const seconds of [10,180,181,7200])for(const [from,to] of [[.2,.35],[.45,.55],[.8,.9]]){
  const s={start:history.timestamp(end-BigInt(seconds)*1000000000n),end:scope.end},anchor=history.instant(history.at(from,s,mode)),next=history.panScope(s,from,to,mode);
  assert.equal(history.instant(next.end)-history.instant(next.start),BigInt(seconds)*1000000000n);assert.equal(history.instant(history.at(to,next,mode)),anchor,'Drag follows the nonlinear inverse rather than linear dx');
  for(const zoom of [.5,2,8]){const finish=history.anchoredEnd(s,1,zoom,from,mode),zoomed=history.viewport(s,zoom,finish),hit=history.instant(history.at(from,zoomed,mode));assert(hit>=anchor-1n&&hit<=anchor+1n,'Pointer-anchored zoom crosses the region/warm-up boundary accurately');}
 }
});
