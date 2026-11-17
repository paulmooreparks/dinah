// Dismiss Request on a notification's card. Dismissing a notification, Clear All and opening a request remove the
// notification only and never dismiss the request behind it. The card's own Dismiss Request (Dismiss Question on
// a task question's card) asks with the Decisions window's dialog and then dismisses the request, with the same
// saved operation and the same check of a lost result.
const { setup, makeRequest } = require('./lib/decisions-fixture');
const USE = ' Use this when you have dealt with it elsewhere, or it can no longer be answered.';
const UNCHECKED = 'The last send or dismissal has not been confirmed. Check its result first.';
const question = (key, minute) => ({ key, service: 'fixture.flow', serviceName: 'Workflow', workspace: 'ws-a', workspaceName: 'Harbor', task: 'task-a', generation: 'g1', taskTitle: 'Review the release', question: 'May check ' + key + ' proceed?', askedAt: '2026-11-15T10:0' + minute + ':00Z', target: { reason: 'This service has no task page.' }, options: [{ optionId: 'go', name: 'Run the check', kind: 'allow_once' }, { optionId: 'stop', name: 'Keep it paused', kind: 'reject_once' }] });
function tools(t) {
  const p = t.page;
  const card = id => p.locator('#questions [data-notice-id="' + id + '"]');
  const button = id => card(id).getByRole('button', { name: 'Dismiss Request…', exact: true });
  const line = id => card(id).locator('.request-dismiss-status');
  return {
    card, button, line,
    check: id => card(id).getByRole('button', { name: 'Check Dismiss Result', exact: true }),
    // A notification as the host keeps it. One with a request says which in requestId (mention_notices.go).
    notify: (id, message, chat, extra = {}) => p.evaluate(({ id, message, chat, extra }) => {
      const item = Object.assign({ id, kind: 'mention', requestId: '', severity: 'info', title: chat + ' sent you a message', body: 'Please decide.\nA second line.', group: '', groupTitle: '', sessionId: chat, sourceId: chat, sessionTitle: chat, message, created: '2026-11-14T00:00:00Z' }, extra);
      sessionStorage.setItem('attention', JSON.stringify(JSON.parse(sessionStorage.getItem('attention') || '[]').filter(n => n.id !== id).concat(item))); __emit('notifications:added', item);
    }, { id, message, chat, extra }),
    // The places and sizes that must not change with the state of a card: the card's place in the list and its size, and
    // within it each of its buttons and the lines under them. The list's own scrolling, which a click may cause, is left out.
    geometry: id => card(id).evaluate(el => { const base = el.getBoundingClientRect(); return JSON.stringify([[el.offsetTop, Math.round(base.width), Math.round(base.height), el.scrollTop]].concat(Array.from(el.querySelectorAll('.notice-actions button, .request-dismiss-status, .task-question-result, .task-question-dismiss-reason')).map(node => { const r = node.getBoundingClientRect(); return [Math.round(r.x - base.x), Math.round(r.y - base.y), Math.round(r.width), Math.round(r.height)]; }))); }),
    words: id => line(id).locator('span').innerText(),
    // Waits for the line under a card's buttons to begin with these words. A check of the whole line reads it with words().
    says: (id, text) => p.waitForFunction(({ id, text }) => { const el = document.querySelector('#questions [data-notice-id="' + id + '"] .request-dismiss-status span'); return !!el && el.textContent.startsWith(text); }, { id, text }),
    gone: id => card(id).waitFor({ state: 'detached' }),
    async ask(locator) { await locator.click(); await p.locator('#ask[open]').waitFor(); return { title: await p.locator('#ask-title').innerText(), body: await p.locator('#ask-body').innerText() }; },
    async answer(yes) { await p.locator(yes ? '#ask-yes' : '#ask-no').click(); await p.locator('#ask[open]').waitFor({ state: 'detached' }); },
    saved: id => p.evaluate(id => (dinahUIState.get('dinah-decisions:' + encodeURIComponent('request:' + id), null) || {}).operation || null, id)
  };
}
async function requests() {
  const partly = makeRequest('r4', { revision: 2 }); partly.questions[0].answered = true; partly.questions[0].answer = 'Use the reviewed release.';
  const t = await setup([makeRequest('r1'), makeRequest('r2'), makeRequest('r3'), partly, makeRequest('r5'), makeRequest('r6'), makeRequest('r7'), makeRequest('r8')], { entryPoints: true, noOpen: true });
  const { page: p, ok } = t, d = tools(t);
  try {
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    for (const n of [1, 2, 3, 4, 5, 6, 7, 8]) await d.notify('card-r' + n, n, 'Review r' + n, { requestId: 'r' + n });
    await d.notify('card-plain', 900, 'Plain Chat');
    await p.evaluate(() => dinahHost.toast('The export finished.', 'info'));
    await d.card('card-r8').waitFor(); await d.card('card-plain').waitFor();
    const report = p.locator('#questions .notification-report', { hasText: 'The export finished.' });
    await report.waitFor();

    // Which cards have the button.
    ok('a request\'s card has Dismiss Request beside its other buttons, in sight and enabled', await d.button('card-r1').isVisible() && await d.button('card-r1').isEnabled() && JSON.stringify(await d.card('card-r1').locator('.notice-actions button').allTextContents()) === '["Open Request in Decisions","Dismiss Request…","Dismiss"]' && await d.button('card-r1').getAttribute('title') === '' && await d.words('card-r1') === '', JSON.stringify(await d.card('card-r1').locator('.notice-actions button').allTextContents()));
    ok('Dismiss Request is shown without hovering', await d.button('card-r1').evaluate(el => { const s = getComputedStyle(el); return s.visibility === 'visible' && s.opacity === '1' && s.display !== 'none'; }));
    ok('a card with no request behind it has no such button', await d.button('card-plain').count() === 0 && await d.card('card-plain').locator('.request-dismiss-status').count() === 0 && await report.getByRole('button', { name: 'Dismiss Request…', exact: true }).count() === 0 && await d.card('card-plain').getByRole('button', { name: 'Dismiss', exact: true }).count() === 1);
    ok('the request\'s card shows all of itself in its fixed height', await d.card('card-r1').evaluate(el => el.scrollHeight <= el.clientHeight + 1), await d.card('card-r1').evaluate(el => el.scrollHeight + ' in ' + el.clientHeight));
    await t.shot('notifications-dismiss-request-card');

    // Dismissing a notification is not dismissing its request.
    await d.card('card-r7').getByRole('button', { name: 'Dismiss', exact: true }).click();
    await d.gone('card-r7');
    ok('dismissing a notification removes the notification and leaves its request open', (await t.calls('DismissRequest')).length === 0 && JSON.stringify((await t.calls('DismissNotification')).map(call => call[1])) === '["card-r7"]' && await p.evaluate(() => __decisionItems.some(item => item.id === 'r7') && __dismissals.length === 0));

    // Cancel.
    const still = await d.geometry('card-r1');
    await d.button('card-r1').focus();
    const asked = await d.ask(d.button('card-r1'));
    ok('the card asks with the Decisions window\'s dialog and wording', asked.title === 'Dismiss This Request?' && asked.body === 'No answer is sent, and it leaves Decisions. Review r1 is told you dismissed it, so it does not wait for you.' + USE && await p.evaluate(() => { const no = document.getElementById('ask-no'), yes = document.getElementById('ask-yes'); return document.activeElement === no && yes.className === 'btn btn-danger' && yes.textContent === 'Dismiss' && !document.querySelector('#ask .btn-primary'); }), JSON.stringify(asked));
    // The request's record has no commands, as the real host gives it, so whether it may be dismissed is read from its state.
    ok('the request and its state are read before the dialog, and nothing is sent until it is confirmed', JSON.stringify((await t.calls('DecisionByID')).map(call => call[1])) === '["r1"]' && JSON.stringify((await t.calls('RequestState')).map(call => call[1])) === '["r1"]' && (await t.calls('DismissRequest')).length === 0);
    await d.answer(false);
    const cancelled = [(await t.calls('DismissRequest')).length === 0, (await t.calls('DismissNotification')).length === 1, await d.card('card-r1').count() === 1, await d.button('card-r1').isEnabled(), await d.words('card-r1') === '', await d.saved('r1') === null, await d.geometry('card-r1') === still, await p.evaluate(() => __decisionItems.some(item => item.id === 'r1'))];
    ok('Cancel changes nothing', cancelled.every(Boolean), JSON.stringify(cancelled));
    ok('Cancel returns the focus to Dismiss Request', await d.button('card-r1').evaluate(el => el === document.activeElement));
    await d.ask(d.button('card-r1')); await p.keyboard.press('Escape'); await p.locator('#ask[open]').waitFor({ state: 'detached' });
    ok('Escape dismisses nothing', (await t.calls('DismissRequest')).length === 0 && await d.card('card-r1').count() === 1);

    // Confirming.
    await d.ask(d.button('card-r1')); await d.answer(true);
    await d.gone('card-r1');
    let calls = await t.calls('DismissRequest');
    ok('confirming calls the host once, with only the request\'s id, its revision and an operation identity', calls.length === 1 && calls[0].length === 2 && JSON.stringify(Object.keys(calls[0][1]).sort()) === '["id","operationId","revision"]' && calls[0][1].id === 'r1' && calls[0][1].revision === 1 && /^[0-9a-f]{32}$/.test(calls[0][1].operationId), JSON.stringify(calls));
    ok('the request is closed and its card is gone, and the other cards stay', await p.evaluate(() => !__decisionItems.some(item => item.id === 'r1') && __dismissals.length === 1 && __closedRecords.r1.decision.status === 'dismissed') && await d.card('card-r2').count() === 1 && await d.card('card-plain').count() === 1);
    const kept = await d.saved('r1');
    ok('the dismissal is the request\'s saved operation, as in Decisions', kept && kept.method === 'DismissRequest' && kept.status === 'committed' && kept.input.operationId === calls[0][1].operationId && kept.receipt.closed === true, JSON.stringify(kept));
    ok('the Decisions window was not opened', await p.locator('.decisions').count() === 0);

    // A refusal.
    const before = await d.geometry('card-r2');
    await p.evaluate(() => { window.__refuseDismiss = { code: 'revision_changed', text: 'This request changed. Refresh it before continuing.' }; });
    await d.ask(d.button('card-r2')); await d.answer(true);
    await d.says('card-r2', 'This request changed. Refresh it before continuing.');
    ok('a refusal keeps the card and says the host\'s reason, with a glyph as well as its colour', await d.card('card-r2').count() === 1 && await d.words('card-r2') === 'This request changed. Refresh it before continuing.' && await d.line('card-r2').evaluate(el => el.dataset.tone === 'danger' && getComputedStyle(el, '::before').maskImage !== 'none' && getComputedStyle(el, '::before').content !== 'none') && await d.button('card-r2').isEnabled() && await p.evaluate(() => __decisionItems.some(item => item.id === 'r2') && __dismissals.length === 1) && (await t.calls('DismissRequest')).length === 2);
    ok('the reason moves nothing on the card', await d.geometry('card-r2') === before, await d.geometry('card-r2') + ' ' + before);
    await t.shot('notifications-dismiss-request-refused');
    await p.evaluate(() => { window.__refuseDismiss = null; });

    // The host's state says Dismiss cannot be used now: nothing is asked or sent, and the card says the host's reason.
    await p.evaluate(() => { window.__requestOff = { r6: 'The host cannot dismiss this request now.' }; });
    await d.button('card-r6').click();
    await d.says('card-r6', 'The host cannot dismiss this request now.');
    ok('a request whose state does not allow Dismiss is not asked about or sent, and the card says why', await p.locator('#ask[open]').count() === 0 && (await t.calls('DismissRequest')).length === 2 && await p.evaluate(() => __decisionItems.some(item => item.id === 'r6')));
    await p.evaluate(() => { window.__requestOff = null; });
    // An answer that cannot be delivered: the host disables Answer with its reason, and the dialog says it.
    await p.evaluate(() => { __decisionItems.find(item => item.id === 'r6').dismissalReason = 'The asking chat or group is no longer available.'; });
    const undeliverable = await d.ask(d.button('card-r6'));
    ok('the dialog says when the asking chat cannot be told', undeliverable.body.includes('The asking chat or group is no longer available. If the chat itself is gone, nobody can be told.'), JSON.stringify(undeliverable));
    await d.answer(false);

    // A result that never arrives. The same operation is checked and, when the host has no result, sent again.
    await p.evaluate(() => { window.__dismissBeforeFailure = true; });
    await d.ask(d.button('card-r2')); await d.answer(true);
    await d.says('card-r2', 'Dismissing has not been confirmed. ');
    ok('an unconfirmed dismissal disables Dismiss Request with its reason and offers the check', await d.button('card-r2').isDisabled() && await d.button('card-r2').getAttribute('title') === UNCHECKED && await d.check('card-r2').isVisible() && await d.check('card-r2').isEnabled() && (await d.saved('r2')).status === 'submitted');
    ok('the check button and the words move nothing on the card', await d.geometry('card-r2') === before, await d.geometry('card-r2') + ' ' + before);
    await t.shot('notifications-dismiss-request-unconfirmed');
    await p.evaluate(() => { window.__dismissBeforeFailure = false; window.__lostDismissal = true; });
    await d.check('card-r2').click();
    await p.waitForFunction(() => __dismissals.length === 2);
    // The check has ended, with its result lost again, when the check button is offered again.
    await p.waitForFunction(() => !document.querySelector('#questions [data-notice-id="card-r2"] [data-request-check]').hidden);
    await d.says('card-r2', 'Dismissing has not been confirmed. ');
    calls = await t.calls('DismissRequest');
    ok('an operation the host has no result for is sent again unchanged', calls.length === 4 && JSON.stringify(calls[2][1]) === JSON.stringify(calls[3][1]) && calls[3][1].id === 'r2' && calls[1][1].operationId !== calls[2][1].operationId && (await t.calls('RequestOperation')).length === 1, JSON.stringify(calls));
    await p.evaluate(() => { window.__lostDismissal = false; });
    await d.check('card-r2').click();
    await d.gone('card-r2');
    const lookups = await t.calls('RequestOperation');
    ok('a lost result is recovered through the operation check without dismissing again', (await t.calls('DismissRequest')).length === 4 && lookups.length === 2 && lookups[1][1] === 'r2' && lookups[1][2] === calls[3][1].operationId && await p.evaluate(() => __dismissals.length === 2), JSON.stringify(lookups));

    // A request that ended another way before its card was used.
    await p.evaluate(() => { __closeRequest('r5', 'withdrawn', 'owner'); });
    await d.button('card-r5').click();
    await d.gone('card-r5');
    ok('a card whose request its chat withdrew goes without asking, and says how the request ended', await p.locator('#ask[open]').count() === 0 && (await t.calls('DismissRequest')).length === 4 && await p.locator('.toast', { hasText: 'Review r5 withdrew its request, so it no longer needs an answer.' }).count() >= 1);

    // The saved operation outlives the window. A partly answered request keeps its answers.
    await p.evaluate(() => { window.__dismissBeforeFailure = true; });
    const partlyAsked = await d.ask(d.button('card-r4'));
    ok('dismissing a partly answered request says its answers are kept', partlyAsked.body === 'No more answers are sent, and it leaves Decisions. The answers you already sent are kept. Review r4 is told you dismissed it, so it does not wait for you.' + USE, partlyAsked.body);
    await d.answer(true);
    await d.says('card-r4', 'Dismissing has not been confirmed. ');
    const lost = (await t.calls('DismissRequest')).pop()[1];
    ok('the dismissal is sent with the request\'s current revision', lost.id === 'r4' && lost.revision === 2 && await p.evaluate(() => __decisionItems.some(item => item.id === 'r4')));
    // After the reload the stubbed host starts again without its failure. The request is still open and its notification is still listed.
    await p.evaluate(() => dinahUIState.flush()); await p.reload();
    await p.waitForFunction(() => window.__dismissals && __dismissals.some(input => input.id === 'r4'));
    await p.waitForFunction(() => dinahHost.notices.list(entry => entry.data.notificationId === 'card-r4').length === 0);
    const after = await t.calls('RequestOperation'), again = await t.calls('DismissRequest');
    ok('a reload checks the saved dismissal and sends it again under its own operation, without asking again', after.length === 1 && after[0][1] === 'r4' && after[0][2] === lost.operationId && again.length === 1 && JSON.stringify(again[0][1]) === JSON.stringify(lost) && await p.locator('#ask[open]').count() === 0 && await p.evaluate(() => dinahHost.notices.list(entry => entry.data.notificationId === 'card-r6').length === 1), JSON.stringify([after, again, lost]));

    // A request the Decisions window has open is dismissed from its card, and the window follows.
    await p.evaluate(() => dinahDecisions.openRequest('r3'));
    await p.locator('.decisions-row[data-request-id="r3"] > a[aria-current="true"]').waitFor();
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await d.ask(d.button('card-r3')); await d.answer(true);
    await d.gone('card-r3');
    await p.locator('.decisions-row[data-request-id="r3"]').waitFor({ state: 'detached' });
    ok('the open Decisions window loses the request its card dismissed', await p.evaluate(() => __dismissals.length === 2 && __dismissals[1].id === 'r3') && (await t.calls('DismissRequest')).length === 2);

    // A request dismissed in the Decisions window loses its card, since no open request is behind it any more.
    await p.evaluate(() => dinahDecisions.openRequest('r8'));
    await p.locator('.decisions-row[data-request-id="r8"] > a[aria-current="true"]').waitFor();
    await p.locator('[data-dismiss-decision]').click(); await p.locator('#ask[open]').waitFor(); await d.answer(true);
    await p.waitForFunction(() => __dismissals.some(input => input.id === 'r8'));
    await p.waitForFunction(() => dinahHost.notices.list(entry => entry.data.notificationId === 'card-r8').length === 0);
    ok('a request dismissed in Decisions loses its card', (await t.calls('DismissRequest')).length === 3 && await p.evaluate(() => dinahHost.notices.list(entry => entry.data.notificationId === 'card-r6').length === 1));

    // Clear All removes notifications and dismisses no request.
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await d.card('card-r6').waitFor();
    await p.locator('#needs-clear').click(); await p.locator('#ask[open]').waitFor();
    await d.answer(true);
    await d.gone('card-r6');
    ok('Clear All removes the notifications and leaves their requests open', (await t.calls('DismissRequest')).length === 3 && (await t.calls('DismissNotification')).some(call => call[1] === 'card-r6') && (await t.calls('DismissNotification')).some(call => call[1] === 'card-plain') && await p.evaluate(() => __decisionItems.some(item => item.id === 'r6') && __decisionItems.some(item => item.id === 'r7') && __dismissals.length === 3 && !__dismissals.some(input => input.id === 'r6' || input.id === 'r7')));
  } finally { await t.close(); }
}
// Opening a request from its card removes the notification and leaves the request open.
async function opening() {
  const t = await setup([makeRequest('r1')], { entryPoints: true, noOpen: true }); const { page: p, ok } = t, d = tools(t);
  try {
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await d.notify('card-r1', 1, 'Review r1', { requestId: 'r1' });
    await d.card('card-r1').getByRole('button', { name: 'Open Request in Decisions', exact: true }).click();
    await p.locator('.decisions-row[data-request-id="r1"] > a[aria-current="true"]').waitFor();
    await p.waitForFunction(() => (window.__dismissed || []).includes('card-r1'));
    ok('opening a request from its card dismisses the notification and not the request', (await t.calls('DismissRequest')).length === 0 && await p.evaluate(() => __decisionItems.some(item => item.id === 'r1') && __dismissals.length === 0) && await p.locator('[data-dismiss-decision]').isEnabled());
  } finally { await t.close(); }
}
async function tasks() {
  const refusal = 'The task host is unavailable or is shutting down.';
  const t = await setup([], { entryPoints: true, noOpen: true, tasks: [question('t1', 1), question('t2', 2), question('t3', 3)], taskOff: { t2: refusal } }); const { page: p, ok } = t, d = tools(t);
  const card = key => p.locator('#questions [data-task-question-key="' + key + '"]');
  const button = key => card(key).getByRole('button', { name: 'Dismiss Question…', exact: true });
  const reason = key => card(key).locator('.task-question-dismiss-reason').innerText();
  const allowed = key => p.waitForFunction(key => dinahTaskQuestions.dismissal(key).enabled, key);
  try {
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await card('t3').waitFor(); await allowed('t1'); await allowed('t3');
    await p.waitForFunction(text => document.querySelector('#questions [data-task-question-key="t2"] .task-question-dismiss-reason').textContent === text, refusal);
    ok('a task question\'s card has Dismiss Question beside its other buttons', await button('t1').isVisible() && await button('t1').isEnabled() && JSON.stringify(await card('t1').locator('.notice-actions button').allTextContents()) === '["Dismiss Question…","Send Answer","Check Question Status"]' && await reason('t1') === '');
    ok('the card no longer says that it cannot dismiss the question', !(await p.locator('#questions').innerText()).includes('This card cannot dismiss the question yet') && await card('t1').getByRole('button', { name: 'Dismiss', exact: true }).count() === 0);
    ok('a question the host says cannot be dismissed shows the button disabled with the host\'s reason', await button('t2').isVisible() && await button('t2').isDisabled() && await button('t2').getAttribute('title') === refusal && await reason('t2') === refusal);
    const still = await d.geometry('task-question-t1');
    const asked = await d.ask(button('t1'));
    ok('the card asks with the Decisions window\'s dialog and wording for a question', asked.title === 'Dismiss This Question?' && asked.body === 'No answer is sent, and it leaves Decisions. Workflow is told you dismissed it, with no answer, and the task decides what happens next.' + USE && await p.evaluate(() => document.activeElement === document.getElementById('ask-no') && document.getElementById('ask-yes').className === 'btn btn-danger'), JSON.stringify(asked));
    await d.answer(false);
    ok('Cancel leaves the question waiting', (await t.calls('DismissRequest')).length === 0 && await card('t1').count() === 1 && await button('t1').isEnabled() && await d.geometry('task-question-t1') === still && await p.evaluate(() => !!dinahTaskQuestions.get('t1') && __taskPending.length === 3 && dinahUIState.get('task-question-dismissal:t1', null) === null));

    // A refusal.
    await p.evaluate(() => { window.__refuseTaskDismiss = { code: 'not_authorized', text: 'The task record is unavailable for settlement.' }; });
    const before = await d.geometry('task-question-t3');
    await d.ask(button('t3')); await d.answer(true);
    await p.waitForFunction(() => document.querySelector('#questions [data-task-question-key="t3"] [data-task-status]').textContent === 'The task record is unavailable for settlement.');
    ok('a refused dismissal keeps the question\'s card and says the host\'s reason, with a glyph as well as its colour', await card('t3').count() === 1 && await button('t3').isDisabled() && await reason('t3') === 'The task record is unavailable for settlement.' && await card('t3').locator('[data-task-status]').evaluate(el => el.dataset.tone === 'danger' && getComputedStyle(el, '::before').maskImage !== 'none') && await p.evaluate(() => __taskDismissals.length === 0 && !!dinahTaskQuestions.get('t3')) && await d.geometry('task-question-t3') === before, await d.geometry('task-question-t3') + ' ' + before);
    await p.evaluate(() => { window.__refuseTaskDismiss = null; });

    // Confirming.
    await d.ask(button('t1')); await d.answer(true);
    await card('t1').waitFor({ state: 'detached' });
    const calls = (await t.calls('DismissRequest')).filter(call => call[1].id === 't1');
    ok('confirming dismisses the question once, with only its key, its revision and an operation identity, and its card goes', calls.length === 1 && JSON.stringify(Object.keys(calls[0][1]).sort()) === '["id","operationId","revision"]' && calls[0][1].revision === 1 && await p.evaluate(() => __taskDismissals.length === 1 && __taskDismissals[0].id === 't1' && !dinahTaskQuestions.get('t1')) && await card('t2').count() === 1 && (await t.calls('Answer')).length === 0, JSON.stringify(calls));
  } finally { await t.close(); }
}
(async () => { await requests(); await opening(); await tasks(); })().catch(error => { console.error(error); process.exitCode = 1; });
