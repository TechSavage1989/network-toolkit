'use strict';
// Production browser navigation with synthetic API responses. Windows native
// launcher-spy tests cover the actual privilege boundary; this never captures.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT,origin='https://capture107.toolkit.test';
const report={passed:false,checks:[],errors:[],screenshots:[],scope:'Synthetic production browser navigation/start/cancel; no live capture or UAC. Native Windows launcher-spy tests are separate.'};
const id='a'.repeat(24),adapter='adapter:{00000000-0000-0000-0000-000000000001}',cancelled='The Windows administrator prompt was cancelled. No capture was started.';
let browser,page,starts=0,lists=0,releaseStart,captures=[],adapterAvailable=true;
const pass=text=>report.checks.push(text);
async function waitForStart(){const deadline=Date.now()+15000;while(!releaseStart){if(Date.now()>deadline)throw Error('Explicit Start request did not arrive');await new Promise(resolve=>setTimeout(resolve,10));}}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});page=await browser.newPage({viewport:{width:1440,height:1000},serviceWorkers:'block'});page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));
 await page.route('**/*',async route=>{const req=route.request(),u=new URL(req.url()),p=u.pathname,method=req.method();assert.equal(u.origin,origin,'No external requests');
  if(p.startsWith('/api/')){let data={};if(method!=='GET')assert(['/api/state','/api/packet-captures/interfaces','/api/packet-captures'].includes(p),'Unexpected mutation '+p);
   if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',system:{hostname:'Synthetic capture fixture',interfaces:[]}};
   if(p==='/api/system')data={hostname:'Synthetic capture fixture',interfaces:[]};if(p==='/api/state'&&method==='GET')data={appUpdatePreferences:{enabled:false}};
   if(p==='/api/jobs')data=[];if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};if(p==='/api/terminal-views')data={available:false,owners:{},views:[]};
   if(p==='/api/credentials')data={vault:{available:false,state:'unavailable'},entries:[]};if(p==='/api/credential-vault')data={available:false,state:'unavailable'};
   if(p==='/api/packet-captures/capabilities')data={available:true,live:true,file:true,elevated:false,note:'Opening and refreshing do not request administrator approval. Start capture requests approval.',interfaces:[]};
   if(p==='/api/packet-captures/interfaces'){lists++;data={interfaces:adapterAvailable?[{id:adapter,name:'Synthetic Ethernet',engine:'auto'}]:[{id:'nics',name:'All adapters',engine:'auto'}]};}
   if(p==='/api/packet-captures'){if(method==='POST'){starts++;assert.equal(req.postDataJSON().interface,adapter);await new Promise(resolve=>releaseStart=resolve);captures=[{id,name:'Cancelled capture',state:'failed',engine:'auto',count:0,bytes:0,revision:1,lost:0,overwritten:0,error:cancelled}];data=captures[0];}else data={captures};}
   if(p.endsWith('/troubleshoot'))data={packetCount:0,duration:0,summary:'No packets captured.',scope:'Synthetic cancelled capture',metrics:{},findings:[],limitations:[]};
   if(p.includes('/packets'))data={capture:captures[0],total:0,offset:0,packets:[]};if(p.endsWith('/events'))data={events:[],sessions:[],after:0};
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{const file=path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','capture107-fixture').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto(origin);await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);
 for(let i=0;i<3;i++){await page.evaluate(()=>navigate('capture'));await page.waitForFunction(()=>!NetworkToolkitPacketCapture.model.interfacesLoading&&document.getElementById('packetInterface')?.value.startsWith('adapter:'));await page.locator('[data-packet-action="interfaces"]').click();await page.waitForFunction(()=>!NetworkToolkitPacketCapture.model.interfacesLoading);await page.evaluate(()=>navigate('overview'));}
 assert.equal(starts,0);assert(lists>=6);pass('Repeated sidebar opening and ordinary adapter refresh never request capture');
 await page.evaluate(()=>navigate('capture'));await page.waitForFunction(()=>!NetworkToolkitPacketCapture.model.interfacesLoading);
 adapterAvailable=false;await page.locator('[data-packet-action="interfaces"]').click();await page.waitForFunction(()=>!NetworkToolkitPacketCapture.model.interfacesLoading&&NetworkToolkitPacketCapture.model.interfaceUnavailable);assert.equal(await page.locator('#packetInterface').inputValue(),adapter);assert.equal(await page.locator('[data-packet-action="start"]').isDisabled(),true);assert.equal(starts,0);adapterAvailable=true;await page.locator('[data-packet-action="interfaces"]').click();await page.waitForFunction(()=>!NetworkToolkitPacketCapture.model.interfacesLoading&&!NetworkToolkitPacketCapture.model.interfaceUnavailable);pass('A removed selected adapter never silently broadens to all adapters; explicit refresh recovers it');
 for(const engine of ['live','file','auto']){await page.locator('#packetAdvancedCapture').evaluate(n=>n.open=true);await page.locator('#packetEngine').selectOption(engine);assert.equal(await page.locator('#packetInterface').inputValue(),adapter);}
 await page.evaluate(()=>{const button=document.querySelector('[data-packet-action="start"]');button.click();button.click();});await page.waitForFunction(()=>NetworkToolkitPacketCapture.model.startPending);await waitForStart();assert.equal(starts,1);assert.equal(await page.locator('[data-packet-action="start"]').isDisabled(),true);releaseStart();await page.waitForFunction(()=>!NetworkToolkitPacketCapture.model.startPending&&document.getElementById('packetCaptureStatus').textContent.includes('cancelled'));
 for(let i=0;i<3;i++)await page.evaluate(()=>NetworkToolkitPacketCapture.refresh(true));await page.evaluate(()=>navigate('overview'));await page.evaluate(()=>navigate('capture'));await page.waitForFunction(()=>document.getElementById('packetCaptureStatus')?.textContent.includes('No capture was started'));assert.equal(starts,1);assert.equal(await page.locator('[data-packet-action="start"]').isDisabled(),false);pass('One explicit Start is deduplicated; cancellation remains visible and navigation/polling never retry');
 for(const width of [1440,390]){await page.setViewportSize({width,height:1000});if(out){const name='packet-capture-privilege-'+width+'.png';await page.screenshot({path:path.join(out,name),fullPage:true});report.screenshots.push(name);}}
 releaseStart=null;await page.locator('#packetCaptureSetup').evaluate(n=>n.open=true);await page.locator('[data-packet-action="start"]').click();await waitForStart();assert.equal(starts,2);releaseStart();await page.waitForFunction(()=>!NetworkToolkitPacketCapture.model.startPending);pass('Only a fresh user Start requests another attempt');
 assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.errors.push(error.stack||String(error));console.error(error);process.exitCode=1;}).finally(async()=>{if(out){fs.writeFileSync(path.join(out,'packet-capture-privilege-report.json'),JSON.stringify(report,null,2)+'\n');if(page&&!report.passed)await page.screenshot({path:path.join(out,'packet-capture-privilege-failure.png'),fullPage:true}).catch(()=>{});}await browser?.close();});
