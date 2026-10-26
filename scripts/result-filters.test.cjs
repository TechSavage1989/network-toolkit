'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(require.resolve('../web/assets/result-filters.js'),'utf8'),html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
function harness(){
 const events=[],refreshes=[],legacy={},context={state:{resultFilters:{}},document:{addEventListener(){},dispatchEvent(event){events.push(event);}},CustomEvent:class{constructor(type,options){this.type=type;this.detail=options.detail;}},saveState(){context.saved=(context.saved||0)+1;},listPreference(view,defaults={}){return {query:'',status:'all',sort:'sequence',direction:'desc',...defaults,...legacy[view]};},saveListPreference(view,value){legacy[view]={...value};},dataListPages:new Map(),refreshDataList(id,controls){refreshes.push({id,controls,eventCount:events.length});},console};
 context.window=context;vm.createContext(context);vm.runInContext(source,context);return {context,api:context.NetworkToolkitResultFilters,legacy,events,refreshes};
}
function config(rows,overrides={}){return {id:'sample-list',view:'result:ping:samples',name:'samples',graphId:'run',job:{id:'run',tool:'ping',options:{mode:'icmp'}},columns:[{id:'sequence',value:row=>row.sequence},{id:'status',value:row=>row.status},{id:'address',value:row=>row.address}],rows,search:row=>JSON.stringify(row),...overrides};}
function choose(h,c,kind,value){h.api.handleChange({target:{value,dataset:{[kind==='status'?'resultFilterStatus':'resultFilterType']:c.id},hasAttribute:attr=>attr==='data-result-filter-'+kind}});}
function filtered(h,c){return c.rows.filter(row=>h.api.graphMatches(c.job.id,row)).map(row=>row.sequence);}
function sample(sequence,rttMs,status='reply',extra={}){return {sequence,rttMs,status,success:status==='reply',time:'2026-10-26T12:00:'+String(sequence).padStart(2,'0')+'Z',...extra};}

test('detailed status, type, text and statistic filters intersect without dropping earlier controls',()=>{
 const h=harness(),rows=[sample(1,2,'reply',{protocol:'TCP',address:'alpha'}),sample(2,null,'timeout',{protocol:'TCP',address:'alpha'}),sample(3,null,'timeout',{protocol:'ICMP',address:'alpha'}),sample(4,null,'timeout',{protocol:'TCP',address:'beta'})],c=h.api.register(config(rows));
 choose(h,c,'status','timeout');choose(h,c,'type','tcp');h.api.setStatistic('run','loss');h.legacy[c.view]={query:'alpha',sort:'sequence',direction:'desc'};
 assert.deepEqual(filtered(h,c),[2]);assert.equal(h.api.current('run').status,'timeout');assert.equal(h.api.current('run').type,'tcp');assert.equal(h.api.current('run').statistic,'loss');
 h.api.setStatistic('run','loss');assert.equal(h.api.current('run').statistic,'');assert.equal(h.api.current('run').status,'timeout');assert.deepEqual(filtered(h,c),[2]);
 assert(h.refreshes.length>=4);assert(h.refreshes.every(refresh=>refresh.controls===true));assert.equal(h.events.at(-1).detail.jobId,'run');assert.equal(h.refreshes.at(-1).eventCount,h.events.length-1,'the filter event follows refreshed rows');
});

test('actual lifetime minimum and maximum retain ties and never substitute an evicted measurement',()=>{
 const h=harness(),rows=[sample(1,7),sample(2,7),sample(3,30),sample(4,30),sample(5,9,'timeout')],c=h.api.register(config(rows,{statistics:{minMs:7,maxMs:30,avgMs:18.5,jitterMs:23}}));
 h.api.setStatistic('run','minimum');assert.deepEqual(filtered(h,c),[1,2]);assert.equal(h.api.current('run').reference,7);assert.equal(h.api.current('run').referenceKind,'run');
 h.api.setStatistic('run','maximum');assert.deepEqual(filtered(h,c),[3,4]);
 const retained=h.api.register(config(rows.slice(2),{statistics:{minMs:7,maxMs:30,avgMs:18.5,jitterMs:23}}));h.api.setStatistic('run','minimum');assert.deepEqual(filtered(h,retained),[]);assert.match(h.api.current('run').message,/not present in retained/);assert.equal(h.api.current('run').reference,7);
});

