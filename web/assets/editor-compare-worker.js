/* Same-origin, local-only, bounded comparison worker.
   No file, credential, network, or app-state access. */
'use strict';
importScripts('editor-compare.js');
self.onmessage=event=>{
 const request=event.data;
 if(!request||!Number.isSafeInteger(request.id)||typeof request.left!=='string'||typeof request.right!=='string')return;
 try{
  const result=self.NTKEditorCompare.compare(request.left,request.right,request.spaces);
  self.postMessage({id:request.id,result});
 }catch{
  self.postMessage({id:request.id,error:'The comparison could not finish within the available resources. Both documents remain unchanged.'});
 }
};
