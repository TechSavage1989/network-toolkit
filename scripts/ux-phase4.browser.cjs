'use strict';
// Synthetic, isolated UI regression. No real devices, credentials or system actions.
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict'),{chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||path.join(root,'test-results','ux-phase4');
const report={passed:false,checks:[],errors:[],assetFailures:[],screenshots:[],scope:'Synthetic production DOM only; no real device access or physical Windows'};
const hostname='SYNTHETIC-UX-PHASE4';let browser,current;
const settle=p=>p.evaluate(()=>new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(r)))));
async function make(repo,viewport,mode='main',dpr=1){
 const context=await browser.newContext({viewport,deviceScaleFactor:dpr,serviceWorkers:'block'}),p=await context.newPage(),version=JSON.parse(fs.readFileSync(path.join(repo,'package.json'))).version;
 p.setDefaultTimeout(15000);p.on('pageerror',e=>report.errors.push({repo:path.basename(repo),error:e.message}));p.on('response',r=>{if(r.status()>=400&&!new URL(r.url()).pathname.startsWith('/api/'))report.assetFailures.push(r.url());});
 await context.route('**/*',async route=>{if(await handleEditorActivityMetadata(route,'https://chrome109.toolkit.test',report))return;
  const request=route.request(),url=new URL(request.url()),name=url.pathname,method=request.method();assert.equal(url.origin,'https://chrome109.toolkit.test');
  if(name.startsWith('/api/')){
   assert(method==='GET'||(name==='/api/state'&&method==='PUT')||(name==='/api/packet-captures/interfaces'&&method==='POST'),'Unexpected operation: '+method+' '+name);let data={};
   if(name==='/api/status')data={version,os:'windows',arch:'amd64',system:{hostname,interfaces:[]}};
   if(name==='/api/state'&&method==='GET')data={appUpdatePreferences:{enabled:false},navigationCollapsed:false};
   if(name==='/api/system')data={hostname,interfaces:[],gateways:[],dnsServers:[]};
   if(name==='/api/jobs')data=[];
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
 await p.locator('#uxSettingsShell').waitFor();
 assert.equal(await p.locator('.ux-settings-nav [role="tab"]').count(),9);
 assert(await p.locator('#uxSettingsPanel-updates #appUpdateSettings').count());
 assert(await p.locator('#uxSettingsPanel-network #vendorSettings').count());
 assert(await p.locator('#uxSettingsPanel-general #storageSettings').count());
 check('Existing storage, vendor and updater controls retain their IDs under nine searchable settings categories');
 await p.locator('#uxSettingsSearch').fill('transfer');
 assert(await p.locator('[data-ux-settings-section="transfers"]').isVisible());
 assert.equal(await p.locator('[data-ux-settings-section="editor"]').isVisible(),false);
 await p.locator('#uxSettingsSearch').fill('');
 await p.locator('[data-ux-settings-section="editor"]').click();
 await p.locator('[data-ux-preference="editor.wordWrap"]').check();
 await p.locator('[data-ux-preference="editor.fontSize"]').fill('16');
 await p.locator('[data-ux-preference="editor.fontSize"]').press('Tab');
 assert.equal(await p.evaluate(()=>state.editorPreferences.wordWrap),true);
 assert.equal(await p.evaluate(()=>state.editorPreferences.fontSize),16);
 check('Editor preferences update the existing saved state without overwriting its other keys');
 await p.locator('[data-ux-settings-section="terminal"]').click();
 await p.locator('[data-ux-preference="terminalFontSize"]').fill('14');
 await p.locator('[data-ux-preference="terminalFontSize"]').press('Tab');
 assert.equal(await p.evaluate(()=>state.terminalFontSize),14);
 await p.locator('[data-ux-settings-section="confirmations"]').click();
 assert(await p.locator('#uxSettingsPanel-confirmations').innerText().then(t=>t.includes('overwrites')&&t.includes('protected network')));
 await p.locator('[data-ux-preference="editor.askSaveBeforeClose"]').uncheck();
 assert.equal(await p.evaluate(()=>state.editorPreferences.askSaveBeforeClose),false);
 await p.locator('[data-ux-preference="editor.askSaveBeforeClose"]').check();
 assert.equal(await p.evaluate(()=>state.editorPreferences.askSaveBeforeClose),true);
 check('Terminal display and confirmations are available without removing protected-operation warnings');
 await p.locator('[data-ux-settings-section="updates"]').click();
 assert(await p.locator('#appUpdateSettings').isVisible());
 for(const theme of ['dark','light']){
  await p.evaluate(theme=>setTheme(theme),theme);
  for(const [width,height] of [[1280,800],[1920,1080]]){
   await p.setViewportSize({width,height});await settle(p);
   const name='settings-'+theme+'-'+width+'x'+height+'.png';
   await p.screenshot({path:path.join(out,name)});report.screenshots.push(name);
  }
 }
 check('Settings reviewed at both approved resolutions in light and dark themes');
 await p.locator('.modal [data-action="close-modal"]').last().click();
 await p.evaluate(()=>{state.profiles=[{id:'phase4-target',name:'Synthetic switch',target:'192.0.2.3',tool:'ping',note:'Synthetic'}];navigate('profiles');});
 await p.locator('[data-action="delete-profile"][data-id="phase4-target"]').click();
 assert.equal(await p.evaluate(()=>state.profiles.length),0);
 await p.locator('#toastStack .toast button').last().click();
 assert.equal(await p.evaluate(()=>state.profiles[0]?.id),'phase4-target');
 check('Deleting a nonsecret saved diagnostic target offers safe same-session Undo');
 await p.evaluate(()=>navigate('editor'));
 await p.evaluate(async()=>{
  await NetworkToolkitFileEditor.open({id:'phase4-draft',path:'Phase4.txt',source:'Untitled',binding:{kind:'localDraft'},create:true,initialContent:'Original',
  read:async()=>({content:'Original'}),write:async()=>{throw Error('No save requested');}});
 });
 await p.locator('[data-ed-text]').fill('Important unsaved content');
 await p.locator('[data-ed-close-tab="phase4-draft"]').click();
 await p.locator('[data-ed-dialog-action="accept"]').waitFor();
 await p.locator('[data-ed-dialog-action="accept"]').click();
 await p.waitForFunction(()=>!NetworkToolkitFileEditor.documents.has('phase4-draft'));
 await p.keyboard.press('Control+Shift+T');
 await p.waitForFunction(()=>NetworkToolkitFileEditor.documents.has('phase4-draft'));
 assert.equal(await p.evaluate(()=>NetworkToolkitFileEditor.documents.get('phase4-draft').content),'Important unsaved content');
 assert.equal(await p.evaluate(()=>NetworkToolkitFileEditor.documents.get('phase4-draft').dirty),true);
 check('Ctrl+Shift+T restores a confirmed-closed Editor document and its unsaved bytes in memory');
 const terminalProtected=await p.evaluate(()=>{
  const el=document.createElement('textarea');el.className='xterm-helper-textarea';document.body.append(el);
  el.focus();const key=new KeyboardEvent('keydown',{key:'T',ctrlKey:true,shiftKey:true,bubbles:true,cancelable:true});
  el.dispatchEvent(key);el.remove();return key.defaultPrevented;
 });
 assert.equal(terminalProtected,false);
 check('Focused terminal keystrokes are not intercepted by the reopen shortcut');
 await p.evaluate(()=>{navigate('terminal');newTerminalTab();
  const id=[...terminalDraftTabs.keys()].at(-1);terminalDraftTabs.get(id).label='Lab draft';
  closeTerminalDraft(id);
 });
 await p.evaluate(()=>NetworkToolkitTabRecovery.reopen());
 assert(await p.evaluate(()=>[...terminalDraftTabs.values()].some(item=>item.label==='Lab draft')));
 check('Reopening an empty Terminal tab does not reconnect or reuse credentials');
 if(report.errors.length||report.assetFailures.length)throw Error(JSON.stringify(report));
 await context.close();report.passed=true;
})().catch(error=>{report.failure=error.stack||String(error);process.exitCode=1;}).finally(async()=>{
 fs.mkdirSync(out,{recursive:true});
 fs.writeFileSync(path.join(out,'ux-phase4-results.json'),JSON.stringify(report,null,2));
 await browser?.close();
 console.log(JSON.stringify({passed:report.passed,checks:report.checks,errors:report.errors,assets:report.assetFailures,failure:report.failure}));
});
