'use strict';
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
// Production application assets, intercepted synthetic settings, native Chromium
// focus/clipboard. No real settings, files, connections, capture or credentials.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT,origin='https://colors107.toolkit.test';
const customAppearance={scheme:'custom',fontFamily:'Consolas',colors:{background:'#122334',foreground:'#eeddcc',selectionBackground:'#33445566'},highlightColors:{address:'#bb88dd'},shells:{powershell:{enabled:false,colors:{command:'#998877'}},cmd:{enabled:true,colors:{}}}};
let stored={appUpdatePreferences:{enabled:false},theme:'dark',accentColor:'#8bdac0',terminalAppearance:customAppearance,colorPickerPreferences:{recent:['#aabbcc80'],saved:['#12345680']},diagnosticColors:{bad:'#ff8790',warn:'#f4c56b',good:'#5dd6a0',info:'#6eabff',latency:'#438ef5'}};
const report={passed:false,checks:[],errors:[],screenshots:[],failedImages:[],scope:'Synthetic production-asset browser acceptance. Real Chromium clipboard, focus and viewport checks; no native Windows or user settings acceptance.'},writes=[];
let browser,page;
const pass=text=>{report.checks.push(text);console.log('PASS '+text);};
const picker=()=>page.locator('[data-color-picker]'),hex=()=>page.locator('[data-color-value="hex"]');
async function shot(name){if(!out)return;const file='color-picker-'+name+'.png';await page.screenshot({path:path.join(out,file),fullPage:true});report.screenshots.push(file);}
async function flush(){await page.evaluate(async()=>{saveState();await flushState();await stateWrites;});}
async function openAccent(){await page.evaluate(()=>{appSettings();window.NetworkToolkitUnifiedSettings?.show('appearance')});await page.locator('#appAccentColor').click();await picker().waitFor();}
async function dismiss(){await page.keyboard.press('Escape');await picker().waitFor({state:'detached'});}
async function startup(){await page.goto(origin+'/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);await page.waitForFunction(()=>[...document.images].every(image=>image.complete&&image.naturalWidth>0));assert(await page.evaluate(()=>!!window.NetworkToolkitColorPicker),'Final app must load shared picker');}
async function noOverflow(){const bounds=await picker().evaluate(el=>{const r=el.getBoundingClientRect(),body=el.querySelector('.ntk-color-body');return {left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:innerWidth,height:innerHeight,overflow:body.scrollWidth>body.clientWidth+1};});assert(bounds.left>=0&&bounds.right<=bounds.width+1&&bounds.top>=0&&bounds.bottom<=bounds.height+1,JSON.stringify(bounds));assert.equal(bounds.overflow,false,'Dialog body must not overflow horizontally');}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true,args:['--no-sandbox']});page=await browser.newPage({viewport:{width:1440,height:1000},serviceWorkers:'block',permissions:['clipboard-read','clipboard-write']});page.setDefaultTimeout(15000);page.on('pageerror',error=>report.errors.push(error.message));
 page.on('response',response=>{if(response.status()>=400&&(response.request().resourceType()==='image'||/Network_Toolkit_Icon|favicon\.ico/.test(response.url())))report.failedImages.push({url:response.url(),status:response.status()});});
 page.on('requestfailed',request=>{if(request.resourceType()==='image')report.failedImages.push({url:request.url(),failure:request.failure()?.errorText});});
 await page.route('**/*',async route=>{if(await handleEditorActivityMetadata(route,origin,report))return;
  const req=route.request(),url=new URL(req.url()),p=url.pathname;assert.equal(url.origin,origin,'No external requests are allowed');
  if(p.startsWith('/api/')){const method=req.method();let data={};
   if(method!=='GET'){assert.equal(p,'/api/state','Color changes must not start connections, jobs, captures, uploads or credential operations');const body=req.postDataJSON();writes.push(JSON.parse(JSON.stringify(body)));stored=body;}
   if(p==='/api/state'&&method==='GET')data=stored;
   if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',storageWritable:true,system:{hostname:'Color fixture',interfaces:[]}};
   if(p==='/api/system')data={hostname:'Color fixture',interfaces:[]};
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};
   if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   if(p==='/api/credential-vault')data={available:false,state:'unavailable'};
   if(p==='/api/credentials')data={vault:{available:false,state:'unavailable'},entries:[]};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,80));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{const icon=/^\/assets\/Network_Toolkit_Icon\.(svg|ico)$/.test(p)||p==='/favicon.ico',file=icon?path.join(root,'assets',p==='/favicon.ico'?'Network_Toolkit_Icon.ico':path.basename(p)):path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','colors107-fixture').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.ico')?'image/x-icon':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await startup();const preserved=await page.evaluate(()=>JSON.parse(JSON.stringify(state.terminalAppearance)));
 await openAccent();assert.equal(await hex().inputValue(),'#8bdac0');assert.equal(await page.locator('[data-color-channel="a"]').count(),0);assert.equal(await page.locator('[data-color-swatch="#12345680"]').count(),0);
 await hex().fill('#bad');assert(await page.locator('[data-color-apply]').isDisabled());assert(await page.locator('[data-color-copy="hex"]').isDisabled());assert.match(await page.locator('[data-color-error]').textContent(),/Use #RRGGBB/);
 await page.locator('[data-color-value="rgb"]').fill('rgb(170, 187, 204)');assert.equal(await hex().inputValue(),'#aabbcc');assert.equal(await page.evaluate(()=>state.accentColor),'#8bdac0');assert.equal(await page.evaluate(()=>document.documentElement.style.getPropertyValue('--accent')),'#aabbcc');
 await page.locator('[data-color-save]').click();await flush();assert.equal(stored.accentColor,'#8bdac0','Unrelated autosave must not capture preview');assert.deepEqual(stored.colorPickerPreferences.saved,['#12345680']);
 await dismiss();assert.equal(await page.evaluate(()=>document.documentElement.style.getPropertyValue('--accent')),'#8bdac0');assert.equal(await page.evaluate(()=>document.activeElement.id),'appAccentColor');assert.equal(await page.locator('.modal').count(),1,'Escape leaves underlying settings modal open');
 pass('Strict validation, opaque capability, live preview without autosave leakage, staged palette and Escape rollback/focus restoration');

 await page.locator('#appAccentColor').click();await hex().fill('#224466');
 for(const [kind,expected]of [['hex','#224466'],['rgb','rgb(34, 68, 102)']]){await page.locator('[data-color-copy="'+kind+'"]').focus();await page.keyboard.press('Enter');await page.waitForFunction(async expected=>(await navigator.clipboard.readText())===expected,expected);assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),expected);}
 const beforeCopy=await page.evaluate(()=>navigator.clipboard.readText());
 await page.evaluate(()=>{window.fixtureColorWrite=navigator.clipboard.writeText;Object.defineProperty(navigator.clipboard,'writeText',{configurable:true,value:async()=>{throw Error('Synthetic clipboard denied');}});});
 await page.locator('[data-color-copy="hex"]').click();await page.waitForFunction(()=>document.querySelector('[data-color-status]')?.textContent.includes('Copy is unavailable'));assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),beforeCopy);assert(!await page.locator('[data-color-status]').textContent().then(text=>text.includes('Copied')));
 await page.evaluate(()=>{Object.defineProperty(navigator.clipboard,'writeText',{configurable:true,value:window.fixtureColorWrite});delete window.fixtureColorWrite;});
 const hue=page.locator('[data-color-channel="h"]');await hue.focus();const priorHue=Number(await hue.inputValue());await page.keyboard.press('ArrowRight');assert.equal(Number(await hue.inputValue()),priorHue+1);await hex().fill('#224466');
 for(const key of ['Tab','Shift+Tab'])for(let i=0;i<55;i++){await page.keyboard.press(key);const focus=await page.evaluate(()=>({contained:!!document.activeElement.closest('[data-color-picker]'),tag:document.activeElement.tagName,id:document.activeElement.id,text:document.activeElement.textContent.slice(0,80)}));assert(focus.contained,'Picker focus remains contained at '+key+' '+i+': '+JSON.stringify(focus));}
 assert(await page.locator('[data-color-apply]').evaluate(button=>{const style=getComputedStyle(button);return NetworkToolkitColorPicker.contrastRatio(style.backgroundColor,style.color)>=4.5;}),'Apply stays readable even while previewing a low-contrast accent');
 await page.locator('[data-color-save]').click();await shot('normal-dark');await noOverflow();await page.locator('[data-color-apply]').click();await picker().waitFor({state:'detached'});await flush();assert.equal(stored.accentColor,'#224466');assert(stored.colorPickerPreferences.saved.includes('#224466'));assert.equal(stored.colorPickerPreferences.recent[0],'#224466');
 pass('Native HEX/RGB clipboard success and denial, keyboard hue selection, native modal focus trap and applied recent/saved color');

 await startup();assert.equal(await page.evaluate(()=>state.accentColor),'#224466');assert.deepEqual(await page.evaluate(()=>JSON.parse(JSON.stringify(state.terminalAppearance))),preserved,'Custom terminal colors and explicit shell settings survive migration/reload');await openAccent();assert(await page.locator('[data-color-saved] [data-color-swatch="#224466"]').count());
 await page.locator('[data-color-reset]').click();assert.equal(await hex().inputValue(),'#6eabff');await dismiss();assert.equal(await page.evaluate(()=>state.accentColor),'#224466');
 await page.locator('#appAccentColor').click();await page.locator('[data-color-reset]').click();await page.locator('[data-color-apply]').click();assert.equal(await page.evaluate(()=>state.accentColor),'','Reset restores theme-following default, not a new explicit color');await flush();
 pass('Synthetic settings restart retains selected/saved/recent colors and intentional custom themes; reset stages then applies theme-following default');

 await page.evaluate(()=>openDiagnosticColorSettings());const originalBad=await page.evaluate(()=>state.diagnosticColors.bad);await page.locator('[data-diagnostic-color="bad"]').click();const warn=await page.evaluate(()=>state.diagnosticColors.warn);await hex().fill(warn);assert.match(await page.locator('[data-color-advice]').textContent(),/close to/);assert.equal(await page.evaluate(()=>state.diagnosticColors.bad),originalBad);await dismiss();assert.equal(await page.evaluate(()=>state.diagnosticColors.bad),originalBad);assert.equal(await page.evaluate(()=>document.getElementById('pageContent').style.getPropertyValue('--diag-bad')),originalBad);
 await page.locator('[data-diagnostic-color="bad"]').click();await hex().fill('#ff6677');await page.locator('[data-color-apply]').click();assert.equal(await page.evaluate(()=>state.diagnosticColors.bad),'#ff6677');assert.equal(await page.evaluate(()=>state.diagnosticColors.warn),warn);await flush();
 pass('Diagnostic semantic warning is explicit and non-destructive; Cancel restores and Apply changes only the chosen role');

 await page.evaluate(()=>{closeModal();window.fixtureColorAlpha=null;NetworkToolkitColorPicker.open({title:'Synthetic terminal selection background',value:'#33445566',defaultValue:'#33445566',allowAlpha:true,onApply:value=>{window.fixtureColorAlpha=value;}});});assert.equal(await page.locator('[data-color-channel="a"]').count(),1);assert(await page.locator('[data-color-swatch="#12345680"]').count());await hex().fill('rgba(16, 32, 48, 0.5)');assert.equal(await page.locator('[data-color-value="rgb"]').inputValue(),'rgba(16, 32, 48, 0.50196)');await page.locator('[data-color-apply]').click();assert.equal(await page.evaluate(()=>window.fixtureColorAlpha),'#10203080');
 pass('Alpha editor is opt-in and preserves exact supported selection alpha; opaque consumers do not silently coerce it');

 await page.evaluate(()=>{closeModal();navigate('terminal');});await page.locator('#terminalSessionToggle').click();await page.locator('#terminalPaletteDetails>summary').click();
 const terminalOriginal=await page.evaluate(()=>state.terminalAppearance.colors.foreground);await page.locator('#terminalColor-foreground').click();await hex().fill('#778899');assert(await page.locator('#terminalSessionPanel').isVisible(),'Picker input must not close terminal Display through click-away');await flush();assert.equal(stored.terminalAppearance.colors.foreground,terminalOriginal,'Terminal preview cannot leak into an unrelated autosave');await dismiss();assert(await page.locator('#terminalSessionPanel').isVisible());assert.equal(await page.evaluate(()=>document.activeElement.id),'terminalColor-foreground');
 await page.locator('#terminalColor-foreground').click();await hex().fill('#778899');await page.locator('[data-color-apply]').click();assert(await page.locator('#terminalSessionPanel').isVisible());assert.equal(await page.evaluate(()=>state.terminalAppearance.colors.foreground),'#778899');assert.equal(await page.evaluate(()=>document.activeElement.id),'terminalColor-foreground');
 await page.locator('#terminalColor-selectionBackground').click();assert.equal(await page.locator('[data-color-channel="a"]').count(),1);await page.locator('[data-color-channel="a"]').focus();await page.keyboard.press('ArrowLeft');await dismiss();assert.equal(await page.evaluate(()=>state.terminalAppearance.colors.selectionBackground),'#33445566');assert(await page.locator('#terminalSessionPanel').isVisible());
 pass('Real terminal Display consumes the same picker, survives pointer input/Apply/Cancel, restores opener focus and preserves unsaved preview/alpha');

 await page.setViewportSize({width:390,height:760});await page.evaluate(()=>{setTheme('light');appSettings();window.NetworkToolkitUnifiedSettings?.show('appearance');});await page.locator('#appAccentColor').click();await hex().fill('#1765c5');await page.locator('[data-color-save]').click();await shot('narrow-light');await noOverflow();assert.equal(await page.locator('[data-color-channel="a"]').count(),0);
 // A visible focus ring must not disappear when the chosen accent matches the panel.
 await page.locator('[data-color-copy="hex"]').focus();await page.keyboard.press('Tab');const focus=await page.evaluate(()=>{const s=getComputedStyle(document.activeElement);return {width:s.outlineWidth,style:s.outlineStyle};});assert.notEqual(focus.style,'none');assert.notEqual(focus.width,'0px');
 await dismiss();await page.locator('#appAccentColor').click();await hex().fill('#556677');await page.evaluate(()=>navigate('overview'));assert.equal(await picker().count(),0);assert.equal(await page.evaluate(()=>state.accentColor),'');assert.equal(await page.evaluate(()=>document.documentElement.style.getPropertyValue('--accent')),'');
 pass('Normal/narrow screenshots, no horizontal clipping, visible keyboard focus, and navigation cancels pending preview');

 assert(writes.length>0);assert.deepEqual(report.failedImages,[],'Canonical production icons must load in the captured fixture');assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.errors.push(error.stack||String(error));console.error(error.stack||String(error));process.exitCode=1;}).finally(async()=>{if(out){fs.writeFileSync(path.join(out,'color-picker-report.json'),JSON.stringify(report,null,2)+'\n');if(page&&!report.passed)await page.screenshot({path:path.join(out,'color-picker-failure.png'),fullPage:true}).catch(()=>{});}await browser?.close();});
