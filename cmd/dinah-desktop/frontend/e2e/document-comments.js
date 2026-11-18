// A document's comment margin (document-comments.js). Choosing a comment's
// quote selects its words and scrolls them into view when they are out of
// sight (Paul, 18 November 2026), and the comment commands are offered to
// the Docs applet, so that its Review menu and its toolbar can have them.
const { launch } = require('./lib/page');
(async () => {
  const lines = Array.from({ length: 160 }, (_, i) => i === 150 ? 'The needle phrase is here, far below the top.' : 'Filler paragraph number ' + i + ' of the long document.');
  const t = await launch({
    viewport: { width: 1600, height: 900 },
    app: lines => {
      window.__calls = [];
      const text = '# Long\n\n' + lines.join('\n\n') + '\n';
      const at = text.indexOf('needle phrase');
      const draft = { id: 'd0000000000000000000000000000001', title: 'long.md', text, revision: 'r1',
        origin: { kind: 'local-folder', workspaceId: 'w1', sourceId: 'local', path: 'long.md' },
        baseline: { text, revision: 'b1', title: 'long.md' } };
      const view = () => Object.assign({}, draft, { spans: {}, suggestions: [], changed: draft.text !== draft.baseline.text, writeBack: 'save', actions: ['update', 'discard', 'save-as'] });
      window.__threads = [{ id: 't1', kind: 'comment', quote: 'needle phrase', from: at, to: at + 'needle phrase'.length, resolved: false,
        messages: [{ author: 'A Reader', at: new Date().toISOString(), text: 'Look at this.' }] }];
      const state = () => ({ workspaceId: 'w1', workspace: 'C:/Fixture', recent: ['C:/Fixture'], documentViews: {}, catalog: [], defaults: {}, pinned: [], titles: {}, templates: [],
        newWindowMode: 'floating', windowModes: {}, windowModesShipped: {},
        workspaceRecords: [{ id: 'w1', name: 'Fixture', mountAlias: 'Fixture', primarySourceId: 'local', sources: [
          { id: 'local', name: 'Fixture', mountAlias: 'Fixture', kind: 'local-folder', path: 'C:/Fixture' }] }] });
      const html = t => t.split('\n\n').filter(Boolean).map(p => p.startsWith('# ') ? '<h1>' + p.slice(2) + '</h1>' : '<p>' + p + '</p>').join('');
      return {
        State: async () => state(),
        RecentDocuments: async () => [],
        OpenDraft: async () => view(),
        GetDraft: async () => view(),
        ListDrafts: async () => [],
        UpdateDraft: async (id, expected, next) => { draft.text = next; draft.revision = 'r' + Date.now(); return view(); },
        SaveDraft: async () => Object.assign(view(), { notes: [] }),
        PreviewDraft: async () => ({ revision: draft.revision, problems: [], blocks: [{ line: 1, html: html(draft.text) }] }),
        PageDocumentOf: async (id, t) => ({ type: 'doc', attrs: { front: '', tail: '' }, content: [{ type: 'source_block', attrs: { uid: 1, gap: '', src: t, kind: 'text', html: html(t) } }] }),
        PageDocumentMarkdown: async doc => doc.content.map(b => (b.attrs.gap || '') + b.attrs.src).join(''),
        DraftThreads: async () => { window.__calls.push(['threads']); return window.__threads; }
      };
    },
    args: lines,
    // The suites' page is not a secure context, where a browser leaves
    // crypto.randomUUID out; Dinah's window is one, and Docs' page view uses it.
    init: () => { if (!crypto.randomUUID) crypto.randomUUID = () => '10000000-1000-4000-8000-100000000000'.replace(/[018]/g, c => (c ^ crypto.getRandomValues(new Uint8Array(1))[0] & 15 >> c / 4).toString(16)); },
    ready: () => window.dinahEditor && window.dinahWorkspaceBrowser.records().length === 1 && document.querySelector('.menubar-ready'),
  });
  const { page: p, ok } = t;
  try {
    await p.evaluate(() => dinahEditor.openResource({ workspaceId: 'w1', sourceId: 'local', path: 'long.md' }));
    const quote = p.locator('.document-comment-quote').first();
    await quote.waitFor();
    await p.waitForFunction(() => [...document.querySelectorAll('.content-applet p')].some(e => e.textContent.includes('needle phrase')));

    // The quote goes somewhere, so it is drawn as PUDL's link: the accent
    // colour and an underline, as an anchor is.
    ok('the quote is drawn as a link', await quote.evaluate(q => {
      const a = document.createElement('a'); a.href = '#'; a.textContent = 'x'; q.parentElement.appendChild(a);
      const s = getComputedStyle(q), l = getComputedStyle(a), same = s.color === l.color && s.textDecorationLine.includes('underline');
      a.remove(); return q.classList.contains('link') && same;
    }));
    await quote.locator('xpath=..').screenshot({ path: require('path').join(require('./lib/page').OUT, 'document-comment-card.png') });

    // The words are far below the top, out of sight, until the quote is chosen.
    const inView = () => p.evaluate(() => {
      const el = [...document.querySelectorAll('.content-applet p')].find(e => e.textContent.includes('needle phrase'));
      let s = el.parentElement;
      while (s && !((getComputedStyle(s).overflowY === 'auto' || getComputedStyle(s).overflowY === 'scroll') && s.scrollHeight > s.clientHeight)) s = s.parentElement;
      const a = el.getBoundingClientRect(), b = s.getBoundingClientRect();
      return a.top >= b.top && a.bottom <= b.bottom;
    });
    ok('the commented words start out of sight', !(await inView()));
    await quote.click();
    await p.waitForTimeout(200);
    ok('choosing the quote selects the commented words', await p.evaluate(() => String(window.getSelection()).toLowerCase() === 'needle phrase'));
    ok('choosing the quote scrolls the commented words into view', await inView());
    await t.shot('document-comments-reveal');

    // The comment commands are the host's for Docs, at most eight in all,
    // and say which choice of shown comments is current.
    const commands = await p.evaluate(() => {
      const dc = window.pudlHost.documentCommands, resource = document.querySelector('.content-applet').dataset.contentResource;
      const ids = dc.list('docs').map(d => d.id);
      const state = id => dc.state(id, { application: 'docs', resource, selected: false });
      return { ids, count: ids.length, hide: state('hide-comments'), open: state('show-comments'), all: state('show-all-comments'), next: state('next-comment') };
    });
    ok('Docs is offered the comment commands, eight commands in all', ['new-comment', 'next-comment', 'previous-comment', 'show-comments', 'show-all-comments', 'hide-comments'].every(id => commands.ids.includes(id)) && commands.count === 8);
    ok('the current choice is greyed and the others can run', commands.open.present && commands.open.disabled && commands.hide.present && !commands.hide.disabled && !commands.all.disabled && !commands.next.disabled);
    await p.evaluate(() => {
      const resource = document.querySelector('.content-applet').dataset.contentResource;
      return window.pudlHost.documentCommands.run('hide-comments', { application: 'docs', resource });
    });
    await p.waitForTimeout(100);
    ok('Hide Comments from the applet hides them', await p.evaluate(() => window.dinahDocumentComments.shown() === 'off'));
    const after = await p.evaluate(() => {
      const resource = document.querySelector('.content-applet').dataset.contentResource;
      return window.pudlHost.documentCommands.state('hide-comments', { application: 'docs', resource });
    });
    ok('Hide Comments is then greyed with its reason', after.present && after.disabled && after.reason === 'The comments are hidden.');
    await p.evaluate(() => window.dinahDocumentComments.setShown('open'));
  } finally { await t.close(); }
})();
