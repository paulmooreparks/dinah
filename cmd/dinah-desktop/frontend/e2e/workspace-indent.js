// One indent rule for the Workspace pane's sections (app.css). A section's
// content starts where its heading's words do, and each nested level one
// step in: the Pinned and Chats rows; In Progress's workspace groups, their
// kinds of work and their rows; and the Sources tree, three levels deep.
// The fixture is the Sources check's: tree typography, source menus
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
    await sources.getByRole('button', { name: 'Expand Working Files', exact: true }).click();
    await sources.getByRole('button', { name: 'Expand Nested', exact: true }).waitFor();
    await sources.getByRole('button', { name: 'Expand Nested', exact: true }).click();
    await resource('Nested Readme').waitFor();
    // Rows of the other sections, drawn as their own modules draw them.
    await page.evaluate(() => {
      const L = window.dinahBrowserList;
      document.getElementById('workspace-chats').appendChild(L.row({ title: 'Sample Chat', meta: ['A chat'] }));
      document.getElementById('workspace-pins').appendChild(L.row({ title: 'Sample Pin', meta: ['A pin'] }));
      const host = document.getElementById('workspace-in-progress');
      const ws = L.group({ id: 'zz-ws', title: 'Delivery', count: 1, open: true }); host.appendChild(ws.el);
      const kind = L.group({ id: 'zz-kind', title: 'Drafts', count: 1, open: true }); kind.el.classList.add('in-progress-kind'); ws.body.appendChild(kind.el);
      kind.body.appendChild(L.row({ title: 'Sample Draft', meta: ['A draft'] }));
      document.querySelectorAll('#workspace-chats-empty, #workspace-in-progress-empty, #workspace-pins-empty').forEach(e => e.hidden = true);
    });
    await page.waitForTimeout(300);
    const m = await page.evaluate(() => {
      const textLeft = el => { const n = [...el.querySelectorAll('*')].concat([el]).find(x => [...x.childNodes].some(c => c.nodeType === 3 && c.textContent.trim())); const tn = [...n.childNodes].find(c => c.nodeType === 3 && c.textContent.trim()); const r = document.createRange(); r.selectNodeContents(tn); return Math.round(r.getBoundingClientRect().left); };
      const left = el => Math.round(el.getBoundingClientRect().left);
      const heading = name => [...document.querySelectorAll('#sidebar .browser-group-title')].find(h => h.textContent.trim() === name);
      const toggleOf = h => left(h.closest('.browser-group-head').querySelector('.browser-group-toggle'));
      const node = name => [...document.querySelectorAll('#workspace-source-tree [data-resource]')].find(a => a.querySelector('.fm-ti-label').textContent === name);
      const sidebar = document.getElementById('sidebar');
      return {
        heads: ['Pinned', 'Chats', 'In Progress', 'Sources'].map(n => textLeft(heading(n))),
        pin: textLeft(document.querySelector('#workspace-pins .browser-row-title')), chat: textLeft(document.querySelector('#workspace-chats .browser-row-title')),
        wsGroup: toggleOf(heading('Delivery')), kindGroup: toggleOf(heading('Drafts')), draft: textLeft(document.querySelector('#workspace-in-progress .in-progress-kind .browser-row-title')),
        tree: ['Working Files', 'Nested', 'Nested Readme'].map(n => ({ row: left(node(n)), toggle: node(n).querySelector('.tree-toggle') ? left(node(n).querySelector('.tree-toggle')) : null })),
        overflow: sidebar.scrollWidth > sidebar.clientWidth + 1,
      };
    });
    const start = m.heads[0], step = 16;
    ok('the words of every section heading start at one line', m.heads.every(h => h === start), JSON.stringify(m.heads));
    ok('Pinned and Chats rows start where the words of their heading start', m.pin === start && m.chat === start, JSON.stringify(m));
    ok('In Progress steps in one level at a time: workspace, kind of work, then its rows', m.wsGroup === start && m.kindGroup === start + step && m.draft === start + 2 * step, JSON.stringify(m));
    ok('the Sources tree starts at the words of its heading and steps in one level at a time, three deep', m.tree[0].row === start && m.tree[1].row === start + step && m.tree[2].row === start + 2 * step && m.tree[0].toggle >= start && m.tree[0].toggle < start + 8, JSON.stringify(m.tree));
    ok('nothing in the pane runs past its edge', !m.overflow);
    await t.shot('workspace-indent', { clip: await page.locator('#sidebar').boundingBox() });
  } finally { await t.close(); }
})();
