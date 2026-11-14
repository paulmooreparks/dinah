const { start, copy, check, finish } = require('./lib/set-aside-fixture');
(async () => {
  const t = await start({ warnings: ['One process could not be inspected.'], updates: true }); const { page: p, ok } = t;
  try {
    ok('the section is independent of Updates and addressable', (await p.evaluate(() => new URL(location.href).searchParams.get('view.settings'))) === 'set-aside' && await p.locator('#settings-tab-updates').isVisible());
    ok('profile programs disclose size, date, fallback and running protection', (await p.locator('#set-aside-list').innerText()).includes('1,024 bytes') && (await p.locator('#set-aside-list').innerText()).includes('command.exe') && (await p.locator('#set-aside-list').innerText()).includes('tools.exe') && (await p.locator('#set-aside-list').innerText()).includes('newest fallback') && (await p.locator('#set-aside-list').innerText()).includes('321') && await check(t, 2).isDisabled() && await check(t, 3).isDisabled());
    ok('inspection warnings remain visible', (await p.locator('#set-aside-warnings').innerText()).includes('could not be inspected'));
    const bounds = () => p.evaluate(() => Object.fromEntries([...document.querySelectorAll('#settings-set-aside > section, #set-aside-status, #set-aside-warnings, #set-aside-list, #set-aside-total, #set-aside-refresh, #set-aside-remove, #set-aside-results, #set-aside-list > div')].map(el => { const r = el.getBoundingClientRect(); return [el.id || el.dataset.copyId || 'card', [r.x, r.y + document.querySelector('#settings-set-aside').scrollTop, r.width, r.height]]; })));
    const before = await bounds();
    await t.shot('settings-set-aside-before');
    await p.evaluate(item => { __asideItems.push(item); __asideWarnings = []; }, copy(4, { name: '<img src=x onerror=alert(1)>.before-fixture-20261112-080000' }));
    await p.locator('#set-aside-refresh').click(); await p.locator('[data-copy-id="' + copy(4).id + '"]').waitFor();
    const measured = await bounds(), after = Object.fromEntries(Object.keys(before).map(key => [key, measured[key]]));
    ok('an arriving copy appends without moving regions or earlier rows', JSON.stringify(before) === JSON.stringify(after), JSON.stringify({ before, after }));
    await t.shot('settings-set-aside-after');
    ok('file names are displayed as text', await p.locator('#set-aside-list img').count() === 0 && (await p.locator('#set-aside-list').innerText()).includes('<img'));
    await check(t, 1).check();
    ok('selection shows the logical total', (await p.locator('#set-aside-total').innerText()).includes('1 selected; total logical size: 1,024 bytes'));
    await p.evaluate(item => { __asideItems = __asideItems.filter(item => item.id !== '1'.padStart(64, '0')); __asideItems.push(item); }, copy(5, { name: copy(1).name }));
    await p.locator('#set-aside-refresh').click(); await p.locator('[data-copy-id="' + copy(5).id + '"]').waitFor();
    ok('a replacement never inherits the earlier identity selection', !await check(t, 5).isChecked() && await check(t, 1).isDisabled() && (await p.locator('#set-aside-total').innerText()).startsWith('0 selected'));
    await check(t, 5).check();
    await p.evaluate(() => { const item = __asideItems.find(item => item.id === '5'.padStart(64, '0')); item.canRemove = false; item.fallback = true; item.reasonCode = 'fallback'; item.reason = 'This is now the newest fallback.'; });
    await p.locator('#set-aside-refresh').click();
    await p.waitForFunction(() => document.querySelector('#set-aside-total').textContent.startsWith('0 selected'));
    ok('a newly protected fallback loses its selection', await check(t, 5).isDisabled() && !await check(t, 5).isChecked());
    await p.evaluate(() => { __asideItems = []; }); await p.locator('#set-aside-refresh').click();
    await p.waitForFunction(() => document.querySelector('#set-aside-status').textContent.includes('No set-aside programs'));
    ok('empty inventory retains row positions and clears obsolete selections', await p.locator('#set-aside-list > div').count() === 5 && await check(t, 1).isDisabled() && (await p.locator('#set-aside-total').innerText()).startsWith('0 selected'));
    await p.evaluate(() => pudlWindows.snap('settings', 'maximized'));
    await p.setViewportSize({ width: 390, height: 844 });
    await p.waitForFunction(() => document.querySelector('[data-win="settings"]').getBoundingClientRect().right <= innerWidth);
    ok('the narrow section has no horizontal overflow', await p.locator('#settings-set-aside').evaluate(el => el.scrollWidth <= el.clientWidth + 1));
    await t.shot('settings-set-aside-narrow');
  } finally { await finish(t, 'set-aside-list'); }
  const absent = await start({ unsupported: true });
  try { absent.ok('older hosts have no unsupported section', await absent.page.locator('#settings-tab-set-aside').isHidden()); } finally { await finish(absent, 'set-aside-absent'); }
  const unsupported = await start({ items: [copy(1, { canRemove: false, reasonCode: 'unsupported', reason: 'Removal is unavailable on this platform.' })] });
  try { unsupported.ok('platform refusal is readable and cannot be selected', await check(unsupported, 1).isDisabled() && (await unsupported.page.locator('#set-aside-list').innerText()).includes('unavailable on this platform')); } finally { await finish(unsupported, 'set-aside-platform'); }
})().catch(error => { console.error(error); process.exitCode = 1; });
