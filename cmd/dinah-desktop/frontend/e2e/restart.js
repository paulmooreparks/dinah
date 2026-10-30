// Restarting Devin from the Devin item in the status area: the menu, the
// warning about running turns, the restart, and the session reopened.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    state: { workspace: 'C:\\W', recent: ['C:\\W'], approvals: 'dinah', devin: 'd', titles: {}, templates: [] },
    app: () => ({
      Sessions: async () => [{ sessionId: 'a', cwd: 'C:\\W', title: 'Install the setup', updatedAt: '2026-10-08T00:00:00Z' }],
      // The fixture defines SkillsForProviderAt, which the page prefers to
      // SkillsAt when it exists, so both record the refresh.
      SkillsAt: async () => { window.__calls.push('Skills'); return []; },
      SkillsForProviderAt: async () => { window.__calls.push('Skills'); return []; },
      OpenSessionAt: async (workspace, id) => { window.__calls.push('OpenSession ' + id); return { sessionId: id, cwd: 'C:\\W', title: 'Install the setup', open: true, working: false, queued: [] }; },
      Restart: async () => { window.__calls.push('Restart'); window.__emit('agent:state', { state: 'stopped', detail: 'Devin stopped' }); window.__emit('agent:state', { state: 'ready', detail: 'Devin Agent is ready' }); },
      SignIn: async () => { window.__calls.push('SignIn'); }
    }),
    init: () => { window.__calls = []; },
  });
  const { page: p, ok } = t;
  require('./lib/windows')(p);
  try {
    await p.waitForTimeout(400);
    await p.click('#sessions a:has-text("Install the setup")');
    await p.waitForTimeout(200);
    await p.evaluate(() => { window.__emit('agent:state', { state: 'ready', detail: 'Devin Agent is ready' }); window.__emit('agent:session', { sessionId: 'a', cwd: 'C:\\W', title: 'Install the setup', open: true, working: true, queued: [] }); });
    await p.waitForTimeout(100);
    await p.click('#status-agent');
    await p.waitForTimeout(200);
    ok('the Devin item opens its menu', await p.evaluate(() => document.getElementById('agent-menu').matches(':popover-open')));
    ok('the menu says what Devin is doing', (await p.textContent('#agent-state')) === 'Devin Agent is ready.');
    ok('the menu warns about a running turn', (await p.textContent('#restart-meta')).startsWith('Stops the turn that is running'), await p.textContent('#restart-meta'));
    await t.shot('agent-menu');
    await p.evaluate(() => { window.__calls = []; });
    await p.click('#agent-menu [data-cmd="restart"]');
    await p.waitForTimeout(400);
    const calls = await p.evaluate(() => window.__calls);
    ok('Restart reopens the session and refreshes its provider skills', calls[0] === 'Restart' && calls.indexOf('OpenSession a') > 0 && calls.lastIndexOf('Skills') > calls.indexOf('OpenSession a'), calls.join('|'));
    ok('the menu closes', !(await p.evaluate(() => document.getElementById('agent-menu').matches(':popover-open'))));
    ok('the badge ends as ready', (await p.textContent('#status-agent-badge')).trim() !== 'off', await p.textContent('#status-agent-badge'));
    await p.click('#status-agent');
    await p.waitForTimeout(150);
    await p.click('#agent-menu [data-cmd="sign-in"]');
    await p.waitForTimeout(200);
    ok('Sign In is in the same menu', (await p.evaluate(() => window.__calls)).includes('SignIn') && !(await p.evaluate(() => document.getElementById('agent-menu').matches(':popover-open'))));
  } finally { await t.close(); }
})();
