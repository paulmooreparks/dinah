const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    app: () => {
      let name = localStorage.getItem('workspace-name') || 'Notes';
      const ref = relative => ({workspaceId:'w1',sourceId:'local',path:relative});
      const entries = () => [
        {path:'/',title:'/',directory:true,actions:['list']},
        {path:'/home',title:'Home',directory:true,actions:['list'],ref:{workspaceId:'',sourceId:'dinah-home',path:''}},
        {path:'/workspaces',title:'Workspaces',directory:true,actions:['list']},
        {path:'/workspaces/' + name,title:name,directory:true,actions:['list'],ref:{workspaceId:'w1',sourceId:'',path:''}},
        ...['','nested','nested/deeper','nested/deeper/file.txt'].map(relative => ({path:'/workspaces/' + name + '/Local' + (relative ? '/' + relative : ''),title:relative.split('/').pop() || 'Local',kind:relative.endsWith('.txt') ? 'file' : 'directory',directory:!relative.endsWith('.txt'),actions:relative.endsWith('.txt') ? ['reveal'] : ['list'],ref:ref(relative)}))
      ];
      const stat = virtual => { const item = entries().find(item => item.path === virtual); if (!item) throw Error('Mount unavailable'); return item; };
      const state = () => ({workspaceId:'w1',workspace:'C:/Fixture',workspaceRecords:[{id:'w1',name,mountAlias:name,primarySourceId:'local',sources:[{id:'local',name:'Local',mountAlias:'Local',kind:'local-folder',path:'C:/Fixture'}]}],recent:['C:/Fixture'],defaultProvider:'codex',catalog:[],defaults:{},pinned:[]});
      window.__renameWorkspace = (next, notify = true) => { name = next; localStorage.setItem('workspace-name',name); if (notify) window.__emit('workspace:changed', {kind:'workspace-renamed',workspaceId:'w1'}); };
      window.__resolverCalls = [];
      return {
        State:async () => state(),
        ReadUIState:async () => JSON.parse(localStorage.getItem('fixture-ui') || '{"version":1,"values":{}}'), SaveUIState:async value => localStorage.setItem('fixture-ui',JSON.stringify(value)),
        StatWorkspaceMount:async virtual => stat(virtual),
        ResolveWorkspaceResourceMount:async expected => { __resolverCalls.push(expected); const item = entries().find(item => item.ref && item.ref.workspaceId === expected.workspaceId && item.ref.sourceId === expected.sourceId && item.ref.path === expected.path); if (!item) throw Error('Resource identity unavailable'); return item; },
        StatWorkspaceMountExpected:async (virtual,expected) => { const item = stat(virtual); if (!item.ref || item.ref.workspaceId !== expected.workspaceId || item.ref.sourceId !== expected.sourceId) throw Error('Resource identity changed'); return item; },
        ListWorkspaceMounts:async (virtual,start,limit) => ({entries:entries().filter(item => item.path !== '/' && (item.path.slice(0,item.path.lastIndexOf('/')) || '/') === virtual).slice(start,start+limit),nextStart:-1}),
        ListWorkspaceMountsExpected:async (virtual,start,limit,expected) => { await go.main.App.StatWorkspaceMountExpected(virtual,expected); return go.main.App.ListWorkspaceMounts(virtual,start,limit); }
      };
    },
  });
  const { page, ok } = t;
  try {
    const original = '/workspaces/Notes/Local/nested';
    const retained = {workspaceId:'w1',sourceId:'local',path:'nested'};
    await page.evaluate(({original,retained}) => dinahResources.openFiles(original,retained), {original,retained});
    let win = page.locator('.win[data-win-kind=files]');
    await win.locator('tr[data-name=deeper]').waitFor();
    await page.evaluate(() => __renameWorkspace('Renamed Notes'));
    await win.locator('.fm-path [aria-current=location][title="/workspaces/Renamed Notes/Local/nested"]').waitFor();
    ok('an open Files folder follows the current workspace name through retained IDs', await win.locator('tr[data-name=deeper]').count() === 1 && await win.locator('.fm-path a[data-path="/workspaces/Notes"]').count() === 0);
    await page.evaluate(() => dinahUIState.flush());
    const key = await win.getAttribute('data-win');
    await page.evaluate(key => pudlWindows.close(key),key);
    await page.evaluate(() => dinahUIState.flush());
    await page.evaluate(() => __renameWorkspace('Current Name',false));
    await page.reload();
    await t.waitReady();
    await page.evaluate(({original,retained}) => dinahResources.openFiles(original,retained), {original,retained});
    win = page.locator('.win[data-win-kind=files]');
    await win.locator('.fm-path [aria-current=location][title="/workspaces/Current Name/Local/nested"]').waitFor();
    await win.locator('tr[data-name=deeper]').waitFor();
    ok('reopened saved Files locations resolve their stable resource reference before the obsolete path', await win.locator('.fm-status-error').count() === 0 && await page.evaluate(() => __resolverCalls.some(ref => ref.workspaceId === 'w1' && ref.sourceId === 'local' && ref.path === 'nested')));
    await win.locator('tr[data-name=deeper]').dblclick();
    await win.locator('tr[data-name="file.txt"]').waitFor();
    ok('navigation after reopening keeps the renamed namespace', await win.locator('.fm-path [aria-current=location]').getAttribute('title') === '/workspaces/Current Name/Local/nested/deeper');
    await t.shot('files-renamed-restored');
  } finally { await t.close(); }
})();
