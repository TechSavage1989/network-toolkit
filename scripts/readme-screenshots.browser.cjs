'use strict';
// README screenshot generator. Synthetic data only: no desktop, device or production network calls.
// Every address is from a documentation range (192.0.2.0/24, 198.51.100.0/24, 10.0.0.0/8) or example.com.
// Run: NTK_APP_EXECUTABLE=<a Network Toolkit build> node scripts/readme-screenshots.browser.cjs  (writes docs/images/*.png)
// The packet capture shot imports a small synthetic PCAP into that build so its real analysis is shown;
// without NTK_APP_EXECUTABLE it is skipped. A Linux build works: go build -mod=vendor -o /tmp/ntk ./cmd/network-toolkit
// SHOTS=hero,terminal limits the run to the named images.
const fs=require('node:fs'),path=require('node:path'),assert=require('node:assert/strict');
const {chromium}=require(process.env.PLAYWRIGHT_MODULE||'playwright');
const {handleEditorActivityMetadata}=require('./editor-activity-fixture.cjs');
const root=path.resolve(__dirname,'..'),out=process.env.TOOLKIT_TEST_OUTPUT||path.join(root,'docs','images');
const only=(process.env.SHOTS||'').split(',').filter(Boolean);
const version=require(path.join(root,'package.json')).version;
let browser,stored={appUpdatePreferences:{enabled:false},profiles:[{id:'kept',name:'Branch office router',target:'192.0.2.1',note:'Edge router',tool:'ping'}],firstLaunchWelcomeSeen:true,target:'example.com'};
let syntheticJobs=[],activity={tools:{}},extraApi=()=>undefined,liveApp=null;
const errors=[];
// What's New text of the published 1.20.2 release (RELEASE_NOTES_1.20.2.txt).
const updateNotes="What's new\n- The progress bar shown while an update downloads, installs and restarts moves again. A shared style from the redesign had stopped it animating.";
const systemInfo={hostname:'EXAMPLE-PC',os:'windows',arch:'amd64',gateways:['192.0.2.1'],dnsServers:['192.0.2.53'],warnings:[],interfaces:[{index:7,name:'Ethernet',addresses:['192.0.2.60/24','fe80::5efe:c000:23c/64'],dns:['192.0.2.53'],gateway:'192.0.2.1',gateways:['192.0.2.1'],loopback:false,mac:'00:00:5E:00:53:60',mtu:1500,up:true},{index:12,name:'Wi-Fi',addresses:['10.20.40.31/24'],dns:['10.20.40.1'],gateway:'',gateways:[],loopback:false,mac:'00:00:5E:00:53:61',mtu:1500,up:true},{index:15,name:'Bluetooth Network Connection',addresses:[],dns:[],gateways:[],loopback:false,mac:'00:00:5E:00:53:62',mtu:1500,up:false}]};
const stamp=n=>new Date(Date.parse('2026-11-02T09:00:00Z')+n*1000).toISOString();
async function make(viewport){
 const context=await browser.newContext({viewport,serviceWorkers:'block'}),p=await context.newPage();
 p.setDefaultTimeout(15000);p.on('pageerror',e=>errors.push(e.message));
 await context.route('**/*',async route=>{
  if(await handleEditorActivityMetadata(route,'https://readme.toolkit.test'))return;
  const request=route.request(),url=new URL(request.url()),name=url.pathname,method=request.method();
  assert.equal(url.origin,'https://readme.toolkit.test');
  if(liveApp&&name.startsWith('/api/packet-captures')&&!['/api/packet-captures/capabilities','/api/packet-captures/interfaces'].includes(name)){
   // Packet capture import and analysis run in the real app (see startLiveApp).
   const headers={...request.headers(),'x-session-token':liveApp.token};delete headers.origin;delete headers.referer;
   return route.fulfill({response:await route.fetch({url:liveApp.base+name.slice(1)+url.search,headers})});
  }
  if(name.startsWith('/api/')){
   let data={};
   const custom=extraApi(name,method,url,request);
   if(custom!==undefined)data=custom;
   else{
    if(!['GET','PUT'].includes(method))console.log('Synthetic reply to '+method+' '+name);
    if(name==='/api/status')data={version,os:'windows',arch:'amd64',portable:true,editorWindowAvailable:true,system:systemInfo};
    if(name==='/api/state'){if(method==='PUT'){stored=request.postDataJSON();}else data=stored;}
    if(name==='/api/system')data=systemInfo;
    if(name==='/api/public-ip')data={provider:'ipify',checkedAt:stamp(0),expiresAt:stamp(600),ipv4:{status:'ok',address:'203.0.113.45'},ipv6:{status:'ok',address:'2001:db8:45::1'}};
    if(name==='/api/updates/status')data={currentVersion:version,supported:true,hasUpdate:false,busy:[],installing:false,checkedAt:stamp(0),latest:{version,notes:updateNotes}};
    if(name==='/api/jobs')data=syntheticJobs;if(name.startsWith('/api/jobs/'))data=syntheticJobs.find(job=>name==='/api/jobs/'+job.id)||{};
    if(name==='/api/activity')data=activity;
    if(name==='/api/terminals'||name==='/api/remote-browser')data={sessions:[]};
    if(name==='/api/terminal-views')data={available:false,views:[],owners:{}};
    if(name==='/api/terminal-window/requests')data={requests:[],after:0};
    if(name==='/api/credentials')data={entries:[],vault:{available:false,state:'unavailable'}};
    if(name==='/api/credential-vault')data={available:false,state:'unavailable'};
    if(name==='/api/local-tools/ip')data={adapters:[]};
    if(name==='/api/local-tools/hosts')data={available:true,editable:true,exists:true,path:'C:\\Windows\\System32\\drivers\\etc\\hosts',content:'127.0.0.1 localhost\n',revision:'synthetic',encoding:'UTF-8',newline:'CRLF',warnings:[]};
    if(name==='/api/packet-captures')data={captures:[]};
    if(name==='/api/packet-captures/interfaces')data={interfaces:[{id:'nics',name:'All network adapters',engine:'auto'}]};
    // The Windows build's capability note (cmd/network-toolkit/packet_capture_windows.go).
    if(name==='/api/packet-captures/capabilities')data={available:true,live:true,file:true,elevated:false,note:'Uses Windows Packet Monitor. Opening this tool and refreshing adapters do not request administrator approval. Start capture requests approval; driver/component availability is checked only then. Live capture supports up to 9000 bytes per packet. File mode shows packet details after Stop.',interfaces:[{id:'nics',name:'All network adapters',engine:'auto'}]};
    if(name==='/api/transfer/local')data={...localFiles,separator:'\\',roots:['C:\\'],drives:[{path:'C:\\',label:'Local Disk',kind:'local'}]};
    if(name.includes('/events'))data={events:[],sessions:[],after:0};
   }
   return route.fulfill({contentType:'application/json',body:JSON.stringify(data)});
  }
  const file=name==='/'?path.join(root,'web/index.html'):path.join(root,'web',name);
  try{let body=fs.readFileSync(fs.existsSync(file)?file:path.join(root,name));
   if(name==='/')body=body.toString().replaceAll('__SESSION_TOKEN__','synthetic-readme').replaceAll('__WINDOW_MODE__','main').replaceAll('__EDITOR_VIEW__','null').replaceAll('__TERMINAL_VIEW__','null');
   return route.fulfill({body,contentType:name==='/'?'text/html':name.endsWith('.js')?'application/javascript':name.endsWith('.css')?'text/css':name.endsWith('.svg')?'image/svg+xml':'application/octet-stream'});
  }catch{return route.fulfill({status:404,body:''});}
 });
 await p.goto('https://readme.toolkit.test/');
 await p.waitForFunction(()=>window.__networkToolkitStartupState===1&&online&&stateLoaded);
 return{p,context};
}
const settle=(p,ms=500)=>p.waitForTimeout(ms);
async function showJob(p,page,job){
 syntheticJobs=[job,...syntheticJobs.filter(j=>j.id!==job.id)];
 await p.evaluate(({page,job})=>{clearTimeout(nextPoll);polling=true;if(job.tool==='discover')state.networkRange=job.target;else state.target=job.target;state.forms[job.tool]={...(state.forms[job.tool]||{}),...job.options};navigate(page);jobs=[job,...jobs.filter(j=>j.id!==job.id)];selected[job.tool]=job.id;lastRender='';renderPage();renderCurrentResult(true);if(typeof setToolPanel==='function')setToolPanel('results');},{page,job});
 await settle(p,700);
}
const done=(tool,target,result,extra={})=>({id:'readme-'+tool,tool,target,status:'completed',started:stamp(0),finished:stamp(extra.seconds||3),eventSequence:1,options:extra.options||{},events:[],result});

