(function(root){
'use strict';
const LIMIT=200, timers=new WeakMap();
function name(value){return typeof value==='string'&&value.length<=100&&/^[A-Za-z0-9][A-Za-z0-9_.-]*$/.test(value)&&/[A-Za-z]/.test(value)&&!/^(?:password|username|login|error|warning|failed|success|yes|no)$/i.test(value)?value:'';}
function parse(line){const m=String(line).match(/^([A-Za-z0-9][A-Za-z0-9_.-]{0,99})(?:\(config(?:-[A-Za-z0-9_-]+)?\))?[#>] ?$/);return m?name(m[1]):'';}
function endpoint(value){return typeof value==='string'&&value.length<=512&&!/[\x00-\x20\x7f-\x9f]/.test(value)?value:'';}
function identity(value){return [value?.type,endpoint(value?.host),Number(value?.port)|| (value?.type==='telnet'?23:22),typeof value?.username==='string'?value.username:''];}
function key(value){return JSON.stringify(identity(value));}
function normalize(value){const seen=new Set(),out=[];for(const item of Array.isArray(value)?value:[]){if(out.length>=LIMIT)break;if(!item||!['ssh','telnet'].includes(item.type)||!endpoint(item.host)||!name(item.name)||!Number.isInteger(item.port)||item.port<1||item.port>65535||typeof item.username!=='string'||item.username.length>256||/[\x00-\x1f\x7f-\x9f]/.test(item.username))continue;const id=key(item);if(seen.has(id))continue;seen.add(id);out.push({type:item.type,host:item.host,port:item.port,username:item.username,name:item.name});}return out;}
function labels(value,entry){const network=['ssh','telnet'].includes(value?.type),address=endpoint(value?.host);if(!network)return {name:value?.label||value?.name||value?.serialPort||value?.type||'Terminal',address:'',network:false};const cached=normalize(state.terminalPromptNames).find(item=>key(item)===key(value)),learned=name(entry?._ntPromptName)||cached?.name||'';return {name:learned||address||'Terminal',address:learned&&learned!==address?address:'',network:true};}
function html(value,entry){const label=labels(value,entry);return '<span class="terminal-device-label"><span>'+esc(label.name)+'</span>'+(label.address?'<small>'+esc(label.address)+'</small>':'')+'</span>';}
function title(value,entry){const label=labels(value,entry);return [label.name,label.address].filter(Boolean).join('\n');}
function observe(entry){
 clearTimeout(timers.get(entry));
 if(!entry?.xterm||!['ssh','telnet'].includes(entry.info?.type)||entry.info.state!=='connected')return;
 const revision=entry.outputRevision,term=entry.xterm;
 timers.set(entry,setTimeout(()=>{
  if(entry.xterm!==term||entry.outputRevision!==revision||entry.info.state!=='connected'||entry.renderPendingBytes||entry.inputBytes||entry.inputSending||term.buffer.active!==term.buffer.normal||term.modes?.bracketedPasteMode)return;
  const b=term.buffer.active,line=b.getLine(b.baseY+b.cursorY);if(!line||line.isWrapped)return;
  const text=line.translateToString(true),learned=parse(text);if(!learned||text.slice(b.cursorX).trim()||entry._ntPromptName===learned)return;
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
root.NetworkToolkitTerminalPromptLabels=Object.freeze({name,parse,normalize,key,labels,html,title,observe,dispose,restore,snapshot});
})(window);
