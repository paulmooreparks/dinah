// The host's Chat menu follows the command table of the chat in front, open or closed.
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
        return !!win.querySelector('.chat-working-chip .chat-state-' + (f.working ? 'working' : 'idle')) && win.querySelector('[data-view-id="queue"]').hidden === !f.queued.length;
      }, fields);
      // Settle the host's scheduled command redraw before reopening its menu.
      await p.evaluate(() => window.dinahCommands.flush());
    }
    // The host's Chat menu, opened from the bar and closed after reading.
    // Its rows are always there, and one that cannot be used is greyed.
    const chatTitle = () => [...document.querySelectorAll('.menubar-title')].find(b => b.textContent.trim() === 'Chat');
    async function menuRows() {
      await p.evaluate(`(${chatTitle})().click()`);
      await p.waitForFunction(() => [...document.querySelectorAll('#menubar-panel > .menu-action')].some(b => (b.offsetWidth || b.offsetHeight) && /Rename/.test(b.textContent)));
      const rows = await p.evaluate(() => [...document.querySelectorAll('#menubar-panel > .menu-action')].filter(b => (b.offsetWidth || b.offsetHeight))
        .map(b => ({ label: (b.querySelector('.menubar-words') || b).textContent.trim(), greyed: b.disabled || b.getAttribute('aria-disabled') === 'true' })));
      await p.evaluate(`new Promise(resolve => {
        const panel = document.getElementById('menubar-panel');
        panel.addEventListener('toggle', function closed(event) {
          if (event.newState !== 'closed') return;
          panel.removeEventListener('toggle', closed);
          resolve();
        });
        (${chatTitle})().click();
      })`);
      return rows;
    }
    const usable = (rows, label) => rows.some(row => row.label === label && !row.greyed);
    const greyed = (rows, label) => rows.some(row => row.label === label && row.greyed);
    const idleRows = await menuRows();
    ok('at rest, Stop and Drop the Queued Messages are greyed', greyed(idleRows, 'Stop') && greyed(idleRows, 'Drop the Queued Messages'), JSON.stringify(idleRows));
    ok('the chat in front has its rows to use', usable(idleRows, 'Rename…') && usable(idleRows, 'Export…') && usable(idleRows, 'Delete…'), JSON.stringify(idleRows));
    ok('the window adds no menu of its own', await p.evaluate(() => !document.querySelector('nav.menubar .menubar-front .menubar-title')));
    await info({ working: true, queued: [] });
    ok('with the turn running, the menu offers Stop', usable(await menuRows(), 'Stop'), '');
    await info({ working: true, queued: ['What about the other file?'] });
    const queuedRows = await menuRows();
    ok('a queue behind it offers Drop the Queued Messages', usable(queuedRows, 'Drop the Queued Messages'), JSON.stringify(queuedRows));
    // Keep Chat open while the queue changes, and press the current row.
    const dropRow = () => p.locator('#menubar-panel > .menu-action').filter({ hasText: 'Drop the Queued Messages' });
    const dropGreyed = () => dropRow().evaluate(b => b.disabled || b.getAttribute('aria-disabled') === 'true');
    await p.evaluate(`(${chatTitle})().click()`);
    await dropRow().click();
    await p.waitForFunction(() => __calls.includes('Unqueue'));
    await p.waitForFunction(() => document.querySelector('.win[data-chat-current] [data-view-id="queue"]').hidden);
    await p.evaluate(() => dinahCommands.flush());
    ok('the offered command drops the queued messages', greyed(await menuRows(), 'Drop the Queued Messages'));
    await p.evaluate(`(${chatTitle})().click()`);
    await dropRow().waitFor();
    await info({ working: true, queued: ['A new queued message'] });
    await p.waitForFunction(() => { const b = [...document.querySelectorAll('#menubar-panel > .menu-action')].find(b => /Drop the Queued Messages/.test(b.textContent)); return b && !b.disabled && b.getAttribute('aria-disabled') !== 'true'; });
    ok('an open Chat menu offers the queued command', !(await dropGreyed()));
    await info({ working: true, queued: [] });
    await p.waitForFunction(() => { const b = [...document.querySelectorAll('#menubar-panel > .menu-action')].find(b => /Drop the Queued Messages/.test(b.textContent)); return b && (b.disabled || b.getAttribute('aria-disabled') === 'true'); });
    ok('an open Chat menu greys the dropped command', await dropGreyed());
    await p.keyboard.press('Escape');
    await info({ working: false, queued: [] });
    const clearedRows = await menuRows();
    ok('sent or dropped, they are greyed again', greyed(clearedRows, 'Drop the Queued Messages') && greyed(clearedRows, 'Stop'), JSON.stringify(clearedRows.slice(0, 14)));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
