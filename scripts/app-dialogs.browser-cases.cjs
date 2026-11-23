'use strict';
const assert=require('node:assert/strict');
// Called by app-interaction.browser.cjs against the complete shipped page.
// The outer fixture rejects every unexpected browser dialog event.
module.exports=async function checkDialogs(page){
 const checks=[],form=page.locator('[data-app-dialog]');
 const result=async expected=>{
  await page.waitForFunction(()=>window.__appDialogAnswer?.done===true);
  assert.deepEqual(await page.evaluate(()=>window.__appDialogAnswer.value),expected);
 };
 const ask=async(kind,options)=>page.evaluate(({kind,options})=>{
  window.__appDialogAnswer={done:false};
  NetworkToolkitDialogs[kind](options).then(value=>{window.__appDialogAnswer={done:true,value};});
 },{kind,options});

 await page.evaluate(()=>{
  closeModal();
  openModal('Saved profiles','<label for="appDialogDraft">Draft</label><input id="appDialogDraft"><button id="appDialogRename" type="button">Rename</button>');
  window.__appDialogPrevious=document.querySelector('#modalRoot').firstElementChild;
  window.__appDialogDraft=document.querySelector('#appDialogDraft');
  window.__appDialogClicks=0;
  document.querySelector('#appDialogRename').addEventListener('click',()=>window.__appDialogClicks++);
 });
 await page.locator('#appDialogDraft').fill('Keep this unsaved text');
 await page.locator('#appDialogRename').focus();
 await ask('prompt',{title:'Rename profile',message:'Profile <name>:',value:'Branch router',acceptLabel:'Save'});
 await form.waitFor({state:'visible'});
 assert.equal(await form.locator('label').innerText(),'Profile <name>:');
 assert.equal(await form.locator('input').inputValue(),'Branch router');
 assert(await form.locator('input').evaluate(node=>node===document.activeElement));
 await form.locator('input').fill('Branch north & south');
 await page.keyboard.press('Enter');await result('Branch north & south');
 assert(await page.evaluate(()=>document.querySelector('#modalRoot').firstElementChild===__appDialogPrevious&&document.querySelector('#appDialogDraft')===__appDialogDraft));
 assert.equal(await page.locator('#appDialogDraft').inputValue(),'Keep this unsaved text');
 assert(await page.locator('#appDialogRename').evaluate(node=>node===document.activeElement));
 await page.locator('#appDialogRename').click();assert.equal(await page.evaluate(()=>__appDialogClicks),1);
 checks.push('App text dialogs preserve literal text and restore the same parent fields, listeners and focus');

 for(const close of ['Cancel','Escape','backdrop']){
  await ask('confirm',{title:'Remove saved profile',message:'Remove this saved entry?',acceptLabel:'Remove',danger:true});
  await form.waitFor({state:'visible'});
  assert.equal(await page.evaluate(()=>document.activeElement.textContent),'Cancel');
  // Tab stays within the app dialog, including its own footer controls.
  for(let i=0;i<5;i++){
   await page.keyboard.press('Tab');
   assert(await page.evaluate(()=>!!document.activeElement.closest('#modalRoot .modal')));
  }
  if(close==='Cancel')await page.locator('#modalRoot .modal-foot [data-action="close-modal"]').click();
  else if(close==='Escape')await page.keyboard.press('Escape');
  else await page.locator('#modalRoot .modal-backdrop').click({position:{x:2,y:2}});
  await result(false);
  assert.equal(await page.locator('#appDialogDraft').inputValue(),'Keep this unsaved text');
 }
 checks.push('Cancel, Escape and backdrop cancel without approving; dialog focus remains contained');

 await ask('confirm',{title:'Confirm removal',message:'Remove the selected saved entry?',acceptLabel:'Remove',danger:true});
 await page.locator('#modalRoot .modal-foot .btn.danger').click();await result(true);
 await ask('prompt',{title:'New name',message:'Name:',value:'Original'});
 await page.evaluate(()=>openModal('Replacement operation','<p id="appDialogReplacement">Keep this new dialog</p>'));
 await result(null);assert(await page.locator('#appDialogReplacement').isVisible());
 assert.equal(await page.locator('#appDialogDraft').count(),0);
 checks.push('Only an explicit accept approves; replacing a question cancels it without restoring an obsolete dialog');

 await page.evaluate(()=>closeModal());
 await ask('prompt',{title:'Pending name',message:'Name:',value:'First'});
 assert.equal(await page.evaluate(()=>NetworkToolkitDialogs.confirm({message:'A second question?'})),false);
 assert.equal(await form.locator('input').inputValue(),'First');
 await page.keyboard.press('Escape');await result(null);
 assert.equal(await page.locator('#modalRoot .modal').count(),0);
 checks.push('Concurrent requests are declined without replacing or approving the pending question');
 await page.evaluate(()=>{for(const key of ['__appDialogAnswer','__appDialogPrevious','__appDialogDraft','__appDialogClicks'])delete window[key];});
 return checks;
};
