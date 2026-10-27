'use strict';
// Shipped UI with isolated API fixtures. No devices, user files or installations are changed.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT;
const report={passed:false,checks:[],layouts:[],preferences:[],interactions:[],packets:[],errors:[],assetFailures:[]};
let savedState={appUpdatePreferences:{enabled:false}};
let browser,page;const check=name=>{report.checks.push(name);console.log('PASS '+name);};
const before='2026-11-01T05:59:59.125Z',after='2026-11-01T06:00:00.625Z',stamp='2026-11-01T06:00:03.125Z';
const samples=[{sequence:1,time:before,address:'192.0.2.20',success:true,rttMs:10,status:'reply',ttl:64},{sequence:2,time:after,address:'192.0.2.20',success:true,rttMs:14,status:'reply',ttl:64},{sequence:3,time:'2026-11-01T06:00:01.125Z',address:'192.0.2.20',success:false,rttMs:999,status:'timeout',error:'No reply <fixture>'},{sequence:4,time:'2026-11-01T06:00:02.125Z',address:'192.0.2.20',success:true,rttMs:30,status:'reply'},{sequence:5,address:'192.0.2.20',success:true,rttMs:20,status:'legacy reply'}];
function job(tool,data,id='ui93-'+tool){return {id,tool,target:tool==='http'?'https://example.test/':tool==='discover'?'192.0.2.0/24':'192.0.2.20',status:'completed',started:'2026-10-01T00:00:00Z',finished:'2026-10-01T00:01:00Z',options:{count:5},events:[],result:{summary:'Timestamp fixture',time:stamp,data}};}
const fixtures=[job('ping',{samples}),job('monitor',{mode:'tcp',port:443,samples}),job('trace',{address:'192.0.2.20',reached:true,hops:[{hop:1,address:'192.0.2.20',status:'reply',reached:true,rttMs:2,samples:[],time:after}]}),job('tcp',{ports:[{port:443,address:'192.0.2.20',state:'open',latencyMs:2,time:after}]}),job('dns',{query:'example.test',server:'192.0.2.53',elapsedMs:2,records:[{type:'A',name:'example.test',value:'192.0.2.20',ttl:300,time:after}],queries:[{type:'A',status:'ok',durationMs:2,time:after}]}),job('discover',{scope:'subnet',total:1,completed:1,responded:1,hosts:[{address:'192.0.2.20',hostname:'fixture.test',responded:true,completed:true,status:'responded',openPorts:[443],time:after}]}),job('connections',{time:after,timeKind:'snapshot',connections:[{local:'192.0.2.4:53000',remote:'192.0.2.20:443',pid:23,protocol:'TCP',state:'ESTABLISHED'}]}),job('routes',{time:after,timeKind:'snapshot',routes:[{destination:'192.0.2.0/24',gateway:'192.0.2.1',interface:'Ethernet',metric:10}]}),job('http',{time:after,statusCode:200,elapsedMs:2,redirects:[{url:'https://example.test/',statusCode:200,time:after}]})];
const eventJob=job('monitor',undefined,'ui93-event-monitor');delete eventJob.result;eventJob.events=[{time:before,level:'success',message:'Completed probe',data:{type:'sample',value:{sequence:1,address:'192.0.2.20',success:true,rttMs:9,status:'reply'}}}];fixtures.push(eventJob);
const packetStamp='2026-11-01T06:00:00.625000001Z',packetEnd='2026-11-01T06:00:02.625000001Z';
const packetRows=[{number:1,time:packetStamp,relative:0,source:'192.0.2.10',destination:'192.0.2.20',sourcePort:53000,destinationPort:443,protocol:'TCP',length:100,captured:100,info:'First timestamped frame',expert:[]},{number:2,time:packetEnd,relative:2,source:'192.0.2.20',destination:'192.0.2.10',protocol:'UDP',length:200,captured:200,info:'Second timestamped frame',expert:[]},{number:3,time:'0001-01-01T00:00:00Z',relative:3,source:'192.0.2.10',destination:'192.0.2.20',protocol:'TCP',length:64,captured:64,info:'Legacy frame with unknown time',expert:[]}];
const capture={id:'a'.repeat(24),name:'Graph interaction capture',state:'completed',engine:'import',count:3,bytes:364,lost:0,overwritten:0,revision:1,started:'2026-10-26T00:00:00Z',options:{}};
let packetStats={protocols:{TCP:2,UDP:1},endpoints:[],conversations:[],bytes:364,timedBytes:300,expert:0,duration:2,started:packetStamp,finished:packetEnd,timedPackets:2,untimedPackets:1,timeline:[{seconds:0,packets:1,bytes:100,time:packetStamp,endTime:'2026-11-01T06:00:01.625000001Z',intervalSeconds:1},{seconds:1,packets:1,bytes:200,time:'2026-11-01T06:00:01.625000001Z',endTime:packetEnd,intervalSeconds:1}]};
const packetCSV='number,time,relative,protocol\r\n1,'+packetStamp+',0,TCP\r\n2,'+packetEnd+',2,UDP\r\n';
const settle=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
async function show(tool,id='ui93-'+tool){await page.evaluate(({tool,id,fixtures})=>{jobs=fixtures;selected[tool]=id;if(LOCAL_TOOLS.includes(tool)){state.activePage='local';state.localTab=tool;renderPage();}else navigate(tool);renderCurrentResult(true);},{tool,id,fixtures});await settle();const results=page.locator('#toolResultsTab');if(await results.isVisible()&&await results.getAttribute('aria-selected')!=='true'){await results.click();await settle();}}
async function openContext(timezone){
 const context=await browser.newContext({viewport:{width:1440,height:1000},locale:'en-US',timezoneId:timezone,permissions:['clipboard-read','clipboard-write'],acceptDownloads:true,serviceWorkers:'block'});page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',error=>report.errors.push({timezone,message:error.message}));page.on('response',response=>{if(response.url().includes('/assets/')&&response.status()>=400)report.assetFailures.push(response.url());});
 await context.route('**/*',async route=>{const request=route.request(),url=new URL(request.url()),p=url.pathname;assert.equal(url.origin,'https://ui93.toolkit.test','Unexpected external request');let data={};
  if(p.startsWith('/api/')){
   if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',system:{hostname:'UI fixture',interfaces:[]}};
   if(p==='/api/state'){if(request.method()==='GET')data=savedState;else{savedState=request.postDataJSON();data=savedState;}}
   if(p==='/api/system')data={hostname:'UI fixture',interfaces:[]};
   if(p==='/api/jobs')data=request.method()==='POST'?fixtures.find(j=>j.tool===request.postDataJSON().tool)||fixtures[0]:fixtures;
   const full=p.match(/^\/api\/jobs\/([^/]+)$/);if(full)data=fixtures.find(j=>j.id===full[1])||{};
   if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};if(p==='/api/terminal-views')data={available:false,owners:{},views:[]};
   if(p.startsWith('/api/public-ip'))data={provider:'fixture',checkedAt:stamp,expiresAt:'2026-11-02T00:00:00Z',ipv4:{status:'ok',address:'192.0.2.45'},ipv6:{status:'unavailable',error:'No IPv6 fixture'}};
   if(p==='/api/local-tools/ip')data={available:true,time:after,adapters:[]};
   if(p==='/api/packet-captures')data={captures:[capture]};
   if(p==='/api/packet-captures/capabilities')data={available:true,note:'Isolated imported capture',interfaces:[{id:'nics',name:'All adapters',engine:'auto'}]};
   if(p==='/api/packet-captures/interfaces')data={interfaces:[{id:'nics',name:'All adapters',engine:'auto'}]};
   if(p.endsWith('/troubleshoot'))data={packetCount:3,duration:2,summary:'Timestamp fixture',scope:'All retained packets.',metrics:{},findings:[],limitations:[]};
   if(p.endsWith('/statistics'))data=packetStats;
   if(p.endsWith('/overview'))data={bins:[],packetMarks:'ABC',signals:[],interesting:0,counts:{error:0,warn:0,note:0}};
   if(p.endsWith('/packets')){const offset=Number(url.searchParams.get('offset')||0),limit=Number(url.searchParams.get('limit')||300);data={capture,total:packetRows.length,offset,packets:packetRows.slice(offset,offset+limit)};}
   const packet=p.match(/\/packet\/(\d+)$/);if(packet)data={packet:packetRows.find(row=>row.number===Number(packet[1])),layers:[],data:'',ascii:''};
   if(p.endsWith('/export'))return route.fulfill({contentType:'text/csv',body:packetCSV});
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,50));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','ui93').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto('https://ui93.toolkit.test/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);return context;
}
async function assertNoPageOverflow(label){const value=await page.evaluate(()=>({window:innerWidth,scroll:document.documentElement.scrollWidth,frame:document.querySelector('.main').getBoundingClientRect().bottom,height:innerHeight}));assert(value.scroll<=value.window+1,label+' horizontal page overflow '+JSON.stringify(value));assert(value.frame<=value.height+2,label+' app frame exceeds window');return value;}
async function pointDetails(){const sidebar=page.locator('#resultEntryBody .result-graph-summary'),target=await sidebar.count()?sidebar:page.locator('[data-rt-tooltip]');return target.first().evaluate(el=>Object.fromEntries([...el.querySelectorAll('dl>div')].map(row=>[row.querySelector('dt').textContent,row.querySelector('dd').textContent])));}

