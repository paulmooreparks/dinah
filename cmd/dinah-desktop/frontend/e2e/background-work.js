// Background work (docs/design/background-activity.md). A chat whose turn
// has ended while its agent's background work runs on shows Background work,
// from the engine's execution state, everywhere a chat's state is shown: its
// title chip, the window's PUDL state used by the taskbar, the Chats list,
// a group's member menu and its aggregate. Foreground Working, Waiting and
// Stuck keep their precedence and say that background work continues. The
// work finishing, failing or being stopped returns the chat to Idle; an
// older snapshot never brings it back, a resync takes the engine's current
// state, and a chat running elsewhere stays unknown. Rows never move and an
// open member menu keeps its focus as states change.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const workspace = 'C:/Fixture';
    const chat = (id, title) => ({ sessionId: id, title, provider: 'claude', cwd: workspace, open: true, working: false, background: [] });
    const chats = [chat('lead', 'Team Lead'), chat('quiet', 'Quiet Chat')];
    const exec = (state, why, more) => Object.assign({ state, why }, more || {});
    const entry = (id, e) => ({ sessionId: id, bus: 'bus-' + id, life: 'open', window: true, exec: e });
    window.__snap = {
      chats: [entry('lead', exec('working', 'Turn in progress', { tasks: 1 })), entry('quiet', exec('idle', 'No work in progress')),
        { sessionId: '', bus: 'bus-far', life: 'open', window: false, elsewhere: 'Running in another Dinah window' }],
      groups: [{ name: 'team', counts: { working: 1, idle: 1, unknown: 1, top: 'working' } }], stateGen: 'g', stateVer: 1,
    };
    const members = [{ id: 'bus-lead', bus: 'bus-lead', chat: 'lead', title: 'Team Lead', workspace: 'Fixture', provider: 'claude' },
      { id: 'bus-quiet', bus: 'bus-quiet', chat: 'quiet', title: 'Quiet Chat', workspace: 'Fixture', provider: 'claude' },
      { id: 'bus-far', bus: 'bus-far', chat: '', title: 'Far Chat', workspace: 'Elsewhere', provider: 'claude' }];
    // Sends the engine's next snapshot: the lead's exec and the group's counts.
    window.__set = (leadExec, counts) => {
      __snap.chats[0].exec = leadExec;
      __snap.groups[0].counts = counts;
      __snap.stateVer++;
      __emit('agent:chatstate', structuredClone(__snap));
    };
    window.__exec = exec;
    return {
      State: async () => ({ workspace, workspaces: [workspace], recent: [workspace], defaultProvider: 'claude' }),
      Sessions: async () => structuredClone(chats), SessionsAt: async () => structuredClone(chats),
      OpenSessionAt: async (cwd, id) => structuredClone(chats.find(c => c.sessionId === id)),
      ChatStates: async () => structuredClone(__snap),
      ChatLead: async () => null, Team: async () => null,
      BusOverview: async () => ({ inbox: [], unread: 0, rooms: [{ name: 'team', title: 'Team', members, counts: __snap.groups[0].counts }], workspaces: [], links: [] }),
      BusGroupMessages: async () => [], BusMarkGroupRead: async () => {},
    };
  } });
  const { page: p, ok } = t;
  const settle = () => p.evaluate(() => new Promise(r => setTimeout(() => requestAnimationFrame(() => requestAnimationFrame(r)), 50)));
  try {
    const chatKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', path: 'lead' }));
    const win = p.locator('[data-win="' + chatKey + '"]');
    await win.locator('.chat-working-chip .chat-state-working').waitFor();
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    const row = p.locator('#sessions .md-item').filter({ hasText: 'Team Lead' });
    await row.locator('.session-badge').first().waitFor();
    const groupKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'bus-room', path: 'team' }));
    const group = p.locator('[data-win="' + groupKey + '"]');
    await group.locator('.group-state-chip').waitFor({ state: 'attached' });
    const read = () => p.evaluate(([chatKey, groupKey]) => {
      const w = document.querySelector('[data-win="' + chatKey + '"]'), chip = w.querySelector('.chat-working-chip .badge');
      const r = [...document.querySelectorAll('#sessions .md-item')].find(a => /Team Lead/.test(a.textContent));
      const slot = r.querySelector('.chat-state-slot'), rowChip = slot.querySelector('.badge:not(.chat-state-reserve)');
      const g = document.querySelector('[data-win="' + groupKey + '"]');
      const member = document.querySelector('[data-group-member="bus-lead"] .badge:not(.chat-state-reserve)');
      const far = document.querySelector('[data-group-member="bus-far"] .badge:not(.chat-state-reserve)');
      return {
        chip: chip.textContent.trim(), chipCls: chip.className, chipLabel: (chip.getAttribute('aria-label') || '') + ' | ' + (chip.title || ''), chipBox: [Math.round(w.querySelector('.chat-working-chip').getBoundingClientRect().width), Math.round(chip.getBoundingClientRect().height)],
        winState: w.dataset.winState, winLabel: w.dataset.winStateLabel,
        row: rowChip.textContent.trim(), rowCls: rowChip.className, rowLabel: r.getAttribute('aria-label'), slotWidth: Math.round(slot.getBoundingClientRect().width), titleWidth: Math.round(r.querySelector('.browser-row-title').getBoundingClientRect().width),
        listGroup: r.closest('.browser-group') && r.closest('.browser-group').querySelector('.browser-group-title').textContent.trim(),
        aggregate: (g.querySelector('.group-state-chip .badge') || g.querySelector('.group-state-chip')).getAttribute('aria-label') || g.querySelector('.group-state-chip').textContent.trim(),
        member: member ? member.className + ' :: ' + (member.getAttribute('aria-label') || member.textContent) : '',
        far: far ? far.className + ' :: ' + far.textContent.trim() : '',
      };
    }, [chatKey, groupKey]);
    const glyphOf = cls => /chat-state-(working|background)/.test(cls);
    // 1. A foreground turn with background work beside it.
    let s = await read();
    const base = s;
    ok('a running turn shows Working, and its name says background work continues', s.chip === 'Working' && /Background work continues: 1 background task/.test(s.chipLabel + s.rowLabel) && s.winState === 'working', JSON.stringify(s));
    // 2. The turn ends; the task runs on.
    await p.evaluate(() => __set(__exec('background', '1 background task: Run tests', { tasks: 1 }), { background: 1, idle: 1, unknown: 1, top: 'background' }));
    await win.locator('.chat-working-chip .chat-state-background').waitFor();
    await settle();
    s = await read();
    ok('after the turn ends, the title chip shows Background, named Background work with the task', s.chip === 'Background' && /Background work/.test(s.chipLabel) && /Run tests/.test(s.chipLabel) && /chat-state-background/.test(s.chipCls), JSON.stringify(s));
    ok('the title chip keeps its width and one line', s.chipBox[0] === base.chipBox[0] && s.chipBox[1] === base.chipBox[1], JSON.stringify({ base: base.chipBox, now: s.chipBox }));
    ok('the window gives PUDL its working state with the Background work label, for the taskbar', s.winState === 'working' && /^Background work/.test(s.winLabel), JSON.stringify(s));
    ok('the Chats list shows Background in the Working group', s.row === 'Background' && glyphOf(s.rowCls) && s.listGroup === 'Working', JSON.stringify(s));
    ok('the group aggregate counts it and ranks it above idle', /1 with background work, 1 idle/.test(s.aggregate) && !/Idle/.test(s.aggregate.split(':')[0]), s.aggregate);
    ok('the state chip slot and the title keep their widths', s.slotWidth === base.slotWidth && s.titleWidth === base.titleWidth, JSON.stringify({ base: [base.slotWidth, base.titleWidth], now: [s.slotWidth, s.titleWidth] }));
    ok('a chat running elsewhere stays unknown, not idle or background', /Running in another Dinah window|Elsewhere/.test(s.far) && !/chat-state-(idle|background)/.test(s.far), s.far);
    await t.shot('background-work-title', { clip: await win.locator('.win-head').boundingBox() });
    await t.shot('background-work-list', { clip: await p.locator('#chat-browser-view').boundingBox() });
    // 3. Concurrent work, with the member menu open and focused.
    await group.getByRole('button', { name: /^Members/ }).click();
    const menuRow = p.locator('[popover]:popover-open [data-group-member="bus-quiet"] button').first();
    await menuRow.waitFor();
    await menuRow.focus();
    await p.waitForFunction(() => document.activeElement && document.activeElement.closest('[data-group-member="bus-quiet"]'));
    const focusedBefore = await p.evaluate(() => document.activeElement.closest('[data-group-member]') && document.activeElement.closest('[data-group-member]').dataset.groupMember);
    await p.evaluate(() => __set(__exec('background', '2 background tasks, 1 subagent', { tasks: 2, subagents: 1 }), { background: 1, idle: 1, unknown: 1, top: 'background' }));
    await p.waitForFunction(key => /2 background tasks/.test(document.querySelector('[data-win="' + key + '"] .chat-working-chip').textContent), chatKey);
    await settle();
    s = await read();
    ok('concurrent tasks and a subagent are counted in the name of the title chip', s.chip === 'Background' && /2 background tasks, 1 subagent/.test(s.chipLabel), JSON.stringify(s));
    const focusedAfter = await p.evaluate(() => document.activeElement.closest('[data-group-member]') && document.activeElement.closest('[data-group-member]').dataset.groupMember);
    const menuOpen = await p.evaluate(key => !!document.querySelector('[data-win="' + key + '"] [popover]:popover-open [data-group-member]'), groupKey);
    ok('the member menu stays open with focus on the same member as states change', menuOpen && focusedBefore === 'bus-quiet' && focusedAfter === 'bus-quiet', JSON.stringify({ menuOpen, focusedBefore, focusedAfter }));
    ok('the lead member in the menu shows Background', /chat-state-background/.test(s.member), s.member);
    await t.shot('background-work-members', { clip: await group.boundingBox() });
    await p.keyboard.press('Escape');
    // 4. Approval and failure keep precedence and say the work continues.
    await p.evaluate(() => __set(__exec('waiting', 'Needs your answer or approval', { tasks: 1 }), { waiting: 1, idle: 1, unknown: 1, top: 'waiting' }));
    await win.locator('.chat-working-chip .chat-state-waiting').waitFor();
    await row.locator('.chat-state-waiting').first().waitFor();
    s = await read();
    ok('a question for the person outranks background work, which its name still mentions', /^Waiting/.test(s.chip) && /Background work continues: 1 background task/.test(s.chipLabel) && /Background work continues: 1 background task/.test(s.rowLabel) && s.winState === 'waiting', JSON.stringify(s));
    await p.evaluate(() => __set(__exec('stuck', 'Last turn failed: overloaded', { tasks: 1 }), { stuck: 1, idle: 1, unknown: 1, top: 'stuck' }));
    await win.locator('.chat-working-chip .chat-state-stuck').waitFor();
    await row.locator('.chat-state-stuck').first().waitFor();
    s = await read();
    ok('a failed turn outranks background work too', /^Stuck/.test(s.chip) && /Background work continues/.test(s.chipLabel + s.rowLabel), JSON.stringify(s));
    // 5. The work completes, fails or is stopped: Idle, everywhere.
    for (const how of ['completes', 'fails', 'is stopped']) {
      await p.evaluate(() => __set(__exec('background', '1 background task', { tasks: 1 }), { background: 1, idle: 1, unknown: 1, top: 'background' }));
      await win.locator('.chat-working-chip .chat-state-background').waitFor();
      await p.evaluate(() => __set(__exec('idle', 'No work in progress'), { idle: 2, unknown: 1, top: 'idle' }));
      await win.locator('.chat-working-chip .chat-state-idle').waitFor();
      await settle();
      s = await read();
      ok('when the task ' + how + ', the chat is Idle in its title, the taskbar state, the list and the group', s.chip === 'Idle' && s.winState === 'idle' && s.row === 'Idle' && s.listGroup !== 'Working' && /2 idle/.test(s.aggregate) && !/Background work continues/.test(s.chipLabel + s.rowLabel), JSON.stringify(s));
    }
    // 6. A stale snapshot is ignored; a resync takes the engine's state.
    await p.evaluate(() => { const old = structuredClone(__snap); old.stateVer -= 3; old.chats[0].exec = __exec('background', '1 background task', { tasks: 1 }); __emit('agent:chatstate', old); });
    await settle();
    s = await read();
    ok('an older snapshot does not bring background work back', s.chip === 'Idle', s.chip);
    await p.evaluate(() => { __snap.chats[0].exec = __exec('background', '1 background task: Watch build', { tasks: 1 }); __snap.groups[0].counts = { background: 1, idle: 1, unknown: 1, top: 'background' }; __snap.stateVer += 5; });
    await p.evaluate(() => __resync());
    const resynced = await win.locator('.chat-working-chip .chat-state-background').waitFor({ timeout: 4000 }).then(() => true, () => false);
    ok('after a reconnect the page takes the engine\'s current state', resynced);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
