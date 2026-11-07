const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1400, height: 900 } });
  const { page: p, ok } = t;
  try {
    for (const kind of ['chat-browser', 'workspace', 'needs-you', 'in-progress', 'bus', 'skills-browser']) {
      const key = await p.evaluate(kind => dinahWorkbench.open(kind), kind);
      const win = p.locator('[data-win="' + key + '"]');
      const bar = win.locator('.ws-frozen, .browser-toolbar, .needs-filters').first();
      await bar.waitFor();
      for (const mode of ['dock', 'floating']) {
        await p.evaluate(({ key, mode }) => mode === 'dock' ? pudlWindows.dock(key, 'right') : pudlWindows.snap(key, null), { key, mode });
        const result = await bar.evaluate(bar => {
          const results = bar.parentElement.querySelector('.bus-results, .skills-results');
          let scroller = results || bar.parentElement;
          while (scroller && !/(auto|scroll)/.test(getComputedStyle(scroller).overflowY)) scroller = scroller.parentElement;
          if (!scroller) return { error: 'No scroll container' };
          const filler = document.createElement('div'); filler.style.cssText = 'height:2000px;flex:none';
          (results || bar.parentElement).appendChild(filler);
          scroller.scrollTop = 0;
          const start = bar.getBoundingClientRect().top;
          scroller.scrollTop = 500;
          const end = bar.getBoundingClientRect().top, top = scroller.getBoundingClientRect().top;
          const result = { start, end, top, scroll: scroller.scrollTop, visible: results ? Math.abs(end - start) < 1 : end >= top - 1 && end < top + 40 };
          scroller.scrollTop = 0; filler.remove(); return result;
        });
        ok(kind + ' ' + mode + ' filters remain at the top when their container scrolls', result.scroll > 0 && result.visible, JSON.stringify(result));
      }
    }
    await t.shot('pinned-filters');
  } finally { await t.close(); }
})();

