(function(root){
'use strict';
let profiles=[],requests=[],pendingRequest='',connecting=false;
const detached=()=>root.NetworkToolkitEditorFiles?.detached?.()===true;
const route='/editor-views/saved-connections';
const safeProfile=p=>p&&typeof p.id==='string'&&/^(ssh|ftp):.{1,100}$/.test(p.id)&&typeof p.name==='string'&&p.name.length<=100&&typeof p.host==='string'&&p.host.length<=255&&Number.isInteger(p.port)&&p.port>0&&p.port<=65535&&['ssh','ftp','ftps'].includes(p.protocol)?{id:p.id,name:p.name,host:p.host,port:p.port,protocol:p.protocol}:null;
async function list(){
 const result=await api(route),child=detached();
 profiles=(Array.isArray(result?.profiles)?result.profiles:[]).slice(0,300).map(safeProfile).filter(p=>p&&(!child||p.protocol==='ssh'));
 pendingRequest=typeof result.pendingRequest==='string'?result.pendingRequest:'';
 syncPending();return {profiles:profiles.map(p=>({...p})),detached:child,requiresApproval:child,pendingRequest};
}
async function connect(id){
 if(connecting)throw Error('Finish the pending saved connection action first.');
 const profile=profiles.find(p=>p.id===id);if(!profile)throw Error('Refresh and select an existing saved file connection.');
 connecting=true;
 try{
  if(detached()){
   const response=await api(route+'/request','POST',{id});pendingRequest=response.requestId;syncPending();
   return {pending:true,requested:true,source:'',message:'Review this request in the main window. After connecting, use Update access in Files and approve again. No connection has been delegated yet.'};
  }
  return await openProfile(profile);
 }finally{connecting=false;}
}
async function openProfile(profile){
 if(detached())throw Error('Connection approvals stay in the main window.');
 const k=profile.id.startsWith('ssh:')?'ssh':'ftp';
 const result=await root.NetworkToolkitRemoteBrowser.useSavedEditorProfile(k,profile.id.slice(4),profile);
 return {...result,requested:false,message:result.pending?'Finish the host verification or sign-in shown here, then Refresh the connected list.':'Connection available. Refresh the connected list to browse files.'};
}
async function cancelRequest(){
 if(!detached()||!pendingRequest)return;
 const id=pendingRequest;await api(route+'/cancel','POST',{requestId:id});if(pendingRequest===id)pendingRequest='';syncPending();
}
function syncPending(){
 let banner=document.querySelector('[data-editor-saved-pending]');
 if(!detached()||!pendingRequest){banner?.remove();return;}
 if(!banner){banner=document.createElement('div');banner.className='nt-editor-files-access-banner';banner.dataset.editorSavedPending='';const span=document.createElement('span');span.textContent='Saved connection requested in the main window. Approval here does not connect or share credentials.';const cancel=document.createElement('button');cancel.type='button';cancel.className='btn small ghost';cancel.textContent='Cancel request';cancel.onclick=()=>void cancelRequest().catch(e=>toast(e.message,'error'));banner.append(span,cancel);document.body.append(banner);}
}
function syncRequests(rows){
 if(detached())return;
 requests=(Array.isArray(rows)?rows:[]).filter(row=>typeof row.viewId==='string'&&/^[a-f0-9]{32}$/.test(row.viewId)&&typeof row.requestId==='string'&&/^[a-f0-9]{32}$/.test(row.requestId)&&safeProfile(row.profile)).map(row=>({viewId:row.viewId,requestId:row.requestId,name:String(row.name||'Editor window'),profile:safeProfile(row.profile)}));
 let banner=document.querySelector('[data-editor-saved-requests]');
 if(!requests.length){banner?.remove();return;}
 if(!banner){banner=document.createElement('div');banner.className='nt-editor-files-access-banner';banner.dataset.editorSavedRequests='';const span=document.createElement('span');span.textContent='An editor window requests a saved SSH connection.';const button=document.createElement('button');button.type='button';button.className='btn small primary';button.textContent='Review connection';button.onclick=manageRequests;banner.append(span,button);document.body.append(banner);}
}
function manageRequests(){
 if(detached())return;
 openModal('Saved connection requests','<p>Review the destination before using its existing host-key verification and sign-in flow. This does not share credentials or approve new editor file access. After connecting, the requesting editor must use Update access and you must approve it separately.</p>'+requests.map(row=>'<div class="setting-row"><span>'+esc(row.name)+' — '+esc(row.profile.name)+' ('+esc(row.profile.host)+':'+row.profile.port+')</span><div class="row gap6"><button class="btn small primary" type="button" data-editor-saved-review="'+row.viewId+'" data-request="'+row.requestId+'">Review / Connect</button><button class="btn small ghost" type="button" data-editor-saved-decline="'+row.viewId+'" data-request="'+row.requestId+'">Decline</button></div></div>').join(''),'<button class="btn ghost" data-action="close-modal">Close</button>');
}
document.addEventListener('click',event=>{
 const button=event.target.closest?.('[data-editor-saved-review],[data-editor-saved-decline]');if(!button||detached())return;
 const viewId=button.dataset.editorSavedReview||button.dataset.editorSavedDecline,requestId=button.dataset.request,row=requests.find(r=>r.viewId===viewId&&r.requestId===requestId);if(!row||button.disabled)return;
 button.disabled=true;
 void (async()=>{try{
  const response=await api(route+'/review','POST',{viewId,requestId,connect:!!button.dataset.editorSavedReview});
  syncRequests(requests.filter(r=>r!==row));closeModal();
  if(response.connect){const editor=document.querySelector('#networkToolkitFileEditor');if((!editor||editor.hidden)&&typeof navigate==='function')navigate('editor');const profile=safeProfile(response.profile);if(!profile||JSON.stringify(profile)!==JSON.stringify(row.profile))throw Error('The requested destination changed. Request it again.');const result=await openProfile(profile);toast(result.message+' Update access remains a separate approval.','info');}
 }catch(e){button.disabled=false;toast(e.message,'error');}})();
});
root.NetworkToolkitEditorSavedConnections=Object.freeze({list,connect,cancelRequest,syncRequests,manageRequests});
})(globalThis);
