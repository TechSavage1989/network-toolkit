'use strict';
// Evidence guard only: never change the page, its timers, or its rendered state.
const same=(a,b)=>!!a&&!!b&&a.state===b.state&&a.classMutations===b.classMutations&&a.childMutations===b.childMutations;
function settled(snapshot){const frames=snapshot?.frames||[],last=frames.at(-1);return frames.length>=2&&same(frames.at(-2),last)&&last.classMutations===snapshot.classMutations&&last.childMutations===snapshot.childMutations;}
function stableCapture(before,after){
 if(!settled(before)||!after||before.classMutations!==after.classMutations||before.childMutations!==after.childMutations)return false;
 const start=before.frames.at(-1),frames=after.frames.filter(frame=>frame.t>=start.t);
 return frames.length>0&&frames.every(frame=>same(start,frame))&&same(start,after.frames.at(-1));
}
// Match terminalEventQuery's empty-event long-poll contract, not a fast poll.
function emptyEventWait(value){
 if(value==null||value==='')return 15000;
 if(!/^[+-]?\d+$/.test(value)||!Number.isSafeInteger(Number(value))||Number(value)<0||Number(value)>20000)throw new Error('Invalid synthetic terminal event wait');
 return Number(value);
}
function waitForEmptyEvents(ms,pending){return new Promise(resolve=>{const finish=value=>{clearTimeout(timer);pending.delete(cancel);resolve(value);},cancel=()=>finish(false),timer=setTimeout(()=>finish(true),ms);pending.add(cancel);});}
module.exports={settled,stableCapture,emptyEventWait,waitForEmptyEvents};
