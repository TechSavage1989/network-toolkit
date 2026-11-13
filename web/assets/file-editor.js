/* Integrated, session-aware text editor for local, SFTP and FTP files.
   Nothing is autosaved. Remote saves use the authenticated file browser and
   refuse stale revisions rather than overwriting a file changed elsewhere. */
(function(root){'use strict';
const documents=new Map();let active='',selectedRemoteSource='',preferredRemotePath='',remoteSourceGeneration=0,embedded=false,showFind=false,showReplace=false,matchCase=false,searchMode='normal',wholeWord=false,wordWrap=false,syntax=true,tabSpaces=4,overlay=null,updating=false,editorFontSize=13,editorPalette='auto',editorCustomColors={};
const escaped=x=>esc(String(x??''));
const paneWorkspace=()=>comparison||splitView;
const groups=root.NTKEditorTabGroups.create(()=>[...documents.values()]);
const findEl=sel=>sel==='[data-ed-text]'&&paneWorkspace()?paneWorkspace().textarea:overlay?.querySelector(sel);
const current=()=>documents.get(active);
const nameOf=p=>String(p||'Untitled').split(/[\\/]/).pop()||'Untitled';
const languageTools=root.NTKEditorLanguages;
const languageRows=languageTools?.languages||[{id:'text',label:'Plain text'}];
const languageFor=(path,content='')=>languageTools?.detect(path,content)||'text';
const languageOf=d=>d.language||d.diskSyntaxLanguage||languageFor(d.path,d.content);
let paintGeneration=0,paintTimer=null,largeScrollFrame=null,pendingLargeInput=null,displayedDocument=null,pendingEditorDialog=null,comparison=null,splitView=null,splitEnabled=false,comparisonReturnLayout=null,tabMenu=null,comparisonHost=null,pendingSavePath='',windowCloseState=null;
let singleDiskView=null,paneUsesDisk=false;
const diskAvailable=()=>root.NetworkToolkitEditorWindows?.bufferSupported?.()&&!!root.NTKEditorDiskSession&&!!root.NTKEditorDiskView;
const memoryEditorUnits=512*1024,memoryEditorLines=4000;
const diskCall=async(d,action,body,signal)=>{
 const payload=d.diskRemote&&['save','reload'].includes(action)?{...body,remote:await d.diskRemote()}:body;
 return root.NetworkToolkitEditorWindows.buffer(d.uid,action,payload,signal);
};
const diskView=()=>paneWorkspace()?paneWorkspace().diskView?.():singleDiskView;
const diskBusy=d=>!!(d.diskOpening||d.promoting||d.diskSession?.pending||d.diskEditing);
const editorComposing=()=>!!(paneWorkspace()?.composing||singleDiskView?.composing);
const formatOf=d=>({encoding:d.encoding,bom:d.bom,eol:d.eol});
const formatDirty=d=>d.eol!==d.savedEol||d.bom!==d.savedBOM||d.encoding!==d.savedEncoding;
// The editor opens any file. A file that is not text (encoding 'Binary') is
// shown read-only: the engine refuses every change and every save of it.
// Windows-1252 text and lines that keep their own ending ('Mixed') exist only
// in the engine's disk-backed documents, which write unedited bytes back
// exactly; such a document is never turned into a page-held one.
const notText=d=>d?.encoding==='Binary';
const engineFormat=format=>format?.encoding==='Binary'||format?.encoding==='Windows-1252'||format?.eol==='Mixed';
function openedNote(d){
 const name=nameOf(d.path);
 if(notText(d))return name+' is not a text file. It is shown read-only and cannot be changed or saved in the Text Editor.';
 const notes=[];
 if(d.encoding==='Windows-1252')notes.push('It is not UTF-8 and was read as Windows-1252; it is saved the same way unless you choose another encoding.');
 if(d.eol==='Mixed')notes.push('Its lines end in different ways. Each line keeps its own ending'+(d.newLineEol?' and new lines use '+d.newLineEol:'')+', unless you choose one under Line endings.');
 return 'Opened '+name+'.'+(notes.length?' '+notes.join(' '):'');
}
// Line endings and Encoding list what this document can be saved as.
function formatControls(d){
 const locked=notText(d),mixed=d.eol==='Mixed'||d.savedEol==='Mixed',legacy=d.encoding==='Windows-1252'||d.savedEncoding==='Windows-1252';
 // The lists are rebuilt only when their choices change; the usual three of
 // each are the markup the editor starts with.
 const fill=(select,usual,rows,value)=>{
  const choices=rows.map(row=>row[0]).join('|');
  if((select.dataset.edChoices||usual)!==choices){select.innerHTML=rows.map(([id,label])=>'<option value="'+id+'">'+label+'</option>').join('');select.dataset.edChoices=choices;}
  select.value=value;select.disabled=locked;
 };
 fill(findEl('[data-ed-eol]'),'LF|CRLF|CR',[...(mixed?[['Mixed','Mixed (keep each line)']]:[]),['LF','LF'],['CRLF','CRLF'],['CR','CR']],d.eol);
 fill(findEl('[data-ed-encoding]'),'UTF-8|UTF-16LE|UTF-16BE',locked?[['Binary','Not text']]:[['UTF-8','UTF-8'],['UTF-16LE','UTF-16LE'],['UTF-16BE','UTF-16BE'],...(legacy?[['Windows-1252','Windows-1252']]:[])],d.encoding);
 const bom=findEl('[data-ed-bom]');bom.checked=d.bom;bom.disabled=locked||d.encoding==='Windows-1252';
}

function ensureDiskSession(d){
 if(d.diskSession)return d.diskSession;
 d.diskSession=root.NTKEditorDiskSession.create({
  request:(action,payload,signal)=>diskCall(d,action,payload,signal),
  exportRange:(payload,signal)=>root.NetworkToolkitEditorWindows.bufferExport(d.uid,payload,signal),
  changed(value){
   const wasDirty=d.dirty;
   d.diskMeta=value.metadata;
   d.editRevision=value.metadata.revision;
   d.dirty=value.metadata.dirty||formatDirty(d);
   if(wasDirty!==d.dirty)drawTabs();
   if(current()===d)status();
  },
  failed(error){d.saveFailed=true;if(current()===d)setMessage('Your changes are still in this tab. '+error.message,true);}
 });
 return d.diskSession;
}

function applyDiskFormat(d,result){
 d.hash=result.hash||'';
 d.encoding=d.savedEncoding=result.format.encoding;
 d.bom=d.savedBOM=result.format.bom;
 d.eol=d.savedEol=result.format.eol;
 d.newLineEol=result.newLineEol||'';
}

function useDiskDocument(d,result){
 d.disk=true;
 d.path=result.path||d.path;
 d.content=d.original=d.saved='';
 d.undo=[];
 d.redo=[];
 d.lineIndex=d.gutterCache=d.syntaxCache=null;
 applyDiskFormat(d,result);
 ensureDiskSession(d).initialize(result.metadata);
}

async function waitForDiskDocument(d){
 for(;;){
  if(documents.get(d.id)!==d)throw Error('The file was closed.');
  const result=await diskCall(d,'status');
  if(documents.get(d.id)!==d)throw Error('The file was closed.');
  if(!result.loading)return result;
  if(current()===d)setMessage('Opening '+nameOf(d.path)+' · '+Number(result.bytesRead||0).toLocaleString()+' bytes read');
  await new Promise(resolve=>root.setTimeout(resolve,125));
 }
}

async function releaseDiskDocument(d){
 const windows=root.NetworkToolkitEditorWindows;
 for(let attempt=0;attempt<4;attempt++){
  try{await windows.releaseBuffer(d.uid);return;}
  catch(error){if(error.status!==409||attempt===3)throw error;await new Promise(resolve=>root.setTimeout(resolve,60));}
 }
}

async function sendBrowserFile(d,file,action,label){
 const chunkBytes=512*1024;
 for(let offset=0;offset<file.size;offset+=chunkBytes){
  if(documents.get(d.id)!==d)throw Error('The file was closed.');
  const bytes=new Uint8Array(await file.slice(offset,offset+chunkBytes).arrayBuffer());
  const binary=[];
  if(!bytes.toBase64)for(let start=0;start<bytes.length;start+=8192)binary.push(String.fromCharCode(...bytes.subarray(start,start+8192)));
  const data=bytes.toBase64?bytes.toBase64():btoa(binary.join(''));
  await diskCall(d,action,{offset,data});
  if(current()===d)setMessage(label+' '+nameOf(d.path)+' · '+Math.floor(100*(offset+bytes.length)/file.size)+'%');
 }
}

async function importBrowserFile(d,file,label='Opening'){
 await sendBrowserFile(d,file,'import-chunk',label);
 await diskCall(d,'import-finish',{size:file.size});
 return waitForDiskDocument(d);
}

async function openDiskDocument(d,mode='',file){
 const windows=root.NetworkToolkitEditorWindows;
 d.diskOpening=true;
 try{
  const identity=documentTransferValue(d);
  const remote=await d.diskRemote?.();
  if(documents.get(d.id)!==d)throw Error('The file was closed.');
  await windows.openBuffer(identity,mode,remote,mode==='restore'?d.restoreRequest:undefined);
  const result=file?await importBrowserFile(d,file):await waitForDiskDocument(d);
  if(mode==='restore')d.restoreStatus=result.restore||null;
  // A kept copy of a local file stays disk-backed so its save baseline stays
  // with the engine; a small restored draft becomes an ordinary memory document.
  if((!mode||mode==='restore'&&d.binding?.kind==='localDraft')&&!d.diskRemote&&result.metadata.units<=memoryEditorUnits&&result.metadata.lines<=memoryEditorLines&&!engineFormat(result.format)){
   const parts=[];
   let start=0;
   while(start<result.metadata.units){
    const page=await diskCall(d,'window',{start,limit:128*1024});
    if(!page.text?.length)throw Error('The editor received an incomplete document.');
    parts.push(page.text);start+=page.text.length;
   }
   await releaseDiskDocument(d);
   return {content:parts.join(''),hash:result.hash,...result.format};
  }
  useDiskDocument(d,result);
  return {disk:true};
 }catch(error){
  if(root.NetworkToolkitEditorWindows.state.owned[d.uid]){
   try{await diskCall(d,'cancel');}catch{}
   try{await releaseDiskDocument(d);}catch{}
  }
  throw error;
 }finally{d.diskOpening=false;}
}

async function streamDiskFile(d,handle,format,beforeCommit){
 const session=ensureDiskSession(d);
 await session.flush();
 const response=await root.NetworkToolkitEditorWindows.bufferExport(d.uid,{revision:d.editRevision,format});
 let writer;
 try{
  if(!response.body)throw Error('The editor returned no file data.');
  writer=await handle.createWritable();
  await response.body.pipeTo(writer,{preventClose:true});
  if(beforeCommit)await beforeCommit();
  await writer.close();
  return response.headers.get('X-Editor-Export');
 }catch(error){
  if(writer)await writer.abort().catch(()=>{});
  if(response.body&&!response.body.locked)await response.body.cancel().catch(()=>{});
  throw error;
 }
}

async function saveBrowserDocument(d,format){
 const handle=d.desktopHandle,source=await handle.getFile();
 await diskCall(d,'verify-start');
 await sendBrowserFile(d,source,'verify-chunk','Checking');
 const result=await diskCall(d,'verify-finish',{size:source.size});
 const latest=await handle.getFile();
 if(result.hash!==d.hash||latest.size!==source.size||latest.lastModified!==source.lastModified)throw Error('The file changed outside the editor. Reload it before saving.');
 const receipt=await streamDiskFile(d,handle,format,async()=>{
  const currentFile=await handle.getFile();
  if(currentFile.size!==source.size||currentFile.lastModified!==source.lastModified)throw Error('The file changed while saving. Reload it before saving.');
 });
 if(!receipt)throw Error('The editor could not confirm the saved copy.');
 const saved=await diskCall(d,'mark-saved',{receipt});
 if(saved.metadata.revision===d.editRevision)d.diskSession.initialize(saved.metadata);
 return saved;
}

async function exportDiskCopy(d){
 if(d.loading||d.saving||d.reloading||d.frozen||d.diskEditing||editorComposing())return;
 if(typeof root.showSaveFilePicker!=='function'){await save(true,d);return;}
 d.saving=true;drawTabs();
 try{
  const handle=await root.showSaveFilePicker({suggestedName:nameOf(d.path)});
  if(d.desktopHandle&&await d.desktopHandle.isSameEntry(handle))throw Error('Use Save to replace the file you opened.');
  await streamDiskFile(d,handle,formatOf(d));
  if(current()===d)setMessage('Saved a copy of '+nameOf(d.path)+'.');
 }catch(error){if(error.name!=='AbortError')setMessage(error.message,true);}
 finally{d.saving=false;drawTabs();}
}

async function reloadBrowserDocument(d){
 const source=d.desktopHandle?await d.desktopHandle.getFile():d.diskFile;
 await diskCall(d,'reload-start',{revision:d.editRevision});
 try{
  const result=await importBrowserFile(d,source,'Reloading');
  if(result.reloadError)throw Error(result.reloadError);
  return result;
 }catch(error){
  try{await diskCall(d,'cancel');}catch{}
  throw error;
 }
}

function shouldPage(value){
 if(value.length>memoryEditorUnits)return true;
 if(value.length<memoryEditorLines)return false;
 let next=-1;
 for(let line=0;line<memoryEditorLines;line++){
  next=value.indexOf('\n',next+1);
  if(next<0)return false;
 }
 return true;
}

function canPromote(d){
 return !!diskAvailable()&&['local','localDraft','terminal','sshConnection'].includes(d.binding?.kind);
}

async function adoptDiskDocument(d,snapshot){
 const windows=root.NetworkToolkitEditorWindows;
 try{await windows.openBuffer(snapshot,'promote');}
 catch(error){if(error.status)throw error;await windows.openBuffer(snapshot,'promote');}
 const result=await waitForDiskDocument(d);
 useDiskDocument(d,result);
 d.eol=snapshot.eol;d.bom=snapshot.bom;d.encoding=snapshot.encoding;
 d.savedEol=snapshot.savedEol;d.savedBOM=snapshot.savedBOM;d.savedEncoding=snapshot.savedEncoding;
 d.dirty=!!d.diskMeta?.dirty||formatDirty(d);
}

function promoteDocument(d,value,caret){
 const snapshot=documentTransferValue(d);
 const fragment=root.NTKEditorDiskSession.fragment(d.content,value);
 d.promoting=true;
 if(current()===d){findEl('[data-ed-text]').disabled=true;setMessage('Preparing this document for large-file editing…');}
 const promotion=(async()=>{
  let failure;
  try{
   await adoptDiskDocument(d,snapshot);
   await ensureDiskSession(d).replace(fragment.from,fragment.to,fragment.text);
   const position=Math.min(d.diskMeta.units,caret??fragment.from+fragment.text.length);
   const here=await diskCall(d,'window',{start:position,limit:2,align:true});
   d.view={start:position,end:position,direction:'none',top:Math.max(0,here.line-2),left:0};
   if(displayedDocument===d)displayedDocument=null;
   return {ok:true};
  }catch(error){
   failure=error;
   return {ok:false,error:error.message};
  }finally{
   d.promoting=false;d.promotionResult=null;
   if(d.disk)updatePaneFactory();
   if(current()===d){draw();setMessage(failure?'Could not apply the edit. '+failure.message:'Ready',!!failure);}
  }
 })();
 d.promotionResult=promotion;
 return true;
}

function mountDiskView(d,{textarea,gutter,container,readOnly=false,changed=()=>{},savedView=d.view}){
 const session=ensureDiskSession(d);
 textarea.disabled=true;
 textarea.classList.remove('highlight-active','is-wrapped');
 textarea.wrap='off';
 let sample, detected;
 const syntaxLanguage=text=>{
  if(!syntax||notText(d))return '';
  if(d.language)return d.language;
  if(d.diskSyntaxLanguage)return d.diskSyntaxLanguage;
  if(sample!==text){
   sample=text;
   const name=nameOf(d.path);
   const candidate=languageFor(name,text);
   if(candidate.startsWith('network-'))d.diskSyntaxLanguage=candidate;
   else if(/forti(?:gate|net|os).*\.(?:cfg|conf)$/i.test(name))d.diskSyntaxLanguage='network-fortios';
   detected=d.diskSyntaxLanguage||candidate;
   if(d.diskSyntaxLanguage&&current()===d)syncLanguageControls(d);
  }
  return detected||'text';
 };
 const view=root.NTKEditorDiskView.create({textarea,gutter,container,session,syntaxLanguage,wordWrap:()=>wordWrap,readOnly:readOnly||notText(d),locked:()=>notText(d),changed,message:setMessage,painted:()=>documentMap?.schedule()});
 let disposed=false,loaded=false;
 const ready=(async()=>{
  if(!session.state().metadata.revision){
   const info=await diskCall(d,'status');
   if(disposed)return;
   session.initialize(info.metadata);
  }
  if(disposed)return;
  if(savedView)await view.restore(savedView,false);
  else await view.load({start:0});
  if(!disposed){loaded=true;textarea.disabled=!!(d.loading||d.frozen||d.reloading||d.promoting);changed();}
 })();
 void ready.catch(error=>{if(!disposed)setMessage(error.message,true);});
 return Object.freeze(Object.defineProperties(Object.create(view),{ready:{value:ready},loaded:{get:()=>loaded},dispose:{value:()=>{disposed=true;view.dispose();}}}));
}

function disposeSingleDiskView(){singleDiskView?.dispose();singleDiskView=null;}
function discardDiskView(d,release=true){
 if(displayedDocument===d)disposeSingleDiskView();
 d.diskSearch?.abort();
 d.diskSession?.close();
 if(release&&d.disk)void releaseDiskDocument(d).catch(error=>setMessage('Could not release the closed document. '+error.message,true));
}

function drawDiskDocument(d,savedView){
 const same=displayedDocument===d&&singleDiskView;
 displayedDocument=d;
 drawTabs();
 syncEditorControls(d);
 const textarea=findEl('[data-ed-text]');
 findEl('[data-ed-title]').textContent=d.source+' · '+d.path;
 findEl('[data-ed-highlight]').hidden=true;
 for(const button of overlay.querySelectorAll('[data-ed="save"],[data-ed="saveAs"],[data-ed="reload"]'))button.disabled=!!(d.loading||d.frozen||d.reloading)||notText(d)&&button.dataset.ed!=='reload';
 if(d.staged){disposeSingleDiskView();textarea.value='';textarea.disabled=true;return;}
 const focus=d.showStart;
 d.showStart=false;
 if(!same){
  disposeSingleDiskView();
  const view=mountDiskView(d,{textarea,gutter:findEl('[data-ed-gutter-lines]'),container:overlay.querySelector('.nt-editor-code'),savedView,changed:status});
  singleDiskView=view;
  if(focus)void view.ready.then(()=>{if(singleDiskView===view)textarea.focus({preventScroll:true});},()=>{});
 }else{
  textarea.disabled=!singleDiskView.loaded||!!(d.loading||d.frozen||d.reloading||d.promoting);
  singleDiskView.setReadOnly(!!(d.frozen||d.reloading||d.diskEditing));
  singleDiskView.refreshWrap();
  if(focus)void singleDiskView.position(0,0,true).catch(error=>setMessage(error.message,true));
 }
}

async function searchDiskDocument(d,query,backwards,replace,all,focus,replacement=''){
 const view=diskView();
 if(!view||d.loading||d.frozen||d.reloading||d.diskEditing)return;
 d.diskSearch?.abort();
 const controller=new AbortController();
 d.diskSearch=controller;
 const {signal}=controller;
 const session=ensureDiskSession(d);
 const options={regex:searchMode==='regex',matchCase,wholeWord};
 try{
  if(!focus)await new Promise(resolve=>root.setTimeout(resolve,140));
  if(signal.aborted||current()!==d||view!==diskView())return;
  await view.ready;
  if(signal.aborted||current()!==d)return;
  setMessage(all?'Replacing matches…':'Searching…');
  if(all){
   d.diskEditing=true;
   view.setReadOnly(true);
   await session.flush();
   const result=await diskCall(d,'replace-all',{query,replacement,revision:d.editRevision,...options},signal);
   session.initialize(result.metadata);
   await view.position(Math.min(view.selection().start,result.metadata.units));
   await view.load({start:session.state().page.start});
   setMessage('Replaced '+result.count.toLocaleString()+' occurrence(s).');
   return;
  }
  const range=view.selection();
  const start=backwards||replace?range.start:range.end;
  let match=await session.find(query,start,{...options,backwards},signal);
  if(!match.found&&start!==0&&!backwards)match=await session.find(query,0,options,signal);
  if(!match.found&&backwards&&start<d.diskMeta.units)match=await session.find(query,d.diskMeta.units,{...options,backwards:true},signal);
  if(signal.aborted||current()!==d||diskView()!==view)return;
  if(!match.found){setMessage('No matches.');return;}
  if(replace&&match.from===range.start&&match.to===range.end){
   // A regular expression's Replace text names groups of this match ($1):
   // the engine expands it exactly as Replace All does.
   let text=replacement;
   if(options.regex){
    await session.flush();
    const expanded=await diskCall(d,'replace-text',{query,replacement,...options,from:match.from,to:match.to,revision:d.editRevision},signal);
    if(signal.aborted||current()!==d)return;
    if(!expanded.found){setMessage('The selection is no longer a match. Search again.');return;}
    text=expanded.text;
   }
   await view.replace(text);
  }
  else await view.position(match.from,match.to,focus);
  setMessage('Match on line '+(match.line+1).toLocaleString()+'.');
 }catch(error){if(!signal.aborted&&current()===d)setMessage(error.message,true);}
 finally{
  if(all){d.diskEditing=false;if(diskView()===view)view.setReadOnly(!!comparison);}
  if(d.diskSearch===controller)d.diskSearch=null;
 }
}

async function transformDiskSelection(d,kind){
 const view=diskView();
 if(!view||d.loading||d.frozen||d.diskEditing||comparison)return;
 const session=ensureDiskSession(d);
 d.diskEditing=true;
 view.setReadOnly(true);
 try{
  await session.flush();
  let {start,end}=view.selection();
  const wholeLines=!['upper','lower'].includes(kind);
  if(wholeLines){
   const [first,last]=await Promise.all([diskCall(d,'window',{start,limit:2,align:true}),diskCall(d,'window',{start:end>start?end-1:end,limit:2,align:true})]);
   start=first.start-first.column;
   end=last.start-last.column+last.lineUnits;
  }
  const text=await session.readText(start,end,1024*1024);
  if(current()!==d||diskView()!==view)return;
  let next=text;
  const unit=tabSpaces?' '.repeat(tabSpaces):'\t';
  switch(kind){
   case 'upper':next=text.toLocaleUpperCase();break;
   case 'lower':next=text.toLocaleLowerCase();break;
   case 'sort':next=text.split('\n').sort((a,b)=>a.localeCompare(b,undefined,{numeric:true,sensitivity:'base'})).join('\n');break;
   case 'duplicate':next=text+'\n'+text;break;
   case 'trim':next=text.split('\n').map(line=>line.replace(/[ \t]+$/,'')).join('\n');break;
   case 'comment':next=languageTools?.toggleComment(text,languageOf(d));break;
   case 'indent':next=text.split('\n').map(line=>unit+line).join('\n');break;
   case 'unindent':next=text.split('\n').map(line=>line.replace(new RegExp('^(?:\\t| {1,'+(tabSpaces||4)+'})'),'')).join('\n');break;
  }
  if(next==null){setMessage('This language does not define comment syntax.');return;}
  await session.replace(start,end,next);
  view.refresh();
  await view.position(start,start+next.length,true);
 }catch(error){if(current()===d)setMessage(error.message,true);}
 finally{d.diskEditing=false;if(diskView()===view)view.setReadOnly(false);}
}

async function insertDiskText(d,text,where){
 const view=diskView(),session=ensureDiskSession(d);
 if(!view)return {ok:false,error:'The editor is not ready.'};
 d.diskEditing=true;
 view.setReadOnly(true);
 try{
  await view.ready;
  await session.flush();
  let from=0,to=0;
  if(where==='cursor')({start:from,end:to}=view.selection());
  else if(where&&typeof where==='object'&&Number.isSafeInteger(where.line)&&where.line>0){
   if(where.line>d.diskMeta.lines)from=to=d.diskMeta.units;
   else from=to=(await diskCall(d,'window',{line:where.line-1,limit:2})).start;
  }else if(where==='end'){
   from=to=d.diskMeta.units;
   if(from&&!text.startsWith('\n')){
    const tail=await diskCall(d,'window',{start:Math.max(0,from-2),limit:3,align:true});
    if(!tail.text.endsWith('\n'))text='\n'+text;
   }
  }
  text=normalize(text);
  await session.replace(from,to,text);
  if(current()===d&&diskView()===view){view.refresh();await view.position(from+text.length);}
  setMessage('Inserted '+text.length.toLocaleString()+' characters from the Terminal. Undo (Ctrl+Z) removes them.');
  return {ok:true};
 }catch(error){return {ok:false,error:error.message};}
 finally{d.diskEditing=false;if(diskView()===view)view.setReadOnly(false);}
}

async function sendDiskTextToTerminal(d,view,send){
 try{
  const range=view.selection();
  if(range.start===range.end){setMessage('Select the commands you want to send to the Terminal.');return;}
  const selection=await view.selectedText(1024*1024);
  if(current()!==d||diskView()!==view)return;
  send.open({name:nameOf(d.path),content:selection,selection,stamp:()=>current()===d?documentStamp(d):'changed',hideFloating:()=>{if(!embedded&&overlay)overlay.hidden=true;}});
 }catch(error){setMessage(error.message,true);}
}
const editorPalettes=['auto','dark','light','pastel','forest','highContrast','custom'];
const editorColorLabels={bg:'Background',fg:'Normal text',comment:'Comments',string:'Strings',keyword:'Keywords',number:'Numbers',property:'Properties'};
const editorColorDefaults={bg:'#202535',fg:'#e5eaf4',comment:'#97c8aa',string:'#edcb98',keyword:'#a8bbff',number:'#ddb8f2',property:'#99dbe7'};
function goodEditorColor(value){return typeof value==='string'&&/^#[0-9a-f]{6}$/i.test(value);}
function colorPickerHTML(){return '<div class="nt-editor-palette-box">'+Object.entries(editorColorLabels).map(([k,label])=>'<label><span>'+label+'</span><input type="color" data-ed-color="'+k+'" value="'+(editorCustomColors[k]||editorColorDefaults[k])+'"><input type="text" data-ed-color-hex="'+k+'" data-input-clear="off" value="'+(editorCustomColors[k]||editorColorDefaults[k])+'" maxlength="7" spellcheck="false" aria-label="'+label+' hex color"></label>').join('')+'</div>';}
const editorPref=()=>root.state?.editorPreferences||{};
let childAskSaveBeforeClose=true,closePreferencePending=false;
const detachedEditorWindow=()=>!!root.EDITOR_VIEW?.id||root.NetworkToolkitEditorWindows?.state?.self!==undefined&&root.NetworkToolkitEditorWindows.state.self!=='';
const askSaveBeforeClose=()=>detachedEditorWindow()?childAskSaveBeforeClose:(typeof state==='undefined'?true:state.editorPreferences?.askSaveBeforeClose!==false);
// Ordinary tab closure may cancel a read/format safely; window close retains its stricter existing preparation guard.
const documentCloseBlocked=d=>!!(d.saving||d.frozen||d.reloading||d.promoting||d.diskEditing);
const closeNeedsConfirmation=list=>list.some(d=>d.saveFailed===true)||askSaveBeforeClose()&&list.some(d=>d.dirty);
// Session restore runs only in the main window and the standalone Text Editor,
// never in detached editor windows, Terminal-only windows or offline pages.
const sessionWorkspace=()=>!detachedEditorWindow()&&!(typeof EDITOR_WINDOW!=='undefined'&&EDITOR_WINDOW)&&!(typeof TERMINAL_ONLY!=='undefined'&&TERMINAL_ONLY)&&typeof OFFLINE!=='undefined'&&!OFFLINE&&typeof api==='function';
// The engine advertises the session capability in its status (editor_session.go).
const sessionEngine=()=>root.NTKEditorSession?.engineAvailable?.()===true;
const sessionAvailable=()=>sessionWorkspace()&&sessionEngine();
const restoreSessionPreference=()=>typeof state==='undefined'||state.editorPreferences?.restoreSession!==false;
let restorePreferencePending=false;
function syncClosePreference(){const restore=findEl('[data-ed-restore-session]');if(restore){const available=sessionAvailable();restore.checked=restoreSessionPreference();restore.disabled=!available||restorePreferencePending;restore.closest('label')?.setAttribute('title',detachedEditorWindow()?'Change this app preference in the main window.':available?'Reopen the files that were open, and their unsaved changes, the next time the Text Editor starts.':'Available in the main window and the Text Editor app.');}const input=findEl('[data-ed-ask-save-close]');if(input){input.checked=askSaveBeforeClose();input.disabled=detachedEditorWindow()||closePreferencePending;input.closest('label')?.setAttribute('title',detachedEditorWindow()?'Change this app preference in the main window. Closing this child returns its documents without discarding.':'Disabling permits unsaved changes to be discarded when documents or the app close.');}const note=findEl('[data-ed-close-policy-note]');if(note)note.textContent=detachedEditorWindow()?'This app preference is changed in the main window. Closing this window returns its documents to the main workspace.':'When disabled, closing documents or the app may discard unsaved changes. Saved files are never deleted.';}
function setClosePolicy(value){childAskSaveBeforeClose=value!==false;syncClosePreference();}
async function changeRestorePreference(value){if(!sessionAvailable()||restorePreferencePending){syncClosePreference();return;}restorePreferencePending=true;syncClosePreference();try{if(typeof state!=='undefined'){state.editorPreferences={...(state.editorPreferences||{}),restoreSession:value!==false};if(typeof saveState==='function')saveState();}if(value!==false){session?.enable();setMessage('Open files and unsaved changes will reopen next time.');}else{await session?.disable();setMessage('Open files will not reopen next time. Kept copies of unsaved text were removed.');}}catch(error){setMessage('The kept session could not be removed: '+error.message,true);}finally{restorePreferencePending=false;syncClosePreference();}}
async function changeClosePreference(value){if(detachedEditorWindow()||closePreferencePending){syncClosePreference();return;}closePreferencePending=true;syncClosePreference();try{if(!value&&await askEditor({title:'Allow unsaved changes to be discarded?',message:'Closing documents or the app without asking may discard unsaved changes. Saved files will not be deleted. Active saves, window moves and failed-save warnings remain protected.',accept:'Allow discard'})===null)return;if(typeof state!=='undefined'){state.editorPreferences={...(state.editorPreferences||{}),askSaveBeforeClose:value!==false};if(typeof saveState==='function')saveState();}}finally{closePreferencePending=false;syncClosePreference();}}
function activityDocuments(){return [...documents.values()].filter(d=>!d.staged).map(d=>({uid:d.uid,loaded:!d.loading&&(!d.untitled||d.content!==''||d.dirty||d.editRevision>1||d.undo.length>0||d.redo.length>0)}));}

const pagePanels={local:false,remote:false};let localPathDraft='',savedConnectionState={profiles:[],detached:false,requiresApproval:false},savedConnectPending=false,remoteSelectionRevision=0;
const mapZooms=[-3,-2,-1,0,.5,1,2,4];
let compareMapZoom=-3,compareHorizontalSync=false,compareUnderline=true,compareIntralineHighlight=true,compareWrapNavigation=true,compareMoveCaret=true,compareShowSummary=true;
// Document Map (editor-document-map.js): off by default, an editor preference
// like Wrap, so it is shared by every Text Editor of the data folder.
let documentMapOn=false,documentMap=null;
function readPreferences(){const v=typeof state==='undefined'?{}:state.editorPreferences||{};compareMapZoom=mapZooms.includes(v.compareMapZoom)?v.compareMapZoom:-3;compareHorizontalSync=v.compareHorizontalSync===true;compareUnderline=v.compareUnderline!==false;compareIntralineHighlight=v.compareIntralineHighlight!==false;compareWrapNavigation=v.compareWrapNavigation!==false;compareMoveCaret=v.compareMoveCaret!==false;compareShowSummary=v.compareShowSummary!==false;documentMapOn=v.documentMap===true;pagePanels.local=v.localPanelOpen===true;pagePanels.remote=v.remotePanelOpen===true;wordWrap=v.wordWrap===true;syntax=v.syntax!==false;tabSpaces=[0,2,4,8].includes(v.indent)?v.indent:4;editorFontSize=Number.isInteger(v.fontSize)&&v.fontSize>=10&&v.fontSize<=24?v.fontSize:13;editorPalette=editorPalettes.includes(v.palette)?v.palette:'auto';editorCustomColors=Object.fromEntries(Object.entries(v.colors||{}).filter(([key,value])=>Object.hasOwn(editorColorLabels,key)&&goodEditorColor(value)));}
function keepPreferences(){if(typeof state==='undefined')return;state.editorPreferences={...(state.editorPreferences||{}),compareMapZoom,compareHorizontalSync,compareUnderline,compareIntralineHighlight,compareWrapNavigation,compareMoveCaret,compareShowSummary,documentMap:documentMapOn,askSaveBeforeClose:askSaveBeforeClose(),restoreSession:restoreSessionPreference(),wordWrap,syntax,indent:tabSpaces,fontSize:editorFontSize,palette:editorPalette,colors:editorCustomColors,localPanelOpen:pagePanels.local,remotePanelOpen:pagePanels.remote};if(typeof saveState==='function')saveState();}

const lineMode=s=>s.includes('\r\n')?'CRLF':s.includes('\r')?'CR':'LF';
const normalize=s=>String(s).replace(/\r\n|\r/g,'\n');
const outgoing=(d)=>d.content.replace(/\n/g,d.eol==='CRLF'?'\r\n':d.eol==='CR'?'\r':'\n');
// File-manager drags carry paths, not editor text. Only a trusted drag that began
// in this renderer may use them; revalidate its listing and transport on every use.
const sidebarFileMIME='application/x-networktoolkit-remote-browser';
let sidebarFileDrag=null,sidebarPointerDrag=null,sidebarDropBusy=false,sidebarDragMessage='';
function captureSidebarFileDrag(event){
 sidebarFileDrag=null;sidebarDragMessage='';
 if(!event.isTrusted)return;
 const row=event.target.closest?.('[data-rb-item]'),host=row?.closest('[data-rb-root]'),browser=root.NetworkToolkitRemoteBrowser,k=host?.dataset.rbRoot,side=row?.dataset.rbSide;
 if(!browser||!['ssh','ftp'].includes(k)||!['local','remote'].includes(side))return;
 const model=browser.get(k),pane=model[side],entry=pane?.entries.find(file=>file.path===row.dataset.rbItem);
 if(!entry)return;
 const files=(pane.selected.has(entry.path)?pane.entries.filter(file=>pane.selected.has(file.path)):[entry]).map(file=>({path:file.path,directory:!!file.directory}));
 sidebarFileDrag={host,k,side,pane,entries:pane.entries,files,identity:browser.dragIdentity(k)};
}
function validateSidebarFileSource(drag){
 const browser=root.NetworkToolkitRemoteBrowser;
 if(!drag||!browser)throw Error('Drag files from this window’s Files sidebar to open them.');
 const model=browser.get(drag.k),pane=model[drag.side];
 if(!drag.host.isConnected||pane!==drag.pane||pane.entries!==drag.entries||drag.files.some(file=>!pane.entries.some(entry=>entry.path===file.path&&!!entry.directory===file.directory)))throw Error('The source folder changed. Drag the file again.');
 if(model.busy||model.connecting)throw Error('Finish the current file operation before opening a dropped file.');
 if(drag.side==='remote'&&(drag.k!=='ssh'||!model.connected||browser.dragIdentity(drag.k)!==drag.identity))throw Error('The SFTP connection changed or is unavailable. Drag the file again from the current connection.');
 if(!drag.files.length||drag.files.length>12||drag.files.some(file=>file.directory))throw Error('Drop text files to open them. Folders cannot be opened as text documents.');
 if(sidebarDropBusy||windowCloseState)throw Error('Wait for the current editor operation to finish before dropping files.');
 const editorGuard=canReorderDocuments();if(editorGuard)throw Error(editorGuard);
 return drag;
}
function sidebarDropSource(event){
 if(!Array.from(event.dataTransfer?.types||[]).includes(sidebarFileMIME))return null;
 if(!event.isTrusted)throw Error('Drag files from this window’s Files sidebar to open them.');
 return validateSidebarFileSource(sidebarFileDrag);
}
function editorFileDragOver(event){
 if(!event.target.closest?.('.nt-editor-document-area'))return;
 if(!Array.from(event.dataTransfer?.types||[]).includes(sidebarFileMIME))return;
 event.preventDefault();event.stopPropagation();let message;
 try{const drag=sidebarDropSource(event);event.dataTransfer.dropEffect='copy';message='Drop to open '+drag.files.length+' file'+(drag.files.length===1?'':'s')+' in editor tabs.';}
 catch(error){event.dataTransfer.dropEffect='none';message=error.message;}
 if(sidebarDragMessage!==message){sidebarDragMessage=message;setMessage(message,event.dataTransfer.dropEffect==='none');}
}
async function openSidebarFileSelection(captured,validate){
 let acquired=false;
 try{
  const drag=validate();
  sidebarDropBusy=true;acquired=true;sidebarFileDrag=null;
  for(const file of drag.files){
   const browser=root.NetworkToolkitRemoteBrowser,model=browser.get(drag.k);
   if(!drag.host.isConnected||model[drag.side].entries!==drag.entries||drag.side==='remote'&&(!model.connected||browser.dragIdentity(drag.k)!==drag.identity))throw Error('The source folder or SFTP connection changed. Remaining files were not opened.');
   await browser.edit(drag.k,false,drag.side,file.path);
  }
 }catch(error){setMessage(error.message,true);if(typeof toast==='function')toast(error.message,'error');}
 finally{if(acquired)sidebarDropBusy=false;if(sidebarFileDrag===captured)sidebarFileDrag=null;if(!sidebarFileDrag)sidebarDragMessage='';}
 return true;
}
async function openSidebarFileDrop(event){
 if(!event.target.closest?.('.nt-editor-document-area')||!Array.from(event.dataTransfer?.types||[]).includes(sidebarFileMIME))return false;
 event.preventDefault();event.stopPropagation();
 return openSidebarFileSelection(sidebarFileDrag,()=>{
  const drag=sidebarDropSource(event),payload=JSON.parse(event.dataTransfer.getData(sidebarFileMIME));
  if(payload.k!==drag.k||payload.side!==drag.side||!Array.isArray(payload.files)||payload.files.length!==drag.files.length||payload.files.some((file,index)=>file.path!==drag.files[index].path||!!file.directory!==drag.files[index].directory))throw Error('The dragged file selection changed. Drag the files again.');
  return drag;
 });
}
// Native file gestures cannot run a competing HTML OLE drag. These methods
// accept only actual pointer events, never caller-provided paths or targets.
function beginSidebarPointerDrag(event){
 cancelSidebarPointerDrag();
 if(typeof root.PointerEvent!=='function'||!(event instanceof root.PointerEvent)||!event.isTrusted||event.type!=='pointerdown'||event.button!==0)return false;
 captureSidebarFileDrag(event);if(!sidebarFileDrag)return false;
 sidebarPointerDrag={source:sidebarFileDrag,id:event.pointerId,x:event.clientX,y:event.clientY};return true;
}
function cancelSidebarPointerDrag(){const pointer=sidebarPointerDrag;sidebarPointerDrag=null;if(pointer&&sidebarFileDrag===pointer.source)sidebarFileDrag=null;}
function finishSidebarPointerDrop(event){
 const pointer=sidebarPointerDrag;sidebarPointerDrag=null;
 if(!pointer)return false;
 if(typeof root.PointerEvent!=='function'||!(event instanceof root.PointerEvent)||!event.isTrusted||event.type!=='pointerup'||event.pointerId!==pointer.id||Math.hypot(event.clientX-pointer.x,event.clientY-pointer.y)<6){if(sidebarFileDrag===pointer.source)sidebarFileDrag=null;return false;}
 const target=document.elementFromPoint(event.clientX,event.clientY);
 if(!target?.closest?.('.nt-editor-document-area')){if(sidebarFileDrag===pointer.source)sidebarFileDrag=null;return false;}
 event.preventDefault();event.stopPropagation();
 void openSidebarFileSelection(pointer.source,()=>validateSidebarFileSource(pointer.source));
 return true;
}
document.addEventListener('dragstart',captureSidebarFileDrag,true);
document.addEventListener('dragend',()=>{sidebarFileDrag=null;sidebarDragMessage='';},true);
document.addEventListener('keydown',event=>{if(event.key==='Escape'){cancelSidebarPointerDrag();sidebarFileDrag=null;sidebarDragMessage='';}},true);
function editorIconButton(action,label,glyph,attributes=''){
 return '<button type="button" class="btn small ghost nt-editor-icon-button" data-ed="'+action+'" aria-label="'+label+'" title="'+label+'"'+attributes+'>'+(typeof icon==='function'?icon(glyph):label)+'</button>';
}

// What the Document Map (editor-document-map.js) reads of the editor: the
// active document and the text area that shows it (single view, or the
// active pane of a split view), and the disk view of a disk-backed document.
const documentMapAdapter=Object.freeze({
 overlay:()=>overlay,
 enabled:()=>documentMapOn,
 comparing:()=>!!comparison,
 document:()=>current()||null,
 textarea:()=>findEl('[data-ed-text]'),
 diskView:()=>diskView(),
 lineStarts:d=>lineIndex(d),
 // The single view measures each line's wrapped height for its line numbers.
 wrapHeights:(d,text,count)=>!paneWorkspace()&&!d.disk&&d.gutterCache?.text===text&&d.gutterCache.heights?.length===count?d.gutterCache.heights:null,
 tabSize:()=>tabSpaces||4,
 language:d=>languageOf(d),
 commentStyle:id=>languageTools?.commentStyle?.(id)||null,
 syntax:()=>syntax,
 message:setMessage
});
function syncDocumentMapControl(){const box=findEl('[data-ed-document-map]');if(box)box.checked=documentMapOn;documentMap?.schedule();}
function setDocumentMap(on){documentMapOn=on===true;syncDocumentMapControl();keepPreferences();if(overlay&&!overlay.hidden)setMessage(documentMapOn?(comparison?'Document map is on. It shows again when the comparison closes.':'Document map shown at the right edge.'):'Document map hidden.');}
// The Document List (editor-document-list.js): the tabs in tab order, by view
// while the editor shows two views.
function documentModel(){
 const workspace=paneWorkspace(),shown=new Set(workspace?Object.values(groups.selection()).filter(Boolean):[active]);
 const entry=d=>({id:d.id,uid:d.uid,name:nameOf(d.path),title:[d.source,d.path].filter(Boolean).join(' · '),dirty:!!d.dirty,saving:!!d.saving,active:d.id===active,shown:shown.has(d.id)});
 if(!workspace)return {split:false,views:[{side:'single',label:'',documents:[...documents.values()].map(entry)}]};
 return {split:true,views:['left','right'].map(side=>({side,label:side==='left'?'Left view':'Right view',documents:groups.list(side).map(entry)}))};
}
// The View options in More (after Text colors) and the map panel at the right
// edge of the workspace, added once when the editor is built.
function mountDocumentView(){
 overlay.querySelector('[data-ed="colors"]')?.insertAdjacentHTML('afterend','<span class="nt-editor-more-heading" role="presentation">View</span><label class="nt-editor-close-preference" title="A miniature of the whole document at the right edge. Click or drag it to move through the document."><input type="checkbox" data-ed-document-map> Document map</label><label class="nt-editor-close-preference" title="The open documents in the editor sidebar, beside Connect and Files."><input type="checkbox" data-ed-document-list> Document list</label>');
 overlay.querySelector('.nt-editor-workspace')?.insertAdjacentHTML('beforeend','<aside class="nt-editor-map" data-ed-map hidden aria-label="Document map"></aside>');
 overlay.addEventListener('change',event=>{const t=event.target;if(t.matches?.('[data-ed-document-map]'))setDocumentMap(t.checked);else if(t.matches?.('[data-ed-document-list]'))root.NetworkToolkitEditorFiles?.showDocuments?.(t.checked,{focus:t.checked});});
 documentMap=root.NTKEditorDocumentMap?.create(documentMapAdapter)||null;documentMap?.mount(overlay.querySelector('[data-ed-map]'));syncDocumentMapControl();
}
function openTabMenuAt(uid,x,y,back=null){const d=[...documents.values()].find(item=>item.uid===uid);if(!d||!overlay||overlay.hidden)return false;openTabMenu(d,x,y,back);return true;}
function focusText(){findEl('[data-ed-text]')?.focus({preventScroll:true});}

function skin(){if(overlay)return;readPreferences();
 overlay=document.createElement('div');overlay.id='networkToolkitFileEditor';overlay.className='nt-editor';overlay.hidden=true;
 overlay.innerHTML='<div class="nt-editor-window" role="dialog" aria-modal="true" aria-label="Network Toolkit text editor">'+
 '<div class="nt-editor-toolbar"><button class="btn small ghost" data-ed="browse" title="Open a Windows file with editing permissions (Ctrl+O)">Open file</button><button class="btn small primary" data-ed="save" title="Save (Ctrl+S)">Save</button><button class="btn small ghost" data-ed="saveAs" title="Save As (Ctrl+Shift+S)">Save as</button>'+editorIconButton('reload','Reload','refresh')+editorIconButton('compare','Compare documents','compare',' aria-pressed="false"')+editorIconButton('split','Split view','split',' aria-pressed="false"')+'<button type="button" class="btn small ghost" data-ed="files" aria-expanded="false">Files</button><span class="nt-editor-rule"></span>'+editorIconButton('undo','Undo','undo')+editorIconButton('redo','Redo','redo')+'<button class="btn small ghost" data-ed="find">Find</button><button class="btn small ghost" data-ed="replace">Replace</button>'+editorIconButton('goto','Go to line','goto')+'<button type="button" class="btn small ghost nt-editor-icon-button" data-ed="sendTerminal" aria-label="Send to terminal" title="Send to terminal: the selection or the whole document (Ctrl+Shift+Enter)" aria-keyshortcuts="Control+Shift+Enter">'+(typeof icon==='function'?icon('sendTerminal'):'&gt;_')+'</button><button type="button" class="btn small ghost nt-editor-tools-button" data-ed="tools" aria-haspopup="menu" aria-expanded="false" title="Text tools: lines, encode and decode, format">Tools</button><details class="nt-editor-more"><summary class="btn small ghost" aria-controls="ntEditorMoreActions" aria-expanded="false">More</summary><div id="ntEditorMoreActions" class="nt-editor-more-actions" role="group" aria-label="More editor actions" hidden><button class="btn small ghost" data-ed="download">Save copy</button><button type="button" class="btn small ghost" data-ed="colors" title="Customize syntax and editor colors">Text colors</button><label class="nt-editor-close-preference"><input type="checkbox" data-ed-ask-save-close checked aria-label="Ask to save unsaved changes before closing" aria-describedby="ntEditorClosePolicyNote"> Confirm unsaved close</label><span id="ntEditorClosePolicyNote" data-ed-close-policy-note class="nt-editor-window-announcement"></span><label class="nt-editor-close-preference"><input type="checkbox" data-ed-restore-session checked aria-describedby="ntEditorRestoreHint"> Reopen files and unsaved changes from the last session</label><small id="ntEditorRestoreHint" class="nt-editor-preference-hint">Unsaved text is kept in this app’s data folder until it is saved or its tab is closed.</small></div></details><button type="button" class="btn small ghost" data-ed="localTools" title="Open the full Text Editor in Tools">Full editor</button><button type="button" class="btn small ghost" data-ed="close" aria-label="Close text editor">Close editor</button><span data-ed-title hidden></span></div>'+
 '<div id="ntEditorRemotePanel" class="nt-editor-file-sessions" data-ed-page-panel="remote" hidden><div class="nt-editor-file-session-row"><label class="sr-only" for="ntEditorRemoteSource">Active or saved file connection</label><select id="ntEditorRemoteSource" aria-label="Connected or saved SSH, SFTP, FTP or FTPS session"><option value="">Loading connections...</option></select><button type="button" class="btn small primary" id="ntEditorConnectSaved" data-ed-page="connectSaved" hidden disabled>Connect</button><button type="button" class="btn small primary" id="ntEditorOpenRemote" data-ed-page="useRemote" disabled>Use</button><button type="button" class="btn small ghost" data-ed-page="refreshRemote" title="Refresh active and saved connections" aria-label="Refresh connections">↻</button></div><details class="nt-editor-file-path-actions"><summary class="btn small ghost">Open / create by path</summary><label>Remote file path<input id="ntEditorRemotePath" type="text" spellcheck="false" placeholder="/home/user/config.txt"></label><div class="row"><button type="button" class="btn small ghost" data-ed-page="browseRemote">Browse…</button><button class="btn small ghost" id="ntEditorOpenRemotePath" data-ed-page="openRemotePath" disabled>Open file</button><button class="btn small ghost" id="ntEditorNewRemote" data-ed-page="newRemote" disabled>New file</button></div></details><div class="nt-editor-remote-hint" id="ntEditorRemoteHint" role="status">Select an active or saved connection.</div></div>'+
 '<div class="nt-editor-find" data-ed-find hidden><label>Find <input type="text" data-ed-query placeholder="Find text (Ctrl+F)"></label><button class="btn small ghost" data-ed="previous">Previous</button><button class="btn small ghost" data-ed="next">Next</button><label><input type="checkbox" data-ed-case> Case</label><label class="nt-editor-search-mode" title="'+escaped(searchModeHelp.normal)+'" data-ed-search-mode-label>Mode <select data-ed-search-mode aria-label="Search mode">'+[['normal','Normal'],['extended','Extended (\\n, \\r, \\t, \\0, \\x…, \\u…)'],['regex','Regular expression']].map(([value,label])=>'<option value="'+value+'" title="'+escaped(searchModeHelp[value])+'">'+escaped(label)+'</option>').join('')+'</select></label><label><input type="checkbox" data-ed-word> Word</label><div data-ed-replace-fields hidden class="nt-editor-replace-fields"><label>Replace <input type="text" data-ed-replacement placeholder="Replacement text"></label><button class="btn small ghost" data-ed="replaceOne">Replace</button><button class="btn small ghost" data-ed="replaceAll">Replace all</button></div><button type="button" class="btn small ghost" data-ed="hideFind">Hide</button></div>'+
 '<div class="nt-editor-options"><label>Line endings <select data-ed-eol><option>LF</option><option>CRLF</option><option>CR</option></select></label><label>Indent <select data-ed-indent><option value="4">4 spaces</option><option value="2">2 spaces</option><option value="0">Tabs</option></select></label><label><input data-ed-wrap type="checkbox"> Wrap</label><label><input data-ed-syntax type="checkbox" checked> Highlight</label><label>Language <select data-ed-language-select>'+'<option value="" data-ed-auto-language>Auto</option>'+languageRows.map(row=>'<option value="'+escaped(row.id)+'">'+escaped(row.label)+'</option>').join('')+'</select></label><label>Encoding <select data-ed-encoding><option>UTF-8</option><option>UTF-16LE</option><option>UTF-16BE</option></select></label><label><input type="checkbox" data-ed-bom> BOM</label><label>Font size <input type="number" data-ed-font data-input-clear="off" min="10" max="24" step="1" title="Hold Ctrl and use the mouse wheel over editor text to zoom" value="13"></label><label>Editor colors <select data-ed-palette><option value="auto">Follow app</option><option value="dark">Dark</option><option value="light">Light</option><option value="pastel">Pastel</option><option value="forest">Forest</option><option value="highContrast">High contrast</option><option value="custom">Custom</option></select></label></div><section class="nt-editor-color-controls" data-ed-custom-palette hidden><div class="row between"><strong>Custom editor colors</strong><button type="button" class="btn small ghost" data-ed="colors">Close</button></div><div data-ed-color-panel></div></section>'+
 '<div class="nt-editor-workspace"><aside class="nt-editor-files-sidebar" data-ed-files-sidebar hidden></aside><div class="nt-editor-document-area"><div class="nt-editor-tabs" data-ed-tabs data-ed-view="single" role="tablist" aria-label="Open document tabs"></div><div class="nt-editor-code"><div class="nt-editor-gutter" data-ed-gutter aria-hidden="true"><div data-ed-gutter-lines></div></div><div class="nt-editor-surface"><pre class="nt-editor-highlight" data-ed-highlight aria-hidden="true"></pre><textarea data-ed-text data-input-clear="off" spellcheck="false" autocapitalize="off" autocomplete="off" autocorrect="off" wrap="off" aria-label="Editable file contents"></textarea></div></div>'+
 '</div></div><div class="nt-editor-status"><span data-ed-message></span><span data-ed-position></span><span data-ed-size></span></div></div>';
 document.body.append(overlay);syncClosePreference();root.NetworkToolkitStandalone?.decorateEditor?.(overlay);
 mountDocumentView();
 const more=overlay.querySelector('.nt-editor-more'),moreSummary=more.querySelector('summary'),moreActions=overlay.querySelector('#ntEditorMoreActions');
 more.addEventListener('toggle',()=>{
  if(!more.open&&moreActions.contains(document.activeElement))moreSummary.focus({preventScroll:true});
  moreActions.hidden=!more.open;moreSummary.setAttribute('aria-expanded',String(more.open));
  root.NTKEditorCompare?.placeToolbarDropdown(moreSummary,moreActions);
 });
 more.addEventListener('keydown',event=>{
  if(event.key!=='Escape'||!more.open)return;
  event.stopPropagation();moreSummary.focus({preventScroll:true});more.open=false;
 });
 root.addEventListener?.('resize',()=>{
  for(const menu of overlay.querySelectorAll('details[open]:is(.nt-editor-more,.nt-compare-settings)')){
   root.NTKEditorCompare?.placeToolbarDropdown(menu.querySelector('summary'),menu.querySelector('div'));
  }
 });
 moreActions.addEventListener('click',event=>{if(event.target.closest('[data-ed]'))more.open=false;});
 moreActions.addEventListener('change',()=>{more.open=false;});
 overlay.addEventListener('pointerdown',event=>{if(more.open&&!more.contains(event.target))more.open=false;
  const settings=comparisonHost?.querySelector('.nt-compare-settings');
  if(settings?.open&&!settings.contains(event.target))settings.open=false;});
 findEl('[data-ed-color-panel]').innerHTML=colorPickerHTML();
 overlay.addEventListener('dragover',editorFileDragOver,true);
 overlay.addEventListener('drop',event=>{if(event.target.closest?.('.nt-editor-document-area')&&Array.from(event.dataTransfer?.types||[]).includes(sidebarFileMIME))void openSidebarFileDrop(event);},true);
 overlay.addEventListener('dragover',event=>{if(event.dataTransfer?.types?.includes('Files')){event.preventDefault();event.dataTransfer.dropEffect='copy';}});
 overlay.addEventListener('drop',event=>{const files=[...(event.dataTransfer?.files||[])];if(files.length){event.preventDefault();event.stopPropagation();void importFiles(files);}});
 if(typeof ResizeObserver==='function')new ResizeObserver(()=>{const d=current();if(d)paintText(d);}).observe(findEl('[data-ed-text]'));
 if(typeof ResizeObserver==='function')new ResizeObserver(revealSelectedTabs).observe(overlay.querySelector('.nt-editor-document-area'));
 overlay.addEventListener('contextmenu',event=>{const tab=event.target.closest('[data-ed-uid]');if(!tab)return;event.preventDefault();const d=[...documents.values()].find(d=>d.uid===tab.dataset.edUid);if(d)openTabMenu(d,event.clientX,event.clientY);});
 document.addEventListener('pointerdown',event=>{if(tabMenu&&!tabMenu.contains(event.target))closeTabMenu(false);},true);
 overlay.addEventListener('wheel',zoomEditorText,{capture:true,passive:false});
 overlay.addEventListener('click',click);overlay.addEventListener('change',change);overlay.addEventListener('beforeinput',beforeInput);overlay.addEventListener('input',input);overlay.addEventListener('keydown',keydown);overlay.addEventListener('scroll',scroll,true);overlay.addEventListener('selectionchange',event=>{if(event.target.matches?.('[data-ed-text]')&&!current()?.disk)status();},true);root.NetworkToolkitEditorWindows?.mount?.();
}
function zoomEditorText(event){
 if(!event.ctrlKey||!event.target.closest?.('.nt-editor-code,.nt-compare-surface,.nt-disk-compare-code,.nt-compare-preview-scroll'))return;
 event.preventDefault();event.stopPropagation();
 if(event.deltaY&&current()&&!editorComposing())setEditorFontSize(editorFontSize+(event.deltaY<0?1:-1));
}
function setEditorFontSize(value){
 const next=Math.max(10,Math.min(24,Number(value)||13));
 findEl('[data-ed-font]').value=String(next);
 if(next===editorFontSize)return;
 editorFontSize=next;
 applyEditorTheme();
 const workspace=paneWorkspace();
 if(workspace?.refreshFont)workspace.refreshFont();
 else if(workspace)workspace.refresh(true);
 else if(singleDiskView)singleDiskView.refreshWrap(true);
 else paintText(current());
 keepPreferences();
}
function captureView(){const paged=diskView();if(paged)return paged.capture();const ta=findEl('[data-ed-text]');return ta?{start:ta.selectionStart||0,end:ta.selectionEnd||0,direction:ta.selectionDirection||'none',top:ta.scrollTop||0,left:ta.scrollLeft||0}:null;}
function restoreView(view,start=view?.start,end=view?.end,focus=false){const paged=diskView();if(paged&&view){void paged.restore({...view,start,end},focus).catch(error=>setMessage(error.message,true));return;}const ta=findEl('[data-ed-text]');if(!ta||!view)return;ta.setSelectionRange(Math.min(start,ta.value.length),Math.min(end,ta.value.length),view.direction);if(focus)ta.focus({preventScroll:true});ta.scrollTop=view.top;ta.scrollLeft=view.left;scroll({target:ta});}
function applyEdit(d,text,start,end,view=captureView()){if(!d||d.loading)return;setContent(d,text);const ta=findEl('[data-ed-text]');if(ta.value!==d.content)ta.value=d.content;restoreView(view,start,end,true);}
const documentStamp=d=>[d.editRevision,d.eol,d.encoding,d.bom,d.diskSession?.state().editVersion].join(':');
// Save As destinations. Local documents use the Windows Save As dialog when
// the native host provides one (its overwrite prompt is the user's consent to
// replace a file); SFTP documents use an in-app folder picker over the live
// connection; everything else keeps the typed-path prompt. Each resolver
// returns {path, overwrite} or null when the user cancelled.
function pathBaseName(path){const text=String(path||'');const at=Math.max(text.lastIndexOf('/'),text.lastIndexOf('\\'));return at>=0?text.slice(at+1):text;}
function pathDirName(path){const text=String(path||'');const at=Math.max(text.lastIndexOf('/'),text.lastIndexOf('\\'));if(at<0)return '';const dir=text.slice(0,at);return /^[A-Za-z]:$/.test(dir)?dir+'\\':dir===''?text.slice(0,1):dir;}
function isLocalDocument(d){const kind=d?.binding?.kind;if(kind==='local'||kind==='localDraft')return true;if(kind)return false;return ['Local computer','Local files','Untitled','Desktop import'].includes(d?.source);}
function joinRemotePath(dir,name){const base=String(dir||'').replace(/\/+$/,'');return base===''||base==='.'?name:base+'/'+name;}
function validSaveName(name){return typeof name==='string'&&name.trim()!==''&&name.length<=255&&!/[\\/:*?"<>|\x00-\x1f]/.test(name)&&name!=='.'&&name!=='..';}
async function nativeSaveDialog(d){
 if(typeof api!=='function')return undefined;
 try{const result=await api('/file-editor/save-dialog','POST',{suggested:pathBaseName(d.untitled?'':d.path)||'untitled.txt',directory:d.untitled?'':pathDirName(d.path)});
  if(result?.cancelled===true)return null;
  if(typeof result?.path==='string'&&result.path.trim())return {path:result.path.trim(),overwrite:true};
  return undefined;
 }catch(error){if(error?.status===409){setMessage(error.message||'A Save As dialog is already open.',true);return null;}return undefined;}
}
async function promptSaveAsPath(d){
 const other=await askEditor({title:'Save as',message:'Enter a new path. Existing destination files will not be overwritten.',value:d.path,accept:'Save',validate:value=>value.trim()&&value.length<=4096&&value.trim()!==d.path?'':'Enter a different file path.'});
 return other===null?null:{path:other.trim(),overwrite:false};
}
async function chooseSaveAsPath(d){
 if(isLocalDocument(d)){const native=await nativeSaveDialog(d);if(native!==undefined)return native;}
 else if(typeof d.list==='function')return remoteSaveAsPicker(d);
 return promptSaveAsPath(d);
}
// Folder picker for SFTP documents: browse directories over the document's own
// connection, name the file, and confirm before replacing an existing one.
function remoteSaveAsPicker(d){
 if(pendingEditorDialog)pendingEditorDialog.finish(null);
 const returnView=captureView(),owner=current(),panel=document.createElement('section');
 panel.className='nt-editor-dialog';panel.setAttribute('role','dialog');panel.setAttribute('aria-modal','true');panel.setAttribute('aria-label','Save as on the server');
 panel.innerHTML='<div class="nt-editor-dialog-card nt-editor-saveas"><h3>Save as</h3><p>Choose a folder on the server and a file name.</p><div class="row gap6"><button type="button" class="btn small ghost" data-ed-pick-action="up" title="Parent folder">Up</button><code class="nt-editor-saveas-path" data-ed-pick-path></code></div><ul class="nt-editor-saveas-list" data-ed-pick-list role="listbox" aria-label="Folders"></ul><label>File name<input data-ed-pick-name data-input-clear="off" type="text" autocomplete="off" spellcheck="false"></label><p data-ed-dialog-error role="status"></p><div class="row"><button type="button" class="btn ghost" data-ed-pick-action="cancel">Cancel</button><button type="button" class="btn primary" data-ed-pick-action="save">Save</button></div></div>';
 overlay.querySelector('.nt-editor-window').append(panel);
 const nameInput=panel.querySelector('[data-ed-pick-name]'),pathEl=panel.querySelector('[data-ed-pick-path]'),listEl=panel.querySelector('[data-ed-pick-list]'),errorEl=panel.querySelector('[data-ed-dialog-error]');
 nameInput.value=pathBaseName(d.path);
 let dir=pathDirName(d.path)||'.',parent='',entries=[],loading=false;
 return new Promise(resolve=>{
  const finish=result=>{panel.remove();if(pendingEditorDialog?.panel===panel)pendingEditorDialog=null;if(current()===owner)restoreView(returnView,returnView?.start,returnView?.end,true);resolve(result);};
  // Enter and Escape arrive through the editor's shared dialog key handling.
  const pending={panel,finish:value=>{if(value===null)finish(null);else panel.querySelector('[data-ed-pick-action="save"]').click();},validate:null};pendingEditorDialog=pending;
  const render=()=>{pathEl.textContent=dir;listEl.innerHTML=entries.filter(e=>e.directory).map(e=>'<li role="option"><button type="button" class="btn small ghost" data-ed-pick-dir="'+escaped(e.path||joinRemotePath(dir,e.name))+'">'+escaped(e.name)+'/</button></li>').join('')||'<li class="nt-editor-saveas-empty">No subfolders</li>';panel.querySelector('[data-ed-pick-action="up"]').disabled=!parent||loading;};
  const load=async target=>{loading=true;errorEl.textContent='';render();try{const data=await d.list(target);dir=data?.path||target;parent=data?.parent||'';entries=(data?.entries||[]).map(e=>({name:e.name,path:e.path,directory:!!e.directory}));}catch(error){errorEl.textContent=error.message||'Folder could not be listed.';}finally{loading=false;render();}};
  panel.addEventListener('click',async event=>{const dirButton=event.target.closest('[data-ed-pick-dir]');if(dirButton){await load(dirButton.dataset.edPickDir);nameInput.focus({preventScroll:true});return;}
   const action=event.target.closest('[data-ed-pick-action]')?.dataset.edPickAction;if(!action)return;
   if(action==='cancel'){finish(null);return;}
   if(action==='up'){if(parent)await load(parent);return;}
   if(action==='save'){const name=nameInput.value.trim();if(!validSaveName(name)){errorEl.textContent='Enter a valid file name without path separators.';nameInput.focus({preventScroll:true});return;}
    const path=joinRemotePath(dir,name);if(path===d.path){errorEl.textContent='Choose a different folder or name.';return;}
    const existing=entries.find(e=>e.name===name);if(existing?.directory){errorEl.textContent='A folder has that name.';return;}
    let overwrite=false;if(existing){pendingEditorDialog=null;const confirmed=await askEditor({title:'Replace file?',message:name+' already exists in '+dir+'. Replace it with this document?',accept:'Replace'});if(!panel.isConnected)return;pendingEditorDialog=pending;if(confirmed!==true)return;overwrite=true;}
    finish({path,overwrite});}
  });
  void load(dir).then(()=>nameInput.focus({preventScroll:true}));
 });
}
function askEditor({title,message='',value,accept='Continue',validate,check}){if(pendingEditorDialog)pendingEditorDialog.finish(null);const returnView=captureView(),owner=current(),panel=document.createElement('section');panel.className='nt-editor-dialog';panel.setAttribute('role','dialog');panel.setAttribute('aria-modal','true');panel.setAttribute('aria-label',title);panel.innerHTML='<div class="nt-editor-dialog-card"><h3>'+escaped(title)+'</h3><p>'+escaped(message)+'</p>'+(value===undefined?'':'<label>'+escaped(title)+'<input data-ed-dialog-input data-input-clear="off" type="text" autocomplete="off" spellcheck="false" value="'+escaped(value)+'"></label>')+(check?'<label class="nt-editor-dialog-check"><input type="checkbox" data-ed-dialog-check'+(check.checked?' checked':'')+'> '+escaped(check.label)+'</label>':'')+'<p data-ed-dialog-error role="status"></p><div class="row"><button type="button" class="btn ghost" data-ed-dialog-action="cancel">Cancel</button><button type="button" class="btn primary" data-ed-dialog-action="accept">'+escaped(accept)+'</button></div></div>';overlay.querySelector('.nt-editor-window').append(panel);return new Promise(resolve=>{const finish=result=>{panel.remove();pendingEditorDialog=null;if(current()===owner)restoreView(returnView,returnView?.start,returnView?.end,true);resolve(result);};pendingEditorDialog={panel,finish,validate,check:!!check};(panel.querySelector('input')||panel.querySelector('button'))?.focus({preventScroll:true});panel.querySelector('input')?.select?.();});}
function answerEditorDialog(accept){const pending=pendingEditorDialog;if(!pending)return;if(!accept){pending.finish(null);return;}const value=(pending.panel.querySelector('[data-ed-dialog-input]')||pending.panel.querySelector('input'))?.value??true,error=pending.validate?.(value);if(error){pending.panel.querySelector('[data-ed-dialog-error]').textContent=error;pending.panel.querySelector('input')?.focus({preventScroll:true});return;}pending.finish(pending.check?{value,checked:!!pending.panel.querySelector('[data-ed-dialog-check]')?.checked}:value);}
const LARGE_EDITOR_CHARS=2*1024*1024,HISTORY_MEMORY_LIMIT=128*1024*1024;
const largeDocument=d=>!!d&&(d.disk||d.content.length>LARGE_EDITOR_CHARS);
const effectiveWrap=d=>wordWrap&&!largeDocument(d);
const workspaceLarge=()=>Object.values(groups.selection()).some(id=>largeDocument(documents.get(id)));
function lineIndex(d){if(d.lineIndex?.text===d.content)return d.lineIndex.starts;const starts=[0];let at=-1;while((at=d.content.indexOf('\n',at+1))!==-1)starts.push(at+1);d.lineIndex={text:d.content,starts};return starts;}
function sourceLineAt(starts,offset){let low=0,high=starts.length;while(low+1<high){const mid=(low+high)>>1;if(starts[mid]<=offset)low=mid;else high=mid;}return low;}
function retainUndo(d,list){while(list.length>80)list.shift();if(!largeDocument(d))return;let bytes=list.reduce((sum,text)=>sum+text.length*2,0);while(list.length>1&&bytes>HISTORY_MEMORY_LIMIT){bytes-=list.shift().length*2;d.historyLimited=true;}}
function largeGutter(d,ta,gutter){const starts=lineIndex(d),style=getComputedStyle(ta),padding=parseFloat(style.paddingTop)||10,bottom=parseFloat(style.paddingBottom)||20,computed=parseFloat(style.lineHeight)||20;const height=starts.length>1&&ta.scrollHeight>ta.clientHeight?Math.max(1,(ta.scrollHeight-padding-bottom)/starts.length):computed,first=Math.max(0,Math.floor((ta.scrollTop-padding)/height)-3),last=Math.min(starts.length,first+Math.ceil((ta.clientHeight||600)/height)+8),key=[d.uid,d.editRevision,first,last,height.toFixed(3)].join(':');if(gutter._ntLargeRange===key)return;gutter._ntLargeRange=key;gutter.style.paddingTop=(first*height)+'px';gutter.innerHTML=Array.from({length:Math.max(0,last-first)},(_,i)=>'<div style="height:'+height+'px;line-height:'+height+'px">'+(first+i+1)+'</div>').join('');}
function paintGutter(d,ta,gutter){if(largeDocument(d)){largeGutter(d,ta,gutter);return;}gutter._ntLargeRange=null;gutter.style.paddingTop='';const rows=d.content.split('\n').slice(0,30000);if(!wordWrap){gutter.innerHTML=rows.map((_,i)=>'<div>'+(i+1)+'</div>').join('');return;}const style=getComputedStyle(ta),width=Math.max(1,ta.clientWidth-parseFloat(style.paddingLeft||0)-parseFloat(style.paddingRight||0)),key=[width,style.font,style.lineHeight,tabSpaces].join(':');let heights=d.gutterCache?.text===d.content&&d.gutterCache.key===key?d.gutterCache.heights:null;if(!heights){const measure=document.createElement('div');measure.className='nt-editor-wrap-measure';Object.assign(measure.style,{width:width+'px',font:style.font,lineHeight:String(Number((parseFloat(style.lineHeight)/parseFloat(style.fontSize)).toFixed(6))),tabSize:String(tabSpaces||4)});measure.innerHTML=rows.map(line=>'<div>'+escaped(line||'\u200b')+'</div>').join('');overlay.append(measure);heights=[...measure.children].map(line=>line.getBoundingClientRect().height||parseFloat(style.lineHeight));measure.remove();d.gutterCache={text:d.content,key,heights};}gutter.innerHTML=heights.map((height,i)=>'<div style="height:'+height+'px">'+(i+1)+'</div>').join('');}
function positionAt(offset,focus=true){const paged=diskView();if(paged){void paged.position(offset,offset,focus).catch(error=>setMessage(error.message,true));return;}if(paneWorkspace()){paneWorkspace().positionAt(offset,focus);return;}const ta=findEl('[data-ed-text]'),d=current();ta.setSelectionRange(offset,offset);if(focus)ta.focus({preventScroll:true});const line=sourceLineAt(lineIndex(d),offset)+1;if(effectiveWrap(d)){const gutter=findEl('[data-ed-gutter-lines]');ta.scrollTop=Math.max(0,[...gutter.children].slice(0,line-1).reduce((sum,row)=>sum+row.getBoundingClientRect().height,0)-parseFloat(getComputedStyle(ta).lineHeight));}else ta.scrollTop=Math.max(0,(line-2)*parseFloat(getComputedStyle(ta).lineHeight));scroll({target:ta});}
async function goToLine(){const d=current();if(!d)return;
 if(d.disk){
  const count=d.diskMeta?.lines||1,paged=diskView();
  if(!paged)return;
  const value=await askEditor({title:'Go to line',message:'Enter a line number from 1 to '+count+'.',value:String((paged.cursor?.line||0)+1),accept:'Go',validate:value=>Number.isSafeInteger(Number(value))&&Number(value)>=1&&Number(value)<=count?'':'Enter an existing line number.'});
  if(value!==null&&current()===d)await paged.goLine(Number(value)-1,true).catch(error=>setMessage(error.message,true));
  return;
 }
 const starts=lineIndex(d);const value=await askEditor({title:'Go to line',message:'Enter a line number from 1 to '+starts.length+'.',value:String(sourceLineAt(starts,findEl('[data-ed-text]').selectionStart)+1),accept:'Go',validate:value=>Number.isSafeInteger(Number(value))&&Number(value)>=1&&Number(value)<=lineIndex(d).length?'':'Enter an existing line number.'});if(value===null||current()!==d||documents.get(d.id)!==d)return;positionAt(lineIndex(d)[Number(value)-1]);}
function setMessage(s,error=false){const el=findEl('[data-ed-message]');if(el){el.textContent=s;el.classList.toggle('error',error);}}
function activateDocument(id,focus=false){const d=documents.get(id);if(!d)return false;if(editorComposing()&&current()!==d){setMessage('Finish text composition before switching documents.',true);return false;}if(!paneWorkspace()&&displayedDocument)displayedDocument.view=captureView();active=id;groups.activate(d.uid);if(paneWorkspace()){displayedDocument=null;drawTabs();paneWorkspace().showDocument(id);}else draw();if(focus)findEl('[data-ed-text]')?.focus({preventScroll:true});noteAsked(id);return true;}
// Reordering changes presentation order only: keep the same document objects,
// active identity and live editor/compare DOM (including selection and history).
function canReorderDocuments({ignoreDragPreparation=false}={}){const windows=root.NetworkToolkitEditorWindows?.state;if(windowCloseState||pendingEditorDialog||paneWorkspace()?.composing||windows?.closing||windows?.outgoing||windows?.incoming||windows?.detaching||(!ignoreDragPreparation&&windows?.dragPreparing)||[...documents.values()].some(d=>d.loading||d.reloading||d.saving||d.formatting||d.frozen))return 'Finish pending editor operations before rearranging tabs.';return '';}
function reorderDocument(uid,beforeUID=null){const error=canReorderDocuments();if(error){setMessage(error,true);return false;}const list=[...documents.values()],from=list.findIndex(d=>d.uid===uid);if(from<0||(beforeUID!==null&&!list.some(d=>d.uid===beforeUID))){setMessage('The document tab is no longer available.',true);return false;}if(uid===beforeUID)return true;const selected=list.splice(from,1)[0],to=beforeUID===null?list.length:list.findIndex(d=>d.uid===beforeUID);list.splice(to,0,selected);if(list.every((d,index)=>d===[...documents.values()][index]))return true;documents.clear();for(const d of list)documents.set(d.id,d);drawTabs();root.NetworkToolkitEditorWindows?.notifyChanged?.();setMessage('Tab moved to position '+(to+1)+' of '+list.length+'.');return true;}
function createPaneWorkspace(comparing){
 skin();paintGeneration++;if(paintTimer!==null){root.clearTimeout(paintTimer);paintTimer=null;}root.NTKEditorLanguageService?.cancelHighlight?.();
 if(displayedDocument)displayedDocument.view=captureView();disposeSingleDiskView();displayedDocument=null;
 groups.enable(current()?.uid);comparisonHost=document.createElement('section');overlay.querySelector('.nt-editor-code').before(comparisonHost);
 overlay.classList.toggle('is-comparing',comparing);overlay.classList.toggle('is-split',!comparing);
 const factory=[...documents.values()].some(d=>d.disk)&&root.NTKEditorDiskCompare?root.NTKEditorDiskCompare:root.NTKEditorCompare;
 paneUsesDisk=factory===root.NTKEditorDiskCompare;
 const instance=factory.create({container:comparisonHost,comparing,ensureDiskSession,mountDiskView,compareRequest:(body,signal)=>root.NetworkToolkitEditorWindows.compareBuffers(body,signal),documents:()=>[...documents.values()],active:current,wordWrap:()=>wordWrap&&(paneUsesDisk||!workspaceLarge()),comparisonPreferences:()=>({compareMapZoom,compareHorizontalSync,compareUnderline,compareIntralineHighlight,compareWrapNavigation,compareMoveCaret,compareShowSummary}),setComparisonPreference:(key,value)=>{if(key==='compareMapZoom'&&mapZooms.includes(value))compareMapZoom=value;if(key==='compareHorizontalSync')compareHorizontalSync=value===true;if(key==='compareUnderline')compareUnderline=value!==false;if(key==='compareIntralineHighlight')compareIntralineHighlight=value!==false;if(key==='compareWrapNavigation')compareWrapNavigation=value!==false;if(key==='compareMoveCaret')compareMoveCaret=value!==false;if(key==='compareShowSummary')compareShowSummary=value!==false;keepPreferences();},syntax:()=>syntax,language:languageOf,selection:groups.selection,sideOf:groups.sideOf,activeSide:()=>groups.activeSide,
 choose:(side,id)=>{const d=documents.get(id);if(d){groups.choose(side,d.uid,{replace:comparing&&!splitEnabled});active=id;drawTabs();}},swap:()=>{groups.swap();drawTabs();},
 selectionChanged:status,activate:d=>{active=d.id;groups.activate(d.uid);displayedDocument=null;drawTabs();syncEditorControls(d);},edit:(d,text)=>setContent(d,text),
 undo:(d,redo)=>{if(!d||d.frozen||d.loading)return;const from=redo?d.redo:d.undo,to=redo?d.undo:d.redo,previous=from.pop();if(previous===undefined)return;to.push(d.content);retainUndo(d,to);setContent(d,previous,false);},save:(d,as)=>save(as,d),createInView:async side=>{try{const d=await newDocument();if(d){groups.move(d.uid,side);active=d.id;drawTabs();}return d;}catch(error){setMessage(error.message,true);return null;}},newDocument,openFiles:openDesktopFile});
 if(comparing)comparison=instance;else splitView=instance;drawTabs();if(current())syncEditorControls(current());
}
function updatePaneFactory(){
 if(!paneWorkspace()||paneUsesDisk||!root.NTKEditorDiskCompare||![...documents.values()].some(d=>d.disk))return;
 const comparing=!!comparison,snapshot=paneWorkspace().snapshot(),returnLayout=comparisonReturnLayout;
 disposePaneWorkspace();
 createPaneWorkspace(comparing);
 paneWorkspace()?.restore(snapshot);
 comparisonReturnLayout=returnLayout;
}
function disposePaneWorkspace(){paneUsesDisk=false;comparisonReturnLayout=null;comparison?.dispose();splitView?.dispose();comparison=null;splitView=null;comparisonHost?.remove();comparisonHost=null;overlay?.classList.remove('is-comparing','is-split');displayedDocument=null;}
function openComparison(){if(comparison)return;const saved=paneWorkspace()?.snapshot();if(splitView)disposePaneWorkspace();createPaneWorkspace(true);if(saved)comparison.restore(saved);drawTabs();}
function openSplit(){if(editorComposing()){setMessage('Finish text composition before changing editor views.',true);return false;}if(paneWorkspace()){if(comparison)closeComparison();disposePaneWorkspace();splitEnabled=false;groups.single();draw();return;}splitEnabled=true;createPaneWorkspace(false);}
function moveDocumentToView(uid,side,beforeUID=null){const error=canReorderDocuments();if(error){setMessage(error,true);return false;}const d=[...documents.values()].find(d=>d.uid===uid);if(!d)return false;side=side|| (groups.sideOf(uid)==='left'?'right':'left');comparisonReturnLayout=null;groups.move(uid,side);splitEnabled=true;active=d.id;if(!paneWorkspace())createPaneWorkspace(false);if(beforeUID)reorderDocument(uid,beforeUID);drawTabs();paneWorkspace().showDocument(d.id);root.NetworkToolkitEditorWindows?.notifyChanged?.();return true;}
function syncEditorControls(d){if(!d)return;for(const [selector,value] of [['[data-ed-language-select]',d.language||''],['[data-ed-indent]',String(tabSpaces)],['[data-ed-font]',String(editorFontSize)],['[data-ed-palette]',editorPalette]])findEl(selector).value=value;formatControls(d);const wrap=findEl('[data-ed-wrap]');wrap.checked=wordWrap;wrap.disabled=!paneUsesDisk&&!!paneWorkspace()&&workspaceLarge();wrap.title=wrap.disabled?'Wrap is paused in large-file mode; your saved preference is retained.':'';findEl('[data-ed-syntax]').checked=syntax;findEl('[data-ed-find]').hidden=!showFind;findEl('[data-ed-replace-fields]').hidden=!showReplace;applyEditorTheme();syncLanguageControls(d);status();}
function closeComparison(){if(!comparison)return;if(comparison.composing){setMessage('Finish text composition before closing the comparison.',true);return false;}const restore=comparisonReturnLayout;disposePaneWorkspace();if(restore)groups.restore(restore);if(splitEnabled)createPaneWorkspace(false);else{groups.single();draw();}}
function setFileValues(d){d.editRevision=(d.editRevision||0)+1;d.eol=lineMode(d.original);d.content=normalize(d.original);d.saved=d.content;d.savedEol=d.eol;d.savedBOM=d.bom;d.savedEncoding=d.encoding||'UTF-8';d.dirty=false;d.saveFailed=false;d.undo=[];d.redo=[];d.bom=!!d.bom;d.showStart=true;}
async function open({id,source,path,read,write,afterSave,list,binding=null,create=false,initialContent='',diskMode='',diskFile=null,diskRemote=null,desktopHandle=null,restore=null}){
 if(windowCloseState)throw Error('The editor is waiting for window close confirmation.');
 if(editorComposing())throw Error('Finish text composition before opening another document.');
 if(!path||typeof write!=='function'||typeof read!=='function')throw Error('Editor requires a file path and read/write functions.');skin();
 if(pendingSavePath===path)throw Error('This path is the destination of a pending Save As. Wait for that save to finish.');
 const key=String(id||source+':'+path),previous=active;
 let doc=documents.get(key);
 if(doc){noteAsked(key);active=key;doc.showStart=true;overlay.hidden=false;root.NetworkToolkitEditorFiles?.initialize?.(overlay);draw();if(doc.loading)return doc.loadingPromise;if(!comparison)findEl('[data-ed-text]')?.focus();return doc;}
 if(documents.size>=12)throw Error('Close an editor tab before opening another file (12 maximum).');
 startSession();
 doc={id:key,restoreRequest:restore,uid:root.crypto?.randomUUID?.()||'xxxxxxxx-xxxx-4xxx-yxxx-xxxxxxxxxxxx'.replace(/[xy]/g,c=>{const n=Math.floor(Math.random()*16);return (c==='x'?n:(n&3)|8).toString(16);}),binding,path,source:source||'SFTP',read,write,afterSave,diskRemote,desktopHandle,diskFile,list:typeof list==='function'?list:null,hash:'',encoding:'UTF-8',bom:false,original:initialContent,content:'',saved:'',dirty:false,eol:'LF',undo:[],redo:[],language:'',untitled:source==='Untitled',loading:!create};
 const paged=!!diskAvailable()&&(!!diskMode||!create&&['local','terminal','sshConnection','ftp'].includes(binding?.kind));
 if(paged)doc.loading=true;
 setFileValues(doc);documents.set(key,doc);if(!paged)root.NetworkToolkitEditorWindows?.prepareDocument?.(doc);
 // A tab of the last session reopens behind a document that was asked for
 // (see noteAsked); every other new document becomes the active tab.
 const behind=restoringRecord&&!!askedBeforeRestore&&!!current();
 if(!behind){noteAsked(key);active=key;}
 overlay.hidden=false;root.NetworkToolkitEditorFiles?.initialize?.(overlay);applyEditorTheme();if(behind)drawTabs();else draw();
 if(!create||paged){
  doc.loadingPromise=(async()=>{
   try{const result=paged?await openDiskDocument(doc,diskMode,diskFile):await read(path);if(result.binary)throw Error('Binary files cannot be opened in the text editor.');
    if(documents.get(key)!==doc)return;
    doc.loading=false;
    if(!result.disk){doc.original=result.content||'';doc.hash=result.hash||'';doc.bom=result.bom===true;doc.encoding=result.encoding||'UTF-8';setFileValues(doc);if(result.eol)doc.eol=doc.savedEol=result.eol;if(paged)root.NetworkToolkitEditorWindows?.prepareDocument?.(doc);}
    else updatePaneFactory();
    if(active===key){
     draw();
     if(doc.disk){setMessage(openedNote(doc));if(notText(doc)&&typeof toast==='function')toast(openedNote(doc),'info');}
     else if(!comparison)findEl('[data-ed-text]')?.focus();
    }else paneWorkspace()?.refresh();
    return doc;
   }catch(error){
    if(documents.get(key)===doc){documents.delete(key);if(active===key){active=documents.has(previous)?previous:[...documents.keys()].at(-1)||'';if(active){draw();setMessage('Cannot open '+nameOf(path)+': '+error.message,true);}else overlay.hidden=true;}}
    throw error;
   }
  })();return doc.loadingPromise;
 }
 if(!comparison)findEl('[data-ed-text]')?.focus();return doc;
}
function doUndo(d,redo=false){if(!d||d.loading||d.frozen)return;if(d.disk){void diskView()?.history(redo).catch(error=>setMessage(error.message,true));return;}const view=captureView(),from=redo?d.redo:d.undo,to=redo?d.undo:d.redo;const previous=from.pop();if(previous===undefined)return;to.push(d.content);retainUndo(d,to);d.content=previous;d.editRevision=(d.editRevision||0)+1;d.dirty=d.content!==d.saved||d.eol!==d.savedEol||d.bom!==d.savedBOM||d.encoding!==d.savedEncoding;syncText(d);restoreView(view,view.start,view.end,true);}
function setContent(d,value,record=true,inputEdit=null,caret=null){if(!d||d.frozen||d.disk||d.promoting)return false;if(value===d.content)return true;
 if(canPromote(d)&&shouldPage(value)){
  if(d.saving){setMessage('Finish the current save before adding more text.',true);return false;}
  return promoteDocument(d,value,caret);
 }
 if(value.length>EDITOR_DOCUMENT_BYTES){setMessage('The edit exceeds the 32 MiB decoded document limit. Existing text is unchanged.',true);return false;}const previous=d.content,priorRevision=d.editRevision,cached=d.lineIndex;if(record){d.undo.push(d.content);retainUndo(d,d.undo);d.redo=[];}d.content=value;d.editRevision=(d.editRevision||0)+1;const updated=inputEdit?.revision===priorRevision?root.NTKEditorLargeIndex?.update?.(cached,previous,value,inputEdit,caret):null;d.lineIndex=updated?{text:value,starts:updated}:null;d.dirty=d.content!==d.saved||d.eol!==d.savedEol||d.bom!==d.savedBOM||d.encoding!==d.savedEncoding;paintText(d);drawTabs();status();paneWorkspace()?.refresh();return true;}
function syncText(d){paneWorkspace()?.refresh(true);const el=findEl('[data-ed-text]');if(el)el.value=d.content;paintText(d);drawTabs();status();}
// The size of the selection in the whole document, so a selection that ends at
// a page edge of a large file cannot pass for the whole document.
function selectedNote(range){const count=range?Math.max(0,range.end-range.start):0;return count?' · '+count.toLocaleString()+' selected':'';}
function status(){const d=current(),text=findEl('[data-ed-text]');if(!d||!text)return;
 if(d.disk){
  const meta=d.diskMeta,view=diskView(),cursor=view?.cursor;
  if(meta){findEl('[data-ed-size]').textContent=meta.units.toLocaleString()+' chars, '+meta.lines.toLocaleString()+' lines'+(d.dirty?' · Unsaved':'')+(notText(d)?' · Read-only: not a text file':d.eol==='Mixed'?' · Mixed line endings':'');}
  if(cursor)findEl('[data-ed-position]').textContent='Ln '+(cursor.line+1).toLocaleString()+', Col '+(cursor.column+1).toLocaleString()+selectedNote(view.selection());
  return;
 }
 const pos=text.selectionStart,starts=lineIndex(d),at=sourceLineAt(starts,pos),column=pos-starts[at]+1;findEl('[data-ed-position]').textContent='Ln '+(at+1)+', Col '+column+selectedNote({start:pos,end:text.selectionEnd});const mode=largeDocument(d)?' · Large-file mode: plain text, Wrap paused; undo up to 128 MiB / 80 entries':'';findEl('[data-ed-size]').textContent=d.content.length.toLocaleString()+' chars, '+starts.length.toLocaleString()+' lines'+(d.dirty?' · Unsaved':'')+mode;}

// Send to terminal: snapshot the active document (and its selection) for the
// dialog in editor-send-terminal.js; a floating editor is hidden so the
// Terminal is visible. Documents and unsaved changes stay open.
function sendToTerminal(){const d=current(),ta=findEl('[data-ed-text]'),send=root.NetworkToolkitEditorSendTerminal;if(!d||d.loading||!send)return;if(d.disk){const view=diskView();if(view)void sendDiskTextToTerminal(d,view,send);return;}const a=Math.min(ta?.selectionStart||0,d.content.length),b=Math.min(ta?.selectionEnd||0,d.content.length);
 send.open({name:nameOf(d.path),content:d.content,selection:b>a?d.content.slice(a,b):'',stamp:()=>current()===d?String(d.editRevision||0)+':'+d.content.length:'changed',hideFloating:()=>{if(!embedded&&overlay)overlay.hidden=true;}});}
// Insert text from the Terminal (terminal-send-editor.js). Edits go through
// setContent, so they are undoable and mark the document unsaved; a document
// that is loading, moving between windows or under comparison is refused.
function insertBlocked(d){return !d?'That document is no longer open.':d.loading?'That document is still loading.':d.frozen?'That document is being moved between windows.':d.promoting||d.diskEditing||d.reloading?'Wait for the document operation to finish.':comparison?'Close the document comparison first.':'';}
function listDocuments(){return [...documents.values()].map(d=>({id:d.id,uid:d.uid,name:nameOf(d.path),source:d.source||'',path:d.path||'',dirty:!!d.dirty,active:d.id===active,blocked:insertBlocked(d)}));}
function lineStart(content,line){if(line<=1)return 0;let at=0;for(let n=1;n<line;n++){const next=content.indexOf('\n',at);if(next<0)return content.length;at=next+1;}return at;}
function insertText(id,text,where='end'){const d=documents.get(id),blocked=insertBlocked(d);if(blocked){setMessage(blocked,true);return {ok:false,error:blocked};}if(typeof text!=='string'||!text)return {ok:false,error:'There is no text to insert.'};
 if(current()!==d&&!activateDocument(id))return {ok:false,error:'Could not switch to that document.'};if(d.disk)return insertDiskText(d,text,where);const ta=findEl('[data-ed-text]');if(!ta)return {ok:false,error:'The editor is not ready.'};
 let a,b;if(where==='cursor'){a=Math.min(ta.selectionStart||0,d.content.length);b=Math.min(ta.selectionEnd||0,d.content.length);}else if(where==='start'){a=b=0;}else if(where&&typeof where==='object'&&Number.isSafeInteger(where.line)&&where.line>0){a=b=lineStart(d.content,where.line);}else{a=b=d.content.length;}
 if(where==='end'&&a>0&&!/\n$/.test(d.content)&&!/^\n/.test(text))text='\n'+text;
 const before=d.content;applyEdit(d,d.content.slice(0,a)+text+d.content.slice(b),a+text.length,a+text.length);if(d.promotionResult)return d.promotionResult;if(d.content===before)return {ok:false,error:'The text could not be inserted.'};
 setMessage('Inserted '+text.length.toLocaleString()+' characters from the Terminal. Undo (Ctrl+Z) removes them.');return {ok:true};}
// "Choose the position": show a bar above the document; the user places the
// cursor (or selects text to replace) and chooses Insert here.
let pickState=null;
function endPick(){pickState=null;overlay?.querySelector('[data-ed-insert-bar]')?.remove();}
function pickInsert(id,text,label='Terminal output'){const d=documents.get(id),blocked=insertBlocked(d);if(blocked){setMessage(blocked,true);return false;}if(!activateDocument(id,true))return false;endPick();
 const lines=(text.match(/\r\n|\r|\n/g)||[]).length+(/[\r\n]$/.test(text)?0:1);pickState={id,text};
 const bar=document.createElement('div');bar.className='nt-editor-insert-bar';bar.dataset.edInsertBar='';bar.setAttribute('role','region');bar.setAttribute('aria-label','Insert terminal output');
 bar.innerHTML='<span class="nt-editor-insert-text"><strong>'+escaped(label)+'</strong> ('+lines.toLocaleString()+(lines===1?' line':' lines')+') is ready for <strong>'+escaped(nameOf(d.path))+'</strong>. Click where it should go, or select text to replace, then choose Insert here.</span><span class="nt-editor-insert-actions"><button type="button" class="btn small primary" data-ed-insert-here>Insert here</button><button type="button" class="btn small ghost" data-ed-insert-cancel>Cancel</button></span>';
 const anchor=overlay.querySelector('.nt-editor-options');if(anchor)anchor.after(bar);else overlay.prepend(bar);findEl('[data-ed-text]')?.focus({preventScroll:true});return true;}
function revealSelectedTabs(){
 for(const tabs of overlay?.querySelectorAll('[data-ed-tabs]')||[]){if(tabs.hidden||!tabs.clientWidth)continue;const tab=tabs.querySelector('[role="tab"][aria-selected="true"]')?.closest('.nt-editor-tab');if(!tab)continue;const area=tabs.getBoundingClientRect(),item=tab.getBoundingClientRect();if(item.left<area.left)tabs.scrollLeft+=item.left-area.left;else if(item.right>area.right)tabs.scrollLeft+=item.right-area.right;}
}
function drawTabs(){
 syncClosePreference();root.NetworkToolkitEditorWindows?.notifyChanged?.();
 groups.reconcile();for(const button of overlay?.querySelectorAll?.('[data-ed]')||[]){const action=button.dataset.ed;if(['find','replace','goto','comment','duplicate','trim','upper','lower','sortLines','format'].includes(action))button.disabled=!!comparison||!!current()?.frozen;}
 const focused=document.activeElement,focusID=focused?.dataset?.edOpen||focused?.dataset?.edCloseTab,closeFocus=!!focused?.dataset?.edCloseTab,workspace=paneWorkspace(),selection=groups.selection();
 for(const tabs of overlay?.querySelectorAll('[data-ed-tabs]')||[]){const side=tabs.dataset.edView||'single';tabs.hidden=side==='single'&&!!workspace;const list=side==='single'?(workspace?[]:[...documents.values()]):groups.list(side),chosen=side==='single'?active:selection[side];
 tabs.innerHTML=list.map((d,index)=>'<div class="nt-editor-tab" role="presentation" draggable="true" data-ed-uid="'+escaped(d.uid)+'"><button type="button" role="tab" id="nt-editor-tab-'+escaped(d.uid)+'" aria-posinset="'+(index+1)+'" aria-setsize="'+list.length+'" aria-keyshortcuts="Alt+Shift+ArrowLeft Alt+Shift+ArrowRight Shift+F10" tabindex="'+(chosen===d.id?'0':'-1')+'" data-ed-open="'+escaped(d.id)+'" aria-selected="'+(chosen===d.id)+'" title="'+escaped(d.source+' · '+d.path)+'" aria-label="'+escaped(nameOf(d.path)+(d.dirty?', unsaved changes':'')+(d.saving?', saving':''))+'">'+'<span class="nt-editor-tab-status" aria-hidden="true">'+(d.saving?'◌':d.dirty?'●':'')+'</span><span class="nt-editor-tab-name">'+escaped(nameOf(d.path))+'</span></button><button type="button" class="ux-editor-tab-actions" data-ed-tab-actions="'+escaped(d.uid)+'" title="More tab actions (Shift+F10)" aria-label="More actions for this document tab" aria-haspopup="menu">⋯</button><button type="button" class="tab-close" data-ed-close-tab="'+escaped(d.id)+'" aria-label="Close '+escaped(nameOf(d.path))+'" '+(d.saving?'disabled':'')+'>'+(typeof icon==='function'?icon('close'):'×')+'</button></div>').join('')+((side==='single'&&!workspace)||side==='left'?'<button type="button" class="nt-editor-new-tab" data-ed="new" aria-label="New document tab" title="New document in left view (Ctrl+N)">'+(typeof icon==='function'?icon('plus'):'+')+'</button>':'');
 }
 revealSelectedTabs();for(const tabs of overlay?.querySelectorAll('[data-ed-tabs]')||[])root.NetworkToolkitTabScroll?.attach(tabs);
 root.NetworkToolkitEditorFiles?.refreshDocuments?.();documentMap?.schedule();
 const compareButton=findEl('[data-ed="compare"]');if(compareButton){compareButton.setAttribute('aria-pressed',String(!!comparison));compareButton.title=comparison?'Exit comparison':'Compare documents';}
 const split=findEl('[data-ed="split"]');if(split){const label=workspace?'Single view':'Split view';split.setAttribute('aria-label',label);split.title=label;split.setAttribute('aria-pressed',String(!!workspace));}
 if(focusID){const node=[...overlay.querySelectorAll(closeFocus?'[data-ed-close-tab]':'[data-ed-open]')].find(node=>(closeFocus?node.dataset.edCloseTab:node.dataset.edOpen)===focusID);node?.focus({preventScroll:true});}
 textTools?.draw();
}

// Paint a plain escaped copy immediately. Late worker replies must match the
// same document, text, language and paint generation before touching the UI.
function paintText(d){if(d?.disk){if(current()===d)diskView()?.refreshSyntax();return;}if(paneWorkspace()){if(current()===d)syncLanguageControls(d);return;}const ta=findEl('[data-ed-text]'),pre=findEl('[data-ed-highlight]'),gutter=findEl('[data-ed-gutter-lines]');if(!ta||!pre||!gutter||current()!==d)return;
 const generation=++paintGeneration;if(paintTimer!==null){root.clearTimeout(paintTimer);paintTimer=null;}
 ta.wrap=effectiveWrap(d)?'soft':'off';ta.classList.toggle('is-wrapped',effectiveWrap(d));const wrap=findEl('[data-ed-wrap]');if(wrap){wrap.disabled=largeDocument(d);wrap.title=largeDocument(d)?'Wrap is paused in large-file mode; your saved preference is retained.':'';}paintGutter(d,ta,gutter);pre.classList.toggle('is-wrapped',wordWrap);pre.style.width=wordWrap?ta.clientWidth+'px':'';
 const language=languageOf(d),paintLimit=root.NTKNetworkConfig?.supports(language)?root.NTKNetworkConfig.MAX_PAINT:250000;const show=syntax&&d.content.length<=paintLimit;pre.hidden=!show;ta.classList.toggle('highlight-active',show);
 const snapshot=d.content;if(syntax&&!show)setMessage('Showing plain text: syntax highlighting is limited to '+paintLimit.toLocaleString()+' characters. Document contents are unchanged.');
 if(show){
  if(d.syntaxCache?.text===snapshot&&d.syntaxCache.language===language){pre.innerHTML=d.syntaxCache.html+'\n';if(d.syntaxCache.limited)setMessage('Showing plain text: '+d.syntaxCache.reason);}
  else{pre.textContent=snapshot+'\n';if(snapshot&&language!=='text'&&root.NTKEditorLanguageService){paintTimer=root.setTimeout(()=>{paintTimer=null;
   void root.NTKEditorLanguageService.highlight(snapshot,language).then(result=>{
    if(current()!==d||documents.get(d.id)!==d||d.content!==snapshot||languageOf(d)!==language||generation!==paintGeneration||!syntax)return;
    d.syntaxCache={text:snapshot,language,html:result.html,limited:!!result.limited,reason:result.reason||''};pre.innerHTML=result.html+'\n';pre.title=result.reason||'';if(result.limited)setMessage('Showing plain text: '+result.reason);
   }).catch(()=>{if(generation===paintGeneration)pre.title='Showing plain text while syntax tools are unavailable.';});
  },70);}}
 }else root.NTKEditorLanguageService?.cancelHighlight?.();
 pre.style.transform='translate('+(-ta.scrollLeft)+'px,'+(-ta.scrollTop)+'px)';gutter.style.transform='translateY('+(-ta.scrollTop)+'px)';
 syncLanguageControls(d);
}
function syncLanguageControls(d){const selected=findEl('[data-ed-language-select]'),automatic=findEl('[data-ed-auto-language]'),language=languageOf(d),button=findEl('[data-ed="format"]'),kind=languageTools?.formatKind(language)||'';
 if(selected)selected.value=d.language||'';if(automatic)automatic.textContent='Auto ('+(languageTools?.label(languageFor(d.path,d.content))||'Plain text')+')';
 if(button){button.disabled=!!d.loading||!!d.formatting||!!d.frozen||!!comparison||d.disk||!kind||d.content.length>250000;button.textContent=kind==='indent'?'Indent document':'Format document';button.title=kind==='indent'?'Adjust code indentation, preserving strings and comments (Ctrl+Shift+F)':kind?'Format '+languageTools.label(language)+' (Ctrl+Shift+F)':'Automatic formatting is unavailable for this language.';}
 if(overlay)overlay.style.setProperty('--nt-editor-tab',String(tabSpaces||4));
}
function draw(){const d=current();if(!d)return;groups.activate(d.uid);if(paneWorkspace()){drawTabs();syncEditorControls(d);paneWorkspace().refresh();return;}if(displayedDocument&&displayedDocument!==d)displayedDocument.view=captureView();const view=d.showStart?null:displayedDocument===d?captureView():d.view;
 if(d.disk){drawDiskDocument(d,view);return;}
 disposeSingleDiskView();displayedDocument=d;drawTabs();const ta=findEl('[data-ed-text]');ta.disabled=!!(d.loading||d.frozen||d.promoting);if(ta.value!==d.content)ta.value=d.content;ta.wrap=effectiveWrap(current())?'soft':'off';ta.classList.toggle('is-wrapped',effectiveWrap(current()));findEl('[data-ed-title]').textContent=d.source+' · '+d.path;formatControls(d);findEl('[data-ed-indent]').value=String(tabSpaces);findEl('[data-ed-wrap]').checked=wordWrap;findEl('[data-ed-syntax]').checked=syntax;findEl('[data-ed-language-select]').value=d.language||'';findEl('[data-ed-font]').value=String(editorFontSize);findEl('[data-ed-palette]').value=editorPalette;applyEditorTheme();findEl('[data-ed-find]').hidden=!showFind;findEl('[data-ed-replace-fields]').hidden=!showReplace;setMessage(d.loading?'Opening '+nameOf(d.path)+'...':'Ready');for(const button of overlay.querySelectorAll('[data-ed="save"],[data-ed="saveAs"],[data-ed="reload"]'))button.disabled=!!d.loading;const focus=d.showStart;d.showStart=false;paintText(d);if(view)restoreView(view,view.start,view.end,focus);else if(focus)restoreView({start:0,end:0,top:0,left:0,direction:'none'},0,0,true);status();}
async function save(as=false,d=current()){if(!d||d.loading||d.saving||d.frozen||d.promoting||d.diskEditing||editorComposing()||documents.get(d.id)!==d)return;if(notText(d)){setMessage(nameOf(d.path)+' is not a text file. It is shown read-only and cannot be saved from the Text Editor.',true);return;}as=as||d.untitled||d.source==='Desktop import';let path=d.path;
 if(as&&d.source==='Desktop'&&!d.disk){exportCopy(d);return;}
 let overwrite=false;
 if(as){const stamp=documentStamp(d),choice=await chooseSaveAsPath(d);if(choice===null||current()!==d||documents.get(d.id)!==d||documentStamp(d)!==stamp)return;path=String(choice.path||'').trim();overwrite=choice.overwrite===true;if(!path||path.length>4096||path===d.path){setMessage('Choose a different path for Save As.',true);return;}}
 if(updating){setMessage('Wait for the current save to finish.',true);return;}if(as&&[...documents.values()].some(other=>other!==d&&other.path===path)){setMessage('That destination is already open in another tab. Close or save that tab first.',true);return;}updating=true;d.saving=true;pendingSavePath=as?path:'';drawTabs();paneWorkspace()?.refresh();setMessage('Saving '+path+'...');
 const written={content:d.content,eol:d.eol,bom:d.bom,encoding:d.encoding};try{const result=d.disk?(d.desktopHandle&&!as?await saveBrowserDocument(d,formatOf(d)):await ensureDiskSession(d).save(path,formatOf(d),overwrite)):await d.write({path,content:outgoing(d),expectedHash:as?'':d.hash,bom:d.bom,encoding:d.encoding,...(as&&overwrite?{overwrite:true}:{})});if(documents.get(d.id)!==d)return;d.hash=result.hash||d.hash;d.saved=written.content;d.savedEol=written.eol;d.savedBOM=written.bom;d.savedEncoding=written.encoding;d.untitled=false;d.saveFailed=false;d.dirty=(d.disk?!!d.diskSession.state().metadata.dirty:d.content!==d.saved)||formatDirty(d);
  if(as){const wasActive=current()===d,oldId=d.id;documents.delete(oldId);if(d.source==='Desktop import'||d.source==='Untitled'||d.disk&&d.source==='Desktop'){d.source='Local computer';d.desktopHandle=d.diskFile=null;if(!d.disk)d.binding={kind:'local'};d.read=p=>api('/file-editor/local','POST',{action:'read-text',path:p});d.write=data=>api('/file-editor/local','POST',{action:'write-text',...data});}d.path=result.path||path;d.id=d.source==='Local computer'?'local:'+d.path:d.source+':'+d.path;documents.set(d.id,d);if(!d.disk)root.NetworkToolkitEditorWindows?.prepareDocument?.(d);if(wasActive)active=d.id;}
  paintText(d);drawTabs();status();if(current()===d)setMessage(d.dirty?'Saved earlier contents. Newer edits remain unsaved.':'Saved '+(globalThis.NetworkToolkitResultTime?.format(Date.now(),{date:false,milliseconds:false})||new Date().toLocaleTimeString(undefined,{hour12:true})));if(typeof d.afterSave==='function')void d.afterSave();}
 catch(e){d.saveFailed=true;if(current()===d)setMessage('Save failed: '+e.message,true);}finally{updating=false;pendingSavePath='';d.saving=false;drawTabs();paneWorkspace()?.refresh();}
}
function rememberClosedDocument(d){
 if(!d||d.dirty||d.saveFailed||d.loading||d.untitled||d.source!=='Local computer'||!['local','localDraft'].includes(d.binding?.kind))return false;
 return root.NetworkToolkitClosedTabs?.recordEditor?.(d.path)===true;
}
async function closeTab(key){const d=documents.get(key);if(!d)return false;if(pendingEditorDialog||documentCloseBlocked(d)||editorComposing()){setMessage('Finish text composition, document saving or the window move before closing it.',true);return false;}const stamp=documentStamp(d);if(closeNeedsConfirmation([d])&&await askEditor({title:'Close file',message:(d.saveFailed?'A previous save failed. ':'')+'Discard unsaved changes to '+nameOf(d.path)+'?',accept:'Discard and close'})===null)return false;if(editorComposing()||documentCloseBlocked(d)||documents.get(key)!==d||documentStamp(d)!==stamp){setMessage('The file changed while the dialog was open. Review it before closing.',true);return false;}const wasActive=active===key;if(d.disk)discardDiskView(d);if(wasActive)displayedDocument=null;rememberClosedDocument(d);documents.delete(key);paneWorkspace()?.refresh(true);if(wasActive)active=[...documents.keys()].at(-1)||'';if(active){if(wasActive)draw();else drawTabs();}else{if(paneWorkspace())disposePaneWorkspace();splitEnabled=false;groups.single();findEl('[data-ed-text]').value='';findEl('[data-ed-text]').disabled=false;findEl('[data-ed-highlight]').textContent='';findEl('[data-ed-gutter-lines]').textContent='';drawTabs();overlay.hidden=!embedded;setMessage('No open documents.');paintGeneration++;root.NTKEditorLanguageService?.cancelHighlight?.();}return true;}
async function closeAll(){if(pendingEditorDialog||editorComposing()||[...documents.values()].some(d=>documentCloseBlocked(d))){setMessage('Finish text composition, pending saves or window moves before closing the editor.',true);return false;}const snapshot=[...documents.values()].map(d=>({d,stamp:documentStamp(d)})),dirty=snapshot.filter(({d})=>d.dirty);if(closeNeedsConfirmation(snapshot.map(x=>x.d))&&await askEditor({title:'Close editor',message:(snapshot.some(x=>x.d.saveFailed)?'A previous save failed. ':'')+'Discard unsaved changes in '+dirty.length+' file(s)?',accept:'Discard and close'})===null)return false;if(editorComposing()||snapshot.length!==documents.size||snapshot.some(({d,stamp})=>documents.get(d.id)!==d||documentStamp(d)!==stamp||documentCloseBlocked(d))){setMessage('An open file changed. Review it before closing.',true);return false;}for(const {d} of snapshot){rememberClosedDocument(d);if(d.disk)discardDiskView(d);}if(paneWorkspace())disposePaneWorkspace();splitEnabled=false;documents.clear();root.NetworkToolkitEditorWindows?.notifyChanged?.();active='';displayedDocument=null;if(embedded)detachPage();overlay.hidden=true;paintGeneration++;if(paintTimer!==null){root.clearTimeout(paintTimer);paintTimer=null;}root.NTKEditorLanguageService?.dispose?.();return true;}
// Search modes of the Find bar. Extended escapes and the Replace template of
// a regular expression are read by editor-text-tools.js, the template with
// the engine's rules (internal/editorbuffer/template.go).
const searchModeHelp={normal:'Normal: finds the text exactly as typed.',extended:'Extended: like Normal, but \\n, \\r, \\t, \\0, \\xNN and \\uNNNN in Find and Replace stand for a line break, a tab and other characters. Find & and replace with \\n to start a new line at every &.',regex:'Regular expression: Find is a regular expression. Replace can use $1, ${name} and $& for the text of groups, and \\n and \\t for a line break and a tab.'};

function replaceSelection(value,select=false){const paged=diskView();if(paged){const start=paged.selection().start;return paged.replace(value).then(()=>{if(select)paged.select(start,start+value.length,'none',true);}).catch(error=>setMessage(error.message,true));}const el=findEl('[data-ed-text]'),d=current();if(!el||!d||d.loading)return;const a=el.selectionStart,b=el.selectionEnd,next=d.content.slice(0,a)+value+d.content.slice(b);applyEdit(d,next,select?a:a+value.length,a+value.length);}
function search(backwards=false,replace=false,all=false,focus=true){const d=current(),ta=findEl('[data-ed-text]'),tools=root.NTKEditorTextTools,regex=searchMode==='regex';let query=findEl('[data-ed-query]')?.value||'',replacement=findEl('[data-ed-replacement]')?.value||'';if(!d||!ta||!query)return;
 // Extended: escapes in Find and Replace are the characters they name.
 if(searchMode==='extended'&&tools){const q=tools.decodeExtended(query),r=tools.decodeExtended(replacement);if(q.error||r.error){setMessage(q.error||r.error,true);return;}query=q.text;replacement=r.text;if(!query)return;}
 if(d.disk){void searchDiskDocument(d,query,backwards,replace,all,focus,replacement);return;}
 let pattern;try{const s=regex?query:query.replace(/[.*+?^${}()|[\]\\]/g,'\\$&');pattern=new RegExp(wholeWord?'\\b(?:'+s+')\\b':s,matchCase?'g':'gi');}catch(e){setMessage('Invalid search: '+e.message,true);return;}
 // Regular expression: the Replace text is a template ($1, ${name}, \n), expanded as the engine does.
 const expand=match=>regex&&tools?tools.expandReplacement(replacement,match,pattern.source):replacement;
 if(all){if(!replace)return;let count=0,next;try{next=d.content.replace(pattern,(...args)=>{count++;return expand(args.slice(0,typeof args.at(-1)==='object'?args.length-3:args.length-2));});}catch(e){setMessage(e.message,true);return;}if(next===d.content){setMessage(count?'Replaced '+count+' occurrence(s); the text did not change.':'No matches.');return;}const view=captureView();applyEdit(d,next,view.start,view.end,view);setMessage('Replaced '+count+' occurrence(s).');return;}
 const results=[...d.content.matchAll(pattern)].filter(m=>m[0].length>0);if(!results.length){setMessage('No matches.');return;}
 const at=ta.selectionStart;const match=backwards?([...results].reverse().find(m=>m.index<at)||results.at(-1)):(results.find(m=>m.index>=Math.max(at,ta.selectionEnd>at?ta.selectionEnd:at+1))||results[0]);
 if(replace){const chosen=results.find(m=>m.index===ta.selectionStart&&m.index+m[0].length===ta.selectionEnd);if(chosen){let text;try{text=expand(chosen);}catch(e){setMessage(e.message,true);return;}replaceSelection(text);return;}}
 positionAt(match.index,focus);const view=captureView();restoreView(view,match.index,match.index+match[0].length,focus);status();setMessage(results.length+' match(es).');
}
function linesAction(kind){const d=current(),ta=findEl('[data-ed-text]');if(!d||!ta||d.loading)return;if(d.disk){void transformDiskSelection(d,kind);return;}const a=ta.selectionStart,b=ta.selectionEnd,start=a===0?0:d.content.lastIndexOf('\n',a-1)+1,limit=b>a&&d.content[b-1]==='\n'?b-1:b,last=d.content.indexOf('\n',limit),end=last<0?d.content.length:last,part=d.content.slice(start,end),lines=part.split('\n');
 let next=part;if(kind==='comment'){next=languageTools?.toggleComment(part,languageOf(d));if(next===null||next===undefined){setMessage('This language does not define comment syntax.');return;}}if(kind==='duplicate')next=part+'\n'+part;if(kind==='trim')next=lines.map(line=>line.replace(/[ \t]+$/,'')).join('\n');
 applyEdit(d,d.content.slice(0,start)+next+d.content.slice(end),start,start+next.length);}
function indentSelection(unindent=false){const d=current(),ta=findEl('[data-ed-text]');if(!d||!ta||d.loading)return;
 if(d.disk){const range=diskView()?.selection();if(!range)return;if(!unindent&&range.start===range.end)replaceSelection(tabSpaces?' '.repeat(tabSpaces):'\t');else void transformDiskSelection(d,unindent?'unindent':'indent');return;}const a=ta.selectionStart,b=ta.selectionEnd;
 if(a===b&&!unindent){replaceSelection(tabSpaces?' '.repeat(tabSpaces):'\t');return;}
 const start=a===0?0:d.content.lastIndexOf('\n',a-1)+1,limit=b>a&&d.content[b-1]==='\n'?b-1:b,last=d.content.indexOf('\n',limit),end=last<0?d.content.length:last,part=d.content.slice(start,end),unit=tabSpaces?' '.repeat(tabSpaces):'\t';
 const transformed=part.split('\n').map(line=>unindent?line.replace(new RegExp('^(?:\\t| {1,'+(tabSpaces||4)+'})'),''):unit+line).join('\n');
 applyEdit(d,d.content.slice(0,start)+transformed+d.content.slice(end),start,start+transformed.length);
}
function exportCopy(d){if(!d)return;if(d.disk){void exportDiskCopy(d);return;}let encoded;try{encoded=encodeDesktopText({content:outgoing(d),encoding:d.encoding,bom:d.bom});}catch(error){setMessage(error.message,true);return;}const blob=new Blob([encoded],{type:'application/octet-stream'});const url=URL.createObjectURL(blob),a=document.createElement('a');a.href=url;a.download=nameOf(d.path);a.click();setTimeout(()=>URL.revokeObjectURL(url),3000);setMessage('Saved a copy to your browser downloads.');}
function click(ev){if(ev.target.closest?.('[data-ed-insert-cancel]')){ev.preventDefault();endPick();setMessage('Insert cancelled. Nothing was added.');return;}if(ev.target.closest?.('[data-ed-insert-here]')){ev.preventDefault();const p=pickState,d=p&&documents.get(p.id);if(!d){endPick();setMessage('That document is no longer open. Nothing was added.',true);return;}if(current()!==d){setMessage('Switch back to '+nameOf(d.path)+' to insert the text there, or choose Cancel.',true);return;}void Promise.resolve(insertText(p.id,p.text,'cursor')).then(r=>{if(r.ok)endPick();else setMessage(r.error,true);});return;}const dialogAction=ev.target.closest('[data-ed-dialog-action]');if(dialogAction){ev.preventDefault();answerEditorDialog(dialogAction.dataset.edDialogAction==='accept');return;}if(pendingEditorDialog)return;const compareTarget=ev.target.closest('[data-ed-compare-target]');if(compareTarget){ev.preventDefault();compareTabTarget(compareTarget.dataset.edCompareTarget);return;}if(ev.target.closest('[data-ed-compare-cancel]')){ev.preventDefault();closeTabMenu();return;}const menuAction=ev.target.closest('[data-ed-context-action]');if(menuAction){ev.preventDefault();void runTabMenu(menuAction.dataset.edContextAction);return;}const tab=ev.target.closest('[data-ed-open]');if(tab){activateDocument(tab.dataset.edOpen);return;}const closing=ev.target.closest('[data-ed-close-tab]');if(closing){closeTab(closing.dataset.edCloseTab);return;}
 const el=ev.target.closest('[data-ed]');if(!el)return;ev.preventDefault();if(!current()&&!['new','browse','close','colors','localTools','compare','split','files','openLocalPath','newLocalPath'].includes(el.dataset.ed)){setMessage('Open or create a document first.');return;}switch(el.dataset.ed){
 case 'compare':if(comparison)closeComparison();else openComparison();break;
 case 'openLocalPath':case 'newLocalPath':void askLocalPath(el.dataset.ed==='newLocalPath');break;
 case 'split':openSplit();break;case 'files':root.NetworkToolkitEditorFiles?.toggle?.(overlay);break;
 case 'colors':{const panel=findEl('[data-ed-custom-palette]');if(panel)panel.hidden=!panel.hidden;break;}
 case 'sendTerminal':sendToTerminal();break;case 'close':{if(typeof EDITOR_STANDALONE!=='undefined'&&EDITOR_STANDALONE){void root.__networkToolkitClose?.();break;}const docked=embedded;void closeAll().then(closed=>{if(closed&&docked&&typeof navigate==='function')navigate('local');});break;}case 'new':void newDocument();break;case 'browse':void openDesktopFile();break;case 'localTools':if(typeof navigate==='function'){navigate('editor');}break;case 'save':void save();break;case 'saveAs':void save(true);break;case 'reload':void reload();break;
 case 'undo':if(paneWorkspace())paneWorkspace().undo();else doUndo(current());break;case 'redo':if(paneWorkspace())paneWorkspace().undo(true);else doUndo(current(),true);break;
 case 'find':showFind=true;showReplace=false;draw();findEl('[data-ed-query]').focus();break;
 case 'replace':showFind=true;showReplace=true;draw();findEl('[data-ed-query]').focus();break;
 case 'hideFind':showFind=false;showReplace=false;draw();break;
 case 'next':search();break;case 'previous':search(true);break;case 'replaceOne':search(false,true);break;case 'replaceAll':search(false,true,true);break;
 case 'tools':root.NetworkToolkitEditorContextMenu?.openTools?.(el);break;
 case 'format':void formatDocument();break;case 'comment':linesAction('comment');break;case 'duplicate':linesAction('duplicate');break;
 default:if(root.NTKEditorTextTools?.tool(el.dataset.ed))void textTools?.run(el.dataset.ed);
 case 'goto':void goToLine();break;
 case 'download':{exportCopy(current());break;}
 }}
function beforeInput(ev){pendingLargeInput=null;if(!ev.target.matches?.('[data-ed-text]')||ev.isComposing||!largeDocument(current()))return;const d=current(),ta=ev.target;pendingLargeInput=root.NTKEditorLargeIndex?.plan?.(d.content,d.editRevision,ta.selectionStart,ta.selectionEnd,ev.inputType,ev.data)||null;}
function input(ev){if(ev.target.matches('[data-ed-text]')&&current()?.disk)return;if(ev.target.matches('[data-ed-text]')){const text=ev.target.value,view=captureView();let d=current();if(!d){void newDocument().catch(error=>setMessage(error.message,true));d=current();}const edit=pendingLargeInput;pendingLargeInput=null;if(d){setContent(d,text,true,edit,view?.start);if(ev.target.value!==d.content)ev.target.value=d.content;restoreView(view,view?.start,view?.end);}}else if(ev.target.matches('[data-ed-query]')&&ev.target.value){search(false,false,false,false);}}
function change(ev){const t=ev.target;if(t.matches?.('[data-ed-ask-save-close]')){void changeClosePreference(t.checked);return;}if(t.matches?.('[data-ed-restore-session]')){void changeRestorePreference(t.checked);return;}if(t.matches?.('[data-ed-color]')){setEditorColor(t.dataset.edColor,t.value);return;}if(t.matches?.('[data-ed-color-hex]')){if(goodEditorColor(t.value.trim())){t.setCustomValidity('');setEditorColor(t.dataset.edColorHex,t.value.trim());}else{t.setCustomValidity('Use #RRGGBB');t.reportValidity();}return;}if(t.matches('[data-ed-palette]')){editorPalette=editorPalettes.includes(t.value)?t.value:'auto';applyEditorTheme();keepPreferences();if(editorPalette==='custom')findEl('[data-ed-custom-palette]').hidden=false;return;}if(t.matches('[data-ed-wrap]')){wordWrap=t.checked;if(paneWorkspace())paneWorkspace().refresh(true);else if(singleDiskView)singleDiskView.refreshWrap();else draw();keepPreferences();return;}if(t.matches('[data-ed-syntax]')){syntax=t.checked;if(paneWorkspace())paneWorkspace().refresh(true);else if(current())paintText(current());keepPreferences();return;}const d=current();if(!d||d.frozen)return;if(notText(d)){formatControls(d);return;}if(t.matches('[data-ed-eol]')){d.eol=t.value;d.dirty=(d.disk?!!d.diskMeta?.dirty:d.content!==d.saved)||formatDirty(d);drawTabs();status();}if(t.matches('[data-ed-bom]')){d.bom=t.checked;d.dirty=(d.disk?!!d.diskMeta?.dirty:d.content!==d.saved)||formatDirty(d);drawTabs();status();}if(t.matches('[data-ed-encoding]')){d.encoding=t.value;d.bom=t.value==='Windows-1252'?false:t.value!=='UTF-8'?true:d.bom;d.dirty=true;drawTabs();formatControls(d);}if(t.matches('[data-ed-language-select]')){d.language=t.value?languageTools?.valid(t.value)||'text':'';paintText(d);paneWorkspace()?.refresh(true);}if(t.matches('[data-ed-font]'))setEditorFontSize(t.value);if(t.matches('[data-ed-indent]')){tabSpaces=Number(t.value);syncLanguageControls(d);if(paneWorkspace())paneWorkspace().refresh(true);else if(singleDiskView)singleDiskView.refreshWrap();else paintText(d);keepPreferences();}if(t.matches('[data-ed-case]'))matchCase=t.checked;if(t.matches('[data-ed-search-mode]')){searchMode=['normal','extended','regex'].includes(t.value)?t.value:'normal';t.closest('label')?.setAttribute('title',searchModeHelp[searchMode]);}if(t.matches('[data-ed-word]'))wholeWord=t.checked;}
function scroll(ev){if(current()?.disk)return;if(paneWorkspace()&&ev.target===paneWorkspace().textarea){status();return;}if(!ev.target.matches?.('[data-ed-text]'))return;const ta=ev.target,pre=findEl('[data-ed-highlight]'),gutter=findEl('[data-ed-gutter-lines]');if(pre)pre.style.transform='translate('+(-ta.scrollLeft)+'px,'+(-ta.scrollTop)+'px)';if(gutter)gutter.style.transform='translateY('+(-ta.scrollTop)+'px)';if(!largeDocument(current())){status();return;}if(typeof root.requestAnimationFrame!=='function'){paintGutter(current(),ta,gutter);status();return;}if(largeScrollFrame!==null)return;const d=current();largeScrollFrame=root.requestAnimationFrame(()=>{largeScrollFrame=null;if(current()!==d||findEl('[data-ed-text]')!==ta||!gutter)return;paintGutter(d,ta,gutter);status();});}
function keydown(ev){if(ev.target.closest?.('[data-ed-files-sidebar]'))return;if(tabMenu){tabMenuKey(ev);return;}const menuTab=ev.target.closest?.('[data-ed-uid]');if(menuTab&&(ev.key==='ContextMenu'||(ev.shiftKey&&ev.key==='F10'))){ev.preventDefault();const d=[...documents.values()].find(d=>d.uid===menuTab.dataset.edUid),rect=menuTab.getBoundingClientRect();if(d)openTabMenu(d,rect.left,rect.bottom);return;}if(pendingEditorDialog){ev.stopPropagation();if(ev.key==='Escape'){ev.preventDefault();answerEditorDialog(false);}else if(ev.key==='Enter'){ev.preventDefault();answerEditorDialog(ev.target.closest?.('[data-ed-dialog-action]')?.dataset.edDialogAction!=='cancel');}else if(ev.key==='Tab'){const controls=[...pendingEditorDialog.panel.querySelectorAll('input,button')],at=controls.indexOf(document.activeElement);ev.preventDefault();controls[(at+(ev.shiftKey?-1:1)+controls.length)%controls.length]?.focus();}return;}const d=current(),ta=findEl('[data-ed-text]'),ctrl=ev.ctrlKey||ev.metaKey;const tab=ev.target.closest?.('[data-ed-open]');if(tab&&ev.altKey&&ev.shiftKey&&['ArrowLeft','ArrowRight'].includes(ev.key)){ev.preventDefault();ev.stopPropagation();const selectedDoc=documents.get(tab.dataset.edOpen),list=paneWorkspace()?groups.list(groups.sideOf(selectedDoc?.uid)):[...documents.values()],at=list.findIndex(doc=>doc.id===tab.dataset.edOpen);if(at>=0&&((ev.key==='ArrowLeft'&&at>0)||(ev.key==='ArrowRight'&&at<list.length-1)))reorderDocument(list[at].uid,ev.key==='ArrowLeft'?list[at-1].uid:list[at+2]?.uid||null);return;}if((tab&&['ArrowLeft','ArrowRight','Home','End'].includes(ev.key))||(ctrl&&['PageUp','PageDown'].includes(ev.key))){const tabDoc=documents.get(tab?.dataset.edOpen||active),ids=(paneWorkspace()?groups.list(groups.sideOf(tabDoc?.uid)):[...documents.values()]).map(d=>d.id),at=ids.indexOf(tab?.dataset.edOpen||active),next=ev.key==='Home'?0:ev.key==='End'?ids.length-1:(at+(ev.key==='ArrowLeft'||ev.key==='PageUp'?-1:1)+ids.length)%ids.length;ev.preventDefault();activateDocument(ids[next]);[...overlay.querySelectorAll('[data-ed-open]')].find(node=>node.dataset.edOpen===ids[next])?.focus({preventScroll:true});return;}if(comparison&&!(ctrl&&['n','o'].includes(ev.key.toLowerCase()))){if(ctrl&&ev.key.toLowerCase()==='s'){ev.preventDefault();void save(ev.shiftKey);}return;}if(ctrl&&['n','o'].includes(ev.key.toLowerCase())){ev.preventDefault();if(ev.key.toLowerCase()==='n')void newDocument();else void openDesktopFile();return;}if(!d)return;
 if(ctrl&&ev.shiftKey&&ev.key==='Enter'&&!ev.isComposing){ev.preventDefault();sendToTerminal();return;}
 if(ctrl){const key=ev.key.toLowerCase(),toolKey=ev.altKey?'':ev.shiftKey?{u:'upper',l:'split',b:'base64-decode'}[key]:{u:'lower',j:'join'}[key];if(toolKey&&textTools?.full()){ev.preventDefault();void textTools.run(toolKey);return;}if(key==='f'&&ev.shiftKey){ev.preventDefault();void formatDocument();return;}if(key==='g'){ev.preventDefault();void goToLine();return;}if(key==='o'){ev.preventDefault();void openDesktopFile();return;}if(key==='n'){ev.preventDefault();void newDocument();return;}if(key==='s'){ev.preventDefault();void save(ev.shiftKey);return;}if(key==='f'){ev.preventDefault();showFind=true;showReplace=false;draw();findEl('[data-ed-query]').focus();return;}if(key==='h'){ev.preventDefault();showFind=true;showReplace=true;draw();findEl('[data-ed-query]').focus();return;}if(key==='z'){ev.preventDefault();doUndo(d,ev.shiftKey);return;}if(key==='y'){ev.preventDefault();doUndo(d,true);return;}if(key==='d'){ev.preventDefault();linesAction('duplicate');return;}if(key==='/'){ev.preventDefault();linesAction('comment');return;}}
 if(ev.key==='Escape'&&ev.target!==ta){showFind=false;showReplace=false;draw();ta.focus();return;}
 if(ev.target!==ta)return;
 if(d.disk&&ev.key==='Enter')return;
 if(ev.key==='Tab'){ev.preventDefault();indentSelection(ev.shiftKey);return;}
 if(ev.key==='Enter'&&!ev.isComposing){const insert=languageTools?.newline(d.content,ta.selectionStart,languageOf(d),tabSpaces?' '.repeat(tabSpaces):'\t');if(insert){ev.preventDefault();replaceSelection(insert.text);if(insert.back){const at=ta.selectionStart-insert.back;ta.setSelectionRange(at,at);}status();}return;}
 if(ev.key==='Home'&&ev.ctrlKey===false){status();}
}
async function reload(){
 const d=current();
 if(!d||d.loading||d.frozen||d.saving||d.reloading||d.promoting||editorComposing())return;
 const stamp=documentStamp(d);
 if(d.dirty&&await askEditor({title:'Reload file',message:'Discard unsaved changes and reload '+nameOf(d.path)+'?',accept:'Discard and reload'})===null)return;
 if(documents.get(d.id)!==d||documentStamp(d)!==stamp)return;
 d.reloading=true;
 const view=d.disk?diskView():null;
 if(view)view.setReadOnly(true);
 let failure=null;
 try{
  if(!d.disk){
   let result;
   try{result=await d.read(d.path);}
   catch(error){if(!canPromote(d)||!/editor limit|16 MiB|32 MiB/i.test(error.message))throw error;}
   if(documents.get(d.id)!==d||documentStamp(d)!==stamp)throw Error('The file changed while reloading. Reload again after reviewing your edits.');
   if(result&&(!canPromote(d)||!shouldPage(result.content||''))){
    d.original=result.content||'';d.hash=result.hash||'';d.bom=!!result.bom;d.encoding=result.encoding||'UTF-8';
    setFileValues(d);
    return;
   }
   if(current()===d)findEl('[data-ed-text]').disabled=true;
   await adoptDiskDocument(d,documentTransferValue(d));
   updatePaneFactory();
  }
  const result=await ensureDiskSession(d).reload(undefined,d.desktopHandle||d.diskFile?()=>reloadBrowserDocument(d):undefined);
  applyDiskFormat(d,result);
  d.dirty=false;d.saveFailed=false;d.showStart=true;
  if(current()===d)diskView()?.refresh();
 }catch(error){failure=error;}
 finally{
  d.reloading=false;
  if(current()===d){diskView()?.setReadOnly(!!comparison);draw();if(failure)setMessage(failure.message,true);else if(d.disk)setMessage(notText(d)?openedNote(d):'Reloaded '+nameOf(d.path)+'.');}
 }
}
function applyEditorTheme(){if(!overlay)return;overlay.dataset.palette=editorPalette;overlay.style.setProperty('--nt-editor-font',editorFontSize+'px');for(const [key,hex] of Object.entries({...editorColorDefaults,...editorCustomColors})){if(editorPalette==='custom'&&goodEditorColor(hex))overlay.style.setProperty('--nt-code-'+key,hex);else overlay.style.removeProperty('--nt-code-'+key);} }
function setEditorColor(key,value){if(!Object.hasOwn(editorColorLabels,key)||!goodEditorColor(value))return;editorCustomColors[key]=value.toLowerCase();editorPalette='custom';const selector=findEl('[data-ed-palette]');if(selector)selector.value='custom';for(const element of overlay.querySelectorAll('[data-ed-color="'+key+'"],[data-ed-color-hex="'+key+'"]'))element.value=editorCustomColors[key];applyEditorTheme();keepPreferences();}

async function formatDocument(){const d=current(),ta=findEl('[data-ed-text]');if(!d||!ta||d.loading||d.reloading||d.formatting||d.frozen||d.disk)return;const language=languageOf(d),kind=languageTools?.formatKind(language);if(!kind){setMessage('Automatic formatting is unavailable for this language.');return;}
 const snapshot=d.content,revision=d.editRevision,selection={start:ta.selectionStart,end:ta.selectionEnd,top:ta.scrollTop,left:ta.scrollLeft};d.formatting=true;syncLanguageControls(d);setMessage(kind==='indent'?'Indenting code...':'Formatting document...');
 try{const result=await root.NTKEditorLanguageService.format(snapshot,language,{indent:tabSpaces});
  if(documents.get(d.id)!==d||current()!==d||d.content!==snapshot||d.editRevision!==revision||languageOf(d)!==language){if(current()===d)setMessage('Document changed while formatting. Run the formatter again.');return;}
  setContent(d,result.text);ta.value=d.content;ta.setSelectionRange(Math.min(selection.start,d.content.length),Math.min(selection.end,d.content.length));ta.focus({preventScroll:true});ta.scrollTop=selection.top;ta.scrollLeft=selection.left;scroll({target:ta});status();setMessage(result.mode==='indent'?'Code indentation updated.':'Document formatted.');
 }catch(error){if(current()===d)setMessage('Cannot format: '+error.message,true);}
 finally{d.formatting=false;if(current()===d)syncLanguageControls(d);}
}
// A desktop File is deliberately not assumed to expose an absolute path.
// Writable File System Access handles are used when the user explicitly chooses a file.
async function readDesktopBuffer(file){if(file.size>EDITOR_FILE_BYTES)throw Error('The text editor supports encoded files up to 16 MiB.');const bytes=new Uint8Array(await file.arrayBuffer());let encoding='UTF-8',bom=false,offset=0;
 if(bytes.length>=2&&((bytes[0]===255&&bytes[1]===254)||(bytes[0]===254&&bytes[1]===255))){encoding=bytes[0]===255?'UTF-16LE':'UTF-16BE';offset=2;bom=true;}
 else if(bytes.length>=3&&bytes[0]===239&&bytes[1]===187&&bytes[2]===191){offset=3;bom=true;}
 const text=new TextDecoder(encoding.toLowerCase(),{fatal:true}).decode(bytes.subarray(offset));if(text.includes('\0'))throw Error('Binary file cannot be edited safely.');
 const hash=await crypto.subtle.digest('SHA-256',bytes);return {content:text,encoding,bom,hash:[...new Uint8Array(hash)].map(x=>x.toString(16).padStart(2,'0')).join('')};
}
function encodeDesktopText(data){const e=data.encoding||'UTF-8',text=String(data.content),limit=()=>{throw Error('The text editor supports encoded files up to 16 MiB. Existing file is unchanged.');};if(text.includes('\0'))throw Error('NUL bytes cannot be written.');if(e==='UTF-8'){const body=new TextEncoder().encode(text),size=body.length+(data.bom?3:0);if(size>EDITOR_FILE_BYTES)limit();if(!data.bom)return body;const bytes=new Uint8Array(size);bytes.set([239,187,191]);bytes.set(body,3);return bytes;}
 const little=e==='UTF-16LE';if(!little&&e!=='UTF-16BE')throw Error('Unsupported encoding');const size=text.length*2+(data.bom?2:0);if(size>EDITOR_FILE_BYTES)limit();const bytes=new Uint8Array(size),view=new DataView(bytes.buffer);let i=0;if(data.bom){bytes[0]=little?255:254;bytes[1]=little?254:255;i=2;}for(const ch of text){const n=ch.codePointAt(0);if(n<65536){view.setUint16(i,n,little);i+=2;}else{const a=n-65536;view.setUint16(i,0xd800+(a>>10),little);i+=2;view.setUint16(i,0xdc00+(a&1023),little);i+=2;}}
 return bytes;
}
async function importFiles(files,handles=[]){for(let i=0;i<Math.min(files.length,12);i++){const file=files[i],handle=handles[i]||null;try{
 const key='desktop:'+Date.now()+':'+i+':'+file.name;
 if(file.size>memoryEditorUnits&&diskAvailable()){
  await open({id:key,path:file.name,source:handle?'Desktop':'Desktop import',binding:{kind:'localDraft'},create:true,diskMode:'upload',diskFile:file,desktopHandle:handle,read:async()=>{throw Error('Use the imported document to save a local copy.');},write:async()=>{throw Error('Use Save As for this imported file.');}});
  setMessage(handle?'Opened writable desktop file.':'Opened '+file.name+'. Use Save As to choose a destination.');
  continue;
 }
 const snapshot=await readDesktopBuffer(file);let lastHash=snapshot.hash;
 const imported=await open({id:key,path:file.name,source:handle?'Desktop':'Desktop import',binding:{kind:handle&&!diskAvailable()?'desktopHandle':'localDraft'},desktopHandle:handle,create:true,initialContent:snapshot.content,
 read:async()=>{const snapshotNext=await readDesktopBuffer(handle?await handle.getFile():file);lastHash=snapshotNext.hash;return snapshotNext;},
 write:async data=>{if(!handle){if(!/^(?:[a-z]:[\\/]|\\\\|\/)/i.test(data.path))throw Error('Save As requires a full local path. Use Save copy to export a downloaded copy.');return api('/file-editor/local','POST',{action:'write-text',...data});}
  const currentFile=await readDesktopBuffer(await handle.getFile());if(data.expectedHash!==lastHash||currentFile.hash!==lastHash)throw Error('Desktop file changed outside the editor. Reload before saving.');const content=encodeDesktopText(data);const writer=await handle.createWritable();try{await writer.write(content);await writer.close();}catch(e){await writer.abort().catch(()=>{});throw e;}const updated=await readDesktopBuffer(await handle.getFile());lastHash=updated.hash;return {hash:lastHash};}});
 const d=imported;if(!d||documents.get(d.id)!==d)continue;d.hash=snapshot.hash;d.bom=snapshot.bom;d.encoding=snapshot.encoding;d.savedEncoding=d.encoding;d.savedBOM=d.bom;if(current()===d)draw();paneWorkspace()?.refresh();setMessage(handle?'Opened writable desktop file.':'Imported desktop file. Use Save copy to export changes.');
 }catch(e){if(typeof toast==='function')toast(file.name+': '+e.message,'error');else setMessage(e.message,true);}}}
async function openDesktopFile(){if(typeof root.showOpenFilePicker==='function'){try{const handles=await root.showOpenFilePicker({multiple:true,excludeAcceptAllOption:false});await importFiles(await Promise.all(handles.map(h=>h.getFile())),handles);return;}catch(e){if(e.name==='AbortError')return;setMessage('File picker: '+e.message,true);}}
 const input=document.createElement('input');input.type='file';input.multiple=true;input.addEventListener('change',()=>void importFiles([...input.files]),{once:true});input.click();}
async function newDocument(){const key='untitled:'+Date.now()+':'+Math.random().toString(36).slice(2,7);const doc=await open({id:key,source:'Untitled',binding:{kind:'localDraft'},path:'Untitled.txt',create:true,initialContent:'',read:async()=>({content:''}),write:data=>api('/file-editor/local','POST',{action:'write-text',...data})});setMessage('New file. Save asks for an absolute local path.');return doc;}
async function openLocalPath(path,create=false,exact=false){const input=String(path||''),text=exact?input:input.trim();if(!text)throw Error('Enter a full path to a local text file.');return open({id:'local:'+text,source:'Local computer',binding:{kind:'local'},path:text,create,initialContent:'',read:p=>api('/file-editor/local','POST',{action:'read-text',path:p}),write:data=>api('/file-editor/local','POST',{action:'write-text',...data})});}
async function askLocalPath(create=false){try{const path=await askEditor({title:create?'New local file at path':'Open local file by path',message:'Enter the full local file path. Nothing is written until you explicitly save.',value:localPathDraft,accept:create?'New document':'Open',validate:value=>value.trim()?'':'Enter a full local file path.'});if(path===null)return;localPathDraft=path;await openLocalPath(path,create);}catch(error){setMessage(error.message,true);if(typeof toast==='function')toast(error.message,'error');}}
function detachPage(){if(!overlay||!embedded)return;root.NetworkToolkitEditorWindows?.dismissMenu?.(false);if(displayedDocument&&!paneWorkspace())displayedDocument.view=captureView();displayedDocument=null;embedded=false;overlay.classList.remove('nt-editor-embedded');overlay.removeAttribute('role');document.body.appendChild(overlay);overlay.hidden=true;}
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
 let saved={profiles:[],detached:false,requiresApproval:false};try{if(root.NetworkToolkitEditorSavedConnections?.list)saved=await root.NetworkToolkitEditorSavedConnections.list();}catch(error){failure=[failure,'Saved connections could not be listed: '+error.message].filter(Boolean).join(' ');}
 if(generation!==remoteSourceGeneration||picker!==document.getElementById('ntEditorRemoteSource'))return;
 savedConnectionState=saved;
 const wanted=selectedRemoteSource||picker.value||'';
 const profiles=Array.isArray(saved.profiles)?saved.profiles:[],savedOptions=profiles.map(row=>({id:'saved:'+row.id,label:row.name+' — '+row.protocol.toUpperCase()+' '+row.host+':'+row.port}));
 picker.innerHTML='<option value="">Select an active or saved connection</option><optgroup label="Connected sessions">'+options.map(row=>'<option value="'+escaped(row.id)+'">'+escaped(row.label)+'</option>').join('')+'</optgroup>'+(savedOptions.length?'<optgroup label="Saved connections — connect explicitly">'+savedOptions.map(row=>'<option value="'+escaped(row.id)+'">'+escaped(row.label)+'</option>').join('')+'</optgroup>':'');
 const active=[...options,...savedOptions].some(row=>row.id===wanted)?wanted:'';
 picker.value=active;
 selectedRemoteSource=active;
 const path=document.getElementById('ntEditorRemotePath');
 if(path&&preferredRemotePath){path.value=preferredRemotePath;preferredRemotePath='';}
 updateRemoteActions();
 const hint=document.getElementById('ntEditorRemoteHint');if(hint)hint.textContent=failure||(saved.requiresApproval?(savedOptions.length?'Select a saved SSH connection, then Request connection. The main workspace must review it; credentials stay there.':'Open Files and request explorer access from the main workspace before listing saved connections.'):!options.length?(savedOptions.length?'Select a saved connection, then use Connect or Request connection. Listing and selection do not connect.':root.NetworkToolkitStandalone?.mode?.()==='editor'?'No active remote sessions. Fill in the connection below, then choose Connect / Browse.':'No active remote sessions. Connect through Terminal or the SFTP / FTP / FTPS browser first.'):'Uses the selected authenticated connection. Saving checks for external file changes.');
}
function updateRemoteActions(){const choice=document.getElementById('ntEditorRemoteSource')?.value||'',saved=choice.startsWith('saved:'),connected=/^(client|terminal):/.test(choice);for(const id of ['ntEditorOpenRemote','ntEditorOpenRemotePath','ntEditorNewRemote']){const button=document.getElementById(id);if(button)button.disabled=!connected;}const use=document.getElementById('ntEditorOpenRemote');if(use)use.hidden=saved;const connect=document.getElementById('ntEditorConnectSaved');if(connect){connect.hidden=!saved;connect.disabled=!saved||savedConnectPending;connect.textContent=savedConnectPending?'Please wait…':savedConnectionState.detached?'Request connection':'Connect';}}
async function connectSavedSource(){
 const picker=document.getElementById('ntEditorRemoteSource'),choice=picker?.value||'';
 if(savedConnectPending||!choice.startsWith('saved:'))return;
 const id=choice.slice(6),revision=remoteSelectionRevision,module=root.NetworkToolkitEditorSavedConnections;
 if(!savedConnectionState.profiles?.some(row=>row.id===id)||!module?.connect)return;
 const hint=document.getElementById('ntEditorRemoteHint'),currentSelection=()=>picker===document.getElementById('ntEditorRemoteSource')&&revision===remoteSelectionRevision;
 savedConnectPending=true;updateRemoteActions();
 try{
  const result=await module.connect(id);
  if(!currentSelection()||picker.value!==choice)return;
  if(['client:ssh','client:ftp'].includes(result.source))selectedRemoteSource=result.source;
  await refreshConnectedSources();
  if(currentSelection()&&hint===document.getElementById('ntEditorRemoteHint'))hint.textContent=result.message||(result.requested?'Connection requested. Review it in the main workspace, then Refresh.':result.pending?'Complete the existing authentication or host-verification dialog, then Refresh.':'Connected. Choose a remote path or browse files.');
 }catch(error){if(currentSelection()&&picker.value===choice){if(hint===document.getElementById('ntEditorRemoteHint'))hint.textContent='Connection was not completed: '+error.message;setMessage(error.message,true);}}
 finally{savedConnectPending=false;updateRemoteActions();}
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
 await open({id:'connected-terminal:'+id+':'+revision+':'+remote,path:remote,source:'SSH terminal - '+(session.label||session.host),binding:{kind:'terminal',sessionId:id,revision},create,
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
 const key=filePicker.sort||'name',direction=filePicker.desc?-1:1;
 const ordered=filePicker.entries.filter(x=>x&&typeof x.path==='string').slice().sort((a,b)=>{if(!!a.directory!==!!b.directory)return a.directory?-1:1;const value=x=>key==='size'?Number(x.size)||0:key==='modified'?Date.parse(x.modified)||0:key==='type'?(x.directory?'Folder':x.kind||'File'):String(x.name||'');const av=value(a),bv=value(b);return direction*(typeof av==='number'?av-bv:av.localeCompare(bv,undefined,{numeric:true,sensitivity:'base'}))||String(a.name).localeCompare(String(b.name),undefined,{numeric:true});});
 const columns=[['name','Name'],['type','Type'],['size','Size'],['modified','Modified']];
 list.innerHTML='<table class="nt-editor-picker-table" data-no-column-resize><colgroup><col class="picker-name"><col class="picker-type"><col class="picker-size"><col class="picker-modified"></colgroup><thead><tr>'+columns.map(([id,name])=>'<th scope="col" aria-sort="'+(key===id?(filePicker.desc?'descending':'ascending'):'none')+'"><button type="button" data-ed-picker-sort="'+id+'" title="Sort by '+name.toLowerCase()+'">'+name+(key===id?icon(filePicker.desc?'arrowDown':'arrowUp'):'')+'</button></th>').join('')+'</tr></thead><tbody>'+ordered.map(x=>'<tr class="nt-editor-picker-row'+(filePicker.selected===x.path?' selected':'')+'" tabindex="0" aria-selected="'+(filePicker.selected===x.path)+'" data-ed-picker-path="'+escaped(x.path)+'" data-ed-picker-dir="'+String(!!x.directory)+'" title="'+escaped(x.path)+'"><td><span class="nt-editor-picker-name">'+icon(x.directory?'folder':'file')+'<span>'+escaped(x.name)+'</span></span></td><td>'+escaped(x.directory?'Folder':x.kind||'File')+'</td><td class="mono">'+escaped(!x.directory&&typeof x.size==='number'?x.size.toLocaleString()+' bytes':'')+'</td><td>'+escaped(x.modified?(globalThis.NetworkToolkitResultTime?.format(new Date(x.modified),{milliseconds:false})||new Date(x.modified).toLocaleString(undefined,{hour12:true})): '')+'</td></tr>').join('')+(ordered.length?'':'<tr><td colspan="4" class="muted">This folder has no files.</td></tr>')+'</tbody></table>';
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
 openModal('Browse remote files','<div class="nt-editor-file-picker"><div class="nt-editor-picker-nav"><button type="button" class="btn small ghost" data-ed-picker-up title="Parent directory">'+icon('arrowUp')+' Up</button><input id="ntEditorFilePickerPath" class="mono" aria-label="Remote folder" placeholder="Remote folder" spellcheck="false"><button type="button" class="btn small ghost" data-ed-picker-go>Go</button><button type="button" class="btn small ghost" data-ed-picker-refresh>'+icon('refresh')+'</button></div><div id="ntEditorFilePickerRows" class="nt-editor-picker-list" role="region" aria-label="Remote folders and files"></div><p class="help-line">Double-click a folder to open it, or double-click a file to edit. Select a file and choose Open.</p></div>',
 '<button type="button" class="btn ghost" data-action="close-modal">Cancel</button><button type="button" class="btn primary" data-ed-picker-open disabled>Open selected file</button>',true);
 void filePickerLoad(filePicker.path);
}
function filePickerOpen(path){if(!filePicker)return;const file=filePicker.entries.find(x=>x.path===(path||filePicker.selected));if(!file)return;if(file.directory){void filePickerLoad(file.path);return;}
 const source=filePicker.source,field=document.getElementById('ntEditorRemotePath'),picker=document.getElementById('ntEditorRemoteSource');
 if(picker&&picker.value!==source){filePicker=null;closeModal();setMessage('The remote connection selection changed. Please browse again.',true);return;}
 if(field)field.value=file.path;filePicker=null;closeModal();void openRemotePath(file.path).catch(error=>{setMessage(error.message,true);if(typeof toast==='function')toast(error.message,'error');});
}
document.addEventListener('click',event=>{const node=event.target.closest?.('[data-ed-picker-sort],[data-ed-picker-path],[data-ed-picker-up],[data-ed-picker-go],[data-ed-picker-refresh],[data-ed-picker-open]');if(!node||!filePicker)return;
 event.preventDefault();event.stopImmediatePropagation();
 if(node.hasAttribute('data-ed-picker-sort')){const key=node.dataset.edPickerSort;filePicker.desc=filePicker.sort===key?!filePicker.desc:false;filePicker.sort=key;const top=document.querySelector('#ntEditorFilePickerRows').scrollTop;filePickerDraw();document.querySelector('#ntEditorFilePickerRows').scrollTop=top;document.querySelector('[data-ed-picker-sort="'+key+'"]').focus({preventScroll:true});return;}
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
document.addEventListener('keydown',event=>{const row=event.target.closest?.('[data-ed-picker-path]');if(!row||!filePicker||!['Enter',' '].includes(event.key))return;event.preventDefault();event.stopImmediatePropagation();if(event.key==='Enter')filePickerOpen(row.dataset.edPickerPath);else row.click();},true);
document.addEventListener('dblclick',event=>{const row=event.target.closest?.('[data-ed-picker-path]');if(!row||!filePicker)return;event.preventDefault();event.stopImmediatePropagation();filePickerOpen(row.dataset.edPickerPath);},true);
document.addEventListener('keydown',event=>{const input=event.target.closest?.('#ntEditorFilePickerPath');if(input&&event.key==='Enter'&&filePicker){event.preventDefault();void filePickerLoad(input.value);}},true);

function renderPage(mount){skin();embedded=true;const target=mount||document.getElementById('pageContent');if(!target)return;
 target.innerHTML='<div id="ntEditorPageMount"></div>';
 overlay.classList.add('nt-editor-embedded');target.querySelector('#ntEditorPageMount').append(overlay);overlay.hidden=false;root.NetworkToolkitEditorFiles?.initialize?.(overlay);
 if(active)draw();else{drawTabs();findEl('[data-ed-title]').textContent='Open a desktop file, an active remote connection, or create a document.';findEl('[data-ed-text]').value='';findEl('[data-ed-text]').disabled=false;findEl('[data-ed-language-select]').value='';findEl('[data-ed-highlight]').textContent='';findEl('[data-ed-gutter-lines]').textContent='1';setMessage('Ready to open a document.');}
 root.NetworkToolkitEditorWindows?.mount?.();void refreshConnectedSources();startSession();
}

function setPagePanel(name,open){if(!Object.hasOwn(pagePanels,name))return;pagePanels[name]=open===true;const panel=document.querySelector('[data-ed-page-panel="'+name+'"]'),button=document.querySelector('[data-ed-panel-toggle="'+name+'"]');if(panel)panel.hidden=!pagePanels[name];button?.setAttribute('aria-expanded',String(pagePanels[name]));keepPreferences();if(name==='remote'&&pagePanels.remote)void refreshConnectedSources();}
document.addEventListener('click',event=>{const button=event.target.closest?.('[data-ed-panel-toggle]');if(!button)return;event.preventDefault();const name=button.dataset.edPanelToggle;setPagePanel(name,!pagePanels[name]);});
document.addEventListener('click',event=>{const el=event.target.closest?.('[data-ed-page]');if(!el)return;event.preventDefault();const action=el.dataset.edPage,path=document.getElementById('ntEditorLocalPath')?.value;if(action==='choose')void openDesktopFile();if(action==='new')void newDocument();if(action==='refreshRemote')void refreshConnectedSources();if(action==='useRemote')void root.NetworkToolkitEditorFiles?.useSource?.(document.getElementById('ntEditorRemoteSource')?.value).catch(error=>setMessage(error.message,true));if(action==='connectSaved')void connectSavedSource();if(action==='browseRemote')showRemoteFilePicker();if(action==='openRemotePath'||action==='newRemote')void openRemotePath(document.getElementById('ntEditorRemotePath')?.value,action==='newRemote').catch(e=>{setMessage(e.message,true);if(typeof toast==='function')toast(e.message,'error');});if(action==='openPath'||action==='newPath')void openLocalPath(path,action==='newPath').catch(e=>{if(typeof toast==='function')toast(e.message,'error');});});
document.addEventListener('change',event=>{if(event.target.id==='ntEditorRemoteSource'){selectedRemoteSource=event.target.value;remoteSelectionRevision++;updateRemoteActions();}});
root.addEventListener('beforeunload',e=>{const list=[...documents.values()],unkept=sessionKeeping()?list.filter(d=>!session.isKept(d)):list;if(pendingEditorDialog||editorComposing()||list.some(d=>documentCloseBlocked(d))||closeNeedsConfirmation(unkept)){e.preventDefault();e.returnValue='';}});
const EDITOR_FILE_BYTES=16*1024*1024,EDITOR_DOCUMENT_BYTES=32*1024*1024,EDITOR_STATE_BYTES=256*1024*1024;
const transferFields=['disk','id','uid','path','source','binding','hash','encoding','bom','original','content','saved','savedEol','savedBOM','savedEncoding','eol','dirty','saveFailed','undo','redo','language','untitled','editRevision','view','compareView'];
function transferDocuments(ids){if(!Array.isArray(ids)||!ids.length||ids.length>12||new Set(ids).size!==ids.length)throw Error('Choose up to 12 distinct document tabs.');return ids.map(uid=>{const d=[...documents.values()].find(d=>d.uid===uid);if(!d)throw Error('A selected document is no longer open.');return d;});}
function canTransfer(ids){try{if(pendingEditorDialog||editorComposing())throw Error('Finish the open editor dialog or text composition before moving documents.');for(const d of transferDocuments(ids)){if([d.content,d.saved,d.original,...d.undo,...d.redo].some(text=>typeof text!=='string'||text.length>EDITOR_DOCUMENT_BYTES)||d.undo.length>80||d.redo.length>80)throw Error('This document exceeds the 32 MiB decoded text or 80-entry undo window-move limit. No state was discarded.');if(d.loading||d.reloading||d.saving||d.formatting||d.frozen)throw Error('Wait for document loading, saving, formatting or another move to finish.');if(d.promoting||d.disk&&diskBusy(d))throw Error('Wait for the current document edit to finish.');if(d.desktopHandle||!['local','localDraft','terminal','sshConnection'].includes(d.binding?.kind))throw Error('This document cannot move without losing its existing file access. Save it as a local file first; the original tab remains unchanged.');}return '';}catch(error){return error.message;}}
// Count serialized UTF-8 before allocating JSON: many Undo entries must not
// allocate an unbounded intermediate string just to discover the 256 MiB limit.
function assertTransferSize(value){let remaining=EDITOR_STATE_BYTES;const sizes=new Map(),charge=n=>{remaining-=n;if(remaining<0)throw Error('Document state exceeds the 256 MiB window limit. No state was discarded.');};const string=value=>{let bytes=sizes.get(value);if(bytes===undefined){bytes=2;for(let i=0;i<value.length;i++){const c=value.charCodeAt(i);if(c===34||c===92)bytes+=2;else if(c<32)bytes+=[8,9,10,12,13].includes(c)?2:6;else if(c<128)bytes++;else if(c<2048)bytes+=2;else if(c>=0xd800&&c<=0xdbff){const next=value.charCodeAt(i+1);if(next>=0xdc00&&next<=0xdfff){bytes+=4;i++;}else bytes+=6;}else bytes+=c>=0xdc00&&c<=0xdfff?6:3;if(bytes>EDITOR_STATE_BYTES)throw Error('Document state exceeds the 256 MiB window limit. No state was discarded.');}sizes.set(value,bytes);}charge(bytes);};const walk=value=>{if(typeof value==='string')string(value);else if(Array.isArray(value)){charge(2);value.forEach((item,i)=>{if(i)charge(1);walk(item===undefined?null:item);});}else if(value&&typeof value==='object'){charge(2);let first=true;for(const [key,item] of Object.entries(value)){if(item===undefined)continue;if(!first)charge(1);first=false;string(key);charge(1);walk(item);}}else charge(typeof value==='number'&&!Number.isFinite(value)?4:String(value===undefined?null:value).length);};walk(value);}
function documentTransferValue(d){const value=Object.fromEntries(transferFields.map(key=>[key,d[key]])),kind=d.binding?.kind;value.saveFailed=d.saveFailed===true;value.undo=[...d.undo];value.redo=[...d.redo];value.binding=kind==='terminal'?{kind,sessionId:d.binding.sessionId,revision:d.binding.revision}:kind==='sshConnection'?{kind,connectionId:d.binding.connectionId}:{kind};for(const field of ['view','compareView'])if(value[field])value[field]=Object.fromEntries(['start','end','direction','top','left'].map(key=>[key,value[field][key]]));return value;}
function snapshotDocument(d){if(!d||documents.get(d.id)!==d)throw Error('The document is no longer open.');if(d.loading||d.reloading||d.formatting||d.frozen)throw Error('Wait for document loading, formatting or another move to finish.');if(!['local','localDraft','terminal','sshConnection'].includes(d.binding?.kind))throw Error('This document cannot move without losing its existing file access.');if([d.content,d.saved,d.original,...d.undo,...d.redo].some(text=>typeof text!=='string'||text.length>EDITOR_DOCUMENT_BYTES)||d.undo.length>80||d.redo.length>80)throw Error('This document exceeds the window state limit. No state was discarded.');const value=documentTransferValue(d);assertTransferSize(value);return value;}
function transferCheckpoint(ids=[...documents.values()].map(d=>d.uid)){const error=ids.length?canTransfer(ids):pendingEditorDialog?'Finish the open editor dialog first.':'';if(error)throw Error(error);if(displayedDocument&&!paneWorkspace())displayedDocument.view=captureView();paneWorkspace()?.snapshot();const selected=ids.length?transferDocuments(ids):[],payload={documents:selected.map(documentTransferValue),active:selected.some(d=>d===current())?current().uid:selected[0]?.uid||'',compare:comparison?.snapshot()||null,layout:{...groups.snapshot(ids),split:splitEnabled},preferences:{compareMapZoom,compareHorizontalSync,compareUnderline,compareIntralineHighlight,compareWrapNavigation,compareMoveCaret,compareShowSummary,documentMap:documentMapOn,wordWrap,syntax,indent:tabSpaces,fontSize:editorFontSize,palette:editorPalette,colors:{...editorCustomColors},appTheme:(typeof state!=='undefined'?state.theme:document.documentElement?.dataset?.theme)==='light'?'light':'dark'}};if(payload.compare&&(!selected.some(d=>d.id===payload.compare.left)||!selected.some(d=>d.id===payload.compare.right)))payload.compare=null;assertTransferSize(payload);return payload;}
let lastCheckpointIdentity=null,lastCheckpointVersion=0;
function checkpointVersion(ids){if(displayedDocument&&!paneWorkspace())displayedDocument.view=captureView();paneWorkspace()?.snapshot();const chosen=transferDocuments(ids),identity={meta:JSON.stringify({documents:chosen.map(d=>Object.fromEntries(transferFields.filter(k=>!['content','original','saved','undo','redo'].includes(k)&&(!largeDocument(d)||!['view','compareView'].includes(k))).map(k=>[k,d[k]]))),active:current()?.uid,compare:comparison?.snapshot()||null,layout:{...groups.snapshot(ids),split:splitEnabled},preferences:{compareMapZoom,compareHorizontalSync,compareUnderline,compareIntralineHighlight,compareWrapNavigation,compareMoveCaret,compareShowSummary,documentMapOn,wordWrap,syntax,tabSpaces,editorFontSize,editorPalette,editorCustomColors,theme:typeof state!=='undefined'?state.theme:''}}),strings:chosen.flatMap(d=>[d.content,d.original,d.saved,...d.undo,...d.redo])};if(!lastCheckpointIdentity||lastCheckpointIdentity.meta!==identity.meta||lastCheckpointIdentity.strings.length!==identity.strings.length||identity.strings.some((text,index)=>text!==lastCheckpointIdentity.strings[index])){lastCheckpointIdentity=identity;lastCheckpointVersion++;}return lastCheckpointVersion;}

function transferSnapshot(ids){const snapshot=transferCheckpoint(ids);for(const d of transferDocuments(ids))d.frozen=true;draw();paneWorkspace()?.refresh();return snapshot;}
async function transferStage(batch,resolveBinding){if(windowCloseState)throw Error('The receiving editor is closing.');if(!batch||!Array.isArray(batch.documents)||!batch.documents.length||batch.documents.length+documents.size>12||typeof resolveBinding!=='function')throw Error('The receiving editor cannot accept these documents.');assertTransferSize(batch);const staged=[],uids=new Set(),ids=new Set();for(const value of batch.documents){if(!value||typeof value.uid!=='string'||typeof value.id!=='string'||typeof value.content!=='string'||uids.has(value.uid)||ids.has(value.id)||documents.has(value.id)||[...documents.values()].some(d=>d.uid===value.uid)||!Array.isArray(value.undo)||!Array.isArray(value.redo)||value.undo.some(x=>typeof x!=='string')||value.redo.some(x=>typeof x!=='string'))throw Error('The receiving editor already owns this document or the state is invalid.');uids.add(value.uid);ids.add(value.id);const binding=await resolveBinding(value.binding,value);if(typeof binding?.read!=='function'||typeof binding?.write!=='function')throw Error('The document file access could not be restored.');staged.push({...documentTransferValue(value),...binding,frozen:true,staged:true,transferActive:value.uid===batch.active,loading:false,saving:false,showStart:false});}if(windowCloseState||staged.length+documents.size>12||staged.some(d=>documents.has(d.id)||[...documents.values()].some(old=>old.uid===d.uid)))throw Error('The receiving editor changed while preparing the move.');skin();for(const d of staged)documents.set(d.id,d);if(!active)active=(staged.find(d=>d.uid===batch.active)||staged[0]).id;overlay.hidden=false;root.NetworkToolkitEditorFiles?.initialize?.(overlay);draw();return staged.map(d=>d.uid);}
function transferActivate(ids){const list=transferDocuments(ids);for(const d of list){d.frozen=false;d.staged=false;}active=(list.find(d=>d.transferActive)||list[0]).id;disposeSingleDiskView();displayedDocument=null;updatePaneFactory();draw();paneWorkspace()?.refresh();}
function transferRelease(ids){const selected=transferDocuments(ids);if(selected.some(d=>!d.frozen||d.staged))throw Error('Only frozen source documents may be released.');for(const d of selected){if(d.disk)discardDiskView(d,false);documents.delete(d.id);}active=documents.has(active)?active:[...documents.keys()][0]||'';displayedDocument=null;if(active)draw();else{if(paneWorkspace())disposePaneWorkspace();splitEnabled=false;groups.single();findEl('[data-ed-text]').value='';findEl('[data-ed-text]').disabled=false;findEl('[data-ed-highlight]').textContent='';findEl('[data-ed-gutter-lines]').textContent='';drawTabs();setMessage('Documents moved to another editor window.');}}
function transferRollback(ids,{staged=false}={}){for(const d of transferDocuments(ids)){if(staged){if(!d.staged)throw Error('Cannot remove an active document as a staged move.');documents.delete(d.id);}else{if(d.staged)throw Error('This document belongs to a pending destination.');d.frozen=false;}}if(!documents.has(active))active=[...documents.keys()][0]||'';disposeSingleDiskView();displayedDocument=null;if(active)draw();else{if(paneWorkspace())disposePaneWorkspace();splitEnabled=false;groups.single();findEl('[data-ed-text]').value='';findEl('[data-ed-text]').disabled=false;findEl('[data-ed-highlight]').textContent='';findEl('[data-ed-gutter-lines]').textContent='';drawTabs();}paneWorkspace()?.refresh();}
async function prepareWindowClose(){if(windowCloseState)return true;if(pendingEditorDialog||editorComposing()||[...documents.values()].some(d=>d.loading||d.reloading||d.saving||d.formatting||d.frozen||d.promoting)){setMessage('Finish pending editor operations before closing this window.',true);return false;}const kept=sessionKeeping()?await keepSessionForClose():new Set();if(windowCloseState)return true;const snapshot=[...documents.values()].map(d=>({d,stamp:documentStamp(d)})),unkept=snapshot.filter(({d})=>!kept.has(d)),dirty=unkept.filter(({d})=>d.dirty);if(closeNeedsConfirmation(unkept.map(x=>x.d))&&await askEditor({title:'Close editor window',message:(snapshot.some(x=>x.d.saveFailed)?'A previous save failed. ':'')+'Discard unsaved changes in '+dirty.length+' file(s) when this window closes?',accept:'Discard and close'})===null)return false;if(editorComposing()||snapshot.length!==documents.size||snapshot.some(({d,stamp})=>documents.get(d.id)!==d||documentStamp(d)!==stamp||d.loading||d.reloading||d.saving||d.formatting||d.frozen||d.promoting||d.diskEditing)){setMessage('An open document changed. Review it before closing this window.',true);return false;}windowCloseState=snapshot;for(const {d} of snapshot)d.frozen=true;if(current())draw();else if(findEl('[data-ed-text]'))findEl('[data-ed-text]').disabled=true;paneWorkspace()?.refresh();return true;}
function cancelWindowClose(){if(!windowCloseState)return;const snapshot=windowCloseState;windowCloseState=null;for(const {d} of snapshot)if(documents.get(d.id)===d)d.frozen=false;if(current())draw();else if(findEl('[data-ed-text]'))findEl('[data-ed-text]').disabled=false;paneWorkspace()?.refresh();}
function commitWindowClose(){if(!windowCloseState)throw Error('Window close was not prepared.');session?.stop();if(windowCloseState.length!==documents.size||windowCloseState.some(({d,stamp})=>documents.get(d.id)!==d||documentStamp(d)!==stamp||!d.frozen))throw Error('The editor changed before window close completed.');if(paneWorkspace())disposePaneWorkspace();for(const d of documents.values())if(d.disk)discardDiskView(d,false);splitEnabled=false;documents.clear();active='';displayedDocument=null;windowCloseState=null;if(overlay){if(embedded)detachPage();overlay.hidden=true;findEl('[data-ed-text]').value='';findEl('[data-ed-text]').disabled=false;drawTabs();}paintGeneration++;if(paintTimer!==null){root.clearTimeout(paintTimer);paintTimer=null;}root.NTKEditorLanguageService?.dispose?.();}
function applyTransferredPreferences(value){if(!value||typeof value!=='object')return;if(mapZooms.includes(value.compareMapZoom))compareMapZoom=value.compareMapZoom;compareHorizontalSync=value.compareHorizontalSync===true;compareUnderline=value.compareUnderline!==false;compareIntralineHighlight=value.compareIntralineHighlight!==false;compareWrapNavigation=value.compareWrapNavigation!==false;compareMoveCaret=value.compareMoveCaret!==false;compareShowSummary=value.compareShowSummary!==false;documentMapOn=value.documentMap===true;syncDocumentMapControl();if(['light','dark'].includes(value.appTheme)&&typeof root.setTheme==='function')root.setTheme(value.appTheme);wordWrap=value.wordWrap===true;syntax=value.syntax!==false;tabSpaces=[0,2,4,8].includes(value.indent)?value.indent:4;editorFontSize=Number.isInteger(value.fontSize)&&value.fontSize>=10&&value.fontSize<=24?value.fontSize:13;editorPalette=editorPalettes.includes(value.palette)?value.palette:'auto';editorCustomColors=Object.fromEntries(Object.entries(value.colors||{}).filter(([key,color])=>Object.hasOwn(editorColorLabels,key)&&goodEditorColor(color)));applyEditorTheme();if(current())draw();paneWorkspace()?.refresh(true);}
// Session restore (editor-session.js). Each record reopens in tab order:
// saved local files by path, unsaved local files from their kept copy with the
// last session's save baseline, and unsaved drafts or copies of remote text as
// new unsaved documents. The engine imports large kept copies itself.
const sessionBackupPath=id=>'/editor-session/backup?id='+encodeURIComponent(id);
const localRead=p=>api('/file-editor/local','POST',{action:'read-text',path:p}),localWrite=data=>api('/file-editor/local','POST',{action:'write-text',...data});
// Text tools (editor-text-tools.js). The tools run in the engine; this is
// what the module needs of the editor.
const textTools=root.NTKEditorTextTools?.create({
 current,overlay:()=>overlay,textarea:()=>findEl('[data-ed-text]'),diskView,setMessage,
 scope:()=>root.NetworkToolkitStandalone?.mode?.()==='terminal'?'terminal':'all',
 session:d=>ensureDiskSession(d),diskCall:(d,action,body)=>diskCall(d,action,body),
 focus:()=>findEl('[data-ed-text]')?.focus({preventScroll:true}),indent:()=>tabSpaces,
 engine:()=>typeof api==='function'&&!(typeof OFFLINE!=='undefined'&&OFFLINE),request:(path,method,body)=>api(path,method,body),ask:askEditor,
 blocked:d=>notText(d)?nameOf(d.path)+' is not a text file. Text tools cannot change it.':d.loading?'The document is still loading.':d.frozen?'The document is being moved between windows.':d.reloading||d.promoting||d.diskEditing||d.formatting?'Wait for the current document operation to finish.':pendingEditorDialog?'Close the open dialog first.':editorComposing()?'Finish text composition first.':comparison&&d.disk?'Close the document comparison first.':'',
 readOnly:()=>!!comparison,changed:()=>{drawTabs();status();},
 // A pane of a split or comparison view applies the result through its own
 // input path (as Convert text did); one undo step either way.
 applyRange:(d,start,end,text)=>{const ta=findEl('[data-ed-text]');if(ta?.inputElement){ta.setRangeText(text,start,end,'select');ta.focus();}else if(paneWorkspace()){ta.setRangeText(text,start,end,'select');ta.dispatchEvent(new Event('input',{bubbles:true}));}else applyEdit(d,d.content.slice(0,start)+text+d.content.slice(end),start,start+text.length);},
 pageAction:id=>{if(id==='format')void formatDocument();else linesAction(id);return true;},
});
const sessionCarry=error=>!(error?.status===400||error?.status===404||/damaged|no longer available/i.test(error?.message||''));
function sessionFormat(d,format,file=false){if(!format)return;d.encoding=format.encoding;d.bom=format.bom===true;
 // Only the engine knows whether the reopened text still has lines that end
 // differently; without them it reports the one ending that is left.
 if(format.eol!=='Mixed'||d.disk&&d.eol==='Mixed')d.eol=format.eol;if(file){d.savedEncoding=d.encoding;d.savedBOM=d.bom;d.savedEol=d.eol;}d.dirty=(d.disk?!!d.diskMeta?.dirty:d.content!==d.saved)||formatDirty(d);}
function sessionLanguage(d,record){if(record.language)d.language=languageTools?.valid?languageTools.valid(record.language):record.language;}
function keepRestoredCopy(d,record){d.sessionBackup={key:(d.disk?'disk:':'memory:')+d.uid,revision:d.editRevision,ref:record.backup,next:0};}
function showRestored(d){if(current()===d)draw();else drawTabs();paneWorkspace()?.refresh();}
function dropSessionDocument(d){if(documents.get(d.id)!==d||d.dirty)return;if(d.disk)discardDiskView(d);if(displayedDocument===d)displayedDocument=null;documents.delete(d.id);if(active===d.id)active=[...documents.keys()].at(-1)||'';if(active)draw();else if(overlay)drawTabs();}
async function sessionText(d,record){const value=await api(sessionBackupPath(record.backup.id));if(documents.get(d.id)!==d)throw Error('The tab was closed while it reopened.');return normalize(value?.text??'');}
function restoreSessionDraft(record,{name=record.name,origin='',note=''}={}){
 const format=record.format||{encoding:'UTF-8',bom:false,eol:'LF'},paged=!!diskAvailable();
 return open({id:'untitled:'+Date.now()+':'+Math.random().toString(36).slice(2,7),source:'Untitled',binding:{kind:'localDraft'},path:name||'Untitled.txt',create:true,initialContent:'',diskMode:'restore',restore:{backup:record.backup.id,hash:'',format},read:async()=>({content:''}),write:localWrite}).then(async d=>{
  if(documents.get(d.id)!==d)throw Error('The tab was closed while it reopened.');
  if(origin)d.sessionOrigin=origin;
  if(!paged){const text=await sessionText(d,record);if(text&&!setContent(d,text,false)){dropSessionDocument(d);throw Error('The unsaved text is too large to reopen in this editor mode.');}}
  else if(!d.disk)d.saved='';
  sessionFormat(d,format,!d.disk);sessionLanguage(d,record);keepRestoredCopy(d,record);showRestored(d);
  return {doc:d,note};
 }).catch(error=>({error,carry:sessionCarry(error)}));
}
// Document hand-over (editor-handover.js, peer_handover.go): a tab that the
// main Toolkit window gives to this standalone editor. A local file opens
// through the ordinary local open with all its checks; its unsaved text then
// replaces the file's text as one undoable edit, but only when the file still
// holds the bytes the main window read and the result shows the same text,
// format and unsaved state. An untitled document arrives as a new unsaved
// document with its title. Nothing here saves a file. A tab that cannot be
// carried exactly is removed again, untouched, and the main window keeps it.
const sameFormat=(d,f)=>!!f&&d.encoding===f.encoding&&d.bom===(f.bom===true)&&d.eol===f.eol;
function removeAdopted(d){if(documents.get(d.id)!==d)return false;if(d.disk)discardDiskView(d);if(displayedDocument===d)displayedDocument=null;documents.delete(d.id);if(active===d.id)active=[...documents.keys()].at(-1)||'';if(active)draw();else if(overlay)drawTabs();return true;}
// Only a tab this hand-over created, and only while nobody has edited it.
function discardAdopted(d){if(!d?.handoverAdopted||d.editRevision!==d.handoverRevision||d.frozen||d.saving||d.loading)return false;return removeAdopted(d);}
// The engine's canonical form of a local path (absolute, links resolved, the
// long form of 8.3 names, and on Windows case-insensitive): a file is never
// opened twice under another spelling. Cached per tab and path.
async function localPathKey(path){const r=await api('/file-editor/local','POST',{action:'canonical-path',path});const key=String(r?.key||'');if(!key)throw Error('The file path could not be checked.');return key;}
async function openLocalTab(path){
 const key=await localPathKey(path);
 for(const d of documents.values()){
  if(d.binding?.kind!=='local'||!d.path)continue;
  if(d.pathKeyFor!==d.path){try{d.pathKey=await localPathKey(d.path);}catch{d.pathKey='';}d.pathKeyFor=d.path;}
  if(d.pathKey===key&&documents.get(d.id)===d)return d;
 }
 return null;
}
// Undo history carried by a hand-over (owner decision, v1.21.50): applied only
// to a page-held tab showing exactly the text it belongs to, within this
// editor's own limits. Returns whether it was applied.
function adoptHistory(d,item,exact){
 const undo=item.undo,redo=item.redo;
 if(!exact||!d||d.disk||!Array.isArray(undo)&&undo!==undefined||!Array.isArray(redo)&&redo!==undefined)return false;
 const list=[...(undo||[]),...(redo||[])];
 if((undo||[]).length>80||(redo||[]).length>80||list.some(text=>typeof text!=='string'||text.length>EDITOR_DOCUMENT_BYTES))return false;
 d.undo=[...(undo||[])];d.redo=[...(redo||[])];retainUndo(d,d.undo);retainUndo(d,d.redo);
 return true;
}
// Returns {document, created, historyLost}: created is false when the file was
// already open here; historyLost when undo history came with the tab but could
// not be restored exactly (the tab is open without it).
async function adoptHandover(item){
 if(windowCloseState)throw Error('The editor is closing.');
 if(!item||typeof item!=='object'||!['file','untitled'].includes(item.kind))throw Error('The tab is invalid.');
 const text=typeof item.text==='string'?item.text:'',fail=(d,message)=>{removeAdopted(d);throw Error(message);};
 const history=Array.isArray(item.undo)&&item.undo.length>0||Array.isArray(item.redo)&&item.redo.length>0;
 if(item.dirty&&shouldPage(text))throw Error('Its unsaved text is too large for this editor mode.');
 let d,exact=false;
 if(item.kind==='untitled'){
  if(documents.size>=12)throw Error('Close an editor tab here first (12 maximum).');
  d=await open({id:'untitled:'+Date.now()+':'+Math.random().toString(36).slice(2,7),source:'Untitled',binding:{kind:'localDraft'},path:String(item.name||'Untitled.txt'),create:true,initialContent:'',read:async()=>({content:''}),write:localWrite});
  if(text&&!setContent(d,text,false))fail(d,'Its unsaved text could not be carried.');
  sessionFormat(d,item.savedFormat||item.format,true);sessionFormat(d,item.format);
  if(d.disk||d.content!==text||!!d.dirty!==!!item.dirty)fail(d,'It could not be carried exactly.');
  exact=true;
 }else{
  const path=String(item.path||''),existing=await openLocalTab(path);
  // Already open here: a saved tab needs nothing more (its own history stays); unsaved text is never merged into another tab.
  if(existing){if(!item.dirty)return {document:existing,created:false,historyLost:history};throw Error(nameOf(path)+' is already open in this editor.');}
  if(documents.size>=12)throw Error('Close an editor tab here first (12 maximum).');
  d=await openLocalPath(path,false,true);
  if(item.dirty){
   if(d.hash!==item.hash)fail(d,nameOf(path)+' changed on disk since it was opened.');
   if(d.disk||!sameFormat(d,item.savedFormat))fail(d,nameOf(path)+' could not be carried exactly.');
   if(!setContent(d,text,true))fail(d,'Its unsaved text could not be carried.');
   sessionFormat(d,item.format);
   if(d.disk||d.content!==text||d.dirty!==true||!sameFormat(d,item.format))fail(d,nameOf(path)+' could not be carried exactly.');
   exact=true;
  }else exact=!d.disk&&d.hash===item.hash; // a saved tab: the history belongs to exactly the bytes the main window read
 }
 const historyLost=history&&!adoptHistory(d,item,exact);
 sessionLanguage(d,item);d.handoverAdopted=true;d.handoverRevision=d.editRevision;showRestored(d);
 return {document:d,created:true,historyLost};
}
function restoreSessionLocal(record){
 const path=record.path,name=nameOf(path),paged=!!diskAvailable();
 return open({id:'local:'+path,source:'Local computer',binding:{kind:'local'},path,diskMode:'restore',restore:{backup:record.backup.id,hash:record.hash||'',format:record.savedFormat||record.format},read:localRead,write:localWrite}).then(async d=>{
  if(documents.get(d.id)!==d)throw Error('The tab was closed while it reopened.');
  let changed=false,missing=false;
  if(d.disk){changed=!!d.restoreStatus?.changed;missing=!!d.restoreStatus?.missing;}
  else{
   // Without disk-backed editing, the file opens as usual and the kept text
   // replaces it as one undoable edit. Saving still checks the old baseline.
   const fileHash=d.hash;let text;
   try{text=await sessionText(d,record);if(!setContent(d,text,true))throw Error('The unsaved text is too large to reopen in this editor mode.');}
   catch(error){dropSessionDocument(d);throw error;}
   if(d.content!==d.saved){d.hash=record.hash||'';changed=fileHash!==d.hash;}
  }
  sessionFormat(d,record.format);sessionLanguage(d,record);keepRestoredCopy(d,record);showRestored(d);
  return {doc:d,note:missing&&record.hash?name+' is no longer on disk. Your unsaved version is shown; use Save As to keep it.':changed?name+' changed on disk since your last session. Your unsaved version is shown.':''};
 },error=>paged?{error,carry:sessionCarry(error)}:restoreSessionDraft(record,{name,note:name+' could not be opened ('+error.message+'). Your unsaved text was restored into a new untitled tab; use Save As to keep it.'}));
}
// A document that was asked for is the active tab, whatever the last session
// held: a file from Explorer (a launch request), a file or text opened from
// another tool, or a tab the user chose. The session may be reopened before
// or after such a request (the two start together when the app starts with a
// file), so until it has been reopened the last request is remembered here.
// Tabs of the last session then reopen behind it, and the session's own
// "which tab was active" applies only when nothing was asked for.
let restoringRecord=false,finishingRestore=false,askedBeforeRestore='';
function noteAsked(id){if(!restoringRecord&&!finishingRestore&&session&&['idle','restoring','failed'].includes(session.phase))askedBeforeRestore=id;}
const askedDocument=()=>askedBeforeRestore?documents.get(askedBeforeRestore)||current()||null:null;
// Called in tab order; each call adds its tab before it waits for anything.
function restoreSessionRecord(record){restoringRecord=true;try{return reopenSessionRecord(record);}finally{restoringRecord=false;}}
function reopenSessionRecord(record){
 if(documents.size>=12)return {carry:true,limit:true};
 if(record.kind==='local'){
  const existing=documents.get('local:'+record.path),name=nameOf(record.path);
  if(!record.dirty){
   if(existing)return {doc:existing};
   if(record.exists===false)return {note:name+' is no longer on disk, so its tab was not reopened.'};
   return openLocalPath(record.path,false,true).then(d=>{sessionLanguage(d,record);return {doc:d};},error=>({error}));
  }
  if(existing)return restoreSessionDraft(record,{name,origin:record.path,note:name+' was already open, so your unsaved version from the last session opened in a separate tab.'});
  if(record.exists===false&&record.hash)return restoreSessionDraft(record,{name,note:name+' no longer exists. Your unsaved text was restored into a new untitled tab; use Save As to keep it.'});
  return restoreSessionLocal(record);
 }
 return restoreSessionDraft(record,{name:record.name,origin:record.kind==='copy'?record.origin:'',note:record.kind==='copy'?'Restored unsaved text from '+record.origin+'. Reconnect and use Save As to save it there.':''});
}
// Restored tabs keep their stored order ahead of tabs opened meanwhile (for
// example from Explorer). The active tab is the document that was asked for
// meanwhile, otherwise the one the session stored.
function finishSessionRestore({docs,active:chosen}){
 const restored=docs.filter((d,index)=>documents.get(d.id)===d&&docs.indexOf(d)===index),list=[...documents.values()];
 const order=[...restored,...list.filter(d=>!restored.includes(d))];
 if(order.some((d,index)=>d!==list[index])){documents.clear();for(const d of order)documents.set(d.id,d);}
 finishingRestore=true;
 try{
  if(chosen&&documents.get(chosen.id)===chosen&&current()!==chosen&&!editorComposing())activateDocument(chosen.id);
  else if(overlay&&documents.size)drawTabs();
 }finally{finishingRestore=false;askedBeforeRestore='';}
 root.NetworkToolkitEditorWindows?.notifyChanged?.();
}
const sessionReadiness=()=>!sessionWorkspace()?'never':(typeof stateLoaded!=='undefined'&&!stateLoaded)||(typeof online!=='undefined'&&!online)?'wait':!sessionEngine()?'never':restoreSessionPreference()?'ready':'off';
const session=root.NTKEditorSession?.create({
 request:(path,method='GET',body)=>api(path,method,body),
 documents:()=>[...documents.values()],
 activeId:()=>active,
 stamp:documentStamp,
 flush:d=>d.diskSession?.flush(),
 pending:d=>!!d.diskSession?.pending,
 lease:d=>root.NetworkToolkitEditorWindows?.state?.owned?.[d.uid]||0,
 enabled:()=>sessionAvailable()&&restoreSessionPreference(),
 idle:()=>!windowCloseState&&!pendingEditorDialog&&!editorComposing()&&![...documents.values()].some(d=>d.promoting||d.diskEditing||d.reloading||d.staged||d.frozen),
 readiness:sessionReadiness,
 restoreRecord:restoreSessionRecord,
 requested:askedDocument,
 finishRestore:finishSessionRestore,
 message:(text,error)=>{if(overlay)setMessage(text,error);if(error&&typeof toast==='function')toast(text,'error');}
});
function startSession(){if(session&&session.phase==='idle')session.start();}
const sessionKeeping=()=>!!session&&session.phase==='ready'&&sessionAvailable()&&restoreSessionPreference();
// Keep the final session before a window close asks about unsaved changes.
// Documents are read-only while their last copies are written.
async function keepSessionForClose(){
 // With "Reopen files next time" off nothing is kept, and kept copies of the
 // open documents leave the session before closing asks about them.
 if(!sessionKeeping()){try{await session?.settle?.();}catch{}return new Set();}
 const list=[...documents.values()];
 for(const d of list)d.frozen=true;
 if(current())draw();paneWorkspace()?.refresh();
 if(list.some(d=>d.dirty))setMessage('Keeping unsaved changes for the next launch…');
 try{return await session.save({final:true});}
 catch{return new Set();}
 finally{for(const d of list)if(documents.get(d.id)===d)d.frozen=false;if(current())draw();paneWorkspace()?.refresh();}
}
const windowAdapter=Object.freeze({async flushDocuments(ids){for(const d of transferDocuments(ids))if(d.diskSession)await d.diskSession.flush();},
// The engine refuses edits once the main workspace starts closing, so pending
// disk-backed typing is sent first and can then be kept for the next launch.
async beforeMainClose(){if(!sessionKeeping())return;for(const d of documents.values())if(d.diskSession?.pending)await d.diskSession.flush().catch(()=>{});},activityDocuments,setClosePolicy,createBlankDocument:newDocument,hasPendingWindowClose:()=>windowCloseState!==null,prepareWindowClose,cancelWindowClose,commitWindowClose,applyPreferences:applyTransferredPreferences,canTransfer,checkpointVersion,checkpoint:transferCheckpoint,snapshotDocument,snapshot:transferSnapshot,stage:transferStage,activate:transferActivate,release:transferRelease,rollback:transferRollback,documents:()=>[...documents.values()],active:()=>current()?.uid||'',compare:()=>comparison?.snapshot()||null,restoreLayout(value,{merge=false}={}){if(!value)return;groups.restore(value,{merge});if(merge&&current())groups.activate(current().uid);if(!merge){splitEnabled=value.split===true;if(paneWorkspace())disposePaneWorkspace();if(splitEnabled)createPaneWorkspace(false);}else if(value.split&&!paneWorkspace()){splitEnabled=true;createPaneWorkspace(false);}drawTabs();paneWorkspace()?.refresh(true);},restoreCompare(value){if(value){openComparison();comparison?.restore(value);}}});

function closeTabMenu(focus=true){if(!tabMenu)return;const uid=tabMenu.dataset.documentUID,back=tabMenu.returnFocus;tabMenu.remove();tabMenu=null;if(!focus)return;const origin=typeof back==='function'?back():null;(origin?.isConnected?origin:overlay.querySelector('[data-ed-uid="'+uid+'"] [data-ed-open]'))?.focus({preventScroll:true});}
// The tab strip's menu. The Document List opens it too: `back` finds the row
// that keeps the focus when the menu closes.
function openTabMenu(d,x,y,back=null){closeTabMenu(false);activateDocument(d.id);const blocked=!!canReorderDocuments(),other=groups.sideOf(d.uid)==='left'?'right':'left';const actions=[['save','Save'],['saveAs','Save as…'],['reload','Reload'],['compareTo','Compare to…'],null,['move','Move to '+other+' view'],['first','Move to start of view'],['last','Move to end of view'],['stepLeft','Move one tab left (Alt+Shift+Left)'],['stepRight','Move one tab right (Alt+Shift+Right)'],['detach','Move to separate window'],null,['copyName','Copy file name'],['copyPath','Copy path'],null,['close','Close'],['others','Close other tabs in this view'],['following','Close tabs to the right']];const menu=document.createElement('div');menu.className='nt-editor-tab-menu';menu.setAttribute('role','menu');menu.setAttribute('aria-label','Document tab actions');menu.dataset.documentUID=d.uid;menu.innerHTML=actions.map(a=>a?'<button type="button" role="menuitem" tabindex="-1" data-ed-context-action="'+a[0]+'" '+((blocked&&!['copyName','copyPath'].includes(a[0]))||(a[0]==='detach'&&!root.NetworkToolkitEditorWindows?.state?.enabled)||(a[0]==='compareTo'&&(documents.size<2||!root.NTKEditorCompare))?'disabled':'')+'>'+a[1]+'</button>':'<div role="separator"></div>').join('');overlay.querySelector('.nt-editor-window').append(menu);tabMenu=menu;menu.returnFocus=back;const rect=menu.getBoundingClientRect();menu.style.left=Math.max(6,Math.min(x,innerWidth-rect.width-6))+'px';menu.style.top=Math.max(6,Math.min(y,innerHeight-rect.height-6))+'px';menu.querySelector('button:not(:disabled)')?.focus({preventScroll:true});}
function openCompareTabMenu(d){
 const error=canReorderDocuments();if(error){setMessage(error,true);closeTabMenu();return;}
 const others=[...documents.values()].filter(other=>other!==d);if(!others.length||!root.NTKEditorCompare){setMessage('Open another document in this window to compare.');closeTabMenu();return;}
 const x=parseFloat(tabMenu?.style.left)||6,y=parseFloat(tabMenu?.style.top)||6;closeTabMenu(false);
 const menu=document.createElement('div');menu.className='nt-editor-tab-menu nt-editor-compare-menu';menu.setAttribute('role','menu');menu.setAttribute('aria-label','Compare '+nameOf(d.path)+' to another open document');menu.dataset.documentUID=d.uid;
 menu.innerHTML='<div class="nt-editor-compare-menu-title" role="presentation">Compare '+escaped(nameOf(d.path))+' to…</div>'+others.map(other=>'<button type="button" role="menuitem" tabindex="-1" data-ed-compare-target="'+escaped(other.uid)+'"><strong>'+escaped(nameOf(other.path))+(other.dirty?' · Unsaved':'')+'</strong><small>'+escaped(other.source+' · '+other.path)+' · Tab '+([...documents.values()].indexOf(other)+1)+'</small></button>').join('')+'<div role="separator"></div><button type="button" role="menuitem" tabindex="-1" data-ed-compare-cancel>Cancel</button>';
 overlay.querySelector('.nt-editor-window').append(menu);tabMenu=menu;const rect=menu.getBoundingClientRect();menu.style.left=Math.max(6,Math.min(x,innerWidth-rect.width-6))+'px';menu.style.top=Math.max(6,Math.min(y,innerHeight-rect.height-6))+'px';menu.querySelector('button')?.focus({preventScroll:true});
}
function compareTabTarget(uid){
 const left=[...documents.values()].find(d=>d.uid===tabMenu?.dataset.documentUID),right=[...documents.values()].find(d=>d.uid===uid),error=canReorderDocuments();closeTabMenu(false);
 if(error||!left||!right||left===right||!root.NTKEditorCompare){setMessage(error||'Those documents are no longer available for comparison.',true);return;}
 const restore=comparisonReturnLayout||groups.snapshot();openComparison();if(!comparison)return;comparisonReturnLayout=restore;
 groups.choose('left',left.uid,{replace:!splitEnabled});groups.choose('right',right.uid,{replace:!splitEnabled});active=left.id;groups.activate(left.uid);drawTabs();syncEditorControls(left);comparison.refresh(true);overlay.querySelector('[data-ed-view="left"] [data-ed-uid="'+left.uid+'"] [data-ed-open]')?.focus({preventScroll:true});setMessage('Comparing '+nameOf(left.path)+' (left) to '+nameOf(right.path)+' (right).');
}
function tabMenuKey(event){if(!tabMenu)return;const items=[...tabMenu.querySelectorAll('button:not(:disabled)')],at=items.indexOf(document.activeElement);if(['ArrowDown','ArrowUp','Home','End'].includes(event.key)){event.preventDefault();event.stopPropagation();const next=event.key==='Home'?0:event.key==='End'?items.length-1:(at+(event.key==='ArrowUp'?-1:1)+items.length)%items.length;items[next]?.focus();}else if(['Escape','Tab'].includes(event.key)){event.preventDefault();event.stopPropagation();closeTabMenu();}}
async function runTabMenu(action){const uid=tabMenu?.dataset.documentUID,d=[...documents.values()].find(d=>d.uid===uid);if(action==='compareTo'&&d){openCompareTabMenu(d);return;}closeTabMenu(false);if(!d)return;activateDocument(d.id);try{if(action==='save')await save();else if(action==='saveAs')await save(true);else if(action==='reload')await reload();else if(action==='move')moveDocumentToView(uid);else if(action==='detach')await root.NetworkToolkitEditorWindows.move([uid],'new');else if(action==='copyName'||action==='copyPath')await root.copyText(action==='copyName'?nameOf(d.path):d.path);else{const list=paneWorkspace()?groups.list(groups.sideOf(uid)):[...documents.values()],at=list.indexOf(d);if(action==='stepLeft'){if(at>0)reorderDocument(uid,list[at-1].uid);}else if(action==='stepRight'){if(at>=0&&at<list.length-1)reorderDocument(uid,list[at+2]?.uid||null);}else if(action==='first')reorderDocument(uid,list[0]?.uid||null);else if(action==='last'){const after=[...documents.values()].find((other,index,all)=>index>all.indexOf(list.at(-1)));reorderDocument(uid,after?.uid||null);}else{const chosen=action==='close'?[d]:action==='others'?list.filter(x=>x!==d):action==='following'?list.slice(at+1):[];for(const item of chosen)if(!await closeTab(item.id))break;}}}catch(error){setMessage(error.message,true);}}

const compareMenuControls=[
 ['compareSwap','Swap comparison sides','[data-compare="swap"]'],
 ['compareFirst','First difference','[data-compare="first"]'],
 ['comparePrevious','Previous difference','[data-compare="previous"]'],
 ['compareNext','Next difference','[data-compare="next"]'],
 ['compareLast','Last difference','[data-compare="last"]'],
 ['compareOnly','Show only differences','[data-compare-only]'],
 ['compareMap','Show change map','[data-compare-show-map]'],
 ['compareSync','Sync vertical scrolling','[data-compare-sync]'],
 ['compareWrapNav','Wrap around differences','[data-compare-wrap-nav]'],
 ['compareMoveCaret','Select changed text on navigation','[data-compare-move-caret]'],
 ['compareSummary','Show comparison summary','[data-compare-show-summary]'],
 ['compareHorizontal','Sync horizontal scrolling','[data-compare-horizontal-sync],[data-compare-horizontal]'],
 ['compareUnderline','Underline changes','[data-compare-underline]'],
 ['compareCharacters','Highlight changed characters','[data-compare-intraline]'],
 ['compareExact','Whitespace: exact','[data-compare-spaces]',''],
 ['compareChangedSpaces','Ignore changed spaces','[data-compare-spaces]','changed'],
 ['compareAllSpaces','Ignore all spaces','[data-compare-spaces]','all']
];
// The Tools menu and the right-click menu list the text tools in groups;
// each item says whether it can run on the active document now.
function toolMenuGroups(){
 const tools=root.NTKEditorTextTools;if(!tools||!textTools)return [];
 const d=current(),open=!!d&&!d.loading,editable=open&&!d.frozen&&!comparison&&!notText(d),convertible=open&&!d.frozen&&!d.reloading&&!notText(d)&&!(comparison&&d.disk),empty=!d||!(d.disk?d.diskMeta?.units:d.content.length),language=d?languageOf(d):'',kind=languageTools?.formatKind(language)||'';
 const enabled=tool=>{
  if(tool.id==='format')return editable&&!d.disk&&!!kind&&d.content.length<=250000&&!d.formatting;
  if(tool.id==='comment'||tool.id==='duplicate')return editable;
  return convertible&&!empty;
 };
 return tools.GROUPS.map(group=>({...group,tools:tools.TOOLS.filter(tool=>tool.group===group.id&&textTools.allows(tool.id)).map(tool=>({...tool,label:tool.id==='format'&&kind==='indent'?'Indent document':tool.label,enabled:enabled(tool)}))})).filter(group=>group.tools.length);
}
const textMenu=Object.freeze({
 // The text area the right-click menu applies to (single view or the focused split pane).
 target(el){const ta=findEl('[data-ed-text]'), input=el?.closest?.('[data-compare-text]')||el;return !!input&&!!overlay&&!overlay.hidden&&overlay.contains(input)&&(input===(ta?.inputElement||ta)||input.matches('[data-compare-text]'));},
 focusTarget(el){el?.closest?.('[data-compare-text]')?.focus({preventScroll:true});},
 toolGroups:toolMenuGroups,
 runTool:id=>textTools?textTools.run(id):Promise.resolve(false),
 compareActions(){
  if(!comparison)return [];
  return [['compareOptions','Compare options...',true],['compareRun','Recalculate comparison',!comparison.composing],...compareMenuControls.flatMap(([action,label,selector,value])=>{
   const control=comparisonHost.querySelector(selector);
   if(!control||control.closest('label')?.hidden)return [];
   const checked=control.type==='checkbox'?control.checked:value===undefined?undefined:control.value===value;
   return [[action,label,!control.disabled,checked]];
  })];
 },
 state(){const d=current(),ta=findEl('[data-ed-text]'),range=d?.disk?diskView()?.selection():null,a=range?.start??ta?.selectionStart??0,b=range?.end??ta?.selectionEnd??0;return {open:!!d,binary:notText(d),editable:!!d&&!d.loading&&!d.frozen&&!comparison,convertible:!!d&&!d.loading&&!d.frozen&&!d.reloading&&!ta?.disabled,selection:!!d&&b>a,empty:!d||!(d.disk?d.diskMeta?.units:d.content.length),canUndo:!!(d?.disk?d.diskMeta?.canUndo:d?.undo?.length),canRedo:!!(d?.disk?d.diskMeta?.canRedo:d?.redo?.length)};},
 selection(){const d=current(),ta=findEl('[data-ed-text]');if(!d||!ta)return '';return d.content.slice(Math.min(ta.selectionStart,ta.selectionEnd),Math.max(ta.selectionStart,ta.selectionEnd));},
 copy(cut){return current()?.disk?diskView()?.copy(cut):undefined;},
 replace(text){const d=current();if(!d||d.loading||d.frozen||comparison)return false;if(d.disk)return diskView()?.replace(String(text??'')).then(()=>true);replaceSelection(String(text??''));status();return true;},
 selectAll(){if(current()?.disk){diskView()?.selectAll();return;}const ta=findEl('[data-ed-text]');if(!ta)return;ta.focus({preventScroll:true});ta.select();status();},
 focus(){findEl('[data-ed-text]')?.focus({preventScroll:true});},
 run(action){
  if(comparison&&action==='compareOptions'){
   const disclosure=comparisonHost.querySelector('.nt-compare-settings');
   disclosure.open=true;
   disclosure.querySelector('summary').focus({preventScroll:true});
   return true;
  }
  if(comparison&&action==='compareRun'){
   if(comparison.composing)return false;
   if(comparison.recalculate)void comparison.recalculate();else comparison.refresh(true);
   return true;
  }
  const compareControl=compareMenuControls.find(([name])=>name===action);
  if(comparison&&compareControl){
   const [,,selector,value]=compareControl,control=comparisonHost.querySelector(selector);
   if(!control||control.disabled)return false;
   if(value===undefined)control.click();
   else if(control.value!==value){control.value=value;control.dispatchEvent(new Event('change',{bubbles:true}));}
   return true;
  }
  if(root.NTKEditorTextTools?.tool(action)){void textTools?.run(action);return true;}
  const button=overlay?.querySelector('[data-ed="'+action+'"]');if(!button||button.disabled)return false;button.click();return true;}
});
root.NetworkToolkitFileEditor={session:Object.freeze({get phase(){return session?.phase||'unavailable';},save:()=>session?session.save():Promise.resolve(new Set()),tick:()=>!!session?.tick(),isKept:d=>!!session?.isKept(d)}),textMenu,listDocuments,insertText,pickInsert,cancelInsertPick:endPick,insertPickActive:()=>!!pickState,openTabActions(uid){const d=[...documents.values()].find(item=>item.uid===uid);if(!d||!overlay||overlay.hidden)return false;const button=[...overlay.querySelectorAll('[data-ed-tab-actions]')].find(el=>el.dataset.edTabActions===uid);if(!button)return false;const rect=button.getBoundingClientRect();openTabMenu(d,rect.left,rect.bottom);return true;},beginSidebarPointerDrag,finishSidebarPointerDrop,cancelSidebarPointerDrag,requestName:(title,message,value)=>{skin();return askEditor({title,message,value,accept:'Create',validate:value=>value.trim()?'':'Enter a name.'});},moveDocumentToView,openSplit,get viewGroups(){return groups.snapshot();},windowAdapter,canReorderDocuments,reorderDocument,open,openDesktopFile,importFiles,openLocalPath,newDocument,adoptHandover,discardAdopted,renderPage,detachPage,closeAll,preferConnection,refreshConnectedSources,openRemotePath,showRemoteFilePicker,get documents(){return documents;},activateDocument,openComparison,closeComparison,get comparison(){return comparison;},get active(){return current();}};
// The Document Map and the Document List (editor-document-map.js, editor-document-list.js).
Object.defineProperties(root.NetworkToolkitFileEditor,{documentModel:{value:documentModel},openTabMenuAt:{value:openTabMenuAt},focusText:{value:focusText},setDocumentMap:{value:setDocumentMap},toggleDocumentMap:{value(){skin();setDocumentMap(!documentMapOn);return documentMapOn;}},documentMapOn:{get:()=>documentMapOn},documentMap:{get:()=>documentMap}});
})(globalThis);