// Ping and monitor samples.
const samples=Array.from({length:60},(_,i)=>({sequence:i+1,time:stamp(i),address:'192.0.2.10',success:i!==41,status:i===41?'Timed out':'Reply',rttMs:i===41?null:Math.round((11+4*Math.sin(i/5)+(i%7)*0.6)*10)/10}));
const ok=samples.filter(s=>s.success).map(s=>s.rttMs);
const ping=done('ping','example.com',{summary:'59 of 60 replies, 1.7% loss',data:{samples,statistics:{sent:60,received:59,lost:1,lossPercent:Math.round(1/60*1000)/10,minMs:Math.min(...ok),maxMs:Math.max(...ok),avgMs:Math.round(ok.reduce((a,b)=>a+b,0)/ok.length*10)/10}}},{seconds:59,options:{count:60,intervalMs:1000}});

// Trace route.
const hopRows=[['192.0.2.1','gateway.example.com',1.2],['10.20.0.1','core-1.example.net',3.8],['10.20.8.6','edge-2.example.net',6.1],[null,'',null],['198.51.100.17','transit-a.example.net',11.4],['198.51.100.42','transit-b.example.net',13.0],['203.0.113.9','lb-1.example.com',14.2],['192.0.2.10','www.example.com',14.6]];
const hops=hopRows.map(([address,hostname,rtt],i)=>({time:stamp(i),hop:i+1,address:address||'',status:address?(i===hopRows.length-1?'Reached':'TTL expired'):'Timed out',reached:i===hopRows.length-1,rttMs:rtt,samples:[0,1,2].map(n=>address?{sequence:n+1,address,success:true,status:'Reply',rttMs:Math.round((rtt+n*0.4)*10)/10}:{sequence:n+1,address:'',success:false,status:'Timed out',rttMs:null}),...(address?{hostname,hostnames:[hostname],hostnameSource:'Reverse DNS',hostnameStatus:'resolved'}:{hostnameStatus:'not-attempted'})}));
const trace=done('trace','example.com',{summary:'Reached 192.0.2.10 in 8 hops',data:{target:'example.com',address:'192.0.2.10',hops,reached:true,resolveHostnames:true,note:'Routers can rate-limit or ignore ICMP; a hop without a reply is not necessarily a fault.'}},{seconds:9,options:{maxHops:24,probes:3}});
// DNS lookup.
const records=[['A','192.0.2.10',300],['A','192.0.2.11',300],['AAAA','2001:db8::10',300],['MX','10 mail.example.com',3600,{preference:10,exchange:'mail.example.com'}],['NS','ns1.example.com',86400],['NS','ns2.example.com',86400],['TXT','v=spf1 mx -all',3600,{purpose:'SPF policy'}],['CAA','0 issue "ca.example.net"',3600],['SOA','ns1.example.com hostmaster.example.com',3600,{serial:2026110201,primary:'ns1.example.com',mailbox:'hostmaster.example.com'}]].map(([type,value,t,details],i)=>({time:stamp(i),name:'example.com',type,value,ttl:t,section:'answer',...(details?{details}:{})}));
const queries=['A','AAAA','MX','NS','TXT','CAA','SOA','CNAME','SRV'].map((type,i)=>({time:stamp(i),type,status:['CNAME','SRV'].includes(type)?'No records':'OK',durationMs:Math.round((8+i*1.7)*10)/10,server:'192.0.2.53'}));
const dns=done('dns','example.com',{summary:records.length+' record(s) from '+queries.length+' query type(s)',data:{query:'example.com',server:'192.0.2.53',records,queries,elapsedMs:41.2,note:'TTL values come from the Windows DNS client where supported or a direct DNS query.'}},{options:{recordType:'ALL'}});
// TCP ports.
const svc={22:'SSH',53:'DNS',80:'HTTP',443:'HTTPS',445:'SMB',3389:'RDP'};
const portState={22:'open',53:'closed',80:'open',443:'open',445:'timeout',3389:'closed'};
const ports=Object.keys(svc).map((port,i)=>({time:stamp(i),port:+port,service:svc[port],state:portState[port],latencyMs:portState[port]==='timeout'?2000:Math.round((3+i*0.9)*10)/10,address:'192.0.2.10:'+port,...(portState[port]==='timeout'?{error:'No response before the timeout'}:portState[port]==='closed'?{error:'Connection refused'}:{})}));
const tcp=done('tcp','192.0.2.10',{summary:'3 open, 2 refused, 1 timed out across 6 tested TCP ports',data:{target:'192.0.2.10',addresses:['192.0.2.10'],ports,openCount:3,closedCount:2,timeoutCount:1,testedCount:6,requestedCount:6,elapsedMs:2031,note:'These are TCP connection tests. Service names are conventional port labels, not detected applications.'}},{options:{ports:'22,53,80,443,445,3389'}});
// Connection monitor (TCP, ten minutes).
const mon=Array.from({length:600},(_,i)=>{const lost=i>=318&&i<=322;return{sequence:i+1,time:stamp(i),address:'192.0.2.20',success:!lost,status:lost?'Timed out':'Connected',rttMs:lost?null:Math.round((4.5+1.8*Math.sin(i/23)+(i%11)*0.25+(i>300&&i<340?6:0))*10)/10};});
const monOk=mon.filter(s=>s.success).map(s=>s.rttMs);
const monitor={...done('monitor','192.0.2.20',{summary:'595 of 600 TCP connections to port 443 succeeded',data:{target:'192.0.2.20',mode:'tcp',port:443,samples:mon,statistics:{sent:600,received:595,lost:5,lossPercent:0.8,minMs:Math.min(...monOk),maxMs:Math.max(...monOk),avgMs:Math.round(monOk.reduce((a,b)=>a+b,0)/monOk.length*10)/10,jitterMs:0.6},continuous:false}},{seconds:599,options:{mode:'tcp',port:443,count:600,intervalMs:1000}})};
// Website and mail checks.
const http=done('http','https://www.example.com',{summary:'HTTP 200, 2 request(s), 182.4 ms total',data:{target:'https://www.example.com',finalURL:'https://www.example.com/',method:'GET',statusCode:200,protocol:'HTTP/2.0',headers:{'Content-Type':'text/html; charset=utf-8','Cache-Control':'max-age=600','Server':'example-web','Strict-Transport-Security':'max-age=31536000','Content-Length':'1256'},redirects:[{url:'http://www.example.com/',statusCode:301,location:'https://www.example.com/',elapsedMs:41.3},{url:'https://www.example.com/',statusCode:200,elapsedMs:141.1}],tls:{version:'TLS 1.3',cipher:'TLS_AES_128_GCM_SHA256',serverName:'www.example.com',verified:true},contentLength:1256,bodyBytesRead:1256,bodyTruncated:false,elapsedMs:182.4,proxy:'Direct connection; system and environment proxies are not used'}});
const cert=(subject,issuer,from,to,dns)=>({subject,issuer,notBefore:from,notAfter:to,dnsNames:dns,serialNumber:'4F:2A:91:0C:7E:11',signatureAlgorithm:'ECDSA-SHA256',publicKey:'ECDSA P-256'});
const tls=done('tls','www.example.com',{summary:'TLS 1.3, certificate valid for 62 more days',data:{version:'TLS 1.3',cipher:'TLS_AES_128_GCM_SHA256',serverName:'www.example.com',alpn:'h2',verified:true,verificationError:'',verificationEnforced:true,certificates:[cert('CN=www.example.com','CN=Example Issuing CA 2','2026-09-03T00:00:00Z','2027-01-03T23:59:59Z',['www.example.com','example.com']),cert('CN=Example Issuing CA 2','CN=Example Root CA','2024-01-01T00:00:00Z','2030-12-31T23:59:59Z',[])],warnings:[]}},{options:{port:443}});
const smtpSteps=[['TCP connection','passed','Connected to 192.0.2.25:587'],['SMTP banner','passed','SMTP greeting accepted'],['EHLO','passed','Server accepted EHLO'],['Capabilities','passed','STARTTLS, AUTH PLAIN LOGIN, SIZE 52428800, 8BITMIME'],['STARTTLS','passed','TLS established and certificate verified'],['Authentication','passed','Credentials accepted'],['Test message','passed','Message accepted for delivery to noc@example.com']];
const smtp=done('smtp','mail.example.com',{summary:'7 steps passed; test message sent',data:{host:'mail.example.com',port:587,securityRequested:'starttls',steps:smtpSteps.map(([stage,status,detail],i)=>({time:stamp(i),stage,status,detail})),messageSent:true}},{options:{port:587,security:'starttls'}});
// Network discovery.
const devices=[['192.0.2.1','gateway.example.com','00:00:5E:00:53:01','Example Networks','Router',[22,80,443],0.8],['192.0.2.5','switch-core.example.com','00:00:5E:00:53:05','Example Networks','Switch',[22,443],1.1],['192.0.2.10','printer-2f.example.com','00:00:5E:00:53:10','Example Imaging','Printer',[80,443,9100],2.1],['192.0.2.12','nas-01.example.com','00:00:5E:00:53:12','Example Storage','NAS',[22,80,443,445],1.4],['192.0.2.20','web-01.example.com','00:00:5E:00:53:20','Example Systems','',[22,80,443],0.9],['192.0.2.21','web-02.example.com','00:00:5E:00:53:21','Example Systems','',[22,80,443],1.0],['192.0.2.33','camera-lobby.example.com','00:00:5E:00:53:33','Example Vision','IP camera',[80,554],3.6],['192.0.2.40','ap-floor2.example.com','00:00:5E:00:53:40','Example Wireless','Access point',[22,443],2.7],['192.0.2.60','EXAMPLE-PC','00:00:5E:00:53:60','Example Computers','',[445,3389],0.1],['192.0.2.77','','00:00:5E:00:53:77','','',[],5.3]];
const hosts=[];for(let n=1;n<=126;n++){const address='192.0.2.'+n;const row=devices.find(x=>x[0]===address);if(row){const [a,hn,mac,vendor,model,open,lat]=row;hosts.push({time:stamp(n),address:a,status:'Responded',completed:true,responded:true,evidence:['ICMP echo reply',...open.map(p=>'TCP '+p+' open')],latencyMs:lat,openPorts:open,mac,vendor,vendorSource:vendor?'IEEE registry':'',macKind:'universal',...(hn?{hostname:hn,hostnames:[hn],hostnameSource:'Reverse DNS',hostnameStatus:'resolved'}:{hostnameStatus:'not-found'}),...(model?{manufacturer:vendor,modelName:model,modelSource:'UPnP description'}:{})});}else hosts.push({time:stamp(n),address,status:'No response',completed:true,responded:false,evidence:[],latencyMs:null,openPorts:[],hostnameStatus:'not-attempted'});}
const discover=done('discover','192.0.2.0/25',{summary:'10 of 126 addresses responded',data:{scope:'subnet',target:'192.0.2.0/25',hosts,progress:{total:126,started:126,completed:126,responded:10,notStarted:0}}},{seconds:28,options:{ports:'22,80,443,445,554,3389,9100'}});

