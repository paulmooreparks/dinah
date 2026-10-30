// Creating a chat is one picker step; adding it is a later membership commit.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    window.__creationCalls = []; window.__failCreation = false; window.__failMembership = false;
    const records = [{ id: 'alpha', name: 'Alpha', executionPath: 'C:/Alpha', primarySourceId: 'main', sources: [{ id: 'main', kind: 'local-folder', path: 'C:/Alpha', mountAlias: 'Alpha' }] }, { id: 'beta', name: 'Beta', executionPath: 'C:/Beta', primarySourceId: 'main', sources: [{ id: 'main', kind: 'local-folder', path: 'C:/Beta', mountAlias: 'Beta' }] }];
    const chats = [{ sessionId: 'lead', title: 'Lead', provider: 'codex', cwd: 'C:/Alpha', open: false }, { sessionId: 'existing', title: 'Existing Chat', provider: 'codex', cwd: 'C:/Alpha', open: false }];
    const group = { name: 'team-room', title: 'Development Team', lead: 'bus-lead', members: [{ id: 'bus-lead', chat: 'lead', title: 'Lead', state: 'open' }] };
    window.__createdCount = 0; window.__members = [];
    return {
      State: async () => ({ workspaceId: 'alpha', workspace: 'C:/Alpha', workspaceRecords: records, workspaces: ['C:/Alpha', 'C:/Beta'], recent: ['C:/Alpha'], defaultProvider: 'codex' }),
      Sessions: async () => chats, SessionsAt: async ws => chats.filter(c => c.cwd === ws),
      AgentProviders: async () => [__provider('codex'), __provider('ollama')],
      ProviderModelPreferences: async id => ({ catalog: [{ id: 'model', category: 'model', options: [{ value: id + '-a', name: id + ' Model A' }, { value: id + '-b', name: id + ' Model B' }] }], defaults: { model: id + '-a' } }),
      BusOverview: async () => ({ inbox: [], unread: 0, rooms: [group], workspaces: [], links: [] }),
      BusGroupMessages: async () => [], BusMarkGroupRead: async () => {},
      CreateNamedChat: async (workspace, provider, model, title) => {
        __creationCalls.push(['create', workspace, provider, model, title]);
        if (__failCreation) throw Error('The provider could not create the chat.');
        const info = { sessionId: 'created-' + (++__createdCount), cwd: workspace, provider, title, open: true };
        chats.push(info); return info;
      },
      BusAddToGroup: async (key, refs) => {
        __creationCalls.push(['add', key, refs]);
        if (__failMembership) throw Error('The team could not be changed.');
        __members.push(...refs.map(c => ({ id: c.id, lead: 'lead' })));
      }
    };
  } });
  const { page: p, ok } = t;
  try {
    await p.waitForFunction(() => dinahChats.known().some(c => c.id === 'existing'));
    const paneBefore = await p.locator('#ws-select').inputValue();
    const key = await p.evaluate(() => dinahWorkbench.open({ kind: 'group-picker', title: 'Add Chats to Development Team', path: JSON.stringify({ group: 'team-room' }) }));
    const picker = p.locator('[data-win="' + key + '"]'), list = picker.locator('.picker-list-pane'), form = picker.locator('.picker-new-chat');
    await picker.locator('[data-chat="existing"]').click();
    await picker.getByRole('searchbox').fill('No matching title');
    await picker.getByRole('button', { name: 'New Chat', exact: false }).click();
    ok('New Chat opens its form inside the picker', await form.count() === 1);
    await form.getByRole('textbox', { name: /^Title/ }).fill('Not created');
    await form.getByRole('textbox', { name: /^Title/ }).press('Escape');
    ok('Escape returns to the same filtered selections and creates nothing', await list.isVisible() && await picker.getByRole('searchbox').inputValue() === 'No matching title' && await picker.locator('[data-chat="existing"]').getAttribute('aria-selected') === 'true' && await p.evaluate(() => __creationCalls.length === 0));
    await picker.getByRole('button', { name: 'New Chat', exact: false }).click();
    await form.getByRole('button', { name: 'Cancel', exact: true }).click();
    ok('Cancel returns to the list without creating a chat', await p.evaluate(() => __createdCount === 0));
    await picker.getByRole('button', { name: 'New Chat', exact: false }).click();
    await form.getByLabel('Workspace', { exact: true }).selectOption('C:/Beta');
    await form.getByLabel('Provider', { exact: true }).selectOption('ollama');
    await form.getByLabel('Model', { exact: true }).selectOption('ollama-b');
    await form.getByRole('textbox', { name: /^Title/ }).fill('Fresh Chat');
    await t.shot('group-new-chat-form');
    await form.getByRole('textbox', { name: /^Title/ }).fill('');
    await form.getByRole('button', { name: 'Create Chat', exact: true }).click();
    ok('a missing title stays in the step and makes nothing', await form.locator('.notice').isVisible() && await p.evaluate(() => __createdCount === 0));
    await form.getByRole('textbox', { name: /^Title/ }).fill('Fresh Chat');
    await p.evaluate(() => { __failCreation = true; });
    await form.getByRole('button', { name: 'Create Chat', exact: true }).click();
    await form.getByText('The provider could not create the chat.', { exact: true }).waitFor();
    ok('a creation failure keeps every choice in the same picker', await form.getByRole('textbox', { name: /^Title/ }).inputValue() === 'Fresh Chat' && await form.getByLabel('Workspace', { exact: true }).inputValue() === 'C:/Beta' && await form.getByLabel('Model', { exact: true }).inputValue() === 'ollama-b');
    await p.evaluate(() => { __failCreation = false; });
    await form.getByRole('button', { name: 'Create Chat', exact: true }).click();
    const row = picker.locator('[data-chat="created-1"]');
    await row.waitFor();
    ok('the new chat returns ticked with the previous selection preserved', await row.getAttribute('aria-selected') === 'true' && await picker.locator('[data-chat="existing"]').getAttribute('aria-selected') === 'true');
    const returned = await picker.evaluate(n => ({ focused: document.activeElement.dataset.chat === 'created-1' && n.contains(document.activeElement), filter: n.querySelector('.picker-filter').value, note: n.querySelector('.picker-count').textContent }));
    ok('the new row is focused and an incompatible filter is explained', returned.focused && returned.filter === '' && /filter was cleared/.test(returned.note), JSON.stringify(returned));
    ok('creation uses the chosen workspace, provider, model and title without adding membership', await p.evaluate(() => __creationCalls.some(c => c[0] === 'create' && c.slice(1).join('|') === 'C:/Beta|ollama|ollama-b|Fresh Chat') && !__creationCalls.some(c => c[0] === 'add') && __members.length === 0));
    ok('creation neither switches the pane nor opens another picker or dialog', await p.locator('#ws-select').inputValue() === paneBefore && await p.locator('.win[data-win-kind="group-picker"]').count() === 1 && await p.locator('dialog[open]').count() === 0);
    await t.shot('group-new-chat-selected');
    await p.evaluate(() => { __failMembership = true; });
    await picker.getByRole('button', { name: 'Add', exact: true }).click();
    await p.waitForFunction(() => __creationCalls.filter(c => c[0] === 'add').length === 1);
    await picker.getByRole('button', { name: 'Add', exact: true }).waitFor({ state: 'visible' });
    ok('a failed Add keeps the new chat selected', await row.getAttribute('aria-selected') === 'true');
    await p.evaluate(() => { __failMembership = false; });
    await picker.getByRole('button', { name: 'Add', exact: true }).click();
    await p.waitForFunction(() => __members.length === 2);
    ok('Add retries membership with the same created chat and the other selection', await p.evaluate(() => __createdCount === 1 && __members.some(m => m.id === 'created-1' && m.lead === 'lead') && __members.some(m => m.id === 'existing')));
    await p.evaluate(() => dinahWorkbench.open({ kind: 'group-picker', title: 'Add Chats to Development Team', path: JSON.stringify({ group: 'team-room' }) }));
    await picker.getByRole('button', { name: 'New Chat', exact: false }).click();
    await form.getByRole('textbox', { name: /^Title/ }).fill('Kept Ordinary');
    await form.getByRole('button', { name: 'Create Chat', exact: true }).click();
    await picker.locator('[data-chat="created-2"]').waitFor();
    await picker.getByRole('button', { name: 'Cancel', exact: true }).click();
    ok('cancelling Add Chats keeps the created chat ordinary', await p.evaluate(() => __createdCount === 2 && !__members.some(m => m.id === 'created-2') && dinahChats.known().some(c => c.id === 'created-2')));

  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