const timeline=()=>page.locator('.rt-timeline[data-rt-id="ui93-monitor"]');
const marker=sequence=>timeline().locator('[data-rt-surface="latency"] [data-rt-sequence="'+sequence+'"]');
const sampleRow=sequence=>page.locator('tr[data-result-graph-id="ui93-monitor"][data-result-sequence="'+sequence+'"]');
const list=()=>page.locator('[data-graph-workspace="ui93-monitor"] [data-graph-results] .table-wrap');
const packetGraph=()=>page.locator('#packetGraphDock [data-packet-timeline]');
const packetBucket=index=>packetGraph().locator('[data-packet-bucket="'+index+'"]');
const popup=()=>page.locator('[data-rt-tooltip]');
const isSelected=async sequence=>assert.equal((await pointDetails()).Sample,String(sequence),'Explicit selection stays on the chosen sample');
const recorded=n=>new Date(Date.parse(after)+n*1000).toISOString();
const replies=count=>Array.from({length:count},(_,i)=>({sequence:i+1,time:recorded(i),address:i===339?'198.51.100.77':'192.0.2.20',success:i%11!==6,rttMs:i%11===6?999:5+i%47,status:i%11===6?'timeout':'reply'}));

async function replaceSamples(rows){
 fixtures.find(job=>job.id==='ui93-monitor').result.data.samples=rows;
 await page.evaluate(rows=>{jobs.find(job=>job.id==='ui93-monitor').result.data.samples=rows;renderCurrentResult(true);},rows);
 await settle();
}

async function popupBounds(locator,label){
 await locator.waitFor({state:'visible'});
 const box=await locator.evaluate(el=>{const b=el.getBoundingClientRect(),s=getComputedStyle(el);return {left:b.left,right:b.right,top:b.top,bottom:b.bottom,width:b.width,height:b.height,viewportWidth:innerWidth,viewportHeight:innerHeight,pointer:s.pointerEvents,position:s.position,text:el.textContent};});
 assert(box.width>100&&box.height>30,label+' has readable details');
 assert(box.left>=-1&&box.top>=-1&&box.right<=box.viewportWidth+1&&box.bottom<=box.viewportHeight+1,label+' is contained in the viewport '+JSON.stringify(box));
 assert.equal(box.pointer,'none',label+' never intercepts the pointer');
 assert(!/NaN|Infinity/.test(box.text),label+' contains no invalid values');
 return box;
}

