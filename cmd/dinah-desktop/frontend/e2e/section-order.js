// The order of the Workspace pane's sections (workspace-sections.js,
// docs/plans/overnight-2026-10-31.md). Pinned stays first and cannot be
// moved. Chats, In Progress and Sources move from a heading's context menu,
// from Actions for the heading that last had focus, and with Alt+Up and
// Alt+Down; a move at an end is offered but disabled. Moving keeps focus on
// the heading and does not scroll the pane. The order is kept across a
// reload and shared by every workspace, and a section added later, such as
// an applet's, goes at the end. (Reordering by dragging needs a PUDL
// reorderable-list contract, which does not exist yet.) The fixture is the
// Sources check's.
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
    const order = () => page.evaluate(() => [...document.querySelectorAll('#sidebar > .browser-group[data-section], #sidebar > .ws-frozen > .browser-group[data-section]')].map(s => s.querySelector('.browser-group-title').textContent.trim()).filter(Boolean));
    const head = name => page.locator('#sidebar .browser-group-head').filter({ has: page.locator('.browser-group-title', { hasText: new RegExp('^' + name + '$') }) });
    const toggle = name => head(name).locator('.browser-group-toggle');
    const focused = () => page.evaluate(() => { const h = document.activeElement && document.activeElement.closest('.browser-group-head'); return h ? h.querySelector('.browser-group-title').textContent.trim() : ''; });
    const scrollTop = () => page.evaluate(() => { let e = document.getElementById('sidebar'), out = []; while (e) { out.push(e.scrollTop); e = e.parentElement; } return out.join(','); });
    const contextRows = async name => {
      await toggle(name).focus();
      await page.keyboard.press('Shift+F10');
      const menu = page.locator('.context-menu');
      await menu.waitFor({ timeout: 3000 });
      return menu.locator('.menu-action').evaluateAll(as => as.map(a => a.textContent.trim() + (a.disabled || a.getAttribute('aria-disabled') === 'true' ? ' (off)' : '')));
    };
    const is = (list) => async () => JSON.stringify(await order()) === JSON.stringify(list);
    await sources.getByRole('button', { name: 'Expand Working Files', exact: true }).waitFor();
    ok('the sections start in the pane order, Pinned first', await is(['Pinned', 'Chats', 'In Progress', 'Sources'])(), JSON.stringify(await order()));
    // The heading's context menu.
    let rows = await contextRows('Chats');
    ok('a heading context menu offers Move Up and Move Down, Up disabled at the top', rows.includes('Move Chats Up (off)') && rows.includes('Move Chats Down'), JSON.stringify(rows));
    const before = await scrollTop();
    await page.locator('.context-menu .menu-action', { hasText: 'Move Chats Down' }).click();
    ok('Move Down moves the section one place down, Pinned staying first', await is(['Pinned', 'In Progress', 'Chats', 'Sources'])(), JSON.stringify(await order()));
    ok('focus stays on its heading and nothing scrolls', await focused() === 'Chats' && await scrollTop() === before, JSON.stringify({ focused: await focused(), before, after: await scrollTop() }));
    // Alt+Down and Alt+Up on a heading.
    await toggle('Chats').focus();
    await page.keyboard.press('Alt+ArrowDown');
    ok('Alt+Down moves it down again', await is(['Pinned', 'In Progress', 'Sources', 'Chats'])(), JSON.stringify(await order()));
    await page.keyboard.press('Alt+ArrowDown');
    ok('at the bottom, Alt+Down changes nothing and focus stays', await is(['Pinned', 'In Progress', 'Sources', 'Chats'])() && await focused() === 'Chats');
    rows = await contextRows('Chats');
    ok('at the bottom, Move Down is offered but disabled', rows.includes('Move Chats Down (off)') && rows.includes('Move Chats Up'), JSON.stringify(rows));
    await page.keyboard.press('Escape');
    await toggle('Chats').focus();
    await page.keyboard.press('Alt+ArrowUp');
    ok('Alt+Up moves it up', await is(['Pinned', 'In Progress', 'Chats', 'Sources'])(), JSON.stringify(await order()));
    // Actions, for the heading that last had focus.
    await toggle('Sources').focus();
    await page.locator('#ws-actions-button').click();
    await page.locator('#ws-actions:popover-open').waitFor();
    await page.waitForTimeout(200);
    const actionRows = await page.locator('#ws-actions .menu-action').evaluateAll(as => as.map(a => a.textContent.trim() + (a.disabled ? ' (off)' : '')));
    ok('Actions offers Move Sources Up and, at the bottom, a disabled Move Sources Down', actionRows.includes('Move Sources Up') && actionRows.includes('Move Sources Down (off)'), JSON.stringify(actionRows));
    await page.locator('#ws-actions .menu-action', { hasText: 'Move Sources Up' }).click();
    ok('Actions moves the heading that last had focus', await is(['Pinned', 'In Progress', 'Sources', 'Chats'])(), JSON.stringify(await order()));
    // Pinned cannot be moved, and nothing moves above it.
    rows = await contextRows('Pinned').catch(() => []);
    ok('the Pinned heading offers no move', !rows.some(r => /^Move/.test(r)), JSON.stringify(rows));
    await page.keyboard.press('Escape');
    await toggle('Pinned').focus();
    await page.keyboard.press('Alt+ArrowDown');
    rows = await contextRows('In Progress');
    ok('Pinned stays first: Alt+Down on it does nothing, and the first section has Up disabled', await is(['Pinned', 'In Progress', 'Sources', 'Chats'])() && rows.includes('Move In Progress Up (off)'), JSON.stringify({ order: await order(), rows }));
    await page.keyboard.press('Escape');
    // A section added later goes at the end.
    await page.evaluate(() => { const g = window.dinahBrowserList.group({ id: 'applet-later', title: 'Later Applet', open: true }); document.getElementById('workspace-applets').appendChild(g.el); });
    await page.waitForFunction(() => { const s = [...document.querySelectorAll('#sidebar > .browser-group[data-section]')]; return s.length && s[s.length - 1].dataset.section === 'applet-later'; });
    ok('a section added later goes at the end of the order', await is(['Pinned', 'In Progress', 'Sources', 'Chats', 'Later Applet'])(), JSON.stringify(await order()));
    // Kept across a reload, and the same in another workspace.
    await page.evaluate(() => window.dinahUIState && window.dinahUIState.flush && window.dinahUIState.flush());
    await page.waitForTimeout(500);
    await page.goto(PAGE);
    await t.waitReady();
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    await page.waitForTimeout(800);
    ok('the order is kept across a reload', await is(['Pinned', 'In Progress', 'Sources', 'Chats'])(), JSON.stringify(await order()));
    await page.evaluate(() => { const s = document.getElementById('ws-select'); const o = [...s.options].find(o => /Other Work/.test(o.textContent)); s.value = o.value; s.dispatchEvent(new Event('change', { bubbles: true })); });
    await page.waitForTimeout(800);
    ok('another workspace shows the same order', await is(['Pinned', 'In Progress', 'Sources', 'Chats'])() && await page.evaluate(() => /Other Work/.test(document.getElementById('ws-select').selectedOptions[0].textContent)), JSON.stringify(await order()));
    await t.shot('section-order', { clip: await page.locator('#sidebar').boundingBox() });
  } finally { await t.close(); }
})();
