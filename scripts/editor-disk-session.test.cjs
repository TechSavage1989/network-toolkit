'use strict';
const test=require('node:test'),assert=require('node:assert/strict');
const {create,fragment,PAGE_UNITS}=require('../web/assets/editor-disk-session.js');
const deferred=()=>{let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b;});return {promise,resolve,reject};};
function fixture(text='one\nλ😀\nlast'){
 let content=text,revision=1,saved=text,counter=0,undo=[],redo=[],calls=[],receipts=new Map();
 const metadata=()=>({revision,units:content.length,bytes:Buffer.byteLength(content),lines:content.split('\n').length,dirty:content!==saved,canUndo:!!undo.length,canRedo:!!redo.length});
 const request=async(action,p)=>{calls.push({action,...p});
  if(action==='window'){const start=p.line===undefined?p.start||0:content.split('\n').slice(0,p.line).reduce((n,s)=>n+s.length+1,0);return {...metadata(),start,line:content.slice(0,start).split('\n').length-1,text:content.slice(start,start+p.limit)};}
  if(action==='status')return {metadata:metadata()};
  if(['edit','undo','redo'].includes(action)){
   if(receipts.has(p.requestId))return receipts.get(p.requestId);
   if(p.revision!==revision)throw Object.assign(Error('revision'),{status:409});
   if(action==='edit'){undo.push(content);redo=[];content=content.slice(0,p.from)+p.text+content.slice(p.to);}
   else{const from=action==='undo'?undo:redo,to=action==='undo'?redo:undo;to.push(content);content=from.pop();}
   revision++;const result=metadata();receipts.set(p.requestId,result);return result;
  }
  if(action==='save'){saved=content;return {metadata:metadata(),path:p.path,hash:'saved'};}
  throw Error(action);
 };
 const session=create({request:(...args)=>fixtureRequest(...args),identity:()=>String(++counter)});
 let fixtureRequest=request;
 session.initialize(metadata());
 return {session,request,calls,metadata,intercept(fn){fixtureRequest=fn;},get text(){return content;}};
}
test('fragments preserve UTF-16 pairs at both changed boundaries',()=>{
 for(const [a,b] of [['a😀z','a😃z'],['😀😀','😃😀'],['a😀','a'],['λ😀a','λ😀b'],['abc','abc']]){
  const d=fragment(a,b);assert.equal(a.slice(0,d.from)+d.text+a.slice(d.to),b);assert.equal(d.text.isWellFormed(),true);
  assert.equal(a.slice(0,d.from).isWellFormed(),true);assert.equal(a.slice(d.to).isWellFormed(),true);
 }
});
test('typing during acknowledgement is retained and coalesced into bounded edits',async()=>{
 const f=fixture();await f.session.load();const held=deferred();let first=true;
 f.intercept(async(action,p)=>{const result=await f.request(action,p);if(action==='edit'&&first){first=false;await held.promise;}return result;});
 f.session.change('A'+f.text);const flushing=f.session.flush();
 f.session.change('AB'+f.text.slice(1));f.session.change('ABC'+f.text.slice(1));
 held.resolve();await flushing;
 assert.equal(f.text,'ABCone\nλ😀\nlast');assert.equal(f.session.state().page.text,f.text);assert.equal(f.session.pending,false);
 assert.equal(f.calls.filter(c=>c.action==='edit').length,2);
});
test('lost responses retry the same request exactly, without duplicate insertions',async()=>{
 const f=fixture('abc');await f.session.load();let lost=true;
 f.intercept(async(action,p)=>{const result=await f.request(action,p);if(action==='edit'&&lost){lost=false;throw Error('connection ended');}return result;});
 f.session.change('aXbc');await f.session.flush();assert.equal(f.text,'aXbc');
 const edits=f.calls.filter(c=>c.action==='edit');assert.equal(edits.length,2);assert.deepEqual(edits[0],edits[1]);
});
test('failed edits keep local text and block navigation until acknowledgement',async()=>{
 const f=fixture();await f.session.load();f.intercept(async()=>{throw Object.assign(Error('closed transport'),{status:409});});
 f.session.change('unsent typing');await assert.rejects(f.session.flush(),/closed transport/);
 await assert.rejects(f.session.load({start:0}),/closed transport/);assert.equal(f.session.state().page.text,'unsent typing');assert.equal(f.session.pending,true);
});
test('a late viewport response cannot overwrite typing',async()=>{
 const f=fixture('abcdef');await f.session.load({start:0},3);const held=deferred();let hold=true;
 f.intercept(async(action,p)=>{const result=await f.request(action,p);if(action==='window'&&hold){hold=false;await held.promise;}return result;});
 const loading=f.session.load({start:3},3);await new Promise(setImmediate);
 f.session.change('abX');await f.session.flush();held.resolve();await loading;
 assert.equal(f.text,'abXdef');assert.equal(f.session.state().page.text,'def');
});
test('a viewport awaiting layout keeps the previous page editable and retries after typing',async()=>{
 const f=fixture('abcdef'), started=deferred(), ready=deferred();
 await f.session.load({start:0},3);
 let preparations=0;
 const loading=f.session.load({start:3},3,async page=>{
  assert.equal(page.text,'def');
  if(++preparations===1){started.resolve();await ready.promise;}
 });
 await started.promise;
 assert.equal(f.session.state().page.text,'abc');
 f.session.change('abX');await f.session.flush();
 ready.resolve();await loading;
 assert.equal(preparations,2);
 assert.equal(f.text,'abXdef');assert.equal(f.session.state().page.text,'def');
});
test('history and saving operate on the document rather than its current page',async()=>{
 const f=fixture('prefix\nmiddle\nsuffix');await f.session.load({line:1},6);f.session.change('change');await f.session.flush();
 assert.equal(f.text,'prefix\nchange\nsuffix');await f.session.history();assert.equal(f.text,'prefix\nmiddle\nsuffix');await f.session.history(true);
 await f.session.save('fixture.txt',{encoding:'UTF-8',eol:'LF',bom:false});assert.equal(f.session.state().metadata.dirty,false);
});
test('multi-gigabyte offsets never allocate the full document in the renderer',async()=>{
 const units=8*1024*1024*1024,calls=[];let revision=1;
 const session=create({request:async(action,p)=>{calls.push({action,...p});return action==='window'?{revision,units,lines:units/8,start:p.start,line:p.start/8,text:'x'.repeat(p.limit)}:{revision:++revision,units:units+1,lines:units/8,dirty:true};},identity:()=> 'bounded-edit'});
 session.initialize({revision,units,lines:units/8});await session.load({start:units-PAGE_UNITS});session.change(session.state().page.text+'!');await session.flush();
 assert.equal(session.state().page.text.length,PAGE_UNITS+1);const edit=calls.find(c=>c.action==='edit');assert.equal(edit.from,units);assert.equal(edit.text,'!');assert.ok(JSON.stringify(calls).length<1000);
});

