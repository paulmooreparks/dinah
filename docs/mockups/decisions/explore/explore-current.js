// Drives the shipped Decisions page headless through the e2e fixture with invented
// requests, and writes captures and frame records for the design review.
//   node explore-current.js <unused> <out folder>
const path = require('path'), fs = require('fs');
const E2E = path.join(__dirname, '..', '..', '..', '..', 'cmd', 'dinah-desktop', 'frontend', 'e2e');
const { setup, makeRequest } = require(path.join(E2E, 'lib', 'decisions-fixture'));
const OUT = process.argv[3] || path.join(require('os').tmpdir(), 'decisions-explore');
fs.mkdirSync(OUT, { recursive: true });
const notes = [];
const note = (k, v) => { notes.push([k, v]); console.log('NOTE ' + k + ' :: ' + (typeof v === 'string' ? v : JSON.stringify(v))); };

const at = (h, m) => '2026-11-15T' + String(h).padStart(2, '0') + ':' + String(m).padStart(2, '0') + ':00Z';
const longText = ['I reviewed the release candidate against the checklist and found three things worth your attention before we go further.', '',
  'First, the installer still writes the old shortcut name on a machine that had the 1.x build. It is cosmetic but people will see it.', '',
  'Second, the Harbor publish step took 19 seconds on the Harbor 07 page, which is under the limit but close to it. I have a fix ready that halves it.', '',
  'Third, the release notes mention a Recycle Bin that is not in this build. I can either cut that paragraph or hold the release until the bin lands.', '',
  ...Array.from({ length: 14 }, (_, i) => 'Detail line ' + (i + 1) + ': the measurements and file names that back the three points above, kept here so you can check them without opening the group.'),
  '', 'I will wait for your answer before touching the release branch.'].join('\n');
const items = () => [
  makeRequest('request:101', { messageId: 101, createdAt: at(9, 12), asker: 'Release Engineer', workspace: 'studio', group: '#release', groupTitle: 'Release 2.0.2', destination: 'Release 2.0.2, replying to Release Engineer',
    text: 'Two decisions before I cut the candidate.', questions: [
      { number: 1, text: 'Should 2.0.2 ship with the shortened publish step, or keep the current one and ship the fix in 2.0.3?', recommendation: 'Ship the shortened step in 2.0.2. It is covered by the publish suite and halves the slowest page.', choices: ['Ship it in 2.0.2', 'Hold it for 2.0.3'], answered: false, answer: '' },
      { number: 2, text: 'The notes mention the Recycle Bin, which is not in this build. Cut the paragraph or hold the release?', recommendation: 'Cut the paragraph and ship on time.', choices: [], answered: false, answer: '' }] }),
  makeRequest('mention:102', { kind: 'mention', questions: [], canDismiss: true, messageId: 102, createdAt: at(9, 40), asker: 'Dinah Design Review', workspace: 'studio', group: '#desktop', groupTitle: 'Dinah Desktop Team', destination: 'Dinah Desktop Team, replying to Dinah Design Review', text: longText }),
  makeRequest('request:103', { messageId: 103, createdAt: at(10, 5), asker: 'Harbor Companion Lead', workspace: 'harbor', group: '#harbor', groupTitle: 'Harbor Companion', destination: 'Harbor Companion, replying to Harbor Companion Lead', text: 'Five setup questions.', questions: [
      { number: 1, text: 'Which Harbor project holds customer requirements?', recommendation: 'Harbor, the existing project.', choices: ['Harbor', 'A new project'], answered: false, answer: '' },
      { number: 2, text: 'May the agent create Harbor issues without asking each time?', recommendation: 'Yes, inside Harbor only.', choices: ['Yes', 'No'], answered: false, answer: '' },
      { number: 3, text: 'How long should a draft page wait before it is published?', recommendation: 'Publish when you approve it, not on a timer.', choices: [], answered: false, answer: '' },
      { number: 4, text: 'Should the trail of a requirement live on the Harbor issue?', recommendation: 'Yes, on the issue.', choices: [], answered: false, answer: '' },
      { number: 5, text: 'Who reviews a case ID before it is used?', recommendation: 'Daniel Palmer.', choices: [], answered: false, answer: '' }] }),
  makeRequest('request:104', { messageId: 104, createdAt: at(10, 30), asker: 'Org Chart Builder', workspace: 'studio', group: '', groupTitle: '', destination: 'Org Chart Builder', text: 'Two questions.', questions: [
      { number: 1, text: 'Should retired chats appear on the org chart?', recommendation: 'Show them greyed under their last team.', choices: [], answered: true, answer: 'Hide them.' },
      { number: 2, text: 'Should the chart open on the team or on the whole workspace?', recommendation: 'Open on the team you came from.', choices: [], answered: false, answer: '' }] }),
  makeRequest('request:105', { messageId: 105, createdAt: at(8, 2), asker: 'Old Migration Agent', workspace: 'studio', group: '#migrate', groupTitle: 'Migration', destination: 'Migration, replying to Old Migration Agent', canDismiss: true,
    dismissalReason: 'The asking chat was retired, so an answer can no longer be delivered. Dismiss this request to close it.', questions: [
      { number: 1, text: 'Should the old settings file be deleted after migration?', recommendation: 'Keep it for 30 days.', choices: [], answered: false, answer: '' }] }),
];
const context = [
  { id: 90, room: '#release', from: 'Dinah Team Lead', text: 'Release Engineer, Paul wants two answers from you before the candidate.', createdAt: at(9, 0) },
  { id: 95, room: '#desktop', from: 'Dinah Team Lead', text: 'Design Review, please look at the release candidate before Release Engineer cuts it.', createdAt: at(9, 30) },
  { id: 98, room: '#desktop', from: 'Release Engineer', text: 'Candidate is on the branch. The publish fix is in, the notes are not final.\nI will wait for Design Review.', createdAt: at(9, 35) },
];

