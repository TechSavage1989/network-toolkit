'use strict';
// Isolated synthetic viewport acceptance, not full application/native acceptance.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||path.join(root,'test-results/editor-disk-view');
fs.mkdirSync(out,{recursive:true});
(async()=>{
 const browser=await chromium.launch({headless:true,...(process.env.TOOLKIT_BROWSER_EXECUTABLE?{executablePath:process.env.TOOLKIT_BROWSER_EXECUTABLE}:{})});
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
   view=NTKEditorDiskView.create({textarea:document.querySelector('textarea'),gutter:document.getElementById('lines'),container:document.querySelector('.nt-editor-code'),session,readOnly:true});
   await view.load({start:0});
  });
  await page.getByRole('scrollbar',{name:'Position in long line'}).press('End');
  await page.waitForFunction(()=>session.state().page.start>8*1024*1024*1024-1000);
  await page.locator('textarea').press('Control+End');
  await page.waitForFunction(()=>view.cursor.column===8*1024*1024*1024);
  assert.ok(await page.locator('textarea').evaluate(ta=>ta.readOnly&&ta.value.length<=49152));
  assert.equal(report.errors.length,0);report.passed=true;
 }finally{fs.writeFileSync(path.join(out,'editor-disk-view-report.json'),JSON.stringify(report,null,2));await browser.close();}
 console.log(JSON.stringify(report));
})().catch(error=>{console.error(error);process.exitCode=1;});
