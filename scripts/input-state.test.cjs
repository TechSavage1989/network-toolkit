'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const lines=fs.readFileSync(require.resolve('../web/index.html'),'utf8').split('\n');
function harness(){
 const document=makeDocument('<div id="utilityBody"></div><div id="optionsBody"></div>'),state={forms:{ping:{}},bandwidth:{size:1,sizeUnit:'GB',rate:100,rateUnit:'Mbps',overhead:0}},saved=[];
 const fields={primary:[{id:'count',name:'Count',type:'number',value:4},{id:'mode',name:'Mode',type:'select',value:1,choices:[[0,'Zero'],[1,'One']]}],advanced:[{id:'enabled',name:'Enabled',type:'checkbox',value:true},{id:'name',name:'Name',value:'example'}]};
 function $(selector,owner=document){const element=owner.querySelector(selector);if(element&&!element.initializedControl&&['INPUT','SELECT'].includes(element.tagName)){element.type=element.getAttribute('type')||(element.tagName==='SELECT'?'select-one':'text');const option=element.tagName==='SELECT'?(element.querySelector('option[selected]')||element.querySelector('option')):null;element.value=option?(option.getAttribute('value')??option.textContent):(element.getAttribute('value')||'');element.initializedControl=true;}return element;}
 const c={document,state,console,$,Number,String,Math,Array,esc:value=>String(value??'').replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;'),scalar:value=>String(value??''),fmt:(value,digits=2)=>Number(value).toFixed(digits),icon:()=>'<svg></svg>',saveState:()=>saved.push(JSON.parse(JSON.stringify(state))),toolFields:()=>fields,handleSubnetPlanInput:()=>false,handleSubnetPlanChange:()=>false,handleLocalConfigurationInput:()=>false,handleTerminalInput:()=>false,handleLocalConfigurationChange:()=>false,handleExtendedChange:()=>false,updateNetworkRangeStatus(){},updateRunButtons(){},safeToolLayout:value=>value||{},TOOLS:{ping:fields},SUBNET_TOOLS:[]};c.window=c;vm.createContext(c);
 for(const name of ['defaults','kv','fieldHTML','renderBandwidth','calculateBandwidth'])vm.runInContext(lines.find(line=>line.startsWith('function '+name+'(')),c);
 for(const type of ['input','change']){const handler=lines.find(line=>line.startsWith("document.addEventListener('"+type+"',")&&line.includes(type==='input'?'handleSubnetPlanInput':'handleSubnetPlanChange'));assert(handler,'Missing application form '+type+' handler');vm.runInContext(handler,c);}
 function renderOption(id){const definition=[...fields.primary,...fields.advanced].find(field=>field.id===id);$('#optionsBody').innerHTML=c.fieldHTML('ping',definition);return $('#opt-ping-'+id);}
 function edit(id,value,types=['input','change']){const element=$('#'+id);element.value=value;for(const type of types)document.emit(type,element);return element;}
 function value(name){return $('#bandwidthResult').querySelectorAll('.data-item').find(item=>item.querySelector('.key').textContent===name)?.querySelector('.val').textContent;}
 c.renderBandwidth();
 return {c,document,state,saved,$,fields,renderOption,edit,value};
}
test('cleared numeric tool options stay blank through input, change, defaults and rerender',()=>{
 const h=harness();let count=h.renderOption('count');assert.equal(count.value,'4');
 h.edit(count.id,'');assert.equal(h.state.forms.ping.count,'');assert.equal(h.saved.at(-1).forms.ping.count,'');
 h.c.defaults();count=h.renderOption('count');assert.equal(count.value,'');assert.equal(count.getAttribute('value'),'');assert.equal(h.state.forms.ping.count,'');
 h.edit(count.id,'0');assert.equal(h.state.forms.ping.count,0);h.edit(count.id,'12');assert.equal(h.state.forms.ping.count,12);assert.equal(h.renderOption('count').value,'12');
});
test('numeric select changes preserve explicit zero and absent values without changing other field types',()=>{
 const h=harness();let mode=h.renderOption('mode');h.edit(mode.id,'0',['change']);assert.equal(h.state.forms.ping.mode,0);assert.equal(h.renderOption('mode').value,'0');
 mode=h.renderOption('mode');h.edit(mode.id,'',['change']);assert.equal(h.state.forms.ping.mode,'');assert.equal(h.saved.at(-1).forms.ping.mode,'');
 const enabled=h.renderOption('enabled');enabled.checked=false;h.document.emit('change',enabled);assert.equal(h.state.forms.ping.enabled,false);
 const name=h.renderOption('name');h.edit(name.id,'0');assert.equal(h.state.forms.ping.name,'0');
});
test('each cleared bandwidth number remains blank across rerender and replaces prior estimates with an entry prompt',()=>{
 for(const [id,key] of [['bwSize','size'],['bwRate','rate'],['bwOverhead','overhead']]){
  const h=harness();assert.equal(h.value('Exact estimate'),'80.000 seconds');
  h.edit(id,'');assert.equal(h.state.bandwidth[key],'');assert.equal(h.saved.at(-1).bandwidth[key],'');assert.equal(h.$('#bandwidthResult').querySelector('.calc-value'),null);assert(h.$('#bandwidthResult').textContent.includes('Enter a data size, link rate, and unavailable capacity percentage'));
  h.c.renderBandwidth();assert.equal(h.$('#'+id).value,'');assert.equal(h.state.bandwidth[key],'');assert.equal(h.saved.at(-1).bandwidth[key],'');assert.equal(h.$('#bandwidthResult').querySelector('.data-grid'),null);
  h.edit(id,key==='rate'?'100':key==='size'?'1':'0');assert.equal(h.value('Exact estimate'),'80.000 seconds');
 }
});
test('explicit zero size and overhead produce a valid finite estimate while a zero rate does not',()=>{
 const h=harness();h.edit('bwSize','0');h.edit('bwOverhead','0');assert.equal(h.state.bandwidth.size,0);assert.equal(h.state.bandwidth.overhead,0);assert.equal(h.value('Exact estimate'),'0.000 seconds');assert.equal(h.value('Data size'),'0 bytes');
 h.c.renderBandwidth();assert.equal(h.$('#bwSize').value,'0');assert.equal(h.$('#bwOverhead').value,'0');assert.equal(h.value('Exact estimate'),'0.000 seconds');
 h.edit('bwRate','0');assert.equal(h.state.bandwidth.rate,0);assert.equal(h.$('#bandwidthResult').querySelector('.calc-value'),null);assert(h.$('#bandwidthResult').textContent.includes('positive link rate'));
});
test('bandwidth estimates validate numeric bounds and derived overflow before displaying results',()=>{
 for(const [id,value] of [['bwSize','-1'],['bwRate','-1'],['bwOverhead','-1'],['bwOverhead','100'],['bwSize','NaN'],['bwRate','Infinity'],['bwSize','1e308'],['bwRate','1e-320']]){
  const h=harness();h.edit(id,value);const result=h.$('#bandwidthResult');assert.equal(result.querySelector('.data-grid'),null,id+'='+value);assert(result.querySelector('.notice.warn'));assert(!/NaN|Infinity/.test(result.textContent));
 }
 const h=harness();h.edit('bwSizeUnit','GiB',['change']);assert.equal(h.value('Data size'),'1073741824 bytes');assert.equal(h.value('Exact estimate'),'85.899 seconds');h.edit('bwOverhead','50');assert.equal(h.value('Exact estimate'),'171.799 seconds');assert(!/NaN|Infinity/.test(h.$('#bandwidthResult').textContent));
});
