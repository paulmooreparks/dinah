// A two-row status viewport keeps Dismiss fixed as host statuses change.
const { launch, OUT } = require('./lib/page');
const fs = require('node:fs'), path = require('node:path');
(async () => {
  const t = await launch({ nativeScrollbars: true, viewport: { width: 1100, height: 1000 }, app: () => ({
    OpenSessionAt: async (ws, id) => ({ sessionId: id, cwd: ws, provider: 'codex', title: 'Status Fixture', open: true,
      chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } })
  }) });
  const { page, ok } = t, traces = [];
  try {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'status-fixture' }));
    await page.locator('[data-win="' + key + '"] .win-execution .win-state[data-state="idle"]').waitFor({ state: 'attached' });
    const tokens = await page.evaluate(() => ['--text-xs', '--text-sm', '--text-md', '--text-base', '--text-lg'].map(name => [name, getComputedStyle(document.documentElement).getPropertyValue(name)]));
    for (const theme of ['light', 'dark']) for (const scale of [1, 2]) {
      await page.evaluate(({ key, theme, scale, tokens }) => {
        pudlSetTheme(theme); tokens.forEach(([name, value]) => document.documentElement.style.setProperty(name, parseFloat(value) * scale + 'px'));
        const win = document.querySelector('[data-win="' + key + '"]');
        pudlWindows.snap(key, 'floating'); Object.assign(win.style, { width: '320px', height: '750px' });
        const group = win.querySelector('.win-statuses');
        // These five rows belong to the host, as application status spans do.
        group.querySelectorAll('.win-status:not(.win-execution):not(.chat-working-chip)').forEach(el => { el.hidden = true; });
        for (let i = 1; i <= 5; i++) {
          let row = group.querySelector('[data-fixture-status="' + i + '"]');
          if (!row) { row = document.createElement('span'); row.className = 'win-status'; row.dataset.fixtureStatus = i; group.appendChild(row); }
          row.hidden = false; row.textContent = 'Status ' + i + ': ' + 'Complete detail remains accessible '.repeat(4);
        }
      }, { key, theme, scale, tokens });
      await page.locator('[data-win="' + key + '"] .win-statuses[data-win-status-compact]').waitFor();
      await page.locator('[data-win="' + key + '"] .win-status-toggle').click();
      const panel = page.locator('.win-status-popover:popover-open');
      await panel.waitFor();
      const list = panel.locator('.win-status-rows');
      await list.evaluate(el => { el.scrollTop = 0; });
      const behind = await page.evaluate(() => [...document.querySelectorAll('#work-split, .transcript')].map(el => [el.scrollTop, el.scrollLeft]));
      await list.hover(); await page.mouse.wheel(0, 100);
      await page.waitForFunction(() => document.querySelector('.win-status-popover:popover-open .win-status-rows').scrollTop > 0);
      ok('the status list accepts pointer scrolling with native scrollbars, ' + theme + '/' + scale, true);
      ok('wheel input leaves the split and transcripts behind the popup still, ' + theme + '/' + scale,
        JSON.stringify(behind) === JSON.stringify(await page.evaluate(() => [...document.querySelectorAll('#work-split, .transcript')].map(el => [el.scrollTop, el.scrollLeft]))));
      const result = await panel.evaluate(async panel => {
        const list = panel.querySelector('.win-status-rows'), dismiss = panel.querySelector('button'), before = dismiss.getBoundingClientRect().toJSON(), rows = [], frames = [], failures = [];
        const rect = el => el.getBoundingClientRect().toJSON();
        const lineHeight = parseFloat(getComputedStyle(list).lineHeight), expectedRow = Math.max(22, lineHeight);
        const actual = [...list.querySelectorAll('li')];
        for (const [index, row] of actual.entries()) {
          const position = row.offsetTop - actual[0].offsetTop;
          list.scrollTop = position;
          await new Promise(requestAnimationFrame);
          const r = rect(row), bounds = rect(list), css = getComputedStyle(row);
          if (r.top < bounds.top - 1 || r.bottom > bounds.bottom + 1) failures.push({ index, issue: 'row cannot be reached completely', r, bounds });
          if (r.height < expectedRow - 1 || css.whiteSpace !== 'nowrap' || css.textOverflow !== 'ellipsis') failures.push({ index, issue: 'row clips vertically or does not truncate to one line', r, expectedRow });
          if (index > 0 && !row.textContent.includes('Complete detail remains accessible')) failures.push({ index, issue: 'full details were lost' });
          rows.push({ index, text: row.textContent, rect: r, list: bounds });
        }
        return { before, rows, frames, failures, viewport: rect(list), expectedRow, nativeScrollbars: true, scrollbarWidth: list.offsetWidth - list.clientWidth,
          tabIndex: list.tabIndex, name: list.getAttribute('aria-label'), role: list.getAttribute('role') };
      });
      ok('six status rows are reachable without vertical clipping, ' + theme + '/' + scale, result.rows.length === 6 && result.failures.length === 0, JSON.stringify(result.failures));
      ok('the status viewport reserves two complete rendered rows, ' + theme + '/' + scale,
        result.viewport.height >= 2 * result.expectedRow && result.viewport.height < 3 * result.expectedRow,
        JSON.stringify({ height: result.viewport.height, row: result.expectedRow }));
      const changing = await page.evaluate(async key => {
        const group = document.querySelector('[data-win="' + key + '"] .win-statuses'), panel = document.querySelector('.win-status-popover:popover-open');
        const dismiss = panel.querySelector('button'), before = dismiss.getBoundingClientRect().toJSON(), samples = [], failures = [];
        dismiss.focus();
        for (const visible of [0, 1, 3, 5, 0, 5]) {
          group.querySelectorAll('[data-fixture-status]').forEach((row, index) => { row.hidden = index >= visible; });
          for (let frame = 0; frame < 4; frame++) {
            await new Promise(requestAnimationFrame);
            const r = dismiss.getBoundingClientRect().toJSON();
            if (['x', 'y', 'width', 'height'].some(k => Math.abs(r[k] - before[k]) > .02) || document.activeElement !== dismiss) failures.push({ visible, frame, before, after: r });
            samples.push({ visible, frame, rect: r });
          }
        }
        return { samples, failures };
      }, key);
      ok('Dismiss keeps its geometry and focus as status rows arrive and disappear, ' + theme + '/' + scale, !changing.failures.length, JSON.stringify(changing.failures.slice(0, 6)));
      traces.push({ theme, scale, ...result, changing });
      ok('the status region has a keyboard stop and accessible name, ' + theme + '/' + scale, result.tabIndex === 0 && result.role === 'region' && result.name === 'Window status', JSON.stringify(result));
      await list.evaluate(el => { el.scrollTop = 0; });
      await list.focus(); await page.keyboard.press('End');
      await page.waitForFunction(() => { const el = document.querySelector('.win-status-popover:popover-open .win-status-rows'); return el.scrollTop + el.clientHeight >= el.scrollHeight - 1; });
      ok('the keyboard can reach the sixth row, ' + theme + '/' + scale, await list.evaluate(el => el === document.activeElement));
      await page.keyboard.press('Tab');
      ok('Tab reaches Dismiss from the status region, ' + theme + '/' + scale, await panel.getByRole('button', { name: 'Dismiss', exact: true }).evaluate(el => el === document.activeElement));
      await t.shot('status-popup-' + theme + '-' + scale);
      await panel.evaluate(panel => {
        const r = panel.querySelector('button').getBoundingClientRect(), behind = document.createElement('button');
        behind.id = 'status-underlay-action'; behind.type = 'button'; behind.textContent = 'Underlying action'; behind.tabIndex = -1;
        Object.assign(behind.style, { position: 'fixed', left: r.x + 'px', top: r.y + 'px', width: r.width + 'px', height: r.height + 'px', zIndex: '2147483646', opacity: '0' });
        window.__underlayClicks = 0; behind.addEventListener('click', () => { __underlayClicks++; }); document.body.appendChild(behind);
      });
      await panel.getByRole('button', { name: 'Dismiss', exact: true }).click();
      await panel.waitFor({ state: 'hidden' });
      ok('Dismiss returns focus to the opener, ' + theme + '/' + scale, await page.locator('[data-win=\"' + key + '\"] .win-status-toggle').evaluate(el => el === document.activeElement));
      ok('Dismiss closes without activating a control directly underneath, ' + theme + '/' + scale, await page.evaluate(() => __underlayClicks === 0));
      await page.locator('#status-underlay-action').evaluate(el => el.remove());
    }
  } finally {
    fs.mkdirSync(OUT, { recursive: true }); fs.writeFileSync(path.join(OUT, 'status-popup-geometry.json'), JSON.stringify(traces, null, 2));
    await t.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
