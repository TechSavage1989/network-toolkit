'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test'),{document:makeDocument,Element}=require('./terminal-dom-fixture.cjs');
const resultTime=require('../web/assets/result-time.js');
function harness(initialCapture=null,config={}){
 const calls=[],copied=[],downloads=[],saved=[],doc=makeDocument('<div id="pageContent"></div>'),window={listeners:{},addEventListener(type,fn){(this.listeners[type]||=[]).push(fn);},removeEventListener(type,fn){this.listeners[type]=(this.listeners[type]||[]).filter(item=>item!==fn);},emit(type,properties={}){for(const fn of [...(this.listeners[type]||[])])fn(properties);}};doc.body=doc;let capture=initialCapture;
 const report=config.report||{packetCount:capture?.count||1,duration:1,summary:'A pattern worth checking was observed.',scope:'All retained packets in this capture.',limitations:['This capture cannot prove the complete network is healthy.'],metrics:{dnsQueries:2,dnsResponses:1,dnsErrors:1,tcpConnectionAttempts:1,tcpResets:0,icmpErrors:0,arpRequests:0},findings:[{id:'dns-errors',severity:'attention',title:'Name lookup failed',observation:'A DNS error reply was observed.',meaning:'The resolver did not return the requested answer.',nextSteps:['Check the requested name and configured DNS resolver.'],count:1,packetNumbers:[1],filter:'frame.number == 1'}]};
 const c={window,document:doc,...(config.fetch?{fetch:config.fetch}:{}),state:{activePage:'capture'},online:true,SESSION:'token',URLSearchParams,Date,Math,Object,atob,Buffer,Blob,Uint8Array,saveState:()=>saved.push(JSON.parse(JSON.stringify(c.state.packetDetails||{}))),setTimeout:()=>0,clearTimeout(){},icon:()=>'',esc:text=>String(text).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;'),NetworkToolkitTerminalInteraction:{copyValue:async text=>{copied.push(text);return true;}},download:(data,name,type)=>downloads.push({data,name,type}),api:async(path,method='GET',body)=>{
  calls.push({path,method,body});
  if(path.endsWith('/capabilities'))return config.capabilities||{available:true,note:'Windows capture',interfaces:[{id:'nics',name:'All adapters',engine:'auto'}]};
  if(path.endsWith('/interfaces')){if(config.loadAdapters)return config.loadAdapters();return {interfaces:config.interfaces||[{id:'nics',name:'All configurable network adapters',engine:'auto'},{id:'live:wifi',name:'Wi-Fi',engine:'live'},{id:'file:12',name:'Wi-Fi',engine:'file'}]};}
  if(path.endsWith('/troubleshoot'))return config.loadReport?config.loadReport():report;
  if(path.includes('/statistics?'))return config.loadStatistics?config.loadStatistics():config.statistics||{timeline:[],protocols:{},conversations:[],endpoints:[],bytes:0,duration:0,expert:0};
  if(method==='POST'){capture={id:'a'.repeat(24),name:'Packet capture',state:'running',engine:'live',count:1,bytes:54,lost:0,overwritten:0,revision:1};return capture;}
  if(/\/packet\/\d+$/.test(path)){const number=Number(path.split('/').at(-1));return (typeof config.detail==='function'?config.detail(number):config.detail)||{packet:{number,protocol:'TCP',tcpFlags:2,source:'192.0.2.1',destination:'198.51.100.2',destinationPort:443,expert:[]},layers:Array.from({length:5},(_,i)=>({name:'Layer '+i,offset:0,length:8,fields:[{name:'Long address',value:'2001:db8:1234:5678:'.repeat(30),offset:0,length:8}]})),data:Buffer.from(Array.from({length:512},(_,i)=>i%256)).toString('base64'),ascii:'PAYLOAD'.repeat(200)};}
  if(path.includes('/overview?'))return config.loadOverview?config.loadOverview():config.overview||{bins:[],packetMarks:'',signals:[],interesting:0,counts:{error:0,warn:0,note:0}};
  if(path.includes('/packets?')){const parsed=new URLSearchParams(path.split('?')[1]);if(config.loadPackets)return config.loadPackets(parsed);const params=parsed,offset=Number(params.get('offset')||0),limit=Number(params.get('limit')||300),packets=config.packets||[{number:1,relative:0,source:'192.0.2.1',destination:'198.51.100.2',protocol:'TCP',length:54,captured:54,info:'SYN',expert:[]}];return {capture,total:config.total??packets.length,offset,packets:config.paginate?packets.slice(offset,offset+limit):packets,overview:config.overview};}
  return {captures:capture?[capture]:[]};
 }};
 vm.runInNewContext(fs.readFileSync(require.resolve('../web/assets/panel-controls.js'),'utf8'),c);window.NetworkToolkitPanelControls=c.NetworkToolkitPanelControls;
 vm.runInNewContext(fs.readFileSync(require.resolve('../web/assets/result-time.js'),'utf8'),c);
 vm.runInNewContext(fs.readFileSync(require.resolve('../web/assets/packet-capture.js'),'utf8'),c);const api=window.NetworkToolkitPacketCapture;api.render();
 const settle=async()=>{for(let i=0;i<40;i++)await Promise.resolve();};
 return {api,doc,c,calls,copied,downloads,saved,settle,setCapture:next=>capture=next};
}
test('opening capture automatically loads actual adapters, retains selection and reloads after leaving the tool',async()=>{
 const h=harness();await h.settle();assert.equal(h.calls.filter(c=>c.path==='/packet-captures/interfaces').length,1);assert.match(h.doc.getElementById('packetInterface').textContent,/Wi-Fi/);assert.equal(h.doc.querySelector('[data-packet-action="interfaces"]').textContent,'Refresh adapters');
 h.doc.getElementById('packetInterface').value='live:wifi';h.doc.emit('click',h.doc.querySelector('[data-packet-action="interfaces"]'));await h.settle();assert.equal(h.doc.getElementById('packetInterface').value,'live:wifi');assert.equal(h.calls.filter(c=>c.path==='/packet-captures/interfaces').length,2);
 h.api.render();await h.settle();assert.equal(h.calls.filter(c=>c.path==='/packet-captures/interfaces').length,2,'rerendering the same tool requested another administrator prompt');
 h.doc.getElementById('pageContent').innerHTML='';h.api.render();await h.settle();assert.equal(h.calls.filter(c=>c.path==='/packet-captures/interfaces').length,3);
});
test('adapter loading is deduplicated and failures remain visible while capture refreshes',async()=>{
 let resolve,reject;const h=harness(null,{loadAdapters:()=>new Promise((yes,no)=>{resolve=yes;reject=no;})});await h.settle();assert.equal(h.doc.getElementById('packetInterface').disabled,true);assert.equal(h.doc.querySelector('[data-packet-action="start"]').disabled,true);h.doc.emit('click',h.doc.querySelector('[data-packet-action="interfaces"]'));assert.equal(h.calls.filter(c=>c.path.endsWith('/interfaces')).length,1);
 reject(new Error('Administrator prompt cancelled'));await h.settle();assert.match(h.doc.getElementById('packetAdapterStatus').textContent,/Administrator prompt cancelled/);await h.api.refresh(true);assert.match(h.doc.getElementById('packetAdapterStatus').textContent,/Administrator prompt cancelled/);
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="interfaces"]'));await h.settle();resolve({interfaces:[]});await h.settle();assert.equal(h.doc.getElementById('packetInterface').value,'');assert.equal(h.doc.querySelector('[data-packet-action="start"]').disabled,true);assert.match(h.doc.getElementById('packetAdapterStatus').textContent,/No configurable adapters/);
 h.api.render();await h.settle();assert.equal(h.doc.getElementById('packetInterface').value,'','an empty validated adapter list was replaced with the initial capability placeholder');assert.equal(h.doc.querySelector('[data-packet-action="start"]').disabled,true);
});
test('unsupported systems retain capture import without requesting adapter enumeration',async()=>{
 const h=harness(null,{capabilities:{available:false,note:'Import is available',interfaces:[]}});await h.settle();assert.equal(h.calls.filter(c=>c.path.endsWith('/interfaces')).length,0);assert.equal(h.doc.querySelector('[data-packet-action="start"]').disabled,true);assert(h.doc.querySelector('[data-packet-action="import"]'));
});
test('capture screen starts a configured recording, lists packets and applies filters through its visible controls',async()=>{
 const h=harness();await h.settle();for(const action of ['start','stop','import','clear','export','export-all'])assert(h.doc.querySelector('[data-packet-action="'+action+'"]'));
 for(const [id,value]of Object.entries({packetEngine:'live',packetInterface:'nics',packetSnapLen:'9000',packetDuration:'60',packetMaxMB:'32',packetMaxPackets:'50000',packetRing:'stop',packetKeepProtocol:'tcp',packetKeepHost:'192.0.2.0/24',packetKeepPorts:'22,443',packetKeepMAC:'',packetKeepVLAN:'',packetDirection:'any'})){const el=h.doc.getElementById(id);el.value=value;el.checkValidity=()=>true;}
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="start"]'));await h.settle();const start=h.calls.find(c=>c.method==='POST'&&c.path==='/packet-captures');assert.equal(start.body.filter.host,'192.0.2.0/24');assert.deepEqual([...start.body.filter.ports],[22,443]);assert.equal(start.body.duration,60);assert(h.doc.getElementById('packetTroubleshooting'));h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();assert(h.doc.querySelector('[data-packet-number="1"]'));assert.equal(h.doc.querySelector('[data-packet-action="stop"]').disabled,false);assert.equal(h.doc.querySelector('[data-packet-action="start"]').disabled,true);
 h.doc.getElementById('packetDisplayFilter').value='tcp.flags.syn';h.doc.getElementById('packetSearch').value='192.0.2.1';h.doc.emit('click',h.doc.querySelector('[data-packet-action="filter"]'));await h.settle();assert(h.calls.some(c=>c.path.includes('filter=tcp.flags.syn')&&c.path.includes('query=192.0.2.1')));h.doc.getElementById('packetKeepPorts').value='1; invalid';assert.throws(()=>h.api.optionRequest(),/Keep ports/);
});
test('live refresh retains the details pane, scroll position, expanded fields and narrow hex rows',async()=>{
 const h=harness({id:'a'.repeat(24),name:'Packet capture',state:'running',engine:'live',count:1,bytes:512,lost:0,overwritten:0});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();
 const inspector=h.doc.getElementById('packetInspector'),body=h.doc.getElementById('packetDetailScroll'),hex=h.doc.getElementById('packetBytes'),sections=inspector.querySelectorAll('.packet-layer-tree details');assert(body);assert.equal(body.closest('.packet-inspector'),inspector);assert.equal(inspector.querySelector('.packet-detail-heading').closest('.packet-detail-scroll'),null);assert.equal(hex.querySelector('.packet-byte-hex').querySelectorAll('span').length,8);assert.equal(hex.children.length,64);
 body.scrollTop=180;hex.scrollLeft=24;sections[4].setAttribute('open','');const field=sections[4].querySelector('.packet-field');
 for(let i=0;i<3;i++)await h.api.refresh(true);
 assert.equal(h.doc.getElementById('packetInspector'),inspector);assert.equal(h.doc.getElementById('packetDetailScroll'),body);assert.equal(body.scrollTop,180);assert.equal(h.doc.getElementById('packetBytes'),hex);assert.equal(hex.scrollLeft,24);assert.equal(sections[4].getAttribute('open'),'');assert.equal(sections[4].querySelector('.packet-field'),field);
 h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();assert.equal(h.doc.getElementById('packetDetailScroll'),body,'reselecting the same packet reset the reader');
});
test('guided setup starts with the troubleshooting task and explains capture visibility without hiding basic controls',async()=>{
 const h=harness();await h.settle();
 assert.equal(h.doc.querySelector('[data-packet-view="troubleshooting"]').getAttribute('aria-selected'),'true');
 const setup=h.doc.getElementById('packetCaptureSetup');assert(setup.hasAttribute('open'));assert(h.doc.getElementById('packetTask'));assert.equal(h.doc.getElementById('packetInterface').closest('details'),setup);assert.equal(h.doc.getElementById('packetDuration').closest('details'),setup);
 const advanced=h.doc.getElementById('packetAdvancedCapture');assert.equal(advanced.hasAttribute('open'),false);assert.equal(h.doc.getElementById('packetKeepPorts').closest('details'),advanced);
 assert.match(h.doc.querySelector('.packet-adapter-hint').textContent,/not every device/);assert.match(h.doc.querySelector('.packet-workflow').textContent,/Reproduce the problem/);assert.match(h.doc.querySelector('.packet-welcome').textContent,/Start with the problem/);
 h.doc.getElementById('packetKeepHost').value='192.0.2.0/24';h.doc.getElementById('packetTask').value='dns';h.doc.emit('change',h.doc.getElementById('packetTask'));
 assert.match(h.doc.getElementById('packetTaskHint').textContent,/Cached answers or encrypted DNS/);assert.equal(h.doc.getElementById('packetKeepHost').value,'192.0.2.0/24');assert.equal(h.calls.filter(call=>call.method==='POST'&&!call.path.endsWith('/interfaces')).length,0);
});
test('troubleshooting reviews all retained packets, escapes evidence text and reuses unchanged reports',async()=>{
 const capture={id:'a'.repeat(24),name:'Imported capture',state:'completed',count:250,bytes:1000,revision:1};const report={packetCount:250,duration:5,summary:'A pattern was observed.',scope:'All 250 retained packets.',metrics:{dnsQueries:4,dnsResponses:3,tcpConnectionAttempts:6,tcpResets:1,icmpErrors:0},limitations:['An absent reply does not prove a device is down.'],findings:[{id:'dns',severity:'attention',title:'<script>untrusted</script>',observation:'No response observed for <example.test>.',meaning:'The answer may be outside the capture.',nextSteps:['Check <DNS> settings.'],count:12,packetNumbers:[1],filter:'frame.number == 1'}]};
 const h=harness(capture,{report});await h.settle();const first=h.doc.getElementById('packetTroubleshooting');assert(first);assert.equal(first.querySelector('script'),null);assert.match(first.textContent,/Packet-list search and display filters do not change this review/);assert.match(first.textContent,/250/);assert.match(first.textContent,/1 example from 12 observations/);
 first.scrollTop=150;await h.api.refresh();assert.equal(h.doc.getElementById('packetTroubleshooting'),first);assert.equal(first.scrollTop,150);assert.equal(h.calls.filter(c=>c.path.endsWith('/troubleshoot')).length,1);
 h.api.model.expression='dns';h.api.model.query='hidden';await h.api.refresh();assert.equal(h.calls.filter(c=>c.path.endsWith('/troubleshoot')).length,1);
 first.querySelector('.packet-glossary').setAttribute('open','');const oldFocus=Element.prototype.focus;let restoredFocus;h.doc.activeElement=first.querySelector('[data-packet-action="copy-review"]');Element.prototype.focus=function(){restoredFocus=this;};
 try{report.summary='Updated live review';h.setCapture({...capture,count:251,revision:2});await h.api.refresh();assert.equal(h.calls.filter(c=>c.path.endsWith('/troubleshoot')).length,2);assert.equal(h.doc.getElementById('packetTroubleshooting'),first);assert.equal(first.scrollTop,150);assert(first.querySelector('.packet-glossary').hasAttribute('open'));assert.match(first.textContent,/Updated live review/);assert.equal(restoredFocus,first.querySelector('[data-packet-action="copy-review"]'));}finally{Element.prototype.focus=oldFocus;}
 assert(h.calls.filter(c=>c.path.includes('/troubleshoot')).every(c=>!c.path.includes('?')));
});
test('setup folds once after a capture is selected, can be reopened, and a different capture starts its review at the top',async()=>{
 const capture={id:'a'.repeat(24),name:'Capture',state:'completed',count:1,bytes:54,revision:1},h=harness(capture);await h.settle();const setup=h.doc.getElementById('packetCaptureSetup');assert.equal(setup.hasAttribute('open'),false);setup.setAttribute('open','');await h.api.refresh();assert(setup.hasAttribute('open'),'polling refolded a manually reopened guide');h.doc.getElementById('packetTroubleshooting').scrollTop=180;
 h.setCapture({...capture,id:'b'.repeat(24)});await h.api.refresh();assert.equal(setup.hasAttribute('open'),false);assert.equal(h.doc.getElementById('packetTroubleshooting').scrollTop,0);assert.equal(h.api.model.selected,'b'.repeat(24));
});
test('choosing New capture retains the empty guided setup through polling and an existing recording can be reopened',async()=>{
 const capture={id:'a'.repeat(24),name:'Retained recording',state:'completed',count:1,bytes:54,revision:1},h=harness(capture);await h.settle();assert.equal(h.api.model.selected,capture.id);
 const picker=h.doc.getElementById('packetCaptureSelect');picker.value='';h.doc.emit('change',picker);await h.settle();assert.equal(h.api.model.selected,'');assert.equal(h.api.model.view,'troubleshooting');assert(h.doc.getElementById('packetCaptureSetup').hasAttribute('open'));assert(h.doc.querySelector('.packet-welcome'));assert.equal(h.doc.getElementById('packetTroubleshooting'),null);
 await h.api.refresh();assert.equal(h.api.model.selected,'');assert.equal(picker.value,'');assert(h.doc.getElementById('packetCaptureSetup').hasAttribute('open'));
 picker.value=capture.id;h.doc.emit('change',picker);await h.settle();assert.equal(h.api.model.selected,capture.id);assert.equal(h.api.model.newCapture,false);assert(h.doc.getElementById('packetTroubleshooting'));assert.equal(h.doc.getElementById('packetCaptureSetup').hasAttribute('open'),false);
});
test('finding examples clear unrelated display search and select retained evidence without changing the recording',async()=>{
 const h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:200,bytes:1000,revision:1});await h.settle();h.doc.getElementById('packetSearch').value='unrelated';h.api.model.query='unrelated';
 h.doc.emit('click',h.doc.querySelector('[data-packet-finding="0"]'));await h.settle();
 assert.equal(h.api.model.view,'packets');assert.equal(h.doc.querySelector('[data-packet-view="packets"]').getAttribute('aria-selected'),'true');assert.equal(h.api.model.query,'');assert.equal(h.doc.getElementById('packetDisplayFilter').value,'frame.number == 1');assert(h.calls.some(c=>c.path.includes('/packets?')&&c.path.includes('filter=frame.number+%3D%3D+1')));assert(h.calls.some(c=>c.path.endsWith('/packet/1')));assert(h.doc.querySelector('.packet-plain-explanation'));assert.equal(h.calls.filter(c=>c.method==='POST'&&!c.path.endsWith('/interfaces')).length,0);
});
test('task focus filters what is inspected and report copy/save retain plain findings and limits',async()=>{
 const h=harness({id:'a'.repeat(24),name:'Capture for review',state:'completed',count:1,bytes:100,revision:1});await h.settle();
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="copy-review"]'));h.doc.emit('click',h.doc.querySelector('[data-packet-action="save-review"]'));await h.settle();
 assert.equal(h.copied.length,1);assert.match(h.copied[0],/Capture for review/);assert.match(h.copied[0],/What to check next:/);assert.match(h.copied[0],/cannot prove the complete network is healthy/);assert.match(h.copied[0],/Example packet numbers: 1/);assert.equal(h.downloads[0].data,h.copied[0]);assert.match(h.downloads[0].name,/Troubleshooting_.*\.txt$/);
 h.doc.getElementById('packetKeepPorts').value='443';h.doc.getElementById('packetTask').value='dns';h.doc.emit('change',h.doc.getElementById('packetTask'));h.doc.emit('click',h.doc.querySelector('[data-packet-action="goal-filter"]'));await h.settle();assert.equal(h.api.model.expression,'dns');assert.equal(h.doc.getElementById('packetKeepPorts').value,'443');assert.equal(h.api.model.view,'packets');
});
test('packet interpretation distinguishes a reset from a proven fault and preserves technical details',async()=>{
 const h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:1,bytes:54,revision:1},{detail:{packet:{number:1,protocol:'TCP',tcpFlags:20,source:'192.0.2.1',destination:'198.51.100.2',expert:[]},layers:[{name:'TCP',offset:0,length:20,fields:[]}],data:Buffer.from('payload').toString('base64'),ascii:'payload'}});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();
 const text=h.doc.querySelector('.packet-plain-explanation').textContent;assert.match(text,/sent a reset to end or reject/);assert.match(text,/reset alone does not identify the cause/);assert(h.doc.querySelector('.packet-layer-tree details'));assert(h.doc.getElementById('packetBytes'));
});
test('a late review response cannot replace the packet view chosen while it was loading',async()=>{
 let resolve;const h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:1,bytes:54,revision:1},{loadReport:()=>new Promise(yes=>resolve=yes)});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));resolve({packetCount:1,summary:'late',findings:[],metrics:{},limitations:[]});await h.settle();assert.equal(h.api.model.report,null);await h.api.refresh(true);assert(h.doc.querySelector('[data-packet-number="1"]'));assert.equal(h.doc.getElementById('packetTroubleshooting'),null);
});
test('manual Quick guide and setup choices survive welcome polls, capture changes, tab changes and tool rerenders',async()=>{
 const h=harness();await h.settle();const welcome=h.doc.querySelector('.packet-welcome'),guide=welcome.querySelector('.packet-glossary'),setup=h.doc.getElementById('packetCaptureSetup');
 h.doc.emit('click',guide.querySelector('summary'));h.doc.emit('click',setup.querySelector('summary'));welcome.scrollTop=90;assert(guide.hasAttribute('open'));assert.equal(setup.hasAttribute('open'),false);
 for(let i=0;i<3;i++)await h.api.refresh();assert.equal(h.doc.querySelector('.packet-welcome'),welcome);assert.equal(welcome.scrollTop,90);assert.equal(welcome.querySelector('.packet-glossary'),guide);assert(guide.hasAttribute('open'));assert.equal(setup.hasAttribute('open'),false);
 h.doc.emit('click',setup.querySelector('summary'));const capture={id:'b'.repeat(24),name:'Imported',state:'completed',count:1,bytes:54,revision:1};h.setCapture(capture);await h.api.refresh();assert(setup.hasAttribute('open'),'a capture change closed a manually opened setup');assert(h.doc.querySelector('#packetTroubleshooting .packet-glossary').hasAttribute('open'));
 h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="troubleshooting"]'));await h.settle();assert(h.doc.querySelector('#packetTroubleshooting .packet-glossary').hasAttribute('open'));
 h.api.render();await h.settle();assert(h.doc.getElementById('packetCaptureSetup').hasAttribute('open'));assert(h.doc.querySelector('#packetTroubleshooting .packet-glossary').hasAttribute('open'));
 h.doc.emit('click',h.doc.querySelector('#packetTroubleshooting .packet-glossary summary'));h.doc.emit('click',h.doc.querySelector('#packetCaptureSetup summary'));h.setCapture({...capture,id:'c'.repeat(24),revision:2});await h.api.refresh();h.api.render();await h.settle();assert.equal(h.doc.getElementById('packetCaptureSetup').hasAttribute('open'),false);assert.equal(h.doc.querySelector('#packetTroubleshooting .packet-glossary').hasAttribute('open'),false);
});
test('ordinary UDP payload layers with null fields render complete details and exact captured bytes',async()=>{
 const payload=Buffer.from('ordinary UDP payload\x00\xff'),detail={packet:{number:1,protocol:'UDP',source:'192.0.2.1',sourcePort:49152,destination:'198.51.100.2',destinationPort:5082,expert:null},layers:[{name:'UDP',offset:0,length:8,fields:[{name:'Destination port',value:'5082',offset:2,length:2}]},{name:'Payload',offset:8,length:payload.length,fields:null}],data:payload.toString('base64'),ascii:null};
 const h=harness({id:'a'.repeat(24),name:'UDP capture',state:'completed',count:1,bytes:54,revision:1},{detail});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();
 assert.match(h.doc.querySelector('.packet-plain-explanation').textContent,/without a TCP-style/);assert.match(h.doc.querySelector('.packet-layer-tree').textContent,/Payload/);assert.match(h.doc.querySelector('.packet-layer-tree').textContent,/No additional fields were decoded/);assert.match(h.doc.querySelector('.packet-ascii-section').textContent,/ordinary UDP payload/);assert.equal(h.doc.querySelectorAll('.packet-byte-hex span').length,payload.length);assert.equal(h.doc.getElementById('packetCaptureError').textContent,'');
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="save-bytes"]'));assert.equal(h.downloads.length,1);assert.deepEqual(Buffer.from(await h.downloads[0].data.arrayBuffer()),payload);const reader=h.doc.getElementById('packetDetailScroll');await h.api.refresh(true);assert.equal(h.doc.getElementById('packetDetailScroll'),reader);
});
test('empty or optional decoder detail fields still provide a usable packet inspector',async()=>{
 for(const fields of [{layers:null,ascii:null,data:null},{layers:[null,{name:'Payload',fields:null}],ascii:undefined,data:''}]){
  const h=harness({id:'a'.repeat(24),name:'Empty UDP',state:'completed',count:1,bytes:0,revision:1},{detail:{packet:{number:1,protocol:'UDP',source:'192.0.2.1',destination:'198.51.100.2'},...fields}});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();assert(h.doc.querySelector('.packet-plain-explanation'));assert.match(h.doc.getElementById('packetBytes').textContent,/no captured bytes/);assert.equal(h.doc.getElementById('packetCaptureError').textContent,'');
 }
});
test('packet details resize and collapse with shared controls while retaining selection and the mounted reader',async()=>{
 const h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:1,bytes:54,revision:1});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();
 const area=h.doc.getElementById('packetCaptureAnalysis'),divider=h.doc.getElementById('packetDetailsDivider'),reader=h.doc.getElementById('packetDetailScroll'),hide=h.doc.querySelector('[data-packet-action="hide-details"]'),rail=h.doc.getElementById('packetDetailsRail');area.getBoundingClientRect=()=>({width:1200,height:600});h.api.layoutDetails();reader.scrollTop=200;
 assert.equal(divider.getAttribute('aria-orientation'),'vertical');assert.equal(divider.getAttribute('aria-valuenow'),'360');assert.equal(hide.getAttribute('aria-label'),'Hide packet details');assert(hide.querySelector('[data-panel-direction="right"]'));assert.equal(hide.textContent,'');
 h.doc.emit('pointerdown',divider,{button:0,clientX:840,pointerId:1});h.c.window.emit('pointermove',{clientX:760,pointerId:2});assert.equal(h.c.state.packetDetails.width,360);h.c.window.emit('pointermove',{clientX:760,pointerId:1});assert.equal(h.c.state.packetDetails.width,440);h.c.window.emit('pointercancel');assert.equal(divider.classList.contains('dragging'),false);assert.equal(h.doc.classList.contains('resizing-columns'),false);assert.equal(h.saved.at(-1).width,440);h.c.window.emit('pointermove',{clientX:0,pointerId:1});assert.equal(h.c.state.packetDetails.width,440,'pointer cleanup left a live drag handler');
 h.doc.emit('keydown',divider,{key:'ArrowLeft'});assert.equal(h.c.state.packetDetails.width,450);h.doc.emit('keydown',divider,{key:'ArrowRight',shiftKey:true});assert.equal(h.c.state.packetDetails.width,420);h.doc.emit('keydown',divider,{key:'Home'});assert.equal(h.c.state.packetDetails.width,360);
 h.doc.emit('click',hide);assert.equal(h.c.state.packetDetails.hidden,true);assert.equal(h.doc.getElementById('packetInspector').hidden,true);assert.equal(rail.hidden,false);assert(rail.querySelector('[data-panel-direction="left"]'));await h.api.refresh();assert.equal(h.doc.getElementById('packetDetailScroll'),reader);assert.equal(reader.scrollTop,200);assert.equal(h.api.model.detail.packet.number,1);assert.equal(h.c.state.packetDetails.width,360);
 h.doc.emit('click',rail.querySelector('button'));assert.equal(h.c.state.packetDetails.hidden,false);assert.equal(divider.getAttribute('aria-valuenow'),'360');h.doc.emit('dblclick',divider);assert.equal(h.c.state.packetDetails.hidden,true);h.doc.emit('keydown',divider,{key:'Enter'});assert.equal(h.c.state.packetDetails.hidden,false);h.doc.emit('keydown',divider,{key:' '});assert.equal(h.c.state.packetDetails.hidden,true);
 const oldFocus=Element.prototype.focus,focused=[];h.doc.activeElement=divider;Element.prototype.focus=function(){focused.push(this);h.doc.activeElement=this;};try{h.doc.emit('keydown',divider,{key:'Enter'});assert.equal(h.doc.activeElement,divider);h.doc.emit('keydown',divider,{key:'Enter'});assert.equal(h.doc.activeElement,divider);h.doc.emit('dblclick',divider);assert.equal(h.doc.activeElement,divider);assert.equal(focused.length,0,'divider toggles moved keyboard focus away from the divider');h.doc.emit('click',hide);assert.equal(h.doc.activeElement,rail.querySelector('button'),'button hide did not move focus to the adjacent restore control');}finally{Element.prototype.focus=oldFocus;}
 h.doc.emit('click',h.doc.querySelector('[data-packet-view="troubleshooting"]'));await h.settle();assert(h.doc.getElementById('packetTroubleshooting'));assert.equal(area.style.gridTemplateColumns,'');assert.equal(area.style.gridTemplateRows,'');assert.equal(h.doc.getElementById('packetDetailsRail'),null);
});
test('narrow packet details dock below the list, resize vertically and preserve separate saved dimensions',async()=>{
 const h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:1,bytes:54,revision:1});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();const area=h.doc.getElementById('packetCaptureAnalysis'),divider=h.doc.getElementById('packetDetailsDivider');area.getBoundingClientRect=()=>({width:700,height:600});h.api.layoutDetails();assert(area.classList.contains('packet-details-bottom'));assert.equal(divider.getAttribute('aria-orientation'),'horizontal');assert.equal(divider.getAttribute('aria-valuenow'),'260');
 h.doc.emit('pointerdown',divider,{button:0,clientY:340,pointerId:1});h.c.window.emit('pointermove',{clientY:390,pointerId:1});h.c.window.emit('pointerup');assert.equal(h.c.state.packetDetails.height,210);h.doc.emit('keydown',divider,{key:'ArrowUp'});assert.equal(h.c.state.packetDetails.height,220);h.doc.emit('keydown',divider,{key:'ArrowUp',shiftKey:true});assert.equal(h.c.state.packetDetails.height,250);h.doc.emit('keydown',divider,{key:'ArrowLeft'});assert.equal(h.c.state.packetDetails.height,250);h.doc.emit('keydown',divider,{key:'Home'});assert.equal(h.c.state.packetDetails.height,260);
 h.doc.emit('dblclick',divider);assert(h.doc.querySelector('#packetDetailsRail [data-panel-direction="up"]'));h.doc.emit('keydown',divider,{key:' '});assert(h.doc.querySelector('#packetInspector [data-panel-direction="down"]'));assert.equal(h.c.state.packetDetails.width,360);
 area.getBoundingClientRect=()=>({width:700,height:300});h.api.layoutDetails();assert(Number(divider.getAttribute('aria-valuenow'))<=195);assert.equal(h.c.state.packetDetails.height,260,'a viewport clamp overwrote the remembered height');area.getBoundingClientRect=()=>({width:1200,height:600});h.c.window.emit('resize');assert.equal(divider.getAttribute('aria-orientation'),'vertical');assert.equal(divider.getAttribute('aria-valuenow'),'360');
});
test('imported packet detail preferences reject invalid values and keep only bounded integer dimensions and a boolean',async()=>{
 const h=harness();await h.settle();const prefs=()=>JSON.parse(JSON.stringify(h.c.state.packetDetails));h.api.mergePreferences({packetDetails:{width:900,height:20,hidden:true,unexpected:'discard'}});assert.deepEqual(prefs(),{width:700,height:80,hidden:true,dock:'auto'});
 for(const value of [null,[],{width:'400',height:NaN,hidden:'true'},{width:Infinity,height:200.5,hidden:1}]){h.api.mergePreferences({packetDetails:value});assert.deepEqual(prefs(),{width:360,height:260,hidden:false,dock:'auto'});}
 h.api.mergePreferences({packetDetails:{width:420,height:310,hidden:false,dock:'auto'}});assert.deepEqual(prefs(),{width:420,height:310,hidden:false,dock:'auto'});
});
test('packet Hide leads both header states and the entire collapsed rail restores once in either dock',async()=>{
 const packet={number:1,protocol:'TCP',ipVersion:4,source:'192.0.2.1',destination:'198.51.100.2',stream:'TCP conversation'},h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:1,bytes:54,revision:1},{detail:{packet,layers:[],data:'',ascii:''}});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();assert.equal(h.doc.querySelector('.packet-detail-heading').firstElementChild.dataset.packetAction,'hide-details');h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();
 const reader=h.doc.getElementById('packetDetailScroll'),heading=h.doc.querySelector('.packet-detail-heading'),area=h.doc.getElementById('packetCaptureAnalysis'),rail=h.doc.getElementById('packetDetailsRail'),restore=rail.querySelector('button');assert.equal(heading.firstElementChild.dataset.packetAction,'hide-details');assert(heading.querySelector('[data-packet-stream]'));reader.scrollTop=45;let relays=0;restore.click=()=>{relays++;h.doc.emit('click',restore);};
 for(const width of [1200,700]){area.getBoundingClientRect=()=>({width,height:600});h.api.layoutDetails();h.doc.emit('click',heading.firstElementChild);assert.equal(h.c.state.packetDetails.hidden,true);for(let i=0;i<3;i++)h.api.layoutDetails();const before=relays;rail.emit('click',rail,{button:0});assert.equal(relays,before+1);assert.equal(h.c.state.packetDetails.hidden,false);assert.equal(h.doc.getElementById('packetDetailScroll'),reader);assert.equal(reader.scrollTop,45);h.doc.emit('click',heading.firstElementChild);rail.emit('click',restore,{button:0});assert.equal(relays,before+1,'the surface relayed an interactive child click');h.doc.emit('click',restore);assert.equal(h.c.state.packetDetails.hidden,false);}
});
test('packet quick actions target the right-clicked IPv6 row and make filtered scope and Show all visible',async()=>{
 const packets=[{number:1,protocol:'TCP',ipVersion:4,source:'192.0.2.1',destination:'198.51.100.1',relative:0,length:60,captured:60,info:'First'},{number:2,protocol:'TLS',ipVersion:6,source:'2001:db8::1',sourcePort:49152,destination:'2001:db8::2',destinationPort:443,stream:'stream "quoted" \\ value',relative:1,length:60,captured:60,info:'Second'}],h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:12,bytes:120,revision:1},{packets,detail:number=>({packet:packets.find(p=>p.number===number),layers:[],data:'',ascii:''})});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();
 const open=()=>h.doc.emit('contextmenu',h.doc.querySelector('[data-packet-number="2"]'),{clientX:100,clientY:100}),choose=action=>h.doc.emit('click',h.doc.querySelector('[data-packet-menu-action="'+action+'"]'));open();assert.match(h.doc.getElementById('packetActionsMenu').textContent,/Packet 2/);await h.settle();assert.equal(h.doc.querySelector('[data-packet-action="packet-menu"]').getAttribute('aria-expanded'),'true');choose('copy-destination');assert.equal(h.copied.at(-1),'[2001:db8::2]:443');assert.equal(h.api.model.detail.packet.number,2);
 h.doc.getElementById('packetSearch').value='unrelated';h.api.model.query='unrelated';h.api.model.protocol='TCP';open();choose('filter-source');await h.settle();assert.equal(h.api.model.expression,'ipv6.src == "2001:db8::1"');assert.equal(h.api.model.query,'');assert.equal(h.api.model.protocol,'any');assert.match(h.doc.querySelector('.packet-list-scope').textContent,/2 displayed of 12 retained packets/);assert.match(h.doc.querySelector('.packet-list-scope').textContent,/Filtered view: ipv6.src/);assert.equal(h.doc.querySelector('.packet-display-options').hasAttribute('open'),false);
 open();choose('filter-conversation');await h.settle();assert.equal(h.api.model.expression,'stream == "stream \\"quoted\\" \\\\ value"');open();choose('exclude-source');await h.settle();assert.equal(h.api.model.expression,'!(ipv6.src == "2001:db8::1")');
 h.doc.getElementById('packetKeepPorts').value='443';h.doc.getElementById('packetSearch').value='again';h.api.model.query='again';h.api.model.protocol='TLS';h.doc.emit('click',h.doc.querySelector('.packet-list-scope [data-packet-action="reset-filter"]'));await h.settle();assert.equal(h.api.model.expression,'');assert.equal(h.api.model.query,'');assert.equal(h.api.model.protocol,'any');assert.equal(h.doc.getElementById('packetKeepPorts').value,'443');assert.equal(h.doc.querySelector('.packet-active-filter'),null);assert.equal(h.calls.filter(call=>call.method!=='GET'&&!call.path.endsWith('/interfaces')).length,0);
});
test('packet menu respects IPv4, ARP, MAC and raw protocol fields without inventing unsupported filters',async()=>{
 const packet={number:1,protocol:'IP protocol 254',ipVersion:4,source:'192.0.2.5',destination:'198.51.100.7'},detail={packet,layers:[],data:'',ascii:''},h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:1,bytes:60,revision:1},{detail});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();const open=()=>h.doc.emit('click',h.doc.querySelector('[data-packet-action="packet-menu"]')),choose=action=>h.doc.emit('click',h.doc.querySelector('[data-packet-menu-action="'+action+'"]'));
 open();choose('filter-destination');await h.settle();assert.equal(h.api.model.expression,'ip.dst == "198.51.100.7"');open();choose('filter-protocol');await h.settle();assert.equal(h.api.model.expression,'protocol == "IP protocol 254"');
 Object.assign(packet,{protocol:'ARP',ipVersion:0,source:'192.0.2.1',destination:'192.0.2.2'});open();assert(h.doc.querySelector('[data-packet-menu-action="filter-source"]').hasAttribute('disabled'));assert(h.doc.querySelector('[data-packet-menu-action="filter-destination"]').hasAttribute('disabled'));assert(h.doc.querySelector('[data-packet-menu-action="filter-conversation"]').hasAttribute('disabled'));choose('filter-source');assert.equal(h.api.model.expression,'protocol == "IP protocol 254"');choose('copy-source');assert.equal(h.copied.at(-1),'192.0.2.1');
 Object.assign(packet,{source:'02:00:00:00:00:01',destination:'02:00:00:00:00:02'});open();choose('filter-source');await h.settle();assert.equal(h.api.model.expression,'eth.src == "02:00:00:00:00:01"');Object.assign(packet,{source:'',destination:''});open();assert(h.doc.querySelector('[data-packet-menu-action="filter-source"]').hasAttribute('disabled'));assert(h.doc.querySelector('[data-packet-menu-action="copy-source"]').hasAttribute('disabled'));h.doc.emit('click',h.doc.getElementById('packetCaptureStatus'));assert.equal(h.doc.getElementById('packetActionsMenu'),null);
});
test('packet and field menus support keyboard focus, Escape and copying the decoded value without changing byte selection',async()=>{
 const h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:1,bytes:54,revision:1});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();const oldFocus=Element.prototype.focus;Element.prototype.focus=function(){h.doc.activeElement=this;};
 try{const row=h.doc.querySelector('[data-packet-number="1"]');h.doc.emit('keydown',row,{key:'F10',shiftKey:true});const menu=h.doc.getElementById('packetActionsMenu');assert(menu);h.doc.emit('keydown',h.doc.activeElement,{key:'End'});assert.equal(h.doc.activeElement.dataset.packetMenuAction,'copy-destination');h.doc.emit('keydown',h.doc.activeElement,{key:'Home'});assert.equal(h.doc.activeElement.dataset.packetMenuAction,'filter-protocol');h.doc.emit('keydown',h.doc.activeElement,{key:'ArrowDown'});assert.equal(h.doc.activeElement.dataset.packetMenuAction,'show-all');h.doc.emit('keydown',h.doc.activeElement,{key:'Escape'});assert.equal(h.doc.getElementById('packetActionsMenu'),null);assert.equal(h.doc.activeElement,row);
 const field=h.doc.querySelector('.packet-field'),value=field.querySelectorAll('span')[1].textContent;h.doc.emit('click',field);const range=JSON.stringify(h.api.model.range),reader=h.doc.getElementById('packetDetailScroll');reader.scrollTop=100;h.doc.emit('keydown',field,{key:'ContextMenu'});assert.match(h.doc.getElementById('packetActionsMenu').getAttribute('aria-label'),/Protocol field/);h.doc.emit('click',h.doc.querySelector('[data-packet-menu-action="copy-field"]'));assert.equal(h.copied.at(-1),value);assert.equal(JSON.stringify(h.api.model.range),range);assert.equal(h.doc.activeElement,field);assert.equal(reader.scrollTop,100);h.doc.emit('keydown',field,{key:'F10',shiftKey:true});h.doc.emit('keydown',h.doc.activeElement,{key:'Tab'});assert.equal(h.doc.getElementById('packetActionsMenu'),null);assert.equal(h.doc.activeElement,field);
 }finally{Element.prototype.focus=oldFocus;}
});
test('capture validation retains its field label when an editable input gains a clear-control wrapper',async()=>{
 const h=harness();await h.settle();const input=h.doc.getElementById('packetDuration'),wrapper=h.doc.createElement('span');input.before(wrapper);wrapper.append(input);input.checkValidity=()=>false;assert.throws(()=>h.api.optionRequest(),/Check Stop after \(seconds\)\./);
});
test('clearing a packet search or filter applies only that cleared field and preserves the current analysis view',async()=>{
 const h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:1,bytes:54,revision:1});await h.settle();h.api.model.expression='dns';h.api.model.query='old search';h.api.model.protocol='UDP';h.doc.getElementById('packetDisplayFilter').value='dns';const search=h.doc.getElementById('packetSearch');search.value='';h.doc.emit('input-clear',search);await h.settle();assert.equal(h.api.model.query,'');assert.equal(h.api.model.expression,'dns');assert.equal(h.api.model.protocol,'UDP');assert.equal(h.api.model.view,'troubleshooting');h.api.render();await h.settle();assert.equal(h.doc.getElementById('packetSearch').value,'');
 h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.api.model.query='retained search';h.doc.getElementById('packetSearch').value='retained search';const filter=h.doc.getElementById('packetDisplayFilter');filter.value='';h.doc.emit('input-clear',filter);await h.settle();assert.equal(h.api.model.expression,'');assert.equal(h.api.model.query,'retained search');assert.equal(h.api.model.protocol,'UDP');assert(h.calls.some(call=>call.path.includes('/packets?')&&call.path.includes('query=retained+search')&&!call.path.includes('filter=')));assert.equal(h.calls.filter(call=>call.method!=='GET'&&!call.path.endsWith('/interfaces')).length,0);
});
test('packet row menus integrate the shared exact-cell Copy value action without replacing packet tools',async()=>{
 const h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:1,bytes:54,revision:1});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();vm.runInNewContext(fs.readFileSync(require.resolve('../web/assets/value-copy.js'),'utf8'),h.c);h.c.window.NetworkToolkitValueCopy=h.c.NetworkToolkitValueCopy;
 const reader=h.doc.getElementById('packetDetailScroll'),cell=h.doc.querySelector('[data-packet-number="1"]').querySelectorAll('td')[2];reader.scrollTop=75;h.doc.emit('contextmenu',cell,{clientX:10,clientY:10});const menu=h.doc.getElementById('packetActionsMenu');assert(menu.querySelector('[data-value-copy-context]'));assert(menu.querySelector('[data-packet-menu-action="copy-source"]'));h.doc.emit('click',menu.querySelector('[data-value-copy-context]'));await h.settle();assert.equal(h.copied.at(-1),'192.0.2.1');assert.equal(h.doc.getElementById('packetActionsMenu'),null);assert.equal(h.doc.getElementById('packetDetailScroll'),reader);assert.equal(reader.scrollTop,75);
});
test('DHCP quick focus and guided task include undecoded port traffic and replace stale display restrictions only',async()=>{
 const h=harness({id:'a'.repeat(24),name:'Capture',state:'completed',count:6,bytes:1500,revision:1});await h.settle();const button=[...h.doc.querySelectorAll('[data-packet-filter]')].find(item=>item.textContent==='Address assignment (DHCP)');assert(button);assert.match(button.dataset.packetFilter,/udp.port == 67/);assert.match(button.dataset.packetFilter,/udp.port == 68/);assert.match(button.dataset.packetFilter,/udp.port == 546/);assert.match(button.dataset.packetFilter,/udp.port == 547/);assert.match(button.getAttribute('title'),/DHCPv6.*appear as UDP/);
 h.doc.getElementById('packetKeepPorts').value='67,68';h.doc.getElementById('packetSearch').value='old name lookup';h.api.model.query='old name lookup';h.api.model.protocol='TCP';h.doc.emit('click',button);await h.settle();assert.equal(h.api.model.query,'');assert.equal(h.api.model.protocol,'any');assert.equal(h.api.model.expression,button.dataset.packetFilter);assert.match(h.doc.getElementById('packetFilterHint').textContent,/port match alone does not confirm DHCP/);
 h.doc.getElementById('packetTask').value='dhcp';h.doc.emit('change',h.doc.getElementById('packetTask'));assert.match(h.doc.getElementById('packetTaskHint').textContent,/full packets with All protocols/);assert.match(h.doc.getElementById('packetTaskHint').textContent,/SLAAC/);assert.match(h.doc.getElementById('packetTaskHint').textContent,/does not renew/);h.doc.getElementById('packetSearch').value='stale';h.api.model.query='stale';h.api.model.protocol='TLS';h.doc.emit('click',h.doc.querySelector('[data-packet-action="goal-filter"]'));await h.settle();assert.equal(h.api.model.expression,button.dataset.packetFilter);assert.equal(h.api.model.query,'');assert.equal(h.api.model.protocol,'any');assert.equal(h.doc.getElementById('packetKeepPorts').value,'67,68');assert.equal(h.calls.filter(call=>call.method!=='GET'&&!call.path.endsWith('/interfaces')).length,0);
});
test('decoded DHCPv4 messages explain observed stages without treating missing stages or acknowledgements as proof',async()=>{
 const kinds=['Discover','Offer','Request','ACK','NAK','Decline','Release','Inform'],checks=[/client is looking for a DHCPv4 server/,/does not mean the client has accepted/,/requesting or renewing/,/does not prove the client applied/,/rejects the requested configuration/,/reports that the proposed address appears to be in use/,/giving up its lease/,/already has an address/],packets=kinds.map((kind,index)=>({number:index+1,protocol:'DHCP',info:'DHCP '+kind+', Host test Transaction 0x12345678',source:'0.0.0.0',sourcePort:68,destination:'255.255.255.255',destinationPort:67,length:300,captured:300,relative:index}));
 const h=harness({id:'a'.repeat(24),name:'DHCP exchange',state:'completed',count:8,bytes:2400,revision:1},{packets,detail:number=>({packet:packets[number-1],layers:[{name:'DHCP / BOOTP',fields:[{name:'Summary',value:packets[number-1].info}]}],data:'',ascii:''})});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();for(let index=0;index<kinds.length;index++){h.doc.emit('click',h.doc.querySelector('[data-packet-number="'+(index+1)+'"]'));await h.settle();const text=h.doc.querySelector('.packet-plain-explanation').textContent;assert.match(text,checks[index]);assert.match(text,/DHCPv4/);assert.match(text,/Renewals may use only Request and ACK/);assert.match(text,/not proof of failure/);assert.match(text,/transaction IDs/);assert(h.doc.querySelector('.packet-layer-tree'));}
});
test('DHCP port candidates remain explicitly undecoded UDP with IPv4 or IPv6-specific hints',async()=>{
 const packets=[{number:1,protocol:'UDP',sourcePort:68,destinationPort:67,length:300,captured:100,relative:0},{number:2,protocol:'UDP',sourcePort:546,destinationPort:547,length:90,captured:90,relative:1}],h=harness({id:'a'.repeat(24),name:'Candidates',state:'completed',count:2,bytes:190,revision:1},{packets,detail:number=>({packet:packets[number-1],layers:[{name:'Payload',fields:null}],data:'',ascii:''})});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();assert.match(h.doc.querySelector('.packet-protocol-note').textContent,/DHCPv4.*not decoded/);assert.match(h.doc.querySelector('.packet-protocol-note').textContent,/full packets/);h.doc.emit('click',h.doc.querySelector('[data-packet-number="2"]'));await h.settle();assert.match(h.doc.querySelector('.packet-protocol-note').textContent,/DHCPv6 messages are not decoded/);assert.match(h.doc.querySelector('.packet-protocol-note').textContent,/port match alone does not confirm DHCP/);assert.equal(h.doc.querySelector('.packet-dhcp-guidance'),null);
});
test('packet empty states distinguish filtered traffic from an empty recording and retain actionable DHCP timing guidance',async()=>{
 const capture={id:'a'.repeat(24),name:'Retained',state:'completed',count:7,bytes:1000,revision:1},h=harness(capture,{packets:[],total:0});await h.settle();const dhcp=[...h.doc.querySelectorAll('[data-packet-filter]')].find(item=>item.textContent==='Address assignment (DHCP)');h.doc.emit('click',dhcp);await h.settle();const text=h.doc.querySelector('.packet-no-results').textContent;assert.match(text,/No packets match these display filters/);assert.match(text,/All 7 retained packets remain/);assert.match(text,/Show all packets above/);assert.match(text,/does not prove address assignment failed/);assert.equal(h.api.model.captures[0].count,7);h.doc.emit('click',h.doc.querySelector('.packet-list-scope [data-packet-action="reset-filter"]'));await h.settle();assert.equal(h.api.model.expression,'');assert.equal(h.api.model.captures[0].count,7);
 h.setCapture({...capture,state:'running',engine:'file',count:0});await h.api.refresh(true);assert.match(h.doc.querySelector('.packet-no-results').textContent,/No decoded packets are retained yet/);assert.match(h.doc.querySelector('.packet-no-results').textContent,/Compatibility file recording loads packets after Stop/);assert.equal(h.calls.filter(call=>call.method!=='GET'&&!call.path.endsWith('/interfaces')).length,0);
});

