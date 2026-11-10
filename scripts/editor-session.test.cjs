'use strict';
// Text Editor session restore: record building, background and close-time
// saves, restore ordering and the preference, through the real modules.
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const session=require('../web/assets/editor-session.js');
const settle=()=>new Promise(resolve=>setImmediate(resolve));
const waitFor=async(check,label)=>{for(let i=0;i<200;i++){if(check())return;await settle();}assert.fail('timed out: '+label);};
const hash=c=>c.repeat(64);
const local=(path,extra={})=>({id:'local:'+path,uid:'u-'+path,path,source:'Local computer',binding:{kind:'local'},hash:hash('a'),encoding:'UTF-8',bom:false,eol:'CRLF',savedEncoding:'UTF-8',savedBOM:false,savedEol:'CRLF',content:'text',dirty:false,editRevision:1,...extra});

test('records keep paths, formats and kept-copy needs, never bindings, connections or credentials',()=>{
 const docs=[
  local('C:\\work\\clean.txt',{language:'yaml'}),
  local('C:\\work\\dirty.txt',{dirty:true,eol:'LF',hash:'not-a-digest'}),
  {id:'untitled:1',uid:'u1',path:'Untitled.txt',source:'Untitled',untitled:true,binding:{kind:'localDraft'},content:'',dirty:false},
  {id:'untitled:2',uid:'u2',path:'Untitled.txt',source:'Untitled',untitled:true,binding:{kind:'localDraft'},content:'draft',dirty:true,encoding:'UTF-16LE',bom:true,eol:'CRLF'},
  {id:'sftp:clean',uid:'u3',path:'/etc/clean.conf',source:'SFTP files',binding:{kind:'sshConnection',connectionId:'secret-connection'},content:'x',dirty:false},
  {id:'sftp:dirty',uid:'u4',path:'/etc/router.cfg',source:'SFTP files',binding:{kind:'sshConnection',connectionId:'secret-connection'},password:'hunter2',content:'x',dirty:true,disk:true,diskMeta:{units:1}},
  {id:'terminal:x',uid:'u5',path:'/root/.profile',source:'SSH terminal',binding:{kind:'terminal',sessionId:'s',revision:3},content:'x',dirty:true},
  {id:'desktop:1',uid:'u6',path:'notes.txt',source:'Desktop',binding:{kind:'localDraft'},desktopHandle:{},content:'x',dirty:true},
  {id:'local:/saved-as.txt',uid:'u7',path:'/srv/saved-as.txt',source:'Local computer',binding:{kind:'localDraft'},disk:true,diskMeta:{units:4},dirty:false},
  {id:'staged',uid:'u8',path:'C:\\moving.txt',source:'Local computer',binding:{kind:'local'},staged:true,dirty:true},
  {id:'restored-copy',uid:'u9',path:'router.cfg',source:'Untitled',untitled:true,sessionOrigin:'SFTP files · /etc/router.cfg',binding:{kind:'localDraft'},content:'x',dirty:true}
 ];
 const rows=session.plan(docs),records=rows.map(row=>row.record);
 assert.deepEqual(records.map(r=>r.kind),['local','local','untitled','copy','copy','copy','local','copy']);
 assert.deepEqual(records[0],{kind:'local',path:'C:\\work\\clean.txt',language:'yaml',dirty:false});
 assert.deepEqual(records[1],{kind:'local',path:'C:\\work\\dirty.txt',dirty:true,hash:'',format:{encoding:'UTF-8',bom:false,eol:'LF'},savedFormat:{encoding:'UTF-8',bom:false,eol:'CRLF'}});
 assert.deepEqual(records[2],{kind:'untitled',name:'Untitled.txt',dirty:true,format:{encoding:'UTF-16LE',bom:true,eol:'CRLF'}});
 assert.deepEqual(records[3],{kind:'copy',name:'router.cfg',origin:'SFTP files · /etc/router.cfg',dirty:true,format:{encoding:'UTF-8',bom:false,eol:'LF'}});
 assert.equal(records[5].origin,'Desktop · notes.txt');
 assert.equal(records[7].origin,'SFTP files · /etc/router.cfg');
 assert.deepEqual(rows.filter(row=>row.keep).map(row=>row.d.uid),['C:\\work\\dirty.txt','u2','u4','u5','u6','u9'].map(x=>x.startsWith('C:')?'u-'+x:x));
 const serialized=JSON.stringify(records);
 for(const secret of ['binding','connectionId','secret-connection','sessionId','hunter2','password','desktopHandle'])assert(!serialized.includes(secret),secret);
});

