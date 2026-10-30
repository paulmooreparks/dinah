/* A link in a chat to a file of a type an extension's view declares opens
   that view (docs/plans/document-links.md, "Files that open in an
   extension's view"). The link shows the view's title and glyph; pressing
   it opens the view in the file's workspace with the file in its state,
   which the frame reads with dinah.openedFile; a second link raises the
   same window and sends it the new state; an extension that is gone fails
   with a clear error; and Settings › Extensions lists the declared types
   and the file extensions that open somewhere else. Against a stub of the
   Go side that answers as document_links.go and extension_claims.go do. */
const { launch } = require('./lib/page');
const path = require('path'), fs = require('fs');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__calls = [];
      const diff = { mediaType: 'text/x-diff', extensions: ['.diff', '.patch'] };
      const manifest = { id: 'test.patches', version: '0.1.0', title: 'Patches', description: 'Patches', publisher: { name: 'Test' }, permissions: ['workspace.read'], actions: [{ name: 'inspect' }],
        views: [{ id: 'viewer', title: 'Patches Viewer', glyph: 'git-branch', page: 'index.html', opens: [diff] }, { id: 'pane', title: 'Patches Pane', page: 'index.html', pane: true }] };
      const other = { id: 'test.later', version: '0.1.0', title: 'Later', description: 'Later', publisher: { name: 'Test' }, permissions: ['workspace.read'], actions: [{ name: 'inspect' }],
        views: [{ id: 'viewer', title: 'Later Viewer', page: 'index.html', opens: [{ mediaType: 'text/x-diff', extensions: ['.diff'] }, { mediaType: 'image/svg+xml', extensions: ['.svg'] }] }] };
      window.__catalog = [
        { manifest, enabled: true, fileTypes: [{ view: 'viewer', viewTitle: 'Patches Viewer', mediaType: 'text/x-diff', extensions: ['.diff', '.patch'] }] },
        { manifest: other, enabled: true, fileTypes: [
          { view: 'viewer', viewTitle: 'Later Viewer', mediaType: 'text/x-diff', extensions: ['.diff'], elsewhere: [{ fileExtension: '.diff', by: 'Patches', why: 'earlier' }] },
          { view: 'viewer', viewTitle: 'Later Viewer', mediaType: 'image/svg+xml', extensions: ['.svg'], elsewhere: [{ fileExtension: '.svg', by: 'the Media Viewer', why: 'dinah' }] }] }
      ];
      const resource = p => ({ workspaceId: 'w1', sourceId: 'local', path: p });
      // A file of the declared type, as Go answers for it: opens 'extension',
      // the view's title and glyph, and the view's state built from the file.
      const claimed = (p, line) => ({ opens: 'extension', extension: 'test.patches', extensionView: 'viewer', mediaType: 'text/x-diff', label: 'Open in Patches Viewer', glyph: 'git-branch', line,
        resource: resource(p), state: JSON.stringify({ open: Object.assign({ path: 'C:/Test/' + p, mediaType: 'text/x-diff', resource: resource(p) }, line ? { line } : {}) }) });
      const disk = { 'C:/Test/change.diff': p => claimed('change.diff', p.line), 'C:/Test/fix.patch': p => claimed('fix.patch', p.line),
        'C:/Test/gone.diff': () => Object.assign(claimed('gone.diff', 0), { extension: 'test.gone' }) };
      const resolve = (cwd, ref) => {
        let p = ref.replace(/\\/g, '/'), line = 0, m = /:(\d+)$/.exec(p);
        if (m) { line = +m[1]; p = p.slice(0, m.index); }
        if (!/^[A-Za-z]:\//.test(p)) p = cwd.replace(/\/$/, '') + '/' + p;
        const f = disk[p];
        return f ? Object.assign({ ref, found: true, path: p }, f({ line })) : { ref, found: false, path: p };
      };
      return {
        NativeWindowState: async () => ({ available: false }),
        State: async () => ({ workspace: 'C:/Test', workspaceId: 'w1', recent: ['C:/Test'], defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [], workspaceRecords: [{ id: 'w1', name: 'Test', primarySourceId: 'local', sources: [{ id: 'local', kind: 'local-folder', name: 'Test', path: 'C:/Test' }] }] }),
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        OpenSessionAt: async (ws, id) => ({ sessionId: id, title: id, provider: 'codex', cwd: ws, open: true, working: false, queued: [] }),
        ResolveDocumentLinks: (cwd, refs) => new Promise(done => setTimeout(() => done(refs.map(r => resolve(cwd, r))), 5)),
        ResolveDocumentLink: async (cwd, ref) => { __calls.push(['resolve', cwd, ref]); return resolve(cwd, ref); },
        OpenPath: async p => { __calls.push(['system', p]); return 'opened'; },
        Extensions: async () => __catalog,
        ExtensionWorkspace: async id => { __calls.push(['workspace', id]); return { id: id || 'w1', name: 'Test', path: 'C:/Test', sources: [{ id: 'local', name: 'Test', path: 'C:/Test' }] }; },
        ExtensionCall: async () => ({}), CancelExtensionCall: async () => {}
      };
    },
    // The fixture's stub runs in the extension's sandboxed frames too; an
    // extension sees no bridge, so the frames are cleared of it.
    init: () => { if (window !== parent) for (const k of ['go', 'runtime', '__emit', '__bridgeCalls', '__runtime', '__state', '__calls', '__catalog']) delete window[k]; },
    goto: false,
  });
  const { page: p, ok } = t;
  try {
    await t.context.route('https://dinah.test/**', async route => {
      const pathname = new URL(route.request().url()).pathname;
      if (pathname === '/extension-sdk/dinah-extension.js') return route.fulfill({ contentType: 'text/javascript', body: fs.readFileSync(path.resolve(__dirname, '../../../../../dinah-extension-go/web/dinah-extension.js'), 'utf8'), headers: { 'Access-Control-Allow-Origin': '*' } });
      if (pathname.endsWith('/fixture.js')) return route.fulfill({ contentType: 'text/javascript', body: `
        const show = (id, value) => { document.getElementById(id).textContent = typeof value === 'string' ? value : JSON.stringify(value); };
        window.__opened = [];
        dinah.ready.then(ready => { const file = dinah.openedFile(ready.state); __opened.push(file); show('opened', file); show('workspace', ready.workspace.id); show('status', 'ready'); });
        dinah.on('state', value => { const file = dinah.openedFile(value); __opened.push(file); show('opened', file); });` });
      if (pathname.startsWith('/extension-assets/')) return route.fulfill({ contentType: 'text/html', headers: { 'Content-Security-Policy': "sandbox allow-scripts allow-forms; default-src 'none'; script-src https://dinah.test/extension-assets/ https://dinah.test/extension-sdk/; style-src 'unsafe-inline'; form-action 'none'; frame-ancestors 'self'" }, body: '<!doctype html><main tabindex="-1" data-extension-focus><output id="status"></output><output id="opened"></output><output id="workspace"></output></main><script defer src="/extension-sdk/dinah-extension.js"></script><script defer src="fixture.js"></script>' });
      const file = path.join(__dirname, '..', pathname);
      return fs.existsSync(file) ? route.fulfill({ path: file }) : route.fulfill({ status: 404, body: '' });
    });
    await p.goto('https://dinah.test/index.html');
    await p.waitForFunction(() => window.dinahWorkbench && window.dinahChatText && window.dinahExtensions?.catalog().length === 2);
    const id = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:claims' }));
    const win = '[data-win="' + id + '"]';
    await p.locator(win + ' [data-view-id="prompt"]').waitFor();
    await p.evaluate(() => __emit('agent:update', { sessionId: 'codex:claims', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'The patch is change.diff:12 and the next one is `fix.patch`; gone.diff is from an extension that was removed.' } } }));
    await p.waitForFunction(win => document.querySelectorAll(win + ' .msg-text a.smart-chip[data-opens="extension"]').length === 3, win);
    const look = await p.evaluate(win => [...document.querySelectorAll(win + ' .msg-text a.smart-chip[data-opens="extension"]')].map(a => ({ ref: a.dataset.chipPath, title: a.title, glyph: a.querySelector(':scope > .link-glyph').style.getPropertyValue('--glyph') })), win);
    ok('a link to a claimed file shows the view it opens in, with the view\'s glyph', look[0].ref === 'change.diff:12' && look[0].title === 'Open in Patches Viewer' && look[0].glyph === 'var(--glyph-git-branch)', JSON.stringify(look));

    const frame = p.frameLocator('.extension-frame[title="Patches Viewer"]');
    await p.locator(win + ' .msg-text a[data-chip-path="change.diff:12"]').click();
    await frame.locator('#status').filter({ hasText: 'ready' }).waitFor();
    const first = await frame.locator('main').evaluate(() => __opened[0]);
    ok('pressing the link opens the extension\'s view with the file, its resource and its line in the state', first && first.path === 'C:/Test/change.diff' && first.mediaType === 'text/x-diff' && first.line === 12 && first.resource.workspaceId === 'w1' && first.resource.path === 'change.diff', JSON.stringify(first));
    ok('the view opens as a window in the file\'s workspace', (await frame.locator('#workspace').textContent()) === 'w1' && await p.evaluate(() => __calls.some(c => c[0] === 'workspace' && c[1] === 'w1')));

    // The viewer's window lies over the chat now; the chat comes to the front first.
    await p.evaluate(id => pudlWindows.raise(id), id);
    await p.locator(win + ' .msg-text a[data-chip-path="fix.patch"]').click();
    await frame.locator('#opened').filter({ hasText: 'fix.patch' }).waitFor();
    const second = await frame.locator('main').evaluate(() => __opened.at(-1));
    ok('a second link raises the same window and sends it the new file as a state event', await p.locator('.extension-frame[title="Patches Viewer"]').count() === 1 && second.path === 'C:/Test/fix.patch' && second.line === undefined, JSON.stringify(second));

    const gone = await p.evaluate(() => dinahChatText.openDocument('C:/Test', 'gone.diff').then(() => '', e => String(e.message || e)));
    ok('a link to an extension that is no longer there fails with a clear error', /unavailable/.test(gone), gone);
    ok('nothing was left to the system', !(await p.evaluate(() => __calls.some(c => c[0] === 'system'))));

    // Settings › Extensions lists what each view opens, and warns about the
    // file extensions that open somewhere else.
    await p.evaluate(() => dinahWorkbench.open('extensions'));
    await p.locator('.extension-library .card').nth(1).waitFor();
    const cards = await p.evaluate(() => [...document.querySelectorAll('.extension-library .card')].map(c => ({ help: [...c.querySelectorAll('p.form-help')].map(e => e.textContent), conflicts: [...c.querySelectorAll('p.extension-conflict')].map(e => ({ text: e.textContent.trim(), glyph: e.querySelector('.glyph')?.style.getPropertyValue('--glyph') })) })));
    ok('Settings lists the file types a view opens', cards[0].help.includes('Opens text/x-diff (.diff, .patch) in Patches Viewer.') && cards[0].conflicts.length === 0, JSON.stringify(cards[0]));
    ok('Settings warns, with a glyph, about a type an extension enabled earlier opens and one Dinah opens itself',
      cards[1].conflicts.length === 2 && cards[1].conflicts[0].text === '.diff files open in Patches instead, because it was enabled first.' && cards[1].conflicts[1].text === '.svg files open in the Media Viewer instead, because Dinah opens them itself.' && cards[1].conflicts.every(c => c.glyph === 'var(--glyph-warning)'), JSON.stringify(cards[1]));
  } catch (error) { ok(error.stack, false); } finally { await t.close(); }
})();
