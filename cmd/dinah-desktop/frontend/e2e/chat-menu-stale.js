// Actions follows the current command table without a menu-refresh workaround.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 800 },
    app: () => {
      window.__calls = [];
      window.__ver = 0;
      window.__menuInfo = fields => __emit('agent:session', Object.assign({ sessionId: 'chat-s', stateGen: 'g', stateVer: ++__ver, provider: 'codex', cwd: 'C:/Fixture', open: true,
        chatState: { life: 'open', window: true, exec: { state: 'working', why: 'Turn in progress' } } }, fields));
      return {
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'] }),
        Sessions: async () => [{ sessionId: 'chat-s', title: 'The Menu Follows', provider: 'codex', cwd: 'C:/Fixture', open: false }],
        OpenSessionAt: async (w, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: w, open: true, working: false, queued: [], chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } }),
        Send: async () => {}, Queue: async () => {}, Steer: async () => {}, Answer: async () => {}, Cancel: async () => {},
        Unqueue: async () => { __calls.push('Unqueue'); __menuInfo({ working: true, queued: [] }); },
      };
    },
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    await p.goto(PAGE);
    await t.waitReady();
    await p.evaluate(() => window.dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'chat-s' }));
    await p.locator('.win[data-win-kind="chat-session"] [data-view-id="prompt"]').waitFor();
    async function info(fields) {
      await p.evaluate(f => new Promise(resolve => {
        document.addEventListener('dinah:commands-change', resolve, { once: true });
        window.__emit('agent:session', Object.assign({ sessionId: 'chat-s', stateGen: 'g', stateVer: ++window.__ver, provider: 'codex', cwd: 'C:/Fixture', open: true,
        chatState: { life: 'open', window: true, exec: { state: f.working ? 'working' : 'idle', why: '' } } }, f));
      }), fields);
      await p.waitForFunction(f => {
        const win = document.querySelector('.win[data-chat-current]');
        return !!win.querySelector('.win-execution .win-state[data-state="' + (f.working ? 'working' : 'idle') + '"]') && win.querySelector('[data-view-id="queue"]').hidden === !f.queued.length;
      }, fields);
      // Settle the host's scheduled command redraw before reopening its menu.
      await p.evaluate(() => window.dinahCommands.flush());
    }
    // The window's Actions menu, opened from the bar and closed after reading.
    async function menuRows() {
      await p.evaluate(() => {
        const title = [...document.querySelectorAll('.menubar-title')].find(b => /^Actions$/.test(b.textContent));
        if (title) title.click();
      });
      await p.waitForFunction(() => [...document.querySelectorAll('.menu-action')].some(b => (b.offsetWidth || b.offsetHeight) && /Stop|Drop the Queued|Rename Chat/.test(b.textContent)));
      const rows = await p.evaluate(() => [...document.querySelectorAll('.menu-action')].filter(b => (b.offsetWidth || b.offsetHeight)).map(b => b.textContent.trim()));
      await p.evaluate(() => new Promise(resolve => {
        const panel = document.getElementById('menubar-panel');
        panel.addEventListener('toggle', function closed(event) {
          if (event.newState !== 'closed') return;
          panel.removeEventListener('toggle', closed);
          resolve();
        });
        const title = [...document.querySelectorAll('.menubar-title')].find(b => /^Actions$/.test(b.textContent));
        title.click();
      }));
      return rows;
    }
    async function checkCoverage(rows) {
      const commands = await p.evaluate(() => {
        const win = document.querySelector('.win[data-chat-current]');
        return pudlCommands.list(win).filter(c => !c.hidden && pudlCommands.find(c.id, win).actions).map(c => c.label);
      });
      ok('every visible Actions command is exported from the table', commands.every(label => rows.some(row => row.includes(label))), JSON.stringify({ commands, rows }));
    }
    const idleRows = await menuRows();
    ok('at rest, the working commands are not there', !idleRows.some(x => /Drop the Queued|^Stop$/.test(x)));
    await checkCoverage(idleRows);
    await info({ working: true, queued: [] });
    ok('with the turn running, the menu offers Stop', (await menuRows()).some(x => /^Stop$/.test(x)), '');
    await info({ working: true, queued: ['What about the other file?'] });
    const queuedRows = await menuRows();
    ok('a queue behind it adds Drop the Queued Messages', queuedRows.some(x => /Drop the Queued Messages/.test(x)), JSON.stringify(queuedRows));
    await checkCoverage(queuedRows);
    // Keep Actions open while the queue changes, and press the current row.
    await p.evaluate(() => [...document.querySelectorAll('.menubar-title')].find(b => b.textContent.trim() === 'Actions').click());
    await p.locator('#menubar-panel .menu-action').filter({ hasText: 'Drop the Queued Messages' }).click();
    await p.waitForFunction(() => __calls.includes('Unqueue'));
    await p.waitForFunction(() => document.querySelector('.win[data-chat-current] [data-view-id="queue"]').hidden);
    await p.evaluate(() => dinahCommands.flush());
    ok('the offered command drops the queued messages', (await menuRows()).every(row => !row.includes('Drop the Queued Messages')));
    await p.evaluate(() => [...document.querySelectorAll('.menubar-title')].find(b => b.textContent.trim() === 'Actions').click());
    await info({ working: true, queued: ['A new queued message'] });
    await p.locator('#menubar-panel .menu-action').filter({ hasText: 'Drop the Queued Messages' }).waitFor();
    ok('an open Actions menu gains the queued command', true);
    await info({ working: true, queued: [] });
    await p.locator('#menubar-panel .menu-action').filter({ hasText: 'Drop the Queued Messages' }).waitFor({ state: 'detached' });
    ok('an open Actions menu removes the dropped command', true);
    await p.keyboard.press('Escape');
    await info({ working: false, queued: [] });
    const clearedRows = await menuRows();
    ok('sent or dropped, they go again', !clearedRows.some(x => /Drop the Queued Messages/.test(x)) && !clearedRows.some(x => /^Stop$/.test(x)), JSON.stringify(clearedRows.slice(0, 14)));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
