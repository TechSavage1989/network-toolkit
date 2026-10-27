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
 const html=workspace.html('"<unsafe>','GRAPH','RESULTS',{details:'<section>Unique recorded run data</section>'});assert.doesNotMatch(html,/data-rt-records|Run data/);assert.equal(workspace.recordsHTML('"<unsafe>'),'<section>Unique recorded run data</section>');assert(html.includes('&quot;&lt;unsafe&gt;'));assert(!html.includes('data-graph-workspace=""<'));
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
 node=h.render('monitor1','RESULTS',{statistics:stats,tool:'monitor'});assert.equal(node.querySelector('[data-graph-pane]').hidden,undefined);assert.equal(node.querySelector('[data-graph-toggle]').textContent,'Collapse');
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
 h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map([['samples',0]]);h.context.dataListRows=()=>[...rows].reverse();h.context.refreshDataList=()=>{};owner.scrollHeight=600;owner.clientHeight=200;owner.querySelector('[data-result-sample-key="s2"]').getBoundingClientRect=()=>({top:-owner.scrollTop,bottom:43-owner.scrollTop,height:43});
 h.api.restore();owner.scrollTop=77;h.document.emit('scroll',owner);h.api.restore();assert.equal(owner.scrollTop,77,'An unchanged poll does not reset ordinary manual scrolling');
 h.api.inspection('job',true);rows.push(second);h.api.restore();assert.equal(owner.scrollTop,77,'Active graph inspection pauses source tracking');h.api.inspection('job',false);assert.equal(owner.scrollTop,0,'End of inspection returns to the newest matching item under descending sorting');
 h.context.dataListRows=()=>[...rows];owner.getBoundingClientRect=()=>({top:0,bottom:200});owner.querySelector('[data-result-sample-key="s2"]').getBoundingClientRect=()=>({top:557-owner.scrollTop,bottom:600-owner.scrollTop,height:43});h.api.inspection('job',true,'row');owner.scrollTop=35;h.document.emit('scroll',owner);h.api.inspection('job',false,'row');assert.equal(owner.scrollTop,400,'Ascending sorting follows the newest sample at the end');
});
test('copy focus and selected result text pause automatic movement until inspection ends',()=>{
 const h=harness(),rows=[{sequence:1,key:'s1'}],config={id:'samples',graphId:'job',graphKey:row=>row.key,rows,pageSize:100};
 const node=h.render('job','<div class="table-wrap"><table><tbody><tr data-result-sample-key="s2"><td><button data-value-copy-control>Copy</button></td></tr><tr data-result-sample-key="s1"><td>1</td></tr></tbody></table></div>'),owner=h.api.resultScroller(node),copy=node.querySelector('[data-value-copy-control]');
 h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map();h.context.dataListRows=()=>[...rows].reverse();h.context.refreshDataList=()=>{};owner.querySelector('[data-result-sample-key="s2"]').getBoundingClientRect=()=>({top:-owner.scrollTop,bottom:30-owner.scrollTop,height:30});h.api.restore();owner.scrollTop=45;h.document.activeElement=copy;rows.push({sequence:2,key:'s2'});h.api.restore();assert.equal(owner.scrollTop,45);
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
 h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map();h.context.dataListRows=()=>[...rows].reverse();h.context.refreshDataList=()=>{};owner.querySelector('[data-result-sample-key="s2"]').getBoundingClientRect=()=>({top:-owner.scrollTop,bottom:30-owner.scrollTop,height:30});h.api.restore();h.document.emit('pointerover',old);owner.scrollTop=43;rows.push({sequence:3,key:'s2'});h.api.restore();assert.equal(owner.scrollTop,43);
 old.remove();h.api.restore();assert.equal(owner.scrollTop,0,'Removed DOM origins cannot leave following paused forever');
});

