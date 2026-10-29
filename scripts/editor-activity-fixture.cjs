'use strict';
// Test-only classification of metadata, never a generic mutation allowlist.
const assert=require('node:assert/strict');
const endpoint='/api/editor-views/current/activity',uidPattern=/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/;
function validateActivityBody(raw){
 assert.equal(typeof raw,'string','Activity metadata requires a JSON body');assert(Buffer.byteLength(raw,'utf8')<=16384,'Activity metadata exceeds 16 KiB');const body=JSON.parse(raw);
 assert(body&&typeof body==='object'&&!Array.isArray(body),'Activity metadata must be an object');assert.deepEqual(Object.keys(body),['documents'],'Only document-count metadata is permitted');assert(Array.isArray(body.documents)&&body.documents.length<=64,'Activity metadata accepts at most 64 documents');const seen=new Set();
 for(const row of body.documents){assert(row&&typeof row==='object'&&!Array.isArray(row),'Activity row must be an object');assert.deepEqual(Object.keys(row).sort(),['lease','loaded','uid'],'Activity rows cannot contain text, paths or credentials');assert.equal(typeof row.uid,'string');assert(uidPattern.test(row.uid),'Activity UID must be canonical UUIDv4');assert(!seen.has(row.uid),'Duplicate activity UID');seen.add(row.uid);assert(Number.isSafeInteger(row.lease)&&row.lease>=0,'Activity lease must be a safe unsigned integer');assert.equal(typeof row.loaded,'boolean','Loaded metadata must be boolean');}
 return body;
}
async function handleEditorActivityMetadata(route,origin,report){
 const request=route.request(),url=new URL(request.url());if(url.pathname!==endpoint)return false;
 assert.equal(url.origin,origin,'Metadata must stay on the exact synthetic app origin');assert.equal(url.search,'','Metadata endpoint does not take query arguments');assert.equal(url.hash,'');assert.equal(request.method(),'POST','Only the explicit metadata POST is permitted');const body=validateActivityBody(request.postData());
 if(report)(report.activityMetadata||=[]).push({path:endpoint,method:'POST',documents:body.documents.length,loaded:body.documents.filter(row=>row.loaded).length});
 await route.fulfill({status:200,contentType:'application/json',body:'{"ok":true}'});return true;
}
module.exports={validateActivityBody,handleEditorActivityMetadata};
