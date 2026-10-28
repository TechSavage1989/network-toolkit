/* Saved profile metadata only. Badges never inspect a vault or connect. */
(function(root){
 'use strict';
 const esc=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 const protocols=new Set(['ssh','telnet']);
 function port(value,fallback){if(value===undefined||value===null||value==='')return fallback||0;const n=Number(value);return Number.isInteger(n)&&n>=1&&n<=65535?n:0;}
 function address(value){
  let text=String(value??'').trim();if(text.startsWith('[')&&text.endsWith(']'))text=text.slice(1,-1);
  const at=text.indexOf('%'),zone=at<0?'':text.slice(at+1),bare=at<0?text:text.slice(0,at);
  if(at>=0&&!/^[\w.-]+$/.test(zone))return '';
  const ip=root.NetworkToolkitDiscoveryFilter?.ip(bare)||'';if(!ip)return '';
  if(at>=0&&!ip.startsWith('6:'))return '';
  if(at>=0&&/^\d+$/.test(zone)&&(!Number.isSafeInteger(Number(zone))||Number(zone)<1||Number(zone)>0xffffffff))return '';
  // Link-local literals are not unique across interfaces. An omitted zone is
  // insufficient identity evidence; never equate such profiles across adapters.
  if(ip.startsWith('6:')&&(parseInt(ip.split(':')[1],16)&0xffc0)===0xfe80&&at<0)return '';
  return ip+(at>=0?'%'+(/^\d+$/.test(zone)?String(Number(zone)):zone):'');
 }
 function endpoint(row){
  if(!row||typeof row!=='object')return null;
  const ip=address(row.host||row.address);if(!ip)return null;
  const protocol=String(row.terminalType||row.endpointScheme||row.endpointProtocol||row.protocol||'').trim().toLowerCase();
  if(protocol&&!protocols.has(protocol)&&!['tcp','icmp','icmpv6','arp'].includes(protocol))return null;
  const explicit=row.endpointPort??row.port,number=port(explicit);
  if(explicit!==undefined&&explicit!==null&&explicit!==''&&!number)return null;
  return {ip,host:String(row.host||row.address),protocol:protocols.has(protocol)?protocol:'',port:number};
 }
 function matches(row,profiles){
  const target=endpoint(row);if(!target)return [];
  const seen=new Set(),list=[],input=Array.isArray(profiles)?profiles:[],counts=new Map();
  for(const profile of input)if(typeof profile?.id==='string')counts.set(profile.id,(counts.get(profile.id)||0)+1);
  for(const profile of input){
   if(!profile||typeof profile.id!=='string'||!profile.id||counts.get(profile.id)!==1||seen.has(profile.id)||!protocols.has(profile.type))continue;
   const number=port(profile.port,profile.type==='ssh'?22:23);
   if(!number||address(profile.host)!==target.ip||target.protocol&&profile.type!==target.protocol||target.port&&number!==target.port)continue;
   seen.add(profile.id);
   // Do not return usernames, credentials, keys or arbitrary profile fields.
   list.push({id:profile.id,name:String(profile.name||'Unnamed session'),type:profile.type,port:number,host:String(profile.host),exactEndpoint:!!target.protocol&&!!target.port});
  }
  return list.sort((a,b)=>a.type.localeCompare(b.type)||a.port-b.port||a.name.localeCompare(b.name)||a.id.localeCompare(b.id));
 }
 function profiles(){try{return state.terminalProfiles||[];}catch{return root.state?.terminalProfiles||[];}}
 function record(key){try{return deviceRecords.get(key);}catch{return root.deviceRecords?.get(key);}}
 function description(profile){const host=profile.host.includes(':')?'['+profile.host.replace(/^\[|\]$/g,'')+']':profile.host;return profile.name+' · '+profile.type.toUpperCase()+' · '+host+':'+profile.port;}
 function content(row,key){
  const list=matches(row,profiles());if(!list.length)return '';
  const title=list.map(description).join('\n')+'\nSaved configuration only; service and availability are not verified. Open to review; no automatic connection.';
  const label=list.length===1?'Saved '+list[0].type.toUpperCase()+' :'+list[0].port:'Saved sessions ('+list.length+')';
  return '<button type="button" class="saved-session-indicator" data-saved-session-open="'+esc(key)+'" title="'+esc(title)+'" aria-label="'+esc(label+'. '+title)+'">'+esc(label)+'</button>';
 }
 function render(row,key){return key&&endpoint(row)?'<span class="saved-session-slot" data-saved-session-slot="'+esc(key)+'">'+content(row,key)+'</span>':'';}
 function sync(){
  for(const slot of root.document?.querySelectorAll?.('[data-saved-session-slot]')||[]){const html=content(record(slot.dataset.savedSessionSlot),slot.dataset.savedSessionSlot);if(slot.innerHTML!==html)slot.innerHTML=html;}
 }
 function review(key,id){
  const row=record(key),match=matches(row,profiles()).find(value=>value.id===id),profile=profiles().find(value=>value.id===id);
  if(!match||!profile){if(typeof toast==='function')toast('This saved session no longer matches this device endpoint. Reopen the saved-session chooser.','warn');sync();return false;}
  if(typeof openTerminalConnect!=='function'||typeof safeTerminalConfig!=='function')return false;
  // This is the existing review form, not connectTerminal or a backend call.
  // It preserves the saved credential ID and all native/vault/host-key gates.
  closeModal();if(typeof navigate==='function')navigate('terminal');openTerminalConnect(safeTerminalConfig(profile),profile.id);return true;
 }
 function open(key){
  const list=matches(record(key),profiles());if(!list.length){sync();return false;}
  if(list.length===1)return review(key,list[0].id);
  openModal('Saved terminal sessions','<p>These profiles match the device IP'+(endpoint(record(key))?.port?' and selected port':'')+'. Protocol and port are shown for each saved configuration; service and availability are not verified.</p><div class="saved-session-choices">'+list.map(profile=>'<button type="button" class="btn saved-session-choice" data-saved-session-review="'+esc(profile.id)+'" data-saved-session-key="'+esc(key)+'"><strong>'+esc(profile.name)+'</strong><span>'+esc(profile.type.toUpperCase()+' · '+profile.host+' · port '+profile.port)+'</span><span>Review connection</span></button>').join('')+'</div><p class="help-line">Nothing connects until you review the existing connection dialog and select Connect. Vault unlock, destination consent and server identity verification still apply.</p>','<button type="button" class="btn ghost" data-action="close-modal">Cancel</button>',true);return true;
 }
 function handleClick(event){const choice=event.target.closest?.('[data-saved-session-review]'),button=event.target.closest?.('[data-saved-session-open]');if(!choice&&!button)return false;event.preventDefault();event.stopImmediatePropagation();if(choice)review(choice.dataset.savedSessionKey,choice.dataset.savedSessionReview);else open(button.dataset.savedSessionOpen);return true;}
 root.document?.addEventListener?.('click',handleClick,true);
 const api=Object.freeze({address,endpoint,matches,render,sync,open,review,handleClick});root.NetworkToolkitSavedSessions=api;if(typeof module==='object'&&module.exports)module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
