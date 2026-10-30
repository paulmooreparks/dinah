// A chat whose agent has work running after its turn (background_tasks.go)
// shows a Background chip with the number of tasks, beside Working when a
// turn runs too; its state line says so and lists the tasks, each with a
// Stop button; and a turn the agent starts of its own shows as Working.
// Against a stub of the Go side.
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
  const state = (info, execution = 'idle') => p.evaluate(([i, execution]) => window.__emit('agent:session', Object.assign({ sessionId: 'bg', cwd: 'C:\\Work\\Parser Workspace', title: 'Builds', open: true, queued: [], chatState: { life: 'open', window: true, exec: { state: execution, why: execution === 'working' ? 'Turn in progress' : 'No work in progress' } } }, i)), [info, execution]);
  const chips = () => p.evaluate(w => {
    const el = document.querySelector(w), working = el.querySelector('.win-execution .win-state[data-state="working"]'), background = el.querySelector('.chat-background-chip');
    return { working: !!working && !working.hidden, background: background && !background.hidden ? background.textContent : '', line: el.querySelector('.win-execution .win-state').title, tasks: Array.from(el.querySelectorAll('.background-task')).map(li => li.textContent) };
  }, win);
  const build = { id: 'build', name: 'Build Windows', type: 'shell', description: 'go build ./...', state: 'running', canStop: true };
  const linux = { id: 'linux', name: 'Build Linux', type: 'shell', summary: 'linking', state: 'running', canStop: false };

  await state({ working: true, background: [build] }, 'working');
  await p.waitForTimeout(200);
  let c = await chips();
  ok('a turn with background work shows Working and Background 1 side by side', c.working && c.background === 'Background 1', JSON.stringify(c));
  // The chips take the title bar's own text colour, with no fill, so they
  // have the title's contrast in every theme, active or not.
  const inks = await p.evaluate(w => {
    const el = document.querySelector(w), title = getComputedStyle(el.querySelector('.win-title')).color;
    return ['.chat-background-chip'].map(sel => { const cs = getComputedStyle(el.querySelector(sel)); return { title, color: cs.color, bg: cs.backgroundColor, active: el.classList.contains('active') }; });
  }, win);
  ok('the background chip inherits the title colour without its own fill', inks.every(i => i.color === i.title && i.bg === 'rgba(0, 0, 0, 0)'), JSON.stringify(inks));
  ok('PUDL owns the execution mark beside the host background status', await p.evaluate(w => {
    const head = document.querySelector(w + ' .win-head'), group = head.querySelector(':scope > .win-statuses');
    return !!group && group.querySelector('.win-execution .win-state').dataset.state === 'working' && group.querySelector('.chat-background-chip').matches('.win-status:not(.strong)');
  }, win));
  ok('PUDL gives the window its Window status command', await p.evaluate(w => pudlWindows.menuCommands(document.querySelector(w).dataset.win).some(c => c.id === 'status'), win));

  await state({ working: false, background: [build, linux] });
  await p.waitForTimeout(200);
  c = await chips();
  ok('after the turn the chat still shows its background work, not Working', !c.working && c.background === 'Background 2', JSON.stringify(c));
  ok('the state line keeps the engine state and background task count separate', c.line.includes('Idle') && c.line.includes('2 background tasks are running.'), c.line);
  ok('each task is listed with what it is doing', c.tasks.length === 2 && c.tasks[0].includes('Build Windows') && c.tasks[0].includes('go build ./...') && c.tasks[1].includes('linking'), JSON.stringify(c.tasks));
  ok('Stop is offered only where the agent allows it', await p.locator(win + ' .background-task button').count() === 1);
  await p.locator(win + ' .background-task button', { hasText: 'Stop' }).click();
  await p.waitForTimeout(200);
  ok('Stop asks the agent to stop that task', await p.evaluate(() => JSON.stringify(window.__calls.find(x => x[0] === 'Stop')) === JSON.stringify(['Stop', 'bg', 'build'])));
  ok('background work does not replace the engine execution state in the taskbar', await p.evaluate(w => document.querySelector(w).dataset.winState === 'idle' && !document.querySelector(w).hasAttribute('data-win-running'), win));
  await t.shot('background-tasks');

  await state({ working: false, background: [linux, { id: 'autonomous-turn-1', name: 'Claude is working', type: 'turn', state: 'running', canStop: true }] }, 'working');
  await p.waitForTimeout(200);
  c = await chips();
  ok('a turn the agent starts of its own shows as Working, and is not counted as background', c.working && c.background === 'Background 1' && c.line.includes('Working'), JSON.stringify(c));

  await state({ working: false, background: [] });
  await p.waitForTimeout(200);
  c = await chips();
  ok('when the work ends, the chips and the list go', !c.working && !c.background && !c.tasks.length && c.line.includes('Idle') && !c.line.includes('background'), JSON.stringify(c));
  await t.close();
})();
