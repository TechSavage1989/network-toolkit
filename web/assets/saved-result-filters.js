/* Versioned, tool-scoped filter presets. No results, credentials or run actions. */
(function(root){
 'use strict';
 const VERSION=1,MAX_FILTERS=64,MAX_PER_SCOPE=20,MAX_QUERY_UNITS=262144,contexts=new Map();let fallback={version:VERSION,scopes:Object.create(null)},dialog=null;
 const esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 const isObject=value=>value&&typeof value==='object'&&!Array.isArray(value);
 const safeScope=value=>typeof value==='string'&&value.length<=200&&/^(?:network|result):[a-z][a-z0-9_-]*(?::[a-zA-Z0-9_.:-]+)?$/.test(value)&&!/(?:^|:)(?:__proto__|constructor|prototype)(?::|$)/.test(value);
 function appState(){try{return state;}catch{return root.state||null;}}
 function name(value){return typeof value==='string'?value.trim():'';}
 function cleanFilter(value,scope){
  if(!isObject(value))return null;
  const network=scope.startsWith('network:'),query=value.query??'',limit=network?32768:300;
  if(typeof query!=='string'||query.length>limit)return null;
  const out={query:root.NetworkToolkitDiscoveryFilter?.singleLine(query)??query.replace(/[\r\n\u2028\u2029]/g,' ')};
  for(const key of network?['status']:['legacyStatus','status','type']){const token=value[key]??'all';if(typeof token!=='string'||token.length>100||!/^[-\w .]*$/.test(token))return null;out[key]=token||'all';}
  if(network)out.respondedOnly=value.respondedOnly===true;
  else{const statistic=value.statistic??'';if(!['','sent','received','loss','minimum','average','maximum','jitter'].includes(statistic))return null;out.statistic=statistic;}
  return out;
 }
 function normalize(value){
  const result={version:VERSION,scopes:Object.create(null)};if(!isObject(value)||value.version!==VERSION||!isObject(value.scopes))return result;
  let count=0,units=0;
  for(const scope of Object.keys(value.scopes).sort()){
   if(!safeScope(scope)||!Array.isArray(value.scopes[scope]))continue;
   const names=new Set(),ids=new Set(),rows=[];
   for(const row of value.scopes[scope]){
    if(count>=MAX_FILTERS||rows.length>=MAX_PER_SCOPE)break;
    const label=name(row?.name),id=row?.id,filter=cleanFilter(row?.filter,scope),folded=label.toLowerCase();
    if(!label||label.length>80||typeof id!=='string'||!/^[a-zA-Z0-9_-]{1,100}$/.test(id)||!filter||names.has(folded)||ids.has(id)||units+filter.query.length>MAX_QUERY_UNITS)continue;
    names.add(folded);ids.add(id);rows.push({id,name:label,filter});units+=filter.query.length;count++;
   }
   if(rows.length)result.scopes[scope]=rows;
  }
  return result;
 }
 function mergePreferences(saved){const next=normalize(saved?.savedResultFilters),target=appState();if(target)target.savedResultFilters=next;else fallback=next;}
 function store(){return normalize(appState()?.savedResultFilters||fallback);}
 function entries(scope){return store().scopes[scope]||[];}
 function persist(next){const target=appState();if(target)target.savedResultFilters=next;else fallback=next;if(typeof saveState==='function')saveState();refreshCounts();}
 function mutate(scope,action,{id='',name:input='',filter,confirm=false}={}){
  if(!safeScope(scope))throw Error('This filter scope is not supported.');
  const next=store(),rows=next.scopes[scope]||[],index=rows.findIndex(value=>value.id===id),label=name(input);
  if(action!=='create'&&index<0)throw Error('That saved filter no longer exists.');
  if(['create','rename'].includes(action)){
   if(!label||label.length>80)throw Error('Enter a filter name from 1 to 80 characters.');
   if(rows.some((value,i)=>value.name.toLowerCase()===label.toLowerCase()&&i!==index))throw Error('That name already exists in this tool. Choose another name, or select the existing filter and use Update saved.');
  }
  if(['create','update'].includes(action)){
   const clean=cleanFilter(filter,scope);if(!clean)throw Error('The filter contains unsupported options or an oversized query.');
   if(action==='update'&&!confirm)throw Error('Confirm Update saved before replacing its filter options.');
   if(action==='create'){
    if(rows.length>=MAX_PER_SCOPE||Object.values(next.scopes).flat().length>=MAX_FILTERS)throw Error('Saved filter limit reached (20 per tool, 64 total). Delete a filter first.');
    id='filter-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,9);rows.push({id,name:label,filter:clean});
   }else rows[index]={...rows[index],filter:clean};
  }else if(action==='rename')rows[index]={...rows[index],name:label};
  else if(action==='delete'){if(!confirm)throw Error('Confirm Delete before removing the saved filter.');rows.splice(index,1);}
  else throw Error('Unknown saved filter action.');
  next.scopes[scope]=rows;
  if(Object.values(next.scopes).flat().reduce((sum,row)=>sum+row.filter.query.length,0)>MAX_QUERY_UNITS)throw Error('Saved queries exceed the combined limit. Shorten or delete a saved filter first.');
  persist(next);return id;
 }
 function register(context){contexts.set(context.id,context);while(contexts.size>240)contexts.delete(contexts.keys().next().value);return context;}
 function button(context){const compact=!!context.config?.graphId,count=entries(context.scope).length;register(context);return '<button type="button" class="btn small ghost saved-filter-button'+(compact?' saved-filter-compact':'')+'" data-saved-filter-open="'+esc(context.id)+'" aria-label="Saved filters for '+esc(context.scope)+' ('+count+')" title="Save and reuse query and filter options for this tool only">'+(compact?'Saved':'Saved filters')+' <span data-saved-filter-count="'+esc(context.scope)+'">('+count+')</span></button>';}
 function networkControl(job){return safeScope('network:'+job?.tool)?button({id:'network:'+job.id,scope:'network:'+job.tool,kind:'network',job}):'';}
 function resultControl(config){return config?.job&&safeScope(config.view)&&(config.view==='result:'+config.job.tool||config.view.startsWith('result:'+config.job.tool+':'))&&Array.isArray(config.rows)&&Array.isArray(config.columns)&&(!config.card||config.job.tool==='quick')?button({id:'result:'+config.id,scope:config.view,kind:'result',config}):'';}
 function capture(context){
  if(context.kind==='network')return cleanFilter(networkFilter(context.job),context.scope);
  const config=context.config,list=listPreference(config.view,config.defaults),detail=root.NetworkToolkitResultFilters?.preferences(config.view)||{};
  return cleanFilter({query:list.query,legacyStatus:list.status,...detail},context.scope);
 }
 function apply(context,id){
  if(!context||!safeScope(context.scope)||(context.kind==='network'?context.scope!=='network:'+context.job?.tool:context.kind!=='result'||context.scope!==context.config?.view))throw Error('This saved filter is not compatible with this tool.');
  const selected=entries(context.scope).find(row=>row.id===id);if(!selected)throw Error('Choose a saved filter first.');
  const filter=cleanFilter(selected.filter,context.scope);if(!filter)throw Error('This saved filter is not compatible with this tool.');
  if(context.kind==='network'){
   const snapshot=networkSnapshot(context.job),allowed=snapshot.neighbor?['all','mac','vendor','model']:['all','responded','finished','open','vendor','model',...(!snapshot.legacyDiscovery?['no_response','error','cancelled']:[])];
   if(!allowed.includes(filter.status))throw Error('This saved outcome is not supported by this older result format. Choose a compatible result; the current filter is unchanged.');
   networkResultFilters.set(context.job.id,{...networkFilter(context.job),...filter,page:0});saveNetworkFilter(context.job.id);
   for(const area of root.document?.querySelectorAll?.('[data-network-results]')||[])if(area.dataset.networkResults===context.job.id)patchResultHTML(area,renderNetworkResults(context.job,!!area.closest('#toolResult')?.querySelector('#toolNetworkStatus')),true);
   if(typeof updateToolResultSticky==='function')requestAnimationFrame(updateToolResultSticky);
  }else{
   const config=context.config,engine=root.NetworkToolkitResultFilters;
   if(!engine?.set||!Array.isArray(config.rows)||!Array.isArray(config.columns)||!config.job||config.card&&config.job.tool!=='quick')throw Error('This result view does not support saved filter options.');
   const legacyOptions=config.statusOptions?.map(option=>String(option[0]))||['all'];
   if(!legacyOptions.includes(filter.legacyStatus))throw Error('The saved list status is not supported by this result view. Current filters are unchanged.');
   const normalized=engine.normalize(filter);
   if(['status','type','statistic'].some(key=>normalized[key]!==filter[key]))throw Error('The saved Status, Type or statistic uses an incompatible format. Current filters are unchanged.');
   if(filter.statistic&&!config.graphId&&!['ping','monitor','trace'].includes(config.job.tool))throw Error('This result view does not support the saved measurement statistic. Current filters are unchanged.');
   // Validate everything before changing either legacy or detailed filter state.
   // A valid dynamic Status/Type absent from current rows remains selected; the
   // shared controls explicitly label it as not in current results.
   saveListPreference(config.view,{...listPreference(config.view,config.defaults),query:filter.query,status:filter.legacyStatus});
   root.NetworkToolkitResultFilters.set(config,{status:filter.status,type:filter.type,statistic:filter.statistic});
  }
  return selected;
 }
 function summary(filter,scope){if(!filter)return '';return '<dl class="saved-filter-summary"><dt>Query</dt><dd class="saved-filter-query">'+esc(filter.query||'(empty: all text)')+'</dd>'+Object.entries(filter).filter(([key])=>key!=='query').map(([key,value])=>'<dt>'+esc(({legacyStatus:'List status',status:scope.startsWith('network:')?'Outcome':'Detailed status',type:'Type',statistic:'Statistic',respondedOnly:'Responded / resolved only'})[key])+'</dt><dd>'+esc(typeof value==='boolean'?(value?'Yes':'No'):value||'None')+'</dd>').join('')+'</dl>';}
 function refreshCounts(){for(const el of root.document?.querySelectorAll?.('[data-saved-filter-count]')||[]){const count=entries(el.dataset.savedFilterCount).length;el.textContent='('+count+')';el.closest?.('.saved-filter-button')?.setAttribute('aria-label','Saved filters for '+el.dataset.savedFilterCount+' ('+count+')');}}
 function draw(message='',error=false){
  const context=contexts.get(dialog?.context);if(!context)return;
  const rows=entries(context.scope),selected=rows.find(value=>value.id===dialog.selected),pending=dialog.pending;
  const html='<p>Scope: <strong>'+esc(context.scope)+'</strong>. Saves only the query and options shown below. No result rows, credentials, scan settings, sorting or connections are saved.</p><details class="saved-filter-current"><summary>Current filter options</summary>'+summary(capture(context),context.scope)+'</details><label class="field">Saved filter<select id="savedFilterChoice"><option value="">Choose a saved filter…</option>'+rows.map(row=>'<option value="'+esc(row.id)+'"'+(row.id===dialog.selected?' selected':'')+'>'+esc(row.name)+'</option>').join('')+'</select></label><label class="field">Name<input id="savedFilterName" maxlength="80" autocomplete="off" value="'+esc(dialog.name||'')+'" placeholder="Name for Save as new or Rename"></label>'+(selected?'<details open class="saved-filter-selected"><summary>Saved options: '+esc(selected.name)+'</summary>'+summary(selected.filter,context.scope)+'</details>':'')+(message?'<p role="status" class="saved-filter-message '+(error?'bad':'')+'">'+esc(message)+'</p>':'')+(pending?'<div class="notice warn"><span>'+esc(pending==='update'?'Replace the selected saved filter with the current options?':'Delete the selected saved filter? Current results and active filters are unchanged.')+'</span><button type="button" class="btn small" data-saved-filter-action="confirm">Confirm '+(pending==='update'?'update':'delete')+'</button><button type="button" class="btn small ghost" data-saved-filter-action="cancel-confirm">Keep saved filter</button></div>':'');
  const disabled=selected?'':' disabled';openModal('Saved filters',html,'<div class="saved-filter-actions"><button type="button" class="btn" data-saved-filter-action="create">Save as new</button><button type="button" class="btn primary" data-saved-filter-action="load"'+disabled+'>Load</button><button type="button" class="btn ghost" data-saved-filter-action="rename"'+disabled+'>Rename</button><button type="button" class="btn ghost" data-saved-filter-action="update"'+disabled+'>Update saved</button><button type="button" class="btn danger" data-saved-filter-action="delete"'+disabled+'>Delete</button><button type="button" class="btn ghost" data-action="close-modal">Close</button></div>',true);
 }
 function open(id){if(!contexts.has(id))return false;dialog={context:id,selected:'',name:'',pending:''};draw();return true;}
 function handleChange(event){if(event.target?.id!=='savedFilterChoice'||!dialog)return false;dialog.selected=event.target.value;dialog.name=entries(contexts.get(dialog.context).scope).find(row=>row.id===dialog.selected)?.name||'';dialog.pending='';draw();root.document.getElementById('savedFilterChoice')?.focus();return true;}
 function handleClick(event){
  const launch=event.target.closest?.('[data-saved-filter-open]'),action=event.target.closest?.('[data-saved-filter-action]');if(!launch&&!action)return false;event.preventDefault();event.stopImmediatePropagation();if(launch)return open(launch.dataset.savedFilterOpen);if(!dialog)return false;
  const context=contexts.get(dialog.context);if(!context)return false;dialog.name=root.document.getElementById('savedFilterName')?.value??dialog.name;const operation=action.dataset.savedFilterAction;
  try{
   if(operation==='load'){const chosen=apply(context,dialog.selected);closeModal();dialog=null;if(typeof toast==='function')toast('Loaded filter: '+chosen.name+'. No scan or connection started.','success');return true;}
   if(['update','delete'].includes(operation)){dialog.pending=operation;draw();return true;}
   if(operation==='cancel-confirm'){dialog.pending='';draw();return true;}
   const actual=operation==='confirm'?dialog.pending:operation;if(!actual)return true;
   const id=mutate(context.scope,actual,{id:actual==='create'?'':dialog.selected,name:dialog.name,filter:capture(context),confirm:operation==='confirm'});
   dialog.selected=actual==='delete'?'':id;dialog.pending='';draw(actual==='delete'?'Saved filter deleted.':'Saved filter '+(actual==='rename'?'renamed.':actual==='update'?'updated.':'created.')+' Active filters were not changed.');
  }catch(error){draw(error.message,true);}
  return true;
 }
 root.document?.addEventListener?.('click',handleClick,true);root.document?.addEventListener?.('change',handleChange);
 const api=Object.freeze({VERSION,MAX_FILTERS,MAX_PER_SCOPE,MAX_QUERY_UNITS,safeScope,cleanFilter,normalize,mergePreferences,entries,mutate,networkControl,resultControl,capture,apply,open,handleClick,handleChange});root.NetworkToolkitSavedFilters=api;if(typeof module==='object'&&module.exports)module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
