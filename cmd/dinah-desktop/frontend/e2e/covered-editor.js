// A document's editor that was covered by another window, and so not drawn
// (covered-windows.js), is drawn and takes typing as soon as it is raised.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1600, height: 1000 },
    state: { workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'], documentViews: { 'text/markdown': 'text' }, titles: {}, templates: [],
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
            const text = '# ' + ref.path + '\n\n' + Array.from({ length: 60 }, (_, i) => 'Line ' + i + ' of the document.').join('\n\n') + '\n';
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
        OpenSessionAt: async (ws, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: ws, open: true, working: false, queued: [] }),
      };
    },
    ready: () => window.dinahEditor && window.dinahWorkspaceBrowser && window.dinahWorkspaceBrowser.records().length === 1,
  });
  const { page: p, ok } = t;
  try {
    const frames = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
    const doc = await p.evaluate(path => dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path }), 'alpha.md');
    await p.locator('[data-win="' + doc + '"] .cm-content').waitFor();
    const chat = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:over' }));
    await p.locator('[data-win="' + chat + '"] [data-view-id="prompt"]').waitFor();
    // The chat, opened last and so in front, is made larger than the document on every side.
    await p.evaluate(([doc, chat]) => {
      const query = new URLSearchParams(location.search);
      query.set('p.' + doc, 'floating:0.3,0.1,0.4,0.6'); query.set('p.' + chat, 'floating:0.28,0.07,0.44,0.66');
      history.replaceState(history.state, '', location.pathname + '?' + query.toString() + location.hash);
      window.dispatchEvent(new PopStateEvent('popstate'));
    }, [doc, chat]);
    await frames();
    const covered = k => p.evaluate(k => document.querySelector('[data-win="' + k + '"]').hasAttribute('data-dinah-covered'), k);
    ok('the document behind the chat is covered and not drawn', await covered(doc) && await p.evaluate(k => getComputedStyle(document.querySelector('[data-win="' + k + '"] > .win-body')).contentVisibility, doc) === 'hidden');
    // The person works in the chat for a while.
    await p.locator('[data-win="' + chat + '"] [data-view-id="prompt"]').click();
    await p.keyboard.type('Some words typed into the chat in front.');
    await frames();
    ok('the document is still covered while the chat is used', await covered(doc));
    await p.evaluate(k => pudlWindows.raise(k), doc);
    await frames();
    ok('the raised document is drawn', !await covered(doc));
    const shown = await p.evaluate(k => {
      const lines = [...document.querySelectorAll('[data-win="' + k + '"] .cm-line')];
      const first = lines[0] && lines[0].getBoundingClientRect(), box = document.querySelector('[data-win="' + k + '"] .cm-content').getBoundingClientRect();
      return { lines: lines.length, height: first ? first.height : 0, wide: box.width > 100, text: lines.slice(0, 3).map(l => l.textContent).join('|') };
    }, doc);
    ok('its lines are laid out and show the document\'s text', shown.lines > 5 && shown.height > 8 && shown.wide && /alpha\.md/.test(shown.text), JSON.stringify(shown));
    await p.locator('[data-win="' + doc + '"] .cm-content').click();
    await p.keyboard.press('Control+Home');
    await p.keyboard.type('Typed after raising. ');
    ok('and it takes typing', /Typed after raising\./.test(await p.locator('[data-win="' + doc + '"] .cm-content').innerText()));
    ok('with the keyboard focus in the document', await p.evaluate(k => document.querySelector('[data-win="' + k + '"]').contains(document.activeElement), doc));
  } finally { await t.close(); }
})();
