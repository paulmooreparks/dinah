// A chat's window that starts while the engine is opening the chat by itself,
// as it does for a message waiting on the bus, joins that opening. When the
// opening ends the window is an open chat's: its loading mark is gone, its
// team membership is known and its message box is there. On 17 November 2026
// a team lead's window stayed at Loading, with "Checking team membership…"
// and no message box, after a restart: the window's start had marked the chat
// as restoring, and joining the engine's opening left that mark set for good.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    init: () => {
      let store;
      Object.defineProperty(window, 'dinahSessionStore', { get: () => store, set: value => { store = { ...value, create: adapter => { const result = value.create(adapter); window.__records = result.records; return result; } }; } });
    },
    app: () => ({
      AgentProviders: async () => [{ id: 'codex', installed: true, available: true }],
      ResolveSessionContinuation: async id => id,
      SessionContinuation: async () => ({ history: [] }),
      SavedConversation: async () => ({ available: true, streamGen: window.__finished ? 'second' : undefined, streamThrough: 1,
        messages: [{ role: 'you', text: 'The original question' }, { role: 'agent', text: 'The saved answer' }] }),
      OpenSessionAt: async (cwd, sessionId) => {
        window.__joined = true;
        await new Promise(resolve => { window.__finish = resolve; });
        window.__finished = true;
        return { sessionId, cwd, provider: 'codex', open: true, historyLoading: false, streamGen: 'second', streamHead: 1, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } };
      },
      ChatEvents: async () => ({ gen: 'second', head: 1, events: [] }),
      // The chat leads a team and reports to nobody.
      ChatLead: async () => { window.__leadReads = (window.__leadReads || 0) + 1; return null; },
      Team: async id => ({ lead: { chat: id }, group: 'lead-team', members: [{ bus: 'bus-a', chat: 'codex:member', title: 'Member', life: 'open', exec: { state: 'idle' } }], counts: { top: 'idle', idle: 1 } }),
    }),
  });
  try {
    const p = t.page, id = 'codex:lead';
    // The page knows the chat from an earlier opening. The engine then opens it
    // again by itself, and the first event of the new opening reaches a page
    // with no window for the chat, which marks the chat closed until its
    // window starts (app.js, the sequence's reset).
    await t.emit('agent:session', { sessionId: id, cwd: 'C:/Fixture', provider: 'codex', title: 'Lead', open: true, historyLoading: true, streamGen: 'first', streamHead: 0 });
    await p.waitForFunction(id => window.__records[id] && window.__records[id].info.historyLoading, id);
    await t.emit('agent:updates', { sessionId: id, gen: 'second', ns: [1], updates: [{ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'A replayed answer' } }] });
    await p.waitForFunction(id => window.__records[id].info.open === false, id, { timeout: 30000 }).catch(() => {});
    t.ok('the chat is marked closed while the engine is still opening it', await p.evaluate(id => window.__records[id].info.open === false && window.__records[id].info.historyLoading === true, id));
    await p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: id }), id);
    await p.waitForFunction(() => !!window.__joined, {}, { timeout: 30000 }).catch(() => {});
    t.ok('the window joins the opening in progress', await p.evaluate(() => !!window.__joined));
    const win = p.locator('.win[data-win-kind="chat-session"]');
    await p.waitForFunction(() => { const chip = document.querySelector('.win[data-win-kind="chat-session"] .chat-loading-chip'); return chip && !chip.hidden; }, {}, { timeout: 30000 }).catch(() => {});
    t.ok('and says it is loading meanwhile', await win.evaluate(w => { const chip = w.querySelector('.chat-loading-chip'); return !!chip && !chip.hidden; }), await win.evaluate(w => w.querySelector('.win-head').outerHTML.slice(0, 600)));
    await p.evaluate(() => window.__finish());
    const state = () => win.evaluate((w, id) => JSON.stringify({ chip: !w.querySelector('.chat-loading-chip').hidden, pending: !!w.querySelector('[data-team-pending]'), note: w.querySelector('.team-member-note').textContent, loading: !!window.__records[id].loading, restoring: !!window.__records[id].restoring }), id);
    const opened = await p.waitForFunction(() => {
      const w = document.querySelector('.win[data-win-kind="chat-session"]'), row = w.querySelector('.composer-row');
      return w.querySelector('.chat-loading-chip').hidden && !w.querySelector('[data-team-pending]') && row && row.getClientRects().length > 0;
    }, {}, { timeout: 30000 }).then(() => true, () => false);
    t.ok('when the opening ends the loading mark is gone, the team is known and the message box is there', opened, await state());
    t.ok('the chat is not left marked as restoring', await p.evaluate(id => !window.__records[id].restoring && !window.__records[id].loading, id), await state());
    t.ok('and it shows its history', await win.locator('.msg-text', { hasText: 'The saved answer' }).isVisible());
    // A change in the team makes the window read its membership again. With
    // the mark left on, that reading waited for an opening that had ended,
    // which is what took the message box away.
    const reads = await p.evaluate(() => window.__leadReads || 0);
    await t.emit('team:state', { lead: { chat: id }, group: 'lead-team', members: [{ bus: 'bus-a', chat: 'codex:member', title: 'Member', life: 'open', exec: { state: 'working' } }], counts: { top: 'working', working: 1 } });
    await p.waitForFunction(n => (window.__leadReads || 0) > n, reads);
    await p.evaluate(() => new Promise(r => setTimeout(r, 100)));
    t.ok('a later change in its team leaves the message box where it is', await win.evaluate(w => !w.querySelector('[data-team-pending]') && w.querySelector('.composer-row').getClientRects().length > 0 && !/Checking team membership/.test(w.textContent)), await state());
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