async function frames(p, sel, run) {
  await p.evaluate(sel => {
    window.__frames = []; window.__sampling = true;
    const tick = () => {
      if (!window.__sampling) return;
      const send = document.querySelector(sel.send), dismiss = document.querySelector(sel.dismiss), receipt = document.querySelector('.decisions-receipt');
      const box = el => { if (!el) return null; const r = el.getBoundingClientRect(); return [Math.round(r.x), Math.round(r.y), Math.round(r.width), Math.round(r.height)]; };
      window.__frames.push({ t: Math.round(performance.now()), send: send ? send.disabled : null, sendStyle: send ? getComputedStyle(send).opacity + '/' + getComputedStyle(send).backgroundColor : null, dismiss: dismiss ? dismiss.disabled : null,
        receipt: receipt ? receipt.textContent : null, sendBox: box(send), scroll: (document.querySelector('.decisions-content') || {}).scrollTop });
      requestAnimationFrame(tick);
    };
    requestAnimationFrame(tick);
  }, sel);
  await run();
  await p.waitForTimeout(300);
  return p.evaluate(() => { window.__sampling = false; return window.__frames; });
}

(async () => {
  // 1. No requests, light and dark, wide and narrow.
  let t = await setup([]);
  let p = t.page;
  try {
    await p.waitForFunction(() => document.querySelector('.decisions-status').textContent.includes('no open'));
    await t.shot('01-empty-1440-light');
    note('empty status', await p.locator('.decisions-status').innerText());
    note('empty detail', await p.locator('.decisions-detail').innerText());
    note('earlier disabled', await p.getByRole('button', { name: 'Earlier Requests' }).isDisabled());
    note('earlier title', await p.getByRole('button', { name: 'Earlier Requests' }).getAttribute('title'));
    await p.evaluate(() => document.documentElement.setAttribute('data-theme', 'dark'));
    await t.shot('02-empty-1440-dark');
    await p.evaluate(() => document.documentElement.setAttribute('data-theme', 'light'));
    await p.setViewportSize({ width: 390, height: 844 });
    await p.waitForTimeout(300);
    await t.shot('03-empty-390-light');
  } finally { await t.close(); }

  // 2. Several requests.
  t = await setup(items());
  p = t.page;
  try {
    await p.evaluate(c => {
      const App = window.go.main.App, save = App.SaveUIState;
      // The real bridge crosses to Go for every save; a quarter of a frame's budget is generous.
      App.SaveUIState = async v => { await new Promise(r => setTimeout(r, window.__saveLatency || 0)); return save(v); };
      App.BusGroupMessageContext = async (group, id) => JSON.parse(JSON.stringify(c.filter(m => m.room === group && m.id < id)));
    }, context);
    await p.waitForFunction(() => document.querySelectorAll('.decisions-row').length === 5);
    await t.shot('04-list-1440-light');
    note('list rows', await p.locator('.decisions-list').innerText());
    note('detail with nothing chosen', await p.locator('.decisions-detail').innerText());
    const bounds = await p.evaluate(() => ['.decisions-toolbar', '.decisions-status', '.decisions-list', '.decisions-detail'].map(s => { const r = document.querySelector(s).getBoundingClientRect(); return s + ' ' + [r.x, r.y, r.width, r.height].map(Math.round).join(','); }));
    note('regions', bounds);

    // Read and answer a two-question request.
    await p.locator('.decisions-row[data-request-id="request:101"] > a').click();
    await p.locator('.decisions-detail h2').waitFor();
    await p.waitForTimeout(200);
    await t.shot('05-request-1440-light');
    note('request detail text', await p.locator('.decisions-detail').innerText());
    const geo = await p.evaluate(() => { const f = document.querySelector('.decisions-detail form').getBoundingClientRect(), c = document.querySelector('.decisions-content'), ctx = document.querySelector('.decisions-context').getBoundingClientRect(); return { formX: Math.round(f.x), formW: Math.round(f.width), contentScroll: c.scrollHeight + '/' + c.clientHeight, contextH: Math.round(ctx.height) }; });
    note('reading geometry', geo);
    const sendSel = { send: '.decisions-detail .btn-primary', dismiss: '[data-dismiss-decision]' };
    note('send disabled before typing', await p.locator(sendSel.send).isDisabled());
    // Type at a person's pace and record every frame.
    await p.evaluate(() => { window.__saveLatency = 30; });
    const input = p.getByLabel('Answer 1', { exact: true });
    await input.click();
    const typing = await frames(p, sendSel, async () => { await p.keyboard.type('Ship it in 2.0.2, and tell me the timing after.', { delay: 70 }); });
    const flips = typing.reduce((n, f, i) => n + (i && f.send !== typing[i - 1].send ? 1 : 0), 0);
    const disabledFrames = typing.filter(f => f.send).length;
    const receipts = [...new Set(typing.map(f => f.receipt))];
    note('typing frames', { frames: typing.length, sendFlips: flips, framesDisabled: disabledFrames, receiptTexts: receipts });
    fs.writeFileSync(path.join(OUT, 'typing-frames.json'), JSON.stringify(typing, null, 1));
    // Catch the disabled frame on camera: hold the save so the momentary state stays.
    await p.evaluate(() => { window.__holdSave = true; });
    await p.keyboard.type('!');
    await p.waitForTimeout(100);
    await t.shot('06-typing-send-disabled-during-save');
    note('receipt while saving', await p.locator('.decisions-receipt').innerText());
    await p.evaluate(() => { window.__holdSave = false; window.__releaseSave && window.__releaseSave(); });
    await p.waitForTimeout(200);
    await t.shot('07-typing-send-enabled-after-save');
    // As Recommended for question 2.
    const rec = await frames(p, sendSel, async () => { await p.locator('[data-question="2"]').getByRole('button', { name: 'As Recommended' }).click(); });
    note('as recommended frames', { flips: rec.reduce((n, f, i) => n + (i && f.send !== rec[i - 1].send ? 1 : 0), 0) });
    note('answer 2 after As Recommended', await p.getByLabel('Answer 2', { exact: true }).inputValue());
    await p.locator('[data-question="2"]').scrollIntoViewIfNeeded();
    await t.shot('08-as-recommended');
    // Send.
    const sent = await frames(p, sendSel, async () => { await p.locator(sendSel.send).click(); await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('was sent')); });
    fs.writeFileSync(path.join(OUT, 'send-frames.json'), JSON.stringify(sent, null, 1));
    note('send frames receipts', [...new Set(sent.map(f => f.receipt))]);
    await p.waitForTimeout(300);
    await t.shot('09-after-send-lingers');
    note('after send detail', await p.locator('.decisions-detail').innerText());
    note('after send list ids', await p.$$eval('.decisions-row', rows => rows.map(r => r.dataset.requestId)));
    note('after send controls', await p.evaluate(() => [...document.querySelectorAll('.decisions-detail button,.decisions-detail textarea')].map(b => (b.textContent || b.getAttribute('aria-label')) + ':' + (b.disabled ? 'disabled' : 'enabled'))));

    // The mention with a long quoted message.
    await p.locator('.decisions-row[data-request-id="mention:102"] > a').click();
    await p.waitForFunction(() => document.querySelector('.decisions-detail h2').textContent === 'Dinah Design Review');
    await p.waitForTimeout(300);
    await t.shot('10-mention-long-1440');
    note('mention labels', await p.evaluate(() => [...document.querySelectorAll('.decisions-detail legend, .decisions-detail label')].map(e => e.tagName + ':' + e.childNodes[0].textContent)));
    const nested = await p.evaluate(() => { const c = document.querySelector('.decisions-content'), ctx = document.querySelector('.decisions-context-body'), q = document.querySelector('.decisions-question p'); return { contentScroll: c.scrollHeight + '/' + c.clientHeight, contextScroll: ctx.scrollHeight + '/' + ctx.clientHeight, questionHeight: Math.round(q.getBoundingClientRect().height) }; });
    note('mention nested scrolling', nested);
    await p.locator('.decisions-content').evaluate(el => { el.scrollTop = el.scrollHeight; });
    await p.waitForTimeout(100);
    await t.shot('11-mention-scrolled-to-actions');
    // Dismiss through the confirmation.
    await p.locator('[data-dismiss-decision]').click();
    await p.locator('#ask').waitFor();
    await t.shot('12-dismiss-confirm');
    note('dismiss dialog', await p.locator('#ask').innerText());
    await p.locator('#ask').getByRole('button', { name: 'Dismiss', exact: true }).click();
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('dismissed'));
    await p.waitForTimeout(300);
    await p.locator('.decisions-content').evaluate(el => { el.scrollTop = 0; });
    await t.shot('13-dismissed-lingers');
    note('dismissed controls', await p.evaluate(() => [...document.querySelectorAll('.decisions-detail button,.decisions-detail textarea')].map(b => (b.textContent || b.getAttribute('aria-label')) + ':' + (b.disabled ? 'disabled' : 'enabled'))));

    // The multi-question request.
    await p.locator('.decisions-row[data-request-id="request:103"] > a').click();
    await p.waitForFunction(() => document.querySelector('.decisions-detail h2').textContent === 'Harbor Companion Lead');
    await p.waitForTimeout(200);
    await t.shot('14-multi-question');
    note('multi scroll', await p.locator('.decisions-content').evaluate(el => el.scrollHeight + '/' + el.clientHeight));
    // Answer only question 3 and send: partial answer.
    await p.getByLabel('Answer 3', { exact: true }).fill('Publish when I approve it.');
    await p.waitForFunction(() => !document.querySelector('.decisions-detail .btn-primary').disabled);
    await p.locator('.decisions-detail .btn-primary').click();
    await p.waitForFunction(() => document.querySelector('.decisions-receipt').textContent.includes('was sent'));
    await p.waitForTimeout(400);
    await p.locator('[data-question="3"]').scrollIntoViewIfNeeded();
    await t.shot('15-partial-answer-sent');
    note('partial answered q3', await p.locator('[data-question="3"]').innerText());

    // Partly answered earlier.
    await p.locator('.decisions-row[data-request-id="request:104"] > a').click();
    await p.waitForFunction(() => document.querySelector('.decisions-detail h2').textContent === 'Org Chart Builder');
    await p.waitForTimeout(200);
    await t.shot('16-partly-answered');
    note('partly answered', await p.locator('.decisions-content').innerText());

    // Retired asker, the closest thing to an expired request.
    await p.locator('.decisions-row[data-request-id="request:105"] > a').click();
    await p.waitForFunction(() => document.querySelector('.decisions-detail h2').textContent === 'Old Migration Agent');
    await p.waitForTimeout(200);
    await t.shot('17-retired-asker');
    note('retired controls', await p.evaluate(() => [...document.querySelectorAll('.decisions-detail button,.decisions-detail textarea')].map(b => (b.textContent || b.getAttribute('aria-label')) + ':' + (b.disabled ? 'disabled' : 'enabled'))));

    // Filter.
    await p.getByLabel('Filter Requests').fill('Harbor');
    await p.waitForTimeout(200);
    await t.shot('18-filter');
    note('filter rows', await p.$$eval('.decisions-row', rows => rows.map(r => r.dataset.requestId)));
    await p.getByLabel('Filter Requests').fill('zzz');
    await p.waitForTimeout(200);
    await t.shot('19-filter-none');
    note('filter none list', await p.locator('.decisions-list').innerText());
    await p.getByLabel('Filter Requests').fill('');

    // Earlier Requests with a small page.
    await p.getByRole('button', { name: 'Requests', exact: true }).click();
    await p.evaluate(() => { __decisionItems.sort((a, b) => b.messageId - a.messageId); window.__pageSize = 2; __emit('bus:changed', {}); });
    await p.waitForTimeout(500);
    note('earlier enabled with more', !(await p.getByRole('button', { name: 'Earlier Requests' }).isDisabled()));
    await p.getByRole('button', { name: 'Earlier Requests' }).click();
    await p.waitForTimeout(400);
    await t.shot('20-earlier-requests');
    note('after earlier rows', await p.$$eval('.decisions-row', rows => rows.map(r => r.dataset.requestId)));
    await p.evaluate(() => { window.__pageSize = 100; });

    // Dark and narrow.
    await p.locator('.decisions-row[data-request-id="request:104"] > a').click();
    await p.evaluate(() => document.documentElement.setAttribute('data-theme', 'dark'));
    await p.waitForTimeout(200);
    await t.shot('21-request-1440-dark');
    await p.evaluate(() => document.documentElement.setAttribute('data-theme', 'light'));
    await p.setViewportSize({ width: 390, height: 844 });
    await p.waitForTimeout(400);
    await t.shot('22-request-390');
    await p.getByRole('button', { name: 'Requests', exact: true }).click();
    await p.waitForTimeout(300);
    await t.shot('23-list-390');
  } finally { await t.close(); }
  fs.writeFileSync(path.join(OUT, 'notes.json'), JSON.stringify(notes, null, 1));
})().catch(e => { console.error(e); process.exitCode = 1; });
