(function(root){
 'use strict';
 const unknown='Not recorded',zero=/^0001-01-01T00:00:00(?:\.0+)?(?:Z|\+00:00)$/i;
 const iso=/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,9})?(?:Z|[+-]\d{2}:\d{2})$/i;
 const escape=value=>String(value??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;').replaceAll("'",'&#39;');
 const formatters=new Map(),zoneChecks=new Map();let localSettings=null,settingsChecked=-Infinity,configured=null,availableZones=null;
 const defaults=Object.freeze({hourFormat:'24',zoneMode:'local',timeZone:'UTC'});
 function isTimeZone(value){
  if(typeof value!=='string'||!value||value.length>100||!/^[-A-Za-z0-9_+/]+$/.test(value))return false;
  if(value==='UTC')return true;
  if(zoneChecks.has(value))return zoneChecks.get(value);
  let valid=false;try{new Intl.DateTimeFormat('en-US',{timeZone:value});valid=true;}catch{}
  if(zoneChecks.size>=64)zoneChecks.delete(zoneChecks.keys().next().value);zoneChecks.set(value,valid);return valid;
 }
 function normalizePreferences(value){
  const input=value&&typeof value==='object'&&!Array.isArray(value)?value:{};
  const hourFormat=input.hourFormat==='12'?'12':'24',requestedMode=['local','utc','named'].includes(input.zoneMode)?input.zoneMode:'local';
  const candidate=typeof input.timeZone==='string'?input.timeZone.trim():'UTC',valid=isTimeZone(candidate);
  return {hourFormat,zoneMode:requestedMode==='named'&&!valid?'local':requestedMode,timeZone:valid?candidate:'UTC'};
 }
 function preferences(){
  let saved=configured;
  // The application owns the saved preferences. Resolve them when formatting,
  // so changing clock display never rewrites recorded instants or sort values.
  try{if(typeof state!=='undefined'&&state&&typeof state==='object'&&state.resultTimePreferences!==undefined)saved=state.resultTimePreferences;}catch{}
  return normalizePreferences(saved);
 }
 function configure(value){configured=normalizePreferences(value);return {...configured};}
 function zoneLabel(value=preferences()){const p=normalizePreferences(value);return p.zoneMode==='local'?'local':p.zoneMode==='utc'?'UTC':p.timeZone;}
 function timeLabel(value=preferences()){const zone=zoneLabel(value);return zone==='local'?'Local time':'Time ('+zone+')';}
 function columnLabel(kind='Recorded'){return kind+' ('+zoneLabel()+')';}
 function timeZones(){
  if(!availableZones){let values=[];try{if(typeof Intl.supportedValuesOf==='function')values=Intl.supportedValuesOf('timeZone');}catch{}
   availableZones=[...new Set(['UTC','America/New_York','America/Chicago','America/Denver','America/Los_Angeles','Europe/London','Europe/Paris','Asia/Tokyo','Asia/Kolkata','Australia/Sydney',...values])].sort();
  }return [...availableZones];
 }
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
 // Compare original instants before presentation formatting. Date.parse keeps
 // milliseconds only, so retain the source's remaining RFC3339 nanoseconds.
 function instantParts(value){
  const milliseconds=epoch(value);if(!Number.isFinite(milliseconds))return null;
  const whole=Math.floor(milliseconds),fraction=typeof value==='string'?value.match(/\.(\d{1,9})(?:Z|[+-]\d{2}:\d{2})$/i)?.[1]:null;
  const nanos=typeof value==='string'?Number(fraction?fraction.padEnd(9,'0').slice(3):0):(milliseconds-whole)*1000000;
  const integral=Math.floor(nanos);
  return {nanoseconds:BigInt(whole)*1000000n+BigInt(integral),remainder:nanos-integral};
 }
 function compareInstants(left,right){
  const a=instantParts(left),b=instantParts(right);
  if(!a||!b)return a?-1:b?1:0;
  if(a.nanoseconds!==b.nanoseconds)return a.nanoseconds<b.nanoseconds?-1:1;
  return Math.sign(a.remainder-b.remainder);
 }
 function format(value,{date=true,milliseconds=true,timezone=true,preferences:selectedPreferences}={}){
  const number=epoch(value);if(!Number.isFinite(number))return unknown;
  const p=selectedPreferences===undefined?preferences():normalizePreferences(selectedPreferences),options={hour:'2-digit',minute:'2-digit',second:'2-digit',hourCycle:p.hourFormat==='12'?'h12':'h23'};
  if(date)Object.assign(options,{year:'numeric',month:'2-digit',day:'2-digit'});
  if(milliseconds)options.fractionalSecondDigits=3;
  if(timezone)options.timeZoneName='short';
  const local=settings(),zone=p.zoneMode==='local'?local.timeZone:p.zoneMode==='utc'?'UTC':p.timeZone,key=JSON.stringify([local.locale,zone,p.hourFormat,date,milliseconds,timezone]);let formatter=formatters.get(key);
  if(!formatter){if(zone)options.timeZone=zone;formatter=new Intl.DateTimeFormat(local.locale,options);if(formatters.size>=16)formatters.delete(formatters.keys().next().value);formatters.set(key,formatter);}
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
  const name=columnLabel(snapshot(job)?'Snapshot':'Recorded');
  return {id:'time',name,type:'date',value:row=>recorded(row,job),render:row=>cell(recorded(row,job),{label:name})};
 }
 const api=Object.freeze({epoch,compareInstants,format,cell,value,isKey,recorded,column,snapshot,unknown,defaults,preferences,normalizePreferences,configure,zoneLabel,timeLabel,columnLabel,timeZones,isTimeZone});
 if(typeof module==='object'&&module.exports)module.exports=api;
 if(root)root.NetworkToolkitResultTime=api;
})(typeof window!=='undefined'?window:typeof globalThis!=='undefined'?globalThis:this);
