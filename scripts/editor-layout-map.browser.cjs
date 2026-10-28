'use strict';
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const current=path.resolve(__dirname,'..'),baseline=process.env.NTK_LAYOUT_BASELINE110_DIR,output=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT;
const report={passed:false,checks:[],layouts:[],errors:[],assetErrors:[]},contexts=[];let browser,lastPage;
const marker='SYNTHETIC_LAYOUT109_DOCUMENT',text=marker+'\n'+Array.from({length:400},(_,i)=>'const sample'+i+' = "'+('synthetic wrapped content '.repeat(i%9===0?18:2))+'";').join('\n'),childID='c'.repeat(32);
const check=message=>{report.checks.push(message);console.log('PASS '+message);};
async function settle(page){await page.waitForFunction(()=>{const side=document.querySelector('#sidebar');return !side||document.body.classList.contains('editor-window')||innerWidth>800||side.getBoundingClientRect().right<=1;});await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))));}
async function start(root,mode,label,initialSaved){
 const context=await browser.newContext({viewport:{width:1440,height:1000},deviceScaleFactor:Number(process.env.NTK_TEST_DPR)||1});contexts.push(context);const page=await context.newPage();lastPage=page;page.setDefaultTimeout(20000);
 const owned={},requests=[];let saved=initialSaved?structuredClone(initialSaved):{activePage:'editor',appUpdatePreferences:{enabled:false},editorPreferences:{wordWrap:true,syntax:true,fontSize:13,palette:'auto'}};
 page.on('pageerror',e=>report.errors.push(label+': '+e.message));
 await context.route('https://layout109.toolkit.test/**',async route=>{try{const req=route.request(),url=new URL(req.url()),p=url.pathname,body=req.postData()?req.postDataJSON():null;requests.push({method:req.method(),path:p});if(p.startsWith('/api/')){
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
 const html=p==='/'||p==='/editor-view/'+childID;let file=path.join(root,'web',html?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);try{let bytes=fs.readFileSync(file);if(html)bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','synthetic-layout109').replaceAll('__WINDOW_MODE__',mode==='detached'?'editor':'main').replaceAll('__EDITOR_VIEW__',mode==='detached'?JSON.stringify({id:childID,ownerInstanceId:'synthetic-layout-engine'}):'null').replaceAll('__TERMINAL_VIEW__','null').replaceAll('src="assets/','src="/assets/').replaceAll('href="assets/','href="/assets/').replaceAll('src="../assets/','src="/assets/').replaceAll('href="../assets/','href="/assets/');return route.fulfill({body:bytes,contentType:html?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{report.assetErrors.push(label+': '+p);return route.fulfill({status:404,body:''});}
 }catch(error){report.errors.push(label+': '+error.message);await route.fulfill({status:500,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture rejected an unexpected operation'})}).catch(()=>{});}
 });
 await page.goto('https://layout109.toolkit.test/'+(mode==='detached'?'editor-view/'+childID:''));await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(mode=>navigate(mode==='modal'?'terminal':'editor'),mode);
 await page.evaluate(async text=>{await NetworkToolkitFileEditor.open({id:'layout-left',path:'layout-left.js',source:'Synthetic local',binding:{kind:'local'},create:true,initialContent:text,read:async()=>({content:text}),write:async()=>{throw Error('No save expected');}});},text);
 await page.locator('[data-ed-text]').press('Control+Home');await page.locator('[data-ed-text]').press('X');
 await page.evaluate(async text=>{await NetworkToolkitFileEditor.open({id:'layout-right',path:'layout-right.js',source:'Synthetic local',binding:{kind:'local'},create:true,initialContent:text.replace('sample90 =','changed90 ='),read:async()=>({content:text}),write:async()=>{throw Error('No save expected');}});},text);
 await page.locator('[data-ed-text]').press('Control+Home');await page.locator('[data-ed-text]').press('X');
 await page.waitForFunction(()=>NetworkToolkitEditorWindows.state.enabled&&!NetworkToolkitEditorWindows.state.running);return {page,context,requests,get saved(){return saved;}};
}
async function geometry(page,compare){return page.evaluate(compare=>{const el=document.querySelector(compare?'.nt-compare-surface':'.nt-editor-surface'),r=el.getBoundingClientRect();let top=Math.max(0,r.top),bottom=Math.min(innerHeight,r.bottom);for(let parent=el.parentElement;parent;parent=parent.parentElement){if(/hidden|auto|scroll|clip/.test(getComputedStyle(parent).overflowY)){const p=parent.getBoundingClientRect();top=Math.max(top,p.top);bottom=Math.min(bottom,p.bottom);}}return {x:r.x,y:r.y,width:r.width,height:r.height,visibleHeight:Math.max(0,bottom-top),viewportWidth:innerWidth,viewportHeight:innerHeight,overflow:Math.max(0,document.documentElement.scrollWidth-innerWidth)};},compare);}
async function selectionCheckpoint(page,label,expected){const actual=await snapshot(page);(report.selectionTransitions||=[]).push({label,start:actual.start,end:actual.end,top:actual.top,dirty:actual.dirty,revision:actual.revision});assert.deepEqual(actual,expected,label);}
async function snapshot(page){return page.evaluate(()=>{const d=NetworkToolkitFileEditor.active,ta=document.querySelector('[data-ed-text]');return {uid:d.uid,text:d.content,dirty:d.dirty,undo:[...d.undo],revision:d.editRevision,start:ta.selectionStart,end:ta.selectionEnd,top:ta.scrollTop,left:ta.scrollLeft};});}

async function ready(p){await p.waitForFunction(()=>document.querySelector('[data-compare-map]')?.getAttribute('aria-disabled')==='false');await settle(p);}
async function compareOpen(p){await p.locator('[data-ed="compare"]').click();await ready(p);}
(async()=>{
 assert(baseline&&fs.existsSync(path.join(baseline,'web/index.html')),'Provide the verified110 baseline');
 if(output)fs.mkdirSync(output,{recursive:true});browser=await chromium.launch({headless:true});
 for(const mode of ['main','detached','modal']){
  const before=await start(baseline,mode,'before'),after=await start(current,mode,'after');lastPage=after.page;
  for(const size of [{width:1440,height:1000},{width:390,height:1000},{width:820,height:540}])for(const theme of ['dark','light']){
   for(const item of [before,after]){await item.page.setViewportSize(size);await item.page.evaluate(theme=>setTheme(theme),theme);await settle(item.page);}
   for(const comparing of [false,true]){
    if(comparing)for(const item of [before,after])await compareOpen(item.page);
    const a=await geometry(before.page,comparing),b=await geometry(after.page,comparing);
    report.layouts.push({mode,theme,comparing,...size,before:a,after:b,gain:b.visibleHeight-a.visibleHeight});
    assert(b.overflow<=1);assert(b.visibleHeight>=a.visibleHeight,JSON.stringify(report.layouts.at(-1)));
    if(output&&size.width!==820)for(const [label,item] of [['before',before],['after',after]])await item.page.screenshot({path:path.join(output,`${label}-${mode}-${size.width}-${theme}-${comparing?'compare':'text'}.png`)});
    if(comparing)for(const item of [before,after])await item.page.locator('[data-compare="close"]').click();
   }
  }
  check(mode+' rendered before/after height, narrow, short, light/dark');
  if(mode==='main'){
   const p=after.page;lastPage=p;await p.setViewportSize({width:1440,height:1000});await p.uncheck('[data-ed-wrap]');await compareOpen(p);
   const left=p.locator('[data-compare-text="left"]'),right=p.locator('[data-compare-text="right"]'),map=p.locator('[data-compare-map]'),track=side=>p.locator('[data-compare-track="'+side+'"]');
   assert.equal(await p.locator('[data-compare-track]').count(),2);assert.equal(await p.locator('.nt-editor-title').count(),0);assert.equal(await p.locator('[data-compare="new"],[data-compare="open"]').count(),0);
   await p.locator('.nt-compare-settings summary').click();await p.uncheck('[data-compare-sync]');
   for(const side of ['left','right']){
    const own=side==='left'?left:right,other=side==='left'?right:left;
    await track(side).press('Home');const otherBefore=await other.evaluate(e=>e.scrollTop);await track(side).hover();await p.mouse.wheel(0,160);await p.waitForTimeout(80);
    assert(await own.evaluate(e=>e.scrollTop)>0);assert.equal(await other.evaluate(e=>e.scrollTop),otherBefore);
    const box=await track(side).boundingBox();await p.mouse.move(box.x+box.width/2,box.y+box.height*.2);await p.mouse.down();await p.mouse.move(box.x+box.width/2,box.y+box.height*.7,{steps:10});await p.mouse.up();
    assert(await own.evaluate(e=>e.scrollTop)>1000);assert.equal(await other.evaluate(e=>e.scrollTop),otherBefore);
   }
   check('Fit wheel and click-drag on either track navigate only the selected document with sync off');
   const font=await left.evaluate(e=>getComputedStyle(e).fontSize);
   for(const scale of ['0.5','1','2','4']){await p.selectOption('[data-compare-scale]',scale);const sizes=await map.evaluate(e=>({view:e.clientHeight,content:e.firstElementChild.offsetHeight}));assert(Math.abs(sizes.content/sizes.view-(1+Number(scale)))<.02);}
   const fixed=await left.evaluate(e=>e.scrollTop);await track('right').hover();await p.mouse.wheel(0,-110);await p.waitForTimeout(80);assert.equal(await left.evaluate(e=>e.scrollTop),fixed);assert.equal(await left.evaluate(e=>getComputedStyle(e).fontSize),font);
   check('all zoom choices visibly expand map, wheel pans/navigates without changing font or other document');
   await p.check('[data-compare-sync]');await p.selectOption('[data-compare-scale]','0');await track('left').press('Home');await track('right').hover();await p.mouse.wheel(0,123.5);await p.waitForTimeout(100);assert(Math.abs(await left.evaluate(e=>e.scrollTop)-await right.evaluate(e=>e.scrollTop))<2);
   await p.check('[data-ed-wrap]');await ready(p);await p.setViewportSize({width:700,height:650});await ready(p);await track('right').press('End');assert(await right.evaluate(e=>e.scrollTop)>0);
   const old=await p.evaluate(()=>NetworkToolkitFileEditor.comparison.selection);await p.locator('[data-compare="swap"]').click();await ready(p);assert.deepEqual(await p.evaluate(()=>NetworkToolkitFileEditor.comparison.selection),{left:old.right,right:old.left});
   await right.focus();await right.press('Control+Home');const original=await right.inputValue();await right.press('X');await ready(p);await right.press('Control+z');await ready(p);assert.equal(await right.inputValue(),original);
   check('sync, fractional wheel input, wrap, resize, swap and comparison undo retain document identity');
   // Unequal documents include blank alignment rows on only the absent side.
   await p.evaluate(()=>{const c=NetworkToolkitFileEditor.comparison;const a=NetworkToolkitFileEditor.documents.get(c.selection.left),b=NetworkToolkitFileEditor.documents.get(c.selection.right);a.content='first\nlast';b.content='first\nextra\nextra2\nlast';a.editRevision++;b.editRevision++;c.refresh(true);});await ready(p);
   assert.equal(await p.locator('[data-compare-map-marks="left"] .gap').count(),1);assert.equal(await p.locator('[data-compare-map-marks="right"] .added').count(),1);assert.equal(await p.locator('[data-compare-map-marks="left"] .equal').count(),2);
   for(const pair of [['',''],['a','b'],[Array.from({length:900},(_,i)=>'left'+i).join('\n'),Array.from({length:950},(_,i)=>'right'+i).join('\n')]]){await p.evaluate(([a,b])=>{const c=NetworkToolkitFileEditor.comparison,l=NetworkToolkitFileEditor.documents.get(c.selection.left),r=NetworkToolkitFileEditor.documents.get(c.selection.right);l.content=a;r.content=b;l.editRevision++;r.editRevision++;c.refresh(true);},pair);await ready(p);assert.equal(await p.locator('[data-compare-map-viewport]:visible').count(),2);}
   check('empty, one-line, unequal-length and heavily changed documents render both tracks');
   await p.setViewportSize({width:1440,height:1000});await p.uncheck('[data-ed-wrap]');await p.evaluate(()=>setTheme('dark'));
   const example=Array.from({length:600},(_,i)=>'const synthetic'+i+' = '+i+';'),revised=example.slice();revised.splice(450,15,...Array.from({length:15},(_,i)=>'const modified'+i+' = true;'));revised.splice(260,0,...Array.from({length:35},(_,i)=>'const added'+i+' = true;'));revised.splice(90,25);await left.fill(example.join('\n'));await ready(p);await right.fill(revised.join('\n'));await ready(p);
   assert(await p.locator('[data-compare-map-marks="left"] .removed').count()>0);assert(await p.locator('[data-compare-map-marks="right"] .added').count()>0);assert.equal(await p.locator('[data-compare-map-marks] .changed').count(),2);
   await p.selectOption('[data-compare-scale]','0');await track('left').press('Home');
   if(output)await p.screenshot({path:path.join(output,'map-fit-all-change-types.png')});
   await p.selectOption('[data-compare-scale]','4');await track('left').press('PageDown');
   const geometryCheck=await p.evaluate(()=>{const c=NetworkToolkitFileEditor.comparison,rows=c.result.rows.length,map=document.querySelector('[data-compare-map-content]');return ['left','right'].map(side=>{const ta=document.querySelector('[data-compare-text="'+side+'"]'),style=getComputedStyle(ta),line=Math.max(0,(ta.scrollTop-parseFloat(style.paddingTop))/parseFloat(style.lineHeight)),expected=NTKEditorCompare.rowAtLine(c.result.positions[side],line)/rows*map.offsetHeight,marker=document.querySelector('[data-compare-map-viewport="'+side+'"]');return {side,expected,actual:parseFloat(marker.style.top)};});});
   for(const g of geometryCheck)assert(Math.abs(g.expected-g.actual)<1,JSON.stringify(g));
   if(output)await p.screenshot({path:path.join(output,'map-zoom-5x.png')});
   check('separate additions/removals/modifications/gaps and pixel-accurate per-document viewport markers');
  }
  await before.context.close();await after.context.close();
 }
 assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetErrors,[]);report.passed=true;
})().catch(async e=>{report.failure=e.stack||String(e);console.error(report.failure);if(lastPage&&!lastPage.isClosed()&&output)await lastPage.screenshot({path:path.join(output,'failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(output)fs.writeFileSync(path.join(output,'editor-layout-map-results.json'),JSON.stringify(report,null,2));for(const c of contexts)await c.close().catch(()=>{});await browser?.close();});
