/* Terminal links (ssh://, telnet://, serial:). Windows Default apps can open
 * these in the Network Toolkit Terminal (cmd/network-toolkit/terminal_link.go
 * parses them). A link only fills in the normal connection form: it never
 * connects by itself, never carries a password and never picks a saved
 * credential. When a saved connection matches it exactly, the form offers it
 * so its saved credential and settings can be used with one click.
 * Terminal settings > Features also gets a short "Links" block that opens
 * Windows Default apps in the installed edition. */
(function(root){
'use strict';
const document=root.document;
const SCHEMES={ssh:'SSH',telnet:'Telnet',serial:'Serial'};
const PASSWORD_NOTE='Passwords in links are ignored.';
const DEFAULTS_LABEL='Make Network Toolkit the default for SSH, Telnet and serial links';
const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]);
const ico=name=>{try{return typeof icon==='function'?icon(name):'';}catch{return '';}};

// The window accepts only the metadata the app produced; anything else is dropped.
function normalize(link){
 if(!link||typeof link!=='object'||!Object.prototype.hasOwnProperty.call(SCHEMES,link.scheme))return null;
 return Object.freeze({scheme:link.scheme,passwordIgnored:link.passwordIgnored===true});
}
const hostKey=value=>String(value??'').trim().replace(/^\[|\]$/g,'').replace(/\.$/,'').toLowerCase();
const portOf=(item,type)=>{const n=Number(item?.port);return Number.isInteger(n)&&n>0?n:type==='telnet'?23:22;};
// Exact match on type, host, port and user (SSH); type, host and port (Telnet,
// which has no user name); type and port name (Serial).
function matches(profile,config){
 if(!profile||!config||typeof profile!=='object'||profile.type!==config.type)return false;
 if(config.type==='serial')return !!config.serialPort&&String(profile.serialPort||'').trim().toUpperCase()===String(config.serialPort).trim().toUpperCase();
 if(config.type!=='ssh'&&config.type!=='telnet')return false;
 if(!config.host||hostKey(profile.host)!==hostKey(config.host)||portOf(profile,config.type)!==portOf(config,config.type))return false;
 return config.type!=='ssh'||String(profile.username||'')===String(config.username||'');
}
function match(profiles,config){return (Array.isArray(profiles)?profiles:[]).find(profile=>profile&&typeof profile.id==='string'&&profile.id&&matches(profile,config))||null;}
function describe(config){
 const c=config||{};
 if(c.type==='serial'){
  const parity={none:'no parity',odd:'odd parity',even:'even parity',mark:'mark parity',space:'space parity'}[c.parity]||'no parity';
  return 'Serial '+c.serialPort+' at '+(c.baudRate||9600)+' baud, '+(c.dataBits||8)+' data bits, '+parity+', '+(c.stopBits||'1')+' stop bit'+(String(c.stopBits||'1')==='1'?'':'s');
 }
 const host=String(c.host||'').includes(':')?'['+c.host+']':String(c.host||'');
 return (SCHEMES[c.type]||'SSH')+' to '+(c.type==='ssh'&&c.username?c.username+'@':'')+host+' port '+portOf(c,c.type);
}
const profileName=profile=>{try{return root.NetworkToolkitTerminalPromptLabels?.labels(profile)?.name||profile.name||profile.host||profile.serialPort||'saved connection';}catch{return profile.name||'saved connection';}};
function noticeHTML(link,config,profile,using){
 const parts=['<strong>Opened from a link.</strong> '+escape(describe(config))+'. Check the details, then choose Connect. Nothing connects until you do.'];
 if(link.passwordIgnored)parts.push('<span class="terminal-link-password">'+PASSWORD_NOTE+'</span>');
 if(using)parts.push('Using saved connection <strong>'+escape(profileName(profile))+'</strong> with its saved settings and credential.');
 return ico('info')+'<div class="grow">'+parts.join(' ')+'</div>'+(profile&&!using?'<button type="button" class="btn small" data-terminal-link-profile="'+escape(profile.id)+'">Use saved connection '+escape(profileName(profile))+'</button>':'');
}
const draft=()=>{try{return typeof terminalDraft==='object'&&terminalDraft?terminalDraft:{};}catch{return {};}};
// The first field the user still has to fill in, otherwise Connect.
function focusTarget(){
 const c=draft(),field=id=>{const el=document.getElementById(id);return el&&!el.disabled&&el.getClientRects().length&&!String(el.value||'').trim()?el:null;};
 const ssh=c.type==='ssh',secret=ssh&&!c.credentialId;
 return field('con-host')||field('con-serialPort')||(ssh?field('con-username'):null)||(secret&&c.authMethod!=='key'&&c.authMethod!=='keyboard'?field('terminalPassword'):null)||(secret&&c.authMethod==='key'?field('terminalPrivateKey'):null)||document.getElementById('terminalConnectButton');
}
function focusForm(){root.setTimeout(()=>{if(document.getElementById('terminalLinkNotice'))focusTarget()?.focus();},0);}
function profiles(){try{return typeof state==='object'&&state?state.terminalProfiles:[];}catch{return [];}}
function render(notice){
 const form=document.getElementById('terminalConnectForm');if(!form||!notice)return;
 const using=notice.__using?profiles().find(p=>p.id===notice.__using)||null:null;
 notice.innerHTML=noticeHTML(notice.__link,notice.__config,using||match(profiles(),notice.__config),!!using);
}
// Called by openTerminalConnect after the connection form opens for a link.
function decorate(link,config){
 link=normalize(link);const form=document.getElementById('terminalConnectForm');if(!link||!form)return false;
 let notice=document.getElementById('terminalLinkNotice');
 if(!notice){notice=document.createElement('div');notice.id='terminalLinkNotice';notice.className='notice info terminal-link-notice';notice.setAttribute('role','status');form.prepend(notice);}
 notice.__link=link;notice.__config={...config};notice.__using='';render(notice);focusForm();return true;
}
function useProfile(id){
 const notice=document.getElementById('terminalLinkNotice'),profile=profiles().find(p=>p.id===id);
 if(!notice||!profile||!matches(profile,notice.__config))return;
 if(typeof chooseTerminalConnectionProfile==='function')chooseTerminalConnectionProfile(id);
 notice.__using=id;render(notice);focusForm();
}

