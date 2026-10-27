(function(root){
 'use strict';
 const defaults=Object.freeze({durationMs:0,anchor:'latest',start:'',end:'',recentMs:60000});
 const esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 let fallback={...defaults},initialized=false;
 const epoch=value=>root.NetworkToolkitResultTime?.epoch?.(value)??(typeof value==='string'&&!/^0001-/.test(value)?Date.parse(value):NaN);
 const compare=(a,b)=>root.NetworkToolkitResultTime?.compareInstants?.(a,b)||Math.sign(epoch(a)-epoch(b));
 const valid=value=>(typeof value==='number'||typeof value==='string'&&value.length<=100)&&Number.isFinite(epoch(value));
 const savedStamp=value=>typeof value==='string'&&valid(value);
 function normalize(value){const v=value&&typeof value==='object'&&!Array.isArray(value)?value:{};const anchor=['latest','end','range'].includes(v.anchor)?v.anchor:'latest',start=savedStamp(v.start)?v.start:'',end=savedStamp(v.end)?v.end:'';return {durationMs:typeof v.durationMs==='number'&&Number.isFinite(v.durationMs)&&v.durationMs>=1000?Math.min(315360000000,Math.round(v.durationMs)):0,anchor:anchor==='latest'||end&&(anchor!=='range'||start&&compare(start,end)<=0)?anchor:'latest',start,end,...(v.allHistory===true?{allHistory:true}:{}),recentMs:typeof v.recentMs==='number'&&Number.isFinite(v.recentMs)?Math.max(1000,Math.min(86400000,Math.round(v.recentMs))):60000};}
 function preferences(){try{return normalize(typeof state!=='undefined'?state.graphTimeRange:fallback);}catch{return normalize(fallback);}}
 function mergePreferences(saved){fallback=normalize(saved?.graphTimeRange);try{if(typeof state!=='undefined')state.graphTimeRange=fallback;}catch{}return {...fallback};}
 function setPreferences(value,{persist=true,fit=false,fitAll=false}={}){fallback=normalize({...preferences(),...value});try{if(typeof state!=='undefined')state.graphTimeRange=fallback;if(persist&&typeof saveState==='function')saveState();}catch{}if(root.document?.dispatchEvent&&typeof root.CustomEvent==='function')root.document.dispatchEvent(new root.CustomEvent('networkToolkitGraphTimeChange',{detail:{...fallback,...(fit?{fit:true}:{}),...(fitAll?{fitAll:true}:{})}}));return {...fallback};}
 function shift(stamp,delta){const ms=epoch(stamp)+delta;if(!Number.isFinite(ms)||!Number.isFinite(new Date(ms).getTime()))return '';const nano=String(stamp).match(/\.(\d{1,9})(?:Z|[+-]\d\d:\d\d)$/i)?.[1]?.padEnd(9,'0');return new Date(ms).toISOString().replace(/\.(\d{3})Z$/,(_,m)=>'.'+m+(nano? nano.slice(3):'')+'Z');}
 function range(items,pref=preferences(),heldEnd='',getStamp=item=>item.time){
  const stamps=items.map(getStamp).filter(valid).sort(compare),first=stamps[0]??'',last=stamps.at(-1)??'',end=pref.anchor==='latest'?(valid(heldEnd)?heldEnd:last):pref.end,start=pref.anchor==='range'?pref.start:pref.durationMs&&valid(end)?shift(end,-pref.durationMs):first;
  return {start,end,from:valid(start)?epoch(start):-Infinity,to:valid(end)?epoch(end):Infinity,enabled:pref.durationMs>0||pref.anchor!=='latest',unknown:items.length-stamps.length,first,last};
 }
 function contains(stamp,scope){return valid(stamp)&&(!valid(scope.start)||compare(stamp,scope.start)>=0)&&(!valid(scope.end)||compare(stamp,scope.end)<=0);}
 function select(items,scope,getStamp=item=>item.time){return items.filter(item=>contains(getStamp(item),scope)).sort((a,b)=>compare(getStamp(a),getStamp(b)));}
 function condense(items,{end,recentMs=60000,preserveRaw=false,getStamp=item=>item.time}={}){
  const sorted=items.filter(item=>valid(getStamp(item))).slice().sort((a,b)=>compare(getStamp(a),getStamp(b))),last=epoch(end??getStamp(sorted.at(-1)||{})),bins=new Map(),result=[],gaps=[];
  // Modest visible histories need no aggregation: preserve their true time spacing.
  if(preserveRaw&&sorted.length<=1400)return sorted.map(item=>({items:[item],aggregate:false,start:getStamp(item),end:getStamp(item),count:1}));
  for(let i=Math.max(1,sorted.length-256);i<sorted.length;i++){const gap=Number(instant(getStamp(sorted[i]))-instant(getStamp(sorted[i-1])))/1000000;if(gap>0)gaps.push(gap);}gaps.sort((a,b)=>a-b);
  const cadence=gaps[Math.floor(gaps.length/2)]||recentMs/60,oldest=Math.max(1,(last-epoch(getStamp(sorted[0]||{})))/recentMs),density=Math.min(120,Math.max(1,recentMs/cadence),1400/Math.max(1,2*(Math.sqrt(oldest)-1)));
  // Bin widths grow with the square root of age, more slowly than the time
  // projection compresses it. Older displayed groups therefore get denser to
  // the left, instead of turning into an evenly spaced row of averages.
  for(const item of sorted){const age=Math.max(0,last-epoch(getStamp(item)));if(age<recentMs){result.push({items:[item],aggregate:false});continue;}const key=String(Math.floor(2*density*(Math.sqrt(Math.max(1,age/recentMs))-1)));let group=bins.get(key);if(!group){group={items:[],aggregate:true};bins.set(key,group);result.push(group);}group.items.push(item);}
  return result.sort((a,b)=>compare(getStamp(a.items[0]),getStamp(b.items[0]))).map(group=>({...group,aggregate:group.items.length>1,start:getStamp(group.items[0]),end:getStamp(group.items.at(-1)),count:group.items.length}));
 }
 function parseTime(value){
  const text=String(value||'').trim();if(valid(text))return text;
  const match=text.match(/^(\d{4})-(\d\d)-(\d\d)[T ](\d{1,2}):(\d\d)(?::(\d\d)(?:\.(\d{1,9}))?)?\s*(AM|PM)?$/i);if(!match)throw Error('Enter a date and time, or an ISO timestamp with its UTC offset.');
  let [year,month,day,hour,minute,second]=match.slice(1,7).map(v=>Number(v||0));if(match[8]){if(hour<1||hour>12)throw Error('Use an hour from 1 through 12 with AM or PM.');hour=hour%12+(match[8].toUpperCase()==='PM'?12:0);}
  const fraction=(match[7]||'').padEnd(9,'0'),wall=String(year).padStart(4,'0')+'-'+String(month).padStart(2,'0')+'-'+String(day).padStart(2,'0')+'T'+String(hour).padStart(2,'0')+':'+String(minute).padStart(2,'0')+':'+String(second).padStart(2,'0')+'.'+fraction+'Z';if(!valid(wall))throw Error('Choose a valid calendar date and time.');
  const p=root.NetworkToolkitResultTime?.preferences?.()||{},zone=p.zoneMode==='utc'?'UTC':p.zoneMode==='named'?p.timeZone:new Intl.DateTimeFormat().resolvedOptions().timeZone||'UTC',formatter=new Intl.DateTimeFormat('en-GB',{timeZone:zone,year:'numeric',month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',hourCycle:'h23'}),target=epoch(wall),matches=new Set();
  const parts=value=>Object.fromEntries(formatter.formatToParts(new Date(value)).filter(p=>p.type!=='literal').map(p=>[p.type,Number(p.value)]));
  for(let offset=-36;offset<=36;offset+=6){const sample=target+offset*3600000,q=parts(sample),asUTC=Date.UTC(q.year,q.month-1,q.day,q.hour,q.minute,q.second),candidate=target-(asUTC-Math.floor(sample/1000)*1000),r=parts(candidate);if(r.year===year&&r.month===month&&r.day===day&&r.hour===hour&&r.minute===minute&&r.second===second)matches.add(candidate);}
  if(matches.size!==1)throw Error(matches.size?'This local time occurs twice. Enter its ISO timestamp with an explicit UTC offset.':'This local time does not exist in the configured timezone.');return new Date([...matches][0]).toISOString().replace(/\.\d{3}Z$/,'.'+fraction+'Z');
 }
 function label(pref=preferences()){const duration=pref.durationMs?pref.durationMs%3600000===0?pref.durationMs/3600000+' h':pref.durationMs%60000===0?pref.durationMs/60000+' min':pref.durationMs/1000+' s':'All retained time';return duration+(pref.anchor==='latest'?' · Latest':pref.anchor==='range'?' · Custom interval':' · Chosen end time');}
 const presets=Object.freeze([[10000,'10s'],[30000,'30s'],[60000,'1m'],[300000,'5m'],[900000,'15m'],[3600000,'1h'],[21600000,'6h'],[86400000,'24h'],[0,'All']]);
 function controlHTML(){const p=preferences(),custom=p.anchor==='range'||!presets.some(([n])=>n===p.durationMs);return '<label class="graph-timeframe"><span>Timeframe</span><select data-graph-time-preset aria-label="Graph timeframe" title="Choose a displayed duration; All means all retained history">'+presets.map(([n,label])=>'<option value="'+n+'"'+(!custom&&p.durationMs===n?' selected':'')+'>'+label+'</option>').join('')+(custom?'<option value="custom" selected>Custom</option>':'')+'</select></label><button type="button" class="btn small ghost graph-range-advanced" data-graph-time-open title="Custom duration and historical date/time range">Range…</button>';}
 function setDuration(value){const n=Number(value),p=preferences();if(!presets.some(([duration])=>duration===n))return false;setPreferences({durationMs:n,allHistory:n===0,...(p.anchor==='range'?{anchor:'end'}:{})},{fit:true});return true;}
 function zoomLevel(value,minimum=.25,maximum=16){return Math.max(minimum,Math.min(maximum,Math.round((Number(value)||1)*1000)/1000));}
 function wheelLevel(event,current,minimum=.25,maximum=16){if(event.ctrlKey||event.metaKey||event.altKey||!Number.isFinite(event.deltaY)||event.deltaY===0)return current;const pixels=event.deltaY*(event.deltaMode===1?16:event.deltaMode===2?160:1);return zoomLevel(current*Math.pow(2,-Math.max(-240,Math.min(240,pixels))/350),minimum,maximum);}
 function anchoredEnd(scope,oldZoom,nextZoom,share,mode='linear'){
  const a=instant(scope?.start),b=instant(scope?.end);if(a===null||b===null||b<=a)return '';
  const p=Math.max(0,Math.min(1,Number(share)||0)),anchor=instant(at(p,scope,mode)),span=Math.max(1,Math.round(Number(b-a)*oldZoom/nextZoom)),age=mode==='condensed'?(scope.legacyCondensed?condensedAge(1-p,span,preferences().recentMs):clusterAge(1-p,clusterHorizon(scope)*oldZoom/nextZoom)):(1-p)*span;
  return timestamp(anchor+BigInt(Math.round(age)));
 }
 // Main-plot dragging keeps the grabbed timestamp under the pointer. The
 // overview and keyboard pans deliberately retain their linear time arithmetic.
 function panScope(scope,fromShare,toShare,mode='linear'){
  const a=instant(scope?.start),b=instant(scope?.end),from=instant(at(fromShare,scope,mode)),to=instant(at(toShare,scope,mode));if(a===null||b===null||from===null||to===null)return scope;
  const start=timestamp(a+from-to),end=timestamp(b+from-to);return {...scope,start,end,from:epoch(start),to:epoch(end)};
 }
 function valueScale(values,{whole=false}={}){
  const measured=values.filter(value=>typeof value==='number'&&Number.isFinite(value)&&value>=0);if(!measured.length)return {minimum:null,maximum:null,lower:0,upper:1,empty:true};
  const minimum=Math.min(...measured),maximum=Math.max(...measured),span=maximum-minimum,margin=span>0?span*.08:Math.max(Math.abs(maximum)*.05,maximum===0?.1:Math.min(.001,maximum*.1));let lower=Math.max(0,minimum-margin),upper=maximum+margin;
  if(whole){lower=Math.max(0,Math.floor(lower));upper=Math.ceil(upper);}if(!(upper>lower)){lower=Math.max(0,minimum-.1);upper=maximum+.1;}return {minimum,maximum,lower,upper,empty:false};
 }
 function scaleLabel(value,digits=4){const standard=new Intl.NumberFormat(undefined,{maximumSignificantDigits:digits}).format(value),scientific=new Intl.NumberFormat(undefined,{maximumSignificantDigits:digits,notation:'scientific'}).format(value);return standard.length>8&&scientific.length<standard.length?scientific:standard;}
 function scaleLabels(scale){let digits=4,labels;do{labels=[scaleLabel(scale.upper,digits),scaleLabel(scale.lower,digits)];digits++;}while(labels[0]===labels[1]&&scale.upper!==scale.lower&&digits<=17);return scale.empty?['—','—']:labels;}
 // Plot coordinates are derived from recorded instants, never sample indices.
 // Keep nanoseconds through projection so capture intervals do not lose precision.
 function instant(value){const ms=epoch(value);if(!Number.isFinite(ms))return null;const fraction=typeof value==='string'?value.match(/\.(\d{1,9})(?:Z|[+-]\d\d:\d\d)$/i)?.[1]:null;return BigInt(Math.floor(ms))*1000000n+BigInt(fraction?fraction.padEnd(9,'0').slice(3):Math.round((ms-Math.floor(ms))*1000000));}
 function timestamp(ns){if(typeof ns!=='bigint')return '';let seconds=ns/1000000000n,part=ns%1000000000n;if(part<0){seconds--;part+=1000000000n;}try{return new Date(Number(seconds*1000n)).toISOString().replace(/\.\d{3}Z$/,'.'+String(part).padStart(9,'0')+'Z');}catch{return '';}}
 function viewport(scope,zoom=1,end=''){let a=instant(scope.start),b=instant(scope.end);if(a===null||b===null)return scope;if(a===b&&Number(zoom)===1&&(!end||instant(end)===b)){const stamp=timestamp(a);return {...scope,start:stamp,end:stamp,from:epoch(stamp),to:epoch(stamp)};}if(b<=a)a=b-1000000000n;const span=BigInt(Math.max(1,Math.round(Number(b-a)/Math.max(1e-18,Number(zoom)||1))));b=instant(end)??b;a=b-span;const start=timestamp(a),finish=timestamp(b);return {...scope,...(scope.condensedMs?{condensedMs:scope.condensedMs/Math.max(1e-18,Number(zoom)||1)}:{}),start,end:finish,from:epoch(start),to:epoch(finish)};}
 // Keep the latest configured interval in the right half at a fixed time
 // resolution. Only the older half changes curvature as history grows. Before
 // three recent intervals exist, Fit fills the plot with a continuous warm-up.
 // The two logarithmic regions share both position and slope at age=recent.
 const condensedCurves=new Map(),logTwo=Math.log(2),logFour=2*logTwo;
 function condensedCurve(span,recentMs){
  const recent=Math.max(1,Number(recentMs)||60000)*1000000,key=span+':'+recent;
  if(condensedCurves.has(key))return condensedCurves.get(key);
  let curve;
  if(span<=3*recent)curve={recent:span/3,warm:true};
  else{
   const older=span/recent-1;let low=0,high=2;
   // beta*log(1+older/beta)=2*log(2) fixes the tail's initial slope.
   // For spans>=3*recent the unique positive solution lies in (0,2].
   for(let i=0;i<52;i++){const beta=(low+high)/2;if(beta*Math.log1p(older/beta)<logFour)low=beta;else high=beta;}
   curve={recent,beta:(low+high)/2,warm:false};
  }
  if(condensedCurves.size>=64)condensedCurves.delete(condensedCurves.keys().next().value);condensedCurves.set(key,curve);return curve;
 }
 function condensedDistance(age,span,recentMs){
  const curve=condensedCurve(span,recentMs),u=Math.max(0,age)/curve.recent;
  if(curve.warm)return Math.log1p(u)/logFour;
  return u<=1?Math.log1p(u)/logFour:.5+curve.beta*Math.log1p((u-1)/curve.beta)/(2*logFour);
 }
 function condensedAge(distance,span,recentMs){
  if(distance<=0)return 0;if(distance>=1)return span;
  const curve=condensedCurve(span,recentMs);
  return curve.recent*(curve.warm||distance<=.5?Math.expm1(distance*logFour):1+curve.beta*Math.expm1((distance-.5)*2*logFour/curve.beta));
 }
 // RTT Condensed uses a fixed finite age domain, never the recording span.
 // The oldest age maps to one aggregate at x=0. Sparse startup leaves space.
 function clusterHorizon(scope,recentMs=preferences().recentMs){return Math.max(.000001,Number(scope?.condensedMs)||3*recentMs)*1000000;}
 function clusterAge(distance,horizon){return horizon*Math.expm1(Math.max(0,Math.min(1,distance))*logFour)/3;}
 function fraction(value,scope,mode='linear',recentMs=preferences().recentMs){const a=instant(scope?.start),b=instant(scope?.end),t=instant(value);if(a===null||b===null||t===null||b<a)return NaN;if(mode==='condensed'&&!scope.legacyCondensed)return 1-Math.log1p(Math.max(0,Math.min(clusterHorizon(scope,recentMs),Number(b-t)))*3/clusterHorizon(scope,recentMs))/logFour;if(b===a)return t===a?.5:t<a?-Infinity:Infinity;const span=Number(b-a),share=Number(t-a)/span;if(mode!=='condensed')return share;if(t===a)return 0;if(t===b)return 1;return 1-condensedDistance(Number(b-t),span,recentMs);}
 function at(share,scope,mode='linear',recentMs=preferences().recentMs){const a=instant(scope?.start),b=instant(scope?.end);if(a===null||b===null||b<a)return '';const p=Math.max(0,Math.min(1,share));if(mode==='condensed'&&!scope.legacyCondensed)return timestamp(b-BigInt(Math.round(clusterAge(1-p,clusterHorizon(scope,recentMs)))));if(b===a)return timestamp(a);const span=b-a;if(mode==='condensed'&&p>0&&p<1){const age=condensedAge(1-p,Number(span),recentMs);return timestamp(b-BigInt(Math.max(0,Math.min(Number(span),Math.round(age)))));}const scale=1000000000000n;return timestamp(a+(span*BigInt(Math.round(p*Number(scale)))+scale/2n)/scale);}
 function axisHTML(scope,width=820,mode='linear'){
  if(instant(scope?.start)===null||instant(scope?.end)===null)return '<span class="rt-axis-unrecorded">Recorded time unavailable</span>';
  if(mode!=='condensed'&&instant(scope.start)===instant(scope.end))return '<time class="rt-time-tick" style="left:50%;transform:translateX(-50%)" datetime="'+esc(scope.start)+'" title="'+esc(scope.start)+'">'+esc(root.NetworkToolkitResultTime?.format?.(scope.start,{milliseconds:true})||scope.start)+'</time>';
  const span=mode==='condensed'&&!scope.legacyCondensed?clusterHorizon(scope)/1000000:Number(instant(scope.end)-instant(scope.start))/1000000,p=root.NetworkToolkitResultTime?.preferences?.()||{},date=span>=86400000,minLabel=date?150:span<10000?125:p.hourFormat==='12'?110:85;let ms=span<10000,nano=span<1,timezone=false;let count=Math.max(2,Math.min(8,Math.floor(Math.max(1,width)/minLabel)+1));
  try{const zone=p.zoneMode==='utc'?'UTC':p.zoneMode==='named'?p.timeZone:undefined,formatter=new Intl.DateTimeFormat('en-US',{timeZone:zone,timeZoneName:'shortOffset'}),offset=stamp=>formatter.formatToParts(new Date(epoch(stamp))).find(part=>part.type==='timeZoneName')?.value;timezone=offset(scope.start)!==offset(scope.end);}catch{}
  const formatted=stamp=>root.NetworkToolkitResultTime?.format?.(stamp,{date,milliseconds:ms,timezone})||stamp;
  let labels=Array.from({length:count},(_,i)=>formatted(at(i/(count-1),scope,mode)));if(new Set(labels).size<labels.length)ms=true;
  const estimated=Math.max(...[scope.start,at(.5,scope,mode),scope.end].map(stamp=>formatted(stamp).length+(nano?6:0)))*6.2;while(count>2&&width/(count-1)<estimated*1.55+8)count--;
  return Array.from({length:count},(_,i)=>{const share=i/(count-1),stamp=at(share,scope,mode);let label=formatted(stamp);if(mode==='condensed'&&!scope.legacyCondensed&&i===0&&scope.historyCluster)label='History';else if(mode==='condensed'&&!scope.legacyCondensed&&i===0&&instant(scope.start)<=instant(stamp))label='≤ '+label;if(nano)label=label.replace(/([.,])\d{3}/,(_,dot)=>dot+stamp.match(/\.(\d{9})Z$/)?.[1]);return '<time class="rt-time-tick" style="left:'+share*100+'%" datetime="'+esc(stamp)+'" title="'+esc((root.NetworkToolkitResultTime?.format?.(stamp)||stamp)+' | '+stamp+' · Axis position, not an additional measurement')+'">'+esc(label)+'</time>';}).join('');
 }
 function open(){if(typeof openModal!=='function')return;const p=preferences(),zone=root.NetworkToolkitResultTime?.zoneLabel?.()||'local';openModal('Graph time range','<form id="graphTimeForm" class="graph-time-form"><p>Dates without an offset use '+esc(zone)+'. ISO timestamps with an explicit offset are also accepted. Ambiguous daylight-saving times require an offset.</p><label>Time range<select id="graphTimeDuration">'+[[0,'All retained time'],[10000,'10 seconds'],[30000,'30 seconds'],[60000,'1 minute'],[300000,'5 minutes'],[900000,'15 minutes'],[3600000,'1 hour'],[21600000,'6 hours'],[86400000,'24 hours'],[-1,'Custom duration']].map(([value,name])=>'<option value="'+value+'"'+((value===p.durationMs||value===-1&&![0,10000,30000,60000,300000,900000,3600000,21600000,86400000].includes(p.durationMs))?' selected':'')+'>'+name+'</option>').join('')+'</select></label><label>Custom duration (seconds)<input id="graphTimeSeconds" type="number" min="1" max="315360000" value="'+(p.durationMs/1000||300)+'"></label><label>Position<select id="graphTimeAnchor">'+[['latest','Follow latest'],['end','End at a chosen date/time'],['range','Explicit start and end']].map(([value,name])=>'<option value="'+value+'"'+(p.anchor===value?' selected':'')+'>'+name+'</option>').join('')+'</select></label><label>Start date/time<input id="graphTimeStart" type="text" value="'+esc(p.start)+'" placeholder="2026-10-26 14:30:00"></label><label>End date/time<input id="graphTimeEnd" type="text" value="'+esc(p.end)+'" placeholder="2026-10-26 15:30:00"></label><label>Condensed history: keep individual results for the latest (seconds)<input id="graphTimeRecent" type="number" min="1" max="86400" value="'+p.recentMs/1000+'"></label><p id="graphTimeError" role="alert"></p><p>These display settings do not remove recorded results or change list filters.</p></form>','<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button type="button" class="btn primary" data-graph-time-apply>Apply</button>',true);updateForm();}
 function updateForm(){const d=root.document,anchor=d?.getElementById('graphTimeAnchor')?.value,duration=d?.getElementById('graphTimeDuration')?.value;for(const [id,disabled] of [['graphTimeSeconds',duration!=='-1'||anchor==='range'],['graphTimeStart',anchor!=='range'],['graphTimeEnd',anchor==='latest'],['graphTimeDuration',anchor==='range']]){const input=d?.getElementById(id);if(input)input.disabled=disabled;}}
 function initialize(){if(initialized||!root.document?.addEventListener)return;initialized=true;root.document.addEventListener('click',event=>{if(event.target.closest?.('[data-graph-time-open]')){event.preventDefault();open();}if(event.target.closest?.('[data-graph-time-apply]')){event.preventDefault();const d=root.document,get=id=>d.getElementById(id).value;try{const anchor=get('graphTimeAnchor'),duration=Number(get('graphTimeDuration')),seconds=Number(get('graphTimeSeconds')),recent=Number(get('graphTimeRecent'));if(duration===-1&&(!(seconds>=1)||seconds>315360000)||!(recent>=1&&recent<=86400))throw Error('Choose a valid duration in seconds.');const start=anchor==='range'?parseTime(get('graphTimeStart')):'',end=anchor!=='latest'?parseTime(get('graphTimeEnd')):'';if(start&&compare(start,end)>0)throw Error('The start must not be after the end.');setPreferences({anchor,start,end,allHistory:anchor==='latest'&&duration===0,durationMs:duration===-1?seconds*1000:duration,recentMs:recent*1000});if(typeof closeModal==='function')closeModal();}catch(error){d.getElementById('graphTimeError').textContent=error.message;}}});root.document.addEventListener('change',event=>{if(event.target.matches?.('[data-graph-time-preset]')){if(event.target.value==='custom')open();else setDuration(event.target.value);return;}if(['graphTimeAnchor','graphTimeDuration'].includes(event.target.id))updateForm();});}
 const api=Object.freeze({defaults,normalize,preferences,mergePreferences,setPreferences,epoch,compare,range,contains,select,condense,clusterHorizon,shift,parseTime,label,controlHTML,presets,setDuration,zoomLevel,wheelLevel,anchoredEnd,panScope,valueScale,scaleLabel,scaleLabels,instant,timestamp,viewport,fraction,at,axisHTML,initialize});if(typeof module==='object'&&module.exports)module.exports=api;root.NetworkToolkitGraphHistory=api;initialize();
})(typeof window!=='undefined'?window:globalThis);
