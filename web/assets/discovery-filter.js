/* Address-list filtering only. Delimited entries are alternatives, not scan targets. */
(function(root){
 'use strict';
 const MAX_QUERY_LENGTH=32768;
 const singleLine=value=>String(value??'').replace(/\r\n|[\r\n\u2028\u2029]/g,' ');
 function terms(value){
  const text=String(value??''),result=[],seen=new Set();let item='',quoted=false;
  const add=()=>{const value=item.trim().replace(/\s+/g,' ').toLowerCase();if(value&&!seen.has(value)){seen.add(value);result.push(value);}item='';};
  for(const char of text){if(char==='"'){quoted=!quoted;continue;}if(!quoted&&/[\s,;]/u.test(char))add();else item+=char;}add();return result;
 }
 function mac(value){
  const text=String(value??'').trim().toLowerCase();
  return /^(?:[\da-f]{12}|(?:[\da-f]{2}:){5}[\da-f]{2}|(?:[\da-f]{2}-){5}[\da-f]{2}|(?:[\da-f]{4}\.){2}[\da-f]{4})$/.test(text)?text.replace(/[:.-]/g,''):'';
 }
 function ip(value){
  let text=String(value??'').trim().toLowerCase();if(text.startsWith('[')&&text.endsWith(']'))text=text.slice(1,-1);
  if(/^(?:\d{1,3}\.){3}\d{1,3}$/.test(text)){const parts=text.split('.').map(Number);return parts.every(n=>n<=255)?'4:'+parts.join('.'):'';}
  if(!text.includes(':'))return '';
  const zoneAt=text.indexOf('%'),zone=zoneAt<0?'':text.slice(zoneAt);if(zoneAt>=0){text=text.slice(0,zoneAt);if(!/^%[\w.-]+$/.test(zone))return '';}
  if(text.includes('.')){const at=text.lastIndexOf(':'),tail=ip(text.slice(at+1));if(!tail.startsWith('4:'))return '';const p=tail.slice(2).split('.').map(Number);text=text.slice(0,at+1)+(p[0]*256+p[1]).toString(16)+':'+(p[2]*256+p[3]).toString(16);}
  if(!/^[\da-f:]+$/.test(text)||text.split('::').length>2)return '';
  const parts=text.split('::'),left=parts[0]?parts[0].split(':'):[],right=parts.length===2&&parts[1]?parts[1].split(':'):[],missing=8-left.length-right.length;
  if([...left,...right].some(p=>!p||p.length>4)||parts.length===1&&missing!==0||parts.length===2&&missing<1)return '';
  return '6:'+left.concat(parts.length===2?Array(missing).fill('0'):[],right).map(p=>parseInt(p,16).toString(16)).join(':')+zone;
 }
 function compile(value){
  const alternatives=terms(value),addresses=new Set(),macs=new Set(),text=[];
  for(const term of alternatives){const m=mac(term),address=ip(term);if(m)macs.add(m);else if(address)addresses.add(address);else text.push(term);}
  return Object.freeze({count:alternatives.length,matches(fields){
   if(!alternatives.length)return true;
   const values=(Array.isArray(fields)?fields:[fields]).filter(v=>v!=null).map(String),haystack=values.join(' ').toLowerCase().replace(/\s+/g,' ');
   if(text.some(term=>haystack.includes(term)))return true;
   if(!addresses.size&&!macs.size)return false;
   for(const value of values){
    // Compare complete address tokens, never an IP prefix such as .1 inside .10.
    const candidates=[value,...value.split(/[\s,;()[\]<>"'=]+/)];
    for(const candidate of candidates){if(macs.size&&macs.has(mac(candidate)))return true;if(addresses.size&&addresses.has(ip(candidate)))return true;}
   }
   return false;
  }});
 }
 function handlePaste(event){
  const input=event.target;if(input?.dataset?.networkFilter!=='query'||!event.clipboardData)return false;
  const text=event.clipboardData.getData('text/plain');if(!text)return false;
  const before=String(input.value||''),start=input.selectionStart??before.length,end=input.selectionEnd??start,insert=singleLine(text),next=before.slice(0,start)+insert+before.slice(end);
  event.preventDefault();
  if(next.length>MAX_QUERY_LENGTH){if(typeof toast==='function')toast('Filter list is too long. Use no more than 32,768 characters.','warn');return true;}
  input.setRangeText(insert,start,end,'end');
  input.dispatchEvent(new root.Event('input',{bubbles:true}));return true;
 }
 const api=Object.freeze({MAX_QUERY_LENGTH,singleLine,terms,mac,ip,compile,handlePaste});
 if(typeof module==='object'&&module.exports)module.exports=api;root.NetworkToolkitDiscoveryFilter=api;
 root.document?.addEventListener?.('paste',handlePaste,true);
})(typeof window!=='undefined'?window:globalThis);
