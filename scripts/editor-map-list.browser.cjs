'use strict';
// Document Map and Document List of the Text Editor (owner request, like
// Notepad++'s View > Document Map and Document List). Synthetic data only.
//
// Part 1, the shipped page with synthetic engine routes (always):
//  - the map is off by default; the More menu's "Document map" turns it on,
//    and the setting is saved and restored after a reload;
//  - the box over the map is the text area's visible share, at every scroll
//    position; a click centres the view there, dragging the box scrolls, and
//    the wheel over the map scrolls the document;
//  - the map follows typing, word wrap, the font size, the light and dark
//    themes and the Custom palette, follows the active pane of a split view,
//    and is hidden in the comparison view;
//  - a 16 MB page-held document: the map draws a down-sampled overview and
//    its own work stays far below 50 ms while scrolling and typing (the text
//    area of a 16 MB page-held document is itself slow to scroll in Chromium,
//    which is why the app keeps such files disk-backed; Part 2 checks long
//    tasks there);
//  - the Document List on the rail (Connect, Files, Docs): every tab in tab
//    order with its extension and unsaved marker, the active one highlighted;
//    click, Up/Down and Enter activate; the right-click menu is the tab menu;
//    left and right view groups in split view; the panel is restored after a
//    reload; command palette and F1 help rows;
//  - a detached editor window at its native minimum size (640 x 400) stays
//    usable with both.
// Part 2, when NTK_EDITOR_STANDALONE_URL is set (TestEditorSessionBrowserMapList
// in editor_map_list_browser_test.go runs the standalone Text Editor against
// its production engine): a 16 MB disk-backed document is mapped by line and
// scrolled through the map with no long task over 50 ms; a 1.7 MB single-line
// Base64 document with Wrap on is mapped by character offset and the map
// reaches its first and last characters; both settings are kept by the engine.
// Screenshots (light and dark, device scale 1) go to TOOLKIT_TEST_OUTPUT.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version;
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR||path.join(root,'test-results','editor-map-list');
const {NTK_EDITOR_STANDALONE_URL:engineURL,NTK_EDITOR_STANDALONE_TOKEN:engineToken,NTK_EDITOR_MAP_FILES:engineFiles}=process.env;
const report={passed:false,checks:[],errors:[]};
const pass=name=>{report.checks.push(name);console.log('PASS '+name);};

// ---- Synthetic documents (192.0.2.x, example.com) ----
function deviceConfig(count,name){
 const out=['! '+name+' synthetic configuration','hostname '+name,'ip domain-name example.com','!'];
 for(let i=0;i<count;i++)out.push('interface GigabitEthernet0/'+i,' description link to 192.0.2.'+(i%250),' ip address 10.'+(i%250)+'.0.1 255.255.255.0',' no shutdown','!');
 out.push('end','');return out.join('\n');
}
const FILES=new Map([
 ['C:\\lab\\core-router.cfg',deviceConfig(140,'core-router')],
 ['C:\\lab\\notes.txt','Synthetic notes for 192.0.2.10\n\nNothing here is from a real network.\n'],
 ['C:\\lab\\inventory.json',JSON.stringify({devices:Array.from({length:40},(_,i)=>({name:'sw-'+i+'.example.com',address:'192.0.2.'+i,role:i%3?'access':'distribution'}))},null,2)+'\n'],
 ['C:\\lab\\long-lines.txt',Array.from({length:60},(_,i)=>'Entry '+i+': '+'synthetic value 192.0.2.'+(i%250)+' '.repeat(1)+'abcdefghij'.repeat(12+(i%5))).join('\n')+'\n'],
]);

