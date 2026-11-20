// The window assistant (assistant.js, judge_assistant.go; the approved mockup
// is docs/mockups/window-assistant): the tray's popover of one size, a
// request or a task read into what it would do, done on the real window
// layer only when Run is pressed and undone, the tasks as a list, and the
// Assistant Tasks window that changes the list.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      let workspace = 'C:/Alpha';
      window.__a = { asked: [], plan: null, tasks: [{ name: 'Tile the Windows That Are Showing', ask: 'tile the windows that are showing' }, { name: 'Tidy Up', ask: 'close what is finished' }] };
      const state = () => ({ workspace, recent: ['C:/Alpha'], devin: 'devin', approvals: 'devin', workspaceRecords: [] });
      // Each chat has ended its turn, which the engine says as an idle execution state.
      const info = (id, ws) => ({ sessionId: id, title: id, cwd: ws, open: true, working: false, queued: [], chatState: { life: 'open', window: true, exec: { state: 'idle', why: '' } } });
      return {
        State: async () => state(),
        Sessions: async () => ['Pricing Rules', 'Release Notes', 'Tax Question'].map(id => ({ ...info(id, 'C:/Alpha'), open: false })),
        OpenSessionAt: async (ws, id) => info(id, ws),
        AssistantTasks: async () => window.__a.tasks,
        SetAssistantTasks: async list => { window.__a.tasks = list.map(x => ({ name: x.name.trim(), ask: x.ask.trim() })); return window.__a.tasks; },
        RestoreAssistantTasks: async () => { window.__a.tasks = [{ name: 'Tile the Windows That Are Showing', ask: 'tile the windows that are showing' }]; return window.__a.tasks; },
        AskAssistant: async (ask, windows) => { window.__a.asked.push({ ask, windows }); return window.__a.plan(ask, windows); },
      };
    },
  });
  const { page: p, ok } = t;
  try {
    const front = () => p.locator('.win.active');
    const keys = {};
    await p.evaluate(() => dinahWorkbench.open('workspace'));
    for (const id of ['Pricing Rules', 'Release Notes', 'Tax Question']) {
      await p.evaluate(() => dinahWorkbench.open('workspace'));
      await p.locator('#workspace-chats a[data-session="' + id + '"]').click();
      await front().locator('.win-title').getByText(id, { exact: true }).waitFor();
      keys[id] = await front().getAttribute('data-win');
    }
    const layout = () => p.evaluate(() => { const s = pudlWindows.state(); return { open: s.open, min: s.min, place: s.place, top: s.top }; });
    const menu = p.locator('#assistant-menu');
    const said = () => p.locator('#assistant-said').textContent();
    const answered = async before => { await p.waitForFunction(b => { const x = document.getElementById('assistant-said').textContent; return x !== b && x !== 'Looking at the windows…'; }, before); };
    const run = p.locator('#assistant-run');

    // The tray item and its key open the popover, with the tasks as a list.
    await p.keyboard.press('Control+Space');
    await p.locator('#assistant-menu:popover-open').waitFor();
    ok('Ctrl+Space opens the assistant with the line to type in focused', await p.evaluate(() => document.activeElement.id) === 'assistant-line');
    await p.waitForFunction(() => document.querySelectorAll('#assistant-tasks .md-row').length === 2);
    const size0 = await menu.boundingBox();
    ok('the tasks are a list of rows', (await p.locator('#assistant-tasks .md-item').allTextContents()).join('|') === 'Tile the Windows That Are Showing|Tidy Up');
    ok('Run and Undo are in the foot, both disabled with nothing read or done', await p.locator('#assistant-menu .popover-actions #assistant-undo').isDisabled() && await p.locator('#assistant-menu .popover-actions #assistant-run').isDisabled());

    // A request to find: the engine is told every window, and its answer is done.
    await p.evaluate(k => { window.__a.plan = () => ({ do: 'gather', windows: [k], say: 'Found 1 window about pricing: Pricing Rules. The rest are put away, not closed.', judged: true }); }, keys['Pricing Rules']);
    const before = await layout();
    let was = await said();
    await p.locator('#assistant-line').fill('find all the windows for the pricing work');
    await p.keyboard.press('Enter');
    await answered(was);
    let now = await layout();
    ok('a typed request is read and says what it would do, and nothing has moved', /^Will show 1 window, tiled: Pricing Rules\. Will put the other 2 windows away, and close nothing\. Press Run to do it\.$/.test(await said()) && JSON.stringify(now) === JSON.stringify(before), await said());
    ok('Run is enabled and has the focus, so Enter again runs it', await run.isEnabled() && await p.evaluate(() => document.activeElement.id) === 'assistant-run');
    was = await said();
    await p.keyboard.press('Enter');
    await answered(was);
    const told = await p.evaluate(() => window.__a.asked[0]);
    ok('the engine is told what was asked and each window by title, kind and state', told.ask === 'find all the windows for the pricing work' && ['Pricing Rules', 'Release Notes', 'Tax Question'].every(n => told.windows.some(w => w.title === n && w.kind === 'chat')) && !told.windows.some(w => 'item' in w), JSON.stringify(told.windows));
    now = await layout();
    ok('the window found is showing and the others are put away, none closed', !now.min[keys['Pricing Rules']] && now.min[keys['Release Notes']] && now.min[keys['Tax Question']] && now.open.length === before.open.length, JSON.stringify(now.min));
    ok('the window found fills the layer', now.place[keys['Pricing Rules']].mode === 'maximized', JSON.stringify(now.place[keys['Pricing Rules']]));
    ok('the line says what was done', /Found 1 window about pricing/.test(await said()));
    ok('the popover is still open, Run is disabled again and Undo has the focus', await menu.evaluate(el => el.matches(':popover-open')) && await run.isDisabled() && await p.evaluate(() => document.activeElement.id) === 'assistant-undo');
    const size1 = await menu.boundingBox();
    ok('the popover is the size it was', size0.width === size1.width && size0.height === size1.height, JSON.stringify([size0, size1]));
    await t.shot('assistant');

    // Undo puts every window back.
    await p.locator('#assistant-undo').click();
    await p.waitForFunction(() => document.getElementById('assistant-said').textContent === 'Put back as it was.');
    now = await layout();
    ok('Undo shows the windows that were put away and returns the one that was moved', !now.min[keys['Release Notes']] && !now.min[keys['Tax Question']] && now.place[keys['Pricing Rules']].mode === before.place[keys['Pricing Rules']].mode && now.top === before.top, JSON.stringify([now.min, now.place[keys['Pricing Rules']], now.top, before.top]));
    ok('Undo is disabled again', await p.locator('#assistant-undo').isDisabled());

    // A press chooses a task and says what it would do; Run tiles what is showing.
    await p.evaluate(() => { window.__a.plan = () => ({ do: 'tile', windows: [], say: 'Tiled.' }); });
    was = await said();
    const unmoved = await layout();
    await p.locator('#assistant-tasks .md-item', { hasText: 'Tile the Windows' }).click();
    await answered(was);
    now = await layout();
    ok('pressing a task chooses it and says what it would do, and nothing has moved', /^Will tile the \d windows that are showing\. Press Run to do it\.$/.test(await said()) && JSON.stringify(now) === JSON.stringify(unmoved) && await p.locator('#assistant-tasks .md-item[aria-selected="true"]').textContent() === 'Tile the Windows That Are Showing', await said());
    was = await said();
    await run.click();
    await answered(was);
    now = await layout();
    const zones = Object.values(keys).map(k => now.place[k]);
    ok('a task asks its words and tiles the windows that are showing', (await p.evaluate(() => window.__a.asked[1].ask)) === 'tile the windows that are showing' && zones.every(z => z.mode === 'zone' || z.mode === 'left' || z.mode === 'right') && /Tiled the \d windows that are showing/.test(await said()), JSON.stringify(zones) + (await said()));

    // Tidying closes what is finished, never the window in front, and Undo opens it again.
    await p.evaluate(() => { window.__a.plan = () => ({ do: 'tidy', windows: [], say: 'Closed.' }); });
    const idle = await p.evaluate(() => window.dinahAssistant.desk());
    was = await said();
    await p.locator('#assistant-tasks .md-item', { hasText: 'Tidy Up' }).click();
    await answered(was);
    ok('tidying first names the windows it would close, and closes none', /^Will close 2 windows: /.test(await said()) && (await layout()).open.length === unmoved.open.length, await said());
    was = await said();
    await run.click();
    await answered(was);
    now = await layout();
    const idleKeys = idle.filter(w => w.state === 'idle').map(w => w.id);
    ok('the chats are known to be idle', idleKeys.length === 3, JSON.stringify(idle));
    ok('tidying closes the idle chats but the one in front', idleKeys.filter(k => !now.open.includes(k)).length === 2 && now.open.includes(now.top), (await said()) + JSON.stringify(now.open));
    await p.locator('#assistant-undo').click();
    await p.waitForFunction(() => /^Put back as it was/.test(document.getElementById('assistant-said').textContent));
    now = await layout();
    ok('Undo opens the closed chats again', idleKeys.every(k => now.open.includes(k)), (await said()) + JSON.stringify(now.open));

    // Edit Tasks opens a window where a task is added, and the list has it.
    await p.locator('#assistant-edit').click();
    const tasks = p.locator('.assistant-tasks-window');
    await tasks.waitFor();
    ok('Edit Tasks closes the popover and opens the Assistant Tasks window', !(await menu.evaluate(el => el.matches(':popover-open'))) && (await front().locator('.win-title').textContent()).trim() === 'Assistant Tasks');
    await tasks.getByRole('button', { name: 'New Task' }).click();
    await tasks.getByLabel('Name').fill('Pricing');
    await tasks.getByLabel('What the Assistant Is Asked').fill('find all the windows for the pricing work');
    await tasks.getByRole('button', { name: 'Save' }).click();
    await p.waitForFunction(() => window.__a.tasks.length === 3);
    ok('a new task is saved at the end of the list', (await p.evaluate(() => window.__a.tasks[2])).ask === 'find all the windows for the pricing work' && /Saved/.test(await tasks.locator('.assistant-tasks-status').textContent()));
    await tasks.getByRole('button', { name: 'Move Up' }).click();
    await p.waitForFunction(() => window.__a.tasks[1].name === 'Pricing');
    ok('Move Up reorders the list', true);
    ok('the Assistant Tasks window is not one of the windows the assistant is told of', !(await p.evaluate(() => window.dinahAssistant.desk())).some(w => w.title === 'Assistant Tasks'));
    await p.locator('#status-assistant').click();
    await p.locator('#assistant-menu:popover-open').waitFor();
    ok('the popover lists the new task in its place', (await p.locator('#assistant-tasks .md-item').allTextContents()).join('|') === 'Tile the Windows That Are Showing|Pricing|Tidy Up');
    const size2 = await menu.boundingBox();
    ok('the popover is the size it was with another task', size0.width === size2.width && size0.height === size2.height, JSON.stringify([size0, size2]));
    await p.keyboard.press('Escape');
    await tasks.getByRole('button', { name: 'Restore the Starter Set…' }).click();
    await p.locator('dialog[open]').getByRole('button', { name: 'Restore the Starter Set' }).click();
    await p.waitForFunction(() => window.__a.tasks.length === 1);
    ok('Restore the Starter Set asks first and then replaces the list', (await tasks.locator('.assistant-tasks-list .md-item').count()) === 1);
    await t.shot('assistant-tasks');
  } finally { await t.close(); }
})();
