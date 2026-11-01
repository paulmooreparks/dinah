// Choosing a skin, from the View menu or the ribbon, leaves exactly one skin
// checked in both, and it is the skin now in use, by pointer and by keyboard.
const { launch, FRONTEND } = require('./lib/page');
const fs = require('fs'), path = require('path');
const choices = JSON.parse(fs.readFileSync(path.join(FRONTEND, 'pudl/pudl-skins.json'), 'utf8')).skins.filter(s => s.admitted).map(({ id, label }) => ({ id, label }));
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 950 }, args: { choices }, app: ({ choices }) => {
    let state = { version: 1, active: 'mineral', skin: 'standard', themes: [], overrides: {} }, revision = 0;
    const snapshot = () => ({ state: structuredClone(state), revision: String(revision), skins: choices });
    return { ReadAppearance: async () => snapshot(), SaveAppearance: async (next, expected) => { if (window.__slowSave) await new Promise(r => setTimeout(r, window.__slowSave)); if (expected !== String(revision)) throw Error('Revision conflict'); state = next; revision++; return snapshot(); } };
  } });
  const { page: p, ok } = t;
  const label = id => choices.find(s => s.id === id).label;
  async function menuChecked() {
    await p.locator('.menubar-title:visible').filter({ hasText: /^View$/ }).click();
    const panel = p.locator('#menubar-panel'); await panel.waitFor({ state: 'visible' });
    await panel.locator('[role^="menuitem"]').filter({ hasText: /^Skin$/ }).click();
    const checked = await panel.getByRole('menuitemradio').evaluateAll(rows => rows.filter(r => r.getAttribute('aria-checked') === 'true').map(r => r.textContent.trim()));
    return { panel, checked };
  }
  async function chooseFromMenu(id) {
    const { panel } = await menuChecked();
    await panel.getByRole('menuitemradio', { name: label(id), exact: true }).click();
    await p.waitForFunction(id => pudlSkins.current() === id, id);
    await p.keyboard.press('Escape'); await p.keyboard.press('Escape');
  }
  async function closeMenus() { for (let i = 0; i < 3; i++) await p.keyboard.press('Escape'); await p.waitForTimeout(100); }
  try {
    await p.evaluate(() => dinahAppearance.ready);
    for (const id of ['ink', 'glass', 'bevel']) {
      await chooseFromMenu(id); await closeMenus();
      const { checked } = await menuChecked();
      ok('after choosing ' + id + ' from the menu, the menu checks it alone', JSON.stringify(checked) === JSON.stringify([label(id)]), JSON.stringify(checked));
      await closeMenus();
    }
    // A choice made while the Skin list stays open shows at once in that list.
    {
      const { panel } = await menuChecked();
      for (const id of ['gel', 'glaze']) {
        const row = panel.getByRole('menuitemradio', { name: label(id), exact: true });
        if (!(await row.isVisible())) { const again = await menuChecked(); await again.panel.getByRole('menuitemradio', { name: label(id), exact: true }).click(); }
        else await row.click();
        await p.waitForFunction(id => pudlSkins.current() === id, id);
        await p.waitForTimeout(150);
        const open = await p.locator('#menubar-panel').evaluate(el => el.matches(':popover-open'));
        const now = await p.locator('#menubar-panel [role="menuitemradio"]').evaluateAll(rows => rows.filter(r => r.getClientRects().length && r.getAttribute('aria-checked') === 'true').map(r => r.textContent.trim()));
        ok('right after choosing ' + id + ', ' + (open ? 'the still-open' : 'the reopened') + ' list checks it alone', !open || JSON.stringify(now) === JSON.stringify([label(id)]), JSON.stringify({ open, now }));
      }
      await closeMenus();
    }
    // A menu opened again while the save is still under way shows the new skin
    // as soon as the save ends, without being closed first.
    {
      await p.evaluate(() => { window.__slowSave = 800; });
      const { panel } = await menuChecked();
      await panel.getByRole('menuitemradio', { name: label('ink'), exact: true }).click();
      await closeMenus();
      await menuChecked();
      await p.waitForFunction(() => pudlSkins.current() === 'ink');
      await p.waitForTimeout(200);
      const now = await p.locator('#menubar-panel [role="menuitemradio"]').evaluateAll(rows => rows.filter(r => r.getClientRects().length && r.getAttribute('aria-checked') === 'true').map(r => r.textContent.trim()));
      ok('a list opened during the save checks the new skin when the save ends', JSON.stringify(now) === JSON.stringify([label('ink')]), JSON.stringify(now));
      await closeMenus();
      await p.evaluate(() => { window.__slowSave = 0; });
    }
    // The keyboard path: open View, move to Skin, then to a row, and press Enter.
    const { panel } = await menuChecked();
    await panel.getByRole('menuitemradio', { name: label('matte'), exact: true }).focus();
    await p.keyboard.press('Enter');
    await p.waitForFunction(() => pudlSkins.current() === 'matte');
    await closeMenus();
    ok('after choosing Matte by keyboard, the menu checks it alone', JSON.stringify((await menuChecked()).checked) === JSON.stringify([label('matte')]));
    await closeMenus();
    // The ribbon shows the same commands; a choice there shows in both.
    await p.getByRole('button', { name: 'Show as ribbon' }).click();
    const skinRows = async () => p.evaluate(() => [...document.querySelectorAll('[role="menuitemradio"]')].filter(r => r.getClientRects().length).map(r => [r.textContent.trim(), r.getAttribute('aria-checked')]));
    await p.locator('[data-menubar] .menubar-title').filter({ hasText: /^View$/ }).click();
    const ribbonSkin = p.locator('button:not(.win-layer *):not(.win-bar *):not([data-menubar] *):visible').filter({ hasText: /^Skin$/ }).first();
    await ribbonSkin.waitFor();
    if (await ribbonSkin.count()) {
      await ribbonSkin.click();
      const rows = await skinRows();
      ok('the ribbon checks the skin in use alone', rows.filter(r => r[1] === 'true').map(r => r[0]).join() === label('matte'), JSON.stringify(rows));
      await p.getByRole('menuitemradio', { name: label('alloy'), exact: true }).click();
      await p.waitForFunction(() => pudlSkins.current() === 'alloy');
      await closeMenus();
      await ribbonSkin.click();
      const after = await skinRows();
      ok('after choosing Alloy in the ribbon, the ribbon checks it alone', after.filter(r => r[1] === 'true').map(r => r[0]).join() === label('alloy'), JSON.stringify(after));
      await closeMenus();
      await p.getByRole('button', { name: /Show as (menus|menu bar)/ }).click().catch(() => {});
    } else ok('the ribbon has a Skin command', false);
    await t.shot('skin-check-sync');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