test('continuous scroll uses bounded packet windows, keeps important events visible and jumps to frames',async()=>{
 const list=Array.from({length:1800},(_,i)=>({number:i+1,relative:i/1000,source:'192.0.2.1',destination:'192.0.2.2',protocol:'UDP',length:60,captured:60,info:'Packet '+(i+1),expert:i===1500?['Packet truncated by capture length']:[]}));
 const overview={bins:Array.from({length:180},(_,i)=>({level:i===150?'note':'',protocol:'UDP',interesting:i===150?1:0})),interesting:1,counts:{error:0,warn:0,note:1},signals:[{id:'truncated',severity:'note',title:'Truncated capture',explanation:'Only part of the packet was recorded.',count:1,number:1501,index:1500}]};
 const h=harness({id:'a'.repeat(24),name:'Long capture',state:'completed',count:1800,bytes:108000,revision:1},{packets:list,total:list.length,paginate:true,overview,detail:n=>({packet:list[n-1],layers:[{name:'Frame '+n,offset:0,length:60,fields:[{name:'Frame number',value:String(n),offset:0,length:0}]}],data:'',ascii:''})});
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 assert(h.doc.querySelectorAll('#packetTableScroll [data-packet-number]').length<=302);assert.equal(h.doc.querySelector('.packet-pagination'),null);assert(h.doc.querySelector('#packetOverviewMap'));assert.match(h.doc.querySelector('#packetInsights').textContent,/Truncated capture/);
 let scroll=h.doc.getElementById('packetTableScroll');scroll.scrollTop=28000;h.doc.emit('scroll',scroll);await h.settle();
 assert(h.api.model.windowStart>=1200);assert(h.doc.querySelectorAll('#packetTableScroll [data-packet-number]').length<=302);assert(h.doc.querySelector('[data-packet-number="1300"]'));
 h.doc.emit('click',h.doc.querySelector('[data-packet-jump="1500"]'));await h.settle();
 assert.equal(h.api.model.detail?.packet.number,1501);assert(h.doc.querySelector('.packet-layer-tree').textContent.includes('Frame 1501'));
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="toggle-insights"]'));assert.equal(h.doc.getElementById('packetInsights').hidden,true);
 assert.equal(h.c.state.packetInsightsOpen,false);assert.equal(h.api.model.total,1800);
});

