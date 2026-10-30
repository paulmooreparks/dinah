// The menu bar's titles carry glyphs (PUDL 0.79.0): a wide window shows each
// title's glyph and words, a narrower one folds the titles to their glyphs,
// each keeping its name for a screen reader and its tooltip, and only a
// very narrow one collapses the bar to its one menu button.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 860 },
    state: { workspace: 'C:/Work/parser', workspaces: ['C:/Work/parser'], recent: ['C:/Work/parser'] },
    app: () => ({
      ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {}
    }),
  });
  const { page: p, ok } = t;
  try {
    await p.locator('[data-win="workspace"]').waitFor({ state: 'attached' });
    const names = ['Documents', 'View', 'Tools', 'Window', 'Help'];
    const titles = () => p.evaluate(names => names.map(n => {
      const b = [...document.querySelectorAll('.menubar-title')].find(t => (t.getAttribute('aria-label') || t.textContent).trim().startsWith(n) || (t.title || '').startsWith(n));
      if (!b) return { n, found: false };
      const words = b.querySelector('.menubar-words'), glyph = b.querySelector('.glyph');
      return { n, found: true, glyph: !!glyph && getComputedStyle(glyph).display !== 'none', words: !!words && getComputedStyle(words).display !== 'none' && words.getBoundingClientRect().width > 0,
        name: b.getAttribute('aria-label') || b.textContent.trim(), tip: b.getAttribute('title') || b.getAttribute('data-tooltip') || '' };
    }), names);
    await p.waitForTimeout(500);
    const wide = await titles();
    ok('a wide window shows every title with its glyph and its words', wide.every(t => t.found && t.glyph && t.words), JSON.stringify(wide));
    await p.setViewportSize({ width: 480, height: 860 });
    await p.waitForFunction(() => document.querySelector('.menubar-title-glyphs'));
    const narrow = await titles();
    ok('a narrower window folds the titles to their glyphs, every menu still in the bar', narrow.every(t => t.found && t.glyph && !t.words), JSON.stringify(narrow));
    ok('a folded title keeps its name and its tooltip', narrow.every(t => t.name.startsWith(t.n) && t.tip.startsWith(t.n)), JSON.stringify(narrow.map(t => [t.name, t.tip])));
    await p.setViewportSize({ width: 260, height: 860 });
    await p.waitForFunction(() => document.querySelector('.menubar-collapsed'));
    const collapsed = await p.evaluate(() => [...document.querySelectorAll('.menubar-title')].filter(t => t.getBoundingClientRect().width > 0).length);
    ok('only a very narrow window collapses the bar to its one button', collapsed <= 2, String(collapsed));
  } finally { await t.close(); }
})();
