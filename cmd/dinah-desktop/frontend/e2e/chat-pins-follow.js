// A chat's pinned button, saved for a session the chat has since been
// continued from, is given the chat's newest session at the start, with no
// window opened and no press of the button. A pinned chat that was not
// continued, and a pinned thing that is no chat, keep their buttons.
const { launch } = require('./lib/page');
const key = item => 'item-' + Buffer.from(JSON.stringify(item), 'utf8').toString('base64').replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
const WS = 'C:/Test';
const OLD = key(['chat-session', WS, 'codex:old']), HEAD = key(['chat-session', WS, 'claude:head']), SAME = key(['chat-session', WS, 'claude:same']), ROOM = key(['bus-room', '', 'a-group']);

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    args: [OLD, SAME, ROOM],
    app: () => {
      window.__calls = [];
      const ws = 'C:/Test';
      const enc = item => 'item-' + btoa(unescape(encodeURIComponent(JSON.stringify(item)))).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
      const pins = [enc(['chat-session', ws, 'codex:old']), enc(['chat-session', ws, 'claude:same']), enc(['bus-room', '', 'a-group'])];
      const state = { version: 1, values: { 'dinah-taskbar-pins-v1': { version: 1, pins: pins, order: pins.map(p => 'pin:' + p), references: pins.map(p => ({ id: p, provider: 'items' })) } } };
      return {
        ReadUIState: async () => JSON.parse(sessionStorage.getItem('pins-follow-state') || JSON.stringify(state)),
        SaveUIState: async value => sessionStorage.setItem('pins-follow-state', JSON.stringify(value)),
        State: async () => ({ workspace: ws, recent: [ws], defaultProvider: 'claude', catalog: [], defaults: {}, pinned: [] }),
        AgentProviders: async () => [{ id: 'claude', installed: true, available: true }],
        ResolveSessionContinuation: async id => { window.__calls.push(id); return id === 'codex:old' ? 'claude:head' : id; },
        SessionContinuation: async () => ({ history: [] }),
        SavedConversation: async () => ({ available: false }),
        OpenSessionAt: async (w, id) => { window.__calls.push('open ' + id); return { sessionId: id, provider: 'claude', title: 'A Chat', cwd: w, open: true, streamGen: 'g', streamHead: 0 }; },
        ChatEvents: async (id, gen) => ({ gen, head: 0, truncated: false, reset: false, events: [] })
      };
    },
  });
  const { page, ok } = t;
  try {
    await page.waitForFunction(() => window.dinahTaskbarPins && pudlWindows.taskbarState().pins.length === 3);
    await page.waitForFunction(old => !pudlWindows.taskbarState().pins.includes(old), OLD, { timeout: 8000 }).catch(() => {});
    const s = await page.evaluate(() => ({ pins: pudlWindows.taskbarState().pins, order: pudlWindows.taskbarState().order, calls: __calls, chats: window.dinahOpenWindows().filter(w => w.kind === 'chat-session').length }));
    ok('the continued chat\'s button is its newest session\'s', s.pins.includes(HEAD) && !s.pins.includes(OLD), JSON.stringify(s));
    ok('it keeps its place among the buttons', s.pins.indexOf(HEAD) === 0 && s.order.indexOf('pin:' + HEAD) === 0, JSON.stringify(s.order));
    ok('a chat that was not continued, and a group, keep their buttons', s.pins.includes(SAME) && s.pins.includes(ROOM) && s.pins.length === 3);
    ok('no chat was opened to find that out', s.chats === 0 && !s.calls.some(c => String(c).startsWith('open ')), JSON.stringify(s.calls));
    await page.evaluate(() => dinahUIState.flush());
    ok('the moved button is what is saved', await page.evaluate(([head, old]) => { const p = dinahUIState.get('dinah-taskbar-pins-v1'); return p.pins.includes(head) && !p.pins.includes(old); }, [HEAD, OLD]));
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
