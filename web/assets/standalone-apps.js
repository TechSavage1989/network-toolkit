/* Standalone apps (owner request). The installed edition starts the same
 * program three ways: Network Toolkit, Network Toolkit Text Editor and Network
 * Toolkit Terminal. Each standalone app shows only what belongs to it, plus
 * what connects it to the other standalone app (Send to terminal, To editor).
 * Rule: nothing visible that does nothing, and nothing from another tool.
 *
 * This module is active only in those two apps. index.html calls configure()
 * once with the window mode; the main Toolkit window and detached child
 * windows are left exactly as they are.
 *   Text Editor: no diagnostics footer; theme, window settings and help
 *     buttons in the editor status bar; its own small settings dialog.
 *   Terminal: actions that need the Toolkit's Text Editor or SFTP tool are
 *     hidden (standalone-apps.css, terminal-tabs.js). SSH, Telnet, Serial,
 *     Command Prompt and PowerShell sessions are all part of the Terminal.
 *   Both: the command palette and the F1 help list only this app's commands. */
(function(root){
'use strict';
const document=root.document;if(!document)return;
let mode='';
const g=name=>{try{return root[name];}catch{return undefined;}};
function svg(name){try{return typeof icon==='function'?icon(name):'';}catch{return '';}}
function configure(options={}){
 mode=options.editor===true?'editor':options.terminal===true?'terminal':'';
 const body=document.body;if(!body)return mode;
 body.classList.toggle('standalone-app',!!mode);body.classList.toggle('standalone-editor',mode==='editor');body.classList.toggle('standalone-terminal',mode==='terminal');
 if(mode){root.NetworkToolkitPeerSend?.start?.(mode);decorateEditor(document.getElementById('networkToolkitFileEditor'));}
 return mode;
}
// The standalone Text Editor has no app sidebar, so its theme, window settings
// and help buttons sit at the end of the editor's status bar (always visible).
function decorateEditor(overlay){
 if(mode!=='editor'||!overlay||overlay.querySelector('.standalone-app-tools'))return false;
 const status=overlay.querySelector('.nt-editor-status');if(!status)return false;
 const group=document.createElement('span');group.className='standalone-app-tools';group.setAttribute('role','group');group.setAttribute('aria-label','App');
 group.innerHTML='<button type="button" class="iconbtn" data-action="theme" title="Switch color theme" aria-label="Switch color theme">'+svg('sun')+'</button>'+
  '<button type="button" class="iconbtn" data-action="settings" title="Window settings: theme and accent color" aria-label="Window settings">'+svg('settings')+'</button>'+
  '<button type="button" class="iconbtn standalone-app-help" id="uxEditorHelpLauncher" title="Keyboard shortcuts and help (F1)" aria-label="Keyboard shortcuts and help" aria-keyshortcuts="F1">?</button>';
 status.append(group);return true;
}
// Window settings of the standalone Text Editor. The main App settings (report
// label, result times, vendor database, updater, diagnostics) do not apply here.
function editorSettings(){
 const openModal=g('openModal'),kv=g('kv'),accent=g('accentPickerHTML');if(typeof openModal!=='function')return false;
 let theme='dark',version='',computer='';
 try{theme=state.theme==='light'?'light':'dark';}catch{}
 try{version=status.version||VERSION;computer=status.system?.hostname||'';}catch{}
 openModal('Text Editor window settings',
  '<div class="setting-row"><div><div class="setting-title">Appearance</div><div class="setting-desc">Choose a dark or light Text Editor window.</div></div><select id="settingsTheme" aria-label="Color theme"><option value="dark" '+(theme==='dark'?'selected':'')+'>Dark</option><option value="light" '+(theme==='light'?'selected':'')+'>Light</option></select></div>'+
  (typeof accent==='function'?accent():'')+
  '<p class="help-line">Font size, wrapping, editor colors and reopening the last session are on the editor toolbar and its More menu.</p>'+
  (typeof kv==='function'?'<div class="data-grid">'+kv('Version',version||'Not reported')+kv('Computer',computer||'This computer')+'</div>':''),
  '<button class="btn primary" data-action="close-modal">Done</button>');
 return true;
}
// Command palette: drop commands that do nothing in a standalone app and add
// the ones that reach the other standalone app.
function commands(list){
 if(!mode)return list;
 const hidden=new Set(['welcome','updates','navigation']),peer=root.NetworkToolkitPeerSend,other=peer?.peerName?.()||'';
 const out=list.filter(item=>!hidden.has(item.id)).map(item=>item.id==='settings'?{...item,label:'Window settings',keywords:'app settings theme appearance'}:item);
 if(mode==='editor')out.push(
  {id:'send-terminal',label:'Send to terminal',group:'Text Editor',shortcut:'Ctrl+Shift+Enter',keywords:'network toolkit terminal session paste commands',run:()=>document.querySelector('#networkToolkitFileEditor [data-ed="sendTerminal"]')?.click()});
 if(mode==='terminal')out.push(
  {id:'send-editor',label:'Send output to Text Editor',group:'Terminal',shortcut:'',keywords:'to editor network toolkit text editor document',run:()=>{let entry=null;try{entry=typeof currentTerminal==='function'?currentTerminal():null;}catch{}if(entry)void root.NetworkToolkitTerminalSendEditor?.open(entry);else{const toast=g('toast');if(typeof toast==='function')toast('Open a terminal session first.');}}});
 if(other)out.push({id:'open-peer',label:'Open '+other,group:mode==='editor'?'Text Editor':'Terminal',shortcut:'',keywords:mode==='editor'?'terminal ssh telnet serial session':'text editor documents files',run:()=>void peer.open()});
 return out;
}
// F1 help: this app's shortcuts only.
function helpRows(all){
 if(!mode)return all;
 const groups=mode==='editor'?['Application','Text Editor','Panels']:['Application','Terminal','Panels'];
 const rows=all.filter(row=>groups.includes(row[0]));
 if(mode==='editor')rows.splice(rows.findIndex(row=>row[0]==='Panels'),0,
  ['Text Editor','Ctrl+Shift+Enter','Send the selection or the document to Network Toolkit Terminal'],
  ['Files sidebar','F2','Rename the selected file'],['Files sidebar','Ctrl / Shift+click','Select several files']);
 else rows.splice(rows.findIndex(row=>row[0]==='Panels'),0,
  ['Terminal','To editor','Send output or selected text to Network Toolkit Text Editor']);
 return rows;
}
function helpIntro(text){return mode==='editor'?'Use the menus and visible action buttons if you prefer not to use shortcuts.':text;}
root.NetworkToolkitStandalone=Object.freeze({configure,mode:()=>mode,decorateEditor,editorSettings,commands,helpRows,helpIntro});
})(window);
