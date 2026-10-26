'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8'),lines=html.split('\n'),planner=require('../web/assets/subnet-planner.js');
function harness(input='192.0.2.17/28',compare=''){
 const document=makeDocument('<div id="utilityBody"></div>'),state={subnetInput:input,subnetPrefix:24,subnetCompare:compare,forms:{}},saved=[];
 function $(selector,owner=document){const element=owner.querySelector(selector);if(element?.tagName==='INPUT'&&!element.initializedValue){element.value=element.getAttribute('value')||'';element.initializedValue=true;}return element;}
 const c={document,state,console,$,$$:(selector,owner=document)=>owner.querySelectorAll(selector),subnetSplit:26,NetworkToolkitSubnetPlanner:planner,Number,String,Math,Array,esc:value=>String(value??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),scalar:value=>String(value??''),fmt:value=>String(value),icon:()=>'<svg></svg>',interfaces:()=>[],addresses:()=>[],saveState:()=>saved.push(JSON.parse(JSON.stringify(state))),handleLocalConfigurationInput:()=>false,handleTerminalInput:()=>false,handleLocalConfigurationChange:()=>false,handleExtendedChange:()=>false,safeToolLayout:value=>value||{},TOOLS:{},SUBNET_TOOLS:[]};c.window=c;vm.createContext(c);
 vm.runInContext(lines.find(line=>line.startsWith('const subnetPlanState=')),c);
 for(const name of ['defaults','empty','compactStats','kv','ipToNum','numToIp','classifyIP','subnetPrefixDetails','subnetCalculator','synchronizeSubnetPrefix','selectSubnetPrefix','possibleNetworksMarkup','subnetSplitMarkup','renderSubnet','subnetClassField','updateSubnetPlanner','setPossibleListControls','possibleNetworksExplanation','renderPossibleNetworks','handleSubnetPlanInput','handleSubnetPlanChange','updateSubnetSplit','calculateSubnet']){
  const source=lines.find(line=>line.startsWith('function '+name+'('));assert(source,'Missing application function '+name);vm.runInContext(source,c);
 }
 for(const type of ['input','change']){const handler=lines.find(line=>line.startsWith("document.addEventListener('"+type+"',")&&line.includes(type==='input'?'handleSubnetPlanInput':'handleSubnetPlanChange'));assert(handler,'Missing application form '+type+' handler');vm.runInContext(handler,c);}
 c.renderSubnet();
 return {c,document,state,saved,$,plan:()=>vm.runInContext('subnetPlanState',c),enter(value){$('#subnetInput').value=value;document.emit('input',$('#subnetInput'));},compare(value){$('#subnetCompare').value=value;document.emit('input',$('#subnetCompare'));}};
}
test('all IPv4 binary masks are visible alongside subnet details without a disclosure',()=>{
 const h=harness();
 for(let prefix=0;prefix<=32;prefix++){
  h.enter('203.0.113.9/'+prefix);
  const binary=h.$('#subnetBinary'),expected=('1'.repeat(prefix)+'0'.repeat(32-prefix)).match(/.{8}/g).join('.');
  assert(binary,'Binary mask is present for /'+prefix);assert.equal(binary.tagName,'DIV');assert.equal(binary.textContent,expected,'/'+prefix);assert.equal(binary.closest('details'),null);assert.equal(binary.closest('.data-grid').parentElement.id,'subnetResult');assert.equal(binary.closest('.subnet-binary-mask').querySelector('.key').textContent,'Binary subnet mask');
  assert.equal(h.$('#subnetInput').value,'203.0.113.9/'+prefix);assert.equal(h.$('#subnetPrefix').value,String(prefix));assert.equal(h.plan().selected.prefix,prefix);
 }
 assert.equal(h.document.querySelectorAll('#subnetBinary').length,1);
});
test('clearing the subnet input persists blank and removes stale details, possible networks and splits',()=>{
 const h=harness('192.0.2.17/28','192.0.2.18');
 assert.equal(h.plan().selected.cidr,'192.0.2.16/28');assert(h.$('#subnetBinary'));assert(h.$('#subnetResult').textContent.includes('belongs to'));
 h.enter('');h.document.emit('change',h.$('#subnetInput'));
 assert.equal(h.state.subnetInput,'');assert.equal(h.$('#subnetInput').value,'');assert.equal(h.state.subnetPrefix,28);assert.equal(h.$('#subnetPrefix').value,'28');assert.equal(h.$('#subnetInput').getAttribute('aria-invalid'),null);assert.equal(h.plan().selected,null);assert.equal(h.plan().result,null);assert.equal(h.$('#subnetBinary'),null);assert(h.$('#subnetResult').textContent.includes('Enter a subnet'));assert(!h.$('#subnetResult').textContent.includes('192.0.2.16'));assert(h.$('#possibleControls').disabled);assert(h.$('#possibleCopyList').disabled);assert(h.$('#possibleExportList').disabled);assert(h.$('#splitPrefix').disabled);assert.equal(h.saved.at(-1).subnetInput,'');
 h.c.defaults();h.c.renderSubnet();h.c.calculateSubnet();
 assert.equal(h.state.subnetInput,'');assert.equal(h.$('#subnetInput').value,'');assert.equal(h.plan().selected,null);assert.equal(h.$('#subnetBinary'),null);assert.equal(h.saved.at(-1).subnetInput,'');
 h.enter('198.51.100.11/31');assert.equal(h.plan().selected.cidr,'198.51.100.10/31');assert.equal(h.$('#subnetBinary').textContent,'11111111.11111111.11111111.11111110');assert(!h.$('#possibleControls').disabled);assert(!h.$('#subnetResult').textContent.includes('192.0.2.16/28'));
});
test('invalid replacement clears prior binary results and clearing comparison preserves the current subnet',()=>{
 const h=harness('192.0.2.17/28','192.0.2.18');
 h.compare('');assert.equal(h.state.subnetCompare,'');assert(!h.$('#subnetResult').textContent.includes('belongs to'));assert.equal(h.plan().selected.cidr,'192.0.2.16/28');assert(h.$('#subnetBinary'));
 h.enter('192.0.2.17/33');assert.equal(h.plan().selected,null);assert.equal(h.$('#subnetBinary'),null);assert.equal(h.$('#subnetInput').getAttribute('aria-invalid'),'true');assert(h.$('#possibleControls').disabled);assert(!h.$('#subnetResult').textContent.includes('192.0.2.16/28'));
 h.enter('');assert.equal(h.$('#subnetInput').getAttribute('aria-invalid'),null);assert.equal(h.$('#subnetBinary'),null);h.enter('192.0.2.17/32');assert.equal(h.plan().selected.cidr,'192.0.2.17/32');assert.equal(h.$('#subnetBinary').textContent,'11111111.11111111.11111111.11111111');assert(h.$('#splitPrefix').disabled);
});
