'use strict';
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const assets=path.resolve(__dirname,'../web/assets'),read=name=>fs.readFileSync(path.join(assets,name),'utf8');
const c={};vm.createContext(c);for(const name of ['editor-vendor/languages.js','network-config-language.js','editor-languages.js'])vm.runInContext(read(name),c,{filename:name});
const api=c.NTKNetworkConfig,language=c.NTKEditorLanguages;
const unpaint=html=>html.replace(/<span class="nt-ed-[a-z]+">|<\/span>/g,'').replace(/&quot;/g,'"').replace(/&gt;/g,'>').replace(/&lt;/g,'<').replace(/&amp;/g,'&');
const samples={
 'network-aruba-cx':'!Version AOS-CX 10.17.0001\nhostname LAB-SW\ninterface 1/1/3\n    no routing\n    vlan access 20\n',
 'network-aruba-switch':'; LAB Configuration Editor; Created on release #WC.16.11\nhostname LAB-SW\nvlan 20\n name "Test VLAN"\n untagged 1-4\n exit\n',
 'network-comware':'#\n version 7.1.070, Release 2702\n sysname LAB-SW\n#\ninterface GigabitEthernet1/0/1\n port link-type access\n undo shutdown\n#\nreturn\n',
 'network-fortios':'#config-version=FGT60F-7.4.0-FW-build0000\nconfig system interface\n edit "port1"\n  set alias "Test interface"\n  set ip 192.0.2.1 255.255.255.0\n next\nend\n',
 'network-sonicos':'firmware-version "SonicOS 7.1"\naddress-object ipv4 "Test Host"\n host 192.0.2.4\n exit\nnat-policy\n exit\nchecksum 00000000\nend\n',
 'network-panos-set':'set deviceconfig system hostname LAB-FW\nset network interface ethernet ethernet1/1 layer3 ip 192.0.2.1/24\nset rulebase security rules Test action allow\n',
 'network-panos-xml':'<?xml version="1.0"?>\n<config version="11.1"><devices><entry name="localhost.localdomain"><deviceconfig><system><hostname>LAB-FW</hostname></system></deviceconfig><network/><vsys/></entry></devices></config>',
 'network-cisco-ios':'version 17.12\nhostname LAB-SW\nservice timestamps log datetime msec\ninterface GigabitEthernet1/0/1\n switchport mode access\n switchport access vlan 20\n!\nline vty 0 4\n',
 'network-cisco-nxos':'version 10.4(1) Bios:version 1.0\nboot nxos bootflash:/nxos.10.4.bin\nfeature bgp\ninterface Ethernet1/1\n no shutdown\n',
 'network-cisco-asa':'ASA Version 9.20(1)\nhostname LAB-FW\ninterface GigabitEthernet0/0\n nameif inside\n security-level 100\n ip address 192.0.2.1 255.255.255.0\n'
};
for(const [id,text] of Object.entries(samples))test('detect and paint synthetic '+id,()=>{assert.equal(api.detect(text),id);assert.equal(language.detect('sample.cfg',text),id);assert.equal(language.valid(id),id);assert.equal(language.formatKind(id),'');const html=api.highlight(text,id);assert.match(html,/nt-ed-(?:keyword|comment)/);assert.equal(unpaint(html),text);});
test('ambiguous generic switch configs gain neutral syntax, not a guessed vendor',()=>{
 const text='config-file-header\nhostname LAB\ninterface gi1\n switchport mode access\nvlan 20\n';assert.equal(api.detect(text),'network');assert.equal(language.detect('sample.txt',text),'network');
 assert.equal(api.detect(samples['network-aruba-cx']+samples['network-fortios']),'network');
});
test('ordinary words, one command, extension alone and quoted vendor names do not identify a vendor',()=>{
 for(const text of ['', 'interface', 'vlan 20', 'set deviceconfig', 'the interface and vlan are examples', 'NX-OS is mentioned in this ordinary note', '# documentation mentions FortiOS and SonicOS', 'description "ASA Version 9.0"', 'config system interface\n set alias "SonicOS firmware-version"\nend'])assert.equal(api.detect(text),'',text);
 assert.equal(language.detect('sample.fortios',''),'network');assert.equal(language.detect('sample.cfg','ordinary prose'),'ini');assert.equal(language.detect('sample.txt','ordinary prose'),'text');assert.equal(language.detect('source.js',samples['network-aruba-cx']),'javascript');
});
test('independent command groups recognize headerless CX and FortiOS',()=>{
 assert.equal(api.detect('interface 1/1/1\n no routing\n vlan access 20\n'),'network-aruba-cx');assert.equal(api.detect('config system interface\n edit "port1"\n set ip 192.0.2.1 255.255.255.0\n next\nend\n'),'network-fortios');assert.equal(api.detect('set name "deviceconfig"\nset address "rulebase"\n'),'');
});
test('multiline values, comments and banner bodies cannot supply evidence or command colors',()=>{
 const banner='banner motd ^C\n!Version AOS-CX 10.17\ninterface 1/1/1\n no routing\n vlan access 20\n^C\n';assert.equal(api.detect(banner),'');const html=api.highlight(banner,'network');assert.equal((html.match(/nt-ed-keyword/g)||[]).length,1);assert.equal(unpaint(html),banner);
 const quote='set comments "text\nASA Version 9.20\ninterface 1/1/1\nend"\n';assert.equal(api.detect(quote),'');const painted=api.highlight(quote,'network-fortios');assert.equal((painted.match(/nt-ed-keyword/g)||[]).length,1);assert.equal(unpaint(painted),quote);assert.equal(api.detect('! interface 1/1/1\n! no routing\n! vlan access 20\n'),'');
});
test('comments, descriptions and sensitive tails never color embedded commands or numeric ciphertext',()=>{
 const source='! set config 123\ndescription interface vlan 123\nset password ENC 123456\nusername synthetic password cipher 123456\nset alias "interface set end 123"\n';const html=api.highlight(source,'network');assert.equal(unpaint(html),source);assert(!html.includes('nt-ed-number'));assert.equal((html.match(/nt-ed-keyword/g)||[]).length,4);assert.match(html,/nt-ed-string[^>]*> ENC 123456/);
});
test('XML requires vendor structure, ignores comments/CDATA and never resolves entities',()=>{
 assert.equal(api.detect('<config><devices/><deviceconfig/><network/></config>'),'network-panos-xml');assert.equal(api.detect('<config><item>normal</item><!-- <devices/><deviceconfig/><network/> --></config>'),'');assert.equal(api.detect('<config><![CDATA[<devices/><deviceconfig/><network/>]]></config>'),'');
 const text='<!DOCTYPE x [<!ENTITY remote SYSTEM "https://invalid.example/private">]><config>&remote;<x a="<script>">value</x></config>';const html=api.highlight(text,'network-panos-xml');assert.equal(unpaint(html),text);assert(!html.includes('<script>'));assert.equal(c.fetch,undefined);
});
test('paint preserves BOM, CRLF, CR, tabs, Unicode, controls and hostile HTML exactly',()=>{
 const text='\uFEFF!Version AOS-CX 10.17\r\ninterface 1/1/1\r\tdescription "α 😀 <img src=x onerror=alert(1)> &"\n\x1b[2K';for(const id of ['network-aruba-cx','network-panos-xml','text']){const html=api.highlight(text,id);assert.equal(unpaint(html),text);assert(!html.includes('<img'));}assert.equal(api.detect('x\0'+samples['network-aruba-cx']),'');
});
test('binary/opaque exports are unrecognized and large input has explicit bounds',()=>{
 assert.equal(api.detect('\0\xff\x01'),'');assert.equal(api.detect('A'.repeat(1000000)),'');assert.equal(api.detect(' '.repeat(api.MAX_DETECT)+samples['network-aruba-cx']),'');const large=samples['network-fortios']+'set alias "synthetic"\n'.repeat(60000);assert.equal(api.detect(large),'network-fortios');assert(!api.highlight(large,'network-fortios').includes('<span'));assert.equal(unpaint(api.highlight(large,'network-fortios')),large);
});
test('network overrides never offer a guessed comment command or formatter',()=>{for(const row of api.languages){assert.equal(language.formatKind(row.id),'');if(row.id!=='network-panos-xml')assert.equal(language.commentStyle(row.id),null);}assert.equal(language.commentStyle('network-panos-xml')[0],'<!--');});
test('worker dispatch uses lexer; plain override is plain and formatting stays disabled',async()=>{
 let reply;const w={};w.self=w;w.postMessage=x=>reply=x;vm.createContext(w);w.importScripts=(...names)=>names.forEach(name=>vm.runInContext(read(name),w,{filename:name}));vm.runInContext(read('editor-language-worker.js'),w);
 await w.onmessage({data:{id:1,action:'highlight',language:'network-fortios',text:samples['network-fortios']}});assert.equal(unpaint(reply.html),samples['network-fortios']);assert.match(reply.html,/nt-ed-property/);
 await w.onmessage({data:{id:2,action:'highlight',language:'text',text:samples['network-fortios']}});assert(!reply.html.includes('<span'));await w.onmessage({data:{id:3,action:'format',language:'network-fortios',text:samples['network-fortios']}});assert.match(reply.error,/not available/);
});
test('CX legacy branding and platform-prefixed versions are recognized without filename hints',()=>{
 for(const header of ['!Version ArubaOS-CX ML.10.17.0001','! Version AOS-CX FL.10.17.0001'])assert.equal(api.detect(header+'\nhostname LAB\n'),'network-aruba-cx');
 assert.notEqual(api.detect('sysname LAB\nundo shutdown\n#\ninterface GigabitEthernet0/0/1\n'),'network-comware');
});
test('unclosed quotes in comments and apostrophes in descriptions do not hide later commands',()=>{
 const source='! comment "\ndescription don\'t change this\ninterface 1/1/1\n no routing\n vlan access 20\n';assert.equal(api.detect(source),'network-aruba-cx');assert.equal(unpaint(api.highlight(source,'network-aruba-cx')),source);
});
test('malformed XML and long open tags are bounded and remain exact text',()=>{
 const text='<x'.repeat(90000);const html=api.highlight(text,'network-panos-xml');assert.equal(unpaint(html),text);
});
test('near-million-character network content paints linearly; dense token budgets fall back completely',()=>{
 const row='set alias "'+ 'synthetic '.repeat(12)+'"\n',text=(samples['network-fortios']+row.repeat(7300)).slice(0,999999);const start=performance.now(),result=api.paint(text,'network-fortios');assert(!result.limited);assert.equal(unpaint(result.html),text);assert(performance.now()-start<2000,'bounded lexical painting should complete within worker deadline');
 const dense='set x 1\n'.repeat(80000),limited=api.paint(dense,'network-fortios');assert(limited.limited);assert.match(limited.reason,/budget/);assert(!limited.html.includes('<span'));assert.equal(unpaint(limited.html),dense);
 const quotedXML='<config><devices/><deviceconfig/><network/><x a="'+ 'q'.repeat(980000)+'"/></config>',xml=api.paint(quotedXML,'network-panos-xml');assert(!xml.limited);assert.equal(unpaint(xml.html),quotedXML);
 const tags='<x/>'.repeat(200000),bounded=api.paint(tags,'network-panos-xml');assert(bounded.limited);assert.equal(unpaint(bounded.html),tags);
});
test('network-only worker size extension never raises Prism or formatting limits',async()=>{
 let reply;const w={};w.self=w;w.postMessage=x=>reply=x;vm.createContext(w);w.importScripts=(...names)=>names.forEach(name=>vm.runInContext(read(name),w,{filename:name}));vm.runInContext(read('editor-language-worker.js'),w);const text='description "'+ 'synthetic '.repeat(40000)+'"';
 await w.onmessage({data:{id:1,action:'highlight',language:'network',text}});assert(!reply.error);assert(!reply.limited);assert.equal(unpaint(reply.html),text);
 await w.onmessage({data:{id:2,action:'highlight',language:'javascript',text}});assert.match(reply.error,/250,000/);
 await w.onmessage({data:{id:3,action:'format',language:'network',text}});assert.match(reply.error,/250,000/);
});
// A decoded SonicOS settings export, made up for the test (documentation addresses only).
const sonicExport=['checksumVersion=1','buildNum=7.1.0-0000-R0000','shortProdName=TZ%20Lab','userIV=0011223344556677','passwordUniqueNum=0',
 'addrObjId_0=Lab%20Host','addrObjIdDisp_0=Lab%20Host','addrObjType_0=1','addrObjZone_0=LAN','addrObjIp1_0=192.0.2.10','addrObjIp2_0=255.255.255.255',
 'addrObjIp1VN_0_1=2001:db8::10','iface_name_0=X0','iface_lan_ip_0=192.0.2.1','smtpServerAuthPassword=123456','snmp_GetCommunity=synthetic',
 'tsAgentsAllowSvcBypass=on','prefs_color_0=#00ff00','emptyValue=','schedObjDaysOfWeek_0=0,1,2'].join('\n')+'\n';
