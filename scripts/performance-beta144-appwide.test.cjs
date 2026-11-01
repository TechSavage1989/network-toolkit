'use strict';
// Golden report bytes were recorded in UTC. Pin only this synthetic test process,
// preserving production local-time preferences and the original expected hashes.
process.env.TZ='UTC';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const root=path.resolve(__dirname,'..'),crypto=require('node:crypto');
const source=file=>fs.readFileSync(path.join(root,file),'utf8');
function fragment(s,start,end){const first=s.indexOf(start),last=s.indexOf(end,first);assert(first>=0&&last>first,start);return s.slice(first,last);}
function sorter(html){const source=fragment(html,'function addressSortParts(', '\nfunction dataListRows('),context={ipToNum(ip){const parts=ip.split('.').map(Number);if(parts.length!==4||parts.some(n=>!Number.isInteger(n)||n<0||n>255))throw Error('invalid IPv4');return parts.reduce((v,n)=>v*256+n,0);},NetworkToolkitResultTime:{epoch(v){return Date.parse(v)},compareInstants(a,b){return Date.parse(a)-Date.parse(b)}}};vm.createContext(context);vm.runInContext(source,context);return context;}
// The pre-Beta-8 behavior is retained as a reference so the repository's
// regression tests do not require an older source tree beside it.
function referenceSort(rows,column,pref,context){const empty=value=>context.listValueEmpty(value)||(column.type==='date'&&context.NetworkToolkitResultTime&&!Number.isFinite(context.NetworkToolkitResultTime.epoch(value)));return rows.map((row,index)=>({row,index})).sort((a,b)=>{const av=column.value(a.row),bv=column.value(b.row),ae=empty(av),be=empty(bv);if(ae||be)return ae===be?a.index-b.index:ae?1:-1;const cmp=context.compareListValues(av,bv,column.type);return cmp?(pref.direction==='desc'?-cmp:cmp):a.index-b.index;}).map(item=>item.row);}
function testRows(){return Array.from({length:5000},(_,i)=>({id:i,number:i%13===0?'Not available':(i*137)%1000,address:i%31===0?'unknown':'192.0.'+(2+Math.floor(i/255))+'.'+(i%255),text:i%37===0?'-':(i%14?'Switch-':'switch-')+String(i%87),when:i%17===0?null:new Date(Date.UTC(2026,0,1)+(i%1100)*1000).toISOString()}));}

test('Beta 8 large result sorting preserves exact order, missing-value placement, stability and input rows',()=>{
 const candidate=sorter(source('web/index.html')),rows=testRows(),snapshot=JSON.stringify(rows);
 for(const [key,type]of [['number','number'],['address','ip'],['text','text'],['when','date']])for(const direction of ['asc','desc']){
  const column={id:key,type,value:row=>row[key]},pref={sort:key,direction};
  const old=Array.from(referenceSort(rows,column,pref,candidate),r=>r.id),now=Array.from(candidate.sortDataRows(rows,[column],pref),r=>r.id);
  assert.deepEqual(now,old,key+' '+direction+' exact historical sort semantics');
 }
 assert.equal(JSON.stringify(rows),snapshot,'source rows are not modified');
});

test('Beta 8 derives each diagnostic/discovery sort key once, including large lists',()=>{
 const candidate=sorter(source('web/index.html')),rows=testRows();let oldCalls=0,newCalls=0;
 const pref={sort:'name',direction:'desc'},oldCol={id:'name',type:'text',value:row=>{oldCalls++;return row.text;}},newCol={id:'name',type:'text',value:row=>{newCalls++;return row.text;}};
 const original=Array.from(referenceSort(rows,oldCol,pref,candidate),r=>r.id),now=Array.from(candidate.sortDataRows(rows,[newCol],pref),r=>r.id);
 assert.deepEqual(now,original);assert.equal(newCalls,rows.length);assert(oldCalls>rows.length*4,'baseline comparator reads the same source value repeatedly');
});