// Terminal: two SSH sessions to fictional switches, output delivered through the normal terminal event feed.
const b64=text=>Buffer.from(text.replace(/\n/g,'\r\n')).toString('base64');
const termOut=`Last login: Mon Nov  2 08:52:11 2026 from 192.0.2.60

switch-core# show interface brief
Interface    Admin  Oper  Speed   VLAN  Description
-----------  -----  ----  ------  ----  ----------------------
1/1/1        up     up    1G      10    uplink-gateway
1/1/2        up     up    1G      10    nas-01
1/1/3        up     down  auto    20    spare
1/1/4        up     up    1G      30    printer-2f
1/1/5        up     up    1G      40    ap-floor2
1/1/48       up     up    10G     --    trunk-to-switch-2
switch-core# show ip route
Codes: C - connected, S - static

S    0.0.0.0/0        via 192.0.2.1, vlan10
C    192.0.2.0/24     is directly connected, vlan10
C    10.20.30.0/24    is directly connected, vlan30
C    10.20.40.0/24    is directly connected, vlan40
switch-core# ping 192.0.2.10 count 3
PING 192.0.2.10: 56 data bytes
64 bytes from 192.0.2.10: icmp_seq=1 ttl=64 time=0.412 ms
64 bytes from 192.0.2.10: icmp_seq=2 ttl=64 time=0.388 ms
64 bytes from 192.0.2.10: icmp_seq=3 ttl=64 time=0.401 ms
--- 192.0.2.10 ping statistics ---
3 packets transmitted, 3 received, 0% packet loss
switch-core# `;
const termSessions=[{id:'readme-term-1',type:'ssh',host:'192.0.2.5',port:22,username:'operator',label:'switch-core',state:'connected',cols:120,rows:32,profileId:'tp-core'},{id:'readme-term-2',type:'ssh',host:'192.0.2.6',port:22,username:'operator',label:'switch-2',state:'connected',cols:120,rows:32,profileId:'tp-2'},{id:'readme-term-3',type:'serial',serialPort:'COM3',label:'Console COM3',state:'connected',cols:120,rows:32}];
const termProfiles=[{id:'tp-core',name:'switch-core',type:'ssh',host:'192.0.2.5',port:22,username:'operator',authMethod:'key'},{id:'tp-2',name:'switch-2',type:'ssh',host:'192.0.2.6',port:22,username:'operator',authMethod:'key'},{id:'tp-fw',name:'firewall-edge',type:'ssh',host:'192.0.2.2',port:22,username:'admin',authMethod:'password'},{id:'tp-ap',name:'ap-floor2',type:'telnet',host:'192.0.2.40',port:23,username:''},{id:'tp-con',name:'Console COM3',type:'serial',serialPort:'COM3',baudRate:9600}];
const remoteFiles=folder=>({path:folder,parent:folder==='/'?'':folder.replace(/\/[^/]+$/,'')||'/',entries:folder==='/backups'?[{name:'2026-10',path:'/backups/2026-10',directory:true,modified:'2026-10-31T23:10:00Z'},{name:'2026-11',path:'/backups/2026-11',directory:true,modified:'2026-11-02T06:00:00Z'},...['switch-core','switch-2','firewall-edge','ap-floor2','gateway'].flatMap((n,i)=>[{name:n+'-2026-11-02.cfg',path:'/backups/'+n+'-2026-11-02.cfg',directory:false,size:8200+i*1733,modified:'2026-11-02T06:0'+i+':00Z'},{name:n+'-2026-11-01.cfg',path:'/backups/'+n+'-2026-11-01.cfg',directory:false,size:8150+i*1730,modified:'2026-11-01T06:0'+i+':00Z'}]),{name:'firmware-notes.txt',path:'/backups/firmware-notes.txt',directory:false,size:1204,modified:'2026-10-28T14:12:00Z'}]:[]});
const localFiles={path:'C:\\Users\\operator\\Documents\\Configs',parent:'C:\\Users\\operator\\Documents',entries:[{name:'archive',path:'C:\\Users\\operator\\Documents\\Configs\\archive',directory:true,modified:'2026-10-20T10:00:00Z'},...['switch-core','switch-2','firewall-edge'].map((n,i)=>({name:n+'-2026-11-01.cfg',path:'C:\\Users\\operator\\Documents\\Configs\\'+n+'-2026-11-01.cfg',directory:false,size:8150+i*1730,modified:'2026-11-01T06:0'+i+':00Z'})),{name:'vlan-plan.xlsx',path:'C:\\Users\\operator\\Documents\\Configs\\vlan-plan.xlsx',directory:false,size:23110,modified:'2026-10-12T09:30:00Z'}]};

