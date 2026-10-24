// Generate the real browser handoff payload for strict Go HTTP decoding.
'use strict';
const fs=require('node:fs'),vm=require('node:vm');global.self=global;const {Terminal}=require('../web/assets/xterm.js'),SerializeAddon=require('../web/assets/addon-serialize.js');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
function source(name){return html.split('\n').find(s=>s.startsWith('function '+name+'(')||s.startsWith('async function '+name+'('));}
(async()=>{
 const term=new Terminal({cols:80,rows:12,scrollback:10000,allowProposedApi:true}),serialize=new SerializeAddon.SerializeAddon();term.loadAddon(serialize);
 // Headless xterm has no browser theme service. Supply its color state only;
 // parsing, scrollback, markers, serialization and handoff are the real code.
 const colors={ansi:Array.from({length:256},()=>({rgba:255})),contrastCache:{clear(){}},halfContrastCache:{clear(){}}};for(const key of ['foreground','background','cursor','cursorAccent','selectionBackgroundTransparent','selectionBackgroundOpaque','selectionForeground'])colors[key]={rgba:255};term._core._themeService={colors,_restoreColors:{...colors},modifyColors:fn=>fn(colors)};
 const entry={info:{id:process.argv[2]||'fixture',state:'connected'},xterm:term,serialize,config:{type:'telnet',host:process.argv[3]||'127.0.0.1',port:Number(process.argv[4])||23,username:'admin',credentialId:'saved-reference-canary',password:'password-canary'}};
 const c={Terminal,SerializeAddon,TextEncoder,TextDecoder,Uint8Array,Int32Array,Uint16Array,Uint32Array,btoa,atob,Date,Math,setTimeout,clearTimeout,document:{querySelector:()=>null},currentTerminal:()=>entry,terminalEntryAcceptsInput:()=>true,terminalFeedAfter:0,terminalPanes:[entry.info.id,'','',''],terminalActive:entry.info.id,terminalPaneFocus:0,terminalPaneMaximized:-1,$:()=>null,terminalWorkspaceDisplay:()=>({theme:'dark'}),terminalWorkspaceEntries:()=>[entry],terminalDisplayBarrier:e=>new Promise(resolve=>e.xterm.write(new Uint8Array(0),resolve))};c.window=c;vm.createContext(c);
 vm.runInContext(html.split('\n').find(s=>s.startsWith('const TERMINAL_SAFE_FIELDS=')),c);
 for(const name of ['safeTerminalConfig','terminalTransferConfig','terminalSnapshotSupport','captureTerminalSnapshot','captureTerminalWorkspace'])vm.runInContext(source(name),c);
 for(const file of ['terminal-state.js','terminal-output-archive.js','terminal-extras.js'])vm.runInContext(fs.readFileSync(require.resolve('../web/assets/'+file),'utf8'),c);
 const extras=c.NetworkToolkitTerminalExtras;
 async function write(text){extras.observeOutput(entry,text);await new Promise(resolve=>term.write(text,resolve));await extras.flush(entry);}
 await write('switch# ');extras.noteManualInput(entry,'show long');await write('show long');extras.noteManualInput(entry,'\r');await write('\r\n'+Array.from({length:12000},(_,i)=>'configuration '+i).join('\r\n')+'\r\nswitch# ');
 const payload=await c.captureTerminalWorkspace();process.stdout.write(JSON.stringify({workspace:payload}));extras.clear(entry);term.dispose();
})().catch(e=>{console.error(e);process.exitCode=1});
