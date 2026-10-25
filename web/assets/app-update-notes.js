(function(root){
  'use strict';
  const escape=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
  const wrap='min-width:0;max-width:100%;overflow-wrap:anywhere;word-break:break-word;box-sizing:border-box;';
  // Release bodies are untrusted text, not HTML or an executable Markdown dialect.
  // Only presentation markers are removed. Links are always inert, visible text.
  function plainInline(value){
    return value.replace(/\*\*([^*\n]+)\*\*/g,'$1').replace(/__([^_\n]+)__/g,'$1').replace(/~~([^~\n]+)~~/g,'$1').replace(/`([^`\n]+)`/g,'$1');
  }
  function parse(notes,version){
    const source=typeof notes==='string'?notes.replace(/\r\n?/g,'\n').trim():'';
    const bullets=[],paragraphs=[],blocks=[];
    let paragraph='',inFence=false;
    const flush=()=>{if(paragraph){paragraphs.push(paragraph);blocks.push({type:'paragraph',text:paragraph});paragraph='';}};
    for(const raw of source.split('\n')){
      const line=raw.trim();
      if(/^(`{3,}|~{3,})/.test(line)){flush();inFence=!inFence;continue;}
      if(inFence){blocks.push({type:'code',text:raw});continue;}
      if(!line){flush();continue;}
      const heading=line.match(/^#{1,6}\s+(.+?)(?:\s+#+)?$/);
      if(heading){flush();blocks.push({type:'heading',text:plainInline(heading[1])});continue;}
      if(/^([-*_])(?:\s*\1){2,}$/.test(line)||/^={3,}$/.test(line)){flush();continue;}
      const bullet=line.match(/^(?:[-*+•]|\d+[.)])\s+(?:\[[ xX]\]\s+)?(.+)$/);
      const text=plainInline(bullet?bullet[1]:line);
      if(bullet){flush();bullets.push(text);blocks.push({type:'bullet',text});}
      else if(/^(?:Network Toolkit \d+\.\d+\.\d+|What['’]s new)$/i.test(text)){flush();blocks.push({type:'heading',text});}
      else paragraph+=(paragraph?' ':'')+text;
    }
    flush();
    const key=text=>text.replace(/\s+/g,' ').trim();
    const highlights=[...new Map((bullets.length?bullets:paragraphs).map(text=>[key(text),text])).values()].slice(0,4);
    // Only remove content already shown in full. Long, shortened highlights keep
    // their complete text available so no substantive release information is lost.
    const shown=new Set(highlights.filter(text=>short(text)===text).map(key));
    const remaining=blocks.filter(block=>block.type==='code'||block.type==='heading'||!shown.has(key(block.text)));
    const details=remaining.filter((block,index)=>{
      if(block.type!=='heading')return true;
      if(!highlights.length)return true;
      if(/^What['’]s new$/i.test(block.text)||block.text.toLowerCase()==='network toolkit '+String(version??'').trim().toLowerCase())return false;
      return remaining.slice(index+1).some(item=>item.type!=='heading');
    });
    return {highlights,full:details.map(block=>(block.type==='bullet'?'• ':'')+block.text).join('\n').trim()};
  }
  function short(text){
    const chars=Array.from(text);
    if(chars.length<=180)return text;
    const prefix=chars.slice(0,177).join('');
    const boundary=prefix.lastIndexOf(' ');
    return (boundary>130?prefix.slice(0,boundary):prefix).trimEnd()+'…';
  }
  function html({version,currentVersion,notes}={}){
    const parsed=parse(notes,version),current=String(currentVersion??'').trim(),next=String(version??'').trim();
    const pill=wrap+'display:inline-block;padding:3px 8px;border:1px solid var(--border,#506078);border-radius:6px;font-size:11px;line-height:1.45;font-variant-numeric:tabular-nums;';
    const versions=(current&&current!==next?'<span aria-label="Installed version" style="'+pill+'color:var(--muted,#a5afbf)">'+escape(current)+'</span><span aria-hidden="true" style="color:var(--muted,#a5afbf)">→</span>':'')+
      '<span aria-label="Release version" style="'+pill+'color:var(--text,#e8edf5);background:var(--accentBg,#18334a)">'+escape(next||'Upcoming release')+'</span>';
    return '<section class="nt-update-notes" aria-label="Release notes" style="'+wrap+'width:100%;margin:10px 0;padding:12px 14px;border:1px solid var(--border,#506078);border-left:3px solid var(--accent,#3caaf5);border-radius:9px;background:var(--panel2,#18212e);color:var(--text,#e8edf5);text-align:left">'+
      '<div class="nt-update-notes-header" style="display:flex;flex-wrap:wrap;align-items:center;justify-content:space-between;gap:8px 12px;'+wrap+'"><h4 style="margin:0;font-size:12px;line-height:1.5;font-weight:650">What\'s new</h4><div class="nt-update-notes-versions" style="display:flex;flex-wrap:wrap;align-items:center;gap:6px;'+wrap+'">'+versions+'</div></div>'+
      (parsed.highlights.length?'<ul class="nt-update-notes-highlights" style="margin:10px 0 0;padding-left:18px;font-size:12px;line-height:1.6;'+wrap+'">'+parsed.highlights.map(text=>'<li style="padding-left:2px;margin:3px 0;'+wrap+'">'+escape(short(text))+'</li>').join('')+'</ul>':
        '<p class="nt-update-notes-empty" style="margin:9px 0 0;color:var(--muted,#a5afbf);font-size:12px;line-height:1.6">Release notes are not available for this update.</p>')+
      (parsed.full?'<details class="nt-update-notes-details" style="margin-top:9px;'+wrap+'"><summary style="cursor:pointer;padding:5px 0;font-size:11px;line-height:1.5;color:var(--accent,#3caaf5);'+wrap+'">Read more</summary><div class="nt-update-notes-full" role="region" aria-label="Additional release notes" tabindex="0" style="margin-top:7px;padding:10px;border:1px solid var(--border,#506078);border-radius:6px;background:var(--panel,#111925);font-size:12px;line-height:1.65;white-space:pre-wrap;max-height:16rem;overflow:auto;overscroll-behavior:contain;'+wrap+'">'+escape(parsed.full)+'</div></details>':'')+
      '</section>';
  }
  const api={html};
  if(typeof module==='object'&&module.exports)module.exports=api;
  root.NetworkToolkitUpdateNotes=api;
})(typeof window==='object'?window:globalThis);
