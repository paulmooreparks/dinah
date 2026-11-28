// The two promises a check waits on in place of a length of time
// (docs/CONTRIBUTING.md, Writing browser checks): dinahCommands.idle() and
// dinahRender.idle(), and t.settled(), which waits for both at once. The
// render queue's own cases are in render-scheduler.js.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ ready: 'workbench' });
  const { page: p, ok } = t;
  try {
    await t.settled();
    ok('a settled page has no redraw waiting and nothing to draw', await p.evaluate(() => { const d = dinahRender.diagnostics(); return !d.framePending && d.pending === d.blocked; }));
    const order = await p.evaluate(async () => {
      const seen = [];
      await dinahCommands.idle(); seen.push('idle at once');
      const heard = () => seen.push('redraw');
      document.addEventListener('dinah:commands-change', heard);
      dinahCommands.changed();
      const waiting = dinahCommands.idle().then(() => seen.push('idle after'));
      await Promise.resolve(); await Promise.resolve();
      seen.push('still waiting');
      await waiting;
      document.removeEventListener('dinah:commands-change', heard);
      return seen;
    });
    ok('the registry is idle at once with no redraw waiting, and after the redraw when one is', JSON.stringify(order) === JSON.stringify(['idle at once', 'still waiting', 'redraw', 'idle after']), JSON.stringify(order));
    // A registered command's state changes and the registry is told. The
    // menu row is read after settled() with no other wait.
    await p.evaluate(() => {
      window.__fixtureOn = false;
      dinahCommands.register({ id: 'fixture-settled', menu: 'view', label: 'Fixture settled', enabled: () => window.__fixtureOn, run: () => {} });
    });
    await t.settled();
    const row = () => p.evaluate(() => { const r = dinahCommands.rows('view').find(r => r !== '-' && r.label === 'Fixture settled'); return r ? !r.disabled : null; });
    ok('the new command is drawn disabled once the page has settled', await row() === false);
    await p.evaluate(() => { window.__fixtureOn = true; dinahCommands.changed(); });
    await t.settled();
    ok('and enabled once its change has settled', await row() === true);
    const read = await t.until(() => p.evaluate(() => 1 + 1), value => value === 2);
    const never = await t.until(() => p.evaluate(() => 1), value => value === 2, 200);
    ok('until returns the value that passed, and the last value when the time is up', read === 2 && never === 1);
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