async function chooseTimestamp({hourFormat='24',zoneMode='local',timeZone='UTC'}){
 await page.locator('#settingsButton').click();
 await page.locator('#settingsTimeFormat').selectOption(hourFormat);
 await page.locator('#settingsTimeZoneMode').selectOption(zoneMode);
 if(zoneMode==='named'){
  await page.locator('#settingsTimeZone').fill(timeZone);
  await page.locator('#settingsTimeZone').dispatchEvent('change');
 }
 const preview=await page.locator('#settingsTimePreview').textContent();
 assert(preview&&preview!=='Not recorded','Settings display a valid timestamp preview');
 await page.evaluate(()=>closeModal());await settle();
 const prefs=await page.evaluate(()=>NetworkToolkitResultTime.preferences());
 assert.equal(prefs.hourFormat,hourFormat);assert.equal(prefs.zoneMode,zoneMode);
 if(zoneMode==='named')assert.equal(prefs.timeZone,timeZone);
 return {prefs,preview};
}

async function assertDisplayedClock(expected,{hourFormat,zoneMode,timeZone}){
 const table=page.locator('#toolResult [data-column-view="result:monitor:samples"]');
 const cell=table.locator('time[datetime="'+after+'"]');await cell.waitFor();
 const text=await cell.textContent();assert(text.replace(/\s+/g,' ').includes(expected),'Result clock reflects the configured representation: '+text);
 assert.equal(await cell.getAttribute('data-copy-value'),text,'Displayed clock copy reflects its configured representation');
 assert((await cell.getAttribute('title')).includes(after),'Exact recorded timestamp stays available');
 await marker(2).hover({force:true});const tip=await popupBounds(popup(),'Latency hover popup');assert(tip.text.replace(/\s+/g,' ').includes(expected),'Hover popup follows timestamp preferences');
 await marker(2).click({force:true});await page.mouse.move(1,1);await settle();
 assert(Object.values(await pointDetails()).some(value=>value.replace(/\s+/g,' ').includes(expected)),'Pinned details follow timestamp preferences');
 return {hourFormat,zoneMode,timeZone,text,popup:tip.text};
}

async function latencyGeometry(label){
 await list().evaluate(el=>{for(const node of [el,...el.querySelectorAll('*')])if(node.scrollTop&&['auto','scroll'].includes(getComputedStyle(node).overflowY))node.scrollTop=0;});await settle();
 await list().scrollIntoViewIfNeeded();await settle();
 const frame=await assertNoPageOverflow(label);
 const geometry=await page.locator('[data-graph-workspace="ui93-monitor"]').evaluate(workspace=>{
  const rect=el=>{const b=el.getBoundingClientRect();return {left:b.left,right:b.right,top:b.top,bottom:b.bottom,width:b.width,height:b.height};};
  const graph=workspace.querySelector('[data-graph-pane]'),results=workspace.querySelector('[data-graph-results]'),panel=workspace.closest('.panel'),title=panel.querySelector('.result-title h2'),source=[...panel.querySelectorAll('.job-source-label')].find(el=>el.getClientRects().length)||panel.querySelector('.job-source-label');
  const textBoxes=el=>{const r=document.createRange();r.selectNodeContents(el);return [...r.getClientRects()].filter(r=>r.width&&r.height).map(rect=>({left:rect.left,right:rect.right}));};
  const stats=[...workspace.querySelectorAll('[data-graph-statistics] .result-stat')].map(card=>({card:rect(card),text:[...textBoxes(card.querySelector('strong')),...textBoxes(card.querySelector('span'))]}));
  const wrap=results.querySelector('.table-wrap'),table=wrap.querySelector('table');let clipTop=0,clipBottom=innerHeight;
  for(let node=wrap;node;node=node.parentElement){if(['auto','scroll','hidden','clip'].includes(getComputedStyle(node).overflowY)){const b=node.getBoundingClientRect();clipTop=Math.max(clipTop,b.top+node.clientTop);clipBottom=Math.min(clipBottom,b.top+node.clientTop+node.clientHeight);}}
  const headers=[...table.querySelectorAll('thead th')].map(rect).filter(b=>b.bottom>clipTop&&b.top<clipBottom),bodyTop=Math.max(clipTop,...headers.map(b=>b.bottom)),bodyBottom=clipBottom,rows=[...table.querySelectorAll('tbody tr[data-result-graph-id]')].map(rect),completeRows=rows.filter(b=>b.top>=bodyTop-1&&b.bottom<=bodyBottom+1).length;
  const graphRect=rect(graph);let graphTop=Math.max(0,graphRect.top),graphBottom=Math.min(innerHeight,graphRect.bottom);for(let node=graph.parentElement;node;node=node.parentElement){if(['auto','scroll','hidden','clip'].includes(getComputedStyle(node).overflowY)){const b=node.getBoundingClientRect();graphTop=Math.max(graphTop,b.top+node.clientTop);graphBottom=Math.min(graphBottom,b.top+node.clientTop+node.clientHeight);}}
  return {workspace:rect(workspace),graph:graphRect,visibleGraphHeight:Math.max(0,graphBottom-graphTop),results:rect(results),reader:rect(wrap),title:rect(title),titleLineHeight:parseFloat(getComputedStyle(title).lineHeight),source:rect(source),runDetails:(()=>{const control=panel.querySelector('.graph-run-details>summary');return control?rect(control):null;})(),scrollHeight:wrap.scrollHeight,overflow:getComputedStyle(wrap).overflowY,stats,tableBody:{top:bodyTop,bottom:bodyBottom,height:Math.max(0,bodyBottom-bodyTop),completeRows,rowHeight:rows[0]?.height||0}};
 });
 assert(geometry.title.width>=Math.min(120,geometry.workspace.width-24)&&geometry.title.height<=3*geometry.titleLineHeight+1,label+' has a readable results heading');
 if(frame.height>=800)assert(geometry.source.width>=Math.min(90,geometry.workspace.width-24),label+' has a readable visible source caption');else assert(geometry.runDetails&&geometry.runDetails.width>=60&&geometry.runDetails.height>=20,label+' short layout keeps the complete run context reachable through Run details');
 assert(geometry.graph.height>=Math.min(frame.height<800?100:150,geometry.workspace.height*.25),label+' graph pane is not crushed');
 assert(geometry.visibleGraphHeight>=90,label+' graph remains visible while its result rows are accessible '+JSON.stringify(geometry));
 assert(geometry.results.height>=Math.min(120,geometry.workspace.height*.25),label+' list pane is not crushed');
 assert(geometry.results.top>=geometry.graph.bottom-1,label+' graph remains above its associated list');
 assert(geometry.scrollHeight>geometry.reader.height+30,label+' result list has independent scrolling');
 assert(['auto','scroll'].includes(geometry.overflow),label+' result list owns its vertical scrollbar');
 const minimumRows=frame.height<800?2:3;report.lastMonitorGeometry={label,...geometry};
 assert(geometry.tableBody.completeRows>=minimumRows,label+' has at least'+minimumRows+' complete visible sample rows after toolbar, headers and footer '+JSON.stringify(geometry.tableBody));
 assert.equal(geometry.stats.length,7,label+' keeps every run statistic available');
 for(const item of geometry.stats)for(const text of item.text)assert(text.left>=item.card.left-1&&text.right<=item.card.right+1,label+' statistic text remains inside its tile');
 const before=geometry.graph;
 await list().evaluate(el=>{el.scrollTop=el.scrollHeight;});await settle();
 const scrolled=await page.locator('[data-graph-workspace="ui93-monitor"]').evaluate(el=>{const g=el.querySelector('[data-graph-pane]').getBoundingClientRect(),r=el.querySelector('[data-graph-results] .table-wrap');return {graphTop:g.top,graphBottom:g.bottom,listScroll:r.scrollTop};});
 assert(scrolled.listScroll>30,label+' rows can scroll to the end');
 assert(Math.abs(scrolled.graphTop-before.top)<1&&Math.abs(scrolled.graphBottom-before.bottom)<1,label+' scrolling results does not move the graph pane');
 return {frame,geometry,scrolled};
}

