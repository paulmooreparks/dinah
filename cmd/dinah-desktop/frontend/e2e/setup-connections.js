// A product that connects to services with a token has a step for each in
// the guided set-up (setup.js): how the token is made, a button that opens
// the service, and a field that saves the token and signs in with it once.
// The step can be passed, a token is never among what Set It Up is given,
// and the step in hand joins its pane.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      window.__s = { applied: [], saved: [], opened: [], token: {} };
      const conn = (id, label, hasToken) => ({ id, label, help: '', address: '', defaultAddress: 'https://' + id + '.example.com', hasToken, testable: true, tokenSteps: ['Open your profile.', 'Create a token and copy it.'], tokenStore: 'the test store' });
      return {
        State: async () => ({ workspace: 'C:/Work/Pricing', recent: [], workspaceRecords: [] }),
        Sessions: async () => [],
        SetupShown: async () => false,
        NoteSetupShown: async () => {},
        SetupOffer: async () => ({ default: 'alpha', agents: [{ id: 'alpha', name: 'Alpha', found: true, judges: true }], uses: [], connections: [conn('wiki', 'Wiki', false), conn('tracker', 'Tracker', true)],
          personName: '', accountName: 'Alex Morgan', workspace: 'C:/Work/Pricing', workspaces: 1, judgeOn: true, judgeName: 'Alpha', judgeCalls: 200 }),
        AgentCheck: async id => ({ id, status: 'connected' }),
        SetupAsking: async () => null,
        OpenConnectionSite: async (id, address) => { window.__s.opened.push(id + ' ' + address); },
        SetConnection: async (id, address, token) => { window.__s.saved.push([id, address, token]); window.__s.token[id] = token; },
        TestConnection: async id => {
          await new Promise(r => setTimeout(r, 150));
          if (window.__s.token[id] === 'bad') throw new Error('The server refused the token (401).');
          return 'Signed in to ' + (id === 'wiki' ? 'Wiki' : 'Tracker') + ' as Alex Morgan (amorgan).';
        },
        ApplySetup: async c => { window.__s.applied.push(c); },
        UndoSetup: async () => {},
      };
    },
  });
  const { page: p, ok } = t;
  try {
    const win = p.locator('.win[data-win-kind="setup"]');
    await win.waitFor({ timeout: 15000 });
    await p.waitForFunction(() => /Answers/.test(document.querySelector('.setup-agents').textContent));
    const next = win.locator('.setup-foot .btn').last();
    const status = () => win.locator('.setup-status').textContent();
    const pane = win.locator('.setup-conn:not([hidden])');
    const badge = () => pane.locator('.badge').textContent();
    ok('each connection has a step between the agent and the questions', (await win.locator('.setup-steps li').allTextContents()).join('|') === '1Agent|2Connect Wiki|3Connect Tracker|4Questions|5Review', (await win.locator('.setup-steps li').allTextContents()).join('|'));
    ok('the step’s button is Next', (await next.textContent()) === 'Next');
    await next.click();
    ok('the step in hand joins its pane', await p.evaluate(() => { const li = document.querySelector('.setup-steps li[aria-current="step"]'); return getComputedStyle(li).marginRight === '-1px' && /Connect Wiki/.test(li.textContent); }));
    ok('a connection with no token says so, and its steps start with opening it', (await badge()) === 'No token yet' && (await pane.locator('.setup-conn-steps li').allTextContents()).join('|') === 'Press Open Wiki and sign in.|Open your profile.|Create a token and copy it.', await badge());
    ok('the step can be passed, and nothing can be saved before a token is typed', await next.isEnabled() && await pane.getByRole('button', { name: 'Save and Test' }).isDisabled());
    ok('the token is typed where it cannot be read', (await pane.locator('input[type="password"]').count()) === 1 && /kept in the test store/.test(await pane.locator('.setup-conn-kept').textContent()));
    const box = async sel => JSON.stringify(await pane.locator(sel).boundingBox());
    const boxes = async () => [await box('.setup-conn-steps'), await box('.setup-conn-form'), await box('.setup-conn-kept'), JSON.stringify(await next.boundingBox())].join();
    const before = await boxes();
    await pane.getByRole('button', { name: 'Open Wiki' }).click();
    await p.waitForFunction(() => window.__s.opened.length === 1);
    ok('Open opens the service at its address', (await p.evaluate(() => window.__s.opened[0])) === 'wiki https://wiki.example.com' && /Wiki is open in your browser/.test(await status()));

    await pane.locator('input[type="password"]').fill('bad');
    await pane.getByRole('button', { name: 'Save and Test' }).click();
    await p.waitForFunction(() => /401/.test(document.querySelector('.setup-status').textContent));
    ok('a token the service refuses is said to be refused', (await badge()) === 'Not connected' && await next.isEnabled(), await badge());
    await pane.locator('input[type="password"]').fill('good');
    await pane.getByRole('button', { name: 'Save and Test' }).click();
    await p.waitForFunction(() => /Signed in to Wiki/.test(document.querySelector('.setup-conn:not([hidden]) .badge').textContent));
    ok('Save and Test saves the token and says who signed in', (await badge()) === 'Signed in to Wiki as Alex Morgan (amorgan)' && JSON.stringify(await p.evaluate(() => window.__s.saved[1])) === '["wiki","https://wiki.example.com","good"]' && (await pane.locator('input[type="password"]').inputValue()) === '', await badge());
    ok('nothing moved while the token was tested', await boxes() === before);

    await next.click();
    ok('a connection that has a token already says who is signed in', (await badge()) === 'Signed in to Tracker as Alex Morgan (amorgan)' && /Signed in to Tracker/.test(await status()), await badge() + ' / ' + await status());
    await next.click();
    for (let i = 0; i < 6 && await win.locator('.setup-question').isVisible(); i++) { await win.locator('.setup-choices .md-item').last().click(); await next.click(); }
    const review = (await win.locator('.setup-review tbody tr').allTextContents()).join(' | ');
    ok('review has a row for each connection and says Undo leaves a token', /Wiki\s*Signed in to Wiki as Alex Morgan \(amorgan\) · Token saved in step 2 · Undo does not remove it/.test(review) && /Tracker\s*Signed in to Tracker as Alex Morgan \(amorgan\) · Connected already/.test(review), review);
    await win.locator('.setup-review tbody tr', { hasText: 'Tracker' }).getByRole('button').click();
    ok('Change goes back to the connection’s step', /Connect Tracker/.test(await win.locator('.setup-steps li[aria-current="step"]').textContent()));
    await next.click(); await next.click();
    for (let i = 0; i < 6 && await win.locator('.setup-question').isVisible(); i++) await next.click();
    await next.click();
    await p.waitForFunction(() => /Everything is set/.test(document.querySelector('.setup-status').textContent));
    const sent = JSON.stringify(await p.evaluate(() => window.__s.applied));
    ok('no token is among what Set It Up is given', (await p.evaluate(() => window.__s.applied.length)) === 1 && !/good|bad|token/i.test(sent), sent);
    ok('the connection’s row keeps its words after Set It Up', /Undo does not remove it/.test(await win.locator('.setup-review').textContent()));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
