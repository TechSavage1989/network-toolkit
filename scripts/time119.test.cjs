'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm'),path=require('node:path');
const root=path.join(__dirname,'..'),time=require('../web/assets/result-time.js'),html=fs.readFileSync(path.join(root,'web/index.html'),'utf8');
const read=name=>fs.readFileSync(path.join(root,'web/assets',name),'utf8');

test('fresh and malformed clock settings default to 12-hour while explicit saved formats survive normalization/reload',()=>{
 const source=read('result-time.js');
 for(const saved of [undefined,null,{},[],{hourFormat:'invalid'},{hourFormat:'24'},{hourFormat:'12'}]){
  const state={resultTimePreferences:saved},before=JSON.stringify(state),c=vm.createContext({Intl,Date,state});vm.runInContext(source,c);const t=c.NetworkToolkitResultTime;
  assert.equal(t.preferences().hourFormat,saved?.hourFormat==='24'?'24':'12');assert.equal(JSON.stringify(state),before,'formatting never rewrites stored preferences');
  assert.equal(t.normalizePreferences(t.preferences()).hourFormat,t.preferences().hourFormat);
 }
 assert.match(html,/resultTimePreferences:\{hourFormat:'12',zoneMode:'local',timeZone:'UTC'\}/);
 for(const [stamp,pattern]of [['2026-10-30T00:00:00Z',/12:00:00\s*AM/],['2026-10-30T12:00:00Z',/12:00:00\s*PM/],['2026-10-30T23:59:59Z',/11:59:59\s*PM/]])assert.match(time.format(stamp,{date:false,milliseconds:false,timezone:false,preferences:{zoneMode:'utc'}}),pattern);
 assert.match(time.format('2026-10-30T00:00:00Z',{date:false,milliseconds:false,timezone:false,preferences:{hourFormat:'24',zoneMode:'utc'}}),/^00:00:00$/);
});

test('total elapsed is HH:MM:SS, crosses midnight and 24 hours without wrapping, and never changes recorded data',()=>{
 const cases=[['2026-10-30T00:00:00Z','2026-10-30T00:00:00.999999999Z','00:00:00'],['2026-10-29T23:59:58Z','2026-10-30T00:00:03Z','00:00:05'],['2026-10-28T01:02:03Z','2026-10-30T02:25:48Z','49:23:45'],['2026-11-01T01:30:00-04:00','2026-11-01T01:30:00-05:00','01:00:00']];
 for(const [started,finished,expected]of cases){const job={started,finished,status:'completed'},before=JSON.stringify(job);assert.equal(time.elapsed(job),expected);assert.equal(JSON.stringify(job),before);}
});

test('live elapsed advances without samples and completed/cancelled/failed runs freeze at recorded finish',()=>{
 const started='2026-10-30T00:00:00Z',now=Date.parse(started),job={started,status:'running'};
 assert.equal(time.elapsed(job,now+1000),'00:00:01');assert.equal(time.elapsed(job,now+62000),'00:01:02');
 for(const status of ['completed','cancelled','failed'])for(const clock of [now+2000,now+3600000])assert.equal(time.elapsed({...job,status,finished:'2026-10-30T00:00:01Z'},clock),'00:00:01');
 assert.equal(time.elapsed({...job,status:'queued'},now),'00:00:00');
 for(const value of [{},null,{started,status:'completed'},{started:'bad',status:'running'},{started,status:'running',finished:'2026-10-29T00:00:00Z'},{started:'0001-01-01T00:00:00Z',status:'running'}])assert.equal(time.elapsed(value,now),time.unknown);
 assert.equal(time.elapsed(job,now-1),time.unknown);assert.equal(time.elapsed(job,NaN),time.unknown);
});

test('elapsed updates only existing value text during unchanged polls, with no graph rebuild or document mutation',()=>{
 let clock=Date.parse('2026-10-30T00:00:01Z'),writes=0,value='00:00:00';
 const node={dataset:{jobElapsed:'run119'},get textContent(){return value;},set textContent(next){writes++;value=next;}},orphan={dataset:{jobElapsed:'gone'},textContent:'untouched'},job={id:'run119',started:'2026-10-30T00:00:00Z',status:'running'},before=JSON.stringify(job);
 const c=vm.createContext({document:{querySelectorAll:selector=>{assert.equal(selector,'[data-job-elapsed]');return[node,orphan];}},jobs:[job],NetworkToolkitResultTime:{elapsed:j=>time.elapsed(j,clock)}});
 const start=html.indexOf('const elapsed=j=>'),end=html.indexOf('\n',html.indexOf('function refreshJobElapsed()',start));vm.runInContext(html.slice(start,end),c);
 c.refreshJobElapsed();assert.equal(value,'00:00:01');assert.equal(writes,1);c.refreshJobElapsed();assert.equal(writes,1);clock+=1000;c.refreshJobElapsed();assert.equal(value,'00:00:02');assert.equal(writes,2);assert.equal(orphan.textContent,'untouched');assert.equal(JSON.stringify(job),before);
 job.status='completed';job.finished='2026-10-30T00:00:02Z';clock+=90000;c.refreshJobElapsed();assert.equal(value,'00:00:02');assert.equal(writes,2);
 const poll=html.slice(html.indexOf('async function pollJobs()'),html.indexOf('\nasync function refreshSystem()'));assert.match(poll,/finally\{refreshJobElapsed\(\);polling=false\}/);
});

test('the graph uses one noninteractive elapsed cell in the existing statistics strip, not another header row',()=>{
 const c=vm.createContext({esc:String,fmt:String,elapsed:time.elapsed});const start=html.indexOf('function sampleRunStatistics('),end=html.indexOf('\nfunction simpleTable(',start);vm.runInContext(html.slice(start,end),c);
 const markup=c.sampleRunStatistics({id:'run119',started:'2026-10-28T01:02:03Z',finished:'2026-10-30T02:25:48Z',status:'completed'},{sent:1,received:1},[]);
 assert(markup.startsWith('<div class="result-stats"><div class="result-stat graph-stat"'));
 assert.match(markup,/data-job-elapsed="run119">49:23:45<\/strong><span>Total elapsed<\/span>/);assert.equal((markup.match(/<button /g)||[]).length,7);assert.equal((markup.match(/data-job-elapsed=/g)||[]).length,1);
 assert(!markup.includes('role="button"'),'duration never becomes a statistics filter action');
});

test('file date display helpers use the shared clock preference without changing source values',()=>{
 for(const [file,prefix,name]of [['remote-browser.js','const when=','when'],['transfer-explorer.js','const humanDate=','humanDate']]){
  let seen;const c=vm.createContext({Date,NetworkToolkitResultTime:{format:(date,opts)=>{seen={date:date.getTime(),opts};return '12:34:56 PM';}}});const line=read(file).split('\n').find(x=>x.startsWith(prefix));vm.runInContext(line+';globalThis.call='+name,c);
  const source='2026-10-30T12:34:56Z';assert.equal(c.call(source),'12:34:56 PM');assert.equal(seen.date,Date.parse(source));assert.equal(seen.opts.milliseconds,false);assert.equal(source,'2026-10-30T12:34:56Z');
 }
 for(const file of ['session-sidebar.js','app-updater.js','remote-browser.js','file-editor.js','transfer-explorer.js']){
  for(const line of read(file).split('\n').filter(line=>/toLocale(?:Time)?String\(/.test(line)&&/new Date|d\.toLocale/.test(line))){assert(line.includes('NetworkToolkitResultTime?.format'),file+': shared display clock required');assert(line.includes('hour12:true'),file+': standalone fallback uses requested default');}
 }
});
