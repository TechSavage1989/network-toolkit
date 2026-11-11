'use strict';
// Unit checks for the document hand-over (editor-handover.js): which of the main window's tabs go to a
// running standalone Text Editor, that the source keeps every tab read-only until the editor confirms
// it and gives back every tab it did not confirm, and how the receiving editor opens and keeps them.
const fs=require('node:fs'),vm=require('node:vm'),path=require('node:path'),assert=require('node:assert/strict'),{test}=require('node:test'),{document:makeDocument}=require('./terminal-dom-fixture.cjs');
const asset=name=>fs.readFileSync(path.join(__dirname,'..','web','assets',name),'utf8');
const HASH='a'.repeat(64),OTHER='b'.repeat(64);

function load(options={}){
 const calls=[],toasts=[];
 const context={console,TextEncoder,setTimeout:(fn)=>setTimeout(fn,0),status:{windowMode:options.windowMode||'main'},toast:(message,kind)=>toasts.push([message,kind])};
 context.globalThis=context;
 context.api=async(route,method='GET',body)=>{calls.push([method,route,body]);const reply=await (options.engine||(()=>({})))(route,method,body,calls);if(reply instanceof Error)throw reply;return reply;};
 vm.runInNewContext(asset('editor-handover.js'),context);
 return {api:context.NetworkToolkitEditorHandover,calls,toasts,context};
}
const doc=(uid,extra={})=>({uid,path:'C:\\Users\\Example\\'+uid+'.txt',source:'Local computer',binding:{kind:'local'},hash:HASH,encoding:'UTF-8',bom:false,eol:'CRLF',savedEncoding:'UTF-8',savedBOM:false,savedEol:'CRLF',content:'text of '+uid,dirty:false,untitled:false,disk:false,language:'',...extra});
// A stand-in for file-editor.js's window adapter: freezing, release and rollback as the real one does.
function adapter(list){
 const log=[];
 return {log,list,documents:()=>list,flushDocuments:async ids=>log.push(['flush',ids]),canTransfer:ids=>ids.some(uid=>list.find(d=>d.uid===uid)?.binding?.kind==='ftp')?'This document cannot move.':'',
  snapshot(ids){for(const uid of ids)list.find(d=>d.uid===uid).frozen=true;log.push(['snapshot',ids]);return {documents:ids.map(uid=>({...list.find(d=>d.uid===uid)}))};},
  release(ids){for(const uid of ids){const d=list.find(d=>d.uid===uid);assert(d.frozen,'only frozen tabs are released');list.splice(list.indexOf(d),1);}log.push(['release',ids]);},
  rollback(ids){for(const uid of ids){const d=list.find(d=>d.uid===uid);assert(d.frozen,'only frozen tabs are given back');d.frozen=false;}log.push(['rollback',ids]);}};
}

