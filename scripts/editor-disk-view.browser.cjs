'use strict';
// Isolated synthetic viewport acceptance, not full application/native acceptance.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||path.join(root,'test-results/editor-disk-view');
fs.mkdirSync(out,{recursive:true});
const settle=page=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>setTimeout(resolve,30)))));

// Word wrap over one 1.7 MB line without line breaks (a synthetic Base64
// settings export): a vertical bar covers the whole line, the wheel and the
// bar reach its last characters, and selections continue past page edges.
async function verifyLongLineWrap(page,editor){
 const units=1748322,pageUnits=49152;
 await page.evaluate(async units=>{
  view.dispose();session.close();
  const letters='ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789+/';
  let seed=0x2545f491;
  const random=()=>{seed^=seed<<13;seed^=seed>>>17;seed^=seed<<5;return (seed>>>0)/4294967296;};
  const parts=new Array(units-2);
  for(let i=0;i<parts.length;i++)parts[i]=letters[(random()*64)|0];
  const text=window.base64=parts.join('')+'==';
  session=NTKEditorDiskSession.create({request:async(action,p)=>{
   if(action!=='window')throw Error('Unexpected long-line request '+action);
   const start=p.line!==undefined?0:p.start||0;
   return {revision:1,units,bytes:units,lines:1,start,line:0,column:start,lineUnits:units,text:text.slice(start,Math.min(units,start+p.limit))};
  }});
  session.initialize({revision:1,units,bytes:units,lines:1});
  window.wrapLongLine=true;
  // Editable, so the keyboard moves the caret (a read-only textarea has
  // none); nothing here edits, and the fixture refuses edits.
  view=NTKEditorDiskView.create({textarea:document.querySelector('textarea'),gutter:document.getElementById('lines'),
   container:document.querySelector('.nt-editor-code'),session,wordWrap:()=>wrapLongLine,
   message:(text,error)=>{if(error)throw Error(text);}});
  await view.load({start:0});
 },units);
 const bar=page.getByRole('scrollbar',{name:'Document position'});
 await bar.waitFor();
 assert.equal(await page.locator('.nt-editor-disk-horizontal').isVisible(),false,'word wrap needs no bar below the line');
 const state=()=>page.evaluate(()=>{
  const ta=document.querySelector('textarea'),bar=document.querySelector('.nt-editor-disk-scroll'),{start,text}=session.state().page;
  return {offset:view.capture().offset,start,end:start+text.length,top:ta.scrollTop,bottom:ta.scrollTop+ta.clientHeight>=ta.scrollHeight-2,
   value:Number(bar.getAttribute('aria-valuenow')),barTop:bar.scrollTop,spacer:bar.firstChild.offsetHeight,tail:ta.value.slice(-64)};
 });
 const atEnd=value=>value.end===units&&value.bottom&&value.value===100;
 let value=await state();
 assert.ok(value.spacer<1000000&&value.spacer>value.barTop,'the thumb reflects the visible share of the line');
 const tail=await page.evaluate(()=>base64.slice(-64));

 // The wheel scrolls through all of the text, never back and never skipping.
 await editor.hover();
 const wheel={events:0,slides:0,maxStep:0};
 for(let previous=value;wheel.events<400&&!atEnd(value);wheel.events++){
  await page.mouse.wheel(0,2400);
  if(wheel.events%4===3){
   await settle(page);
   value=await state();
   assert.ok(value.offset>=previous.offset,'wheel moved the view back: '+previous.offset+' to '+value.offset);
   assert.ok(value.barTop>=previous.barTop,'the bar did not follow the wheel');
   if(value.start!==previous.start)wheel.slides++;
   wheel.maxStep=Math.max(wheel.maxStep,value.offset-previous.offset);
   previous=value;
  }
 }
 await settle(page);value=await state();
 assert.ok(atEnd(value),'the wheel reached the end: '+JSON.stringify(value));
 assert.equal(value.tail,tail,'the last characters of the line are shown');
 assert.ok(wheel.slides>10,'the page slid along the line');
 await page.screenshot({path:path.join(out,'editor-long-line-wrap-end.png')});

 // The bar: Home, a click on the track, and a drag to the end.
 await bar.press('Home');
 await page.waitForFunction(()=>view.capture().offset===0&&session.state().page.start===0);
 const track=await bar.boundingBox();
 await page.mouse.click(track.x+track.width/2,track.y+track.height-24);
 await page.waitForFunction(()=>view.capture().offset>0);
 for(const share of [.25,.5,.75,1]){
  await bar.evaluate((el,share)=>{el.scrollTop=(el.scrollHeight-el.clientHeight)*share;},share);
  await page.waitForFunction(share=>{
   const value=Number(document.querySelector('.nt-editor-disk-scroll').getAttribute('aria-valuenow'));
   return Math.abs(value-share*100)<=1;
  },share);
 }
 await page.waitForFunction(units=>{
  const ta=document.querySelector('textarea'),{start,text}=session.state().page;
  return start+text.length===units&&ta.scrollTop+ta.clientHeight>=ta.scrollHeight-2;
 },units);
 assert.equal((await state()).tail,tail);

 // A mouse selection continues past the end of a page while the pointer
 // stays below the text, and keeps its anchor.
 await bar.press('Home');
 await page.waitForFunction(()=>view.capture().offset===0);
 const area=await editor.boundingBox();
 await page.mouse.move(area.x+40,area.y+16);
 await page.mouse.down();
 await page.mouse.move(area.x+80,area.y+area.height+30,{steps:4});
 await page.waitForFunction(pageUnits=>view.selection().end>pageUnits*1.5&&session.state().page.start>0,pageUnits,{timeout:30000});
 await page.mouse.up();
 const dragged=await page.evaluate(()=>view.selection());
 assert.ok(dragged.start<200&&dragged.end>pageUnits*1.5,'drag selection crossed the page edge: '+JSON.stringify(dragged));
 await settle(page);
 assert.deepEqual(await page.evaluate(()=>{const {start,end}=view.selection();return {start,end};}),{start:dragged.start,end:dragged.end});

 // Shift+click extends a selection whose anchor has scrolled out of the page.
 await page.mouse.click(area.x+60,area.y+16);
 const anchor=await page.evaluate(()=>view.selection().start);
 await bar.press('End');
 await page.waitForFunction(units=>session.state().page.start+session.state().page.text.length===units,units);
 await settle(page);
 await page.keyboard.down('Shift');
 await page.mouse.click(area.x+60,area.y+area.height/2);
 await page.keyboard.up('Shift');
 const extended=await page.evaluate(()=>view.selection());
 assert.equal(extended.start,anchor,'Shift+click kept the anchor');
 assert.ok(extended.end>units-pageUnits,'Shift+click extended to the end: '+JSON.stringify(extended));

 // Shift+PageDown and Shift+ArrowDown continue past a page edge.
 await editor.press('Control+Home');
 await page.waitForFunction(()=>view.selection().end===0&&session.state().page.start===0);
 for(let i=0;i<60&&(await page.evaluate(()=>view.selection().end))<pageUnits*1.2;i++)await editor.press('Shift+PageDown');
 await editor.press('Shift+ArrowDown');
 await settle(page);
 const keyed=await page.evaluate(()=>view.selection());
 assert.equal(keyed.start,0);
 assert.ok(keyed.end>pageUnits*1.2,'Shift+PageDown crossed the page edge: '+JSON.stringify(keyed));

 // Ctrl+End and Ctrl+Home still reach both ends of the line.
 await editor.press('Control+End');
 await page.waitForFunction(units=>view.selection().end===units&&view.cursor.column===units,units);
 await editor.press('Control+Home');
 await page.waitForFunction(()=>view.selection().end===0&&view.capture().offset===0);

 // Without word wrap the bar below the line is visible and moves the view.
 await page.evaluate(()=>{wrapLongLine=false;view.refreshWrap();});
 const below=page.getByRole('scrollbar',{name:'Position in long line'});
 await below.waitFor();
 const [lineBar,surface]=await Promise.all([below.boundingBox(),page.locator('.nt-editor-surface').boundingBox()]);
 assert.ok(lineBar.height>=12&&Math.abs(lineBar.x-surface.x)<=1&&Math.abs(lineBar.width-surface.width)<=1,'the line bar spans the text');
 await below.evaluate(el=>{el.scrollLeft=(el.scrollWidth-el.clientWidth)/2;});
 await page.waitForFunction(units=>Math.abs(session.state().page.start-units/2)<units/50,units);
 return wheel;
}

