'use strict';
// Real production UI/assets with isolated synthetic session/state routes only.
// No shell/device connection, native Windows window or user file is accessed.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT;
const report={passed:false,checks:[],errors:[],screenshots:[],layouts:[],scope:'Production UI in main/separate/detached modes, synthetic APIs and terminal bytes. Browser deviceScaleFactor is not native Windows DPI/WebView2 acceptance.'};
const transcript=[
 '(C) Copyright 2017-2026 Example Enterprise Development LP',
 'to be bound by its terms. Do not use the software.',
 'User "admin" has logged in 4 times in the past 30 days',
 '\x1b[33mDevice ANSI yellow banner prose stays yellow.\x1b[0m',
 'Last login: 2026-10-27 22:33:21 from 192.0.2.26',
 'https://support.example.invalid/port1',
 'Gi1/0/1  up  192.0.2.1  00:11:22:33:44:55',
 '┌──────────┬──────────┐',
 '│ IPv4     │ IPv6     │',
 '└──────────┴──────────┘',
 'café e\u0301 Δ λ 日本語 → ✓',
 'switch# '
].join('\r\n');
const pause=ms=>new Promise(resolve=>setTimeout(resolve,ms));
let browser,page,activeCase='',activeCalls=[];
async function shot(name){if(!out)return;const file='terminal-style107-'+name+'.png';await page.screenshot({path:path.join(out,file),fullPage:true});report.screenshots.push(file);}
async function settle(){await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));}
async function display(){if(await page.locator('#terminalSessionPanel.hidden').count())await page.locator('#terminalSessionToggle').click();await page.locator('#terminalAppearanceEditor').waitFor({state:'visible'});}
async function fitted(){
 try{
  // Keep the full acceptance predicate: matching DOM cell geometry AND an
  // acknowledged backend size, with no in-flight or pending resize.
  await page.waitForFunction(()=>{const e=currentTerminal(),t=e?.xterm,p=e?.fit?.proposeDimensions();return !!t&&!!p&&t.cols===terminalDimensions(p).cols&&t.rows===terminalDimensions(p).rows&&!e.desiredResize&&!e.resizeSending&&e.lastResize===t.cols+'x'+t.rows;});await settle();
 }catch(error){
  const state=await page.evaluate(()=>{
   const e=currentTerminal(),t=e?.xterm,p=e?.fit?.proposeDimensions(),parent=t?.element?.parentElement,rect=parent?.getBoundingClientRect(),style=parent?getComputedStyle(parent):null;
   return {actual:t?{cols:t.cols,rows:t.rows}:null,proposed:p||null,normalized:p?terminalDimensions(p):null,lastResize:e?.lastResize,desiredResize:e?.desiredResize||null,resizeSending:!!e?.resizeSending,inputEpoch:e?.inputEpoch||0,sessionState:e?.info.state,online,activePage:state.activePage,visible:!!e&&terminalEntryVisible(e),owner:e?terminalSessionOwner(e.info.id):null,canMutate:!!e&&terminalOwnerCanMutate(e),detachedReady:terminalDetachedReady,transferBusy:!!e?.transferBusy,restoring:!!e?.restoring,awaitingAcceptance:!!e?.awaitingAcceptance,checkpointBusy:terminalCheckpointBusy,feedPaused:terminalFeedPaused,font:t?{family:t.options.fontFamily,size:t.options.fontSize,lineHeight:t.options.lineHeight}:null,parent:rect?{width:rect.width,height:rect.height,cssWidth:style.width,cssHeight:style.height}:null};
  }).catch(failure=>({diagnosticError:String(failure)}));
  // Route records contain synthetic fixture data only; retain bounded resize
  // and ownership request metadata, never full settings or terminal output.
  const calls=activeCalls.filter(call=>call.p.endsWith('/resize')||call.p.startsWith('/api/terminal-views')).slice(-20).map(call=>({path:call.p,method:call.method,...(call.p.endsWith('/resize')?{size:{cols:call.body?.cols,rows:call.body?.rows}}:{})}));
  (report.fitFailures??=[]).push({case:activeCase,state,calls});
  console.error('Terminal fit diagnostics: '+JSON.stringify({case:activeCase,state,calls}));throw error;
 }
}
async function replay(){await page.evaluate(async text=>{const e=currentTerminal();e.xterm.reset();appendTerminalOutput(e,new TextEncoder().encode(text));await terminalDisplayBarrier(e);e.xterm.scrollToBottom();e.highlighter.refresh();},transcript);await settle();}
async function snapshot(){return page.evaluate(()=>{const t=currentTerminal().xterm,b=t.buffer.active,logical=[];for(let y=0;y<b.length;y++){const l=b.getLine(y),text=l.translateToString(true);if(l.isWrapped&&logical.length)logical[logical.length-1]+=text;else logical.push(text);}return{logical:logical.filter(Boolean),cols:t.cols,rows:t.rows,cursorX:b.cursorX,cursorY:b.cursorY,baseY:b.baseY};});}
async function evidence(name){
 const data=await page.evaluate(()=>{const e=currentTerminal(),t=e.xterm,b=t.buffer.active,plain=[],cells=[];for(let y=0;y<b.length;y++){const line=b.getLine(y),text=line.translateToString(true);if(text.startsWith('(C) Copyright')||text.startsWith('to be bound')||text.startsWith('User "admin"')||text.startsWith('Device ANSI'))plain.push({text,tokens:e.highlighter.tokensAtRow(y)});if(text.startsWith('Device ANSI'))cells.push({color:line.getCell(0).getFgColor(),palette:!!line.getCell(0).isFgPalette()});}
 const row=e.element.querySelector('.xterm-rows>div'),screen=e.element.querySelector('.xterm-screen'),r=row?.getBoundingClientRect(),s=screen?.getBoundingClientRect();return{plain,cells,dpr:devicePixelRatio,screen:s?{width:s.width,height:s.height}:null,row:r?{width:r.width,height:r.height}:null,cols:t.cols,rows:t.rows};});
 for(const line of data.plain)assert.equal(line.tokens.length,0,'Ordinary banner received Toolkit decorations: '+line.text);assert.deepEqual(data.cells,[{color:3,palette:true}]);assert(data.row.width>0&&data.row.height>0);assert(Math.abs(data.screen.width-data.row.width)<2);report.layouts.push({name,...data});
}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,executablePath:process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox']});
 for(const variant of [{mode:'main',scale:1},{mode:'terminal',scale:1},{mode:'detached',scale:1},{mode:'main',scale:1.25},{mode:'main',scale:1.5},{mode:'main',scale:2}]){
  const {mode,scale}=variant,name=mode+'-scale'+scale;activeCase=name;
  const context=await browser.newContext({viewport:{width:1280,height:1000},deviceScaleFactor:scale,serviceWorkers:'block'});page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',error=>report.errors.push(name+': '+error.message));
  const session={id:'style-fixture',label:'Synthetic style terminal',type:'ssh',host:'style.test',port:22,state:'connected',cols:80,rows:24},view={id:'style-view',sessionId:session.id,state:'detached',revision:0};
  const calls=[];activeCalls=calls;let saved={appUpdatePreferences:{enabled:false},activePage:'terminal',terminalSidebarCollapsed:true,terminalSidebarTab:'saved',terminalAppearance:{fontFamily:'Consolas',highlightColors:{address:'#22aaff'},highlightUnderlines:false},terminalInteraction:{autoPager:false,copyOnSelect:false,copyOnClick:false}},assetErrors=[];
  page.on('response',response=>{if(response.url().includes('/assets/')&&response.status()>=400)assetErrors.push(response.url());});
  await page.route('**/*',async route=>{
   const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method(),body=request.postData()?request.postDataJSON():null;
   if(url.origin!=='http://toolkit.local'){report.errors.push('Unexpected external/font request: '+url.href);return route.abort();}
   if(p.startsWith('/api/')){
    calls.push({p,method,body});let data={};
    if(p==='/api/status')data={version,os:'windows',arch:'amd64',terminalWindowAvailable:true};
    if(p==='/api/state'){if(method==='GET')data=saved;else saved=body;}
    if(p==='/api/jobs')data=[];
    if(p==='/api/terminals')data={sessions:[session]};
    if(p==='/api/terminal-views')data={available:true,views:[],owners:{}};
    if(p==='/api/terminal-views/current')data={view,session,snapshot:{fresh:true,cols:80,rows:24,after:0,config:{type:'ssh',host:'style.test',port:22}}};
    if(p==='/api/terminal-views/current/ready')data={view};
    if(p==='/api/credentials')data={vault:{available:true,state:'locked'},entries:[]};
    if(p==='/api/credential-vault')data={available:true,state:'locked'};
    if(p==='/api/terminal-automation/library')data={macros:[],bindings:[],revision:'fixture'};
    if(p==='/api/terminal-automation/targets')data={targets:[]};
    if(p==='/api/terminal-automation/runs')data={runs:[]};
    if(p.endsWith('/resize'))Object.assign(session,body);
    if(p.includes('/events')){await pause(80);data={sessions:[session],events:[],after:0};}
    return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
   }
   try{const file=path.join(root,'web',p==='/'?'index.html':p);let content=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));if(p==='/')content=content.toString().replaceAll('__SESSION_TOKEN__','style-fixture').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__',JSON.stringify(mode==='detached'?view:null));return route.fulfill({body:content,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
  });
  await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);if(mode==='main')await page.evaluate(()=>navigate('terminal'));
  await page.waitForFunction(()=>!!currentTerminal()?.xterm&&terminalEntryAcceptsInput(currentTerminal()));await fitted();await replay();
  const baseline=await snapshot();assert.equal(baseline.cursorX,8);await evidence(name+'-normal');await shot(name+'-normal-output');
  await display();await page.locator('#terminalAppearanceFont').fill('Missing Synthetic Font 107');await page.locator('#terminalAppearanceFont').press('Tab');await page.locator('#terminalFontSample').scrollIntoViewIfNeeded();
  await page.waitForFunction(()=>state.terminalAppearance.fontFamily==='Missing Synthetic Font 107');assert.match(await page.locator('#terminalFontAvailability').textContent(),/not detected|detection is unavailable/);await shot(name+'-font-fallback');
  assert.match(await page.evaluate(()=>currentTerminal().xterm.options.fontFamily),/^"Missing Synthetic Font 107".*monospace$/);
  // Size changes traverse the real appearance handler, xterm metrics, guarded
  // fitting, and recorded backend resize request. No resize transport is stubbed.
  for(const size of [10,20,13]){
   const before=calls.filter(call=>call.p.endsWith('/resize')).length;await page.locator('#terminalFontSize').selectOption(String(size));await fitted();const after=await snapshot();assert(after.cols>0&&after.rows>0);assert(calls.filter(call=>call.p.endsWith('/resize')).length>before,'font size must notify backend in '+mode);assert.equal(await page.evaluate(()=>currentTerminal().xterm.options.fontSize),size);
   await page.evaluate(()=>{const t=currentTerminal().xterm,b=t.buffer.active;for(let row=0;row<b.length;row++){const line=b.getLine(row);if(line.translateToString(true).startsWith('│ IPv4')){t.select(0,row,23);return;}}throw Error('Box-drawing row missing after font resize');});assert.equal(await page.evaluate(()=>currentTerminal().xterm.getSelection()),'│ IPv4     │ IPv6     │');
   const unicode=await page.evaluate(()=>{const t=currentTerminal().xterm,b=t.buffer.active;for(let row=0;row<b.length;row++){const line=b.getLine(row);if(line.translateToString(true).startsWith('café ')){return {combined:line.getCell(5).getChars(),width:line.getCell(5).getWidth(),wide:line.getCell(11).getWidth()};}}return null;});assert.deepEqual(unicode,{combined:'é',width:1,wide:2});
  }
  await page.locator('#terminalHighlightRules summary').click();await page.locator('[data-terminal-highlight-rule="url"]').uncheck();await page.evaluate(()=>flushState());assert.equal(saved.terminalAppearance.highlightRules.url,false);assert.equal(saved.terminalAppearance.highlightColors.address,'#22aaff');assert.equal(saved.terminalAppearance.highlightUnderlines,false);await page.locator('[data-terminal-rules-reset]').click();assert.equal(await page.evaluate(()=>state.terminalAppearance.highlightRules.url),true);assert.equal(await page.evaluate(()=>state.terminalAppearance.fontFamily),'Missing Synthetic Font 107');
  if(scale===1){
   // The production resize listener intentionally closes Display. Await that
   // observable state before reopening; otherwise the pending resize can close
   // a panel that display() just observed as already open.
   assert.equal(await page.locator('#terminalSessionToggle').getAttribute('aria-expanded'),'true','Display is open before the real viewport resize');assert.equal(await page.locator('#terminalSessionPanel').isVisible(),true);await page.setViewportSize({width:390,height:1000});await page.waitForFunction(()=>document.querySelector('#terminalSessionPanel')?.classList.contains('hidden')&&document.querySelector('#terminalSessionToggle')?.getAttribute('aria-expanded')==='false');await settle();await display();await page.locator('#terminalFontSample').scrollIntoViewIfNeeded();await settle();assert(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth+1),'narrow page must not overflow');const bounds=await page.locator('#terminalSessionPanel').evaluate(el=>({width:el.clientWidth,scroll:el.scrollWidth}));assert(bounds.scroll<=bounds.width+1);await shot(name+'-narrow-settings');
   await page.locator('#terminalSessionToggle').click();await fitted();await replay();await evidence(name+'-narrow');await shot(name+'-narrow-output');
  }else await page.locator('#terminalSessionToggle').click();
  await page.evaluate(()=>flushState());await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);if(mode==='main')await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(()=>!!currentTerminal()?.xterm);assert.equal(await page.evaluate(()=>state.terminalAppearance.fontFamily),'Missing Synthetic Font 107');assert.equal(await page.evaluate(()=>state.terminalFontSize),13);assert.equal(await page.evaluate(()=>state.terminalAppearance.highlightColors.address),'#22aaff');
  assert.deepEqual(assetErrors,[]);assert(!calls.some(call=>call.method==='POST'&&call.p==='/api/terminals'));assert(!calls.some(call=>call.p.endsWith('/input')||call.p.startsWith('/api/credentials/')||call.method==='POST'&&call.p==='/api/terminal-automation/runs'));report.checks.push(name+': prose/ANSI, fallback, Unicode/box drawing, selection, 10/20/13 px backend resize, rules reset, preference reload, zero unintended connections/input');
  await context.close();page=null;
 }
 assert.deepEqual(report.errors,[]);report.passed=true;console.log(JSON.stringify(report));
})().catch(error=>{report.errors.push(error.stack||String(error));console.error(error.stack||String(error));process.exitCode=1;}).finally(async()=>{
 if(out){fs.writeFileSync(path.join(out,'terminal-style107-report.json'),JSON.stringify(report,null,2)+'\n');if(page&&!report.passed)await page.screenshot({path:path.join(out,'terminal-style107-failure.png'),fullPage:true}).catch(()=>{});}
 if(browser)await browser.close();
});
