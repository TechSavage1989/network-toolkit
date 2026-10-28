'use strict';
// Real bundled baseline/candidate UIs, native timers, synthetic intercepted jobs only.
const fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const current=path.resolve(__dirname,'..'),baseline=process.env.NTK_GRAPH_BASELINE_DIR,out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT;
const baselineHash='b95b6bd5fd9262a74134d94cc2b466ae215caaba744ba4b8bae0a4e91da9382b';
const report={passed:false,checks:[],cases:[],errors:[],assetFailures:[],requests:[],baseline:{source:'d3494a879604fb2523e0867ad1fb175a2f9abbf5',timelineSHA256:baselineHash}},contexts=[];
const hash=bytes=>crypto.createHash('sha256').update(bytes).digest('hex');
const origin='https://temporal110.toolkit.test',jobID='graph110-synthetic';let browser,lastPage;
const stamp=n=>new Date(Date.parse('2026-10-28T11:00:00Z')+n*1000).toISOString();
const sample=n=>({sequence:n,time:stamp(n),address:'192.0.2.110',success:n!==4&&n!==8,status:n===4?'Timed out':n===8?'Pending':n===6?'Partial':'Reply',rttMs:n===4||n===8?null:n%2?100:1});
function makeJob(status,mode){const rows=Array.from({length:mode==='condensed'?301:21},(_,i)=>sample(i+1));return {id:jobID,tool:'ping',target:'192.0.2.110',status,options:{count:0,intervalMs:1000},started:stamp(0),finished:status==='running'?'':rows.at(-1).time,events:[],result:{summary:rows.length+' synthetic samples',data:{samples:rows,statistics:{sent:rows.length,received:rows.length-2,lost:2,minMs:1,maxMs:100,avgMs:50}}}};}
const settle=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
async function start(root,label){
 const context=await browser.newContext({viewport:{width:1440,height:1000},serviceWorkers:'block'});contexts.push(context);const page=await context.newPage();lastPage=page;page.setDefaultTimeout(15000);
 const engine={page,label,root,job:makeJob('completed','linear'),saved:{activePage:'overview',appUpdatePreferences:{enabled:false}},requests:[]};
 page.on('pageerror',error=>report.errors.push({label,error:error.message}));page.on('response',response=>{if(response.url().includes('/assets/')&&response.status()>=400)report.assetFailures.push({label,url:response.url(),status:response.status()});});
 await context.route('**/*',async route=>{try{const req=route.request(),url=new URL(req.url()),p=url.pathname;assert.equal(url.origin,origin,'External/device requests are forbidden');
  if(p.startsWith('/api/')){const request={label,path:p,method:req.method(),at:Date.now()};engine.requests.push(request);report.requests.push(request);let data={};
   if(req.method()!=='GET'){assert.equal(p,'/api/state','No scan, capture, connection or other operation may start');assert.equal(req.method(),'PUT');engine.saved=req.postDataJSON();}
   if(p==='/api/state')data=engine.saved;
   if(p==='/api/status')data={version:require(path.join(root,'package.json')).version,os:'windows',arch:'amd64',system:{hostname:'Synthetic temporal110',interfaces:[]}};
   if(p==='/api/system')data={hostname:'Synthetic temporal110',interfaces:[]};if(p==='/api/jobs')data=[engine.job];if(p==='/api/jobs/'+jobID)data=engine.job;
   if(p==='/api/terminals')data={sessions:[]};if(p==='/api/terminal-views')data={available:false,owners:{},views:[]};if(p==='/api/activity')data={tools:{ping:engine.job.status==='running'?{operations:1}:{}}};
   if(p.startsWith('/api/public-ip'))data={provider:'fixture',checkedAt:stamp(0),expiresAt:stamp(99999),ipv4:{status:'unavailable'},ipv6:{status:'unavailable'}};
   if(p==='/api/scan-networks')data={interfaces:[]};if(p==='/api/local-tools/ip')data={available:true,adapters:[]};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,60));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);let body=fs.readFileSync(file);
  if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','synthetic-temporal110').replaceAll('__WINDOW_MODE__','main');
  return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
 }catch(error){report.errors.push({label,route:route.request().url(),error:error.message});return route.fulfill({status:500,body:'Synthetic fixture refused request'});}});
 await page.goto(origin);await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);return engine;
}
async function target(page,selector){return page.locator(selector).first().evaluate(el=>{const marker=el.querySelector('[data-rt-marker]'),r=marker.getBoundingClientRect(),s=el.closest('svg').getBoundingClientRect();return {x:r.x+r.width/2,y:s.bottom-4,key:el.dataset.rtKey,sequence:Number(el.dataset.rtSequence||0)};});}
async function configure(engine,c){
 const {page}=engine;lastPage=page;engine.job=makeJob(c.status,c.mode);
 await page.mouse.move(300,30);await page.evaluate(({job,theme,mode})=>{NetworkToolkitResultTimeline.clearSelection(job.id);NetworkToolkitResultTimeline.setPreferences({mode},{persist:false});setTheme(theme);jobs=[job];selected.ping=job.id;navigate('ping');renderCurrentResult(true);},{job:engine.job,theme:c.theme,mode:c.mode});await settle(page);
 await page.locator('[data-rt-surface]').waitFor({state:'visible'});
 let selectedKey=null,hoveredKey=null;
 if(c.selected){const p=await target(page,c.mode==='condensed'?'[data-rt-aggregate]':'[data-rt-sequence="5"]');selectedKey=p.key;await page.mouse.click(p.x,p.y);await settle(page);}
 if(c.pointer==='hover'){const p=await target(page,c.mode==='condensed'&&!c.selected?'[data-rt-aggregate]':c.mode==='condensed'?'[data-rt-sequence="299"]':'[data-rt-sequence="11"]');hoveredKey=p.key;await page.mouse.move(p.x,p.y);}else await page.mouse.move(300,30);
 await settle(page);return {selectedKey,hoveredKey};
}
async function observe(engine,c,expected){
 const {page}=engine,label=engine.label,name=[c.status,c.theme,c.mode,c.selected?'selected':'unselected',c.pointer,label].join('-');
 const started=Date.now();await page.evaluate(()=>{
  const ids=new WeakMap();let nextID=1;const nodeID=n=>{if(!ids.has(n))ids.set(n,nextID++);return ids.get(n);};
  const surface=document.querySelector('[data-rt-surface]'),palette=document.createElement('span');palette.hidden=true;document.querySelector('#pageContent').append(palette);
  const resolve=value=>{palette.style.color=value;return getComputedStyle(palette).color;};
  const colors={accent:resolve('var(--accent)'),panel:resolve('var(--panel)'),good:resolve('var(--diag-good,var(--good))'),bad:resolve('var(--diag-bad,var(--bad))'),warn:resolve('var(--diag-warn,var(--warn))'),info:resolve('var(--diag-info,var(--accent))')};palette.remove();
  const state=window.graph110Frames={active:true,started:performance.now(),frames:[],states:[],colors,childMutations:0,classMutations:0,visibility:document.visibilityState},known=new Map();
  window.graph110Observer=new MutationObserver(records=>{for(const r of records)if(r.type==='childList')state.childMutations++;else state.classMutations++;});graph110Observer.observe(surface,{subtree:true,childList:true,attributes:true,attributeFilter:['class','style','stroke','stroke-width']});
  function frame(){if(!state.active)return;const svg=document.querySelector('[data-rt-surface]');
   const marks=[...svg.querySelectorAll('[data-rt-point]')].map(n=>{const m=n.querySelector('[data-rt-marker]'),s=getComputedStyle(m),r=m.getBoundingClientRect();return {key:n.dataset.rtKey,sequence:Number(n.dataset.rtSequence||0),aggregate:Number(n.dataset.rtAggregate||0),node:nodeID(n),class:n.getAttribute('class'),stroke:s.stroke,width:s.strokeWidth,fill:s.fill,opacity:s.opacity,x:r.x,y:r.y};});
   const value={marks,line:getComputedStyle(svg.querySelector('.rt-series polyline')).stroke,held:NetworkToolkitResultTimeline.isHeld('graph110-synthetic'),details:document.querySelector('#resultEntryBody .result-graph-summary')?.textContent||''},signature=JSON.stringify(value);
   if(!known.has(signature)){known.set(signature,state.states.length);state.states.push(value);}state.frames.push({t:performance.now()-state.started,state:known.get(signature),childMutations:state.childMutations,classMutations:state.classMutations});requestAnimationFrame(frame);
  }requestAnimationFrame(frame);
 });
 const captures=[],captured=new Set();const until=Date.now()+7800,deadline=Date.now()+16000;
 while(Date.now()<until||engine.requests.filter(r=>r.path==='/api/jobs'&&r.at>=started).length<3){
  assert(Date.now()<deadline,'Three native job polls must occur within the observation window');await page.waitForTimeout(100);
  const frame=await page.evaluate(()=>graph110Frames.frames.at(-1));if(frame&&!captured.has(frame.state)&&captured.size<2&&out){const file='graph-temporal110-'+name+'-state'+frame.state+'.png',before=await page.evaluate(()=>graph110Frames.frames.at(-1));await page.screenshot({path:path.join(out,file),clip:await page.locator('.rt-timeline').boundingBox()});const after=await page.evaluate(()=>graph110Frames.frames.at(-1));if(before.state===after.state){captured.add(after.state);captures.push({file,before,after});}}
 }
 const data=await page.evaluate(()=>{graph110Frames.active=false;graph110Observer.disconnect();return graph110Frames;});
 const polls=engine.requests.filter(r=>r.path==='/api/jobs'&&r.at>=started).map(r=>r.at-started),result={...c,label,name,expected,polls,captures,data};report.cases.push(result);
 if(out)fs.writeFileSync(path.join(out,'graph-temporal110-'+name+'.json'),JSON.stringify(result));
 assert(captures.length>=(label==='candidate'?1:2),'Actual stable-state PNGs must accompany each candidate/control observation');assert(data.frames.length>=20,'Actual requestAnimationFrame samples are required');assert(polls.length>=3,'Several native running/stopped polls are required');assert.equal(data.visibility,'visible');
 assert(data.states.every(s=>s.held===c.selected),'Background refresh cannot change selection ownership');
 const points=new Map();for(const s of data.states){assert.equal(s.line,data.colors.accent);for(const m of s.marks){const severity=['good','bad','warn','info'].find(x=>m.class.includes('rt-severity-'+x));assert(severity);assert.equal(m.fill,data.colors[severity]);if(!points.has(m.key))points.set(m.key,{styles:new Set(),nodes:new Set()});points.get(m.key).styles.add(JSON.stringify([m.class,m.stroke,m.width,m.fill]));points.get(m.key).nodes.add(m.node);}}
 result.changedKeys=[...points].filter(([,v])=>v.styles.size>1).map(([key])=>key);assert([...points.values()].every(v=>v.nodes.size===1),'Stable synthetic data must retain actual SVG point identities');
 if(label==='candidate'){
  assert.deepEqual(result.changedKeys,[],'Unchanged candidate marks cannot flash between hover/ordinary outlines');
  for(const s of data.states){assert.deepEqual(s.marks.filter(m=>m.class.includes('rt-point-hovered')).map(m=>m.key),expected.hoveredKey?[expected.hoveredKey]:[]);assert.deepEqual(s.marks.filter(m=>m.class.includes('rt-point-selected')).map(m=>m.key),expected.selectedKey?[expected.selectedKey]:[]);for(const m of s.marks){const hovered=m.key===expected.hoveredKey,selected=m.key===expected.selectedKey;assert.equal(m.stroke,hovered||selected?data.colors.accent:data.colors.panel);assert.equal(Number.parseFloat(m.width),selected?hovered?4:3:hovered?2.5:1);}}
 }else assert(result.changedKeys.length>0,'Exact109 negative control must reproduce real temporal outline changes');
 // Full actual UI evidence for representative normal light/dark and aggregate states.
 if(out&&((c.mode==='linear'&&!c.selected&&c.pointer==='outside')||(c.mode==='condensed'&&c.selected&&c.pointer==='hover'&&c.theme==='dark'))){const file='graph-temporal110-'+name+'-full.png';await page.screenshot({path:path.join(out,file)});result.fullScreenshot=file;}
 console.log('PASS '+name+'; '+polls.length+' polls; '+data.frames.length+' RAF samples; '+result.changedKeys.length+' changing point styles');return result;
}
(async()=>{
 assert(baseline,'NTK_GRAPH_BASELINE_DIR must identify an extracted exact109 source tree');assert(out,'Artifact output directory is required for temporal acceptance');fs.mkdirSync(out,{recursive:true});
 assert.equal(JSON.parse(fs.readFileSync(path.join(baseline,'package.json'),'utf8')).version,'1.17.109');assert.equal(hash(fs.readFileSync(path.join(baseline,'web/assets/result-timeline.js'))),baselineHash,'Baseline is immutable published109, not a reconstructed control');report.candidateTimelineSHA256=hash(fs.readFileSync(path.join(current,'web/assets/result-timeline.js')));assert.notEqual(report.candidateTimelineSHA256,baselineHash);
 browser=await chromium.launch({headless:true,args:['--no-sandbox']});const before=await start(baseline,'baseline109'),after=await start(current,'candidate');
 for(const mode of ['linear','condensed'])for(const status of ['running','completed'])for(const theme of ['dark','light'])for(const selected of [false,true])for(const pointer of ['outside','hover']){
  const c={mode,status,theme,selected,pointer},expectations=await Promise.all([configure(before,c),configure(after,c)]);await Promise.all([observe(before,c,expectations[0]),observe(after,c,expectations[1])]);
 }
 assert.equal(report.cases.length,64);assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetFailures,[]);report.checks.push('Exact109 negative control reproduces outline changes while candidate is stable across32 real-timer states','Running/stopped, stationary/off-plot, selected/unselected, ordinary/aggregate and light/dark frames preserve status colors, theme lines, selection and node identities');report.passed=true;
})().catch(async error=>{report.failure=error.stack;console.error(error);if(lastPage&&!lastPage.isClosed()&&out)await lastPage.screenshot({path:path.join(out,'graph-temporal110-failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'graph-temporal110-results.json'),JSON.stringify(report,null,2));for(const c of contexts)await c.close().catch(()=>{});await browser?.close();});
