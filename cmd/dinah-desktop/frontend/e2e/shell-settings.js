// The status area's Notifications and Workspace menus, the new-window
// preference, and saving the interface state before a native close.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      let mode = 'floating';
      const state = () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture', 'C:/Other'], defaults: {}, catalog: [], pinned: [], newWindowMode: mode });
      return {
        ReadUIState: async () => ({ version: 1, values: {} }),
        SaveUIState: async value => { __calls.push(['save', value]); if (window.__saveError) throw new Error('Fixture disk write failed'); },
        AckUIStateFlush: async (id, success) => { __calls.push(['ack', id, success]); },
        State: async () => { __calls.push(['state']); return state(); },
        SetNewWindowMode: async value => { __calls.push(['mode', value]); if (window.__modeError) throw new Error('Fixture preference write failed'); mode = value; },
        ChooseWorkspace: async () => { __calls.push(['choose']); return state(); }
      };
    },
    init: () => {
      window.__calls = [];
      const quit = window.runtime.Quit;
      window.runtime.Quit = () => { __calls.push(['quit']); quit(); };
    },
  });
  const { page, ok, errors } = t;
  try {
    await page.waitForFunction(() => document.querySelector('#status-ws-recent').textContent.includes('Other'));
    await page.locator('#status-needs').click();
    await page.locator('#notifications-status-menu').waitFor({ state: 'visible' });
    ok('empty Notifications menu says so and disables Clear All', await page.locator('#notifications-status-menu .notices-empty').textContent() === 'No notifications.' && await page.locator('#notifications-status-menu [data-cmd="clear-notifications"]').isDisabled());
    ok('opening the Notifications menu creates no notification', await page.locator('#questions .question').count() === 0);
    await t.shot('notifications-menu');
    await page.locator('#notifications-status-menu [data-cmd="needs-you"]').click();
    await page.locator('[data-win="needs-you"]').waitFor({ state: 'visible' });
    await page.locator('#status-workspace').click();
    await page.locator('#workspace-status-menu').waitFor({ state: 'visible' });
    ok('Workspace menu offers recent workspaces', await page.locator('#status-ws-recent').innerText().then(text => text.includes('Other')));
    await t.shot('workspace-menu');
    const before = await page.evaluate(() => __calls.filter(x => x[0] === 'state').length);
    await page.locator('#workspace-status-menu [data-cmd="refresh-workspace"]').click();
    await page.waitForFunction(count => __calls.filter(x => x[0] === 'state').length > count, before);
    ok('Refresh Workspace reads the current workspace again', true);
    await page.evaluate(() => dinahWorkbench.open('settings'));
    const mode = value => page.locator('#new-window-mode [role="radio"][data-value="' + value + '"]');
    ok('new window preference defaults to Floating', await mode('floating').getAttribute('aria-checked') === 'true');
    await mode('maximized').click();
    await page.waitForFunction(() => !document.querySelector('#new-window-mode [role="radio"]').disabled && dinahUIState.newWindowMode() === 'maximized');
    ok('new window preference is saved before applying', await page.evaluate(() => __calls.some(x => x[0] === 'mode' && x[1] === 'maximized')));
    await page.locator('#new-window-mode').scrollIntoViewIfNeeded();
    await t.shot('window-preference');
    await page.evaluate(() => { window.__modeError = true; });
    await mode('floating').click();
    await page.waitForFunction(() => !document.querySelector('#new-window-mode [role="radio"]').disabled);
    ok('failed preference save restores the previous selection', await mode('maximized').getAttribute('aria-checked') === 'true');
    await page.evaluate(() => { dinahUIState.set('fixture-close', 1); __emit('dinah:ui-state-flush', { id: 'close-1' }); });
    await page.waitForFunction(() => __calls.some(x => x[0] === 'ack' && x[1] === 'close-1'));
    ok('native close saves state before acknowledging success', await page.evaluate(() => { const index = __calls.findIndex(x => x[0] === 'ack' && x[1] === 'close-1'); return __calls[index][2] && __calls.slice(0, index).some(x => x[0] === 'save' && x[1].values['fixture-close'] === 1); }));
    await page.evaluate(() => { window.__saveError = true; dinahUIState.set('fixture-close', 2); __emit('dinah:ui-state-flush', { id: 'close-2' }); });
    await page.waitForFunction(() => __calls.some(x => x[0] === 'ack' && x[1] === 'close-2'));
    ok('failed state save refuses native close', await page.evaluate(() => __calls.some(x => x[0] === 'ack' && x[1] === 'close-2' && x[2] === false) && !__calls.some(x => x[0] === 'quit')));
    await t.shot('shell-settings');
    ok('shell settings has no script errors', errors.length === 0);
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
