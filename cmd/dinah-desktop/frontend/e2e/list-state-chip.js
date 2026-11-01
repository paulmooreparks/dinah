// A chat row's state chip is the standard chip, as wide as its own glyph and
// word, at the same right edge in Chats and Workspace. A row shows its own
// chat's state only, never its team's, and nothing on the row moves as the
// state changes.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 950 }, app: () => {
    const cwd = 'C:/Fixture'; let version = 1;
    let state = { life: 'open', window: true, exec: { state: 'idle', why: '' } };
    const lead = { life: 'open', window: true, exec: { state: 'working', why: '' } };
    const team = { top: 'idle', counts: { idle: 2 } };
    const chats = () => [
      { sessionId: 'plain', title: 'Plain chat', cwd, provider: 'codex', open: true, chatState: state },
      { sessionId: 'lead', title: 'Team lead chat', cwd, provider: 'codex', open: true, chatState: lead, team }
    ];
    const states = () => ({ stateGen: 'chip', stateVer: version, chats: [{ sessionId: 'plain', ...state }, { sessionId: 'lead', ...lead }], groups: [] });
    window.__chipState = kind => {
      state = kind === 'closed' ? { life: 'closed', window: false } : { life: 'open', window: true, exec: kind === 'unknown' ? null : { state: kind, why: '' } };
      version++; __emit('agent:chatstate', states());
    };
    return { State: async () => ({ workspace: cwd, recent: [cwd] }), Sessions: async () => chats(), SessionsAt: async () => chats(),
      ChatStates: async () => states(), OpenSessionAt: async (_, id) => chats().find(c => c.sessionId === id),
      ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace,chat-browser' } }), SaveUIState: async () => {} };
  } });
  const { page: p, ok } = t;
  const words = { idle: 'Idle', unknown: 'Unknown', closed: 'Closed', working: 'Working' };
  const measure = () => p.evaluate(() => {
    const out = {};
    for (const [pane, host] of [['chats', '#sessions'], ['workspace', '#workspace-chats']]) {
      for (const id of ['plain', 'lead']) {
        const row = document.querySelector(host + ' [data-session-row="' + id + '"]');
        if (!row) continue;
        const line = row.querySelector('.browser-row-line'), slot = line.querySelector('.session-badge');
        const chip = slot.querySelector(':scope > .badge:not(.chat-state-reserve)'), link = row.querySelector('a[data-session]');
        const r = el => { const b = el.getBoundingClientRect(); return { x: b.x, right: b.right, width: b.width, y: b.y, height: b.height }; };
        out[pane + '/' + id] = { line: r(line), title: r(line.querySelector('.session-title')), slot: r(slot), chip: r(chip), link: r(link), word: chip.textContent,
          widest: Math.max(...[...slot.querySelectorAll('.chat-state-reserve')].map(c => c.getBoundingClientRect().width)),
          teamChips: row.querySelectorAll('.team-state-slot, .team-stuck-badge').length, chipLabel: chip.getAttribute('aria-label') };
      }
    }
    return out;
  });
  try {
    await p.locator('#sessions [data-session-row="plain"]').waitFor();
    await p.locator('#workspace-chats [data-session-row="plain"]').waitFor();
    let first = null;
    for (const state of ['idle', 'unknown', 'closed', 'working', 'idle']) {
      await p.evaluate(s => __chipState(s), state);
      await p.waitForFunction(w => [...document.querySelectorAll('[data-session-row="plain"] .session-badge > .badge:not(.chat-state-reserve)')].every(c => c.textContent === w), words[state]);
      await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
      const m = await measure();
      if (!first) first = m;
      for (const pane of ['chats', 'workspace']) {
        const d = m[pane + '/plain'], label = state + ' ' + pane;
        ok(label + ': the chip shows ' + words[state], d.word === words[state], JSON.stringify(d));
        ok(label + ': the chip is narrower than its slot unless it is the widest state', d.chip.width <= d.widest + 0.5 && d.chip.width < d.slot.width + 0.5, JSON.stringify(d));
        ok(label + ': the chip sits at the slot\'s right edge, and the slot at the line\'s', Math.abs(d.chip.right - d.slot.right) < 1 && Math.abs(d.slot.right - d.line.right) < 1, JSON.stringify(d));
        ok(label + ': the title and the slot hold their place', Math.abs(d.title.width - first[pane + '/plain'].title.width) < 0.5 && Math.abs(d.slot.x - first[pane + '/plain'].slot.x) < 0.5, JSON.stringify([d, first[pane + '/plain']]));
      }
      ok(state + ': both panes leave the same room after the chip', Math.abs((m['chats/plain'].link.right - m['chats/plain'].line.right) - (m['workspace/plain'].link.right - m['workspace/plain'].line.right)) < 1, JSON.stringify([m['chats/plain'], m['workspace/plain']]));
      for (const pane of ['chats', 'workspace']) {
        const d = m[pane + '/lead'];
        ok(state + ' ' + pane + ': the lead row shows its own Working chip and no team chip', d.word === 'Working' && d.teamChips === 0, JSON.stringify(d));
      }
      await t.shot('list-state-chip-' + state);
    }
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