test('which tabs can be handed over and what each carries',()=>{
 const {api}=load();
 assert.equal(api.reasonFor(doc('a')),'');
 assert.equal(api.reasonFor(doc('a',{dirty:true,content:'edited'})),'');
 assert.equal(api.reasonFor(doc('a',{binding:{kind:'terminal',sessionId:'s1'}})),'sftp');
 assert.equal(api.reasonFor(doc('a',{binding:{kind:'sshConnection',connectionId:'c1'}})),'sftp');
 assert.equal(api.reasonFor(doc('a',{binding:{kind:'ftp'}})),'notLocal');
 assert.equal(api.reasonFor(doc('a',{disk:true,encoding:'Binary'})),'binary');
 assert.equal(api.reasonFor(doc('a',{disk:true,dirty:true})),'large');
 assert.equal(api.reasonFor(doc('a',{disk:true,encoding:'Windows-1252',savedEncoding:'Windows-1252',eol:'Mixed',savedEol:'Mixed'})),'','a saved file is reopened from disk whatever its format');
 assert.equal(api.reasonFor(doc('a',{dirty:true,encoding:'Windows-1252'})),'format');
 assert.equal(api.reasonFor(doc('a',{dirty:true,eol:'Mixed'})),'format');
 assert.equal(api.reasonFor(doc('a',{path:'\\\\server\\share\\a.txt'})),'notLocal','UNC paths stay');
 assert.equal(api.reasonFor(doc('a',{path:'a.txt',source:'Desktop import',binding:{kind:'localDraft'}})),'notLocal');
 assert.equal(api.reasonFor(doc('a',{hash:''})),'unsaved');
 assert.equal(api.reasonFor(doc('a',{untitled:true,path:'Untitled.txt',source:'Untitled',binding:{kind:'localDraft'},hash:''})),'');
 assert.equal(api.reasonFor(doc('a',{untitled:true,path:'Untitled.txt',source:'Untitled',binding:{kind:'localDraft'},hash:'',disk:true})),'large');
 assert.deepEqual(JSON.parse(JSON.stringify(api.itemFor(doc('a',{language:'cisco'})))),{id:'a',kind:'file',path:'C:\\Users\\Example\\a.txt',hash:HASH,language:'cisco'},'a saved file carries no text');
 assert.deepEqual(JSON.parse(JSON.stringify(api.itemFor(doc('a',{dirty:true,content:'edited',eol:'LF'})))),{id:'a',kind:'file',path:'C:\\Users\\Example\\a.txt',hash:HASH,dirty:true,text:'edited',format:{encoding:'UTF-8',bom:false,eol:'LF'},savedFormat:{encoding:'UTF-8',bom:false,eol:'CRLF'}});
 assert.deepEqual(JSON.parse(JSON.stringify(api.itemFor(doc('a',{disk:true,dirty:false})))),{id:'a',kind:'file',path:'C:\\Users\\Example\\a.txt',hash:HASH});
 assert.deepEqual(JSON.parse(JSON.stringify(api.itemFor(doc('u',{untitled:true,path:'notes\u0007.txt',dirty:true,content:'draft',hash:'',binding:{kind:'localDraft'}})))),{id:'u',kind:'untitled',name:'notes .txt',dirty:true,text:'draft',format:{encoding:'UTF-8',bom:false,eol:'CRLF'},savedFormat:{encoding:'UTF-8',bom:false,eol:'CRLF'}});
});

test('only the main window hands over, and only to a running editor',async()=>{
 const child=load({windowMode:'editor-standalone'});const a=adapter([doc('a')]);
 assert.deepEqual(JSON.parse(JSON.stringify(await child.api.handOver(['a'],{adapter:a}))),{rest:['a'],message:''});assert.deepEqual(child.calls,[]);
 const none=load({engine:()=>({available:false,name:'Network Toolkit Text Editor'})});
 assert.deepEqual(JSON.parse(JSON.stringify(await none.api.handOver(['a'],{adapter:a}))),{rest:['a'],message:''});
 assert.deepEqual(none.calls.map(c=>c[0]+' '+c[1]),['GET /peer-handover/receiver']);assert.equal(a.list[0].frozen,undefined,'nothing was frozen');
 // Only remote tabs: nothing to ask.
 const remote=load();const r=adapter([doc('s',{binding:{kind:'terminal',sessionId:'s1'}})]);
 assert.deepEqual(JSON.parse(JSON.stringify(await remote.api.handOver(['s'],{adapter:r}))),{rest:['s'],message:''});assert.deepEqual(remote.calls,[]);
 // A tab that cannot move anywhere keeps today's answer for the whole move.
 const ftp=load({engine:()=>({available:true})});const f=adapter([doc('a'),doc('f',{binding:{kind:'ftp'}})]);
 assert.deepEqual(JSON.parse(JSON.stringify(await ftp.api.handOver(['a','f'],{adapter:f}))),{rest:['a','f'],message:''});assert.deepEqual(ftp.calls,[]);
});

