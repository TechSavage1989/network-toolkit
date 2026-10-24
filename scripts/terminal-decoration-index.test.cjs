'use strict';
const assert=require('node:assert/strict'),{test}=require('node:test');
const fixture=require('./terminal-render-fixture.cjs');

test('decoration index deletes exact objects after their row keys change in arbitrary order',t=>{
 const list=new fixture.SortedList(item=>item?.line),entries=Array.from({length:20},(_,line)=>({line}));t.after(()=>list.clear());
 for(const entry of entries)list.insert(entry);
 assert.deepEqual([...list.values()],entries);
 const removed=[14,2,18,4,11,0,19,7];
 for(const index of removed){entries[index].line=-1;assert.equal(list.delete(entries[index]),true,'lost a disposed marker at '+index);}
 assert.deepEqual([...list.values()],entries.filter((_,index)=>!removed.includes(index)));
});

test('duplicate queued decoration removal cannot discard a neighboring live entry',t=>{
 const list=new fixture.SortedList(item=>item?.line),entries=[{line:1},{line:2},{line:3}];t.after(()=>list.clear());
 for(const entry of entries)list.insert(entry);
 assert.equal(list.delete(entries[1]),true);assert.equal(list.delete(entries[1]),false);
 assert.deepEqual([...list.values()],[entries[0],entries[2]]);assert.equal(list.delete(entries[1]),false);
});

test('pending insertion and deletion keep live decoration rows sorted after marker disposal',t=>{
 const list=new fixture.SortedList(item=>item?.line),entries=[{line:10},{line:20},{line:30},{line:40}];t.after(()=>list.clear());
 for(const entry of entries)list.insert(entry);
 assert.equal([...list.values()].length,4);
 entries[2].line=-1;assert(list.delete(entries[2]));
 const inserted=[{line:5},{line:25},{line:35}];for(const entry of inserted)list.insert(entry);
 entries[1].line=-1;assert(list.delete(entries[1]));
 assert.deepEqual([...list.values()].map(entry=>entry.line),[5,10,25,35,40]);
});
