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
        PreviewSessionRecovery: async (id, provider) => { __calls.push(['preview', id, provider]); if (window.__holdRecovery) return new Promise(resolve => { window.__releaseRecovery = resolve; }); return { revision: 'saved-v1', warnings: [] }; },
        RecoverSession: async (id, provider, revision) => { __calls.push(['recover', id, provider, revision]); return { session: { sessionId: 'codex:recovered', provider, cwd: 'C:/Test', title: 'Recovered Chat', open: true }, history }; },
        SaveRun: async text => { __calls.push(['save', text]); return 'C:/Test/saved-chat.md'; },
        Send: async (id, text) => { __calls.push(['send', id, text]); }
      };
    },
    // The engine adds the Codex module's advice to every failed open of a Codex chat
    // (agent_threads.go). codex:plain stands for a chat whose provider adds none.
    init: () => {
      window.__calls = [];
      window.__openAdvice = (method, args) => args[1] === 'codex:plain' ? '' : 'Codex gives a conversation to one program at a time, so if this conversation is open in another program that uses Codex, such as the Codex app or an editor, closing it there and pressing Try Again may free it.';
    },
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
    // The note keeps the agent's own words as they were and adds the provider's advice as a
    // paragraph of its own below them, inside the note and above its buttons.
    const noteParts = () => front().evaluate(win => {
      const box = id => { const el = win.querySelector('[data-view-id="' + id + '"]'), r = el.getBoundingClientRect(); return { top: r.top, bottom: r.bottom, left: r.left, right: r.right, height: r.height, width: r.width, text: el.textContent, hidden: el.hidden }; };
      const notice = win.querySelector('[data-view-id="session-note"] .notice');
      return { note: box('session-note'), body: box('session-note-body'), advice: box('session-note-advice'), retry: box('session-retry'), prompt: box('prompt'), overflow: notice.scrollWidth - notice.clientWidth };
    });
    const withAdvice = await noteParts();
    ok("the note keeps the agent's own words exactly as before", withAdvice.body.text === 'Codex reported: thread not loaded: missing Your saved history is shown above. You can save it or continue it in a fresh chat.');
    ok('the advice is a paragraph of its own below those words and above the buttons', !withAdvice.advice.hidden && withAdvice.advice.text.startsWith('Codex gives a conversation to one program at a time, so ') && withAdvice.advice.text.endsWith('pressing Try Again may free it.') &&
      withAdvice.advice.top >= withAdvice.body.bottom && withAdvice.retry.top >= withAdvice.advice.bottom && withAdvice.advice.left >= withAdvice.note.left && withAdvice.advice.right <= withAdvice.note.right);
    // With the advice taken away the note is shorter by the paragraph and its gap, and nothing else has another size.
    await front().evaluate(win => { win.querySelector('[data-view-id="session-note-advice"]').hidden = true; });
    const without = await noteParts();
    await front().evaluate(win => { win.querySelector('[data-view-id="session-note-advice"]').hidden = false; });
    ok('the advice adds its own lines to the note and changes no other size', Math.abs((withAdvice.note.height - without.note.height) - (withAdvice.advice.height + 8)) <= 1 &&
      without.body.height === withAdvice.body.height && without.note.width === withAdvice.note.width && without.prompt.width === withAdvice.prompt.width && without.prompt.left === withAdvice.prompt.left && without.retry.height === withAdvice.retry.height);
    // At a narrow width the advice wraps inside the note.
    const wide = page.viewportSize();
    await page.setViewportSize({ width: 420, height: 900 });
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    const narrow = await noteParts();
    ok('at a narrow width the advice wraps inside the note', narrow.overflow <= 0 && narrow.advice.left >= narrow.note.left && narrow.advice.right <= narrow.note.right && narrow.advice.height > withAdvice.advice.height && narrow.retry.top >= narrow.advice.bottom);
    await front().locator('[data-view-id="session-note"]').screenshot({ path: require('path').join(process.argv[3] || '.', 'session-note-advice-narrow.png') });
    await page.setViewportSize(wide);
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await front().locator('[data-view-id="session-save-history"]').click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'save'));
    ok('saved history can be exported without the provider', await page.evaluate(() => __calls.find(c => c[0] === 'save')[1].includes('The original work is saved.')));
    await front().locator('[data-view-id="prompt"]').fill('Continue carefully.');
    await page.evaluate(() => { window.__holdRecovery = true; });
    await front().locator('[data-view-id="session-recover"]').click();
    const pendingRecovery = page.locator('dialog.switch-confirm[open]');
    await pendingRecovery.waitFor();
    ok('recovery opens its confirmation before saved-history facts return', await pendingRecovery.getByRole('button', { name: 'Continue', exact: true }).isDisabled());
    await page.keyboard.press('Escape');
    await page.evaluate(() => { __releaseRecovery({ revision: 'late-saved', warnings: ['Late warning'] }); window.__holdRecovery = false; });
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    ok('cancelled recovery ignores late facts and creates no session', await pendingRecovery.count() === 0 && await page.evaluate(() => !__calls.some(c => c[0] === 'recover')));
    await front().locator('[data-view-id="session-recover"]').click();
    await page.locator('dialog.switch-confirm[open]').waitFor();
    await page.locator('dialog.switch-confirm[open]').getByRole('button', { name: 'Continue', exact: true }).click();
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

    // A chat whose provider adds no advice has the note as it was, with no line for one.
    await page.evaluate(() => __emit('agent:session', { sessionId: 'codex:plain', provider: 'codex', cwd: 'C:/Test', open: false }));
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:plain' }));
    await front().locator('[data-view-id="session-note-title"]').getByText('Codex could not open this session.', { exact: true }).waitFor();
    ok('a failed open with no advice shows no line for it', await front().locator('[data-view-id="session-note-advice"]').isHidden() && (await front().locator('[data-view-id="session-note-advice"]').textContent()) === '');
  } finally { await t.close(); }
})();
