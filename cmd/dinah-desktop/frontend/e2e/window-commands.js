// An agent's questions about the open windows (applet_window_commands.go):
// the Go side sends page:ask and the page answers through AnswerPage. An
// agent sees only its workspace's windows, lists a window's commands as data
// with their state, runs one by its id as pressing it would, and is refused
// a command that cannot run now, one that would destroy something, and one
// in another workspace's window. The window here is a plain one with test
// commands, so the suite follows the bridge, not any one applet.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    state: { workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'],
      workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [{ id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] },
    app: () => ({ AnswerPage: async (id, result, err) => { window.__answers[id] = { result: result ? JSON.parse(result) : null, err }; } }),
    init: () => { window.__answers = {}; },
    ready: 'workbench',
  });
  const p = t.page, ok = t.ok;
  try {
    const key = await p.evaluate(async () => {
      window.dinahWorkbench.register('test-commands', { title: 'Test Commands', glyph: 'app', init: function (root) {
        window.__flips = 0;
        window.pudlCommands.register(root, [
          { id: 'flip', label: 'Flip', shortcut: 'Ctrl+Alt+F', run: function () { window.__flips++; } },
          { id: 'later', label: 'Later', disabled: function () { return true; }, reason: 'Not now.', run: function () { window.__flips += 100; } },
          { id: 'wipe', label: 'Wipe Everything', danger: true, run: function () { window.__destroyed = true; } }
        ]);
        return {};
      } });
      return window.dinahWorkbench.open({ kind: 'test-commands', workspace: 'w1', path: 'one', title: 'Test Commands' });
    });
    await p.locator('.win[data-win="' + key + '"]').waitFor();
    await p.evaluate(k => window.dinahWorkbench.ready(k).catch(() => {}), key);
    const ask = (op, args) => p.evaluate(async ([op, args]) => {
      const id = 'q' + Math.random();
      window.__emit('page:ask', { id, op, args });
      for (let i = 0; i < 100 && !window.__answers[id]; i++) await new Promise(r => setTimeout(r, 20));
      return window.__answers[id];
    }, [op, args]);
    const mine = { workspace: 'C:\\Fixture\\', workspaceId: 'w1' };

    const wins = await ask('windows', mine);
    const win = wins.result && wins.result.windows.find(w => w.kind === 'test-commands');
    ok('the workspace\'s windows are listed for an agent, with the window in front', win && win.window === key && win.front === true && win.title.length > 0, JSON.stringify(wins));
    const other = await ask('windows', { workspace: 'C:/Elsewhere', workspaceId: 'w2' });
    ok('another workspace\'s agent sees none of them', other.result && !other.result.windows.some(w => w.window === key), JSON.stringify(other));

    const cmds = await ask('commands', Object.assign({ window: key }, mine));
    const byId = Object.fromEntries(((cmds.result && cmds.result.commands) || []).map(c => [c.id, c]));
    ok('a window\'s commands are listed through the bridge, with their keys and state', byId.flip && byId.flip.shortcut === 'Ctrl+Alt+F' && byId.later && byId.later.disabled === true && byId.later.reason === 'Not now.', JSON.stringify(cmds).slice(0, 300));

    const ran = await ask('run', Object.assign({ window: key, command: 'flip' }, mine));
    ok('a command run through the bridge runs as pressing it would', ran.result && ran.result.ran === 'Flip' && await p.evaluate(() => window.__flips) === 1, JSON.stringify(ran));
    const later = await ask('run', Object.assign({ window: key, command: 'later' }, mine));
    ok('a command that cannot run now is refused, saying so', !!later.err && await p.evaluate(() => window.__flips) === 1, JSON.stringify(later));
    const elsewhere = await ask('run', { window: key, command: 'flip', workspace: 'C:/Elsewhere', workspaceId: 'w2' });
    ok('another workspace cannot run the window\'s commands', !!elsewhere.err && !elsewhere.result && await p.evaluate(() => window.__flips) === 1, JSON.stringify(elsewhere));
    const danger = await ask('run', Object.assign({ window: key, command: 'wipe' }, mine));
    ok('a command that would destroy something is left to the person', !!danger.err && !(await p.evaluate(() => window.__destroyed)), JSON.stringify(danger));
    const missing = await ask('run', Object.assign({ window: key, command: 'no-such-command' }, mine));
    ok('an unknown command is refused, saying so', !!missing.err && /no command/.test(missing.err), JSON.stringify(missing));
  } finally {
    await t.close();
  }
})();