// ---- The shipped page with synthetic engine routes ----
async function serve(page,{mode='main',view=null,state}){
 page.on('pageerror',error=>report.errors.push(mode+': '+error.message));
 await page.route('https://toolkit.local/**',async route=>{
  const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method();
  if(p.startsWith('/api/')){
   const body=request.postData()?request.postDataJSON():null;let data={},status=200;
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',windowMode:mode,editorWindowAvailable:!!view,editorDiskAvailable:false,editorSessionAvailable:false};
   else if(p==='/api/state'){if(method==='GET')data=structuredClone(state.saved);else{const {_sharedBase,...rest}=body||{};state.saved={...state.saved,...rest};state.posts++;data={ok:true};}}
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/terminals')data={sessions:[]};
   else if(p==='/api/editor-views'||p==='/api/editor-views/current')data={available:true,ownerInstanceId:'e'.repeat(32),self:view?.id||'',views:[],incoming:[],outgoing:[],owned:{},recovery:[]};
   else if(p==='/api/editor-views/current/state')data={owned:Object.fromEntries((body?.batch?.documents||[]).map(d=>[d.uid,1]))};
   else if(p==='/api/file-editor/local'||/^\/api\/editor-views\/current\/local/.test(p)){
    if(body?.action==='read-text'){const content=FILES.get(body.path);if(content!==undefined)data={content,encoding:'UTF-8',bom:false,hash:'a'.repeat(64)};else{status=400;data={error:'The system cannot find the file specified.'};}}
    else data={hash:'b'.repeat(64)};
   }else if(p.includes('/events')){await new Promise(resolve=>setTimeout(resolve,100));data={events:[],sessions:[],after:0};}
   else data={sessions:[],entries:[]};
   return route.fulfill({status,contentType:'application/json',body:JSON.stringify(data)});
  }
  try{
   const html=p==='/'||p.startsWith('/editor-view/'),file=path.join(root,'web',html?'index.html':p);
   let bytes=fs.readFileSync(fs.existsSync(file)?file:path.join(root,p));
   if(html)bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','map-list-fixture').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__','null').replaceAll('__EDITOR_VIEW__',view?JSON.stringify(view):'null').replaceAll('src="assets/','src="/assets/').replaceAll('href="assets/','href="/assets/');
   return route.fulfill({body:bytes,contentType:html?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
}

// ---- Helpers that read the page ----
const frames=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(resolve,20)))));
// The map's box against the active text area's own scroll geometry.
function mapInfo(page){
 return page.evaluate(()=>{
  const host=document.querySelector('#networkToolkitFileEditor [data-ed-map]'),box=host?.querySelector('[data-ed-map-view]'),map=NetworkToolkitFileEditor.documentMap,layout=map?.layout;
  const d=NetworkToolkitFileEditor.active,ta=NetworkToolkitFileEditor.comparison?null:(document.querySelector('.nt-editor.is-split [data-compare-text]:focus')||null);
  const area=[...document.querySelectorAll('#networkToolkitFileEditor textarea[data-ed-text],#networkToolkitFileEditor textarea[data-compare-text]')].filter(el=>el.offsetParent);
  const style=host&&getComputedStyle(host),rect=host?.getBoundingClientRect();
  return {hidden:!host||host.hidden||style.display==='none',width:rect?.width||0,height:host?.clientHeight||0,top:rect?.top||0,left:rect?.left||0,
   box:box&&!box.hidden?{top:parseFloat(box.style.top),height:parseFloat(box.style.height)}:null,
   layout:layout?{kind:layout.kind,g:layout.g,key:layout.key}:null,stats:{...map?.stats},
   doc:d?{uid:d.uid,name:d.path.split(/[\\/]/).pop(),disk:!!d.disk}:null,areas:area.length,focused:!!ta};
 });
}
// The text area that shows the active document (single view or the active pane).
function activeArea(page){
 return page.evaluate(()=>{
  const ta=(()=>{const groups=NetworkToolkitFileEditor.viewGroups,split=document.querySelector('#networkToolkitFileEditor.is-split');if(!split)return document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');return document.querySelector('#networkToolkitFileEditor [data-compare-text="'+groups.activeSide+'"]');})();
  const style=getComputedStyle(ta);
  return {scrollTop:ta.scrollTop,scrollHeight:ta.scrollHeight,clientHeight:ta.clientHeight,lineHeight:parseFloat(style.lineHeight),wrap:ta.wrap,side:NetworkToolkitFileEditor.viewGroups.activeSide};
 });
}
async function setActiveScroll(page,fraction){
 await page.evaluate(fraction=>{
  const split=document.querySelector('#networkToolkitFileEditor.is-split'),ta=split?document.querySelector('#networkToolkitFileEditor [data-compare-text="'+NetworkToolkitFileEditor.viewGroups.activeSide+'"]'):document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');
  ta.scrollTop=Math.round((ta.scrollHeight-ta.clientHeight)*fraction);ta.dispatchEvent(new Event('scroll'));
 },fraction);
 await frames(page);
}
// The box must be the visible share of the text area: top = scrollTop x scale,
// height = clientHeight x scale (never less than the minimum box).
async function assertBoxMatches(page,label){
 const [info,ta]=await Promise.all([mapInfo(page),activeArea(page)]);
 assert(!info.hidden&&info.box&&info.layout,label+': the map shows a box');
 assert.equal(info.layout.kind,'text',label+': a page-held document is mapped by its text area');
 const scale=info.layout.g.scale,used=info.layout.g.used;
 assert(Math.abs(info.layout.g.total-ta.scrollHeight)<=1,label+': the map covers the whole scroll height ('+info.layout.g.total+' / '+ta.scrollHeight+')');
 assert(Math.abs(used-Math.min(info.height,ta.scrollHeight/ta.lineHeight*3))<=1,label+': the map is 3 px per row, or fits the whole document to its height');
 const height=Math.max(10,ta.clientHeight*scale),top=Math.min(ta.scrollTop*scale,used-Math.min(used,height));
 assert(Math.abs(info.box.top-top)<=1,label+': box top '+info.box.top+' should be '+top);
 assert(Math.abs(info.box.height-Math.min(used,height))<=1,label+': box height '+info.box.height+' should be '+height);
 return {info,ta};
}
const canvasSum=page=>page.evaluate(()=>{const c=document.querySelector('#networkToolkitFileEditor [data-ed-map] canvas'),x=c.getContext('2d').getImageData(0,0,c.width,c.height).data;let sum=0;for(let i=0;i<x.length;i+=7)sum=(sum*31+x[i])>>>0;return sum;});
const canvasPixel=(page,x,y)=>page.evaluate(([x,y])=>{const c=document.querySelector('#networkToolkitFileEditor [data-ed-map] canvas'),r=devicePixelRatio||1;return [...c.getContext('2d').getImageData(Math.round(x*r),Math.round(y*r),1,1).data].slice(0,3);},[x,y]);
const listRows=page=>page.evaluate(()=>[...document.querySelectorAll('#networkToolkitFileEditor [data-editor-documents] [data-ed-doc-uid]')].map(row=>({uid:row.dataset.edDocUid,name:row.querySelector('.nt-editor-documents-name').textContent,ext:row.querySelector('.nt-editor-documents-ext').textContent,mark:row.querySelector('.nt-editor-documents-mark').textContent,active:row.getAttribute('aria-selected')==='true',label:row.getAttribute('aria-label'),focused:row===document.activeElement,group:row.closest('[role="group"]')?.querySelector('.nt-editor-documents-group')?.textContent||''})));
const tabOrder=page=>page.evaluate(()=>[...NetworkToolkitFileEditor.documents.values()].map(d=>d.uid));
const activeUID=page=>page.evaluate(()=>NetworkToolkitFileEditor.active?.uid||'');
async function shot(page,name){if(output)await page.screenshot({path:path.join(output,name+'.png')});}
async function openMore(page){const more=page.locator('#networkToolkitFileEditor .nt-editor-more');if(!await more.evaluate(el=>el.open))await more.locator('summary').click();await page.locator('#ntEditorMoreActions').waitFor({state:'visible'});}
async function paletteRun(page,label){
 await page.keyboard.press('Control+Shift+P');await page.locator('#uxCommandOverlay .ux-command-input').waitFor();
 await page.locator('#uxCommandOverlay .ux-command-input').fill(label);
 await page.locator('#uxCommandResults .ux-command-row').filter({hasText:label}).first().click();await frames(page);
}

// ---- Part 1: the shipped page ----
async function mainWindow(browser){
 const state={saved:{appUpdatePreferences:{enabled:false},theme:'light',activePage:'editor',firstLaunchWelcomeSeen:true,welcomeDismissed:true,firstLaunchWelcome:{dismissed:true}},posts:0};
 const context=await browser.newContext({viewport:{width:1280,height:820},deviceScaleFactor:1}),page=await context.newPage();
 await serve(page,{state});
 const start=async()=>{await page.goto('https://toolkit.local/');await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online);await page.evaluate(()=>navigate('editor'));await page.locator('#networkToolkitFileEditor [data-ed-text]').first().waitFor();};
 await start();
 for(const file of FILES.keys())assert.equal(await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file).then(()=>'',error=>error.message),file),'');
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][0]));
 const text=page.locator('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');
 await frames(page);

 // Off by default.
 let info=await mapInfo(page);
 assert.equal(info.hidden,true,'the document map is off by default');
 await openMore(page);
 const mapBox=page.locator('#ntEditorMoreActions [data-ed-document-map]'),listBox=page.locator('#ntEditorMoreActions [data-ed-document-list]');
 assert.equal(await mapBox.isChecked(),false);assert.equal(await listBox.isChecked(),false);
 assert.equal(await page.locator('#ntEditorMoreActions .nt-editor-more-heading').textContent(),'View');
 await mapBox.check();await frames(page);
 info=await mapInfo(page);
 assert.equal(info.hidden,false,'the More menu shows the document map');
 assert(info.width>=90&&info.width<=100,'the map is a narrow panel: '+info.width);
 const editorBox=await page.locator('#networkToolkitFileEditor .nt-editor-workspace').boundingBox();
 assert(Math.abs(info.left+info.width-(editorBox.x+editorBox.width))<=1,'the map is at the right edge of the document area');
 await page.waitForFunction(()=>state.editorPreferences?.documentMap===true);
 for(let i=0;i<40&&state.saved.editorPreferences?.documentMap!==true;i++)await page.waitForTimeout(50);
 assert.equal(state.saved.editorPreferences?.documentMap,true,'the setting is saved with the editor preferences');
 pass('Document map: off by default; the More menu turns it on at the right edge and saves it');

 // The box is the visible share at every scroll position.
 for(const fraction of [0,0.25,0.5,0.8,1])await setActiveScroll(page,fraction),await assertBoxMatches(page,'scroll '+fraction);
 info=await mapInfo(page);
 assert(info.layout.g.scale*(await activeArea(page)).lineHeight<1,'a 705-line document is fitted to the map height');
 // A click centres the view there.
 await setActiveScroll(page,0);
 const used=info.layout.g.used;
 for(const at of [0.6,0.3]){
  const y=used*at;await page.mouse.click(info.left+info.width/2,info.top+y);await frames(page);
  const ta=await activeArea(page),g=(await mapInfo(page)).layout.g,expected=Math.max(0,Math.min(ta.scrollHeight-ta.clientHeight,y/g.scale-ta.clientHeight/2));
  assert(Math.abs(ta.scrollTop-expected)<=2/g.scale,'a click at '+at+' centres the view there ('+ta.scrollTop+' / '+expected+')');
  await assertBoxMatches(page,'after a click');
 }
 // Dragging the box scrolls by the distance moved.
 {
  const before=await activeArea(page),{box,layout}=await mapInfo(page),x=info.left+info.width/2,y=info.top+box.top+box.height/2;
  await page.mouse.move(x,y);await page.mouse.down();for(let step=1;step<=6;step++)await page.mouse.move(x,y+step*10);await page.mouse.up();await frames(page);
  const after=await activeArea(page);
  assert(Math.abs(after.scrollTop-before.scrollTop-60/layout.g.scale)<=2/layout.g.scale,'dragging the box 60 px scrolls 60 map pixels ('+(after.scrollTop-before.scrollTop)+')');
  assert.equal(await text.evaluate(el=>el===document.activeElement||!document.activeElement.closest('[data-ed-map]')),true,'the map takes no focus from the text');
  await assertBoxMatches(page,'after a drag');
 }
 // The wheel over the map scrolls the document.
 {
  await setActiveScroll(page,0.2);const before=await activeArea(page);
  await page.mouse.move(info.left+info.width/2,info.top+20);await page.mouse.wheel(0,300);await frames(page);
  const after=await activeArea(page);
  assert(Math.abs(after.scrollTop-before.scrollTop-300)<=1,'the wheel over the map scrolls the text by its distance ('+(after.scrollTop-before.scrollTop)+')');
  await assertBoxMatches(page,'after the wheel');
 }
 pass('Document map: the box matches the text area scroll; a click centres the view, dragging the box and the wheel scroll it');

 // Typing: the map follows the edit.
 {
  const before=await canvasSum(page),total=(await mapInfo(page)).layout.g.total;
  await text.focus();await page.keyboard.press('Control+End');
  for(let i=0;i<30;i++){await page.keyboard.press('Enter');await page.keyboard.type('interface Loopback'+i);}
  await frames(page);
  const after=await mapInfo(page);
  assert(after.layout.g.total>total,'the map grows with the document');
  assert.notEqual(await canvasSum(page),before,'the map redraws after typing');
  await assertBoxMatches(page,'after typing');
 }
 // Word wrap, then a larger font: still the visible share.
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][3]));await frames(page);
 const unwrapped=(await mapInfo(page)).layout.g.total;
 await page.locator('#networkToolkitFileEditor [data-ed-wrap]').check();await frames(page);
 {const ta=await activeArea(page);assert.equal(ta.wrap,'soft');const wrapped=(await mapInfo(page)).layout;assert(wrapped.g.total>unwrapped*1.5,'with Wrap on the map covers the wrapped rows');assert.match(wrapped.key,/\|true\|/,'the map knows the text wraps');}
 for(const fraction of [0,0.5,1])await setActiveScroll(page,fraction),await assertBoxMatches(page,'wrapped '+fraction);
 const wrappedSum=await canvasSum(page);
 await page.locator('#networkToolkitFileEditor [data-ed-font]').fill('16');await page.locator('#networkToolkitFileEditor [data-ed-font]').press('Enter');await frames(page);
 await assertBoxMatches(page,'font 16');
 assert.notEqual(await canvasSum(page),wrappedSum,'the map redraws for a new font size');
 await page.locator('#networkToolkitFileEditor [data-ed-font]').fill('13');await page.locator('#networkToolkitFileEditor [data-ed-font]').press('Enter');
 await page.locator('#networkToolkitFileEditor [data-ed-wrap]').uncheck();await frames(page);
 await assertBoxMatches(page,'unwrapped again');
 pass('Document map: follows typing, word wrap and the font size');

 // Light, dark and a Custom palette: the map uses the editor's colours.
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][0]));await frames(page);
 const bottom=async()=>{const i=await mapInfo(page);return canvasPixel(page,i.width-2,i.height-2);};
 const background=()=>page.evaluate(()=>getComputedStyle(document.querySelector('#networkToolkitFileEditor .nt-editor-code')).backgroundColor.match(/\d+/g).slice(0,3).map(Number));
 const light=await bottom();assert.deepEqual(light,await background(),'light: the map has the editor background');
 await shot(page,'editor-map-list-light');
 await page.evaluate(()=>setTheme('dark'));await frames(page);
 const dark=await bottom();assert.deepEqual(dark,await background(),'dark: the map has the editor background');
 assert(dark.reduce((a,b)=>a+b)<light.reduce((a,b)=>a+b),'the dark map is darker');
 await shot(page,'editor-map-list-dark');
 await page.locator('#networkToolkitFileEditor [data-ed-palette]').selectOption('custom');
 await page.evaluate(()=>{const input=document.querySelector('#networkToolkitFileEditor [data-ed-color-hex="bg"]');input.value='#123456';input.dispatchEvent(new Event('change',{bubbles:true}));});await frames(page);
 assert.deepEqual(await bottom(),[0x12,0x34,0x56],'Custom: the map has the custom background');
 await page.locator('#networkToolkitFileEditor [data-ed-palette]').selectOption('auto');await frames(page);
 assert.deepEqual(await bottom(),dark,'back to Follow app');
 pass('Document map: follows the light and dark themes and the Custom palette');

 // ---- Document List ----
 const rail=page.locator('#networkToolkitFileEditor [data-editor-files-panel="documents"]');
 assert.deepEqual(await page.locator('#networkToolkitFileEditor [data-editor-files-panel]').evaluateAll(list=>list.map(el=>[el.dataset.editorFilesPanel,el.textContent,el.getAttribute('aria-label')])),[['connections','Connect','Connections'],['files','Files','Files'],['documents','Docs','Documents']],'Docs is the third item of the editor rail');
 await rail.click();await frames(page);
 assert.equal(await page.locator('#networkToolkitFileEditor [data-editor-documents]').isVisible(),true,'the rail opens the document list');
 assert.equal(await page.locator('#networkToolkitFileEditor [data-editor-file-browser]').isVisible(),false,'the list replaces the file browser in the sidebar');
 assert.equal(await page.evaluate(()=>document.querySelector('#ntEditorMoreActions [data-ed-document-list]').checked),true,'the More menu shows the list is open');
 let rows=await listRows(page);
 assert.deepEqual(rows.map(row=>row.uid),await tabOrder(page),'every tab, in tab order');
 assert.deepEqual(rows.map(row=>row.name+'|'+row.ext),['core-router|cfg','notes|txt','inventory|json','long-lines|txt']);
 assert.deepEqual(rows.map(row=>row.mark),['●','','',''],'the unsaved marker of the edited document');
 assert.deepEqual(rows.map(row=>row.active),[true,false,false,false],'the active document is highlighted');
 assert.match(rows[0].label,/^core-router\.cfg, unsaved changes$/);
 assert.equal(await page.locator('#networkToolkitFileEditor [data-ed-doc-uid].is-active').evaluate(el=>getComputedStyle(el).backgroundColor!==getComputedStyle(el.parentElement).backgroundColor),true,'the active row has its own background');
 // Click, then Up/Down and Enter.
 await page.locator('#networkToolkitFileEditor [data-ed-doc-uid]').nth(2).click();
 assert.equal(await activeUID(page),rows[2].uid,'a click activates that tab');
 assert.equal((await listRows(page))[2].active,true);
 await page.keyboard.press('ArrowUp');assert.equal(await activeUID(page),rows[2].uid,'Up only moves through the rows');
 await page.keyboard.press('Enter');assert.equal(await activeUID(page),rows[1].uid,'Enter activates the row');
 await page.keyboard.press('End');await page.keyboard.press(' ');assert.equal(await activeUID(page),rows[3].uid,'End and Space');
 await page.keyboard.press('Home');await page.keyboard.press('ArrowDown');await page.keyboard.press('Enter');assert.equal(await activeUID(page),rows[1].uid,'Home, Down and Enter');
 assert.equal((await listRows(page)).find(row=>row.focused)?.uid,rows[1].uid,'the focus stays on the row');
 // The right-click menu is the tab strip's menu; Escape returns to the row.
 await page.locator('#networkToolkitFileEditor [data-ed-doc-uid]').nth(2).click({button:'right'});
 const menu=page.locator('.nt-editor-tab-menu[role="menu"]');await menu.waitFor();
 assert.equal(await activeUID(page),rows[2].uid,'the menu acts on that document');
 assert.deepEqual(await menu.locator('[role="menuitem"]').evaluateAll(list=>list.map(el=>el.dataset.edContextAction)),['save','saveAs','reload','compareTo','move','first','last','stepLeft','stepRight','detach','copyName','copyPath','close','others','following']);
 await shot(page,'editor-document-list-menu');
 await page.keyboard.press('Escape');await menu.waitFor({state:'detached'});
 assert.equal((await listRows(page)).find(row=>row.focused)?.uid,rows[2].uid,'Escape returns the focus to the row');
 await page.keyboard.press('Shift+F10');await menu.waitFor();await page.keyboard.press('Escape');
 // The menu's actions work from the list: move a tab one place left.
 await page.locator('#networkToolkitFileEditor [data-ed-doc-uid]').nth(2).click({button:'right'});await menu.locator('[data-ed-context-action="stepLeft"]').click();await frames(page);
 rows=await listRows(page);assert.deepEqual(rows.map(row=>row.uid),await tabOrder(page),'the list follows a tab moved from its menu');
 assert.deepEqual(rows.map(row=>row.name),['core-router','inventory','notes','long-lines']);
 pass('Document List: every tab in tab order with extension and unsaved marker; click, keys and the tab menu');

 // Split view: groups, and the map follows the active pane.
 await page.locator('#networkToolkitFileEditor [data-ed="split"]').click();await frames(page);
 await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor.is-split'));
 rows=await listRows(page);
 const groups=await page.evaluate(()=>NetworkToolkitFileEditor.viewGroups);
 const sideOf=uid=>groups.groups.find(g=>g.uid===uid)?.side;
 assert.deepEqual([...new Set(rows.map(row=>row.group))],['Left view','Right view'],'split view shows the view groups');
 for(const row of rows)assert.equal(row.group,sideOf(row.uid)==='right'?'Right view':'Left view',row.name+' is listed in its view');
 assert.equal(await page.locator('#networkToolkitFileEditor .nt-editor-documents-group').count(),2);
 const right=rows.find(row=>row.group==='Right view');
 await page.locator('#networkToolkitFileEditor [data-ed-doc-uid="'+right.uid+'"]').click();await frames(page);
 assert.equal((await activeArea(page)).side,'right','a row of the right view activates the right pane');
 for(const fraction of [0,0.7])await setActiveScroll(page,fraction),await assertBoxMatches(page,'right pane '+fraction);
 const left=rows.find(row=>row.group==='Left view'&&row.uid===rows[0].uid);
 await page.locator('#networkToolkitFileEditor [data-ed-doc-uid="'+left.uid+'"]').click();await frames(page);
 assert.equal((await activeArea(page)).side,'left');
 await setActiveScroll(page,0.4);await assertBoxMatches(page,'left pane');
 await shot(page,'editor-map-list-split-dark');
 // The comparison view has its own change map.
 await page.locator('#networkToolkitFileEditor [data-ed="compare"]').click();await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor.is-comparing'));await frames(page);
 assert.equal((await mapInfo(page)).hidden,true,'the map is hidden in the comparison view');
 await page.locator('#networkToolkitFileEditor [data-ed="compare"]').click();await page.waitForFunction(()=>!document.querySelector('#networkToolkitFileEditor.is-comparing'));await frames(page);
 assert.equal((await mapInfo(page)).hidden,false,'and shown again after it');
 await page.locator('#networkToolkitFileEditor [data-ed="split"]').click();await page.waitForFunction(()=>!document.querySelector('#networkToolkitFileEditor.is-split'));await frames(page);
 assert.equal((await listRows(page)).some(row=>row.group),false,'one view: no groups');
 pass('Split view: the list shows the left and right views and the map follows the active pane; hidden in the comparison view');

 // Command palette and F1 help.
 await paletteRun(page,'Show or hide the document map');assert.equal((await mapInfo(page)).hidden,true,'the palette hides the map');
 await paletteRun(page,'Show or hide the document map');assert.equal((await mapInfo(page)).hidden,false,'and shows it');
 await page.locator('#networkToolkitFileEditor [data-editor-files-panel="files"]').click();
 await paletteRun(page,'Show the document list');
 assert.equal(await page.locator('#networkToolkitFileEditor [data-editor-documents]').isVisible(),true,'the palette opens the list');
 assert.equal((await listRows(page)).find(row=>row.focused)?.active,true,'with the keyboard on the active row');
 await page.keyboard.press('Escape');assert.equal(await page.evaluate(()=>document.activeElement?.matches('[data-ed-text]')),true,'Escape returns to the text');
 await page.keyboard.press('F1');await page.locator('[aria-label="Keyboard shortcuts and help"][role="dialog"]').waitFor();
 const help=await page.locator('#uxCommandOverlay .ux-help-row').allInnerTexts();
 assert(help.some(row=>/Document map/.test(row)),'F1 help explains the document map');assert(help.some(row=>/List the open documents/.test(row)),'and the document list');
 await page.keyboard.press('Escape');
 pass('Command palette and F1 help: Document map and Document list');

 // A reload keeps both.
 await page.waitForTimeout(400);
 for(let i=0;i<40&&state.saved.editorFilesPreferences?.panel!=='documents';i++)await page.waitForTimeout(50);
 assert.equal(state.saved.editorFilesPreferences?.panel,'documents','the sidebar remembers the Document List');
 await start();
 for(const file of FILES.keys())assert.equal(await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file).then(()=>'',error=>error.message),file),'');
 await frames(page);
 assert.equal((await mapInfo(page)).hidden,false,'the map is still on after a reload');
 await openMore(page);assert.equal(await mapBox.isChecked(),true);assert.equal(await listBox.isChecked(),true);await page.keyboard.press('Escape');
 assert.equal(await page.locator('#networkToolkitFileEditor [data-editor-documents]').isVisible(),true,'the Document List is still open after a reload');
 assert.equal((await listRows(page)).length,4);
 // Turning the list off from the More menu closes the sidebar.
 await openMore(page);await listBox.uncheck();await frames(page);
 assert.equal(await page.locator('#networkToolkitFileEditor [data-ed-files-sidebar]').isVisible(),false,'unchecking Document list closes it');
 await openMore(page);await listBox.check();await frames(page);
 assert.equal(await page.locator('#networkToolkitFileEditor [data-editor-documents]').isVisible(),true);
 pass('Reload: the map and the Document List are restored');

 // A 16 MB page-held document: a down-sampled overview, bounded map work.
 await page.evaluate(()=>{const map=NetworkToolkitFileEditor.documentMap;Object.assign(map.stats,{updates:0,draws:0,maxUpdate:0,maxDraw:0});});
 const large=await page.evaluate(async()=>{
  const line=i=>'interface GigabitEthernet0/'+(i%48)+' description synthetic uplink to 192.0.2.'+(i%250)+(i%9===0?' ! checked':'')+'\n';
  const parts=[];let size=0;for(let i=0;size<16*1024*1024;i++){const l=line(i);parts.push(l);size+=l.length;}
  const text=parts.join('').slice(0,16*1024*1024);
  await NetworkToolkitFileEditor.open({id:'synthetic-16mb',path:'synthetic-16mb.cfg',source:'Local computer',binding:{kind:'local'},create:true,initialContent:text,read:async()=>({content:text}),write:async()=>({hash:'a'.repeat(64)})});
  return text.length;
 });
 assert.equal(large,16*1024*1024);
 await frames(page);
 {
  const i=await mapInfo(page);
  assert.equal(i.doc.name,'synthetic-16mb.cfg');assert.equal(i.layout.kind,'text');
  const ta=await activeArea(page),lines=await page.evaluate(()=>NetworkToolkitFileEditor.active.content.split('\n').length);
  assert(lines>i.layout.g.used*8,'more lines than map pixels times eight: the overview samples pixel rows ('+lines+' lines)');
  assert.equal(i.layout.g.used,i.height,'the whole document is fitted to the map');
  for(const fraction of [0.5,1])await setActiveScroll(page,fraction),await assertBoxMatches(page,'16 MB '+fraction);
  void ta;
  const x=i.left+i.width/2;
  await page.mouse.click(x,i.top+i.height*0.25);await frames(page);
  await page.mouse.move(x,i.top+(await mapInfo(page)).box.top+5);await page.mouse.down();for(let step=1;step<=8;step++)await page.mouse.move(x,i.top+i.height*0.25+step*20);await page.mouse.up();
  await page.mouse.wheel(0,600);await frames(page);
  await text.focus();await page.keyboard.press('Control+Home');for(let k=0;k<6;k++)await page.keyboard.type('x');
  await page.waitForTimeout(500);await frames(page);
  const stats=(await mapInfo(page)).stats;
  assert(stats.updates>10,'the map followed the scrolling');
  assert(stats.maxUpdate<50&&stats.maxDraw<50,'the map work stays under 50 ms on a 16 MB document: '+JSON.stringify(stats));
  report.large16MiB={pageHeld:true,lines,stats};
 }
 pass('16 MB page-held document: down-sampled overview, map work '+JSON.stringify(report.large16MiB.stats));
 await page.evaluate(()=>NetworkToolkitFileEditor.active.dirty=false);
 await context.close();
}

