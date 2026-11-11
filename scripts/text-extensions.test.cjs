'use strict';
// The Text Editor can be the Windows default for many file types and is
// offered for every file. These checks run everywhere (no Windows needed):
// the one list of types is what Setup, its hosted check and the guide use,
// and Setup's registration keeps to what Windows permits and the project
// allows. scripts/installer/verify-windows.ps1 checks the registry itself on
// the hosted Windows runner.
const fs=require('node:fs'),path=require('node:path'),vm=require('node:vm'),assert=require('node:assert/strict'),{test}=require('node:test');
const list=require('./text-extensions.cjs');
const read=file=>fs.readFileSync(path.join(list.ROOT,file),'utf8');
const nsi=read('scripts/installer/windows.nsi'),ps1=read('scripts/installer/verify-windows.ps1');
// The installer script without its comments.
const code=nsi.split(/\r?\n/).filter(line=>!/^\s*;/.test(line)).join('\n');
const all=list.extensions();

test('the list holds text formats only, each once, and keeps every type a released Setup registered',()=>{
 assert(all.length>=100,'the list is the long one');
 assert.equal(new Set(all).size,all.length,'no type twice');
 for(const ext of all)assert.match(ext,/^\.[a-z0-9][a-z0-9_-]*$/,ext+' is lower case, one extension');
 for(const group of list.groups())assert(group.extensions.length,'the group '+group.title+' is empty');
 for(const released of ['.txt','.log','.ini','.cfg','.conf'])assert(all.includes(released),released+' was registered by released versions');
 // Not text, or sometimes not text: these open through "Open with" only.
 const never=['.exe','.com','.dll','.msi','.scr','.lnk','.url','.zip','.7z','.rar','.gz','.tar','.iso','.png','.jpg','.jpeg','.gif','.bmp','.ico','.svg','.webp','.pdf','.rtf','.doc','.docx','.xls','.xlsx','.ppt','.pptx','.odt','.mp3','.mp4','.wav','.avi','.mkv','.bin','.dat','.db','.sqlite','.pcap','.pcapng','.key','.cer','.crt','.der','.pfx','.p12','.pub','.jar','.class','.pyc','.o','.obj','.lib'];
 for(const ext of never)assert(!all.includes(ext),ext+' is not a text format');
});

test('Setup registers exactly the list, from the one file',()=>{
 assert.match(code,/^!include "text-extensions\.nsh"$/m);
 assert.doesNotMatch(nsi,/!macro TextExtensions/,'the list is not repeated in windows.nsi');
 assert.equal((code.match(/!insertmacro TextExtensions RegisterExtension/g)||[]).length,1);
 assert((code.match(/!insertmacro TextExtensions RemoveExtension/g)||[]).length>=1);
 assert.doesNotMatch(code,/!insertmacro (?:RegisterExtension|RemoveExtension) /,'no type is registered outside the list');
 // The five types of released versions are named once more, for upgrades (checked below).
 assert.doesNotMatch(code.replace(/^\s*!insertmacro AdoptReleasedExtension "\.[a-z]+"$/gm,''),/"\.(?:txt|log|ini|cfg|conf|json|xml|py)"/,'no file type is written out in windows.nsi');
});

