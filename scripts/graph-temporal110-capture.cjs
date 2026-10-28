'use strict';
// Evidence guard only: never change the page, its timers, or its rendered state.
const same=(a,b)=>!!a&&!!b&&a.state===b.state&&a.classMutations===b.classMutations&&a.childMutations===b.childMutations;
function settled(snapshot){const frames=snapshot?.frames||[],last=frames.at(-1);return frames.length>=2&&same(frames.at(-2),last)&&last.classMutations===snapshot.classMutations&&last.childMutations===snapshot.childMutations;}
function stableCapture(before,after){
 if(!settled(before)||!after||before.classMutations!==after.classMutations||before.childMutations!==after.childMutations)return false;
 const start=before.frames.at(-1),frames=after.frames.filter(frame=>frame.t>=start.t);
 return frames.length>0&&frames.every(frame=>same(start,frame))&&same(start,after.frames.at(-1));
}
module.exports={settled,stableCapture};
