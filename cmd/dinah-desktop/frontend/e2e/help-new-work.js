const { launch } = require('./lib/page');
(async () => {
  const t = await launch(); const { page: p, ok } = t;
  try {
    await p.keyboard.press('Alt+h');
    await p.keyboard.press('w');
    const guide = p.locator('.win[data-win="help-chats"]');
    await guide.waitFor();
    ok('Help opens the guide as an addressable window', await p.evaluate(() => pudlWindows.state().top === 'help-chats' && location.search.includes('help-chats')));
    ok('the guide scrolls within its window', await guide.locator('.win-body').evaluate(el => ['auto', 'scroll'].includes(getComputedStyle(el).overflowY)));
    await p.reload();
    await guide.waitFor();
    await guide.locator('article').waitFor();
    ok('reloading the guide address restores its content', await guide.locator('h3').count() === 6);
    await p.evaluate(() => dinahCommands.run('help-chats'));
    ok('opening the guide again raises the same window', await guide.count() === 1);
    const changes = (await t.calls()).filter(c => /^(AnswerDecision|DismissDecision|ConfirmDecisionBroadcast|PauseAndRestart|RestartToUpdate|CancelRestart|SetBackgroundChatLimit|CallService|InstallPackages)$/.test(c[0]));
    ok('reading help performs no feature operation', changes.length === 0, JSON.stringify(changes));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