test('the hosted Setup check reads the same file and finds the same types',()=>{
 assert.match(ps1,/\$extensionList = Join-Path \$PSScriptRoot 'text-extensions\.nsh'/);
 assert.doesNotMatch(ps1,/\$extensions\s*=\s*'\./,'no list of its own');
 const pattern=/\[regex\]::Matches\(\[IO\.File\]::ReadAllText\(\$extensionList\), '\(\?m\)([^']+)'\)/.exec(ps1);
 assert(pattern,'the check reads the list with one regular expression');
 const found=[...fs.readFileSync(list.LIST,'utf8').matchAll(new RegExp(pattern[1],'gm'))].map(match=>match[1]);
 assert.deepEqual(found,all,'the expression in verify-windows.ps1 reads every type of the list');
 // What the check asserts about them and about the other registrations.
 for(const text of ['Assert-ClassKeysRestored','Get-EditorUserChoices','$applicationSubkey','$menuVerbSubkey',"'/S /DesktopShortcut /ContextMenu /D=$installDir'".replace(/'/g,'"'),'/S /ContextMenu /D=$lateFailureDir'])assert(ps1.includes(text),'verify-windows.ps1 no longer has '+text);
});

test('the guide shows the list as generated',()=>{
 assert.equal(list.guideBlock(),list.block(),'docs/guide/editor.md is out of date: run node scripts/text-extensions.cjs --write');
 assert.match(read('AGENTS.md'),/scripts\/installer\/text-extensions\.nsh/,'AGENTS.md names the one list');
});

test('Setup only offers the editor: per user, no UserChoice, no default value of a type, no type limit',()=>{
 assert.match(code,/^RequestExecutionLevel user$/m);
 assert.doesNotMatch(code,/UserChoice/i,'Setup never writes a UserChoice');
 assert.doesNotMatch(code,/FileExts/i,'Setup never writes the user\'s per-type choices');
 const writes=code.match(/^\s*(?:WriteReg\w+|DeleteRegKey|DeleteRegValue)\b.*$/gm)||[];
 assert(writes.length>40);
 for(const line of writes)assert.match(line,/^\s*(?:WriteReg\w+|DeleteRegKey(?: \/ifempty)?|DeleteRegValue) HKCU "/,'only the current user\'s registry is changed: '+line.trim());
 // Under a file type, the only thing written is the ProgID in OpenWithProgids.
 // Each macro's registry writes are compared as the exact list, in order: a
 // repeated, reordered or added write fails here.
 const writesOf=name=>{const body=new RegExp('^!macro '+name+' EXTENSION\\n([\\s\\S]*?)^!macroend','m').exec(code);assert(body,'windows.nsi has the macro '+name);return (body[1].match(/^\s*WriteReg\w+ .*$/gm)||[]).map(line=>line.trim());};
 assert.deepEqual(writesOf('RegisterExtension'),[
  'WriteRegStr HKCU "${CAPABILITIES}\\FileAssociations" "${EXTENSION}" "${PROG_ID}"',
  'WriteRegStr HKCU "Software\\Classes\\${EXTENSION}\\OpenWithProgids" "${PROG_ID}" ""',
  'WriteRegDWORD HKCU "${CREATED_KEYS}" "${EXTENSION}" 1',
  'WriteRegDWORD HKCU "${CREATED_KEYS}" "${EXTENSION}\\OpenWithProgids" 1']);
 // Adopting a released version's key writes a record and nothing under the type.
 assert.deepEqual(writesOf('AdoptReleasedExtension'),[
  'WriteRegDWORD HKCU "${CREATED_KEYS}" "${EXTENSION}" 1',
  'WriteRegDWORD HKCU "${CREATED_KEYS}" "${EXTENSION}\\OpenWithProgids" 1']);
 const typeWrites=writes.filter(line=>line.includes('${EXTENSION}')&&/^\s*WriteReg/.test(line));
 assert.equal(typeWrites.length,6,'no other macro writes under a file type');
 // A whole key under a file type is deleted only through the empty-key check.
 for(const line of writes.filter(line=>/DeleteRegKey/.test(line)&&line.includes('${EXTENSION}')))assert.fail('a file type key is deleted directly: '+line.trim());
 assert.doesNotMatch(code,/SupportedTypes/,'SupportedTypes would hide the editor for unknown file types');
 assert.doesNotMatch(code,/NoOpenWith/);
 assert.doesNotMatch(code,/OpenWithList/,'OpenWithProgids is the supported list');
});

test('every way Windows starts the editor with a file passes it as the one quoted value of --open-file',()=>{
 assert.match(code,/^!define EDITOR_ARGS "--editor-standalone"$/m);
 assert(code.includes('!define EDITOR_COMMAND \'"$INSTDIR\\${APP_FILENAME}" ${EDITOR_ARGS} --open-file "%1"\''),'the command is the quoted program, the mode and the quoted file');
 const commands=code.match(/^\s*WriteRegStr HKCU "[^"]*\\command" "" .*$/gm)||[];
 assert.deepEqual(commands.map(line=>/"(\$\{[A-Z_]+\})\\/.exec(line)[1]+' '+line.trim().split(' "" ')[1]).sort(),[
  "${APPLICATION_KEY} '${EDITOR_COMMAND}'",
  "${LINK_PROG_KEY} '\"$INSTDIR\\${APP_FILENAME}\" --terminal --open-url \"%1\"'",
  "${MENU_VERB_KEY} '${EDITOR_COMMAND}'",
  "${PROG_KEY} '${EDITOR_COMMAND}'"]);
 assert.equal((code.match(/%1/g)||[]).length,2,'"%1" appears in the editor command and the terminal link command only');
});

test('the right-click entry is optional, off unless chosen, remembered, and removed with the rest',()=>{
 assert.match(code,/^Section \/o "Right-click menu: \$\{MENU_TEXT\}" ContextMenu$/m,'an unselected optional section');
 assert.match(code,/\$\{GetOptions\} \$0 "\/ContextMenu" \$1/,'the switch for silent setup');
 // /NoContextMenu clears it in a silent setup; both together are refused through StopSetup.
 assert.match(code,/\$\{GetOptions\} \$0 "\/NoContextMenu" \$1\s+\$\{IfNot\} \$\{Errors\}\s+\$\{GetOptions\} \$0 "\/ContextMenu" \$1\s+\$\{IfNot\} \$\{Errors\}\s+!insertmacro StopSetup "Choose either \/ContextMenu or \/NoContextMenu, not both\."\s+\$\{EndIf\}\s+!insertmacro UnselectSection \$\{ContextMenu\}/);
 for(const text of ["'/S /ContextMenu /NoContextMenu' 'Choose either /ContextMenu or /NoContextMenu, not both'","Invoke-Silent $setup 0 '/S /NoContextMenu'"])assert(ps1.includes(text),'verify-windows.ps1 no longer has '+text);
 // An upgrade from a version without records adopts only the five released
 // types, and only on the first such upgrade.
 assert.deepEqual((code.match(/!insertmacro AdoptReleasedExtension "(\.[a-z]+)"/g)||[]).map(line=>/"(.+)"/.exec(line)[1]),['.txt','.log','.ini','.cfg','.conf']);
 assert.match(code,/\$\{If\} \$RegisteredInstallDir != ""\s+EnumRegKey \$0 HKCU "\$\{EDITOR_KEY\}" 0\s+\$\{If\} \$\{Errors\}\s+StrCpy \$AdoptReleased 1/,'adoption only when the installed version kept no record');
 assert(ps1.includes('adopts only the Open With keys that hold nothing but the Text Editor'),'the hosted check covers the upgrade from a released version');
 assert.match(code,/ReadRegDWORD \$0 HKCU "\$\{UNINSTALL_KEY\}" "ContextMenu"\s+\$\{If\} \$0 == 1\s+!insertmacro SelectSection \$\{ContextMenu\}/,'the choice is remembered on upgrade');
 assert.match(code,/^!define MENU_VERB_KEY "\$\{ALL_FILES_KEY\}\\shell\\NetworkToolkit\.TextEditor"$/m);
 const unregister=/!macro UnregisterShell\n([\s\S]*?)!macroend/.exec(code)[1];
 for(const step of ['TextExtensions RemoveExtension','RemoveApplication','RemoveContextMenu'])assert(unregister.includes('!insertmacro '+step),'uninstall and rollback run '+step);
 assert(unregister.indexOf('RemoveContextMenu')<unregister.indexOf('DeleteRegKey HKCU "${EDITOR_KEY}"'),'the record of created keys is read before it is removed');
 // Each new step names itself for the log of a failed install.
 for(const step of ['adding the Text Editor to Open with','updating the right-click menu entry'])assert(code.includes('StrCpy $FailedStep "'+step+'"'),step);
});

test('a type from the list is highlighted only by a language the editor already has; unknown types are plain text',()=>{
 const assets=path.join(list.ROOT,'web/assets'),context=vm.createContext({});
 for(const name of ['editor-vendor/languages.js','network-config-language.js','editor-languages.js'])vm.runInContext(fs.readFileSync(path.join(assets,name),'utf8'),context,{filename:name});
 const languages=context.NTKEditorLanguages,known=new Set(languages.languages.map(row=>row.id));
 const modes=Object.fromEntries(all.map(ext=>[ext,languages.detect('file'+ext,'')]));
 for(const [ext,id]of Object.entries(modes))assert(known.has(id),ext+' maps to '+id+', which is not a language of the editor');
 // The types that have no language of their own. A change here is a decision.
 assert.deepEqual(Object.keys(modes).filter(ext=>modes[ext]==='text').sort(),['.cisco','.csr','.gitattributes','.junos','.list','.nfo','.out','.pem','.rules','.text','.txt'].sort());
 for(const [ext,id]of [['.conf','ini'],['.log','log'],['.rsc','network'],['.ios','network'],['.json','json'],['.yml','yaml'],['.ps1','powershell'],['.gitignore','ignore'],['.reg','registry'],['.zone','dns-zone-file'],['.env','bash'],['.inf','ini'],['.gitmodules','git']])assert.equal(modes[ext],id,ext);
 for(const unknown of ['notes.zz9','running-config','archive.bak','README'])assert.equal(languages.detect(unknown,'plain words\n'),'text',unknown+' is plain text');
 if(process.env.NTK_PRINT_TEXT_EXTENSION_MODES)console.log(JSON.stringify(modes));
});
