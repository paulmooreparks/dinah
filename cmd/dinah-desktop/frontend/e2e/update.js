// Restart to Update: the tray's Update item appears when a newer build is
// installed, restarts once the running turns end or, after a question, at
// once, says how many chats are still working while it waits, can be
// cancelled, and the Dinah menu offers the same.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 860 },
    state: { workspace: 'C:/Work/parser', workspaces: ['C:/Work/parser'], recent: ['C:/Work/parser'] },
    app: () => {
      window.__calls = [];
      window.__update = { ready: false, pending: false, working: 0 };
      return {
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }),
        UpdateState: async () => window.__update,
        RestartToUpdate: async now => { __calls.push(['restart', now]); window.__update = Object.assign({}, window.__update, { pending: true }); __emit('update:state', window.__update); },
        CancelRestart: async () => { __calls.push(['cancel']); window.__update = Object.assign({}, window.__update, { pending: false }); __emit('update:state', window.__update); },
      };
    },
    ready: () => document.querySelector('[data-win="workspace"]'),
  });
  const { page: p, ok } = t;
  try {
    await p.waitForTimeout(500);
    const item = p.locator('#status-update');
    ok('with no newer build, the tray shows no Update item', await item.isHidden());
    await p.evaluate(() => { window.__update = { ready: true, pending: false, working: 2 }; __emit('update:state', window.__update); });
    await item.waitFor();
    ok('a newer build shows Update in the tray', (await item.innerText()).includes('Update'));
    // A toast says so, with Restart Now as its action (PUDL 0.90.0): its
    // words say what the action does, it stays, Notifications keeps the
    // same step, and pressing it asks the tray's question and dismisses it.
    const toast = p.locator('.toast').filter({ hasText: 'A newer build of' });
    await toast.waitFor();
    ok('the toast says what Restart Now does', (await toast.locator('.toast-text').textContent()).includes('Restart Now restarts'));
    ok('the toast\'s action is the restart-now command, labelled Restart Now', await toast.locator('button.toast-action').getAttribute('data-command') === 'restart-now' && (await toast.locator('button.toast-action').textContent()).trim() === 'Restart Now');
    ok('Notifications keeps the update with Restart Now', await p.evaluate(() => { const e = dinahHost.notices.get('dinah-update'); return !!e && e.actions.some(a => a.label === 'Restart Now…'); }));
    await p.waitForTimeout(5600);
    ok('a toast with an action does not leave by itself', await toast.isVisible());
    await toast.locator('button.toast-action').click();
    await p.locator('#ask[open]').waitFor();
    await toast.waitFor({ state: 'detached' });
    ok('the toast\'s action asks before restarting, and the toast goes', (await p.locator('#ask-body').textContent()).includes('unfinished'));
    await p.locator('#ask-no').click();
    ok('declining the toast\'s restart leaves the window as it was', !(await p.evaluate(() => __calls.some(c => c[0] === 'restart' && c[1] === true))));
    await item.click();
    await p.locator('#update-restart').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'restart' && c[1] === false));
    await p.waitForFunction(() => document.querySelector('#status-update .status-words').textContent === 'Restarting');
    ok('Restart When Chats Finish waits for the running turns, and the tray says so', (await p.locator('#update-note').textContent()).includes('2 chats are still working'));
    await item.click();
    await p.locator('#update-cancel').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'cancel'));
    await p.waitForFunction(() => document.querySelector('#status-update .status-words').textContent === 'Update');
    ok('the restart can be cancelled', true);
    await item.click();
    await p.locator('#update-now').click();
    await p.locator('#ask[open]').waitFor();
    ok('Restart Now asks first and says what ends unfinished', (await p.locator('#ask-body').textContent()).includes('unfinished'));
    await p.locator('#ask-no').click();
    ok('declining leaves the window as it was', !(await p.evaluate(() => __calls.some(c => c[0] === 'restart' && c[1] === true))));
    await item.click();
    await p.locator('#update-now').click();
    await p.locator('#ask-yes').click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'restart' && c[1] === true));
    ok('accepting restarts at once', true);
    await p.evaluate(() => __emit('update:state', { ready: true, pending: false, working: 0 }));
    await p.waitForFunction(() => dinahCommands.rows('dinah').some(r => r.label === 'Restart When Chats Finish'));
    ok('the Dinah menu offers Restart When Chats Finish', true);
    // The engine sends the count when the update arrives and while a restart waits, and not when
    // chats finish in between, so Restart Now reads the state before it asks.
    await p.evaluate(() => { window.__update = { ready: true, pending: false, working: 2 }; __emit('update:state', window.__update); });
    await p.waitForFunction(() => document.querySelector('#update-note').textContent.includes('newer build'));
    await p.evaluate(() => { window.__update = { ready: true, pending: false, working: 0 }; });
    await item.click();
    await p.locator('#update-now').click();
    await p.locator('#ask[open]').waitFor();
    const quiet = await p.locator('#ask-body').textContent();
    ok('Restart Now asks with the count it reads now, which fell to 0 since the update arrived', /No chat is working/.test(quiet) && !/unfinished/.test(quiet), quiet);
    await p.locator('#ask-no').click();
    await p.evaluate(() => __emit('update:state', { ready: false, pending: false, working: 0 }));
    await p.waitForFunction(() => !dinahHost.notices.get('dinah-update'));
    ok('the Notifications card goes when no newer build is waiting', true);
  } catch (err) {
    t.errors.push(String(err && err.stack || err));
  } finally {
    await t.close();
  }
})();
