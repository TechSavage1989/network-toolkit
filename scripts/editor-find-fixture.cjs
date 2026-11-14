'use strict';
// A stand-in for the engine's Find route (POST /api/editor-views/find,
// cmd/network-toolkit/editor_find.go) for browser checks that serve the page
// with synthetic API answers and no engine. It knows only what those checks
// use: Find Next of plain text in page-held text (with wrap-around), Count,
// and the search history. The real search is tested in Go
// (internal/editorbuffer find_test.go) and through the engine
// (editor-find-window.browser.cjs).
const history={find:[],replace:[]};
const jobs=new Map();
let nextJob=0;
function plain(search){
 if(search?.regex)throw Error('Regular expressions are not available in the synthetic fixture.');
 return {query:String(search?.query||''),fold:!search?.matchCase};
}
function positions(text,search,from=0,to=text.length){
 const {query,fold}=plain(search),hay=fold?text.toLowerCase():text,needle=fold?query.toLowerCase():query,out=[];
 if(!needle)return out;
 for(let at=hay.indexOf(needle,from);at>=0&&at+needle.length<=to;at=hay.indexOf(needle,at+needle.length))out.push([at,at+needle.length]);
 return out;
}
const lineOf=(text,at)=>text.slice(0,at).split('\n').length-1;
function answer(body){
 if(body.action==='history')return {find:[...history.find],replace:[...history.replace]};
 if(body.action==='remember'){const list=history[body.list];if(list&&body.value){history[body.list]=[body.value,...list.filter(v=>v!==body.value)].slice(0,20);}return {ok:true};}
 const text=String(body.text||''),from=body.within?.[0]??0,to=body.within?.[1]>=0?body.within[1]:text.length;
 if(body.action==='next'){
  const hits=positions(text,body.search,from,to),at=Math.max(from,Math.min(body.at||0,to));
  let hit=body.backwards?[...hits].reverse().find(h=>h[0]<at):hits.find(h=>h[0]>=at),wrapped=false;
  if(!hit&&body.wrap&&hits.length){hit=body.backwards?hits.at(-1):hits[0];wrapped=true;}
  return hit?{found:true,from:hit[0],to:hit[1],line:lineOf(text,hit[0]),wrapped}:{found:false};
 }
 if(body.action==='matches')return {hits:positions(text,body.search,Math.max(from,body.from||0),Math.min(to,body.to??text.length)).map(([a,b])=>({from:a,to:b,line:lineOf(text,a)})),more:false};
 if(body.action==='start'&&body.kind==='count'){const id='fixture-'+(++nextJob);jobs.set(id,{state:'done',kind:'count',count:positions(text,body.search,from,to).length});return {job:id};}
 if(body.action==='status')return jobs.get(body.job)||{state:'failed',error:'Unknown search.'};
 if(body.action==='release'||body.action==='cancel'){jobs.delete(body.job);return {ok:true};}
 throw Error('This Find request is not available in the synthetic fixture.');
}
// handleFind answers the route and reports whether it did.
async function handleFind(route){
 const request=route.request();
 if(new URL(request.url()).pathname!=='/api/editor-views/find')return false;
 let status=200,body;
 try{body=answer(request.postDataJSON());}catch(error){status=409;body={error:error.message};}
 await route.fulfill({status,contentType:'application/json',body:JSON.stringify(body)});
 return true;
}
module.exports={handleFind,answer};