test('average filters measured successful contributors, while aggregate reference remains truthful',()=>{
 const h=harness(),rows=[sample(1,0),sample(2,10),sample(3,null),sample(4,'', 'reply'),sample(5,500,'timeout'),sample(6,true),sample(7,-3),sample(8,'20')],c=h.api.register(config(rows,{statistics:{avgMs:50}})),before=JSON.stringify(rows);
 h.api.setStatistic('run','average');assert.deepEqual(filtered(h,c),[1,2,8]);assert.equal(h.api.current('run').reference,50);assert.match(h.api.current('run').message,/aggregate/);assert.equal(JSON.stringify(rows),before,'filtering does not change recorded samples');
});

test('RTT variability shows only both members of real adjacent measured pairs, with no loss or sequence gaps bridged',()=>{
 const h=harness(),rows=[sample(1,2),sample(2,4),sample(3,null,'timeout'),sample(4,5),sample(6,8),sample(7,12),sample(8,null),sample(9,5)],c=h.api.register(config(rows,{statistics:{jitterMs:3}}));
 h.api.setStatistic('run','jitter');assert.deepEqual(filtered(h,c),[1,2,6,7]);assert.equal(h.api.current('run').reference,3);
 assert.equal(h.api.graphMatches('run',{...rows[0]}),true,'graph copies retain sample identity');assert.equal(h.api.graphMatches('run',{...rows[3]}),false);
 const isolated=h.api.register(config([sample(1,2),sample(3,4)]));assert.deepEqual(filtered(h,isolated),[]);assert.match(h.api.current('run').message,/No consecutive/);
});

test('unknown data does not invent a success or loss, and severity follows outcomes instead of latency thresholds',()=>{
 const h=harness(),rows=[sample(1,100000),sample(2,null,'timeout'),{sequence:3,status:'unknown'},{sequence:4,status:'cancelled',success:false}],c=h.api.register(config(rows));
 h.api.setStatistic('run','received');assert.deepEqual(filtered(h,c),[1]);h.api.setStatistic('run','loss');assert.deepEqual(filtered(h,c),[2,4]);h.api.setStatistic('run','sent');assert.deepEqual(filtered(h,c),[1,2,3,4]);
 assert.equal(h.api.severity(rows[0],'ping'),'good');assert.equal(h.api.severity(rows[1],'ping'),'bad');assert.equal(h.api.severity(rows[2],'ping'),'info');assert.equal(h.api.severity(rows[3],'ping'),'warn');assert.equal(h.api.severity({status:'ttl_expired',address:'192.0.2.1'},'trace'),'info');
});

test('types are tool-specific and detailed statuses normalize without hiding meaningful distinctions',()=>{
 const h=harness();assert.equal(h.api.status({status:'Timed out'},'ping'),'timeout');assert.equal(h.api.status({status:404},'http'),'http_404');assert.equal(h.api.status({responded:false},'discover'),'no_response');
 const c=h.api.register(config([{type:'A',status:'ok'},{type:'AAAA',status:'ok'},{type:'MX',status:'error'}],{view:'result:dns:records',job:{id:'dns-run',tool:'dns'}}));
 choose(h,c,'type','aaaa');assert.equal(c.rows.filter(row=>h.api.matches(row,c)).length,1);assert.match(h.api.controls(c),/data-result-filter-type/);assert.equal(h.api.type({},config([])),'icmp');assert.equal(h.api.type({},config([],{job:{tool:'tcp'}})),'tcp');
});

test('clear filters resets all filter families while preserving active sort, original rows and independent views',()=>{
 const h=harness(),rows=[sample(1,2),sample(2,null,'timeout')],c=h.api.register(config(rows)),other=h.api.register(config(rows,{id:'other',view:'result:monitor:samples',job:{id:'other-run',tool:'monitor'}}));
 h.legacy[c.view]={query:'needle',status:'no_response',sort:'address',direction:'asc'};choose(h,c,'status','timeout');h.api.setStatistic('run','loss');choose(h,other,'status','reply');
 h.api.clear(c);assert.equal(h.api.current('run').active,false);assert.equal(h.legacy[c.view].sort,'address');assert.equal(h.legacy[c.view].direction,'asc');assert.equal(h.legacy[c.view].query,'');assert.equal(h.legacy[c.view].status,'all');assert.equal(h.api.current('other-run').status,'reply');assert.equal(c.rows,rows);
});

