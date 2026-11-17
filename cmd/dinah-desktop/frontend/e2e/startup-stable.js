// Nothing the person sees moves while the page starts. The page is built
// with its default windows first, and the saved layout arrives from the
// engine some time later; until 17 November 2026 both were shown, so the
// default docks stood beside an empty launcher pane for about a second and
// everything then moved to its saved place. The page now keeps its contents
// out of sight until the saved layout has been drawn (app.css,
// body[aria-busy]; ui-state.js, layoutSettled), and the launcher's pane
// starts closed.
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
    const held = ['.launcher-workspace', '.topbar > *', '.win-dock'].map(selector => { const el = document.querySelector(selector); return el ? getComputedStyle(el).opacity : ''; });
    return { parts, seen: held.every(value => value === '1'), hidden: held.every(value => value === '0' || value === ''), busy: !!document.body && document.body.hasAttribute('aria-busy'), starting: (() => { const el = document.getElementById('status-starting'); return !!el && !el.hidden && getComputedStyle(el).opacity === '1'; })() };
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
      ok('the page was recorded while it started, with its contents out of sight', before.length > 0 && before.every(frame => frame.hidden && frame.busy), JSON.stringify(before.filter(frame => !frame.hidden || !frame.busy).slice(0, 2)));
      ok('the line that says the program is starting is in sight meanwhile', before.some(frame => frame.starting));
      ok('the default windows were built out of sight, in other places than the saved layout has', before.some(frame => text(frame) !== text(last) && frame.parts['window needs-you']), before.map(frame => Object.keys(frame.parts).join(' ')).join(' / '));
      ok('the saved layout is what is shown', !!last.parts['window chat-session'] && !last.parts['window needs-you'] && last.seen && !last.busy, text(last));
      ok('nothing moved once the contents could be seen', seen.length > 0 && seen.every(frame => text(frame) === text(last)), seen.map(frame => frame.t + ' ' + text(frame)).join('\n'));
      ok('the launcher pane never stood empty beside the windows', frames.every(frame => !frame.parts.slot), frames.filter(frame => frame.parts.slot).map(frame => frame.t + ' ' + frame.parts.slot).join(' '));
      // Out of sight, the contents took no pointer; shown, they take it again.
      ok('the contents take the pointer once they are shown', await p.evaluate(() => ['.launcher-workspace', '.win-dock'].every(selector => getComputedStyle(document.querySelector(selector)).pointerEvents !== 'none')));
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
      ok('and they did not move once they could be seen', seen.every(frame => JSON.stringify(frame.parts) === JSON.stringify(last.parts)), seen.map(frame => frame.t + ' ' + JSON.stringify(frame.parts)).join('\n'));
    } finally { await t.close(); }
  }
  // A page whose state cannot be read is still shown, with the reason.
  {
    const t = await launch({ viewport: { width: 1600, height: 1000 }, goto: false, app: () => ({ State: async () => { throw new Error('The state could not be read.'); } }) });
    const { page: p, ok } = t;
    try {
      await p.goto('http://dinah.test/index.html');
      const shown = await p.waitForFunction(() => !document.body.hasAttribute('aria-busy') && getComputedStyle(document.querySelector('.launcher-workspace')).opacity === '1', null, { timeout: 8000 }).then(() => true, () => false);
      ok('a page whose state could not be read is shown all the same', shown);
      ok('and says why', await p.locator('.toast', { hasText: 'The state could not be read.' }).count() > 0);
    } finally { await t.close(); }
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
