// Host command entry points must enforce current state, even after a row was drawn.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    app: () => ({ ChooseWorkspace: async () => { window.__chosen++; return window.__state; } }),
    init: () => { window.__chosen = 0; }
  });
  const { page: p, ok } = t;
  try {
    await p.waitForFunction(() => window.dinahCommands && !document.body.hasAttribute('aria-busy'));
    const stale = await p.evaluate(async () => {
      const c = dinahCommands, app = go.main.App;
      const choose = app.ChooseWorkspace;
      const search = c.searchable().find(row => row.id === 'new-workspace');
      const delegate = c.describe('workspace');
      app.ChooseWorkspace = null;
      const aliasRefused = c.run('workspace') === false;
      search.run(); delegate.run();
      // The button still looks enabled; dispatch must consult current state.
      const button = document.querySelector('[data-cmd="new-workspace"]');
      button.disabled = false; button.removeAttribute('aria-disabled'); button.click();
      await Promise.resolve(); await Promise.resolve();
      app.ChooseWorkspace = choose;
      c.changed();
      return aliasRefused && window.__chosen === 0;
    });
    ok('stale search, alias, delegate and button cannot run a disabled command', stale);
    ok('a menu row rechecks state after it was drawn', await p.evaluate(async () => {
      let allowed = true, calls = 0;
      dinahCommands.register({ id: 'fixture-stale-menu', menu: 'view', label: 'Fixture stale menu', enabled: () => allowed, run: () => { calls++; } });
      const row = dinahCommands.rows('view').find(entry => entry.label === 'Fixture stale menu');
      allowed = false;
      row.run();
      await Promise.resolve(); await Promise.resolve();
      return !row.disabled && calls === 0;
    }));

    await p.evaluate(() => dinahCommands.run('go-to'));
    await p.locator('#go-to[open]').waitFor();
    ok('host dispatch refuses commands while Find is modal', await p.evaluate(() => dinahCommands.run('workspace') === false));
    await p.locator('#go-to-filter').fill('> new workspace');
    await p.locator('#go-to-list a').filter({ hasText: 'New Workspace…' }).click();
    await p.waitForFunction(() => window.__chosen === 1);
    ok('Find closes its dialog before executing the selected host command', !(await p.locator('#go-to').evaluate(node => node.open)));

    await p.evaluate(() => {
      const control = document.querySelector('[data-cmd="new-workspace"]');
      control.click();
    });
    await p.waitForFunction(() => window.__chosen === 2);
    await p.evaluate(() => dinahCommands.run('workspace'));
    await p.waitForFunction(() => window.__chosen === 3);
    ok('button, search and legacy alias use the same workspace action', true);

    await p.evaluate(() => dinahCommands.run('go-to'));
    await p.locator('#go-to[open]').waitFor();
    const guarded = await p.evaluate(async () => {
      const button = document.querySelector('[data-cmd="new-workspace"]');
      button.click();
      await Promise.resolve(); await Promise.resolve();
      return window.__chosen === 3;
    });
    ok('synthetic host clicks cannot bypass the modal guard', guarded);
    await p.keyboard.press('Escape');
    ok('host actions are not inherited into the agent command surface', await p.evaluate(() => !pudlCommands.list(document.body).some(command => command.id === 'new-workspace')));
  } catch (error) {
    t.errors.push(String(error.stack || error));
  } finally { await t.close(); }
})();
