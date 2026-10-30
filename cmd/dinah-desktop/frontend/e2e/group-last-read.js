// A group window remembers the last message the person had seen. When they come back, the
// messages from others after it are marked, a line says where they begin, and a button goes there.
// The list still opens at its end. The first time a group is opened nothing is marked.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1200, height: 800 },
    app: () => {
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' };
      const rooms = ['seen-group', 'fresh-group', 'short-group'].map(name => ({ name, title: { 'seen-group': 'Seen Group', 'fresh-group': 'Fresh Group', 'short-group': 'Short Group' }[name], lastAt: '', unread: 0, members: [docs, person], budget: -1, used: 0 }));
      const long = n => 'Message ' + n + '.\n\n' + Array.from({ length: 12 }, (_, i) => 'Line ' + i + ' of message ' + n + '.').join('\n\n');
      const messages = Array.from({ length: 8 }, (_, i) => ({ id: i + 1, room: 'x', from: i === 5 ? 'Paul' : 'Docs Pass', fromPerson: i === 5, fromWorkspace: 'docs', to: '', text: long(i + 1), createdAt: '2026-10-29T06:0' + i + ':00Z', kind: 'message' }));
      window.__messages = messages; window.__long = long;
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'] }),
        AgentProviders: async () => [], Sessions: async () => [], SessionsAt: async () => [],
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace', 'bus-seen': { 'group:seen-group': 3, 'group:short-group': 3 } } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: [], unread: 0, rooms, workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1, noTurnBudget: -1 }),
        BusGroupMessages: async key => key === 'short-group' ? Array.from({ length: 5 }, (_, i) => ({ id: i + 1, room: 'x', from: 'Docs Pass', fromWorkspace: 'docs', to: '', text: 'Short ' + (i + 1) + '.', createdAt: '2026-10-29T06:0' + i + ':00Z', kind: 'message' })) : messages, BusMarkGroupRead: async () => {}
      };
    },
  });
  const { page, ok } = t;
  const open = async (key, title) => {
    const k = await page.evaluate(path => dinahWorkbench.open({ kind: 'bus-room', path }), key);
    const win = page.locator('.win[data-win-kind="bus-room"]').filter({ has: page.locator('.win-title', { hasText: title }) });
    await win.locator('.bus-message').first().waitFor();
    await page.waitForTimeout(400);
    return win;
  };
  try {
    const first = await open('fresh-group', 'Fresh Group');
    ok('the first time a group is opened nothing is marked and there is no button', await first.locator('.bus-unseen').count() === 0 && await first.locator('.bus-jump:not([hidden])').count() === 0);
    ok('and what was seen is then kept', await page.evaluate(() => (dinahUIState.get('bus-seen') || {})['group:fresh-group']) === 8);

    const win = await open('seen-group', 'Seen Group');
    await win.locator('.bus-new-line').waitFor();
    ok('messages from others after the last seen one are marked', await win.locator('.bus-unseen').count() === 4, String(await win.locator('.bus-unseen').count()));
    ok('the person\'s own message among them is not', await win.locator('.bus-message', { hasText: 'Message 6.' }).evaluate(el => !el.classList.contains('bus-unseen')));
    ok('a line says where they begin, in words', /4 new since you last looked/.test(await win.locator('.bus-new-line').innerText()));
    ok('the list still opens at its end', await win.locator('.bus-log').evaluate(el => el.scrollHeight - el.scrollTop - el.clientHeight < 24));
    const jump = win.locator('.bus-jump');
    ok('a button offers to go to where they stopped', await jump.isVisible() && /Jump to Where You Stopped/.test(await jump.innerText()));
    await jump.click();
    await page.waitForTimeout(300);
    await t.shot('group-last-read');
    ok('the button shows the line and goes away', await win.locator('.bus-new-line').evaluate(el => { const a = el.getBoundingClientRect(), b = el.closest('.bus-log').getBoundingClientRect(); return a.top >= b.top - 1 && a.bottom <= b.bottom + 1; }) && await jump.isHidden());
    await win.locator('.bus-log').evaluate(el => { el.scrollTop = el.scrollHeight; });
    await page.waitForFunction(() => (dinahUIState.get('bus-seen') || {})['group:seen-group'] === 8);
    ok('reading to the end keeps the last message as seen', true);

    // New messages that are all in view at the end need no button to find them.
    const short = await open('short-group', 'Short Group');
    await short.locator('.bus-new-line').waitFor();
    await page.waitForTimeout(500);
    ok('messages after the last seen one are marked in a short group', await short.locator('.bus-unseen').count() === 2);
    ok('and the button is not offered when the line is already in view at the end', await short.locator('.bus-jump').isHidden());

    // A group left open behind another window shows what arrived meanwhile when it comes back.
    await page.evaluate(k => pudlWindows.raise(k), await first.evaluate(el => el.dataset.win));
    await page.waitForTimeout(300);
    ok('while the group is in front and at its end, nothing is marked', await first.locator('.bus-unseen').count() === 0);
    await page.evaluate(k => pudlWindows.raise(k), await win.evaluate(el => el.dataset.win));
    await page.waitForTimeout(200);
    await page.evaluate(() => {
      for (const n of [9, 10, 11]) window.__messages.push({ id: n, room: 'x', from: n === 11 ? 'Paul' : 'Docs Pass', fromPerson: n === 11, fromWorkspace: 'docs', to: '', text: window.__long(n), createdAt: '2026-10-29T07:00:00Z', kind: 'message' });
      __emit('bus:changed', {});
    });
    await first.locator('.bus-message', { hasText: 'Message 10.' }).waitFor({ state: 'attached' });
    ok('messages that arrive while the group is behind another window are not marked yet', await first.locator('.bus-unseen').count() === 0);
    await page.evaluate(k => pudlWindows.raise(k), await first.evaluate(el => el.dataset.win));
    await first.locator('.bus-new-line').waitFor();
    ok('when it comes to the front, the ones from others that arrived are marked', await first.locator('.bus-unseen').count() === 2 && /2 new since you last looked/.test(await first.locator('.bus-new-line').innerText()));
    ok('and a button offers to go to the first of them', await first.locator('.bus-jump').isVisible());
    await first.locator('.bus-jump').click();
    await page.waitForTimeout(200);
    await first.locator('.bus-log').evaluate(el => { el.scrollTop = el.scrollHeight; });
    await page.waitForFunction(() => (dinahUIState.get('bus-seen') || {})['group:fresh-group'] === 11);
    ok('reading to the end then keeps the newest as seen', true);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
