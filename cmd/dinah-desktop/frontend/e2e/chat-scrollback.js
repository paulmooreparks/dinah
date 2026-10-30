// A long chat keeps its latest rows on the page and lets the oldest go, with
// a row at the top that says how many left and opens the chat's log; a very
// long tool output is cut on the page; a Markdown table in a message is
// drawn as a table. Against a stub of the Go side.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => {
      const ws = 'C:\\Work\\Parser Workspace';
      const opts = [{ id: 'model', name: 'Model', category: 'model', type: 'select', currentValue: 'swe-1-6', options: [{ value: 'swe-1-6', name: 'SWE-1.6' }] }];
      return {
        State: async () => ({ workspace: ws, recent: [ws], approvals: 'dinah', devin: 'devin.exe', catalog: opts, defaults: {}, pinned: [], showThinking: true }),
        AgentProviders: async () => [],
        NewSessionAt: async () => { setTimeout(() => window.__emit('agent:options', { sessionId: 'long', options: opts }), 0); return { sessionId: 'long', cwd: ws, title: '', open: true, working: false, queued: [] }; },
        SetSessionOption: async () => opts,
        Send: async (id, text) => { window.__calls.push(['Send', id, text]); },
        ShowChatLog: async (id) => { window.__calls.push(['ShowChatLog', id]); },
        Queue: async () => {}, Steer: async () => {}, Unqueue: async () => {}, Answer: async () => {}, Cancel: async () => {}, Stop: async () => {},
        SetWorkspace: async () => ({}), ForgetWorkspace: async () => ({}), ChooseWorkspace: async () => ({}), SetApprovals: async () => {},
        SetDefault: async () => ({}), SetPinned: async () => [], SetShowThinking: async () => {}, SaveHistory: async () => '', ClearHistory: async () => {}, SaveRun: async () => ''
      };
    },
    init: () => { window.__calls = []; },
  });
  const p = require('./lib/windows')(t.page);
  const { ok } = t;
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Start a long piece of work.');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  // The updates are for the session the message started, so they wait until
  // the page has it and has sent the message to it.
  await p.waitForFunction(() => window.__calls.some(c => c[0] === 'Send' && c[1] === 'long'), null, { timeout: 30000 });
  // Three hundred turns, each a thought, a tool call and a message: nine
  // hundred rows before any leave.
  await p.evaluate(async () => {
    const up = u => window.__emit('agent:update', { sessionId: 'long', update: u });
    for (let t = 0; t < 300; t++) {
      up({ sessionUpdate: 'agent_thought_chunk', content: { type: 'text', text: 'Thinking about step ' + t + '.' } });
      up({ sessionUpdate: 'tool_call', toolCallId: 'tc' + t, title: 'Read file ' + t, kind: 'read', status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'contents ' + t } }] });
      up({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Message number ' + t + '.' } });
      if (t % 50 === 0) await new Promise(r => setTimeout(r, 0));
    }
    up({ sessionUpdate: 'tool_call', toolCallId: 'huge', title: 'Read a huge file', kind: 'read', status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'x'.repeat(100 * 1024) } }] });
    // PowerShell 7's table, as Get-Process prints it: a green header, a
    // window title and a stray bell among the text.
    up({ sessionUpdate: 'tool_call', toolCallId: 'ansi', title: 'Ran command', kind: 'execute', status: 'completed', content: [{ type: 'content', content: { type: 'text', text: '\x1b]0;pwsh\x07\x1b[32;1mName        Id Path\x1b[0m\n\x1b[32;1m----        -- ----\x1b[0m\ndinah 5154424 C:\\bin\\dinah.exe\x07 & <tags>' } }] });
    up({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'THE LAST WORD' } });
  });
  const logs = '.win[data-chat-current] [data-view-id=logs]';
  // The rows are drawn in frames the render queue schedules, which a busy
  // machine spreads over seconds, so the checks wait until the last update
  // is on the page. A wait that times out leaves the checks to fail.
  const drawn = (sel, text) => p.waitForFunction(([sel, text]) => { const el = document.querySelector(sel); return !!el && el.textContent.includes(text); }, [sel, text], { timeout: 30000 }).catch(() => {});
  await drawn(logs, 'THE LAST WORD');
  // A tool row's output is drawn by the same queue, and can land after the message that followed it,
  // so what is read of the huge output and of the table waits for each to be on the page.
  const preDrawn = (start) => p.waitForFunction(([sel, start]) => Array.from(document.querySelector(sel).querySelectorAll('pre')).some(x => x.textContent.includes(start)), [logs, start], { timeout: 30000 }).catch(() => {});
  await preDrawn('xxxx');
  await preDrawn('dinah 5154424');
  await drawn(logs, 'The rest of this output is in the chat\'s log.');
  const rows = await p.evaluate(sel => document.querySelector(sel).querySelector('ol').children.length, logs);
  ok('the page keeps at most 600 rows of the chat', rows <= 600, String(rows));
  const marker = p.locator(logs + ' li.trimmed-row');
  ok('a row at the top says entries left the page', await marker.count() === 1 && /earlier entries of this chat are no longer shown here/.test(await marker.textContent()));
  ok('the marker is the first row', await p.evaluate(sel => document.querySelector(sel).querySelector('ol').firstElementChild.classList.contains('trimmed-row'), logs));
  const text = await p.textContent(logs);
  ok('the latest message is still there', text.includes('THE LAST WORD'));
  ok('the oldest messages are gone from the page', !text.includes('Message number 0.') && text.includes('Message number 299.'));
  const huge = await p.evaluate(sel => {
    const pres = Array.from(document.querySelector(sel).querySelectorAll('pre'));
    const pre = pres.find(x => x.textContent.startsWith('xxxx'));
    return pre ? pre.textContent.length : -1;
  }, logs);
  ok('a huge tool output is cut on the page', huge > 0 && huge < 40 * 1024, String(huge));
  ok('and says where the rest is', (await p.textContent(logs)).includes('The rest of this output is in the chat\'s log.'));
  const ansi = await p.evaluate(sel => {
    const pre = Array.from(document.querySelector(sel).querySelectorAll('pre')).find(x => x.textContent.includes('dinah 5154424'));
    return pre ? { text: pre.textContent, green: Array.from(pre.querySelectorAll('.ansi-fg2.ansi-b')).map(s => s.textContent) } : null;
  }, logs);
  ok('a command\'s terminal colours are drawn, and its other control codes left out', ansi && !/[\x00-\x08\x0b-\x1f]/.test(ansi.text) && ansi.green[0] === 'Name        Id Path' &&
    ansi.text === 'Name        Id Path\n----        -- ----\ndinah 5154424 C:\\bin\\dinah.exe & <tags>', JSON.stringify(ansi));
  await marker.locator('button', { hasText: 'Show the Chat Log' }).click();
  await p.waitForFunction(() => window.__calls.some(c => c[0] === 'ShowChatLog'), null, { timeout: 10000 }).catch(() => {});
  const calls = await p.evaluate(() => window.__calls);
  ok('Show the Chat Log asks for this chat\'s log', calls.some(c => c[0] === 'ShowChatLog' && c[1] === 'long'), JSON.stringify(calls));
  // More arrives: the count grows, and there is still one marker.
  await p.evaluate(() => { for (let t = 300; t < 500; t++) window.__emit('agent:update', { sessionId: 'long', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'More ' + t + '.' } } }) || window.__emit('agent:update', { sessionId: 'long', update: { sessionUpdate: 'tool_call', toolCallId: 'm' + t, title: 'Ran', status: 'completed' } }); });
  await drawn(logs, 'More 499.');
  ok('still one marker after more rows', await p.locator(logs + ' li.trimmed-row').count() === 1);
  ok('still at most 600 rows', await p.evaluate(sel => document.querySelector(sel).querySelector('ol').children.length, logs) <= 600);
  // Tables in messages. As GitHub's tables do, a pipe in a cell is escaped
  // with a backslash, inside a code span too.
  await p.evaluate(() => {
    window.__emit('agent:update', { sessionId: 'long', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'I would separate the stages.\n\n| Stage | What happens | Your involvement |\n|---|---|---|\n| Collect | Accept files | Add documents |\n| Validate | Check `a \\| b` evidence | None \\| rarely |\n\nAfter the table.' } } });
  });
  await drawn(logs, 'After the table.');
  const cells = await p.$$eval(logs + ' .msg-table td', tds => tds.map(t => t.textContent));
  ok('a table in a message is drawn as a table', (await p.$$(logs + ' .msg-table th')).length === 3 && cells.length === 6, JSON.stringify(cells));
  ok('pipes in code and escaped pipes stay in their cell', cells[4] === 'Check a | b evidence' && cells[5] === 'None | rarely', JSON.stringify(cells));
  ok('the text after the table is a paragraph', (await p.textContent(logs)).includes('After the table.'));
  await t.shot('chat-scrollback');
  await t.close();
})();
