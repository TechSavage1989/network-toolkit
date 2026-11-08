/* Large-file line indexes are updated from verified browser edit boundaries.
   This never modifies document text or persists content outside the editor. */
(function(root){'use strict';
function replace(starts,oldLength,newLength,from,to,insert){
 if(!Array.isArray(starts)||starts[0]!==0||!Number.isSafeInteger(oldLength)||!Number.isSafeInteger(newLength)||!Number.isSafeInteger(from)||!Number.isSafeInteger(to)||from<0||to<from||to>oldLength||typeof insert!=='string'||newLength!==oldLength+insert.length-(to-from))return null;
 // The source index includes the start of every line, including a trailing blank line.
 const delta=newLength-oldLength,upperBound=value=>{let lo=0,hi=starts.length;while(lo<hi){const mid=(lo+hi)>>>1;if(starts[mid]<=value)lo=mid+1;else hi=mid;}return lo;};
 const left=upperBound(from),right=upperBound(to),next=starts.slice(0,left);
 for(let i=0;i<insert.length;i++)if(insert.charCodeAt(i)===10)next.push(from+i+1);
 for(let i=right;i<starts.length;i++)next.push(starts[i]+delta);
 if(next[0]!==0||next.some((at,i)=>at>newLength||(i&&at<=next[i-1])))return null;
 return next;
}
function plan(input,revision,from,to,type,data){
 if(!Number.isSafeInteger(from)||!Number.isSafeInteger(to)||from>to)return null;
 if(!/^insert(?:Text|LineBreak|FromPaste)$|^deleteContent(?:Backward|Forward)$/.test(type||''))return null;
 return {input,revision,from,to,type,data:typeof data==='string'?data:null};
}
function update(cached,oldText,newText,pending,caret){
 if(!cached||cached.text!==oldText||!pending||pending.input!==oldText||!Number.isSafeInteger(caret))return null;
 const {from,to,type,data}=pending;
 let start=from,end=to;
 if(type==='deleteContentBackward'&&start===end){end=start;start-=oldText.length-newText.length;}
 if(type==='deleteContentForward'&&start===end)end+=oldText.length-newText.length;
 const insertLength=newText.length-oldText.length+(end-start);
 if(start<0||end>oldText.length||insertLength<0||insertLength>65536)return null;
 const insert=newText.slice(start,start+insertLength);
 if(insert.length<0||insert.length>65536||!type.startsWith('delete')&&data!==null&&insert!==data&&!(type==='insertLineBreak'&&insert==='\n'))return null;
 if(caret!==start+insert.length)return null;
 return replace(cached.starts,oldText.length,newText.length,start,end,insert);
}
const api=Object.freeze({replace,plan,update});
root.NTKEditorLargeIndex=api;
if(typeof module==='object'&&module.exports)module.exports=api;
})(typeof window==='undefined'?globalThis:window);
