'use strict';
// Use actual visible controls in both the published topbar and compact sidebar.
async function openAppSettings(page){
 const inClosedDrawer=await page.locator('#settingsButton').evaluate(button=>button.closest('#sidebar')?.getAttribute('aria-hidden')==='true');
 if(inClosedDrawer)await page.locator('#mobileMenu').click();
 await page.locator('#settingsButton').click();
}
module.exports={openAppSettings};
