'use strict';
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const current=path.resolve(process.env.TOOLKIT_TEST_ROOT||path.join(__dirname,'..')),output=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT;
const report={passed:false,checks:[],layouts:[],errors:[],assetErrors:[],violations:[]},contexts=[],only=process.env.TOOLKIT_MOTION_ONLY||'';let browser,lastPage,currentScenario=null;
const marker='SYNTHETIC_MOTION114_DOCUMENT',text=marker+'\n'+Array.from({length:800},(_,i)=>'const sample'+i+' = "'+('synthetic wrapped content '.repeat(i%9===0?18:2))+'";').join('\n'),rightText=text.split('\n').flatMap((line,i)=>i&&i%80===0?['// right-only synthetic line '+i,line]:[line]).join('\n'),childID='c'.repeat(32);
const check=message=>{report.checks.push(message);console.log('PASS '+message);};
async function settle(page){await page.waitForFunction(()=>{const side=document.querySelector('#sidebar');return !side||document.body.classList.contains('editor-window')||innerWidth>800||side.getBoundingClientRect().right<=1;});await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))));}
async function start(root,mode,label,initialSaved){
 const context=await browser.newContext({viewport:{width:1440,height:1000}});contexts.push(context);const page=await context.newPage();lastPage=page;page.setDefaultTimeout(20000);
 const owned={},requests=[];let saved=initialSaved?structuredClone(initialSaved):{activePage:'editor',appUpdatePreferences:{enabled:false},editorPreferences:{wordWrap:true,syntax:true,fontSize:13,palette:'auto'}};
 page.on('pageerror',e=>report.errors.push(label+': '+e.message));
 await context.route('https://divider113.toolkit.test/**',async route=>{try{if(await handleEditorActivityMetadata(route,'https://divider113.toolkit.test',report))return;const req=route.request(),url=new URL(req.url()),p=url.pathname,body=req.postData()?req.postDataJSON():null;requests.push({method:req.method(),path:p});if(p.startsWith('/api/')){
  let data={};if(p==='/api/status')data={version:require(path.join(root,'package.json')).version,os:'windows',arch:'amd64',editorWindowAvailable:true};
  else if(p==='/api/state'){if(req.method()==='GET')data=saved;else{assert(!JSON.stringify(body).includes(marker),'Document bytes must not enter persisted settings');saved=structuredClone(body);data={ok:true};}}
  else if(p==='/api/jobs')data=[];
  else if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};
  else if(p==='/api/activity')data={tools:{}};
  else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
  else if(p==='/api/editor-views'||p==='/api/editor-views/current')data={available:true,ownerInstanceId:'synthetic-layout-engine',self:mode==='detached'?childID:'',views:[],incoming:[],outgoing:[],owned,recovery:[]};
  else if(p==='/api/editor-views/current/state'){for(const d of body.batch.documents)owned[d.uid]=1;data={owned};}
  else if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,60));data={events:[],sessions:[],after:0};}
  else if(req.method()!=='GET')throw Error('Unexpected operation in presentation-only fixture: '+req.method()+' '+p);
  return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
 }
 const html=p==='/'||p==='/editor-view/'+childID;let file=path.join(root,'web',html?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);try{let bytes=fs.readFileSync(file);if(html)bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','synthetic-divider113').replaceAll('__WINDOW_MODE__',mode==='detached'?'editor':'main').replaceAll('__EDITOR_VIEW__',mode==='detached'?JSON.stringify({id:childID,ownerInstanceId:'synthetic-layout-engine'}):'null').replaceAll('__TERMINAL_VIEW__','null').replaceAll('src="assets/','src="/assets/').replaceAll('href="assets/','href="/assets/').replaceAll('src="../assets/','src="/assets/').replaceAll('href="../assets/','href="/assets/');return route.fulfill({body:bytes,contentType:html?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{report.assetErrors.push(label+': '+p);return route.fulfill({status:404,body:''});}
 }catch(error){report.errors.push(label+': '+error.message);await route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture rejected an unexpected operation'})}).catch(()=>{});}
 });
 await page.goto('https://divider113.toolkit.test/'+(mode==='detached'?'editor-view/'+childID:''));await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(mode=>navigate(mode==='modal'?'terminal':'editor'),mode);
 await page.evaluate(async text=>{await NetworkToolkitFileEditor.open({id:'layout-left',path:'layout-left.js',source:'Synthetic local',binding:{kind:'local'},create:true,initialContent:text,read:async()=>({content:text}),write:async()=>{throw Error('No save expected');}});},text);
 await page.locator('[data-ed-text]').press('Control+Home');await page.locator('[data-ed-text]').press('X');
 await page.evaluate(async text=>{await NetworkToolkitFileEditor.open({id:'layout-right',path:'layout-right.js',source:'Synthetic local',binding:{kind:'local'},create:true,initialContent:text.replace('sample90 =','changed90 ='),read:async()=>({content:text}),write:async()=>{throw Error('No save expected');}});},rightText);
 await page.locator('[data-ed-text]').press('Control+Home');await page.locator('[data-ed-text]').press('X');
 await page.waitForFunction(()=>NetworkToolkitEditorWindows.state.enabled&&!NetworkToolkitEditorWindows.state.running);return {page,context,requests,get saved(){return saved;}};
}
// Capture every animation frame through the gesture, rather than sampling only
// its final converged position. Values are source/viewport geometry, not secrets.
function beginCapture({label,side,kind}){
 window.motion114={label,frames:[],inputs:[],running:true};const record=window.motion114,c=NetworkToolkitFileEditor.comparison;
 const target=kind==='map'?document.querySelector('[data-compare-track='+side+']'):c.sourceView(side).scrollElement,type=kind==='map'?'pointermove':'wheel';
 const input=event=>{if(kind==='map'&&!(event.buttons&1))return;record.inputs.push({at:performance.now(),frame:record.frames.length});};
 target.addEventListener(type,input,{passive:true,capture:true});window.motion114Cleanup=()=>target.removeEventListener(type,input,{capture:true});
 const rowByLine=Object.fromEntries(['left','right'].map(side=>[side,new Map(c.result.rows.flatMap((row,i)=>row[side]===null?[]:[[row[side]+1,i]]))]));
 function capture(){if(!record.running)return;const panes=['left','right'].map(side=>{
  // Never call a CodeMirror layout reader here: elementAtHeight can flush a
  // pending measure and synchronize both panes midway through this snapshot.
  // Read only painted DOM rectangles and scroll offsets in one synchronous turn.
  const facade=c.sourceView(side),input=facade.inputElement,s=facade.scrollElement,r=s.getBoundingClientRect();
  const visible=[...input.querySelectorAll('.cm-line[data-source-line]')].map(el=>{const b=el.getBoundingClientRect();return {row:rowByLine[side].get(Number(el.dataset.sourceLine)),line:Number(el.dataset.sourceLine),y:b.top-r.top,h:b.height,visible:Math.min(b.bottom,r.bottom)-Math.max(b.top,r.top)}}).filter(x=>x.visible>1);
  const first=visible[0],position=first?first.row+Math.max(0,Math.min(1,-first.y/Math.max(1,first.h))):null;
  return {side,top:s.scrollTop,position,visible,map:Number(document.querySelector('[data-compare-track='+side+']').getAttribute('aria-valuenow'))};
 });const common=panes[0].visible.map(a=>{const b=panes[1].visible.find(b=>b.row===a.row);return b?{row:a.row,delta:a.y-b.y}:null}).find(Boolean);record.frames.push({at:performance.now(),panes,delta:common?.delta??null});requestAnimationFrame(capture)}requestAnimationFrame(capture);
}
async function finishCapture(page){await page.evaluate(()=>new Promise(resolve=>{let left=5;const step=()=>--left?requestAnimationFrame(step):resolve();requestAnimationFrame(step)}));return page.evaluate(()=>{motion114.running=false;window.motion114Cleanup?.();delete window.motion114Cleanup;return motion114;});}
function summarize(record,side){const s=side==='right'?1:0,p=1-s,frames=record.frames,movementFrames=[],movementTimes=frames.length?[frames[0].at]:[];let moving=0,together=0,stall=0,maxStall=0;for(let i=1;i<frames.length;i++){
 // Actual scrollDOM pixels are authoritative for painted motion. Native virtual
 // block indices can refine a frame later without any pixel displacement.
 const a=frames[i],b=frames[i-1],sourceMoved=Math.abs(a.panes[s].top-b.panes[s].top)>1,peerMoved=Math.abs(a.panes[p].top-b.panes[p].top)>1;
 if(sourceMoved){moving++;movementTimes.push(a.at);movementFrames.push({at:a.at,index:i});if(peerMoved){together++;stall=0}else{stall++;maxStall=Math.max(maxStall,stall)}}else if(peerMoved)stall=0;
 }if(frames.length)movementTimes.push(frames.at(-1).at);const pauses=movementTimes.slice(1).map((time,i)=>time-movementTimes[i]).sort((a,b)=>a-b);const responses=(record.inputs||[]).map(input=>{const moved=movementFrames.find(f=>f.at>=input.at);return moved?{ms:moved.at-input.at,frames:moved.index-input.frame+1}:null});const deltas=frames.filter(f=>f.delta!==null).map(f=>Math.abs(f.delta)).sort((a,b)=>a-b);return {label:record.label,frames:frames.length,moving,together,maxStall,deliveredInputs:responses.length,unansweredInputs:responses.filter(r=>!r).length,maxInputResponseMs:Math.max(0,...responses.filter(Boolean).map(r=>r.ms)),maxInputResponseFrames:Math.max(0,...responses.filter(Boolean).map(r=>r.frames)),gestureDurationMs:frames.length?frames.at(-1).at-frames[0].at:0,maxSourcePauseMs:pauses.at(-1)??null,p95SourcePauseMs:pauses[Math.floor(pauses.length*.95)]??null,p95PeerOffset:deltas[Math.floor(deltas.length*.95)]??null,maxPeerOffset:deltas.at(-1)??null,first:frames[0]?.panes.map(p=>p.position),last:frames.at(-1)?.panes.map(p=>p.position)};}
