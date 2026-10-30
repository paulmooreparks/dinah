// A team lead follows output, and explicit navigation replaces old anchors.
const { launch, OUT } = require('./lib/page');
const fs = require('fs');
const path = require('path');
(async () => {
  const t = await launch({ viewport: { width: 1280, height: 850 }, app: () => ({
    State: async () => ({ workspace: 'C:/Fixture', workspaces: ['C:/Fixture'], recent: ['C:/Fixture'] }),
    OpenSessionAt: async (cwd, sessionId) => ({ sessionId, title: sessionId, cwd, provider: 'codex', open: true, working: true, life: 'open', window: true, exec: { state: 'working', why: '' } }),
    Team: async id => id === 'lead' ? { lead: { chat: id, bus: 'bus-lead', title: 'Lead' }, group: 'team-room', members: [], stateGen: 'tail', stateVer: 1 } : null,
    ChatLead: async () => null
  }) });
  const { page: p, ok } = t, measurements = [];
  const frames = n => t.deadline('layout frames', p.evaluate(n => new Promise(resolve => { function next() { if (--n) requestAnimationFrame(next); else resolve(); } requestAnimationFrame(next); }), n));
  async function measure(win, stage) {
    const value = await win.locator('[data-view-id="activity"]').evaluate(log => ({ top: log.scrollTop, height: log.clientHeight, extent: log.scrollHeight, gap: log.scrollHeight - log.clientHeight - log.scrollTop, readBack: log.hasAttribute('data-preserve-scroll-layout') }));
    measurements.push({ stage, ...value }); return value;
  }
  async function tail(win, stage) {
    await frames(5);
    const value = await measure(win, stage);
    ok(stage, value.gap < 3 && await win.locator('[data-view-id="jump-end"]').isHidden(), JSON.stringify(value));
  }
  async function output(id, round, team) {
    await p.evaluate(([id, round, team]) => {
      const update = u => __emit('agent:update', { sessionId: id, update: u });
      if (team) __emit('agent:teamsent', { sessionId: id, message: { id: 100 + round, room: 'team-room', roomTitle: 'Team', from: 'Lead' }, text: 'Assignment ' + round + '. ' + 'Instructions. '.repeat(80) });
      update({ sessionUpdate: 'tool_call', toolCallId: 'step-' + round, title: 'Run step ' + round, status: 'in_progress' });
      update({ sessionUpdate: 'tool_call_update', toolCallId: 'step-' + round, status: 'completed', content: [{ type: 'content', content: { type: 'text', text: 'Completed output.\n'.repeat(10) } }] });
      update({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Reply ' + round + '. ' + 'Streaming words. '.repeat(80) + '\n\n' } });
    }, [id, round, team]);
  }
  async function strip(version) {
    await p.evaluate(version => __emit('team:state', { lead: { chat: 'lead', bus: 'bus-lead', title: 'Lead' }, group: 'team-room', members: [{ chat: 'member', bus: 'bus-member', title: 'Member', life: 'open', window: true, exec: { state: 'working', why: 'Working on assignment ' + version }, task: 'Work on the task', usage: [], members: [] }], stateGen: 'tail', stateVer: version }), version);
  }
  try {
    for (const id of ['ordinary', 'lead']) {
      const key = await p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: id }), id);
      const win = p.locator('[data-win="' + key + '"]');
      if (id === 'ordinary') await win.locator('[data-win-action="maximize"]').click();
      const state = win.locator('.win-execution .win-state[data-state="working"]');
      await state.waitFor({ state: 'attached' });
      await frames(3);
      if (!await state.isVisible()) {
        await win.getByRole('button', { name: 'Window status', exact: true }).click();
        await p.locator('.win-status-popover:popover-open .win-state[data-state="working"]').waitFor();
        ok(id + ' exposes its folded execution state', true);
        await p.keyboard.press('Escape');
      } else ok(id + ' exposes its execution state directly', true);
      if (id === 'lead') { await strip(2); }
      for (let round = 0; round < 6; round++) { await output(id, round, id === 'lead'); await tail(win, id + ' follows output ' + round); }
      if (id === 'lead') {
        await win.locator('.chat-team > button').click(); await tail(win, 'lead follows an expanded strip');
        await win.locator('.chat-team > button').click(); await tail(win, 'lead follows a collapsed strip');
      }
      const log = win.locator('[data-view-id="activity"]');
      await log.hover();
      const extent = await log.evaluate(n => n.scrollHeight);
      await p.mouse.wheel(0, -extent * 2);
      await p.waitForFunction(key => document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]').scrollTop === 0, key);
      const target = win.locator('.activity-row').filter({ hasText: /Reply 2\./ }).first();
      await target.evaluate(row => { const log = row.closest('[data-view-id="activity"]'); log.scrollTop += row.getBoundingClientRect().top - log.getBoundingClientRect().top - 60; });
      await frames(5);
      const before = await target.evaluate(row => row.getBoundingClientRect().top - row.closest('[data-view-id="activity"]').getBoundingClientRect().top);
      await output(id, 6, id === 'lead');
      if (id === 'lead') await strip(3);
      await frames(5);
      const after = await target.evaluate(row => row.getBoundingClientRect().top - row.closest('[data-view-id="activity"]').getBoundingClientRect().top);
      ok(id + ' preserves a scrolled-back reader', Math.abs(before - after) < 3, JSON.stringify({ before, after }));
      await win.locator('[data-view-id="jump-end"]').click();
      await tail(win, id + ' jumps to the end');
      await p.setViewportSize({ width: 1100, height: 850 });
      await tail(win, id + ' keeps following through a layout change after jumping');
      // A later strip update and fold recount must not restore the old reader.
      if (id === 'lead') { await strip(4); await tail(win, 'lead stays at the end after delayed strip update'); }
      if (id === 'lead') {
        for (const count of [1, 2]) {
          await p.evaluate(count => __emit('agent:teamsent', { sessionId: 'lead', message: { id: 200 + count, room: 'team-room', roomTitle: 'Team', from: 'Lead' }, text: 'Delayed assignment ' + count }), count);
          await tail(win, 'lead stays at the end after fold insertion or recount ' + count);
        }
      }
      await output(id, 7, id === 'lead');
      await tail(win, id + ' stays at the end after new traffic');
      if (id === 'lead') await t.shot('teams-tail-following');
    }
  } finally {
    fs.writeFileSync(path.join(OUT, 'teams-tail-positions.json'), JSON.stringify(measurements, null, 2));
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
