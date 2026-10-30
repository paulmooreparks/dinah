// Export Workspace… and Import Workspace… (docs/design/workspace-export.md):
// the export command, the import window's sources and choices, a source of
// a kind this build does not know kept and not checked, a folder that is
// not on the computer shown dimmed in the Sources tree with Create the
// Folders…, and a workspace whose main folder is missing.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__calls = []; window.__notesMissing = true; window.__oldMissing = true;
      const sample = { id: 'w1', name: 'SampleLab', mountAlias: 'SampleLab', executionPath: 'D:/SampleLab', primarySourceId: 'main', plannedFolders: { notes: ['Meetings', 'Meetings/2026', 'Research'] },
        sources: [{ id: 'main', kind: 'local-folder', name: 'SampleLab', mountAlias: 'SampleLab', path: 'D:/SampleLab' }, { id: 'notes', kind: 'local-folder', name: 'Notes', mountAlias: 'Notes', path: 'D:/Notes' },
          { id: 'web', kind: 'web-page', name: 'Concepts', mountAlias: 'Concepts', baseUrl: 'https://pages.example.com', pageId: '123' }],
        pins: [{ kind: 'flow', title: 'Review', flowScope: 'workspace', flowFile: 'review.flow.md' }, { kind: 'resource', title: 'Meetings', ref: { workspaceId: 'w1', sourceId: 'notes', path: 'Meetings' }, directory: true }] };
      const old = { id: 'w2', name: 'Old Work', mountAlias: 'Old Work', executionPath: 'D:/Old', primarySourceId: 'm2', sources: [{ id: 'm2', kind: 'local-folder', name: 'Old Work', mountAlias: 'Old Work', path: 'D:/Old' }] };
      let current = 'w1';
      const state = () => ({ workspaceId: current, workspace: current === 'w1' ? 'D:/SampleLab' : 'D:/Old', workspaceRecords: [sample, old], recent: ['D:/SampleLab', 'D:/Old'], workspaces: ['D:/SampleLab'],
        missingWorkspaces: window.__oldMissing ? ['w2'] : [], defaultProvider: 'codex', catalog: [], defaults: {}, pinned: [] });
      const item = (p, title, ref, missing, actions) => ({ path: p, title, kind: 'directory', directory: true, actions, ref, missing, writable: !missing });
      const plan = (choices) => {
        const chosen = (choices || []).find(c => c.key === 's2' && c.path);
        return { file: 'C:/Downloads/SampleLab.dinah-workspace', name: 'SampleLab', pins: 2, uncheckedPins: 1, sources: [
          { key: 's1', kind: 'local-folder', name: 'SampleLab', main: true, path: 'C:/Users/me/OneDrive/SampleLab', found: true, folders: 4, checked: true, actions: ['choose-folder'] },
          { key: 's2', kind: 'local-folder', name: 'Notes', main: false, path: chosen ? chosen.path : 'C:/Users/me/OneDrive/Notes', found: !!chosen, folders: 3, checked: true, actions: chosen ? ['choose-folder'] : ['choose-folder', 'create-folder'] },
          { key: 's3', kind: 'web-page', name: 'Concepts', checked: false, actions: [] },
          { key: 's4', kind: 'test-site', name: 'Site', site: 'https://site.example.com', reference: '7', checked: true, problem: 'Needs a connection to https://site.example.com', actions: [] }] };
      };
      window.__listed = {};
      window.__list = async p => {
        const ref = (source, rel) => ({ workspaceId: 'w1', sourceId: source, path: rel || '' });
        if (p === '/workspaces/SampleLab') return { nextStart: -1, entries: [
          { ...item('/workspaces/SampleLab/SampleLab', 'SampleLab', ref('main'), false, ['list']), kind: 'local-folder' },
          { ...item('/workspaces/SampleLab/Notes', 'Notes', ref('notes'), window.__notesMissing, window.__notesMissing ? ['list', 'create-folders', 'choose-folder'] : ['list']), kind: 'local-folder' },
          { path: '/workspaces/SampleLab/Concepts', title: 'Concepts', kind: 'web-page', directory: false, actions: [], ref: ref('web'), unavailable: true }] };
        if (p === '/workspaces/SampleLab/Notes') return { nextStart: -1, entries: [
          item('/workspaces/SampleLab/Notes/Meetings', 'Meetings', ref('notes', 'Meetings'), window.__notesMissing, window.__notesMissing ? ['list', 'create-folders'] : ['list']),
          item('/workspaces/SampleLab/Notes/Research', 'Research', ref('notes', 'Research'), window.__notesMissing, window.__notesMissing ? ['list', 'create-folders'] : ['list'])] };
        if (p === '/workspaces/SampleLab/Notes/Meetings') return { nextStart: -1, entries: [item('/workspaces/SampleLab/Notes/Meetings/2026', '2026', ref('notes', 'Meetings/2026'), window.__notesMissing, ['list'])] };
        return { entries: [], nextStart: -1 };
      };
      const App = {
        State: async () => state(),
        ExportWorkspace: async id => { __calls.push(['export', id]); return 'D:/SampleLab/SampleLab.dinah-workspace'; },
        ChooseWorkspaceImport: async () => { __calls.push(['choose-import']); return plan(); },
        PlanWorkspaceImport: async (file, choices) => { __calls.push(['plan', file, choices]); return plan(choices); },
        ChooseImportFolder: async start => { __calls.push(['choose-folder', start]); return 'E:/Elsewhere/Notes'; },
        ImportWorkspace: async (file, choices) => { __calls.push(['import', file, choices]); return { state: state(), workspaceId: 'w1', existing: false, sources: 4, pins: 2, waiting: 1 }; },
        CreateWorkspaceFolders: async (ws, src) => { __calls.push(['create', ws, src]); if (ws === 'w1') { window.__notesMissing = false; delete sample.plannedFolders; } else window.__oldMissing = false; return ws === 'w1' ? sample : old; },
        RelocateWorkspaceSource: async (ws, src) => { __calls.push(['relocate', ws, src]); return sample; },
        DetachWorkspaceSource: async (ws, src) => { __calls.push(['detach', ws, src]); return sample; },
        SelectWorkspace: async id => { __calls.push(['select', id]); if (id === 'w2' && window.__oldMissing) throw new Error('D:/Old is not a folder that can be opened'); current = id; return state(); },
        StatWorkspaceMount: async p => window.__listed[p] || ({ path: p, title: p === '/workspaces/SampleLab' ? 'SampleLab' : 'Workspaces', kind: 'catalog', directory: true, actions: ['list'] }),
        ListWorkspaceMounts: async p => {
          const page = await window.__list(p);
          page.entries.forEach(e => { window.__listed[e.path] = e; });
          return page;
        },
      };
      App.ListWorkspaceMountsExpected = (p, s, l) => App.ListWorkspaceMounts(p, s, l);
      App.StatWorkspaceMountExpected = p => App.StatWorkspaceMount(p);
      return App;
    },
    ready: () => window.dinahWorkbench && window.dinahCommands,
  });
  const { page, ok } = t;
  try {
    // A toast is shown when its words are on the page as the person sees it.
    const shown = text => page.waitForFunction(t => document.body.innerText.includes(t), text, { timeout: 5000 }).then(() => true, () => false);
    const shot = name => t.shot('workspace-transfer-' + name);

    // Export Workspace… writes the workspace in front and says where.
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd=export-workspace]').click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'export'));
    ok('Export Workspace… exports the workspace in front and says where the file is', await page.evaluate(() => __calls.find(c => c[0] === 'export')[1] === 'w1') &&
      await shown('Exported "SampleLab" to D:/SampleLab/SampleLab.dinah-workspace'));

    // Import Workspace… shows each source with what was found.
    await page.evaluate(() => { document.querySelectorAll('[popover]:popover-open').forEach(p => p.hidePopover()); document.querySelector('#workspace-status-menu [data-cmd=import-workspace]').click(); });
    const win = page.locator('dialog[open]#workspace-import');
    await win.waitFor();
    const row = key => win.locator('.import-source[data-key="' + key + '"]');
    ok('the import window says what it makes', (await win.textContent()).includes('This makes the workspace "SampleLab" with these sources.'));
    ok('the main folder is found and marked', (await row('s1').textContent()).includes('Main Folder') && (await row('s1').textContent()).includes('Found'));
    ok('a folder not here yet says so and offers Choose… and Create It Now', (await row('s2').textContent()).includes('Not on This Computer Yet') &&
      await row('s2').getByRole('button', { name: 'Choose…' }).count() === 1 && await row('s2').getByLabel(/Create It Now, With 3 Folders Inside/).count() === 1);
    ok('a found folder offers no Create It Now', await row('s1').getByLabel(/Create It Now/).count() === 0);
    ok('a source of a kind this build does not know is kept, not checked, with no controls', (await row('s3').textContent()).includes('Kept, Not Checked') &&
      (await row('s3').textContent()).includes('does not use this kind of source (web-page)') && await row('s3').locator('button, input').count() === 0);
    ok('a registered kind shows its site and its problem', (await row('s4').textContent()).includes('https://site.example.com · 7') && (await row('s4').textContent()).includes('Needs a connection to https://site.example.com'));
    ok('the pins line says which are kept unchecked', (await page.locator('#workspace-import-pins').textContent()).includes('2 pins come with it. 1 of them is of a kind'));
    await shot('import');
    await row('s2').getByLabel(/Create It Now/).check();
    ok('choosing to create a folder says what it does and warns about OneDrive', (await row('s2').textContent()).includes('Created on Import') && (await row('s2').textContent()).includes('leave it for that service'));
    await row('s2').getByLabel(/Create It Now/).uncheck();
    await row('s2').getByRole('button', { name: 'Choose…' }).click();
    await page.waitForFunction(() => document.querySelector('#workspace-import .import-source[data-key="s2"]').textContent.includes('E:/Elsewhere/Notes'));
    ok('Choose… plans the import again with the chosen folder', await page.evaluate(() => { const c = __calls.find(c => c[0] === 'plan'); return c && c[2][0].key === 's2' && c[2][0].path === 'E:/Elsewhere/Notes'; }) &&
      (await row('s2').textContent()).includes('Found'));
    await win.getByRole('button', { name: 'Import', exact: true }).click();
    await page.locator('#workspace-import').waitFor({ state: 'hidden' });
    ok('Import imports the file with the choices and says what came in', await page.evaluate(() => { const c = __calls.find(c => c[0] === 'import'); return c && c[1] === 'C:/Downloads/SampleLab.dinah-workspace' && c[2].some(x => x.key === 's2' && x.path === 'E:/Elsewhere/Notes'); }) &&
      await shown('Imported the workspace "SampleLab". 1 folder is not on this computer yet.'));

    // The folder that is not here is shown dimmed, can be browsed, and can be created.
    await page.evaluate(() => dinahWorkbench.open('workspace'));
    const sources = page.locator('#workspace-sources');
    const resource = title => sources.locator('[data-resource]').filter({ has: page.locator('.fm-ti-label', { hasText: new RegExp('^' + title + '$') }) });
    await resource('Notes').waitFor();
    const missing = await resource('Notes').evaluate(a => ({ dim: a.parentElement.classList.contains('workspace-resource-missing'), badge: a.querySelector('.badge') && a.querySelector('.badge').textContent, italic: getComputedStyle(a).fontStyle }));
    ok('a folder not on this computer is dimmed with Not Created Yet', missing.dim && missing.badge === 'Not Created Yet' && missing.italic === 'italic', JSON.stringify(missing));
    ok('a folder that is here is not', !(await resource('SampleLab').evaluate(a => a.parentElement.classList.contains('workspace-resource-missing'))));
    const web = await resource('Concepts').evaluate(a => ({ dim: a.parentElement.classList.contains('workspace-resource-missing'), badge: a.querySelector('.badge') && a.querySelector('.badge').textContent, expands: a.hasAttribute('aria-expanded') }));
    ok('a source of a kind this build does not use is dimmed, Not Available Here, and cannot be expanded', web.dim && web.badge === 'Not Available Here' && !web.expands, JSON.stringify(web));
    ok('a pin of a kind this page does not know has no row; the others do', await page.evaluate(() => { const rows = [...document.querySelectorAll('#workspace-pins [data-pin-key]')]; return rows.length === 1 && rows[0].textContent.includes('Meetings'); }));
    await sources.getByRole('button', { name: 'Expand Notes', exact: true }).click();
    await resource('Meetings').waitFor();
    await sources.getByRole('button', { name: 'Expand Meetings', exact: true }).click();
    await resource('2026').waitFor();
    ok('the folders recorded inside it can be browsed, dimmed', await resource('2026').evaluate(a => a.parentElement.classList.contains('workspace-resource-missing')));
    await shot('missing-tree');
    const ctx = page.locator('.context-menu');
    await resource('Concepts').hover();
    await resource('Concepts').locator('xpath=..').locator(':scope > [data-row-actions]').click();
    await ctx.waitFor();
    ok('the kept source offers only Remove from Workspace…', (await ctx.locator('.menu-action').allTextContents()).join('|') === 'Remove from Workspace…', (await ctx.locator('.menu-action').allTextContents()).join('|'));
    await page.keyboard.press('Escape');
    await resource('Notes').hover();
    await resource('Notes').locator('xpath=..').locator(':scope > [data-row-actions]').click();
    await ctx.waitFor();
    const labels = await ctx.locator('.menu-action').allTextContents();
    ok('its menu has Create the Folders…, Choose the Folder… and no commands that need the folder', labels.includes('Create the Folders…') && labels.includes('Choose the Folder…') && !labels.includes('Upload Files…') && !labels.includes('Open in Files') && !labels.includes('Pin to Workspace'), labels.join(', '));
    await ctx.locator('.menu-action', { hasText: 'Create the Folders…' }).click();
    await page.locator('#ask[open]').waitFor();
    ok('Create the Folders… asks first, counting the folders and warning about OneDrive', (await page.locator('#ask-body').textContent()).includes('D:/Notes and the 3 folders recorded inside it') && (await page.locator('#ask-body').textContent()).includes('OneDrive'));
    await page.locator('#ask-yes').click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'create' && c[1] === 'w1' && c[2] === 'notes'));
    await page.waitForFunction(() => { const a = [...document.querySelectorAll('#workspace-sources [data-resource]')].find(a => a.textContent.startsWith('Notes')); return a && !a.parentElement.classList.contains('workspace-resource-missing'); });
    ok('once created, the folder is shown as any other', !(await resource('Notes').evaluate(a => !!a.querySelector('.badge'))));

    // A workspace whose main folder is missing is labelled, and choosing it offers to create it.
    await page.locator('#status-workspace').click();
    const oldRow = page.locator('#status-ws-recent [data-workspace-id=w2]');
    ok('a workspace whose main folder is missing is listed as not on this computer yet', (await page.locator('#ws-select option[data-workspace-id=w2]').textContent()).includes('(Not on This Computer Yet)'));
    await oldRow.click();
    await page.locator('#ask[open]').waitFor();
    ok('choosing it offers to create its folders', (await page.locator('#ask-title').textContent()).includes('Old Work') && (await page.locator('#ask-body').textContent()).includes('D:/Old is not on this computer yet'));
    await page.locator('#ask').getByRole('button', { name: 'Cancel' }).click();
    await page.waitForTimeout(200);
    ok('declining leaves it waiting, without opening it', await page.evaluate(() => !__calls.some(c => c[0] === 'select' && c[1] === 'w2') && !__calls.some(c => c[0] === 'create' && c[1] === 'w2')));
    await page.locator('#status-workspace').click();
    await oldRow.click();
    await page.locator('#ask[open]').waitFor();
    await page.locator('#ask-yes').click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'select' && c[1] === 'w2'));
    ok('accepting creates its main folder and opens it', await page.evaluate(() => __calls.some(c => c[0] === 'create' && c[1] === 'w2' && c[2] === 'm2')) && await page.locator('#status-workspace-name').textContent() === 'Old Work');
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