test('packet overview clicks select the exact proportional frame and keep viewport centered',async()=>{
 const list=Array.from({length:1800},(_,i)=>({number:i+1,relative:i/1000,source:'192.0.2.10',destination:'192.0.2.11',protocol:'UDP',length:64,captured:64,info:'Frame '+(i+1),expert:[]}));
 const h=harness({id:'f'.repeat(24),name:'Map alignment capture',state:'completed',count:list.length,bytes:115200,revision:2},{packets:list,total:list.length,paginate:true,overview:{bins:[],signals:[],counts:{},interesting:0},detail:n=>({packet:list[n-1],layers:[],data:'',ascii:''})});
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const bar=h.doc.getElementById('packetOverviewMap'),scroll=h.doc.getElementById('packetTableScroll');
 bar.getBoundingClientRect=()=>({top:100,bottom:500,left:100,right:116,height:400,width:16});bar.clientHeight=400;
 scroll.scrollHeight=1800*21+26;scroll.clientHeight=350;
 for(const ratio of [.1,.5,.9]){
  h.doc.emit('click',bar,{clientY:100+ratio*400});await h.settle();
  const target=Math.floor(ratio*list.length),selected=target+1;
  assert.equal(h.api.model.detail?.packet.number,selected,'Map position selects its exact frame index');
  assert.equal(h.api.model.selectedIndex,target,'Map selected-frame index matches the sorted filtered packet index');
  assert.equal(h.doc.getElementById('packetTableScroll'),scroll,'Packet scroller is not replaced on a map click');
  assert(h.doc.querySelector('[data-packet-number="'+selected+'"].selected'));
  const marker=h.doc.getElementById('packetMapSelected'),overlay=h.doc.getElementById('packetMapViewport'),ymarker=Number(marker.style.transform.match(/,([\d.]+)px/)[1]),yviewport=Number(overlay.style.transform.match(/,([\d.]+)px/)[1])+Number(overlay.style.height.replace('px',''))/2;
  assert(Math.abs(ymarker-ratio*400)<=2,'Selected-frame map tick matches clicked location');
  assert(Math.abs(yviewport-ratio*400)<8,'Visible-range box is centered on the clicked location');
 }
 const initialCalls=h.calls.filter(c=>c.path.includes('/packets?')).length;
 await h.api.refresh();assert.equal(h.calls.filter(c=>c.path.includes('/packets?')).length,initialCalls,'Polling a finished capture should not reload the same window');
 scroll.scrollTop=28000;h.doc.emit('scroll',scroll);await h.settle();
 assert.equal(h.doc.getElementById('packetTableScroll'),scroll,'Scroll node persists across a virtual-window shift');
 assert(h.calls.some(c=>c.path.includes('overview=0')),'Adjacent pages omit full-capture overview computation');
});

