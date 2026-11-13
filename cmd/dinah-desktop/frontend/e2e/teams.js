// Teams retain ordinary chat controls and histories. The lead's Members
// dropdown shows its direct reports and their states; the team group's Team
// Report shows the hierarchy in detail, and its toolbar edits the allowances.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const ws = 'C:/Fixture';
    window.__teamCalls = [];
    const parent = { bus: 'bus-lead', chat: 'lead', title: 'Team Lead', group: 'team-room' };
    const member = (chat, title, state, extra) => {
      const row = Object.assign({ bus: 'bus-' + chat, chat, title, workspace: 'Fixture', provider: 'codex', model: 'model-a', state, task: 'Build the shared controls', taskMessage: 11, group: 'team-room', open: true, retired: false, members: [], usage: [{ today: 200, todayUnknown: 0, used: 200, unit: 'tokens', complete: true }] }, extra);
      row.life = state === 'retired' || state === 'closed' ? state : 'open'; row.window = row.life === 'open';
      if (row.life === 'open') row.exec = { state, why: row.stateWhy || '' };
      return row;
    };
    window.__team = { lead: parent, group: 'team-room', stateGen: 'team-g1', stateVer: 1, stuck: 1, counts: { top: 'stuck', stuck: 1, waiting: 2, working: 1, idle: 0, closed: 1, unknown: 0 }, members: [
      member('builder', 'Builder', 'working'),
      member('nested', 'Nested Lead', 'waiting', { stateWhy: 'Waiting for its lead to relay', members: [member('child', 'Nested Worker', 'stuck', { stateWhy: 'The provider is unavailable', usage: [{ today: 0, todayUnknown: 3, used: 0, unit: 'tokens', unknown: 3, complete: false }] })], stuckBelow: 1 }),
      member('held', 'Held Member', 'waiting', { stateWhy: 'Context full' }),
      member('retired', 'Earlier Worker', 'retired', { retired: true })
    ], budget: { providers: [{ provider: 'codex', name: 'Codex', unit: 'tokens' }, { provider: 'claude', name: 'Claude Code', unit: 'tokens' }], unmeasured: ['claude'], periodStart: '2026-10-26T00:00:00+08:00', periodEnd: '2026-11-02T00:00:00+08:00', rows: [
      { provider: 'codex', name: 'Codex', unit: 'tokens', hasAllowance: true, allowance: 1000, used: 600, today: 200, unknown: 1, todayUnknown: 1, complete: false, over: false }
    ] } };
    window.__closedMember = (closed, reason) => {
      const row = __team.members[2], wasClosed = row.life === 'closed';
      row.life = closed ? 'closed' : 'open'; row.window = row.open = !closed;
      row.state = closed ? 'closed' : 'waiting'; row.stateWhy = closed ? reason : 'Context full';
      if (closed) delete row.exec; else row.exec = { state: 'waiting', why: 'Context full' };
      if (wasClosed !== closed) { __team.counts.closed += closed ? 1 : -1; __team.counts.waiting += closed ? -1 : 1; }
      __team.stateVer++; __emit('team:state', structuredClone(__team));
    };
    window.__memberLead = true;
    const chats = ['lead', 'builder', 'nested', 'child', 'held', 'retired'].map(id => ({ id: 'bus-' + id, chat: id, title: id === 'lead' ? 'Team Lead' : id === 'builder' ? 'Builder' : id, workspace: 'Fixture', state: 'open' }));
    window.__teamRoom = { name: 'team-room', title: 'Development Team', members: chats.concat([{ id: 'paul', title: 'Paul', state: 'person' }]), unread: 0, lead: 'bus-lead' };
    window.__teamMessages = [{ id: 11, createdAt: '2026-10-30T00:00:00Z', from: 'Team Lead', fromSession: 'bus-lead', room: 'team-room', roomTitle: 'Development Team', text: 'The full assignment to build the controls.', mentions: [] }, { id: 12, createdAt: '2026-10-30T00:01:00Z', from: 'Builder', room: 'team-room', text: 'A decision for Paul.', relay: { lead: 'bus-lead', leadTitle: 'Team Lead', group: 'team-room', message: 12 }, mentions: [] }];
    return {
      State: async () => ({ workspace: ws, recent: [ws], workspaces: [ws] }),
      Sessions: async () => chats.map(c => ({ sessionId: c.chat, title: c.title, provider: 'codex', cwd: ws, open: false })),
      OpenSessionAt: async (w, id) => ({ sessionId: id, title: chats.find(c => c.chat === id)?.title || id, provider: 'codex', cwd: w, open: true, working: id === 'builder', lead: id === 'builder' && __memberLead ? parent : null }),
      Team: async id => id === 'lead' ? structuredClone(__team) : { lead: { chat: id, bus: 'bus-' + id, title: id }, group: '', members: [], stuck: 0, stateGen: 'team-g1', stateVer: 1 },
      ChatLead: async id => id === 'builder' && __memberLead ? parent : null,
      SetTeamLead: async (m, l) => { __teamCalls.push(['set-lead', m, l]); return 'team-room'; },
      ClearTeamLead: async id => { __teamCalls.push(['clear-lead', id]); __memberLead = false; },
      MakeGroupATeam: async (group, lead) => { __teamCalls.push(['make-team', group, lead]); return { group, lead, made: [], leftOut: [] }; },
      SetTeamBudget: async (id, rows) => { __teamCalls.push(['budget', id, rows]); return __team.budget; },
      Send: async () => { __teamCalls.push(['send']); }, Queue: async () => { __teamCalls.push(['queue']); }, Steer: async () => { __teamCalls.push(['steer']); },
      Cancel: async id => { __teamCalls.push(['stop', id]); }, CloseSession: async id => { __teamCalls.push(['close-session', id]); },
      BusOverview: async () => ({ inbox: [], unread: 0, rooms: [__teamRoom], workspaces: [{ name: 'Fixture', label: 'Fixture', chats }], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
      BusGroupMessages: async () => structuredClone(__teamMessages), BusMarkGroupRead: async () => {},
      BusSendToGroup: async () => { __teamCalls.push(['group-send']); },
      Notifications: async () => [], DismissNotification: async id => { __teamCalls.push(['dismiss', id]); }
    };
  } });
  const { page: p, ok } = t;
  try {
    const leadKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'lead' }));
    const lead = p.locator('[data-win="' + leadKey + '"]');
    await lead.locator('.chat-team').waitFor();
    // The lead's Members dropdown: its direct reports, each with its named state chip (f22f9b5).
    const openMembers = async () => {
      await p.evaluate(key => pudlWindows.raise(key), leadKey);
      if (!(await lead.locator('.team-members:popover-open').count())) await lead.locator('.chat-team .team-toggle').click();
      await lead.locator('.team-members:popover-open').waitFor();
    };
    const leadChip = id => lead.locator('.team-members [data-team-member="' + id + '"] .team-state > .badge.chat-state:not(.chat-state-reserve)');
    await openMembers();
    await lead.locator('.team-members [data-team-member="bus-builder"]').waitFor();
    ok('the lead sees its direct reports\' named state glyphs and reasons, and retired members apart', /Working/.test(await leadChip('bus-builder').getAttribute('aria-label')) && /Context full/.test(await leadChip('bus-held').getAttribute('aria-label'))
      && await lead.locator('.team-members .member-list[aria-label="Members"] [data-team-member]').count() === 3 && await lead.locator('.team-members [data-team-member="bus-child"]').count() === 0
      && await lead.locator('.team-members .member-list[aria-label="Retired members"] [data-team-member="bus-retired"]').count() === 1);
    const reason = 'Its chat is not open, and a message waits for it: all 64 places for chats open in the background are taken by chats that are working';
    const compact = lead.locator('.team-members [data-team-member="bus-held"] .member-open');
    const rect = locator => locator.evaluate(el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; });
    const compactBefore = await rect(compact);
    await compact.focus();
    await p.evaluate(reason => __closedMember(true, reason), reason);
    await leadChip('bus-held').filter({ hasText: 'Closed' }).waitFor();
    ok('a closed admission wait stays Closed with its reason in the Members tooltip and accessible name', (await compact.getAttribute('title')).includes(reason) && (await compact.getAttribute('aria-label')).includes(reason) && (await leadChip('bus-held').getAttribute('aria-label')).includes(reason));
    ok('the closed-wait wording preserves Members row geometry and focus', JSON.stringify(compactBefore) === JSON.stringify(await rect(compact)) && await compact.evaluate(el => el === document.activeElement));
    await p.evaluate(() => __closedMember(false));
    await p.keyboard.press('Escape');
    // Reasons, tasks, usage and allowances are the team group's Team Report (9f20613).
    await p.evaluate(() => dinahBus.openGroup('team-room'));
    const group = p.locator('.win[data-win-kind="bus-room"]');
    const report = group.getByRole('button', { name: 'Team Report', exact: true });
    const panel = p.locator('.team-report:popover-open');
    await report.click();
    await panel.locator('[data-team-member="bus-builder"] .team-task').waitFor();
    ok('the Team Report shows named state glyphs, reasons, task and provider usage', /Working/.test(await panel.locator('[data-team-member="bus-builder"] > a .team-state > .badge.chat-state:not(.chat-state-reserve)').getAttribute('aria-label')) && /Context full/.test(await panel.locator('[data-team-member="bus-held"]').innerText()) && /200 tokens/.test(await panel.locator('[data-team-member="bus-builder"]').innerText()) && /Build the shared controls/.test(await panel.locator('[data-team-member="bus-builder"]').innerText()));
    const detailRow = panel.locator('[data-team-member="bus-held"] > a');
    const detailBefore = await rect(detailRow);
    await detailRow.focus();
    await p.evaluate(reason => __closedMember(true, reason), reason);
    await panel.locator('[data-team-member="bus-held"] .team-reason').filter({ hasText: reason }).waitFor();
    ok('Team Report keeps a closed admission wait separate from execution', (await detailRow.locator('.chat-state:not(.chat-state-reserve)').textContent()).trim() === 'Closed' && (await detailRow.getAttribute('aria-label')).includes(reason));
    ok('the closed-wait reason preserves Team Report row geometry and focus', JSON.stringify(detailBefore) === JSON.stringify(await rect(detailRow)) && await detailRow.evaluate(el => el === document.activeElement));
    await p.setViewportSize({ width: 520, height: 900 });
    const narrowBefore = await rect(detailRow);
    const longerReason = reason + '. The message remains queued until a place is free.';
    await p.evaluate(reason => __closedMember(true, reason), longerReason);
    await panel.locator('[data-team-member="bus-held"] .team-reason').filter({ hasText: longerReason }).waitFor();
    ok('longer admission text updates in place at narrow width and keeps its full accessible explanation', JSON.stringify(narrowBefore) === JSON.stringify(await rect(detailRow)) && await detailRow.locator('.team-reason').getAttribute('title') === longerReason && await detailRow.locator('.team-reason').evaluate(el => el.scrollWidth > el.clientWidth));
    await t.shot('teams-closed-admission-wait');
    await p.setViewportSize({ width: 1440, height: 1000 });
    await p.evaluate(() => __closedMember(false));
    const nested = panel.locator('[data-team-member="bus-nested"] > a');
    await nested.focus(); await nested.press('ArrowLeft');
    ok('a nested team folds separately from its chat link', await nested.getAttribute('aria-expanded') === 'false');
    await nested.press('ArrowRight');
    await panel.locator('[data-team-member="bus-child"]').waitFor();
    ok('nested members show incomplete usage in words', /3 turns unmeasured/.test(await panel.locator('[data-team-member="bus-child"]').innerText()));
    ok('retired members are listed apart, and the lead\'s title bar has no team chip', await panel.locator('ul[aria-label="Retired members"] [data-team-member="bus-retired"]').count() === 1 && await panel.locator('ul[aria-label="Team members"] [data-team-member="bus-retired"]').count() === 0 && await lead.locator('.team-stuck-chip').count() === 0);
    ok('allowances keep the provider unit and explain incomplete remaining usage', /Remaining usage is incomplete/.test(await panel.locator('.team-budget').textContent()) && await panel.locator('meter').count() === 1);
    const taskFocus = panel.locator('[data-team-member="bus-builder"] .team-task');
    await taskFocus.focus();
    await p.evaluate(() => { __team.stateVer++; __team.members[0].usage[0].today = 250; __emit('team:state', structuredClone(__team)); });
    await p.waitForFunction(() => document.querySelector('.team-report [data-team-member="bus-builder"] .team-usage').textContent.includes('250'));
    ok('live usage updates preserve keyboard focus on the assignment link', await taskFocus.evaluate(n => document.activeElement === n));
    await t.shot('teams-lead-light-focused');
    await p.keyboard.press('Escape');
    await panel.waitFor({ state: 'detached' });
    const allowancesButton = group.getByRole('button', { name: 'Team Allowances…', exact: true });
    await allowancesButton.click();
    const allowances = p.locator('.team-budget-dialog');
    await allowances.waitFor();
    ok('the allowance dialog has its accessible name', await p.getByRole('dialog', { name: 'Team Allowances', exact: true }).count() === 1);
    const codexAllowance = allowances.getByLabel('Codex (tokens)', { exact: true });
    await codexAllowance.fill('0');
    await allowances.getByRole('button', { name: 'Save', exact: true }).click();
    await allowances.waitFor({ state: 'detached' });
    ok('a zero allowance is saved as a real limit', await p.evaluate(() => __teamCalls.some(c => c[0] === 'budget' && c[2].some(r => r.provider === 'codex' && r.allowance === 0))));
    await allowancesButton.click();
    await codexAllowance.fill('');
    await allowances.getByRole('button', { name: 'Save', exact: true }).click();
    await allowances.waitFor({ state: 'detached' });
    ok('an empty allowance removes the limit', await p.evaluate(() => !__teamCalls.filter(c => c[0] === 'budget').at(-1)[2].some(r => r.provider === 'codex')));

    await report.click();
    await panel.locator('.team-budget').scrollIntoViewIfNeeded();
    await t.shot('teams-allowances');
    await p.keyboard.press('Escape');
    await panel.waitFor({ state: 'detached' });
    await p.evaluate(() => { __emit('agent:session', { sessionId: 'builder', title: 'Builder', cwd: 'C:/Fixture', provider: 'codex', open: true, working: true, lead: { bus: 'bus-lead', chat: 'lead', title: 'Team Lead', group: 'team-room' } }); __emit('agent:update', { sessionId: 'builder', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Working in the background.' } } }); });
    await p.waitForFunction(() => document.querySelector('#applet-storage [data-team-member]'));
    ok('background member updates create no window or taskbar button', await p.locator('.win[data-win-kind="chat-session"]').count() === 1);
    await openMembers();
    await lead.locator('.team-members [data-team-member="bus-builder"] .member-open').click();
    const builderKey = await p.evaluate(() => dinahWorkbench.key({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'builder' }));
    const member = p.locator('[data-win="' + builderKey + '"]');
    await member.locator('.team-member-note').waitFor();
    ok('a member keeps its toolbar and approvals outside the removed message box', await member.locator('.composer-row').isHidden() && await member.locator('.composer-bar').isVisible() && await member.locator('[data-command="provider"]').count() > 0 && await member.locator('[data-view-id="context-hold"]').count() === 1);
    ok('team bodies omit aggregates, and the title shows the lead\'s own state alone', await lead.locator('.team-aggregate').count() === 0 && await lead.locator('.team-stuck-chip').count() === 0 && await lead.locator('.chat-working-chip .chat-state').count() === 1);
    await t.shot('teams-member');
    // The team draft survives and gains a mention, without being sent.
    await p.evaluate(() => dinahBus.openGroup('team-room'));
    await group.locator('textarea').fill('Existing draft text');
    await p.evaluate(() => dinahOpenChat('builder'));
    await member.locator('.team-member-note a').click();
    await p.waitForFunction(() => document.querySelector('.group-composer textarea').value.includes('@Builder'));
    ok('Open Team Chat preserves the draft, adds its mention and sends nothing', /Existing draft text.*@Builder/.test(await group.locator('textarea').inputValue()) && await p.evaluate(() => !__teamCalls.some(c => c[0] === 'group-send')));
    ok('member and routed team composer have no bottom recipient panel', await member.locator('.group-notice-note, .group-routing-summary, .group-routing-details').count() === 0 && await group.locator('.group-composer .group-notice-note').count() === 0 && await group.locator('.group-toolbar .group-mention-everyone').isVisible());
    await group.locator('[data-bus-message="12"] .bus-relay-mark').waitFor();
    ok('a relayed group message says how it is addressed without a notification', /For Paul through Team Lead/.test(await group.locator('.bus-relay-mark').textContent()) && await p.evaluate(() => dinahHost.notices.list(e => e.data.notificationId).length) === 0);
    await p.evaluate(key => pudlWindows.raise(key), await group.getAttribute('data-win'));
    await report.click();
    await panel.locator('[data-team-member="bus-builder"] .team-task').click();
    await group.locator('[data-bus-message="11"].bus-target-message').waitFor();
    ok('the task link opens the assignment at its stable message address', JSON.parse(new URL(await p.url()).searchParams.get('view.' + await group.getAttribute('data-win'))).message === 11);
    await p.evaluate(() => { document.documentElement.dataset.theme = 'dark'; pudlWindows.raise(document.querySelector('.win[data-win-kind="chat-session"] .chat-team')?.closest('.win').dataset.win); });
    await t.shot('teams-lead-dark-focused');
    await p.evaluate(() => { document.documentElement.dataset.theme = 'light'; __team.stateVer++; __team.members[0].exec.state = 'idle'; __team.counts.working = 0; __team.counts.idle = 1; __emit('team:state', structuredClone(__team)); });
    await openMembers();
    await lead.locator('.team-members [data-team-member="bus-builder"] .team-state > .badge.chat-state-idle:not(.chat-state-reserve)').waitFor();
    await p.evaluate(() => { __team.stateVer--; __team.members[0].exec.state = 'working'; __emit('team:state', structuredClone(__team)); });
    ok('an older team event cannot replace the live snapshot', /Idle/.test(await leadChip('bus-builder').getAttribute('aria-label')));
    await p.keyboard.press('Escape');

    await p.evaluate(() => __emit('notifications:added', { id: 'relay-12', kind: 'relay', severity: 'warning', title: 'The lead could not relay the decision', body: 'A decision for Paul.', group: 'team-room', message: 12, lead: 'lead', details: ['The provider could not run.'] }));
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    const failedRelay = p.locator('#questions [data-notice-id="relay-12"]');
    await failedRelay.waitFor();
    ok('a failed relay keeps the reason and both source and lead actions', /The provider could not run/.test(await failedRelay.innerText()) && await failedRelay.getByRole('button', { name: 'Open Group' }).count() === 1 && await failedRelay.getByRole('button', { name: 'Open Lead' }).count() === 1);
    await t.shot('teams-relay-failure');
    const memberKey = await member.getAttribute('data-win');
    await p.evaluate(key => pudlWindows.close(key), memberKey);
    ok('closing a member view does not stop or close its session', await p.evaluate(() => !__teamCalls.some(c => c[0] === 'stop' || c[0] === 'close-session')));
    await p.evaluate(() => dinahOpenChat('builder'));
    await member.locator('.team-member-note').waitFor();
    ok('a reopened member view still has its team link and no private message box', await member.locator('.composer-row').isHidden());
    await p.evaluate(() => { __memberLead = false; __emit('agent:session', { sessionId: 'builder', stateGen: 'chat-g', stateVer: 1, lead: null, provider: 'codex', cwd: 'C:/Fixture', open: true }); });
    await member.locator('.composer-row').waitFor();
    ok('making a member ordinary restores its message box', await member.locator('.team-member-note').isHidden());
    await p.setViewportSize({ width: 520, height: 900 });
    await p.evaluate(key => pudlWindows.raise(key), leadKey);
    await t.shot('teams-narrow');
    await p.setViewportSize({ width: 1440, height: 1000 });
    await p.evaluate(() => { void dinahTeams.convertGroup({ name: 'team-room', members: [{ chat: 'lead', title: 'Team Lead', state: 'open' }] }); });
    const convert = p.locator('.team-dialog');
    await convert.waitFor();
    ok('the conversion dialog has its accessible name', await p.getByRole('dialog', { name: 'Make This Group a Team', exact: true }).count() === 1);
    ok('making a group a team asks for its lead with Cancel first', await convert.locator('.dialog-actions button').first().textContent() === 'Cancel' && await convert.locator('.btn-primary').count() === 1);
    await convert.getByRole('button', { name: 'Make This Group a Team', exact: true }).click();
    await convert.waitFor({ state: 'detached' });
    ok('conversion uses the stable group key and selected chat', await p.evaluate(() => __teamCalls.some(c => c[0] === 'make-team' && c[1] === 'team-room' && c[2] === 'lead')));

  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
