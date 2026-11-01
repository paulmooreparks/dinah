// The Members dropdown, shared by a team's group chat and its lead's chat
// (docs/proposals/team-members-approved.md). Each row is a member's name
// and standard state chip and opens its chat. The group's rows have a …
// menu with Open Chat, Address Message and the group's own commands;
// Address Message mentions the chat at the composer's caret, bound to it.
// The lead's rows have no … button. Members toggles and Escape closes; a
// live state change leaves the open dropdown, its focus and the caret. The
// toolbars fold rather than wrap, and their labels come back when wide.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const cwd = 'C:/Fixture';
    const ref = (id, title, state) => ({ id: 'bus-' + id, chat: id, title, life: 'open', window: true, exec: { state: state || 'idle', why: '' } });
    const room = { name: 'team-a', title: 'Team A', lead: 'bus-lead', members: [ref('lead', 'Team Lead'), ref('alpha', 'Alpha', 'working'), ref('beta', 'Beta'), { id: 'person', state: 'person', title: 'Paul' }] };
    let ver = 1, alpha = 'working';
    window.__sent = [];
    const team = () => ({ lead: { chat: 'lead', bus: 'bus-lead', title: 'Team Lead' }, group: 'team-a', stateGen: 'g', stateVer: ++ver, counts: { top: alpha, [alpha]: 1, idle: 1 },
      members: [{ chat: 'alpha', bus: 'bus-alpha', title: 'Alpha', life: 'open', window: true, exec: { state: alpha, why: '' } }, { chat: 'beta', bus: 'bus-beta', title: 'Beta', life: 'open', window: true, exec: { state: 'idle', why: '' } }], budget: { rows: [] } });
    window.__alpha = state => { alpha = state; room.members[1] = ref('alpha', 'Alpha', state); __emit('team:state', team()); __emit('bus:changed', {}); };
    const info = id => ({ sessionId: id, title: id === 'lead' ? 'Team Lead' : id, cwd, provider: 'codex', open: true, working: false });
    return { State: async () => ({ workspace: cwd, recent: [cwd] }), Sessions: async () => [info('lead')], SessionsAt: async () => [info('lead')],
      ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
      OpenSessionAt: async (w, id) => info(id), ChatLead: async () => null, Team: async id => id === 'lead' ? team() : { lead: { chat: id }, group: '', members: [], stateGen: 'g', stateVer: ++ver, budget: { rows: [] } },
      BusOverview: async () => ({ inbox: [], rooms: [room], workspaces: [], links: [] }), BusResolveGroup: async () => ({ state: 'resolved', room }),
      BusGroupMessages: async () => [], BusMarkGroupRead: async () => {}, BusSendToGroup: async (key, text, mentions) => { __sent.push({ text, mentions }); } };
  } });
  const { page: p, ok } = t;
  const rowsOf = loc => loc.evaluate(panel => [...panel.querySelectorAll('.member-row')].map(r => ({ name: r.querySelector('.member-name').textContent, chip: (r.querySelector('.badge.chat-state:not(.chat-state-reserve), .member-you') || {}).textContent || '', more: !!r.querySelector('.member-menu-btn') })));
  try {
    // The team's group chat.
    await p.evaluate(() => dinahBus.openGroup('team-a', { message: 0 }));
    const group = p.locator('.win[data-win-kind="bus-room"]'), members = group.getByRole('button', { name: /^Members/ });
    await members.waitFor();
    const order = await group.evaluate(g => [...g.querySelectorAll('.group-toolbar button')].filter(b => b.getClientRects().length && !b.closest('[popover]')).map(b => b.textContent.trim()));
    ok('group: Last Read first, then Members, then the team commands', /Last Read/.test(order[0]) && /^Members \(4\)/.test(order[1]) && order.some(x => /Team Report/.test(x)) && order.some(x => /Team Allowances/.test(x)), JSON.stringify(order));
    const draft = group.locator('textarea');
    await draft.fill('Please check this today');
    await draft.evaluate(t => t.setSelectionRange(13, 18));
    await members.click();
    const panel = group.locator('.members-menu:popover-open');
    await panel.waitFor();
    let rows = await rowsOf(panel);
    ok('group: every participant, name and standard chip, with a … button each chat has', rows.length === 4 && rows[1].name === 'Alpha' && rows[1].chip === 'Working' && rows[0].more && rows[3].name === 'Paul' && rows[3].chip === 'You' && !rows[3].more, JSON.stringify(rows));
    ok('group: the dropdown has no heading and no close button', await panel.locator('h2, h3, .popover-close, [aria-label="Close"]').count() === 0);
    await t.shot('members-group-open');
    // A live state change while a row has focus.
    const alphaOpen = panel.locator('[data-group-member="bus-alpha"] .member-open');
    await alphaOpen.focus();
    await alphaOpen.evaluate(b => { b.__mark = 1; });
    await p.evaluate(() => __alpha('waiting'));
    await panel.locator('[data-group-member="bus-alpha"] .badge.chat-state:not(.chat-state-reserve)').filter({ hasText: 'Waiting' }).waitFor();
    ok('group: a state change keeps the dropdown open and focus on the same row', await panel.count() === 1 && await p.evaluate(() => document.activeElement.__mark === 1));
    // Address Message at the saved caret.
    await panel.locator('[data-group-member="bus-alpha"] .member-menu-btn').click();
    const menu = group.locator('.group-member-menu:popover-open');
    await menu.waitFor();
    const items = await menu.locator('.menu-action').allTextContents();
    ok('group: the … menu has Open Chat, Address Message and the group command', JSON.stringify(items) === '["Open Chat","Address Message","Remove From Group…"]', JSON.stringify(items));
    ok('group: the dropdown stays open under the … menu', await panel.count() === 1);
    await menu.getByRole('menuitem', { name: 'Address Message' }).click();
    const after = await draft.evaluate(t => ({ value: t.value, caret: t.selectionStart, focused: document.activeElement === t }));
    ok('group: Address Message replaces the selection at the caret, keeps the draft, and focuses after the mention', after.value === 'Please check @Alpha today' && after.caret === 'Please check @Alpha '.length && after.focused, JSON.stringify(after));
    ok('group: the dropdown and menu are closed after addressing', await group.locator('.members-menu:popover-open, .group-member-menu:popover-open').count() === 0);
    await draft.press('Control+Enter').catch(() => {});
    await group.getByRole('button', { name: 'Send', exact: true }).click().catch(() => {});
    await p.waitForFunction(() => __sent.length > 0);
    const sent = await p.evaluate(() => __sent[0]);
    ok('group: the sent message is bound to Alpha by its id', JSON.stringify(sent.mentions || []).includes('bus-alpha'), JSON.stringify(sent));
    // Toggle and Escape.
    await members.click(); await panel.waitFor();
    await members.click(); await p.waitForTimeout(150);
    ok('group: Members toggles the dropdown closed', await panel.count() === 0);
    await members.click(); await panel.waitFor();
    await p.keyboard.press('Escape'); await p.waitForTimeout(150);
    ok('group: Escape closes it', await panel.count() === 0);
    // A row opens its member's chat.
    await members.click(); await panel.waitFor();
    await panel.locator('[data-group-member="bus-beta"] .member-open').click();
    await p.locator('.win[data-win-kind="chat-session"]').filter({ has: p.locator('.win-title', { hasText: /^beta$/ }) }).waitFor();
    ok('group: a row opens its member\'s chat', true);
    await t.shot('members-group');
    // The lead's chat.
    const key = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'lead' }));
    const win = p.locator('[data-win="' + key + '"]'), leadMembers = win.locator('.chat-team-toolbar').getByRole('button', { name: /^Members/ });
    await leadMembers.waitFor();
    const leadOrder = await win.evaluate(w => [...w.querySelectorAll('.chat-team-toolbar button')].filter(b => b.getClientRects().length && !b.closest('[popover]')).map(b => b.textContent.trim()));
    ok('lead: Last Read first, then Members, then Open Team', /Last Read/.test(leadOrder[0]) && /^Members \(2\)/.test(leadOrder[1]) && leadOrder[2] === 'Open Team', JSON.stringify(leadOrder));
    await leadMembers.click();
    const leadPanel = win.locator('.members-menu:popover-open');
    await leadPanel.waitFor();
    rows = await rowsOf(leadPanel);
    ok('lead: its direct reports, name and standard chip, with no … button', rows.length === 2 && rows.every(r => !r.more) && rows[0].name === 'Alpha' && rows[0].chip === 'Waiting', JSON.stringify(rows));
    await leadPanel.locator('.member-open').first().focus();
    await p.evaluate(() => { document.activeElement.__mark = 1; __alpha('working'); });
    await leadPanel.locator('.badge.chat-state:not(.chat-state-reserve)').filter({ hasText: 'Working' }).first().waitFor();
    ok('lead: a state change keeps the dropdown open and focus on the same row', await leadPanel.count() === 1 && await p.evaluate(() => document.activeElement.__mark === 1));
    await t.shot('members-lead');
    await p.keyboard.press('Escape'); await p.waitForTimeout(150);
    ok('lead: Escape closes it', await leadPanel.count() === 0);
    await leadMembers.click(); await leadPanel.waitFor();
    await leadPanel.locator('.member-open').nth(1).click();
    await p.locator('.win[data-win-kind="chat-session"]').filter({ has: p.locator('.win-title', { hasText: /^beta$/ }) }).waitFor();
    ok('lead: a row opens its member\'s chat', await leadPanel.count() === 0);
    // Folding: narrow, then wide again.
    const gkey = await group.getAttribute('data-win');
    const words = () => group.evaluate(g => ({ shown: [...g.querySelectorAll('.group-toolbar .toolbar-words')].filter(w => w.getClientRects().length && !w.closest('[popover]')).map(w => w.textContent), height: g.querySelector('.group-toolbar').getBoundingClientRect().height }));
    await p.evaluate(k => { pudlWindows.raise(k); pudlWindows.snap(k, 'maximized'); }, gkey);
    await p.waitForTimeout(400);
    const wide = await words();
    await t.shot('members-group-wide');
    await p.setViewportSize({ width: 520, height: 1000 });
    await p.waitForTimeout(500);
    const narrow = await words();
    await t.shot('members-group-narrow');
    await p.setViewportSize({ width: 1440, height: 1000 });
    await p.waitForTimeout(500);
    const wideAgain = await words();
    ok('the toolbar folds to one row when narrow, Members keeping its label longest', narrow.height <= wide.height + 2 && narrow.shown.length < wide.shown.length, JSON.stringify({ wide, narrow }));
    ok('labels come back when the window widens', JSON.stringify(wideAgain.shown) === JSON.stringify(wide.shown), JSON.stringify({ wide, wideAgain }));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
