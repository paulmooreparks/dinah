// Member rows and the taskbar's chat rows. Each row in a team group's
// Members dropdown and in its lead's Members dropdown has the chat glyph the
// taskbar draws for a chat window, before the name, 8px from it as in the
// taskbar's stack rows, with the type and trailing-control inset those rows
// have. The rows keep the standard state chip, Background included, the
// group's … menu, the lead's direct-open rows and Close in the stack. The
// Group Chats taskbar stack has the Members bust glyph.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 900, height: 900 }, app: () => {
    const cwd = 'C:/Fixture';
    const st = { lead: 'idle', alpha: 'working', beta: 'background' };
    const why = { idle: '', working: 'Turn in progress', background: '1 background task: Run tests' };
    const exec = id => ({ state: st[id], why: why[st[id]], ...(st[id] === 'background' ? { tasks: 1 } : {}) });
    const ref = (id, title) => ({ id: 'bus-' + id, chat: id, title, life: 'open', window: true, exec: exec(id) });
    const room = name => ({ name, title: name === 'team-a' ? 'Team A' : 'Team B', lead: 'bus-lead', members: [ref('lead', 'Team Lead'), ref('alpha', 'Alpha'), ref('beta', 'Beta')] });
    const team = () => ({ lead: { chat: 'lead', bus: 'bus-lead', title: 'Team Lead' }, group: 'team-a', stateGen: 'g', stateVer: 2, counts: { top: 'working', working: 1, background: 1 },
      members: [{ chat: 'alpha', bus: 'bus-alpha', title: 'Alpha', life: 'open', window: true, exec: exec('alpha') }, { chat: 'beta', bus: 'bus-beta', title: 'Beta', life: 'open', window: true, exec: exec('beta') }], budget: { rows: [] } });
    const info = id => ({ sessionId: id, title: id === 'lead' ? 'Team Lead' : id[0].toUpperCase() + id.slice(1), cwd, provider: 'codex', open: true, working: st[id] === 'working', chatState: { life: 'open', window: true, exec: exec(id) } });
    return { State: async () => ({ workspace: cwd, recent: [cwd], workspaceId: 'w1', workspaceRecords: [{ id: 'w1', name: 'Fixture', executionPath: cwd, sources: [] }] }),
      Sessions: async () => ['lead', 'alpha', 'beta'].map(info), SessionsAt: async () => ['lead', 'alpha', 'beta'].map(info),
      ReadUIState: async () => ({ version: 1, values: {} }), SaveUIState: async () => {},
      OpenSessionAt: async (w, id) => info(id), ChatLead: async () => null, Team: async id => id === 'lead' ? team() : null,
      ChatStates: async () => ({ chats: ['lead', 'alpha', 'beta'].map(id => ({ sessionId: id, bus: 'bus-' + id, life: 'open', window: true, exec: exec(id) })), groups: [], stateGen: 's', stateVer: 1 }),
      BusOverview: async () => ({ inbox: [], rooms: [room('team-a'), room('team-b')], workspaces: [], links: [] }), BusResolveGroup: async key => ({ state: 'resolved', room: room(key) }),
      BusGroupMessages: async () => [], BusMarkGroupRead: async () => {} };
  } });
  const { page: p, ok } = t;
  const measure = sel => p.evaluate(sel => {
    const panel = document.querySelector(sel), pr = panel.getBoundingClientRect();
    return [...panel.querySelectorAll('.win-stack-row, .member-row')].filter(r => r.getClientRects().length).map(r => {
      const name = r.querySelector('.win-item-title, .member-name'), glyph = r.querySelector('.win-item-glyph, .member-glyph'), action = r.querySelector('.win-stack-close, .member-menu-btn'), chip = r.querySelector('.badge.chat-state:not(.chat-state-reserve)');
      return { name: name.textContent.trim(), font: getComputedStyle(name).fontSize, glyph: glyph ? glyph.style.getPropertyValue('--glyph').trim() : '', gap: glyph ? Math.round(name.getBoundingClientRect().left - glyph.getBoundingClientRect().right) : null,
        inset: action ? Math.round(pr.right - action.getBoundingClientRect().right) : null, chip: chip ? chip.textContent.trim() + '|' + chip.className : '' };
    });
  }, sel);
  try {
    for (const id of ['alpha', 'beta', 'lead']) await p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', path: id, workspace: 'w1' }), id);
    await p.evaluate(() => dinahBus.openGroup('team-b', { message: 0 }));
    await p.evaluate(() => dinahBus.openGroup('team-a', { message: 0 }));
    // The real layer stacks only when the taskbar is full; this test asks
    // PUDL to stack always, which draws the same popup.
    await p.evaluate(() => document.querySelector('[data-win-layer]').setAttribute('data-win-taskbar-stacks', 'always'));
    const chatStack = p.locator('[data-win-stack]').filter({ hasText: /Chats/ }).first();
    await chatStack.waitFor();
    const groupStack = await p.evaluate(() => { const s = [...document.querySelectorAll('[data-win-stack]')].find(s => /Group Chats/.test(s.textContent)); return s ? s.querySelector('.glyph').style.getPropertyValue('--glyph') : ''; });
    ok('the Group Chats taskbar stack has the Members bust glyph', /--glyph-account/.test(groupStack), groupStack);
    ok('each group chat window keeps its own chat glyph', await p.evaluate(() => [...document.querySelectorAll('.win[data-win-kind="bus-room"]')].every(w => w.getAttribute('data-win-glyph') === 'comments')));
    await chatStack.click();
    const stackPanel = '.win-stack-panel:popover-open';
    await p.locator(stackPanel).waitFor();
    const stack = (await measure(stackPanel)).filter(r => r.name !== 'New Chat');
    ok('the taskbar row for background work reads Background, as the chips do', await p.evaluate(() => [...document.querySelectorAll('.win-stack-panel:popover-open .win-stack-row')].some(r => /Beta/.test(r.textContent) && r.querySelector('.win-state-words').textContent === 'Background')));
    ok('the taskbar chat rows keep the chat glyph, words and Close', stack.length === 3 && stack.every(r => /--glyph-comments/.test(r.glyph) && r.inset !== null), JSON.stringify(stack));
    await t.shot('rows-stack', { clip: await p.locator(stackPanel).boundingBox() });
    await p.keyboard.press('Escape');
    // The group's Members dropdown.
    const group = p.locator('.win.active[data-win-kind="bus-room"]');
    await p.evaluate(() => dinahBus.openGroup('team-a', { message: 0 }));
    await group.getByRole('button', { name: /^Members/ }).click();
    const groupPanel = '.win.active[data-win-kind="bus-room"] .members-menu:popover-open';
    await p.locator(groupPanel).waitFor();
    const g = await measure(groupPanel);
    ok('group Members: each row has the chat glyph 8px before the name, as the taskbar rows do', g.length === 3 && g.every(r => /--glyph-comments/.test(r.glyph) && r.gap === stack[0].gap), JSON.stringify({ g, stackGap: stack[0].gap }));
    ok('group Members: the same type size and trailing inset as the taskbar rows', g.every(r => r.font === stack[0].font && r.inset === stack[0].inset), JSON.stringify({ g, stack: stack[0] }));
    ok('group Members: the standard chips, Background included', /^Working\|/.test(g[1].chip) && /^Background\|.*chat-state-background/.test(g[2].chip), JSON.stringify(g.map(r => r.chip)));
    await t.shot('rows-group', { clip: await p.locator(groupPanel).boundingBox() });
    await p.locator(groupPanel + ' [data-group-member="bus-alpha"] .member-menu-btn').click();
    const items = await p.locator('.group-member-menu:popover-open .menu-action').allTextContents();
    ok('group Members: the … menu keeps its commands', items.includes('Open Chat') && items.includes('Address Message'), JSON.stringify(items));
    await p.keyboard.press('Escape'); await p.keyboard.press('Escape');
    // The lead's Members dropdown.
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', path: 'lead', workspace: 'w1' }));
    const lead = p.locator('.win[data-chat-current] .chat-team-toolbar').getByRole('button', { name: /^Members/ });
    await lead.first().click();
    const leadPanel = '.members-menu:popover-open';
    await p.locator(leadPanel).waitFor();
    const l = await measure(leadPanel);
    ok('lead Members: each row has the chat glyph 8px before the name and the standard chip', l.length === 2 && l.every(r => /--glyph-comments/.test(r.glyph) && r.gap === stack[0].gap && r.font === stack[0].font) && /^Background\|/.test(l[1].chip), JSON.stringify(l));
    await t.shot('rows-lead', { clip: await p.locator(leadPanel).boundingBox() });
    const opened = await p.evaluate(() => { const b = document.querySelector('.members-menu:popover-open .member-open'); b.focus(); return document.activeElement === b && !document.querySelector('.members-menu:popover-open .member-menu-btn'); });
    ok('lead Members: rows open their chat directly, with no … button, and take focus', opened);
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
