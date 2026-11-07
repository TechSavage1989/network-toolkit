/* Paste warnings modeled on MobaXterm (T3b, v1.21.9). check(text) lists risky
   patterns in text about to be pasted into a terminal; it never blocks or
   changes the paste. Each kind can be turned off in Mouse and keyboard defaults. */
(function(root){
'use strict';
const KINDS=Object.freeze([
 {id:'homograph',label:'Look-alike, invisible or typographic characters'},
 {id:'pipeToShell',label:'Commands that download and run a script (curl … | sh)'},
 {id:'shellProfile',label:'Changes to shell startup files (.bashrc, $PROFILE)'},
 {id:'systemArchive',label:'Archives extracted into system folders'},
 {id:'link',label:'Plain-HTTP or shortened links'}
]);
const IDS=KINDS.map(kind=>kind.id);
// Bounded scan: a 1 MiB paste is checked line by line, never with one huge match.
const LIMITS=Object.freeze({lines:4000,lineChars:8192,sample:60});
const INVISIBLE=/[­؜᠎​-‏‪-‮⁠-⁤⁦-⁩﻿]/u;
const TYPOGRAPHIC=/[‘’‚‛“”„‟′″‐-―−﹣－]/u;
const LATIN=/\p{Script=Latin}/u, LOOKALIKE=/[\p{Script=Cyrillic}\p{Script=Greek}\p{Script=Armenian}]/u;
const FETCH='(?:curl|wget|fetch|iwr|irm|invoke-webrequest|invoke-restmethod)';
const INTERPRETER='(?:(?:ba|da|z|k|c|tc|fi|a)?sh|python[0-9.]*|perl|ruby|node|php|lua|iex|invoke-expression|powershell|pwsh)';
// The interpreter must end the word ("| sh-check" is not a shell), and the pipe
// must come from the download command itself, not a later one after ; or &.
const END='(?=$|[\\s;&|)"\'])';
const PIPE=new RegExp('\\b'+FETCH+'\\b[^|;&\\n]{0,2000}\\|\\s*(?:sudo\\s+(?:-\\S+\\s+){0,4})?(?:env\\s+)?(?:[\\w/.-]{0,80}/)?'+INTERPRETER+END,'i');
const SUBSTITUTE=new RegExp('\\b'+INTERPRETER+END+'[^\\n]{0,200}(?:<\\(|\\$\\(|`)\\s*'+FETCH+'\\b','i');
const POWERSHELL=/\b(?:iex|invoke-expression)\b[^\n]{0,200}\b(?:downloadstring|iwr|irm|invoke-webrequest|invoke-restmethod)\b/i;
const PROFILE=/(?:^|[\s"'=:/])(?:~\/|\$HOME\/|\$\{HOME\}\/|\/root\/|\/home\/[^/\s]+\/)?\.(?:bashrc|bash_profile|bash_login|profile|zshrc|zprofile|zshenv|zlogin|kshrc|cshrc|tcshrc|config\/fish\/config\.fish)\b|\/etc\/(?:profile(?:\.d\/\S*)?|bash\.bashrc|bashrc|zshrc|zsh\/zshrc|environment)\b|\$profile\b/i;
const WRITES=/>>?|\btee\b|\bsed\b[^\n]{0,200}\s-i|\b(?:add|set)-content\b|\bout-file\b|\b(?:cp|mv|install|ln)\s/i;
const EXTRACT=/\btar\b(?=[^\n]{0,2000}(?:\s-?[A-Za-z]{0,8}x[A-Za-z]{0,8}(?:\s|$)|--extract\b|--get\b))|\bunzip\b|\b7za?\s+x\b|\bexpand-archive\b|\bcpio\b[^\n]{0,200}\s-[a-z]{0,6}i/i;
const SYSTEM_DIR=/(?:\s-C\s*|--directory[=\s]+|\s-d\s+|\s-o|-destinationpath\s+)["']?(?:\/(?:etc|usr|bin|sbin|lib|lib32|lib64|boot|sys|root)?(?=[/"'\s]|$)|[a-z]:\\(?:windows|program files)\b)/i;
const URL=/\bhttps?:\/\/[^\s"'<>`]+/gi;
const SHORTENERS=new Set(['bit.ly','tinyurl.com','t.co','goo.gl','ow.ly','is.gd','buff.ly','rebrand.ly','cutt.ly','shorturl.at','tiny.cc','rb.gy','t.ly','s.id','v.gd','lnkd.in']);
function sample(text){const value=String(text).trim();return Array.from(value).length>LIMITS.sample?Array.from(value).slice(0,LIMITS.sample-1).join('')+'…':value;}
function internalHost(host){
 const h=host.toLowerCase().replace(/^\[|\]$/g,'').replace(/\.$/,'');
 if(!h.includes('.')&&!h.includes(':'))return true;
 if(h==='localhost'||/\.(?:local|lan|internal|home\.arpa|localdomain)$/.test(h))return true;
 const v4=h.match(/^(\d{1,3})\.(\d{1,3})\.(\d{1,3})\.(\d{1,3})$/);
 if(v4){const [a,b]=[Number(v4[1]),Number(v4[2])];return a===10||a===127||a===192&&b===168||a===172&&b>=16&&b<=31||a===169&&b===254||a===100&&b>=64&&b<=127;}
 return h==='::1'||/^f[cd][0-9a-f]{0,2}:/.test(h)||/^fe[89ab][0-9a-f]?:/.test(h);
}
function enabledKinds(enabled){const e=enabled&&typeof enabled==='object'?enabled:{};return IDS.filter(id=>e[id]!==false);}
function check(text,enabled){
 if(typeof text!=='string'||!text)return [];
 const kinds=new Set(enabledKinds(enabled)),found=new Map(),add=(kind,key,message,value)=>{if(kinds.has(kind)&&!found.has(key))found.set(key,{kind,message,sample:value===undefined?'':sample(value)});};
 const lines=text.split(/\r\n|\r|\n/).slice(0,LIMITS.lines);
 for(const full of lines){
  const line=full.slice(0,LIMITS.lineChars);
  if(kinds.has('homograph')){
   const invisible=line.match(INVISIBLE);if(invisible)add('homograph','invisible','Invisible or text-direction characters are hidden in the text (U+'+invisible[0].codePointAt(0).toString(16).toUpperCase().padStart(4,'0')+').');
   if(TYPOGRAPHIC.test(line))add('homograph','typographic','Typographic quotes or dashes (“ ” ‘ ’ – —) are not ordinary quotes or hyphens to a shell or device.',line);
   if(LOOKALIKE.test(line))for(const word of line.split(/[^\p{L}\p{N}_.-]+/u))if(LATIN.test(word)&&LOOKALIKE.test(word)){add('homograph','mixed','“'+sample(word)+'” mixes Latin letters with look-alike Cyrillic, Greek or Armenian letters.');break;}
  }
  if(kinds.has('pipeToShell')&&(PIPE.test(line)||SUBSTITUTE.test(line)||POWERSHELL.test(line)))add('pipeToShell','pipe','Downloads a script and runs it at once; it runs whatever the server sends.',line);
  if(kinds.has('shellProfile')&&PROFILE.test(line)&&WRITES.test(line))add('shellProfile','profile','Changes a shell startup file, which then runs every time a shell starts.',line);
  if(kinds.has('systemArchive')&&EXTRACT.test(line)&&SYSTEM_DIR.test(line))add('systemArchive','archive','Extracts an archive into a system folder, which can replace system files.',line);
  if(kinds.has('link'))for(const url of line.match(URL)||[]){
   const host=url.match(/^https?:\/\/(?:[^@/\s]*@)?(\[[^\]]+\]|[^/:?#\s]+)/i)?.[1]||'';
   if(SHORTENERS.has(host.toLowerCase().replace(/^www\./,'')))add('link','short','Shortened link: you cannot see where it leads.',url);
   else if(/^http:/i.test(url)&&!internalHost(host))add('link','http','Plain HTTP link: what it sends or receives can be read or changed on the way.',url);
  }
  if(found.size>=7)break;
 }
 return [...found.values()];
}
root.NetworkToolkitPasteWarnings=Object.freeze({kinds:KINDS,ids:IDS,limits:LIMITS,check,internalHost});
if(typeof module==='object'&&module.exports)module.exports=root.NetworkToolkitPasteWarnings;
})(typeof window!=='undefined'?window:globalThis);
