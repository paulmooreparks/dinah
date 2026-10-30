// A chat window restored with the layout shows Dinah's saved copy of
// the chat at once, without asking the agent to replay it, says so above
// the prompt, and reconnects only when a message is sent, Reconnect Now is
// pressed or the Skills menu is opened. A chat with no saved copy reopens
// as before.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    app: () => {
      const saved = {
        'saved-one': [{ role: 'you', text: 'Summarise the plan.', images: [{ name: 'plan.png', mimeType: 'image/png', data: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+aG1kAAAAASUVORK5CYII=' }] }, { role: 'tool', text: 'Read file\nStatus: completed\nThe plan text.' }, { role: 'agent', text: 'The plan asks for a config reload.' }, { role: 'status', text: 'Turn ended: end_turn' }],
        'saved-two': [{ role: 'you', text: 'Second chat.' }, { role: 'agent', text: 'Second answer.' }],
        'saved-three': [{ role: 'you', text: 'Third chat.' }, { role: 'agent', text: 'Third answer.' }],
        'saved-far': [{ role: 'you', text: 'A chat elsewhere.' }, { role: 'agent', text: 'Far answer.' }]
      };
      return {
        // The first state is held until the test lets it go, so the page can
        // be seen starting; a page loaded later gets it at once.
        State: async () => (sessionStorage.getItem('state-held') ? null : (sessionStorage.setItem('state-held', '1'), await new Promise(r => { window.__releaseState = r; })), { workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider: 'devin', catalog: [], defaults: {}, pinned: [] }),
        AgentProviders: async () => [{ id: 'devin', installed: true, available: true }],
        ResolveSessionContinuation: async id => id,
        SavedConversation: async id => (window.__calls.push(['saved', id]), saved[id]) ? { available: true, messages: saved[id] } : { available: false, messages: [] },
        SessionContinuation: async () => ({ history: [] }),
        OpenSessionAt: async (ws, id) => {
          window.__calls.push(['open', id]);
          // The first chat's agent replays only when the test lets it.
          if (id === 'saved-one') await window.__gate;
          window.__emit('agent:updates', { sessionId: id, updates: [
            { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Summarise the plan.' } },
            { sessionUpdate: 'agent_thought_chunk', content: { type: 'text', text: 'Thinking it over.' } },
            { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'The plan asks for a config reload.' } }] });
          return { sessionId: id, provider: 'devin', title: 'Restored Chat', cwd: ws, open: true, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } };
        },
        Send: async (id, text, doc, atts) => { window.__calls.push(['send', id, text, atts]); }
      };
    },
    init: () => {
      window.__calls = [];
      // Held on the first page only; a page loaded later replays at once.
      window.__gate = sessionStorage.getItem('gate-opened') ? Promise.resolve() :
        new Promise(r => { window.__openGate = () => { sessionStorage.setItem('gate-opened', '1'); r(); }; });
    },
    ready: () => window.dinahWorkbench && typeof window.__releaseState === 'function',
  });
  const { page, ok } = t;
  try {
    // While the app starts, the status area says so and the page is busy.
    ok('the status area says Dinah is starting', await page.locator('#status-starting').isVisible() && await page.evaluate(() => document.body.getAttribute('aria-busy') === 'true'));
    await page.evaluate(() => window.__releaseState());
    await page.locator('#status-starting').waitFor({ state: 'hidden' });
    ok('once started, the starting line goes and the page is not busy', await page.evaluate(() => !document.body.hasAttribute('aria-busy')));
    const front = () => page.locator('.win.active');
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'saved-one' }));
    await front().locator('.sent-images img').waitFor();
    ok('a saved message restores its image without a page error', await front().locator('.sent-images img').evaluate(img => img.complete && img.naturalWidth === 1) && t.errors.length === 0);
    await front().locator('.activity-row', { hasText: 'Summarise the plan.' }).waitFor();
    const log = await front().locator('.activity-row').allTextContents();
    ok('a restored chat shows its saved history at once', log.join(' ').includes('Summarise the plan.') && log.join(' ').includes('The plan asks for a config reload.') && log.join(' ').includes('Read file'));
    // Then it reconnects by itself, so it answers the bus without the
    // person touching it (23 October 2026); the saved copy stays in sight,
    // with no "Reopening" line, while the agent replays.
    await page.waitForFunction(() => __calls.some(c => c[0] === 'open' && c[1] === 'saved-one'));
    await page.waitForTimeout(200);
    const during = await front().locator('.activity-row').allTextContents();
    ok('it reconnects by itself, keeping the saved copy in sight while the agent replays', during.join(' ').includes('The plan asks for a config reload.') &&
      !/Reopening the session/.test(await front().innerText()));
    // Choosing it in a list of chats, as the first click of a double-click to
    // rename does, starts no second reconnect.
    // The suite lists no chats, so the row is the list's own markup for one.
    await page.evaluate(() => {
      const a = document.createElement('a');
      a.href = '#'; a.dataset.session = 'saved-one'; a.textContent = 'Saved One';
      document.getElementById('sessions').appendChild(a);
      a.click(); a.remove();
    });
    await page.waitForTimeout(300);
    ok('choosing a reconnecting chat in the list starts no second reconnect', await page.evaluate(() => __calls.filter(c => c[0] === 'open').length === 1));
    await t.shot('chat-saved-restore');
    await page.evaluate(() => {
      window.__partialReplay = [];
      const frame = () => {
        document.querySelectorAll('.chat-instance [data-view-id=logs] > *').forEach(log => {
          if (!log.hidden && getComputedStyle(log).visibility !== 'hidden' && log.textContent.includes('Thinking it over.') && !log.textContent.includes('The plan asks for a config reload.')) __partialReplay.push(log.textContent);
        });
        if (!window.__stopReplayFrames) requestAnimationFrame(frame);
      };
      requestAnimationFrame(frame); __openGate();
    });
    await front().locator('.activity-row', { hasText: 'Thinking it over.' }).waitFor({ state: 'attached' });
    ok('once the agent has replayed, the chat shows its fuller history and no saved note', await front().locator('[data-view-id="saved-note"]').isHidden() &&
      (await front().locator('.activity-row', { hasText: 'Summarise the plan.' }).count()) === 1);
    ok('replay is revealed only after rendering settles', await page.evaluate(() => { __stopReplayFrames = true; return __partialReplay.length === 0; }));
    await front().locator('[data-view-id="prompt"]').fill('And the next step?');
    await front().locator('[data-view-id="prompt"]').press('Control+Enter');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'send'));
    ok('sending goes to the reconnected chat without reconnecting it again', await page.evaluate(() => __calls.filter(c => c[0] === 'open' && c[1] === 'saved-one').length === 1));
    // Another restored chat reconnects by itself too.
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'saved-two' }));
    await page.waitForFunction(() => __calls.some(c => c[0] === 'open' && c[1] === 'saved-two'));
    ok('every restored chat reconnects by itself', true);
    // A chat with no saved copy reopens as before.
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'never-saved' }));
    await page.waitForFunction(() => __calls.some(c => c[0] === 'open' && c[1] === 'never-saved'));
    ok('a chat with no saved copy is reopened at once', true);
    // A chat has one window: a second window for it under another workspace,
    // as a layout could keep, closes, and the first is brought forward.
    const firstKey = await page.evaluate(() => dinahWorkbench.key({ kind: 'chat-session', workspace: 'C:/Test', path: 'saved-one' }));
    const dupKey = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Other', path: 'saved-one' }));
    await page.waitForFunction(k => !pudlWindows.state().open.includes(k), dupKey);
    ok('a second window for the same chat closes, and the chat\'s window is in front', await page.evaluate(([a, b]) => pudlWindows.state().top === a && !pudlWindows.state().open.includes(b), [firstKey, dupKey]) &&
      (await page.locator('.win[data-win="' + firstKey + '"] .activity-row').count()) > 0);
    // Restored with the layout, a chat behind the window in front starts when
    // it is seen or in idle time (PUDL 0.66.0), keeping the title it had.
    const backKey = await page.evaluate(() => dinahWorkbench.key({ kind: 'chat-session', workspace: 'C:/Test', path: 'saved-one' }));
    await page.evaluate(k => pudlWindows.retitle(k, 'Config Reload Summary'), backKey);
    await page.waitForFunction(k => ((dinahUIState.get('dinah-window-titles') || {})[k] || {}).title === 'Config Reload Summary', backKey);
    ok('each open window\'s title is kept, for its markup when the layout is restored', true);
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'saved-two' }));
    const restoredURL = page.url();
    await page.goto(restoredURL);
    await page.waitForFunction(() => window.dinahWorkbench && window.pudlApplets && window.pudlWindows);
    const back = await page.evaluate(k => ({ open: pudlWindows.state().open.includes(k), top: pudlWindows.state().top === k }), backKey);
    ok('the layout is restored with the chat behind another window', back.open && !back.top);
    await page.waitForFunction(k => pudlApplets.started(k), backKey, { timeout: 10000 });
    ok('the chat behind starts in idle time, after the one in front', true);
    // Its saved copy is drawn through the replay's handlers; once drawn, the
    // chat shows its history, not the replay's progress line (23 October 2026).
    await page.waitForFunction(k => { const w = document.querySelector('[data-win="' + CSS.escape(k) + '"]'); return w && w.querySelector('.activity-row'); }, backKey);
    await page.waitForFunction(k => { const w=document.querySelector('[data-win="'+CSS.escape(k)+'"]'), line=w?.querySelector('[data-view-id="replay"]'); return w && (!line || line.hidden) && !/Reopening the session/.test(w.innerText); }, backKey);
    ok('a chat restored behind another shows its saved history, not "Reopening the session"', await page.evaluate(k => {
      const w = document.querySelector('[data-win="' + CSS.escape(k) + '"]'), line = w.querySelector('[data-view-id="replay"]');
      return (!line || line.hidden) && !/Reopening the session/.test(w.innerText);
    }, backKey));
    // Once it has reconnected behind the other window, its own header says so.
    await page.waitForFunction(k => /Idle/.test(document.querySelector('[data-win="' + CSS.escape(k) + '"] [data-view-id="page-state"]').textContent), backKey);
    ok('a chat that reconnects behind another says in its header that it is open', true);
    // A restored chat can be mentioned for asking: it is listed under Open
    // Chats, and sending connects it behind the asking chat's window.
    await page.evaluate(() => __emit('agent:titles', { 'saved-one': 'Config Reload Summary', 'saved-two': 'Second Chat' }));
    const threeKey = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'saved-three' }));
    await front().locator('.activity-row', { hasText: 'Third chat.' }).waitFor();
    const prompt = front().locator('[data-view-id="prompt"]'), picker = front().locator('[data-view-id="slash-pop"]');
    await prompt.fill('@config');
    await picker.getByText('Config Reload Summary', { exact: true }).waitFor();
    ok('a chat restored with the layout is offered under Open Chats', /open chats/i.test(await picker.innerText()));
    await prompt.press('Enter');
    await prompt.pressSequentially('which config files?');
    const before = await page.evaluate(() => __calls.length);
    await prompt.press('Control+Enter');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'send' && c[1] === 'saved-three'));
    // The mentioned chat may have reconnected by itself already; either way
    // it is connected before the message goes.
    const sent = await page.evaluate(n => {
      const open = __calls.findIndex(c => c[0] === 'open' && c[1] === 'saved-one'), send = __calls.findIndex((c, i) => i >= n && c[0] === 'send' && c[1] === 'saved-three');
      return { order: open >= 0 && send > open, atts: __calls[send][3] };
    }, before);
    ok('the mentioned chat is connected before the message goes with it attached', sent.order && sent.atts.length === 1 && sent.atts[0].kind === 'chat' && sent.atts[0].sessionId === 'saved-one', JSON.stringify(sent));
    ok('the asking chat stays in front, and the asked chat\'s window shows its history', await page.evaluate(k => pudlWindows.state().top === k, threeKey) &&
      (await page.locator('.win[data-win="' + backKey + '"] .activity-row').count()) > 0 && await page.locator('.win[data-win="' + backKey + '"] .activity').first().isVisible());
    // Cross-workspace collaboration offers chats independently of the sidebar.
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Far', path: 'saved-far' }));
    await front().locator('.activity-row', { hasText: 'A chat elsewhere.' }).waitFor();
    await front().locator('[data-view-id="prompt"]').fill('@');
    await front().locator('[data-view-id="slash-pop"]').waitFor();
    ok('a chat can mention a restored chat in another workspace', (await front().locator('[data-view-id="slash-pop"]').innerText()).includes('Config Reload Summary'));
  } finally { await t.close(); }
})();
