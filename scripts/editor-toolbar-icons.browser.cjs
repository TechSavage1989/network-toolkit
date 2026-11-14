'use strict';
// Text Editor toolbar buttons (owner request after v1.21.57: "don't need a
// files button when the sidebar for files is already there. Docs should be at
// the top of the tab list in the text editor sidebar. Move/merge can be an icon
// instead of text. Save and save as can be icons. Open file can be an icon ...
// make a configurable option to toggle between text and icon buttons").
// Driven by TestEditorSessionBrowserToolbarButtons (editor_toolbar_browser_test.go):
// the shipped page as the main Toolkit window and its detached Text Editor
// window, both of ONE real engine; nothing is intercepted. Synthetic text only.
//  - the toolbar has no Files button; the sidebar rail reads Docs, Connect,
//    Files and opens, switches and closes the sidebar by click, keyboard and
//    the command palette;
//  - the buttons are icons by default: each keeps its accessible name and a
//    tooltip with its shortcut, Save keeps its primary look, and at 1280 px
//    the toolbar is one row;
//  - More → Toolbar buttons → Text shows their text with the same names and
//    tooltips; Find, Split and Compare keep their pressed look in both modes;
//  - the choice survives a reload (the engine keeps it in the shared
//    settings) and reaches a detached editor window through the engine's move
//    batch; that window at its native minimum (640 x 400) and the main window
//    at 1024 px never scroll sideways and keep every button inside the toolbar;
//  - Move / merge as an icon opens its menu, and a pending move still shows
//    its words and its indicator beside the icon, and can be cancelled there.
// Screenshots (light and dark) go to TOOLKIT_TEST_OUTPUT.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const E=process.env,url=E.NTK_TOOLBAR_URL,output=E.TOOLKIT_TEST_OUTPUT||E.NTK_BROWSER_OUTPUT_DIR;
const checks=[],errors=[];
const pass=text=>{checks.push(text);console.log('PASS '+text);};
const ED='#networkToolkitFileEditor';
// [data-ed, accessible name, tooltip, text]. The tooltip of Find and Replace
// is set by the Find window (editor-find.js) and says what a press does.
const BUTTONS=[
 ['browse','Open file','Open file (Ctrl+O)','Open file'],
 ['save','Save','Save (Ctrl+S)','Save'],
 ['saveAs','Save as','Save as (Ctrl+Shift+S)','Save as'],
 ['reload','Reload','Reload','Reload'],
 ['compare','Compare documents','Compare documents','Compare'],
 ['split','Split view','Split view','Split'],
 ['undo','Undo','Undo (Ctrl+Z)','Undo'],
 ['redo','Redo','Redo (Ctrl+Y)','Redo'],
 ['find','Find','Open the Find window on Find (Ctrl+F)','Find'],
 ['replace','Replace','Open the Find window on Replace (Ctrl+H)','Replace'],
 ['goto','Go to line','Go to line (Ctrl+G)','Go to line'],
 ['sendTerminal','Send to terminal','Send to terminal: the selection or the whole document (Ctrl+Shift+Enter)','Send to terminal'],
 ['tools','Text tools','Text tools: lines, encode and decode, format','Tools'],
 ['close','Close text editor','Close the text editor','Close editor'],
];
const SAMPLE=['! synthetic switch configuration','hostname lab-switch-01','ip domain-name example.com','!',...Array.from({length:40},(_,i)=>'interface GigabitEthernet0/'+(i+1)+'\n description uplink to 192.0.2.'+(i+10)+'\n no shutdown\n!'),'end',''].join('\n');

