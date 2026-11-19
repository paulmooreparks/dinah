// A thought is a row of the transcript, shown when Show Thinking is on, for
// any agent program (Paul, 20 November 2026: thinking was not shown although
// the setting was on, since a thought sat inside the Working block, which
// starts closed). The Working block has the tool calls only.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1200, height: 900 }, app: () => ({ OpenSessionAt: async (cwd, sessionId) => ({ sessionId, cwd, provider: 'codex', open: true, working: true }) }) });
  const p = t.page, id = 'codex:thinking';
  const emit = update => p.evaluate(({ id, update }) => __emit('agent:update', { sessionId: id, update }), { id, update });
  const thought = text => emit({ sessionUpdate: 'agent_thought_chunk', content: { type: 'text', text } });
  try {
    const key = await p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: id }), id);
    const win = p.locator('[data-win="' + key + '"]');
    await win.locator('[data-view-id="prompt"]').waitFor();
    await win.locator('[data-win-action="maximize"]').click();
    await p.evaluate(() => document.body.classList.add('show-thoughts'));
    await emit({ sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Do the work' } });
    await thought('First I look for the setting.');
    const first = win.locator('.thought-row', { hasText: 'First I look for the setting.' });
    await first.waitFor();
    t.ok('with Show Thinking on, a thought is shown at once, with nothing to open', await first.isVisible());
    t.ok('it is a row of the transcript and is in no work block', await first.evaluate(el => !!el.closest('.transcript') && !el.closest('.work-group') && el.parentElement.classList.contains('run-log')));
    t.ok('it has the word Thinking, its time and its text', await first.evaluate(el => el.querySelector('.thought-label').textContent === 'Thinking' && !!el.querySelector('.activity-time') && el.querySelector('.thought').textContent === 'First I look for the setting.'));
    const place = await first.evaluate(el => { const l = el.querySelector('.thought-label').getBoundingClientRect(), c = el.querySelector('.activity-time').getBoundingClientRect(), x = el.querySelector('.thought').getBoundingClientRect(); return { sameLine: Math.abs(l.top - c.top) < 6, timeAtEnd: c.left > l.right, textUnder: x.top >= l.bottom - 1 }; });
    t.ok('the word and the time are on one line and the text is under them', place.sameLine && place.timeAtEnd && place.textUnder, JSON.stringify(place));
    t.ok('thinking alone makes no work block', await p.locator('.work-group').count() === 0);
    await emit({ sessionUpdate: 'tool_call', toolCallId: 'one', title: 'Read settings', kind: 'read', status: 'completed' });
    await p.locator('.work-group').waitFor();
    await thought('The second place keeps a copy.');
    await win.locator('.thought-row', { hasText: 'The second place keeps a copy.' }).waitFor();
    t.ok('the work block has the tool call and no thought', await p.locator('.work-group').evaluate(el => el.querySelectorAll('.tool-row').length === 1 && el.querySelectorAll('.thought-row').length === 0));
    await emit({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'The change is made.' } });
    await p.evaluate(id => __emit('agent:turn', { sessionId: id, stopReason: 'end_turn', seconds: 5 }), id);
    await p.waitForFunction(() => document.querySelector('.work-title')?.textContent === 'Worked for 5 s');
    const order = await win.locator('.transcript .run-log > .activity-row').evaluateAll(rows => rows.map(r => r.classList.contains('work-group') ? 'work' : r.classList.contains('thought-row') ? 'thought' : r._msg ? r._msg.kind : 'other').filter(k => k !== 'other'));
    t.ok('finished work is put ahead of the thoughts and answers that followed the request', order.join(' ') === 'you work thought thought agent', order.join(' '));
    await t.shot('thinking-rows');
    // Off: no thought is shown anywhere, and the work block is where it was.
    const before = await p.locator('.work-group').evaluate(el => el.getBoundingClientRect().top);
    await p.evaluate(() => document.body.classList.remove('show-thoughts'));
    t.ok('with Show Thinking off, no thought is shown', await win.locator('.thought-row').evaluateAll(rows => rows.every(r => r.getBoundingClientRect().height === 0)));
    t.ok('the work block stays where it was', Math.abs(await p.locator('.work-group').evaluate(el => el.getBoundingClientRect().top) - before) < 1);
  } finally { await t.close(); }
})();
