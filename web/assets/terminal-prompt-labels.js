(function(root){
'use strict';
const LIMIT=200, timers=new WeakMap();
function name(value){return typeof value==='string'&&value.length<=100&&!/[\x00-\x20\x7f-\x9f]/.test(value)&&/^[A-Za-z0-9][A-Za-z0-9_.-]*$/.test(value)&&!/^\d+(?:\.\d+)+$/.test(value)&&!/^(?:password|username|login|error|warning|failed|success|yes|no|loader(?:-[a-z])?|auditor|console|(?:bash|zsh|sh|fish)(?:-[0-9.]+)?|ps|edit)$/i.test(value)?value:'';}
// Recognition is display-only. These bounded grammars do not authorize paced sending.
function parse(line,previousName=''){
 if(typeof line!=='string'||line.length>512||/[\x00-\x1f\x7f-\x9f]/.test(line))return '';
 const previous=name(previousName),host='([A-Za-z0-9][A-Za-z0-9_.-]{0,99})',user='[A-Za-z0-9_][A-Za-z0-9_.-]{0,63}';
 const path='(?:~[A-Za-z0-9_.\\/-]*|\\/[A-Za-z0-9_.\\/~+-]*)';
 const mode='(?:config(?:-[A-Za-z0-9_./-]+)?|conf(?:-[A-Za-z0-9_./-]+)?|vlan-[0-9]+|eth-[0-9/]+|stacking)';
 const match=pattern=>line.match(new RegExp('^'+pattern+' ?$'));
 let m;
 // Junos/PAN-OS/SonicWall operational prompts, and Linux user@host:path prompts.
 if((m=match(user+'@'+host+'(?:'+':'+path+')?[#$>]')))return name(m[1]);
 // RouterOS: optional menu path and safe-mode suffix, but only the bracketed host.
 if((m=match('\\['+user+'@'+host+'\\](?: /[A-Za-z0-9_./-]*)? ?(?:>|<SAFE>)')))return name(m[1]);
 // ESXi bracketed shell and Check Point Expert shell (numeric context, not host).
 if((m=match('\\['+user+'@'+host+':(?:'+path+'|[0-9]{1,5})\\][#$]?')))return name(m[1]);
 // fish/zsh common user@host forms; local PS/CMD paths are deliberately excluded.
 if((m=match(user+'@'+host+' '+path+' ?[>%]')))return name(m[1]);
 if((m=match(host+'%')))return name(m[1]);
 if((m=match('\\['+host+'\\]: PS [A-Za-z]:\\\\[A-Za-z0-9_ .\\\\/-]*>')))return name(m[1]);
 // Cisco IOS XR location is not part of the configured hostname.
 if((m=match('(?:RP|LC)/[0-9]+/(?:RP|RSP|LC)?[0-9]+/CPU[0-9]+:'+host+'(?:\\('+mode+'\\))?[#>]')))return name(m[1]);
 // Nokia classic and MD CLI. Slot/status/username/context never become names.
 if((m=match('\\*?[A-D]:'+user+'@'+host+'[#>]')))return name(m[1]);
 if((m=match('\\*?[A-D]:'+host+'(?:>[A-Za-z0-9_.-]+)*[#$>]')))return name(m[1]);
 // NetApp cluster CLI keeps the cluster identifier before the double colon.
 if((m=match(host+'::(?:\\*|[A-Za-z][A-Za-z0-9_-]*)?>')))return name(m[1]);
 if((m=match('SP '+host+'>')))return name(m[1]);
 // Aruba controller/Instant parentheses enclose the hostname, unlike config modes.
 if((m=match('\\('+host+'\\)(?: \\[[A-Za-z0-9_./-]+\\])?(?: \\('+mode+'\\))? ?[#>]')))return /^(?:config|conf)(?:-|$)/i.test(m[1])?'':name(m[1]);
 // FortiGate and SonicWall configuration forms explicitly retain the host.
 if((m=match(host+'(?: \\([A-Za-z0-9_.-]{1,64}\\))? [#>]'))) {
  // EXOS uses a space before # and an incrementing suffix. Do not strip an
  // ambiguous dotted hostname unless a prior name or documented model identifies it.
  const counter=m[1].match(/^(.+)\.[0-9]+$/);
  if(counter&&(counter[1]===previous||/^X[0-9]{3,}[A-Za-z0-9_-]*$/.test(counter[1])))return name(counter[1]);
  return name(m[1]);
 }
 if((m=match('config\\('+host+'\\)#')))return name(m[1]);
 // F5 tmsh includes explicit role/partition/tmos context after the host.
 if((m=match(user+'@'+host+'(?:\\((?:Active|Standby|/Common|cfg-sync [A-Za-z0-9_-]+)\\)){1,4}\\(tmos(?:\\.[A-Za-z0-9_-]+)*\\)#')))return name(m[1]);
 // EXOS stack/member prefix makes the final numeric prompt counter unambiguous.
 if((m=match('\\*? ?Slot-[0-9]+ (?:VPEX )?'+host+'\\.[0-9]+ #')))return name(m[1]);
 // Comware user view is unambiguous. System/subview requires a previously known
 // hostname, because hyphens belong to both valid hostnames and view suffixes.
 if((m=match('<'+host+'>')))return name(m[1]);
 if((m=match('\\[[~*]?([A-Za-z0-9][A-Za-z0-9_./-]{0,199})\\]')))
  return previous&&(m[1]===previous||m[1].startsWith(previous+'-'))?previous:'';
 // ASA slash fields are configurable and may omit the hostname altogether.
 if((m=match('([A-Za-z0-9][A-Za-z0-9_./-]{0,199})(?:\\('+mode+'\\))?[#>]'))&&m[1].includes('/'))
  return previous&&m[1].split('/').includes(previous)?previous:'';
 // Classic IOS/Arista/Dell/Ruckus/Aruba modes; never strip a hostname hyphen.
 if((m=match(host+'(?:\\('+mode+'\\))?[#>]')))return name(m[1]);
 return '';
}
function endpoint(value){return typeof value==='string'&&value.length<=512&&!/[\x00-\x20\x7f-\x9f]/.test(value)?value:'';}
function identity(value){return [value?.type,endpoint(value?.host),Number(value?.port)|| (value?.type==='telnet'?23:22),typeof value?.username==='string'?value.username:''];}
function key(value){return JSON.stringify(identity(value));}
function normalize(value){const seen=new Set(),out=[];for(const item of Array.isArray(value)?value:[]){if(out.length>=LIMIT)break;if(!item||!['ssh','telnet'].includes(item.type)||!endpoint(item.host)||!name(item.name)||!Number.isInteger(item.port)||item.port<1||item.port>65535||typeof item.username!=='string'||item.username.length>256||/[\x00-\x1f\x7f-\x9f]/.test(item.username))continue;const id=key(item);if(seen.has(id))continue;seen.add(id);out.push({type:item.type,host:item.host,port:item.port,username:item.username,name:item.name});}return out;}
function displayName(value){return typeof value==='string'&&value.trim().length<=100&&!/[\x00-\x1f\x7f-\x9f]/.test(value)?value.trim():'';}
function manual(value){const candidate=displayName(value?.manualName||value?.name||value?.label);if(!candidate)return '';if(value?.nameExplicit===true||value?.labelSource==='manual')return candidate;if(value?.nameExplicit===false||value?.labelSource==='default')return '';const host=endpoint(value?.host),port=Number(value?.port)||(value?.type==='telnet'?23:22),address=host.includes(':')?'['+host+']':host;const defaults=[host,address,address+':'+port,host+':'+port,String(value?.type||''),String(value?.type||'')+' '+address+':'+port,'Terminal'];return defaults.some(v=>candidate.toLowerCase()===v.toLowerCase())?'':candidate;}
function profileFor(value){const profiles=Array.isArray(state.terminalProfiles)?state.terminalProfiles:[];if(value?.profileId)return profiles.find(item=>item.id===value.profileId)||null;const matches=profiles.filter(item=>key(item)===key(value));return matches.length===1?matches[0]:null;}
function eligibleRecent(value){const matches=(state.terminalProfiles||[]).filter(item=>key(item)===key(value));return matches.length<=1?(state.terminalRecent||[]).filter(item=>key(item)===key(value)):[];}
function labels(value,entry){const network=['ssh','telnet'].includes(value?.type),address=endpoint(value?.host);if(!network)return {name:value?.label||value?.name||value?.serialPort||value?.type||'Terminal',address:'',network:false};const cached=normalize(state.terminalPromptNames).find(item=>key(item)===key(value)),learned=name(entry?._ntPromptName)||cached?.name||'',profile=profileFor(value),recent=(state.terminalRecent||[]).find(item=>key(item)===key(value));let chosen=manual(profile);if(!chosen&&!profile){const staleProfile=value?.profileId&&!profile&&!(typeof DETACHED_TERMINAL!=='undefined'&&DETACHED_TERMINAL);chosen=staleProfile&&value?.labelSource!=='manual'?'':manual(value);}if(!chosen&&!profile&&!value?.profileId)chosen=manual(recent);chosen=chosen||learned||address||'Terminal';return {name:chosen,address:chosen!==address?address:'',network:true};}
// Update display metadata only; connection settings and credential references are untouched.
function rename(value,label){label=displayName(label);if(!label)return false;const profile=profileFor(value);if(profile){profile.name=label;profile.nameExplicit=true;}for(const item of eligibleRecent(value)){item.name=label;item.nameExplicit=true;}if(value){value.label=label;value.manualName=label;value.labelSource='manual';value.nameExplicit=true;}return true;}
function revision(value){return Number.isSafeInteger(value?.nameRevision)&&value.nameRevision>=0?value.nameRevision:0;}
// Responses carry only metadata for this already-authorized session. Older
// polling snapshots cannot undo a local pending edit or a newer persisted name.
function sync(value){const rev=revision(value);if(!rev)return false;const profile=profileFor(value),items=[...(profile?[profile]:[]),...eligibleRecent(value)];let changed=false;for(const item of items){const old=revision(item);if(rev<old)continue;if(item._namePending){if(rev>old)item.nameRevision=rev;continue;}const explicit=value.nameExplicit===true,candidate=displayName(value.manualName);if(explicit&&candidate)item.name=candidate;item.nameExplicit=explicit;item.nameRevision=rev;changed=true;}return changed;}
function accept(value){const profile=profileFor(value);for(const item of [...(profile?[profile]:[]),...eligibleRecent(value)]){if(revision(value)>=revision(item))delete item._namePending;}return sync(value);}
function mergeSaved(value){if(!value||typeof value!=='object')return;for(const [field,collection] of [['profiles','terminalProfiles'],['recent','terminalRecent']])for(const incoming of Array.isArray(value[field])?value[field]:[]){const items=(state[collection]||[]).filter(item=>item.id===incoming?.id);if(items.length!==1||!Number.isSafeInteger(incoming.nameRevision)||incoming.nameRevision<revision(items[0]))continue;const item=items[0],candidate=displayName(incoming.name);if(!candidate)continue;const same=item.name===candidate&&item.nameExplicit===incoming.nameExplicit;item.nameRevision=incoming.nameRevision;if(!item._namePending||same){item.name=candidate;item.nameExplicit=incoming.nameExplicit===true;delete item._namePending;}}}
function html(value,entry){const label=labels(value,entry);return '<span class="terminal-device-label"><span>'+esc(label.name)+'</span>'+(label.address?'<small>'+esc(label.address)+'</small>':'')+'</span>';}
function title(value,entry){const label=labels(value,entry);return [label.name,label.address].filter(Boolean).join('\n');}
function observe(entry){
 clearTimeout(timers.get(entry));
 if(!entry?.xterm||!['ssh','telnet'].includes(entry.info?.type)||entry.info.state!=='connected')return;
 const revision=entry.outputRevision,term=entry.xterm;
 timers.set(entry,setTimeout(()=>{
  if(entry.xterm!==term||entry.outputRevision!==revision||entry.info.state!=='connected'||entry.renderPendingBytes||entry.inputBytes||entry.inputSending||term.buffer.active!==term.buffer.normal||term.modes?.bracketedPasteMode)return;
  const b=term.buffer.active,line=b.getLine(b.baseY+b.cursorY);if(!line||line.isWrapped)return;
  const text=line.translateToString(true),learned=parse(text,snapshot(entry));if(!learned||text.slice(b.cursorX).trim()||entry._ntPromptName===learned)return;
  remember(entry,learned);
  renderTerminalChrome();
 },250));
}
function remember(entry,value){const learned=name(value);if(!learned||!['ssh','telnet'].includes(entry.info?.type))return;
  entry._ntPromptName=learned;
  const config={...entry.config,...entry.info},id=key(config),metadata={type:config.type,host:config.host,port:config.port|| (config.type==='telnet'?23:22),username:config.username||'',name:learned};
  const old=normalize(state.terminalPromptNames);state.terminalPromptNames=normalize([metadata,...old.filter(item=>key(item)!==id)]);
  if(JSON.stringify(old)!==JSON.stringify(state.terminalPromptNames)&&!(typeof DETACHED_TERMINAL!=='undefined'&&DETACHED_TERMINAL))saveState();
 }
function restore(entry,value){remember(entry,value);}
function snapshot(entry){return name(entry?._ntPromptName)||normalize(state.terminalPromptNames).find(item=>key(item)===key(entry?.info))?.name||'';}
function dispose(entry){clearTimeout(timers.get(entry));timers.delete(entry);}
root.NetworkToolkitTerminalPromptLabels=Object.freeze({name,parse,normalize,key,manual,rename,revision,sync,accept,mergeSaved,labels,html,title,observe,dispose,restore,snapshot});
})(window);