function fakeHost(docs,overrides={}){
 const calls=[],h={docs,current:docs[0]?.id||'',calls,ready:'ready',failText:new Set(),
  request:async(path,method='GET',body)=>{calls.push({path,method,body:body&&JSON.parse(JSON.stringify(body))});
   if(path==='/editor-session/backup'&&method==='POST'){if(body.source==='text'&&h.failText.has(body.text))throw Object.assign(Error('Disk full'),{status:507});const id=(calls.length.toString(16)).padStart(32,'0');return {id,size:3+(body.text||'').length,sha256:hash('b'),revision:body.source==='buffer'?h.docs.find(d=>d.uid===body.uid).editRevision:0,elapsedMs:0};}
   if(path==='/editor-session'&&method==='GET')return h.session||{version:1,active:-1,tabs:[]};
   return {ok:true};},
  documents:()=>h.docs,activeId:()=>h.current,stamp:d=>[d.editRevision,d.eol,d.encoding,d.bom].join(':'),flush:async d=>{calls.push({flush:d.uid});},pending:()=>false,lease:d=>d.lease||7,enabled:()=>true,idle:()=>true,readiness:()=>h.ready,
  restoreRecord:record=>h.restoreRecord(record),finishRestore:value=>{h.finished=value;},message:(text,error)=>{h.messages.push({text,error});},messages:[],...overrides};
 return h;
}

test('saves send memory text or disk handles once, reuse current copies and track what is kept',async()=>{
 const memory=local('C:\\a.txt',{dirty:true,content:'unsaved memory text'}),disk=local('C:\\big.log',{uid:'disk-uid',dirty:true,disk:true,diskMeta:{units:9},editRevision:40}),clean=local('C:\\clean.txt');
 const host=fakeHost([memory,disk,clean]);host.current=disk.id;const keeper=session.create(host);
 await keeper.restore();host.calls.length=0;
 assert.equal(keeper.phase,'ready');
 let kept=await keeper.save({final:true});
 const posts=host.calls.filter(c=>c.path==='/editor-session/backup');
 assert.deepEqual(posts.map(c=>c.body),[{source:'text',text:'unsaved memory text'},{source:'buffer',uid:'disk-uid',lease:7}]);
 assert(host.calls.some(c=>c.flush==='disk-uid'),'pending disk edits are flushed before the snapshot');
 const put=host.calls.find(c=>c.method==='PUT').body;
 assert.equal(put.version,1);assert.equal(put.active,1);assert.deepEqual(put.tabs.map(t=>[t.kind,t.path,!!t.backup]),[['local','C:\\a.txt',true],['local','C:\\big.log',true],['local','C:\\clean.txt',false]]);
 assert.deepEqual([...kept].map(d=>d.path).sort(),['C:\\a.txt','C:\\big.log','C:\\clean.txt']);
 host.calls.length=0;kept=await keeper.save({final:true});
 assert.equal(host.calls.filter(c=>c.path==='/editor-session/backup').length,0,'unchanged documents reuse their copies');
 assert.equal(host.calls.filter(c=>c.method==='PUT').length,0,'an unchanged index is not rewritten');
 memory.content+='!';memory.editRevision++;
 assert.equal(keeper.isKept(memory),false,'a newer edit is not kept until the next save');
 assert.equal(keeper.tick(),true);await waitFor(()=>!keeper.busy,'background save');
 assert.deepEqual(host.calls.filter(c=>c.path==='/editor-session/backup').map(c=>c.body.text),['unsaved memory text!']);
 assert.equal(keeper.isKept(memory),true);
 assert.equal(keeper.tick(),false,'nothing changed since the last background save');
});

