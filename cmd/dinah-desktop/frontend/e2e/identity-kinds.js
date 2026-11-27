// Host classification and PUDL projection are separate assertions during adoption.
const { launch, OUT } = require('./lib/page');
const fs = require('fs'), path = require('path');
const destinations = {
  'chat-browser': 'conversation', bus: 'conversation', 'needs-you': 'conversation',
  'org-chart': 'conversation', decisions: 'conversation', 'chat-cleanup': 'conversation',
  'in-progress': 'conversation', workspace: 'tools', 'skills-browser': 'tools', settings: 'tools',
  'theme-editor': 'tools', extensions: 'tools', files: 'files', editor: 'document',
  'content-docs': 'document', 'content-diagrams': 'document', 'content-slides': 'document', terminal: 'tools'
};
(async () => {
  const t = await launch({ viewport: { width: 1600, height: 1000 }, app: () => ({ TerminalShells: async () => ({ shells: [{ id: 'fixture-shell', name: 'Fixture Shell' }], default: 'fixture-shell' }) }) }), p = t.page, observations = [];
  try {
    await p.waitForFunction(() => window.dinahTaskbarPins && window.dinahWorkbench && window.pudlLauncher);
    const authored = await p.evaluate(() => [...document.querySelectorAll('#dinah-launcher [data-launcher-action]')].map(el => ({
      action: el.dataset.launcherAction, kind: el.querySelector('.glyph')?.dataset.identityKind || '', label: el.textContent.trim()
    })));
    fs.writeFileSync(path.join(OUT, 'identity-launcher.json'), JSON.stringify(authored, null, 2));
    for (const entry of authored) {
      const id = entry.action.startsWith('tool:') ? entry.action.slice(5) : entry.action.startsWith('terminal:') ? 'terminal' : entry.action === 'show-workspace' ? 'workspace' : entry.action;
      if (destinations[id]) t.ok('launcher identity ' + entry.action, entry.kind === destinations[id], JSON.stringify(entry));
      else t.ok('an unclassified launcher stays neutral ' + entry.action, entry.kind === '', JSON.stringify(entry));
    }
    const apiOnly = { 'dinah.team': 'conversation', 'dinah.requests': 'conversation', 'dinah.standalone': 'document', 'dinah.document-comments': 'document', 'dinah.windows': 'tools', 'dinah.diagnostics': 'tools', 'com.parkscomputing.dinah-git': 'tools' };
    for (const [id, expected] of Object.entries(apiOnly)) t.ok('bus-only identity ' + id, await p.evaluate(id => dinahWorkbench.identityKind(id), id) === expected);
    // Keep the real workbench registration and window source; provider work is
    // outside this metadata gate. The window's content may report unavailability.
    for (const [kind, expected] of Object.entries(destinations)) {
      const result = await p.evaluate(async ({ kind }) => {
        const item = { kind, workspace: 'C:/Fixture', path: kind === 'files' ? '/workspaces' : kind === 'terminal' ? JSON.stringify(['/', 'identity-terminal', null, 'fixture-shell']) : 'identity-fixture' };
        const key = await dinahWorkbench.open(item);
        const win = dinahHelpers.windowOf(key), ref = dinahWorkbench.pinReference(item);
        const resolved = ref && dinahWorkbench.pinResolve(ref.provider, ref.id);
        return { key, kind: win.dataset.winIdentityKind || '', destination: resolved?.identityKind || '', ref };
      }, { kind });
      observations.push({ ...result, windowKind: kind, expected });
      t.ok('window source identity ' + kind, result.kind === expected, JSON.stringify(result));
      if (kind !== 'terminal') t.ok('resolved taskbar destination identity ' + kind, result.destination === expected, JSON.stringify(result));
      if (result.ref) {
        await p.evaluate(async ref => { await pudlWindows.refreshTaskbarDestinations(); pudlWindows.pinTaskbar(ref.id, true); }, result.ref);
        await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
        const pin = await p.locator('[data-win-pin="' + result.ref.id + '"] .glyph').evaluateAll(els => els.map(el => el.dataset.identityKind || ''));
        t.ok('provider taskbar pin identity ' + kind, pin.includes(expected), JSON.stringify({ ref: result.ref, pin }));
        await p.evaluate(ref => pudlWindows.pinTaskbar(ref.id, false), result.ref);
      }
      await p.evaluate(key => pudlWindows.dock(key, null), result.key);
      await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
      const taskbar = await p.locator('[data-win-dock] [data-win-tab="' + result.key + '"] .glyph').evaluateAll(els => els.map(el => el.dataset.identityKind || ''));
      if (kind !== 'workspace') t.ok('generated taskbar identity ' + kind, taskbar.includes(expected), JSON.stringify(taskbar));
      await p.evaluate(key => pudlWindows.dock(key, 'left'), result.key);
      await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
      const dock = await p.locator('.win-dock-tablist [data-dock-key="' + result.key + '"] .glyph').evaluateAll(els => els.map(el => el.dataset.identityKind || ''));
      t.ok('generated dock identity ' + kind, dock.includes(expected), JSON.stringify(dock));
    }
    const editor = observations.find(o => o.windowKind === 'editor');
    const rekeyed = await p.evaluate(key => dinahWorkbench.rekey(key, { kind: 'files', path: '/workspaces/reclassified', workspace: 'C:/Fixture' }), editor.key);
    t.ok('a changed window kind is classified through the production rekey path', await p.locator('[data-win="' + rekeyed + '"]').getAttribute('data-win-identity-kind') === 'files');
    const changed = observations.find(o => o.windowKind === 'settings');
    await p.evaluate(key => { dinahHelpers.windowOf(key).dataset.winIdentityKind = 'document'; }, changed.key);
    await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
    t.ok('a root identity change updates the existing dock glyph', await p.locator('.win-dock-tablist [data-dock-key="settings"] .glyph[data-identity-kind="document"]').count() > 0);
    await p.evaluate(() => dinahHelpers.windowOf('settings').removeAttribute('data-win-identity-kind'));
    await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
    t.ok('removing the root kind removes its generated classification', await p.locator('.win-dock-tablist [data-dock-key="settings"] .glyph[data-identity-kind]').count() === 0);
    t.ok('action and status glyphs remain neutral', await p.locator('[data-win-action][data-identity-kind], [data-win-action] [data-identity-kind], [data-row-actions][data-identity-kind], [data-row-actions] [data-identity-kind], .win-statuses [data-identity-kind], .win-item-state [data-identity-kind], .menubar-panel [data-identity-kind], .start-search [data-identity-kind]').count() === 0);
    t.ok('unknown third-party identities remain neutral', await p.evaluate(() => ['example.unknown', '__proto__', 'constructor'].every(id => dinahWorkbench.identityKind(id) === '')));
    const geometry = () => [...document.querySelectorAll('.win, .win-dock-group, .win-bar, #dinah-launcher [data-launcher-action]')].map(el => {
      const r = el.getBoundingClientRect(); return [el.dataset.win || el.dataset.launcherAction || el.className, r.x, r.y, r.width, r.height];
    });
    await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
    const classifiedGeometry = await p.evaluate(geometry);
    await p.evaluate(() => {
      window.__identityAttributes = [...document.querySelectorAll('[data-win-identity-kind], .glyph[data-identity-kind]')].map(el => {
        const name = el.matches('.glyph') ? 'data-identity-kind' : 'data-win-identity-kind', value = el.getAttribute(name);
        el.removeAttribute(name); return { el, name, value };
      });
    });
    await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
    const neutralGeometry = await p.evaluate(geometry);
    t.ok('classification moves no window, dock, taskbar or launcher row', JSON.stringify(classifiedGeometry) === JSON.stringify(neutralGeometry), JSON.stringify({ classifiedGeometry, neutralGeometry }));
    await p.evaluate(() => __identityAttributes.forEach(({ el, name, value }) => { if (el.isConnected) el.setAttribute(name, value); }));
    fs.writeFileSync(path.join(OUT, 'identity-geometry.json'), JSON.stringify({ classifiedGeometry, neutralGeometry }, null, 2));
    fs.writeFileSync(path.join(OUT, 'identity-window-projections.json'), JSON.stringify(observations, null, 2));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
