'use strict';
// Synthetic transcript acceptance against real Chromium, xterm and application
// assets. The production enqueue function is unchanged; only its transport is
// a recording fixture. No device, credential, application backend or live shell.
const assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),http=require('node:http');
const root=path.resolve(__dirname,'..'),out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT;
const playwright=require(process.env.PLAYWRIGHT_MODULE||(process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES?path.join(process.env.CODEX_PRIMARY_RUNTIME_NODE_MODULES,'playwright'):'playwright'));
const html=fs.readFileSync(path.join(root,'web/index.html'),'utf8');
const tail=html.slice(html.search(/^function enqueueTerminalInput\(/m)),enqueue=tail.slice(0,tail.slice(1).search(/\n(?:async )?function /)+1);
assert(enqueue.startsWith('function enqueueTerminalInput('),'Production enqueue function must be available');
const report={passed:false,checks:[],errors:[],screenshots:[],scope:'Synthetic transcript/browser acceptance; no live device or native Windows acceptance.'};
let server,browser,page;
const prompt='-- MORE --, next page: Space, next line: Enter, quit: q';
function pass(name){report.checks.push(name);console.log('PASS '+name);}
const fixture=`<!doctype html><meta charset="utf-8"><link rel="stylesheet" href="/assets/xterm.css"><style>body{margin:0;padding:18px;background:#18202c;color:#eee;font:14px sans-serif}h1{font-size:18px}#terminal{background:#202020;padding:8px;width:max-content;max-width:100%}.modal{margin-top:15px;background:#253046;padding:10px}textarea{width:95vw;height:300px}</style><h1>Network Toolkit - synthetic terminal output</h1><div id="terminal"></div><div id="fixtureModal"></div><script>
window.state={activePage:'terminal',terminalFontSize:13,terminalInteraction:{autoPager:true,pagerKey:'auto',pasteReview:false,copyOnSelect:false,copyOnClick:false,ctrlC:'send',rightClick:'paste'}};
window.terminalPaneFocus=0;window.terminalPaneContext='main';window.terminalWorkspaceTransferBusy=false;window.terminalDetachedReturning=false;window.online=true;window.terminalSessions=new Map();window.messages=[];window.sent=[];
window.currentTerminal=()=>window.entry;window.terminalSessionRevision=()=>1;window.terminalOwnerCanMutate=()=>true;window.terminalEntryVisible=()=>true;window.terminalEntryAcceptsInput=e=>e?.info.state==='connected';window.terminalPaneShortcut=()=>false;window.selectTerminal=()=>{};window.saveState=()=>{};window.setTerminalToolsOpen=()=>{};window.icon=()=>'';window.esc=s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;');window.toast=s=>messages.push(s);
window.openModal=(title,body)=>{document.querySelector('#fixtureModal').innerHTML='<section class="modal"><h2>'+esc(title)+'</h2>'+body+'</section>';};window.closeModal=()=>{document.querySelector('#fixtureModal').textContent='';};
window.drainTerminalInput=e=>{for(const bytes of e.inputChunks)sent.push({data:new TextDecoder().decode(bytes),origin:bytes.terminalInputOrigin||'user'});e.inputChunks=[];e.inputBytes=0;};
${enqueue}
</script><script src="/assets/xterm.js"></script><script src="/assets/terminal-output-archive.js"></script><script src="/assets/terminal-extras.js"></script><script src="/assets/terminal-input.js"></script><script src="/assets/terminal-interaction.js"></script><script src="/assets/terminal-macros.js"></script><script>
window.setup=({cols=84,rows=12,scrollback=24,autoPager=true}={})=>{
const term=new Terminal({cols,rows,scrollback,allowProposedApi:true,fontSize:13,theme:{background:'#202020',foreground:'#dce4ef'}});term.open(document.querySelector('#terminal'));
window.entry={info:{id:'synthetic',state:'connected',type:'ssh'},config:{},xterm:term,element:document.querySelector('#terminal'),disposables:[],inputChunks:[],inputBytes:0,_ntSessionPrefs:{autoPager,autoScroll:true}};terminalSessions.set(entry.info.id,entry);
term.onData(data=>enqueueTerminalInput(entry,data));NetworkToolkitTerminalInteraction.bind(entry);NetworkToolkitTerminalInput.bind(entry);term.focus();
};
window.receive=async text=>{const e=entry,api=NetworkToolkitTerminalExtras,position=api.beforeOutput(e);api.observeOutput(e,text);await new Promise(resolve=>e.xterm.write(text,resolve));api.afterOutput(e,position);await api.flush(e);};
window.captured=(choice='last-command',include=false)=>{const api=NetworkToolkitTerminalExtras;return api.output(entry,api.current(entry,choice),include)?.text;};
</script>`;
async function fresh(options,initialPrompt='switch# '){await page.goto('http://127.0.0.1:'+server.address().port+'/',{waitUntil:'load'});await page.evaluate(options=>setup(options),options||{});await receive(initialPrompt);}
async function receive(text){await page.evaluate(text=>receive(text),text);}
async function settle(){await page.waitForTimeout(90);}
async function command(text,{echo=true}={}){await page.keyboard.type(text);if(echo)await receive(text);await page.keyboard.press('Enter');}
async function keys(){await settle();return page.evaluate(()=>sent.filter(x=>x.origin==='pager').map(x=>x.data));}
async function capture(choice='last-command',include=false){return page.evaluate(({choice,include})=>captured(choice,include),{choice,include});}
async function count(){return page.evaluate(()=>entry._ntOutputHistory?.length||0);}
async function shot(name,fullPage=false){if(!out)return;const file=path.join(out,'terminal-pager-'+name+'.png');await page.screenshot({path:file,fullPage});report.screenshots.push(path.basename(file));}
(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 server=http.createServer((req,res)=>{if(req.url==='/'){res.setHeader('Content-Type','text/html');res.end(fixture);return;}const match=req.url.match(/^\/assets\/([a-z0-9.-]+)$/);if(!match){res.writeHead(404).end();return;}const file=path.join(root,'web/assets',match[1]);if(!fs.existsSync(file)){res.writeHead(404).end();return;}res.setHeader('Content-Type',file.endsWith('.css')?'text/css':'text/javascript');res.end(fs.readFileSync(file));});
 await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
 browser=await playwright.chromium.launch({headless:true,executablePath:process.env.CHROME_EXECUTABLE||process.env.CHROMIUM_EXECUTABLE||undefined,args:['--no-sandbox','--disable-dev-shm-usage']});
 const context=await browser.newContext({viewport:{width:1100,height:700},permissions:['clipboard-read','clipboard-write']});page=await context.newPage();page.setDefaultTimeout(15000);page.on('pageerror',e=>report.errors.push(e.message));

 await fresh();
 const settings=async value=>page.evaluate(value=>{
  if(!document.querySelector('#terminalInteractionSettings'))document.body.insertAdjacentHTML('beforeend',NetworkToolkitTerminalInteraction.settingsHTML());
  if(value===null)delete state.terminalInteraction;else state.terminalInteraction=value;
  NetworkToolkitTerminalInteraction.sync();
  const fields={copyOnSelect:'terminalCopyOnSelect',copyOnClick:'terminalCopyOnClick',pasteReview:'terminalPasteReview',pasteAddEnter:'terminalPasteAddEnter',rightClick:'terminalRightClick',ctrlC:'terminalCtrlC',autoPager:'terminalAutoPager',pagerKey:'terminalPagerKey'};
  return Object.fromEntries(Object.entries(fields).map(([key,id])=>{const el=document.getElementById(id);return [key,el.type==='checkbox'?el.checked:el.value];}));
 },value);
 const defaults={copyOnSelect:true,copyOnClick:true,pasteReview:true,pasteAddEnter:false,rightClick:'paste',ctrlC:'copy-selection',autoPager:true,pagerKey:'auto'};
 assert.deepEqual(await settings(null),defaults,'Unset preferences render screenshot-matched defaults in production controls');
 const saved={copyOnSelect:false,copyOnClick:false,pasteReview:false,pasteAddEnter:true,rightClick:'menu',ctrlC:'send',autoPager:false,pagerKey:'enter'};
 assert.deepEqual(await settings(saved),saved,'Explicit saved choices survive normalization and render unchanged');
 assert.deepEqual(await settings({}),defaults,'An empty/reset preference object returns to the same defaults');
 await shot('default-settings');
 pass('Unset and empty preferences render screenshot defaults; explicit saved false/send/menu/enter choices remain unchanged');

 await receive('\r\nCOPY DEFAULT');
 await page.evaluate(async()=>{await navigator.clipboard.writeText('BEFORE COPY');entry.xterm.select(0,1,'COPY DEFAULT'.length);entry.xterm.focus();});
 await page.keyboard.press('Control+c');await page.waitForFunction(()=>!entry.xterm.hasSelection());
 assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),'COPY DEFAULT');await settle();assert.deepEqual(await page.evaluate(()=>sent),[],'Default Ctrl+C with selection does not send an interrupt');
 await page.keyboard.press('Control+c');await settle();assert.deepEqual(await page.evaluate(()=>sent.map(item=>item.data)),['\x03'],'Default Ctrl+C without selection sends one interrupt');assert.equal(await count(),0);
 pass('Native Ctrl+C defaults copy selected text without input, then send a single interrupt when no selection remains');

 await fresh();await command('show running-config');await receive('\r\ninterface 1/1/18\r\n    vlan access 30\r\n');
 for(const chunk of ['\x1b[','7m-- MO','RE --, next page: Spa','ce, next line: Enter, quit: q','\x1b[0m'])await receive(chunk);
 assert.deepEqual(await keys(),[' ']);await shot('aruba-cx-prompt');
 const expected=['interface 1/1/18','    vlan access 30'];
 for(let i=0;i<3;i++){const line='PAGE '+i;expected.push(line);await receive('\r\x1b[2K'+line+'\r\n'+prompt);assert.equal((await keys()).length,i+2,'Each new prompt advances once, even with identical suffix and rapid pages');}
 await receive('\r\x1b[2KFINAL PAGE\r\nswitch# ');expected.push('FINAL PAGE');
 assert.equal(await capture(),expected.join('\n'));assert.equal(await count(),1);
 await receive('\x1b[0m\x07');assert.equal((await keys()).length,4,'Idle prompt and control-only output do not re-advance');
 pass('Screenshot-specific Aruba CX prompt, fragmented ANSI/text, repeated pages and prompt return preserve one complete command');

 await fresh();await command('show version');await receive('\r\nVERSION\r\n--More--');assert.deepEqual(await keys(),[' ']);await receive('\r\x1b[2KSECOND\r\n<--- More --->');assert.deepEqual(await keys(),[' ',' ']);
 await page.evaluate(()=>entry._ntSessionPrefs.pagerKey='enter');await receive('\r\x1b[2KTHIRD\r\n'+"-- MORE --, next page: Space, next line: Enter, quit: Control-C");assert.deepEqual(await keys(),[' ',' ','\r']);
 pass('Cisco IOS/ASA and ArubaOS-Switch-style prompts use default Space and explicit Enter override');

 for(const key of ['Space','Enter','q','Control+c']){
  await fresh({autoPager:false});await command('show paged');await receive('\r\nBEFORE\r\n'+prompt);await page.keyboard.press(key);await settle();assert.equal(await count(),1);assert.equal(await page.evaluate(()=>!!entry._ntInputDraft),false,'Manual '+key+' must not start another command draft');
  await receive('\r\x1b[2KAFTER\r\nswitch# ');assert.equal(await capture(),'BEFORE\nAFTER');assert.deepEqual(await keys(),[]);
 }
 pass('Native Space, Enter, q and Ctrl+C pass through manual paging without splitting output blocks');

 await fresh({autoPager:false});await command('show noecho',{echo:false});await receive('\r\nFIRST WITHOUT ECHO\r\nSECOND WITHOUT ECHO\r\nswitch# ');assert.equal(await capture(),'FIRST WITHOUT ECHO\nSECOND WITHOUT ECHO');
 pass('A command without remote input echo retains its first output line');

 for(const gesture of ['keyboard','mouse']){
  await fresh({autoPager:false});await page.evaluate(async()=>navigator.clipboard.writeText('show one\nshow two\n'));
  if(gesture==='keyboard')await page.keyboard.press('Control+v');else await page.locator('#terminal').click({button:'right',position:{x:70,y:30}});
  await page.waitForFunction(()=>entry._ntOutputHistory?.length===1&&entry._ntOutputHistory[0].kind==='paste');
  await receive('show one\r\nFIRST PASTED OUTPUT\r\n'+prompt);await page.keyboard.press('Space');await receive('\r\x1b[2KCONTINUED FIRST\r\nswitch# show two\r\nSECOND PASTED OUTPUT\r\nswitch# ');
  assert.equal(await count(),1);assert.equal(await capture('last-paste'),'FIRST PASTED OUTPUT\nCONTINUED FIRST\nSECOND PASTED OUTPUT');assert.equal(await capture('last-command'),undefined);
  assert.match(await capture('last-paste',true),/switch# show one\nFIRST PASTED OUTPUT/);
 }
 pass('Native Ctrl+V and right-click multiline paste retain ordered echoes and all manually paged output');
 // End multiline pastes with Enter (owner request after v1.21.79): the user's
 // own multiline paste gets a final Enter; Send to terminal never does.
 {
  const typed=async()=>{await settle();return page.evaluate(()=>sent.filter(x=>x.origin!=='pager').map(x=>x.data).join(''));};
  await fresh({autoPager:false});await page.evaluate(()=>{state.terminalInteraction={...state.terminalInteraction,pasteAddEnter:true};sent.length=0;});
  await page.evaluate(async()=>navigator.clipboard.writeText('show one\nshow two'));await page.keyboard.press('Control+v');
  assert.equal(await typed(),'show one\rshow two\r','the last command of an own multiline paste is sent with Enter');
  await page.evaluate(()=>{sent.length=0;NetworkToolkitTerminalInteraction.pasteText(entry,'show three\nshow four');});
  assert.equal(await typed(),'show three\rshow four','text sent to the terminal from another tool never gets an Enter');
  await page.evaluate(()=>{sent.length=0;state.terminalInteraction={...state.terminalInteraction,pasteAddEnter:false};});
  await page.evaluate(async()=>navigator.clipboard.writeText('show five\nshow six'));await page.keyboard.press('Control+v');
  assert.equal(await typed(),'show five\rshow six','with the setting off a paste is sent as it was copied');
 }
 pass('End multiline pastes with Enter adds Enter only to the user\'s own multiline paste');


 // ESXi's prompt ends with ']', not a traditional shell terminator. Exercise
 // the real parsed screen/archive and the macro adapter's verified captures;
 // neither the synthetic transport nor the capture endpoint runs a command.
 const esxiHome='[root@lab-esxi:~] ',esxiStore='[root@lab-esxi:/vmfs/volumes/Lab Store] ';
 const esxiCommands=["lsof | grep -i 'LabVM'","ps -c | grep -i 'LabVM'"];
 const esxiResponses=['/vmfs/volumes/Lab Store/LabVM/LabVM.vmdk\n[completed]','72516 72516 vmx LabVM'];
 const outputOnly=esxiResponses.join('\n');
 const rawWithCommands=esxiHome+esxiCommands[0]+'\n'+esxiResponses[0]+'\n'+esxiStore+esxiCommands[1]+'\n'+esxiResponses[1];
 const capturedWithCommands=esxiCommands[0]+'\n'+esxiResponses[0]+'\n'+esxiCommands[1]+'\n'+esxiResponses[1];
 const esxiTranscript=esxiCommands[0]+'\r\n'+esxiResponses[0].replaceAll('\n','\r\n')+'\r\n'+esxiStore+esxiCommands[1]+'\r\n'+esxiResponses[1]+'\r\n'+esxiHome;
 const labels={off:'Output with recognized command echoes excluded',on:'Output with command lines included'};
 const gapHint='The output is separated by other terminal lines. Select it here without including those lines.';
 const captureHint='The captured output differs from the text still in the terminal. Select or copy it here.';
 const clipboardText=text=>text.replace(/\n/g,process.platform==='win32'?'\r\n':'\n');
 async function selectAndCopyESXi({include=false,choice='last-paste',expected,modal,label,hint,name}){
  const before=await page.evaluate(()=>sent.length);
  assert.equal(await page.evaluate(({include,choice,name})=>{
   closeModal();entry.xterm.clearSelection();entry._ntSessionPrefs.pasteIncludeCommands=include;entry._ntOutputChoice=choice;
   document.querySelector('h1').textContent='Network Toolkit - '+name.replaceAll('-',' ');
   return NetworkToolkitTerminalExtras.select(entry);
  },{include,choice,name}),true);
  if(modal){
   await page.locator('#terminalSelectedOutput').waitFor();
   assert.equal(await page.locator('#terminalSelectedOutput').inputValue(),expected);
   assert.deepEqual(await page.locator('#terminalSelectedOutput').evaluate(el=>[el.selectionStart,el.selectionEnd]),[0,expected.length]);
   if(label)assert.equal(await page.locator('label[for="terminalSelectedOutput"]').innerText(),label);
   if(hint)assert((await page.locator('#fixtureModal').innerText()).includes(hint),'The selection explains why it uses a separate text window');
  }else{
   assert.equal(await page.locator('#terminalSelectedOutput').count(),0,'Contiguous matching output stays selected in the terminal');
   await page.waitForFunction(()=>entry.xterm.hasSelection());
   // Native complete-line selection may include its last newline. All other
   // characters, command echoes and output boundaries must match exactly.
   assert.equal((await page.evaluate(()=>entry.xterm.getSelection())).replace(/\r\n?/g,'\n').replace(/\n$/,''),expected);
  }
  assert.equal(await page.evaluate(()=>NetworkToolkitTerminalExtras.copy(entry)),true);
  assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),clipboardText(expected));
  assert.equal(await page.evaluate(()=>sent.length),before,'Selecting and copying output never sends terminal input');
  await shot(name,true);
 }

 await fresh({autoPager:false},esxiHome);
 await page.evaluate(async text=>navigator.clipboard.writeText(text),esxiCommands.join('\n')+'\n');
 await page.keyboard.press('Control+v');
 await page.waitForFunction(()=>entry._ntOutputHistory?.length===1&&entry._ntOutputHistory[0].kind==='paste');
 await receive(esxiTranscript);
 assert.equal(await capture('last-paste'),outputOnly);
 assert.equal(await capture('last-paste',true),rawWithCommands);
 await selectAndCopyESXi({expected:outputOnly,modal:true,label:labels.off,hint:gapHint,name:'esxi-raw-output-only'});
 await selectAndCopyESXi({include:true,expected:rawWithCommands,modal:false,name:'esxi-raw-include-commands'});
 pass('ESXi raw two-command paste excludes both bracketed echoes and final prompt; Include commands restores displayed echoes, unrelated bracketed output remains, and matching output selects directly when contiguous');

 for(const source of ['paced','macro']){
  await fresh({autoPager:false},esxiHome);
  await page.evaluate(text=>{window.fixtureBatch=NetworkToolkitTerminalExtras.notePaste(entry,text);},esxiCommands.join('\n')+'\n');
  await receive(esxiTranscript);
  const runId='esxi-'+source,steps=[
   {index:1,command:esxiCommands[0],output:esxiCommands[0]+'\r\n'+esxiResponses[0].replaceAll('\n','\r\n')+'\r\n'+esxiStore},
   {index:2,command:esxiCommands[1],output:esxiStore+esxiCommands[1]+'\r\n'+esxiResponses[1]+'\r\n'+esxiHome}
  ];
  assert.equal(await page.evaluate(({source,runId,steps})=>{
   const run={id:runId,macroId:source==='paced'?'literal-paste':'saved-esxi-macro',state:'completed',targets:[{id:entry.info.id,generation:7,state:'completed',step:2,total:2}]};
   const capture={runId,sessionId:entry.info.id,state:'completed',steps};window.captureCalls=[];
   window.api=async path=>{captureCalls.push(path);if(path!=='/terminal-automation/runs/'+runId+'/capture?sessionId='+entry.info.id)throw Error('Unexpected fixture request '+path);return {capture};};
   return NetworkToolkitTerminalMacros.trackPacedPaste(entry,fixtureBatch,run);
  },{source,runId,steps}),true);
  await page.waitForFunction(()=>fixtureBatch.pacedCapture?.steps.length===2);
  assert.deepEqual(await page.evaluate(()=>captureCalls),['/terminal-automation/runs/'+runId+'/capture?sessionId=synthetic']);
  assert.equal(await capture('last-paste'),outputOnly);
  assert.equal(await capture('last-paste',true),capturedWithCommands);
  assert.equal(await capture('last-command'),esxiResponses[1]);
  assert.equal(await capture('last-command',true),esxiCommands[1]+'\n'+esxiResponses[1]);
  await selectAndCopyESXi({expected:outputOnly,modal:true,label:labels.off,hint:gapHint,name:'esxi-'+source+'-output-only'});
  await selectAndCopyESXi({include:true,expected:capturedWithCommands,modal:true,label:labels.on,hint:captureHint,name:'esxi-'+source+'-include-commands'});
  await selectAndCopyESXi({choice:'last-command',expected:esxiResponses[1],modal:true,label:labels.off,hint:'This output is retained separately from the console scrollback.',name:'esxi-'+source+'-last-command'});
  await selectAndCopyESXi({include:true,choice:'last-command',expected:esxiCommands[1]+'\n'+esxiResponses[1],modal:true,label:labels.on,hint:'This output is retained separately from the console scrollback.',name:'esxi-'+source+'-last-command-included'});
 }
 pass('Verified paced and saved-macro ESXi captures preserve every output block, include each submitted command once, copy the selected preference, and explain noncontiguous or captured-text selection without falsely labeling included commands as excluded');

 await fresh({autoPager:false},esxiHome);await command(esxiCommands[0]);
 await receive('\r\n'+esxiResponses[0].replaceAll('\n','\r\n')+'\r\n'+esxiHome);
 await selectAndCopyESXi({choice:'last-command',expected:esxiResponses[0],modal:false,name:'esxi-contiguous-output-only'});
 pass('A contiguous ESXi command response selects directly in xterm and copies only the output, without opening a separate selection window');


 // A prompt that already wraps before typing has its boundary on the first
 // logical row, not on the cursor's final physical row. Starting the next
 // command must close the earlier block before that entire wrapped prompt.
 const wrappedStart='[root@lab-esxi:/vmfs/volumes/Engineering Test Store/Initial VM] ';
 const wrappedNext='[root@lab-esxi:/vmfs/volumes/Engineering Test Store/Second VM Folder] ';
 const priorResponse='FIRST RESPONSE\nKEEP FIRST DETAIL',nextResponse='SECOND RESPONSE\nKEEP SECOND DETAIL';
 await fresh({cols:36,rows:12,scrollback:24,autoPager:false},wrappedStart);
 assert(await page.evaluate(()=>entry.xterm.buffer.normal.cursorY>0),'The initial ESXi prompt must already wrap before command input');
 await command('echo FIRST');await receive('\r\n'+priorResponse.replaceAll('\n','\r\n')+'\r\n'+wrappedNext);
 const priorBlock=await page.evaluate(()=>NetworkToolkitTerminalExtras.current(entry,'last-command').id);
 assert.equal(await capture(priorBlock),priorResponse,'The first response is clean before another command starts');
 await command('echo SECOND');await receive('\r\n'+nextResponse.replaceAll('\n','\r\n')+'\r\n'+esxiHome);
 assert.equal(await page.evaluate(id=>NetworkToolkitTerminalExtras.current(entry,id).closed,priorBlock),true);
 assert.equal(await capture(priorBlock),priorResponse,'Closing the first block keeps every response line and excludes the entire next wrapped prompt');
 assert.equal(await capture(priorBlock,true),wrappedStart+'echo FIRST\n'+priorResponse);
 assert.equal(await capture('last-command'),nextResponse,'The final response excludes the wrapped echo and final idle prompt');
 assert.equal(await capture('last-command',true),wrappedNext+'echo SECOND\n'+nextResponse);
 await selectAndCopyESXi({choice:'last-command',expected:nextResponse,modal:false,name:'esxi-wrapped-datastore-output'});
 pass('Wrapped initial and changed ESXi datastore prompts preserve both command responses across block finalization, remove complete prompt boundaries, and retain direct output selection');

 await fresh({cols:46,rows:8,scrollback:12,autoPager:false});await command('show archive');
 const lines=Array.from({length:420},(_,i)=>'line '+String(i).padStart(3,'0')+' '+('wide 界 '.repeat(i%3+1)).trimEnd());
 await receive('\r\n');
 // At 46 columns this pager occupies two physical rows. Model its complete
 // erase/redraw, not an invalid one-row erase that leaves half a prompt behind.
 for(let i=0;i<lines.length;i+=35){await receive(lines.slice(i,i+35).map(s=>'\x1b[32m'+s+'\x1b[0m').join('\r\n')+'\r\n'+prompt);await page.keyboard.press(i%70?'Enter':'Space');await receive('\r\x1b[2K\x1b[1A\x1b[2K');}
 await receive('switch# ');assert.equal(await count(),1);assert.equal(await capture(),lines.join('\n'));assert.equal(await page.evaluate(()=>entry._ntOutputHistory[0].start.isDisposed),true);
 assert.equal(await page.evaluate(()=>NetworkToolkitTerminalExtras.select(entry)),true);assert.equal(await page.locator('#terminalSelectedOutput').inputValue(),lines.join('\n'));assert.deepEqual(await page.locator('#terminalSelectedOutput').evaluate(el=>[el.selectionStart,el.selectionEnd]),[0,lines.join('\n').length]);
 await page.evaluate(()=>NetworkToolkitTerminalExtras.copy(entry));assert.equal(await page.evaluate(()=>navigator.clipboard.readText()),lines.join(process.platform==='win32'?'\r\n':'\n'));await shot('retained-output-selection');
 await page.evaluate(()=>closeModal());await page.evaluate(()=>NetworkToolkitTerminalExtras.jump(entry,'end'));assert.equal(await page.locator('#terminalSelectedOutput').evaluate(el=>el.value.slice(el.selectionStart,el.selectionEnd)),lines.at(-1));
 pass('Narrow wrapped ANSI output survives scrollback trimming, manual paging, complete selection/copy and accurate end navigation');

 await fresh();await command('show safe');
 for(const text of ['ordinary text has more information','description --More--','Delete configuration? [yes/no]','Erase data? Press Enter to continue','More: really delete? [y/n]']){await receive('\r\x1b[2K'+text);await settle();}
 assert.deepEqual(await keys(),[]);pass('Ordinary more text and yes/no/destructive confirmation prompts never auto-advance');
 assert.deepEqual(report.errors,[],'Browser page errors');report.passed=true;
})().catch(error=>{report.errors.push(error.stack||String(error));console.error(error.stack||String(error));process.exitCode=1;}).finally(async()=>{
 if(out){fs.writeFileSync(path.join(out,'terminal-pager-report.json'),JSON.stringify(report,null,2)+'\n');if(page&&!report.passed)await page.screenshot({path:path.join(out,'terminal-pager-failure.png')}).catch(()=>{});}
 if(browser)await browser.close();if(server)await new Promise(resolve=>server.close(resolve));
});
