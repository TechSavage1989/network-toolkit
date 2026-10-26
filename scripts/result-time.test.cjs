'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),{spawnSync}=require('node:child_process');
const time=require('../web/assets/result-time.js');

test('recorded time accepts explicit instants and keeps missing legacy time unknown',()=>{
 const instant='2026-10-26T19:20:30.123456789Z';
 assert.equal(time.epoch(instant),Date.parse('2026-10-26T15:20:30.123-04:00'));
 assert.equal(time.epoch(0),0);
 for(const value of [null,undefined,'',' ','0','12/11/2026','2026-10-26T12:30:00','2026-02-30T00:00:00Z','2026-01-01T24:00:00Z','0001-01-01T00:00:00Z','not a date',Infinity]){
  assert(Number.isNaN(time.epoch(value)),String(value));assert.equal(time.format(value),'Not recorded');
 }
 assert.equal(time.value({time:'',observedAt:instant}),instant);
 assert.equal(time.value({}),undefined);
});
test('snapshot columns use recorded acquisition metadata while sample rows never inherit job completion',()=>{
 const stamp='2026-10-26T19:20:30.123Z',completed='2026-10-26T19:30:30Z';
 const snapshot={tool:'connections',finished:completed,result:{data:{captured:stamp,timeKind:'snapshot'}}};
 assert.equal(time.column(snapshot).name,'Snapshot (local)');assert.equal(time.column(snapshot).value({}),stamp);
 assert.equal(time.column({tool:'ping',finished:completed,result:{time:completed,data:{time:completed}}}).value({sequence:1}),undefined);
 assert.equal(time.column(snapshot).value({time:'2026-10-26T19:19:00Z'}),'2026-10-26T19:19:00Z');
 assert.equal(time.value({captured:stamp}),stamp);
});
test('local timestamp retains milliseconds and original nanosecond instant for copy and details',()=>{
 const stamp='2026-10-26T19:20:30.123456789Z',cell=time.cell(stamp);
 assert(cell.includes('datetime="'+stamp+'"'));assert(cell.includes('data-copy-value="'));
 assert(cell.includes('123'));assert(cell.includes('Recorded timestamp: '+stamp));
 assert.equal(time.cell(undefined).includes('<time'),false);
 assert(!time.cell(stamp,{label:'<unsafe "label">'}).includes('<unsafe'));
});
test('local display follows DST while chronological sorting retains the actual instant',()=>{
 const source=`const t=require(${JSON.stringify(require.resolve('../web/assets/result-time.js'))});process.stdout.write(JSON.stringify(['2026-11-01T05:30:00.456Z','2026-11-01T06:30:00.456Z'].map(v=>({text:t.format(v),epoch:t.epoch(v)}))));`;
 const r=spawnSync(process.execPath,['-e',source],{env:{...process.env,TZ:'America/New_York'},encoding:'utf8'});assert.equal(r.status,0,r.stderr);
 const values=JSON.parse(r.stdout);assert(values[0].text.includes('01:30:00.456'));assert(values[1].text.includes('01:30:00.456'));
 assert.notEqual(values[0].text,values[1].text,'Repeated local clock hour includes its distinct timezone');
 assert.equal(values[1].epoch-values[0].epoch,3600000);
});
test('a large render reuses bounded formatters and periodically refreshes local zone settings',()=>{
 const vm=require('node:vm'),fs=require('node:fs');let constructions=0,clock=0;
 class ClockDate extends Date{static now(){return clock;}}
 const DateTimeFormat=function(...args){constructions++;return new Intl.DateTimeFormat(...args);};
 const context=vm.createContext({Date:ClockDate,Intl:{DateTimeFormat}});
 vm.runInContext(fs.readFileSync(require.resolve('../web/assets/result-time.js'),'utf8'),context);
 for(let i=0;i<1000;i++)context.NetworkToolkitResultTime.format('2026-10-26T19:20:30.123Z');
 assert.equal(constructions,2,'One settings resolver and one formatter per render option');
 clock=1000;context.NetworkToolkitResultTime.format('2026-10-26T19:20:30.123Z');
 assert.equal(constructions,3,'Local settings refresh without recreating an unchanged formatter');
});
