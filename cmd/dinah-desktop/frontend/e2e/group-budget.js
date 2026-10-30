// A group chat's turn budget: shown in the group's window only when the group has
// one, and set from its Turn Budget… control, with Unlimited the first choice.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      window.__calls = [];
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' };
      const bus = { rooms: [
        { name: 'open-group', title: 'Open Group', lastAt: '', unread: 0, members: [docs, person], budget: -1, used: 0 },
        { name: 'counted-group', title: 'Counted Group', lastAt: '', unread: 0, members: [docs, person], budget: 5, used: 2 },
        { name: 'spent-group', title: 'Spent Group', lastAt: '', unread: 0, members: [docs, person], budget: 3, used: 3 }] };
      window.__bus = bus;
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'] }),
        AgentProviders: async () => [], Sessions: async () => [], SessionsAt: async () => [],
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: bus.rooms, workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1, noTurnBudget: -1 }),
        BusGroupMessages: async () => [],
        BusMarkGroupRead: async () => {},
        BusGroupBudget: async (key, n) => {
          window.__calls.push(['budget', key, n]);
          if (window.__failBudget) throw new Error('Fixture: the budget could not be saved');
          if (!(n === -1 || n >= 0)) throw new Error('a budget is -1 or a count');
          const r = bus.rooms.find(r => r.name === key); r.budget = n; r.used = 0;
        }
      };
    },
  });
  const { page, ok } = t;
  const open = async key => {
    await page.evaluate(k => dinahWorkbench.open({ kind: 'bus-room', path: k }), key);
    const win = page.locator('.win[data-win-kind="bus-room"]').filter({ has: page.locator('.win-title', { hasText: { 'open-group': 'Open Group', 'counted-group': 'Counted Group', 'spent-group': 'Spent Group' }[key] }) });
    await win.locator('.group-members .group-turn-budget').waitFor();
    return win;
  };
  const raise = win => win.evaluate(el => pudlWindows.raise(el.dataset.win));
  try {
    const none = await open('open-group');
    ok('a group with no budget shows nothing about one', await none.locator('[data-group-budget]').count() === 0);
    const counted = await open('counted-group');
    ok('a group with a budget shows the turns left', (await counted.locator('[data-group-budget]').innerText()) === '3 turns left before the group waits for you', await counted.locator('[data-group-budget]').innerText());
    const spent = await open('spent-group');
    ok('a spent budget says so and what starts it again', /spent/i.test(await spent.locator('[data-group-budget]').innerText()) && /next message starts it again/.test(await spent.locator('[data-group-budget]').innerText()));
    ok('and is marked as a warning, not by colour alone', await spent.locator('[data-group-budget="spent"].warn').count() === 1);

    // The control: Unlimited first, then a count.
    await raise(counted);
    await counted.locator('.group-turn-budget').click();
    const dialog = page.locator('dialog.group-budget-dialog');
    await dialog.waitFor();
    ok('the control opens with the group\'s budget set', await dialog.locator('input[type=number]').inputValue() === '5' && await dialog.locator('.switch').getAttribute('aria-checked') === 'false');
    ok('Unlimited is the first choice', await dialog.locator('form > *').first().evaluate(el => el.classList.contains('switch') && /Unlimited/.test(el.textContent)));
    await dialog.locator('input[type=number]').fill('-4');
    await dialog.getByRole('button', { name: 'Save' }).click();
    ok('a count below 0 is refused in the dialog, with no call made', await dialog.locator('[role=alert]').isVisible() && (await page.evaluate(() => window.__calls.length)) === 0);
    await dialog.locator('input[type=number]').fill('12');
    await dialog.getByRole('button', { name: 'Save' }).click();
    await page.waitForFunction(() => window.__calls.length === 1);
    ok('saving sends the group and the count', JSON.stringify(await page.evaluate(() => window.__calls[0])) === JSON.stringify(['budget', 'counted-group', 12]));
    await page.waitForFunction(() => !document.querySelector('dialog.group-budget-dialog'));
    await counted.locator('[data-group-budget]').filter({ hasText: '12 turns left' }).waitFor();
    ok('the window shows the new budget, its turns given back', true);

    await raise(counted);
    await counted.locator('.group-turn-budget').click();
    await dialog.waitFor();
    await dialog.locator('.switch').click();
    ok('Unlimited disables the count', await dialog.locator('input[type=number]').isDisabled());
    await dialog.getByRole('button', { name: 'Save' }).click();
    await page.waitForFunction(() => window.__calls.length === 2);
    ok('Unlimited sends -1', JSON.stringify(await page.evaluate(() => window.__calls[1])) === JSON.stringify(['budget', 'counted-group', -1]));
    await page.waitForFunction(() => !document.querySelector('[data-group-budget]') || true);
    await counted.locator('.group-turn-budget').waitFor();
    await page.waitForFunction(k => !document.querySelector('.win[data-win-kind="bus-room"] [data-group-budget="open"]'), null);
    ok('and a group back to unlimited shows no budget', await counted.locator('[data-group-budget]').count() === 0);

    // A save that fails is shown in the dialog and keeps it open.
    await page.evaluate(() => { window.__failBudget = true; });
    await open('open-group');
    await raise(none);
    await none.locator('.group-turn-budget').click();
    await dialog.waitFor();
    await dialog.locator('.switch').click();
    await dialog.locator('input[type=number]').fill('4');
    await dialog.getByRole('button', { name: 'Save' }).click();
    await dialog.locator('[role=alert]').filter({ hasText: 'could not be saved' }).waitFor();
    ok('a failed save is shown in the dialog, which stays open', await dialog.isVisible());
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
