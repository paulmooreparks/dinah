const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ app: () => {
    const entry = (id, cwd, updatedAt, state = 'idle') => ({ sessionId: id, title: id, cwd, updatedAt, provider: 'codex', open: true,
      chatState: { life: 'open', window: true, exec: { state, why: '' } } });
    const lists = {
      'C:/Alpha': [entry('old', 'C:/Alpha', '2026-11-01T00:00:00Z'), entry('tie-a', 'C:/Alpha', '2026-11-02T00:00:00Z'), entry('unknown', 'C:/Alpha', ''), entry('working-old', 'C:/Alpha', '2026-11-01T00:00:00Z', 'working')],
      'C:/Beta': [entry('new', 'C:/Beta', '2026-11-03T08:00:00+08:00'), entry('tie-b', 'C:/Beta', '2026-11-02T00:00:00Z'), entry('invalid', 'C:/Beta', 'invalid'), entry('working-new', 'C:/Beta', '2026-11-04T00:00:00Z', 'working')]
    };
    window.__recentUpdate = (id, updatedAt) => {
      const item = Object.values(lists).flat().find(item => item.sessionId === id);
      item.updatedAt = updatedAt; __emit('agent:session', item);
    };
    return { State: async () => ({ workspace: 'C:/Alpha', recent: ['C:/Alpha','C:/Beta'], workspaces: ['C:/Alpha','C:/Beta'] }),
      Sessions: async () => lists['C:/Alpha'], SessionsAt: async ws => lists[ws] || [] };
  } });
  const p = t.page;
  const order = () => p.locator('#sessions [data-session-row]').evaluateAll(rows => rows.map(row => row.dataset.sessionRow));
  try {
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.selectOption('#chat-scope', 'all');
    await p.waitForFunction(() => document.querySelectorAll('#sessions [data-session-row]').length === 8);
    const initial = ['working-new','working-old','new','tie-a','tie-b','old','unknown','invalid'];
    t.ok('activity sorts across workspaces within priority groups; ties stay stable and unavailable dates go last', JSON.stringify(await order()) === JSON.stringify(initial), JSON.stringify(await order()));
    await p.mouse.move(1, 1);
    await p.locator('#sessions [data-session-row="old"] a').focus();
    await p.evaluate(() => __recentUpdate('old', '2026-11-05T00:00:00Z'));
    await p.waitForTimeout(150);
    t.ok('focused rows hold their positions during an activity update', JSON.stringify(await order()) === JSON.stringify(initial));
    await p.evaluate(() => document.activeElement.blur());
    await p.waitForFunction(() => document.querySelector('#sessions [data-section="chats-Chats"] [data-session-row]').dataset.sessionRow === 'old');
    t.ok('new activity moves to the top after focus leaves', (await order())[2] === 'old');
    await p.selectOption('#chat-scope', 'current');
    await p.waitForFunction(() => document.querySelectorAll('#sessions [data-session-row]').length === 4);
    t.ok('workspace filtering retains activity order', JSON.stringify(await order()) === JSON.stringify(['working-old','old','tie-a','unknown']));
    await p.fill('#chat-filter', 'tie');
    await p.waitForFunction(() => document.querySelectorAll('#sessions [data-session-row]').length === 1);
    t.ok('text filtering remains effective', JSON.stringify(await order()) === '["tie-a"]');
  } finally { await t.close(); }
})();
