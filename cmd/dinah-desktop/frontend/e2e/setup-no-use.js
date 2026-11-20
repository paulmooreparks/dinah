// A product whose people all do one kind of work offers no uses, and the
// guided set-up (setup.js) then leaves that question out: it is not asked,
// review has no row for it, and Set It Up opens no window. A folder that is
// a workspace already is said to be put in front, since none is made of it.
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
        SetupOffer: async () => ({ default: 'alpha', agents: [{ id: 'alpha', name: 'Alpha', found: true, judges: true }], uses: [], personName: '', accountName: 'Alex Morgan', workspace: '', judgeOn: false, judgeName: 'Alpha', judgeCalls: 200 }),
        AgentCheck: async id => ({ id, status: 'connected' }),
        SetupAsking: async () => null,
        ChooseImportFolder: async () => 'C:/Work/Pricing',
        SetupFolder: async () => ({ known: true, name: 'Pricing', front: false }),
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
    const seen = [await win.locator('.setup-question').textContent()];
    await win.locator('.setup-choices .md-item').first().click();
    await win.locator('.setup-choices .md-row.active').waitFor();
    ok('a folder that is a workspace already is offered as one to put in front', (await win.locator('.setup-choices .md-item').first().textContent()) === 'Put your workspace Pricing in front');
    await next.click();
    for (let i = 0; i < 6 && await win.locator('.setup-question').isVisible(); i++) { seen.push(await win.locator('.setup-question').textContent()); await win.locator('.setup-choices .md-item').last().click(); await next.click(); }
    ok('the question about what the product is used for is not asked', seen.length === 3 && /^Which folder/.test(seen[0]), JSON.stringify(seen));
    ok('review has no row for it', !/Windows open at the start/.test(await win.locator('.setup-review').textContent()));
    ok('review says the workspace is put in front', /Workspace\s*Your workspace Pricing is put in front/.test(await win.locator('.setup-review').textContent()), await win.locator('.setup-review').textContent());
    const before = await p.evaluate(() => pudlWindows.state().open.length);
    await next.click();
    await p.waitForFunction(() => /Everything is set/.test(document.querySelector('.setup-status').textContent));
    ok('Set It Up sets the answers and opens no window', (await p.evaluate(() => window.__s.applied.length)) === 1 && (await p.evaluate(() => pudlWindows.state().open.length)) === before);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