test('the detailed map defaults to one pixel per packet and virtualizes offscreen stripes',async()=>{
 const n=2000,packets=Array.from({length:n},(_,i)=>({number:i+1,relative:i/100,source:'192.0.2.1',destination:'192.0.2.2',protocol:'UDP',length:60,captured:60,info:String(i),expert:[]}));
 // A=ordinary, h=warning DNS (32+3), R=note TCP (16+1).
 const marks=Array.from({length:n},(_,i)=>i===600?'j':i===1021?'R':'C').join('');
 const h=harness({id:'d'.repeat(24),name:'Granular',state:'completed',count:n,bytes:n*60,revision:7},{packets,total:n,paginate:true,overview:{packetMarks:marks,bins:[],signals:[],counts:{},interesting:0},detail:num=>({packet:packets[num-1],layers:[],data:'',ascii:''})});
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const lane=h.doc.getElementById('packetDetailMap'),content=h.doc.getElementById('packetDetailMapContent'),scroll=h.doc.getElementById('packetTableScroll');
 lane.clientHeight=400;lane.scrollHeight=n;scroll.clientHeight=350;scroll.scrollHeight=n*21+26;
 assert.equal(content.style.height,n+'px');
 assert.equal(h.doc.querySelector('[data-map-index="0"]').getAttribute('style'),'top:0px');
 assert.equal(h.doc.querySelector('[data-map-index="1"]').getAttribute('style'),'top:1px');
 assert(h.doc.querySelectorAll('.packet-detail-line').length<650,'stripes should be virtualized, not one DOM node per retained packet');
 scroll.scrollTop=21*1000;h.doc.emit('scroll',scroll);await h.settle();
 assert(h.doc.querySelector('[data-map-index="1021"]'),'moving the packet list exposes corresponding per-packet stripes');
 assert(h.doc.querySelector('[data-map-index="1021"]').classList.contains('packet-map-note'),'exact packet position retains its severity');
 assert.equal(h.doc.querySelector('[data-map-index="0"]'),null,'distant frames are not rendered as DOM nodes');
 const box=lane.getBoundingClientRect(),before=h.api.model.windowStart;
 h.doc.emit('click',lane,{clientY:box.top+200});await h.settle();
 const expected=Math.floor(lane.scrollTop+200)+1;
 assert.equal(h.api.model.detail.packet.number,expected,'clicking a specific 1-pixel line selects that packet');
 assert.equal(h.api.model.selectedIndex,expected-1);
 assert(h.api.model.windowStart>=before-180);
});

test('dragging the main map thumb retains the pointer offset; outside-clicks still select the exact frame',async()=>{
 const n=2400,packets=Array.from({length:n},(_,i)=>({number:i+1,relative:i/100,source:'192.0.2.1',destination:'192.0.2.2',protocol:'UDP',length:60,captured:60,info:String(i),expert:[]}));
 const h=harness({id:'b'.repeat(24),name:'Drag',state:'completed',count:n,bytes:120000,revision:1},{packets,total:n,paginate:true,overview:{packetMarks:'C'.repeat(n),bins:[],signals:[],counts:{},interesting:0},detail:num=>({packet:packets[num-1],layers:[],data:'',ascii:''})});
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const bar=h.doc.getElementById('packetOverviewMap'),scroll=h.doc.getElementById('packetTableScroll');bar.clientHeight=400;bar.getBoundingClientRect=()=>({top:100,bottom:500,left:100,right:112,height:400,width:12});scroll.scrollHeight=n*21+26;scroll.clientHeight=400;
 // Seed a nonzero thumb position with a click before starting the drag.
 h.doc.emit('click',bar,{clientY:280});await h.settle();
 const overlay=h.doc.getElementById('packetMapViewport'),top=Number(overlay.style.transform.match(/,([\d.]+)px/)[1]),height=Number(overlay.style.height.replace('px',''));
 const originalTop=scroll.scrollTop,down=100+top+height/2;
 h.doc.emit('pointerdown',bar,{button:0,clientY:down,pointerId:44});
 assert.equal(scroll.scrollTop,originalTop,'pressing within the thumb does not snap it to the pointer');
 h.c.window.emit('pointermove',{clientY:down+44,pointerId:44,preventDefault(){}});await h.settle();
 assert(scroll.scrollTop>originalTop+300,'dragging the thumb moves the packet list');
 const overlayAfter=h.doc.getElementById('packetMapViewport'),newTop=Number(overlayAfter.style.transform.match(/,([\d.]+)px/)[1]);
 assert(Math.abs((newTop-top)-44)<2,'the viewport tracks the pointer while preserving the initial offset');
 h.c.window.emit('pointerup',{clientY:down+44,pointerId:44});
 h.doc.emit('click',bar,{clientY:down+44});await h.settle(); // suppress synthetic click at end of drag
 h.doc.emit('click',bar,{clientY:460});await h.settle();
 assert.equal(h.api.model.detail.packet.number,Math.floor(.9*n)+1,'normal map click still selects the correct packet');
});

