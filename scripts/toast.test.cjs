'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const source=fs.readFileSync(require.resolve('../web/index.html'),'utf8').split('\n').find(line=>line.startsWith('function toast('));
function harness(){
 const document=makeDocument('<div id="toastStack"></div>'),timers=new Map(),cleared=[];let sequence=0;
 const c={document,$:s=>document.querySelector(s),String,icon:()=>'',esc:s=>String(s).replaceAll('&','&amp;').replaceAll('<','&lt;'),setTimeout:(fn,ms)=>{const id=++sequence;timers.set(id,{fn,ms});return id;},clearTimeout:id=>{cleared.push(id);timers.delete(id);}};
 vm.createContext(c);vm.runInContext(source,c);return {c,document,timers,cleared,stack:document.querySelector('#toastStack')};
}
test('repeated clipboard feedback reuses one visible notification and refreshes its expiry',()=>{
 const h=harness();h.c.toast('Copied to clipboard.','success');const node=h.stack.children[0],first=node.toastTimer;
 h.c.toast('Copied to clipboard.','success');h.c.toast('Copied to clipboard.','success');
 assert.equal(h.stack.children.length,1);assert.equal(h.stack.children[0],node);assert(h.cleared.includes(first));assert.equal(h.timers.size,1);
 const last=h.timers.get(node.toastTimer);assert.equal(last.ms,3800);last.fn();assert.equal(h.stack.children.length,0);
});
test('different messages and severities remain independent and escaped',()=>{
 const h=harness();h.c.toast('<saved>','success');h.c.toast('<saved>','error');h.c.toast('Connection failed','error');h.c.toast('<saved>','error');
 assert.equal(h.stack.children.length,3);assert.equal(h.timers.size,3);assert(h.stack.children[0].innerHTML.includes('&lt;saved>'));assert.equal(h.timers.get(h.stack.children[1].toastTimer).ms,6500);
});
