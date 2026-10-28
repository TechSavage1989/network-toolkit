/* Bounded, local worker requests. Separate workers keep formatting independent
 * of live syntax painting. Timeout termination also bounds pathological regexes. */
(function(root){'use strict';
const slots=new Map(),channels=new Set(['default','compare-left','compare-right']);let sequence=0;
const channelOf=options=>channels.has(options?.channel)?options.channel:'default';
const slotKey=(action,options)=>action==='highlight'?'highlight:'+channelOf(options):action;
function terminate(key,error){const slot=slots.get(key);if(!slot)return;slots.delete(key);slot.worker.terminate();for(const pending of slot.pending.values()){root.clearTimeout(pending.timer);pending.reject(error);}slot.pending.clear();}
function request(action,text,language,options={}){
 const key=slotKey(action,options);
 if(typeof root.Worker!=='function')return Promise.reject(Error('Language worker is unavailable.'));
 let slot=slots.get(key);
 if(!slot){let worker;try{worker=new root.Worker('/assets/editor-language-worker.js');}catch(error){return Promise.reject(error);}slot={worker,pending:new Map()};slots.set(key,slot);
  worker.onmessage=event=>{const data=event.data||{},pending=slot.pending.get(data.id);if(!pending)return;slot.pending.delete(data.id);root.clearTimeout(pending.timer);if(data.error)pending.reject(Error(data.error));else pending.resolve(data);};
  worker.onerror=()=>{if(slots.get(key)===slot)terminate(key,Error('Language tools could not be loaded.'));};
 }
 // Coalesce queued paints. Terminating a busy old highlight prevents its parser
 // from holding the newest file behind a costly, obsolete request.
 if(action==='highlight'&&slot.pending.size){terminate(key,Error('Superseded syntax request.'));return request(action,text,language,options);}
 const id=++sequence;return new Promise((resolve,reject)=>{const timer=root.setTimeout(()=>{if(slots.get(key)===slot)terminate(key,Error('Language operation timed out. The document was not changed.'));},action==='format'?8000:2000);slot.pending.set(id,{resolve,reject,timer});try{slot.worker.postMessage({id,action,text,language,options});}catch(error){slot.pending.delete(id);root.clearTimeout(timer);reject(error);}});
}
root.NTKEditorLanguageService=Object.freeze({highlight:(text,id,options={})=>request('highlight',text,id,{channel:channelOf(options)}),format:(text,id,options)=>request('format',text,id,options),cancelHighlight:channel=>terminate(slotKey('highlight',{channel}),Error('Syntax request cancelled.')),dispose:()=>{for(const key of [...slots.keys()])terminate(key,Error('Editor closed.'));}});
})(globalThis);