test('compact packet guidance explains common patterns and applies supported filters without permanently occupying list height',async()=>{
 const h=harness({id:'c'.repeat(24),name:'Guided',state:'completed',count:1,bytes:60,revision:1});
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const guide=h.doc.getElementById('packetQuickGuide'),scroll=h.doc.getElementById('packetTableScroll');
 assert.equal(guide.hasAttribute('hidden'),true);assert(h.doc.querySelector('[data-packet-action="toggle-guide"]'));
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="toggle-guide"]'));assert.equal(guide.hidden,false);
 assert.match(guide.textContent,/DHCP Discover/);assert.match(guide.textContent,/NXDOMAIN/);assert.match(guide.textContent,/SYN\/ACK/);assert.match(guide.textContent,/retransmission/);
 const action=guide.querySelector('[data-packet-filter="dns"]');h.doc.emit('click',action);await h.settle();
 assert.equal(h.api.model.expression,'dns');assert.equal(h.api.model.guideOpen,false);
 assert.equal(h.doc.getElementById('packetTableScroll')===scroll,false,'filter change may rebuild the list as before');
 const newer=h.doc.getElementById('packetTableScroll');h.doc.emit('click',h.doc.querySelector('[data-packet-action="toggle-guide"]'));
 await h.api.refresh();assert.equal(h.doc.getElementById('packetTableScroll'),newer,'ordinary refreshes retain the scroller');
 assert.equal(h.doc.getElementById('packetQuickGuide').hidden,false,'an ordinary refresh does not unexpectedly close the guide');
});

test('packet colors and map sizes can be customized without removing the packet list or restarting capture',async()=>{
 const n=800,packets=Array.from({length:n},(_,i)=>({number:i+1,relative:i/100,source:'192.0.2.1',destination:'192.0.2.2',protocol:i%2?'DNS':'TCP',length:64,captured:64,info:'Frame '+(i+1),expert:[]}));
 const h=harness({id:'e'.repeat(24),name:'Color options',state:'completed',count:n,bytes:n*64,revision:1},{packets,total:n,paginate:true,overview:{packetMarks:'A'.repeat(n),bins:[{protocol:'TCP',level:'normal'}],signals:[],counts:{},interesting:0},detail:num=>({packet:packets[num-1],layers:[],data:'',ascii:''})});
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 h.doc.getElementById('packetFollow').checked=false;h.doc.emit('change',h.doc.getElementById('packetFollow'));h.doc.emit('click',h.doc.querySelector('[data-packet-action="scroll-first"]'));await h.settle();
 const viewport=h.doc.getElementById('packetTableScroll'),lane=h.doc.getElementById('packetDetailMap'),content=h.doc.getElementById('packetDetailMapContent');
 assert.equal(content.style.height,n+'px','one pixel per packet is the default');
 assert(h.doc.querySelector('[data-packet-action="toggle-appearance"]'));
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="toggle-appearance"]'));
 const panel=h.doc.getElementById('packetAppearanceSettings');assert.equal(panel.hidden,false);
 const color=h.doc.querySelector('[data-packet-color="tcp"]');color.value='#123abc';h.doc.emit('input',color);
 assert.equal(h.c.state.packetMapSettings.colors.tcp,'#123abc');
 assert.equal(h.doc.getElementById('packetCaptureWorkspace').style['--packet-color-tcp'],'#123abc');
 assert.equal(h.doc.getElementById('packetTableScroll'),viewport,'changing a palette does not rebuild the list');
 const row=h.doc.getElementById('packetRows').querySelector('[data-packet-number="1"]');
 assert.match(row.getAttribute('style'),/--packet-row-color:var\(--packet-color-tcp\)/,'row uses the editable protocol color');
 const rowStyle=h.doc.getElementById('packetRowStyle');rowStyle.value='strong';h.doc.emit('change',rowStyle);
 assert(h.doc.getElementById('packetCaptureWorkspace').classList.contains('packet-row-mode-strong'));
 const overview=h.doc.getElementById('packetOverviewWidth');overview.value='24';h.doc.emit('change',overview);
 const width=h.doc.getElementById('packetDetailWidth');width.value='32';h.doc.emit('change',width);
 const mode=h.doc.getElementById('packetMapColorMode');mode.value='alerts';h.doc.emit('change',mode);
 assert(h.doc.getElementById('packetCaptureWorkspace').classList.contains('packet-map-mode-alerts'));
 assert.equal(h.doc.getElementById('packetCaptureWorkspace').style['--packet-map-overview-width'],'24px');
 assert.equal(h.doc.getElementById('packetCaptureWorkspace').style['--packet-map-detail-width'],'32px');
 const scale=h.doc.getElementById('packetMapLineHeight');scale.value='4';h.doc.emit('change',scale);
 assert.equal(content.style.height,(n*4)+'px');
 assert.equal(h.doc.querySelector('[data-map-index="1"]').getAttribute('style'),'top:4px');
 assert.equal(h.doc.getElementById('packetTableScroll'),viewport);
 assert.equal(h.doc.getElementById('packetDetailMap'),lane);
 lane.scrollHeight=n*4;lane.clientHeight=380;lane.scrollTop=0;viewport.scrollHeight=n*21+26;viewport.clientHeight=350;
 const rect=lane.getBoundingClientRect();h.doc.emit('click',lane,{clientY:rect.top+42});await h.settle();
 const expected=Math.floor((lane.scrollTop+42)/4)+1;
 assert.equal(h.api.model.detail.packet.number,expected,'custom line-height preserves exact click selection');
 assert(h.saved.length>=1,'appearance changes persist');
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="reset-appearance"]'));
 assert.equal(h.c.state.packetMapSettings.lineHeight,1);
 assert.equal(h.c.state.packetMapSettings.colors.tcp,'#a7c6e9');
 assert.equal(content.style.height,n+'px');
 assert.equal(h.c.state.packetMapSettings.rowStyle,'marker');
});

test('saved packet map settings are validated, restored, and cannot inject CSS values',async()=>{
 const h=harness();await h.settle();
 h.api.mergePreferences({packetMapSettings:{lineHeight:2,overviewWidth:20,detailWidth:24,rowStyle:'subtle',mapColors:'protocols',colors:{dns:'#1a2b3c',tcp:'red; color:white',error:'#ffff00'}}});
 assert.equal(h.c.state.packetMapSettings.lineHeight,2);
 assert.equal(h.c.state.packetMapSettings.overviewWidth,20);
 assert.equal(h.c.state.packetMapSettings.colors.dns,'#1a2b3c');
 assert.equal(h.c.state.packetMapSettings.colors.error,'#ffff00');
 assert.equal(h.c.state.packetMapSettings.colors.tcp,'#a7c6e9','invalid CSS is rejected');
 assert(h.doc.getElementById('packetCaptureWorkspace').classList.contains('packet-row-mode-subtle'));
 assert(h.doc.getElementById('packetCaptureWorkspace').classList.contains('packet-map-mode-protocols'));
 h.api.mergePreferences({packetMapSettings:{lineHeight:0,overviewWidth:500,rowStyle:'javascript:',colors:{dns:'blue'}}});
 assert.equal(h.c.state.packetMapSettings.lineHeight,1);
 assert.equal(h.c.state.packetMapSettings.overviewWidth,12);
 assert.equal(h.c.state.packetMapSettings.rowStyle,'marker');
 assert.equal(h.c.state.packetMapSettings.colors.dns,'#b8a5d9');
});

test('fine map viewport drag follows pointer one-to-one at 1, 2, and 4 pixels per packet',async()=>{
 const n=3000,packets=Array.from({length:n},(_,i)=>({number:i+1,relative:i/100,source:'192.0.2.1',destination:'192.0.2.2',protocol:'UDP',length:60,captured:60,info:String(i),expert:[]}));
 const h=harness({id:'9'.repeat(24),name:'Direct drag',state:'completed',count:n,bytes:n*60,revision:1},{packets,total:n,paginate:true,overview:{packetMarks:'C'.repeat(n),bins:[],signals:[],counts:{},interesting:0}});
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const list=h.doc.getElementById('packetTableScroll'),lane=h.doc.getElementById('packetDetailMap'),panel=h.doc.getElementById('packetCaptureWorkspace');
 list.scrollHeight=n*21+26;list.clientHeight=400;lane.clientHeight=400;lane.getBoundingClientRect=()=>({top:30,height:400,bottom:430});
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="toggle-appearance"]'));
 for(const scale of [1,2,4]){
  const ctrl=h.doc.getElementById('packetMapLineHeight');ctrl.value=String(scale);h.doc.emit('change',ctrl);
  lane.scrollTop=0;list.scrollTop=21*160+26;h.doc.emit('scroll',list);await h.settle();
  const overlay=h.doc.getElementById('packetDetailMapViewport');const firstTop=parseFloat(overlay.style.transform.match(/,([\d.]+)px/)[1]);
  const press=30+firstTop-lane.scrollTop+parseFloat(overlay.style.height)/2;
  const before=list.scrollTop;
  h.doc.emit('pointerdown',lane,{button:0,clientY:press,pointerId:scale+70});
  assert.equal(list.scrollTop,before,'grabbing thumb must not change list position');
  h.c.window.emit('pointermove',{clientY:press+12,pointerId:scale+70,preventDefault(){}});await h.settle();
  const desired=12*21/scale;
  assert(Math.abs((list.scrollTop-before)-desired)<2,'list drag corresponds to exactly the packet lines under mouse');
  const secondTop=parseFloat(overlay.style.transform.match(/,([\d.]+)px/)[1]);
  assert(Math.abs(secondTop-firstTop-12)<1.5,'thumb should follow pointer by 12 CSS px without recentering');
  assert.equal(h.doc.getElementById('packetTableScroll'),list,'drag must not recreate packet list');
  h.c.window.emit('pointerup',{clientY:press+12,pointerId:scale+70});
  assert(panel.classList.contains('packet-row-mode-marker'));
 }
});

test('full map supports Fit and exact 1-4 pixel packets, click selection, dragging, and restores defaults',async()=>{
 const n=2800,packets=Array.from({length:n},(_,i)=>({number:i+1,relative:i/100,source:'192.0.2.1',destination:'192.0.2.2',protocol:'UDP',length:60,captured:60,info:String(i),expert:[]}));
 const h=harness({id:'8'.repeat(24),name:'Scaled full overview',state:'completed',count:n,bytes:n*60,revision:1},{packets,total:n,paginate:true,overview:{packetMarks:'C'.repeat(n),bins:Array.from({length:160},()=>({protocol:'UDP',level:'normal'})),signals:[],counts:{},interesting:0},detail:num=>({packet:packets[num-1],layers:[],data:'',ascii:''})});
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 h.doc.getElementById('packetFollow').checked=false;h.doc.emit('change',h.doc.getElementById('packetFollow'));h.doc.emit('click',h.doc.querySelector('[data-packet-action="scroll-first"]'));await h.settle();
 const bar=h.doc.getElementById('packetOverviewMap'),list=h.doc.getElementById('packetTableScroll');
 list.scrollHeight=n*21+26;list.clientHeight=400;bar.clientHeight=400;bar.getBoundingClientRect=()=>({top:55,height:400,bottom:455,left:400,width:12,right:412});
 assert.equal(bar.classList.contains('packet-overview-scaled'),false,'default should show whole capture');
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="toggle-appearance"]'));
 const scaleField=h.doc.getElementById('packetOverviewScale');
 for(const scale of [1,2,3,4]){
  scaleField.value=String(scale);h.doc.emit('change',scaleField);
  assert.equal(bar.classList.contains('packet-overview-scaled'),true);
  assert.equal(h.doc.getElementById('packetOverviewContent').style.height,(n*scale)+'px');
  assert(h.doc.querySelectorAll('.packet-overview-line').length<650,'overview should virtualize individual packet lines');
  assert.equal(h.doc.querySelector('[data-overview-index="1"]').getAttribute('style'),'top:'+scale+'px');
  list.scrollTop=21*65+26;bar.scrollTop=0;h.doc.emit('scroll',list);await h.settle();
  const overlay=h.doc.getElementById('packetMapViewport'),first=parseFloat(overlay.style.transform.match(/,([\d.]+)px/)[1]);
  const press=55+first-bar.scrollTop+parseFloat(overlay.style.height)/2;
  const before=list.scrollTop;
  h.doc.emit('pointerdown',bar,{button:0,clientY:press,pointerId:scale+30});
  assert.equal(list.scrollTop,before);
  h.c.window.emit('pointermove',{clientY:press+9,pointerId:scale+30,preventDefault(){}});await h.settle();
  assert(Math.abs((list.scrollTop-before)-9*21/scale)<2);
  const second=parseFloat(overlay.style.transform.match(/,([\d.]+)px/)[1]);
  assert(Math.abs(second-first-9)<1.5,'full map thumb should move 9 pixels with pointer');
  h.c.window.emit('pointerup',{clientY:press+9,pointerId:scale+30});
  h.doc.emit('click',bar,{clientY:press+9}); // Synthetic click at the end of a drag is suppressed.
  bar.scrollTop=100*scale;h.doc.emit('scroll',bar);await h.settle();
  const clicked=130;h.doc.emit('click',bar,{clientY:55+clicked*scale-bar.scrollTop});await h.settle();
  assert.equal(h.api.model.detail.packet.number,clicked+1,'scaled full map click maps to exact frame');
 }
 scaleField.value='fit';h.doc.emit('change',scaleField);
 assert.equal(bar.classList.contains('packet-overview-scaled'),false);
 assert.equal(h.doc.getElementById('packetOverviewContent').style.height,'');
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="reset-appearance"]'));
 assert.equal(h.c.state.packetMapSettings.overviewScale,'fit');
 assert.equal(h.c.state.packetMapSettings.lineHeight,1);
 assert.equal(h.doc.getElementById('packetTableScroll'),list);
});

