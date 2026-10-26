'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument,Element}=require('./terminal-dom-fixture.cjs');
const source=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const reconcile=source.slice(source.indexOf('function resultNodeKey('),source.indexOf('function patchResultHTML('));
const render=source.slice(source.indexOf('function renderCurrentResult('),source.indexOf("\ndocument.addEventListener('selectionchange'",source.indexOf('function renderCurrentResult(')));

function harness(){
 const document=makeDocument('<div id="toolResult"></div>');document.body=document;document.activeElement=document;
 const originalMatches=Element.prototype.matches,originalRemove=Element.prototype.remove;
 function upgrade(node){
  if(node._reconciliationDOM){for(const child of node.children)upgrade(child);return node;}node._reconciliationDOM=true;
  Object.defineProperties(node,{nodeType:{get:()=>node.tagName==='#TEXT'?3:node.tagName==='FRAGMENT'?11:node.tagName==='DOCUMENT'?9:1},childNodes:{get:()=>node.children},firstChild:{get:()=>node.children[0]||null},nextSibling:{get:()=>node.parentElement?.children[node.parentElement.children.indexOf(node)+1]||null},attributes:{get:()=>Object.entries(node.attrs).map(([name,value])=>({name,value}))},nodeValue:{get:()=>node.text||'',set:value=>{node.text=value;}}});
  Object.defineProperty(node.classList,'0',{get:()=>node.className.split(/\s+/).filter(Boolean)[0]});
  node.matches=function(selector){if(selector.includes(','))return selector.split(',').some(part=>this.matches(part.trim()));if(selector.includes('>')){const parts=selector.split('>').map(part=>part.trim()),last=parts.pop();return this.matches(last)&&!!this.parentElement?.matches(parts.join('>'));}return originalMatches.call(this,selector);};
  node.remove=function(){if(this.isConnected&&this.contains(document.activeElement))document.activeElement=document.body;originalRemove.call(this);};
  node.insertBefore=function(child,before){if(child===before)return;child.remove();child.parentElement=this;const index=before?this.children.indexOf(before):-1;if(index<0)this.children.push(child);else this.children.splice(index,0,child);};
  node.replaceWith=function(replacement){if(!this.parentElement)return;this.parentElement.insertBefore(replacement,this);this.remove();};
  node.replaceChildren=function(...children){for(const child of [...this.children])child.remove();for(const child of children){if(child.nodeType===11)this.append(...[...child.children]);else this.append(child);}};
  node.cloneNode=function(deep){const copy=upgrade(new Element(this.tagName.toLowerCase(),{...this.attrs}));copy.text=this.text;if(deep)for(const child of this.children)copy.append(upgrade(child).cloneNode(true));return copy;};
  node.focus=()=>{document.activeElement=node;};for(const child of node.children)upgrade(child);return node;
 }
 upgrade(document);
 document.createElement=tag=>{const node=upgrade(new Element(tag));if(tag==='template'){const content=upgrade(new Element('fragment'));node.content=content;Object.defineProperty(node,'innerHTML',{get:()=>content.innerHTML,set:html=>{Reflect.getOwnPropertyDescriptor(Element.prototype,'innerHTML').set.call(content,html);for(const child of content.children)upgrade(child);}});}return node;};
 const context={document,console,copyText:value=>context.copied.push(value),copied:[],state:{activePage:'monitor'},lastRender:'',resultControlPending:false,resultUpdateDeferred:false,activeTool:()=> 'monitor',selectedJob:()=>context.job,titleFor:()=> 'Connection monitor',TOOLS:{monitor:{icon:'monitor',run:'Start monitor'}},getSelection:()=>({isCollapsed:true}),$:selector=>document.querySelector(selector),NetworkToolkitWorkspace:{prepareResult(){},afterRender(){}},NetworkToolkitResultDetails:{afterRender(){}},bindToolResultRegion(){},updateRunButtons(){},patchNetworkProgressOnly(){},empty:()=>'',online:true};
 context.window=context;vm.createContext(context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/value-copy.js'),'utf8'),context);
 const copy=context.NetworkToolkitValueCopy;context.NetworkToolkitValueCopy={...copy,prepare(container){const result=copy.prepare(container);upgrade(container);return result;}};
 context.renderJob=job=>'<div class="panel"><div class="result-body"><div class="graph-workspace" data-graph-workspace="'+job.id+'"><section class="graph-workspace-graph"><div class="rt-timeline" data-rt-id="'+job.id+'">Graph</div></section><section class="graph-workspace-results"><div class="data-list" data-list-view="samples"><div class="table-wrap"><table data-column-view="result:monitor:samples"><thead><tr><th data-column-id="sequence" data-column-name="#">#</th><th data-column-id="address" data-column-name="Address">Address</th></tr></thead><tbody>'+job.result.data.samples.map(sample=>'<tr data-entry-key="shared-endpoint" data-result-graph-id="'+job.id+'" data-result-sample-key="sample-'+sample.sequence+'"><td data-column-id="sequence">'+sample.sequence+'</td><td data-column-id="address">192.0.2.20</td></tr>').join('')+'</tbody></table></div></div></section></div></div></div>';
 vm.runInContext(reconcile+'\n'+render,context);
 function update(sequences){context.job={id:'live-monitor',started:'2026-10-26T18:00:00Z',tool:'monitor',status:'running',result:{data:{samples:sequences.map(sequence=>({sequence}))}}};context.renderCurrentResult(true);context.NetworkToolkitValueCopy.prepare(document);/* The real observer also prepares inserted values after reconciliation. */}
 return {document,context,update};
}

test('new same-endpoint monitor samples retain the original focused value-copy control and its value',()=>{
 const h=harness();h.update([1]);const originalRow=h.document.querySelector('[data-result-sample-key="sample-1"]'),button=originalRow.querySelector('[data-value-copy-control]');assert(button,'Initial live render prepares real copy controls');button.focus();
 h.update([2,1]);h.update([3,2,1]);
 assert(button.isConnected,'The original sample control is not replaced during live reconciliation');assert.equal(h.document.activeElement,button,'Focus stays on that exact control');assert.equal(button.closest('[data-result-sample-key]').dataset.resultSampleKey,'sample-1','A shared endpoint must not repurpose an older sample row');assert.equal(h.document.querySelector('[data-result-sample-key="sample-1"]'),originalRow);
 h.document.emit('click',button);assert.equal(h.context.copied.at(-1),'1','The preserved control still copies its original sample');assert.notEqual(h.document.querySelector('[data-result-sample-key="sample-3"] [data-value-copy-control]'),button);
});

test('sample reconciliation identity is scoped by graph and takes precedence over endpoint identity',()=>{
 const h=harness(),one=h.document.createElement('tr'),two=h.document.createElement('tr'),other=h.document.createElement('tr');
 for(const [node,graph,sample] of [[one,'run-a','sample-1'],[two,'run-a','sample-2'],[other,'run-b','sample-1']]){node.setAttribute('data-entry-key','same-endpoint');node.setAttribute('data-result-graph-id',graph);node.setAttribute('data-result-sample-key',sample);}
 assert.notEqual(h.context.resultNodeKey(one),h.context.resultNodeKey(two));assert.notEqual(h.context.resultNodeKey(one),h.context.resultNodeKey(other));
});
