(function(root){
 'use strict';
 const unknown='Not recorded',zero=/^0001-01-01T00:00:00(?:\.0+)?(?:Z|\+00:00)$/i;
 const iso=/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/i;
 const escape=value=>String(value??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;').replaceAll("'",'&#39;');
 const formatters=new Map();let localSettings=null,settingsChecked=-Infinity;
 function settings(){
  // The render clock only expires this small formatter cache. Measurement
  // instants always come from the recorded value passed to epoch/format.
  const now=Date.now();if(!localSettings||now-settingsChecked>=1000||now<settingsChecked){localSettings=new Intl.DateTimeFormat().resolvedOptions();settingsChecked=now;}
  return localSettings;
 }
 function epoch(value){
  if(value instanceof Date)return Number.isFinite(value.getTime())?value.getTime():NaN;
  if(typeof value==='number')return Number.isFinite(value)&&Number.isFinite(new Date(value).getTime())?value:NaN;
  if(typeof value!=='string'||!iso.test(value)||zero.test(value))return NaN;
  const [year,month,day,hour,minute,second]=value.match(/^\d+|\d+/g).slice(0,6).map(Number),leap=year%4===0&&(year%100!==0||year%400===0),days=[31,leap?29:28,31,30,31,30,31,31,30,31,30,31];
  if(month<1||month>12||day<1||day>days[month-1]||hour>23||minute>59||second>59)return NaN;
  const number=Date.parse(value);return Number.isFinite(number)?number:NaN;
 }
 function format(value,{date=true,milliseconds=true,timezone=true}={}){
  const number=epoch(value);if(!Number.isFinite(number))return unknown;
  const options={hour:'2-digit',minute:'2-digit',second:'2-digit',hourCycle:'h23'};
  if(date)Object.assign(options,{year:'numeric',month:'2-digit',day:'2-digit'});
  if(milliseconds)options.fractionalSecondDigits=3;
  if(timezone)options.timeZoneName='short';
  const local=settings(),key=JSON.stringify([local.locale,local.timeZone,date,milliseconds,timezone]);let formatter=formatters.get(key);
  if(!formatter){if(local.timeZone)options.timeZone=local.timeZone;formatter=new Intl.DateTimeFormat(local.locale,options);if(formatters.size>=16)formatters.delete(formatters.keys().next().value);formatters.set(key,formatter);}
  return formatter.format(new Date(number));
 }
 function value(row){
  if(!row||typeof row!=='object')return undefined;
  for(const key of ['time','timestamp','observedAt','recordedAt','snapshotTime','captured','checkedAt'])if(Number.isFinite(epoch(row[key])))return row[key];
  return undefined;
 }
 function cell(stamp,{label='Recorded time',milliseconds=true}={}){
  const number=epoch(stamp);if(!Number.isFinite(number))return '<span class="muted result-time-unknown">'+unknown+'</span>';
  const full=format(stamp,{milliseconds}),original=typeof stamp==='string'?stamp:new Date(number).toISOString();
  return '<time class="mono result-recorded-time" datetime="'+escape(original)+'" data-copy-label="'+escape(label)+'" data-copy-value="'+escape(full)+'" title="'+escape(full+'\nRecorded timestamp: '+original)+'">'+escape(full)+'</time>';
 }
 const isKey=key=>['time','timestamp','observedAt','recordedAt','snapshotTime','captured','checkedAt'].includes(key);
 const snapshot=job=>['routes','arp','connections','wifi','ipconfig'].includes(job?.tool)||job?.result?.data?.timeKind==='snapshot';
 function recorded(row,job){return value(row)??(snapshot(job)?value(job?.result?.data):undefined);}
 function column(job){
  const name=snapshot(job)?'Snapshot (local)':'Recorded (local)';
  return {id:'time',name,type:'date',value:row=>recorded(row,job),render:row=>cell(recorded(row,job),{label:name})};
 }
 const api=Object.freeze({epoch,format,cell,value,isKey,recorded,column,snapshot,unknown});
 if(typeof module==='object'&&module.exports)module.exports=api;
 if(root)root.NetworkToolkitResultTime=api;
})(typeof window!=='undefined'?window:typeof globalThis!=='undefined'?globalThis:this);