test('a copy that cannot be written keeps the previous copy in the index and is not kept for close',async()=>{
 const one=local('C:\\one.txt',{dirty:true,content:'first'}),two=local('C:\\two.txt',{dirty:true,content:'second'});
 const host=fakeHost([one,two]);const keeper=session.create(host);await keeper.restore();
 await keeper.save({final:true});const firstCopy=one.sessionBackup.ref.id;
 one.content='first, newer';one.editRevision++;host.failText.add('first, newer');host.calls.length=0;
 const kept=await keeper.save({final:true});
 assert.equal(kept.has(one),false);assert.equal(kept.has(two),true);
 assert.equal(host.calls.filter(c=>c.method==='PUT').length,0,'the index already names the older copy');
 assert.equal(one.sessionBackup.ref.id,firstCopy,'the older copy stays referenced so it is not deleted');
 assert.match(host.messages.at(-1).text,/Could not keep one\.txt/);
 host.request=async()=>{throw Error('Engine stopped');};one.editRevision++;one.eol='LF';
 await assert.rejects(keeper.save({final:true}),/Engine stopped/);
 assert.equal(keeper.isKept(one),false);
});

test('restore keeps stored order, carries limited records and leaves the active tab to files opened meanwhile',async()=>{
 const records=[{kind:'local',path:'C:\\a.txt',dirty:false},{kind:'untitled',name:'Untitled.txt',dirty:true,format:{encoding:'UTF-8',bom:false,eol:'LF'},backup:{id:'1'.repeat(32),size:4,sha256:hash('c')}},{kind:'copy',name:'x.cfg',origin:'SFTP files · /x.cfg',dirty:true,format:{encoding:'UTF-8',bom:false,eol:'LF'},backup:{id:'2'.repeat(32),size:4,sha256:hash('d')}}];
 const docs=[],host=fakeHost(docs);host.session={version:1,active:1,tabs:records,warning:''};
 host.restoreRecord=record=>{if(record.kind==='copy')return {carry:true,limit:true};const d=record.kind==='local'?local(record.path):{id:'untitled:1',uid:'u1',path:'Untitled.txt',source:'Untitled',untitled:true,binding:{kind:'localDraft'},content:'restored',dirty:true,editRevision:1,sessionBackup:{key:'memory:u1',revision:1,ref:record.backup}};docs.push(d);return Promise.resolve({doc:d,note:record.kind==='untitled'?'restored draft':''});};
 const keeper=session.create(host);assert.equal(await keeper.restore(),true);
 assert.deepEqual(host.finished.docs.map(d=>d.path),['C:\\a.txt','Untitled.txt']);assert.equal(host.finished.active.path,'Untitled.txt');
 assert.deepEqual(keeper.carried.map(r=>r.kind),['copy']);
 assert.match(host.messages.at(-1).text,/restored draft.*1 unsaved tab\(s\).*fewer than 12/);
 host.calls.length=0;await keeper.save();
 assert.deepEqual(host.calls.find(c=>c.method==='PUT').body.tabs.map(t=>t.kind),['local','untitled','copy'],'carried records stay in the session');
 const busy=[{id:'explorer',uid:'e',path:'C:\\explorer.txt',dirty:false,editRevision:1}],other=fakeHost(busy);other.session={version:1,active:0,tabs:[records[0]]};
 other.restoreRecord=()=>{const d={id:'a',uid:'a',path:'C:\\a.txt',editRevision:1};busy.push(d);return {doc:d};};
 await session.create(other).restore();assert.equal(other.finished.active,null,'a file opened from Explorer keeps focus');
 const failing=fakeHost([]);failing.request=async()=>{throw Error('offline');};const stopped=session.create(failing);
 assert.equal(await stopped.restore(),false);assert.equal(stopped.phase,'failed');assert.equal(stopped.tick(),false,'a session that could not be read is never overwritten');
});

