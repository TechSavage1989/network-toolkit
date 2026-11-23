'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const source=fs.readFileSync(require.resolve('../web/assets/app-interaction.js'),'utf8');

function harness(){
 const listeners=[];
 vm.runInNewContext(source,{document:{addEventListener(type,listener,options){listeners.push({type,listener,options});}}});
 return listeners;
}

test('browser context menus are cancelled for chrome, editable fields and output without consuming app handlers',()=>{
 const listeners=harness();
 assert.equal(listeners.length,1);assert.equal(listeners[0].type,'contextmenu');assert.equal(listeners[0].options.capture,true);
 for(const tag of ['BUTTON','INPUT','TEXTAREA','DIV','TD']){
  const event=new Event('contextmenu',{cancelable:true,bubbles:true});
  Object.defineProperty(event,'target',{value:{tagName:tag}});
  event.stopPropagation=()=>assert.fail('App handlers must still receive '+tag+' contextmenu');
  event.stopImmediatePropagation=event.stopPropagation;
  listeners[0].listener(event);
  assert.equal(event.defaultPrevented,true,tag);
  let opened=false;
  const appHandler=received=>{assert.equal(received,event);opened=true;};
  appHandler(event);assert.equal(opened,true);
 }
});

test('already handled and keyboard-origin context menus remain cancelled without changing their target',()=>{
 const [{listener}]=harness(),target={tagName:'TEXTAREA'},event=new Event('contextmenu',{cancelable:true});
 Object.defineProperties(event,{target:{value:target},clientX:{value:0},clientY:{value:0}});
 event.preventDefault();listener(event);
 assert.equal(event.defaultPrevented,true);assert.equal(event.target,target);assert.equal(event.clientX,0);assert.equal(event.clientY,0);
});

test('the shared guard leaves native edit, selection, clipboard and keyboard events unregistered',()=>{
 const types=harness().map(item=>item.type);
 for(const type of ['keydown','keyup','mousedown','pointerdown','selectstart','selectionchange','copy','cut','paste','beforeinput','input'])assert(!types.includes(type),type);
});

test('all app window modes receive the common interaction assets once',()=>{
 const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
 assert.equal((html.match(/href="assets\/app-interaction\.css"/g)||[]).length,1);
 assert.equal((html.match(/src="assets\/app-interaction\.js"/g)||[]).length,1);
 assert(html.indexOf('assets/app-interaction.js')<html.indexOf('assets/editor-context-menu.js'),'The guard is present before app menus initialize');
});
