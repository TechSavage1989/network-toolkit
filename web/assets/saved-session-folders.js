(function(root,factory){
 'use strict';
 const api=factory();
 if(typeof module==='object'&&module.exports)module.exports=api;
 if(root)root.NetworkToolkitSessionFolders=api;
})(typeof window!=='undefined'?window:globalThis,function(){
 'use strict';
 function normalize(value){return typeof value==='string'?Array.from(value.replace(/[\u0000-\u001f\u007f]/g,'').trim()).slice(0,100).join(''):'';}
 function names(...groups){return [...new Set(groups.flatMap(rows=>Array.isArray(rows)?rows.map(row=>normalize(row?.folder)):[]).filter(Boolean))].sort((a,b)=>a.localeCompare(b));}
 function matches(item,filter){const folder=normalize(item?.folder);return !filter||(filter==='unfiled'?!folder:filter==='folder:'+folder);}
 function escape(value){return String(value).replace(/[&<>"']/g,ch=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[ch]));}
 function inputHTML(id,value,folders=[]){
  return '<div class="field"><label for="'+escape(id)+'">Folder (optional)</label><input id="'+escape(id)+'" list="'+escape(id)+'Choices" maxlength="100" value="'+escape(normalize(value))+'" placeholder="Choose or create a folder" autocomplete="off"><datalist id="'+escape(id)+'Choices">'+folders.map(name=>'<option value="'+escape(name)+'"></option>').join('')+'</datalist><p class="help-line">Leave blank for Unfiled.</p></div>';
 }
 return Object.freeze({normalize,names,matches,inputHTML});
});
