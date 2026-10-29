/* SSH/SFTP and FTP remote file browser. Credentials stay in process memory and
   are submitted only to the authenticated local transfer API. */
(function(root){'use strict';
const TYPES=new Set(['ssh','ftp']);
const configurations={
 ssh:{host:'',port:22,username:'',authMode:'password',password:'',keyPath:'',passphrase:'',fingerprint:'',security:'ssh',sshCompatibility:'modern',credentialId:'',profileName:''},
 ftp:{host:'',port:21,username:'',authMode:'password',password:'',fingerprint:'',security:'explicit',credentialId:'',profileName:''}
};
const model={};
const detachedEditor=()=>root.NetworkToolkitEditorFiles?.detached?.()===true;
let clipboard=null, contextMenu=null;
const escape=value=>esc(value??'');
const simpleName=value=>typeof value==='string'&&value!=='.'&&value!=='..'&&!/[\\/\0\r\n]/.test(value)&&value.length<256;
const joinRemote=(base,name)=>((base||'.')==='/'?'/':String(base||'.').replace(/\/$/,'')+'/')+name;
const joinLocal=(base,name)=>String(base).replace(/[\\/]$/,'')+(String(base).includes('\\')?'\\':'/')+name;
const dirname=(v,side)=>{if(side==='remote'){const text=String(v||'.').replace(/\/$/,'');if(text==='/'||text==='.')return text==='/'?'/':'.';const pos=text.lastIndexOf('/');return pos===0?'/':pos<0?'.':text.slice(0,pos)}return String(v||'')};
const bytes=n=>{if(n===null||n===undefined)return '-';let v=Number(n);if(!Number.isFinite(v))return '-';for(const u of ['B','KB','MB','GB','TB']){if(v<1024||u==='TB')return v.toLocaleString(undefined,{maximumFractionDigits:u==='B'?0:1})+' '+u;v/=1024}return '-'};
const when=x=>{const d=new Date(x);return !x||isNaN(d.getTime())?String(x||'-'):d.toLocaleString()};
function newPane(){return {path:'',parent:'',home:'',entries:[],loading:false,history:[],index:-1,filter:'',sort:'name',reverse:false,selected:new Set(),anchor:-1,initialized:false,requestId:0,pending:null};}
function get(k){if(!model[k])model[k]={local:newPane(),remote:newPane(),connected:false,connecting:false,connectionId:'',borrowedConnection:false,connectionOptions:[],connectionEpoch:0,sessionId:'',terminalCandidate:'',sessionOptions:[],autoTerminal:'',sourceChoice:'manual',showSettings:false,profileId:'',editingProfileId:'',libraryOpen:false,libraryTab:'saved',libraryFilter:'',busy:false,cancelPending:false,queue:[],lastError:'',split:50,activity:[],active:'remote',saved:false};return model[k]}
function loadSaved(k){const m=get(k);if(m.saved)return;m.saved=true;const saved=state.remoteBrowserSettings?.[k];if(!saved||typeof saved!=='object')return;
 for(const key of ['host','port','username','authMode','keyPath','fingerprint','security','sshCompatibility','credentialId','profileName'])if(Object.hasOwn(saved,key))configurations[k][key]=saved[key];
 if(k==='ssh'&&configurations.ssh.sshCompatibility!=='legacy')configurations.ssh.sshCompatibility='modern';
 if(typeof saved.localPath==='string')m.local.path=saved.localPath;
 if(typeof saved.remotePath==='string')m.remote.path=saved.remotePath;
 if(typeof saved.split==='number')m.split=Math.max(26,Math.min(74,saved.split));
}
function remember(k){state.remoteBrowserSettings=state.remoteBrowserSettings||{};const c=configurations[k],m=get(k);state.remoteBrowserSettings[k]={host:c.host,port:c.port,username:c.username,authMode:c.authMode,keyPath:c.keyPath,fingerprint:c.fingerprint,sshCompatibility:c.sshCompatibility,security:c.security,credentialId:c.credentialId||'',profileName:c.profileName||'',localPath:m.local.path,remotePath:m.remote.path,split:m.split};saveState()}
function pageActive(k){return !!sidebarMount(k)||(state.activePage===k||state.activePage==='sftp'&&k==='ssh')&&window.NetworkToolkitTransfers?.mode(state.activePage)==='client'}
const sidebarMounts=new Map();
function sidebarMount(k){const value=sidebarMounts.get(k),editor=value?.host?.closest?.('#networkToolkitFileEditor');return value?.host?.isConnected&&!value.host.closest('[data-ed-files-sidebar]')?.hidden&&editor&&!editor.hidden?value:null;}
function rootNode(k){const mount=sidebarMount(k);return mount?mount.host.querySelector('[data-rb-root="'+k+'"]'):document.querySelector('#pageContent [data-rb-root="'+k+'"]')}
function mountSidebar(host,side='local',protocol='ssh'){if(!TYPES.has(protocol)||detachedEditor()&&protocol!=='ssh')throw Error('This editor may use approved SFTP connections only.');for(const [k,value] of sidebarMounts)if(value.host===host)sidebarMounts.delete(k);sidebarMounts.set(protocol,{host,side:side==='remote'?'remote':'local'});get(protocol).active=side==='remote'?'remote':'local';render(protocol);}
function unmountSidebar(host){for(const [k,value] of sidebarMounts)if(value.host===host){sidebarMounts.delete(k);host.replaceChildren();if((state.activePage===k||k==='ssh'&&state.activePage==='sftp')&&root.NetworkToolkitTransfers?.mode(state.activePage)==='client')render(k);}}

async function requestPayload(k,action,extra={}){const c=configurations[k],m=get(k);const editorCheck=m.editorProfileCheck&&(!m.connected||['connect','fingerprint'].includes(action))?m.editorProfileCheck:null;
 if(k==='ssh'&&!['connect','fingerprint'].includes(action)&&m.connectionId)return {protocol:'ssh',action,connectionId:m.connectionId,...extra};
 if(k==='ssh'&&!['connect','fingerprint'].includes(action)&&m.sessionId)return {protocol:'ssh',action,sessionId:m.sessionId,...extra};
 if(k==='ssh'&&!['connect','fingerprint'].includes(action))throw new Error('Connect to an SSH/SFTP session before using remote files.');
 editorCheck?.();
 const details={protocol:k,action,host:c.host.trim(),port:Number(c.port),username:c.username.trim(),password:c.password,keyPath:c.keyPath,privateKey:c.privateKey||'',passphrase:c.passphrase,authMode:c.authMode,fingerprint:c.fingerprint.trim(),sshCompatibility:c.sshCompatibility,security:c.security,...extra};
 if(k==='ssh'&&action==='connect'&&(state.terminalProfiles||[]).some(row=>row.id===m.profileId&&row.type==='ssh'))details.profileId=m.profileId;
 // Only credential references enter the browser request. The engine resolves
 // saved secrets after destination authorization and host verification.
 let credentialId=k==='ftp'&&!c.password?c.credentialId:'';
 if(k==='ssh'&&m.profileId&&action!=='fingerprint'){
  const profile=sshRecord(m.profileId);if(profile?.timeoutMs)details.timeoutMs=Number(profile.timeoutMs);
  if(!profile||profile.host!==c.host||Number(profile.port)!==Number(c.port))throw new Error('Saved SSH profile changed. Choose the profile again.');
  if(!m.profileManualAuth)credentialId=c.credentialId||profile.credentialId||'';
 }else if(k==='ssh'&&!m.profileManualAuth)credentialId=c.credentialId||'';
 if(credentialId&&action!=='fingerprint'){
  const credential=await root.NetworkToolkitCredentials.prepare({...details,credentialId});
  editorCheck?.();
  details.credentialId=credential.credentialId;details.username=credential.username;details.authMode=credential.authMethod;
  for(const field of ['password','privateKey','passphrase','keyPath'])delete details[field];
 }
 if(action==='fingerprint')for(const field of ['password','privateKey','passphrase','keyPath','credentialId'])delete details[field];
 return details;
}
async function request(k,action,extra={}){const m=get(k),identity=connectionIdentity(k);try{return await api('/remote-browser','POST',await requestPayload(k,action,extra));}catch(error){if(k==='ssh'&&m.connected&&!['connect','fingerprint','open-terminal'].includes(action)&&identity===connectionIdentity(k)&&/SFTP connection (?:is |was )?(?:closed|not found|not connected)|SSH terminal is not available|SSH terminal is not connected|SSH connection (?:is )?closed|^EOF$|unexpected EOF|connection reset by peer|use of closed network connection|broken pipe/i.test(error.message||'')){invalidateConnection(k);restoreConnectionControls(k);paint(k,'remote','Connection closed. Use Reconnect to connect again.');note(k,'The SSH/SFTP connection closed. Use Reconnect to connect again.','error');}throw error;}}
function invalidateConnection(k,{clear=true,release=true}={}){const m=get(k),owned=m.connectionId,borrowed=m.borrowedConnection;m.borrowedConnection=false;m.connectionEpoch++;m.connectionId='';m.sessionId='';m.connected=false;m.connecting=false;m.remote.requestId++;m.remote.loading=false;m.remote.pending=null;if(clear){m.remote.entries=[];m.remote.initialized=false;m.remote.selected.clear();}root.NetworkToolkitNativeDrag?.cancelHover?.();if(owned&&release&&!borrowed&&!detachedEditor())void api('/remote-browser','POST',{protocol:'ssh',action:'disconnect',connectionId:owned}).catch(()=>{});return owned;}
async function closeOwnedConnection(k){const m=get(k),owned=m.connectionId,borrowed=m.borrowedConnection;m.pendingClose=(m.pendingClose||0)+1;try{invalidateConnection(k,{release:false});if(owned&&!borrowed&&!detachedEditor())try{await api('/remote-browser','POST',{protocol:'ssh',action:'disconnect',connectionId:owned});}catch(error){if(!/not found|closed/i.test(error.message||''))throw error;}}finally{m.pendingClose--;}}

function note(k,message,mode='info'){const m=get(k);m.lastError=mode==='error'?message:'';const node=rootNode(k)?.querySelector('[data-rb-feedback]');if(node){node.hidden=!message;node.textContent=message||'';node.className='rb-notice '+mode}}
function options(values,current){return values.map(([v,label])=>'<option value="'+escape(v)+'"'+(v===current?' selected':'')+'>'+escape(label)+'</option>').join('')}
function sshKnownRecords(){return [...(state.terminalProfiles||[]).filter(x=>x?.type==='ssh'&&x.host),...(state.terminalRecent||[]).filter(x=>x?.type==='ssh'&&x.host),...(state.sftpRecent||[]).filter(x=>x?.type==='ssh'&&x.host)];}
function sshRecord(id){return sshKnownRecords().find(x=>x.id===id)||null;}
function sftpRecent(){return Array.isArray(state.sftpRecent)?state.sftpRecent:[];}
function rememberSFTPRecent(){const c=configurations.ssh,m=get('ssh');if(!c.host||!c.username||!m.connected||m.sessionId)return;
 const prior=sftpRecent().find(x=>x.host===c.host&&Number(x.port)===Number(c.port)&&x.username===c.username);
 const current=sshRecord(m.profileId);const profile={id:prior?.id||'sftp-recent-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,7),name:current?.name||c.host,type:'ssh',host:c.host,port:Number(c.port),username:c.username,authMethod:c.authMode==='key'?'key':'password',credentialId:c.credentialId||current?.credentialId||'',fingerprint:c.fingerprint,sshCompatibility:c.sshCompatibility,lastUsed:Date.now()};
 state.sftpRecent=[profile,...sftpRecent().filter(x=>x.id!==profile.id)].slice(0,50);
 const recent=Array.isArray(state.terminalRecent)?state.terminalRecent:[];
 const matched=recent.find(x=>x.type==='ssh'&&x.host===profile.host&&Number(x.port)===profile.port&&x.username===profile.username);
 const terminalEntry={...profile,id:matched?.id||'recent-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,8),timeoutMs:10000,sshCompatibility:current?.sshCompatibility||'modern'};
 state.terminalRecent=[terminalEntry,...recent.filter(x=>x.id!==terminalEntry.id)].slice(0,50);saveState();root.NetworkToolkitSessionSidebar?.render?.();}
function ftpProfiles(){return Array.isArray(state.ftpProfiles)?state.ftpProfiles:[];}
function ftpRecent(){return Array.isArray(state.ftpRecent)?state.ftpRecent:[];}
function rememberFTPRecent(){const c=configurations.ftp,m=get('ftp');if(!m.connected||!c.host||!c.username)return;
 const found=ftpRecent().find(x=>x.host===c.host&&Number(x.port)===Number(c.port)&&x.username===c.username&&x.security===c.security);
 const named=ftpProfileRow(m.profileId);
 const item={id:found?.id||'ftp-recent-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,8),name:named?.name||c.host,host:c.host,port:Number(c.port),username:c.username,security:c.security,fingerprint:c.fingerprint,credentialId:c.credentialId||named?.credentialId||'',lastUsed:Date.now()};
 state.ftpRecent=[item,...ftpRecent().filter(x=>x.id!==item.id)].slice(0,50);saveState();root.NetworkToolkitSessionSidebar?.render?.();}

function ftpProfileRow(id){return ftpProfiles().find(x=>x.id===id)||null;}
function syncCredentials(k){const c=configurations[k],node=rootNode(k)?.querySelector('[data-rb-credential]');if(!node)return;node.innerHTML=root.NetworkToolkitCredentials?.options?.(c.credentialId||'',k==='ftp'?'password':'')||'<option value="">Do not use a saved credential</option>';node.value=c.credentialId||'';}
async function ftpCredentialsRefresh(k='ftp'){if(detachedEditor())return;try{await root.NetworkToolkitCredentials?.refresh?.();syncCredentials(k);}catch(error){note(k,'Password manager: '+error.message,'error');}}

function ftpProfilePicker(){const m=get('ftp');const list=[['manual','Manual FTP/FTPS connection'],...ftpProfiles().map(x=>['profile:'+x.id,'Saved: '+x.name+' ('+x.username+'@'+x.host+':'+x.port+' / '+x.security.toUpperCase()+')']),...ftpRecent().map(x=>['recent:'+x.id,'Recent: '+x.name+' ('+x.username+'@'+x.host+':'+x.port+')'])];return '<div class="rb-source"><label for="rbFTPSource">Connection source</label><select id="rbFTPSource" class="connection-source-select" data-rb-connection-source aria-label="Saved FTP and FTPS connection profiles">'+options(list,m.sourceChoice||'manual')+'</select>'+
 '<button class="btn small primary" data-rb-action="selectSource">Use connection</button><button class="btn small ghost" data-rb-action="librarySaved">'+icon('bookmark')+' Saved</button><button class="btn small ghost" data-rb-action="libraryRecent">'+icon('clock')+' Recent</button><button class="btn small ghost" data-rb-action="toggleSettings" aria-expanded="'+String(m.showSettings)+'">'+(m.showSettings?'Hide settings':'Connection settings')+'</button><button class="btn small ghost" data-rb-action="saveProfile">Save profile</button><button class="btn small ghost" data-rb-action="manageProfiles">Manage profiles</button><button class="btn small ghost" data-rb-action="passwordManager">Password manager</button></div>';}
function loadFTPProfile(id,kind='saved'){const item=kind==='recent'?ftpRecent().find(x=>x.id===id):ftpProfileRow(id),m=get('ftp'),c=configurations.ftp;if(!item){c.credentialId='';c.profileName='';m.editingProfileId='';m.profileId='';m.sourceChoice='manual';m.connected=false;m.showSettings=true;render('ftp');return;}
 Object.assign(c,{host:item.host,port:item.port,username:item.username,security:item.security,fingerprint:item.fingerprint||'',password:'',credentialId:item.credentialId||'',profileName:item.name||''});
 m.profileId=kind==='saved'?id:'';m.sourceChoice=(kind==='recent'?'recent:':'profile:')+id;m.showSettings=!item.credentialId;m.connected=false;m.remote.entries=[];render('ftp');
 note('ftp',item.credentialId?'Profile loaded. Connect to use its protected credential.':'Profile loaded. Enter a password or link a protected credential under settings.','info');}
function saveFTPProfile(nameOverride){const c=configurations.ftp,m=get('ftp'),host=c.host.trim(),user=c.username.trim(),port=Number(c.port);
 if(!host||!user||!Number.isInteger(port)||port<1||port>65535){m.showSettings=true;render('ftp');note('ftp','Enter a valid host, port, and username before saving the profile.','error');return;}
 const existing=ftpProfileRow(m.profileId||m.editingProfileId),name=typeof nameOverride==='string'?nameOverride:prompt('FTP / FTPS profile name:',existing?.name||host);if(name===null)return;if(!name.trim()){note('ftp','Enter a profile name.','error');return;}
 if(!existing&&ftpProfiles().length>=100){note('ftp','The profile list has reached its 100-item limit.','error');return;}
 const id=existing?.id||'ftps-profile-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,8);
 const item={id,name:name.trim().slice(0,100),folder:root.NetworkToolkitSessionFolders.normalize(existing?.folder),host:host.slice(0,253),port,username:user.slice(0,256),security:['explicit','implicit','plain'].includes(c.security)?c.security:'explicit',fingerprint:c.fingerprint.trim().slice(0,256),credentialId:c.credentialId||''};
 state.ftpProfiles=[item,...ftpProfiles().filter(x=>x.id!==id)].slice(0,100);m.profileId=id;m.editingProfileId='';c.profileName=name.trim();m.sourceChoice='profile:'+id;saveState();root.NetworkToolkitSessionSidebar?.render?.();render('ftp');
 note('ftp','Connection profile saved without its password. Link a protected credential to reuse authentication.','success');}
function showFTPProfiles(){const entries=ftpProfiles();openModal('FTP / FTPS connection profiles','<p>Saved profiles are shared with the FTP/FTPS client. Connection settings remain in preferences; linked passwords are kept in the protected password vault.</p><div class="rb-ftp-profiles">'+(entries.map(x=>'<div class="rb-ftp-profile"><div><strong>'+escape(x.name)+'</strong><div class="help-line mono">'+escape(x.username+'@'+x.host+':'+x.port+' - '+x.security+(x.credentialId?' (credential linked)':''))+'</div></div><div class="row gap6"><button type="button" class="btn small ghost" data-ftp-profile-action="load" data-id="'+escape(x.id)+'">Load</button><button type="button" class="btn small ghost" data-ftp-profile-action="rename" data-id="'+escape(x.id)+'">Rename</button><button type="button" class="btn small danger" data-ftp-profile-action="delete" data-id="'+escape(x.id)+'">Delete</button></div></div>').join('')||'<p class="help-line">No saved FTP/FTPS profiles. Configure a connection and select Save profile.</p>')+'</div>','<button type="button" class="btn ghost" data-credential-action="manage">Password manager</button><button class="btn primary" data-action="close-modal">Done</button>',true);}
async function storeConnectionCredential(k){const c=configurations[k],m=get(k),identity=connectionIdentity(k),profileId=m.profileId||m.editingProfileId;if(!c.username.trim()){note(k,'Enter the username before saving a credential.','error');return;}
 m.pendingCredentialSave=(m.pendingCredentialSave||0)+1;try{let privateKey=c.privateKey||'';if(k==='ssh'&&c.authMode==='key'&&!privateKey&&c.keyPath){const data=await api('/file-editor/local','POST',{action:'read-text',path:c.keyPath});privateKey=data.content||'';}
 if(identity!==connectionIdentity(k))throw new Error('Connection settings changed. Review the destination before saving a credential.');
 const entry=await root.NetworkToolkitCredentials.saveConnection({protocol:k,security:c.security,fingerprint:c.fingerprint,port:c.port,credentialId:c.credentialId||'',name:c.profileName||c.host,host:c.host,username:c.username,authMethod:k==='ssh'&&c.authMode==='key'?'key':'password',password:c.password,privateKey,passphrase:c.passphrase});
 if(identity!==connectionIdentity(k))throw new Error('Connection settings changed. Select the saved credential for this connection.');
 c.credentialId=entry.id;c.password='';c.privateKey='';c.passphrase='';m.profileManualAuth=false;
 const profile=(k==='ssh'?state.terminalProfiles||[]:state.ftpProfiles||[]).find(row=>row.id===profileId);if(profile)profile.credentialId=entry.id;
 remember(k);render(k);root.NetworkToolkitSessionSidebar?.render?.();note(k,'Credential saved and linked to this connection'+(profile?' and its saved profile.':'. Save the profile to reuse these settings.'),'success');
 }catch(error){note(k,error.message,'error');}finally{m.pendingCredentialSave--;}}
async function approveConnectionCredential(k){const c=configurations[k],identity=connectionIdentity(k);try{await root.NetworkToolkitCredentials.approveConnection({...c,protocol:k});if(identity!==connectionIdentity(k))return;note(k,'Credential destination approved. Connect when ready.','success');}catch(error){if(identity===connectionIdentity(k))note(k,error.message,'error');}}
async function storeFTPPassword(){return storeConnectionCredential('ftp');}

function savedProfilePicker(k){
 if(k!=='ssh')return '';
 const m=get('ssh'),profiles=(state.terminalProfiles||[]).filter(x=>x.type==='ssh'&&x.host);
 return '<div class="rb-saved-profiles"><label for="rbSavedTerminalProfile">Saved SSH terminal connection</label><div class="rb-saved-profile-row"><select id="rbSavedTerminalProfile" data-rb-profile title="Use existing SSH terminal profile settings"><option value="">Manual SFTP connection</option>'+profiles.map(x=>'<option value="'+escape(x.id)+'"'+(m.profileId===x.id?' selected':'')+'>'+escape(x.name+' - '+x.username+'@'+x.host+':'+x.port+(x.credentialId?' (saved credential)':''))+'</option>').join('')+'</select><button type="button" class="btn small ghost" data-rb-action="useProfile"'+(!m.profileId?' disabled':'')+'>Load profile</button></div><p class="help-line">Use a terminal profile and its linked Windows-protected credential. An already connected terminal can be reused above without another login.</p></div>';
}
async function loadProfile(k,id){
 if(k!=='ssh')return;
 const m=get(k),profile=sshRecord(id);
 if(!profile){m.profileId='';return;}
 m.pendingProfileLoad=(m.pendingProfileLoad||0)+1;try{
 invalidateConnection(k);const c=configurations.ssh,changed=c.host!==profile.host||Number(c.port)!==Number(profile.port);
 m.profileId=id;m.terminalKnownHost=false;m.profileManualAuth=false;m.sessionId='';m.connected=false;m.terminalCandidate='';m.sourceChoice=((state.terminalRecent||[]).some(x=>x.id===id)||(state.sftpRecent||[]).some(x=>x.id===id)?'recent:':'profile:')+id;m.showSettings=false;m.remote.entries=[];
 c.credentialId=profile.credentialId||'';c.profileName=profile.name||'';m.editingProfileId='';c.host=profile.host;c.port=Number(profile.port)||22;c.username=profile.username||'';
 c.authMode=profile.authMethod==='key'?'key':'password';c.sshCompatibility=profile.sshCompatibility==='legacy'?'legacy':'modern';c.password='';c.privateKey='';c.passphrase='';c.keyPath='';
 if(changed)c.fingerprint='';
 // Reuse a host key that was previously confirmed by the terminal's trusted
 // known-hosts store. Unknown hosts still require manual independent verification.
 try{
  const data=await api('/terminals/known-hosts');
  const host=String(c.host),port=String(c.port);
  const address=host.includes(':')?'['+host+']:'+port:host+':'+port;
  const known=(data.hosts||[]).find(x=>String(x.host).toLowerCase()===address.toLowerCase());
  if(known&&known.fingerprint){c.fingerprint=known.fingerprint; m.terminalKnownHost=true;}
 }catch(_error){}
 // A saved profile without a linked credential must not silently try a blank
 // password or missing private key. Make manual authentication visible.
 const identity=String(c.host).toLowerCase()+':'+String(c.port);
 const override=state.sftpVerifiedHostKeys?.[identity];
 if(!m.terminalKnownHost&&typeof override==='string'&&/^SHA256:[A-Za-z0-9+/=]{20,100}$/.test(override))c.fingerprint=override;
 const needsLogin=!profile.credentialId;
 m.showSettings=!c.fingerprint;
 remember(k);render(k);
 note(k,!c.fingerprint?'This saved profile needs host-key verification before SFTP can connect.':needsLogin?'Profile loaded. Use connection to sign in with a password or private key.':'Trusted terminal host key loaded. Use connection to browse with the saved credential.','info');
 return profile;
 }finally{m.pendingProfileLoad--;}
}
// Profiles and live sessions share a picker, but a saved profile is NOT a live
// session. Finish its host verification and authentication instead of just
// loading fields or silently attempting an empty-password login.
function sshProfileById(id){return sshRecord(id);}
function profileReadyForAuthentication(profile){const c=configurations.ssh,m=get('ssh');return !!profile&&m.profileId===profile.id&&(m.sourceChoice==='profile:'+profile.id||m.sourceChoice==='recent:'+profile.id)&&c.host===profile.host&&Number(c.port)===Number(profile.port);}
function showSSHProfileSignIn(profile,credentialError=''){
 if(!profileReadyForAuthentication(profile))return;
 const key=profile.authMethod==='key';
 const body='<form id="rbSSHProfileAuthForm" data-rb-auth-profile="'+escape(profile.id)+'" autocomplete="off"><p>Sign in to <strong>'+escape(profile.name||profile.host)+'</strong> ('+escape(profile.username||'')+'@'+escape(profile.host)+':'+escape(profile.port)+'). Authentication remains in memory for this SFTP connection only.</p>'+
 (credentialError?'<div class="rb-notice error">'+escape(credentialError)+'</div>':'')+
 (key?'<label class="field"><span>SSH private key</span><textarea id="rbSSHProfilePrivateKey" rows="5" spellcheck="false" autocomplete="off" placeholder="Paste an OpenSSH or PEM private key, or provide a local key path below"></textarea></label><label class="field mt"><span>Private key file path (alternative)</span><input id="rbSSHProfileKeyPath" autocomplete="off" placeholder="C:\\Users\\...\\id_ed25519"></label><label class="field mt"><span>Key passphrase (optional)</span><input type="password" id="rbSSHProfilePassphrase" autocomplete="new-password" maxlength="4096"></label>':
 '<label class="field"><span>SSH password</span><input id="rbSSHProfilePassword" type="password" autocomplete="new-password" maxlength="4096" required></label>')+
 '<p class="help-line">For automatic sign-in next time, link a protected credential to this profile using Terminal connection profiles and Password Manager.</p></form>';
 openModal('SFTP sign-in: '+(profile.name||profile.host),body,'<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button type="submit" form="rbSSHProfileAuthForm" class="btn primary">Connect / Browse</button>',true);
}
async function showSSHProfileVerification(profile){
 const m=get('ssh'),c=configurations.ssh;
 m.showSettings=true;render('ssh');
 note('ssh','Verifying the SSH host key for '+profile.host+':'+profile.port+' before connecting.','info');
 let observed='';
 try{const response=await request('ssh','fingerprint');observed=String(response.fingerprint||'').trim();if(!observed.startsWith('SHA256:'))throw Error('The SSH server did not provide a SHA-256 fingerprint.');}
 catch(error){note('ssh','Could not inspect the host key: '+error.message+'. Inspect the host key and enter an independently verified fingerprint under Connection settings.','error');return;}
 if(!profileReadyForAuthentication(profile))return;
 openModal('Verify SSH host key: '+(profile.name||profile.host),
 '<form id="rbSSHProfileVerifyForm" data-rb-verify-profile="'+escape(profile.id)+'" autocomplete="off"><p>This host is not in the terminal trusted-hosts list. Confirm its fingerprint using a trusted source before connecting. A fingerprint read from the server itself is <strong>not</strong> proof of identity.</p><label class="field"><span>Fingerprint reported by server</span><div class="rb-fingerprint mono">'+escape(observed)+'</div></label><label class="field mt"><span>Independently verified SHA-256 fingerprint</span><input id="rbSSHProfileVerifiedFingerprint" autocomplete="off" spellcheck="false" placeholder="SHA256:..." required></label><p id="rbSSHProfileVerifyError" class="rb-notice error" hidden></p><p class="help-line">The verified value must match the reported host key. It will be remembered for this SFTP endpoint; passwords and private keys will not be saved.</p></form>',
 '<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button type="submit" form="rbSSHProfileVerifyForm" class="btn primary">Verify and continue</button>',true);
 // Keep the observed value in the pending dialog's lifetime only; a new
 // connection selection cannot inherit it.
 m.pendingFingerprint={profileId:profile.id,observed};
}
async function matchingTerminalSession(profile){
 try{const response=await api('/remote-browser');const sessions=(response.sessions||[]).filter(x=>x.id&&x.host);
  get('ssh').sessionOptions=sessions;
  return sessions.find(x=>String(x.host).toLowerCase()===String(profile.host).toLowerCase()&&Number(x.port)===Number(profile.port)&&String(x.username||'')===String(profile.username||''));
 }catch(_error){return null;}
}
async function useSavedSSHProfile(id,check=null){
 const m=get('ssh'),c=configurations.ssh,profile=sshProfileById(id);
 if(!profile){note('ssh','The selected SSH profile was deleted. Refresh and choose an existing profile.','error');return;}
 if(m.connecting||m.profileSelectionPending)return;
 m.profileSelectionPending=true;
 try{
  // The terminal has already authenticated and verified the host key. A
  if(m.connected&&m.connectionId&&!m.profileManualAuth&&m.profileId===id&&profile.host===c.host&&Number(profile.port)===Number(c.port)&&profile.username===c.username){note('ssh','Using the connected SSH/SFTP session.','success');return;}
  // matching active session is the authoritative SSH connection for SFTP.
  const selectedEpoch=m.connectionEpoch;const active=await matchingTerminalSession(profile);
  check?.();if(check&&selectedEpoch!==m.connectionEpoch)throw Error('The connection selection changed while choosing the saved profile.');
  if(active){await connectTerminal('ssh',active.id);return;}
  if(!profileReadyForAuthentication(profile))await loadProfile('ssh',id);
  check?.();
  if(!profileReadyForAuthentication(profile))return;
  if(!c.fingerprint){await showSSHProfileVerification(profile);return;}
  if(profile.credentialId&&!m.profileManualAuth){
   const selectedIdentity=connectionIdentity('ssh');
   try{await root.NetworkToolkitCredentials.prepare({...profile,protocol:'ssh'});check?.();if(check&&(selectedIdentity!==connectionIdentity('ssh')||!profileReadyForAuthentication(profile)))throw Error('The connection settings changed during credential approval. Choose the profile again.');}catch(error){m.showSettings=true;render('ssh');note('ssh',error.message,'error');return;}
   await connect('ssh');return;
  }
  if(c.authMode==='key'?(!!c.privateKey||!!c.keyPath):!!c.password){await connect('ssh');return;}
  showSSHProfileSignIn(profile);
 }catch(error){m.showSettings=true;render('ssh');note('ssh','Cannot use SSH profile: '+error.message,'error');}
 finally{m.profileSelectionPending=false;}
}
function resetSSHConnectionProfile(){configurations.ssh.credentialId='';configurations.ssh.profileName='';}
function resetSSHProfileAuth(){const m=get('ssh'),c=configurations.ssh;m.profileManualAuth=false;m.pendingFingerprint=null;c.password='';c.privateKey='';c.passphrase='';c.keyPath='';}
function sessionTools(k){const m=get(k),borrowed=k==='ssh'&&!!m.sessionId;if(sidebarMount(k))return '<section class="rb-session-tools nt-editor-files-session-tools" role="toolbar" aria-label="'+(k==='ssh'?'SFTP':'FTP / FTPS')+' active session tools"><span data-rb-endpoint class="rb-session-endpoint"></span><button type="button" class="btn small ghost" data-rb-action="refreshSession" aria-label="Refresh connection files" title="Refresh connection files"'+(!m.connected||m.connecting||m.busy?' disabled':'')+'>'+icon('refresh')+'</button><button type="button" class="btn small ghost" data-rb-action="connectionSettings" aria-label="Connection settings" title="Connection settings">'+icon('settings')+'</button><button type="button" class="btn small danger" data-rb-action="disconnect" aria-label="'+(borrowed?'Disconnect browser':'Disconnect')+'" title="'+(borrowed?'Disconnect browser; SSH terminal stays connected':'Disconnect file connection')+'"'+(!m.connected&&!m.connecting?' disabled':'')+'>'+icon('stop')+'</button><details class="nt-editor-files-session-menu"><summary class="btn small ghost" aria-label="More connection actions" title="More connection actions">⋯</summary><div><button type="button" class="btn small ghost" data-rb-action="reconnect"'+(m.connecting||m.busy||(!m.connected&&!configurations[k].host)?' disabled':'')+'>'+(borrowed?'Refresh connection':'Reconnect')+'</button>'+(k==='ssh'?'<button type="button" class="btn small ghost" data-rb-action="openTerminal"'+(!m.connected||m.connecting||m.busy?' disabled':'')+'>Open terminal</button>':'')+'<button type="button" class="btn small ghost" data-rb-action="copyEndpoint"'+(!m.connected?' disabled':'')+'>Copy server address</button></div></details></section>';const position=state.terminalActiveActionsPosition==='bottom'?'bottom':'top';return '<section class="rb-session-tools terminal-active-actions" data-position="'+position+'" role="toolbar" aria-label="'+(k==='ssh'?'SFTP':'FTP / FTPS')+' active session tools"><h3>Active session</h3><div class="terminal-toolgroup">'+[
 ['disconnect',borrowed?'Disconnect browser':'Disconnect','stop',!m.connected&&!m.connecting,borrowed?'Detach this browser. The SSH terminal remains connected.':'Close the active file connection.'],
 ['reconnect',borrowed?'Refresh connection':'Reconnect','refresh',m.connecting||m.busy||(!m.connected&&!configurations[k].host),borrowed?'Refresh the SFTP channel on the existing SSH connection.':'Close and reconnect to the server using these connection settings.'],
 ...(k==='ssh'?[['openTerminal','Open terminal','terminal',!m.connected||m.connecting||m.busy,borrowed?'Show this SSH session in Terminal.':'Open a terminal channel over this authenticated SSH connection.']]:[]),
 ['refreshSession','Refresh files','folder',!m.connected||m.connecting||m.busy,'Refresh both file panes.'],
 ['connectionSettings','Connection settings','settings',false,'Open connection settings.']
 ].map(([action,label,symbol,disabled,title])=>'<button type="button" class="btn small '+(action==='disconnect'?'danger':'ghost')+'" data-rb-action="'+action+'" title="'+title+'"'+(disabled?' disabled':'')+'>'+icon(symbol)+label+'</button>').join('')+'</div><label class="terminal-active-position"><span>Position</span><select data-rb-session-position aria-label="Active session button position"><option value="top"'+(position==='top'?' selected':'')+'>Top</option><option value="bottom"'+(position==='bottom'?' selected':'')+'>Bottom</option></select></label></section>';}
function positionSessionTools(k){const el=rootNode(k),tools=el?.querySelector('.rb-session-tools'),panes=el?.querySelector('[data-rb-panes]');if(!tools||!panes)return;if(sidebarMount(k)){const connection=el.querySelector('.nt-editor-files-connection');if(connection)connection.before(tools);return;}const position=state.terminalActiveActionsPosition==='bottom'?'bottom':'top';tools.dataset.position=position;if(position==='bottom')panes.after(tools);else panes.before(tools);const select=tools.querySelector('[data-rb-session-position]');if(select&&document.activeElement!==select)select.value=position;}
function restoreConnectionControls(k){const m=get(k),el=rootNode(k);if(!el)return;const select=el.querySelector('[data-rb-action="selectSource"]');if(select)select.disabled=m.connecting;const direct=el.querySelector('[data-rb-action="connect"]');if(direct){direct.disabled=m.connecting;direct.textContent=m.connecting?'Connecting...':'Connect / Browse';}const status=el.querySelector('[data-rb-status]');if(status){status.textContent=m.connecting?'Connecting...':m.connected?(m.sessionId?'Using SSH terminal':'Connected'):'Not connected';status.classList.toggle('good',m.connected);}
 const tools=el.querySelector('.rb-session-tools');if(tools){const replacement=document.createElement('div');replacement.innerHTML=sessionTools(k);tools.replaceWith(replacement.firstElementChild);positionSessionTools(k);}
 const endpoint=el.querySelector('[data-rb-endpoint]');if(endpoint){const session=m.sessionOptions.find(x=>x.id===m.sessionId),c=configurations[k],host=session?.host||c.host,user=session?.username||c.username,port=session?.port||c.port;endpoint.textContent=m.connected?((user?user+'@':'')+host+':'+port+(m.sessionId?' · Terminal session':' · '+(k==='ssh'?'SSH connection':'FTP/FTPS'))):'No active connection';endpoint.title=endpoint.textContent;}const summary=el.querySelector('.nt-editor-files-connection>summary');if(summary){const info=m.sessionOptions.find(x=>x.id===m.sessionId),c=configurations[k],protocol=k==='ssh'?'SFTP':'FTP / FTPS';summary.textContent=m.connected?protocol+' · '+(info?.host||c.host):protocol+' connection';summary.title=m.connected?(info?.username||c.username)+'@'+(info?.host||c.host)+':'+(info?.port||c.port):'Choose or connect to an '+protocol+' server';}}

function connectionPicker(k){
 if(k==='ftp')return ftpProfilePicker();
 if(k!=='ssh')return '';
 const m=get(k),profiles=(state.terminalProfiles||[]).filter(x=>x.type==='ssh'&&x.host);
 const terminalRecent=(state.terminalRecent||[]).filter(x=>x.type==='ssh'&&x.host);
 const sftpEntries=sftpRecent().filter(x=>x.host&&!terminalRecent.some(y=>y.host===x.host&&Number(y.port)===Number(x.port)&&y.username===x.username));
 const entries=[['manual','Manual SSH connection'],...m.connectionOptions.map(x=>['connection:'+x.id,'SFTP: '+(x.username||'')+'@'+x.host+':'+x.port]),...m.sessionOptions.map(x=>['session:'+x.id,'Active: '+(x.label||x.host)+' ('+(x.username||'')+'@'+x.host+':'+x.port+')']),...profiles.map(x=>['profile:'+x.id,'Saved: '+(x.name||x.host)+' ('+(x.username||'')+'@'+x.host+':'+x.port+')']),...terminalRecent.map(x=>['recent:'+x.id,'Recent SSH: '+(x.name||x.host)+' ('+(x.username||'')+'@'+x.host+':'+x.port+')']),...sftpEntries.map(x=>['recent:'+x.id,'Recent SFTP: '+(x.name||x.host)+' ('+(x.username||'')+'@'+x.host+':'+x.port+')'])];
 const current=m.sessionId?'session:'+m.sessionId:m.sourceChoice||'manual';
 return '<div class="rb-source"><label for="rbConnectionSource">Connection source</label><select id="rbConnectionSource" class="connection-source-select" data-rb-connection-source aria-label="Active SSH terminals, saved SSH profiles and manual login">'+options(entries,current)+'</select>'+
 '<button class="btn small primary" data-rb-action="selectSource"'+(m.connecting?' disabled':'')+'>'+(current==='manual'?'Configure':'Use connection')+'</button>'+
 '<button class="btn small ghost" data-rb-action="toggleSettings" aria-expanded="'+String(m.showSettings)+'">'+(m.showSettings?'Hide settings':'Connection settings')+'</button>'+
 '<button type="button" class="btn small ghost" data-rb-action="librarySaved" title="Browse saved SSH/SFTP sessions">'+icon('bookmark')+' Saved</button><button type="button" class="btn small ghost" data-rb-action="libraryRecent" title="Browse recent SSH/SFTP sessions">'+icon('clock')+' Recent</button>'+
 '<button type="button" class="btn small ghost" data-rb-action="saveProfile" title="Save SFTP login as a reusable terminal SSH profile">Save profile</button>'+
 '<button type="button" class="btn small ghost" data-rb-action="manageProfiles" title="Manage SSH profiles and protected credentials">Profiles</button>'+
 '</div>';
}
async function refreshTerminalChoices(k){
 if(k!=='ssh'||!pageActive(k))return;
 const before=get(k),epoch=before.connectionEpoch,expectedSession=before.sessionId,expectedConnection=before.connectionId,query=before.sessionQueryId=(before.sessionQueryId||0)+1;
 try{const data=await api('/remote-browser');const m=get(k);if(m.connectionEpoch!==epoch||m.sessionQueryId!==query)return;
  m.sessionOptions=(data.sessions||[]).filter(x=>x.id&&x.host);m.connectionOptions=(data.connections||[]).filter(x=>x.id&&x.host&&x.connected!==false);if(detachedEditor()){const select=rootNode(k)?.querySelector('[data-rb-connection-source]');if(select){select.innerHTML=options(delegatedChoices(m),m.sourceChoice);}return;}
  if(!pageActive(k))return;
  const picker=rootNode(k)?.querySelector('[data-rb-connection-source]');if(!picker)return;
  const selected=picker.value||m.sourceChoice||'manual';
  const profiles=(state.terminalProfiles||[]).filter(x=>x.type==='ssh'&&x.host);
  const terminalRecent=(state.terminalRecent||[]).filter(x=>x.type==='ssh'&&x.host);
  const sftpEntries=sftpRecent().filter(x=>x.host&&!terminalRecent.some(y=>y.host===x.host&&Number(y.port)===Number(x.port)&&y.username===x.username));
  picker.innerHTML=options([['manual','Manual SSH connection'],...m.connectionOptions.map(x=>['connection:'+x.id,'SFTP: '+(x.username||'')+'@'+x.host+':'+x.port]),...m.sessionOptions.map(x=>['session:'+x.id,'Active: '+(x.label||x.host)+' ('+(x.username||'')+'@'+x.host+':'+x.port+')']),...profiles.map(x=>['profile:'+x.id,'Saved: '+(x.name||x.host)+' ('+(x.username||'')+'@'+x.host+':'+x.port+')']),...terminalRecent.map(x=>['recent:'+x.id,'Recent SSH: '+(x.name||x.host)+' ('+(x.username||'')+'@'+x.host+':'+x.port+')']),...sftpEntries.map(x=>['recent:'+x.id,'Recent SFTP: '+(x.name||x.host)+' ('+(x.username||'')+'@'+x.host+':'+x.port+')'])],selected);
  if(!Array.from(picker.options).some(x=>x.value===selected)){m.sourceChoice='manual';picker.value='manual';}
  if(m.sessionId&&m.sessionId===expectedSession&&!m.sessionOptions.some(x=>x.id===m.sessionId)){invalidateConnection(k);note(k,'The SSH terminal disconnected. Open Terminal to reconnect the SSH session.','error');}
  if(m.connectionId&&m.connectionId===expectedConnection&&Array.isArray(data.connections)&&!data.connections.some(x=>x.id===m.connectionId&&x.connected!==false)){invalidateConnection(k);note(k,'The SFTP connection closed. Use Reconnect to connect again.','error');}
  restoreConnectionControls(k);
  picker.title=picker.selectedOptions?.[0]?.textContent||'';
 }catch(e){note(k,'SSH session list unavailable: '+e.message,'error');}
}
async function connectTerminal(k,sessionId,{preserveFolder=false}={}){
 const m=get(k);if(m.connecting||!sessionId)return;
 const sameSession=m.sessionId===sessionId;await closeOwnedConnection(k);const epoch=m.connectionEpoch;
 if(!sameSession&&!preserveFolder)m.remote=newPane();
 m.connecting=true;m.sessionId=sessionId;m.terminalCandidate=sessionId;m.sourceChoice='session:'+sessionId;m.profileId='';m.showSettings=false;
 resetSSHProfileAuth();render(k);
 try{const response=await request(k,'list',{path:m.remote.path||'.'});if(m.connectionEpoch!==epoch)return;
  m.connected=true;acceptListing(k,'remote',response,true);actionLog(k,'Reused active SSH terminal for SFTP file browsing','success');render(k);note(k,'SFTP browser is using the existing SSH terminal connection.','success');
 }catch(e){if(m.connectionEpoch!==epoch)return;m.connected=false;m.sessionId='';m.terminalCandidate='';render(k);note(k,'Cannot use SSH terminal for SFTP: '+e.message,'error');actionLog(k,'SFTP over terminal failed: '+e.message,'error');}
 finally{if(m.connectionEpoch===epoch)m.connecting=false;restoreConnectionControls(k);}
}

function useTerminalSession(sessionId){
 if(typeof sessionId!=='string'||!sessionId||sessionId.length>128)return;
 loadSaved('ssh');const m=get('ssh'),sameSession=m.sessionId===sessionId;invalidateConnection('ssh');if(!sameSession)m.remote=newPane();m.sessionId='';m.terminalCandidate=sessionId;m.autoTerminal=sessionId;m.connected=false;
 m.remote.entries=[];m.sourceChoice='session:'+sessionId;configurations.ssh.password='';configurations.ssh.passphrase='';
}
function libraryRows(k,kind){
 if(k==='ftp')return kind==='saved'?ftpProfiles():ftpRecent();
 if(kind==='saved')return (state.terminalProfiles||[]).filter(x=>x.type==='ssh'&&x.host);
 const entries=[...(state.terminalRecent||[]),...sftpRecent()].filter(x=>x.type==='ssh'&&x.host);
 const seen=new Set();return entries.filter(x=>{const key=[x.host,Number(x.port),x.username].join('|');if(seen.has(key))return false;seen.add(key);return true;}).sort((a,b)=>(b.lastUsed||0)-(a.lastUsed||0));
}
function libraryItem(k,kind,id){return libraryRows(k,kind).find(x=>x.id===id)||null;}
function libraryHTML(k){const m=get(k),kind=m.libraryTab==='recent'?'recent':'saved',raw=libraryRows(k,kind),filter=m.libraryFilter.trim().toLowerCase();
 const entries=raw.filter(x=>([x.name,x.host,x.username,x.security].join(' ').toLowerCase()).includes(filter));
 return '<section class="rb-session-library" data-rb-library="'+k+'"'+(!m.libraryOpen?' hidden':'')+' aria-label="'+(k==='ssh'?'SSH and SFTP':'FTP and FTPS')+' saved and recent sessions">'+
 '<div class="rb-library-heading"><strong>Connection library</strong><button type="button" class="iconbtn" data-rb-library-command="close" title="Close session library" aria-label="Close session library">'+icon('close')+'</button></div>'+
 '<div class="rb-library-tabs" role="tablist" aria-label="Session library">'+['saved','recent'].map(tab=>'<button type="button" role="tab" aria-selected="'+String(kind===tab)+'" data-rb-library-tab="'+tab+'">'+icon(tab==='saved'?'bookmark':'clock')+' '+(tab==='saved'?'Saved':'Recent')+' <small>'+libraryRows(k,tab).length+'</small></button>').join('')+'</div>'+
 '<label class="sr-only" for="rbLibrarySearch">Search connections</label><input type="search" id="rbLibrarySearch" data-rb-library-search placeholder="Search sessions" value="'+escape(m.libraryFilter)+'" autocomplete="off">'+
 '<div class="rb-library-list" role="tabpanel">'+(entries.map(item=>'<div class="session-library-row rb-library-row" data-rb-library-kind="'+kind+'" data-rb-library-id="'+escape(item.id)+'"><button type="button" class="session-library-entry" data-rb-library-command="open" title="Double-click or press Enter to connect"><span class="session-library-icon">'+icon(k==='ssh'?'terminal':'folder')+'</span><span class="session-library-text"><strong>'+escape(item.name||item.host)+'</strong><small>'+escape((item.username||'')+'@'+item.host+':'+item.port+(k==='ftp'?' · '+(item.security||'FTPS'):'')+(item.credentialId?' · Protected login':''))+'</small></span></button><button type="button" class="iconbtn" data-rb-library-command="edit" title="Edit connection" aria-label="Edit connection">'+icon('settings')+'</button><button type="button" class="iconbtn session-library-delete" data-rb-library-command="remove" title="Remove from '+kind+'" aria-label="Remove session">'+icon('close')+'</button></div>').join('')||'<p class="session-library-empty">'+(filter?'No matching sessions.':kind==='saved'?'No saved sessions yet.':'Connections you use appear here.')+'</p>')+'</div>'+
 '<div class="rb-library-footer"><button type="button" class="btn small ghost" data-rb-library-command="new">'+icon('plus')+' New</button><button type="button" class="btn small ghost" data-rb-library-command="save">'+icon('bookmark')+' Save current</button>'+(kind==='recent'?'<button type="button" class="btn small ghost" data-rb-library-command="clear">Clear recent</button>':'')+'</div><p class="help-line">Double-click to connect. Right-click a session for more actions.</p></section>';
}
function showLibrary(k,tab){const app=rootNode(k);if(app?.clientSidebarSelect){app.clientSidebarSelect(tab,true);return;}const m=get(k);m.libraryOpen=true;m.libraryTab=tab;render(k);}
function libraryNotice(k,msg){note(k,msg,'info');}
async function useLibraryItem(k,kind,id,editOnly=false){const entry=libraryItem(k,kind,id);if(!entry){note(k,'This connection no longer exists.','error');return;}
 get(k).libraryOpen=false;
 if(k==='ssh'){
  if(editOnly){if(typeof openTerminalConnect==='function')openTerminalConnect(entry,kind==='saved'?entry.id:'');else{await loadProfile('ssh',id);get('ssh').showSettings=true;render('ssh');}return;}
  get('ssh').sourceChoice=(kind==='saved'?'profile:':'recent:')+id;
  await useSavedSSHProfile(id);return;
 }
 loadFTPProfile(id,kind);
 if(editOnly){get('ftp').showSettings=true;render('ftp');return;}
 if(configurations.ftp.credentialId||configurations.ftp.password)await connect('ftp');
 else{get('ftp').showSettings=true;render('ftp');note('ftp','Enter a password or select a protected credential, then connect.','info');}
}
function libraryRename(k,kind,id,duplicate=false,promote=false){const item=libraryItem(k,kind,id);if(!item)return;
 openModal(duplicate?'Duplicate session':promote?'Save recent connection':'Rename connection','<form id="rbLibraryNameForm" data-rb-library-proto="'+k+'" data-rb-library-kind="'+kind+'" data-rb-library-id="'+escape(id)+'" data-rb-library-op="'+(duplicate?'duplicate':promote?'promote':'rename')+'"><label class="field">Connection name<input id="rbLibraryName" required maxlength="100" autocomplete="off" value="'+escape(duplicate?item.name+' copy':item.name)+'"></label></form>','<button class="btn ghost" data-action="close-modal">Cancel</button><button class="btn primary" form="rbLibraryNameForm" type="submit">Save</button>',true);
}
function saveConnectionDialog(k){const m=get(k),existing=k==='ssh'?sshRecord(m.profileId):ftpProfileRow(m.profileId),c=configurations[k];
 if(!c.host||!c.username){m.libraryOpen=false;m.showSettings=true;render(k);note(k,'Enter a host and username before saving a profile.','error');return;}
 openModal('Save '+(k==='ssh'?'SSH / SFTP':'FTP / FTPS')+' connection','<form id="rbLibrarySaveForm" data-rb-library-proto="'+k+'"><label class="field">Connection name<input id="rbLibraryName" maxlength="100" required autocomplete="off" value="'+escape(c.profileName||existing?.name||c.host)+'"></label><p class="help-line">Connection settings are saved without passwords or private keys. Choose a protected credential to reuse authentication.</p></form>','<button class="btn ghost" data-action="close-modal">Cancel</button><button class="btn primary" form="rbLibrarySaveForm" type="submit">Save profile</button>',true);
}
function commitLibraryName(form){const k=form.dataset.rbLibraryProto,kind=form.dataset.rbLibraryKind,id=form.dataset.rbLibraryId,operation=form.dataset.rbLibraryOp,name=String(form.querySelector('#rbLibraryName')?.value||'').trim();if(!TYPES.has(k)||!name||name.length>100)return;
 const item=libraryItem(k,kind,id);if(!item)return;
 const collection=k==='ssh'?'terminalProfiles':'ftpProfiles',current=Array.isArray(state[collection])?state[collection]:[];
 if(operation==='rename'){item.name=name;}else if(current.length>=100){toast('Saved connection list is full.','error');return;}
 else{const entry={...item,id:(k==='ssh'?'terminal-profile-':'ftps-profile-')+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,8),name};delete entry.lastUsed;state[collection]=[entry,...current];}
 saveState();closeModal();render(k);root.NetworkToolkitSessionSidebar?.render?.();}
function removeLibraryEntry(k,kind,id){const collection=kind==='saved'?(k==='ssh'?'terminalProfiles':'ftpProfiles'):(k==='ssh'?'sftpRecent':'ftpRecent');
 if(!libraryItem(k,kind,id))return;
 // SSH recent may originate in the terminal history, not SFTP-only history.
 if(k==='ssh'&&kind==='recent'){state.terminalRecent=(state.terminalRecent||[]).filter(x=>x.id!==id);state.sftpRecent=(state.sftpRecent||[]).filter(x=>x.id!==id);}
 else state[collection]=(state[collection]||[]).filter(x=>x.id!==id);
 if(get(k).profileId===id)get(k).profileId='';saveState();render(k);root.NetworkToolkitSessionSidebar?.render?.();}
function libraryCommand(k,command,kind='',id=''){
 const m=get(k);
 if(command==='close'){m.libraryOpen=false;render(k);return;}
 if(command==='new'){invalidateConnection(k);configurations[k].credentialId='';configurations[k].profileName='';m.editingProfileId='';m.libraryOpen=false;m.showSettings=true;m.sourceChoice='manual';m.profileId='';m.sessionId='';m.connected=false;if(k==='ssh')resetSSHProfileAuth();else{configurations[k].credentialId='';configurations[k].password='';}render(k);rootNode(k)?.clientSidebarSelect?.('settings');return;}
 if(command==='save'){saveConnectionDialog(k);return;}
 if(command==='clear'){if(!confirm('Clear recent '+(k==='ssh'?'SSH/SFTP':'FTP/FTPS')+' connections? Saved profiles will remain.'))return;
  if(k==='ssh'){state.sftpRecent=[];state.terminalRecent=(state.terminalRecent||[]).filter(x=>x.type!=='ssh');}else state.ftpRecent=[];
  saveState();render(k);return;}
 if(!libraryItem(k,kind,id))return;
 if(command==='open'){void useLibraryItem(k,kind,id);return;}
 if(command==='edit'){void useLibraryItem(k,kind,id,true);return;}
 if(command==='remove'){removeLibraryEntry(k,kind,id);return;}
 if(command==='rename')libraryRename(k,kind,id);if(command==='duplicate')libraryRename(k,kind,id,true);if(command==='promote')libraryRename(k,kind,id,false,true);
}

function delegatedChoices(m){return [['','Choose connection'],...m.sessionOptions.map(x=>['session:'+x.id,'Terminal: '+(x.label||x.host)]),...m.connectionOptions.map(x=>['connection:'+x.id,'SFTP: '+(x.username||'')+'@'+x.host+':'+x.port])];}
async function connectExisting(k,id){const m=get(k),info=m.connectionOptions.find(x=>x.id===id);if(!info||m.connecting)return;await closeOwnedConnection(k);m.connectionId=id;m.borrowedConnection=true;m.sourceChoice='connection:'+id;m.remote=newPane();Object.assign(configurations[k],{host:info.host,port:info.port,username:info.username});m.connecting=true;try{const result=await request(k,'list',{path:'.'});m.connected=true;acceptListing(k,'remote',result,true);render(k);}catch(error){m.connected=false;note(k,error.message,'error');}finally{m.connecting=false;restoreConnectionControls(k);}}
function connectionUI(k){const c=configurations[k],m=get(k),expanded=true;if(detachedEditor())return '<section class="panel rb-connect"><label>Approved connection<select data-rb-connection-source>'+options(delegatedChoices(m),m.sourceChoice)+'</select></label><div class="row wrap"><button type="button" class="btn small primary" data-rb-action="selectSource">Use connection</button><button type="button" class="btn small ghost" data-rb-action="renewAccess">Update access</button></div><p class="help-line">Open new connections in the main window, then update access here.</p><div class="rb-notice" data-rb-feedback hidden></div></section>';
 return '<section class="panel rb-connect"><div class="rb-connect-head"><strong>'+(k==='ssh'?'SFTP connection':'FTP / FTPS connection')+'</strong><span class="pill '+(m.connected?'good':'')+'" data-rb-status>'+(m.connected?(m.sessionId?'Using terminal':'Connected'):'Not connected')+'</span><span class="rb-session-endpoint" data-rb-endpoint></span><button type="button" class="btn small ghost" data-rb-action="copyEndpoint">Copy server address</button></div>'+connectionPicker(k)+libraryHTML(k)+
 '<label class="rb-profile-name">Profile name (optional)<input data-rb-setting="profileName" value="'+escape(c.profileName||'')+'" maxlength="100" autocomplete="off"></label>'+
 '<div class="rb-connection-details"'+(!expanded?' hidden':'')+'><div class="rb-connection-fields"'+(m.sessionId?' hidden':'')+'>'+
 '<label>Server<input data-rb-setting="host" placeholder="Host or IP" value="'+escape(c.host)+'" autocomplete="off"></label>'+
 '<label>Port<input data-rb-setting="port" type="number" min="1" max="65535" value="'+escape(c.port)+'"></label>'+
 '<label>Username<input data-rb-setting="username" value="'+escape(c.username)+'" autocomplete="username"></label>'+
 (k==='ssh'?'<label>Authentication<select data-rb-setting="authMode">'+options([['password','Password'],['key','SSH private key']],c.authMode)+'</select></label>':'<label>Security<select data-rb-setting="security">'+options([['explicit','Explicit FTPS (TLS)'],['implicit','Implicit FTPS (TLS)'],['plain','Plain FTP (unencrypted)']],c.security)+'</select></label>')+
 '<label>'+(k==='ssh'&&c.authMode==='key'?'Key passphrase':'Password')+'<input type="password" data-rb-setting="'+(k==='ssh'&&c.authMode==='key'?'passphrase':'password')+'" value="'+escape(k==='ssh'&&c.authMode==='key'?c.passphrase:c.password)+'" autocomplete="off"></label>'+
 '<label>Saved credential<select data-rb-credential'+(k==='ftp'?' data-rb-ftp-credential':'')+'>'+ (root.NetworkToolkitCredentials?.options?.(c.credentialId||'',k==='ftp'?'password':'')||'<option value="">Do not use a saved credential</option>') +'</select></label><div class="rb-credential-controls"><button type="button" class="btn small ghost" data-rb-action="saveCredential">Save / update credential</button><button type="button" class="btn small ghost" data-rb-action="approveCredential">Approve destination</button><button type="button" class="btn small ghost" data-rb-action="passwordManager">Password manager</button></div>'+
 (k==='ssh'?'<label>SSH compatibility<select data-rb-setting="sshCompatibility" title="Matches the terminal SSH profile. Legacy is optional and allows older key exchanges and ciphers.">'+options([['modern','Modern (default)'],['legacy','Legacy (explicit opt-in)']],c.sshCompatibility)+'</select></label>':'')+
 (k==='ssh'&&c.authMode==='key'?'<label class="rb-long">SSH private key<textarea data-rb-setting="privateKey" spellcheck="false" autocomplete="off" placeholder="Paste an OpenSSH or PEM private key">'+escape(c.privateKey||'')+'</textarea></label><div class="rb-credential-controls"><button type="button" class="btn small ghost" data-rb-action="chooseKey">Choose key file</button><input type="file" data-rb-key-file hidden></div>':'')+
 (k==='ssh'?'<label>SSH private key path<input data-rb-setting="keyPath" value="'+escape(c.keyPath)+'" placeholder="C:\\Users\\...\\id_ed25519" '+(c.authMode==='key'?'':'disabled')+'></label>':'')+
 '<label class="rb-long">'+(k==='ssh'?'Verified host fingerprint (SHA-256)':'FTPS certificate pin (optional)')+'<input class="mono" data-rb-setting="fingerprint" value="'+escape(c.fingerprint)+'" placeholder="SHA256:..." autocomplete="off"></label></div>'+ 
 '<div class="rb-connection-buttons"'+(m.sessionId?' hidden':'')+'><button class="btn small ghost" data-rb-action="inspect">Inspect '+(k==='ssh'?'host key':'certificate')+'</button><button class="btn small primary" data-rb-action="connect"'+(m.connecting?' disabled':'')+'>'+(m.connecting?'Connecting...':'Connect / Browse')+'</button></div>'+
 '<p class="help-line">'+(k==='ssh'?'Verify a new server fingerprint using a trusted source. Profiles use linked protected credentials when configured.':'FTPS verifies TLS certificates. Use an optional pin for additional verification.')+'</p></div>'+
 '<div class="rb-notice" data-rb-feedback hidden></div></section>';}
function controls(k,side){const title=side==='local'?'This computer':'Remote '+(k==='ssh'?'SFTP':'FTP')+' server';return '<section class="rb-pane" data-rb-pane="'+side+'">'+(side==='local'?'<label class="rb-local-drives">Drive<select data-rb-drive aria-label="Local drive or mounted volume"><option value="">Loading drives…</option></select></label>':'')+'<div class="rb-pane-heading"><strong>'+title+'</strong><span data-rb-count="'+side+'">0 items</span></div><div class="rb-navigation"><button class="btn small ghost" title="Back" data-rb-nav="back" data-rb-side="'+side+'">'+icon('arrowLeft')+'</button><button class="btn small ghost" title="Forward" data-rb-nav="forward" data-rb-side="'+side+'">'+icon('arrowRight')+'</button><button class="btn small ghost" title="Parent folder" data-rb-nav="up" data-rb-side="'+side+'">'+icon('arrowUp')+'</button><button class="btn small ghost" title="Home directory" aria-label="Home directory" data-rb-nav="home" data-rb-side="'+side+'">'+icon('home')+'</button><button class="btn small ghost" title="Root directory" aria-label="Root directory" data-rb-nav="root" data-rb-side="'+side+'">/</button><button class="btn small ghost" title="Refresh" data-rb-nav="refresh" data-rb-side="'+side+'">'+icon('refresh')+'</button><input class="mono" data-rb-path="'+side+'" aria-label="'+title+' folder path"><button class="btn small ghost" data-rb-nav="go" data-rb-side="'+side+'">Go</button></div><nav class="transfer-breadcrumbs" data-rb-breadcrumbs="'+side+'" aria-label="'+title+' breadcrumbs"></nav>'+(side==='remote'?'<p class="transfer-drop-hint" data-rb-drop-destination>Connect before dropping files. Every drop requires upload confirmation.</p>':'')+'<div class="rb-filter"><input data-rb-filter="'+side+'" aria-label="Filter '+title+' files" placeholder="Filter names..."><small data-rb-selection="'+side+'"></small></div><div class="rb-scroll" data-rb-scroll="'+side+'" tabindex="0" aria-label="'+title+' file list"><table><thead><tr>'+['name','size','modified'].map(s=>'<th data-rb-sort="'+s+'" data-rb-side="'+side+'">'+{name:'Name',size:'Size',modified:'Modified'}[s]+'</th>').join('')+'</tr></thead><tbody data-rb-rows="'+side+'"></tbody></table></div><div class="rb-pane-footer">Right-click → Copy to Windows Explorer · Files and folders · F2 rename · Right-click for actions</div></section>'}
function explorerUI(k){return '<section class="panel rb-explorer"><div class="rb-toolbar"><button class="btn small primary" data-rb-action="upload">'+icon('arrowRight')+' Upload</button><button class="btn small ghost" data-rb-action="download">'+icon('arrowLeft')+' Download</button><span class="rb-divider"></span><button class="btn small ghost" data-rb-action="mkdir">'+icon('plus')+' New folder</button><button class="btn small ghost" data-rb-action="rename">'+icon('edit')+' Rename / move</button><button class="btn small ghost" data-rb-action="delete">'+icon('trash')+' Delete</button><button class="btn small ghost" data-rb-action="edit">'+icon('edit')+' Edit</button>'+'<button class="btn small ghost" data-rb-action="fullEditor" title="Open the Text Editor in Tools">'+icon('fileEdit')+' Text Editor</button><button class="btn small ghost" data-rb-action="preview">'+icon('file')+' Preview</button><button class="btn small ghost" data-rb-action="copyPath">'+icon('copy')+' Copy path</button><button type="button" class="btn small ghost" data-rb-action="copyLocation">Copy location</button><button class="btn small ghost" data-rb-action="copyFiles">Copy</button><button class="btn small ghost" data-rb-action="pasteFiles">Paste</button><button class="btn small ghost" data-rb-action="newFile">+ File</button><button class="btn small ghost" data-rb-action="uploadOS">Upload files</button><input data-rb-os-files type="file" multiple hidden><label class="rb-overwrite"><input type="checkbox" data-rb-overwrite> Overwrite</label></div>'+sessionTools(k)+'<div class="rb-panes" data-rb-panes style="--rb-left:'+get(k).split+'%">'+controls(k,'local')+'<div class="rb-splitter" data-rb-splitter role="separator" tabindex="0" aria-label="Resize browser columns"></div>'+controls(k,'remote')+'</div></section><section class="panel rb-activity"><div class="row between"><h2>Transfer queue and activity</h2><div class="row gap6"><button class="btn small ghost" data-rb-action="stopPending">Stop pending</button><button class="btn small ghost" data-rb-action="clearActivity">Clear</button></div></div><div class="rb-queue-status" data-rb-queue>Ready</div><div class="rb-activity-list" data-rb-activity></div></section>'}
function render(k){if(!TYPES.has(k))return;loadSaved(k);const mount=sidebarMount(k);if(!mount&&!pageActive(k))return;const host=mount?.host||document.getElementById('pageContent');if(!host)return;host.innerHTML='<div class="rb-app'+(mount?' is-editor-sidebar':'')+'" data-rb-root="'+k+'"'+(mount?' data-sidebar-side="'+mount.side+'"':'')+'>'+(mount?'<details class="nt-editor-files-connection"><summary class="btn small ghost">'+(k==='ssh'?'SFTP':'FTP / FTPS')+' connection</summary>'+connectionUI(k)+'</details>':connectionUI(k))+explorerUI(k)+'</div>';const m=get(k),app=rootNode(k);
 if(mount){if(detachedEditor())app.dataset.delegated='true';const feedback=app.querySelector('[data-rb-feedback]');app.prepend(feedback);m.active=mount.side;const toolbar=app.querySelector('.rb-toolbar'),details=document.createElement('details');details.className='nt-editor-files-actions';details.innerHTML='<summary class="btn small ghost">Actions</summary>';toolbar.before(details);details.append(toolbar);const quick=document.createElement('div');quick.className='nt-editor-files-quick';quick.innerHTML='<button type="button" class="btn small ghost" data-rb-action="edit">Open</button><button type="button" class="btn small ghost" data-rb-action="newFile">New file</button><button type="button" class="btn small ghost" data-rb-action="mkdir">New folder</button>';details.before(quick);const activity=app.querySelector('.rb-activity'),activityBox=document.createElement('details');activityBox.className='nt-editor-files-activity';activityBox.innerHTML='<summary>Activity</summary>';activity.before(activityBox);activityBox.append(activity);}

 if(!mount)root.NetworkToolkitTransferClientLayout?.mount(k,{app,settings:app.querySelector('.rb-connect'),library:app.querySelector('.rb-session-library'),libraryHTML:tab=>{m.libraryTab=tab;m.libraryOpen=true;return libraryHTML(k);}});
 for(const side of ['local','remote']){const pane=m[side];if(side==='remote'&&!m.connected){paint(k,side,m.connecting?'Connecting...':'Connect to a server to browse its files.');continue;}if(pane.loading)paint(k,side,'Loading...');else if(pane.initialized)paint(k,side);else void load(k,side,pane.path, true);}
 drawActivity(k);restoreConnectionControls(k);note(k,m.lastError,m.lastError?'error':'info');void ftpCredentialsRefresh(k);if(k==='ssh'){void refreshTerminalChoices(k);if(m.autoTerminal){const id=m.autoTerminal;m.autoTerminal='';void connectTerminal(k,id,{preserveFolder:true})}}}
function connectionIdentity(k){const m=get(k),c=configurations[k];return [m.connectionEpoch,c.host,c.port,c.username,c.security].join('|');}
function canNativeDrag(k,side){if(side==='local')return true;const c=configurations[k],m=get(k);return k==='ssh'?!!(m.connectionId||m.sessionId):!!c.credentialId&&!c.password&&c.security!=='plain';}
function dragIdentity(k){const m=get(k);return connectionIdentity(k)+'|'+(m.connectionId||m.sessionId||'');}
function acceptListing(k,side,data,skipHistory=false){const p=get(k)[side];
 if(side==='local')p.drives=localDriveChoices(data);
 p.path=data.path||'';if(!p.home)p.home=p.path;p.parent=data.parent||'';p.entries=(data.entries||[]).filter(x=>simpleName(x.name)).map(x=>({name:x.name,path:x.path,directory:!!x.directory,size:x.size,modified:x.modified}));p.selected.clear();p.anchor=-1;p.initialized=true;
 if(skipHistory&&p.index<0){p.history=[p.path];p.index=0}else if(!skipHistory&&p.history[p.index]!==p.path){p.history=p.history.slice(0,p.index+1).concat(p.path).slice(-80);p.index=p.history.length-1}
 p.loading=false;paint(k,side);remember(k);
} 
function sortedItems(pane){let rows=pane.entries.filter(item=>!pane.filter||item.name.toLowerCase().includes(pane.filter.toLowerCase()));return rows.sort((a,b)=>{if(a.directory!==b.directory)return a.directory?-1:1;let x;if(pane.sort==='size')x=Number(a.size||0)-Number(b.size||0);else if(pane.sort==='modified')x=String(a.modified||'').localeCompare(String(b.modified||''));else x=a.name.localeCompare(b.name,undefined,{sensitivity:'base',numeric:true});return pane.reverse?-x:x})}
function localDriveChoices(data){
 const entries=Array.isArray(data?.drives)?data.drives:(Array.isArray(data?.roots)?data.roots:[]).map(path=>({path,label:path,kind:'other'})),seen=new Set();
 return entries.slice(0,128).filter(row=>row&&typeof row.path==='string'&&row.path.length>0&&row.path.length<=4096&&!/[\x00-\x1f]/.test(row.path)).filter(row=>{const key=row.path.replace(/\\/g,'/').toLowerCase();if(seen.has(key))return false;seen.add(key);return true;}).map(row=>({path:row.path,label:typeof row.label==='string'?row.label.slice(0,256):row.path,kind:['local','network','removable','optical','memory','other'].includes(row.kind)?row.kind:'other'}));
}
function paintDrives(area,pane){const select=area.querySelector('[data-rb-drive]');if(!select)return;const drives=pane.drives||[],path=String(pane.path||'').replace(/\\/g,'/').toLowerCase(),selected=drives.filter(d=>path.startsWith(d.path.replace(/\\/g,'/').toLowerCase().replace(/\/$/,'')+'/')||path===d.path.replace(/\\/g,'/').toLowerCase().replace(/\/$/,'')).sort((a,b)=>b.path.length-a.path.length)[0]?.path||'';select.innerHTML='<option value="">Choose a drive / mounted volume</option>'+drives.map(d=>'<option value="'+escape(d.path)+'">'+escape(d.label||d.path)+(d.kind==='network'?' · Network':d.kind==='removable'?' · Removable':'')+'</option>').join('');select.value=selected;}
function paint(k,side,message=''){const m=get(k),pane=m[side],rootEl=rootNode(k);if(!rootEl)return;const body=rootEl.querySelector('[data-rb-rows="'+side+'"]');if(!body)return;
 if(side==='local')paintDrives(rootEl,pane);
 const entries=sortedItems(pane);body.innerHTML=message?'<tr><td colspan="3" class="rb-empty">'+escape(message)+'</td></tr>':entries.length?entries.map(item=>'<tr data-rb-item="'+escape(item.path)+'" data-rb-side="'+side+'" data-rb-dir="'+String(!!item.directory)+'" draggable="true" class="'+(pane.selected.has(item.path)?'selected':'')+'"><td><div class="rb-file-name">'+icon(item.directory?'folder':'file')+'<span title="'+escape(item.name)+'">'+escape(item.name)+'</span></div></td><td>'+ (item.directory?'Folder':bytes(item.size))+'</td><td title="'+escape(item.modified||'')+'">'+escape(when(item.modified))+'</td></tr>').join(''):'<tr><td colspan="3" class="rb-empty">No files in this folder.</td></tr>';
 const filter=rootEl.querySelector('[data-rb-filter="'+side+'"]');if(filter&&document.activeElement!==filter)filter.value=pane.filter;
 const address=rootEl.querySelector('[data-rb-path="'+side+'"]');if(address&&document.activeElement!==address)address.value=pane.path;
 const crumbs=rootEl.querySelector('[data-rb-breadcrumbs="'+side+'"]');if(crumbs){const text=String(pane.path||'').replace(/\\/g,'/'),parts=text.split('/').filter(Boolean);let base=text.startsWith('/')?'/':'';const rows=text.startsWith('/')?[[base,'/']]:[];for(const part of parts){base+=(base&&!base.endsWith('/')?'/':'')+part;if(/^[A-Za-z]:$/.test(base))base+='/';rows.push([base,part]);}crumbs.innerHTML=rows.map(([path,name])=>'<button type="button" class="btn small ghost" data-rb-breadcrumb="'+escape(path)+'" data-rb-side="'+side+'">'+escape(name)+'</button>').join('<span aria-hidden="true">/</span>');}
 const hint=rootEl.querySelector('[data-rb-drop-destination]');if(hint){const c=(m.sessionId?m.sessionOptions.find(row=>row.id===m.sessionId):m.connectionId?m.connectionOptions.find(row=>row.id===m.connectionId):null)||configurations[k];hint.textContent=k==='ftp'?'Windows Explorer uploads are unavailable for FTP/FTPS; use SFTP or Upload selection.':m.connected?'Upload files to remote '+c.host+':'+c.port+' — '+m.remote.path+' (confirmation required; files and folders)':'Connect before dropping files. Every drop requires upload confirmation.';}
 const n=rootEl.querySelector('[data-rb-count="'+side+'"]');if(n)n.textContent=pane.entries.length+' items';
 const s=rootEl.querySelector('[data-rb-selection="'+side+'"]');if(s)s.textContent=pane.selected.size?pane.selected.size+' selected':'';updateSidebarActions(k);
}
async function load(k,side,target='',skipHistory=false){const m=get(k),p=m[side];if(side==='remote'&&!m.connected){paint(k,side,'Connect first.');return;}
 const path=side==='remote'?(target||'.'):target,identity=connectionIdentity(k);
 if(p.loading&&p.pendingPath===path&&p.pendingIdentity===identity)return p.pending;
 const requestId=++p.requestId;p.pendingPath=path;p.pendingIdentity=identity;p.loading=true;root.NetworkToolkitNativeDrag?.cancelHover?.();paint(k,side,'Loading...');
 p.pending=(async()=>{try{const data=side==='local'?await api('/transfer/local'+(path?'?path='+encodeURIComponent(path):'')):await request(k,'list',{path});
 if(requestId!==p.requestId||side==='remote'&&(!m.connected||identity!==connectionIdentity(k)))return;
 acceptListing(k,side,data,skipHistory);
 }catch(e){if(requestId!==p.requestId||side==='remote'&&identity!==connectionIdentity(k))return;p.loading=false;paint(k,side,e.message);note(k,'Cannot open '+side+' folder: '+e.message,'error');}
 finally{if(requestId===p.requestId){p.loading=false;p.pending=null;}}})();return p.pending;
}
function paintSelection(k,side){
 const pane=get(k)[side],area=rootNode(k);
 if(!area)return;
 area.querySelectorAll('[data-rb-item]').forEach(row=>{
  if(row.dataset.rbSide===side)row.classList.toggle('selected',pane.selected.has(row.dataset.rbItem));
 });
 const counter=area.querySelector('[data-rb-selection="'+side+'"]');
 if(counter)counter.textContent=pane.selected.size?pane.selected.size+' selected':'';
 updateSidebarActions(k);
}
function rowSelect(k,side,path,event){const m=get(k),pane=m[side],entries=sortedItems(pane),at=entries.findIndex(x=>x.path===path);if(at<0)return;m.active=side;pane.cursor=path;
 if(event.shiftKey&&pane.anchor>=0){if(!event.ctrlKey)pane.selected.clear();for(let i=Math.min(at,pane.anchor);i<=Math.max(at,pane.anchor);i++)pane.selected.add(entries[i].path)}else if(event.ctrlKey||event.metaKey){if(pane.selected.has(path))pane.selected.delete(path);else pane.selected.add(path);pane.anchor=at}else{pane.selected=new Set([path]);pane.anchor=at}paintSelection(k,side)}
function updateSidebarActions(k){const area=rootNode(k);if(!area?.classList?.contains('is-editor-sidebar'))return;const m=get(k),items=chosen(k,m.active),remote=m.active==='remote',unavailable=remote&&!m.connected;for(const button of area.querySelectorAll('[data-rb-action]')){let disabled=false;const action=button.dataset.rbAction;if(['edit','rename','preview'].includes(action))disabled=items.length!==1||(action==='preview'&&items[0]?.directory)||unavailable;else if(['delete','copyFiles','cutFiles'].includes(action))disabled=!items.length||unavailable;else if(['newFile','mkdir'].includes(action))disabled=unavailable;else if(action==='upload')disabled=!m.connected||!m.local.selected.size;else if(action==='download')disabled=!m.connected||!m.remote.selected.size||!m.local.path;else if(action==='uploadOS')disabled=!m.connected;else if(action==='pasteFiles')disabled=!clipboard?.paths?.length||unavailable;else continue;button.disabled=m.busy||disabled;}}
function chosen(k,side){const p=get(k)[side];return p.entries.filter(x=>p.selected.has(x.path))}
function actionLog(k,text,kind='info'){const m=get(k);m.activity.unshift({time:new Date().toLocaleTimeString(),text,kind});m.activity.length=Math.min(m.activity.length,120);drawActivity(k)}
function drawActivity(k){updateSidebarActions(k);const el=rootNode(k);if(!el)return;const log=el.querySelector('[data-rb-activity]');if(log)log.innerHTML=get(k).activity.map(x=>'<div class="rb-log '+x.kind+'"><time>'+escape(x.time)+'</time> '+escape(x.text)+'</div>').join('')||'<p class="muted">No file operations yet.</p>';const status=el.querySelector('[data-rb-queue]');if(status)status.textContent=get(k).busy?'Transfer in progress. Stop pending will finish the active file first.':'Ready'}
function showKeyMismatch(k,error){
 if(k!=='ssh')return false;
 const text=String(error?.message||error),match=text.match(/SSH host key mismatch:\s*expected\s+(SHA256:[A-Za-z0-9+/=]+),\s*presented\s+(SHA256:[A-Za-z0-9+/=]+)/i);
 if(!match)return false;
 const c=configurations.ssh,m=get('ssh'),oldPin=match[1],newPin=match[2];
 const id=m.profileId,host=c.host,port=c.port;
 m.pendingFingerprint={profileId:id,observed:newPin,previous:oldPin,host,port};
 openModal('SSH host key changed',
  '<p class="rb-notice error">The server presented a different SSH host key. This can happen after a device reinstall, but it can also indicate interception. The connection remains blocked until you independently verify the new fingerprint.</p>'+
  '<form id="rbSSHKeyChangeForm" autocomplete="off"><p>Host: <strong class="mono">'+escape(host+':'+port)+'</strong></p><p>Previously trusted:</p><div class="rb-fingerprint mono">'+escape(oldPin)+'</div><p>Presented now:</p><div class="rb-fingerprint mono">'+escape(newPin)+'</div>'+
  '<label class="field mt">Fingerprint verified through a trusted separate source<input id="rbSSHChangedFingerprint" type="text" autocomplete="off" spellcheck="false" required placeholder="SHA256:..."></label><p id="rbSSHKeyChangeError" class="rb-notice error" hidden></p><p class="help-line">Do not accept a changed key based only on the value displayed by this connection attempt.</p></form>',
  '<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button type="submit" class="btn primary" form="rbSSHKeyChangeForm">Verify and reconnect</button>',true);
 return true;
}
async function connect(k){const m=get(k);if(m.connecting)return;const c=configurations[k];if(!c.host.trim()||!c.username.trim()){note(k,'Enter the server host and username.','error');return;}if(k==='ssh'&&!c.fingerprint.trim()){note(k,'Inspect and independently verify the SSH host key, then enter its fingerprint.','error');return;}
 try{await closeOwnedConnection(k);}catch(error){note(k,'Cannot close the previous connection: '+error.message,'error');return;}
 const epoch=m.connectionEpoch,identity=connectionIdentity(k),submittedSecrets={password:c.password,privateKey:c.privateKey,passphrase:c.passphrase};m.connecting=true;m.remote.initialized=false;render(k);
 try{const response=await request(k,k==='ssh'?'connect':'list',{path:m.remote.path||'.'});
  if(m.connectionEpoch!==epoch||identity!==connectionIdentity(k)){if(response.connectionId)void api('/remote-browser','POST',{protocol:'ssh',action:'disconnect',connectionId:response.connectionId}).catch(()=>{});return;}
  if(k==='ssh'){if(typeof response.connectionId!=='string'||!response.connectionId)throw new Error('The server did not return an authenticated SFTP connection.');m.connectionId=response.connectionId;for(const field of ['password','privateKey','passphrase'])if(c[field]===submittedSecrets[field])c[field]='';}
  m.connected=true;acceptListing(k,'remote',response,true);if(k==='ssh'){rememberSFTPRecent();if(m.profileId)m.showSettings=false;}else{rememberFTPRecent();if(m.profileId)m.showSettings=false;}
  actionLog(k,'Connected '+k.toUpperCase()+' file browser to '+c.host,'success');render(k);note(k,k==='ssh'?'SSH session connected. File operations reuse this authenticated connection.':'Server verified. File browsing is available.','success');
 }catch(error){if(m.connectionEpoch!==epoch)return;if(!showKeyMismatch(k,error))note(k,'Connect failed: '+error.message,'error');actionLog(k,'Connection failed: '+error.message,'error');}
 finally{if(m.connectionEpoch===epoch)m.connecting=false;restoreConnectionControls(k);}
}
async function disconnect(k){const m=get(k),borrowed=!!m.sessionId;try{await closeOwnedConnection(k);render(k);actionLog(k,borrowed?'Disconnected browser from the terminal session. SSH terminal remains connected.':'Disconnected from the file server.','info');note(k,borrowed?'Browser detached. The SSH terminal is still connected.':'Disconnected. Reconnect uses the current connection settings.','info');}catch(error){render(k);note(k,'Disconnect failed: '+error.message,'error');}}
async function reconnect(k){const m=get(k);if(m.connecting||m.busy)return;if(m.sessionId){const id=m.sessionId;await connectTerminal(k,id);return;}if(k==='ssh'&&m.profileId){await closeOwnedConnection(k);await useSavedSSHProfile(m.profileId);return;}if(k==='ssh'){const c=configurations[k];if(!c.credentialId&&!(c.authMode==='key'?(c.privateKey||c.keyPath):c.password)){m.showSettings=true;render(k);rootNode(k)?.clientSidebarSelect?.('settings');note(k,'Enter your SSH credentials again in Connection settings, then choose Connect / Browse. The current session is unchanged.','info');return;}}await connect(k);}
async function openTerminal(k){if(k!=='ssh')return;const m=get(k);if(!m.connected||m.connecting||m.busy)return;let id=m.sessionId;
 if(!id){const owned=m.connectionId,epoch=m.connectionEpoch;m.connecting=true;restoreConnectionControls(k);try{const response=await request(k,'open-terminal');if(!response.session?.id)throw new Error('The server did not return a terminal session.');
  if(epoch!==m.connectionEpoch)throw new Error('The browser connection changed while opening Terminal.');
  if(response.viewState&&typeof receiveTerminalViewState==='function')receiveTerminalViewState(response.viewState);
  id=response.session.id;const entry=typeof upsertTerminalSession==='function'?upsertTerminalSession(response.session):null;
  if(entry){const c=configurations[k],config={type:'ssh',host:c.host,port:c.port,username:c.username,authMethod:c.authMode,credentialId:c.credentialId||'',sshCompatibility:c.sshCompatibility,timeoutMs:Number(sshRecord(m.profileId)?.timeoutMs)||10000};entry.config=typeof safeTerminalConfig==='function'?safeTerminalConfig(config):config;root.NetworkToolkitSessionSidebar?.record?.(entry,c.profileName||c.host);}
  m.connectionId='';m.sessionId=id;m.sourceChoice='session:'+id;m.sessionOptions=[{id,host:configurations[k].host,port:configurations[k].port,username:configurations[k].username,label:configurations[k].profileName||configurations[k].host},...m.sessionOptions.filter(x=>x.id!==id)];actionLog(k,'Opened Terminal over the existing authenticated SSH connection.','success');
 }catch(error){note(k,'Cannot open Terminal: '+error.message,'error');return;}finally{m.connecting=false;restoreConnectionControls(k);}}
 if(typeof navigate==='function')navigate('terminal');if(typeof selectTerminal==='function')selectTerminal(id);if(typeof startTerminalFeed==='function')startTerminalFeed();if(typeof scheduleTerminalFit==='function')scheduleTerminalFit();
}

async function inspect(k){try{const r=await request(k,'fingerprint');const fp=String(r.fingerprint||'');openModal(k==='ssh'?'Inspect SSH server fingerprint':'Inspect FTPS certificate', '<p>Verify this fingerprint with the server administrator or another trusted source before trusting the connection.</p><div class="rb-fingerprint mono">'+escape(fp)+'</div><p class="help-line">Fingerprint inspection does not authenticate the server. Use the verified value in the connection settings.</p>','<button class="btn ghost" data-action="close-modal">Close</button>',true);actionLog(k,'Inspected remote '+(k==='ssh'?'SSH key':'FTPS certificate')+' fingerprint','info')}catch(e){note(k,'Fingerprint inspection failed: '+e.message,'error')}}
function nav(k,side,how){if(get(k).busy){note(k,'Finish or cancel the current operation before changing folders.','info');return;}const p=get(k)[side];if(how==='home'){void load(k,side,p.home||'');return;}if(how==='root'){const current=p.path||'';const location=side==='remote'?'/':(/^[A-Za-z]:[\\/]/.test(current)?current.slice(0,3):'/');void load(k,side,location);return;}if(how==='go'){const path=rootNode(k)?.querySelector('[data-rb-path="'+side+'"]')?.value;void load(k,side,path||'');return}if(how==='refresh'){void load(k,side,p.path,true);return}if(how==='up'){void load(k,side,p.parent||dirname(p.path,side));return}if(how==='back'&&p.index>0){p.index--;void load(k,side,p.history[p.index],true);return}if(how==='forward'&&p.index<p.history.length-1){p.index++;void load(k,side,p.history[p.index],true)}}
function dialog(k,kind){const m=get(k),side=m.active,entries=chosen(k,side),first=entries[0];let html='',text='Run';
 if(kind==='mkdir'){text='Create folder';html='<label>New folder name<input id="rbDialogName" placeholder="Folder name"></label>'}
 if(kind==='rename'){if(entries.length!==1){toast('Select one file or folder to rename.','info');return}text='Rename / move';html='<label>New path<input id="rbDialogName" class="mono" value="'+escape(first.path)+'"></label>'}
 if(kind==='delete'){if(!entries.length){toast('Select files or folders to delete.','info');return}text='Delete';html='<p>Delete '+entries.length+' selected item(s)? Remote and local deletion is permanent. Selected folders and their contents will also be deleted.</p><label><input type="checkbox" id="rbDialogConfirm"> I confirm permanent deletion</label>'}
 if(kind==='upload'||kind==='download'){const src=kind==='upload'?'local':'remote';if(!chosen(k,src).length){toast('Select one or more '+(src==='local'?'computer':'remote')+' files.','info');return}text=kind==='upload'?'Upload':'Download';html='<p>Copy '+chosen(k,src).length+' item(s), including selected folders and their contents?</p><p class="help-line">Target folder: '+escape(get(k)[kind==='upload'?'remote':'local'].path)+'</p>'}
 if(!html)return;openModal(text,html,'<button class="btn ghost" data-action="close-modal">Cancel</button><button class="btn '+(kind==='delete'?'danger':'primary')+'" data-rb-confirm="'+kind+'" data-rb-kind="'+k+'">'+text+'</button>',true)
}
function safeName(v){if(!simpleName(v))throw Error('Unsafe file name returned by server: '+v);return v}
async function copyRecursive(k,direction,entry,targetPath,overwrite,tally){
 const m=get(k);if(m.cancelPending)throw new Error('Pending work stopped');if(++tally.count>1500)throw new Error('Transfer queue is limited to 1,500 files and directories');
 const source=direction==='upload'?'local':'remote',destination=direction==='upload'?'remote':'local';
 if(entry.directory){
  // Create the target directory; existing folders are reused.
  try{if(destination==='remote')await request(k,'mkdir',{path:targetPath});else await api('/transfer/local-files','POST',{action:'mkdir',path:targetPath})}catch(e){if(!/exist|already/i.test(String(e.message)))throw e}
  const data=source==='local'?await api('/transfer/local?path='+encodeURIComponent(entry.path)):await request(k,'list',{path:entry.path});for(const item of data.entries||[]){safeName(item.name);await copyRecursive(k,direction,item,destination==='remote'?joinRemote(targetPath,item.name):joinLocal(targetPath,item.name),overwrite,tally)}
 } else {
  const requestOptions=direction==='upload'?{path:targetPath,localPath:entry.path,overwrite}:{path:entry.path,localPath:targetPath,overwrite};
  await request(k,direction,requestOptions);
  tally.files++;
  actionLog(k,(direction==='upload'?'Uploaded ':'Downloaded ')+entry.name+' ('+bytes(entry.size)+')','success')
 }
}
async function removeTree(k,side,entry,tally){if(++tally.count>1500)throw Error('Delete limit of 1,500 items reached');if(!entry.directory){if(side==='local')await api('/transfer/local-files','POST',{action:'delete',path:entry.path,confirm:true});else await request(k,'remove',{path:entry.path,confirm:true});return;}
 if(entry.path==='/'||entry.path==='.'||/^[A-Z]:\\?$/i.test(entry.path))throw Error('Cannot delete a filesystem root');
 const children=side==='local'?await api('/transfer/local?path='+encodeURIComponent(entry.path)):await request(k,'list',{path:entry.path});
 for(const child of children.entries||[]){safeName(child.name);await removeTree(k,side,child,tally);}
 if(side==='local')await api('/transfer/local-files','POST',{action:'delete',path:entry.path,confirm:true});else await request(k,'rmdir',{path:entry.path,confirm:true});
}
function protectOpenFiles(k,side,paths){const m=get(k),normalize=value=>{const s=String(value).replace(/\\/g,'/').replace(/\/$/,'');return side==='local'?s.toLowerCase():s;};for(const d of root.NetworkToolkitFileEditor?.documents?.values?.()||[]){const b=d.binding,matching=side==='local'?['local','localDraft'].includes(b?.kind):b?.kind==='terminal'?b.sessionId===m.sessionId:b?.kind==='sshConnection'&&b.connectionId===m.connectionId;if(!matching)continue;const file=normalize(d.path);if(paths.some(p=>file===normalize(p)||file.startsWith(normalize(p)+'/')))throw Error('Close '+nameOfEditorPath(d.path)+' before changing its file or folder. Unsaved edits remain open.');}}
function nameOfEditorPath(path){return String(path).split(/[\\/]/).pop();}
async function perform(k,action){const m=get(k);if(m.busy)return;let entries=[],side=m.active;const dialogName=String(document.querySelector('#rbDialogName')?.value||'').trim();const confirmed=!!document.querySelector('#rbDialogConfirm')?.checked;const overwrite=!!rootNode(k)?.querySelector('[data-rb-overwrite]')?.checked;
 if(action==='upload'||action==='download'){side=action==='upload'?'local':'remote';entries=chosen(k,side);if(!entries.length)return}
 if(action==='delete'){entries=chosen(k,side);if(!confirmed){toast('Confirm permanent deletion first.','error');return}}
 if(action==='rename'){entries=chosen(k,side);if(entries.length!==1||!dialogName)return}
 if(action==='mkdir'&&!(root.NetworkToolkitTransferDrop?.validName(dialogName)??simpleName(dialogName))){toast('Enter one valid folder name.','error');return}
 closeModal();m.busy=true;m.cancelPending=false;drawActivity(k);const tally={files:0,count:0};let completed=0,failed=0;
 try{
  if(['delete','rename'].includes(action))protectOpenFiles(k,side,entries.map(item=>item.path));
  if(action==='mkdir'){const path=side==='local'?joinLocal(m.local.path,dialogName):joinRemote(m.remote.path,dialogName);if(side==='local')await api('/transfer/local-files','POST',{action:'mkdir',path});else await request(k,'mkdir',{path});actionLog(k,'Created '+path,'success')}
  else if(action==='rename'){const old=entries[0].path;if(side==='local')await api('/transfer/local-files','POST',{action:'rename',path:old,newPath:dialogName});else await request(k,'rename',{path:old,newPath:dialogName});actionLog(k,'Renamed '+old+' to '+dialogName,'success')}
  else if(action==='delete'){for(const item of entries){if(m.cancelPending)break;try{await removeTree(k,side,item,{count:0});completed++;actionLog(k,'Deleted '+item.path,'success')}catch(e){failed++;actionLog(k,'Delete '+item.path+': '+e.message,'error')}}}
  else if(action==='upload'||action==='download'){const dest=action==='upload'?'remote':'local';for(const entry of entries){if(m.cancelPending)break;try{safeName(entry.name);const path=dest==='remote'?joinRemote(m.remote.path,entry.name):joinLocal(m.local.path,entry.name);await copyRecursive(k,action,entry,path,overwrite,tally);completed++}catch(e){failed++;actionLog(k,entry.name+': '+e.message,'error')}}actionLog(k,action+' finished: '+completed+' items, '+tally.files+' files, '+failed+' errors',failed?'error':'success')}
 }catch(e){failed++;actionLog(k,action+' failed: '+e.message,'error');note(k,e.message,'error')}
 finally{m.busy=false;drawActivity(k);await Promise.all([load(k,'local',m.local.path,true),m.connected?load(k,'remote',m.remote.path,true):Promise.resolve()]);if(!failed)toast('File operation completed','success');else toast(failed+' operation(s) failed. Review activity.','error')}
}
async function preview(k){const m=get(k),items=chosen(k,m.active);if(items.length!==1||items[0].directory){toast('Select exactly one text file.','info');return}try{const data=m.active==='remote'?await request(k,'preview',{path:items[0].path}):await api('/transfer/local?preview='+encodeURIComponent(items[0].path));if(data.binary){toast('Binary data cannot be previewed. Download or open in a suitable editor.','info');return}
 openModal('Preview '+items[0].name,'<pre class="transfer-file-preview">'+escape(data.content||'')+'</pre>','<button class="btn primary" data-action="close-modal">Close</button>',true)}catch(e){note(k,e.message,'error')}}
async function edit(k,newFile=false,side=get(k).active,pathOverride=''){
 const m=get(k),items=chosen(k,side);let file=items[0],path=pathOverride||file?.path;
 if(side==='remote'&&!m.connected){note(k,'Connect to the remote server before editing a file.','error');return;}
 if(!newFile&&!pathOverride&&(items.length!==1||file.directory)){toast('Select one text file to edit.','info');return;}
 if(newFile&&!pathOverride){const name=await root.NetworkToolkitFileEditor.requestName('New text file','File name','new-file.txt');if(name===null)return;if(!simpleName(name)){toast('Enter a valid file name.','error');return;}path=side==='remote'?joinRemote(m.remote.path,name):joinLocal(m.local.path,name);}
 if(typeof path!=='string'||!path.trim()||path.length>4096)throw Error('Enter a valid file path.');
 const r=path,identity=connectionIdentity(k),binding=side==='local'?{kind:'local'}:k==='ssh'&&m.connectionId?{kind:'sshConnection',connectionId:m.connectionId}:k==='ssh'&&m.sessionId&&typeof terminalSessionRevision==='function'?{kind:'terminal',sessionId:m.sessionId,revision:revisionFor(m.sessionId)}:null;
 const check=()=>{const now=get(k),cfg=configurations[k];if(!now.connected||connectionIdentity(k)!==identity)throw Error('Connection changed or disconnected. Reopen this file to prevent saving to another server.');};
 try{await root.NetworkToolkitFileEditor.open({source:(side==='local'?'Local ':k.toUpperCase()+' ')+ (side==='remote'&&m.sessionId?'terminal':'files'),path:r,id:k+':'+side+':'+connectionIdentity(k)+':'+r,create:newFile,binding,
  read:async path=>{if(side==='local')return api('/file-editor/local','POST',{action:'read-text',path});check();return request(k,'read-text',{path});},
  write:async data=>{if(side==='local')return api('/file-editor/local','POST',{action:'write-text',...data});check();return request(k,'write-text',data);},
  afterSave:()=>void load(k,side,get(k)[side].path,true)});
 }catch(e){note(k,'Cannot open editor: '+e.message,'error');if(pathOverride)throw e;}
}
async function dropFiles(k,files,folder,side='remote'){if(side==='remote'&&k==='ftp'){note(k,'Explorer uploads to FTP/FTPS cannot guarantee safe atomic collision handling. Use SFTP or the existing explicit FTP transfer controls.','error');return;}const m=get(k),helper=root.NetworkToolkitTransferDrop;if(side==='remote'&&!m.connected){note(k,'Connect to a remote server before uploading files.','error');return;}if(m.busy){note(k,'Finish or cancel the current operation first.','error');return;}
 const identity=connectionIdentity(k),connection=dragIdentity(k),originalFolder=m[side].path;
 const check=()=>{if(side==='remote'&&(!m.connected||identity!==connectionIdentity(k)||connection!==dragIdentity(k))||m[side].path!==originalFolder||!pageActive(k))throw Error('Connection or destination changed. Drop the selection again; nothing else will be sent.');};
 const join=side==='local'?joinLocal:joinRemote;
 helper.show({files,entries:m[side].entries,remote:side==='remote',local:side==='local',label:(side==='local'?'Local':'SFTP')+' — '+folder,check,busy:value=>{m.busy=value;drawActivity(k);},send:async(row,onProgress,signal)=>{
  let importID="";
  const sendFile=async(file,directory,name,overwrite)=>{check();if(signal.aborted)throw Object.assign(Error('Cancelled before sending; completed files remain in the new folder.'),{name:'AbortError'});const form=new FormData(),headers={};
   if(side==='local'){form.set('destination',JSON.stringify({path:directory,name,overwrite,...(importID?{id:importID}:{})}));form.set('file',file,file.name);return helper.upload('/api/transfer/local-drop',form,{signal,onProgress});}
   const details=await requestPayload(k,'upload',{path:directory,overwrite});check();form.set('connection',JSON.stringify(details));form.set('name',name);form.set('file',file,file.name);if(details.sessionId)headers['X-Terminal-Revision']=String(revisionFor(details.sessionId));return helper.upload('/api/remote-browser/drop-upload',form,{signal,onProgress,headers});};
  if(!row.file.directory)return sendFile(row.file,folder,row.name,row.choice==='overwrite');
  if(side==='local')importID=(await api('/transfer/local-drop','POST',{action:'begin',path:folder})).id;
  const top=side==='local'?row.name:join(folder,row.name);try{for(const entry of row.file.items){check();if(signal.aborted)throw Object.assign(Error('Cancelled; completed items remain in the new folder.'),{name:'AbortError'});const relative=entry.path.split('/').filter(Boolean),target=relative.reduce((base,part)=>join(base,part),top);
   if(entry.directory){if(side==='local')await api('/transfer/local-drop','POST',{action:'mkdir',id:importID,path:target.replaceAll('\\','/')});else await request(k,'mkdir',{path:target});}
   else {const name=relative.pop();const parent=relative.reduce((base,part)=>join(base,part),top);await sendFile(entry.file,side==='local'?parent.replaceAll('\\','/'):parent,name,false);}
  }}finally{if(importID)await api('/transfer/local-drop','POST',{action:'end',id:importID}).catch(()=>{});}
 },done:async report=>{for(const row of report.rows)actionLog(k,row.name+': '+row.status,row.status==='Copied'?'success':'error');if(pageActive(k)&&(side==='local'||m.connected&&identity===connectionIdentity(k)))await load(k,side,m[side].path,true);note(k,report.copied+' copied, '+report.failed+' failed, '+report.cancelled+' cancelled, '+report.skipped+' skipped. Original sources stay unchanged; partial new folders may contain completed files.',report.failed||report.cancelled?'error':'info');}});
}
async function prepareExplorerCopy(k,side=get(k).active){const m=get(k),files=chosen(k,side);if(!files.length||m.busy)return;if(side==='remote'&&(k!=='ssh'||!m.connected)){note(k,'Connect an SFTP session before preparing a Windows Explorer copy.','error');return;}
 const binding=side==='local'?{kind:'local'}:m.sessionId?{kind:'terminal',sessionId:m.sessionId,revision:revisionFor(m.sessionId)}:{kind:'sshConnection',connectionId:m.connectionId};
 await root.NetworkToolkitNativeDrag.prepareCopy({binding,paths:files.map(file=>file.path)});
}
async function dropTransfer(k,from,target,external=null){const m=get(k);if(m.busy)return;
 if(external){const paths=external.files||[];if(external.sessionId){
  if(target!=='local')return;const dest=m.local.path;
  const externalRequest=(action,extra)=>api('/remote-browser','POST',{protocol:'ssh',sessionId:external.sessionId,action,...extra});
  async function fetchTree(file,destination,limit){if(++limit.count>1500)throw Error('Folder transfer is limited to 1,500 items');
   if(!file.directory){await externalRequest('download',{path:file.path,localPath:destination,overwrite:false});return;}
   await api('/transfer/local-files','POST',{action:'mkdir',path:destination});
   const listing=await externalRequest('list',{path:file.path});
   for(const child of listing.entries||[]){if(!simpleName(child.name))throw Error('Unsafe remote filename');await fetchTree(child,joinLocal(destination,child.name),limit);}
  }
  let errors=0;for(const file of paths){const name=file.path?.split('/').pop();if(!simpleName(name)){errors++;continue;}
   try{await fetchTree(file,joinLocal(dest,name),{count:0});actionLog(k,'Downloaded '+name+' from terminal','success');}catch(e){errors++;actionLog(k,name+': '+e.message,'error');}}
  if(errors)note(k,errors+' download(s) failed','error');void load(k,'local',m.local.path,true);return;
 }}
 if(from===target)return;
 const items=external?.files||chosen(k,from);if(!items.length)return;
 const destination=target==='remote'?'remote':'local',direction=destination==='remote'?'upload':'download',overwrite=!!rootNode(k)?.querySelector('[data-rb-overwrite]')?.checked;
 m.busy=true;m.cancelPending=false;drawActivity(k);let count=0,failed=0;
 try{for(const entry of items){try{const path=destination==='remote'?joinRemote(m.remote.path,entry.name):joinLocal(m.local.path,entry.name);await copyRecursive(k,direction,entry,path,overwrite,{files:0,count:0});count++;}catch(e){failed++;actionLog(k,entry.name+': '+e.message,'error');}}}
 finally{m.busy=false;drawActivity(k);await Promise.all([load(k,'local',m.local.path,true),m.connected?load(k,'remote',m.remote.path,true):Promise.resolve()]);note(k,failed?'Copied '+count+', failed '+failed+'. Review activity.':'Copied '+count+' item(s).',failed?'error':'success');}
}
async function pasteFiles(k){if(!clipboard)return;const m=get(k),destSide=m.active;const src=clipboard;if(m.busy)return;if(src.side==='remote'&&src.identity!==dragIdentity(k)){note(k,'The copied files belong to a different SFTP connection. Copy them again from the current connection.','error');return;}
 if(src.k!==k){toast('Copy between separate servers is not available. Open the target server and upload or download the files.','info');return;}
 if(src.side!==destSide){if(src.move){note(k,'Move stays within the same file location. Use Copy for a local/SFTP transfer.','info');return;}void dropTransfer(k,src.side,destSide,{files:src.paths});return;}
 const destination=m[destSide].path,identity=connectionIdentity(k);m.busy=true;m.cancelPending=false;drawActivity(k);let done=0,errors=0;
 const current=()=>{if(m.cancelPending)throw Error('Pending paste cancelled');if(destSide==='remote'&&identity!==connectionIdentity(k))throw Error('SFTP connection changed; remaining paste stopped');};
 async function remoteCopyTree(entry,target,limit){current();if(++limit.count>1500)throw Error('Copy is limited to 1,500 items');if(!entry.directory)return request(k,'copy',{path:entry.path,newPath:target});
  await request(k,'mkdir',{path:target});const contents=await request(k,'list',{path:entry.path});for(const item of contents.entries||[]){safeName(item.name);await remoteCopyTree(item,joinRemote(target,item.name),limit);}
 }
 try{for(const entry of src.paths){if(m.cancelPending||destSide==='remote'&&identity!==connectionIdentity(k)){errors+=src.paths.length-done-errors;break;}const target=destSide==='remote'?joinRemote(destination,entry.name):joinLocal(destination,entry.name);
  if(entry.path===target){errors++;continue;}
  try{if(src.move){if(destSide==='remote')await request(k,'rename',{path:entry.path,newPath:target});else await api('/transfer/local-files','POST',{action:'rename',path:entry.path,newPath:target});}
   else if(destSide==='remote')await remoteCopyTree(entry,target,{count:0});else await api('/transfer/local-files','POST',{action:'copy',path:entry.path,newPath:target,overwrite:false});done++;}
  catch(e){errors++;actionLog(k,'Paste '+entry.name+': '+e.message,'error');}
 }
 if(src.move&&!errors)clipboard=null;
 }finally{m.busy=false;drawActivity(k);if(destSide==='local'||identity===connectionIdentity(k))await load(k,destSide,m[destSide].path,true);}
 note(k,'Pasted '+done+' item(s)'+(errors?', '+errors+' failed or stopped. Review Activity.':''),errors?'error':'success');
}
function locationSnapshot(k,side,path=null){
 const m=get(k),c=configurations[k];if(side==='local')return {scope:'local',localPath:path??m.local.path};
 if(path!==null&&!m.connected)throw new Error('Connect before copying a remote file location. Copy server address is available for the configured destination.');
 const session=m.sessionId?m.sessionOptions.find(row=>row.id===m.sessionId):null;if(m.sessionId&&!session)throw new Error('The active terminal endpoint is unavailable. Refresh its session information before copying.');
 return {scope:'remote',protocol:k==='ssh'?'sftp':'ftp',security:c.security,host:session?.host||c.host,port:session?.port||c.port,path};
}
function copyLocation(k,side=get(k).active,path){try{const pane=get(k)[side],item=chosen(k,side)[0];root.NetworkToolkitTransferLocations.show(locationSnapshot(k,side,path===undefined?(item?.path??pane.path):path));}catch(error){note(k,error.message,'error');}}
function menu(k,side,event){const m=get(k);m.active=side;const file=chosen(k,side)[0],label=(name,action)=>'<button type="button" data-rb-menu="'+action+'">'+name+'</button>';
 if(contextMenu)contextMenu.remove();contextMenu=document.createElement('div');contextMenu.className='rb-context-menu';contextMenu.dataset.rbContext=k;
 const clicked=event.target.closest?.('[data-rb-item]'),copyPath=clicked?.dataset.rbItem||m[side].path;try{contextMenu.transferLocation=locationSnapshot(k,side,copyPath);}catch(error){contextMenu.transferLocationError=error.message;}
 contextMenu.innerHTML=(file?(file.directory?label('Open folder','open'):label('Edit text file','edit')+label('Preview','preview'))+label('Rename / Move','rename')+label('Copy','copyFiles')+label('Copy to Windows Explorer…','explorerCopy')+label('Cut','cutFiles')+label('Delete','delete')+(k==='ssh'&&side==='remote'?label('Permissions','permissions'):'')+label(side==='remote'?'Copy remote path':'Copy local path','copyPath'):'')+label('Copy location','copyLocation')+label('Paste here','pasteFiles')+label('New folder','mkdir')+label('New text file','newFile')+(side==='remote'?label('Upload from Windows','uploadOS'):label('Upload selection','upload'))+label('Refresh','refresh');
 document.body.append(contextMenu);const x=Math.min(event.clientX,innerWidth-230),y=Math.min(event.clientY,innerHeight-contextMenu.offsetHeight-8);contextMenu.style.left=Math.max(4,x)+'px';contextMenu.style.top=Math.max(4,y)+'px';
}
function saveSharedSSHProfile(nameOverride){const c=configurations.ssh,m=get('ssh');const host=String(c.host||'').trim(),username=String(c.username||'').trim(),port=Number(c.port);
 if(!host||!username||!Number.isInteger(port)||port<1||port>65535){m.showSettings=true;render('ssh');note('ssh','Enter a valid host, port, and username before saving the profile.','error');return;}
 const existing=(state.terminalProfiles||[]).find(x=>x.id===(m.profileId||m.editingProfileId)&&x.type==='ssh');const name=typeof nameOverride==='string'?nameOverride:prompt('SSH / SFTP profile name:',existing?.name||host);if(name===null)return;if(!name.trim()){note('ssh','Enter a profile name.','error');return;}
 if(!existing&&(state.terminalProfiles||[]).length>=100){note('ssh','The profile list has reached its 100-item limit.','error');return;}
 const id=existing?.id||'terminal-profile-'+Date.now().toString(36)+'-'+Math.random().toString(36).slice(2,8);
 const profile={id,name:name.trim().slice(0,100),folder:root.NetworkToolkitSessionFolders.normalize(existing?.folder),type:'ssh',host:host.slice(0,512),port,username:username.slice(0,256),authMethod:c.authMode==='key'?'key':'password',sshCompatibility:c.sshCompatibility==='legacy'?'legacy':'modern',timeoutMs:10000,credentialId:c.credentialId||''};
 state.terminalProfiles=[profile,...(state.terminalProfiles||[]).filter(x=>x.id!==id)].slice(0,100);m.profileId=id;m.editingProfileId='';c.profileName=name.trim();m.sourceChoice='profile:'+id;remember('ssh');root.NetworkToolkitSessionSidebar?.render?.();render('ssh');
 note('ssh','SSH profile saved and available in both Terminal and SFTP. Passwords and private keys are not stored in the profile. Manage credentials through the protected credential manager.','success');}
function dispatch(k,name){const m=get(k);if(name==='renewAccess'){void api('/editor-views/current/explorer','POST',{}).then(()=>{const editor=document.querySelector('#networkToolkitFileEditor');root.NetworkToolkitEditorFiles.toggle(editor);root.NetworkToolkitEditorFiles.toggle(editor);});return;}if(name==='copyLocation'){copyLocation(k);return;}if(name==='copyEndpoint'){try{root.NetworkToolkitTransferLocations.show(locationSnapshot(k,'remote'));}catch(error){note(k,error.message,'error');}return;}if(name==='disconnect'){void disconnect(k);return;}if(name==='reconnect'){void reconnect(k);return;}if(name==='openTerminal'){void openTerminal(k);return;}if(name==='refreshSession'){if(m.connected){void load(k,'local',m.local.path,true);void load(k,'remote',m.remote.path,true);}return;}if(name==='connectionSettings'){const el=rootNode(k),details=el?.querySelector('.nt-editor-files-connection');if(details){details.open=!details.open;if(details.open)details.querySelector('input,select,button')?.focus({preventScroll:true});}else el?.clientSidebarSelect?.('settings');return;}if(name==='saveCredential'){void storeConnectionCredential(k);return;}if(name==='approveCredential'){void approveConnectionCredential(k);return;}if(name==='chooseKey'){rootNode(k)?.querySelector('[data-rb-key-file]')?.click();return;}if(name==='saveProfile'){const value=configurations[k].profileName?.trim();if(value){if(k==='ssh')saveSharedSSHProfile(value);else saveFTPProfile(value);}else saveConnectionDialog(k);return;}if(name==='librarySaved'){showLibrary(k,'saved');return;}if(name==='libraryRecent'){showLibrary(k,'recent');return;}if(name==='manageProfiles'&&k==='ftp'){showFTPProfiles();return;}if(name==='savePassword'&&k==='ftp'){void storeFTPPassword();return;}if(name==='passwordManager'){void root.NetworkToolkitCredentials?.show?.();return;}if(name==='manageProfiles'&&k==='ssh'){if(typeof showTerminalProfiles==='function')showTerminalProfiles();else root.NetworkToolkitCredentials?.show?.();return;}
 if(name==='toggleSettings'&&rootNode(k)?.clientSidebarSelect){rootNode(k).clientSidebarSelect('settings',true);return;}if(name==='toggleSettings'){m.showSettings=!m.showSettings;const root=rootNode(k);root?.querySelector('.rb-connection-details')?.toggleAttribute('hidden',!m.showSettings);const btn=root?.querySelector('[data-rb-action="toggleSettings"]');if(btn){btn.textContent=m.showSettings?'Hide settings':'Connection settings';btn.setAttribute('aria-expanded',String(m.showSettings));}return;}
 if(name==='selectSource'){
  const picked=rootNode(k)?.querySelector('[data-rb-connection-source]')?.value||'manual';m.sourceChoice=picked;
  if(k==='ftp'){
   if(picked.startsWith('profile:')||picked.startsWith('recent:')){
    const recent=picked.startsWith('recent:');loadFTPProfile(picked.slice(recent?7:8),recent?'recent':'saved');
    if(configurations.ftp.credentialId||configurations.ftp.password)void connect('ftp');return;
   }
   loadFTPProfile('');return;
  }
  if(picked.startsWith('connection:')){void connectExisting(k,picked.slice(11));return;}
  if(picked.startsWith('session:')){void connectTerminal(k,picked.slice(8));return;}
  if(picked.startsWith('profile:')||picked.startsWith('recent:')){void useSavedSSHProfile(picked.slice(picked.indexOf(':')+1));return;}
  invalidateConnection(k);m.showSettings=true;m.profileId='';m.editingProfileId='';m.sessionId='';m.terminalCandidate='';m.connected=false;resetSSHConnectionProfile();resetSSHProfileAuth();render(k);return;
 }
if(name==='fullEditor'){const file=chosen(k,m.active)[0];root.NetworkToolkitFileEditor?.preferConnection?.('client:'+k,m.active==='remote'&&!file?.directory?file?.path||'':'');if(typeof navigate==='function')navigate('editor');return;}if(name==='edit'){const item=chosen(k,m.active)[0];if(item?.directory)void load(k,m.active,item.path);else void edit(k);return;}if(name==='newFile'){void edit(k,true);return;}
 if(name==='uploadOS'){rootNode(k)?.querySelector('[data-rb-os-files]')?.click();return;}
 if(name==='copyFiles'||name==='cutFiles'){clipboard={k,side:m.active,identity:m.active==='remote'?dragIdentity(k):'',paths:chosen(k,m.active).map(x=>({...x})),move:name==='cutFiles'};updateSidebarActions(k);toast(clipboard.paths.length+' item(s) copied to file manager clipboard.','info');return;}
 if(name==='pasteFiles'){void pasteFiles(k);return;}
 if(name==='refresh'){void load(k,m.active,m[m.active].path,true);return;}
 if(name==='open'){const item=chosen(k,m.active)[0];if(item?.directory)void load(k,m.active,item.path);return;}
 if(name==='permissions'){const item=chosen(k,m.active)[0];if(!item||m.active!=='remote'||k!=='ssh')return;const mode=prompt('Remote permissions (octal, e.g. 0644 for file or 0755 for folder):','0644');if(mode===null)return;if(!/^[0-7]{3,4}$/.test(mode)){toast('Enter 3 or 4 octal digits.','error');return;}void request(k,'chmod',{path:item.path,permissions:mode}).then(()=>{toast('Permissions updated','success');void load(k,'remote',m.remote.path,true);}).catch(e=>note(k,e.message,'error'));return;}
 if(name==='explorerCopy'){void prepareExplorerCopy(k).catch(e=>note(k,e.message,'error'));return;}if(name==='inspect'){void inspect(k);return}if(name==='connect'){void connect(k);return}if(name==='useTerminal'){const select=rootNode(k)?.querySelector('[data-rb-terminal-picker]');void connectTerminal(k,select?.value||m.terminalCandidate);return}if(name==='useProfile'){const select=rootNode(k)?.querySelector('[data-rb-profile]');void loadProfile(k,select?.value||m.profileId);return}if(name==='independent'){invalidateConnection(k);m.connected=false;m.sessionId='';m.terminalCandidate='';render(k);return}if(name==='stopPending'){m.cancelPending=true;toast('Pending items will stop after the current file.','info');return}if(name==='clearActivity'){m.activity=[];drawActivity(k);return}if(name==='preview'){void preview(k);return}if(name==='copyPath'){const item=chosen(k,m.active)[0];const path=item?.path||m[m.active].path;void copyText(path);return}if(['mkdir','rename','delete','upload','download'].includes(name)){dialog(k,name)}}
function events(){document.addEventListener('keydown',event=>{const item=event.target.closest?.('.rb-context-menu button');if(!item||!contextMenu)return;const buttons=[...contextMenu.querySelectorAll('button')];if(event.key==='Escape'){event.preventDefault();const k=contextMenu.dataset.rbContext;contextMenu.remove();contextMenu=null;rootNode(k)?.querySelector('[data-rb-scroll="'+get(k).active+'"]')?.focus();return;}if(['ArrowUp','ArrowDown','Home','End'].includes(event.key)){event.preventDefault();const at=buttons.indexOf(item);buttons[event.key==='Home'?0:event.key==='End'?buttons.length-1:(at+(event.key==='ArrowUp'?-1:1)+buttons.length)%buttons.length]?.focus();}},true);document.addEventListener('pointerdown',event=>{for(const menu of document.querySelectorAll('.nt-editor-files-actions[open]'))if(!menu.contains(event.target))menu.open=false;},true);document.addEventListener('keydown',event=>{const menu=event.target.closest?.('.nt-editor-files-actions[open]');if(menu&&event.key==='Escape'){event.preventDefault();menu.open=false;menu.querySelector('summary').focus();}},true);document.addEventListener('click',event=>{const crumb=event.target.closest?.('[data-rb-breadcrumb]');if(!crumb)return;const k=crumb.closest('[data-rb-root]')?.dataset.rbRoot;if(TYPES.has(k)){event.preventDefault();if(get(k).busy)return;void load(k,crumb.dataset.rbSide,crumb.dataset.rbBreadcrumb);}},true);document.addEventListener('click',event=>{
 const lib=event.target.closest?.('[data-rb-library-command],[data-rb-library-tab]');
 if(lib){const area=lib.closest?.('[data-rb-library]'),k=area?.dataset.rbLibrary;if(!TYPES.has(k)||!pageActive(k))return;
  event.preventDefault();event.stopImmediatePropagation();
  if(lib.dataset.rbLibraryTab){get(k).libraryTab=lib.dataset.rbLibraryTab;render(k);return;}
  const row=lib.closest('[data-rb-library-kind][data-rb-library-id]');libraryCommand(k,lib.dataset.rbLibraryCommand,row?.dataset.rbLibraryKind,row?.dataset.rbLibraryId);return;
 }

  const libraryMenu=event.target.closest?.('[data-rb-library-menu]');if(libraryMenu&&contextMenu){event.preventDefault();event.stopImmediatePropagation();const k=contextMenu.dataset.rbContext,kind=libraryMenu.dataset.rbLibraryKind,id=libraryMenu.dataset.rbLibraryId,action=libraryMenu.dataset.rbLibraryMenu;contextMenu.remove();contextMenu=null;libraryCommand(k,action,kind,id);return;}
  const menuItem=event.target.closest('[data-rb-menu]');if(menuItem&&contextMenu){event.preventDefault();event.stopImmediatePropagation();const k=contextMenu.dataset.rbContext;const action=menuItem.dataset.rbMenu,location=contextMenu.transferLocation,error=contextMenu.transferLocationError;contextMenu.remove();contextMenu=null;if(action==='copyLocation'){if(error)note(k,error,'error');else if(location)root.NetworkToolkitTransferLocations.show(location);return;}if(TYPES.has(k))dispatch(k,action);return;}
  if(contextMenu&&!event.target.closest('.rb-context-menu')){contextMenu.remove();contextMenu=null;}

 const node=event.target.closest('[data-rb-action],[data-rb-nav],[data-rb-item],[data-rb-sort],[data-rb-confirm]');if(!node)return;let rootEl=node.closest('[data-rb-root]');if(!rootEl&&node.dataset.rbConfirm)rootEl=rootNode(node.dataset.rbKind);const k=rootEl?.dataset.rbRoot||node.dataset.rbKind;if(!TYPES.has(k)||!pageActive(k))return;
 event.preventDefault();event.stopPropagation();const m=get(k);
 if(node.dataset.rbAction){const menu=node.closest('.nt-editor-files-actions,.nt-editor-files-session-menu');if(menu)menu.open=false;dispatch(k,node.dataset.rbAction);return}
 if(node.dataset.rbNav){nav(k,node.dataset.rbSide,node.dataset.rbNav);return}
 if(node.dataset.rbConfirm){void perform(k,node.dataset.rbConfirm);return}
 if(node.dataset.rbSort){const v=m[node.dataset.rbSide];if(v.sort===node.dataset.rbSort)v.reverse=!v.reverse;else{v.sort=node.dataset.rbSort;v.reverse=false};paint(k,node.dataset.rbSide);return}
 if(node.dataset.rbItem){rowSelect(k,node.dataset.rbSide,node.dataset.rbItem,event);node.closest('[data-rb-scroll]')?.focus({preventScroll:true});}
 },true);
 document.addEventListener('dblclick',event=>{const row=event.target.closest?.('[data-rb-library-id]');const panel=row?.closest('[data-rb-library]');if(!panel)return;event.preventDefault();event.stopImmediatePropagation();void useLibraryItem(panel.dataset.rbLibrary,row.dataset.rbLibraryKind,row.dataset.rbLibraryId);},true);
 document.addEventListener('contextmenu',event=>{const row=event.target.closest?.('[data-rb-library-id]');const panel=row?.closest('[data-rb-library]');if(!panel)return;event.preventDefault();event.stopImmediatePropagation();
 const k=panel.dataset.rbLibrary,kind=row.dataset.rbLibraryKind,id=row.dataset.rbLibraryId;if(contextMenu)contextMenu.remove();contextMenu=document.createElement('div');contextMenu.className='rb-context-menu';contextMenu.dataset.rbContext=k;
 contextMenu.innerHTML=[['open','Connect'],['edit','Edit connection'],['rename','Rename'],['duplicate','Duplicate'],...(kind==='recent'?[['promote','Save to Saved']]:[]),['remove','Remove']].map(([op,label])=>'<button type="button" data-rb-library-menu="'+op+'" data-rb-library-kind="'+kind+'" data-rb-library-id="'+escape(id)+'">'+label+'</button>').join('');
 document.body.append(contextMenu);contextMenu.style.left=Math.max(4,Math.min(event.clientX,innerWidth-225))+'px';contextMenu.style.top=Math.max(4,Math.min(event.clientY,innerHeight-contextMenu.offsetHeight-8))+'px';},true);
 document.addEventListener('contextmenu',event=>{const pane=event.target.closest('[data-rb-pane]');if(!pane)return;const k=pane.closest('[data-rb-root]')?.dataset.rbRoot;if(!TYPES.has(k)||!pageActive(k))return;event.preventDefault();const side=pane.dataset.rbPane,row=event.target.closest('[data-rb-item]');if(row&&!get(k)[side].selected.has(row.dataset.rbItem))rowSelect(k,side,row.dataset.rbItem,{});menu(k,side,event);},true);
 document.addEventListener('change',event=>{const drive=event.target.closest('[data-rb-drive]');if(drive){const k=drive.closest('[data-rb-root]')?.dataset.rbRoot;if(TYPES.has(k)&&get(k).local.drives?.some(item=>item.path===drive.value))void load(k,'local',drive.value);return;}const position=event.target.closest('[data-rb-session-position]');if(position){const k=position.closest('[data-rb-root]')?.dataset.rbRoot;if(TYPES.has(k)){state.terminalActiveActionsPosition=position.value==='bottom'?'bottom':'top';saveState();positionSessionTools(k);root.NetworkToolkitTerminalExtras?.sync?.();}return;}const input=event.target.closest('[data-rb-os-files]');if(!input)return;const k=input.closest('[data-rb-root]')?.dataset.rbRoot;if(TYPES.has(k)&&input.files?.length)void dropFiles(k,[...input.files],get(k).remote.path||'.');input.value='';},true);
 document.addEventListener('dblclick',event=>{const row=event.target.closest('[data-rb-item]');if(!row)return;const k=row.closest('[data-rb-root]')?.dataset.rbRoot;if(!TYPES.has(k)||!pageActive(k))return;event.preventDefault();const side=row.dataset.rbSide,item=get(k)[side].entries.find(x=>x.path===row.dataset.rbItem);if(!item)return;if(item.directory)void load(k,side,item.path);else{get(k).active=side;get(k)[side].selected=new Set([item.path]);void edit(k,false,side)}},true);
 document.addEventListener('input',event=>{
 if(event.target.matches?.('[data-rb-library-search]')){const k=event.target.closest('[data-rb-library]')?.dataset.rbLibrary;if(!TYPES.has(k))return;get(k).libraryFilter=event.target.value;
  const query=event.target.value.trim().toLowerCase();for(const row of event.target.closest('[data-rb-library]')?.querySelectorAll('[data-rb-library-id]')||[]){row.hidden=!row.textContent.toLowerCase().includes(query);}return;}
 const el=event.target,k=el.closest('[data-rb-root]')?.dataset.rbRoot;if(!TYPES.has(k))return;const c=configurations[k];if(el.dataset.rbCredential!==undefined||el.dataset.rbFtpCredential!==undefined){invalidateConnection(k);c.credentialId=el.value;c.password='';c.privateKey='';c.passphrase='';get(k).profileManualAuth=false;const item=root.NetworkToolkitCredentials.meta(el.value);if(item){if(!c.username)c.username=item.username||'';if(k==='ssh')c.authMode=item.kind==='ssh-key'?'key':'password';}remember(k);render(k);return;}if(el.dataset.rbConnectionSource!==undefined){get(k).sourceChoice=el.value;return;}if(el.dataset.rbTerminalPicker!==undefined){get(k).terminalCandidate=el.value;const button=rootNode(k)?.querySelector('[data-rb-action="useTerminal"]');if(button)button.disabled=!el.value;return}if(el.dataset.rbProfile!==undefined){const button=rootNode(k)?.querySelector('[data-rb-action="useProfile"]');if(button)button.disabled=!el.value;return}if(el.dataset.rbSetting){const key=el.dataset.rbSetting;c[key]=key==='port'?Number(el.value):el.value;if(k==='ssh'&&['password','passphrase','privateKey'].includes(key)&&el.value)get(k).profileManualAuth=true;remember(k);if(['host','port','username','authMode','security','fingerprint','keyPath','sshCompatibility'].includes(key)){invalidateConnection(k);restoreConnectionControls(k);get(k).connected=false;if(k==='ftp'&&['host','port','username','security'].includes(key)){get(k).editingProfileId=get(k).profileId||get(k).editingProfileId;get(k).profileId='';get(k).sourceChoice='manual';}if(k==='ssh'&&['host','port','username','authMode','keyPath'].includes(key)){get(k).editingProfileId=get(k).profileId||get(k).editingProfileId;get(k).profileId='';get(k).sourceChoice='manual';get(k).profileManualAuth=key==='keyPath';configurations.ssh.privateKey='';}}if(key==='authMode'||key==='security'){const v=c.password,p=c.passphrase;render(k);configurations[k].password=v;configurations[k].passphrase=p}return}if(el.dataset.rbFilter){const v=get(k)[el.dataset.rbFilter];v.filter=el.value;paint(k,el.dataset.rbFilter)}},true);
 document.addEventListener('keydown',event=>{const input=event.target.closest('[data-rb-path]');if(input&&event.key==='Enter'){const k=input.closest('[data-rb-root]')?.dataset.rbRoot;if(TYPES.has(k)){event.preventDefault();nav(k,input.dataset.rbPath,'go')}return}
 const pane=event.target.closest('[data-rb-pane]'),k=pane?.closest('[data-rb-root]')?.dataset.rbRoot;if(!pane||!TYPES.has(k)||event.target.closest('input,textarea,select'))return;const side=pane.dataset.rbPane,m=get(k);m.active=side;
 if(m.busy)return;
 const command=event.ctrlKey||event.metaKey,key=event.key.toLowerCase();
 if(command&&key==='a'){event.preventDefault();m[side].selected=new Set(sortedItems(m[side]).map(x=>x.path));paintSelection(k,side);return;}
 if(command&&['c','x','v'].includes(key)){event.preventDefault();dispatch(k,{c:'copyFiles',x:'cutFiles',v:'pasteFiles'}[key]);return;}
 if(event.altKey&&['ArrowLeft','ArrowRight','ArrowUp'].includes(event.key)){event.preventDefault();nav(k,side,{ArrowLeft:'back',ArrowRight:'forward',ArrowUp:'up'}[event.key]);return;}
 if(['ArrowUp','ArrowDown','Home','End'].includes(event.key)){
  event.preventDefault();const entries=sortedItems(m[side]);if(!entries.length)return;
  const current=entries.findIndex(item=>item.path===m[side].cursor),at=event.key==='Home'?0:event.key==='End'?entries.length-1:Math.max(0,Math.min(entries.length-1,current+(event.key==='ArrowUp'?-1:1)));
  rowSelect(k,side,entries[at].path,{shiftKey:event.shiftKey});
  [...pane.querySelectorAll('[data-rb-item]')].find(row=>row.dataset.rbItem===entries[at].path)?.scrollIntoView({block:'nearest'});return;
 }
 if(event.key==='Enter'){event.preventDefault();dispatch(k,'edit');return;}
 if(event.key==='Escape'){event.preventDefault();m[side].selected.clear();paintSelection(k,side);return;}
 if(event.key==='ContextMenu'||event.shiftKey&&event.key==='F10'){event.preventDefault();const item=chosen(k,side)[0],row=[...pane.querySelectorAll('[data-rb-item]')].find(row=>row.dataset.rbItem===item?.path),r=(row||pane).getBoundingClientRect();menu(k,side,{target:row||pane,clientX:r.left+16,clientY:r.top+16});contextMenu?.querySelector('button')?.focus();return;}

 const actions={F2:'rename',Delete:'delete',F5:'refresh',Backspace:'up'};const op=actions[event.key];if(op){event.preventDefault();if(op==='refresh'||op==='up')nav(k,side,op);else dispatch(k,op)}
 },true);
 // Export preparation is explicit, never triggered by hover.
 document.addEventListener('dragstart',event=>{const row=event.target.closest('[data-rb-item]');if(!row)return;const k=row.closest('[data-rb-root]')?.dataset.rbRoot;if(!TYPES.has(k))return;const side=row.dataset.rbSide,m=get(k);if(!m[side].selected.has(row.dataset.rbItem))rowSelect(k,side,row.dataset.rbItem,{});
  const files=chosen(k,side);event.dataTransfer?.setData('application/x-networktoolkit-remote-browser',JSON.stringify({k,side,files}));event.dataTransfer.effectAllowed='copy';
  if(files.length===1&&!files[0].directory){const entry=files[0],key=k+':'+side+':'+entry.path+(side==='remote'?':'+dragIdentity(k):'');if(canNativeDrag(k,side))root.NetworkToolkitNativeDrag?.apply(event,key);else note(k,'Drag within the file browser, or use Download. Windows drag-out requires an active SSH connection or an approved saved credential.','info');}
 },true);
 document.addEventListener('dragover',event=>{const pane=event.target.closest('[data-rb-pane]');if(!pane)return;const k=pane.closest('[data-rb-root]')?.dataset.rbRoot;if(!TYPES.has(k))return;event.preventDefault();const external=root.NetworkToolkitTransferDrop.hasFiles(event.dataTransfer),valid=!get(k).busy&&(!external||pane.dataset.rbPane==='local'||k!=='ftp'&&get(k).connected);pane.classList.toggle('rb-drag-target',valid);if(event.dataTransfer)event.dataTransfer.dropEffect=valid?'copy':'none';},true);
 document.addEventListener('dragleave',event=>{event.target.closest('[data-rb-pane]')?.classList.remove('rb-drag-target')},true);
 document.addEventListener('drop',event=>{const pane=event.target.closest('[data-rb-pane]');if(!pane)return;const k=pane.closest('[data-rb-root]')?.dataset.rbRoot;if(!TYPES.has(k))return;event.preventDefault();pane.classList.remove('rb-drag-target');const target=pane.dataset.rbPane,dt=event.dataTransfer;
  if(root.NetworkToolkitTransferDrop.hasFiles(dt)){const folder=get(k)[target].path||'.';void root.NetworkToolkitTransferDrop.fromDropTree(dt).then(files=>dropFiles(k,files,folder,target)).catch(error=>note(k,error.message,'error'));return;}
  const external=dt.getData('application/x-networktoolkit-sidebar-sftp');if(external){try{void dropTransfer(k,null,target,JSON.parse(external));}catch(e){note(k,e.message,'error');}return;}
  const value=dt.getData('application/x-networktoolkit-remote-browser');if(!value)return;
  try{const d=JSON.parse(value);if(d.k!==k||d.side===target)return;get(k)[d.side].selected=new Set(d.files.map(x=>x.path));void dropTransfer(k,d.side,target);}catch(e){note(k,e.message,'error');}
 },true);
 document.addEventListener('pointerdown',event=>{const gutter=event.target.closest('[data-rb-splitter]');if(!gutter)return;const k=gutter.closest('[data-rb-root]')?.dataset.rbRoot;if(!TYPES.has(k))return;event.preventDefault();const m=get(k),area=gutter.parentElement,bounds=area.getBoundingClientRect();const move=e=>{m.split=Math.max(26,Math.min(74,Math.round((e.clientX-bounds.left)/bounds.width*100)));area.style.setProperty('--rb-left',m.split+'%')};const done=()=>{window.removeEventListener('pointermove',move);window.removeEventListener('pointerup',done);remember(k)};window.addEventListener('pointermove',move);window.addEventListener('pointerup',done)},true);
}
document.addEventListener('submit',event=>{
 const form=event.target;
 if(form?.id==='rbLibraryNameForm'||form?.id==='rbLibrarySaveForm'){
  event.preventDefault();event.stopImmediatePropagation();
  const k=form.dataset.rbLibraryProto;
  if(form.id==='rbLibrarySaveForm'){
   const value=String(form.querySelector('#rbLibraryName')?.value||'').trim();if(!value)return;
   closeModal();if(k==='ssh')saveSharedSSHProfile(value);else if(k==='ftp')saveFTPProfile(value);
  }else commitLibraryName(form);return;
 }
 if(!['rbSSHProfileAuthForm','rbSSHProfileVerifyForm','rbSSHKeyChangeForm'].includes(form?.id))return;
 event.preventDefault();event.stopImmediatePropagation();
 const m=get('ssh'),c=configurations.ssh;
 if(form.id==='rbSSHKeyChangeForm'){
  const pending=m.pendingFingerprint,entered=String(form.querySelector('#rbSSHChangedFingerprint')?.value||'').trim();
  if(!pending||c.host!==pending.host||Number(c.port)!==Number(pending.port)){closeModal();note('ssh','The connection changed. Try again.','error');return;}
  if(entered!==pending.observed){const error=form.querySelector('#rbSSHKeyChangeError');if(error){error.hidden=false;error.textContent='The verified fingerprint does not match the presented key. Connection blocked.';}return;}
  const identity=String(c.host).toLowerCase()+':'+String(c.port);
  state.sftpVerifiedHostKeys=state.sftpVerifiedHostKeys||{};state.sftpVerifiedHostKeys[identity]=entered;
  c.fingerprint=entered;m.pendingFingerprint=null;remember('ssh');saveState();closeModal();
  if(m.profileId)void useSavedSSHProfile(m.profileId);else void connect('ssh');return;
 }
 if(form.id==='rbSSHProfileVerifyForm'){
  const profile=sshProfileById(form.dataset.rbVerifyProfile),pending=m.pendingFingerprint;
  if(!profileReadyForAuthentication(profile)||pending?.profileId!==profile.id){closeModal();note('ssh','The selected SSH profile changed. Choose the connection again.','error');return;}
  const verified=String(form.querySelector('#rbSSHProfileVerifiedFingerprint')?.value||'').trim();
  if(!verified||verified!==pending.observed){const status=form.querySelector('#rbSSHProfileVerifyError');if(status){status.hidden=false;status.textContent='The independently verified fingerprint does not match the server. Connection blocked.';}return;}
  c.fingerprint=verified;m.pendingFingerprint=null;state.sftpVerifiedHostKeys=state.sftpVerifiedHostKeys||{};state.sftpVerifiedHostKeys[String(c.host).toLowerCase()+':'+String(c.port)]=verified;remember('ssh');closeModal();void useSavedSSHProfile(profile.id);return;
 }
 const profile=sshProfileById(form.dataset.rbAuthProfile);
 if(!profileReadyForAuthentication(profile)){closeModal();note('ssh','The selected SSH profile changed. Choose the connection again.','error');return;}
 if(profile.authMethod==='key'){
  const privateKey=String(form.querySelector('#rbSSHProfilePrivateKey')?.value||'').trim(),keyPath=String(form.querySelector('#rbSSHProfileKeyPath')?.value||'').trim();
  if(!privateKey&&!keyPath){const input=form.querySelector('#rbSSHProfileKeyPath');input?.setCustomValidity('Provide the SSH private key or its local file path.');input?.reportValidity();return;}
  c.authMode='key';c.privateKey=privateKey;c.keyPath=keyPath;c.passphrase=String(form.querySelector('#rbSSHProfilePassphrase')?.value||'');c.password='';
 }else{
  const password=String(form.querySelector('#rbSSHProfilePassword')?.value||'');if(!password)return;
  c.authMode='password';c.password=password;c.privateKey='';c.keyPath='';c.passphrase='';
 }
 m.profileManualAuth=true;closeModal();void connect('ssh');
},true);
document.addEventListener('click',event=>{const node=event.target.closest?.('[data-ftp-profile-action]');if(!node)return;event.preventDefault();event.stopImmediatePropagation();const id=node.dataset.id,action=node.dataset.ftpProfileAction,item=ftpProfileRow(id);if(!item)return;
 if(action==='load'){closeModal();navigate('ftp');loadFTPProfile(id);return;}
 if(action==='rename'){const name=prompt('Rename FTP/FTPS profile:',item.name);if(name===null)return;if(!name.trim()){toast('Enter a profile name.','error');return;}item.name=name.trim().slice(0,100);saveState();showFTPProfiles();return;}
 if(action==='delete'){if(!confirm('Delete saved FTP/FTPS profile "'+item.name+'"? The linked credential will remain in the password manager.'))return;state.ftpProfiles=ftpProfiles().filter(x=>x.id!==id);if(get('ftp').profileId===id){get('ftp').profileId='';get('ftp').sourceChoice='manual';configurations.ftp.credentialId='';}saveState();showFTPProfiles();}
},true);
// Discovery opens a fresh, reviewable form, never a saved identity or a live
// connection. Do not silently displace ongoing transfers or authenticated users.
function prepareDeviceConnection(protocol,host,port){
 const k=protocol==='sftp'?'ssh':['ftp','ftps'].includes(protocol)?'ftp':'';
 const target=typeof endpointHost==='function'?endpointHost(host):'',number=Number(port);
 if(!k||!target||!Number.isInteger(number)||number<1||number>65535)throw new Error('Choose a valid device host and TCP port.');
 const m=get(k);
 if(m.connected||m.connecting||m.busy||m.profileSelectionPending||m.pendingProfileLoad||m.pendingCredentialSave||m.pendingClose||m.pendingKeyRead||m.connectionId||m.sessionId||m.remote.loading)throw new Error('The '+(k==='ssh'?'SFTP':'FTP / FTPS')+' client already has an active connection or operation. Wait for the operation to finish or disconnect the existing session first; the existing session has not changed.');
 Object.assign(configurations[k],{host:target,port:number,username:'',authMode:'password',password:'',privateKey:'',keyPath:'',passphrase:'',fingerprint:'',security:k==='ssh'?'ssh':protocol==='ftps'?'implicit':'explicit',sshCompatibility:'modern',credentialId:'',profileName:''});
 Object.assign(m,{saved:true,profileId:'',editingProfileId:'',sourceChoice:'manual',autoTerminal:'',terminalCandidate:'',profileManualAuth:false,pendingFingerprint:null,showSettings:true,lastError:'',remote:newPane()});
 state.transferModes=state.transferModes||{};state.transferModes[k==='ssh'?'sftp':'ftp']='client';state.fileTransferProtocol=k==='ssh'?'sftp':'ftp';
 remember(k);navigate(k==='ssh'?'sftp':'ftp');rootNode(k)?.clientSidebarSelect?.('settings');
 note(k,k==='ssh'?'Review the destination and independently verify its SSH fingerprint before connecting.':'Review the destination and TLS mode before connecting. Plain FTP is unencrypted; saved credentials require FTPS.','info');
}
function resetLayout(k){if(!TYPES.has(k))return;const m=get(k);m.split=50;for(const side of ['local','remote']){m[side].sort='name';m[side].reverse=false;m[side].filter='';m[side].selected.clear();m[side].anchor=-1}remember(k);if(pageActive(k))render(k)}
function revisionFor(sessionId){return get('ssh').sessionOptions.find(x=>x.id===sessionId)?.revision??(typeof terminalSessionRevision==='function'?terminalSessionRevision(sessionId):0)}

// The editor reuses this existing connection UI, never a second credential path.
async function useSavedEditorProfile(k,id,expected){
 if(detachedEditor()||!TYPES.has(k))throw Error('Saved connections must be opened in the main window.');
 const m=get(k),lookup=()=>k==='ssh'?(state.terminalProfiles||[]).find(p=>p.id===id&&p.type==='ssh'):ftpProfileRow(id),profile=lookup();
 if(!profile)throw Error('The saved profile no longer exists. Refresh the list.');
 const protocol=k==='ssh'?'ssh':['explicit','implicit'].includes(profile.security)?'ftps':'ftp';
 if(expected&&(profile.name!==expected.name||profile.host!==expected.host||Number(profile.port)!==expected.port||protocol!==expected.protocol))throw Error('The saved profile changed. Refresh and review it again.');
 if(m.connecting||m.busy||m.profileSelectionPending||m.pendingProfileLoad||m.pendingCredentialSave||m.pendingClose||m.pendingKeyRead||m.remote.loading||document.querySelector('#rbSSHProfileAuthForm,#rbSSHProfileVerifyForm,#rbSSHKeyChangeForm'))throw Error('Finish or cancel the existing connection operation first.');
 if(m.connected||m.connectionId||m.sessionId){
  const c=configurations[k];if(m.connected&&m.profileId===id&&c.host===profile.host&&Number(c.port)===Number(profile.port)&&c.username===profile.username&&(k==='ssh'||c.security===profile.security))return {source:'client:'+k,pending:false};
  throw Error('The file client already has another active connection. Use that connected session, or save its open documents and disconnect it explicitly first.');
 }
 const signature=JSON.stringify(profile),check=()=>{if(JSON.stringify(lookup())!==signature)throw Error('The saved profile changed during connection approval. Cancel and choose it again.');};
 m.autoTerminal='';root.NetworkToolkitEditorFiles.showConnection(k);
 m.editorProfileCheck=()=>{if(m.profileId!==id){if(m.editorConnectionPending)throw Error('The connection selection changed during approval.');return;}check();const c=configurations[k];if(c.host!==profile.host||Number(c.port)!==Number(profile.port)||c.username!==profile.username)throw Error('The saved connection destination changed. Choose the profile again.');};
 m.editorConnectionPending=true;try{if(k==='ssh')await useSavedSSHProfile(id,check);else await useLibraryItem(k,'saved',id);
 check();if(m.lastError)throw Error(m.lastError);
 rootNode(k)?.querySelector('.nt-editor-files-connection')?.setAttribute('open','');
 if(!m.connected)rootNode(k)?.clientSidebarSelect?.('settings');
 return {source:m.connected?'client:'+k:'',pending:!m.connected};
 }finally{m.editorConnectionPending=false;rootNode(k)?.querySelector('.nt-editor-files-connection')?.setAttribute('open','');}
}
async function openLibraryEntry(k,kind,id){if(!TYPES.has(k))return;
 state.transferModes=state.transferModes||{};state.transferModes[k==='ssh'?'sftp':'ftp']='client';state.fileTransferProtocol=k==='ssh'?'sftp':'ftp';
 if(typeof navigate==='function')navigate(k==='ssh'?'sftp':'ftp');
 return useLibraryItem(k,kind,id);
}
document.addEventListener('credentials-updated',()=>{for(const k of TYPES)syncCredentials(k);});
document.addEventListener('change',event=>{const input=event.target.closest?.('[data-rb-key-file]');if(!input?.files?.length)return;const k=input.closest('[data-rb-root]')?.dataset.rbRoot;if(k!=='ssh')return;const file=input.files[0];if(file.size>1048576){note(k,'Private key file exceeds 1 MiB.','error');return;}const m=get(k);m.pendingKeyRead=(m.pendingKeyRead||0)+1;void file.text().then(text=>{configurations.ssh.privateKey=text;m.profileManualAuth=true;render('ssh');}).catch(error=>note(k,error.message,'error')).finally(()=>{m.pendingKeyRead--;});},true);
events();root.NetworkToolkitRemoteBrowser={useSavedEditorProfile,mountSidebar,unmountSidebar,render,get,load,request,configurations,resetLayout,useTerminalSession,connectTerminal,loadProfile,revisionFor,dropFiles,dropTransfer,edit,requestPayload,useSavedSSHProfile,openLibraryEntry,libraryRows,connect,disconnect,reconnect,openTerminal,sessionTools,positionSessionTools,invalidateConnection,connectionIdentity,dragIdentity,canNativeDrag,storeConnectionCredential,approveConnectionCredential,saveSharedSSHProfile,saveFTPProfile,prepareDeviceConnection,locationSnapshot,copyLocation,prepareExplorerCopy};
})(globalThis);