test('confirmed tabs leave, unconfirmed tabs stay, remote tabs go to a connected window',async()=>{
 const list=[doc('a',{dirty:true,content:'edited'}),doc('b'),doc('u',{untitled:true,path:'Untitled.txt',binding:{kind:'localDraft'},hash:'',dirty:true,content:'draft'}),doc('s',{binding:{kind:'terminal',sessionId:'s1'}}),doc('c',{dirty:true})];
 const a=adapter(list);let polls=0;
 const h=load({engine(route,method,body){
  if(route==='/peer-handover/receiver')return {available:true,name:'Network Toolkit Text Editor'};
  if(route==='/peer-handover/start'){assert.deepEqual(JSON.parse(JSON.stringify(body.items.map(i=>i.id))),['a','b','u','c']);assert(list.filter(d=>['a','b','u','c'].includes(d.uid)).every(d=>d.frozen),'frozen before anything is sent');return {requestId:'1'.repeat(32),state:'queued',sent:['a','b','u'],ineligible:[{id:'c',reason:'c.txt changed on disk since it was opened'}],name:'Network Toolkit Text Editor'};}
  if(route.startsWith('/peer-handover/result')){polls++;
   // Until the editor confirms, every sent tab is still here and read-only.
   assert(list.filter(d=>['a','b','u'].includes(d.uid)).every(d=>d.frozen));
   return polls<2?{state:'taken'}:{state:'done',items:[{id:'a',ok:true},{id:'b',ok:true},{id:'u',ok:false,error:'A dialog is open in Network Toolkit Text Editor.'}]};}
  return new Error('unexpected '+route);}});
 const result=await h.api.handOver(['a','b','u','s','c'],{adapter:a});
 assert.deepEqual(JSON.parse(JSON.stringify(result.rest)),['s','c'],'remote and changed tabs go to a connected window');
 assert.deepEqual(list.map(d=>[d.uid,!!d.frozen]),[['u',false],['s',false],['c',false]],'confirmed tabs left; the rest are editable again');
 assert.equal(result.message,'2 tabs moved to the open Network Toolkit Text Editor; 2 tabs that need this window open in a connected window; 1 tab stayed here: A dialog is open in Network Toolkit Text Editor.');
 assert.deepEqual(h.toasts.at(-1)[1],'error');
 const sent=h.calls.find(c=>c[1]==='/peer-handover/start')[2].items;
 assert.equal(sent.find(i=>i.id==='a').text,'edited');assert.equal(sent.find(i=>i.id==='b').text,undefined,'a saved file carries no text');
});

test('one reason is named in plain words',async()=>{
 const list=[doc('a'),doc('b'),doc('c'),doc('s',{binding:{kind:'sshConnection',connectionId:'c1'}})];
 const h=load({engine(route){if(route==='/peer-handover/receiver')return {available:true,name:'Network Toolkit Text Editor'};if(route==='/peer-handover/start')return {requestId:'2'.repeat(32),sent:['a','b','c'],ineligible:[]};return {state:'done',items:['a','b','c'].map(id=>({id,ok:true}))};}});
 const result=await h.api.handOver(['a','b','c','s'],{adapter:adapter(list)});
 assert.equal(result.message,'3 tabs moved to the open Network Toolkit Text Editor; 1 tab that uses an SFTP connection opens in a connected window.');
 assert.deepEqual(h.toasts.at(-1),[result.message,'success']);
});

