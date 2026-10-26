(function(root){
'use strict';
const SHELLS=root.NTKTerminalShellSyntax;
const shellTypes=[['powershell','PowerShell'],['cmd','Command Prompt']];
const ANSI=['black','red','green','yellow','blue','magenta','cyan','white','brightBlack','brightRed','brightGreen','brightYellow','brightBlue','brightMagenta','brightCyan','brightWhite'];
const CONSOLE=['background','foreground','cursor','selectionBackground'];
const COLORS=[...CONSOLE,...ANSI];
const HIGHLIGHTS={address:['IP addresses and networks','Example: 192.168.10.1/24'],mac:['MAC addresses','Example: 00:11:22:33:44:55'],interface:['Interface names','Examples: wan1, Gi1/0/1, 1/1/1, named interface rows'],command:['Commands and config verbs','Examples: config, edit, set, show, commit'],field:['Setting and field names','Examples: mode, allowaccess, mtu, source-address'],protocol:['Protocols and services','Examples: SSH, HTTPS, TCP, BGP, OSPF'],value:['Modes and named values','Examples: static, tunnel, enable, quoted object names'],number:['Numbers and units','Examples: VLAN 20, MTU 1500, port 443'],comment:['Comments and descriptions','Examples: interface descriptions, remarks, ! comments'],success:['Up and success','Examples: up, connected, OK'],warning:['Warnings and pending','Examples: warning, pending, filtered'],error:['Errors and down','Examples: down, failed, connection refused']};
const DARK={background:'#0b1220',foreground:'#dce6f6',cursor:'#7eb0ff',selectionBackground:'#537bac66',black:'#243149',red:'#f37d88',green:'#6aca9e',yellow:'#eec16f',blue:'#7baaff',magenta:'#c996e9',cyan:'#67cdd1',white:'#c8d4e8',brightBlack:'#73829a',brightRed:'#ffa1a9',brightGreen:'#8ce2b6',brightYellow:'#ffda91',brightBlue:'#9fc1ff',brightMagenta:'#deb4f6',brightCyan:'#8ce4e7',brightWhite:'#f1f6ff'};
const LIGHT={background:'#f9fbfe',foreground:'#20324c',cursor:'#2465bc',selectionBackground:'#93bfff77',black:'#23344b',red:'#b3273b',green:'#17734a',yellow:'#916400',blue:'#2465bc',magenta:'#7b4099',cyan:'#07847e',white:'#74849b',brightBlack:'#6c7f96',brightRed:'#cc354b',brightGreen:'#208d5e',brightYellow:'#a87400',brightBlue:'#3477ce',brightMagenta:'#9654b5',brightCyan:'#0c9890',brightWhite:'#172c47'};
function palette(background,foreground,cursor,selection,ansi){return {background,foreground,cursor,selectionBackground:selection,...Object.fromEntries(ANSI.map((key,i)=>[key,ansi[i]]))};}
const PRESETS={
 dark:{name:'Network Toolkit Dark',colors:DARK},light:{name:'Network Toolkit Light',colors:LIGHT},
 mobaCurrent:{name:'MobaXterm - Imported color theme',colors:palette('#1e1e1e','#ececec','#b4b4c0','#64688099',['#1e1e1e','#ff6060','#0eb16c','#bebe12','#1296be','#ff4dff','#54ccef','#cccccc','#808080','#ff8080','#18ed93','#dedc12','#1bbae9','#ff7dff','#8eddf4','#cccccc'])},
 pastelDusk:{name:'Pastel Dusk',colors:palette('#292638','#ebe5f6','#e9b7d2','#594963aa',['#302b42','#ed9ca8','#a9deb6','#eed6a7','#a5c5f5','#d9b7ee','#a4dfe7','#e1dbea','#716d8b','#ffc0c7','#c4edce','#ffe8c4','#c7d9ff','#edcafa','#c2eef3','#f9f6ff'])},
 pastelMint:{name:'Pastel Mint',colors:palette('#243630','#e2f5e8','#b1eed3','#4f807077',['#283d34','#eaa7ab','#a7dfbe','#f0d9ac','#a7c5de','#cdb7ec','#a8dedc','#d5eee1','#63857a','#ffc5cc','#c8f2d6','#ffecc9','#c0daef','#e8d4fa','#c8faf2','#f4fff7'])},
 pastelLavender:{name:'Pastel Lavender',colors:palette('#352e45','#f1e8ff','#d6b8ff','#8b73a566',['#39334f','#f0a9be','#b3e6c8','#f9ddaa','#b9cdff','#dfbbff','#ace6ee','#e6e0f3','#8b85a4','#ffceda','#cff5dc','#fcebc6','#d5e2ff','#f0d7ff','#c7f4f5','#ffffff'])},
 rosePine:{name:'Rosé Pine',colors:palette('#191724','#e0def4','#c4a7e7','#403d5266',['#26233a','#eb6f92','#9ccfd8','#f6c177','#31748f','#c4a7e7','#ebbcba','#e0def4','#6e6a86','#eb6f92','#9ccfd8','#f6c177','#31748f','#c4a7e7','#ebbcba','#e0def4'])},
 catppuccin:{name:'Catppuccin Mocha',colors:palette('#1e1e2e','#cdd6f4','#f5e0dc','#45475a99',['#45475a','#f38ba8','#a6e3a1','#f9e2af','#89b4fa','#f5c2e7','#94e2d5','#bac2de','#585b70','#f38ba8','#a6e3a1','#f9e2af','#89b4fa','#f5c2e7','#94e2d5','#a6adc8'])},
 nord:{name:'Nord',colors:palette('#2e3440','#eceff4','#88c0d0','#434c5e99',['#3b4252','#bf616a','#a3be8c','#ebcb8b','#81a1c1','#b48ead','#88c0d0','#e5e9f0','#4c566a','#bf616a','#a3be8c','#ebcb8b','#81a1c1','#b48ead','#8fbcbb','#eceff4'])},
 tokyoNight:{name:'Tokyo Night',colors:palette('#1a1b26','#c0caf5','#7aa2f7','#3b4261aa',['#15161e','#f7768e','#9ece6a','#e0af68','#7aa2f7','#bb9af7','#7dcfff','#a9b1d6','#565f89','#f7768e','#9ece6a','#e0af68','#7aa2f7','#bb9af7','#7dcfff','#c0caf5'])},
 gruvbox:{name:'Gruvbox Dark',colors:palette('#282828','#ebdbb2','#fabd2f','#504945aa',['#282828','#cc241d','#98971a','#d79921','#458588','#b16286','#689d6a','#a89984','#928374','#fb4934','#b8bb26','#fabd2f','#83a598','#d3869b','#8ec07c','#ebdbb2'])},
 dracula:{name:'Dracula',colors:palette('#282a36','#f8f8f2','#f8f8f2','#44475a',['#21222c','#ff5555','#50fa7b','#f1fa8c','#bd93f9','#ff79c6','#8be9fd','#f8f8f2','#6272a4','#ff6e6e','#69ff94','#ffffa5','#d6acff','#ff92df','#a4ffff','#ffffff'])},
 solarizedDark:{name:'Solarized Dark',colors:palette('#002b36','#839496','#93a1a1','#073642',['#073642','#dc322f','#859900','#b58900','#268bd2','#d33682','#2aa198','#eee8d5','#586e75','#cb4b16','#93a1a1','#839496','#657b83','#6c71c4','#93a1a1','#fdf6e3'])},
 solarizedLight:{name:'Solarized Light',colors:palette('#fdf6e3','#657b83','#586e75','#eee8d5',['#073642','#dc322f','#859900','#b58900','#268bd2','#d33682','#2aa198','#eee8d5','#586e75','#cb4b16','#93a1a1','#839496','#657b83','#6c71c4','#93a1a1','#fdf6e3'])},
 campbell:{name:'Windows Campbell',colors:palette('#0c0c0c','#cccccc','#ffffff','#264f78',['#0c0c0c','#c50f1f','#13a10e','#c19c00','#0037da','#881798','#3a96dd','#cccccc','#767676','#e74856','#16c60c','#f9f1a5','#3b78ff','#b4009e','#61d6d6','#f2f2f2'])}
};
function normalize(value){
 const v=value&&typeof value==='object'&&!Array.isArray(value)?value:{},family=typeof v.fontFamily==='string'?v.fontFamily.trim():'';
 const out={fontFamily:Array.from(family).length>0&&Array.from(family).length<=80&&/^[\p{L}\p{N} _.-]+$/u.test(family)?family:'Consolas',fontWeight:v.fontWeight==='bold'?'bold':'normal',lineHeight:typeof v.lineHeight==='number'&&Number.isFinite(v.lineHeight)&&v.lineHeight>=1&&v.lineHeight<=2?Math.round(v.lineHeight*100)/100:1.15,scheme:typeof v.scheme==='string'&&(v.scheme==='custom'||Object.hasOwn(PRESETS,v.scheme))?v.scheme:'app',colors:{},highlightUnderlines:v.highlightUnderlines!==false,highlightColors:{}};
 for(const key of COLORS)if(typeof v.colors?.[key]==='string'&&new RegExp(key==='selectionBackground'?'^#[0-9a-fA-F]{6}([0-9a-fA-F]{2})?$':'^#[0-9a-fA-F]{6}$').test(v.colors[key]))out.colors[key]=v.colors[key].toLowerCase();
 for(const kind of Object.keys(HIGHLIGHTS))if(typeof v.highlightColors?.[kind]==='string'&&/^#[0-9a-f]{6}$/i.test(v.highlightColors[kind]))out.highlightColors[kind]=v.highlightColors[kind].toLowerCase();
 out.shells=SHELLS?.normalizePreferences(v.shells)||{powershell:{enabled:true,colors:{}},cmd:{enabled:true,colors:{}}};
 return out;
}
function theme(value,appTheme='dark'){
 const clean=normalize(value),base=appTheme==='light'?LIGHT:DARK;
 return clean.scheme==='custom'?{...base,...clean.colors}:{...(PRESETS[clean.scheme]?.colors||base)};
}
function mode(value,appTheme='dark'){
 const color=theme(value,appTheme).background.slice(1),rgb=[0,2,4].map(at=>parseInt(color.slice(at,at+2),16)/255).map(n=>n<=.04045?n/12.92:((n+.055)/1.055)**2.4);
 return .2126*rgb[0]+.7152*rgb[1]+.0722*rgb[2]>.35?'light':'dark';
}
function options(value,appTheme='dark'){const clean=normalize(value);return {fontFamily:clean.fontFamily==='monospace'?'monospace':JSON.stringify(clean.fontFamily)+', monospace',fontWeight:clean.fontWeight,lineHeight:clean.lineHeight,theme:theme(clean,appTheme)};}
const names={background:'Console background',foreground:'Normal text',cursor:'Typing cursor',selectionBackground:'Selected text background',black:'Black',red:'Red',green:'Green',yellow:'Yellow',blue:'Blue',magenta:'Magenta',cyan:'Cyan',white:'White',brightBlack:'Bright black',brightRed:'Bright red',brightGreen:'Bright green',brightYellow:'Bright yellow',brightBlue:'Bright blue',brightMagenta:'Bright magenta',brightCyan:'Bright cyan',brightWhite:'Bright white'};
const descriptions={background:'The area behind all terminal text.',foreground:'Text without a device color or automatic highlight.',cursor:'The marker showing where you are typing.',selectionBackground:'The background behind text you select.'};
function highlightOptions(value,appTheme='dark',language){const clean=normalize(value);return {theme:mode(clean,appTheme),colors:clean.highlightColors,underlines:clean.highlightUnderlines,...(SHELLS?.supported(language)?{shellColors:clean.shells[language].colors}:{})};}
function enabled(value,language,deviceEnabled=true){return SHELLS?.supported(language)?normalize(value).shells[language].enabled:deviceEnabled;}
function shellSettingsHTML(){if(!SHELLS)return '';return '<section class="terminal-local-shell-settings"><h4>Local shell syntax colors</h4><p class="hint">PowerShell and Command Prompt have separate controls. Colors cover commands and recognized output headings, fields, addresses and status text. Colors sent by programs are preserved.</p>'+shellTypes.map(([type,label])=>{
 const text=type==='powershell'?'PS C:\\> Get-ChildItem -Path "$env:TEMP" | Select-Object -First 2 # files':'C:\\> for %A in ("text") do echo %A';
 const tokens=SHELLS.analyze(text,type),escape=v=>String(v).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('"','&quot;');let preview='',last=0;
 for(const token of tokens){preview+=escape(text.slice(last,token.start))+'<span data-terminal-shell-preview="'+type+':'+token.kind.slice(6)+'">'+escape(text.slice(token.start,token.end))+'</span>';last=token.end;}preview+=escape(text.slice(last));
 return '<div class="terminal-shell-settings" data-terminal-shell-card="'+type+'"><div class="terminal-shell-heading"><strong>'+label+' colors</strong><label><input type="checkbox" data-terminal-shell-enabled="'+type+'">Color commands and output</label></div><pre class="terminal-shell-preview" data-terminal-shell-sample="'+type+'" aria-label="'+label+' syntax color preview">'+preview+'</pre><details><summary>Customize '+label+' and output colors</summary><div class="terminal-color-grid">'+Object.entries(SHELLS.roles).map(([role,name])=>'<label class="terminal-color-control" for="terminalShell-'+type+'-'+role+'"><input type="color" id="terminalShell-'+type+'-'+role+'" data-terminal-shell-color="'+type+':'+role+'" aria-label="'+label+' '+name+' color"><span class="terminal-color-description"><strong>'+name+'</strong></span><input type="text" class="terminal-color-hex" data-input-clear="off" data-terminal-shell-hex="'+type+':'+role+'" maxlength="7" spellcheck="false" aria-label="'+label+' '+name+' hex color"></label>').join('')+'</div><p class="hint">Defaults follow the terminal palette. A custom syntax color overrides the shell color for that recognized command or output token.</p><button type="button" class="btn small ghost" data-terminal-shell-reset="'+type+'">Reset '+label+' syntax colors</button></details></div>';
 }).join('')+'</section>';}
function syncShellSettings(section,clean,colors){if(!SHELLS)return;for(const [type]of shellTypes){const pref=clean.shells[type],palette=SHELLS.palette(colors,mode(clean,state.theme),pref.colors);for(const input of section.querySelectorAll('[data-terminal-shell-enabled="'+type+'"]'))input.checked=pref.enabled;for(const input of section.querySelectorAll('[data-terminal-shell-color],[data-terminal-shell-hex]')){const key=input.dataset.terminalShellColor||input.dataset.terminalShellHex,[kind,role]=key.split(':');if(kind!==type)continue;if(input!==document.activeElement)input.value=palette['shell-'+role];input.disabled=!pref.enabled;}for(const preview of section.querySelectorAll('[data-terminal-shell-sample="'+type+'"]')){preview.style.backgroundColor=colors.background;preview.style.color=colors.foreground;preview.style.fontFamily=options(clean,state.theme).fontFamily;}for(const span of section.querySelectorAll('[data-terminal-shell-preview]')){const [kind,role]=span.dataset.terminalShellPreview.split(':');if(kind===type)span.style.color=pref.enabled?palette['shell-'+role]:colors.foreground;}}const confirm=section.querySelector('#terminalConfirmClose');if(confirm)confirm.checked=state.terminalConfirmClose!==false;}
function colorField(key,label,help,highlight=false){
 const id=(highlight?'terminalHighlightColor-':'terminalColor-')+key;
 return '<label class="terminal-color-control" for="'+id+'"><input id="'+id+'" type="color" '+(highlight?'data-terminal-highlight-color':'data-terminal-color')+'="'+key+'" aria-label="'+label+' color" aria-describedby="'+id+'Hint"><span class="terminal-color-description"><strong>'+label+'</strong><small id="'+id+'Hint">'+help+'</small></span><input type="text" class="terminal-color-hex" data-terminal-hex="'+(highlight?'highlight:':'console:')+key+'" inputmode="text" maxlength="7" spellcheck="false" aria-label="'+label+' hex color" placeholder="#RRGGBB"></label>';
}
function mount(panel){
 if(!panel||document.getElementById('terminalAppearanceEditor'))return;
 const section=document.createElement('section');section.id='terminalAppearanceEditor';section.className='terminal-tools-section terminal-appearance-editor';
 const fonts=['Consolas','Cascadia Mono','Cascadia Code','Courier New','Lucida Console','DejaVu Sans Mono','monospace'].map(font=>'<option value="'+font+'"></option>').join('');
 const schemes=Object.entries(PRESETS).map(([id,preset])=>'<option value="'+id+'">'+preset.name+'</option>').join('');
 const consoleColors=CONSOLE.map(key=>colorField(key,names[key],descriptions[key])).join('');
 const highlightColors=Object.entries(HIGHLIGHTS).map(([key,[label,help]])=>colorField(key,label,help,true)).join('');
 const deviceColors=ANSI.map((key,index)=>colorField(key,names[key],'Device color slot '+index+(index>=8?' (bright variant).':'.'))).join('');
 section.innerHTML=`<h3>Terminal appearance</h3>
 <div class="terminal-appearance-grid"><div class="field"><label for="terminalAppearanceFont">Font</label><input id="terminalAppearanceFont" data-terminal-appearance="fontFamily" list="terminalFontFamilies" maxlength="80" autocomplete="off"><datalist id="terminalFontFamilies">${fonts}</datalist></div><div class="field"><label for="terminalAppearanceWeight">Font weight</label><select id="terminalAppearanceWeight" data-terminal-appearance="fontWeight"><option value="normal">Regular</option><option value="bold">Bold</option></select></div><div class="field"><label for="terminalAppearanceLineHeight">Line spacing</label><input id="terminalAppearanceLineHeight" type="number" data-terminal-appearance="lineHeight" min="1" max="2" step="0.05"></div><div class="field"><label for="terminalAppearanceScheme">Color scheme</label><select id="terminalAppearanceScheme" data-terminal-appearance="scheme"><option value="app">Follow app theme</option>${schemes}<option value="custom">Custom colors</option></select></div></div>
 <p class="hint">Choose a preset to set the colors together, or customize them below. Fonts must be installed on this PC. Font size is available above.</p>
 ${shellSettingsHTML()}
 <section class="terminal-close-settings"><h4>Closing tabs and sessions</h4><label class="terminal-close-confirmation"><input type="checkbox" id="terminalConfirmClose">Confirm before closing active terminal sessions or using Close other tabs</label><p class="hint">Turn this off to disconnect and close those tabs immediately. Saved and Recent entry removal has its own confirmation setting.</p></section>
 <pre id="terminalAppearancePreview" class="terminal-appearance-preview" aria-label="Live terminal style preview"><span>router# <span data-terminal-preview-highlight="command">show</span> <span data-terminal-preview-highlight="field">interfaces</span></span><span data-terminal-preview-cursor aria-hidden="true"> </span>
<span data-terminal-preview-highlight="interface">wan1</span>  <span data-terminal-preview-highlight="value">static</span>  <span data-terminal-preview-highlight="success">up</span>  <span data-terminal-preview-highlight="value">physical</span>
<span data-terminal-preview-highlight="interface">Gi1/0/2</span>  <span data-terminal-preview-highlight="error">down</span>  No link
<span data-terminal-preview-highlight="address">192.168.10.1/24</span>  <span data-terminal-preview-highlight="mac">00:11:22:33:44:55</span>
<span data-terminal-preview-highlight="command">set</span> <span data-terminal-preview-highlight="field">allowaccess</span> <span data-terminal-preview-highlight="protocol">ssh https</span>
<span data-terminal-preview-highlight="command">set</span> <span data-terminal-preview-highlight="field">mtu</span> <span data-terminal-preview-highlight="number">1500</span>
<span data-terminal-preview-highlight="command">set</span> <span data-terminal-preview-highlight="field">description</span> <span data-terminal-preview-highlight="comment">"Router uplink"</span>
<span data-terminal-preview-highlight="warning">Warning</span>: pending link check
Device color: <span data-terminal-preview-color="red">red slot</span>  <span data-terminal-preview-selection>Selected text</span></pre>
 <label class="terminal-highlight-underlines" for="terminalHighlightUnderlines"><input id="terminalHighlightUnderlines" type="checkbox" data-terminal-appearance="highlightUnderlines"><span>Underline highlighted text</span></label>
 <p class="hint">Turn off underlines for color-only automatic highlights. Formatting sent by the device stays intact.</p>
 <details id="terminalPaletteDetails"><summary>Customize colors</summary>
 <h4>Console colors</h4><div class="terminal-color-grid">${consoleColors}</div>
 <h4>Automatic highlight colors</h4><p class="hint">These colors identify configuration roles, recognized values and operational states when Highlight device output is enabled. Enable/disable and allow/deny are setting values, not a verdict on device health. Distinct colors sent by the device take precedence. Neutral white, gray and normal text can receive automatic highlights.</p><div class="terminal-color-grid">${highlightColors}</div>
 <details id="terminalDeviceColorsDetails"><summary>Device colors (advanced)</summary><p class="hint">These 16 ANSI color slots are chosen by the connected device or application. They have no fixed meaning such as error or success. Most users can keep the preset.</p><p class="hint">Names such as White are traditional slot names. A light theme may use dark text for that slot so it remains readable.</p><div class="terminal-color-grid terminal-ansi-grid">${deviceColors}</div></details>
 <p class="hint">Editing a color selects Custom colors. Choosing a preset restores its colors; the underline option is kept.</p></details>
 <button type="button" class="btn small ghost" data-terminal-appearance-reset>Reset appearance</button>`;
 panel.append(section);sync();
}
function sync(){
 const section=document.getElementById('terminalAppearanceEditor');if(!section)return;
 const clean=normalize(state.terminalAppearance),colors=theme(clean,state.theme),font=options(clean,state.theme),highlightSettings=highlightOptions(clean,state.theme),legend=root.NTTerminalHighlight?.legend?.(highlightSettings.theme,highlightSettings)||[],marks=Object.fromEntries(legend.map(mark=>[mark.kind,mark]));
 for(const input of section.querySelectorAll('[data-terminal-appearance]')){if(input.getAttribute('type')==='checkbox')input.checked=clean[input.dataset.terminalAppearance]===true;else if(input!==document.activeElement)input.value=String(clean[input.dataset.terminalAppearance]);}
 for(const input of section.querySelectorAll('[data-terminal-color]'))if(input!==document.activeElement)input.value=colors[input.dataset.terminalColor].slice(0,7);
 for(const input of section.querySelectorAll('[data-terminal-highlight-color]'))if(input!==document.activeElement)input.value=marks[input.dataset.terminalHighlightColor]?.color||colors.foreground;
 for(const input of section.querySelectorAll('[data-terminal-hex]')){if(input===document.activeElement)continue;const [scope,key]=input.dataset.terminalHex.split(':');input.value=scope==='highlight'?(marks[key]?.color||colors.foreground):colors[key].slice(0,7);}
 syncShellSettings(section,clean,colors);
 const preview=document.getElementById('terminalAppearancePreview');preview.style.backgroundColor=colors.background;preview.style.color=colors.foreground;preview.style.fontFamily=font.fontFamily;preview.style.fontWeight=clean.fontWeight;preview.style.fontSize=(state.terminalFontSize||13)+'px';preview.style.lineHeight=String(clean.lineHeight);
 for(const span of preview.querySelectorAll('[data-terminal-preview-highlight]')){const mark=marks[span.dataset.terminalPreviewHighlight],enabled=state.terminalHighlight!==false&&!!mark;span.style.color=enabled?mark.color:colors.foreground;span.style.borderBottom=enabled&&mark.borderWidth?mark.borderWidth+'px '+mark.borderStyle+' '+mark.color:'none';}
 for(const span of preview.querySelectorAll('[data-terminal-preview-color]'))span.style.color=colors[span.dataset.terminalPreviewColor];
 const selection=preview.querySelector('[data-terminal-preview-selection]');selection.style.backgroundColor=colors.selectionBackground;
 const cursor=preview.querySelector('[data-terminal-preview-cursor]');cursor.style.backgroundColor=colors.cursor;
 const size=document.getElementById('terminalFontSize');if(size&&size!==document.activeElement)size.value=String(state.terminalFontSize||13);
}
function apply(value){state.terminalAppearance=normalize(value);saveState();updateTerminalTheme();sync();}
function changed(event){
 const input=event.target;if(!input.closest?.('#terminalAppearanceEditor'))return;
 if(input.id==='terminalConfirmClose'){state.terminalConfirmClose=input.checked;saveState();return;}
 const shellEnabled=input.dataset.terminalShellEnabled,shellColor=input.dataset.terminalShellColor||input.dataset.terminalShellHex;
 if(shellEnabled||shellColor){const next=normalize(state.terminalAppearance);if(shellEnabled){next.shells[shellEnabled].enabled=input.checked;}else{const [type,role]=shellColor.split(':'),value=input.value.trim();if(!/^#[0-9a-f]{6}$/i.test(value)){input.setCustomValidity('Use a six-digit hex color, such as #aabbcc');input.reportValidity();return;}input.setCustomValidity('');next.shells[type].colors[role]=value.toLowerCase();}apply(next);return;}

 const hex=input.dataset.terminalHex;if(hex){if(!/^#[0-9a-f]{6}$/i.test(input.value.trim())){input.setCustomValidity('Use a six-digit hex color, such as #aabbcc');input.reportValidity();return;}input.setCustomValidity('');const [scope,name]=hex.split(':');const next=normalize(state.terminalAppearance);next.colors=theme(next,state.theme);if(scope==='highlight')next.highlightColors[name]=input.value.toLowerCase();else next.colors[name]=input.value.toLowerCase();next.scheme='custom';apply(next);return;}
 const key=input.dataset.terminalAppearance,color=input.dataset.terminalColor,highlight=input.dataset.terminalHighlightColor;if(!key&&!color&&!highlight)return;
 const next=normalize(state.terminalAppearance);
 if(color){next.colors={...theme(next,state.theme),[color]:input.value};next.scheme='custom';}
 else if(highlight){next.colors=theme(next,state.theme);next.highlightColors[highlight]=input.value;next.scheme='custom';}
 else if(key==='scheme'){if(input.value==='custom'&&next.scheme!=='custom')next.colors=theme(next,state.theme);else if(input.value!=='custom'){next.colors={};next.highlightColors={};}next.scheme=input.value;}
 else next[key]=key==='highlightUnderlines'?input.checked:key==='lineHeight'?Number(input.value):input.value;
 apply(next);
}
document.addEventListener('change',changed);
document.addEventListener('input',event=>{if(event.target.dataset.terminalColor||event.target.dataset.terminalHighlightColor||event.target.dataset.terminalShellColor)changed(event);});
document.addEventListener('click',event=>{const reset=event.target.closest?.('[data-terminal-shell-reset]');if(reset){event.preventDefault();const next=normalize(state.terminalAppearance);next.shells[reset.dataset.terminalShellReset]={enabled:true,colors:{}};apply(next);return;}if(event.target.closest?.('[data-terminal-appearance-reset]')){event.preventDefault();state.terminalFontSize=13;apply({});}});
root.NetworkToolkitTerminalAppearance=Object.freeze({normalize,theme,mode,options,highlightOptions,enabled,mount,sync,presets:PRESETS,colors:COLORS,highlightKinds:Object.keys(HIGHLIGHTS)});
})(window);
