// Dismiss in the Decisions window: a chat's request and a task's question, from the foot and from the row menu,
// with the host's refusals, a lost result, and the record of how an entry ended.
const { setup, makeRequest, choose } = require('./lib/decisions-fixture');
const USE = ' Use this when you have dealt with it elsewhere, or it can no longer be answered.';
const question = (key, minute, extra = {}) => Object.assign({ key, service: 'fixture.flow', serviceName: 'Workflow', workspace: 'ws-a', workspaceName: 'Harbor', task: 'task-a', generation: 'g1', taskTitle: 'Review the release', question: 'May check ' + key + ' proceed?', askedAt: '2026-11-15T10:0' + minute + ':00Z', target: { reason: 'This service has no task page.' }, options: [{ optionId: 'go', name: 'Run the check', kind: 'allow_once' }, { optionId: 'stop', name: 'Keep it paused', kind: 'reject_once' }] }, extra);
function tools(t) {
  const p = t.page, button = p.locator('[data-dismiss-decision]'), send = p.locator('.decisions-foot .btn-primary');
  const box = locator => locator.evaluate(el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; });
  return {
    button, send,
    foot: () => p.locator('.decisions-foot-status').innerText(),
    reason: () => p.locator('.decisions-dismiss-reason').innerText(),
    // The places and sizes that must not change with the state of the open entry.
    geometry: async () => JSON.stringify([await box(button), await box(send), await box(p.locator('.decisions-dismiss-reason')), await box(p.locator('.decisions-foot-status')), await box(p.locator('.decisions-foot'))]),
    current: () => p.evaluate(() => { const a = document.querySelector('.decisions-row a[aria-current="true"]'); return a ? a.parentElement.dataset.requestId : ''; }),
    footHas: text => p.waitForFunction(text => { const line = document.querySelector('.decisions-foot-status'); return !!line && line.textContent.includes(text); }, text),
    async ask() { await button.click(); await p.locator('#ask[open]').waitFor(); return { title: await p.locator('#ask-title').innerText(), body: await p.locator('#ask-body').innerText() }; },
    async answer(yes) { await p.locator(yes ? '#ask-yes' : '#ask-no').click(); await p.locator('#ask[open]').waitFor({ state: 'detached' }); },
    async rowMenu(id) { await p.locator('.decisions-row[data-request-id="' + id + '"] [data-decision-actions]').click(); await p.getByRole('menuitem', { name: /^Dismiss / }).waitFor(); return p.getByRole('menuitem', { name: /^Dismiss / }); }
  };
}
async function requests() {
  const partly = makeRequest('r4', { revision: 2 }); partly.questions[0].answered = true; partly.questions[0].answer = 'Use the reviewed release.';
  const t = await setup([makeRequest('r1'), makeRequest('r2'), makeRequest('r3'), partly, makeRequest('r5', { dismissalReason: 'The asking chat or group is no longer available.' })]);
  const { page: p, ok } = t, d = tools(t);
  try {
    await choose(t, 'r1');
    await p.getByLabel('Answer 1', { exact: true }).fill('Keep this draft.');
    await p.waitForFunction(() => !document.querySelector('.decisions-foot .btn-primary').disabled);
    ok('an ordinary open request can be answered and can be dismissed', await d.button.isEnabled() && await d.button.getAttribute('title') === '' && await d.reason() === '' && !await p.getByLabel('Answer 1', { exact: true }).isDisabled());
    const still = await d.geometry();
    await d.button.focus();
    const asked = await d.ask();
    ok('the confirmation says no answer is sent and that the asking chat is told', asked.title === 'Dismiss This Request?' && asked.body === 'No answer is sent, and it leaves Decisions. Review r1 is told you dismissed it, so it does not wait for you.' + USE, JSON.stringify(asked));
    ok('Cancel comes first and has the focus, and the committing button is the danger button with no primary button', await p.evaluate(() => { const no = document.getElementById('ask-no'), yes = document.getElementById('ask-yes'); return document.activeElement === no && !!(no.compareDocumentPosition(yes) & Node.DOCUMENT_POSITION_FOLLOWING) && yes.className === 'btn btn-danger' && yes.textContent === 'Dismiss' && !document.querySelector('#ask .btn-primary'); }));
    await t.shot('decisions-dismiss-dialog');
    await d.answer(false);
    ok('Cancel changes nothing', (await t.calls('DismissRequest')).length === 0 && await d.current() === 'r1' && await p.locator('.decisions-row').count() === 5 && await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'Keep this draft.' && await d.geometry() === still && await p.evaluate(() => !dinahUIState.get('dinah-decisions:request%3Ar1').operation));
    ok('Cancel returns the focus to Dismiss', await d.button.evaluate(el => el === document.activeElement));
    await d.ask(); await p.keyboard.press('Escape'); await p.locator('#ask[open]').waitFor({ state: 'detached' });
    ok('Escape dismisses nothing', (await t.calls('DismissRequest')).length === 0 && await d.current() === 'r1');
    await d.ask(); await d.answer(true);
    await p.waitForFunction(() => __dismissals.length === 1);
    await p.locator('.decisions-row[data-request-id="r2"] > a[aria-current="true"]').waitFor();
    let calls = await t.calls('DismissRequest');
    ok('the host is called once, with only the request\'s id, its revision and an operation identity', calls.length === 1 && calls[0].length === 2 && JSON.stringify(Object.keys(calls[0][1]).sort()) === '["id","operationId","revision"]' && calls[0][1].id === 'r1' && calls[0][1].revision === 1 && /^[0-9a-f]{32}$/.test(calls[0][1].operationId), JSON.stringify(calls));
    ok('the dismissed request leaves the list and the next one opens', await p.locator('.decisions-row[data-request-id="r1"]').count() === 0 && await p.locator('.decisions-row').count() === 4 && await p.locator('.md-sidebar-head .badge').innerText() === '4' && await p.getByLabel('Answer 1', { exact: true }).isVisible());
    ok('the status line says the request was dismissed and no answer was sent', (await d.foot()).startsWith('You dismissed the request from Review r1. No answer was sent.') && await p.locator('.decisions-foot-status').getByRole('button', { name: 'Show the Request', exact: true }).count() === 1 && (await t.calls('AnswerDecision')).length === 0, await d.foot());
    ok('the foot keeps its places and sizes after a dismissal', await d.geometry() === still);
    await t.shot('decisions-dismiss-done');
    await p.locator('.decisions-foot-status').getByRole('button', { name: 'Show the Request', exact: true }).click();
    await p.waitForFunction(() => { const record = document.querySelector('.decisions-record'); return !!record && record.textContent.includes('You dismissed this request'); });
    const dismissedRecord = await p.locator('.decisions-record .decisions-delivery').innerText();
    ok('a dismissed request\'s record says that it was dismissed and when', /^You dismissed this request on \S.*\. No answer was sent\. This view is read-only\.$/.test(dismissedRecord) && await p.locator('.decisions-record').getByText('This question was dismissed without an answer.', { exact: true }).count() === 2 && await p.locator('.decisions-record input,.decisions-record textarea').count() === 0, dismissedRecord);
    ok('the record\'s status line keeps its height', await p.locator('.decisions-record .decisions-delivery').evaluate(el => el.scrollHeight <= el.clientHeight + 1));
    await t.shot('decisions-dismiss-record');
    await p.getByRole('button', { name: 'Back to Requests', exact: true }).click();

    // A refusal.
    await choose(t, 'r2');
    await p.evaluate(() => { window.__refuseDismiss = { code: 'revision_changed', text: 'This request changed. Refresh it before continuing.' }; });
    await d.ask(); await d.answer(true);
    await d.footHas('This request changed. Refresh it before continuing.');
    ok('a refusal is shown with the host\'s reason, as a warning, and the request stays', await d.button.isEnabled() && await p.locator('.decisions-foot-status').evaluate(el => el.classList.contains('danger') && getComputedStyle(el, '::before').maskImage !== 'none') && await d.current() === 'r2' && await p.locator('.decisions-row[data-request-id="r2"]').count() === 1 && await p.evaluate(() => __dismissals.length === 1) && !await p.getByLabel('Answer 1', { exact: true }).isDisabled() && await d.geometry() === still, await d.foot());
    await p.evaluate(() => { window.__refuseDismiss = null; });

    // A result that never arrives. The same operation is checked and, when the host has no result, sent again.
    await p.evaluate(() => { window.__dismissBeforeFailure = true; });
    await d.ask(); await d.answer(true);
    await d.footHas('Dismissing has not been confirmed.');
    await p.waitForFunction(() => { const check = document.querySelector('[data-operation]'); return !check.hidden && !check.disabled; });
    ok('an unconfirmed dismissal disables Dismiss and Send and says why', await d.button.isDisabled() && await d.reason() === 'The last send or dismissal has not been confirmed. Check its result first.' && await d.button.getAttribute('title') === await d.reason() && await d.send.isDisabled() && await p.getByRole('button', { name: 'Check Dismiss Result', exact: true }).isVisible());
    ok('a reason and the check button move nothing in the foot', await d.geometry() === still, await d.geometry() + ' ' + still);
    await t.shot('decisions-dismiss-unconfirmed');
    await p.evaluate(() => { window.__dismissBeforeFailure = false; window.__lostDismissal = true; });
    await p.getByRole('button', { name: 'Check Dismiss Result', exact: true }).click();
    await p.waitForFunction(() => __dismissals.length === 2);
    await d.footHas('Dismissing has not been confirmed.');
    await p.waitForFunction(() => { const check = document.querySelector('[data-operation]'); return !check.hidden && !check.disabled; });
    calls = await t.calls('DismissRequest');
    ok('an operation the host has no result for is sent again unchanged', calls.length === 4 && JSON.stringify(calls[2][1]) === JSON.stringify(calls[3][1]) && calls[3][1].id === 'r2' && calls[1][1].operationId !== calls[2][1].operationId, JSON.stringify(calls));
    await p.evaluate(() => { window.__lostDismissal = false; });
    await p.getByRole('button', { name: 'Check Dismiss Result', exact: true }).click();
    await p.locator('.decisions-row[data-request-id="r3"] > a[aria-current="true"]').waitFor();
    const lookups = await t.calls('RequestOperation');
    ok('a lost result is recovered through the operation check without dismissing again', (await t.calls('DismissRequest')).length === 4 && lookups.length === 2 && lookups[1][1] === 'r2' && lookups[1][2] === calls[3][1].operationId && await p.locator('.decisions-row[data-request-id="r2"]').count() === 0 && (await d.foot()).startsWith('You dismissed the request from Review r2. No answer was sent.'), JSON.stringify(lookups));

    // The saved operation outlives the window.
    await p.evaluate(() => { window.__lostDismissal = true; });
    await d.ask(); await d.answer(true);
    await p.waitForFunction(() => __dismissals.length === 3);
    await d.footHas('Dismissing has not been confirmed.');
    await p.evaluate(() => dinahUIState.flush()); await p.reload(); await p.locator('.decisions').waitFor();
    await p.locator('.decisions-row[data-request-id="r4"] > a[aria-current="true"]').waitFor();
    await d.footHas('You dismissed the request from Review r3. No answer was sent.');
    ok('a reload checks the saved dismissal and does not send it again', (await t.calls('DismissRequest')).length === 0 && (await t.calls('RequestOperation')).length === 1 && (await t.calls('RequestOperation'))[0][1] === 'r3');

    // The row menu, and a request whose answer cannot be delivered.
    const command = await d.rowMenu('r5');
    ok('every row offers the same commands, and none to withdraw', JSON.stringify(await p.locator('.context-menu .menubar-words').allTextContents()) === '["Open","Open Chat","Open Group","Open Task","Dismiss Request…"]' && await command.getAttribute('aria-disabled') !== 'true' && await command.evaluate(el => el.classList.contains('danger')), JSON.stringify(await p.locator('.context-menu [role=menuitem]').allTextContents()));
    await command.click(); await p.locator('#ask[open]').waitFor();
    ok('a request whose asking chat is unavailable says nobody can be told if the chat is gone', await p.locator('#ask-body').innerText() === 'No answer is sent, and it leaves Decisions. The asking chat or group is no longer available. If the chat itself is gone, nobody can be told.' + USE && await p.evaluate(() => document.activeElement === document.getElementById('ask-no')), await p.locator('#ask-body').innerText());
    await d.answer(false);
    ok('Cancel from a row\'s menu leaves the open request open', await d.current() === 'r4' && (await t.calls('DismissRequest')).length === 0);
    await choose(t, 'r5');
    ok('a request that cannot be delivered blocks answers, says why, and can still be dismissed', await p.getByLabel('Answer 1', { exact: true }).isDisabled() && await d.send.isDisabled() && await d.button.isEnabled() && await d.foot() === 'An answer cannot be delivered. Dismissing closes the request.' && await p.locator('.decisions-detail .notice').innerText() === 'The asking chat or group is no longer available.' && await d.geometry() === still, await d.foot());
    await choose(t, 'r4');
    await (await d.rowMenu('r5')).click(); await p.locator('#ask[open]').waitFor(); await d.answer(true);
    await p.waitForFunction(() => __dismissals.some(input => input.id === 'r5'));
    await p.locator('.decisions-row[data-request-id="r5"]').waitFor({ state: 'detached' });
    await d.footHas('You dismissed the request from Review r5. No answer was sent.');
    ok('a request dismissed from its row leaves the list and the remaining request is open', await d.current() === 'r4' && await p.locator('.decisions-row').count() === 1);

    // A partly answered request keeps its answers.
    const partlyAsked = await d.ask();
    ok('dismissing a partly answered request says its answers are kept', partlyAsked.body === 'No more answers are sent, and it leaves Decisions. The answers you already sent are kept. Review r4 is told you dismissed it, so it does not wait for you.' + USE, partlyAsked.body);
    await d.answer(true);
    await p.locator('.decisions-empty').waitFor({ state: 'visible' });
    const done = p.locator('.decisions-done');
    ok('the last dismissal is confirmed under the empty state, with a glyph', (await done.innerText()).startsWith('You dismissed the request from Review r4. The answers already sent are kept.') && await done.locator('.glyph[data-glyph="check"]').count() === 1 && (await t.calls('DismissRequest')).pop()[1].revision === 2, await done.innerText());
    await t.shot('decisions-dismiss-last');
    await done.getByRole('button', { name: 'Show the Request', exact: true }).click();
    await p.waitForFunction(() => { const record = document.querySelector('.decisions-record'); return !!record && record.textContent.includes('You dismissed this request'); });
    ok('the record of a partly answered request keeps the answer that was sent', /^You dismissed this request on .*\. The answers sent before then are kept\. This view is read-only\.$/.test(await p.locator('.decisions-record .decisions-delivery').innerText()) && (await p.locator('.decisions-record').innerText()).includes('Answer: Use the reviewed release.') && await p.locator('.decisions-record').getByText('This question was dismissed without an answer.', { exact: true }).count() === 1, await p.locator('.decisions-record').innerText());
  } finally { await t.close(); }
}
async function elsewhere() {
  const t = await setup([makeRequest('r1'), makeRequest('r2')]); const { page: p, ok } = t, d = tools(t);
  try {
    await choose(t, 'r1');
    await p.getByLabel('Answer 1', { exact: true }).fill('Use the reviewed release.');
    await p.waitForFunction(() => !document.querySelector('.decisions-foot .btn-primary').disabled);
    await d.send.click();
    await p.waitForFunction(() => __delivered.length === 1);
    await p.waitForFunction(() => document.querySelectorAll('.decisions-question textarea').length === 1);
    ok('answering still works for an open request the host says can be dismissed', await p.evaluate(() => __decisionItems[0].canDismiss === true) && (await t.calls('AnswerDecision'))[0][1].id === 'r1' && (await t.calls('DismissRequest')).length === 0 && await d.button.isEnabled());
    // The asking chat withdraws the open request.
    await p.evaluate(() => { __closeRequest('r1', 'withdrawn', 'owner'); __emit('bus:changed', {}); });
    await d.footHas('Review r1 withdrew its request, so it no longer needs an answer. Anything you had typed for it is gone.');
    ok('a withdrawn request leaves the list, the next one opens, and the line says which chat withdrew it', await d.current() === 'r2' && await p.locator('.decisions-row').count() === 1);
    await p.locator('.decisions-foot-status').getByRole('button', { name: 'Show What Was Withdrawn', exact: true }).click();
    await p.waitForFunction(() => { const record = document.querySelector('.decisions-record'); return !!record && record.textContent.includes('withdrew this request'); });
    const record = p.locator('.decisions-record');
    ok('a withdrawn request\'s record says which chat withdrew it and when', /^Review r1 withdrew this request on \S.*\. This view is read-only\.$/.test(await record.locator('.decisions-delivery').innerText()) && /^Review r1 withdrew this request on \S.*[^.]$/.test(await record.locator('.notice-title').innerText()) && await record.locator('.notice-body').innerText() === 'It no longer needs an answer from you.' && (await record.innerText()).includes('Answer: Use the reviewed release.') && await record.getByText('This question was withdrawn without an answer.', { exact: true }).count() === 1, await record.innerText());
    await t.shot('decisions-dismiss-withdrawn-record');
    await p.getByRole('button', { name: 'Back to Requests', exact: true }).click();
    // The person dismisses the open request somewhere else.
    await choose(t, 'r2');
    await p.evaluate(() => { __closeRequest('r2', 'dismissed', 'person'); __emit('bus:changed', {}); });
    await p.waitForFunction(() => { const line = document.querySelector('.decisions-done'); return !!line && line.textContent.includes('You dismissed the request from Review r2. No answer was sent.'); });
    ok('a request dismissed elsewhere is said as dismissed', await p.locator('.decisions-done').getByRole('button', { name: 'Show the Request', exact: true }).count() === 1);
  } finally { await t.close(); }
}
async function tasks() {
  const refusal = 'The task host is unavailable or is shutting down.';
  const t = await setup([makeRequest('r1')], { tasks: [1, 2, 3, 4, 5, 6, 7, 8].map(n => question('t' + n, n)).concat(question('t9', 9, { serviceName: 'Publisher' })), taskOff: { t2: refusal } }); const { page: p, ok } = t, d = tools(t);
  const allowed = () => p.waitForFunction(() => !document.querySelector('[data-dismiss-decision]').disabled);
  const check = () => p.locator('.decisions-detail').getByRole('button', { name: 'Check Question Status', exact: true }).click();
  try {
    await p.locator('.decisions-row[data-request-id="task:t9"]').waitFor();
    await choose(t, 'r1');
    const still = await d.geometry();
    // The host says a question cannot be dismissed.
    await choose(t, 'task:t2');
    await p.waitForFunction(reason => document.querySelector('.decisions-dismiss-reason').textContent === reason, refusal);
    ok('a question the host says cannot be dismissed shows Dismiss disabled with the host\'s reason', await d.button.isVisible() && await d.button.isDisabled() && await d.button.getAttribute('title') === refusal && await d.geometry() === still, await d.geometry() + ' ' + still);
    const off = await d.rowMenu('task:t2');
    ok('its row offers Dismiss Question disabled with the same reason', await off.getAttribute('aria-disabled') === 'true' && (await off.innerText()).startsWith('Dismiss Question…') && (await off.innerText()).includes(refusal), await off.innerText());
    await t.shot('decisions-dismiss-question-menu');
    await p.keyboard.press('Escape');
    await p.evaluate(() => { __taskOff = {}; });
    await check(); await allowed();
    ok('checking the question again reads the host\'s command again', await d.reason() === '' && (await t.calls('DismissRequest')).length === 0);

    // Dismissing a task question.
    await choose(t, 'task:t1'); await allowed();
    ok('a task question has the same foot as a request', await d.geometry() === still);
    const asked = await d.ask();
    ok('the confirmation says the service is told and the task decides what happens next', asked.title === 'Dismiss This Question?' && asked.body === 'No answer is sent, and it leaves Decisions. Workflow is told you dismissed it, with no answer, and the task decides what happens next.' + USE && await p.evaluate(() => document.activeElement === document.getElementById('ask-no') && document.getElementById('ask-yes').className === 'btn btn-danger'), JSON.stringify(asked));
    await t.shot('decisions-dismiss-question');
    await d.answer(false);
    ok('Cancel leaves the question waiting', (await t.calls('DismissRequest')).length === 0 && await d.current() === 'task:t1' && await p.evaluate(() => !!dinahTaskQuestions.get('t1') && __taskPending.length === 9));
    const reads = (await t.calls('RequestState')).filter(call => call[1] === 't1').length;
    await d.ask(); await d.answer(true);
    await p.waitForFunction(() => __taskDismissals.length === 1);
    await p.locator('.decisions-row[data-request-id="task:t1"]').waitFor({ state: 'detached' });
    await d.footHas('You dismissed the question from Workflow. No answer was sent.');
    const sequence = (await t.calls()).filter(call => (call[0] === 'RequestState' || call[0] === 'DismissRequest') && (call[1] === 't1' || call[1].id === 't1')).map(call => call[0]);
    let calls = await t.calls('DismissRequest');
    ok('the question\'s revision is read and it is dismissed once, with only its key, revision and an operation identity', calls.length === 1 && JSON.stringify(Object.keys(calls[0][1]).sort()) === '["id","operationId","revision"]' && calls[0][1].id === 't1' && calls[0][1].revision === 1 && sequence.length === reads + 2 && sequence.slice(-2).join() === 'RequestState,DismissRequest', JSON.stringify([calls, sequence, reads]));
    ok('the dismissed question leaves the list, the next entry opens, and nothing is left saved', await d.current() === 'r1' && await p.evaluate(() => !dinahTaskQuestions.get('t1') && dinahUIState.get('task-question-dismissal:t1', null) === null) && await p.locator('.decisions-foot-status button').count() === 0 && await d.geometry() === still && (await t.calls('Answer')).length === 0);

    // From a row's menu, while another entry is open.
    await p.waitForFunction(() => dinahTaskQuestions.dismissal('t9').enabled);
    await (await d.rowMenu('task:t9')).click(); await p.locator('#ask[open]').waitFor(); await d.answer(false);
    ok('Cancel from the row of a question leaves the open entry open', await d.current() === 'r1' && (await t.calls('DismissRequest')).length === 1);
    await (await d.rowMenu('task:t9')).click(); await p.locator('#ask[open]').waitFor(); await d.answer(true);
    await p.locator('.decisions-row[data-request-id="task:t9"]').waitFor({ state: 'detached' });
    await d.footHas('You dismissed the question from Publisher. No answer was sent.');
    ok('a question dismissed from its row leaves the list and its result is said', await p.evaluate(() => __taskDismissals.length === 2 && __taskDismissals[1].id === 't9') && await d.current() === 'r1');

    // A refusal.
    await choose(t, 'task:t3'); await allowed();
    await p.evaluate(() => { window.__refuseTaskDismiss = { code: 'not_authorized', text: 'The task record is unavailable for settlement.' }; });
    await d.ask(); await d.answer(true);
    await p.waitForFunction(() => document.querySelector('.decisions-dismiss-reason').textContent === 'The task record is unavailable for settlement.');
    ok('a refused dismissal leaves the question waiting, with Dismiss disabled and the host\'s reason', await d.button.isDisabled() && await d.foot() === 'The task record is unavailable for settlement.' && await p.locator('.decisions-foot-status').evaluate(el => el.classList.contains('danger')) && await d.current() === 'task:t3' && await p.evaluate(() => __taskDismissals.length === 2 && dinahUIState.get('task-question-dismissal:t3', null) === null) && await d.geometry() === still);
    await p.locator('.decisions-detail').getByRole('radio', { name: 'Run the check', exact: true }).check();
    ok('a refused dismissal leaves the question answerable', await d.send.isEnabled());
    await p.evaluate(() => { window.__refuseTaskDismiss = null; });
    await check(); await allowed();

    // A result lost after the host committed, with the event lost as well.
    await choose(t, 'task:t4'); await allowed();
    await p.evaluate(() => { window.__lostTaskDismissal = true; window.__silentTaskEnd = true; });
    await d.ask(); await d.answer(true);
    await d.footHas('Dismissing has not been confirmed.');
    const saved = await p.evaluate(() => dinahUIState.get('task-question-dismissal:t4', null));
    ok('an unconfirmed dismissal keeps its operation and disables Dismiss and Send', saved && saved.id === 't4' && saved.revision === 1 && await d.button.isDisabled() && await d.reason() === 'Dismissing has not been confirmed. Check the question status.' && await d.send.isDisabled() && await d.geometry() === still, JSON.stringify(saved));
    await p.evaluate(() => { window.__lostTaskDismissal = false; });
    await check();
    await d.footHas('You dismissed the question from Workflow. No answer was sent.');
    let lookups = await t.calls('RequestOperation');
    ok('the lost result is recovered through the operation check without dismissing again', lookups.length === 1 && lookups[0][1] === 't4' && lookups[0][2] === saved.operationId && (await t.calls('DismissRequest')).filter(call => call[1].id === 't4').length === 1 && await p.evaluate(() => !dinahTaskQuestions.get('t4') && dinahUIState.get('task-question-dismissal:t4', null) === null));

    // A dismissal the host never received is sent again under the same operation.
    await choose(t, 'task:t5'); await allowed();
    await p.evaluate(() => { window.__taskBeforeFailure = true; });
    await d.ask(); await d.answer(true);
    await d.footHas('Dismissing has not been confirmed.');
    await p.evaluate(() => { window.__taskBeforeFailure = false; });
    await check();
    await p.locator('.decisions-row[data-request-id="task:t5"]').waitFor({ state: 'detached' });
    calls = (await t.calls('DismissRequest')).filter(call => call[1].id === 't5');
    ok('an operation the host has no result for is sent again unchanged', calls.length === 2 && JSON.stringify(calls[0][1]) === JSON.stringify(calls[1][1]) && await p.evaluate(() => __taskDismissals.filter(input => input.id === 't5').length === 1));
    await p.evaluate(() => { window.__silentTaskEnd = false; });

    // How a question ended, from the host's event and from its record.
    await choose(t, 'task:t6');
    await p.evaluate(() => __endTask('t6', 'withdrawn', true));
    await d.footHas('Workflow withdrew its question, so it no longer needs an answer. Anything you had typed for it is gone.');
    ok('a withdrawn question is said as withdrawn', await p.locator('.decisions-row[data-request-id="task:t6"]').count() === 0);
    await choose(t, 'task:t7');
    await p.evaluate(() => { __endTask('t7', 'cancelled', false); return dinahTaskQuestions.refresh(); });
    await d.footHas('The task Review the release was cancelled, so its question closed. No answer was sent.');
    ok('a question that ended without its event is described from the host\'s record', (await t.calls('TaskQuestionStatus')).some(call => call[1] === 't7'));
    await choose(t, 'task:t2');
    await p.locator('.decisions-detail').getByRole('radio', { name: 'Run the check', exact: true }).check();
    await d.send.click();
    await d.footHas('Your answer was sent to Workflow, which continues the task Review the release.');
    ok('an answered question is said as answered', JSON.stringify(await p.evaluate(() => __taskAnswers)) === '[["t2","go"]]');

    // The saved operation outlives the window. The stubbed host starts again with every question waiting.
    await choose(t, 'task:t8'); await allowed();
    await p.evaluate(() => { window.__lostTaskDismissal = true; window.__silentTaskEnd = true; });
    await d.ask(); await d.answer(true);
    await d.footHas('Dismissing has not been confirmed.');
    const kept = await p.evaluate(() => dinahUIState.get('task-question-dismissal:t8', null));
    await p.evaluate(() => dinahUIState.flush()); await p.reload(); await p.locator('.decisions').waitFor();
    await p.waitForFunction(() => window.__taskDismissals.some(input => input.id === 't8'));
    await p.waitForFunction(() => !dinahTaskQuestions.get('t8'));
    lookups = await t.calls('RequestOperation'); calls = await t.calls('DismissRequest');
    ok('a reload checks the saved dismissal under its own operation', lookups.length === 1 && lookups[0][1] === 't8' && lookups[0][2] === kept.operationId && calls.length === 1 && JSON.stringify(calls[0][1]) === JSON.stringify(kept), JSON.stringify([lookups, calls, kept]));
  } finally { await t.close(); }
}
// The foot at a phone's width, where its buttons wrap.
async function narrow() {
  const t = await setup([makeRequest('r1'), makeRequest('r2')]); const { page: p, ok } = t, d = tools(t);
  try {
    await p.setViewportSize({ width: 390, height: 1000 });
    await choose(t, 'r1');
    const still = await d.geometry();
    await p.evaluate(() => { window.__dismissBeforeFailure = true; });
    await d.ask();
    ok('the confirmation fits a narrow window', await p.locator('#ask').evaluate(el => { const r = el.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth && el.scrollWidth <= el.clientWidth + 1; }));
    await d.answer(true);
    await d.footHas('Dismissing has not been confirmed.');
    await p.waitForFunction(() => { const check = document.querySelector('[data-operation]'); return !check.hidden && !check.disabled; });
    ok('a reason and the check button move nothing in the narrow foot', await d.geometry() === still && await d.reason() !== '' && await p.locator('.decisions-foot').evaluate(el => Array.from(el.querySelectorAll('button:not([hidden]),small')).every(node => { const a = node.getBoundingClientRect(), b = el.getBoundingClientRect(); return a.left >= b.left && a.right <= b.right + 1 && a.top >= b.top && a.bottom <= b.bottom + 1; })) && await p.locator('.decisions-dismiss-reason').evaluate(el => el.scrollHeight <= el.clientHeight + 1), await d.geometry() + ' ' + still);
    await t.shot('decisions-dismiss-narrow');
  } finally { await t.close(); }
}
(async () => { await requests(); await elsewhere(); await tasks(); await narrow(); })().catch(error => { console.error(error); process.exitCode = 1; });