test('a decoded SonicOS settings export is recognized by its keys whatever the file is called',()=>{
 for(const name of ['sonicwall.exp','sample.txt','sample.ini','sample.properties','sample.json','source.js','hosts','noextension','sample.cfg'])assert.equal(language.detect(name,sonicExport),'network-sonicos-exp',name);
 assert.equal(api.detect(sonicExport),'network-sonicos-exp');assert.equal(language.valid('network-sonicos-exp'),'network-sonicos-exp');assert.equal(language.label('network-sonicos-exp'),'SonicWall SonicOS settings export');
 const rows=sonicExport.trim().split('\n');
 assert.equal(language.detect('a.txt',rows.join('&')+'&&'),'network-sonicos-exp','decoded pairs still joined by &');
 assert.equal(language.detect('a.txt',[...rows].sort().join('\r\n')),'network-sonicos-exp','sorted, CRLF');
 assert.equal(language.detect('a.txt',unescape(sonicExport)),'network-sonicos-exp','URL-decoded values');
 assert.equal(language.detect('a.txt','﻿'+sonicExport),'network-sonicos-exp');
 const sortedHead=['addrObjId_0=A','addrObjIdDisp_0=A','addrObjType_0=1','addrObjZone_0=LAN','addrObjId_1=B','addrObjIdDisp_1=B','addrObjType_1=1','addrObjZone_1=LAN'].join('\n');
 assert.equal(api.settingsExport(sortedHead),'network-sonicos-exp','four keys of one family');
});
test('ordinary key=value files and mentions of SonicOS keys are not a SonicOS export',()=>{
 const ini=Array.from({length:20},(_,i)=>'name'+i+'=value'+i).join('\n');
 for(const text of [ini,ini+'\nshortProdName=x',ini+'\naddrObjId_0=x\naddrObjType_0=1','checksumVersion=1\nshortProdName=x\naddrObjId_0=x','# checksumVersion=1\n# shortProdName=x\n'+'text line\n'.repeat(10),'checksumVersion=1\nshortProdName=x\n'+'not a pair\n'.repeat(10),'\0'+sonicExport])assert.equal(api.settingsExport(text),'',JSON.stringify(text.slice(0,40)));
 assert.equal(language.detect('sample.ini',ini),'ini');assert.equal(language.detect('settings.properties',ini),'properties');
 const code='const checksumVersion=1;\nconst shortProdName="x";\n'.repeat(6);assert.equal(language.detect('source.js',code),'javascript');
});
test('SonicOS export highlighting: keys, indexes, numbers, escapes and opaque secrets, exact text',()=>{
 const html=api.highlight(sonicExport,'network-sonicos-exp');assert.equal(unpaint(html),sonicExport);
 const row=key=>html.split('\n').find(line=>unpaint(line).startsWith(key+'='));
 assert.equal(row('shortProdName'),'<span class="nt-ed-property">shortProdName</span><span class="nt-ed-operator">=</span><span class="nt-ed-string">TZ</span><span class="nt-ed-punctuation">%20</span><span class="nt-ed-string">Lab</span>');
 assert.equal(row('addrObjIp1VN_0_1'),'<span class="nt-ed-property">addrObjIp1VN</span>_<span class="nt-ed-number">0</span>_<span class="nt-ed-number">1</span><span class="nt-ed-operator">=</span><span class="nt-ed-number">2001:db8::10</span>');
 assert.match(row('addrObjIp1_0'),/nt-ed-number">192\.0\.2\.10</);assert.match(row('prefs_color_0'),/nt-ed-number">#00ff00</);assert.match(row('schedObjDaysOfWeek_0'),/nt-ed-number">0,1,2</);
 assert.match(row('smtpServerAuthPassword'),/nt-ed-string">123456</,'a numeric secret is one opaque run');assert.match(row('snmp_GetCommunity'),/nt-ed-string">synthetic</);
 assert.match(row('tsAgentsAllowSvcBypass'),/nt-ed-keyword">on</,'Bypass is not a password');
 assert.equal(row('emptyValue'),'<span class="nt-ed-property">emptyValue</span><span class="nt-ed-operator">=</span>');
 const joined=sonicExport.trim().split('\n').slice(0,3).join('&')+'&&';const painted=api.highlight(joined,'network-sonicos-exp');assert.equal(unpaint(painted),joined);assert.equal((painted.match(/nt-ed-property/g)||[]).length,3);
 const hostile='addrObjId_0=<img src=x onerror=alert(1)>&"\r\nkey=a&b=c\r';const safe=api.highlight(hostile,'network-sonicos-exp');assert.equal(unpaint(safe),hostile);assert(!safe.includes('<img'));
 assert.equal(language.formatKind('network-sonicos-exp'),'');assert.equal(language.commentStyle('network-sonicos-exp'),null);
});
test('a long SonicOS export paints quickly and the worker colours it',async()=>{
 const body=Array.from({length:12000},(_,i)=>'addrObjId_'+i+'=Host%20'+i+'\naddrObjIp1_'+i+'=192.0.2.'+(i%250)).join('\n'),text=sonicExport+body;
 const start=performance.now(),slice=text.slice(0,200000),result=api.paint(slice,'network-sonicos-exp');assert(!result.limited);assert.equal(unpaint(result.html),slice);assert(performance.now()-start<1000);
 let reply;const w={};w.self=w;w.postMessage=x=>reply=x;vm.createContext(w);w.importScripts=(...names)=>names.forEach(name=>vm.runInContext(read(name),w,{filename:name}));vm.runInContext(read('editor-language-worker.js'),w);
 await w.onmessage({data:{id:1,action:'highlight',language:'network-sonicos-exp',text:sonicExport}});assert.equal(unpaint(reply.html),sonicExport);assert.match(reply.html,/nt-ed-property/);
 assert.equal(language.detect('large.txt',text),'network-sonicos-exp');
});
