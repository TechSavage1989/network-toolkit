'use strict';
// Owner request: each standalone app (Network Toolkit Text Editor, Network Toolkit Terminal) shows
// only what belongs to it, plus what connects it to the other standalone app. Nothing visible that
// does nothing; nothing from another tool. The main Toolkit window is unchanged.
// Also an owner requirement: the standalone Terminal still opens Command Prompt and PowerShell,
// exactly as the main window does.
// Production assets in Chromium with intercepted synthetic data; no connections, shells, devices,
// files or credentials are used. Not native Windows WebView2 acceptance.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const root=path.resolve(__dirname,'..'),version=require('../package.json').version,out=process.env.NTK_BROWSER_OUTPUT_DIR||process.env.TOOLKIT_TEST_OUTPUT,origin='https://standalone-apps.toolkit.test';
const report={passed:false,checks:[],errors:[],screenshots:[],scope:'Synthetic production-asset browser acceptance in Chromium. Not native Windows WebView2 acceptance.'};
const pass=text=>{report.checks.push(text);console.log('PASS '+text);};
const session={id:'s1',label:'lab-switch-01',type:'ssh',host:'192.0.2.10',port:22,username:'operator',state:'connected',cols:100,rows:28};
let browser,page,fixture;
async function shot(name){if(!out)return;const file='standalone-apps-'+name+'.png';await page.screenshot({path:path.join(out,file)});report.screenshots.push(file);}

async function open(mode,options={}){
 fixture={mode,calls:[],opens:0,saved:{appUpdatePreferences:{enabled:false},theme:'light',activePage:options.activePage||(mode==='terminal'?'terminal':mode==='editor-standalone'?'editor':'overview'),terminalSidebarCollapsed:false,welcomeDismissed:true,firstLaunchWelcome:{dismissed:true}}};
 const f=fixture,sessions=options.sessions||[];
 const context=await browser.newContext({viewport:{width:1280,height:800},serviceWorkers:'block'});page=await context.newPage();page.setDefaultTimeout(15000);
 page.on('pageerror',error=>report.errors.push(mode+': '+error.message));
 await page.route('**/*',async route=>{if(await handleEditorActivityMetadata(route,origin,report))return;
  const req=route.request(),url=new URL(req.url()),p=url.pathname,method=req.method();assert.equal(url.origin,origin,'No external requests are allowed');
  if(p.startsWith('/api/')){let data={};const body=()=>{try{return req.postDataJSON();}catch{return null;}};f.calls.push(method+' '+p);
   if(p==='/api/state'&&method==='GET')data=f.saved;else if(p==='/api/state')f.saved=body();
   else if(p==='/api/status')data={version,os:'windows',arch:'amd64',windowMode:mode,storageWritable:true,localShellAvailable:true,terminalWindowAvailable:true,editorLaunchAvailable:mode==='editor-standalone',editorLaunchReusable:mode==='editor-standalone',system:{hostname:'Standalone fixture',interfaces:[]}};
   else if(p==='/api/system')data={hostname:'Standalone fixture',interfaces:[]};
   else if(p==='/api/jobs')data=[];
   else if(p==='/api/terminals')data={sessions};
   else if(p==='/api/terminals/events'){await new Promise(resolve=>setTimeout(resolve,80));data={events:[],sessions,after:0};}
   else if(p==='/api/terminal-views')data={available:false,views:[],owners:{}};
   else if(p==='/api/terminal-window/requests')data={requests:[],latest:0,oldest:0};
   else if(p==='/api/editor-launch')data={requests:[]};
   else if(p==='/api/activity')data={tools:{}};
   else if(p==='/api/credential-vault')data={available:false,state:'unavailable'};
   else if(p==='/api/credentials')data={vault:{available:false,state:'unavailable'},entries:[]};
   else if(p==='/api/remote-browser'&&method==='GET')data={sessions:[]};
   else if(p==='/api/remote-browser'){const b=body()||{};
    // A synthetic SFTP subsystem for the Terminal's SFTP sidebar.
    if(b.action==='probe')data={supported:true};else if(b.action==='list')data={path:'/home/operator',entries:[{name:'running-config.txt',path:'/home/operator/running-config.txt',size:21},{name:'backups',path:'/home/operator/backups',directory:true}]};
    else if(b.action==='read-text')data={content:'hostname lab-switch-01\n',hash:'synthetic',encoding:'UTF-8',bom:false};
    else return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture: not available'})});}
   else if(p==='/api/peer-send/inbox')data={request:null,report:false};
   else if(p==='/api/peer-send/report')data={ok:true};
   else if(p==='/api/peer-send/targets')data={app:mode==='terminal'?'editor':'terminal',name:mode==='terminal'?'Network Toolkit Text Editor':'Network Toolkit Terminal',receiver:f.opens?'standalone':'',running:f.opens>0,ready:f.opens>0,targets:[]};
   else if(p==='/api/peer-send/open'){f.opens++;data={receiver:'standalone',running:true,ready:true};}
   else if(/\/api\/terminals\/[^/]+\/resize$/.test(p))data={};
   else if(p.startsWith('/api/storage'))data={rootDir:'C:\\Synthetic\\Network Toolkit',defaultDir:'C:\\Synthetic\\Network Toolkit',explicit:false};
   else if(method!=='GET')return route.fulfill({status:503,contentType:'application/json',body:JSON.stringify({error:'Synthetic fixture: not available'})});
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});}
  let file=path.join(root,'web',p==='/'?'index.html':p);if(!fs.existsSync(file))file=path.join(root,p);if(!fs.existsSync(file))return route.fulfill({status:404,body:''});let bytes=fs.readFileSync(file);
  if(p==='/')bytes=bytes.toString().replaceAll('__SESSION_TOKEN__','standalone-apps').replaceAll('__WINDOW_MODE__',mode).replaceAll('__TERMINAL_VIEW__','null').replaceAll('__EDITOR_VIEW__','null');
  return route.fulfill({body:bytes,contentType:p==='/'?'text/html':p.endsWith('.js')?'application/javascript':p.endsWith('.css')?'text/css':p.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});});
 await page.goto(origin+'/');await page.waitForFunction(()=>__networkToolkitStartupState===1&&online);
 return context;
}
const visible=selector=>page.evaluate(s=>{const el=document.querySelector(s);if(!el)return false;const style=getComputedStyle(el);return style.display!=='none'&&style.visibility!=='hidden'&&el.getClientRects().length>0;},selector);
const palette=()=>page.evaluate(()=>NetworkToolkitCommands.catalog().map(item=>item.group+' | '+item.label));
const helpRows=()=>page.evaluate(()=>NetworkToolkitCommands.helpText().map(row=>row.join(' | ')));
// Run a command the way a user does: open the palette, type its name, press Enter.
async function run(label){await page.evaluate(()=>{NetworkToolkitCommands.close();NetworkToolkitCommands.open('commands');});await page.locator('#uxCommandOverlay .ux-command-input').fill(label);
 const row=page.locator('#uxCommandResults .ux-command-row').filter({has:page.locator('strong',{hasText:new RegExp('^'+label.replace(/[.*+?^${}()|[\]\\\/]/g,'\\$&')+'$')})}).first();await row.waitFor();await row.click();if(label!=='Search tools and commands')await page.waitForFunction(()=>!document.querySelector('#uxCommandOverlay .ux-command-input'));}
