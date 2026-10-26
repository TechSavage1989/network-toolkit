'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const workspace=require('../web/assets/graph-workspace.js'),{document:makeDocument}=require('./terminal-dom-fixture.cjs');
function harness(){
 const document=makeDocument('');let saved=0,observer;
 const context={document,state:{},console,requestAnimationFrame:fn=>fn(),saveState:()=>saved++,MutationObserver:class{constructor(fn){observer=fn;}observe(){}}};
 context.window=context;vm.createContext(context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/graph-workspace.js'),'utf8'),context);
 const api=context.NetworkToolkitGraphWorkspace;
 function render(id='job',content='<table><tbody><tr><td>Sample</td></tr></tbody></table>'){
  const html=api.html(id,'<div>Graph and point details</div>',content);document.innerHTML=html;api.restore();return document.querySelector('[data-graph-workspace]');
 }
 return {context,document,api,render,saved:()=>saved,observe:()=>observer?.()};
}
test('layout preferences normalize invalid restoration and bound both pane shares',()=>{
 for(const value of [undefined,null,[],{ratio:null},{ratio:''},{ratio:'0.7'},{ratio:NaN},{ratio:Infinity}])assert.deepEqual(workspace.normalize(value),{ratio:.6});
 assert.deepEqual(workspace.normalize({ratio:0}),{ratio:.3});assert.deepEqual(workspace.normalize({ratio:1}),{ratio:.75});assert.deepEqual(workspace.normalize({ratio:.52}),{ratio:.52});
 const html=workspace.html('"<unsafe>','GRAPH','RESULTS');assert(html.includes('&quot;&lt;unsafe&gt;'));assert(!html.includes('data-graph-workspace=""<'));
 assert.match(html,/data-graph-pane/);assert.match(html,/data-graph-results/);assert.match(html,/role="separator"/);assert.match(html,/aria-orientation="horizontal"/);
});
test('keyboard divider resizing persists actual bounded shares without affecting scan data',()=>{
 const h=harness(),node=h.render(),divider=node.querySelector('[data-graph-divider]');h.context.state.jobs=[{id:'untouched'}];
 h.document.emit('keydown',divider,{key:'ArrowDown'});assert.equal(divider.getAttribute('aria-valuenow'),'65');assert.equal(h.saved(),1);
 h.document.emit('keydown',divider,{key:'End'});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.75);
 h.document.emit('keydown',divider,{key:'ArrowDown'});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.75);
 h.document.emit('keydown',divider,{key:'Home'});assert.equal(divider.getAttribute('aria-valuenow'),'30');
 h.document.emit('keydown',divider,{key:'Enter'});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.6);
 assert.equal(h.context.state.jobs[0].id,'untouched');
 h.document.emit('dblclick',divider);assert.equal(h.context.state.graphWorkspacePreferences.ratio,.6);
});
test('pointer divider resizing uses available workspace height and ignores other pointers',()=>{
 const h=harness(),node=h.render(),divider=node.querySelector('[data-graph-divider]');node.getBoundingClientRect=()=>({top:100,height:410});
 h.document.emit('pointerdown',divider,{button:0,pointerId:7,clientY:340});
 h.document.emit('pointermove',divider,{pointerId:8,clientY:400});assert.equal(h.context.state.graphWorkspacePreferences,undefined);
 h.document.emit('pointermove',divider,{pointerId:7,clientY:300});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.5);assert.equal(h.saved(),0);
 h.document.emit('pointerup',divider,{pointerId:8});assert.equal(h.saved(),0);
 h.document.emit('pointerup',divider,{pointerId:7});assert.equal(h.saved(),1);assert.equal(node.classList.contains('graph-workspace-dragging'),false);
});
test('graph and list scroll independently and keep their positions across live replacement',()=>{
 const h=harness();let node=h.render(),graph=node.querySelector('[data-graph-pane]'),results=node.querySelector('[data-graph-results]');
 graph.scrollTop=44;graph.scrollLeft=3;h.document.emit('scroll',graph);results.scrollTop=275;results.scrollLeft=60;h.document.emit('scroll',results);
 node=h.render();graph=node.querySelector('[data-graph-pane]');results=node.querySelector('[data-graph-results]');
 assert.equal(graph.scrollTop,44);assert.equal(graph.scrollLeft,3);assert.equal(results.scrollTop,275);assert.equal(results.scrollLeft,60);
 node=h.render('different-job');assert.equal(node.querySelector('[data-graph-pane]').scrollTop,0);assert.equal(node.querySelector('[data-graph-results]').scrollTop,0);
 node=h.render('job');assert.equal(node.querySelector('[data-graph-results]').scrollTop,275);
});
test('saved graph layout merges only its own normalized preference',()=>{
 const h=harness();h.context.state.target='192.0.2.20';h.api.mergePreferences({graphWorkspacePreferences:{ratio:.72},target:'changed'});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.72);assert.equal(h.context.state.target,'192.0.2.20');
 h.api.mergePreferences({graphWorkspacePreferences:{ratio:99}});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.75);
 h.api.mergePreferences({graphWorkspacePreferences:{ratio:'bad'}});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.6);
});
test('nested sample table keeps both scroll axes and the short-workspace fallback position',()=>{
 const h=harness(),content='<div class="data-list-tools">Filter</div><div class="table-wrap"><table><tbody><tr><td>Sample</td></tr></tbody></table></div><div class="data-list-foot">Page controls</div>';
 let node=h.render('nested',content),owner=h.api.resultScroller(node);assert(owner.classList.contains('table-wrap'));
 owner.scrollTop=312;owner.scrollLeft=145;node.scrollTop=61;h.document.emit('scroll',owner);h.document.emit('scroll',node);
 node=h.render('nested',content);owner=h.api.resultScroller(node);assert.equal(owner.scrollTop,312);assert.equal(owner.scrollLeft,145);assert.equal(node.scrollTop,61);
 assert.equal(node.querySelector('[data-graph-results]').scrollTop,undefined,'The fixed filter/footer region is not the sample-row scroll owner');
});
test('pane minimum accounts for real toolbar, footer, header and sample rows without changing the saved split',()=>{
 const h=harness(),node=h.render('minimum','<div class="data-list-tools">Filter</div><div class="table-wrap"><table><thead><tr><th>Sample</th></tr></thead><tbody><tr><td>Result</td></tr></tbody></table></div><div class="data-list-foot">Pages</div>'),region=node.querySelector('[data-graph-results]'),owner=h.api.resultScroller(node);
 node.querySelector('.data-list-tools').getBoundingClientRect=()=>({height:41});node.querySelector('.data-list-foot').getBoundingClientRect=()=>({height:39});owner.querySelector('thead').getBoundingClientRect=()=>({height:30});owner.querySelector('tbody tr').getBoundingClientRect=()=>({height:38});owner.offsetHeight=170;owner.clientHeight=154;h.context.state.graphWorkspacePreferences={ratio:.6};
 h.context.innerHeight=1000;h.api.restore();assert.equal(region.style['--graph-table-min-height'],'162px');assert.equal(node.style['--graph-list-min-height'],'244px');
 h.context.innerHeight=650;h.api.restore();assert.equal(region.style['--graph-table-min-height'],'140px');assert.equal(node.style['--graph-list-min-height'],'222px');assert.equal(h.context.state.graphWorkspacePreferences.ratio,.6);
});
test('graph-driven result reveal uses actual filtered sort position and never clears filters',()=>{
 const h=harness(),rows=Array.from({length:240},(_,i)=>({key:'sample-'+(240-i)})),config={id:'samples',graphId:'job',graphKey:row=>row.key,rows,pageSize:100};
 const node=h.render('job','<div class="data-list-tools">Filters</div><table><thead><tr><th>Sample</th></tr></thead><tbody><tr data-result-sample-key="sample-61"><td>61</td></tr></tbody></table>');
 let refreshes=0;h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map([['samples',0]]);h.context.dataListRows=()=>rows;h.context.refreshDataList=id=>{assert.equal(id,'samples');refreshes++;};
 const results=node.querySelector('[data-graph-results]');results.scrollTop=0;results.getBoundingClientRect=()=>({top:100,bottom:300,height:200});const row=results.querySelector('[data-result-sample-key]');row.getBoundingClientRect=()=>({top:350,bottom:371,height:21});
 assert.equal(h.api.revealSample('job','sample-61'),true);assert.equal(h.context.dataListPages.get('samples'),1);assert.equal(refreshes,1);assert.equal(results.scrollTop,71);
 assert.equal(h.api.revealSample('job','filtered-out'),false);assert.equal(refreshes,1,'A filtered row does not reset the filter or page');assert.equal(h.document.querySelector('[data-graph-pane]').textContent,'Graph and point details');
});
