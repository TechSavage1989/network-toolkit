'use strict';
// Dropdowns that match the app (v1.21.10). Production assets with intercepted
// synthetic settings; no connections, files or credentials are used.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT,origin='https://dropdowns.toolkit.test';
let stored={appUpdatePreferences:{enabled:false},theme:'light',activePage:'terminal',terminalAppearance:{fontFamily:'Consolas'}};
const report={passed:false,checks:[],errors:[],screenshots:[],scope:'Synthetic production-asset browser acceptance in Chromium. Not native Windows WebView2 acceptance.'};
const pass=text=>{report.checks.push(text);console.log('PASS '+text);};
let browser,page;
async function shot(name){if(!out)return;const file='dropdowns-'+name+'.png';await page.screenshot({path:path.join(out,file)});report.screenshots.push(file);}
async function geometry(){return page.evaluate(()=>[...document.querySelectorAll('select')].filter(s=>s.getClientRects().length).map(s=>{const r=s.getBoundingClientRect();return [s.id||s.className||s.name,Math.round(r.width*10)/10,Math.round(r.height*10)/10];}));}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true,args:['--no-sandbox']});page=await browser.newPage({viewport:{width:1440,height:950},serviceWorkers:'block'});page.setDefaultTimeout(15000);
 page.on('pageerror',error=>report.errors.push(error.message));
 await page.route('**/*',async route=>{if(await handleEditorActivityMetadata(route,origin,report))return;
  const req=route.request(),url=new URL(req.url()),p=url.pathname;assert.equal(url.origin,origin,'No external requests are allowed');
  if(p.startsWith('/api/')){const method=req.method();let data={};
   if(method!=='GET'){assert.equal(p,'/api/state','Dropdowns must not start connections or jobs');stored=req.postDataJSON();}
   if(p==='/api/state'&&method==='GET')data=stored;
   if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',storageWritable:true,system:{hostname:'Dropdown fixture',interfaces:[]}};
   if(p==='/api/system')data={hostname:'Dropdown fixture',interfaces:[]};
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};
   if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   if(p==='/api/credential-vault')data={available:false,state:'unavailable'};
   if(p==='/api/credentials')data={vault:{available:false,state:'unavailable'},entries:[]};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,80));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{const icon=/^\/assets\/Network_Toolkit_Icon\.(svg|ico)$/.test(p)||p==='/favicon.ico',file=icon?path.join(root,'assets',p==='/favicon.ico'?'Network_Toolkit_Icon.ico':path.basename(p)):path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','dropdowns-fixture').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.ico')?'image/x-icon':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto(origin+'/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);
 assert.equal(await page.evaluate(()=>NetworkToolkitDropdowns.baseSelect),true,'this Chromium supports customizable selects');
 for(const theme of ['light','dark']){
  await page.evaluate(theme=>{setTheme(theme);navigate('terminal');},theme);
  if(await page.locator('#terminalSessionPanel.hidden').count())await page.locator('#terminalSessionToggle').click();
  const font=page.locator('#terminalAppearanceFont');await font.scrollIntoViewIfNeeded();
  // Suggestion list: every choice is listed whatever is typed, each font in its own face.
  assert.equal(await font.getAttribute('list'),null,'the browser suggestion popup is replaced');assert.equal(await font.getAttribute('role'),'combobox');
  // Typing never opens the list by itself (it would cover dialog buttons); Down opens it at the best match.
  await font.fill('Cour');assert.equal(await page.locator('#comboboxPopup').isHidden(),true,'typing alone does not open the list');
  await font.press('ArrowDown');await page.waitForFunction(()=>!document.querySelector('#comboboxPopup')?.hidden);
  const list=await page.evaluate(()=>{const options=[...document.querySelectorAll('#comboboxPopup [role=option]')];return {count:options.length,total:document.querySelectorAll('#terminalFontFamilies option').length,active:options.find(o=>o.classList.contains('active'))?.querySelector('.combobox-value').textContent,faces:options.map(o=>[o.querySelector('.combobox-value').textContent,o.querySelector('.combobox-value').style.fontFamily])};});
  assert.equal(list.count,list.total,'typing must not hide the other choices');assert.equal(list.active,'Courier New','typing moves to the best match');
  for(const [name,face] of list.faces)assert(face.includes(name)||face===name,'each font is shown in its own face: '+name+' → '+face);
  await shot('font-'+theme);
  await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter');
  assert.equal(await font.inputValue(),list.faces[list.faces.findIndex(([n])=>n==='Courier New')+1][0]);
  await font.click();assert.equal(await page.locator('#comboboxPopup').isHidden(),true,'clicking in the field only places the caret');assert.equal(await page.locator('#comboboxPopup').isHidden(),true);
  assert.equal(await page.evaluate(()=>state.terminalAppearance.fontFamily),await font.inputValue(),'choosing applies through the normal change handler');
  await page.locator('#terminalAppearanceEditor').locator('.combobox-toggle').first().click();assert.equal(await page.locator('#comboboxPopup').isVisible(),true,'the arrow opens the full list');
  // Choosing with the mouse applies the font and leaves the settings panel open.
  await page.locator('#comboboxPopup [role=option]').filter({hasText:'JetBrains Mono'}).click();
  assert.equal(await font.inputValue(),'JetBrains Mono');assert.equal(await page.evaluate(()=>state.terminalAppearance.fontFamily),'JetBrains Mono');
  assert.equal(await page.locator('#terminalSessionPanel').isVisible(),true,'clicking a choice must not close the panel');assert.equal(await page.locator('#comboboxPopup').isHidden(),true);
  await page.locator('#terminalAppearanceEditor').locator('.combobox-toggle').first().click();assert.equal(await page.locator('#comboboxPopup').isVisible(),true);
  await page.keyboard.press('Escape');assert.equal(await page.locator('#comboboxPopup').isHidden(),true);assert.equal(await page.locator('#terminalSessionPanel').isVisible(),true,'Escape closes only the list');
  pass(theme+': font field lists every font in its own face, follows typing and applies the choice');
  // Selects: closed size unchanged; the opened list uses the app colors and closes back to native.
  const closed=await geometry();
  const cursor=page.locator('#terminalAppearanceCursor');const before=await cursor.boundingBox();
  await cursor.click();await page.waitForFunction(()=>document.querySelector('#terminalAppearanceCursor').matches(':open'));
  const open=await page.evaluate(()=>{const s=document.querySelector('#terminalAppearanceCursor');const probe=document.createElement('div');probe.style.background='var(--panel)';document.body.append(probe);const panel=getComputedStyle(probe).backgroundColor;probe.remove();return {cls:s.classList.contains('ntk-picker'),picker:getComputedStyle(s,'::picker(select)').backgroundColor,panel};});
  assert.equal(open.cls,true);const during=await cursor.boundingBox();assert.deepEqual([during.width,during.height].map(Math.round),[before.width,before.height].map(Math.round),'an open select keeps its size');
  assert.equal(open.picker,open.panel,'the list uses the app panel color');
  await shot('select-'+theme);
  await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter');
  await page.waitForFunction(()=>!document.querySelector('#terminalAppearanceCursor').classList.contains('ntk-picker'));
  assert.notEqual(await page.evaluate(()=>state.terminalAppearance.cursor),undefined);
  assert.deepEqual(await geometry(),closed,'closed selects keep their native size');
  // Keyboard opening (Alt+Down) uses the same list.
  await cursor.focus();await page.keyboard.press('Alt+ArrowDown');await page.waitForFunction(()=>document.querySelector('#terminalAppearanceCursor').matches(':open'));await page.keyboard.press('Escape');
  await page.waitForFunction(()=>!document.querySelector('#terminalAppearanceCursor').classList.contains('ntk-picker'));
  assert.equal(await page.locator('#terminalSessionPanel').isVisible(),true,'Escape closes only the open list, not the panel');
  pass(theme+': select lists open in the app colors, keep their size and restore the native control when closed');
 }
 // Native geometry check: with the new stylesheet disabled, every closed select measures the same.
 const withCss=await geometry();await page.evaluate(()=>{for(const l of document.querySelectorAll('link[href*="dropdowns.css"]'))l.disabled=true;});const withoutCss=await geometry();await page.evaluate(()=>{for(const l of document.querySelectorAll('link[href*="dropdowns.css"]'))l.disabled=false;});
 assert.deepEqual(withCss,withoutCss);pass('closed selects are identical with and without the dropdown styles');
 // Other suggestion fields are converted too (time zone in App settings).
 await page.locator('[data-terminal-extra="close-session"]').click();await page.evaluate(()=>appSettings());await page.locator('[data-settings-section]').first().waitFor();
 for(const section of await page.locator('[data-settings-section]').all()){await section.click();if(await page.locator('#settingsTimeZone').count())break;}
 if(await page.locator('#settingsTimeZone').count()){assert.equal(await page.locator('#settingsTimeZone').getAttribute('list'),null);assert.equal(await page.locator('#settingsTimeZone').getAttribute('role'),'combobox');pass('time zone suggestions use the same list');}
 assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.errors.push(String(error?.stack||error));console.error(error);process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'dropdowns-results.json'),JSON.stringify(report,null,2));await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,errors:report.errors}));});
