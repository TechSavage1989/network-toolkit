/* Display-only syntax for standard PowerShell and cmd prompts. Never reads
 * keyboard events or writes to the session. ANSI colors from the shell remain
 * authoritative; xterm's 16 configured slots supply fallback syntax colors. */
(function(root){'use strict';
const roles={command:'Command',keyword:'Keyword',parameter:'Parameter / switch',variable:'Variable',string:'String',operator:'Operator',number:'Number',type:'Type',comment:'Comment'};
const psKeywords=new Set('begin break catch class continue data define do dynamicparam else elseif end enum exit filter finally for foreach from function if in inlinescript parallel param process return sequence switch throw trap try until using var while workflow'.split(' '));
const cmdKeywords=new Set('if else for in do not exist defined errorlevel cmdextversion equ neq lss leq gtr geq call set setlocal endlocal echo goto exit shift title prompt color chcp cd chdir dir copy del erase move ren rename type start pushd popd path pause cls assoc ftype verify vol ver md mkdir rd rmdir rem'.split(' '));
const psOperators=/^-(?:[ic]?(?:eq|ne|ge|gt|le|lt|notcontains|contains|notin|in|notlike|like|notmatch|match|replace|split)|bnot|band|bor|bxor|and|or|xor|not|as|isnot|is|join|f|shl|shr)\b/i;
function supported(language){return language==='powershell'||language==='cmd';}
function palette(theme={},mode='dark'){
 const light=mode==='light',fallback={foreground:light?'#20324c':'#dce6f6',yellow:light?'#916400':'#eec16f',brightYellow:light?'#a87400':'#ffda91',green:light?'#17734a':'#6aca9e',brightGreen:light?'#208d5e':'#8ce2b6',cyan:light?'#07847e':'#67cdd1',brightBlack:light?'#6c7f96':'#73829a',blue:light?'#2465bc':'#7baaff'};
 const c={...fallback,...Object.fromEntries(Object.entries(theme).filter(([,v])=>typeof v==='string'&&/^#[a-f\d]{6}$/i.test(v)))};
 return {'shell-command':c.brightYellow,'shell-keyword':c.brightGreen,'shell-parameter':c.brightBlack,'shell-variable':c.brightGreen,'shell-string':c.cyan,'shell-operator':c.foreground,'shell-number':c.foreground,'shell-type':c.cyan,'shell-comment':c.green};
}
function legend(theme,mode){const colors=palette(theme,mode);return Object.entries(roles).map(([role,label])=>({kind:'shell-'+role,label,color:colors['shell-'+role],borderStyle:'none',borderWidth:0}));}
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
   const quote=ch;i++;
   while(i<text.length){if(text[i]===(powershell?'`':'^')&&quote!=="'"){i+=2;continue;}if(text[i]===quote){if(powershell&&quote==="'"&&text[i+1]==="'"){i+=2;continue;}i++;break;}i++;}
   add(start,Math.min(i,text.length),'string');command=false;continue;
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
root.NTKTerminalShellSyntax=Object.freeze({analyze,supported,palette,legend});
})(globalThis);
