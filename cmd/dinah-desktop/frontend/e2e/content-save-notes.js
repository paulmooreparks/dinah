// The real Editor and both Save As routes consume an invented draft service.
const { launch } = require('./lib/page');
(async () => {
  for (const theme of ['light', 'dark']) {
    const t = await launch({ state: {
      workspace: 'C:/Fixture', workspaceId: 'w', recent: ['C:/Fixture'],
      workspaceRecords: [{ id: 'w', name: 'Fixture', executionPath: 'C:/Fixture', primarySourceId: 'files',
        sources: [{ id: 'files', name: 'Files', kind: 'local-folder', path: 'C:/Fixture' },
          { id: 'pages', name: 'Pages', kind: 'document-provider' }] }]
    }, app: () => {
      const d = { id: 'a'.repeat(32), title: 'Invented notes.md', text: '# Invented notes\n\n![Invented picture](missing-invented.png)\n', revision: '1',
        origin: { kind: 'local-folder', workspaceId: 'w', sourceId: 'files', path: 'notes.md' },
        writeBack: 'save', changed: false, actions: ['update', 'save-as', 'discard'], suggestions: [], spans: {} };
      const copy = value => JSON.parse(JSON.stringify(value));
      window.__notes = []; window.__saves = []; window.__answers = {};
      return {
        GetDraft: async () => copy(d),
        UpdateDraft: async (id, rev, text) => { d.text = text; d.revision = String(+d.revision + 1); return copy(d); },
        RetitleDraft: async (id, rev, title) => { d.title = title; return copy(d); },
        SaveDraftAs: async (id, rev, ref) => {
          __saves.push({ ref: copy(ref), text: d.text, notes: copy(__notes || []) });
          d.origin = { kind: ref.sourceId === 'pages' ? 'document-provider' : 'local-folder', ...ref };
          d.changed = false; d.writeBack = ref.sourceId === 'pages' ? 'publish' : 'save';
          return Object.assign(copy(d), __notes === null ? {} : { notes: copy(__notes) });
        },
        AnswerPage: async (id, result, error) => { __answers[id] = { result, error }; },
        CompareDraft: async () => ({ moved: false }), ShortcodeCatalog: async () => [],
        DraftFiles: async () => [], DocumentComments: async () => []
      };
    }});
    const p = t.page;
    try {
      await t.deadline('opening the invented document', p.evaluate(async theme => {
        pudlSetTheme(theme);
        window.__notices = [];
        const toast = dinahHost.toast.bind(dinahHost);
        dinahHost.toast = (text, kind, options) => { __notices.push({ text, kind, options }); return toast(text, kind, options); };
        window.__key = await dinahContentWindows.open(await go.main.App.GetDraft(), 'editor');
        await dinahWorkbench.ready(__key);
        dinahFilePicker.pick = async options => {
          await options.commit({ name: 'Invented copy.md', dir: { creates: true, writable: false,
            ref: { workspaceId: 'w', sourceId: 'pages', path: 'invented-folder' } } });
        };
      }, theme));
      const notes = ['The picture missing-invented.png was not found beside the original, so the copy is without it.',
        'The copy has no generated preview.'];
      for (const mode of ['notes', 'empty', 'absent']) {
        const expected = mode === 'notes' ? notes : [];
        for (const route of ['person', 'agent']) {
          await p.evaluate(value => { __notes = value; __notices.length = 0; document.querySelectorAll('.toast').forEach(n => n.remove()); }, mode === 'absent' ? null : expected);
          const before = await p.evaluate(() => __saves.length);
          if (route === 'person') {
            const result = await t.deadline('Save As to the document provider', p.evaluate(() => dinahDocuments.get(__key).saveAs()));
            t.ok('Save As completes with ' + mode + ' in ' + theme, result === true);
          } else {
            const id = 'save-' + theme + '-' + mode;
            await t.emit('page:ask', { id, op: 'content', args: { app: 'editor', action: 'yavc.editor/save@1', live: true,
              input: { path: 'copies/invented-' + mode + '.md' }, workspaceId: 'w', workspace: 'C:/Fixture' } });
            await p.waitForFunction(id => !!__answers[id], id);
            const answer = await p.evaluate(id => __answers[id], id);
            t.ok('agent save returns its document with ' + mode + ' in ' + theme,
              !answer.error && !!JSON.parse(answer.result).output.path, JSON.stringify(answer));
          }
          const got = await p.evaluate(() => ({ notices: __notices, writes: __saves.length, saved: __saves.at(-1) }));
          t.ok(route + ' preserves the completed copy with ' + mode + ' in ' + theme,
            got.writes === before + 1 && got.saved.text.includes('missing-invented.png') && JSON.stringify(got.saved.notes) === JSON.stringify(expected));
          const warnings = got.notices.filter(n => n.kind === 'warn');
          t.ok(route + ' shows every returned note once as a kept warning with ' + mode + ' in ' + theme,
            JSON.stringify(warnings.map(n => n.text)) === JSON.stringify(expected) && warnings.every(n => n.options && n.options.keep === true), JSON.stringify(warnings));
          for (const note of expected) {
            t.ok(route + ' renders the returned warning in ' + theme + ': ' + note,
              await p.locator('.toast.warn').filter({ hasText: note }).count() === 1);
          }
          if (mode === 'notes') await t.shot('save-notes-' + route + '-' + theme);
        }
      }
    } finally { await t.close(); }
  }
})().catch(err => { console.error(err); process.exitCode = 1; });
