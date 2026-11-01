// An ordinary chat keeps Last Read in its top bar, which stays mounted, so
// nothing moves when there comes to be something unread or when it is read.
// Last Read is disabled while there is nowhere to go back to, and pointer and
// keyboard both take the reader to where they stopped.
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
    const geometry = () => page.evaluate(key => {
      const win = document.querySelector('[data-win="' + key + '"]'), r = el => { const b = el.getBoundingClientRect(); return { y: b.y, height: b.height }; };
      return { bar: r(win.querySelector('.chat-team')), log: r(win.querySelector('[data-view-id="activity"]')), composer: r(win.querySelector('.composer-toolbar')) };
    }, key);
    const same = (a, b) => ['bar', 'log', 'composer'].every(k => Math.abs(a[k].y - b[k].y) < 0.5 && Math.abs(a[k].height - b[k].height) < 0.5);
    const inView = () => page.evaluate(key => {
      const win = document.querySelector('[data-win="' + key + '"]'), log = win.querySelector('[data-view-id="activity"]'), d = win.querySelector('.transcript-unread-divider');
      if (!d) return false;
      const l = log.getBoundingClientRect(), r = d.getBoundingClientRect();
      return r.bottom > l.top && r.top < l.bottom;
    }, key);
    const toTop = async () => {
      await log.focus(); await page.keyboard.press('Control+Home');
      await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]').scrollTop < 2, key);
    };
    await win.locator('.win-execution .win-state[data-state="working"]').waitFor({ state: 'attached' });
    await page.evaluate(() => {
      window.__readingUpdate = update => __emit('agent:update', { sessionId: 'reading', update });
      for (let i = 0; i < 16; i++) __readingUpdate({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Earlier report ' + i + '.\n\n' + 'Report paragraph.\n\n'.repeat(12) } });
    });
    await page.waitForFunction(key => {
      const log = document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]');
      return log.scrollHeight > log.clientHeight * 3 && log.scrollHeight - log.scrollTop - log.clientHeight < 24;
    }, key);
    ok('an ordinary chat keeps its top bar with Last Read in it', await win.locator('.chat-team').isVisible() && await win.locator('.chat-team .chat-last-read').isVisible());
    ok('Last Read says what it does', (await lastRead.getAttribute('title')) === 'Go to where you stopped reading');
    ok('with nothing unread Last Read is disabled', await lastRead.isDisabled());
    ok('there is no row of its own for it', await win.locator('.transcript-unread-controls').count() === 0);
    await toTop();
    const before = await geometry();
    await page.evaluate(() => __readingUpdate({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'New report.\n\n' } }));
    await win.locator('.transcript-unread-divider').waitFor({ state: 'attached' });
    await page.waitForFunction(key => !document.querySelector('[data-win="' + key + '"] .chat-last-read').disabled, key);
    ok('something unread enables Last Read', await lastRead.isEnabled());
    ok('the bar, transcript and composer do not move when it is enabled', same(before, await geometry()), JSON.stringify([before, await geometry()]));
    ok('the place is not yet in view', !(await inView()));
    await lastRead.focus(); await page.keyboard.press('Enter');
    await page.waitForFunction(inViewKey => {
      const win = document.querySelector('[data-win="' + inViewKey + '"]'), log = win.querySelector('[data-view-id="activity"]'), d = win.querySelector('.transcript-unread-divider');
      if (!d) return false;
      const l = log.getBoundingClientRect(), r = d.getBoundingClientRect();
      return r.bottom > l.top && r.top < l.bottom;
    }, key);
    ok('the keyboard takes the reader to where they stopped', true);
    await toTop();
    if (await lastRead.isEnabled()) {
      await lastRead.click();
      await page.waitForFunction(key => {
        const win = document.querySelector('[data-win="' + key + '"]'), log = win.querySelector('[data-view-id="activity"]'), d = win.querySelector('.transcript-unread-divider');
        if (!d) return log.scrollHeight - log.scrollTop - log.clientHeight < 24;
        const l = log.getBoundingClientRect(), r = d.getBoundingClientRect();
        return r.bottom > l.top && r.top < l.bottom;
      }, key);
      ok('the pointer takes the reader to where they stopped', true);
    }
    const enabled = await geometry();
    await win.getByRole('button', { name: /Jump to the latest/i }).click().catch(() => {});
    await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] .chat-last-read').disabled, key);
    ok('reading everything disables Last Read again', await lastRead.isDisabled());
    const after = await geometry();
    ok('the bar and composer do not move when it is disabled', ['bar', 'composer'].every(k => Math.abs(after[k].y - enabled[k].y) < 0.5 && Math.abs(after[k].height - enabled[k].height) < 0.5) && Math.abs(after.log.y - enabled.log.y) < 0.5, JSON.stringify([enabled, after]));
    await t.shot('chat-last-read');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
