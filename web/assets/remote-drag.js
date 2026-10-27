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
