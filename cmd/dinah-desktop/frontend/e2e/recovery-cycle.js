// Continue from Saved History, then closing and reopening, twice
// (docs/plans/recurring-chat-faults.md). The page's half of the cycle,
// against a backend that behaves as the engine now does: a session the
// provider no longer has is refused as session-not-found; recovery
// allocates a fresh session and starts its continuation at once, so the
// chat works; reopening by the new ID opens it, and reopening by the old
// ID opens its newest continuation. The page sends nothing itself, asks for
// recovery once, and after each restart the chat opens with its history and
// no error note. The engine's own test drives the same operations through
// its provider; this one checks what the person sees.
const { launch, PAGE } = require('./lib/page');
(async () => {
  // The backend's state, outside the page so it outlives a restart.
  const history = [{ role: 'you', text: 'Build the release.' }, { role: 'agent', provider: 'claude', text: 'The release is built.' }];
  const engine = { dead: new Set(['claude:dead']), newest: {}, recovered: 0, calls: [] };
  const t = await launch({ viewport: { width: 1440, height: 900 }, goto: false, state: { workspace: 'C:/Test', recent: ['C:/Test'] }, app: () => {
    const call = (op, ...args) => window.__engine(op, ...args);
    return {
      AgentProviders: async () => [{ id: 'claude', name: 'Claude', installed: true, available: true, status: 'detected' }],
      SessionContinuation: async () => ({ history: [] }),
      SavedConversation: async () => call('saved'),
      // An opened session's provider replays its conversation, the inherited
      // history included, as updates.
      OpenSessionAt: async (ws, id) => {
        const r = await call('open', id);
        if (r && r.error) { const e = new Error(r.error); e.detail = r.detail; throw e; }
        window.__emit('agent:updates', { sessionId: r.sessionId, updates: [
          { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Build the release.' } },
          { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'The release is built.' } }] });
        return r;
      },
      PreviewSessionRecovery: async (id, provider) => call('preview', id, provider),
      RecoverSession: async (id, provider, revision) => {
        const r = await call('recover', id, provider, revision);
        // The engine starts the continuation as the new session's first
        // prompt as soon as it allocates it: the chat works, then is idle.
        const working = w => window.__emit('agent:session', Object.assign({}, r.session, { working: w, chatState: { life: 'open', window: true, exec: { state: w ? 'working' : 'idle', why: w ? 'Turn in progress' : 'No work in progress' } } }));
        setTimeout(() => working(true), 0); setTimeout(() => working(false), 600);
        return r;
      },
      Send: async (id, text) => call('send', id, text),
    };
  } });
  const { page: p, ok } = t;
  const info = id => ({ sessionId: id, provider: 'claude', cwd: 'C:/Test', title: 'Release Chat', open: true, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } });
  await t.context.exposeBinding('__engine', (source, op, a, b, c) => {
    engine.calls.push([op, a, b, c]);
    switch (op) {
      case 'saved': return { available: true, complete: true, recoverable: true, messages: history };
      case 'preview': return { revision: 'saved-v1', warnings: [] };
      case 'recover': { const id = 'claude:fresh-' + (++engine.recovered); engine.newest[a] = id; Object.keys(engine.newest).forEach(k => { if (engine.newest[k] === a) engine.newest[k] = id; }); return { session: info(id), history }; }
      case 'open': {
        if (engine.newest[a]) return info(engine.newest[a]);
        if (engine.dead.has(a)) return { error: 'Resource not found: ' + a, detail: { kind: 'session-not-found', provider: 'claude', sessionId: a, recoverable: true } };
        return info(a);
      }
      default: return null;
    }
  });
  const front = () => p.locator('.win.active');
  const v = id => front().locator('[data-view-id="' + id + '"]');
  const restart = async () => { await p.goto(PAGE); await t.waitReady(); await p.evaluate(() => __emit('agent:session', { sessionId: 'claude:dead', provider: 'claude', cwd: 'C:/Test', title: 'Release Chat', open: false })); };
  const opensFine = async (id, label) => {
    await p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: id }), id);
    await p.waitForFunction(() => { const w = document.querySelector('.win.active'); return w && !w.querySelector('[data-view-id="session-note-title"]') || document.querySelector('.win.active [data-view-id="session-note"]').hidden; }, null, { timeout: 5000 }).catch(() => {});
    await p.waitForTimeout(500);
    const note = await v('session-note').isVisible().catch(() => false);
    const text = await front().innerText();
    ok(label, !note && /The release is built\./.test(text) && !/no longer has this session|could not open/.test(text), JSON.stringify({ note, text: text.slice(0, 300) }));
  };
  try {
    await restart();
    // The dead chat, and its one recovery.
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:dead' }));
    await v('session-note-title').getByText('Claude no longer has this session.', { exact: true }).waitFor();
    await v('session-recover').click();
    await p.locator('dialog.switch-confirm[open]').waitFor();
    await p.locator('dialog.switch-confirm[open]').getByRole('button', { name: 'Continue', exact: true }).click();
    await front().getByText('Continued from saved history with Claude.', { exact: true }).waitFor();
    const workingSeen = await p.waitForFunction(() => !!document.querySelector('.win.active .chat-working-chip .chat-state-working'), null, { timeout: 3000 }).then(() => true, () => false);
    ok('after Continue from Saved History the chat works on its continuation at once, in the new session', workingSeen && engine.calls.filter(c => c[0] === 'recover').length === 1);
    ok('the page sends nothing itself; the continuation is the engine\'s', !engine.calls.some(c => c[0] === 'send'));
    await p.waitForFunction(() => !!document.querySelector('.win.active .chat-working-chip .chat-state-idle'), null, { timeout: 3000 });
    for (const round of [1, 2]) {
      // Close the chat without any further prompt, then restart.
      await p.evaluate(() => { const key = pudlWindows.state().top; pudlWindows.close(key); });
      await restart();
      await opensFine('claude:fresh-1', 'round ' + round + ': after a restart, the chat opens by its new ID with its history and no error');
      await p.evaluate(() => { const key = pudlWindows.state().top; pudlWindows.close(key); });
      await opensFine('claude:dead', 'round ' + round + ': opening it by its old ID opens its continuation, with no error');
    }
    ok('recovery ran once in all, and the dead ID was never resumed after it', engine.calls.filter(c => c[0] === 'recover').length === 1 && engine.recovered === 1);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