function listHarness(html){const fragments=[fragment(html,'function dataList(', '\nfunction refreshDataList('),fragment(html,'function refreshDataList(', '\nfunction changeDataList(')];let reads=0;const area={dataset:{listView:'sample-list'},querySelector(selector){return selector==='[data-list-count]'?this.counter:selector==='[data-list-content]'?this.content:null},counter:{textContent:''},content:{innerHTML:''},innerHTML:''};const config={id:'sample-list',rows:[{id:1},{id:2}]};const ctx={dataListViews:new Map(),dataListRows(c){reads++;return c.rows;},dataListToolbar(c,count=null){return 'toolbar'+(count??ctx.dataListRows(c).length);},dataListContent(c,rows=null){return 'content'+(rows??this.dataListRows(c)).length;},dataListResetButton(){return 'reset';},NetworkToolkitResultFilters:null,esc:s=>String(s),$$:()=>[area],requestAnimationFrame(fn){},updateToolResultSticky(){},console};ctx.window=ctx;vm.createContext(ctx);for(const code of fragments)vm.runInContext(code,ctx);return{ctx,area,config,get reads(){return reads},reset(){reads=0}};}
test('Beta 8 reuses one list filtering and sorting pass for toolbar, content and count',()=>{
 const now=listHarness(source('web/index.html'));
 now.ctx.dataList(now.config);now.ctx.refreshDataList(now.config.id,false);now.ctx.refreshDataList(now.config.id,true);
 assert.equal(now.reads,3,'one pass each initial paint and refresh instead of seven prior list passes');
 assert.equal(now.area.counter.textContent,'2 matching');assert.equal(now.area.content.innerHTML,'content2');assert.equal(now.area.innerHTML,'toolbar2<div data-list-content>content2</div>');
});

function discovery(html){const code=fragment(html,'function filteredNetworkRows(', '\nfunction networkResultBody('),stats={hostnames:0,macs:0,model:0},ctx={NetworkToolkitDiscoveryFilter:require('../web/assets/discovery-filter.js'),deviceHostname(r){stats.hostnames++;return r.hostname;},rowMAC(r){stats.macs++;return r.mac;},deviceModel(r){stats.model++;return r.model||'Not reported';},macVendorLabel:r=>r.vendor||'Unknown',resolvedNeighbor:()=>true,networkSortColumns:()=>[],sortDataRows:rows=>rows,console};vm.createContext(ctx);vm.runInContext(code,ctx);return{fn:ctx.filteredNetworkRows,stats};}
test('Beta 8 avoids constructing discovery search fields for empty queries while keeping nonempty matching exact',()=>{
 const now=discovery(source('web/index.html'));
 const snapshot={rows:Array.from({length:2500},(_,i)=>({address:'192.0.2.'+i,hostname:'switch-'+i,mac:'02:00:00:11:22:33',model:'Access switch',responded:i%2===0,status:i%2===0?'responded':'no_response',openPorts:[22,443]}))},filter={query:'',respondedOnly:false,status:'all'};
 assert.equal(now.fn(snapshot,filter).length,2500);assert.equal(now.stats.hostnames,0);assert.equal(now.stats.macs,0);
 for(const [q,count]of [['switch-9',111],['192.0.2.34',1],['02:00:00:11:22:33',2500]]){filter.query=q;assert.equal(now.fn(snapshot,filter).length,count,'compiled search semantics for '+q);}
});

test('Beta 8 Ping and monitor summary is finite for very large histories without spreading argument arrays',()=>{
 const code=fragment(source('web/index.html'),'function sampleStats(', '\nfunction latencyChart('),ctx={};vm.createContext(ctx);vm.runInContext(code,ctx);
 const rows=Array.from({length:180000},(_,i)=>({success:i%7!==0,rttMs:i%31===0?'invalid':i%17}));const s=ctx.sampleStats(rows),valid=rows.filter(r=>r.success&&Number.isFinite(+r.rttMs));
 assert.equal(s.sent,rows.length);assert.equal(s.received,rows.filter(r=>r.success).length);assert.equal(s.lost,rows.length-s.received);
 assert.equal(s.minMs,Math.min(...valid.slice(0,30000).map(r=>+r.rttMs)));
 assert.equal(s.maxMs,16);assert(Number.isFinite(s.avgMs));
 assert.deepEqual(JSON.parse(JSON.stringify(ctx.sampleStats([]))),{sent:0,received:0,lost:0,lossPercent:0,minMs:null,maxMs:null,avgMs:null});
});

