(function(){
 'use strict';
 const M=window.NetworkToolkitHostsModel;
 const ui={view:'entries',query:'',filter:'all',page:0,selected:new Set(),undo:[],redo:[],last:null,analysis:null,analysisText:null,issues:false,wrap:false,findOpen:false,find:'',replace:'',matchCase:false,whole:false,typingAt:0};
 const PAGE=100;
 let analyzeTimer,imported=null,editBase=null;
 const item=()=>localConfiguration.hosts;
 const active=()=>state.activePage==='hosts';
 const editable=()=>!!item().snapshot?.editable&&!item().loading&&!localOperationBusy('hosts');
 const q=selector=>document.querySelector(selector);
 const button=(action,label,disabled=false,extra='')=>'<button type="button" class="btn small ghost" data-hosts-action="'+action+'"'+(disabled?' disabled':'')+extra+'>'+label+'</button>';
 const doc=()=>{
  if(ui.analysisText!==item().draft){ui.analysis=M.parse(item().draft);ui.analysisText=item().draft;}
  return ui.analysis;
 };
 function trimHistory(stack){let total=stack.reduce((n,x)=>n+x.text.length*2,0);while(stack.length>1&&(stack.length>100||total>16*1024*1024))total-=stack.shift().text.length*2;}
 function capture(){const editor=q('#hostsSourceEditor');return {text:item().draft,start:editor?.selectionStart||0,end:editor?.selectionEnd||0};}
 function setDraft(text,{typing=false}={}){
  if(!editable())return;
  text=M.normalize(text);if(text===item().draft)return;
  const now=Date.now();
  if(!typing||now-ui.typingAt>700){ui.undo.push(capture());trimHistory(ui.undo);}
  ui.typingAt=typing?now:0;ui.redo=[];item().draft=text;item().error='';ui.last=text;ui.selected.clear();ui.analysisText=null;
  if(typing){clearTimeout(analyzeTimer);controls();analyzeTimer=setTimeout(()=>{doc();controls();renderIssues();updateFindCount();},180);}
  else{doc();render();}
 }
 function travel(direction){
  if(!editable())return;
  const from=direction==='undo'?ui.undo:ui.redo,to=direction==='undo'?ui.redo:ui.undo;if(!from.length)return;
  to.push(capture());trimHistory(to);const entry=from.pop();item().draft=entry.text;ui.last=entry.text;ui.typingAt=0;ui.selected.clear();ui.analysisText=null;render();
  const editor=q('#hostsSourceEditor');if(editor){editor.focus();editor.setSelectionRange(Math.min(entry.start,entry.text.length),Math.min(entry.end,entry.text.length));}
 }
 function render(){
  if(!active())return;
  const target=q('#pageContent'),current=item(),snapshot=current.snapshot;
  if(ui.last!==null&&ui.last!==current.draft){ui.undo=[];ui.redo=[];ui.selected.clear();ui.analysisText=null;}
  ui.last=current.draft;
  const ready=!!snapshot?.available;
  target.innerHTML='<section id="hostsConfiguration" class="local-config-panel hosts-config-panel he-app" data-no-value-copy>'+
   '<div class="local-config-toolbar"><div class="grow"><h3>Hosts file editor</h3><p class="muted">Manage local hostname mappings and review changes before saving.</p></div><button type="button" class="btn small ghost" data-local-config-action="hosts-reload" '+(!online||current.loading||localOperationBusy('hosts')?'disabled':'')+'>'+icon('refresh',current.loading?'spinner':'')+'Reload</button><button type="button" class="btn small primary" id="hostsReviewButton" data-local-config-action="hosts-review" disabled>'+icon('save')+'Review and save</button></div>'+
   '<div id="hostsConfigurationStatus" class="local-config-status" aria-live="polite">'+localConfigurationStatusHTML('hosts')+'</div>'+
   (!ready?'<div class="local-config-scroll">'+(!snapshot?empty('file',current.loading?'Reading the hosts file':online?'Hosts file has not been loaded':'Open the Windows application',current.loading?'The existing file will appear here.':online?'Select Reload to read this computer\'s hosts file.':'Hosts file editing is available in the Windows application.'):'<div class="notice info">'+esc(snapshot.note||'Hosts file editing is unavailable.')+'</div>')+'</div>':
    '<div class="hosts-file-meta"><span class="mono he-path" title="'+esc(snapshot.path)+'">'+esc(snapshot.path)+'</span><div class="row gap6 wrap"><span>'+esc(snapshot.encoding||'Text')+'</span><span>'+esc(snapshot.newline||'')+'</span><strong id="hostsDraftStatus"></strong></div></div>'+
    (snapshot.note?'<div class="hosts-file-notes">'+esc(snapshot.note)+'</div>':'')+
    '<div class="he-toolbar"><div class="he-tabs" role="tablist" aria-label="Hosts editor view"><button type="button" id="hostsViewEntries" role="tab" aria-controls="hostsEntries" data-hosts-view="entries" aria-selected="'+(ui.view==='entries')+'">Entries</button><button type="button" id="hostsViewText" role="tab" aria-controls="hostsText" data-hosts-view="text" aria-selected="'+(ui.view==='text')+'">Text</button></div>'+button('add','Add entry',!editable())+button('undo','Undo',!ui.undo.length||!editable())+button('redo','Redo',!ui.redo.length||!editable())+button('find','Find / replace')+
    '<details class="he-more"><summary class="btn small ghost">More</summary><div class="he-menu">'+button('format','Format entries',!editable())+button('sort','Sort entry groups by hostname',!editable())+button('deduplicate','Remove exact duplicates',!editable())+button('import','Import / restore backup',!editable())+button('export','Export draft')+button('export-loaded','Export loaded copy')+button('preview','Preview changes',!hostsConfigurationDirty())+button('revert','Revert unsaved changes',!editable()||!hostsConfigurationDirty())+'</div></details>'+button('issues','Validation',false,' id="hostsIssueButton" aria-expanded="'+ui.issues+'"')+'<span class="he-counts" id="hostsEntryCounts"></span></div>'+
    '<div id="hostsIssues" class="he-issues"'+(ui.issues?'':' hidden')+'></div>'+
    '<div id="hostsEntryControls" class="he-entry-controls"'+(ui.view==='entries'?'':' hidden')+'><label class="he-filter"><span class="sr-only">Filter hosts entries</span><input type="search" id="hostsEntrySearch" placeholder="Search IP, hostname, alias or comment" value="'+esc(ui.query)+'" autocomplete="off"></label><select id="hostsEntryFilter" aria-label="Entry status filter"><option value="all">All entries</option><option value="enabled">Enabled</option><option value="disabled">Disabled</option><option value="issues">With validation issues</option></select><span id="hostsSelectionCount" class="faint"></span>'+button('enable','Enable selected',true)+button('disable','Disable selected',true)+button('delete','Delete selected',true)+'</div>'+
    '<div id="hostsFindPanel" class="he-find"'+(ui.view==='text'&&ui.findOpen?'':' hidden')+'><label><span>Find</span><input id="hostsFind" value="'+esc(ui.find)+'" autocomplete="off" placeholder="Find text"></label><label><span>Replace with</span><input id="hostsReplacement" value="'+esc(ui.replace)+'" autocomplete="off"></label><label class="he-check"><input type="checkbox" id="hostsFindCase"'+(ui.matchCase?' checked':'')+'>Match case</label><label class="he-check"><input type="checkbox" id="hostsFindWhole"'+(ui.whole?' checked':'')+'>Whole hostname</label>'+button('find-next','Next')+button('find-previous','Previous')+button('replace-one','Replace',!editable())+button('replace-all','Replace all',!editable())+'<span class="faint" id="hostsFindCount"></span>'+button('close-find','Close')+'</div>'+
    '<div id="hostsEntries" class="he-entries" role="tabpanel" aria-labelledby="hostsViewEntries"'+(ui.view==='entries'?'':' hidden')+'><div class="he-table-scroll"><table class="he-table"><thead><tr><th><input type="checkbox" id="hostsSelectPage" aria-label="Select entries on this page"></th><th>Enabled</th><th>IP address</th><th>Hostnames / aliases</th><th>Comment</th><th>Line</th><th>Actions</th></tr></thead><tbody id="hostsEntryRows"></tbody></table></div><div class="he-pagination"><span id="hostsPageSummary"></span><div>'+button('select-matching','Select all matches')+button('clear-selection','Clear selection')+button('previous-page','Previous')+button('next-page','Next')+'</div></div></div>'+
    '<div id="hostsText" class="he-text" role="tabpanel" aria-labelledby="hostsViewText"'+(ui.view==='text'?'':' hidden')+'><div class="he-text-tools">'+button('goto','Go to line')+'<label class="he-check"><input type="checkbox" id="hostsWordWrap"'+(ui.wrap?' checked':'')+'>Word wrap</label><span class="faint">Ctrl+F find · Ctrl+H replace · Ctrl+S review and save</span></div><div class="he-source"><pre id="hostsLineNumbers" class="he-line-numbers" aria-hidden="true"'+(ui.wrap?' hidden':'')+'></pre><textarea id="hostsSourceEditor" class="he-textarea'+(ui.wrap?' is-wrap':'')+'" aria-label="Hosts file source text" wrap="'+(ui.wrap?'soft':'off')+'" spellcheck="false" autocomplete="off" autocapitalize="off"></textarea></div></div>'+
    '<div class="hosts-editor-help"><span id="hostsLineCount"></span><span>Drafts and undo history stay in this session. Changes take effect after Review and save.</span></div><input type="file" id="hostsImportFile" hidden>')+'</section>';
  if(ready){const editor=q('#hostsSourceEditor');editor.value=current.draft;q('#hostsEntryFilter').value=ui.filter;doc();renderRows();renderIssues();lineNumbers();controls();updateFindCount();}
  if(online&&!snapshot&&!current.loading&&!current.attempted)void loadLocalConfiguration('hosts');
 }
 function filtered(){
  const data=doc(),term=ui.query.toLowerCase();
  return data.rows.filter(row=>(ui.filter==='all'||ui.filter==='enabled'&&row.enabled||ui.filter==='disabled'&&!row.enabled||ui.filter==='issues'&&data.related.has(row.line))&&(!term||[row.address,...row.hosts,row.comment].join(' ').toLowerCase().includes(term)));
 }
 function renderRows(){
  const body=q('#hostsEntryRows');if(!body)return;
  const rows=filtered(),pages=Math.max(1,Math.ceil(rows.length/PAGE));ui.page=Math.min(ui.page,pages-1);const shown=rows.slice(ui.page*PAGE,(ui.page+1)*PAGE),data=doc();
  body.innerHTML=shown.length?shown.map(row=>'<tr data-hosts-line="'+row.line+'" class="'+(!row.valid?'he-invalid':data.related.has(row.line)?'he-warning':'')+'"><td><input type="checkbox" data-hosts-select="'+row.line+'" aria-label="Select line '+row.line+'"'+(ui.selected.has(row.line)?' checked':'')+'></td><td><input type="checkbox" data-hosts-enabled="'+row.line+'" data-hosts-valid="'+row.valid+'" aria-label="Enable mapping on line '+row.line+'"'+(row.enabled?' checked':'')+(!editable()||!row.valid?' disabled':'')+'></td><td class="mono">'+esc(row.address||'Invalid entry')+'<small>'+(!row.valid?'Invalid mapping':data.related.has(row.line)?'Review validation':'IPv'+row.family)+'</small></td><td><button type="button" class="he-name" data-hosts-edit="'+row.line+'"'+(!editable()?' disabled':'')+'>'+esc(row.hosts.join(' ')||'Correct this entry')+'</button></td><td>'+esc(row.comment)+'</td><td>'+row.line+'</td><td><div class="he-row-actions"><button type="button" class="btn small ghost" data-hosts-edit="'+row.line+'"'+(!editable()?' disabled':'')+'>Edit</button><button type="button" class="btn small ghost" data-hosts-jump="'+row.line+'">Text</button></div></td></tr>').join(''):'<tr><td colspan="7" class="he-empty">'+(data.rows.length?'No entries match this filter.':'No mappings yet. Add an entry or use the Text view.')+'</td></tr>';
  q('#hostsPageSummary').textContent=rows.length?(ui.page*PAGE+1)+'-'+Math.min((ui.page+1)*PAGE,rows.length)+' of '+rows.length.toLocaleString()+' entries':'0 entries';
  q('[data-hosts-action="previous-page"]').disabled=ui.page===0;q('[data-hosts-action="next-page"]').disabled=ui.page>=pages-1;
  const all=q('#hostsSelectPage'),chosen=shown.filter(row=>ui.selected.has(row.line)).length;all.checked=shown.length>0&&chosen===shown.length;all.indeterminate=chosen>0&&chosen<shown.length;
  controls();
 }
 function controls(){
  if(!active()||!q('#hostsConfiguration'))return;
  const current=item(),dirty=hostsConfigurationDirty(),allowed=editable(),checked=ui.analysisText===current.draft?ui.analysis:null;
  const save=q('#hostsReviewButton');if(save)save.disabled=!online||!allowed||!dirty||!checked||checked.errors>0;
  const status=q('#hostsDraftStatus');if(status)status.textContent=dirty?'Unsaved changes':'Saved';
  const editor=q('#hostsSourceEditor');if(editor)editor.readOnly=!allowed;
  for(const name of ['add','format','sort','deduplicate','import']){const b=q('[data-hosts-action="'+name+'"]');if(b)b.disabled=!allowed;}
  for(const name of ['enable','disable','delete']){const b=q('[data-hosts-action="'+name+'"]');if(b)b.disabled=!allowed||!ui.selected.size;}
  for(const name of ['preview','revert']){const b=q('[data-hosts-action="'+name+'"]');if(b)b.disabled=!dirty||(name==='revert'&&!allowed);}
  for(const name of ['undo','redo']){const b=q('[data-hosts-action="'+name+'"]');if(b)b.disabled=!allowed||!ui[name].length;}
  for(const e of document.querySelectorAll('[data-hosts-edit],[data-hosts-enabled]'))e.disabled=!allowed||(e.hasAttribute('data-hosts-enabled')&&e.dataset.hostsValid!=='true');
  const selection=q('#hostsSelectionCount');if(selection)selection.textContent=ui.selected.size?ui.selected.size+' selected':'';
  const summary=q('#hostsEntryCounts');if(summary)summary.textContent=checked?checked.enabled.toLocaleString()+' enabled · '+checked.disabled.toLocaleString()+' disabled':'Checking entries…';
  const issues=q('#hostsIssueButton');if(issues){issues.textContent=checked?(checked.errors+' errors · '+checked.warnings+' warnings'):'Checking…';issues.classList.toggle('he-has-errors',!!checked?.errors);}
  const count=q('#hostsLineCount');if(count)count.textContent=(checked?.lines.length||M.normalize(current.draft).split('\n').length).toLocaleString()+' lines';
  const header=q('#hostsSelectPage');if(header){const boxes=[...document.querySelectorAll('[data-hosts-select]')],chosen=boxes.filter(e=>ui.selected.has(Number(e.dataset.hostsSelect))).length;header.checked=boxes.length>0&&chosen===boxes.length;header.indeterminate=chosen>0&&chosen<boxes.length;}
  if(checked&&ui.view==='text')lineNumbers();
 }
 function lineNumbers(){const gutter=q('#hostsLineNumbers');if(gutter&&ui.view==='text'){const count=doc().lines.length;if(gutter.dataset.count!==String(count)){gutter.textContent=Array.from({length:Math.min(count,100000)},(_,n)=>n+1).join('\n');gutter.dataset.count=String(count);}gutter.scrollTop=q('#hostsSourceEditor')?.scrollTop||0;}}
 function renderIssues(){
  const root=q('#hostsIssues');if(!root)return;root.hidden=!ui.issues;if(!ui.issues)return;
  const data=doc();root.innerHTML=data.issues.length?data.issues.map(issue=>'<button type="button" data-hosts-jump="'+issue.line+'" class="he-issue '+issue.severity+'"><strong>Line '+issue.line+'</strong> '+esc(issue.message)+'</button>').join('')+((data.errors+data.warnings)>data.issues.length?'<p class="help-line">Showing the first 200 issues.</p>':''):'<p class="help-line">No validation issues. IPv4 and IPv6 mappings for the same hostname are supported.</p>';
 }
 function jump(line){
  ui.view='text';render();const editor=q('#hostsSourceEditor'),lines=doc().lines;line=Math.max(1,Math.min(lines.length,Number(line)||1));
  const start=lines.slice(0,line-1).reduce((n,text)=>n+text.length+1,0);editor.focus();editor.setSelectionRange(start,start+lines[line-1].length);editor.scrollTop=Math.max(0,(line-4)*20);q('#hostsLineNumbers').scrollTop=editor.scrollTop;
 }
 function entryDialog(line){
  if(!editable())return;
  const row=doc().rows.find(row=>row.line===Number(line)),entry=row||{address:'',hosts:[],comment:'',enabled:true};editBase={text:item().draft,line:row?.line||0};
  openModal(row?'Edit hosts entry':'Add hosts entry','<form id="hostsEntryForm" class="form-stack" autocomplete="off"><div class="field"><label for="hostsEntryAddress">IP address</label><input id="hostsEntryAddress" value="'+esc(entry.address)+'" placeholder="192.168.1.10 or 2001:db8::10" spellcheck="false"></div><div class="field"><label for="hostsEntryNames">Hostname and aliases</label><textarea id="hostsEntryNames" rows="3" placeholder="server.example.local server" spellcheck="false">'+esc(entry.hosts.join(' '))+'</textarea><span class="hint">Separate names with spaces, commas or new lines. Enter hostnames rather than URLs.</span></div><div class="field"><label for="hostsEntryComment">Comment</label><input id="hostsEntryComment" value="'+esc(entry.comment)+'"></div><label class="he-check"><input type="checkbox" id="hostsEntryEnabled"'+(entry.enabled?' checked':'')+'>Enabled</label><p class="help-line">This edits your draft. Review and save applies it to Windows.</p><p id="hostsEntryError" class="he-form-error" role="alert"></p></form>',button('cancel-modal','Cancel')+'<button type="button" class="btn primary" data-hosts-action="entry-commit">'+(row?'Update entry':'Add entry')+'</button>');
  q('#hostsEntryAddress')?.focus();
 }
 function commitEntry(){
  if(!editable()||!editBase)return;
  try{
   if(editBase.text!==item().draft)throw new Error('The draft changed while this form was open. Close it and select the entry again.');
   const entry={address:q('#hostsEntryAddress').value,hosts:q('#hostsEntryNames').value.split(/[\s,;]+/).filter(Boolean),comment:q('#hostsEntryComment').value,enabled:q('#hostsEntryEnabled').checked};
   const text=editBase.line?M.replaceLine(item().draft,editBase.line,entry):M.append(item().draft,entry);closeModal();setDraft(text);
  }catch(error){q('#hostsEntryError').textContent=error.message;}
 }
 function previewHTML(before=item().snapshot?.content||'',after=item().draft){
  const delta=M.diff(before,after),parts=[];
  for(const [mark,lines] of [['-',delta.removed],['+',delta.added]]){for(const line of lines.slice(0,120))parts.push(mark+' '+line);if(lines.length>120)parts.push(mark+' … '+(lines.length-120)+' more lines');}
  return '<p class="help-line">Changed region starts at line '+delta.start+'. '+delta.removed.length+' original lines and '+delta.added.length+' draft lines. Unchanged surrounding lines are omitted.</p><pre class="hosts-change-preview he-diff">'+esc(parts.join('\n')||'No text changes.')+'</pre>';
 }
 function confirmDraft(title,text){
  if(text===item().draft){toast('No draft changes are needed.','info');return;}
  const base=item().draft;ui.pendingDraft={base,text};openModal(title,previewHTML(base,text)+'<p class="help-line">Apply this change to the draft? Undo remains available.</p>',button('cancel-modal','Cancel')+'<button type="button" class="btn primary" data-hosts-action="draft-confirm">Update draft</button>');
 }
 function updateFindCount(){const count=q('#hostsFindCount');if(count&&ui.findOpen)count.textContent=M.matches(item().draft,ui.find,ui.matchCase,ui.whole).length.toLocaleString()+' matches';}
 function find(direction=1){
  const editor=q('#hostsSourceEditor');if(!editor)return;
  const found=M.matches(item().draft,ui.find,ui.matchCase,ui.whole),position=direction>0?editor.selectionEnd:editor.selectionStart;
  const match=direction>0?(found.find(x=>x.start>=position)||found[0]):([...(found)].reverse().find(x=>x.end<=position)||found[found.length-1]);
  q('#hostsFindCount').textContent=found.length.toLocaleString()+' matches';if(!match)return;
  editor.focus();editor.setSelectionRange(match.start,match.end);editor.scrollTop=Math.max(0,(item().draft.slice(0,match.start).split('\n').length-4)*20);q('#hostsLineNumbers').scrollTop=editor.scrollTop;
 }
 function replace(one){
  if(!editable()||!ui.find)return;
  if(!one){const result=M.replaceAll(item().draft,ui.find,ui.replace,ui.matchCase,ui.whole);setDraft(result.text);toast(result.count+' matches replaced in the draft.','info');return;}
  const editor=q('#hostsSourceEditor'),found=M.matches(item().draft,ui.find,ui.matchCase,ui.whole),match=found.find(x=>x.start>=editor.selectionStart)||found[0];if(!match)return;
  const text=item().draft.slice(0,match.start)+ui.replace+item().draft.slice(match.end);setDraft(text);const next=q('#hostsSourceEditor');next.focus();next.setSelectionRange(match.start+ui.replace.length,match.start+ui.replace.length);
 }
 function decodeImport(bytes,encoding){
  if(encoding!=='auto')return new TextDecoder(encoding,{fatal:true}).decode(bytes);
  if(bytes[0]===0xff&&bytes[1]===0xfe)return new TextDecoder('utf-16le',{fatal:true}).decode(bytes);
  if(bytes[0]===0xfe&&bytes[1]===0xff)return new TextDecoder('utf-16be',{fatal:true}).decode(bytes);
  return new TextDecoder('utf-8',{fatal:true}).decode(bytes);
 }
 async function importFile(file){
  if(!editable()||!file)return;
  if(file.size>M.LIMIT*2+2){toast('The selected file is too large for the hosts editor.','error');return;}
  try{
   const bytes=new Uint8Array(await file.arrayBuffer());if(!active()||!editable())return;
   imported={bytes,name:file.name,base:item().draft,text:null};
   openModal('Import / restore hosts text','<p>Load <strong>'+esc(file.name)+'</strong> into the draft. Saving to Windows remains a separate reviewed action.</p><div class="grid cols2"><div class="field"><label for="hostsImportEncoding">Source encoding</label><select id="hostsImportEncoding"><option value="auto">Automatic UTF-8 / UTF-16</option><option value="utf-8">UTF-8</option><option value="utf-16le">UTF-16 little endian</option><option value="utf-16be">UTF-16 big endian</option><option value="windows-1252">Windows-1252</option></select></div><div class="field"><label for="hostsImportMode">Load method</label><select id="hostsImportMode"><option value="replace">Replace draft / restore backup</option><option value="append">Append to draft</option></select></div></div><p id="hostsImportSummary" class="help-line"></p><pre id="hostsImportPreview" class="he-import-preview"></pre><p id="hostsImportError" class="he-form-error" role="alert"></p>',button('cancel-modal','Cancel')+'<button type="button" class="btn primary" data-hosts-action="import-commit">Load into draft</button>');
   importPreview();
  }catch(error){toast('Cannot read the selected file: '+error.message,'error');}
 }
 function importPreview(){
  if(!imported)return;
  const confirm=q('[data-hosts-action="import-commit"]');
  try{
   const text=M.normalize(decodeImport(imported.bytes,q('#hostsImportEncoding').value));
   if(new TextEncoder().encode(text).length>M.LIMIT)throw new Error('The decoded text exceeds the 1 MiB limit.');
   if(/[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]/.test(text))throw new Error('This is not a supported hosts text file. Choose the correct text encoding.');
   imported.text=text;const parsed=M.parse(text);q('#hostsImportSummary').textContent=parsed.rows.length+' entries · '+parsed.errors+' errors · '+parsed.warnings+' warnings. Existing comments are retained.';
   q('#hostsImportPreview').textContent=text.slice(0,12000)+(text.length>12000?'\n… Preview limited to 12,000 characters.':'');q('#hostsImportError').textContent='';confirm.disabled=false;
  }catch(error){imported.text=null;q('#hostsImportPreview').textContent='';q('#hostsImportError').textContent='Cannot decode this file. Choose its encoding. '+error.message;confirm.disabled=true;}
 }
 function exportDraft(loaded=false){
  let text=M.normalize(loaded?item().snapshot.content:item().draft);if(item().snapshot.newline==='CRLF')text=text.replace(/\n/g,'\r\n');
  download(text,loaded?'hosts-loaded-copy.txt':'hosts-draft.txt','text/plain;charset=utf-8');
 }
 function click(event){
  const view=event.target.closest('[data-hosts-view]');if(view){ui.view=view.dataset.hostsView;render();q('[data-hosts-view="'+ui.view+'"]').focus();return;}
  const edit=event.target.closest('[data-hosts-edit]');if(edit){entryDialog(edit.dataset.hostsEdit);return;}
  const go=event.target.closest('[data-hosts-jump]');if(go){jump(go.dataset.hostsJump);return;}
  const target=event.target.closest('[data-hosts-action]');if(!target||target.disabled||!active())return;
  event.preventDefault();const action=target.dataset.hostsAction;for(const menu of document.querySelectorAll('.he-more[open]'))menu.open=false;
  try{switch(action){
   case'add':entryDialog();break;
   case'entry-commit':commitEntry();break;
   case'cancel-modal':closeModal();break;
   case'undo':case'redo':travel(action);break;
   case'issues':ui.issues=!ui.issues;q('#hostsIssueButton').setAttribute('aria-expanded',ui.issues);renderIssues();break;
   case'enable':case'disable':setDraft(M.toggle(item().draft,ui.selected,action==='enable'));break;
   case'delete':confirmDraft('Delete selected entries',M.remove(item().draft,ui.selected));break;
   case'format':confirmDraft('Format '+(ui.selected.size?'selected':'all valid')+' entries',M.format(item().draft,ui.selected));break;
   case'sort':confirmDraft('Sort contiguous entry groups',M.sort(item().draft));break;
   case'deduplicate':{const result=M.deduplicate(item().draft);if(!result.count)toast('No exact active duplicates were found.','info');else confirmDraft('Remove '+result.count+' exact duplicates',result.text);break;}
   case'draft-confirm':if(!editable())break;if(ui.pendingDraft?.base!==item().draft)throw new Error('The draft changed. Review the operation again.');{const text=ui.pendingDraft.text;ui.pendingDraft=null;closeModal();setDraft(text);}break;
   case'preview':openModal('Preview hosts file changes',previewHTML(),button('cancel-modal','Close'));break;
   case'revert':confirmDraft('Revert unsaved hosts edits',item().snapshot.content||'');break;
   case'previous-page':ui.page=Math.max(0,ui.page-1);renderRows();break;
   case'next-page':ui.page++;renderRows();break;
   case'select-matching':for(const row of filtered())ui.selected.add(row.line);renderRows();break;
   case'clear-selection':ui.selected.clear();renderRows();break;
   case'find':ui.view='text';ui.findOpen=true;render();q('#hostsFind').focus();break;
   case'close-find':ui.findOpen=false;render();q('#hostsSourceEditor').focus();break;
   case'find-next':find();break;
   case'find-previous':find(-1);break;
   case'replace-one':replace(true);break;
   case'replace-all':replace(false);break;
   case'goto':openModal('Go to hosts line','<div class="field"><label for="hostsGoLine">Line number</label><input id="hostsGoLine" type="number" min="1" max="'+doc().lines.length+'" value="1"></div>',button('cancel-modal','Cancel')+'<button type="button" class="btn primary" data-hosts-action="goto-commit">Go</button>');q('#hostsGoLine').focus();q('#hostsGoLine').select();break;
   case'goto-commit':{const line=Number(q('#hostsGoLine').value);closeModal();jump(line);break;}
   case'import':q('#hostsImportFile').value='';q('#hostsImportFile').click();break;
   case'import-commit':if(!editable()||!imported||imported.text===null)break;if(imported.base!==item().draft)throw new Error('The draft changed during import. Select the file again.');{const source=imported.text,append=q('#hostsImportMode').value==='append',text=append?item().draft+(item().draft&&!item().draft.endsWith('\n')?'\n':'')+source:source;closeModal();imported=null;setDraft(text);}break;
   case'export':exportDraft();break;
   case'export-loaded':exportDraft(true);break;
  }}catch(error){toast(error.message,'error');}
 }
 function validate(text){const result=M.parse(text);if(result.errors)throw new Error(result.issues.filter(x=>x.severity==='error').slice(0,8).map(x=>'Line '+x.line+': '+x.message).join('\n'));return result.issues.filter(x=>x.severity==='warning').slice(0,8).map(x=>'Line '+x.line+': '+x.message);}
 window.NetworkToolkitHostsEditor={render,updateControls:controls,validate,previewHTML};
 document.addEventListener('click',click);
 document.addEventListener('submit',event=>{if(event.target.id==='hostsEntryForm'){event.preventDefault();commitEntry();}});
 document.addEventListener('dblclick',event=>{const row=event.target.closest('[data-hosts-line]');if(row&&!event.target.closest('button,input'))entryDialog(row.dataset.hostsLine);});
 document.addEventListener('input',event=>{
  const element=event.target;
  if(element.id==='hostsSourceEditor'){setDraft(element.value,{typing:true});return;}
  if(element.id==='hostsEntrySearch'){ui.query=element.value;ui.page=0;renderRows();return;}
  if(element.id==='hostsFind')ui.find=element.value;
  if(element.id==='hostsReplacement')ui.replace=element.value;
  if(['hostsFind','hostsReplacement'].includes(element.id))q('#hostsFindCount').textContent=M.matches(item().draft,ui.find,ui.matchCase,ui.whole).length.toLocaleString()+' matches';
 });
 document.addEventListener('change',event=>{
  const e=event.target;
  try{
   if(e.dataset.hostsSelect){e.checked?ui.selected.add(Number(e.dataset.hostsSelect)):ui.selected.delete(Number(e.dataset.hostsSelect));controls();return;}
   if(e.dataset.hostsEnabled){setDraft(M.toggle(item().draft,new Set([Number(e.dataset.hostsEnabled)]),e.checked));return;}
   if(e.id==='hostsSelectPage'){for(const row of filtered().slice(ui.page*PAGE,(ui.page+1)*PAGE))e.checked?ui.selected.add(row.line):ui.selected.delete(row.line);renderRows();return;}
   if(e.id==='hostsEntryFilter'){ui.filter=e.value;ui.page=0;renderRows();}
   if(e.id==='hostsWordWrap'){ui.wrap=e.checked;render();}
   if(e.id==='hostsFindCase'){ui.matchCase=e.checked;updateFindCount();}
   if(e.id==='hostsFindWhole'){ui.whole=e.checked;updateFindCount();}
   if(e.id==='hostsImportFile')void importFile(e.files[0]);
   if(e.id==='hostsImportEncoding')importPreview();
  }catch(error){toast(error.message,'error');}
 });
 document.addEventListener('scroll',event=>{if(event.target.id==='hostsSourceEditor'&&q('#hostsLineNumbers'))q('#hostsLineNumbers').scrollTop=event.target.scrollTop;},true);
 document.addEventListener('dragover',event=>{if(event.target.closest('#hostsConfiguration')&&event.dataTransfer?.types.includes('Files')){event.preventDefault();event.dataTransfer.dropEffect='copy';}});
 document.addEventListener('drop',event=>{if(event.target.closest('#hostsConfiguration')&&event.dataTransfer?.files.length){event.preventDefault();void importFile(event.dataTransfer.files[0]);}});
 document.addEventListener('keydown',event=>{
  if(!active())return;
  if(event.key==='Escape'){for(const menu of document.querySelectorAll('.he-more[open]'))menu.open=false;}
  if(event.target.id==='hostsFind'&&event.key==='Enter'){event.preventDefault();find(event.shiftKey?-1:1);return;}
  if(event.target.id==='hostsGoLine'&&event.key==='Enter'){event.preventDefault();const line=Number(event.target.value);closeModal();jump(line);return;}
  if(event.target.matches('[data-hosts-view]')&&['ArrowLeft','ArrowRight','Home','End'].includes(event.key)){event.preventDefault();ui.view=event.key==='Home'?'entries':event.key==='End'?'text':ui.view==='entries'?'text':'entries';render();q('[data-hosts-view="'+ui.view+'"]').focus();return;}
  if(event.key==='F3'&&ui.view==='text'){event.preventDefault();find(event.shiftKey?-1:1);return;}
  const mod=event.ctrlKey||event.metaKey;if(!mod)return;const key=event.key.toLowerCase();
  if(key==='s'&&event.target.closest('#hostsConfiguration')){event.preventDefault();doc();controls();reviewLocalConfiguration('hosts');return;}
  if(!event.target.closest('#hostsConfiguration'))return;
  if(['f','h'].includes(key)){event.preventDefault();ui.view='text';ui.findOpen=true;render();q(key==='h'?'#hostsReplacement':'#hostsFind').focus();return;}
  if(event.target.matches('input,textarea')&&event.target.id!=='hostsSourceEditor')return;
  if(key==='z'||key==='y'){event.preventDefault();travel(key==='y'||event.shiftKey?'redo':'undo');}
 });
})();
