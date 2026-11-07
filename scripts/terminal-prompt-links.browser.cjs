'use strict';
// Device prompt name color, URL / network-path underlines and the Universal
// Customize page (v1.21.12). Production assets with intercepted synthetic
// settings; no connections, devices, files or credentials are used.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT,origin='https://prompt-links.toolkit.test';
let stored={appUpdatePreferences:{enabled:false},theme:'light',activePage:'terminal',terminalAppearance:{fontFamily:'Consolas',highlightColors:{address:'#22aaff'}}};
const report={passed:false,checks:[],errors:[],screenshots:[],scope:'Synthetic production-asset browser acceptance in Chromium. Not native Windows WebView2 acceptance.'};
const pass=text=>{report.checks.push(text);console.log('PASS '+text);};
let browser,page;
async function shot(name){if(!out)return;const file='prompt-links-'+name+'.png';await page.screenshot({path:path.join(out,file)});report.screenshots.push(file);}
const appearance=()=>page.evaluate(()=>JSON.parse(JSON.stringify(state.terminalAppearance)));
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true});page=await browser.newPage({viewport:{width:1280,height:800},serviceWorkers:'block'});page.setDefaultTimeout(15000);
 page.on('pageerror',error=>report.errors.push(error.message));
 await page.route('**/*',async route=>{if(await handleEditorActivityMetadata(route,origin,report))return;
  const req=route.request(),url=new URL(req.url()),p=url.pathname;assert.equal(url.origin,origin,'No external requests are allowed');
  if(p.startsWith('/api/')){const method=req.method();let data={};
   if(method!=='GET'){assert.equal(p,'/api/state','Highlighting settings must not start connections or jobs');stored=req.postDataJSON();}
   if(p==='/api/state'&&method==='GET')data=stored;
   if(p==='/api/status')data={version:require('../package.json').version,os:'windows',arch:'amd64',storageWritable:true,system:{hostname:'Prompt fixture',interfaces:[]}};
   if(p==='/api/system')data={hostname:'Prompt fixture',interfaces:[]};
   if(p==='/api/jobs')data=[];
   if(p==='/api/terminals'||p==='/api/remote-browser')data={sessions:[]};
   if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   if(p==='/api/credential-vault')data={available:false,state:'unavailable'};
   if(p==='/api/credentials')data={vault:{available:false,state:'unavailable'},entries:[]};
   if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,80));data={events:[],sessions:[],after:0};}
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  try{const icon=/^\/assets\/Network_Toolkit_Icon\.(svg|ico)$/.test(p)||p==='/favicon.ico',file=icon?path.join(root,'assets',p==='/favicon.ico'?'Network_Toolkit_Icon.ico':path.basename(p)):path.join(root,'web',p==='/'?'index.html':p);let body=fs.readFileSync(file);if(p==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','prompt-links-fixture').replaceAll('__WINDOW_MODE__','main');return route.fulfill({body,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':p.endsWith('.ico')?'image/x-icon':'application/octet-stream'});}catch{return route.fulfill({status:404,body:''});}
 });
 await page.goto(origin+'/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);
 async function highlighting(){if(await page.locator('#terminalSessionPanel.hidden').count())await page.locator('#terminalSessionToggle').click();await page.locator('[data-terminal-settings-tab="highlighting"]').click();await page.locator('#terminalPromptOptions').waitFor({state:'visible'});}
 for(const theme of ['light','dark']){
  // Each pass starts from the app colors (editing a color selects Custom colors).
  await page.evaluate(theme=>{setTheme(theme);navigate('terminal');state.terminalAppearance=window.NetworkToolkitTerminalAppearance.normalize({...state.terminalAppearance,scheme:'app',colors:{}});updateTerminalTheme();window.NetworkToolkitTerminalSyntaxUI.choose('smart');},theme);
  await highlighting();
  // Device prompt name: one control for every profile, with its own color.
  const prompt=page.locator('#terminalPromptHighlight');
  assert.deepEqual(await prompt.locator('option').allTextContents(),['Color it unless the device does','Always my color','No color']);
  await prompt.selectOption('always');assert.equal((await appearance()).promptHighlight,'always');
  assert.equal(await page.locator('#terminalPromptColor').isVisible(),true,'the prompt name color sits beside its choice');
  // Two separate underline switches.
  const link=page.locator('#terminalHighlightLinkUnderlines'),all=page.locator('#terminalHighlightUnderlines');
  await all.check();await link.uncheck();let saved=await appearance();assert.equal(saved.highlightLinkUnderlines,false);assert.equal(saved.highlightUnderlines,true,'turning off link underlines leaves the others');
  await link.check();await all.uncheck();saved=await appearance();assert.equal(saved.highlightLinkUnderlines,true);assert.equal(saved.highlightUnderlines,false);
  const chip=page.locator('#terminalHighlightLegendInline [data-highlight-legend-kind="url"]');assert.match(await chip.evaluate(el=>el.style.borderBottom),/1px solid/,'URL chip keeps its underline');
  assert.match(await page.locator('#terminalHighlightLegendInline [data-highlight-legend-kind="address"]').evaluate(el=>el.style.borderBottom),/^0px/,'other chips lose theirs');
  assert.equal(await page.locator('#terminalHighlightRules,#terminalHighlightColorDetails').count(),0,'rules and colors are no longer repeated in the tab');
  await shot('tab-'+theme);
  pass(theme+': Highlighting tab has the prompt name choice and color and two separate underline switches');
  // Rules and colors… opens Universal in the Customize dialog.
  await page.locator('#terminalUniversalOptions [data-terminal-syntax-set="smart"]').click();
  const dialog=page.locator('#modalRoot [data-terminal-universal-editor]');await dialog.waitFor();
  assert.equal(await page.locator('#syntaxEditSet').inputValue(),'smart');assert.equal(await page.locator('#modalRoot [data-syntax-act="use"]').textContent(),'Use Universal');
  assert.equal(await dialog.locator('[data-terminal-highlight-rule]').count(),15);assert.equal(await dialog.locator('[data-terminal-highlight-color]').count(),15);
  assert.equal(await page.locator('#syntaxPromptName').inputValue(),'always');assert.equal(await page.locator('#syntaxUniversalUnderlines').isChecked(),false);assert.equal(await page.locator('#syntaxUniversalLinkUnderlines').isChecked(),true);
  const preview=page.locator('#syntaxPreview');
  const span=kind=>preview.locator('[data-universal-kind="'+kind+'"]').first();
  assert.equal(await span('prompt-name').textContent(),'biqu@CR10SPro');assert.equal(await span('url').textContent(),'\\\\fileserver\\backups\\core-1.cfg');
  assert.match(await span('url').evaluate(el=>el.style.borderBottom),/solid/);
  await page.locator('#syntaxUniversalLinkUnderlines').uncheck();assert.equal(await span('url').evaluate(el=>el.style.borderBottom),'','the preview follows the link underline switch');
  await page.locator('#syntaxUniversalLinkUnderlines').check();
  await dialog.locator('[data-terminal-highlight-rule="address"]').uncheck();assert.equal(await preview.locator('[data-universal-kind="address"]').count(),0);
  await dialog.locator('[data-terminal-rules-reset]').click();assert(await preview.locator('[data-universal-kind="address"]').count()>0);
  await page.locator('#syntaxPromptName').selectOption('off');assert.equal(await span('prompt-plain').textContent(),'biqu@CR10SPro');assert.equal((await appearance()).promptHighlight,'off');
  await page.locator('#syntaxPromptName').selectOption('always');
  const fits=await page.evaluate(()=>{const body=document.querySelector('#modalRoot .modal-body');return body.scrollWidth<=body.clientWidth+1;});assert.equal(fits,true,'the Universal page fits without sideways scrolling');
  await shot('universal-'+theme);
  // Change a color from the dialog with the shared color picker.
  await dialog.locator('#terminalHighlightColor-prompt').click();await page.locator('.ntk-color-dialog').waitFor();
  await page.locator('.ntk-color-dialog [data-color-value="hex"]').fill('#cc5500');
  await page.locator('.ntk-color-dialog').getByRole('button',{name:'Apply',exact:true}).click();
  assert.equal((await appearance()).highlightColors.prompt,'#cc5500');assert.match(await span('prompt-name').evaluate(el=>el.style.color),/rgb\(204, 85, 0\)|#cc5500/);
  await dialog.locator('[data-terminal-highlight-colors-reset]').click();assert.deepEqual((await appearance()).highlightColors,{});
  // Switching to a rule set and back keeps the dialog usable.
  await page.locator('#syntaxEditSet').selectOption('network');await page.locator('#syntaxRule-red').waitFor({state:'attached'});assert.equal(await page.locator('#modalRoot [data-syntax-act="use"]').textContent(),'Use this rule set');
  // v1.21.14: rule set pages use the same cards as Universal.
  assert.equal(await page.locator('#modalRoot [data-syntax-slot]').count(),7);assert.equal(await page.locator('#modalRoot [data-terminal-prompt-card]').count(),1);
  assert.equal(await page.locator('#syntaxRulePromptName').inputValue(),await page.evaluate(()=>state.terminalAppearance.promptHighlight),'the prompt card shows the saved choice');
  await page.locator('#syntaxSlotOn-red').click();await page.waitForFunction(()=>state.terminalSyntax.off?.network?.[0]==='red');assert.equal(await page.locator('#syntaxSlotOn-red').isChecked(),false);
  assert.equal(await page.evaluate(()=>{const body=document.querySelector('#modalRoot .modal-body');return body.scrollWidth<=body.clientWidth+1;}),true,'rule set cards fit without sideways scrolling');
  await shot('ruleset-cards-'+theme);
  await page.locator('#syntaxSlotOn-red').click();await page.waitForFunction(()=>!state.terminalSyntax.off?.network);
  await page.locator('#syntaxEditSet').selectOption('smart');await dialog.waitFor();
  await page.locator('#modalRoot [data-syntax-act="use"]').click();assert.equal(await page.evaluate(()=>state.terminalSyntax.active),'smart');
  await page.locator('#modalRoot [data-action="close-modal"]').first().click();
  pass(theme+': Universal opens in Customize with every rule, color, underline and prompt choice and a live preview');
  // With a keyword rule set the prompt name choice still applies and shows in the legend.
  await page.evaluate(()=>window.NetworkToolkitTerminalSyntaxUI.choose('network'));await highlighting();
  assert.equal(await page.locator('#terminalUniversalOptions').isHidden(),true);assert.equal(await page.locator('#terminalPromptOptions').isVisible(),true);
  const promptChip=page.locator('#terminalHighlightLegendInline [data-highlight-legend-kind="prompt"]');assert.equal(await promptChip.textContent(),'Device prompt name');
  await page.locator('#terminalPromptHighlight').selectOption('auto');assert.equal(await promptChip.count(),0,'Automatic leaves rule sets alone');
  await page.locator('#terminalPromptHighlight').selectOption('always');
  pass(theme+': the prompt name choice works with keyword rule sets');
  // v1.21.13: the Appearance preview shows the rule set's own highlighting.
  await page.locator('[data-terminal-settings-tab="appearance"]').click();const sample=page.locator('#terminalAppearancePreview');await sample.waitFor({state:'visible'});
  assert.match(await page.locator('#terminalPreviewProfile').textContent(),/Network configuration/);
  assert(await sample.locator('[data-terminal-preview-highlight^="syn-"]').count()>0,'the preview uses the rule set');assert.equal(await sample.locator('[data-terminal-preview-highlight="address"]').count(),0,'no Universal colors while a rule set is active');
  assert.equal(await sample.locator('[data-terminal-preview-highlight="prompt-name"]').textContent(),'router');
  await shot('preview-ruleset-'+theme);
  await page.evaluate(()=>window.NetworkToolkitTerminalSyntaxUI.choose('smart'));assert.equal(await page.locator('#terminalPreviewProfile').textContent(),'· Universal');assert(await sample.locator('[data-terminal-preview-highlight="address"]').count()>0);
  pass(theme+': the Appearance preview follows the active profile, including rule sets');
  await page.locator('#terminalSessionToggle').click();
 }
 // Smallest supported window.
 await page.setViewportSize({width:960,height:600});await page.evaluate(()=>window.NetworkToolkitTerminalSyntaxUI.open('smart'));await page.locator('#modalRoot [data-terminal-universal-editor]').waitFor();
 // A keyword rule set is active here; the Universal page still shows Universal's own colors.
 const expected=await page.evaluate(()=>{const s=window.NetworkToolkitTerminalAppearance.highlightOptions(state.terminalAppearance,state.theme);return NTTerminalHighlight.legend(s.theme,{...s,ruleset:null,ruleSet:null}).find(m=>m.kind==='error').color;});
 assert.equal(await page.locator('#terminalHighlightColor-error').evaluate(el=>el.value),expected);assert(await page.locator('#syntaxPreview [data-universal-kind="error"]').count()>0,'the preview colors Universal output while a rule set is active');
 assert.equal(await page.evaluate(()=>{const body=document.querySelector('#modalRoot .modal-body');return body.scrollWidth<=body.clientWidth+1&&document.documentElement.scrollWidth<=innerWidth+1;}),true,'the dialog fits at 960×600');
 await shot('universal-960');await page.locator('#modalRoot [data-action="close-modal"]').first().click();
 pass('the Universal page fits the minimum 960×600 window');
 // Choices survive a reload.
 await page.evaluate(()=>flushState());await page.reload();await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);
 const restored=await appearance();assert.equal(restored.promptHighlight,'always');assert.equal(restored.highlightLinkUnderlines,true);assert.equal(restored.highlightUnderlines,false);
 pass('prompt name and underline choices survive a reload');
 assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.errors.push(String(error?.stack||error));console.error(error);process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'prompt-links-results.json'),JSON.stringify(report,null,2));await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,errors:report.errors}));});
