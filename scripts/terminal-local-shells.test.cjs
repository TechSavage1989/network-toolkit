'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8');
function harness(available=true){const c={status:{localShellAvailable:available},DETACHED_TERMINAL:false,DETACHED_WORKSPACE:false,esc:String,icon:()=>''};vm.createContext(c);vm.runInContext(html.match(/^const TERMINAL_SAFE_FIELDS=.*$/m)[0],c);for(const name of ['safeTerminalConfig','terminalConnectPayload','validateTerminalConfig','terminalProtocolLabel','terminalConnectionButtonsHTML']){const line=html.split('\n').find(s=>s.startsWith('function '+name+'('));assert(line,name);vm.runInContext(line,c);}return c;}
for(const type of ['cmd','powershell'])test(type+' local session shares terminal routes without remote settings or credentials',()=>{
 const h=harness(),config=h.safeTerminalConfig({type,host:'remote.test',port:2222,username:'admin',credentialId:'remote-vault',serialPort:'COM4',password:'secret',privateKey:'secret',localEcho:true});
 assert.equal(config.type,type);assert.equal(config.host,'');assert.equal(config.port,0);assert.equal(config.username,'');assert.equal(config.serialPort,'');assert(!config.credentialId);assert.equal(config.lineEnding,'cr');assert.equal(config.localEcho,false);
 assert.equal(h.validateTerminalConfig(config),config);
 const body=JSON.parse(JSON.stringify(h.terminalConnectPayload(config,{password:'secret',privateKey:'secret'})));
 assert.deepEqual(body,{type,cols:80,rows:24,terminal:'xterm-256color',timeoutMs:10000,workingDirectory:'',runAsAdministrator:false});
 assert.match(h.terminalProtocolLabel(config),/\(local\)/);assert.match(h.terminalConnectionButtonsHTML(0),new RegExp('data-terminal-new-type="'+type+'"'));
 const unsupported=harness(false);assert.doesNotMatch(unsupported.terminalConnectionButtonsHTML(0),new RegExp('data-terminal-new-type="'+type+'"'));assert.throws(()=>unsupported.validateTerminalConfig(config),/require Windows/);
});

test('local directory is literal nonsecret data, admin defaults off and remote types ignore it',()=>{
 const h=harness(),directory='C:\\Lab & tools\\日本語';
 assert.equal(h.safeTerminalConfig({type:'cmd'}).runAsAdministrator,false);
 const config=h.safeTerminalConfig({type:'powershell',workingDirectory:directory,runAsAdministrator:true});
 const body=JSON.parse(JSON.stringify(h.terminalConnectPayload(config)));
 assert.equal(body.workingDirectory,directory);assert.equal(body.runAsAdministrator,true);
 assert.equal(h.safeTerminalConfig({type:'cmd',runAsAdministrator:'true'}).runAsAdministrator,false);
 const remote=h.terminalConnectPayload({type:'ssh',host:'example.test',workingDirectory:directory,runAsAdministrator:true});
 assert(!('workingDirectory' in remote));assert(!('runAsAdministrator' in remote));
});

test('retained local startup dialog prevents background initial-pane assignment; closing restores normal initialization',()=>{
 const context={terminalInitialized:true,terminalAssetsReady:()=>true,DETACHED_WORKSPACE:false,terminalSessions:new Map(),terminalInitialPanesFilled:false,terminalConnectBusy:false,terminalPanes:['','','',''],terminalPaneFocus:0,terminalActive:'',terminalFeedAfter:0,terminalPaneCount:()=>1,renderTerminalChrome(){},terminalTabExists:id=>context.terminalSessions.has(id),terminalPaneContext:0,terminalDraftTabs:new Map(),terminalPaneMaximized:-1,form:{},$:selector=>selector==='#terminalConnectForm'?context.form:null};
 context.upsertTerminalSession=info=>context.terminalSessions.set(info.id,{info});
 vm.createContext(context);
 const start=html.indexOf('function processTerminalBatch('),end=html.indexOf('\nfunction startTerminalFeed(',start);assert(start>=0&&end>start);vm.runInContext(html.slice(start,end),context);
 vm.runInContext(html.split('\n').find(s=>s.startsWith('function placeConnectedTerminal(')),context);
 const destination={pane:0,prior:'',draftId:'',context:0,panes:['','','','']};
 context.processTerminalBatch({sessions:[{id:'cancelled',state:'error'}],events:[],next:1});
 assert.equal(context.terminalPanes[0],'');assert.equal(context.terminalInitialPanesFilled,false);
 context.processTerminalBatch({sessions:[{id:'cancelled',state:'error'},{id:'retry',state:'connected'}],events:[],next:2});
 assert.equal(context.placeConnectedTerminal(context.terminalSessions.get('retry'),destination),0);assert.equal(context.terminalPanes[0],'retry');assert.equal(context.terminalActive,'retry');
 // A genuine explicit pane change still wins; the retry cannot steal it.
 context.terminalPanes[0]='cancelled';assert.equal(context.placeConnectedTerminal(context.terminalSessions.get('retry'),destination),-1);assert.equal(context.terminalPanes[0],'cancelled');
 context.form=null;context.terminalInitialPanesFilled=false;context.terminalPanes=['','','',''];context.processTerminalBatch({sessions:[],events:[],next:3});assert.equal(context.terminalInitialPanesFilled,true);assert.equal(context.terminalPanes[0],'cancelled');
});
