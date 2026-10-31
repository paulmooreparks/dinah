// A restored address can mount windows before workspace facts arrive.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const key = 'item-' + Buffer.from(JSON.stringify(['chat-session', 'C:/Later', 'late-chat'])).toString('base64url');
  const group = 'applet-' + Buffer.from(JSON.stringify(['dinah.chat', 'ws_later'])).toString('base64url');
  const t = await launch({
    goto: false,
    app: () => ({
      State: () => new Promise(resolve => {
        window.__releaseGroupingState = () => resolve({ workspace: 'C:/Later', workspaceId: 'ws_later', defaultProvider: 'codex',
          workspaceRecords: [{ id: 'ws_later', name: 'Later', executionPath: 'C:/Later', sources: [] }],
          workspaces: ['C:/Later'], recent: ['C:/Later'], catalog: [], defaults: {}, pinned: [] });
      }),
      OpenSessionAt: async (ws, id) => ({ sessionId: id, provider: 'codex', title: 'Restored Chat', cwd: ws, open: true, working: false, queued: [] })
    })
  });
  const { page, ok } = t;
  try {
    await page.goto(PAGE + '?open=' + key);
    const win = page.locator('.win[data-win="' + key + '"]');
    await win.waitFor();
    await page.waitForFunction(() => typeof __releaseGroupingState === 'function');
    ok('a mounted restored chat with unresolved ownership stays ungrouped', await win.getAttribute('data-win-group') === null);
    await page.evaluate(() => __releaseGroupingState());
    await page.waitForFunction(([key, group]) => document.querySelector('.win[data-win="' + key + '"]').getAttribute('data-win-group') === group, [key, group]);
    ok('workspace facts regroup the restored window without selecting a workspace', await win.getAttribute('data-win-group-label') === 'Chats (Later)' && (await t.calls('SelectWorkspace')).length === 0);
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
