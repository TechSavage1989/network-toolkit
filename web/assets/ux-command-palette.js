(function(root){
'use strict';
// Phase 3 commands are a view over existing controls. They never store
// profiles, terminal output, inputs, credentials, or other application data.
const doc=root.document;if(!doc)return;
const toolGroups={overview:'Home',profiles:'Home',reports:'Home',
 quick:'Diagnose',ping:'Diagnose',trace:'Diagnose',dns:'Diagnose',smtp:'Diagnose',
 tcp:'Diagnose',http:'Diagnose',tls:'Diagnose',identify:'Diagnose',discover:'Diagnose',
 monitor:'Diagnose',capture:'Diagnose',terminal:'Connect',wol:'Connect',
 sftp:'Transfer',ftp:'Transfer',tftp:'Transfer',
 local:'Configure',hosts:'Configure',subnet:'Configure',editor:'Configure'};
const shortcutLabels={editor:'',terminal:'',overview:'',ping:'',trace:'',dns:''};
const launcher=()=>doc.getElementById('uxCommandLauncher'),helpLauncher=()=>doc.getElementById('uxHelpLauncher');
let overlay=null,input=null,results=null,items=[],selection=0,previousFocus=null,mode='';
const $=selector=>doc.querySelector(selector);
const make=(tag,cls,text)=>{const el=doc.createElement(tag);if(cls)el.className=cls;if(text!==undefined)el.textContent=text;return el;};
function inTerminalInput(event){
 const target=event?.target;
 if(!target||typeof target.closest!=='function')return false;
 return !!target.closest('.xterm,.xterm-helper-textarea,.xterm-screen,.terminal-xterm,[data-terminal-input-surface],.terminal-pane .xterm,textarea[data-terminal-console]');
}
function inOwnerDialog(){return !!$('#modalRoot .modal,#modalRoot [role="dialog"],.nt-editor-dialog,[data-ed-dialog]');}
function allowedTool(id){if(doc.body.classList.contains('editor-window'))return id==='editor';
 if(doc.body.classList.contains('terminal-window'))return id==='terminal';return true;}
function visible(el){
 if(!el||!el.isConnected||el.disabled||el.getAttribute('aria-hidden')==='true'||el.closest('[hidden],.hidden,[inert]'))return false;
 const style=root.getComputedStyle(el);return style.display!=='none'&&style.visibility!=='hidden'&&el.getClientRects().length>0;
}
function buttons(){
 const selectors=['#toolWorkspace button[data-action]','#toolWorkspace button[data-ed]',
 '#toolWorkspace button[data-terminal-action]','#toolWorkspace button[data-fm-action]',
 '#toolWorkspace button[data-hosts-action]','#pageActions button',
 '#toolConfiguration button','#toolCommandBar button','#terminalTabs [data-terminal-tab-actions]',
 '#terminalToolbar button','#appUpdateNotice button'];
 const seen=new Set(),out=[];
 for(const button of doc.querySelectorAll(selectors.join(','))){
  if(!visible(button)||button.closest('[role="dialog"],.terminal-output,.xterm')||seen.has(button))continue;
  seen.add(button);
  const label=(button.getAttribute('aria-label')||button.getAttribute('title')||button.textContent||'').replace(/\s+/g,' ').trim();
  if(!label||label.length>85||/^(cancel|done|close dialog|yes|no)$/i.test(label))continue;
  const id='control-'+out.length;
  out.push({id,label,group:'Current tool actions',shortcut:button.getAttribute('aria-keyshortcuts')||'',run:()=>{if(visible(button))button.click();}});
 }
 return out;
}
function catalog(){
 const tools=[...doc.querySelectorAll('#navigation button[data-page]')].filter(btn=>allowedTool(btn.dataset.page)).map(btn=>{
  const id=btn.dataset.page;const name=btn.querySelector('span:not(.badge)')?.textContent?.trim()||id;
  const keywords=id==='sftp'?'ftp ftps file transfer ssh file browser':id==='tftp'?'tftp file transfer':id==='quick'?'test a target device check':id==='trace'?'traceroute network hops':id;
  return {id:'tool-'+id,label:name,group:toolGroups[id]||'Tools',shortcut:shortcutLabels[id]||'',keywords,run:()=>{if(typeof root.navigate==='function')root.navigate(id);else if(typeof navigate==='function')navigate(id);}};
 });
 const global=[
  {id:'help',label:'Keyboard shortcuts and help',group:'Application',shortcut:'F1',run:()=>open('help')},
  {id:'palette',label:'Search tools and commands',group:'Application',shortcut:'Ctrl+Shift+P',run:()=>open('commands')},
  {id:'settings',label:'App settings',group:'Application',run:()=>$('#settingsButton')?.click()},
  {id:'updates',label:'Update settings / Check for updates',group:'Application',keywords:'update manually',run:()=>{
   $('#settingsButton')?.click();const settings=$('#appUpdateSettings');settings?.scrollIntoView({block:'nearest'});settings?.querySelector('button')?.focus({preventScroll:true});
  }},
  {id:'theme',label:'Switch light/dark theme',group:'Application',run:()=>$('#themeButton')?.click()},
  {id:'navigation',label:'Show or hide navigation',group:'Application',run:()=>root.NetworkToolkitNavigation?.toggle?.()},
 ];
 if((doc.body.classList.contains('terminal-page')||doc.body.classList.contains('terminal-window'))&&allowedTool('terminal'))global.push(
  {id:'new-terminal',label:'Open a new terminal tab',group:'Terminal',run:()=>{if(typeof newTerminalTab==='function')newTerminalTab();}},
  {id:'reconnect-all',label:'Reconnect all terminal sessions',group:'Terminal',run:()=>root.NetworkToolkitTerminalTabs?.requestReconnectAll?.()},
  {id:'terminal-tab-menu',label:'More actions for selected Terminal tab',group:'Terminal',run:()=>{
   const btn=doc.querySelector('#terminalTabs [data-terminal-tab][aria-selected="true"]');if(btn){const r=btn.getBoundingClientRect();root.NetworkToolkitTerminalTabs?.open?.(btn.dataset.terminalTab,r.left,r.bottom);}
  }}
 );
 if(allowedTool('editor'))global.push(
  {id:'new-document',label:'New Text Editor document',group:'Text Editor',shortcut:'Ctrl+N',run:()=>root.NetworkToolkitFileEditor?.newDocument?.()},
  {id:'editor-tab-actions',label:'More actions for selected Editor tab',group:'Text Editor',shortcut:'Shift+F10',run:()=>{
   const btn=doc.querySelector('.nt-editor-tab [data-ed-tab-actions]');if(btn)btn.click();
  }}
 );
 return [...tools,...global,...buttons()];
}
function rank(item,words){
 const target=(item.label+' '+(item.keywords||'')+' '+item.group+' '+item.shortcut).toLowerCase();
 if(!words.every(word=>target.includes(word)))return -1;
 const name=item.label.toLowerCase(),query=words.join(' ');
 return name===query?0:name.startsWith(query)?1:target.startsWith(query)?2:3;
}
function close(){
 if(!overlay)return;overlay.remove();overlay=null;input=null;results=null;items=[];const previous=previousFocus;previousFocus=null;
 if(previous?.isConnected&&typeof previous.focus==='function')previous.focus({preventScroll:true});
}
function run(item){
 if(!item)return;close();try{item.run();}catch(e){root.console?.error?.('Command could not be opened',e);}
}
function render(){
 if(!results||mode!=='commands')return;const words=input.value.toLowerCase().trim().split(/\s+/).filter(Boolean);
 items=catalog().map(x=>({...x,rank:rank(x,words)})).filter(x=>x.rank>=0).sort((a,b)=>a.rank-b.rank||a.group.localeCompare(b.group)||a.label.localeCompare(b.label)).slice(0,80);
 selection=Math.max(0,Math.min(selection,items.length-1));results.replaceChildren();
 if(!items.length){results.append(make('div','ux-command-empty','No matching commands. Try a tool name such as Ping, Terminal, or Reports.'));return;}
 items.forEach((item,index)=>{
  const button=make('button','ux-command-row'+(index===selection?' is-selected':''));button.type='button';button.setAttribute('role','option');button.setAttribute('aria-selected',String(index===selection));button.id='ux-command-option-'+index;
  const info=make('span','ux-command-info');info.append(make('strong','',item.label),make('small','',item.group));button.append(info);
  if(item.shortcut)button.append(make('kbd','ux-command-shortcut',item.shortcut));
  button.addEventListener('mouseenter',()=>{selection=index;highlight();});
  button.addEventListener('click',()=>run(item));results.append(button);
 });
 highlight();
}
function highlight(){
 if(!results)return;for(const [index,button] of [...results.querySelectorAll('button')].entries()){
  button.classList.toggle('is-selected',selection===index);button.setAttribute('aria-selected',String(selection===index));
 }
 if(input){input.setAttribute('aria-activedescendant',items.length?'ux-command-option-'+selection:'');}
}
function helpText(){
 const all=[['Application','Ctrl+Shift+P','Search tools and commands'],['Application','F1','Open this help sheet'],
 ['Text Editor','Ctrl+N','New document'],['Text Editor','Alt+Shift+Left / Right','Reorder a document tab'],
 ['Text Editor','Shift+F10','Open the document tab menu'],
 ['Terminal','Alt+Shift+Left / Right','Reorder a terminal tab'],
 ['Terminal','Shift+F10','Open the terminal tab menu'],
 ['Terminal','Ctrl+Shift+F','Find in terminal output'],
 ['File browser','F2','Rename selected file'],['File browser','Ctrl / Shift+click','Select multiple files'],
 ['Panels','Enter or Space on divider','Collapse or restore a sidebar'],
 ['Panels','Arrow keys on divider','Resize a panel']];
 return all;
}
function open(nextMode='commands'){
 if(overlay){if(mode===nextMode){input?.focus({preventScroll:true});return;}close();}
 if(inOwnerDialog())return false;
 previousFocus=doc.activeElement;mode=nextMode;
 overlay=make('div','ux-command-overlay');overlay.id='uxCommandOverlay';
 const backdrop=make('div','ux-command-backdrop');backdrop.setAttribute('aria-hidden','true');backdrop.addEventListener('click',close);
 const dialog=make('section','ux-command-dialog');dialog.setAttribute('role','dialog');dialog.setAttribute('aria-modal','true');dialog.setAttribute('aria-label',nextMode==='help'?'Keyboard shortcuts and help':'Search tools and commands');
 const head=make('header','ux-command-header'),title=make('h2','',nextMode==='help'?'Keyboard shortcuts and help':'Search tools and commands');
 const closeButton=make('button','ux-command-close','Close');closeButton.type='button';closeButton.setAttribute('aria-label','Close');closeButton.addEventListener('click',close);
 head.append(title,closeButton);dialog.append(head);
 if(nextMode==='commands'){
  input=make('input','ux-command-input');input.type='search';input.placeholder='Find a tool or action…';input.setAttribute('aria-label','Search tools and actions');input.setAttribute('role','combobox');input.setAttribute('aria-autocomplete','list');input.setAttribute('aria-controls','uxCommandResults');input.setAttribute('aria-expanded','true');
  results=make('div','ux-command-results');results.id='uxCommandResults';results.setAttribute('role','listbox');input.addEventListener('input',()=>{selection=0;render();});
  dialog.append(input,results,make('footer','ux-command-footer','Arrow keys to choose • Enter to open • Esc to close'));
 }else{
  const info=make('p','ux-help-intro','Use the menus and visible action buttons if you prefer not to use shortcuts. Terminal keystrokes are reserved for the connected device while its console is focused.');
  const list=make('div','ux-help-list');for(const [group,key,label] of helpText()){
   const row=make('div','ux-help-row');row.append(make('span','ux-help-description',group+' · '+label),make('kbd','',key));list.append(row);
  }
  dialog.append(info,list);
 }
 overlay.append(backdrop,dialog);doc.body.append(overlay);(input||closeButton).focus({preventScroll:true});if(input)render();
 return true;
}
doc.addEventListener('keydown',event=>{
 if(overlay){
  if(event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();close();return;}
  if(event.key==='Tab'){const focusable=[...overlay.querySelectorAll('input,button')].filter(visible);if(!focusable.length)return;
   const first=focusable[0],last=focusable.at(-1);if(event.shiftKey&&doc.activeElement===first){event.preventDefault();last.focus();}
   else if(!event.shiftKey&&doc.activeElement===last){event.preventDefault();first.focus();}
   return;
  }
  if(mode==='commands'&&(event.key==='ArrowDown'||event.key==='ArrowUp')){event.preventDefault();selection=(selection+(event.key==='ArrowDown'?1:-1)+items.length)%Math.max(1,items.length);highlight();results?.querySelector('.is-selected')?.scrollIntoView({block:'nearest'});return;}
  if(mode==='commands'&&event.key==='Enter'&&doc.activeElement===input){event.preventDefault();run(items[selection]);return;}
  return;
 }
 if(event.defaultPrevented||event.isComposing||inTerminalInput(event)||inOwnerDialog())return;
 if(event.ctrlKey&&event.shiftKey&&!event.altKey&&!event.metaKey&&event.key.toLowerCase()==='p'){
  event.preventDefault();event.stopImmediatePropagation();open('commands');return;
 }
 if(event.key==='F1'&&!event.altKey&&!event.ctrlKey&&!event.shiftKey&&!event.metaKey){
  event.preventDefault();event.stopImmediatePropagation();open('help');
 }
},true);
// Capture before existing document handlers that own Editor tab gestures.
root.addEventListener('click',event=>{
 if(event.target.closest?.('#uxCommandLauncher,#uxCommandLauncherNarrow')){event.preventDefault();open('commands');}
 else if(event.target.closest?.('#uxHelpLauncher,#uxTerminalHelpLauncher')){event.preventDefault();open('help');}
 else if(event.target.closest?.('[data-ed-tab-actions]')){
  event.preventDefault();event.stopImmediatePropagation();
  const button=event.target.closest('[data-ed-tab-actions]');
  root.NetworkToolkitFileEditor?.openTabActions?.(button.dataset.edTabActions);
 }
 else if(event.target.closest?.('[data-terminal-tab-actions]')){
  event.preventDefault();event.stopImmediatePropagation();const button=event.target.closest('[data-terminal-tab-actions]');const r=button.getBoundingClientRect();root.NetworkToolkitTerminalTabs?.open?.(button.dataset.terminalTabActions,r.left,r.bottom);
 }
},true);
root.NetworkToolkitCommands=Object.freeze({open,close,catalog,inTerminalInput,helpText});
})(window);