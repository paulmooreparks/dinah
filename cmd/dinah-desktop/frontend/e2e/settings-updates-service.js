// Service discovery controls whether a build offers the Updates section.
const { start, settle, ledger, finish } = require('./lib/updates-fixture');
(async () => {
  for (const [name, changes, supported] of [
    ['absent', { noService: true }, false],
    ['unrelated', { services: ['documents'] }, false],
    ['failed', { discoveryError: true }, false],
    ['missing', { missingDiscovery: true }, false],
    ['present', {}, true]
  ]) {
    const t = await start(changes, false);
    const { page, ok } = t;
    try {
      // Flush discovery and rendering before asserting a hidden section.
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      ok(name + ': Updates entry follows service availability', await page.locator('#settings-tab-updates').isVisible() === supported);
      if (supported) {
        await page.locator('#settings-tab-updates').click();
        await settle(t);
        ok(name + ': supplied service renders its page', await page.locator('#settings-updates').isVisible() && (await ledger(t)).some(c => c[0] === 'Status'));
      } else {
        await page.evaluate(() => {
          const key = document.querySelector('#settings').closest('[data-win]').dataset.win;
          dinahWorkbench.setState(key, 'updates');
          document.querySelector('#settings-tab-updates').click();
        });
        ok(name + ': hidden section rejects restored addresses and clicks', await page.locator('#settings-appearance').isVisible() && !await page.locator('#settings-updates').isVisible());
        ok(name + ': no update operations run', (await ledger(t)).length === 0);
        ok(name + ': keyboard navigation skips the hidden entry', await page.evaluate(() => {
          const tab = document.querySelector('#settings-tab-packages'); tab.focus();
          tab.dispatchEvent(new KeyboardEvent('keydown', { key: 'ArrowRight', bubbles: true }));
          return document.activeElement !== document.querySelector('#settings-tab-updates');
        }));
      }
    } finally { await finish(t, 'settings-updates-service-' + name); }
  }
})();
