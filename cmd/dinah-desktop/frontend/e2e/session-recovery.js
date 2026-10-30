// A chat whose provider cannot open it: the saved history kept, exported,
// recovered into a new thread, and a long chat cut off mid-turn.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspace: 'C:/Test', recent: ['C:/Test'] },
    app: () => {
      const history = [{ role: 'you', text: 'Preserve the original work.' }, { role: 'agent', provider: 'codex', text: 'The original work is saved.' }];
      return {
        AgentProviders: async () => [{ id: 'codex', name: 'Codex', installed: true, available: true, status: 'detected' }],
        OpenSessionAt: async () => { throw new Error('thread not loaded: missing'); },
        SessionContinuation: async () => ({ history: [] }),
        // codex:cut is a long chat whose last turn was cut off when Dinah closed.
        SavedConversation: async id => id === 'codex:cut'
          ? { available: true, complete: false, recoverable: true, warning: 'The last turn was cut off before it finished, so its end may be missing from this saved copy.', messages: Array.from({ length: 1000 }, (_, i) => ({ role: i % 2 ? 'agent' : 'you', provider: 'codex', text: 'Long message ' + i })) }
          : { available: true, complete: true, messages: history },
        PreviewSessionRecovery: async (id, provider) => { __calls.push(['preview', id, provider]); return { context: 'Reviewed saved history', revision: 'saved-v1', warnings: [] }; },
        RecoverSession: async (id, provider, revision) => { __calls.push(['recover', id, provider, revision]); return { session: { sessionId: 'codex:recovered', provider, cwd: 'C:/Test', title: 'Recovered Chat', open: true }, history }; },
        SaveRun: async text => { __calls.push(['save', text]); return 'C:/Test/saved-chat.md'; },
        Send: async (id, text) => { __calls.push(['send', id, text]); }
      };
    },
    init: () => { window.__calls = []; },
    ready: () => window.dinahWorkbench && window.dinahEditor,
  });
  const { page, ok } = t;
  try {
    // The engine reports a chat's provider; a window for a chat it has not reported on has none to name.
    await page.evaluate(() => ['codex:missing', 'codex:cut'].forEach(id => __emit('agent:session', { sessionId: id, provider: 'codex', cwd: 'C:/Test', open: false })));
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:missing' }));
    const front = () => page.locator('.win.active');
    await front().locator('[data-view-id="session-note-title"]').getByText('Codex could not open this session.', { exact: true }).waitFor();
    await front().getByText('The original work is saved.', { exact: true }).waitFor();
    ok('a failed Codex load retains local history and names the correct provider', !(await front().innerText()).includes('Devin could not open'));
    await front().locator('[data-view-id="session-save-history"]').click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'save'));
    ok('saved history can be exported without the provider', await page.evaluate(() => __calls.find(c => c[0] === 'save')[1].includes('The original work is saved.')));
    await front().locator('[data-view-id="prompt"]').fill('Continue carefully.');
    await front().locator('[data-view-id="session-recover"]').click();
    await page.locator('#ask').waitFor();
    await page.locator('#ask-yes').click();
    await front().getByText('Continued from saved history with Codex.', { exact: true }).waitFor();
    ok('recovery preserves history and draft without sending a model turn', (await front().locator('[data-view-id="prompt"]').inputValue()) === 'Continue carefully.' && await page.evaluate(() => !__calls.some(c => c[0] === 'send') && __calls.some(c => c[0] === 'recover' && c[3] === 'saved-v1')));
    await front().locator('[data-view-id="prompt"]').press('Control+Enter');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'send'));
    ok('the next message uses the recovered thread ID', await page.evaluate(() => __calls.find(c => c[0] === 'send')[1] === 'codex:recovered'));

    // A long chat whose last turn was cut off: only its latest messages are
    // drawn, the row at the top counts the rest, and it can still be recovered.
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:cut' }));
    await front().locator('[data-view-id="session-note-title"]').getByText('Codex could not open this session.', { exact: true }).waitFor();
    await front().getByText('Long message 999', { exact: true }).waitFor();
    const marker = (await front().locator('.trimmed-row .trimmed-words').textContent()).trim();
    ok('a long saved chat draws only its latest 400 messages and counts the rest at the top', marker.startsWith('600 earlier entries') &&
      await front().getByText('Long message 600', { exact: true }).count() === 1 && await front().getByText('Long message 599', { exact: true }).count() === 0);
    ok('a chat cut off mid-turn says so and can still be recovered', (await front().locator('[data-view-id="session-note-body"]').innerText()).includes('cut off') && !(await front().locator('[data-view-id="session-recover"]').isDisabled()));
  } finally { await t.close(); }
})();
