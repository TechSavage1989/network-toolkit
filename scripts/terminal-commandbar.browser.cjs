'use strict';
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,output=process.env.TOOLKIT_TEST_OUTPUT;
(async()=>{
 const browser=await chromium.launch({headless:true,args:['--no-sandbox']}),page=await browser.newPage({viewport:{width:1280,height:900}}),errors=[],writes=[];
 page.on('pageerror',error=>errors.push(error.message));
 await page.route('http://toolkit.local/**',async route=>{
  const request=route.request(),p=new URL(request.url()).pathname;
  if(p.startsWith('/api/')){
   if(request.method()!=='GET')writes.push(p);let data={};
   if(p==='/api/status')data={version,os:'windows',arch:'amd64'};
   if(p==='/api/state'&&request.method()==='GET')data={appUpdatePreferences:{enabled:false}};
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminals')data={sessions:[]};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{
   const file=path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));
   if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','commandbar-fixture').replaceAll('__WINDOW_MODE__','main');
   return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.ico')?'image/x-icon':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 const input=page.locator('#terminalMacroCommand'),composer=page.locator('#terminalMacroComposer');
 async function metrics(){return input.evaluate(el=>{const s=getComputedStyle(el),bar=getComputedStyle(el,'::-webkit-scrollbar');return {horizontalGutter:el.offsetHeight-el.clientHeight-parseFloat(s.borderTopWidth)-parseFloat(s.borderBottomWidth),verticalGutter:el.offsetWidth-el.clientWidth-parseFloat(s.borderLeftWidth)-parseFloat(s.borderRightWidth),scrollWidth:el.scrollWidth,clientWidth:el.clientWidth,scrollHeight:el.scrollHeight,clientHeight:el.clientHeight,scrollbarWidth:s.scrollbarWidth,webkitDisplay:bar.display,overflowX:s.overflowX,overflowY:s.overflowY};});}
 async function noTracks(label){const m=await metrics();assert(m.horizontalGutter<=1&&m.verticalGutter<=1,label+': '+JSON.stringify(m));assert.equal(m.scrollbarWidth,'none');assert.equal(m.webkitDisplay,'none');assert.equal(m.overflowX,'auto');assert.equal(m.overflowY,'auto');return m;}
 async function collapsed(){if(await composer.evaluate(el=>el.classList.contains('is-expanded')))await page.locator('#terminalMacroExpand').click();}
 try{
  if(output)fs.mkdirSync(output,{recursive:true});
  await page.goto('http://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('terminal'));
  // Restore the released scrolling policy and classic tracks to reproduce the crop.
  await input.fill('show '+ 'x'.repeat(1000));
  const old=await page.addStyleTag({content:'#terminalMacroCommand{scrollbar-width:auto!important}#terminalMacroCommand::-webkit-scrollbar{display:block!important;width:15px!important;height:15px!important}'}),negative=await metrics();
  assert(negative.horizontalGutter>=14&&negative.verticalGutter>=14,'Released track policy did not reproduce both gutters: '+JSON.stringify(negative));
  if(output)await composer.screenshot({path:path.join(output,'terminal-commandbar-released-tracks.png')});await old.evaluate(el=>el.remove());await noTracks('restored candidate');
  const layouts=[];
  for(const width of [1280,1024,768])for(const theme of ['light','dark'])for(const side of ['left','right']){
   await page.setViewportSize({width,height:900});await page.evaluate(({theme,side})=>{setTheme(theme);state.terminalSidebarSide=side;NetworkToolkitSessionSidebar.render();NetworkToolkitTerminalMacros.sync();},{theme,side});
   await collapsed();
   for(const value of ['', 'show version','show '+ 'x'.repeat(1000)]){
    await input.fill(value);await noTracks(width+'/'+theme+'/'+side+'/compact');assert.equal(await input.inputValue(),value);
   }
   await input.press('Control+Home');await input.press('Control+End');assert(await input.evaluate(el=>el.scrollLeft>0),'Long commands must remain reachable by caret');
   if(output&&width===1280&&theme==='light'&&side==='right'){await input.fill('');await composer.screenshot({path:path.join(output,'terminal-commandbar-compact.png')});}
   const multiline=Array.from({length:40},(_,i)=>'show interface ethernet '+(i+1)).join('\n');await input.fill(multiline);
   assert(await composer.evaluate(el=>el.classList.contains('is-expanded')));const expanded=await noTracks(width+'/'+theme+'/'+side+'/expanded');assert(expanded.scrollHeight>expanded.clientHeight);
   await input.press('Control+Home');await input.press('Control+End');assert(await input.evaluate(el=>el.scrollTop>0),'Multiline commands must remain reachable by caret');
   await input.evaluate(el=>el.scrollTop=0);await input.hover();await page.mouse.wheel(0,150);await page.waitForFunction(()=>document.querySelector('#terminalMacroCommand').scrollTop>0);
   assert.equal(await input.inputValue(),multiline);await noTracks('after wheel');
   if(output&&width===1280&&theme==='light'&&side==='right')await composer.screenshot({path:path.join(output,'terminal-commandbar-expanded.png')});
   await page.locator('[data-macro-action="hide-composer"]').click();await page.locator('#terminalMacroComposerToggle').click();assert.equal(await input.inputValue(),multiline);await noTracks('after reopening');
   layouts.push({width,theme,side});
  }
  assert.equal(writes.filter(p=>p==='/api/terminals'||p.includes('/terminal-macros/send')).length,0);assert.deepEqual(errors,[]);
  const result={passed:true,renderedCases:layouts.length,classicScrollbarRegression:negative,noVisibleTracks:true,longCommandCaret:true,multilineCaretAndWheel:true,inputPreserved:true,noCommandsSent:true,errors};
  if(output)fs.writeFileSync(path.join(output,'terminal-commandbar-results.json'),JSON.stringify(result,null,2));console.log(JSON.stringify(result));
 }catch(error){if(output)await page.screenshot({path:path.join(output,'terminal-commandbar-failure.png')}).catch(()=>{});throw error;}
 finally{await browser.close();}
})().catch(error=>{console.error(error);process.exitCode=1;});
