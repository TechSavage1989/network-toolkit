'use strict';
// Real page/form lifecycle; SMTP jobs and settings use isolated API fixtures.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version;
const deferred=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve};};
(async()=>{
 const browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 try{
  for(const width of [390,1280]){
   const page=await browser.newPage({viewport:{width,height:900}}),errors=[],sent=[],saved=[],results=[];
   let mode='completed',gate=null;
   page.on('pageerror',error=>errors.push(error.message));
   await page.route('http://toolkit.local/**',async route=>{
    const request=route.request(),p=new URL(request.url()).pathname;
    if(p.startsWith('/api/')){
     let data={};
     if(p==='/api/status')data={version,os:'windows',arch:'amd64'};
     if(p==='/api/state'){
      if(request.method()==='GET')data={appUpdatePreferences:{enabled:false}};
      else saved.push(request.postDataJSON());
     }
     if(p==='/api/jobs'&&request.method()==='POST'){
      const body=request.postDataJSON();sent.push(body);
      assert.equal(body.tool,'smtp');assert.equal(body.options.sendTest,false);
      if(gate)await gate.promise;
      if(mode==='request-error')return route.fulfill({status:502,contentType:'application/json',body:JSON.stringify({error:'Fixture connection unavailable'})});
      const {password,...options}=body.options;
      data={id:'smtp-'+sent.length,tool:'smtp',target:body.target,options,status:mode,started:new Date().toISOString(),finished:new Date().toISOString(),events:[],result:{summary:mode==='failed'?'Fixture SMTP authentication failed.':'Fixture SMTP test completed.',data:{connected:mode!=='failed'}}};
      results.unshift(data);
     }else if(p==='/api/jobs')data=results;
     if(p.startsWith('/api/jobs/'))data=results.find(job=>job.id===p.split('/').at(-1))||{};
     if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
     return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
    }
    try{
     let body=fs.readFileSync(path.join(root,'web',p==='/'?'index.html':p));
     if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','smtp-fixture').replaceAll('__WINDOW_MODE__','main');
     return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':'application/octet-stream'});
    }catch{return route.fulfill({status:404,body:''});}
   });
   try{
    await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);
    await page.evaluate(()=>navigate('smtp'));await page.fill('#globalTarget','smtp.example.test');await page.fill('#opt-smtp-username','tester');await page.fill('#opt-smtp-password','smtp-fixture-secret');
    for(const outcome of ['completed','completed','failed','request-error']){
     mode=outcome;
     await page.evaluate(async()=>{revealToolConfiguration();await startJob();});
     assert.equal(await page.locator('#opt-smtp-password').inputValue(),'smtp-fixture-secret');
     assert.equal(sent.at(-1).options.password,'smtp-fixture-secret');
     await page.evaluate(()=>flushState());
     assert(!JSON.stringify(saved).includes('smtp-fixture-secret'));
     assert.equal(await page.evaluate(()=>JSON.stringify(state).includes('smtp-fixture-secret')),false);
     assert.equal(await page.locator('#opt-smtp-password').getAttribute('type'),'password');
    }
    await page.evaluate(()=>{navigate('ping');navigate('smtp');revealToolConfiguration();});assert.equal(await page.locator('#opt-smtp-password').inputValue(),'');
    // A response for the old form must not clear a password entered after returning.
    await page.fill('#opt-smtp-password','old-pending-secret');mode='completed';gate=deferred();
    const submitted=page.waitForRequest(request=>request.url().endsWith('/api/jobs')&&request.method()==='POST');
    await page.evaluate(()=>{window.smtpPendingTest=startJob();});await page.waitForFunction(()=>document.querySelector('#runButton').disabled);
    await submitted;
    await page.evaluate(()=>{navigate('overview');navigate('smtp');revealToolConfiguration();});assert.equal(await page.locator('#opt-smtp-password').inputValue(),'');
    await page.fill('#opt-smtp-password','new-form-secret');gate.resolve();await page.evaluate(()=>window.smtpPendingTest);
    assert.equal(await page.locator('#opt-smtp-password').inputValue(),'new-form-secret');
    await page.evaluate(()=>{navigate('overview');navigate('smtp');});assert.equal(await page.locator('#opt-smtp-password').inputValue(),'');
    assert.deepEqual(errors,[]);console.log('PASS SMTP password retention, retries, state exclusion and navigation at '+width+'px');
   }finally{gate?.resolve();await page.close();}
  }
 }finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
