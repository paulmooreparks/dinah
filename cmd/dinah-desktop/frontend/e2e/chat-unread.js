// Streaming chunks belong to one logical unread message. A deliberate jump
// sees it and restores following through subsequent output.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1200, height: 850 },
    app: () => ({
      OpenSessionAt: async (cwd, sessionId) => ({ sessionId, cwd, provider: 'codex', open: true, life: 'open', window: true, exec: { state: 'working', why: '' } })
    })
  });
  const { page, ok } = t;
  try {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'reading' }));
    const win = page.locator('[data-win="' + key + '"]');
    const log = win.locator('[data-view-id="activity"]');
    await win.locator('.chat-state-working').first().waitFor();
    await page.evaluate(() => {
      window.__readingUpdate = update => __emit('agent:update', { sessionId: 'reading', update });
      for (let i = 0; i < 16; i++) {
        __readingUpdate({ sessionUpdate: 'tool_call', toolCallId: 'old-' + i, title: 'Read report ' + i, status: 'completed' });
        __readingUpdate({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Earlier report ' + i + '.\n\n' + 'Report paragraph.\n\n'.repeat(12) } });
      }
    });
    await page.waitForFunction(key => {
      const log = document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]');
      return log.scrollHeight > log.clientHeight * 3 && log.scrollHeight - log.scrollTop - log.clientHeight < 24;
    }, key);
    await log.focus(); await page.keyboard.press('Control+Home');
    await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]').scrollTop < 2, key);
    await page.evaluate(() => {
      __readingUpdate({ sessionUpdate: 'tool_call', toolCallId: 'new-report', title: 'Prepare the new report', status: 'completed' });
      __readingUpdate({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'New report.\n\n' } });
    });
    const divider = win.locator('.transcript-unread-divider');
    await divider.waitFor();
    const before = Number((await divider.innerText()).match(/\d+/)[0]);
    for (let i = 0; i < 5; i++) await page.evaluate(() => __readingUpdate({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Another chunk in the same report.\n\n' } }));
    await win.locator('.activity-row').filter({ hasText: 'Another chunk in the same report.' }).first().waitFor({ state: 'attached' });
    ok('streaming chunks do not increase the logical unread-message count', Number((await divider.innerText()).match(/\d+/)[0]) === before);
    await win.getByRole('button', { name: /Jump to the latest/i }).click();
    await divider.waitFor({ state: 'detached' });
    await page.evaluate(() => __readingUpdate({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'More output after the jump.\n\n'.repeat(20) } }));
    await page.waitForFunction(key => {
      const log = document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]');
      return log.dataset.following === 'true' && log.scrollHeight - log.scrollTop - log.clientHeight < 24 && !log.querySelector('.transcript-unread-divider');
    }, key);
    ok('the jump clears unread messages and keeps following later chunks', true);
    await t.shot('chat-unread-following');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