async function captureGesture(page,label,side,action){await page.evaluate(beginCapture,{label,side,kind:label.includes('-map-')?'map':'wheel'});await action();const record=await finishCapture(page),summary=summarize(record,side);report.motion.push({...summary,record});return summary;}
function validateMotion(summary){
 try{
 assert(summary.frames>=15,summary.label+' must observe at least15 animation frames during input');
 // Browsers coalesce pointer positions. Measure delay after actual DOM input
 // delivery, not CDP transport gaps or a magic number of sampled positions.
 // Both frame and wall-time bounds reject a stopped-both-panes catch-up burst.
 assert(summary.moving>0&&summary.deliveredInputs>0&&summary.unansweredInputs===0&&summary.maxInputResponseFrames<=4&&summary.maxInputResponseMs<=200,summary.label+' every delivered input must produce visible source motion within4 observed frames and200ms: '+JSON.stringify(summary));
 assert(summary.maxStall<=4,summary.label+' must not leave its peer stopped while source continues: '+JSON.stringify(summary));
 assert(summary.together/summary.moving>=.65,summary.label+' must move its peer throughout input, not after stopping: '+JSON.stringify(summary));
 const wheel=summary.label.includes('-wheel-');
 assert(summary.p95PeerOffset!==null&&summary.p95PeerOffset<=(wheel?72:2),summary.label+' actual common source rows must remain aligned throughout input within at most two wheel increments: '+JSON.stringify(summary));
 }catch(error){report.violations.push({label:summary.label,message:error.message,summary});console.error('MOTION VIOLATION '+error.message);}
}
async function converged(page){await page.waitForFunction(()=>{const c=NetworkToolkitFileEditor.comparison;if(!c?.result||c.result.limited||document.querySelector('[data-compare-map]')?.getAttribute('aria-disabled')!=='false')return false;const a=document.querySelector('[data-compare-map-viewport=left]'),b=document.querySelector('[data-compare-map-viewport=right]');return Math.abs(parseFloat(a.style.top)-parseFloat(b.style.top))<1;});await page.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r))));}
(async()=>{
 assert(output,'Set isolated TOOLKIT_TEST_OUTPUT');fs.mkdirSync(output,{recursive:true});report.motion=[];report.observationOnly=!!process.env.TOOLKIT_MOTION_OBSERVE_ONLY;report.sourceHash=require('node:crypto').createHash('sha256').update(fs.readFileSync(path.join(current,'web/assets/editor-aligned-compare.js'))).digest('hex');browser=await chromium.launch({headless:true});
 for(const mode of ['main','detached'])for(const width of [1440,390]){
  if(only&&!only.startsWith(mode+'-'+width+'-'))continue;
  const theme=width===390?'dark':'light',f=await start(current,mode,'motion-'+mode+'-'+width),page=f.page;await page.setViewportSize({width,height:1000});await page.evaluate(theme=>setTheme(theme),theme);await settle(page);await page.locator('[data-ed-wrap]').check();await page.locator('[data-ed=compare]').click();await page.selectOption('[data-compare-select=left]','layout-left');await page.selectOption('[data-compare-select=right]','layout-right');await converged(page);assert(await page.locator('[data-ed-wrap]').isChecked(),'Each renderer must use actual wrapped text, including child defaults');
  const original=await page.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].map(d=>({uid:d.uid,content:d.content,undo:JSON.stringify(d.undo),redo:JSON.stringify(d.redo),revision:d.editRevision})));
  for(const side of ['left','right'])for(const direction of [1,-1]){
   const label=mode+'-'+width+'-'+side+'-wheel-'+direction;if(only&&only!==label)continue;const scroller=page.locator('[data-compare-pane='+side+'] .cm-scroller');await scroller.hover();
   const summary=await captureGesture(page,label,side,async()=>{for(let step=0;step<40;step++){await page.mouse.wheel(0,direction*36);await page.waitForTimeout(12);}});if(!process.env.TOOLKIT_MOTION_OBSERVE_ONLY)validateMotion(summary);await converged(page);
  }
  for(const scale of ['0','2']){
   await page.locator('.nt-compare-settings summary').click();await page.selectOption('[data-compare-scale]',scale);await page.locator('.nt-compare-settings summary').click();await converged(page);
   for(const side of ['left','right'])for(const direction of [1,-1]){
    const label=mode+'-'+width+'-'+side+'-map-'+scale+'-'+direction;if(only&&only!==label)continue;
    const track=page.locator('[data-compare-track='+side+']'),map=page.locator('[data-compare-map]'),rect=await map.boundingBox(),box=await track.boundingBox(),x=box.x+box.width/2,y=rect.y+rect.height*(direction>0?.25:.65);await page.mouse.move(x,y);await page.mouse.down();
    const summary=await captureGesture(page,label,side,async()=>{for(let step=0;step<40;step++){await page.mouse.move(x,y+direction*rect.height*.4*(step+1)/40);await page.waitForTimeout(12);}});await page.mouse.up();if(!process.env.TOOLKIT_MOTION_OBSERVE_ONLY)validateMotion(summary);await converged(page);
    // Real wheel over both fit and zoomed map remains functional.
    const before=await page.evaluate(side=>NetworkToolkitFileEditor.comparison.sourceView(side).scrollTop,side);await track.hover();await page.mouse.wheel(0,-120);await page.waitForFunction(({side,before})=>Math.abs(NetworkToolkitFileEditor.comparison.sourceView(side).scrollTop-before)>1,{side,before});await converged(page);
   }
  }
  assert.deepEqual(await page.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].map(d=>({uid:d.uid,content:d.content,undo:JSON.stringify(d.undo),redo:JSON.stringify(d.redo),revision:d.editRevision}))),original,'Scroll gestures must not change source/undo/dirty ownership');
  await page.screenshot({path:path.join(output,'editor-motion114-'+mode+'-'+width+'-'+theme+'.png')});report.layouts.push({mode,width,theme});check(mode+' '+width+' '+(report.observationOnly?'baseline observation only (candidate motion assertions omitted); ':'candidate continuous motion recorded; ')+'left/right wheel and both maps, fit/zoom, unequal wrapped source bytes retained');await f.context.close();
 }
 assert(report.motion.length>0,'Motion selector must match a real gesture');assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetErrors,[]);assert.deepEqual(report.violations,[],'All continuous gesture cases must pass');report.passed=true;
})().catch(async error=>{report.failure=error.stack||String(error);console.error(report.failure);if(lastPage&&!lastPage.isClosed())await lastPage.screenshot({path:path.join(output,'editor-motion114-failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(output)fs.writeFileSync(path.join(output,'editor-motion114-results.json'),JSON.stringify(report,null,2));for(const context of contexts)await context.close().catch(()=>{});await browser?.close();});
