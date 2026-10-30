// Permission questions beside the chat's composer: visible with
// Notifications closed, bounded in height, answered through one route,
// retried after a failed answer, and isolated between chats.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1200, height: 850 },
    app: () => ({
      OpenSessionAt: async (ws, id) => ({ sessionId: id, provider: 'codex', cwd: ws, title: id, open: true, working: true }),
      Answer: async (key, option) => { window.__answers.push([key, option]); if (window.__failAnswer) throw Error('Answer could not be sent'); },
    }),
    init: () => { window.__answers = []; },
    ready: 'chat',
  });
  const { page, ok } = t;
  try {
    await page.evaluate(async () => {
      __emit('agent:session', { sessionId: 'first', cwd: 'C:/Fixture', provider: 'codex', title: 'Release Notes', open: true, working: true });
      await dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'first' });
      pudlWindows.snap(dinahWorkbench.key({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'first' }), 'maximized');
      window.__ask = (key, sessionId = 'first') => __emit('agent:permission', { key, sessionId, toolCall: { title: 'Write release-notes.md' }, options: [{ optionId: 'yes', name: 'Allow Once', kind: 'allow_once' }, { optionId: 'no', name: 'Reject', kind: 'reject_once' }] });
    });
    const first = page.locator('.win[data-chat-current]');
    await first.locator('[data-view-id=prompt]').focus();
    await page.evaluate(() => __ask('approval-1'));
    await first.locator('[data-view-id=chat-approvals]').waitFor();
    ok('approval is visible in its chat while Notifications is closed', await first.getByText('Waiting for Your Approval', { exact: true }).isVisible() && !await page.locator('[data-win="needs-you"]').isVisible());
    ok('the chat status explains the wait and typing focus is preserved', (await first.locator('[data-view-id=page-state]').textContent()).includes('Waiting for your approval.') && await first.locator('[data-view-id=prompt]').evaluate(el => el === document.activeElement));
    ok('a short question fits without clipping its choices', await first.locator('[data-view-id=chat-approvals]').evaluate(el => el.clientHeight >= 60 && el.scrollHeight <= el.clientHeight + 1));
    await page.evaluate(() => __emit('agent:permission', { key: 'long-question', sessionId: 'first', toolCall: { title: 'Please review this question. '.repeat(300) }, options: [{ optionId: 'ok', name: 'Answer the Long Question', kind: 'allow_once' }] }));
    await first.getByRole('button', { name: 'Answer the Long Question' }).waitFor({ state: 'attached' });
    ok('long questions scroll within half the output space', await first.locator('[data-view-id=chat-approvals]').evaluate(el => el.scrollHeight > el.clientHeight && el.getBoundingClientRect().height <= el.parentElement.getBoundingClientRect().height / 2 + 1));
    await first.getByRole('button', { name: 'Answer the Long Question' }).scrollIntoViewIfNeeded();
    ok('long question choices remain reachable', await first.getByRole('button', { name: 'Answer the Long Question' }).isVisible());
    await page.setViewportSize({ width: 390, height: 844 });
    ok('narrow chats retain bounded questions and a usable output area', await first.locator('[data-view-id=chat-approvals]').evaluate(el => el.clientHeight > 30 && el.getBoundingClientRect().height <= el.parentElement.getBoundingClientRect().height / 2 + 1 && el.scrollWidth <= el.clientWidth + 1));
    await page.setViewportSize({ width: 1200, height: 850 });
    await page.evaluate(() => __emit('agent:answered', { sessionId: 'first', key: 'long-question' }));
    await t.shot('chat-approvals');
    await first.getByRole('button', { name: 'Allow Once', exact: true }).click();
    await page.waitForFunction(() => __answers.length === 1);
    ok('one shared answer route submits the correct request and disables both views', JSON.stringify(await page.evaluate(() => __answers)) === '[["approval-1","yes"]]' && await page.locator('[data-permission-key="approval-1"] .permission-option:enabled').count() === 0);
    await page.evaluate(() => __emit('agent:answered', { sessionId: 'first', key: 'approval-1' }));
    await page.waitForFunction(() => !document.querySelector('[data-permission-key="approval-1"]'));
    ok('answer acknowledgement removes both copies', await page.locator('[data-permission-key="approval-1"]').count() === 0 && await first.locator('[data-view-id=chat-approvals]').isHidden());
    ok('an empty question area occupies no space', await first.locator('[data-view-id=chat-approvals]').evaluate(el => el.getBoundingClientRect().height === 0));
    await page.evaluate(() => { __ask('cancel-me'); __emit('agent:answered', { sessionId: 'first', key: 'cancel-me', cancelled: true }); });
    ok('cancellation removes inline and Notifications requests', await page.locator('[data-permission-key="cancel-me"]').count() === 0);
    await page.evaluate(() => { window.__failAnswer = true; __ask('retry-me'); });
    await first.getByRole('button', { name: 'Allow Once', exact: true }).click();
    await page.waitForFunction(() => document.querySelectorAll('[data-permission-key="retry-me"] .permission-option:enabled').length === 4);
    ok('a failed answer restores both sets of choices', await page.locator('[data-permission-key="retry-me"] .permission-option:enabled').count() === 4);
    await page.evaluate(() => { __emit('agent:answered', { sessionId: 'first', key: 'retry-me' }); __emit('agent:session', { sessionId: 'other', cwd: 'C:/Fixture', provider: 'codex', title: 'Other Chat', open: true }); });
    await first.locator('[data-view-id=prompt]').focus();
    await page.evaluate(() => __ask('other-request', 'other'));
    await page.locator('#questions [data-permission-key=other-request]').waitFor({ state: 'attached' });
    ok('another chat request does not appear here or steal focus', await first.locator('[data-view-id=chat-approvals]').isHidden() && await first.locator('[data-view-id=prompt]').evaluate(el => el === document.activeElement));
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'other' }));
    await page.locator('.win[data-chat-current] [data-view-id=chat-approvals]').waitFor();
    ok('opening the owning chat shows its pending request', await page.locator('.win[data-chat-current] [data-permission-key="other-request"]').isVisible());
    await page.evaluate(() => __emit('agent:state', { provider: 'codex', state: 'stopped', detail: 'Codex stopped' }));
    await page.waitForFunction(() => !document.querySelector('[data-permission-key="other-request"]'));
    ok('provider stop removes both copies', await page.locator('[data-permission-key="other-request"]').count() === 0);
  } finally { await t.close(); }
})();
