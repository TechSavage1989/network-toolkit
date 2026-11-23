'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument,Element}=require('./terminal-dom-fixture.cjs');
const source=fs.readFileSync(require.resolve('../web/assets/app-dialogs.js'),'utf8');
const index=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
function harness(){
 const doc=makeDocument('<button id="opener">Open</button><div id="modalRoot"></div>'),observers=[],events={};
 const html=Object.getOwnPropertyDescriptor(Element.prototype,'innerHTML');
 function prepare(node){
  if(node.prepared)return;node.prepared=true;
  Object.defineProperty(node,'childNodes',{get:()=>node.children});
  Object.defineProperty(node,'innerHTML',{get(){return html.get.call(this);},set(value){for(const child of [...this.children])child.remove();html.set.call(this,value);for(const child of this.children)prepare(child);}});
  node.replaceChildren=(...children)=>{for(const child of [...node.children])child.remove();node.append(...children);};
  node.focus=()=>doc.activeElement=node;
  node.removeEventListener=(type,fn)=>{node.listeners[type]=(node.listeners[type]||[]).filter(item=>item!==fn);};
  for(const child of node.children)prepare(child);
 }
 prepare(doc);doc.body=doc;doc.activeElement=doc.getElementById('opener');
 const context={document:doc,console,Promise,setTimeout(){},icon:()=>'',esc:String,$:selector=>doc.querySelector(selector),
  prompt(){throw Error('Native prompt must never be used');},confirm(){throw Error('Native confirm must never be used');},
  addEventListener:(type,fn)=>{(events[type]||=[]).push(fn);},
  MutationObserver:class{constructor(fn){this.fn=fn;this.stopped=false;observers.push(this);}observe(){}disconnect(){this.stopped=true;}}
 };
 context.window=context;context.globalThis=context;vm.createContext(context);
 // Exercise the real integration hooks as well as the helper itself.
 vm.runInContext("let modalLastFocus=null;\n"+index.split('\n').filter(line=>line.startsWith('function openModal(')||line.startsWith('function closeModal(')).join('\n'),context);
 vm.runInContext(source,context);
 const modal=()=>doc.getElementById('modalRoot'),form=()=>doc.querySelector('[data-app-dialog]');
 const submit=()=>{let prevented=false;const f=form();f.emit('submit',f,{preventDefault(){prevented=true;}});assert(prevented);};
 return {doc,context,api:context.NetworkToolkitDialogs,modal,form,submit,observers,events};
}
test('text dialogs accept literal input once through app modal and restore the opener',async()=>{
 const h=harness(),answer=h.api.prompt({title:'Save profile',message:'Profile <name>',value:'Existing',acceptLabel:'Save'});
 const input=h.doc.querySelector('[data-app-dialog] input');
 assert.equal(input.value,'Existing');assert.equal(h.doc.activeElement,input);assert.equal(input.selectionEnd,8);
 assert.match(h.form().innerHTML,/Profile &lt;name&gt;/);assert.equal(h.form().getAttribute('novalidate'),'');
 input.value='Branch <north> & south';h.submit();
 assert.equal(await answer,'Branch <north> & south');assert.equal(h.modal().children.length,0);assert.equal(h.doc.activeElement.id,'opener');
 assert.equal(h.api.close(),false,'the completed question cannot settle twice');
});
test('confirmation starts on Cancel and never approves a cancelled dialog',async()=>{
 const h=harness();let answer=h.api.confirm({title:'Delete saved profile',message:'Remove the selected entry?',danger:true,acceptLabel:'Delete'});
 assert.equal(h.doc.activeElement.textContent,'Cancel');assert.match(h.modal().innerHTML,/class="btn danger"/);
 h.context.closeModal();assert.equal(await answer,false);assert.equal(h.modal().children.length,0);
 answer=h.api.confirm({message:'Continue?'});h.submit();assert.equal(await answer,true);
});
test('a question preserves existing modal nodes, field edits, listeners and focus',async()=>{
 const h=harness();h.context.openModal('Profiles','<input id="draft"><button id="rename">Rename</button>');
 const before=[...h.modal().childNodes],draft=h.doc.getElementById('draft'),button=h.doc.getElementById('rename');let clicks=0;
 draft.value='Unsaved profile edits';button.addEventListener('click',()=>clicks++);button.focus();
 const answer=h.api.prompt({message:'Profile name:',value:'Current'});h.context.closeModal();assert.equal(await answer,null);
 assert.deepEqual(h.modal().childNodes,before);assert.equal(h.doc.getElementById('draft'),draft);assert.equal(draft.value,'Unsaved profile edits');
 assert.equal(h.doc.activeElement,button);button.emit('click',button);assert.equal(clicks,1);assert.equal(h.doc.body.style.overflow,'hidden');
 h.context.closeModal();assert.equal(h.doc.activeElement.id,'opener');
});
test('accepting a question also restores its parent modal without recreating its fields',async()=>{
 const h=harness();h.context.openModal('Profiles','<input id="draft">');const previous=h.modal().firstElementChild;
 const answer=h.api.prompt({message:'Name:',value:'Router'});h.submit();assert.equal(await answer,'Router');assert.equal(h.modal().firstElementChild,previous);
});
test('replacement cancels the pending question without restoring an obsolete modal',async()=>{
 const h=harness();h.context.openModal('Previous','<input id="previous">');const answer=h.api.confirm({message:'Remove?'});
 h.context.openModal('New operation','<input id="current">');assert.equal(await answer,false);
 assert(h.doc.getElementById('current'));assert.equal(h.doc.getElementById('previous'),null);assert.equal(h.api.close(),false);
});
test('direct replacement and page close cancel without later approval or restoring old content',async()=>{
 const h=harness();let answer=h.api.prompt({message:'Name:'});h.modal().innerHTML='<p>Other content</p>';
 h.observers.at(-1).fn();assert.equal(await answer,null);assert.match(h.modal().innerHTML,/Other content/);
 answer=h.api.confirm({message:'Proceed?'});h.events.pagehide[0]();assert.equal(await answer,false);assert.equal(h.api.close(),false);
});
test('a concurrent question is declined and does not replace the question already shown',async()=>{
 const h=harness(),first=h.api.prompt({message:'First name',value:'First'}),form=h.form();
 assert.equal(await h.api.confirm({message:'Second question'}),false);assert.equal(await h.api.prompt({message:'Second name'}),null);assert.equal(h.form(),form);
 h.submit();assert.equal(await first,'First');
});
test('a detached form cannot approve before the replacement observer runs',async()=>{
 const h=harness(),answer=h.api.confirm({message:'Remove?'}),form=h.form();
 h.modal().innerHTML='<p>Replacement</p>';form.emit('submit',form);
 assert.equal(await answer,false);assert.match(h.modal().innerHTML,/Replacement/);
});
test('replacing only the modal body cancels the pending text question',async()=>{
 const h=harness(),answer=h.api.prompt({message:'Name:'});
 h.doc.querySelector('.modal-body').innerHTML='<p>New body</p>';h.observers.at(-1).fn();
 assert.equal(await answer,null);assert.match(h.modal().innerHTML,/New body/);assert.equal(h.api.close(),false);
});
test('missing app modal support cancels safely and never falls back to native browser dialogs',async()=>{
 const h=harness();h.context.openModal=undefined;assert.equal(await h.api.prompt({message:'Name'}),null);assert.equal(await h.api.confirm({message:'Continue?'}),false);
});
