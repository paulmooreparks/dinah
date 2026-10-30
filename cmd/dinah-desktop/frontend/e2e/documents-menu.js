// The Documents menu: its rows
// and keys from the command registry, the current document they act on,
// Recent and Show All…, command search, Keyboard Shortcuts, the Window
// menu's keys, and the Editor's own window setting.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 2560, height: 1200 },
    app: () => {
      window.__calls = [];
      let n = 0;
      const rev = () => 'r' + (++n);
      const drafts = {};
      const view = d => Object.assign({}, d, { spans: {}, suggestions: [], changed: d.text !== d.baseline.text,
        writeBack: d.origin.kind === 'new' ? '' : 'save',
        actions: ['update', 'discard', 'save-as'] });
      const get = id => { const d = drafts[id]; if (!d) throw new Error('the draft no longer exists'); return d; };
      window.__draft = id => drafts[id];
      window.__drafts = drafts;
      // Twelve recent documents, newest first, two of them with one title.
      window.__recent = Array.from({ length: 12 }, (_, i) => ({
        ref: { workspaceId: 'w1', sourceId: 'local', path: 'doc' + i + '.md' },
        kind: 'local-folder', title: i === 4 || i === 5 ? 'Notes' : 'Document ' + i,
        workspaceName: i === 5 ? 'Other' : 'Fixture', opened: new Date(Date.now() - i * 3600e3).toISOString() }));
      window.__windowModes = { editor: 'maximized' };
      const state = () => ({ workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'], documentViews: { 'text/markdown': 'split' }, catalog: [], defaults: {}, pinned: [], titles: {}, templates: [],
        newWindowMode: 'floating', windowModes: Object.assign({}, window.__windowModes), windowModesShipped: { editor: 'maximized' },
        workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [
          { id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] });
      return {
        State: async () => state(),
        RecentDocuments: async () => { window.__calls.push(['recent']); return window.__recent; },
        SetWindowKindMode: async (kind, mode) => { window.__calls.push(['window-mode', kind, mode]); if (mode) window.__windowModes[kind] = mode; else delete window.__windowModes[kind]; },
        OpenDraft: async ref => {
          window.__calls.push(['open', ref.sourceId, ref.path]);
          let d = Object.values(drafts).find(x => x.origin.sourceId === ref.sourceId && x.origin.path === ref.path);
          if (!d) {
            const text = '# Notes\n';
            const id = 'd' + Object.keys(drafts).length.toString().padStart(31, '0');
            d = drafts[id] = { id, origin: { kind: 'local-folder', workspaceId: ref.workspaceId, sourceId: ref.sourceId, path: ref.path },
              baseline: { text, revision: 'b1', title: ref.path }, title: ref.path, text, revision: rev() };
          }
          return view(d);
        },
        OpenComputerFile: async (ws, file) => { window.__calls.push(['computer-open', ws, file]); return window.go.main.App.OpenDraft({ workspaceId: 'w1', sourceId: 'local', path: 'computer.md' }); },
        GetDraft: async id => view(get(id)),
        // As the app does, only the drafts with something unsaved in them.
        ListDrafts: async () => Object.values(drafts).map(view).filter(v => v.origin.kind === 'new' ? v.text.trim() !== '' : v.changed || v.suggestions.length > 0),
        UpdateDraft: async (id, expected, text) => {
          const d = get(id);
          if (d.revision !== expected) throw new Error('stale');
          d.text = text; d.revision = rev();
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'updated' });
          return view(d);
        },
        SaveDraft: async (id, expected) => {
          const d = get(id);
          window.__calls.push(['save', d.origin.path]);
          if (window.__refuseSaves) throw new Error('the fixture refuses saves just now');
          d.baseline = { text: d.text, revision: 'b2', title: d.title };
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'saved' });
          return Object.assign(view(d), { notes: [] });
        },
        DiscardDraft: async id => { window.__calls.push(['discard', id]); delete drafts[id]; window.__emit('draft:changed', { id, kind: 'discarded' }); },
        PreviewDraft: async id => ({ revision: get(id).revision, problems: [], blocks: [{ line: 1, html: '<p>Preview</p>' }] }),
        // Page view, as the whole text in one block (page-view.js covers it).
        PageDocumentOf: async (id, text) => ({ type: 'doc', attrs: { front: '', tail: '' }, content: [{ type: 'source_block', attrs: { uid: 1, gap: '', src: text, kind: 'text', html: '<p>Preview</p>' } }] }),
        PageDocumentMarkdown: async doc => doc.content.map(b => (b.attrs.gap || '') + b.attrs.src).join('')
      };
    },
    ready: () => window.dinahEditor && window.dinahWorkspaceBrowser.records().length === 1 && document.querySelector('.menubar-ready'),
  });
  const { page: p, ok } = t;
  try {
    await p.waitForFunction(() => __calls.some(c => c[0] === 'recent'));

    const title = name => p.locator('nav[data-menubar] .menubar-title').filter({ hasText: new RegExp('^' + name + '$') }).first();
    const rows = () => p.evaluate(() => [...document.querySelectorAll('.menubar-panel .menu-action')].filter(e => e.offsetParent).map(e => ({
      label: (e.querySelector('.menubar-label') || e).textContent.replace(/Ctrl\+.*$|Alt\+.*$|F\d+$/, '').trim(),
      text: e.textContent, disabled: e.disabled || e.getAttribute('aria-disabled') === 'true' })));
    async function openMenu(name) { await p.keyboard.press('Escape'); await title(name).click(); await p.locator('.menubar-panel .menu-action').first().waitFor(); return rows(); }
    const row = (list, start) => list.find(r => r.text.startsWith(start));

    // The title stands right of the site menu.
    const titles = await p.locator('nav[data-menubar] .menubar-title').allTextContents();
    ok('Documents stands right of the site menu', titles[1] === 'Documents' && titles.indexOf('Documents') < titles.indexOf('View'));

    // Documents holds what is about no particular document; the rows that
    // act on one are in that document's Editor, under Document.
    let docs = await openMenu('Documents');
    ok('Documents holds New Document, Open and Recent, with their keys', ['New Document', 'Open…', 'Recent']
      .every((label, i, all) => { const at = docs.findIndex(r => r.text.startsWith(label)); return at >= 0 && (i === 0 || at > docs.findIndex(r => r.text.startsWith(all[i - 1]))); }) &&
      row(docs, 'Open…').text.includes('Ctrl+O') && !docs.some(r => /^(Save|Close Document|Discard Draft)/.test(r.text)), JSON.stringify(docs.map(r => r.text)));
    ok('with no Editor in front there is no Document menu', await title('Document').count() === 0);
    await t.shot('documents-menu-empty');
    const tools = await openMenu('Tools');
    ok('New Document and Open Document have left Tools', !tools.some(r => /New Document|Open Document/.test(r.text)));

    // Recent: ten rows, newest first, two of one title told apart, then Show All….
    docs = await openMenu('Documents');
    await p.locator('.menubar-panel .menu-action').filter({ hasText: /^Recent/ }).first().hover();
    await p.waitForFunction(() => { const last = [...document.querySelectorAll('.menubar-panel')].pop(); return last && last.textContent.includes('Show All…') && last.textContent.includes('Document 0'); });
    const recentRows = await p.evaluate(() => [...document.querySelectorAll('.menubar-panel')].pop().querySelectorAll('.menu-action').length);
    const recentText = await p.evaluate(() => [...document.querySelectorAll('.menubar-panel')].pop().textContent);
    ok('Recent shows ten documents and Show All…', recentRows === 11 && recentText.includes('Document 0') && !recentText.includes('Document 10') && recentText.includes('Show All…'));
    const notes = await p.evaluate(() => [...[...document.querySelectorAll('.menubar-panel')].pop().querySelectorAll('.menu-action')].map(e => e.textContent).filter(t => /^\d\. Notes/.test(t)));
    ok('two documents of one title are told apart by their workspace on the second line', notes.length === 2 && notes.some(t => t.includes('Fixture')) && notes.some(t => t.includes('Other')));
    ok('the first nine Recent rows are numbered, so the digit is their letter', /1\. /.test(recentText) && /9\. /.test(recentText) && !/10\. /.test(recentText));
    await t.shot('documents-menu-recent');
    await p.locator('.menubar-panel .menu-action').filter({ hasText: /^\d\. Document 1$/ }).click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'open' && c[2] === 'doc1.md'));
    const editor = p.locator('.win[data-win-kind="editor"]').first();
    await editor.locator('.cm-content').waitFor();
    ok('a Recent row opens its document in the Editor', true);
    ok('the Editor opens maximized while Open New Windows is Floating', await editor.getAttribute('data-win-mode') === 'maximized');

    // A redraw while the menu is open leaves its rows working.
    await openMenu('Documents');
    await p.evaluate(() => dinahCommands.changed());
    await p.waitForTimeout(150);
    await p.locator('.menubar-panel .menu-action').filter({ hasText: /^In Progress/ }).click();
    await p.locator('.in-progress-browser').waitFor();
    ok('a row still works after the rows changed while its menu was open', true);
    await p.evaluate(() => pudlWindows.raise([...document.querySelectorAll('.win[data-win-kind="editor"]')].pop().dataset.win));

    // The document has changes, so Ctrl+S would save it if a dialog did not
    // hold the key. The document would also save itself a moment after the
    // typing, which on a busy machine can land while the dialog is open, so
    // the fixture refuses that save: automatic saving stops, the document
    // stays changed, and only a save Ctrl+S asked for could follow.
    await p.evaluate(() => { window.__refuseSaves = true; });
    await editor.locator('.cm-content').click();
    await p.keyboard.press('Control+End');
    await p.keyboard.type(' Pending.');
    await p.waitForFunction(() => { const s = pudlWindows.state(); return document.querySelector('[data-win="' + CSS.escape(s.top) + '"] .win-title').textContent.includes('(changed)'); });
    await p.waitForFunction(() => __calls.some(c => c[0] === 'save' && c[1] === 'doc1.md'), null, { timeout: 30000 });
    await editor.getByText(/Automatic saving stopped/).first().waitFor({ state: 'attached', timeout: 30000 });
    const savesBeforeDialog = await p.evaluate(() => __calls.filter(c => c[0] === 'save').length);

    // Show All… lists all twelve; Escape closes it, and the arrow keys and
    // Enter open one.
    docs = await openMenu('Documents');
    await p.locator('.menubar-panel .menu-action').filter({ hasText: /^Recent/ }).first().hover();
    await p.waitForTimeout(300);
    await p.locator('.menubar-panel .menu-action').filter({ hasText: /^Show All…/ }).click();
    await p.locator('dialog.recent-documents[open]').waitFor();
    await p.keyboard.press('Escape');
    await p.waitForFunction(() => !document.querySelector('dialog.recent-documents'));
    ok('Escape closes Show All', true);
    docs = await openMenu('Documents');
    await p.locator('.menubar-panel .menu-action').filter({ hasText: /^Recent/ }).first().hover();
    await p.waitForTimeout(300);
    await p.locator('.menubar-panel .menu-action').filter({ hasText: /^Show All…/ }).click();
    await p.locator('dialog.recent-documents[open]').waitFor();
    ok('Show All lists every kept document with its workspace and time', await p.locator('dialog.recent-documents tbody tr').count() === 12 &&
      (await p.locator('dialog.recent-documents tbody tr').nth(5).textContent()).includes('Other'));
    await t.shot('documents-show-all');
    // A key does nothing while a dialog is open.
    await p.keyboard.press('Control+S');
    await p.keyboard.press('ArrowDown');
    await p.keyboard.press('ArrowDown');
    await p.keyboard.press('Enter');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'open' && c[2] === 'doc2.md'));
    ok('arrow keys and Enter open the chosen document and close the dialog', await p.locator('dialog.recent-documents').count() === 0);
    ok('Ctrl+S did nothing to a changed document while the dialog was open', await p.evaluate(n => __calls.filter(c => c[0] === 'save').length === n, savesBeforeDialog));
    await p.evaluate(() => { window.__refuseSaves = false; });

    // A local file: Save from the keyboard inside the text.
    const doc2 = p.locator('.win[data-win-kind="editor"]').filter({ has: p.locator('.cm-content') }).last();
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="editor"]').length === 2);
    const front = () => p.evaluate(() => { const s = pudlWindows.state(); return document.querySelector('[data-win="' + CSS.escape(s.top) + '"] .win-title').textContent; });
    ok('the newest document is in front', (await front()).startsWith('doc2.md'));
    await p.locator('.win[data-win-kind="editor"] .cm-content').last().click();
    await p.keyboard.press('Control+End');
    await p.keyboard.type(' Changed.');
    await p.waitForFunction(() => document.querySelector('.win[data-win-kind="editor"]:last-of-type') && Object.values(window.go).length);
    await p.waitForFunction(() => { const s = pudlWindows.state(); return document.querySelector('[data-win="' + CSS.escape(s.top) + '"] .win-title').textContent.includes('(changed)'); });
    docs = await openMenu('Document');
    ok('the Editor\'s Document menu has the document\'s rows in order with their keys', ['Save', 'Save As…', 'Rename…', 'Close Document', 'Discard Draft…']
      .every((label, i, all) => { const at = docs.findIndex(r => r.text.startsWith(label)); return at >= 0 && (i === 0 || at > docs.findIndex(r => r.text.startsWith(all[i - 1]))); }) &&
      row(docs, 'Save').text.includes('Ctrl+S') && row(docs, 'Save As…').text.includes('Ctrl+Shift+S'), JSON.stringify(docs.map(r => r.text)));
    ok('a changed file can be saved', !row(docs, 'Save').disabled && !row(docs, 'Save As…').disabled);
    // F2 is the Editor window's own key, so the menu row shows none.
    ok('a file can be renamed from the Document menu, whose row has no key', !row(docs, 'Rename…').disabled && !row(docs, 'Rename…').text.includes('F2'));
    await p.keyboard.press('Escape');
    await p.locator('.win[data-win-kind="editor"] .cm-content').last().click();
    await p.keyboard.press('Control+S');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'save' && c[1] === 'doc2.md'));
    ok('Ctrl+S in the text saves the document through its Document menu', true);

    // A document's keys act on its own window only: with the Workspace in
    // front there is no Document menu, and Ctrl+S saves nothing behind it.
    // The file saves itself after typing, so the count is taken once it has.
    const savedBefore = await p.evaluate(() => __calls.filter(c => c[0] === 'save').length);
    await p.locator('.win[data-win-kind="editor"] .cm-content').last().click();
    await p.keyboard.type(' Again.');
    await p.waitForFunction(n => __calls.filter(c => c[0] === 'save').length > n, savedBefore, { timeout: 8000 });
    await p.waitForTimeout(500);
    await p.evaluate(() => dinahWorkbench.open('workspace'));
    await p.waitForFunction(() => pudlWindows.state().top === 'workspace');
    await title('Document').waitFor({state:'detached'});
    ok('with the Workspace in front there is no Document menu', await title('Document').count() === 0);
    const saves = await p.evaluate(() => __calls.filter(c => c[0] === 'save').length);
    await p.locator('#workspace-select, .win[data-win="workspace"] select').first().focus().catch(() => {});
    await p.keyboard.press('Control+S');
    await p.waitForTimeout(300);
    ok('Ctrl+S with the Workspace in front saves no document behind it', await p.evaluate(n => __calls.filter(c => c[0] === 'save').length === n, saves));

    // A third document, for the keys that go from one document to the next.
    await p.evaluate(() => dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'third.md' }));
    await p.waitForFunction(() => { const s = pudlWindows.state(); return document.querySelector('[data-win="' + CSS.escape(s.top) + '"] .win-title').textContent.startsWith('third.md'); });

    // Window: Close has Ctrl+F4, and Ctrl+F6 goes to the next document.
    const windowRows = await openMenu('Window');
    ok('Window lists Close with Ctrl+F4 and the document keys', row(windowRows, 'Close').text.includes('Ctrl+F4') && row(windowRows, 'Next Document').text.includes('Ctrl+F6') &&
      windowRows.some(r => r.text.includes('Alt+1')));
    await p.keyboard.press('Escape');
    const before = await front();
    await p.locator('body').click({ position: { x: 5, y: 880 } }).catch(() => {});
    await p.keyboard.press('Control+F6');
    await p.waitForFunction(b => { const s = pudlWindows.state(); return document.querySelector('[data-win="' + CSS.escape(s.top) + '"] .win-title').textContent !== b; }, before);
    ok('Ctrl+F6 brings the next document to the front', (await p.evaluate(() => document.querySelector('[data-win="' + CSS.escape(pudlWindows.state().top) + '"]').dataset.winKind)) === 'editor');

    // The other stage-1 keys reach their commands.
    await p.evaluate(() => {
      dinahEditor.openDocument = () => { __calls.push(['open-dialog']); return Promise.resolve(); };
      dinahFilePicker.pick = options => { __calls.push(['pick', options.mode]); return Promise.resolve(false); };
    });
    await (async () => { const t = p.locator('[data-win="' + (await p.evaluate(() => pudlWindows.state().top)) + '"] .cm-content'); await t.click(); })();
    await p.keyboard.press('Control+O');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'open-dialog'));
    ok('Ctrl+O opens Open…', true);
    await p.keyboard.press('Control+Shift+S');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'pick' && c[1] === 'save'));
    ok('Ctrl+Shift+S opens Save As…', true);
    const firstWindow = await p.evaluate(() => dinahOpenWindows()[0].key);
    await p.keyboard.press('Alt+1');
    await p.waitForFunction(k => pudlWindows.state().top === k, firstWindow);
    ok('Alt+1 brings the first window to the front', true);
    await p.keyboard.press('Control+J');
    await p.waitForFunction(() => pudlWindows.state().top === 'chat');
    ok('Ctrl+J goes to the chat', true);
    await p.locator('body').click({ position: { x: 700, y: 880 } }).catch(() => {});
    await p.keyboard.press('F9');
    await p.waitForFunction(() => pudlWindows.state().top === 'needs-you');
    ok('F9 with no question waiting goes to Notifications', true);
    ok('with Notifications in front there is no current document', await p.evaluate(() => dinahDocuments.current() === null));
    await p.evaluate(() => { const w = [...document.querySelectorAll('.win[data-win-kind="editor"]')].pop(); pudlWindows.raise(w.dataset.win); });
    const closing = await p.locator('.win[data-win-kind="editor"]').count();
    await p.keyboard.press('Control+F4');
    await p.waitForFunction(n => document.querySelectorAll('.win[data-win-kind="editor"]').length < n, closing);
    ok('Ctrl+F4 closes the front window', true);

    // F1 lists the keys, from inside the text too.
    const frontText = async () => p.locator('[data-win="' + (await p.evaluate(() => pudlWindows.state().top)) + '"] .cm-content');
    await p.evaluate(() => { const w = [...document.querySelectorAll('.win[data-win-kind="editor"]')].pop(); pudlWindows.raise(w.dataset.win); });
    // A second chat window declares the chat's keys again, which must not
    // list them twice.
    await p.evaluate(() => dinahCommands.addHelp('Chat', [{ label: 'Stop the Turn', key: 'Esc' }]));
    await p.evaluate(() => dinahCommands.addHelp('Chat', [{ label: 'Stop the Turn', key: 'Esc' }]));
    await (await frontText()).click();
    await p.keyboard.press('F1');
    await p.locator('dialog.keyboard-help[open]').waitFor();
    const keys = await p.locator('dialog.keyboard-help tbody').textContent();
    ok('F1 in the text opens Keyboard Shortcuts, listing the registry\'s keys', keys.includes('Ctrl+S') && keys.includes('Ctrl+F4') && keys.includes('Alt+1 to Alt+9') && keys.includes('Ctrl+Shift+P'));
    await p.locator('dialog.keyboard-help #keyboard-help-for').selectOption('Chat');
    ok('a group of keys declared twice is listed once', await p.locator('dialog.keyboard-help tbody tr').filter({ hasText: 'Stop the Turn' }).count() === 1);
    await t.shot('documents-keyboard-help');
    await p.locator('dialog.keyboard-help [data-close]').click();

    // Command search: Ctrl+Shift+P, and ">" typed in Find and Open.
    await p.keyboard.press('Control+Shift+P');
    await p.locator('#go-to[open]').waitFor();
    await p.locator('#go-to-filter').fill('discard');
    const commandList = await p.locator('#go-to-list').textContent();
    ok('Commands lists commands with their menu', (await p.locator('#go-to-title').textContent()) === 'Commands' && /Discard Draft/.test(commandList) && commandList.includes('Document'), commandList);
    await p.locator('#go-to').getByRole('button', { name: 'Close', exact: true }).click();
    await p.keyboard.press('Control+P');
    await p.locator('#go-to[open]').waitFor();
    ok('Find and Open lists the most recent document first', (await p.locator('#go-to-list tr').first().textContent()).trim().startsWith('Document 0'));
    await p.locator('#go-to-filter').fill('>save as');
    ok('">" in Find and Open lists commands', (await p.locator('#go-to-list').textContent()).includes('Save As…'));
    await p.locator('#go-to-filter').fill('');
    await p.keyboard.press('Enter');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'open' && c[2] === 'doc0.md'));
    ok('Ctrl+P then Enter opens the last document', true);

    // A Recent change told by Go is read again; a failure is shown.
    const reads = await p.evaluate(() => __calls.filter(c => c[0] === 'recent').length);
    await p.evaluate(() => __emit('documents:recent', { error: 'The document opened, but it could not be added to Recent: locked.' }));
    await p.waitForFunction(n => __calls.filter(c => c[0] === 'recent').length > n, reads);
    ok('a failure to record an open is shown', (await p.locator('.toast').allTextContents()).some(t => t.includes('could not be added to Recent')));

    // Close Document and Discard Draft… act on the front Editor only.
    const editors = await p.locator('.win[data-win-kind="editor"]').count();
    await p.evaluate(() => dinahCommands.run('close-document'));
    await p.waitForFunction(n => document.querySelectorAll('.win[data-win-kind="editor"]').length < n, editors);
    ok('Close Document closes the front Editor', true);
    await p.evaluate(() => dinahWorkbench.open('workspace'));
    await p.waitForFunction(() => pudlWindows.state().top === 'workspace');
    ok('Discard Draft… does not reach an Editor behind the front window', !(await p.evaluate(() => dinahCommands.run('discard-draft'))));

    // Settings: the Editor's own row under Open New Windows, with Reset.
    await p.evaluate(() => { const b = document.createElement('button'); b.dataset.cmd = 'settings'; document.body.append(b); b.click(); b.remove(); });
    await p.locator('#window-kind-modes [data-kind="editor"]').waitFor();
    const editorRow = p.locator('#window-kind-modes [data-kind="editor"]');
    ok('Settings shows Editor Windows under Open New Windows, at Maximized, with Reset off', (await editorRow.locator('.form-label').textContent()) === 'Editor Windows' &&
      await editorRow.locator('[role="radio"][data-value="maximized"]').getAttribute('aria-checked') === 'true' && await editorRow.locator('[data-reset]').isDisabled());
    await editorRow.locator('[role="radio"][data-value="same"]').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'window-mode' && c[2] === 'same'));
    await p.waitForFunction(() => !document.querySelector('#window-kind-modes [data-reset]').disabled);
    ok('Like Other Windows is saved and Reset comes on', true);
    await p.evaluate(() => dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'floating.md' }));
    await p.waitForFunction(() => [...document.querySelectorAll('.win[data-win-kind="editor"]')].some(w => w.querySelector('[aria-label="Text of floating.md"]')));
    ok('an Editor set to Like Other Windows opens floating', await p.evaluate(() => [...document.querySelectorAll('.win[data-win-kind="editor"]')].find(w => w.querySelector('[aria-label="Text of floating.md"]')).dataset.winMode === 'floating'));
    await p.evaluate(() => { const b = document.createElement('button'); b.dataset.cmd = 'settings'; document.body.append(b); b.click(); b.remove(); });
    await t.shot('documents-window-setting');
    await editorRow.locator('[data-reset]').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'window-mode' && c[2] === ''));
    await p.waitForFunction(() => document.querySelector('#window-kind-modes [data-reset]').disabled);
    ok('Reset returns the Editor to the choice it ships with', await editorRow.locator('[role="radio"][data-value="maximized"]').getAttribute('aria-checked') === 'true');

    // The Editor's own menus keep Edit; File is gone.
    await p.evaluate(() => { const w = document.querySelector('.win[data-win-kind="editor"]'); if (w) pudlWindows.raise(w.dataset.win); });
    await p.waitForTimeout(300);
    await (await frontText()).click();
    await p.waitForTimeout(300);
    const allTitles = await p.locator('nav[data-menubar] .menubar-title').allTextContents();
    ok('the Editor has no File menu', allTitles.includes('Edit') && !allTitles.includes('File'));

    // Documents › In Progress opens the pane: drafts and changed documents,
    // a group to a workspace. A document with nothing unsaved in it, and a
    // new one with nothing in it, are not listed.
    await p.keyboard.press('Escape');
    await p.evaluate(() => {
      Object.keys(__drafts).forEach(k => delete __drafts[k]);
      const at = h => new Date(Date.now() - h * 3600e3).toISOString();
      const d = (id, title, origin, text, base, updated, extra) => { __drafts[id] = Object.assign({ id, title, origin, text, baseline: { text: base, revision: 'b1', title }, revision: 'r-' + id, updated }, extra || {}); };
      d('a'.repeat(32), 'Note 09 Design', { kind: 'local-folder', workspaceId: 'w1', sourceId: 'local', path: 'design/note-09.md' }, '# Changed\n', '# Old\n', at(1));
      d('b'.repeat(32), 'Release Plan', { kind: 'local-folder', workspaceId: 'w1', sourceId: 'local', path: 'release-plan.md' }, '# Same\n', '# Same\n', at(0));
      d('c'.repeat(32), 'Meeting Notes', { kind: 'new', workspaceId: 'w1', sourceId: '', path: '' }, '# Notes\n', '', at(2));
      d('e'.repeat(32), 'Untitled', { kind: 'new', workspaceId: 'w1', sourceId: '', path: '' }, '', '', at(2));
    });
    await p.evaluate(() => dinahCommands.run('in-progress'));
    const pane = p.locator('.in-progress-browser');
    // The app says when a draft changes; the fixture changed one.
    await p.evaluate(() => __emit('draft:changed', { id: 'a'.repeat(32), kind: 'updated' }));
    await pane.locator('.browser-row').nth(1).waitFor();
    const paneRows = () => pane.evaluate(el => [...el.querySelectorAll('.in-progress-kind .browser-group-title, .browser-row')].map(n => n.classList.contains('browser-row') ? n.querySelector('.browser-row-title').textContent + ' | ' + [...n.querySelectorAll('.md-meta')].map(m => m.textContent).join(' | ') : '# ' + n.textContent));
    let listed = await paneRows();
    ok('In Progress lists the draft and the changed document under their headings, and nothing else', listed.length === 4 && listed[0] === '# Drafts' && listed[1].startsWith('Meeting Notes | Not saved anywhere yet') &&
      listed[2] === '# Changed Documents' && listed[3].startsWith('Note 09 Design | design/note-09.md | Changes not saved'), JSON.stringify(listed));
    ok('the group is the workspace', (await pane.locator('.browser-group:not(.in-progress-kind) > .browser-group-head .browser-group-title').allTextContents()).join() === 'Fixture');
    // Each kind is a group of its own, which closes and opens again.
    const drafts = pane.locator('.in-progress-kind').filter({ hasText: 'Drafts' });
    await drafts.locator('.browser-group-toggle').first().click();
    ok('the Drafts group closes', await drafts.locator('.browser-row').isHidden());
    await drafts.locator('.browser-group-toggle').first().click();
    ok('and opens again', await drafts.locator('.browser-row').isVisible());
    ok('the Workspace pane\'s kinds are groups too', await p.locator('#workspace-in-progress .in-progress-kind').count() === 2);
    ok('the Workspace pane has the same two for this workspace', await p.locator('#workspace-in-progress .browser-row').count() === 2 && await p.locator('#workspace-in-progress-empty').isHidden());
    await pane.locator('input[type="search"]').fill('note 09');
    // Search queues a render; wait for its result before reading the list.
    await p.waitForFunction(() => {
      const rows = document.querySelectorAll('.in-progress-browser .browser-row');
      return rows.length === 1 && rows[0].querySelector('.browser-row-title').textContent === 'Note 09 Design';
    });
    listed = await paneRows();
    ok('the search narrows the list', listed.length === 2 && !listed.some(r => r.startsWith('Meeting Notes')), JSON.stringify(listed));
    await t.shot('in-progress');
    await pane.locator('input[type="search"]').fill('');
    // A row's menu, from its PUDL row actions button, shown once the pointer is on the row.
    const menuOf = async title => {
      const row = pane.locator('.browser-row').filter({ hasText: title });
      await row.locator('a').hover(); await row.locator('[data-row-actions]').click();
      await p.locator('.context-menu').waitFor();
    };
    await menuOf('Meeting Notes');
    await p.locator('.context-menu').getByRole('menuitem', { name: 'Discard Draft…' }).click();
    await p.locator('#ask[open]').waitFor();
    ok('Discard Draft… says the draft was never saved anywhere', (await p.locator('#ask-body').textContent()).includes('never saved anywhere'));
    await p.locator('#ask-yes').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'discard' && c[1] === 'c'.repeat(32)));
    await pane.locator('.browser-row').filter({ hasText: 'Meeting Notes' }).waitFor({ state: 'detached' });
    ok('Discard Draft… asks, discards and lists the rest', true);
    await menuOf('Note 09 Design');
    ok('a changed document\'s menu has Open and Discard Changes…', (await p.locator('.context-menu .menu-action').allTextContents()).join('|') === 'Open|Discard Changes…');
    await p.keyboard.press('Escape');
    await pane.locator('.browser-row').filter({ hasText: 'Note 09 Design' }).locator('a').click();
    await p.waitForFunction(() => [...document.querySelectorAll('.win[data-win-kind="editor"] .win-title')].some(t => t.textContent.startsWith('Note 09 Design')));
    ok('a row opens its document in the Editor', true);
  } catch (err) {
    t.errors.push(String(err && err.stack || err));
  } finally {
    await t.close();
  }
})();