test('default event colors are pure RGB primary colors; full strength row coloring is configurable',async()=>{
 const h=harness({id:'7'.repeat(24),name:'Full color',state:'completed',count:1,bytes:64,revision:1});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const workspace=h.doc.getElementById('packetCaptureWorkspace');
 assert.equal(h.c.state.packetMapSettings.colors.error,'#ff0000');
 assert.equal(h.c.state.packetMapSettings.colors.warn,'#ffff00');
 assert.equal(h.c.state.packetMapSettings.colors.note,'#0000ff');
 assert.equal(workspace.style['--packet-contrast-error'],'#101820');
 assert.equal(workspace.style['--packet-contrast-warn'],'#101820');
 assert.equal(workspace.style['--packet-contrast-note'],'#ffffff');
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="toggle-appearance"]'));
 const dropdown=h.doc.getElementById('packetRowStyle');dropdown.value='full';h.doc.emit('change',dropdown);
 assert.equal(h.c.state.packetMapSettings.rowStyle,'full');
 assert(workspace.classList.contains('packet-row-mode-full'));
 assert.match(h.doc.querySelector('[data-packet-number="1"]').getAttribute('style'),/--packet-row-contrast:var\(--packet-contrast-tcp\)/);
 const set=h.doc.querySelector('[data-packet-color="error"]');set.value='#7e00fd';h.doc.emit('input',set);
 assert.equal(workspace.style['--packet-color-error'],'#7e00fd');
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="reset-appearance"]'));
 assert.equal(h.c.state.packetMapSettings.rowStyle,'marker');assert.equal(h.c.state.packetMapSettings.colors.error,'#ff0000');
});

test('upgrade migrates old untouched event colors but preserves custom palette',async()=>{
 const h=harness();await h.settle();
 h.api.mergePreferences({packetMapSettings:{colors:{error:'#be3f48',warn:'#dfa344',note:'#66b9cf',udp:'#e1e1e1'}}});
 assert.equal(h.c.state.packetMapSettings.colors.error,'#ff0000');
 assert.equal(h.c.state.packetMapSettings.colors.warn,'#ffff00');
 assert.equal(h.c.state.packetMapSettings.colors.note,'#0000ff');
 assert.equal(h.c.state.packetMapSettings.colors.udp,'#e1e1e1');
 h.api.mergePreferences({packetMapSettings:{colors:{error:'#bd0944',warn:'#c4d501',note:'#1212ce'}}});
 assert.equal(h.c.state.packetMapSettings.colors.error,'#bd0944');
 assert.equal(h.c.state.packetMapSettings.colors.warn,'#c4d501');
 assert.equal(h.c.state.packetMapSettings.colors.note,'#1212ce');
});

test('backend alert severity drives matching packet row and fine map colors; individual map visibility persists',async()=>{
 const packet={number:1,relative:0,source:'192.0.2.3',destination:'192.0.2.4',protocol:'TCP',transport:'TCP',length:60,captured:60,info:'SYN',expert:[],highlight:'warn'};
 const settings={packets:[packet],total:1,paginate:true,overview:{packetMarks:'h',bins:[{protocol:'TCP',level:'warn',interesting:1}],signals:[],counts:{warn:1,error:0,note:0},interesting:1}};
 const h=harness({id:'d'.repeat(24),name:'Alert alignment',state:'completed',count:1,bytes:60,revision:2},settings);
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const row=h.doc.querySelector('[data-packet-number="1"]'),fineLine=h.doc.querySelector('[data-map-index="0"]');
 assert(row.classList.contains('packet-highlight-warn'),'packet row uses server-assigned warning');
 assert.match(row.getAttribute('style'),/--packet-color-warn/,'row tint uses warning palette');
 assert(fineLine.classList.contains('packet-map-warn'),'same frame has warning in the fine map');
 assert(h.doc.querySelector('#packetOverviewMap .packet-map-bin').classList.contains('packet-map-warn'),'overview bin has matching severity');
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="toggle-appearance"]'));
 const overviewToggle=h.doc.querySelector('[data-packet-visibility="showOverview"]'),fineToggle=h.doc.querySelector('[data-packet-visibility="showDetail"]');
 assert.equal(h.c.state.packetMapSettings.showOverview,true);assert.equal(h.c.state.packetMapSettings.showDetail,true);
 overviewToggle.checked=false;h.doc.emit('change',overviewToggle);assert.equal(h.doc.getElementById('packetOverviewMap').hidden,true);
 assert.equal(h.doc.getElementById('packetDetailMap').hidden,false,'fine map independently remains visible');
 fineToggle.checked=false;h.doc.emit('change',fineToggle);assert.equal(h.doc.getElementById('packetDetailMap').hidden,true);
 assert.equal(h.doc.getElementById('packetTableScroll')!==null,true,'packet list continues to exist with maps hidden');
 fineToggle.checked=true;h.doc.emit('change',fineToggle);assert.equal(h.doc.getElementById('packetOverviewMap').hidden,true);
 assert.equal(h.doc.getElementById('packetDetailMap').hidden,false);
 assert.equal(h.c.state.packetMapSettings.showOverview,false);
 assert.equal(h.c.state.packetMapSettings.showDetail,true);
 h.api.mergePreferences({packetMapSettings:h.c.state.packetMapSettings});
 assert.equal(h.doc.getElementById('packetOverviewMap').hidden,true);
 assert.equal(h.doc.getElementById('packetDetailMap').hidden,false);
 assert(h.saved.length>=3,'each visibility toggle is persisted');
});

test('Follow new packets changes the real scroller and requests the newest live packet window',async()=>{
 const packetCount=1250,packets=Array.from({length:1400},(_,i)=>({number:i+1,relative:i/50,source:'192.0.2.2',destination:'198.51.100.1',protocol:'TCP',length:74,captured:74,info:'Sequence '+(i+1),expert:[]}));
 const config={packets,total:packetCount,paginate:true,overview:{packetMarks:'B'.repeat(1400),bins:[],signals:[],counts:{},interesting:0}};
 const h=harness({id:'e'.repeat(24),name:'Live frames',state:'running',count:packetCount,bytes:100000,revision:1},config);
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const scroller=h.doc.getElementById('packetTableScroll');scroller.scrollHeight=packetCount*21+26;scroller.clientHeight=300;
 const follow=h.doc.getElementById('packetFollow');follow.checked=true;h.doc.emit('change',follow);await h.settle();
 assert.equal(h.api.model.follow,true);
 assert.equal(scroller.scrollTop,scroller.scrollHeight-scroller.clientHeight,'follow positions the actual DOM scroll owner at the bottom');
 config.total=1400;scroller.scrollHeight=1400*21+26;
 h.setCapture({id:'e'.repeat(24),name:'Live frames',state:'running',count:1400,bytes:112000,revision:2});
 await h.api.refresh(true);await h.settle();
 assert.equal(h.api.model.total,1400);
 assert(h.api.model.loaded.some(row=>row.number===1400),'live tail window includes most recent frame');
 assert.equal(scroller.scrollTop,scroller.scrollHeight-scroller.clientHeight,'new frames push the scroll viewport to the bottom');
 assert(h.calls.some(c=>c.path.includes('/packets?')&&c.path.includes('overview=0')),'live packet requests use lightweight window endpoint');
 const oldTop=scroller.scrollTop;follow.checked=false;h.doc.emit('change',follow);await h.settle();
 assert.equal(h.api.model.follow,false);
 assert.equal(scroller.scrollTop,oldTop,'disabling live tail does not jump to an unrelated position');
});

test('slow overview refresh does not prevent packet windows from rendering',async()=>{
 let resolveOverview;
 const pendingOverview=new Promise(resolve=>{resolveOverview=resolve;});
 const config={packets:Array.from({length:400},(_,i)=>({number:i+1,relative:i/40,source:'192.0.2.1',destination:'192.0.2.2',protocol:'UDP',length:80,captured:80,info:'Frame '+(i+1),expert:[]})),total:400,paginate:true,loadOverview:()=>pendingOverview};
 const h=harness({id:'f'.repeat(24),name:'Large capture',state:'completed',count:400,bytes:32000,revision:1},config);
 await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const scroller=h.doc.getElementById('packetTableScroll');assert(scroller,'packet table rendered independently of delayed overview');
 assert(h.doc.querySelector('[data-packet-number="400"]'),'default tracking renders the newest packet independently of the delayed overview');
 assert(h.calls.some(c=>c.path.includes('/overview?')),'overview fetched independently');
 scroller.scrollTop=6000;h.doc.emit('scroll',scroller);await h.settle();
 assert(h.api.model.windowStart>0,'packet window advanced while overview request was still unresolved');
 resolveOverview({packetMarks:'C'.repeat(400),bins:[],signals:[],counts:{},interesting:0});await h.settle();
 assert.equal(h.doc.getElementById('packetTableScroll'),scroller);
});

test('packet arrival rows and inspector retain original nanoseconds while showing actual local time and elapsed seconds',async()=>{
 const stamp='2026-11-01T05:59:59.625123456Z',base={source:'192.0.2.1',destination:'198.51.100.2',protocol:'UDP',length:80,captured:80,info:'Fixture',expert:[]},packets=[{...base,number:1,time:stamp,relative:0},{...base,number:2,time:'0001-01-01T00:00:00Z',relative:.25}];
 const h=harness({id:'1'.repeat(24),name:'Imported capture',state:'completed',count:2,bytes:160,revision:1},{packets,detail:number=>({packet:packets[number-1],layers:[],data:'',ascii:''})});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const cell=h.doc.querySelector('[data-packet-number="1"] .packet-local-time'),time=cell.querySelector('time');assert.equal(time.getAttribute('datetime'),stamp);assert.match(time.textContent,/\.625$/);assert(cell.getAttribute('title').includes(resultTime.format(stamp)));assert(cell.getAttribute('title').includes(stamp));assert.match(cell.querySelector('.packet-elapsed').textContent,/0\.000000 s/);assert(cell.getAttribute('data-copy-value').includes(stamp));
 const missing=h.doc.querySelector('[data-packet-number="2"] .packet-local-time');assert.equal(missing.querySelector('time').getAttribute('datetime'),null);assert.match(missing.textContent,/Not recorded/);assert.match(missing.textContent,/0\.250000 s/);
 h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();assert.equal(h.doc.querySelector('#packetInspector time').getAttribute('datetime'),stamp);assert.equal(h.doc.querySelector('#packetInspector time').textContent,resultTime.format(stamp));
 h.doc.emit('click',h.doc.querySelector('[data-packet-sort="time"]'));await h.settle();assert(h.calls.some(call=>call.path.includes('sort=time')),'time sorting must retain the native absolute-time sort request');
 h.doc.emit('click',h.doc.querySelector('[data-packet-action="copy-packet"]'));assert(JSON.parse(h.copied.at(-1)).packet.time===stamp,'copying details retains the original capture precision');
});

test('packet traffic intervals expose local time and measured rates on hover, pin, keyboard and refresh',async()=>{
 const started='2026-11-01T05:59:59.625123456Z',middle='2026-11-01T06:00:00.625123456Z',finished='2026-11-01T06:00:01.625123456Z';
 let statistics={started,finished,duration:2,bytes:400,timedBytes:300,timedPackets:2,untimedPackets:1,expert:0,protocols:{TCP:2,UDP:1},endpoints:[],conversations:[],timeline:[{time:started,endTime:middle,intervalSeconds:1,seconds:0,packets:1,bytes:100},{time:middle,endTime:finished,intervalSeconds:1,seconds:1,packets:1,bytes:200}]};
 const h=harness({id:'2'.repeat(24),name:'Timed capture',state:'completed',count:3,bytes:400,revision:1},{loadStatistics:()=>statistics});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="statistics"]'));await h.settle();
 let first=h.doc.querySelector('[data-packet-bucket="0"]'),last=h.doc.querySelector('[data-packet-bucket="1"]');assert(first&&last);h.doc.emit('pointerover',first);let details=h.doc.getElementById('packetTimelineDetails');assert(details.textContent.includes(resultTime.format(started)));assert.match(details.textContent,/800 bit\/s/);h.doc.emit('click',first);assert.equal(first.getAttribute('aria-pressed'),'true');h.doc.emit('pointerout',first);assert.match(details.textContent,/800 bit\/s/);
 h.doc.emit('pointerover',last);assert.match(h.doc.getElementById('packetGraphPopup').textContent,/1,?600 bit\/s/);assert.match(details.textContent,/800 bit\/s/,'hover keeps the pinned details stable');h.doc.emit('pointerout',last);assert.match(details.textContent,/800 bit\/s/);
 h.doc.emit('keydown',first,{key:'End'});assert.equal(h.doc.querySelector('[data-packet-bucket="1"]').getAttribute('aria-pressed'),'true');assert.match(details.textContent,/1,?600 bit\/s/);assert(details.textContent.includes(resultTime.format(middle)),'adjacent interval boundaries must select the later bucket');
 const reader=h.doc.querySelector('.packet-summary-scroll');reader.scrollTop=123;await h.api.refresh(true);details=h.doc.getElementById('packetTimelineDetails');assert.equal(h.api.model.timelineState.index,1);assert.match(details.textContent,/1,?600 bit\/s/);assert.equal(h.doc.querySelector('.packet-summary-scroll').scrollTop,123);assert.match(h.doc.querySelector('.packet-protocol-distribution').textContent,/66\.7%/);assert.equal(h.doc.querySelectorAll('.packet-protocol-share').length,2);
 h.doc.emit('keydown',h.doc.querySelector('[data-packet-bucket="1"]'),{key:'Escape'});assert.equal(h.api.model.timelineState.pinned,false);assert.match(h.doc.getElementById('packetTimelineDetails').textContent,/Hover over/);
 statistics={...statistics,finished:started,duration:0,timedBytes:100,timedPackets:1,untimedPackets:0,timeline:[{time:started,endTime:started,intervalSeconds:0,seconds:0,packets:1,bytes:100}]};await h.api.refresh(true);h.doc.emit('click',h.doc.querySelector('[data-packet-bucket="0"]'));assert.match(h.doc.getElementById('packetTimelineDetails').textContent,/Not measured/);assert.doesNotMatch(h.doc.getElementById('packetTimelineDetails').textContent,/Infinity|NaN/);
 statistics={...statistics,started:undefined,finished:undefined,timedBytes:0,timedPackets:0,untimedPackets:1,timeline:[]};await h.api.refresh(true);assert.equal(h.doc.querySelectorAll('[data-packet-bucket]').length,0);assert.match(h.doc.querySelector('[data-packet-timeline]').textContent,/No packets with recorded timestamps/);assert.match(h.doc.querySelector('[data-packet-timeline]').textContent,/excluded from the time graph/);
});

test('packet endpoint and conversation summaries show observed first and last times without inventing missing values',async()=>{
 const start='2026-10-26T14:30:00.123456789Z',end='2026-10-26T14:30:02.123456789Z',statistics={endpoints:[{address:'192.0.2.1',packets:2,sent:1,received:1,bytes:200,start,end}],conversations:[{id:'flow',protocol:'TCP',a:'192.0.2.1:1234',b:'198.51.100.2:443',packets:2,aToB:1,bToA:1,bytes:200,duration:2,resets:0,retransmissions:0,start,end}]};
 const h=harness({id:'3'.repeat(24),name:'Summary capture',state:'completed',count:2,bytes:200,revision:1},{statistics});await h.settle();for(const view of ['endpoints','conversations']){h.doc.emit('click',h.doc.querySelector('[data-packet-view="'+view+'"]'));await h.settle();const cells=h.doc.querySelectorAll('.packet-observed-time');assert.equal(cells.length,2);assert.equal(cells[0].textContent,resultTime.format(start));assert.equal(cells[1].textContent,resultTime.format(end));}
});


