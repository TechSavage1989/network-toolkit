/* Integrated, session-aware text editor for local, SFTP and FTP files.
   Nothing is autosaved. Remote saves use the authenticated file browser and
   refuse stale revisions rather than overwriting a file changed elsewhere. */
(function(root){'use strict';
const documents=new Map();let active='',selectedRemoteSource='',preferredRemotePath='',remoteSourceGeneration=0,embedded=false,showFind=false,showReplace=false,matchCase=false,useRegex=false,wholeWord=false,wordWrap=false,syntax=true,tabSpaces=4,overlay=null,updating=false,editorFontSize=13,editorPalette='auto',editorCustomColors={};
const escaped=x=>esc(String(x??''));
const findEl=sel=>overlay?.querySelector(sel);
const current=()=>documents.get(active);
const nameOf=p=>String(p||'Untitled').split(/[\\/]/).pop()||'Untitled';
const languageTools=root.NTKEditorLanguages;
const languageRows=languageTools?.languages||[{id:'text',label:'Plain text'}];
const languageFor=(path,content='')=>languageTools?.detect(path,content)||'text';
const languageOf=d=>d.language||languageFor(d.path,d.content);
let paintGeneration=0,paintTimer=null;
const editorPalettes=['auto','dark','light','pastel','forest','highContrast','custom'];
const editorColorLabels={bg:'Background',fg:'Normal text',comment:'Comments',string:'Strings',keyword:'Keywords',number:'Numbers',property:'Properties'};
const editorColorDefaults={bg:'#202535',fg:'#e5eaf4',comment:'#97c8aa',string:'#edcb98',keyword:'#a8bbff',number:'#ddb8f2',property:'#99dbe7'};
function goodEditorColor(value){return typeof value==='string'&&/^#[0-9a-f]{6}$/i.test(value);}
function colorPickerHTML(){return '<div class="nt-editor-palette-box">'+Object.entries(editorColorLabels).map(([k,label])=>'<label><span>'+label+'</span><input type="color" data-ed-color="'+k+'" value="'+(editorCustomColors[k]||editorColorDefaults[k])+'"><input type="text" data-ed-color-hex="'+k+'" data-input-clear="off" value="'+(editorCustomColors[k]||editorColorDefaults[k])+'" maxlength="7" spellcheck="false" aria-label="'+label+' hex color"></label>').join('')+'</div>';}
const editorPref=()=>root.state?.editorPreferences||{};
function readPreferences(){const v=typeof state==='undefined'?{}:state.editorPreferences||{};wordWrap=v.wordWrap===true;syntax=v.syntax!==false;tabSpaces=[0,2,4,8].includes(v.indent)?v.indent:4;editorFontSize=Number.isInteger(v.fontSize)&&v.fontSize>=10&&v.fontSize<=24?v.fontSize:13;editorPalette=editorPalettes.includes(v.palette)?v.palette:'auto';editorCustomColors=Object.fromEntries(Object.entries(v.colors||{}).filter(([key,value])=>Object.hasOwn(editorColorLabels,key)&&goodEditorColor(value)));}
function keepPreferences(){if(typeof state==='undefined')return;state.editorPreferences={wordWrap,syntax,indent:tabSpaces,fontSize:editorFontSize,palette:editorPalette,colors:editorCustomColors};if(typeof saveState==='function')saveState();}

const lineMode=s=>s.includes('\r\n')?'CRLF':s.includes('\r')?'CR':'LF';
const normalize=s=>String(s).replace(/\r\n|\r/g,'\n');
const outgoing=(d)=>d.content.replace(/\n/g,d.eol==='CRLF'?'\r\n':d.eol==='CR'?'\r':'\n');
function skin(){if(overlay)return;readPreferences();
 overlay=document.createElement('div');overlay.id='networkToolkitFileEditor';overlay.className='nt-editor';overlay.hidden=true;
 overlay.innerHTML='<div class="nt-editor-window" role="dialog" aria-modal="true" aria-label="Network Toolkit text editor"><div class="nt-editor-title"><strong>Text editor</strong><span class="nt-editor-filetitle" data-ed-title></span><button type="button" class="btn small ghost" data-ed="localTools" title="Open the full Text Editor in Tools">Full editor</button><button type="button" class="btn small ghost" data-ed="close" aria-label="Close text editor">Close</button></div>'+ 
 '<div class="nt-editor-tabs" data-ed-tabs role="tablist" aria-label="Open files"></div>'+ 
 '<div class="nt-editor-toolbar"><button class="btn small ghost" data-ed="new" title="New file (Ctrl+N)">New</button><button class="btn small ghost" data-ed="browse" title="Open a Windows file with editing permissions (Ctrl+O)">Open file</button><button class="btn small primary" data-ed="save" title="Save (Ctrl+S)">Save</button><button class="btn small ghost" data-ed="saveAs" title="Save As (Ctrl+Shift+S)">Save as</button><button class="btn small ghost" data-ed="reload">Reload</button><span class="nt-editor-rule"></span><button class="btn small ghost" data-ed="undo">Undo</button><button class="btn small ghost" data-ed="redo">Redo</button><button class="btn small ghost" data-ed="find">Find</button><button class="btn small ghost" data-ed="replace">Replace</button><button class="btn small ghost" data-ed="goto">Go to line</button><span class="nt-editor-rule"></span><button class="btn small ghost" data-ed="comment" title="Toggle comment (Ctrl+/)">Comment</button><button class="btn small ghost" data-ed="duplicate" title="Duplicate line (Ctrl+D)">Duplicate</button><button class="btn small ghost" data-ed="trim">Trim spaces</button><button class="btn small ghost" data-ed="upper">UPPER</button><button class="btn small ghost" data-ed="lower">lower</button><button class="btn small ghost" data-ed="sortLines">Sort lines</button><button class="btn small ghost" data-ed="format" title="Format document (Ctrl+Shift+F)">Format document</button><button class="btn small ghost" data-ed="download">Save copy</button><button type="button" class="btn small ghost" data-ed="colors" title="Customize syntax and editor colors">Text colors</button></div>'+ 
 '<div class="nt-editor-find" data-ed-find hidden><label>Find <input type="text" data-ed-query placeholder="Find text (Ctrl+F)"></label><button class="btn small ghost" data-ed="previous">Previous</button><button class="btn small ghost" data-ed="next">Next</button><label><input type="checkbox" data-ed-case> Case</label><label><input type="checkbox" data-ed-regex> Regex</label><label><input type="checkbox" data-ed-word> Word</label><div data-ed-replace-fields hidden class="nt-editor-replace-fields"><label>Replace <input type="text" data-ed-replacement placeholder="Replacement text"></label><button class="btn small ghost" data-ed="replaceOne">Replace</button><button class="btn small ghost" data-ed="replaceAll">Replace all</button></div><button type="button" class="btn small ghost" data-ed="hideFind">Hide</button></div>'+ 
 '<div class="nt-editor-options"><label>Line endings <select data-ed-eol><option>LF</option><option>CRLF</option><option>CR</option></select></label><label>Indent <select data-ed-indent><option value="4">4 spaces</option><option value="2">2 spaces</option><option value="0">Tabs</option></select></label><label><input data-ed-wrap type="checkbox"> Wrap</label><label><input data-ed-syntax type="checkbox" checked> Highlight</label><label>Language <select data-ed-language-select>'+'<option value="" data-ed-auto-language>Auto</option>'+languageRows.map(row=>'<option value="'+escaped(row.id)+'">'+escaped(row.label)+'</option>').join('')+'</select></label><label>Encoding <select data-ed-encoding><option>UTF-8</option><option>UTF-16LE</option><option>UTF-16BE</option></select></label><label><input type="checkbox" data-ed-bom> BOM</label><label>Font size <input type="number" data-ed-font data-input-clear="off" min="10" max="24" step="1" value="13"></label><label>Editor colors <select data-ed-palette><option value="auto">Follow app</option><option value="dark">Dark</option><option value="light">Light</option><option value="pastel">Pastel</option><option value="forest">Forest</option><option value="highContrast">High contrast</option><option value="custom">Custom</option></select></label></div><section class="nt-editor-color-controls" data-ed-custom-palette hidden><div class="row between"><strong>Custom editor colors</strong><button type="button" class="btn small ghost" data-ed="colors">Close</button></div><div data-ed-color-panel></div></section>'+ 
 '<div class="nt-editor-code"><div class="nt-editor-gutter" data-ed-gutter aria-hidden="true"></div><div class="nt-editor-surface"><pre class="nt-editor-highlight" data-ed-highlight aria-hidden="true"></pre><textarea data-ed-text data-input-clear="off" spellcheck="false" autocapitalize="off" autocomplete="off" autocorrect="off" wrap="off" aria-label="Editable file contents"></textarea></div></div>'+ 
 '<div class="nt-editor-status"><span data-ed-message></span><span data-ed-position></span><span data-ed-size></span></div></div>';
 document.body.append(overlay);
 findEl('[data-ed-color-panel]').innerHTML=colorPickerHTML();
 overlay.addEventListener('dragover',event=>{if(event.dataTransfer?.types?.includes('Files')){event.preventDefault();event.dataTransfer.dropEffect='copy';}});
 overlay.addEventListener('drop',event=>{const files=[...(event.dataTransfer?.files||[])];if(files.length){event.preventDefault();event.stopPropagation();void importFiles(files);}});
 overlay.addEventListener('click',click);overlay.addEventListener('change',change);overlay.addEventListener('input',input);overlay.addEventListener('keydown',keydown);overlay.addEventListener('scroll',scroll,true);
}
function setMessage(s,error=false){const el=findEl('[data-ed-message]');if(el){el.textContent=s;el.classList.toggle('error',error);}}
function setFileValues(d){d.editRevision=(d.editRevision||0)+1;d.eol=lineMode(d.original);d.content=normalize(d.original);d.saved=d.content;d.savedEol=d.eol;d.savedBOM=d.bom;d.savedEncoding=d.encoding||'UTF-8';d.dirty=false;d.undo=[];d.redo=[];d.bom=!!d.bom;d.showStart=true;}
async function open({id,source,path,read,write,afterSave,create=false,initialContent=''}){
 if(!path||typeof write!=='function'||typeof read!=='function')throw Error('Editor requires a file path and read/write functions.');skin();
 const key=String(id||source+':'+path),previous=active;
 let doc=documents.get(key);
 if(doc){active=key;doc.showStart=true;overlay.hidden=false;draw();if(doc.loading)return doc.loadingPromise;findEl('[data-ed-text]')?.focus();return;}
 if(documents.size>=12)throw Error('Close an editor tab before opening another file (12 maximum).');
 doc={id:key,path,source:source||'SFTP',read,write,afterSave,hash:'',encoding:'UTF-8',bom:false,original:initialContent,content:'',saved:'',dirty:false,eol:'LF',undo:[],redo:[],language:'',untitled:source==='Untitled',loading:!create};
 setFileValues(doc);documents.set(key,doc);active=key;overlay.hidden=false;applyEditorTheme();draw();
 if(!create){
  doc.loadingPromise=(async()=>{
   try{const result=await read(path);if(result.binary)throw Error('Binary files cannot be opened in the text editor.');
    if(documents.get(key)!==doc)return;
    doc.original=result.content||'';doc.hash=result.hash||'';doc.bom=result.bom===true;doc.encoding=result.encoding||'UTF-8';doc.loading=false;setFileValues(doc);
    if(active===key){draw();findEl('[data-ed-text]')?.focus();}
   }catch(error){
    if(documents.get(key)===doc){documents.delete(key);if(active===key){active=documents.has(previous)?previous:[...documents.keys()].at(-1)||'';if(active){draw();setMessage('Cannot open '+nameOf(path)+': '+error.message,true);}else overlay.hidden=true;}}
    throw error;
   }
  })();return doc.loadingPromise;
 }
 findEl('[data-ed-text]')?.focus();
}
function doUndo(d,redo=false){const from=redo?d.redo:d.undo,to=redo?d.undo:d.redo;const previous=from.pop();if(previous===undefined)return;to.push(d.content);d.content=previous;d.editRevision=(d.editRevision||0)+1;d.dirty=d.content!==d.saved||d.eol!==d.savedEol||d.bom!==d.savedBOM||d.encoding!==d.savedEncoding;syncText(d);}
function setContent(d,value,record=true){if(value===d.content)return;if(record){d.undo.push(d.content);if(d.undo.length>80)d.undo.shift();d.redo=[];}d.content=value;d.editRevision=(d.editRevision||0)+1;d.dirty=d.content!==d.saved||d.eol!==d.savedEol||d.bom!==d.savedBOM||d.encoding!==d.savedEncoding;paintText(d);drawTabs();status();}
function syncText(d){const el=findEl('[data-ed-text]');if(el)el.value=d.content;paintText(d);drawTabs();status();}
function status(){const d=current(),text=findEl('[data-ed-text]');if(!d||!text)return;const pos=text.selectionStart,left=d.content.slice(0,pos),line=left.split('\n').length,column=pos-(left.lastIndexOf('\n')+1)+1;findEl('[data-ed-position]').textContent='Ln '+line+', Col '+column;findEl('[data-ed-size]').textContent=d.content.length.toLocaleString()+' chars, '+d.content.split('\n').length.toLocaleString()+' lines'+(d.dirty?' · Unsaved':'');}
function drawTabs(){const tabs=findEl('[data-ed-tabs]');if(!tabs)return;tabs.innerHTML=[...documents.values()].map(d=>'<div class="nt-editor-tab" role="presentation"><button type="button" role="tab" data-ed-open="'+escaped(d.id)+'" aria-selected="'+(active===d.id)+'" title="'+escaped(d.path)+'">'+escaped(nameOf(d.path))+(d.dirty?' *':'')+'</button><button type="button" data-ed-close-tab="'+escaped(d.id)+'" aria-label="Close '+escaped(nameOf(d.path))+'">×</button></div>').join('');}
// Paint a plain escaped copy immediately. Late worker replies must match the
// same document, text, language and paint generation before touching the UI.
function paintText(d){const ta=findEl('[data-ed-text]'),pre=findEl('[data-ed-highlight]'),gutter=findEl('[data-ed-gutter]');if(!ta||!pre||!gutter||current()!==d)return;
 const generation=++paintGeneration;if(paintTimer!==null){root.clearTimeout(paintTimer);paintTimer=null;}
 const n=Math.min(30000,d.content.split('\n').length);gutter.textContent=Array.from({length:n},(_,i)=>String(i+1)).join('\n');
 const show=syntax&&!wordWrap&&d.content.length<=250000;pre.hidden=!show;ta.classList.toggle('highlight-active',show);
 const language=languageOf(d),snapshot=d.content;
 if(show){
  if(d.syntaxCache?.text===snapshot&&d.syntaxCache.language===language)pre.innerHTML=d.syntaxCache.html+'\n';
  else{pre.textContent=snapshot+'\n';if(snapshot&&language!=='text'&&root.NTKEditorLanguageService){paintTimer=root.setTimeout(()=>{paintTimer=null;
   void root.NTKEditorLanguageService.highlight(snapshot,language).then(result=>{
    if(current()!==d||documents.get(d.id)!==d||d.content!==snapshot||languageOf(d)!==language||generation!==paintGeneration||!syntax||wordWrap)return;
    d.syntaxCache={text:snapshot,language,html:result.html};pre.innerHTML=result.html+'\n';pre.title='';
   }).catch(()=>{if(generation===paintGeneration)pre.title='Showing plain text while syntax tools are unavailable.';});
  },70);}}
 }else root.NTKEditorLanguageService?.cancelHighlight?.();
 pre.style.transform='translate('+(-ta.scrollLeft)+'px,'+(-ta.scrollTop)+'px)';gutter.style.transform='translateY('+(-ta.scrollTop)+'px)';
 syncLanguageControls(d);
}
function syncLanguageControls(d){const selected=findEl('[data-ed-language-select]'),automatic=findEl('[data-ed-auto-language]'),language=languageOf(d),button=findEl('[data-ed="format"]'),kind=languageTools?.formatKind(language)||'';
 if(selected)selected.value=d.language||'';if(automatic)automatic.textContent='Auto ('+(languageTools?.label(languageFor(d.path,d.content))||'Plain text')+')';
 if(button){button.disabled=!!d.loading||!!d.formatting||!kind||d.content.length>250000;button.textContent=kind==='indent'?'Indent document':'Format document';button.title=kind==='indent'?'Adjust code indentation, preserving strings and comments (Ctrl+Shift+F)':kind?'Format '+languageTools.label(language)+' (Ctrl+Shift+F)':'Automatic formatting is unavailable for this language.';}
 if(overlay)overlay.style.setProperty('--nt-editor-tab',String(tabSpaces||4));
}
function draw(){const d=current();if(!d)return;drawTabs();const ta=findEl('[data-ed-text]');ta.disabled=!!d.loading;if(ta.value!==d.content)ta.value=d.content;ta.wrap=wordWrap?'soft':'off';ta.classList.toggle('is-wrapped',wordWrap);findEl('[data-ed-title]').textContent=d.source+' · '+d.path;findEl('[data-ed-eol]').value=d.eol;findEl('[data-ed-indent]').value=String(tabSpaces);findEl('[data-ed-wrap]').checked=wordWrap;findEl('[data-ed-syntax]').checked=syntax;findEl('[data-ed-bom]').checked=d.bom;findEl('[data-ed-encoding]').value=d.encoding;findEl('[data-ed-language-select]').value=d.language||'';findEl('[data-ed-font]').value=String(editorFontSize);findEl('[data-ed-palette]').value=editorPalette;applyEditorTheme();findEl('[data-ed-find]').hidden=!showFind;findEl('[data-ed-replace-fields]').hidden=!showReplace;setMessage(d.loading?'Opening '+nameOf(d.path)+'...':'Ready');for(const button of overlay.querySelectorAll('[data-ed="save"],[data-ed="saveAs"],[data-ed="reload"]'))button.disabled=!!d.loading;if(d.showStart){ta.setSelectionRange(0,0);ta.focus();ta.scrollTop=0;ta.scrollLeft=0;d.showStart=false;}paintText(d);status();}
async function save(as=false){const d=current();if(!d||d.loading)return;as=as||d.untitled||d.source==='Desktop import';let path=d.path;
 if(as&&d.source==='Desktop'){exportCopy(d);return;}
 if(as){const other=prompt('Save as a new path. Existing destination files will not be overwritten.',d.path);if(other===null)return;path=other.trim();if(!path||path.length>4096||path===d.path){setMessage('Choose a different path for Save As.',true);return;}}
 if(updating)return;updating=true;setMessage('Saving '+path+'...');
 try{const result=await d.write({path,content:outgoing(d),expectedHash:as?'':d.hash,bom:d.bom,encoding:d.encoding});d.hash=result.hash||d.hash;d.saved=d.content;d.savedEol=d.eol;d.savedBOM=d.bom;d.savedEncoding=d.encoding;d.untitled=false;d.dirty=false;
  if(as){const oldId=d.id;documents.delete(oldId);if(d.source==='Desktop import'){d.source='Local computer';d.read=p=>api('/file-editor/local','POST',{action:'read-text',path:p});d.write=data=>api('/file-editor/local','POST',{action:'write-text',...data});}d.path=path;d.id=d.source+':'+path;documents.set(d.id,d);active=d.id;}
  drawTabs();status();setMessage('Saved '+new Date().toLocaleTimeString());if(typeof d.afterSave==='function')void d.afterSave();}
 catch(e){setMessage('Save failed: '+e.message,true);}finally{updating=false;}
}
function closeTab(key){const d=documents.get(key);if(!d)return;if(d.dirty&&!confirm('Discard unsaved changes to '+nameOf(d.path)+'?'))return;documents.delete(key);if(active===key)active=[...documents.keys()].at(-1)||'';if(active)draw();else overlay.hidden=true;}
function closeAll(){if(embedded){detachPage();}const dirty=[...documents.values()].filter(d=>d.dirty);if(dirty.length&&!confirm('Close editor and discard unsaved changes in '+dirty.length+' file(s)?'))return;documents.clear();active='';overlay.hidden=true;paintGeneration++;if(paintTimer!==null){root.clearTimeout(paintTimer);paintTimer=null;}root.NTKEditorLanguageService?.dispose?.();}
function replaceSelection(value){const el=findEl('[data-ed-text]'),d=current();if(!el||!d)return;const a=el.selectionStart,b=el.selectionEnd,next=d.content.slice(0,a)+value+d.content.slice(b);setContent(d,next);el.value=d.content;el.focus();el.setSelectionRange(a+value.length,a+value.length);}
function search(backwards=false,replace=false,all=false){const d=current(),ta=findEl('[data-ed-text]'),query=findEl('[data-ed-query]')?.value||'';if(!d||!ta||!query)return;
 let pattern;try{const s=useRegex?query:query.replace(/[.*+?^${}()|[\]\\]/g,'\\$&');pattern=new RegExp(wholeWord?'\\b(?:'+s+')\\b':s,matchCase?'g':'gi');}catch(e){setMessage('Invalid search: '+e.message,true);return;}
 if(all){if(!replace)return;let count=0;const next=d.content.replace(pattern,(...args)=>{count++;return findEl('[data-ed-replacement]')?.value||'';});setContent(d,next);ta.value=d.content;setMessage('Replaced '+count+' occurrence(s).');return;}
 const results=[...d.content.matchAll(pattern)].filter(m=>m[0].length>0);if(!results.length){setMessage('No matches.');return;}
 const at=ta.selectionStart;const match=backwards?([...results].reverse().find(m=>m.index<at)||results.at(-1)):(results.find(m=>m.index>=Math.max(at,ta.selectionEnd>at?ta.selectionEnd:at+1))||results[0]);
 if(replace){const chosen=results.find(m=>m.index===ta.selectionStart&&m.index+m[0].length===ta.selectionEnd);if(chosen){replaceSelection(findEl('[data-ed-replacement]')?.value||'');return;}}
 ta.focus();ta.setSelectionRange(match.index,match.index+match[0].length);const before=d.content.slice(0,match.index);ta.scrollTop=Math.max(0,(before.split('\n').length-5)*parseFloat(getComputedStyle(ta).lineHeight));status();setMessage(results.length+' match(es).');
}
function linesAction(kind){const d=current(),ta=findEl('[data-ed-text]');if(!d||!ta||d.loading)return;const a=ta.selectionStart,b=ta.selectionEnd,start=d.content.lastIndexOf('\n',a-1)+1,limit=b>a&&d.content[b-1]==='\n'?b-1:b,last=d.content.indexOf('\n',limit),end=last<0?d.content.length:last,part=d.content.slice(start,end),lines=part.split('\n');
 let next=part;if(kind==='comment'){next=languageTools?.toggleComment(part,languageOf(d));if(next===null||next===undefined){setMessage('This language does not define comment syntax.');return;}}if(kind==='duplicate')next=part+'\n'+part;if(kind==='trim')next=lines.map(line=>line.replace(/[ \t]+$/,'')).join('\n');
 setContent(d,d.content.slice(0,start)+next+d.content.slice(end));ta.value=d.content;ta.focus();ta.setSelectionRange(start,start+next.length);}
function indentSelection(unindent=false){const d=current(),ta=findEl('[data-ed-text]');if(!d||!ta||d.loading)return;const a=ta.selectionStart,b=ta.selectionEnd;
 if(a===b&&!unindent){replaceSelection(tabSpaces?' '.repeat(tabSpaces):'\t');return;}
 const start=d.content.lastIndexOf('\n',a-1)+1,limit=b>a&&d.content[b-1]==='\n'?b-1:b,last=d.content.indexOf('\n',limit),end=last<0?d.content.length:last,part=d.content.slice(start,end),unit=tabSpaces?' '.repeat(tabSpaces):'\t';
 const transformed=part.split('\n').map(line=>unindent?line.replace(new RegExp('^(?:\\t| {1,'+(tabSpaces||4)+'})'),''):unit+line).join('\n');
 setContent(d,d.content.slice(0,start)+transformed+d.content.slice(end));ta.value=d.content;ta.focus();ta.setSelectionRange(start,start+transformed.length);
}
function exportCopy(d){if(!d)return;const blob=new Blob([encodeDesktopText({content:outgoing(d),encoding:d.encoding,bom:d.bom})],{type:'application/octet-stream'});const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=nameOf(d.path);a.click();setTimeout(()=>URL.revokeObjectURL(url),3000);setMessage('Saved a copy to your browser downloads.');}
function click(ev){const tab=ev.target.closest('[data-ed-open]');if(tab){active=tab.dataset.edOpen;draw();return;}const closing=ev.target.closest('[data-ed-close-tab]');if(closing){closeTab(closing.dataset.edCloseTab);return;}
 const el=ev.target.closest('[data-ed]');if(!el)return;ev.preventDefault();switch(el.dataset.ed){
 case 'colors':{const panel=findEl('[data-ed-custom-palette]');if(panel)panel.hidden=!panel.hidden;break;}
 case 'close':if(embedded){if(typeof navigate==='function'){navigate('local');}else closeAll();}else closeAll();break;case 'new':void newDocument();break;case 'browse':void openDesktopFile();break;case 'localTools':if(typeof navigate==='function'){navigate('editor');}break;case 'save':void save();break;case 'saveAs':void save(true);break;case 'reload':void reload();break;
 case 'undo':doUndo(current());break;case 'redo':doUndo(current(),true);break;
 case 'find':showFind=true;showReplace=false;draw();findEl('[data-ed-query]').focus();break;
 case 'replace':showFind=true;showReplace=true;draw();findEl('[data-ed-query]').focus();break;
 case 'hideFind':showFind=false;showReplace=false;draw();break;
 case 'next':search();break;case 'previous':search(true);break;case 'replaceOne':search(false,true);break;case 'replaceAll':search(false,true,true);break;
 case 'upper':transformSelection('upper');break;case 'lower':transformSelection('lower');break;case 'sortLines':transformSelection('sort');break;case 'format':void formatDocument();break;case 'comment':linesAction('comment');break;case 'duplicate':linesAction('duplicate');break;case 'trim':linesAction('trim');break;
 case 'goto':{const d=current(),ta=findEl('[data-ed-text]'),val=prompt('Go to line number:',String(d.content.slice(0,ta.selectionStart).split('\n').length));if(val===null)return;const line=Number(val);if(!Number.isSafeInteger(line)||line<1||line>d.content.split('\n').length)return;const start=d.content.split('\n').slice(0,line-1).reduce((sum,text)=>sum+text.length+1,0);ta.focus();ta.setSelectionRange(start,start);ta.scrollTop=(line-2)*parseFloat(getComputedStyle(ta).lineHeight);status();break;}
 case 'download':{exportCopy(current());break;}
 }}
function input(ev){if(ev.target.matches('[data-ed-text]')){const d=current();if(d)setContent(d,ev.target.value);}else if(ev.target.matches('[data-ed-query]')&&ev.target.value){search();}}
function change(ev){const t=ev.target;if(t.matches?.('[data-ed-color]')){setEditorColor(t.dataset.edColor,t.value);return;}if(t.matches?.('[data-ed-color-hex]')){if(goodEditorColor(t.value.trim())){t.setCustomValidity('');setEditorColor(t.dataset.edColorHex,t.value.trim());}else{t.setCustomValidity('Use #RRGGBB');t.reportValidity();}return;}const d=current();if(!d)return;if(t.matches('[data-ed-eol]')){d.eol=t.value;d.dirty=d.content!==d.saved||d.eol!==d.savedEol||d.bom!==d.savedBOM||d.encoding!==d.savedEncoding;drawTabs();status();}if(t.matches('[data-ed-bom]')){d.bom=t.checked;d.dirty=d.content!==d.saved||d.eol!==d.savedEol||d.bom!==d.savedBOM||d.encoding!==d.savedEncoding;drawTabs();status();}if(t.matches('[data-ed-encoding]')){d.encoding=t.value;d.bom=t.value!=='UTF-8'?true:d.bom;d.dirty=true;drawTabs();}if(t.matches('[data-ed-language-select]')){d.language=t.value?languageTools?.valid(t.value)||'text':'';paintText(d);}if(t.matches('[data-ed-font]')){editorFontSize=Math.max(10,Math.min(24,Number(t.value)||13));applyEditorTheme();keepPreferences();}if(t.matches('[data-ed-palette]')){editorPalette=t.value;applyEditorTheme();keepPreferences();if(t.value==='custom')findEl('[data-ed-custom-palette]').hidden=false;}if(t.matches('[data-ed-indent]')){tabSpaces=Number(t.value);syncLanguageControls(d);}if(t.matches('[data-ed-wrap]')){wordWrap=t.checked;draw();}if(t.matches('[data-ed-syntax]')){syntax=t.checked;paintText(d);}if(t.matches('[data-ed-case]'))matchCase=t.checked;if(t.matches('[data-ed-regex]'))useRegex=t.checked;if(t.matches('[data-ed-word]'))wholeWord=t.checked;if(t.matches('[data-ed-wrap],[data-ed-syntax],[data-ed-indent]'))keepPreferences();}
function scroll(ev){if(!ev.target.matches?.('[data-ed-text]'))return;const ta=ev.target,pre=findEl('[data-ed-highlight]'),gutter=findEl('[data-ed-gutter]');if(pre)pre.style.transform='translate('+(-ta.scrollLeft)+'px,'+(-ta.scrollTop)+'px)';if(gutter)gutter.style.transform='translateY('+(-ta.scrollTop)+'px)';status();}
function keydown(ev){const d=current();if(!d)return;const ta=findEl('[data-ed-text]'),ctrl=ev.ctrlKey||ev.metaKey;
 if(ctrl){const key=ev.key.toLowerCase();if(key==='f'&&ev.shiftKey){ev.preventDefault();void formatDocument();return;}if(key==='o'){ev.preventDefault();void openDesktopFile();return;}if(key==='n'){ev.preventDefault();void newDocument();return;}if(key==='s'){ev.preventDefault();void save(ev.shiftKey);return;}if(key==='f'){ev.preventDefault();showFind=true;showReplace=false;draw();findEl('[data-ed-query]').focus();return;}if(key==='h'){ev.preventDefault();showFind=true;showReplace=true;draw();findEl('[data-ed-query]').focus();return;}if(key==='z'){ev.preventDefault();doUndo(d,ev.shiftKey);return;}if(key==='y'){ev.preventDefault();doUndo(d,true);return;}if(key==='d'){ev.preventDefault();linesAction('duplicate');return;}if(key==='/'){ev.preventDefault();linesAction('comment');return;}}
 if(ev.key==='Escape'&&ev.target!==ta){showFind=false;showReplace=false;draw();ta.focus();return;}
 if(ev.target!==ta)return;
 if(ev.key==='Tab'){ev.preventDefault();indentSelection(ev.shiftKey);return;}
 if(ev.key==='Enter'&&!ev.isComposing){const insert=languageTools?.newline(d.content,ta.selectionStart,languageOf(d),tabSpaces?' '.repeat(tabSpaces):'\t');if(insert){ev.preventDefault();replaceSelection(insert.text);if(insert.back){const at=ta.selectionStart-insert.back;ta.setSelectionRange(at,at);}status();}return;}
 if(ev.key==='Home'&&ev.ctrlKey===false){status();}
}
async function reload(){const d=current();if(!d)return;if(d.dirty&&!confirm('Discard unsaved changes and reload '+nameOf(d.path)+'?'))return;try{const result=await d.read(d.path);if(documents.get(d.id)!==d)return;d.original=result.content||'';d.hash=result.hash||'';d.bom=!!result.bom;d.encoding=result.encoding||'UTF-8';setFileValues(d);if(current()===d)draw();}catch(e){if(current()===d)setMessage(e.message,true);}}
function applyEditorTheme(){if(!overlay)return;overlay.dataset.palette=editorPalette;overlay.style.setProperty('--nt-editor-font',editorFontSize+'px');for(const [key,hex] of Object.entries({...editorColorDefaults,...editorCustomColors})){if(editorPalette==='custom'&&goodEditorColor(hex))overlay.style.setProperty('--nt-code-'+key,hex);else overlay.style.removeProperty('--nt-code-'+key);} }
function setEditorColor(key,value){if(!Object.hasOwn(editorColorLabels,key)||!goodEditorColor(value))return;editorCustomColors[key]=value.toLowerCase();editorPalette='custom';const selector=findEl('[data-ed-palette]');if(selector)selector.value='custom';for(const element of overlay.querySelectorAll('[data-ed-color="'+key+'"],[data-ed-color-hex="'+key+'"]'))element.value=editorCustomColors[key];applyEditorTheme();keepPreferences();}

function transformSelection(kind){const d=current(),ta=findEl('[data-ed-text]');if(!d||!ta)return;const a=ta.selectionStart,b=ta.selectionEnd;if(kind==='upper'||kind==='lower'){const text=d.content.slice(a,b);if(!text){setMessage('Select text to change its case.');return;}replaceSelection(kind==='upper'?text.toLocaleUpperCase():text.toLocaleLowerCase());return;}
 const start=d.content.lastIndexOf('\n',a-1)+1,endIndex=d.content.indexOf('\n',b),end=endIndex<0?d.content.length:endIndex;
 const original=d.content.slice(start,end),lines=original.split('\n'),sorted=lines.sort((x,y)=>x.localeCompare(y,undefined,{numeric:true,sensitivity:'base'}));setContent(d,d.content.slice(0,start)+sorted.join('\n')+d.content.slice(end));ta.value=d.content;ta.focus();}
async function formatDocument(){const d=current(),ta=findEl('[data-ed-text]');if(!d||!ta||d.loading||d.formatting)return;const language=languageOf(d),kind=languageTools?.formatKind(language);if(!kind){setMessage('Automatic formatting is unavailable for this language.');return;}
 const snapshot=d.content,revision=d.editRevision,selection={start:ta.selectionStart,end:ta.selectionEnd,top:ta.scrollTop,left:ta.scrollLeft};d.formatting=true;syncLanguageControls(d);setMessage(kind==='indent'?'Indenting code...':'Formatting document...');
 try{const result=await root.NTKEditorLanguageService.format(snapshot,language,{indent:tabSpaces});
  if(documents.get(d.id)!==d||current()!==d||d.content!==snapshot||d.editRevision!==revision||languageOf(d)!==language){if(current()===d)setMessage('Document changed while formatting. Run the formatter again.');return;}
  setContent(d,result.text);ta.value=d.content;ta.setSelectionRange(Math.min(selection.start,d.content.length),Math.min(selection.end,d.content.length));ta.focus({preventScroll:true});ta.scrollTop=selection.top;ta.scrollLeft=selection.left;scroll({target:ta});status();setMessage(result.mode==='indent'?'Code indentation updated.':'Document formatted.');
 }catch(error){if(current()===d)setMessage('Cannot format: '+error.message,true);}
 finally{d.formatting=false;if(current()===d)syncLanguageControls(d);}
}
// A desktop File is deliberately not assumed to expose an absolute path.
// Writable File System Access handles are used when the user explicitly chooses a file.
async function readDesktopBuffer(file){if(file.size>8388608)throw Error('The text editor supports files up to 8 MiB.');const bytes=new Uint8Array(await file.arrayBuffer());let encoding='UTF-8',bom=false,offset=0;
 if(bytes.length>=2&&((bytes[0]===255&&bytes[1]===254)||(bytes[0]===254&&bytes[1]===255))){encoding=bytes[0]===255?'UTF-16LE':'UTF-16BE';offset=2;bom=true;}
 else if(bytes.length>=3&&bytes[0]===239&&bytes[1]===187&&bytes[2]===191){offset=3;bom=true;}
 const text=new TextDecoder(encoding.toLowerCase(),{fatal:true}).decode(bytes.subarray(offset));if(text.includes('\0'))throw Error('Binary file cannot be edited safely.');
 const hash=await crypto.subtle.digest('SHA-256',bytes);return {content:text,encoding,bom,hash:[...new Uint8Array(hash)].map(x=>x.toString(16).padStart(2,'0')).join('')};
}
function encodeDesktopText(data){const e=data.encoding||'UTF-8',text=String(data.content);if(text.includes('\0'))throw Error('NUL bytes cannot be written.');if(e==='UTF-8'){const body=new TextEncoder().encode(text);return data.bom?new Uint8Array([239,187,191,...body]):body;}
 const little=e==='UTF-16LE';if(!little&&e!=='UTF-16BE')throw Error('Unsupported encoding');const bytes=new Uint8Array(text.length*4+2),view=new DataView(bytes.buffer);let i=0;if(data.bom){bytes[0]=little?255:254;bytes[1]=little?254:255;i=2;}for(const ch of text){const n=ch.codePointAt(0);if(n<65536){view.setUint16(i,n,little);i+=2;}else{const a=n-65536;view.setUint16(i,0xd800+(a>>10),little);i+=2;view.setUint16(i,0xdc00+(a&1023),little);i+=2;}}
 return bytes.slice(0,i);
}
async function importFiles(files,handles=[]){for(let i=0;i<Math.min(files.length,12);i++){const file=files[i],handle=handles[i]||null;try{const snapshot=await readDesktopBuffer(file);const key='desktop:'+Date.now()+':'+i+':'+file.name;let lastHash=snapshot.hash;
 await open({id:key,path:file.name,source:handle?'Desktop':'Desktop import',create:true,initialContent:snapshot.content,
 read:async()=>{const snapshotNext=await readDesktopBuffer(handle?await handle.getFile():file);lastHash=snapshotNext.hash;return snapshotNext;},
 write:async data=>{if(!handle){if(!/^(?:[a-z]:[\\/]|\\\\|\/)/i.test(data.path))throw Error('Save As requires a full local path. Use Save copy to export a downloaded copy.');return api('/file-editor/local','POST',{action:'write-text',...data});}
  const currentFile=await readDesktopBuffer(await handle.getFile());if(data.expectedHash!==lastHash||currentFile.hash!==lastHash)throw Error('Desktop file changed outside the editor. Reload before saving.');const content=encodeDesktopText(data);const writer=await handle.createWritable();try{await writer.write(content);await writer.close();}catch(e){await writer.abort().catch(()=>{});throw e;}const updated=await readDesktopBuffer(await handle.getFile());lastHash=updated.hash;return {hash:lastHash};}});
 const d=current();d.hash=snapshot.hash;d.bom=snapshot.bom;d.encoding=snapshot.encoding;d.savedEncoding=d.encoding;d.savedBOM=d.bom;draw();setMessage(handle?'Opened writable desktop file.':'Imported desktop file. Use Save copy to export changes.');
 }catch(e){if(typeof toast==='function')toast(file.name+': '+e.message,'error');else setMessage(e.message,true);}}}
async function openDesktopFile(){if(typeof root.showOpenFilePicker==='function'){try{const handles=await root.showOpenFilePicker({multiple:true,excludeAcceptAllOption:false});await importFiles(await Promise.all(handles.map(h=>h.getFile())),handles);return;}catch(e){if(e.name==='AbortError')return;setMessage('File picker: '+e.message,true);}}
 const input=document.createElement('input');input.type='file';input.multiple=true;input.addEventListener('change',()=>void importFiles([...input.files]),{once:true});input.click();}
async function newDocument(){const key='untitled:'+Date.now()+':'+Math.random().toString(36).slice(2,7);await open({id:key,source:'Untitled',path:'Untitled.txt',create:true,initialContent:'',read:async()=>({content:''}),write:data=>api('/file-editor/local','POST',{action:'write-text',...data})});setMessage('New file. Save asks for an absolute local path.');}
async function openLocalPath(path,create=false){const text=String(path||'').trim();if(!text)throw Error('Enter a full path to a local text file.');return open({id:'local:'+text,source:'Local computer',path:text,create,initialContent:'',read:p=>api('/file-editor/local','POST',{action:'read-text',path:p}),write:data=>api('/file-editor/local','POST',{action:'write-text',...data})});}
function detachPage(){if(!overlay||!embedded)return;embedded=false;overlay.classList.remove('nt-editor-embedded');overlay.removeAttribute('role');document.body.appendChild(overlay);overlay.hidden=true;}
function preferConnection(source,path=''){selectedRemoteSource=String(source||'');preferredRemotePath=String(path||'');}
async function refreshConnectedSources(){
 const picker=document.getElementById('ntEditorRemoteSource');if(!picker)return;
 const generation=++remoteSourceGeneration,options=[],browser=root.NetworkToolkitRemoteBrowser;
 for(const [kind,label] of [['ssh','SFTP client'],['ftp','FTP / FTPS client']]){
  const model=browser?.get?.(kind);
  if(!model?.connected)continue;
  const config=browser.configurations[kind];
  options.push({id:'client:'+kind,label:label+' - '+(model.sessionId?'shared SSH terminal':config?.username+'@'+config?.host+':'+config?.port)});
 }
 let failure='';
 if(!OFFLINE&&online){
  try{
   const live=await api('/remote-browser');
   for(const row of live.sessions||[]){if(row.id&&row.host)options.push({id:'terminal:'+row.id,label:'SSH terminal - '+(row.label||row.host)+' ('+(row.username||'')+'@'+row.host+':'+row.port+')'});}
  }catch(error){failure='SSH sessions could not be listed: '+error.message;}
 }
 if(generation!==remoteSourceGeneration||picker!==document.getElementById('ntEditorRemoteSource'))return;
 const wanted=selectedRemoteSource||picker.value||'';
 picker.innerHTML='<option value="">Select a connected SFTP, FTP / FTPS or SSH session</option>'+options.map(row=>'<option value="'+escaped(row.id)+'">'+escaped(row.label)+'</option>').join('');
 const active=options.some(row=>row.id===wanted)?wanted:'';
 picker.value=active;
 selectedRemoteSource=active;
 const path=document.getElementById('ntEditorRemotePath');
 if(path&&preferredRemotePath){path.value=preferredRemotePath;preferredRemotePath='';}
 const openButton=document.getElementById('ntEditorOpenRemote');if(openButton)openButton.disabled=!active;
 const newButton=document.getElementById('ntEditorNewRemote');if(newButton)newButton.disabled=!active;
 const hint=document.getElementById('ntEditorRemoteHint');if(hint)hint.textContent=failure||(!options.length?'No active remote sessions. Connect through Terminal or the SFTP / FTP / FTPS browser first.':'Uses the selected authenticated connection. Saving checks for external file changes.');
}
async function openRemotePath(path,create=false){
 const choice=document.getElementById('ntEditorRemoteSource')?.value||'';
 const remote=String(path||'').trim();
 if(!choice)throw Error('Choose an active remote connection first.');
 if(!remote||remote.length>4096)throw Error('Enter the full remote file path.');
 if(choice.startsWith('client:')){
  const kind=choice.slice(7),browser=root.NetworkToolkitRemoteBrowser;
  if(!['ssh','ftp'].includes(kind)||!browser?.get?.(kind)?.connected)throw Error('This file browser is no longer connected. Reconnect and refresh.');
  await browser.edit(kind,create,'remote',remote);
  return;
 }
 if(!choice.startsWith('terminal:'))throw Error('Invalid remote connection.');
 const id=choice.slice(9);
 const alive=async()=>{
  const rows=(await api('/remote-browser')).sessions||[];
  const found=rows.find(row=>row.id===id);
  if(!found)throw Error('This SSH terminal has disconnected or moved. Reconnect before editing.');
  return found;
 };
 const session=await alive(),revision=session.revision;
 const verify=async()=>{
  const current=await alive();
  if(current.revision!==revision)throw Error('This SSH session has changed. Reopen the file before saving.');
 };
 const request=async(action,body)=>{
  await verify();
  return api('/remote-browser','POST',{protocol:'ssh',sessionId:id,action,...body});
 };
 await open({id:'connected-terminal:'+id+':'+revision+':'+remote,path:remote,source:'SSH terminal - '+(session.label||session.host),create,
  read:p=>request('read-text',{path:p}),
  write:data=>request('write-text',data)});
}
/* File picker for the active authenticated connection. Unlike a path-only
   field, this lists remote directories without storing credentials. */
let filePicker=null;
function filePickerParent(path){const p=String(path||'.').replace(/\/$/,'');if(!p||p==='.'||p==='/')return p==='/'?'/':'.';const i=p.lastIndexOf('/');return i<0?'.':i===0?'/':p.slice(0,i);}
async function filePickerRequest(source,path){
 const browser=root.NetworkToolkitRemoteBrowser;
 if(source.startsWith('client:')){
  const kind=source.slice(7);
  if(!browser?.get?.(kind)?.connected)throw Error('The selected SFTP or FTP/FTPS session is disconnected.');
  return browser.request(kind,'list',{path});
 }
 if(source.startsWith('terminal:')){
  const id=source.slice(9),known=(await api('/remote-browser')).sessions||[];
  if(!known.some(x=>x.id===id))throw Error('The selected SSH terminal has disconnected.');
  return api('/remote-browser','POST',{protocol:'ssh',sessionId:id,action:'list',path});
 }
 throw Error('Choose a connected remote session.');
}
function filePickerDraw(){if(!filePicker)return;
 const list=document.querySelector('#ntEditorFilePickerRows'),label=document.querySelector('#ntEditorFilePickerPath');if(!list||!label)return;
 label.value=filePicker.path;
 list.innerHTML=filePicker.entries.length?filePicker.entries.filter(x=>x&&typeof x.path==='string').map(x=>'<button type="button" class="nt-editor-picker-row'+(filePicker.selected===x.path?' selected':'')+'" data-ed-picker-path="'+escaped(x.path)+'" data-ed-picker-dir="'+String(!!x.directory)+'" title="'+escaped(x.path)+'"><span>'+icon(x.directory?'folder':'file')+'</span><span>'+escaped(x.name)+'</span><small>'+escaped(x.directory?'Folder':typeof x.size==='number'?x.size.toLocaleString()+' bytes':'File')+'</small></button>').join(''):'<p class="muted">This folder has no files.</p>';
 const button=document.querySelector('[data-ed-picker-open]');if(button)button.disabled=!filePicker.selected||!!filePicker.entries.find(x=>x.path===filePicker.selected&&x.directory);
}
async function filePickerLoad(path){if(!filePicker)return;const snapshot=filePicker;const list=document.querySelector('#ntEditorFilePickerRows');if(list)list.innerHTML='<p class="muted">Loading remote folder...</p>';
 try{const response=await filePickerRequest(snapshot.source,path||'.');if(filePicker!==snapshot)return;snapshot.path=response.path||path||'.';snapshot.selected='';snapshot.entries=(response.entries||[]).filter(x=>x&&typeof x.name==='string'&&x.name!=='.'&&x.name!=='..'&&!/[\\/\r\n\x00]/.test(x.name));filePickerDraw();}
 catch(error){if(filePicker!==snapshot)return;const area=document.querySelector('#ntEditorFilePickerRows');if(area)area.innerHTML='<div class="rb-notice error">'+escaped(error.message)+'</div>';}
}
function showRemoteFilePicker(){const source=document.getElementById('ntEditorRemoteSource')?.value||'';if(!source){setMessage('Select a connected session before browsing files.',true);return;}
 const typed=document.getElementById('ntEditorRemotePath')?.value.trim()||'',browser=root.NetworkToolkitRemoteBrowser;
 const kind=source.startsWith('client:')?source.slice(7):'',selectedPath=kind?browser?.get?.(kind)?.remote?.path||'.':'.';
 filePicker={source,path:typed?filePickerParent(typed):selectedPath,selected:'',entries:[]};
 openModal('Browse remote files','<div class="nt-editor-file-picker"><div class="nt-editor-picker-nav"><button type="button" class="btn small ghost" data-ed-picker-up title="Parent directory">'+icon('arrowUp')+' Up</button><input id="ntEditorFilePickerPath" class="mono" aria-label="Remote folder" placeholder="Remote folder" spellcheck="false"><button type="button" class="btn small ghost" data-ed-picker-go>Go</button><button type="button" class="btn small ghost" data-ed-picker-refresh>'+icon('refresh')+'</button></div><div id="ntEditorFilePickerRows" class="nt-editor-picker-list" role="listbox" aria-label="Remote folders and files"></div><p class="help-line">Double-click a folder to open it, or double-click a file to edit. Select a file and choose Open.</p></div>',
 '<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button type="button" class="btn primary" data-ed-picker-open disabled>Open selected file</button>',true);
 void filePickerLoad(filePicker.path);
}
function filePickerOpen(path){if(!filePicker)return;const file=filePicker.entries.find(x=>x.path===(path||filePicker.selected));if(!file)return;if(file.directory){void filePickerLoad(file.path);return;}
 const source=filePicker.source,field=document.getElementById('ntEditorRemotePath'),picker=document.getElementById('ntEditorRemoteSource');
 if(picker&&picker.value!==source){filePicker=null;closeModal();setMessage('The remote connection selection changed. Please browse again.',true);return;}
 if(field)field.value=file.path;filePicker=null;closeModal();void openRemotePath(file.path).catch(error=>{setMessage(error.message,true);if(typeof toast==='function')toast(error.message,'error');});
}
document.addEventListener('click',event=>{const node=event.target.closest?.('[data-ed-picker-path],[data-ed-picker-up],[data-ed-picker-go],[data-ed-picker-refresh],[data-ed-picker-open]');if(!node||!filePicker)return;
 event.preventDefault();event.stopImmediatePropagation();
 if(node.hasAttribute('data-ed-picker-path')){
  // Preserve the clicked DOM row so the following click produces a real
  // double-click event. Rebuilding the list here breaks native dblclick.
  filePicker.selected=node.dataset.edPickerPath;
  document.querySelectorAll('#ntEditorFilePickerRows [data-ed-picker-path]').forEach(row=>{
   const selected=row.dataset.edPickerPath===filePicker.selected;
   row.classList.toggle('selected',selected);
   row.setAttribute('aria-selected',String(selected));
  });
  const openButton=document.querySelector('[data-ed-picker-open]');
  if(openButton)openButton.disabled=!!filePicker.entries.find(x=>x.path===filePicker.selected&&x.directory);
  return;
 }
 if(node.hasAttribute('data-ed-picker-up')){void filePickerLoad(filePickerParent(filePicker.path));return;}
 if(node.hasAttribute('data-ed-picker-go')){void filePickerLoad(document.getElementById('ntEditorFilePickerPath')?.value||'.');return;}
 if(node.hasAttribute('data-ed-picker-refresh')){void filePickerLoad(filePicker.path);return;}
 if(node.hasAttribute('data-ed-picker-open'))filePickerOpen();
},true);
document.addEventListener('dblclick',event=>{const row=event.target.closest?.('[data-ed-picker-path]');if(!row||!filePicker)return;event.preventDefault();event.stopImmediatePropagation();filePickerOpen(row.dataset.edPickerPath);},true);
document.addEventListener('keydown',event=>{const input=event.target.closest?.('#ntEditorFilePickerPath');if(input&&event.key==='Enter'&&filePicker){event.preventDefault();void filePickerLoad(input.value);}},true);

function renderPage(mount){skin();embedded=true;const target=mount||document.getElementById('pageContent');if(!target)return;
 target.innerHTML='<div class="nt-editor-page-toolbar"><label>Local file path <input id="ntEditorLocalPath" type="text" spellcheck="false" placeholder="C:\\Users\\Public\\Documents\\example.txt"></label><button class="btn small ghost" data-ed-page="openPath">Open path</button><button class="btn small ghost" data-ed-page="newPath">New at path</button><button class="btn small ghost" data-ed-page="choose">Browse Windows files</button><button class="btn small ghost" data-ed-page="new">New document</button></div>'+
 '<div class="nt-editor-remote-toolbar"><label>Connected session<select id="ntEditorRemoteSource" aria-label="Connected SSH, SFTP, FTP or FTPS session"><option value="">Loading connections...</option></select></label><label>Remote file path<input id="ntEditorRemotePath" type="text" spellcheck="false" placeholder="/home/user/config.txt"></label><button class="btn small primary" id="ntEditorOpenRemote" data-ed-page="browseRemote" disabled>Browse / Open remote</button><button class="btn small ghost" data-ed-page="openRemotePath" title="Open by typing a full remote path">Open path</button><button class="btn small ghost" id="ntEditorNewRemote" data-ed-page="newRemote" disabled>New remote</button><button class="btn small ghost" data-ed-page="refreshRemote" title="Update list of active sessions">Refresh</button><div class="nt-editor-remote-hint" id="ntEditorRemoteHint" role="status">Connect through Terminal, SFTP, or FTP / FTPS before opening a remote file.</div></div>'+
 '<div id="ntEditorPageMount"></div>';
 overlay.classList.add('nt-editor-embedded');target.querySelector('#ntEditorPageMount').append(overlay);overlay.hidden=false;
 if(active)draw();else{findEl('[data-ed-tabs]').innerHTML='';findEl('[data-ed-title]').textContent='Open a desktop file, an active remote connection, or create a document.';findEl('[data-ed-text]').value='';findEl('[data-ed-language-select]').value='';findEl('[data-ed-highlight]').textContent='';findEl('[data-ed-gutter]').textContent='1';setMessage('Ready to open a document.');}
 void refreshConnectedSources();
}

document.addEventListener('click',event=>{const el=event.target.closest?.('[data-ed-page]');if(!el)return;event.preventDefault();const action=el.dataset.edPage,path=document.getElementById('ntEditorLocalPath')?.value;if(action==='choose')void openDesktopFile();if(action==='new')void newDocument();if(action==='refreshRemote')void refreshConnectedSources();if(action==='browseRemote')showRemoteFilePicker();if(action==='openRemotePath'||action==='newRemote')void openRemotePath(document.getElementById('ntEditorRemotePath')?.value,action==='newRemote').catch(e=>{setMessage(e.message,true);if(typeof toast==='function')toast(e.message,'error');});if(action==='openPath'||action==='newPath')void openLocalPath(path,action==='newPath').catch(e=>{if(typeof toast==='function')toast(e.message,'error');});});
document.addEventListener('change',event=>{if(event.target.id==='ntEditorRemoteSource'){selectedRemoteSource=event.target.value;const enabled=!!selectedRemoteSource;for(const id of ['ntEditorOpenRemote','ntEditorNewRemote']){const button=document.getElementById(id);if(button)button.disabled=!enabled;}}});
root.addEventListener('beforeunload',e=>{if([...documents.values()].some(d=>d.dirty)){e.preventDefault();e.returnValue='';}});
root.NetworkToolkitFileEditor={open,openDesktopFile,importFiles,openLocalPath,newDocument,renderPage,detachPage,closeAll,preferConnection,refreshConnectedSources,openRemotePath,showRemoteFilePicker,get documents(){return documents;},get active(){return current();}};
})(globalThis);
