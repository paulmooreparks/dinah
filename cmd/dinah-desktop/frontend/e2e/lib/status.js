// Status actions keep their own panel in the full and folded tray. Folded,
// an item is reached through PUDL's Status item (data-status-fold, PUDL
// 0.97.0), whose menu names each row as the item is named.
async function openStatus(page, id) {
 const item=page.locator('#'+id);
 if(await item.isVisible()) { await item.click(); return; }
 const name=await item.evaluate(el=>(el.getAttribute('aria-label')||el.getAttribute('title')||el.textContent).replace(/\s+/g,' ').trim());
 const open=page.locator('.menu-panel:popover-open');
 if(!await open.count()) await page.locator('.status-area > .status-fold').click();
 await page.locator('.menu-panel:popover-open .menu-action').filter({hasText:name}).first().click();
}
module.exports={openStatus};
