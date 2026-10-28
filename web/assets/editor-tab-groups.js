(function(root){
'use strict';
// Presentation membership only: document objects, file bindings and ownership
// remain in the editor. Newly opened documents always join the primary view.
function create(getDocuments){
 const groups=new Map(),selected={left:'',right:''};let activeSide='left',split=false;
 const documents=()=>getDocuments();
 const validSide=side=>side==='right'?'right':'left';
 function reconcile(){
  const docs=documents(),alive=new Set(docs.map(d=>d.uid));
  for(const uid of groups.keys())if(!alive.has(uid))groups.delete(uid);
  for(const d of docs)if(!groups.has(d.uid))groups.set(d.uid,'left');
  for(const side of ['left','right'])if(!alive.has(selected[side])||groups.get(selected[side])!==side)selected[side]=docs.find(d=>groups.get(d.uid)===side)?.uid||'';
 }
 function list(side){reconcile();return documents().filter(d=>groups.get(d.uid)===validSide(side));}
 function sideOf(uid){reconcile();return groups.get(uid)||'left';}
 function activate(uid){reconcile();if(!groups.has(uid))return false;activeSide=groups.get(uid);selected[activeSide]=uid;return true;}
 function move(uid,side){reconcile();if(!groups.has(uid))return false;side=validSide(side);groups.set(uid,side);selected[side]=uid;activeSide=side;split=true;reconcile();return true;}
 function enable(activeUID){reconcile();const first=!split;split=true;if(activeUID)activate(activeUID);if(first&&!selected.right){const candidate=list('left').find(d=>d.uid!==selected.left);if(candidate){groups.set(candidate.uid,'right');selected.right=candidate.uid;}}reconcile();}
 function single(){reconcile();const active=selected[activeSide];for(const uid of groups.keys())groups.set(uid,'left');selected.left=active||selected.left;selected.right='';activeSide='left';split=false;reconcile();}
 function choose(side,uid,{replace=false}={}){reconcile();if(!groups.has(uid))return false;side=validSide(side);const source=groups.get(uid),previous=selected[side];if(replace&&source!==side&&previous&&previous!==uid)groups.set(previous,source);groups.set(uid,side);selected[side]=uid;activeSide=side;reconcile();return true;}
 function swap(){reconcile();const left=selected.left,right=selected.right;if(!left||!right)return false;groups.set(left,'right');groups.set(right,'left');selected.left=right;selected.right=left;activeSide=activeSide==='left'?'right':'left';return true;}
 function selection(){reconcile();return Object.fromEntries(['left','right'].map(side=>[side,documents().find(d=>d.uid===selected[side])?.id||'']));}
 function snapshot(ids=documents().map(d=>d.uid)){reconcile();const allowed=new Set(ids);return {split,activeSide,left:allowed.has(selected.left)?selected.left:'',right:allowed.has(selected.right)?selected.right:'',groups:documents().filter(d=>allowed.has(d.uid)).map(d=>({uid:d.uid,side:groups.get(d.uid)}))};}
 function restore(value,{merge=false}={}){
  if(!value||!Array.isArray(value.groups))return false;reconcile();const alive=new Set(documents().map(d=>d.uid));
  if(!merge)groups.clear();for(const item of value.groups)if(alive.has(item.uid)&&['left','right'].includes(item.side))groups.set(item.uid,item.side);
  if(!merge){split=value.split===true;activeSide=validSide(value.activeSide);selected.left=String(value.left||'');selected.right=String(value.right||'');}reconcile();return true;
 }
 return Object.freeze({reconcile,list,sideOf,activate,move,enable,single,choose,swap,selection,snapshot,restore,get activeSide(){return activeSide;},get split(){return split;}});
}
const api={create};root.NTKEditorTabGroups=api;if(typeof module==='object'&&module.exports)module.exports=api;
})(globalThis);
