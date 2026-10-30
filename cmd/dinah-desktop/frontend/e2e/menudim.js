// Disabled menu items are faded, enabled ones are not.
const { launch, PAGE } = require('./lib/page');
(async () => {
  for (const theme of ['light', 'dark']) {
    // The theme is read from storage before the page's scripts run, so it is
    // set by an init script of this page, after the fixture's stub.
    const t = await launch({
      viewport: { width: 1000, height: 600 },
      state: { workspace: 'C:\\W', recent: ['C:\\W'], approvals: 'dinah', devin: 'd', titles: {}, templates: [] },
      goto: false,
    });
    const { page: p, ok } = t;
    require('./lib/windows')(p);
    try {
      await p.addInitScript(th => { try { localStorage.setItem('pudl-theme', th); } catch (e) {} }, theme);
      await p.goto(PAGE);
      await t.waitReady();
      await p.waitForTimeout(400);
      await p.evaluate(() => dinahWorkbench.open('chat'));
      await p.locator('nav[data-menubar] .menubar-title, nav[data-menubar] [role="menuitem"]').filter({ hasText: /^Actions$/ }).first().click();
      await p.waitForTimeout(250);
      const r = await p.evaluate(() => {
        const items = [...document.querySelectorAll('.menubar-panel .menu-action')].filter(e => e.offsetParent);
        const op = e => getComputedStyle(e).opacity;
        const dis = items.filter(e => e.disabled || e.getAttribute('aria-disabled') === 'true');
        const en = items.filter(e => !(e.disabled || e.getAttribute('aria-disabled') === 'true'));
        return { disabled: dis.length, enabled: en.length, dOp: dis.map(op)[0], eOp: en.map(op)[0] };
      });
      ok(theme + ': disabled items are faded, enabled are not', r.disabled && r.enabled && r.dOp === '0.5' && r.eOp === '1', JSON.stringify(r));
      await t.shot('menudim-' + theme);
      await p.keyboard.press('Escape');
      for (const command of ['find-tool', 'all-windows', 'go-to']) {
        await p.evaluate(cmd => {
          const button = document.createElement('button'); button.dataset.cmd = cmd;
          document.body.append(button); button.click(); button.remove();
        }, command);
        await p.locator('#go-to[open] tbody tr').first().waitFor();
        const choices = await p.locator('#go-to-list tr').evaluateAll(elements => elements.length > 0 && elements.every(element => {
          const style = getComputedStyle(element);
          return element.cells.length === 2 && style.boxShadow === 'none' && style.backgroundImage === 'none' && style.marginTop === '0px';
        }));
        await p.locator('#go-to-filter').press('ArrowDown');
        const selected = await p.locator('#go-to-list [aria-selected="true"]').count();
        const frame = await p.locator('#go-to-frame').evaluate(element => { const style = getComputedStyle(element); return style.overflowY === 'auto' && element.querySelector('table').getAttribute('role') === 'grid'; });
        ok(theme + ': ' + command + ' has a two-column PUDL grid and keyboard selection', choices && frame && selected === 1);
        await p.locator('#go-to-list tr').last().click();
        ok(theme + ': row click selects without opening', await p.locator('#go-to').evaluate(element => element.open));
        await t.shot(command + '-' + theme);
        await p.locator('#go-to').getByRole('button', { name: 'Close', exact: true }).click();
      }
      await p.evaluate(() => {
        window.dinahOpenWindows = () => Array.from({ length: 40 }, (_, index) => ({ title: 'Window ' + String(index).padStart(2, '0'), detail: 'Scrollable list fixture', open() {} }));
        const command = document.createElement('button'); command.dataset.cmd = 'all-windows'; document.body.append(command); command.click(); command.remove();
      });
      await p.locator('#go-to-filter').press('ArrowDown');
      await p.keyboard.press('End');
      const scrolled = await p.locator('#go-to-frame').evaluate(element => element.scrollTop > 0 && element.scrollHeight > element.clientHeight && element.querySelector('tbody').lastElementChild.getAttribute('aria-selected') === 'true');
      ok(theme + ': a long list scrolls to the keyboard selection', scrolled);
      await t.shot('all-windows-scrolled-' + theme);
      await p.locator('#go-to').getByRole('button', { name: 'Close', exact: true }).click();
      ok(theme + ': no flat md-item buttons remain', await p.locator('button.md-item:not(.btn)').count() === 0);
    } finally { await t.close(); }
  }
})();
