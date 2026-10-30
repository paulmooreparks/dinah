// WAITS ON PUDL: the menu bar keeps stale Actions content (message 1082).
// Dinah currently calls pudlMenubar.refresh after its command changes.
// This probe removes that call in the served fixture; DINAH_MENU_KEEP_REFRESH=1 supplies
// the control run. Promote it and remove the workaround when PUDL fixes it.
// A chat's menu changes when its turn or queue changes; the bar's copy must
// follow (Architecture, message 1082). Paul's case: a queue behind a running
// turn adds Drop the Queued Messages, and its going removes it again.
// Each read opens the window's menu from the bar and closes it after.
const { launch, PAGE } = require('../lib/page');
const fs = require('node:fs');
const path = require('node:path');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 800 },
    app: () => {
      window.__calls = [];
      window.__ver = 0;
      return {
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'] }),
        Sessions: async () => [{ sessionId: 'chat-s', title: 'The Menu Follows', provider: 'codex', cwd: 'C:/Fixture', open: false }],
        OpenSessionAt: async (w, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: w, open: true, working: false, queued: [], chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } }),
        Send: async () => {}, Queue: async () => {}, Steer: async () => {}, Answer: async () => {}, Cancel: async () => {},
      };
    },
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    // Test PUDL itself without Dinah's explicit rebuild after registry changes.
    const menus = fs.readFileSync(path.join(__dirname, '../../host-menus.js'), 'utf8');
    const refresh = 'if (window.pudlMenubar) window.pudlMenubar.refresh();';
    if (!menus.includes(refresh)) throw new Error('The menu-refresh workaround changed; review this probe');
    if (!process.env.DINAH_MENU_KEEP_REFRESH) await t.context.route('http://dinah.test/host-menus.js', route => route.fulfill({ body: menus.replace(refresh, ''), contentType: 'application/javascript' }));
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
    ok('at rest, the working commands are not there', !(await menuRows()).some(x => /Drop the Queued|^Stop$/.test(x)));
    await info({ working: true, queued: [] });
    ok('with the turn running, the menu offers Stop', (await menuRows()).some(x => /^Stop$/.test(x)), '');
    await info({ working: true, queued: ['What about the other file?'] });
    const queuedRows = await menuRows();
    ok('a queue behind it adds Drop the Queued Messages', queuedRows.some(x => /Drop the Queued Messages/.test(x)), JSON.stringify(queuedRows));
    await info({ working: false, queued: [] });
    const clearedRows = await menuRows();
    ok('sent or dropped, they go again', !clearedRows.some(x => /Drop the Queued Messages/.test(x)) && !clearedRows.some(x => /^Stop$/.test(x)), JSON.stringify(clearedRows.slice(0, 14)));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