// Terminal settings > Features: Links.
let registration=null,checking=null;
function settingsHTML(){
 const r=registration;
 if(!r?.supported)return '';
 const text=r.registered
  ?'Choose Network Toolkit Terminal for SSH, TELNET and SERIAL links in Windows Default apps. Each link opens the connection form for you to confirm.'+(r.current?'':' Links open the installed copy of Network Toolkit.')
  :'The Windows setup adds this. Windows cannot choose the portable edition as the default app for SSH, Telnet and serial links; install Network Toolkit with its setup to use it for links.';
 return '<h4>Links</h4><p class="hint" id="terminalLinkStatus">'+escape(text)+'</p>'+(r.registered?'<button type="button" class="btn small ghost" data-terminal-link-defaults>'+escape(DEFAULTS_LABEL)+'</button>':'');
}
function renderSettings(){const section=document.getElementById('terminalLinkSettings');if(!section)return;section.innerHTML=settingsHTML();section.hidden=!section.innerHTML;}
function refresh(){
 if(checking)return checking;
 if(typeof api!=='function')return Promise.resolve(null);
 checking=api('/terminal-links').then(value=>{registration=value&&typeof value==='object'?{supported:value.supported===true,registered:value.registered===true,current:value.current===true}:null;}).catch(()=>{registration=null;}).finally(()=>{checking=null;renderSettings();});
 return checking;
}
function mount(panel){
 if(!panel||document.getElementById('terminalLinkSettings'))return;
 const page=panel.querySelector('[data-terminal-settings-panel="behavior"]');if(!page)return;
 const section=document.createElement('section');section.id='terminalLinkSettings';section.className='terminal-tools-section terminal-behavior-settings terminal-link-settings';section.setAttribute('aria-label','Links');section.hidden=true;page.append(section);
 void refresh();
}
async function openDefaults(button){
 if(button)button.disabled=true;
 try{await api('/terminal-links/default-apps','POST',{});if(typeof toast==='function')toast('Windows Default apps is open. Choose Network Toolkit Terminal for SSH, TELNET and SERIAL.');}
 catch(error){if(typeof toast==='function')toast(error.message||String(error),'error');}
 finally{if(button)button.disabled=false;void refresh();}
}
if(document)document.addEventListener('click',event=>{
 const use=event.target.closest?.('[data-terminal-link-profile]');if(use){event.preventDefault();useProfile(use.dataset.terminalLinkProfile);return;}
 const defaults=event.target.closest?.('[data-terminal-link-defaults]');if(defaults){event.preventDefault();void openDefaults(defaults);}
});
root.NetworkToolkitTerminalLinks=Object.freeze({PASSWORD_NOTE,DEFAULTS_LABEL,normalize,matches,match,describe,noticeHTML,decorate,useProfile,focusTarget,mount,refresh,settingsHTML,setRegistration(value){registration=value;renderSettings();}});
if(typeof module==='object'&&module.exports)module.exports=root.NetworkToolkitTerminalLinks;
})(typeof window!=='undefined'?window:globalThis);
