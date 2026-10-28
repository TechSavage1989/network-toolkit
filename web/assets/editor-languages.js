/* Offline file recognition and editing conventions. Language names/grammars
 * come from Prism; common file associations follow the Notepad++ language list.
 * Recognition and syntax display never change the source file. */
(function(root){'use strict';
const overrides={markup:'Markup',html:'HTML',xml:'XML',bash:'Shell / Bash',batch:'Batch / Command Prompt',cpp:'C++',csharp:'C#',fsharp:'F#',objectivec:'Objective-C',jsx:'JavaScript (JSX)',tsx:'TypeScript (TSX)',jsonc:'JSON with comments',jsonlines:'JSON Lines',network:'Network config',text:'Plain text',registry:'Windows registry',vue:'Vue'};
const extra=[['html','HTML'],['xml','XML'],['jsonc','JSON with comments'],['jsonlines','JSON Lines'],['network','Network config'],['registry','Windows registry'],['vue','Vue'],['text','Plain text']];
const all=new Map((root.NTKEditorLanguageData||[]).map(row=>[row.id,{...row,label:overrides[row.id]||row.label}]));
for(const row of root.NTKNetworkConfig?.languages||[])all.set(row.id,row);
for(const [id,label] of extra)if(!all.has(id))all.set(id,{id,label,aliases:[]});
const languages=Object.freeze([...all.values()].sort((a,b)=>a.label.localeCompare(b.label,'en')));
const extensions={
 javascript:'js mjs cjs',jsx:'jsx',typescript:'ts mts cts',tsx:'tsx',json:'json webmanifest ipynb',json5:'json5',jsonc:'jsonc code-workspace',jsonlines:'jsonl ndjson',
 c:'c h',cpp:'cpp cxx cc c++ hpp hxx hh h++ ino',csharp:'cs csx',objectivec:'m mm',fsharp:'fs fsi fsx',java:'java',kotlin:'kt kts',scala:'scala sc',groovy:'groovy gvy gy gsh',gradle:'gradle',
 powershell:'ps1 psm1 psd1 pssc psrc',batch:'bat cmd',bash:'sh bash zsh ksh fish',python:'py pyw pyi pyx pxd',ruby:'rb rbw rake gemspec',perl:'pl pm pod',raku:'raku rakumod rakutest p6 pm6',php:'php php3 php4 php5 php7 phtml',lua:'lua',go:'go',rust:'rs',swift:'swift',dart:'dart',d:'d di',
 html:'html htm xhtml shtml',xml:'xml xsd xsl xslt svg csproj fsproj vbproj vcxproj props targets resx plist xaml nuspec manifest',vue:'vue',css:'css',scss:'scss',sass:'sass',less:'less',markdown:'md markdown mdown mkd',rest:'rst rest',asciidoc:'adoc asciidoc',latex:'tex latex sty cls',textile:'textile',wiki:'wiki',bbcode:'bbcode',
 aspnet:'aspx ascx asax ashx asmx',cshtml:'cshtml razor',ftl:'ftl',avro:'avdl',
 yaml:'yaml yml',toml:'toml',ini:'ini cfg conf cnf config',properties:'properties',systemd:'service socket timer mount target automount path',registry:'reg',editorconfig:'editorconfig',docker:'dockerfile',makefile:'mak mk makefile',cmake:'cmake',hcl:'tf tfvars hcl',nginx:'nginx',apacheconf:'htaccess',
 sql:'sql',plsql:'pls plsql pks pkb',graphql:'graphql gql',csv:'csv tsv',diff:'diff patch',log:'log err',network:'rsc cli ios aoscx procurve fortios routeros',
 actionscript:'as',ada:'ada adb ads',asn1:'asn asn1',nasm:'asm inc',armasm:'s',autoit:'au3',autohotkey:'ahk',avisynth:'avs avsi',basic:'bas',purebasic:'pb pbi',visualbasic:'vb vbs vba',vbnet:'vb',ocaml:'ml mli',coffeescript:'coffee litcoffee',cobol:'cob cbl cpy',erlang:'erl hrl',fortran:'f for f77 f90 f95 f03 f08',haskell:'hs lhs',lisp:'lisp lsp cl el',scheme:'scm ss',smalltalk:'st',matlab:'matlab',nim:'nim nims nimble',nsis:'nsi nsh',pascal:'pas pp lpr',prolog:'pro prolog',r:'r rmd',sas:'sas',tcl:'tcl tk',verilog:'v vh',vhdl:'vhd vhdl',
 clojure:'clj cljs cljc edn',elixir:'ex exs',elm:'elm',julia:'jl',nix:'nix',zig:'zig',protobuf:'proto',bicep:'bicep',gdscript:'gd',godot:'gdshader',haxe:'hx hxml',glsl:'glsl vert frag geom comp',hlsl:'hlsl fx fxh',wgsl:'wgsl',wasm:'wat wast',llvm:'ll',solidity:'sol',qsharp:'qs',sml:'sml sig',purescript:'purs',reason:'re rei',rescript:'res resi',puppet:'pp',rego:'rego',awk:'awk',racket:'rkt',vala:'vala vapi',v:'v',
 apacheconf:'htaccess apacheconf',http:'http rest.http',dnszone:'zone',dot:'dot gv',mermaid:'mmd mermaid',plantuml:'puml plantuml',haml:'haml',pug:'pug jade',ejs:'ejs',erb:'erb rhtml',django:'jinja jinja2 j2',handlebars:'hbs handlebars',liquid:'liquid',twig:'twig',smarty:'tpl',gettext:'po pot',robotframework:'robot resource',gherkin:'feature',ignore:'gitignore dockerignore',git:'gitconfig',text:'txt text nfo'
};
const aliases={raku:'perl',visualbasic:'visual-basic',dnszone:'dns-zone-file',plantuml:'plant-uml'};
const byExtension=new Map();
for(const [key,list] of Object.entries(extensions)){const id=aliases[key]||key;if(all.has(id))for(const ext of list.split(' '))if(!byExtension.has(ext))byExtension.set(ext,id);}
// Less common languages often use their published language ID/alias as the
// extension. Explicit associations above take precedence over ambiguous names.
for(const row of languages)for(const ext of [row.id,...row.aliases])if(/^[a-z][\w+-]*$/.test(ext)&&!byExtension.has(ext))byExtension.set(ext,row.id);
const filenames={'dockerfile':'docker','containerfile':'docker','makefile':'makefile','gnumakefile':'makefile','cmakelists.txt':'cmake','gemfile':'ruby','rakefile':'ruby','vagrantfile':'ruby','jenkinsfile':'groovy','go.mod':'go-module','go.sum':'go-module','cargo.lock':'toml','.bashrc':'bash','.bash_profile':'bash','.bash_logout':'bash','.zshrc':'bash','.zprofile':'bash','.profile':'bash','.gitconfig':'git','.gitignore':'ignore','.dockerignore':'ignore','.editorconfig':'editorconfig','.npmrc':'ini','.yarnrc':'yaml','.prettierrc':'json','.eslintrc':'json','.babelrc':'json','nginx.conf':'nginx','httpd.conf':'apacheconf','apache2.conf':'apacheconf','.htaccess':'apacheconf','.env':'bash','hosts':'network','requirements.txt':'text'};
const nameOf=path=>String(path||'').split(/[\\/]/).pop()||'';
function valid(id){return all.has(id)?id:'text';}
function detect(path,content=''){
 const name=nameOf(path).toLowerCase(),head=String(content).replace(/^\uFEFF/,'').slice(0,4096);
 if(Object.hasOwn(filenames,name))return valid(filenames[name]);
 if(/^(?:dockerfile|containerfile)\./.test(name))return 'docker';
 if(/^\.env(?:\.|$)/.test(name))return 'bash';
 const ext=name.includes('.')?name.split('.').pop():'';
 // Content signatures override only ambiguous config/text associations, not code.
 if(!ext||['txt','cfg','conf','config','cli','ios','aoscx','procurve','fortios','xml','exp'].includes(ext)||!byExtension.has(ext)){const network=root.NTKNetworkConfig?.detect(String(content));if(network)return network;}
 if(ext==='h'&&/\b(?:namespace|template|class)\b/.test(head))return 'cpp';
 if(ext==='m'&&!/^\s*(?:#\s*(?:import|include)|@(?:interface|implementation|import))\b/m.test(head))return 'matlab';
 if(['config','conf','cfg'].includes(ext)&&/^\s*<\?xml\b/i.test(head))return 'xml';
 if(ext==='inc'&&/^\s*<\?php\b/i.test(head))return 'php';
 if(byExtension.has(ext))return byExtension.get(ext);
 const interpreter=/^#![^\n]*?\b(pwsh|powershell|python[\d.]*|ruby|perl|raku|node|deno|bash|sh|zsh|ksh|fish|lua|php|Rscript)\b/i.exec(head);
 if(interpreter){const bin=interpreter[1].toLowerCase();return /^(?:pwsh|powershell)$/.test(bin)?'powershell':/^python/.test(bin)?'python':/^(?:node|deno)$/.test(bin)?'javascript':bin==='rscript'?'r':bin==='raku'?'perl':/^(?:bash|sh|zsh|ksh|fish)$/.test(bin)?'bash':valid(bin);}
 if(/^\s*<\?xml\b/i.test(head))return 'xml';
 if(/^\s*(?:<!doctype\s+html|<html[\s>])/i.test(head))return 'html';
 if(/^\s*<\?php\b/.test(head))return 'php';
 if(/^Windows Registry Editor Version 5\.00/.test(head))return 'registry';
 if(/^[\s]*[\[{]/.test(head)&&String(content).length<=250000){try{JSON.parse(content);return 'json';}catch(_){}}
 return 'text';
}
const parsers={javascript:'babel',jsx:'babel',typescript:'typescript',tsx:'typescript',json:'json',jsonc:'jsonc',json5:'json5',html:'html',vue:'vue',css:'css',scss:'scss',less:'less',markdown:'markdown',yaml:'yaml',graphql:'graphql'};
const braceLanguages=new Set(['c','cpp','csharp','java','objectivec','powershell','go','rust','swift','d','kotlin','scala','groovy','hcl','bicep']);
function formatKind(id){return parsers[id]?'format':braceLanguages.has(id)?'indent':'';}
const hashComment=new Set(['powershell','python','bash','ruby','perl','yaml','toml','ini','r','nim','julia','docker','makefile','cmake','hcl','nginx','apacheconf','network','properties','systemd','git','ignore','editorconfig','gherkin','elixir','graphql','nix']);
function commentStyle(id){if(id.startsWith('network-'))return id==='network-panos-xml'?['<!--','-->']:null;if(['html','xml','markup','vue'].includes(id))return ['<!--','-->'];if(['css','scss','less','jsonc'].includes(id))return ['/*','*/'];if(['sql','plsql','lua','haskell','ada'].includes(id))return ['--'];if(['batch'].includes(id))return ['REM'];if(['visual-basic','vbnet'].includes(id))return ["'"];if(['lisp','scheme','clojure','racket','nasm','armasm'].includes(id))return [';'];if(hashComment.has(id))return ['#'];if(id==='text'||id==='json'||id==='jsonlines')return null;return ['//'];}
function toggleComment(text,id){const style=commentStyle(id);if(!style)return null;const lines=String(text).split('\n'),[open,close]=style;
 const escape=s=>s.replace(/[.*+?^${}()|[\]\\]/g,'\\$&');
 if(close){const re=new RegExp('^(\\s*)'+escape(open)+' ?([\\s\\S]*?) ?'+escape(close)+'(\\s*)$');return lines.every(line=>!line.trim()||re.test(line))?lines.map(line=>line.trim()?line.replace(re,'$1$2$3'):line).join('\n'):lines.map(line=>line.trim()?line.replace(/^(\s*)(.*)$/,(_,indent,body)=>indent+open+' '+body+' '+close):line).join('\n');}
 const re=new RegExp('^(\\s*)'+escape(open)+'(?: |$)','i'),uncomment=lines.every(line=>!line.trim()||re.test(line));return lines.map(line=>!line.trim()?line:uncomment?line.replace(re,'$1'):line.replace(/^(\s*)/,(_,indent)=>indent+open+' ')).join('\n');
}
function newline(text,position,id,unit){const left=String(text).slice(0,position),line=left.slice(left.lastIndexOf('\n')+1),indent=line.match(/^[\t ]*/)[0];let extra=false;
 const code=line.replace(/"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|\/\/.*$|\s+#.*$/g,'').trimEnd();
 if(braceLanguages.has(id)||['javascript','typescript','jsx','tsx','json','jsonc','json5','css','scss','less','batch'].includes(id))extra=/[({[]$/.test(code);
 else if(id==='python'||id==='yaml')extra=/:$/.test(code);
 else if(['ruby','lua','bash'].includes(id))extra=/\b(?:do|then|else|function|begin)\b[^\n]*$/.test(code);
 else if(['html','xml','markup','vue'].includes(id))extra=/<([\w:-]+)(?:\s[^<>]*)?>$/.test(code)&&!/<(?:area|base|br|col|embed|hr|img|input|link|meta|param|source|track|wbr)\b|\/>$/i.test(code);
 const paired=extra&&/^[\t ]*[})\]]/.test(String(text).slice(position));return {text:'\n'+indent+(extra?unit:'')+(paired?'\n'+indent:''),back:paired?indent.length+1:0};
}
root.NTKEditorLanguages=Object.freeze({languages,detect,valid,label:id=>all.get(id)?.label||'Plain text',parser:id=>parsers[id]||'',formatKind,commentStyle,toggleComment,newline,nameOf});
})(globalThis);
