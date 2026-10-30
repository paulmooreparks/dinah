// One surface for documents: a document a chat or Find and Open lists opens
// in the Editor, and a document window kept in an older layout does too. The
// resource viewer that remains keeps its links, problems and preview.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => {
      const workspace = 'C:/First Workspace';
      window.__calls = [];
      const state = () => ({ workspace, workspaceId: 'w1', recent: [workspace], approvals: 'devin', devin: 'devin', catalog: [], defaults: {}, pinned: [], titles: {}, templates: [],
        workspaceRecords: [{ id: 'w1', name: 'First', mountAlias: 'First', primarySourceId: 'local', sources: [{ id: 'local', name: 'First', mountAlias: 'First', kind: 'local-folder', path: workspace }] }] });
      return {
        State: async () => state(),
        RecentDocuments: async () => [{ title: 'First Document', ref: { workspaceId: 'w1', sourceId: 'local', path: 'first.md' }, workspaceName: 'First', opened: '2026-10-01T09:00:00Z' }],
        DocumentRefAt: async (ws, rel) => { window.__calls.push(['ref', ws, rel]); return { workspaceId: 'w1', sourceId: 'local', path: rel }; },
        PreviewWorkspaceResource: async ref => ({ resource: { ref, title: 'Notes', revision: '1', actions: ['preview'] }, format: '',
          blocks: Array.from({ length: 35 }, (_, i) => ({ line: i + 1, html: '<p' + (i === 25 ? ' id="scope"' : '') + '>Passage ' + (i + 1) + ' in ' + ref.path + '.</p>' +
            (i === 0 ? '<a href="#scope">Scope</a> <a href="https://example.com/notes">External Notes</a>' : '') })),
          problems: [{ line: 3, severity: 'warning', message: 'Check this passage.' }] })
      };
    },
    ready: () => !!window.dinahEditor,
  });
  const p = require('./lib/windows')(t.page);
  const ok = t.ok;
  try {
    // The Editor's own opening is the editor suite's; here it is recorded.
    await p.evaluate(() => { window.dinahEditor.openResource = ref => { window.__calls.push(['editor', ref.path]); return Promise.resolve(); }; });

    await require('./lib/open-document')(p, 'First Document');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'editor'));
    ok('a document in Find and Open opens in the Editor', await p.evaluate(() => JSON.stringify(__calls) === JSON.stringify([['editor', 'first.md']])) &&
      await p.locator('.win[data-win-kind="document"]').count() === 0);

    // A document window kept in a layout from before opens the Editor for
    // its file and closes itself.
    await p.evaluate(() => { __calls.length = 0; return dinahWorkbench.open({ kind: 'document', workspace: 'C:/First Workspace', path: 'first.md' }); });
    await p.waitForFunction(() => __calls.some(c => c[0] === 'editor'));
    await p.waitForFunction(() => !document.querySelector('.win[data-win-kind="document"]'));
    ok('a document window from an older layout opens the Editor and closes', await p.evaluate(() => __calls.some(c => c[0] === 'ref' && c[2] === 'first.md')));

    // The resource viewer that remains.
    const key = await p.evaluate(() => dinahWorkbench.open(dinahWorkbench.resourceItem({ workspaceId: 'w1', sourceId: 'local', path: 'notes.md' }, 'Notes')));
    const first = p.locator('[data-win="' + key + '"]');
    await first.locator('.doc-block').first().waitFor();
    await first.getByRole('link', { name: 'External Notes' }).evaluate(el => el.click());
    ok('web links open externally without navigating the app', await p.evaluate(() => window.__runtime.filter(r => r[0] === 'BrowserOpenURL').map(r => r[1]).join('|') === 'https://example.com/notes') && p.url().startsWith('http://dinah.test/'));
    await first.getByRole('link', { name: 'Scope', exact: true }).evaluate(el => el.click());
    ok('table of contents links stay inside their own window', await first.locator('[data-doc-scroll]').evaluate(el => el.scrollTop) > 300);
    await first.locator('[data-doc-problems-btn]').click();
    await first.locator('.doc-problems button').click();
    ok('problem links go to the passage', await first.locator('.doc-block.flash').count() === 1);
    await t.shot('document-windows-light');
    await p.evaluate(() => window.pudlSetTheme('dark'));
    await t.shot('document-windows-dark');
    ok('the preview follows Mineral dark mode', await first.locator('.doc-page').evaluate(el => getComputedStyle(el).backgroundColor) === 'rgb(23, 36, 45)');
    ok('no duplicate IDs in mounted applets', await p.evaluate(() => {
      const ids = [...document.querySelectorAll('[id]')].map(el => el.id); return new Set(ids).size === ids.length;
    }));
  } finally { await t.close(); }
})().catch(err => { console.error(err); process.exitCode = 1; });