const shots={},light=new Set(['website-http','file-transfer','network-utilities']);
shots['hero']=async p=>{await p.evaluate(()=>setTheme('dark'));await showJob(p,'ping',ping);};
for(const [name,page,job] of [['trace-route','trace',trace],['dns-lookup','dns',dns],['tcp-ports','tcp',tcp],['connection-monitor','monitor',monitor],['website-http','http',http],['website-tls','tls',tls],['smtp-test','smtp',smtp]])
 shots[name]=async p=>{await p.evaluate(()=>setTheme('dark'));await showJob(p,page,job);};

// A small synthetic capture: ARP, DNS (one NXDOMAIN), HTTP, TLS, ping with a lost reply,
// a refused and an unanswered TCP connection, and DHCP. Documentation addresses only.
const examplePcap='H4sIAAAAAAACA92Xe0xTVxzHf+f2UkmxPqAJPma5FonyKq/CDBRFCsjrbhWKcwQXu65iK9BKEWZwiG7qNIaguESSqUziMuMGY84ZIBqjcaCFxD9Mncb4SCSDxOBCAj6CsHPuvUBhd1tL/Mub3Obc8/ic7/d3fic95871X76jgIaJZ3wcAJGCrt+6YCVAGPDvOPcAfAL5W3ylgHxBSpOO5PsqUFp+NC4hMpL5YGok34f7RZMjKf6b6z/FwCPtCxHk4DE5EyOFdl9IB0gmM6YsHEkmvfEbf10P8RBakolIA+I1SKqqquaYPzeW2kvMEpOtlDRwovQpCKy4g3WmKIIuEtBKguXEEHhmz2sEe2s5wr+hxecDFAE05ijIzE0SyhtTG4gphxUhN1NGu0XUVG0s5Z2pDZD5MhghN1NiaPH5ppm6PUwBi8ezYqbWCjMnTZrKw6bC/QcQNWXKZ6ulxOwQtfXzCsl0uLstd/iErTwIrzMjav+XHsAfN0ogAXdJEFO+ioNLR2oE5YquVNDjKPWTvnqKgZRAfnfUnptBcZfoTlFwEvWYAwFfY9KA3p+BiBqesu2nWWgZwKRD+gWYsoenrL4ogWO4eEyM8pVAUYlSFjFwrglgfbqBiXJUGCt2OphMg0EfFaOOkcsybY6KRAYnuFqIoxrHUS4rcJjLI9cVm8two9AQaSqx4G88LFouW2cyme24LSwqTC6Ty4jCTRIajmBtR8SiVSMoXDUjWocA6HSiMO1PgAlVTGx0NPNhjlyms5VV4CkjDbvs5kTGaLeXWEzGCoutLMrqsJVNteeay4ortiUyMfFETLWKd6lKVNm2q77g1iCM9noNaNwU8Ei/kIHwHmENYmnv8+ERJmVwFCdPYRJmoSUDkx6TfJig6M97RdGhTvcMX3Oep/zV4Y0j1Nmlc8/wcH+e8seVWWiZzPDwAJ7yw+80PMfF52KUJzxl+IYoBeeP4QVAoEQCv55lby0Kie5cVhoQ8dAVXK2+p1xbHuRcvCMw7bgrS3uySLPlSgyjtIzdbxrsb2/cU7Q7tPFNa19DiNZlaGVf31WsbGq5MDYQHHV4aHnQqadHb7T5Pbu4MWlUaWtIrXF2j4UW9PXWZTyou3PpWiUbVshWVklfbj5QT2W7LjenLB3odvbcjB/pDTKqrE/yV7cs0QAb1yFfYb2au7etY2hN0qvB4Renw7M0rQeH8/ws7TnyE+r9nQXEfrfCZ7p996Vwt+++FGT7nCH26TrBfkHnfvUJeU67xS9v+GCrJiv89IvhwVdJa4Y62vbmXrWukHfEsaBZ0rI6/4lVZQzqHYm/2ePsHlia0nzZlU3VH9j8UlpVyRaGsZXXLt2pe5BR19tXEDrW7axJbbApR5M2Xnzm13bj6NNTQcuHDkcFD4xdaGlaqbj7mm01uLQhDX2tbxpDdxftaWzvH2y6P2ZRMjFXtmiKTmqzXMfTAncsdgaVr1XeU1cHux5GBJQu64wOWXSLPcvlUJKP1zlEnwFQpJK9Fdok5JDOx+t8VuBdSjdzlNM8pX79LLQ0Y5KO5PME5bbBF7JxMVuMouUoaGSfQMEK2ULy72X81PSZeWvxNot1e0lpmc2+o9xRsbOyarKakH8snEF2d+lORvy5rcBj8rNiP+80f4zrKE/I3aV+3mn2mOxbPc87zZtwneT/yIiQMyjvyB/hOtoT8tMcyrtoeEzWzPfgtHFA0Dy3K1eeDMJpjfwvvD8ff2DKewEenHwIZS4hyZO7coXLBaVXMJAwigtilP/WwqLLE+dBoiUV70vJW6HQb40y+v0ciMZ9yTvtvoQpSMUfWnc5QbhNQRro0LxvTAhJcY2/BkQfPrrvymPal2+KR5JYconQzpWlb1rH6nPTI/W6cRK/b3+bA1ocJS36Z14hNR+/4drJvNdBGloSGkfNiN/UbfRdjZ9PAokfisMX8A21iCa5JOFqpOQ3fvxvi+OS+NQPAAA=';
async function startLiveApp(){
 const {spawn}=require('node:child_process'),os=require('node:os'),data=fs.mkdtempSync(path.join(os.tmpdir(),'ntk-readme-'));
 const app=spawn(process.env.NTK_APP_EXECUTABLE,['-no-browser','-port','0','-data-dir',data],{stdio:['ignore','pipe','pipe']});let log='';app.stdout.on('data',b=>log+=b);app.stderr.on('data',b=>log+=b);
 for(let i=0;i<150;i++){const base=log.match(/http:\/\/127\.0\.0\.1:\d+\//)?.[0];if(base){try{const html=await (await fetch(base)).text();return {app,data,base,token:html.match(/'([a-f0-9]{64})'/)[1]};}catch{}}await new Promise(r=>setTimeout(r,100));}
 app.kill();throw new Error('App did not start: '+log);
}
async function openCapture(p){
 const file=path.join(require('node:os').tmpdir(),'example-office.pcap');fs.writeFileSync(file,require('node:zlib').gunzipSync(Buffer.from(examplePcap,'base64')));
 await p.evaluate(()=>{setTheme('dark');navigate('capture');});await settle(p,500);
 await p.locator('#packetCaptureFile').setInputFiles(file);
 await p.waitForFunction(()=>NetworkToolkitPacketCapture.model.report?.packetCount>0);await settle(p,1200);
}
shots['packet-capture']=async p=>{await openCapture(p);await p.fill('#packetCheckTarget','192.0.2.12:3389');await p.click('[data-packet-action="check-target"]');await p.waitForSelector('.packet-target-step');await settle(p,600);};
shots['network-discovery']=async p=>{await p.evaluate(()=>setTheme('dark'));await showJob(p,'discover',discover);await p.locator('#toolResult').getByText('192.0.2.10',{exact:true}).first().click();await settle(p,300);await p.mouse.move(988,598);await p.mouse.down();await p.mouse.move(988,700,{steps:5});await p.mouse.move(988,742,{steps:5});await p.mouse.up();await p.mouse.move(5,895);await settle(p,900);};
shots['hosts-file-review']=async p=>{
 extraApi=name=>{if(name==='/api/local-tools/hosts')return {available:true,editable:true,exists:true,path:'C:\\Windows\\System32\\drivers\\etc\\hosts',content:hostsText,revision:'readme',encoding:'UTF-8',newline:'CRLF',warnings:[]};};
 await p.evaluate(()=>{setTheme('dark');navigate('hosts');});await settle(p,800);
 await p.locator('tbody tr').filter({hasText:'staging.example.com'}).locator('input[type=checkbox]').nth(1).click();
 await p.getByRole('button',{name:'Review and save'}).click();await settle(p,900);
};
shots['terminal']=async p=>{
 let delivered=false;
 extraApi=(name,method)=>{
  if(name==='/api/terminals')return {sessions:termSessions};
  if(name==='/api/terminals/events'){if(delivered)return {sessions:termSessions,events:[],next:2};delivered=true;return {sessions:termSessions,events:[{seq:1,sessionId:'readme-term-1',type:'output',data:b64(termOut)},{seq:2,sessionId:'readme-term-2',type:'output',data:b64('switch-2# ')}],next:2};}
  if(method==='POST'&&name.startsWith('/api/terminals/'))return {ok:true};
 };
 await p.evaluate(({termProfiles})=>{setTheme('dark');state.terminalProfiles=termProfiles;state.terminalSidebarTab='saved';navigate('terminal');},{termProfiles});
 await p.waitForFunction(()=>terminalSessions.get('readme-term-1')?.xterm);await settle(p,1500);
};
shots['file-transfer']=async p=>{
 extraApi=(name,method,url,request)=>{if(name==='/api/remote-browser'&&method==='POST'){const body=request.postDataJSON()||{};return remoteFiles(body.path||'/backups');}};
 await p.evaluate(({localFiles,remote})=>{setTheme('dark');state.fileTransferProtocol='sftp';state.transferModes={sftp:'client',ftp:'client',tftp:'client'};navigate('sftp');
  const model=NetworkToolkitRemoteBrowser.get('ssh'),config=NetworkToolkitRemoteBrowser.configurations.ssh;
  Object.assign(config,{host:'192.0.2.12',port:22,username:'operator',fingerprint:'SHA256:qX8nV1rE3bT2yH6cK9mP4sW7aZ0dF5gJ1lN3oQ8uR2'});
  Object.assign(model,{connected:true,sessionId:'',connectionId:'readme-connection',profileId:'',saved:true});
  Object.assign(model.local,{...localFiles,home:localFiles.path,initialized:true,loading:false});
  Object.assign(model.remote,{...remote,home:'/backups',initialized:true,loading:false});
  model.activity=[{time:'06:00:12',text:'Connected to 192.0.2.12:22 as operator',kind:'info'},{time:'06:01:40',text:'Downloaded switch-core-2026-11-02.cfg (9.6 KB)',kind:'success'},{time:'06:01:41',text:'Downloaded switch-2-2026-11-02.cfg (11.3 KB)',kind:'success'}];
  renderPage();},{localFiles,remote:remoteFiles('/backups')});
 await settle(p,800);await p.mouse.move(988,594);await p.mouse.down();await p.mouse.move(988,700,{steps:5});await p.mouse.move(988,770,{steps:5});await p.mouse.up();await p.mouse.move(5,895);await settle(p,500);
};
const adapter={id:'{readme-ethernet}',index:7,name:'Ethernet',description:'Example Gigabit Network Adapter',type:'Ethernet',status:'Up',mac:'00:00:5E:00:53:60',dhcp:false,editable:true,addressEditable:true,addresses:[{address:'192.0.2.60',prefixLength:24,origin:'manual'}],gateways:['192.0.2.1'],dnsServers:['192.0.2.53','192.0.2.54'],dnsAutomatic:false,revision:'readme',adminStatus:'enabled',adminEditable:true,speed:'1 Gbps'};
const wifiAdapter={...adapter,id:'{readme-wifi}',index:12,name:'Wi-Fi',description:'Example Wireless Adapter',type:'Wireless',mac:'00:00:5E:00:53:61',dhcp:true,addresses:[{address:'10.20.40.31',prefixLength:24,origin:'dhcp'}],gateways:['10.20.40.1'],dnsServers:['10.20.40.1'],dnsAutomatic:true,speed:'866 Mbps'};
shots['local-network']=async p=>{
 extraApi=name=>{if(name==='/api/local-tools/ip')return {available:true,adapters:[adapter,wifiAdapter]};};
 await p.evaluate(()=>{setTheme('dark');state.localTab='ipconfig';navigate('local');});
 await p.locator('#ipConfiguration .ip-configuration-grid').first().waitFor();await settle(p,900);
};
const hostsText='# Hosts file managed by the IT team\r\n127.0.0.1       localhost\r\n::1             localhost\r\n\r\n# Lab equipment\r\n192.0.2.5       switch-core.lab.example.com switch-core\r\n192.0.2.6       switch-2.lab.example.com switch-2\r\n192.0.2.12      nas-01.lab.example.com nas-01\r\n10.20.30.15     printer-2f.lab.example.com\r\n\r\n# Staging site\r\n198.51.100.20   staging.example.com\r\n';
shots['network-utilities']=async p=>{
 await p.evaluate(()=>{setTheme('dark');navigate('subnet');});await settle(p,400);
await p.locator('[placeholder="192.168.10.25/26"]').fill('192.0.2.77/26');await p.locator('[placeholder="192.168.10.50"]').fill('192.0.2.100');
 await p.getByRole('button',{name:'Calculate'}).click();await settle(p,600);
};
const cfgOld=`hostname switch-core
!
vlan 10
 name servers
vlan 20
 name spare
vlan 30
 name printers
!
interface 1/1/1
 description uplink-gateway
 switchport access vlan 10
!
interface 1/1/3
 description spare
 switchport access vlan 20
 shutdown
!
interface 1/1/4
 description printer-2f
 switchport access vlan 30
!
ip route 0.0.0.0/0 192.0.2.1
ntp server 192.0.2.123
logging host 192.0.2.50
!
end
`;
const cfgNew=cfgOld.replace('vlan 30\n name printers\n','vlan 30\n name printers\nvlan 40\n name wireless\n').replace('interface 1/1/4\n description printer-2f\n switchport access vlan 30\n!\n','interface 1/1/4\n description printer-2f\n switchport access vlan 30\n!\ninterface 1/1/5\n description ap-floor2\n switchport access vlan 40\n!\n').replace('ntp server 192.0.2.123','ntp server 192.0.2.123\nntp server 192.0.2.124').replace('logging host 192.0.2.50','logging host 192.0.2.51');
shots['text-editor-compare']=async p=>{
 const owned={};
 extraApi=(name,method,url,request)=>{
  if(name==='/api/editor-views'||name==='/api/editor-views/current')return {available:true,ownerInstanceId:'readme-owner',self:'',views:[],incoming:[],outgoing:[],owned,recovery:[]};
  if(name==='/api/editor-views/current/state'){const body=request.postDataJSON()||{};for(const d of body.batch?.documents||[])owned[d.uid]=1;return {owned};}
 };
 await p.evaluate(()=>{setTheme('dark');navigate('editor');});await settle(p,400);
 await p.evaluate(async({cfgOld,cfgNew})=>{const docs=[['switch-core-2026-11-01.cfg',cfgOld],['switch-core-2026-11-02.cfg',cfgNew]];for(const [file,text] of docs)await NetworkToolkitFileEditor.open({id:file,path:'C:\\Users\\operator\\Documents\\Configs\\'+file,source:'Local computer',binding:{kind:'local'},create:true,initialContent:text,read:async()=>({content:text}),write:async()=>{throw Error('read-only screenshot');}});},{cfgOld,cfgNew});
 await p.locator('[data-ed="compare"]').click();
 await p.selectOption('[data-compare-select="left"]','switch-core-2026-11-01.cfg');await p.selectOption('[data-compare-select="right"]','switch-core-2026-11-02.cfg');
 await settle(p,1200);
};
shots['settings-updates']=async p=>{await p.evaluate(()=>setTheme('dark'));await p.locator('#settingsButton').click();await settle(p,500);await p.locator('[data-settings-section="updates"]').click();await settle(p,700);};

(async()=>{
 fs.mkdirSync(out,{recursive:true});browser=await chromium.launch({headless:true,args:['--no-sandbox']});
 for(const [name,fn] of Object.entries(shots)){
  if(only.length&&!only.includes(name))continue;
  if(name.startsWith('packet-')&&!liveApp){if(!process.env.NTK_APP_EXECUTABLE){console.log('Skipped '+name+': set NTK_APP_EXECUTABLE to a Network Toolkit build (it analyses the capture)');continue;}liveApp=await startLiveApp();}
  syntheticJobs=[];activity={tools:{}};extraApi=()=>undefined;
  const {p,context}=await make({width:1440,height:900});
  await fn(p);
  if(light.has(name)){await p.evaluate(()=>setTheme('light'));await settle(p,400);}
  await p.screenshot({path:path.join(out,name+'.png')});
  console.log('Wrote '+name);
  await context.close();
 }
 assert.deepEqual(errors,[]);
})().catch(error=>{console.error(error);process.exitCode=1;}).finally(async()=>{if(browser)await browser.close();if(liveApp){liveApp.app.kill();fs.rmSync(liveApp.data,{recursive:true,force:true});}});
