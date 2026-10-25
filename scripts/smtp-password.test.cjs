'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const {document:makeDocument}=require('./terminal-dom-fixture.cjs');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8'),lines=html.split('\n');
const startJob=html.slice(html.indexOf('async function startJob(){'),html.indexOf('\nfunction useTarget('));
const deferred=()=>{let resolve,reject;const promise=new Promise((a,b)=>{resolve=a;reject=b});return {promise,resolve,reject};};
function harness(respond){
 const document=makeDocument('<aside id="sidebar"></aside><div id="workspaceScroll"></div><div id="options"></div><button id="runButton"></button>');
 const calls=[],saved=[],messages=[],state={activePage:'smtp',forms:{smtp:{}}};
 const c={document,state,console,online:true,shutDown:false,TERMINAL_ONLY:false,jobs:[],selected:{},fullVersions:new Map(),jobVersions:new Map(),lastRender:'',
  $:selector=>document.querySelector(selector),esc:value=>String(value??'').replaceAll('&','&amp;').replaceAll('"','&quot;').replaceAll('<','&lt;'),
  activeTool:()=>state.activePage==='smtp'?'smtp':null,runningToolJob:()=>null,currentTarget:()=> 'smtp.example.test',LOCAL_TOOLS:[],SUBNET_TOOLS:[],
  networkMode:()=>false,parseSubnet:()=>{throw Error('Not a CIDR')},NetworkToolkitSource:{validation:()=>'',requestOptions:()=>({})},
  toast:(...args)=>messages.push(args),revealToolConfiguration(){},fullVersion:job=>job.id,renderCurrentResult(){},splitToolPage:()=>false,updateEngine(){},pollSoon(){},
  clearWifiSecret(){},closeEntryContextMenu(){},loadCurrentJob(){},resetToolScroll(){},saveState:()=>saved.push(structuredClone(state)),
  handleSubnetPlanInput:()=>false,handleSubnetPlanChange:()=>false,handleLocalConfigurationInput:()=>false,handleTerminalInput:()=>false,
  handleLocalConfigurationChange:()=>false,handleExtendedChange:()=>false,
  api:async(path,method,body)=>{calls.push({path,method,body,sent:structuredClone(body)});return respond?respond(body):{id:'smtp-'+calls.length,tool:'smtp',status:'completed'};}};
 c.window=c;document.getElementById('workspaceScroll').scrollTo=()=>{};vm.createContext(c);
 vm.runInContext('const TOOLS={'+lines.find(line=>line.startsWith('smtp:{'))+'};',c);
 vm.runInContext("function toolFields(t){return TOOLS[t]}\n"+lines.find(line=>line.startsWith('const toolOptions=')),c);
 for(const name of ['fieldHTML','navigate'])vm.runInContext(lines.find(line=>line.startsWith('function '+name+'(')),c);
 for(const type of ['input','change'])vm.runInContext(lines.find(line=>line.startsWith("document.addEventListener('"+type+"',")),c);
 c.renderPage=()=>{document.getElementById('options').innerHTML=state.activePage==='smtp'?c.fieldHTML('smtp',{id:'password',name:'Password',type:'password'}):'';};
 vm.runInContext(startJob,c);c.renderPage();
 return {c,document,state,calls,saved,messages,password:()=>document.getElementById('opt-smtp-password')};
}
for(const status of ['completed','failed'])test('SMTP password survives a '+status+' test and is reused on the next run',async()=>{
 const h=harness(()=>({id:'smtp-result',tool:'smtp',status})),field=h.password();field.value='repeat-test-secret';
 await h.c.startJob();assert.equal(h.password(),field);assert.equal(field.value,'repeat-test-secret');
 await h.c.startJob();assert.deepEqual(h.calls.map(call=>call.sent.options.password),['repeat-test-secret','repeat-test-secret']);
 assert(h.calls.every(call=>call.body.options.password===''),'Temporary request options are scrubbed');
 assert(!JSON.stringify(h.state).includes('repeat-test-secret'));assert.equal(h.messages.length,0);
});
test('SMTP request failures leave the current password available for retry',async()=>{
 const h=harness(()=>{throw Error('Connection unavailable')});h.password().value='retry-secret';
 await h.c.startJob();await h.c.startJob();assert.equal(h.password().value,'retry-secret');
 assert.deepEqual(h.calls.map(call=>call.sent.options.password),['retry-secret','retry-secret']);
 assert(h.calls.every(call=>call.body.options.password===''));assert.equal(h.messages[0][0],'Connection unavailable');
});
test('SMTP input events never copy the password to saved state and navigation returns an empty field',async()=>{
 const h=harness(),field=h.password();field.value='navigation-secret';
 h.document.emit('input',field);h.document.emit('change',field);assert.equal(h.saved.length,0);
 await h.c.startJob();h.c.navigate('overview');assert.equal(h.password(),null);h.c.navigate('smtp');assert.equal(h.password().value,'');
 assert(!JSON.stringify(h.state).includes('navigation-secret'));assert(!JSON.stringify(h.saved).includes('navigation-secret'));
 assert.equal(h.password().getAttribute('type'),'password');assert(h.password().hasAttribute('data-transient-secret'));
});
for(const reject of [false,true])test('a late SMTP '+(reject?'failure':'response')+' cannot restore or erase a newly entered password after navigation',async()=>{
 const pending=deferred(),h=harness(()=>pending.promise);h.password().value='old-secret';const running=h.c.startJob();
 h.c.navigate('overview');h.c.navigate('smtp');assert.equal(h.password().value,'');h.password().value='new-secret';
 if(reject)pending.reject(Error('Late failure'));else pending.resolve({id:'late',tool:'smtp',status:'completed'});
 await running;assert.equal(h.password().value,'new-secret');assert.equal(h.calls[0].sent.options.password,'old-secret');assert.equal(h.calls[0].body.options.password,'');
});
test('manually clearing the retained SMTP password sends an empty password on the next test',async()=>{
 const h=harness();h.password().value='discard-secret';await h.c.startJob();h.password().value='';h.document.emit('input',h.password());
 await h.c.startJob();assert.deepEqual(h.calls.map(call=>call.sent.options.password),['discard-secret','']);assert.equal(h.password().value,'');
});
