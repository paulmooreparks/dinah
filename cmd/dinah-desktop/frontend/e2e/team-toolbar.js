// A lead's chat opens its team group with Open Team; its Team list shows
// each member's name and standard state chip alone and opens the member's
// chat; the team group's window has Team Report, with the members in
// detail and the allowances, and Team Allowances, which no other group
// shows; and the report is reached from the folded toolbar too.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const cwd = 'C:/Fixture';
    const ref = (id, title, extra) => Object.assign({ id: 'bus-' + id, chat: id, title, life: 'open', window: true, exec: { state: 'idle', why: '' } }, extra || {});
    const rooms = [
      { name: 'team-a', title: 'Team A', lead: 'bus-lead', members: [ref('lead', 'Lead'), ref('alpha', 'Alpha'), ref('beta', 'Beta')] },
      { name: 'plain', title: 'Plain Group', members: [ref('alpha', 'Alpha'), ref('beta', 'Beta')] }];
    let alphaState = 'working', ver = 1;
    window.__calls = [];
    const team = () => ({ lead: { chat: 'lead', bus: 'bus-lead', title: 'Lead' }, group: 'team-a', stateGen: 'g', stateVer: ++ver, counts: { top: alphaState, [alphaState]: 1, idle: 1 },
      members: [
        { chat: 'alpha', bus: 'bus-alpha', title: 'Alpha', life: 'open', window: true, exec: { state: alphaState, why: 'Writing the parser' }, workspace: 'Parser', provider: 'codex', model: 'gpt-5', usage: { used: 1234, unit: 'requests' }, task: 'Build the lexer first', taskMessage: 7 },
        { chat: 'beta', bus: 'bus-beta', title: 'Beta', life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' }, workspace: 'Docs', provider: 'claude', model: 'opus', usage: { used: 2, unit: 'turns' } }],
      budget: { periodStart: '2026-11-01T00:00:00Z', periodEnd: '2026-11-08T00:00:00Z', providers: [{ provider: 'codex', name: 'Codex' }], unmeasured: [],
        rows: [{ provider: 'codex', name: 'Codex', used: 1234, unit: 'requests', hasAllowance: true, allowance: 5000 }] } });
    window.__stuck = () => { alphaState = 'stuck'; __emit('team:state', team()); };
    const info = id => ({ sessionId: id, title: id === 'lead' ? 'Lead' : id, cwd, provider: 'codex', open: true, working: false });
    return { State: async () => ({ workspace: cwd, recent: [cwd] }), Sessions: async () => [info('lead'), info('alpha'), info('beta')], SessionsAt: async () => [info('lead'), info('alpha'), info('beta')],
      ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace', 'dinah-draft:chat:lead': { text: 'Keep the lead draft' } } }), SaveUIState: async () => {},
      OpenSessionAt: async (w, id) => { __calls.push(['open', id]); return info(id); },
      ChatLead: async () => null, Team: async id => id === 'lead' ? team() : { lead: { chat: id }, group: '', members: [], stateGen: 'g', stateVer: ++ver, budget: { rows: [] } },
      SetTeamBudget: async () => { __calls.push(['budget']); return {}; },
      BusOverview: async () => ({ inbox: [], rooms, workspaces: [], links: [] }), BusResolveGroup: async name => ({ state: 'resolved', room: rooms.find(r => r.name === name) }),
      BusGroupMessages: async () => [], BusMarkGroupRead: async () => {} };
  } });
  const { page: p, ok } = t;
  try {
    const key = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'lead' })), win = p.locator('[data-win="' + key + '"]');
    const open = win.locator('.chat-team-toolbar').getByRole('button', { name: 'Open Team', exact: true });
    await open.waitFor();
    ok('a lead\'s toolbar has Last Read first, then Members and Open Team', await win.evaluate(w => {
      const shown = [...w.querySelectorAll('.chat-team-toolbar button')].filter(b => b.getClientRects().length && !b.closest('.toolbar-fold') && !b.closest('[popover]'));
      return /Last Read/.test(shown[0].textContent) && shown.some(b => /^Members/.test(b.textContent.trim())) && shown.some(b => b.textContent.trim() === 'Open Team' || b.getAttribute('aria-label') === 'Open Team');
    }));
    // The Team list: names and standard chips alone.
    await win.locator('.chat-team .team-toggle').click();
    await win.locator('.members-menu:popover-open .member-open').first().waitFor();
    const rows = await win.evaluate(w => [...w.querySelectorAll('.team-members .member-open')].map(a => {
      const chip = a.querySelector('.badge.chat-state:not(.chat-state-reserve)');
      return { name: a.querySelector('.member-name').textContent, chip: chip ? chip.textContent.trim() : '', chipWidth: chip ? chip.getBoundingClientRect().width : 0, rowWidth: a.getBoundingClientRect().width,
        extra: a.querySelectorAll('.team-reason, .team-meta, .team-usage, .team-task').length, text: a.textContent };
    }));
    ok('the Team list shows each member\'s name and standard chip', rows.length === 2 && rows[0].name === 'Alpha' && rows[0].chip === 'Working' && rows[1].chip === 'Idle', JSON.stringify(rows));
    ok('the chip is as wide as its glyph and word, not stretched', rows.every(r => r.chipWidth > 0 && r.chipWidth < r.rowWidth / 3), JSON.stringify(rows));
    ok('the Team list has no progress, workspace, provider, model, usage or last message', rows.every(r => !r.extra && !/Parser|gpt-5|requests|lexer|Writing the parser/.test(r.text)), JSON.stringify(rows));
    ok('the Team list has no allowances', await win.locator('.team-members').getByText(/Allowance/).count() === 0 && await win.getByRole('button', { name: /Team Allowances/ }).count() === 0);
    await p.evaluate(() => __stuck());
    await win.locator('.team-members .member-open').first().locator('.badge.chat-state:not(.chat-state-reserve)').filter({ hasText: 'Stuck' }).waitFor();
    ok('the Team list follows a member\'s state live', true);
    await win.locator('.team-members .member-open').first().click();
    await p.locator('.win[data-win-kind="chat-session"]').filter({ has: p.locator('.win-title', { hasText: /^alpha$/ }) }).waitFor();
    ok('a member\'s row opens its chat', true);
    // Open Team opens the team group and keeps the lead's draft.
    await p.evaluate(k => pudlWindows.raise(k), key);
    await open.click();
    const group = p.locator('.win[data-win-kind="bus-room"]').filter({ has: p.locator('.win-title', { hasText: 'Team A' }) });
    await group.locator('textarea').waitFor();
    ok('Open Team opens the lead\'s team group and keeps its draft', await group.count() === 1 && await win.locator('[data-view-id=prompt]').inputValue() === 'Keep the lead draft');
    // The team group's toolbar.
    const report = group.getByRole('button', { name: 'Team Report', exact: true }), allowances = group.getByRole('button', { name: 'Team Allowances…', exact: true });
    await report.waitFor();
    ok('the team group has Team Report and Team Allowances after Last Read', await group.evaluate(g => {
      const shown = [...g.querySelectorAll('.group-toolbar button')].filter(b => b.getClientRects().length && !b.closest('.toolbar-fold'));
      return /Last Read/.test(shown[0].textContent) && shown.some(b => /Team Report/.test(b.textContent)) && shown.some(b => /Team Allowances/.test(b.textContent));
    }));
    await report.click();
    const panel = p.locator('.team-report:popover-open');
    await panel.getByText('Writing the parser').waitFor();
    const detail = await panel.textContent();
    ok('Team Report shows each member in detail and the allowances', /Parser/.test(detail) && /gpt-5/.test(detail) && /1,234 requests/.test(detail) && /Build the lexer first/.test(detail) && /Allowance: 5,000/.test(detail), detail.slice(0, 400));
    await t.shot('team-report');
    await p.keyboard.press('Escape');
    await allowances.click();
    const dialog = p.locator('dialog[open]').filter({ hasText: 'Team Allowances' });
    await dialog.waitFor();
    ok('Team Allowances opens the allowance editor', await dialog.locator('input[type=number]').count() === 1);
    await dialog.getByRole('button', { name: /Cancel/ }).click();
    // An unrelated group has neither.
    await p.evaluate(() => dinahBus.openGroup('plain', { message: 0 }));
    const plain = p.locator('.win[data-win-kind="bus-room"]').filter({ has: p.locator('.win-title', { hasText: 'Plain Group' }) });
    await plain.locator('textarea').waitFor();
    ok('an unrelated group shows no team controls', await plain.getByRole('button', { name: /Team Report|Team Allowances/ }).count() === 0 || await plain.evaluate(g => [...g.querySelectorAll('.group-team-report, .group-team-allowances')].every(b => b.hidden)));
    // A narrow team group folds its toolbar; the report is still reached.
    const gkey = await group.getAttribute('data-win');
    await p.evaluate(k => pudlWindows.raise(k), gkey);
    await group.evaluate(g => { g.style.setProperty('--win-w', '0.16'); });
    await p.waitForTimeout(400);
    await t.shot('team-toolbar-narrow');
    const folded = await group.evaluate(g => !g.querySelector('.group-team-report').getClientRects().length);
    if (folded) {
      await group.locator('.group-toolbar .toolbar-fold > .menu-btn:visible, .group-toolbar .toolbar-more > .menu-btn:visible').first().click();
      await p.locator('.menu-panel:popover-open .menu-action').filter({ hasText: /Team Report/ }).first().click();
    } else await report.click();
    await panel.getByText('Writing the parser').waitFor();
    ok('on a narrow window the toolbar ' + (folded ? 'folds and its menu opens Team Report' : 'still shows Team Report'), await panel.count() === 1);
    await p.keyboard.press('Escape');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
