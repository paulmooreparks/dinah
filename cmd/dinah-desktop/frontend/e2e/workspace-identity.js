// Workspace identity: names, stable-ID selection, duplicate names told
// apart, Unix case and backslash paths, rename, Remove Workspace…, the
// frozen Pinned group and reload against a backend fixture that outlasts
// the page.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const records = [
    { id: 'ws-a', name: 'Parser', sources: [{ id: 'root-a', kind: 'local-folder', path: 'C:/Projects/Alpha' }] },
    { id: 'ws-b', name: 'Parser', sources: [{ id: 'root-b', kind: 'local-folder', path: 'D:/Projects/Beta' }] },
    { id: 'ws-missing', name: 'Offline Work', sources: [{ id: 'root-missing', kind: 'local-folder', path: 'Z:/Offline' }] },
    { id: 'ws-unix-upper', name: 'Upper Case Folder', sources: [{ id: 'root-unix-upper', kind: 'local-folder', path: '/home/test/Work' }], pins: [{ kind: 'chat', title: 'Pinned Chat', session: 'chat-b' }] },
    { id: 'ws-unix-lower', name: 'Lower Case Folder', sources: [{ id: 'root-unix-lower', kind: 'local-folder', path: '/home/test/work' }] },
    { id: 'ws-unix-backslash', name: 'Backslash Folder', sources: [{ id: 'root-unix-backslash', kind: 'local-folder', path: '/tmp/a\\b' }] },
    { id: 'ws-unix-slash', name: 'Nested Folder', sources: [{ id: 'root-unix-slash', kind: 'local-folder', path: '/tmp/a/b' }] }
  ];
  let selected = 'ws-a';
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => {
      window.__calls = [];
      const root = record => record.sources[0].path;
      const state = async () => {
        const stored = await window.__workspaceStore('read');
        const record = stored.records.find(record => record.id === stored.selected);
        return { workspace: root(record), workspaceId: record.id, workspaceRecords: stored.records, recent: stored.records.map(root), workspaces: stored.records.slice(0, 2).map(root), defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [] };
      };
      const sessions = ws => [{ sessionId: ws.includes('Alpha') ? 'chat-a' : 'chat-b', provider: 'codex', cwd: ws, title: ws.includes('Alpha') ? 'Alpha Chat' : 'Beta Chat', open: false }];
      return {
        State: state,
        RenameWorkspace: async (id, name) => {
          window.__calls.push(['rename', id, name]);
          if (window.__renameFail) throw new Error('The workspace name could not be saved.');
          if (window.__holdRename) await new Promise(resolve => { window.__finishRename = resolve; });
          const stored = await window.__workspaceStore('rename', id, name);
          __emit('settings:changed', {});
          return stored.records.find(record => record.id === id);
        },
        SelectWorkspace: async id => {
          window.__calls.push(['select', id]);
          if (id === 'ws-missing') throw new Error('The workspace folder is unavailable: Z:/Offline');
          await window.__workspaceStore('select', id); return state();
        },
        SetWorkspace: async () => { throw new Error('Workspace selection must use its ID'); },
        RemoveWorkspace: async id => { window.__calls.push(['remove', id]); await window.__workspaceStore('remove', id); return state(); },
        ForgetWorkspace: async () => { throw new Error('Unavailable workspaces must be retained'); },
        Sessions: async () => sessions((await state()).workspace), SessionsAt: async ws => sessions(ws),
        OpenSessionAt: async (ws, id) => { window.__calls.push(['open-chat', ws, id]); return { sessionId: id, provider: 'codex', cwd: ws, title: 'Alpha Chat', open: true }; },
        Send: async (...args) => { window.__calls.push(['send', ...args]); },
      };
    },
    goto: false,
  });
  const { page, ok } = t;
  try {
    // The store outlasts the page, so a reload reads what the last page saved.
    await t.context.exposeBinding('__workspaceStore', (source, operation, id, name) => {
      if (operation === 'rename') records.find(record => record.id === id).name = name;
      if (operation === 'select') selected = id;
      if (operation === 'remove') { records.splice(records.findIndex(record => record.id === id), 1); selected = 'ws-unix-upper'; }
      return { selected, records: structuredClone(records) };
    });
    await page.goto(PAGE);
    await page.locator('#status-workspace-name').getByText('Parser (C:/Projects/Alpha)', { exact: true }).waitFor();
    ok('duplicate workspace names are disambiguated in the status label', await page.locator('#status-workspace-name').textContent() === 'Parser (C:/Projects/Alpha)');
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    ok('workspace choices show stable IDs, names and separate paths', await page.locator('#status-ws-recent [data-workspace-id="ws-a"]').textContent() === 'ParserC:/Projects/Alpha' && await page.locator('#status-ws-recent [data-workspace-id="ws-b"]').textContent() === 'ParserD:/Projects/Beta');
    ok('the pane\'s chooser tells workspaces of one name apart', await page.locator('#ws-select option[data-workspace-id="ws-a"]').textContent() !== await page.locator('#ws-select option[data-workspace-id="ws-b"]').textContent());
    ok('Unix paths differing only by case retain distinct workspace labels and identities', await page.locator('#status-ws-recent [data-workspace-id="ws-unix-upper"]').textContent() === 'Upper Case Folder/home/test/Work' && await page.locator('#status-ws-recent [data-workspace-id="ws-unix-lower"]').textContent() === 'Lower Case Folder/home/test/work');
    ok('POSIX backslash filenames remain distinct from nested folder paths', await page.locator('#status-ws-recent [data-workspace-id="ws-unix-backslash"]').textContent() === 'Backslash Folder/tmp/a\\b' && await page.locator('#status-ws-recent [data-workspace-id="ws-unix-slash"]').textContent() === 'Nested Folder/tmp/a/b');
    await page.locator('#workspace-chats [data-session="chat-a"]').click();
    const chat = page.locator('.win[data-win-kind="chat-session"]');
    await chat.locator('[data-view-id="prompt"]').fill('Keep this chat draft');
    const chatKey = await chat.getAttribute('data-win');
    const before = await page.evaluate(() => __calls.slice());
    const rename = async () => {
      await page.locator('#status-workspace').click();
      await page.locator('#workspace-status-menu [data-cmd="rename-workspace"]').click();
    };
    await rename();
    await page.locator('#rename-workspace-name').fill('Parser Release');
    await page.evaluate(() => { window.__holdRename = true; });
    await page.locator('#rename-workspace-save').click();
    await page.waitForFunction(() => typeof __finishRename === 'function');
    ok('rename waits for durable success before changing the label', await page.locator('#rename-workspace-name').isDisabled() && await page.locator('#status-workspace-name').textContent() === 'Parser (C:/Projects/Alpha)');
    await t.shot('workspace-rename');
    await page.evaluate(() => { __holdRename = false; __finishRename(); });
    await page.locator('#rename-workspace').waitFor({ state: 'hidden' });
    await page.locator('#status-workspace-name').getByText('Parser Release', { exact: true }).waitFor();
    ok('renaming preserves open window identity and unsaved content', await chat.getAttribute('data-win') === chatKey && await chat.locator('[data-view-id="prompt"]').inputValue() === 'Keep this chat draft');
    ok('chat labels update without replay or source writes', (await chat.locator('[data-view-id="page-state"]').textContent()).includes('Parser Release') && await page.evaluate(count => __calls.slice(count).every(call => call[0] === 'rename'), before.length));
    await page.evaluate(() => dinahWorkbench.open('chat-browser'));
    await page.locator('#chat-scope').selectOption('all');
    await page.waitForFunction(() => document.getElementById('sessions').textContent.includes('Parser Release'));
    ok('chat browser shows workspace display names', (await page.locator('#sessions').textContent()).includes('Parser Release'));
    await page.keyboard.press('Control+p');
    await page.locator('#go-to-filter').fill('Parser Release');
    ok('Go To finds items by workspace display name', (await page.locator('#go-to-list').textContent()).includes('Alpha Chat'));
    await page.locator('#go-to').press('Escape');
    await rename();
    await page.locator('#rename-workspace-name').fill('Failed Name');
    await page.evaluate(() => { window.__renameFail = true; });
    await page.locator('#rename-workspace-save').click();
    await page.locator('#rename-workspace-error').getByText(/could not be saved/).waitFor();
    ok('failed rename retains the saved name and allows retry', await page.locator('#status-workspace-name').textContent() === 'Parser Release' && await page.locator('#rename-workspace-save').isEnabled());
    await page.locator('#rename-workspace-cancel').click();
    await page.locator('#status-workspace').click();
    await page.locator('#status-ws-recent [data-workspace-id="ws-missing"]').click();
    await page.getByText('Error: The workspace folder is unavailable: Z:/Offline', { exact: true }).waitFor();
    ok('unavailable selection retains current workspace and the missing record', await page.locator('#status-workspace-name').textContent() === 'Parser Release' && await page.locator('#status-ws-recent [data-workspace-id="ws-missing"]').count() === 1);
    if (!await page.locator('#workspace-status-menu').isVisible()) await page.locator('#status-workspace').click();
    await page.locator('#status-ws-recent [data-workspace-id="ws-b"]').click();
    await page.locator('#status-workspace-name').getByText('Parser', { exact: true }).waitFor();
    ok('selection uses stable identity while open chat paths stay unchanged', await page.evaluate(() => __calls.some(call => call[0] === 'select' && call[1] === 'ws-b')) && await chat.getAttribute('data-win') === chatKey);
    await page.reload();
    await page.locator('#status-workspace-name').getByText('Parser', { exact: true }).waitFor();
    ok('rename and selected identity survive reload from the native fixture', await page.locator('#status-ws-recent [data-workspace-id="ws-a"]').textContent() === 'Parser ReleaseC:/Projects/Alpha' && records[0].sources[0].id === 'root-a' && selected === 'ws-b');
    await page.locator('#status-workspace').click();
    await page.locator('#status-ws-recent [data-workspace-id="ws-unix-backslash"]').click();
    await page.locator('#status-workspace-name').getByText('Backslash Folder', { exact: true }).waitFor();
    ok('selecting a POSIX backslash folder uses its own stable identity', selected === 'ws-unix-backslash' && await page.evaluate(() => __calls.some(call => call[0] === 'select' && call[1] === 'ws-unix-backslash')));
    await page.locator('#status-workspace').click();
    await page.locator('#status-ws-recent [data-workspace-id="ws-unix-slash"]').click();
    await page.locator('#status-workspace-name').getByText('Nested Folder', { exact: true }).waitFor();
    ok('selecting the nested POSIX folder uses the other stable identity', selected === 'ws-unix-slash' && await page.evaluate(() => __calls.some(call => call[0] === 'select' && call[1] === 'ws-unix-slash')));
    // Remove Workspace… asks first, then forgets the workspace in front and
    // shows the one that takes its place.
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd="remove-workspace"]').click();
    await page.locator('#ask').getByText('Remove "Nested Folder"?').waitFor();
    await page.locator('#ask').getByRole('button', { name: 'Remove' }).click();
    await page.locator('#status-workspace-name').getByText('Upper Case Folder', { exact: true }).waitFor();
    ok('Remove Workspace… asks, forgets the workspace and shows another', !records.some(r => r.id === 'ws-unix-slash') &&
      await page.locator('#status-ws-recent [data-workspace-id="ws-unix-slash"]').count() === 0);
    // Pinned is frozen under the toolbar while the rest of the pane scrolls,
    // and still opens and closes.
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    await page.locator('#workspace-pins').getByText('Pinned Chat').waitFor();
    const pinsTop = async () => (await page.locator('#workspace-pins-group').boundingBox()).y;
    const pinsBefore = await pinsTop();
    await page.evaluate(() => { const s = document.getElementById('sidebar'); const pad = document.createElement('div'); pad.style.height = '2000px'; s.appendChild(pad); s.scrollTop = 1500; });
    await page.waitForTimeout(100);
    ok('Pinned stays at the top of the Workspace pane while it scrolls', Math.abs(await pinsTop() - pinsBefore) < 2 &&
      await page.evaluate(() => document.getElementById('workspace-pins-group').parentElement.classList.contains('ws-frozen')));
    await page.locator('#workspace-pins-group .browser-group-toggle').click();
    ok('Pinned still closes', await page.locator('#workspace-pins').isHidden());
    await page.locator('#workspace-pins-group .browser-group-toggle').click();
  } finally { await t.close(); }
})();
