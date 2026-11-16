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
//  - the Document List on the rail (Docs, Connect, Files): every tab in tab
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
// `transfer` ({id, batch}) is a move from the main window waiting for this
// detached window, accepted as the engine's move routes accept it.
async function serve(page,{mode='main',view=null,state,transfer=null}){
 page.on('pageerror',error=>report.errors.push(mode+': '+error.message));
 await page.route('https://toolkit.local/**',async route=>{
  const request=route.request(),url=new URL(request.url()),p=url.pathname,method=request.method();
  if(p.startsWith('/api/')){
   const body=request.postData()?request.postDataJSON():null;let data={},status=200;
   if(p==='/api/status')data={version,os:'windows',arch:'amd64',windowMode:mode,editorWindowAvailable:!!view,editorDiskAvailable:false,editorSessionAvailable:false};
   else if(p==='/api/state'){if(method==='GET')data=structuredClone(state.saved);else{const {_sharedBase,...rest}=body||{};state.saved={...state.saved,...rest};state.posts++;data={ok:true};}}
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/terminals')data={sessions:[]};
   else if(p==='/api/editor-views'||p==='/api/editor-views/current')data={available:true,ownerInstanceId:'e'.repeat(32),self:view?.id||'',views:[],incoming:transfer&&!transfer.accepted?[{id:transfer.id,batch:transfer.batch}]:[],outgoing:[],owned:transfer?.accepted?transfer.leases:{},recovery:[]};
   else if(transfer&&p==='/api/editor-views/transfers/'+transfer.id+'/accept'){transfer.accepted=true;transfer.leases=Object.fromEntries(transfer.batch.documents.map(d=>[d.uid,1]));data={state:'accepted',leases:transfer.leases};}
   else if(transfer&&p==='/api/editor-views/transfers/'+transfer.id+'/complete')data={ok:true};
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
  const d=NetworkToolkitFileEditor.active,style=host&&getComputedStyle(host),rect=host?.getBoundingClientRect();
  return {hidden:!host||host.hidden||style.display==='none',width:rect?.width||0,height:host?.clientHeight||0,top:rect?.top||0,left:rect?.left||0,
   box:box&&!box.hidden?{top:parseFloat(box.style.top),height:parseFloat(box.style.height)}:null,
   layout:layout?{kind:layout.kind,mode:layout.mode,zoom:layout.zoom,start:layout.start,columns:layout.columns,g:layout.g,key:layout.key}:null,stats:{...map?.stats},
   doc:d?{uid:d.uid,name:d.path.split(/[\\/]/).pop(),disk:!!d.disk}:null};
 });
}
// The text area that shows the active document (single view or the active pane).
function activeArea(page){
 return page.evaluate(()=>{
  const ta=(()=>{const groups=NetworkToolkitFileEditor.viewGroups,split=document.querySelector('#networkToolkitFileEditor.is-split');if(!split)return document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');return document.querySelector('#networkToolkitFileEditor [data-compare-text="'+groups.activeSide+'"]');})();
  const style=getComputedStyle(ta);
  // The laid-out line pitch: Chrome snaps a fractional line height to layout
  // units, which adds up over a long document, so it is measured.
  // (Counted once per text, on the document's string: reading a 16 MB text
  // area's value copies it, and that garbage would land in the map's frames.)
  const value=NetworkToolkitFileEditor.active?.content??ta.value,memo=window.__mapTestLines;
  let lines=memo?.text===value?memo.lines:1;if(memo?.text!==value){for(let at=-1;(at=value.indexOf('\n',at+1))!==-1;)lines++;window.__mapTestLines={text:value,lines};}
  const paddingTop=parseFloat(style.paddingTop)||0,paddingBottom=parseFloat(style.paddingBottom)||0;
  return {scrollTop:ta.scrollTop,scrollHeight:ta.scrollHeight,clientHeight:ta.clientHeight,lineHeight:parseFloat(style.lineHeight),paddingTop,lines,pitch:(ta.scrollHeight-paddingTop-paddingBottom)/lines,wrap:ta.wrap,side:NetworkToolkitFileEditor.viewGroups.activeSide};
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
// Wait for a condition checked in Node (a saved fixture value), never a fixed pause.
async function until(check,label,timeout=5000){const end=Date.now()+timeout;while(!(await check())){if(Date.now()>end)throw new Error('Timed out: '+label);await new Promise(resolve=>setTimeout(resolve,25));}}

// ---- Width and text mode ----
const SEP='#networkToolkitFileEditor [data-ed-map-resize]';
// The map's width and limits, its separator, its mode, and the text area.
function mapSize(page){
 return page.evaluate(()=>{
  const editor=document.querySelector('#networkToolkitFileEditor'),host=editor.querySelector('[data-ed-map]'),sep=editor.querySelector('[data-ed-map-resize]'),map=NetworkToolkitFileEditor.documentMap,l=map.layout;
  const box=el=>{const r=el.getBoundingClientRect();return {left:r.left,right:r.right,width:r.width,top:r.top,height:r.height};};
  const ta=editor.classList.contains('is-split')?editor.querySelector('[data-compare-text="'+NetworkToolkitFileEditor.viewGroups.activeSide+'"]'):editor.querySelector('.nt-editor-code [data-ed-text]');
  const style=getComputedStyle(ta),ctx=document.createElement('canvas').getContext('2d');ctx.font=style.font;
  return {width:host.hidden?0:host.getBoundingClientRect().width,hidden:host.hidden,map:box(host),
   sep:{...box(sep),hidden:sep.hidden,role:sep.getAttribute('role'),orientation:sep.getAttribute('aria-orientation'),label:sep.getAttribute('aria-label'),min:Number(sep.getAttribute('aria-valuemin')),max:Number(sep.getAttribute('aria-valuemax')),now:Number(sep.getAttribute('aria-valuenow')),tabindex:sep.tabIndex,controls:sep.getAttribute('aria-controls')},
   controls:host.id,limits:map.limits(),mode:l?.mode||'',font:l?.zoom?.font||0,mapColumns:l?.zoom?.columns||0,columns:l?.columns||0,
   // What text mode should be for this width: the module's rule over the text area's own metrics.
   expected:NTKEditorDocumentMap.textZoom({width:host.getBoundingClientRect().width,columns:l?.columns||0,charWidth:ctx.measureText('0').width,fontSize:parseFloat(style.fontSize)}),
   pref:state.editorPreferences?.documentMapWidth,text:box(ta),textClient:ta.clientWidth,scrollTop:ta.scrollTop};
 });
}
async function dragSeparator(page,dx,steps=8){
 const b=await page.locator(SEP).boundingBox(),x=b.x+b.width/2,y=b.y+Math.min(80,b.height/2);
 await page.mouse.move(x,y);await page.mouse.down();await page.mouse.move(x+dx,y,{steps});await page.mouse.up();await frames(page);
}
async function pressSeparator(page,key){await page.locator(SEP).focus();await page.keyboard.press(key);await frames(page);}
const textRows=page=>page.evaluate(()=>NetworkToolkitFileEditor.documentMap.textRows.map(row=>({...row})));
// A row as the map draws it (rowCells): tabs to the next stop from the row
// start, carriage returns as nothing, other control characters as a blank,
// cut at `columns`.
function shownRow(piece,columns,tab=4){
 let out='';
 for(const char of piece){
  if(out.length>=columns)break;
  if(char==='\r'||char==='\n')continue;
  if(char==='\t'){out+=' '.repeat(Math.min(columns,out.length+tab-(out.length%tab))-out.length);continue;}
  out+=char.charCodeAt(0)<32||char.charCodeAt(0)===127?' ':char;
 }
 return out;
}
// Every row text mode drew is the document's own text at that row: a whole
// line without Wrap, a piece of `wrap` characters of it with Wrap on.
function assertRows(rows,text,{columns,wrap=0,label}){
 let lines=1;for(let at=-1;lines<6&&(at=text.indexOf('\n',at+1))!==-1;)lines++;
 assert(rows.length>=Math.min(5,lines),label+': the map drew text rows ('+rows.length+')');
 for(const row of rows){
  const end=text.indexOf('\n',row.offset),line=text.slice(row.offset,end<0?text.length:end),piece=wrap?line.slice(0,wrap):line;
  assert.equal(row.text,shownRow(piece,columns),label+': row '+row.line+'.'+row.row+' at '+row.offset);
 }
}
// The rows from offsets: the first row of each line starts after a line break.
function assertLineStarts(rows,text,label){
 for(const row of rows.filter(row=>row.row===0))assert(row.offset===0||text[row.offset-1]==='\n',label+': line '+row.line+' starts at '+row.offset);
}
// In text mode the box is a thumb over the map and the row at its top is the
// first line on screen; returns what it read.
async function assertTextBox(page,label){
 const [info,ta,rows]=await Promise.all([mapInfo(page),activeArea(page),textRows(page)]);
 assert.equal(info.layout.mode,'text',label+': text mode');
 const g=info.layout.g,natural=ta.scrollHeight*g.scale,size=Math.min(g.used,ta.clientHeight*g.scale);
 assert(Math.abs(info.box.height-size)<=1,label+': the box is the view at the text scale ('+info.box.height+' / '+size+')');
 const share=Math.max(0,Math.min(1,ta.scrollTop/Math.max(1,ta.scrollHeight-ta.clientHeight)));
 const top=natural>info.height?share*(g.used-size):ta.scrollTop*g.scale;
 assert(Math.abs(info.box.top-top)<=1,label+': box top '+info.box.top+' should be '+top);
 // The text area's top padding is drawn too, so the first row starts below the box top by its scaled height.
 const height=g.scale*ta.pitch,probe=info.box.top+ta.paddingTop*g.scale+0.5,at=rows.find(row=>row.y<=probe&&row.y+height>probe);
 assert(at,label+': a text row lies at the box top');
 if(ta.wrap!=='soft'){const first=Math.floor(Math.max(0,ta.scrollTop-ta.paddingTop)/ta.pitch);assert(Math.abs(at.line-first)<=1,label+': the row at the box top is line '+at.line+', the text area shows line '+first+' first');}
 return {info,ta,rows,height};
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
 await page.locator('#networkToolkitFileEditor [data-ed-custom-palette] [data-ed="colors"]').click();await frames(page);
 pass('Document map: follows the light and dark themes and the Custom palette');

 // ---- Document List ----
 const rail=page.locator('#networkToolkitFileEditor [data-editor-files-panel="documents"]');
 assert.deepEqual(await page.locator('#networkToolkitFileEditor [data-editor-files-panel]').evaluateAll(list=>list.map(el=>[el.dataset.editorFilesPanel,el.textContent,el.getAttribute('aria-label')])),[['documents','Docs','Documents'],['connections','Connect','Connections'],['files','Files','Files']],'Docs is the first item of the editor rail');
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
 await shot(page,'editor-map-and-list-dark');await page.evaluate(()=>setTheme('light'));await frames(page);await shot(page,'editor-map-and-list-light');await page.evaluate(()=>setTheme('dark'));await frames(page);
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

 // ---- Resizing (owner request after v1.21.58) ----
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][0]));await frames(page);
 const savedWidth=()=>state.saved.editorPreferences?.documentMapWidth;
 const kept=async(width,label)=>{await page.waitForFunction(width=>state.editorPreferences?.documentMapWidth===width,width);await until(()=>savedWidth()===width,label);};
 let size=await mapSize(page);
 assert.deepEqual([size.sep.role,size.sep.orientation,size.sep.label,size.sep.tabindex,size.sep.controls],['separator','vertical','Resize document map',0,size.controls],'a keyboard-reachable separator that names the map');
 assert(Math.abs(size.sep.right-size.map.left)<=1&&size.sep.width>=4,'the separator is the map\'s inner edge');
 assert(size.text.right<=size.sep.left+1,'the text area ends at the separator');
 assert.deepEqual([size.width,size.sep.now,size.sep.min,size.sep.max],[96,96,96,size.limits.max],'the default width, with its limits on the separator');
 assert(size.limits.max>=300&&size.limits.max<=640,'up to half of the space beside the document: '+size.limits.max);
 assert.equal(size.pref,undefined,'nothing saved while the map has its default width');
 assert.equal(size.mode,'blocks','the default width draws blocks');
 // Dragging the separator: the map follows the pointer, the text gives way, nothing scrolls.
 await setActiveScroll(page,0.3);
 const before=await activeArea(page),beforeSum=await canvasSum(page);
 // While the pointer is down only the map moves: it previews its width over
 // the text, and the text area keeps its width until the release.
 {
  const b=await page.locator(SEP).boundingBox(),x=b.x+b.width/2,y=b.y+80;
  await page.mouse.move(x,y);await page.mouse.down();await page.mouse.move(x-150,y,{steps:6});await frames(page);
  const during=await mapSize(page);
  assert.equal(during.sep.now,246,'the separator reports the width being dragged');
  assert(Math.abs(during.sep.left-(size.sep.left-150))<=1,'the separator follows the pointer');
  assert.deepEqual([during.text.left,during.text.width],[size.text.left,size.text.width],'the text area is not laid out again while dragging');
  assert.equal(during.width,96,'the map keeps its place in the layout until the release');
  assert.equal(await page.evaluate(()=>{const inner=document.querySelector('#networkToolkitFileEditor .nt-editor-map-inner');return inner.getBoundingClientRect().width-parseFloat(inner.style.clipPath.match(/inset\(0(?:px)? 0(?:px)? 0(?:px)? ([\d.]+)px\)/)[1]);}),246,'its drawing shows the new width');
  await page.mouse.up();await frames(page);
 }
 await dragSeparator(page,0);
 size=await mapSize(page);
 assert(Math.abs(size.width-246)<=1,'dragging the separator 150 px to the left widens the map to 246 px: '+size.width);
 assert.equal(size.sep.now,Math.round(size.width));
 assert(size.text.right<=size.sep.left+1,'the text area gives way');
 assert.equal((await activeArea(page)).scrollTop,before.scrollTop,'resizing does not scroll the text');
 assert.notEqual(await canvasSum(page),beforeSum,'the map is drawn at its new width');
 await kept(Math.round(size.width),'the width is saved with the editor preferences');
 // The limits: half of the space beside the document, never narrower than the default.
 await dragSeparator(page,-3000);size=await mapSize(page);
 assert.equal(size.width,size.limits.max,'dragging far left stops at the largest width');
 assert(size.text.width+80>=size.width,'the text area keeps about half: text '+size.text.width+', map '+size.width);
 await dragSeparator(page,3000);size=await mapSize(page);
 assert.equal(size.width,96,'dragging far right stops at the default width');
 await kept(undefined,'the default width is not saved as a number');
 // Keys on the separator: Left widens, Right narrows, Shift for larger steps, Home / End use the limits, Enter resets.
 await pressSeparator(page,'ArrowLeft');assert.equal((await mapSize(page)).width,112);
 await pressSeparator(page,'Shift+ArrowLeft');assert.equal((await mapSize(page)).width,176);
 await pressSeparator(page,'ArrowRight');size=await mapSize(page);assert.equal(size.width,160);assert.equal(size.sep.now,160);
 await kept(160,'a key press saves the width');
 await pressSeparator(page,'End');size=await mapSize(page);assert.equal(size.width,size.limits.max,'End: the largest width');
 await pressSeparator(page,'Home');assert.equal((await mapSize(page)).width,96,'Home: the smallest');
 await pressSeparator(page,'Shift+ArrowLeft');await pressSeparator(page,'Enter');assert.equal((await mapSize(page)).width,96,'Enter resets');
 assert.equal(await page.evaluate(()=>document.activeElement?.matches('[data-ed-map-resize]')),true,'the keys keep the focus on the separator');
 // A double-click resets.
 await dragSeparator(page,-200);assert.equal(Math.round((await mapSize(page)).width),296);
 await page.locator(SEP).dblclick();await frames(page);
 assert.equal((await mapSize(page)).width,96,'a double-click resets the default width');await kept(undefined,'and forgets the saved width');
 pass('Resizing: the separator drags (the text never scrolls), stops at the default and at half of the space, takes Left/Right/Home/End/Enter, a double-click resets, and the width is saved');

 // ---- Text mode: the threshold ----
 await setActiveScroll(page,0);
 const steps=[];
 await page.locator(SEP).focus();
 for(;;){
  size=await mapSize(page);steps.push({width:size.width,mode:size.mode,font:size.font,expected:size.expected?.font||0});
  assert.equal(size.mode,size.expected?'text':'blocks','at '+size.width+' px the map shows '+(size.expected?'text ('+size.expected.font+' px)':'blocks'));
  if(size.width>=size.limits.max)break;
  await page.keyboard.press('ArrowLeft');await frames(page);
 }
 const firstText=steps.findIndex(step=>step.mode==='text');
 assert(firstText>0,'blocks at the default width, text once it is wide enough: '+JSON.stringify(steps.map(step=>step.width+':'+step.mode)));
 assert(steps.slice(firstText).every(step=>step.mode==='text'),'and text from then on');
 assert(steps[firstText].font>=6&&steps[firstText].font<=7.8,'text starts at a legible 6 px or more: '+steps[firstText].font);
 report.threshold={blocksUpTo:steps[firstText-1].width,textFrom:steps[firstText].width,font:steps[firstText].font,largest:steps.at(-1)};
 // And back: blocks again below the threshold.
 await page.keyboard.press('Home');await frames(page);assert.equal((await mapSize(page)).mode,'blocks','narrow again: blocks');
 for(let i=0;i<40&&(await mapSize(page)).mode!=='text';i++){await page.keyboard.press('ArrowLeft');await frames(page);}
 assert.equal((await mapSize(page)).mode,'text');assert.equal((await mapSize(page)).width,steps[firstText].width,'the switch happens at the same width both ways');
 pass('Text mode: blocks up to '+report.threshold.blocksUpTo+' px, the document\'s text from '+report.threshold.textFrom+' px ('+report.threshold.font+' px font), on its own as the width crosses it');

 // ---- Text mode: the real text, the box and the pointer ----
 await page.keyboard.press('End');await frames(page);
 let content=await page.evaluate(()=>NetworkToolkitFileEditor.active.content);
 size=await mapSize(page);
 assert(size.mapColumns>=40,'a wide map shows many columns: '+size.mapColumns);
 assertRows(await textRows(page),content,{columns:size.mapColumns,label:'top'});
 assert.equal((await textRows(page))[0].text,shownRow(content.split('\n')[0],size.mapColumns),'the first row is the first line');
 // The box tracks the scroll; the row at its top is the first line on screen.
 let read;
 for(const fraction of [0,0.25,0.5,0.75,1]){await setActiveScroll(page,fraction);read=await assertTextBox(page,'text '+fraction);assertRows(read.rows,content,{columns:size.mapColumns,label:'text '+fraction});}
 assert(read.ta.scrollHeight*read.info.layout.g.scale>read.info.height*2,'a long document scrolls inside the map');
 // A click centres the view on the line under the pointer.
 await setActiveScroll(page,0.2);
 for(const pick of [g=>g.used-15,()=>8]){
  const {info,rows,height}=await assertTextBox(page,'before a click'),y=pick(info.layout.g),row=rows.find(r=>r.y<=y&&r.y+height>y);
  assert(y<info.box.top||y>info.box.top+info.box.height,'the click is outside the box');
  await page.mouse.click(info.left+info.width/2,info.top+y);await frames(page);
  const ta=await activeArea(page),middle=Math.floor((ta.scrollTop+ta.clientHeight/2-ta.paddingTop)/ta.pitch);
  assert(Math.abs(middle-row.line)<=1,'a click on line '+row.line+' shows it in the middle ('+middle+')');
  await assertTextBox(page,'after a click');
 }
 // Dragging the box: it follows the pointer, the text follows the box.
 {
  const {info}=await assertTextBox(page,'before a drag'),start=await activeArea(page),x=info.left+info.width/2,y=info.top+info.box.top+info.box.height/2;
  await page.mouse.move(x,y);await page.mouse.down();for(let step=1;step<=5;step++)await page.mouse.move(x,y+step*8);await page.mouse.up();await frames(page);
  const after=await activeArea(page),moved=after.scrollTop-start.scrollTop,expected=40/info.layout.g.drag;
  assert(Math.abs(moved-expected)<=2/info.layout.g.drag,'dragging the box 40 px scrolls '+expected.toFixed(1)+' px ('+moved+')');
  const read=await assertTextBox(page,'after a drag');assert(Math.abs(read.info.box.top-info.box.top-40)<=1.5,'and the box moved with the pointer');
 }
 // The wheel over the map scrolls the document.
 {
  const {info}=await assertTextBox(page,'before the wheel'),start=await activeArea(page);
  await page.mouse.move(info.left+info.width/2,info.top+20);await page.mouse.wheel(0,300);await frames(page);
  assert(Math.abs((await activeArea(page)).scrollTop-start.scrollTop-300)<=1,'the wheel over the text map scrolls 300 px');
  await assertTextBox(page,'after the wheel');
 }
 // Typing: the rows follow the edit.
 await text.focus();await page.keyboard.press('Control+End');await page.keyboard.type('interface Loopback99');await frames(page);
 content=await page.evaluate(()=>NetworkToolkitFileEditor.active.content);
 read=await assertTextBox(page,'after typing');assertRows(read.rows,content,{columns:size.mapColumns,label:'after typing'});
 assert.equal(read.rows.at(-1).text,'interface Loopback99','the new line is drawn');
 pass('Text mode: the rows are the document\'s lines; the box tracks the scroll with the first line on screen at its top; a click centres its line, the box drags, the wheel scrolls, typing redraws');

 // The editor's own syntax colours, light and dark, on the editor background.
 const colours=()=>page.evaluate(()=>{
  const c=document.querySelector('#networkToolkitFileEditor [data-ed-map] canvas'),r=devicePixelRatio||1,x=c.getContext('2d').getImageData(0,0,c.width,Math.min(c.height,Math.round(160*r))).data,set=new Set();
  for(let i=0;i<x.length;i+=4)set.add(x[i]+','+x[i+1]+','+x[i+2]);return set.size;
 });
 await setActiveScroll(page,0);
 for(const theme of ['light','dark']){
  await page.evaluate(theme=>setTheme(theme),theme);await frames(page);
  const i=await mapInfo(page);assert.deepEqual(await canvasPixel(page,i.width-2,i.height-2),await background(),theme+': text mode is drawn on the editor background');
  assert(await colours()>12,theme+': the text is drawn in several colours (syntax and anti-aliasing)');
  await shot(page,'editor-map-text-'+theme);
 }
 assert.equal(await page.evaluate(()=>[...NetworkToolkitFileEditor.documentMap.textRows].length>0&&!!NetworkToolkitFileEditor.active.syntaxCache),true,'the editor has painted its syntax colours for the map to use');
 await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.layout?.colored===true);
 // A Custom palette: text mode is drawn on its background too.
 await page.locator('#networkToolkitFileEditor [data-ed-palette]').selectOption('custom');
 await page.evaluate(()=>{const input=document.querySelector('#networkToolkitFileEditor [data-ed-color-hex="bg"]');input.value='#123456';input.dispatchEvent(new Event('change',{bubbles:true}));});await frames(page);
 {const i=await mapInfo(page);assert.deepEqual(await canvasPixel(page,i.width-2,i.height-2),[0x12,0x34,0x56],'Custom: text mode has the custom background');}
 await page.locator('#networkToolkitFileEditor [data-ed-palette]').selectOption('auto');await frames(page);
 await page.locator('#networkToolkitFileEditor [data-ed-custom-palette] [data-ed="colors"]').click();await frames(page);
 pass('Text mode: drawn in the editor\'s syntax colours on its background, light, dark and a Custom palette');

 // Word wrap: rows are pieces of the text area's columns; resizing keeps the line at the top.
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][3]));await frames(page);
 await page.locator('#networkToolkitFileEditor [data-ed-wrap]').check();await frames(page);
 content=await page.evaluate(()=>NetworkToolkitFileEditor.active.content);
 for(const fraction of [0,0.5,1]){
  await setActiveScroll(page,fraction);const read=await assertTextBox(page,'wrapped '+fraction),columns=read.info.layout.columns;
  assert(read.rows.some(row=>row.row>0),'wrapped rows are drawn');
  assertLineStarts(read.rows,content,'wrapped '+fraction);
  for(const row of read.rows){const lineStart=content.lastIndexOf('\n',row.offset-1)+1;assert.equal((row.offset-lineStart)%columns,0,'a wrapped row starts on the wrap grid');assert.equal(row.text,shownRow(content.slice(row.offset,Math.min(row.offset+columns,content.indexOf('\n',row.offset)<0?content.length:content.indexOf('\n',row.offset))),(await mapSize(page)).mapColumns),'wrapped row '+row.line+'.'+row.row);}
 }
 const topLine=()=>page.evaluate(()=>{const ta=document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]'),rows=[...document.querySelectorAll('#networkToolkitFileEditor [data-ed-gutter-lines]>div')];let y=parseFloat(getComputedStyle(ta).paddingTop)||0;for(let i=0;i<rows.length;i++){const h=rows[i].getBoundingClientRect().height;if(y+h>ta.scrollTop+1)return i;y+=h;}return rows.length;});
 await setActiveScroll(page,0.6);
 const readingLine=await topLine();
 await dragSeparator(page,120);assert.equal(await topLine(),readingLine,'narrowing the map rewraps the text and keeps line '+readingLine+' at the top');
 await dragSeparator(page,-120);assert.equal(await topLine(),readingLine,'and widening it again');
 await pressSeparator(page,'Shift+ArrowRight');assert.equal(await topLine(),readingLine,'and a key press');
 await assertTextBox(page,'wrapped after resizing');
 await shot(page,'editor-map-text-wrapped-dark');
 await page.locator('#networkToolkitFileEditor [data-ed-wrap]').uncheck();await frames(page);
 pass('Text mode with Wrap on: rows are pieces of the text area\'s columns, as the blocks wrap; resizing keeps the line at the top in place');

 // Split view: the map follows the active pane; hidden in the comparison view.
 await pressSeparator(page,'End');
 await page.locator('#networkToolkitFileEditor [data-ed="split"]').click();await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor.is-split'));await frames(page);
 for(const side of ['left','right']){
  const uid=(await page.evaluate(side=>NetworkToolkitFileEditor.viewGroups.groups.find(g=>g.side===side)?.uid,side));
  await page.locator('#networkToolkitFileEditor [data-ed-doc-uid="'+uid+'"]').click();await frames(page);
  assert.equal((await activeArea(page)).side,side);
  const paneText=await page.evaluate(()=>NetworkToolkitFileEditor.active.content);
  await setActiveScroll(page,0.4);const read=await assertTextBox(page,side+' pane');assertRows(read.rows,paneText,{columns:read.info.layout.zoom.columns,label:side+' pane'});
  // The pane's own syntax colours reach the map (a configuration or JSON is coloured, plain text is not).
  const coloured=await page.evaluate(()=>/\.(cfg|json)$/.test(NetworkToolkitFileEditor.active.path));
  await page.waitForFunction(coloured=>NetworkToolkitFileEditor.documentMap.layout?.colored===coloured,coloured);
 }
 await page.locator('#networkToolkitFileEditor [data-ed="compare"]').click();await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor.is-comparing'));await frames(page);
 size=await mapSize(page);assert.equal(size.hidden,true,'the map is hidden in the comparison view');assert.equal(size.sep.hidden,true,'and so is its separator');
 await page.locator('#networkToolkitFileEditor [data-ed="compare"]').click();await page.waitForFunction(()=>!document.querySelector('#networkToolkitFileEditor.is-comparing'));await frames(page);
 size=await mapSize(page);assert.equal(size.hidden,false);assert.equal(size.sep.hidden,false);assert.equal(size.width,size.limits.max,'and back at its width');
 await page.locator('#networkToolkitFileEditor [data-ed="split"]').click();await page.waitForFunction(()=>!document.querySelector('#networkToolkitFileEditor.is-split'));await frames(page);
 pass('Text mode in split view follows the active pane; the map and its separator hide in the comparison view');

 // A reload keeps the width and the text.
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][0]));
 const wide=Math.round((await mapSize(page)).width);await kept(wide,'the largest width is saved');
 await page.evaluate(()=>{for(const d of NetworkToolkitFileEditor.documents.values())d.dirty=false;});
 await start();
 for(const file of FILES.keys())assert.equal(await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file).then(()=>'',error=>error.message),file),'');
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][0]));await frames(page);
 await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.layout?.mode==='text');
 size=await mapSize(page);assert.equal(Math.round(size.width),wide,'the width is restored after a reload');assert.equal(size.mode,'text');
 pass('Reload: the map keeps its width ('+wide+' px) and its text');
 // The batch a new editor window gets from this one (editor-windows.js move):
 // its preferences carry the width.
 const batch=await page.evaluate(()=>NetworkToolkitFileEditor.windowAdapter.checkpoint([...NetworkToolkitFileEditor.documents.values()].slice(0,2).map(d=>d.uid)));
 assert.equal(batch.preferences.documentMapWidth,wide,'a new editor window gets the width');assert.equal(batch.preferences.documentMap,true);

 // A 16 MB page-held document: a down-sampled overview, bounded map work.
 await pressSeparator(page,'Home');
 await page.evaluate(()=>{const map=NetworkToolkitFileEditor.documentMap;Object.assign(map.stats,{updates:0,draws:0,maxUpdate:0,maxDraw:0,textDraws:0,maxTextDraw:0});});
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
  const lines=await page.evaluate(()=>NetworkToolkitFileEditor.active.content.split('\n').length);
  assert(lines>i.layout.g.used*8,'more lines than map pixels times eight: the overview samples pixel rows ('+lines+' lines)');
  assert.equal(i.layout.g.used,i.height,'the whole document is fitted to the map');
  for(const fraction of [0.5,1])await setActiveScroll(page,fraction),await assertBoxMatches(page,'16 MB '+fraction);
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

 // The same document in text mode: only the rows in the map are drawn.
 await pressSeparator(page,'End');
 await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.layout?.mode==='text');
 const pageRows=()=>page.evaluate(()=>{const d=NetworkToolkitFileEditor.active;return NetworkToolkitFileEditor.documentMap.textRows.map(row=>{const end=d.content.indexOf('\n',row.offset);return {...row,line:d.content.slice(row.offset,end<0?undefined:end)};});});
 const checkRows=async label=>{const rows=await pageRows(),columns=(await mapSize(page)).mapColumns;assert(rows.length>=5&&rows.length<=200,label+': only the rows in the map are drawn ('+rows.length+')');for(const row of rows)assert.equal(row.text,shownRow(row.line,columns),label+': row at '+row.offset);return rows;};
 // Typing into a 16 MB page-held text makes 16 MB strings, and a garbage
 // collection after it can land in any frame, the map's too: a pass that sees
 // a pause over 50 ms is repeated once and the repeat decides (map work that
 // is slow in itself is slow both times; both passes are reported).
 const textWork=async typed=>{
  await page.evaluate(()=>{const map=NetworkToolkitFileEditor.documentMap;Object.assign(map.stats,{updates:0,draws:0,maxUpdate:0,maxDraw:0,textDraws:0,maxTextDraw:0});});
  for(const fraction of [0.5,1,0.25])await setActiveScroll(page,fraction),await assertTextBox(page,'16 MB text '+fraction),await checkRows('16 MB text '+fraction);
  const i=await mapInfo(page),x=i.left+i.width/2;
  await page.mouse.click(x,i.top+i.height*0.25);await frames(page);
  await assertTextBox(page,'16 MB text after a click');
  const box=(await mapInfo(page)).box;
  await page.mouse.move(x,i.top+box.top+5);await page.mouse.down();for(let step=1;step<=8;step++)await page.mouse.move(x,i.top+box.top+5+step*20);await page.mouse.up();
  await page.mouse.wheel(0,600);await frames(page);
  await assertTextBox(page,'16 MB text after a drag and the wheel');await checkRows('16 MB text after a drag and the wheel');
  await text.focus();await page.keyboard.press('Control+Home');for(let k=0;k<6;k++)await page.keyboard.type(typed);
  await page.waitForFunction(typed=>NetworkToolkitFileEditor.documentMap.textRows[0]?.text.startsWith(typed.repeat(6)),typed);
  const stats=(await mapInfo(page)).stats;
  assert(stats.textDraws>5,'text mode drew while scrolling: '+JSON.stringify(stats));
  return stats;
 };
 {
  const passes=[await textWork('y')];
  const fast=stats=>stats.maxUpdate<50&&stats.maxDraw<50&&stats.maxTextDraw<50;
  if(!fast(passes[0]))passes.push(await textWork('z'));
  assert(fast(passes.at(-1)),'the map work stays under 50 ms on a 16 MB document in text mode: '+JSON.stringify(passes));
  report.large16MiBText={pageHeld:true,width:(await mapSize(page)).width,font:(await mapSize(page)).font,stats:passes.at(-1),passes};
 }
 pass('16 MB page-held document in text mode: the rows in view only, map work '+JSON.stringify(report.large16MiBText.passes));
 await page.evaluate(()=>NetworkToolkitFileEditor.active.dirty=false);
 await context.close();
 return batch;
}

