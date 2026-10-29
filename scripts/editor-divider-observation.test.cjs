'use strict';
const test=require('node:test'),assert=require('node:assert/strict'),fs=require('node:fs'),path=require('node:path'),vm=require('node:vm');
const source=fs.readFileSync(path.join(__dirname,'editor-divider113.browser.cjs'),'utf8');
const start=source.indexOf('function nativePairIssues('),end=source.indexOf('\nasync function nativeFrame',start);
assert(start>=0&&end>start);
const {issues,stable}=vm.runInNewContext(source.slice(start,end)+';({issues:nativePairIssues,stable:nativeStabilityIssues})');
function pair(){return ['left','right'].map(side=>({side,mapLine:178,firstSource:{line:179,relativeTop:-63,height:161,sourceTextMatches:true,gutter:{line:179,relativeTop:-63,height:161}}}));}
test('native alignment accepts a matching partially clipped source and gutter',()=>{assert.equal(issues(pair()).length,0);});
test('map equality cannot hide late native source or offset divergence',()=>{for(const change of [p=>p[1].firstSource.line=175,p=>p[1].firstSource.relativeTop+=40]){const p=pair();change(p);assert(issues(p).length>0);}});
test('native alignment rejects stale rendered source, wrong gutter and wrong map',()=>{for(const change of [p=>p[1].firstSource.sourceTextMatches=false,p=>p[1].firstSource.gutter.line=175,p=>p[1].firstSource.gutter.relativeTop+=20,p=>p[1].mapLine=174]){const p=pair();change(p);assert(issues(p).length>0);}});
test('native alignment requires both real source rows',()=>{assert(issues([]).length>0);const p=pair();p[0].firstSource=null;assert(issues(p).length>0);});
test('native settling rejects common-mode source drift after transient convergence',()=>{const anchor=pair(),p=pair();for(const pane of p){pane.firstSource.relativeTop-=20;pane.firstSource.gutter.relativeTop-=20;}assert.equal(issues(p).length,0);assert(stable(p,anchor).length>0);assert.equal(stable(pair(),anchor).length,0);});
test('divider acceptance observes actual frames before and after screenshots',()=>{assert.match(source,/stableNativeFrames\(page,12,'before screenshot',converged\)/);assert.match(source,/nativeStability\.afterScreenshot=await nativeGeometry\(page\)/);assert.match(source,/stableNativeFrames\(page,6,'after screenshot',nativeStability\.converged\)/);assert.match(source,/requestAnimationFrame\(resolve\)/);assert.match(source,/assert\.equal\(report\.layouts\.length,36\)/);assert.match(source,/assert\.deepEqual\(report\.violations,\[\]/);});