test('the selected statistic card is a stable compact Clear action across observer and live message updates',()=>{
 const h=harness(),current={statistic:'received',message:'Showing successful replies'},stats='<div class="result-stats"><button data-result-stat="received" data-result-stat-job="job" data-graph-stat-label="Received" data-graph-stat-hint="Show successful replies"><strong>3</strong><span>Received</span></button></div><div class="graph-stat-context" data-graph-stat-context>Showing successful replies</div>';
 h.context.NetworkToolkitResultFilters={current:()=>current};const node=h.render('job','RESULTS',{statistics:stats,tool:'monitor'}),context=node.querySelector('[data-graph-stat-context]'),button=node.querySelector('[data-result-stat-clear]'),label=button.querySelector('span');
 assert(button.outerHTML.includes('data-result-stat-clear=""'),'Fixture reproduces browser normalization of the selected card empty data attribute');assert.equal(label.textContent,'Received · Clear filter');assert.equal(button.querySelector('strong').textContent,'3');assert.equal(context.classList.contains('sr-only'),true);assert.equal(context.querySelector('button'),null,'No additional permanent Clear control row is created');
 let writes=0;Object.defineProperty(context,'innerHTML',{get(){return this.children.map(child=>child.outerHTML).join('');},set(){writes++;throw new Error('Normalized markup must not replace the context');}});
 for(let i=0;i<8;i++){h.api.restore();h.observe();assert.equal(node.querySelector('[data-result-stat-clear]'),button);assert.equal(button.isConnected,true);}
 h.document.activeElement=button;current.message='Showing2successful replies';h.api.restore();assert.equal(node.querySelector('[data-result-stat-clear]'),button);assert.equal(button.querySelector('span'),label);assert.equal(context.textContent,current.message);assert.equal(button.getAttribute('aria-description'),current.message);assert(button.getAttribute('title').includes(current.message));assert.equal(h.document.activeElement,button);assert.equal(writes,0);
 current.statistic='';h.api.restore();assert.equal(context.textContent,'');assert.equal(node.querySelector('[data-result-stat-clear]'),null);assert.equal(button.isConnected,true);assert.equal(button.querySelector('strong').textContent,'3');assert.equal(label.textContent,'Received');assert.equal(button.getAttribute('title'),'Show successful replies');h.api.restore();assert.equal(writes,0);
});

test('graph reveal fits the complete row inside client scrollbars, sticky headers and clipped ancestors',()=>{
 const h=harness(),target={sequence:2,key:'target'},config={id:'samples',graphId:'job',graphKey:row=>row.key,rows:[target],pageSize:100},node=h.render('job','<div class="table-wrap"><table><thead><tr><th>Sample</th></tr></thead><tbody><tr data-result-sample-key="target"><td>2</td></tr></tbody></table></div>'),owner=h.api.resultScroller(node),region=node.querySelector('[data-graph-results]'),row=owner.querySelector('[data-result-sample-key]');
 h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map([['samples',0]]);h.context.dataListRows=()=>[target];h.context.refreshDataList=()=>{};
 owner.getBoundingClientRect=()=>({top:100,bottom:320,height:220});owner.clientTop=2;owner.clientHeight=198;owner.querySelector('thead').getBoundingClientRect=()=>({height:30});owner.scrollTop=100;row.getBoundingClientRect=()=>({top:280,bottom:323,height:43});
 h.api.revealSample('job','target');assert.equal(owner.scrollTop,123,'The full43px row ends above the horizontal scrollbar at client bottom300, not border bottom320');
 region.getBoundingClientRect=()=>({top:70,bottom:292,height:222});region.clientTop=1;region.clientHeight=180;h.context.getComputedStyle=element=>({overflowY:element===region?'hidden':'visible'});h.context.innerHeight=280;owner.scrollTop=100;row.getBoundingClientRect=()=>({top:230,bottom:273,height:43});
 h.api.revealSample('job','target');assert.equal(owner.scrollTop,122,'A row is brought fully above the clipped ancestor client bottom251');
 config.rows=[{sequence:1,key:'other'},target];h.context.dataListRows=()=>config.rows;owner.scrollHeight=600;owner.scrollTop=100;h.api.latest('job');assert.equal(owner.scrollTop,122,'Ascending Latest preserves the fitted clipped position instead of overwriting it with scrollHeight-clientHeight');
 region.clientHeight=300;h.context.innerHeight=270;owner.scrollTop=100;row.getBoundingClientRect=()=>({top:250,bottom:293,height:43});h.api.revealSample('job','target');assert.equal(owner.scrollTop,123,'The window is also a real visibility boundary');
 owner.scrollTop=100;row.getBoundingClientRect=()=>({top:115,bottom:158,height:43});h.api.revealSample('job','target');assert.equal(owner.scrollTop,83,'Scrolling upward leaves the row below the sticky table heading ending at132');
});

