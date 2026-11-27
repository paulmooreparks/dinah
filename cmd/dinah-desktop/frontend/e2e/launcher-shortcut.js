const { launch } = require('./lib/page');
(async () => {
  for (const mac of [false, true]) for (const ribbon of [false, true]) {
    const name = (mac ? 'command' : 'control') + (ribbon ? '-ribbon' : '-menu');
    const t = await launch({ viewport: { width: 1440, height: 1000 }, args: { mac },
      init: ({ mac }) => { if (mac) Object.defineProperty(navigator, 'platform', { get: () => 'MacIntel' }); },
      app: () => {
        const drafts = {}, info = { sessionId: 'shortcut-chat', title: 'Shortcut Chat', cwd: 'C:/Fixture', provider: 'codex', open: true, working: false, queued: [], chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } };
        let n = 0;
        const view = d => Object.assign({}, d, { actions: ['update', 'discard', 'save-as'], spans: {}, suggestions: [], changed: false });
        window.__terminalInput = [];
        return {
          OpenSessionAt: async () => info, Sessions: async () => [info],
          NewEmptyDraft: async () => { const id = 'd' + String(++n).padStart(31, '0'); return view(drafts[id] = { id, text: 'Keyboard fixture', title: 'Fixture.txt', revision: 'r1', origin: { kind: 'new', workspaceId: '' }, baseline: { text: 'Keyboard fixture', title: 'Fixture.txt', revision: 'r1' } }); },
          GetDraft: async id => view(drafts[id]), RetitleDraft: async (id, rev, title) => view(Object.assign(drafts[id], { title })), ListDrafts: async () => [],
          TerminalStart: async () => ({ id: 'shortcut-terminal', name: 'Fixture Shell', folder: 'C:/Fixture' }),
          TerminalWrite: async (...args) => { __terminalInput.push(args); }, TerminalResize: async () => {}, TerminalClose: async () => {}
        };
      }
    });
    const p = t.page, chord = (mac ? 'Meta' : 'Control') + '+Shift+x';
    try {
      await p.waitForFunction(() => window.pudlLauncher && pudlLauncher.state(document.getElementById('dinah-launcher')));
      await p.evaluate(ribbon => pudlMenubar.ribbon(ribbon), ribbon);
      t.ok('the opener metadata comes from the command registry ' + name, await p.evaluate(() => document.querySelector('[data-menubar]').getAttribute('data-menubar-launcher-shortcut') === dinahCommands.rows('view').find(c => c.label === 'Launcher').shortcut));
      if (ribbon) {
        const opener = await p.evaluate(() => { const el = pudlMenubar.launcherButton({ visible: true }); return { tooltip: el.getAttribute('data-tooltip'), keys: el.getAttribute('aria-keyshortcuts') }; });
        t.ok('the ribbon opener tooltip shows the platform key ' + name, /Launcher.*X/.test(opener.tooltip || '') && (opener.tooltip || '').includes(mac ? '⌘' : 'Ctrl'), JSON.stringify(opener));
        t.ok('the ribbon opener exposes the platform accessibility key ' + name, opener.keys === (mac ? 'Meta' : 'Control') + '+Shift+X', JSON.stringify(opener));
      }
      const editorKey = await p.evaluate(() => dinahContentWindows.newDocument(undefined, '', 'editor'));
      await p.locator('.win[data-win-kind="editor"] .cm-content').waitFor();
      const chatKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'shortcut-chat' }));
      await p.locator('.win[data-win-kind="chat-session"] [data-view-id="prompt"]').waitFor();
      const terminalKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'terminal', path: JSON.stringify(['/', 'shortcut-terminal', null, 'fixture-shell']) }));
      await p.locator('[data-terminal-ready="true"] .xterm-helper-textarea').waitFor({ state: 'attached' });
      const sources = [
        ['none', null, null],
        ['editor', editorKey, '.win[data-win-kind="editor"] .cm-content'],
        ['prompt', chatKey, '.win[data-win-kind="chat-session"] [data-view-id="prompt"]'],
        ['terminal', terminalKey, '[data-terminal-ready="true"] .xterm-helper-textarea']
      ];
      for (const [source, key, selector] of sources) {
        if (key) await p.evaluate(key => pudlWindows.raise(key), key);
        if (selector) await p.locator(selector).focus();
        else await p.evaluate(() => document.activeElement.blur());
        await p.evaluate(async () => { for (let frame = 0; frame < 8; frame++) await new Promise(requestAnimationFrame); });
        const before = await p.evaluate(() => {
          window.__shortcutReturn = document.activeElement;
          const rect = el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; };
          return { top: pudlWindows.state().top, rectangles: Array.from(document.querySelectorAll('.win,.win-dock-group')).filter(el => el.getClientRects().length).map(el => [el.dataset.win || el.dataset.dockEdge, rect(el)]),
            scroll: [scrollX, scrollY], input: JSON.stringify(__terminalInput), text: document.querySelector('.cm-content').textContent, prompt: document.querySelector('.win[data-win-kind="chat-session"] [data-view-id="prompt"]').value,
            selectedMenu: document.querySelector('.menubar-title[aria-selected="true"]')?.textContent };
        });
        await p.keyboard.press(chord);
        await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
        const opened = await p.evaluate(() => pudlLauncher.state(document.getElementById('dinah-launcher')).visible);
        t.ok('the shortcut opens Launcher from ' + source + ' ' + name, opened);
        if (!opened) continue;
        t.ok('opening focuses the search field ' + source + ' ' + name, await p.locator('#dinah-launcher [data-start-query]').evaluate(el => el === document.activeElement));
        await p.keyboard.press(chord);
        await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
        t.ok('the second press closes Launcher ' + source + ' ' + name, await p.evaluate(() => !pudlLauncher.state(document.getElementById('dinah-launcher')).visible));
        t.ok('closing restores prior focus ' + source + ' ' + name, await p.evaluate(source => document.activeElement === (source === 'none' ? pudlMenubar.launcherButton({ visible: true }) : __shortcutReturn), source));
        const after = await p.evaluate(() => {
          const rect = el => { const r = el.getBoundingClientRect(); return [r.x, r.y, r.width, r.height]; };
          return { top: pudlWindows.state().top, rectangles: Array.from(document.querySelectorAll('.win,.win-dock-group')).filter(el => el.getClientRects().length).map(el => [el.dataset.win || el.dataset.dockEdge, rect(el)]),
            scroll: [scrollX, scrollY], input: JSON.stringify(__terminalInput), text: document.querySelector('.cm-content').textContent, prompt: document.querySelector('.win[data-win-kind="chat-session"] [data-view-id="prompt"]').value,
            selectedMenu: document.querySelector('.menubar-title[aria-selected="true"]')?.textContent };
        });
        t.ok('the chord changes no window, text, scroll or terminal input ' + source + ' ' + name, JSON.stringify(before) === JSON.stringify(after), JSON.stringify({ before, after }));
      }
      const listing = await p.evaluate(() => ({ command: dinahCommands.searchable().filter(c => c.id === 'toggle-launcher'), menu: dinahCommands.rows('view').filter(c => c.label === 'Launcher'), keys: pudlMenubar.shortcuts().filter(c => c.key === 'Mod+Shift+X') }));
      t.ok('the registry exposes one Launcher command and one bound shortcut ' + name, listing.command.length === 1 && listing.menu.length === 1 && listing.keys.length === 1, JSON.stringify(listing));
      t.ok('command search shows the platform key ' + name, listing.command[0]?.shortcut === (mac ? 'Cmd' : 'Ctrl') + '+Shift+X', JSON.stringify(listing.command));
      await p.evaluate(() => dinahCommands.run('keyboard-shortcuts'));
      await p.locator('.keyboard-help').waitFor();
      t.ok('Keyboard Shortcuts lists Launcher with its key ' + name, await p.locator('.keyboard-help tr').filter({ hasText: 'Launcher' }).filter({ hasText: (mac ? 'Cmd' : 'Ctrl') + '+Shift+X' }).count() === 1);
      await p.locator('.keyboard-help [data-close]').click();
      if (ribbon) {
        await p.locator('.menubar-title').filter({ hasText: /^View$/ }).click();
        t.ok('the command tooltip shows its registered key ' + name, await p.locator('[data-tooltip]').evaluateAll((els, mac) => els.some(el => el.getClientRects().length && /Launcher.*X/.test(el.dataset.tooltip) && el.dataset.tooltip.includes(mac ? '⌘' : 'Ctrl')), mac));
      }
    } finally { await t.close(); }
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
