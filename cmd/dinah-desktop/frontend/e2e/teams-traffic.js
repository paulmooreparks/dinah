// Team traffic is folded without changing conversation data or exports.
const { launch, OUT } = require('./lib/page');
const fs = require('fs');
const path = require('path');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const ws = 'C:/Fixture';
    const mark = id => ({ id, room: 'team-room', roomTitle: 'Development Team', from: id === 32 ? 'Member' : 'Lead', time: '2026-10-30T01:00:00Z' });
    const history = [
      { role: 'tool', text: 'Send the assignment\nStatus: completed\nThe assignment was posted.' },
      { role: 'sent', text: 'First outgoing assignment.', delivered: mark(31) },
      { role: 'delivered', text: 'The incoming answer.', delivered: mark(32) },
      { role: 'you', text: 'Paul asks a direct question.' },
      { role: 'sent', text: 'Second outgoing assignment.', delivered: mark(33) }
    ];
    window.__exported = '';
    return {
      State: async () => ({ workspace: ws, workspaces: [ws], recent: [ws] }),
      Sessions: async () => [{ sessionId: 'lead', title: 'Lead', cwd: ws, provider: 'codex', open: false }],
      SavedConversation: async () => ({ available: true, messages: history }),
      OpenSessionAt: async () => {
        __emit('agent:updates', { sessionId: 'lead', updates: history.map(m => m.role === 'tool' ? { sessionUpdate: 'tool_call', toolCallId: 'successful-send', title: 'Send the assignment (replayed)', status: 'completed' } : m.role === 'you' ? { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: m.text } } : { sessionUpdate: m.role === 'sent' ? 'dinah_team_sent' : 'dinah_delivered', message: m }) });
        return { sessionId: 'lead', title: 'Lead', cwd: ws, provider: 'codex', open: true };
      },
      Team: async () => ({ lead: { bus: 'bus-lead', chat: 'lead', title: 'Lead' }, group: 'team-room', members: [], stuck: 0, stateGen: 'tg', stateVer: 1 }),
      ChatLead: async () => null,
      SaveRun: async text => { __exported = text; return ''; },
      BusOverview: async () => ({ rooms: [{ name: 'team-room', title: 'Development Team', members: [], lead: 'bus-lead' }], workspaces: [], inbox: [], links: [] }),
      BusGroupMessages: async () => [{ id: 31, from: 'Lead', text: 'First outgoing assignment.' }], BusMarkGroupRead: async () => {}
    };
  } });
  const { page: p, ok } = t;
  try {
    const key = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'lead' }));
    const win = p.locator('[data-win="' + key + '"]');
    // The saved copy is visible before reconnect replays it. Wait for the
    // connected history before emitting live events.
    await win.locator('.tool-row').filter({ hasText: 'Send the assignment (replayed)' }).waitFor();
    await win.locator('.team-traffic-fold').first().waitFor();
    await p.waitForFunction(key => document.querySelector('[data-win="' + key + '"] .team-traffic-fold')?.textContent.startsWith('2 team messages'), key);
    ok('incoming and outgoing messages share a fold, interrupted by the person', await win.locator('.team-traffic-fold').count() === 2 && await win.locator('.you-row, .msg-row').filter({ hasText: 'Paul asks a direct question.' }).count() > 0);
    await p.evaluate(() => __emit('agent:teamsent', { sessionId: 'lead', message: { id: 34, room: 'team-room', roomTitle: 'Development Team', from: 'Lead' }, text: 'Live outgoing assignment.' }));
    await p.waitForFunction(key => [...document.querySelectorAll('[data-win="' + key + '"] .team-traffic-fold')].at(-1)?.textContent.startsWith('2 team messages'), key);
    await p.evaluate(() => __emit('agent:update', { sessionId: 'lead', update: { sessionUpdate: 'tool_call', toolCallId: 'failed-send', title: 'A failed send', status: 'failed', content: [{ type: 'content', content: { type: 'text', text: 'The delivery failed.' } }] } }));
    await win.locator('.tool-row').filter({ hasText: 'A failed send' }).waitFor();
    ok('failed sends remain visible outside the traffic folds', await win.locator('.tool-row.team-traffic-original').count() === 0);
    await win.evaluate(w => pudlCommands.run('export-chat', undefined, w.querySelector('.chat-instance').parentElement));
    await p.waitForFunction(() => __exported.includes('Live outgoing assignment.'));
    ok('exports retain incoming, outgoing and direct message text', await p.evaluate(() => ['First outgoing assignment.', 'The incoming answer.', 'Paul asks a direct question.', 'Second outgoing assignment.', 'Live outgoing assignment.'].every(s => __exported.includes(s))));
    await win.locator('[data-win-action="maximize"]').click();
    await win.locator('.chat-team > button').click();
    await win.locator('[data-view-id="activity"]').focus(); await p.keyboard.press('Control+Home');
    await t.shot('teams-traffic-folds');
    ok('the successful send tool call remains visible alongside its structured sent entry', await win.locator('.tool-row').filter({ hasText: 'Send the assignment' }).isVisible());
    await p.evaluate(() => { for (let i = 0; i < 80; i++) __emit('agent:sent', { sessionId: 'lead', kind: 'asked', text: 'Reader position ' + i + '. ' + 'A line of ordinary conversation. '.repeat(8) }); });
    await p.waitForFunction(key => document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]').textContent.includes('Reader position 79.'), key);
    await win.evaluate(w => {
      const log = w.querySelector('[data-view-id="activity"]');
      window.__trafficPositions = [];
      window.__trafficMeasure = stage => {
        const row = [...log.querySelectorAll('.activity-row')].find(r => r.textContent.includes('Reader position 40.'));
        if (!row) return;
        const a = log.getBoundingClientRect(), b = row.getBoundingClientRect();
        const value = { stage, time: performance.now(), offset: b.top - a.top, rowHeight: b.height, viewportTop: a.top, viewportHeight: log.clientHeight, scrollTop: log.scrollTop, scrollHeight: log.scrollHeight, preserved: log.hasAttribute('data-preserve-scroll-layout') };
        const prev = __trafficPositions.at(-1);
        if (stage !== 'frame' || !prev || Object.keys(value).some(k => k !== 'time' && k !== 'stage' && value[k] !== prev[k])) __trafficPositions.push(value);
      };
      let frames = 0;
      function sample() { __trafficMeasure('frame'); if (++frames < 900) window.__trafficFrame = requestAnimationFrame(sample); }
      sample();
    });
    await win.locator('[data-view-id="activity"]').hover();
    // mouse.wheel returns before scrolling finishes. Start at the end and
    // wait for its destination before positioning the row under test.
    await win.locator('[data-view-id="activity"]').focus(); await p.keyboard.press('Control+End');
    await p.waitForFunction(key => document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]').scrollTop > 0, key);
    const extent = await win.locator('[data-view-id="activity"]').evaluate(log => log.scrollHeight);
    await p.mouse.wheel(0, -extent * 2);
    await p.waitForFunction(key => { const log = document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]'); return log.scrollTop === 0 && log.hasAttribute('data-preserve-scroll-layout'); }, key);
    const target = win.locator('.activity-row').filter({ hasText: /^.*Reader position 40\./ }).first();
    await target.waitFor();
    await target.evaluate(row => { const log = row.closest('[data-view-id="activity"]'); log.scrollTop += row.getBoundingClientRect().top - log.getBoundingClientRect().top - 40; });
    await p.waitForFunction(key => { const log = document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]'); const row = [...log.querySelectorAll('.activity-row')].find(r => r.textContent.includes('Reader position 40.')); const a = log.getBoundingClientRect(), b = row.getBoundingClientRect(); return b.bottom > a.top && b.top < a.bottom; }, key);
    const before = await target.evaluate(row => {
      const log = row.closest('[data-view-id="activity"]'), a = log.getBoundingClientRect(), b = row.getBoundingClientRect();
      if (!(b.bottom > a.top && b.top < a.bottom)) throw Error('The reader target left the viewport before the team update.');
      __trafficMeasure('before-update');
      const offset = b.top - a.top;
      __emit('team:state', { lead: { chat: 'lead', bus: 'bus-lead', title: 'Lead' }, group: 'team-room', stateGen: 'tg', stateVer: 2, stuck: 1, members: [{ chat: 'member', bus: 'bus-member', title: 'Member', state: 'stuck', stateWhy: 'Its context is full and needs recovery.', task: 'Continue the assignment', taskMessage: 31, taskGroup: 'team-room', usage: [], members: [] }] });
      __trafficMeasure('synchronous-update');
      return offset;
    });
    await p.evaluate(() => __trafficMeasure('after-update'));
    await win.locator('[data-team-member="bus-member"]').waitFor();
    await p.waitForFunction(([key, want]) => { const log = document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]'); const row = [...log.querySelectorAll('.activity-row')].find(r => r.textContent.includes('Reader position 40.')); const a = log.getBoundingClientRect(), b = row.getBoundingClientRect(); return b.bottom > a.top && b.top < a.bottom && Math.abs(b.top - a.top - want) < 2; }, [key, before]);
    ok('a live team-strip update keeps the visible reader target in place', true);
    await t.shot('teams-reader-position');
    await p.evaluate(() => { __trafficMeasure('reading-check-complete'); cancelAnimationFrame(__trafficFrame); });
    fs.writeFileSync(path.join(OUT, 'teams-traffic-position.json'), JSON.stringify(await p.evaluate(() => __trafficPositions), null, 2));
    await win.locator('.team-traffic-fold a').first().click();
    await p.locator('[data-bus-message="31"].bus-target-message').waitFor();
    ok('the fold opens its original group message', true);
    await p.reload();
    await p.locator('.team-traffic-fold').first().waitFor();
    ok('restored history retains the structured folds', await p.locator('.team-traffic-fold').count() === 2);
  } catch (error) {
    const state = await p.evaluate(() => ({ exported: __exported, calls: __bridgeCalls.filter(c => c[0] === 'SaveRun'), rows: [...document.querySelectorAll('.activity-row')].map(r => ({ text: r.textContent, message: r._msg && r._msg.text })) }));
    fs.writeFileSync(path.join(OUT, 'teams-traffic-failure.json'), JSON.stringify(state, null, 2));
    throw error;
  } finally {
    const positions = await p.evaluate(() => { cancelAnimationFrame(window.__trafficFrame); return window.__trafficPositions || []; }).catch(() => []);
    if (positions.length) fs.writeFileSync(path.join(OUT, 'teams-traffic-position.json'), JSON.stringify(positions, null, 2));
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
