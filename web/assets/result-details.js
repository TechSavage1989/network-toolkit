(function (root) {
  'use strict';

  let initialized = false, pendingClick = 0, pointerStart = null, activeDetail = null, sideDetail = null;
  let resizeObserver, observedArea, finishResize;
  const defaults = {width:350,height:260,hidden:false,dock:'auto'};
  const clamp = (value,min,max) => Math.max(min,Math.min(max,value));
  const interactive = 'button,a,input,select,textarea,label,summary,[contenteditable="true"],[role="button"],[role="link"],[role="checkbox"],[role="separator"],[data-column-resize]';

  function preferences() { return state.entryDetails || (state.entryDetails={...defaults}); }
  function mergePreferences(saved) {
    const source=saved?.entryDetails,value=source&&typeof source==='object'&&!Array.isArray(source)?source:{};
    state.entryDetails={width:Number.isInteger(value.width)?clamp(value.width,260,700):defaults.width,height:Number.isInteger(value.height)?clamp(value.height,80,650):defaults.height,hidden:value.hidden===true,dock:['auto','right','bottom'].includes(value.dock)?value.dock:'auto'};
    if(Number.isInteger(value.graphHeight))state.entryDetails.graphHeight=clamp(value.graphHeight,80,650);layoutDetails();
  }
  function persist() { if(typeof saveState==='function')saveState(); }
  function sticky() { if(typeof root.requestAnimationFrame==='function'&&typeof updateToolResultSticky==='function')root.requestAnimationFrame(updateToolResultSticky); }
  function layoutDetails() {
    const area=document.getElementById('toolResult'),pane=document.getElementById('resultEntryPane');if(!sideDetail||!area||!pane)return;
    const pref=preferences(),bounds=area.getBoundingClientRect(),width=bounds.width||area.clientWidth||1000,height=bounds.height||area.clientHeight||400,bottom=pref.dock==='bottom'||pref.dock!=='right'&&width<(sideDetail.graph?640:900);
    const minimum=bottom?Math.min(sideDetail.graph?130:80,Math.max(40,height-96)):260,maximum=bottom?Math.max(minimum,Math.min(650,height*.65,height-96)):Math.max(260,Math.min(700,width-326)),dimension=Math.round(clamp(sideDetail.graph&&bottom&&pref.dock==='auto'?(pref.graphHeight??Math.min(pref.height,160)):pref[bottom?'height':'width'],minimum,maximum));
    area.classList.toggle('has-entry-details',!pref.hidden);area.classList.toggle('entry-details-hidden',pref.hidden);area.classList.toggle('entry-details-bottom',bottom);
    area.style.setProperty('--entry-details-size',(pref.hidden?34:dimension)+'px');
    pane.classList.toggle('hidden',pref.hidden);const rail=document.getElementById('resultEntryRail');rail?.classList.toggle('hidden',!pref.hidden);
    const divider=document.getElementById('resultEntryDivider');if(divider){divider.setAttribute('aria-orientation',bottom?'horizontal':'vertical');divider.setAttribute('aria-valuenow',pref.hidden?'0':String(dimension));divider.setAttribute('aria-valuemin',String(pref.hidden?0:Math.round(minimum)));divider.setAttribute('aria-valuemax',String(Math.round(maximum)));divider.setAttribute('aria-label',pref.hidden?'Show entry details':'Resize or hide entry details');divider.setAttribute('aria-valuetext',pref.hidden?'Collapsed':dimension+' pixels');divider.title=pref.hidden?'Double-click or press Enter or Space to show entry details.':'Drag to resize. Double-click or press Enter or Space to hide details. Arrow keys resize; Home resets.';}
    for(const button of area.querySelectorAll('[data-result-side-close],[data-result-side-show]'))root.NetworkToolkitPanelControls.set(button,{name:'entry details',edge:bottom?'bottom':'right',expanded:!pref.hidden});
    root.NetworkToolkitPanelControls.bindReveal(rail,{collapsed:()=>!!sideDetail&&preferences().hidden,button:()=>document.getElementById('resultEntryShow')});
    const dock=document.getElementById('resultEntryDock');if(dock)dock.value=pref.dock;
    if(!resizeObserver&&typeof root.ResizeObserver==='function')resizeObserver=new root.ResizeObserver(layoutDetails);
    if(resizeObserver&&observedArea!==area){if(observedArea)resizeObserver.unobserve(observedArea);observedArea=area;resizeObserver.observe(area);}
    sticky();
  }
  function toggleDetails(hidden,moveFocus=true) {
    if(!sideDetail)return;preferences().hidden=hidden===undefined?!preferences().hidden:hidden;layoutDetails();persist();
    if(moveFocus)(preferences().hidden?document.getElementById('resultEntryShow'):document.querySelector('#resultEntryPane [data-result-side-close]'))?.focus({preventScroll:true});
  }
  function resizeDetails(event,divider) {
    if(event.button!==0||!sideDetail||preferences().hidden)return;
    finishResize?.();event.preventDefault();event.stopPropagation();divider.focus({preventScroll:true});
    const area=document.getElementById('toolResult'),bottom=area.classList.contains('entry-details-bottom'),name=bottom?'height':'width',start=bottom?event.clientY:event.clientX,initial=Number(divider.getAttribute('aria-valuenow')),pointer=event.pointerId;
    divider.classList.add('dragging');document.body.classList.add(bottom?'resizing-rows':'resizing-columns');
    const move=next=>{if(pointer!==undefined&&next.pointerId!==undefined&&next.pointerId!==pointer)return;if(!area.isConnected||!sideDetail){finish();return;}const coordinate=bottom?next.clientY:next.clientX;if(!Number.isFinite(coordinate))return;preferences()[name]=Math.round(clamp(initial-(coordinate-start),Number(divider.getAttribute('aria-valuemin')),Number(divider.getAttribute('aria-valuemax'))));if(sideDetail.graph&&bottom)preferences().graphHeight=preferences().height;layoutDetails();};
    const finish=next=>{if(next?.pointerId!==undefined&&pointer!==undefined&&next.pointerId!==pointer)return;root.removeEventListener('pointermove',move);for(const type of ['pointerup','pointercancel','blur'])root.removeEventListener(type,finish);divider.classList.remove('dragging');document.body.classList.remove('resizing-rows','resizing-columns');finishResize=null;persist();};
    finishResize=finish;root.addEventListener('pointermove',move);for(const type of ['pointerup','pointercancel','blur'])root.addEventListener(type,finish);
  }

  function rowAttributes(config, row) {
    if (!config || config.card || config.rowDetails === false || !Array.isArray(config.rows)) return '';
    const index = config.rows.indexOf(row);
    if (index < 0) return '';
    const title = recordTitle(row);
    return ' data-result-list="' + esc(config.id) + '" data-result-row="' + index + '" tabindex="0" aria-label="View ' + esc(config.name || 'entry') + ' details' + (title ? ' for ' + esc(title) : '') + ' and available tools" title="View details and available tools"';
  }

  function candidate(target) {
    const row = target.closest?.('tr[data-result-row], [data-entry-key]');
    if (!row || !row.hasAttribute('data-result-row') && row.closest('[data-network-results]') || target.closest('.xterm')) return null;
    const control = target.closest(interactive);
    if (control && row.contains(control)) return null;
    return row;
  }

  function captureRow(element) {
    const list = element.dataset.resultList, index = Number(element.dataset.resultRow), config = dataListViews.get(list);
    if (!config || !Number.isInteger(index) || index < 0 || index >= config.rows.length) return null;
    const original = config.rows[index];
    let record;
    try { record = JSON.parse(JSON.stringify(original)); } catch { record = original; }
    return {record, name: config.name || 'Entry', view: config.view || '', list, key: element.dataset.entryKey || '', context:config.job?{tool:config.job.tool,target:config.job.target,started:config.job.started,mode:config.job.options?.mode,port:config.job.options?.port}:null};
  }

  function recordTitle(record) {
    if (!record || typeof record !== 'object') return '';
    if (record.hop != null) return 'Hop ' + record.hop + (record.address ? ': ' + record.address : '');
    if (record.port != null) return 'Port ' + record.port + (record.address ? ': ' + record.address : '');
    if (record.sequence != null) return 'Probe ' + record.sequence + (record.address ? ': ' + record.address : '');
    if (record.type && record.value != null) return String(record.type) + ': ' + String(record.name || record.value).slice(0,120);
    if (record.protocol && record.remote) return String(record.protocol).toUpperCase() + ': ' + String(record.remote).slice(0,120);
    for (const key of ['hostname', 'address', 'host', 'name', 'destination', 'remote', 'local', 'subnet', 'input', 'serialPort']) {
      if (typeof record[key] === 'string' && record[key].trim()) return record[key].trim().slice(0,120);
    }
    return '';
  }

  function fieldsHTML(record) {
    if (!record || typeof record !== 'object' || Array.isArray(record)) return '<pre class="result-record-value">' + esc(typeof record === 'object' ? JSON.stringify(record, null, 2) : scalar(record)) + '</pre>';
    const entries = Object.entries(record), simple = entries.filter(([,value]) => value == null || typeof value !== 'object'), nested = entries.filter(([,value]) => value != null && typeof value === 'object');
    const values = simple.length ? '<dl class="result-record-grid">' + simple.map(([key,value]) => '<div><dt>' + esc(root.NetworkToolkitResultTime?.isKey(key)?(root.NetworkToolkitResultTime.columnLabel?.()||'Recorded (local)'):label(key)) + '</dt><dd>' + (root.NetworkToolkitResultTime?.isKey(key)?root.NetworkToolkitResultTime.cell(value):esc(scalar(value))) + '</dd></div>').join('') + '</dl>' : '';
    return values + nested.map(([key,value]) => '<details class="result-record-nested"><summary>' + esc(label(key)) + (Array.isArray(value) ? ' (' + value.length + ' ' + (value.length === 1 ? 'entry' : 'entries') + ')' : '') + '</summary>' + renderStaticObject(value, 1) + '</details>').join('');
  }

  function launchButtons(key, record) {
    return ['http', 'https', 'rdp'].map(protocol => {
      const action = deviceConnectionLaunch(record, protocol);
      if (!action.ok) return '';
      const allowed = online && status.connectionLaunchAvailable === true;
      const hint = !online ? 'Open the executable to launch this connection.' : !status.connectionLaunchAvailable ? 'Available in the Windows application.' : action.target;
      return '<button class="btn small ghost" data-entry-action="open:' + protocol + '" data-entry-record="' + esc(key) + '" data-connection-host="' + esc(action.host) + '" data-connection-port="' + action.port + '" ' + (!allowed ? 'disabled ' : '') + 'title="' + esc(hint) + '">' + esc(action.label) + '<span class="connection-destination">' + esc(action.target) + '</span></button>';
    }).join('');
  }

  function toolsHTML(detail) {
    const entry = detail.key ? deviceRecords.get(detail.key) : null, parts = [];
    if (entry) {
      const host = deviceTarget(entry), port = endpointPort(entry.endpointPort);
      if (host) {
        const diagnostics = [['ping','Run ping'],['trace','Run traceroute'],['dns',usableDeviceAddress(host) ? 'Reverse DNS' : 'DNS lookup'],['tcp',port ? 'Test TCP port ' + port : 'Test TCP ports']].filter(([tool]) => deviceApplicability(entry, tool).ok);
        const terminals = [['ssh','SSH'],['telnet','Telnet']].filter(([tool]) => deviceApplicability(entry, tool).ok).map(([tool,name]) => '<button class="btn small" data-device-connect="' + tool + '" data-device-record="' + esc(detail.key) + '" ' + (!online ? 'disabled ' : '') + '>' + icon(tool === 'ssh' ? 'lock' : 'terminal') + name + (port ? ' :' + port : '') + '</button>').join('');
        const connections = launchButtons(detail.key, entry);
        parts.push('<div class="result-detail-target"><span>' + esc(entry.endpointRole || 'Device target') + '</span><strong class="mono">' + esc((host.includes(':') ? '[' + host + ']' : host) + (port ? ':' + port : '')) + '</strong></div>');
        if (terminals || connections) parts.push('<section class="result-detail-section"><h3>Connect</h3><div class="result-detail-actions">' + terminals + '</div>' + (connections ? '<div class="device-open-actions">' + connections + '</div>' : '') + '</section>');
        if (diagnostics.length) parts.push('<section class="result-detail-section"><h3>Diagnostics</h3>' + (root.NetworkToolkitSource?.deviceControlHTML ? root.NetworkToolkitSource.deviceControlHTML() : '') + '<div class="result-detail-actions">' + diagnostics.map(([tool,name]) => '<button class="btn small ghost" data-run-device="' + tool + '" data-device-record="' + esc(detail.key) + '" ' + (!online ? 'disabled ' : '') + '>' + icon(TOOLS[tool]?.icon || 'play') + esc(name) + '</button>').join('') + '</div></section>');
        parts.push('<button class="btn small ghost" data-device-key="' + esc(detail.key) + '">' + icon('settings') + 'All device tools</button>');
      } else if (entry.endpointUnavailable) parts.push('<p class="help-line">' + esc(entry.endpointUnavailable) + '</p>');
    }
    const value = detail.record, mac = value && typeof value === 'object' ? rowMAC({mac: value.mac || value.colon || ''}) : '';
    if (mac) parts.push('<button class="btn small ghost" data-wol-mac="' + esc(mac) + '">' + icon('power') + 'Configure Wake on LAN</button>');
    if (detail.view === 'utility:subnet-split' && typeof value?.subnet === 'string') parts.push('<button class="btn small ghost" data-use-target="' + esc(value.subnet) + '">' + icon('network') + 'Use subnet in Discovery</button>');
    return parts.length ? '<section class="result-detail-tools"><h3>Available tools</h3>' + parts.join('') + '</section>' : '';
  }

  function graphExtras(detail){
    if(!detail.graph)return '';const g=detail.graph,point=g.point,members=point.members||[],page=Math.max(0,Math.min(Math.ceil(members.length/50)-1,detail.contributorPage||0)),start=page*50;
    const controls='<div class="result-detail-actions"><button type="button" class="btn small ghost" data-result-graph-clear="'+esc(g.id)+'">Clear selection</button><button type="button" class="btn small ghost" data-result-graph-latest="'+esc(g.id)+'">Latest</button></div>';
    const contributors=members.length?'<section class="result-graph-contributors"><strong>Contributing recorded results ('+members.length+')</strong><p>Choose an individual result to inspect its exact recorded fields.</p>'+members.slice(start,start+50).map(p=>'<button type="button" class="btn small ghost" data-result-graph-source="'+esc(p.key)+'" data-result-graph-id="'+esc(g.id)+'">#'+esc(p.sequence)+' · '+esc(root.NetworkToolkitResultTime?.format?.(p.time)||p.time)+' · '+esc(p.status)+' · '+(p.rtt==null?'Not measured':esc(p.rtt)+' ms')+'</button>').join('')+(members.length>50?'<div class="result-detail-actions"><button type="button" class="btn small ghost" data-result-graph-page="'+(page-1)+'" '+(page===0?'disabled':'')+'>Previous</button><span>'+(start+1)+'-'+Math.min(start+50,members.length)+' of '+members.length+'</span><button type="button" class="btn small ghost" data-result-graph-page="'+(page+1)+'" '+(start+50>=members.length?'disabled':'')+'>Next</button></div>':'')+'</section>':'';
    return '<section class="result-graph-summary"><p class="help-line">'+esc(g.message)+'</p>'+controls+(root.NetworkToolkitResultTimeline?.pointDetails?.(point)||'')+contributors+'</section>';
  }
  function showGraphSelection(graph){
    const area=document.getElementById('toolResult');if(!area||!graph?.point)return false;
    const config=typeof dataListViews!=='undefined'?[...dataListViews.values()].find(c=>c.graphId===String(graph.id)):null,point=graph.point,job=graph.job||config?.job;
    const key=String(point.key),same=sideDetail?.graph?.id===graph.id&&sideDetail.graph.point.key===key;
    if(same){sideDetail.graph={...graph};if(graph.force&&preferences().hidden){preferences().hidden=false;persist();}afterRender();return true;}
    let record;try{record=JSON.parse(JSON.stringify(point.source||{}));}catch{record={...point.source};}
    const anchor=[...area.querySelectorAll('[data-result-sample-key]')].find(row=>row.dataset.resultSampleKey===key)||area.querySelector('[data-graph-workspace]')||area;
    const detail={record,name:point.members?'Condensed interval':'Sample',list:config?.id||'',view:config?.view||'',key:'',context:job?{tool:job.tool,target:job.target,started:job.started,mode:job.options?.mode,port:job.options?.port,source:job.options?.source}:null,graph:{...graph},anchor,run:area.dataset.scrollContext,page:viewToken()};
    if(sideDetail?.graph?.id===graph.id){sideDetail=detail;if(graph.force&&preferences().hidden){preferences().hidden=false;persist();}afterRender();return true;}
    return openSide(detail,anchor);
  }
  function clearGraphSelection(id){if(sideDetail?.graph?.id===id)closeSide();}

  function detailBody(detail) {
    const json=JSON.stringify(detail.record,null,2),context=detail.context;
    const run=context?'<section class="result-run-context"><dl class="result-record-grid">'+[['Tool',titleFor(context.tool)],['Target',context.target],['Run started',context.started],['Method',context.mode],['Port',context.port],['Source interface',context.source]].filter(([,value])=>value!==undefined&&value!=='').map(([key,value])=>'<div><dt>'+esc(key)+'</dt><dd>'+(key==='Run started'&&root.NetworkToolkitResultTime?root.NetworkToolkitResultTime.cell(value,{label:'Run started'}):esc(scalar(value)))+'</dd></div>').join('')+'</dl></section>':'';
    return '<div class="result-row-detail"><p class="result-detail-caption">Recorded '+esc(String(detail.name).replace(/\./g,' '))+' entry</p><div class="result-detail-actions result-detail-copy"><button class="btn small ghost" data-result-copy="text">'+icon('copy')+'Copy details</button><button class="btn small ghost" data-result-copy="json">Copy JSON</button></div>'+graphExtras(detail)+run+fieldsHTML(detail.record)+toolsHTML(detail)+'<details class="result-record-json"><summary>Raw record JSON</summary><pre>'+esc(json)+'</pre></details></div>';
  }

  function closeSide(focus=false) {
    finishResize?.();const old=sideDetail;sideDetail=null;
    if(observedArea){resizeObserver?.unobserve(observedArea);observedArea=null;}
    const area=document.getElementById('toolResult');area?.classList.remove('has-entry-details','entry-details-hidden','entry-details-bottom');area?.style.removeProperty('--entry-details-size');document.getElementById('resultEntryPane')?.remove();document.getElementById('resultEntryRail')?.remove();document.getElementById('resultEntryDivider')?.remove();
    for(const row of area?.querySelectorAll('tr[data-result-row]')||[]){row.classList.remove('result-entry-selected');row.removeAttribute('aria-current');}
    if(focus)(old?.anchor?.isConnected?old.anchor:document.getElementById('toolResultsScroll'))?.focus?.({preventScroll:true});
    sticky();
  }

  function afterRender() {
    if(!sideDetail)return;
    const area=document.getElementById('toolResult');
    if(!area||area.dataset.scrollContext!==sideDetail.run||viewToken()!==sideDetail.page){closeSide();return;}
    let pane=document.getElementById('resultEntryPane');
    if(!pane){
      pane=document.createElement('aside');pane.id='resultEntryPane';pane.className='result-entry-pane';pane.setAttribute('role','region');pane.setAttribute('aria-label','Selected entry details');
      pane.innerHTML='<div class="result-entry-heading"><button type="button" class="iconbtn" data-result-side-close aria-controls="resultEntryPane"></button><div><strong>Entry details</strong><span id="resultEntryTitle" aria-live="polite"></span></div><label class="sr-only" for="resultEntryDock">Details position</label><select id="resultEntryDock" aria-label="Details position" title="Details position"><option value="auto">Auto</option><option value="right">Right</option><option value="bottom">Bottom</option></select></div><div id="resultEntryBody" class="result-entry-body" tabindex="0" aria-label="Selected entry information"></div><div class="result-entry-footer"><button type="button" class="btn small ghost" data-result-side-expand>'+icon('maximize')+'Expand details</button></div>';
      area.append(pane);
    }
    let rail=document.getElementById('resultEntryRail');if(!rail){rail=document.createElement('div');rail.id='resultEntryRail';rail.className='result-entry-rail';area.append(rail);}
    let show=document.getElementById('resultEntryShow');if(!show){show=document.createElement('button');show.id='resultEntryShow';show.className='iconbtn result-entry-show';show.setAttribute('type','button');show.setAttribute('data-result-side-show','');show.setAttribute('aria-controls','resultEntryPane');rail.append(show);}
    if(!document.getElementById('resultEntryDivider')){const divider=document.createElement('div');divider.id='resultEntryDivider';divider.setAttribute('role','separator');divider.setAttribute('tabindex','0');divider.setAttribute('aria-controls','resultEntryPane');area.append(divider);}
    pane.classList.toggle('result-graph-entry',!!sideDetail.graph);const signature=JSON.stringify(sideDetail.record),displaySignature=signature+'|'+JSON.stringify(root.NetworkToolkitResultTime?.preferences?.()||{})+'|'+(sideDetail.graph?.message||'')+'|'+(sideDetail.contributorPage||0),body=document.getElementById('resultEntryBody');
    if(body._recordSignature!==displaySignature){
      const sameRecord=body._sourceRecordSignature===signature,scrollTop=sameRecord?body.scrollTop:0,expanded=sameRecord?[...body.querySelectorAll('details')].map(detail=>detail.open):null;
      body.innerHTML=detailBody(sideDetail);body._recordSignature=displaySignature;body._sourceRecordSignature=signature;
      if(expanded)[...body.querySelectorAll('details')].forEach((detail,index)=>{detail.open=expanded[index]===true;});
      body.scrollTop=scrollTop;document.getElementById('resultEntryTitle').textContent=sideDetail.graph?.point.members?'Condensed interval · '+sideDetail.graph.point.members.length+' samples':(recordTitle(sideDetail.record)||sideDetail.name)+(sideDetail.graph?' · '+(sideDetail.graph.point.rtt==null?'Not measured':sideDetail.graph.point.rtt+' ms')+' · '+sideDetail.graph.point.status:'');
    }
    for(const row of area.querySelectorAll('tr[data-result-row]')){
      const detail=captureRow(row),selected=sideDetail.graph&&!sideDetail.graph.point.members?row.dataset.resultSampleKey===sideDetail.graph.point.key:detail?.list===sideDetail.list&&JSON.stringify(detail.record)===signature;
      row.classList.toggle('result-entry-selected',selected);if(selected)row.setAttribute('aria-current','true');else row.removeAttribute('aria-current');
    }
    layoutDetails();
  }

  function openSide(detail,anchor) {
    const area=anchor?.closest?.('#toolResult');if(!detail||!area||anchor.closest('.modal'))return false;
    closeEntryContextMenu(false);
    if(preferences().hidden){preferences().hidden=false;persist();}sideDetail={...detail,anchor,run:area.dataset.scrollContext,page:viewToken()};afterRender();return true;
  }

  function openRow(detail, anchor, expanded=false) {
    if (!detail) return;
    if(!expanded&&openSide(detail,anchor))return;
    closeEntryContextMenu(false);
    const modalRoot = document.getElementById('modalRoot'), previous = modalRoot.firstElementChild, previousFocus = modalLastFocus;
    const previousScroll = previous ? [...previous.querySelectorAll('*')].filter(element => element.scrollTop || element.scrollLeft).map(element => [element,element.scrollTop,element.scrollLeft]) : [];
    const title = recordTitle(detail.record), json = JSON.stringify(detail.record, null, 2);
    anchor?.focus({preventScroll:true});
    activeDetail = {...detail, previous, previousFocus, previousScroll, anchor, json};
    const body = detailBody(detail);
    const footer = (previous ? '<button class="btn ghost" data-result-back>' + icon('chevron') + 'Back to previous view</button>' : '') + '<button class="btn primary" data-action="close-modal">Done</button>';
    openModal('Entry details' + (title ? ': ' + title : ''), body, footer, true);
  }

  function returnToPrevious() {
    const detail = activeDetail;
    if (!detail?.previous) return;
    document.getElementById('modalRoot').replaceChildren(detail.previous);
    modalLastFocus = detail.previousFocus;
    document.body.style.overflow = 'hidden';
    for (const [element,top,left] of detail.previousScroll) { element.scrollTop = top; element.scrollLeft = left; }
    if (detail.anchor?.isConnected) detail.anchor.focus({preventScroll:true});
    activeDetail = null;
  }

  function cancelPending() { if (pendingClick) clearTimeout(pendingClick); pendingClick = 0; }
  function showCandidate(row, detail) {
    if(row.dataset.resultGraphId&&row.dataset.resultSampleKey&&root.NetworkToolkitResultTimeline?.selectResult?.(row.dataset.resultGraphId,row.dataset.resultSampleKey))return;
    if (row.hasAttribute('data-result-row')) openRow(detail || captureRow(row), row.isConnected ? row : null);
    else if (row.dataset.entryKey && deviceRecords.has(row.dataset.entryKey)) { row.focus({preventScroll:true}); openDeviceTools(row.dataset.entryKey); }
  }
  function viewToken() { return JSON.stringify([state.activePage,state.localTab,state.utilityTab]); }

  function init() {
    if (initialized) return;
    initialized = true;
    root.addEventListener?.('resize',layoutDetails);
    document.addEventListener('change',event=>{if(event.target.id==='resultEntryDock'&&['auto','right','bottom'].includes(event.target.value)){preferences().dock=event.target.value;layoutDetails();persist();}});
    document.addEventListener('dblclick',event=>{if(event.target.id==='resultEntryDivider'&&sideDetail){event.preventDefault();event.stopPropagation();toggleDetails(undefined,false);}});
    document.addEventListener('pointerdown', event => {
      cancelPending();
      if(event.target.id==='resultEntryDivider'){pointerStart=null;resizeDetails(event,event.target);return;}
      pointerStart = event.button === 0 ? {row:candidate(event.target),x:event.clientX,y:event.clientY} : null;
    }, true);
    document.addEventListener('click', event => {
      cancelPending();
      const graphClear=event.target.closest('[data-result-graph-clear]'),graphLatest=event.target.closest('[data-result-graph-latest]'),graphSource=event.target.closest('[data-result-graph-source]'),graphPage=event.target.closest('[data-result-graph-page]');
      if(graphClear||graphLatest||graphSource||graphPage){event.preventDefault();event.stopImmediatePropagation();if(graphClear)root.NetworkToolkitResultTimeline?.clearSelection?.(graphClear.dataset.resultGraphClear);else if(graphLatest)root.NetworkToolkitResultTimeline?.jumpLatest?.(graphLatest.dataset.resultGraphLatest);else if(graphSource)root.NetworkToolkitResultTimeline?.selectResult?.(graphSource.dataset.resultGraphId,graphSource.dataset.resultGraphSource);else if(sideDetail){sideDetail.contributorPage=Number(graphPage.dataset.resultGraphPage)||0;afterRender();}return;}
      if(event.target.closest('[data-result-side-close]')&&sideDetail){event.preventDefault();event.stopImmediatePropagation();toggleDetails(true);return;}
      if(event.target.closest('[data-result-side-show]')&&sideDetail){event.preventDefault();event.stopImmediatePropagation();toggleDetails(false);return;}
      if(event.target.closest('[data-result-side-expand]')&&sideDetail){event.preventDefault();event.stopImmediatePropagation();openRow(sideDetail,null,true);return;}
      const copy = event.target.closest('[data-result-copy]');
      const detailForCopy=copy?.closest('#resultEntryPane')?sideDetail:activeDetail;
      if (copy && detailForCopy && copy.closest('.result-row-detail')) {
        event.preventDefault(); event.stopImmediatePropagation();
        const record = detailForCopy.record, text = copy.dataset.resultCopy === 'json' || !record || typeof record !== 'object' ? JSON.stringify(record,null,2) : Object.entries(record).map(([key,value]) => label(key) + ': ' + (value != null && typeof value === 'object' ? JSON.stringify(value, null, 2) : root.NetworkToolkitResultTime?.isKey(key)?root.NetworkToolkitResultTime.format(value)+' ('+String(value??'Not recorded')+')':scalar(value))).join('\n');
        void copyText(text); return;
      }
      if (event.target.closest('[data-result-back]')) { event.preventDefault(); event.stopImmediatePropagation(); returnToPrevious(); return; }
      const row = candidate(event.target);
      if (!row || event.defaultPrevented || event.button !== 0 || event.ctrlKey || event.altKey || event.metaKey || event.shiftKey || event.detail > 1 || root.getSelection()?.isCollapsed === false) return;
      if (pointerStart?.row === row && (Math.abs(event.clientX-pointerStart.x)>5 || Math.abs(event.clientY-pointerStart.y)>5)) return;
      if(row.dataset.resultGraphId&&row.dataset.resultSampleKey&&root.NetworkToolkitResultTimeline?.selectResult?.(row.dataset.resultGraphId,row.dataset.resultSampleKey)){event.preventDefault();event.stopImmediatePropagation();return;}
      const detail = row.hasAttribute('data-result-row') ? captureRow(row) : null, view = viewToken(), parentModal = row.closest('.modal');
      // Let double-click or drag selection finish before opening a row dialog.
      pendingClick = setTimeout(() => { pendingClick = 0; if (viewToken() === view && parentModal === document.querySelector('.modal') && root.getSelection()?.isCollapsed !== false) showCandidate(row, detail); }, 250);
    }, true);
    document.addEventListener('keydown', event => {
      if(event.target.id==='resultEntryDivider'&&sideDetail&&['Enter',' ','ArrowLeft','ArrowRight','ArrowUp','ArrowDown','Home'].includes(event.key)){
        event.preventDefault();event.stopImmediatePropagation();if(event.key==='Enter'||event.key===' '){toggleDetails(undefined,false);return;}
        const area=document.getElementById('toolResult'),bottom=area.classList.contains('entry-details-bottom');if(bottom&&!['ArrowUp','ArrowDown','Home'].includes(event.key)||!bottom&&!['ArrowLeft','ArrowRight','Home'].includes(event.key))return;
        if(preferences().hidden){preferences().hidden=false;layoutDetails();}const name=bottom?'height':'width';preferences()[name]=event.key==='Home'?defaults[name]:Math.round(clamp(Number(event.target.getAttribute('aria-valuenow'))+(['ArrowLeft','ArrowUp'].includes(event.key)?1:-1)*(event.shiftKey?30:10),Number(event.target.getAttribute('aria-valuemin')),Number(event.target.getAttribute('aria-valuemax'))));if(sideDetail.graph&&bottom)preferences().graphHeight=preferences().height;layoutDetails();persist();return;
      }
      if (event.key === 'Escape') {cancelPending();if(sideDetail&&!document.querySelector('.modal')){event.preventDefault();event.stopImmediatePropagation();if(sideDetail.graph)root.NetworkToolkitResultTimeline?.clearSelection?.(sideDetail.graph.id);else closeSide(true);return;}}
      const row = candidate(event.target);
      if (!row || event.target !== row || !['Enter',' '].includes(event.key) || event.ctrlKey || event.altKey || event.metaKey || event.shiftKey) return;
      cancelPending(); event.preventDefault(); event.stopImmediatePropagation(); showCandidate(row);
    }, true);
  }

  root.NetworkToolkitResultDetails = Object.freeze({init,rowAttributes,openRow,afterRender,closeSide,captureRow,mergePreferences,layoutDetails,showGraphSelection,clearGraphSelection});
})(globalThis);