const frames=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(resolve,20)))));
// What the toolbar shows: every visible button with its name, tooltip, text,
// whether its icon or its text shows, and its box; the toolbar's box and any
// sideways scrolling of the toolbar or the page.
function readToolbar(page){
 return page.evaluate(()=>{
  const root=document.querySelector('#networkToolkitFileEditor'),bar=root.querySelector('.nt-editor-toolbar'),box=bar.getBoundingClientRect();
  const shown=el=>!!el&&getComputedStyle(el).display!=='none'&&el.getClientRects().length>0&&!el.closest('[hidden]');
  const name=b=>b.dataset.ed||(b.hasAttribute('data-editor-window-toggle')?'moveMerge':b.hasAttribute('data-editor-window-detach')?'detach':'');
  const buttons=[...bar.querySelectorAll('button')].filter(shown).map(b=>{const r=b.getBoundingClientRect(),icon=b.querySelector('svg.nt-editor-tool-icon'),text=b.querySelector('.nt-editor-tool-text');return {ed:name(b),label:b.getAttribute('aria-label'),title:b.getAttribute('title'),text:text?.textContent||'',icon:shown(icon),textShown:shown(text),primary:b.classList.contains('primary'),left:r.left,right:r.right,top:r.top,bottom:r.bottom,width:r.width};});
  return {mode:root.dataset.toolbarLabels,pref:NetworkToolkitFileEditor.toolbarLabels,box:{left:box.left,right:box.right,top:box.top,bottom:box.bottom,height:box.height},barScroll:bar.scrollWidth-bar.clientWidth,pageScroll:document.documentElement.scrollWidth-innerWidth,files:root.querySelectorAll('[data-ed="files"]').length,buttons};
 });
}
async function assertToolbar(page,mode,where,{oneRow=false}={}){
 const t=await readToolbar(page);
 assert.equal(t.mode,mode,where+': the editor shows '+mode);assert.equal(t.pref,mode,where+': the preference is '+mode);
 assert.equal(t.files,0,where+': no Files button in the toolbar');
 const by=Object.fromEntries(t.buttons.map(b=>[b.ed,b]));
 for(const [ed,label,title,text] of BUTTONS){
  const b=by[ed];assert(b,where+': '+ed+' is shown');
  assert.equal(b.label,label,where+': '+ed+' name');assert.equal(b.title,title,where+': '+ed+' tooltip');assert.equal(b.text,text,where+': '+ed+' text');
  assert.equal(b.icon,mode==='icons',where+': '+ed+' icon shown in '+mode);assert.equal(b.textShown,mode==='text',where+': '+ed+' text shown in '+mode);
  if(mode==='icons')assert(b.width<=44,where+': '+ed+' is a compact icon button ('+b.width+' px)');
 }
 assert(by.save.primary,where+': Save keeps its primary look');
 const move=by.moveMerge;assert(move,where+': Move / merge is shown');assert.match(move.label,/^Move or merge documents\. /);assert.equal(move.title,move.label,where+': Move / merge tooltip names the action');
 assert.equal(move.icon,mode==='icons');assert.equal(move.textShown,mode==='text');assert.equal(move.text,'Move / merge');
 const detach=by.detach;assert(detach,where+': Detach is shown');assert.equal(detach.label,'Detach editor');assert.equal(detach.title,'Detach editor');assert.equal(detach.icon,mode==='icons');assert.equal(detach.textShown,mode==='text');
 assert(t.barScroll<=1,where+': the toolbar does not scroll sideways ('+t.barScroll+')');assert(t.pageScroll<=1,where+': the page does not scroll sideways ('+t.pageScroll+')');
 for(const b of t.buttons)assert(b.left>=t.box.left-0.5&&b.right<=t.box.right+0.5&&b.top>=t.box.top-0.5&&b.bottom<=t.box.bottom+0.5,where+': '+b.ed+' lies inside the toolbar '+JSON.stringify({button:b,box:t.box}));
 const rows=new Set(t.buttons.map(b=>Math.round(b.top)));
 if(oneRow)assert.equal(rows.size,1,where+': the toolbar is one row ('+[...rows].join(', ')+')');
 return {...t,rows:rows.size};
}
// The rail: Docs first, then Connect and Files.
async function assertRail(page,where){
 const rail=await page.evaluate(()=>[...document.querySelectorAll('#networkToolkitFileEditor [data-editor-files-panel]')].map(b=>({panel:b.dataset.editorFilesPanel,label:b.getAttribute('aria-label'),text:b.textContent.trim()})));
 assert.deepEqual(rail.map(r=>r.panel),['documents','connections','files'],where+': the rail order');
 assert.deepEqual(rail.map(r=>r.label),['Documents','Connections','Files']);assert.deepEqual(rail.map(r=>r.text),['Docs','Connect','Files']);
}
const sidebar=page=>page.evaluate(()=>{const host=document.querySelector('#networkToolkitFileEditor [data-ed-files-sidebar]'),tab=document.querySelector('#networkToolkitFileEditor [data-editor-files-panel][aria-selected="true"]');return {open:!!host&&!host.hidden,panel:tab?.dataset.editorFilesPanel||'',focused:document.activeElement?.dataset?.editorFilesPanel||''};});
async function paletteRun(page,label){
 await page.keyboard.press('Control+Shift+P');await page.locator('#uxCommandOverlay .ux-command-input').waitFor();
 await page.locator('#uxCommandOverlay .ux-command-input').fill(label);
 await page.locator('#uxCommandResults .ux-command-row').filter({hasText:label}).first().click();await frames(page);
}
const styleOf=(page,ed)=>page.evaluate(ed=>{const b=document.querySelector('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="'+ed+'"]'),s=getComputedStyle(b);return {pressed:b.getAttribute('aria-pressed'),look:s.backgroundColor+'|'+s.boxShadow+'|'+s.color};},ed);
// Find, Split and Compare are toggles: pressed, they look pressed (in both modes).
async function assertToggles(page,mode){
 for(const [ed,open,shut] of [
  ['find',()=>page.waitForFunction(()=>!document.querySelector('#networkToolkitFileEditor [data-ed-find]')?.hidden),()=>page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-find]')?.hidden!==false)],
  ['split',()=>page.waitForFunction(()=>!!document.querySelector('#networkToolkitFileEditor.is-split')),()=>page.waitForFunction(()=>!document.querySelector('#networkToolkitFileEditor.is-split'))],
  ['compare',()=>page.waitForFunction(()=>!!document.querySelector('#networkToolkitFileEditor.is-comparing')),()=>page.waitForFunction(()=>!document.querySelector('#networkToolkitFileEditor.is-comparing'))],
 ]){
  const button=page.locator(ED+' .nt-editor-toolbar [data-ed="'+ed+'"]'),before=await styleOf(page,ed);
  assert.notEqual(before.pressed,'true',mode+': '+ed+' starts unpressed');
  await button.click();await open();await page.waitForFunction(ed=>document.querySelector('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="'+ed+'"]').getAttribute('aria-pressed')==='true',ed);await frames(page);
  const pressed=await styleOf(page,ed);assert.notEqual(pressed.look,before.look,mode+': pressed '+ed+' looks pressed');
  await shot(page,mode+'-'+ed+'-pressed');
  await button.click();await shut();await page.waitForFunction(ed=>document.querySelector('#networkToolkitFileEditor .nt-editor-toolbar [data-ed="'+ed+'"]').getAttribute('aria-pressed')==='false',ed);await frames(page);
 }
 pass(mode+': Find, Split and Compare keep their pressed look');
}
// More → View → Toolbar buttons: Icons or Text.
async function chooseLabels(page,value,name=''){
 await page.locator(ED+' .nt-editor-more summary').click();await page.locator(ED+' #ntEditorMoreActions').waitFor();
 const select=page.locator(ED+' #ntEditorMoreActions [data-ed-toolbar-labels]');
 assert.deepEqual(await select.locator('option').evaluateAll(list=>list.map(o=>o.value+'='+o.textContent)),['icons=Icons','text=Text']);
 assert.equal(await select.evaluate(el=>el.closest('label').textContent.trim().startsWith('Toolbar buttons')),true,'the choice is labelled Toolbar buttons');
 if(name)await shot(page,name);
 await select.selectOption(value);
 await page.waitForFunction(value=>document.querySelector('#networkToolkitFileEditor').dataset.toolbarLabels===value,value);await frames(page);
}
// The engine's copy of the saved settings (what a reload or another app reads).
const saved=page=>page.evaluate(async()=>(await api('/state'))?.editorPreferences?.toolbarLabels);
async function waitSaved(page,value){for(let i=0;i<100;i++){if(await saved(page)===value)return;await page.waitForTimeout(100);}assert.fail('the engine never kept Toolbar buttons = '+value);}
const textHeight=page=>page.evaluate(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').getBoundingClientRect().height);
// Screenshots wait for the theme's colour transitions and keep the pointer off
// the toolbar, so no button shows a hover look.
const calm=async page=>{const size=page.viewportSize();await page.mouse.move(size.width-4,size.height-4);await page.waitForFunction(()=>document.getAnimations().every(a=>a.playState!=='running'||a.effect?.getTiming?.().iterations===Infinity));await frames(page);};
async function theme(page,value){await page.evaluate(value=>setTheme(value),value);await page.waitForFunction(value=>document.documentElement.dataset.theme===value,value);await calm(page);}
async function shot(page,name){if(!output)return;await calm(page);await page.screenshot({path:path.join(output,'editor-toolbar-icons-'+name+'.png')});}

(async()=>{
 assert(url&&E.NTK_TOOLBAR_MAIN_TOKEN&&E.NTK_TOOLBAR_EDITOR_LAUNCHES,'run through go test (TestEditorSessionBrowserToolbarButtons)');
 if(output)fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,args:['--no-sandbox'],...(E.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:E.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 const pages=[];
 async function open(pathname,token,name,ready,viewport){
  const context=await browser.newContext({viewport}),page=await context.newPage();page.setDefaultTimeout(20000);pages.push([name,page]);
  page.on('pageerror',error=>errors.push(name+': '+error.message));
  await page.route('**/*',route=>new URL(route.request().url()).origin===url?route.continue():route.abort());
  await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},token);
  await page.goto(url+pathname);await page.waitForFunction(ready);
  return page;
 }
 const launches=async count=>{for(let i=0;i<200;i++){try{const list=JSON.parse(fs.readFileSync(E.NTK_TOOLBAR_EDITOR_LAUNCHES,'utf8'));if(list.length>=count)return list;}catch{}await new Promise(r=>setTimeout(r,100));}assert.fail('the main window never opened editor window '+count);};
 const ready=()=>window.__networkToolkitStartupState===1&&online;
 try{
  // ---- The main window, icons by default ----
  let main=await open('/',E.NTK_TOOLBAR_MAIN_TOKEN,'main window',ready,{width:1280,height:800});
  const showEditor=async page=>{await page.evaluate(()=>navigate('editor'));await page.locator(ED+' .nt-editor-toolbar').waitFor();await page.waitForFunction(()=>NetworkToolkitEditorWindows.state.enabled&&!!document.querySelector('[data-editor-window-toggle]'));};
  const openSamples=page=>page.evaluate(async text=>{const ed=NetworkToolkitFileEditor;for(const [id,body] of [['lab-a','lab-switch-01.cfg'],['lab-b','lab-switch-02.cfg']])await ed.open({id,path:body,source:'Synthetic local',binding:{kind:'local'},create:true,initialContent:id==='lab-b'?text.replace('lab-switch-01','lab-switch-02'):text,read:async()=>({content:text}),write:async()=>{throw Error('No save expected');}});ed.activateDocument('lab-a');},SAMPLE);
  await showEditor(main);await openSamples(main);await frames(main);
  assert.equal(await main.evaluate(()=>NetworkToolkitFileEditor.listDocuments().some(d=>d.dirty)),false,'the samples are clean');
  // ---- The rail and the palette open the sidebar ----
  const tab=panel=>main.locator(ED+' [data-editor-files-panel="'+panel+'"]');
  if(!(await sidebar(main)).open)await main.locator(ED+' [data-editor-files-reopen]').click();
  await assertRail(main,'main');
  await tab('documents').click();assert.deepEqual(await sidebar(main),{open:true,panel:'documents',focused:'documents'});
  assert.equal(await main.locator(ED+' [data-editor-documents]').isVisible(),true,'Docs shows the document list');
  await tab('documents').click();assert.equal((await sidebar(main)).open,false,'the open panel\'s rail item closes the sidebar');
  await tab('files').click();assert.deepEqual(await sidebar(main),{open:true,panel:'files',focused:'files'});
  await main.keyboard.press('ArrowUp');assert.equal((await sidebar(main)).panel,'connections','Up moves to Connect');
  await main.keyboard.press('ArrowUp');assert.equal((await sidebar(main)).panel,'documents','and to Docs');
  await main.keyboard.press('End');assert.equal((await sidebar(main)).panel,'files','End is Files');
  await paletteRun(main,'Show or hide the editor sidebar');assert.equal((await sidebar(main)).open,false,'the palette hides the sidebar');
  await paletteRun(main,'Show the connections sidebar');assert.deepEqual(await sidebar(main),{open:true,panel:'connections',focused:'connections'});
  await paletteRun(main,'Show the files sidebar');assert.deepEqual(await sidebar(main),{open:true,panel:'files',focused:'files'});
  await tab('documents').click();assert.deepEqual(await sidebar(main),{open:true,panel:'documents',focused:'documents'});
  pass('Sidebar: Docs, Connect, Files on the rail; it opens, switches (keyboard too) and closes the sidebar, and the palette shows Files and Connect or hides it');

  // ---- Icons by default ----
  for(const mode of ['light','dark']){await theme(main,mode);await assertToolbar(main,'icons','main 1280 '+mode,{oneRow:true});await shot(main,'main-1280-icons-'+mode);}
  await theme(main,'light');
  pass('Main window: no Files button; icon buttons with names and shortcut tooltips, Save primary, one row at 1280 px, light and dark');

  await assertToggles(main,'icons');
  // Move / merge as an icon opens its menu.
  const toggle=main.locator('[data-editor-window-toggle]');
  await toggle.click();await main.locator('[data-editor-window-controls]').waitFor();assert.equal(await toggle.getAttribute('aria-expanded'),'true');
  await main.keyboard.press('Escape');await main.locator('[data-editor-window-controls]').waitFor({state:'hidden'});assert.equal(await toggle.getAttribute('aria-expanded'),'false');
  pass('Move / merge as an icon opens and closes its menu');

  // ---- Text: the same buttons with their words ----
  await chooseLabels(main,'text','main-1280-more-menu');
  const wide=await assertToolbar(main,'text','main 1280 light');await shot(main,'main-1280-text-light');
  await theme(main,'dark');await assertToolbar(main,'text','main 1280 dark');await shot(main,'main-1280-text-dark');await theme(main,'light');
  await main.setViewportSize({width:1024,height:768});await frames(main);
  const narrow=await assertToolbar(main,'text','main 1024 light');await shot(main,'main-1024-text-light');
  await main.setViewportSize({width:1280,height:800});await frames(main);
  await assertToggles(main,'text');
  pass('Text: the buttons show their words with the same names and tooltips; '+wide.rows+' row(s) at 1280 px and '+narrow.rows+' at 1024 px, nothing scrolls sideways');

  // ---- Kept across a reload ----
  await waitSaved(main,'text');
  await main.evaluate(()=>NetworkToolkitFileEditor.closeAll());await main.waitForFunction(()=>NetworkToolkitFileEditor.listDocuments().length===0);
  await main.reload();await main.waitForFunction(ready);await showEditor(main);
  await main.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor').dataset.toolbarLabels==='text');
  await openSamples(main);await frames(main);await assertToolbar(main,'text','main after a reload');
  assert.equal(await main.locator(ED+' #ntEditorMoreActions [data-ed-toolbar-labels]').inputValue(),'text','More shows the kept choice');
  pass('The engine keeps Toolbar buttons = Text in the shared settings and a reload restores it');

  // ---- A detached editor window gets the choice ----
  await main.evaluate(()=>NetworkToolkitFileEditor.closeAll());await main.waitForFunction(()=>NetworkToolkitFileEditor.listDocuments().length===0);
  await main.evaluate(async text=>{const ed=NetworkToolkitFileEditor,doc=await ed.newDocument();const r=await ed.insertText(doc.id,text,'start');if(!r?.ok)throw new Error(r?.error||'insert');},SAMPLE);
  await main.waitForFunction(()=>!NetworkToolkitEditorWindows.state.running);
  await main.locator('[data-editor-window-detach]').click();
  const [first]=await launches(1);
  // Pending in Text mode: the words say so.
  await main.waitForFunction(()=>NetworkToolkitEditorWindows.state.outgoing===1);
  assert.equal(await toggle.getAttribute('data-editor-window-status'),'pending');
  const child=await open('/editor-view/'+first.view,first.token,'editor window',()=>online&&NetworkToolkitFileEditor.listDocuments().length===1,{width:640,height:400});
  await main.waitForFunction(()=>NetworkToolkitEditorWindows.state.outgoing===0&&NetworkToolkitFileEditor.listDocuments().length===0);
  await child.locator(ED+' .nt-editor-toolbar').waitFor();await child.waitForFunction(()=>!!document.querySelector('[data-editor-window-toggle]')&&!document.querySelector('[data-editor-window-toggle]').hidden);await frames(child);
  await assertRail(child,'editor window');
  const heights={};
  for(const mode of ['light','dark']){await theme(child,mode);await assertToolbar(child,'text','editor window 640x400 '+mode);heights.text=await textHeight(child);await shot(child,'window-640-text-'+mode);}
  pass('Detached window: the main window\'s Text choice arrived through the engine\'s move batch; 640 x 400 keeps every button inside the toolbar, light and dark');
  await theme(child,'light');await chooseLabels(child,'icons');
  for(const mode of ['light','dark']){await theme(child,mode);await assertToolbar(child,'icons','editor window 640x400 '+mode);heights.icons=await textHeight(child);await shot(child,'window-640-icons-'+mode);}
  assert(heights.icons>=heights.text,'icons leave the text at least as much room ('+JSON.stringify(heights)+')');
  assert(heights.text>=100,'the text keeps a usable height at 640 x 400 ('+JSON.stringify(heights)+')');
  pass('Detached window: Icons at 640 x 400, light and dark; the text area keeps '+Math.round(heights.text)+' px with text buttons and '+Math.round(heights.icons)+' px with icons');

  // ---- A pending move with icon buttons ----
  await chooseLabels(main,'icons');
  await main.evaluate(async text=>{const ed=NetworkToolkitFileEditor,doc=await ed.newDocument();const r=await ed.insertText(doc.id,text,'start');if(!r?.ok)throw new Error(r?.error||'insert');},SAMPLE);
  await main.waitForFunction(()=>!NetworkToolkitEditorWindows.state.running);
  await main.locator('[data-editor-window-detach]').click();await launches(2);
  await main.waitForFunction(()=>NetworkToolkitEditorWindows.state.outgoing===1);await frames(main);
  const pending=await main.evaluate(()=>{const b=document.querySelector('[data-editor-window-toggle]'),shown=el=>!!el&&getComputedStyle(el).display!=='none'&&el.getClientRects().length>0;return {status:b.dataset.editorWindowStatus,icon:shown(b.querySelector('svg')),label:shown(b.querySelector('[data-editor-window-label]'))?b.querySelector('[data-editor-window-label]').textContent:'',indicator:shown(b.querySelector('[data-editor-window-indicator]'))?b.querySelector('[data-editor-window-indicator]').textContent:''};});
  assert.deepEqual(pending,{status:'pending',icon:true,label:'Move pending · details',indicator:'…'},'a pending move shows its words and indicator beside the icon');
  await shot(main,'main-1280-icons-pending');
  await toggle.click();await main.locator('[data-editor-window-controls]').waitFor();
  await main.locator('[data-editor-window-cancel]').click();
  await main.waitForFunction(()=>NetworkToolkitEditorWindows.state.outgoing===0&&NetworkToolkitFileEditor.listDocuments().length===1&&[...NetworkToolkitFileEditor.documents.values()].every(d=>!d.frozen));
  await main.waitForFunction(()=>document.querySelector('[data-editor-window-toggle]').dataset.editorWindowStatus!=='pending');
  pass('Move / merge as an icon: a pending move shows "Move pending · details" and its indicator, and its menu cancels it; the tab stays');

  // Leave Text in the shared settings for the engine-side check.
  await chooseLabels(main,'text');await waitSaved(main,'text');
  assert.deepEqual(errors,[]);
  console.log('All '+checks.length+' toolbar button checks passed.');
 }catch(error){
  console.error(error.stack||String(error));
  for(const [name,page] of pages){try{if(output&&!page.isClosed())await page.screenshot({path:path.join(output,'editor-toolbar-icons-failure-'+name.replace(/\W+/g,'-')+'.png')});}catch{}}
  if(errors.length)console.error('Page errors: '+errors.join('; '));
  process.exitCode=1;
 }finally{await browser.close();}
})();
