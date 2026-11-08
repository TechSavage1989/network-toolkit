'use strict';
// The terminal Tools menu is gone (owner request, v1.21.18): its unique items
// moved to the Saved sidebar footer (Password manager, SSH host keys), beside
// the layout picker (Pane shortcuts), the Start a terminal card (Separate
// terminal window) and, in separate terminal windows, the toolbar (theme, app
// settings, help). Production assets with intercepted synthetic data; no
// connections, devices, files or credentials are used.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT,origin='https://tools-removed.toolkit.test';
let stored={appUpdatePreferences:{enabled:false},theme:'light',activePage:'terminal',terminalAppearance:{fontFamily:'Consolas',highlightColors:{address:'#22aaff'}}};
const report={passed:false,checks:[],errors:[],screenshots:[],scope:'Synthetic production-asset browser acceptance in Chromium. Not native Windows WebView2 acceptance.'};
const pass=text=>{report.checks.push(text);console.log('PASS '+text);};
let browser,page;
async function shot(name){if(!out)return;const file='tools-removed-'+name+'.png';await page.screenshot({path:path.join(out,file)});report.screenshots.push(file);}

async function open(mode){
 const context=await browser.newContext({viewport:{width:1280,height:800},serviceWorkers:'block'});page=await context.newPage();page.setDefaultTimeout(15000);
 page.on('pageerror',error=>report.errors.push(mode+': '+error.message));
 await page.route('**/*',async route=>{if(await handleEditorActivityMetadata(route,origin,report))return;
  const req=route.request(),url=new URL(req.url()),p=url.pathname;assert.equal(url.origin,origin,'No external requests are allowed');
  if(p.startsWith('/api/')){const method=req.method();let data={};
   if(method!=='GET')assert.equal(p,'/api/state','Opening these controls must not start connections or jobs');
   if(p==='/api/state'&&method==='GET')data=stored;else if(p==='/api/state')stored=req.postDataJSON();
   if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',storageWritable:true,terminalWindowAvailable:true,system:{hostname:'Tools fixture',interfaces:[]}};
   if(p==='/api/system')data={hostname:'Tools fixture',interfaces:[]};
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};
   if(p==='/api/terminals/known-hosts')data={hosts:[{host:'192.0.2.10',port:22,algorithm:'ssh-ed25519',fingerprint:'SHA256:synthetic-fixture-key'}]};
   if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   if(p==='/api/credential-vault')data={available:false,state:'unavailable'};
   if(p==='/api/credentials')data={vault:{available:false,state:'unavailable'},entries:[]};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,80));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{const icon=/^\/assets\/Network_Toolkit_Icon\.(svg|ico)$/.test(p)||p==='/favicon.ico',file=icon?path.join(root,'assets',p==='/favicon.ico'?'Network_Toolkit_Icon.ico':path.basename(p)):path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','tools-fixture').replaceAll('__WINDOW_MODE__',mode);return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.ico')?'image/x-icon':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto(origin+'/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);
 if(mode==='main')await page.evaluate(()=>navigate('terminal'));
 await page.locator('#terminalSessionToggle').waitFor();
 return context;
}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true});
 let context=await open('main');
 const bar=page.locator('#terminalCommandBar');
 assert.equal(await page.locator('#terminalToolsToggle').count(),0,'the Tools button is gone');
 assert.equal(await page.locator('#terminalToolsPanel').isVisible(),false,'nothing is shown from the old Tools panel');
 assert.deepEqual(await bar.locator('.terminal-quick-tools > button.btn:visible').allTextContents(),['Command bar','Settings'],'Command bar and Settings stay at the end of the toolbar');
 assert.equal(await page.locator('.terminal-app-tools').count(),0,'the home window uses the app sidebar for theme, settings and help');
 // The active session buttons and the Settings panel still come from the staging container.
 assert(await page.locator('#terminalActiveActions [data-terminal-action="disconnect"]').isVisible());
 await page.locator('#terminalSessionToggle').click();await page.locator('#terminalSessionPanel [data-terminal-settings-tab="highlighting"]').click();assert(await page.locator('#terminalSyntaxSelect').isVisible(),'Settings still holds the highlighting profile');
 await page.locator('[data-terminal-extra="close-session"]').click();
 pass('the toolbar has no Tools menu; Command bar, Settings and the active session buttons remain');
 // Pane shortcuts beside the layout picker.
 await page.locator('.terminal-command-start [data-terminal-action="layout-help"]').click();await page.locator('#modalRoot').getByText('Ctrl+Alt+1').waitFor();
 await shot('pane-shortcuts');await page.locator('#modalRoot [data-action="close-modal"]').first().click();
 pass('Pane shortcuts opens from the button beside the layout picker');
 // Password manager and SSH host keys in the Saved sidebar footer.
 const footer=page.locator('#sessionLibrarySettings .session-library-tools');
 assert.deepEqual((await footer.locator('button').allTextContents()).map(text=>text.trim()),['Password manager','SSH host keys']);
 await footer.locator('[data-terminal-action="known-hosts"]').click();await page.locator('#modalRoot').getByText('SHA256:synthetic-fixture-key').waitFor();
 await page.locator('#modalRoot [data-action="close-modal"]').first().click();
 await page.locator('[data-session-tab="recent"]').click();assert.equal(await footer.isVisible(),true,'also shown with Recent');await page.locator('[data-session-tab="saved"]').click();
 await shot('sidebar-footer');
 pass('Password manager and SSH host keys sit in the Saved and Recent sidebar footer');
 // Separate terminal window on the Start a terminal card.
 const separate=page.locator('#terminalStage [data-terminal-action="separate-window"]').first();
 assert.equal(await separate.isVisible(),true);assert.equal(await separate.isEnabled(),true,'available when the app can open terminal windows');
 await shot('start-card');
 pass('Separate terminal window is offered on the Start a terminal card');
 await context.close();
 // A separate terminal window has no app sidebar: theme, settings and help are in its toolbar.
 context=await open('terminal');
 assert.equal(await page.locator('#terminalToolsToggle').count(),0);
 const app=page.locator('#terminalCommandBar .terminal-app-tools');
 assert.deepEqual(await app.locator('button').evaluateAll(buttons=>buttons.map(b=>b.getAttribute('aria-label'))),['Switch color theme','App settings','Keyboard shortcuts and help']);
 await app.locator('#uxTerminalHelpLauncher').click();await page.locator('[aria-label="Keyboard shortcuts and help"][role="dialog"]').waitFor();await page.keyboard.press('Escape');
 assert.equal(await page.locator('#terminalStage [data-terminal-action="separate-window"]').count(),0,'no nested separate window from a separate window');
 await shot('terminal-window');
 pass('separate terminal windows keep theme, app settings and help in the toolbar');
 await context.close();
 assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.errors.push(String(error?.stack||error));console.error(error);process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'tools-removed-results.json'),JSON.stringify(report,null,2));await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,errors:report.errors}));});
