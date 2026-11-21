// A product whose people all do one kind of work offers no uses, and the
// guided set-up (setup.js) then leaves that question out: it is not asked,
// review has no row for it, and Set It Up opens no window. A person who has
// a workspace already is not asked about one either.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      window.__s = { applied: [] };
      return {
        State: async () => ({ workspace: '', recent: [], workspaceRecords: [] }),
        Sessions: async () => [],
        SetupShown: async () => false,
        NoteSetupShown: async () => {},
        SetupOffer: async () => ({ default: 'alpha', agents: [{ id: 'alpha', name: 'Alpha', found: true, judges: true }], uses: [], personName: '', accountName: 'Alex Morgan', workspace: 'C:/Work/Pricing', workspaces: 2, judgeOn: false, judgeName: 'Alpha', judgeCalls: 200 }),
        AgentCheck: async id => ({ id, status: 'connected' }),
        SetupAsking: async () => null,
        ApplySetup: async c => { window.__s.applied.push(c); },
      };
    },
  });
  const { page: p, ok } = t;
  try {
    const win = p.locator('.win[data-win-kind="setup"]');
    await win.waitFor({ timeout: 15000 });
    await p.waitForFunction(() => /Answers/.test(document.querySelector('.setup-agents').textContent));
    const next = win.locator('.setup-foot .btn').last();
    await next.click();
    const seen = [];
    for (let i = 0; i < 6 && await win.locator('.setup-question').isVisible(); i++) { seen.push(await win.locator('.setup-question').textContent()); await win.locator('.setup-choices .md-item').last().click(); await next.click(); }
    ok('neither what the product is used for nor a workspace is asked about', seen.length === 2 && /^What should the agents call you/.test(seen[0]), JSON.stringify(seen));
    ok('review has no row for either', !/Windows open at the start|workspace/i.test(await win.locator('.setup-review').textContent()), await win.locator('.setup-review').textContent());
    const before = await p.evaluate(() => pudlWindows.state().open.length);
    await next.click();
    await p.waitForFunction(() => /Everything is set/.test(document.querySelector('.setup-status').textContent));
    ok('Set It Up sets the answers and opens no window', (await p.evaluate(() => window.__s.applied.length)) === 1 && (await p.evaluate(() => window.__s.applied[0].folder)) === '' && (await p.evaluate(() => pudlWindows.state().open.length)) === before);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
