// The card for messages held because a chat's opening never finished. It says
// how many messages wait, offers Retry Opening, Open Chat and Dismiss, and a
// dismissed card comes back when the host gives it a new incident version,
// which it does when another message is held for the chat.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    state: { workspace: 'C:/Fixture', recent: ['C:/Fixture'] },
    app: () => {
      window.__calls = []; window.__dismissed = {};
      window.__held = { id: 'waiting_held', kind: 'delivery-failure', severity: 'error', title: 'Messages held for Parser Team Lead',
        body: '8 messages are waiting. Opening this chat on 21 Nov never finished.', details: ['Original chat: codex:parser; recipient: parser-bus', 'All pending messages are retained. Review the saved chat\'s recovery controls. Stage: opening'],
        sessionId: 'codex:parser', sessionTitle: 'Parser Team Lead', originTitle: 'Parser Team Lead', workspace: 'C:/Fixture', provider: 'codex', lead: 'codex:parser',
        retryAllowed: true, heldOpening: true, incidentVersion: 'v1', created: '2026-11-21T08:33:00Z' };
      return {
        Notifications: async () => [__held].filter(n => __dismissed[n.id] !== n.incidentVersion),
        DismissNotification: async id => { __calls.push(['dismiss', id]); __dismissed[id] = __held.incidentVersion; },
        RetryWaitingSession: async origin => { __calls.push(['retry', origin]); return { sessionId: origin, open: true }; },
        OpenSessionAt: async (cwd, id) => { __calls.push(['open', cwd, id]); throw new Error('Resource not found: fixture'); },
        SavedConversation: async () => ({ available: true, complete: true, messages: [{ role: 'agent', text: 'The saved history remains readable.' }] }),
        SessionContinuation: async () => ({ history: [] }),
      };
    },
    ready: () => document.querySelector('[data-notification-id="waiting_held"]'),
  });
  const { page, ok } = t;
  try {
    await page.evaluate(() => dinahWorkbench.open('needs-you'));
    const card = page.locator('[data-notification-id="waiting_held"]');
    const labels = async () => (await card.locator('.notice-actions button').allInnerTexts()).map(s => s.trim());
    const text = await card.textContent();
    ok('the title and body say what is held and why', text.includes('Messages held for Parser Team Lead') && text.includes('8 messages are waiting. Opening this chat on 21 Nov never finished.'), text);
    ok('the buttons are Retry Opening, Open Chat and Dismiss, in that order', JSON.stringify(await labels()) === JSON.stringify(['Retry Opening', 'Open Chat', 'Dismiss']), await labels());
    ok('Retry Opening is the primary button', await card.getByRole('button', { name: 'Retry Opening', exact: true }).evaluate(b => b.classList.contains('btn-primary')));
    ok('the technical detail is still on the card', text.includes('Stage: opening'));

    // The count changing must not move or resize anything on the card.
    const geometry = () => card.evaluate(el => { const base = el.getBoundingClientRect(); return JSON.stringify([Math.round(base.width), Math.round(base.height)].concat(Array.from(el.querySelectorAll('.notice-actions button')).map(b => { const r = b.getBoundingClientRect(); return [Math.round(r.x - base.x), Math.round(r.y - base.y), Math.round(r.width), Math.round(r.height)]; }))); });
    const before = await geometry();
    await page.evaluate(() => { __held = Object.assign({}, __held, { body: '9 messages are waiting. Opening this chat on 21 Nov never finished.', incidentVersion: 'v2' }); __emit('notification:delivery'); });
    await page.waitForFunction(() => { const el = document.querySelector('[data-notification-id="waiting_held"]'); return !!el && el.textContent.includes('9 messages are waiting.'); });
    ok('a new count redraws the card in place, the same size with its buttons where they were', await geometry() === before, [before, await geometry()]);

    await card.getByRole('button', { name: 'Retry Opening', exact: true }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'retry'));
    ok('Retry Opening calls RetryWaitingSession for the original chat', await page.evaluate(() => __calls.some(c => c[0] === 'retry' && c[1] === 'codex:parser')));

    await page.evaluate(() => dinahWorkbench.open('needs-you'));
    await card.getByRole('button', { name: 'Dismiss', exact: true }).click();
    await card.waitFor({ state: 'detached' });
    ok('Dismiss hides the card and records it with the host', await page.evaluate(() => __calls.some(c => c[0] === 'dismiss' && c[1] === 'waiting_held')));
    await page.evaluate(() => __emit('notification:delivery'));
    await page.waitForTimeout(300);
    ok('the card stays hidden while nothing new is held', await card.count() === 0);

    await page.evaluate(() => { __held = Object.assign({}, __held, { body: '10 messages are waiting. Opening this chat on 21 Nov never finished.', incidentVersion: 'v3' }); __emit('notification:delivery'); });
    await card.waitFor();
    ok('a new incident version brings the card back', (await card.textContent()).includes('10 messages are waiting.'));

    await card.getByRole('button', { name: 'Open Chat', exact: true }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'open'));
    ok('Open Chat opens the chat in its workspace', await page.evaluate(() => __calls.some(c => c[0] === 'open' && c[1] === 'C:/Fixture' && c[2] === 'codex:parser')));
    await page.screenshot({ path: require('path').join(t.out || require('./lib/page').OUT, 'held-delivery-card.png'), fullPage: true });
  } finally { await t.close(); }
})();
