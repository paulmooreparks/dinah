// Local-folder sources in the Workspace pane: tree typography, source menus
// and keys, lazy paged listing, an unavailable source recovering through
// Retry, empty folders, refresh, folder actions, selection without a
// preview, restore on reload, read-only resource windows keeping their owner
// after a workspace switch, Add Source…, live labels and confirmed removal.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    app: () => {
      window.__calls = [];
      let records = JSON.parse(localStorage.getItem('fixture-records') || 'null') || [
        { id: 'w1', name: 'Delivery', primarySourceId: 'primary', sources: [{ id: 'primary', kind: 'local-folder', name: 'Working Files', path: 'C:/Working' }, { id: 'shared', kind: 'local-folder', name: 'Shared Drive', path: 'S:/Shared' }] },
        { id: 'w2', name: 'Other Work', primarySourceId: 'second', sources: [{ id: 'second', kind: 'local-folder', name: 'Other Folder', path: 'C:/Other' }] }
      ];
      let selected = localStorage.getItem('fixture-selected') || 'w1';
      const current = () => records.find(record => record.id === selected);
      const aliases = () => records.forEach(record => { record.mountAlias ||= record.id; record.sources.forEach(source => { source.mountAlias ||= source.id; }); });
      aliases();
      window.__renameFixtureSource = (id, title) => { current().sources.find(source => source.id === id).name = title; save(); window.__emit('workspace:changed', {kind:'source-renamed',workspaceId:selected,sourceId:id}); };
      const state = () => { aliases(); return { workspace: current().sources[0].path, workspaceId: selected, workspaceRecords: records, workspaces: ['C:/Working', 'C:/Other'], recent: ['C:/Working', 'C:/Other'], defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [] }; };
      const save = () => { aliases(); localStorage.setItem('fixture-records', JSON.stringify(records)); };
      const item = (ref, name, directory = false, actions = directory ? ['list'] : ['read', 'preview', 'open-external']) => ({ ref, title: name, kind: directory ? 'folder' : 'text', directory, revision: '1', actions });
      const backend = {
        State: async () => state(), ReadUIState: async () => JSON.parse(localStorage.getItem('fixture-ui') || '{"version":1,"values":{}}'), SaveUIState: async value => localStorage.setItem('fixture-ui', JSON.stringify(value)),
        SelectWorkspace: async id => { selected = id; localStorage.setItem('fixture-selected', id); return state(); },
        ChooseWorkspaceSource: async id => { window.__calls.push(['choose', id]); const record = records.find(record => record.id === id); if (!window.__cancelFolder) record.sources.push({ id: 'added', name: 'Reference Files', kind: 'local-folder', path: 'D:/References' }); save(); return record; },
        RenameDocumentFile: async (ref, name) => { window.__calls.push(['rename', ref.path, name]); return { ref: { ...ref, path: name }, name, drafts: [] }; },
        DetachWorkspaceSource: async (id, sourceId) => { window.__calls.push(['detach', id, sourceId]); const record = records.find(record => record.id === id); record.sources = record.sources.filter(source => source.id !== sourceId); save(); return record; },
        ListWorkspaceResources: async (ref, start, limit) => {
          window.__calls.push(['list', ref, start, limit]);
          // The shared drive is offline until a check brings it back, then
          // gives an empty first page with a next cursor.
          if (ref.sourceId === 'shared') {
            if (!window.__sharedOnline) throw new Error('The folder S:/Shared is unavailable');
            if (window.__sharedEmpty || ref.path === 'archive') return { entries: [], nextStart: -1 };
            if (start === 0) return { entries: [], nextStart: 7 };
            return { entries: [item({ ...ref, path: 'archive' }, 'Archive', true)], nextStart: -1 };
          }
          if (ref.path === 'nested') return { entries: [item({ ...ref, path: 'nested/readme.md' }, 'Nested Readme')], nextStart: -1 };
          if (ref.path || ref.sourceId !== 'primary') return { entries: [], nextStart: -1 };
          return start ? { entries: [item({ ...ref, path: 'last.md' }, 'Last Page')], nextStart: -1 } : { entries: [{ ...item({ ...ref, path: 'nested' }, 'Nested', true), writable: true, path: '/workspaces/delivery/working/nested' }, item({ ...ref, path: 'report.md' }, 'Report', false, ['read', 'preview', 'open-external', 'rename']), item({ ...ref, path: 'slow.md' }, 'Slow Page'), item({ ...ref, path: 'script.ps1' }, 'Script', false, ['reveal'])], nextStart: 4 };
        },
        PreviewWorkspaceResource: async ref => {
          window.__calls.push(['preview', ref]);
          if (ref.path === 'slow.md' && window.__holdPreview) await new Promise(resolve => { window.__finishPreview = resolve; });
          if (window.__previewFail) throw new Error('Resource unavailable');
          const title = ref.path;
          return { resource: item(ref, title, false, ['preview', 'open-external']), blocks: [{ line: 1, html: '<h2>Preview ' + title + '</h2><img data-attachment="diagram.png"><p>' + 'Read-only content. '.repeat(200) + '</p>' }], problems: [] };
        },
        OpenWorkspaceResource: async ref => { window.__calls.push(['external', ref]); },
      };
      const mountedItems = new Map();
      const resolve = virtual => {
        const parts = virtual.split('/').filter(Boolean), record = records.find(record => record.mountAlias === parts[1]);
        const source = record && record.sources.find(source => source.mountAlias === parts[2]);
        if (parts.length > 1 && !record || parts.length > 2 && !source) throw new Error('Mount unavailable');
        return { record, source, ref: source && {workspaceId:record.id,sourceId:source.id,path:parts.slice(3).join('/')} };
      };
      const mount = (record, source, resource) => ({...resource,path:'/workspaces/' + record.mountAlias + '/' + source.mountAlias + (resource.ref.path ? '/' + resource.ref.path : '')});
      backend.StatWorkspaceMount = async virtual => {
        const {record,source,ref} = resolve(virtual);
        if (!source) return {path:virtual,title:record ? record.name : 'Workspaces',kind:'catalog',directory:true,actions:['list']};
        if (ref.path && mountedItems.has(virtual)) return mountedItems.get(virtual);
        const result = item({...ref,path:ref.path || ''},ref.path || source.name,true,['list']);
        // A source's root has the source's kind, as Go gives it.
        if (!ref.path) result.kind = source.kind;
        return mount(record,source,result);
      };
      backend.StatWorkspaceMountExpected = async (virtual, expected) => {
        const result = await backend.StatWorkspaceMount(virtual);
        if (result.ref && (result.ref.workspaceId !== expected.workspaceId || result.ref.sourceId !== expected.sourceId)) throw new Error('Mount identity changed');
        return result;
      };
      backend.ListWorkspaceMounts = async (virtual,start,limit) => {
        const {record,source,ref} = resolve(virtual);
        if (source) { const result = await backend.ListWorkspaceResources(ref,start,limit); return {...result,entries:result.entries.map(resource => { const value = mount(record,source,resource); mountedItems.set(value.path,value); return value; })}; }
        const entries = record ? record.sources.map(source => mount(record,source,{...item({workspaceId:record.id,sourceId:source.id,path:''},source.name,true,['list']),kind:source.kind})) : [];
        return {entries,nextStart:-1};
      };
      backend.ListWorkspaceMountsExpected = async (virtual,start,limit,expected) => { await backend.StatWorkspaceMountExpected(virtual,expected); return backend.ListWorkspaceMounts(virtual,start,limit); };
      return backend;
    },
    goto: false,
  });
  const { page, ok } = t;
  try {
    await t.context.route('http://dinah.test/resource-attachment?*', route => route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="80" height="50"><rect width="80" height="50" fill="teal"/></svg>' }));
    await page.goto(PAGE);
    await t.waitReady();
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    const sources = page.locator('#workspace-sources');
    // A row by its name.
    const resource = title => sources.locator('[data-resource]').filter({ has: page.locator('.fm-ti-label', { hasText: new RegExp('^' + title + '$') }) });
    // A row's PUDL row actions button, and its menu opened from the pointer.
    const actions = title => resource(title).locator('xpath=../button[@data-row-actions]');
    const openActions = async title => { await resource(title).hover(); await actions(title).click(); await page.locator('.context-menu').waitFor(); };
    await sources.getByRole('button', { name: 'Expand Working Files', exact: true }).waitFor();
    const quiet = await actions('Shared Drive').evaluate(button => ({ hidden: getComputedStyle(button).visibility === 'hidden', aria: button.getAttribute('aria-hidden'), tab: button.tabIndex, width: button.getBoundingClientRect().width }));
    await resource('Shared Drive').hover();
    ok('source rows use PUDL row actions buttons, shown only on the row under the pointer', quiet.hidden && quiet.aria === 'true' && quiet.tab === -1 && quiet.width >= 24 && await actions('Shared Drive').isVisible());
    const treeStyle = await resource('Working Files').evaluate(link => {
      const reference = document.createElement('ul'); reference.className = 'tree';
      reference.innerHTML = '<li><a href="#"><span class="tree-toggle"></span>Reference</a></li>'; document.body.appendChild(reference);
      const actual = getComputedStyle(link), expected = getComputedStyle(reference.querySelector('a'));
      const toggle = link.querySelector('.tree-toggle'), menu = link.parentElement.querySelector(':scope > [data-row-actions]');
      const result = { shared: actual.fontSize === expected.fontSize && actual.lineHeight === expected.lineHeight && actual.paddingInlineStart === expected.paddingInlineStart && actual.columnGap === expected.columnGap, toggleWidth:toggle.getBoundingClientRect().width, margin:link.parentElement.getBoundingClientRect().right - menu.getBoundingClientRect().right };
      reference.remove(); return result;
    });
    ok('Workspace uses the Files PUDL tree typography and disclosure spacing with a right menu margin', treeStyle.shared && treeStyle.toggleWidth === 16 && treeStyle.margin >= 4);
    ok('all source roots are visible without eager listing or preview calls', await resource('Shared Drive').isVisible() && await page.evaluate(() => !__calls.some(call => ['list', 'preview'].includes(call[0]))));
    await openActions('Working Files');
    ok('the original folder offers the same detach action as other sources', await page.locator('.context-menu').getByText('Remove from Workspace…', { exact: true }).count() === 1);
    const covered = await page.evaluate(() => {
      const b = document.querySelector('li[data-menu-for] > [data-row-actions]').getBoundingClientRect(), m = document.querySelector('.context-menu').getBoundingClientRect();
      return m.top < b.bottom && m.bottom > b.top && m.left < b.right && m.right > b.left;
    });
    ok('the menu opens beside its button, never over it', !covered);
    await actions('Working Files').click();
    ok('a second click closes the source menu', await page.locator('.context-menu').count() === 0);
    await openActions('Shared Drive');
    ok('another source button opens its own actions', await page.locator('.context-menu').count() === 1 && await page.locator('li[data-menu-for]').evaluate(row => row.querySelector(':scope > a .fm-ti-label').textContent) === 'Shared Drive');
    await page.keyboard.press('Escape');
    await resource('Shared Drive').click({ button: 'right' });
    ok('a right-click opens the same menu as the button', await page.locator('.context-menu').getByText('Refresh Source', { exact: true }).count() === 1 && await page.locator('li[data-menu-for]').evaluate(row => row.querySelector(':scope > a .fm-ti-label').textContent) === 'Shared Drive');
    await page.keyboard.press('Escape');
    await resource('Working Files').focus();
    await page.keyboard.press('Shift+F10');
    ok('Shift+F10 opens and focuses the source menu', await page.locator('.context-menu').isVisible() && await page.locator('.context-menu .menu-action').first().evaluate(element => element === document.activeElement));
    await page.keyboard.press('Escape');
    await sources.getByRole('button', { name: 'Expand Working Files', exact: true }).click();
    await resource('Working Files').focus();
    await resource('Working Files').press('ArrowDown');
    ok('PUDL tree keys move focus to the next visible source resource', await resource('Nested').evaluate(link => link === document.activeElement));
    await resource('Report').waitFor();
    ok('local resources list lazily with the shared bounded page size', await page.evaluate(() => __calls.some(call => call[0] === 'list' && call[1].sourceId === 'primary' && call[2] === 0 && call[3] === 100)));
    await sources.getByRole('button', { name: 'Load More', exact: true }).click();
    await resource('Last Page').waitFor();
    ok('Load More appends the next page without repeating the first page', await resource('Report').count() === 1 && await page.evaluate(() => __calls.some(call => call[0] === 'list' && call[2] === 4)));
    // A folder on the computer takes uploads, opens in Files and copies its
    // path, and its row takes dropped files.
    await openActions('Nested');
    const nestedMenu = await page.locator('.context-menu .menu-action').allTextContents();
    ok('a folder on the computer offers Upload Files…, Open in Files and Copy Path, and takes dropped files', ['Upload Files…', 'Open in Files', 'Copy Path'].every(label => nestedMenu.includes(label)) &&
      await resource('Nested').evaluate(link => !!link.closest('[data-upload-target]')));
    await page.keyboard.press('Escape');
    await sources.getByRole('button', { name: 'Expand Nested', exact: true }).click();
    await resource('Nested Readme').waitFor();
    await resource('Slow Page').click();
    await resource('Report').click();
    ok('single click only selects with no Workspace preview or working-folder chip', await resource('Report').getAttribute('aria-current') === 'true' && await page.locator('#workspace-resource-preview').count() === 0 && await sources.getByText('Working Folder', { exact: true }).count() === 0 && await page.evaluate(() => !__calls.some(call => call[0] === 'preview')));
    ok('local folders and local files have distinct glyphs', await resource('Working Files').locator('.glyph').getAttribute('style') !== await resource('Report').locator('.glyph').getAttribute('style'));
    await page.evaluate(() => dinahUIState.flush());
    await page.reload();
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    await resource('Report').waitFor();
    ok('expanded folders and selected resource survive reload without reading content', await resource('Nested Readme').count() === 1 && await resource('Report').getAttribute('aria-current') === 'true' && await page.evaluate(() => !__calls.some(call => call[0] === 'preview')));
    // A file whose actions allow it is renamed from its row's menu.
    await openActions('Report');
    await page.locator('.context-menu').getByRole('menuitem', { name: 'Rename…', exact: true }).click();
    const renameDialog = page.locator('dialog.dinah-ask-name[open]');
    await renameDialog.getByLabel('Name').fill('summary.md');
    await renameDialog.getByRole('button', { name: 'Rename', exact: true }).click();
    await renameDialog.waitFor({ state: 'detached' });
    ok('a file row offers Rename…, which renames the file', await page.evaluate(() => __calls.some(call => call[0] === 'rename' && call[1] === 'report.md' && call[2] === 'summary.md')));
    await resource('Report').press('Enter');
    const resourceWindow = page.locator('.win[data-win-kind="workspace-resource"]');
    await resourceWindow.locator('[data-doc-page]').getByText('Preview report.md', { exact: true }).waitFor();
    await resourceWindow.locator('img[data-attachment]').evaluate(image => image.decode());
    ok('resource windows identify their owner and load source attachments', (await resourceWindow.locator('[data-doc-meta]').textContent()).includes('Delivery · Working Files') && await resourceWindow.locator('img[data-attachment]').evaluate(image => image.naturalWidth === 80 && new URL(image.src).searchParams.get('path') === 'report.md'));
    const resourceKey = await resourceWindow.getAttribute('data-win');
    ok('Enter opens a read-only document addressed by workspace and source IDs', await page.evaluate(key => { const item = dinahWorkbench.item(key); const ref = dinahWorkbench.resourceRef(item); return ref.workspaceId === 'w1' && ref.sourceId === 'primary' && ref.path === 'report.md'; }, resourceKey) && await resourceWindow.locator('[data-doc-request]').isHidden());
    await page.locator('#status-workspace').click();
    await page.locator('#status-ws-recent [data-workspace-id="w2"]').click();
    await page.locator('#status-workspace-name').getByText('Other Work', { exact: true }).waitFor();
    await resourceWindow.locator('[data-doc-refresh]').click();
    ok('resource windows retain their source after the current workspace changes', await resourceWindow.getAttribute('data-win') === resourceKey && await page.evaluate(() => __calls.filter(call => call[0] === 'preview').at(-1)[1].workspaceId === 'w1'));
    await page.locator('#status-workspace').click();
    await page.locator('#status-ws-recent [data-workspace-id="w1"]').click();
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    await resource('Script').click();
    await openActions('Script');
    await page.locator('.context-menu').getByRole('menuitem', { name: 'Show in Folder', exact: true }).waitFor();
    ok('unsupported file types offer reveal without promising a preview or execution', await page.locator('.context-menu').getByRole('menuitem', { name: 'Show in Folder', exact: true }).count() === 1 && await page.locator('.context-menu').getByRole('menuitem', { name: 'Open', exact: true }).count() === 0 && await page.locator('.context-menu').getByRole('menuitem', { name: 'Rename…', exact: true }).count() === 0);
    await page.locator('.context-menu').getByRole('menuitem', { name: 'Show in Folder', exact: true }).click();
    await sources.getByRole('button', { name: 'Expand Shared Drive', exact: true }).click();
    await sources.getByText('Unavailable: The folder S:/Shared is unavailable', { exact: true }).waitFor();
    await page.evaluate(() => { window.__sharedOnline = true; });
    await sources.getByRole('button', { name: 'Retry', exact: true }).click();
    const sharedBranch = resource('Shared Drive').locator('..');
    await sharedBranch.getByRole('button', { name: 'Load More', exact: true }).click();
    await resource('Archive').waitFor();
    ok('unavailable sources recover and empty pages with a next cursor remain pageable', await page.evaluate(() => __calls.filter(call => call[0] === 'list' && call[1].sourceId === 'shared').length === 3 && __calls.some(call => call[0] === 'list' && call[1].sourceId === 'shared' && call[2] === 7)));
    await sources.getByRole('button', { name: 'Expand Archive', exact: true }).click();
    const archiveBranch = resource('Archive').locator('..');
    await archiveBranch.getByText('No items.', { exact: true }).waitFor();
    ok('an empty folder says it has no items', await archiveBranch.getByText('No items.', { exact: true }).count() === 1);
    await page.evaluate(() => { window.__sharedEmpty = true; });
    await openActions('Shared Drive');
    await page.locator('.context-menu').getByRole('menuitem', { name: 'Refresh Source', exact: true }).click();
    await resource('Archive').waitFor({ state: 'hidden' });
    ok('refreshing an emptied source root says it has no items', await sharedBranch.getByText('No items.', { exact: true }).count() === 1);
    await t.shot('workspace-empty-source');
    await page.evaluate(() => { window.__sharedEmpty = false; });
    await openActions('Shared Drive');
    await page.locator('.context-menu').getByRole('menuitem', { name: 'Refresh Source', exact: true }).click();
    await sharedBranch.getByRole('button', { name: 'Load More', exact: true }).click();
    await resource('Archive').waitFor();
    ok('refresh can restore children to a previously empty root', await sources.getByRole('button', { name: 'Collapse Shared Drive', exact: true }).count() === 1);
    // Add Source… asks for a folder; Cancel in the folder dialog adds nothing.
    await page.evaluate(() => { window.__cancelFolder = true; });
    await sources.getByRole('button', { name: 'Add Source…', exact: true }).click();
    await page.waitForFunction(() => __calls.some(call => call[0] === 'choose'));
    ok('canceling the native folder picker leaves sources unchanged', await resource('Reference Files').count() === 0);
    await page.evaluate(() => { window.__cancelFolder = false; });
    await sources.getByRole('button', { name: 'Add Source…', exact: true }).click();
    await resource('Reference Files').waitFor();
    ok('Add Source… adds the chosen folder to the workspace in front', await page.evaluate(() => __calls.filter(call => call[0] === 'choose').every(call => call[1] === 'w1')) && await resource('Working Files').count() === 1);
    await page.evaluate(() => __renameFixtureSource('added', 'Updated Files'));
    await resource('Updated Files').waitFor();
    ok('Workspace updates mounted source labels live', await resource('Reference Files').count() === 0);
    await page.evaluate(() => __renameFixtureSource('added', 'Reference Files'));
    await resource('Reference Files').waitFor();
    await openActions('Reference Files');
    await page.locator('.context-menu').getByRole('menuitem', { name: 'Remove from Workspace…', exact: true }).click();
    await page.locator('#ask-no').click();
    ok('canceling Remove preserves the attached source', await resource('Reference Files').count() === 1);
    await openActions('Reference Files');
    await page.locator('.context-menu').getByRole('menuitem', { name: 'Remove from Workspace…', exact: true }).click();
    await page.locator('#ask-yes').click();
    await resource('Reference Files').waitFor({ state: 'hidden' });
    ok('Remove detaches only the selected source', await page.evaluate(() => __calls.some(call => call[0] === 'detach' && call[1] === 'w1' && call[2] === 'added')));
    await page.setViewportSize({ width: 560, height: 760 });
    await t.shot('workspace-sources-narrow');
    await page.setViewportSize({ width: 1400, height: 1000 });
    await t.shot('workspace-sources');
    await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'dark'));
    await t.shot('workspace-sources-dark');
    const sticky = await page.evaluate(() => {
      const sidebar = document.getElementById('sidebar'), selector = document.getElementById('ws-select');
      const filler = document.createElement('div'); filler.style.minHeight = '1200px'; sidebar.appendChild(filler);
      const before = selector.getBoundingClientRect().top; sidebar.scrollTop = 600;
      const after = selector.getBoundingClientRect().top, scrolled = sidebar.scrollTop;
      filler.remove(); sidebar.scrollTop = 0;
      return { before, after, scrolled };
    });
    ok('workspace switcher stays pinned while the sidebar scrolls', sticky.scrolled > 100 && Math.abs(sticky.before - sticky.after) < 1);
    await page.evaluate(() => document.documentElement.setAttribute('data-theme', 'light'));
    await page.evaluate(() => { window.__holdPreview = true; });
    await page.evaluate(() => dinahWorkbench.open(dinahWorkbench.resourceItem({ workspaceId: 'w1', sourceId: 'primary', path: 'slow.md' }, 'Slow Document')));
    await page.waitForFunction(() => __calls.some(call => call[0] === 'preview' && call[1].path === 'slow.md'));
    const slowKey = await page.locator('.win.active').getAttribute('data-win');
    await page.evaluate(key => { pudlWindows.close(key); __finishPreview(); }, slowKey);
    ok('closing a resource window discards its late preview', await page.locator('[data-win="' + slowKey + '"]').count() === 0);
  } finally { await t.close(); }
})();
