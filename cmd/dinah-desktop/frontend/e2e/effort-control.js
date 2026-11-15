// The composer's Effort control (docs/design/effort-control.md), against a
// stub of the Go side: the button always says Effort and its mark has a cell
// for each offered level, filled up to the one in effect; nothing in the
// toolbar or composer moves when the level changes; the button is one width
// for one, three and five levels and absent with none; the panel holds only
// the word and the levels; the keyboard path; the folded phone toolbar; and
// what the working line says about a change during a running turn, for each
// program's documented timing (providers.Row.EffortChange).
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const ws = 'C:/Fixture';
    const model = { id: 'model', category: 'model', name: 'Model', type: 'select', currentValue: 'one', options: [{ value: 'one', name: 'Fixture Model' }] };
    const level = (id, current, list) => ({ id, category: 'thought_level', name: 'Reasoning effort', type: 'select', currentValue: current,
      options: list.map(([value, name]) => ({ value, name, description: 'A description that must not be shown for ' + name + '.' })) });
    window.__chats = {
      three: { provider: 'codex', title: 'Three Levels', options: [model, level('reasoning_effort', 'medium', [['low', 'low'], ['medium', 'medium'], ['high', 'high']])] },
      five: { provider: 'claude', title: 'Five Levels', options: [model, level('effort', 'high', [['low', 'Low'], ['medium', 'Medium'], ['high', 'High'], ['xhigh', 'Extra High'], ['max', 'Max']])] },
      one: { provider: 'ollama', title: 'One Level', options: [model, level('think', 'true', [['true', 'On']])] },
      none: { provider: 'codex', title: 'No Setting', options: [model] },
      devin: { provider: 'devin', title: 'Devin Levels', options: [model, level('thinking', 'Normal', [['Low', 'Low'], ['Normal', 'Normal'], ['High', 'High']])] },
      local: { provider: 'ollama', title: 'Local Levels', options: [model, level('think', 'false', [['false', 'Off'], ['true', 'On']])] }
    };
    window.__saved = [];
    const info = id => ({ sessionId: id, cwd: ws, provider: __chats[id].provider, title: __chats[id].title, open: true, working: false, queued: [] });
    return {
      State: async () => ({ workspace: ws, recent: [ws], defaultProvider: 'codex', catalog: [model], defaults: {}, pinned: ['one'] }),
      AgentProviders: async () => ['codex', 'claude', 'ollama', 'devin'].map(id => window.__provider(id, { default: id === 'codex' })),
      Sessions: async () => Object.keys(__chats).map(id => ({ sessionId: id, cwd: ws, provider: __chats[id].provider, title: __chats[id].title })),
      OpenSessionAt: async (w, id) => { setTimeout(() => __emit('agent:options', { sessionId: id, options: __chats[id].options }), 0); return info(id); },
      SetSessionOption: async (id, option, value) => {
        __saved.push([id, option, value]);
        if (window.__refuse) throw new Error('The setting could not be changed.');
        if (window.__delay) await new Promise(r => { window.__release = r; });
        __chats[id].options = __chats[id].options.map(o => o.id === option ? { ...o, currentValue: value } : o);
        return __chats[id].options;
      }
    };
  } });
  const p = t.page, ok = t.ok;
  const frame = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
  const chat = p.locator('.win[data-chat-current]');
  const effort = () => chat.locator('[data-command="effort"]');
  const panel = () => p.locator('.menu-panel:popover-open .effort-panel');
  const open = async title => {
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.locator('#sessions a').filter({ hasText: title }).click();
    if ((await chat.getAttribute('data-win-mode')) !== 'maximized') await chat.locator('[data-win-action="maximize"]').click().catch(() => {});
    const id = await p.evaluate(t => Object.keys(__chats).find(k => __chats[k].title === t), title);
    // The chat's options have arrived once the Model control shows its model.
    await p.waitForFunction(() => { const m = document.querySelector('.win[data-chat-current] [data-command="model"] .toolbar-words'); return m && m.textContent === 'Fixture Model'; }, null, { timeout: 5000 }).catch(async () => { console.log('DEBUG ' + id + ' ' + await p.evaluate(() => document.querySelector('.win[data-chat-current] .composer-toolbar').outerHTML.slice(0, 1500))); });
    await frame(); await frame();
  };
  const mark = () => effort().evaluate(b => { const m = b.querySelector('.effort-mark'); return { cells: m.children.length, on: m.querySelectorAll('[data-on]').length, width: Math.round(m.getBoundingClientRect().width), words: b.querySelector('.toolbar-words').textContent, name: b.getAttribute('aria-label') }; });
  const geometry = () => chat.evaluate(root => Array.from(root.querySelectorAll('.composer-toolbar button, .composer-bar, textarea')).filter(e => e.getClientRects().length).map(e => { const r = e.getBoundingClientRect(); return [r.x, r.y, r.width, r.height].map(Math.round).join(','); }).join(' '));
  const focused = () => p.evaluate(() => { const a = document.activeElement; return a ? (a.dataset.level || a.dataset.command || a.getAttribute('aria-label') || a.tagName) : ''; });
  const panelShape = () => panel().evaluate(el => ({ children: Array.from(el.children).map(c => c.className), text: Array.from(el.querySelectorAll('.md-section-label, button')).map(c => c.textContent), size: Math.round(el.getBoundingClientRect().width) + 'x' + Math.round(el.getBoundingClientRect().height), pressed: Array.from(el.querySelectorAll('[aria-pressed="true"]')).map(b => b.dataset.level) }));
  const working = (id, on) => p.evaluate(([id, on]) => __emit('agent:session', { sessionId: id, cwd: 'C:/Fixture', provider: __chats[id].provider, title: __chats[id].title, open: true, working: on, queued: [], chatState: { life: 'open', window: true, exec: { state: on ? 'working' : 'idle', why: on ? 'Turn in progress' : 'No work in progress' } } }), [id, on]);
  const note = () => chat.evaluate(root => { const n = root.querySelector('.live-work-note'); return n && !n.hidden && n.getClientRects().length ? n.textContent : ''; });
  try {
    // Three levels: the button, the mark and choosing with the pointer.
    await open('Three Levels');
    let m = await mark();
    ok('the button says Effort and its mark has three cells, filled up to medium', m.words === 'Effort' && m.cells === 3 && m.on === 2 && m.width === 26, JSON.stringify(m));
    ok('a screen reader hears Effort and the level', /^Effort, medium, level 2 of 3$/.test(m.name), m.name);
    ok('there is no Thinking button', await chat.locator('[data-command="thinking"]').count() === 0);
    const w3 = Math.round(await effort().evaluate(b => b.getBoundingClientRect().width)), before = await geometry();
    await effort().click(); await panel().waitFor();
    let shape = await panelShape();
    const idle3 = shape.size;
    ok('the panel holds only the word Effort and the three levels, medium pressed', JSON.stringify(shape.text) === JSON.stringify(['Effort', 'low', 'medium', 'high']) && shape.children.length === 2 && shape.pressed.join() === 'medium', JSON.stringify(shape));
    ok('no description is shown', !(await panel().textContent()).includes('description'));
    await t.shot('effort-panel-1440');
    await panel().locator('[data-level="high"]').click(); await frame();
    m = await mark();
    ok('one press chooses, closes the panel and gives focus back to Effort', await panel().count() === 0 && await focused() === 'effort' && m.on === 3);
    ok('the choice uses the ordinary per-chat option call, once', await p.evaluate(() => JSON.stringify(__saved) === JSON.stringify([['three', 'reasoning_effort', 'high']])));
    ok('nothing in the toolbar or composer moved or resized', (await geometry()) === before);
    await effort().click(); await panel().waitFor();
    await panel().locator('[data-level="high"]').click(); await frame();
    ok('choosing the level already in effect sends nothing', await p.evaluate(() => __saved.length === 1));

    // Five, one and no levels.
    await open('Five Levels');
    m = await mark();
    const w5 = Math.round(await effort().evaluate(b => b.getBoundingClientRect().width));
    ok('five levels: five cells, filled up to High', m.cells === 5 && m.on === 3, JSON.stringify(m));
    await open('One Level');
    m = await mark();
    const w1 = Math.round(await effort().evaluate(b => b.getBoundingClientRect().width));
    ok('one level shows the button with one full cell', await effort().isVisible() && m.cells === 1 && m.on === 1);
    ok('the button is one width for one, three and five levels', w1 === w3 && w3 === w5, [w1, w3, w5].join(', '));
    await effort().click(); await panel().waitFor();
    shape = await panelShape();
    ok('the one-level panel holds the word and its one level, pressed', JSON.stringify(shape.text) === JSON.stringify(['Effort', 'On']) && shape.pressed.join() === 'true');
    await p.keyboard.press('Escape'); await frame();
    await open('No Setting');
    ok('a chat with no effort setting shows no button', await effort().evaluate(b => b.hidden || !b.getClientRects().length));

    // The keyboard: reached by Left and Right along the toolbar.
    await open('Five Levels');
    // From the message box, Shift+Tab reaches the toolbar's one stop, and
    // the toolbar's Left and Right keys move along it to Effort.
    await chat.locator('textarea.composer-input').click();
    const inBar = () => p.evaluate(() => !!(document.activeElement && document.activeElement.closest('.win[data-chat-current] .composer-toolbar')));
    for (let i = 0; i < 6 && !(await inBar()); i++) await p.keyboard.press('Shift+Tab');
    for (let i = 0; i < 8 && (await focused()) !== 'effort'; i++) await p.keyboard.press((await p.evaluate(() => { const a = document.activeElement, all = Array.from(document.querySelectorAll('.win[data-chat-current] .composer-toolbar button')).filter(b => b.getClientRects().length); return all.indexOf(a) < all.findIndex(b => b.dataset.command === 'effort'); })) ? 'ArrowRight' : 'ArrowLeft');
    const reached = await focused();
    ok('Shift+Tab and the toolbar’s arrow keys reach Effort from the message box', reached === 'effort', reached);
    await p.keyboard.press('Enter'); await panel().waitFor(); await frame();
    ok('Enter opens the panel with focus on the level in effect', await focused() === 'high');
    await p.keyboard.press('ArrowRight');
    const afterRight = await focused();
    await p.keyboard.press('Home'); const afterHome = await focused();
    await p.keyboard.press('End'); const afterEnd = await focused();
    ok('Right, Home and End move along the levels without choosing', afterRight === 'xhigh' && afterHome === 'low' && afterEnd === 'max' && await p.evaluate(() => __saved.length === 1), [afterRight, afterHome, afterEnd].join(' '));
    await p.keyboard.press('Escape'); await frame();
    ok('Escape closes without a change and gives focus back to Effort', await panel().count() === 0 && await focused() === 'effort' && (await mark()).on === 3);
    await p.keyboard.press('Space'); await panel().waitFor(); await frame();
    await p.keyboard.press('ArrowLeft'); await p.keyboard.press('Space'); await frame();
    ok('Space opens it, Left then Space chooses Medium and closes', await panel().count() === 0 && (await mark()).on === 2 && await focused() === 'effort' && await p.evaluate(() => JSON.stringify(__saved[__saved.length - 1]) === JSON.stringify(['five', 'effort', 'medium'])));

    // A change while a turn runs: the working line says what each program documents.
    const cases = [
      ['Three Levels', 'three', 'low', 'The turn already running keeps high; low applies from the next turn.'],
      ['Five Levels', 'five', 'max', 'Claude doesn’t say whether Max applies to the turn already running.'],
      ['Devin Levels', 'devin', 'High', 'Devin doesn’t say whether High applies to the turn already running.'],
      ['Local Levels', 'local', 'true', 'Ollama uses On from its next request to the model, which may come before this turn ends.']
    ];
    for (const [title, id, to, words] of cases) {
      await open(title);
      await effort().click(); await panel().waitFor();
      const idle = await panelShape(); await p.keyboard.press('Escape'); await frame();
      await working(id, true); await frame();
      await effort().click(); await panel().waitFor();
      const busy = await panelShape();
      await panel().locator('[data-level="' + to + '"]').click(); await frame(); await frame();
      const line = await note();
      ok(title + ': the panel is the same while the turn runs, and says nothing more', busy.size === idle.size && JSON.stringify(busy.text) === JSON.stringify(idle.text), idle.size + ' ' + busy.size);
      ok(title + ': the working line says “' + words + '”', line === words, line);
      await working(id, false); await frame();
      ok(title + ': the note goes when the turn ends', (await note()) === '');
      await working(id, true); await frame();
      ok(title + ': the next turn starts without it', (await note()) === '');
      await working(id, false); await frame();
    }

    // A refused change: the level stays, and the working line claims nothing.
    await open('Three Levels');
    const keep = await p.evaluate(() => __chats.three.options[1].currentValue);
    await working('three', true);
    await p.evaluate(() => { window.__refuse = true; });
    await effort().click(); await panel().waitFor();
    await panel().locator('[data-level="' + (keep === 'low' ? 'medium' : 'low') + '"]').click(); await frame(); await frame();
    ok('a refused change keeps the level and puts nothing on the working line', (await note()) === '' && await p.evaluate(k => __chats.three.options[1].currentValue === k, keep) && (await mark()).on === ['low', 'medium', 'high'].indexOf(keep) + 1, (await note()) + ' / ' + keep);
    await p.evaluate(() => { window.__refuse = false; });
    // A slow answer that arrives after another program's chat came to the front.
    await p.evaluate(() => { window.__delay = true; });
    const fiveBefore = await p.evaluate(() => JSON.stringify(__chats.five.options));
    const target = keep === 'medium' ? 'high' : 'medium';
    await effort().click(); await panel().waitFor();
    await panel().locator('[data-level="' + target + '"]').click(); await frame();
    await open('Five Levels');
    await p.evaluate(() => { window.__delay = false; window.__release(); }); await frame(); await frame();
    ok('the slow answer changed only its own chat, and the chat in front shows no note', await p.evaluate(([t, f]) => __chats.three.options[1].currentValue === t && JSON.stringify(__chats.five.options) === f, [target, fiveBefore]) && (await note()) === '' && (await mark()).on === await p.evaluate(() => __chats.five.options[1].options.findIndex(o => o.value === __chats.five.options[1].currentValue) + 1), JSON.stringify({ note: await note(), mark: await mark() }));
    await open('Three Levels');
    const owned = await note();
    ok('its note, back in its own chat, uses its own program’s wording', owned === 'The turn already running keeps ' + keep + '; ' + target + ' applies from the next turn.', owned);
    await working('three', false); await frame();

    // The phone: the toolbar folds, and the same panel opens in the Agent menu.
    await p.setViewportSize({ width: 390, height: 844 }); await frame();
    await open('Five Levels');
    const agent = chat.locator('.composer-toolbar .menu-btn[aria-label="Agent"]');
    ok('on a phone the Agent group folds into a menu', await agent.isVisible());
    await agent.click(); await panel().waitFor();
    const folded = await panel().evaluate(el => ({ headings: Array.from(el.closest('.menu-panel').querySelectorAll('.md-section-label')).filter(h => h.getClientRects().length && h.textContent === 'Effort').length, levels: el.querySelectorAll('[data-level]').length, fits: el.getBoundingClientRect().right <= innerWidth }));
    ok('the Agent menu holds the five levels under one Effort heading, and fits', folded.headings === 1 && folded.levels === 5 && folded.fits, JSON.stringify(folded));
    await t.shot('effort-folded-390');
    await panel().locator('[data-level="low"]').click(); await frame();
    ok('choosing there sets the level, closes the menu and gives focus back to Agent', await panel().count() === 0 && await focused() === 'Agent' && await p.evaluate(() => JSON.stringify(__saved[__saved.length - 1]) === JSON.stringify(['five', 'effort', 'low'])));
    ok('the page has no sideways overflow', await p.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
