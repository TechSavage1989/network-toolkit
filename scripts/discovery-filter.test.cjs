'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const api=require('../web/assets/discovery-filter.js'),html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
const row=(address,macAddress,extra={})=>({address,mac:macAddress,hostname:'switch-'+address.split('.').at(-1),responded:true,completed:true,status:'responded',...extra});
const rows=[row('192.0.2.1','AA:BB:CC:00:00:01',{vendor:'Aruba Networks',friendlyName:'Main office',openPorts:[22]}),row('192.0.2.10','aa-bb-cc-00-00-02',{manufacturer:'Example Lab',status:'no_response',responded:false}),row('192.0.2.30','aabb.cc00.0003',{vendor:'Office Wireless',hostnames:['ap.example.test']}),row('2001:db8::1','AABBCC000004',{interfaceDescription:'South uplink'})];
function harness(){
 const context={NetworkToolkitDiscoveryFilter:api,rowMAC:r=>r.mac||'',deviceHostname:r=>r.hostname||'',deviceModel:r=>r.model||'Not reported',macVendorLabel:r=>r.vendor||'Unknown',resolvedNeighbor:r=>!!r.mac,networkSortColumns:()=>[],sortDataRows:r=>r,state:{listPreferences:{}},saveState(){},console};vm.createContext(context);
 for(const name of ['normalizeListPreference','mergeListPreferences','listPreference','saveListPreference','filteredNetworkRows']){const a=html.indexOf('function '+name+'('),b=html.indexOf('\nfunction ',a+10);vm.runInContext(html.slice(a,b),context);}return context;
}
const matches=(query,fields)=>api.compile(query).matches(fields);
test('spaces, pasted lines, tabs, commas and semicolons are OR alternatives; duplicates and empty delimiters are harmless',()=>{
 assert.deepEqual(api.terms(' alpha,\r\nbeta\tALPHA ; gamma,,, \n'),['alpha','beta','gamma']);
 for(const separator of [' ',',','\n','\r\n','\t',';']){assert(matches('alpha'+separator+'beta',['BETA device']));assert(matches('alpha'+separator+'beta',['alpha device']));assert(!matches('alpha'+separator+'beta',['other device']));}
 for(const query of ['',' , ; \r\n\t','""'])assert(matches(query,['anything']));
});
test('ordinary text remains case-insensitive substring matching, while quoted names remain one phrase',()=>{
 assert(matches('arub',['Aruba Networks']));assert(matches('uplink',['South Uplink 2']));assert(matches('"Main office",south',['Main office switch']));assert(!matches('"Main office"',['Main reception','Other office']));assert(matches('"Main   office"',['Main office switch']));assert(matches('"Main office',['Main office']));assert(matches('office wireless',['Wireless vendor']));
});
test('MAC formats compare complete normalized addresses without treating a partial prefix as a full address',()=>{
 for(const query of ['AA:BB:CC:00:00:01','aa-bb-cc-00-00-01','aabb.cc00.0001','AABBCC000001'])for(const field of ['aa:bb:cc:00:00:01','AA-BB-CC-00-00-01','aabb.cc00.0001','aabbcc000001'])assert(matches(query,[field]),query+' / '+field);
 assert(!matches('AA:BB:CC:00:00:01',['AA:BB:CC:00:00:10']));assert(matches('AA:BB',['AA:BB:CC:00:00:01']),'partial non-address text still uses substring matching');assert(!matches('aabbcc000001',['xaabbcc000001x']));
});
test('full IPv4 and IPv6 queries use normalized exact tokens, not prefix matches',()=>{
 assert(matches('192.0.2.1',['192.0.2.1']));assert(!matches('192.0.2.1',['192.0.2.10']));assert(!matches('192.0.2.1',['1192.0.2.1']));assert(matches('192.0.2.1',['Seen at (192.0.2.1) today']));assert(matches('192.0.2.',['192.0.2.10']));
 assert(matches('2001:DB8::1',['2001:0db8:0:0:0:0:0:1']));assert(!matches('2001:db8::1',['2001:db8::10']));assert(matches('[2001:db8::1]',['2001:db8::1']));assert(matches('::ffff:192.0.2.1',['::ffff:c000:201']));assert(matches('fe80::1%12',['fe80:0:0:0:0:0:0:1%12']));assert(!matches('fe80::1%12',['fe80::1%13']));
 for(const invalid of ['1.2.3.999','2001:::1','1:2:3:4:5:6:7','1:2:3:4:5:6:7:8:9'])assert.equal(api.ip(invalid),'');
});
test('the real address-row pipeline combines OR query entries with existing status and responded-only gates',()=>{
 const h=harness(),snapshot={rows,neighbor:false},filter={query:'192.0.2.1,192.0.2.10',status:'all',respondedOnly:false};
 const run=()=>Array.from(h.filteredNetworkRows(snapshot,filter),r=>r.address);assert.deepEqual(run(),['192.0.2.1','192.0.2.10']);filter.respondedOnly=true;assert.deepEqual(run(),['192.0.2.1']);filter.respondedOnly=false;filter.status='no_response';assert.deepEqual(run(),['192.0.2.10']);filter.status='open';assert.deepEqual(run(),['192.0.2.1']);filter.status='all';filter.query='aabbcc000003 "south uplink"';assert.deepEqual(run(),['192.0.2.30','2001:db8::1']);filter.query='ap.example';assert.deepEqual(run(),['192.0.2.30']);filter.query='aruba "example lab"';assert.deepEqual(run(),['192.0.2.1','192.0.2.10']);
 assert.equal(JSON.stringify(rows),JSON.stringify(snapshot.rows));
});
test('shared address results include neighbor views and every formerly searchable device field',()=>{
 const h=harness(),fields={address:'198.51.100.1',hostname:'label-name',hostnames:['alias-name'],friendlyName:'friendly-name',mac:'00:11:22:33:44:55',manufacturer:'maker-name',vendor:'vendor-name',model:'model-name',state:'cache-state',interface:'adapter-name',interfaceName:'interface-name',interfaceDescription:'interface-description',interfaceIndex:27,status:'outcome-name',icmpError:'icmp-reason',openPorts:[2222],evidence:['evidence-name'],ports:[{port:2223,service:'service-name',state:'port-state',error:'port-reason'}]};
 for(const query of ['198.51.100.1','label-name','alias-name','friendly-name','001122334455','maker-name','vendor-name','model-name','cache-state','adapter-name','interface-name','interface-description','27','outcome-name','icmp-reason','2222','evidence-name','2223','service-name','port-state','port-reason'])assert.equal(h.filteredNetworkRows({rows:[fields],neighbor:true},{query,status:'all',respondedOnly:true}).length,1,query);
});
test('hundreds of exact identifiers compile once and match without duplicate result rows',()=>{
 const query=Array.from({length:1000},(_,i)=>'02:00:'+i.toString(16).padStart(8,'0').match(/../g).join(':')).join('\n'),compiled=api.compile(query+'\n'+query),before=performance.now();
 assert.equal(compiled.count,1000);for(let i=0;i<1024;i++)assert.equal(compiled.matches(['fixture','02:00:'+i.toString(16).padStart(8,'0').match(/../g).join(':')]),i<1000);
 assert(performance.now()-before<2000,'1,024 rows × 1,000 exact MAC alternatives stay bounded and responsive');
});
test('hundreds of ordinary text alternatives remain responsive even when most rows do not match',()=>{
 const compiled=api.compile(Array.from({length:700},(_,i)=>'named-device-'+String(i).padStart(4,'0')).join(' ')),before=performance.now();assert.equal(compiled.count,700);
 for(let i=0;i<1024;i++)assert.equal(compiled.matches(['198.51.100.'+(i%250+1),'Example vendor',i%100===0?'named-device-0699':'Other synthetic host '+i,'Long adapter information '.repeat(12)]),i%100===0);
 assert(performance.now()-before<3000,'1,024 rows × 700 ordinary text alternatives stay responsive');
});
test('network preference round trips preserve large lists, while unrelated search fields retain their 300-character limit',()=>{
 const h=harness(),query=Array.from({length:700},(_,i)=>'192.0.'+Math.floor(i/250)+'.'+(i%250+1)).join('\n');h.saveListPreference('network:discover',{query,status:'open',sort:'address',direction:'desc',respondedOnly:false});const saved=h.state.listPreferences;h.mergeListPreferences(saved);const result=h.listPreference('network:discover');assert.equal(result.query,api.singleLine(query));assert.equal(result.status,'open');assert.equal(result.direction,'desc');assert.equal(h.normalizeListPreference({query},'result:dns').query.length,300);assert.equal(h.normalizeListPreference({query:'x'.repeat(40000)},'network:arp').query.length,32768);
});
test('paste reads raw clipboard lines before search-input sanitation and replaces only the selected range',()=>{
 const events=[],input={dataset:{networkFilter:'query'},value:'old tail',selectionStart:0,selectionEnd:3,setRangeText(text,start,end){this.value=this.value.slice(0,start)+text+this.value.slice(end);},dispatchEvent(event){events.push(event.type);}};let prevented=false;
 assert(api.handlePaste({target:input,clipboardData:{getData:()=> '192.0.2.1\r\n192.0.2.30\r2001:db8::1'},preventDefault(){prevented=true;}}));assert(prevented);assert.equal(input.value,'192.0.2.1 192.0.2.30 2001:db8::1 tail');assert.deepEqual(events,['input']);
 assert(!api.handlePaste({target:{dataset:{listQuery:'other'}},clipboardData:{getData:()=> 'not discovery'}}));
});
test('oversize paste is rejected rather than silently dropping identifiers or partially replacing the old filter',()=>{
 const input={dataset:{networkFilter:'query'},value:'retained',selectionStart:0,selectionEnd:8,setRangeText(){assert.fail('must not modify existing query');},dispatchEvent(){assert.fail('must not dispatch changed query');}};let prevented=false;assert(api.handlePaste({target:input,clipboardData:{getData:()=> 'x'.repeat(32769)},preventDefault(){prevented=true;}}));assert(prevented);assert.equal(input.value,'retained');
 assert.match(html,/maxlength="32768" placeholder="IPs, MACs, names/);assert.match(html,/Maximum 32,768 characters/);
});
