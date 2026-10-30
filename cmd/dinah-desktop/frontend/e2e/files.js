const { launch, PAGE } = require('./lib/page');
const { press, openMenu } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    state: { workspace: 'C:/Fixture', workspaceId: 'w1', workspaceRecords: [{ id: 'w1', name: 'Alpha Delivery', mountAlias: 'alpha', primarySourceId: 'local', sources: [{ id: 'local', name: 'Working Documents', mountAlias: 'documents', kind: 'local-folder', path: 'C:/Fixture' }] }], recent: ['C:/Fixture'] },
    app: () => {
      const item = (p, title, directory, kind = 'folder', source = '') => ({ path: p, title, directory, kind, actions: directory ? ['list'] : ['read', 'preview', 'open-external'], ...(source ? { ref: { workspaceId: 'w1', sourceId: source, path: p.split('/').slice(4).join('/') } } : {}) });
      const entries = [item('/', '/', true), item('/workspaces', 'Workspaces', true), item('/workspaces/alpha', 'Alpha Delivery', true), item('/workspaces/beta', 'Beta Delivery', true), item('/workspaces/alpha/documents', 'Working Documents', true, 'local-folder', 'local'), item('/workspaces/alpha/documents/readme.md', 'Read Me', false, 'file', 'local'), item('/workspaces/alpha/documents/slow.md', 'Slow Preview', false, 'file', 'local'), item('/workspaces/alpha/documents/script.ps1', 'Script', false, 'file', 'local'), item('/workspaces/alpha/documents/later.md', 'Later Page', false, 'file', 'local'), item('/workspaces/alpha/documents/paged', 'Paged Folder', true, 'folder', 'local'), item('/workspaces/alpha/documents/paged/end.md', 'End', false, 'file', 'local')];
      entries.find(x => x.path.endsWith('script.ps1')).actions = ['reveal'];
      // A Markdown file can be renamed in its folder.
      entries.find(x => x.path.endsWith('later.md')).actions.push('rename');
      window.__renameFile = (ref, name) => {
        __calls.push(['rename', ref.sourceId, ref.path, name]);
        if (entries.some(x => x.ref && x.ref.sourceId === ref.sourceId && x.ref.path === name)) throw Error('a file called ' + name + ' is already in the folder');
        const e = entries.find(x => x.ref && x.ref.sourceId === ref.sourceId && x.ref.path === ref.path);
        e.path = e.path.slice(0, e.path.lastIndexOf('/') + 1) + name; e.ref.path = name; e.title = name;
        return { ref: { ...e.ref }, name, drafts: [] };
      };
      // The local folder can be written, so it takes a new folder.
      entries.find(x => x.path === '/workspaces/alpha/documents').writable = true;
      window.__newFolder = (ref, name) => {
        __calls.push(['new-folder', ref.sourceId, ref.path, name]);
        const made = item('/workspaces/alpha/documents/' + name, name, true, 'directory', 'local');
        made.writable = true; entries.push(made);
        return JSON.parse(JSON.stringify(made));
      };
      const stat = p => { const result = entries.find(x => x.path === p); if (!result) throw Error('Resource unavailable'); const copy = JSON.parse(JSON.stringify(result)); if (localStorage.getItem('replaced-source') && copy.ref && copy.ref.sourceId === 'local') copy.ref.sourceId = 'replacement'; return copy; };
      // A drop as the browser delivers one from the system's file manager:
      // dragover, then drop, at a point, carrying the files' contents.
      window.__drop = (x, y, names) => {
        const el = document.elementFromPoint(x, y), dt = new DataTransfer();
        names.forEach(n => dt.items.add(new File(['contents of ' + n], n)));
        el.dispatchEvent(new DragEvent('dragover', { bubbles: true, cancelable: true, dataTransfer: dt }));
        const outlined = !!document.querySelector('.upload-drop-active');
        el.dispatchEvent(new DragEvent('drop', { bubbles: true, cancelable: true, dataTransfer: dt }));
        return outlined;
      };
      window.__renameMount = (path, title) => { entries.find(entry => entry.path === path).title = title; document.dispatchEvent(new CustomEvent('dinah:workspace-resource-labels')); };
      return {
        ReadUIState: async () => JSON.parse(localStorage.getItem('fixture-ui') || '{"version":1,"values":{}}'), SaveUIState: async value => localStorage.setItem('fixture-ui', JSON.stringify(value)),
        StatWorkspaceMount: async p => { __calls.push(['stat', p]); return stat(p); },
        StatWorkspaceMountExpected: async (p, expected) => { if (window.__holdVerify) await new Promise(resolve => { window.__finishVerify = resolve; }); const value = stat(p); if (!value.ref || value.ref.workspaceId !== expected.workspaceId || value.ref.sourceId !== expected.sourceId) throw Error('The workspace source was replaced'); return value; },
        ListWorkspaceMountsExpected: async (p, start, limit, expected) => { await window.go.main.App.StatWorkspaceMountExpected(p, expected); return window.go.main.App.ListWorkspaceMounts(p, start, limit); },
        ListWorkspaceMounts: async (p, start, limit) => {
          __calls.push(['list', p, start, limit]);
          if (window.__failList === p) throw Error('The source is offline');
          let children = entries.filter(x => x.path !== '/' && (x.path.slice(0, x.path.lastIndexOf('/')) || '/') === p).map(x => stat(x.path));
          if (p.endsWith('/paged')) return start === 0 ? { entries: [], nextStart: 1 } : { entries: children, nextStart: -1 };
          const size = p.endsWith('/documents') ? 4 : 100;
          return { entries: children.slice(start, start + size), nextStart: start + size < children.length ? start + size : -1 };
        },
        PreviewWorkspaceResource: async ref => {
          __calls.push(['preview', ref.path]);
          if (ref.path === 'slow.md') await new Promise(resolve => { __holdPreview[ref.path] = resolve; });
          const resource = entries.find(x => x.ref && x.ref.sourceId === ref.sourceId && x.ref.path === ref.path);
          if (window.__diagramTest && ref.path === 'slow.md') return {resource,blocks:[{line:1,html:'<div data-preview-diagram="late"><pre>late-files</pre></div>',diagrams:[{id:'late',language:'d2',source:'late-files',line:1}]}],problems:[]};
          if (window.__diagramTest && ref.path === 'later.md') return {resource,blocks:[{line:1,html:'<div data-preview-diagram="broken"><pre>broken-files</pre></div>',diagrams:[{id:'broken',language:'d2',source:'broken-files',line:1}]}],problems:[]};
          return { resource, blocks: [{ line: 1, html: '<h1>' + (resource ? resource.title : 'Preview') + '</h1><p>Reviewed preview content</p><img data-attachment="diagram.png" alt="Resource Diagram"><img data-attachment="missing.png">' }, ...(ref.path === 'readme.md' ? [{line:2,html:'<div data-preview-diagram="d2"><pre>Read -> Review</pre></div><div data-preview-diagram="mermaid"><pre>flowchart TD</pre></div>',diagrams:[{id:'d2',language:'d2',source:'Read -> Review',line:2},{id:'mermaid',language:'mermaid',source:'flowchart TD\n Read --> Review',line:3}]}] : [])], problems: [] };
        },
        RenderD2: async source => {if(source==='late-files')await new Promise(resolve=>{window.__finishDiagram=resolve;});if(source==='broken-files')throw Error('Files diagram syntax error');return '<svg xmlns="http://www.w3.org/2000/svg" width="150" height="50"><text y="25">D2 Preview</text></svg>';},
        CreateWorkspaceFolder: async (ref, name) => window.__newFolder(ref, name),
        RenameDocumentFile: async (ref, name) => window.__renameFile(ref, name),
        ChooseUploadFiles: async () => ['C:/Users/p/Downloads/notes.md', 'C:/Users/p/Downloads/Tender.docx'],
        UploadConflicts: async (ref, paths) => paths.filter(p => p.endsWith('notes.md')).map(p => p.split('/').pop()),
        UploadNameConflicts: async (ref, names) => names.filter(n => n === 'notes.md'),
        UploadToWorkspaceFolder: async (ref, paths, existing) => { __calls.push(['upload', ref.path, paths.join('|'), existing]); return { added: ['Tender.docx'], replaced: [], kept: existing === 'keep-both' ? ['notes (2).md'] : [], skipped: existing === 'skip' ? ['notes.md'] : [], refused: [] }; },
        WorkspaceLocalPath: async ref => 'C:/Fixture/' + ref.path,
        OpenWorkspaceMount: async p => { __calls.push(['open-external', p]); }, OpenWorkspaceResource: async ref => { __calls.push(['open-external-ref', ref]); }
      };
    },
    init: () => { window.__calls = []; window.__holdPreview = {}; },
    goto: false,
  });
  const { page, ok } = t;
  try {
    // These routes are tried before the fixture's, which serves the page's
    // files.
    await t.context.route('http://dinah.test/resource-attachment?*', route => {
      const name = new URL(route.request().url()).searchParams.get('name');
      return name === 'missing.png' ? route.fulfill({ status: 404, body: 'Unavailable' }) : route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="90" height="60"><rect width="90" height="60" fill="teal"/></svg>' });
    });
    // Files dropped from the system's file manager are posted to /upload as
    // a multipart form; the test keeps each body and answers as Go would.
    const uploads = [];
    await t.context.route('http://dinah.test/upload', route => {
      uploads.push(route.request().postData() || '');
      return route.fulfill({ contentType: 'application/json', body: JSON.stringify({ added: ['Tender.docx'], replaced: [], kept: [], skipped: [], refused: [] }) });
    });
    await page.goto(PAGE);
    await t.waitReady();
    await page.evaluate(() => dinahResources.openFiles('/workspaces'));
    let win = page.locator('.win[data-win-kind=files]');
    await win.locator('tr[data-name=alpha]').waitFor();
    ok('Files lists the workspace catalog without reading descendants', await win.locator('tr[data-name=beta]').count() === 1 && await page.evaluate(() => !__calls.some(c => c[0] === 'list' && c[1].split('/').length > 3)));
    ok('New Folder is off where nothing can be written', await win.locator('[data-command="new-folder"]:disabled').waitFor({ state: 'attached', timeout: 3000 }).then(() => true, () => false));
    await win.locator('tr[data-name=alpha]').dblclick();
    await win.locator('tr[data-name=documents]').waitFor();
    await win.locator('tr[data-name=documents]').dblclick();
    await win.locator('tr[data-name="readme.md"]').waitFor();
    // New Folder… makes a folder in a folder on the computer and selects it.
    await press(page, win, 'new-folder');
    const folderDialog = page.locator('dialog.dinah-new-folder[open]');
    await folderDialog.getByLabel('Folder Name').fill('Drafts');
    await folderDialog.getByRole('button', { name: 'Create' }).click();
    await folderDialog.waitFor({ state: 'detached' });
    await win.locator('.fm-status').getByText('Made the folder Drafts').waitFor();
    ok('New Folder makes the folder where Files is and says so', await page.evaluate(() => __calls.some(c => c[0] === 'new-folder' && c[1] === 'local' && c[2] === '' && c[3] === 'Drafts')) && (await win.locator('.fm-status').textContent()).includes('Made the folder Drafts'));
    // Upload Files… copies chosen files into the folder shown; a name the
    // folder has is asked about once, with Replace, Keep Both and Skip.
    await press(page, win, 'upload');
    const askDialog = page.locator('dialog#ask[open]');
    await askDialog.waitFor();
    ok('a name the folder already has is asked about with Cancel, Skip, Keep Both and Replace', (await askDialog.locator('.dialog-actions button').allTextContents()).join('|') === 'Cancel|Skip|Keep Both|Replace' && (await askDialog.locator('#ask-title').textContent()) === 'Replace notes.md?');
    await askDialog.getByRole('button', { name: 'Keep Both' }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'upload'));
    ok('Keep Both uploads the files into the folder shown, keeping both', await page.evaluate(() => __calls.some(c => c[0] === 'upload' && c[1] === '' && c[2] === 'C:/Users/p/Downloads/notes.md|C:/Users/p/Downloads/Tender.docx' && c[3] === 'keep-both')));
    // Files dropped onto the listing go into the folder shown, sent with
    // their contents; the target is outlined while they are over it, and a
    // drop outside a target uploads nothing.
    const target = await win.locator('[data-fb="main"]').boundingBox();
    const outlined = await page.evaluate(([x, y]) => window.__drop(x, y, ['Tender.docx']), [target.x + 20, target.y + target.height - 20]);
    await page.waitForFunction(() => document.querySelector('.toast') && /uploaded/.test(document.body.innerText), null, { timeout: 5000 }).catch(() => {});
    const sent = uploads[0] || '';
    ok('files dropped onto the listing are uploaded into the folder shown', uploads.length === 1 && outlined &&
      sent.includes('name="folder"') && sent.includes('"sourceId":"local"') && sent.includes('name="existing"\r\n\r\nskip') && sent.includes('filename="Tender.docx"') && sent.includes('contents of Tender.docx'));
    await page.evaluate(() => window.__drop(2, 2, ['Tender.docx']));
    await page.waitForTimeout(300);
    ok('a drop outside a folder uploads nothing', uploads.length === 1);
    ok('folders show display labels while retaining canonical alias paths', (await win.locator('.fm-path').textContent()).includes('Working Documents') && await win.locator('.fm-path [aria-current="location"]').getAttribute('title') === '/workspaces/alpha/documents' && await win.getByRole('button', { name: 'Load More', exact: true }).last().isVisible());
    await win.locator('tr[data-name="readme.md"]').click();
    await win.locator('.fm-preview h1').waitFor();
    ok('single selection previews without opening a document', await win.locator('.fm-preview h1').textContent() === 'Read Me' && await page.locator('.win[data-win-kind=workspace-resource]').count() === 0);
    await page.evaluate(() => __renameMount('/workspaces/alpha', 'Renamed Delivery'));
    await win.locator('.fm-path a[data-path="/workspaces/alpha"]').filter({hasText:'Renamed Delivery'}).waitFor();
    ok('workspace rename updates tree and breadcrumb live without losing selection', await win.locator('[data-fb="tree"] a[data-path="/workspaces/alpha"] .fm-ti-label').textContent() === 'Renamed Delivery' && await win.locator('tr[data-name="readme.md"][aria-selected=true]').count() === 1);
    await page.evaluate(() => __renameMount('/workspaces/alpha', 'Alpha Delivery'));
    await win.locator('.fm-preview img[data-attachment="diagram.png"]').evaluate(image => image.decode());
    await win.locator('.fm-preview .dinah-broken-image').waitFor();
    await win.locator('.fm-preview [data-doc-problems-btn]').click();
    ok('Files hosts the resource viewer including attachment problems and controls', await win.locator('.fm-preview img[data-attachment="diagram.png"]').evaluate(image => image.naturalWidth === 90 && new URL(image.src).searchParams.get('path') === 'readme.md') && (await win.locator('.fm-preview [data-doc-problems]').textContent()).includes('missing.png') && await win.locator('.fm-preview [data-embedded-kind="workspace-resource"]').count() === 1);
    ok('embedded viewer preserves the Files window title', await win.locator('.win-title').textContent() === 'Files');
    await openMenu(page, win, 'files-actions');
    await win.getByRole('menuitemcheckbox', { name: 'Show the Preview Pane', exact: true }).click();
    ok('preview pane can be hidden', await win.locator('.fm-preview').isHidden());
    await openMenu(page, win, 'files-actions');
    await win.getByRole('menuitemcheckbox', { name: 'Show the Preview Pane', exact: true }).click();
    await win.locator('.fm-preview h1').waitFor();
    await win.getByRole('button', { name: 'Expand Image: Resource Diagram', exact: true }).last().click();
    await page.locator('#media-viewer [data-media-stage] img').waitFor();
    ok('Files attachment previews open in the shared lightbox', await page.locator('#media-viewer [data-media-stage] img').evaluate(image => image.naturalWidth === 90));
    await page.keyboard.press('Escape');
    await win.locator('.fm-preview .msg-diagram svg').nth(1).waitFor();
    ok('Files previews render D2 and Mermaid with source retained',await win.locator('.fm-preview .msg-diagram details').count()===2&&(await win.locator('.fm-preview').textContent()).includes('D2 Preview'));
    await win.getByRole('button',{name:'Expand D2 Diagram',exact:true}).click();
    await page.locator('#media-viewer [data-media-stage] svg').waitFor();
    ok('Files diagrams use the shared vector viewer',(await page.locator('#media-viewer [data-media-stage] svg').textContent()).includes('D2 Preview'));
    await page.keyboard.press('Escape');
    await page.evaluate(() => pudlWindows.snap(dinahWorkbench.key({ kind: 'files', path: '/workspaces' }), 'maximized'));
    await t.shot('files-desktop');
    await win.locator('tr[data-name="slow.md"]').click();
    await page.waitForFunction(() => typeof __holdPreview['slow.md'] === 'function');
    await win.locator('tr[data-name="readme.md"]').click();
    await win.locator('.fm-preview h1').waitFor();
    await page.evaluate(() => __holdPreview['slow.md']());
    ok('a late preview cannot replace a newer selection', await win.locator('.fm-preview h1').textContent() === 'Read Me');
    await page.evaluate(()=>{window.__diagramTest=true;delete __holdPreview['slow.md'];});
    await win.locator('tr[data-name="slow.md"]').click();
    await page.waitForFunction(()=>typeof __holdPreview['slow.md']==='function');
    await page.evaluate(()=>__holdPreview['slow.md']());
    await page.waitForFunction(()=>typeof __finishDiagram==='function');
    await win.locator('tr[data-name="readme.md"]').click();
    await win.locator('.fm-preview h1').waitFor();
    await page.evaluate(()=>__finishDiagram());
    ok('a late diagram cannot replace the next Files selection',await win.locator('.fm-preview h1').textContent()==='Read Me'&&!(await win.locator('.fm-preview').textContent()).includes('late-files'));
    await win.locator('tr[data-name="later.md"]').click();
    await win.locator('.fm-preview .msg-diagram-error').waitFor();
    ok('Files diagram errors retain source and an actionable explanation',(await win.locator('.fm-preview .msg-diagram-error').textContent()).includes('Files diagram syntax error')&&await win.locator('.fm-preview pre').textContent()==='broken-files');
    await win.locator('tr[data-name="readme.md"]').click();
    await win.locator('.fm-preview h1').waitFor();
    await win.locator('tr[data-name="readme.md"]').press('Enter');
    await page.locator('.win[data-win-kind=workspace-resource] [data-doc-page] h1').waitFor();
    ok('Enter opens the existing resource document applet', await page.locator('.win[data-win-kind=workspace-resource]').count() === 1);
    await page.evaluate(() => pudlWindows.raise(dinahWorkbench.key({ kind: 'files', path: '/workspaces' })));
    await win.locator('tr[data-name="script.ps1"]').click();
    await openMenu(page, win, 'files-actions');
    await win.getByRole('button', { name: 'Show in Folder', exact: true }).waitFor();
    // Upload Files… belongs to the folder shown, not to the selected file.
    ok('unknown executable resources offer only safe reveal and no mutation commands', await win.getByRole('button', { name: 'Show in Folder', exact: true }).isVisible() && await win.locator('.dinah-files .menu-panel').getByRole('button', { name: /Delete|Rename|New File|Run|Execute/ }).count() === 0);
    await win.getByRole('button', { name: 'Show in Folder', exact: true }).click();
    ok('external commands preserve the resource identity', await page.evaluate(() => __calls.some(c => c[0] === 'open-external-ref' && c[1].workspaceId === 'w1' && c[1].sourceId === 'local' && c[1].path === 'script.ps1')));
    // Rename… renames a file whose actions allow it, in its own folder: the
    // name is offered with the extension unselected, a name the folder has
    // is refused in the dialog, and the file stays selected under its new
    // name.
    await win.locator('tr[data-name="later.md"]').click();
    await openMenu(page, win, 'files-actions');
    await win.getByRole('button', { name: /^Rename…/ }).click();
    const renameDialog = page.locator('dialog.dinah-ask-name[open]');
    const nameField = renameDialog.getByLabel('Name');
    await nameField.waitFor();
    ok('Rename… offers the file name with only the part before the extension selected', await nameField.inputValue() === 'later.md' && await nameField.evaluate(el => el.selectionStart === 0 && el.selectionEnd === 5) && await renameDialog.locator('.dialog-title').textContent() === 'Rename the File');
    await nameField.fill('readme.md');
    await renameDialog.getByRole('button', { name: 'Rename', exact: true }).click();
    await renameDialog.locator('.form-error').filter({ hasText: 'already in the folder' }).waitFor();
    ok('a name the folder already has is refused in the dialog, which stays open', await renameDialog.isVisible() && await nameField.getAttribute('aria-invalid') !== null);
    await nameField.fill('later notes.md');
    await renameDialog.getByRole('button', { name: 'Rename', exact: true }).click();
    await renameDialog.waitFor({ state: 'detached' });
    await win.locator('tr[data-name="later notes.md"][aria-selected=true]').waitFor();
    ok('Rename… renames the file and keeps it selected under its new name', await page.evaluate(() => __calls.some(c => c[0] === 'rename' && c[1] === 'local' && c[2] === 'later.md' && c[3] === 'later notes.md')) && await win.locator('tr[data-name="later.md"]').count() === 0);
    await page.evaluate(() => window.__renameFile({ workspaceId: 'w1', sourceId: 'local', path: 'later notes.md' }, 'later.md'));
    await win.locator('[data-action=more]').click();
    await win.locator('tr[data-name=paged]').waitFor();
    await win.locator('tr[data-name=paged]').dblclick();
    await win.locator('[data-action=more]').waitFor();
    await win.locator('[data-action=more]').click();
    await win.locator('tr[data-name="end.md"]').waitFor();
    ok('empty pages with a next cursor can still load more', true);
    await win.getByRole('button', { name: 'Up One Folder', exact: true }).click();
    await win.locator('tr[data-name="readme.md"]').waitFor();
    await win.locator('tr[data-name="readme.md"]').click();
    await page.evaluate(() => dinahUIState.flush());
    await page.reload();
    win = page.locator('.win[data-win-kind=files]');
    await win.locator('tr[data-name="readme.md"][aria-selected=true]').waitFor();
    ok('reload restores folder and selection through the native UI state bridge', true);
    await page.setViewportSize({ width: 600, height: 850 });
    await page.evaluate(() => { const key = dinahWorkbench.key({ kind: 'files', path: '/workspaces' }); pudlWindows.raise(key); pudlWindows.snap(key, 'maximized'); });
    await win.locator('.fm-preview h1').waitFor();
    ok('narrow layout retains the selected list and bounds horizontal content', await win.locator('.fm-tree-pane').isHidden() && await win.locator('.dinah-files').evaluate(el => el.scrollWidth <= el.clientWidth + 1));
    await t.shot('files-narrow');
    await win.locator('tr[data-name="slow.md"]').click();
    await page.waitForFunction(() => typeof __holdPreview['slow.md'] === 'function');
    await page.evaluate(() => { pudlWindows.close(dinahWorkbench.key({ kind: 'files', path: '/workspaces' })); __holdPreview['slow.md'](); });
    ok('closing during a preview destroys its visible window', await win.count() === 0);
    await page.evaluate(() => { localStorage.setItem('replaced-source', 'yes'); __calls.length = 0; return dinahResources.openFiles('/workspaces'); });
    await win.locator('.fm-status-error').waitFor();
    ok('saved identity refuses an alias reused by a replacement source', (await win.locator('.fm-status').textContent()).includes('different workspace source') && await page.evaluate(() => !__calls.some(c => c[0] === 'list' && c[1] === '/workspaces/alpha/documents')));
    await page.evaluate(() => { pudlWindows.close(dinahWorkbench.key({ kind: 'files', path: '/workspaces' })); localStorage.removeItem('replaced-source'); return dinahResources.openFiles('/workspaces/alpha/documents/paged'); });
    const paged = page.locator('.win[data-win-kind=files]');
    await paged.locator('[data-action=more]').click();
    await paged.locator('tr[data-name="end.md"]').click();
    await paged.locator('.fm-preview h1').waitFor();
    await openMenu(page, paged, 'files-actions');
    await paged.getByRole('menuitemcheckbox', { name: 'Show the Preview Pane', exact: true }).click();
    await page.evaluate(() => dinahUIState.flush());
    await page.reload();
    await page.locator('.win[data-win-kind=files] tr[data-name="end.md"][aria-selected=true]').waitFor();
    ok('a selection and hidden preview survive restart', await page.locator('.win[data-win-kind=files] .fm-preview').isHidden());
    await page.evaluate(() => { document.querySelectorAll('.win[data-win-kind=files]').forEach(win => pudlWindows.close(win.dataset.win)); });
    await page.evaluate(() => dinahResources.openFiles('/workspaces/alpha/documents'));
    await page.locator('.win[data-win-kind=files] tr[data-name="readme.md"]').waitFor();
    await page.locator('.win[data-win-kind=files] tr[data-name="readme.md"]').click();
    ok('new Files locations inherit the last hidden preview preference', await page.locator('.win[data-win-kind=files] .fm-preview').isHidden());
    await page.evaluate(() => { pudlWindows.close(dinahWorkbench.key({kind:'files',path:'/workspaces/alpha/documents'})); });
    await page.evaluate(() => { window.__failList = '/workspaces/beta'; return dinahResources.openFiles('/workspaces/beta'); });
    const offline = page.locator('.win[data-win-kind=files]');
    await offline.locator('.fm-status-error').waitFor();
    ok('an unavailable source remains an explicit error', (await offline.locator('.fm-status').textContent()).includes('offline'));
    await page.evaluate(() => { window.__failList = ''; });
    await openMenu(page, offline, 'files-actions');
    await offline.getByRole('button', { name: 'Retry', exact: true }).click();
    await page.waitForFunction(() => document.querySelector('.dinah-files .fm-path').textContent.includes('Beta Delivery'));
    ok('Retry recovers without changing the selected workspace', await page.evaluate(async () => (await go.main.App.State()).workspaceId === 'w1'));
    ok('ancestor refresh preserves retained old identities and rejects their use', await page.evaluate(async () => {
      const tree = dinahResources.createTree(), parent = await tree.lookup('/workspaces/alpha');
      await tree.load(parent); const old = tree.resolve(null, '/workspaces/alpha/documents');
      await tree.load(old); const child = tree.resolve(null, '/workspaces/alpha/documents/readme.md');
      localStorage.setItem('replaced-source', 'yes'); await tree.load(parent, false, true);
      const replacement = tree.resolve(null, old.path); let rejected = false;
      try { await tree.load(old); } catch (_) { rejected = true; }
      localStorage.removeItem('replaced-source'); tree.destroy();
      return old !== replacement && old.ref.sourceId === 'local' && child.ref.sourceId === 'local' && replacement.ref.sourceId === 'replacement' && rejected;
    }));
    const beforeWindows = await page.locator('.win[data-win-kind=workspace-resource]').count();
    await page.evaluate(async () => {
      const resource = await go.main.App.StatWorkspaceMount('/workspaces/alpha/documents/later.md');
      window.__currentOpen = true; window.__holdVerify = true;
      window.__openDone = dinahResources.open(resource, () => window.__currentOpen);
    });
    await page.waitForFunction(() => typeof __finishVerify === 'function');
    await page.evaluate(async () => { window.__currentOpen = false; window.__holdVerify = false; __finishVerify(); await __openDone; });
    ok('a canceled open cannot create a document after delayed verification', await page.locator('.win[data-win-kind=workspace-resource]').count() === beforeWindows);
    ok('New Terminal Here serializes the chosen resource identity', await page.evaluate(async () => {
      const original = dinahWorkbench.open; let address;
      dinahWorkbench.open = async item => { address = JSON.parse(item.path); };
      try { await dinahResources.openTerminal('/workspaces/alpha/documents', { workspaceId: 'w1', sourceId: 'local', path: '' }); }
      finally { dinahWorkbench.open = original; }
      return address[0] === '/workspaces/alpha/documents' && address[2].sourceId === 'local';
    }));
  } finally { await t.close(); }
})();
