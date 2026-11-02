// The order of the Workspace pane's sections (workspace-sections.js,
// docs/plans/overnight-2026-10-31.md). Pinned stays first and cannot be
// moved. Chats, In Progress and Sources move from a heading's context menu,
// from Actions for the heading that last had focus, and with Alt+Up and
// Alt+Down; a move at an end is offered but disabled. Moving keeps focus on
// the heading and does not scroll the pane. The order is kept across a
// reload and shared by every workspace, and a section added later, such as
// an applet's, goes at the end. Reorder Sections, on PUDL's reorderable
// list, collapses the sections and moves them by grip, key or drag (see the
// end). The fixture is the Sources check's.
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

    // Reorder Sections (PUDL's reorderable list, 0.97.0): from Actions,
    // every section's body is hidden and each movable heading has PUDL's
    // grip where its toggle stands, Pinned none. Up and Down on a grip and a
    // drag of a grip move a section and save the order; Done and Escape end
    // the mode, and every section is open or closed as it was.
    const bodies = () => page.evaluate(() => [...document.querySelectorAll('#sidebar .browser-group[data-section]:not([hidden])')].map(s => s.dataset.section + ':' + (s.querySelector(':scope > .browser-group-body').getClientRects().length > 0)));
    const openAtStart = await bodies();
    const titlesAt = () => page.evaluate(() => [...document.querySelectorAll('#sidebar .browser-group[data-section]:not([hidden]) > .browser-group-head .browser-group-title')].map(t => Math.round(t.getBoundingClientRect().left)));
    const titlesBefore = await titlesAt();
    const enter = async () => { await page.click('#ws-actions-button'); await page.locator('#ws-actions [data-cmd="workspace-sections-reorder"]').click(); await page.locator('#sidebar .reorder-grip').first().waitFor(); };
    await enter();
    const mode = await page.evaluate(() => ({
      grips: [...document.querySelectorAll('#sidebar .reorder-grip')].map(g => [g.closest('[data-section]').dataset.section, g.getAttribute('aria-label') || g.textContent, !!g.closest('.browser-group-head')]),
      pinned: !!document.querySelector('#workspace-pins-group .reorder-grip'),
      open: [...document.querySelectorAll('#sidebar .browser-group[data-section]')].filter(s => s.querySelector(':scope > .browser-group-body').getClientRects().length > 0).length,
      toggles: [...document.querySelectorAll('#sidebar > .browser-group[data-reorder-id] .browser-group-toggle')].filter(b => b.getClientRects().length > 0).length,
      bar: !document.querySelector('.ws-reorder-bar').hidden,
      focus: document.activeElement && document.activeElement.matches('.reorder-grip')
    }));
    const titlesDuring = await titlesAt();
    ok('no heading\'s words move as the mode starts', JSON.stringify(titlesDuring) === JSON.stringify(titlesBefore), JSON.stringify({ titlesBefore, titlesDuring }));
    await t.shot('section-reorder-mode', { clip: await page.locator('#sidebar').boundingBox() });
    ok('Reorder Sections hides every body and puts a grip on each movable heading in its toggle\'s place', mode.grips.length === 3 && mode.grips.every(g => g[2] && /^Move /.test(g[1])) && !mode.pinned && mode.open === 0 && mode.toggles === 0 && mode.bar && mode.focus, JSON.stringify(mode));
    await page.keyboard.press('ArrowDown');
    await page.waitForTimeout(200);
    const savedOrder = () => page.evaluate(() => JSON.parse(dinahUIState.get('dinah-workspace-section-order', '[]')));
    ok('Down on a grip moves its section one place and saves the order', await is(['Pinned', 'Sources', 'In Progress', 'Chats'])() && (await savedOrder()).slice(0, 3).join() === 'workspace-sources,workspace-in-progress,workspace-chats', JSON.stringify({ order: await order(), saved: await savedOrder() }));
    // A drag of Chats' grip to above Sources' heading.
    const grip = page.locator('#sidebar [data-section="workspace-chats"] .reorder-grip'), target = await page.locator('#sidebar [data-section="workspace-sources"] > .browser-group-head').boundingBox(), from = await grip.boundingBox();
    await page.mouse.move(from.x + from.width / 2, from.y + from.height / 2); await page.mouse.down();
    await page.mouse.move(from.x + from.width / 2, from.y - 10, { steps: 4 });
    await page.mouse.move(target.x + 20, target.y + 2, { steps: 8 });
    await page.mouse.up();
    await page.waitForTimeout(300);
    ok('dragging a grip moves its section and saves the order', await is(['Pinned', 'Chats', 'Sources', 'In Progress'])() && (await savedOrder())[0] === 'workspace-chats', JSON.stringify({ order: await order(), saved: await savedOrder() }));
    ok('the frozen top and the applets\' place keep their places', await page.evaluate(() => document.getElementById('sidebar').firstElementChild.classList.contains('ws-frozen') && document.getElementById('sidebar').lastElementChild.id === 'workspace-applets'));
    await page.locator('#ws-reorder-done').click();
    await page.waitForFunction(() => !document.querySelector('#sidebar .reorder-grip'));
    const afterDone = await bodies();
    ok('Done ends the mode and every section is open or closed as it was', JSON.stringify(afterDone.map(x => x.split(':')[1])) === JSON.stringify(openAtStart.map(x => x.split(':')[1])) && await page.evaluate(() => document.querySelector('.ws-reorder-bar').hidden && document.activeElement && document.activeElement.matches('.browser-group-toggle, #ws-actions-button')), JSON.stringify({ openAtStart, afterDone }));
    await enter();
    await page.keyboard.press('Escape');
    await page.waitForFunction(() => !document.querySelector('#sidebar .reorder-grip'));
    ok('Escape ends the mode too, leaving the order as it was', await is(['Pinned', 'Chats', 'Sources', 'In Progress'])());
    await page.reload(); await page.waitForTimeout(1200);
    ok('the order made in Reorder Sections is kept across a reload', await is(['Pinned', 'Chats', 'Sources', 'In Progress'])(), JSON.stringify(await order()));
  } finally { await t.close(); }
})();
