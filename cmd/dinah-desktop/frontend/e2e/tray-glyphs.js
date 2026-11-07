const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 900, height: 700 } });
  try {
    const p = t.page;
    await p.evaluate(() => dinahWorkbench.open('settings'));
    await p.setViewportSize({ width: 560, height: 700 });
    await p.waitForTimeout(300);
    t.ok('tray retains individual glyph buttons without a Status summary', await p.evaluate(() => {
      const tray = document.querySelector('.status-area[data-status-compact-priority]');
      return tray && !tray.querySelector('.status-fold') && tray.closest('.win-bar').hasAttribute('data-win-bar-compact') &&
        [...tray.querySelectorAll('.status-item:not([hidden])')].every(item => item.getBoundingClientRect().width > 0 &&
          [...item.querySelectorAll('.badge')].every(badge => getComputedStyle(badge).display === 'none'));
    }));
    await t.shot('tray-glyphs-narrow');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