test('preference restore validates known fields, strips payloads and rejects unsafe views and unknown statistics',()=>{
 const h=harness(),saved=JSON.parse('{"resultFilters":{"result:ping:samples":{"status":"Timed Out","type":"TCP","statistic":"minimum","rows":[{"secret":"discard"}],"password":"discard"},"__proto__":{"status":"error"},"invalid":{"status":{},"type":[],"statistic":"invented"}}}');h.api.mergePreferences(saved);
 assert.equal(h.api.preferences('result:ping:samples').status,'timeout');assert.equal(h.api.preferences('result:ping:samples').type,'tcp');assert.equal(h.api.preferences('result:ping:samples').statistic,'minimum');assert.equal(h.api.preferences('invalid').status,'all');assert.equal(h.api.preferences('invalid').statistic,'');assert.equal(Object.hasOwn(h.context.state.resultFilters,'__proto__'),false);assert.equal(JSON.stringify(h.context.state).includes('discard'),false);
 const many={};for(let i=0;i<190;i++)many['view-'+i]={status:'error'};h.api.mergePreferences({resultFilters:many});assert.equal(Object.keys(h.context.state.resultFilters).length,160);h.api.mergePreferences({resultFilters:[]});assert.equal(Object.keys(h.context.state.resultFilters).length,0);
 assert.equal(h.api.graphMatches('unregistered',{}),true);assert.equal(h.api.current('unregistered').active,false);
});

test('real generic list rows apply detailed and text filters before an arbitrary active sort without changing raw rows',()=>{
 const h=harness(),rows=[sample(1,8,'reply',{address:'beta'}),sample(2,6,'reply',{address:'alpha'}),sample(3,null,'timeout',{address:'alpha'}),sample(4,6,'reply',{address:'alpha'})],before=JSON.stringify(rows),c=h.api.register(config(rows));
 h.context.sortDataRows=(values,columns,pref)=>values.slice().sort((a,b)=>(String(a[pref.sort]).localeCompare(String(b[pref.sort]),undefined,{numeric:true}))*(pref.direction==='desc'?-1:1));
 const a=html.indexOf('function dataListRows('),b=html.indexOf('\nfunction sortHeader(',a);vm.runInContext(html.slice(a,b),h.context);
 h.legacy[c.view]={query:'alpha',sort:'rttMs',direction:'desc'};h.api.setStatistic('run','minimum');assert.deepEqual(Array.from(h.context.dataListRows(c),row=>row.sequence),[2,4]);assert.equal(JSON.stringify(rows),before);
 h.legacy[c.view]={query:'alpha',sort:'sequence',direction:'desc'};h.api.clearStatistic('run');assert.deepEqual(Array.from(h.context.dataListRows(c),row=>row.sequence),[4,3,2]);
});

test('empty source and a filtered empty view have distinct generic list messages',()=>{
 const a=html.indexOf('function dataListContent('),b=html.indexOf('\nfunction dataList(',a),fn=html.slice(a,b),h=harness();
 Object.assign(h.context,{esc:String,dataListRows:c=>c.rows,sortHeader:()=>'',diagnosticRowClass:()=>'',NetworkToolkitResultDetails:{rowAttributes:()=>''},dataListFooter:()=>''});vm.runInContext(fn,h.context);
 assert.match(h.context.dataListContent(config([])),/No results have been recorded/);h.context.dataListRows=()=>[];assert.match(h.context.dataListContent(config([sample(1,2)])),/No results match the current filters/);
});

