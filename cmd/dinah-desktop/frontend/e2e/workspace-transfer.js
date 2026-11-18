// Export Workspace… and Import Workspace… (docs/design/workspace-export.md):
// the export window with its chats, their total and what goes of their
// groups and teams, the import window's sources and choices, a source of
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
      // The export plan has the shape PlanWorkspaceExport gives
      // (workspace_export_chats.go): sizes in bytes, members with here set
      // for the plan's own chats, and a lead with the members it leads.
      const mb = n => Math.round(n * 1048576);
      const member = (chatId, title, here, workspace) => ({ chatId, title, workspace: workspace || 'SampleLab', here });
      const lead = member('chat-lead', 'Harbor Lead', true), review = member('chat-review', 'Checkout Review', true),
        research = member('chat-research', 'Keyboard Research', true), builder = member('chat-builder', 'Harbor Builder', true),
        docks = member('chat-docks', 'Docks Builder', false, 'Docks');
      const exportPlan = { workspaceId: 'w1', name: 'SampleLab', pins: 1,
        sources: [{ name: 'SampleLab', kind: 'local-folder', main: true }, { name: 'Notes', kind: 'local-folder', main: false }, { name: 'Concepts', kind: 'web-page', main: false }],
        chats: [
          { chatId: 'chat-lead', title: 'Harbor Lead', provider: 'claude', providerName: 'Claude', model: 'Opus 5.5', size: mb(41.2), open: true, hidden: false },
          { chatId: 'chat-review', title: 'Checkout Review', provider: 'codex', providerName: 'Codex', model: 'GPT-5.4', size: mb(12.8), open: true, hidden: false },
          { chatId: 'chat-research', title: 'Keyboard Research', provider: 'claude', providerName: 'Claude', model: 'Sonnet 5.5', size: mb(23.5), open: true, hidden: false },
          { chatId: 'chat-builder', title: 'Harbor Builder', provider: 'devin', providerName: 'Devin', model: 'Adaptive', size: mb(66.0), open: true, hidden: false },
          { chatId: 'chat-notes', title: 'Release Notes', provider: 'claude', providerName: 'Claude', model: 'Haiku 5.5', size: mb(3.1), open: false, hidden: true }],
        groups: [{ title: 'Harbor Team', members: [lead, review, research, builder, docks] }],
        teams: [{ lead, members: [review, research, builder] }] };
      // Import plans with chats, groups and teams, in the shape PlanWorkspaceImport
      // gives (workspace_import_chats.go, workspace_import_groups.go).
      const harborSource = { key: 's1', kind: 'local-folder', name: 'Harbor', main: true, path: 'C:/Users/sam/OneDrive/Harbor', found: true, folders: 0, checked: true, actions: ['choose-folder'] };
      window.__laptopPlan = () => ({ file: 'C:/Downloads/Harbor.dinah-workspace', name: 'Harbor', pins: 0, uncheckedPins: 0, sources: [harborSource],
        chats: [
          { key: 'c1', chatId: 'chat-lead', title: 'Harbor Lead', provider: 'claude', providerName: 'Claude', model: 'Opus 5.5', mark: 'new' },
          { key: 'c2', chatId: 'chat-review', title: 'Checkout Review', provider: 'codex', providerName: 'Codex', model: 'GPT-5.4', mark: 'new', providerMissing: true }],
        groups: [{ title: 'Harbor Team', existing: false, inFile: 4, here: 0, waiting: [{ title: 'Docks Builder', workspace: 'Docks' }], messages: 312, new: 312 }],
        teams: [{ lead: 'Harbor Lead', members: ['Checkout Review', 'Keyboard Research', 'Harbor Builder'], existing: false, waiting: [] }],
        links: [{ a: 'Docks', b: 'Harbor' }] });
      window.__backPlan = () => ({ file: 'C:/Downloads/Harbor.dinah-workspace', name: 'Harbor', existingId: 'w1', existingName: 'Harbor', pins: 0, uncheckedPins: 0, sources: [harborSource],
        chats: [
          { key: 'c1', chatId: 'chat-lead', title: 'Harbor Lead', provider: 'claude', providerName: 'Claude', model: 'Opus 5.5', mark: 'updated' },
          { key: 'c2', chatId: 'chat-review', title: 'Checkout Review', provider: 'claude', providerName: 'Claude', model: 'Opus 5.5', mark: 'up-to-date' },
          { key: 'c3', chatId: 'chat-research', title: 'Keyboard Research', provider: 'claude', providerName: 'Claude', model: 'Sonnet 5.5', mark: 'changed-on-both' },
          { key: 'c4', chatId: 'chat-builder', title: 'Harbor Builder', provider: 'claude', providerName: 'Claude', model: 'Opus 5.5', mark: 'close-to-update' }],
        groups: [{ title: 'Harbor Team', existing: true, inFile: 4, here: 0, waiting: [], messages: 330, new: 18, keptLead: 'Pier Lead' }],
        teams: [], links: [] });
      const App = {
        State: async () => state(),
        PlanWorkspaceExport: async id => { __calls.push(['plan-export', id]); return JSON.parse(JSON.stringify(exportPlan)); },
        ExportWorkspace: async (id, choice) => { __calls.push(['export', id, choice]); return window.__cancelSave ? '' : 'D:/SampleLab/SampleLab.dinah-workspace'; },
        ChooseWorkspaceImport: async () => { __calls.push(['choose-import']); return window.__nextImport ? window.__nextImport() : plan(); },
        PlanWorkspaceImport: async (file, choices) => { __calls.push(['plan', file, choices]); return plan(choices); },
        ChooseImportFolder: async start => { __calls.push(['choose-folder', start]); return 'E:/Elsewhere/Notes'; },
        ImportWorkspace: async (file, choices) => { __calls.push(['import', file, choices]); return Object.assign({ state: state(), workspaceId: 'w1', existing: false, sources: 4, pins: 2, waiting: 1 }, window.__importResult || {}); },
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

    // Export Workspace… first shows the export window, with every chat ticked.
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu [data-cmd=export-workspace]').click();
    const exp = page.locator('dialog[open]#workspace-export');
    await exp.waitFor();
    const chat = id => exp.locator('.import-source[data-chat-id="' + id + '"]');
    const text = async sel => (await page.locator(sel).textContent()).trim();
    ok('Export Workspace… plans the export of the workspace in front before any file is chosen', await page.evaluate(() => __calls.some(c => c[0] === 'plan-export' && c[1] === 'w1') && !__calls.some(c => c[0] === 'export')));
    ok('the export window says what it saves and which sources and pins go', (await exp.textContent()).includes('This saves the workspace "SampleLab" to a file that Import Workspace… reads on another computer.') &&
      await text('#workspace-export-sources') === '3 sources and 1 pin go with it: the main folder SampleLab, the folder Notes and the source Concepts.', await text('#workspace-export-sources'));
    ok('every chat is listed and ticked', await exp.locator('#workspace-export-chats input[type=checkbox]').count() === 5 && await exp.locator('#workspace-export-chats input[type=checkbox]:checked').count() === 5);
    ok('a chat shows its program, model and size', (await chat('chat-lead').textContent()).includes('Claude · Opus 5.5') && (await chat('chat-lead').locator('.import-size').textContent()) === '41.2 MB');
    ok('a closed chat and a hidden chat are marked', (await chat('chat-notes').textContent()).includes('Closed') && (await chat('chat-notes').textContent()).includes('Hidden') && !(await chat('chat-lead').textContent()).includes('Closed'));
    ok('the total counts every ticked chat', await text('#workspace-export-total') === '5 chats with 146.6 MB of history.', await text('#workspace-export-total'));
    const lines = async () => (await page.locator('#workspace-export-groups p').allTextContents()).map(s => s.trim());
    let said = await lines();
    ok('the group line counts the members here and names the one in another workspace', said[0] === 'The group Harbor Team goes with them, with its conversation: 4 members are in this workspace, and Docks Builder is in the workspace Docks.', said[0]);
    ok('the team line says who leads it', said[1] === 'Harbor Lead leads a team of 4.', said[1]);
    ok('the window warns that the file holds the conversations', (await text('#workspace-export-private')).includes('Keep it as private as the chats.'));
    await shot('export');
    const warningAt = async () => (await page.locator('#workspace-export-private').boundingBox()).y;
    const before = await warningAt();
    await chat('chat-lead').getByRole('checkbox').uncheck();
    said = await lines();
    ok('unticking a chat takes it off the total', await text('#workspace-export-total') === '4 chats with 105.4 MB of history.', await text('#workspace-export-total'));
    ok('the group line counts one member fewer', said[0] === 'The group Harbor Team goes with them, with its conversation: 3 members are in this workspace, and Docks Builder is in the workspace Docks.', said[0]);
    ok('without its lead the team goes without its budget', said[1] === 'Harbor Lead stays behind, so its team\'s budget does not go. 3 of its members go and rejoin it on a computer that has Harbor Lead.', said[1]);
    ok('ticking does not move the warning', await warningAt() === before);
    for (const id of ['chat-review', 'chat-research', 'chat-builder']) await chat(id).getByRole('checkbox').uncheck();
    said = await lines();
    ok('with none of its members ticked the group does not go', said[0] === 'The group Harbor Team does not go, because none of its members is ticked.' && said[1] === 'Harbor Lead\'s team does not go, because none of its chats here is ticked.', said.join(' | '));
    ok('the warning still has not moved', await warningAt() === before);
    await chat('chat-lead').getByRole('checkbox').check();
    await page.evaluate(() => { window.__cancelSave = true; });
    await exp.getByRole('button', { name: 'Export…' }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'export'));
    ok('cancelling the save window leaves the export window open as it was', await page.locator('dialog[open]#workspace-export').count() === 1 && await exp.locator('#workspace-export-chats input:checked').count() === 2);
    await page.evaluate(() => { window.__cancelSave = false; __calls.length = 0; });
    await exp.getByRole('button', { name: 'Export…' }).click();
    await page.locator('#workspace-export').waitFor({ state: 'hidden' });
    ok('Export… exports the workspace with the ticked chats and says where the file is', await page.evaluate(() => { const c = __calls.find(c => c[0] === 'export'); return c && c[1] === 'w1' && JSON.stringify(c[2]) === JSON.stringify({ chats: ['chat-lead', 'chat-notes'] }); }) &&
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

    // Import on the laptop: the chats are new, and the group and team come with them.
    const openImport = async () => {
      await page.evaluate(() => { document.querySelectorAll('[popover]:popover-open').forEach(p => p.hidePopover()); document.querySelector('#workspace-status-menu [data-cmd=import-workspace]').click(); });
      await win.waitFor();
    };
    await page.evaluate(() => { window.__nextImport = window.__laptopPlan; window.__importResult = { existing: false, sources: 1, pins: 0, waiting: 0, chats: 2, groups: 1, messages: 312 }; __calls.length = 0; });
    await openImport();
    const chatRow = key => win.locator('#workspace-import-chats .import-source[data-chat-key="' + key + '"]');
    const groupRows = () => win.locator('#workspace-import-groups .import-source');
    ok('the import window says it makes the workspace with its chats', (await page.locator('#workspace-import-summary').textContent()) === 'This makes the workspace "Harbor" with these sources and chats.');
    ok('each chat is listed as New with its program and model', (await chatRow('c1').textContent()).includes('New') && (await chatRow('c1').textContent()).includes('Claude · Opus 5.5'));
    ok('a chat whose program is not here says so', (await chatRow('c2').textContent()).includes('Codex is not on this computer. The chat opens on its saved history and asks which program continues it.'));
    const groupText = await groupRows().nth(0).textContent();
    ok('a new group says which members are in the file and which wait for their workspace', groupText.includes('New Group') && groupText.includes('4 members are in this file.') &&
      groupText.includes('Docks Builder waits for the workspace Docks; importing Docks adds it.') && groupText.includes('Its conversation comes too: 312 messages.'), groupText);
    const teamText = await groupRows().nth(1).textContent();
    ok('a team says who reports to its lead', teamText.includes("Harbor Lead's team") && teamText.includes('New Team') && teamText.includes('Checkout Review, Keyboard Research and Harbor Builder report to Harbor Lead.'), teamText);
    ok('the trusted link is listed as not imported', (await page.locator('#workspace-import-links').textContent()).includes('Not imported: the trusted link between Docks and Harbor. A file cannot grant agents that trust'));
    await shot('import-chats');
    await win.getByRole('button', { name: 'Import', exact: true }).click();
    await page.locator('#workspace-import').waitFor({ state: 'hidden' });
    ok('Import says how many chats and groups came', await shown('Imported the workspace "Harbor" with 2 chats and 1 group.'));

    // Import back on the desktop: the chats already here are marked, and a chat changed on both can be added separately.
    await page.evaluate(() => { window.__nextImport = window.__backPlan; window.__importResult = { existing: true, sources: 0, pins: 0, waiting: 0, chats: 0, updated: 1, separate: 1, messages: 18 }; __calls.length = 0; });
    await openImport();
    ok('the window says it adds to the workspace and updates chats worked on elsewhere', (await page.locator('#workspace-import-summary').textContent()).includes('adds what it does not have and updates chats worked on elsewhere') &&
      (await win.locator('#workspace-import-go').textContent()) === 'Add to the Workspace');
    ok('each chat already here has its mark', (await chatRow('c1').textContent()).includes('Updated From the Other Computer') && (await chatRow('c2').textContent()).includes('Already Up to Date') &&
      (await chatRow('c3').textContent()).includes('Changed on Both Computers') && (await chatRow('c4').textContent()).includes('Close It to Update It'));
    ok('only a chat changed on both offers Import as a Separate Chat, off', await win.getByLabel('Import as a Separate Chat').count() === 1 && !(await chatRow('c3').getByLabel('Import as a Separate Chat').isChecked()));
    const existing = await groupRows().nth(0).textContent();
    ok('a group here already says what it gains and which lead it keeps', existing.includes('Joins the Existing Group') && existing.includes('All its members are here already.') &&
      existing.includes('18 newer messages are added to its conversation.') && existing.includes('It keeps its lead here, Pier Lead'), existing);
    const groupsAt = async () => (await page.locator('#workspace-import-groups').boundingBox()).y;
    const groupsBefore = await groupsAt();
    await chatRow('c3').getByLabel('Import as a Separate Chat').check();
    ok('ticking it says what is added, and moves nothing', (await chatRow('c3').textContent()).includes('It is added as "Keyboard Research (Imported)"') && await groupsAt() === groupsBefore);
    await shot('import-back');
    await win.getByRole('button', { name: 'Add to the Workspace' }).click();
    await page.locator('#workspace-import').waitFor({ state: 'hidden' });
    ok('the import is sent with the separate chat chosen', await page.evaluate(() => { const c = __calls.find(c => c[0] === 'import'); return c && c[2].some(x => x.chat === 'c3' && x.separate === true); }));
    ok('the toast says what was updated and added', await shown('Updated 1 chat worked on elsewhere. Added 1 separate chat. Added 18 group messages.'));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
