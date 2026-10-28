/* One transactional color editor for app, diagnostic and terminal appearance. */
(function (root) {
  'use strict';
  const LIMITS = Object.freeze({recent:12,saved:24});
  const PALETTES = Object.freeze({
    'Toolkit dark':['#6eabff','#25d0c5','#5dd6a0','#f4c56b','#ff8790','#c3b3fa','#e6edf8','#0d1119'],
    'Toolkit light':['#1765c5','#07867e','#147c52','#936000','#c33244','#7545a0','#182638','#ffffff'],
    'Terminal':['#82b6ff','#8bdac0','#c3b3fa','#f2b9d0','#facf92','#9fcaec','#e6edf8','#151d29']
  });
  let configuration = {}, active = null, sequence = 0;
  const escape = value => String(value).replaceAll('&','&amp;').replaceAll('<','&lt;').replaceAll('>','&gt;').replaceAll('"','&quot;').replaceAll("'",'&#39;');
  function parse(value, allowAlpha = false) {
    if (typeof value !== 'string' || value.length > 80) return null;
    const text = value.trim(); let channels, hasAlpha = false;
    if (/^#[\da-f]{6}(?:[\da-f]{2})?$/i.test(text)) {
      hasAlpha = text.length === 9;
      if (hasAlpha && !allowAlpha) return null;
      channels = [1,3,5].map(i=>parseInt(text.slice(i,i+2),16));
      channels.push(hasAlpha?parseInt(text.slice(7,9),16):255);
    } else {
      const match = text.match(/^(rgb|rgba)\(\s*(\d{1,3})\s*,\s*(\d{1,3})\s*,\s*(\d{1,3})(?:\s*,\s*((?:0(?:\.\d{1,5})?|1(?:\.0{1,5})?|\.\d{1,5})))?\s*\)$/i);
      if (!match) return null;
      hasAlpha = match[1].toLowerCase() === 'rgba';
      if (hasAlpha !== (match[5] !== undefined) || (hasAlpha && !allowAlpha)) return null;
      channels = [Number(match[2]),Number(match[3]),Number(match[4]),hasAlpha?Math.round(Number(match[5])*255):255];
      if (channels.some(n=>n<0||n>255)) return null;
    }
    const [r,g,b,a] = channels, hex = '#'+channels.slice(0,hasAlpha?4:3).map(n=>n.toString(16).padStart(2,'0')).join('');
    return {r,g,b,a,hasAlpha,hex,rgb:hasAlpha?'rgba('+[r,g,b,Number((a/255).toFixed(5))].join(', ')+')':'rgb('+[r,g,b].join(', ')+')'};
  }
  function normalizePreferences(value) {
    const out = {recent:[],saved:[]};
    for (const key of Object.keys(LIMITS)) {
      if (!Array.isArray(value?.[key])) continue;
      for (const item of value[key].slice(0,256)) {
        // Stored swatches have one strict representation, not arbitrary CSS.
        if (typeof item!=='string'||!/^#[\da-f]{6}(?:[\da-f]{2})?$/i.test(item)) continue;
        const color=item.toLowerCase();
        if (!out[key].includes(color)) out[key].push(color);
        if (out[key].length===LIMITS[key]) break;
      }
    }
    return out;
  }
  function configure(value={}) { configuration={...configuration,...value}; }
  function rgbToHSV(color) {
    const c=typeof color==='string'?parse(color,true):color;if(!c)return {h:0,s:0,v:0};
    const [r,g,b]=[c.r,c.g,c.b].map(n=>n/255),max=Math.max(r,g,b),min=Math.min(r,g,b),d=max-min;
    let h=0;if(d)h=max===r?((g-b)/d)%6:max===g?(b-r)/d+2:(r-g)/d+4;
    return {h:(h*60+360)%360,s:max?d/max*100:0,v:max*100};
  }
  function hsvToHex(h,s,v,a) {
    h=((Number(h)%360)+360)%360;s=Math.max(0,Math.min(100,Number(s)))/100;v=Math.max(0,Math.min(100,Number(v)))/100;
    const c=v*s,x=c*(1-Math.abs((h/60)%2-1)),m=v-c,segments=[[c,x,0],[x,c,0],[0,c,x],[0,x,c],[x,0,c],[c,0,x]],rgb=segments[Math.floor(h/60)].map(n=>Math.round((n+m)*255));
    if(a!==undefined)rgb.push(Math.max(0,Math.min(255,Math.round(a))));
    return '#'+rgb.map(n=>n.toString(16).padStart(2,'0')).join('');
  }
  function contrastRatio(first, second) {
    const luminance=value=>{const c=parse(value);if(!c)return null;return [c.r,c.g,c.b].map(n=>{n/=255;return n<=.04045?n/12.92:((n+.055)/1.055)**2.4;}).reduce((sum,n,i)=>sum+n*[.2126,.7152,.0722][i],0);};
    const a=luminance(first),b=luminance(second);return a===null||b===null?null:(Math.max(a,b)+.05)/(Math.min(a,b)+.05);
  }
  function advisories(color, options={}) {
    const messages=[],candidate=parse(color,options.allowAlpha===true);if(!candidate)return messages;
    const ratio=contrastRatio(candidate.hex,options.background);
    if(ratio!==null&&ratio<4.5)messages.push('Low text contrast against the supplied background ('+ratio.toFixed(2)+':1). Keep a readable foreground/background pair.');
    for(const [label,value] of Object.entries(options.semanticColors||{})) {
      const peer=parse(value);if(!peer||candidate.hasAlpha)continue;
      const distance=Math.hypot(candidate.r-peer.r,candidate.g-peer.g,candidate.b-peer.b);
      if(distance<55)messages.push('This color is close to '+String(label).slice(0,60)+'. Keep warning, error and success colors distinct; retain their text labels.');
    }
    return messages;
  }
  function buttonHTML({id='',label='Color',value='#6eabff'}={}) {
    const color=parse(value,true)?.hex||'#6eabff';
    return '<button type="button"'+(id?' id="'+escape(id)+'"':'')+' class="ntk-color-trigger" aria-label="Choose '+escape(label)+'" title="Choose '+escape(label)+'"><span class="ntk-color-chip" style="--ntk-swatch:'+color+'" aria-hidden="true"></span><code>'+color+'</code></button>';
  }
  function syncButton(button,value) {
    const color=parse(value,true)?.hex;if(!button||!color)return;
    button.value=color;button.dataset.colorValue=color;
    const chip=button.querySelector('.ntk-color-chip'),code=button.querySelector('code');if(chip)chip.style.setProperty('--ntk-swatch',color);if(code)code.textContent=color;
  }
  function cancel() { active?.close(); }
  function open(options={}) {
    cancel();
    const doc=root.document;if(!doc?.body)return null;
    const allowAlpha=options.allowAlpha===true,initial=parse(options.value,allowAlpha),defaultColor=parse(options.defaultValue,allowAlpha);
    if(!initial)throw new TypeError('Color picker requires a valid '+(allowAlpha?'HEX/RGB(A)':'opaque HEX/RGB')+' color.');
    let draft=initial,reset=false,closed=false,preferences=normalizePreferences(configuration.getPreferences?.()),hsv=rgbToHSV(initial),invalid=false;
    const initialSaved=preferences.saved.slice();
    const id='ntkColor'+(++sequence),opener=doc.activeElement,dialog=doc.createElement('dialog');dialog.className='ntk-color-dialog';dialog.setAttribute('aria-labelledby',id+'Title');dialog.setAttribute('data-color-picker','');
    dialog.innerHTML='<header><h2 id="'+id+'Title">'+escape(options.title||'Choose color')+'</h2><button type="button" class="iconbtn" data-color-close aria-label="Cancel color changes">×</button></header><div class="ntk-color-body"><div class="ntk-color-preview"><div><span>Current</span><span class="ntk-color-chip" data-color-original aria-hidden="true" style="--ntk-swatch:'+initial.hex+'"></span><code>'+initial.hex+'</code></div><div><span>Preview</span><span class="ntk-color-chip" data-color-preview aria-hidden="true"></span><code data-color-preview-value></code></div></div><div class="ntk-color-visual"><div class="ntk-color-plane" data-color-plane aria-hidden="true"><span data-color-position></span></div><div class="ntk-color-sliders">'+[['h','Hue',0,359],['s','Saturation',0,100],['v','Brightness',0,100],...(allowAlpha?[['a','Opacity',0,100]]:[])].map(([key,label,min,max])=>'<label for="'+id+key+'">'+label+' <output data-color-output="'+key+'"></output><input id="'+id+key+'" type="range" min="'+min+'" max="'+max+'" step="1" data-color-channel="'+key+'"></label>').join('')+'</div></div><div class="ntk-color-values">'+[['hex',allowAlpha?'HEX / HEX alpha':'HEX'],['rgb',allowAlpha?'RGB / RGBA':'RGB']].map(([kind,label])=>'<label for="'+id+kind+'">'+label+'<span><input id="'+id+kind+'" type="text" data-color-value="'+kind+'" data-input-clear="off" spellcheck="false" autocomplete="off" maxlength="80" aria-describedby="'+id+'Error"><button type="button" class="btn small" data-color-copy="'+kind+'" aria-label="Copy '+label+'">Copy</button></span></label>').join('')+'</div><p class="ntk-color-error" id="'+id+'Error" data-color-error role="status"></p><p class="ntk-color-advice" data-color-advice></p><label class="ntk-color-palette-label" for="'+id+'Palette">Preset palette<select id="'+id+'Palette" data-color-palette>'+Object.keys(PALETTES).map(name=>'<option>'+name+'</option>').join('')+'</select></label><div class="ntk-color-swatches" data-color-presets aria-label="Preset colors"></div><section><h3>Recent colors</h3><div class="ntk-color-swatches" data-color-recent></div></section><section><div class="ntk-color-saved-heading"><h3>Saved colors</h3><button type="button" class="btn small ghost" data-color-save>Save preview color</button></div><div class="ntk-color-swatches" data-color-saved></div></section><p class="hint">Preview does not save settings. Apply keeps this color and palette edits; Cancel discards them.'+(allowAlpha?' Opacity is supported by this setting.':' This setting uses opaque colors only.')+'</p><p class="ntk-color-status" data-color-status role="status" aria-live="polite"></p></div><footer><button type="button" class="btn ghost" data-color-reset'+(!defaultColor?' disabled':'')+'>Reset to default</button><span></span><button type="button" class="btn ghost" data-color-cancel>Cancel</button><button type="button" class="btn primary" data-color-apply>Apply</button></footer>';
    const find=selector=>dialog.querySelector(selector),status=text=>{find('[data-color-status]').textContent=text;};
    function swatches(container,colors,saved=false) {
      const supported=colors.filter(value=>parse(value,allowAlpha));
      container.innerHTML=supported.length?supported.map(color=>'<span class="ntk-color-swatch-item"><button type="button" class="ntk-color-swatch" data-color-swatch="'+color+'" style="--ntk-swatch:'+color+'" title="Use '+color+'" aria-label="Use '+color+'"></button>'+(saved?'<button type="button" class="ntk-color-remove" data-color-remove="'+color+'" aria-label="Remove saved '+color+'" title="Remove saved '+color+'">×</button>':'')+'</span>').join(''):'<span class="hint">'+(colors.length?'No opaque colors available for this setting.':'None yet.')+'</span>';
    }
    function syncPalettes() {
      swatches(find('[data-color-presets]'),PALETTES[find('[data-color-palette]').value]||Object.values(PALETTES)[0]);
      swatches(find('[data-color-recent]'),preferences.recent);swatches(find('[data-color-saved]'),preferences.saved,true);
    }
    function validation(message='') {
      invalid=!!message;find('[data-color-error]').textContent=message;
      for(const input of dialog.querySelectorAll('[data-color-value]'))input.setAttribute('aria-invalid',String(invalid));
      for(const button of dialog.querySelectorAll('[data-color-apply],[data-color-save],[data-color-copy]'))button.disabled=invalid;
    }
    function sync(preserveInput) {
      find('[data-color-preview]').style.setProperty('--ntk-swatch',draft.hex);find('[data-color-preview-value]').textContent=draft.hex;
      for(const input of dialog.querySelectorAll('[data-color-value]'))if(input!==preserveInput)input.value=draft[input.dataset.colorValue];
      const values={...hsv,a:draft.a/255*100};
      for(const input of dialog.querySelectorAll('[data-color-channel]')){const key=input.dataset.colorChannel;input.value=String(Math.round(values[key]));find('[data-color-output="'+key+'"]').textContent=Math.round(values[key])+(key==='h'?'°':'%');}
      find('[data-color-plane]').style.setProperty('--ntk-hue',hsvToHex(hsv.h,100,100));
      const position=find('[data-color-position]');position.style.left=hsv.s+'%';position.style.top=(100-hsv.v)+'%';
      find('[data-color-advice]').textContent=advisories(draft.hex,options).join(' ');
    }
    function change(value,{preserveInput,fromHSV=false,isReset=false}={}) {
      const color=parse(value,allowAlpha);if(!color){validation(allowAlpha?'Use #RRGGBB, #RRGGBBAA, rgb(0–255, 0–255, 0–255) or rgba(..., 0–1).':'Use #RRGGBB or rgb(0–255, 0–255, 0–255). Transparency is not supported.');return false;}
      draft=color;reset=isReset;if(!fromHSV)hsv=rgbToHSV(draft);validation();sync(preserveInput);options.onPreview?.(draft.hex,{reset});return true;
    }
    function finish(applied) {
      if(closed)return;closed=true;if(active?.element===dialog)active=null;
      try { if(applied)options.onApply?.(draft.hex,{reset});else options.onCancel?.(initial.hex); }
      finally { if(dialog.open&&typeof dialog.close==='function')dialog.close();dialog.remove();if(opener?.isConnected)opener.focus?.({preventScroll:true}); }
    }
    const controller={element:dialog,close:()=>finish(false)};active=controller;
    // The picker is a top-layer child of body, not of the originating sidebar.
    // Keep pointer/click-away and application shortcuts from closing or acting
    // behind it. Native focus, text editing and range-key defaults still run.
    dialog.addEventListener('pointerdown',event=>event.stopPropagation());
    dialog.addEventListener('input',event=>{
      const input=event.target;
      if(input.dataset.colorValue)change(input.value,{preserveInput:input});
      if(input.dataset.colorChannel){const key=input.dataset.colorChannel;if(key!=='a')hsv[key]=Number(input.value);const alpha=allowAlpha?(key==='a'?Number(input.value)/100*255:draft.a):undefined;change(hsvToHex(hsv.h,hsv.s,hsv.v,alpha),{fromHSV:true});}
    });
    dialog.addEventListener('change',event=>{if(event.target.hasAttribute('data-color-palette'))syncPalettes();else if(event.target.dataset.colorValue&&!invalid)sync();});
    dialog.addEventListener('click',async event=>{
      event.stopPropagation();
      const button=event.target.closest('button');if(event.target===dialog){const box=dialog.getBoundingClientRect();if(event.clientX<box.left||event.clientX>box.right||event.clientY<box.top||event.clientY>box.bottom)finish(false);return;}if(!button||button.disabled)return;
      if(button.hasAttribute('data-color-cancel')||button.hasAttribute('data-color-close')){finish(false);return;}
      if(button.hasAttribute('data-color-reset')){if(defaultColor)change(defaultColor.hex,{isReset:true});return;}
      if(button.dataset.colorSwatch){change(button.dataset.colorSwatch);return;}
      if(button.dataset.colorRemove){preferences.saved=preferences.saved.filter(color=>color!==button.dataset.colorRemove);syncPalettes();find('[data-color-save]').focus();status('Saved color removed from this draft. Apply to keep the change.');return;}
      if(button.hasAttribute('data-color-save')){if(preferences.saved.includes(draft.hex)){status('This color is already saved.');return;}if(preferences.saved.length>=LIMITS.saved){status('Saved palette is full (24 colors). Remove one before adding another.');return;}preferences.saved.push(draft.hex);syncPalettes();status('Color added to this draft palette. Apply to save it.');return;}
      if(button.dataset.colorCopy){const value=draft[button.dataset.colorCopy];try{if(typeof configuration.copy==='function'){const result=await configuration.copy(value);if(result===false)throw new Error('Clipboard denied');}else{if(!root.navigator?.clipboard?.writeText)throw new Error('Clipboard unavailable');await root.navigator.clipboard.writeText(value);}if(!closed)status('Copied '+button.dataset.colorCopy.toUpperCase()+' color.');}catch{if(!closed){const input=find('[data-color-value="'+button.dataset.colorCopy+'"]');input.focus();input.select();status('Copy is unavailable. The value is selected for manual copying.');}}return;}
      if(button.hasAttribute('data-color-apply')&&!invalid){const latest=normalizePreferences(configuration.getPreferences?.()),removed=initialSaved.filter(color=>!preferences.saved.includes(color)),added=preferences.saved.filter(color=>!initialSaved.includes(color)),merged=latest.saved.filter(color=>!removed.includes(color));for(const color of added)if(!merged.includes(color))merged.push(color);if(merged.length>LIMITS.saved){status('The saved palette changed elsewhere and is now full. Cancel and reopen to review it.');return;}preferences.saved=merged;preferences.recent=[draft.hex,...latest.recent.filter(color=>color!==draft.hex)].slice(0,LIMITS.recent);try{configuration.setPreferences?.(normalizePreferences(preferences));}catch{status('Could not save the color palette. Your preview has not been applied.');return;}finish(true);}
    });
    dialog.addEventListener('keydown',event=>{event.stopPropagation();if(event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();finish(false);}});
    dialog.addEventListener('cancel',event=>{event.preventDefault();finish(false);});dialog.addEventListener('close',()=>finish(false));
    // Range inputs offer the keyboard equivalent of this pointer-only visual plane.
    const plane=find('[data-color-plane]');let pointer=null;
    function point(event){const box=plane.getBoundingClientRect();hsv.s=Math.max(0,Math.min(100,(event.clientX-box.left)/Math.max(1,box.width)*100));hsv.v=Math.max(0,Math.min(100,100-(event.clientY-box.top)/Math.max(1,box.height)*100));change(hsvToHex(hsv.h,hsv.s,hsv.v,allowAlpha?draft.a:undefined),{fromHSV:true});}
    plane.addEventListener('pointerdown',event=>{if(event.button!==0)return;event.preventDefault();pointer=event.pointerId;plane.setPointerCapture?.(pointer);point(event);});
    plane.addEventListener('pointermove',event=>{if(event.pointerId===pointer)point(event);});
    plane.addEventListener('pointerup',event=>{if(event.pointerId===pointer){plane.releasePointerCapture?.(pointer);pointer=null;}});plane.addEventListener('pointercancel',()=>{pointer=null;});
    doc.body.append(dialog);sync();syncPalettes();
    if(typeof dialog.showModal==='function')dialog.showModal();else{dialog.setAttribute('open','');dialog.setAttribute('role','dialog');dialog.setAttribute('aria-modal','true');}
    find('[data-color-value="hex"]').focus();
    return controller;
  }
  const api=Object.freeze({open,cancel,close:cancel,configure,parse,normalizePreferences,rgbToHSV,hsvToHex,contrastRatio,advisories,buttonHTML,syncButton,palettes:PALETTES,limits:LIMITS});
  root.NetworkToolkitColorPicker=api;
  if(typeof module==='object'&&module.exports)module.exports=api;
  // Registered before application-wide terminal shortcuts: Escape dismisses
  // only the foreground picker, not the appearance/tools panel underneath it.
  root.document?.addEventListener('keydown',event=>{if(active&&event.key==='Escape'){event.preventDefault();event.stopImmediatePropagation();cancel();}},true);
  root.addEventListener?.('pagehide',cancel);root.addEventListener?.('hashchange',cancel);
})(globalThis);
