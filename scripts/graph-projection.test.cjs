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
 const s={start:'2026-10-20T00:00:00Z',end:'2026-10-27T00:00:00Z'};for(const share of [0,.1,.25,.5,.75,.9,1]){const stamp=history.at(share,s,'condensed',60000);assert(Math.abs(history.fraction(stamp,s,'condensed',60000)-share)<1e-10);}assert.equal(history.at(0,s,'condensed'), '2026-10-26T23:57:00.000000000Z');assert.equal(history.at(1,s,'condensed'),'2026-10-27T00:00:00.000000000Z');
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

test('Condensed has fixed recent pixel spacing from sparse startup through unlimited summary growth',()=>{
 const end=history.instant(scope.end),atAge=seconds=>history.timestamp(end-BigInt(Math.round(seconds*1e9))),ages=[0,.25,1,5,15,30,59,60,120,179];let reference;
 for(const seconds of [0,1,10,60,179,180,181,706,1401,7201,1000000]){
  const s={start:atAge(seconds),end:scope.end},positions=ages.map(age=>history.fraction(atAge(age),s,'condensed'));
  assert.equal(positions[0],1);assert.equal(history.fraction(atAge(180),s,'condensed'),0);assert.equal(history.fraction(atAge(1000000),s,'condensed'),0,'All older history occupies exactly one boundary');
  if(seconds<180)assert(history.fraction(s.start,s,'condensed')>0,'Sparse startup leaves honest blank space');
  if(reference)for(const width of [280,760])positions.forEach((x,i)=>assert(Math.abs((x-reference[i])*width)<1e-9));else reference=positions;
  for(const [i,stamp] of ages.map(atAge).entries())assert.equal(history.at(positions[i],s,'condensed'),stamp);
 }
});
test('fixed finite curve is smooth with progressively compressed older intervals and no warm-up change',()=>{
 const end=history.instant(scope.end),atAge=seconds=>history.timestamp(end-BigInt(Math.round(seconds*1e9))),s={start:atAge(1000000),end:scope.end},x=age=>history.fraction(atAge(age),s,'condensed');let previous=Infinity;
 for(const age of [0,1,30,59,60,61,120,179]){const slope=(x(age)-x(age+.001))/.001;assert(slope>0&&slope<=previous);previous=slope;}
 for(const age of [1,30,60,100,179])assert.equal(history.fraction(atAge(age),{start:atAge(179.999),end:scope.end},'condensed'),history.fraction(atAge(age),{start:atAge(180.001),end:scope.end},'condensed'));
});
test('inverse plot drag and explicit zoom preserve grabbed instant using the same fixed horizon',()=>{
 const end=history.instant(scope.end);
 for(const mode of ['linear','condensed'])for(const seconds of [10,180,181,7200])for(const [from,to] of [[.2,.35],[.45,.55],[.8,.9]]){
  const s={start:history.timestamp(end-BigInt(seconds)*1000000000n),end:scope.end,condensedMs:180000},anchor=history.instant(history.at(from,s,mode)),next=history.panScope(s,from,to,mode);
  assert.equal(history.instant(next.end)-history.instant(next.start),BigInt(seconds)*1000000000n);assert.equal(history.instant(history.at(to,next,mode)),anchor);
  for(const zoom of [.5,2,8]){const finish=history.anchoredEnd(s,1,zoom,from,mode),zoomed=history.viewport(s,zoom,finish),hit=history.instant(history.at(from,zoomed,mode));assert(hit>=anchor-1n&&hit<=anchor+1n);}
 }
});
test('packet interval scope keeps its published full-span nonlinear projection',()=>{
 const s={...scope,legacyCondensed:true};for(const share of [0,.1,.5,.9,1])assert(Math.abs(history.fraction(history.at(share,s,'condensed'),s,'condensed')-share)<1e-9);assert.equal(history.fraction(scope.start,s,'condensed'),0);assert.equal(history.fraction(scope.end,s,'condensed'),1);
});
