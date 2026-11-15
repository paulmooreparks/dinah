const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
(async () => {
  for (const theme of ['light', 'dark']) for (const width of [1440, 390]) {
    const t = await setup([
      makeRequest('r2', { createdAt: '2026-11-14T02:00:00Z', kind: 'mention', questions: [], text: Array.from({ length: 30 }, (_, n) => 'Message line ' + n).join('\n') }),
      makeRequest('r1', { createdAt: '2026-11-14T01:00:00Z', questions: [{ number: 1, text: 'Which release?', recommendation: 'Use the reviewed release.', choices: ['Reviewed', 'Wait'], answered: false, answer: '' }] })
    ]);
    const { page: p, ok } = t;
    try {
      await p.setViewportSize({ width, height: 1000 });
      await p.evaluate(theme => document.documentElement.dataset.theme = theme, theme);
      await p.locator('.decisions-row').first().waitFor();
      ok('requests are oldest first ' + width + theme, await p.locator('.decisions-row').first().getAttribute('data-request-id') === 'r1');
      const firstActions = p.locator('[data-request-id="r1"] [data-decision-actions]');
      ok('Actions is exposed as a Tab stop', await firstActions.evaluate(el => el.tabIndex === 0 && el.getAttribute('aria-hidden') !== 'true'));
      await p.locator('[data-request-id="r1"] a').focus(); await p.keyboard.press('Tab');
      ok('Tab reaches the row Actions button', await firstActions.evaluate(el => el === document.activeElement));
      await p.evaluate(() => { __listFailure = true; __emit('bus:changed'); });
      const notice = p.locator('.decisions-list-notice');
      await notice.waitFor({ state: 'visible' });
      ok('a failed read with no selection has a visible list notice', (await notice.innerText()).includes('Request service unavailable') && await notice.evaluate(el => el.getBoundingClientRect().width > 1 && el.getBoundingClientRect().height > 1) && await p.locator('.decisions-row').count() === 2);
      await p.evaluate(() => { __listFailure = false; });
      await notice.getByRole('button', { name: 'Refresh', exact: true }).click();
      await notice.waitFor({ state: 'hidden' });
      ok('a successful list retry clears the notice and retains the rows', await p.locator('.decisions-row').count() === 2);
      await choose(t, 'r1');
      ok('one radio set contains recommendation, suggestions and own answer', await p.locator('[data-question="1"] input[type=radio]').count() === 4 && await p.locator('[data-question="1"] select').count() === 0);
      const send = p.getByRole('button', { name: 'Send', exact: true });
      ok('unavailable settlement stays visible with a reason', await p.getByRole('button', { name: 'Dismiss', exact: true }).isDisabled() && (await p.getByRole('button', { name: 'Dismiss', exact: true }).getAttribute('title')).includes('not available'));
      await p.getByLabel('Answer 1', { exact: true }).fill('Keep this draft.');
      await p.evaluate(() => dinahUIState.flush());
      await p.waitForFunction(() => !document.querySelector('.decisions-foot .btn-primary').disabled);
      const before = await send.boundingBox();
      await p.evaluate(() => { window.__holdSave = true; });
      await p.getByLabel('Answer 1', { exact: true }).fill('Keep the latest draft.');
      await p.waitForFunction(() => !!window.__releaseSave);
      ok('draft saving neither disables nor moves Send', !await send.isDisabled() && JSON.stringify(before) === JSON.stringify(await send.boundingBox()));
      const line = await p.locator('.decisions-foot-status').innerText();
      ok('saving does not flash the status line', line === 'Your answer is ready to send.');
      await p.evaluate(() => { __holdSave = false; __releaseSave(); });
      await p.evaluate(() => dinahUIState.flush());
      if (width === 390) {
        await p.getByRole('button', { name: 'About This Request', exact: true }).click();
        ok('About has its own address and pane', await p.evaluate(() => new URL(location.href).searchParams.get('view.decisions').includes('pane=about')) && await p.locator('.decisions-about').isVisible() && !await p.locator('.decisions-split > .decisions-scroll').isVisible());
        ok('About focuses Back to Request', await p.getByRole('button', { name: 'Back to Request', exact: true }).evaluate(el => el === document.activeElement));
        await p.getByRole('button', { name: 'Back to Request', exact: true }).click();
        ok('Back restores About focus', await p.getByRole('button', { name: 'About This Request', exact: true }).evaluate(el => el === document.activeElement));
      }
      const footerBefore = await p.locator('.decisions-foot').boundingBox();
      await p.locator('.decisions-scroll').evaluate(el => { el.scrollTop = el.scrollHeight; });
      ok('the footer stays in its viewport', JSON.stringify(footerBefore) === JSON.stringify(await p.locator('.decisions-foot').boundingBox()) && footerBefore.x >= 0 && footerBefore.x + footerBefore.width <= width + 1);
      await t.shot('decisions-redesign-' + width + '-' + theme);
      await send.click();
      await p.waitForFunction(() => window.__delivered.length === 1);
      await p.waitForFunction(() => document.querySelector('.decisions-row a[aria-current="true"]').parentElement.dataset.requestId === 'r2');
      ok('settlement automatically opens the next request and removes the finished row', await p.locator('.decisions-row[data-request-id="r1"]').count() === 0 && await p.getByLabel('Your Reply', { exact: true }).isVisible());
      ok('completion names the prior asking chat and destination', (await p.locator('.decisions-foot-status').innerText()).includes('Review Group, replying to Review r1'));
      await p.evaluate(() => { __saveFailure = true; });
      await p.getByLabel('Your Reply', { exact: true }).fill('Retain this unsaved draft.');
      await p.waitForFunction(() => document.querySelector('.decisions-foot-status').textContent.includes('Your draft could not be saved.'));
      ok('current save errors take priority over the prior completion', await send.isDisabled() && !(await p.locator('.decisions-foot-status').innerText()).includes('Your reply was sent') && await p.getByLabel('Your Reply', { exact: true }).inputValue() === 'Retain this unsaved draft.');
      await p.evaluate(() => { __saveFailure = false; });
      await p.getByLabel('Your Reply', { exact: true }).fill('Retain this saved draft.');
      await p.waitForFunction(() => !document.querySelector('.decisions-foot .btn-primary').disabled);
      ok('a plain message is quoted once', await p.locator('.decisions-message').count() === 1 && (await p.locator('.decisions-message').innerText()).includes('Message line 29'));
      await p.getByRole('button', { name: 'Show the Whole Message' }).click();
      ok('long-message expansion stays in the main scroll', await p.locator('.decisions-message').getAttribute('data-folded') === 'false' && await p.locator('.decisions-message').evaluate(el => getComputedStyle(el).overflowY !== 'auto'));
      if (width === 390) await p.getByRole('button', { name: 'Back to Requests', exact: true }).click();
      const actions = p.locator('[data-request-id="r2"] [data-decision-actions]');
      await actions.focus(); await p.keyboard.press('Shift+F10');
      await p.getByRole('menuitem', { name: /^Open Task/ }).waitFor();
      ok('row keyboard menu retains disabled commands and reasons', await p.getByRole('menuitem', { name: /^Open Task/ }).getAttribute('aria-disabled') === 'true');
      await p.keyboard.press('Escape');
      ok('row menu restores its actual Actions opener', await actions.evaluate(el => el === document.activeElement));
      await p.evaluate(() => {
        const q = { key: 'text-q', service: 'fixture.review', serviceName: 'Review Service', workspace: 'fixture', workspaceName: 'Harbor', task: 'review', generation: 'g1', taskTitle: 'Review release', question: 'Write a short answer.', askedAt: '2026-11-14T03:00:00Z', target: { reason: 'This service has no task page.' }, answerType: 'text', minLength: 1, maxLength: 3, options: null };
        window.__taskPending = [q]; window.__typedAnswers = [];
        go.main.App.PendingTaskPermissions = async () => JSON.parse(JSON.stringify(__taskPending));
        go.main.App.AnswerTaskQuestion = async (key, raw) => { __typedAnswers.push([key, JSON.parse(raw)]); __taskPending = __taskPending.filter(q => q.key !== key); __emit('task:answered', { key }); };
        __emit('task:permission', q);
      });
      await choose(t, 'task:text-q');
      const text = p.locator('.decisions-detail').getByLabel('Your Answer', { exact: true });
      await text.fill('😀ab');
      ok('text limits count Unicode scalars and retain all input', await p.locator('.decisions-detail [data-task-count]').innerText() === '3 of 3 characters (minimum 1)' && !await send.isDisabled());
      await text.fill('😀abc');
      ok('over-limit answers are visible and cannot be sent', await text.inputValue() === '😀abc' && await send.isDisabled() && await text.getAttribute('aria-invalid') === 'true');
      await text.fill('😀ab');
      await p.evaluate(() => dinahWorkbench.open('needs-you'));
      const notification = p.locator('#questions [data-task-question-key="text-q"] textarea');
      await notification.waitFor({ state: 'visible' });
      ok('Notifications shares the Decisions draft', await notification.inputValue() === '😀ab');
      await notification.fill('Yes');
      await p.evaluate(() => dinahWorkbench.open('decisions'));
      ok('edits in Notifications return to Decisions', await text.inputValue() === 'Yes');
      await send.click();
      await p.waitForFunction(() => __typedAnswers.length === 1);
      ok('typed text uses only the trusted typed entry point', await p.evaluate(() => JSON.stringify(__typedAnswers[0]) === '["text-q",{"type":"text","text":"Yes"}]'));
      if (width === 390) await p.getByRole('button', { name: 'Back to Requests', exact: true }).click();
      await p.evaluate(() => {
        const q = { key: 'multi-q', service: 'fixture.review', serviceName: 'Review Service', workspace: 'fixture', workspaceName: 'Harbor', task: 'review', generation: 'g1', taskTitle: 'Review release', question: 'Choose the checks.', askedAt: '2026-11-14T04:00:00Z', target: { reason: 'No task page.' }, answerType: 'multiple', minSelections: 1, maxSelections: 2, options: ['A', 'B', 'C'].map(name => ({ optionId: name, name, kind: 'allow_once' })) };
        __taskPending.push(q); __emit('task:permission', q);
      });
      await choose(t, 'task:multi-q');
      await p.locator('.decisions-detail').getByRole('checkbox', { name: 'C', exact: true }).check();
      await p.locator('.decisions-detail').getByRole('checkbox', { name: 'A', exact: true }).check();
      await p.locator('.decisions-detail').getByRole('checkbox', { name: 'B', exact: true }).check();
      ok('multiple selection keeps an over-limit choice visible', await p.locator('.decisions-detail input:checked').count() === 3 && await send.isDisabled());
      await p.locator('.decisions-detail').getByRole('checkbox', { name: 'B', exact: true }).uncheck();
      await send.click(); await p.waitForFunction(() => __typedAnswers.length === 2);
      await p.evaluate(() => { __decisionItems = []; __emit('bus:changed'); });
      await p.locator('.decisions-empty').waitFor({ state: 'visible' });
      ok('empty state explains which requests appear', (await p.locator('.decisions-empty').innerText()).includes('When an agent asks you a question or mentions you in a group'));
      ok('multiple answers retain offered order', await p.evaluate(() => JSON.stringify(__typedAnswers[1]) === '["multi-q",{"type":"multiple","optionIds":["A","C"]}]'));
    } finally { await t.close(); }
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
