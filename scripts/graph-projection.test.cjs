'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
const time=require('../web/assets/result-time.js');global.NetworkToolkitResultTime=time;
const history=require('../web/assets/graph-history.js');
const scope={start:'2026-10-27T00:00:00.000000001Z',end:'2026-10-27T00:00:10.000000001Z'};
test('each mode projects the complete 10-second domain to the viewport and preserves genuine gaps',()=>{
 for(const mode of ['window','progressive']){assert.equal(history.fraction(scope.start,scope,mode),0);assert.equal(history.fraction(scope.end,scope,mode),1);assert.equal(history.fraction('2026-10-27T00:00:01.000000001Z',scope,mode),.1);assert.equal(history.fraction('2026-10-27T00:00:08.000000001Z',scope,mode),.8);}
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
