/* Conservative offline network configuration recognition and lexical display.
 * This is not a parser/validator. Never evaluates commands, resolves XML entities,
 * decodes exports, or changes source text. See docs/NETWORK_CONFIG_EDITOR.md. */
(function(root){'use strict';
const languages=Object.freeze([
 ['network-aruba-cx','Aruba AOS-CX config'],['network-aruba-switch','ArubaOS-Switch / ProCurve config'],
 ['network-comware','HPE / H3C Comware config'],['network-fortios','Fortinet FortiOS config'],
 ['network-sonicos','SonicWall SonicOS CLI config'],['network-panos-set','Palo Alto PAN-OS set config'],
 ['network-panos-xml','Palo Alto PAN-OS XML config'],['network-cisco-ios','Cisco IOS / IOS-XE config'],
 ['network-cisco-nxos','Cisco NX-OS config'],['network-cisco-asa','Cisco ASA config']
].map(([id,label])=>Object.freeze({id,label,aliases:[]})));
const ids=new Set(['network',...languages.map(x=>x.id)]),MAX_DETECT=65536,MAX_PAINT=1000000,MAX_SPANS=100000,MAX_HTML=8*1024*1024;
let budget=null;
function charge(chars,spans=0){if(!budget)return;budget.chars+=chars;budget.spans+=spans;if(budget.chars>MAX_HTML||budget.spans>MAX_SPANS)throw RangeError('Network syntax display budget exceeded.');}
const escape=s=>{const html=s.replace(/&/g,'&amp;').replace(/</g,'&lt;').replace(/>/g,'&gt;').replace(/"/g,'&quot;');charge(html.length);return html;};
const span=(kind,s)=>{if(!s)return '';const open='<span class="nt-ed-'+kind+'">';charge(open.length+7,1);return open+escape(s)+'</span>';};
// Walk physical lines without normalizing CRLF, CR, BOM, tabs or control bytes.
function lines(text,visit){let at=0;while(at<text.length){let end=at;while(end<text.length&&text[end]!=='\r'&&text[end]!=='\n')end++;let next=end;if(text[next]==='\r')next++;if(text[next]==='\n')next++;visit(text.slice(at,end),text.slice(end,next));at=next;}}
function quoteEnd(s,start,quote){for(let i=start;i<s.length;i++){if(s[i]==='\\'){i++;continue;}if(s[i]===quote)return i+1;}return -1;}
function bannerStart(line){const m=/^\s*banner\s+(?:motd|login|exec)\s+(\^.|[^\s])([\s\S]*)$/i.exec(line);return m&&!m[2].includes(m[1])?m[1]:'';}
function shape(text){const rows=[];let quote='',banner='';lines(text,(line)=>{if(banner){if(line.includes(banner))banner='';return;}if(quote){const end=quoteEnd(line,0,quote);if(end<0)return;quote='';return;}banner=bannerStart(line);if(banner)return;if(/^\s*[!#;]/.test(line)){rows.push(line);return;}let safe='',at=0;for(let i=0;i<line.length;i++){if(line[i]!== '"'&&line[i]!=="'"||i>0&&/[\w]/.test(line[i-1]))continue;safe+=line.slice(at,i);const end=quoteEnd(line,i+1,line[i]);if(end<0){quote=line[i];at=line.length;break;}safe+=' ';i=end-1;at=end;}safe+=line.slice(at);rows.push(safe);});return rows;}
function detect(text){
 if(typeof text!=='string'||!text)return '';const head=text.slice(0,MAX_DETECT).replace(/^\uFEFF/,'');
 if(/[\x00-\x08\x0e-\x1f]/.test(head))return ''; // Binary/terminal-control export: do not reinterpret it.
 const rows=shape(head),has=re=>rows.some(s=>re.test(s)),count=re=>rows.filter(s=>re.test(s)).length;
 const hits=new Set();
 if(has(/^\s*!\s*Version\s+(?:ArubaOS-CX|AOS-CX)\s+(?:[a-z]{1,5}\.)?\d/i))hits.add('network-aruba-cx');
 if(has(/^\s*;[^\r\n]*(?:ProCurve|ArubaOS-Switch|Configuration Editor)[^\r\n]*(?:release|Configuration Editor)/i))hits.add('network-aruba-switch');
 if(has(/^\s*#config-version=(?:FGT|FGVM|FG[0-9]|FortiGate)/i))hits.add('network-fortios');
 if(has(/^\s*ASA Version\s+\d/i))hits.add('network-cisco-asa');
 if(has(/^\s*(?:!\s*)?(?:Cisco )?NX-?OS(?:\s+Software,?\s+|\s*:\s*|\s+)Version\s+\d/i)||has(/^\s*boot\s+nxos\s+\S+/i))hits.add('network-cisco-nxos');
 if(has(/^\s*firmware-version\s+/i)&&has(/^\s*(?:address-(?:object|group)\s+ipv[46]|nat-policy|access-rule\s+ipv[46])\b/i))hits.add('network-sonicos');
 if(has(/^\s*sysname\s+\S+/i)&&has(/^\s*(?:version\s+\d+\.\d+[^\r\n]*\b(?:Release|Test)\b|port\s+link-mode\s+(?:bridge|route))/i)&&has(/^\s*(?:#\s*$|return\s*$|interface\s+(?:GigabitEthernet|Bridge-Aggregation))/i))hits.add('network-comware');
 // The same common words occur in many vendors. Require independent structure.
 const commands=rows.filter(s=>!/^\s*[!#;]/.test(s));const cmd=re=>commands.some(s=>re.test(s));
 if(cmd(/^\s*interface\s+\d+\/\d+\/\d+\s*$/i)&&cmd(/^\s*vlan\s+(?:access|trunk (?:allowed|native))\b/i)&&cmd(/^\s*(?:no\s+)?routing\s*$/i))hits.add('network-aruba-cx');
 if(cmd(/^\s*vlan\s+\d+\s*$/i)&&cmd(/^\s*(?:un)?tagged\s+[a-z]?\d[\d,a-z/-]*\s*$/i)&&cmd(/^\s*(?:name\s+|timesync\s+|snmp-server\s+|spanning-tree\s*)/i))hits.add('network-aruba-switch');
 if(cmd(/^\s*config\s+(?:system|firewall|router|vpn|vdom|antivirus|webfilter)\s+\S+/i)&&cmd(/^\s*set\s+\S+\s+/i)&&cmd(/^\s*end\s*$/i)&&(cmd(/^\s*edit\s+/i)&&cmd(/^\s*next\s*$/i)||count(/^\s*config\s+/i)>1))hits.add('network-fortios');
 const panPaths=new Set();for(const line of commands){const m=/^\s*set\s+(deviceconfig|mgt-config|rulebase|network|zone|shared|vsys)\s+\S+/i.exec(line);if(m)panPaths.add(m[1].toLowerCase());}
 if(panPaths.size>=2&&(panPaths.has('deviceconfig')||panPaths.has('mgt-config')||panPaths.has('rulebase')))hits.add('network-panos-set');
 // Lexical XML recognition only: comments/CDATA cannot supply vendor evidence.
 const xml=head.replace(/<!--[\s\S]*?(?:-->|$)|<!\[CDATA\[[\s\S]*?(?:\]\]>|$)/g,'');
 if(/^\s*(?:<\?xml[^>]*>\s*)?<config\b/i.test(xml)&&/<devices[\s/>]/i.test(xml)&&/<deviceconfig[\s/>]/i.test(xml)&&/<(?:vsys|mgt-config|network)[\s/>]/i.test(xml))hits.add('network-panos-xml');
 if(!hits.size&&cmd(/^\s*(?:version\s+\d+\.\d+|!\s*Cisco IOS)/i)&&cmd(/^\s*interface\s+(?:GigabitEthernet|FastEthernet|TenGigabitEthernet|TwentyFiveGigE|HundredGigE|Vlan)\s*\d/i)&&cmd(/^\s*(?:switchport\s+(?:mode|access|trunk)|line\s+(?:vty|con)|service\s+timestamps)\b/i)&&!cmd(/^\s*config-file-header\b/i))hits.add('network-cisco-ios');
 if(hits.size===1)return [...hits][0];if(hits.size>1)return 'network';
 const families=[/^\s*interface\s+\S+/i,/^\s*(?:switchport\s+(?:mode|access|trunk)|vlan\s+\d+)/i,/^\s*(?:hostname|sysname)\s+\S+/i,/^\s*(?:ip\s+(?:address|route|routing)|snmp-server|spanning-tree)\b/i];
 return families.filter(cmd).length>=3?'network':'';
}
const starters=new Set(('interface vlan switchport router hostname sysname ip ipv6 address description alias set unset edit next end config firewall policy route dhcp enable disable exit return save commit no undo permit deny port trunk bridge tagged untagged forbid name snmp-server snmp-agent spanning-tree timesync ntp sntp logging aaa username user password secret key banner version boot feature line access-list access-group object object-group nameif security-level nat service channel-group lacp duplex speed shutdown routing vrf ospf bgp zone deviceconfig mgt-config rulebase network shared vsys address-object address-group service-object service-group nat-policy access-rule firmware-version checksum config-file-header config-file-trailer').split(' '));
const opaqueFields=new Set(['description','alias','comment','comments','password','passwd','secret','psksecret','private-key','pre-shared-key','community','phash','key-string','encrypted-password','ciphertext']);
function trackQuote(s,state){for(let i=0;i<s.length;i++){if((s[i]!== '"'&&s[i]!=="'")||i>0&&/\w/.test(s[i-1]))continue;const end=quoteEnd(s,i+1,s[i]);if(end<0){state.quote=s[i];return;}i=end-1;}}
function values(s,state){let out='',at=0;const re=/["']|\b\d+(?:\.\d+){0,3}(?:\/\d+)?\b/g;let m;while((m=re.exec(s))){out+=escape(s.slice(at,m.index));if(m[0]==='"'||m[0]==="'"){const end=quoteEnd(s,m.index+1,m[0]);if(end<0){state.quote=m[0];out+=span('string',s.slice(m.index));return out;}out+=span('string',s.slice(m.index,end));at=end;re.lastIndex=end;}else{out+=span('number',m[0]);at=re.lastIndex;}}return out+escape(s.slice(at));}
function cliHighlight(text){let out='';const state={quote:'',banner:''};lines(text,(line,eol)=>{charge(eol.length);
 if(state.banner){out+=span('string',line)+eol;if(line.includes(state.banner))state.banner='';return;}
 if(state.quote){const end=quoteEnd(line,0,state.quote);if(end<0){out+=span('string',line)+eol;return;}out+=span('string',line.slice(0,end))+escape(line.slice(end))+eol;state.quote='';return;}
 if(/^\s*[!#;]/.test(line)){out+=span('comment',line)+eol;return;}
 const m=/^([\t \uFEFF]*)([\w-]+)([\s\S]*)$/.exec(line);if(!m||!starters.has(m[2].toLowerCase())){out+=escape(line)+eol;return;}
 charge(m[1].length);let prefix=m[1]+span('keyword',m[2]),rest=m[3],command=m[2].toLowerCase();
 if(command==='banner'){state.banner=bannerStart(line);out+=prefix+span('string',rest)+eol;return;}
 if(command==='no'||command==='undo'){const sub=/^(\s+)([\w-]+)([\s\S]*)$/.exec(rest);if(sub&&starters.has(sub[2].toLowerCase())){charge(sub[1].length);prefix+=sub[1]+span('keyword',sub[2]);rest=sub[3];command=sub[2].toLowerCase();}}
 if(opaqueFields.has(command)){trackQuote(rest,state);out+=prefix+span('string',rest)+eol;return;}
 // Field names are meaningful only in their command position, never in values.
 if(['set','unset','config','ip','ipv6','port','switchport','vlan','object','object-group','service','snmp-server','snmp-agent'].includes(command)){
  const field=/^(\s+)([a-z][\w-]*)([\s\S]*)$/i.exec(rest);if(field){charge(field[1].length);prefix+=field[1]+span('property',field[2]);rest=field[3];if(opaqueFields.has(field[2].toLowerCase())){trackQuote(rest,state);out+=prefix+span('string',rest)+eol;return;}}
 }
 // Sensitive tails and free descriptions remain opaque, including numeric-looking
 // ciphertext. Match unquoted words only; quoted object names are not fields.
 let opaque=-1;const words=/"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|\b[\w-]+\b/g;let w;while((w=words.exec(rest))){if(opaqueFields.has(w[0].toLowerCase())){opaque=w.index+w[0].length;break;}}
 if(opaque>=0){trackQuote(rest.slice(opaque),state);out+=prefix+values(rest.slice(0,opaque),state)+span('string',rest.slice(opaque))+eol;return;}
 out+=prefix+values(rest,state)+eol;
 });return out;}
function xmlHighlight(text){let out='',at=0;while(at<text.length){const start=text.indexOf('<',at);if(start<0){out+=escape(text.slice(at));break;}out+=escape(text.slice(at,start));let end,kind='keyword';if(text.startsWith('<!--',start)){const close=text.indexOf('-->',start+4);end=close<0?text.length:close+3;kind='comment';}else if(text.startsWith('<![CDATA[',start)){const close=text.indexOf(']]>',start+9);end=close<0?text.length:close+3;kind='string';}else{let quote='';end=start+1;for(;end<text.length;end++){const ch=text[end];if(quote){if(ch===quote)quote='';}else if(ch==='"'||ch==="'")quote=ch;else if(ch==='>'){end++;break;}}}const tag=text.slice(start,end);if(kind!=='keyword')out+=span(kind,tag);else{let p=0;const attrs=/"[^"]*"|'[^']*'/g;let a;while((a=attrs.exec(tag))){out+=span('keyword',tag.slice(p,a.index))+span('string',a[0]);p=attrs.lastIndex;}out+=span('keyword',tag.slice(p));}at=end;}return out;}
function paint(text,id){text=String(text);if(!ids.has(id))return {html:escape(text),limited:false,reason:''};
 const previous=budget;budget=null;try{if(text.length>MAX_PAINT)return {html:escape(text),limited:true,reason:'Network syntax display is limited to 1,000,000 characters; the complete source remains editable as plain text.'};
 budget={chars:0,spans:0};return {html:id==='network-panos-xml'?xmlHighlight(text):cliHighlight(text),limited:false,reason:''};}
 catch(error){if(!(error instanceof RangeError))throw error;budget=null;return {html:escape(text),limited:true,reason:'Network syntax display exceeded its token/HTML budget; the complete source remains editable as plain text.'};}
 finally{budget=previous;}
}
function highlight(text,id){return paint(text,id).html;}
root.NTKNetworkConfig=Object.freeze({languages,detect,supports:id=>ids.has(id),highlight,paint,MAX_DETECT,MAX_PAINT,MAX_SPANS,MAX_HTML});
})(globalThis);
