// A turn the person starts after a finished turn (docs/design/
// background-activity.md). When the engine reports the chat Working, with a
// long-running tool call still pending in its transcript, the chat shows
// Working in its title, the window's PUDL state the taskbar uses, the
// taskbar's stack row, the Chats list and the team group's Members
// dropdown, and keeps showing it while the call runs; the call's own chip
// keeps the call's status. When the turn ends the chat is Idle everywhere.
// The page takes the state from the engine alone: the pending call does not
// make it Working by itself.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1200, height: 900 }, app: () => {
    const cwd = 'C:/Fixture';
    window.__st = { lead: 'idle', alpha: 'idle' };
    const why = { idle: 'No work in progress', working: 'Turn in progress' };
    const exec = id => ({ state: __st[id], why: why[__st[id]] });
    const ref = (id, title) => ({ id: 'bus-' + id, chat: id, title, life: 'open', window: true, exec: exec(id) });
    const room = () => ({ name: 'team-a', title: 'Team A', lead: 'bus-lead', members: [ref('lead', 'Team Lead'), ref('alpha', 'Alpha')] });
    const info = id => ({ sessionId: id, title: id === 'lead' ? 'Team Lead' : 'Alpha', cwd, provider: 'claude', open: true, working: __st[id] === 'working', queued: [], chatState: { life: 'open', window: true, exec: exec(id) } });
    let ver = 1;
    const snapshot = () => ({ chats: ['lead', 'alpha'].map(id => ({ sessionId: id, bus: 'bus-' + id, life: 'open', window: true, exec: exec(id) })), groups: [{ name: 'team-a', counts: { top: __st.lead === 'working' ? 'working' : 'idle', working: __st.lead === 'working' ? 1 : 0, idle: __st.lead === 'working' ? 1 : 2 } }], stateGen: 'fg', stateVer: ++ver });
    // The engine's events for the lead's state: its session and the shared snapshot.
    window.__engine = state => { __st.lead = state; __emit('agent:session', info('lead')); __emit('agent:chatstate', snapshot()); };
    return { State: async () => ({ workspace: cwd, recent: [cwd], workspaceId: 'w1', workspaceRecords: [{ id: 'w1', name: 'Fixture', executionPath: cwd, sources: [] }] }),
      Sessions: async () => ['lead', 'alpha'].map(info), SessionsAt: async () => ['lead', 'alpha'].map(info),
      ReadUIState: async () => ({ version: 1, values: {} }), SaveUIState: async () => {},
      OpenSessionAt: async (w, id) => info(id), ChatLead: async () => null, Team: async () => null,
      ChatStates: async () => snapshot(),
      BusOverview: async () => ({ inbox: [], rooms: [room()], workspaces: [], links: [] }), BusResolveGroup: async () => ({ state: 'resolved', room: room() }),
      BusGroupMessages: async () => [], BusMarkGroupRead: async () => {} };
  } });
  const { page: p, ok } = t;
  const settle = () => p.evaluate(() => new Promise(r => setTimeout(() => requestAnimationFrame(() => requestAnimationFrame(r)), 100)));
  try {
    const leadKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', path: 'lead', workspace: 'w1' }));
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', path: 'alpha', workspace: 'w1' }));
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.evaluate(() => dinahBus.openGroup('team-a', { message: 0 }));
    await p.evaluate(() => document.querySelector('[data-win-layer]').setAttribute('data-win-taskbar-stacks', 'always'));
    const win = p.locator('[data-win="' + leadKey + '"]');
    await win.locator('.chat-working-chip .chat-state-idle').waitFor({ state: 'attached' });
    const read = () => p.evaluate(key => {
      const w = document.querySelector('[data-win="' + key + '"]'), chip = w.querySelector('.chat-working-chip .badge');
      const row = [...document.querySelectorAll('#sessions .md-item')].find(a => /Team Lead/.test(a.textContent));
      const rowChip = row && row.querySelector('.chat-state-slot .badge:not(.chat-state-reserve)');
      const tools = w.querySelectorAll('.tool-row .badge'), tool = tools[tools.length - 1];
      return { title: chip.textContent.trim() + '|' + (chip.dataset.state || ''), win: w.dataset.winState, list: rowChip ? rowChip.textContent.trim() : '', listGroup: row && row.closest('.browser-group') ? row.closest('.browser-group').querySelector('.browser-group-title').textContent.trim() : '', tool: tool ? tool.textContent.trim() : '' };
    }, leadKey);
    const members = async () => {
      const g = p.locator('.win.active[data-win-kind="bus-room"]');
      await p.evaluate(() => dinahBus.openGroup('team-a', { message: 0 }));
      await g.getByRole('button', { name: /^Members/ }).click();
      const panel = g.locator('.members-menu:popover-open');
      await panel.waitFor();
      const chip = (await panel.locator('[data-group-member="bus-lead"] .badge.chat-state:not(.chat-state-reserve)').textContent()).trim();
      await p.keyboard.press('Escape');
      return chip;
    };
    const stackRow = async () => {
      const stack = p.locator('[data-win-stack]').filter({ hasText: /Chats/ }).first();
      await stack.click();
      await p.locator('.win-stack-panel:popover-open').waitFor();
      const words = await p.evaluate(() => { const r = [...document.querySelectorAll('.win-stack-panel:popover-open .win-stack-row')].find(r => /Team Lead/.test(r.textContent)); const b = r && r.querySelector('.win-state'); return b ? b.textContent.trim() + '|' + (b.dataset.state || '') : ''; });
      await p.keyboard.press('Escape');
      return words;
    };
    // A finished turn: Idle.
    let s = await read();
    ok('after a finished turn the chat is Idle', /^Idle\|idle$/.test(s.title) && s.win === 'idle', JSON.stringify(s));
    // A pending call alone does not make the page say Working.
    await p.evaluate(() => __emit('agent:update', { sessionId: 'lead', update: { sessionUpdate: 'tool_call', toolCallId: 'early', title: 'Stray call', status: 'pending', kind: 'execute' } }));
    await settle();
    s = await read();
    ok('the page does not infer Working from a pending call; the engine decides', /^Idle\|/.test(s.title), JSON.stringify(s));
    await p.evaluate(() => __emit('agent:update', { sessionId: 'lead', update: { sessionUpdate: 'tool_call_update', toolCallId: 'early', status: 'completed' } }));
    // The person starts a turn; it runs a long tool call.
    await p.evaluate(() => { __engine('working'); __emit('agent:update', { sessionId: 'lead', update: { sessionUpdate: 'tool_call', toolCallId: 'long', title: 'Run the release checks', status: 'pending', kind: 'execute' } }); });
    await win.locator('.chat-working-chip .chat-state-working').waitFor({ state: 'attached' });
    await settle();
    s = await read();
    ok('the title shows Working with the PUDL working chip', s.title === 'Working|working', JSON.stringify(s));
    ok('the window gives the taskbar PUDL\'s working state', s.win === 'working', JSON.stringify(s));
    ok('the Chats list shows Working in the Working group', s.list === 'Working' && s.listGroup === 'Working', JSON.stringify(s));
    ok('the long call keeps its own status chip', s.tool === 'waiting', JSON.stringify(s));
    ok('the taskbar stack row shows Working', await stackRow() === 'Working|working');
    ok('the team group\'s Members dropdown shows Working', await members() === 'Working');
    // The call runs on; a later engine update keeps Working.
    await p.evaluate(() => __engine('working'));
    await settle();
    s = await read();
    ok('while the call runs, every surface stays Working', s.title === 'Working|working' && s.win === 'working' && s.list === 'Working', JSON.stringify(s));
    await p.evaluate(key => pudlWindows.raise ? pudlWindows.raise(key) : null, leadKey).catch(() => {});
    await t.shot('foreground-turn-working');
    // The turn ends.
    await p.evaluate(() => { __emit('agent:update', { sessionId: 'lead', update: { sessionUpdate: 'tool_call_update', toolCallId: 'long', status: 'completed' } }); __engine('idle'); });
    await win.locator('.chat-working-chip .chat-state-idle').waitFor({ state: 'attached' });
    await settle();
    s = await read();
    ok('when the turn ends the chat is Idle everywhere, and the call shows done', /^Idle\|idle$/.test(s.title) && s.win === 'idle' && s.list === 'Idle' && s.listGroup !== 'Working' && s.tool === 'done', JSON.stringify(s));
    ok('the stack row and Members show Idle', await stackRow() === 'Idle|idle' && await members() === 'Idle');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
