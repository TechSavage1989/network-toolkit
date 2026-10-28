'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const picker=require('../web/assets/color-picker.js');
const {Element,document:makeDocument}=require('./terminal-dom-fixture.cjs');
const plain=value=>JSON.parse(JSON.stringify(value));
test('strict HEX and RGB parsing validates channels, alpha capability and arbitrary CSS',()=>{
 assert.equal(picker.parse(' #AAbbCC ').hex,'#aabbcc');assert.equal(picker.parse('rgb(1, 2, 255)').hex,'#0102ff');
 assert.equal(picker.parse('#10203080',true).rgb,'rgba(16, 32, 48, 0.50196)');assert.equal(picker.parse('rgba(16,32,48,0.5)',true).hex,'#10203080');
 for(const value of ['red','#fff','#12345','#1234567','#12345678','#123456789','rgb(256,0,0)','rgb(-1,2,3)','rgb(1.1,2,3)','rgb(1,2,3,1)','rgba(1,2,3)','rgba(1,2,3,0.5)','rgb(1 2 3)','rgb(1%,2%,3%)','url(secret)','var(--bad)','<script>',null,{},123])assert.equal(picker.parse(value),null,String(value));
 for(const value of ['rgba(1,2,3,1.1)','rgba(1,2,3,-0.1)','rgba(1,2,3,NaN)','rgba(1,2,3,Infinity)'])assert.equal(picker.parse(value,true),null,value);
 for(let alpha=0;alpha<256;alpha++){const color='#123456'+alpha.toString(16).padStart(2,'0');assert.equal(picker.parse(picker.parse(color,true).rgb,true).hex,color);}
});
test('HSV visual selection roundtrips channels and supports full transparent selection',()=>{
 for(const color of ['#000000','#ffffff','#ff0000','#00ff00','#0000ff','#a1b2c3','#010203','#6eabff']){const hsv=picker.rgbToHSV(color);assert.equal(picker.hsvToHex(hsv.h,hsv.s,hsv.v),color);}
 assert.equal(picker.hsvToHex(360,100,100),'#ff0000');assert.equal(picker.hsvToHex(-60,100,100),'#ff00ff');assert.equal(picker.hsvToHex(120,100,100,0),'#00ff0000');
});
test('recent and saved preferences use bounded strict unique representations only',()=>{
 const input={recent:['#ABCDEF','#abcdef','#11223380',' #445566','rgb(1,2,3)','canary',{}],saved:['#123456','#123456',null],secret:'canary'};
 const value=picker.normalizePreferences(input);assert.deepEqual(value,{recent:['#abcdef','#11223380'],saved:['#123456']});assert.deepEqual(picker.normalizePreferences(value),value);assert.deepEqual(input.recent[0],'#ABCDEF');
 const many=Array.from({length:300},(_,i)=>'#'+i.toString(16).padStart(6,'0'));assert.equal(picker.normalizePreferences({recent:many,saved:many}).recent.length,12);assert.equal(picker.normalizePreferences({recent:many,saved:many}).saved.length,24);
 assert.deepEqual(picker.normalizePreferences({saved:'bad'}),{recent:[],saved:[]});
});
test('contrast and semantic warnings are advisory and never rewrite colors',()=>{
 assert.equal(picker.contrastRatio('#000000','#ffffff'),21);assert.equal(picker.contrastRatio('#abcdef80','#ffffff'),null);
 assert(picker.advisories('#ff0000',{background:'#ff0000',semanticColors:{Warning:'#ff0000'}}).length===2);assert.deepEqual(picker.advisories('#000000',{background:'#ffffff',semanticColors:{Warning:'#ffff00'}}),[]);
 const button=picker.buttonHTML({id:'x" onfocus="bad',label:'<script>',value:'url(secret)'});assert(!button.includes('<script>'));assert(!button.includes('url(secret)'));assert(button.includes('&quot;'));assert(button.includes('#6eabff'));
});
function harness(initial={recent:[],saved:[]},copy){
 const document=makeDocument('<body><button id="opener">Choose</button><button id="background">Background</button></body>');document.body=document.querySelector('body');let prefs=plain(initial),saves=0,applies=[],previews=[],cancels=[];
 function decorate(node){node.style.setProperty=(key,value)=>{node.style[key]=value;};node.focus=()=>{document.activeElement=node;};node.getBoundingClientRect=()=>({left:10,top:10,right:510,bottom:610,width:500,height:600});for(const child of node.children)decorate(child);return node;}
 decorate(document);document.getElementById('opener').focus();
 document.createElement=tag=>{const el=decorate(new Element(tag)),descriptor=Object.getOwnPropertyDescriptor(Element.prototype,'innerHTML');Object.defineProperty(el,'innerHTML',{get(){return descriptor.get.call(this)},set(value){descriptor.set.call(this,value);decorate(this);}});el.showModal=()=>{el.open=true};el.close=()=>{el.open=false;el.emit('close',el)};return el;};
 const context={document,navigator:{clipboard:{writeText:copy||(async()=>{})}},addEventListener(){}};context.globalThis=context;vm.createContext(context);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/color-picker.js'),'utf8'),context);
 const api=context.NetworkToolkitColorPicker;api.configure({getPreferences:()=>prefs,setPreferences:value=>{prefs=plain(value);saves++;}});
 const open=(options={})=>api.open({title:'Synthetic color',value:'#112233',defaultValue:'#aabbcc',onPreview:(value,meta)=>previews.push({value,...plain(meta)}),onApply:(value,meta)=>applies.push({value,...plain(meta)}),onCancel:value=>cancels.push(value),...options});
 const dialog=()=>document.querySelector('[data-color-picker]');const get=selector=>dialog()?.querySelector(selector);
 const click=selector=>{const el=get(selector);assert(el,selector);dialog().emit('click',el);};const input=(selector,value)=>{const el=get(selector);el.value=value;dialog().emit('input',el);};
 return{api,document,open,get,click,input,dialog,applies,previews,cancels,get prefs(){return prefs},set prefs(value){prefs=value},get saves(){return saves}};
}
test('editing previews only; Cancel discards color and palette draft and restores focus',()=>{
 const h=harness({recent:['#abcdef'],saved:['#ffffff']});h.open();h.input('[data-color-value="hex"]','#334455');h.click('[data-color-save]');assert.equal(h.saves,0);assert.deepEqual(h.previews,[{value:'#334455',reset:false}]);h.click('[data-color-cancel]');
 assert.equal(h.dialog(),null);assert.equal(h.saves,0);assert.deepEqual(h.prefs,{recent:['#abcdef'],saved:['#ffffff']});assert.deepEqual(h.cancels,['#112233']);assert.deepEqual(h.applies,[]);assert.equal(h.document.activeElement.id,'opener');
});
test('Apply persists canonical chosen color and saved draft exactly once, reusable across reopen',()=>{
 const h=harness();h.open();h.input('[data-color-value="rgb"]','rgb(170, 187, 204)');assert.equal(h.get('[data-color-value="hex"]').value,'#aabbcc');h.click('[data-color-save]');h.click('[data-color-apply]');assert.equal(h.saves,1);assert.deepEqual(h.applies,[{value:'#aabbcc',reset:false}]);assert.deepEqual(h.prefs,{recent:['#aabbcc'],saved:['#aabbcc']});h.api.cancel();assert.equal(h.cancels.length,0);
 h.open();assert(h.get('[data-color-saved]').querySelector('[data-color-swatch="#aabbcc"]'));h.api.cancel();
});
test('invalid text disables Apply/save/copy without touching the previous valid preview',()=>{
 const h=harness();h.open();h.input('[data-color-value="hex"]','#fff');assert(h.get('[data-color-apply]').disabled);assert(h.get('[data-color-save]').disabled);assert(h.get('[data-color-copy="hex"]').disabled);assert.match(h.get('[data-color-error]').textContent,/Use #RRGGBB/);h.click('[data-color-apply]');assert.equal(h.saves,0);assert(h.dialog());assert.equal(h.previews.length,0);
 h.input('[data-color-value="rgb"]','rgb(1, 2, 3)');assert.equal(h.get('[data-color-apply]').disabled,false);assert.equal(h.get('[data-color-value="hex"]').value,'#010203');h.api.cancel();
});
test('Reset is a staged default and Cancel rolls it back; Apply returns explicit reset metadata',()=>{
 const h=harness();h.open();h.click('[data-color-reset]');assert.deepEqual(h.previews,[{value:'#aabbcc',reset:true}]);assert.equal(h.saves,0);h.click('[data-color-cancel]');assert.equal(h.applies.length,0);
 h.open();h.click('[data-color-reset]');h.click('[data-color-apply]');assert.deepEqual(h.applies,[{value:'#aabbcc',reset:true}]);
});
test('opaque controls reject alpha and hide translucent saved/recent colors rather than coercing',()=>{
 const h=harness({recent:['#12345680','#112233'],saved:['#abcdef00']});h.open();assert.equal(h.get('[data-color-channel="a"]'),null);assert.equal(h.get('[data-color-swatch="#12345680"]'),null);assert.equal(h.get('[data-color-swatch="#abcdef00"]'),null);h.input('[data-color-value="hex"]','#12345680');assert(h.get('[data-color-apply]').disabled);h.api.cancel();
 h.open({value:'#12345680',allowAlpha:true});assert(h.get('[data-color-channel="a"]'));assert(h.get('[data-color-swatch="#abcdef00"]'));h.input('[data-color-channel="a"]','0');h.click('[data-color-apply]');assert.equal(h.applies[0].value,'#12345600');
});
test('Escape, close, backdrop and replacement restore once; Apply does not call Cancel',()=>{
 for(const mode of ['escape','close','backdrop','replace']){const h=harness();const first=h.open();h.input('[data-color-value="hex"]','#334455');if(mode==='escape')h.document.emit('keydown',h.get('[data-color-value="hex"]'),{key:'Escape'});if(mode==='close')h.dialog().close();if(mode==='backdrop')h.dialog().emit('click',h.dialog(),{clientX:0,clientY:0});if(mode==='replace')h.open();first.close();assert.deepEqual(h.cancels,['#112233'],mode);assert.equal(h.saves,0);h.api.cancel();}
});
test('saved edits merge concurrent swatches and recent colors without replacing unrelated preferences',()=>{
 const h=harness({recent:['#111111'],saved:['#222222','#333333']});h.open();h.click('[data-color-remove="#222222"]');h.click('[data-color-save]');h.prefs={recent:['#444444'],saved:['#222222','#333333','#555555']};h.click('[data-color-apply]');assert.deepEqual(h.prefs,{recent:['#112233','#444444'],saved:['#333333','#555555','#112233']});
});
test('picker input shields underlying popup click-away and shortcuts without blocking native control defaults',()=>{
 const h=harness();h.open();
 for(const type of ['pointerdown','click','keydown']){let stopped=0,prevented=0;h.dialog().emit(type,h.get('[data-color-value="hex"]'),{key:'ArrowRight',stopPropagation(){stopped++},preventDefault(){prevented++}});assert(stopped>0,type+' propagation must stop');assert.equal(prevented,0,type+' must preserve native text/range/focus behavior');}
 h.api.cancel();
});
test('copy success and denial are explicit, with no false copied status or fallback writes',async()=>{
 const copied=[];let h=harness(undefined,async value=>copied.push(value));h.open();h.click('[data-color-copy="rgb"]');await new Promise(setImmediate);assert.deepEqual(copied,['rgb(17, 34, 51)']);assert.match(h.get('[data-color-status]').textContent,/Copied RGB/);h.api.cancel();
 h=harness(undefined,async()=>{throw Error('Denied')});h.open();h.click('[data-color-copy="hex"]');await new Promise(setImmediate);assert.match(h.get('[data-color-status]').textContent,/Copy is unavailable/);assert.equal(h.document.activeElement,h.get('[data-color-value="hex"]'));assert.equal(h.document.activeElement.selectionEnd,7);assert.equal(h.saves,0);h.api.cancel();
});