test('restore waits for settings, honours the preference and never runs outside primary workspaces',async()=>{
 const host=fakeHost([]);host.ready='never';const never=session.create(host);never.start();assert.equal(never.phase,'unavailable');
 host.ready='off';const off=session.create(host);off.start();assert.equal(off.phase,'off');assert.equal(off.tick(),false);
 off.enable();assert.equal(off.phase,'ready');await off.disable();assert.equal(off.phase,'off');assert(host.calls.some(c=>c.method==='DELETE'));
 host.ready='ready';const keeper=session.create(host);keeper.start();await settle();assert.equal(keeper.phase,'ready');keeper.stop();assert.equal(keeper.phase,'closed');assert.equal(keeper.tick(),false);
});

// ---- Integration through the real file-editor.js in memory mode ----
const editorSource=fs.readFileSync(require.resolve('../web/assets/file-editor.js'),'utf8');
function harness({preferences={},stored={version:1,active:-1,tabs:[]},files={},failBackup=()=>false}={}){
 const document=makeDocument('');document.body=document;document.body.appendChild=document.body.append;
 const create=document.createElement;document.createElement=tag=>{const el=create(tag);el.style.setProperty=(k,v)=>el.style[k]=v;el.style.removeProperty=k=>delete el.style[k];return el;};
 const calls=[],kept=new Map(),index={value:stored};
 const api=async(path,method='GET',body)=>{
  calls.push({path,method,body:body&&JSON.parse(JSON.stringify(body))});
  if(path==='/file-editor/local'&&body.action==='read-text'){if(!(body.path in files))throw Object.assign(Error('The system cannot find the file specified.'),{status:400});return {content:files[body.path].content,hash:files[body.path].hash,encoding:'UTF-8',bom:false};}
  if(path==='/editor-session'&&method==='GET')return index.value;
  if(path==='/editor-session'&&method==='PUT'){index.value=JSON.parse(JSON.stringify(body));return {ok:true};}
  if(path==='/editor-session'&&method==='DELETE'){index.value={version:1,active:-1,tabs:[]};kept.clear();return {ok:true};}
  if(path==='/editor-session/backup'&&method==='POST'){if(failBackup(body))throw Object.assign(Error('Not enough disk space'),{status:507});const id=String(kept.size+1).padStart(32,'0');kept.set(id,body.text);return {id,size:body.text.length+3,sha256:'e'.repeat(64),revision:0,elapsedMs:0};}
  if(path.startsWith('/editor-session/backup?id=')){const id=decodeURIComponent(path.split('=')[1]);if(!kept.has(id))throw Object.assign(Error('The kept copy of this document is no longer available.'),{status:404});return {text:kept.get(id)};}
  throw Error('Unexpected '+method+' '+path);
 };
 const c={document,console,Map,Set,WeakMap,Date,Math,JSON,Promise,TextDecoder,TextEncoder,Uint8Array,DataView,Blob,URL,setTimeout,clearTimeout,state:{editorPreferences:preferences},saveState(){c.saved++;},saved:0,esc:x=>String(x).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),addEventListener(){},OFFLINE:false,online:true,stateLoaded:true,status:{editorSessionAvailable:true},api};
 vm.createContext(c);
 for(const file of ['editor-tab-groups.js','editor-session.js'])vm.runInContext(fs.readFileSync(require.resolve('../web/assets/'+file),'utf8'),c);
 vm.runInContext(editorSource,c);
 const editor=c.NetworkToolkitFileEditor,root=()=>document.getElementById('networkToolkitFileEditor');
 const mount=()=>{const node=document.createElement('div');document.append(node);editor.renderPage(node);};
 const type=(d,text)=>{editor.activateDocument(d.id);const ta=root().querySelector('[data-ed-text]');ta.value=text;root().emit('input',ta);};
 return {c,editor,calls,kept,index,root,mount,type};
}
test('app close keeps dirty tabs without asking, then still asks for a document that could not be kept',async()=>{
 const files={'C:\\work\\notes.txt':{content:'saved line\n',hash:hash('1')}};
 const h=harness({files});h.mount();await waitFor(()=>h.editor.session.phase==='ready','restore');
 const notes=await h.editor.openLocalPath('C:\\work\\notes.txt');h.type(notes,'saved line\nunsaved line\n');
 const draft=await h.editor.newDocument();h.type(draft,'draft text');
 await h.editor.newDocument();// blank draft: nothing to keep
 const closing=h.editor.windowAdapter.prepareWindowClose();
 assert.equal(await closing,true,'every unsaved change was kept, so close proceeds without a prompt');
 assert.equal(h.root().querySelector('.nt-editor-dialog'),null);
 const saved=h.index.value;
 assert.deepEqual(saved.tabs.map(t=>t.kind),['local','untitled']);
 assert.equal(saved.tabs[0].hash,hash('1'));assert.equal(saved.tabs[0].dirty,true);
 assert.deepEqual([...h.kept.values()],['saved line\nunsaved line\n','draft text']);
 assert(!JSON.stringify(saved).includes('unsaved line'),'the index never holds document text');
 h.editor.windowAdapter.cancelWindowClose();
 const failing=harness({files,failBackup:body=>body.text==='second draft'});failing.mount();await waitFor(()=>failing.editor.session.phase==='ready','restore');
 const one=await failing.editor.newDocument();failing.type(one,'first draft');const two=await failing.editor.newDocument();failing.type(two,'second draft');
 const pending=failing.editor.windowAdapter.prepareWindowClose();
 await waitFor(()=>failing.root().querySelector('.nt-editor-dialog'),'close prompt');
 assert.match(failing.root().querySelector('.nt-editor-dialog').textContent,/Discard unsaved changes in 1 file\(s\) when this window closes\?/);
 failing.root().emit('click',failing.root().querySelector('[data-ed-dialog-action="cancel"]'));
 assert.equal(await pending,false);assert.equal(failing.editor.documents.size,2);assert.equal(one.frozen,false);
});