test('packet graph retains an exact snapshot across hover and live rebucketing until an outside view click',async()=>{
 const start='2026-10-26T14:30:00Z',middle='2026-10-26T14:30:01Z',end='2026-10-26T14:30:02Z';
 let statistics={started:start,finished:end,duration:2,bytes:300,timedBytes:300,timedPackets:2,untimedPackets:0,protocols:{TCP:2},timeline:[{time:start,endTime:middle,intervalSeconds:1,packets:1,bytes:100},{time:middle,endTime:end,intervalSeconds:1,packets:1,bytes:200}]};
 const capture={id:'4'.repeat(24),name:'Pinned capture',state:'running',count:2,bytes:300,revision:1},h=harness(capture,{loadStatistics:()=>statistics});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 let point=h.doc.querySelector('[data-packet-bucket="0"]');h.doc.emit('click',point);const pin=h.api.model.timelineState.pin;h.doc.emit('pointerover',h.doc.querySelector('[data-packet-bucket="1"]'),{clientX:300,clientY:150});assert.equal(point.getAttribute('aria-pressed'),'true');assert.match(h.doc.getElementById('packetTimelineDetails').textContent,/800 bit\/s/);assert.match(h.doc.getElementById('packetGraphPopup').textContent,/1,?600 bit\/s/);h.doc.emit('pointerout',h.doc.querySelector('[data-packet-bucket="1"]'));assert.equal(h.doc.getElementById('packetGraphPopup'),null);
 statistics={...statistics,timeline:[{time:start,endTime:end,intervalSeconds:2,packets:2,bytes:300}]};h.setCapture({...capture,revision:2});await h.api.refresh(true);assert.equal(h.api.model.timelineState.pinned,true);assert.equal(h.api.model.timelineState.pin,pin);assert.equal(h.api.model.timelineState.index,null,'a regrouped interval must not silently replace the selected interval');assert.match(h.doc.getElementById('packetTimelineDetails').textContent,/regrouped/);assert.match(h.doc.getElementById('packetTimelineDetails').textContent,/800 bit\/s/);assert.equal(h.doc.querySelector('[data-packet-bucket="0"]').getAttribute('aria-pressed'),'false');
 h.doc.emit('click',h.doc.querySelector('[data-packet-view="statistics"]'));await h.settle();assert.equal(h.api.model.timelineState.pin,null,'clicking outside the graph clears the retained point');assert.equal(h.api.model.timelineState.pinned,false);
});

test('packet graph and packet rows highlight each other with half-open interval boundaries and unknown times excluded',async()=>{
 const a='2026-10-26T14:30:00Z',b='2026-10-26T14:30:01Z',c='2026-10-26T14:30:02Z',base={source:'192.0.2.1',destination:'198.51.100.2',protocol:'TCP',length:100,captured:100,info:'fixture',expert:[]};
 const packets=[{...base,number:1,time:a,relative:0},{...base,number:2,time:b,relative:1},{...base,number:3,time:c,relative:2},{...base,number:4,relative:3}],statistics={started:a,finished:c,duration:2,bytes:400,timedBytes:300,timedPackets:3,untimedPackets:1,protocols:{TCP:4},timeline:[{time:a,endTime:b,intervalSeconds:1,packets:1,bytes:100},{time:b,endTime:c,intervalSeconds:1,packets:2,bytes:200}]};
 const h=harness({id:'5'.repeat(24),name:'Correlated',state:'completed',count:4,bytes:400,revision:1},{packets,statistics});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const row=n=>h.doc.querySelector('#packetRows [data-packet-number="'+n+'"]'),first=h.doc.querySelector('[data-packet-bucket="0"]'),last=h.doc.querySelector('[data-packet-bucket="1"]');
 h.doc.emit('pointerover',first);assert(row(1).classList.contains('packet-graph-hovered'));assert(!row(2).classList.contains('packet-graph-hovered'),'boundary time belongs to the later interval');assert(!row(4).classList.contains('packet-graph-hovered'));h.doc.emit('pointerout',first);
 h.doc.emit('pointerover',row(2));assert(last.classList.contains('packet-bucket-hovered'));assert(row(2).classList.contains('packet-graph-hovered'));assert(row(3).classList.contains('packet-graph-hovered'),'the final observed endpoint is included');assert(!row(4).classList.contains('packet-graph-hovered'));h.doc.emit('pointerout',row(2));assert(!last.classList.contains('packet-bucket-hovered'));
 h.doc.emit('click',last);assert(row(2).classList.contains('packet-graph-pinned'));assert(row(3).classList.contains('packet-graph-pinned'));h.doc.emit('pointerover',row(4));assert.equal(h.doc.getElementById('packetGraphPopup'),null,'unknown timestamps cannot highlight a guessed interval');
});

test('progressive packet graphs grow with observed time, retain horizontal position and follow only at the end or by choice',async()=>{
 const start='2026-10-26T14:30:00Z',end='2026-10-26T14:31:40Z';let statistics={started:start,finished:end,duration:100,bytes:100,timedBytes:100,timedPackets:1,untimedPackets:0,protocols:{TCP:1},timeline:[{time:start,endTime:end,intervalSeconds:100,packets:1,bytes:100}]};
 const capture={id:'6'.repeat(24),name:'Progressive',state:'running',count:1,bytes:100,revision:1},h=harness(capture,{loadStatistics:()=>statistics});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();const mode=h.doc.querySelector('[data-packet-graph-mode]');mode.value='progressive';h.doc.emit('change',mode);assert.equal(h.c.state.graphPreferences.mode,'progressive');assert.match(h.doc.querySelector('.packet-timeline').getAttribute('style'),/width:1200px/);assert.match(h.doc.querySelector('.packet-timeline-aggregation').textContent,/may regroup/);
 let scroll=h.doc.querySelector('[data-packet-graph-scroll]');Object.assign(scroll,{scrollWidth:1200,clientWidth:600,scrollLeft:125});h.doc.emit('scroll',scroll);assert.equal(h.api.model.timelineState.follow,false);assert.equal(h.api.model.timelineState.scrollLeft,125);
 statistics={...statistics,finished:'2026-10-26T14:33:20Z',duration:200,timeline:[{time:start,endTime:'2026-10-26T14:33:20Z',intervalSeconds:200,packets:1,bytes:100}]};h.setCapture({...capture,revision:2});await h.api.refresh(true);scroll=h.doc.querySelector('[data-packet-graph-scroll]');assert.equal(scroll.scrollLeft,125);assert.match(h.doc.querySelector('.packet-timeline').getAttribute('style'),/width:2400px/);Object.assign(scroll,{scrollWidth:2400,clientWidth:600});h.doc.emit('click',h.doc.querySelector('[data-packet-graph-follow]'));assert.equal(scroll.scrollLeft,1800);assert.equal(h.api.model.timelineState.follow,true);
 const chosen=h.doc.querySelector('[data-packet-graph-mode]');chosen.value='window';h.doc.emit('change',chosen);assert.equal(h.c.state.graphPreferences.mode,'window');assert.equal(h.doc.querySelector('.packet-timeline').getAttribute('style'),null);
});

test('packet graph is above the independent packet reader and timestamp preference refresh preserves its state without requests',async()=>{
 const stamp='2026-10-26T14:30:00.123456789Z',base={source:'192.0.2.1',destination:'198.51.100.2',protocol:'TCP',length:100,captured:100,info:'fixture',expert:[]},packet={...base,number:1,time:stamp,relative:0},statistics={started:stamp,finished:stamp,duration:0,bytes:100,timedBytes:100,timedPackets:1,untimedPackets:0,protocols:{TCP:1},timeline:[{time:stamp,endTime:stamp,intervalSeconds:0,packets:1,bytes:100}]};
 const h=harness({id:'7'.repeat(24),name:'Preferences',state:'completed',count:1,bytes:100,revision:1},{packets:[packet],statistics,detail:{packet,layers:[],data:'',ascii:''}});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();const dock=h.doc.getElementById('packetGraphDock'),analysis=h.doc.getElementById('packetCaptureAnalysis');assert.equal(dock.parentElement,analysis.parentElement);assert(!dock.hidden);assert.equal(h.doc.getElementById('packetTableScroll').closest('#packetGraphDock'),null);assert(h.doc.getElementById('packetGraphDivider'));
 h.doc.emit('click',h.doc.querySelector('[data-packet-number="1"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-bucket="0"]'));const pin=h.api.model.timelineState.pin,scroller=h.doc.getElementById('packetTableScroll');scroller.scrollTop=75;h.api.model.sort='time';const count=h.calls.length;h.c.state.resultTimePreferences={hourFormat:'12',zoneMode:'utc',timeZone:'UTC'};h.api.refreshTimeDisplay();assert.equal(h.calls.length,count);assert.equal(h.doc.getElementById('packetTableScroll'),scroller);assert.equal(scroller.scrollTop,75);assert.equal(h.api.model.sort,'time');assert.equal(h.api.model.timelineState.pin,pin);assert.match(h.doc.querySelector('.packet-local-time time').textContent,/PM/);assert.match(h.doc.querySelector('#packetInspector time').textContent,/PM/);assert.match(h.doc.getElementById('packetTimelineDetails').textContent,/Start \(UTC\)/);assert.equal(h.doc.querySelector('.packet-local-time time').getAttribute('datetime'),stamp);
 h.doc.emit('keydown',h.doc.getElementById('packetGraphDivider'),{key:'ArrowDown'});assert.equal(h.c.state.packetGraphPreferences.height,245);h.doc.emit('keydown',h.doc.getElementById('packetGraphDivider'),{key:'Home'});assert.equal(h.c.state.packetGraphPreferences.height,230);
});


test('packet correlation preserves nanosecond interval boundaries instead of rounding them to milliseconds',async()=>{
 const a='2026-10-26T14:30:00.123456789Z',b='2026-10-26T14:30:01.123456789Z',before='2026-10-26T14:30:01.123456788Z',c='2026-10-26T14:30:02.123456789Z',base={source:'192.0.2.1',destination:'198.51.100.2',protocol:'TCP',length:100,captured:100,info:'fixture',expert:[]};
 const packets=[{...base,number:1,time:before,relative:1},{...base,number:2,time:b,relative:1}],statistics={started:a,finished:c,duration:2,bytes:200,timedBytes:200,timedPackets:2,untimedPackets:0,protocols:{TCP:2},timeline:[{time:a,endTime:b,intervalSeconds:1,packets:1,bytes:100},{time:b,endTime:c,intervalSeconds:1,packets:1,bytes:100}]};
 const h=harness({id:'8'.repeat(24),name:'Precise boundaries',state:'completed',count:2,bytes:200,revision:1},{packets,statistics});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();h.doc.emit('pointerover',h.doc.querySelector('[data-packet-bucket="0"]'));assert(h.doc.querySelector('[data-packet-number="1"]').classList.contains('packet-graph-hovered'));assert(!h.doc.querySelector('[data-packet-number="2"]').classList.contains('packet-graph-hovered'));h.doc.emit('pointerout',h.doc.querySelector('[data-packet-bucket="0"]'));h.doc.emit('pointerover',h.doc.querySelector('[data-packet-bucket="1"]'));assert(!h.doc.querySelector('[data-packet-number="1"]').classList.contains('packet-graph-hovered'));assert(h.doc.querySelector('[data-packet-number="2"]').classList.contains('packet-graph-hovered'));
});

test('compact packet layout reserves readable rows after wrapped list controls and clamps only the displayed graph height',async()=>{
 const h=harness({id:'9'.repeat(24),name:'Compact layout',state:'completed',count:1,bytes:100,revision:1});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const area=h.doc.getElementById('packetCaptureAnalysis'),dock=h.doc.getElementById('packetGraphDock'),panel=area.querySelector('.packet-list-panel'),table=h.doc.getElementById('packetTableScroll');
 area.getBoundingClientRect=()=>({width:360,height:180});dock.getBoundingClientRect=()=>({width:360,height:230});panel.getBoundingClientRect=()=>({width:360,height:144});table.getBoundingClientRect=()=>({width:360,height:50});h.c.state.packetDetails.hidden=true;h.c.state.packetGraphPreferences={height:500};h.api.layoutDetails();
 assert(parseFloat(area.style.minHeight)-94-36>=140,'the list reserve includes all wrapped toolbar/footer height plus readable packet rows');assert(parseFloat(dock.style.flexBasis)<=180+230-parseFloat(area.style.minHeight),'the graph leaves that reserve available');assert.equal(h.c.state.packetGraphPreferences.height,500,'a small window does not overwrite the saved graph size');assert(h.doc.getElementById('packetCaptureWorkspace').classList.contains('packet-has-traffic-graph'));
 h.c.state.packetDetails.hidden=false;h.api.layoutDetails();assert(parseFloat(area.style.minHeight)-94-86>=140,'an expanded inspector is accounted for separately');h.doc.emit('click',h.doc.querySelector('[data-packet-view="statistics"]'));await h.settle();assert.equal(area.style.minHeight,'');assert(!h.doc.getElementById('packetCaptureWorkspace').classList.contains('packet-has-traffic-graph'));
});


test('live packet captions refresh filtered and retained counts without replacing the reader or focused controls',async()=>{
 const packet=number=>({number,time:'2026-10-26T14:30:00Z',relative:number/10,source:'192.0.2.1',destination:'198.51.100.2',protocol:'TCP',length:100,captured:100,info:'fixture',expert:[]}),capture={id:'a'.repeat(24),name:'Growing capture',state:'running',count:10,bytes:1000,revision:1},config={packets:[packet(1),packet(2),packet(3)],total:3};
 const h=harness(capture,config);await h.settle();h.api.model.expression='tcp';h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();const caption=h.doc.querySelector('.packet-list-scope'),reader=h.doc.getElementById('packetTableScroll'),control=caption.querySelector('[data-packet-action="toggle-insights"]');assert.match(caption.textContent,/3 displayed of 10 retained packets/);reader.scrollTop=75;reader.scrollLeft=24;h.doc.activeElement=control;
 config.packets=Array.from({length:12},(_,i)=>packet(i+1));config.total=12;h.setCapture({...capture,count:81,bytes:8100,revision:2});await h.api.refresh(true);
 assert.equal(h.doc.querySelector('.packet-list-scope'),caption);assert.match(caption.textContent,/12 displayed of 81 retained packets/,'display-filter matches and all retained packets use their distinct current counts');assert.equal(h.doc.getElementById('packetTableScroll'),reader);assert.equal(reader.scrollTop,75);assert.equal(reader.scrollLeft,24);assert.equal(h.doc.activeElement,control);assert.equal(caption.querySelector('[data-packet-action="toggle-insights"]'),control);assert.equal(h.api.model.expression,'tcp');
});

