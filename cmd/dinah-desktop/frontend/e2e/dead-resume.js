// A session the provider no longer has (docs/plans/recurring-chat-faults.md).
// When the engine reports the open refused as not found, with the detail
// kind session-not-found, the note says the provider no longer has the
// session and that trying again would not help. It offers no Try Again
// and one recovery: Continue from Saved History when the saved history can
// be continued, otherwise New Chat. Save History… stays while there is
// history to export. Opening the chat again shows the same note, the page
// asks the engine once per opening and never by itself, and other open
// failures keep Try Again. A first turn that failed before the session
// was confirmed for reopening (session-unverified) gets the same actions,
// in words that do not say the provider lost it.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    state: { workspace: 'C:/Test', workspaces: ['C:/Test'], recent: ['C:/Test'], defaultProvider: 'claude', approvals: 'devin', promptSendKey: 'enter' },
    app: () => {
      window.__calls = [];
      const notFound = (id, recoverable) => {
        const e = new Error('Resource not found: ' + id);
        e.detail = { kind: 'session-not-found', provider: 'claude', sessionId: id, recoverable };
        return e;
      };
      const providers = [{ id: 'claude', name: 'Claude', installed: true, available: true, status: 'detected', path: 'C:/Tools/node.exe', claude: { adapterVersion: '0.87.0', adapterInstalled: true, canInstall: true, apiKey: false } }];
      return {
        AgentProviders: async () => providers,
        SessionContinuation: async () => ({ history: [] }),
        SavedConversation: async id => id === 'claude:bare' ? { available: false } : ({ available: true, complete: true, messages: [{ role: 'you', text: 'Build the release.' }, { role: 'agent', provider: 'claude', text: 'The release is built.' }] }),
        OpenSessionAt: async (ws, id) => {
          __calls.push(['open', id]);
          if (id === 'claude:dead') throw notFound(id, true);
          if (id === 'claude:bare') throw notFound(id, false);
          if (id === 'claude:first') { const e = new Error('You have reached your weekly limit'); e.detail = { kind: 'session-unverified', provider: 'claude', sessionId: id, recoverable: true }; throw e; }
          throw new Error('The agent stopped while opening the chat');
        },
        PreviewSessionRecovery: async id => { __calls.push(['preview-recovery', id]); return { messages: [] }; },
        PreviewSessionContinuation: async id => { __calls.push(['preview-continuation', id]); return { messages: [] }; },
      };
    },
    ready: () => window.dinahWorkbench && window.dinahEditor,
  });
  const { page: p, ok } = t;
  const front = () => p.locator('.win.active');
  const v = id => front().locator('[data-view-id="' + id + '"]');
  const buttons = () => front().evaluate(w => ['session-retry', 'session-open-with', 'session-recover', 'session-save-history', 'session-new'].map(id => {
    const b = w.querySelector('[data-view-id="' + id + '"]'); return b && b.getClientRects().length ? b.textContent.trim() + (b.classList.contains('btn-primary') ? '*' : '') : null;
  }).filter(Boolean));
  const opens = id => p.evaluate(id => __calls.filter(c => c[0] === 'open' && c[1] === id).length, id);
  try {
    await p.evaluate(() => ['claude:dead', 'claude:bare', 'claude:first', 'claude:other'].forEach(id => __emit('agent:session', { sessionId: id, provider: 'claude', cwd: 'C:/Test', title: id.slice(7), open: false })));
    // Not found, with saved history.
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:dead' }));
    await v('session-note-title').getByText('Claude no longer has this session.', { exact: true }).waitFor();
    const body = await v('session-note-body').innerText();
    ok('the note says the provider no longer has it and trying again would not help', /could not find it to reopen it, so trying again would not help/.test(body) && /Continue from Saved History carries on this chat in a fresh session/.test(body), body);
    let shown = await buttons();
    ok('it offers no Try Again and one recovery, Continue from Saved History, with Save History… to export', JSON.stringify(shown) === JSON.stringify(['Continue from Saved History*', 'Save History…']), JSON.stringify(shown));
    await t.shot('dead-resume-recoverable', { clip: await front().locator('[data-view-id="session-note"]').boundingBox() });
    // Try Again cannot be reached, and pressing it anyway resumes nothing.
    const before = await opens('claude:dead');
    await v('session-retry').evaluate(b => b.click());
    await p.waitForTimeout(400);
    ok('a Try Again press resumes nothing', await opens('claude:dead') === before, JSON.stringify({ before, after: await opens('claude:dead') }));
    // Reopening the chat shows the same note, and the page asks once.
    await p.evaluate(() => { const key = pudlWindows.state().top; pudlWindows.close(key); });
    await p.waitForTimeout(300);
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:dead' }));
    await v('session-note-title').getByText('Claude no longer has this session.', { exact: true }).waitFor();
    await p.waitForTimeout(800);
    ok('reopening shows the same note, and the page asked the engine once for that opening, never by itself', await opens('claude:dead') === before + 1 && JSON.stringify(await buttons()) === JSON.stringify(shown), JSON.stringify({ opens: await opens('claude:dead'), before }));
    await v('session-recover').click();
    await p.waitForFunction(() => __calls.some(c => /^preview-/.test(c[0])));
    ok('Continue from Saved History starts the recovery from saved history', true);
    await p.keyboard.press('Escape');
    // Not found, with no history to continue.
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:bare' }));
    await v('session-note-title').getByText('Claude no longer has this session.', { exact: true }).waitFor();
    await p.waitForTimeout(300);
    shown = await buttons();
    ok('with no history to continue, the one recovery is New Chat', JSON.stringify(shown) === JSON.stringify(['New Chat*']), JSON.stringify(shown));
    ok('and the note says to start a new chat', /Start a new chat to carry on/.test(await v('session-note-body').innerText()));
    await t.shot('dead-resume-bare', { clip: await front().locator('[data-view-id="session-note"]').boundingBox() });
    // A first turn that failed before the session was confirmed.
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:first' }));
    await v('session-note-title').getByText('The first turn in this session could not finish.', { exact: true }).waitFor();
    await p.waitForTimeout(300);
    const firstBody = await v('session-note-body').innerText();
    shown = await buttons();
    ok('an unverified first turn gets the same one recovery and no Try Again', JSON.stringify(shown) === JSON.stringify(['Continue from Saved History*', 'Save History…']), JSON.stringify(shown));
    ok('and its note does not say the provider lost or refused the session', /not confirmed for reopening/.test(firstBody) && !/no longer has|could not find|not found/i.test(firstBody), firstBody);
    const firstOpens = await opens('claude:first');
    await v('session-retry').evaluate(b => b.click());
    await p.waitForTimeout(300);
    ok('a Try Again press resumes nothing there either', await opens('claude:first') === firstOpens);
    await t.shot('dead-resume-unverified', { clip: await front().locator('[data-view-id="session-note"]').boundingBox() });
    // Any other failure keeps Try Again.
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:other' }));
    await v('session-note-title').getByText('Claude could not open this session.', { exact: true }).waitFor();
    await p.waitForTimeout(300);
    shown = await buttons();
    ok('another failure keeps Try Again first, with the other actions', shown[0] === 'Try Again*' && shown.includes('New Chat') && shown.includes('Continue from Saved History'), JSON.stringify(shown));
    const n = await opens('claude:other');
    await v('session-retry').click();
    await p.waitForFunction(n => __calls.filter(c => c[0] === 'open' && c[1] === 'claude:other').length > n, n);
    ok('and its Try Again asks the engine again', true);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
