(function(root){
'use strict';
const relevant=tool=>['ping','monitor'].includes(tool);
function field(tool,definition,value,id){
 if(!relevant(tool)||definition.id!=='count')return null;
 const blank=value==null||String(value).trim()==='',unlimited=!blank&&Number(value)===0,remembered=Number(state.forms[tool]?.finiteCount),finite=unlimited?(Number.isInteger(remembered)&&remembered>0?remembered:definition.value):(blank?'':value);
 return '<div class="field sample-count-field"><label for="'+id+'">'+esc(definition.name)+'</label><input id="'+id+'" type="number" data-tool="'+tool+'" data-option="count" min="0" max="1000000000" value="'+esc(finite)+'" '+(unlimited?'disabled':'')+'><label class="sample-unlimited-option"><input type="checkbox" data-sample-unlimited="'+tool+'" '+(unlimited?'checked':'')+'>Run until stopped</label><span class="hint">Unlimited mode runs until Stop. Lifetime totals are kept; the latest sample rows are retained.</span></div>';
}
document.addEventListener('change',event=>{
 const input=event.target,tool=input.dataset.sampleUnlimited||input.dataset.tool;
 if(!relevant(tool)||!input.hasAttribute?.('data-sample-unlimited')&&input.dataset.option!=='count')return;
 const form=state.forms[tool],number=document.getElementById('opt-'+tool+'-count'),toggle=document.querySelector('[data-sample-unlimited="'+tool+'"]');if(!form||!number||!toggle)return;
 event.stopImmediatePropagation();
 const blank=String(number.value).trim()==='',unlimited=input===toggle?toggle.checked:!blank&&Number(number.value)===0;
 if(input!==toggle&&blank){form.count='';toggle.checked=false;number.disabled=false;saveState();updateRunButtons();return;}
 const finite=Number(number.value),fallback=Number(form.finiteCount)||TOOLS[tool].primary.find(field=>field.id==='count').value;
 if(unlimited){if(finite>0)form.finiteCount=finite;form.count=0;number.value=String(finite>0?finite:fallback);}else{form.count=finite>0?finite:fallback;if(Number.isInteger(form.count)&&form.count<=1000000000)form.finiteCount=form.count;number.value=String(form.count);}
 toggle.checked=unlimited;number.disabled=unlimited;saveState();updateRunButtons();
},true);
root.NetworkToolkitSamplingControls=Object.freeze({field});
})(window);
