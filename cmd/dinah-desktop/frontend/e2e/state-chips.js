// One state chip everywhere a chat's state is shown (chat-state.js, PUDL's
// execution-state badge). For Working, Waiting, Stuck, Idle and Background,
// the chip in the Chats list, on the chat's Notifications card, in the
// Group Chats pane, in the group's Members dropdown and on the group's own
// title shows the same short word, glyph and size as the chat's title chip,
// with the same PUDL colour wherever it is not on a title bar (a title bar
// draws its chips in the bar's own colour, WINDOW-STATUS.md). A reason, such
// as "Needs your answer or approval", is in the chip's tooltip and name,
// never inside it. Nothing moves when the state changes.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const cwd = 'C:/Fixture';
    window.__exec = { state: 'working', why: 'Turn in progress' };
    const entry = () => ({ sessionId: 'x', bus: 'bus-x', life: 'open', window: true, exec: __exec });
    const room = () => ({ name: 'team', title: 'Team', counts: { [__exec.state]: 1, top: __exec.state }, members: [{ id: 'bus-x', bus: 'bus-x', chat: 'x', title: 'Release Chat', workspace: 'Fixture', provider: 'claude', life: 'open', window: true, exec: __exec }] });
    let ver = 1;
    window.__snapshot = () => ({ chats: [entry()], groups: [{ name: 'team', counts: { [__exec.state]: 1, top: __exec.state } }], stateGen: 'chips', stateVer: ++ver });
    const info = () => ({ sessionId: 'x', title: 'Release Chat', cwd, provider: 'claude', open: true, chatState: entry() });
    return { State: async () => ({ workspace: cwd, recent: [cwd], workspaceId: 'w1', workspaceRecords: [{ id: 'w1', name: 'Fixture', executionPath: cwd, sources: [] }] }),
      Sessions: async () => [info()], SessionsAt: async () => [info()], OpenSessionAt: async () => info(), SavedConversation: async () => ({ available: false }),
      ChatStates: async () => __snapshot(), ChatLead: async () => null, Team: async () => null,
      Notifications: async () => [{ id: 'n1', kind: 'turn-failure', severity: 'error', title: 'An earlier turn failed', body: 'The chat has since started another turn', sessionId: 'x', sessionTitle: 'Release Chat', workspace: cwd, provider: 'claude', created: new Date().toISOString() }],
      BusOverview: async () => ({ inbox: [], unread: 0, rooms: [room()], workspaces: [], links: [] }), BusResolveGroup: async () => ({ state: 'resolved', room: room() }),
      BusGroupMessages: async () => [], BusMarkGroupRead: async () => {} };
  } });
  const { page: p, ok } = t;
  const STATES = [
    { exec: { state: 'working', why: 'Turn in progress' }, word: 'Working', pudl: 'working' },
    { exec: { state: 'waiting', why: 'Needs your answer or approval' }, word: 'Waiting', pudl: 'waiting', reason: 'Needs your answer or approval' },
    { exec: { state: 'stuck', why: 'Last turn failed: overloaded' }, word: 'Stuck', pudl: 'stuck', reason: 'Last turn failed: overloaded' },
    { exec: { state: 'idle', why: 'No work in progress' }, word: 'Idle', pudl: 'idle' },
    { exec: { state: 'background', why: '1 background task: Run tests', tasks: 1 }, word: 'Background', pudl: 'working', reason: '1 background task: Run tests' },
  ];
  // The chip on each surface: its words, PUDL state, glyph, size and paint.
  const read = (chatKey, groupKey) => p.evaluate(([chatKey, groupKey]) => {
    const glyphs = {}; ['play', 'pause', 'warning', 'square'].forEach(n => { glyphs[getComputedStyle(document.documentElement).getPropertyValue('--glyph-' + n).trim()] = n; });
    const look = b => {
      if (!b) return null;
      const cs = getComputedStyle(b), before = getComputedStyle(b, '::before');
      return { text: b.textContent.trim(), state: b.dataset.state || '', glyph: glyphs[before.getPropertyValue('--glyph').trim()] || 'none', font: cs.fontSize + '/' + cs.fontWeight, height: Math.round(b.getBoundingClientRect().height), color: cs.color, bg: cs.backgroundColor, name: b.getAttribute('aria-label') || '', tip: b.title || '' };
    };
    const chip = sel => document.querySelector(sel);
    const row = [...document.querySelectorAll('#sessions .md-item')].find(a => /Release Chat/.test(a.textContent));
    return {
      title: look(chip('[data-win="' + chatKey + '"] .chat-working-chip .badge')),
      list: look(row && row.querySelector('.chat-state-slot .badge:not(.chat-state-reserve)')),
      notice: look(chip('#questions .notice-chat-state')),
      pane: look(chip('[data-section="bus-chats"] [data-bus-chat="bus-x"] .badge, [data-section="bus-chats"] .badge.chat-state')),
      members: look(chip('.members-menu [data-group-member="bus-x"] .badge.chat-state:not(.chat-state-reserve)')),
      group: look(chip('[data-win="' + groupKey + '"] .group-state-chip .badge')),
      geometry: { listTitle: row ? Math.round(row.querySelector('.browser-row-title').getBoundingClientRect().width) : 0, listSlot: row ? Math.round(row.querySelector('.chat-state-slot').getBoundingClientRect().width) : 0,
        chatTitle: Math.round(document.querySelector('[data-win="' + chatKey + '"] .win-title').getBoundingClientRect().width), chatChip: Math.round(document.querySelector('[data-win="' + chatKey + '"] .chat-working-chip').getBoundingClientRect().width),
        member: (() => { const r = document.querySelector('.members-menu [data-group-member="bus-x"]'); return r ? [Math.round(r.getBoundingClientRect().height), Math.round(r.querySelector('.member-name').getBoundingClientRect().width)] : null; })() }
    };
  }, [chatKey, groupKey]);
  try {
    const chatKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', path: 'x', workspace: 'w1' }));
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await p.evaluate(() => dinahWorkbench.open('bus'));
    const groupKey = await p.evaluate(() => dinahBus.openGroup('team', { message: 0 }).then(() => pudlWindows.state().top));
    await p.locator('.notice-chat-state').first().waitFor({ state: 'attached' });
    const group = p.locator('[data-win="' + groupKey + '"]');
    const geometries = [];
    // Each surface is read with its window in front, as a person sees it.
    const front = async key => { await p.evaluate(k => pudlWindows.raise(k), key); await p.waitForTimeout(400); return read(chatKey, groupKey); };
    for (const st of STATES) {
      await p.evaluate(s => { window.__exec = s; __emit('agent:chatstate', __snapshot()); }, st.exec);
      await p.waitForFunction(w => { const b = document.querySelector('.chat-working-chip .badge'); return b && b.textContent.trim() === w; }, st.word);
      await p.waitForTimeout(400);
      const fromChat = await front(chatKey), fromList = await front('chat-browser'), fromNotes = await front('needs-you'), fromPane = await front('bus');
      await p.evaluate(k => pudlWindows.raise(k), groupKey); await p.waitForTimeout(300);
      await group.getByRole('button', { name: /^Members/ }).click();
      await p.locator('.members-menu:popover-open').waitFor();
      const fromGroup = await read(chatKey, groupKey);
      await p.keyboard.press('Escape');
      const r = { title: fromChat.title, list: fromList.list, notice: fromNotes.notice, pane: fromPane.pane, members: fromGroup.members, group: fromGroup.group };
      geometries.push({ list: [fromList.geometry.listTitle, fromList.geometry.listSlot], chat: [fromChat.geometry.chatTitle, fromChat.geometry.chatChip], member: fromGroup.geometry.member });
      const surfaces = ['list', 'notice', 'pane', 'members'];
      const missing = surfaces.concat(['group']).filter(k => !r[k]);
      ok(st.word + ': every surface has its chip', !missing.length && r.title, JSON.stringify({ missing }));
      if (missing.length || !r.title) continue;
      const same = (k, fields) => fields.every(f => r[k][f] === r.title[f]);
      ok(st.word + ': each chip shows the title chip\'s word, PUDL state, glyph, type and height', surfaces.every(k => same(k, ['text', 'state', 'glyph', 'font', 'height'])) && r.title.text === st.word && r.title.state === st.pudl,
        JSON.stringify(Object.fromEntries(['title'].concat(surfaces).map(k => [k, [r[k].text, r[k].state, r[k].glyph, r[k].font, r[k].height]]))));
      ok(st.word + ': the group\'s title chip shows its state as the chat\'s does', same('group', ['text', 'state', 'glyph', 'font', 'height']), JSON.stringify([r.group.text, r.group.state, r.group.glyph]));
      ok(st.word + ': off the title bar, each chip has the same PUDL colour', surfaces.every(k => r[k].color === r.list.color && r[k].bg === r.list.bg), JSON.stringify(Object.fromEntries(surfaces.map(k => [k, [r[k].color, r[k].bg]]))));
      if (st.reason) ok(st.word + ': the reason is in the chip\'s name, not inside it', ['list', 'notice', 'pane', 'members'].every(k => !r[k].text.includes(st.reason) && (r[k].name + r[k].tip).includes(st.reason)), JSON.stringify(Object.fromEntries(['list', 'notice', 'pane', 'members'].map(k => [k, [r[k].text, r[k].name]]))));
      if (st.word === 'Waiting') {
        await p.evaluate(() => pudlWindows.raise('needs-you')); await p.waitForTimeout(300);
        const gap = await p.evaluate(() => { const card = document.querySelector('#questions .notices-card'), a = card.querySelector('.notice-actions'), c = card.querySelector('.notice-chat-state'); return Math.round(c.getBoundingClientRect().top - a.getBoundingClientRect().bottom); });
        ok('on a Notifications card the chip stands a gap below the actions', gap >= 8, String(gap));
      }
      if (st.word === 'Waiting') { await p.evaluate(() => pudlWindows.raise('needs-you')); await p.waitForTimeout(300); await t.shot('state-chips-waiting-notice', { clip: await p.locator('#questions .notices-card').first().boundingBox() }); }
    }
    ok('nothing moves as the state changes', geometries.every(g => JSON.stringify(g) === JSON.stringify(geometries[0])), JSON.stringify(geometries));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
