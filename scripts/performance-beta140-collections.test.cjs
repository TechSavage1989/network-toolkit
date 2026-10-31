'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const assets=name=>fs.readFileSync(path.join(__dirname,'../web/assets/',name),'utf8');
function section(name,start,end){
 const source=assets(name),at=source.indexOf(start),after=source.indexOf(end,at);
 assert(at>=0&&after>at,'Missing cached file-list function in '+name);
 const context={};vm.createContext(context);vm.runInContext(source.slice(at,after),context);return context;
}
const remote=()=>section('remote-browser.js','const remoteFileSortCache=','\nfunction localDriveChoices(');
const server=()=>section('transfer-explorer.js','function extension(','\nfunction paint(');
const list=(names,base='/test')=>names.map((name,i)=>({name,path:base+'/'+name,directory:name.endsWith('/'),size:i+1,modified:'2026-10-31T10:00:00Z'}));
test('SSH/FTP file browser reuses listing order during repeated selection, but refresh/filter/sort all invalidate it',()=>{
 const h=remote(),p={entries:list(['file10','file2','file1','.hidden','folder/']),filter:'',sort:'name',reverse:false};
 const before=JSON.stringify(p.entries),one=h.sortedItems(p);
 assert.equal(one[0].name,'folder/');assert.deepEqual(Array.from(one.slice(1,4),r=>r.name),['.hidden','file1','file2']);
 assert.strictEqual(h.sortedItems(p),one,'same listing and preferences must reuse the ordered view');
 p.filter='FILE';const filtered=h.sortedItems(p);assert.notStrictEqual(filtered,one);assert.deepEqual(Array.from(filtered,r=>r.name),['file1','file2','file10']);
 assert.strictEqual(h.sortedItems(p),filtered);
 p.reverse=true;assert.deepEqual(Array.from(h.sortedItems(p),r=>r.name),['file10','file2','file1']);
 p.sort='size';assert.deepEqual(Array.from(h.sortedItems(p),r=>r.name),['file1','file2','file10']);
 p.entries=list(['file6','file4']);assert.notStrictEqual(h.sortedItems(p),filtered);
 assert.equal(JSON.stringify(list(['file10','file2','file1','.hidden','folder/'])),before,'source rows remain unmodified');
});
test('SSH/FTP order stays stable for equal size, preserves directory-first sorting, and separates panes',()=>{
 const h=remote(),p={entries:list(['a1','a2','b2','group/']),sort:'size',reverse:true,filter:''},first=h.sortedItems(p);
 const other={...p};assert.notStrictEqual(h.sortedItems(other),first);
 assert.equal(first[0].name,'group/');
 p.entries=p.entries.map(item=>({...item,size:5}));assert.deepEqual(Array.from(h.sortedItems(p),r=>r.name),['group/','a1','a2','b2']);
});
test('TFTP/SFTP explorer caches sorted and filtered views until the folder, filter or options change',()=>{
 const h=server(),p={entries:list(['entry10','entry2','.secret','dir/','entry1']),sort:'name',reverse:false,hidden:false,filter:''};
 const prior=JSON.stringify(p.entries),ordered=h.ordered(p),shown=h.visible(p);
 assert.strictEqual(h.ordered(p),ordered);assert.strictEqual(h.visible(p),shown);assert.deepEqual(Array.from(shown,r=>r.name),['dir/','entry1','entry2','entry10']);
 p.hidden=true;const unhidden=h.visible(p);assert.notStrictEqual(unhidden,shown);assert.equal(unhidden.length,5);
 p.filter='ENTRY';const filtered=h.visible(p);assert.deepEqual(Array.from(filtered,r=>r.name),['entry1','entry2','entry10']);assert.strictEqual(h.visible(p),filtered);
 p.reverse=true;assert.deepEqual(Array.from(h.visible(p),r=>r.name),['entry10','entry2','entry1']);
 p.sort='size';const sizes=h.visible(p);assert.deepEqual(Array.from(sizes,r=>r.name),['entry1','entry2','entry10']);
 p.entries=list(['newfile','newdir/']);p.filter='';assert.notStrictEqual(h.visible(p),sizes);
 assert.equal(JSON.stringify(list(['entry10','entry2','.secret','dir/','entry1'])),prior);
});
test('file-list caches do not change modified/type sorting semantics or share remote listing state',()=>{
 const h=server(),entries=[{name:'alpha.txt',path:'/alpha.txt',directory:false,size:4,modified:'2026-10-30T12:00:00Z'},{name:'beta.png',path:'/beta.png',directory:false,size:2,modified:'2026-10-31T12:00:00Z'},{name:'folder',path:'/folder',directory:true,size:0,modified:'2026-10-29T12:00:00Z'}],p={entries,sort:'modified',reverse:false,filter:'',hidden:false};
 assert.deepEqual(Array.from(h.ordered(p),r=>r.name),['folder','alpha.txt','beta.png']);
 p.sort='type';assert.deepEqual(Array.from(h.ordered(p),r=>r.name),['folder','beta.png','alpha.txt']);
 p.reverse=true;assert.deepEqual(Array.from(h.ordered(p),r=>r.name),['folder','alpha.txt','beta.png']);
});
test('graph time range uses linear extrema while preserving unknown samples and original order',()=>{
 const source=assets('graph-history.js');let comparisons=0;
 const ctx={NetworkToolkitResultTime:{epoch:x=>typeof x==='number'?x:Date.parse(x),compareInstants:(a,b)=>{comparisons++;return Math.sign(a-b);}}};vm.createContext(ctx);vm.runInContext(source,ctx);
 const n=5000,rows=Array.from({length:n},(_,i)=>({time:(i*293)%n,value:i})),original=rows.map(r=>r.time);
 const scope=ctx.NetworkToolkitGraphHistory.range(rows,{anchor:'latest',durationMs:0});
 assert.equal(scope.first,0);assert.equal(scope.last,n-1);assert.equal(scope.unknown,0);
 assert(comparisons<=n*3,'extrema selection must not sort the entire sample history');
 assert.deepEqual(rows.map(r=>r.time),original,'range calculation cannot mutate samples');
 const reversed=rows.slice().reverse(),range=ctx.NetworkToolkitGraphHistory.range(reversed,{anchor:'range',start:100,end:104});
 const selected=ctx.NetworkToolkitGraphHistory.select(reversed,range);
 assert.deepEqual(Array.from(selected,r=>r.time),[100,101,102,103,104]);
 const linear=Array.from({length:100},(_,i)=>({time:i}));
 assert.deepEqual(Array.from(ctx.NetworkToolkitGraphHistory.select(linear,{start:10,end:20}),r=>r.time),Array.from({length:11},(_,i)=>i+10));
 assert.deepEqual(ctx.NetworkToolkitGraphHistory.range([{time:0},{time:null},{time:1}],{anchor:'latest',durationMs:0}).unknown,1);
});
