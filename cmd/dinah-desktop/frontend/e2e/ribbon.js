// The ribbon (PUDL 0.79.0, a preview): off until the person turns it on with
// the topbar's toggle, then each menu title is a tab whose rows are tools.
// A tool runs its command, a glyph-only tool is named in a tooltip, a danger
// row is a danger button, and turning the toggle off brings the menus back.
// Only the menu bar's documented behaviour is checked, never the ribbon's
// own markup, which may change before PUDL marks it stable.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 860 },
    state: { workspace: 'C:/Work/parser', workspaces: ['C:/Work/parser'], recent: ['C:/Work/parser'] },
    app: () => ({
      ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {}
    }),
    ready: () => document.querySelector('[data-win="workspace"]'),
  });
  const { page: p, ok, errors } = t;
  try {
    await p.evaluate(() => window.dinahWorkbench.open('settings'));
    const toggle = p.getByRole('button', { name: 'Show as ribbon' });
    await toggle.waitFor();
    // A tool is a visible button outside the windows, the dock and the
    // taskbar, found by its name; where PUDL draws the ribbon is its own.
    const TOOLS = 'button:not(.win-layer *):not(.win-bar *):not([data-menubar] *)';
    const tool = name => p.locator(TOOLS + ':visible').filter({ hasText: new RegExp('^' + name + '$') });
    const title = name => p.locator('[data-menubar] .menubar-title').filter({ hasText: new RegExp('^' + name + '$') });
    ok('the ribbon is off until the person turns it on', (await tool('In Progress').count()) === 0);
    await toggle.click();
    await title('Documents').click();
    await tool('In Progress').first().waitFor();
    ok('turned on, a menu title is a tab whose rows are tools', true);
    await tool('In Progress').first().click();
    await p.waitForFunction(() => pudlWindows.state().open.includes('in-progress'));
    ok('a tool runs its command', true);

    await title('Window').click();
    await p.locator(TOOLS + '.btn-danger:visible').first().waitFor();
    ok('a danger row is a danger button', true);
    // A tool whose words are folded away keeps them as its name and its
    // tooltip (PUDL 0.80.0).
    const named = await p.evaluate(sel => [...document.querySelectorAll(sel)].filter(b => b.offsetWidth && getComputedStyle(b).visibility !== 'hidden' && !b.innerText.trim())
      .map(b => [b.getAttribute('aria-label') || '', b.getAttribute('data-tooltip') || '']), TOOLS);
    ok('a tool folded to its glyph keeps its name and a tooltip', named.every(n => n[0].trim() && n[1].trim()), JSON.stringify(named));

    await toggle.click();
    await p.waitForTimeout(300);
    ok('turning the ribbon off brings the menus back', (await tool('In Progress').count()) === 0 && await title('Documents').isVisible());
    await t.shot('ribbon');
  } catch (err) {
    errors.push(String(err && err.stack || err));
  } finally {
    await t.close();
  }
})();
