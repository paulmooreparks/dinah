// Pointer motion protects a reach; a resting pointer must release regrouping.
const fs = require('fs'), path = require('path');
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 700, height: 1000 }, app: () => {
    const cwd = 'C:/Fixture'; let state = 'idle', version = 1;
    const chats = () => ['alpha','beta'].map(id => ({ sessionId: id, title: 'Chat ' + id, cwd, provider: 'codex', open: true,
      chatState: { life: 'open', window: true, exec: { state: id === 'alpha' ? state : 'working', why: '' } } }));
    const snapshot = () => ({ stateGen: 'pointer-hold', stateVer: version, chats: chats().map(c => ({ sessionId: c.sessionId, ...c.chatState })), groups: [] });
    window.__pointerState = value => { state = value; version++; __emit('agent:chatstate', snapshot()); };
    return { State: async () => ({ workspace: cwd, workspaceId: 'fixture', workspaceRecords: [{ id: 'fixture', name: 'Fixture', sources: [{ id: 'root', kind: 'local-folder', path: cwd }], pins: [] }], recent: [cwd], workspaces: [cwd] }),
      Sessions: async () => chats(), SessionsAt: async () => chats(), ChatStates: async () => snapshot() };
  } });
  const p = t.page, evidence = [];
  try {
    const key = await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.evaluate(key => pudlWindows.dock(key, null), key);
    const win = p.locator('[data-win="' + key + '"]');
    if (await win.locator('[data-win-action="maximize"]').getAttribute('title') === 'Maximize') await win.locator('[data-win-action="maximize"]').click();
    const row = p.locator('#sessions [data-session-row="alpha"]');
    await row.waitFor();
    const tokens = await p.evaluate(() => ['2xs','xs','sm','md','base','lg','xl','2xl','3xl'].map(s => ['--text-' + s, getComputedStyle(document.documentElement).getPropertyValue('--text-' + s)]));
    await p.evaluate(() => {
      const host = document.querySelector('#sessions');
      window.__lastListMotion = 0;
      host.addEventListener('pointermove', () => { __lastListMotion = performance.now(); });
      window.__startPointerTrace = name => {
        const trace = { name, started: performance.now(), lastMotion: __lastListMotion, frames: [], recording: true };
        window.__pointerTrace = trace;
        function read() {
          const row = host.querySelector('[data-session-row="alpha"]');
          trace.frames.push({ at: performance.now(), lastMotion: __lastListMotion, group: row.closest('.browser-group').dataset.section, rect: row.getBoundingClientRect().toJSON(), focus: host.contains(document.activeElement) });
        }
        function sample() { if (!trace.recording) return; read(); requestAnimationFrame(sample); }
        window.__finishPointerTrace = () => { trace.recording = false; read(); return trace; };
        sample();
      };
    });
    const group = name => p.waitForFunction(name => document.querySelector('#sessions [data-session-row="alpha"]').closest('.browser-group').dataset.section === 'chats-' + name, name);
    async function reset() {
      await p.mouse.move(1, 1);
      await p.evaluate(() => { document.activeElement.blur(); __pointerState('idle'); });
      await group('Chats');
    }
    async function finish(name, held) {
      const trace = await p.evaluate(() => __finishPointerTrace());
      if (held) t.ok(name + ' retains its row and geometry', trace.frames.every(f => f.group === 'chats-Chats' && ['x','y','width','height'].every(k => Math.abs(f.rect[k] - trace.frames[0].rect[k]) < .02)), JSON.stringify(trace.frames.filter(f => f.group !== 'chats-Chats').slice(0, 2)));
      evidence.push(trace); return trace;
    }
    for (const theme of ['light','dark']) for (const scale of [1,2]) {
      const width = scale === 1 ? 700 : 390;
      await p.setViewportSize({ width, height: 1000 });
      await p.evaluate(({ theme, tokens, scale }) => { pudlSetTheme(theme); tokens.forEach(([name, value]) => document.documentElement.style.setProperty(name, parseFloat(value) * scale + 'px')); }, { theme, tokens, scale });
      const variant = [theme,width,scale].join('/');
      await reset();
      await row.hover();
      await p.evaluate(() => { document.activeElement.blur(); __startPointerTrace('rest/' + document.documentElement.dataset.theme); __pointerState('waiting'); });
      await group('Needs You');
      const resting = await finish('resting pointer ' + variant, false);
      const first = resting.frames.find(f => f.group === 'chats-Needs You');
      t.ok('a resting pointer releases after two seconds ' + variant, !!first && first.at - resting.lastMotion >= 1995, JSON.stringify({ elapsed: first && first.at - resting.lastMotion }));
      t.ok('the idle pointer case has no focus hold ' + variant, resting.frames.every(f => !f.focus));

      await reset(); await row.hover();
      const area = await row.boundingBox();
      await p.evaluate(() => { __startPointerTrace('moving'); __pointerState('waiting'); });
      for (let i = 0; i < 6; i++) {
        await p.waitForTimeout(450);
        await p.mouse.move(area.x + 20 + i % 2, area.y + 10);
      }
      await finish('moving pointer ' + variant, true);
      await p.mouse.move(1, 1); await group('Needs You');

      await reset();
      const bounds = await p.locator('#sessions').boundingBox();
      await p.mouse.move(bounds.x + 2, bounds.y + 2); await p.mouse.down();
      await p.evaluate(() => { document.activeElement.blur(); __startPointerTrace('pressed'); __pointerState('waiting'); });
      await p.waitForTimeout(2300);
      const pressed = await finish('pressed pointer ' + variant, true);
      t.ok('the pressed pointer is the only hold ' + variant, pressed.frames.every(f => !f.focus));
      await p.mouse.move(1, 1); await p.mouse.up(); await group('Needs You');

      await reset(); await row.locator('a[data-session]').focus();
      await p.evaluate(() => { __startPointerTrace('focused'); __pointerState('waiting'); });
      await p.waitForTimeout(2300);
      await finish('focused row ' + variant, true);
      await p.evaluate(() => document.activeElement.blur()); await group('Needs You');
    }
    // A real menu remains a deliberate hold after its pointer lease expires.
    await p.setViewportSize({ width: 700, height: 1000 });
    const workspaceKey = await p.evaluate(() => dinahWorkbench.open('workspace'));
    await p.evaluate(key => pudlWindows.dock(key, null), workspaceKey);
    const workspace = p.locator('[data-win="' + workspaceKey + '"]');
    if (await workspace.locator('[data-win-action="maximize"]').getAttribute('title') === 'Maximize') await workspace.locator('[data-win-action="maximize"]').click();
    await p.evaluate(() => { document.activeElement.blur(); __pointerState('idle'); });
    const workspaceRow = p.locator('#workspace-chats [data-session-row="alpha"]');
    await workspaceRow.hover(); await workspaceRow.locator('[data-row-actions]').click();
    await p.waitForFunction(() => document.querySelector('#workspace-chats [data-menu-for]'));
    const menuBefore = await workspaceRow.boundingBox();
    await p.mouse.move(1, 1); await p.evaluate(() => __pointerState('waiting'));
    await p.waitForTimeout(2300);
    const menuAfter = await workspaceRow.boundingBox();
    t.ok('an open menu keeps its unlimited hold', await workspaceRow.getAttribute('data-menu-for') !== null && ['x','y','width','height'].every(k => Math.abs(menuBefore[k] - menuAfter[k]) < .02), JSON.stringify({ menuBefore, menuAfter }));
    await p.keyboard.press('Escape'); await p.evaluate(() => document.activeElement.blur());
    await p.waitForFunction(() => document.querySelector('#workspace-chats [data-session-row]').dataset.sessionRow === 'alpha');
    fs.writeFileSync(path.join(process.argv[3], 'list-pointer-hold.json'), JSON.stringify(evidence, null, 2));
  } finally { await p.mouse.up().catch(() => {}); await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
