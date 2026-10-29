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
 root.openModal('Copy to Windows Explorer','<p>Prepare a private copy, then drag it into a Windows Explorer folder. Original files remain unchanged. Windows handles destination collision confirmation; moves are not offered.</p><p role="status" aria-live="polite" data-native-copy-status>Preparing…</p><button type="button" class="btn primary" data-native-copy-handle disabled aria-label="Drag prepared files and folders to Windows Explorer">Drag prepared copy</button>','<button type="button" class="btn ghost" data-native-copy-cancel>Cancel / Close</button>',true);
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
