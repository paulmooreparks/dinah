// Real workbench placement with invented windows and a mounted chat draft.
const { launch } = require('./lib/page');
const fs = require('node:fs');
const path = require('node:path');
const out = process.argv[3] || path.join(require('node:os').tmpdir(), 'dinah-e2e');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => ({
    AgentProviders: async () => [window.__provider('codex')],
    OpenSessionAt: async (ws, id) => ({ sessionId: id, cwd: ws, provider: 'codex', title: 'Draft Chat', open: true, working: false }),
  }) });
  const p = t.page, observations = [];
  const frame = () => p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  try {
    const chatKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'draft-chat' }));
    const prompt = p.locator('[data-win="' + chatKey + '"] [data-view-id="prompt"]');
    await prompt.fill('Keep this unsent draft through every width.');
    await p.evaluate(async chatKey => {
      window.__narrowPromptNode = document.querySelector('[data-win="' + CSS.escape(chatKey) + '"] [data-view-id="prompt"]');
      dinahWorkbench.register('narrow-fixture', { title: 'Window', glyph: 'app', init(root) { root.innerHTML = '<p>Invented window content.</p>'; return {}; } });
      window.__narrowKeys = [];
      for (let i = 0; i < 6; i++) __narrowKeys.push(await dinahWorkbench.open({ kind: 'narrow-fixture', path: String(i) }));
    }, chatKey);
    await frame();
    const baseline = await p.evaluate(() => ({ place: pudlWindows.state().place, url: location.href }));
    const placementAddress = value => [...new URL(value).searchParams].filter(([key]) => key.startsWith('p.')).sort(([a],[b]) => a.localeCompare(b));
    const docks = ['workspace', 'in-progress', 'chat-browser', 'needs-you'];
    for (const theme of ['light', 'dark']) {
      await p.evaluate(theme => pudlSetTheme(theme), theme);
      await p.setViewportSize({ width: 1440, height: 1000 });
      await p.waitForFunction(() => __narrowKeys.every(key => pudlWindows.effectivePlacement(key).mode === 'floating'));
      await frame();
      const wide = await p.locator('[data-win-kind="narrow-fixture"]').last().boundingBox();
      for (const width of [390, 1440, 390, 641, 640, 390]) {
        await p.setViewportSize({ width, height: width === 1440 ? 1000 : 844 });
        await p.waitForFunction(narrow => __narrowKeys.every(key => pudlWindows.effectivePlacement(key).mode === (narrow ? 'maximized' : 'floating')), width <= 640);
        await frame();
        const actual = await p.evaluate(({ docks, chatKey }) => {
          const box = el => { const r = el.getBoundingClientRect(); return { x: r.x, y: r.y, right: r.right, bottom: r.bottom, width: r.width, height: r.height }; };
          const active = document.querySelector('[data-win="' + CSS.escape(__narrowKeys.at(-1)) + '"]');
          return { width: innerWidth, layer: box(document.querySelector('[data-win-layer]')), window: box(active), close: box(active.querySelector('[data-win-action="close"]')),
            place: pudlWindows.state().place, url: location.href,
            docks: docks.map(key => ({ key, policy: document.querySelector('[data-win="' + key + '"]').dataset.winNarrow, mode: pudlWindows.effectivePlacement(key).mode })),
            mounted: __narrowPromptNode === document.querySelector('[data-win="' + CSS.escape(chatKey) + '"] [data-view-id="prompt"]') };
        }, { docks, chatKey });
        observations.push({ theme, wide, ...actual });
        const label = theme + ' at ' + width;
        t.ok('title-bar controls stay in the workspace, ' + label, actual.close.x >= actual.layer.x - 1 && actual.close.right <= actual.layer.right + 1);
        if (width === 1440) t.ok('widening restores the rendered rectangle, ' + theme, Math.abs(actual.window.x-wide.x)<=1 && Math.abs(actual.window.y-wide.y)<=1 && Math.abs(actual.window.width-wide.width)<=1 && Math.abs(actual.window.height-wide.height)<=1);
        if (width === 640 || width === 641) t.ok('the boundary uses the actual layer width, ' + label, actual.layer.width === width);
        t.ok('dock strips keep their policy, ' + label, actual.docks.every(d => d.policy === 'keep' && d.mode.startsWith('dock-')));
        t.ok('saved placement and address survive resizing, ' + label, JSON.stringify(actual.place) === JSON.stringify(baseline.place) && JSON.stringify(placementAddress(actual.url)) === JSON.stringify(placementAddress(baseline.url)));
        t.ok('the chat stays mounted with its unsent draft, ' + label, actual.mounted && await prompt.inputValue() === 'Keep this unsent draft through every width.');
        if (width === 390 || width === 1440) await t.shot('workbench-narrow-' + theme + '-' + width);
      }
    }
    // A new window opened while restricted must also fit, without replacing its canonical placement.
    const fresh = await p.evaluate(() => dinahWorkbench.open({ kind: 'narrow-fixture', path: 'opened-narrow' }));
    await p.waitForFunction(key => pudlWindows.effectivePlacement(key).mode === 'maximized', fresh);
    await frame();
    t.ok('a newly opened narrow window keeps Close reachable', await p.locator('[data-win="' + fresh + '"] [data-win-action="close"]').evaluate(el => { const r=el.getBoundingClientRect(); return r.left>=0 && r.right<=innerWidth; }));
    fs.writeFileSync(path.join(out, 'workbench-narrow-observations.json'), JSON.stringify({ baseline, observations }, null, 2));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
