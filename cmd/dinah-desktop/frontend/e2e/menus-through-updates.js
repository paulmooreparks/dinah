// An open menu, submenu or ribbon dropdown stays open, with focus on the
// same row, while a chat in the background keeps changing state and title
// (PUDL 0.96.2). The keyboard goes on from that row, a skin chosen there is
// the skin in use, and the Skin list ticks it alone.
const { launch, PAGE, FRONTEND } = require('./lib/page');
const fs = require('fs'), path = require('path');
const choices = JSON.parse(fs.readFileSync(path.join(FRONTEND, 'pudl/pudl-skins.json'), 'utf8')).skins.filter(s => s.admitted).map(({ id, label }) => ({ id, label }));
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 800 },
    args: { choices },
    app: ({ choices }) => {
      window.__ver = 0;
      window.__info = (state, title) => __emit('agent:session', { sessionId: 'chat-s', title, stateGen: 'g', stateVer: ++__ver, provider: 'codex', cwd: 'C:/Fixture', open: true, working: state === 'working', queued: [],
        chatState: { life: 'open', window: true, exec: { state, why: state } } });
      let look = { version: 1, active: 'mineral', skin: 'standard', themes: [], overrides: {} }, revision = 0;
      const snapshot = () => ({ state: structuredClone(look), revision: String(revision), skins: choices });
      return {
        State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'] }),
        Sessions: async () => [{ sessionId: 'chat-s', title: 'Updating', provider: 'codex', cwd: 'C:/Fixture', open: false }],
        OpenSessionAt: async (w, id) => ({ sessionId: id, title: 'Updating', provider: 'codex', cwd: w, open: true, working: false, queued: [], chatState: { life: 'open', window: true, exec: { state: 'idle', why: '' } } }),
        ReadAppearance: async () => snapshot(),
        SaveAppearance: async (next, expected) => { if (expected !== String(revision)) throw Error('Revision conflict'); look = next; revision++; return snapshot(); },
      };
    },
    goto: false,
  });
  const { page: p, ok } = t;
  const label = id => choices.find(s => s.id === id).label;
  /* What is open, where focus is, and whether any popup closed or focus
     left a row since the last look. */
  const look = () => p.evaluate(() => {
    const a = document.activeElement;
    const r = { panel: !!document.querySelector('#menubar-panel:popover-open'), sub: !!document.querySelector('#menubar-sub:popover-open'),
      drop: !!document.querySelector('#menubar-ribbon-row .menu-panel:popover-open'), focus: a ? (a.textContent || '').trim() : '', same: a.__mark === 1, events: window.__events.slice() };
    window.__events = [];
    return r;
  });
  const burst = async () => { for (let i = 0; i < 6; i++) { await p.evaluate(i => __info(i % 2 ? 'working' : 'waiting', 'Updating ' + i), i); await p.waitForTimeout(120); } };
  const checked = () => p.evaluate(() => [...document.querySelectorAll('[role="menuitemradio"]')].filter(r => r.getClientRects().length && r.getAttribute('aria-checked') === 'true').map(r => r.textContent.trim()));
  try {
    await p.goto(PAGE); await t.waitReady();
    await p.evaluate(() => dinahAppearance.ready);
    await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'chat-s' }));
    await p.locator('.win[data-win-kind="chat-session"] [data-view-id="prompt"]').waitFor();
    await p.evaluate(() => {
      window.__events = [];
      document.addEventListener('toggle', e => { if (e.newState === 'closed' && e.target.matches('.menu-panel')) __events.push('closed ' + (e.target.id || 'panel')); }, true);
      document.addEventListener('focusout', e => { if (e.target.closest && e.target.closest('.menu-panel')) __events.push('focusout ' + e.target.textContent.trim()); }, true);
    });

    // The menu bar: View, Skin, a skin row, by keyboard.
    await p.locator('.menubar-title:visible').filter({ hasText: /^View$/ }).focus();
    await p.keyboard.press('ArrowDown');
    await p.locator('#menubar-panel').waitFor();
    for (let i = 0; i < 30 && (await look()).focus !== 'Skin'; i++) await p.keyboard.press('ArrowDown');
    await p.keyboard.press('ArrowRight');
    await p.locator('#menubar-sub').waitFor();
    await p.keyboard.press('ArrowDown');
    const row = (await look()).focus;
    await p.evaluate(() => { document.activeElement.__mark = 1; window.__events = []; });
    await burst();
    let s = await look();
    ok('menu: chat updates leave the View menu and its Skin list open, focus on ' + row, s.panel && s.sub && s.same && !s.events.length, JSON.stringify(s));
    await p.keyboard.press('ArrowDown');
    const next = (await look()).focus;
    ok('menu: the keyboard goes on from that row', next !== row && next.length > 0, next);
    const target = choices.find(c => c.label === next);
    await p.keyboard.press('Enter');
    if (target) await p.waitForFunction(id => pudlSkins.current() === id, target.id);
    ok('menu: Enter chooses the row as it is now', !!target && (await p.evaluate(() => pudlSkins.current())) === target.id, next);
    for (let i = 0; i < 3; i++) await p.keyboard.press('Escape');
    await p.locator('.menubar-title:visible').filter({ hasText: /^View$/ }).click();
    await p.locator('#menubar-panel [role^="menuitem"]').filter({ hasText: /^Skin$/ }).click();
    await burst();
    ok('menu: the Skin list ticks the chosen skin alone', JSON.stringify(await checked()) === JSON.stringify([target && target.label]), JSON.stringify(await checked()));
    for (let i = 0; i < 3; i++) await p.keyboard.press('Escape');

    // The ribbon: View's tab, the Skin dropdown, a skin row.
    await p.getByRole('button', { name: 'Show as ribbon' }).click();
    await p.locator('[data-menubar] .menubar-title').filter({ hasText: /^View$/ }).click();
    const skin = p.locator('#menubar-ribbon-row .menu > .menu-btn').filter({ hasText: /^Skin$/ }).first();
    await skin.focus();
    await p.keyboard.press('ArrowDown');
    await p.locator('#menubar-ribbon-row .menu-panel:popover-open').waitFor();
    await p.keyboard.press('ArrowDown');
    const drow = (await look()).focus;
    await p.evaluate(() => { document.activeElement.__mark = 1; window.__events = []; });
    await burst();
    s = await look();
    ok('ribbon: chat updates leave the Skin dropdown open, focus on ' + drow, s.drop && s.same && !s.events.length, JSON.stringify(s));
    await p.keyboard.press('ArrowDown');
    const dnext = (await look()).focus;
    const dtarget = choices.find(c => c.label === dnext);
    await p.keyboard.press('Enter');
    if (dtarget) await p.waitForFunction(id => pudlSkins.current() === id, dtarget.id);
    ok('ribbon: the keyboard goes on and Enter chooses the row as it is now', !!dtarget && (await p.evaluate(() => pudlSkins.current())) === dtarget.id, dnext);
    await skin.click();
    await p.locator('#menubar-ribbon-row .menu-panel:popover-open').waitFor();
    await burst();
    ok('ribbon: the Skin dropdown ticks the chosen skin alone', JSON.stringify(await checked()) === JSON.stringify([dtarget && dtarget.label]), JSON.stringify(await checked()));
    await p.keyboard.press('Escape');
    await p.evaluate(() => __info('working', 'After Escape'));
    await p.waitForTimeout(150);
    ok('ribbon: Escape then an update leaves the dropdown closed', !(await look()).drop);
    await t.shot('menus-through-updates');
    await p.getByRole('button', { name: /Show as (menus|menu bar)/ }).click().catch(() => {});
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
