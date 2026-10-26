'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const workspace=require('../web/assets/graph-workspace.js'),{document:makeDocument}=require('./terminal-dom-fixture.cjs');
function harness(){
 const document=makeDocument('');let saved=0,observer;
 const context={document,state:{},console,requestAnimationFrame:fn=>fn(),saveState:()=>saved++,MutationObserver:class{constructor(fn){observer=fn;}observe(){}}};
 context.window=context;vm.createContext(context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/graph-workspace.js'),'utf8'),context);
 const api=context.NetworkToolkitGraphWorkspace;
 function render(id='job',content='<table><tbody><tr><td>Sample</td></tr></tbody></table>',options={}){
  const html=api.html(id,'<div>Graph and point details</div>',content,options);document.innerHTML=html;api.restore();return document.querySelector('[data-graph-workspace]');
 }
 return {context,document,api,render,saved:()=>saved,observe:()=>observer?.()};
}
test('layout preferences normalize invalid restoration and bound both pane shares',()=>{
 for(const value of [undefined,null,[],{ratio:null},{ratio:''},{ratio:'0.7'},{ratio:NaN},{ratio:Infinity}])assert.deepEqual(workspace.normalize(value),{ratio:.4,collapsedTools:{}});
 assert.deepEqual(workspace.normalize({ratio:0}),{ratio:.3,collapsedTools:{}});assert.deepEqual(workspace.normalize({ratio:1}),{ratio:.75,collapsedTools:{}});assert.deepEqual(workspace.normalize({ratio:.52}),{ratio:.52,collapsedTools:{}});
 const html=workspace.html('"<unsafe>','GRAPH','RESULTS');assert(html.includes('&quot;&lt;unsafe&gt;'));assert(!html.includes('data-graph-workspace=""<'));
 assert.match(html,/data-graph-pane/);assert.match(html,/data-graph-results/);assert.match(html,/role="separator"/);assert.match(html,/aria-orientation="horizontal"/);
});
test('keyboard divider resizing persists actual bounded shares without affecting scan data',()=>{
 const h=harness(),node=h.render(),divider=node.querySelector('[data-graph-divider]');h.context.state.jobs=[{id:'untouched'}];
 h.document.emit('keydown',divider,{key:'ArrowDown'});assert.equal(divider.getAttribute('aria-valuenow'),'45');assert.equal(h.saved(),1);
 h.document.emit('keydown',divider,{key:'End'});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.75);
 h.document.emit('keydown',divider,{key:'ArrowDown'});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.75);
 h.document.emit('keydown',divider,{key:'Home'});assert.equal(divider.getAttribute('aria-valuenow'),'30');
 h.document.emit('keydown',divider,{key:'Enter'});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.4);
 assert.equal(h.context.state.jobs[0].id,'untouched');
 h.document.emit('dblclick',divider);assert.equal(h.context.state.graphWorkspacePreferences.ratio,.4);
});
test('pointer divider resizing uses available workspace height and ignores other pointers',()=>{
 const h=harness(),node=h.render(),divider=node.querySelector('[data-graph-divider]');node.getBoundingClientRect=()=>({top:100,height:410});
 h.document.emit('pointerdown',divider,{button:0,pointerId:7,clientY:340});
 h.document.emit('pointermove',divider,{pointerId:8,clientY:400});assert.equal(h.context.state.graphWorkspacePreferences,undefined);
 h.document.emit('pointermove',divider,{pointerId:7,clientY:300});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.5);assert.equal(h.saved(),0);
 h.document.emit('pointerup',divider,{pointerId:8});assert.equal(h.saved(),0);
 h.document.emit('pointerup',divider,{pointerId:7});assert.equal(h.saved(),1);assert.equal(node.classList.contains('graph-workspace-dragging'),false);
});
test('keyboard resize starts at the actual constrained divider without passively changing saved preferences',()=>{
 const h=harness(),node=h.render(),divider=node.querySelector('[data-graph-divider]');h.context.state.graphWorkspacePreferences={ratio:.6};
 node.querySelector('[data-graph-pane]').getBoundingClientRect=()=>({height:260});node.querySelector('[data-graph-results]').getBoundingClientRect=()=>({height:240});
 h.api.restore();assert.equal(divider.getAttribute('aria-valuenow'),'52');assert.equal(h.context.state.graphWorkspacePreferences.ratio,.6);assert.equal(h.saved(),0,'Responsive minimums do not overwrite the stored choice');
 h.document.emit('keydown',divider,{key:'ArrowUp'});assert(Math.abs(h.context.state.graphWorkspacePreferences.ratio-.47)<1e-9,'The explicit step uses52percentactualminus5, not60percentsavedminus5');assert.equal(h.saved(),1);
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
 h.api.mergePreferences({graphWorkspacePreferences:{ratio:'bad'}});assert.equal(h.context.state.graphWorkspacePreferences.ratio,.4);
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
 h.context.innerHeight=650;h.api.restore();assert.equal(region.style['--graph-table-min-height'],'124px');assert.equal(node.style['--graph-list-min-height'],'206px');assert.equal(h.context.state.graphWorkspacePreferences.ratio,.6);
});
test('graph-driven result reveal uses actual filtered sort position and never clears filters',()=>{
 const h=harness(),rows=Array.from({length:240},(_,i)=>({key:'sample-'+(240-i)})),config={id:'samples',graphId:'job',graphKey:row=>row.key,rows,pageSize:100};
 const node=h.render('job','<div class="data-list-tools">Filters</div><table><thead><tr><th>Sample</th></tr></thead><tbody><tr data-result-sample-key="sample-61"><td>61</td></tr></tbody></table>');
 let refreshes=0;h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map([['samples',0]]);h.context.dataListRows=()=>rows;h.context.refreshDataList=id=>{assert.equal(id,'samples');refreshes++;};
 const results=node.querySelector('[data-graph-results]');results.scrollTop=0;results.getBoundingClientRect=()=>({top:100,bottom:300,height:200});const row=results.querySelector('[data-result-sample-key]');row.getBoundingClientRect=()=>({top:350,bottom:371,height:21});
 assert.equal(h.api.revealSample('job','sample-61'),true);assert.equal(h.context.dataListPages.get('samples'),1);assert.equal(refreshes,1);assert.equal(results.scrollTop,71);
 assert.equal(h.api.revealSample('job','filtered-out'),false);assert.equal(refreshes,1,'A filtered row does not reset the filter or page');assert.equal(h.document.querySelector('[data-graph-pane]').textContent,'Graph and point details');
});

