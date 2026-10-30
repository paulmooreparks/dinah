// The six parity features: @ mentions, paste and drop, edit and resend,
// message actions, folded tool runs, drafts per session.
const { launch, OUT } = require('./lib/page');
const path = require('path');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    state: { workspace: 'C:\\Work\\parser', recent: ['C:\\Work\\parser'], approvals: 'dinah', devin: 'd', defaultProvider: 'devin', titles: {} },
    app: () => {
      window.__calls = [];
      const ws = 'C:\\Work\\parser';
      const skills = [{ name: 'edit-request', description: 'Turn a request into an edit request.', hint: '["<document.md>"] <the change, in your words>', source: 'workspace' }];
      let n = 0;
      return {
        SkillsAt: async () => skills,
        NewSessionAt: async () => ({ sessionId: 's' + (++n), cwd: ws, title: '', open: true, working: false, queued: [] }),
        Send: async (id, text, doc, atts) => { window.__calls.push({ id, text, atts }); },
      };
    },
    init: () => { window.runtime.ClipboardSetText = async (t) => { (window.__copied = window.__copied || []).push(t); return true; }; },
  });
  const p = require('./lib/windows')(t.page);
  const { ok } = t;
  try {
    await p.waitForTimeout(500);
    const last = () => p.evaluate(() => window.__calls[window.__calls.length - 1]);

    // @ mentions.
    await p.click('.win[data-chat-current] [data-view-id=prompt]');
    await p.type('.win[data-chat-current] [data-view-id=prompt]', 'Compare @Edi');
    await p.waitForTimeout(150);
    ok('@ offers the tool windows', await p.isVisible('.win[data-chat-current] [data-view-id=slash-pop]') && (await p.textContent('.win[data-chat-current] [data-view-id=slash-pop]')).includes('Editor'));
    await t.shot('mention');
    await p.keyboard.press('Enter');
    await p.type('.win[data-chat-current] [data-view-id=prompt]', 'with @Fil');
    await p.keyboard.press('Tab');
    await p.waitForTimeout(100);
    const v = await p.inputValue('.win[data-chat-current] [data-view-id=prompt]');
    ok('mentions are written into the text', v === 'Compare @Editor with @Files ', v);
    await p.click('.win[data-chat-current] [data-view-id=send]');
    await p.waitForTimeout(300);
    let c = await last();
    const windows = (c.atts || []).map(a => a.kind + ':' + a.window);
    ok('each mention goes with the message', windows.join('|') === 'window:editor|window:files', JSON.stringify(windows));
    // A mention deleted from the text does not go.
    await p.type('.win[data-chat-current] [data-view-id=prompt]', 'Look at @Fil');
    await p.keyboard.press('Enter');
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Never mind the files.');
    await p.click('.win[data-chat-current] [data-view-id=send]');
    await p.waitForTimeout(200);
    c = await last();
    ok('a mention removed from the text is not sent', (c.atts || []).length === 0);

    // Paste and drop.
    await p.evaluate(async () => {
      const png = Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg=='), ch => ch.charCodeAt(0));
      const dt = new DataTransfer();
      dt.items.add(new File([png], 'screen.png', { type: 'image/png' }));
      document.querySelector('.win[data-chat-current] [data-view-id=prompt]').dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
      const dt2 = new DataTransfer();
      dt2.items.add(new File(['From: Dana\nSubject: retry interval\n\n60 seconds.'], 'answer.eml', { type: 'message/rfc822' }));
      const zone = document.querySelector('.chat-pane');
      zone.dispatchEvent(new DragEvent('dragover', { dataTransfer: dt2, bubbles: true, cancelable: true }));
      zone.dispatchEvent(new DragEvent('drop', { dataTransfer: dt2, bubbles: true, cancelable: true }));
    });
    await p.waitForTimeout(300);
    ok('a pasted image and a dropped file become chips', (await p.$$('.win[data-chat-current] [data-view-id=attachments] .attachment')).length === 2 && (await p.$$('.win[data-chat-current] [data-view-id=attachments] img')).length === 1);
    await t.shot('attach');
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'What does Dana say?');
    await p.click('.win[data-chat-current] [data-view-id=send]');
    await p.waitForTimeout(300);
    c = await last();
    ok('the files go with the message', (c.atts || []).length === 2 && c.atts[0].kind === 'file' && c.atts[0].mimeType === 'image/png' && c.atts[1].name === 'answer.eml', JSON.stringify((c.atts || []).map(a => a.name)));
    ok('and the chips clear', await p.isHidden('.win[data-chat-current] [data-view-id=attachments]'));

    // Edit and resend.
    await p.locator('.msg-you').last().hover();
    await p.locator('.msg-you').last().locator('.msg-head [data-act="edit"]').click();
    ok('Edit puts the message back in the box', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === 'What does Dana say?');
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', '');

    // Message actions on Devin's answer.
    await p.evaluate(() => window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Dana wants a retry interval of 60 seconds.' } } }));
    await p.waitForTimeout(100);
    await p.locator('.msg:not(.msg-you)').last().hover();
    await p.locator('.msg:not(.msg-you)').last().locator('.msg-head [data-act="quote"]').click();
    ok('Quote puts the answer in the box as a quotation', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === '> Dana wants a retry interval of 60 seconds.\n\n');
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', '');
    ok('an answer offers no edit request, which the Editor now makes', await p.locator('.msg:not(.msg-you)').last().locator('[data-act="request"]').count() === 0);

    // A long answer repeats its actions at its end, where it is finished. The
    // answer grows past the threshold chunk by chunk, so the foot appears on a
    // later draw of the same message.
    const answer = p.locator('.msg:not(.msg-you)').last();
    ok('a short answer has its actions at its head only', await answer.locator('.msg-foot').isHidden());
    const points = Array.from({ length: 24 }, (_, i) => '\n\nPoint ' + (i + 1) + ' of the long answer.');
    for (let i = 0; i < points.length; i += 6) {
      await p.evaluate(text => window.__emit('agent:update', { sessionId: 's1', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } } }), points.slice(i, i + 6).join(''));
      await p.waitForTimeout(60);
    }
    await p.waitForTimeout(150);
    const foot = answer.locator('.msg-foot');
    ok('a long answer repeats Quote and Copy at its end', await foot.isVisible() && await foot.locator('[data-act="quote"]').count() === 1 && await foot.locator('.msg-copy').count() === 1 &&
      await foot.evaluate(f => f.getBoundingClientRect().top > f.parentElement.querySelector('.msg-text').getBoundingClientRect().bottom - 1));
    ok('the repeated actions are a labelled group with no IDs', await foot.evaluate(f => f.querySelector('[role=group]').getAttribute('aria-label') === 'Actions at the end of the message' && !f.querySelector('[id]')));
    const whole = 'Dana wants a retry interval of 60 seconds.' + points.join('');
    await answer.hover();
    await p.evaluate(() => { window.__copied = []; });
    await answer.locator('.msg-head .msg-copy').click();
    await foot.locator('.msg-copy').click();
    await p.waitForTimeout(100);
    const copies = await p.evaluate(() => window.__copied.slice());
    ok('both Copy buttons copy the whole answer', copies.length === 2 && copies.every(t => t === whole.trim()), JSON.stringify(copies.map(t => t.length)));
    await foot.scrollIntoViewIfNeeded(); await p.locator('.win[data-chat-current]').screenshot({ path: path.join(OUT, 'long-answer-foot.png') });
    await foot.locator('[data-act="quote"]').focus();
    await p.keyboard.press('Enter');
    ok('Quote at the end works from the keyboard and quotes the whole answer', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')).includes('> Point 24 of the long answer.'));
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', '');

    // Folded tool runs.
    await p.evaluate(() => {
      const e = window.__emit;
      for (let i = 0; i < 5; i++) e('agent:update', { sessionId: 's1', update: { sessionUpdate: 'tool_call', toolCallId: 'r' + i, title: 'Read file ' + i, kind: 'read', status: 'completed' } });
      e('agent:update', { sessionId: 's1', update: { sessionUpdate: 'tool_call', toolCallId: 'x1', title: 'Ran go vet', kind: 'execute', status: 'failed' } });
      e('agent:update', { sessionId: 's1', update: { sessionUpdate: 'tool_call', toolCallId: 'x2', title: 'Ran go test', kind: 'execute', status: 'in_progress' } });
    });
    await p.waitForTimeout(150);
    const words = await p.textContent('.tool-group .tool-group-words');
    ok('a run of tool calls folds into one counted line', words === 'Read 5 files · Ran 2 commands · 1 failed', words);
    ok('the line names the step in hand', (await p.textContent('.tool-group .tool-group-now')) === 'now: Ran go test');
    ok('the calls are hidden until opened', await p.isHidden('.tool-group-list'));
    await t.shot('fold');
    await p.click('.tool-group .card-toggle');
    ok('opening shows them', await p.isVisible('.tool-group-list') && (await p.$$('.tool-group-list > li')).length === 7);
    await t.shot('fold-open');

    // Drafts per session.
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Half a thought for session one');
    await p.click('[data-cmd="new-session"].btn');
    await p.waitForTimeout(300);
    ok('a new session starts with an empty box', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === '');
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Something for session two');
    await p.evaluate(() => { const a = [...document.querySelectorAll('#sessions a')].find(x => x.dataset.session === 's1'); a.click(); });
    await p.waitForTimeout(200);
    ok('switching back brings back that session\'s draft', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === 'Half a thought for session one', await p.inputValue('.win[data-chat-current] [data-view-id=prompt]'));
    await p.evaluate(() => window.dinahWorkbench.open('chat'));
    await p.waitForTimeout(200);
    ok('and the new chat draft keeps its own', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === 'Something for session two');
  } finally { await t.close(); }
})();
