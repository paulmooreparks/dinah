// Nothing the person sees moves while the page starts. The page is built
// while its saved layout arrives from the engine; until 17 November 2026
// both the default and saved layouts were shown, so the
// default docks stood beside an empty launcher pane for about a second and
// everything then moved to its saved place. The start screen is now in front
// of the page until the saved layout has been drawn (app.css, .start-screen;
// ui-state.js, layoutSettled), and the launcher's pane starts closed.
//
// Every frame from the first is recorded: where the parts are, and whether
// they can be seen. What is checked is that every frame in which they can be
// seen has them in the same places.
const { launch } = require('./lib/page');
const record = () => {
  window.__frames = [];
  const box = el => { if (!el) return ''; const r = el.getBoundingClientRect(); return r.width || r.height ? [r.left, r.top, r.width, r.height].map(Math.round).join(',') : ''; };
  const snap = () => {
    const parts = {};
    [['slot', '#launcher-slot'], ['space', '.dinah-window-space'], ['topbar', '.topbar'], ['bar', '.win-bar'], ['dock', '.win-dock'], ['status', '.status-area']].forEach(([name, selector]) => { parts[name] = box(document.querySelector(selector)); });
    document.querySelectorAll('.win-layer > .win').forEach(win => { const b = box(win); if (b) parts['window ' + win.dataset.winKind] = b; });
    // The start screen covers the whole page, in front of it, until the page has started.
    const screen = document.getElementById('start-screen'), area = screen ? screen.getBoundingClientRect() : null;
    const covering = !!area && area.width >= innerWidth && area.height >= innerHeight && getComputedStyle(screen).display !== 'none' && document.elementFromPoint(innerWidth / 2, 8) && screen.contains(document.elementFromPoint(innerWidth / 2, 8)) && screen.contains(document.elementFromPoint(8, innerHeight - 8));
    // A frame may be taken while the start screen's own parts are still being read from the page,
    // on a slow machine, and a part that is not there yet has no words.
    const words = (root, selector) => { const el = root.querySelector(selector); return el ? el.textContent : ''; };
    // What each part of the status area is, how wide, and its words. This is printed with a frame
    // that moved, to say which part of the status area changed; it is not compared.
    const chips = Array.from(document.querySelectorAll('.status-area > *')).filter(el => el.getBoundingClientRect().width > 0).map(el => (el.id || el.className) + ' ' + Math.round(el.getBoundingClientRect().width) + ' ' + JSON.stringify(el.textContent.trim().replace(/\s+/g, ' '))).join(', ');
    return { parts, chips, seen: !!screen && !covering, hidden: covering, busy: !!document.body && document.body.hasAttribute('aria-busy'), starting: covering && /\S/.test(words(screen, '.start-screen-words')) && words(screen, '.start-screen-name').trim() !== '' };
  };
  let last = '';
  (function tick() {
    const frame = snap(), text = JSON.stringify(frame);
    if (text !== last) { last = text; frame.t = Math.round(performance.now()); window.__frames.push(frame); }
    requestAnimationFrame(tick);
  })();
};
(async () => {
  // A saved layout that differs from the default one, read after a wait as the engine's is.
  {
    const t = await launch({
      viewport: { width: 1600, height: 1000 }, goto: false,
      app: () => {
        const key = 'item-' + btoa(JSON.stringify(['chat-session', 'C:/Fixture', 'owned'])).replace(/\+/g, '-').replace(/\//g, '_').replace(/=+$/, '');
        const layout = '?open=workspace,chat-browser,' + key + '&top=' + key + '&z.order=workspace,chat-browser,' + key;
        return {
          ReadUIState: () => new Promise(resolve => setTimeout(() => resolve({ version: 1, values: { 'dinah-workbench-layout': layout } }), 300)),
          SavedChats: async () => ({ generation: 'layout', chats: [{ chatId: 'owned', sessionId: 'owned', cwd: 'C:/Fixture', provider: 'devin', title: 'Kept chat', resolution: 'resolved' }] }),
          DeclaredProviders: async () => [__provider('devin')],
          ResolveSessionContinuation: async id => id,
          SavedConversation: async () => ({ available: false }),
          OpenSessionAt: async (cwd, sessionId) => ({ cwd, sessionId, provider: 'devin', title: 'Kept chat', open: true, chatState: { life: 'open', exec: { state: 'idle' } } }),
        };
      },
      init: record,
    });
    const { page: p, ok } = t;
    try {
      await p.goto('http://dinah.test/index.html');
      await p.waitForFunction(() => !document.body.hasAttribute('aria-busy'));
      await p.waitForTimeout(1500);
      const frames = await p.evaluate(() => window.__frames);
      const text = frame => JSON.stringify(frame.parts);
      const seen = frames.filter(frame => frame.seen), before = frames.filter(frame => !frame.seen && frame.parts.space);
      const last = frames[frames.length - 1];
      ok('the page was recorded while it started, with its contents out of sight', before.length > 0 && before.every(frame => frame.hidden), JSON.stringify(before.filter(frame => !frame.hidden).slice(0, 2)));
      ok('the start screen shows the name of the product and says what is happening meanwhile', before.some(frame => frame.starting));
      ok('an unfinished layout was recorded behind the start screen', before.some(frame => text(frame) !== text(last)), before.map(frame => Object.keys(frame.parts).join(' ')).join(' / '));
      ok('the saved layout is what is shown', !!last.parts['window chat-session'] && !last.parts['window needs-you'] && last.seen && !last.busy, text(last));
      ok('nothing moved once the contents could be seen', seen.length > 0 && seen.every(frame => text(frame) === text(last)), seen.map(frame => frame.t + ' ' + text(frame) + ' status: ' + frame.chips).join(' // '));
      ok('the launcher pane never stood empty beside the windows', frames.every(frame => !frame.parts.slot), frames.filter(frame => frame.parts.slot).map(frame => frame.t + ' ' + frame.parts.slot).join(' '));
      ok('the start screen is gone, and a press reaches the page', await p.evaluate(() => getComputedStyle(document.getElementById('start-screen')).display === 'none' && !!document.elementFromPoint(innerWidth / 2, innerHeight / 2).closest('.dinah-workbench')));
      ok('the start screen had the logo, the name and a line of progress', await p.evaluate(() => { const s = document.getElementById('start-screen'); return !!s.querySelector('img.start-screen-logo') && s.querySelector('.start-screen-name').textContent === 'Dinah' && !!s.querySelector('progress') && s.querySelector('.start-screen-words').textContent === 'Opening your windows…'; }));
      await t.shot('startup-stable');
    } finally { await t.close(); }
  }
  // A page with no saved layout is shown with the default one.
  {
    const t = await launch({ viewport: { width: 1600, height: 1000 }, goto: false, init: record });
    const { page: p, ok } = t;
    try {
      await p.goto('http://dinah.test/index.html');
      await p.waitForFunction(() => !document.body.hasAttribute('aria-busy'));
      await p.waitForTimeout(800);
      const frames = await p.evaluate(() => window.__frames), last = frames[frames.length - 1];
      const seen = frames.filter(frame => frame.seen);
      ok('with no saved layout the default windows are shown', last.seen && !!last.parts['window workspace'], JSON.stringify(last.parts));
      ok('and Chats is the pane in front in its dock, with Notifications behind it', !!last.parts['window chat-browser'] && !last.parts['window needs-you'], JSON.stringify(last.parts));
      ok('and they did not move once they could be seen', seen.every(frame => JSON.stringify(frame.parts) === JSON.stringify(last.parts)), seen.map(frame => frame.t + ' ' + JSON.stringify(frame.parts) + ' status: ' + frame.chips).join(' // '));
    } finally { await t.close(); }
  }
  // The page's scripts are deferred, and the saved state can arrive before the last of them has run.
  // Here PUDL's window script is held back until the state has been restored and twelve more frames
  // have been drawn, as a loaded machine holds it. Until 28 November 2026 the start screen lifted in
  // those frames, over a page with no windows, and the windows and the bar then moved in plain sight.
  {
    const t = await launch({ viewport: { width: 1600, height: 1000 }, goto: false, init: record });
    const { page: p, ok } = t;
    try {
      let release;
      const held = new Promise(resolve => { release = resolve; });
      await t.context.route('http://dinah.test/pudl/pudl-windows.js', async route => { await held; await route.fallback(); });
      await p.goto('http://dinah.test/index.html', { waitUntil: 'commit' });
      await p.waitForFunction(() => document.documentElement.dataset.uiStateReady === 'true');
      await p.evaluate(() => new Promise(resolve => { let left = 12; (function next() { if (left-- <= 0) resolve(); else requestAnimationFrame(next); })(); }));
      const early = await p.evaluate(() => window.__frames.filter(frame => frame.seen).length);
      release();
      await p.waitForFunction(() => !document.body.hasAttribute('aria-busy') && getComputedStyle(document.getElementById('start-screen')).display === 'none');
      // Thirty more frames are recorded, in which a late change would show.
      await p.evaluate(() => new Promise(resolve => { let left = 30; (function next() { if (left-- <= 0) resolve(); else requestAnimationFrame(next); })(); }));
      const frames = await p.evaluate(() => window.__frames), last = frames[frames.length - 1];
      const seen = frames.filter(frame => frame.seen);
      ok('the start screen stays up while a script of the page has still to run', early === 0, String(early));
      ok('and the page is first seen with its default windows', seen.length > 0 && !!seen[0].parts['window workspace'] && !!seen[0].parts['window chat-browser'], JSON.stringify(seen[0] && seen[0].parts));
      ok('and nothing moved once it could be seen', seen.every(frame => JSON.stringify(frame.parts) === JSON.stringify(last.parts)), seen.map(frame => frame.t + ' ' + JSON.stringify(frame.parts) + ' status: ' + frame.chips).join(' // '));
    } finally { await t.close(); }
  }
  // A page whose state cannot be read is still shown, with the reason.
  {
    const t = await launch({ viewport: { width: 1600, height: 1000 }, goto: false, app: () => ({ State: async () => { throw new Error('The state could not be read.'); } }) });
    const { page: p, ok } = t;
    try {
      await p.goto('http://dinah.test/index.html');
      const shown = await p.waitForFunction(() => !document.body.hasAttribute('aria-busy') && getComputedStyle(document.getElementById('start-screen')).display === 'none', null, { timeout: 8000 }).then(() => true, () => false);
      ok('a page whose state could not be read is shown all the same', shown);
      ok('and says why', await p.locator('.toast', { hasText: 'The state could not be read.' }).count() > 0);
    } finally { await t.close(); }
  }
  // The start screen does not wait for the agent programs to be found: a
  // scan that never answers leaves the page in sight and in use, with the
  // status bar still saying that the program is starting.
  {
    const t = await launch({ viewport: { width: 1600, height: 1000 }, goto: false, app: () => ({ AgentProviders: () => new Promise(() => {}), DeclaredProviders: () => new Promise(() => {}) }) });
    const { page: p, ok } = t;
    try {
      await p.goto('http://dinah.test/index.html');
      const shown = await p.waitForFunction(() => getComputedStyle(document.getElementById('start-screen')).display === 'none', null, { timeout: 8000 }).then(() => true, () => false);
      ok('a scan of the agent programs that never answers does not keep the start screen up', shown);
      ok('the page still counts as starting, and the status bar’s own line went with the start screen', await p.evaluate(() => document.body.hasAttribute('aria-busy') && document.getElementById('status-starting').hidden));
      await p.locator('#status-workspace').click({ timeout: 4000 }).then(() => ok('and the page can be used', true), error => ok('and the page can be used', false, String(error).slice(0, 200)));
    } finally { await t.close(); }
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
