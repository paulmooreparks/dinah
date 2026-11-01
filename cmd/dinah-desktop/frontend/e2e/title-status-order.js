// A lead chat's title bar shows its own state alone, never its team's;
// the team's state as a whole stays on its group window. Loading stands
// just before the status chip while a chat's history arrives, through
// state updates in either order, at normal width and in the narrow
// window's Window status list.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1400, height: 1000 }, app: () => {
    let release, ver = 1;
    window.__gate = new Promise(r => { release = r; });
    window.__release = release;
    const ref = (id, title) => ({ id: 'bus-' + id, chat: id, title, life: 'open', window: true, exec: { state: 'idle', why: '' } });
    const rooms = [{ name: 'team-a', title: 'Team A', lead: 'bus-claude:lead', members: [ref('claude:lead', 'Lead'), ref('alpha', 'Alpha')], counts: { top: 'stuck', stuck: 1, idle: 1 } }];
    const session = (id, state) => ({ sessionId: id, provider: 'claude', title: 'Lead', cwd: 'C:/Test', open: true, stateGen: 'g', stateVer: ++ver, chatState: { life: 'open', window: true, exec: { state, why: state } } });
    window.__state = state => __emit('agent:session', session('claude:lead', state));
    return {
      State: async () => ({ workspace: 'C:/Test', recent: ['C:/Test'], defaultProvider: 'claude', catalog: [], defaults: {}, pinned: [] }),
      AgentProviders: async () => [{ id: 'claude', installed: true, available: true }],
      ResolveSessionContinuation: async id => id, SessionContinuation: async () => ({ history: [] }),
      SavedConversation: async () => ({ available: true, complete: true, messages: [{ role: 'you', text: 'Plan the work.' }, { role: 'agent', text: 'SAVED-ANSWER' }] }),
      OpenSessionAt: async (ws, id) => { __emit('agent:session', session(id, 'working')); await __gate; return session(id, 'idle'); },
      ChatLead: async () => null,
      Team: async id => ({ lead: { chat: id, bus: 'bus-' + id, title: 'Lead' }, group: 'team-a', stateGen: 'tg', stateVer: ++ver, counts: { top: 'stuck', stuck: 1, idle: 1 },
        members: [{ chat: 'alpha', bus: 'bus-alpha', title: 'Alpha', life: 'open', window: true, exec: { state: 'stuck', why: 'Blocked' } }], budget: { rows: [] } }),
      BusOverview: async () => ({ inbox: [], rooms, workspaces: [], links: [] }), BusResolveGroup: async name => ({ state: 'resolved', room: rooms.find(r => r.name === name) }),
      BusGroupMessages: async () => [], BusMarkGroupRead: async () => {},
    };
  } });
  const { page: p, ok } = t;
  try {
    const key = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'claude:lead' }));
    const win = p.locator('[data-win="' + key + '"]');
    await win.getByText('SAVED-ANSWER').first().waitFor();
    await p.waitForFunction(k => { const el = document.querySelector('[data-win="' + k + '"] .chat-loading-chip'); return el && !el.hidden; }, key);
    await win.locator('.chat-team .team-toggle').waitFor();
    const order = () => win.evaluate(w => {
      const group = w.querySelector('.win-head > .win-statuses'), shown = [...group.children].filter(c => c.classList.contains('win-status') && !c.hidden);
      const loading = group.querySelector('.chat-loading-chip'), status = group.querySelector('.chat-working-chip');
      return { names: shown.map(c => c.className.replace(/win-status|strong/g, '').trim()), next: loading && loading.nextElementSibling === status,
        left: loading && status && !loading.hidden ? loading.getBoundingClientRect().right <= status.getBoundingClientRect().left + 1 : null,
        team: group.querySelectorAll('.team-stuck-chip').length, states: group.querySelectorAll('.chat-working-chip .badge.chat-state:not(.chat-state-reserve)').length,
        text: status ? status.textContent.trim() : '' };
    });
    let s = await order();
    ok('Loading stands just left of the status chip while history arrives', s.next && s.left === true, JSON.stringify(s));
    ok('the lead\'s title bar has its own state and no team chip', s.team === 0 && s.states === 1 && !/Stuck/.test(s.text), JSON.stringify(s));
    for (const state of ['waiting', 'working', 'idle', 'working']) { await p.evaluate(st => __state(st), state); await p.waitForTimeout(80); }
    s = await order();
    ok('state updates arriving during the load keep Loading first', s.next && s.left === true && s.team === 0, JSON.stringify(s));
    // The narrow window folds the chips into Window status, in the same order.
    await win.evaluate(w => w.style.setProperty('--win-w', '0.2'));
    await p.waitForTimeout(300);
    const toggle = win.getByRole('button', { name: 'Window status', exact: true });
    if (await toggle.isVisible()) {
      await toggle.click();
      const rows = await p.locator('.win-status-popover:popover-open li').allTextContents();
      const li = rows.findIndex(r => /Loading/.test(r)), si = rows.findIndex(r => /Working|Idle|Waiting/.test(r));
      ok('narrow: Window status lists Loading before the status, and no team state', li >= 0 && si > li && !rows.some(r => /Stuck/.test(r)), JSON.stringify(rows));
      await t.shot('title-status-order-narrow');
      await p.keyboard.press('Escape');
    } else ok('narrow: the window folded its status chips', false);
    await win.evaluate(w => w.style.removeProperty('--win-w'));
    await p.evaluate(() => __release());
    await p.waitForFunction(k => document.querySelector('[data-win="' + k + '"] .chat-loading-chip').hidden, key);
    await p.evaluate(() => __state('working'));
    await p.waitForTimeout(150);
    s = await order();
    ok('after the load, the status chip stands alone', !s.names.includes('chat-loading-chip') && s.team === 0, JSON.stringify(s));
    await t.shot('title-status-order');
    // The team group window keeps the team's state.
    await p.evaluate(() => dinahBus.openGroup('team-a', { message: 0 }));
    const group = p.locator('.win[data-win-kind="bus-room"]');
    await group.locator('.group-state-chip .badge.chat-state').waitFor();
    const groupChip = await group.locator('.group-state-chip').textContent();
    ok('the team\'s group window keeps the team\'s state', /stuck/i.test(groupChip), groupChip);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