// A detached editor window at its native minimum size, 640 x 400.
async function detachedWindow(browser){
 const view={id:'c'.repeat(32),ownerInstanceId:'e'.repeat(32)};
 const state={saved:{appUpdatePreferences:{enabled:false},theme:'dark'},posts:0};
 const context=await browser.newContext({viewport:{width:640,height:400},deviceScaleFactor:1}),page=await context.newPage();
 await serve(page,{mode:'editor',view,state});
 await page.goto('https://toolkit.local/editor-view/'+view.id);
 await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online&&EDITOR_WINDOW===true);
 for(const file of ['C:\\lab\\core-router.cfg','C:\\lab\\notes.txt'])assert.equal(await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file).then(()=>'',error=>error.message),file),'');
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][0]));
 await openMore(page);await page.locator('#ntEditorMoreActions [data-ed-document-map]').check();await frames(page);
 const info=await mapInfo(page);
 assert.equal(info.hidden,false,'the map is shown in a detached window');
 assert(info.width<=64,'a narrow window has a narrower map: '+info.width);
 await assertBoxMatches(page,'detached');
 const geometry=()=>page.evaluate(()=>{const ta=document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');return {overflow:document.documentElement.scrollWidth-innerWidth,text:ta.clientWidth,textHeight:ta.clientHeight};});
 let g=await geometry();
 assert(g.overflow<=0,'no horizontal page scroll at 640 px');assert(g.text>=360,'the text keeps most of the width: '+g.text);assert(g.textHeight>=120,'and a usable height: '+g.textHeight);
 await page.mouse.click(info.left+info.width/2,info.top+info.layout.g.used*0.7);await frames(page);
 assert((await activeArea(page)).scrollTop>0,'a click on the map scrolls in the small window');
 await shot(page,'editor-map-list-detached-640x400-dark');
 // The Document List is the rail's third item; the narrow sidebar covers the
 // text and closes when a document is chosen.
 await page.locator('#networkToolkitFileEditor [data-editor-files-panel="documents"]').click();await frames(page);
 assert.equal(await page.locator('#networkToolkitFileEditor [data-editor-documents]').isVisible(),true,'the list opens in the detached window');
 const rows=await listRows(page);assert.deepEqual(rows.map(row=>row.name),['core-router','notes']);
 await shot(page,'editor-map-list-detached-list-640x400-dark');
 await page.locator('#networkToolkitFileEditor [data-ed-doc-uid]').nth(1).click();await frames(page);
 assert.equal(await activeUID(page),rows[1].uid,'a click activates the tab');
 assert.equal(await page.locator('#networkToolkitFileEditor [data-ed-files-sidebar]').isVisible(),false,'the narrow sidebar closes after the choice');
 assert.equal(await page.evaluate(()=>document.activeElement?.matches('[data-ed-text]')),true,'and the text has the focus');
 g=await geometry();assert(g.overflow<=0);
 assert.equal(JSON.parse(await page.evaluate(()=>sessionStorage.getItem('ntk.editor.files'))).panel,'documents','the detached window keeps its sidebar choice like Files');
 pass('Detached window at 640 x 400: the map and the Document List stay usable');
 await context.close();
}

// ---- Part 2: the standalone Text Editor against its production engine ----
async function engine(browser){
 const context=await browser.newContext({viewport:{width:1280,height:820},deviceScaleFactor:1}),page=await context.newPage();
 page.on('pageerror',error=>report.errors.push('engine: '+error.message));
 await page.addInitScript(value=>{window.__networkToolkitNativeSession=value;},engineToken);
 const start=async()=>{await page.goto(engineURL);await page.waitForFunction(()=>window.NetworkToolkitEditorWindows?.bufferSupported()&&window.__networkToolkitStartupState===1);await page.waitForFunction(()=>['ready','off'].includes(NetworkToolkitFileEditor.session.phase));};
 await start();
 assert.equal(await page.evaluate(()=>EDITOR_STANDALONE),true);
 const settled=()=>page.waitForFunction(()=>{const d=NetworkToolkitFileEditor.active;return d&&!d.loading&&!d.diskOpening&&!d.diskSession?.pending;});
 const open=async name=>{assert.equal(await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file,false,true).then(()=>'',error=>error.message),path.join(engineFiles,name)),'');await settled();};
 const diskState=()=>page.evaluate(()=>{const view=NetworkToolkitFileEditor.documentMap&&document.querySelector('#networkToolkitFileEditor .nt-editor-disk-scroll');const d=NetworkToolkitFileEditor.active;return {disk:!!d.disk,lines:d.diskMeta?.lines,units:d.diskMeta?.units,value:Number(view?.getAttribute('aria-valuenow'))};});
 if(!await page.evaluate(()=>NetworkToolkitFileEditor.documentMapOn))await page.evaluate(()=>NetworkToolkitFileEditor.toggleDocumentMap());
 await page.evaluate(()=>NetworkToolkitEditorFiles.showDocuments(true));

 // 16 MB disk-backed: mapped by line, scrolled through the map, no long task.
 await open('large.cfg');
 let d=await diskState();assert.equal(d.disk,true,'a 16 MB file is disk-backed');
 await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.layout?.kind==='lines');
 let info=await mapInfo(page);
 assert.equal(info.layout.g.total,d.lines,'the map covers every line of the file');
 await page.evaluate(()=>{window.mapLongTasks=[];new PerformanceObserver(list=>mapLongTasks.push(...list.getEntries().map(e=>e.duration))).observe({type:'longtask'});});
 const editorArea=page.locator('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');
 const topLine=()=>page.evaluate(()=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l?l.g.box.top/l.g.scale:0;});
 const x=info.left+info.width/2;
 // Click at three places: the view centres there.
 for(const at of [0.5,0.9,0.1]){
  await page.mouse.click(x,info.top+info.layout.g.used*at);
  await page.waitForFunction(at=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l&&Math.abs((l.g.box.top+l.g.box.height/2)/l.g.used-at)<0.02;},at,{timeout:15000});
  const cursor=await page.evaluate(()=>Number(document.querySelector('#networkToolkitFileEditor [data-ed-gutter-lines] [data-source-line]')?.dataset.sourceLine));
  assert(Math.abs(cursor/d.lines-at)<0.03,'a click at '+at+' shows line '+cursor+' of '+d.lines);
 }
 // Drag the box down, then the wheel over the map and over the text.
 info=await mapInfo(page);
 await page.mouse.move(x,info.top+info.box.top+info.box.height/2);await page.mouse.down();
 for(let step=1;step<=12;step++){await page.mouse.move(x,info.top+info.box.top+info.box.height/2+step*15);await page.waitForTimeout(16);}
 await page.mouse.up();
 await page.waitForFunction(top=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l&&l.g.box.top>top+150;},info.box.top,{timeout:15000});
 const dragged=await topLine();
 await page.mouse.move(x,info.top+20);for(let i=0;i<20;i++){await page.mouse.wheel(0,400);await page.waitForTimeout(16);}
 await page.waitForFunction(line=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l&&l.g.box.top/l.g.scale>line;},dragged,{timeout:15000});
 await editorArea.hover();for(let i=0;i<20;i++){await page.mouse.wheel(0,-400);await page.waitForTimeout(16);}
 await frames(page);await page.waitForTimeout(300);
 const longTasks=await page.evaluate(()=>mapLongTasks);
 assert(longTasks.every(ms=>ms<=50),'no long task over 50 ms while scrolling a 16 MB disk-backed document with the map: '+JSON.stringify(longTasks));
 report.disk16MiB={lines:d.lines,longTasks,stats:(await mapInfo(page)).stats};
 await shot(page,'editor-map-list-disk-16mb-dark');
 pass('16 MB disk-backed document: mapped by line; clicks, a drag and the wheel move the view with no long task over 50 ms');

 // 1.7 MB on one line (a Base64 export) with Wrap on: mapped by offset.
 await open('export.b64');
 d=await diskState();assert.equal(d.disk,true);assert.equal(d.lines,1,'one line');
 if(!await page.locator('#networkToolkitFileEditor [data-ed-wrap]').isChecked())await page.locator('#networkToolkitFileEditor [data-ed-wrap]').check();
 await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.layout?.kind==='offsets');
 info=await mapInfo(page);
 assert.equal(info.layout.g.total,d.units,'the map covers every character of the line');
 assert(info.layout.g.used>info.height*0.9,'a 1.7 MB line fills the map');
 const offset=()=>page.evaluate(()=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l.g.box.top/l.g.scale;});
 await page.mouse.click(x,info.top+info.layout.g.used*0.5);
 await page.waitForFunction(()=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l&&Math.abs((l.g.box.top+l.g.box.height/2)/l.g.used-0.5)<0.02;},null,{timeout:15000});
 const middle=await page.evaluate(()=>{const ta=document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]'),view=NetworkToolkitFileEditor.active.diskSession.state().page;return {start:view.start,end:view.start+view.text.length,shown:ta.value.slice(0,16)};});
 assert(middle.start>d.units*0.4&&middle.end<d.units*0.65,'a click in the middle shows the middle of the line: '+JSON.stringify(middle));
 // Drag the box to the very end: the last characters are shown.
 info=await mapInfo(page);
 await page.mouse.move(x,info.top+info.box.top+info.box.height/2);await page.mouse.down();await page.mouse.move(x,info.top+info.height+40,{steps:12});await page.mouse.up();
 await page.waitForFunction(units=>{const s=NetworkToolkitFileEditor.active.diskSession.state().page;const ta=document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');return s.start+s.text.length>=units&&ta.scrollTop+ta.clientHeight>=ta.scrollHeight-2;},d.units,{timeout:15000});
 const tail=await page.evaluate(()=>document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]').value.slice(-32));
 const expectedTail=fs.readFileSync(path.join(engineFiles,'export.b64'),'utf8').slice(-32);
 assert.equal(tail,expectedTail,'dragging the box to the end shows the last characters');
 await page.waitForFunction(()=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l&&Math.abs(l.g.box.top+l.g.box.height-l.g.used)<=1;});
 // And back to the start.
 info=await mapInfo(page);
 await page.mouse.move(x,info.top+info.box.top+info.box.height/2);await page.mouse.down();await page.mouse.move(x,info.top-40,{steps:12});await page.mouse.up();
 await page.waitForFunction(()=>{const s=NetworkToolkitFileEditor.active.diskSession.state().page;const ta=document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');return s.start===0&&ta.scrollTop<2;},null,{timeout:15000});
 void offset;
 await shot(page,'editor-map-list-base64-wrap-dark');
 pass('1.7 MB single-line Base64 with Wrap on: mapped by character offset; the map reaches the middle, the end and the start');

 // The engine keeps the settings: reload.
 await page.waitForTimeout(600);
 await start();
 assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.documentMapOn),true,'the map setting is restored by the engine');
 assert.equal(await page.evaluate(()=>NetworkToolkitEditorFiles.panel),'documents','the Document List is restored by the engine');
 pass('Standalone Text Editor: the engine keeps the map and the Document List after a reload');
 await context.close();
}

(async()=>{
 fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 try{
  await mainWindow(browser);
  await detachedWindow(browser);
  if(engineURL)await engine(browser);
  assert.deepEqual(report.errors,[]);
  report.passed=true;
 }finally{
  await browser.close();
  fs.writeFileSync(path.join(output,'editor-map-list-results.json'),JSON.stringify(report,null,2));
 }
})().catch(error=>{console.error(error);console.error(JSON.stringify(report.errors));process.exitCode=1;});
