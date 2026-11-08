/* Bounded renderer state for a disk-backed editor document. No full document
   strings or histories cross this interface. It also retains unacknowledged
   typing if a request fails; callers must not navigate away from that page. */
(function(root){'use strict';
const PAGE_UNITS=49152;
const countLines=text=>{let count=0,at=-1;while((at=text.indexOf('\n',at+1))>=0)count++;return count;};
const splitsPair=(text,at)=>at>0&&at<text.length&&text.charCodeAt(at-1)>=0xd800&&text.charCodeAt(at-1)<=0xdbff&&text.charCodeAt(at)>=0xdc00&&text.charCodeAt(at)<=0xdfff;
function fragment(before,after){
 let from=0,to=before.length,end=after.length;
 while(from<to&&from<end&&before.charCodeAt(from)===after.charCodeAt(from))from++;
 if(splitsPair(before,from)||splitsPair(after,from))from--;
 while(to>from&&end>from&&before.charCodeAt(to-1)===after.charCodeAt(end-1)){to--;end--;}
 if(splitsPair(before,to)||splitsPair(after,end)){to++;end++;}
 return {from,to,text:after.slice(from,end)};
}
function create({request,changed=()=>{},failed=()=>{},identity=()=>[...root.crypto.getRandomValues(new Uint8Array(16))].map(n=>n.toString(16).padStart(2,'0')).join('')}){
 let metadata={revision:0,units:0,bytes:0,lines:1,dirty:false,canUndo:false,canRedo:false};
 let page={start:0,line:0,column:0,text:''},acknowledged='',flushing=null,failure=null,closed=false,epoch=0,readTicket=0;
 let retry=null,external=null,scheduled=false;
 const cache=new Map();
 function notify(){changed(state());}
 function state(){return {metadata:{...metadata,units:metadata.units+page.text.length-acknowledged.length,lines:metadata.lines+countLines(page.text)-countLines(acknowledged),dirty:metadata.dirty||page.text!==acknowledged||!!external,canUndo:metadata.canUndo||page.text!==acknowledged},page:{...page},pending:!!flushing||!!retry||!!external||page.text!==acknowledged,blocked:!!external,error:failure};}
 function ensure(){if(closed)throw Error('The document buffer is no longer attached to this editor.');}
 function receive(value){if(!value||!Number.isSafeInteger(value.revision)||!Number.isSafeInteger(value.units)||!Number.isSafeInteger(value.lines))throw Error('The document engine returned invalid metadata.');metadata=Object.fromEntries(['revision','units','bytes','lines','dirty','canUndo','canRedo'].map(key=>[key,value[key]]));}
 async function mutate(payload){
  // The request ID and all payload fields stay identical across an uncertain
  // response. A definite rejection is never retried as a different edit.
  let error;
  for(let attempt=0;attempt<2;attempt++){try{return await request(payload.action,payload);}catch(e){error=e;if(e.status&&e.status<500)break;}}
  throw error;
 }
 function change(text){ensure();if(external)throw Error('Confirm the pending document change before typing.');if(typeof text!=='string')throw TypeError('Expected page text.');if(text===page.text)return;
  page={...page,text};epoch++;cache.clear();notify();
  if(!scheduled&&!failure){scheduled=true;queueMicrotask(()=>{scheduled=false;void flush().catch(()=>{});});}
 }
 async function drain(){
  if(external){const operation=external;
   if(!operation.applied){const result=await mutate(operation.payload);ensure();receive(result);operation.applied=true;retry=null;cache.clear();epoch++;}
   const value=await request('window',{start:Math.min(operation.target,metadata.units),limit:PAGE_UNITS,align:true});ensure();receive(value);page={start:value.start,line:value.line,column:value.column||0,lineUnits:value.lineUnits||0,text:value.text};acknowledged=page.text;external=null;failure=null;notify();
  }
  while(retry||page.text!==acknowledged){
   ensure();
   if(!retry){const edit=fragment(acknowledged,page.text);retry={sent:page.text,payload:{action:'edit',requestId:identity(),revision:metadata.revision,from:page.start+edit.from,to:page.start+edit.to,text:edit.text}};}
   const sent=retry,result=await mutate(sent.payload);ensure();receive(result);acknowledged=sent.sent;retry=null;failure=null;cache.clear();notify();
  }
 }
 function flush(){ensure();if(flushing)return flushing;
  flushing=drain().catch(error=>{failure=error;failed(error);throw error;}).finally(()=>{flushing=null;notify();});return flushing;
 }
 async function load(target={start:0},limit=PAGE_UNITS){
  ensure();const ticket=++readTicket;
  for(;;){await flush();ensure();const stamp=epoch,revision=metadata.revision,key=JSON.stringify([revision,target,limit]);
   const value=cache.get(key)||await request('window',{...target,limit,align:true});
   if(ticket!==readTicket||closed)return null;
   // Typing continues while a page is read. Never install a stale page over it.
   if(epoch!==stamp||page.text!==acknowledged||retry)continue;
   if(value.loading)throw Error('The file is still opening.');
   receive(value);page={start:value.start,line:value.line,column:value.column||0,lineUnits:value.lineUnits||0,text:value.text};acknowledged=page.text;failure=null;epoch++;notify();return {...page};
  }
 }
 async function prefetch(target,limit=PAGE_UNITS){
  ensure();if(page.text!==acknowledged||retry)return;const revision=metadata.revision,key=JSON.stringify([revision,target,limit]);if(cache.has(key))return;
  const value=await request('window',{...target,limit,align:true});
  if(!closed&&!value.loading&&value.revision===revision&&metadata.revision===revision&&page.text===acknowledged){cache.set(key,value);while(cache.size>3)cache.delete(cache.keys().next().value);}
 }
 async function replace(from,to,text){
  await flush();ensure();if(from>=page.start&&to<=page.start+page.text.length){change(page.text.slice(0,from-page.start)+text+page.text.slice(to-page.start));await flush();return;}
  // Offscreen ranges (including Select all) are edited by offsets, not fetched
  // into the renderer just to delete or replace them.
  external={payload:{action:'edit',requestId:identity(),revision:metadata.revision,from,to,text},target:from};notify();await flush();
 }
 async function history(redo=false){await flush();ensure();external={payload:{action:redo?'redo':'undo',requestId:identity(),revision:metadata.revision},target:page.start};notify();await flush();return {...page};}
 async function find(query,start,options={},signal){await flush();ensure();const revision=metadata.revision,result=await request('find',{query,start,...options},signal);if(result.revision!==revision||metadata.revision!==revision||page.text!==acknowledged)throw Error('The document changed during the search. Search again.');return result;}
 async function save(path,format,overwrite=false,signal){await flush();ensure();const result=await request('save',{revision:metadata.revision,path,format,overwrite},signal);ensure();
  // A save snapshots the acknowledged revision. Later editing may already
  // have updated metadata by the time its response arrives.
  if(result.metadata.revision===metadata.revision&&!retry)receive(result.metadata);
  notify();return result;
 }
 async function reload(signal){await flush();ensure();const result=await request('reload',{},signal);receive(result.metadata);cache.clear();acknowledged='';page={start:0,line:0,column:0,text:''};retry=null;failure=null;epoch++;await load();return result;}
 function initialize(value){ensure();receive(value);notify();}
 return Object.freeze({change,flush,load,prefetch,replace,history,find,save,reload,initialize,state,close(){closed=true;readTicket++;cache.clear();},get pending(){return !!flushing||!!retry||!!external||page.text!==acknowledged;}});
}
const api=Object.freeze({create,fragment,PAGE_UNITS});root.NTKEditorDiskSession=api;
if(typeof module==='object'&&module.exports)module.exports=api;
})(globalThis);
