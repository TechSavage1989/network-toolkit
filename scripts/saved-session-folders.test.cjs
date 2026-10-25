'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),vm=require('node:vm');
const folders=require('../web/assets/saved-session-folders.js');
test('session folder labels normalize bounded Unicode names and reject structured values',()=>{
 assert.equal(folders.normalize('  Site A\n\u0000 / MDF\u007f  '),'Site A / MDF');
 assert.equal(folders.normalize({password:'not-a-label'}),'');
 assert.equal(Array.from(folders.normalize('😀'.repeat(101))).length,100);
 assert.equal(folders.normalize(undefined),'');
});
test('folder choices combine terminal and FTP profiles without changing their records',()=>{
 const ssh=[{id:'ssh',folder:'MDF'},{folder:' IDF '},{folder:'MDF'},{folder:''}],ftp=[{id:'ftp',folder:'Branch'}],before=JSON.stringify([ssh,ftp]);
 assert.deepEqual(folders.names(ssh,ftp),['Branch','IDF','MDF']);assert.equal(JSON.stringify([ssh,ftp]),before);
});
test('All folders and Unfiled do not collide with actual folder names',()=>{
 assert(folders.matches({folder:'MDF'},''));assert(folders.matches({},'unfiled'));
 assert(!folders.matches({folder:'unfiled'},'unfiled'));assert(folders.matches({folder:'unfiled'},'folder:unfiled'));
 assert(folders.matches({folder:'folder:MDF'},'folder:folder:MDF'));assert(!folders.matches({folder:'MDF'},'folder:IDF'));
});
test('folder controls escape imported labels and keep folder text inert',()=>{
 const html=folders.inputHTML('folder','<img src=x onerror=alert(1)>',['"><script>alert(1)</script>']);
 assert(!html.includes('<script>'));assert(!html.includes('<img'));assert(html.includes('&lt;img'));assert(html.includes('&quot;&gt;&lt;script&gt;'));
 assert(html.includes('maxlength="100"'));assert(html.includes('Leave blank for Unfiled'));
});
for(const kind of ['ssh','ftp'])test(kind+' file-client edits retain a saved session folder and credential reference',()=>{
 const record={id:'existing',name:'Core',type:'ssh',host:'core.test',port:22,username:'operator',folder:'Branch',credentialId:'vault-reference'};
 const state={terminalProfiles:kind==='ssh'?[record]:[],ftpProfiles:kind==='ftp'?[record]:[]},writes=[];
 const c={state,document:{addEventListener(){},querySelector(){return null},getElementById(){return null}},esc:String,icon:()=>'',saveState:()=>writes.push(JSON.stringify(state)),console,NetworkToolkitSessionFolders:folders};
 c.window=c;vm.createContext(c);vm.runInContext(fs.readFileSync(require.resolve('../web/assets/remote-browser.js'),'utf8'),c);
 const remote=c.NetworkToolkitRemoteBrowser;Object.assign(remote.configurations[kind],record,{password:'transient-secret'});remote.get(kind).profileId='existing';
 if(kind==='ssh')remote.saveSharedSSHProfile('Updated');else remote.saveFTPProfile('Updated');
 const profile=state[kind==='ssh'?'terminalProfiles':'ftpProfiles'][0];assert.equal(profile.id,'existing');assert.equal(profile.name,'Updated');assert.equal(profile.folder,'Branch');assert.equal(profile.credentialId,'vault-reference');assert(!JSON.stringify(state).includes('transient-secret'));assert(writes.length>0);
});
