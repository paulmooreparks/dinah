// A chat keeps its window's place when its provider session changes
// (chat-applet.js, follow). A chat recovered or switched has a new native
// session ID; the window follows it under the new key and keeps its
// position, size and place in the taskbar's order, whether the change comes
// from Continue from Saved History, from the engine opening the newest
// continuation of the session the window asked for, or from a restored
// window whose saved session has been continued (the start-up path). The
// window then shows the continuation's own saved copy. When the continuation
// already has a window of its own, that window comes forward and the other
// closes, leaving one window and none stuck loading. The chat is listed once.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 900 }, state: { workspace: 'C:/Test', recent: ['C:/Test'] }, app: () => {
    window.__calls = [];
    const history = [{ role: 'you', text: 'Build the release.' }, { role: 'agent', provider: 'claude', text: 'The release is built.' }];
    const info = (id, extra) => Object.assign({ sessionId: id, provider: 'claude', cwd: 'C:/Test', title: 'Chat ' + id.slice(7), open: true, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } }, extra);
    // Gates hold the engine's answers, so the window can be measured first.
    window.__gates = {};
    const gate = name => new Promise(r => { window.__gates[name] = r; });
    window.__heads = { 'claude:restored': 'claude:restored-next' };
    return {
      AgentProviders: async () => [{ id: 'claude', name: 'Claude', installed: true, available: true, status: 'detected' }],
      SessionContinuation: async () => ({ history: [] }),
      SavedConversation: async id => /dead/.test(id) ? { available: true, complete: true, recoverable: true, messages: history } :
        id === 'claude:handed-next' ? { available: true, complete: true, streamGen: 'g1', streamThrough: 0, messages: [{ role: 'you', text: 'Continue the handed chat.' }, { role: 'agent', provider: 'claude', text: 'The handed chat carries on here.' }] } : { available: false },
      ChatEvents: async (id, gen) => ({ gen, head: 0, truncated: false, reset: false, events: [] }),
      ResolveSessionContinuation: async id => { if (id === 'claude:restored' && !window.__gates.resolve) await gate('resolve'); return __heads[id] || id; },
      OpenSessionAt: async (ws, id) => {
        __calls.push(['open', id]);
        if (id === 'claude:handed') { await gate('open'); return info('claude:handed-next', { streamGen: 'g1', streamHead: 0 }); }
        if (id === 'claude:twin') return info('claude:twin-next');
        if (id === 'claude:dead') { const e = new Error('Resource not found: ' + id); e.detail = { kind: 'session-not-found', provider: 'claude', sessionId: id, recoverable: true }; throw e; }
        return info(id);
      },
      PreviewSessionRecovery: async () => ({ revision: 'saved-v1', warnings: [] }),
      RecoverSession: async () => ({ session: info('claude:dead-next'), history }),
    };
  } });
  const { page: p, ok } = t;
  const place = key => p.evaluate(key => {
    const w = document.querySelector('.win[data-win="' + CSS.escape(key) + '"]'), r = w && w.getBoundingClientRect(), s = pudlWindows.state();
    return w ? { rect: [Math.round(r.left), Math.round(r.top), Math.round(r.width), Math.round(r.height)], order: s.open.indexOf(key), mode: w.dataset.winMode } : null;
  }, key);
  const keyOf = id => p.evaluate(id => [...document.querySelectorAll('.win[data-win]')].map(w => w.dataset.win).find(k => { try { return JSON.parse(document.querySelector('.win[data-win="' + CSS.escape(k) + '"] .dinah-applet').dataset.item).path === id; } catch (e) { return false; } }) || '', id);
  const openChat = id => p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: id }), id);
  const settle = () => p.waitForTimeout(500);
  const same = (a, b) => a && b && JSON.stringify(a.rect) === JSON.stringify(b.rect) && a.order === b.order && a.mode === b.mode;
  try {
    await p.evaluate(() => ['claude:first', 'claude:restored', 'claude:handed', 'claude:dead', 'claude:last', 'claude:twin', 'claude:twin-next'].forEach(id => __emit('agent:session', { sessionId: id, provider: 'claude', cwd: 'C:/Test', title: 'Chat ' + id.slice(7), open: false })));
    // Windows on either side, so a window opened again would land elsewhere in the order.
    await openChat('claude:first'); await settle();
    // 1. A restored window whose saved session has been continued.
    const restoredKey = await openChat('claude:restored');
    await p.waitForFunction(() => !!window.__gates.resolve);
    await openChat('claude:last'); await settle();
    await p.evaluate(() => pudlWindows.raise(pudlWindows.state().open.find(k => /cmVzdG9yZWQ/.test(k)) || ''));
    const before1 = await place(restoredKey);
    await p.evaluate(() => window.__gates.resolve());
    await p.waitForFunction(() => [...document.querySelectorAll('.dinah-applet')].some(a => /restored-next/.test(a.dataset.item || '')));
    await settle();
    const after1 = await place(await keyOf('claude:restored-next'));
    ok('a restored window follows its continued session and keeps its place, size and taskbar slot', same(before1, after1) && !(await keyOf('claude:restored')), JSON.stringify({ before1, after1 }));
    // 2. The engine opens the newest continuation of the session asked for.
    const handedKey = await openChat('claude:handed');
    await p.waitForFunction(() => !!window.__gates.open);
    await settle();
    const before2 = await place(handedKey);
    await p.evaluate(() => window.__gates.open());
    await p.waitForFunction(() => [...document.querySelectorAll('.dinah-applet')].some(a => /handed-next/.test(a.dataset.item || '')));
    await settle();
    const after2 = await place(await keyOf('claude:handed-next'));
    ok('a window whose opening is handed to the continuation keeps its place, size and taskbar slot', same(before2, after2) && !(await keyOf('claude:handed')), JSON.stringify({ before2, after2 }));
    // The saved copy is judged once the display has settled: each of its lines
    // exactly once, still so after a further pause. A doubled or cleared copy
    // fails; a frame caught mid-draw does not. The counts go with a failure.
    const handedWin = p.locator('.win[data-win="' + (await keyOf('claude:handed-next')) + '"]');
    const copyLines = async () => [await handedWin.getByText('Continue the handed chat.').count(), await handedWin.getByText('The handed chat carries on here.').count()];
    for (let waited = 0, lines = await copyLines(); waited < 5000 && !(lines[0] === 1 && lines[1] === 1); waited += 100, lines = await copyLines()) await p.waitForTimeout(100);
    await settle();
    const lines = await copyLines();
    ok('and the window shows the continuation\'s own saved copy', lines[0] === 1 && lines[1] === 1, JSON.stringify({ person: lines[0], agent: lines[1] }));
    // 2b. The continuation already has a window: that one comes forward and
    // the other closes, and neither is left loading.
    const twinKey = await openChat('claude:twin-next'); await settle();
    await openChat('claude:twin');
    await p.waitForFunction(k => pudlWindows.state().top === k && !document.querySelector('.dinah-applet[data-item*=\'"claude:twin"\']'), twinKey);
    await settle();
    const twins = await p.evaluate(() => [...document.querySelectorAll('.win[data-win]')].filter(w => /claude:twin/.test(w.querySelector('.dinah-applet') ? w.querySelector('.dinah-applet').dataset.item || '' : '')).map(w => ({ item: w.querySelector('.dinah-applet').dataset.item, title: w.querySelector('.win-title').textContent })));
    ok('a chat whose continuation already has a window leaves that one window, in front and not loading', twins.length === 1 && /twin-next/.test(twins[0].item) && !/Loading/.test(twins[0].title) && (await p.evaluate(() => pudlWindows.state().top)) === twinKey, JSON.stringify(twins));
    // 3. Continue from Saved History.
    const deadKey = await openChat('claude:dead');
    const front = p.locator('.win.active');
    await front.locator('[data-view-id="session-recover"]').waitFor();
    await settle();
    const before3 = await place(deadKey);
    await front.locator('[data-view-id="session-recover"]').click();
    await p.locator('dialog.switch-confirm[open]').getByRole('button', { name: 'Continue', exact: true }).click();
    await p.waitForFunction(() => [...document.querySelectorAll('.dinah-applet')].some(a => /dead-next/.test(a.dataset.item || '')));
    await settle();
    const after3 = await place(await keyOf('claude:dead-next'));
    ok('Continue from Saved History keeps the window\'s place, size and taskbar slot', same(before3, after3), JSON.stringify({ before3, after3 }));
    // Each chat is listed once, under its current session.
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await settle();
    const listed = await p.evaluate(() => [...document.querySelectorAll('#sessions [data-session-row]')].map(r => r.dataset.sessionRow));
    ok('the Chats list shows each continued chat once, under its current session', ['claude:restored', 'claude:handed', 'claude:dead'].every(id => !listed.includes(id)) && listed.includes('claude:handed-next') && listed.includes('claude:dead-next'), JSON.stringify(listed));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
