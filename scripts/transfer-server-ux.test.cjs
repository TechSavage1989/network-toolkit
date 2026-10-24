'use strict';
const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const path=require('node:path');
const server=fs.readFileSync(path.join(__dirname,'../web/assets/transfer-server.js'),'utf8');
const explorer=fs.readFileSync(path.join(__dirname,'../web/assets/transfer-explorer.js'),'utf8');
test('TFTP server startup never uses browser confirmation suppressed by the native Windows host',()=>{
 assert.doesNotMatch(server,/[^a-zA-Z]confirm\(/);
 assert.match(server,/data-transfer-confirm-start/);
 assert.match(server,/openModal\('Start '\+p\.toUpperCase\(\)\+' server'/);
 assert.match(server,/start:async function\(p,confirmed=false\)/);
});
test('SFTP and TFTP retain a visible root and status when settings are collapsed',()=>{
 assert.match(server,/data-transfer-current-root/);
 assert.match(server,/data-transfer-current-bind/);
 assert.match(server,/data-transfer-current-access/);
 assert.match(server,/data-transfer-current-active/);
 assert.doesNotMatch(server,/data-transfer-mode-root/); // Compact server UI shows the root only once in its summary
 assert.match(server,/id="transferServerFeedback"/);
 assert.match(server,/Enter an SSH username before starting/);
 assert.match(server,/Enter a session password or select/);
});
test('server settings remain open until user explicitly toggles them, across redraws',()=>{
 assert.match(explorer,/settingsVisible\?\.\(p\)/);
 assert.match(explorer,/setSettingsVisible\?\.\(p,!d.open\)/);
 assert.doesNotMatch(explorer,/d\.open=!rootPath/);
 assert.match(server,/settingsVisible:function\(p\)/);
});
