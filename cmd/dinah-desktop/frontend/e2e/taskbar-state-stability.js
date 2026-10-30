// PUDL owns execution marks. State changes preserve taskbar and popup controls.
const { launch, OUT } = require('./lib/page');
const fs = require('node:fs'), path = require('node:path');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    state: { workspace: 'C:/Fixture', workspaceId: 'fixture', workspaceRecords: [{ id: 'fixture', name: 'Fixture', executionPath: 'C:/Fixture', sources: [] }] },
    app: () => {
      window.__stateVersion = 0;
      window.__stateChat = (id, state) => ({ sessionId: id, title: id, provider: 'codex', cwd: 'C:/Fixture', open: state !== 'closed', working: state === 'working', queued: [],
        chatState: { life: state === 'closed' ? 'closed' : 'open', window: true,
          ...(state === 'unknown' ? { elsewhere: 'Running in another Dinah window' } : state === 'closed' ? {} : { exec: { state, why: state === 'stuck' ? 'The provider cannot run this chat until its settings are corrected' : state === 'waiting' ? 'Waiting for your approval' : 'Fixture reason' } }) } });
      return { OpenSessionAt: async (ws, id) => __stateChat(id, 'idle'), CanSteer: async () => true };
    }
  });
  const { page: p, ok } = t, traces = [];
  try {
    const keys = await p.evaluate(async () => {
      const keys = [];
      for (const id of ['State One', 'State Two']) keys.push(await dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: id }));
      return keys;
    });
    await p.evaluate(keys => {
      window.__stateKeys = keys;
      window.__stateTokens = [...document.styleSheets].length && ['--text-2xs', '--text-xs', '--text-sm', '--text-md', '--text-base', '--text-lg', '--text-xl', '--text-2xl', '--text-3xl', '--chat-size'].map(name => [name, parseFloat(getComputedStyle(document.documentElement).getPropertyValue(name))]);
      window.__stateWindows = keys.map(k => document.querySelector('[data-win="' + CSS.escape(k) + '"]'));
      window.__transition = state => {
        // Publish complete snapshots in a fixed order, after constructing both.
        const snapshots = ['State One', 'State Two'].map(id => Object.assign(__stateChat(id, state), { stateGen: 'states', stateVer: ++__stateVersion }));
        snapshots.forEach(info => __emit('agent:session', structuredClone(info)));
      };
      window.__geometry = () => {
        const out = {};
        const add = (id, el) => { const r = el.getBoundingClientRect(); out[id] = [r.x, r.y, r.width, r.height]; };
        document.querySelectorAll('nav.win-dock[data-win-dock-omit-docked] > button').forEach(el => add('taskbar:' + (el.dataset.winTab || el.dataset.winStack), el));
        document.querySelectorAll('.win-stack-panel button, .win-stack-panel a').forEach(el => add('stack:' + el.tagName + ':' + (el.dataset.winTab || el.dataset.winAction || el.getAttribute('aria-label')), el));
        __stateWindows.forEach((win, i) => win.querySelectorAll('.win-head button').forEach((el, j) => add('chrome:' + i + ':' + (el.dataset.winAction || j), el)));
        document.querySelectorAll('.win-status-popover:popover-open button').forEach((el, i) => add('status:' + i, el));
        return out;
      };
    }, keys);
    for (const theme of ['light', 'dark']) for (const width of [1440, 700]) for (const scale of [1, 2]) {
      await p.setViewportSize({ width, height: 1000 });
      await p.evaluate(({ theme, scale, width }) => {
        pudlSetTheme(theme);
        __stateTokens.forEach(([name, value]) => { if (Number.isFinite(value)) document.documentElement.style.setProperty(name, value * scale + 'px'); });
        __stateKeys.forEach((key, i) => { pudlWindows.snap(key, 'floating'); const win = __stateWindows[i]; Object.assign(win.style, { width: '320px', height: '700px', left: (i * 20 + 20) + 'px', top: (i * 20 + 90) + 'px' }); });
        __transition('idle');
      }, { theme, scale, width });
      await p.waitForFunction(() => __stateWindows.every(win => win.dataset.winState === 'idle'));
      await p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      await p.locator('[data-win="' + keys[1] + '"] .win-statuses[data-win-status-compact]').waitFor();
      const stack = p.locator('nav.win-dock[data-win-dock-omit-docked] > [data-win-stack="fixture"]');
      if (await stack.getAttribute('aria-expanded') !== 'true') await stack.click();
      await p.locator('.win-stack-panel').waitFor();
      await p.evaluate(() => pudlWindows.menuCommands(__stateKeys[1]).find(c => c.id === 'status').run());
      await p.locator('.win-status-popover:popover-open').waitFor();
      await p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      await p.locator('.win-status-popover:popover-open > button').focus();
      const result = await p.evaluate(async () => {
        const before = __geometry(), focused = document.activeElement, frames = [], failures = [];
        for (const state of ['working', 'waiting', 'stuck', 'idle', 'unknown', 'closed', 'working']) {
          __transition(state);
          for (let frame = 0; frame < 6; frame++) {
            await new Promise(requestAnimationFrame);
            const boxes = __geometry();
            for (const id of new Set([...Object.keys(before), ...Object.keys(boxes)])) {
              if (!before[id] || !boxes[id]) failures.push({ state, frame, id, issue: 'control appeared or disappeared' });
              else if (boxes[id].some((value, i) => Math.abs(value - before[id][i]) > 0.02)) failures.push({ state, frame, id, before: before[id], after: boxes[id] });
            }
            const expected = ['unknown', 'closed'].includes(state) ? null : state;
            if (__stateWindows.some(win => win.getAttribute('data-win-state') !== expected)) failures.push({ state, frame, issue: 'wrong engine state' });
            const marks = __stateWindows.map(win => win.querySelector('.win-execution .win-state'));
            if (marks.some(mark => expected ? mark.hidden || mark.dataset.state !== expected : !mark.hidden)) failures.push({ state, frame, issue: 'wrong rendered execution mark' });
            if (document.activeElement !== focused) failures.push({ state, frame, issue: 'status focus was lost' });
            if (document.querySelector('[data-win-running]')) failures.push({ state, frame, issue: 'legacy running attribute' });
            frames.push({ state, frame, boxes, stackOpen: !!document.querySelector('.win-stack-panel'), statusOpen: !!document.querySelector('.win-status-popover:popover-open') });
          }
        }
        return { before, frames, failures };
      });
      traces.push({ theme, width, scale, ...result });
      ok('taskbar, stack, status and chrome stay fixed ' + [theme, width, scale].join('/'), result.failures.length === 0, JSON.stringify(result.failures.slice(0, 12)));
      ok('stack and Window status remain open through every transition', result.frames.every(f => f.stackOpen && f.statusOpen));
      const native = p.locator('[data-win="' + keys[1] + '"]');
      ok('native state label is the short word', await native.getAttribute('data-win-state-label') === 'Working');
      ok('native state detail retains workspace and reason', /Fixture.*Fixture reason/.test(await native.getAttribute('data-win-state-detail')));
      ok('the stack shows the short word with complete accessible context', await p.locator('.win-stack-panel .win-state[data-state="working"]').evaluateAll(marks => marks.length === 2 && marks.every(mark => mark.textContent.trim() === 'Working' && /Fixture.*Fixture reason/.test(mark.getAttribute('aria-label')))));
      await t.shot('taskbar-states-' + theme + '-' + width + '-' + scale);
      await p.keyboard.press('Escape');
      await p.keyboard.press('Escape');
    }
  } finally {
    fs.mkdirSync(OUT, { recursive: true });
    fs.writeFileSync(path.join(OUT, 'taskbar-state-frames.json'), JSON.stringify(traces, null, 2));
    await t.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
