// The Editor: drafts kept as the person types, changes made elsewhere,
// suggestions, New Document and Save As, Open, saving without a press, and
// restoring the window.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 3840, height: 1200 },
    app: () => {
      window.__calls = [];
      let n = 0;
      const rev = () => 'r' + (++n);
      // Drafts survive a reload, as the draft service's store does.
      const load = () => JSON.parse(localStorage.getItem('fixture-drafts') || '{}');
      const drafts = load();
      const keep = () => localStorage.setItem('fixture-drafts', JSON.stringify(drafts));
      // Suggestions as the draft service keeps them: placed by their text.
      const utf16 = s => s.length;
      const spans = d => { const out = {}; (d.suggestions || []).forEach(s => { const at = d.text.indexOf(s.before + s.suffix); if (at >= 0 && !s.stale) out[s.id] = [utf16(d.text.slice(0, at)), utf16(d.text.slice(0, at + s.before.length))]; }); return out; };
      window.__suggest = (id, author, changes, quiet) => {
        const d = drafts[id], group = 'g' + (++n);
        d.suggestions = (d.suggestions || []).concat(changes.map((c, i) => ({ id: group + 's' + i, group, author, reason: 'the review asked for it', before: c[0], after: c[1], suffix: '' })));
        keep();
        // Another program changes the stored draft without this app's events.
        if (!quiet) window.__emit('draft:changed', { id, revision: d.revision, kind: 'suggested' });
      };
      const view = d => Object.assign({}, d, { spans: spans(d), suggestions: d.suggestions || [] }, {
        changed: d.text !== d.baseline.text,
        writeBack: d.origin.kind === 'new' ? '' : 'save',
        actions: ['update', 'discard', 'save-as'].concat(d.text !== d.baseline.text && d.origin.kind !== 'new' ? ['save'] : [])
      });
      const get = id => { const d = drafts[id]; if (!d) throw new Error('the draft no longer exists'); return d; };
      window.__draft = id => drafts[id];
      window.__change = (id, text) => { const d = get(id); d.text = text; d.revision = rev(); keep(); window.__emit('draft:changed', { id, revision: d.revision, kind: 'updated' }); };
      // A change another program makes, which raises no event here.
      window.__silentChange = (id, text) => { const d = get(id); d.text = text; d.revision = rev(); keep(); };
      const state = () => ({ workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'], documentViews: { 'text/markdown': 'split' }, windowModes: { editor: 'maximized' }, windowModesShipped: { editor: 'maximized' }, catalog: [], defaults: {}, pinned: [], titles: {}, templates: [],
        workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [
          { id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] });
      const entry = (path, item, children) => ({ item: Object.assign({ path, title: path.slice(path.lastIndexOf('/') + 1) || '/', actions: [] }, item), children });
      const local = p => ({ workspaceId: 'w1', sourceId: 'local', path: p });
      const mounts = () => {
        const m = {};
        [entry('/', { kind: 'directory', directory: true, actions: ['list'] }, ['/home', '/workspaces']),
          entry('/home', { kind: 'catalog', directory: true, actions: ['list'] }, ['/home/dinah']),
          entry('/home/dinah', { kind: 'directory', directory: true, actions: ['list'], writable: true, ref: { workspaceId: '', sourceId: 'dinah-home', path: '' } }, []),
          entry('/workspaces', { kind: 'catalog', directory: true, actions: ['list'] }, ['/workspaces/Fixture']),
          entry('/workspaces/Fixture', { kind: 'workspace', title: 'Fixture', directory: true, actions: ['list'], ref: { workspaceId: 'w1' } }, ['/workspaces/Fixture/Fixture']),
          entry('/workspaces/Fixture/Fixture', { kind: 'local-folder', title: 'Fixture', directory: true, actions: ['list'], writable: true, ref: local('') }, ['/workspaces/Fixture/Fixture/plan.md', '/workspaces/Fixture/Fixture/notes.md', '/workspaces/Fixture/Fixture/taken.md', '/workspaces/Fixture/Fixture/bound.md', '/workspaces/Fixture/Fixture/logo.png']),
          entry('/workspaces/Fixture/Fixture/plan.md', { kind: 'file', actions: ['read', 'preview', 'edit'], writable: true, size: 8, ref: local('plan.md') }),
          entry('/workspaces/Fixture/Fixture/notes.md', { kind: 'file', actions: ['read', 'preview', 'edit'], writable: true, size: 8, ref: local('notes.md') }),
          entry('/workspaces/Fixture/Fixture/taken.md', { kind: 'file', actions: ['read', 'preview', 'edit'], writable: true, size: 8, ref: local('taken.md') }),
          entry('/workspaces/Fixture/Fixture/bound.md', { kind: 'file', actions: ['read', 'preview', 'edit'], writable: true, size: 8, ref: local('bound.md') }),
          entry('/workspaces/Fixture/Fixture/logo.png', { kind: 'file', actions: ['open-external'], writable: true, size: 800, ref: local('logo.png') })
        ].forEach(x => { m[x.item.path] = x; });
        // Folders made with New Folder… in the workspace's local folder.
        (window.__folders || []).forEach(name => {
          const path = '/workspaces/Fixture/Fixture/' + name;
          m[path] = entry(path, { kind: 'directory', directory: true, actions: ['list'], writable: true, ref: local(name) }, []);
          m['/workspaces/Fixture/Fixture'].children = m['/workspaces/Fixture/Fixture'].children.concat([path]);
        });
        // Files saved with Save As into those folders.
        Object.values(JSON.parse(localStorage.getItem('fixture-drafts') || '{}')).forEach(d => {
          if (d.origin.kind !== 'local-folder' || d.origin.path.indexOf('/') < 0) return;
          const path = '/workspaces/Fixture/Fixture/' + d.origin.path, dir = path.slice(0, path.lastIndexOf('/'));
          if (!m[dir] || m[path]) return;
          m[path] = entry(path, { kind: 'file', actions: ['read', 'preview', 'edit'], writable: true, size: 8, ref: local(d.origin.path) });
          m[dir].children = m[dir].children.concat([path]);
        });
        return m;
      };
      return {
        State: async () => state(),
        // The file system the Open and Save As dialog shows: the workspace's
        // folder, and the home folder.
        HomeMountPath: async () => '/home/dinah',
        CreateWorkspaceFolder: async (ref, name) => {
          window.__calls.push(['new-folder', ref.path, name]);
          if (ref.sourceId !== 'local' || ref.path) throw new Error('a folder can be made only in a folder on the computer');
          if ((window.__folders || []).includes(name) || name === 'notes.md') throw new Error(name + ' already exists here; choose another name');
          (window.__folders ||= []).push(name);
          return { ref: local(name), title: name, kind: 'directory', directory: true, actions: ['list'], writable: true };
        },
        StatWorkspaceMount: async path => { const m = mounts()[path]; if (!m) throw new Error('no such file or folder'); return m.item; },
        StatWorkspaceMountExpected: async path => window.go.main.App.StatWorkspaceMount(path),
        ResolveWorkspaceResourceMount: async ref => { const m = Object.values(mounts()).find(x => x.item.ref && x.item.ref.workspaceId === ref.workspaceId && (x.item.ref.sourceId || '') === (ref.sourceId || '') && (x.item.ref.path || '') === (ref.path || '')); if (!m) throw new Error('not mounted'); return m.item; },
        ListWorkspaceMounts: async path => { const m = mounts()[path]; if (!m) throw new Error('no such folder'); return { entries: (m.children || []).map(c => mounts()[c].item), nextStart: -1 }; },
        ListWorkspaceMountsExpected: async path => window.go.main.App.ListWorkspaceMounts(path),
        OpenComputerDocument: async ws => { window.__calls.push(['computer-open', ws]); return window.go.main.App.OpenDraft({ workspaceId: 'w1', sourceId: 'local', path: 'bound.md' }); },
        OpenComputerFile: async (ws, path) => { window.__calls.push(['computer-open', ws, path]); return window.go.main.App.OpenDraft({ workspaceId: 'w1', sourceId: 'local', path: 'bound.md' }); },
        ChooseComputerSavePath: async (ws, name) => { window.__calls.push(['computer-choose', name]); return 'C:/Out/' + name; },
        SaveDraftToComputer: async (id, path, replace) => { window.__calls.push(['computer-save', path, replace]); if (!replace) throw new Error('a file with that name already exists'); },
        OpenDraft: async ref => {
          let d = Object.values(drafts).find(x => x.origin.sourceId === ref.sourceId && x.origin.path === ref.path);
          if (!d) {
            const text = ref.path === 'plan.md' ? '# Release Plan\n\nThe plan as written.\n' : '# Notes\n', title = ref.path.slice(ref.path.lastIndexOf('/') + 1);
            const id = 'd' + Object.keys(drafts).length.toString().padStart(31, '0');
            d = drafts[id] = { id, origin: { kind: 'local-folder', workspaceId: ref.workspaceId, sourceId: ref.sourceId, path: ref.path },
              baseline: { text, revision: 'b1', title }, title, text, revision: rev() };
            keep();
          }
          return view(d);
        },
        GetDraft: async id => { window.__getDrafts = (window.__getDrafts || 0) + 1; return view(get(id)); },
        DraftStamp: async id => { const d = get(id); window.__stamps = (window.__stamps || 0) + 1; return d.revision + '|' + ((d.baseline && d.baseline.revision) || '') + '|' + (d.suggestions || []).map(s => s.id).join(','); },
        ListDrafts: async () => Object.values(drafts).map(view),
        AcceptSuggestion: async (id, expected, sid, all) => {
          const d = get(id), s = d.suggestions.find(x => x.id === sid);
          window.__calls.push(['accept', sid, all]);
          if (d.revision !== expected) throw new Error('stale');
          const take = all ? d.suggestions.filter(x => x.group === s.group) : [s];
          take.forEach(x => { d.text = d.text.replace(x.before, x.after); });
          d.suggestions = d.suggestions.filter(x => !take.includes(x));
          d.revision = rev(); keep();
          return view(d);
        },
        ReviseSuggestion: async (id, sid, after) => {
          // A new object, as Go's answer is, not the one the page holds.
          const d = get(id);
          window.__calls.push(['revise', sid, after]);
          d.suggestions = d.suggestions.map(x => x.id === sid && x.after !== after ? Object.assign({}, x, { after, edited: true }) : x);
          keep();
          return view(d);
        },
        RejectSuggestion: async (id, sid, all) => {
          const d = get(id), s = d.suggestions.find(x => x.id === sid);
          window.__calls.push(['reject', sid, all]);
          d.suggestions = d.suggestions.filter(x => all ? x.group !== s.group : x.id !== sid); keep();
          return view(d);
        },
        UpdateDraft: async (id, expected, text) => {
          const d = get(id);
          window.__calls.push(['update', text]);
          if (d.revision !== expected) throw new Error('the draft was changed in another window since this one read it; reload it before changing it again');
          d.text = text; d.revision = rev(); keep();
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'updated' });
          return view(d);
        },
        // Page view, as the whole text in one block (page-view.js covers it).
        PageDocumentOf: async (id, text) => ({ type: 'doc', attrs: { front: '', tail: '' }, content: [{ type: 'source_block', attrs: { uid: 1, gap: '', src: text, kind: 'text', html: '<p>' + text.replace(/</g, '&lt;') + '</p>' } }] }),
        PageDocumentMarkdown: async doc => doc.content.map(b => (b.attrs.gap || '') + b.attrs.src).join(''),
        PreviewDraft: async id => {
          const d = get(id);
          // An image paragraph is drawn as the preview draws an attachment.
          const line = l => { const m = /^!\[\]\(<?([^)>]+)>?\)$/.exec(l.trim()); return m ? '<img class="dinah-attachment-image" data-attachment="' + m[1] + '" alt=""/>' : l.replace(/</g, '&lt;'); };
          return { revision: d.revision, problems: [], blocks: d.text.split('\n\n').map((part, i) => ({ line: i * 2 + 1, html: '<p>' + part.split('\n').map(line).join('\n') + '</p>' })) };
        },
        CompareDraft: async id => {
          const d = get(id), remote = window.__remote;
          return { moved: !!remote, revision: '9', title: d.title, theirs: remote ? '@@ -1 +1 @@\n+' + remote.added : '', mine: '', conflicts: [],
            merged: remote ? d.text + '\n' + remote.added + '\n' : d.text };
        },
        MergeIntoDraft: async (id, expected) => {
          const d = get(id), remote = window.__remote;
          window.__calls.push(['merge']);
          if (d.revision !== expected) throw new Error('stale');
          d.text = d.text + '\n' + remote.added + '\n'; d.baseline = { text: '# Release Plan\n\n' + remote.added + '\n', revision: '9', title: d.title };
          d.revision = rev(); window.__remote = null; keep();
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'updated' });
          return view(d);
        },
        SaveDraft: async (id, expected) => {
          const d = get(id);
          window.__calls.push(['save', d.text]);
          if (window.__remote) throw new Error('the document was changed elsewhere since it was opened; compare or reload before saving');
          if (d.revision !== expected) throw new Error('stale');
          d.baseline = { text: d.text, revision: String(+d.baseline.revision + 1 || 'b2'), title: d.title }; keep();
          window.__emit('draft:changed', { id, revision: d.revision, kind: 'saved' });
          return Object.assign(view(d), { notes: [] });
        },
        SaveDraftAs: async (id, expected, ref) => {
          const d = get(id);
          window.__calls.push(['save-as', ref.sourceId, ref.path]);
          if (d.revision !== expected) throw new Error('stale');
          if (ref.path === 'taken.md') throw new Error('something already exists there; choose another name');
          d.origin = { kind: 'local-folder', workspaceId: ref.workspaceId, sourceId: ref.sourceId, path: ref.path };
          d.baseline = { text: d.text, revision: 'c1', title: d.title }; keep();
          return Object.assign(view(d), { notes: [] });
        },
        // Files kept with a draft: the native dialog picks a chart or a note,
        // and a pasted file keeps the name it is given.
        PickDraftFiles: async (id, images) => { window.__calls.push(['pick', images]); return images ? ['chart.png'] : ['notes v2.pdf']; },
        AddDraftFile: async (id, name, data) => { window.__calls.push(['add-file', name, data.length]); return name; },
        DiscardDraft: async (id) => { delete drafts[id]; keep(); window.__emit('draft:changed', { id, kind: 'discarded' }); },
        // Rename… on a file renames it, and its drafts follow it
        // (documents.Ops.Rename).
        RenameDocumentFile: async (ref, name) => {
          window.__calls.push(['rename-file', ref.path, name]);
          if (name === 'taken.md') throw new Error('a file called taken.md is already in the folder');
          const moved = Object.values(drafts).filter(d => d.origin.sourceId === ref.sourceId && d.origin.path === ref.path);
          moved.forEach(d => { if (d.title === d.baseline.title) d.title = name; d.baseline.title = name; d.origin.path = name; });
          keep();
          moved.forEach(d => window.__emit('draft:changed', { id: d.id, revision: d.revision, kind: 'renamed' }));
          return { ref: { ...ref, path: name }, name, drafts: moved.map(view) };
        },
        // New Document, as Go makes one: empty and untitled, in the workspace.
        NewEmptyDraft: async ws => {
          window.__calls.push(['new', ws]);
          const id = 'e' + Object.keys(drafts).length.toString().padStart(31, '0');
          const d = drafts[id] = { id, origin: { kind: 'new', workspaceId: ws }, baseline: { text: '', revision: '', title: '' }, title: 'Untitled', text: '', revision: rev() };
          keep();
          return view(d);
        },
        // The judgement agent's answer, held back while __namewait is set, is
        // the judge's name by default, then whatever a check puts there.
        SuggestDocumentName: async id => { window.__calls.push(['suggest-name', id]); if (window.__namewait) await window.__namewait; return window.__namesuggestion || { name: 'The Fixture Guide', used: 'judge', call: 7 }; },
        CorrectJudgement: async (id, value) => { window.__calls.push(['correct-judgement', id, value]); }
      };
    },
    ready: () => window.dinahEditor && window.dinahWorkspaceBrowser.records().length === 1,
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    // A draft's images, served as the app serves /draft-file: a 1x1 PNG.
    await t.context.route(/\/draft-file\?/, route => route.fulfill({ contentType: 'image/png', body: Buffer.from('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==', 'base64') }));
    await p.goto(PAGE);
    await t.waitReady();

    // A document in the workspace's folder opens as a draft, in Split view.
    await p.evaluate(() => window.dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'plan.md' }));
    const editor = p.locator('.win[data-win-kind="editor"]').first();
    await editor.locator('.cm-content').waitFor();
    await editor.locator('[data-ed-page] .doc-block').first().waitFor();
    ok('a document opens in Split view with text and preview', await editor.locator('[data-ed-panes]').getAttribute('data-view') === 'split' &&
      (await editor.locator('.cm-content').textContent()).includes('The plan as written.'));
    const focusedAtOnce = await editor.locator('.cm-editor.cm-focused').waitFor({ timeout: 3000 }).then(() => true, () => false);
    ok('a document opens with the cursor in its text, so typing starts at once', focusedAtOnce);
    // While the ribbon shows, the Formatting toolbar gives way to it, and
    // the Document toolbar stays (PUDL 0.80.0).
    const formatting = editor.locator('[data-ed-toolbar]');
    ok('the Formatting toolbar shows while the ribbon is off', await formatting.isVisible());
    await p.getByRole('button', { name: 'Show as ribbon' }).click();
    await p.waitForFunction(() => document.documentElement.hasAttribute('data-menubar-ribbon-on'));
    ok('while the ribbon shows, the Formatting toolbar is hidden and the Document toolbar stays', await formatting.isHidden() && await editor.locator('[data-ed-head]').isVisible());
    await p.getByRole('button', { name: 'Show as ribbon' }).click();
    await p.waitForFunction(() => !document.documentElement.hasAttribute('data-menubar-ribbon-on'));
    ok('turning the ribbon off brings the Formatting toolbar back', await formatting.isVisible());
    await editor.locator('.cm-content').click();
    ok('an unchanged document offers Save, switched off', await editor.locator('[data-ed-save]:disabled').filter({ hasText: /^Save$/ }).waitFor({ timeout: 3000 }).then(() => true, () => false));

    // Typing is kept in the draft after a pause.
    await editor.locator('.cm-content').click();
    await p.keyboard.press('Control+End');
    await p.keyboard.type('Added in the Editor.');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'update' && c[1].includes('Added in the Editor.')));
    await editor.locator('[data-ed-page]').getByText('Added in the Editor.').waitFor();
    ok('typing is written to the draft and the preview follows', await editor.locator('[data-ed-save]').isEnabled());
    const shownTitle = await p.evaluate(() => document.querySelector('.win[data-win-kind="editor"] .win-title').textContent);
    ok('the window title says the draft has changed', shownTitle.includes('(changed)'));
    const shownDetail = await p.evaluate(() => { const w = document.querySelector('.win[data-win-kind="editor"]'); const d = w.querySelector('.win-title-detail'); return { attr: w.getAttribute('data-win-detail'), text: d && !d.hidden ? d.textContent : '' }; });
    ok('the document\'s workspace is the window title\'s detail, after the title', shownDetail.attr === 'Fixture' && shownDetail.text === 'Fixture' && !shownTitle.includes('Fixture'), JSON.stringify(shownDetail) + ' ' + shownTitle);
    const gap = await p.evaluate(() => {
      const w = document.querySelector('.win[data-win-kind="editor"]'), t = w.querySelector('.win-title'), d = w.querySelector('.win-title-detail');
      const range = document.createRange(); range.selectNodeContents(t);
      return d.getBoundingClientRect().left - range.getBoundingClientRect().right;
    });
    ok('the detail follows the title with no gap', gap < 12, String(gap));
    // The document saves itself after the pause; the checks of saving
    // without a press are further on.
    await p.waitForFunction(() => __calls.some(c => c[0] === 'save' && c[1].includes('Added in the Editor.')));
    await p.waitForFunction(() => !document.querySelector('.win[data-win-kind="editor"] .win-title').textContent.includes('(changed)'));
    ok('once it has saved itself nothing is left to save', await editor.locator('[data-ed-save]').isDisabled());

    // A change made elsewhere is shown when the window holds nothing unsent.
    const id = await p.evaluate(() => Object.keys(JSON.parse(localStorage.getItem('fixture-drafts')))[0]);
    await p.evaluate(id => __change(id, '# Release Plan\n\nRewritten by an agent.\n'), id);
    await editor.locator('.cm-content').getByText('Rewritten by an agent.').waitFor();
    ok('an agent\'s change appears in the open Editor', (await editor.locator('[data-ed-notice]').isHidden()));
    // A change by another program raises no event; the window's check of
    // the draft's stamp finds it, and reads the whole draft only then.
    const reads = await p.evaluate(() => window.__getDrafts || 0);
    await p.evaluate(id => __silentChange(id, '# Release Plan\n\nChanged by the command line.\n'), id);
    await editor.locator('.cm-content').getByText('Changed by the command line.').waitFor({ timeout: 15000 });
    ok('a change no event told of is found by the stamp check, which reads the draft only when its stamp changed', await p.evaluate(r => (window.__stamps || 0) > 0 && window.__getDrafts - r <= 2, reads));

    // While the person has unsent changes, they choose.
    await editor.locator('.cm-content').click();
    await p.keyboard.press('Control+End');
    await p.keyboard.type(' Mine.');
    await p.evaluate(id => __change(id, '# Release Plan\n\nTheirs.\n'), id);
    await editor.locator('[data-ed-notice]').waitFor({ state: 'visible' });
    ok('a competing change asks the person which text to keep', await editor.locator('[data-ed-save]').isDisabled());
    await editor.locator('[data-ed-theirs]').click();
    await editor.locator('.cm-content').getByText('Theirs.').waitFor();
    ok('Take Theirs shows the other change and drops the unsent typing', !(await editor.locator('.cm-content').textContent()).includes('Mine.'));

    // A suggestion shows in the text where it applies, and changes the text
    // only when accepted.
    await p.evaluate(id => __suggest(id, 'Rework (chat s7)', [['Theirs.\n', 'Theirs, improved.\n']]), id);
    await editor.locator('.cm-sugg-ins').waitFor();
    ok('a suggestion strikes the old text and shows the new', (await editor.locator('.cm-sugg-del').textContent()) === 'Theirs.' &&
      (await editor.locator('.cm-sugg-ins').textContent()).startsWith('Theirs, improved.') &&
      (await editor.locator('.editor-proposal-text').textContent()).startsWith('Rework suggests one change'));
    ok('the suggestion has not changed the text', !(await p.evaluate(id => __draft(id).text, id)).includes('improved'));
    await t.shot('editor-suggestion');
    // Edit in the text: the suggested words go in a box in their place, and
    // Ctrl+Enter keeps the new wording in the suggestion.
    await editor.locator('.cm-sugg-btn[title="Edit"]').click();
    const textBox = editor.locator('.cm-sugg-ins textarea.sugg-edit-text');
    ok('Edit in the text puts the suggested words in a box', await textBox.inputValue() === 'Theirs, improved.' && await textBox.evaluate(el => el === document.activeElement));
    await textBox.fill('Theirs, as I would put it.');
    await p.keyboard.press('Control+Enter');
    await editor.locator('.cm-sugg-ins').filter({ hasText: 'Theirs, as I would put it.' }).waitFor();
    ok('the edit is kept in the suggestion with its line ending, and the text is unchanged', await p.evaluate(() => __calls.some(c => c[0] === 'revise' && c[2] === 'Theirs, as I would put it.\n')) &&
      !(await p.evaluate(id => __draft(id).text, id)).includes('as I would put it') && (await editor.locator('.cm-sugg-ins').getAttribute('title')).includes('edited by you'));
    await editor.locator('.cm-sugg-btn[title="Accept"]').click();
    await editor.locator('.cm-content').getByText('Theirs, as I would put it.').waitFor();
    await p.evaluate(id => { const d = __draft(id); d.text = d.text.replace('Theirs, as I would put it.', 'Theirs, improved.'); }, id);
    await editor.locator('.cm-content').evaluate(el => { const v = CM.EditorView.findFromDOM(el); const t = v.state.doc.toString(), at = t.indexOf('Theirs, as I would put it.'); if (at >= 0) v.dispatch({ changes: { from: at, to: at + 'Theirs, as I would put it.'.length, insert: 'Theirs, improved.' } }); });
    await editor.locator('.cm-content').getByText('Theirs, improved.').waitFor();
    ok('accepting puts the suggestion in the text and clears its marks', await editor.locator('.cm-sugg-ins').count() === 0 &&
      await editor.locator('[data-ed-proposals]').isHidden() && await p.evaluate(() => __calls.some(c => c[0] === 'accept' && !c[2])));
    await p.evaluate(id => __suggest(id, 'Draft (chat s7)', [['# Release Plan\n', '# Release Plan, Revised\n'], ['Theirs, improved.\n', 'Theirs, final.\n']]), id);
    await editor.locator('.editor-proposal').getByText('suggests 2 changes').waitFor();
    // Go To selects the proposal's first change in the text, and again the next.
    const selectedRange = () => editor.locator('.cm-content').evaluate(el => { const s = CM.EditorView.findFromDOM(el).state.selection.main; return [s.from, s.to]; });
    await editor.locator('.editor-proposal').getByRole('button', { name: 'Go To' }).click();
    const at1 = await selectedRange();
    await editor.locator('.editor-proposal').getByRole('button', { name: 'Go To' }).click();
    const at2 = await selectedRange();
    ok('Go To selects a proposal\'s changes in the text, one after another', at1[1] > at1[0] && at2[1] > at2[0] && at2[0] > at1[0], JSON.stringify([at1, at2]));
    await editor.locator('.cm-content').evaluate(el => { const v = CM.EditorView.findFromDOM(el); v.dispatch({ selection: { anchor: 0 } }); });
    await editor.locator('.editor-proposal').getByRole('button', { name: 'Reject All' }).click();
    await editor.locator('[data-ed-proposals]').waitFor({ state: 'hidden' });
    ok('Reject All removes a whole proposal and leaves the text', await editor.locator('.cm-sugg-ins').count() === 0 &&
      !(await editor.locator('.cm-content').textContent()).includes('final'));

    // A suggestion made by another program, such as the command line, raises
    // no event here; the open window finds it on its own.
    await p.evaluate(id => __suggest(id, 'Command line (chat s8)', [['# Release Plan\n', '# Release Plan, Again\n']], true), id);
    let found = true;
    await editor.locator('.cm-sugg-ins').first().waitFor({ timeout: 10000 }).catch(() => { found = false; });
    ok('a suggestion made by another program appears in an open window', found);
    if (found) {
      await editor.locator('.cm-sugg-btn[title="Reject"]').first().click();
      await editor.locator('[data-ed-proposals]').waitFor({ state: 'hidden' });
    }

    // The chat works with the document in front: its draft and the selected
    // paragraph, so the person can ask a chat to change it.
    await p.evaluate(() => { window.__contexts = []; document.addEventListener('dinah:item-context', e => window.__contexts.push(e.detail)); });
    await editor.locator('.cm-line', { hasText: 'Theirs, improved.' }).click({ clickCount: 3 });
    await p.waitForFunction(() => __contexts.some(c => c.selection === 'Theirs, improved.\n'));
    ok('the Editor tells the chat its draft and the selected paragraph', await p.evaluate(id => __contexts.some(c => c.draftId === id && c.selection === 'Theirs, improved.\n'), id) &&
      (await p.locator('.win[data-chat-current] [data-view-id=context-chip]').textContent()).endsWith(', selection'));
    // Clearing the chip keeps it cleared while the person goes on selecting,
    // until the Editor's window comes to the front again.
    await p.evaluate(() => document.querySelector('.win[data-chat-current] [data-view-id=context-clear]').click());
    await editor.locator('.cm-line', { hasText: 'Theirs, improved.' }).click();
    await editor.locator('.cm-line', { hasText: 'Theirs, improved.' }).click({ clickCount: 3 });
    await p.waitForTimeout(400);
    ok('a cleared chip stays cleared while the person selects', await p.locator('.win[data-chat-current] [data-view-id=context]').isHidden());
    await p.evaluate(() => window.pudlWindows.raise(document.querySelector('.win[data-chat-current]').dataset.win));
    await p.evaluate(key => window.pudlWindows.raise(key), await editor.getAttribute('data-win'));
    await p.waitForFunction(() => !document.querySelector('.win[data-chat-current] [data-view-id=context]').hidden);
    ok('bringing the Editor forward gives the chat its document again', true);

    // A change made to the file on disk while the window is open is told in
    // the window, which compares it and brings it in.
    await p.evaluate(() => { window.__remote = { added: 'Added on disk.' }; });
    await editor.locator('[data-ed-moved]').waitFor({ state: 'visible', timeout: 9000 });
    ok('the window says the file changed on disk and offers to bring the change in', (await editor.locator('[data-ed-moved-body]').textContent()).includes('changed on disk') &&
      await editor.locator('[data-ed-bring]').isVisible());
    await editor.locator('[data-ed-compare]').click();
    const compareDialog = p.locator('dialog.editor-compare[open]');
    await compareDialog.getByText('Added on disk.').waitFor();
    ok('Compare shows what changed elsewhere', await compareDialog.locator('.editor-diff .add').count() === 1);
    await t.shot('editor-compare');
    await compareDialog.getByRole('button', { name: 'Bring In Their Changes' }).click();
    await editor.locator('.cm-content').getByText('Added on disk.').waitFor();
    ok('bringing their changes in keeps the draft and adds theirs', (await editor.locator('.cm-content').textContent()).includes('Theirs, improved.') && await editor.locator('[data-ed-moved]').isHidden());

    // Views switch, and the window comes back as it was after a reload.
    await editor.locator('[data-command="view-text"]').click();
    ok('Text view hides the preview', await editor.locator('[data-ed-scroll]').isHidden());
    await t.shot('editor-text');
    await editor.locator('[data-command="view-split"]').click();
    await t.shot('editor-light');
    await p.evaluate(() => window.pudlSetTheme('dark'));
    await t.shot('editor-dark');
    await p.evaluate(() => window.pudlSetTheme('light'));
    await editor.locator('.cm-content').evaluate(el => {
      window.__appearanceEditor = CM.EditorView.findFromDOM(el);
      window.__appearanceText = __appearanceEditor.state.doc.toString();
      window.__appearanceSelection = [__appearanceEditor.state.selection.main.from, __appearanceEditor.state.selection.main.to];
      window.go.main.App.SaveAppearance = async state => ({ state, revision: 'editor-font-test' });
    });
    await p.evaluate(() => dinahAppearance.overrides({ editor: { font: { families: ['Consolas'], fallback: 'monospace' }, size: 18 } }));
    await p.waitForFunction(() => getComputedStyle(document.querySelector('.cm-editor')).fontSize === '18px');
    ok('font changes preserve the CodeMirror instance, document and selection', await editor.locator('.cm-content').evaluate(el => {
      const view = CM.EditorView.findFromDOM(el);
      return view === __appearanceEditor && view.state.doc.toString() === __appearanceText && JSON.stringify([view.state.selection.main.from, view.state.selection.main.to]) === JSON.stringify(__appearanceSelection);
    }));
    await p.evaluate(() => dinahAppearance.overrides({}));

    await editor.locator('[data-command="view-preview"]').click();
    await p.waitForTimeout(400);
    await p.goto(p.url());
    await p.locator('.win[data-win-kind="editor"] [data-ed-page] .doc-block').first().waitFor();
    ok('the Editor window is restored in the view it was left in', await p.locator('.win[data-win-kind="editor"] [data-ed-panes]').getAttribute('data-view') === 'preview');

    // New Document starts an empty, untitled document in the workspace, and
    // Save As places it.
    await p.evaluate(() => window.dinahEditor.newDocument());
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="editor"]').length === 2);
    ok('New Document starts an empty draft in the current workspace', await p.evaluate(() => __calls.some(c => c[0] === 'new' && c[1] === 'w1')));
    const fresh = p.locator('.win[data-win-kind="editor"]').nth(1);
    // A toolbar's words leave out a label's "…"; its tooltip keeps it.
    await fresh.locator('[data-ed-save]').filter({ hasText: /^Save As$/ }).waitFor();
    ok('a new document offers Save As in place of Save', await fresh.locator('[data-ed-save]').textContent() === 'Save As' && (await fresh.locator('[data-ed-save]').getAttribute('title')).startsWith('Save As…'));
    await fresh.locator('[data-ed-save]').click({ timeout: 20000 });
    // Save As shows the file system, starting at the workspace, which is
    // the list of what is mounted in it and takes no file: it is marked
    // read-only, the name field is disabled, and nothing is shown as an
    // error, since the person has done nothing wrong.
    const saveAs = p.locator('dialog.dinah-picker[open]');
    await saveAs.locator('tr.fm-row[data-name="Fixture"]').waitFor();
    await saveAs.locator('[data-pick="readonly"]').waitFor();
    ok('Save As starts at the workspace, marked read-only, with Save and the name disabled', (await saveAs.locator('[data-fb="crumbs"]').textContent()).includes('Fixture') &&
      await saveAs.locator('[data-fb="crumbs"] [aria-current="location"] .fm-readonly-mark').isVisible() &&
      (await saveAs.locator('[data-pick="readonly"]').textContent()).startsWith('Read-only.') && await saveAs.locator('[data-pick="readonly"] .fm-readonly-mark').isVisible() &&
      await saveAs.getByRole('button', { name: 'Save', exact: true }).isDisabled() && await saveAs.locator('[data-pick="input"]').isDisabled() &&
      await saveAs.locator('[data-pick="error"]').isHidden());
    ok('the rows that take no document carry the read-only mark, and a folder does not', await saveAs.locator('tr.fm-row[data-name="Fixture"] .fm-readonly-mark').count() === 0 &&
      await saveAs.locator('[data-fb="tree"] a[data-path="/home/dinah"] .fm-readonly-mark').count() === 0 &&
      await saveAs.locator('[data-fb="tree"] a[data-path="/workspaces"] .fm-readonly-mark').count() === 1);
    await t.shot('editor-save-as');
    ok('To Your Computer is the primary button at a read-only place', await saveAs.getByRole('button', { name: 'To Your Computer…' }).evaluate(el => el.classList.contains('btn-primary')));
    await saveAs.locator('tr.fm-row[data-name="Fixture"]').dblclick();
    await saveAs.locator('tr.fm-row[data-name="taken.md"]').waitFor();
    await p.waitForFunction(() => document.querySelector('[data-pick="input"]').value === 'The Fixture Guide.md');
    ok('a folder of the workspace takes a file name, and a box the person has not typed in holds the judge’s suggestion', (await p.evaluate(() => __calls.some(c => c[0] === 'suggest-name'))) && (await saveAs.getByLabel('File Name').inputValue()) === 'The Fixture Guide.md' &&
      await saveAs.getByRole('button', { name: 'Save', exact: true }).isEnabled(), await saveAs.getByLabel('File Name').inputValue());
    await saveAs.getByLabel('File Name').fill('taken.md');
    await saveAs.getByRole('button', { name: 'Save', exact: true }).click();
    await saveAs.locator('[data-pick="error"]').getByText('already exists').waitFor();
    ok('Save As refuses a name that is taken, and says so', await saveAs.isVisible() && !(await p.evaluate(() => __calls.some(c => c[0] === 'save-as'))));
    // New Folder… makes a folder in the folder shown and goes into it, so
    // the document can be saved in a folder that did not exist.
    ok('New Folder is offered in a folder on the computer', await saveAs.locator('[data-pick="folder"]').isEnabled());
    await saveAs.locator('[data-pick="folder"]').click();
    const folderDialog = p.locator('dialog.dinah-new-folder[open]');
    await folderDialog.getByLabel('Folder Name').fill('notes.md');
    await folderDialog.getByRole('button', { name: 'Create' }).click();
    await folderDialog.locator('.form-error').getByText('already exists').waitFor();
    ok('New Folder refuses a name that is taken and stays open', await folderDialog.isVisible());
    await folderDialog.getByLabel('Folder Name').fill('Note 12');
    await folderDialog.getByRole('button', { name: 'Create' }).click();
    await folderDialog.waitFor({ state: 'detached' });
    await p.waitForFunction(() => (document.querySelector('dialog.dinah-picker[open] [data-fb="crumbs"]') || {}).textContent.includes('Note 12'));
    ok('after New Folder, Save As shows the new folder, which takes the file', await saveAs.getByRole('button', { name: 'Save', exact: true }).isEnabled() && await saveAs.getByLabel('File Name').inputValue() === 'taken.md');
    await t.shot('editor-save-as-new-folder');
    await saveAs.getByLabel('File Name').fill('note-12.md');
    await saveAs.getByRole('button', { name: 'Save', exact: true }).click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'save-as' && c[2] === 'Note 12/note-12.md'));
    await fresh.locator('[data-ed-save]').getByText('Save', { exact: true }).waitFor();
    ok('after Save As the document saves to its file', (await fresh.locator('[data-ed-meta]').textContent()).includes('note-12.md'));
    ok('saving under another name than the suggestion records the choice with the call, and applies nothing', await p.evaluate(() => __calls.some(c => c[0] === 'correct-judgement' && c[1] === 7 && c[2] === 'note-12.md')), JSON.stringify(await p.evaluate(() => __calls.filter(c => c[0] === 'correct-judgement'))));

    // A name the person typed first is never replaced by the late answer.
    // (Each new window is found by how many there were, so a window still
    // closing under load cannot slip into the count.)
    let editorCount = await p.evaluate(() => document.querySelectorAll('.win[data-win-kind="editor"]').length);
    await p.evaluate(() => { window.__namewait = new Promise(r => { window.__namerelease = r; }); window.__namesuggestion = { name: 'A Late Suggestion', used: 'judge', call: 9 }; window.dinahEditor.newDocument(); });
    const fresh2 = p.locator('.win[data-win-kind="editor"]').nth(editorCount);
    await fresh2.waitFor({ state: 'attached' });
    await fresh2.locator('[data-ed-save]').filter({ hasText: /^Save As$/ }).waitFor();
    await fresh2.locator('[data-ed-save]').click();
    const saveAs2 = p.locator('dialog.dinah-picker[open]');
    await saveAs2.locator('tr.fm-row[data-name="Fixture"]').dblclick();
    await saveAs2.getByLabel('File Name').fill('my-own.md');
    await p.evaluate(() => window.__namerelease());
    await p.waitForTimeout(250);
    ok('a name the person typed first survives the late suggestion', (await saveAs2.getByLabel('File Name').inputValue()) === 'my-own.md', await saveAs2.getByLabel('File Name').inputValue());
    await saveAs2.locator('[data-pick="cancel"]').click();
    // The window goes before the next one opens, so counts stay exact.
    await p.evaluate(() => { const ws = document.querySelectorAll('.win[data-win-kind="editor"]'); window.pudlWindows.close(ws[ws.length - 1].dataset.win); });
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="editor"]').length === 2);
    // Neither probe draft stays: a window's close keeps the draft, and an
    // unsaved Untitled one would then be offered in Find and Open.
    await p.evaluate(async () => {
      const extra = Object.values(JSON.parse(localStorage.getItem('fixture-drafts') || '{}')).filter(d => d.title === 'Untitled' && d.origin.kind === 'new');
      for (const d of extra) await window.go.main.App.DiscardDraft(d.id);
    });
    // With no judge the rule answers Untitled: the box is as today, and no
    // call means no correction can be recorded.
    await p.evaluate(() => { window.__namesuggestion = { name: 'Untitled', used: 'rule', call: 0 }; window.dinahEditor.newDocument(); });
    const fresh3 = p.locator('.win[data-win-kind="editor"]').nth(2);
    await fresh3.waitFor({ state: 'attached' });
    await fresh3.locator('[data-ed-save]').filter({ hasText: /^Save As$/ }).waitFor();
    await fresh3.locator('[data-ed-save]').click();
    const saveAs3 = p.locator('dialog.dinah-picker[open]');
    await saveAs3.locator('tr.fm-row[data-name="Fixture"]').dblclick();
    await p.waitForFunction(() => __calls.filter(c => c[0] === 'suggest-name').length === 3);
    ok('the rule’s Untitled leaves the box as it was, and call 0 records nothing', (await saveAs3.getByLabel('File Name').inputValue()) === 'Untitled.md' && (await p.evaluate(() => __calls.filter(c => c[0] === 'correct-judgement').length)) === 1);
    await saveAs3.locator('[data-pick="cancel"]').click();
    await p.evaluate(() => { const ws = document.querySelectorAll('.win[data-win-kind="editor"]'); window.pudlWindows.close(ws[ws.length - 1].dataset.win); });
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="editor"]').length === 2);
    // Neither probe draft stays: a window's close keeps the draft, and an
    // unsaved Untitled one would then be offered in Find and Open.
    await p.evaluate(async () => {
      const extra = Object.values(JSON.parse(localStorage.getItem('fixture-drafts') || '{}')).filter(d => d.title === 'Untitled' && d.origin.kind === 'new');
      for (const d of extra) await window.go.main.App.DiscardDraft(d.id);
    });


    // In Split, a handle between the text and the preview resizes them, and
    // the text's share of the width is kept.
    await fresh.locator('[data-command="view-split"]').click();
    const splitHandle = fresh.locator('[data-ed-split-handle]');
    const textBefore = (await fresh.locator('[data-ed-text]').boundingBox()).width;
    await splitHandle.focus();
    await p.keyboard.press('Shift+ArrowRight');
    await p.waitForFunction(() => typeof dinahUIState.get('dinah-editor-text-share') === 'number');
    const share = await p.evaluate(() => dinahUIState.get('dinah-editor-text-share'));
    const textAfter = (await fresh.locator('[data-ed-text]').boundingBox()).width;
    const whole = (await fresh.locator('[data-ed-split]').boundingBox()).width;
    ok('the handle between the text and the preview widens the text, and its share is kept', await splitHandle.getAttribute('role') === 'separator' &&
      textAfter > textBefore && Math.abs(share - textAfter / whole) < 0.02);
    await fresh.locator('[data-command="view-text"]').click();
    ok('Text shows the text alone, with no handle', await splitHandle.isHidden() && await fresh.locator('[data-ed-scroll]').isHidden() && await fresh.locator('[data-ed-text]').isVisible());
    await fresh.locator('[data-command="view-split"]').click();

    // Drafts with work in them are offered in Find and Open once their
    // window is closed; closing keeps the draft.
    await p.keyboard.press('Escape');
    await p.evaluate(() => window.pudlWindows.close(document.querySelector('.win[data-win-kind="editor"]').dataset.win));
    await p.waitForFunction(() => document.querySelectorAll('.win[data-win-kind="editor"]').length === 1);
    await p.evaluate(id => __change(id, '# Release Plan\n\nChanged after the window closed.\n'), id);
    await p.evaluate(() => document.activeElement && document.activeElement.blur());
    await p.keyboard.press('Control+p');
    await p.locator('#go-to-filter').fill('plan');
    const row = p.locator('#go-to-list tr', { hasText: 'plan.md' });
    await row.waitFor();
    ok('Find and Open lists a draft that is not yet saved', (await row.textContent()).includes('Draft · not saved'));
    await p.locator('#go-to-filter').fill('Untitled');
    ok('a saved draft with nothing changed is not listed', await p.locator('#go-to-list tr', { hasText: 'Draft ·' }).count() === 0);
    await p.keyboard.press('Escape');

    await p.evaluate(() => { const d = document.getElementById('go-to'); if (d.open) d.close(); });
    // Open shows the file system too, listing only what the Editor opens.
    await p.evaluate(() => { dinahEditor.openDocument(); });
    const openDialog = p.locator('dialog.dinah-picker[open]');
    await openDialog.locator('tr.fm-row[data-name="Fixture"]').waitFor();
    await openDialog.locator('tr.fm-row[data-name="Fixture"]').dblclick();
    await openDialog.locator('tr.fm-row[data-name="notes.md"]').waitFor();
    ok('Open lists the documents in a folder and leaves out other files', await openDialog.locator('tr.fm-row[data-name="logo.png"]').count() === 0 &&
      await openDialog.getByRole('button', { name: 'From Your Computer…' }).isVisible());
    // The dialog resizes from its corner and the browser fills it; the size
    // it is left at is where the next one opens.
    const resizable = await openDialog.evaluate(d => getComputedStyle(d).resize === 'both');
    await openDialog.evaluate(d => { d.style.width = '1100px'; d.style.height = '760px'; });
    await p.waitForTimeout(300);
    const fills = await openDialog.evaluate(d => { const browse = d.querySelector('.fb-pick-browse').getBoundingClientRect(); return browse.height > 420; });
    ok('Open resizes from its corner, and the folders and list grow with it', resizable && fills);
    await openDialog.locator('tr.fm-row[data-name="notes.md"]').click();
    ok('choosing a file shows its path', await openDialog.getByLabel('Path').inputValue() === '/workspaces/Fixture/Fixture/notes.md');
    await t.shot('editor-open');
    await openDialog.getByRole('button', { name: 'Open', exact: true }).click();
    await p.waitForFunction(() => Object.values(JSON.parse(localStorage.getItem('fixture-drafts'))).some(d => d.origin.path === 'notes.md'));
    ok('the chosen file opens in the Editor', await openDialog.count() === 0);
    await p.evaluate(() => { dinahEditor.openDocument(); });
    await openDialog.getByLabel('Path').waitFor();
    ok('the next Open opens at the size the last was left at', await openDialog.evaluate(d => Math.abs(d.offsetWidth - 1100) < 2 && Math.abs(d.offsetHeight - 760) < 2));
    await openDialog.getByLabel('Path').fill('nothing-here.md');
    await openDialog.getByRole('button', { name: 'Open', exact: true }).click();
    await openDialog.locator('[data-pick="error"]').getByText('There is nothing at nothing-here.md.').waitFor();
    ok('a path that leads nowhere is refused in the dialog', await openDialog.isVisible());
    await openDialog.getByRole('button', { name: 'From Your Computer…' }).click();
    await p.waitForFunction(() => Object.values(JSON.parse(localStorage.getItem('fixture-drafts'))).some(d => d.origin.path === 'bound.md'));
    ok('From Your Computer opens the computer\'s file in the Editor', await p.evaluate(() => __calls.some(c => c[0] === 'computer-open' && c[1] === 'w1')) && await openDialog.count() === 0);

    // The Editor's menus: Edit is undoing and finding; Insert, Format and
    // Review have the rest.
    const boundWin = p.locator('.win[data-win-kind="editor"]', { has: p.locator('[data-ed-meta]', { hasText: 'bound.md' }) });
    await p.evaluate(key => window.pudlWindows.raise(key), await boundWin.getAttribute('data-win'));
    await p.waitForTimeout(200);
    const menuText = async name => {
      await p.keyboard.press('Escape');
      await p.locator('.menubar-row .menubar-title').filter({ hasText: new RegExp('^' + name + '$') }).click();
      await p.locator('#menubar-panel .menu-action').first().waitFor();
      return p.locator('#menubar-panel').textContent();
    };
    const editMenu = await menuText('Edit');
    ok('Edit is undoing and finding only', editMenu.includes('Find…') && editMenu.includes('Undo') && !editMenu.includes('Bold') && !editMenu.includes('Suggestion') && !editMenu.includes('Problem'));
    const insertMenu = await menuText('Insert'), formatMenu = await menuText('Format'), reviewMenu = await menuText('Review');
    ok('Insert has links, pictures, files, tables and code blocks', ['Link…', 'Image…', 'File…', 'Table', 'Code Block'].every(l => insertMenu.includes(l)));
    ok('Format has the styles, lists and table commands', ['Bold', 'Strikethrough', 'Paragraph Style', 'Bulleted List', 'Quote', 'Table'].every(l => formatMenu.includes(l)));
    ok('Review has the suggestions and problems', ['Next Suggestion', 'Accept the Suggestion', 'Next Problem'].every(l => reviewMenu.includes(l)));
    const order = await p.locator('.menubar-row .menubar-title').allTextContents();
    const at = name => order.indexOf(name);
    ok('the menu bar has Edit, Insert, Format and Review in that order', at('Edit') >= 0 && at('Edit') < at('Insert') && at('Insert') < at('Format') && at('Format') < at('Review'));
    await t.shot('editor-menus');
    await p.keyboard.press('Escape');

    // Save As starts beside the file it was opened from, and To Your
    // Computer saves a copy, asking before it replaces a file.
    await p.evaluate(key => window.pudlWindows.raise(key), await boundWin.getAttribute('data-win'));
    await boundWin.locator('[data-ed-save-as]').click();
    const copyDialog = p.locator('dialog.dinah-picker[open]');
    await copyDialog.locator('tr.fm-row[data-name="bound.md"]').waitFor();
    ok('Save As starts in the folder of the file, suggesting a copy\'s name', await copyDialog.getByLabel('File Name').inputValue() === 'bound copy.md');
    ok('and it costs no suggestion call, not being new', (await p.evaluate(() => __calls.filter(c => c[0] === 'suggest-name').length)) === 3);
    await copyDialog.getByRole('button', { name: 'To Your Computer…' }).click();
    await p.locator('#ask[open]').waitFor();
    ok('replacing a file on the computer asks first', (await p.locator('#ask-title').textContent()) === 'Replace the File?' &&
      await p.evaluate(() => __calls.some(c => c[0] === 'computer-choose' && c[1] === 'bound copy.md')));
    await p.locator('#ask-yes').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'computer-save' && c[2] === true));
    ok('the copy is saved and the draft still belongs to its file', await copyDialog.count() === 0 &&
      await p.evaluate(() => Object.values(JSON.parse(localStorage.getItem('fixture-drafts'))).some(d => d.origin.path === 'bound.md')));
    // A document opened to read opens in Preview; a character typed there
    // switches to Split without typing it, Space only scrolls, and opening it
    // again to work keeps Split.
    const readKey = await p.evaluate(() => window.dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'taken.md' }, 'read'));
    const reader = p.locator('[data-win="' + readKey + '"]');
    await reader.locator('[data-ed-page] .doc-block').first().waitFor();
    ok('a document opened to read opens in Preview', await reader.locator('[data-ed-panes]').getAttribute('data-view') === 'preview');
    const previewFocused = await p.waitForFunction(k => document.querySelector('[data-win="' + k + '"] [data-ed-scroll]')?.contains(document.activeElement), readKey, { timeout: 3000 }).then(() => true, () => false);
    ok('a document opened in Preview has the focus in its page, without a click', previewFocused);
    await reader.locator('[data-ed-page]').click();
    const unread = await reader.locator('.cm-content').textContent();
    await p.keyboard.press('Space');
    ok('Space in Preview scrolls and changes nothing', await reader.locator('[data-ed-panes]').getAttribute('data-view') === 'preview');
    await p.keyboard.press('X');
    await reader.locator('.cm-content').waitFor();
    ok('a character typed in Preview switches to Split, with the text unchanged and focused', await reader.locator('[data-ed-panes]').getAttribute('data-view') === 'split' &&
      (await reader.locator('.cm-content').textContent()) === unread && await reader.locator('.cm-editor.cm-focused').count() === 1);
    await reader.locator('[data-command="view-preview"]').click();
    await p.evaluate(() => window.dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'taken.md' }));
    ok('opening it again to work switches the open window to Split', await reader.locator('[data-ed-panes]').getAttribute('data-view') === 'split');
    await p.evaluate(() => document.activeElement && document.activeElement.blur());
    await p.evaluate(() => window.dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'taken.md' }));
    const refocused = await reader.locator('.cm-editor.cm-focused').waitFor({ timeout: 3000 }).then(() => true, () => false);
    ok('opening an open document again brings its window to the front with the cursor in its text', refocused && await p.evaluate(k => pudlWindows.state().top === k, readKey));
    // A change made to the file elsewhere is seen while the window is open,
    // without a save being refused first.
    await p.evaluate(() => { window.__remote = { added: 'Added on disk.' }; });
    await reader.locator('[data-ed-moved]').waitFor({ state: 'visible', timeout: 9000 });
    ok('the window says when its file changed elsewhere, before any save', (await reader.locator('[data-ed-moved-body]').textContent()).includes('was changed on disk since you opened it. ') &&
      (await reader.locator('[data-ed-moved-body]').textContent()).endsWith('brought into your draft now.'));
    await p.evaluate(() => { window.__remote = null; });
    await reader.locator('[data-ed-moved]').waitFor({ state: 'hidden', timeout: 9000 });

    // A document kept in a folder saves itself a moment after a pause in
    // typing. When its file was changed elsewhere, automatic saving stops and
    // says so; a Save that succeeds starts it again.
    await reader.locator('.cm-content').click();
    await p.keyboard.press('Control+End');
    await p.keyboard.type('\nTyped and kept.');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'save' && c[1].includes('Typed and kept.')), null, { timeout: 8000 });
    await reader.locator('[data-ed-status]').getByText('Saved.').waitFor();
    ok('a document kept in a folder saves itself after a pause in typing, without Save', true);
    await p.evaluate(() => { window.__remote = { added: 'Added on disk.' }; });
    await p.keyboard.type(' More.');
    await reader.locator('[data-ed-status]').getByText('Automatic saving stopped').waitFor({ timeout: 8000 });
    ok('when the file changed elsewhere, automatic saving stops and says so', (await reader.locator('[data-ed-status]').textContent()).includes('Save once the document is settled'));
    const savesWhileStopped = await p.evaluate(() => __calls.filter(c => c[0] === 'save').length);
    await p.keyboard.type(' Still more.');
    await p.waitForTimeout(2500);
    ok('while it is stopped, typing saves nothing', await p.evaluate(n => __calls.filter(c => c[0] === 'save').length === n, savesWhileStopped));
    await p.evaluate(() => { window.__remote = null; });
    await reader.locator('[data-ed-save]').click();
    await reader.locator('[data-ed-status]').getByText('Saved.').waitFor();
    await reader.locator('.cm-content').click();
    await p.keyboard.press('Control+End');
    await p.keyboard.type(' Again.');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'save' && c[1].includes('Again.')), null, { timeout: 8000 });
    ok('a Save that succeeds starts automatic saving again', true);

    // Images and files: Insert Image and Attach File in the toolbar keep
    // the chosen file with the draft and put it in the text, an image as an
    // image the preview loads from the draft and any other file as a link;
    // a pasted image is kept and put in the same way.
    await reader.locator('.cm-content').click();
    await p.keyboard.press('Control+End');
    await reader.locator('[data-command="insert-image"]').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'update' && c[1].includes('![](chart.png)')));
    await reader.locator('[data-command="view-preview"]').click();
    const img = reader.locator('[data-ed-page] img[data-attachment="chart.png"]');
    await img.waitFor({ state: 'attached' });
    await p.waitForFunction(k => { const i = document.querySelector('[data-win="' + k + '"] [data-ed-page] img[data-attachment="chart.png"]'); return i && i.complete && i.naturalWidth > 0; }, readKey);
    ok('Insert Image keeps the picked image with the draft, and the preview loads it from the draft', (await img.getAttribute('src')).startsWith('/draft-file?') && await p.evaluate(() => __calls.some(c => c[0] === 'pick' && c[1] === true)));
    await reader.locator('[data-command="view-split"]').click();
    await reader.locator('.cm-content').click();
    await p.keyboard.press('Control+End');
    await reader.locator('[data-command="insert-file"]').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'update' && c[1].includes('[notes v2.pdf](<notes v2.pdf>)')));
    ok('Attach File puts a link to the picked file in the text', true);
    await p.evaluate(() => {
      const bytes = Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='), c => c.charCodeAt(0));
      const data = new DataTransfer();
      data.items.add(new File([bytes], 'image.png', { type: 'image/png' }));
      document.activeElement.dispatchEvent(new ClipboardEvent('paste', { clipboardData: data, bubbles: true, cancelable: true }));
    });
    await p.waitForFunction(() => __calls.some(c => c[0] === 'update' && /!\[\]\(pasted-[^)]+\.png\)/.test(c[1])));
    ok('a pasted image is kept with the draft under a name of its own and put in the text', await p.evaluate(() => __calls.some(c => c[0] === 'add-file' && /^pasted-.+\.png$/.test(c[1]) && c[2] > 0)));
    // The Visual view builds the document's page, which starts at the
    // window's edge and keeps its text off it.
    await reader.locator('[data-command="view-page"]').click();
    await reader.locator('[data-ed-page] .ProseMirror').waitFor();
    ok('Visual builds the page of the document', !(await reader.locator('[data-ed-status]').textContent()).includes('could not be built') &&
      await reader.locator('[data-command="view-page"]').getAttribute('aria-pressed') === 'true');
    ok('the page starts at the window\'s edge and keeps its text off it', await reader.locator('[data-ed-page]').evaluate(el => {
      const host = el.parentElement.getBoundingClientRect(), page = el.getBoundingClientRect(), style = getComputedStyle(el);
      return Math.abs(page.left - host.left) < 1 && parseFloat(style.paddingLeft) >= 24 && parseFloat(style.paddingRight) >= 24 && parseFloat(style.borderLeftWidth) === 0;
    }));
    ok('a table column is never narrowed below its longest word', await reader.locator('[data-ed-page]').evaluate(el => {
      // A copy of the page holding a table whose long second column presses
      // on its first, as a requirements table's does.
      const copy = el.cloneNode(false);
      copy.innerHTML = '<table><tbody><tr><th>Requirement ID</th><th>Requirement</th></tr><tr><td>REQ-MWP-FR001</td><td>' + 'A long requirement that fills its column. '.repeat(40) + '</td></tr></tbody></table><span id="word">Requirement</span>';
      el.parentElement.appendChild(copy);
      const th = copy.querySelector('th'), word = copy.querySelector('#word').getBoundingClientRect().width, padding = parseFloat(getComputedStyle(th).paddingLeft) + parseFloat(getComputedStyle(th).paddingRight);
      const fits = th.clientWidth - padding >= word - 0.5;
      copy.remove();
      return fits;
    }));
    // Find and Replace is the Editor's own bar (page-view.js covers it on
    // the page): in the text, the matches are marked there, Match Case is a
    // pressed toggle, and the replace row appears only when asked for.
    await reader.locator('[data-command="view-split"]').click();
    await reader.locator('.cm-content').click();
    await p.keyboard.press('Control+f');
    await reader.locator('[data-ed-find]').waitFor();
    ok('Ctrl+F opens the find bar without the replace row, with the field focused', await reader.locator('[data-ed-find-replace-row]').isHidden() &&
      await reader.locator('[data-ed-find-input]').evaluate(el => el === document.activeElement));
    await reader.locator('[data-ed-find-input]').fill('KEPT');
    await reader.locator('[data-ed-find-count]').getByText('1 of 1').waitFor();
    ok('the matches are marked in the text and counted, the current one set apart', await reader.locator('.cm-find-current').count() === 1);
    await t.shot('editor-find');
    await reader.locator('[data-ed-find-option="matchCase"]').click();
    await reader.locator('[data-ed-find-count]').getByText('No matches').waitFor();
    ok('Match Case is a pressed toggle that narrows the search', await reader.locator('[data-ed-find-option="matchCase"]').getAttribute('aria-pressed') === 'true');
    await reader.locator('[data-ed-find-option="matchCase"]').click();
    await reader.locator('[data-ed-find-show-replace]').click();
    await reader.locator('[data-ed-find-replace-row]').waitFor({ state: 'visible' });
    await reader.locator('[data-ed-replace-input]').fill('held');
    await reader.locator('[data-ed-replace-one]').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'update' && c[1].includes('Typed and held.')));
    ok('Replace changes the text and moves on', (await reader.locator('[data-ed-find-count]').textContent()) === 'No matches');
    await reader.locator('[data-ed-find-input]').fill('held');
    await reader.locator('[data-ed-replace-input]').fill('kept');
    await reader.locator('[data-ed-replace-all]').click();
    await p.waitForFunction(() => __calls.filter(c => c[0] === 'update').pop()[1].includes('Typed and kept.'));
    ok('Replace All puts it back', true);
    await reader.locator('[data-ed-find-close]').click();
    ok('Close hides the bar and its marks', await reader.locator('[data-ed-find]').isHidden() && await reader.locator('.cm-find-match').count() === 0);
    // The formatting toolbar is there in Text too, acting on the text, with
    // its pressed states from the line and the marks around the cursor.
    await reader.locator('[data-command="view-text"]').click();
    await reader.locator('[data-ed-toolbar] [data-command="bold"]').waitFor({ state: 'visible' });
    await reader.locator('.cm-content').click();
    await p.keyboard.press('Control+Home');
    await p.waitForTimeout(100);
    ok('in Text the toolbar is shown, the Paragraph menu enabled and saying the heading the cursor is in', await reader.locator('[data-ed-toolbar] .editor-toolbar-style').isEnabled() &&
      (await reader.locator('[data-ed-toolbar] .editor-toolbar-style .toolbar-words').textContent()) === 'Heading 1');
    await reader.locator('.cm-content').evaluate(el => { const v = CM.EditorView.findFromDOM(el), t = v.state.doc.toString(), at = t.indexOf('kept'); v.dispatch({ selection: { anchor: at, head: at + 4 } }); });
    await reader.locator('[data-ed-toolbar] [data-command="bold"]').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'update' && c[1].includes('**kept**')));
    await p.waitForTimeout(100);
    ok('Bold in the toolbar wraps the selected text, and shows as pressed', await reader.locator('[data-ed-toolbar] [data-command="bold"]').getAttribute('aria-pressed') === 'true');
    await reader.locator('[data-ed-toolbar] [data-command="bold"]').click();
    await p.waitForFunction(() => !__calls.filter(c => c[0] === 'update').pop()[1].includes('**kept**'));
    await reader.locator('[data-command="view-split"]').click();
    // F2 renames the file in front, in its folder, with the part before its
    // extension selected and a name the folder has refused in the dialog;
    // the window follows the file.
    const fileKey = await p.evaluate(() => window.dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'old-name.md' }));
    const fileWin = p.locator('[data-win="' + fileKey + '"]');
    await fileWin.locator('.cm-content').click();
    await p.keyboard.press('F2');
    const nameDialog = p.locator('dialog.dinah-ask-name[open]');
    const nameField = nameDialog.getByLabel('Name');
    await nameField.waitFor();
    ok('F2 in an Editor on a file asks for its new name, the extension unselected', await nameField.inputValue() === 'old-name.md' && await nameField.evaluate(el => el.selectionEnd === 8) &&
      await nameDialog.locator('.dialog-title').textContent() === 'Rename the File');
    await nameField.fill('taken.md');
    await p.keyboard.press('Enter');
    await nameDialog.locator('.form-error').filter({ hasText: 'already in the folder' }).waitFor();
    await nameField.fill('new-name.md');
    await p.keyboard.press('Enter');
    await nameDialog.waitFor({ state: 'detached' });
    await p.waitForFunction(k => document.querySelector('[data-win="' + k + '"] .win-title').textContent.startsWith('new-name.md'), fileKey);
    ok('Rename renames the file and the window follows it', await p.evaluate(() => __calls.some(c => c[0] === 'rename-file' && c[1] === 'old-name.md' && c[2] === 'new-name.md')));
    // A new document has no file name, so it cannot be renamed.
    await p.evaluate(() => window.dinahEditor.newDocument());
    await p.waitForFunction(() => __calls.some(c => c[0] === 'new'));
    await p.locator('.win[data-win-kind="editor"]').last().locator('.cm-content').click();
    await p.keyboard.press('F2');
    await p.waitForTimeout(200);
    ok('F2 on a new document asks nothing', await p.locator('dialog.dinah-ask-name').count() === 0);
    ok('no duplicate IDs in mounted applets', await p.evaluate(() => {
      const ids = [...document.querySelectorAll('[id]')].map(el => el.id); return new Set(ids).size === ids.length;
    }));
  } finally { await t.close(); }
})().catch(err => { console.error(err); process.exitCode = 1; });

