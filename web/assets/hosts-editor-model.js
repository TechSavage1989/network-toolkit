/* Hosts-file draft operations. No file writes or network requests. */
(function(root,factory){
 'use strict';
 const api=factory();
 if(typeof module==='object'&&module.exports)module.exports=api;
 if(root)root.NetworkToolkitHostsModel=api;
})(typeof globalThis==='object'?globalThis:this,function(){
 'use strict';
 const LIMIT=1024*1024;
 const normalize=text=>String(text??'').replace(/\r\n?|\n/g,'\n');
 const key=name=>String(name).replace(/\.$/,'').toLowerCase();
 function address(value){
  const text=String(value||'');
  if(/^(?:0|[1-9]\d{0,2})(?:\.(?:0|[1-9]\d{0,2})){3}$/.test(text)&&text.split('.').every(x=>Number(x)<=255))return {family:4,canonical:text};
  if(!text.includes(':')||!/^[0-9a-f:.]+$/i.test(text))return null;
  if(text.includes('.')&&!address(text.slice(text.lastIndexOf(':')+1)))return null;
  try{const hostname=new URL('http://['+text+']/').hostname;if(hostname.startsWith('['))return {family:6,canonical:hostname.slice(1,-1).toLowerCase()};}catch{}
  return null;
 }
 function hostname(value){
  const name=String(value||'').replace(/\.$/,'');
  return !!name&&name.length<=253&&name.split('.').every(label=>label.length<=63&&/^[A-Za-z0-9_](?:[A-Za-z0-9_-]*[A-Za-z0-9_])?$/.test(label));
 }
 function parse(text){
  const source=normalize(text),lines=source.split('\n'),rows=[],issues=[],seen=new Map(),related=new Set();
  let errors=0,warnings=0,enabled=0,disabled=0;
  function issue(line,severity,code,message,otherLine){
   if(severity==='error')errors++;else warnings++;
   related.add(line);if(otherLine)related.add(otherLine);
   if(issues.length<200)issues.push({line,severity,code,message,otherLine});
  }
  if(new TextEncoder().encode(source).length>LIMIT)issue(1,'error','size','The hosts file exceeds the 1 MiB editing limit.');
  lines.forEach((raw,index)=>{
   const line=index+1,indent=raw.match(/^[ \t]*/)[0],trimmed=raw.trimStart();
   const controls=/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/.test(raw);
   if(controls)issue(line,'error','control','Unsupported control character.');
   if(!trimmed)return;
   const commented=trimmed.startsWith('#'),body=commented?trimmed.slice(1).trimStart():trimmed;
   const hash=body.indexOf('#'),content=(hash<0?body:body.slice(0,hash)).trim(),comment=hash<0?'':body.slice(hash+1).trim();
   const fields=content.split(/\s+/),ip=address(fields[0]),hosts=fields.slice(1),valid=!!ip&&hosts.length>0&&hosts.every(hostname);
   // Ordinary comments are never interpreted as mappings or rewritten.
   if(commented&&!valid)return;
   const row={line,index,raw,indent,enabled:!commented,address:fields[0]||'',hosts,comment,valid,family:ip?.family||0,canonical:ip?.canonical||''};
   rows.push(row);row.enabled?enabled++:disabled++;
   if(!row.enabled)return;
   if(controls){row.valid=false;return;}
   if(!ip){issue(line,'error','address','Enter a valid IPv4 or IPv6 address without a prefix or zone.');return;}
   if(!hosts.length){issue(line,'error','hostname','An address needs at least one hostname.');return;}
   for(const name of hosts){
    if(!hostname(name)){issue(line,'error','hostname','Invalid hostname: '+name+'. Use an ASCII hostname or punycode.');continue;}
    const normalized=key(name),same=seen.get(normalized)||{addresses:new Map(),families:new Map()},signature=ip.family+'|'+ip.canonical;
    const duplicate=same.addresses.get(signature);
    const first=same.families.get(ip.family),conflict=first&&first.address!==ip.canonical?first:null;
    if(duplicate)issue(line,'warning','duplicate',name+' duplicates its mapping on line '+duplicate.line+'.',duplicate.line);
    else if(conflict)issue(line,'warning','conflict',name+' has another IPv'+ip.family+' address on line '+conflict.line+'.',conflict.line);
    const entry={address:ip.canonical,family:ip.family,line};
    if(!duplicate)same.addresses.set(signature,entry);
    if(!first)same.families.set(ip.family,entry);
    seen.set(normalized,same);
   }
  });
  return {source,lines,rows,issues,errors,warnings,enabled,disabled,related};
 }
 function lineText(entry){
  const ip=String(entry.address||'').trim(),hosts=Array.isArray(entry.hosts)?entry.hosts:String(entry.hosts||'').split(/[\s,;]+/).filter(Boolean);
  if(!address(ip))throw new Error('Enter a valid IPv4 or IPv6 address without a prefix or zone.');
  if(!hosts.length||hosts.some(name=>!hostname(name)))throw new Error('Enter one or more valid ASCII hostnames or aliases.');
  const comment=String(entry.comment||'').trim();
  if(/[\r\n\x00-\x1f\x7f]/.test(comment))throw new Error('Comments must fit on one line and contain no control characters.');
  return String(entry.indent||'').replace(/[^ \t]/g,'')+(entry.enabled===false?'# ':'')+ip+'\t'+hosts.join(' ')+(comment?'\t# '+comment:'');
 }
 function replaceLine(text,line,entry){
  const doc=parse(text),row=doc.rows.find(row=>row.line===Number(line));
  if(!row)throw new Error('This entry changed. Select it again.');
  doc.lines[row.index]=lineText({...entry,indent:row.indent});return doc.lines.join('\n');
 }
 function append(text,entry){
  const source=normalize(text),line=lineText(entry);
  return source+(source&&!source.endsWith('\n')?'\n':'')+line+'\n';
 }
 function selectedRows(doc,selection){
  const lines=new Set(Array.from(selection||[],Number));return doc.rows.filter(row=>lines.has(row.line));
 }
 function toggle(text,selection,enabled){
  const doc=parse(text);
  for(const row of selectedRows(doc,selection)){
   if(enabled&&!row.valid)throw new Error('Correct invalid entries before enabling them.');
   if(row.enabled===enabled)continue;
   doc.lines[row.index]=enabled?row.raw.replace(/^(\s*)#\s?/,'$1'):row.indent+'# '+row.raw.slice(row.indent.length);
  }
  return doc.lines.join('\n');
 }
 function remove(text,selection){
  const doc=parse(text),indices=new Set(selectedRows(doc,selection).map(row=>row.index));
  return doc.lines.filter((_,index)=>!indices.has(index)).join('\n');
 }
 function format(text,selection){
  const doc=parse(text),rows=selection?.size?selectedRows(doc,selection):doc.rows;
  for(const row of rows)if(row.valid)doc.lines[row.index]=lineText(row);
  return doc.lines.join('\n');
 }
 function sort(text){
  const doc=parse(text),byLine=new Map(doc.rows.filter(row=>row.valid).map(row=>[row.index,row]));
  let group=[];
  const flush=()=>{
   const ordered=group.map(index=>byLine.get(index)).sort((a,b)=>key(a.hosts[0]).localeCompare(key(b.hosts[0]),undefined,{numeric:true})||a.address.localeCompare(b.address,undefined,{numeric:true}));
   group.forEach((index,n)=>{doc.lines[index]=ordered[n].raw;});group=[];
  };
  doc.lines.forEach((_,index)=>{if(byLine.has(index))group.push(index);else flush();});flush();
  return doc.lines.join('\n');
 }
 function deduplicate(text){
  const doc=parse(text),seen=new Set(),removed=new Set();
  for(const row of doc.rows){
   if(!row.valid||!row.enabled)continue;
   // Different comments and alias order may carry intent: retain them.
   const signature=JSON.stringify([row.canonical,row.hosts.map(key),row.comment]);
   if(seen.has(signature))removed.add(row.index);else seen.add(signature);
  }
  return {text:doc.lines.filter((_,index)=>!removed.has(index)).join('\n'),count:removed.size};
 }
 function matches(text,query,matchCase=false,whole=false){
  if(!query)return [];
  const source=normalize(text),out=[],literal=String(query).replace(/[.*+?^${}()|[\]\\]/g,'\\$&'),pattern=new RegExp(literal,matchCase?'gu':'giu');
  for(const match of source.matchAll(pattern)){
   const at=match.index,end=at+match[0].length;
   if(!whole||(!/[A-Za-z0-9_.-]/.test(source[at-1]||'')&&!/[A-Za-z0-9_.-]/.test(source[end]||'')))out.push({start:at,end});
  }
  return out;
 }
 function replaceAll(text,query,replacement,matchCase=false,whole=false){
  const source=normalize(text),found=matches(source,query,matchCase,whole);let output='',end=0;
  for(const match of found){output+=source.slice(end,match.start)+replacement;end=match.end;}
  return {text:output+source.slice(end),count:found.length};
 }
 function diff(before,after){
  const a=normalize(before).split('\n'),b=normalize(after).split('\n');let start=0,endA=a.length,endB=b.length;
  while(start<endA&&start<endB&&a[start]===b[start])start++;
  while(endA>start&&endB>start&&a[endA-1]===b[endB-1]){endA--;endB--;}
  return {start:start+1,removed:a.slice(start,endA),added:b.slice(start,endB)};
 }
 return Object.freeze({LIMIT,normalize,address,hostname,parse,lineText,replaceLine,append,toggle,remove,format,sort,deduplicate,matches,replaceAll,diff});
});
