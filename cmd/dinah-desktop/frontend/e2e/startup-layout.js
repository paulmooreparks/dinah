const { launch } = require('./lib/page');
(async () => {
  for (const keptChat of [false, true]) {
    const t = await launch({ args: keptChat, app: kept => {
      const key = 'item-' + btoa(JSON.stringify(['chat-session', 'C:/Fixture', 'owned'])).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
      const layout = '?open=workspace,chat' + (kept ? ',' + key : '') + '&top=chat&z.order=workspace,chat' + (kept ? ',' + key : '');
      return {
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': layout } }),
        SavedChats: async () => ({ generation: 'layout', chats: [{ chatId: 'owned', sessionId: 'owned', cwd: 'C:/Fixture', provider: 'devin', title: 'Kept chat', resolution: 'resolved' }] }),
        DeclaredProviders: async () => [__provider('devin')],
        ResolveSessionContinuation: async id => id,
        SavedConversation: async () => ({ available: false }),
        OpenSessionAt: async (cwd, sessionId) => ({ cwd, sessionId, provider: 'devin', title: 'Kept chat', open: true, chatState: { life: 'open', exec: { state: 'idle' } } })
      };
    } });
    try {
      await t.page.waitForFunction(() => !document.body.hasAttribute('aria-busy'));
      t.ok('an old saved layout does not restore its blank New Chat window', await t.page.evaluate(() => !pudlWindows.state().open.some(key => dinahWorkbench.item(key)?.kind === 'chat')));
      if (keptChat) {
        await t.page.waitForFunction(() => __bridgeCalls.some(call => call[0] === 'OpenSessionAt' && call[2] === 'owned'));
        t.ok('the same layout still restores its saved conversation', await t.page.evaluate(() => pudlWindows.state().open.some(key => dinahWorkbench.item(key)?.path === 'owned')));
      } else {
        t.ok('restoring only a former blank composer allocates no conversation', (await t.calls()).every(call => !/^(OpenSession|NewSession)/.test(call[0])));
      }
    } finally { await t.close(); }
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
