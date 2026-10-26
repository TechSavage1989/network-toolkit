'use strict';
// Real app assets/API, native xterm selection/keys and a loopback Telnet line
// editor. No application function, asset or response is replaced.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),os=require('node:os'),net=require('node:net'),{spawn}=require('node:child_process'),{StringDecoder}=require('node:string_decoder');
const help='Usage: NTK_APP_EXECUTABLE=/path/to/app [PLAYWRIGHT_MODULE=playwright] [NTK_BROWSER_OUTPUT_DIR=/path/to/results] node scripts/terminal-selection-live.cjs';
if(process.argv.includes('--help')){console.log(help);process.exit(0);}if(!process.env.NTK_APP_EXECUTABLE){console.error(help);process.exit(2);}
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright'),delay=ms=>new Promise(resolve=>setTimeout(resolve,ms)),temporary=fs.mkdtempSync(path.join(os.tmpdir(),'ntk-selection-')),sockets=new Set(),out=process.env.NTK_BROWSER_OUTPUT_DIR;
const report={passed:false,checks:[],errors:[],assetFailures:[],submitted:[]},prompt='switch# ',previous='Earlier output stays here';
let app,browser,page,peer,log='',editor,cols=80;
function pass(name){report.checks.push(name);console.log('PASS '+name);}
function makePeer(){return net.createServer(socket=>{
 sockets.add(socket);socket.on('error',()=>{});socket.on('close',()=>sockets.delete(socket));
 const state={text:'',cursor:0,wire:''},decoder=new StringDecoder('utf8');editor=state;
 let negotiation=0,sub=false,subIAC=false,escape='';
 function draw(){const cell=prompt.length+state.cursor;socket.write('\x1b[1;1H\x1b[2K'+previous+'\r\n\x1b[J'+prompt+state.text+'\x1b['+(2+Math.floor(cell/cols))+';'+(1+cell%cols)+'H');}
 draw();
 function input(character){
  state.wire+=character;
  if(escape){
   escape+=character;
   if(/^\x1b(?:\[|O)?$/.test(escape))return;
   if(/^\x1b(?:\[|O)[CD]$/.test(escape)){state.cursor=Math.max(0,Math.min(state.text.length,state.cursor+(character==='D'?-1:1)));escape='';return;}
   if(/^\x1b\[[0-9;]*$/.test(escape))return;
   escape='';return;
  }
  if(character==='\x1b'){escape=character;return;}
  if(character==='\x03'){state.text='';state.cursor=0;return;}
  if(character==='\r'){report.submitted.push(state.text);state.text='';state.cursor=0;return;}
  if(character==='\x7f'||character==='\b'){if(state.cursor){state.text=state.text.slice(0,state.cursor-1)+state.text.slice(state.cursor);state.cursor--;}return;}
  if(character<' ')return;
  state.text=state.text.slice(0,state.cursor)+character+state.text.slice(state.cursor);state.cursor++;
 }
 socket.on('data',data=>{
  const text=[];
  for(const byte of data){
   if(sub){if(subIAC&&byte===240){sub=false;subIAC=false;}else subIAC=byte===255;continue;}
   if(negotiation===1){if(byte===250){sub=true;negotiation=0;}else negotiation=[251,252,253,254].includes(byte)?2:0;continue;}
   if(negotiation===2){negotiation=0;continue;}if(byte===255){negotiation=1;continue;}text.push(byte);
  }
  for(const character of decoder.write(Buffer.from(text)))input(character);
  draw();
 });
});}
async function start(){
 peer=makePeer();await new Promise(resolve=>peer.listen(0,'127.0.0.1',resolve));
 app=spawn(path.resolve(process.env.NTK_APP_EXECUTABLE),['-no-browser','-port','0','-data-dir',path.join(temporary,'data')],{stdio:['ignore','pipe','pipe']});let error;app.on('error',e=>error=e);app.stdout.on('data',data=>log+=data);app.stderr.on('data',data=>log+=data);
 for(let i=0;i<150;i++){if(error)throw error;if(app.exitCode!==null)throw new Error(log);const base=log.match(/http:\/\/127\.0\.0\.1:\d+\//)?.[0];if(base){try{if((await fetch(base)).ok)return base;}catch(_){}}await delay(100);}throw new Error('App did not start: '+log);
}
async function settled(text,cursor=text.length){
 await page.waitForFunction(({text,cursor,prompt,previous})=>{
  const entry=currentTerminal(),term=entry?.xterm,b=term?.buffer.normal;if(!b)return false;
  const count=Math.max(1,Math.ceil((prompt.length+text.length)/term.cols)),shown=Array.from({length:count},(_,i)=>b.getLine(1+i)?.translateToString(false)||'').join('');
  return !entry.inputSending&&!entry.inputBytes&&!entry.renderPendingBytes&&b.getLine(0)?.translateToString(true)===previous&&shown.slice(0,prompt.length+text.length)===prompt+text&&b.baseY+b.cursorY===1+Math.floor((prompt.length+cursor)/term.cols)&&b.cursorX===(prompt.length+cursor)%term.cols;
 },{text,cursor,prompt,previous});
 assert.equal(editor.text,text);assert.equal(editor.cursor,cursor);
}
async function fitted(){
 await page.waitForFunction(()=>{
  const entry=currentTerminal(),term=entry?.xterm,size=entry?.fit?.proposeDimensions();
  return size&&term.cols===size.cols&&term.rows===size.rows&&!entry.resizeSending&&!entry.desiredResize&&entry.info.cols===term.cols&&entry.info.rows===term.rows&&entry.lastResize===term.cols+'x'+term.rows;
 });
 cols=await page.evaluate(()=>currentTerminal().xterm.cols);
}
async function command(text){
 await fitted();
 await page.evaluate(()=>currentTerminal().xterm.focus());await page.keyboard.press('Control+c');await settled('');editor.wire='';
 await page.keyboard.type(text);await settled(text);editor.wire='';
}
async function select(start,end){
 await page.evaluate(({start,end})=>{const term=currentTerminal().xterm;term.focus();term.select(start%term.cols,1+Math.floor(start/term.cols),end-start);},{start,end});
 assert(await page.evaluate(()=>currentTerminal().xterm.hasSelection()));
}
async function screenshot(name){if(out)await page.screenshot({path:path.join(out,'terminal-selection-'+name+'.png')});}
(async()=>{
 const base=await start();browser=await chromium.launch({executablePath:process.env.CHROME_EXECUTABLE||undefined,headless:true,args:['--no-sandbox','--disable-dev-shm-usage']});
 const context=await browser.newContext({viewport:{width:1450,height:1000},permissions:['clipboard-read','clipboard-write']});page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',error=>report.errors.push(error.message));page.on('response',r=>{if(r.url().includes('/assets/')&&r.status()>=400)report.assetFailures.push(r.url());});
 if(out)fs.mkdirSync(out,{recursive:true});await page.goto(base,{waitUntil:'domcontentloaded'});await page.click('[data-page="terminal"]');await page.locator('#terminalStage [data-terminal-new-type="telnet"]').first().click();await page.fill('#con-host','127.0.0.1');await page.fill('#con-port',String(peer.address().port));await page.click('#terminalConnectButton');await page.waitForFunction(()=>currentTerminal()?.info.state==='connected'&&currentTerminal()?.xterm);
 await fitted();assert(cols>35);await settled('');
 for(const key of ['Backspace','Delete']){
  await command('show version');await select(13,cols-2);await screenshot(key+'-selected');await page.keyboard.press(key);await settled('show ');
  assert.equal(editor.wire,'\x7f'.repeat(7));assert.equal(await page.evaluate(()=>currentTerminal().xterm.hasSelection()),false);pass(key+' deletes selected text plus empty screen cells without deleting extra characters');
 }
 await command('show version');
 const points=await page.evaluate(()=>{const t=currentTerminal().xterm,r=currentTerminal().element.querySelector('.xterm-screen').getBoundingClientRect(),w=r.width/t.cols,h=r.height/t.rows;return {x:r.left+13*w+.1,y:r.top+1.5*h,end:r.left+(t.cols-2)*w+.1};});
 await page.mouse.move(points.x,points.y);await page.mouse.down();await page.mouse.move(points.end,points.y,{steps:8});await page.mouse.up();
 const drag=await page.evaluate(()=>currentTerminal().xterm.getSelectionPosition());assert.equal(drag.start.x,13);assert(drag.end.x>20);await page.keyboard.press('Delete');await settled('show ');pass('Native mouse drag with trailing padding follows the same deletion path');
 await command('show version');for(let i=0;i<3;i++)await page.keyboard.press('ArrowLeft');await settled('show version',9);editor.wire='';await select(13,cols-2);await page.keyboard.press('Delete');await settled('show ');assert.equal(editor.wire,'\x1b[C'.repeat(3)+'\x7f'.repeat(7));pass('Padded deletion uses the actual device cursor after moving within the command');
 const wrapped='show '+ 'x'.repeat(cols+7);
 await command(wrapped);await select(13,2*cols);await page.keyboard.press('Backspace');await settled('show ');assert.equal(editor.wire,'\x7f'.repeat(wrapped.length-5));pass('Wrapped command selection ending at the next row boundary deletes only command text');
 await command('show version   ');await select(13,cols-2);await page.keyboard.press('Delete');await settled('show ');assert.equal(editor.wire,'\x7f'.repeat(10));pass('Trailing spaces before the device cursor are deleted without counting empty cells');
 await command('show version');await select(13,cols-2);await page.keyboard.type('x');await settled('show x');assert.equal(editor.wire,'\x7f'.repeat(7)+'x');pass('Typing replaces selected text when the highlight includes empty cells');
 await command('show version');await page.evaluate(()=>navigator.clipboard.writeText('other'));await select(13,cols-2);await page.keyboard.press('Control+v');await settled('show other');assert.equal(editor.wire,'\x7f'.repeat(7)+'other');pass('Native clipboard paste replaces selected text with trailing padding');
 await command('show version');
 const protectedSelections=await page.evaluate(()=>{const entry=currentTerminal(),t=entry.xterm,results=[];for(const [x,y,length] of [[0,0,t.cols+20],[0,1,25],[21,1,10],[13,1,t.cols]]){t.select(x,y,length);results.push(NetworkToolkitTerminalInput.deleteSelection(entry));}t.clearSelection();return results;});
 assert.deepEqual(protectedSelections,[false,false,false,false]);assert.equal(editor.wire,'');await settled('show version');pass('Previous output, prompts, empty-only and later hard-line selections cannot trigger selection edits');
 for(const theme of ['light','dark']){await page.evaluate(theme=>setTheme(theme),theme);await screenshot('fixed-'+theme);}
 assert.deepEqual(report.submitted,[]);assert.deepEqual(report.errors,[]);assert.deepEqual(report.assetFailures,[]);report.passed=true;console.log(JSON.stringify(report));
})().catch(async error=>{
 report.failure=error.stack||String(error);report.peer=editor;console.error(report.failure);process.exitCode=1;
 if(page){report.terminal=await page.evaluate(()=>{const e=currentTerminal(),t=e?.xterm,b=t?.buffer.normal;return {cols:t?.cols,rows:t?.rows,cursorX:b?.cursorX,cursorY:b?.cursorY,baseY:b?.baseY,selection:t?.getSelectionPosition(),anchor:e&&NetworkToolkitTerminalInput.snapshot(e),fit:e?.fit?.proposeDimensions(),lines:b&&Array.from({length:4},(_,i)=>b.getLine(i)?.translateToString(false)),inputBytes:e?.inputBytes};}).catch(()=>null);await screenshot('failure').catch(()=>{});}
}).finally(async()=>{
 if(out)fs.writeFileSync(path.join(out,'terminal-selection-report.json'),JSON.stringify(report,null,2)+'\n');if(browser)await browser.close();if(app)app.kill();for(const socket of sockets)socket.destroy();if(peer)await new Promise(resolve=>peer.close(resolve));fs.rmSync(temporary,{recursive:true,force:true});
});
