'use strict';
// Optional development check. Install Playwright separately; no download runs
// here. CHROME_EXECUTABLE=/path/to/chrome-headless-shell node
// scripts/terminal-browser-scroll.cjs [--output-dir /path/to/results]
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const args=process.argv.slice(2);
if(args.includes('--help')){console.log('Usage: node scripts/terminal-browser-scroll.cjs [--output-dir PATH]\nOptional CHROME_EXECUTABLE selects an installed Chrome/Headless Shell.');process.exit(0);}
assert(args.length===0||args.length===2&&args[0]==='--output-dir','Expected --output-dir PATH');
const output=args[1]||process.env.NTK_BROWSER_OUTPUT_DIR;
const outputDirectory=output?path.resolve(output):null;
if(outputDirectory)fs.mkdirSync(outputDirectory,{recursive:true});
function loadPlaywright(){
 try{return require('playwright');}
 catch(error){
  if(error.code!=='MODULE_NOT_FOUND')throw error;
  if(process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES)return require(path.join(process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES,'playwright'));
  throw new Error('Optional browser check requires Playwright installed in the development environment. No browser or dependency is downloaded by this script.');
 }
}
const reports=[];let browser,page;
async function frame(){await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(resolve)));}
async function inspect(label){
 return page.evaluate(label=>{
  const term=window.term,highlight=window.highlight,b=term.buffer.active,missing=[];
  const palette=Object.fromEntries(NTTerminalHighlight.legend('dark',{underlines:false}).map(item=>[item.kind,'rgb('+item.color.slice(1).match(/../g).map(value=>parseInt(value,16)).join(', ')+')']));
  const elements=term.element.querySelector('.xterm-rows').children;
  for(let offset=0;offset<term.rows;offset++){
   const text=b.getLine(b.viewportY+offset)?.translateToString(true)||'',hits=NTTerminalHighlight.analyze(text);
   const cells=Array.from(elements[offset].querySelectorAll('span')).flatMap(span=>Array.from(span.textContent,()=>getComputedStyle(span).color));
   const mismatches=hits.filter(hit=>cells.slice(hit.start,hit.end).length!==hit.end-hit.start||cells.slice(hit.start,hit.end).some(color=>color!==palette[hit.kind]));
   if(mismatches.length)missing.push({offset,row:b.viewportY+offset,text,kinds:mismatches.map(hit=>hit.kind)});
  }
  // Iteration consumes pending native insert/delete work before validating the
  // real renderer index, rather than trusting application-owned mark records.
  const entries=Array.from(term._core._decorationService.decorations),keys=entries.map(item=>item.marker.line);
  const index={count:entries.length,disposed:entries.filter(item=>item.marker.isDisposed||item.marker.line<0).length,sorted:keys.every((key,i)=>i===0||keys[i-1]<=key)};
  return{label,first:b.viewportY,missing,index,expectedCount:highlight.getState().count};
 },label);
}
async function check(label){
 const result=await inspect(label);reports.push(result);
 console.log(JSON.stringify({label,first:result.first,missing:result.missing.length,index:result.index}));
 assert.equal(result.missing.length,0,label+': incorrect computed token colors '+JSON.stringify(result.missing.slice(0,6)));
 assert.equal(result.index.disposed,0,label+': disposed markers remained in the native index');
 assert.equal(result.index.sorted,true,label+': native decoration index is not sorted');
 assert.equal(result.index.count,result.expectedCount,label+': native and application decoration counts differ');
}
(async()=>{
 browser=await loadPlaywright().chromium.launch({executablePath:process.env.CHROME_EXECUTABLE||undefined,headless:true,args:['--disable-dev-shm-usage']});
 page=await browser.newPage({viewport:{width:1334,height:1044}});
 const errors=[];page.on('pageerror',error=>errors.push(error.message));
 await page.setContent('<!DOCTYPE html><style>body{margin:0;background:#eee}#term{width:1310px;height:900px;padding:8px;background:#141414}</style><div id="term"></div>');
 const assets=path.resolve(__dirname,'../web/assets');
 await page.addStyleTag({path:path.join(assets,'xterm.css')});
 for(const name of ['xterm.js','terminal-highlight.js'])await page.addScriptTag({path:path.join(assets,name)});
 await page.evaluate(async()=>{
  window.term=new Terminal({cols:180,rows:52,scrollback:10000,fontSize:13,lineHeight:1,allowProposedApi:true,theme:{background:'#141414',foreground:'#eeeeee'}});
  term.open(document.querySelector('#term'));
  window.highlight=NTTerminalHighlight.attach(term,{underlines:false});
  await new Promise(resolve=>term.write(Array.from({length:1800},(_,i)=>'        set snmp-index '+i).join('\r\n'),resolve));
 });
 await page.waitForTimeout(600);await check('initial output');
 for(const destination of [1500,1478,1490,1450,1460,1400,1380,1420,1400,1360]){
  await page.evaluate(y=>term.scrollToLine(y),destination);await frame();await check('direction change '+destination);
 }
 for(let i=0;i<12;i++){
  await page.evaluate(()=>term.scrollLines(-25));await frame();await check('upward scroll '+i);
 }
 await page.mouse.move(500,400);await page.mouse.wheel(0,-1100);await page.waitForTimeout(150);await check('native wheel');
 const scrollbar=await page.evaluate(()=>{
  const v=term.element.querySelector('.xterm-viewport'),r=v.getBoundingClientRect();
  return{x:r.right-7,y:r.top+v.scrollTop/v.scrollHeight*v.clientHeight+Math.max(10,v.clientHeight*v.clientHeight/v.scrollHeight/2),targetY:r.top+r.height*.4,first:term.buffer.active.viewportY};
 });
 await page.mouse.move(scrollbar.x,scrollbar.y);await page.mouse.down();await page.mouse.move(scrollbar.x,scrollbar.targetY,{steps:8});await page.mouse.up();await page.waitForTimeout(150);
 assert.notEqual(await page.evaluate(()=>term.buffer.active.viewportY),scrollbar.first,'Scrollbar drag did not move the viewport');await check('native scrollbar');
 for(let i=0;i<15;i++){
  await page.evaluate(async i=>{term.scrollToLine(800-i*17);await new Promise(resolve=>term.write('\r\n        set snmp-index '+(1800+i),resolve));},i);
  await frame();await check('scroll during output '+i);
 }
 await page.evaluate(()=>term.refresh(0,term.rows-1));await frame();await check('explicit refresh');
 await page.evaluate(()=>highlight.setEnabled(false));await page.waitForTimeout(100);
 const disabled=await page.evaluate(()=>({count:highlight.getState().count,colored:Array.from(term.element.querySelectorAll('.xterm-rows span')).filter(span=>getComputedStyle(span).color!=='rgb(238, 238, 238)').length}));
 assert.deepEqual(disabled,{count:0,colored:0},'Disabling highlights left semantic colors');
 await page.evaluate(()=>highlight.setEnabled(true));await page.waitForTimeout(150);await check('highlight reenabled');
 assert.deepEqual(errors,[],'Browser page errors');
 console.log('PASS: '+reports.length+' browser checks; every recognized token has the expected computed color.');
})().catch(error=>{console.error(error.stack||String(error));process.exitCode=1;}).finally(async()=>{
 if(outputDirectory){
  fs.writeFileSync(path.join(outputDirectory,'terminal-browser-scroll-report.json'),JSON.stringify({passed:!process.exitCode,reports},null,2)+'\n');
  if(page)await page.screenshot({path:path.join(outputDirectory,'terminal-browser-scroll.png')}).catch(()=>{});
 }
 if(browser)await browser.close();
});
