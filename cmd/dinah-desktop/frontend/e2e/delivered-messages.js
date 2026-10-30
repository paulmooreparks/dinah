// Messages the bus delivers into a chat are cards of their own, never the
// person's messages (delivered.go): a reply from another chat, another
// agent's message, and two messages delivered in one prompt as two cards.
// The same cards come from a replay Dinah recognised and from the saved
// copy, a long message is folded behind PUDL's disclosure, and a result's
// status is a glyph and a word. A card from a chat shows the chat's title as
// it is now, and its workspace by name. Against a stub of the Go side.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    app: () => {
      window.__calls = [];
      const ws = 'C:\\Work\\Dinah';
      const reply = { role: 'delivered', text: 'Request ee23 is done.\n\nReply from the chat "PUDL Development" (session codex:pudl):\n\n1. **Empty groups are supported.**\n2. Take v0.88.0 now.', provider: 'codex', delivered: { id: 41, kind: 'result', from: 'PUDL Development', time: '2026-10-26T09:15:00Z', status: 'done' } };
      const note = { role: 'delivered', text: 'The parser release is out.', provider: 'codex', delivered: { id: 42, kind: 'text', from: 'Parser Release Check', fromSession: 'window-parser', workspace: 'Parser', time: '2026-10-26T09:20:00Z' } };
      const long = { role: 'delivered', text: Array.from({ length: 40 }, (_, i) => 'Line ' + (i + 1) + ' of the release notes.').join('\n'), provider: 'codex', delivered: { id: 43, kind: 'release', from: 'PUDL Development', fromSession: 'window-pudl', workspace: 'PUDL', time: '2026-10-26T09:25:00Z' } };
      // The bus's chats, as the Bus pane's data gives them: empty until the
      // test renames a chat.
      window.__busOverview = { inbox: [], unread: 0, rooms: [], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 };
      window.__saved = [
        { role: 'you', text: 'Ask PUDL about empty groups.', provider: 'codex' },
        { role: 'agent', text: 'I asked the PUDL chat.', provider: 'codex' },
        reply,
        { role: 'agent', text: 'PUDL says empty groups work.', provider: 'codex' },
        note, long
      ];
      window.__entries = { reply, note, long };
      window.__reconnect = new Promise(resolve => { window.__letReconnect = resolve; });
      return {
        HostLocale: async () => 'en-GB-u-hc-h23',
        State: async () => ({ workspace: ws, recent: [ws], approvals: 'dinah', defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [], showThinking: true, personName: window.__personName || '', accountName: 'Paul Parks', busHandle: 'paul' }),
        SetPersonName: async name => { window.__personName = name; },
        AgentProviders: async () => [{ id: 'codex', installed: true, available: true }],
        NewSessionAt: async () => ({ sessionId: 'live', provider: 'codex', cwd: ws, title: 'Live', open: true, working: false, queued: [] }),
        ResolveSessionContinuation: async id => id,
        SavedConversation: async id => id === 'saved' ? { available: true, complete: true, messages: window.__saved } : { available: false, messages: [] },
        SessionContinuation: async () => ({ history: [] }),
        OpenSessionAt: async (w, id) => {
          window.__calls.push(['open', id]);
          // A restored chat reconnects by itself a moment after its saved copy
          // is drawn, which on a busy machine can come before the test has
          // read the saved copy and set the replay; the reconnect waits here
          // until the test lets it go.
          if (id === 'saved') await window.__reconnect;
          // The agent's replay, as Dinah passes it on once it has recognised the
          // deliveries in it.
          if (window.__replay) window.__emit('agent:updates', { sessionId: id, updates: window.__replay });
          return { sessionId: id, provider: 'codex', title: 'Replayed', cwd: w, open: true };
        },
        Send: async (id, text) => { window.__calls.push(['Send', id, text]); },
        SaveRun: async text => { window.__calls.push(['SaveRun', text]); return ''; },
        Queue: async () => {}, Steer: async () => {}, Unqueue: async () => {}, Answer: async () => {}, Cancel: async () => {}, Stop: async () => {},
        SetWorkspace: async () => ({}), ForgetWorkspace: async () => ({}), ChooseWorkspace: async () => ({}), SetApprovals: async () => {},
        SetDefault: async () => ({}), SetPinned: async () => [], SetShowThinking: async () => {}, SaveHistory: async () => '', ClearHistory: async () => {},
        BusOverview: async () => window.__busOverview
      };
    },
  });
  const p = require('./lib/windows')(t.page);
  const ok = t.ok;

  // A live chat: the person asks, and the bus delivers.
  const win = '.win[data-chat-current]';
  await p.fill(win + ' [data-view-id=prompt]', 'Ask PUDL about empty groups.');
  await p.click(win + ' [data-view-id=send]');
  await p.waitForFunction(() => __calls.some(c => c[0] === 'Send' && c[1] === 'live'), null, { timeout: 30000 });
  // Each delivery is drawn in a frame the render queue schedules, so a check
  // waits for what it reads to be on the page. A wait that times out leaves
  // the check to fail.
  const drawn = (f, arg) => p.waitForFunction(f, arg, { timeout: 30000 }).catch(() => {});
  // A card's body is drawn as Markdown after the card itself.
  const cardCount = n => drawn(([s, n]) => { const all = document.querySelectorAll(s + ' .delivered-card'); return all.length === n && [...all].every(c => c.querySelector('.delivered-text').innerHTML !== ''); }, [win, n]);
  const cards = sel => p.evaluate(s => Array.from(document.querySelectorAll(s + ' .delivered-card')).map(c => ({
    kind: c.dataset.kind,
    heading: c.querySelector('.delivered-heading').textContent,
    badge: c.querySelector('.badge') ? { text: c.querySelector('.badge').textContent, cls: c.querySelector('.badge').className, glyph: getComputedStyle(c.querySelector('.badge'), '::before').content } : null,
    open: c.querySelector('.card-toggle').getAttribute('aria-expanded') === 'true',
    bodyShown: getComputedStyle(c.querySelector('.card-body')).display !== 'none',
    html: c.querySelector('.delivered-text').innerHTML,
    summary: c.querySelector('.delivered-summary').textContent,
    time: c.closest('li').querySelector('.activity-time').dateTime,
    shadow: getComputedStyle(c).boxShadow,
    inYouRow: !!c.closest('.msg-you')
  })), sel);
  const youRows = sel => p.evaluate(s => Array.from(document.querySelectorAll(s + ' .msg-you')).map(m => m.textContent), sel);

  await p.evaluate(() => window.__emit('agent:sent', { sessionId: 'live', kind: 'bus', label: 'Messages from the bus', text: 'Dinah bus: 1 new message.\n...', delivered: [window.__entries.reply] }));
  await cardCount(1);
  let c = await cards(win);
  ok('a reply from another chat is a card of its own', c.length === 1 && c[0].kind === 'result' && c[0].heading === 'Reply from PUDL Development', JSON.stringify(c));
  ok('the card is not one of the person\'s messages', c.length === 1 && !c[0].inYouRow && (await youRows(win)).every(t => !t.includes('Dinah bus')), JSON.stringify(await youRows(win)));
  ok('its status is a glyph and a word', c[0].badge && c[0].badge.text === 'done' && /positive/.test(c[0].badge.cls) && c[0].badge.glyph !== 'none', JSON.stringify(c[0].badge));
  ok('its body is the reply drawn as Markdown', /<strong>Empty groups are supported\.<\/strong>/.test(c[0].html) && /<ol>/.test(c[0].html), c[0].html);
  ok('its time is when it was sent', c[0].time === '2026-10-26T09:15:00.000Z', c[0].time);
  ok('the card is flat, as display is', c[0].shadow === 'none', c[0].shadow);

  await p.evaluate(() => window.__emit('agent:sent', { sessionId: 'live', kind: 'bus', label: 'Messages from the bus', text: 'Dinah bus: 2 new messages.', delivered: [window.__entries.note, window.__entries.long] }));
  await cardCount(3);
  c = await cards(win);
  ok('two messages delivered in one prompt are two cards, each from a chat by its title', c.length === 3 && c[1].heading === 'Message from Parser Release Check (workspace Parser)' && c[2].heading === 'Release notice from PUDL Development (workspace PUDL)', JSON.stringify(c.map(x => x.heading)));
  ok('another agent\'s message has no status', c[1].badge === null && c[1].open && c[1].bodyShown);
  ok('a long message is folded, its first line showing', !c[2].open && !c[2].bodyShown && c[2].summary === 'Line 1 of the release notes.', JSON.stringify(c[2]));
  await p.locator(win + ' .delivered-card[data-kind=release] .card-toggle').click();
  await drawn(s => { const c = document.querySelector(s + ' .delivered-card[data-kind=release]'); return c.querySelector('.card-toggle').getAttribute('aria-expanded') === 'true' && getComputedStyle(c.querySelector('.card-body')).display !== 'none'; }, win);
  c = await cards(win);
  ok('the disclosure opens it', c[2].open && c[2].bodyShown && c[2].html.includes('Line 40 of the release notes.'), JSON.stringify({ open: c[2].open, shown: c[2].bodyShown }));
  // The picture: the window maximised, at the reply's card.
  await p.evaluate(w => { pudlWindows.snap(document.querySelector(w).dataset.win, 'maximized'); }, win);
  await p.waitForTimeout(300);
  await p.evaluate(w => document.querySelector(w + ' .delivered-card').scrollIntoView({ block: 'start' }), win);
  await p.waitForTimeout(200);
  await t.shot('delivered-messages');
  await p.evaluate(() => window.__emit('agent:sent', { sessionId: 'live', kind: 'bus', label: 'Messages from the bus', text: 'Dinah bus: 1 new message. Old.' }));
  await drawn(s => Array.from(document.querySelectorAll(s + ' .msg-you')).some(m => m.textContent.includes('Old.')), win);
  ok('an event without entries, from before they were kept, is drawn as before', (await youRows(win)).some(t => t.includes('Messages from the bus') && t.includes('Old.')), JSON.stringify(await youRows(win)));

  // A chat renamed after its message came is shown by its new title on the
  // card already drawn, and a chat no longer on the bus keeps the title it
  // had when the message came.
  const heading42 = () => p.evaluate(s => { const c = document.querySelector(s + ' .delivered-card[data-kind=text] .delivered-heading'); return c && c.textContent; }, win);
  await p.evaluate(() => {
    window.__busOverview = Object.assign({}, window.__busOverview, { workspaces: [{ name: 'parser', label: 'Parser Library', path: 'C:/Work/parser', chats: [{ id: 'window-parser', title: 'Parser 2.4 Release', address: 'Parser 2.4 Release (parser)' }] }] });
    window.__emit('bus:changed', { counter: 2 });
  });
  await drawn(s => document.querySelector(s + ' .delivered-card[data-kind=text] .delivered-heading').textContent.includes('2.4'), win);
  ok('a renamed chat\'s card shows its new title and workspace', (await heading42()) === 'Message from Parser 2.4 Release (workspace Parser Library)', await heading42());
  await p.evaluate(() => {
    window.__busOverview = Object.assign({}, window.__busOverview, { workspaces: [] });
    window.__emit('bus:changed', { counter: 3 });
  });
  await drawn(s => !document.querySelector(s + ' .delivered-card[data-kind=text] .delivered-heading').textContent.includes('2.4'), win);
  ok('a chat no longer on the bus keeps the title its message came with', (await heading42()) === 'Message from Parser Release Check (workspace Parser)', await heading42());

  // The person's own bus messages carry their name, not their bus handle:
  // the account's name until they choose one, then the one they chose,
  // which also rewrites the cards already drawn (person_name.go).
  await p.evaluate(() => window.__emit('agent:sent', { sessionId: 'live', kind: 'bus', label: 'Messages from the bus', text: 'Dinah bus: 1 new message.', delivered: [{ role: 'delivered', text: 'Please look at the parser.', provider: 'codex', delivered: { id: 50, kind: 'text', from: 'paul', time: '2026-10-26T09:30:00Z' } }] }));
  await cardCount(4);
  const mine = () => p.evaluate(w => Array.from(document.querySelectorAll(w + ' .delivered-heading')).map(h => h.textContent).pop(), win);
  ok('the person\'s bus message shows the account\'s name, not the handle', (await mine()) === 'Message from Paul Parks', await mine());
  await p.evaluate(() => { const f = document.getElementById('set-person-name'); f.value = 'P. Parks'; f.dispatchEvent(new Event('change')); });
  await drawn(s => Array.from(document.querySelectorAll(s + ' .delivered-heading')).pop().textContent === 'Message from P. Parks', win);
  ok('a chosen name replaces it, on the cards already drawn too', (await mine()) === 'Message from P. Parks' && await p.evaluate(() => window.__personName === 'P. Parks'), await mine());

  // The same cards from the saved copy of a restored chat.
  await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:\\Work\\Dinah', path: 'saved' }));
  const front = '.win.active';
  await p.locator(front + ' .delivered-card').first().waitFor();
  c = await cards(front);
  ok('the saved copy draws the same cards', c.length === 3 && c[0].heading === 'Reply from PUDL Development' && c[0].badge && c[0].badge.text === 'done' && c[1].heading === 'Message from Parser Release Check (workspace Parser)' && !c[2].open, JSON.stringify(c.map(x => [x.heading, x.open])));
  ok('the saved copy keeps the person\'s own message as theirs', (await youRows(front)).length === 1 && (await youRows(front))[0].includes('Ask PUDL about empty groups.'));
  ok('the saved cards keep their sent times', c[1].time === '2026-10-26T09:20:00.000Z', c[1].time);
  await t.shot('delivered-messages-saved');

  // A replay with the deliveries Dinah recognised in it draws them alike.
  await p.evaluate(() => {
    window.__replay = [
      { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Ask PUDL about empty groups.' } },
      { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'I asked the PUDL chat.' } },
      { sessionUpdate: 'dinah_delivered', message: window.__entries.reply },
      { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'PUDL says empty groups work, replayed.' } },
      { sessionUpdate: 'dinah_delivered', message: window.__entries.note },
      { sessionUpdate: 'dinah_delivered', message: window.__entries.long }
    ];
    window.__letReconnect();
  });
  // The chat may already be reconnecting by itself; Reconnect Now starts it
  // otherwise.
  if (!(await p.evaluate(() => __calls.some(c => c[0] === 'open' && c[1] === 'saved')))) await p.locator(front + ' [data-view-id="saved-reconnect"]').click({ timeout: 5000 }).catch(() => {});
  await p.waitForFunction(() => __calls.some(c => c[0] === 'open' && c[1] === 'saved'));
  await p.locator(front + ' .msg', { hasText: 'empty groups work, replayed.' }).waitFor({ timeout: 30000 });
  await drawn(f => { const all = document.querySelectorAll(f + ' .delivered-card'); return all.length === 3 && [...all].every(c => c.querySelector('.delivered-text').innerHTML !== ''); }, front);
  c = await cards(front);
  ok('the replay draws the same cards', c.length === 3 && c[0].heading === 'Reply from PUDL Development' && c[2].heading === 'Release notice from PUDL Development (workspace PUDL)' && !c[2].open, JSON.stringify(c.map(x => [x.heading, x.open])));
  ok('the replay keeps the times they were sent', c[0].time === '2026-10-26T09:15:00.000Z' && c[1].time === '2026-10-26T09:20:00.000Z', JSON.stringify(c.map(x => x.time)));
  ok('the replay keeps the person\'s own message as theirs', (await youRows(front)).length === 1);

  // Export Chat writes each card under its sender, not as the person's.
  await p.evaluate(() => pudlCommands.run('export-chat', undefined, document.querySelector('.win.active .chat-instance')));
  await drawn(() => __calls.some(c => c[0] === 'SaveRun'));
  const exported = await p.evaluate(() => (__calls.find(c => c[0] === 'SaveRun') || [])[1] || '');
  ok('an exported chat gives each card its sender\'s heading', /## Reply from PUDL Development, \d\d:\d\d\n\nRequest ee23 is done\./.test(exported) && /## Message from Parser Release Check \(workspace Parser\), /.test(exported) && (exported.match(/## You,/g) || []).length === 1, exported.slice(0, 600));
  await t.close();
})();