for(const [name,engine,why] of [
 ['a refused start',route=>route==='/peer-handover/start'?Object.assign(Error('Network Toolkit Text Editor is not open or not ready. The tabs stay here.'),{status:503}):{available:true},'not open or not ready'],
 ['a refusal by the editor',route=>route==='/peer-handover/start'?{requestId:'3'.repeat(32),sent:['a','b']}:route.startsWith('/peer-handover/result')?{state:'failed',message:'A dialog is open in Network Toolkit Text Editor.'}:{available:true},'A dialog is open'],
 ['a timeout',route=>route==='/peer-handover/start'?{requestId:'4'.repeat(32),sent:['a','b']}:route.startsWith('/peer-handover/result')?{state:'expired',message:'Network Toolkit Text Editor did not confirm the tabs in time.'}:{available:true},'did not confirm'],
 ['an editor that closed',route=>route==='/peer-handover/start'?{requestId:'5'.repeat(32),sent:['a','b']}:route.startsWith('/peer-handover/result')?{state:'lost',message:'Network Toolkit Text Editor closed or stopped answering before it confirmed the tabs.'}:{available:true},'closed'],
 ['an unknown request',route=>route==='/peer-handover/start'?{requestId:'6'.repeat(32),sent:['a','b']}:route.startsWith('/peer-handover/result')?Object.assign(Error('That hand-over is no longer known.'),{status:404}):{available:true},'no longer known'],
 ['a confirmation without items',route=>route==='/peer-handover/start'?{requestId:'7'.repeat(32),sent:['a','b']}:route.startsWith('/peer-handover/result')?{state:'done'}:{available:true},'stayed here'],
]){
 test('the source keeps every tab after '+name,async()=>{
  const list=[doc('a',{dirty:true,content:'edited'}),doc('b')],before=JSON.stringify(list),a=adapter(list),h=load({engine});
  const result=await h.api.handOver(['a','b'],{adapter:a});
  assert.deepEqual(JSON.parse(JSON.stringify(result.rest)),[],'nothing goes to a connected window either: the tabs stay');
  assert(list.every(d=>d.frozen===false),'every tab is editable again');
  assert.deepEqual(list.map(({frozen,...rest})=>rest),JSON.parse(before),'tabs unchanged');
  assert(!a.log.some(entry=>entry[0]==='release'),'nothing released');
  assert.match(result.message,/2 tabs stayed here/);assert.match(result.message,new RegExp(why));
 });
}

// ---- Receiving ----
test('the receiving editor opens each tab, keeps unsaved text and confirms per tab',async()=>{
 // editor-handover.js reads NetworkToolkitFileEditor from its own global object.
 const ctx=load({windowMode:'editor-standalone'}).context;
 const opened=[],discarded=[],activated=[];let saves=0;
 ctx.NetworkToolkitFileEditor={session:{phase:'ready',save:async()=>{saves++;},isKept:d=>d.uid!=='r-c'},async adoptHandover(item){if(item.fail)throw Error(item.fail);opened.push(item.id);return {id:'local:'+item.id,uid:'r-'+item.id,dirty:!!item.dirty,handoverAdopted:true};},discardAdopted:d=>{discarded.push(d.id);return true;},activateDocument:id=>activated.push(id)};
 const api=ctx.NetworkToolkitEditorHandover;
 const result=await api.receive({requestId:'1'.repeat(32),documents:[{id:'x',fail:'notes.txt changed on disk since it was opened.'},{id:'a',dirty:true},{id:'b'},{id:'c',dirty:true}]},{guard:()=>''});
 assert.deepEqual(JSON.parse(JSON.stringify(result)),{ok:true,outcome:'opened',items:[{id:'x',ok:false,error:'notes.txt changed on disk since it was opened.'},{id:'a',ok:true},{id:'b',ok:true},{id:'c',ok:false,error:'Its unsaved text could not be kept for next time.'}]});
 assert.equal(saves,1,'the session is kept before the confirmation');assert.deepEqual(discarded,['local:c']);assert.deepEqual(activated,['local:a'],'the first confirmed tab is active');
 // A dialog: nothing opens.
 opened.length=0;
 assert.deepEqual(JSON.parse(JSON.stringify(await api.receive({requestId:'2'.repeat(32),documents:[{id:'a'}]},{guard:()=>'A dialog is open in Network Toolkit Text Editor.'}))),{ok:false,error:'A dialog is open in Network Toolkit Text Editor.'});assert.deepEqual(opened,[]);
 // A dialog that opens while tabs are opening stops the rest.
 let n=0;const partial=await api.receive({requestId:'3'.repeat(32),documents:[{id:'a'},{id:'b'}]},{guard:()=>n++>=2?'A dialog is open.':''});
 assert.deepEqual(JSON.parse(JSON.stringify(partial.items)),[{id:'a',ok:true},{id:'b',ok:false,error:'A dialog is open.'}]);
 // Session still reopening: unsaved text is not confirmed; "Reopen files next time" off: held like any tab.
 ctx.NetworkToolkitFileEditor.session.phase='restoring';discarded.length=0;
 assert.equal((await api.receive({requestId:'4'.repeat(32),documents:[{id:'d',dirty:true},{id:'e'}]},{guard:()=>''})).items[0].ok,false);assert.deepEqual(discarded,['local:d']);
 ctx.NetworkToolkitFileEditor.session.phase='off';
 assert.equal((await api.receive({requestId:'5'.repeat(32),documents:[{id:'f',dirty:true}]},{guard:()=>''})).items[0].ok,true);
 ctx.NetworkToolkitFileEditor.session.phase='ready';ctx.NetworkToolkitFileEditor.session.save=async()=>{throw Error('disk full');};
 assert.match((await api.receive({requestId:'6'.repeat(32),documents:[{id:'g',dirty:true}]},{guard:()=>''})).items[0].error,/could not be kept for next time: disk full/);
});

