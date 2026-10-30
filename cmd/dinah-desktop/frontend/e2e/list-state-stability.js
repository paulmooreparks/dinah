// Reading a list or its menu must hold each row in place while state repaints.
const fs = require('fs'), path = require('path');
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const cwd = 'C:/Fixture';
    let version = 1, state = { life: 'open', window: true, exec: { state: 'idle', why: '' } };
    const chats = () => ['alpha', 'beta', 'gamma'].map(id => ({ sessionId: id, title: 'Chat ' + id, cwd, provider: 'codex', open: true,
      chatState: id === 'alpha' ? state : { life: 'open', window: true, exec: { state: id === 'beta' ? 'working' : 'waiting', why: '' } } }));
    const snapshot = () => ({ stateGen: 'list-stability', stateVer: version, chats: chats().map(c => ({ sessionId: c.sessionId, bus: 'bus-' + c.sessionId, ...c.chatState })), groups: [] });
    const team = () => ({ lead: { chat: 'beta', bus: 'bus-beta', title: 'Chat beta' }, group: 'team', stateGen: 'list-team', stateVer: version,
      counts: { top: state.exec?.state || '', [state.exec?.state || (state.life === 'closed' ? 'closed' : 'unknown')]: 1 }, members: [{ chat: 'alpha', bus: 'bus-alpha', title: 'Chat alpha', workspace: 'Fixture', provider: 'codex', model: 'Fixture model', ...state }] });
    window.__listStep = kind => {
      version++;
      state = kind === 'closed' || kind === 'retired' ? { life: kind, window: false } : kind === 'unknown' ? { life: 'open', window: true } : kind === 'elsewhere' ? { life: 'open', window: false, elsewhere: 'Running in another Dinah window' } : { life: 'open', window: true, exec: { state: kind, why: kind === 'waiting' || kind === 'stuck' ? 'The full reason remains available while its visible line is clipped. '.repeat(12) : '' } };
      __emit('agent:chatstate', structuredClone(snapshot()));
      __emit('team:state', structuredClone(team()));
    };
    return { State: async () => ({ workspace: cwd, workspaceId: 'fixture', workspaceRecords: [{ id: 'fixture', name: 'Fixture', executionPath: cwd, sources: [{ id: 'root', kind: 'local-folder', path: cwd }], pins: [] }], workspaces: [cwd], recent: [cwd] }), Sessions: async () => chats(), SessionsAt: async () => chats(),
      ChatStates: async () => snapshot(), OpenSessionAt: async (ws, id) => structuredClone(chats().find(c => c.sessionId === id)),
      ChatLead: async () => null, Team: async id => id === 'beta' ? structuredClone(team()) : null };
  } });
  const p = t.page, evidence = [];
  try {
    const teamKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'beta' }));
    const team = p.locator('[data-win="' + teamKey + '"]');
    await team.locator('.chat-team > .disclosure').click();
    await team.locator('[data-team-member="bus-alpha"]').waitFor();
    await team.locator('[data-win-action="maximize"]').click();
    const listKey = await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    const listWin = p.locator('[data-win="' + listKey + '"]');
    await p.evaluate(key => pudlWindows.dock(key, null), listKey);
    if (await listWin.locator('[data-win-action="maximize"]').getAttribute('title') === 'Maximize') await listWin.locator('[data-win-action="maximize"]').click();
    const tokens = await p.evaluate(() => ['2xs','xs','sm','md','base','lg','xl','2xl','3xl'].map(s => ['--text-' + s, getComputedStyle(document.documentElement).getPropertyValue('--text-' + s)]));
    const frames = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(() => requestAnimationFrame(r)))));
    for (const theme of ['light', 'dark']) for (const width of [1440, 700, 390]) for (const scale of [1, 2]) {
      await p.setViewportSize({ width, height: 1000 });
      await p.mouse.move(1, 1);
      await p.evaluate(({ theme, tokens, scale }) => { pudlSetTheme(theme); tokens.forEach(([name, value]) => document.documentElement.style.setProperty(name, parseFloat(value) * scale + 'px')); document.activeElement.blur(); __listStep('idle'); }, { theme, tokens, scale });
      await frames();
      const row = p.locator('#sessions [data-session-row="alpha"]'), link = row.locator('a[data-session]');
      await link.focus();
      const trace = await p.evaluate(async () => {
        const host = document.querySelector('#sessions'), anchor = host.querySelector('[data-session-row="alpha"] a');
        const sample = () => [...host.querySelectorAll('.browser-group-head, [data-session-row], .session-title, .session-reason, .session-badge, .session-attention-slot')].map((el, i) => ({ key: el.dataset.sessionRow || el.className + '/' + i, ...el.getBoundingClientRect().toJSON() }));
        const before = sample(), samples = [], failures = [], parent = anchor.closest('.browser-group');
        for (const state of ['working', 'waiting', 'stuck', 'idle', 'closed', 'retired', 'elsewhere', 'unknown']) {
          __listStep(state);
          for (let frame = 0; frame < 8; frame++) {
            await new Promise(requestAnimationFrame);
            const now = sample(); samples.push({ state, frame, rectangles: now });
            now.forEach((r, i) => { const b = before[i]; if (!b || ['x','y','width','height'].some(k => Math.abs(r[k] - b[k]) > .02)) failures.push({ state, frame, before: b, after: r }); });
            if (document.activeElement !== anchor || anchor.closest('.browser-group') !== parent) failures.push({ state, frame, lostFocusOrGroup: true });
          }
          const reason = anchor.querySelector('.session-reason');
          const expected = ['closed','retired','elsewhere'].includes(state) ? 'chat-lifecycle' : 'chat-state-' + state;
          if (!anchor.querySelector('.session-badge').classList.contains(expected)) failures.push({ state, painted: anchor.querySelector('.session-badge').className });
          if (reason.scrollHeight > reason.clientHeight + 1) failures.push({ state, reasonClippedVertically: true });
          if (state === 'waiting' && !reason.title.includes('full reason')) failures.push({ state, missingFullReason: true });
        }
        for (const count of [1, 9, 10, 99, 100, 0]) {
          for (let i = 0; i < 100; i++) {
            const id = 'list-question-' + i;
            if (i < count) dinahHost.notices.add({ id, type: 'question', description: 'Fixture question ' + i, target: { chat: 'alpha' } });
            else dinahHost.notices.remove(id);
          }
          for (let frame = 0; frame < 8; frame++) {
            await new Promise(requestAnimationFrame);
            const now = sample(); samples.push({ count, frame, rectangles: now });
            now.forEach((r, i) => { const b = before[i]; if (!b || ['x','y','width','height'].some(k => Math.abs(r[k] - b[k]) > .02)) failures.push({ count, frame, before: b, after: r }); });
          }
          const badge = anchor.querySelector('.session-attention'), slot = anchor.querySelector('.session-attention-slot');
          if (count && (!badge || badge.textContent !== (count > 99 ? '99+' : String(count)) || badge.scrollWidth > badge.clientWidth + 1)) failures.push({ count, badgeClippedOrWrong: true });
          if (!count && (badge || slot.getAttribute('aria-hidden') !== 'true')) failures.push({ count, emptyBadgeAccessible: true });
        }
        return { before, samples, failures };
      });
      t.ok('list focus holds geometry through every state ' + [theme,width,scale].join('/'), trace.failures.length === 0, JSON.stringify(trace.failures.slice(0, 2)));
      // A pointer holds regrouping even after focus leaves.
      await row.hover();
      await p.evaluate(() => { document.activeElement.blur(); __listStep('waiting'); });
      await frames();
      t.ok('pointer keeps the changing row in its old group ' + [theme,width,scale].join('/'), await row.evaluate(el => el.closest('.browser-group').dataset.section === 'chats-Chats'));
      await p.mouse.move(1, 1);
      await p.waitForFunction(() => document.querySelector('#sessions [data-session-row="alpha"]').closest('.browser-group').dataset.section === 'chats-Needs You');
      t.ok('regrouping applies after pointer and focus leave ' + [theme,width,scale].join('/'), true);
      await t.shot('list-stability-' + theme + '-' + width + '-' + scale);
      // The team surface uses the same reason and lifecycle states.
      await p.evaluate(key => pudlWindows.raise(key), teamKey);
      await frames();
      const teamTrace = await team.evaluate(async root => {
        const sample = () => [...root.querySelectorAll('.chat-team, [data-team-member], .team-person, .team-reason, .team-state')].map(el => el.getBoundingClientRect().toJSON());
        const before = sample(), samples = [], failures = [];
        for (const state of ['working','waiting','stuck','idle','closed','elsewhere','unknown']) {
          __listStep(state);
          for (let frame = 0; frame < 8; frame++) { await new Promise(requestAnimationFrame); const now = sample(); samples.push({ state, frame, rectangles: now }); now.forEach((r, i) => { const b = before[i]; if (!b || ['x','y','width','height'].some(k => Math.abs(r[k] - b[k]) > .02)) failures.push({ state, frame, before: b, after: r }); }); }
          const mark = root.querySelector('[data-team-member="bus-alpha"] .team-state');
          const expected = ['closed','elsewhere'].includes(state) ? 'chat-lifecycle' : 'chat-state-' + state;
          if (!mark.classList.contains(expected)) failures.push({ state, painted: mark.className });
        }
        return { before, samples, failures };
      });
      t.ok('team rows keep their one-line slots ' + [theme,width,scale].join('/'), teamTrace.failures.length === 0, JSON.stringify(teamTrace.failures.slice(0, 2)));
      await t.shot('team-row-stability-' + theme + '-' + width + '-' + scale);
      await p.evaluate(key => pudlWindows.raise(key), listKey);
      evidence.push({ theme, width, scale, list: trace, team: teamTrace });
    }
    // A real row menu holds its anchor even when pointer and focus are outside the list.
    await p.setViewportSize({ width: 1440, height: 1000 });
    const workspaceKey = await p.evaluate(() => dinahWorkbench.open('workspace'));
    await p.evaluate(key => pudlWindows.dock(key, null), workspaceKey);
    const workspace = p.locator('[data-win="' + workspaceKey + '"]');
    if (await workspace.locator('[data-win-action="maximize"]').getAttribute('title') === 'Maximize') await workspace.locator('[data-win-action="maximize"]').click();
    const row = p.locator('#workspace-chats [data-session-row="alpha"]');
    await row.hover();
    await row.locator('[data-row-actions]').click();
    await t.shot('list-row-menu');
    fs.writeFileSync(path.join(process.argv[3], 'list-menu-state.json'), JSON.stringify(await p.evaluate(() => ({ marked: [...document.querySelectorAll('[data-menu-for]')].map(el => el.outerHTML), menus: [...document.querySelectorAll('[role="menu"]')].filter(el => el.getBoundingClientRect().height).map(el => el.outerHTML) })), null, 2));
    await p.waitForFunction(() => document.querySelector('#workspace-chats [data-menu-for]'));
    const held = await row.boundingBox();
    await p.mouse.move(1, 1);
    const menuTrace = await row.evaluate(async row => {
      const menu = document.getElementById(row.getAttribute('data-menu-for')), active = document.activeElement;
      const sample = () => [row, ...menu.querySelectorAll('[role="menuitem"]')].map(el => el.getBoundingClientRect().toJSON());
      const before = sample(), samples = [], failures = [];
      __listStep('working');
      for (let frame = 0; frame < 8; frame++) {
        await new Promise(requestAnimationFrame);
        const now = sample(); samples.push({ frame, rectangles: now });
        now.forEach((r, i) => { if (['x','y','width','height'].some(k => Math.abs(r[k] - before[i][k]) > .02)) failures.push({ frame, before: before[i], after: r }); });
        if (document.activeElement !== active || !row.hasAttribute('data-menu-for')) failures.push({ frame, lostMenuOrFocus: true });
      }
      return { before, samples, failures };
    });
    const after = await row.boundingBox();
    t.ok('an open row menu retains its anchor position', ['x','y','width','height'].every(k => Math.abs(held[k] - after[k]) < .02), JSON.stringify({ held, after }));
    t.ok('the open menu and its focused command hold on every frame', menuTrace.failures.length === 0, JSON.stringify(menuTrace.failures.slice(0, 2)));
    await p.keyboard.press('Escape');
    await p.evaluate(() => document.activeElement.blur());
    await p.waitForFunction(() => [...document.querySelectorAll('#workspace-chats [data-session-row]')].map(el => el.dataset.sessionRow).join(',') === 'gamma,alpha,beta');
    t.ok('closing the menu applies the pending regrouping', await row.count() === 1);
    fs.writeFileSync(path.join(process.argv[3], 'list-state-stability.json'), JSON.stringify(evidence, null, 2));
    fs.writeFileSync(path.join(process.argv[3], 'list-menu-geometry.json'), JSON.stringify(menuTrace, null, 2));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
