(function(root){
'use strict';
const CATEGORIES=['General','Appearance','Terminal','Editor','Graphs','Transfers','Network','Updates','Confirmations'];
function enhance(category='General'){
 const modal=root.document.querySelector('#modalRoot .modal');
 const body=modal?.querySelector('.modal-body');
 if(!body||!modal.querySelector('#modalTitle')?.textContent?.includes('App settings'))return false;
 if(body.querySelector('.ux-settings-layout'))return select(body,category);
 modal.classList.add('wide','ux-settings-dialog');
 const original=[...body.children];
 const layout=root.document.createElement('div');
 layout.className='ux-settings-layout';
 const nav=root.document.createElement('nav');nav.className='ux-settings-nav';nav.setAttribute('aria-label','Settings sections');
 const search=root.document.createElement('input');search.type='search';search.id='uxSettingsSearch';search.placeholder='Search settings';search.setAttribute('aria-label','Search settings');
 nav.append(search);
 const content=root.document.createElement('div');content.className='ux-settings-content';
 const sections=new Map();
 for(const name of CATEGORIES){
  const key=name.toLowerCase();
  const button=root.document.createElement('button');button.type='button';button.className='btn small ghost';button.dataset.settingsSection=key;button.textContent=name;button.setAttribute('aria-pressed','false');
  nav.append(button);
  const section=root.document.createElement('section');section.className='ux-settings-section';section.dataset.settingsPanel=key;
  const heading=root.document.createElement('h3');heading.textContent=name;section.append(heading);
  content.append(section);sections.set(key,section);
 }
 // Reparent the original, already-wired controls. No control is cloned,
 // and all existing listeners, IDs and saved preference keys are preserved.
 const general=sections.get('general'),appearance=sections.get('appearance');
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
 const help={
  terminal:'Terminal appearance, tab layout, sidebar and session confirmations remain available from the Terminal workspace. Existing settings are unchanged.',
  editor:'Text Editor appearance, wrapping, fonts and saving options remain available from the Editor controls.',
  graphs:'Graph color, zoom and timeline preferences remain available in their existing graph controls.',
  transfers:'File transfer and SFTP/FTP/TFTP preferences remain available in the corresponding transfer tools.',
  network:'Network adapter and Hosts file changes still require review and Windows authorization.',
  confirmations:'Security, unsaved documents, file overwrites, active connections and privileged changes continue to require the existing confirmations.'
 };
 for(const [key,message] of Object.entries(help)){
  const p=root.document.createElement('p');p.className='help-line';p.textContent=message;sections.get(key).append(p);
 }
 layout.append(nav,content);body.append(layout);
 nav.addEventListener('click',event=>{const button=event.target.closest('[data-settings-section]');if(button)select(body,button.dataset.settingsSection);});
 search.addEventListener('input',()=>{
  const query=search.value.trim().toLocaleLowerCase();
  for(const [key,section] of sections){
   section.hidden=!!query&&!section.textContent.toLocaleLowerCase().includes(query);
  }
  for(const button of nav.querySelectorAll('[data-settings-section]')){
   button.hidden=sections.get(button.dataset.settingsSection).hidden;
  }
  content.classList.toggle('ux-settings-searching',!!query);
  if(query){for(const section of sections.values())if(!section.hidden)section.classList.add('ux-settings-match');}
  else{for(const section of sections.values())section.classList.remove('ux-settings-match');select(body,body.dataset.activeSettingsSection||'general');}
 });
 select(body,category);
 return true;
}
function select(body,name){
 const key=String(name||'general').toLowerCase();
 const section=body.querySelector('[data-settings-panel="'+key+'"]');
 if(!section)return false;
 body.dataset.activeSettingsSection=key;
 const search=body.querySelector('#uxSettingsSearch');
 if(search?.value)search.value='';
 for(const panel of body.querySelectorAll('[data-settings-panel]')){panel.hidden=panel!==section;panel.classList.remove('ux-settings-match');}
 for(const button of body.querySelectorAll('[data-settings-section]')){button.hidden=false;button.setAttribute('aria-pressed',String(button.dataset.settingsSection===key));}
 body.querySelector('.ux-settings-content')?.classList.remove('ux-settings-searching');
 return true;
}
root.NetworkToolkitSettings=Object.freeze({enhance,select});
})(globalThis);