const modalTitle=()=>page.locator('#modalTitle').innerText();
const closeModal=()=>page.evaluate(()=>closeModal());
// The connection form as a user sees it: every control, its label, state and choices.
const connectFormNow=()=>page.evaluate(()=>{const modal=document.querySelector('#modalRoot .modal');if(!modal)return null;const shown=el=>{const s=getComputedStyle(el);return s.display!=='none'&&s.visibility!=='hidden'&&el.getClientRects().length>0;};
 return {title:modal.querySelector('#modalTitle')?.textContent,controls:[...modal.querySelectorAll('input,select,textarea,button')].filter(el=>el.type!=='hidden').map(el=>({tag:el.tagName,id:el.id||'',name:el.getAttribute('name')||'',type:el.type||'',label:(el.labels?.[0]?.textContent||el.getAttribute('aria-label')||el.textContent||'').replace(/\s+/g,' ').trim().slice(0,80),value:el.tagName==='SELECT'?el.value:el.type==='checkbox'||el.type==='radio'?String(el.checked):el.tagName==='BUTTON'?'':el.value,options:el.tagName==='SELECT'?[...el.options].map(o=>o.value+'='+o.textContent.trim()):undefined,disabled:el.disabled,shown:shown(el)}))};});
// input-clear.js and dropdowns.js add their Clear and Show all choices buttons on the animation frame after the
// form appears, so a form read at once may or may not have them. Read it only when it has stopped changing.
const connectForm=async()=>{const frames=()=>page.evaluate(()=>new Promise(resolve=>requestAnimationFrame(()=>requestAnimationFrame(()=>requestAnimationFrame(resolve)))));
 let previous=JSON.stringify(await connectFormNow());for(let attempt=0;attempt<100;attempt++){await frames();const form=await connectFormNow(),text=JSON.stringify(form);if(text===previous)return form;previous=text;}
 throw new Error('The connection form kept changing.');};

