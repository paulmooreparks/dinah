// Dismissing a request, end to end: real hidden desktop, real bus MCP helper, and an isolated ACP stand-in.
// Usage: node decisions-dismiss-hidden.cjs <desktop.exe> <agent.exe> <fresh-output-folder>
//
// Eight chats each really ask the person two questions through the bus helper. The person dismisses requests from
// the foot of the Decisions window, from a row's menu and from a notification's card, and the asking agent is told.
// Dismissing a notification, opening a request from its card and Clear All remove the notice only. An ordinary
// request is still answered. A dismissal whose answer is lost is looked up after a reload and is not sent again.
// The program is then started again on the same home, and what was dismissed stays dismissed.
//
// The programs are built as decisions-hidden.cjs says. Each part below stands by itself with its own chat, so a
// part that fails does not keep the later parts from running. A part that cannot finish is reported as a failure.
'use strict';
const { isDeepStrictEqual: same } = require('node:util');
const { randomBytes } = require('node:crypto');
const { hiddenDesktop } = require('./lib/hidden-desktop.cjs');
const [desktop, agent, out] = process.argv.slice(2);

// What the program tells the asking chat (internal/bus/request_settlement.go), and the end of every confirmation.
const WORDING = 'The person dismissed this request without answering.';
const USE = ' Use this when you have dealt with it elsewhere, or it can no longer be answered.';
const UNCHECKED = 'The last send or dismissal has not been confirmed. Check its result first.';

