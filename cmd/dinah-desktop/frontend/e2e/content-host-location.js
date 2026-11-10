// The content host tells the applets when a document has no file yet, so
// that their Save asks for a place (yavc.app v0.2.61, needsLocation), and
// passes on the build's cell colour shortcode (cellColor) only for an
// inline shortcode.
const { launch } = require('./lib/page');
(async () => {
  const ws = 'C:\\W';
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    state: { workspace: ws, recent: [ws], approvals: 'devin', devin: 'd', titles: {}, templates: [] },
    app: () => {
      const draft = (id, origin) => ({ id, title: id === 'n'.repeat(32) ? 'Untitled' : 'notes.md', text: '# Notes\n', revision: 'r1', documentType: '', origin, actions: ['update', 'discard', 'save-as'], suggestions: [], spans: {}, changed: false, writeBack: origin.kind === 'new' ? '' : 'save' });
      const drafts = { ['n'.repeat(32)]: draft('n'.repeat(32), { kind: 'new', workspaceId: 'C:\\W' }), ['f'.repeat(32)]: draft('f'.repeat(32), { kind: 'local-folder', workspaceId: 'C:\\W', sourceId: 's', path: 'notes.md' }) };
      return {
        GetDraft: async id => drafts[id],
        UpdateDraft: async (id, rev, text) => Object.assign({}, drafts[id], { text, revision: 'r2', changed: true }),
        SaveDraft: async (id) => Object.assign({}, drafts[id], { revision: 'r3' }),
        ShortcodeCatalog: async () => [
          { name: 'cell-colour', title: 'Cell colour', kind: 'inline', cellColor: true },
          { name: 'band', title: 'Band', kind: 'container', cellColor: true },
        ],
      };
    },
  });
  const p = t.page;
  const { ok } = t;
  try {
    await p.waitForFunction(() => window.dinahContentHost);
    const got = await p.evaluate(async () => {
      const host = window.dinahContentHost;
      const fresh = host.resourceFor(await go.main.App.GetDraft('n'.repeat(32)));
      const saved = host.resourceFor(await go.main.App.GetDraft('f'.repeat(32)));
      await host.install().catch(() => {});
      const resources = window.pudlHost && window.pudlHost.contentResources;
      const newRead = await host.read({ resource: fresh });
      const fileRead = await host.read({ resource: saved });
      const newWrite = resources ? await resources.write({ resource: fresh, expectedRevision: 'r1', text: '# Changed\n' }) : null;
      const fileWrite = resources ? await resources.write({ resource: saved, expectedRevision: 'r1', text: '# Changed\n' }) : null;
      const workspace = window.pudlHost && window.pudlHost.contentWorkspace;
      const catalog = workspace && workspace.shortcodes ? (await workspace.shortcodes({})).shortcodes : null;
      return { newRead, fileRead, newWrite, fileWrite, catalog };
    });
    ok('a new document is read as needing a location', got.newRead.needsLocation === true, JSON.stringify(got.newRead));
    ok('a document that is a file is read without the field', !('needsLocation' in got.fileRead), JSON.stringify(got.fileRead));
    ok('a write of a new document says it still needs a location', got.newWrite && got.newWrite.needsLocation === true, JSON.stringify(got.newWrite));
    ok('a write of a file says nothing of a location', got.fileWrite && !('needsLocation' in got.fileWrite), JSON.stringify(got.fileWrite));
    const colour = got.catalog && got.catalog.find(d => d.name === 'cell-colour');
    const band = got.catalog && got.catalog.find(d => d.name === 'band');
    ok('the inline cell colour shortcode reaches the applets marked', colour && colour.cellColor === true, JSON.stringify(got.catalog));
    ok('a container is never marked for a cell colour', band && !('cellColor' in band), JSON.stringify(got.catalog));
  } finally { await t.close(); }
})();
