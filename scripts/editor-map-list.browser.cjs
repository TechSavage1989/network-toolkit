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
//  - the sidebar tabs (owner request after v1.21.58: More → View → Document
//    list, Connections, Files): ticked by default; unticking one removes its
//    rail button and panel and the sidebar falls back to the first remaining
//    tab in rail order (keyboard too), keeping its open state, side and width;
//    with all three off there is no sidebar and the text takes the full width;
//    ticking one brings it back in its place, open; a saved Docs panel falls
//    back while Docs is off and returns when it is ticked; explicit requests
//    (the palette, Choose a connection, Use) turn a hidden tab back on, a
//    setting synced from another app does not open anything; kept after a
//    reload;
//  - a detached editor window at its native minimum size (640 x 400) stays
//    usable with both, and with the sidebar tabs turned off.
// Part 2, when NTK_EDITOR_STANDALONE_URL is set (TestEditorSessionBrowserMapList
// in editor_map_list_browser_test.go runs the standalone Text Editor against
// its production engine): a 16 MB disk-backed document is mapped by line and
// scrolled through the map with no long task over 50 ms; a 1.7 MB single-line
// Base64 document with Wrap on is mapped by character offset and the map
// reaches its first and last characters; both settings, and a sidebar tab
// turned off, are kept by the engine.
// Screenshots (light and dark, device scale 1) go to TOOLKIT_TEST_OUTPUT.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version;
const output=process.env.TOOLKIT_TEST_OUTPUT||process.env.NTK_BROWSER_OUTPUT_DIR||path.join(root,'test-results','editor-map-list');
const {NTK_EDITOR_STANDALONE_URL:engineURL,NTK_EDITOR_STANDALONE_TOKEN:engineToken,NTK_EDITOR_MAP_FILES:engineFiles}=process.env;
const report={passed:false,checks:[],errors:[]};
const pass=name=>{report.checks.push(name);console.log('PASS '+name);};
// A full garbage collection (DevTools protocol) before a timed pass, so
// garbage the check itself made while building large test documents is not
// counted as the map's work.
const collectGarbage=async page=>{const cdp=await page.context().newCDPSession(page);try{await cdp.send('HeapProfiler.collectGarbage');}finally{await cdp.detach();}};

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
// The sidebar: which rail buttons show, the selected one, the open panel and
// the More menu boxes.
const railState=page=>page.evaluate(()=>{
 const ed=document.querySelector('#networkToolkitFileEditor'),shell=ed.querySelector('.nt-editor-files-shell'),host=ed.querySelector('[data-ed-files-sidebar]'),visible=el=>!!el&&getComputedStyle(el).display!=='none'&&el.getClientRects().length>0;
 return {rail:[...ed.querySelectorAll('[data-editor-files-panel]')].filter(visible).map(el=>el.dataset.editorFilesPanel),selected:ed.querySelector('[data-editor-files-panel][aria-selected="true"]')?.dataset.editorFilesPanel||'',
  shell:visible(shell),dock:visible(ed.querySelector('[data-editor-files-dock]')),open:visible(host),panel:host?.dataset.editorSidebarPanel||'',width:shell?Math.round(shell.getBoundingClientRect().width):0,right:!!shell?.classList.contains('dock-right'),
  list:visible(ed.querySelector('[data-editor-documents]')),connect:visible(ed.querySelector('#ntEditorRemotePanel')),files:visible(ed.querySelector('.nt-editor-files-locations')),
  boxes:Object.fromEntries([...ed.querySelectorAll('[data-ed-sidebar-panel]')].map(box=>[box.dataset.edSidebarPanel,box.checked])),pref:NetworkToolkitFileEditor.sidebarPanels};
});
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
   // The font the map should draw text at: the width's font, or the largest
   // at which every row of the document fits the map's height; 0 (blocks)
   // below the width threshold or below 4 px.
   fit:(()=>{const size=parseFloat(style.fontSize),zoom=NTKEditorDocumentMap.textZoom({width:host.getBoundingClientRect().width,columns:l?.columns||0,charWidth:ctx.measureText('0').width,fontSize:size});if(!zoom)return 0;if(ta.scrollHeight*zoom.scale<=host.clientHeight)return zoom.font;const font=Math.floor(size*host.clientHeight/ta.scrollHeight*4)/4;return font>=4?font:0;})(),
   pref:state.editorPreferences?.documentMapWidth,text:box(ta),textClient:ta.clientWidth,scrollTop:ta.scrollTop};
 });
}
async function dragSeparator(page,dx,steps=8){
 const b=await page.locator(SEP).boundingBox(),x=b.x+b.width/2,y=b.y+Math.min(80,b.height/2);
 await page.mouse.move(x,y);await page.mouse.down();await page.mouse.move(x+dx,y,{steps});await page.mouse.up();await frames(page);
}
async function pressSeparator(page,key){await page.locator(SEP).focus();await page.keyboard.press(key);await frames(page);}
// What the map's canvas holds over the document's height: how many of 20
// bands have anything drawn, and how many hues (directions from the
// background colour, so the shades of one translucent colour count once).
const mapPaint=page=>page.evaluate(()=>{
 const canvas=document.querySelector('#networkToolkitFileEditor [data-ed-map] canvas'),ctx=canvas.getContext('2d'),l=NetworkToolkitFileEditor.documentMap.layout;
 const used=Math.max(1,Math.floor(l.g.used*devicePixelRatio)),data=ctx.getImageData(0,0,canvas.width,used).data,counts=new Map();
 for(let i=0;i<data.length;i+=4){const key=data[i]<<16|data[i+1]<<8|data[i+2];counts.set(key,(counts.get(key)||0)+1);}
 const bg=[...counts].sort((a,b)=>b[1]-a[1])[0][0],base=[bg>>16,bg>>8&255,bg&255],hues=new Set(),bands=new Set();
 for(let y=0;y<used;y++)for(let x=0;x<canvas.width;x++){
  const i=(y*canvas.width+x)*4,d=[data[i]-base[0],data[i+1]-base[1],data[i+2]-base[2]],size=Math.hypot(...d);
  if(size<24)continue;
  bands.add(Math.floor(y*20/used));hues.add(d.map(v=>Math.round(v/size*3)).join(','));
 }
 return {bands:bands.size,hues:hues.size,mode:l.mode,sampled:!!l.sampled,offset:l.g.offset,used:l.g.used};
});
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
 const mapBox=page.locator('#ntEditorMoreActions [data-ed-document-map]'),listBox=page.locator('#ntEditorMoreActions [data-ed-sidebar-panel="documents"]');
 const panelBox=name=>page.locator('#ntEditorMoreActions [data-ed-sidebar-panel="'+name+'"]');
 assert.equal(await mapBox.isChecked(),false);
 assert.deepEqual(await page.locator('#ntEditorMoreActions [data-ed-sidebar-panel]').evaluateAll(list=>list.map(box=>[box.dataset.edSidebarPanel,box.closest('label').textContent.trim(),box.checked])),[['documents','Document list',true],['connections','Connections',true],['files','Files',true]],'the sidebar tab boxes, in rail order, ticked by default');
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
 assert.equal(await page.evaluate(()=>document.querySelector('#ntEditorMoreActions [data-ed-sidebar-panel="documents"]').checked),true,'the Document list box shows Docs is on the rail');
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
 await paletteRun(page,'Show the documents sidebar');
 assert.equal(await page.locator('#networkToolkitFileEditor [data-editor-documents]').isVisible(),true,'the palette opens the list');
 assert.equal((await listRows(page)).find(row=>row.focused)?.active,true,'with the keyboard on the active row');
 await page.keyboard.press('Escape');assert.equal(await page.evaluate(()=>document.activeElement?.matches('[data-ed-text]')),true,'Escape returns to the text');
 await page.keyboard.press('F1');await page.locator('[aria-label="Keyboard shortcuts and help"][role="dialog"]').waitFor();
 const help=await page.locator('#uxCommandOverlay .ux-help-row').allInnerTexts();
 assert(help.some(row=>/Document map/.test(row)),'F1 help explains the document map');assert(help.some(row=>/List the open documents/.test(row)),'and the document list');
 assert(help.some(row=>/More > Document list \/ Connections \/ Files/.test(row)&&/Show or remove Docs, Connect or Files on the sidebar rail; with all three off there is no sidebar/.test(row)),'and the sidebar tab boxes');
 await page.keyboard.press('Escape');
 pass('Command palette and F1 help: Document map, Document list and the sidebar tab boxes');

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
 pass('Reload: the map and the Document List are restored');

 // ---- Sidebar tabs: More → View → Document list, Connections, Files ----
 const settled=async(check,label)=>{for(let i=0;i<60;i++){const r=await railState(page);if(check(r))return r;await frames(page);}assert.fail(label+': '+JSON.stringify(await railState(page)));};
 const saved=()=>state.saved.editorPreferences?.sidebarPanels;
 const waitSaved=async(want,label)=>{for(let i=0;i<60&&JSON.stringify(saved())!==JSON.stringify(want);i++)await page.waitForTimeout(50);assert.deepEqual(saved(),want,label);};
 const tick=async(name,on)=>{await openMore(page);if(on)await panelBox(name).check();else await panelBox(name).uncheck();await frames(page);};
 const textBox=()=>page.evaluate(()=>{const area=document.querySelector('#networkToolkitFileEditor .nt-editor-document-area').getBoundingClientRect(),work=document.querySelector('#networkToolkitFileEditor .nt-editor-workspace').getBoundingClientRect(),map=document.querySelector('#networkToolkitFileEditor [data-ed-map]').getBoundingClientRect(),sep=document.querySelector('#networkToolkitFileEditor [data-ed-map-resize]')?.getBoundingClientRect();return {left:area.left-work.left,width:area.width,room:work.width-map.width-(sep?.width||0)};});
 let bar=await railState(page);
 assert.deepEqual([bar.rail,bar.selected,bar.open,bar.list],[['documents','connections','files'],'documents',true,true],'Docs, Connect and Files on the rail, the list open');
 const width=bar.width;
 // Unticking Document list removes Docs and its panel; the sidebar stays open
 // on the first remaining tab, Connect, at the same width.
 const postsBefore=state.posts;
 await tick('documents',false);
 bar=await settled(r=>!r.rail.includes('documents'),'Docs leaves the rail');
 assert.deepEqual({rail:bar.rail,selected:bar.selected,open:bar.open,panel:bar.panel,list:bar.list,connect:bar.connect,width:bar.width,boxes:bar.boxes},{rail:['connections','files'],selected:'connections',open:true,panel:'connections',list:false,connect:true,width,boxes:{documents:false,connections:true,files:true}},'unticked: no Docs button or panel; the sidebar falls back to Connect and keeps its width');
 assert.deepEqual(bar.pref,{documents:false,connections:true,files:true});
 await waitSaved({documents:false,connections:true,files:true},'the setting is saved with the editor preferences');
 assert(state.posts>postsBefore);
 assert.equal(state.saved.editorFilesPreferences?.panel,'documents','the saved panel choice is not rewritten by the fallback');
 await page.keyboard.press('Escape');
 // The keyboard on the rail skips the hidden Docs.
 await page.locator('#networkToolkitFileEditor [data-editor-files-panel="connections"]').focus();
 await page.keyboard.press('ArrowUp');assert.equal((await railState(page)).selected,'files','Up from Connect wraps to Files, past the hidden Docs');
 await page.keyboard.press('Home');assert.equal((await railState(page)).selected,'connections','Home is Connect');
 await page.keyboard.press('End');await page.keyboard.press('ArrowDown');assert.equal((await railState(page)).selected,'connections','Down from Files wraps to Connect');
 assert.equal(await page.evaluate(()=>document.activeElement?.dataset?.editorFilesPanel),'connections');
 // A reload with Docs off and the Docs panel saved as the choice shows the
 // first remaining tab and keeps the stored choice for when Docs returns.
 await page.waitForTimeout(700);
 await page.evaluate(()=>{state.editorFilesPreferences={...state.editorFilesPreferences,panel:'documents'};saveState();});
 for(let i=0;i<40&&state.saved.editorFilesPreferences?.panel!=='documents';i++)await page.waitForTimeout(50);
 await start();
 for(const file of FILES.keys())assert.equal(await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file).then(()=>'',error=>error.message),file),'');
 bar=await settled(r=>r.open,'the sidebar after a reload');
 assert.deepEqual({rail:bar.rail,selected:bar.selected,list:bar.list,boxes:bar.boxes},{rail:['connections','files'],selected:'connections',list:false,boxes:{documents:false,connections:true,files:true}},'after a reload Docs is still off and a saved Docs panel falls back to Connect');
 assert.equal(state.saved.editorFilesPreferences.panel,'documents','the stored choice is kept');
 // Ticking it brings Docs back first, with its panel open.
 await tick('documents',true);
 bar=await settled(r=>r.list,'Docs returns');
 assert.deepEqual({rail:bar.rail,selected:bar.selected,open:bar.open,boxes:bar.boxes,width:bar.width},{rail:['documents','connections','files'],selected:'documents',open:true,boxes:{documents:true,connections:true,files:true},width},'ticked: Docs is first again and its panel shows');
 assert.equal((await listRows(page)).length,4);
 await waitSaved({documents:true,connections:true,files:true},'saved on again');
 await page.keyboard.press('Escape');
 pass('Sidebar tabs: Document list on by default; unticked, Docs and its panel leave the rail, the sidebar falls back to Connect (keyboard too) and a reload keeps it; ticked, Docs is first again and open');

 // Each box on its own, then all three off: no sidebar, the text takes the width.
 await tick('connections',false);
 bar=await settled(r=>!r.rail.includes('connections'),'Connect leaves the rail');
 assert.deepEqual([bar.rail,bar.selected,bar.open],[['documents','files'],'documents',true],'Connections unticked: Connect is gone, the open Docs panel stays');
 await page.keyboard.press('Escape');await page.locator('#networkToolkitFileEditor [data-editor-files-panel="files"]').click();
 bar=await settled(r=>r.files,'Files opens');
 await tick('files',false);
 bar=await settled(r=>!r.rail.includes('files'),'Files leaves the rail');
 assert.deepEqual([bar.rail,bar.selected,bar.open,bar.list],[['documents'],'documents',true,true],'Files unticked while showing: the sidebar falls back to Docs');
 await tick('connections',true);
 bar=await settled(r=>r.connect,'Connect returns');
 assert.deepEqual([bar.rail,bar.selected,bar.open],[['documents','connections'],'connections',true],'Connections ticked: Connect returns in its place and opens');
 const sidebarText=await textBox();
 await tick('connections',false);await tick('documents',false);
 bar=await settled(r=>!r.shell,'the sidebar disappears');
 assert.deepEqual({shell:bar.shell,dock:bar.dock,rail:bar.rail,open:bar.open,boxes:bar.boxes},{shell:false,dock:false,rail:[],open:false,boxes:{documents:false,connections:false,files:false}},'all three off: no rail, no side button, no sidebar');
 const full=await textBox();
 assert(full.left<=1&&Math.abs(full.width-full.room)<=1,'the text takes the full width: '+JSON.stringify(full));
 assert(full.width>sidebarText.width+200,'wider than beside the sidebar: '+JSON.stringify({full,sidebarText}));
 await waitSaved({documents:false,connections:false,files:false},'all three off is saved');
 await page.keyboard.press('Escape');
 assert.match(await page.locator('#networkToolkitFileEditor [data-ed-message]').textContent(),/The editor sidebar is hidden/,'the status line says how to bring it back');
 await shot(page,'editor-sidebar-tabs-all-off-dark');
 // Ticking any box brings the rail back with that tab open.
 await tick('files',true);
 bar=await settled(r=>r.files,'Files returns');
 assert.deepEqual([bar.shell,bar.dock,bar.rail,bar.selected,bar.open,bar.width],[true,true,['files'],'files',true,width],'Files ticked: the rail is back with Files open at the same width');
 await page.keyboard.press('Escape');
 pass('Sidebar tabs: Connections and Files on their own; with all three off there is no rail and the text takes the full width; ticking one brings the rail back open on it');

 // Explicit requests turn a hidden tab back on; a synced setting opens nothing.
 // Files off, palette "Show the files sidebar":
 await tick('connections',true);await tick('files',false);await page.keyboard.press('Escape');
 bar=await settled(r=>r.rail.join()==='connections','only Connect');
 await paletteRun(page,'Show the files sidebar');
 bar=await settled(r=>r.files,'the palette shows Files');
 assert.deepEqual([bar.rail,bar.selected,bar.boxes.files],[['connections','files'],'files',true],'Show the files sidebar turns Files back on (its box is ticked) and shows it');
 // Connect off, the Files panel's "Choose a connection ›" (its SFTP side):
 await tick('connections',false);await page.keyboard.press('Escape');
 bar=await settled(r=>r.rail.join()==='files'&&r.files,'only Files');
 await page.locator('#networkToolkitFileEditor [data-editor-files-side="remote"]').click();
 const choose=page.locator('#networkToolkitFileEditor [data-ed-files-sidebar] [data-rb-editor-connections]').first();await choose.waitFor();
 await choose.click();
 bar=await settled(r=>r.connect,'Choose a connection shows Connect');
 assert.deepEqual([bar.rail,bar.selected,bar.boxes.connections],[['connections','files'],'connections',true],'Choose a connection in Files turns Connect back on (its box is ticked) and shows it');
 // Connect off again, palette "Show the connections sidebar":
 await tick('connections',false);await page.keyboard.press('Escape');
 await paletteRun(page,'Show the connections sidebar');
 bar=await settled(r=>r.connect,'the palette shows Connect');
 assert.deepEqual([bar.rail,bar.selected,bar.boxes.connections],[['connections','files'],'connections',true],'Show the connections sidebar turns Connect back on and shows it');
 // Files off, Connect's Use on a connected file client (a synthetic one).
 await tick('files',false);await page.keyboard.press('Escape');
 bar=await settled(r=>r.rail.join()==='connections'&&r.connect,'only Connect');
 await page.evaluate(()=>{const b=NetworkToolkitRemoteBrowser,c=b.configurations.ssh;Object.assign(c,{host:'192.0.2.10',port:22,username:'operator'});b.get('ssh').connected=true;return NetworkToolkitFileEditor.refreshConnectedSources();});
 await page.waitForFunction(()=>[...document.querySelectorAll('#ntEditorRemoteSource option')].some(o=>o.value==='client:ssh'));
 await page.locator('#ntEditorRemoteSource').selectOption('client:ssh');
 await page.locator('#ntEditorOpenRemote').click();
 bar=await settled(r=>r.selected==='files'&&r.open,'Use shows Files');
 assert.deepEqual([bar.rail,bar.boxes.files],[['connections','files'],true],'Use turns Files back on (its box is ticked) and shows the connection\'s files');
 await page.evaluate(()=>{NetworkToolkitRemoteBrowser.get('ssh').connected=false;});
 // Docs off, palette "Show or hide the document list" turns it on with the keyboard on the list.
 await tick('documents',false);await page.keyboard.press('Escape');
 await paletteRun(page,'Show or hide the document list');
 bar=await settled(r=>r.list,'the palette shows the list');
 assert.deepEqual([bar.rail,bar.selected,bar.boxes.documents],[['documents','connections','files'],'documents',true],'Show or hide the document list turns Docs on');
 assert.equal(await page.evaluate(()=>!!document.activeElement?.closest('[data-editor-documents]')),true,'with the keyboard on the list');
 await paletteRun(page,'Show or hide the document list');
 bar=await settled(r=>!r.rail.includes('documents'),'and off');
 assert.equal(bar.boxes.documents,false);
 // All off, palette "Show or hide the editor sidebar" brings back the last panel.
 await tick('connections',false);await tick('files',false);await page.keyboard.press('Escape');
 bar=await settled(r=>!r.shell,'no sidebar');
 await paletteRun(page,'Show or hide the editor sidebar');
 bar=await settled(r=>r.shell&&r.open,'the palette brings the sidebar back');
 assert.equal(bar.rail.length,1,'one tab comes back: '+JSON.stringify(bar));assert.equal(bar.boxes[bar.selected],true);
 // A setting synced from another app removes and adds tabs but opens nothing
 // and writes nothing.
 for(const name of ['documents','connections','files'])await tick(name,true);
 await page.keyboard.press('Escape');
 await page.evaluate(()=>NetworkToolkitEditorFiles.selectPanel('connections'));
 bar=await settled(r=>r.connect,'Connect shows');
 await page.waitForTimeout(700);
 const quiet=state.posts,keptPanel=state.saved.editorFilesPreferences.panel;
 await page.evaluate(()=>NetworkToolkitFileEditor.windowAdapter.applyPreferences({...state.editorPreferences,sidebarPanels:{documents:true,connections:false,files:true}}));
 bar=await settled(r=>!r.rail.includes('connections'),'a synced setting removes Connect');
 assert.deepEqual([bar.rail,bar.selected,bar.open],[['documents','files'],'documents',true],'the sidebar falls back to Docs');
 await page.evaluate(()=>NetworkToolkitFileEditor.windowAdapter.applyPreferences({...state.editorPreferences,sidebarPanels:{documents:true,connections:true,files:true}}));
 bar=await settled(r=>r.rail.includes('connections'),'a synced setting adds Connect');
 assert.deepEqual([bar.rail,bar.selected],[['documents','connections','files'],'documents'],'Connect returns to the rail without taking over the sidebar');
 await page.waitForTimeout(700);
 assert.equal(state.posts,quiet,'a synced setting writes nothing');assert.equal(state.saved.editorFilesPreferences.panel,keptPanel);
 pass('Sidebar tabs: Show the files sidebar and Show the connections sidebar (palette), Choose a connection (Files) and Use (Connect) turn a hidden tab back on; the palette toggles Docs and brings back a sidebar with no tabs; a synced setting opens nothing');


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

 // ---- The map always shows the whole document (owner request after v1.21.59) ----
 // At every width all of the document is in the map and nothing scrolls inside
 // it: its real text when every row fits at 4 px or more, else blocks.
 const whole=async label=>{
  const [info,size,ta]=await Promise.all([mapInfo(page),mapSize(page),activeArea(page)]),g=info.layout.g;
  assert.equal(g.offset,0,label+': nothing scrolls inside the map');
  assert(g.used<=info.height+0.5&&Math.abs(g.total-ta.scrollHeight)<=1,label+': the whole document is in the map');
  assert.equal(size.font,size.fit,label+': '+(size.fit?'text at '+size.fit+' px':'blocks')+' at '+size.width+' px');
  assert.equal(size.mode,size.fit?'text':'blocks',label);
  const height=Math.max(10,ta.clientHeight*g.scale),top=Math.min(ta.scrollTop*g.scale,g.used-Math.min(g.used,height));
  assert(Math.abs(info.box.top-top)<=1&&Math.abs(info.box.height-Math.min(g.used,height))<=1,label+': the box is the part on screen');
  return {info,size,ta};
 };
 // A long document (706 lines): blocks at every width, all of it.
 await setActiveScroll(page,0.4);
 await pressSeparator(page,'Home');
 const widths=[];
 for(;;){
  const {size:step}=await whole('long document');widths.push(step.width+':'+step.mode);
  if(step.width>=step.limits.max)break;
  await pressSeparator(page,'ArrowLeft');
 }
 assert(widths.length>5&&widths.every(step=>step.endsWith(':blocks')),'a long document is blocks at every width: '+widths.join(' '));
 {const paint=await mapPaint(page);assert.equal(paint.bands,20,'every part of the long document is drawn');assert(paint.hues>=3,'in the syntax colours: '+JSON.stringify(paint));}
 await shot(page,'editor-map-wide-blocks-dark');
 pass('The whole document at every width: a long document stays blocks in its colours, the box the part on screen');

 // A short document (60 lines): wide enough, its real text, every line of it.
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][3]));await frames(page);
 await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.layout?.mode==='text');
 let content=await page.evaluate(()=>NetworkToolkitFileEditor.active.content);
 {
  const {size:wideSize,info}=await whole('short document');
  assert(wideSize.font>=4&&wideSize.font<=9,'a legible font: '+wideSize.font);
  const rows=await textRows(page),lines=content.split('\n');
  assertRows(rows,content,{columns:wideSize.mapColumns,label:'short document'});
  assert(rows.length>=60&&rows.length<=lines.length,'every line of the document is drawn: '+rows.length);
  for(const at of [0,29,59])assert.equal(rows[at].text,shownRow(lines[at],wideSize.mapColumns),'row '+at+' is line '+at);
  assert(rows[59].y<info.layout.g.used,'the last line is inside the map');
  for(const fraction of [0.5,1,0])await setActiveScroll(page,fraction),await whole('short document at '+fraction);
  // A click centres the view; typing redraws the text.
  await page.mouse.click(info.left+info.width/2,info.top+info.layout.g.used-4);await frames(page);
  const ta=await activeArea(page);assert(ta.scrollTop>=ta.scrollHeight-ta.clientHeight-2,'a click at the end of the map shows the end');
  await text.focus();await page.keyboard.press('Control+End');await page.keyboard.type('interface Loopback99');await frames(page);
  content=await page.evaluate(()=>NetworkToolkitFileEditor.active.content);
  await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.textRows.at(-1)?.text==='interface Loopback99');
  await whole('after typing');
  await shot(page,'editor-map-text-dark');
  // Narrower than the threshold: blocks again, still all of it.
  await pressSeparator(page,'Home');const {size:narrow}=await whole('short document, narrow');assert.equal(narrow.mode,'blocks');
  await pressSeparator(page,'End');await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.layout?.mode==='text');
  // Wrap on: still the whole document.
  await page.locator('#networkToolkitFileEditor [data-ed-wrap]').check();await frames(page);await frames(page);
  await whole('short document, wrapped');
  await page.locator('#networkToolkitFileEditor [data-ed-wrap]').uncheck();await frames(page);
 }
 pass('A short document, wide enough: its real text, every line, the box the part on screen; blocks again when narrow');

 // A reload keeps the width.
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][0]));
 const wide=Math.round((await mapSize(page)).width);await kept(wide,'the largest width is saved');
 await page.evaluate(()=>{for(const d of NetworkToolkitFileEditor.documents.values())d.dirty=false;});
 await start();
 for(const file of FILES.keys())assert.equal(await page.evaluate(file=>NetworkToolkitFileEditor.openLocalPath(file).then(()=>'',error=>error.message),file),'');
 await page.evaluate(()=>NetworkToolkitFileEditor.activateDocument([...NetworkToolkitFileEditor.documents.keys()][0]));await frames(page);
 await page.waitForFunction(()=>NetworkToolkitFileEditor.documentMap.layout);
 size=await mapSize(page);assert.equal(Math.round(size.width),wide,'the width is restored after a reload');
 pass('Reload: the map keeps its width ('+wide+' px)');
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
 await collectGarbage(page);
 await page.evaluate(()=>{const map=NetworkToolkitFileEditor.documentMap;Object.assign(map.stats,{updates:0,draws:0,maxUpdate:0,maxDraw:0,textDraws:0,maxTextDraw:0});});
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

 // The same document with the map at its widest: still all of it, as blocks.
 await pressSeparator(page,'End');await frames(page);
 await collectGarbage(page);
 await page.evaluate(()=>{const map=NetworkToolkitFileEditor.documentMap;Object.assign(map.stats,{updates:0,draws:0,maxUpdate:0,maxDraw:0,textDraws:0,maxTextDraw:0});});
 for(const fraction of [0.5,1,0.25])await setActiveScroll(page,fraction),await assertBoxMatches(page,'16 MB wide '+fraction);
 {
  const i=await mapInfo(page),paint=await mapPaint(page);
  assert.equal(i.layout.mode,'blocks');assert.equal(i.layout.g.used,i.height,'the whole document is fitted to the wide map');assert.equal(paint.bands,20);
  assert(i.stats.maxUpdate<50&&i.stats.maxDraw<50,'the map work stays under 50 ms on a 16 MB document at the widest map: '+JSON.stringify(i.stats));
  report.large16MiBWide={pageHeld:true,width:(await mapSize(page)).width,stats:i.stats};
 }
 pass('16 MB page-held document at the widest map: the whole document as blocks, map work '+JSON.stringify(report.large16MiBWide.stats));
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
  assert(size.textClient>=240,'the text keeps a usable width: '+size.textClient);
  assert(await page.evaluate(()=>document.documentElement.scrollWidth-innerWidth)<=0,'no horizontal page scroll');
  const content=await page.evaluate(()=>NetworkToolkitFileEditor.active.content);
  assert.equal((await mapInfo(page)).layout.g.offset,0,'the whole document is in the map');
  await shot(page,'editor-map-text-detached-640x400-dark');
  // Resizing in the small window: the same clamps, the text stays usable.
  await dragSeparator(page,-1000);size=await mapSize(page);
  assert.equal(size.width,size.limits.max);assert(size.textClient>=240,'at the largest width the text keeps '+size.textClient+' px');
  await dragSeparator(page,1000);assert.equal((await mapSize(page)).width,64,'and at least 64 px');
  await dragSeparator(page,-150);assert.equal(Math.round((await mapSize(page)).width),214);
  await page.locator(SEP).dblclick();await frames(page);
  size=await mapSize(page);assert.equal(size.width,64,'a double-click resets the narrow default');assert.equal(size.mode,'blocks');
 }
 pass('Detached window at 640 x 400: the move batch carries the map\'s width; the clamps and the reset work there');
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
 // The sidebar tabs at 640 x 400: the setting a new window gets in its batch
 // (TestEditorSessionBrowserToolbarButtons sends a real one through the
 // engine), then the boxes of this window's More menu.
 const settledBar=async(check,label)=>{for(let i=0;i<60;i++){const r=await railState(page);if(check(r))return r;await frames(page);}assert.fail(label+': '+JSON.stringify(await railState(page)));};
 await page.evaluate(()=>NetworkToolkitFileEditor.windowAdapter.applyPreferences({...state.editorPreferences,documentMap:true,sidebarPanels:{documents:false}}));
 let bar=await settledBar(r=>!r.rail.includes('documents'),'the batch removes Docs');
 assert.deepEqual([bar.rail,bar.boxes],[['connections','files'],{documents:false,connections:true,files:true}],'a batch with Document list off: Connect and Files only');
 assert.equal(JSON.parse(await page.evaluate(()=>sessionStorage.getItem('ntk.editor.files'))).panel,'documents','the window\'s own panel choice is kept');
 await page.locator('#networkToolkitFileEditor [data-editor-files-panel="files"]').click();
 bar=await settledBar(r=>r.open&&r.files,'Files opens over the text');
 await shot(page,'editor-sidebar-tabs-detached-640x400-dark');
 await page.keyboard.press('Escape');bar=await settledBar(r=>!r.open,'Escape closes the narrow sidebar');
 for(const name of ['connections','files']){await openMore(page);await page.locator('#ntEditorMoreActions [data-ed-sidebar-panel="'+name+'"]').uncheck();await frames(page);}
 await page.keyboard.press('Escape');
 bar=await settledBar(r=>!r.shell,'no sidebar in the small window');
 assert.deepEqual([bar.rail,bar.dock,bar.open],[[],false,false],'all three off at 640 x 400: no rail and no side button');
 g=await geometry();
 const full=await page.evaluate(()=>{const area=document.querySelector('#networkToolkitFileEditor .nt-editor-document-area').getBoundingClientRect(),work=document.querySelector('#networkToolkitFileEditor .nt-editor-workspace').getBoundingClientRect(),map=document.querySelector('#networkToolkitFileEditor [data-ed-map]').getBoundingClientRect(),sep=document.querySelector('#networkToolkitFileEditor [data-ed-map-resize]')?.getBoundingClientRect();return {left:area.left-work.left,width:area.width,room:work.width-map.width-(sep?.width||0)};});
 assert(full.left<=1&&Math.abs(full.width-full.room)<=1&&g.overflow<=0,'the text takes the full width at 640 x 400: '+JSON.stringify({full,g}));
 await shot(page,'editor-sidebar-tabs-detached-all-off-640x400-dark');
 await openMore(page);await page.locator('#ntEditorMoreActions [data-ed-sidebar-panel="connections"]').check();await frames(page);
 bar=await settledBar(r=>r.shell&&r.open&&r.connect,'Connections brings the rail back');
 assert.deepEqual([bar.rail,bar.selected],[['connections'],'connections'],'ticked: the rail is back with Connect open');
 pass('Detached window at 640 x 400: the sidebar tabs follow the batch and the More menu; with all three off the text takes the full width');
 // Closing the last tab leaves an empty editor with nothing of that document
 // (owner report on v1.21.58: no tab and no text, yet the status bar read
 // "Ln 1, Col 1 · 3,932 chars, 116 lines" and the language box "Auto (Aruba
 // AOS-CX config)"). Light theme, Pastel colors and the map, as in the report.
 await page.keyboard.press('Escape');await page.locator('#networkToolkitFileEditor [data-editor-files-panel="connections"]').click();await settledBar(r=>!r.open,'the narrow sidebar closes');
 await page.evaluate(()=>setTheme('light'));await page.locator('#networkToolkitFileEditor [data-ed-palette]').selectOption('pastel');
 const empty=()=>page.evaluate(()=>{const ed=document.querySelector('#networkToolkitFileEditor'),q=s=>ed.querySelector(s);return {tabs:ed.querySelectorAll('[data-ed-tabs] [data-ed-open]').length,text:q('[data-ed-text]').value,gutter:q('[data-ed-gutter-lines]').textContent,position:q('[data-ed-position]').textContent,size:q('[data-ed-size]').textContent,language:q('[data-ed-auto-language]').textContent,message:q('[data-ed-message]').textContent};});
 assert.match((await empty()).size,/ lines$/,'a document is open');
 for(let left=await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length);left>0;left--){await page.locator('#networkToolkitFileEditor [data-ed-tabs] [data-ed-close-tab]').first().click();await page.waitForFunction(n=>NetworkToolkitFileEditor.listDocuments().length===n,left-1);}
 assert.deepEqual(await empty(),{tabs:0,text:'',gutter:'',position:'',size:'',language:'Auto',message:'No open documents.'},'the empty editor shows no line count, position or language of the closed document');
 await shot(page,'editor-no-documents-640x400-light');
 pass('Detached window: closing the last tab leaves no position, line count or language of it in the status bar');
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

 // The whole file in the map, in colour, from the engine's samples; at the
 // widest map too (never only the part that was read).
 for(const key of ['Home','End']){
  await pressSeparator(page,key);
  await page.waitForFunction(()=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l?.kind==='lines'&&l.sampled;},null,{timeout:15000});
  await frames(page);
  const paint=await mapPaint(page);info=await mapInfo(page);
  assert.equal(paint.mode,'blocks');assert.equal(paint.offset,0);assert.equal(info.layout.g.total,d.lines,'the map covers every line');
  assert.equal(paint.bands,20,'every part of the 16 MB file is drawn ('+key+'): '+JSON.stringify(paint));
  assert(paint.hues>=3,'in the syntax colours: '+JSON.stringify(paint));
 }
 await page.evaluate(()=>{mapLongTasks.length=0;});
 info=await mapInfo(page);
 await page.mouse.click(info.left+info.width/2,info.top+info.layout.g.used*0.7);
 await page.waitForFunction(()=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l&&Math.abs((l.g.box.top+l.g.box.height/2)/l.g.used-0.7)<0.02;},null,{timeout:15000});
 await frames(page);await page.waitForTimeout(300);
 {
  const longTasks=await page.evaluate(()=>mapLongTasks);
  assert(longTasks.every(ms=>ms<=50),'no long task over 50 ms at the widest map on a 16 MB file: '+JSON.stringify(longTasks));
  report.disk16MiBWide={longTasks,stats:(await mapInfo(page)).stats};
 }
 await shot(page,'editor-map-list-disk-16mb-wide-dark');
 pass('16 MB disk-backed document: the whole file in the map in colour, at the default and the widest width, with no long task over 50 ms');
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

 // The whole line in the map from the engine's samples, at the widest map too.
 for(const key of ['Home','End']){
  await pressSeparator(page,key);
  await page.waitForFunction(()=>{const l=NetworkToolkitFileEditor.documentMap.layout;return l?.kind==='offsets'&&l.sampled;},null,{timeout:15000});
  await frames(page);
  const paint=await mapPaint(page);
  assert.equal(paint.mode,'blocks');assert.equal(paint.offset,0);
  assert.equal(paint.bands,20,'every part of the 1.7 MB line is drawn ('+key+'): '+JSON.stringify(paint));
 }
 await shot(page,'editor-map-list-base64-wrap-wide-dark');
 pass('1.7 MB single-line Base64 with Wrap on: the whole line in the map at the default and the widest width');

 // The engine keeps the settings: reload. Connections is turned off first.
 await openMore(page);await page.locator('#ntEditorMoreActions [data-ed-sidebar-panel="connections"]').uncheck();await page.keyboard.press('Escape');
 await page.waitForFunction(()=>NetworkToolkitFileEditor.sidebarPanels.connections===false&&document.querySelector('#networkToolkitFileEditor [data-editor-files-panel="connections"]').hidden);
 const keptWidth=Math.round((await mapSize(page)).width);
 await page.waitForFunction(width=>state.editorPreferences?.documentMapWidth===width,keptWidth);
 await page.waitForTimeout(600);
 await start();
 assert.equal(await page.evaluate(()=>NetworkToolkitFileEditor.documentMapOn),true,'the map setting is restored by the engine');
 assert.equal(await page.evaluate(()=>state.editorPreferences?.documentMapWidth),keptWidth,'and its width ('+keptWidth+' px)');
 assert.equal(await page.evaluate(()=>NetworkToolkitEditorFiles.panel),'documents','the Document List is restored by the engine');
 assert.deepEqual(await page.evaluate(()=>[NetworkToolkitFileEditor.sidebarPanels,[...document.querySelectorAll('#networkToolkitFileEditor [data-editor-files-panel]')].filter(b=>!b.hidden).map(b=>b.dataset.editorFilesPanel)]),[{documents:true,connections:false,files:true},['documents','files']],'Connections stays off after a reload');
 pass('Standalone Text Editor: the engine keeps the map, the Document List and Connections off after a reload');
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
