'use strict';
// The shared expected byte streams are checked against the production Go
// normalizer in ssh_banner_test.go. Here they reach the actual bundled xterm
// renderer unchanged, including byte-sized WebSocket-style fragmentation.
// This is synthetic compatibility coverage, not a captured device session.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const root=path.resolve(__dirname,'..'),out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT;
const playwright=require(process.env.PLAYWRIGHT_MODULE||(process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES?path.join(process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES,'playwright'):'playwright'));
const fixtures=JSON.parse(fs.readFileSync(path.join(root,'scripts/fixtures/ssh-banner-lines.json'),'utf8'));
const banner=fixtures.cases.find(item=>item.name==='aruba-style-synthetic');
assert(banner,'Screenshot-shaped synthetic banner is required');
const app=fs.readFileSync(path.join(root,'web/index.html'),'utf8');
assert.match(app,/convertEol:false/,'The application must not globally translate normal PTY line feeds');
const report={passed:false,checks:[],errors:[],screenshots:[],layouts:[],scope:'Synthetic SSH authentication-banner bytes rendered by bundled xterm; no device, credential, application backend or native Windows acceptance.'};
let server,browser,page;
function pass(message){report.checks.push(message);console.log('PASS '+message);}
function bytes(text,size){const value=Buffer.from(text);if(!size)return [Array.from(value)];const chunks=[];for(let i=0;i<value.length;i+=size)chunks.push(Array.from(value.subarray(i,i+size)));return chunks;}
const fixture=`<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="/assets/xterm.css"><style>
*{box-sizing:border-box}body{margin:0;padding:12px;background:#18202c;color:#e5edf7;font:13px sans-serif}h1{margin:0 0 9px;font-size:18px}p{margin:0 0 12px;color:#afc0d5;font-size:12px}#terminal{padding:7px;background:#202020;width:max-content;max-width:100%;border:1px solid #55647b}
</style><h1>Network Toolkit — SSH login banner</h1><p id="caption">Synthetic screenshot-shaped fixture; no switch connection or password.</p><div id="terminal"></div><script src="/assets/xterm.js"></script><script>
window.setup=({cols=170,rows=34}={})=>{
 if(window.term)term.dispose();document.querySelector('#terminal').textContent='';
 window.term=new Terminal({cols,rows,scrollback:200,convertEol:false,allowProposedApi:true,fontSize:13,fontFamily:'Consolas, "Courier New", monospace',lineHeight:1.15,theme:{background:'#202020',foreground:'#e5edf7'}});
 term.open(document.querySelector('#terminal'));term.focus();
};
window.receive=async chunks=>{for(const chunk of chunks)await new Promise(resolve=>term.write(new Uint8Array(chunk),resolve));};
window.snapshot=()=>{
 const b=term.buffer.active,lines=[];for(let i=0;i<b.length;i++){const line=b.getLine(i);lines.push({text:line.translateToString(true),wrapped:line.isWrapped});}
 return {lines,cursorX:b.cursorX,cursorY:b.cursorY,baseY:b.baseY,cols:term.cols,rows:term.rows};
};
</script>`;
async function setup(layout){await page.evaluate(layout=>setup(layout),layout);}
async function receive(text,size){await page.evaluate(chunks=>receive(chunks),bytes(text,size));}
async function snapshot(){return page.evaluate(()=>snapshot());}
async function render(text,layout,size){await setup(layout);await receive(text,size);return snapshot();}
async function shot(name){if(!out)return;await page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(resolve))));const file='terminal-banner-'+name+'.png';await page.screenshot({path:path.join(out,file),fullPage:true});report.screenshots.push(file);}
function findLine(state,text){return state.lines.findIndex(line=>line.text.includes(text));}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 server=http.createServer((req,res)=>{
  if(req.url==='/'){res.setHeader('Content-Type','text/html');res.end(fixture);return;}
  if(!['/assets/xterm.js','/assets/xterm.css'].includes(req.url)){res.writeHead(404).end();return;}
  res.setHeader('Content-Type',req.url.endsWith('.css')?'text/css':'text/javascript');res.end(fs.readFileSync(path.join(root,'web',req.url)));
 });
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 browser=await playwright.chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE||process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox','--disable-dev-shm-usage']});
 page=await browser.newPage({viewport:{width:1440,height:860}});page.setDefaultTimeout(15000);page.on('pageerror',error=>report.errors.push(error.message));
 await page.goto('http://127.0.0.1:'+server.address().port+'/',{waitUntil:'load'});
 for(const layout of [{name:'normal',width:1440,height:860,cols:170,rows:34},{name:'narrow',width:390,height:960,cols:44,rows:43}]){
  await page.setViewportSize({width:layout.width,height:layout.height});
  for(const item of fixtures.cases){
   const whole=await render(item.normalized,layout);
   assert.deepEqual(await render(item.normalized,layout,1),whole,item.name+' renders identically across every UTF-8/control byte boundary ('+layout.name+')');
   assert.deepEqual(await render(item.normalized,layout,7),whole,item.name+' renders identically in uneven network chunks ('+layout.name+')');
  }
  pass('All shared Go-verified banner fixtures render identically whole, byte-by-byte and chunked at '+layout.cols+' columns');

  const state=await render(banner.normalized,layout,1),legend=findLine(state,'RESTRICTED RIGHTS LEGEND');
  assert(legend>=0,'The deliberately indented legend is visible');
  assert.equal(state.lines[legend].text.indexOf('RESTRICTED RIGHTS LEGEND'),20,'Preserve intended banner indentation rather than accumulating the previous cursor column');
  const confidential=findLine(state,'Confidential computer software.');
  assert.equal(state.lines[confidential].text.indexOf('Confidential computer software.'),1,'The following logical line starts at its one-space indentation');
  assert.equal(state.lines[findLine(state,'We would like')].text.indexOf('We would like'),0,'A later logical line resets to the left edge');
  await receive("\r\nPlease configure the 'admin' user account password.\r\nEnter new password: ");
  const login=await snapshot(),passwordLine=findLine(login,'Enter new password:');
  assert.equal(login.lines[passwordLine].text.indexOf('Enter new password:'),0,'The following normal PTY prompt stays left aligned');
  assert.equal(login.cursorX,'Enter new password: '.length);
  assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,'Normal/narrow screenshot has no horizontal page overflow');
  await shot(layout.name+'-login');
  report.layouts.push({name:layout.name,cols:layout.cols,legendRow:legend,legendColumn:20,passwordRow:passwordLine,passwordColumn:0});
  pass('Screenshot-shaped '+layout.name+' login banner preserves intended columns, wraps and the subsequent password prompt');

  // Deliberately raw PTY output, after authentication: a bare LF must retain
  // column 3. A global convertEol=true workaround would fail this assertion.
  await receive('\r\n\x1b[32mSTATUS UP\x1b[0m\r\nraw\nNEXT\r\nswitch# ',1);
  const shell=await snapshot(),raw=findLine(shell,'raw'),next=raw+1,status=findLine(shell,'STATUS UP');
  assert.equal(shell.lines[raw].text,'raw');assert.equal(shell.lines[next].text,'   NEXT','Normal PTY bare LF is not rewritten');
  assert.equal(shell.lines[findLine(shell,'switch#')].text,'switch# ');assert.equal(shell.cursorX,'switch# '.length);
  assert.deepEqual(await page.evaluate(row=>{const c=term.buffer.active.getLine(row).getCell(0);return {palette:!!c.isFgPalette(),color:c.getFgColor()};},status),{palette:true,color:2},'Normal PTY ANSI color is preserved');
  pass('Post-banner '+layout.name+' PTY output retains bare LF, ANSI color and shell-prompt cursor semantics');
 }

 await page.setViewportSize({width:1440,height:860});
 const layout={cols:170,rows:34},raw=await render(banner.chunks.join(''),layout),fixed=await render(banner.normalized,layout);
 const rawLegend=findLine(raw,'RESTRICTED RIGHTS LEGEND'),fixedLegend=findLine(fixed,'RESTRICTED RIGHTS LEGEND');
 assert(raw.lines[rawLegend].text.indexOf('RESTRICTED RIGHTS LEGEND')>20,'The unnormalized LF negative control reproduces cumulative rightward indentation');
 assert.notDeepEqual(raw,fixed,'The regression control must genuinely differ from the corrected banner');
 await render(banner.chunks.join(''),layout);
 await page.locator('#caption').evaluate(el=>el.textContent='Negative control: original LF-only banner reproduces the staircase; not the corrected output.');
 await shot('lf-negative-control');
 pass('Unnormalized LF-only negative control reproduces the screenshot-class staircase');
 assert.deepEqual(report.errors,[],'Browser page errors');report.passed=true;
})().catch(error=>{report.errors.push(error.stack||String(error));console.error(error.stack||String(error));process.exitCode=1;}).finally(async()=>{
 if(out){fs.writeFileSync(path.join(out,'terminal-banner-report.json'),JSON.stringify(report,null,2)+'\n');if(page&&!report.passed)await page.screenshot({path:path.join(out,'terminal-banner-failure.png'),fullPage:true}).catch(()=>{});}
 if(browser)await browser.close();if(server)await new Promise(resolve=>server.close(resolve));
});
