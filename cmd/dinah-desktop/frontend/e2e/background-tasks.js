// A chat whose agent has work running after its turn (background_tasks.go)
// shows it in its one state chip, Background, whose tooltip and state line
// count the tasks; with a turn running too it shows Working and says the
// background work continues. The title has no count chip of its own, so it
// never moves as the work starts and ends (Paul's ruling, 2 November 2026).
// The tasks are listed, each with a Stop button, and a turn the agent starts
// of its own shows as Working. Against a stub of the Go side.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => {
      const ws = 'C:\\Work\\Parser Workspace';
      const opts = [{ id: 'model', name: 'Model', category: 'model', type: 'select', currentValue: 'opus', options: [{ value: 'opus', name: 'Opus' }] }];
      return {
        State: async () => ({ workspace: ws, recent: [ws], approvals: 'dinah', devin: 'devin.exe', catalog: opts, defaults: {}, pinned: [], showThinking: true }),
        AgentProviders: async () => [],
        NewSessionAt: async () => { setTimeout(() => window.__emit('agent:options', { sessionId: 'bg', options: opts }), 0); return { sessionId: 'bg', cwd: ws, title: 'Builds', open: true, working: false, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } }, queued: [] }; },
        SetSessionOption: async () => opts,
        Send: async (id, text) => { window.__calls.push(['Send', id, text]); },
        StopBackgroundTask: async (id, task) => { window.__calls.push(['Stop', id, task]); },
        Queue: async () => {}, Steer: async () => {}, Unqueue: async () => {}, Answer: async () => {}, Cancel: async () => {}, Stop: async () => {},
        SetWorkspace: async () => ({}), ForgetWorkspace: async () => ({}), ChooseWorkspace: async () => ({}), SetApprovals: async () => {},
        SetDefault: async () => ({}), SetPinned: async () => [], SetShowThinking: async () => {}, SaveHistory: async () => '', ClearHistory: async () => {}, SaveRun: async () => ''
      };
    },
    init: () => { window.__calls = []; },
  });
  const p = require('./lib/windows')(t.page);
  const { ok } = t;
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Build it for Windows and Linux.');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  await p.waitForTimeout(300);
  const win = '.win[data-chat-current]';
  const state = (info, execution = 'idle') => p.evaluate(([i, execution]) => window.__emit('agent:session', Object.assign({ sessionId: 'bg', cwd: 'C:\\Work\\Parser Workspace', title: 'Builds', open: true, queued: [], chatState: { life: 'open', window: true, exec: typeof execution === 'object' ? execution : { state: execution, why: execution === 'working' ? 'Turn in progress' : 'No work in progress' } } }, i)), [info, execution]);
  const chips = () => p.evaluate(w => {
    const el = document.querySelector(w), working = el.querySelector('.chat-working-chip .chat-state-working'), group = el.querySelector('.win-head > .win-statuses'), title = el.querySelector('.win-title');
    const shown = [...group.querySelectorAll(':scope > .win-status')].filter(c => !c.hidden && c.getClientRects().length && c.textContent.trim());
    return { working: !!working && !working.hidden, chips: shown.map(c => c.className.replace('win-status ', '') + ':' + c.textContent.trim()), line: el.querySelector('.chat-working-chip').title, tasks: Array.from(el.querySelectorAll('.background-task')).map(li => li.textContent),
      geometry: [Math.round(title.getBoundingClientRect().width), Math.round(group.getBoundingClientRect().left), Math.round(group.getBoundingClientRect().width)] };
  }, win);
  const build = { id: 'build', name: 'Build Windows', type: 'shell', description: 'go build ./...', state: 'running', canStop: true };
  const linux = { id: 'linux', name: 'Build Linux', type: 'shell', summary: 'linking', state: 'running', canStop: false };

  // The engine's execution state counts the background work (exec.tasks,
  // docs/design/background-activity.md).
  await state({ working: true, background: [build] }, { state: 'working', why: 'Turn in progress', tasks: 1 });
  await p.waitForTimeout(200);
  let c = await chips();
  const geometry = [c.geometry];
  ok('a turn with background work shows one chip, Working, whose tooltip says the background work continues', c.working && c.chips.length === 1 && /^chat-working-chip/.test(c.chips[0]) && /Background work continues: 1 background task/.test(c.line), JSON.stringify(c));
  ok('the title has no count chip of its own', await p.evaluate(w => !document.querySelector(w + ' .chat-background-chip, ' + w + ' .chat-subagents-chip'), win));
  ok('PUDL gives the window its Window status command', await p.evaluate(w => pudlWindows.menuCommands(document.querySelector(w).dataset.win).some(c => c.id === 'status'), win));

  await state({ working: false, background: [build, linux] }, { state: 'background', why: '2 background tasks', tasks: 2 });
  await p.waitForTimeout(200);
  c = await chips();
  geometry.push(c.geometry);
  ok('after the turn the one chip says Background, not Working', !c.working && c.chips.length === 1 && /^chat-working-chip:Background /.test(c.chips[0]), JSON.stringify(c));
  ok('the state line says Background work with the count from the engine, once', c.line.startsWith('Background work') && (c.line.match(/2 background tasks/g) || []).length === 1 && !c.line.includes('Idle'), c.line);
  ok('each task is listed with what it is doing', c.tasks.length === 2 && c.tasks[0].includes('Build Windows') && c.tasks[0].includes('go build ./...') && c.tasks[1].includes('linking'), JSON.stringify(c.tasks));
  ok('Stop is offered only where the agent allows it', await p.locator(win + ' .background-task button').count() === 1);
  await p.locator(win + ' .background-task button', { hasText: 'Stop' }).click();
  await p.waitForTimeout(200);
  ok('Stop asks the agent to stop that task', await p.evaluate(() => JSON.stringify(window.__calls.find(x => x[0] === 'Stop')) === JSON.stringify(['Stop', 'bg', 'build'])));
  ok('the taskbar shows the PUDL working state labelled Background, with Background work in its detail', await p.evaluate(w => document.querySelector(w).dataset.winState === 'working' && document.querySelector(w).dataset.winStateLabel === 'Background' && /^Background work/.test(document.querySelector(w).dataset.winStateDetail) && !document.querySelector(w).hasAttribute('data-win-running'), win));
  await t.shot('background-tasks');

  await state({ working: false, background: [linux, { id: 'autonomous-turn-1', name: 'Claude is working', type: 'turn', state: 'running', canStop: true }] }, { state: 'working', why: "Agent's own turn in progress", tasks: 1 });
  await p.waitForTimeout(200);
  c = await chips();
  geometry.push(c.geometry);
  ok('a turn the agent starts of its own shows as Working, and is not counted as background', c.working && c.chips.length === 1 && c.line.includes('Working') && /1 background task/.test(c.line), JSON.stringify(c));

  await state({ working: false, background: [] });
  await p.waitForTimeout(200);
  c = await chips();
  geometry.push(c.geometry);
  ok('when the work ends, the chip says Idle and the list goes', !c.working && c.chips.length === 1 && /^chat-working-chip:Idle /.test(c.chips[0]) && !c.tasks.length && c.line.includes('Idle') && !c.line.includes('background'), JSON.stringify(c));
  ok('the title and its chip keep their place and width through Working, Background, Working and Idle', geometry.every(g => JSON.stringify(g) === JSON.stringify(geometry[0])), JSON.stringify(geometry));
  await t.close();
})();
