// The Group Chats pane lists Paul's inbox, the group chats, every chat by
// workspace with its state and the trusted links; sends a new message from
// its form; opens a group, a thread and the picker in windows of their own.
// A group's window shows its members as chips with their states and menus,
// its conversation under the senders' titles with the lines saying who
// joined, and a box whose @ list offers the members and puts a mention in
// whole. The picker starts a group of open and closed chats, asks an
// untitled chat for its title in place, and keeps its choices in its
// address. A chat window's Actions menu starts a group and adds the chat to
// one. A chat is shown by its title and a workspace by its name in the
// window, never by the bus's old handles.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      // Messages come as the window gives them: a chat by its title, a
      // workspace by its name in the window, the person by the person's name.
      const spaces = { 'Parser Release': 'Parser Library', 'Guide Fixes': 'Docs Site', 'Docs Pass': 'docs' };
      const msg = (id, from, to, text, extra) => Object.assign({ id, createdAt: '2026-10-22T08:0' + (id % 10) + ':00Z', kind: 'text', from, fromPerson: from === 'Paul', fromSession: from === 'Paul' ? '' : 'window-' + id, fromWorkspace: spaces[from] || '', to, room: '', mentions: [], replyTo: 0, text, attachments: [], unread: false }, extra);
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const parser = { id: 'window-p1', chat: 'chat-parser', title: 'Parser Release', workspace: 'parser', address: 'Parser Release (parser)', state: 'closed' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' };
      const bus = {
        inbox: [msg(3, 'Parser Release', 'Paul', 'Release 2.4 is tagged.\nNotes are in CHANGELOG.md.', { unread: true, replyTo: 1 }), msg(2, 'Guide Fixes', 'Paul', 'The guide builds again.'),
          msg(6, 'Docs Pass', 'Release Review', 'Paul, the guide needs your call.', { room: 'release-review', roomTitle: 'Release Review', mentions: [{ id: 'paul', title: 'Paul' }] })],
        rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '2026-10-22T09:00:00Z', unread: 2, latest: { id: 9, from: 'Docs Pass', line: 'The guide is ready for a second look' }, members: [parser, docs, person] },
          { name: 'docs-review', title: 'Docs Review', lastAt: '', unread: 0, members: [docs, person] }],
        roomMessages: { 'release-review': [
          msg(4, 'Parser Release', 'Release Review', '@Docs Pass please check the guide against 2.4.', { room: 'release-review', roomTitle: 'Release Review', mentions: [{ id: 'window-d1', title: 'Docs Pass' }] }),
          { id: 5, createdAt: '2026-10-22T08:05:00Z', kind: 'member', from: 'Paul', fromPerson: true, room: 'release-review', line: 'Paul added Docs Pass to the group.', text: 'added', mentions: [], attachments: [] }] },
        threads: { 1: [msg(1, 'Paul', 'Parser Library', 'Is 2.4 ready?'), msg(3, 'Parser Release', 'Paul', 'Release 2.4 is tagged.', { replyTo: 1 })] },
        workspaces: [{ name: 'parser', label: 'Parser Library', path: 'C:/Work/parser', chats: [] },
          { name: 'docs', label: 'Docs Site', path: 'C:/Work/docs', chats: [{ id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', address: 'Docs Pass (docs)' }] }],
        links: []
      };
      window.__bus = bus;
      // The window's chats: two in parser, one of them untitled, and two in
      // docs, one open.
      const chats = { 'C:/Work/parser': [{ sessionId: 'chat-parser', provider: 'claude', title: 'Parser Release', cwd: 'C:/Work/parser', open: false, updatedAt: '2026-10-22T08:00:00Z' },
          { sessionId: 'chat-blank', provider: 'codex', title: '', cwd: 'C:/Work/parser', open: false, updatedAt: '2026-10-21T08:00:00Z' }],
        'C:/Work/docs': [{ sessionId: 'chat-docs', provider: 'claude', title: 'Docs Pass', cwd: 'C:/Work/docs', open: true, updatedAt: '2026-10-22T07:00:00Z' },
          { sessionId: 'chat-outside', provider: 'ollama', title: 'Outside Chat', cwd: 'C:/Work/docs', open: false, updatedAt: '2026-10-20T07:00:00Z' }] };
      const titles = {};
      let next = 10;
      return {
        State: async () => ({ defaultProvider: 'devin', workspace: 'C:/Work/parser', workspaces: ['C:/Work/parser', 'C:/Work/docs'], recent: ['C:/Work/parser', 'C:/Work/docs'] }),
        AgentProviders: async () => [{ id: 'devin', name: 'Devin', installed: true, available: true, status: 'detected' }, { id: 'codex', name: 'Codex', installed: true, available: true, status: 'detected' }, { id: 'claude', name: 'Claude', installed: true, available: true, status: 'detected' }, { id: 'ollama', name: 'Ollama', installed: true, available: true, status: 'detected' }],
        Sessions: async () => chats['C:/Work/parser'], SessionsAt: async ws => chats[ws] || [],
        OpenSessionAt: async (ws, id) => { __calls.push(['open', ws, id]); const c = Object.values(chats).flat().find(x => x.sessionId === id); return Object.assign({}, c, { open: true, working: false, queued: [] }); },
        RenameSession: async (id, name) => { __calls.push(['rename', id, name]); titles[id] = name; return Object.assign({}, titles); },
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: bus.inbox, unread: bus.inbox.filter(m => m.unread).length, rooms: bus.rooms, workspaces: bus.workspaces, links: bus.links, defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusTrust: async (a, b, hops) => { __calls.push(['trust', a, b, hops]); const label = n => bus.workspaces.find(w => w.name === n).label; bus.links = bus.links.filter(l => !(l.a === a && l.b === b)).concat([{ a, b, aLabel: label(a), bLabel: label(b), hopLimit: hops }]); },
        BusUntrust: async (a, b) => { __calls.push(['untrust', a, b]); bus.links = bus.links.filter(l => !(l.a === a && l.b === b)); },
        MarkBusInboxRead: async () => { __calls.push(['read']); bus.inbox.forEach(m => { m.unread = false; }); },
        BusThread: async id => bus.threads[id === 3 ? 1 : id] || [],
        BusSend: async (to, text, replyTo) => {
          __calls.push(['send', to, text, replyTo]);
          if (!to.trim()) throw new Error('say who the message is for');
          return msg(next++, 'Paul', to, text);
        },
        BusReplyInThread: async (id, text) => { __calls.push(['reply', id, text]); const m = msg(next++, 'Paul', 'Parser Release', text, { replyTo: 1 }); bus.threads[1].push(m); return m; },
        BusGroupMessages: async key => bus.roomMessages[key] || [],
        BusMarkGroupRead: async key => { __calls.push(['read-group', key]); const r = bus.rooms.find(r => r.name === key); if (r) r.unread = 0; },
        BusSendToGroup: async (key, text, mentions) => {
          __calls.push(['group-send', key, text, mentions]);
          const r = bus.rooms.find(r => r.name === key);
          const m = msg(next++, 'Paul', r.title, text, { room: key, roomTitle: r.title, mentions: mentions.map(id => ({ id, title: r.members.find(x => x.id === id).title })) });
          (bus.roomMessages[key] ||= []).push(m);
          return m;
        },
        BusStartGroup: async (name, refs) => {
          __calls.push(['start', name, refs]);
          const r = { name: 'new-group', title: name || refs.map(x => x.title).join(' and '), lastAt: '', unread: 0, members: refs.map((x, i) => ({ id: 'window-n' + i, chat: x.id, title: x.title, workspace: 'parser', address: '', state: 'closed' })).concat([person]) };
          bus.rooms.push(r);
          return r;
        },
        BusAddToGroup: async (key, refs) => { __calls.push(['add', key, refs]); },
        BusRemoveFromGroup: async (key, id) => { __calls.push(['remove-member', key, id]); const r = bus.rooms.find(r => r.name === key); r.members = r.members.filter(m => m.id !== id); },
        BusDeleteGroup: async key => { __calls.push(['delete', key]); bus.rooms = bus.rooms.filter(r => r.name !== key); }
      };
    },
    init: () => { window.__calls = []; },
    ready: () => document.querySelector('[data-win="workspace"]'),
  });
  const { page: p, ok } = t;
  p.on('console', m => { if (m.type() === 'error') console.log('console: ' + m.text()); });
  try {
    const viewEntry = await p.evaluate(() => window.dinahCommands.rows('view').map(r => r.label).filter(l => /Group Chats|^Bus$/.test(l)));
    ok('the View menu lists the pane as Group Chats', JSON.stringify(viewEntry) === '["Group Chats"]', JSON.stringify(viewEntry));
    await p.evaluate(() => window.dinahCommands.run('bus'));
    const pane = p.locator('[data-win="bus"]');
    await pane.locator('[data-bus-message="3"]').waitFor();
    await pane.locator('[data-bus-chat="chat-outside"]').waitFor();
    ok('the pane is titled Group Chats', (await pane.locator('.win-title').innerText()) === 'Group Chats');
    ok('the toolbar offers New Group Chat… and New Message…', await pane.locator('.browser-commands').getByRole('button', { name: 'New Group Chat…' }).count() === 1 &&
      await pane.locator('.browser-commands').getByRole('button', { name: 'New Message…' }).count() === 1);
    const sections = await pane.locator('[data-section]').evaluateAll(els => els.map(e => e.dataset.section));
    ok('the pane has the inbox, the group chats, the chats and the trusted links, in that order', JSON.stringify(sections) === JSON.stringify(['bus-inbox', 'bus-groups', 'bus-chats', 'bus-links']), JSON.stringify(sections));
    const inbox = await pane.locator('[data-section="bus-inbox"]').innerText();
    ok('the inbox lists Paul\'s messages, newest first, the unread one marked', inbox.indexOf('Parser Release') < inbox.indexOf('Guide Fixes') && /New/.test(inbox) && inbox.includes('Release 2.4 is tagged.') && !inbox.includes('Notes are in'), inbox);
    ok('an inbox row shows the sending chat\'s title and its workspace, and a group\'s name', inbox.includes('Parser Release (Parser Library)') && inbox.includes('Guide Fixes (Docs Site)') && inbox.includes('Docs Pass (docs) in Release Review'), inbox);
    const groupsText = await pane.locator('[data-section="bus-groups"]').innerText();
    ok('a group chat is listed by its name, with its members and its unread messages', groupsText.includes('Release Review') && groupsText.includes('Parser Release, Docs Pass and you') && /2 New/.test(groupsText), groupsText);
    ok('a group with unread messages shows who sent the newest and its line, and a group with none shows no line', groupsText.includes('Docs Pass: The guide is ready for a second look') && (groupsText.match(/second look/g) || []).length === 1, groupsText);
    const chatsText = await pane.locator('[data-section="bus-chats"]').innerText();
    ok('every chat is listed under its workspace, closed ones too, with its state', /DOCS[\s\S]*Docs Pass[\s\S]*Open[\s\S]*Outside Chat[\s\S]*Closed[\s\S]*PARSER[\s\S]*Parser Release[\s\S]*Closed[\s\S]*Untitled Chat in parser/i.test(chatsText), chatsText);
    ok('a chat\'s state has its glyph: open is the positive badge, closed the plain one', await pane.locator('[data-bus-chat="chat-docs"] .badge.positive').count() === 1 && await pane.locator('[data-bus-chat="chat-outside"] .badge:not(.positive):not(.accent)').count() === 1);
    await p.evaluate(() => __emit('agent:session', { sessionId: 'chat-docs', cwd: 'C:/Work/docs', title: 'Docs Pass', open: true, working: true }));
    await pane.locator('[data-bus-chat="chat-docs"] .badge.accent.running').waitFor();
    ok('a working chat shows Working with the ring', (await pane.locator('[data-bus-chat="chat-docs"]').innerText()).includes('Working'));
    const whole = await pane.innerText();
    ok('no section shows a handle or the old list of who is on the bus', !/parser-1|docs-1|On the Bus/.test(whole), whole);

    await pane.getByRole('button', { name: 'New Message…' }).click();
    const suggested = await pane.locator('.bus-form datalist option').evaluateAll(os => os.map(o => o.value));
    ok('the To field suggests chats by title and workspace, workspaces by name, and groups by name', suggested.includes('Docs Pass (Docs Site)') && suggested.includes('Docs Site') && suggested.includes('#Release Review') && !suggested.some(v => /docs-1/.test(v)), JSON.stringify(suggested));
    await pane.locator('.bus-form input').fill('Docs Site');
    await pane.locator('.bus-form textarea').fill('Please check the guide against 2.4.');
    await pane.locator('.bus-form').getByRole('button', { name: 'Send' }).click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'send' && c[1] === 'Docs Site'));
    await p.waitForFunction(() => !document.querySelector('[data-win="bus"] .bus-form'));
    ok('a new message goes to the address typed, and the form closes', true);
    await pane.getByRole('button', { name: 'New Message…' }).click();
    await pane.locator('.bus-form textarea').fill('Nobody to send to.');
    await pane.locator('.bus-form').getByRole('button', { name: 'Send' }).click();
    await p.locator('.toast', { hasText: 'say who the message is for' }).first().waitFor();
    ok('a message the bus refuses keeps the form and says why', await pane.locator('.bus-form textarea').inputValue() === 'Nobody to send to.');
    await pane.locator('.bus-form').getByRole('button', { name: 'Cancel' }).click();

    await pane.locator('[data-bus-message="3"]').click();
    const thread = p.locator('.win[data-win-kind="bus-thread"]');
    await thread.locator('.bus-message').nth(1).waitFor();
    ok('a message opens its whole thread, from the question it answers', (await thread.locator('.bus-message').count()) === 2 && (await thread.locator('.bus-message').first().innerText()).includes('Is 2.4 ready?'));
    const answer = await thread.locator('.bus-message').nth(1).innerText();
    ok('a thread heads each message with the chat\'s title and workspace, and the person by name', answer.includes('Parser Release') && /in\s+Parser Library/.test(answer) && /Paul[\s\S]*to\s+Parser Library/.test(await thread.locator('.bus-message').first().innerText()), answer);
    ok('a thread\'s window is named for the chat', await p.locator('.win[data-win-kind="bus-thread"] .win-title').innerText() === 'Thread With Parser Release');
    await thread.locator('textarea').fill('Thanks, announce it.');
    await thread.locator('textarea').press('Control+Enter');
    await thread.locator('.bus-message').nth(2).waitFor();
    ok('Ctrl+Enter replies in the thread', await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'reply'))) === JSON.stringify(['reply', 1, 'Thanks, announce it.']));

    // A group's window: its members, its conversation and its box.
    await pane.locator('[data-bus-room="release-review"]').click();
    const room = p.locator('.win[data-win-kind="bus-room"]').filter({ has: p.locator('.win-title', { hasText: 'Release Review' }) });
    await room.locator('.bus-message').first().waitFor();
    ok('a group opens in its own window, titled with its name', await room.count() === 1);
    const chips = await room.locator('.group-members .member-chip').allInnerTexts();
    ok('the members are chips: each chat by its title, then the person', JSON.stringify(chips.map(t => t.trim())) === JSON.stringify(['Parser Release', 'Docs Pass', 'Paul (You)']), JSON.stringify(chips));
    // A chip shows a chat in the window as the window has it, so Docs Pass,
    // which the bus has as open, is working, as the window was told above.
    ok('each chat\'s chip shows its state as a glyph with its word', await room.locator('[data-group-member="window-p1"] .state-glyph.closed[aria-label="Closed"]').count() === 1 &&
      await room.locator('[data-group-member="window-d1"] .state-glyph.working[aria-label="Working"]').count() === 1);
    ok('the members are followed by Add Chats…', await room.locator('.group-members').getByRole('button', { name: 'Add Chats…' }).count() === 1);
    const said = await room.locator('.bus-log').innerText();
    ok('the conversation shows each message under its sender\'s title, with its mentions marked', said.includes('Parser Release') && await room.locator('[data-bus-message="4"] .mention').innerText() === '@Docs Pass', said);
    ok('a line says who was added to the group', await room.locator('.system-line[data-bus-message="5"]').innerText() === 'Paul added Docs Pass to the group.');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'read-group' && c[1] === 'release-review'));
    ok('opening the group marks its messages read', true);

    // The @ list: the members, everyone, then Add a Chat…; a mention goes in whole.
    const box = room.locator('textarea');
    await box.click();
    await box.pressSequentially('Thanks. @');
    const pop = room.locator('.group-mention-pop');
    await pop.waitFor();
    await t.shot('bus-group');
    const offered = await pop.locator('.mention-row').allInnerTexts();
    ok('@ offers the group\'s members, everyone, and Add a Chat… after them', offered.length === 4 && /Parser Release/.test(offered[0]) && /Docs Pass/.test(offered[1]) && /everyone/.test(offered[2]) && /Add a Chat…/.test(offered[3]) && !offered.some(o => /Outside Chat/.test(o)), JSON.stringify(offered));
    await box.pressSequentially('do');
    ok('typing narrows the list', (await pop.locator('.mention-row').count()) === 2 && /Docs Pass/.test(await pop.locator('.mention-row').first().innerText()));
    await box.press('Enter');
    ok('choosing a member puts its mention in whole', await box.inputValue() === 'Thanks. @Docs Pass ');
    await box.press('Backspace');
    await box.press('Backspace');
    ok('Backspace takes a mention out whole', await box.inputValue() === 'Thanks. ', JSON.stringify(await box.inputValue()));
    await box.pressSequentially('@Doc');
    await box.press('Tab');
    await box.pressSequentially(', please look at the guide.');
    ok('the @ list closes once a mention is chosen and the message goes on', await pop.isHidden());
    await box.pressSequentially(' @zz');
    ok('typing after @ that matches no member closes the list', await pop.isHidden());
    for (let i = 0; i < 4; i++) await box.press('Backspace');
    await box.press('Backspace');
    await room.getByRole('button', { name: 'Send' }).click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'group-send'));
    ok('the message is sent with the list of chats it mentions', await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'group-send'))) === JSON.stringify(['group-send', 'release-review', 'Thanks. @Docs Pass , please look at the guide', ['window-d1']]),
      await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'group-send'))));
    await room.locator('.bus-message', { hasText: 'please look at the guide' }).waitFor();
    await p.evaluate(() => { __bus.roomMessages['release-review'].push({ id: 50, createdAt: '2026-10-22T09:00:00Z', kind: 'text', from: 'Docs Pass', fromPerson: false, fromSession: 'window-d1', fromWorkspace: 'docs', to: 'Release Review', room: 'release-review', roomTitle: 'Release Review', mentions: [], replyTo: 0, text: 'Seen.', attachments: [], unread: false }); __emit('bus:changed', { counter: 9 }); });
    await room.locator('[data-bus-message="50"]').waitFor();
    ok('a group draws again when the bus changes, each message under its chat\'s title', (await room.locator('[data-bus-message="50"]').innerText()).includes('Docs Pass'));

    // A chip's menu opens the chat or takes it out of the group.
    await room.locator('[data-group-member="window-d1"] .member-menu-btn').click();
    const chipMenu = room.locator('.group-member-menu');
    await chipMenu.waitFor();
    ok('a chip\'s menu offers Open Chat and Remove From Group…', JSON.stringify(await chipMenu.locator('.menu-action').allInnerTexts()) === JSON.stringify(['Open Chat', 'Remove From Group…']));
    await chipMenu.getByText('Remove From Group…').click();
    await p.locator('#ask-yes').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'remove-member' && c[1] === 'release-review' && c[2] === 'window-d1'));
    await room.locator('[data-group-member="window-d1"]').waitFor({ state: 'detached' });
    ok('Remove From Group… asks, then takes the chat out', true);

    // The group's menu has Delete Group… and no Leave.
    await p.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click();
    const groupActions = await p.locator('#menubar-panel .menu-action').allInnerTexts();
    ok('the group\'s menu has Delete Group… and no Leave', groupActions.some(a => /Delete Group…/.test(a)) && !groupActions.some(a => /Leave/.test(a)), JSON.stringify(groupActions));
    await p.locator('#menubar-panel').getByText('Delete Group…').click();
    await p.locator('#ask-yes').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'delete' && c[1] === 'release-review'));
    await p.waitForFunction(() => ![...document.querySelectorAll('.win[data-win-kind="bus-room"] .win-title')].some(t => t.textContent === 'Release Review'));
    ok('Delete Group… asks, deletes the group and closes its window', true);

    // The picker: every chat by workspace, a filter, ticks, a title asked for in place.
    await pane.locator('.browser-commands').getByRole('button', { name: 'New Group Chat…' }).click();
    const picker = p.locator('.win[data-win-kind="group-picker"]');
    await picker.locator('[data-chat="chat-parser"]').waitFor();
    ok('the picker is a window titled Start a Group Chat', await picker.locator('.win-title').innerText() === 'Start a Group Chat');
    const pickerText = await picker.locator('.picker-list').innerText();
    ok('the picker lists every chat by workspace with its provider and state', /DOCS[\s\S]*Docs Pass[\s\S]*Claude[\s\S]*Working[\s\S]*Outside Chat[\s\S]*Ollama[\s\S]*Closed[\s\S]*PARSER[\s\S]*Parser Release/i.test(pickerText), pickerText);
    ok('with nothing ticked, Start waits for a chat', await picker.getByRole('button', { name: 'Start' }).isDisabled() && (await picker.locator('.picker-count').innerText()).includes('Choose at least one chat'));
    await picker.locator('.picker-filter').fill('ollama');
    ok('the filter narrows the list', await picker.locator('.listbox-row:visible').count() === 1 && await picker.locator('[data-chat="chat-outside"]').isVisible());
    await picker.locator('.picker-filter').fill('');
    await picker.locator('[data-chat="chat-parser"]').click();
    ok('a closed chat ticked shows its note', /is closed/.test(await picker.locator('[data-chat="chat-parser"] .row-note').innerText()) && await picker.locator('[data-chat="chat-parser"]').getAttribute('aria-selected') === 'true');
    ok('an untitled chat says it needs a title', /title/.test(await picker.locator('[data-chat="chat-blank"] .row-note').innerText()));
    await picker.locator('[data-chat="chat-blank"]').click();
    const give = picker.locator('[data-chat="chat-blank"] .picker-title-input');
    await give.waitFor();
    ok('choosing an untitled chat asks for its title in place, without ticking it', await picker.locator('[data-chat="chat-blank"]').getAttribute('aria-selected') === 'false');
    await give.fill('Lexer Work');
    await give.press('Enter');
    await picker.locator('[data-chat="chat-blank"][aria-selected="true"]').waitFor();
    ok('the title is given to the chat, which is then ticked', await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'rename'))) === JSON.stringify(['rename', 'chat-blank', 'Lexer Work']) &&
      (await picker.locator('[data-chat="chat-blank"]').innerText()).includes('Lexer Work'));
    ok('the name the group would take is the placeholder', await picker.locator('.picker-name').getAttribute('placeholder') === 'Parser Release and Lexer Work' &&
      (await picker.locator('.picker-count').innerText()) === 'The group will have 2 chats and you.');
    await t.shot('bus-picker');
    const pickerKey = await picker.getAttribute('data-win');
    ok('the picker has an address, with its choices', await p.evaluate(key => { const q = new URLSearchParams(location.search); return (q.get('open') || '').split(',').includes(key) && /chat-blank/.test(q.get('view.' + key) || ''); }, pickerKey));
    await picker.getByRole('button', { name: 'Start' }).click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'start'));
    ok('Start makes the group of the chats ticked, named from them', await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'start'))) ===
      JSON.stringify(['start', '', [{ id: 'chat-parser', cwd: 'C:/Work/parser', title: 'Parser Release' }, { id: 'chat-blank', cwd: 'C:/Work/parser', title: 'Lexer Work' }]]));
    await p.locator('.win[data-win-kind="bus-room"] .win-title', { hasText: 'Parser Release and Lexer Work' }).waitFor();
    ok('the group\'s window opens and the picker closes', await p.locator('.win[data-win-kind="group-picker"]').count() === 0);

    // A chat window's Actions menu starts a group and adds the chat to one.
    await p.evaluate(() => window.dinahChats.open('chat-parser'));
    await p.locator('.win.active[data-win-kind="chat-session"] [data-view-id="session-title"]', { hasText: 'Parser Release' }).waitFor();
    await p.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click();
    const chatActions = await p.locator('#menubar-panel .menu-action').allInnerTexts();
    const at = label => chatActions.findIndex(a => a.includes(label));
    ok('a chat\'s Actions menu has Start a Group Chat… and Add to a Group Chat… after Pin to Workspace', at('Pin to Workspace') >= 0 && at('Start a Group Chat…') === at('Pin to Workspace') + 1 && at('Add to a Group Chat…') === at('Start a Group Chat…') + 1, JSON.stringify(chatActions));
    await p.locator('#menubar-panel').getByText('Add to a Group Chat…').click();
    const sub = p.locator('#menubar-sub');
    await sub.locator('.menu-action').first().waitFor();
    const subRows = await sub.locator('.menu-action').allInnerTexts();
    ok('Add to a Group Chat… lists the groups the chat is not in, with New Group Chat… at its foot', subRows.length === 2 && /Docs Review/.test(subRows[0]) && /New Group Chat…/.test(subRows[1]), JSON.stringify(subRows));
    await sub.getByText('Docs Review').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'add'));
    ok('choosing a group puts the chat in it', await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'add'))) === JSON.stringify(['add', 'docs-review', [{ id: 'chat-parser', cwd: 'C:/Work/parser', title: 'Parser Release' }]]));
    await p.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click();
    await p.locator('#menubar-panel').getByText('Start a Group Chat…').click();
    const fromChat = p.locator('.win[data-win-kind="group-picker"]');
    await fromChat.locator('[data-chat="chat-parser"]').waitFor();
    ok('Start a Group Chat… opens the picker with the chat ticked and marked', await fromChat.locator('[data-chat="chat-parser"]').getAttribute('aria-selected') === 'true' &&
      (await fromChat.locator('[data-chat="chat-parser"]').innerText()).includes('This Chat'));
    ok('the chat\'s workspace comes first', /^PARSER/i.test((await fromChat.locator('.picker-list').innerText()).trim()));
    await fromChat.getByRole('button', { name: 'Cancel' }).click();
    await fromChat.waitFor({ state: 'detached' });

    // Add Chats… in a group's window opens the picker for that group.
    await pane.locator('[data-bus-room="docs-review"]').click();
    const docsRoom = p.locator('.win[data-win-kind="bus-room"]').filter({ has: p.locator('.win-title', { hasText: 'Docs Review' }) });
    await docsRoom.locator('.group-members').getByRole('button', { name: 'Add Chats…' }).click();
    const adding = p.locator('.win[data-win-kind="group-picker"]');
    await adding.locator('[data-chat="chat-docs"]').waitFor();
    ok('Add Chats… opens the picker for the group, its members shown as in it', await adding.locator('.win-title').innerText() === 'Add Chats to Docs Review' &&
      await adding.locator('[data-chat="chat-docs"]').getAttribute('aria-disabled') === 'true' && await adding.locator('.picker-name').count() === 0);
    await adding.getByRole('button', { name: 'Cancel' }).click();

    // Trusted links: one made with the offered limit, its limit turned off
    // only after a warning, then removed after a question.
    await pane.locator('[data-section="bus-links"]').getByRole('button', { name: 'Trust a Link…' }).click();
    const form = pane.locator('[data-bus-link-form]');
    await form.locator('select').nth(1).selectOption('docs');
    ok('a new link is offered the default limit', await form.locator('input[type=number]').inputValue() === '10');
    ok('the link\'s workspaces are chosen by name', JSON.stringify(await form.locator('select').nth(1).locator('option').allInnerTexts()) === JSON.stringify(['Parser Library', 'Docs Site']));
    await form.getByRole('button', { name: 'Trust Link' }).click();
    await pane.locator('[data-bus-link="parser|docs"]').waitFor();
    ok('a link is trusted with its limit and listed', await p.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'trust'))) === JSON.stringify(['trust', 'parser', 'docs', 10]) &&
      (await pane.locator('[data-bus-link="parser|docs"]').innerText()).includes('Up to 10 automatic handoffs'));
    ok('a link shows its workspaces by name', (await pane.locator('[data-bus-link="parser|docs"]').innerText()).includes('Parser Library and Docs Site'));
    await pane.locator('[data-bus-link="parser|docs"]').click();
    await form.getByRole('switch', { name: 'No Limit' }).click();
    await form.getByRole('button', { name: 'Save' }).click();
    await p.locator('#ask[open]').waitFor();
    ok('turning the limit off warns first', (await p.locator('#ask-title').innerText()) === 'Turn Off the Limit?');
    await p.locator('#ask-no').click();
    ok('declining the warning keeps the limit', !(await p.evaluate(() => __calls.some(c => c[0] === 'trust' && c[3] === -1))));
    await form.getByRole('button', { name: 'Save' }).click();
    await p.locator('#ask-yes').click();
    await pane.locator('[data-bus-link="parser|docs"]', { hasText: 'No Limit' }).waitFor();
    ok('accepting it turns the limit off, and the link is marked', true);
    await pane.locator('[data-bus-link="parser|docs"]').click();
    await form.getByRole('button', { name: 'Remove Link…' }).click();
    await p.locator('#ask-yes').click();
    await pane.locator('[data-section="bus-links"]', { hasText: 'No Trusted Links' }).waitFor();
    ok('a link is removed after a question', await p.evaluate(() => __calls.some(c => c[0] === 'untrust' && c[1] === 'parser' && c[2] === 'docs')));
    await t.shot('bus');
  } catch (err) {
    t.errors.push(String(err && err.stack || err));
  } finally {
    await t.close();
  }
})();