test('collapse preserves permanent statistics, frees the plot track and persists independently per tool',()=>{
 const h=harness(),stats='<div class="result-stats"><button data-result-stat="sent">Sent30</button></div>';
 let node=h.render('ping1','RESULTS',{statistics:stats,tool:'ping'}),graph=node.querySelector('[data-graph-pane]'),summary=node.querySelector('[data-graph-statistics]');
 assert(node.innerHTML.indexOf('data-graph-statistics')<node.innerHTML.indexOf('data-graph-pane'));assert.equal(summary.textContent,'Sent30');
 h.context.state.graphWorkspacePreferences={ratio:.67};h.document.emit('click',node.querySelector('[data-graph-toggle]'));
 assert.equal(graph.hidden,true);assert.equal(node.querySelector('[data-graph-divider]').hidden,true);assert.equal(node.querySelector('[data-graph-toggle]').getAttribute('aria-expanded'),'false');assert.equal(summary.textContent,'Sent30');assert.equal(h.context.state.graphWorkspacePreferences.ratio,.67);assert.equal(h.context.state.graphWorkspacePreferences.collapsedTools.ping,true);
 node=h.render('ping2','RESULTS',{statistics:stats,tool:'ping'});assert.equal(node.querySelector('[data-graph-pane]').hasAttribute('hidden'),true);
 node=h.render('monitor1','RESULTS',{statistics:stats,tool:'monitor'});assert.equal(node.querySelector('[data-graph-pane]').hidden,undefined);assert.equal(node.querySelector('[data-graph-toggle]').textContent,'Collapse graph');
 node=h.render('ping1','RESULTS',{statistics:stats,tool:'ping'});h.api.toggle('ping1',false);assert.equal(node.querySelector('[data-graph-pane]').hidden,false);assert.equal(node.querySelector('[data-graph-divider]').hidden,false);assert.equal(node.querySelector('[data-graph-toggle]').getAttribute('aria-expanded'),'true');
 assert.equal(h.context.state.graphWorkspacePreferences.ratio,.67);assert.equal(h.saved(),2);
});
test('normalization retains only supported boolean collapse settings and keeps a saved pane ratio',()=>{
 assert.deepEqual(workspace.normalize({ratio:.61,collapsedTools:{ping:true,monitor:false,latency:'yes',other:true}}),{ratio:.61,collapsedTools:{ping:true,monitor:false}});
});
test('newest matching sample follows source sequence at its position in any sorted filtered view',()=>{
 const rows=[{sequence:1,rttMs:30},{sequence:2,rttMs:5},{sequence:3,rttMs:20},{sequence:4,rttMs:10}],config={rows,graphKey:row=>'s'+row.sequence};
 for(const sorted of [[...rows],[...rows].reverse(),[rows[1],rows[3],rows[2],rows[0]]]){const choice=workspace.newestMatching(config,sorted);assert.equal(choice.row.sequence,4);assert.equal(choice.index,sorted.indexOf(rows[3]));}
 const matching=[rows[2],rows[0]];assert.equal(workspace.newestMatching(config,matching).row.sequence,3);assert.equal(workspace.newestMatching(config,[]),null);assert.deepEqual(rows.map(row=>row.sequence),[1,2,3,4]);
 const unordered=[{key:'first'},{key:'last'},{key:'middle'}],source=[unordered[0],unordered[2],unordered[1]],choice=workspace.newestMatching({rows:source,graphKey:row=>row.key},unordered);assert.equal(choice.key,'last');assert.equal(choice.index,1);
});
test('latest result respects filtered sorted pagination without changing the preference',()=>{
 const h=harness(),rows=Array.from({length:240},(_,i)=>({sequence:i+1,key:'s'+(i+1)})),source=[...rows],matching=[...rows].reverse();matching.splice(matching.findIndex(row=>row.sequence===240),1);matching.splice(175,0,source.at(-1));
 const node=h.render('job','<div class="table-wrap"><table><thead><tr><th>Sample</th></tr></thead><tbody><tr data-result-sample-key="s240"><td>240</td></tr></tbody></table></div>'),owner=h.api.resultScroller(node),config={id:'samples',view:'sample-view',job:{id:'job'},graphId:'job',graphKey:row=>row.key,rows:source,pageSize:100};
 const preference={query:'retained',sort:'rttMs',direction:'asc',status:'all'};h.context.state.listPreferences={'sample-view':preference};h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map([['samples',0]]);h.context.dataListRows=()=>matching;let refreshes=0;h.context.refreshDataList=()=>refreshes++;
 owner.scrollTop=10;owner.getBoundingClientRect=()=>({top:100,bottom:300});owner.querySelector('[data-result-sample-key]').getBoundingClientRect=()=>({top:350,bottom:371});
 assert.equal(h.api.latest('job'),true);assert.equal(h.context.dataListPages.get('samples'),1);assert.equal(refreshes,1);assert.equal(owner.scrollTop,81);assert.equal(h.context.state.listPreferences['sample-view'],preference);
});
test('automatic latest follows new source samples but pauses inspection and resumes when inspection ends',()=>{
 const h=harness(),first={sequence:1,key:'s1'},second={sequence:2,key:'s2'},rows=[first],config={id:'samples',graphId:'job',graphKey:row=>row.key,rows,pageSize:100};
 const node=h.render('job','<div class="table-wrap"><table><tbody><tr data-result-sample-key="s2"><td>2</td></tr><tr data-result-sample-key="s1"><td>1</td></tr></tbody></table></div>'),owner=h.api.resultScroller(node);
 h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map([['samples',0]]);h.context.dataListRows=()=>[...rows].reverse();h.context.refreshDataList=()=>{};owner.scrollHeight=600;owner.clientHeight=200;
 h.api.restore();owner.scrollTop=77;h.document.emit('scroll',owner);h.api.restore();assert.equal(owner.scrollTop,77,'An unchanged poll does not reset ordinary manual scrolling');
 h.api.inspection('job',true);rows.push(second);h.api.restore();assert.equal(owner.scrollTop,77,'Active graph inspection pauses source tracking');h.api.inspection('job',false);assert.equal(owner.scrollTop,0,'End of inspection returns to the newest matching item under descending sorting');
 h.context.dataListRows=()=>[...rows];h.api.inspection('job',true,'row');owner.scrollTop=35;h.document.emit('scroll',owner);h.api.inspection('job',false,'row');assert.equal(owner.scrollTop,400,'Ascending sorting follows the newest sample at the end');
});
test('copy focus and selected result text pause automatic movement until inspection ends',()=>{
 const h=harness(),rows=[{sequence:1,key:'s1'}],config={id:'samples',graphId:'job',graphKey:row=>row.key,rows,pageSize:100};
 const node=h.render('job','<div class="table-wrap"><table><tbody><tr data-result-sample-key="s2"><td><button data-value-copy-control>Copy</button></td></tr><tr data-result-sample-key="s1"><td>1</td></tr></tbody></table></div>'),owner=h.api.resultScroller(node),copy=node.querySelector('[data-value-copy-control]');
 h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map();h.context.dataListRows=()=>[...rows].reverse();h.context.refreshDataList=()=>{};h.api.restore();owner.scrollTop=45;h.document.activeElement=copy;rows.push({sequence:2,key:'s2'});h.api.restore();assert.equal(owner.scrollTop,45);
 h.document.activeElement=null;h.document.emit('focusout',copy);assert.equal(owner.scrollTop,0);
 const text=node.querySelector('td');h.context.getSelection=()=>({isCollapsed:false,anchorNode:text});owner.scrollTop=52;rows.push({sequence:3,key:'s2'});h.api.restore();assert.equal(owner.scrollTop,52);h.context.getSelection=()=>({isCollapsed:true});h.document.emit('selectionchange',text);assert.equal(owner.scrollTop,0);
});
test('toolbar minimum counts a nested status toolbar once and includes a separate toolbar when present',()=>{
 const h=harness(),node=h.render('job','<div class="data-list-tools"><div data-result-filter-controls>Status</div></div><div class="table-wrap"><table><thead><tr><th>Sample</th></tr></thead><tbody><tr><td>Reply</td></tr></tbody></table></div><div class="data-list-foot">Pages</div>'),region=node.querySelector('[data-graph-results]');
 node.querySelector('.data-list-tools').getBoundingClientRect=()=>({height:42});node.querySelector('[data-result-filter-controls]').getBoundingClientRect=()=>({height:30});node.querySelector('.data-list-foot').getBoundingClientRect=()=>({height:28});h.context.innerHeight=650;h.api.restore();assert.equal(region.style['--graph-list-toolbar-height'],'42px');assert.equal(node.style['--graph-list-min-height'],'182px');
 const filters=node.querySelector('[data-result-filter-controls]');region.append(filters);h.api.restore();assert.equal(region.style['--graph-list-toolbar-height'],'72px');assert.equal(node.style['--graph-list-min-height'],'212px');
});

test('removing a hovered result row releases the temporary inspection without requiring pointerout',()=>{
 const h=harness(),rows=[{sequence:1,key:'s1'},{sequence:2,key:'s2'}],config={id:'samples',graphId:'job',graphKey:row=>row.key,rows,pageSize:100};
 const node=h.render('job','<div class="table-wrap"><table><tbody><tr data-result-graph-id="job" data-result-sample-key="s2"><td>2</td></tr><tr data-result-graph-id="job" data-result-sample-key="s1"><td>1</td></tr></tbody></table></div>'),owner=h.api.resultScroller(node),old=node.querySelector('[data-result-sample-key="s1"]');
 h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map();h.context.dataListRows=()=>[...rows].reverse();h.context.refreshDataList=()=>{};h.api.restore();h.document.emit('pointerover',old);owner.scrollTop=43;rows.push({sequence:3,key:'s2'});h.api.restore();assert.equal(owner.scrollTop,43);
 old.remove();h.api.restore();assert.equal(owner.scrollTop,0,'Removed DOM origins cannot leave following paused forever');
});
