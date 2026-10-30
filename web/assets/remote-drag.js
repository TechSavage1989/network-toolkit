/* Windows drag-out for local and remote browser files. A drag ticket has one
   use and a short lifetime; neither the OS payload nor the URL contains
   credentials, session IDs, filesystem paths, or authentication headers. */
(function(root){'use strict';
const ready=new Map(),pending=new Map(),TTL=65000,MAX=24;
let hoverTimer=null,hoverRow=null;
function cancelHover(){clearTimeout(hoverTimer);hoverTimer=null;hoverRow=null;}
function hover(row,key,payload){
 if(row===hoverRow||current(key))return;
 cancelHover();hoverRow=row;
 // Moving across names or double-clicking must not start extra SSH logins.
 hoverTimer=setTimeout(()=>{hoverTimer=null;if(!row.isConnected||hoverRow!==row)return;
  void prepare(key,payload).then(ok=>{if(ok&&row.isConnected)row.title='Drag to Windows Explorer to copy this file';});
 },800);
}
if(typeof document!=='undefined'){
 document.addEventListener('pointerout',event=>{if(hoverRow?.contains(event.target)&&!hoverRow.contains(event.relatedTarget))cancelHover();},true);
 document.addEventListener('pointerdown',cancelHover,true);
 document.addEventListener('dblclick',cancelHover,true);
 document.addEventListener('visibilitychange',cancelHover);
}
const current=key=>{const value=ready.get(key);if(value&&Date.now()-value.time<TTL)return value;if(value)ready.delete(key);return null;};
function prepare(key,payload){
 if(current(key))return Promise.resolve(true);
 if(pending.has(key))return pending.get(key);
 const run=Promise.resolve().then(payload).then(body=>{if(['password','privateKey','passphrase','keyPath'].some(field=>body[field]))throw Error('Native drag-out cannot retain manual credentials. Use Download, or use an approved saved credential.');return api('/remote-browser/drag-ticket','POST',body);}).then(result=>{
   if(!/^[0-9a-f]{48}$/i.test(result.ticket||'')||!/^[^\\/\r\n]+$/.test(result.filename||''))throw Error('Invalid drag ticket from local service');
   ready.set(key,{ticket:result.ticket,filename:result.filename,time:Date.now()});
   for(const [id,value] of ready){if(ready.size<=MAX)break;ready.delete(id);}
   return true;
 }).catch(()=>false).finally(()=>pending.delete(key));
 pending.set(key,run);return run;
}
function apply(event,key){
 const item=current(key),transfer=event.dataTransfer;
 if(!item||!transfer)return false;
 const base=root.location?.origin||'';if(!/^https?:\/\//.test(base))return false;
 const url=base+'/drag-download/'+item.ticket;
 try{transfer.setData('DownloadURL','application/octet-stream:'+item.filename+':'+url);
   transfer.effectAllowed='copy';ready.delete(key);return true;}catch{return false;}
}
root.NetworkToolkitNativeDrag=Object.freeze({prepare,apply,hover,cancelHover,has:key=>!!current(key)});
})(globalThis);

// Explicit prepared file/folder copy for the native Windows host. No content is
// read on hover. The opaque job is armed only after a real pointer drag begins.
(function(root){'use strict';
const old=root.NetworkToolkitNativeDrag;
let active=null;
function readable(n){return n<1048576?n+' B':(n/1048576).toFixed(1)+' MiB';}
async function prepareCopy(body){
 if(typeof root.__networkToolkitNativeSession!=='string')throw Error('Windows Explorer drag-out requires the native Toolkit window. Use Download in this browser.');
 if(active)throw Error('Finish or cancel the existing prepared copy first.');
 const response=await api('/explorer-transfer','POST',{action:'prepare',...body});
 const job={id:response.id,closed:false,timer:null,state:response.state,gesture:0,nativePending:false};active=job;
 root.openModal('Copy to Windows Explorer','<p>Prepare a private copy, then drag it into a Windows Explorer folder. Original files remain unchanged. Windows handles destination collision confirmation; moves are not offered. Press Escape to cancel. A pending window close finishes after you end the Windows drag.</p><p role="status" aria-live="polite" data-native-copy-status>Preparing…</p><button type="button" class="btn primary" data-native-copy-handle disabled aria-label="Drag prepared files and folders to Windows Explorer">Drag prepared copy</button>','<button type="button" class="btn ghost" data-native-copy-cancel>Cancel / Close</button>',true);
 const handle=document.querySelector('[data-native-copy-handle]');handle.style.touchAction='none';let pointer=null;
 handle.addEventListener('pointerdown',event=>{if(event.button!==0||job.state!=='ready')return;pointer={id:event.pointerId,x:event.clientX,y:event.clientY,armed:false,gesture:++job.gesture};});
 handle.addEventListener('pointermove',event=>{if(!pointer||pointer.id!==event.pointerId||pointer.armed||!(event.buttons&1)||Math.hypot(event.clientX-pointer.x,event.clientY-pointer.y)<5)return;pointer.armed=true;job.nativePending=true;void api('/explorer-transfer','POST',{action:'arm',id:job.id,gesture:pointer.gesture}).catch(error=>{document.querySelector('[data-native-copy-status]').textContent=error.message;});});
 const release=()=>{if(!pointer)return;const ended=pointer;pointer=null;if(!ended.armed)return;const disarm=async()=>{for(let n=0;n<3&&!job.closed;n++){try{const state=await api('/explorer-transfer','POST',{action:'disarm',id:job.id,gesture:ended.gesture});if(state.state==='ready'&&job.gesture===ended.gesture)job.nativePending=false;return;}catch{await new Promise(resolve=>setTimeout(resolve,100));}}};void disarm();};window.addEventListener('pointerup',release,true);window.addEventListener('pointercancel',release,true);window.addEventListener('blur',release);job.release=()=>{window.removeEventListener('pointerup',release,true);window.removeEventListener('pointercancel',release,true);window.removeEventListener('blur',release);};
 async function poll(){if(job.closed)return;if(!handle.isConnected){void cancel();return;}try{const state=await api('/explorer-transfer?id='+encodeURIComponent(job.id));if(job.closed||active!==job||!handle.isConnected)return;job.state=state.state;if(['finished','failed','cancelled'].includes(state.state))job.nativePending=false;const status=document.querySelector('[data-native-copy-status]');if(status)status.textContent=state.message+' · '+state.entries+' items · '+readable(state.bytes);handle.disabled=state.state!=='ready';if(['finished','failed','cancelled'].includes(state.state))return;}catch(error){const status=document.querySelector('[data-native-copy-status]');if(status)status.textContent=error.message;return;}job.timer=setTimeout(poll,200);}
 void poll();return response;
}
async function cancel(){const job=active;if(!job)return;try{await api('/explorer-transfer','POST',{action:'cancel',id:job.id});}catch(error){root.toast?.(error.message||'Cancellation is not confirmed. Release the drag or press Escape, then retry.','info');return;}job.closed=true;job.release?.();clearTimeout(job.timer);active=null;root.closeModal();}
if(typeof document!=='undefined'){document.addEventListener('click',event=>{if(active&&event.target.closest?.('[data-native-copy-cancel],[data-action="close-modal"],[data-action="modal-backdrop"]')&&!event.target.closest?.('.modal-body')){event.preventDefault();event.stopImmediatePropagation();void cancel();}},true);document.addEventListener('keydown',event=>{if(event.key==='Escape'&&active){event.stopImmediatePropagation();if(active.nativePending||['claimed','dragging'].includes(active.state))return;void cancel();}},true);}
root.NetworkToolkitNativeDrag=Object.freeze({...old,prepareCopy});
})(globalThis);

// Native file-row pointer bridge. HTML drag remains unchanged in browser mode.
// Inside this window the original validated manager/editor routes are used;
// outside it one copy-only native OLE loop takes over, never a second HTML drag.
(function(root){'use strict';
if(!root.document)return;
let gesture=null,operation=null,suppress=false;
const browser=()=>root.NetworkToolkitRemoteBrowser;
const native=()=>typeof root.__networkToolkitNativeSession==='string';
function release(g){if(!g)return;g.row.draggable=true;try{if(g.row.hasPointerCapture?.(g.pointer))g.row.releasePointerCapture(g.pointer);}catch{};g.indicator?.remove();}
function suppressClick(){suppress=true;setTimeout(()=>{suppress=false;},500);}
function discardRegistration(g){if(!g?.id||g.discarded)return Promise.resolve();g.discarded=true;return api('/explorer-transfer','POST',{action:'cancel',id:g.id}).catch(()=>{});}
function cancel(){const g=gesture;gesture=null;root.NetworkToolkitFileEditor?.cancelSidebarPointerDrag?.();release(g);if(g?.started)suppressClick();void discardRegistration(g);}
function begin(event){if(!native()||event.isTrusted===false||event.button!==0||event.isPrimary===false||event.pointerType==='touch'||event.altKey||event.ctrlKey||event.metaKey||event.shiftKey||operation)return;
 const row=event.target.closest?.('[data-rb-item]');if(!row)return;const source=browser()?.capturePointerFileDrag?.(event);if(!source)return;
 cancel();const g={source,row,pointer:event.pointerId,x:event.clientX,y:event.clientY,started:false,handoff:false};gesture=g;row.draggable=false;root.NetworkToolkitFileEditor?.beginSidebarPointerDrag?.(event);try{row.setPointerCapture(event.pointerId);}catch{cancel();}
}
function valid(g){return g===gesture&&!g.handoff&&browser()?.validPointerFileDrag?.(g.source)&&g.row.isConnected;}
function outside(event){return event.clientX<0||event.clientY<0||event.clientX>=root.innerWidth||event.clientY>=root.innerHeight;}
function draw(g,event){if(!g.indicator){g.indicator=document.createElement('div');g.indicator.className='terminal-tab-drag-indicator';g.indicator.setAttribute('role','status');g.indicator.style.pointerEvents='none';document.body.append(g.indicator);}g.indicator.textContent='Copy files · Drop into editor or folder · Esc cancels';g.indicator.style.left=Math.max(4,Math.min(root.innerWidth-300,event.clientX+12))+'px';g.indicator.style.top=Math.max(4,Math.min(root.innerHeight-50,event.clientY+14))+'px';}
// Register only metadata at a genuine moved gesture, without arming or reading
// files. This removes the extra registration round-trip from the window edge.
function register(g){if(g.registration)return g.registration;
 g.registration=(async()=>{try{const state=await api('/explorer-transfer','POST',{action:'direct',...browser().pointerFileDragPayload(g.source)});g.id=state.id;
  if(gesture!==g||!browser().validPointerFileDrag(g.source)){await discardRegistration(g);return false;}return true;
 }catch(error){g.registrationError=error;return false;}})();return g.registration;
}
async function handoff(g){if(g.handoff)return;g.handoff=true;root.NetworkToolkitFileEditor?.cancelSidebarPointerDrag?.();
 try {if(!await register(g)){if(gesture===g&&g.registrationError)throw g.registrationError;return;}
  if(gesture!==g||!browser().validPointerFileDrag(g.source)){await discardRegistration(g);return;}
  await api('/explorer-transfer','POST',{action:'arm',id:g.id,gesture:1});
  if(gesture!==g&&!g.nativeAdmitted){const ended=await api('/explorer-transfer','POST',{action:'disarm',id:g.id,gesture:1});if(ended.state!=='dragging'){await discardRegistration(g);return;}}
  operation={id:g.id,started:Date.now()};poll(operation);g.indicator?.remove();
 }catch(error){if(gesture===g){gesture=null;release(g);suppressClick();}void discardRegistration(g);root.toast?.(error.message,'error');}
}
function progress(job,state){
 if(!job.status){const box=document.createElement('div');box.className='terminal-tab-drag-indicator';box.dataset.nativeDirectProgress='';box.setAttribute('role','status');Object.assign(box.style,{left:'8px',top:'auto',bottom:'16px',maxWidth:'calc(100vw - 16px)',pointerEvents:'auto',display:'flex',gap:'8px',alignItems:'center'});const text=document.createElement('span'),button=document.createElement('button');button.type='button';button.className='btn small';button.textContent='Cancel';button.setAttribute('aria-label','Cancel copy to Windows Explorer');button.onclick=()=>{button.disabled=true;void api('/explorer-transfer','POST',{action:'cancel',id:job.id}).catch(error=>{button.disabled=false;root.toast?.(error.message,'error');});};box.append(text,button);document.body.append(box);job.status=box;job.text=text;}
 job.text.textContent=(state.message||'Copying to Windows Explorer…')+' · '+(state.entries||0)+' items · '+(state.bytes||0).toLocaleString()+' B';
}
async function poll(job){try{const state=await api('/explorer-transfer?id='+encodeURIComponent(job.id));if(operation!==job)return;
 if(['finished','failed','cancelled'].includes(state.state)){operation=null;if(gesture?.id===job.id){const g=gesture;gesture=null;release(g);suppressClick();}job.status?.remove();root.toast?.(state.message,state.state==='failed'?'error':'info');return;}
 if(state.state==='dragging')progress(job,state);
 if(state.state==='ready'&&Date.now()-job.started>2500){const ended=await api('/explorer-transfer','POST',{action:'disarm',id:job.id,gesture:1});if(ended.state!=='dragging'){await api('/explorer-transfer','POST',{action:'cancel',id:job.id});operation=null;job.status?.remove();if(gesture?.id===job.id){const g=gesture;gesture=null;release(g);}return;}}
 setTimeout(()=>poll(job),250);}catch(error){operation=null;job.status?.remove();root.toast?.(error.message,'error');}}
function move(event){const g=gesture;if(!g||event.pointerId!==g.pointer||g.handoff)return;if(!valid(g)||!(event.buttons&1)){cancel();return;}if(!g.started&&Math.hypot(event.clientX-g.x,event.clientY-g.y)<6)return;if(!g.started){g.started=true;void register(g);}event.preventDefault();draw(g,event);if(outside(event))void handoff(g);}
function up(event){const g=gesture;if(!g||event.pointerId!==g.pointer)return;
 if(g.handoff){gesture=null;release(g);suppressClick();if(g.id)void api('/explorer-transfer','POST',{action:'disarm',id:g.id,gesture:1}).then(state=>{if(state.state!=='dragging')return api('/explorer-transfer','POST',{action:'cancel',id:g.id});}).catch(()=>{});return;}
 const okay=valid(g),moved=g.started;gesture=null;release(g);void discardRegistration(g);if(!moved){root.NetworkToolkitFileEditor?.cancelSidebarPointerDrag?.();return;}
 suppressClick();event.preventDefault();if(!okay){root.NetworkToolkitFileEditor?.cancelSidebarPointerDrag?.();return;}
 if(root.NetworkToolkitFileEditor?.finishSidebarPointerDrop?.(event))return;
 root.NetworkToolkitFileEditor?.cancelSidebarPointerDrag?.();browser().finishPointerFileDrag(event,g.source);
}
// Called only after the native host has claimed and acknowledged this exact job.
// Releasing here (not on an HTTP state poll) gives OLE sole capture ownership.
function releaseFileRowCapture(id,serial){const g=gesture;
 if(!g||!g.handoff||!g.started||g.id!==id||serial!==1||g.nativeAdmitted||!g.row.isConnected||!browser()?.validPointerFileDrag?.(g.source))return false;
 g.nativeAdmitted=true;gesture=null;release(g);suppressClick();return true;
}
root.NetworkToolkitNativeDrag=Object.freeze({...root.NetworkToolkitNativeDrag,releaseFileRowCapture});
document.addEventListener('pointerdown',begin,true);document.addEventListener('pointermove',move,true);document.addEventListener('pointerup',up,true);
document.addEventListener('pointercancel',()=>{if(gesture&&!gesture.handoff)cancel();},true);
document.addEventListener('lostpointercapture',event=>{if(gesture&&event.pointerId===gesture.pointer&&!gesture.handoff)cancel();},true);
document.addEventListener('keydown',event=>{if(event.key!=='Escape')return;if(gesture){event.preventDefault();cancel();}else if(operation)void api('/explorer-transfer','POST',{action:'cancel',id:operation.id}).catch(()=>{});},true);
document.addEventListener('click',event=>{if(suppress&&event.target.closest?.('[data-rb-item]')){event.preventDefault();event.stopImmediatePropagation();}},true);
})(globalThis);
