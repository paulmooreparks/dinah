// A chat that mentions the person raises a card in Notifications (kind "mention"), from the
// event while the page runs and from the saved list at start. Open Group opens the group's
// window, the engine ends the card when the message is read (notifications:dismissed), and
// the card's own Dismiss tells the engine.
const { launch } = require('./lib/page');

(async () => {
  const mention = (n, extra) => Object.assign({ id: 'mention-' + n, kind: 'mention', severity: 'info', title: 'Docs Pass mentioned you in Release Group', body: 'The notes are ready. Which version do we ship?',
    sessionTitle: 'Docs Pass', sessionId: 'window-d1', group: 'release-group', groupTitle: 'Release Group', message: n, details: ['From the workspace docs.'], created: '2026-10-29T06:00:00Z', dismissed: false }, extra);
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      window.__calls = [];
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' };
      const room = { name: 'release-group', title: 'Release Group', lastAt: '', unread: 0, members: [docs, person], budget: -1, used: 0 };
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'] }),
        AgentProviders: async () => [], Sessions: async () => [], SessionsAt: async () => [],
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: [room], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1, noTurnBudget: -1 }),
        BusGroupMessages: async () => [], BusMarkGroupRead: async () => {},
        Notifications: async () => JSON.parse(sessionStorage.getItem('saved') || '[]'),
        DismissNotification: async id => { window.__calls.push(['dismiss', id]); }
      };
    },
  });
  const { page, ok } = t;
  const card = id => page.locator('#questions [data-notification-id="' + id + '"], #questions [data-notice-id="' + id + '"]');
  try {
    await page.evaluate(() => dinahWorkbench.open('needs-you'));
    await page.locator('#questions').waitFor({ state: 'attached' });
    await page.evaluate(item => __emit('notifications:added', item), mention(41));
    await page.locator('#questions').getByText('Docs Pass mentioned you in Release Group').first().waitFor();
    const text = await page.locator('#questions').innerText();
    ok('a mention raises a card with its title and the start of the message', /mentioned you in Release Group/.test(text) && /Which version do we ship\?/.test(text), text);
    ok('and the card needs the person', await page.evaluate(() => dinahHost.notices.list(e => e.id === 'mention-41' && e.needsPerson).length) === 1);
    const mentionCard = page.locator('#questions [data-notice-id="mention-41"]');
    ok('Open Group is the only primary action and Dismiss is secondary', await mentionCard.locator('.btn-primary').count() === 1 && await mentionCard.getByRole('button', { name: 'Open Group' }).evaluate(n => n.classList.contains('btn-primary')) && !await mentionCard.getByRole('button', { name: 'Dismiss', exact: true }).evaluate(n => n.classList.contains('btn-primary')));


    await page.evaluate(item => __emit('notifications:added', item), mention(41));
    await page.waitForTimeout(200);
    ok('the same message again adds no second card', await page.evaluate(() => dinahHost.notices.list(e => e.id === 'mention-41').length) === 1);

    await page.locator('#questions').getByRole('button', { name: 'Open Group' }).first().click();
    await page.locator('.win[data-win-kind="bus-room"]').waitFor();
    await t.shot('mention-notification');
    ok('Open Group opens the group\'s window', await page.locator('.win[data-win-kind="bus-room"] .win-title', { hasText: 'Release Group' }).count() === 1);

    await page.evaluate(() => __emit('notifications:dismissed', { id: 'mention-41' }));
    await page.waitForFunction(() => dinahHost.notices.list(e => e.id === 'mention-41').length === 0);
    ok('the engine ending the card removes it', true);
    ok('and that is not a Dismiss, so the engine is not asked to dismiss it', await page.evaluate(() => window.__calls.length) === 0);

    // The card's own Dismiss.
    await page.evaluate(item => __emit('notifications:added', item), mention(42, { group: '', groupTitle: '', title: 'Docs Pass sent you a message' }));
    await page.locator('#questions').getByText('Docs Pass sent you a message').first().waitFor();
    await page.locator('#questions .notices-dismiss').first().click();
    await page.waitForFunction(() => window.__calls.some(c => c[0] === 'dismiss' && c[1] === 'mention-42'));
    ok('Dismiss on a card tells the engine and removes the card', await page.evaluate(() => dinahHost.notices.list(e => e.id === 'mention-42').length) === 0);

    // At start the saved cards come back.
    await page.evaluate(item => sessionStorage.setItem('saved', JSON.stringify([item])), mention(43));
    await page.reload();
    await page.waitForFunction(() => window.dinahHost && window.dinahHost.notices && dinahHost.notices.list(e => e.id === 'mention-43').length === 1, null, { timeout: 10000 });
    ok('a mention saved by the engine is restored at start', true);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
