'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const root=path.resolve(__dirname,'..'),html=fs.readFileSync(path.join(root,'web/index.html'),'utf8');
test('all first-party classic application scripts parse before launch',()=>{
 const assets=[...html.matchAll(/<script\b[^>]*\bsrc="([^"]+\.js)"/g)]
  .map(match=>match[1].replace(/^\.?\.?(?:\/)?/,'').replace(/^\/+/,''))
  .filter(url=>url.startsWith('assets/')&&!url.startsWith('assets/vendor/')&&!url.startsWith('assets/editor-vendor/'));
 assert(assets.length>=50,'The loaded asset list must be inspected when the application bootstrap changes');
 for(const name of new Set(assets)){
  const file=path.join(root,'web',name);
  assert(fs.existsSync(file),'Missing application script '+name);
  assert.doesNotThrow(()=>new vm.Script(fs.readFileSync(file,'utf8'),{filename:name}),'Invalid external browser script: '+name);
 }
});
