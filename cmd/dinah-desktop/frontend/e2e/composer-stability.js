// Live composer geometry and provider-aware defaults, sampled on every frame.
const { launch, OUT } = require('./lib/page');
const fs = require('fs'), path = require('path');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => {
      window.__capable = true; window.__primaryCalls = []; window.__queueDecisions = [];
      window.__working = false;
      window.__session = working => ({ sessionId: 'claude:composer', cwd: 'C:/Fixture', provider: 'claude', title: 'Composer', open: true, working,
        queued: [], chatState: { life: 'open', window: true, exec: { state: working ? 'working' : 'idle', why: working ? 'Turn in progress' : 'No work in progress' } } });
      window.__work = working => { __working = working; __emit('agent:session', __session(working)); };
      return {
        OpenSessionAt: async () => __session(false),
        Cancel: async () => {},
        CanSteer: async () => {
          __primaryCalls.push(__capable);
          if (window.__late) return new Promise(resolve => { window.__resolveLate = resolve; });
          if (window.__failPrimary) throw new Error('Fixture capability failure');
          return __capable;
        },
        LogQueueDefault: async (...args) => { __queueDecisions.push(args); },
        SkillsForProviderAt: async () => [], SkillsAt: async () => []
      };
    },
    ready: () => window.dinahWorkbench
  });
  const { page, ok } = t, traces = [];
  try {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'claude:composer' }));
    const win = page.locator('[data-win="' + key + '"]'), box = win.locator('[data-view-id=prompt]');
    await box.waitFor();
    ok('the chat body has no duplicate heading or status line', await win.locator('.win-body h1, .win-body .page-state, .session-actions').count() === 0);
    await page.evaluate(k => { window.__composerRoot = document.querySelector('[data-win="' + k + '"]'); }, key);
    const primary = name => page.waitForFunction(name => __composerRoot.querySelector('[data-view-id="' + name + '"]').classList.contains('btn-primary'), name);
    await page.evaluate(() => __work(true));
    await primary('steer-send');
    ok('a capable working agent defaults to Steer before typing', true);
    await page.evaluate(k => pudlWindows.snap(k, 'maximized'), key);
    ok('the window keeps its accessible name', await page.getByRole('dialog', { name: 'Composer', exact: true }).count() === 1);
    await win.getByRole('img', { name: 'Working: Fixture · Turn in progress', exact: true }).waitFor();
    ok('title status exposes its complete accessible name', true);
    await box.fill('A follow-up'); await primary('steer-send');
    ok('typing rechecks the capable agent', await page.evaluate(() => __primaryCalls.length > 1));
    await page.evaluate(() => { __capable = false; __work(true); }); await primary('send');
    ok('a queue-only working agent defaults to Queue', await win.locator('[data-view-id=send]').textContent() === 'Queue');
    await page.evaluate(() => { __capable = true; });
    await box.press('End'); await box.press('Space'); await primary('steer-send');
    ok('typing observes capability becoming available during an existing turn', true);
    await page.evaluate(() => { __capable = false; __work(true); }); await primary('send');
    await page.evaluate(() => { __capable = true; __work(true); }); await primary('steer-send');
    ok('a session change rechecks capability without typing', true);
    await page.evaluate(() => { __late = true; __work(true); });
    await page.waitForFunction(() => !!window.__resolveLate);
    await page.evaluate(() => { __late = false; __capable = true; __work(true); }); await primary('steer-send');
    await page.evaluate(() => __resolveLate(false));
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(resolve)));
    ok('a superseded false answer cannot replace Steer', await win.locator('[data-view-id=steer-send]').evaluate(el => el.classList.contains('btn-primary')));
    await page.evaluate(() => { __failPrimary = true; __work(true); }); await primary('send');
    const logged = await page.evaluate(() => __queueDecisions.length);
    await box.press('Space'); await primary('send');
    ok('an unchanged failed Queue decision is logged once', await page.evaluate(() => __queueDecisions.length) === logged && await page.evaluate(() => __queueDecisions.some(c => c[2].includes('Fixture capability failure'))));
    await page.evaluate(() => { __failPrimary = false; __capable = true; __work(true); }); await primary('steer-send');
    await box.fill('A follow-up');
    await win.locator('.composer-stop').focus();
    await page.evaluate(() => __work(false));
    await page.waitForFunction(() => document.activeElement === __composerRoot.querySelector('[data-view-id=prompt]'));
    ok('ending a turn with Stop focused returns focus to the prompt', true);
    ok('idle Stop and Steer have no accessible action', await win.getByRole('button', { name: 'Stop', exact: true }).count() === 0 && await win.getByRole('button', { name: 'Steer', exact: true }).count() === 0);
    ok('idle action slots reject pointer targeting and keyboard focus', await page.evaluate(() => ['.composer-stop', '[data-view-id=steer-send]'].every(selector => {
      const el = __composerRoot.querySelector(selector), r = el.getBoundingClientRect();
      const hit = document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2);
      el.focus();
      return el.inert && el.getAttribute('aria-hidden') === 'true' && document.activeElement !== el && hit !== el && !el.contains(hit);
    })));
    await page.evaluate(() => __work(true)); await primary('steer-send');
    await win.locator('[data-view-id=steer-send]').focus();
    await page.evaluate(() => __work(false));
    await page.waitForFunction(() => document.activeElement === __composerRoot.querySelector('[data-view-id=prompt]'));
    ok('ending a turn with Steer focused returns focus to the prompt', true);
    await box.fill('');
    const textBaseline = await page.evaluate(() => {
      const style = getComputedStyle(document.documentElement);
      return { tokens: ['2xs', 'xs', 'sm', 'md', 'base', 'lg', 'xl', '2xl', '3xl'].map(size => ['--text-' + size, style.getPropertyValue('--text-' + size)]).concat([['--chat-size', style.getPropertyValue('--chat-size')]]), prompt: parseFloat(getComputedStyle(__composerRoot.querySelector('textarea')).fontSize) };
    });
    for (const theme of ['light', 'dark']) for (const width of [2100, 1440, 700, 500]) for (const scale of [1, 2]) {
      await page.setViewportSize({ width, height: 1000 });
      await page.evaluate(({ theme, scale, tokens, width, key }) => {
        pudlSetTheme(theme);
        tokens.forEach(([name, value]) => document.documentElement.style.setProperty(name, parseFloat(value) * scale + 'px'));
        pudlWindows.snap(key, 'floating');
        const windowWidth = width === 2100 ? 1000 : width === 1440 ? 680 : width === 700 ? 420 : 320;
        __composerRoot.style.width = windowWidth + 'px';
        __composerRoot.style.height = '800px'; __composerRoot.style.top = '50px';
        __composerRoot.style.left = Math.round((innerWidth - windowWidth) / 2) + 'px';
        __work(false);
      }, { theme, scale, tokens: textBaseline.tokens, width, key });
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      const sample = await page.evaluate(async () => {
        const root = __composerRoot.querySelector('.composer');
        function boxes() {
          const out = {};
          [...root.querySelectorAll('button,textarea,input,select,a'), ...__composerRoot.querySelectorAll('.win-head button')].forEach(el => {
            const r = (el.matches('.composer-stop') ? el.parentElement : el).getBoundingClientRect(); if (!r.width || !r.height) return;
            function structuralKey(node) { const parts = []; while (node && node !== __composerRoot) { parts.unshift(node.tagName + ':' + [...node.parentElement.children].indexOf(node)); node = node.parentElement; } return parts.join('/'); }
            const id = el.dataset.viewId || el.dataset.command || el.dataset.winAction || el.id || structuralKey(el);
            if (out[id]) throw new Error('Duplicate composer identity ' + id);
            out[id] = { x: r.x, y: r.y, width: r.width, height: r.height };
          });
          return out;
        }
        const before = boxes(), frames = [], hint = root.querySelector('textarea').placeholder;
        for (const [phase, working] of [true, false, true, true, true, false].entries()) {
          __work(working);
          if (phase === 3) root.querySelector('.composer-stop').click();
          for (let frame = 0; frame < 4; frame++) {
            await new Promise(requestAnimationFrame);
            const stop = root.querySelector('.composer-stop'), slot = stop.parentElement.getBoundingClientRect(), control = stop.getBoundingClientRect();
            frames.push({ working, boxes: boxes(), stop: { hidden: stop.hidden, inert: stop.inert, text: stop.textContent, slot: slot.toJSON(), control: control.toJSON() }, hint: root.querySelector('textarea').placeholder,
              label: root.querySelector('[data-view-id=send]').textContent });
          }
        }
        return { before, frames, hint, titleReason: __composerRoot.querySelector('.win-execution .win-state').title };
      });
      const actualTextSize = await box.evaluate(el => parseFloat(getComputedStyle(el).fontSize));
      ok('the text-size fixture applies scale ' + scale + ' at ' + width, Math.abs(actualTextSize - textBaseline.prompt * scale) < .02, String(actualTextSize));
      const arrangement = await win.evaluate(win => {
        const row = win.querySelector('.composer-row'), input = row.querySelector('textarea'), actions = row.querySelector('.composer-actions');
        const widest = [...actions.querySelectorAll('button')].map(button => {
          const css = getComputedStyle(button), label = document.createElement('span');
          Object.assign(label.style, { position: 'fixed', visibility: 'hidden', whiteSpace: 'nowrap', font: css.font, letterSpacing: css.letterSpacing });
          label.textContent = button.dataset.reserveLabel || button.textContent;
          document.body.appendChild(label);
          const result = label.getBoundingClientRect().width + ['paddingLeft', 'paddingRight', 'borderLeftWidth', 'borderRightWidth'].reduce((n, key) => n + parseFloat(css[key]), 0);
          label.remove(); return result;
        });
        const bar = document.querySelector('.win-bar');
        return { row: row.getBoundingClientRect().toJSON(), input: input.getBoundingClientRect().toJSON(), actions: actions.getBoundingClientRect().toJSON(), widest: Math.max(...widest), window: win.getBoundingClientRect().toJSON(), viewportBottom: bar ? bar.getBoundingClientRect().top : innerHeight };
      });
      if (width !== 2100 && (width !== 1440 || scale === 2)) ok('narrow input keeps the whole row above its buttons at text ' + scale,
        Math.abs(arrangement.input.width - arrangement.row.width) < 1 && arrangement.actions.top >= arrangement.input.bottom,
        JSON.stringify(arrangement));
      else ok('wide button column fits its widest label and gives the rest to the input at text ' + scale,
        Math.abs(arrangement.actions.width - arrangement.widest) < 1 && arrangement.input.right <= arrangement.actions.left,
        JSON.stringify(arrangement));
      ok('typing actions remain inside the window and viewport at ' + width + '/' + scale,
        arrangement.actions.bottom <= Math.min(arrangement.window.bottom, arrangement.viewportBottom), JSON.stringify(arrangement.actions));
      const toolbar = await win.evaluate(win => {
        const bar = win.querySelector('.composer-toolbar');
        const controls = [...bar.querySelectorAll('button')].filter(el => !el.closest('.menu-panel') && el.getClientRects().length && getComputedStyle(el).visibility !== 'hidden').map(el => {
          const r = el.getBoundingClientRect(), clips = [];
          for (let parent = el.parentElement; parent; parent = parent.parentElement) {
            const css = getComputedStyle(parent), bounds = parent.getBoundingClientRect();
            const x = /hidden|clip|auto|scroll/.test(css.overflowX), y = /hidden|clip|auto|scroll/.test(css.overflowY);
            if ((x && (r.left < bounds.left + parent.clientLeft - .1 || r.right > bounds.left + parent.clientLeft + parent.clientWidth + .1)) ||
                (y && (r.top < bounds.top + parent.clientTop - .1 || r.bottom > bounds.top + parent.clientTop + parent.clientHeight + .1))) clips.push({ className: parent.className, rect: bounds.toJSON() });
          }
          const hit = document.elementFromPoint(r.x + r.width / 2, r.y + r.height / 2);
          return { label: el.textContent.trim() || el.getAttribute('aria-label'), rect: r.toJSON(), clips, hit: hit === el || el.contains(hit), hitTarget: hit && hit.outerHTML.slice(0, 240), pointerEvents: getComputedStyle(el).pointerEvents, command: el.dataset.command, disabled: el.disabled || el.getAttribute('aria-disabled') === 'true', menu: el.classList.contains('menu-btn') };
        });
        const stop = win.querySelector('.composer-stop-slot'), reserve = stop.querySelector('.composer-stop-reserve');
        return { rect: bar.getBoundingClientRect().toJSON(), controls, stopWidth: stop.getBoundingClientRect().width, reserveWidth: reserve.getBoundingClientRect().width };
      });
      ok('all presented toolbar controls fit and are hit-testable before scrolling at ' + width + '/' + scale,
        toolbar.controls.length > 0 && toolbar.controls.every(c => !c.clips.length && (c.disabled || c.hit)), JSON.stringify(toolbar));
      ok('Stop reserves only its longest label width at ' + width + '/' + scale, Math.abs(toolbar.stopWidth - toolbar.reserveWidth) < .02, JSON.stringify(toolbar));
      const menu = toolbar.controls.find(c => c.label === 'More' && !c.disabled && c.hit) || toolbar.controls.find(c => c.menu && !c.disabled && c.hit);
      ok('the toolbar exposes a menu at ' + width + '/' + scale, !!menu, JSON.stringify(toolbar.controls));
      if (menu) {
        await page.mouse.click(menu.rect.x + menu.rect.width / 2, menu.rect.y + menu.rect.height / 2);
        await page.locator('.menu-panel:popover-open').waitFor();
        ok('the presented menu opens its commands without preparatory scrolling at ' + width + '/' + scale, await page.locator('.menu-panel:popover-open .menu-action').count() > 0);
        await page.keyboard.press('Escape');
      }
      const failures = [];
      for (const frame of sample.frames) {
        for (const id of new Set([...Object.keys(sample.before), ...Object.keys(frame.boxes)])) {
          const a = sample.before[id], b = frame.boxes[id];
          if (!a || !b) failures.push({ id, issue: 'unexpected appearance or disappearance' });
          else if (['x', 'y', 'width', 'height'].some(k => Math.abs(a[k] - b[k]) > .02)) failures.push({ id, before: a, after: b });
        }
        if (frame.working && (frame.stop.hidden || frame.stop.inert || ['x', 'y', 'width', 'height'].some(k => Math.abs(frame.stop.slot[k] - frame.stop.control[k]) > .02))) failures.push({ issue: 'the active Stop control does not fill its reserved slot', stop: frame.stop });
        if (!frame.working && (!frame.stop.hidden || !frame.stop.inert)) failures.push({ issue: 'idle Stop remains actionable' });
        if (frame.hint !== sample.hint) failures.push({ issue: 'placeholder changed' });
        if (frame.label !== (frame.working ? 'Queue' : 'Send')) failures.push({ issue: 'incorrect action label' });
      }
      traces.push({ theme, width, scale, textSize: actualTextSize, window: await win.boundingBox(), arrangement, ...sample, failures });
      ok('composer controls stay fixed on every frame, ' + theme + ', ' + width + ' px, text ' + scale, failures.length === 0, JSON.stringify(failures.slice(0, 8)));
      ok('the measured transitions include Stopping at ' + width + '/' + scale, sample.frames.some(frame => frame.stop.text.includes('Stopping')));
      await t.shot('composer-' + theme + '-' + width + '-' + scale + '-idle');
      await page.evaluate(() => __work(true)); await primary('steer-send');
      await t.shot('composer-' + theme + '-' + width + '-' + scale + '-working');
    }
    await page.evaluate(tokens => { tokens.forEach(([name]) => document.documentElement.style.removeProperty(name)); __work(true); }, textBaseline.tokens); await primary('steer-send');
    const continuation = await page.evaluate(async () => {
      const values = [];
      for (let stop = 0; stop < 3; stop++) {
        // A continuation snapshot leaves the reported working state unchanged.
        __work(true);
        for (let i = 0; i < 4; i++) { await new Promise(requestAnimationFrame); values.push({
          label: __composerRoot.querySelector('[data-view-id=send]').textContent,
          steer: __composerRoot.querySelector('[data-view-id=steer-send]').classList.contains('btn-primary'),
          state: __composerRoot.querySelector('.win-execution .win-state').title
        }); }
      }
      return values;
    });
    ok('continuation snapshots keep Working and Steer on every frame', continuation.every(v => v.label === 'Queue' && v.steer && /Working/.test(v.state)), JSON.stringify(continuation));
    fs.writeFileSync(path.join(OUT, 'composer-stability-frames.json'), JSON.stringify({ textBaseline, traces, continuation }, null, 2));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
