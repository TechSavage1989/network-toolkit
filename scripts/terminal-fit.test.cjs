'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const html=fs.readFileSync(path.join(__dirname,'../web/index.html'),'utf8');
function source(name){const start=html.search(new RegExp('^function '+name+'\\(','m'));assert(start>=0);const tail=html.slice(start),next=tail.slice(1).search(/\n(?:async )?function /);return tail.slice(0,next+1);}
function setup(){let now=0,seq=0;const timers=new Map(),fits=[];const entry={info:{state:'connected'},xterm:{cols:120,rows:40,resize(cols,rows){this.cols=cols;this.rows=rows;fits.push({at:now,cols,rows});}},fit:{proposeDimensions:()=>({cols:60,rows:20})}};const c={entry,fits,state:{activePage:'terminal'},Date:{now:()=>now},sizeTerminalStage(){},terminalVisibleEntries:()=>[entry],terminalEntryVisible:()=>true,terminalOwnerCanMutate:()=>true,NetworkToolkitTerminalExtras:{beforeFit:()=>null,afterFit(){}},queueTerminalResize(){},renderTerminalChrome(){},setTimeout(fn,delay){const id=++seq;timers.set(id,{fn,at:now+Math.max(4,delay)});return id;},clearTimeout:id=>timers.delete(id)};c.window=c;c.elapse=ms=>{now+=ms;};c.advance=ms=>{const end=now+ms;for(;;){const next=[...timers].filter(([,t])=>t.at<=end).sort((a,b)=>a[1].at-b[1].at)[0];if(!next)break;now=Math.max(now,next[1].at);timers.delete(next[0]);next[1].fn();}now=Math.max(now,end);};vm.createContext(c);for(const name of ['terminalDimensions','scheduleTerminalFit','cancelTerminalFit','cancelTerminalResizeTimer'])vm.runInContext(source(name),c);return c;}
test('continuous divider/window motion fits the latest dimensions during the drag, not only after it',()=>{const c=setup();for(let i=0;i<20;i++){c.entry.fit.proposeDimensions=()=>({cols:60+i,rows:20});c.scheduleTerminalFit();c.advance(10);}assert(c.fits.length>=5,'continuous motion must not starve local reflow');assert(c.fits[0].at<=60);c.advance(60);assert.equal(c.entry.xterm.cols,79);});
test('scheduled fits respect hidden, transfer, restoration and ownership guards',()=>{for(const flag of ['transferBusy','restoring','awaitingAcceptance']){const c=setup();c.scheduleTerminalFit();c.entry[flag]=true;c.advance(100);assert.equal(c.fits.length,0);c.entry[flag]=false;c.scheduleTerminalFit();c.advance(100);assert.equal(c.fits.length,1);}for(const name of ['terminalEntryVisible','terminalOwnerCanMutate']){const c=setup();c[name]=()=>false;c.scheduleTerminalFit();c.advance(100);assert.equal(c.fits.length,0);}});
test('externally canceled fit can resume and unchanged fits do not resize xterm',()=>{const c=setup();c.scheduleTerminalFit();c.cancelTerminalFit(c.entry);c.advance(100);c.scheduleTerminalFit();c.advance(100);assert.equal(c.fits.length,1);c.scheduleTerminalFit();c.advance(100);assert.equal(c.fits.length,1);});
test('zero-size and invalid layout proposals do not replace valid geometry',()=>{for(const size of [{cols:NaN,rows:20},{cols:30,rows:Infinity},{cols:30,rows:1}]){const c=setup();c.entry.fit.proposeDimensions=()=>size;c.scheduleTerminalFit();c.advance(100);assert.equal(c.fits.length,0);}const c=setup();c.entry.element={clientWidth:0,clientHeight:100};c.scheduleTerminalFit();c.advance(100);assert.equal(c.fits.length,0);c.entry.element.clientWidth=400;c.scheduleTerminalFit();c.advance(100);assert.equal(c.fits.length,1);});
test('individual view observer survives reparenting and disconnects on display disposal',()=>{let callback,observed,disconnected=0,scheduled=0;const c={ResizeObserver:class{constructor(fn){callback=fn;}observe(node){observed=node;}disconnect(){disconnected++;}},scheduleTerminalFit(){scheduled++;}};vm.createContext(c);vm.runInContext(source('observeTerminalGeometry'),c);const entry={element:{parentElement:'first-pane'},disposables:[]};c.observeTerminalGeometry(entry);assert.equal(observed,entry.element);entry.element.parentElement='returned-pane';callback();assert.equal(scheduled,1);entry.disposables.forEach(d=>d.dispose());assert.equal(disconnected,1);});

