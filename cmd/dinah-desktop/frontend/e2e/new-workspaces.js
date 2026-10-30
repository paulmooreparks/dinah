const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    app: () => {
      let records = JSON.parse(localStorage.getItem('records') || '[]'), current = localStorage.getItem('selected') || '';
      const state = () => ({ workspaceId: current, workspace: records.find(r => r.id === current)?.executionPath || '', workspaceRecords: records, recent: records.map(r => r.executionPath), workspaces: records.map(r => r.executionPath), defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [] });
      const save = () => { localStorage.setItem('records', JSON.stringify(records)); localStorage.setItem('selected', current); };
      return {
        State: async () => state(),
        ListWorkspaceMounts: async () => ({ entries: [], nextStart: -1 }),
        StatWorkspaceMount: async path => ({path,title:records.find(r => path === '/workspaces/' + r.mountAlias)?.name || 'Workspaces',kind:'catalog',directory:true,actions:['list']}),
        ChooseWorkspaceSource: async id => records.find(r => r.id === id),
        CreateWorkspace: async () => { __calls.push(['create']); if (window.__failCreate) throw Error('Workspace could not be saved'); const n = records.length + 1; const record = { id: 'w' + n, name: n === 1 ? 'Workspace' : 'Workspace ' + n, mountAlias: 'workspace-' + n, executionPath: 'C:/PrivateConfig/workspaces/w' + n + '/files', sources: [] }; records.push(record); current = record.id; save(); return state(); },
        // The native folder picker returns D:/Parser, or nothing when cancelled.
        ChooseWorkspace: async () => { __calls.push(['choose']); if (window.__cancelPicker) return state(); let record = records.find(r => r.executionPath === 'D:/Parser'); if (!record) { record = { id: 'w' + (records.length + 1), name: 'Parser', mountAlias: 'parser', executionPath: 'D:/Parser', primarySourceId: 'main', sources: [{ id: 'main', kind: 'local-folder', name: 'Parser', path: 'D:/Parser' }] }; records.push(record); } current = record.id; save(); return state(); },
        SelectWorkspace: async id => { __calls.push(['select', id]); current = id; save(); return state(); },
        RenameWorkspace: async (id, name) => { const r = records.find(r => r.id === id); r.name = name; save(); return r; }
      };
    },
    init: () => { window.__calls = []; },
  });
  const { page, ok } = t;
  try {
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd=new-empty-workspace]').click();
    await page.locator('#status-workspace-name').getByText('Workspace', { exact: true }).waitFor();
    await page.locator('#workspace-sources').waitFor();
    ok('New Empty Workspace creates a named empty record without a folder picker', await page.evaluate(() => __calls.length === 1 && __calls[0][0] === 'create') && await page.locator('#workspace-source-tree .workspace-resource-node').count() === 0);
    ok('empty workspaces expose Add Source and hide internal execution paths', await page.locator('#workspace-source-choose').isEnabled() && !(await page.locator('#status-workspace').getAttribute('title')).includes('PrivateConfig'));
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd=new-empty-workspace]').click();
    await page.locator('#status-workspace-name').getByText('Workspace 2', { exact: true }).waitFor();
    await page.locator('#status-workspace').click();
    ok('all empty workspaces remain selectable by ID without exposing private folders', await page.locator('#status-ws-recent [data-workspace-id]').count() === 2 && !(await page.locator('#status-ws-recent').textContent()).includes('PrivateConfig'));
    await page.locator('#status-ws-recent [data-workspace-id=w1]').click();
    await page.locator('#status-workspace-name').getByText('Workspace', { exact: true }).waitFor();
    ok('selection uses the stable workspace ID', await page.evaluate(() => __calls.some(c => c[0] === 'select' && c[1] === 'w1')));
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd=rename-workspace]').click();
    await page.locator('#rename-workspace-name').fill('Design Review');
    await page.locator('#rename-workspace-save').click();
    await page.locator('#status-workspace-name').getByText('Design Review', { exact: true }).waitFor();
    await page.reload();
    await page.locator('#status-workspace-name').getByText('Design Review', { exact: true }).waitFor();
    ok('renamed empty workspace identity and selection survive reload', await page.locator('#workspace-source-tree .workspace-resource-node').count() === 0);
    await page.evaluate(() => { window.__failCreate = true; });
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd=new-empty-workspace]').click();
    await page.waitForFunction(() => !document.querySelector('#workspace-status-menu [data-cmd=new-empty-workspace]').disabled);
    ok('failed creation leaves the selected workspace intact', await page.locator('#status-workspace-name').textContent() === 'Design Review' && await page.evaluate(() => JSON.parse(localStorage.getItem('records')).length === 2));
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    await t.shot('new-workspace-empty');
    // New Workspace asks for the main folder; cancelling the picker changes nothing.
    await page.evaluate(() => { window.__failCreate = false; window.__cancelPicker = true; __calls.length = 0; });
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd=new-workspace]').click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'choose') && !document.querySelector('#workspace-status-menu [data-cmd=new-workspace]').disabled);
    ok('cancelling the folder picker keeps the selected workspace', await page.locator('#status-workspace-name').textContent() === 'Design Review' && await page.evaluate(() => JSON.parse(localStorage.getItem('records')).length === 2));
    await page.evaluate(() => { window.__cancelPicker = false; });
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd=new-workspace]').click();
    await page.locator('#status-workspace-name').getByText('Parser', { exact: true }).waitFor();
    ok('New Workspace makes the chosen folder the main folder of a new workspace', await page.evaluate(() => { const r = JSON.parse(localStorage.getItem('records')); return r.length === 3 && r[2].executionPath === 'D:/Parser'; }) && await page.evaluate(() => !__calls.some(c => c[0] === 'create')));
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd=new-workspace]').click();
    await page.waitForFunction(() => __calls.filter(c => c[0] === 'choose').length === 3 && !document.querySelector('#workspace-status-menu [data-cmd=new-workspace]').disabled);
    ok('choosing a folder that is already a main folder selects its workspace', await page.evaluate(() => JSON.parse(localStorage.getItem('records')).length === 3) && await page.locator('#status-workspace-name').textContent() === 'Parser');
    await page.evaluate(() => { const r = JSON.parse(localStorage.getItem('records')); localStorage.setItem('records', JSON.stringify(r.slice(0, 2))); });
    await page.evaluate(() => {
      const records = JSON.parse(localStorage.getItem('records'));
      records[0].sources = [{ id: 'attached-other', kind: 'local-folder', name: 'Other Workspace Files', path: records[1].executionPath }];
      localStorage.setItem('records', JSON.stringify(records));
      localStorage.setItem('selected', 'w2');
    });
    await page.reload();
    await page.locator('#status-workspace-name').getByText('Workspace 2', { exact: true }).waitFor();
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd=rename-workspace]').click();
    ok('an attached execution folder does not change another workspace identity', await page.locator('#rename-workspace-name').inputValue() === 'Workspace 2');
    await page.locator('#rename-workspace-name').fill('Second Workspace');
    await page.locator('#rename-workspace-save').click();
    await page.locator('#status-workspace-name').getByText('Second Workspace', { exact: true }).waitFor();
    ok('rename targets the selected identity despite another workspace attachment', await page.evaluate(() => {
      const records = JSON.parse(localStorage.getItem('records'));
      return records[0].name === 'Design Review' && records[1].name === 'Second Workspace';
    }));
  } finally { await t.close(); }
})();
