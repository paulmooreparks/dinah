// The shared transcript counts unseen logical messages and clears them as
// real navigation brings them into view. Following always means the current end.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    nativeScrollbars: true,
    viewport: { width: 1200, height: 900 },
    app: () => {
      const member = { id: 'member', chat: 'docs', title: 'Docs Pass', workspace: 'docs', life: 'open', window: true, exec: { state: 'idle', why: '' } };
      let next = 0;
      const message = () => ({ id: ++next, room: 'reading', from: 'Docs Pass', text: 'Message ' + next + '.' + '\n\n' + Array.from({ length: 8 }, (_, n) => 'Paragraph ' + n + ' in this report.').join('\n\n'), createdAt: '2026-10-30T08:00:00Z', kind: 'message' });
      const messages = Array.from({ length: 12 }, message);
      window.__appendReports = count => { while (count--) messages.push(message()); __emit('bus:changed', {}); };
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'] }),
        AgentProviders: async () => [], Sessions: async () => [], SessionsAt: async () => [],
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: [{ name: 'reading', title: 'Reading', members: [member], top: 'idle', counts: { idle: 1 } }], workspaces: [], links: [] }),
        BusGroupMessages: async () => messages.slice(), BusMarkGroupRead: async () => {}
      };
    }
  });
  const { page, ok } = t;
  try {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'bus-room', path: 'reading' }));
    const win = page.locator('.win[data-win-kind="bus-room"]');
    const log = win.locator('.bus-log');
    await win.locator('[data-bus-message="12"]').waitFor({ state: 'attached' });
    const atEnd = () => log.evaluate(el => el.scrollHeight - el.scrollTop - el.clientHeight < 24);
    const endSettled = () => page.waitForFunction(() => {
      const el = document.querySelector('.win[data-win-kind="bus-room"] .bus-log');
      return el && el.dataset.following === 'true' && el.scrollHeight - el.scrollTop - el.clientHeight < 24 && !el.querySelector('.transcript-unread-divider');
    });
    const unseen = () => win.locator('.transcript-unread-divider').innerText().then(text => Number(text.match(/\d+/)[0]));
    const append = async count => {
      const before = await win.locator('.bus-message').count();
      await page.evaluate(n => __appendReports(n), count);
      await page.waitForFunction(n => document.querySelectorAll('.win[data-win-kind="bus-room"] .bus-message').length === n, before + count);
    };
    const back = async () => {
      await log.focus(); await page.keyboard.press('Control+Home');
      await page.waitForFunction(() => document.querySelector('.win[data-win-kind="bus-room"] .bus-log').scrollTop < 2);
    };
    await endSettled();
    ok('a fresh group follows its current end without unread marks', await atEnd());
    await back(); await append(5);
    await win.locator('.transcript-unread-divider').waitFor();
    ok('every new message is counted while the reader is scrolled back', await unseen() === 5);
    ok('blue side bars are absent', await win.locator('.bus-unseen').count() === 0);
    await win.getByRole('button', { name: 'Jump to Where You Stopped', exact: true }).click();
    console.log('after unread jump', JSON.stringify(await logGeometry(page)));
    await page.waitForFunction(() => {
      const divider = document.querySelector('.win[data-win-kind="bus-room"] .transcript-unread-divider');
      return divider && Number(divider.textContent.match(/\d+/)[0]) < 5;
    });
    await log.focus();
    await page.keyboard.press('PageDown');
    console.log('after unread PageDown', JSON.stringify(await logGeometry(page)));
    await page.waitForFunction(() => {
      const divider = document.querySelector('.win[data-win-kind="bus-room"] .transcript-unread-divider');
      return divider && Number(divider.textContent.match(/\d+/)[0]) < 5;
    });
    ok('reading partway reduces the count and moves the divider', await unseen() > 0 && await win.locator('.transcript-unread-divider').evaluate(el => Number(el.nextElementSibling.dataset.busMessage) > 13));
    await t.shot('group-unread-partway');
    await log.focus(); await page.keyboard.press('Control+End'); await endSettled();
    ok('keyboard navigation to the end clears unread and resumes following', true);
    for (const input of ['wheel', 'scrollbar', 'jump']) {
      await back(); await append(2); await win.locator('.transcript-unread-divider').waitFor();
      if (input === 'jump') await win.getByRole('button', { name: 'Jump to the latest', exact: true }).click();
      else if (input === 'wheel') {
        await log.hover(); await page.mouse.wheel(0, 100000);
      } else {
        // Drag the native scrollbar thumb from its top to its bottom.
        await log.evaluate(el => { el.style.scrollbarGutter = 'stable'; el.style.scrollbarWidth = 'auto'; });
        await log.evaluate(el => { window.__barEvents = []; ['pointerdown', 'mousedown', 'scroll', 'pointerup'].forEach(name => el.addEventListener(name, event => __barEvents.push({ name, top: el.scrollTop, x: event.clientX, y: event.clientY }))); });
        const r = await log.evaluate(el => { const r = el.getBoundingClientRect(); return { x: r.x, y: r.y, width: r.width, height: r.height, gutter: el.offsetWidth - el.clientWidth }; });
        console.log('native scrollbar geometry', JSON.stringify(r));
        await t.shot('group-scrollbar-before-drag');
        const x = r.x + r.width - Math.max(5, r.gutter / 2);
        await page.mouse.move(x, r.y + r.gutter * 2);
        await page.mouse.down(); await page.mouse.move(x, r.y + r.height - 4, { steps: 20 }); await page.mouse.up();
        console.log('native scrollbar events', await page.evaluate(() => JSON.stringify(__barEvents)));
      }
      await endSettled();
      ok(input + ' reaches the end and clears every unread message', true);
      await append(1); await endSettled();
      ok('later messages keep following after ' + input, true);
    }
    await t.shot('group-transcript-latest');
    // Saving while following must survive a larger history on reopen.
    await page.evaluate(k => pudlWindows.close(k), key);
    await page.evaluate(() => __appendReports(3));
    await page.evaluate(() => dinahWorkbench.open({ kind: 'bus-room', path: 'reading' }));
    await endSettled();
    ok('reopening a saved following transcript uses the end of the current content', await atEnd());
  } catch (error) {
    console.error('transcript geometry', JSON.stringify(await logGeometry(page))); await t.shot('group-unread-failure'); throw error;
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
function logGeometry(page) {
  return page.evaluate(() => Array.from(document.querySelectorAll('[data-follow-end]')).map(el => ({ top: el.scrollTop, height: el.clientHeight, extent: el.scrollHeight, following: el.dataset.following, divider: el.querySelector('.transcript-unread-divider')?.textContent,
    active: pudlWindows.state().top, own: el.closest('.win')?.dataset.win, visibility: document.visibilityState,
    rows: Array.from(el.querySelectorAll('[data-bus-message]')).slice(-5).map(row => ({ id: row.dataset.busMessage, top: row.getBoundingClientRect().top - el.getBoundingClientRect().top, height: row.getBoundingClientRect().height })) })));
}
