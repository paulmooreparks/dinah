// Status actions keep their own panel in the full and folded tray.
async function openStatus(page, id) {
 const item=page.locator('#'+id);
 if(await item.isVisible()) { await item.click(); return; }
 const folded=page.locator('#status-folded');
 const menu=page.locator('#status-tray-menu');
 if(!await menu.evaluate(el=>el.matches(':popover-open'))) await folded.click();
 await menu.locator('[data-status-for="'+id+'"]').click();
}
module.exports={openStatus};
