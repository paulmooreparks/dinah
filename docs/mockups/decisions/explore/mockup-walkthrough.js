// Uses the live mockup headless the way Paul would, and checks each result:
// choosing requests, answering, sending, a partial answer, dismissing, the
// record of a finished request, filtering, Show More, the Try menu (arrivals,
// a task question, a cancelled task, a changed request, an unconfirmed send),
// emptying the list, the phone layout's back link and About pane, the rows' menus, Dismiss
// for every entry, and a request its chat withdraws.
//   pwsh scripts/with-test-lock.ps1 node docs/mockups/decisions/explore/mockup-walkthrough.js
const path = require('path');
const E2E = path.join(__dirname, '..', '..', '..', '..', 'cmd', 'dinah-desktop', 'frontend', 'e2e');
const { chromium } = require('playwright-core');
const { acquire } = require(path.join(E2E, 'lib', 'browser-lock'));
const ROOT = path.join(__dirname, '..', '..', '..', '..');
const EDGE = 'msedge';
const PAGE = 'http://mock.test/docs/mockups/decisions/decisions-redesign.html';
let failures = 0;
function check(ok, what) { console.log((ok ? 'ok   ' : 'FAIL ') + what); if (!ok) failures++; }
(async () => {
  const release = await acquire();
  const browser = await chromium.launch({ channel: EDGE, headless: true });
  try {
    const context = await browser.newContext({ viewport: { width: 1440, height: 1000 } });
    await context.route('http://mock.test/**', route => route.fulfill({ path: path.join(ROOT, decodeURIComponent(new URL(route.request().url()).pathname)) }));
    const page = await context.newPage();
    const errors = []; page.on('pageerror', e => errors.push(String(e)));
    const rows = () => page.locator('.decisions-rows a.md-item').count();
    const title = () => page.locator('.decisions-head h2').textContent();
    const status = () => page.locator('.decisions-foot-status').textContent();
    const sendBtn = () => page.locator('[data-act="send"]');
    const tryIt = v => page.locator('.mock-try select').selectOption(v);

    await page.goto(PAGE + '?state=index');
    check(await page.evaluate(() => document.body.className === 'index'), '?state=index lists the states');
    await page.goto(PAGE);
    check(await rows() === 5, 'the plain file path opens on several requests, with five rows');
    check(await sendBtn().isDisabled(), 'Send starts disabled');

    // A message: reply, send, it leaves and the next request opens.
    await page.locator('a.md-item[data-key="m102"]').click();
    check(await title() === 'Dinah Design Review', 'choosing the message opens it');
    check((await page.url()).includes('request=mention%3A102'), 'the address follows the open request');
    await page.locator('[data-act="fold"]').click();
    check(await page.locator('.decisions-message').getAttribute('data-folded') === 'false', 'the message unfolds in place');
    await page.locator('#reply').fill('Cut the paragraph and ship on time.');
    check(!(await sendBtn().isDisabled()), 'typing a reply enables Send');
    await sendBtn().click();
    check((await status()).startsWith('Sending your reply'), 'Send shows Sending');
    await page.waitForFunction(() => !document.querySelector('a.md-item[data-key="m102"]'));
    check(await rows() === 4, 'the sent message left the list');
    check(await title() === 'Harbor Companion Lead', 'the next request opened in its place');
    check((await status()).includes('Your reply was sent to Dinah Design Review'), 'the foot confirms the reply');
    await page.locator('.decisions-foot-status [data-act^="record:"]').click();
    check(await page.locator('.decisions-answered').first().textContent().then(t => t.includes('Cut the paragraph')), 'Show the Reply opens the record with the reply');
    check(await page.locator('a.md-item[aria-current]').count() === 0, 'no row is marked while the record is open');
    await page.locator('[data-act="back"]').click();
    check(await page.locator('#reply').count() === 0 && await page.locator('a.md-item[aria-current]').count() === 1, 'Back to Open Requests returns to the list');

    // A partial answer keeps the request open.
    await page.locator('a.md-item[data-key="r103"]').click();
    await page.locator('input[name="answer-2"][value="rec"]').check();
    check((await status()).startsWith('1 of 5 answered'), 'one of five answered');
    await sendBtn().click();
    await page.waitForFunction(() => /stay open/.test(document.querySelector('.decisions-foot-status').textContent));
    check(await page.locator('a.md-item[data-key="r103"] .badge').textContent() === '4 of 5 still open', 'the row says 4 of 5 still open');
    check(await page.locator('.decisions-answered').count() === 1, 'the answered question shows its answer');
    check(await sendBtn().isDisabled(), 'Send is disabled again with nothing left to send');

    // Own words: typing chooses My own answer.
    await page.locator('#answer-1-text').fill('A new project');
    check(await page.locator('input[name="answer-1"][value="own"]').isChecked(), 'typing chooses My own answer');

    // Dismissing asks first; Cancel keeps it, Dismiss closes it.
    await page.locator('a.md-item[data-key="r105"]').click();
    check(await sendBtn().isDisabled() && (await status()).includes('cannot be delivered'), 'a request that cannot be delivered keeps Send, disabled, and says why');
    await page.locator('[data-act="dismiss"]').click();
    await page.locator('#ask button[value="cancel"]').click();
    await page.waitForSelector('#ask', { state: 'detached' });
    check(await page.locator('#ask').count() === 0 && await rows() === 4, 'Cancel keeps the request');
    await page.locator('[data-act="dismiss"]').click();
    await page.locator('#ask button[value="yes"]').click();
    await page.waitForSelector('#ask', { state: 'detached' });
    check(await rows() === 3 && (await status()).includes('You dismissed the request from Old Migration Agent'), 'Dismiss closes it and says so');

    // Filtering, and a filter that matches nothing.
    await page.locator('#filter').pressSequentially('Harbor');
    check(await rows() === 1 && await page.locator('#filter').evaluate(e => document.activeElement === e), 'filtering keeps focus and shows one row');
    await page.locator('#filter').pressSequentially('zz');
    check(await page.locator('[data-act="clear-filter"]').count() === 1, 'a filter that matches nothing offers Clear Filter');
    await page.locator('[data-act="clear-filter"]').click();
    check(await rows() === 3, 'Clear Filter brings every row back');

    // The Try menu.
    await tryIt('arrive');
    check(await rows() === 4 && await page.locator('.decisions-rows a.md-item').last().getAttribute('data-key') === 'r108', 'a new request joins the end of the list');
    // Task questions arrive in the order the Try menu gives them: the two typed ones first.
    await tryIt('task'); await tryIt('task');
    check(await page.locator('.decisions-rows a.md-item').last().getAttribute('data-key') === 't113', 'the typed task questions arrive first');
    await tryIt('task');
    await page.locator('a.md-item[data-key="t106"]').click();
    check(await page.locator('[data-act="dismiss"]').count() === 1 && await sendBtn().count() === 1, 'a task question has the same Dismiss and Send as every entry');
    await page.locator('input[name="answer-task"]').nth(1).check();
    await tryIt('fail');
    await sendBtn().click();
    await page.waitForSelector('[data-act="check"]');
    check((await status()).includes('could not confirm'), 'an unconfirmed send says so and offers Check Send Result');
    await page.locator('[data-act="check"]').click();
    await page.waitForFunction(() => !document.querySelector('a.md-item[data-key="t106"]'));
    check((await status()).includes('Your answer was sent to Harbor Publisher'), 'checking finds it arrived and the question leaves');
    await tryIt('task');
    await page.locator('a.md-item[data-key="t107"]').click();
    check((await page.locator('.decisions-head').textContent()).includes('did not give a page'), 'a task with no page says why');
    await tryIt('cancel');
    check(!(await page.locator('a.md-item[data-key="t107"]').count()) && (await status()).includes('was cancelled'), 'a cancelled task closes its question and says so');
    await page.locator('a.md-item[data-key="r101"]').click();
    await tryIt('change');
    check(await page.locator('[data-act="ack"]').count() === 1 && await page.locator('.badge.warn', { hasText: 'Changed' }).count() === 1, 'a changed request marks the question and asks to read it');
    await page.locator('[data-act="ack"]').click();
    check(await page.locator('[data-act="ack"]').count() === 0, 'I Have Read the Change clears it');

    // Empty the list: the empty state keeps the confirmation.
    for (let i = 0; i < 10 && await rows() > 0; i++) {
      const key = await page.locator('.decisions-rows a.md-item').first().getAttribute('data-key');
      await page.locator('a.md-item[data-key="' + key + '"]').click();
      if (await page.locator('[data-act="dismiss"]').count()) { await page.locator('[data-act="dismiss"]').click(); await page.locator('#ask button[value="yes"]').click(); await page.waitForSelector('#ask', { state: 'detached' }); continue; }
      if (await page.locator('#answer-task-text').count()) await page.locator('#answer-task-text').fill('Leave out fiscal printing.');
      if (await page.locator('input[name="answer-task-pick"]').count()) await page.locator('input[name="answer-task-pick"]').first().check();
      const radios = await page.locator('input[type="radio"][value="rec"]').count();
      for (let j = 0; j < radios; j++) await page.locator('input[type="radio"][value="rec"]').nth(j).check();
      await sendBtn().click();
      await page.waitForFunction(k => !document.querySelector('a.md-item[data-key="' + k + '"]'), key);
    }
    check(await page.locator('.decisions-empty').count() === 1 && await page.locator('.decisions-done').count() === 1, 'the last request leaves and the empty state confirms it');
    await tryIt('arrive');
    check(await rows() === 1, 'a request arriving on the empty window opens the list again');

    // Show More Requests.
    await page.goto(PAGE + '?state=earlier');
    await page.locator('[data-act="more"]').click();
    check(await rows() === 7 && await page.locator('[data-act="more"]').count() === 0, 'Show More Requests adds the next page');

    // A reload keeps the open request.
    await page.locator('a.md-item[data-key="r104"]').click();
    await page.reload();
    check(await title() === 'Org Chart Builder', 'a reload keeps the open request');

    // The phone layout.
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(PAGE + '?state=several&pane=list');
    await page.locator('a.md-item[data-key="r103"]').click();
    check(await page.locator('.md-layout').getAttribute('data-md-pane') === 'detail', 'on a phone a row opens the request pane');
    await page.locator('[data-act="about"]').click();
    check(await page.locator('.decisions-about').isVisible() && await page.locator('.decisions-question').count() === 0 && await page.evaluate(() => document.activeElement && document.activeElement.dataset.act === 'about-back'), 'on a phone About This Request is its own pane, with focus on its way back');
    await page.locator('[data-act="about-back"]').click();
    check(await page.locator('.decisions-question').count() > 0 && await page.locator('.decisions-about').count() === 0, 'Back to the Request returns to the questions');
    await page.locator('a.md-back').click();
    check(await page.locator('.md-layout').getAttribute('data-md-pane') === 'list', 'Open Requests goes back to the list');

    // Typed task questions: free text with a limit counted in Unicode characters.
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(PAGE + '?state=task-text');
    const count = () => page.locator('#answer-task-count').textContent();
    // The line's place below the text area and its height (the text area itself grows with its text, as the reply box does).
    const countBox = () => page.locator('#answer-task-count').evaluate(p => { const r = p.getBoundingClientRect(), t = document.getElementById('answer-task-text').getBoundingClientRect(); return Math.round(r.y - t.bottom) + ',' + Math.round(r.height) + ',' + Math.round(r.x); });
    check((await count()).startsWith('67 of 280 characters') && !(await sendBtn().isDisabled()), 'a text answer within the limit counts its characters and can be sent');
    check(await page.locator('#answer-task-text').evaluate(t => !t.hasAttribute('maxlength')), 'the text area has no maxlength, which counts UTF-16 units instead of characters');
    const box1 = await countBox();
    await page.locator('#answer-task-text').fill('😀'.repeat(282));
    check((await count()).startsWith('282 of 280 characters, 2 over the limit') && await sendBtn().isDisabled() && (await status()).includes('2 characters over the limit of 280'), '282 emoji count as 282 characters: 2 over, and Send waits');
    check(await page.locator('#answer-task-count').evaluate(p => p.dataset.tone === 'danger' && !!p.querySelector('.glyph')), 'over the limit the line has a warning glyph and the danger colour');
    check((await countBox()) === box1, 'the line under the answer keeps its place below the text area and its height as the count changes');
    await page.locator('#answer-task-text').fill('');
    check(await sendBtn().isDisabled() && (await status()).includes('Write an answer to send it'), 'an empty answer waits, since this question requires one');
    await page.locator('#answer-task-text').fill('Leave out fiscal printing.');
    await sendBtn().click();
    await page.waitForFunction(() => !document.querySelector('a.md-item[data-key="t112"]'), null, { timeout: 5000 });
    check((await status()).includes('sent to Flow'), 'sending the text answer closes the question');
    await page.locator('.decisions-foot-status button.link').click();
    check((await page.locator('.decisions-answered').textContent()).includes('Leave out fiscal printing.'), 'the record shows the text as written');

    // Multiple selection with a minimum and a maximum.
    await page.goto(PAGE + '?state=task-multiple');
    await page.locator('input[name="answer-task-pick"][value="checkout"]').uncheck();
    check(await sendBtn().isDisabled() && (await count()).includes('0 chosen'), 'with nothing chosen Send waits');
    await page.locator('input[name="answer-task-pick"][value="reporting"]').check();
    await page.locator('input[name="answer-task-pick"][value="checkout"]').check();
    await page.locator('input[name="answer-task-pick"][value="loyalty"]').check();
    check(await sendBtn().isDisabled() && (await status()).includes('Choose at most 2. You chose 3.'), 'three chosen of at most two: Send waits and says why');
    await page.locator('input[name="answer-task-pick"][value="loyalty"]').uncheck();
    await sendBtn().click();
    await page.waitForFunction(() => !document.querySelector('a.md-item[data-key="t113"]'), null, { timeout: 5000 });
    await page.locator('.decisions-foot-status button.link').click();
    check((await page.locator('.decisions-answered').textContent()).includes('Checkout and payment, Reporting'), 'the record lists the choices in the order the service offered them');

    // Notifications: the same questions, one draft, one settlement.
    await page.goto(PAGE + '?state=notifications');
    check(await page.locator('#notes [data-note]').count() === 3, 'Notifications shows the three task questions as cards');
    await page.locator('#notes [data-note="t112"] textarea').fill('Leave out loyalty.');
    check(await page.locator('#answer-task-text').inputValue() === 'Leave out loyalty.' && (await count()).startsWith('18 of 280'), 'typing in the card is the same draft Decisions shows');
    await page.locator('#notes [data-act="note-answer:t112"]').click();
    check(await page.locator('a.md-item[data-key="t112"]').count() === 0 && await page.locator('#notes [data-note="t112"]').count() === 0 && (await status()).includes('from Notifications'), 'answering in Notifications settles it in both places');
    check((await page.locator('[data-notes-count]').textContent()) === '2', 'the Notifications count drops to two');

    // Every row's menu: right-click, Shift+F10 and the always-visible actions button.
    await page.setViewportSize({ width: 1440, height: 1000 });
    await page.goto(PAGE + '?state=task');
    const buttons = await page.evaluate(() => [...document.querySelectorAll('.decisions-rows [data-row-actions]')].map(b => getComputedStyle(b).visibility === 'visible' && b.tabIndex === 0 && !b.hasAttribute('aria-hidden')));
    check(buttons.length === 7 && buttons.every(Boolean), 'every row has an actions button, always visible and a Tab stop (' + buttons.length + ')');
    const menuRows = async () => page.evaluate(() => [...document.querySelectorAll('[role="menuitem"]')].map(m => m.querySelector('.menubar-words').textContent + (m.getAttribute('aria-disabled') === 'true' ? ' [off: ' + (m.querySelector('.menubar-detail') || {}).textContent + ']' : '')));
    await page.locator('[data-row="t107"] [data-row-actions]').click();
    await page.getByRole('menuitem').first().waitFor();
    let mr = await menuRows();
    check(mr.join('|') === 'Open|Open Chat [off: A task question comes from a service, not a chat.]|Open Group [off: A task question is not asked in a group.]|Open Task [off: Harbor Publisher did not give a page for this task.]|Dismiss Question…', 'a task row’s menu has every command, the ones that cannot apply disabled with their reason');
    await page.keyboard.press('Escape');
    await page.locator('a.md-item[data-key="r105"]').click({ button: 'right' });
    await page.getByRole('menuitem').first().waitFor();
    mr = await menuRows();
    check(mr.length === 5 && mr[1].startsWith('Open Chat [off: The chat that asked was retired.') && mr[4] === 'Dismiss Request…', 'right-click on a retired chat’s request: the same commands, Open Chat disabled with why');
    await page.keyboard.press('Escape');
    await page.locator('a.md-item[data-key="r101"]').focus(); await page.keyboard.press('Shift+F10');
    await page.getByRole('menuitem').first().waitFor();
    mr = await menuRows();
    check(mr.join('|') === 'Open|Open Chat|Open Group|Open Task [off: Only a question from a task has a task to open.]|Dismiss Request…', 'Shift+F10 on a group request opens the same menu');
    await page.keyboard.press('Escape');

    // Dismiss from a row's menu, for a request that is not the open one.
    const before = await rows();
    await page.locator('[data-row="m102"] [data-row-actions]').click();
    await page.getByRole('menuitem', { name: /Dismiss Message/ }).click();
    check((await page.locator('#ask .dialog-body').textContent()).includes('Dinah Design Review is told you dismissed it'), 'the dialog says the asking chat is told');
    await page.locator('#ask button[value="yes"]').click(); await page.waitForSelector('#ask', { state: 'detached' });
    check(await rows() === before - 1 && await page.locator('a.md-item[data-key="m102"]').count() === 0, 'dismissing from the menu takes that entry out of the list');

    // Dismiss a task question from its own surface.
    await page.locator('a.md-item[data-key="t106"]').click();
    await page.locator('[data-act="dismiss"]').click();
    check((await page.locator('#ask .dialog-body').textContent()).includes('Harbor Publisher is told you dismissed it, with no answer'), 'dismissing a task question says the service is told');
    await page.locator('#ask button[value="yes"]').click(); await page.waitForSelector('#ask', { state: 'detached' });
    check((await status()).includes('You dismissed the question from Harbor Publisher'), 'the task question leaves, and the status line says so');

    // About This Request is a pane on the right.
    const side = await page.evaluate(() => { const a = document.querySelector('.decisions-about'), q = document.querySelector('.decisions-split > .decisions-scroll'); return a && q && a.getBoundingClientRect().left >= q.getBoundingClientRect().right - 1; });
    check(side, 'About This Request is a pane to the right of the request');

    // The asking chat withdraws its request.
    await page.locator('a.md-item[data-key="r103"]').click();
    await tryIt('withdraw');
    check(await page.locator('a.md-item[data-key="r103"]').count() === 0 && (await status()).includes('Harbor Companion Lead withdrew its request'), 'a withdrawn request leaves the list and the status line says who withdrew it');
    await page.locator('.decisions-foot-status button.link').click();
    check((await page.locator('.decisions-main .notice').textContent()).includes('Harbor Companion Lead withdrew this request') && (await status()).includes('withdrew this at'), 'its record says the chat withdrew it');

    check(errors.length === 0, 'no page errors' + (errors.length ? ': ' + errors.join(' | ') : ''));
    await context.close();
  } finally { await browser.close(); await release(); }
  console.log(failures ? failures + ' FAILED' : 'all passed');
  process.exitCode = failures ? 1 : 0;
})().catch(e => { console.error(e); process.exitCode = 1; });
