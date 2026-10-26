'use strict';
const fs=require('node:fs'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');const c={};vm.runInNewContext(fs.readFileSync(require.resolve('../web/assets/terminal-shell-syntax.js'),'utf8'),c);const syntax=c.NTKTerminalShellSyntax;
const tokens=(text,language)=>syntax.analyze(text,language).map(t=>[text.slice(t.start,t.end),t.kind]);
test('PowerShell token roles include variables, parameters, operators, types and quoted comments',()=>{
 const line='PS C:\\Work> [string]$env:NAME = Get-Item -Path "a # string" | Where-Object { $_.Length -gt 3 } # comment';const result=tokens(line,'powershell');
 for(const [text,kind]of [['[string]','type'],['$env:NAME','variable'],['Get-Item','command'],['-Path','parameter'],['"a # string"','string'],['-gt','operator'],['3','number'],['# comment','comment']])assert(result.some(t=>t[0]===text&&t[1]==='shell-'+kind),text);
 assert(!result.some(t=>t[0].startsWith('PS ')));assert.equal(tokens('ordinary output Get-Item -Path','powershell').length,0);
});
test('cmd environment, delayed, positional and for variables stay complete; escaped operators stay literal',()=>{
 const line='C:\\Work> echo %PATH% !NAME! %1 %~dp0 %%A %A ^& && dir /a';const result=tokens(line,'cmd');for(const value of ['%PATH%','!NAME!','%1','%~dp0','%%A','%A'])assert(result.some(t=>t[0]===value&&t[1]==='shell-variable'),value);assert(!result.some(t=>t[0]==='^&'));assert(result.some(t=>t[0]==='&&'&&t[1]==='shell-operator'));assert(result.some(t=>t[0]==='/a'&&t[1]==='shell-parameter'));
 assert(tokens('C:\\> REM keep %PATH% literal','cmd').some(t=>t[0]==='REM keep %PATH% literal'&&t[1]==='shell-comment'));assert.equal(tokens('no prompt echo %PATH%','cmd').length,0);
});
test('Fit console is absent from toolbar and context choices while automatic fitting stays wired',()=>{
 const html=fs.readFileSync(require.resolve('../web/index.html'),'utf8'),interaction=fs.readFileSync(require.resolve('../web/assets/terminal-interaction.js'),'utf8');assert(!html.includes('Fit console'));assert(!interaction.includes("id:'fit'"));assert(html.includes('function scheduleTerminalFit()'));
});
