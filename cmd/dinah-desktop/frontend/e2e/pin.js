// The pinned last prompt and the copy buttons.
const { launch } = require('./lib/page');
const windows = require('./lib/windows');
(async () => {
  const t = await launch({
    // Wide enough for both default docks and a chat between them.
    viewport: { width: 1920, height: 820 },
    state: { workspace: 'C:\\x', recent: ['C:\\x'], approvals: 'dinah', devin: 'd', defaultProvider: 'devin', titles: {} },
    app: () => {
      const ws = 'C:\\x';
      return {
        NewSessionAt: async () => ({ sessionId: 's1', cwd: ws, title: '', open: true, working: false, queued: [] }),
        Send: async () => { window.__emit('agent:session', { sessionId: 's1', cwd: ws, title: 'Long run', open: true, working: true, queued: [] }); },
      };
    },
    init: () => { window.__copied = []; window.runtime.ClipboardSetText = async (t) => { window.__copied.push(t); return true; }; },
  });
  const p = windows(t.page);
  const { ok } = t;
  try {
    const prompt = 'Draft the Note 14 design doc for the parser.\nUse the grammar rules from the meeting notes in parser/notes.\nKeep the summary short.\nFlag anything the sources do not settle as an open issue.';
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', prompt);
    await p.click('.win[data-chat-current] [data-view-id=send]');
    await p.locator('.win[data-chat-current] .msg-you').waitFor();
    ok('the pin is hidden while the message is in sight', await p.isHidden('.win[data-chat-current] [data-view-id=pin]'));
    await p.evaluate(() => {
      for (let i = 0; i < 40; i++) window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'tool_call', toolCallId: 't' + i, title: 'Read source ' + i, status: 'completed' } });
      window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: Array.from({ length: 30 }, (_, i) => 'Finding ' + (i + 1) + ' from the sources.').join('\n\n') } } });
      window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'tool_call', toolCallId: 'z', title: 'Wrote the draft', status: 'completed' } });
      window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Here is the draft summary:\n\n- **Requirement:** error recovery after a missing semicolon.' } } });
    });
    await p.waitForFunction(() => document.querySelector('.win[data-chat-current] [data-view-id=activity]').textContent.includes('error recovery after a missing semicolon.'));
    await p.locator('.win[data-chat-current] [data-view-id=activity]').hover();
    await p.mouse.wheel(0, 10000);
    // Rendering may still refine row heights under load. Establish the actual
    // viewport premise instead of assuming elapsed time made the prompt leave.
    await p.waitForFunction(() => {
      const win = document.querySelector('.win[data-chat-current]');
      const log = win.querySelector('[data-view-id=activity]');
      log.scrollTop = log.scrollHeight;
      const row = win.querySelector('.msg-you').closest('.activity-row');
      return row.getBoundingClientRect().height > 0 && row.getBoundingClientRect().bottom < log.getBoundingClientRect().top;
    });
    await p.locator('.win[data-chat-current] [data-view-id=pin-expand]').waitFor({ state: 'visible' });
    ok('the pin shows once the message has scrolled away', await p.isVisible('.win[data-chat-current] [data-view-id=pin]'));
    const clamp = await p.$eval('.win[data-chat-current] [data-view-id=pin-text]', el => el.scrollHeight > el.clientHeight);
    ok('it shows the first lines only', clamp);
    await t.shot('pin');
    await p.click('.win[data-chat-current] [data-view-id=pin-expand]');
    await p.waitForFunction(() => document.querySelector('.win[data-chat-current] [data-view-id=pin-expand]').getAttribute('aria-expanded') === 'true');
    ok('expanding shows the whole message', await p.$eval('.win[data-chat-current] [data-view-id=pin-text]', el => el.scrollHeight <= el.clientHeight + 1));
    await t.shot('pin-open');
    await p.click('.win[data-chat-current] [data-view-id=pin-copy]');
    await p.waitForFunction(() => window.__copied.length > 0);
    ok('the pin copies the message', (await p.evaluate(() => window.__copied.pop())) === prompt);
    await p.locator('.msg:not(.msg-you)').last().hover();
    await p.locator('.msg:not(.msg-you)').last().locator('.msg-head .msg-copy').click();
    await p.$eval('.win[data-chat-current] [data-view-id=activity]', el => { el.scrollTop = el.scrollHeight; });
    await p.waitForFunction(() => window.__copied.length > 0);
    ok('a reply copies as its Markdown', (await p.evaluate(() => window.__copied.pop())) === 'Here is the draft summary:\n\n- **Requirement:** error recovery after a missing semicolon.');
    await p.click('.win[data-chat-current] [data-view-id=pin-go]');
    await p.locator('.win[data-chat-current] [data-view-id=pin]').waitFor({ state: 'hidden' });
    ok('going to the message hides the pin', await p.isHidden('.win[data-chat-current] [data-view-id=pin]'));
    // Following the end: back in the history, new output leaves the reader
    // where they are and Jump to the Latest shows; at the end, a reply far
    // taller than the window keeps the end in view.
    const activity = '.win[data-chat-current] [data-view-id=activity]';
    const atEnd = () => p.$eval(activity, el => el.scrollHeight - el.scrollTop - el.clientHeight < 2);
    const longReply = n => p.evaluate(n => window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: Array.from({ length: 40 }, (_, i) => 'Point ' + n + '.' + (i + 1) + ' of a long reply.').join('\n\n') } } }), n);
    ok('Jump to the Latest shows back in the history', await p.isVisible('.win[data-chat-current] [data-view-id=jump-end]'));
    const before = await p.$eval(activity, el => el.scrollTop);
    await longReply(1);
    await p.waitForFunction(() => document.querySelector('.win[data-chat-current] [data-view-id=activity]').textContent.includes('Point 1.40 of a long reply.'));
    ok('new output leaves a reader in the history where they are', await p.$eval(activity, el => el.scrollTop) === before);
    await t.shot('jump-end');
    await p.evaluate(() => {
      window.__pinGeometry = []; window.__pinRecording = true; window.__pinPhase = 'before-jump';
      document.querySelector('.win[data-chat-current] [data-view-id=jump-end]').addEventListener('click', () => { __pinPhase = 'after-jump'; });
      function sample() {
        const win = document.querySelector('.win[data-chat-current]'), log = win.querySelector('[data-view-id=activity]');
        __pinGeometry.push({ phase: __pinPhase, top: log.scrollTop, extent: log.scrollHeight, height: log.clientHeight, following: log.dataset.following, preserved: log.hasAttribute('data-preserve-scroll-layout'), hidden: win.querySelector('[data-view-id=jump-end]').hidden });
        if (__pinRecording && __pinGeometry.length < 600) requestAnimationFrame(sample);
      }
      sample();
    });
    await p.click('.win[data-chat-current] [data-view-id=jump-end]');
    await p.waitForFunction(() => { const win = document.querySelector('.win[data-chat-current]'), el = win.querySelector('[data-view-id=activity]'); return el.scrollHeight - el.scrollTop - el.clientHeight < 2 && win.querySelector('[data-view-id=jump-end]').hidden; });
    const jumpPosition = await p.$eval(activity, log => ({ top: log.scrollTop, extent: log.scrollHeight, height: log.clientHeight, following: log.dataset.following }));
    ok('Jump to the Latest goes to the end and hides', jumpPosition.extent - jumpPosition.top - jumpPosition.height < 2 && await p.isHidden('.win[data-chat-current] [data-view-id=jump-end]'), JSON.stringify(jumpPosition));
    await p.evaluate(async () => { for (let i = 0; i < 30; i++) await new Promise(requestAnimationFrame); __pinRecording = false; });
    const frames = await p.evaluate(() => __pinGeometry.filter(frame => frame.phase === 'after-jump'));
    ok('every sampled frame after the jump remains at the end', frames.length > 2 && frames.every(frame => frame.extent - frame.top - frame.height < 2 && frame.following === 'true' && frame.hidden), JSON.stringify(frames));
    require('fs').writeFileSync(require('path').join(require('./lib/page').OUT, 'pin-jump-geometry.json'), JSON.stringify(await p.evaluate(() => __pinGeometry), null, 2));
    await longReply(2);
    await p.waitForFunction(() => document.querySelector('.win[data-chat-current] [data-view-id=activity]').textContent.includes('Point 2.40 of a long reply.'));
    await p.evaluate(() => window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'tool_call', toolCallId: 'after', title: 'Checked the draft', status: 'completed' } }));
    await longReply(3);
    await p.waitForFunction(() => { const el = document.querySelector('.win[data-chat-current] [data-view-id=activity]'); return el.textContent.includes('Point 3.40 of a long reply.') && el.scrollHeight - el.scrollTop - el.clientHeight < 2; });
    ok('at the end, replies taller than the window keep the end in view', await atEnd(), await p.$eval(activity, el => [el.scrollHeight, el.scrollTop, el.clientHeight].join(' ')));
  } finally { await t.close(); }
})();
