const test=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const vm=require('node:vm');
const source=fs.readFileSync(path.join(__dirname,'../internal/desktop/native_bootstrap.go'),'utf8');
const template=source.match(/const nativeSessionBootstrapTemplate = `([\s\S]*?)`/)[1];
const token='ab'.repeat(32);
function script(origin,document){return template.replace('__ORIGIN__',JSON.stringify(origin)).replace('__PATH__',JSON.stringify(document)).replace('__TOKEN__',JSON.stringify(token));}
function execute({origin='http://127.0.0.1:1234',pathname='/',search='',frame=false,appPath='/'}={}){
 const window={};window.top=frame?{}:window;
 const context=vm.createContext({window,location:{origin,pathname,search}});
 vm.runInContext(script('http://127.0.0.1:1234',appPath),context);
 return window;
}
test('native capability is installed before document scripts only for the exact top-level page',()=>{
 const root=execute();assert.equal(root.__networkToolkitNativeSession,token);
 assert.deepEqual(Object.getOwnPropertyDescriptor(root,'__networkToolkitNativeSession'),{value:token,writable:false,enumerable:false,configurable:false});
 assert.equal(Object.keys(root).includes('__networkToolkitNativeSession'),false);
 const view='/terminal-view/'+'ab'.repeat(16);assert.equal(execute({pathname:view,appPath:view}).__networkToolkitNativeSession,token);
});
test('foreign pages, other workspace paths, query variants and iframes receive no capability',()=>{
 for(const fixture of [{origin:'http://127.0.0.1:1235'},{origin:'http://localhost:1234'},{origin:'https://127.0.0.1:1234'},{origin:'https://attacker.invalid'},{pathname:'/api/state'},{pathname:'/terminal-view/'+'cd'.repeat(16)},{search:'?redirect=1'},{frame:true}])assert.equal(execute(fixture).__networkToolkitNativeSession,undefined,JSON.stringify(fixture));
});
test('native bootstrap is not a generic host bridge or URL capability',()=>{
 assert(!template.includes('fetch('));assert(!template.includes('postMessage'));assert(!template.includes('location.href='));
 const windows=fs.readFileSync(path.join(__dirname,'../internal/desktop/native_bootstrap_windows.go'),'utf8');
 assert(windows.includes('comCall(w.core, 27'));assert(windows.includes('w.cfg.Post(func()'));assert(windows.includes('w.bootstrapScriptID = id'));
 const main=fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/main.go'),'utf8');
 assert(main.includes('"__SESSION_TOKEN__", httpBootstrapToken(a.token)'));
 const detached=fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/terminal_views_http.go'),'utf8');
 assert(detached.includes('"__SESSION_TOKEN__", httpBootstrapToken(token)'));
 const activation=fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/terminal_window.go'),'utf8');
 assert(activation.includes('Version: 2, Host: a.host, Token: a.windowActivationToken'));
 assert(!activation.includes('Token: a.token'));
});
