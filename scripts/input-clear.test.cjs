'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test'),{document:makeDocument}=require('./terminal-dom-fixture.cjs');
const source=fs.readFileSync(require.resolve('../web/assets/input-clear.js'),'utf8');
class Event{constructor(type,options={}){this.type=type;Object.assign(this,options);}preventDefault(){this.defaultPrevented=true;}stopPropagation(){this.stopped=true;}stopImmediatePropagation(){this.stopped=true;}}
class CustomEvent extends Event{constructor(type,options={}){super(type,options);this.detail=options.detail;}}
function harness(markup,ready='loading'){
 const doc=makeDocument(markup);doc.body=doc;doc.readyState=ready;const frames=[],mutations=[],resizes=[],events=[],state={};
 function prepare(node){
  node.nodeType=1;const style=node.style,priorities={};style.getPropertyValue=key=>style[key.replace(/-([a-z])/g,(_,c)=>c.toUpperCase())]||'';style.getPropertyPriority=key=>priorities[key]||'';style.setProperty=(key,value,priority='')=>{style[key.replace(/-([a-z])/g,(_,c)=>c.toUpperCase())]=value;priorities[key]=priority;};style.removeProperty=key=>{delete style[key.replace(/-([a-z])/g,(_,c)=>c.toUpperCase())];delete priorities[key];};
  const nativeMatches=node.matches.bind(node);node.matches=selector=>selector===':disabled'?(node.disabled||node.hasAttribute('disabled')||!!node.closest('fieldset[disabled]')):nativeMatches(selector);
  node.getBoundingClientRect=()=>({left:100,right:340,top:20,bottom:52,width:240,height:32});node.getClientRects=()=>node.hidden?[]:[node.getBoundingClientRect()];node.clientWidth=240;node.offsetWidth=240;node.clientLeft=0;node.clientTop=0;
  node.focus=()=>doc.activeElement=node;node.dispatchEvent=event=>{event.target=node;for(let owner=node;owner;owner=event.bubbles&&!event.stopped?owner.parentElement:null)for(const fn of owner.listeners[event.type]||[])fn(event);return !event.defaultPrevented;};node.click=()=>node.dispatchEvent(new Event('click',{bubbles:true,button:0}));
  if(['INPUT','TEXTAREA'].includes(node.tagName))node.value=node.getAttribute('value')||node.textContent||'';return node;
 }
 for(const node of [doc,...doc.querySelectorAll('*')])prepare(node);
 const create=doc.createElement;doc.createElement=tag=>prepare(create(tag));
 const c={document:doc,Event,CustomEvent,console,requestAnimationFrame:fn=>frames.push(fn),setTimeout:fn=>frames.push(fn),addEventListener(){},getComputedStyle:node=>({position:node.style.position||(node.classList.contains('input-clear-host')?'relative':'static'),paddingRight:node.style.paddingRight||'8px'}),MutationObserver:class{constructor(fn){this.fn=fn;mutations.push(this);}observe(){}},ResizeObserver:class{constructor(fn){this.fn=fn;this.nodes=new Set();resizes.push(this);}observe(n){this.nodes.add(n);}unobserve(n){this.nodes.delete(n);}}};c.window=c;vm.createContext(c);
 doc.addEventListener('input',event=>{events.push(['input',event.target.id,event.target.value]);state[event.target.id]=event.target.value;});doc.addEventListener('change',event=>{events.push(['change',event.target.id,event.target.value]);state[event.target.id]=event.target.value;});doc.addEventListener('input-clear',event=>events.push(['input-clear',event.target.id,event.detail]));doc.addEventListener('submit',()=>events.push(['submit']));
 vm.runInContext(source,c);const api=c.NetworkToolkitInputClear;api.init();
 const flush=()=>{let count=0;while(frames.length){assert(count++<30,'clear helper scheduled an endless update loop');frames.shift()();}};flush();
 return{doc,c,api,frames,events,state,mutations,resizes,flush,prepare,button:id=>doc.querySelector('[data-input-clear-for="'+id+'"]')};
}
test('editable fields receive one accessible icon without replacing inputs or parent layout',()=>{
 const h=harness('<form><div class="field"><label for="name">Profile name</label><input id="name" value="Core switch"></div><div><textarea id="commands">show version</textarea></div></form>');const input=h.doc.getElementById('name'),parent=input.parentElement,button=h.button('name');
 assert(button);assert.equal(button.getAttribute('aria-label'),'Clear Profile name');assert.equal(button.getAttribute('type'),'button');assert.equal(button.textContent,'');assert.equal(input.parentElement,parent);assert.equal(parent.firstElementChild.tagName,'LABEL');assert.equal(parent.querySelectorAll('input').length,1);assert(h.button('commands'));
 h.api.init();h.api.refresh();h.flush();assert.equal(h.doc.querySelectorAll('[data-input-clear-for="name"]').length,1);assert.equal(h.doc.getElementById('name'),input);
});
test('clear emits normal events and an explicit clear hook, keeps focus/caret and never submits',()=>{
 const h=harness('<form><div><input id="value" value="keep no secret in events"></div></form>'),input=h.doc.getElementById('value');h.button('value').click();h.flush();
 assert.equal(input.value,'');assert.equal(h.state.value,'');assert.equal(h.doc.activeElement,input);assert.equal(input.selectionStart,0);assert.equal(input.selectionEnd,0);assert.deepEqual(h.events.map(e=>e[0]),['input','change','input-clear']);assert.deepEqual(JSON.parse(JSON.stringify(h.events[2][2])),{source:'clear-button'});assert.equal(h.button('value').hidden,true);
});
test('numbers can remain empty and passwords reserve room for existing reveal controls',()=>{
 const h=harness('<div><input id="count" type="number" value="60"></div><div><input id="secret" type="password" value="secret"><button id="reveal" type="button">Reveal</button></div>'),count=h.doc.getElementById('count'),secret=h.doc.getElementById('secret'),reveal=h.doc.getElementById('reveal');reveal.getBoundingClientRect=()=>({left:310,right:337,top:22,bottom:50,width:27,height:28});h.api.refresh();h.flush();
 count.setSelectionRange=()=>{throw new Error('Numbers do not support range selection');};h.button('count').click();h.flush();assert.equal(count.value,'');assert.equal(h.state.count,'');assert.equal(secret.getAttribute('type'),'password');assert.equal(reveal.textContent,'Reveal');assert(parseFloat(h.button('secret').style.left)<188,'clear icon must sit before the reveal control');
});
test('read-only, disabled, fieldset-disabled, hidden types and terminal/output fields are excluded',()=>{
 const h=harness('<div><input id="readonly" readonly value="output"><input id="disabled" disabled value="busy"><input id="hidden" type="hidden" value="token"><input id="file" type="file"><input id="check" type="checkbox"><input id="radio" type="radio"><input id="range" type="range"><input id="color" type="color"><select id="select"></select><textarea id="output" readonly>retained output</textarea><fieldset disabled><input id="fieldset" value="locked"></fieldset><div class="xterm"><textarea id="terminal">remote input</textarea></div><div data-output-only><textarea id="preview">output</textarea></div><input id="allowed" type="email" value="admin@example.test"></div>');
 for(const id of ['readonly','disabled','hidden','file','check','radio','range','color','select','output','fieldset','terminal','preview'])assert.equal(h.button(id),null,id+' must have no clear control');assert(h.button('allowed'));
});
test('existing target and Find clear buttons are normalized without duplicates or replacing native handlers',()=>{
 const h=harness('<div><input id="globalTarget" value="router.example"><button id="clearCurrentTarget" aria-label="Clear current target">Clear</button></div><div><input id="terminalSearchInput" type="search" value="interface"><button id="terminalSearchClear" aria-label="Clear terminal search">Clear</button></div>');let native=0;const input=h.doc.getElementById('globalTarget'),button=h.doc.getElementById('clearCurrentTarget');button.addEventListener('click',()=>{native++;input.value='';input.focus();});
 assert.equal(h.button('globalTarget'),button);assert.equal(button.textContent,'');assert.equal(button.style.position,'static');assert.equal(h.doc.querySelectorAll('[data-input-clear-for="globalTarget"]').length,1);button.click();h.api.refresh();h.flush();assert.equal(native,1);assert.equal(input.value,'');assert.equal(h.doc.activeElement,input);assert.equal(h.events.length,0,'specialized clear handlers must not be replaced with generic behavior');assert.equal(h.button('terminalSearchInput').id,'terminalSearchClear');assert(h.doc.getElementById('terminalSearchInput').classList.contains('input-clear-enhanced'),'reused search clears also suppress the native duplicate icon');
});
test('live result patches restore owned clears when the input survives but siblings and host classes do not',()=>{
 const h=harness('<div id="host"><input id="filter" value="router"></div>'),host=h.doc.getElementById('host'),input=h.doc.getElementById('filter'),button=h.button('filter');
 button.remove();host.classList.remove('input-clear-host');input.classList.remove('input-clear-enhanced');h.mutations[0].fn([{type:'childList',target:host,addedNodes:[],removedNodes:[button]}]);h.flush();
 assert.equal(h.doc.getElementById('filter'),input);assert.equal(h.button('filter'),button);assert(host.classList.contains('input-clear-host'));assert(input.classList.contains('input-clear-enhanced'));assert.equal(h.doc.querySelectorAll('[data-input-clear-for="filter"]').length,1);button.click();h.flush();assert.equal(input.value,'');assert.equal(h.state.filter,'');
 const replacement=h.prepare(h.doc.createElement('div'));replacement.id='replacement';h.doc.append(replacement);replacement.append(input);h.api.refresh();h.flush();assert.equal(button.parentElement,replacement);assert(!host.classList.contains('input-clear-host'));assert(replacement.classList.contains('input-clear-host'));
});
test('dynamic fields are enhanced and removed records are cleaned without observer feedback loops',()=>{
 const h=harness('<div id="host"><input id="first" value="one"></div>'),host=h.doc.getElementById('host'),dynamic=h.prepare(h.doc.createElement('input'));dynamic.id='second';dynamic.value='two';host.append(dynamic);h.mutations[0].fn([{type:'childList',target:host,addedNodes:[dynamic]}]);h.flush();assert(h.button('second'));h.resizes[0].fn();h.flush();assert.equal(h.frames.length,0);
 const owned=h.button('second');h.mutations[0].fn([{type:'childList',target:owned,addedNodes:owned.children}]);assert.equal(h.frames.length,0,'own icon changes must not reschedule scanning');dynamic.remove();h.api.refresh();h.flush();assert.equal(h.button('second'),null);assert(!h.resizes[0].nodes.has(dynamic));
});
test('auto initialization works once when the document is already ready and fields can opt out',()=>{
 const h=harness('<div><input id="ready" value="text"><textarea id="optout" data-input-clear="off">draft</textarea></div>','complete');assert(h.button('ready'));assert.equal(h.button('optout'),null);h.api.init();h.flush();assert.equal(h.doc.querySelectorAll('.input-clear-button').length,1);
});
