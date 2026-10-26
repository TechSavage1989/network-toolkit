(function(root){
 'use strict';
 const configs=new Map(),jobs=new Map(),contexts=new WeakMap(),fallback=Object.create(null),statistics=new Set(['sent','received','loss','minimum','average','maximum','jitter']);
 const names={sent:'All samples',received:'Successful replies',loss:'Lost or unsuccessful samples',minimum:'Minimum RTT',average:'Average RTT contributors',maximum:'Maximum RTT',jitter:'RTT variability contributors'};
 const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 const token=value=>String(value??'').trim().toLowerCase().replace(/[\s-]+/g,'_').slice(0,100);
 const statusToken=value=>{const v=token(value);return ['timed_out','timedout'].includes(v)?'timeout':v==='no_reply'?'no_response':v;};
 function appState(){try{return typeof state==='object'&&state?state:root.state&&typeof root.state==='object'?root.state:null;}catch{return null;}}
 function safeView(value){return typeof value==='string'&&value.length>0&&value.length<=240&&!['__proto__','constructor','prototype'].includes(value);}
 function normalize(value){const v=value&&typeof value==='object'&&!Array.isArray(value)?value:{};return {status:typeof v.status==='string'&&v.status?statusToken(v.status)||'all':'all',type:typeof v.type==='string'&&v.type?token(v.type)||'all':'all',statistic:statistics.has(v.statistic)?v.statistic:''};}
 function preferences(view){return normalize((appState()?.resultFilters||fallback)[view]);}
 function mergePreferences(saved){const target=appState(),next=Object.create(null),input=saved?.resultFilters;if(input&&typeof input==='object'&&!Array.isArray(input))for(const [view,value]of Object.entries(input).slice(-160))if(safeView(view))next[view]=normalize(value);if(target)target.resultFilters=next;else{for(const key of Object.keys(fallback))delete fallback[key];Object.assign(fallback,next);}}
 function eligible(config){return !!config?.job&&Array.isArray(config.rows)&&(!config.card||config.job.tool==='quick')&&Array.isArray(config.columns);}
 function register(config){if(!eligible(config))return config;configs.delete(config.id);configs.set(config.id,config);while(configs.size>240)configs.delete(configs.keys().next().value);const id=String(config.job.id??''),prior=jobs.get(id);if(id&&(!prior||config.graphId||config.name==='samples'||!prior.graphId&&prior.name!=='samples')){jobs.delete(id);jobs.set(id,config);while(jobs.size>128)jobs.delete(jobs.keys().next().value);}return config;}
 function status(row,tool=''){
  if(!row||typeof row!=='object')return 'recorded';
  let value=row.status??row.state??row.result??row.level??row.outcome;
  if(typeof value==='number'&&tool==='http')return 'http_'+value;
  if(value!==undefined&&value!==null&&typeof value!=='object'&&String(value).trim())return statusToken(value);
  if(typeof row.success==='boolean')return row.success?'reply':'unsuccessful';
  if(typeof row.responded==='boolean')return row.responded?'responded':'no_response';
  if(typeof row.valid==='boolean')return row.valid?'valid':'invalid';
  if(row.error||row.errorCode)return 'error';
  if(tool==='http'&&Number.isInteger(row.code))return 'http_'+row.code;
  return 'recorded';
 }
 function severity(row,tool=''){
  const value=status(row,tool);
  if(tool==='trace'&&row?.reached===true)return 'good';
  if(tool==='trace'&&row?.address&&['ttl_expired','reply'].includes(value))return 'info';
  if(/^(?:warn|warning|partial|cancelled|canceled|unknown|pending|queued|running|in_progress)$/.test(value))return ['unknown','pending','queued','running','in_progress'].includes(value)?'info':'warn';
  if(row?.error||/^(?:fail|failed|failure|error|closed|invalid|unreachable|unavailable|no_response|timeout|unsuccessful|refused|denied|blocked|offline|down|expired)$/.test(value)||/^http_[45]\d\d$/.test(value))return 'bad';
  if(row?.success===true||row?.responded===true||row?.valid===true||/^(?:reply|success|successful|passed|open|reachable|valid|ok|responded|completed|sent|up)$/.test(value)||/^http_2\d\d$/.test(value))return 'good';
  if(row?.success===false||row?.responded===false)return 'bad';
  return 'info';
 }
 function pill(row,tool,text){return '<span class="pill '+severity(row,tool)+'">'+escape(text??label(status(row,tool)))+'</span>';}
 function type(row,config={}){
  const tool=config.job?.tool||'',v=row?.protocol??row?.recordType??row?.type??row?.method;
  if(v!==undefined&&v!==null&&typeof v!=='object'&&String(v).trim())return token(v);
  if(['ping','monitor','trace'].includes(tool))return token(config.job?.options?.mode||'icmp');
  if(tool==='tcp')return 'tcp';
  if(tool==='dns')return token(config.job?.options?.recordType||'dns');
  if(tool==='smtp')return token(row?.stage||row?.step||'smtp');
  if(tool==='http')return /header/i.test(config.name||'')?'http_header':'http';
  if(tool==='tls')return 'certificate';
  if(tool==='quick')return token(row?.check||row?.name||'check');
  return '';
 }
 function label(value){if(/^http_\d+$/.test(value))return 'HTTP '+value.slice(5);if(['icmp','tcp','udp','dns','smtp','http','https','tls','a','aaaa','mx','txt','soa','ns','srv','ptr','cname','caa','arp'].includes(value))return value.toUpperCase();return value.replace(/_/g,' ').replace(/^./,c=>c.toUpperCase());}
 function successful(row,tool){const v=status(row,tool);if(row?.success===false||row?.responded===false||row?.valid===false||row?.error)return false;return row?.success===true||row?.responded===true||row?.valid===true||['reply','success','successful','ok','passed','responded','reachable','open','valid'].includes(v)||/^http_2\d\d$/.test(v);}
 function measurement(row,tool){const raw=row?.rttMs??row?.latencyMs??row?.responseTimeMs;if(!successful(row,tool)||raw===null||raw===undefined||raw===''||typeof raw==='boolean')return null;const value=Number(raw);return Number.isFinite(value)&&value>=0?value:null;}
 function identity(row){if(!row||typeof row!=='object')return '';const time=row.time??row.timestamp??row.observedAt??row.recordedAt;if(row.sequence===undefined&&time===undefined)return '';return JSON.stringify([row.sequence??'',time??'',row.address??'']);}
 function context(config){
  let cached=contexts.get(config);if(cached)return cached;
  const rows=config.rows,tool=config.job?.tool||'',measured=rows.map(row=>measurement(row,tool)),values=measured.filter(value=>value!==null),pairs=new Set(),pairKeys=new Set();let previous=-1;
  for(let i=0;i<rows.length;i++){if(measured[i]===null){previous=-1;continue;}if(previous>=0){const before=Number(rows[previous].sequence),after=Number(rows[i].sequence),consecutive=rows[previous].sequence==null||rows[i].sequence==null||Number.isFinite(before)&&Number.isFinite(after)&&after===before+1;if(consecutive){pairs.add(rows[previous]);pairs.add(rows[i]);for(const row of [rows[previous],rows[i]]){const key=identity(row);if(key)pairKeys.add(key);}}}previous=i;}
  const stats=config.statistics||config.job?.result?.data?.statistics||config.job?.sampleStatistics||{},retained={minMs:values.length?Math.min(...values):null,maxMs:values.length?Math.max(...values):null,avgMs:values.length?values.reduce((a,b)=>a+b,0)/values.length:null};
  const metric=key=>{if(Object.prototype.hasOwnProperty.call(stats,key)){const raw=stats[key];return {value:raw!==null&&raw!==undefined&&Number.isFinite(Number(raw))?Number(raw):null,kind:'run'};}return {value:retained[key]??null,kind:'retained'};};
  cached={measured,pairs,pairKeys,metric,values};contexts.set(config,cached);return cached;
 }
 function statisticMatches(row,config,statistic){
  if(!statistic||statistic==='sent')return true;
  const tool=config.job?.tool||'';
  if(statistic==='received')return successful(row,tool);
  if(statistic==='loss')return !successful(row,tool)&&(row?.success===false||row?.responded===false||severity(row,tool)==='bad');
  const ctx=context(config),value=measurement(row,tool);
  if(statistic==='average')return value!==null;
  if(statistic==='jitter')return ctx.pairs.has(row)||!!identity(row)&&ctx.pairKeys.has(identity(row));
  const ref=ctx.metric(statistic==='minimum'?'minMs':'maxMs').value;return ref!==null&&value!==null&&value===ref;
 }
 function matches(row,config){if(!eligible(config))return true;const p=preferences(config.view),tool=config.job?.tool||'';return (p.status==='all'||status(row,tool)===p.status)&&(p.type==='all'||type(row,config)===p.type)&&statisticMatches(row,config,p.statistic);}
 function legacyMatches(row,config){let p={};try{if(typeof listPreference==='function')p=listPreference(config.view,config.defaults);}catch{}const query=String(p.query||'').trim().toLowerCase();return (!query||String(config.search?config.search(row):config.columns.map(column=>column.value(row)).join(' ')).toLowerCase().includes(query))&&(!config.status||p.status===undefined||p.status==='all'||String(config.status(row))===p.status);}
 function graphMatches(id,row){const config=jobs.get(String(id));return !config||matches(row,config)&&legacyMatches(row,config);}
 function current(id){
  const config=jobs.get(String(id));if(!config)return {status:'all',type:'all',statistic:'',active:false,reference:null,referenceKind:'retained',message:'',jobId:String(id),view:''};
  const p=preferences(config.view),ctx=context(config),key={minimum:'minMs',maximum:'maxMs',average:'avgMs',jitter:'jitterMs'}[p.statistic],ref=key?ctx.metric(key):{value:null,kind:'retained'};let query='',legacyStatus='all';try{if(typeof listPreference==='function'){const legacy=listPreference(config.view,config.defaults);query=legacy.query||'';legacyStatus=legacy.status||'all';}}catch{}
  let message=p.statistic==='average'?'Measured replies contribute to the '+ref.kind+' average; the value is an aggregate, not an individual sample.':p.statistic==='jitter'?(ctx.pairs.size?'Shown samples form consecutive measured reply pairs in retained history. Missing, failed or skipped samples break a pair.':'No consecutive measured reply pairs are present in retained history. Missing, failed or skipped samples break a pair.'):'';
  if(['minimum','maximum','average'].includes(p.statistic)&&ref.value===null)message='No measured replies are available for this statistic.';
  if(['minimum','maximum'].includes(p.statistic)&&ref.value!==null&&!config.rows.some(row=>measurement(row,config.job.tool)===ref.value))message='The '+ref.kind+' '+p.statistic+' is not present in retained samples. No different measurement is substituted.';
  return {...p,active:p.status!=='all'||p.type!=='all'||!!p.statistic||!!query||legacyStatus!=='all',reference:ref.value,referenceKind:ref.kind,message,jobId:String(config.job.id),view:config.view};
 }
 function choices(config,kind){const values=new Set(config.rows.map(row=>kind==='status'?status(row,config.job.tool):type(row,config)).filter(Boolean));return [...values].sort((a,b)=>label(a).localeCompare(label(b)));}
 function controls(config){
  if(!eligible(config))return '';const p=preferences(config.view),statuses=choices(config,'status'),types=choices(config,'type');if(!statuses.length&&!types.length&&!p.statistic&&p.status==='all'&&p.type==='all')return '';
  const select=(kind,values,selected)=>'<label><span>'+ (kind==='status'?'Status':'Type')+'</span><select data-result-filter-'+kind+'="'+escape(config.id)+'" aria-label="Filter '+escape(config.name||'results')+' by detailed '+kind+'"><option value="all"'+(selected==='all'?' selected':'')+'>All '+(kind==='status'?'statuses':'types')+'</option>'+[...new Set([...values,...(selected!=='all'?[selected]:[])])].map(value=>'<option value="'+escape(value)+'"'+(value===selected?' selected':'')+'>'+escape(label(value)+(values.includes(value)?'':' (not in current results)'))+'</option>').join('')+'</select></label>';
  const active=[p.status!=='all'?'Status: '+label(p.status):'',p.type!=='all'?'Type: '+label(p.type):'',p.statistic?names[p.statistic]:''].filter(Boolean),message=current(config.job.id).message;
  return '<div class="result-filter-controls" data-result-filter-controls="'+escape(config.id)+'">'+select('status',statuses,p.status)+(types.length>1||p.type!=='all'?select('type',types,p.type):'')+(active.length?'<div class="result-filter-summary" data-result-filter-summary="'+escape(config.id)+'"><span>'+escape(active.join(' · '))+'</span><button type="button" class="btn small ghost" data-result-filter-clear="'+escape(config.id)+'">Clear filters</button></div>':'')+(message?'<p class="result-filter-note">'+escape(message)+'</p>':'')+'</div>';
 }
 function statusSummary(id,entries){const selected=current(id);return '<div class="result-status-summary" data-result-status-summary="'+escape(id)+'"><div class="result-stats">'+entries.map(([key,name,value,color])=>'<button type="button" class="result-stat result-status-stat" data-result-status-stat="'+escape(key)+'" data-result-stat-job="'+escape(id)+'" aria-pressed="'+(!selected.statistic&&selected.status===key)+'" title="Show '+escape(key==='all'?'all checked ports':name.toLowerCase()+' port results')+' within the current search and type filters"><strong class="result-status-'+escape(color||'info')+'">'+escape(value)+'</strong><span>'+escape(name)+'</span></button>').join('')+'</div><div class="result-status-context" data-result-status-context>'+statusContext(selected)+'</div></div>';}
 function statusContext(selected){return selected.status!=='all'||selected.statistic?'<span>Status summary: '+escape(selected.statistic?names[selected.statistic]:label(selected.status))+'</span><button type="button" class="btn small ghost" data-result-status-clear data-result-stat-job="'+escape(selected.jobId)+'">Clear status filter</button>':'';}
 function statisticSummary(id,entries){const selected=current(id);return '<div class="result-status-summary" data-result-statistic-summary="'+escape(id)+'"><div class="result-stats">'+entries.map(([key,name,value,color])=>'<button type="button" class="result-stat result-status-stat" data-result-stat="'+escape(key)+'" data-result-stat-job="'+escape(id)+'" aria-pressed="'+(selected.statistic===key)+'"><strong class="result-status-'+escape(color||'info')+'">'+escape(value)+'</strong><span>'+escape(name)+'</span></button>').join('')+'</div><div class="result-status-context" data-result-statistic-context>'+statisticContext(selected)+'</div></div>';}
 function statisticContext(selected){return selected.statistic?'<span>'+escape(selected.message||names[selected.statistic])+'</span><button type="button" class="btn small ghost" data-result-stat-clear data-result-stat-job="'+escape(selected.jobId)+'">Clear statistics filter</button>':'';}
 function updateStatusSummaries(config){if(!config?.job||!root.document?.querySelectorAll)return;const selected=current(config.job.id);for(const area of root.document.querySelectorAll('[data-result-status-summary]'))if(area.dataset.resultStatusSummary===String(config.job.id)){for(const button of area.querySelectorAll('[data-result-status-stat]'))button.setAttribute('aria-pressed',String(!selected.statistic&&button.dataset.resultStatusStat===selected.status));const context=area.querySelector('[data-result-status-context]');if(context){const html=statusContext(selected);if(typeof patchResultHTML==='function')patchResultHTML(context,html);else context.innerHTML=html;}}}
 function updateStatisticSummaries(config){if(!config?.job||!root.document?.querySelectorAll)return;const selected=current(config.job.id);for(const area of root.document.querySelectorAll('[data-result-statistic-summary]'))if(area.dataset.resultStatisticSummary===String(config.job.id)){for(const button of area.querySelectorAll('[data-result-stat]'))button.setAttribute('aria-pressed',String(button.dataset.resultStat===selected.statistic));const context=area.querySelector('[data-result-statistic-context]');if(context){const html=statisticContext(selected);if(typeof patchResultHTML==='function')patchResultHTML(context,html);else context.innerHTML=html;}}}
 function notify(config){if(!config)return;updateStatusSummaries(config);updateStatisticSummaries(config);try{if(typeof root.CustomEvent==='function')root.document?.dispatchEvent?.(new root.CustomEvent('networkToolkitResultFilterChange',{detail:{jobId:String(config.job.id),view:config.view}}));}catch{}}
 function set(config,patch){
  if(!eligible(config)||!safeView(config.view))return;const prior=preferences(config.view),target=appState(),store=target?(target.resultFilters||(target.resultFilters=Object.create(null))):fallback;delete store[config.view];store[config.view]=normalize({...prior,...patch});const keys=Object.keys(store);for(const key of keys.slice(0,Math.max(0,keys.length-160)))delete store[key];
  if(typeof saveState==='function')saveState();
  for(const value of configs.values())if(value.view===config.view){if(typeof dataListPages!=='undefined')dataListPages.set(value.id,0);if(typeof refreshDataList==='function')refreshDataList(value.id,true);}notify(config);
 }
 function setStatistic(id,key){const config=jobs.get(String(id));if(!config||!statistics.has(key))return;const p=preferences(config.view);set(config,{statistic:p.statistic===key?'':key});}
 function clearStatistic(id){const config=jobs.get(String(id));if(config)set(config,{statistic:''});}
 function setStatusSummary(id,key){const config=jobs.get(String(id));if(!config||typeof key!=='string'||!key)return;const value=statusToken(key),p=preferences(config.view);set(config,{status:value===p.status&&!p.statistic?'all':value,statistic:''});}
 function clearStatusSummary(id){const config=jobs.get(String(id));if(config)set(config,{status:'all',statistic:''});}
 function clear(config){if(!eligible(config))return false;try{if(typeof listPreference==='function'&&typeof saveListPreference==='function')saveListPreference(config.view,{...listPreference(config.view,config.defaults),query:'',status:'all'});}catch{}set(config,{status:'all',type:'all',statistic:''});return true;}
 function handleChange(event){const target=event.target;if(target?.hasAttribute?.('data-result-filter-status')){set(configs.get(target.dataset.resultFilterStatus),{status:target.value});return true;}if(target?.hasAttribute?.('data-result-filter-type')){set(configs.get(target.dataset.resultFilterType),{type:target.value});return true;}return false;}
 function handleClick(event){const target=event.target,statusStat=target?.closest?.('[data-result-status-stat]');if(statusStat){event.preventDefault();setStatusSummary(statusStat.dataset.resultStatJob,statusStat.dataset.resultStatusStat);return true;}const statusClear=target?.closest?.('[data-result-status-clear]');if(statusClear){event.preventDefault();clearStatusSummary(statusClear.dataset.resultStatJob);return true;}const stat=target?.closest?.('[data-result-stat]');if(stat){event.preventDefault();setStatistic(stat.dataset.resultStatJob,stat.dataset.resultStat);return true;}const statClear=target?.closest?.('[data-result-stat-clear]');if(statClear){event.preventDefault();clearStatistic(statClear.dataset.resultStatJob);return true;}const reset=target?.closest?.('[data-result-filter-clear]');if(reset){event.preventDefault();clear(configs.get(reset.dataset.resultFilterClear));return true;}return false;}
 let initialized=false;function initialize(){if(initialized||!root.document?.addEventListener)return;initialized=true;root.document.addEventListener('change',handleChange);root.document.addEventListener('click',handleClick);}
 const api=Object.freeze({register,controls,matches,severity,pill,status,type,current,graphMatches,setStatistic,clearStatistic,statisticSummary,statusSummary,setStatusSummary,clearStatusSummary,normalize,mergePreferences,preferences,clear,changed:notify,handleChange,handleClick,initialize});
 if(typeof module==='object'&&module.exports)module.exports=api;root.NetworkToolkitResultFilters=api;initialize();
})(typeof window!=='undefined'?window:globalThis);
