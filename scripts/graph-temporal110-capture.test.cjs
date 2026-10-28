'use strict';
const {test}=require('node:test'),assert=require('node:assert/strict'),{settled,stableCapture}=require('./graph-temporal110-capture.cjs');
const frame=(t,state=0,classMutations=0,childMutations=0)=>({t,state,classMutations,childMutations});
const snapshot=(frames,overrides={})=>({frames,classMutations:frames.at(-1).classMutations,childMutations:frames.at(-1).childMutations,...overrides});
test('capture evidence accepts unchanged preceding and intervening rendered frames',()=>{const before=snapshot([frame(0),frame(16)]),after=snapshot([frame(16),frame(32),frame(48)]);assert(settled(before));assert(stableCapture(before,after));});
test('equal endpoint states cannot hide an intermediate outline change',()=>{const before=snapshot([frame(0),frame(16)]),after=snapshot([frame(16),frame(32,1),frame(48)]);assert.equal(stableCapture(before,after),false);});
test('capture evidence rejects unsettled history and mutations between sampled animation frames',()=>{const before=snapshot([frame(0),frame(16)]);assert.equal(settled(snapshot([frame(0,1),frame(16)])),false);assert.equal(settled(snapshot([frame(0),frame(16)],{classMutations:1})),false);assert.equal(stableCapture(before,snapshot([frame(16),frame(32)],{classMutations:2})),false);assert.equal(stableCapture(before,snapshot([frame(16),frame(32)],{childMutations:1})),false);});