test('backwards wall clock cannot push a fit beyond its frame-sized budget',()=>{const c=setup();c.scheduleTerminalFit();c.Date.now=()=>-3600000;c.scheduleTerminalFit();c.advance(16);assert.equal(c.fits.length,1);});

test('reentrant chrome updates cannot cancel later panes awaiting their first fit',()=>{const c=setup(),entries=Array.from({length:4},(_,i)=>({...c.entry,info:{state:'connected'},xterm:{cols:120,rows:40,resize(cols,rows){this.cols=cols;this.rows=rows;c.fits.push({pane:i,cols,rows});}}}));c.terminalVisibleEntries=()=>entries;c.renderTerminalChrome=()=>{c.elapse(10);c.scheduleTerminalFit();};c.scheduleTerminalFit();c.advance(200);assert.deepEqual([...new Set(c.fits.map(f=>f.pane))],[0,1,2,3]);});

test('unchanged dimensions skip redundant full chrome redraws',()=>{
 const c=setup();let renders=0;c.renderTerminalChrome=()=>{renders++;};
 c.scheduleTerminalFit();c.advance(100);
 assert.equal(renders,1);
 c.scheduleTerminalFit();c.advance(100);
 c.scheduleTerminalFit();c.advance(100);
 assert.equal(renders,1,'unchanged geometry should not trigger full panel rendering');
 c.entry.fit.proposeDimensions=()=>({cols:61,rows:20});
 c.scheduleTerminalFit();c.advance(100);
 assert.equal(renders,2,'a real geometry change still updates status and controls');
});

test('continuous splitter movement updates CSS tracks without rebuilding all terminal panes',()=>{
 let renders=0,fits=0,saves=0,tracks=[],aria=[];
 const grid={dataset:{layout:'columns'},style:{setProperty:(name,value)=>tracks.push([name,value])}};
 const divider={setAttribute:(name,value)=>aria.push([name,value])};
 const c={state:{terminalLayout:{kind:'columns',columnRatio:.5,rowRatio:.5}},
  safeTerminalLayout:value=>({...value}),
  $:(selector)=>selector==='#terminalPaneGrid'?grid:selector==='[data-split-axis="column"]'?divider:null,
  renderTerminalPanes:()=>{renders++;},scheduleTerminalFit:()=>{fits++;},saveState:()=>{saves++;}
 };
 vm.createContext(c);vm.runInContext(source('updateTerminalPaneRatio'),c);
 for(let i=1;i<=100;i++)c.updateTerminalPaneRatio('column',.5+i*.002);
 assert.equal(renders,0,'pointer motion must avoid full control-panel rebuilds');
 assert.equal(fits,100,'pane fitting still follows each geometric change');
 assert.deepEqual(tracks.at(-1),['--terminal-column','70%']);
 assert.deepEqual(aria.at(-1),['aria-valuenow','70']);
 c.updateTerminalPaneRatio('column',.8,true);
 assert.equal(renders,1,'keyboard and committed changes retain full state synchronization');
 assert.equal(saves,1);
});

test('phase1 beta: repeated observer events skip redundant stage measurement while fit is pending',()=>{
 const c=setup();let measurements=0;c.sizeTerminalStage=()=>{measurements++;};
 for(let i=0;i<300;i++)c.scheduleTerminalFit();
 assert.equal(measurements,1,'one pending fit should not cause 300 synchronous layout reads');
 c.advance(16);assert.equal(c.fits.length,1);
 for(let i=0;i<200;i++)c.scheduleTerminalFit();
 assert.equal(measurements,2,'the next frame may measure the latest layout again');
 c.advance(16);assert.equal(c.fits.length,1,'unchanged columns still avoid xterm reflow');
});
test('phase1 beta: visible resize refreshes highlights once and defers cosmetic chrome redraw',()=>{
 const c=setup();let refreshes=0,immediate=0,cosmetic=0;
 c.entry.highlighter={refresh(){refreshes++;}};
 c.window.requestAnimationFrame=()=>0;
 c.scheduleTerminalOutputChrome=()=>{cosmetic++;};
 c.renderTerminalChrome=()=>{immediate++;};
 c.scheduleTerminalFit();c.advance(16);
 assert.equal(c.fits.length,1);
 assert.equal(refreshes,1);
 assert.equal(cosmetic,1);
 assert.equal(immediate,0,'full chrome repaint should be deferred while dragging');
 c.scheduleTerminalFit();c.advance(16);
 assert.equal(refreshes,1,'unchanged grid must not repaint highlight decorations');
 assert.equal(cosmetic,1);
 c.entry.fit.proposeDimensions=()=>({cols:70,rows:20});
 c.scheduleTerminalFit();c.advance(16);
 assert.equal(refreshes,2);assert.equal(cosmetic,2);
});
