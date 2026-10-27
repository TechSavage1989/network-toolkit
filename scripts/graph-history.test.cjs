'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),time=require('../web/assets/result-time.js'),history=require('../web/assets/graph-history.js');
const stamp=n=>new Date(Date.parse('2026-10-26T12:00:00Z')+n*1000).toISOString();
test('time-range preferences reject malformed state and preserve supported durations and anchors',()=>{
 for(const value of [null,[],{durationMs:'300000',recentMs:'10',anchor:'bad'},{durationMs:Infinity}])assert.deepEqual(history.normalize(value),history.defaults);
 const valid=history.normalize({durationMs:300000,recentMs:10000,anchor:'range',start:stamp(1),end:stamp(9)});assert.equal(valid.anchor,'range');assert.equal(valid.durationMs,300000);assert.equal(valid.recentMs,10000);
 assert.equal(history.normalize({...valid,start:stamp(10)}).anchor,'latest');assert.equal(history.normalize({durationMs:-1}).durationMs,0);
});
test('relative, fixed-end and explicit interval views include exact recorded boundaries without altering data',()=>{
 const rows=Array.from({length:20},(_,i)=>({sequence:i+1,time:stamp(i),rttMs:i})),before=JSON.stringify(rows),pref=history.normalize({durationMs:5000});let range=history.range(rows,pref);assert.equal(time.compareInstants(range.start,stamp(14)),0);assert.deepEqual(history.select(rows,range).map(r=>r.sequence),[15,16,17,18,19,20]);
 range=history.range(rows,pref,stamp(10));assert.deepEqual(history.select(rows,range).map(r=>r.sequence),[6,7,8,9,10,11]);
 range=history.range(rows,history.normalize({anchor:'range',start:stamp(3),end:stamp(5)}));assert.deepEqual(history.select(rows,range).map(r=>r.sequence),[4,5,6]);assert.equal(JSON.stringify(rows),before);
});
test('range boundaries preserve nanoseconds, UTC offsets and unknown-time honesty',()=>{
 const values=[{time:'2026-11-01T01:00:00.625000001-04:00'},{time:'2026-11-01T05:00:00.625000002Z'},{time:'0001-01-01T00:00:00Z'},{}],range=history.range(values,history.normalize({anchor:'range',start:values[1].time,end:values[1].time}));assert.equal(history.select(values,range).length,1);assert.equal(range.unknown,2);assert.equal(history.shift(values[0].time,1000),'2026-11-01T05:00:01.625000001Z');
});
test('condensed history keeps recent samples individual and groups older data without loss or duplication',()=>{
 const rows=Array.from({length:7201},(_,i)=>({sequence:i+1,time:stamp(i),success:i%19!==0,rttMs:i===1?999:10})),before=JSON.stringify(rows),groups=history.condense(rows,{end:stamp(7200),recentMs:60000}),members=groups.flatMap(g=>g.items);assert(groups.length<250);assert.equal(members.length,rows.length);assert.equal(new Set(members).size,rows.length);assert(groups.filter(g=>!g.aggregate).length>=60);assert(groups.find(g=>g.items.includes(rows[1])).items.some(r=>r.rttMs===999));assert.equal(members.filter(r=>!r.success).length,rows.filter(r=>!r.success).length);assert.equal(JSON.stringify(rows),before);
 const older=groups.filter(g=>g.aggregate);assert(older[0].items.length>older.at(-1).items.length,'older intervals are more condensed');
});
test('time entry uses the configured named zone and rejects ambiguous or nonexistent DST wall times',()=>{
 time.configure({hourFormat:'12',zoneMode:'named',timeZone:'America/New_York'});assert.equal(history.parseTime('2026-10-26 2:30:00 PM'),'2026-10-26T18:30:00.000000000Z');assert.equal(history.parseTime('2026-10-26T14:30:00.123456789-04:00'),'2026-10-26T14:30:00.123456789-04:00');
 assert.throws(()=>history.parseTime('2026-11-01 01:30:00'),/occurs twice/);assert.throws(()=>history.parseTime('2026-03-08 02:30:00'),/does not exist/);assert.throws(()=>history.parseTime('2026-02-30 12:00'),/valid calendar/);time.configure({zoneMode:'utc'});
});
test('all retained time, empty ranges and saved display settings remain independent of raw data',()=>{
 history.setPreferences({durationMs:900000,anchor:'end',end:stamp(10)},{persist:false});const saved=history.preferences();history.mergePreferences({graphTimeRange:saved});assert.deepEqual(history.preferences(),saved);assert.equal(history.select([{time:stamp(12)}],history.range([{time:stamp(12)}],saved)).length,0);history.mergePreferences({});assert.equal(history.range([]).unknown,0);assert.deepEqual(history.select([],history.range([])),[]);
});
