// An ordinary chat offers Last Read in its top bar, not in a row of its own,
// only while there is something unread, and the bottom composer stays where it
// was. Pointer and keyboard both take the reader to where they stopped.
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
    const lastRead = win.getByRole('button', { name: 'Last Read', exact: true });
    const composerBottom = async () => (await win.locator('.composer-toolbar').boundingBox()).y;
    await win.locator('.win-execution .win-state[data-state="working"]').waitFor({ state: 'attached' });
    await page.evaluate(() => {
      window.__readingUpdate = update => __emit('agent:update', { sessionId: 'reading', update });
      for (let i = 0; i < 16; i++) __readingUpdate({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Earlier report ' + i + '.\n\n' + 'Report paragraph.\n\n'.repeat(12) } });
    });
    await page.waitForFunction(key => {
      const log = document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]');
      return log.scrollHeight > log.clientHeight * 3 && log.scrollHeight - log.scrollTop - log.clientHeight < 24;
    }, key);
    ok('nothing unread offers no Last Read and no top bar for an ordinary chat', !(await lastRead.isVisible()) && await win.locator('.chat-team').isHidden());
    const composerBefore = await composerBottom();
    await log.focus(); await page.keyboard.press('Control+Home');
    await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]').scrollTop < 2, key);
    await page.evaluate(() => __readingUpdate({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'New report.\n\n' } }));
    const divider = win.locator('.transcript-unread-divider');
    await divider.waitFor({ state: 'attached' });
    await lastRead.waitFor();
    ok('Last Read is in the top bar', await win.locator('.chat-team .chat-last-read').isVisible());
    ok('Last Read says what it does', (await lastRead.getAttribute('title')) === 'Go to where you stopped reading');
    ok('there is no row of its own for it', await win.locator('.transcript-unread-controls').count() === 0);
    ok('the bottom composer has not moved', Math.abs(await composerBottom() - composerBefore) < 0.5);
    const inView = () => page.evaluate(key => {
      const win = document.querySelector('[data-win="' + key + '"]'), log = win.querySelector('[data-view-id="activity"]'), d = win.querySelector('.transcript-unread-divider');
      const l = log.getBoundingClientRect(), r = d.getBoundingClientRect();
      return r.bottom > l.top && r.top < l.bottom;
    }, key);
    ok('the place is not yet in view', !(await inView()));
    await lastRead.focus(); await page.keyboard.press('Enter');
    await page.waitForFunction(key => {
      const win = document.querySelector('[data-win="' + key + '"]'), log = win.querySelector('[data-view-id="activity"]'), d = win.querySelector('.transcript-unread-divider');
      if (!d) return false;
      const l = log.getBoundingClientRect(), r = d.getBoundingClientRect();
      return r.bottom > l.top && r.top < l.bottom;
    }, key);
    ok('the keyboard takes the reader to where they stopped', true);
    await log.focus(); await page.keyboard.press('Control+Home');
    await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]').scrollTop < 2, key);
    if (await lastRead.isVisible()) {
      await lastRead.click();
      await page.waitForFunction(key => {
        const win = document.querySelector('[data-win="' + key + '"]'), log = win.querySelector('[data-view-id="activity"]'), d = win.querySelector('.transcript-unread-divider');
        if (!d) return log.scrollHeight - log.scrollTop - log.clientHeight < 24;
        const l = log.getBoundingClientRect(), r = d.getBoundingClientRect();
        return r.bottom > l.top && r.top < l.bottom;
      }, key);
      ok('the pointer takes the reader to where they stopped', true);
    }
    await t.shot('chat-last-read');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
