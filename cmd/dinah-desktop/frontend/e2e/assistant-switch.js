// The assistant's switch of a chat to another provider (assistant.js) is the
// chat window's own switch with no dialog (chat-applet.js, dinahChatSwitch):
// what the dialog would refuse is refused with its reason and nothing is
// asked, and a switch that goes through moves the window to the new chat as
// a switch by hand does.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 800 },
    app: () => {
      window.__calls = [];
      window.__ver = 0;
      const info = id => ({ sessionId: id, provider: 'codex', cwd: 'C:/Alpha', title: 'Chat S', open: true, working: false, queued: [] });
      return {
        State: async () => ({ workspace: 'C:/Alpha', recent: ['C:/Alpha'], workspaceRecords: [{ id: 'alpha', name: 'Alpha', executionPath: 'C:/Alpha', sources: [{ id: 'local', name: 'Local', kind: 'local-folder', path: 'C:/Alpha' }] }] }),
        AgentProviders: async () => [{ id: 'codex', name: 'Codex', installed: true, available: true }, { id: 'claude', name: 'Claude', installed: true, available: true }],
        Sessions: async () => [info('chat-s')],
        OpenSessionAt: async (w, id) => { window.__calls.push(['open', id]); return info(id); },
        Send: async () => {}, Queue: async () => {}, Steer: async () => {}, Answer: async () => {},
        PreviewSessionContinuation: async (id, provider) => {
          window.__calls.push(['preview', id, provider]);
          if (window.__blocked) return { sourceSessionId: id, provider, revision: '', context: '', history: [], warnings: [], blocked: 'An approval is waiting for an answer.' };
          return { sourceSessionId: id, provider, revision: 'r2', context: 'ctx', history: [], warnings: [] };
        },
        ContinueSessionWithProvider: async (id, provider, revision) => {
          window.__calls.push(['continue', id, provider, revision]);
          const next = { sessionId: id === 'chat-s' ? 'chat-c' : 'chat-d', provider, cwd: 'C:/Alpha', title: 'Chat S', open: true, working: false, queued: [] };
          window.__emit('agent:session', Object.assign({}, next, { stateGen: 'g', stateVer: ++window.__ver }));
          return { session: next, parentSessionId: id, history: [] };
        },
      };
    },
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    await p.goto(PAGE);
    await t.waitReady();
    ok('the page offers the quiet switch', await p.evaluate(() => typeof window.dinahChatSwitch === 'function'));
    // A chat that is not open in a window is opened first.
    await p.evaluate(() => { window.__blocked = true; });
    const refused = await p.evaluate(() => window.dinahChatSwitch('chat-s', 'claude'));
    const win = p.locator('.win[data-win-kind="chat-session"]');
    ok('the chat is opened in a window for the switch', await win.count() === 1);
    ok('a switch the product refuses is refused with its reason, with no dialog and no switch', refused.why === 'An approval is waiting for an answer.' && await p.locator('dialog.switch-confirm').count() === 0 && !(await p.evaluate(() => window.__calls.some(c => c[0] === 'continue'))), JSON.stringify(refused));
    ok('a chat that already uses the provider is not switched', (await p.evaluate(() => window.dinahChatSwitch('chat-s', 'codex'))).why === 'it already uses that provider');
    await p.evaluate(() => { window.__blocked = false; });
    const done = await p.evaluate(() => window.dinahChatSwitch('chat-s', 'claude'));
    ok('a switch that goes through gives the new chat, with the preview’s revision and no dialog', done.why === '' && done.id === 'chat-c' && await p.evaluate(() => window.__calls.some(c => c[0] === 'continue' && c[1] === 'chat-s' && c[2] === 'claude' && c[3] === 'r2')) && await p.locator('dialog.switch-confirm').count() === 0, JSON.stringify(done));
    await win.getByText('Switched from Codex to Claude.').waitFor();
    ok('the window takes the new chat and says so, as a switch by hand does', await win.count() === 1);
    const back = await p.evaluate(() => window.dinahChatSwitch('chat-c', 'codex'));
    ok('the new chat can be switched back, which is what Undo does', back.why === '' && back.id === 'chat-d' && await p.evaluate(() => window.__calls.some(c => c[0] === 'continue' && c[1] === 'chat-c' && c[2] === 'codex')), JSON.stringify(back));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
