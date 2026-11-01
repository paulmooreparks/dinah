// The Inbox (docs/design/inbox.md). It shows unread messages by default,
// counting every unread one; each row leads with the sending chat by name. Opening one
// shows that exact message, highlighted, in its group (loading the group
// from before it when it is older than the newest messages) or in a thread
// that starts with it, and marks that one message read once it is shown,
// with no blanket group read. Dismiss persists across a reload and does
// not mark read; All and Dismissed show the rest, with Load Older; Mark
// All Read is bounded and confirmed; the counts reach zero truthfully.
const { launch, PAGE } = require('./lib/page');
(async () => {
  // The backend's state, kept outside the page so it outlives a reload.
  const room = { name: 'team-a', title: 'Team A', members: [] };
  const all = [];
  let id = 0;
  const add = (o) => { const m = Object.assign({ id: ++id, kind: 'message', createdAt: new Date(Date.UTC(2026, 10, 1, 8, id)).toISOString(), fromPerson: false }, o); all.push(m); return m; };
  // Group traffic: 30 messages, one early mention of the person.
  for (let i = 1; i <= 30; i++) add({ from: 'Engine', room: 'team-a', roomTitle: 'Team A', text: i === 4 ? '@Paul the old schema question is still open.' : 'Group note ' + i + '.', inbox: i === 4 });
  const direct = add({ from: 'Release', fromWorkspace: 'studio', to: 'Paul', text: 'Install finished.', inbox: true });
  const reply = add({ from: 'Quality', fromWorkspace: 'studio', to: 'Paul', text: 'Re: the checks, all passed.', replyTo: direct.id, inbox: true });
  for (let i = 1; i <= 2; i++) add({ from: 'Front End', fromSession: 'fe-1', to: 'Paul', text: 'Status ' + i + ' for you.', inbox: true });
  // A sender gone from the bus, with no title left: the backend sends no name.
  add({ from: '', fromSession: 'yavchn-1', to: 'Paul', text: 'Status 3 for you.', inbox: true });
  // A renamed chat: its title as it is now, its old address only in its words.
  add({ from: 'XferLang Reboot', fromSession: 'xfer-1', fromWorkspace: 'xferlang', to: 'Paul', text: 'From xfer-1: Status 4 for you.', inbox: true });
  const state = { read: new Set(), dismissed: new Set(), calls: [] };
  const inbox = () => all.filter(m => m.inbox).sort((a, b) => b.id - a.id);
  const view = (m) => Object.assign({}, m, { unread: !state.read.has(m.id), dismissed: state.dismissed.has(m.id) });
  const t = await launch({ viewport: { width: 1440, height: 950 }, goto: false, app: () => {
    const call = (op, ...args) => window.__store(op, ...args);
    return {
      State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'] }), Sessions: async () => [], SessionsAt: async () => [],
      ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=bus' } }), SaveUIState: async () => {},
      BusOverview: () => call('overview'), BusResolveGroup: () => call('resolve'),
      BusInboxPage: (v, before) => call('page', v, before), BusInboxMessage: id => call('message', id),
      BusMarkInboxRead: id => call('read', id), BusDismissInbox: id => call('dismiss', id),
      BusMarkInboxReadThrough: id => call('readThrough', id), BusDismissInboxThrough: id => call('dismissThrough', id),
      BusGroupMessages: () => call('group'), BusGroupMessagesFrom: (g, id, n) => call('groupFrom', id, n),
      BusThread: id => call('thread', id), BusMarkGroupRead: () => call('groupRead'), MarkBusInboxRead: () => call('readAll'),
    };
  } });
  const { page: p, ok } = t;
  try {
    await t.context.exposeBinding('__store', (source, op, a, b) => {
      state.calls.push([op, a, b]);
      const unread = () => inbox().filter(m => !state.read.has(m.id) && !state.dismissed.has(m.id)).length;
      switch (op) {
        case 'overview': return { inbox: inbox().filter(m => !state.read.has(m.id) && !state.dismissed.has(m.id)).map(view), unread: unread(), rooms: [room], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 };
        case 'resolve': return { state: 'resolved', room };
        case 'page': {
          const list = inbox().filter(m => a === 'all' ? true : a === 'dismissed' ? state.dismissed.has(m.id) : !state.dismissed.has(m.id) && (a === 'unread' ? !state.read.has(m.id) : state.read.has(m.id)));
          const from = b ? list.filter(m => m.id < b) : list, items = from.slice(0, 3);
          return { items: items.map(view), unread: unread(), nextBefore: from.length > 3 ? items[items.length - 1].id : 0, newest: inbox()[0].id };
        }
        case 'message': return view(all.find(m => m.id === a));
        case 'read': state.read.add(a); return null;
        case 'dismiss': state.dismissed.add(a); return null;
        case 'readThrough': inbox().forEach(m => { if (m.id <= a) state.read.add(m.id); }); return null;
        case 'dismissThrough': inbox().forEach(m => { if (m.id <= a) state.dismissed.add(m.id); }); return null;
        case 'group': return all.filter(m => m.room).slice(-10);
        case 'groupFrom': { const g = all.filter(m => m.room), at = g.findIndex(m => m.id === a); return g.slice(Math.max(0, at - b)); }
        case 'thread': return [all.find(m => m.id === a)].concat(all.filter(m => m.replyTo === a));
        default: return null;
      }
    });
    const open = async () => { await p.goto(PAGE); await t.waitReady(); await p.evaluate(() => dinahWorkbench.open('bus')); };
    await open();
    const pane = p.locator('[data-section="bus-inbox"]');
    const rows = () => pane.locator('.bus-inbox-row');
    await rows().first().waitFor();
    const count = () => pane.locator('.browser-group-count, .md-section-count, .count').first().innerText().catch(() => '');
    const header = async () => (await pane.innerText()).split('\n').slice(0, 3).join(' ');
    let first = await rows().first().innerText();
    ok('by default the inbox shows unread messages, its count every unread one (7), beyond the page shown', (await rows().count()) === 3 && /\b7\b/.test(await header()), JSON.stringify({ header: await header(), n: await rows().count() }));
    ok('a row leads with the sending chat\'s name, then its message, then where and when', /^XferLang Reboot\n/.test(first) && /From xfer-1: Status 4 for you\./.test(first) && /To you · xferlang · /.test(first), first);
    await t.shot('inbox-names');
    const gone = rows().filter({ hasText: 'Status 3 for you' });
    const goneText = await gone.innerText(), goneTip = await gone.locator('.md-item').getAttribute('title');
    ok('a sender with no title left is said plainly, its id only in the tooltip', /^A chat that is no longer available\n/.test(goneText) && !/yavchn-1/.test(goneText) && /yavchn-1/.test(goneTip), JSON.stringify({ goneText, goneTip }));
    // An old group mention, beyond the group's newest messages.
    await pane.getByRole('button', { name: 'Load Older' }).click();
    await pane.getByRole('button', { name: 'Load Older' }).click();
    const old = rows().filter({ hasText: 'old schema question' });
    await old.waitFor();
    await old.locator('.md-item').click();
    const group = p.locator('.win[data-win-kind="bus-room"]');
    const target = group.locator('[data-bus-message="4"].bus-target-message');
    await target.waitFor();
    await p.waitForFunction(() => true);
    await p.waitForTimeout(300);
    let calls = state.calls.map(c => c[0]);
    ok('a group mention opens its group at that very message, highlighted, loaded from before it', await target.isVisible() && calls.includes('groupFrom') && /old schema question/.test(await target.innerText()));
    ok('only that message is marked read, and the group is not marked read', state.read.has(4) && state.read.size === 1 && !calls.includes('groupRead') && !calls.includes('readAll'), JSON.stringify([...state.read]));
    await p.evaluate(() => __emit('bus:changed', {}));
    await p.waitForTimeout(400);
    ok('an update leaves the target loaded and highlighted', await target.isVisible());
    await p.evaluate(() => dinahWorkbench.open('bus'));
    await p.waitForTimeout(300);
    ok('the opened row is no longer listed as unread, and the count is 6', (await rows().filter({ hasText: 'old schema question' }).count()) === 0 && /\b6\b/.test(await header()), await header());
    await t.shot('inbox-group-target');
    // A direct reply opens a thread that starts with it.
    await p.evaluate(() => dinahWorkbench.open('bus'));
    const replyRow = rows().filter({ hasText: 'all passed' });
    await replyRow.locator('.md-item').click();
    const thread = p.locator('.win[data-win-kind="bus-thread"]');
    await thread.locator('.bus-target-message').waitFor();
    const firstInThread = await thread.locator('[data-bus-message]').first().getAttribute('data-bus-message');
    await p.waitForTimeout(300);
    ok('a direct reply opens a thread that starts with it, highlighted', firstInThread === String(reply.id) && /all passed/.test(await thread.locator('.bus-target-message').innerText()) && state.read.has(reply.id), JSON.stringify({ firstInThread, read: [...state.read] }));
    await t.shot('inbox-thread-target');
    // Dismiss persists and does not mark read.
    await p.evaluate(() => dinahWorkbench.open('bus'));
    const status3 = rows().filter({ hasText: 'Status 3 for you' });
    await status3.locator('.bus-inbox-dismiss').click();
    await status3.waitFor({ state: 'detached' });
    ok('Dismiss removes the row without marking it read', state.dismissed.has(direct.id + 4) && !state.read.has(direct.id + 4), JSON.stringify({ d: [...state.dismissed] }));
    await open();
    await rows().first().waitFor();
    ok('a dismissed message stays dismissed after a reload', (await rows().filter({ hasText: 'Status 3 for you' }).count()) === 0 && /\b4\b/.test(await header()), await header());
    await pane.getByRole('button', { name: 'Dismissed', exact: true }).click();
    await rows().filter({ hasText: 'Status 3 for you' }).waitFor();
    ok('Dismissed lists it, marked dismissed', /Dismissed/.test(await rows().first().innerText()));
    await pane.getByRole('button', { name: 'All', exact: true }).click();
    await p.waitForTimeout(300);
    await pane.getByRole('button', { name: 'Load Older' }).click();
    await p.waitForTimeout(300);
    ok('All shows read and older messages, a page at a time', (await rows().count()) === 6, String(await rows().count()));
    // Mark All Read: bounded, confirmed, and the count reaches zero.
    await pane.getByRole('button', { name: 'Unread', exact: true }).click();
    await p.waitForTimeout(300);
    await pane.getByRole('button', { name: 'Mark All Read', exact: true }).click();
    const ask = p.locator('dialog[open]');
    await ask.waitFor();
    ok('Mark All Read says it does not answer or approve anything', /does not answer it or approve anything/.test(await ask.innerText()));
    await ask.getByRole('button', { name: 'Mark Read', exact: true }).click();
    await pane.getByText('No Unread Messages', { exact: true }).waitFor();
    calls = state.calls.filter(c => c[0] === 'readThrough');
    ok('it marks read through the newest inbox message it was read with, and the count is zero', calls.length === 1 && calls[0][1] === inbox()[0].id && /\b0\b/.test(await header()), JSON.stringify({ calls, header: await header() }));
    await t.shot('inbox-empty');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