(async()=>{
 const browser=await chromium.launch({headless:true,ignoreDefaultArgs:['--hide-scrollbars'],...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
 const report={passed:false,scope:'Isolated paged renderer with an 8 GiB synthetic logical document; no physical-file or full-application claim.',errors:[]};
 try{
  const page=await browser.newPage({viewport:{width:960,height:600}});page.on('pageerror',error=>report.errors.push(error.message));
  await page.setContent('<style>body{margin:0;background:#202535;color:#e5eaf4;font-family:sans-serif}main{height:530px;display:flex;flex-direction:column;margin:20px}textarea{color:white}h1{font-size:16px} .nt-editor-gutter{width:60px;flex-shrink:0;overflow:hidden;text-align:right} .nt-editor-gutter>div{padding:10px 8px 20px}</style><main><h1>Network Toolkit — generated large-file viewport</h1><div class="nt-editor-code"><div class="nt-editor-gutter"><div id="lines"></div></div><div class="nt-editor-surface"><textarea aria-label="Editable file contents" wrap="off" spellcheck="false"></textarea></div></div><p id="status"></p></main>');
  await page.addStyleTag({path:path.join(root,'web/assets/file-editor.css')});
  await page.addScriptTag({path:path.join(root,'web/assets/editor-disk-session.js')});
  await page.addScriptTag({path:path.join(root,'web/assets/editor-disk-view.js')});
  await page.evaluate(async()=>{
   const base=8*1024*1024*1024,row='0123456789012345678901234567890\n';
   let pieces=[{start:0,length:base}],revision=1,saved=pieces,undo=[],redo=[];const receipts=new Map();
   window.requests=[];window.longTasks=[];new PerformanceObserver(list=>longTasks.push(...list.getEntries().map(e=>e.duration))).observe({type:'longtask',buffered:true});
   const size=()=>pieces.reduce((n,p)=>n+p.length,0),read=(from,to)=>{let offset=0,result='';for(const p of pieces){const a=Math.max(0,from-offset),b=Math.min(p.length,to-offset);if(b>a){if(p.text!==undefined)result+=p.text.slice(a,b);else{const at=(p.start+a)%row.length;result+=row.repeat(Math.ceil((at+b-a)/row.length)).slice(at,at+b-a);}}offset+=p.length;if(offset>=to)break;}return result;};
   const slice=(from,to)=>{let offset=0,result=[];for(const p of pieces){const a=Math.max(0,from-offset),b=Math.min(p.length,to-offset);if(b>a)result.push(p.text===undefined?{start:p.start+a,length:b-a}:{text:p.text.slice(a,b),length:b-a});offset+=p.length;}return result;};
   const metadata=()=>({revision,units:size(),bytes:size(),lines:base/32+1,dirty:pieces!==saved,canUndo:!!undo.length,canRedo:!!redo.length});
   window.session=NTKEditorDiskSession.create({request:async(action,p)=>{requests.push({action,...p});
    if(action==='window'){const start=p.line!==undefined?p.line*32:p.start||0;return {...metadata(),start,line:Math.floor(start/32),column:start%32,lineUnits:31,text:read(start,Math.min(size(),start+p.limit))};}
    if(action==='edit'){if(receipts.has(p.requestId))return receipts.get(p.requestId);if(revision!==p.revision)throw Error('Stale edit');undo.push(pieces);redo=[];pieces=[...slice(0,p.from),{text:p.text,length:p.text.length},...slice(p.to,size())];revision++;const result=metadata();receipts.set(p.requestId,result);return result;}
    if(action==='undo'||action==='redo'){const from=action==='undo'?undo:redo,to=action==='undo'?redo:undo;to.push(pieces);pieces=from.pop();revision++;return metadata();}
    if(action==='save'){saved=pieces;return {metadata:metadata(),hash:'fixture'};}
    throw Error('Unexpected '+action);
   }});
   session.initialize(metadata());window.tail=()=>read(size()-40,size());window.view=NTKEditorDiskView.create({textarea:document.querySelector('textarea'),gutter:document.getElementById('lines'),container:document.querySelector('.nt-editor-code'),session,changed:()=>{document.getElementById('status').textContent=session.state().metadata.units.toLocaleString()+' characters';},message:(text,error)=>{if(error)throw Error(text);}});await view.load({start:0});
  });
  await page.evaluate(()=>view.goLine(1000000,true));
  assert.equal(await page.evaluate(()=>view.cursor.line),1000000);
  const edge=await page.evaluate(()=>{
   const {start,text}=session.state().page;
   const caret=start+text.length-3;view.select(caret,caret,'none',true);
   return {caret,end:start+text.length};
  });
  await page.locator('textarea').press('Shift+ArrowDown');
  await page.waitForFunction(({caret,end})=>view.selection().start===caret&&view.selection().end>end,edge);
  const copied=await page.evaluate(async()=>{
   const from=50000000,to=from+100000;
   await view.position(from,to);
   let written='';
   Object.defineProperty(window,'ClipboardItem',{configurable:true,value:class{constructor(data){this.data=data;}}});
   Object.defineProperty(navigator,'clipboard',{configurable:true,value:{async write(items){written=await (await items[0].data['text/plain']).text();}}});
   await view.copy();
   const before=session.state().metadata.units;
   navigator.clipboard.write=async()=>{throw Error('Fixture clipboard refused the selection');};
   try{await view.copy(true);}catch(error){if(!error.message.includes('refused'))throw error;}
   return {length:written.length,first:written.slice(0,32),unchanged:before===session.state().metadata.units};
  });
  assert.deepEqual(copied,{length:100000,first:'0123456789012345678901234567890\n',unchanged:true});
  await page.locator('textarea').press('Control+Shift+End');
  await page.waitForFunction(()=>view.selection().start===50000000&&view.selection().end===8*1024*1024*1024);
  await page.locator('textarea').press('Control+End');await page.waitForFunction(()=>view.selection().start===8*1024*1024*1024);
  await page.locator('textarea').pressSequentially('TAIL');await page.evaluate(()=>session.flush());
  await page.locator('textarea').press('Control+z');await page.evaluate(()=>session.flush());
  await page.locator('textarea').press('Control+y');await page.evaluate(()=>session.flush());
  await page.evaluate(()=>session.save('fixture.txt',{encoding:'UTF-8',eol:'LF'}));
  assert.ok((await page.evaluate(()=>tail())).endsWith('TAIL'));
  report.metrics=await page.evaluate(()=>({pageUnits:document.querySelector('textarea').value.length,gutterNodes:document.getElementById('lines').childElementCount,largestEdit:Math.max(0,...requests.filter(r=>r.action==='edit').map(r=>r.text.length)),fullDocumentUnits:session.state().metadata.units,longTasks:longTasks.filter(ms=>ms>50)}));
  assert.ok(report.metrics.pageUnits<=49156);assert.ok(report.metrics.gutterNodes<50);assert.ok(report.metrics.largestEdit<=4);assert.equal(report.errors.length,0);
  await page.screenshot({path:path.join(out,'editor-disk-view.png')});
  await page.evaluate(async()=>{
   view.dispose();session.close();
   const size=8*1024*1024*1024;
   session=NTKEditorDiskSession.create({request:async(action,p)=>{
    if(action!=='window')throw Error('Unexpected long-line request '+action);
    const start=p.start||0;
    return {revision:1,units:size,lines:1,start,line:0,column:start,lineUnits:size,text:'x'.repeat(Math.min(p.limit,size-start))};
   }});
   session.initialize({revision:1,units:size,lines:1});
   window.wrapLongLine=false;
   view=NTKEditorDiskView.create({textarea:document.querySelector('textarea'),gutter:document.getElementById('lines'),container:document.querySelector('.nt-editor-code'),session,wordWrap:()=>wrapLongLine,readOnly:true});
   await view.load({start:0});
  });
  await page.getByRole('scrollbar',{name:'Position in long line'}).press('End');
  await page.waitForFunction(()=>session.state().page.start>8*1024*1024*1024-1000);
  await page.locator('textarea').press('Control+End');
  await page.waitForFunction(()=>view.cursor.column===8*1024*1024*1024);
  const editor=page.locator('textarea');
  assert.ok(await editor.evaluate(ta=>ta.readOnly&&ta.value.length<=49152));
  await editor.press('Control+Home');
  await page.waitForFunction(()=>session.state().page.start===0);
  await page.evaluate(()=>{wrapLongLine=true;view.refreshWrap();});
  await page.waitForFunction(()=>{
   const editor=document.querySelector('textarea'),gutter=document.getElementById('lines');
   return editor.scrollHeight>2*editor.clientHeight&&editor.scrollWidth<=editor.clientWidth+1&&gutter.textContent==='1';
  });
  await editor.evaluate(ta=>{ta.scrollTop=ta.scrollHeight;});
  await editor.hover();
  await page.mouse.wheel(0,120);
  await page.waitForFunction(()=>session.state().page.start>0&&document.getElementById('lines').textContent.trim()==='');
  // With word wrap the document bar covers the long line; no bar below it.
  assert.equal(await page.getByRole('scrollbar',{name:'Section of long source line'}).count(),0);
  await page.getByRole('scrollbar',{name:'Document position'}).press('End');
  await page.waitForFunction(()=>{
   const ta=document.querySelector('textarea'),{start,text}=session.state().page;
   return start+text.length===8*1024*1024*1024&&ta.scrollTop+ta.clientHeight>=ta.scrollHeight-2;
  });
  assert.ok(await editor.evaluate(ta=>ta.value.length<=49152));

  // Native textarea selection and visible syntax text share a baseline deep
  // inside a page, where fractional line heights can accumulate error.
  await page.evaluate(async()=>{
   view.dispose();session.close();
   const text=Array.from({length:1024},(_,i)=>'set alias FIXTURE_'+String(i).padStart(4,'0')).join('\n');
   const metadata={revision:1,units:text.length,lines:1024};
   session=NTKEditorDiskSession.create({request:async(action,p)=>{
    if(action!=='window')throw Error('Unexpected syntax fixture request '+action);
    const start=p.start||0, prefix=text.slice(0,start);
    return {...metadata,start,line:prefix.split('\n').length-1,column:start-prefix.lastIndexOf('\n')-1,
     lineUnits:22,text:text.slice(start,start+p.limit)};
   }});
   session.initialize(metadata);
   window.NTKEditorLanguageService={highlight:async source=>({html:source.replace(/FIXTURE_\d{4}/g,
    token=>'<span class="nt-ed-string">'+token+'</span>')})};
   const container=document.querySelector('.nt-editor-code');
   container.style.fontSize='13px';
   view=NTKEditorDiskView.create({textarea:document.querySelector('textarea'),gutter:document.getElementById('lines'),
    container,session,syntaxLanguage:()=> 'fixture',readOnly:true});
   await view.load({start:0});
   await view.goLine(846);
  });
  const token=page.locator('.nt-editor-disk-highlight .nt-ed-string').filter({hasText:'FIXTURE_0850'});
  await token.waitFor();
  const box=await token.boundingBox();
  await page.mouse.dblclick(box.x+box.width/2,box.y+box.height*.8);
  assert.equal(await editor.evaluate(ta=>ta.value.slice(ta.selectionStart,ta.selectionEnd)),'FIXTURE_0850');

  report.longLineWrap=await verifyLongLineWrap(page,editor);
  assert.equal(report.errors.length,0);report.passed=true;
 }finally{fs.writeFileSync(path.join(out,'editor-disk-view-report.json'),JSON.stringify(report,null,2));await browser.close();}
 console.log(JSON.stringify(report));
})().catch(error=>{console.error(error);process.exitCode=1;});