test('a delayed save acknowledgement cannot mark later typing as saved',async()=>{
 const f=fixture('abc');await f.session.load();const held=deferred();
 f.intercept(async(action,p)=>{const result=await f.request(action,p);if(action==='save')await held.promise;return result;});
 const saving=f.session.save('fixture.txt',{encoding:'UTF-8',eol:'LF'});await new Promise(setImmediate);
 f.session.change('abc newer');await f.session.flush();held.resolve();await saving;
 assert.equal(f.text,'abc newer');assert.equal(f.session.state().metadata.dirty,true);assert.equal(f.session.state().metadata.revision,2);
});

test('an uncertain offscreen replacement is recovered without reapplying it',async()=>{
 const f=fixture('before\nvisible\nafter');await f.session.load({line:1},7);let losses=2;
 f.intercept(async(action,p)=>{const value=await f.request(action,p);if(action==='edit'&&losses-->0)throw Error('response lost');return value;});
 await assert.rejects(f.session.replace(0,f.text.length,'replacement'),/response lost/);
 assert.equal(f.session.state().blocked,true);assert.throws(()=>f.session.change('new'),/pending/);
 await f.session.flush();assert.equal(f.text,'replacement');assert.equal(f.session.state().page.text,'replacement');assert.equal(f.session.state().metadata.revision,2);
 const edits=f.calls.filter(c=>c.action==='edit');assert.equal(new Set(edits.map(c=>c.requestId)).size,1);
});

