// Recovery cards for chats that could not be opened, on a page that has
// never loaded them. Two chats with the same title are told apart by their
// provider and start. Remove Chat removes exactly the card's chat from this
// app, cancelling its waiting messages, with no New Chat or other chat in
// its place; a failed removal says so and changes nothing. Dismiss hides a
// card, which stays hidden after a reload. Ordinary Remove opens no New
// Chat either. Every button is usable again once its action ends.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const store = { removed: [], dismissed: [], failRemove: 1, deleted: [] };
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    state: { workspace: 'C:/Fixture', recent: ['C:/Fixture'] },
    goto: false,
    app: () => {
      const card = (id, lead, provider, started, pending) => ({ id, kind: 'delivery-failure', severity: 'error', title: 'Messages are waiting for Team Lead',
        body: pending + ' messages are waiting. Recover the chat, or remove it.', details: ['Original recipient ' + lead], sessionId: lead, sessionTitle: 'Team Lead',
        originTitle: 'Team Lead', originProvider: provider, originStarted: started, workspace: 'C:/Fixture', provider, lead, retryAllowed: false, created: '2026-11-01T10:00:00Z' });
      const all = [card('waiting_a', 'codex:a', 'codex', '2026-10-30T09:15:00Z', 9), card('waiting_b', 'claude:b', 'claude', '2026-10-31T14:00:00Z', 3)];
      const info = id => ({ sessionId: id, title: 'Ordinary', provider: 'codex', cwd: 'C:/Fixture', open: true, working: false });
      return {
        Notifications: async () => { const s = await __store('read'); return all.filter(n => !s.removed.includes(n.lead) && !s.dismissed.includes(n.id)); },
        DismissNotification: async id => { await __store('dismiss', id); },
        ClearNotifications: async () => { await __store('clear'); },
        RemoveChat: async id => { await __store('remove', id); },
        DeleteSession: async id => { await __store('delete', id); },
        OpenSessionAt: async () => info('codex:ordinary'),
        Sessions: async () => [info('codex:ordinary')], SessionsAt: async () => [info('codex:ordinary')],
        SavedConversation: async () => ({ available: true, complete: true, messages: [{ role: 'agent', text: 'Saved history.' }] }),
        SessionContinuation: async () => ({ history: [] }),
        AgentProviders: async () => [{ id: 'claude', name: 'Claude', installed: true }, { id: 'codex', name: 'Codex', installed: true }],
      };
    },
  });
  const { page: p, ok } = t;
  try {
    await t.context.exposeBinding('__store', (source, op, id) => {
      if (op === 'dismiss') store.dismissed.push(id);
      if (op === 'clear') store.dismissed.push('waiting_a', 'waiting_b');
      if (op === 'delete') store.deleted.push(id);
      if (op === 'remove') { if (store.failRemove) { store.failRemove--; throw new Error('The removal could not be saved.'); } store.removed.push(id); }
      return structuredClone(store);
    });
    const open = async () => { await p.goto(PAGE); await t.waitReady(); await p.evaluate(() => dinahWorkbench.open('needs-you')); };
    await open();
    const a = p.locator('[data-notification-id="waiting_a"]'), b = p.locator('[data-notification-id="waiting_b"]');
    await a.waitFor(); await b.waitFor();
    ok('both cards offer Recover, Remove Chat and Dismiss, and Clear All is available',
      await a.getByRole('button', { name: 'Remove Chat…', exact: true }).count() === 1 && await a.getByRole('button', { name: 'Dismiss', exact: true }).count() === 1 &&
      await a.getByRole('button', { name: 'Recover Original Chat', exact: true }).count() === 1 && await p.locator('#needs-clear').isEnabled());
    const windows = () => p.evaluate(() => pudlWindows.state().open.slice().sort());
    const before = await windows();
    // A removal that fails changes nothing and can be tried again.
    await a.getByRole('button', { name: 'Remove Chat…', exact: true }).click();
    const dialog = p.locator('#remove');
    await dialog.waitFor();
    ok('Remove asks first, saying the waiting messages are cancelled', /cancelled and never delivered/.test(await dialog.textContent()) && /Team Lead/.test(await dialog.textContent()));
    await dialog.getByRole('button', { name: 'Remove', exact: true }).click();
    await dialog.getByText('The Chat Was Not Removed', { exact: true }).waitFor();
    ok('a failed removal says so, changes nothing, and Remove can be pressed again', await a.count() === 1 && await dialog.getByRole('button', { name: 'Remove', exact: true }).isEnabled() && store.removed.length === 0);
    await dialog.getByRole('button', { name: 'Remove', exact: true }).click();
    await a.waitFor({ state: 'detached' });
    const after = await windows();
    ok('Remove removes exactly the card\'s chat, with no window opened in its place', JSON.stringify(store.removed) === '["codex:a"]' && store.deleted.length === 0 && JSON.stringify(after) === JSON.stringify(before), JSON.stringify({ store, before, after }));
    ok('the other chat with the same title keeps its card', await b.count() === 1);
    // Dismiss hides the card, through a reload too.
    await b.getByRole('button', { name: 'Dismiss', exact: true }).click();
    await b.waitFor({ state: 'detached' });
    await open();
    await p.waitForTimeout(400);
    ok('a dismissed card stays hidden after a reload, and the removed one stays gone', await b.count() === 0 && await a.count() === 0 && JSON.stringify(store.dismissed) === '["waiting_b"]');
    // Ordinary Remove opens no New Chat.
    const beforeOrdinary = await windows();
    const key = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:ordinary' }));
    await p.locator('[data-win="' + key + '"] [data-view-id="prompt"]').waitFor();
    await p.evaluate(() => dinahCommands.run('remove-session'));
    await dialog.waitFor();
    await dialog.getByRole('button', { name: 'Remove', exact: true }).click();
    await p.locator('[data-win="' + key + '"]').waitFor({ state: 'detached' });
    await p.waitForTimeout(300);
    const afterOrdinary = await windows();
    ok('ordinary Remove closes the chat\'s window and opens no New Chat or other window', JSON.stringify(afterOrdinary) === JSON.stringify(beforeOrdinary) && JSON.stringify(store.deleted) === '["codex:ordinary"]', JSON.stringify({ beforeOrdinary, afterOrdinary }));
    await t.shot('recovery-card');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