test('TCP summary actions select exact states and Checked clears summary selection while preserving query, type and sort',()=>{
 const h=harness(),rows=[{port:22,state:'open',protocol:'TCP',address:'alpha'},{port:23,state:'closed',protocol:'TCP',address:'alpha'},{port:25,state:'timeout',protocol:'TCP',address:'alpha'},{port:80,state:'open',protocol:'TCP',address:'beta'}],c=h.api.register(config(rows,{view:'result:tcp:ports',job:{id:'tcp-run',tool:'tcp'}})),before=JSON.stringify(rows);
 h.legacy[c.view]={query:'alpha',sort:'port',direction:'desc'};choose(h,c,'type','tcp');
 for(const [key,expected]of [['open',[22]],['closed',[23]],['timeout',[25]],['all',[22,23,25]]]){h.api.setStatusSummary('tcp-run',key);assert.deepEqual(rows.filter(row=>h.api.graphMatches('tcp-run',row)).map(row=>row.port),expected);assert.equal(h.api.current('tcp-run').type,'tcp');assert.equal(h.legacy[c.view].query,'alpha');assert.equal(h.legacy[c.view].sort,'port');assert.equal(h.legacy[c.view].direction,'desc');}
 h.api.setStatusSummary('tcp-run','closed');assert.match(h.api.statusSummary('tcp-run',[['closed','Closed',1,'bad']]),/data-result-status-stat="closed"[^>]*aria-pressed="true"/);h.api.clearStatusSummary('tcp-run');assert.equal(h.api.current('tcp-run').status,'all');assert.equal(h.api.current('tcp-run').type,'tcp');assert.equal(JSON.stringify(rows),before);
});

test('Quick check cards combine detailed status and meaningful check type without changing original check data',()=>{
 const h=harness(),rows=[{name:'DNS resolution',status:'passed',detail:'alpha'},{name:'ICMP reachability',status:'warning',detail:'alpha'},{name:'TCP service',status:'failed',detail:'alpha'},{name:'TCP service',status:'passed',detail:'beta'}],before=JSON.stringify(rows),c=h.api.register(config(rows,{view:'result:quick:checks',job:{id:'quick-run',tool:'quick'},card:row=>row.name}));
 choose(h,c,'type','tcp_service');choose(h,c,'status','failed');h.legacy[c.view]={query:'alpha',sort:'name',direction:'asc'};assert.deepEqual(rows.filter(row=>h.api.graphMatches('quick-run',row)).map(row=>row.name),['TCP service']);assert.match(h.api.controls(c),/data-result-filter-type/);assert.match(h.api.controls(c),/data-result-filter-status/);assert.equal(JSON.stringify(rows),before);
});

test('Discovery summary actions preserve search and sorting, expose silent rows, and finished means completed recorded rows',()=>{
 const h=harness(),rows=[{address:'192.0.2.1',hostname:'alpha',responded:true,completed:true,status:'responded'},{address:'192.0.2.2',hostname:'alpha',responded:false,completed:true,status:'no_response'},{address:'192.0.2.3',hostname:'alpha',responded:false,completed:false,status:'cancelled'},{address:'192.0.2.4',hostname:'alpha',responded:false,completed:true,status:'error'},{address:'192.0.2.5',hostname:'beta',responded:true,completed:true,status:'responded'}],before=JSON.stringify(rows),job={id:'discovery-run',tool:'discover'},filter={query:'alpha',sort:'address',direction:'desc',respondedOnly:true,status:'all',page:2};
 Object.assign(h.context,{networkResultJobs:new Map([[job.id,job]]),networkFilter:()=>filter,saveNetworkFilter(){},closeEntryContextMenu(){},$$:()=>[],requestAnimationFrame:fn=>fn(),updateToolResultSticky(){},deviceHostname:row=>row.hostname||'',rowMAC:()=>'',deviceModel:()=> 'Not reported',networkSortColumns:()=>[],sortDataRows:values=>values.slice().sort((a,b)=>b.address.localeCompare(a.address))});
 for(const name of ['filteredNetworkRows','setNetworkSummary']){const a=html.indexOf('function '+name+'('),b=html.indexOf('\nfunction ',a+10);vm.runInContext(html.slice(a,b),h.context);}
 for(const [key,expected]of [['no_response',['192.0.2.2']],['error',['192.0.2.4']],['cancelled',['192.0.2.3']],['finished',['192.0.2.4','192.0.2.2','192.0.2.1']],['responded',['192.0.2.1']],['all',['192.0.2.4','192.0.2.3','192.0.2.2','192.0.2.1']]]){h.context.setNetworkSummary(job.id,key);assert.deepEqual(Array.from(h.context.filteredNetworkRows({rows,neighbor:false},filter),row=>row.address),expected);assert.equal(filter.query,'alpha');assert.equal(filter.sort,'address');assert.equal(filter.direction,'desc');assert.equal(filter.respondedOnly,false);assert.equal(filter.page,0);}
 assert.equal(JSON.stringify(rows),before);assert.match(html,/these addresses never started and have no recorded result row/);
});

