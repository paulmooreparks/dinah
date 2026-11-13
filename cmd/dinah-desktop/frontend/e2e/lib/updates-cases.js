const { start, settle, setRelease, ledger, geometry, finish } = require('./updates-fixture');
const path = require('path');

module.exports = async function run(name) {
  const t = await start(name === 'loading' ? { initialHold: { Status: true, Check: true, InstalledPackages: true } } : name === 'missing-services' ? { unsupported: ['Status', 'PreviewAgentSettings', 'RepairAgentSettings'] } : {});
  const { page, ok } = t;
  const text = id => page.locator('#' + id).textContent();
  try {
    if (name === 'loading') {
      await page.waitForFunction(() => __pending.Status && __pending.Check && __pending.InstalledPackages);
      ok('initial load identifies pending sections', (await text('updates-inventory-status')).includes('Reading') && (await text('updates-release-status')).includes('Checking') && (await text('updates-package-status')).includes('Reading'));
      ok('pending inventory does not claim no running programs', (await text('updates-running-status')).includes('not been read'));
      ok('opening never installs or repairs', !(await ledger(t)).some(c => ['Download', 'RepairAgentSettings', 'InstallPersonalPackages'].includes(c[0])));
      const before = await geometry(t);
      await page.evaluate(() => { __hold = {}; Object.values(__pending).flat().forEach(resolve => resolve()); });
      await settle(t);
      ok('loading and settled geometry are identical', JSON.stringify(before) === JSON.stringify(await geometry(t)));
      // A newer reopening wins even when the earlier reading completes last.
      await page.evaluate(() => { __hold.Status = true; });
      await page.locator('#updates-refresh').click();
      await page.waitForFunction(() => __pending.Status.length === 2);
      await page.locator('#settings-tab-appearance').click();
      await page.evaluate(() => { __hold.Status = false; __u.status.installed[0].version = '2.0.2'; });
      await page.locator('#settings-tab-updates').click(); await settle(t);
      await page.evaluate(() => __pending.Status[1]());
      await page.waitForTimeout(50);
      ok('late response cannot replace newer inventory', (await text('updates-programs')).includes('2.0.2'));
    } else {
      await settle(t);
      if (name === 'current') {
        ok('installed and running versions appear', (await text('updates-programs')).includes('Installed: 2.0.0') && (await text('updates-programs')).includes('Running: 2.0.0'));
        ok('source suffix remains visible without a mismatch claim', (await text('updates-programs')).includes('2.0.0+source.abcdef') && !(await text('updates-inventory-status')).includes('mismatch'));
        ok('current state has no stale restart warning', (await text('updates-running-status')).startsWith('0 older'));
        const selfBefore = await geometry(t);
        await page.evaluate(() => { __u.status.running[0].self = true; });
        await page.locator('#updates-refresh').click(); await settle(t);
        ok('only the caller is labelled this window without changing geometry', await page.locator('#updates-running [data-key="100"]').textContent().then(s => s.includes('PID 100 (this window): installed copy')) && (await text('updates-running')).split('(this window)').length === 2 && JSON.stringify(selfBefore) === JSON.stringify(await geometry(t)));
        await page.evaluate(() => { __u.status.running[0].current = false; __u.status.behind = 1; });
        await page.locator('#updates-refresh').click(); await settle(t);
        ok('an older caller keeps its this window label and restart explanation', (await page.locator('#updates-running [data-key="100"]').textContent()).includes('(this window): older copy; restart its owner when ready'));
        await page.evaluate(() => { __u.status.running[0].current = true; __u.status.behind = 0; });
        ok('Settings commands preserve PUDL disabled action meaning', await page.evaluate(() => pudlApplets.commandsIn(document.querySelector('#settings').closest('.win')).some(c => c.label === 'Install Update' && c.disabled)));
        await setRelease(t, { current: '2.0.1', latest: '2.0.1', available: false });
        ok('already current is explicit', (await text('updates-release-status')).includes('No newer release') && await page.locator('#updates-install').isDisabled());
        await t.shot('updates-current');
        await page.evaluate(() => { __u.status.running.reverse(); });
        const ids = await page.locator('#updates-running > div').evaluateAll(es => es.map(e => e.dataset.key));
        await page.locator('#updates-running .updates-row-text').first().focus();
        await page.locator('#updates-refresh').evaluate(el => el.click()); await settle(t);
        ok('reordering process inventory retains row identity and focus', JSON.stringify(ids) === JSON.stringify(await page.locator('#updates-running > div').evaluateAll(es => es.map(e => e.dataset.key))) && await page.locator('#updates-running .updates-row-text').first().evaluate(el => el === document.activeElement));
        await page.evaluate(() => { __u.status.running = [__u.status.running[0], {pid:999,path:'C:/Fixture/tools.exe',version:'unknown',current:false}]; });
        await page.locator('#updates-refresh').click(); await settle(t);
        ok('removed rows keep place and new processes append', (await text('updates-running')).includes('No longer reported') && (await page.locator('#updates-running > div').last().getAttribute('data-key')) === '999');
      } else if (name === 'available') {
        const before = await geometry(t);
        await setRelease(t, { latest: '2.0.1', available: true, notes: '<img src=x onerror=alert(1)>\nA much longer explanation. '.repeat(500) });
        ok('available release shows both versions and permits install', (await text('updates-release-status')).includes('2.0.1') && (await text('updates-release-status')).includes('2.0.0') && await page.locator('#updates-install').isEnabled());
        ok('notes are inert text', await page.locator('#updates-notes img').count() === 0 && (await text('updates-notes')).includes('<img'));
        ok('long notes do not move or resize controls', JSON.stringify(before) === JSON.stringify(await geometry(t)));
        await t.shot('updates-available');
        await setRelease(t, { latest: '2.0.1', available: true, notes: '' });
        ok('missing notes are reported without suppressing release', (await text('updates-notes')).includes('unavailable') && await page.locator('#updates-install').isEnabled());
        ok('viewing notes performs no mutation', !(await ledger(t)).some(c => c[0] === 'Download'));
        await page.setViewportSize({ width: 560, height: 800 });
        ok('narrow Settings has no horizontal page overflow', await page.locator('#settings-updates').evaluate(el => el.scrollWidth <= el.clientWidth + 1));
        await page.locator('#updates-check').focus();
        ok('keyboard focus reaches named controls', await page.locator('#updates-check').evaluate(el => el === document.activeElement) && await page.locator('#updates-install').isEnabled());
        await page.locator('#settings-tab-appearance').click();
        await page.locator('#settings-theme [data-theme-choice="dark"]').click();
        await page.locator('#settings-tab-updates').click(); await settle(t);
        ok('dark theme retains text and control names', await page.evaluate(() => pudlThemePreference() === 'dark') && (await text('updates-install')) === 'Install Update');
        await t.shot('updates-narrow');
      } else if (name === 'installing') {
        await setRelease(t, { latest: '2.0.1', available: true, notes: 'A synthetic change.' });
        const before = await geometry(t);
        await page.evaluate(() => { __hold.Download = true; });
        await page.locator('#updates-install').evaluate(el => { el.focus(); el.click(); el.click(); });
        await page.waitForFunction(() => __pending.Download);
        ok('double activation admits one download', (await ledger(t)).filter(c => c[0] === 'Download').length === 1);
        ok('install focus and geometry are preserved', await page.locator('#updates-install').evaluate(el => el === document.activeElement) && JSON.stringify(before) === JSON.stringify(await geometry(t)));
        await page.evaluate(() => { __hold.Download = false; __pending.Download[0](); });
        await page.waitForFunction(() => document.querySelector('#updates-release-status').textContent.includes('Downloading'));
        const pointer = await page.evaluate(() => { const r=document.querySelector('#updates-install').getBoundingClientRect();return [r.x,r.y,r.width,r.height,document.querySelector('#settings-updates').scrollTop]; });
        await page.evaluate(() => { __u.release.received = 200; __u.release.size = 100; });
        await page.waitForFunction(() => document.querySelector('#updates-progress').max === 100);
        ok('known-size progress is clamped', await page.locator('#updates-progress').evaluate(el => el.value === 100));
        ok('live progress preserves screen pointer target and panel scroll', JSON.stringify(pointer) === JSON.stringify(await page.evaluate(() => { const r=document.querySelector('#updates-install').getBoundingClientRect();return [r.x,r.y,r.width,r.height,document.querySelector('#settings-updates').scrollTop]; })));
        await t.shot('updates-downloading');
        await page.evaluate(() => { __u.release.size = 0; });
        await page.waitForFunction(() => !document.querySelector('#updates-progress').hasAttribute('value'));
        ok('unknown-size progress is indeterminate', (await text('updates-progress-text')).includes('not known'));
        // A receiving product can exercise its existing entry point without
        // putting its filenames or page globals into the shared suite.
        if (process.env.DINAH_UPDATE_ENTRY_SCRIPT) {
          await page.addScriptTag({ path: path.resolve(process.env.DINAH_UPDATE_ENTRY_SCRIPT) });
          await page.evaluate(key => window[key].open(), process.env.DINAH_UPDATE_ENTRY_GLOBAL);
          const dialog = page.locator(process.env.DINAH_UPDATE_ENTRY_DIALOG);
          ok('existing update dialog observes the ongoing download', (await dialog.textContent()).includes('being downloaded') && await dialog.locator('button').count() === 1);
          await dialog.locator('button').click();
        } else {
          await page.evaluate(() => dinahServices.object('updates').Download());
          ok('another entry point observes the same service-owned operation', (await page.evaluate(() => __u.downloads)) === 1);
        }
        await page.evaluate(() => { pudlWindows.close(document.querySelector('#settings').closest('[data-win]').dataset.win); });
        await page.evaluate(() => { __u.release = { current:'2.0.0',latest:'2.0.1',available:true,ready:true,installed:true,downloading:false }; });
        await page.evaluate(() => dinahWorkbench.open('settings'));
        await page.locator('#settings-tab-updates').click(); await settle(t);
        ok('reopening observes service-owned completion without another download', (await text('updates-release-status')).includes('is installed') && (await page.evaluate(() => __u.downloads)) === 1);
      } else if (name === 'installed') {
        await setRelease(t, { latest: '2.0.1', available: true, ready: true, installed: false });
        ok('prepared and installed are distinct', (await text('updates-release-status')).includes('has not been installed') && await page.locator('#updates-install').isEnabled());
        await page.evaluate(() => {
          __u.status.installed.forEach(p => {p.version='2.0.1';});
          __u.status.running = [
            {pid:1,path:'C:/Fixture/desktop.exe.old',version:'2.0.0',current:false,startedBy:'explorer.exe'},
            {pid:2,path:'C:/Fixture/tools.exe.old',version:'2.0.0',current:false,startedBy:'client-a.exe'},
            {pid:3,path:'C:/Fixture/tools.exe.old',version:'2.0.0+old.source',current:false,startedBy:'client-b.exe'}];
          __u.status.behind=3;
        });
        await setRelease(t, { latest: '2.0.1', available: true, installed: true });
        await page.locator('#updates-refresh').click(); await settle(t);
        ok('installed release leaves older desktop and agent owners visible', (await text('updates-release-status')).includes('is installed') && (await text('updates-running')).includes('client-a.exe') && (await text('updates-running')).includes('client-b.exe') && (await text('updates-running')).includes('older copy'));
        ok('installed release cannot install itself again', await page.locator('#updates-install').isDisabled());
        const before = await geometry(t);
        await t.emit('update:state', {ready:true,pending:true,working:2,writing:1});
        ok('restart wait includes active document writes without layout changes', (await text('updates-restart-status')).includes('2 active chats; 1 document writes') && JSON.stringify(before) === JSON.stringify(await geometry(t)));
        await page.evaluate(() => { __u.status.runningError = 'Process inventory unavailable'; });
        await page.locator('#updates-refresh').click(); await settle(t);
        ok('running-inventory failure preserves installed versions and marks unknown', (await text('updates-programs')).includes('Installed: 2.0.1') && (await text('updates-running-status')).includes('unknown'));
      } else if (name === 'unavailable') {
        for (const reason of ['No configured release source.', 'Development builds do not update themselves.', 'This copy was not started from the installed path.']) {
          await setRelease(t, { reason });
          ok('explicit limitation remains visible: ' + reason, (await text('updates-release-status')) === reason && await page.locator('#updates-install').isDisabled());
        }
        await setRelease(t, { latest: '', available: false });
        ok('no published release does not claim latest', (await text('updates-release-status')).includes('No published release') && !(await text('updates-release-status')).includes('latest version'));
        await page.evaluate(() => { __u.sources=0; __u.installed=[]; __u.status.running=[]; __u.status.behind=0; });
        await page.locator('#settings-tab-appearance').click(); await page.locator('#settings-tab-updates').click(); await settle(t);
        ok('missing package source is explicit', (await text('updates-package-status')).includes('No package update source') && await page.locator('#updates-package-check').isDisabled());
        ok('empty running result is a successful explicit observation', (await text('updates-running-status')).includes('No running copies were reported'));
      } else if (name === 'errors') {
        await page.evaluate(() => { __fail.Check='Synthetic release check failed'; });
        await page.locator('#updates-check').click(); await settle(t);
        ok('failed check preserves the last successful reading visibly', (await text('updates-release-status')).includes('last successful') && await page.locator('#updates-install').isDisabled());
        await page.evaluate(() => { __fail.Check=''; });
        const cases = ['Download failed before placement', 'Candidate refused: missing component', 'Reported-version mismatch', 'Placement failed; previous components restored', 'Placement and restore failed; installed releases are mixed'];
        for (const error of cases) {
          await setRelease(t, { latest:'2.0.1',available:true,error });
          ok('failure reason is retained: ' + error, (await text('updates-release-status')).includes(error) && !(await text('updates-release-status')).includes('is installed'));
        }
        await page.evaluate(() => { __u.status.installed[1].version='2.0.1'; __u.status.installed[2].version=''; __u.status.installed[2].error='version query failed'; });
        await page.locator('#updates-refresh').click(); await settle(t);
        ok('mixed and unknown installed components cannot hide behind success', (await text('updates-programs')).includes('version query failed') && (await text('updates-inventory-status')).includes('unidentifiable'));
        await t.shot('updates-mixed-installation');
        await page.evaluate(() => { __fail.Status='Synthetic inventory failure'; });
        await page.locator('#updates-refresh').click(); await settle(t);
        ok('failed inventory retains data as an old reading', (await text('updates-inventory-status')).includes('last successful reading'));
        await page.evaluate(() => { __fail.FindPackageUpdates='Synthetic package failure'; });
        await page.locator('#updates-package-check').click();
        await page.waitForFunction(() => document.querySelector('#updates-package-status').textContent.includes('Synthetic package failure'));
        ok('package failure stays in its own section', !(await text('updates-release-status')).includes('package failure'));
      } else if (name === 'packages') {
        await page.locator('#updates-package-check').click();
        await page.waitForFunction(() => document.querySelector('#updates-package-status').textContent.includes('finished'));
        ok('package check shows current installed versions and scope', (await text('updates-packages')).includes('1.0.0') && (await text('updates-packages')).includes('personal, all workspaces') && (await text('updates-packages')).includes('workspace: C:/Fixture'));
        await page.evaluate(() => { __u.packages.updates=[{name:'Writing skills',target:'me',from:'1.0.0',to:'1.1.0',bundle:'C:/Fixture/package.zip'},{name:'Project skills',target:'workspace',workspace:'C:/Fixture',from:'1.0.0',to:'1.1.0',bundle:'C:/Fixture/blocked.zip',reason:'Locally edited file must be kept'}]; });
        await page.locator('#updates-package-check').click();
        await page.waitForFunction(() => document.querySelector('#updates-packages button').getAttribute('aria-disabled') === 'false');
        ok('blocked update has an explanation and no enabled bypass', (await text('updates-packages')).includes('Locally edited file') && await page.locator('#updates-packages button').nth(1).isDisabled());
        await page.locator('#updates-packages button').first().click();
        await page.locator('.install-plan .dialog-actions [data-no]').click();
        await page.waitForFunction(() => !document.querySelector('#updates-package-check').disabled);
        ok('declined authorization never calls installer', !(await ledger(t)).some(c => c[0] === 'InstallPersonalPackages'));
        await page.locator('#updates-packages button').first().click();
        await page.locator('.install-plan .dialog-actions [data-yes]').click();
        await page.waitForFunction(() => __updateCalls.some(c => c[0] === 'InstallPersonalPackages'));
        await page.waitForFunction(() => !document.querySelector('#updates-package-check').disabled);
        ok('only selected package changes through existing installer token', (await ledger(t)).filter(c => c[0] === 'InstallPersonalPackages').length === 1 && (await page.evaluate(() => __u.installed[1].version)) === '1.0.0');
        await page.evaluate(() => { __fail.InstallPersonalPackages='Synthetic package install failure'; __u.packages.updates=[{name:'Writing skills',target:'me',from:'1.1.0',to:'1.2.0',bundle:'C:/Fixture/next.zip'}]; });
        await page.locator('#updates-package-check').click();
        await page.waitForFunction(() => document.querySelector('#updates-packages button').getAttribute('aria-disabled') === 'false');
        await page.locator('#updates-packages button').first().click();
        await page.locator('.install-plan .dialog-actions [data-yes]').click();
        await page.waitForFunction(() => document.querySelector('#updates-package-status').textContent.includes('Synthetic package install failure'));
        ok('package install failure leaves its previous version and release state intact', (await page.evaluate(() => __u.installed[0].version)) === '1.1.0' && !(await text('updates-release-status')).includes('package install failure'));
      } else if (name === 'missing-services') {
        ok('missing Status method is shown as unavailable', (await text('updates-inventory-status')).includes('no method Status'));
        ok('supported release and package sections remain usable', (await text('updates-release-status')).includes('No newer release') && await page.locator('#updates-package-check').isEnabled());
        await page.locator('#updates-repair-check').click();
        await page.waitForFunction(() => document.querySelector('#updates-repair-status').textContent.includes('no method PreviewAgentSettings'));
        ok('missing repair service cannot fabricate a successful preview', await page.locator('#updates-repair-apply').isDisabled());
        ok('missing capabilities do not trigger shell or mutation fallbacks', !(await ledger(t)).some(c => ['Download', 'RepairAgentSettings', 'InstallPersonalPackages'].includes(c[0])));
      } else if (name === 'repair') {
        ok('repair is gated before preview', await page.locator('#updates-repair-apply').isDisabled());
        await page.locator('#updates-repair-check').click();
        await page.waitForFunction(() => document.querySelector('#updates-repair-apply').getAttribute('aria-disabled') === 'false');
        ok('preview names stale client, scope, custom entry and installed destination', (await text('updates-repair')).includes('Would repair') && (await text('updates-repair')).includes('project C:/Project') && (await text('updates-repair')).includes('Custom entry kept') && (await text('updates-repair')).includes('Installed tools: C:/Fixture/bin/tools.exe'));
        ok('service details and unreported-override limitation stay visible', (await text('updates-repair-lines')).includes('owned earlier entry') && (await text('settings-updates')).includes('Unreported workspace overrides remain unverified'));
        ok('preview is read only', !(await ledger(t)).some(c => c[0] === 'RepairAgentSettings'));
        await t.shot('updates-repair-preview');
        await page.evaluate(() => { __hold.RepairAgentSettings=true; __u.repaired={findings:[{...__u.preview.findings[0],changed:true,backup:'C:/Fixture/backup'},{...__u.preview.findings[2],state:'unread',error:'Claude read-back failed; backup C:/Fixture/backup'}]}; });
        await page.locator('#updates-repair-apply').evaluate(el => {el.click();el.click();});
        await page.waitForFunction(() => __pending.RepairAgentSettings);
        ok('double repair admits one preview-bound operation', (await ledger(t)).filter(c => c[0] === 'RepairAgentSettings').length === 1 && (await ledger(t)).find(c => c[0] === 'RepairAgentSettings')[1] === 'preview-1');
        await page.evaluate(() => { __hold.RepairAgentSettings=false;__pending.RepairAgentSettings[0](); });
        await page.waitForFunction(() => document.querySelector('#updates-repair-status').textContent.includes('Some settings'));
        ok('partial repair reports successes, failures and backup without retry', (await text('updates-repair')).includes('Repaired') && (await text('updates-repair')).includes('read-back failed') && (await text('updates-repair')).includes('Backup:'));
        await page.evaluate(() => { __u.preview.findings.forEach(f => {f.state='current';});__u.preview.findings[0].state='absent'; });
        await page.locator('#updates-repair-check').click();
        await page.waitForFunction(() => document.querySelector('#updates-repair-status').textContent.includes('No owned stale'));
        ok('absent entries are never added and current entries need no repair', await page.locator('#updates-repair-apply').isDisabled() && (await text('updates-repair')).includes('none will be added'));
        await page.evaluate(() => { __fail.PreviewAgentSettings='Settings parse failed'; });
        await page.locator('#updates-repair-check').click();
        await page.waitForFunction(() => document.querySelector('#updates-repair-status').textContent.includes('parse failed'));
        ok('repair failure preserves other working sections', (await text('updates-inventory-status')).includes('read') && await page.locator('#updates-check').isEnabled());
        await page.evaluate(() => { __fail.PreviewAgentSettings=''; __u.preview.findings[0].state='stale'; delete __u.preview.token; });
        await page.locator('#updates-repair-check').click();
        await page.waitForFunction(() => document.querySelector('#updates-repair-status').textContent.includes('lacks its token'));
        ok('malformed preview refuses mutation instead of inventing authority', await page.locator('#updates-repair-apply').isDisabled());
      }
    }
  } finally { await finish(t, 'settings-updates-' + name); }
};
