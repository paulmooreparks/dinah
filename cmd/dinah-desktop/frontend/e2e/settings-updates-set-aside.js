const { start, settle, setRelease, finish } = require('./lib/updates-fixture');
const { copy, start: startAside, finish: finishAside } = require('./lib/set-aside-fixture');
(async () => {
  const t = await start({ asideItems: [copy(1), copy(2, { fallback: true, canRemove: false, reasonCode: 'fallback', reason: 'This is the newest fallback.' }), copy(3, { runningPids: [123], canRemove: false, reasonCode: 'in_use', reason: 'A running process uses this copy.' })] });
  const { page: p, ok } = t;
  const row = n => p.locator('[data-copy-id="' + copy(n).id + '"]');
  const ready = () => p.waitForFunction(() => document.querySelector('#set-aside-refresh').getAttribute('aria-disabled') === 'false');
  const bounds = () => p.evaluate(() => {
    const panel = document.querySelector('#settings-updates');
    return [...panel.querySelectorAll(':scope > section, #updates-set-aside, #set-aside-card, #set-aside-card [id], #set-aside-list > div')].map(el => { const r = el.getBoundingClientRect(); return [el.id || el.dataset.copyId || el.getAttribute('aria-labelledby'), r.x, r.y + panel.scrollTop, r.width, r.height]; });
  });
  try {
    await settle(t); await ready();
    ok('Updates places the existing card after running programs', await p.evaluate(() => {
      const card = document.querySelector('#set-aside-card'), slot = document.querySelector('#updates-set-aside');
      return card.parentElement === slot && slot.previousElementSibling.getAttribute('aria-labelledby') === 'updates-running-title' && slot.nextElementSibling.getAttribute('aria-labelledby') === 'updates-packages-title' && document.querySelectorAll('#set-aside-card').length === 1;
    }));
    ok('both protections are visible and cannot be selected', await row(2).locator('input').isDisabled() && await row(3).locator('input').isDisabled() && (await p.locator('#set-aside-list').innerText()).includes('newest fallback'));
    await row(1).locator('input').check();
    await p.locator('#settings-tab-set-aside').click(); await ready();
    ok('the original Settings location keeps the same selection and component', await p.locator('#settings-set-aside #set-aside-card').count() === 1 && await row(1).locator('input').isChecked());
    await p.evaluate(item => { __u.asideItems.push(item); }, copy(4)); await p.locator('#set-aside-refresh').click(); await row(4).waitFor();
    await p.locator('#settings-tab-updates').click(); await ready();
    ok('a Settings refresh is reflected in Updates', await row(4).count() === 1 && await row(1).locator('input').isChecked() && (await p.locator('#set-aside-total').innerText()).includes('1,024 bytes'));
    const before = await bounds();
    await p.evaluate(item => { __u.asideItems.push(item); }, copy(5)); await p.locator('#updates-refresh').click(); await row(5).waitFor(); await ready();
    const after = await bounds(), old = new Map(before.map(value => [value[0], value]));
    ok('new copies leave all existing card and page bounds unchanged', after.filter(value => old.has(value[0])).every(value => JSON.stringify(value) === JSON.stringify(old.get(value[0]))));
    await p.locator('#set-aside-remove').click(); await p.locator('#ask[open]').waitFor();
    ok('removal asks before invoking the service', !(await p.evaluate(() => __updateCalls)).some(call => call[0] === 'RemoveSetAsidePrograms'));
    await p.locator('#ask-no').click(); await ready();
    ok('cancellation retains the selected copy', await row(1).locator('input').isChecked());
    await p.locator('#set-aside-remove').click(); await p.locator('#ask-yes').click();
    await p.waitForFunction(() => document.querySelector('#set-aside-results').textContent.includes('Reported removed size: 1,024 bytes')); await ready();
    ok('removal uses only the selected identity and retains its row', await p.evaluate(id => { const calls = __updateCalls.filter(call => call[0] === 'RemoveSetAsidePrograms'); return calls.length === 1 && JSON.stringify(calls[0][1]) === JSON.stringify({ ids: [id] }); }, copy(1).id) && await row(1).locator('input').isDisabled());
    await p.locator('#settings-tab-set-aside').click(); await ready();
    ok('Updates removal is reflected in Settings with its result', (await p.locator('#set-aside-results').innerText()).includes('1,024 bytes') && await row(1).locator('input').isDisabled() && await row(2).locator('input').isDisabled() && await row(3).locator('input').isDisabled());
    await p.locator('#settings-tab-updates').click(); await ready();
    await setRelease(t, { latest: '2.1.0', available: true }); await p.locator('#updates-install').click();
    await p.waitForFunction(() => window.__u.release.downloading);
    await p.evaluate(item => { __u.asideItems.push(item); Object.assign(__u.release, { downloading: false, available: false, installed: true }); }, copy(6));
    await row(6).waitFor(); await ready();
    ok('installation completion refreshes program copies without another click', await row(6).count() === 1);
    await t.shot('updates-set-aside');
    await p.reload(); await p.locator('#settings-updates #set-aside-card').waitFor(); await ready();
    ok('an Updates address restores the shared card in Updates', await p.locator('#settings-updates').isVisible() && await p.locator('#settings-updates #set-aside-card').count() === 1);
    await p.evaluate(() => pudlWindows.snap('settings', 'maximized')); await p.setViewportSize({ width: 390, height: 844 });
    await p.waitForFunction(() => document.querySelector('[data-win="settings"]').getBoundingClientRect().right <= innerWidth);
    await p.locator('#set-aside-card').scrollIntoViewIfNeeded();
    ok('the shared card fits the narrow Updates page', await p.locator('#settings-updates').evaluate(el => el.scrollWidth <= el.clientWidth + 1) && await p.locator('#set-aside-card').evaluate(el => el.scrollWidth <= el.clientWidth + 1));
    await t.shot('updates-set-aside-narrow');
  } finally { await finish(t, 'updates-set-aside'); }
  const standalone = await startAside();
  try {
    const page = standalone.page;
    standalone.ok('Settings keeps its original card without an updates service', await page.locator('#settings-tab-updates').isHidden() && await page.locator('#settings-set-aside #set-aside-card').isVisible() && await page.locator('#updates-set-aside').isHidden());
    await page.locator('#set-aside-list input').first().check();
    standalone.ok('the standalone card still selects eligible copies', (await page.locator('#set-aside-total').innerText()).includes('1 selected; total logical size: 1,024 bytes'));
    await page.reload(); await page.locator('#settings-set-aside [data-copy-id]').first().waitFor();
    standalone.ok('the standalone address restores without an Updates entry', await page.locator('#settings-tab-updates').isHidden() && await page.locator('#settings-set-aside #set-aside-card').isVisible() && await page.locator('#set-aside-card').count() === 1);
    standalone.ok('no Updates operation is needed for standalone Settings', !(await standalone.calls('CallService')).some(call => call[1] === 'updates'));
  } finally { await finishAside(standalone, 'updates-set-aside-standalone'); }
})().catch(error => { console.error(error); process.exitCode = 1; });