const EDITOR_COMMANDS=['Application | Keyboard shortcuts and help','Application | Search tools and commands','Application | Window settings','Application | Reopen recently closed tab','Application | Switch light/dark theme','Text Editor | New Text Editor document','Text Editor | More actions for selected Editor tab','Text Editor | Send to terminal','Text Editor | Open Network Toolkit Terminal'];
const TERMINAL_COMMANDS=['Application | Keyboard shortcuts and help','Application | Search tools and commands','Application | Window settings','Application | Reopen recently closed tab','Application | Switch light/dark theme','Terminal | Open a new terminal tab','Terminal | Reconnect all terminal sessions','Terminal | More actions for selected Terminal tab','Terminal | Send output to Text Editor','Terminal | Open Network Toolkit Text Editor'];
const GONE=['Welcome and quick starts','App settings','Update settings / Check for updates','Show or hide navigation'];

async function standaloneEditor(){
 const context=await open('editor-standalone'),f=fixture,editor='#networkToolkitFileEditor';
 await page.locator(editor+' .nt-editor-status .standalone-app-tools').waitFor();
 assert.deepEqual(await page.evaluate(()=>[...document.body.classList].filter(c=>c.startsWith('standalone')).sort()),['standalone-app','standalone-editor']);
 assert.equal(await visible('.app-foot'),false,'the diagnostics footer ("Local network diagnostics", "No diagnostic has run.") is not shown');
 assert.equal(await visible('#sidebar'),false);assert.equal(await visible('.page-heading'),false);assert.equal(await visible(editor+' [data-ed="localTools"]'),false,'no "Full editor" button: this is the full editor');
 assert.equal(await page.locator('#uxPhase5Welcome,.ux-welcome,[data-ux-welcome]').count(),0,'no first-run welcome for the Toolkit');
 pass('Text Editor: no Toolkit navigation, page heading, diagnostics footer or welcome');
 // Theme, window settings and help are visible in the status bar.
 const tools=page.locator(editor+' .nt-editor-status .standalone-app-tools');
 assert.deepEqual(await tools.locator('button').evaluateAll(buttons=>buttons.map(b=>b.getAttribute('aria-label'))),['Switch color theme','Window settings','Keyboard shortcuts and help']);
 for(const button of await tools.locator('button').all()){assert.equal(await button.isVisible(),true);const box=await button.boundingBox();assert(box.width>=20&&box.height>=20&&box.x+box.width<=1280&&box.y+box.height<=800,'status bar button inside the window: '+JSON.stringify(box));}
 const theme=await page.evaluate(()=>state.theme);await tools.locator('[data-action="theme"]').click();assert.notEqual(await page.evaluate(()=>state.theme),theme,'the theme button switches the theme');assert.equal(await page.evaluate(()=>document.documentElement.dataset.theme),theme==='dark'?'light':'dark');await tools.locator('[data-action="theme"]').click();
 await shot('editor');
 // Window settings: this app's own, not the Toolkit's App settings.
 await tools.locator('[data-action="settings"]').click();await page.locator('#modalTitle').filter({hasText:'Text Editor window settings'}).waitFor();
 const settings=await page.locator('#modalRoot .modal').innerText();
 for(const unrelated of ['report label','Result time','Vendor','vendor','App data location','Check for updates','Updates','Quit app','diagnostic','Terminal','Graphs','Transfers','Network','Confirmations'])assert.equal(settings.includes(unrelated),false,'Text Editor window settings mention "'+unrelated+'"');
 for(const id of ['#settingsReportLabel','#storageSettings','#vendorSettings','#appUpdateSettings','#resultTimeSettings','.ux-settings-nav','[data-action="quit-confirm"]','[data-settings-go]'])assert.equal(await page.locator('#modalRoot '+id).count(),0,id+' is not part of the Text Editor settings');
 await page.locator('#settingsTheme').selectOption('dark');assert.equal(await page.evaluate(()=>state.theme),'dark','the Appearance choice applies');await page.locator('#settingsTheme').selectOption('light');
 assert.equal(await page.locator('#modalRoot [data-app-accent-picker]').count(),1,'the accent color the editor already keeps is offered');
 await page.locator('#modalRoot [data-app-accent-swatch]').first().click();await page.waitForFunction(()=>/^#[0-9a-f]{6}$/i.test(state.accentColor||''));await page.locator('#modalRoot [data-app-accent-default]').click();
 await shot('editor-settings');await closeModal();
 pass('Text Editor: theme, window settings and help buttons are visible; settings hold only appearance and version');
 // Help: this app's shortcuts.
 await tools.locator('#uxEditorHelpLauncher').click();await page.locator('[aria-label="Keyboard shortcuts and help"][role="dialog"]').waitFor();
 const help=await helpRows();
 assert.equal(help.some(row=>/^(Terminal|File browser) \|/.test(row)),false,'no Terminal or File browser shortcuts: '+help.join('; '));
 assert(help.includes('Text Editor | Ctrl+Shift+Enter | Send the selection or the document to Network Toolkit Terminal'));assert(help.includes('Text Editor | Ctrl+N | New document'));
 assert.deepEqual([...new Set(help.map(row=>row.split(' | ')[0]))],['Application','Text Editor','Files sidebar','Panels']);
 const helpText=await page.locator('#uxCommandOverlay').innerText();assert.equal(/Terminal keystrokes|welcome and quick starts/i.test(helpText),false);assert.equal(await page.locator('#uxCommandOverlay .ux-help-row').count(),help.length);
 await shot('editor-help');await page.keyboard.press('Escape');
 await page.keyboard.press('F1');await page.locator('[aria-label="Keyboard shortcuts and help"][role="dialog"]').waitFor();await page.keyboard.press('Escape');
 pass('Text Editor: F1 help lists editor shortcuts and Send to terminal only');
 // Command palette: only this app's commands, and each one does something.
 assert.deepEqual(await palette(),EDITOR_COMMANDS);
 await page.keyboard.press('Control+Shift+P');await page.locator('#uxCommandOverlay .ux-command-input').waitFor();
 for(const label of GONE.concat(['Ping','Terminal','Reports','File Transfer','Overview'])){await page.locator('#uxCommandOverlay .ux-command-input').fill(label);await page.waitForTimeout(30);assert.equal(await page.locator('#uxCommandResults .ux-command-row strong').evaluateAll((rows,l)=>rows.some(r=>r.textContent===l),label),false,label+' is not offered');}
 await shot('editor-palette');await page.keyboard.press('Escape');
 await run('Window settings');assert.equal(await modalTitle(),'Text Editor window settings');await closeModal();
 {const before=await page.evaluate(()=>state.theme);await run('Switch light/dark theme');assert.notEqual(await page.evaluate(()=>state.theme),before);await run('Switch light/dark theme');}
 await run('Keyboard shortcuts and help');await page.locator('[aria-label="Keyboard shortcuts and help"][role="dialog"]').waitFor();await page.keyboard.press('Escape');
 await run('Search tools and commands');await page.locator('#uxCommandOverlay .ux-command-input').waitFor();await page.keyboard.press('Escape');
 {const before=await page.evaluate(()=>NetworkToolkitFileEditor.listDocuments().length);await run('New Text Editor document');await page.waitForFunction(n=>NetworkToolkitFileEditor.listDocuments().length===n+1,before);}
 await run('More actions for selected Editor tab');await page.getByRole('menuitem',{name:/^Save as/}).first().waitFor();await page.keyboard.press('Escape');
 await page.locator(editor+' [data-ed-text]').fill('show version');
 // Send to terminal with nothing running opens the Terminal app by itself (owner request; this used to
 // show "Network Toolkit Terminal is not open." and wait for the user). A new Terminal has no session yet.
 await run('Send to terminal');await page.locator('[data-editor-send-peer-missing]').waitFor();assert.equal(f.opens,1,'the Terminal app is started without a question');assert.match(await page.locator('#modalRoot .modal-body').innerText(),/No terminal session is connected in Network Toolkit Terminal\./);await closeModal();
 await run('Open Network Toolkit Terminal');await page.waitForFunction(()=>[...document.querySelectorAll('#toastStack .toast')].some(n=>n.textContent.includes('Network Toolkit Terminal is open.')));assert.equal(f.opens,2);
 pass('Text Editor: the command palette lists only editor commands plus the Terminal connection, and every one of them works');
 // The Connect sidebar: nothing that points to, or starts, another Toolkit tool.
 await closeModal();await page.locator(editor+' button[aria-label="Connections"],'+editor+' button[title="Connections"]').first().click();
 await page.waitForFunction(()=>/Fill in the connection below, then choose Connect \/ Browse\./.test(document.querySelector('#ntEditorRemoteHint')?.textContent||''));
 assert.equal(/Terminal|FTPS browser/.test(await page.locator('#ntEditorRemoteHint').textContent()),false,'the hint does not send the user to tools this app does not have');
 const openTerminal=await page.evaluate(()=>[...document.querySelectorAll('#networkToolkitFileEditor [data-rb-action="openTerminal"]')].map(el=>getComputedStyle(el).display));
 assert(openTerminal.length>0&&openTerminal.every(display=>display==='none'),'"Open terminal" on a file connection is hidden: '+JSON.stringify(openTerminal));
 assert.equal(await page.locator(editor+' [data-rb-action="connect"]').first().isVisible(),true,'file connections stay');
 await shot('editor-connect');
 pass('Text Editor: the Connect sidebar keeps file connections and offers no terminal');
 // The Toolkit's services are never asked for by this app.
 const unrelated=f.calls.filter(call=>/\/api\/(updates|vendors|storage|jobs|report|system|local-tools|packet-captures|public-ip|scan-networks|terminal-window|terminals)\b/.test(call));
 assert.deepEqual([...new Set(unrelated)],[],'the standalone Text Editor does not use these Toolkit services');
 pass('Text Editor: opens no updater, vendor database, storage, diagnostics or terminal requests');
 await context.close();
}

async function standaloneTerminal(){
 // ---- Start card and local shells (owner requirement) ----
 let context=await open('terminal');
 await page.locator('#terminalStage [data-terminal-new-type="ssh"]').first().waitFor();
 assert.deepEqual(await page.evaluate(()=>[...document.body.classList].filter(c=>c.startsWith('standalone')).sort()),['standalone-app','standalone-terminal']);
 const start=async()=>page.locator('#terminalStage .terminal-welcome-actions').first().locator('button').evaluateAll(buttons=>buttons.map(b=>[b.dataset.terminalNewType,b.textContent.trim(),b.disabled]));
 const offered=await start();
 assert.deepEqual(offered,[['ssh','SSH',false],['telnet','Telnet',false],['serial','Serial',false],['cmd','Command Prompt',false],['powershell','PowerShell',false]],'the Start a terminal card offers remote sessions and both local Windows shells');
 assert.match(await page.locator('#terminalStage').innerText(),/or start a local Windows shell/);
 await shot('terminal-start');
 const forms={};
 for(const type of ['cmd','powershell','ssh']){await page.locator('#terminalStage [data-terminal-new-type="'+type+'"]').first().click();await page.locator('#terminalConnectForm').waitFor();forms[type]=await connectForm();if(type==='cmd')await shot('terminal-command-prompt-form');await closeModal();await page.locator('#terminalConnectForm').waitFor({state:'detached'});}
 for(const type of ['cmd','powershell']){const text=JSON.stringify(forms[type]);assert.match(text,/[Ss]tarting (folder|directory)/,type+': the starting folder option is offered');assert.match(text,/administrator/i,type+': Run as administrator is offered');}
 assert.equal(fixture.calls.some(call=>call==='POST /api/terminals'),false,'opening the form starts no shell');
 await context.close();
 // The same card and the same forms as the main Toolkit window.
 context=await open('main');await page.evaluate(()=>navigate('terminal'));await page.locator('#terminalStage [data-terminal-new-type="ssh"]').first().waitFor();
 assert.deepEqual(await start(),offered,'the main window offers the same session types');
 for(const type of ['cmd','powershell','ssh']){await page.locator('#terminalStage [data-terminal-new-type="'+type+'"]').first().click();await page.locator('#terminalConnectForm').waitFor();const form=await connectForm(),extra=form.controls.filter(control=>control.label==='Open in terminal window');
  // The only difference: the main window can also hand the form to the separate Terminal window.
  assert.equal(extra.length,1);form.controls=form.controls.filter(control=>!extra.includes(control));assert.deepEqual(form,forms[type],type+': the standalone Terminal opens the same form as the main window');await closeModal();await page.locator('#terminalConnectForm').waitFor({state:'detached'});}
 await context.close();
 // The card is built before the app has answered whether local shells exist; it must catch up.
 context=await open('main',{activePage:'terminal'});await page.locator('#terminalStage [data-terminal-new-type="ssh"]').first().waitFor();
 assert.deepEqual(await start(),offered,'the main window offers local shells when Terminal is its startup page');
 pass('Terminal: SSH, Telnet, Serial, Command Prompt and PowerShell start from the same card and the same forms as the main window');
 // ---- The main window keeps everything (unchanged) ----
 assert.deepEqual(await page.evaluate(()=>[...document.body.classList].filter(c=>c.startsWith('standalone'))),[]);assert.equal(await page.locator('.standalone-app-tools').count(),0);
 const mainCommands=await palette();for(const label of GONE)assert(mainCommands.some(row=>row.endsWith('| '+label)),'main window keeps '+label);
 for(const label of ['Ping','Text Editor','Terminal','Reports'])assert(mainCommands.some(row=>row.endsWith('| '+label)),'main window keeps the '+label+' tool');
 assert.equal(mainCommands.some(row=>/Open Network Toolkit (Terminal|Text Editor)|\| Window settings$/.test(row)),false);
 const mainHelp=await helpRows();assert.deepEqual([...new Set(mainHelp.map(row=>row.split(' | ')[0]))],['Application','Text Editor','Terminal','File browser','Panels']);assert.equal(mainHelp.length,13);
 await page.evaluate(()=>navigate('editor'));await page.locator('#networkToolkitFileEditor button[aria-label="Connections"],#networkToolkitFileEditor button[title="Connections"]').first().click();
 await page.waitForFunction(()=>/Connect through Terminal or the SFTP \/ FTP \/ FTPS browser first\./.test(document.querySelector('#ntEditorRemoteHint')?.textContent||''));
 assert.equal(await page.evaluate(()=>[...document.querySelectorAll('#networkToolkitFileEditor [data-rb-action="openTerminal"]')].some(el=>getComputedStyle(el).display!=='none')),true,'the main window keeps Open terminal for file connections');
 assert.equal(await page.locator('#networkToolkitFileEditor .standalone-app-tools').count(),0);
 await page.evaluate(()=>navigate('overview'));assert.equal(await visible('.app-foot'),true,'the main window keeps its footer');
 await page.evaluate(()=>appSettings());await page.locator('#modalRoot .ux-settings-nav').waitFor();assert.equal(await modalTitle(),'App settings');assert.equal(await page.locator('#modalRoot #appUpdateSettings').count(),1);await closeModal();
 await context.close();
 pass('Main window: palette, help, footer and App settings are unchanged');

 // ---- A connected SSH session ----
 context=await open('terminal',{sessions:[session]});const f=fixture;
 await page.waitForFunction(()=>terminalSessions.size===1&&currentTerminal()?.info.id==='s1');
 assert.equal(await visible('#sidebar'),false);assert.equal(await visible('.app-foot'),false);
 // Palette and help.
 assert.deepEqual((await palette()).filter(row=>!row.startsWith('Current tool actions |')),TERMINAL_COMMANDS);
 await page.evaluate(()=>NetworkToolkitCommands.open('commands'));await page.locator('#uxCommandOverlay .ux-command-input').waitFor();
 for(const label of GONE.concat(['Ping','Text Editor','Reports','File Transfer','SFTP','Overview'])){await page.locator('#uxCommandOverlay .ux-command-input').fill(label);await page.waitForTimeout(30);assert.equal(await page.locator('#uxCommandResults .ux-command-row strong').evaluateAll((rows,l)=>rows.some(r=>r.textContent===l),label),false,label+' is not offered');}
 await shot('terminal-palette');await page.keyboard.press('Escape');
 const help=await helpRows();
 assert.deepEqual([...new Set(help.map(row=>row.split(' | ')[0]))],['Application','Terminal','Panels']);assert(help.includes('Terminal | Send to Text Editor | Send output or selected text to Network Toolkit Text Editor (button under the console, tab menu, right-click menu)'));assert(help.includes('Terminal | Ctrl+Shift+F | Find in terminal output'));
 await page.locator('#uxTerminalHelpLauncher').click();await page.locator('[aria-label="Keyboard shortcuts and help"][role="dialog"]').waitFor();assert.equal(await page.locator('#uxCommandOverlay .ux-help-row').count(),help.length);
 assert.equal(/welcome and quick starts|Rename selected file|New document/i.test(await page.locator('#uxCommandOverlay').innerText()),false);await shot('terminal-help');await page.keyboard.press('Escape');
 pass('Terminal: the command palette and F1 help list terminal commands and Send to Text Editor only');
 // Each remaining palette command works.
 await run('Window settings');assert.equal(await modalTitle(),'Terminal window settings');
 let text=await page.locator('#modalRoot .modal').innerText();assert.equal(/main diagnostics window/i.test(text),false,'no wording about a main diagnostics window');assert.match(text,/Ends this window's connections\. Other Network Toolkit windows you have open are not affected\./);
 await shot('terminal-settings');
 await page.locator('#modalRoot [data-action="quit-confirm"]').click();await page.locator('#modalRoot [data-action="quit-app"]').waitFor();
 text=await page.locator('#modalRoot .modal').innerText();assert.equal(/main diagnostics window/i.test(text),false);assert.match(text,/Close this terminal workspace\? Connections still in this window will close\. Other Network Toolkit windows you have open are not affected\./);await closeModal();
 {const before=await page.evaluate(()=>state.theme);await run('Switch light/dark theme');assert.notEqual(await page.evaluate(()=>state.theme),before);await run('Switch light/dark theme');}
 await run('Keyboard shortcuts and help');await page.locator('[aria-label="Keyboard shortcuts and help"][role="dialog"]').waitFor();await page.keyboard.press('Escape');
 {const before=await page.locator('#terminalTabs [data-terminal-tab]').count();await run('Open a new terminal tab');await page.waitForFunction(n=>document.querySelectorAll('#terminalTabs [data-terminal-tab]').length===n+1,before);await page.evaluate(()=>selectTerminal('s1',true));}
 await run('More actions for selected Terminal tab');await page.locator('#terminalTabContextMenu').waitFor();await page.keyboard.press('Escape');
 await run('Send output to Text Editor');await page.waitForFunction(()=>[...document.querySelectorAll('#toastStack .toast')].some(n=>n.textContent.includes('No retained output or selected text to send yet.')));
 await run('Open Network Toolkit Text Editor');await page.waitForFunction(()=>[...document.querySelectorAll('#toastStack .toast')].some(n=>n.textContent.includes('Network Toolkit Text Editor is open.')));assert.equal(f.opens,1);
 pass('Terminal: Window settings and the close prompt are worded for the standalone app, and every palette command works');
 // Tab menu: nothing that cannot work here.
 await page.waitForFunction(()=>NetworkToolkitTerminalSFTP.available(currentTerminal()));
 await page.locator('#terminalTabs [data-terminal-tab="s1"]').click({button:'right'});const menu=page.locator('#terminalTabContextMenu');await menu.waitFor();
 assert.equal(await menu.locator('[data-terminal-tab-action="editor"]').count(),0,'"Open Text Editor" is not offered');
 assert.equal(await menu.locator('[data-terminal-tab-action="browse"]').isEnabled(),true,'"Browse SFTP files" works for a session with SFTP');
 assert.equal(await menu.locator('[data-terminal-tab-action="send-editor"]').isEnabled(),true,'"Send output to Text Editor" reaches the Text Editor app');
 for(const action of ['cmd','powershell'])assert.equal(await menu.locator('[data-terminal-tab-action="'+action+'"]').count(),0);
 await shot('terminal-tab-menu');
 await menu.locator('[data-terminal-tab-action="browse"]').click();
 // SFTP sidebar: the buttons that open the Toolkit's Text Editor or File Transfer tool are hidden.
 const sftp=page.locator('#terminalSidebarSFTP .tsftp-browser');await sftp.waitFor();await sftp.locator('[data-tsftp-item="/home/operator/running-config.txt"]').waitFor();
 assert.equal(await sftp.locator('[data-tsftp="textEditor"]').isVisible(),false,'the sidebar "Editor" button is hidden');assert.equal(await sftp.locator('[data-tsftp="full"]').isVisible(),false,'the sidebar "Expand" button is hidden');
 for(const action of ['up','home','refresh','upload','download','edit','newFile','preview'])assert.equal(await sftp.locator('[data-tsftp="'+action+'"]').isVisible(),true,action+' stays');
 await shot('terminal-sftp-sidebar');
 // The floating editor still edits a remote file, without a "Full editor" button.
 await sftp.locator('[data-tsftp-item="/home/operator/running-config.txt"]').click();await sftp.locator('[data-tsftp="edit"]').click();
 const floating=page.locator('#networkToolkitFileEditor:not(.nt-editor-embedded)');await floating.locator('[data-ed-text]').waitFor();await page.waitForFunction(()=>document.querySelector('#networkToolkitFileEditor [data-ed-text]').value==='hostname lab-switch-01\n');
 assert.equal(await floating.locator('[data-ed="localTools"]').isVisible(),false,'"Full editor" is hidden');assert.equal(await floating.locator('[data-ed="close"]').isVisible(),true);assert.equal(await floating.locator('[data-ed="sendTerminal"]').isVisible(),true,'the editor inside the Terminal can still send to its own sessions');
 assert.equal(await floating.locator('.standalone-app-tools').count(),0,'the Terminal keeps its app buttons in the sidebar rail');
 await shot('terminal-floating-editor');await floating.locator('[data-ed="close"]').click();
 pass('Terminal: the tab menu, the SFTP sidebar and the floating editor offer nothing that needs another Toolkit tool');
 // FTP/FTPS connections saved for the Toolkit's File Transfer tool are not listed in this app.
 await page.evaluate(()=>{state.ftpProfiles=[{id:'ftp-1',name:'Synthetic FTP',folder:'',host:'192.0.2.20',port:21,username:'operator',security:'explicit',fingerprint:'',credentialId:''}];NetworkToolkitSessionSidebar.activateTab('saved');NetworkToolkitSessionSidebar.render();});
 assert.equal(await page.locator('.transfer-session-section').evaluateAll(nodes=>nodes.some(node=>getComputedStyle(node).display!=='none')),false,'FTP/FTPS connections cannot be opened here and are not listed');
 const app=page.locator('#terminalSessionSidebar .terminal-sidebar-rail .terminal-app-tools');assert.deepEqual(await app.locator('button').evaluateAll(buttons=>buttons.map(b=>b.getAttribute('aria-label'))),['Switch color theme','Window settings','Keyboard shortcuts and help']);
 pass('Terminal: theme, window settings and help stay in the sidebar rail; FTP connections of another tool are not listed');
 await context.close();

 // ---- The main window keeps those actions ----
 context=await open('main',{sessions:[session]});await page.evaluate(()=>navigate('terminal'));await page.waitForFunction(()=>terminalSessions.size===1&&currentTerminal()?.info.id==='s1');
 await page.waitForFunction(()=>NetworkToolkitTerminalSFTP.available(currentTerminal()));
 await page.locator('#terminalTabs [data-terminal-tab="s1"]').click({button:'right'});const mainMenu=page.locator('#terminalTabContextMenu');await mainMenu.waitFor();
 assert.equal(await mainMenu.locator('[data-terminal-tab-action="editor"]').isEnabled(),true);assert.equal(await mainMenu.locator('[data-terminal-tab-action="browse"]').isEnabled(),true);
 await mainMenu.locator('[data-terminal-tab-action="browse"]').click();const mainSftp=page.locator('#terminalSidebarSFTP .tsftp-browser');await mainSftp.waitFor();
 assert.equal(await mainSftp.locator('[data-tsftp="textEditor"]').isVisible(),true);assert.equal(await mainSftp.locator('[data-tsftp="full"]').isVisible(),true);
 await page.evaluate(()=>{state.ftpProfiles=[{id:'ftp-1',name:'Synthetic FTP',folder:'',host:'192.0.2.20',port:21,username:'operator',security:'explicit',fingerprint:'',credentialId:''}];NetworkToolkitSessionSidebar.activateTab('saved');NetworkToolkitSessionSidebar.render();});
 await page.locator('.transfer-session-section').first().waitFor();assert.equal(await page.locator('.transfer-session-section').first().isVisible(),true);
 pass('Main window: Open Text Editor, the SFTP sidebar\'s Editor and Expand buttons and saved FTP connections are unchanged');
 await context.close();
}

(async()=>{
 if(out)fs.mkdirSync(out,{recursive:true});
 browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 await standaloneEditor();await standaloneTerminal();
 report.errors=report.errors.filter(e=>!/Synthetic fixture/.test(e));assert.deepEqual(report.errors,[]);report.passed=true;
})().catch(error=>{report.errors.push(String(error?.stack||error));console.error(error);process.exitCode=1;}).finally(async()=>{if(out)fs.writeFileSync(path.join(out,'standalone-apps-results.json'),JSON.stringify(report,null,2));await browser?.close();console.log(JSON.stringify({passed:report.passed,checks:report.checks.length,errors:report.errors}));});
