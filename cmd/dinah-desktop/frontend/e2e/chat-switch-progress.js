// A switch visibly in progress (docs/plans/chat-workspaces-and-switching.md,
// While It Runs): the box rests, an indeterminate bar in its place names the
// stage in the engine's words, the transcript reads on; a failure says why
// there until dismissed, and the chat comes back as it was.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 800 },
    app: () => {
      window.__calls = [];
      return {
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'] }),
        AgentProviders: async () => [{ id: 'codex', name: 'Codex', installed: true, available: true }, { id: 'claude', name: 'Claude', installed: true, available: true }],
        Sessions: async () => [{ sessionId: 'chat-s', title: 'Switch In Progress', provider: 'codex', cwd: 'C:/Fixture', open: false }],
        OpenSessionAt: async (w, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: w, open: true, working: false, queued: [] }),
        Send: async () => {}, Queue: async () => {}, Steer: async () => {}, Answer: async () => {}, Cancel: async () => {},
        DismissSwitchFailure: async id => { window.__calls.push(['dismiss', id]); },
      };
    },
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    await p.goto(PAGE);
    await t.waitReady();
    await p.evaluate(() => window.dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'chat-s' }));
    const win = p.locator('.win[data-win-kind="chat-session"]');
    await win.locator('[data-view-id="prompt"]').waitFor();
    // The engine numbers the chat's information, so the suites' events must.
    await p.evaluate(() => {
      window.emitChat = fields => { window.__ver = (window.__ver || 0) + 1; window.__emit('agent:session', Object.assign({ sessionId: 'chat-s', stateGen: 'g', stateVer: window.__ver, provider: 'codex', cwd: 'C:/Fixture', open: true, working: false, queued: [] }, fields)); };
      window.__emit('agent:update', { sessionId: 'chat-s', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'A message to read while the switch runs.\n\n' } } });
      window.emitChat({ chatState: { life: 'open', window: true, exec: { state: 'working', why: 'Turn in progress' } }, switch: { kind: 'provider', to: 'claude', stage: 'summary', label: 'Building the summary of the conversation' } });
    });
    const prog = win.locator('.switch-progress');
    await prog.waitFor();
    ok('while it runs, the box rests and the bar names the stage in the engine’s words', (await prog.locator('.switch-stage').textContent()) === 'Building the summary of the conversation' && await prog.locator('.switch-running').isVisible() && await prog.locator('progress.progress').isVisible() && await prog.isVisible());
    // Title status retains the explanation when the body status line is absent.
    ok('title status retains the switch explanation', await win.locator('.win-execution .win-state[data-state="working"]').count() === 1 && /Switching to Claude./.test((await win.locator('.win-execution .win-state').getAttribute('title')) || ''), await win.locator('.win-execution .win-state').getAttribute('title'));
    await t.shot('switch-in-progress');
    ok('and the composer and its pickers are out of it', await win.locator('.composer').isHidden() && await win.locator('[data-command="provider"]').isHidden());
    ok('the transcript still reads and scrolls', await win.locator('[data-view-id="activity"]').getByText('A message to read while the switch runs.').isVisible());
    await p.evaluate(() => window.emitChat({ provider: 'claude', switch: { kind: 'provider', to: 'claude', stage: 'starting', label: 'Starting Claude' } }));
    await p.waitForFunction(() => { const el = document.querySelector('.win[data-win-kind="chat-session"] .switch-stage'); return el && el.textContent === 'Starting Claude'; });
    ok('the next stage is named as it begins', (await prog.locator('.switch-stage').textContent()) === 'Starting Claude');
    await p.evaluate(() => window.emitChat({ provider: 'claude' }));
    await p.waitForFunction(() => { const el = document.querySelector('.win[data-win-kind="chat-session"] .switch-progress'); return el && el.hidden; });
    ok('done, the box is back', await win.locator('.composer').isVisible());
    await p.evaluate(() => window.emitChat({ switch: { kind: 'provider', to: 'claude', stage: '', label: '', failed: 'The chat changed while connecting; review the provider switch again' } }));
    await p.waitForFunction(() => { const el = document.querySelector('.win[data-win-kind="chat-session"] .switch-progress'); const f = document.querySelector('.win[data-win-kind="chat-session"] .switch-failed'); return el && !el.hidden && f && !f.hidden; });
    ok('a failure says why there in the engine’s words, until dismissed', (await win.locator('.switch-failed').textContent()) === 'The chat changed while connecting; review the provider switch again' && await win.locator('.composer').isVisible());
    ok('as the danger notice it is, with its name and the way out', await win.locator('.switch-failure.notice.danger').isVisible() && /The switch to Claude failed/.test(await win.locator('.switch-failure-title').textContent()) && await win.locator('.switch-dismiss').isVisible());
    await t.shot('switch-failure');
    await win.locator('.switch-dismiss').click();
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'dismiss' && c[1] === 'chat-s'));
    ok('dismissing asks the engine to clear it', true);
    await p.evaluate(() => window.emitChat({}));
    await p.waitForFunction(() => { const el = document.querySelector('.win[data-win-kind="chat-session"] .switch-progress'); return el && el.hidden; });
    ok('and the chat comes back as it was', await win.locator('.composer').isVisible());
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