test('recorded-probe ICMP summaries filter the exact retained immutable address samples and use run metrics',()=>{
 const h=harness(),row={address:'192.0.2.8',samples:[sample(1,5),sample(2,5),sample(3,null,'timeout'),sample(4,30)],ports:[],statistics:{sent:4,received:3,lossPercent:25,minMs:5,avgMs:40/3,maxMs:30},_probeFormat:'full'},before=JSON.stringify(row),key='recorded-device';
 Object.assign(h.context,{deviceRecords:new Map([[key,row]]),deviceTarget:r=>r.address,networkColumnCell:()=>'',hostnameDetailsHTML:()=>'',kv:()=>'',esc:String,fmt:value=>String(value),icon:()=>'',label:String,pill:String,typedColumn:(id,name,value,render,type)=>({id,name,value:value||((r)=>r[id]),render,type}),dataList:c=>{h.api.register(c);return '<div data-retained-list>'+c.rows.length+'</div>';},compactStats:()=>'',openModal:(title,body)=>{h.context.modalBody=body;},registerProbePort(){},entryToolsButton(){}});
 const a=html.indexOf('function openProbeDetails('),b=html.indexOf('\nfunction openDeviceTools',a);vm.runInContext(html.slice(a,b),h.context);h.context.openProbeDetails(key);
 assert.match(h.context.modalBody,/data-result-stat="minimum"/);const id='probe:'+key+':icmp';h.api.setStatistic(id,'minimum');assert.deepEqual(row.samples.filter(sample=>h.api.graphMatches(id,sample)).map(sample=>sample.sequence),[1,2]);h.api.setStatistic(id,'loss');assert.deepEqual(row.samples.filter(sample=>h.api.graphMatches(id,sample)).map(sample=>sample.sequence),[3]);h.api.setStatistic(id,'average');assert.equal(h.api.current(id).reference,40/3);assert.equal(JSON.stringify(row),before);
});

test('cancelled samples use the warning palette consistently without legacy negative row-cell classes',()=>{
 const h=harness(),row=sample(1,null,'cancelled'),c=h.api.register(config([row]));assert.equal(h.api.severity(row,'ping'),'warn');assert.match(h.api.pill(row,'ping',row.status),/^<span class="pill warn">cancelled<\/span>$/);
 Object.assign(h.context,{esc:String,scalar:String,dataListRows:c=>c.rows,sortHeader:()=>'',diagnosticRowClass:()=> 'diagnostic-bad-row',NetworkToolkitResultDetails:{rowAttributes:()=>''},dataListFooter:()=>''});const a=html.indexOf('function dataListContent('),b=html.indexOf('\nfunction dataList(',a);vm.runInContext(html.slice(a,b),h.context);const rendered=h.context.dataListContent(c);assert.match(rendered,/<tr class="result-filter-row-warn"/);assert.doesNotMatch(rendered,/diagnostic-bad-row/);assert.match(html,/NetworkToolkitResultFilters\.pill\(row,j\.tool,row\.status/);
});

test('TCP and recorded-probe timeout labels match negative rows while trace destinations and intermediate hops retain their meanings',()=>{
 const h=harness();for(const tool of ['tcp','ping','trace']){assert.equal(h.api.severity({state:'timeout'},tool),'bad');assert.match(h.api.pill({state:'timeout'},tool,'Timeout'),/class="pill bad"/);}assert.match(h.api.pill({status:'cancelled',success:false},'ping','Cancelled'),/class="pill warn"/);assert.equal(h.api.severity({status:'reply',address:'192.0.2.1',reached:false},'trace'),'info');assert.match(h.api.pill({status:'ttl_expired',address:'192.0.2.1'},'trace','Hop replied'),/class="pill info"/);assert.match(h.api.pill({status:'reply',address:'192.0.2.9',reached:true},'trace','Destination'),/class="pill good"/);
 assert.match(html,/NetworkToolkitResultFilters\.pill\(row,'tcp',label\(row\.state\)\)/);assert.match(html,/NetworkToolkitResultFilters\.pill\(row,'trace',row\.reached\?/);assert.match(html,/NetworkToolkitResultFilters\.pill\(sample,'ping',sample\.success\?/);assert.match(html,/NetworkToolkitResultFilters\.pill\(port,'tcp',label\(port\.state\)\)/);
});
