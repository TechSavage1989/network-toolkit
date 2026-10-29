'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),crypto=require('node:crypto'),vm=require('node:vm');
const root=path.resolve(__dirname,'..'),vendor=path.join(root,'web/assets/vendor/codemirror'),builder=path.join(root,'scripts/vendor/codemirror');
const read=(where,name)=>fs.readFileSync(path.join(where,name)),sha=b=>crypto.createHash('sha256').update(b).digest('hex'),manifest=JSON.parse(read(vendor,'SOURCE.json')),lock=JSON.parse(read(builder,'package-lock.json'));
test('offline CodeMirror bundle and provenance hashes match exact checked-in inputs',()=>{
 for(const [name,info] of Object.entries(manifest.artifacts)){const bytes=read(vendor,name);assert.equal(bytes.length,info.bytes,name);assert.equal(sha(bytes),info.sha256,name);}
 for(const [name,key] of [['entry.mjs','entrySHA256'],['package-lock.json','lockSHA256'],['build.mjs','buildScriptSHA256']])assert.equal(sha(read(builder,name)),manifest.inputs[key],name);
 assert.equal(manifest.bundler.version,'0.28.2');assert.equal(manifest.bundler.sourcemap,false);
});
test('every runtime/build dependency is pinned to official registry integrity and licensed',()=>{
 const pkg=JSON.parse(read(builder,'package.json'));
 for(const version of Object.values({...pkg.dependencies,...pkg.devDependencies}))assert.match(version,/^\d+\.\d+\.\d+$/);
 const expected=['@codemirror/state','@codemirror/view','@marijn/find-cluster-break','crelt','style-mod','w3c-keyname'];assert.deepEqual(manifest.packages.map(p=>p.name),expected);
 for(const [name,p] of Object.entries(lock.packages)){if(!name)continue;assert.match(p.resolved,/^https:\/\/registry\.npmjs\.org\//);assert.match(p.integrity,/^sha512-/);assert.equal(p.license,'MIT');}
 for(const p of manifest.packages){const record=lock.packages['node_modules/'+p.name];assert.equal(record.version,p.version);assert.equal(record.integrity,p.integrity);assert(read(vendor,'LICENSES.txt').includes('===== '+p.name+' '+p.version+' ====='));}
 assert(read(builder,'BUILD_LICENSES.txt').includes('Copyright (c) 2020 Evan Wallace'));
});
test('runtime exports only requested core and needs no network, eval or dynamic imports',()=>{
 const code=read(vendor,'codemirror.js').toString();assert(!/sourceMappingURL|\bimport\s*\(|\b(?:fetch|XMLHttpRequest|WebSocket|importScripts|eval|Function)\s*\(/.test(code));
 const context=vm.createContext({setTimeout,clearTimeout,console,fetch(){throw Error('network forbidden')},XMLHttpRequest(){throw Error('network forbidden')},WebSocket(){throw Error('network forbidden')},Worker(){throw Error('worker forbidden')}},{codeGeneration:{strings:false,wasm:false}});
 vm.runInContext(code,context,{timeout:3000});const api=context.NetworkToolkitCodeMirror;
 assert.deepEqual(Object.keys(api).sort(),manifest.exports);for(const name of manifest.exports)assert.notEqual(api[name],undefined,name);
 const state=api.EditorState.create({doc:'one\n😀 two\nthree'}),next=state.update({changes:{from:4,to:4,insert:'new\n'},selection:api.EditorSelection.cursor(8)}).state;
 assert.equal(next.doc.toString(),'one\nnew\n😀 two\nthree');assert.equal(next.selection.main.head,8);assert.equal(state.doc.toString(),'one\n😀 two\nthree');
 assert.equal(api.Decoration.line({class:'synthetic-line'}).range(4).from,4);
});
