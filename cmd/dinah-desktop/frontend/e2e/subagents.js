// A chat's subagents (subagents.go, subagents.js): the row in the chat
// where the agent handed work to one, kept current; the Subagents chip; the
// Subagents menu button in the composer, absent until a subagent starts,
// with its count, its menu of subagents and Show All Subagents, the same
// submenu under Actions, and its folding in a narrow window; the
// tree of the chat's subagents with nesting, states and Stop where the
// agent allows it; one subagent's work, read-only and live; the
// disconnected state; a subagent's question in its chat; and the subagents
// of earlier runs, kept out of the menu, the tree and the counts and listed
// newest first in the Earlier Subagents window, reached from the Subagents
// menu, the tree and the chat's entry in the Chats pane. Against a stub of
// the Go side.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    state: { workspace: 'C:\\Work\\Parser Workspace', recent: ['C:\\Work\\Parser Workspace'], approvals: 'dinah', devin: 'devin.exe', catalog: [{ id: 'model', name: 'Model', category: 'model', type: 'select', currentValue: 'opus', options: [{ value: 'opus', name: 'Opus' }] }], showThinking: true },
    app: () => {
      const ws = 'C:\\Work\\Parser Workspace';
      const opts = [{ id: 'model', name: 'Model', category: 'model', type: 'select', currentValue: 'opus', options: [{ value: 'opus', name: 'Opus' }] }];
      return {
        NewSessionAt: async () => { setTimeout(() => window.__emit('agent:options', { sessionId: 'sa', options: opts }), 0); return { sessionId: 'sa', cwd: ws, title: 'Parser Hunt', open: true, working: false, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } }, queued: [] }; },
        SetSessionOption: async () => opts,
        Send: async (id, text) => { window.__calls.push(['Send', id, text]); },
        SubagentUpdates: async id => {
          window.__calls.push(['SubagentUpdates', id]);
          if (id !== 'sub-a') return { subagent: Object.values(window.__subs).find(s => s.id === id) || {}, seq: 0, updates: [] };
          return { subagent: Object.assign({}, window.__subs.a, { prompt: 'Find the parser and report the file it is in.' }), seq: 3, updates: [
            { sessionUpdate: 'agent_thought_chunk', content: { type: 'text', text: 'I will search for it.' } },
            { sessionUpdate: 'tool_call', toolCallId: 't1', title: 'Search for "func Parse"', kind: 'search', status: 'in_progress' },
            { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'The parser ' } }
          ] };
        },
        CancelSubagent: async id => { window.__calls.push(['CancelSubagent', id]); },
        Queue: async () => {}, Steer: async () => {}, Unqueue: async () => {}, Answer: async () => {}, Cancel: async () => {}, Stop: async () => {},
        SetWorkspace: async () => ({}), ForgetWorkspace: async () => ({}), ChooseWorkspace: async () => ({}), SetApprovals: async () => {},
        SetDefault: async () => ({}), SetPinned: async () => [], SetShowThinking: async () => {}, SaveHistory: async () => '', ClearHistory: async () => {}, SaveRun: async () => ''
      };
    },
    init: () => {
      window.__calls = [];
      const started = new Date(Date.now() - 65000).toISOString();
      window.__subs = {
        a: { id: 'sub-a', parentId: 'sa', rootId: 'sa', thisRun: true, name: 'Explore', task: 'Find where the parser lives', state: 'running', canCancel: false, started },
        b: { id: 'sub-b', parentId: 'sub-a', rootId: 'sa', thisRun: true, name: 'Verify', task: 'Run the parser tests', state: 'running', canCancel: true, started }
      };
    },
  });
  const { page: p, ok } = t;
  require('./lib/windows')(p);
  try {
  await p.waitForTimeout(500);
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Find the parser.');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  await p.waitForTimeout(300);
  const win = '.win[data-chat-current]';
  const emit = (n, d) => p.evaluate(([n, d]) => window.__emit(n, d), [n, d]);
  const sub = async (key, change) => { const s = await p.evaluate(([k, c]) => Object.assign(window.__subs[k], c || {}), [key, change]); await emit('agent:subagent', { sessionId: 'sa', subagent: s }); };
  await emit('agent:session', { sessionId: 'sa', cwd: 'C:\\Work\\Parser Workspace', title: 'Parser Hunt', open: true, queued: [], working: true, chatState: { life: 'open', window: true, exec: { state: 'working', why: 'Turn in progress' } }, background: [], subagents: [] });
  // A window wide enough for the composer's toolbar to show its groups open.
  const chatKey = await p.evaluate(w => document.querySelector(w).dataset.win, win);
  // The window stands clear of the docks at the page's sides.
  const chatWidth = w => p.evaluate(([k, w]) => { const el = document.querySelector('.win[data-win="' + k + '"]'); el.style.left = Math.round((innerWidth - w) / 2) + 'px'; el.style.width = w + 'px'; }, [chatKey, w]);
  await p.evaluate(k => window.pudlWindows.snap(k, 'floating'), chatKey);
  await chatWidth(800);
  await p.waitForTimeout(200);

  // The Subagents menu button in the composer's Turn group, and the same
  // menu under the window's Actions.
  const button = win + ' .composer-toolbar [data-command=subagents]';
  const control = () => p.evaluate(sel => { const b = document.querySelector(sel); return b ? { shown: !b.hidden && b.offsetParent !== null, text: b.textContent, name: b.getAttribute('aria-label'), group: b.closest('.toolbar-group').getAttribute('data-toolbar-group') } : null; }, button);
  const actionsRows = async () => {
    await p.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click();
    const rows = await p.locator('#menubar-panel .menu-action').allInnerTexts();
    await p.keyboard.press('Escape');
    return rows;
  };
  const menuOf = panel => p.evaluate(panel => Array.from(document.querySelector(panel).querySelectorAll('.menu-action, .menu-sep')).map(r => r.classList.contains('menu-sep') ? '-' : {
    label: r.querySelector('.menu-words').textContent, detail: (r.querySelector('.menu-row-detail') || {}).textContent || '',
    glyph: r.querySelector('.glyph') ? r.querySelector('.glyph').style.getPropertyValue('--glyph') : '', tone: r.getAttribute('data-tone') || '' }), panel);
  const openMenu = async () => { await p.click(button); const id = await p.getAttribute(button, 'popovertarget'); await p.waitForSelector('#' + id + ':popover-open'); return '#' + id; };
  let ctl = await control();
  ok('the Subagents button is absent while the chat has no subagents', ctl && !ctl.shown, JSON.stringify(ctl));
  let actions = await actionsRows();
  ok('so is the Subagents submenu under Actions', actions.length > 0 && !actions.some(a => /Subagents/.test(a)), JSON.stringify(actions));

  // The agent hands work to a subagent, which hands some to another.
  await sub('a');
  await emit('agent:update', { sessionId: 'sa', update: { sessionUpdate: 'subagent_spawned', subagentSessionId: 'sub-a', name: 'Explore', task: 'Find where the parser lives', capabilities: {} } });
  await p.waitForTimeout(200);
  ctl = await control();
  ok('the Subagents button appears in the Turn group with the count', ctl && ctl.shown && ctl.group === 'Turn' && ctl.text === 'Subagents1' && ctl.name === 'Subagents, 1', JSON.stringify(ctl));
  await sub('b');
  await p.waitForTimeout(300);
  ctl = await control();
  ok('its count follows the subagents as they start, and its name says it', ctl && ctl.text === 'Subagents2' && ctl.name === 'Subagents, 2', JSON.stringify(ctl));
  let panel = await openMenu();
  let menu = await menuOf(panel);
  ok('its menu lists each subagent with its state as a glyph and a word, then Show All Subagents', menu.length === 4 &&
    menu[0].label === 'Explore' && menu[0].detail === 'Running: Find where the parser lives' && menu[0].glyph === 'var(--glyph-diamond)' &&
    menu[1].label === 'Verify' && menu[1].detail === 'Running, under Explore: Run the parser tests' && menu[1].glyph === 'var(--glyph-diamond)' &&
    menu[2] === '-' && menu[3].label === 'Show All Subagents', JSON.stringify(menu));
  await p.locator(panel + ' .menu-action').filter({ hasText: 'Verify' }).click();
  await p.waitForSelector('.win[data-win-kind=subagent] [data-sa-name]');
  await p.waitForFunction(() => document.querySelector('.win[data-win-kind=subagent] [data-sa-name]').textContent === 'Verify');
  ok('choosing a subagent opens its window', await p.evaluate(() => { const w = document.querySelectorAll('.win[data-win-kind=subagent]'); return w.length === 1 && JSON.parse(window.dinahWorkbench.item(w[0].dataset.win).path)[1] === 'sub-b'; }));
  await p.evaluate(() => window.pudlWindows.close(document.querySelector('.win[data-win-kind=subagent]').dataset.win));
  await p.waitForFunction(() => !document.querySelector('.win[data-win-kind=subagent]'));
  // The keys: Enter opens the menu from the button, the arrows move along
  // its rows, and Escape closes it.
  await p.focus(button);
  await p.keyboard.press('Enter');
  await p.waitForSelector(panel + ':popover-open');
  await p.keyboard.press('ArrowDown');
  const focused = await p.evaluate(panel => { const a = document.activeElement; return a && a.closest(panel) ? a.textContent : ''; }, panel);
  await p.keyboard.press('Escape');
  ok('the menu works with the menu keys', /Explore|Verify/.test(focused) && !(await p.evaluate(panel => document.querySelector(panel).matches(':popover-open'), panel)), focused);
  actions = await actionsRows();
  ok('Actions has the Subagents submenu while the chat has subagents', actions.some(a => /^Subagents/.test(a)) && !actions.some(a => /Show All Subagents/.test(a)), JSON.stringify(actions));
  await p.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click();
  await p.locator('#menubar-panel .menu-action').filter({ hasText: /^Subagents/ }).click();
  await p.waitForSelector('#menubar-sub:popover-open');
  const subRows = await p.locator('#menubar-sub .menu-action').allInnerTexts();
  await p.keyboard.press('Escape');
  await p.keyboard.press('Escape');
  ok('its submenu lists the same subagents and Show All Subagents', subRows.length === 3 && subRows[0].startsWith('Explore\nRunning') && subRows[1].startsWith('Verify\nRunning') && subRows[2] === 'Show All Subagents', JSON.stringify(subRows));
  const chipControls = await p.evaluate(w => { const c = document.querySelector(w + ' .chat-subagents-chip'); return c ? c.querySelectorAll('button, a, input, [tabindex]').length : -1; }, win);
  ok('the title bar\'s Subagents chip only reports, as PUDL\'s status chips must, and the button beside the prompt is the action', chipControls === 0);
  const inline = () => p.evaluate(w => Array.from(document.querySelectorAll(w + ' .subagent-row')).map(li => ({ id: li.dataset.subagent, text: li.textContent, state: li.querySelector('.subagent-state') && li.querySelector('.subagent-state').textContent, open: !!li.querySelector('button.subagent-open') })), win);
  let rows = await inline();
  ok('the chat shows a row where the agent handed work to a subagent, with its name, task, state and Open', rows.length === 1 && rows[0].id === 'sub-a' && rows[0].text.includes('Explore') && rows[0].text.includes('Find where the parser lives') && rows[0].state === 'Running' && rows[0].open, JSON.stringify(rows));
  ok('the state is a glyph and a word', await p.evaluate(w => { const b = document.querySelector(w + ' .subagent-row .subagent-state'); return b.classList.contains('accent') && getComputedStyle(b, '::before').content !== 'none'; }, win));
  const chips = () => p.evaluate(w => { const el = document.querySelector(w), c = el.querySelector('.chat-subagents-chip'), wk = el.querySelector('.chat-working-chip .chat-state-working'); return { subagents: c && !c.hidden ? c.textContent : '', working: !!wk && !wk.hidden, line: el.querySelector('[data-view-id=page-state]').textContent }; }, win);
  let c = await chips();
  ok('the chat head shows Subagents 2 beside Working while they run', c.subagents === 'Subagents 2' && c.working, JSON.stringify(c));
  ok('the state line counts the running subagents', c.line.includes('2 subagents are running.'), c.line);
  ok('the nested subagent has no row in the chat itself', rows.every(r => r.id !== 'sub-b'));

  // The tree, from Show All Subagents in the Subagents menu.
  panel = await openMenu();
  await p.locator(panel + ' .menu-action').filter({ hasText: 'Show All Subagents' }).click();
  await p.waitForTimeout(400);
  const tree = () => p.evaluate(() => {
    const t = document.querySelector('.win[data-win-kind=subagents] .subagent-tree');
    if (!t) return null;
    const walk = ul => Array.from(ul.children).map(li => ({ id: li.dataset.subagent, state: li.querySelector('.subagent-state').textContent, title: li.querySelector('.subagent-state').title, name: li.querySelector('.subagent-name').textContent, task: li.querySelector('.subagent-task').textContent, ran: li.querySelector('.subagent-ran').textContent, stop: !!li.querySelector(':scope > .subagent-line .subagent-stop'), open: !!li.querySelector(':scope > .subagent-line .subagent-open'), kids: li.querySelector(':scope > ul') ? walk(li.querySelector(':scope > ul')) : [] }));
    return walk(t);
  });
  let tr = await tree();
  ok('Show All Subagents opens the tree with the nested subagent under its parent', tr && tr.length === 1 && tr[0].id === 'sub-a' && tr[0].kids.length === 1 && tr[0].kids[0].id === 'sub-b' && tr[0].kids[0].name === 'Verify', JSON.stringify(tr));
  ok('each node has its state, task, how long it has run and Open', tr && tr[0].state === 'Running' && tr[0].task === 'Find where the parser lives' && /^1 min [0-9]+ s$/.test(tr[0].ran) && tr[0].open && tr[0].kids[0].open, JSON.stringify(tr && tr[0]));
  ok('Stop is offered only where the agent allows it', tr && !tr[0].stop && tr[0].kids[0].stop);
  const before = tr && tr[0].ran;
  await p.waitForTimeout(1300);
  tr = await tree();
  ok('a running subagent\'s time ticks', tr && tr[0].ran !== before, before + ' -> ' + (tr && tr[0].ran));
  await p.click('.win[data-win-kind=subagents] li[data-subagent=sub-b] .subagent-stop');
  await p.waitForTimeout(200);
  ok('Stop cancels that subagent', await p.evaluate(() => JSON.stringify(window.__calls.find(x => x[0] === 'CancelSubagent')) === JSON.stringify(['CancelSubagent', 'sub-b'])));

  // One subagent's work, opened from the chat's row.
  await p.click(win + ' .subagent-row .subagent-open');
  await p.waitForTimeout(500);
  const one = '.win[data-win-kind=subagent]';
  const child = () => p.evaluate(one => {
    const w = document.querySelector(one);
    if (!w) return null;
    return { title: w.querySelector('.win-title').textContent, state: w.querySelector('[data-sa-state]').textContent, name: w.querySelector('[data-sa-name]').textContent, task: w.querySelector('[data-sa-task]').textContent,
      prompt: w.querySelector('[data-sa-prompt]').hidden ? '' : w.querySelector('[data-sa-prompt-text]').textContent, promptOpen: w.querySelector('[data-sa-prompt]').open,
      messages: Array.from(w.querySelectorAll('.msg-text')).map(m => m.innerHTML), thoughts: Array.from(w.querySelectorAll('.thought')).map(m => m.textContent),
      tools: Array.from(w.querySelectorAll('.tool-row')).map(r => r.querySelector('.tool-title').textContent + ' | ' + r.querySelector('.badge').textContent),
      nested: Array.from(w.querySelectorAll('.subagent-row')).map(r => r.textContent), note: w.querySelector('[data-sa-note]').hidden ? '' : w.querySelector('[data-sa-note]').textContent,
      inputs: w.querySelectorAll('textarea, input, [contenteditable=true], [data-view-id=prompt]').length };
  }, one);
  let k = await child();
  ok('Open shows the subagent\'s window with its name, task and state', k && k.name === 'Explore' && k.task === 'Find where the parser lives' && k.state === 'Running' && k.title.includes('Explore'), JSON.stringify(k));
  ok('the prompt is there, folded', k && k.prompt === 'Find the parser and report the file it is in.' && !k.promptOpen, JSON.stringify(k && k.prompt));
  ok('the kept work is drawn: thought, tool call and the message so far', k && k.thoughts[0] === 'I will search for it.' && k.tools[0] === 'Search for "func Parse" | running' && k.messages.length === 1 && k.messages[0].includes('The parser'), JSON.stringify(k));
  ok('the subagent\'s window offers no prompt box', k && k.inputs === 0, String(k && k.inputs));

  // Live updates: one the kept copy holds is passed over, the rest arrive.
  const live = (seq, update) => emit('agent:subagent-update', { rootId: 'sa', subagentId: 'sub-a', seq, update });
  await live(3, { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'The parser ' } });
  await live(4, { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'is in **parse.go**.' } });
  await live(5, { sessionUpdate: 'tool_call_update', toolCallId: 't1', status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'parse.go:12: func Parse(' } }] });
  await live(6, { sessionUpdate: 'subagent_spawned', subagentSessionId: 'sub-b', name: 'Verify', task: 'Run the parser tests', capabilities: { cancel: true } });
  await p.waitForTimeout(400);
  k = await child();
  ok('messages arrive live, drawn as Markdown, without repeating what was kept', k && k.messages.length === 1 && k.messages[0].includes('The parser is in <strong>parse.go</strong>.') && !k.messages[0].includes('The parser The parser'), JSON.stringify(k && k.messages));
  ok('tool calls change live', k && k.tools.length === 1 && k.tools[0] === 'Search for "func Parse" | done', JSON.stringify(k && k.tools));
  ok('a subagent of the subagent shows as a row in its window', k && k.nested.length === 1 && k.nested[0].includes('Verify') && k.nested[0].includes('Running'), JSON.stringify(k && k.nested));
  ok('the subagent\'s updates never reach the chat', await p.evaluate(w => !document.querySelector(w + ' .chat-instance').textContent.includes('func Parse('), win));
  await t.shot('subagents');

  // The window follows its output while it is at the end, and stays where
  // the person scrolled to otherwise.
  const tailEnd = () => p.evaluate(one => { const b = document.querySelector(one + ' [data-sa-body]'); return b.scrollTop + b.clientHeight >= b.scrollHeight - 24; }, one);
  for (let i = 0; i < 40; i++) await live(100 + i, { sessionUpdate: 'tool_call', toolCallId: 'tail' + i, title: 'Read file ' + i, status: 'completed' });
  await p.waitForTimeout(400);
  ok('a subagent window at its end follows new output', await tailEnd());
  await p.evaluate(one => { const b = document.querySelector(one + ' [data-sa-body]'); b.scrollTop = 0; b.dispatchEvent(new Event('scroll')); }, one);
  for (let i = 40; i < 50; i++) await live(100 + i, { sessionUpdate: 'tool_call', toolCallId: 'tail' + i, title: 'Read file ' + i, status: 'completed' });
  await p.waitForTimeout(400);
  ok('a subagent window scrolled back stays where it is', await p.evaluate(one => document.querySelector(one + ' [data-sa-body]').scrollTop === 0, one));
  await p.evaluate(one => { const b = document.querySelector(one + ' [data-sa-body]'); b.scrollTop = b.scrollHeight; b.dispatchEvent(new Event('scroll')); }, one);
  for (let i = 50; i < 60; i++) await live(100 + i, { sessionUpdate: 'tool_call', toolCallId: 'tail' + i, title: 'Read file ' + i, status: 'completed' });
  await p.waitForTimeout(400);
  ok('scrolling back to the end follows again', await tailEnd());

  // The ends: one completes, and the turn ends before the agent reports the other.
  await sub('b', { state: 'completed', ended: new Date().toISOString() });
  await sub('a', { state: 'disconnected', local: true, ended: new Date().toISOString() });
  await emit('agent:session', { sessionId: 'sa', cwd: 'C:\\Work\\Parser Workspace', title: 'Parser Hunt', open: true, queued: [], working: false, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } }, background: [], subagents: await p.evaluate(() => [window.__subs.a, window.__subs.b]) });
  await p.waitForTimeout(400);
  tr = await tree();
  ok('the tree shows each end as a glyph and a word', tr && tr[0].kids[0].state === 'Completed' && tr[0].state === 'Disconnected', JSON.stringify(tr));
  ok('disconnected says the outcome is unknown and is neither failed nor cancelled', tr && /unknown/.test(tr[0].title) && !/fail|cancel/i.test(tr[0].state), tr && tr[0].title);
  ok('ended subagents offer no Stop', tr && !tr[0].stop && !tr[0].kids[0].stop);
  ok('the disconnected state has a glyph of its own', await p.evaluate(() => { const b = document.querySelector('.win[data-win-kind=subagents] li[data-subagent=sub-a] .subagent-state'); return b.classList.contains('subagent-unknown') && !b.classList.contains('danger') && getComputedStyle(b, '::before').content !== 'none'; }));
  rows = await inline();
  ok('the chat\'s row follows the subagent to its end', rows[0].state === 'Disconnected', JSON.stringify(rows));
  c = await chips();
  ok('the chip goes when no subagent runs', !c.subagents && !c.line.includes('subagent'), JSON.stringify(c));
  ctl = await control();
  ok('the Subagents button stays while the ended subagents are listed', ctl && ctl.shown && ctl.text === 'Subagents2', JSON.stringify(ctl));
  panel = await openMenu();
  menu = await menuOf(panel);
  await p.keyboard.press('Escape');
  ok('its menu shows each end as a glyph and a word', menu[0].detail.startsWith('Disconnected') && menu[0].glyph === 'var(--glyph-ring)' &&
    menu[1].detail.startsWith('Completed') && menu[1].glyph === 'var(--glyph-circle)' && menu[1].tone === 'positive', JSON.stringify(menu));
  k = await child();
  ok('the subagent\'s window says its outcome is unknown', k && k.state === 'Disconnected' && /unknown/.test(k.note), JSON.stringify(k && k.note));
  const ran = await p.evaluate(() => document.querySelector('.win[data-win-kind=subagents] li[data-subagent=sub-a] .subagent-ran').textContent);
  await p.waitForTimeout(1200);
  ok('an ended subagent\'s time stops', ran === await p.evaluate(() => document.querySelector('.win[data-win-kind=subagents] li[data-subagent=sub-a] .subagent-ran').textContent), ran);
  await p.evaluate(() => window.pudlWindows.raise(document.querySelector('.win[data-win-kind=subagents]').dataset.win));
  await p.waitForTimeout(200);
  await t.shot('subagents-tree');

  // A subagent's question goes to its chat, saying which subagent asks.
  await emit('agent:permission', { sessionId: 'sa', key: 'q1', toolCall: { toolCallId: 'x1', title: 'Run go test ./parser' }, options: [{ optionId: 'allow', name: 'Allow', kind: 'allow_once' }], subagent: { id: 'sub-b', name: 'Verify' } });
  await p.waitForTimeout(300);
  const card = await p.evaluate(w => { const el = document.querySelector(w + ' [data-view-id=chat-approvals]'); return el ? el.textContent : ''; }, win);
  ok('a subagent\'s question is in its chat and says which subagent asks', card.includes('The subagent "Verify" wants to: Run go test ./parser'), card);

  // Another subagent starts, and fails.
  await p.evaluate(() => { window.__subs.c = { id: 'sub-c', parentId: 'sa', rootId: 'sa', thisRun: true, name: 'Lint', task: 'Lint the parser', state: 'running', canCancel: false, started: new Date().toISOString() }; });
  await sub('c');
  await p.waitForTimeout(200);
  ctl = await control();
  ok('a new subagent raises the count', ctl && ctl.text === 'Subagents3', JSON.stringify(ctl));
  await sub('c', { state: 'failed', ended: new Date().toISOString() });
  await p.waitForTimeout(200);
  panel = await openMenu();
  menu = await menuOf(panel);
  await p.keyboard.press('Escape');
  ok('and its row in the menu follows it to its end', menu.length === 5 && menu[2].label === 'Lint' && menu[2].detail === 'Failed: Lint the parser' && menu[2].glyph === 'var(--glyph-square)' && menu[2].tone === 'danger', JSON.stringify(menu));

  // A narrow window folds the Turn group, and its menu keeps the subagents.
  await p.evaluate(k => { window.pudlWindows.raise(k); window.pudlWindows.snap(k, 'floating'); }, chatKey);
  await chatWidth(230);
  const turn = win + ' .composer-toolbar [data-toolbar-group="Turn"]';
  const folded = await p.waitForFunction(t => Number(document.querySelector(t).getAttribute('data-toolbar-stage')) >= 3, turn).then(() => true, () => false);
  const stage = await p.getAttribute(turn, 'data-toolbar-stage');
  const foldButton = stage === '5' ? win + ' .composer-toolbar > .toolbar-more > .menu-btn' : turn + ' > .toolbar-fold > .menu-btn';
  await p.click(foldButton);
  const foldPanel = '#' + await p.getAttribute(foldButton, 'popovertarget');
  await p.waitForSelector(foldPanel + ':popover-open');
  const foldRows = await p.locator(foldPanel + ' .menu-action').allInnerTexts();
  ok('a narrow window folds the Turn group, and its menu keeps every subagent and Show All Subagents', folded && ['Explore', 'Verify', 'Lint', 'Show All Subagents'].every(l => foldRows.some(r => r.startsWith(l))), stage + ' ' + JSON.stringify(foldRows));
  await p.locator(foldPanel + ' .menu-action').filter({ hasText: 'Lint' }).click();
  await p.waitForFunction(() => Array.from(document.querySelectorAll('.win[data-win-kind=subagent]')).some(w => JSON.parse(window.dinahWorkbench.item(w.dataset.win).path)[1] === 'sub-c'));
  ok('a subagent chosen from the folded menu opens its window', true);

  // A long task is drawn on two lines at most wherever subagents are listed,
  // its whole text kept in the page and as the tooltip; the empty update for
  // the call that started it draws no row; and a running subagent is listed
  // first in the menu (28 October 2026).
  await chatWidth(800);
  await p.waitForTimeout(300);
  const longTask = 'Read the whole of the chat window and the subagent windows, and report every place where a subagent task is drawn, with the line numbers of each. '.repeat(4).trim();
  await p.evaluate(t => { window.__subs.d = { id: 'sub-d', parentId: 'sa', rootId: 'sa', thisRun: true, name: 'Survey', task: t, state: 'running', canCancel: false, started: new Date().toISOString() }; }, longTask);
  await sub('d');
  const rowsBefore = await p.evaluate(w => document.querySelectorAll(w + ' .chat-instance .activity-row').length, win);
  await emit('agent:update', { sessionId: 'sa', update: { sessionUpdate: 'subagent_spawned', subagentSessionId: 'sub-d', name: 'Survey', task: longTask, capabilities: {} } });
  await emit('agent:update', { sessionId: 'sa', update: { sessionUpdate: 'tool_call_update', toolCallId: 'agent-call-d' } });
  await p.waitForTimeout(300);
  const lines = el => { const cs = getComputedStyle(el); return Math.round(el.getBoundingClientRect().height / parseFloat(cs.lineHeight)); };
  const rowTask = await p.evaluate(([w, lines]) => { const f = new Function('return ' + lines)(); const t = document.querySelector(w + ' .subagent-row[data-subagent="sub-d"] .subagent-row-task'); return t && { lines: f(t), text: t.textContent, title: t.title }; }, [win, lines.toString()]);
  ok('a long task in the chat\'s row is drawn on two lines with its whole text kept and as its tooltip', rowTask && rowTask.lines === 2 && rowTask.text === longTask && rowTask.title === longTask, JSON.stringify(rowTask));
  const added = await p.evaluate(w => Array.from(document.querySelectorAll(w + ' .chat-instance .activity-row')).map(r => r.className), win);
  ok('the empty update for the call that started the subagent draws no "waiting" row', added.length === rowsBefore + 1 && !added.slice(rowsBefore).some(c => /tool-row/.test(c)) && !(await p.evaluate(w => Array.from(document.querySelectorAll(w + ' .tool-row .badge')).some(b => b.textContent === 'waiting'), win)), JSON.stringify(added.slice(rowsBefore)));
  panel = await openMenu();
  menu = await menuOf(panel);
  const detail = await p.evaluate(([panel, lines]) => { const f = new Function('return ' + lines)(); const d = Array.from(document.querySelectorAll(panel + ' .menu-row-detail')).find(x => x.textContent.includes('Read the whole')); return d && { lines: f(d), title: d.title, whole: d.textContent.endsWith('line numbers of each.') }; }, [panel, lines.toString()]);
  await p.keyboard.press('Escape');
  ok('the Subagents menu lists the running subagent first', menu[0].label === 'Survey' && menu[1].label === 'Explore', JSON.stringify(menu.map(m => m.label || m)));
  ok('the menu draws the task on two lines, its whole text the tooltip', detail && detail.lines === 2 && detail.whole && detail.title.endsWith(longTask), JSON.stringify(detail));
  await p.evaluate(() => window.pudlWindows.raise(document.querySelector('.win[data-win-kind=subagents]').dataset.win));
  await p.waitForTimeout(300);
  const treeTask = await p.evaluate(lines => { const f = new Function('return ' + lines)(); const t = document.querySelector('.win[data-win-kind=subagents] li[data-subagent=sub-d] .subagent-task'); return t && { lines: f(t), text: t.textContent, title: t.title }; }, lines.toString());
  ok('the tree draws it on two lines with its whole text kept and as its tooltip', treeTask && treeTask.lines === 2 && treeTask.text === longTask && treeTask.title === longTask, JSON.stringify(treeTask));
  await p.evaluate(k => window.pudlWindows.raise(k), chatKey);
  await p.waitForTimeout(200);
  await p.click(win + ' .subagent-row[data-subagent="sub-d"] .subagent-open');
  await p.waitForFunction(() => Array.from(document.querySelectorAll('.win[data-win-kind=subagent] [data-sa-name]')).some(n => n.textContent === 'Survey'));
  await p.waitForTimeout(200);
  const headTask = await p.evaluate(lines => { const f = new Function('return ' + lines)(); const w = Array.from(document.querySelectorAll('.win[data-win-kind=subagent]')).find(x => x.querySelector('[data-sa-name]').textContent === 'Survey'); const t = w.querySelector('[data-sa-task]'); return { lines: f(t), text: t.textContent, title: t.title }; }, lines.toString());
  ok('the subagent window\'s head draws it on two lines with its whole text kept and as its tooltip', headTask.lines === 2 && headTask.text === longTask && headTask.title === longTask, JSON.stringify(headTask));
  await t.shot('subagents-long-task');

  // Subagents from earlier runs (Paul, 28 October 2026). While the chat has
  // none, Earlier Subagents… is absent everywhere.
  const raiseKind = kind => p.evaluate(kind => window.pudlWindows.raise(document.querySelector('.win[data-win-kind=' + kind + ']').dataset.win), kind);
  const chatsPart = () => p.evaluate(() => Array.from(document.querySelectorAll('#sessions [data-earlier-subagents]')).map(a => ({ id: a.dataset.earlierSubagents, text: a.textContent, nested: a.closest('.browser-row').classList.contains('browser-row-part') })));
  await p.evaluate(() => dinahWorkbench.open('chat-browser'));
  await p.waitForTimeout(300);
  panel = await openMenu();
  menu = await menuOf(panel);
  await p.keyboard.press('Escape');
  await raiseKind('subagents');
  let foot = await p.evaluate(() => document.querySelector('.win[data-win-kind=subagents] [data-sa-foot]').hidden);
  let parts = await chatsPart();
  ok('Earlier Subagents is absent from the menu, the tree and the Chats pane while the chat has none',
    !menu.some(m => m.label === 'Earlier Subagents…') && foot && parts.length === 0 && await p.locator('#sessions [data-session="sa"]').count() === 1, JSON.stringify({ menu: menu.map(m => m.label || m), foot, parts }));

  // The chat's history brings back two subagents from an earlier run, the
  // second under the first; their rows stay in the chat.
  const earlierSub = s => emit('agent:subagent', { sessionId: 'sa', subagent: Object.assign({ rootId: 'sa', replayed: true, canCancel: false }, s) });
  await earlierSub({ id: 'old-1', parentId: 'sa', name: 'Archaeology', task: 'Dig through the old parser', state: 'completed' });
  await emit('agent:update', { sessionId: 'sa', update: { sessionUpdate: 'subagent_spawned', subagentSessionId: 'old-1', name: 'Archaeology', task: 'Dig through the old parser', capabilities: {} } });
  await earlierSub({ id: 'old-2', parentId: 'old-1', name: 'Survey Old', task: 'List what the old parser did', state: 'failed' });
  await p.waitForTimeout(400);
  ctl = await control();
  ok('earlier subagents do not change the count', ctl && ctl.text === 'Subagents4', JSON.stringify(ctl));
  c = await chips();
  ok('nor the title bar\'s count or the state line', c.subagents === 'Subagents 1' && c.line.includes('1 subagent is running.'), JSON.stringify(c));
  panel = await openMenu();
  menu = await menuOf(panel);
  await p.keyboard.press('Escape');
  ok('the Subagents menu lists only this run\'s subagents, with Earlier Subagents… at its foot',
    menu.filter(m => m !== '-').map(m => m.label).join('|') === 'Survey|Explore|Verify|Lint|Show All Subagents|Earlier Subagents…' && menu[menu.length - 2] === '-', JSON.stringify(menu.map(m => m.label || m)));
  await p.evaluate(k => window.pudlWindows.raise(k), chatKey);
  await p.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click();
  await p.locator('#menubar-panel .menu-action').filter({ hasText: /^Subagents/ }).click();
  await p.waitForSelector('#menubar-sub:popover-open');
  const subRows2 = await p.locator('#menubar-sub .menu-action').allInnerTexts();
  await p.keyboard.press('Escape');
  await p.keyboard.press('Escape');
  ok('so does the Subagents submenu under Actions', !subRows2.some(r => /Archaeology|Survey Old/.test(r)) && subRows2[subRows2.length - 1] === 'Earlier Subagents…' && subRows2.length === 6, JSON.stringify(subRows2));
  await raiseKind('subagents');
  await p.waitForTimeout(200);
  tr = await tree();
  const flat = n => n.reduce((all, x) => all.concat([x.id], flat(x.kids)), []);
  foot = await p.evaluate(() => document.querySelector('.win[data-win-kind=subagents] [data-sa-foot]').hidden);
  ok('the tree shows only this run\'s subagents, with Earlier Subagents… at its foot', tr && !flat(tr).some(id => /^old-/.test(id)) && flat(tr).length === 4 && !foot, JSON.stringify(tr && flat(tr)));
  rows = await inline();
  ok('the chat keeps the row of an earlier subagent from its history', rows.some(r => r.id === 'old-1' && r.state === 'Completed'), JSON.stringify(rows.map(r => r.id)));

  // The archive, from the tree's foot: newest first, with state, task and
  // when each ran, and its own address.
  await p.click('.win[data-win-kind=subagents] [data-sa-earlier]');
  await p.waitForSelector('.win[data-win-kind=subagent-archive] .subagent-node');
  const archive = () => p.evaluate(() => {
    const w = document.querySelector('.win[data-win-kind=subagent-archive]');
    return w && { key: w.dataset.win, title: w.querySelector('.win-title').textContent, meta: w.querySelector('[data-sa-meta]').textContent,
      rows: Array.from(w.querySelectorAll('.subagent-node')).map(li => ({ id: li.dataset.subagent, name: li.querySelector('.subagent-name').textContent, state: li.querySelector('.subagent-state').textContent,
        task: li.querySelector('.subagent-task').textContent, when: li.querySelector('time.subagent-ran').textContent, at: li.querySelector('time.subagent-ran').getAttribute('datetime') || '', open: !!li.querySelector('.subagent-open'), stop: !!li.querySelector('.subagent-stop') })) };
  });
  let arc = await archive();
  ok('Earlier Subagents in the tree opens the archive, newest first, with each one\'s state and task',
    arc && arc.rows.length === 2 && arc.rows[0].id === 'old-2' && arc.rows[0].name === 'Survey Old, under Archaeology' && arc.rows[0].state === 'Failed' && arc.rows[0].task === 'List what the old parser did' &&
    arc.rows[1].id === 'old-1' && arc.rows[1].state === 'Completed' && arc.rows[1].task === 'Dig through the old parser' && arc.rows.every(r => r.open && !r.stop), JSON.stringify(arc));
  ok('it says when each ran: the time of its row in the chat, or Earlier Run when the chat shows none', arc && arc.rows[1].at !== '' && arc.rows[1].when !== '' && arc.rows[0].at === '' && arc.rows[0].when === 'Earlier Run', JSON.stringify(arc && arc.rows));
  ok('it lists none of this run\'s subagents', arc && !arc.rows.some(r => /^sub-/.test(r.id)) && /2 subagents from earlier runs/.test(arc.meta), arc && arc.meta);
  ok('the archive has its own address', await p.evaluate(key => (new URLSearchParams(location.search).get('open') || '').split(',').includes(key), arc.key) && await p.evaluate(key => window.dinahWorkbench.item(key).kind === 'subagent-archive' && window.dinahWorkbench.item(key).path === 'sa', arc.key), arc && arc.key);
  const archiveTask = await p.evaluate(lines => { const f = new Function('return ' + lines)(); const t = document.querySelector('.win[data-win-kind=subagent-archive] .subagent-task'); return f(t) <= 2; }, lines.toString());
  ok('a task in the archive is drawn on two lines at most', archiveTask);
  await t.shot('subagents-archive');

  // Opening one from the archive opens its window read-only.
  await p.click('.win[data-win-kind=subagent-archive] li[data-subagent="old-2"] .subagent-open');
  await p.waitForFunction(() => Array.from(document.querySelectorAll('.win[data-win-kind=subagent]')).some(w => JSON.parse(window.dinahWorkbench.item(w.dataset.win).path)[1] === 'old-2'));
  const oldWin = await p.evaluate(() => { const w = Array.from(document.querySelectorAll('.win[data-win-kind=subagent]')).find(x => JSON.parse(window.dinahWorkbench.item(x.dataset.win).path)[1] === 'old-2'); return { inputs: w.querySelectorAll('textarea, input, [contenteditable=true], [data-view-id=prompt]').length, stop: !!w.querySelector('.subagent-stop') }; });
  ok('opening an earlier subagent shows its window read-only', oldWin.inputs === 0 && !oldWin.stop, JSON.stringify(oldWin));
  await p.evaluate(key => window.pudlWindows.close(key), arc.key);
  await p.waitForFunction(() => !document.querySelector('.win[data-win-kind=subagent-archive]'));

  // The archive from the foot of the Subagents menu.
  await p.evaluate(k => window.pudlWindows.raise(k), chatKey);
  panel = await openMenu();
  await p.locator(panel + ' .menu-action').filter({ hasText: 'Earlier Subagents…' }).click();
  await p.waitForSelector('.win[data-win-kind=subagent-archive] .subagent-node');
  arc = await archive();
  ok('Earlier Subagents… in the Subagents menu opens the same archive', arc && arc.rows.map(r => r.id).join() === 'old-2,old-1', JSON.stringify(arc && arc.rows.map(r => r.id)));
  const archiveKey = arc.key;
  await p.evaluate(key => window.pudlWindows.close(key), archiveKey);
  await p.waitForFunction(() => !document.querySelector('.win[data-win-kind=subagent-archive]'));

  // The archive from the chat's entry in the Chats pane, where old chats
  // are found.
  await p.evaluate(() => dinahWorkbench.open('chat-browser'));
  await p.waitForTimeout(300);
  parts = await chatsPart();
  ok('the chat\'s entry in the Chats pane has a Subagents part for its earlier subagents', parts.length === 1 && parts[0].id === 'sa' && /Subagents/.test(parts[0].text) && /2 from earlier runs/.test(parts[0].text) && parts[0].nested, JSON.stringify(parts));
  ok('the part sits right under the chat\'s own entry and is not counted as a chat', await p.evaluate(() => { const part = document.querySelector('#sessions [data-earlier-subagents="sa"]').closest('.browser-row'); const prev = part.previousElementSibling; const group = part.closest('.browser-group'); return !!prev && prev.dataset.sessionRow === 'sa' && group.querySelector('.browser-group-count').textContent === String(group.querySelectorAll('[data-session-row]').length); }));
  await p.click('#sessions [data-earlier-subagents="sa"]');
  await p.waitForSelector('.win[data-win-kind=subagent-archive] .subagent-node');
  arc = await archive();
  ok('pressing it opens the archive, at the same address', arc && arc.key === archiveKey && arc.rows.map(r => r.id).join() === 'old-2,old-1', JSON.stringify(arc && { key: arc.key, rows: arc.rows.map(r => r.id) }));
  } finally { await t.close(); }
})();
