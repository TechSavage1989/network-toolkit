'use strict';
// Synthetic test only: no desktop, device or production network calls.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||path.join(root,'test-results','ux-phase6');
let browser,stored={appUpdatePreferences:{enabled:false},profiles:[{id:'kept',name:'Existing target',target:'192.0.2.10',note:'keep',tool:'dns'}],ftpProfiles:[{id:'ftp-kept',name:'Kept FTP',host:'192.0.2.20',port:21,username:'operator',security:'explicit'}],fileTransferProtocol:'ftp',firstLaunchWelcomeSeen:true,target:'192.0.2.10'};
let syntheticJobs=[],activity={tools:{}};
const traffic=[],errors=[],screenshots=[],checks=[];
async function make(viewport){
 const context=await browser.newContext({viewport,serviceWorkers:'block'}),p=await context.newPage();
 p.setDefaultTimeout(15000);p.on('pageerror',e=>errors.push(e.message));
 await context.route('**/*',async route=>{
  if(await handleEditorActivityMetadata(route,'https://phase6.toolkit.test'))return;
  const request=route.request(),url=new URL(request.url()),name=url.pathname,method=request.method();
  assert.equal(url.origin,'https://phase6.toolkit.test');traffic.push(method+' '+name);
  if(name.startsWith('/api/')){
   assert(['GET','PUT'].includes(method),'Unexpected API write: '+method+' '+name);
   let data={};
   if(name==='/api/status')data={version:'1.18.4',os:'windows',arch:'amd64',system:{hostname:'SYNTHETIC-PHASE6',interfaces:[]}};
   if(name==='/api/state'){if(method==='PUT'){stored=request.postDataJSON();}else data=stored;}
   if(name==='/api/system')data={hostname:'SYNTHETIC-PHASE6',interfaces:[],gateways:[],dnsServers:[]};
   if(name==='/api/jobs'||name==='/api/jobs?brief=1')data=syntheticJobs;
   if(name==='/api/activity')data=activity;
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
   if(name==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','synthetic-phase6').replaceAll('__WINDOW_MODE__','main').replaceAll('__EDITOR_VIEW__','null').replaceAll('__TERMINAL_VIEW__','null');
   return route.fulfill({body,contentType:name==='/'?'text/html':name.endsWith('.js')?'application/javascript':name.endsWith('.css')?'text/css':name.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 await p.goto('https://phase6.toolkit.test/');
 await p.waitForFunction(()=>window.__networkToolkitStartupState===1&&online&&stateLoaded);
 return{p,context};
}

const check=name=>{checks.push(name);console.log('PASS '+name);};
const navLabels=p=>p.locator('#navigation .nav-item span:not(.badge):not(.sidebar-activity-dot)').allTextContents();
(async()=>{
 fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 let {p,context}=await make({width:1280,height:800});
 const labels=await navLabels(p);
 for(const gone of ['Ping','Trace route','DNS lookup','TCP ports','TFTP','SFTP / FTP / FTPS'])assert(!labels.includes(gone),gone+' should be merged');
 assert(labels.includes('Test a target')&&labels.includes('File Transfer'));
 assert.equal(await p.evaluate(()=>state.fileTransferProtocol),'ftp','saved protocol migrates');
 assert.equal(await p.evaluate(()=>state.targetTest),'ping','missing test choice defaults to Ping');
 assert.equal(await p.evaluate(()=>state.profiles[0].name),'Existing target');
 check('Navigation merges four tests and three transfer protocols; existing preferences kept');
 await p.locator('#navigation .nav-item',{hasText:'Test a target'}).click();
 assert.equal(await p.evaluate(()=>state.activePage),'ping');
 assert.equal(await p.locator('.nt-target-tests [role="tab"]').count(),4);
 assert.equal(await p.inputValue('#globalTarget'),'192.0.2.10','target entered once is shared');
 await p.locator('.nt-target-tests [data-target-test="dns"]').click();
 assert.equal(await p.evaluate(()=>state.activePage),'dns');assert.equal(await p.inputValue('#globalTarget'),'192.0.2.10');
 assert.equal(await p.locator('#navigation .nav-item.active span').first().textContent(),'Test a target');
 assert(!traffic.some(x=>x==='POST /api/jobs'),'switching tests must not run anything');
 check('Test tabs share the target and never start a test on their own');
 const now=new Date().toISOString();
 syntheticJobs=[{id:'p6-ping',tool:'ping',target:'192.0.2.10',status:'completed',started:now,finished:now,eventSequence:1},{id:'p6-trace',tool:'trace',target:'192.0.2.10',status:'running',started:now,eventSequence:1},{id:'p6-tcp',tool:'tcp',target:'198.51.100.9',status:'failed',started:now,finished:now,error:'Synthetic',eventSequence:1}];
 await p.evaluate(()=>pollJobs());
 await p.waitForFunction(()=>document.querySelector('.nt-target-tests [data-target-test="trace"] .is-running'));
 const status=await p.evaluate(()=>Object.fromEntries([...document.querySelectorAll('.nt-target-tests [data-target-test]')].map(b=>[b.dataset.targetTest,b.querySelector('.nt-target-test-status').className.match(/is-(\w+)/)[1]])));
 assert.deepEqual(status,{ping:'done',trace:'running',dns:'none',tcp:'none'},'status reflects only the current target');
 await p.screenshot({path:path.join(out,'test-a-target-1280-dark.png')});screenshots.push('test-a-target-1280-dark.png');
 check('Each test tab shows its latest result for the current target');
 await p.evaluate(()=>navigate('overview'));
 await p.locator('#navigation .nav-item',{hasText:'Test a target'}).click();
 assert.equal(await p.evaluate(()=>state.activePage),'dns','returns to last test');
 await p.locator('#navigation .nav-item',{hasText:'File Transfer'}).click();
 assert.equal(await p.evaluate(()=>state.activePage),'ftp','returns to saved protocol');
 assert.equal(await p.locator('[data-transfer-protocol]').count(),3);
 assert.match(await p.locator('[data-transfer-protocol="tftp"]').getAttribute('title'),/no encryption/i);
 await p.locator('[data-transfer-protocol="tftp"]').click();
 assert.equal(await p.evaluate(()=>state.activePage),'tftp');
 assert.equal(await p.locator('#navigation .nav-item.active span').first().textContent(),'File Transfer');
 await p.screenshot({path:path.join(out,'file-transfer-tftp-1280-dark.png')});screenshots.push('file-transfer-tftp-1280-dark.png');
 assert.equal(await p.evaluate(()=>state.ftpProfiles[0].name),'Kept FTP');
 check('File Transfer picker includes TFTP, shows its security note and remembers the protocol');
 activity={tools:{tftp:{servers:1}}};await p.evaluate(()=>NetworkToolkitSidebarActivity.refresh());
 await p.waitForFunction(()=>[...document.querySelectorAll('#navigation .nav-item')].some(b=>/File Transfer, Server running/.test(b.getAttribute('aria-label')||'')));
 activity={tools:{}};
 check('Running TFTP server is still shown on the merged File Transfer entry');
 for(const [query,page] of [['TFTP','tftp'],['Trace route','trace'],['TCP ports','tcp'],['SFTP','sftp']]){
  await p.evaluate(()=>navigate('overview'));
  await p.keyboard.press('Control+Shift+P');await p.locator('.ux-command-input').fill(query);
  const option=p.locator('#uxCommandResults [role=option]').first();
  assert.match(await option.textContent(),new RegExp('^'+query));await option.click();
  await p.waitForFunction(page=>state.activePage===page,page);
 }
 check('Former tool names remain searchable in the command palette and open directly');
 await p.evaluate(()=>navigate('terminal'));await p.waitForTimeout(200);
 assert.equal(await p.locator('#terminalSessionSidebar [data-session-action="collapse"]').count(),0);
 assert.equal(await p.locator('#terminalSessionSidebar [data-session-tab]').count()>=3,true);
 await p.screenshot({path:path.join(out,'terminal-sidebar-1280-dark.png')});screenshots.push('terminal-sidebar-1280-dark.png');
 check('Terminal sidebar has no separate Sidebar button; its tabs remain');
 await p.waitForFunction(()=>savedStateRevision===stateRevision);
 assert.equal(stored.targetTest,'tcp');assert.equal(stored.fileTransferProtocol,'sftp');
 await context.close();({p,context}=await make({width:1920,height:1080}));
 assert.equal(await p.evaluate(()=>state.targetTest),'tcp');
 await p.evaluate(()=>{setTheme('light');});await p.locator('#navigation .nav-item',{hasText:'Test a target'}).click();
 assert.equal(await p.evaluate(()=>state.activePage),'tcp');
 await p.screenshot({path:path.join(out,'test-a-target-1920-light.png')});screenshots.push('test-a-target-1920-light.png');
 check('Last test and protocol choices persist after reopening');
 assert.deepEqual(errors,[]);await context.close();
 fs.writeFileSync(path.join(out,'ux-phase6-report.json'),JSON.stringify({passed:true,checks,screenshots,errors},null,2)+'\n');
 console.log('PASS '+checks.join(' | '));
})().catch(error=>{console.error(error);process.exitCode=1;}).finally(async()=>{if(browser)await browser.close();});
