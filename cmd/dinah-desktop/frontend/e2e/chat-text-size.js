// The chat text size from Appearance (--chat-size) reaches the text people read and write in a
// chat and in a group chat: the messages, the text delivered into a chat, and the box that
// takes a message.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' };
      const room = { name: 'sized-group', title: 'Sized Group', lastAt: '', unread: 0, members: [docs, person], budget: -1, used: 0 };
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'] }),
        AgentProviders: async () => [], Sessions: async () => [], SessionsAt: async () => [],
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: [room], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1, noTurnBudget: -1 }),
        BusGroupMessages: async () => [{ id: 1, room: 'sized-group', from: 'Docs Pass', fromWorkspace: 'docs', to: '', text: 'A message to the group.\n\n# A heading', createdAt: '2026-10-29T06:00:00Z', kind: 'message' }],
        BusMarkGroupRead: async () => {},
        ResolveSessionContinuation: async id => id,
        SessionContinuation: async () => ({ history: [] }),
        SavedConversation: async () => ({ available: true, complete: true, messages: [{ role: 'you', text: 'A question in the chat.' }, { role: 'tool', text: 'Read file\nStatus: completed\nThe file text.' }, { role: 'agent', text: 'An answer in the chat.' }, { role: 'delivered', text: 'A reply from another chat.', provider: 'codex', delivered: { id: 7, kind: 'text', from: 'Docs Pass', fromSession: 'window-d1', workspace: 'docs', time: '2026-10-29T06:00:00Z' } }] }),
        // The chat reopens by resuming, so its saved copy stays as drawn and the page does not redraw it.
        OpenSessionAt: async (ws, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: ws, open: true, resumed: true, streamGen: 'r1', streamHead: 0 }),
        ChatEvents: async (id, gen) => ({ gen, head: 0, truncated: false, reset: false, events: [] })
      };
    },
  });
  const { page, ok } = t;
  // The element is drawn by the render queue, so it is waited for before its size is read.
  const size = async (win, selector) => { const el = win.locator(selector).first(); await el.waitFor({ state: 'attached' }); return el.evaluate(e => getComputedStyle(e).fontSize); };
  try {
    await page.evaluate(() => document.documentElement.style.setProperty('--chat-size', '21px'));
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Work/docs', path: 'codex:sized' }));
    const chat = page.locator('[data-win="' + key + '"]');
    await chat.locator('.msg-text', { hasText: 'An answer in the chat.' }).waitFor();
    ok('a chat\'s messages follow the chat text size', await size(chat, '.msg-text') === '21px', await size(chat, '.msg-text'));
    ok('and so does its box', await size(chat, '.composer-input') === '21px', await size(chat, '.composer-input'));
    await chat.locator('.delivered-card').first().waitFor();
    const ratio = async (selector, of) => (parseFloat(await size(chat, selector)) / of).toFixed(3);
    ok('the title of a delivered message\'s card keeps its ratio to the text',await ratio('.delivered-card .card-title', 21) === (13 / 15).toFixed(3), await size(chat, '.delivered-card .card-title'));
    ok('the text of that card is at the chat size', await size(chat, '.delivered-card .card-body') === '21px');
    await chat.locator('.tool-detail > summary').first().waitFor({ state: 'attached' });
    ok('a tool row\'s summary keeps its ratio to the text', await ratio('.tool-detail > summary', 21) === (12 / 15).toFixed(3), await size(chat, '.tool-detail > summary'));

    await page.evaluate(() => dinahWorkbench.open({ kind: 'bus-room', path: 'sized-group' }));
    const group = page.locator('.win[data-win-kind="bus-room"]');
    await group.locator('.msg-text', { hasText: 'A message to the group.' }).waitFor();
    ok('a group chat\'s messages follow the chat text size', await size(group, '.msg-text') === '21px', await size(group, '.msg-text'));
    ok('and so does its box', await size(group, '.composer-input') === '21px', await size(group, '.composer-input'));
    ok('a heading in a message stays larger than the text around it', parseFloat(await size(group, '.msg-text h1')) > 21);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
