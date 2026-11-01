'use strict';
// Synthetic test only: no desktop, device or production network calls.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||path.join(root,'test-results','ux-phase5');
let browser,stored={appUpdatePreferences:{enabled:false},profiles:[{id:'kept',name:'Existing target',target:'192.0.2.10',note:'keep',tool:'ping'}],firstLaunchWelcomeSeen:false};
const traffic=[],errors=[],screenshots=[],checks=[];
async function make(viewport){
 const context=await browser.newContext({viewport,serviceWorkers:'block'}),p=await context.newPage();
 p.setDefaultTimeout(15000);p.on('pageerror',e=>errors.push(e.message));
 await context.route('**/*',async route=>{
  if(await handleEditorActivityMetadata(route,'https://phase5.toolkit.test'))return;
  const request=route.request(),url=new URL(request.url()),name=url.pathname,method=request.method();
  assert.equal(url.origin,'https://phase5.toolkit.test');traffic.push(method+' '+name);
  if(name.startsWith('/api/')){
   assert(['GET','PUT'].includes(method),'Unexpected API write: '+method+' '+name);
   let data={};
   if(name==='/api/status')data={version:'1.18.4',os:'windows',arch:'amd64',system:{hostname:'SYNTHETIC-PHASE5',interfaces:[]}};
   if(name==='/api/state'){if(method==='PUT'){stored=request.postDataJSON();}else data=stored;}
   if(name==='/api/system')data={hostname:'SYNTHETIC-PHASE5',interfaces:[],gateways:[],dnsServers:[]};
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
   if(name.includes('/events'))data={events:[],sessions:[],after:0};
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  const file=name==='/'?path.join(root,'web/index.html'):path.join(root,'web',name);
  try{let body=fs.readFileSync(fs.existsSync(file)?file:path.join(root,name));
   if(name==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','synthetic-phase5').replaceAll('__WINDOW_MODE__','main').replaceAll('__EDITOR_VIEW__','null').replaceAll('__TERMINAL_VIEW__','null');
   return route.fulfill({body,contentType:name==='/'?'text/html':name.endsWith('.js')?'application/javascript':name.endsWith('.css')?'text/css':name.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 await p.goto('https://phase5.toolkit.test/');
 await p.waitForFunction(()=>window.__networkToolkitStartupState===1&&online&&stateLoaded);
 return{p,context};
}
(async()=>{
 fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 let {p,context}=await make({width:1280,height:800});
 await p.locator('#uxFirstLaunchWelcome').waitFor();
 assert(await p.getByRole('button',{name:'Open a terminal'}).isVisible());
 assert.equal(await p.evaluate(()=>state.profiles[0].name),'Existing target');
 await p.screenshot({path:path.join(out,'welcome-dark-1280.png')});screenshots.push('welcome-dark-1280.png');
 checks.push('First launch displays choices and preserves profiles');
 await p.locator('#uxFirstLaunchWelcome [data-ux-welcome-page="quick"]').click();
 assert.equal(await p.evaluate(()=>state.activePage),'quick');
 await p.waitForFunction(()=>state.firstLaunchWelcomeSeen===true);
 await p.waitForFunction(()=>document.querySelector('#uxFirstLaunchWelcome')===null);
 await p.waitForFunction(()=>!window.NetworkToolkitPhase5||state.firstLaunchWelcomeSeen===true);
 await p.waitForTimeout(850);
 assert.equal(stored.firstLaunchWelcomeSeen,true);
 assert.equal(stored.profiles[0].target,'192.0.2.10');
 assert(!traffic.some(x=>x==='POST /api/jobs'));
 checks.push('Quick start navigates without scanning, dismissal persists');
 await p.evaluate(()=>navigate('ping'));
 assert((await p.locator('#toolResult').textContent()).includes('Measure response time'));
 assert((await p.locator('#toolResult').textContent()).includes('192.0.2.10'));
 checks.push('No-result screen explains what to enter');
 await p.evaluate(()=>{
  jobs=[{id:'synthetic-failure',tool:'ping',target:'192.0.2.10',status:'failed',started:new Date().toISOString(),finished:new Date().toISOString(),error:'Synthetic transport failed',options:{},events:[],result:{}}];
  selected.ping='synthetic-failure';lastRender='';renderCurrentResult(true);
 });
 assert((await p.locator('#toolResult').textContent()).includes('Synthetic transport failed'));
 assert((await p.locator('#toolResult').textContent()).includes('Check the target, your network connection'));
 checks.push('Failure shows original error and actionable retry guidance');
 await p.locator('#uxHelpLauncher').click();
 await p.getByRole('button',{name:'Show welcome and quick starts'}).click();
 assert(await p.locator('#uxFirstLaunchWelcome').isVisible());
 await p.locator('#uxFirstLaunchWelcome [data-ux-welcome-close]').click();
 checks.push('F1 help can reopen dismissed welcome');
 await context.close();
 ({p,context}=await make({width:1920,height:1080}));
 assert.equal(await p.locator('#uxFirstLaunchWelcome').count(),0);
 checks.push('Returning user does not see welcome again');
 await p.locator('#themeButton').click();await p.locator('#uxHelpLauncher').click();
 await p.getByRole('button',{name:'Show welcome and quick starts'}).click();
 await p.screenshot({path:path.join(out,'welcome-light-1920.png')});screenshots.push('welcome-light-1920.png');
 await context.close();
 delete stored.firstLaunchWelcomeSeen;
 ({p,context}=await make({width:1280,height:800}));
 assert.equal(await p.locator('#uxFirstLaunchWelcome').count(),0);
 assert.equal(await p.evaluate(()=>state.profiles[0].name),'Existing target');
 checks.push('Upgraded existing users keep profiles and do not get an unexpected welcome');
 await context.close();
 if(errors.length)throw Error('Browser errors: '+errors.join('; '));
 console.log('PASS '+checks.join(' | '));
})().catch(e=>{console.error(e.stack||e);process.exitCode=1;}).finally(async()=>{
 fs.mkdirSync(out,{recursive:true});fs.writeFileSync(path.join(out,'phase5-results.json'),JSON.stringify({passed:process.exitCode!==1,checks,errors,traffic,screenshots},null,2));
 await browser?.close();
});