(async () => {
  const t = await hiddenDesktop({ desktop, agent, out });
  const { check, until, call, records, prompts, keep } = t;
  for (const method of ['DismissRequest', 'RequestOperation', 'DismissDecision', 'DecisionByID', 'AnswerDecision', 'DismissNotification', 'ClearNotifications']) t.watch.add(method);
  let failure = null;

  const decisions = async () => (await call('Decisions', 0)).items;
  const listed = async id => (await decisions()).some(x => x.id === id);
  const idle = label => until(async () => (await call('UpdateState')).working === 0, label);
  // The prompts in which a chat's agent was told that its request was dismissed.
  const told = r => prompts(r.session).filter(x => x.includes(WORDING));
  const row = id => t.page.locator('.decisions-row[data-request-id="' + id + '"]');
  const card = r => t.page.locator('#questions [data-notice-id="' + r.notice + '"]');
  const noticeKept = async r => (await call('Notifications')).some(x => x.id === r.notice || x.requestId === r.id);
  const savedOperation = id => t.page.evaluate(id => window.dinahDecisionDrafts.read('request:' + id).operation || null, id);
  const currentRow = () => t.page.evaluate(() => { const a = document.querySelector('.decisions-row a[aria-current="true"]'); return a ? a.parentElement.dataset.requestId : ''; });
  const openDecisions = async () => { await t.page.evaluate(() => window.dinahWorkbench.open('decisions')); await t.page.locator('.decisions').waitFor(); };
  const openNotifications = () => t.page.evaluate(() => window.dinahWorkbench.open('needs-you'));
  const choose = async id => { await row(id).locator('> a').click(); await row(id).locator('> a[aria-current="true"]').waitFor(); await t.page.locator('.decisions-detail h2').waitFor(); };
  // Waits for the line the Decisions window says of what was last done, in the foot or under the empty list.
  const footSays = text => t.page.waitForFunction(text => Array.from(document.querySelectorAll('.decisions-foot-status, .decisions-done')).some(el => el.textContent.includes(text)), text);
  const sendEnabled = () => t.page.waitForFunction(() => !document.querySelector('.decisions-foot .btn-primary').disabled);
  const dialog = {
    async read() {
      const p = t.page; await p.locator('#ask[open]').waitFor();
      return { title: await p.locator('#ask-title').innerText(), body: await p.locator('#ask-body').innerText(), commit: await p.locator('#ask-yes').innerText(),
        danger: await p.locator('#ask-yes').evaluate(el => el.className === 'btn btn-danger'), cancelFocused: await p.evaluate(() => document.activeElement === document.getElementById('ask-no')) };
    },
    async answer(yes) { await t.page.locator(yes ? '#ask-yes' : '#ask-no').click(); await t.page.locator('#ask[open]').waitFor({ state: 'detached' }); }
  };
  const confirmation = r => 'No answer is sent, and it leaves Decisions. ' + r.title + ' is told you dismissed it, so it does not wait for you.' + USE;
  const asksAsApproved = (asked, r) => asked.title === 'Dismiss This Request?' && asked.body === confirmation(r) && asked.commit === 'Dismiss' && asked.danger && asked.cancelFocused;
  const stateOf = async id => { const s = await call('RequestState', id); return { status: s.status, revision: s.revision, settledAt: s.settledAt || '', actorKind: s.actorKind || '', wording: s.wording || '', commands: s.commands }; };
  // The request is open in the program, at the revision it was asked with, listed, and the person may dismiss it.
  const stillOpen = async r => { const s = await stateOf(r.id); return s.status === 'open' && s.revision === r.revision && s.commands.some(c => c.name === 'Dismiss' && c.enabled) && await listed(r.id); };

  // A chat really asks the person two questions through the bus helper, and the program keeps a notification of it.
  async function ask(title) {
    const session = (await call('NewSessionWithProviderAt', t.workspace, 'devin')).sessionId;
    await call('RenameSession', session, title);
    await call('Send', session, 'fixture ask', null, null);
    const request = await until(async () => (await decisions()).find(x => x.kind === 'request' && x.asker === title), 'the request of ' + title);
    const tool = await until(() => records().find(x => x.tool === 'ask_person' && x.session === session), 'the ask_person result of ' + title);
    const notice = await until(async () => (await call('Notifications')).find(x => x.requestId === request.id), 'the notification of the request of ' + title);
    await idle(title + ' ending the turn in which it asked');
    if (tool.isError) throw Error('The bus helper refused the request of ' + title + ': ' + tool.text);
    return { title, session, id: request.id, revision: request.revision, notice: notice.id };
  }

  // The bus hands a chat everything that waits for it at once and in the order it was sent (Deliver, internal/bus/bus.go).
  // So once a later message from the person has reached a chat's agent, anything the program sent that chat before it
  // has reached the agent too. A check that an agent was sent nothing, or was told once only, sends such a message
  // and waits for it, and so never waits for a fixed time.
  let sentinels = 0;
  async function sentinel(r) {
    const text = 'Sentinel ' + (++sentinels) + ' for ' + r.title + '.';
    await call('BusSend', r.title, text, 0);
    await until(() => prompts(r.session).some(x => x.includes(text)), 'the sentinel reaching ' + r.title);
    await idle(r.title + ' ending its turn after the sentinel');
    return text;
  }
  const occurrences = r => prompts(r.session).join('\n').split(WORDING).length - 1;

  // What is true of every request once the person has confirmed its dismissal, wherever it was dismissed from.
  // from is the place in the record of bridge calls just before the confirming press.
  async function dismissedOutcome(r, where, from) {
    await until(async () => !(await listed(r.id)), 'the request of ' + r.title + ' leaving the open requests');
    const made = await until(() => { const calls = t.pageCalls('DismissRequest', from); return calls.length && calls[0].answer ? calls : null; }, 'the answer to the dismissal of the request of ' + r.title);
    const input = made[0].args[0], receipt = made[0].answer.result;
    check('The page asks the program to dismiss once, with only the request, its revision and an operation identity, from ' + where + '.',
      made.length === 1 && Object.keys(input).sort().join() === 'id,operationId,revision' && input.id === r.id && input.revision === r.revision && /^[0-9a-f]{32}$/.test(input.operationId), made);
    check('The receipt of the program says committed and dismissed, from ' + where + '.',
      !!receipt && receipt.state === 'committed' && receipt.outcome === 'dismissed' && receipt.id === r.id && receipt.operationId === input.operationId && receipt.revision === r.revision + 1 && Number.isFinite(Date.parse(receipt.settledAt)), made[0].answer);
    const looked = await call('RequestOperation', r.id, input.operationId);
    check('The program keeps that receipt under the operation, from ' + where + '.', same(looked, receipt), looked);
    const state = await stateOf(r.id);
    check('The program records the request as dismissed by the person and leaves no command for it, from ' + where + '.',
      state.status === 'dismissed' && state.actorKind === 'person' && state.settledAt === receipt.settledAt && state.revision === receipt.revision && state.wording === WORDING && state.commands.every(c => !c.enabled), state);
    const message = await until(() => told(r)[0], 'the agent of ' + r.title + ' being told of the dismissal');
    await idle(r.title + ' ending the turn in which it was told');
    check('The asking agent really receives the message that the person dismissed the request without answering, from ' + where + '.',
      told(r).length === 1 && occurrences(r) === 1 && message.includes(r.id) && prompts(r.session).length === 2, prompts(r.session));
    await card(r).waitFor({ state: 'detached' });
    check('The notification of the dismissed request is gone, from ' + where + '.', !await noticeKept(r) && await row(r.id).count() === 0);
    keep({ dismissedFrom: where, request: r, input, receipt, state, message });
    r.done = { input, receipt, state };
    return r.done;
  }

  // A part that cannot finish is a failure of its own, and the later parts still run.
  async function part(name, run) {
    console.log('-- ' + name);
    try { await run(); } catch (error) {
      check('This part ran to its end: ' + name, false, String(error && error.message || error));
      keep({ unfinished: name, error: String(error && error.stack || error), page: await t.page.locator('body').innerText().catch(() => '') });
      // A dialog or a menu left open would cover the next part. Nothing is ever confirmed here.
      if (await t.page.locator('#ask[open]').count().catch(() => 0)) await dialog.answer(false).catch(() => {});
      await t.page.keyboard.press('Escape').catch(() => {});
    }
  }

  try {
    await t.start();
    // A fresh home with a working agent opens the Judgements question by itself once, in a flyout over the lower part
    // of the page (judgements-applet.js, offerSetupOnce), where it took the press meant for a button on the hosted
    // runner. This home is marked as one where the question has already opened, with the call the page itself makes,
    // before the agent is set. cold-start-hidden.cjs covers a first start.
    await call('NoteJudgementSetupShown');
    await call('SetAgentExecutable', 'devin', t.agent); await call('SetWorkspace', t.workspace);
    await openDecisions();
    const foot = await ask('Foot Chat'), menu = await ask('Menu Chat'), carded = await ask('Card Chat'), noticed = await ask('Notice Chat');
    const opened = await ask('Opened Chat'), cleared = await ask('Cleared Chat'), answered = await ask('Answered Chat'), lost = await ask('Lost Chat');
    const all = [foot, menu, carded, noticed, opened, cleared, answered, lost];
    keep({ pid: t.pid, requests: all });
    check('Eight chats each asked a request through the bus helper, and each has its own notification.', new Set(all.map(r => r.id)).size === 8 && new Set(all.map(r => r.notice)).size === 8 && all.every(r => r.revision === 1), all);

    await part('A request is dismissed from the foot of the Decisions window.', async () => {
      const r = foot, p = t.page;
      await openDecisions(); await choose(r.id);
      const button = p.locator('[data-dismiss-decision]');
      check('An open request has Dismiss in the foot, enabled, with no reason against it, and can still be answered.',
        await button.isEnabled() && await button.innerText() === 'Dismiss' && await p.locator('.decisions-dismiss-reason').innerText() === '' && !await p.getByLabel('Answer 1', { exact: true }).isDisabled() && await p.locator('.decisions-detail .notice-warn').count() === 0);
      await p.getByLabel('Answer 1', { exact: true }).fill('Keep this draft.');
      await sendEnabled();
      let from = t.calls.length;
      await button.click();
      const asked = await dialog.read();
      check('The confirmation of the window says that no answer is sent and that the asking chat is told.', asksAsApproved(asked, r), asked);
      await dialog.answer(false);
      check('Cancel in the window changes nothing.',
        t.pageCalls('DismissRequest', from).length === 0 && await stillOpen(r) && await currentRow() === r.id && await p.getByLabel('Answer 1', { exact: true }).inputValue() === 'Keep this draft.' && await savedOperation(r.id) === null && prompts(r.session).length === 1,
        { calls: t.pageCalls('DismissRequest', from), state: await stateOf(r.id), saved: await savedOperation(r.id), prompts: prompts(r.session) });
      from = t.calls.length;
      await button.click(); await dialog.read(); await dialog.answer(true);
      await row(r.id).waitFor({ state: 'detached' });
      await footSays('You dismissed the request from ' + r.title + '. No answer was sent.');
      check('Confirming removes the request from the list, and the window says that it was dismissed with no answer sent.', await row(r.id).count() === 0 && await currentRow() !== r.id && !await listed(r.id));
      const done = await dismissedOutcome(r, 'the foot of the Decisions window', from);
      const op = await savedOperation(r.id);
      check('The window keeps the dismissal as the saved operation of the request, committed.', !!op && op.method === 'DismissRequest' && op.status === 'committed' && op.receipt.state === 'committed' && op.receipt.closed === true && op.input.operationId === done.input.operationId, op);
      check('The draft answer that was typed is not sent.', t.pageCalls('AnswerDecision').length === 0 && prompts(r.session).every(x => !x.includes('Keep this draft.')), prompts(r.session));
      await p.locator('.decisions-foot-status').getByRole('button', { name: 'Show the Request', exact: true }).click();
      await p.waitForFunction(() => { const record = document.querySelector('.decisions-record'); return !!record && record.textContent.includes('You dismissed this request'); });
      const line = await p.locator('.decisions-record .decisions-delivery').innerText();
      check('The record of the dismissed request says that the person dismissed it with no answer, and is read-only.',
        line.startsWith('You dismissed this request on ') && line.endsWith('. No answer was sent. This view is read-only.') && await p.locator('.decisions-record').getByText('This question was dismissed without an answer.', { exact: true }).count() === 2 && await p.locator('.decisions-record input, .decisions-record textarea').count() === 0, line);
      await p.getByRole('button', { name: 'Back to Requests', exact: true }).click();
    });

    await part('Dismissing the same operation again does not dismiss twice and reports the first result.', async () => {
      const r = foot;
      if (!r.done) throw Error('The request was not dismissed in the part before, so there is no operation to repeat.');
      const again = await call('DismissRequest', r.done.input);
      check('The same operation sent again returns the first receipt.', same(again, r.done.receipt), again);
      const changed = await call('DismissRequest', { operationId: r.done.input.operationId, id: r.id, revision: r.done.receipt.revision });
      check('The same operation identity with other contents is refused and dismisses nothing.', changed.state === 'refused' && changed.code === 'operation_changed', changed);
      const other = await call('DismissRequest', { operationId: randomBytes(16).toString('hex'), id: r.id, revision: r.done.receipt.revision });
      check('A new operation on the dismissed request is refused as already closed.', other.state === 'refused' && other.code === 'request_closed' && other.outcome === 'dismissed', other);
      check('The record of the request has not changed, so it was dismissed once.', same(await stateOf(r.id), r.done.state) && same(await call('RequestOperation', r.id, r.done.input.operationId), r.done.receipt), await stateOf(r.id));
      await sentinel(r);
      check('The asking agent was told of the dismissal once only.', told(r).length === 1 && occurrences(r) === 1 && prompts(r.session).length === 3, prompts(r.session));
    });

    await part('A request is dismissed from the menu of its row while another request is open.', async () => {
      const r = menu, p = t.page;
      await openDecisions();
      const open = await currentRow();
      const commands = async () => { await row(r.id).locator('[data-decision-actions]').click(); const item = p.getByRole('menuitem', { name: 'Dismiss Request…' }); await item.waitFor(); return item; };
      let command = await commands();
      check('The menu of a row has Dismiss Request, enabled, after its other commands.',
        same(await p.locator('.context-menu .menubar-words').allTextContents(), ['Open', 'Open Chat', 'Open Group', 'Open Task', 'Dismiss Request…']) && await command.getAttribute('aria-disabled') !== 'true', await p.locator('.context-menu .menubar-words').allTextContents());
      let from = t.calls.length;
      await command.click();
      const asked = await dialog.read();
      check('The confirmation from a row is the one the window gives.', asksAsApproved(asked, r), asked);
      await dialog.answer(false);
      check('Cancel from a row changes nothing and leaves the open request open.', t.pageCalls('DismissRequest', from).length === 0 && await stillOpen(r) && await currentRow() === open && prompts(r.session).length === 1);
      from = t.calls.length;
      command = await commands();
      await command.click(); await dialog.read(); await dialog.answer(true);
      await row(r.id).waitFor({ state: 'detached' });
      await footSays('You dismissed the request from ' + r.title + '. No answer was sent.');
      await dismissedOutcome(r, 'the menu of a row', from);
    });

    await part('A request is dismissed from its notification card with Dismiss Request.', async () => {
      const r = carded, p = t.page;
      await openNotifications(); await card(r).waitFor();
      const button = card(r).getByRole('button', { name: 'Dismiss Request…', exact: true });
      check('The card of a request has Dismiss Request beside its other buttons, shown and enabled.',
        await button.isVisible() && await button.isEnabled() && same(await card(r).locator('.notice-actions button').allTextContents(), ['Open Request in Decisions', 'Dismiss Request…', 'Dismiss']), await card(r).locator('.notice-actions button').allTextContents());
      let from = t.calls.length;
      await button.click();
      // The card either asks, or says under its buttons why it cannot dismiss.
      const words = () => card(r).locator('.request-dismiss-status span').innerText().catch(() => '');
      const first = await until(async () => await p.locator('#ask[open]').count() ? 'asked' : await words() ? 'said' : '', 'the card asking for confirmation or saying why it cannot dismiss');
      if (first !== 'asked') {
        const read = t.pageCalls('DecisionByID', from).map(c => c.answer.error ? c.answer : { closed: c.answer.result.closed, status: c.answer.result.decision.status, canDismiss: c.answer.result.decision.canDismiss, dismissalReason: c.answer.result.decision.dismissalReason });
        const evidence = { cardSays: await words(), buttonEnabled: await button.isEnabled(), programAnsweredDecisionByID: read, programState: await stateOf(r.id), dismissRequestCalls: t.pageCalls('DismissRequest', from).length, listed: await listed(r.id), prompts: prompts(r.session) };
        keep({ cardDidNotAsk: evidence });
        check('Dismiss Request on the card asks for confirmation.', false, evidence);
        return;
      }
      const asked = await dialog.read();
      check('The confirmation from the card is the one the window gives.', asksAsApproved(asked, r), asked);
      await dialog.answer(false);
      check('Cancel on the card changes nothing.', t.pageCalls('DismissRequest', from).length === 0 && await stillOpen(r) && await card(r).count() === 1 && await button.isEnabled() && await savedOperation(r.id) === null && prompts(r.session).length === 1);
      from = t.calls.length;
      await button.click(); await dialog.read(); await dialog.answer(true);
      await card(r).waitFor({ state: 'detached' });
      await dismissedOutcome(r, 'the notification card', from);
    });

    await part('Dismissing a notification, opening its request and Clear All remove the notice only.', async () => {
      const p = t.page;
      // The plain Dismiss of a card.
      await openNotifications(); await card(noticed).waitFor();
      let from = t.calls.length;
      await card(noticed).getByRole('button', { name: 'Dismiss', exact: true }).click();
      await card(noticed).waitFor({ state: 'detached' });
      let dismissals = t.pageCalls('DismissNotification', from);
      check('Dismiss on a card dismisses that notification in the program and sends no dismissal of the request.',
        dismissals.length === 1 && dismissals[0].args[0] === noticed.notice && !dismissals[0].answer.error && t.pageCalls('DismissRequest', from).length === 0 && !await noticeKept(noticed), { dismissals, requests: t.pageCalls('DismissRequest', from) });
      check('The request whose notification was dismissed stays open.', await stillOpen(noticed) && await savedOperation(noticed.id) === null, await stateOf(noticed.id));

      // Open Request in Decisions on a card.
      await openNotifications(); await card(opened).waitFor();
      from = t.calls.length;
      await card(opened).getByRole('button', { name: 'Open Request in Decisions', exact: true }).click();
      await row(opened.id).locator('> a[aria-current="true"]').waitFor();
      await card(opened).waitFor({ state: 'detached' });
      dismissals = t.pageCalls('DismissNotification', from);
      check('Opening a request from its card shows it in Decisions, dismisses that notification, and sends no dismissal of the request.',
        dismissals.length === 1 && dismissals[0].args[0] === opened.notice && !dismissals[0].answer.error && t.pageCalls('DismissRequest', from).length === 0 && !await noticeKept(opened) && await p.locator('[data-dismiss-decision]').isEnabled(), { dismissals, requests: t.pageCalls('DismissRequest', from) });
      check('The request opened from its card stays open.', await stillOpen(opened) && await savedOperation(opened.id) === null, await stateOf(opened.id));

      // Clear All.
      await openNotifications(); await card(cleared).waitFor();
      from = t.calls.length;
      await p.locator('#needs-clear').click();
      const asked = await dialog.read();
      await dialog.answer(true);
      await card(cleared).waitFor({ state: 'detached' });
      await until(async () => !await noticeKept(cleared), 'the cleared notification leaving the list of the program');
      check('Clear All removes the notifications and sends no dismissal of any request.', asked.title === 'Clear All Notifications?' && t.pageCalls('DismissRequest', from).length === 0 && t.pageCalls('DismissDecision', from).length === 0, { asked, requests: t.pageCalls('DismissRequest', from) });
      check('The request whose notification was cleared stays open.', await stillOpen(cleared), await stateOf(cleared.id));

      // Each of the three is still shown by a fresh reading of the open requests, and its agent was sent nothing.
      for (const r of [noticed, opened, cleared]) {
        await p.evaluate(id => window.dinahDecisions.openRequest(id), r.id);
        await row(r.id).locator('> a[aria-current="true"]').waitFor();
        await sentinel(r);
        check(r.title + ' is still listed in Decisions with Dismiss and its answers enabled, and its agent was sent nothing about its request.',
          await row(r.id).count() === 1 && await p.locator('[data-dismiss-decision]').isEnabled() && !await p.getByLabel('Answer 1', { exact: true }).isDisabled() && await stillOpen(r) && prompts(r.session).length === 2 && told(r).length === 0, prompts(r.session));
      }
    });

    await part('An ordinary request is still answered, and the answer reaches the asking agent.', async () => {
      const r = answered, p = t.page;
      await openDecisions(); await choose(r.id);
      check('An open request is not said to be undeliverable, and its answers can be written.',
        (await row(r.id).innerText()).includes('2 questions waiting') && !(await row(r.id).innerText()).includes('Cannot be delivered') && await p.locator('.decisions-detail .notice-warn').count() === 0 && !await p.getByLabel('Answer 2', { exact: true }).isDisabled(), await row(r.id).innerText());
      await p.locator('[data-question="1"]').getByRole('radio', { name: 'As recommended', exact: true }).check();
      await p.getByLabel('Answer 2', { exact: true }).fill('Proceed next Monday after review.');
      await sendEnabled();
      const from = t.calls.length;
      await p.locator('.decisions-foot').getByRole('button', { name: 'Send', exact: true }).click();
      await until(async () => !(await listed(r.id)), 'the answered request leaving the open requests');
      await until(() => { const text = prompts(r.session).join('\n'); return text.includes('Use the reviewed release.') && text.includes('Proceed next Monday after review.'); }, 'both answers reaching the asking agent');
      await idle(r.title + ' ending the turn in which it was answered');
      const sent = t.pageCalls('AnswerDecision', from), state = await stateOf(r.id), text = prompts(r.session).join('\n');
      check('The program commits the answers and closes the request as answered.', sent.length === 1 && sent[0].answer.result.state === 'committed' && sent[0].answer.result.closed === true && state.status === 'answered' && t.pageCalls('DismissRequest', from).length === 0, { sent, state });
      check('Both answers reach the asking agent with their questions, and it is told of no dismissal.', text.includes('Which release should we use?') && text.includes('When should we proceed?') && told(r).length === 0, prompts(r.session));
      const count = prompts(r.session).length;
      await sentinel(r);
      check('The agent of the answered request is sent nothing more about it.', told(r).length === 0 && prompts(r.session).length === count + 1, prompts(r.session));
    });

    await part('A dismissal whose answer is lost is looked up after a reload and is not sent again.', async () => {
      const r = lost, p = t.page;
      await openDecisions(); await choose(r.id);
      let from = t.calls.length;
      // The program receives the dismissal and answers it. The answer is dropped on its way back to the page.
      t.lose = 'DismissRequest';
      // Only this one answer is dropped, whether or not the page comes to send the dismissal.
      try { await p.locator('[data-dismiss-decision]').click(); await dialog.read(); await dialog.answer(true); await footSays('Dismissing has not been confirmed.'); } finally { t.lose = ''; }
      const sent = t.pageCalls('DismissRequest', from);
      check('The program commits the dismissal, and its answer does not reach the page.', sent.length === 1 && sent[0].lost === true && sent[0].answer.result.state === 'committed' && sent[0].answer.result.outcome === 'dismissed', sent);
      const input = sent[0].args[0], receipt = sent[0].answer.result;
      await p.waitForFunction(() => { const button = document.querySelector('[data-operation]'); return !!button && !button.hidden && !button.disabled; });
      const unconfirmed = await savedOperation(r.id);
      check('The page says that the dismissal is not confirmed, keeps its operation, disables Dismiss and Send, and offers to check the result.',
        !!unconfirmed && unconfirmed.status === 'submitted' && same(unconfirmed.input, input) && await p.locator('[data-dismiss-decision]').isDisabled() && await p.locator('.decisions-dismiss-reason').innerText() === UNCHECKED && await p.locator('.decisions-foot .btn-primary').isDisabled() && await p.getByRole('button', { name: 'Check Dismiss Result', exact: true }).isVisible(),
        { unconfirmed, reason: await p.locator('.decisions-dismiss-reason').innerText() });
      await until(() => told(r)[0], 'the agent of ' + r.title + ' being told of the dismissal');
      await idle(r.title + ' ending the turn in which it was told');
      const state = await stateOf(r.id);
      from = t.calls.length;
      await p.evaluate(() => window.dinahUIState.flush());
      await p.reload(); await t.ready();
      await footSays('You dismissed the request from ' + r.title + '. No answer was sent.');
      const looked = t.pageCalls('RequestOperation', from);
      check('After the reload the page looks the saved operation up, is given the first receipt, and sends no dismissal again.',
        looked.length === 1 && looked[0].args[0] === r.id && looked[0].args[1] === input.operationId && same(looked[0].answer.result, receipt) && t.pageCalls('DismissRequest', from).length === 0, { looked, again: t.pageCalls('DismissRequest', from) });
      const confirmed = await savedOperation(r.id);
      check('The saved operation is now committed, and the record of the request has not changed.', !!confirmed && confirmed.status === 'committed' && confirmed.input.operationId === input.operationId && same(await stateOf(r.id), state) && state.status === 'dismissed' && state.settledAt === receipt.settledAt, { confirmed, state });
      check('The request dismissed with a lost answer is not listed and has no notification.', !await listed(r.id) && await row(r.id).count() === 0 && !await noticeKept(r) && await card(r).count() === 0);
      await sentinel(r);
      check('The agent of the request dismissed with a lost answer was told once only.', told(r).length === 1 && occurrences(r) === 1 && prompts(r.session).length === 3, prompts(r.session));
      r.done = { input, receipt, state };
    });

    await part('After the program is started again, what was dismissed stays dismissed and nothing is asked again.', async () => {
      const dismissed = all.filter(r => r.done), kept = [noticed, opened, cleared].concat(carded.done ? [] : [carded]);
      const before = { asked: records().filter(x => x.tool === 'ask_person').length, open: (await decisions()).map(x => x.id).sort(), told: dismissed.map(r => prompts(r.session).length) };
      const firstPid = t.pid;
      await t.page.evaluate(() => window.dinahUIState.flush());
      await t.stop();
      await t.start();
      const p = t.page;
      check('The program is running again as a new process on the same home.', t.pid !== firstPid && t.pid > 0, { firstPid, pid: t.pid });
      const open = (await decisions()).map(x => x.id).sort();
      check('The same requests are open as before the restart, and no dismissed request is among them.', same(open, before.open) && dismissed.every(r => !open.includes(r.id)) && kept.every(r => open.includes(r.id)), { open, before: before.open });
      for (const r of dismissed) {
        const state = await stateOf(r.id), looked = await call('RequestOperation', r.id, r.done.input.operationId), again = await call('DismissRequest', r.done.input);
        check('The request of ' + r.title + ' is still dismissed after the restart, with the record and the receipt it had, and its operation sent again returns that receipt.', same(state, r.done.state) && same(looked, r.done.receipt) && same(again, r.done.receipt), { state, looked, again });
      }
      const notes = (await call('Notifications')).filter(x => x.requestId || x.kind === 'mention');
      check('The program lists no notification of any request after the restart.', notes.length === 0, notes);
      await openDecisions();
      for (const id of open) await row(id).waitFor();
      check('The Decisions window lists the open requests and no dismissed one.', await p.locator('.decisions-row').count() === open.length && (await Promise.all(dismissed.map(r => row(r.id).count()))).every(n => n === 0));
      // Each asking chat is closed now. A message from the person makes the program open it again, which is when a
      // notice that was owed to it would be sent.
      for (const r of dismissed) await sentinel(r);
      check('No agent of a dismissed request is told again after the restart.', dismissed.every((r, i) => told(r).length === 1 && occurrences(r) === 1 && prompts(r.session).length === before.told[i] + 1), dismissed.map(r => prompts(r.session)));
      // A request that only lost its notification is still answered after the restart.
      const r = noticed;
      await choose(r.id);
      await p.locator('[data-question="1"]').getByRole('radio', { name: 'As recommended', exact: true }).check();
      await p.locator('[data-question="2"]').getByRole('radio', { name: 'As recommended', exact: true }).check();
      await sendEnabled();
      await p.locator('.decisions-foot').getByRole('button', { name: 'Send', exact: true }).click();
      await until(async () => !(await listed(r.id)), 'the request answered after the restart leaving the open requests');
      await until(() => { const text = prompts(r.session).join('\n'); return text.includes('Use the reviewed release.') && text.includes('Proceed tomorrow.'); }, 'the answers given after the restart reaching the asking agent');
      await idle(r.title + ' ending the turn in which it was answered');
      check('A request left open by dismissing only its notification is answered after the restart, and its agent is told of no dismissal.', (await stateOf(r.id)).status === 'answered' && told(r).length === 0, prompts(r.session));
      check('Nothing was asked again: no chat called ask_person again, and no request or notification of a request came back.',
        records().filter(x => x.tool === 'ask_person').length === before.asked && same((await decisions()).map(x => x.id).sort(), open.filter(id => id !== r.id)) && (await call('Notifications')).filter(x => x.requestId).length === 0 && await p.locator('#questions .attention-request').count() === 0,
        { asked: records().filter(x => x.tool === 'ask_person').length, open: (await decisions()).map(x => x.id) });
    });

    keep({ agentRecord: Object.fromEntries(all.map(r => [r.title, prompts(r.session)])) });
  } catch (error) { failure = error; console.error(error); }
  await t.finish(failure);
  console.log(failure || t.failures.length ? 'FAIL real dismissal of requests' : 'PASS real dismissal of requests from the Decisions window and from notification cards, across a reload and a restart');
})();
