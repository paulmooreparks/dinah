const { launch } = require('./lib/page');
const { choose } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      let workspace = 'C:/Alpha', n = 0;
      window.__calls = [];
      const state = () => ({ workspace, recent: ['C:/Alpha', 'C:/Beta'], devin: 'devin', approvals: 'devin', workspaceRecords: window.__wsRecords || [] });
      const info = (id, ws) => ({ sessionId: id, title: id, cwd: ws, open: true, working: false, queued: [] });
      return {
        State: async () => state(), SetWorkspace: async ws => { workspace = ws; return state(); },
        Sessions: async () => ['First', 'Second'].map(id => ({ ...info(id, 'C:/Alpha'), open: false })),
        OpenSessionAt: async (ws, id) => { window.__calls.push(['open', ws, id]); window.__emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'History of ' + id } } }); return info(id, ws); },
        NewSessionAt: async ws => { window.__calls.push(['new-session', ws]); return info('New ' + (++n), ws); },
        SelectWorkspace: async id => { window.__calls.push(['select-workspace', id]); const record = (window.__wsRecords || []).find(r => r.id === id); if (record) workspace = record.executionPath; return state(); },
        Send: async (id, text, doc, atts) => { window.__calls.push(['send', id, text, atts]); },
        Queue: async (id, text) => { window.__calls.push(['queue', id, text]); },
        RenameSession: async (id, name) => ({ [id]: name })
      };
    },
  });
  const { page: p, ok } = t;
  try {
    const front = () => p.locator('.win.active');
    const prompt = () => front().locator('[data-view-id="prompt"]');
    const open = async id => {
      await p.evaluate(() => window.dinahWorkbench.open('chat-browser'));
      await p.locator('#sessions a[data-session="' + id + '"]').click();
      await front().locator('.win-title').getByText(id, { exact: true }).waitFor();
    };
    await p.evaluate(() => dinahWorkbench.open('workspace'));
    await p.locator('#workspace-chats a[data-session="First"]').click();
    await front().locator('.win-title').getByText('First', { exact: true }).waitFor();
    ok('Workspace opens the owning chat directly', await front().getAttribute('data-win-kind') === 'chat-session');
    const firstKey = await front().getAttribute('data-win');
    await prompt().fill('First draft');
    await open('Second');
    const secondKey = await front().getAttribute('data-win');
    await prompt().fill('Second draft');
    await p.evaluate(key => window.pudlWindows.raise(key), firstKey);
    ok('each chat retains its own draft', await prompt().inputValue() === 'First draft');
    ok('each chat receives only its own replay', (await front().locator('[data-view-id="logs"]').textContent()).includes('History of First') && !(await front().locator('[data-view-id="logs"]').textContent()).includes('Second'));
    await p.evaluate(() => window.__emit('agent:update', { sessionId: 'Second', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: ' Second background update' } } }));
    await p.locator('[data-win="' + secondKey + '"] [data-view-id="logs"]').getByText(/Second background update/).waitFor();
    ok('background output stays in the owning chat', !(await front().locator('[data-view-id="logs"]').textContent()).includes('background'));
    await prompt().press('Control+Enter');
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'send'));
    ok('Send targets the owning session', await p.evaluate(() => window.__calls.some(c => c[0] === 'send' && c[1] === 'First' && c[2] === 'First draft')));
    await p.evaluate(key => window.pudlWindows.close(key), secondKey);
    await open('Second');
    ok('closing and reopening keeps an unsent draft', await prompt().inputValue() === 'Second draft');
    ok('reopening reuses the existing controller without replaying twice', await p.evaluate(() => window.__calls.filter(c => c[0] === 'open' && c[2] === 'Second').length) === 1);
    await p.evaluate(() => window.dinahWorkbench.open('chat'));
    await prompt().fill('Start here');
    await prompt().press('Control+Enter');
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'send' && c[1] === 'New 1'));
    ok('the first prompt turns the draft window into its session window', await front().getAttribute('data-win-kind') === 'chat-session' && await p.locator('.win[data-win-kind="chat-session"]').count() === 3);
    // The agent may report the session's folder spelled differently from the
    // window's (case, separators); the chat must still be the one window,
    // so what the agent says is drawn once.
    const newKey = await front().getAttribute('data-win');
    await p.evaluate(() => window.__emit('agent:session', { sessionId: 'New 1', title: 'New 1', cwd: 'c:\\alpha\\', open: true, working: true, queued: [] }));
    await p.evaluate(() => { for (const t of ['Hello', ' from', ' the agent.']) window.__emit('agent:update', { sessionId: 'New 1', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: t } } }); });
    await p.locator('[data-win="' + newKey + '"] [data-view-id="logs"]').getByText(/the agent\./).waitFor();
    const said = await p.locator('[data-win="' + newKey + '"] [data-view-id="logs"]').textContent();
    ok('a reply is drawn once when the agent spells the folder differently', said.includes('Hello from the agent.') && !said.includes('HelloHello'));
    await p.evaluate(() => window.dinahWorkbench.open('chat'));
    ok('a fresh draft can open beside the session made from the previous draft', await front().getAttribute('data-win-kind') === 'chat' && await prompt().inputValue() === '');
    // The agent's first updates can arrive before the new chat has its
    // session; they must still be drawn once, in the one window.
    await p.evaluate(() => {
      const made = window.go.main.App.NewSessionAt;
      window.go.main.App.NewSessionAt = async ws => {
        window.__emit('agent:update', { sessionId: 'Early', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Early bird.' } } });
        window.go.main.App.NewSessionAt = made;
        return { sessionId: 'Early', title: 'Early', cwd: 'c:\\alpha\\', open: true, working: false, queued: [] };
      };
    });
    await prompt().fill('Go early');
    await prompt().press('Control+Enter');
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'send' && c[1] === 'Early'));
    const earlyKey = await front().getAttribute('data-win');
    await p.evaluate(() => { for (const t of [' And', ' later.']) window.__emit('agent:update', { sessionId: 'Early', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: t } } }); });
    await p.locator('[data-win="' + earlyKey + '"] [data-view-id="logs"]').getByText(/later\./).waitFor();
    const early = await p.locator('[data-win="' + earlyKey + '"] [data-view-id="logs"]').textContent();
    ok('updates that arrive before the chat has its session are drawn once', early.split('Early bird.').length === 2 && early.includes('And later.') && !early.includes('And And') &&
      await p.locator('.chat-instance [data-view-id="logs"]').filter({ hasText: 'Early bird' }).count() === 1);
    // New Chat in the Dinah menu opens a fresh chat.
    await p.evaluate(() => document.querySelector('[data-command-menu="dinah"] [data-cmd="new-chat"]').click());
    await p.waitForFunction(() => document.querySelector('.win.active') && document.querySelector('.win.active').getAttribute('data-win-kind') === 'chat');
    ok('New Chat in the Dinah menu opens a fresh chat', await prompt().inputValue() === '');
    await open('Second');
    await open('Second');
    ok('choosing the current chat in its browser raises its window', await front().getAttribute('data-win') === secondKey);
    // @ offers the documents open in the Editor, by any word of the title.
    await p.evaluate(() => { window.dinahDocuments.open = () => [{ draftId: 'ab'.repeat(16), title: 'Note 09 - Config Reload vX.X (working version)', workspaceId: 'ws_1' }, { draftId: 'cd'.repeat(16), title: 'api-review-findings-2026-10-12.md', workspaceId: 'ws_1' }]; });
    await prompt().fill('');
    await prompt().pressSequentially('@api');
    const pop = front().locator('[data-view-id="slash-pop"]');
    await pop.getByText('api-review-findings-2026-10-12.md').waitFor();
    await prompt().fill('');
    await prompt().pressSequentially('@Note_09');
    await pop.getByText('Note 09 - Config Reload vX.X (working version)').waitFor();
    await prompt().press('Enter');
    ok('@ offers a document open in the Editor and puts its token in the message', (await prompt().inputValue()).startsWith('@Note_09_-_Config_Reload'));
    // Add › Browse the Workspace… opens the Open dialog, and what is chosen
    // there goes with the message. There is no separate + button.
    await p.evaluate(() => { window.dinahFilePicker.pick = o => { window.__pick = o; return Promise.resolve(o.commit({ node: { title: 'findings.md', ref: { workspaceId: 'ws_1', sourceId: 'src_1', path: 'review/findings.md' } } })); }; });
    ok('the composer has one Add button and no + button', await front().locator('[data-view-id="attach-button"]').count() === 0);
    await choose(p, front(), 'add', 'Browse the Workspace…');
    await front().locator('[data-view-id="attachments"]').getByText('findings.md').waitFor();
    ok('Browse the Workspace… opens the Open dialog for any file, and the choice is attached', await p.evaluate(() => window.__pick.mode === 'open' && window.__pick.any === true));
    await prompt().press('Control+Enter');
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'send' && c[1] === 'Second' && /@Note_09/.test(c[2])));
    ok('the open document and the attached file go with the message', await p.evaluate(() => {
      const c = window.__calls.filter(c => c[0] === 'send' && c[1] === 'Second').pop(), atts = c[3] || [];
      return atts.some(a => a.kind === 'draft' && a.draftId === 'ab'.repeat(16)) && atts.some(a => a.kind === 'resource' && a.ref.path === 'review/findings.md');
    }));
    // Renaming a chat renames its menu in the host bar.
    await p.keyboard.press('F2');
    await p.locator('.chat-rename-dialog input.session-rename').fill('Note 08 Research');
    await p.locator('.chat-rename-dialog input.session-rename').press('Enter');
    await p.waitForFunction(() => [...document.querySelectorAll('[data-menubar] button, [data-menubar] [role="menuitem"]')].some(b => b.textContent.trim() === 'Note 08 Research'));
    ok('renaming a chat renames its menu in the host bar', true);
    // New Chat asks which workspace it belongs to when more than one is
    // configured, defaulting to the one in front (chat-workspaces-and-switching
    // .md, New Chat's Workspace).
    // The choice's names are the records' own, read after the refresh their change brings.
    await p.evaluate(() => { window.__wsRecords = [{ id: 'alpha', name: 'Alpha', executionPath: 'C:/Alpha', sources: [{ id: 'local', name: 'Local', kind: 'local-folder', path: 'C:/Alpha' }] }, { id: 'beta', name: 'Beta Workspace', executionPath: 'C:/Beta', sources: [{ id: 'local', name: 'Local', kind: 'local-folder', path: 'C:/Beta' }] }]; __emit('settings:changed', {}); });
    await p.waitForFunction(() => /Beta Workspace/.test(document.body.textContent));
    await p.evaluate(() => document.querySelector('[data-command-menu="dinah"] [data-cmd="new-chat"]').click());
    const askDlg = p.locator('dialog.dinah-new-chat-workspace[open]');
    await askDlg.waitFor();
    ok('with two workspaces New Chat asks, with the one in front the default', (await askDlg.locator('#new-chat-workspace option').count()) === 2 && (await askDlg.locator('#new-chat-workspace').inputValue()) === '0' && /Alpha/.test(await askDlg.locator('#new-chat-workspace').textContent()) && /Beta Workspace/.test(await askDlg.locator('#new-chat-workspace').textContent()), await askDlg.locator('#new-chat-workspace').textContent());
    ok('and says what the choice decides', /agent works in its folder, and its skills, files and permissions are the workspace/.test(await askDlg.textContent()));
    await askDlg.getByRole('button', { name: 'Cancel' }).click();
    await askDlg.waitFor({ state: 'detached' });
    ok('cancel starts nothing and switches nothing', await p.evaluate(() => !window.__calls.some(c => c[0] === 'select-workspace')));
    // The default stays the workspace in front, which takes no call to switch.
    await p.evaluate(() => document.querySelector('[data-command-menu="dinah"] [data-cmd="new-chat"]').click());
    await askDlg.waitFor();
    await askDlg.getByRole('button', { name: 'New Chat' }).click();
    await askDlg.waitFor({ state: 'detached' });
    await p.locator('.win.active[data-win-kind="chat"] [data-view-id="prompt"]').waitFor();
    ok('keeping the default asks the engine for nothing and opens the chat', await p.evaluate(() => !window.__calls.some(c => c[0] === 'select-workspace')));
    // Choosing the other workspace leaves the pane where it is, and the chat
    // is made under the answer's folder.
    await p.evaluate(() => document.querySelector('[data-command-menu="dinah"] [data-cmd="new-chat"]').click());
    await askDlg.waitFor();
    await askDlg.locator('#new-chat-workspace').selectOption('1');
    await askDlg.getByRole('button', { name: 'New Chat' }).click();
    await askDlg.waitFor({ state: 'detached' });
    await p.waitForTimeout(150);
    ok('the pane is not switched for the new chat', (await p.locator('#status-workspace-name').textContent()).includes('Alpha') && await p.evaluate(() => !window.__calls.some(c => c[0] === 'select-workspace' && c[1] === 'beta')));
    // A new chat's title status retains its destination before the first message.
    await front().locator('.chat-working-chip').waitFor();
    ok('the new chat keeps its workspace in title status before the first message', /Beta Workspace/.test(await front().locator('.chat-working-chip').getAttribute('title')), await front().locator('.chat-working-chip').getAttribute('title'));
    ok('the new chat chip shows its destination without repeating the title', (await front().locator('.chat-working-chip .chat-lifecycle').innerText()) === 'Beta Workspace');
    await front().locator('[data-view-id="prompt"]').waitFor();
    await prompt().fill('hello from Beta');
    await prompt().press('Enter');
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'new-session' && c[1] === 'C:/Beta'));
    ok('the chat is made in the chosen workspace', await p.evaluate(() => window.__calls.some(c => c[0] === 'new-session' && c[1] === 'C:/Beta')));
    // One workspace configured asks nothing at all.
    await p.evaluate(() => { window.__wsRecords = [{ id: 'beta', name: 'Beta Workspace', executionPath: 'C:/Beta', sources: [{ id: 'local', name: 'Local', kind: 'local-folder', path: 'C:/Beta' }] }]; });
    await p.evaluate(() => document.querySelector('[data-command-menu="dinah"] [data-cmd="new-chat"]').click());
    await front().locator('[data-view-id="prompt"]').waitFor();
    ok('with one workspace configured nothing is asked', (await askDlg.count()) === 0);
    await t.shot('chat-windows');
    ok('all controls have unique IDs', await p.evaluate(() => { const ids = [...document.querySelectorAll('[id]')].map(el => el.id); return ids.length === new Set(ids).size; }));
  } finally { await t.close(); }
})();
