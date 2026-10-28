'use strict';
// The complete bundled UI and renderer with isolated API state. Native Windows
// transcripts are replayed; this script does not execute shell commands.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,out=process.env.TOOLKIT_TEST_OUTPUT;
const captures=require('./fixtures/windows-local-shells.json').captures;
const report={passed:false,checks:[],layouts:[],errors:[],assetFailures:[]};let browser,page;
let saved={appUpdatePreferences:{enabled:false},terminalHighlight:false},sessions=[],serial=0;const requests=[];
const check=name=>{report.checks.push(name);console.log('PASS '+name);};
const settle=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));
async function start(type){
 await page.evaluate(type=>openTerminalConnect({type}),type);await page.locator('#terminalConnectButton').click();await page.locator('#terminalConnectForm').waitFor({state:'detached'});
 const id='shell85-'+serial;await page.waitForFunction(id=>terminalSessions.get(id)?.xterm,id);return id;
}
async function display(){if(await page.locator('#terminalSessionPanel.hidden').count())await page.locator('#terminalSessionToggle').click();await page.locator('#terminalAppearanceEditor').waitFor({state:'visible'});}
async function replay(id,base64){await page.evaluate(async({id,base64})=>{const e=terminalSessions.get(id);selectTerminal(id,false);e.xterm.reset();e.xterm.resize(120,20);appendTerminalOutput(e,Uint8Array.from(atob(base64),c=>c.charCodeAt(0)));await terminalDisplayBarrier(e);},{id,base64});await page.waitForFunction(id=>terminalSessions.get(id).element.querySelector('[data-nt-highlight="shell-variable"]'),id);await settle();await page.waitForFunction(id=>terminalSessions.get(id).info.type!=='cmd'||__shellColors(id,'%TEMP%').includes('rgb(140, 226, 182)'),id);}
async function buffer(id){return page.evaluate(id=>{const b=terminalSessions.get(id).xterm.buffer.normal;return Array.from({length:b.length},(_,y)=>b.getLine(y).translateToString(true)).join('\n').replace(/\n+$/,'');},id);}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 const context=await browser.newContext({viewport:{width:1440,height:1000},serviceWorkers:'block'});
 await context.addInitScript(()=>{globalThis.__shellColors=(id,word)=>{const host=terminalSessions.get(id).element;for(const row of host.querySelectorAll('.xterm-rows>div')){const start=row.textContent.indexOf(word);if(start<0)continue;const colors=[];let at=0;for(const span of row.children){const end=at+span.textContent.length;if(end>start&&at<start+word.length)colors.push(getComputedStyle(span).color);at=end;}return colors;}return [];};});
 page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));
 page.on('response',r=>{if(r.url().includes('/assets/')&&r.status()>=400)report.assetFailures.push(r.url());});
 await context.route('**/*',async route=>{
  const req=route.request(),url=new URL(req.url());assert.equal(url.origin,'https://shells.toolkit.test','Unexpected request '+req.url());const p=url.pathname,body=req.postData()?req.postDataJSON():null;
  if(p.startsWith('/api/')){
   requests.push({path:p,method:req.method(),body});let data={};
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',localShellAvailable:true,terminalWindowAvailable:true};
   if(p==='/api/state'){if(req.method()==='GET')data=saved;else saved=body;}
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminals'){if(req.method()==='POST'){const info={id:'shell85-'+(++serial),type:body.type,label:body.type==='cmd'?'Command Prompt':'PowerShell',state:'connected',cols:120,rows:20};sessions.push(info);data={session:info};}else data={sessions};}
   const session=p.match(/^\/api\/terminals\/([^/]+)(\/disconnect)?$/);if(session){const info=sessions.find(s=>s.id===session[1]);if(session[2]&&info)info.state='closed';else if(req.method()==='DELETE')sessions=sessions.filter(s=>s.id!==session[1]);}
   if(p==='/api/terminal-views')data={available:true,owners:{},views:[]};
   if(p==='/api/credentials')data={entries:[]};if(p==='/api/remote-browser')data={sessions:[]};
   if(p==='/api/terminal-automation/library')data={revision:'fixture',macros:[],bindings:[]};if(p==='/api/terminal-automation/targets')data={targets:[]};if(p==='/api/terminal-automation/runs')data={runs:[]};
   if(p==='/api/updates/status')data={currentVersion:version,supported:true,hasUpdate:false};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions,after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{const file=path.join(root,'web',p==='/'?'index.html':p),fallback=path.join(root,p);let data=fs.readFileSync(fs.existsSync(file)?file:fallback);if(p==='/')data=data.toString().replaceAll('__SESSION_TOKEN__','features-85').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body:data,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto('https://shells.toolkit.test/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
 await page.locator('#mobileMenu').click();await page.waitForFunction(()=>state.navigationCollapsed===true);assert(await page.locator('body').evaluate(el=>el.classList.contains('app-nav-collapsed')));
 for(const id of ['ping','hosts','terminal']){await page.locator('#navigation [data-page="'+id+'"]').click();await page.waitForFunction(id=>state.activePage===id,id);assert.equal(await page.evaluate(()=>state.navigationCollapsed),true);}
 await page.locator('#appNavigationDivider').click();assert.equal(await page.evaluate(()=>state.navigationCollapsed),true);await page.locator('#appNavigationDivider').dblclick();assert.equal(await page.evaluate(()=>state.navigationCollapsed),false);
 await page.locator('#mobileMenu').click();await page.locator('#appNavigationDivider').focus();await page.keyboard.press('Enter');assert.equal(await page.evaluate(()=>state.navigationCollapsed),false);await page.locator('#mobileMenu').click();
 await page.evaluate(()=>flushState());await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);assert.equal(await page.evaluate(()=>state.navigationCollapsed),true);await page.evaluate(()=>navigate('terminal'));
 check('collapsed tool icons navigate without opening; arrow, divider double-click and keyboard toggle; preference reloads');
 const cmd=await start('cmd'),ps=await start('powershell');await replay(cmd,captures.cmd);
 assert.equal(await page.evaluate(()=>state.terminalHighlight),false);assert.deepEqual([...new Set(await page.evaluate(id=>__shellColors(id,'%TEMP%'),cmd))],['rgb(140, 226, 182)']);
 await replay(ps,captures.powershell);const original=await buffer(ps);
 await display();await page.locator('[data-terminal-shell-card="powershell"] summary').click();await page.locator('[data-terminal-shell-color="powershell:variable"]').click();const picker=page.locator('[data-color-picker]');await picker.locator('[data-color-value="hex"]').fill('#00ab91');await picker.locator('[data-color-apply]').click();
 await page.waitForFunction(id=>__shellColors(id,'$env:TEMP').includes('rgb(0, 171, 145)'),ps);assert.equal(await buffer(ps),original);
 await page.locator('[data-terminal-shell-enabled="powershell"]').uncheck();await page.waitForFunction(id=>terminalSessions.get(id).highlighter.getState().enabled===false,ps);assert.equal(await page.evaluate(id=>terminalSessions.get(id).highlighter.getState().enabled,cmd),true);
 await page.locator('[data-terminal-shell-enabled="powershell"]').check();await page.evaluate(()=>{applyTerminalWorkspaceDisplay({...terminalWorkspaceDisplay(),terminalHighlight:false});});assert.equal(await page.evaluate(id=>terminalSessions.get(id).highlighter.getState().enabled,ps),true);
 await page.evaluate(()=>flushState());await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(id=>terminalSessions.get(id)?.xterm,ps);assert.equal(await page.evaluate(()=>state.terminalAppearance.shells.powershell.colors.variable),'#00ab91');await replay(ps,captures.powershell);
 await page.waitForFunction(id=>__shellColors(id,'$env:TEMP').includes('rgb(0, 171, 145)'),ps);
 check('real app session rendering replays native cmd and PowerShell transcripts with independent settings, custom native-token colors and reload persistence');
 await display();await page.locator('[data-terminal-shell-card="powershell"] summary').click();await page.locator('[data-terminal-shell-reset="powershell"]').click();assert.deepEqual(await page.evaluate(()=>state.terminalAppearance.shells.powershell.colors),{});
 for(const width of [1440,820,390])for(const theme of ['dark','light']){
  await page.setViewportSize({width,height:1000});await page.evaluate(theme=>setTheme(theme),theme);await display();
  for(const type of ['powershell','cmd'])await page.locator('[data-terminal-shell-card="'+type+'"] details').evaluate(el=>el.open=true);
  await page.locator('.terminal-local-shell-settings').scrollIntoViewIfNeeded();await settle();
  const metrics=await page.locator('#terminalSessionPanel').evaluate(panel=>{const r=panel.getBoundingClientRect(),bad=[];for(const input of panel.querySelectorAll('[data-terminal-shell-color],[data-terminal-shell-hex],#terminalConfirmClose')){const b=input.getBoundingClientRect();if(b.width&&b.height&&(b.left<r.left-1||b.right>r.right+1))bad.push(input.getAttribute('aria-label')||input.id);}for(const label of panel.querySelectorAll('.terminal-shell-settings .terminal-color-control')){const children=[...label.children].map(el=>el.getBoundingClientRect());for(let i=0;i<children.length;i++)for(let j=i+1;j<children.length;j++){const a=children[i],b=children[j];if(Math.min(a.right,b.right)-Math.max(a.left,b.left)>1&&Math.min(a.bottom,b.bottom)-Math.max(a.top,b.top)>1)bad.push('color field overlap');}}return {left:r.left,right:r.right,viewport:innerWidth,scroll:panel.scrollWidth,width:panel.clientWidth,bad};});
  assert(metrics.left>=-1&&metrics.right<=width+1,JSON.stringify(metrics));assert(metrics.scroll<=metrics.width+1,JSON.stringify(metrics));assert.deepEqual(metrics.bad,[],JSON.stringify({width,theme,metrics}));assert.equal(await page.locator('[data-terminal-shell-color]').count(),32);assert.equal(await page.locator('[data-terminal-shell-color].ntk-color-trigger').count(),32);assert.equal(await page.locator('[data-terminal-shell-hex]').count(),0);
  report.layouts.push({width,theme,metrics});if(out&&(width===1440||width===390))await page.screenshot({path:path.join(out,'shell85-settings-'+theme+'-'+width+'.png')});
 }
 check('separate shell settings and color fields fit in six desktop/compact layouts without input overlap');
 await page.setViewportSize({width:1440,height:1000});await page.evaluate(()=>setTheme('dark'));await replay(cmd,Buffer.from('\x1b[37;40mC:\\Work> echo %TEMP% && dir /a').toString('base64'));assert.deepEqual([...new Set(await page.evaluate(id=>__shellColors(id,'%TEMP%'),cmd))],['rgb(140, 226, 182)']);if(out)await page.screenshot({path:path.join(out,'shell85-live-console.png')});
 await page.locator('[data-terminal-close="'+cmd+'"]').click();await page.locator('#terminalCloseDontAsk').check();await page.locator('.modal-foot [data-action="close-modal"]').click();assert.equal(await page.evaluate(()=>state.terminalConfirmClose),true);assert(sessions.some(s=>s.id===cmd&&s.state==='connected'));
 await page.locator('[data-terminal-close="'+cmd+'"]').click();await page.locator('#terminalCloseDontAsk').check();await page.locator('[data-terminal-remove-confirm]').click();await page.waitForFunction(id=>!terminalSessions.has(id),cmd);assert.equal(await page.evaluate(()=>state.terminalConfirmClose),false);
 assert.deepEqual(requests.filter(r=>r.path==='/api/terminals/'+cmd||r.path==='/api/terminals/'+cmd+'/disconnect').map(r=>[r.path,r.method]),[['/api/terminals/'+cmd+'/disconnect','POST'],['/api/terminals/'+cmd,'DELETE']]);
 await page.evaluate(()=>flushState());await page.reload();await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('terminal'));assert.equal(await page.evaluate(()=>state.terminalConfirmClose),false);await page.waitForFunction(id=>terminalSessions.get(id)?.xterm,ps);await page.locator('[data-terminal-close="'+ps+'"]').click();await page.waitForFunction(id=>!terminalSessions.has(id),ps);assert.equal(await page.locator('.modal').count(),0);
 check('single close confirms by default, cancel does not opt out, confirmed opt-out persists, disconnect then delete skips later prompts');
 const keep=await start('cmd'),other=await start('powershell');await display();await page.locator('#terminalConfirmClose').check();await page.locator('[data-terminal-extra="close-session"]').click();
 await page.locator('[data-terminal-tab="'+keep+'"]').click({button:'right'});await page.locator('[data-terminal-tab-action="close-others"]').click();await page.locator('#terminalBulkCloseDontAsk').check();await page.locator('[data-terminal-tab-confirm]').click();await page.waitForFunction(id=>!terminalSessions.has(id),other);assert.equal(await page.evaluate(()=>state.terminalConfirmClose),false);assert(sessions.some(s=>s.id===keep));
 const immediate=await start('powershell');await page.locator('[data-terminal-tab="'+keep+'"]').click({button:'right'});await page.locator('[data-terminal-tab-action="close-others"]').click();await page.waitForFunction(id=>!terminalSessions.has(id),immediate);assert.equal(await page.locator('.modal').count(),0);
 await display();await page.locator('#terminalConfirmClose').check();await page.locator('[data-terminal-extra="close-session"]').click();await page.locator('[data-terminal-close="'+keep+'"]').click();await page.locator('#terminalCloseDontAsk').waitFor();await page.locator('.modal-foot [data-action="close-modal"]').click();
 assert.equal(await page.evaluate(()=>state.terminalSessionConfirmDelete),true);check('Close other tabs shares the preference and Display can restore confirmation without changing saved-entry deletion');
 await page.setViewportSize({width:820,height:1000});await page.locator('#mobileMenu').click();await page.locator('#navigation [data-page="ping"]').click();assert.equal(await page.evaluate(()=>state.activePage),'ping');assert.equal(await page.locator('#sidebar').evaluate(el=>el.classList.contains('open')),false);assert.equal(await page.locator('.main').evaluate(el=>el.inert),false);check('mobile tool drawer continues to navigate, close and release focus/inert state');
 assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetFailures,[]);report.passed=true;
})().catch(async error=>{report.error=error.stack||String(error);console.error(report.error);if(out&&page)await page.screenshot({path:path.join(out,'shell85-failure.png')}).catch(()=>{});process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'shell85-results.json'),JSON.stringify(report,null,2));await browser?.close();});
