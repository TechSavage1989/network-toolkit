(function(root){
'use strict';
const CATEGORIES=['General','Appearance','Terminal','Editor','Graphs','Transfers','Network','Updates','Confirmations'];
let openTool=null;
function configure(options={}){if(typeof options.openTool==='function')openTool=options.openTool;}
function offerUndo(message,restore){
 if(typeof restore!=='function')return false;
 const host=root.document.getElementById('toastStack');if(!host)return false;
 const alert=root.document.createElement('div');alert.className='toast info ux-undo-toast';alert.setAttribute('role','status');
 const text=root.document.createElement('span');text.textContent=String(message||'Item removed');
 const button=root.document.createElement('button');button.type='button';button.className='btn small ghost';button.textContent='Undo';
 alert.append(text,button);host.append(alert);let active=true;
 const timer=root.setTimeout(()=>{active=false;alert.remove();},10000);
 button.addEventListener('click',()=>{if(!active)return;active=false;root.clearTimeout(timer);alert.remove();restore();},{once:true});
 return true;
}
function make(tag,cls,text){const el=root.document.createElement(tag);if(cls)el.className=cls;if(text!==undefined)el.textContent=text;return el;}
const tools={
 terminal:[['Open Terminal preferences','terminal']],
 editor:[['Open Text Editor preferences','editor']],
 graphs:[['Customize diagnostic and graph colors','diagnostic-colors'],['Open monitoring graphs','monitor']],
 transfers:[['Open SFTP preferences','sftp'],['Open FTP/FTPS preferences','ftp'],['Open TFTP preferences','tftp']],
 network:[['Open IP configuration','local'],['Open Hosts file editor','hosts']],
 confirmations:[['Review Terminal confirmation options','terminal']]
};
const notes={
 terminal:'Terminal fonts, appearance, tabs, and session controls are available through Terminal tools. Saved choices are retained.',
 editor:'The Text Editor retains its appearance, indentation, wrapping and save preferences.',
 graphs:'Graph scale, zoom, history and follow preferences stay attached to the graphs they affect.',
 transfers:'Protocol-specific transfer preferences remain with SFTP, FTP/FTPS and TFTP. Connection security and overwrite reviews are unchanged.',
 network:'Network settings and Hosts file changes continue to require an explicit review and Windows approval.',
 confirmations:'Closing active sessions, discarding unsaved work, overwriting files, changing system settings, deleting security data and installing updates keep their existing safety checks. Low-risk saved targets can be restored with Undo.'
};
function select(body,name){
 const key=String(name||'general').toLowerCase(),section=body.querySelector('[data-settings-panel="'+key+'"]');
 if(!section)return false;
 const search=body.querySelector('#uxSettingsSearch');if(search)search.value='';
 body.dataset.activeSettingsSection=key;
 for(const panel of body.querySelectorAll('[data-settings-panel]')){panel.hidden=panel!==section;panel.classList.remove('ux-settings-match');}
 for(const button of body.querySelectorAll('[data-settings-section]')){button.hidden=false;button.setAttribute('aria-pressed',String(button.dataset.settingsSection===key));}
 body.querySelector('.ux-settings-content')?.classList.remove('ux-settings-searching');
 return true;
}
function enhance(category='General'){
 const modal=root.document.querySelector('#modalRoot .modal'),body=modal?.querySelector('.modal-body');
 if(!body||!modal.querySelector('#modalTitle')?.textContent?.includes('App settings'))return false;
 if(body.querySelector('.ux-settings-layout'))return select(body,category);
 modal.classList.add('wide','ux-settings-dialog');
 const original=[...body.children],layout=make('div','ux-settings-layout'),nav=make('nav','ux-settings-nav'),content=make('div','ux-settings-content');
 nav.setAttribute('aria-label','Settings sections');
 const search=make('input');search.type='search';search.id='uxSettingsSearch';search.placeholder='Search settings';search.setAttribute('aria-label','Search settings');nav.append(search);
 const sections=new Map();
 for(const name of CATEGORIES){
  const key=name.toLowerCase(),button=make('button','btn small ghost',name);button.type='button';button.dataset.settingsSection=key;button.setAttribute('aria-pressed','false');nav.append(button);
  const section=make('section','ux-settings-section');section.dataset.settingsPanel=key;section.append(make('h3','',name));content.append(section);sections.set(key,section);
 }
 const general=sections.get('general'),appearance=sections.get('appearance');
 // Reparent the original inputs and handlers, never clone or rename them.
 for(const node of original){
  if(node.id==='resultTimeSettings'||node.classList.contains('app-accent-settings')||node.querySelector?.('#settingsTheme'))appearance.append(node);
  else if(node.classList.contains('form-stack')){
   for(const child of [...node.children]){
    const id=child.id||'';
    if(id==='appUpdateSettings')sections.get('updates').append(child);
    else if(id==='storageSettings'||id==='vendorSettings')general.append(child);
    else if(child.querySelector?.('[data-action="quit-confirm"]'))sections.get('confirmations').append(child);
    else general.append(child);
   }
  }else general.append(node);
 }
 for(const [key,message] of Object.entries(notes)){
  const section=sections.get(key);section.append(make('p','help-line',message));
  for(const [label,page] of tools[key]||[]){
   const button=make('button','btn small ghost ux-settings-tool-link',label);button.type='button';button.dataset.settingsGo=page;
   button.title=label;section.append(button);
  }
 }
 layout.append(nav,content);body.append(layout);
 nav.addEventListener('click',event=>{
  const categoryButton=event.target.closest('[data-settings-section]');
  if(categoryButton){select(body,categoryButton.dataset.settingsSection);return;}
 });
 content.addEventListener('click',event=>{
  const button=event.target.closest('[data-settings-go]');
  if(button&&typeof openTool==='function')openTool(button.dataset.settingsGo);
 });
 search.addEventListener('input',()=>{
  const query=search.value.trim().toLocaleLowerCase(),searching=!!query;
  content.classList.toggle('ux-settings-searching',searching);
  for(const [key,section] of sections){
   const match=!searching||section.textContent.toLocaleLowerCase().includes(query);
   section.hidden=!match;section.classList.toggle('ux-settings-match',searching&&match);
   const button=nav.querySelector('[data-settings-section="'+key+'"]');if(button)button.hidden=!match;
  }
  if(!searching)select(body,body.dataset.activeSettingsSection||'general');
 });
 select(body,category);
 return true;
}
root.NetworkToolkitSettings=Object.freeze({configure,enhance,select,offerUndo});
})(globalThis);
