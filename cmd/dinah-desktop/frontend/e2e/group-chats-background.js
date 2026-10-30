// A chat that a message on the bus wakes while it is not connected is
// connected by the path Reconnect Now takes: a chat whose window was
// restored and could not reconnect by itself is reconnected in its window,
// and a closed chat is opened without a window. A chat open in the
// background shows the state Open in Background, with PUDL's eye-off glyph,
// in the Group Chats pane, the picker, a group's member chips and the Chats
// list, and one that closed itself for being idle shows Closed. The picker's
// note on a closed chat says that a mention opens it in the background. The
// page tells Dinah which chats have windows. The Group Chats pane's tab
// reads Group Chats even in a layout saved when the pane was called Bus.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspace: 'C:/Work/parser', workspaces: ['C:/Work/parser', 'C:/Work/docs'], recent: ['C:/Work/parser', 'C:/Work/docs'] },
    app: () => {
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      // The bus still has the parser chat as closed; the window knows better.
      const parser = { id: 'window-p1', chat: 'chat-parser', title: 'Parser Release', workspace: 'parser', address: 'Parser Release (parser)', state: 'closed' };
      const bus = {
        inbox: [], rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [parser, person] }],
        workspaces: [{ name: 'parser', label: 'Parser Library', path: 'C:/Work/parser', chats: [] }, { name: 'docs', label: 'Docs Site', path: 'C:/Work/docs', chats: [] }], links: []
      };
      const chats = { 'C:/Work/parser': [{ sessionId: 'chat-parser', provider: 'claude', title: 'Parser Release', cwd: 'C:/Work/parser', open: false, updatedAt: '2026-10-22T08:00:00Z' }],
        'C:/Work/docs': [{ sessionId: 'chat-restored', provider: 'codex', title: 'Restored Chat', cwd: 'C:/Work/docs', open: false, updatedAt: '2026-10-22T07:00:00Z' },
          { sessionId: 'chat-outside', provider: 'ollama', title: 'Outside Chat', cwd: 'C:/Work/docs', open: false, updatedAt: '2026-10-20T07:00:00Z' }] };
      let restoredTries = 0;
      return {
        Sessions: async () => chats['C:/Work/parser'], SessionsAt: async ws => chats[ws] || [],
        ResolveSessionContinuation: async id => id,
        SessionContinuation: async () => ({ history: [] }),
        SavedConversation: async id => id === 'chat-restored' ? { available: true, messages: [{ role: 'you', text: 'Check the release notes.' }, { role: 'agent', text: 'The notes are ready.' }] } : { available: false, messages: [] },
        OpenSessionAt: async (ws, id) => {
          __calls.push(['open', ws, id]);
          // The restored chat cannot reconnect by itself, so it waits in its
          // window; the next try works. The outside chat never opens.
          if (id === 'chat-restored' && ++restoredTries === 1) throw new Error('the agent is busy');
          if (id === 'chat-outside') throw new Error('Ollama is not running');
          const c = Object.values(chats).flat().find(x => x.sessionId === id);
          return Object.assign({}, c, { open: true, working: false, queued: [], inBackground: id === 'chat-parser' });
        },
        ChatWindows: async ids => { __calls.push(['windows', ids]); },
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace,bus', 'dinah-window-titles': { bus: { title: 'Bus', detail: '' } } } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: bus.inbox, unread: 0, rooms: bus.rooms, workspaces: bus.workspaces, links: bus.links, defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusGroupMessages: async () => [], BusMarkGroupRead: async () => {}
      };
    },
    init: () => { window.__calls = []; },
  });
  const { page: p, ok } = t;
  try {
    await p.locator('[data-win="workspace"]').waitFor({ state: 'attached' });
    const pane = p.locator('[data-win="bus"]');
    await pane.waitFor({ state: 'attached' });

    // The pane's title and its tab, from a layout that kept the old title.
    await p.waitForFunction(() => [...document.querySelectorAll('.win-dock-tablist [role="tab"]')].some(t => /Group Chats/.test(t.textContent)));
    const tabs = await p.locator('.win-dock-tablist [role="tab"]').allInnerTexts();
    ok('the docked pane\'s tab reads Group Chats, not Bus', tabs.some(t => /Group Chats/.test(t)) && !tabs.some(t => /^\s*Bus\s*$/.test(t)), JSON.stringify(tabs));
    ok('the pane\'s window is titled Group Chats', (await pane.locator('.win-title').textContent()).trim() === 'Group Chats');

    await p.waitForFunction(() => __calls.some(c => c[0] === 'windows'));
    ok('the page tells Dinah which chats have windows', await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'windows')[1])) === '[]');

    // A restored chat that could not reconnect by itself waits in its window
    // with the reason, as the prompt reports it.
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Work/docs', path: 'chat-restored' }));
    const restored = p.locator('.win[data-win-kind="chat-session"]').filter({ has: p.locator('.activity-row', { hasText: 'Check the release notes.' }) });
    await restored.waitFor();
    await restored.locator('[data-view-id="session-note"]').waitFor({ state: 'visible' });
    ok('a failed reconnect is reported above the prompt, and the saved copy stays', /could not open this session/.test(await restored.locator('[data-view-id="session-note"]').innerText()) &&
      /the agent is busy/.test(await restored.locator('[data-view-id="session-note"]').innerText()) && await restored.locator('.activity-row', { hasText: 'The notes are ready.' }).count() === 1);
    await p.waitForFunction(() => __calls.some(c => c[0] === 'windows' && c[1].includes('chat-restored')));
    ok('the restored chat\'s window is among the chats with windows', true);
    const windowsBefore = await p.locator('.win[data-win-kind="chat-session"]').count();
    await p.evaluate(() => __emit('chat:connect', { sessionId: 'chat-restored', cwd: 'C:/Work/docs', background: false }));
    await p.waitForFunction(() => __calls.filter(c => c[0] === 'open' && c[2] === 'chat-restored').length === 2);
    await restored.locator('[data-view-id="session-note"]').waitFor({ state: 'hidden' });
    ok('a message that wakes it reconnects it in its window, as Reconnect Now does', await restored.locator('[data-view-id="saved-note"]').isHidden());

    // A closed chat is opened in the background, without a window.
    await p.evaluate(() => __emit('chat:connect', { sessionId: 'chat-parser', cwd: 'C:/Work/parser', background: true }));
    await p.waitForFunction(() => __calls.some(c => c[0] === 'open' && c[1] === 'C:/Work/parser' && c[2] === 'chat-parser'));
    await p.waitForTimeout(300);
    ok('a closed chat opens without a window', await p.locator('.win[data-win-kind="chat-session"]').count() === windowsBefore);
    const row = pane.locator('[data-bus-chat="chat-parser"]');
    await row.locator('.badge.positive.background').waitFor();
    ok('the pane shows it as Open in Background', (await row.innerText()).includes('Open in Background'));
    const glyph = await row.locator('.badge.background').evaluate(el => getComputedStyle(el, '::before').getPropertyValue('--glyph').trim());
    ok('Open in Background has PUDL\'s eye-off glyph', /glyph-eye-off/.test(glyph) || /svg/.test(glyph), glyph);
    // The Workspace pane's list of chats is drawn as the Chats pane's is.
    const listed = p.locator('#workspace-chats [data-session-row="chat-parser"]');
    await listed.locator('.badge.background').waitFor();
    ok('the chat lists show it as Open in Background', (await listed.innerText()).includes('Open in Background'));

    // A group's chip and the picker show the same state.
    await pane.locator('[data-bus-room="release-review"]').click();
    const room = p.locator('.win[data-win-kind="bus-room"]');
    await room.locator('[data-group-member="window-p1"] .state-glyph.background').waitFor();
    ok('a group\'s member chip shows Open in Background', await room.locator('[data-group-member="window-p1"] .state-glyph.background[aria-label="Open in Background"]').count() === 1);
    await p.evaluate(() => window.dinahBus.startGroup(''));
    const picker = p.locator('.win[data-win-kind="group-picker"]');
    await picker.locator('[data-chat="chat-parser"]').waitFor();
    ok('the picker shows Open in Background', (await picker.locator('[data-chat="chat-parser"]').innerText()).includes('Open in Background'));
    await picker.locator('[data-chat="chat-outside"]').click();
    const note = await picker.locator('[data-chat="chat-outside"] .row-note').innerText();
    ok('the picker\'s note on a closed chat says a mention opens it in the background', /closed/.test(note) && /opens it in the background/.test(note) && !/until it opens\.$/.test(note.split(';')[0]), note);
    await picker.getByRole('button', { name: 'Cancel' }).click();

    // A chat with no window that cannot be opened says so in a notice.
    await p.evaluate(() => __emit('chat:connect', { sessionId: 'chat-outside', cwd: 'C:/Work/docs', background: true }));
    await p.locator('.toast', { hasText: 'Ollama is not running' }).first().waitFor();
    ok('a chat opened in the background that fails says why, and that its messages keep waiting', /keep waiting/.test(await p.locator('.toast', { hasText: 'Ollama is not running' }).first().innerText()));

    // One that closed itself for being idle is closed again.
    await p.evaluate(() => __emit('agent:session', { sessionId: 'chat-parser', cwd: 'C:/Work/parser', title: 'Parser Release', open: true, working: false, inBackground: false, parked: true }));
    await row.locator('.badge.background').waitFor({ state: 'detached' });
    ok('a chat that closed itself in the background shows Closed', (await pane.locator('[data-bus-chat="chat-parser"]').innerText()).includes('Closed'));
    await t.shot('group-chats-background');
  } finally { await t.close(); }
})();
