'use strict';
// Synthetic, isolated UI regression. No real devices, credentials or system actions.
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||path.join(root,'test-results','ux-phase4');
const report={passed:false,checks:[],errors:[],assetFailures:[],screenshots:[],scope:'Synthetic Phase 4 browser fixtures; no device or physical Windows access'};
const hostname='SYNTHETIC-UX-PHASE4';let browser,current;
const settle=p=>p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(r)))));
async function make(repo,viewport,mode='main',dpr=1){
 const context=await browser.newContext({viewport,deviceScaleFactor:dpr,serviceWorkers:'block'}),p=await context.newPage(),version=JSON.parse(fs.readFileSync(path.join(repo,'package.json'))).version;
 p.setDefaultTimeout(15000);p.on('pageerror',e=>report.errors.push({repo:path.basename(repo),error:e.message}));p.on('response',r=>{if(r.status()>=400&&!new URL(r.url()).pathname.startsWith('/api/'))report.assetFailures.push(r.url());});
 await context.route('**/*',async route=>{if(await handleEditorActivityMetadata(route,'https://chrome109.toolkit.test',report))return;
  const request=route.request(),url=new URL(request.url()),name=url.pathname,method=request.method();assert.equal(url.origin,'https://chrome109.toolkit.test');
  if(name.startsWith('/api/')){
   assert(method==='GET'||(name==='/api/state'&&method==='PUT')||(name==='/api/packet-captures/interfaces'&&method==='POST')||(name==='/api/file-editor/local'&&method==='POST'),'Unexpected operation: '+method+' '+name);let data={};
   if(name==='/api/status')data={version,os:'windows',arch:'amd64',system:{hostname,interfaces:[]}};
   if(name==='/api/state'&&method==='GET')data={appUpdatePreferences:{enabled:false},navigationCollapsed:false};
   if(name==='/api/system')data={hostname,interfaces:[],gateways:[],dnsServers:[]};
   if(name==='/api/jobs')data=[];
   if(name==='/api/file-editor/local')data={content:'hostname SYNTHETIC-SWITCH\\n',encoding:'UTF-8',hash:'synthetic-hash',bom:false};
   if(name==='/api/activity')data={tools:{}};
   if(name==='/api/terminals'||name==='/api/remote-browser')data={sessions:[]};
   if(name==='/api/terminal-views')data={available:false,views:[],owners:{}};
   if(name==='/api/terminal-window/requests')data={requests:[],after:0};
   if(name==='/api/credentials')data={entries:[],vault:{available:false,state:'unavailable'}};
   if(name==='/api/credential-vault')data={available:false,state:'unavailable'};
   if(name==='/api/local-tools/ip')data={adapters:[]};
   if(name==='/api/local-tools/hosts')data={available:true,editable:true,exists:true,path:'C:\\synthetic\\hosts',content:'127.0.0.1 localhost\n',revision:'synthetic',encoding:'UTF-8',newline:'LF',warnings:[]};
   if(name==='/api/packet-captures')data={captures:[]};
   if(name==='/api/packet-captures/interfaces')data={interfaces:[],engines:[]};
   if(name.includes('/events')){await new Promise(r=>setTimeout(r,70));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{const entry=name==='/',file=path.join(repo,'web',entry?'index.html':name);let body=fs.readFileSync(fs.existsSync(file)?file:path.join(repo,name));if(entry)body=body.toString().replaceAll('__SESSION_TOKEN__','synthetic-chrome109').replaceAll('__WINDOW_MODE__',mode).replaceAll('__EDITOR_VIEW__',mode==='editor'?JSON.stringify({id:'10910910910910910910910910910910',ownerInstanceId:'10810810810810810810810810810810'}):'null').replaceAll('__TERMINAL_VIEW__','null');return route.fulfill({body,contentType:entry?'text/html':name.endsWith('.js')?'application/javascript':name.endsWith('.css')?'text/css':name.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await p.goto('https://chrome109.toolkit.test/');await p.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await p.waitForFunction(()=>Array.from(document.images).every(img=>img.complete&&img.naturalWidth>0));if(mode==='editor')assert(await p.locator('body').evaluate(el=>el.classList.contains('editor-window')),'Synthetic child must enter the real scoped editor shell');if(mode==='terminal')assert(await p.locator('body').evaluate(el=>el.classList.contains('terminal-window')));await settle(p);return{p,context};
}
const check=name=>{report.checks.push(name);console.log('PASS '+name)};
(async()=>{
 fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 const {p,context}=await make(root,{width:1280,height:800});current=p;
 await p.locator('#settingsButton').click();
 await p.locator('#uxSettingsSearch').waitFor();
 assert.equal(await p.locator('[data-settings-section]').count(),9);
 await p.locator('[data-settings-section="appearance"]').click();
 assert(await p.locator('#settingsTheme').isVisible());
 await p.locator('#settingsTheme').selectOption('light');
 assert.equal(await p.evaluate(()=>state.theme),'light');
 await p.locator('#settingsTheme').selectOption('dark');
 await p.locator('[data-settings-section="updates"]').click();
 assert(await p.locator('#appUpdateSettings').isVisible());
 check('Searchable Settings retains the original live theme and updater controls');
 await p.locator('#uxSettingsSearch').fill('timezone');
 assert(await p.locator('[data-settings-panel="appearance"]').isVisible());
 assert.equal(await p.locator('[data-settings-panel="updates"]').isVisible(),false);
 await p.locator('#uxSettingsSearch').fill('');
 await p.locator('[data-settings-section="terminal"]').click();
 const go=p.locator('[data-settings-go="terminal"]');
 assert(await go.isVisible());
 await go.click();await p.waitForFunction(()=>state.activePage==='terminal');
 assert.equal(await p.locator('.ux-settings-layout').count(),0);
 check('Category search and links work without discarding existing preferences');
 await p.evaluate(()=>{
  state.profiles=[{id:'ux-profile',name:'Synthetic switch',target:'192.0.2.50',note:'lab',tool:'quick'}];
  saveState();navigate('profiles');
 });
 await p.locator('[data-action="delete-profile"][data-id="ux-profile"]').click();
 await p.locator('.ux-undo-toast button').waitFor();
 assert.equal(await p.evaluate(()=>state.profiles.length),0);
 assert.equal(await p.locator('#modalRoot .modal').count(),0);
 await p.locator('.ux-undo-toast button').click();
 assert.equal(await p.evaluate(()=>state.profiles.length),1);
 assert.equal(await p.evaluate(()=>state.profiles[0].target),'192.0.2.50');
 check('Deleting a saved target offers Undo and restores its exact record');
 await p.evaluate(()=>navigate('editor'));
 await p.evaluate(async()=>{
  await NetworkToolkitFileEditor.open({id:'local:C:\\synthetic\\switch.cfg',path:'C:\\synthetic\\switch.cfg',source:'Local computer',binding:{kind:'local'},create:true,initialContent:'hostname SYNTHETIC-SWITCH\\n',
   read:async()=>({content:'hostname SYNTHETIC-SWITCH\\n',hash:'synthetic-hash'}),
   write:async()=>{throw Error('No save expected');}});
 });
 await p.locator('[data-ed-tab-actions]').first().click();
 await p.locator('.nt-editor-tab-menu [data-ed-context-action="close"]').click();
 await p.waitForFunction(()=>NetworkToolkitFileEditor.documents.size===0);
 assert.equal(await p.evaluate(()=>NetworkToolkitClosedTabs.count()),1);
 await p.keyboard.press('Control+Shift+T');
 await p.waitForFunction(()=>NetworkToolkitFileEditor.documents.size===1);
 const reopened=await p.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].map(d=>({path:d.path,dirty:d.dirty,content:d.content})));
 assert.equal(reopened.length,1);
 assert.equal(reopened[0].content,'hostname SYNTHETIC-SWITCH\\n');
 assert.equal(reopened[0].dirty,false);
 check('Ctrl+Shift+T reopens a clean local Editor file from its path without storing unsaved content');
 await p.evaluate(()=>{navigate('terminal');newTerminalTab();const draft=[...terminalDraftTabs.values()].at(-1);draft.label='Synthetic draft';closeTerminalDraft(draft.id);});
 assert.equal(await p.evaluate(()=>NetworkToolkitClosedTabs.count()),1);
 await p.keyboard.press('Control+Shift+T');
 assert(await p.evaluate(()=>[...terminalDraftTabs.values()].some(x=>x.label==='Synthetic draft')));
 assert.equal(await p.evaluate(()=>terminalSessions.size),0);
 check('Closed unused Terminal tab returns without creating a remote connection');
 const protect=await p.evaluate(()=>{
  newTerminalTab();const draft=[...terminalDraftTabs.values()].at(-1);closeTerminalDraft(draft.id);
  const input=document.createElement('textarea');input.className='xterm-helper-textarea';document.body.append(input);input.focus();
  const event=new KeyboardEvent('keydown',{key:'T',ctrlKey:true,shiftKey:true,bubbles:true,cancelable:true});input.dispatchEvent(event);
  const value={prevented:event.defaultPrevented,history:NetworkToolkitClosedTabs.count()};input.remove();return value;
 });
 assert.equal(protect.prevented,false);assert(protect.history>=1);
 check('Ctrl+Shift+T leaves device-focused Terminal keystrokes untouched');
 await p.evaluate(()=>navigate('overview'));await settle(p);
 if(report.errors.length||report.assetFailures.length)throw Error(JSON.stringify(report));
 const name='phase4-settings.png';await p.locator('#settingsButton').click();await settle(p);
 await p.screenshot({path:path.join(out,name)});report.screenshots.push(name);
 await context.close();report.passed=true;
})().catch(async error=>{report.failure=error.stack||String(error);process.exitCode=1;}).finally(async()=>{
 fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'ux-phase4-results.json'),JSON.stringify(report,null,2));
 await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks,errors:report.errors,assets:report.assetFailures,failure:report.failure}));
});
