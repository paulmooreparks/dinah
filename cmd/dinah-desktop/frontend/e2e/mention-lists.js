// A chat's @ list and a group's @ list are one component (FE-17,
// mention-list.js): each composer decides what an @ offers, and the list
// they draw has the same markup and answers the same keys. The suite opens
// a chat in one page and a group in another, types @ in each, and compares
// the shape of the list and what Down, Up, Escape and Tab do.
const { launch } = require('./lib/page');

// The list's shape, without its words: each child's tag and classes, a
// row's role and selection, and the classes inside its link in order.
const shape = list => list.evaluate(el => [...el.children].map(c => {
  const s = { tag: c.tagName.toLowerCase(), cls: [...c.classList].sort().join(' ') };
  if (c.getAttribute('role')) { s.role = c.getAttribute('role'); s.selected = c.getAttribute('aria-selected'); s.inside = [...c.querySelector('a').children].map(n => n.className.split(' ')[0]).filter(n => n !== 'badge').join(' '); }
  return s;
}));
const rows = shape => shape.filter(s => s.role);
const summary = s => ({ heading: s.find(x => x.cls === 'md-section-label'), row: rows(s)[0] && Object.assign({}, rows(s)[0], { selected: undefined }), hint: s[s.length - 1] });

async function drive(page, box, list, ok, who) {
  const selected = () => list.evaluate(el => [...el.querySelectorAll('[role="option"]')].findIndex(r => r.getAttribute('aria-selected') === 'true'));
  await box.fill('');
  await box.pressSequentially('@');
  await list.locator('[role="option"]').nth(1).waitFor();
  const s = await shape(list), n = rows(s).length;
  const steps = [await selected()];
  await box.press('ArrowDown'); steps.push(await selected());
  await box.press('ArrowUp'); steps.push(await selected());
  await box.press('ArrowUp'); steps.push(await selected());
  ok(who + ': the list opens on its first row, and Down and Up move one row, Up from the first going to the last', steps.join() === [0, 1, 0, n - 1].join(), JSON.stringify({ steps, n }));
  ok(who + ': every row says whether it is selected, and only one is', rows(s).every(r => r.selected === 'true' || r.selected === 'false') && rows(s).filter(r => r.selected === 'true').length === 1, JSON.stringify(rows(s).map(r => r.selected)));
  await box.press('Escape');
  await list.waitFor({ state: 'hidden' });
  ok(who + ': Escape closes the list and leaves the text', await box.inputValue() === '@');
  await box.fill('');
  await box.pressSequentially('@');
  await list.locator('[role="option"]').first().waitFor();
  const first = await list.locator('[role="option"] .mention-name').first().textContent();
  await box.press('Tab');
  await list.waitFor({ state: 'hidden' });
  const value = await box.inputValue();
  ok(who + ': Tab puts the first row in the box', value.length > 2 && value.startsWith('@') && value.replace(/_/g, ' ').includes(first), JSON.stringify({ first, value }));
  return summary(s);
}

(async () => {
  const shapes = {};
  // A chat, whose @ offers windows, applets and other chats.
  {
    const t = await launch({
      viewport: { width: 1280, height: 900 },
      app: () => {
        const workspace = 'C:/Synthetic', info = id => ({ sessionId: id, title: id, cwd: workspace, open: true, working: false, queued: [] });
        return {
          State: async () => ({ workspace, recent: [workspace] }),
          AgentProviders: async () => [],
          Sessions: async () => ['First', 'Second'].map(id => ({ ...info(id), open: false })),
          OpenSessionAt: async (ws, id) => info(id),
          AppletInvocationsAt: async () => [],
          AppletCatalog: async () => [{ id: 'example.sketch', title: 'Sketch Pad', description: 'Draw and describe sketches.', actions: [{ ref: 'example.sketch/draw@1', title: 'Draw Sketch', description: 'Create an SVG.' }] }]
        };
      },
      ready: 'workbench',
    });
    const { page, ok } = t;
    try {
      await page.evaluate(() => dinahWorkbench.open('workspace'));
      await page.locator('#workspace-chats a[data-session="First"]').click();
      const front = page.locator('.win.active[data-win-kind="chat-session"]');
      await front.locator('.win-title').getByText('First', { exact: true }).waitFor();
      shapes.chat = await drive(page, front.locator('[data-view-id="prompt"]'), front.locator('[data-view-id="slash-pop"]'), ok, 'chat');
    } finally { await t.close(); }
  }
  // A group, whose @ offers its members, everyone and Add a Chat.
  {
    const t = await launch({
      viewport: { width: 1280, height: 900 },
      app: () => {
        const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } };
        const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
        return {
          State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'] }),
          ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
          BusOverview: async () => ({ inbox: [], unread: 0, rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [docs, person], counts: { top: 'idle', idle: 1, working: 0, waiting: 0, stuck: 0, closed: 0, unknown: 0 } }], workspaces: [{ name: 'docs', label: 'docs', path: 'C:/Work/docs', chats: [{ id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass' }] }], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
          BusGroupMessages: async () => [], BusMarkGroupRead: async () => {}
        };
      },
    });
    const { page: p, ok } = t;
    try {
      await p.locator('[data-win="workspace"]').waitFor({ state: 'attached' });
      await p.evaluate(() => dinahBus.openGroup('release-review'));
      const room = p.locator('.win[data-win-kind="bus-room"]');
      const box = room.locator('.group-composer textarea');
      await box.waitFor();
      shapes.group = await drive(p, box, room.locator('.group-mention-pop'), ok, 'group');
      // The two lists are drawn alike: the same heading, the same row with
      // its title before its details, and the same line of keys at the end.
      ok('a chat\'s and a group\'s @ lists have the same markup', JSON.stringify(shapes.chat) === JSON.stringify(shapes.group), JSON.stringify(shapes));
    } finally { await t.close(); }
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
