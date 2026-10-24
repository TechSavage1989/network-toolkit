'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test'),{document:makeDocument}=require('./terminal-dom-fixture.cjs');
test('Run until stopped sends count zero and restores the previous finite count',()=>{
 const document=makeDocument('<div id="controls"></div>'),state={forms:{monitor:{count:60}}};let saves=0;const c={document,state,esc:String,saveState(){saves++;},updateRunButtons(){},TOOLS:{monitor:{primary:[{id:'count',value:60}]}},toolFields:()=>({primary:[{id:'count',value:60}],advanced:[]}),NetworkToolkitSource:{requestOptions:()=>({})}};c.window=c;vm.createContext(c);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/continuous-sampling.js'),'utf8'),c);
 const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');vm.runInContext(html.split('\n').find(line=>line.startsWith('const toolOptions=')),c);
 document.getElementById('controls').innerHTML=c.NetworkToolkitSamplingControls.field('monitor',{id:'count',name:'Samples',value:60},60,'opt-monitor-count');const number=document.getElementById('opt-monitor-count'),toggle=document.querySelector('[data-sample-unlimited]');number.value='75';toggle.checked=true;document.emit('change',toggle);assert.equal(state.forms.monitor.count,0);assert.equal(number.disabled,true);assert.equal(vm.runInContext("toolOptions('monitor').count",c),0);
 toggle.checked=false;document.emit('change',toggle);assert.equal(state.forms.monitor.count,75);assert.equal(number.disabled,false);assert.equal(saves,2);
});

for(const tool of ['ping','monitor'])test(tool+' keeps a cleared sample count blank through rendering and retains the finite count for unlimited mode',()=>{
 const document=makeDocument('<div id="controls"></div>'),definition={id:'count',name:'Samples',value:tool==='ping'?10:60},state={forms:{[tool]:{count:5,finiteCount:5}}};let saves=0,updates=0;
 const c={document,state,esc:String,saveState(){saves++;},updateRunButtons(){updates++;},TOOLS:{[tool]:{primary:[definition]}}};c.window=c;vm.createContext(c);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/continuous-sampling.js'),'utf8'),c);
 function render(){document.getElementById('controls').innerHTML=c.NetworkToolkitSamplingControls.field(tool,definition,state.forms[tool].count,'opt-'+tool+'-count');const number=document.getElementById('opt-'+tool+'-count'),toggle=document.querySelector('[data-sample-unlimited]');number.value=number.getAttribute('value');number.disabled=number.hasAttribute('disabled');toggle.checked=toggle.hasAttribute('checked');return{number,toggle};}
 let {number,toggle}=render();number.value='';document.emit('change',number);assert.equal(state.forms[tool].count,'');assert.equal(state.forms[tool].finiteCount,5);assert.equal(toggle.checked,false);assert.equal(number.disabled,false);assert.equal(number.value,'');
 ({number,toggle}=render());assert.equal(number.value,'');assert.equal(number.disabled,false);assert.equal(toggle.checked,false);toggle.checked=true;document.emit('change',toggle);assert.equal(state.forms[tool].count,0);assert.equal(number.value,'5');assert.equal(number.disabled,true);
 toggle.checked=false;document.emit('change',toggle);assert.equal(state.forms[tool].count,5);assert.equal(number.disabled,false);number.value='0';document.emit('change',number);assert.equal(state.forms[tool].count,0);assert.equal(toggle.checked,true);assert.equal(number.disabled,true);
 ({number,toggle}=render());assert.equal(number.value,'5');assert.equal(number.disabled,true);assert.equal(toggle.checked,true);assert.equal(saves,4);assert.equal(updates,4);
});

test('live sampling reports retain lifetime totals after old sample events are trimmed',()=>{
 const renderer=require('../web/assets/report-export.js'),report={version:'fixture',exported:'2026-10-22T14:00:00Z',system:{},jobs:[{id:'live',tool:'monitor',target:'router.example',status:'running',sampleStatistics:{sent:12500,received:12000,lost:500},events:[{time:'2026-10-22T14:00:00Z',data:{type:'sample',value:{sequence:12500,success:true,rttMs:1}}}]}]};
 for(const value of [renderer.toHTML(report),renderer.toText(report)]){assert(value.includes('12500'));assert(value.includes('12000'));assert(value.includes('Statistics cover the complete run.'));}
});
