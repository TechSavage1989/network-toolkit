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
 const glyph='<svg class="saved-session-bookmark" viewBox="0 0 16 16" aria-hidden="true" focusable="false"><path d="M4 2.25h8v11.5L8 11l-4 2.75z"/></svg>';
 let chooser=null,sequence=0;
 function caption(list){return 'Saved '+(list.length===1?'connection':'connections ('+list.length+')')+'. '+list.map(description).join('; ')+'. Saved configuration only; service and availability are not verified. Review connection; no automatic connection.';}
 function portEndpoint(row,number,hint){
  if(!row||!port(number)||hint?.verified&&!hint.protocol)return null;
  return {...row,terminalType:'',endpointScheme:'',endpointProtocol:hint?.protocol||'tcp',endpointPort:port(number)};
 }
 function portData(row,key,number){
  const hint=root.NetworkToolkitDiscoveryPorts?.service?.(row,Number(number))||{},target=portEndpoint(row,number,hint),list=matches(target,profiles());
  return {target,list,caption:list.length?caption(list):'',html:'<span class="saved-session-port-slot" data-saved-session-slot="'+esc(key)+'" data-saved-session-port="'+esc(number)+'">'+(list.length?glyph:'')+'</span>'};
 }
 function content(row,key){
  const list=matches(row,profiles());if(!list.length)return '';
  const title=caption(list);
  return '<button type="button" class="saved-session-indicator" data-saved-session-open="'+esc(key)+'" title="'+esc(title)+'" aria-label="'+esc(title)+'">'+glyph+'</button>';
 }
 function render(row,key){return key&&endpoint(row)?'<span class="saved-session-slot" data-saved-session-slot="'+esc(key)+'">'+content(row,key)+'</span>':'';}
 function sync(){
  for(const slot of root.document?.querySelectorAll?.('[data-saved-session-slot]')||[]){
   const key=slot.dataset.savedSessionSlot,row=record(key);
   if(slot.dataset.savedSessionPort){
    const data=portData(row,key,slot.dataset.savedSessionPort),html=data.list.length?glyph:'',button=slot.closest?.('[data-network-port]');
    if(slot.innerHTML!==html)slot.innerHTML=html;
    if(button){for(const [name,base] of [['title',button.dataset.savedPortTitle],['aria-label',button.dataset.savedPortLabel]])if(base!==undefined)button.setAttribute(name,base+(data.caption?' '+data.caption:''));}
   }else{const html=content(row,key);if(slot.innerHTML!==html)slot.innerHTML=html;}
  }
 }
 function resolve(key,options={}){
  if(!options.port)return record(key);
  const row=root.NetworkToolkitDiscoveryPorts?.selected?.(key,options.port);if(!row)return null;
  const hint=root.NetworkToolkitDiscoveryPorts?.service?.(row,Number(options.port))||{};
  if(options.service!==undefined&&JSON.stringify(hint)!==options.service)return null;
  return portEndpoint(row,options.port,options.protocol?{protocol:options.protocol}:hint);
 }
 function signature(row){const target=endpoint(row);return target?JSON.stringify([target.ip,target.protocol,target.port]):'';}
 function profileSignature(profile){return JSON.stringify([profile.id,address(profile.host),profile.type,profile.port]);}
 function review(key,id,options={},expected){
  const row=resolve(key,options),match=matches(row,profiles()).find(value=>value.id===id),profile=profiles().find(value=>value.id===id);
  if(!match||!profile||expected&&(signature(row)!==expected.target||profileSignature(match)!==expected.profile)){
   if(typeof toast==='function')toast('This saved session no longer matches the selected device endpoint. Reopen the saved-session chooser.','warn');sync();return false;
  }
  if(typeof openTerminalConnect!=='function'||typeof safeTerminalConfig!=='function')return false;
  // Existing profile-ID review only: credential binding, vault and host-key
  // approvals still happen through the established connection workflow.
  chooser=null;closeModal();if(typeof navigate==='function')navigate('terminal');openTerminalConnect(safeTerminalConfig(profile),profile.id);return true;
 }
 function open(key,options={}){
  const row=resolve(key,options),list=matches(row,profiles());if(!list.length){sync();return false;}
  if(list.length===1)return review(key,list[0].id,options);
  const token=String(++sequence);chooser={token,key,options,target:signature(row),profiles:new Map(list.map(profile=>[profile.id,profileSignature(profile)]))};
  openModal('Saved terminal sessions','<p>These profiles match the device IP'+(endpoint(row)?.port?' and selected port':'')+'. Protocol and port are shown for each saved configuration; service and availability are not verified.</p><div class="saved-session-choices">'+list.map(profile=>'<button type="button" class="btn saved-session-choice" data-saved-session-review="'+esc(profile.id)+'" data-saved-session-key="'+esc(key)+'" data-saved-session-choice="'+token+'"><strong>'+esc(profile.name)+'</strong><span>'+esc(profile.type.toUpperCase()+' · '+profile.host+' · port '+profile.port)+'</span><span>Review connection</span></button>').join('')+'</div><p class="help-line">Nothing connects until you review the existing connection dialog and select Connect. Vault unlock, destination consent and server identity verification still apply.</p>','<button type="button" class="btn ghost" data-action="close-modal">Cancel</button>',true);return true;
 }
 function openPort(key,number,protocol){
  const row=root.NetworkToolkitDiscoveryPorts?.selected?.(key,number);if(!row)return false;
  const hint=root.NetworkToolkitDiscoveryPorts?.service?.(row,Number(number))||{};
  return open(key,{port:Number(number),protocol:protocol||'',service:JSON.stringify(hint)});
 }
 function handleClick(event){
  const choice=event.target.closest?.('[data-saved-session-review]'),button=event.target.closest?.('[data-saved-session-open]');if(!choice&&!button)return false;
  event.preventDefault();event.stopImmediatePropagation();
  if(choice){const current=chooser,id=choice.dataset.savedSessionReview;if(current?.token===choice.dataset.savedSessionChoice&&current.key===choice.dataset.savedSessionKey&&current.profiles.has(id))review(current.key,id,current.options,{target:current.target,profile:current.profiles.get(id)});else if(typeof toast==='function')toast('This chooser is no longer current. Reopen the saved connections.','warn');}
  else open(button.dataset.savedSessionOpen);return true;
 }
 root.document?.addEventListener?.('click',handleClick,true);
 const api=Object.freeze({address,endpoint,matches,render,portData,portEndpoint,sync,open,openPort,review,handleClick});root.NetworkToolkitSavedSessions=api;if(typeof module==='object'&&module.exports)module.exports=api;
})(typeof window!=='undefined'?window:globalThis);
