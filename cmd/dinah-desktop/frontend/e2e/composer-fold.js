// A chat's composer toolbar folds as its window narrows. The Agent group
// (provider, mode and model) folds into one menu button whose menu keeps each
// choice as a submenu (PUDL 0.78.0), and widening the window unfolds it. A
// change to what the toolbar holds while one of its menus is open folds it
// again once the menu closes.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      const catalog = [
        { id: 'mode', category: 'mode', name: 'Session Mode', type: 'select', currentValue: 'code', options: [{ value: 'code', name: 'Code' }, { value: 'ask', name: 'Ask' }] },
        { id: 'model', category: 'model', name: 'Model', type: 'select', currentValue: 'one', options: [{ value: 'one', name: 'Model One' }, { value: 'two', name: 'Model Two' }] }];
      window.__calls = [];
      return {
        State: async () => ({ workspace: 'C:/Work/parser', recent: ['C:/Work/parser'], defaultProvider: 'devin', catalog, defaults: { mode: 'code', model: 'one' }, pinned: ['one'] }),
        NewSessionAt: async () => ({ sessionId: 'sa', cwd: 'C:/Work/parser', title: 'Parser Hunt', open: true, working: false, queued: [] }),
        SetSessionOption: async () => catalog, Send: async (id, text) => { window.__calls.push(['Send', id, text]); },
        Queue: async () => {}, Steer: async () => {}, Unqueue: async () => {}, Cancel: async () => {},
        AgentProviders: async () => [{ id: 'devin', name: 'Devin', available: true, installed: true }, { id: 'claude', name: 'Claude', available: true, installed: true }],
      };
    },
  });
  const { page: p, ok } = t;
  try {
    const chat = p.locator('.win[data-win="chat"]');
    await chat.locator('.composer-toolbar').waitFor();
    await p.evaluate(() => window.pudlWindows.snap('chat', 'floating'));
    const agent = chat.locator('.composer-toolbar [data-toolbar-group="Agent"]');
    const width = w => p.evaluate(w => { const win = document.querySelector('.win[data-win="chat"]'); win.style.width = w + 'px'; }, w);
    await width(900);
    const settled = await p.waitForFunction(() => document.querySelector('.win[data-win="chat"] [data-toolbar-group="Agent"]').getAttribute('data-toolbar-stage') === '1').then(() => true, () => false);
    const wide = await agent.evaluate(g => ({ stage: g.getAttribute('data-toolbar-stage'), buttons: g.querySelectorAll('[data-command]:not([hidden])').length }));
    ok('a wide composer shows provider, mode and model as their own buttons', settled && wide.buttons === 3, JSON.stringify(wide));
    await width(340);
    await p.waitForFunction(() => Number(document.querySelector('.win[data-win="chat"] [data-toolbar-group="Agent"]').getAttribute('data-toolbar-stage')) > 1);
    const fold = agent.locator('.toolbar-fold > button');
    ok('a narrow composer folds the Agent group into one button', Number(await agent.getAttribute('data-toolbar-stage')) > 1 && await fold.isVisible() && (await fold.getAttribute('aria-label')) === 'Agent');
    await fold.click();
    const panel = agent.locator('.toolbar-fold > .menu-panel');
    await panel.waitFor();
    const sections = await panel.locator('.md-section-label').allTextContents();
    const checked = await panel.locator('[aria-checked="true"]').allTextContents();
    ok('its menu keeps the provider, the mode and the model, each with its choice ticked', ['Provider', 'Mode', 'Model'].every(s => sections.includes(s)) &&
      ['Devin', 'Code', 'Model One'].every(c => checked.some(t => t.startsWith(c))), JSON.stringify({ sections, checked }));
    await panel.locator('.menu-action', { hasText: 'Ask' }).click();
    await p.waitForFunction(() => /Ask/.test(document.querySelector('.win[data-win="chat"] [data-command="mode"]').getAttribute('aria-label')));
    ok('a choice made in the folded menu is the chat\'s choice', true);
    await width(900);
    await p.waitForFunction(() => document.querySelector('.win[data-win="chat"] [data-toolbar-group="Agent"]').getAttribute('data-toolbar-stage') === '1');
    ok('widening the window unfolds the group', true);

    // Every control has a glyph, so each group shows its controls as glyphs
    // alone before any group folds into a menu, and the Agent group is not
    // the first to fold (28 October 2026).
    const glyphs = await chat.evaluate(w => Array.from(w.querySelectorAll('.composer-toolbar .toolbar-group > [data-command], .composer-toolbar .toolbar-group > .menu > [data-command]'))
      .filter(c => !c.hidden).map(c => c.getAttribute('data-command') + ':' + (c.querySelector(':scope > .glyph') ? c.querySelector(':scope > .glyph').style.getPropertyValue('--glyph') : 'none')));
    ok('every control in the composer toolbar has a glyph', glyphs.length >= 4 && glyphs.every(g => !g.endsWith(':none')), JSON.stringify(glyphs));
    ok('the provider, the mode and the model have glyphs of their own', ['provider:var(--glyph-app)', 'mode:var(--glyph-shield)', 'model:var(--glyph-tag)'].every(g => glyphs.includes(g)), JSON.stringify(glyphs));
    const read = () => chat.evaluate(w => {
      const bar = w.querySelector('.composer-toolbar'), st = {};
      bar.querySelectorAll(':scope > .toolbar-group').forEach(g => { if (g.getClientRects().length || g.getAttribute('data-toolbar-stage') === '5') st[g.getAttribute('data-toolbar-group')] = Number(g.getAttribute('data-toolbar-stage')); });
      const shown = Array.from(bar.querySelectorAll('[data-command]')).filter(c => c.getClientRects().length).map(c => ({ id: c.getAttribute('data-command'), name: c.getAttribute('aria-label'), title: c.getAttribute('title'),
        words: (() => { const x = c.querySelector(':scope > .toolbar-words'); return !!x && getComputedStyle(x).display !== 'none'; })() }));
      return { st, shown };
    });
    const seen = [];
    for (let w = 1200; w >= 300; w -= 20) {
      await width(w);
      await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
      await p.waitForTimeout(30);
      seen.push(Object.assign({ w }, await read()));
    }
    const at = w => seen.find(x => x.w === w);
    console.log('stages ' + [1200, 900, 700, 520, 380].map(w => w + ': ' + JSON.stringify(at(w).st)).join('; '));
    ok('no group folds into a menu while another still shows its words', seen.every(x => !Object.values(x.st).some(s => s >= 3) || Object.values(x.st).every(s => s >= 2)), JSON.stringify(seen.map(x => [x.w, x.st])));
    const glyphOnly = seen.find(x => x.st.Agent === 2);
    ok('the Agent group shows its controls as glyphs alone before it folds', !!glyphOnly && glyphOnly.shown.filter(c => ['provider', 'mode', 'model'].includes(c.id)).every(c => !c.words) &&
      glyphOnly.shown.some(c => c.id === 'model'), JSON.stringify(glyphOnly));
    ok('as glyphs alone they keep their words as their names and tooltips', !!glyphOnly && glyphOnly.shown.some(c => c.id === 'model' && c.name === 'Model: Model One' && c.title === 'Model\nModel One') &&
      glyphOnly.shown.some(c => c.id === 'mode' && /^Mode: /.test(c.name) && /^Mode\n/.test(c.title)) && glyphOnly.shown.some(c => c.id === 'provider' && c.name === 'Provider: Devin'), JSON.stringify(glyphOnly && glyphOnly.shown));
    // The Message group folds first: wherever the Agent group is folded into
    // a menu, the Message group is folded at least as far.
    const ahead = seen.filter(x => x.st.Agent >= 3 && !(x.st.Message >= x.st.Agent));
    ok('the Agent group is not the first to fold into a menu', seen.some(x => x.st.Agent >= 3) && !ahead.length, JSON.stringify(ahead));
    ok('every control in sight has its accessible name at every width', seen.every(x => x.shown.every(c => c.name && c.name.length > 1)), JSON.stringify(seen.filter(x => x.shown.some(c => !c.name))));
    await width(700);
    await p.waitForTimeout(200);
    await t.shot('composer-fold-glyphs');
    await width(900);
    await t.shot('composer-fold');

    // A change made while one of the toolbar's menus is open folds the
    // toolbar once the menu closes (28 October 2026). PUDL does not fold a
    // toolbar while one of its menus is open, and before 0.93.1 it forgot
    // the fold it skipped, so the old folding stayed until the window's
    // width changed: a turn starting while the Model menu was open left the
    // Stop button past the end of the row, and a turn ending left the groups
    // folded with room to spare. PUDL 0.93.1 folds the toolbar again as the
    // menu closes, and Dinah no longer does it for PUDL.
    await p.fill('.win[data-win="chat"] [data-view-id=prompt]', 'Find the parser.');
    await p.evaluate(() => document.querySelector('.win[data-win="chat"] [data-view-id=send]').click());
    await p.waitForFunction(() => window.__calls.some(c => c[0] === 'Send'));
    const session = working => p.evaluate(working => window.__emit('agent:session', { sessionId: 'sa', cwd: 'C:/Work/parser', title: 'Parser Hunt', open: true, queued: [], working, background: [], subagents: [] }), working);
    await session(false);
    const cur = '.win[data-chat-current]';
    const curKey = await p.locator(cur).getAttribute('data-win');
    await p.evaluate(k => window.pudlWindows.snap(k, 'floating'), curKey);
    // The window stands clear of the docks at the page's sides.
    const curWidth = w => p.evaluate(([k, w]) => { const el = document.querySelector('.win[data-win="' + k + '"]'); el.style.left = Math.round((innerWidth - w) / 2) + 'px'; el.style.width = w + 'px'; }, [curKey, w]);
    const settle = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(() => setTimeout(r, 150)))));
    const folding = () => p.evaluate(k => {
      const bar = document.querySelector('.win[data-win="' + k + '"] .composer-toolbar'), st = {};
      bar.querySelectorAll(':scope > .toolbar-group:not([hidden])').forEach(g => { st[g.getAttribute('data-toolbar-group')] = Number(g.getAttribute('data-toolbar-stage')); });
      const stop = bar.querySelector('[data-command="stop-turn"]');
      return { st, fits: bar.scrollWidth <= bar.clientWidth + 1, stop: !!stop && !stop.hidden };
    }, curKey);
    const open = (st) => Object.values(st).every(s => s === 1);
    // The narrowest width at which the toolbar holds every group open
    // without the Stop button.
    let roomy = 0;
    for (let w = 900; w >= 300; w -= 10) {
      await curWidth(w); await settle();
      if (!open((await folding()).st)) break;
      roomy = w;
    }
    await curWidth(roomy); await settle();
    const idle = await folding();
    const viaMenu = async change => {
      await p.evaluate(k => document.querySelector('.win[data-win="' + k + '"] .composer-toolbar [data-command="model"]').click(), curKey);
      await p.waitForSelector(cur + ' .composer-toolbar .menu-panel:popover-open');
      await change();
      await settle();
      await p.evaluate(k => document.querySelectorAll('.win[data-win="' + k + '"] .composer-toolbar .menu-panel:popover-open').forEach(m => m.hidePopover()), curKey);
      await p.waitForFunction(k => !document.querySelector('.win[data-win="' + k + '"] .composer-toolbar .menu-panel:popover-open'), curKey);
      await settle();
      return folding();
    };
    const started = await viaMenu(() => session(true));
    ok('a turn starting while a menu of the toolbar is open folds the toolbar once the menu closes', roomy > 0 && open(idle.st) && !idle.stop &&
      started.stop && started.fits && !open(started.st), JSON.stringify({ roomy, idle, started }));
    // A change of width folds the toolbar for the running turn whatever the
    // case above found, so the case below starts folded.
    await curWidth(roomy + 10); await settle();
    await curWidth(roomy); await settle();
    const busy = await folding();
    const ended = await viaMenu(() => session(false));
    ok('a turn ending while a menu of the toolbar is open unfolds it once the menu closes', busy.stop && !open(busy.st) && !ended.stop && ended.fits && open(ended.st), JSON.stringify({ busy, ended }));
    // The Turn group folds first (Paul, 28 October 2026): with a turn running,
    // wherever the Agent or Message group has gone further than its words,
    // the Turn group has gone at least as far, and some width shows Turn
    // folded while the others keep their words.
    await session(true);
    const turnSeen = [];
    for (let w = 900; w >= 300; w -= 20) {
      await curWidth(w); await settle();
      const f = await folding();
      if (f.st.Turn !== undefined) turnSeen.push({ w, st: f.st });
    }
    const turnBehind = turnSeen.filter(x => (x.st.Agent || 1) > x.st.Turn || (x.st.Message || 1) > x.st.Turn);
    ok('the Turn group folds before the Agent and Message groups', turnSeen.length > 0 && !turnBehind.length && turnSeen.some(x => x.st.Turn >= 2 && x.st.Agent === 1 && x.st.Message === 1), JSON.stringify(turnBehind.length ? turnBehind : turnSeen));
  } catch (err) {
    t.errors.push(String(err && err.stack || err));
  } finally {
    await t.close();
  }
})();