test('live insertions keep the inspected graph or linked-row source visible while latest tracking is paused',()=>{
 const h=harness(),rows=[{sequence:1,key:'s1'},{sequence:2,key:'s2'},{sequence:3,key:'s3'}],config={id:'samples',view:'samples',graphId:'job',graphKey:row=>row.key,rows,pageSize:100},node=h.render('job','<div class="table-wrap"><table><thead><tr><th>Sample</th></tr></thead><tbody><tr data-result-graph-id="job" data-result-sample-key="s5"><td>5</td></tr><tr data-result-graph-id="job" data-result-sample-key="s4"><td>4</td></tr><tr data-result-graph-id="job" data-result-sample-key="s3"><td>3</td></tr><tr data-result-graph-id="job" data-result-sample-key="s2"><td>2</td></tr><tr data-result-graph-id="job" data-result-sample-key="s1"><td>1</td></tr></tbody></table></div>'),owner=h.api.resultScroller(node),inspected=node.querySelector('[data-result-sample-key="s2"]');
 h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map([['samples',0]]);h.context.dataListRows=()=>[...rows].reverse();h.context.refreshDataList=()=>{};h.context.state.listPreferences={samples:{sort:'sequence',direction:'desc',query:'kept',status:'all'}};const pref=h.context.state.listPreferences.samples;
 owner.scrollTop=0;owner.clientHeight=200;owner.getBoundingClientRect=()=>({top:100,bottom:300});owner.querySelector('thead').getBoundingClientRect=()=>({height:30});let insertions=0,rectReads=0;
 inspected.getBoundingClientRect=()=>{rectReads++;return {top:257+insertions*43-owner.scrollTop,bottom:300+insertions*43-owner.scrollTop,height:43};};for(const key of ['s3','s4','s5'])node.querySelector('[data-result-sample-key="'+key+'"]').getBoundingClientRect=()=>({top:130-owner.scrollTop,bottom:173-owner.scrollTop,height:43});
 h.api.inspection('job',true);assert.equal(h.api.revealSample('job','s2'),true);assert.equal(owner.scrollTop,0);
 rows.push({sequence:4,key:'s4'});insertions++;h.api.restore();assert.equal(owner.scrollTop,43,'A new descending row cannot push the inspected source below the clipped viewport');assert.equal(rectReads,2);for(let i=0;i<5;i++){h.api.restore();h.observe();}assert.equal(rectReads,2,'Unchanged observer delivery does not re-reveal or create a feedback loop');assert.equal(owner.scrollTop,43);assert.equal(h.context.state.listPreferences.samples,pref);
 h.api.inspection('job',false);assert.equal(owner.scrollTop,0,'Leaving graph inspection resumes the current source-newest result');h.api.revealSample('job','s2');h.document.emit('pointerover',inspected);rows.push({sequence:5,key:'s5'});insertions++;h.api.restore();assert.equal(owner.scrollTop,86,'A linked result row uses its own inspected key across live insertion');
 h.document.emit('pointerout',inspected);assert.equal(owner.scrollTop,0,'Leaving linked-row inspection resumes latest without changing filters');
});
test('row inspection keeps its source key and exact pixel position through a live DOM patch before pointer hit testing',()=>{
 const h=harness(),rows=[{sequence:1,key:'s1'},{sequence:2,key:'s2'},{sequence:3,key:'s3'}],config={id:'samples',graphId:'job',graphKey:row=>row.key,rows,pageSize:100},content='<div class="table-wrap"><table><thead><tr><th>Sample</th></tr></thead><tbody>'+[5,4,3,2,1].map(n=>'<tr data-result-graph-id="job" data-result-sample-key="s'+n+'"><td>'+n+'</td></tr>').join('')+'</tbody></table></div>';
 let node=h.render('job',content),owner,inspected,insertions=0;const configure=()=>{owner=h.api.resultScroller(node);owner.clientHeight=200;owner.getBoundingClientRect=()=>({top:100,bottom:300});owner.querySelector('thead').getBoundingClientRect=()=>({height:30});inspected=node.querySelector('[data-result-sample-key="s2"]');inspected.getBoundingClientRect=()=>({top:170+insertions*43-owner.scrollTop,bottom:213+insertions*43-owner.scrollTop,height:43});for(const key of ['s3','s4','s5'])node.querySelector('[data-result-sample-key="'+key+'"]').getBoundingClientRect=()=>({top:130-owner.scrollTop,bottom:173-owner.scrollTop,height:43});};configure();
 h.context.dataListViews=new Map([['samples',config]]);h.context.dataListPages=new Map([['samples',0]]);h.context.dataListRows=()=>[...rows].reverse();h.context.refreshDataList=()=>{};owner.scrollTop=0;h.document.elementFromPoint=()=>inspected;h.document.emit('pointerover',inspected,{clientX:50,clientY:180});const old=inspected;
 const next=h.api.html('job','Graph',content);rows.push({sequence:4,key:'s4'});insertions++;h.document.innerHTML=next;node=h.document.querySelector('[data-graph-workspace]');configure();h.api.restore();assert.equal(owner.scrollTop,43);assert.equal(inspected.getBoundingClientRect().top,170,'Synchronous after-patch restoration keeps the source under the stationary pointer, even while it would still fit elsewhere');
 h.document.emit('pointerout',old,{clientX:50,clientY:180,relatedTarget:node.querySelector('[data-result-sample-key="s3"]')});h.document.emit('pointerover',node.querySelector('[data-result-sample-key="s3"]'),{clientX:50,clientY:180});
 h.api.html('job','Graph',content);rows.push({sequence:5,key:'s5'});insertions++;h.api.restore();assert.equal(owner.scrollTop,86,'Queued layout hit-test events cannot replace the independently recorded source key');assert.equal(inspected.getBoundingClientRect().top,170);h.api.restore();assert.equal(owner.scrollTop,86,'The captured pixel anchor is consumed once');
 h.document.elementFromPoint=()=>null;h.document.emit('pointerout',inspected,{clientX:50,clientY:350});assert.equal(owner.scrollTop,0,'Actual pointer departure releases inspection and follows the latest source');
});
