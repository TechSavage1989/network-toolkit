'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test'),{document:makeDocument,Element}=require('./terminal-dom-fixture.cjs');
global.self=global;const {Terminal}=require('../web/assets/xterm.js');
const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8'),source=fs.readFileSync(require.resolve('../web/assets/terminal-appearance.js'),'utf8');
function harness(){
 const document=makeDocument('<div id="terminalSessionPanel"><select id="terminalFontSize"></select></div>'),state={theme:'dark',terminalFontSize:13};let saves=0,fits=0;
 const c={document,state,terminalSessions:new Map(),terminalInitialized:true,saveState(){saves++;},scheduleTerminalFit(){fits++;}};c.window=c;
 vm.createContext(c);for(const name of ['terminal-shell-syntax.js','terminal-highlight.js'])vm.runInContext(fs.readFileSync(require.resolve('../web/assets/'+name),'utf8'),c);vm.runInContext(source,c);
 for(const name of ['terminalTheme','updateTerminalTheme'])vm.runInContext(html.split('\n').find(line=>line.startsWith('function '+name+'(')),c);
 const api=c.NetworkToolkitTerminalAppearance;api.mount(document.getElementById('terminalSessionPanel'));
 return{c,api,document,state,get saves(){return saves},get fits(){return fits}};
}
test('terminal appearance normalizes fonts and color values and excludes unsupported settings',()=>{
 const h=harness(),value=h.api.normalize({fontFamily:'url(secret-canary)',fontWeight:'bolder',lineHeight:99,scheme:'__proto__',password:'secret-canary',colors:{background:'#ABCDEF',selectionBackground:'#12345688',red:'url(secret-canary)',unknown:'secret-canary'}});
 assert.equal(value.fontFamily,'Consolas');assert.equal(value.scheme,'app');assert.equal(value.lineHeight,1.15);assert.equal(value.fontWeight,'normal');assert.deepEqual(JSON.parse(JSON.stringify(value.colors)),{background:'#abcdef',selectionBackground:'#12345688'});assert(!JSON.stringify(value).includes('secret-canary'));
});
test('preset themes cover all ANSI colors and independent light/dark choices survive app theme changes',()=>{
 const h=harness();for(const scheme of Object.keys(h.api.presets)){const theme=h.api.theme({scheme},'dark');assert.equal(Object.keys(theme).length,20);for(const color of Object.values(theme))assert.match(color,/^#[a-f0-9]{6}([a-f0-9]{2})?$/);}
 assert.equal(h.api.theme({},'light').background,'#f9fbfe');assert.equal(h.api.theme({scheme:'dracula'},'light').background,'#282a36');assert.equal(h.api.mode({scheme:'solarizedLight'},'dark'),'light');assert.equal(h.api.mode({scheme:'dracula'},'light'),'dark');
});
test('font and palette edits apply to existing terminals without changing retained output',async()=>{
 const h=harness(),term=new Terminal({cols:40,rows:6,allowProposedApi:true});await new Promise(resolve=>term.write('router# retained output',resolve));const entry={xterm:term,element:new Element(),highlighter:{setAppearance:settings=>{entry.mode=settings.theme;entry.highlightAppearance=settings;}}};h.c.terminalSessions.set('one',entry);
 const font=h.document.getElementById('terminalAppearanceFont');font.value='Cascadia Mono';h.document.emit('change',font);assert.equal(term.options.fontFamily,'"Cascadia Mono", monospace');assert.equal(h.saves,1);
 const scheme=h.document.getElementById('terminalAppearanceScheme');scheme.value='light';h.document.emit('change',scheme);assert.equal(term.options.theme.background,'#f9fbfe');assert.equal(entry.mode,'light');
 const color=h.document.getElementById('terminalColor-background');color.value='#112233';h.document.emit('input',color);assert.equal(h.state.terminalAppearance.scheme,'custom');assert.equal(term.options.theme.background,'#112233');assert.equal(entry.element.style.backgroundColor,'#112233');assert.equal(term.buffer.normal.getLine(0).translateToString(true),'router# retained output');
 const preview=h.document.getElementById('terminalAppearancePreview');assert.equal(preview.style.backgroundColor,'#112233');assert.equal(preview.style.fontFamily,'"Cascadia Mono", monospace');term.dispose();
});
test('appearance restores from stored preferences and reset keeps the session button position',()=>{
 const h=harness();h.state.terminalAppearance=h.api.normalize({fontFamily:'Courier New',fontWeight:'bold',lineHeight:1.4,scheme:'custom',colors:{background:'#101010',foreground:'#fafafa',brightRed:'#ff8888'}});h.state.terminalActiveActionsPosition='bottom';h.api.sync();assert.equal(h.document.getElementById('terminalAppearanceFont').value,'Courier New');assert.equal(h.document.getElementById('terminalColor-brightRed').value,'#ff8888');
 h.document.emit('click',h.document.querySelector('[data-terminal-appearance-reset]'));assert.equal(h.state.terminalAppearance.scheme,'app');assert.equal(h.state.terminalAppearance.fontFamily,'Consolas');assert.equal(h.state.terminalFontSize,13);assert.equal(h.state.terminalActiveActionsPosition,'bottom');
});
test('palette groups explain console, automatic highlight and advanced device colors by purpose',()=>{
 const h=harness(),d=h.document;assert(d.getElementById('terminalPaletteDetails').textContent.includes('Console colors'));assert(d.getElementById('terminalPaletteDetails').textContent.includes('Automatic highlight colors'));assert.equal(d.querySelectorAll('[data-terminal-highlight-color]').length,12);assert.equal(d.querySelectorAll('#terminalDeviceColorsDetails [data-terminal-color]').length,16);assert(d.getElementById('terminalDeviceColorsDetails').textContent.includes('no fixed meaning'));assert(d.getElementById('terminalDeviceColorsDetails').textContent.includes('light theme may use dark text'));assert.equal(d.getElementById('terminalColor-foreground').getAttribute('aria-label'),'Normal text color');assert(d.getElementById('terminalColor-foregroundHint').textContent.includes('without a device color'));assert.equal(d.getElementById('terminalHighlightUnderlines').checked,true);
});
test('underline and named highlight color edits match the preview and live terminal settings',()=>{
 const h=harness(),d=h.document,entry={xterm:{options:{}},element:new Element(),highlighter:{setAppearance:value=>{entry.settings=value;}}};h.c.terminalSessions.set('one',entry);
 const underline=d.getElementById('terminalHighlightUnderlines');underline.checked=false;d.emit('change',underline);assert.equal(h.state.terminalAppearance.highlightUnderlines,false);assert.equal(entry.settings.underlines,false);
 const address=d.getElementById('terminalHighlightColor-address');address.value='#123abc';d.emit('input',address);assert.equal(h.state.terminalAppearance.highlightColors.address,'#123abc');assert.equal(h.state.terminalAppearance.scheme,'custom');assert.equal(entry.settings.colors.address,'#123abc');
 const preview=d.querySelector('[data-terminal-preview-highlight="address"]');assert.equal(preview.style.color,'#123abc');assert.equal(preview.style.borderBottom,'none');
 underline.checked=true;d.emit('change',underline);assert.match(preview.style.borderBottom,/solid #123abc/);
 h.state.terminalHighlight=false;h.api.sync();assert.equal(preview.style.borderBottom,'none');assert.equal(preview.style.color,h.api.theme(h.state.terminalAppearance,h.state.theme).foreground);
 const scheme=d.getElementById('terminalAppearanceScheme');scheme.value='light';d.emit('change',scheme);assert.deepEqual(JSON.parse(JSON.stringify(h.state.terminalAppearance.highlightColors)),{});assert.equal(h.state.terminalAppearance.highlightUnderlines,true);
 const clean=h.api.normalize({highlightUnderlines:'false',highlightColors:{address:'#ABCDEF',error:'url(secret-canary)',unknown:'#ffffff'}});assert.equal(clean.highlightUnderlines,true);assert.deepEqual(JSON.parse(JSON.stringify(clean.highlightColors)),{address:'#abcdef'});
});
test('PowerShell and Command Prompt controls are independent, survive normalization and match live session settings',()=>{
 const h=harness(),d=h.document;for(const type of ['powershell','cmd']){assert(d.querySelector('[data-terminal-shell-card="'+type+'"]'));assert(d.querySelector('[data-terminal-shell-enabled="'+type+'"]').checked);assert.equal(d.querySelectorAll('[data-terminal-shell-color]').filter(el=>el.dataset.terminalShellColor.startsWith(type+':')).length,9);}
 const normal=h.api.normalize({shells:{powershell:{enabled:false,colors:{command:'#ABCDEF',string:'url(secret)',unknown:'#123456'}},cmd:{enabled:true,colors:{variable:'#112233'}}}});assert.equal(normal.shells.powershell.enabled,false);assert.deepEqual(JSON.parse(JSON.stringify(normal.shells.powershell.colors)),{command:'#abcdef'});assert.equal(normal.shells.cmd.colors.variable,'#112233');assert(!JSON.stringify(normal).includes('secret'));
 h.state.terminalHighlight=false;assert.equal(h.api.enabled({},'powershell',false),true);assert.equal(h.api.enabled({},'cmd',false),true);assert.equal(h.api.enabled({},'ssh',false),false);
 const entry={info:{type:'powershell'},xterm:{options:{}},element:new Element(),highlighter:{setAppearance:s=>entry.settings=s,setEnabled:e=>entry.enabled=e}};h.c.terminalSessions.set('ps',entry);
 const hex=d.querySelector('[data-terminal-shell-hex="powershell:command"]');hex.value='#123abc';d.emit('change',hex);assert.equal(h.state.terminalAppearance.shells.powershell.colors.command,'#123abc');assert.equal(entry.settings.shellColors.command,'#123abc');assert(entry.enabled);assert.equal(d.querySelector('[data-terminal-shell-preview="powershell:command"]').style.color,'#123abc');assert.equal(h.state.terminalAppearance.scheme,'app');
 const enabled=d.querySelector('[data-terminal-shell-enabled="powershell"]');enabled.checked=false;d.emit('change',enabled);assert.equal(entry.enabled,false);assert(h.api.enabled(h.state.terminalAppearance,'cmd',false));
 d.emit('click',d.querySelector('[data-terminal-shell-reset="powershell"]'));assert(h.state.terminalAppearance.shells.powershell.enabled);assert.deepEqual(JSON.parse(JSON.stringify(h.state.terminalAppearance.shells.powershell.colors)),{});
 const close=d.getElementById('terminalConfirmClose');close.checked=false;d.emit('change',close);assert.equal(h.state.terminalConfirmClose,false);
});
