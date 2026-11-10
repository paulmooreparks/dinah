// A build that registers kinds of source besides a local folder: Add
// Source… opens a dialog offering the folder and each kind, draws the
// chosen kind's fields, sends what was filled in, shows the kind's refusal
// in place, and lists the added source like any other. Settings gains a
// Connections section when the build registers connections, and keeps it
// hidden when it registers none.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 1000 },
    app: () => {
      window.__calls = [];
      const records = [{ id: 'w1', mountAlias: 'w1', name: 'Delivery', primarySourceId: 'primary', sources: [{ id: 'primary', mountAlias: 'primary', kind: 'local-folder', name: 'Working Files', path: 'C:/Working' }] }];
      const state = () => ({ workspace: 'C:/Working', workspaceId: 'w1', workspaceRecords: records, workspaces: ['C:/Working'], recent: ['C:/Working'], defaultProvider: 'devin', catalog: [], defaults: {}, pinned: [] });
      const item = (ref, name, directory, actions, kind) => ({ ref, title: name, kind: kind || (directory ? 'folder' : 'text'), directory, revision: '1', actions });
      const connections = [{ id: 'wiki', label: 'Wiki', help: 'Pages come from here.', address: '', defaultAddress: 'https://wiki.example', hasToken: false, testable: true }];
      const backend = {
        State: async () => state(), ReadUIState: async () => JSON.parse(localStorage.getItem('fixture-ui') || '{"version":1,"values":{}}'), SaveUIState: async value => localStorage.setItem('fixture-ui', JSON.stringify(value)),
        AgentProviders: async () => [{ id: 'devin', name: 'Devin', installed: true, available: true }],
        WorkspaceSourceKinds: async () => window.__noKinds ? [] : [
          { kind: 'wiki-page', label: 'Wiki Page', help: 'The page uses your Wiki connection.', fields: [{ id: 'page', label: 'Page ID or URL', kind: 'text', required: true, placeholder: '12345' }, { id: 'children', label: 'Include Child Pages', kind: 'check', default: 'true' }] },
          { kind: 'tracker-project', label: 'Tracker Project', help: '', fields: [{ id: 'key', label: 'Project Key', kind: 'text', required: true }] }
        ],
        AttachWorkspaceSource: async (id, kind, values) => {
          window.__calls.push(['attach', id, kind, values]);
          if (values.page === '999') throw new Error('the page could not be found');
          records[0].sources.push({ id: 'wiki1', mountAlias: 'wiki1', kind, name: 'Customer Home' });
          return records[0];
        },
        ChooseWorkspaceSource: async id => { window.__calls.push(['choose', id]); return records[0]; },
        Connections: async () => { window.__calls.push(['connections']); return window.__noConnections ? [] : connections; },
        SetConnection: async (id, address, token) => { window.__calls.push(['set', id, address, token]); connections[0].address = address; connections[0].hasToken = connections[0].hasToken || !!token; },
        TestConnection: async id => { window.__calls.push(['test', id]); return 'Signed in as paul.'; },
        RemoveConnection: async id => { window.__calls.push(['remove', id]); connections[0].address = ''; connections[0].hasToken = false; },
        ListWorkspaceResources: async (ref, start, limit) => {
          if (ref.sourceId === 'wiki1') return { entries: [{ ...item({ ...ref, path: '200' }, 'Architecture', true, ['list', 'read', 'preview', 'edit', 'open-external'], 'wiki-page'), glyph: 'globe', url: 'https://wiki.example/pages/200' }], nextStart: -1 };
          return { entries: [], nextStart: -1 };
        },
        PreviewWorkspaceResource: async ref => ({ resource: item(ref, ref.path, false, ['preview']), blocks: [{ line: 1, html: '<h2>Preview</h2>' }], problems: [] }),
        OpenWorkspaceResource: async ref => { window.__calls.push(['external', ref]); },
      };
      const resolve = virtual => {
        const parts = virtual.split('/').filter(Boolean), record = records.find(record => record.mountAlias === parts[1]);
        const source = record && record.sources.find(source => source.mountAlias === parts[2]);
        if (parts.length > 1 && !record || parts.length > 2 && !source) throw new Error('Mount unavailable');
        return { record, source, ref: source && { workspaceId: record.id, sourceId: source.id, path: parts.slice(3).join('/') } };
      };
      const mount = (record, source, resource) => ({ ...resource, path: '/workspaces/' + record.mountAlias + '/' + source.mountAlias + (resource.ref.path ? '/' + resource.ref.path : '') });
      backend.StatWorkspaceMount = async virtual => {
        const { record, source, ref } = resolve(virtual);
        if (!source) return { path: virtual, title: record ? record.name : 'Workspaces', kind: 'catalog', directory: true, actions: ['list'] };
        const result = item({ ...ref, path: ref.path || '' }, ref.path || source.name, true, ['list']);
        if (!ref.path) result.kind = source.kind;
        return mount(record, source, result);
      };
      backend.StatWorkspaceMountExpected = async virtual => backend.StatWorkspaceMount(virtual);
      backend.ListWorkspaceMounts = async (virtual, start, limit) => {
        const { record, source, ref } = resolve(virtual);
        if (source) { const result = await backend.ListWorkspaceResources(ref, start, limit); return { ...result, entries: result.entries.map(resource => mount(record, source, resource)) }; }
        const entries = record ? record.sources.map(source => mount(record, source, { ...item({ workspaceId: record.id, sourceId: source.id, path: '' }, source.name, true, source.kind === 'wiki-page' ? ['list', 'open-external'] : ['list']), kind: source.kind, glyph: source.kind === 'wiki-page' ? 'globe' : undefined, url: source.kind === 'wiki-page' ? 'https://wiki.example/pages/100' : undefined })) : [];
        return { entries, nextStart: -1 };
      };
      backend.ListWorkspaceMountsExpected = async (virtual, start, limit) => backend.ListWorkspaceMounts(virtual, start, limit);
      return backend;
    },
  });
  const { page, ok } = t;
  try {
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    const sources = page.locator('#workspace-sources');
    await sources.getByRole('button', { name: 'Expand Working Files', exact: true }).waitFor();
    await page.locator('#workspace-source-choose').click();
    const dialog = page.locator('#add-workspace-source');
    await dialog.waitFor({ state: 'visible' });
    const kinds = await dialog.locator('#workspace-source-kind [data-kind]').allTextContents();
    ok('the dialog offers a local folder first, then each registered kind by its label', kinds.join('|') === 'Local Folder|Wiki Page|Tracker Project');
    ok('the folder is chosen to begin with and its button says so', await dialog.locator('[data-kind="local-folder"]').getAttribute('aria-checked') === 'true' && await dialog.locator('#workspace-source-add').textContent() === 'Choose a Folder…' && await dialog.locator('#workspace-source-fields').isHidden());
    await dialog.locator('[data-kind="wiki-page"]').click();
    ok('choosing a kind draws its help and fields', await dialog.locator('#workspace-source-help').textContent() === 'The page uses your Wiki connection.' && await dialog.locator('#workspace-source-field-page').getAttribute('placeholder') === '12345' && await dialog.locator('#workspace-source-field-children').isChecked() && await dialog.locator('#workspace-source-add').textContent() === 'Add Source');
    await page.keyboard.press('ArrowRight');
    ok('the arrow keys move between kinds', await dialog.locator('[data-kind="tracker-project"]').getAttribute('aria-checked') === 'true' && await dialog.locator('#workspace-source-field-key').count() === 1);
    await dialog.locator('[data-kind="wiki-page"]').click();
    await dialog.locator('#workspace-source-field-page').fill('999');
    await dialog.locator('#workspace-source-add').click();
    await dialog.locator('#workspace-source-error').waitFor({ state: 'visible' });
    ok('the kind\'s refusal is shown in the dialog, which stays open', await dialog.locator('#workspace-source-error').textContent() === 'the page could not be found' && await dialog.isVisible() && await page.evaluate(() => __calls.filter(c => c[0] === 'attach').length === 1));
    await dialog.locator('#workspace-source-field-page').fill('100');
    await dialog.locator('#workspace-source-field-children').uncheck();
    await dialog.locator('#workspace-source-add').click();
    await dialog.waitFor({ state: 'hidden' });
    ok('a filled form is sent with its values, the check as true or false', await page.evaluate(() => JSON.stringify(__calls.filter(c => c[0] === 'attach').pop()) === JSON.stringify(['attach', 'w1', 'wiki-page', { page: '100', children: 'false' }])));
    const added = sources.locator('[data-resource]').filter({ has: page.locator('.fm-ti-label', { hasText: /^Customer Home$/ }) });
    await added.waitFor();
    ok('the added source is listed with the others', await added.count() === 1 && await sources.getByRole('button', { name: 'Expand Customer Home', exact: true }).count() === 1);
    await sources.getByRole('button', { name: 'Expand Customer Home', exact: true }).click();
    const child = sources.locator('[data-resource]').filter({ has: page.locator('.fm-ti-label', { hasText: /^Architecture$/ }) });
    await child.waitFor();
    ok('the source lists its entries from the backend', await child.count() === 1);
    const glyphs = await page.evaluate(() => Array.from(document.querySelectorAll('#workspace-sources [data-resource] > .glyph')).map(g => [g.nextElementSibling.textContent, g.style.getPropertyValue('--glyph')]));
    ok('a source and an entry of a kind with its own glyph are drawn with it, and a folder as a folder', JSON.stringify(glyphs.filter(g => ['Working Files', 'Customer Home', 'Architecture'].includes(g[0])).sort()) === JSON.stringify([['Architecture', 'var(--glyph-globe)'], ['Customer Home', 'var(--glyph-globe)'], ['Working Files', 'var(--glyph-folder)']]));
    for (const [name, has] of [['Architecture', true], ['Customer Home', true], ['Working Files', false]]) {
      await sources.locator('[data-resource]').filter({ has: page.locator('.fm-ti-label', { hasText: new RegExp('^' + name + '$') }) }).first().click({ button: 'right' });
      const menu = page.locator('.context-menu:popover-open');
      await menu.waitFor();
      ok(name + (has ? ' offers Copy Link' : ' has no Copy Link'), (await menu.getByText('Copy Link', { exact: true }).count() === 1) === has);
      await page.keyboard.press('Escape');
      await menu.waitFor({ state: 'hidden' });
    }
    await page.evaluate(() => { window.__copied = null; dinahHost.copy = function (text) { window.__copied = text; }; });
    await sources.locator('[data-resource]').filter({ has: page.locator('.fm-ti-label', { hasText: /^Architecture$/ }) }).first().click({ button: 'right' });
    await page.locator('.context-menu:popover-open').getByText('Copy Link', { exact: true }).click();
    await page.waitForFunction(() => window.__copied !== null);
    ok('Copy Link puts the entry\'s address on the clipboard', await page.evaluate(() => window.__copied) === 'https://wiki.example/pages/200');
    await page.evaluate(() => { window.__noKinds = true; });
    await page.locator('#workspace-source-choose').click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'choose'));
    ok('with no registered kind, Add Source… goes straight to the folder dialog', await dialog.isHidden());

    await page.evaluate(() => dinahWorkbench.open('settings'));
    const settings = page.locator('#settings');
    const tab = settings.getByRole('tab', { name: 'Connections', exact: true });
    await tab.waitFor();
    ok('Settings shows a Connections section when the build registers connections', await tab.isVisible());
    await tab.click();
    const card = settings.locator('[data-connection="wiki"]');
    await card.waitFor();
    ok('a connection card shows its label, help, the default address and that no token is saved', await card.locator('.card-title').textContent() === 'Wiki' && await card.locator('#connection-wiki-address').getAttribute('placeholder') === 'https://wiki.example' && (await card.locator('#connection-wiki-token').getAttribute('placeholder')).startsWith('Paste') && await card.getByRole('button', { name: 'Test Connection' }).isDisabled());
    await card.locator('#connection-wiki-address').fill('https://wiki.example');
    await card.locator('#connection-wiki-token').fill('secret-token');
    await card.getByRole('button', { name: 'Save' }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'set'));
    const refreshed = settings.locator('[data-connection="wiki"]');
    await refreshed.getByRole('button', { name: 'Forget' }).waitFor();
    ok('Save sends the address and token, and the card redraws with the token saved and Test enabled', await page.evaluate(() => JSON.stringify(__calls.find(c => c[0] === 'set')) === JSON.stringify(['set', 'wiki', 'https://wiki.example', 'secret-token'])) && (await refreshed.locator('#connection-wiki-token').getAttribute('placeholder')).startsWith('A token is saved') && await refreshed.getByRole('button', { name: 'Test Connection' }).isEnabled());
    ok('the token field is a password field and is cleared after saving', await refreshed.locator('#connection-wiki-token').getAttribute('type') === 'password' && await refreshed.locator('#connection-wiki-token').inputValue() === '');
    await refreshed.getByRole('button', { name: 'Test Connection' }).click();
    await refreshed.locator('.settings-status').filter({ hasText: 'Signed in as paul.' }).waitFor();
    ok('Test Connection shows the service\'s answer', true);
    await refreshed.getByRole('button', { name: 'Forget' }).click();
    await page.locator('.dialog[open]').getByRole('button', { name: 'Forget', exact: true }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'remove'));
    ok('Forget asks first, then removes the connection', true);
    await page.evaluate(() => { window.__noConnections = true; });
    await page.evaluate(() => dinahConnectionsSettings.reload());
    await page.waitForFunction(() => document.querySelector('[data-settings-section="connections"]').hidden);
    ok('with no registered connection, the Connections tab is hidden', true);
    // An entry whose name is an identifier (the page's number here) is shown
    // by its title alone in Files, and the path under the selection is made
    // of titles; the entry keeps its name for everything that acts on it.
    await page.evaluate(() => dinahResources.openFiles('/workspaces/w1/wiki1'));
    const files = page.locator('.dinah-files').last();
    const row = files.locator('.fm-row[data-name="200"]');
    await row.waitFor();
    ok('a titled entry is listed by its title, without its identifier beside it', await row.locator('.fm-name').textContent() === 'Architecture/' && !await row.locator('.fm-title').isVisible());
    await row.click();
    await page.waitForFunction(() => { const code = Array.from(document.querySelectorAll('.dinah-files')).pop().querySelector('.fm-detail code'); return code && code.textContent.endsWith('/Architecture'); });
    ok('the path under a selected entry is made of titles', await files.locator('.fm-detail code').textContent() === '/Workspaces/Delivery/Customer Home/Architecture');
  } finally { await t.close(); }
})();
