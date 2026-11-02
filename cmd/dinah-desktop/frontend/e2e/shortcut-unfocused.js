// A command shortcut with nothing focused (PUDL 0.96.7). After a chat window
// is activated by its title bar, focus is on the page's body; F2, the chat
// window's Rename Chat shortcut, reaches that window's applet and is taken
// from the browser. A key in a field outside the windows is left alone, and
// a modal dialog keeps its keys. (The menu bar's own shortcuts for an
// applet's contributed menus still need focus in the applet; not checked.)
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1280, height: 860 }, app: () => {
    const cwd = 'C:/Fixture';
    const info = id => ({ sessionId: id, title: 'Chat ' + id, cwd, provider: 'claude', open: true, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } });
    return { State: async () => ({ workspace: cwd, recent: [cwd], workspaceId: 'w1', workspaceRecords: [{ id: 'w1', name: 'Fixture', executionPath: cwd, sources: [] }] }),
      Sessions: async () => [info('one'), info('two')], SessionsAt: async () => [info('one'), info('two')],
      OpenSessionAt: async (w, id) => info(id), SavedConversation: async () => ({ available: false }), ChatLead: async () => null, Team: async () => null };
  } });
  const { page: p, ok } = t;
  // What the page did with the last key: whether something took it from the browser.
  await p.evaluate(() => window.addEventListener('keydown', e => { window.__taken = e.defaultPrevented; }));
  const renaming = () => p.evaluate(() => !!document.querySelector('.win.active input.session-rename, .win.active .chat-rename input, .win.active input[data-view-id="rename"]') || !!document.activeElement && document.activeElement.matches('input') && /rename/i.test(document.activeElement.className + document.activeElement.getAttribute('aria-label')));
  try {
    for (const id of ['one', 'two']) await p.evaluate(id => dinahWorkbench.open({ kind: 'chat-session', path: id, workspace: 'w1' }), id);
    await p.waitForTimeout(800);
    // Activate the first chat by its title bar, leaving nothing focused.
    const first = p.locator('.win[data-win-kind="chat-session"]').filter({ hasText: 'Chat one' }).first();
    await first.locator('.win-title').click();
    await p.evaluate(() => { if (document.activeElement && document.activeElement !== document.body) document.activeElement.blur(); });
    const focus = await p.evaluate(() => ({ body: document.activeElement === document.body, front: document.querySelector('.win.active .win-title').textContent }));
    await p.keyboard.press('F2');
    await p.waitForTimeout(200);
    const taken = await p.evaluate(() => window.__taken), started = await renaming();
    ok('with nothing focused, F2 reaches the front chat window and is taken from the browser', focus.body && /one/.test(focus.front) && taken === true && started, JSON.stringify({ focus, taken, started }));
    await p.keyboard.press('Escape');
    // A field outside the windows keeps the ordinary lookup: the key is left alone.
    await p.evaluate(() => { const i = document.createElement('input'); i.id = 'loose'; i.style.cssText = 'position:fixed;left:0;top:0;'; document.body.appendChild(i); i.focus(); });
    await p.keyboard.press('F2');
    await p.waitForTimeout(150);
    ok('in a field outside the windows, F2 is left to the browser', await p.evaluate(() => window.__taken === false && document.activeElement.id === 'loose'));
    await p.evaluate(() => document.getElementById('loose').remove());
    // A modal dialog keeps its keys.
    await p.evaluate(() => { const d = document.createElement('dialog'); d.id = 'modal'; d.innerHTML = '<p>Modal</p><button>OK</button>'; document.body.appendChild(d); d.showModal(); });
    await p.keyboard.press('F2');
    await p.waitForTimeout(150);
    ok('with a modal dialog open, F2 does not reach the window behind it', await p.evaluate(() => window.__taken === false && document.getElementById('modal').open));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
