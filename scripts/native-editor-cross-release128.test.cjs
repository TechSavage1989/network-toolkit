'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path');
const source=fs.readFileSync(path.join(__dirname,'native-editor-drag.ps1'),'utf8');
test('real Cross drag settles final pointer then mouse-up before delayed Alt-up, with bounded Cross-only waits',()=>{
 const move=source.indexOf('for($i=1;'),settle=source.indexOf('Start-Sleep -Milliseconds 120'),up=source.indexOf('::mouse_event(0x0004'),hold=source.indexOf("try{if($Mode -eq 'Cross'){Start-Sleep -Milliseconds 150}}"),altUp=source.indexOf('::keybd_event(18,0,2');
 assert.ok(move>=0&&move<settle&&settle<up&&up<hold&&hold<altUp);
 assert.match(source.slice(move,up),/if\(\$Mode -eq 'Cross'\)\{\s+Start-Sleep -Milliseconds 120\s+\$inputState\.targetClientX=/);
 assert.match(source,/\$inputState\.altBeforeMouseUp=\(\[EditorDragInput\]::GetAsyncKeyState\(18\) -lt 0\)/);
 assert.match(source,/finally\{if\(\$alt\)\{\$inputState\.altUpAt=.*::keybd_event\(18,0,2/);
 assert.equal((source.match(/::mouse_event\(0x0002/g)||[]).length,1);assert.equal((source.match(/::mouse_event\(0x0004/g)||[]).length,1);
 assert.doesNotMatch(source,/dispatchEvent|SendKeys|claimDrag/);
});
test('ordinary and cancellation gestures retain original semantics and native identity checks',()=>{
 assert.match(source,/\$alt=\$Mode -eq 'Cross' -or \$Mode -eq 'OutsideCancel'/);
 assert.match(source,/if\(\$Mode -eq 'Escape'\)\{\[EditorDragInput\]::keybd_event\(27,0,0/);
 assert.match(source,/Visible-Point \$owner \$from/);assert.match(source,/if\(\$Mode -eq 'Cross'\)\{Visible-Point \$target \$to\}/);
 assert.match(source,/fixture executable identity mismatch/);assert.match(source,/inputState=\$inputState/);
});
test('existing native fixture still requires actual pending transfer, frozen source and destination ACK',()=>{
 const go=fs.readFileSync(path.join(__dirname,'../cmd/network-toolkit/editor_native_windows_test.go'),'utf8');
 assert.match(go,/real backend never recorded pending transfer/);assert.match(go,/source lost\/freed\/editable document before destination ACK/);
 assert.match(go,/actual Alt-mouse drag between two production WebViews/);
 // Hosted proof must compare final trusted receiver point against recorded
 // mouse-up/Alt-up and register -> claim -> transfer. No automatic retry here.
 const trace=fs.readFileSync(path.join(__dirname,'editor-native-trace128.js'),'utf8');
 assert.match(trace,/trusted:e\.isTrusted===true/);assert.match(trace,/effect:effects\.has/);assert.match(trace,/x:Number\.isFinite/);
});

function assertCrossReleaseProof(report){
 assert.equal(report.passed,true);
 const input=report.tabGestureEvidence.find(e=>e.mode==='Cross').inputState;
 assert.equal(input.sourceForeground,true);assert.equal(input.altDown,true);assert.equal(input.altBeforeMouseUp,true);
 assert.ok(input.altUpAt-input.mouseUpAt>=140);
 const events=report.diagnostic128Views.find(v=>v.view==='main').events.events;
 const final=events.find(e=>e.type==='dragover'&&e.trusted&&e.alt&&e.effect==='move'&&Math.abs(e.x-input.targetClientX)<=1&&Math.abs(e.y-input.targetClientY)<=1&&e.at<=input.mouseUpAt&&e.at>=input.mouseUpAt-250);
 assert.ok(final,'receiver must observe final trusted point with move before mouse-up');
 const claim=report.diagnostic128Broker.find(e=>e.action==='claim'&&e.status===200&&e.at>=input.mouseUpAt);
 assert.ok(claim,'successful actual claim must follow mouse-up');
 assert.ok(report.diagnostic128Broker.some(e=>e.action==='transfer'&&e.status===200&&e.at>=claim.at));
}
test('hosted proof rejects cancellation and missing final receiver point even if a summary says passed',()=>{
 const r={passed:true,tabGestureEvidence:[{mode:'Cross',inputState:{sourceForeground:true,altDown:true,altBeforeMouseUp:true,targetClientX:30,targetClientY:165,mouseUpAt:1000,altUpAt:1150}}],diagnostic128Views:[{view:'main',events:{events:[{type:'dragover',trusted:true,alt:true,effect:'move',x:30,y:165,at:950}]}}],diagnostic128Broker:[{action:'claim',status:200,at:1010},{action:'transfer',status:200,at:1080}]};
 assertCrossReleaseProof(r);r.diagnostic128Views[0].events.events[0].x=52;assert.throws(()=>assertCrossReleaseProof(r),/final trusted point/);r.diagnostic128Views[0].events.events[0].x=30;r.diagnostic128Broker[0].action='cancel';assert.throws(()=>assertCrossReleaseProof(r),/actual claim/);
});
module.exports={assertCrossReleaseProof};
