// Test-only full-page command/geometry observer; not embedded in production.
(async()=>{'use strict';const errors=[],sleep=ms=>new Promise(r=>setTimeout(r,ms));addEventListener('error',e=>errors.push(e.message));addEventListener('unhandledrejection',e=>errors.push(String(e.reason?.message||e.reason)));
const wait=async(fn,label)=>{const end=Date.now()+18000;while(Date.now()<end){const value=fn();if(value)return value;await sleep(30);}throw Error('SFTP121 timeout: '+label);};
const ed=()=>NetworkToolkitFileEditor,win=()=>NetworkToolkitEditorWindows,browser=()=>NetworkToolkitRemoteBrowser,rect=e=>{const r=e.getBoundingClientRect();return {left:r.left,top:r.top,width:r.width,height:r.height};};
let original=null;
const fields=['uid','id','path','source','content','original','saved','dirty','undo','redo','view','compareView','frozen','saving','binding'];
function snapshot(){const host=document.querySelector('[data-ed-text]'),rows=[...document.querySelectorAll('[data-rb-rows="remote"] [data-rb-item]')];return {documents:[...ed().documents.values()].map(d=>Object.fromEntries(fields.map(k=>[k,d[k]]))),active:ed().active?.uid||'',window:win().state,startup:globalThis.__networkToolkitStartupState,nativeCapability:/^[a-f0-9]{64}$/.test(globalThis.__networkToolkitNativeSession||''),location:location.pathname,geometry:{width:innerWidth,height:innerHeight,editor:host?rect(host):null,tabs:rows.filter(r=>r.getClientRects().length).map(r=>({uid:r.dataset.rbItem,...rect(r.querySelector('td')||r)}))},errors:[...errors]};}
async function execute(c){
 if(c.action==='snapshot')return snapshot();
 if(c.action==='setup-sftp'){
  if(!ed().documents.size)await ed().newDocument();const d=ed().active,ta=document.querySelector('[data-ed-text]');ta.value='SYNTHETIC DIRTY SFTP121 '+(win().state.self?'CHILD':'MAIN');ta.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:ta.value}));ta.value+='\nSECOND EDIT';ta.dispatchEvent(new InputEvent('input',{bubbles:true,inputType:'insertText',data:'\nSECOND EDIT'}));ta.setSelectionRange(2,12);d.view={...(d.view||{}),start:2,end:12};original={doc:d,content:d.content,undo:JSON.stringify(d.undo)};
  const host=document.querySelector('[data-ed-files-sidebar]');if(host.hidden)document.querySelector('[data-editor-files-reopen]').click();NetworkToolkitEditorFiles.selectPanel('connections');
  const select=await wait(()=>{const s=document.querySelector('[data-rb-connection-source]');return s&&[...s.options].some(o=>o.value==='connection:'+c.destination)&&s;},'existing real loopback connection offered');select.value='connection:'+c.destination;select.dispatchEvent(new Event('change',{bubbles:true}));document.querySelector('[data-rb-action="selectSource"]').click();await wait(()=>browser().get('ssh').connected&&!browser().get('ssh').connecting,'retained SFTP connected');
  NetworkToolkitEditorFiles.selectPanel('files');document.querySelector('[data-editor-files-side="remote"]').click();await browser().load('ssh','remote','/',true);await wait(()=>document.querySelector('[data-rb-item="/one.txt"]'),'actual loopback listing');return snapshot();
 }
 if(c.action==='row'){
  NetworkToolkitEditorFiles.selectPanel('files');const row=await wait(()=>document.querySelector('[data-rb-item="'+CSS.escape(c.destination)+'"]'),'requested real row');row.scrollIntoView({block:'nearest'});await new Promise(r=>requestAnimationFrame(()=>requestAnimationFrame(r)));return snapshot();
 }
 if(c.action==='verify-dirty'){
  if(!original||ed().documents.get(original.doc.id)!==original.doc||original.doc.content!==original.content||!original.doc.dirty||JSON.stringify(original.doc.undo)!==original.undo)throw Error('File drag mutated pre-existing dirty document/undo');return snapshot();
 }
 if(c.action==='wait-open'){
  await wait(()=>[...ed().documents.values()].some(d=>d.path==='/one.txt'&&!d.loading&&d.content==='SYNTHETIC DIRECT121 FILE\n'),'actual SFTP read into a new editor document');return execute({action:'verify-dirty'});
 }
 throw Error('Unknown fixed native fixture command '+c.action);
}
const headers={'Content-Type':'application/json','X-Session-Token':globalThis.__networkToolkitNativeSession};const exchange=async body=>{const r=await fetch('/__native_editor_fixture',{method:'POST',headers,body:JSON.stringify(body)});if(!r.ok)throw Error('Fixture channel '+r.status);return r.json();};
try{await wait(()=>globalThis.__networkToolkitStartupState===1&&globalThis.NetworkToolkitFileEditor&&globalThis.NetworkToolkitEditorWindows,'startup');while(true){const c=await exchange({kind:'next'});if(!c.id)continue;let state,error;try{state=await execute(c);}catch(e){error=e.message;state=snapshot();}await exchange({kind:'result',id:c.id,state,error});}}catch(e){await exchange({kind:'fatal',error:e.message}).catch(()=>{});}
})();
