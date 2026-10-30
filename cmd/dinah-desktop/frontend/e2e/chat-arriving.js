// A message waiting for a chat that is still replaying is shown at once as arriving,
// a card at the end of the transcript that the delivered message replaces where it
// stands, and that goes with the chat when the chat closes.
const { launch, PAGE } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => {
      window.__release = null;
      return {
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        OpenSessionAt: async (ws, id) => {
          if (window.__hold) await new Promise(r => { window.__release = r; });
          return { sessionId: id, title: id, provider: 'claude', cwd: ws, open: true, working: false, queued: [] };
        },
        SkillsForProviderAt: async () => [], SkillsAt: async () => []
      };
    },
    // The open is held until the test lets it go, so the chat is replaying.
    init: () => { window.__hold = true; },
    ready: () => window.dinahWorkbench && window.dinahEditor,
    goto: false,
  });
  const { page, ok } = t;
  const ID = 'claude:arriving';
  const msg = (id, text, extra) => Object.assign({ id, kind: 'text', from: 'Docs Pass', text, sentAt: '2026-10-29T04:00:00Z', group: 'Release Review' }, extra || {});
  const arrive = (messages, more) => page.evaluate(([i, ms, m]) => __emit('agent:arriving', { sessionId: i, messages: ms, more: m }), [ID, messages, more || 0]);
  try {
    await page.goto(PAGE);
    await t.waitReady();
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'claude:arriving' }));
    const win = page.locator('[data-win="' + key + '"]');
    await win.locator('[data-view-id="prompt"]').waitFor();
    // The replay is under way (the open is held), so the transcript is hidden.
    await page.waitForFunction(k => !document.querySelector('[data-win="' + k + '"] [data-view-id="replay"]').hidden, key);

    await arrive([msg(7, 'Please review the guide.'), msg(8, 'And the notes.')], 3);
    await page.waitForFunction(k => /waiting for/.test(document.querySelector('[data-win="' + k + '"] [data-view-id="replay"]').textContent), key);
    ok('while the replay hides the transcript, the progress line says messages are waiting', /5 messages are waiting/.test(await win.locator('[data-view-id="replay"]').textContent()), await win.locator('[data-view-id="replay"]').textContent());

    await page.evaluate(() => { window.__release(); window.__hold = false; });
    await page.waitForFunction(k => document.querySelector('[data-win="' + k + '"] [data-view-id="replay"]').hidden, key);
    const cards = win.locator('.arriving-card');
    ok('when the transcript shows, the waiting messages are cards at its end', await cards.count() === 2);
    ok('a card says who sent the message and that it is waiting', /Waiting/.test(await cards.first().innerText()) && /Arriving: message from Docs Pass/.test(await cards.first().innerText()));
    ok('with the text', (await cards.first().innerText()).includes('Please review the guide.'));
    ok('and the group it came through', (await cards.first().innerText()).includes('Release Review'));
    ok('and a line counts the rest', (await win.locator('.arriving-more').innerText()).includes('3 more messages are waiting'));
    ok('a card is display: nothing in it can be pressed', await cards.first().locator('button, input, a').count() === 0);

    // Told again, nothing doubles.
    await arrive([msg(7, 'Please review the guide.'), msg(9, 'A third.')], 2);
    await page.waitForFunction(() => document.querySelectorAll('.arriving-card').length === 3);
    ok('a message announced twice has one card', await cards.count() === 3);
    ok('and the count of the rest follows the latest event', (await win.locator('.arriving-more').innerText()).includes('2 more messages'));

    // Something said after the cards, then the delivery: the delivered card takes the first card's place.
    await page.evaluate(i => __emit('agent:update', { sessionId: i, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Still replaying: later words.' } } }), ID);
    await page.evaluate(i => __emit('agent:sent', { sessionId: i, kind: 'bus', text: 'Please review the guide.', delivered: [{ text: 'Please review the guide.', delivered: { id: 7, kind: 'text', from: 'Docs Pass', time: '2026-10-29T04:00:00Z' } }] }), ID);
    await win.locator('.delivered-card').waitFor();
    ok('the delivered card replaces its waiting card', await cards.count() === 2 && await win.locator('.delivered-card').count() === 1);
    const order = await win.locator('[data-view-id="logs"] > li, .run-log > li').evaluateAll(lis => lis.map(li => li.classList.contains('delivered-row') ? 'delivered' : li.classList.contains('arriving-row') && !li.classList.contains('arriving-more') ? 'arriving' : li.classList.contains('arriving-more') ? 'more' : 'other'));
    ok('in the same place, ahead of the waiting messages after it', order.indexOf('delivered') >= 0 && order.indexOf('delivered') < order.indexOf('arriving'), order.join(','));

    // A chat that closes takes its waiting cards.
    await page.evaluate(i => __emit('agent:session', { sessionId: i, open: false, working: false, queued: [] }), ID);
    await page.waitForFunction(() => document.querySelectorAll('.arriving-card').length === 0);
    ok('closing the chat removes the cards and the count', await win.locator('.arriving-more').count() === 0);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