test('the preference defaults on, survives as a saved setting and turning it off removes kept copies and restores today\'s prompt',async()=>{
 const h=harness();h.mount();await waitFor(()=>h.editor.session.phase==='ready','restore');
 const box=h.root().querySelector('[data-ed-restore-session]');assert.equal(box.checked,true);assert.equal(box.disabled,false);
 assert.match(h.root().querySelector('#ntEditorRestoreHint').textContent,/kept in this app.s data folder until it is saved or its tab is closed/);
 const draft=await h.editor.newDocument();h.type(draft,'keep me');await h.editor.session.save();assert.equal(h.kept.size,1);
 box.checked=false;h.root().emit('change',box);await waitFor(()=>h.calls.some(c=>c.method==='DELETE'),'disable');await settle();
 assert.equal(h.c.state.editorPreferences.restoreSession,false);assert(h.c.saved>0);assert(h.calls.some(c=>c.method==='DELETE'&&c.path==='/editor-session'));assert.equal(h.kept.size,0);
 const pending=h.editor.windowAdapter.prepareWindowClose();
 assert(h.root().querySelector('.nt-editor-dialog'),'with restore off the existing prompt appears as before');
 h.root().emit('click',h.root().querySelector('[data-ed-dialog-action="cancel"]'));assert.equal(await pending,false);
 assert.equal(h.calls.filter(c=>c.method==='PUT').length,1,'no session is written while restore is off');
 const off=harness({preferences:{restoreSession:false},stored:{version:1,active:0,tabs:[{kind:'untitled',name:'Untitled.txt',dirty:true,format:{encoding:'UTF-8',bom:false,eol:'LF'},backup:{id:'1'.repeat(32),size:4,sha256:hash('c')}}]}});
 off.mount();await settle();assert.equal(off.editor.session.phase,'off');assert.equal(off.editor.documents.size,0);assert.equal(off.root().querySelector('[data-ed-restore-session]').checked,false);
 assert(!off.calls.some(c=>c.path==='/editor-session'),'a disabled session is never read');
 box.checked=true;h.root().emit('change',box);await settle();assert.equal(h.c.state.editorPreferences.restoreSession,true);assert.equal(h.editor.session.phase,'ready');
});

