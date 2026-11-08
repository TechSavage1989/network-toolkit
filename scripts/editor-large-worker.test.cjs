'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),vm=require('node:vm'),fs=require('node:fs'),path=require('node:path');
const assets=path.resolve(__dirname,'../web/assets');
function harness(){
 const replies=[],context={};
 context.self=context;context.postMessage=value=>replies.push(value);
 context.importScripts=name=>{assert.equal(name,'editor-compare.js');vm.runInContext(fs.readFileSync(path.join(assets,name),'utf8'),context);};
 vm.createContext(context,{codeGeneration:{strings:false,wasm:false}});
 vm.runInContext(fs.readFileSync(path.join(assets,'editor-compare-worker.js'),'utf8'),context);
 return {context,replies};
}
test('same-origin worker computes exact line differences without app or network access',()=>{
 const {context,replies}=harness();
 context.onmessage({data:{id:7,left:'host\ninterface A\nend',right:'host\ninterface B\nend'}});
 assert.equal(replies.length,1);
 assert.equal(replies[0].id,7);
 assert.equal(replies[0].result.limited,false);
 assert.equal(replies[0].result.hunks.length,1);
 assert.equal(replies[0].result.rows[1].type,'changed');
 assert.equal(typeof context.fetch,'undefined');
 assert.equal(typeof context.XMLHttpRequest,'undefined');
});
test('worker keeps bounded result for unsupported inputs',()=>{
 const {context,replies}=harness();
 context.onmessage({data:{id:4,left:'x'.repeat(context.NTKEditorCompare.LIMITS.characters+1),right:''}});
 assert.equal(replies[0].result.limited,true);
 context.onmessage({data:{id:'bad',left:'a',right:'b'}});
 assert.equal(replies.length,1);
});
test('worker reports failure without including file content or secrets',()=>{
 const {context,replies}=harness();
 const old=context.NTKEditorCompare;
 context.NTKEditorCompare={compare(){throw Error('secret file content');}};
 context.onmessage({data:{id:3,left:'secret file content',right:'other'}});
 assert(replies[0].error.includes('Both documents remain unchanged'));
 assert(!replies[0].error.includes('secret'));
 context.NTKEditorCompare=old;
});
