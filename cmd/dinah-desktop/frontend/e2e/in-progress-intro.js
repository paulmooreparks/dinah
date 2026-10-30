// A layout saved before the In Progress pane existed gets the pane once, as a
// tab behind the one in front; once closed, it is not opened again.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspace: 'C:/Alpha', workspaces: ['C:/Alpha'], recent: ['C:/Alpha'] },
    app: () => {
      // The native store is kept in the page's storage, so a reload reads
      // what the last page saved.
      const seed = { version: 1, values: { 'dinah-workbench-layout': '?open=workspace,chat-browser' } };
      return {
        ReadUIState: async () => JSON.parse(localStorage.getItem('fixture-ui') || JSON.stringify(seed)),
        SaveUIState: async value => localStorage.setItem('fixture-ui', JSON.stringify(value))
      };
    },
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    p.on('console', m => { if (m.type() === 'error') console.log('console: ' + m.text()); });
    await p.goto(PAGE);
    await p.locator('[data-win="workspace"]').waitFor({ state: 'attached' });
    await p.waitForFunction(() => pudlWindows.state().open.includes('in-progress'));
    const first = await p.evaluate(() => { const s = pudlWindows.state(); return { open: s.open, left: s.docks && s.docks.left && s.docks.left.selected }; });
    ok('a layout saved before the pane existed gets it once, behind the tab in front', first.open.includes('in-progress') && first.left !== 'in-progress', JSON.stringify(first));
    await p.evaluate(() => dinahUIState.flush());
    await p.evaluate(() => pudlWindows.close('in-progress'));
    await p.waitForFunction(() => !pudlWindows.state().open.includes('in-progress'));
    await p.waitForTimeout(400);
    await p.evaluate(() => dinahUIState.flush());
    await p.goto(PAGE);
    await p.waitForFunction(() => document.readyState === 'complete' && document.documentElement.dataset.uiStateReady === 'true');
    await p.waitForTimeout(800);
    ok('once closed, the pane is not opened again', !(await p.evaluate(() => pudlWindows.state().open.includes('in-progress'))));
  } finally { await t.close(); }
})();
