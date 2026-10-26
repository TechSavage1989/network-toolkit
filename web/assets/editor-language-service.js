/* Bounded, local worker requests. Separate workers keep formatting independent
 * of live syntax painting. Timeout termination also bounds pathological regexes. */
(function(root){'use strict';
const slots=new Map();let sequence=0;
function terminate(action,error){const slot=slots.get(action);if(!slot)return;slots.delete(action);slot.worker.terminate();for(const pending of slot.pending.values()){root.clearTimeout(pending.timer);pending.reject(error);}slot.pending.clear();}
function request(action,text,language,options={}){
 if(typeof root.Worker!=='function')return Promise.reject(Error('Language worker is unavailable.'));
 let slot=slots.get(action);
 if(!slot){let worker;try{worker=new root.Worker('/assets/editor-language-worker.js');}catch(error){return Promise.reject(error);}slot={worker,pending:new Map()};slots.set(action,slot);
  worker.onmessage=event=>{const data=event.data||{},pending=slot.pending.get(data.id);if(!pending)return;slot.pending.delete(data.id);root.clearTimeout(pending.timer);if(data.error)pending.reject(Error(data.error));else pending.resolve(data);};
  worker.onerror=()=>terminate(action,Error('Language tools could not be loaded.'));
 }
 // Coalesce queued paints. Terminating a busy old highlight prevents its parser
 // from holding the newest file behind a costly, obsolete request.
 if(action==='highlight'&&slot.pending.size){terminate(action,Error('Superseded syntax request.'));return request(action,text,language,options);}
 const id=++sequence;return new Promise((resolve,reject)=>{const timer=root.setTimeout(()=>terminate(action,Error('Language operation timed out. The document was not changed.')),action==='format'?8000:2000);slot.pending.set(id,{resolve,reject,timer});try{slot.worker.postMessage({id,action,text,language,options});}catch(error){slot.pending.delete(id);root.clearTimeout(timer);reject(error);}});
}
root.NTKEditorLanguageService=Object.freeze({highlight:(text,id)=>request('highlight',text,id),format:(text,id,options)=>request('format',text,id,options),cancelHighlight:()=>terminate('highlight',Error('Syntax request cancelled.')),dispose:()=>{for(const action of [...slots.keys()])terminate(action,Error('Editor closed.'));}});
})(globalThis);
