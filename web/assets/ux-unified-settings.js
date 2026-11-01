/* Phase 4: organize existing controls without replacing their saved state or handlers. */
(function(root){
'use strict';
const doc=root.document;
if(!doc)return;
const sections=[
 ['general','General','Reports, time display, application data and status'],
 ['appearance','Appearance','Theme and accent colors'],
 ['terminal','Terminal','Terminal display and shortcuts'],
 ['editor','Editor','Text editing preferences'],
 ['graphs','Graphs','Graph layout'],
 ['transfers','Transfers','File transfer preferences'],
 ['network','Network','Local network and vendor information'],
 ['updates','Updates','Automatic checks and beta options'],
 ['confirmations','Confirmations','Session and list confirmations']
];
let active=null,selectCurrent=null;
function element(tag,cls,text){
 const node=doc.createElement(tag);if(cls)node.className=cls;
 if(text!==undefined)node.textContent=text;return node;
}
function field(label,name,type,value,choices){
 const row=element('label','ux-settings-field'),text=element('span','',label);row.append(text);
 let input;
 if(type==='select'){
  input=element('select');for(const [key,title] of choices){
   const option=element('option','',title);option.value=key;option.selected=String(value)===key;input.append(option);
  }
 }else{input=element('input');input.type=type;input.value=String(value);if(type==='checkbox')input.checked=value===true;}
 input.dataset.uxPreference=name;
 if(type==='number'){input.min='10';input.max='24';input.step='1';}
 if(type==='range'){input.min='25';input.max='75';input.step='5';}
 row.append(input);return row;
}
function note(message){return element('p','help-line',message);}
function button(label,page){
 const item=element('button','btn small ghost',label);
 item.type='button';item.dataset.uxSettingsJump=page;return item;
}
function card(parent,heading,description){
 const section=element('div','ux-settings-card');
 section.append(element('h3','',heading),note(description));parent.append(section);return section;
}
function checked(state,key){return state[key]!==false;}
function enhance(ctx,selected='general'){
 const modal=doc.querySelector('#modalRoot .modal'),body=modal?.querySelector('.modal-body');
 if(!body||body.querySelector('.ux-settings-shell'))return false;
 const nodes=[...body.children],form=nodes.find(node=>node.classList.contains('form-stack'));
 const panels=new Map(),tabs=new Map();
 const shell=element('div','ux-settings-shell');
 shell.id='uxSettingsShell';
 const searchLabel=element('label','ux-settings-search');
 searchLabel.append(element('span','sr-only','Search settings'));
 const search=element('input');search.id='uxSettingsSearch';search.type='search';search.placeholder='Search settings';search.autocomplete='off';search.setAttribute('aria-label','Search settings');searchLabel.append(search);
 const columns=element('div','ux-settings-columns'),nav=element('nav','ux-settings-nav'),content=element('div','ux-settings-content');
 nav.setAttribute('role','tablist');nav.setAttribute('aria-orientation','vertical');nav.setAttribute('aria-label','Settings categories');
 for(const [id,title,description] of sections){
  const tab=element('button','ux-settings-tab',title);
  tab.type='button';tab.dataset.uxSettingsSection=id;tab.id='uxSettingsTab-'+id;
  tab.setAttribute('role','tab');tab.setAttribute('aria-controls','uxSettingsPanel-'+id);
  nav.append(tab);tabs.set(id,tab);
  const panel=element('section','ux-settings-panel');
  panel.id='uxSettingsPanel-'+id;panel.setAttribute('role','tabpanel');panel.setAttribute('aria-labelledby',tab.id);
  panel.append(element('h2','',title),note(description));
  content.append(panel);panels.set(id,panel);
 }
 const general=panels.get('general'),appearance=panels.get('appearance');
 for(const child of nodes){
  if(child===form)continue;
  if(child.id==='resultTimeSettings')general.append(child);
  else appearance.append(child);
 }
 if(form)for(const child of [...form.children]){
  if(child.id==='appUpdateSettings')panels.get('updates').append(child);
  else if(child.id==='vendorSettings')panels.get('network').append(child);
  else general.append(child);
 }
 const st=ctx.state,ed=st.editorPreferences||{};
 const terminal=panels.get('terminal');
 const terminalCard=card(terminal,'Terminal display','Changes to terminal display preferences are saved with the existing terminal settings.');
 terminalCard.append(field('Font size','terminalFontSize','number',st.terminalFontSize||13));
 terminalCard.append(field('Highlight command output','terminalHighlight','checkbox',checked(st,'terminalHighlight')));
 terminalCard.append(button('Open Terminal','terminal'));
 const editor=panels.get('editor');
 const editorCard=card(editor,'Text editing','These are the same preferences used by the Text Editor. Open documents and unsaved text are not replaced.');
 editorCard.append(field('Wrap long lines','editor.wordWrap','checkbox',ed.wordWrap===true));
 editorCard.append(field('Syntax highlighting','editor.syntax','checkbox',ed.syntax!==false));
 editorCard.append(field('Editor font size','editor.fontSize','number',ed.fontSize||13));
 editorCard.append(note('Unsaved-file close protection remains available in the Text Editor. Existing close preferences are retained.'));
 editorCard.append(button('Open Text Editor','editor'));
 const graphs=panels.get('graphs'),graph=card(graphs,'Graph proportions','Set the initial graph and results split. Existing graph history and zoom settings are kept.');
 const ratio=Math.round(100*(Number(st.graphWorkspacePreferences?.ratio)||0.5));
 graph.append(field('Graph height (%)','graph.ratio','range',Math.min(75,Math.max(25,ratio))));
 graph.append(button('Open graph tool','monitor'));
 const transfers=panels.get('transfers'),transfer=card(transfers,'Preferred file browser','Saved transfer connections, protocols and permissions are unchanged.');
 transfer.append(field('Preferred browser','fileTransferProtocol','select',st.fileTransferProtocol==='ftp'?'ftp':'sftp',[['sftp','SFTP'],['ftp','FTP / FTPS']]));
 transfer.append(button('Open file transfers','sftp'));
 panels.get('network').append(button('Open network configuration','local'));
 const confirm=panels.get('confirmations'),confirmCard=card(confirm,'Existing confirmation choices','File overwrites, credential management, protected network changes, and update installation always retain their required approvals.');
 for(const [name,label] of [
  ['terminalConfirmClose','Ask before closing a live terminal'],
  ['terminalConfirmReconnectAll','Ask before reconnecting all sessions'],
  ['terminalConfirmDisconnectAll','Ask before disconnecting all sessions'],
  ['terminalConfirmCloseAll','Ask before closing all terminal tabs'],
  ['terminalSessionConfirmDelete','Ask before removing saved or recent session entries']
 ])confirmCard.append(field(label,name,'checkbox',checked(st,name)));
 confirmCard.append(note('Saving or discarding unsaved Editor changes remains governed by the Editor close policy. No security or file overwrite approvals are bypassed.'));
 body.replaceChildren(shell);shell.append(searchLabel,columns);columns.append(nav,content);
 modal.classList.add('ux-settings-modal');
 function select(id,focus=false){
  if(!panels.has(id)||tabs.get(id).hidden)return false;
  active=id;
  for(const [key,panel] of panels){
   const on=key===id;panel.hidden=!on;
   const tab=tabs.get(key);tab.setAttribute('aria-selected',String(on));tab.tabIndex=on?0:-1;
  }
  if(focus)tabs.get(id).focus({preventScroll:true});
  return true;
 }
 function matches(){
  const query=search.value.toLowerCase().trim().split(/\s+/).filter(Boolean);
  const match=sections.filter(([id,title,description])=>{
   const text=(title+' '+description+' '+panels.get(id).textContent).toLowerCase();
   return query.every(word=>text.includes(word));
  }).map(([id])=>id);
  for(const [id,tab] of tabs)tab.hidden=!match.includes(id);
  if(!match.includes(active))select(match[0]||'',false);
  if(!match.length){for(const panel of panels.values())panel.hidden=true;empty.hidden=false;}
  else empty.hidden=true;
 }
 const empty=element('p','ux-settings-empty','No settings found. Try another word.');
 empty.hidden=true;columns.append(empty);
 search.addEventListener('input',matches);
 nav.addEventListener('click',event=>{const tab=event.target.closest('[data-ux-settings-section]');if(tab)select(tab.dataset.uxSettingsSection,true);});
 nav.addEventListener('keydown',event=>{
  if(!['ArrowDown','ArrowUp','Home','End'].includes(event.key))return;
  const options=[...tabs.keys()].filter(key=>!tabs.get(key).hidden),i=options.indexOf(active);
  if(!options.length)return;
  const next=event.key==='Home'?0:event.key==='End'?options.length-1:(i+(event.key==='ArrowDown'?1:-1)+options.length)%options.length;
  event.preventDefault();select(options[next],true);
 });
 shell.addEventListener('click',event=>{
  const action=event.target.closest('[data-ux-settings-jump]');
  if(!action)return;
  const page=action.dataset.uxSettingsJump;
  ctx.closeModal();ctx.navigate(page);
 });
 shell.addEventListener('change',event=>{
  const control=event.target.closest('[data-ux-preference]');if(!control)return;
  const key=control.dataset.uxPreference;
  if(key==='terminalFontSize'){
   const n=Number(control.value);if(!Number.isInteger(n)||n<10||n>24){control.value=String(st.terminalFontSize||13);return;}
   st.terminalFontSize=n;ctx.updateTerminalTheme();root.NetworkToolkitTerminalAppearance?.sync?.();
  }else if(key==='terminalHighlight'){
   ctx.setTerminalHighlight(control.checked);
   return;
  }else if(key.startsWith('editor.')){
   st.editorPreferences={...(st.editorPreferences||{}),[key.slice(7)]:key==='editor.fontSize'?Math.max(10,Math.min(24,Number(control.value)||13)):control.checked};
   root.NetworkToolkitFileEditor?.refreshPreferences?.();
  }else if(key==='graph.ratio'){
   const n=Number(control.value);
   if(!Number.isFinite(n)||n<25||n>75)return;
   st.graphWorkspacePreferences={...(st.graphWorkspacePreferences||{}),ratio:n/100};
   const number=graph.querySelector('[data-ux-preference="graph.ratio"]');
   number?.setAttribute('aria-valuetext',n+'% graph height');
  }else if(key==='fileTransferProtocol'){
   if(!['sftp','ftp'].includes(control.value))return;
   st.fileTransferProtocol=control.value;
  }else if(Object.hasOwn(st,key)&&key.startsWith('terminalConfirm')||key==='terminalSessionConfirmDelete'){
   st[key]=control.checked;
  }else return;
  ctx.saveState();
 });
 selectCurrent=select;
 select(panels.has(selected)?selected:'general');
 return true;
}
root.NetworkToolkitUnifiedSettings=Object.freeze({enhance,show:id=>selectCurrent?.(id)===true});
})(window);
