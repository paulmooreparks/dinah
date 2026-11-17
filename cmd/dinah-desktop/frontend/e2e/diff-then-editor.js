// A document opens in the Editor whether or not a Diff window was opened
// first, and the Diff window opens whether or not an Editor was.
//
// The Diff view and the content applets each bring their own build of the
// editing library, with different parts in each. Both used to put it in the
// page under one name, and the applets take a library already there under
// that name as theirs, so after a Diff window had opened every document
// failed with "The editor could not start: Cannot read properties of
// undefined (reading 'define')" (17 November 2026).
const { launch } = require('./lib/page');
(async () => {
  for (const first of ['diff', 'editor']) {
    const t = await launch({
      viewport: { width: 1400, height: 900 },
      state: { workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'], documentViews: { 'text/markdown': 'split' }, titles: {}, templates: [],
        workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [{ id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] },
      app: () => {
        let n = 0;
        const drafts = {};
        const get = id => { const d = drafts[id]; if (!d) throw new Error('the draft no longer exists'); return d; };
        const view = d => Object.assign({}, d, { spans: {}, suggestions: [], changed: d.text !== d.baseline.text, writeBack: 'save', actions: ['update', 'discard', 'save-as', 'save'] });
        return {
          OpenDraft: async ref => {
            let d = Object.values(drafts).find(x => x.origin.path === ref.path);
            if (!d) {
              const text = '# ' + ref.path + '\n\nFirst line.\n';
              const id = 'k' + Object.keys(drafts).length.toString().padStart(31, '0');
              d = drafts[id] = { id, origin: { kind: 'local-folder', workspaceId: 'w1', sourceId: 'local', path: ref.path }, baseline: { text, revision: 'b1', title: ref.path }, title: ref.path, text, revision: 'r' + (++n) };
            }
            return view(d);
          },
          GetDraft: async id => view(get(id)), ListDrafts: async () => Object.values(drafts).map(view),
          UpdateDraft: async (id, expected, text) => { const d = get(id); d.text = text; d.revision = 'r' + (++n); return view(d); },
          PageDocumentOf: async (id, text) => ({ type: 'doc', attrs: { front: '', tail: '' }, content: [{ type: 'source_block', attrs: { uid: 1, gap: '', src: text, kind: 'text', html: '<p>Text</p>' } }] }),
          PageDocumentMarkdown: async doc => doc.content.map(b => (b.attrs.gap || '') + b.attrs.src).join(''),
          PreviewDraft: async id => ({ revision: get(id).revision, problems: [], blocks: [{ line: 1, html: '<p>Text</p>' }] }),
          CompareDraft: async id => ({ moved: false, revision: 'b1', title: get(id).title, theirs: '', mine: '', merged: get(id).text, conflicts: [] }),
        };
      },
      ready: () => window.dinahEditor && window.dinahDiff && window.dinahWorkspaceBrowser && window.dinahWorkspaceBrowser.records().length === 1,
    });
    const { page: p, ok } = t;
    try {
      const diff = async () => {
        await p.evaluate(() => window.dinahDiff.open({ key: 'C:/Fixture|a.go|unstaged', title: 'a.go', detail: 'Modified', path: 'a.go', language: 'a.go', before: 'package a\n\nvar old = 1\n', after: 'package a\n\nvar now = 2\n', mode: 'inline', readOnly: true, actions: [] }));
        await p.locator('.win[data-win-kind="diff"] .cm-mergeView, .win[data-win-kind="diff"] .cm-editor').first().waitFor();
      };
      const editor = async file => {
        const key = await p.evaluate(path => dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path }), file);
        // Either the text appears or the Editor says why it could not start; the check below tells which.
        await p.locator('[data-win="' + key + '"] .cm-content, [data-win="' + key + '"] .ed-failed').first().waitFor();
        return key;
      };
      if (first === 'diff') {
        await diff();
        const key = await editor('alpha.md');
        ok('after a Diff window, a document opens in the Editor', await p.locator('[data-win="' + key + '"] .cm-content').count() === 1 && await p.locator('.ed-failed').count() === 0, await p.locator('.ed-failed').allInnerTexts().then(a => a.join(' | ')));
        ok('and its text can be typed into', await (async () => { await p.locator('[data-win="' + key + '"] .cm-content').click(); await p.keyboard.type('Typed. '); return /Typed\./.test(await p.locator('[data-win="' + key + '"] .cm-content').innerText()); })());
      } else {
        const key = await editor('alpha.md');
        ok('a document opens in the Editor', await p.locator('[data-win="' + key + '"] .cm-content').count() === 1);
        await diff();
        ok('after an Editor, the Diff window draws its comparison', await p.locator('.win[data-win-kind="diff"] .cm-changedLine, .win[data-win-kind="diff"] .cm-deletedChunk, .win[data-win-kind="diff"] .cm-insertedLine').count() > 0);
        const second = await editor('beta.md');
        ok('and a second document still opens after it', await p.locator('[data-win="' + second + '"] .cm-content').count() === 1 && await p.locator('.ed-failed').count() === 0, await p.locator('.ed-failed').allInnerTexts().then(a => a.join(' | ')));
      }
      ok(first + ' first: the applets\' library is the one under the shared name, and the Diff view\'s has a name of its own',
        await p.evaluate(() => !!window.CM && typeof window.CM.Facet === 'function' && !window.CM.MergeView && !!window.dinahCodeMirror && typeof window.dinahCodeMirror.MergeView === 'function'));
    } finally { await t.close(); }
  }
})();
