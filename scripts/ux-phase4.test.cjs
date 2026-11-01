'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict');
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const root=path.resolve(__dirname,'..');
function recovery(){
 const handlers={},document={
  addEventListener(name,handler){(handlers[name]??=[]).push(handler);},
  getElementById(){return null;},querySelector(){return null;}
 };
 const window={document,setTimeout:()=>0};
 vm.runInNewContext(fs.readFileSync(path.join(root,'web/assets/ux-tab-recovery.js'),'utf8'),{window});
 return {api:window.NetworkToolkitTabRecovery,handlers,window};
}
test('recently closed Editor documents restore in newest-first order without persistence',()=>{
 const {api}=recovery(),restored=[];
 assert.equal(api.canReopen(),false);
 api.remember('editor','first',()=>{restored.push('first');return true;});
 api.remember('editor','second',()=>{restored.push('second');return true;});
 assert.equal(api.historyCount(),2);assert.equal(api.reopen(),true);
 assert.equal(api.reopen(),true);assert.deepEqual(restored,['second','first']);
 assert.equal(api.reopen(),false);
});
test('failed tab restoration retains the entry without claiming success',()=>{
 const {api}=recovery();let possible=false;
 api.remember('terminal','draft',()=>possible);
 assert.equal(api.reopen(),false);assert.equal(api.canReopen(),true);
 possible=true;assert.equal(api.reopen(),true);assert.equal(api.historyCount(),0);
});
test('tab history is bounded and never accepts live connection records',()=>{
 const {api}=recovery();assert.equal(api.remember('connection','ssh',()=>true),false);
 for(let i=0;i<40;i++)api.remember('editor','draft'+i,()=>true);
 assert.equal(api.historyCount(),8);
});
test('reopen shortcut reserves focused terminal input and existing dialogs',()=>{
 const {api,handlers,window}=recovery();let restored=0;
 api.remember('editor','doc',()=>{restored++;return true;});
 const target={closest:selector=>null};
 function event(withTarget){return{target:withTarget||target,key:'T',ctrlKey:true,shiftKey:true,altKey:false,isComposing:false,defaultPrevented:false,preventDefault(){this.defaultPrevented=true;},stopImmediatePropagation(){}};}
 const terminal=event({closest:()=>({})});handlers.keydown[0](terminal);
 assert.equal(terminal.defaultPrevented,false);assert.equal(restored,0);
 const plain=event();handlers.keydown[0](plain);
 assert.equal(plain.defaultPrevented,true);assert.equal(restored,1);
});
test('settings module and app call preserve existing control IDs and file permissions',()=>{
 const src=fs.readFileSync(path.join(root,'web/assets/ux-unified-settings.js'),'utf8');
 const html=fs.readFileSync(path.join(root,'web/index.html'),'utf8');
 assert.match(src,/appUpdateSettings/);assert.match(src,/vendorSettings/);
 assert.match(src,/storageSettings/);assert.match(src,/root\.NetworkToolkitFileEditor\?\.refreshPreferences/);
 assert.match(src,/ctx\.saveState\(\)/);
 assert.match(html,/NetworkToolkitUnifiedSettings\?\.enhance/);
 assert.match(html,/function reviewLocalConfiguration/);
 assert.match(html,/function confirmDeleteStorage/);
 assert.match(html,/function deleteSavedProfile/);
 assert.match(html,/NetworkToolkitTabRecovery\?\.remember/);
});
