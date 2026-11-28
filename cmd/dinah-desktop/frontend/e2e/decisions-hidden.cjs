// Real hidden desktop, real bus MCP helper, and an isolated ACP stand-in.
// Usage: node decisions-hidden.cjs <desktop.exe> <agent.exe> <fresh-output-folder>
//
// A chat really asks the person two numbered questions through the bus helper. The person answers one, then the
// other, in the Decisions window, and both answers reach the asking agent. A plain mention goes to Notifications
// and opens in its chat. Broadcast to Active previews and reaches only the chat that is working.
//
// Build the programs first, from the repository root, into one folder:
//   go build -tags desktop,production -ldflags "-H windowsgui" -o <bin>/dinah-desktop.exe ./cmd/dinah-desktop
//   go build -o <bin>/dinah.exe ./cmd/dinah
//   go build -ldflags "-H windowsgui" -o <bin>/decisions-agent.exe ./cmd/dinah-desktop/testdata/decisions-agent
// The run needs playwright-core, which NODE_PATH may give. It takes the shared browser test lock itself.
// Dismissing a request is checked by decisions-dismiss-hidden.cjs.
'use strict';
const fs = require('node:fs'), path = require('node:path');
const { hiddenDesktop } = require('./lib/hidden-desktop.cjs');
const [desktop, agent, out] = process.argv.slice(2);

