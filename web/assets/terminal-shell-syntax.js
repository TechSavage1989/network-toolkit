/* Display-only syntax for standard PowerShell and cmd prompts. Never reads
 * keyboard events or writes to the session. ANSI colors from the shell remain
 * authoritative; xterm's 16 configured slots supply fallback syntax colors. */
(function(root){'use strict';
const roles={command:'Command',keyword:'Keyword',parameter:'Parameter / switch',variable:'Variable',string:'String',operator:'Operator',number:'Number',type:'Type',comment:'Comment',outputHeading:'Output heading',outputField:'Output field',outputValue:'Output value',outputAddress:'Output address',outputSuccess:'Output success text',outputWarning:'Output warning text',outputError:'Output error text'};
const psKeywords=new Set('begin break catch class continue data define do dynamicparam else elseif end enum exit filter finally for foreach from function if in inlinescript parallel param process return sequence switch throw trap try until using var while workflow'.split(' '));
const cmdKeywords=new Set('if else for in do not exist defined errorlevel cmdextversion equ neq lss leq gtr geq call set setlocal endlocal echo goto exit shift title prompt color chcp cd chdir dir copy del erase move ren rename type start pushd popd path pause cls assoc ftype verify vol ver md mkdir rd rmdir rem'.split(' '));
const psOperators=/^-(?:[ic]?(?:eq|ne|ge|gt|le|lt|notcontains|contains|notin|in|notlike|like|notmatch|match|replace|split)|bnot|band|bor|bxor|and|or|xor|not|as|isnot|is|join|f|shl|shr)\b/i;
function supported(language){return language==='powershell'||language==='cmd';}
function normalizePreferences(value){const out={};for(const type of ['powershell','cmd']){const source=value&&typeof value==='object'&&!Array.isArray(value)?value[type]:null,colors={};if(source&&typeof source==='object'&&!Array.isArray(source))for(const role of Object.keys(roles))if(typeof source.colors?.[role]==='string'&&/^#[a-f\d]{6}$/i.test(source.colors[role]))colors[role]=source.colors[role].toLowerCase();out[type]={enabled:source?.enabled!==false,colors};}return out;}
function palette(theme={},mode='dark',overrides={}){
 const light=mode==='light',fallback={foreground:light?'#20324c':'#dce6f6',yellow:light?'#916400':'#eec16f',brightYellow:light?'#a87400':'#ffda91',green:light?'#17734a':'#6aca9e',brightGreen:light?'#208d5e':'#8ce2b6',cyan:light?'#07847e':'#67cdd1',brightBlack:light?'#6c7f96':'#73829a',blue:light?'#2465bc':'#7baaff'};
 const c={...fallback,...Object.fromEntries(Object.entries(theme).filter(([,v])=>typeof v==='string'&&/^#[a-f\d]{6}$/i.test(v)))};
 const result={'shell-outputHeading':c.brightYellow,'shell-outputField':c.cyan,'shell-outputValue':c.brightGreen,'shell-outputAddress':c.magenta||c.blue,'shell-outputSuccess':c.brightGreen,'shell-outputWarning':c.yellow,'shell-outputError':c.brightRed||c.red||'#ff7d88','shell-command':c.brightYellow,'shell-keyword':c.brightGreen,'shell-parameter':c.brightBlack,'shell-variable':c.brightGreen,'shell-string':c.cyan,'shell-operator':c.foreground,'shell-number':c.foreground,'shell-type':c.cyan,'shell-comment':c.green};
 for(const role of Object.keys(roles))if(typeof overrides[role]==='string'&&/^#[a-f\d]{6}$/i.test(overrides[role]))result['shell-'+role]=overrides[role].toLowerCase();return result;
}
function legend(theme,mode,overrides){const colors=palette(theme,mode,overrides);return Object.entries(roles).map(([role,label])=>({kind:'shell-'+role,label,color:colors['shell-'+role],borderStyle:'none',borderWidth:0}));}
function promptLength(text,language){
 const match=language==='powershell'?/^(?:\[[^\]\r\n]+\]:\s*)?PS [^\r\n]*?> ?/.exec(text):/^(?:(?:[A-Za-z]:\\|\\\\)[^>\r\n]*>|>) ?/.exec(text);
 return match?match[0].length:-1;
}
function analyze(text,language){
 if(!supported(language)||typeof text!=='string'||text.length>4096)return [];
 let i=promptLength(text,language);if(i<0)return [];
 const found=[],powershell=language==='powershell';let command=true;
 const add=(start,end,role)=>{if(end>start&&found.length<256)found.push({start,end,kind:'shell-'+role});};
 while(i<text.length&&found.length<256){const start=i,ch=text[i];
  if(/\s/.test(ch)){i++;continue;}
  if(powershell&&ch==='#'||!powershell&&command&&(text.slice(i,i+2)==='::'||/^rem(?:\s|$)/i.test(text.slice(i)))){add(i,text.length,'comment');break;}
  if(ch===(powershell?'`':'^')){i=Math.min(text.length,i+2);continue;}
  if(ch==='"'||powershell&&ch==="'"){
   const quote=ch;let closed=false;i++;
   while(i<text.length){if(text[i]===(powershell?'`':'^')&&quote!=="'"){i+=2;continue;}if(text[i]===quote){if(powershell&&quote==="'"&&text[i+1]==="'"){i+=2;continue;}i++;closed=true;break;}i++;}
   const end=Math.min(i,text.length);
   if(quote==='"'){
    const variables=powershell?/\$(?:\{[^}\r\n]*\}|[\w?:^$]+(?::[\w]+)?)/g:/(?:%%[A-Za-z]|%~[^%\s]*[0-9]|%[0-9*]|%[^%\r\n]+%|%[A-Za-z]|![^!\r\n]+!)/g;
    const inner=text.slice(start+1,end-(closed?1:0));let match,last=start;
    while((match=variables.exec(inner))){const at=start+1+match.index;let escapes=0;for(let back=at-1;back>start&&text[back]===(powershell?'`':'^');back--)escapes++;if(escapes%2)continue;add(last,at,'string');add(at,at+match[0].length,'variable');last=at+match[0].length;}
    add(last,end,'string');
   }else add(start,end,'string');command=false;continue;
  }
  let match;
  if(powershell&&(match=/^\$(?:\{[^}\r\n]*\}|[\w?:^$]+(?::[\w]+)?)/.exec(text.slice(i)))){i+=match[0].length;add(start,i,'variable');command=false;continue;}
  if(!powershell&&(match=/^(?:%%[A-Za-z]|%~[^%\s]*[0-9]|%[0-9*]|%[^%\r\n]+%|%[A-Za-z]|![^!\r\n]+!)/.exec(text.slice(i)))){i+=match[0].length;add(start,i,'variable');command=false;continue;}
  if(powershell&&(match=psOperators.exec(text.slice(i)))){i+=match[0].length;add(start,i,'operator');command=false;continue;}
  if(powershell&&(match=/^-{1,2}[A-Za-z][\w-]*/.exec(text.slice(i)))||!powershell&&(match=/^\/[A-Za-z?][\w:.-]*/.exec(text.slice(i)))){i+=match[0].length;add(start,i,'parameter');command=false;continue;}
  if(powershell&&(match=/^\[[A-Za-z_][\w.\[\],+]*\]/.exec(text.slice(i)))){i+=match[0].length;add(start,i,'type');continue;}
  if((match=/^(?:0x[\da-f]+|\d+(?:\.\d+)?(?:e[+-]?\d+)?(?:kb|mb|gb|tb|l|d)?)(?![\w.])/i.exec(text.slice(i)))){i+=match[0].length;add(start,i,'number');command=false;continue;}
  if(/[|&;=(){}<>+*/!,\[\]-]/.test(ch)){
   while(i<text.length&&/[|&;=(){}<>+*/!,\[\]-]/.test(text[i]))i++;
   add(start,i,'operator');command=/[|&;=({]/.test(text.slice(start,i));continue;
  }
  match=/^[^\s|&;=(){}<>+*!,\[\]"']+/.exec(text.slice(i));
  if(!match){i++;continue;}i+=match[0].length;
  const word=match[0].toLowerCase();
  if((powershell?psKeywords:cmdKeywords).has(word)){add(start,i,'keyword');command=/^(?:return|throw|do|else|call|start)$/.test(word);}
  else{if(command||powershell&&/^[a-z]+-[a-z][\w-]*$/i.test(match[0]))add(start,i,'command');command=false;}
 }
 return found;
}
function analyzeOutput(text,validators={}){
 if(typeof text!=='string'||!text||text.length>4096)return [];
 const found=[],add=(start,end,role)=>{if(end>start&&found.length<256&&!found.some(t=>start<t.end&&end>t.start))found.push({start,end,kind:'shell-'+role});};
 let m;const candidates=/[A-Za-z0-9_.:%/~-]+/g;
 while((m=candidates.exec(text))){const value=m[0].replace(/\.+$/,'');if(validators.address?.(value)||validators.mac?.(value))add(m.index,m.index+value.length,'outputAddress');}
 const statuses=/\b(?:Request timed out|Destination (?:host|net(?:work)?) unreachable|Access is denied|Permission denied|not recognized as (?:an internal|the name)|(?:FullyQualifiedErrorId|CategoryInfo)\s*:|Exception|ERROR|FAILED|WARNING|Media disconnected|Disconnected|Stopped|Running|Established|Reply from|Successfully)\b/gi;
 while((m=statuses.exec(text))){const word=m[0];add(m.index,m.index+word.length,/warning|disconnected|stopped/i.test(word)?'outputWarning':/running|established|reply from|successfully/i.test(word)?'outputSuccess':'outputError');}
 const field=/^(\s*)([A-Za-z][A-Za-z0-9 ._()\/-]{0,100}?)\s*:\s+(\S.*)$/.exec(text);
 if(field){const start=field[1].length,label=field[2].replace(/[.\s]+$/,'');add(start,start+label.length,'outputField');const at=text.length-field[3].length;let cursor=at;for(const token of [...found].sort((a,b)=>a.start-b.start)){if(token.end<=at)continue;if(cursor<token.start)add(cursor,token.start,'outputValue');cursor=Math.max(cursor,token.end);}if(cursor<text.length)add(cursor,text.length,'outputValue');}
 else if(/^\s*(?:Windows IP Configuration|(?:Ethernet|Wireless LAN|Unknown|Tunnel|PPP|Bluetooth Network Connection) adapter .+:|Directory(?: of|:) .+|(?:Status\s+Name\s+DisplayName|Mode\s+LastWriteTime\s+Length\s+Name|Handles\s+NPM\(K\)|Proto\s+Local Address|Image Name\s+PID|Name\s+InterfaceDescription|InterfaceAlias\s+InterfaceIndex).*)\s*$/i.test(text)){add(0,text.length,'outputHeading');}
 return found.sort((a,b)=>a.start-b.start);
}
root.NTKTerminalShellSyntax=Object.freeze({analyze,analyzeOutput,isPrompt:(text,language)=>promptLength(text,language)>=0,supported,palette,legend,normalizePreferences,roles:Object.freeze(roles)});
})(globalThis);
