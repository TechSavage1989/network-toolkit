from playwright.sync_api import sync_playwright
from pathlib import Path
import re, json
root=Path(__file__).resolve().parents[1]/'web'
html=(root/'index.html').read_text()
html=re.sub(r'<link rel="stylesheet" href="([^"]+)">',lambda m:'<style>'+(root/m[1]).read_text()+'</style>' if (root/m[1]).is_file() else '',html)
html=re.sub(r'<script src="([^"]+)"></script>',lambda m:'<script>'+(root/m[1]).read_text().replace('</script>','<\\/script>')+'</script>' if (root/m[1]).is_file() else '',html)
html=re.sub(r"const OFFLINE=location\.protocol[^;]+;","const OFFLINE=false;",html)
with sync_playwright() as p:
 browser=p.chromium.launch(headless=True,executable_path='/usr/bin/chromium',args=['--no-sandbox','--disable-dev-shm-usage','--disable-gpu'])
 page=browser.new_page(viewport={'width':1250,'height':850})
 errors=[];page.on('pageerror', lambda e: errors.append(str(e)))
 page.set_content(html,wait_until='domcontentloaded',timeout=50000)
 page.wait_for_timeout(150)
 page.evaluate('''() => {
  online=true;state.transferModes={...state.transferModes,sftp:'client'};
  api=async(path,method='GET',payload)=>{
   if(path==='/remote-browser'&&method==='GET')return {sessions:[]};
   if(path==='/remote-browser'&&method==='POST'){
    if(payload.action==='list')return {path:payload.path||'/etc',entries:[{name:'hostname',path:'/etc/hostname',size:10,directory:false}]};
    if(payload.action==='read-text')return {path:payload.path,content:'test-router',hash:'HASH123',encoding:'UTF-8'};
    return {ok:true};
   }
   if(path.startsWith('/transfer/local'))return {path:'C:\\\\',entries:[]};
   return {};
  };
  navigate('editor');
 }''')
 page.wait_for_timeout(150)
 page.evaluate('''() => {
  navigate('sftp');
  const m=NetworkToolkitRemoteBrowser.get('ssh');
  m.connected=true;
  m.sessionId='';
  m.remote.path='/etc';
  m.remote.entries=[{name:'hostname',path:'/etc/hostname',size:10,directory:false}];
  m.remote.selected=new Set();
  m.active='remote';
  NetworkToolkitRemoteBrowser.render('ssh');
 }''')
 row=page.locator('[data-rb-root="ssh"] [data-rb-item="/etc/hostname"]')
 row.wait_for(timeout=10000)
 row.dblclick()
 page.wait_for_function('''() => NetworkToolkitFileEditor.active?.path==='/etc/hostname' && document.getElementById('networkToolkitFileEditor')?.isConnected && !document.getElementById('networkToolkitFileEditor')?.hidden''',timeout=15000)
 assert not errors,errors
 print(json.dumps({'checks':['Standalone editor can be left without losing shared editor','SFTP main file-browser doubleclick opens selected remote text file','Shared editor overlay is attached and visible','No browser JavaScript exceptions'],'active':page.evaluate('NetworkToolkitFileEditor.active.path'),'errors':errors},indent=2))
 browser.close()
