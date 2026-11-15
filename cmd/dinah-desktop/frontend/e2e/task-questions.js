const fs = require('node:fs'), path = require('node:path');
const { launch, OUT } = require('./lib/page');
const question = (key, extra = {}) => Object.assign({ key, service: 'fixture.flow', serviceName: 'Workflow', workspace: 'ws-a', workspaceName: 'Harbor', task: 'task-a', generation: 'g1', taskTitle: 'Review the release', question: 'May the reviewed release check proceed?', askedAt: '2026-11-15T10:00:00Z', target: { href: '?open=fixture-task&view.fixture-task=task%3Dtask-a' }, options: [{ optionId: 'go', name: 'Run the check', kind: 'allow_once' }, { optionId: 'stop', name: 'Keep it paused', kind: 'reject_once' }] }, extra);
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, args: { initial: question('initial') }, ready: 'workbench',
    app: args => {
      window.__taskPending = [args.initial]; window.__answers = []; window.__chatPending = [];
      return {
        PendingTaskPermissions: async () => {
          if (window.__pendingFailure) throw Error('Questions unavailable');
          const snapshot = JSON.parse(JSON.stringify(__taskPending));
          if (window.__holdPending) await new Promise(resolve => { window.__releasePending = resolve; });
          return snapshot;
        },
        PendingPermissions: async () => JSON.parse(JSON.stringify(__chatPending)),
        Answer: async (key, option) => {
          __answers.push([key, option]);
          if (window.__holdAnswer) await new Promise(resolve => { window.__releaseAnswer = resolve; });
          if (window.__answerFailure) throw Error('Answer unavailable');
          __taskPending = __taskPending.filter(q => q.key !== key);
          if (window.__loseAfterCommit) throw Error('Answer response lost');
          __emit('task:answered', { key, service: 'fixture.flow', workspace: 'ws-a', task: 'task-a', generation: 'g1' });
        }
      };
    }
  });
  const { page: p, ok } = t, card = key => p.locator('#questions [data-task-question-key="' + key + '"]');
  try {
    const chatWindows = await p.locator('.win[data-chat-current]').count();
    await p.evaluate(() => {
      window.__taskOpens = [];
      dinahWorkbench.register('fixture-task', { applet: { id: 'fixture.task', title: 'Task', glyph: 'file' }, singleton: true, title: 'Task', init: (root, opts) => { __taskOpens.push(opts.state); root.textContent = 'Task details'; return {}; } });
      return dinahWorkbench.open('needs-you');
    });
    await card('initial').waitFor({ state: 'visible' });
    ok('initial pending query recovers a task question with its owner', (await card('initial').innerText()).includes('Workflow asks about "Review the release" in Harbor.'));
    ok('task questions have only offered choices and no chat or dismissal action', await card('initial').locator('input[type=radio]').count() === 2 && await card('initial').locator('textarea, .notices-open, .notices-dismiss').count() === 0);
    await card('initial').getByRole('link', { name: 'Open Task' }).click();
    ok('the authoritative task destination opens through ordinary page navigation', JSON.stringify(await p.evaluate(() => __taskOpens)) === '["task=task-a"]');
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await p.getByPlaceholder('Filter by Text').focus();
    const unavailable = question('no-page', { taskTitle: '<img src=x onerror=alert(1)>', target: { reason: 'The task page is unavailable.' } });
    await p.evaluate(q => { __taskPending.push(q); __emit('task:permission', q); __emit('task:permission', q); }, unavailable);
    await card('no-page').waitFor({ state: 'visible' });
    ok('a task question arrives without stealing keyboard focus', await p.getByPlaceholder('Filter by Text').evaluate(el => el === document.activeElement));
    ok('an unavailable task page leaves one answerable question and plain source text', await card('no-page').count() === 1 && await card('no-page').getByRole('link', { name: 'Open Task' }).count() === 0 && (await card('no-page').innerText()).includes('You can still answer') && await card('no-page').locator('img').count() === 0);
    await p.evaluate(() => { __chatPending = [{ key: 'chat-one', sessionId: 'chat-one', toolCall: { title: 'Run the chat check' }, options: [{ optionId: 'yes', name: 'Allow chat check', kind: 'allow_once' }] }]; __emit('agent:permission', __chatPending[0]); __resync(); });
    await p.waitForFunction(() => document.querySelector('#questions [data-permission-key="chat-one"]'));
    ok('chat and task questions coexist after independent reconciliation', await card('initial').count() === 1 && await card('no-page').count() === 1);
    await p.evaluate(() => { __chatPending = []; __resync(); });
    await p.waitForFunction(() => !document.querySelector('#questions [data-permission-key="chat-one"]'));
    ok('an empty chat snapshot cannot erase task questions', await card('initial').count() === 1);
    // A second mounted reading surface proves shared state without changing Decisions.
    await p.evaluate(() => { const copy = document.createElement('section'); copy.id = 'task-question-copy'; copy.hidden = true; document.body.appendChild(copy); dinahTaskQuestions.render(copy, 'initial'); });
    await card('initial').getByRole('radio', { name: 'Run the check', exact: true }).check();
    await p.evaluate(() => { window.__holdAnswer = true; window.__taskFrames = []; const owner = document.querySelector('#questions [data-task-question-key="initial"]'); const nodes = [owner, owner.querySelector('.task-question-choices'), owner.querySelector('.notice-actions'), document.querySelector('#questions [data-task-question-key="no-page"]')]; window.__sampleTaskFrames = true; function frame() { __taskFrames.push(nodes.map(n => { const b = n.getBoundingClientRect(); return [b.x,b.y,b.width,b.height]; })); if (__sampleTaskFrames) requestAnimationFrame(frame); } requestAnimationFrame(frame); });
    await card('initial').getByRole('button', { name: 'Send Answer', exact: true }).click();
    await p.waitForFunction(() => __answers.length === 1);
    await p.evaluate(() => new Promise(resolve => { let n = 0; function tick() { if (++n === 12) { __sampleTaskFrames = false; resolve(); } else requestAnimationFrame(tick); } requestAnimationFrame(tick); }));
    const frames = await p.evaluate(() => __taskFrames);
    fs.writeFileSync(path.join(OUT, 'task-question-frames.json'), JSON.stringify(frames, null, 2));
    ok('sending preserves the card, choices, controls and following card in every frame', frames.length >= 12 && frames.every(f => JSON.stringify(f) === JSON.stringify(frames[0])));
    ok('answering disables all mounted copies and sends one key and offered option', await p.locator('[data-task-question-key="initial"] input:enabled').count() === 0 && JSON.stringify(await p.evaluate(() => __answers)) === '[["initial","go"]]');
    await p.evaluate(() => { __holdAnswer = false; __releaseAnswer(); });
    await card('initial').waitFor({ state: 'detached' });
    ok('an answer removes Notifications and ends the shared question', await p.evaluate(() => !dinahTaskQuestions.get('initial')));
    await t.emit('task:permission', question('initial'));
    ok('a replay cannot restore an answered question', await p.evaluate(() => !dinahTaskQuestions.get('initial')));
    await p.evaluate(() => { __taskPending = []; __emit('task:answered', { key: 'no-page' }); });
    await card('no-page').waitFor({ state: 'detached' });
    const late = question('late');
    await p.evaluate(q => { __taskPending = [q]; __emit('task:permission', q); window.__holdPending = true; window.__refresh = dinahTaskQuestions.refresh(); }, late);
    await p.waitForFunction(() => !!window.__releasePending);
    await p.evaluate(() => { __emit('task:answered', { key: 'late' }); __holdPending = false; __releasePending(); return __refresh; });
    ok('a terminal event wins over an older pending snapshot', await p.evaluate(() => !dinahTaskQuestions.get('late')));
    await p.evaluate(() => { __taskPending = []; __holdPending = true; __releasePending = null; window.__refresh = dinahTaskQuestions.refresh(); });
    await p.waitForFunction(() => !!window.__releasePending);
    await t.emit('task:permission', question('new-arrival'));
    await p.evaluate(() => { __holdPending = false; __releasePending(); return __refresh; });
    ok('an old empty snapshot cannot erase a question that just arrived', await p.evaluate(() => !!dinahTaskQuestions.get('new-arrival')));
    await t.emit('task:answered', { key: 'new-arrival' });
    const lost = question('lost');
    await p.evaluate(q => { __taskPending = [q]; __emit('task:permission', q); __answerFailure = true; __pendingFailure = true; }, lost);
    await card('lost').getByRole('radio', { name: 'Run the check', exact: true }).check();
    await card('lost').getByRole('button', { name: 'Send Answer', exact: true }).click();
    await card('lost').getByRole('status').filter({ hasText: 'Check the question status' }).waitFor();
    ok('an uncertain answer stays disabled while its status cannot be read', await card('lost').getByRole('button', { name: 'Send Answer', exact: true }).isDisabled() && await p.evaluate(() => __answers.length === 2));
    await p.evaluate(() => { __pendingFailure = false; __answerFailure = false; });
    await card('lost').getByRole('button', { name: 'Check Question Status' }).click();
    await card('lost').getByRole('status').filter({ hasText: 'still waiting' }).waitFor();
    ok('authoritative pending status permits an explicit retry without sending one', await card('lost').getByRole('button', { name: 'Send Answer', exact: true }).isEnabled() && await p.evaluate(() => __answers.length === 2));
    for (const close of await p.locator('.toast-close').all()) await close.click();
    for (const width of [1440, 390]) {
      await p.setViewportSize({ width, height: 1000 });
      await p.evaluate(() => pudlWindows.snap('needs-you', 'maximized'));
      for (const mode of ['light', 'dark']) {
        await p.evaluate(mode => pudlSetTheme(mode), mode);
        await card('lost').scrollIntoViewIfNeeded();
        await card('lost').getByRole('button', { name: 'Send Answer', exact: true }).focus();
        await p.keyboard.press('Tab');
        ok(mode + ' task controls have a visible keyboard focus ring at ' + width, await card('lost').getByRole('button', { name: 'Check Question Status' }).evaluate(el => { const s = getComputedStyle(el); return el === document.activeElement && (s.outlineStyle !== 'none' && parseFloat(s.outlineWidth) > 0 || s.boxShadow !== 'none'); }));
        ok(mode + ' task question fits at ' + width, await card('lost').evaluate(el => el.scrollWidth <= el.clientWidth + 1 && Array.from(el.querySelectorAll('button,input')).every(n => { const a=n.getBoundingClientRect(), b=el.getBoundingClientRect(); return a.left >= b.left && a.right <= b.right; })));
        await t.shot('task-questions-' + mode + '-' + width);
      }
    }
    await card('lost').getByRole('button', { name: 'Send Answer', exact: true }).click();
    await card('lost').waitFor({ state: 'detached' });
    await p.evaluate(q => { __taskPending = [q]; __loseAfterCommit = true; __emit('task:permission', q); }, question('lost-after-commit'));
    await card('lost-after-commit').getByRole('radio', { name: 'Run the check', exact: true }).check();
    const answersBeforeLostCommit = await p.evaluate(() => __answers.length);
    await card('lost-after-commit').getByRole('button', { name: 'Send Answer', exact: true }).click();
    await card('lost-after-commit').waitFor({ state: 'detached' });
    ok('a response lost after an answer is reconciled without repeating it', await p.evaluate(() => __answers.length) === answersBeforeLostCommit + 1 && await p.evaluate(() => !dinahTaskQuestions.get('lost-after-commit')));
    await p.evaluate(() => { __taskPending = []; __chatPending = [{ key: 'chat-two', sessionId: 'chat-two', toolCall: { title: 'Keep chat question' }, options: [{ optionId: 'yes', name: 'Keep chat question', kind: 'allow_once' }] }]; __emit('agent:permission', __chatPending[0]); return dinahTaskQuestions.refresh(); });
    await p.locator('#questions [data-permission-key="chat-two"]').waitFor({ state: 'attached' });
    ok('an empty task snapshot cannot erase a chat question', await p.locator('#questions [data-permission-key="chat-two"]').count() === 1);
    const taskCalls = (await t.calls()).filter(c => /OpenSession|NewSession|TakeTurn/.test(c[0]));
    ok('task questions and answers open no chat or provider and write no answer row', taskCalls.length === 0 && await p.locator('.win[data-chat-current]').count() === chatWindows && await p.locator('.answer-row').count() === 0, JSON.stringify({ taskCalls, chatWindowsBefore: chatWindows, chatWindowsAfter: await p.locator('.win[data-chat-current]').count(), answerRows: await p.locator('.answer-row').count() }));
  } finally { await t.close(); }
})();
