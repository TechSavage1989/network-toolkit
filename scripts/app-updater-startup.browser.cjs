'use strict';
// Full-page browser regression. Run with PLAYWRIGHT_MODULE / CHROMIUM_PATH overrides
// when Playwright or Chromium are installed outside the usual local preview paths.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
let playwright;try{playwright=require(process.env.PLAYWRIGHT_MODULE||'playwright')}catch{playwright=require('/opt/codex/cua_node/lib/node_modules/playwright')}
const root=path.resolve(__dirname,'..'),available={currentVersion:'1.17.61',supported:true,hasUpdate:true,busy:[],installing:false,latest:{version:'1.17.62',notes:"What's new\n- Faster startup checks.\n- Clear update notes.\n- Settings are preserved.\n- Update when you choose.\n- More details.\n<script>window.notesExecuted=true</script>"}};
const {waitForAutomaticCompletion}=require('./browser-updater-readiness.cjs');
const output=process.env.TOOLKIT_TEST_OUTPUT||'/tmp/ntk-updater-browser';
fs.mkdirSync(output,{recursive:true});
const pause=()=>{let resolve;const promise=new Promise(r=>resolve=r);return {promise,resolve}};
(async()=>{
  const browser=await playwright.chromium.launch({headless:true,...(process.env.CHROMIUM_PATH?{executablePath:process.env.CHROMIUM_PATH}:fs.existsSync('/usr/bin/chromium')?{executablePath:'/usr/bin/chromium'}:{}),args:['--no-sandbox']});
  try{
    async function scenario(name,options={},verify){
      const page=await browser.newPage(),errors=[],requests=[],installBodies=[];let checks=0,latest=options.staged?{...available,staged:{version:'1.17.62',sha256:'verified'}}:{...available,hasUpdate:false,latest:null};
      const stateGate=options.holdState?pause():null,unrelatedGate=options.holdUnrelated?pause():null;
      const report={name,passed:false,requestTrace:[],barriers:[]};let pendingUpdateRequests=0,activeChecks=0,maxActiveChecks=0,statusReads=0;
      const readUpdater=async()=>({...await page.evaluate(()=>{
        const template=document.createElement('template');template.innerHTML=window.NetworkToolkitUpdater?.html?.()||'';
        return {virtualNow:Date.now(),hidden:document.hidden,online:navigator.onLine,schedule:template.content.querySelector('[data-app-update-schedule]')?.textContent||'',checkDisabled:template.content.querySelector('[data-app-update="check"]')?.disabled??null};
      }),checks,pendingUpdateRequests,activeChecks,maxActiveChecks});
      page.on('pageerror',error=>errors.push(error.message));
      await page.clock.install();
      await page.route('http://toolkit.local/**',async route=>{
        const url=new URL(route.request().url()),p=url.pathname;
        if(p.startsWith('/api/')){
          requests.push(p);let data={};
          const tracked=p.startsWith('/api/updates/'),check=p==='/api/updates/check';
          const trace=tracked?{path:p,method:route.request().method(),started:Date.now()}:null;
          if(tracked){report.requestTrace.push(trace);pendingUpdateRequests++;}
          if(check){activeChecks++;maxActiveChecks=Math.max(maxActiveChecks,activeChecks);}
          try{
          if(p==='/api/status')data={version:'1.17.61',os:'windows',arch:'amd64'};
          if(p==='/api/state'){if(route.request().method()==='GET'){if(stateGate)await stateGate.promise;data={appUpdatePreferences:options.preferences||{}};}}
          if(p==='/api/system'||p==='/api/jobs'){if(unrelatedGate)await unrelatedGate.promise;data=p==='/api/jobs'?[]:{}}
          if(p==='/api/jobs'&&url.search)data=[];
          if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
          if(p==='/api/updates/status'){
            // Deliberately exceed the old50ms settling assumption for the focus
            // check. Only the local synthetic response is delayed.
            if(++statusReads===2&&options.delayFocusStatus)await new Promise(resolve=>setTimeout(resolve,350));
            data=latest;
          }
          if(p==='/api/updates/check'){
            checks++;
            if(options.rateAfterFirst&&checks===2){
              latest={...latest,checkAfter:await page.evaluate(()=>Date.now()+600000)};
              return await route.fulfill({status:429,contentType:'application/json',body:JSON.stringify({error:'GitHub rate limit; wait before checking again',checkAfter:latest.checkAfter})});
            }
            if(options.failFirst&&checks===1)return await route.fulfill({status:502,contentType:'application/json',body:JSON.stringify({error:'Temporary network interruption'})});
            latest=options.staleFirst&&checks===1?{...available,hasUpdate:false,latest:{version:'1.17.61'}}:options.staged?{...available,staged:{version:'1.17.62',sha256:'verified'}}:options.release||available;data=latest;
          }
          if(p==='/api/updates/download'){assert.equal(options.installFlow,true,'Unapproved download');assert.equal(route.request().postDataJSON().version,'1.17.62');latest={...available,staged:{version:'1.17.62',sha256:'verified'}};data=latest;}
          if(p==='/api/updates/install'){assert.equal(options.installFlow,true,'Unapproved install');installBodies.push(route.request().postDataJSON());data={restarting:true};}
          return await route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
          }finally{if(tracked){pendingUpdateRequests--;trace.completed=Date.now();}if(check)activeChecks--;}
        }
        try{
          let body=fs.readFileSync(p==='/'?path.join(root,'web/index.html'):path.join(root,'web',p));
          if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','browser-test').replaceAll('__WINDOW_MODE__','main');
          return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':'application/octet-stream'});
        }catch{return route.fulfill({status:404,body:''})}
      });
      try{
        await page.goto('http://toolkit.local/');
        // Let initial fetches settle before advancing the actual app's timers.
        await page.waitForFunction(()=>typeof NetworkToolkitUpdater==='object'&&online);
        await page.waitForTimeout(50);
        const h={page,requests,installBodies,get checks(){return checks},stateGate,unrelatedGate,
          advance:async ms=>{await page.clock.runFor(ms);await page.waitForTimeout(50)},
          completed:async expected=>{const state=await waitForAutomaticCompletion(readUpdater,expected);report.barriers.push(state);return state;},
          settings:async()=>{await page.evaluate(()=>appSettings());await page.waitForTimeout(50)},
          notice:()=>page.locator('#appUpdateNotice').innerText()};
        await verify(h);
        assert.deepEqual(errors,[]);if(!options.installFlow)assert.equal(requests.some(p=>p==='/api/updates/download'||p==='/api/updates/install'),false);
        assert.ok(maxActiveChecks<=1,'Overlapping release-feed checks');report.passed=true;
        console.log('PASS '+name+' ('+checks+' release checks)');
      }catch(error){report.failure=error.stack||String(error);throw error;}finally{
        report.errors=errors;report.checks=checks;report.maxActiveChecks=maxActiveChecks;
        report.final=await readUpdater().catch(error=>({unavailable:error.message}));
        const file='startup-'+name.replace(/[^a-z0-9]+/gi,'-').toLowerCase();
        if(!report.passed)await page.screenshot({path:path.join(output,file+'-failure.png')}).catch(()=>{});
        fs.writeFileSync(path.join(output,file+'.json'),JSON.stringify(report,null,2));
        stateGate?.resolve();unrelatedGate?.resolve();await page.close();
      }
    }
    await scenario('startup independent of slow system/jobs; dialog and navigation preserved',{holdUnrelated:true},async h=>{
      await h.page.evaluate(()=>openModal('Keep my work','<input id="testDraft" value="not lost">'));
      await h.advance(100);assert.equal(h.checks,1);assert.match(await h.notice(),/1.17.62 is available/);
      assert.equal(await h.page.locator('#testDraft').inputValue(),'not lost');
      assert.equal(await h.page.evaluate(()=>window.__networkToolkitStartupState),0);
      await h.page.evaluate(()=>{closeModal();navigate('subnet')});assert.match(await h.notice(),/1.17.62 is available/);
      await h.settings();assert.match(await h.page.locator('[data-app-update-schedule]').innerText(),/Last automatic attempt.*Update available: 1.17.62.*Next automatic check/s);
      await h.page.evaluate(()=>closeModal());assert.match(await h.notice(),/1.17.62 is available/);
    });
    await scenario('delayed preferences honor saved opt-out',{holdState:true,preferences:{enabled:false}},async h=>{
      await h.advance(6000);assert.equal(h.checks,0);await h.settings();assert.equal(await h.page.locator('[data-app-update-auto]').isDisabled(),true);
      h.stateGate.resolve();await h.page.waitForFunction(()=>window.__networkToolkitStartupState===1);await h.advance(5100);assert.equal(h.checks,0);
      await h.settings();assert.match(await h.page.locator('[data-app-update-schedule]').innerText(),/turned off/);
      await h.page.locator('[data-app-update="check"]').click();await h.page.waitForFunction(()=>document.querySelector('#appUpdateNotice').textContent.includes('1.17.62'));assert.equal(h.checks,1);
    });
    await scenario('transient startup failure retries quietly',{failFirst:true},async h=>{
      await h.advance(100);assert.equal(h.checks,1);assert.equal(await h.notice(),'');await h.settings();
      assert.match(await h.page.locator('[data-app-update-schedule]').innerText(),/Temporary network interruption.*Next automatic retry/s);
      await h.advance(30100);assert.equal(h.checks,2);assert.match(await h.notice(),/1.17.62 is available/);
    });
    await scenario('manual check replaces next automatic timer without duplicates',{preferences:{dismissed:'download:1.17.62',until:Date.now()+86400000}},async h=>{
      await h.advance(100);const previous=h.checks;await h.settings();await h.page.locator('[data-app-update="check"]').click();await h.page.waitForFunction(()=>document.querySelector('#appUpdateNotice').textContent.includes('1.17.62'));
      await h.advance(6000);assert.equal(h.checks,previous+1);assert.match(await h.notice(),/1.17.62 is available/);
    });
    await scenario('five-minute checks and focus freshness preserve opt-in and no automatic install',{delayFocusStatus:true},async h=>{
      await h.advance(100);await h.completed(1);assert.equal(h.checks,1);
      await h.page.evaluate(()=>{window.dispatchEvent(new Event('focus'));window.dispatchEvent(new Event('online'));});
      await h.advance(100);await h.completed(1);assert.equal(h.checks,1);
      // Never fast-forward timeout timers across an unfinished fetch.
      // The completion barrier observes the actual rendered scheduler state,
      // not just a POST received by the route or a fixed settling delay.
      await h.advance(120000);
      await h.page.evaluate(()=>{window.dispatchEvent(new Event('focus'));document.dispatchEvent(new Event('visibilitychange'));window.dispatchEvent(new Event('online'));});
      await h.advance(100);await h.completed(2);assert.equal(h.checks,2);
      await h.advance(300100);await h.completed(3);assert.equal(h.checks,3);
      await h.settings();await h.page.locator('[data-app-update-auto]').uncheck();
      await h.advance(100);
      await h.page.evaluate(()=>window.dispatchEvent(new Event('focus')));
      await h.advance(300100);assert.equal(h.checks,3);
    });
    await scenario('server cooldown survives focus and manual checks without losing known release',{rateAfterFirst:true},async h=>{
      await h.advance(100);assert.equal(h.checks,1);await h.settings();
      await h.page.locator('[data-app-update="check"]').click();await h.page.waitForFunction(()=>document.querySelector('#appUpdateSettings').textContent.includes('GitHub rate limit'));
      assert.equal(h.checks,2);assert.match(await h.notice(),/1.17.62 is available/);
      await h.advance(120100);
      await h.page.evaluate(()=>{window.dispatchEvent(new Event('focus'));window.dispatchEvent(new Event('online'));});
      await h.page.locator('[data-app-update="check"]').click();await h.advance(100);assert.equal(h.checks,2);
      await h.advance(480100);assert.equal(h.checks,3);
    });
    await scenario('current-version release notes show one badge with no duplicate disclosure',{release:{...available,currentVersion:'1.17.62',hasUpdate:false,latest:{version:'1.17.62',notes:"Network Toolkit 1.17.62\n\nWhat's new\n- One\n- Two\n- Three\n- Four"}}},async h=>{
      await h.advance(100);await h.settings();
      const card=h.page.locator('.nt-update-notes');
      assert.equal(await card.locator('.nt-update-notes-versions').innerText(),'1.17.62');
      assert.equal(await card.locator('details').count(),0);assert.equal(await card.locator('li').count(),4);
      const out=process.env.TOOLKIT_TEST_OUTPUT||'/tmp/ntk-updater-browser';fs.mkdirSync(out,{recursive:true});
      for(const width of [390,768,1280]){
        await h.page.setViewportSize({width,height:900});await card.scrollIntoViewIfNeeded();
        assert.equal(await card.evaluate(el=>el.scrollWidth<=el.clientWidth+1),true);
        await h.page.screenshot({path:path.join(out,'update-current-notes-'+width+'.png')});
      }
    });
    await scenario('staged banner and settings expose one final install action',{staged:true,installFlow:true},async h=>{
      await h.advance(100);
      for(const width of [390,768,1280]){
        await h.page.setViewportSize({width,height:900});
        const banner=h.page.locator('#appUpdateNotice'),text=await banner.innerText();
        assert.match(text,/Save your work before restarting\./);
        assert.match(text,/1.17.62/);assert.doesNotMatch(text,/Review install/);
        assert.equal(await banner.getByRole('button',{name:'Install and restart',exact:true}).getAttribute('data-app-update'),'confirm');
        assert.equal(await banner.evaluate(el=>el.scrollWidth<=el.clientWidth+1),true);
        assert.equal(h.installBodies.length,0);
        const out=process.env.TOOLKIT_TEST_OUTPUT||'/tmp/ntk-updater-browser';fs.mkdirSync(out,{recursive:true});
        await h.page.screenshot({path:path.join(out,'update-ready-'+width+'.png')});
      }
      await h.settings();const settings=h.page.locator('#appUpdateSettings');assert.match(await settings.innerText(),/Save your work before restarting/);assert.equal(await settings.locator('[data-app-update="review"]').count(),0);await settings.getByRole('button',{name:'Install and restart',exact:true}).click();await h.page.waitForFunction(()=>document.querySelector('#appUpdateNotice').textContent==='');assert.deepEqual(h.installBodies,[{confirm:true,version:'1.17.62'}]);
    });
    for(const width of [390,768,1280])for(const surface of ['modal','banner'])await scenario('download goes directly to final '+surface+' install action at '+width,{installFlow:true},async h=>{
      await h.page.setViewportSize({width,height:900});await h.advance(100);await h.page.locator('#appUpdateNotice [data-app-update="download"]').click();assert.equal(h.requests.includes('/api/updates/download'),false);await h.page.locator('#modalRoot [data-app-update="confirm-download"]').click();const modal=h.page.locator('#modalRoot');await modal.locator('[data-app-update="confirm"]').waitFor({state:'visible'});assert.equal(h.requests.filter(p=>p==='/api/updates/download').length,1);assert.equal(h.installBodies.length,0);assert.match(await modal.innerText(),/Save your work before restarting/);assert.match(await modal.innerText(),/Replace the current app with version 1.17.62 and restart\?/);assert.equal(await h.page.locator('[data-app-update="review"]').count(),0);assert.equal(await modal.evaluate(el=>el.scrollWidth<=el.clientWidth+1),true);
      const out=process.env.TOOLKIT_TEST_OUTPUT||'/tmp/ntk-updater-browser';fs.mkdirSync(out,{recursive:true});await h.page.screenshot({path:path.join(out,'update-install-'+surface+'-'+width+'.png')});
      if(surface==='banner'){await modal.getByRole('button',{name:'Cancel',exact:true}).click();assert.equal(h.installBodies.length,0);await h.page.locator('#appUpdateNotice').getByRole('button',{name:'Install and restart',exact:true}).click();assert.equal(await modal.innerText(),'');}else await modal.getByRole('button',{name:'Install and restart',exact:true}).click();
      await h.page.waitForFunction(()=>document.querySelector('#appUpdateNotice').textContent==='');assert.deepEqual(h.installBodies,[{confirm:true,version:'1.17.62'}]);
    });
    await scenario('notes are safe and readable before any download',{},async h=>{
      await h.advance(100);assert.equal(h.checks,1);
      for(const width of [390,768,1280]){
        await h.page.setViewportSize({width,height:900});
        await h.page.locator('#appUpdateNotice [data-app-update="download"]').click();
        const card=h.page.locator('.nt-update-notes');await card.waitFor();
        assert.match(await card.innerText(),/What's new/);
        assert.equal(await h.page.evaluate(()=>window.notesExecuted===true),false);
        assert.equal(await card.locator('a,script,img,iframe').count(),0);
        assert.equal(await card.evaluate(el=>el.scrollWidth<=el.clientWidth+1),true);
        const summary=card.locator('summary');await summary.focus();await summary.press('Enter');
        assert.equal(await card.locator('details').getAttribute('open'),'');
        assert.doesNotMatch(await card.locator('.nt-update-notes-full').innerText(),/Faster startup checks|Clear update notes|Settings are preserved|Update when you choose/);
        assert.match(await card.locator('.nt-update-notes-full').innerText(),/More details/);
        const out=process.env.TOOLKIT_TEST_OUTPUT||'/tmp/ntk-updater-browser';fs.mkdirSync(out,{recursive:true});
        await h.page.screenshot({path:path.join(out,'update-notes-'+width+'.png')});
        await h.page.getByRole('button',{name:'Not now',exact:true}).click();
        assert.match(await h.notice(),/1.17.62/);
      }
    });
  }finally{await browser.close()}
})().catch(error=>{console.error(error);process.exitCode=1});