test('a failed refresh after an acknowledged edit cannot turn stale page text into a new insertion',async()=>{
 const f=fixture('before\nvisible\nafter');await f.session.load({line:1},7);let fail=true;
 f.intercept(async(action,p)=>{if(action==='window'&&fail)throw Error('read failed');return f.request(action,p);});
 await assert.rejects(f.session.replace(0,f.text.length,'replacement'),/read failed/);
 assert.equal(f.session.state().blocked,true);assert.throws(()=>f.session.change('stale page'),/pending/);
 fail=false;await f.session.flush();assert.equal(f.session.state().page.text,'replacement');assert.equal(f.calls.filter(c=>c.action==='edit').length,1);
});

test('multi-megabyte Unicode paste stages bounded fragments and commits one undo operation',async()=>{
 const inserted='€'.repeat(128*1024-1)+'😀'+'λ'.repeat(400000),chunks=[];
 const original='prefix:old:suffix';let content=original,revision=1,staged,committed=0;
 const metadata=()=>({revision,units:content.length,lines:1,dirty:revision>1,canUndo:revision>1});
 const session=create({identity:()=> 'transaction-id',request:async(action,p)=>{
  if(action==='window')return {...metadata(),start:p.start||0,line:0,text:content.slice(p.start||0,(p.start||0)+p.limit)};
  if(action==='edit-begin'){assert.equal(p.revision,1);staged={...p,text:''};return {};}
  if(action==='edit-chunk'){
   assert.equal(p.requestId,staged.requestId);assert.equal(p.offset,staged.text.length);
   assert.equal(p.text.isWellFormed(),true);assert.ok(Buffer.byteLength(p.text)<=1024*1024);
   chunks.push(p.text.length);staged.text+=p.text;return {received:staged.text.length};
  }
  if(action==='edit-commit'){
   assert.equal(p.requestId,staged.requestId);assert.equal(content,original);
   content=content.slice(0,staged.from)+staged.text+content.slice(staged.to);committed++;revision++;
   return metadata();
  }
  if(action==='undo'){content=original;revision++;return metadata();}
  throw Error('Unexpected action '+action);
 }});
 session.initialize(metadata());await session.load();await session.replace(7,10,inserted);
 assert.equal(content,'prefix:'+inserted+':suffix');assert.equal(committed,1);assert.ok(chunks.length>2);
 assert.ok(session.state().page.text.length<=PAGE_UNITS);await session.history();assert.equal(content,original);
});

test('superseded viewport requests abort and viewport reads do not advance the edit stamp',async()=>{
 const f=fixture('abcdefghijklmnopqrstuvwxyz'),started=deferred();
 f.intercept(async(action,p,signal)=>{
  if(action==='window'&&p.start===0){
   started.resolve();return new Promise((_,reject)=>signal.addEventListener('abort',()=>reject(signal.reason),{once:true}));
  }
  return f.request(action,p);
 });
 const old=f.session.load({start:0});await started.promise;
 await f.session.load({start:20});assert.equal(await old,null);
 assert.equal(f.session.state().page.text,'uvwxyz');assert.equal(f.session.state().editVersion,0);
 f.session.change('uvwxyZ');assert.equal(f.session.state().editVersion,1);await f.session.flush();
});

test('reading a cross-page selection rejects concurrent changes instead of returning mixed revisions',async()=>{
 const f=fixture('x'.repeat(PAGE_UNITS*2+10));await f.session.load();
 const text=f.session.readParts(0,f.text.length);
 assert.equal((await text.next()).value.length,PAGE_UNITS);
 f.session.change('Y'+f.session.state().page.text.slice(1));
 await assert.rejects(text.next(),/changed while reading/);await f.session.flush();
 await assert.rejects(f.session.readText(0,1000,10),/too large for this command/);
});

test('large clipboard selections use a binary range from one acknowledged document revision',async()=>{
 const seen=[];
 const session=create({request:()=>{throw Error('Clipboard export must not read text windows');},exportRange:async payload=>{
  seen.push(payload);return new Response(new TextEncoder().encode('one\nλ😀'),{headers:{'Content-Type':'application/octet-stream'}});
 }});
 session.initialize({revision:9,units:8*1024*1024*1024,lines:3});
 const from=4*1024*1024*1024,to=from+7;
 const blob=await session.readBlob(from,to);
 assert.deepEqual(seen,[{revision:9,from,to,format:{encoding:'UTF-8',bom:false,eol:'LF'}}]);
 assert.equal(blob.type,'text/plain');assert.equal(await blob.text(),'one\nλ😀');
});