(async () => {
  const t = await hiddenDesktop({ desktop, agent, out });
  const { check, until, call, records, prompts, keep } = t;
  let failure = null;
  try {
    await t.start();
    const page = t.page;
    const decisions = () => call('Decisions', 0);
    const idle = label => until(async () => (await call('UpdateState')).working === 0, label);
    const choose = async id => { await page.locator('.decisions-row[data-request-id="' + id + '"] > a').click(); await page.locator('.decisions-detail h2').waitFor(); };
    const send = async () => { await page.waitForFunction(() => !document.querySelector('.decisions-foot .btn-primary').disabled); await page.locator('.decisions-foot').getByRole('button', { name: 'Send', exact: true }).click(); };

    // A fresh home with a working agent opens the Judgements question by itself once, in a flyout over the lower part
    // of the page (judgements-applet.js, offerSetupOnce), where it took the press meant for a button on the hosted
    // runner. This home is marked as one where the question has already opened, with the call the page itself makes,
    // before the agent is set. cold-start-hidden.cjs covers a first start.
    await call('NoteJudgementSetupShown');
    await call('SetAgentExecutable', 'devin', t.agent); await call('SetWorkspace', t.workspace);
    const create = async title => { const s = await call('NewSessionWithProviderAt', t.workspace, 'devin'); await call('RenameSession', s.sessionId, title); return s.sessionId; };
    const asker = await create('Fixture Asker'), active = await create('Fixture Working'), idleChat = await create('Fixture Idle');
    keep({ sessions: { asker, active, idle: idleChat }, pid: t.pid });

    // A chat asks two numbered questions through the real bus helper.
    await page.evaluate(() => dinahWorkbench.open('decisions'));
    await call('Send', asker, 'fixture ask', null, null);
    const request = await until(async () => (await decisions()).items.find(x => x.kind === 'request'), 'the request the chat asked');
    const asked = await until(() => records().find(x => x.tool === 'ask_person'), 'the result of the ask_person tool');
    check('The bus helper accepts the two questions and returns a receipt.', !asked.isError && asked.text.includes('"id":"' + request.id + '"') && request.revision === 1 && request.questions.length === 2, asked);
    await choose(request.id);
    const detail = page.locator('.decisions-detail');
    check('Decisions shows the recommendation of each question.', (await detail.innerText()).includes('Use the reviewed release.') && (await detail.innerText()).includes('Proceed tomorrow.'), await detail.innerText());
    check('An open request that can be answered is not said to be undeliverable.', request.dismissalReason === '' && await detail.locator('.notice-warn').count() === 0 && !(await page.locator('.decisions-row[data-request-id="' + request.id + '"]').innerText()).includes('Cannot be delivered') && !await page.getByLabel('Answer 2', { exact: true }).isDisabled());
    keep({ request, asked });

    // Choosing the recommendation sends nothing. Send commits it, and only the first question is answered.
    await page.locator('[data-question="1"]').getByRole('radio', { name: 'As recommended', exact: true }).check();
    await page.waitForFunction(() => !document.querySelector('.decisions-foot .btn-primary').disabled);
    check('Choosing As recommended alone leaves the request at its first revision.', (await decisions()).items.find(x => x.id === request.id).revision === 1 && prompts(asker).length === 1, prompts(asker));
    await send();
    const partial = await until(async () => (await decisions()).items.find(x => x.id === request.id && x.revision === 2), 'the second revision of the request');
    check('Send commits the recommendation for question 1 and leaves question 2 open.', partial.questions[0].answered && partial.questions[0].answer === 'Use the reviewed release.' && partial.questions[1].answered === false, partial.questions);
    const first = await until(() => prompts(asker).find(p => p.includes('Use the reviewed release.') && p.includes('Which release should we use?')), 'the first answer reaching the asking agent');
    check('The asking agent receives the first answer with its question, readable by itself.', !first.includes('Proceed next Monday after review.'), first);
    keep({ partial, first });

    // A written answer to the second question closes the request.
    await page.getByLabel('Answer 2', { exact: true }).fill('Proceed next Monday after review.');
    await send();
    await until(async () => !(await decisions()).items.some(x => x.id === request.id), 'the request leaving the open requests');
    const second = await until(() => prompts(asker).find(p => p.includes('Proceed next Monday after review.') && p.includes('When should we proceed?')), 'the second answer reaching the asking agent');
    await idle('the asking chat ending its turn');
    await page.locator('.decisions-row[data-request-id="' + request.id + '"]').waitFor({ state: 'detached' });
    const state = await call('RequestState', request.id);
    check('The answered request is closed as answered and has left the list.', state.status === 'answered' && state.commands.every(c => !c.enabled), state);
    check('The asking agent is told nothing of a dismissal.', prompts(asker).every(p => !p.includes('dismissed this request')), prompts(asker));
    keep({ second, answers: prompts(asker).slice(1), state });

    // A plain mention of the person is a notification that opens in its chat. It is not a request.
    await call('BusStartGroup', 'Fixture Review', [{ id: asker, cwd: t.workspace, title: 'Fixture Asker' }]);
    await call('Send', asker, 'fixture mention', null, null);
    const mention = await until(async () => (await call('Notifications')).find(x => x.kind === 'mention' && !x.requestId && !x.dismissed && x.body.includes('Fixture plain decision')), 'the notification of the plain mention');
    check('A plain mention is not listed in Decisions.', (await decisions()).items.every(x => x.kind !== 'mention') && await page.locator('.decisions-row').count() === 0, await decisions());
    await idle('the mentioning chat ending its turn');
    await page.evaluate(() => dinahWorkbench.open('needs-you'));
    const card = page.locator('#questions [data-notice-id="' + mention.id + '"]');
    await card.waitFor();
    check('The card of a plain mention has no Dismiss Request button.', await card.getByRole('button', { name: 'Dismiss Request…', exact: true }).count() === 0, await card.innerText());
    await card.getByRole('button', { name: 'Open in Chat', exact: true }).click();
    await page.locator('.win.active .bus-target-message[data-bus-message="' + mention.message + '"]').waitFor();
    await until(async () => !(await call('Notifications')).some(x => x.id === mention.id && !x.dismissed), 'the mention being acknowledged');
    check('Open in Chat shows the message in its group and acknowledges the notification.', await card.count() === 0);
    keep({ acknowledgedMention: mention });

    // Broadcast to Active previews and reaches only the chat that is working.
    await idle('every chat being idle before the broadcast');
    await call('Send', active, 'fixture hold', null, null);
    await until(() => prompts(active).includes('fixture hold'), 'the working chat receiving its prompt');
    check('One chat is working before the broadcast.', (await call('UpdateState')).working === 1);
    await page.evaluate(() => dinahCommands.run('broadcast-active'));
    const broadcast = 'Fixture broadcast only to working chats.';
    await page.getByLabel('Broadcast Message', { exact: true }).fill(broadcast);
    await page.getByRole('button', { name: 'Preview Recipients', exact: true }).click();
    await page.locator('.decisions-preview li').first().waitFor();
    const frozen = await page.evaluate(() => dinahDecisionDrafts.read('broadcast-active').preview);
    const preview = await page.locator('.decisions-preview').innerText();
    check('The preview lists only the working chat.', await page.locator('.decisions-preview li').count() === 1 && frozen.count === 1 && frozen.targets[0].title === 'Fixture Working' && preview.includes('Fixture Working') && !preview.includes('Fixture Idle'), preview);
    check('Nothing is sent before the broadcast is confirmed.', !records().some(x => x.prompt && x.prompt.includes(broadcast)));
    await page.getByRole('button', { name: 'Confirm Broadcast…', exact: true }).click();
    await page.locator('#ask[open]').waitFor();
    check('The confirmation lists the chat that will receive the broadcast.', (await page.locator('#ask-body').innerText()).includes('Fixture Working'), await page.locator('#ask-body').innerText());
    await page.click('#ask-yes');
    await page.waitForFunction(() => { const line = document.querySelector('.decisions-receipt'); return !!line && line.textContent.includes('Your message is waiting'); });
    const committed = await page.evaluate(() => dinahDecisionDrafts.read('broadcast-active').operation.receipt);
    check('The broadcast is committed with one message.', committed.state === 'committed' && committed.messageIds.length === 1, committed);
    // This stand-in has no steering hook. The bus waits for its held turn.
    fs.writeFileSync(path.join(out, 'finish'), 'end held turn');
    await until(() => prompts(active).some(p => p.includes(broadcast)), 'the broadcast reaching the working chat');
    await idle('the broadcast turn ending');
    check('The broadcast reaches the working chat once and the idle chat not at all.', records().filter(x => x.prompt && x.prompt.includes(broadcast)).length === 1 && prompts(idleChat).length === 0, { idle: prompts(idleChat) });
    keep({ preview, frozen, committed, broadcast: prompts(active).filter(p => p.includes(broadcast)) });
  } catch (error) { failure = error; console.error(error); }
  await t.finish(failure);
  console.log(failure || t.failures.length ? 'FAIL real MCP questions, partial answers, mention navigation and active broadcast' : 'PASS real MCP questions, partial answers, mention navigation and active broadcast');
})();
