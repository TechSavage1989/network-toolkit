'use strict';
// A stand-in for the engine's text tools route (POST /api/editor-views/
// text-tools, cmd/network-toolkit/editor_text_tools.go) for browser checks that
// serve the page with synthetic API answers and no engine. It knows only the
// simple tools those checks use; the real tools are tested in Go
// (internal/texttools) and through the engine (editor-text-tools.browser.cjs).
const tools={
 upper:text=>text.toUpperCase(),
 lower:text=>text.toLowerCase(),
 'trim-trailing':text=>text.replace(/[ \t]+$/gm,''),
 'sort-asc':text=>{const end=text.endsWith('\n');const lines=(end?text.slice(0,-1):text).split('\n').sort((a,b)=>a.localeCompare(b,undefined,{numeric:true,sensitivity:'base'}));return lines.join('\n')+(end?'\n':'');},
 'base64-encode':text=>Buffer.from(text,'utf8').toString('base64'),
 'base64-decode':text=>Buffer.from(text.replace(/\s/g,''),'base64').toString('utf8'),
 'hex-encode':text=>Buffer.from(text,'utf8').toString('hex'),
 'hex-decode':text=>Buffer.from(text.replace(/[\s:,-]/g,''),'hex').toString('utf8')
};
function runTool(body){
 const fn=tools[body.tool];
 if(!fn)return {ok:false,error:'This text tool is not available in the synthetic fixture.'};
 const text=fn(body.text);
 return {ok:true,changed:text!==body.text,count:1,text};
}
// handleTextTools answers the route and reports whether it did.
async function handleTextTools(route){
 const request=route.request();
 if(new URL(request.url()).pathname!=='/api/editor-views/text-tools')return false;
 await route.fulfill({contentType:'application/json',body:JSON.stringify(runTool(request.postDataJSON()))});
 return true;
}
module.exports={handleTextTools,runTool};
