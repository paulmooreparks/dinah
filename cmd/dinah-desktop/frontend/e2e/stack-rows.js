// A taskbar stack's chat rows (PUDL 0.96.5). The list widens to its longest
// title, up to PUDL's cap, so titles show whole where there is room and a
// title past the cap ends in an ellipsis. The New Chat draft is not yet a
// chat and has no state. Every chat row has a state: an
// execution-state chip for a running chat, and its lifecycle word, Closed
// or Unknown, as a plain badge for a chat with none. The chips end in one
// line before Close, and a change of state moves nothing.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 900 }, app: () => {
    const cwd = 'C:/Fixture';
    const titles = { a: 'PUDL Team Lead', b: 'PUDL Implementation Engineer', c: 'PUDL Build and Release', d: 'PUDL Verification', e: 'Unknown Chat' };
    window.__exec = { a: { state: 'working', why: 'Turn in progress' }, b: { state: 'idle', why: 'No work in progress' } };
    const entry = id => id === 'c' || id === 'd' ? { sessionId: id, bus: '', life: 'closed', window: false } : id === 'e' ? null : { sessionId: id, bus: '', life: 'open', window: true, exec: __exec[id] };
    const info = id => ({ sessionId: id, title: titles[id], cwd, provider: 'claude', open: id === 'a' || id === 'b' });
    let ver = 1;
    window.__snapshot = () => ({ chats: ['a', 'b', 'c', 'd'].map(entry), groups: [], stateGen: 'rows', stateVer: ++ver });
    window.__titles = titles;
    return { State: async () => ({ workspace: cwd, recent: [cwd], workspaceId: 'w1', workspaceRecords: [{ id: 'w1', name: 'Fixture', executionPath: cwd, sources: [] }] }),
      Sessions: async () => Object.keys(titles).map(info), SessionsAt: async () => Object.keys(titles).map(info),
      ReadUIState: async () => ({ version: 1, values: {} }), SaveUIState: async () => {},
      OpenSessionAt: async (w, id) => Object.assign(info(id), { open: true }), SavedConversation: async () => ({ available: false }),
      ChatStates: async () => __snapshot(), ChatLead: async () => null, Team: async () => null };
  } });
  const { page: p, ok } = t;
  // Whether text is cut as drawn: its natural width, from a range, against
  // the box it is drawn in. scrollWidth and clientWidth are whole pixels and
  // cannot see a cut of under a pixel.
  const read = () => p.evaluate(() => {
    const cut = el => { if (!el || !el.firstChild) return false; const r = document.createRange(); r.selectNodeContents(el); return r.getBoundingClientRect().width > el.getBoundingClientRect().width + 0.01; };
    const panel = document.querySelector('.win-stack-panel:popover-open'), pr = panel.getBoundingClientRect();
    return { width: Math.round(pr.width), rows: [...panel.querySelectorAll('.win-stack-row')].map(r => {
      const title = r.querySelector('.win-item-title'), mark = r.querySelector('.win-state'), close = r.querySelector('.win-stack-close');
      const words = mark && mark.querySelector('.win-state-words');
      return { title: title.textContent, cut: cut(title), markCut: cut(words), titleLeft: Math.round(title.getBoundingClientRect().left),
        mark: mark && !mark.hidden ? mark.textContent.trim() : '', state: mark ? mark.dataset.state || '' : '', lifecycle: !!(mark && mark.hasAttribute('data-lifecycle')),
        glyph: mark ? getComputedStyle(mark, '::before').content : '', markRight: mark ? Math.round(mark.getBoundingClientRect().right) : 0, closeLeft: Math.round(close.getBoundingClientRect().left) };
    }) };
  });
  const openStack = async () => {
    const stack = p.locator('[data-win-stack]').filter({ hasText: /Chats/ }).first();
    await stack.click();
    await p.locator('.win-stack-panel:popover-open').waitFor();
    await p.waitForTimeout(200);
  };
  try {
    for (const id of ['a', 'b', 'c', 'd', 'e']) await p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', path: id, workspace: 'w1' }), id);
    await p.evaluate(() => __emit('agent:chatstate', __snapshot()));
    await p.evaluate(() => document.querySelector('[data-win-layer]').setAttribute('data-win-taskbar-stacks', 'always'));
    await p.waitForTimeout(1500);
    await openStack();
    let s = await read();
    const row = name => s.rows.find(r => r.title === name);
    ok('titles that fit show whole, not cut by even a fraction of a pixel', ['PUDL Team Lead', 'PUDL Implementation Engineer', 'PUDL Build and Release', 'PUDL Verification'].every(n => row(n) && !row(n).cut), JSON.stringify(s.rows));
    ok('no chip is cut', s.rows.every(r => !r.markCut), JSON.stringify(s.rows.map(r => [r.mark, r.markCut])));
    ok('running chats show their state chips', row('PUDL Team Lead').mark === 'Working' && row('PUDL Team Lead').state === 'working' && row('PUDL Implementation Engineer').mark === 'Idle', JSON.stringify(s.rows));
    ok('closed chats say Closed, as a plain badge with no glyph', ['PUDL Build and Release', 'PUDL Verification'].every(n => row(n).mark === 'Closed' && row(n).lifecycle && row(n).glyph === 'none'), JSON.stringify(s.rows));
    ok('a chat whose state is not known says Unknown', row('Unknown Chat').mark === 'Unknown' && row('Unknown Chat').lifecycle, JSON.stringify(row('Unknown Chat')));
    // The New Chat window is a draft, not yet a chat, and has no state; its row keeps the empty column.
    const chats = s.rows.filter(r => r.title !== 'New Chat');
    ok('every chat row has a chip, and every chip ends in one line before Close', chats.length === 5 && chats.every(r => r.mark) && new Set(chats.map(r => r.markRight)).size === 1 && new Set(s.rows.map(r => r.closeLeft)).size === 1, JSON.stringify(s.rows.map(r => [r.title, r.mark, r.markRight, r.closeLeft])));
    await t.shot('stack-rows', { clip: await p.locator('.win-stack-panel:popover-open').boundingBox() });
    // A change of state moves nothing.
    const before = s;
    await p.evaluate(() => { __exec.b = { state: 'waiting', why: 'Needs your answer or approval' }; __emit('agent:chatstate', __snapshot()); });
    await p.waitForFunction(() => [...document.querySelectorAll('.win-stack-panel:popover-open .win-stack-row')].some(r => /Implementation/.test(r.textContent) && /Waiting/.test(r.textContent)));
    s = await read();
    ok('a change of state moves no title, chip end or Close', s.width === before.width && s.rows.every((r, i) => r.titleLeft === before.rows[i].titleLeft && r.markRight === before.rows[i].markRight && r.closeLeft === before.rows[i].closeLeft), JSON.stringify({ before: before.rows.map(r => [r.titleLeft, r.markRight, r.closeLeft]), after: s.rows.map(r => [r.titleLeft, r.markRight, r.closeLeft]) }));
    await p.keyboard.press('Escape');
    // A title longer than the cap allows ends in an ellipsis.
    await p.evaluate(() => { const key = pudlWindows.state().open.find(k => /ImEiXQ/.test(k)); pudlWindows.retitle(key, 'PUDL Implementation Engineer for the very long taskbar title that cannot fit'); });
    await openStack();
    s = await read();
    const long = s.rows.find(r => /very long/.test(r.title));
    ok('a long title widens the list to its cap and ends in an ellipsis there', long && long.cut && s.width <= 560 && s.width >= 540, JSON.stringify({ width: s.width, long }));
    await t.shot('stack-rows-long', { clip: await p.locator('.win-stack-panel:popover-open').boundingBox() });
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