test('packet zoom keeps absolute bucket indices, raw pins, severity details and independent latest controls',async()=>{
 const a='2026-10-26T14:30:00.123456789Z',b='2026-10-26T14:30:01.123456789Z',c='2026-10-26T14:30:01.323456789Z',base={source:'192.0.2.1',destination:'198.51.100.2',protocol:'TCP',transport:'TCP',length:100,captured:100,info:'fixture',expert:[]};
 const packets=[{...base,number:1,time:a},{...base,number:2,time:b}],statistics={started:a,finished:c,duration:1.2,bytes:200,timedBytes:200,protocols:{TCP:2},timeline:[{time:a,endTime:b,intervalSeconds:1,packets:1,bytes:100,severities:{error:1}},{time:b,endTime:c,intervalSeconds:.2,packets:1,bytes:100,severities:{normal:1}}]};
 const h=harness({id:'2'.repeat(24),name:'Zoom proof',state:'completed',count:2,revision:1},{packets,statistics});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 const summary=h.doc.getElementById('packetGraphSummary'),dock=h.doc.getElementById('packetGraphDock');assert.equal(summary.parentElement,dock);assert(dock.children.indexOf(summary)<dock.children.indexOf(h.doc.getElementById('packetGraphReader')));
 let first=h.doc.querySelector('[data-packet-bucket="0"]');assert(Math.abs(Number(first.querySelector('.packet-timeline-hit').getAttribute('width'))-500)<.001,'partial final interval retains proportional absolute-time hit geometry');assert.equal(first.dataset.packetSeverity,'error');h.doc.emit('click',first);const pin=h.api.model.timelineState.pin;
 h.doc.emit('click',h.doc.querySelector('[data-packet-zoom="in"]'));assert.equal(h.c.state.packetGraphPreferences.zoom,2);assert.match(h.doc.querySelector('.packet-timeline').getAttribute('style'),/min-width:200%/);assert.equal(h.api.model.timelineState.pin,pin);assert.equal(h.doc.querySelector('[data-packet-bucket="0"]').getAttribute('aria-pressed'),'true');
 h.doc.emit('pointerover',h.doc.querySelector('[data-packet-bucket="0"]'),{clientX:20,clientY:30});assert.match(h.doc.getElementById('packetGraphPopup').textContent,/1 decoder errors/);h.doc.emit('pointerout',h.doc.querySelector('[data-packet-bucket="0"]'));assert.equal(h.api.model.timelineState.pin,pin,'pointer leave retains exact snapshot');assert.equal(pin.time,a);
 const graph=h.doc.querySelector('[data-packet-graph-scroll]');graph.scrollWidth=1800;graph.clientWidth=600;graph.scrollLeft=400;h.doc.emit('scroll',graph);h.doc.emit('click',h.doc.querySelector('[data-packet-action="scroll-last"]'));await h.settle();assert.equal(graph.scrollLeft,400,'list Latest does not move graph');h.doc.emit('click',h.doc.querySelector('[data-packet-graph-latest]'));assert.equal(graph.scrollLeft,1200);
 assert.equal(h.api.model.timelineState.pinned,false,'list Latest is an outside-graph click');h.doc.emit('click',h.doc.querySelector('[data-packet-bucket="0"]'));const repin=h.api.model.timelineState.pin;h.doc.emit('click',h.doc.querySelector('[data-packet-zoom="reset"]'));assert.equal(h.c.state.packetGraphPreferences.zoom,1);assert.equal(h.api.model.timelineState.pin,repin);
 h.api.model.sort='source';h.api.model.direction='desc';h.doc.emit('click',h.doc.querySelector('.packet-list-footer'));assert.equal(h.api.model.timelineState.pinned,false);assert.equal(h.api.model.sort,'source');assert.equal(h.api.model.direction,'desc');
 h.doc.emit('click',h.doc.querySelector('[data-packet-graph-collapse]'));assert.equal(h.doc.getElementById('packetGraphReader'),null);assert.equal(h.doc.getElementById('packetGraphDivider'),null);assert(h.doc.getElementById('packetGraphSummary'));assert.equal(h.doc.querySelector('[data-packet-graph-collapse]').getAttribute('aria-expanded'),'false');assert.equal(h.c.state.packetGraphPreferences.collapsed,true);
 h.api.mergePreferences({packetGraphPreferences:{height:300,zoom:4,collapsed:true}});h.doc.emit('click',h.doc.querySelector('[data-packet-graph-collapse]'));assert(h.doc.getElementById('packetGraphReader'));assert.equal(h.c.state.packetGraphPreferences.zoom,4);
});

test('packet statistic, severity and recorded-type filters compose on the full API and matching export',async()=>{
 const stamp='2026-10-26T14:30:00.123456789Z',packets=[{number:1,time:stamp,source:'192.0.2.1',destination:'192.0.2.2',protocol:'DNS',length:100,captured:100,info:'SERVFAIL',expert:['decoder note'],highlight:'warn'}],baseStats={protocols:{DNS:1,TCP:2},bytes:600,expert:1,duration:2,timedBytes:600,started:stamp,finished:'2026-10-26T14:30:02.123456789Z',timeline:[{time:stamp,endTime:'2026-10-26T14:30:02.123456789Z',intervalSeconds:2,packets:3,bytes:600,severities:{warn:1,normal:2}}]},fetches=[];
 const h=harness({id:'3'.repeat(24),name:'Full predicate',state:'completed',count:3,revision:1},{packets,statistics:baseStats,fetch:async(url)=>{fetches.push(url);return {ok:true,blob:async()=>new Blob(['fixture'])};}});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();
 h.doc.getElementById('packetDisplayFilter').value='dns || tcp';h.doc.getElementById('packetSearch').value='query';h.doc.emit('click',h.doc.querySelector('[data-packet-action="filter"]'));await h.settle();
 const severity=h.doc.getElementById('packetSeverityFilter');severity.value='warn';h.doc.emit('change',severity);await h.settle();const type=h.doc.getElementById('packetTypeFilter');type.value='DNS';h.doc.emit('change',type);await h.settle();
 h.doc.emit('click',h.doc.querySelector('[data-packet-stat="expert"]'));await h.settle();const params=h.api.parameters();assert.equal(params.get('filter'),'(dns || tcp) && (frame.severity == "warn") && (protocol == "DNS") && (expert)');assert.equal(params.get('query'),'query');assert.equal(h.doc.querySelector('[data-packet-stat="expert"]').getAttribute('aria-pressed'),'true');assert(h.doc.querySelector('[data-packet-clear-stat]'));assert(h.calls.some(call=>call.path.includes('/packets?')&&new URLSearchParams(call.path.split('?')[1]).get('filter')===params.get('filter')));
 h.doc.getElementById('packetExportFormat').value='pcapng';h.doc.emit('click',h.doc.querySelector('[data-packet-action="export"]'));await h.settle();assert.equal(new URLSearchParams(fetches[0].split('?')[1]).get('filter'),params.get('filter'));
 h.doc.emit('click',h.doc.querySelector('[data-packet-stat="average"]'));await h.settle();assert(!h.api.parameters().get('filter').includes('frame.len =='),'an average must not invent an exact packet value');assert.match(h.doc.querySelector('.packet-stat-filter-label').textContent,/aggregate reference/);assert(h.doc.querySelector('[data-packet-bucket="0"]').classList.contains('packet-stat-matched'));
 h.doc.emit('pointerover',h.doc.querySelector('[data-packet-bucket="0"]'));assert.match(h.doc.getElementById('packetGraphPopup').textContent,/Aggregate average reference/);h.doc.emit('pointerout',h.doc.querySelector('[data-packet-bucket="0"]'));
 h.doc.emit('click',h.doc.querySelector('[data-packet-clear-stat]'));await h.settle();assert.equal(h.api.model.statFilter,'');assert.equal(h.api.model.severityFilter,'warn');assert.equal(h.api.model.typeFilter,'DNS');assert.equal(h.api.model.query,'query');assert.equal(h.doc.getElementById('packetTypeFilter').value,'DNS');
 h.doc.emit('click',h.doc.querySelector('[data-packet-stat="duration"]'));await h.settle();assert.match(h.api.parameters().get('filter'),/frame.time_recorded/);h.doc.emit('click',h.doc.querySelector('[data-packet-action="reset-filter"]'));await h.settle();assert.equal(h.api.parameters().get('filter'),null);assert.equal(h.api.model.typeFilter,'any');assert.equal(h.api.model.severityFilter,'any');assert.equal(baseStats.bytes,600,'display filters preserve source statistics');
});

test('packet auto tracking uses global newest index under every sort and suspends keyboard or text inspection',async()=>{
 const count=1200,stamp='2026-10-26T14:30:00Z',source=Array.from({length:count},(_,i)=>({number:i+1,time:stamp,source:'192.0.2.'+(i%8),destination:'192.0.2.250',protocol:'TCP',length:100,captured:100,info:'fixture',expert:[]}));
 const loadPackets=params=>{const rows=[...source],field=params.get('sort'),direction=params.get('direction');if(field==='source')rows.sort((a,b)=>a.source.localeCompare(b.source)||a.number-b.number);if(direction==='desc')rows.reverse();const offset=Number(params.get('offset')||0),limit=Number(params.get('limit')||300);return {total:rows.length,offset,packets:rows.slice(offset,offset+limit),newestIndex:rows.findIndex(row=>row.number===count)};};
 const h=harness({id:'4'.repeat(24),name:'Newest tracking',state:'running',count,revision:1},{loadPackets});await h.settle();h.doc.emit('click',h.doc.querySelector('[data-packet-view="packets"]'));await h.settle();let reader=h.doc.getElementById('packetTableScroll');reader.clientHeight=300;reader.scrollHeight=count*21+26;assert.equal(h.api.model.follow,true);await h.api.followPacketLatest();assert(h.api.model.loaded.some(packet=>packet.number===count));assert.equal(reader.scrollTop,reader.scrollHeight-reader.clientHeight);
 h.doc.emit('click',h.doc.querySelector('[data-packet-sort="number"]'));await h.settle();reader=h.doc.getElementById('packetTableScroll');reader.clientHeight=300;assert.equal(h.api.model.direction,'desc');await h.api.followPacketLatest();assert.equal(reader.scrollTop,0);assert.equal(h.api.model.loaded[0].number,count);
 h.doc.emit('click',h.doc.querySelector('[data-packet-sort="source"]'));await h.settle();reader=h.doc.getElementById('packetTableScroll');reader.clientHeight=300;reader.scrollHeight=count*21+26;await h.api.followPacketLatest();assert(h.api.model.loaded.some(packet=>packet.number===count));const newest=source.slice().sort((a,b)=>a.source.localeCompare(b.source)||a.number-b.number).findIndex(packet=>packet.number===count);assert.equal(h.api.model.newestIndex,newest);assert.equal(reader.scrollTop,newest*(h.api.model.measuredRowHeight||21)+(h.api.model.measuredHeaderHeight||26)-150,'global sorted index uses the measured row and header sizes');
 const row=h.doc.querySelector('[data-packet-number="1200"]');h.doc.emit('keydown',row,{key:'Tab'});h.doc.emit('focusin',row);reader.scrollTop=88;await h.api.followPacketLatest();assert.equal(reader.scrollTop,88);h.doc.emit('focusout',row);await h.settle();assert.notEqual(reader.scrollTop,88);
 h.c.window.getSelection=()=>({isCollapsed:false,toString:()=> 'copied packet field'});reader.scrollTop=99;await h.api.followPacketLatest();assert.equal(reader.scrollTop,99);h.c.window.getSelection=()=>({isCollapsed:true,toString:()=>''});h.doc.emit('selectionchange',h.doc);await h.settle();assert.notEqual(reader.scrollTop,99);
 h.doc.getElementById('packetFollow').checked=false;h.doc.emit('change',h.doc.getElementById('packetFollow'));await h.settle();reader.scrollTop=77;await h.api.followPacketLatest();assert.equal(reader.scrollTop,77);h.doc.emit('click',h.doc.querySelector('[data-packet-action="scroll-last"]'));await h.settle();assert.notEqual(reader.scrollTop,77);assert.equal(h.api.model.follow,true);
});

test('packet fallback clue colors agree with full-capture recorded severity predicates',async()=>{
 const h=harness();await h.settle();assert.equal(h.api.packetLevel({protocol:'ICMP',info:'Destination unreachable'}),'warn');assert.equal(h.api.packetLevel({protocol:'ICMPv6',info:'Administratively prohibited'}),'warn');assert.equal(h.api.packetLevel({protocol:'TCP',transport:'TCP',info:'Out-of-order'}),'note');assert.equal(h.api.packetLevel({protocol:'TCP',transport:'TCP',info:'Normal acknowledgement'}),'');assert.equal(h.api.packetLevel({protocol:'TCP',highlight:'warn',expert:[]}),'warn');
});

test('troubleshooting summary counters focus exact contributor predicates without replacing existing filters or sort',async()=>{
 const h=harness({id:'5'.repeat(24),name:'Summary actions',state:'completed',count:5,revision:1});await h.settle();h.api.model.expression='tcp || dns';h.api.model.query='192.0.2';h.api.model.severityFilter='note';h.api.model.typeFilter='TCP';h.api.model.sort='source';h.api.model.direction='desc';
 assert.equal(h.doc.querySelectorAll('.packet-review-metrics [data-packet-stat]').length,6);
 h.doc.emit('click',h.doc.querySelector('.packet-review-metrics [data-packet-stat="tcpstarts"]'));await h.settle();assert.equal(h.api.model.view,'packets');assert.equal(h.api.model.statFilter,'tcpstarts');assert.equal(h.api.model.sort,'source');assert.equal(h.api.model.direction,'desc');assert.equal(h.api.model.query,'192.0.2');assert.match(h.api.parameters().get('filter'),/\(tcp \|\| dns\).*frame.severity == "note".*protocol == "TCP".*tcp.flags.syn && !tcp.flags.ack && !tcp.flags.fin && !tcp.flags.rst/);assert.match(h.doc.querySelector('.packet-stat-filter-label').textContent,/distinct connection attempts/);assert.match(h.doc.querySelector('.packet-stat-filter-label').textContent,/more than one packet/);
 for(const [key,expression]of [['dnsrequests','dns.request'],['dnsreplies','dns.response'],['tcpresets','tcp.flags.rst'],['icmperrors','icmp.error']]){h.doc.emit('click',h.doc.querySelector('[data-packet-view="troubleshooting"]'));await h.settle();h.doc.emit('click',h.doc.querySelector('.packet-review-metrics [data-packet-stat="'+key+'"]'));await h.settle();assert(h.api.parameters().get('filter').includes('('+expression+')'));assert.equal(h.api.model.query,'192.0.2');}
 h.api.model.typeFilter='ImportedUnknownProtocol';h.api.model.severityFilter='error';h.api.model.statFilter='expert';h.api.model.sequence++;await h.api.refresh(true);assert.equal(h.doc.getElementById('packetTypeFilter').value,'ImportedUnknownProtocol');assert(h.doc.getElementById('packetTypeFilter').querySelectorAll('option').some(option=>option.getAttribute('value')==='ImportedUnknownProtocol'),'active type remains visible even under an empty or restricted combination');
});
