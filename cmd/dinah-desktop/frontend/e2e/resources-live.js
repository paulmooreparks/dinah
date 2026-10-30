// The resource tree on its own page: source identity, label and backend
// refreshes, watched folders, launchers and the generic applet shim.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    state: { workspaceId: 'w1', workspace: 'C:/Fixture', workspaceRecords: [{ id: 'w1', mountAlias: 'alpha', primarySourceId: 'files', sources: [{ id: 'docs', kind: 'local-folder', path: 'D:/Documents' }, { id: 'files', kind: 'local-folder', path: 'C:/Fixture' }] }, { id: 'w2', mountAlias: 'other', executionPath: 'C:/Other' }] },
    app: () => {
      const ref = (source, resource = '') => ({ workspaceId: source === 'dinah-home' ? '' : 'w1', sourceId: source, path: resource });
      const item = (mountedPath, title, directory, source, resource = '') => ({ path: mountedPath, title, directory, kind: directory ? 'directory' : 'file', actions: directory ? ['list'] : ['read', 'preview'], ...(source ? { ref: ref(source, resource) } : {}) });
      const clone = value => JSON.parse(JSON.stringify(value));
      function stat(mountedPath) {
        const values = {
          '/': item('/', '/', true),
          '/home': item('/home', 'home', true),
          '/home/dinah': item('/home/dinah', 'Home', true, 'dinah-home'),
          '/home/dinah/notes.md': item('/home/dinah/notes.md', 'Personal Notes', false, 'dinah-home', 'notes.md'),
          '/workspaces': item('/workspaces', 'Workspaces', true),
          '/workspaces/alpha': item('/workspaces/alpha', 'Delivery ' + __revision, true),
          '/workspaces/alpha/docs': item('/workspaces/alpha/docs', 'Documents ' + __revision, true, 'docs', ''),
          '/workspaces/alpha/docs/guide.md': item('/workspaces/alpha/docs/guide.md', 'Guide ' + __revision, false, 'docs', 'guide.md')
        };
        if (!values[mountedPath]) throw Error('Resource unavailable');
        return clone(values[mountedPath]);
      }
      return {
        StatWorkspaceMount: async mountedPath => { __calls.push(['stat', mountedPath]); return stat(mountedPath); },
        StatWorkspaceMountExpected: async (mountedPath, expected) => {
          __calls.push(['stat-expected', mountedPath, clone(expected)]);
          const value = stat(mountedPath);
          if (__wrongSource && mountedPath.includes('/docs')) value.ref.sourceId = 'replacement';
          return value;
        },
        ListWorkspaceMounts: async (mountedPath, start, limit) => {
          __calls.push(['list', mountedPath, start, limit]);
          const children = { '/': ['/home', '/workspaces'], '/home': ['/home/dinah'], '/home/dinah': ['/home/dinah/notes.md'], '/workspaces': ['/workspaces/alpha'], '/workspaces/alpha': ['/workspaces/alpha/docs'], '/workspaces/alpha/docs': ['/workspaces/alpha/docs/guide.md'] };
          return { entries: (children[mountedPath] || []).map(stat), nextStart: -1 };
        },
        ListWorkspaceMountsExpected: async (mountedPath, start, limit, expected) => {
          const value = stat(mountedPath);
          if (expected.sourceId !== value.ref.sourceId || expected.workspaceId !== value.ref.workspaceId) throw Error('Wrong source identity');
          __calls.push(['list-expected', clone(expected)]);
          return go.main.App.ListWorkspaceMounts(mountedPath, start, limit);
        },
        AppletCatalog: async () => [{ id: 'dinah.fixture', title: 'Fixture Applet', actions: [{ ref: 'dinah.fixture/render@1', inputSchema: { type: 'object' } }] }],
        InvokeAppletAt: async (workspace, provider, action, inputJSON) => { __calls.push(['invoke', workspace, provider, action, inputJSON]); return JSON.parse(inputJSON).slow ? { id: 'fixture-invocation', status: 'running' } : { id: 'fixture-invocation', status: 'done', outputJSON: '{"at":"start"}' }; },
        AppletInvocationAt: async (workspace, id) => { __calls.push(['status', workspace, id]); return window.__fail ? { id, status: 'failed', error: 'The fixture failed.' } : { id, status: 'done', outputJSON: '{"at":"end"}' }; },
        CancelAppletInvocationAt: async (workspace, id) => { __calls.push(['cancel', workspace, id]); },
        WatchFolders: async refs => { (window.__watched = window.__watched || []).push(clone(refs)); }
      };
    },
    // __calls and the flags are defined before the stub's App reads them,
    // since the stub is sent to the page first and the App methods run later.
    init: () => {
      window.__calls = []; window.__revision = 0; window.__notifications = 0; window.__wrongSource = false;
      window.dinahWorkbench = {
        register() {},
        resourceItem: (resourceRef, title) => ({ kind: 'workspace-resource', ref: resourceRef, title }),
        open: async value => { __calls.push(['open', value]); return value; },
        launchers: () => [{ id: 'fixture-launcher', title: 'Fixture Launcher', run: async () => { __calls.push(['launch']); } }]
      };
    },
    goto: false,
  });
  const { page, ok, errors } = t;
  try {
    // The page is a fixture of the resource scripts alone; the other files
    // come from the shared route.
    await t.context.route('http://dinah.test/fixture.html', route => route.fulfill({ contentType: 'text/html', body: '<!doctype html><html><head></head><body><script src="helpers.js"></script><script src="regional.js"></script><script src="applet-sdk.js"></script><script src="resources.js"></script></body></html>' }));
    await page.goto('http://dinah.test/fixture.html');
    await page.evaluate(async () => {
      window.__tree = dinahResources.createTree();
      window.__docs = await __tree.lookup('/workspaces/alpha/docs');
      await __tree.load(__docs);
      window.__child = __docs.children[0];
      window.__owner = __docs.parent;
      window.__savedRefs = JSON.stringify([__docs.ref, __child.ref]);
      __tree.onChange(() => { __notifications++; });
    });
    ok('a source root keeps its directory object and loads its children by its own identity', await page.evaluate(() => __tree.resolve(null, '/workspaces/alpha/docs') === __docs && __docs.ref.path === '' && __docs.children[0].ref.path === 'guide.md' && __calls.some(call => call[0] === 'list-expected' && call[1].sourceId === 'docs')));
    await page.evaluate(async () => { await dinahResources.open(__child); });
    ok('Open addresses a resource by its workspace and source', await page.evaluate(() => __calls.some(call => call[0] === 'open' && call[1].kind === 'workspace-resource' && call[1].ref.path === 'guide.md' && call[1].ref.sourceId === 'docs')));
    const mismatch = await page.evaluate(async () => {
      window.__wrongSource = true;
      try { await dinahResources.verify(__docs); return false; } catch (error) { return error.message.includes('different workspace source'); }
      finally { window.__wrongSource = false; }
    });
    ok('a matching mount path cannot bypass a changed source ID', mismatch);
    await page.evaluate(() => { __revision = 1; document.dispatchEvent(new CustomEvent('dinah:workspace-resource-labels')); });
    await page.waitForFunction(() => __notifications === 1);
    ok('label events update directory and ancestor labels without replacing node identities', await page.evaluate(() => __docs.title === 'Documents 1' && __owner.title === 'Delivery 1' && __child.title === 'Guide 1' && __tree.resolve(null, __docs.path) === __docs && __docs.children[0] === __child && JSON.stringify([__docs.ref, __child.ref]) === __savedRefs));
    await page.evaluate(() => { __revision = 2; __emit('workspace:changed', { kind: 'source-renamed', workspaceId: 'w1', sourceId: 'docs' }); });
    await page.waitForFunction(() => __notifications === 2);
    ok('backend workspace events refresh loaded resources through the same subscription', await page.evaluate(() => __docs.title === 'Documents 2' && __owner.title === 'Delivery 2' && __child.title === 'Guide 2' && __docs.children[0] === __child));
    await page.evaluate(async () => { window.__home = await __tree.lookup('/home/dinah'); await __tree.load(__home); window.__note = await __tree.lookup('/home/dinah/notes.md'); });
    // The folders the tree has opened are watched on disk, and a change there
    // lists them again.
    await page.waitForFunction(() => (window.__watched || []).length && __watched[__watched.length - 1].some(r => r.sourceId === 'dinah-home'));
    ok('the folders a tree has opened are watched on disk', await page.evaluate(() => { const last = __watched[__watched.length - 1]; return last.some(r => r.sourceId === 'docs') && last.some(r => r.sourceId === 'dinah-home'); }));
    await page.evaluate(() => { __revision = 3; __emit('workspace:changed', { kind: 'disk-changed', workspaceId: 'w1', sourceId: 'docs' }); });
    await page.waitForFunction(() => __notifications === 3);
    ok('a change on disk lists the opened folders again', await page.evaluate(() => __docs.title === 'Documents 3' && __docs.children[0] === __child));
    ok('Home lookup preserves its dedicated source and empty workspace identity', await page.evaluate(() => __home === __tree.home() && __note.parent === __home && __home.children[0] === __note && __note.ref.workspaceId === '' && __note.ref.sourceId === 'dinah-home' && __calls.some(call => call[0] === 'list-expected' && call[1].sourceId === 'dinah-home')));
    ok('commands enumerate the launcher registry and generic applet shim', await page.evaluate(() => dinahResources.commands().some(entry => entry.name === 'fixture-launcher') && dinahResources.commands().some(entry => entry.name === 'applets')));
    await page.evaluate(async () => { await dinahResources.invoke('fixture-launcher', [], () => false); await dinahResources.invoke('fixture-launcher', [], () => true); });
    ok('launcher invocation respects cancellation before dispatch', await page.evaluate(() => __calls.filter(call => call[0] === 'launch').length === 1));
    const catalog = await page.evaluate(async () => [await dinahResources.invoke('applets', ['list']), await dinahResources.invoke('applets', ['actions', 'dinah.fixture'])]);
    ok('generic applet discovery uses the real SDK catalog', catalog[0].includes('dinah.fixture') && catalog[1].includes('dinah.fixture/render@1'));
    const result = await page.evaluate(() => dinahResources.invoke('applets', ['invoke', 'dinah.fixture/render@1', '{"path":"/workspaces/alpha/docs/guide.md"}'], () => true));
    ok('generic action invocation uses its discovered provider and the workspace\'s folder', result.includes('fixture-invocation') && await page.evaluate(() => __calls.some(call => call[0] === 'invoke' && call[1] === 'C:/Fixture' && call[2] === 'dinah.fixture' && call[3] === 'dinah.fixture/render@1' && JSON.parse(call[4]).path === '/workspaces/alpha/docs/guide.md')));
    await page.evaluate(async () => { await dinahResources.invoke('applets', ['invoke', 'dinah.fixture/render@1', '{}'], () => false); await dinahResources.invoke('applets', ['status', 'fixture-invocation']); await dinahResources.invoke('applets', ['cancel', 'fixture-invocation']); });
    ok('generic invocation cancellation suppresses dispatch and lifecycle calls retain the workspace', await page.evaluate(() => __calls.filter(call => call[0] === 'invoke').length === 1 && __calls.some(call => call[0] === 'status' && call[1] === 'C:/Fixture') && __calls.some(call => call[0] === 'cancel' && call[1] === 'C:/Fixture')));
    await page.evaluate(async () => {
      await dinahResources.invoke('applets', ['invoke', 'dinah.fixture/render@1', '{}'], () => true, {path:'/workspaces/other/files',ref:{workspaceId:'w2',sourceId:'s2',path:''}});
      await dinahResources.invoke('applets', ['invoke', 'dinah.fixture/render@1', '{}'], () => true, {path:'/home',ref:{workspaceId:'',sourceId:'dinah-home',path:''}});
    });
    const ran = await page.evaluate(async () => {
      const quick = await dinahResources.invoke('applets', ['run', 'dinah.fixture/render@1', '{}'], () => true);
      const slow = await dinahResources.invoke('applets', ['run', 'dinah.fixture/render@1', '{"slow":true}'], () => true);
      window.__fail = true;
      let failed = '';
      try { await dinahResources.invoke('applets', ['run', 'dinah.fixture/render@1', '{"slow":true}'], () => true); } catch (e) { failed = e.message; }
      window.__fail = false;
      return [quick, slow, failed];
    });
    ok('applets run gives the output of an action once it ends, and a failure as an error', ran[0] === '{"at":"start"}' && ran[1] === '{"at":"end"}' && ran[2] === 'The fixture failed.');
    ok('Terminal action scope follows its VFS location instead of the active workspace, given by its folder', await page.evaluate(() => __calls.some(call => call[0] === 'invoke' && call[1] === 'C:/Other') && __calls.some(call => call[0] === 'invoke' && call[1] === '')));
    const count = await page.evaluate(() => { __tree.destroy(); return __calls.length; });
    await page.evaluate(() => { document.dispatchEvent(new CustomEvent('dinah:workspace-resource-labels')); __emit('workspace:changed', { kind: 'source-added', workspaceId: 'w1' }); });
    ok('destroying the tree removes live resource subscriptions', await page.evaluate(() => __calls.length) === count);
    await page.waitForFunction(() => __watched[__watched.length - 1].length === 0);
    ok('a closed tree\'s folders are no longer watched', true);
    ok('no browser errors or dialogs', errors.length === 0);
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
