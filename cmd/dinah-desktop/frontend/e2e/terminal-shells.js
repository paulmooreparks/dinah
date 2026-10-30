// A Terminal running a real shell: it starts the shell chosen in Settings,
// draws what the shell sends even before the start call returns, sends typed
// text in order, follows its size, copies a selection with Ctrl+C, says when
// the shell ends and starts it again on Enter, and closes the shell with the
// window. Tools › New Terminal lists the other shells.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspace: 'C:/Work/parser', workspaces: ['C:/Work/parser'], recent: ['C:/Work/parser'] },
    app: () => {
      const emit = (name, data) => window.__emit(name, data);
      const b64 = s => btoa(unescape(encodeURIComponent(s)));
      const shells = { shells: [{ id: 'pwsh', name: 'PowerShell', path: 'C:/pwsh.exe' }, { id: 'git-bash', name: 'Git Bash', path: 'C:/Git/bin/bash.exe' }, { id: 'dinah', name: 'Dinah Shell', path: '' }], default: 'pwsh' };
      let serial = 0;
      window.__end = (id, code) => emit('terminal:exit', { id, code });
      window.__draw = (id, text) => emit('terminal:output', { id, data: b64(text) });
      return {
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        StatWorkspaceMount: async p => ({ path: p, directory: true }),
        TerminalShells: async () => shells,
        SetTerminalShell: async id => { __calls.push(['default', id]); shells.default = id; },
        TerminalStart: async (shell, ref, cols, rows) => {
          const id = 'term-' + (++serial);
          __calls.push(['start', shell, cols, rows]);
          // The shell draws its prompt before the start call returns.
          emit('terminal:output', { id, data: b64('PS C:\\Work\\parser> ') });
          await new Promise(r => setTimeout(r, 50));
          return { id, shell, name: shell === 'pwsh' ? 'PowerShell' : 'Git Bash', folder: 'C:\\Work\\parser' };
        },
        TerminalWrite: async (id, data) => {
          __calls.push(['write', id, data]);
          await new Promise(r => setTimeout(r, 20));
          if (data.includes('\r')) emit('terminal:output', { id, data: b64('\r\nhello from ' + id + '\r\nPS C:\\Work\\parser> ') });
        },
        TerminalResize: async (id, cols, rows) => { __calls.push(['resize', id, cols, rows]); },
        TerminalClose: async id => { __calls.push(['close', id]); },
        ClipboardRead: async () => ({ html: '', text: 'pasted words' })
      };
    },
    init: () => { window.__calls = []; window.__copied = []; window.dinahHostCopy = null; },
    ready: () => document.querySelector('[data-win="workspace"]'),
  });
  const { page: p, ok, errors } = t;
  try {
    await p.waitForFunction(() => window.dinahShellTerminal && window.dinahShellTerminal.shells().length === 3);
    await p.evaluate(() => { window.dinahHost.copy = text => __copied.push(text); });

    await p.evaluate(() => window.dinahResources.openTerminal('/workspaces'));
    const win = p.locator('.win[data-win-kind="terminal"]').last();
    await win.locator('[data-terminal-ready="true"]').waitFor();
    ok('terminal reaches the content edges without padding', await win.locator('.xterm').evaluate(el => { const css = getComputedStyle(el), screen = el.parentElement.getBoundingClientRect(), rect = el.getBoundingClientRect(); return ['paddingTop', 'paddingRight', 'paddingBottom', 'paddingLeft'].every(k => css[k] === '0px') && Math.abs(rect.left - screen.left) < 1 && Math.abs(rect.top - screen.top) < 1 && Math.abs(rect.width - screen.width) < 1 && Math.abs(rect.height - screen.height) < 1; }));
    ok('a new Terminal runs the shell chosen in Settings, sized to its window', await p.evaluate(() => { const c = __calls.find(c => c[0] === 'start'); return c && c[1] === 'pwsh' && c[2] > 20 && c[3] > 5; }));
    await win.locator('.xterm-rows', { hasText: 'PS C:\\Work\\parser>' }).waitFor();
    ok('what the shell drew before its start call returned is shown', true);
    ok('the window is titled with the shell and its folder', (await win.locator('.win-title').innerText()) === 'PowerShell');

    await win.locator('.xterm').click();
    await p.keyboard.type('Get-Date');
    await p.keyboard.press('Enter');
    await win.locator('.xterm-rows', { hasText: 'hello from term-1' }).waitFor();
    const typed = await p.evaluate(() => __calls.filter(c => c[0] === 'write').map(c => c[2]).join(''));
    ok('typed text reaches the shell whole and in order', typed === 'Get-Date\r', JSON.stringify(typed));

    // A program running in the shell titles the window, and its taskbar
    // button, with the title it sets (OSC 0 or 2).
    const taskName = () => p.evaluate(() => { const k = document.querySelector('.win[data-win-kind="terminal"]').dataset.win, b = document.querySelector('[data-win-tab="' + CSS.escape(k) + '"]'); return b ? b.innerText.replace(/\s+/g, ' ').trim() : ''; });
    await p.evaluate(() => __draw('term-1', '\x1b]0;vim notes.md\x07'));
    await p.waitForFunction(() => document.querySelector('.win[data-win-kind="terminal"] .win-title').innerText === 'vim notes.md');
    const shownTask = await taskName();
    ok('the program running in the shell titles the window and its taskbar button', shownTask.startsWith('vim notes.md'), shownTask);

    await p.evaluate(() => { const w = document.querySelector('.win[data-win-kind="terminal"]'); w.style.width = '900px'; w.style.height = '600px'; });
    await p.waitForFunction(() => __calls.some(c => c[0] === 'resize'));
    ok('a change of size reaches the shell', true);

    await win.locator('.xterm').dblclick({ position: { x: 40, y: 8 } });
    await p.keyboard.press('Control+c');
    await p.waitForTimeout(200);
    const copied = await p.evaluate(() => __copied.slice());
    const interrupts = await p.evaluate(() => __calls.filter(c => c[0] === 'write' && c[2] === '\x03').length);
    ok('Ctrl+C with text selected copies it rather than interrupting', copied.length === 1 && copied[0].length > 0 && interrupts === 0, JSON.stringify({ copied, interrupts }));
    await p.keyboard.press('Control+c');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'write' && c[2] === '\x03'));
    ok('Ctrl+C with nothing selected goes to the shell', true);
    await p.keyboard.press('Control+Shift+v');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'write' && c[2].includes('pasted words')));
    ok('Ctrl+Shift+V pastes the clipboard', true);

    await p.evaluate(() => { window.go.main.App.SaveAppearance = async state => ({ state, revision: 'terminal-font-test' }); });
    await p.evaluate(() => dinahAppearance.overrides({ terminal: { font: { families: ['Courier New'], fallback: 'monospace' }, size: 17 } }));
    await p.waitForFunction(() => getComputedStyle(document.querySelector('.xterm-rows')).fontSize === '17px');
    ok('font changes update the running terminal without restarting its shell', await win.locator('.xterm-rows').evaluate(el => getComputedStyle(el).fontFamily.includes('Courier New')) && await p.evaluate(() => __calls.filter(c => c[0] === 'start').length === 1));
    await p.evaluate(() => dinahAppearance.overrides({}));

    await p.evaluate(() => __end('term-1', 3));
    await win.locator('.xterm-rows', { hasText: 'The shell ended with code 3' }).waitFor();
    await p.keyboard.press('Enter');
    await p.waitForFunction(() => __calls.filter(c => c[0] === 'start').length === 2);
    await win.locator('[data-terminal-id="term-2"]').waitFor();
    ok('when the shell ends Enter starts it again', true);

    await p.evaluate(() => pudlWindows.close(document.querySelector('.win[data-win-kind="terminal"]').dataset.win));
    await p.waitForFunction(() => __calls.some(c => c[0] === 'close' && c[1] === 'term-2'));
    ok('closing the window closes its shell', true);

    await p.locator('[data-menubar] button', { hasText: 'Tools' }).first().click();
    const sub = p.locator('#menubar-panel').getByText('New Terminal');
    ok('Tools offers one New Terminal submenu', await sub.count() === 1);
    ok('Default Shell precedes specific shell choices', await p.locator('#host-tools').evaluate(el => { const row = [...el.children].find(li => li.firstChild?.textContent === 'New Terminal'); return row?.querySelector('ul button')?.textContent === 'Default Shell' && !el.textContent.includes('New Terminal With'); }));
    await p.keyboard.press('Escape');

    await p.evaluate(() => window.dinahWorkbench.open('settings'));
    await p.locator('#settings-tab-terminal').click();
    ok('Settings shows the shells and the chosen one', (await p.locator('#set-terminal-shell').inputValue()) === 'pwsh' && (await p.locator('#set-terminal-shell option').count()) === 3);
    await p.locator('#set-terminal-shell').selectOption('git-bash');
    await p.waitForFunction(() => __calls.some(c => c[0] === 'default' && c[1] === 'git-bash'));
    ok('choosing a shell in Settings makes it the default', true);
    await t.shot('terminal-shells');
  } catch (err) {
    errors.push(String(err && err.stack || err));
  } finally {
    await t.close();
  }
})();
