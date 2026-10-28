/* Browser/WebView File-only import boundary. No native source paths, shell
   commands, drag tickets, credentials or HTTP requests are obtained on hover. */
(function(root){'use strict';
const LIMIT=512*1024*1024,MAX_FILES=100;
const safe=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
function validName(name){return typeof name==='string'&&name.length>0&&new TextEncoder().encode(name).length<=255&&!/^[.]{1,2}$|[\\/<>:"|?*\x00-\x1f\x7f]|[. ]$/.test(name)&&!/^(CON|PRN|AUX|NUL|COM[1-9¹²³]|LPT[1-9¹²³])(?:\.|$)/i.test(name);}
function validateFiles(files){const list=Array.from(files||[]);if(!list.length)throw Error('No regular files were selected.');if(list.length>MAX_FILES)throw Error('Choose at most '+MAX_FILES+' files per copy.');for(const file of list){if(!validName(file.name)||/\.(lnk|url)$/i.test(file.name))throw Error('The whole selection was rejected: unsafe filename or shortcut '+String(file.name||'')+'.');if(file.webkitRelativePath)throw Error('Folder drops are not supported. Nothing was imported; select regular files directly.');if(!Number.isSafeInteger(file.size)||file.size<0||file.size>LIMIT)throw Error('The whole selection was rejected: each file must be no larger than 512 MiB.');}return list;}
function fromDrop(data){
 const items=Array.from(data?.items||[]),files=Array.from(data?.files||[]);
 // Inspect every item synchronously while the OS drop's protected data store is
 // readable. Never enumerate folders: one folder rejects the ENTIRE selection.
 if(!items.length||items.length!==files.length)throw Error('Drop regular files only. Folder or unsupported items reject the entire drop.');
 for(const item of items){if(item.kind!=='file')throw Error('Only regular file drops are supported; nothing was imported.');const entry=item.webkitGetAsEntry?.();if(!entry||entry.isDirectory||!entry.isFile)throw Error('Folder, link or unsupported drops are not supported; nothing was imported.');}
 return validateFiles(files);
}
function hasFiles(data){return Array.from(data?.types||[]).includes('Files');}
function sizes(n){if(n<1024)return n+' B';if(n<1048576)return (n/1024).toFixed(1)+' KiB';return (n/1048576).toFixed(1)+' MiB';}
function upload(url,form,{signal,onProgress,headers={}}={}){return new Promise((resolve,reject)=>{
 const xhr=new XMLHttpRequest();let done=false;
 const finish=(err,data)=>{if(done)return;done=true;signal?.removeEventListener('abort',abort);err?reject(err):resolve(data);};
 const abort=()=>xhr.abort();xhr.open('POST',url);xhr.setRequestHeader('X-Session-Token',SESSION);for(const [key,value]of Object.entries(headers))xhr.setRequestHeader(key,value);
 xhr.upload.onprogress=event=>onProgress?.(event.loaded,event.lengthComputable?event.total:0);
 xhr.upload.onload=()=>onProgress?.(1,1,'finishing');
 xhr.onload=()=>{let data;try{data=JSON.parse(xhr.responseText);}catch{finish(Error('The server returned an unreadable result. Refresh the destination before retrying.'));return;}finish(xhr.status>=200&&xhr.status<300?null:Error(data.error||'Copy failed'),data);};
 xhr.onerror=()=>finish(Error('Connection lost. The final result is unknown; refresh the destination before retrying.'));
 xhr.onabort=()=>{const error=Error('Cancelled. Check the destination before retrying if publication had already started.');error.name='AbortError';finish(error);};
 signal?.addEventListener('abort',abort,{once:true});if(signal?.aborted){finish(Object.assign(Error('Cancelled before sending'),{name:'AbortError'}));return;}xhr.send(form);
 });}
function createPlan(files,existing=[]){const names=new Set(existing.map(item=>String(item.name).toLocaleLowerCase()));return validateFiles(files).map(file=>({file,name:file.name,collision:names.has(file.name.toLocaleLowerCase()),choice:names.has(file.name.toLocaleLowerCase())?'skip':'keep',status:'Waiting',progress:0}));}
function validatePlan(plan){const names=new Set();for(const row of plan){if(!['skip','keep','overwrite','rename'].includes(row.choice))throw Error('Choose a collision action for each file.');if(row.choice==='skip')continue;if(!validName(row.name)||/\.(lnk|url)$/i.test(row.name))throw Error('Enter a valid filename for '+row.file.name+'.');const key=row.name.toLocaleLowerCase();if(names.has(key))throw Error('Two selected files have the same destination name. Rename or skip one.');names.add(key);if(row.choice==='rename'&&row.name===row.file.name)throw Error('Enter a different name or choose Keep name.');}return plan;}
async function execute(plan,options){validatePlan(plan);const report={copied:0,failed:0,skipped:0,cancelled:0,uncertain:0,rows:[]};
 for(let index=0;index<plan.length;index++) {const row=plan[index];if(row.choice==='skip'){row.status='Skipped';report.skipped++;options.update?.();continue;}
  if(options.signal.aborted){row.status='Cancelled before sending';report.cancelled++;options.update?.();continue;}
  try{options.check();row.status='Sending';options.update?.();await options.send(row,(loaded,total,phase)=>{row.progress=total?Math.min(100,Math.round(loaded/total*100)):0;row.status=phase==='finishing'?'Finishing at destination (not yet confirmed)':('Sending '+row.progress+'%');options.update?.();},options.signal);row.status='Copied';row.progress=100;report.copied++;}
  catch(error){row.status=error.message;if(error.name==='AbortError'){report.cancelled++;report.uncertain++;}else report.failed++;}
  report.rows.push({name:row.name,status:row.status});options.update?.();
 }
 return report;
}
let active=null;
function show(options){
 if(active?.running){root.toast?.('Finish or cancel the current file copy first.','info');return false;}
 let plan;try{options.check();plan=createPlan(options.files,options.entries);}catch(error){root.toast?.(error.message,'error');return false;}
 const job={options,plan,running:false,controller:new AbortController()};active=job;
 const body='<section class="transfer-drop-dialog" data-transfer-drop-dialog><p><strong>'+safe(options.remote?'Upload to remote server':'Copy into local shared-server folder')+'</strong></p><p class="transfer-drop-destination mono">'+safe(options.label)+'</p><p class="help-line">Original files stay unchanged. Folders and shortcuts are not supported. Maximum 100 files / 512 MiB each. '+(options.remote?'Data is sent only after you confirm below. Remote cancellation may leave a clearly named staging file if the server becomes unavailable.':'No file is published until its complete copy is received.')+'</p><div class="transfer-drop-list">'+plan.map((row,i)=>'<div class="transfer-drop-row" data-drop-row="'+i+'"><label><span>'+safe(row.file.name)+' · '+sizes(row.file.size)+(row.collision?' · already exists':'')+'</span><input data-drop-name="'+i+'" aria-label="Destination filename for '+safe(row.file.name)+'" value="'+safe(row.name)+'" disabled></label><select data-drop-choice="'+i+'" aria-label="Collision choice for '+safe(row.file.name)+'">'+[['keep','Keep name (never overwrite)'],['skip','Skip'],['overwrite','Overwrite explicitly'],['rename','Rename (never overwrite)']].map(([value,label])=>'<option value="'+value+'"'+(row.choice===value?' selected':'')+'>'+label+'</option>').join('')+'</select><progress data-drop-progress="'+i+'" max="100" value="0"></progress><span role="status" data-drop-status="'+i+'">Waiting</span></div>').join('')+'</div><p role="status" aria-live="polite" data-drop-summary>Review every destination and collision choice.</p></section>';
 root.openModal(options.remote?'Confirm remote upload':'Copy files to shared folder',body,'<button type="button" class="btn ghost" data-drop-cancel>Cancel</button><button type="button" class="btn primary" data-drop-start>'+(options.remote?'Confirm upload':'Copy files')+'</button>',true);return true;
}
function update(job){const dialog=document.querySelector('[data-transfer-drop-dialog]');if(!dialog||active!==job)return;job.plan.forEach((row,i)=>{const status=dialog.querySelector('[data-drop-status="'+i+'"]'),progress=dialog.querySelector('[data-drop-progress="'+i+'"]');if(status)status.textContent=row.status;if(progress)progress.value=row.progress;});}
async function start(){const job=active;if(!job||job.running)return;try{for(const [i,row]of job.plan.entries()){row.choice=document.querySelector('[data-drop-choice="'+i+'"]')?.value||row.choice;row.name=document.querySelector('[data-drop-name="'+i+'"]')?.value||row.name;}validatePlan(job.plan);job.options.check();}catch(error){root.toast?.(error.message,'error');return;}
 job.running=true;job.options.busy?.(true);const button=document.querySelector('[data-drop-start]');if(button)button.disabled=true;for(const input of document.querySelectorAll('[data-drop-choice],[data-drop-name]'))input.disabled=true;
 const cancel=document.querySelector('[data-drop-cancel]');if(cancel)cancel.textContent='Cancel remaining / current copy';
 let report;try{report=await execute(job.plan,{...job.options,signal:job.controller.signal,update:()=>update(job)});}finally{job.running=false;job.options.busy?.(false);}
 const summary=report.copied+' copied · '+report.skipped+' skipped · '+report.failed+' failed · '+report.cancelled+' cancelled'+(report.uncertain?' · '+report.uncertain+' final result(s) require destination refresh':'');
 const output=document.querySelector('[data-drop-summary]');if(output)output.textContent=summary;const done=document.querySelector('[data-drop-cancel]');if(done)done.textContent='Close';root.toast?.(summary,report.failed||report.cancelled?'error':'success');await job.options.done?.(report);
}
if(typeof document!=='undefined'){
 document.addEventListener('change',event=>{const select=event.target.closest?.('[data-drop-choice]');if(!select)return;const input=document.querySelector('[data-drop-name="'+select.dataset.dropChoice+'"]');if(input)input.disabled=select.value!=='rename';});
 document.addEventListener('click',event=>{if(event.target.closest?.('[data-drop-start]')){event.preventDefault();void start();}else if(event.target.closest?.('[data-drop-cancel]')){event.preventDefault();if(active?.running)active.controller.abort();else{active=null;root.closeModal();}}},true);
 // Escape / the global close button means cancellation, never detached copying.
 document.addEventListener('keydown',event=>{if(event.key==='Escape'&&active?.running&&document.querySelector('[data-transfer-drop-dialog]'))active.controller.abort();},true);
 document.addEventListener('click',event=>{if(event.target.closest?.('[data-action="close-modal"]')&&active?.running&&document.querySelector('[data-transfer-drop-dialog]'))active.controller.abort();},true);
}
root.NetworkToolkitTransferDrop=Object.freeze({validName,validateFiles,fromDrop,hasFiles,upload,createPlan,validatePlan,execute,show});
})(globalThis);