// A detached editor window at its native minimum size, 640 x 400.
async function detachedWindow(browser,batch){
 const view={id:'c'.repeat(32),ownerInstanceId:'e'.repeat(32)};
 const state={saved:{appUpdatePreferences:{enabled:false},theme:'dark'},posts:0};
 const context=await browser.newContext({viewport:{width:640,height:400},deviceScaleFactor:1}),page=await context.newPage();
 await serve(page,{mode:'editor',view,state,transfer:{id:'d'.repeat(32),batch}});
 await page.goto('https://toolkit.local/editor-view/'+view.id);
 await page.waitForFunction(()=>window.__networkToolkitStartupState===1&&online&&EDITOR_WINDOW===true);
 // The main window's move arrives with its preferences: the map, at the
 // main window's width as far as this window allows.
 await page.waitForFunction(()=>NetworkToolkitFileEditor.documents.size===2&&NetworkToolkitFileEditor.documentMapOn&&NetworkToolkitFileEditor.documentMap.layout);
 await frames(page);
 {
  let size=await mapSize(page);
  assert.equal(size.limits.min,64,'a narrow window\'s default is 64 px');
  assert(size.limits.max>=250&&size.limits.max<=330,'and at most half of its 640 px: '+size.limits.max);
  assert.equal(Math.round(size.width),Math.min(batch.preferences.documentMapWidth,size.limits.max),'the main window\'s width ('+batch.preferences.documentMapWidth+' px) within this window\'s limit');
  assert.equal(size.mode,'text','wide enough for text in a 640 px window too');
  assert(size.textClient>=240,'the text keeps a usable width: '+size.textClient);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)<=0,'no horizontal page scroll');
  const content=await page.evaluate(()=>NetworkToolkitFileEditor.active.content);
  assertRows(await textRows(page),content,{columns:size.mapColumns,label:'detached'});
  await assertTextBox(page,'detached text');
  await shot(page,'editor-map-text-detached-640x400-dark');
  // Resizing in the small window: the same clamps, the text stays usable.
  await dragSeparator(page,-1000);size=await mapSize(page);
  assert.equal(size.width,size.limits.max);assert(size.textClient>=240,'at the largest width the text keeps '+size.textClient+' px');
  await dragSeparator(page,1000);assert.equal((await mapSize(page)).width,64,'and at least 64 px');
  await dragSeparator(page,-150);assert.equal(Math.round((await mapSize(page)).width),214);
  await page.locator(SEP).dblclick();await frames(page);
  size=await mapSize(page);assert.equal(size.width,64,'a double-click resets the narrow default');assert.equal(size.mode,'blocks');
 }
 pass('Detached window at 640 x 400: the move batch carries the map\'s width; text mode, the clamps and the reset work there');
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
 // Escape in the list closes the narrow sidebar too.
 await page.locator('#networkToolkitFileEditor [data-editor-files-panel="documents"]').click();await frames(page);
 await page.locator('#networkToolkitFileEditor [data-ed-doc-uid]').first().focus();await page.keyboard.press('Escape');await frames(page);
 assert.equal(await page.locator('#networkToolkitFileEditor [data-ed-files-sidebar]').isVisible(),false,'Escape closes the narrow sidebar');
 assert.equal(await page.evaluate(()=>document.activeElement?.matches('[data-ed-text]')),true,'and returns to the text');
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
 const diskState=()=>page.evaluate(()=>{const d=NetworkToolkitFileEditor.active;return {disk:!!d.disk,lines:d.diskMeta?.lines,units:d.diskMeta?.units};});
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

 // The same file in text mode: the loaded page's lines; the rest a texture.
 await pressSeparator(page,'End');
 await page.waitForFunction(()=>{const m=NetworkToolkitFileEditor.documentMap,l=m.layout;return l?.kind==='lines'&&l.mode==='text'&&m.textRows.length>0;});
 const fileLines=fs.readFileSync(path.join(engineFiles,'large.cfg'),'utf8').split('\n');
 const diskRows=async label=>{
  const [info,rows,size]=await Promise.all([mapInfo(page),textRows(page),mapSize(page)]),l=info.layout,height=l.g.scale;
  assert(rows.length>=5&&rows.length<=200,label+': the rows in the map ('+rows.length+')');
  for(const row of rows)assert.equal(row.text,shownRow(fileLines[row.line],size.mapColumns),label+': line '+row.line);
  const at=rows.find(row=>row.y<=info.box.top+0.5&&row.y+height>info.box.top+0.5);
  if(at)assert(Math.abs(at.line-Math.floor(l.start))<=1,label+': the row at the box top is line '+at.line+', the view starts at '+l.start);
  return {info,rows,at};
 };
 await page.evaluate(()=>{mapLongTasks.length=0;const map=NetworkToolkitFileEditor.documentMap;Object.assign(map.stats,{updates:0,draws:0,maxUpdate:0,maxDraw:0,textDraws:0,maxTextDraw:0});});
 info=await mapInfo(page);
 assert.equal(info.layout.g.total,d.lines,'text mode still covers every line');
 assert(info.layout.g.offset>0||info.layout.start===0,'a 16 MB file scrolls inside the map');
 {
  const {at}=await diskRows('16 MB text');assert(at,'the view\'s top line is drawn at the box top');
  // A click centres the view on the line under the pointer, near the bottom and near the top of the map.
  for(const pick of [g=>g.used-12,()=>10]){
   const {info,rows}=await diskRows('16 MB text before a click'),y=pick(info.layout.g),row=rows.find(r=>r.y<=y&&r.y+info.layout.g.scale>y);
   assert(row&&(y<info.box.top||y>info.box.top+info.box.height),'a click outside the box on a drawn line');
   await page.mouse.click(x,info.top+y);
   await page.waitForFunction(line=>{const l=NetworkToolkitFileEditor.documentMap.layout,s=NetworkToolkitFileEditor.active;return l&&l.mode==='text'&&Math.abs(l.start+l.g.span/2-line)<=2&&!s.diskSession?.pending;},row.line,{timeout:15000});
   await settled();await page.waitForFunction(()=>{const m=NetworkToolkitFileEditor.documentMap,l=m.layout;return m.textRows.some(r=>r.y<=l.g.box.top+0.5&&r.y+l.g.scale>l.g.box.top+0.5);},null,{timeout:15000});
   const after=await diskRows('16 MB text after a click');assert(after.at,'the line at the box top is drawn after the click');
  }
  // Drag the box down, then the wheel over the map and over the text.
  info=await mapInfo(page);
  await page.mouse.move(x,info.top+info.box.top+info.box.height/2);await page.mouse.down();
  for(let step=1;step<=8;step++){await page.mouse.move(x,info.top+info.box.top+info.box.height/2+step*10);await page.waitForTimeout(16);}
  await page.mouse.up();
  await page.waitForFunction(top=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l&&l.g.box.top>top+60;},info.box.top,{timeout:15000});
  const draggedTo=(await mapInfo(page)).layout.start;
  assert(draggedTo>info.layout.start+(80/info.layout.g.drag)*0.9,'dragging the box 80 px moves the view by the thumb\'s scale: '+info.layout.start+' to '+draggedTo);
  await page.mouse.move(x,info.top+20);for(let i=0;i<20;i++){await page.mouse.wheel(0,400);await page.waitForTimeout(16);}
  await page.waitForFunction(line=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l&&l.start>line;},draggedTo,{timeout:15000});
  await editorArea.hover();for(let i=0;i<20;i++){await page.mouse.wheel(0,-400);await page.waitForTimeout(16);}
  await settled();await frames(page);await page.waitForTimeout(300);
  await diskRows('16 MB text after the drag and the wheel');
  // Resizing the map on the 16 MB file: narrower, then wider again.
  const wide=(await mapSize(page)).width;
  await dragSeparator(page,80,10);await settled();
  assert.equal(Math.round((await mapSize(page)).width),Math.round(wide)-80,'the separator narrows the map over a 16 MB file');
  await dragSeparator(page,-200,10);await settled();await frames(page);
  assert.equal(Math.round((await mapSize(page)).width),Math.round(wide),'and widens it to the largest width again');
  await diskRows('16 MB text after resizing');
 }
 {
  const longTasks=await page.evaluate(()=>mapLongTasks),stats=(await mapInfo(page)).stats;
  assert(longTasks.every(ms=>ms<=50),'no long task over 50 ms in text mode on a 16 MB disk-backed document: '+JSON.stringify(longTasks));
  assert(stats.textDraws>5&&stats.maxUpdate<50&&stats.maxDraw<50,'the map drew text and its work stays far under 50 ms: '+JSON.stringify(stats));
  report.disk16MiBText={lines:d.lines,width:(await mapSize(page)).width,font:(await mapSize(page)).font,longTasks,stats};
 }
 await shot(page,'editor-map-list-disk-16mb-text-dark');
 pass('16 MB disk-backed document in text mode: the loaded page\'s lines, the top line at the box top; clicks, a drag and the wheel with no long task over 50 ms '+JSON.stringify(report.disk16MiBText.stats));
 await pressSeparator(page,'Home');

 // 1.7 MB on one line (a Base64 export) with Wrap on: mapped by offset.
 await open('export.b64');
 d=await diskState();assert.equal(d.disk,true);assert.equal(d.lines,1,'one line');
 if(!await page.locator('#networkToolkitFileEditor [data-ed-wrap]').isChecked())await page.locator('#networkToolkitFileEditor [data-ed-wrap]').check();
 await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.layout?.kind==='offsets');
 info=await mapInfo(page);
 assert.equal(info.layout.g.total,d.units,'the map covers every character of the line');
 assert(info.layout.g.used>info.height*0.9,'a 1.7 MB line fills the map');
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
 await shot(page,'editor-map-list-base64-wrap-dark');
 pass('1.7 MB single-line Base64 with Wrap on: mapped by character offset; the map reaches the middle, the end and the start');

 // The same line in text mode: rows of the wrap columns, the view's row at the box top.
 const base64=fs.readFileSync(path.join(engineFiles,'export.b64'),'utf8');
 await pressSeparator(page,'End');
 await page.waitForFunction(()=>{const m=NetworkToolkitFileEditor.documentMap,l=m.layout;return l?.kind==='offsets'&&l.mode==='text'&&m.textRows.length>0;});
 const flowRows=async label=>{
  const [info,rows,size]=await Promise.all([mapInfo(page),textRows(page),mapSize(page)]),l=info.layout,columns=l.columns;
  assert(rows.length>=5,label+': rows are drawn');
  for(const [k,row] of rows.entries()){
   assert.equal(row.text,base64.slice(row.offset,row.offset+Math.min(columns,size.mapColumns)),label+': the row at '+row.offset+' is the file\'s text there');
   if(k)assert.equal(row.offset,rows[k-1].offset+columns,label+': rows follow each other by the wrap columns');
  }
  const at=rows.find(row=>row.offset<=l.start&&l.start<row.offset+columns);
  assert(at&&Math.abs(at.y-info.box.top)<=1,label+': the row holding the view\'s first character ('+l.start+') is at the box top');
  return {info,rows};
 };
 await page.evaluate(()=>{mapLongTasks.length=0;});
 await flowRows('Base64 text at the start');
 info=await mapInfo(page);
 await page.mouse.click(x,info.top+info.layout.g.used-10);
 await page.waitForFunction(start=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l&&l.start>start;},info.layout.start,{timeout:15000});
 await settled();await page.waitForFunction(()=>{const m=NetworkToolkitFileEditor.documentMap,l=m.layout;return m.textRows.some(r=>r.offset<=l.start&&l.start<r.offset+l.columns);},null,{timeout:15000});
 await flowRows('Base64 text after a click');
 // Drag the box to the very end: the last characters, in the last row.
 info=await mapInfo(page);
 await page.mouse.move(x,info.top+info.box.top+info.box.height/2);await page.mouse.down();await page.mouse.move(x,info.top+info.height+40,{steps:12});await page.mouse.up();
 await page.waitForFunction(units=>{const s=NetworkToolkitFileEditor.active.diskSession.state().page;const ta=document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');return s.start+s.text.length>=units&&ta.scrollTop+ta.clientHeight>=ta.scrollHeight-2;},d.units,{timeout:15000});
 await page.waitForFunction(units=>NetworkToolkitFileEditor.documentMap.textRows.some(r=>r.offset+r.text.length>=units),d.units,{timeout:15000});
 {const {rows}=await flowRows('Base64 text at the end');assert.equal(rows.at(-1).text,base64.slice(rows.at(-1).offset),'the last row ends with the file\'s last characters');}
 // And back to the start.
 info=await mapInfo(page);
 await page.mouse.move(x,info.top+info.box.top+info.box.height/2);await page.mouse.down();await page.mouse.move(x,info.top-40,{steps:12});await page.mouse.up();
 await page.waitForFunction(()=>{const s=NetworkToolkitFileEditor.active.diskSession.state().page;const ta=document.querySelector('#networkToolkitFileEditor .nt-editor-code [data-ed-text]');return s.start===0&&ta.scrollTop<2;},null,{timeout:15000});
 await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.textRows[0]?.offset===0);
 {const {rows}=await flowRows('Base64 text back at the start');assert.equal(rows[0].offset,0);}
 {
  const longTasks=await page.evaluate(()=>mapLongTasks);
  assert(longTasks.every(ms=>ms<=50),'no long task over 50 ms in text mode on the 1.7 MB line: '+JSON.stringify(longTasks));
  report.base64Text={units:d.units,longTasks,stats:(await mapInfo(page)).stats};
 }
 await shot(page,'editor-map-list-base64-wrap-text-dark');
 pass('1.7 MB single-line Base64 with Wrap on in text mode: rows are the file\'s text by the wrap columns, the view\'s row at the box top, from the start to the end');

 // The engine keeps the settings: reload.
 const keptWidth=Math.round((await mapSize(page)).width);
 await page.waitForFunction(width=>state.editorPreferences?.documentMapWidth===width,keptWidth);
 await page.waitForTimeout(600);
 await start();
 assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.documentMapOn),true,'the map setting is restored by the engine');
 assert.equal(await page.evaluate(()=>state.editorPreferences?.documentMapWidth),keptWidth,'and its width ('+keptWidth+' px)');
 assert.equal(await page.evaluate(()=>NetworkToolkitEditorFiles.panel),'documents','the Document List is restored by the engine');
 pass('Standalone Text Editor: the engine keeps the map and the Document List after a reload');
 await context.close();
}

(async()=>{
 fs.mkdirSync(output,{recursive:true});
 const browser=await chromium.launch({headless:true,...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 try{
  const batch=await mainWindow(browser);
  await detachedWindow(browser,batch);
  if(engineURL)await engine(browser);
  assert.deepEqual(report.errors,[]);
  report.passed=true;
 }finally{
  await browser.close();
  fs.writeFileSync(path.join(output,'editor-map-list-results.json'),JSON.stringify(report,null,2));
 }
})().catch(error=>{console.error(error);console.error(JSON.stringify(report.errors));process.exitCode=1;});
