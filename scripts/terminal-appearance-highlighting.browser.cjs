'use strict';
// Highlighting is part of the Appearance tab (owner request, v1.21.19: "I believe the highlighting settings
// should be with appearance, or at the very least allow selecting syntax highlighting from appearance as well
// so in real time you can see the preview"). The Syntax highlighting picker and the preview stay in view while
// the rest of the tab scrolls. Production assets with intercepted synthetic data; no connections, devices,
// files or credentials are used.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT,origin='https://appearance-highlighting.toolkit.test';
let stored={appUpdatePreferences:{enabled:false},theme:'light',activePage:'terminal',terminalAppearance:{fontFamily:'Consolas',highlightColors:{address:'#22aaff'}}};
const report={passed:false,checks:[],errors:[],screenshots:[],scope:'Synthetic production-asset browser acceptance in Chromium. Not native Windows WebView2 acceptance.'};
const pass=text=>{report.checks.push(text);console.log('PASS '+text);};
let browser,page;
async function shot(name){if(!out)return;const file='appearance-highlighting-'+name+'.png';await page.screenshot({path:path.join(out,file)});report.screenshots.push(file);}

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
 await open('main');
 await page.locator('#terminalSessionToggle').click();await page.locator('#terminalSessionPanel').waitFor({state:'visible'});
 const panel=page.locator('#terminalSessionPanel');
 assert.deepEqual(await panel.locator('[data-terminal-settings-tab]').allTextContents(),['Appearance','Features','Mouse & keyboard'],'no separate Highlighting tab');
 assert.equal(await panel.locator('[data-terminal-settings-panel="highlighting"]').count(),0);
 const appearance=panel.locator('[data-terminal-settings-panel="appearance"]');
 for(const id of ['#terminalAppearanceFont','#terminalSyntaxSelect','#terminalAppearancePreview','#terminalHighlightLegendInline','#terminalPromptHighlight','#terminalHighlightUnderlines','#terminalPaletteDetails'])
  assert.equal(await appearance.locator(id).count(),1,id+' is on the Appearance tab');
 // Order: font, then the picker and preview together, then the legend and options, then the palette.
 const order=await page.evaluate(()=>['#terminalAppearanceFont','#terminalSyntaxSelect','#terminalAppearancePreview','#terminalHighlightLegendInline','#terminalPromptHighlight','#terminalPaletteDetails'].map(id=>document.querySelector(id).getBoundingClientRect().top+document.querySelector('#terminalSessionPanel').scrollTop));
 assert.deepEqual([...order].sort((a,b)=>a-b),order,'Appearance reads font, picker, preview, legend, options, palette: '+JSON.stringify(order));
 assert.equal(await page.evaluate(()=>document.querySelector('#terminalSyntaxSelect').closest('.terminal-preview-sticky')===document.querySelector('#terminalAppearancePreview').closest('.terminal-preview-sticky')),true,'picker and preview are pinned together');
 await shot('top');
 pass('Highlighting is part of Appearance: font, Syntax highlighting picker with the preview, legend, options, palette');
 // Scrolled to the end, the picker and preview stay in view below the tabs.
 await page.evaluate(()=>{document.querySelector('#terminalSessionPanel').scrollTop=1e6;});await page.waitForTimeout(100);
 const pinned=await page.evaluate(()=>{const r=e=>document.querySelector(e).getBoundingClientRect(),p=r('#terminalSessionPanel'),tabs=r('.terminal-settings-tabs'),select=r('#terminalSyntaxSelect'),preview=r('#terminalAppearancePreview');return{panelBottom:p.bottom,tabsBottom:tabs.bottom,selectTop:select.top,previewBottom:preview.bottom,scrolled:document.querySelector('#terminalSessionPanel').scrollTop};});
 assert(pinned.scrolled>200,'the tab scrolls: '+JSON.stringify(pinned));
 assert(pinned.selectTop>=pinned.tabsBottom-1&&pinned.previewBottom<=pinned.panelBottom,'picker and preview stay visible while scrolled: '+JSON.stringify(pinned));
 const visibleAtPoint=await page.evaluate(()=>{const s=document.querySelector('#terminalSyntaxSelect').getBoundingClientRect();const hit=document.elementFromPoint(s.left+s.width/2,s.top+s.height/2);return hit===document.querySelector('#terminalSyntaxSelect')||document.querySelector('#terminalSyntaxSelect').contains(hit);});
 assert.equal(visibleAtPoint,true,'the pinned picker is not covered');
 // Choosing a profile while scrolled down updates the pinned preview at once.
 const before=await page.locator('#terminalPreviewText').innerHTML();
 await page.selectOption('#terminalSyntaxSelect','network');
 await page.waitForFunction(()=>/Network configuration/i.test(document.querySelector('#terminalPreviewProfile').textContent));
 assert.notEqual(await page.locator('#terminalPreviewText').innerHTML(),before,'the preview shows the new profile');
 assert.equal(await page.evaluate(()=>state.terminalSyntax.active),'network');
 await shot('pinned-network');
 // A highlighting option lower down also shows in the pinned preview.
 await page.selectOption('#terminalSyntaxSelect','smart');await page.waitForFunction(()=>/Universal/i.test(document.querySelector('#terminalPreviewProfile').textContent));
 await page.locator('#terminalHighlightUnderlines').scrollIntoViewIfNeeded();
 const underline=()=>page.evaluate(()=>[...document.querySelectorAll('#terminalPreviewText [style]')].filter(e=>/border-bottom: \d/.test(e.getAttribute('style'))).length);
 const lines=await underline();await page.locator('#terminalHighlightUnderlines').setChecked(false);await page.waitForFunction(n=>[...document.querySelectorAll('#terminalPreviewText [style]')].filter(e=>/border-bottom: \d/.test(e.getAttribute('style'))).length<n,lines);
 assert.equal(await page.locator('#terminalAppearancePreview').isVisible(),true);
 await page.locator('#terminalHighlightUnderlines').setChecked(true);
 pass('The picker and preview stay in view while scrolled, and profile or option changes show in the preview at once');
 // Older links to the Highlighting tab open Appearance at the picker.
 await page.locator('[data-terminal-extra="close-session"]').click();
 await page.evaluate(()=>NetworkToolkitTerminalExtras.openSettings('highlighting'));
 assert.equal(await page.locator('[data-terminal-settings-tab="appearance"]').getAttribute('aria-selected'),'true');
 assert.equal(await page.locator('#terminalSyntaxSelect').isVisible(),true);
 pass('Opening the old Highlighting tab shows Appearance with the Syntax highlighting picker');
 // Dark theme screenshot for review.
 await page.evaluate(()=>setTheme('dark'));await page.waitForTimeout(150);await page.evaluate(()=>{document.querySelector('#terminalSessionPanel').scrollTop=500;});await page.waitForTimeout(100);await shot('dark-scrolled');
 assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.errors.push(String(error?.stack||error));console.error(error);process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'appearance-highlighting-results.json'),JSON.stringify(report,null,2));await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,errors:report.errors}));});