async function packetGeometry(label){
 await page.locator('#packetTableScroll').scrollIntoViewIfNeeded();await settle();
 const frame=await assertNoPageOverflow(label);
 const geometry=await page.evaluate(()=>{
  const rect=el=>{const b=el.getBoundingClientRect();return {left:b.left,right:b.right,top:b.top,bottom:b.bottom,width:b.width,height:b.height};};
  const graph=document.querySelector('#packetGraphDock'),reader=document.querySelector('#packetGraphReader'),table=document.querySelector('#packetTableScroll'),row=document.querySelector('#packetRows [data-packet-number]'),cell=row.querySelector('.packet-local-time'),clock=cell.querySelector('time');
  let clipTop=0,clipBottom=innerHeight;for(let node=table;node;node=node.parentElement){if(['auto','scroll','hidden','clip'].includes(getComputedStyle(node).overflowY)){const b=node.getBoundingClientRect();clipTop=Math.max(clipTop,b.top+node.clientTop);clipBottom=Math.min(clipBottom,b.top+node.clientTop+node.clientHeight);}}
  const headers=[...table.querySelectorAll('thead th')].map(rect).filter(b=>b.bottom>clipTop&&b.top<clipBottom),bodyTop=Math.max(clipTop,...headers.map(b=>b.bottom)),bodyBottom=clipBottom,rows=[...table.querySelectorAll('tbody tr[data-packet-number]')].map(rect),completeRows=rows.filter(b=>b.top>=bodyTop-1&&b.bottom<=bodyBottom+1).length;
  const graphRect=rect(graph);let graphTop=Math.max(0,graphRect.top),graphBottom=Math.min(innerHeight,graphRect.bottom);for(let node=graph.parentElement;node;node=node.parentElement){if(['auto','scroll','hidden','clip'].includes(getComputedStyle(node).overflowY)){const b=node.getBoundingClientRect();graphTop=Math.max(graphTop,b.top+node.clientTop);graphBottom=Math.min(graphBottom,b.top+node.clientTop+node.clientHeight);}}
  return {graph:graphRect,visibleGraphHeight:Math.max(0,graphBottom-graphTop),reader:rect(reader),table:rect(table),row:rect(row),cell:rect(cell),clock:rect(clock),scrollHeight:table.scrollHeight,overflow:getComputedStyle(table).overflowY,workspaceScroll:document.querySelector('#packetCaptureWorkspace').scrollTop,tableBody:{top:bodyTop,bottom:bodyBottom,height:Math.max(0,bodyBottom-bodyTop),completeRows,rowHeight:rows[0]?.height||0}};
 });
 report.lastPacketGeometry={label,...geometry};
 assert(geometry.reader.height>=90,label+' retains readable graph details '+JSON.stringify(geometry));
 assert(geometry.visibleGraphHeight>=90,label+' graph remains visible while its packet rows are accessible '+JSON.stringify(geometry));
 assert(geometry.table.height>=90,label+' retains usable packet list height '+JSON.stringify(geometry));
 const minimumRows=frame.height<800?2:3;
 assert(geometry.tableBody.completeRows>=minimumRows,label+' has at least'+minimumRows+' complete visible packet rows after header, scrollbar and surrounding panes '+JSON.stringify(geometry));
 assert(geometry.row.height<=22,label+' retains the packet virtualization row height');
 assert(geometry.clock.left>=geometry.cell.left-1&&geometry.clock.right<=geometry.cell.right+1,label+' clock text stays inside its packet column');
 assert(geometry.table.top>=geometry.graph.bottom-1,label+' packet graph stays above its list');
 assert(geometry.scrollHeight>geometry.table.height+30,label+' packet list has independent scrolling');
 const graphTop=geometry.graph.top;
 await page.locator('#packetTableScroll').evaluate(el=>{el.scrollTop=el.scrollHeight;});await settle();
 const scrolled=await page.evaluate(()=>({graphTop:document.querySelector('#packetGraphDock').getBoundingClientRect().top,scroll:document.querySelector('#packetTableScroll').scrollTop}));
 assert(scrolled.scroll>30,label+' packet list scrolls');assert(Math.abs(scrolled.graphTop-graphTop)<1,label+' packet scrolling keeps graph at the top');
 return {frame,geometry,scrolled};
}