test('peer-send.js gives documents to the hand-over in the standalone Text Editor only',async()=>{
 const run=mode=>{
  const document=makeDocument('<div id="modalRoot"></div><div id="toastStack"></div>');document.body=document;
  const acks=[],inbox=[],received=[],timers=[],window={document,setTimeout};
  window.api=async(route,method,body)=>{if(route==='/peer-send/inbox'){const next=inbox.shift();return next?{request:next,kind:'documents',report:false}:{request:null,report:false};}if(route==='/peer-send/ack')acks.push(body);return {ok:true};};
  window.NetworkToolkitEditorHandover={receive:async(request,options)=>{received.push([request.requestId,typeof options.guard]);return {ok:true,outcome:'opened',items:request.documents.map(d=>({id:d.id,ok:true}))};}};
  window.NetworkToolkitFileEditor={listDocuments:()=>[]};
  vm.runInNewContext(asset('peer-send.js'),{window,console,setTimeout,setInterval:(fn,ms)=>{timers.push(fn);return 1;},TextEncoder,online:true,shutDown:false,OFFLINE:false});
  return {api:window.NetworkToolkitPeerSend,inbox,acks,received,document};
 };
 const editor=run('editor');editor.api.start('editor');
 editor.inbox.push({requestId:'a'.repeat(32),text:'',documents:[{id:'d1',kind:'untitled'}]});await editor.api.poll();await editor.api.poll();
 assert.deepEqual(editor.received,[['a'.repeat(32),'function']]);
 assert.deepEqual(JSON.parse(JSON.stringify(editor.acks.at(-1))),{requestId:'a'.repeat(32),ok:true,outcome:'opened',items:[{id:'d1',ok:true}]});
 editor.document.querySelector('#modalRoot').innerHTML='<section class="modal"></section>';
 editor.inbox.push({requestId:'b'.repeat(32),text:'',documents:[{id:'d2',kind:'untitled'}]});await editor.api.poll();
 assert.equal(editor.received.length,1,'nothing opens behind a dialog');
 assert.deepEqual(JSON.parse(JSON.stringify(editor.acks.at(-1))),{requestId:'b'.repeat(32),ok:false,error:'A dialog is open in Network Toolkit Text Editor. Finish or close it there, then send again.'});
 for(const mode of ['terminal','main']){
  const other=run(mode);if(mode==='main')other.api.startMain();else other.api.start(mode);
  other.inbox.push({requestId:'c'.repeat(32),text:'',documents:[{id:'d3',kind:'untitled'}]});await other.api.poll();await other.api.poll();
  assert.deepEqual(other.received,[],mode+' never opens documents');assert.equal(other.acks.at(-1)?.ok,false);
 }
});
