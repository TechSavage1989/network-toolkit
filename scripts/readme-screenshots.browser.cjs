'use strict';
// README screenshot generator. Synthetic data only: no desktop, device or production network calls.
// Run: node scripts/readme-screenshots.browser.cjs  (writes docs/images/*.png)
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||path.join(root,'docs','images');
let browser,stored={appUpdatePreferences:{enabled:false},profiles:[{id:'kept',name:'Existing target',target:'192.0.2.10',note:'keep',tool:'dns'}],ftpProfiles:[{id:'ftp-kept',name:'Kept FTP',host:'192.0.2.20',port:21,username:'operator',security:'explicit'}],fileTransferProtocol:'ftp',firstLaunchWelcomeSeen:true,target:'example.com'};
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
   if(name==='/api/status')data={version:require(path.join(root,'package.json')).version,os:'windows',arch:'amd64',system:{hostname:'EXAMPLE-PC',interfaces:[]}};
   if(name==='/api/state'){if(method==='PUT'){stored=request.postDataJSON();}else data=stored;}
   if(name==='/api/system')data={hostname:'EXAMPLE-PC',interfaces:[],gateways:[],dnsServers:[]};
   if(name==='/api/jobs'||name==='/api/jobs?brief=1')data=syntheticJobs;if(name.startsWith('/api/jobs/'))data=syntheticJobs.find(job=>name==='/api/jobs/'+job.id)||{};
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


const stamp=n=>new Date(Date.parse('2026-11-02T09:00:00Z')+n*1000).toISOString();
const samples=Array.from({length:60},(_,i)=>({sequence:i+1,time:stamp(i),address:'192.0.2.10',success:i!==41,status:i===41?'Timed out':'Reply',rttMs:i===41?null:Math.round((11+4*Math.sin(i/5)+(i%7)*0.6)*10)/10}));
const ok=samples.filter(s=>s.success).map(s=>s.rttMs);
const ping={id:'readme-ping',tool:'ping',target:'example.com',status:'completed',started:stamp(0),finished:stamp(59),eventSequence:1,options:{count:60,intervalMs:1000},events:[],result:{summary:'59 of 60 replies, 1.7% loss',data:{samples,statistics:{sent:60,received:59,lost:1,lossPercent:Math.round(1/60*1000)/10,minMs:Math.min(...ok),maxMs:Math.max(...ok),avgMs:Math.round(ok.reduce((a,b)=>a+b,0)/ok.length*10)/10}}}};
(async()=>{
 fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 let {p,context}=await make({width:1440,height:900});
 syntheticJobs=[ping,{id:'readme-dns',tool:'dns',target:'example.com',status:'completed',started:stamp(70),finished:stamp(71),eventSequence:1}];
 await p.evaluate(async()=>{await pollJobs();});
 await p.evaluate(()=>{setTheme('dark');navigate('ping');selected.ping='readme-ping';lastRender='';});
 await p.evaluate(job=>{clearTimeout(nextPoll);polling=true;jobs=[job,...jobs.filter(j=>j.id!==job.id)];selected.ping=job.id;lastRender='';renderPage();renderCurrentResult(true);if(typeof setToolPanel==='function')setToolPanel('results');},ping);
 await p.waitForTimeout(600);
 await p.screenshot({path:path.join(out,'test-a-target-dark.png')});
 await p.evaluate(()=>{setTheme('light');state.fileTransferProtocol='sftp';navigate('sftp');});await p.waitForTimeout(400);
 await p.screenshot({path:path.join(out,'file-transfer-light.png')});
 assert.deepEqual(errors,[]);await context.close();console.log('Wrote '+out);
})().catch(error=>{console.error(error);process.exitCode=1;}).finally(async()=>{if(browser)await browser.close();});
