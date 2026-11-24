/* Small text/confirmation dialogs use the app's modal, never the browser's
   synchronous prompt/confirm. Keep the previous dialog's actual nodes so its
   fields and listeners survive a question opened from a profile list. */
(function(root){
 'use strict';
 const document=root.document;
 let active=null,sequence=0;
 const escaped=value=>String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
 function finish(accepted,restore){
  const current=active;if(!current)return false;
  active=null;current.watch?.disconnect();
  current.form.removeEventListener('submit',current.submit);
  const result=accepted&&current.host.contains(current.form)?(current.kind==='prompt'?current.input.value:current.kind==='form'?current.value:true):current.cancelled;
  current.value=undefined;
  // A form's typed values (passwords) never stay in the page after it closes.
  if(current.kind==='form')for(const field of current.form.querySelectorAll('input,textarea'))if(field.type!=='checkbox'&&field.type!=='radio')field.value='';
  if(restore&&current.host.contains(current.form)){
   if(current.previous.length){
    current.host.replaceChildren(...current.previous);
    document.body.style.overflow='hidden';
    if(current.focus?.isConnected)current.focus.focus({preventScroll:true});
   }else root.closeModal();
  }
  current.resolve(result);return true;
 }
 function show(kind,options={}){
  const cancelled=kind==='prompt'||kind==='form'?null:false,host=document?.getElementById('modalRoot');
  // A second request never approves or replaces a pending question.
  if(active||!host||typeof root.openModal!=='function'||typeof root.closeModal!=='function')return Promise.resolve(cancelled);
  const previous=Array.from(host.childNodes),focus=document.activeElement,id='appDialog'+(++sequence);
  const message=escaped(options.message||''),input=kind==='prompt',isForm=kind==='form';
  // A form's body is the caller's own markup (fixed text and fields only).
  const body='<form id="'+id+'" data-app-dialog="'+kind+'" autocomplete="off" novalidate>'+
   (isForm?(message?'<p>'+message+'</p>':'')+String(options.body||'')+'<p class="help-line bad" data-app-dialog-error role="alert" hidden></p>':input?'<div class="field"><label for="'+id+'Value">'+message+'</label><input id="'+id+'Value" type="text" autocomplete="off" spellcheck="false"></div>':'<p>'+message+'</p>')+'</form>';
  const footer='<button class="btn ghost" type="button" data-action="close-modal">Cancel</button><button class="btn '+(options.danger?'danger':'primary')+'" type="submit" form="'+id+'">'+escaped(options.acceptLabel||(input?'Save':'Continue'))+'</button>';
  root.openModal(options.title||(input?'Enter text':'Confirm action'),body,footer);
  const form=document.getElementById(id),field=input?document.getElementById(id+'Value'):null;
  if(!form)return Promise.resolve(cancelled);
  if(field)field.value=String(options.value??'');
  return new Promise(resolve=>{
   const current={kind,host,form,input:field,previous,focus,resolve,cancelled,watch:null,submit:null};
   current.submit=event=>{event.preventDefault();if(active!==current)return;
    if(kind==='form'){const error=form.querySelector('[data-app-dialog-error]');try{current.value=options.read?options.read(form):true;}catch(problem){current.value=undefined;if(error){error.textContent=problem?.message||String(problem);error.hidden=false;}return;}}
    finish(true,true);};
   form.addEventListener('submit',current.submit);active=current;
   // Also cancel when a specialized dialog replaces the DOM directly.
   if(typeof root.MutationObserver==='function'){
    current.watch=new root.MutationObserver(()=>{if(active===current&&!host.contains(form))finish(false,false);});
    current.watch.observe(host,{childList:true,subtree:true});
   }
   if(field){field.focus();field.select();}
   else if(kind==='form'){options.ready?.(form);(form.querySelector('input:not([type=checkbox]):not([type=hidden]),textarea')||host.querySelector('.modal-foot .btn.primary'))?.focus();}
   else host.querySelector('.modal-foot [data-action="close-modal"]')?.focus();
  });
 }
 root.NetworkToolkitDialogs=Object.freeze({
  prompt:options=>show('prompt',options),confirm:options=>show('confirm',options),
  // form({title, message, body, acceptLabel, read(form), ready(form)}): read
  // returns the value, or throws an Error whose message is shown in the form.
  form:options=>show('form',options),
  close:()=>finish(false,true),replaced:()=>finish(false,false)
 });
 root.addEventListener?.('pagehide',()=>finish(false,false));
})(typeof window!=='undefined'?window:globalThis);
