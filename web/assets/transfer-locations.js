/* Explicit transfer-location copying. No credentials, API requests, file reads,
   connection launches or transfer actions belong in this module. */
(function(root){'use strict';
 const protocols=new Set(['tftp','sftp','ftp']);
 const clean=(value,max=4096)=>typeof value==='string'&&value.length<=max&&!/[\x00-\x1f\x7f]/.test(value)&&![...value].some(c=>c.length===1&&c.charCodeAt(0)>=0xd800&&c.charCodeAt(0)<=0xdfff)?value:null;
 const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 function host(value){
  let text=clean(value,320);if(!text||text!==text.trim())return null;
  if(text.startsWith('[')&&text.endsWith(']'))text=text.slice(1,-1);
  if(/[\s/@\\?#]/.test(text))return null;
  if(text.includes(':')){
   const split=text.split('%');if(split.length>2||split.length===2&&!/^[a-z\d_.-]{1,64}$/i.test(split[1]))return null;
   let address;try{address=new URL('http://['+split[0]+']/').hostname.slice(1,-1).toLowerCase();}catch{return null;}
   if(!address.includes(':')||address.startsWith('ff'))return null;
   const mapped=address.match(/^::ffff:([\da-f]+):([\da-f]+)$/),v4=mapped?parseInt(mapped[1],16)*65536+parseInt(mapped[2],16):null;
   if(v4!==null&&((v4>>>28)===14||v4===4294967295))return null;
   return {value:address+(split[1]?'%'+split[1]:''),family:6,wildcard:address==='::'||v4===0,loopback:address==='::1'||v4!==null&&(v4>>>24)===127,scoped:!!split[1],linkLocal:(parseInt(address.split(':')[0],16)&0xffc0)===0xfe80};
  }
  if(/^\d+(?:\.\d+)*$/.test(text)){
   const parts=text.split('.');if(parts.length!==4||parts.some(p=>!/^\d{1,3}$/.test(p)||String(Number(p))!==p||Number(p)>255))return null;
   if(Number(parts[0])>=224||text==='255.255.255.255')return null;
   return {value:text,family:4,wildcard:text==='0.0.0.0',loopback:parts[0]==='127',scoped:false};
  }
  if(text.length>253||!text.split('.').every(label=>/^[a-z\d](?:[a-z\d-]{0,61}[a-z\d])?$/i.test(label))||/^(?:\d+|0x[\da-f]*)$/i.test(text.split('.').at(-1)))return null;
  return {value:text.toLowerCase(),family:0,wildcard:false,loopback:text.toLowerCase()==='localhost',scoped:false};
 }
 function remotePath(value,server=false){const text=clean(value);if(text===null||text.includes('\\')||/^[a-z]:/i.test(text)||text.split('/').some(p=>p==='.'||p==='..')||server&&text.startsWith('/'))return null;return text;}
 function localPath(value){const text=clean(value);return text&&(/^[a-z]:[\\/]/i.test(text)||text.startsWith('/')||/^\\\\[^\\/]+[\\/][^\\/]+/.test(text))?text:null;}
 function joinLocal(base,relative){const folder=localPath(base),path=remotePath(relative,true);if(!folder||path===null)return null;if(!path)return folder;return folder.replace(/[\\/]$/,'')+(folder.includes('\\')?'\\':'/')+path.split('/').join(folder.includes('\\')?'\\':'/');}
 function capture(input){
  const scope=['server','remote','local'].includes(input?.scope)?input.scope:'remote',protocol=protocols.has(input?.protocol)?input.protocol:'',port=Number(input?.port);
  if(scope!=='local'&&(!protocol||!Number.isInteger(port)||port<1||port>65535))throw Error('The transfer protocol or port is unavailable.');
  const endpoint=scope==='local'?null:host(input.host);if(scope!=='local'&&!endpoint)throw Error('The server address is unavailable or invalid.');
  if(endpoint?.linkLocal&&!endpoint.scoped)throw Error('An IPv6 link-local address needs its explicit interface scope (%zone) before it can be copied as a destination.');
  const path=input?.path==null?null:remotePath(input.path,scope==='server');if(input?.path!=null&&path===null)throw Error('This path cannot be represented safely. Copy a valid server-relative or remote path.');
  const local=input?.localPath==null?null:localPath(input.localPath);if(input?.localPath!=null&&!local)throw Error('The local disk path is unavailable or invalid.');
  const security=protocol==='ftp'&&['plain','explicit','implicit'].includes(input.security)?input.security:protocol==='ftp'?'explicit':'';
  const candidates=[];for(const item of Array.isArray(input?.candidates)?input.candidates:[]){const parsed=host(item?.host);if(!parsed||parsed.wildcard||parsed.loopback||parsed.linkLocal&&!parsed.scoped||!parsed.family||endpoint?.family&&parsed.family!==endpoint.family||candidates.some(c=>c.host===parsed.value))continue;candidates.push({host:parsed.value,label:clean(item.label,300)||parsed.value});if(candidates.length===256)break;}
  return Object.freeze({scope,protocol,security,port:scope==='local'?0:port,host:endpoint?.value||'',wildcard:!!endpoint?.wildcard,loopback:!!endpoint?.loopback,path,localPath:local,running:input?.running===true,candidates:Object.freeze(candidates.map(Object.freeze))});
 }
 function values(snapshot,selected=''){
  const list=[],add=(id,label,value)=>{if(value!==null&&value!==undefined)list.push({id,label,value});};
  if(snapshot.localPath)add('local','Local disk path',snapshot.localPath);
  if(snapshot.path!==null&&snapshot.path!=='')add('path',snapshot.scope==='server'?'Server-relative path / filename':'Remote path / filename',snapshot.path);
  if(snapshot.scope==='local')return list;
  const value=snapshot.wildcard?snapshot.candidates.find(c=>c.host===selected)?.host:snapshot.host,parsed=host(value);if(!parsed||parsed.wildcard||parsed.linkLocal&&!parsed.scoped)return list;
  const authority=parsed.family===6?'['+parsed.value+']':parsed.value,defaultPort=snapshot.protocol==='sftp'?22:snapshot.protocol==='tftp'?69:snapshot.security==='implicit'?990:21,endpoint=authority+':'+snapshot.port;
  add('host',snapshot.scope==='server'?'Server IP / hostname':'Remote host',parsed.value);add('endpoint',snapshot.scope==='server'?'Server endpoint (port included)':'Remote endpoint (port included)',endpoint);
  if(snapshot.path!==null){const path=snapshot.protocol==='tftp'?snapshot.path:snapshot.path.replace(/^\//,'');add('network','Network location',(snapshot.port===defaultPort?authority:endpoint)+'/'+path);
   // FTP URLs can interpret /path relative to the login directory (RFC 1738
   // section 3.2.2). Do not guess an absolute remote FTP path's URL semantics.
   // FTPS modes, relative SFTP paths and scoped IPv6 also lack a portable URL.
   if(!parsed.scoped&&(snapshot.protocol!=='ftp'||snapshot.security==='plain'&&snapshot.scope==='server')&&(snapshot.protocol!=='sftp'||snapshot.scope==='server'||snapshot.path.startsWith('/'))){// RFC 3617 section 2 treats the whole TFTP filename as escaped data,
    // including nested or leading slashes: decoding must recover it exactly.
    const encode=part=>encodeURIComponent(part).replace(/[!'()*]/g,c=>'%'+c.charCodeAt(0).toString(16).toUpperCase()),encoded=snapshot.protocol==='tftp'?encode(path):path.split('/').map(encode).join('/');add('url','Protocol URL',snapshot.protocol+'://'+(snapshot.port===defaultPort?authority:endpoint)+'/'+encoded);}
  }
  const order=['network','url','path','local','host','endpoint'];return list.sort((a,b)=>order.indexOf(a.id)-order.indexOf(b.id));
 }
 function adapterCandidates(list){const result=[];for(const adapter of Array.isArray(list)?list:[]){if(adapter?.up!==true||adapter.loopback)continue;for(const item of adapter.addresses||[]){const raw=typeof item==='string'?item:typeof item?.address==='string'?item.address:'';const ip=host(raw.split('/')[0]);if(ip&&!ip.wildcard&&!ip.loopback&&!(ip.linkLocal&&!ip.scoped)&&ip.family)result.push({host:ip.value,label:(clean(adapter.description||adapter.name,250)||'Network adapter')+' · '+ip.value});}}return result;}
 let active=null,serial=0;
 function rowsHTML(context){return values(context.snapshot,context.selected).map(row=>'<div class="transfer-location-row"><div><strong>'+escape(row.label)+'</strong><div class="mono transfer-location-value" data-location-value="'+row.id+'" data-no-value-copy>'+escape(row.value||'(server root)')+'</div></div><button type="button" class="btn small ghost" data-transfer-location-copy="'+row.id+'" aria-label="Copy '+escape(row.label.toLowerCase())+'">Copy</button></div>').join('');}
 function show(input){
  let snapshot;try{snapshot=capture(input);}catch(error){toast(error.message,'error');return null;}
  const context={snapshot,selected:'',id:++serial};active=context;
  const description=snapshot.scope==='server'?(snapshot.running?'Listening server snapshot.':'Configured server snapshot — not listening.'):snapshot.scope==='remote'?'Remote location snapshot; copying does not connect or transfer.':'Local disk path; not a network share or remote filename.';
  const selection=snapshot.wildcard?'<label class="field">Network address to copy<select id="transferLocationAddress" data-transfer-location-address><option value="">Choose an active adapter address…</option>'+snapshot.candidates.map(c=>'<option value="'+escape(c.host)+'">'+escape(c.label)+'</option>').join('')+'</select></label><p class="help-line">The wildcard listener is not a device destination. Choose the interface your equipment can reach; firewall and routing availability are not verified.'+(!snapshot.candidates.length?' No suitable active adapter address is available.':'')+'</p>':'';
  const mode=snapshot.protocol==='ftp'?'FTP · '+({plain:'Plain FTP (unencrypted)',explicit:'Explicit FTPS (TLS)',implicit:'Implicit FTPS (TLS)'}[snapshot.security]):snapshot.protocol.toUpperCase();
  openModal('Copy transfer location','<div class="transfer-locations" data-transfer-location-dialog="'+context.id+'"><p>'+escape(description)+'</p>'+(mode?'<p class="help-line">'+escape(mode)+(snapshot.protocol==='ftp'&&snapshot.security!=='plain'?'. Configure this TLS mode in the client; no ambiguous FTPS URL is offered.':snapshot.protocol==='ftp'&&snapshot.scope==='remote'?'. Remote FTP URL path rules vary; use the exact remote path with your client.':'')+'</p>':'')+(snapshot.loopback?'<div class="notice warn">Loopback address: usable on this computer only, not from another device.</div>':'')+selection+'<div id="transferLocationValues">'+rowsHTML(context)+'</div><p class="help-line">Network locations and remote filenames are not Windows UNC shares or local disk paths. URLs encode filename characters; literal paths do not. No username, password, key or token is included.</p></div>','<button type="button" class="btn ghost" data-action="close-modal">Close</button>',true);return snapshot;
 }
 document.addEventListener('change',event=>{if(!event.target.matches?.('[data-transfer-location-address]')||!active)return;active.selected=event.target.value;const container=document.getElementById('transferLocationValues');if(container)container.innerHTML=rowsHTML(active);},true);
 document.addEventListener('click',event=>{const button=event.target.closest?.('[data-transfer-location-copy]');if(!button)return;event.preventDefault();event.stopImmediatePropagation();const context=active;if(!context||button.closest('[data-transfer-location-dialog]')?.dataset.transferLocationDialog!==String(context.id))return;const value=values(context.snapshot,context.selected).find(row=>row.id===button.dataset.transferLocationCopy);if(value)void copyText(value.value);},true);
 root.NetworkToolkitTransferLocations=Object.freeze({host,remotePath,localPath,joinLocal,capture,values,adapterCandidates,show});
})(globalThis);
