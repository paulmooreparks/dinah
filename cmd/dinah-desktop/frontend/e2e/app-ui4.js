// Reopening long sessions, a refused reopen, and choosing a model before a
// session exists, against a stub of the Go side.
const { launch } = require('./lib/page');
const { choose, shows } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => {
      const emit = window.__emit;
      const models = [['adaptive', 'Adaptive'], ['swe-1-6', 'SWE-1.6'], ['claude-opus-5-5-medium', 'Claude Opus 5.5 Medium'], ['gpt-6-sol-high', 'GPT-6 Sol High']];
      const mkOpts = (model) => [
        { id: 'mode', name: 'Session Mode', category: 'mode', type: 'select', currentValue: 'accept-edits', options: [{ value: 'accept-edits', name: 'Code' }, { value: 'ask', name: 'Ask' }, { value: 'plan', name: 'Plan' }] },
        { id: 'model', name: 'Model', category: 'model', type: 'select', currentValue: model, options: models.map(m => ({ value: m[0], name: m[1] })) }];
      const ws = 'C:\\Work\\Parser Workspace';
      const opts = {};
      // Devin replays a long history in many small chunks, as it does.
      async function replay(id) {
        for (let t = 0; t < 30; t++) {
          emit('agent:update', { sessionId: id, update: { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Request ' + t + ': make change number ' + t + '.' } } });
          for (let c = 0; c < 100; c++) emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'word' + c + ' ' + (c % 20 === 19 ? '\n\n' : '') } } });
          emit('agent:update', { sessionId: id, update: { sessionUpdate: 'tool_call', toolCallId: id + t, title: 'Ran command', status: 'completed', rawInput: { command: 'go vet ./...' } } });
          await new Promise(r => setTimeout(r, 5));
        }
        emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'THE LAST WORD' } } });
      }
      return {
        State: async () => ({ workspace: ws, recent: [ws], approvals: 'dinah', devin: 'devin.exe', catalog: mkOpts('swe-1-6'), defaults: { model: 'claude-opus-5-5-medium' }, pinned: ['claude-opus-5-5-medium', 'gpt-6-sol-high'], showThinking: false }),
        AgentProviders: async () => [],
        Sessions: async () => [{ sessionId: 'long', cwd: ws, title: 'A long session', updatedAt: '2026-10-07T07:00:00Z' }, { sessionId: 'locked', cwd: ws, title: 'Open in Devin Desktop', updatedAt: '2026-10-07T06:00:00Z' }],
        OpenSessionAt: async (workspace, id) => {
          // A test may hold the replay back until it has seen the page waiting.
          if (window.__hold) await window.__hold;
          await replay(id);
          if (id === 'locked') throw "Session 'locked' is already open in another process. Close the other instance before opening it here. (code -32015)";
          opts[id] = mkOpts('swe-1-6');
          emit('agent:options', { sessionId: id, options: opts[id] });
          return { sessionId: id, cwd: ws, title: 'A long session', open: true, working: false, queued: [] };
        },
        NewSessionAt: async () => { opts.n1 = mkOpts('claude-opus-5-5-medium'); setTimeout(() => emit('agent:options', { sessionId: 'n1', options: opts.n1 }), 0); return { sessionId: 'n1', cwd: ws, title: '', open: true, working: false, queued: [] }; },
        SetSessionOption: async (id, o, v) => { window.__calls.push(['SetSessionOption', id, o, v]); opts[id] = opts[id].map(x => x.id === o ? Object.assign({}, x, { currentValue: v }) : x); return opts[id]; },
        Send: async (id, text) => { window.__calls.push(['Send', id, text]); },
        Queue: async () => {}, Steer: async () => {}, Unqueue: async () => {}, Answer: async () => {}, Cancel: async () => {}, Stop: async () => {},
        SetWorkspace: async () => ({}), ForgetWorkspace: async () => ({}), ChooseWorkspace: async () => ({}), SetApprovals: async () => {},
        SetDefault: async () => ({}), SetPinned: async () => [], SetShowThinking: async () => {}, SaveHistory: async () => '', ClearHistory: async () => {}, SaveRun: async () => ''
      };
    },
    init: () => { window.__calls = []; },
  });
  const p = require('./lib/windows')(t.page);
  const { ok } = t;
  // No session yet: the bar shows what the next session starts with.
  // A command's control is drawn shown or hidden in the next frame; folded,
  // it is in its group's menu but still not hidden.
  const chat = () => p.locator('.win[data-chat-current]');
  const offered = (id, yes) => chat().locator('[data-command="' + id + '"]' + (yes ? ':not([hidden])' : '[hidden]')).first()
    .waitFor({ state: 'attached', timeout: 3000 }).then(() => true, () => false);
  ok('with no session the bar shows the mode', await offered('mode', true));
  ok('and the default model', await shows(chat(), 'model', 'Claude Opus 5.5 Medium'));
  await p.locator('.menubar-title').filter({ hasText: /^Actions$/ }).click();
  ok('Choose the Model is enabled', await p.locator('#menubar-panel').getByText('Choose the Model…', { exact: true }).isEnabled());
  await p.keyboard.press('Escape');
  await choose(p, chat(), 'model', 'GPT-6 Sol High');
  ok('choosing it with no session changes the bar', await shows(chat(), 'model', 'GPT-6 Sol High'));
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Start something.');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  await p.waitForFunction(() => window.__calls.some(c => c[0] === 'Send'), null, { timeout: 30000 }).catch(() => {});
  const calls = await p.evaluate(() => window.__calls);
  ok('the new session gets the model chosen before it existed, then the message', JSON.stringify(calls) === JSON.stringify([['SetSessionOption', 'n1', 'model', 'gpt-6-sol-high'], ['Send', 'n1', 'Start something.']]), JSON.stringify(calls));
  // A long session reopens fast and ends at the bottom. The replay is held
  // until the progress line has been seen, since on a busy machine a fixed
  // pause could end before the line shows or after the replay is done; the
  // time is taken from when the history starts to arrive.
  await p.evaluate(() => { window.__hold = new Promise(resolve => { window.__release = resolve; }); });
  await p.click('#sessions a:has-text("A long session")');
  ok('a progress line shows while the history arrives', await p.locator('.win[data-chat-current] [data-view-id=replay]').waitFor({ timeout: 30000 }).then(() => true, () => false));
  await t.shot('r-progress');
  const t0 = Date.now();
  await p.evaluate(() => { window.__hold = null; window.__release(); });
  await p.waitForFunction(() => document.querySelector('.win[data-chat-current] [data-view-id=replay]').hidden, null, { timeout: 30000 });
  await p.waitForTimeout(300);
  const ms = Date.now() - t0;
  ok('3,000 replayed parts open in under 3 seconds', ms < 3000, ms + ' ms');
  const atBottom = await p.evaluate(() => { const b = document.querySelector('.win[data-chat-current] [data-view-id=activity]'); return b.scrollHeight - b.scrollTop - b.clientHeight < 4; });
  ok('the view is left at the end', atBottom);
  ok('the last word is drawn', (await p.textContent('.win[data-chat-current] [data-view-id=logs]')).includes('THE LAST WORD'));
  ok('the session is usable', await offered('model', true) && (await p.textContent('.win[data-chat-current] [data-view-id=page-state]')).includes('waiting for you'));
  await t.shot('r-done');
  // A session Devin will not open.
  await p.click('#sessions a:has-text("Open in Devin Desktop")');
  await p.waitForFunction(() => document.querySelector('.win[data-chat-current] [data-view-id=replay]').hidden, null, { timeout: 30000 });
  await p.locator('.win[data-chat-current] [data-view-id=session-note]', { hasText: 'already open in another process' }).waitFor({ timeout: 30000 }).catch(() => {});
  ok('the refusal is shown over the prompt', await p.isVisible('.win[data-chat-current] [data-view-id=session-note]') && (await p.textContent('.win[data-chat-current] [data-view-id=session-note]')).includes('already open in another process'));
  ok('the replayed history stays readable', (await p.textContent('.win[data-chat-current] [data-view-id=logs]')).includes('THE LAST WORD'));
  ok('no model selector for a session that is not open', await offered('model', false));
  await p.locator('.menubar-title').filter({ hasText: /^Actions$/ }).click();
  ok('Choose the Model is greyed out', await p.locator('#menubar-panel').getByText('Choose the Model…', { exact: true }).isDisabled());
  await p.keyboard.press('Escape');
  await t.shot('r-locked');
  // A dialog taller than the window keeps its buttons in view, in a band at
  // its foot, while the content above them scrolls; PUDL marks it as having
  // more below, which draws the line above the buttons.
  const pinned = await p.evaluate(async () => {
    const d = document.createElement('dialog');
    d.className = 'dialog';
    d.innerHTML = '<h2 class="dialog-title">Long</h2>' + '<p class="dialog-body">A line of the form.</p>'.repeat(80) + '<div class="dialog-actions"><button class="btn">Cancel</button><button class="btn btn-primary">Save</button></div>';
    document.body.appendChild(d);
    d.showModal();
    // Opening focuses the first button, which scrolls to it; the content's
    // top is where a person starts reading.
    d.scrollTop = 0;
    d.dispatchEvent(new Event('scroll'));
    await new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
    const actions = d.querySelector('.dialog-actions').getBoundingClientRect(), box = d.getBoundingClientRect();
    const r = { scrolls: d.scrollHeight > d.clientHeight, inView: actions.bottom <= window.innerHeight && Math.abs(actions.bottom - box.bottom) < 2, more: d.hasAttribute('data-dialog-more-below') };
    d.close(); d.remove();
    return r;
  });
  ok('a long dialog keeps its buttons in a band at its foot while its content scrolls', pinned.scrolls && pinned.inView && pinned.more, JSON.stringify(pinned));
  await t.close();
})();
