// The confirmation and the two moves a switch makes (docs/plans/chat-
// workspaces-and-switching.md): a blocked switch offers to stop the turn or
// nothing, and Continue in Another Workspace asks, confirms and moves
// (chat-applet.js).
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 800 },
    app: () => {
      window.__calls = [];
      window.__ver = 0;
      const facts = blocked => ({
        kind: 'provider', from: { provider: 'codex', providerName: 'Codex', model: 'gpt-sol', workspace: 'C:/Alpha', workspaceName: 'Alpha' },
        to: { provider: 'claude', providerName: 'Claude', model: 'opus-max', workspace: 'C:/Alpha', workspaceName: 'Alpha' },
        handoff: { messages: 20, whole: 8, summarised: 12, chars: 9000, budget: 80000, judged: true },
        carried: [], notCarried: [{ id: 'mode', name: 'Mode', value: 'Default' }],
        running: { turn: true, queued: 0, approvals: false, applets: 0, canStop: true },
      });
      const info = id => ({ sessionId: id, provider: 'codex', cwd: 'C:/Alpha', title: 'Chat S', open: true, working: false, queued: [] });
      window.emitInfo = fields => { window.__ver++; window.__emit('agent:session', Object.assign(info('chat-s'), fields, { stateGen: 'g', stateVer: window.__ver })); };
      return {
        State: async () => ({ workspace: 'C:/Alpha', recent: ['C:/Alpha', 'C:/Beta'], workspaceRecords: [
          { id: 'alpha', name: 'Alpha', executionPath: 'C:/Alpha', sources: [{ id: 'local', name: 'Local', kind: 'local-folder', path: 'C:/Alpha' }] },
          { id: 'beta', name: 'Beta Workspace', executionPath: 'C:/Beta', sources: [{ id: 'local', name: 'Local', kind: 'local-folder', path: 'C:/Beta' }] }] }),
        AgentProviders: async () => [{ id: 'codex', name: 'Codex', installed: true, available: true }, { id: 'claude', name: 'Claude', installed: true, available: true }],
        Sessions: async () => [info('chat-s')],
        OpenSessionAt: async (w, id) => info(id),
        Send: async () => {}, Queue: async () => {}, Steer: async () => {}, Answer: async () => {},
        Cancel: async id => { window.__calls.push(['cancel', id]); setTimeout(() => window.__emit('agent:turn', { sessionId: id, stopReason: 'cancelled' }), 0); },
        PreviewSessionContinuation: async (id, provider) => {
          window.__calls.push(['preview', id, provider]);
          if (window.__blocked) return { sourceSessionId: id, provider, revision: 'r1', context: '', history: [], warnings: [], facts: facts(true), blocked: 'A turn is still running.' };
          return { sourceSessionId: id, provider, revision: 'r2', context: 'ctx', history: [], warnings: [], facts: Object.assign(facts(false), { running: { turn: false, queued: 0, approvals: false, applets: 0, canStop: false } }) };
        },
        ContinueSessionWithProvider: async (id, provider, revision) => {
          window.__calls.push(['continue', id, provider, revision]);
          const next = { sessionId: 'chat-c', provider, cwd: 'C:/Alpha', title: 'Chat S', open: true, working: false, queued: [] };
          window.__emit('agent:session', Object.assign({}, next, { stateGen: 'g', stateVer: ++window.__ver }));
          return { session: next, parentSessionId: id, history: [] };
        },
        PreviewSessionWorkspaceMove: async (id, folder) => { window.__calls.push(['preview-move', id, folder]); return { sourceSessionId: id, provider: 'codex', revision: 'm1', context: '', history: [], warnings: [], facts: { kind: 'workspace', from: { provider: 'codex', providerName: 'Codex', model: 'gpt-sol', workspace: 'C:/Alpha', workspaceName: 'Alpha' }, to: { provider: 'codex', providerName: 'Codex', model: 'gpt-sol', workspace: folder, workspaceName: 'Beta Workspace' }, handoff: { messages: 20, whole: 8, summarised: 12, chars: 9000, budget: 80000, judged: false }, carried: [{ id: 'model', name: 'Model', value: 'gpt-sol' }], notCarried: [{ id: 'mode', name: 'Mode', value: 'Bypass permissions' }], running: { turn: false, queued: 0, approvals: false, applets: 0, canStop: false } } }; },
        ContinueSessionInWorkspace: async (id, folder, revision) => {
          window.__calls.push(['move', id, folder, revision]);
          const next = { sessionId: 'chat-m', provider: 'codex', cwd: folder, title: 'Chat S', open: true, working: false, queued: [] };
          window.__emit('agent:session', Object.assign({}, next, { stateGen: 'g', stateVer: ++window.__ver }));
          return { session: next, parentSessionId: id, history: [], optionsNotCarried: ['Mode'] };
        },
      };
    },
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    await p.goto(PAGE);
    await t.waitReady();
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Alpha', path: 'chat-s' }));
    const win = p.locator('.win[data-win-kind="chat-session"]');
    await win.locator('[data-view-id="prompt"]').waitFor();

    // The blocked switch: the confirmation offers to stop the turn, else Cancel alone.
    await p.evaluate(() => { window.__blocked = true; });
    // The provider switch starts from the composer's own menu, as every suite drives it.
    const { choose } = require('./lib/commands');
    await choose(p, win, 'provider', 'Claude');
    const conf = p.locator('dialog.switch-confirm[open]');
    await conf.waitFor();
    ok('a blocked switch says so and offers to stop the turn, not to continue', /A turn is still running/.test(await conf.textContent()) && (await conf.getByRole('button', { name: 'Stop the Turn and Continue' }).count()) === 1 && (await conf.getByRole('button', { name: 'Continue', exact: true }).count()) === 0);
    await t.shot('switch-blocked-confirm');
    ok('and it shows the judged summary and what does not carry', /a summary the judge wrote of 12 earlier messages, plus the 8 recent ones in full/.test(await conf.textContent()) ? true : /a summary the judge wrote of 12 earlier messages, plus the 8 recent in full/.test(await conf.textContent()), (await conf.textContent()).slice(0, 300));
    await conf.getByRole('button', { name: 'Stop the Turn and Continue' }).click();
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'cancel'));
    ok('the stop stops the turn first', true);
    // After the turn ends the preview is asked again, and the switch continues.
    await p.waitForFunction(() => window.__calls.filter(c => c[0] === 'preview').length === 2);
    await p.evaluate(() => { window.__blocked = false; });
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'continue' && c[1] === 'chat-s' && c[2] === 'claude'));
    ok('and when it ends the move goes ahead', true);
    await win.getByText('Switched from Codex to Claude.').waitFor();

    // The workspace move: the command from the window's menu, the chooser, the move.
    await p.evaluate(() => window.dinahCommands.rows('actions'));
    await p.evaluate(() => {
      const rows = [...document.querySelectorAll('.menu-action')].filter(() => true);
      // The window's Actions menu opens with its rows; the row's command is pressed as the bar's act() would.
    });
    // Open the front window's Actions menu and press the row's command.
    await p.getByRole('menuitem', { name: 'Actions' }).click();
    await p.waitForFunction(() => [...document.querySelectorAll('.menu-action')].some(b => /Continue in Another Workspace/.test(b.textContent) && (b.offsetWidth || b.offsetHeight)));
    await p.evaluate(() => [...document.querySelectorAll('.menu-action')].find(b => /Continue in Another Workspace/.test(b.textContent) && (b.offsetWidth || b.offsetHeight))._item.run());
    const pick = p.locator('dialog.dinah-new-chat-workspace[open]');
    await pick.waitFor();
    ok('the chooser offers the other workspace, not the chat’s own', /Beta Workspace/.test(await pick.textContent()) && !/Alpha \(C/.test(await pick.textContent()), await pick.textContent());
    await pick.locator('select').selectOption('0');
    await pick.getByRole('button', { name: 'Continue' }).click();
    const conf2 = p.locator('dialog.switch-confirm[open]');
    await conf2.waitFor();
    // The chat after the provider switch is its continuation, chat-c.
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'preview-move' && c[1] === 'chat-c' && c[2] === 'C:/Beta'));
    ok('the move’s confirmation is the workspace one', (await conf2.locator('.dialog-title').textContent()) === 'Continue in Beta Workspace?' && /its files, skills and applet actions are that workspace.s/.test(await conf2.textContent()));
    await t.shot('switch-move-confirm');
    ok('and says the authority settings left behind', /Mode \(Bypass permissions\)/.test(await conf2.textContent()));
    await conf2.getByRole('button', { name: 'Continue' }).click();
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'move' && c[1] === 'chat-c' && c[2] === 'C:/Beta' && c[3] === 'm1'));
    ok('and the move goes to its folder with the preview’s revision', true);
    await win.getByText('Continued in Beta Workspace.').waitFor();
    ok('the chat says where it went', true);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
