const { launch, PAGE } = require('./lib/page');
const { choose, shows } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    state: { workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider: 'codex' },
    app: () => {
      window.__calls = [];
      const old = { sessionId: 'codex-old', provider: 'codex', cwd: 'C:/Test', title: 'Ongoing Work', open: false };
      const next = { ...old, sessionId: 'devin-next', provider: 'devin' };
      const history = [{ role: 'user', text: 'Create one test fixture.', provider: 'codex', sessionId: old.sessionId }, { role: 'assistant', text: 'The test fixture is complete.', provider: 'codex', sessionId: old.sessionId }, { role: 'provider_switch', text: 'Switched from Codex to Devin.', provider: 'devin', sessionId: next.sessionId }];
      const handoff = 'Previous request: Create one test fixture.\nCompleted action: The test fixture is complete.\nDo not repeat completed actions.';
      const current = () => localStorage.getItem('test-continuation') ? next : old;
      return {
        AgentProviders: async () => ['devin', 'codex'].map(id => ({ id, installed: true, available: true, name: id === 'devin' ? 'Devin' : 'Codex' })),
        Sessions: async () => [current()],
        SessionContinuation: async id => ({ parentSessionId: id === next.sessionId ? old.sessionId : '', history: id === next.sessionId ? history : [] }),
        ResolveSessionContinuation: async id => localStorage.getItem('test-continuation') && id === old.sessionId ? next.sessionId : id,
        OpenSessionAt: async (ws, id) => {
          window.__calls.push(['open', id]);
          const info = id === next.sessionId ? next : old;
          window.__emit('agent:session', { ...info, open: true });
          if (id === old.sessionId) {
            window.__emit('agent:update', { sessionId: id, update: { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Create one test fixture.' } } });
            window.__emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'The test fixture is complete.' } } });
          } else if (localStorage.getItem('test-last-prompt')) {
            window.__emit('agent:update', { sessionId: id, update: { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: localStorage.getItem('test-last-prompt') } } });
            window.__emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'The existing test fixture has been reviewed.' } } });
          }
          return { ...info, open: true };
        },
        PreviewSessionContinuation: async (id, provider) => { window.__calls.push(['preview', id, provider]); return { sourceSessionId: id, provider, revision: 'history-v1', context: handoff, history, warnings: ['Earlier image contents and artifact access will not be transferred.'],
          facts: { kind: 'provider', from: { provider: 'codex', providerName: 'Codex', model: 'gpt-6.1-sol', workspace: 'C:/Test', workspaceName: 'Test' }, to: { provider: 'devin', providerName: 'Devin', model: 'ds-2-ultra', workspace: 'C:/Test', workspaceName: 'Test' }, handoff: { messages: 2, whole: 2, summarised: 0, chars: 60, budget: 8000, judged: false }, carried: [], notCarried: [{ id: 'mode', name: 'Mode', value: 'Default' }], running: { turn: false, queued: 0, approvals: false, applets: 0, canStop: true } } }; },
        ContinueSessionWithProvider: async (id, provider, revision) => {
          window.__calls.push(['continue', id, provider, revision]);
          if (window.__failSwitch) throw new Error('History changed. Review the handoff again.');
          localStorage.setItem('test-continuation', 'yes');
          // Provider notifications can precede the continuation RPC reply.
          window.__emit('agent:session', { ...next, open: true });
          window.__emit('agent:update', { sessionId: next.sessionId, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Early provider notification.' } } });
          await new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve)));
          return { session: { ...next, open: true }, parentSessionId: old.sessionId, history };
        },
        Send: async (id, text, doc, attachments) => { window.__calls.push(['send', id, text, attachments]); localStorage.setItem('test-last-prompt', text); },
        PickAttachments: async () => [{ name: 'draft.txt', mimeType: 'text/plain', data: 'ZHJhZnQ=', size: 5 }]
      };
    },
    goto: false,
  });
  const { page, ok } = t;
  try {
    if (process.env.DINAH_CONTINUATION_APP_SCRIPT) await t.context.route('http://dinah.test/app.js', route => route.fulfill({ path: process.env.DINAH_CONTINUATION_APP_SCRIPT }));
    await page.goto(PAGE);
    await t.waitReady();
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    await page.locator('#workspace-chats [data-session="codex-old"]').click();
    const front = () => page.locator('.win.active');
    // The provider is a menu of choices in the composer's toolbar.
    const provider = name => shows(front(), 'provider', name);
    const switchTo = name => choose(page, front(), 'provider', name);
    const locked = () => front().locator('[data-command="provider"]:disabled').first().waitFor({ state: 'attached', timeout: 3000 }).then(() => true, () => false);
    const prompt = () => front().locator('[data-view-id="prompt"]');
    await front().getByText('The test fixture is complete.', { exact: true }).waitFor();
    const originalRoot = await front().getAttribute('data-win');
    const originalURL = page.url();
    await prompt().fill('Review the completed test fixture.');
    await choose(page, front(), 'add', 'Files From This Computer…');
    await front().getByText('draft.txt', { exact: true }).waitFor();
    await switchTo('Devin');
    await page.locator('dialog.switch-confirm[open]').waitFor();
    const conf = page.locator('dialog.switch-confirm');
    ok('the confirmation says what changes from the preview’s facts', (await conf.locator('.dialog-title').textContent()) === 'Continue with Devin?' && /continues as a new chat with Devin/.test(await conf.textContent()) && /stays as it is/.test(await conf.textContent()) && /linked/.test(await conf.textContent()), await conf.textContent());
    ok('and how much of the conversation each part covers', /2 messages \(60 characters\b/.test(await conf.textContent()) && /8,000 budget/.test(await conf.textContent()), (await conf.textContent()).slice(0, 400));
    ok('the confirmation discloses omitted binary content and artifact access', await conf.getByText(/Earlier image contents and artifact access will not be transferred/).isVisible());
    ok('and which settings do not carry', /Mode: Default/.test(await conf.textContent()) && /stay with Codex/.test(await conf.textContent()));
    await t.shot('provider-continuation-preview');
    await conf.getByRole('button', { name: 'Cancel' }).click();
    await conf.waitFor({ state: 'detached' });
    ok('cancel keeps the provider, draft and attachments', await provider('Codex') && await prompt().inputValue() === 'Review the completed test fixture.' && await front().getByText('draft.txt', { exact: true }).isVisible());
    await page.evaluate(() => { window.__failSwitch = true; });
    await switchTo('Devin'); await page.locator('dialog.switch-confirm').getByRole('button', { name: 'Continue' }).click();
    await page.getByText('Error: History changed. Review the handoff again.', { exact: true }).waitFor();
    ok('a rejected handoff leaves the original conversation intact', await front().getAttribute('data-win') === originalRoot && await provider('Codex'));
    await page.evaluate(() => { window.__failSwitch = false; });
    await switchTo('Devin');
    await page.locator('dialog.switch-confirm').getByRole('button', { name: 'Continue' }).click();
    await front().getByText('Switched from Codex to Devin.', { exact: true }).waitFor();
    ok('switching keeps one conversation window and its visible history', await page.locator('.win[data-win-kind="chat-session"]').count() === 1 && await front().getByText('The test fixture is complete.', { exact: true }).count() === 1);
    ok('switching does not send a model prompt or repeat any action', await page.evaluate(() => !__calls.some(c => c[0] === 'send') && __calls.some(c => c[0] === 'continue' && c[3] === 'history-v1')));
    ok('the draft and attachments survive a successful switch', await prompt().inputValue() === 'Review the completed test fixture.' && await front().getByText('draft.txt', { exact: true }).isVisible());
    await page.evaluate(() => {
      window.__emit('agent:update', { sessionId: 'devin-next', update: { sessionUpdate: 'tool_call', toolCallId: 'boundary', title: 'Read fixture', status: 'completed' } });
      for (const text of ['I', "'ll add a Calendar", ' view.']) window.__emit('agent:update', { sessionId: 'devin-next', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } } });
      window.__emit('agent:turn', { sessionId: 'devin-next', stopReason: 'end_turn' });
    });
    await front().getByText('Devin finished its turn.', { exact: true }).first().waitFor();
    await page.waitForFunction(() => [...document.querySelectorAll('.win.active .msg-text')].some(n => n.textContent.includes('view.')));
    ok('early continuation events leave one consumer for each streamed chunk', await front().getByText("I'll add a Calendar view.", { exact: true }).count() === 1);
    ok('text arriving before the continuation reply is retained once', await front().getByText('Early provider notification.', { exact: true }).count() === 1);
    ok('a continued turn finishes once', await front().getByText('Devin finished its turn.', { exact: true }).count() === 1);
    await page.evaluate(() => __emit('agent:session', { sessionId: 'devin-next', provider: 'devin', cwd: 'C:/Test', title: 'Ongoing Work', open: true, working: true }));
    ok('a running turn prevents another switch', await locked());
    await page.evaluate(() => __emit('agent:session', { sessionId: 'devin-next', provider: 'devin', cwd: 'C:/Test', title: 'Ongoing Work', open: true, working: false, queued: ['queued'] }));
    ok('queued messages prevent another switch', await locked());
    await page.evaluate(() => __emit('agent:queue', { sessionId: 'devin-next', queued: [] }));
    await prompt().press('Control+Enter');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'send'));
    ok('the next prompt and retained attachment target the new provider session', await page.evaluate(() => __calls.some(c => c[0] === 'send' && c[1] === 'devin-next' && c[3][0].name === 'draft.txt')));
    await t.shot('provider-continuation');
    await page.reload();
    await front().getByText('Switched from Codex to Devin.', { exact: true }).waitFor();
    await front().getByText('The test fixture is complete.', { exact: true }).waitFor();
    await front().getByText('The existing test fixture has been reviewed.', { exact: true }).waitFor();
    const restored = { messages: await front().getByText('The test fixture is complete.', { exact: true }).count(), devin: await provider('Devin') };
    ok('reopening restores the provider boundary and previous transcript once ' + JSON.stringify(restored), restored.messages === 1 && restored.devin);
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex-old' }));
    await page.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="chat-session"]').length === 1);
    ok('old session links open the current continuation', await provider('Devin') && await front().getByText('Switched from Codex to Devin.', { exact: true }).count() === 1);
    await page.goto(originalURL);
    await front().getByText('Switched from Codex to Devin.', { exact: true }).waitFor();
    await front().getByText('The test fixture is complete.', { exact: true }).waitFor();
    await page.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="chat-session"]').length === 1);
    ok('an old saved window address restores the current conversation', await provider('Devin'));
  } finally { await t.close(); }
})();
