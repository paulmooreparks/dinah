const { start, card, quiet } = require('./lib/whats-new-fixture');
(async () => {
  const t = await start(); const { page: p, ok } = t;
  const back = () => p.evaluate(() => dinahWorkbench.open('needs-you'));
  try {
    await card(t).waitFor();
    ok('one quiet dismissible card describes the features', await card(t).count() === 1 && (await card(t).innerText()).includes('Pause Chats and Restart') && await card(t).getByRole('button', { name: 'Dismiss', exact: true }).count() === 1 && await p.locator('#status-needs [data-status-badge]').innerText() === '0');
    ok('a product without Updates has four feature links', await card(t).locator('.notice-actions > button').count() === 5 && await card(t).getByRole('button', { name: 'Open Updates', exact: true }).count() === 0);
    await card(t).getByRole('button', { name: 'Open Decisions', exact: true }).click(); await p.locator('.decisions').waitFor();
    ok('Decisions opens without answering anything', await p.evaluate(() => pudlWindows.state().top === 'decisions'));
    await back(); await card(t).getByRole('button', { name: 'Open Working with Chats', exact: true }).click(); await p.locator('[data-win="help-chats"] article').waitFor();
    ok('the existing Help guide opens', await p.locator('[data-win="help-chats"] article h2').innerText() === 'Working with Chats');
    await back(); await card(t).getByRole('button', { name: 'Open Background Chats', exact: true }).click();
    await p.waitForFunction(() => document.activeElement.id === 'set-background-chat-limit');
    ok('Background Chats has a direct address and focused limit control', await p.evaluate(() => new URL(location.href).searchParams.get('view.settings') === 'chat?background'));
    await p.reload(); await p.waitForFunction(() => document.activeElement.id === 'set-background-chat-limit');
    ok('the background section address survives reload', await p.locator('#settings-chat').isVisible());
    await p.evaluate(() => dinahWorkbench.open('help-chats'));
    await p.reload(); await p.locator('[data-win="help-chats"] article').waitFor();
    await p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    ok('a restored background Settings window does not take focus', await p.evaluate(() => pudlWindows.state().top === 'help-chats' && document.activeElement.id !== 'set-background-chat-limit'));
    await back(); await card(t).getByRole('button', { name: 'Open Set-aside Programs', exact: true }).click(); await p.locator('#settings-set-aside').waitFor({ state: 'visible' });
    ok('Set-aside Programs opens without removing files', await p.evaluate(() => new URL(location.href).searchParams.get('view.settings') === 'set-aside'));
    await back(); await card(t).waitFor();
    ok('opening links leaves the card available', await card(t).count() === 1);
    await card(t).getByRole('button', { name: 'Dismiss', exact: true }).click(); await card(t).waitFor({ state: 'detached' });
    await p.reload(); await p.locator('#questions [data-notification-id="earlier-report"]').waitFor();
    ok('dismissal survives page reload', await card(t).count() === 0);
  } finally { await quiet(t); }
  const updates = await start({ updates: true });
  try {
    await card(updates).waitFor(); await card(updates).getByRole('button', { name: 'Open Updates', exact: true }).click();
    await updates.page.locator('#settings-updates').waitFor({ state: 'visible' });
    updates.ok('a product service adds the Updates destination', await updates.page.evaluate(() => new URL(location.href).searchParams.get('view.settings') === 'updates'));
  } finally { await quiet(updates); }
})().catch(error => { console.error(error); process.exitCode = 1; });
