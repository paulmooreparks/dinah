// A cold start of the real program: hidden, on a fresh isolated home, with nothing typed or clicked first.
// Usage: node cold-start-hidden.cjs <desktop.exe> <agent.exe> <fresh-output-folder>
//
// The browser suites drive the page against a stub that answers every bridge call at once, so they cannot see a
// fault that needs the real program's start-up order. This check starts the program as a person's first launch
// would, waits only for the page to say it is ready, and then reads the page before any input reaches it: the menus
// are drawn, every drawn row agrees with its command's live state, the status area has left its starting state, and
// the first key pressed runs its command. The programs are built as decisions-hidden.cjs says. The stand-in agent
// is never started; the shared helper asks for it.
'use strict';
const { hiddenDesktop } = require('./lib/hidden-desktop.cjs');
const [desktop, agent, out] = process.argv.slice(2);

(async () => {
  const t = await hiddenDesktop({ desktop, agent, out });
  const { check, keep } = t;
  let failure = null;
  try {
    await t.start();
    const page = t.page;

    // The menu bar and the rows the registry draws into it.
    await page.locator('.menubar-ready').waitFor();
    const menus = await page.evaluate(() => Array.from(document.querySelectorAll('[data-command-menu]')).map(list => ({
      menu: list.getAttribute('data-command-menu'),
      drawn: Array.from(list.querySelectorAll(':scope > li[data-generated] > button')).map(button => ({ label: button.textContent, disabled: button.disabled })),
      live: window.dinahCommands.rows(list.getAttribute('data-command-menu')).filter(row => row !== '-' && !row.items).map(row => ({ label: String(row.label), disabled: !!row.disabled })),
    })));
    keep({ menus });
    for (const name of ['chat', 'documents', 'view', 'help']) {
      const menu = menus.find(m => m.menu === name);
      check('The ' + name + ' menu has rows with no input.', menu && menu.drawn.length > 0, menu);
    }
    // A row drawn before the program finished starting must have been drawn again since.
    const stale = [];
    for (const menu of menus) for (const row of menu.drawn) {
      const live = menu.live.find(r => r.label === row.label);
      if (live && live.disabled !== row.disabled) stale.push({ menu: menu.menu, label: row.label, drawnDisabled: row.disabled, liveDisabled: live.disabled });
    }
    check('Every drawn menu row agrees with its command with no input.', stale.length === 0, stale);

    // The status area.
    const status = await page.evaluate(() => ({
      starting: !document.getElementById('status-starting') || document.getElementById('status-starting').hidden || getComputedStyle(document.getElementById('status-starting')).display === 'none',
      items: Array.from(document.querySelectorAll('.status-area .status-item')).filter(item => !item.hidden).map(item => ({ id: item.id, label: item.getAttribute('aria-label') || item.title || item.textContent.trim() })),
    }));
    keep({ status });
    check('The status area is no longer starting.', status.starting, status);
    check('The status area shows the workspace, notifications and agent items.', ['status-workspace', 'status-needs', 'status-agent'].every(id => status.items.some(item => item.id === id)), status.items);
    const needs = status.items.find(item => item.id === 'status-needs');
    check('Notifications reports none on a fresh home.', needs && /: none$/i.test(needs.label), needs);

    // The first keys. Nothing has been pressed or clicked before these.
    await page.keyboard.press('F1');
    await page.locator('dialog.keyboard-help[open]').waitFor({ timeout: 15000 }).then(() => check('The first key, F1, opens Keyboard Shortcuts.', true), error => check('The first key, F1, opens Keyboard Shortcuts.', false, String(error)));
    await page.keyboard.press('Escape');
    await page.locator('dialog.keyboard-help').waitFor({ state: 'detached', timeout: 15000 }).catch(() => {});
    const launcher = await page.evaluate(() => { const row = window.dinahCommands.rows('view').find(r => r !== '-' && r.label === 'Launcher'); return row ? { disabled: !!row.disabled } : null; });
    check('The Launcher command is enabled once the page is ready.', launcher && !launcher.disabled, launcher);

    check('The page reported no errors.', t.errors.length === 0, t.errors);
  } catch (error) { failure = error; console.log('FAIL the check could not finish :: ' + String(error && error.stack || error)); }
  await t.finish(failure);
})();
