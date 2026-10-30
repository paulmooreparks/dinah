// Settings › Packages (docs/design/packages.md): Install From a File… asks
// who the package is for, shows the plan and installs on Install; something
// in the way stops an install for the person; an installed package has
// Remove…, which shows what goes. With no package source, as in Dinah's own
// build, nothing offers updates; with one, an update found is offered in
// Notifications and Update Packages… plans the fetched package.
const { launch } = require('./lib/page');
(async () => {
  // Each build is its own launch; the failures and page errors of the one
  // before are carried into it, so the last close reports them all.
  let t;
  const ok = (name, value, detail) => t.ok(name, value, detail);
  async function open(sources) {
    const prev = t;
    if (prev) await prev.browser.close();
    const record = { id: 'w1', name: 'SampleLab', mountAlias: 'SampleLab', executionPath: 'C:/Fixture', primarySourceId: 'main', sources: [{ id: 'main', kind: 'local-folder', name: 'SampleLab', mountAlias: 'SampleLab', path: 'C:/Fixture' }] };
    t = await launch({
      viewport: { width: 1440, height: 1000 },
      state: { workspaceId: 'w1', workspace: 'C:/Fixture', workspaceRecords: [record], recent: ['C:/Fixture'], workspaces: ['C:/Fixture'], defaultProvider: 'devin', packageSources: sources },
      app: () => {
        window.__calls = []; window.__conflict = false; window.__installed = [];
        const plan = extra => Object.assign({ name: 'team-setup', from: '', to: '2', added: 3, updated: 0, removed: 0, unchanged: 0, changed: [], warnings: [], token: 't' }, extra);
        return {
          AgentRestart: async provider => { __calls.push(['agent-restart', provider]); },
          InstalledPackages: async () => window.__installed,
          ChoosePackageBundle: async () => { __calls.push(['choose']); return 'C:/Downloads/team-setup-v2.zip'; },
          PlanPersonalPackages: async (ws, zip) => { __calls.push(['plan-personal', ws, zip]); return plan({ home: 'C:/Users/me/.dinah/packages/team-setup', conflicts: window.__conflict ? ['C:/Users/me/AppData/Roaming/devin/skills/review'] : [],
            duplicates: ['review', 'summarise'], homeOnly: ['.devin/flows/', '.devin/rules/'], copied: 2 }); },
          InstallPersonalPackages: async (ws, zip, token) => { __calls.push(['install-personal', ws, zip, token]); window.__installed = [{ target: 'me', name: 'team-setup', version: '2' }]; window.__emit('packages:changed'); return plan({ backup: '' }); },
          PlanPackages: async (ws, zip) => plan({ from: '1', updated: 2, changed: ['.devin/rules/x.md'] }),
          InstallPackages: async (ws, zip, token) => { __calls.push(['install', ws, zip]); window.__installed = window.__installed.concat([{ target: 'workspace', workspace: 'C:/Fixture', name: 'team-setup', version: '2' }]); return plan({ from: '1', backup: 'C:/Users/me/setup-backups/x' }); },
          PlanRemovePackages: async (target, ws, name) => { __calls.push(['plan-remove', target, ws, name]); return plan({ from: '2', to: '', added: 0, removed: 4, token: 'r', home: 'C:/Users/me/.dinah/packages/team-setup', conflicts: [], duplicates: [], homeOnly: [] }); },
          RemovePackages: async (target, ws, name, token) => { __calls.push(['remove', target, ws, name, token]); window.__installed = window.__installed.filter(p => p.target !== target); return { name: 'team-setup', backup: 'C:/Users/me/setup-backups/y' }; },
          FindPackageUpdates: async () => ({ installed: [{ target: 'me', name: 'team-setup', version: '2' }],
            updates: [{ target: 'me', name: 'team-setup', from: '2', to: '3', bundle: 'C:/Users/me/.dinah/package-updates/team-setup-v3.zip', reason: 'files of the package were changed by hand' }], available: [] })
        };
      },
      ready: () => window.dinahWorkbench && window.dinahCommands && document.documentElement.dataset.uiStateReady === 'true',
    });
    if (prev) { t.errors.push(...prev.errors); t.failures.push(...prev.failures); }
    const page = t.page;
    await page.evaluate(() => dinahWorkbench.open('settings'));
    await page.locator('#settings').getByRole('tab', { name: 'Packages', exact: true }).click();
    return page;
  }
  const shown = (page, text) => page.waitForFunction(t => document.body.innerText.includes(t), text, { timeout: 5000 }).then(() => true, () => false);
  const shot = (page, name) => t.shot('packages-' + name);
  try {
    // Dinah's own build has no package source.
    let page = await open(0);
    const panel = page.locator('#settings-packages');
    ok('with no package source there is no Update Packages… in Settings', await panel.locator('[data-cmd="update-packages"]').isHidden() && await panel.getByRole('button', { name: 'Update Packages…' }).count() === 0);
    ok('with no package source the main menu has no Update Packages…', await page.evaluate(() => !dinahCommands.rows('dinah').some(r => /Update Packages/.test(r.label))));
    ok('with nothing installed the panel says so and offers no Remove…', await panel.locator('#packages-none').isVisible() && await panel.getByRole('button', { name: /Remove/ }).count() === 0);

    // Install From a File… asks who it is for, for the person first, then
    // shows the plan and installs on Install.
    const install = () => panel.getByRole('button', { name: 'Install From a File…' }).click();
    await install();
    const target = page.locator('dialog[open][aria-labelledby="install-target-title"]');
    await target.waitFor();
    ok('installing asks who the package is for, for the person first', await target.locator('#install-target').inputValue() === 'me');
    await target.getByRole('button', { name: 'Choose the Package…' }).click();
    const plan = page.locator('dialog[open].install-plan');
    await plan.waitFor();
    const text = await plan.textContent();
    ok('the plan for the person says where, which skills the workspace has too, and what stays in the package\'s folder',
      (await plan.locator('.dialog-title').textContent()) === 'Install team-setup for You?' && (await plan.locator('.install-plan-facts').textContent()).includes('C:/Users/me/.dinah/packages/team-setup') &&
      await plan.locator('.install-plan-names li').count() === 2 && text.includes('Kept in the Package\'s Folder') && text.includes('.devin/flows/'));
    await shot(page, 'plan');
    await plan.getByRole('button', { name: 'Install' }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'install-personal' && c[2].endsWith('team-setup-v2.zip') && c[3] === 't'));
    ok('Install installs for the person and says every workspace has the skills', await shown(page, 'for you. Every workspace now has its skills.'));
    ok('an install restarts the agent so it reads the new skills', await page.evaluate(() => __calls.some(c => c[0] === 'agent-restart')));
    await panel.getByRole('button', { name: /Remove team-setup for you/ }).waitFor();
    ok('the installed package is listed with Remove…', (await panel.locator('#packages-installed').textContent()).includes('team-setup version 2, for you, in every workspace.') && await panel.locator('#packages-none').isHidden());

    // Something in the way refuses the install for the person.
    await page.evaluate(() => { window.__conflict = true; });
    await install();
    await target.waitFor();
    await target.getByRole('button', { name: 'Choose the Package…' }).click();
    ok('something in the way that the program did not install stops the install', await shown(page, 'will not overwrite them') &&
      await page.evaluate(() => __calls.filter(c => c[0] === 'install-personal').length === 1));

    // This Workspace Only plans and installs in the workspace in front.
    await install();
    await target.waitFor();
    await target.locator('#install-target').selectOption('workspace');
    await target.getByRole('button', { name: 'Choose the Package…' }).click();
    await plan.waitFor();
    ok('the plan for the workspace says what changes, the hand-changed files among it', (await plan.locator('.dialog-title').textContent()) === 'Install team-setup in This Workspace?' &&
      (await plan.locator('.install-plan-facts').textContent()).includes('1 → 2') && (await plan.textContent()).includes('.devin/rules/x.md'));
    await plan.getByRole('button', { name: 'Install' }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'install' && c[1] === 'C:/Fixture'));
    ok('Install installs in the workspace and says where replaced files are kept', await shown(page, 'Replaced files are kept in C:/Users/me/setup-backups/x'));

    // Remove… beside a package shows what goes, with a danger button.
    await panel.getByRole('button', { name: /Remove team-setup for you/ }).click();
    await plan.waitFor();
    ok('removing shows what goes, with a danger button', (await plan.locator('.dialog-title').textContent()) === 'Remove team-setup for You?' &&
      (await plan.locator('.install-plan-facts').textContent()).includes('4 removed') && await plan.locator('.btn-danger').textContent() === 'Remove');
    await plan.getByRole('button', { name: 'Remove' }).click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'remove' && c[1] === 'me' && c[3] === 'team-setup' && c[4] === 'r'));
    ok('Remove removes the package and says where the copies are', await shown(page, 'Removed team-setup for you.'));

    // A build with a package source offers updates.
    page = await open(1);
    const panel2 = page.locator('#settings-packages');
    ok('with a package source Settings has Update Packages…', await panel2.getByRole('button', { name: 'Update Packages…' }).isVisible());
    ok('with a package source the main menu has Update Packages…', await page.evaluate(() => dinahCommands.rows('dinah').some(r => r.label === 'Update Packages…')));
    await page.evaluate(() => __emit('packages:update-available', { target: 'me', name: 'team-setup', from: '2', to: '3', bundle: 'C:/Users/me/.dinah/package-updates/team-setup-v3.zip', reason: 'files of the package were changed by hand' }));
    const offer = page.locator('.package-update');
    await offer.first().waitFor({ state: 'attached' });
    ok('an update is offered under Notifications, with why', (await offer.first().textContent()).includes('Version 3 of team-setup is available.') &&
      (await offer.first().textContent()).includes('changed by hand'));
    await panel2.getByRole('button', { name: 'Update Packages…' }).click();
    const updates = page.locator('dialog[open]#package-update');
    await updates.getByRole('button', { name: 'Update…' }).waitFor();
    ok('Update Packages says what is installed, what is new and what is in its way', (await updates.textContent()).includes('team-setup version 2, for you, in every workspace: version 3 is available.') &&
      (await updates.textContent()).includes('changed by hand'));
    await shot(page, 'updates');
    await updates.getByRole('button', { name: 'Update…' }).click();
    await page.locator('dialog[open].install-plan').waitFor();
    ok('its Update… plans the fetched package without asking for a file', await page.evaluate(() => __calls.some(c => c[0] === 'plan-personal' && c[2].endsWith('team-setup-v3.zip')) && !__calls.some(c => c[0] === 'choose')));
    await page.locator('dialog[open].install-plan').getByRole('button', { name: 'Cancel' }).click();
  } catch (e) {
    console.log('FAIL ' + (e && e.stack || e));
    if (t) t.failures.push('exception');
  } finally {
    if (t) await t.close();
  }
})();