test('restoring reopens tabs in order with dirty text, reports a changed file and falls back for a missing one',async()=>{
 const files={'C:\\work\\clean.txt':{content:'clean file\n',hash:hash('1')},'C:\\work\\edited.txt':{content:'changed by someone else\n',hash:hash('9')}};
 const h=harness({files});
 h.kept.set('1'.repeat(32),'my unsaved edit\n');h.kept.set('2'.repeat(32),'draft body');h.kept.set('3'.repeat(32),'text for a deleted file');h.kept.set('4'.repeat(32),'remote unsaved');
 const fmt={encoding:'UTF-8',bom:false,eol:'LF'},backup=n=>({id:String(n).repeat(32),size:9,sha256:hash('f')});
 h.index.value={version:1,active:1,tabs:[
  {kind:'local',path:'C:\\work\\clean.txt',dirty:false,language:'yaml',exists:true},
  {kind:'local',path:'C:\\work\\edited.txt',dirty:true,hash:hash('1'),format:{encoding:'UTF-8',bom:false,eol:'CRLF'},savedFormat:fmt,backup:backup(1),exists:true},
  {kind:'untitled',name:'Untitled.txt',dirty:true,format:fmt,backup:backup(2)},
  {kind:'local',path:'C:\\work\\deleted.txt',dirty:true,hash:hash('2'),format:fmt,savedFormat:fmt,backup:backup(3),exists:false},
  {kind:'copy',name:'router.cfg',origin:'SFTP files · /etc/router.cfg',dirty:true,format:fmt,backup:backup(4)},
  {kind:'local',path:'C:\\work\\vanished-clean.txt',dirty:false,exists:false}]};
 h.mount();await waitFor(()=>h.editor.session.phase==='ready','restore');
 const docs=[...h.editor.documents.values()];
 assert.deepEqual(docs.map(d=>d.path),['C:\\work\\clean.txt','C:\\work\\edited.txt','Untitled.txt','deleted.txt','router.cfg']);
 assert.deepEqual(docs.map(d=>!!d.dirty),[false,true,true,true,true]);
 assert.deepEqual(docs.map(d=>d.content),['clean file\n','my unsaved edit\n','draft body','text for a deleted file','remote unsaved']);
 assert.equal(h.editor.active.path,'C:\\work\\edited.txt','the stored active tab is selected');
 assert.equal(docs[0].language,'yaml');
 const edited=docs[1];assert.equal(edited.hash,hash('1'),'saving still checks the file revision from the last session');assert.equal(edited.eol,'CRLF');assert.equal(edited.savedEol,'LF');
 assert.deepEqual(Array.from(edited.undo),['changed by someone else\n'],'Undo returns to the file now on disk');
 assert.equal(docs[3].untitled,true);assert.equal(docs[4].untitled,true);assert.equal(docs[4].sessionOrigin,'SFTP files · /etc/router.cfg');
 const message=h.root().querySelector('[data-ed-message]').textContent;
 assert.match(message,/edited\.txt changed on disk since your last session\. Your unsaved version is shown\./);
 assert.match(message,/deleted\.txt no longer exists\. Your unsaved text was restored into a new untitled tab/);
 assert.match(message,/Restored unsaved text from SFTP files · \/etc\/router\.cfg\. Reconnect and use Save As to save it there\./);
 assert.match(message,/vanished-clean\.txt is no longer on disk/);
 // Restored tabs reuse their kept copies and the next save keeps them as copies.
 const posts=h.calls.filter(c=>c.path==='/editor-session/backup'&&c.method==='POST').length;
 await h.editor.session.save();
 assert.equal(h.calls.filter(c=>c.path==='/editor-session/backup'&&c.method==='POST').length,posts,'unchanged restored text is not copied again');
 assert.deepEqual(h.index.value.tabs.map(t=>t.kind),['local','local','untitled','untitled','copy']);
 assert.equal(h.index.value.tabs[4].origin,'SFTP files · /etc/router.cfg');
});
