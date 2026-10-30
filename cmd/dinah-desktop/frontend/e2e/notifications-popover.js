// The Notifications item's popover lists the notifications themselves:
// questions first, then the rest, each newest first; a glyph and a colour
// per type; descriptions clamped to two lines with the whole text in the
// row's name and tooltip; a question opening its chat and a report opening
// the pane; the arrow keys and Escape; the empty state; the badge; and
// rows that change while the popover is open.
const { launch, OUT } = require('./lib/page');
const path = require('path');
const {openStatus}=require('./lib/status');
(async () => {
  const t = await launch({
    viewport: { width: 1300, height: 900 },
    app: () => ({
      OpenSessionAt: async (ws, id) => ({ sessionId: id, provider: 'codex', cwd: ws, title: id, open: true, working: true }),
      Answer: async () => {},
      Notifications: async () => [],
      SaveNotification: async n => { __store.push(n); return n; },
      DismissNotification: async id => { const n = __store.find(x => x.id === id); if (n) n.dismissed = true; },
    }),
    init: () => { window.__store = []; },
    ready: 'workbench',
  });
  const { page, ok } = t;
  try {
    const menu = page.locator('#notifications-status-menu');
    const rows = menu.locator('.notices-row');
    const open = async () => { if (!await menu.evaluate(m => m.matches(':popover-open'))) await openStatus(page,'status-needs'); await page.waitForFunction(() => { const m = document.querySelector('#notifications-status-menu'); return m.matches(':popover-open') && !m.classList.contains('placing'); }); };
    const close = async () => { if (await menu.evaluate(m => m.matches(':popover-open'))) await menu.evaluate(m => m.hidePopover()); };

    // Empty.
    await open();
    ok('an empty list says so in one line', await menu.locator('.notices-empty').isVisible() && await menu.locator('.notices-empty').textContent() === 'No notifications.' && await rows.count() === 0);
    ok('the badge is hidden with nothing waiting', await page.locator('#status-needs [data-status-badge]').isHidden());
    await close();

    // A report of each kind, then two questions.
    const long = 'The workspace index could not be rebuilt because the folder C:/Fixture/very/deep/path/with/many/parts/that/goes/on/and/on was locked by another program while Dinah was reading it, so the search results may be stale until it is rebuilt.';
    await page.evaluate(long => {
      dinahHost.toast('Saved "notes.md".', 'positive');
      dinahHost.toast('Choose a workspace first.', 'warn');
      dinahHost.toast('The export finished.', 'info');
      dinahHost.toast(long, 'danger');
      __emit('agent:session', { sessionId: 'chat-a', cwd: 'C:/Fixture', provider: 'codex', title: 'Release Notes', open: true });
      __emit('agent:session', { sessionId: 'chat-b', cwd: 'C:/Fixture', provider: 'codex', title: 'Parser Work', open: true });
      __emit('agent:permission', { sessionId: 'chat-a', key: 'qa', toolCall: { title: 'Write release-notes.md' }, options: [{ optionId: 'yes', name: 'Allow Once', kind: 'allow_once' }] });
    }, long);
    await page.waitForTimeout(20);
    await page.evaluate(() => __emit('agent:permission', { sessionId: 'chat-b', key: 'qb', toolCall: { title: 'Run go test ./parser' }, options: [{ optionId: 'yes', name: 'Allow Once', kind: 'allow_once' }] }));
    await page.waitForFunction(() => document.querySelector('#status-needs .badge').textContent === '3');
    ok('the badge counts the questions and the errors', await page.textContent('#status-needs .badge') === '3');

    await open();
    const listed = await rows.evaluateAll(rs => rs.map(r => r.dataset.noticeType + ':' + r.textContent));
    ok('questions come first, newest first, then the reports newest first',
      listed.length === 6 && listed[0] === 'question:The agent wants to: Run go test ./parser' && listed[1] === 'question:The agent wants to: Write release-notes.md' &&
      listed[2].startsWith('error:The workspace index') && listed[3] === 'info:The export finished.' && listed[4] === 'warning:Choose a workspace first.' && listed[5] === 'success:Saved "notes.md".');
    const paneOrder = await page.evaluate(() => [...document.querySelectorAll('#questions > .notice')].map(c => c.dataset.noticeId));
    const listOrder = await rows.evaluateAll(rs => rs.map(r => r.dataset.noticeId));
    ok('the list and the pane have the same order', JSON.stringify(paneOrder) === JSON.stringify(listOrder));

    // Glyph and colour per type; the colour is the type's token.
    const looks = await rows.evaluateAll(rs => rs.map(r => {
      const g = r.querySelector('.notices-glyph'), probe = document.createElement('span');
      probe.style.color = { question: 'var(--warn)', warning: 'var(--warn)', error: 'var(--danger)', success: 'var(--positive)', info: 'var(--accent)' }[r.dataset.noticeType];
      r.appendChild(probe);
      const token = getComputedStyle(probe).color; probe.remove();
      return { type: r.dataset.noticeType, colour: getComputedStyle(g).backgroundColor, token, glyph: getComputedStyle(g).maskImage };
    }));
    ok('each row has its type\'s colour', looks.every(l => l.colour === l.token));
    ok('each type has a glyph of its own, so colour is not the only signal', looks.every(l => l.glyph.includes('svg')) && new Set(looks.map(l => l.type + l.glyph)).size === new Set(looks.map(l => l.glyph)).size && new Set(looks.map(l => l.glyph)).size === 5);

    // The long description is clamped to two lines and reachable whole.
    const clamp = await menu.locator('.notices-row[data-notice-type="error"]').evaluate(r => {
      const t = r.querySelector('.notices-text'), lh = parseFloat(getComputedStyle(t).lineHeight);
      return { lines: Math.round(t.clientHeight / lh), cut: t.scrollHeight > t.clientHeight + 1, label: r.getAttribute('aria-label'), title: r.title };
    });
    ok('a long description is clamped to two lines', clamp.lines === 2 && clamp.cut);
    ok('the whole text is in the row\'s name and tooltip', clamp.label === 'Error: ' + long && clamp.title === long);
    ok('a question\'s name says its type and its chat', await rows.first().getAttribute('aria-label') === 'Question: The agent wants to: Run go test ./parser, Parser Work');
    for (const theme of ['light', 'dark']) {
      await page.evaluate(t => pudlSetTheme(t), theme);
      await page.waitForTimeout(100);
      await t.shot('notifications-popover-' + theme);
    }
    await page.evaluate(() => pudlSetTheme('light'));

    // The keyboard: the popover opens with focus on its first row; the
    // arrows move; Escape closes and returns to the item.
    await close();
    await page.focus('#status-needs');
    await page.keyboard.press('Enter');
    await page.waitForFunction(() => document.querySelector('#notifications-status-menu').matches(':popover-open'));
    await page.waitForFunction(() => document.activeElement && document.activeElement.classList.contains('notices-row'));
    ok('opening the list puts focus on its first row', await rows.first().evaluate(r => r === document.activeElement));
    await page.keyboard.press('ArrowDown');
    ok('Down moves to the next row', await rows.nth(1).evaluate(r => r === document.activeElement));
    await page.keyboard.press('ArrowUp');
    ok('Up moves back', await rows.first().evaluate(r => r === document.activeElement));

    // Live: a new entry while the list is open appears in place, and the
    // focused row keeps focus.
    await page.keyboard.press('ArrowDown');
    await page.evaluate(() => dinahHost.toast('A new report arrived.', 'info'));
    await menu.locator('.notices-row', { hasText: 'A new report arrived.' }).waitFor();
    ok('a new entry appears while the list is open', (await rows.nth(2).textContent()) === 'A new report arrived.');
    ok('the focused row keeps focus as the list changes', await page.evaluate(() => document.activeElement.dataset.noticeId === 'question-qa'));
    await page.evaluate(() => __emit('agent:answered', { key: 'qb' }));
    await page.waitForFunction(() => !document.querySelector('#notifications-status-menu [data-notice-id="question-qb"]'));
    const after = await page.evaluate(() => [document.querySelector('#status-needs .badge').textContent, document.activeElement.dataset.noticeId, dinahHost.notices.list().map(e => e.type + ':' + e.description).join(' | ')]);
    ok('an answered question leaves the open list and the badge', after[0] === '2' && after[1] === 'question-qa');
    if (after[0] !== '2' || after[1] !== 'question-qa') console.log(after.join('\n'));
    await page.keyboard.press('Escape');
    await page.waitForFunction(() => !document.querySelector('#notifications-status-menu').matches(':popover-open'));
    ok('Escape closes the list and focus returns to the item', await page.evaluate(() => document.activeElement && document.activeElement.id === 'status-needs'));

    // A question opens its chat with the question in view.
    await open();
    await menu.locator('.notices-row[data-notice-id="question-qa"]').click();
    const chat = page.locator('.win[data-chat-current]');
    await chat.locator('[data-view-id=chat-approvals] [data-permission-key="qa"]').waitFor();
    ok('a question opens its chat with the question in view', await chat.locator('[data-view-id=chat-approvals] [data-permission-key="qa"]').isVisible() &&
      await page.evaluate(() => { const w = document.querySelector('.win[data-chat-current]'); return dinahWorkbench.item(w.dataset.win).path === 'chat-a'; }));
    ok('the popover closes when a row is chosen', !await menu.evaluate(m => m.matches(':popover-open')));

    // A report opens the pane at its entry.
    await open();
    await menu.locator('.notices-row', { hasText: 'The export finished.' }).click();
    await page.locator('[data-win="needs-you"]').waitFor({ state: 'visible' });
    const card = page.locator('#questions .notice', { hasText: 'The export finished.' });
    await page.waitForFunction(() => { const c = [...document.querySelectorAll('#questions .notice')].find(n => n.textContent.includes('The export finished.')); return c && c.contains(document.activeElement); });
    ok('a report opens the pane with focus in its card', await card.isVisible());
    await page.evaluate(() => document.querySelectorAll('.toast').forEach(t => t.remove()));
    await page.locator('[data-win="needs-you"]').screenshot({ path: path.join(OUT, 'notifications-pane-full.png') });

    // Show All and Clear All.
    await open();
    ok('Clear All is offered while there are reports', !await menu.locator('[data-cmd="clear-notifications"]').isDisabled());
    await menu.locator('[data-cmd="clear-notifications"]').click();
    await page.locator('#ask[open]').waitFor();
    await page.click('#ask-yes');
    await page.waitForFunction(() => document.querySelectorAll('#notifications-status-menu .notices-row').length === 1);
    await open();
    ok('Clear All leaves only the question', await rows.count() === 1 && await rows.first().getAttribute('data-notice-type') === 'question' && await menu.locator('[data-cmd="clear-notifications"]').isDisabled());
    await menu.locator('[data-cmd="needs-you"]').click();
    ok('Show All opens Notifications', await page.locator('[data-win="needs-you"]').isVisible());
  } catch (err) {
    console.log('FAIL ' + String(err && err.stack || err));
    t.failures.push('exception');
  } finally { await t.close(); }
})();