test('Beta 8 report HTML and text outputs remain byte-for-byte identical on large synthetic data',()=>{
 const current=require('../web/assets/report-export.js');
 const code=fragment(source('scripts/report-export.test.cjs'),'function reportFixture(count = 3) {','\nfunction deepFreeze('),context={};vm.createContext(context);vm.runInContext(code,context);
 for(const size of [3,250,1500]){
  const report=context.reportFixture(size);
  const expected={3:['12fe48ef5b9ab26517dbc14946450c74bdfe8c17fc9cfae638fa2a9e845675ad','6e6f386e7251abbfbf44615f419275f7b078f823a27f971c1d090a3db5e80949'],250:['de0ce8ff3872d3595d91b0da404e4e529fc0a179917eaa793b8640b59a23702c','3174249b9669ce7aa77c46a019e3c093fb763d22a6a8d786aa358118848f1867'],1500:['977cc115861c746f6a8ff635e87101ef5f1a52b889c47d9ec1e2512b94e65145','04fe7e584c1c393282c7952b3998288e82877c45825c13804ba2c0e87ac513b2']};
  assert.equal(crypto.createHash('sha256').update(current.toHTML(report)).digest('hex'),expected[size][0],'same baseline HTML bytes for '+size+' hosts');
  assert.equal(crypto.createHash('sha256').update(current.toText(report)).digest('hex'),expected[size][1],'same baseline text bytes for '+size+' hosts');
 }
});

test('Beta 8 app chrome avoids repeated aria/role writes without losing updated identity or counts',()=>{
 const {document:makeDocument}=require('./terminal-dom-fixture.cjs'),doc=makeDocument('<aside id="sidebar"><span id="hostTop">LAB-SW01</span><div class="brand-mark"></div><span id="runningCount">3 running</span></aside>');doc.body=doc;
 let writes=0;for(const el of [doc.getElementById('hostTop'),doc.querySelector('.brand-mark'),doc.getElementById('runningCount')]){const original=el.setAttribute.bind(el);el.setAttribute=(k,v)=>{writes++;return original(k,v);};}
 const ctx={document:doc,getComputedStyle:()=>({display:'flex',visibility:'visible'})};vm.createContext(ctx);vm.runInContext(source('web/assets/app-chrome.js'),ctx);
 ctx.NetworkToolkitAppChrome.sync();const initial=writes;assert(initial>0);
 for(let i=0;i<50;i++)ctx.NetworkToolkitAppChrome.sync();assert.equal(writes,initial,'unchanged status cannot rewrite accessibility metadata');
 doc.getElementById('hostTop').textContent='LAB-SW02';doc.getElementById('runningCount').textContent='4 running';ctx.NetworkToolkitAppChrome.sync();assert(writes>initial);assert.equal(doc.querySelector('.brand-mark').getAttribute('aria-label'),'Network Toolkit. System: LAB-SW02');assert.equal(doc.getElementById('runningCount').dataset.count,'4');
});

test('Beta 8 result statistics handle extended Ping histories without argument-list overflow',()=>{
 const api=require('../web/assets/result-filters.js'),rows=Array.from({length:140000},(_,i)=>({sequence:i+1,rttMs:i%11,success:true,status:'reply'}));
 const cfg={id:'beta144:long-monitor',view:'beta144:long-monitor',graphId:'beta144:long-monitor',job:{id:'beta144:long-monitor',tool:'monitor'},name:'samples',columns:[{id:'rttMs',value:r=>r.rttMs}],rows};
 api.register(cfg);api.setStatistic(cfg.job.id,'minimum');
 assert.equal(api.graphMatches(cfg.job.id,rows[0]),true);
 assert.equal(api.graphMatches(cfg.job.id,rows[1]),false);
 api.setStatistic(cfg.job.id,'maximum');
 assert.equal(api.graphMatches(cfg.job.id,rows[10]),true);
 assert.equal(api.graphMatches(cfg.job.id,rows[0]),false);
 api.clearStatistic(cfg.job.id);
});