const workspace=()=>page.locator('[data-graph-workspace="ui93-monitor"]');
async function graphAction96(name,action){await timeline().locator('[data-rt-options]').click();await page.locator('[data-rt-'+name+(action?'="'+action+'"':'')+']').click();await page.evaluate(()=>closeModal());await settle();}
async function packetAction96(name,action){await page.locator('[data-packet-graph-options]').click();await page.locator('[data-packet-'+name+(action?'="'+action+'"':'')+']').click();await page.evaluate(()=>closeModal());await settle();}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 const context=await openContext('America/New_York');await show('monitor');

 // Exercise actual settings controls, their save path and restored state.
 report.preferences.push(await chooseTimestamp({hourFormat:'12',zoneMode:'utc'}));
 report.preferences.push(await assertDisplayedClock('06:00:00.625 AM UTC',{hourFormat:'12',zoneMode:'utc'}));
 report.preferences.push(await chooseTimestamp({hourFormat:'12',zoneMode:'named',timeZone:'America/Los_Angeles'}));
 report.preferences.push(await assertDisplayedClock('11:00:00.625 PM PDT',{hourFormat:'12',zoneMode:'named',timeZone:'America/Los_Angeles'}));
 await page.locator('#settingsButton').click();const validPrefs=await page.evaluate(()=>NetworkToolkitResultTime.preferences());
 await page.locator('#settingsTimeZone').fill('Invalid/Timezone');await page.locator('#settingsTimeZone').dispatchEvent('change');
 assert.equal(await page.locator('#settingsTimeZone').getAttribute('aria-invalid'),'true','Invalid timezone receives clear validation');
 assert.deepEqual(await page.evaluate(()=>NetworkToolkitResultTime.preferences()),validPrefs,'Invalid named timezone cannot overwrite valid saved settings');
 await page.evaluate(()=>closeModal());
 await page.evaluate(()=>flushState());assert.equal(savedState.resultTimePreferences.hourFormat,'12');assert.equal(savedState.resultTimePreferences.timeZone,'America/Los_Angeles');
 await page.reload();await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await show('monitor');
 assert.deepEqual(await page.evaluate(()=>NetworkToolkitResultTime.preferences()),validPrefs,'Clock and zone preferences survive app state reload');
 report.preferences.push({restored:validPrefs,invalidZoneRejected:true});
 await chooseTimestamp({hourFormat:'24',zoneMode:'local'});report.preferences.push(await assertDisplayedClock('01:00:00.625 EST',{hourFormat:'24',zoneMode:'local'}));
 const raw=page.waitForEvent('download');await page.locator('[data-action="export-job"][data-id="ui93-monitor"]').click();await page.locator('[data-export-format="json"]').click();const rawData=JSON.parse(fs.readFileSync(await (await raw).path(),'utf8'));assert(JSON.stringify(rawData).includes(after)&&JSON.stringify(rawData).includes(before),'Formatting preferences preserve original exported instants');
 check('Timestamp settings apply12/24-hour local, UTC and named-zone displays to tables, graph details and hover popups, reject invalid zones and survive saved-state reload');

 await show('monitor');assert.equal(await page.evaluate(()=>NetworkToolkitResultTimeline.isHeld('ui93-monitor')),true,'Export preserves the held clock-test result');await graphAction96('clear');await marker(2).click({force:true});await isSelected(2);await graphAction96('clear');assert.equal(await page.evaluate(()=>NetworkToolkitResultTimeline.isHeld('ui93-monitor')),false,'Clear selection removes a freshly selected point');
 let rows=replies(240);await replaceSamples(rows);
 assert.equal(await timeline().locator('[data-rt-surface="latency"] [data-rt-point]').count(),180,'Default moving window still retains180displayed points');
 await marker(61).click({force:true});await isSelected(61);
 await marker(62).hover({force:true});const hover=await popupBounds(popup(),'Pinned graph hover popup');assert(hover.text.includes('62'),'Popup describes hovered point');await isSelected(61);
 assert(await marker(61).evaluate(el=>el.classList.contains('rt-point-selected')),'Pinned point remains visibly selected during other hover');
 await page.mouse.move(1,1);await settle();await isSelected(61);assert.equal(await popup().isVisible(),false,'Popup dismisses when the pointer leaves');
 rows=replies(340);await replaceSamples(rows);await isSelected(61);assert((await page.locator('#resultEntryBody').innerText()).includes('Selection held'),'Held evidence remains in the entry sidebar');assert(await marker(61).count(),'The held graph viewport stays anchored as new results arrive');
 assert.equal(await page.evaluate(()=>NetworkToolkitResultTimeline.isHeld('ui93-monitor')),true,'Off-window pin can still be cleared');
 report.interactions.push({kind:'persistentPin',sequence:61,hover:62,sourceRows:340,movingWindow:180,outsideWindowRetained:true});
 check('Clicked latency evidence stays selected during other hover, pointer exit and live movement beyond the moving window');

 await timeline().locator('[data-rt-mode]').selectOption('progressive');await settle();
 const progressive=await timeline().locator('[data-rt-surface]').evaluate(el=>({width:el.clientWidth,total:el.scrollWidth}));assert.equal(progressive.total,progressive.width,'Progressive history fits the shared viewport');await marker(1).click({force:true});await isSelected(1);await page.evaluate(()=>NetworkToolkitResultTimeline.pan('ui93-monitor',-.25));await settle();const manual=await timeline().locator('.rt-time-axis').innerText();rows=replies(360);await replaceSamples(rows);const retainedScroll=await timeline().locator('.rt-time-axis').innerText();assert.equal(retainedScroll,manual,'Live update preserves chosen time viewport');await isSelected(1);await workspace().locator('[data-graph-follow]').click();await settle();await isSelected(360);rows=replies(370);await replaceSamples(rows);await isSelected(370);const followed=await workspace().locator('[data-graph-follow]').getAttribute('data-follow-state');assert.equal(followed,'following');await page.evaluate(()=>flushState());assert.equal(savedState.graphPreferences.mode,'progressive');await timeline().locator('[data-rt-mode]').selectOption('window');await settle();assert.equal(await timeline().locator('[data-rt-point]').count(),180);await isSelected(370);report.preferences.push({graphMode:'progressive',persisted:true,manual,retainedScroll,followed});report.interactions.push({kind:'progressive',retained:370,progressive,manual,retainedScroll,followed});check('Progressive history fits time, preserves data panning and follows new matching results while retaining moving-window mode');

 await graphAction96('clear');
 // Correlation uses source identities through changes to list ordering/filtering.
 const resultView=page.locator('[data-graph-results] [data-list-view]');
 await resultView.locator('[data-list-sort="sequence"]').click();await resultView.locator('[data-list-sort="sequence"]').click();await settle();
 await sampleRow(370).hover();await settle();assert(await marker(370).evaluate(el=>el.classList.contains('rt-point-hovered')),'Hovering a sorted row highlights its actual graph point');
 await marker(340).hover({force:true});await settle();await sampleRow(340).waitFor();assert(await sampleRow(340).evaluate(el=>el.classList.contains('result-graph-hovered')),'Hovering a graph point highlights its actual sorted result row');
 const query=resultView.locator('[data-list-query]');await query.fill('198.51.100.77');await settle();assert.equal(await list().locator('tbody tr[data-result-graph-id]').count(),1,'Search selects the unique source sample');
 await sampleRow(340).hover();await settle();assert(await marker(340).evaluate(el=>el.classList.contains('rt-point-hovered')),'Filtered-row hover still maps by source identity');
 await query.fill('no-matching-source-fixture');await settle();await marker(340).hover({force:true});assert.equal(await query.inputValue(),'no-matching-source-fixture','Graph hover does not erase list filters');
 await query.fill('');await settle();await marker(370).click({force:true});await page.mouse.move(1,1);await settle();assert(await sampleRow(370).evaluate(el=>el.classList.contains('result-graph-selected')),'Pinned point marks its associated result row');
 report.interactions.push({kind:'linkedHover',sorted:true,filtered:true,pinnedRow:370,filterPreserved:true});
 check('Graph and result-list highlighting stays linked to the original sample through sorting, filtering and persistent selection');

 // Readable pane minima can make ArrowUp at the current lower bound a
 // correct no-op. Exercise the real keyboard range with explicit headroom.
 await page.setViewportSize({width:1440,height:1200});await show('monitor');await page.mouse.move(1,1);await page.locator('#pageTitle').click();await settle();
 const separator=page.locator('[data-graph-workspace="ui93-monitor"] [data-graph-divider]'),resizeMetrics=()=>page.locator('[data-graph-workspace="ui93-monitor"]').evaluate(node=>{const graph=node.querySelector('[data-graph-pane]'),results=node.querySelector('[data-graph-results]'),plot=graph.querySelector('[data-rt-surface="latency"]'),style=getComputedStyle(node),g=graph.getBoundingClientRect(),p=plot.getBoundingClientRect();return {viewport:{width:innerWidth,height:innerHeight},workspace:node.getBoundingClientRect().height,graph:g.height,results:results.getBoundingClientRect().height,plot:p.height,plotOffset:p.top-g.top+graph.scrollTop,grid:style.gridTemplateRows,graphMinimum:style.getPropertyValue('--graph-min-height').trim()||'190px default',listMinimum:style.getPropertyValue('--graph-list-min-height').trim(),tableMinimum:getComputedStyle(results).getPropertyValue('--graph-table-min-height').trim(),ratio:state.graphWorkspacePreferences.ratio};});
 await separator.focus();await page.keyboard.press('Home');await settle();const low=await resizeMetrics();await page.keyboard.press('End');await settle();const high=await resizeMetrics();report.resize={low,high};assert(high.graph>low.graph+5,'Keyboard separator exposes a real resize range with headroom '+JSON.stringify(report.resize));await page.keyboard.press('ArrowUp');await settle();const resized=await resizeMetrics();report.resize.afterUp=resized;assert(high.graph>resized.graph+5,'Keyboard separator physically shrinks the graph from its upper range '+JSON.stringify(report.resize));
 await page.evaluate(()=>renderCurrentResult(true));await settle();const restored=await resizeMetrics();assert(Math.abs(restored.graph-resized.graph)<3,'Pane allocation survives live render');assert.equal(restored.ratio,resized.ratio,'Pane sizing preference survives live render');report.preferences.push({layoutKeyboardResize:true,low,high,resized,restored});
 await separator.focus();await page.keyboard.press('Enter');await settle();assert.equal(await page.evaluate(()=>state.graphWorkspacePreferences.ratio),.4,'Enter resets the fixture pane ratio');await page.locator('#pageTitle').click();await page.mouse.move(1,1);await page.setViewportSize({width:1440,height:1000});await show('monitor');await settle();
 for(const viewport of [{width:1440,height:1000},{width:820,height:1000},{width:390,height:1000},{width:820,height:650}])for(const theme of ['dark','light']){
  await page.setViewportSize(viewport);await page.evaluate(theme=>setTheme(theme),theme);await show('monitor');await settle();
  await page.evaluate(()=>NetworkToolkitResultTimeline.clearSelection('ui93-monitor'));await settle();const geometry=await latencyGeometry('Monitor '+viewport.width+'x'+viewport.height+'/'+theme);
  if(out)await page.screenshot({path:path.join(out,'ui93-monitor-'+viewport.width+'x'+viewport.height+'-'+theme+'.png')});report.layouts.push({kind:'monitor',...viewport,theme,...geometry});
 }
 check('Graphs and details stay above independently scrollable result lists with usable adjustable space across desktop, compact, narrow and short layouts');

 await page.setViewportSize({width:1440,height:1000});await show('monitor');await timeline().locator('[data-rt-mode]').selectOption('progressive');const savedRatio=await page.evaluate(()=>state.graphWorkspacePreferences.ratio);await page.evaluate(()=>flushState());await page.reload();await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await show('monitor');assert.equal(await timeline().locator('[data-rt-mode]').inputValue(),'progressive','Graph view preference survives app state reload');assert.equal(await page.evaluate(()=>state.graphWorkspacePreferences.ratio),savedRatio,'Graph/list space preference survives app state reload');report.preferences.push({graphModeRestored:'progressive',paneRatioRestored:savedRatio});await timeline().locator('[data-rt-mode]').selectOption('window');

 await page.setViewportSize({width:1440,height:1000});await page.evaluate(()=>navigate('capture'));await page.waitForFunction(()=>NetworkToolkitPacketCapture.model.selected&&!NetworkToolkitPacketCapture.model.busy);await page.locator('[data-packet-view="packets"]').click();await page.locator('#packetRows [data-packet-number="1"]').waitFor();await packetBucket(0).waitFor();
 await packetBucket(0).hover({force:true});const packetTip=await popupBounds(page.locator('#packetGraphPopup'),'Packet interval popup');assert(packetTip.text.includes('01:00:00.625')&&packetTip.text.includes('100 B'),'Packet popup contains recorded interval evidence');
 assert(await page.locator('#packetRows [data-packet-number="1"]').evaluate(el=>el.classList.contains('packet-graph-hovered')),'Hovering an interval highlights associated visible packets');
 assert(!await page.locator('#packetRows [data-packet-number="2"]').evaluate(el=>el.classList.contains('packet-graph-hovered')),'Unrelated packets are not highlighted');
 await packetBucket(0).click({force:true});await packetBucket(1).hover({force:true});assert(await packetBucket(0).evaluate(el=>el.classList.contains('selected')),'Packet pin remains visibly selected during other hover');
 assert((await page.locator('#packetInspector .packet-selected-interval-details').innerText()).includes('100 B'),'Packet pin details remain on the explicitly selected interval');
 await page.locator('#packetRows [data-packet-number="2"]').hover();await settle();assert(await packetBucket(1).evaluate(el=>el.classList.contains('packet-bucket-hovered')),'Hovering a packet highlights its actual traffic interval');
 await page.locator('#packetRows [data-packet-number="3"]').hover();await settle();assert.equal(await packetGraph().locator('.packet-bucket-hovered').count(),0,'An untimed packet cannot highlight an invented interval');
 await page.mouse.move(1,1);assert((await page.locator('#packetInspector .packet-selected-interval-details').innerText()).includes('100 B'),'Packet pin survives pointer exit');
 await chooseTimestamp({hourFormat:'12',zoneMode:'utc'});const packetClock=page.locator('#packetRows [data-packet-number="1"] time');assert((await packetClock.innerText()).replace(/\s+/g,' ').includes('06:00:00.625 AM'),'Packet rows immediately reflect configured clock and timezone');assert.equal(await packetClock.getAttribute('datetime'),packetStamp,'Packet source nanoseconds remain unchanged');await packetBucket(0).hover({force:true});assert((await page.locator('#packetGraphPopup').innerText()).replace(/\s+/g,' ').includes('06:00:00.625 AM UTC'),'Packet hover popup immediately reflects timestamp settings');assert.equal(await page.evaluate(()=>NetworkToolkitPacketCapture.model.timelineState.pinned),true,'Clock settings preserve held packet selection');report.preferences.push({packetClock:'12-hour UTC',source:packetStamp,outsideControlsPreserved:true});await chooseTimestamp({hourFormat:'24',zoneMode:'local'});await packetBucket(0).click({force:true});await page.mouse.move(1,1);assert.equal(await packetBucket(0).getAttribute('aria-pressed'),'true','A fresh packet selection survives pointer exit before the retained-snapshot regression');
 const originalStats=packetStats;
 packetStats={...packetStats,timeline:[{seconds:0,packets:2,bytes:300,time:packetStamp,endTime:packetEnd,intervalSeconds:2}]};await page.evaluate(()=>NetworkToolkitPacketCapture.refresh(true));await page.waitForFunction(()=>NetworkToolkitPacketCapture.model.statistics?.timeline?.length===1);await settle();
 const retainedPacketPin=await page.locator('#packetInspector .packet-selected-interval-details').innerText();assert(retainedPacketPin.includes('100 B'),'Rebucketing preserves the selected interval snapshot instead of relabeling new data');const retainedPacketContext=await page.locator('#packetInspector').innerText();assert(/no longer|previous|snapshot|outside/i.test(retainedPacketContext),'Rebucketed pin explains its relationship to the current graph in its entry sidebar');
 report.packets.push({hoverPopup:packetTip.text,linkedRows:[1,2],untimedRow:3,retainedPacketPin,retainedPacketContext});
 packetStats={...originalStats,duration:0,finished:packetStamp,timedPackets:1,untimedPackets:2,timeline:[{seconds:0,packets:1,bytes:100,time:packetStamp,endTime:packetStamp,intervalSeconds:0}]};await page.evaluate(()=>NetworkToolkitPacketCapture.refresh(true));await page.waitForFunction(()=>NetworkToolkitPacketCapture.model.statistics?.duration===0);await packetBucket(0).hover({force:true});const zeroPopup=await popupBounds(page.locator('#packetGraphPopup'),'Zero-span interval popup');assert(zeroPopup.text.includes('Not measured'),'A zero-span popup cannot invent a rate');assert(!/NaN|Infinity/.test(zeroPopup.text));report.packets.push({zeroSpanRate:'Not measured',popup:zeroPopup.text});
 check('Packet interval popups and bidirectional highlighting preserve explicit selection, respect unknown times and retain honest selected evidence after rebucketing');

 // A larger retained capture verifies actual independent packet scrolling.
 // Leave the inspected zero-span bar before advancing the recording. Native
 // selectOption changes a control without moving the pointer, so otherwise
 // this fixture correctly retains the hovered snapshot and queues new data.
 await page.mouse.move(1,1);await settle();
 for(let i=4;i<=81;i++)packetRows.push({number:i,time:new Date(Date.parse(packetStamp)+i*3000).toISOString(),relative:i*3,source:'192.0.2.10',destination:'192.0.2.20',protocol:'TCP',length:100,captured:100,info:'Retained frame '+i,expert:[]});
 capture.count=packetRows.length;capture.bytes=packetRows.reduce((sum,row)=>sum+row.length,0);capture.revision++;
 const span=243,startMs=Date.parse(packetStamp),end=new Date(startMs+span*1000).toISOString();
 const bins=Array.from({length:60},(_,i)=>({seconds:i*span/60,packets:0,bytes:0,time:new Date(startMs+i*span/60*1000).toISOString(),endTime:new Date(startMs+(i+1)*span/60*1000).toISOString(),intervalSeconds:span/60}));
 for(const row of packetRows){const t=Date.parse(row.time);if(!Number.isFinite(t)||row.time.startsWith('0001-'))continue;const bin=bins[Math.min(59,Math.floor((t-startMs)/(span/60*1000)))];bin.packets++;bin.bytes+=row.length;}
 packetStats={...originalStats,duration:span,finished:end,bytes:capture.bytes,timedBytes:capture.bytes-64,protocols:{TCP:80,UDP:1},timedPackets:80,untimedPackets:1,timeline:bins};
 await page.evaluate(()=>NetworkToolkitPacketCapture.refresh(true));await page.waitForFunction(()=>NetworkToolkitPacketCapture.model.total===81&&!NetworkToolkitPacketCapture.model.busy&&NetworkToolkitPacketCapture.model.statistics?.duration===243&&NetworkToolkitPacketCapture.model.statistics?.timeline?.length===60);await settle();
 await page.locator('[data-packet-graph-mode]').selectOption('progressive');await settle();
 const progressivePacket=await packetGraph().evaluate(el=>[...el.querySelectorAll('*')].map(node=>({width:node.clientWidth,scrollWidth:node.scrollWidth,overflow:getComputedStyle(node).overflowX})).find(node=>['auto','scroll'].includes(node.overflow)&&node.scrollWidth>node.width+100));assert(progressivePacket,'Progressive capture timeline provides horizontal history scrolling');
 await page.locator('[data-packet-graph-mode]').selectOption('window');await settle();
 report.packets.push({retainedPackets:81,progressivePacket});
 for(const viewport of [{width:1440,height:1000},{width:820,height:1000},{width:390,height:1000},{width:820,height:650}])for(const theme of ['dark','light']){
  await page.setViewportSize(viewport);await page.evaluate(theme=>setTheme(theme),theme);await settle();await page.locator('#packetTableScroll').evaluate(el=>{el.scrollTop=0;});await settle();
  await page.evaluate(()=>NetworkToolkitPacketCapture.clearSelection());await settle();const geometry=await packetGeometry('Capture '+viewport.width+'x'+viewport.height+'/'+theme);
  if(out)await page.screenshot({path:path.join(out,'ui93-packets-'+viewport.width+'x'+viewport.height+'-'+theme+'.png')});report.layouts.push({kind:'packet',...viewport,theme,...geometry});
 }
 check('Packet graphs stay above independently scrollable virtual packet lists and offer progressive history without clipping clocks or shrinking the usable workspace');
 assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetFailures,[]);report.passed=true;await context.close();
})().catch(async error=>{report.failure=error.stack||String(error);console.error(report.failure);if(page&&out)await page.screenshot({path:path.join(out,'ui93-failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'ui93-results.json'),JSON.stringify(report,null,2));await browser?.close();});
