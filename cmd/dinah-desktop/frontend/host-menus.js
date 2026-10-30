/* Host menus use PUDL's source lists and public window commands. The
   Documents, View, Window and Help rows come from the command registry
   (commands.js); Tools comes from the launchers. */
(function () {
  'use strict';
  var bench = window.dinahWorkbench;
  var tools = document.getElementById('host-tools'), windows = document.getElementById('host-windows');
  function command(list, definition) {
    var li = document.createElement('li');
    li.dataset.generated = '';
    if (definition === '-') li.textContent = '-';
    else if (definition.items) {
      li.appendChild(document.createTextNode(definition.label));
      if (definition.mnemonic) li.dataset.mnemonic = definition.mnemonic;
      if (definition.glyph) li.dataset.menuGlyph = definition.glyph;
      var nested = document.createElement('ul'); li.appendChild(nested);
      definition.items.forEach(function (item) { command(nested, item); });
    } else if (definition.heading) li.textContent = definition.heading;
    else {
      var button = document.createElement('button'); button.type = 'button'; button.textContent = definition.label;
      button.disabled = !!definition.disabled;
      if (typeof definition.checked === 'boolean') button.setAttribute('aria-checked', String(definition.checked));
      // PUDL reads a markup row's extras from these (0.61.0): a destructive
      // command drawn as one, a glyph, a second line, and for a disabled row
      // the reason it cannot run.
      if (definition.mnemonic) button.dataset.mnemonic = definition.mnemonic;
      if (definition.danger) button.dataset.menuDanger = '';
      if (definition.glyph) button.dataset.menuGlyph = definition.glyph;
      if (definition.glyphOnly) button.setAttribute('data-menu-glyph-only', '');
      if (definition.detail) button.dataset.menuDetail = definition.detail;
      if (definition.disabled && definition.reason) button.dataset.menuReason = definition.reason;
      if (definition.shortcut) button.dataset.shortcut = definition.shortcut;
      button.addEventListener('click', function () { Promise.resolve().then(definition.run).catch(function (error) { document.dispatchEvent(new CustomEvent('dinah:menu-error', { detail: error })); }); });
      li.appendChild(button);
    }
    list.appendChild(li);
    return li;
  }
  function drawTools() {
    tools.replaceChildren();
    bench.launchers().filter(function (tool) { return tool.id !== 'terminal'; }).sort(function (a, b) { return a.title.localeCompare(b.title); }).forEach(function (tool) { command(tools, { label: tool.title, mnemonic: tool.mnemonic, glyph: tool.glyph, run: tool.run }); });
    var terminal = bench.launchers().find(function (tool) { return tool.id === 'terminal'; });
    var shells = window.dinahShellTerminal ? window.dinahShellTerminal.shells() : [];
    var choices = [{ label: 'Default Shell', run: terminal ? terminal.run : function () {}, disabled: !terminal }, '-'];
    shells.forEach(function (shell) { choices.push({ label: shell.name, run: function () { return window.dinahResources.currentPath().then(function (path) { return window.dinahResources.openTerminal(path, null, shell.id); }); } }); });
    if (window.dinahShellTerminal) { choices.push('-'); choices.push({ label: 'Refresh Shells', run: window.dinahShellTerminal.refresh }); }
    command(tools, { label: 'New Terminal', mnemonic: 'N', glyph: 'terminal', items: choices });
    command(tools, '-'); command(tools, { label: 'Find a Tool…', mnemonic: 'D', glyph: 'search', run: function () { window.dinahCommands.run('find-tool'); } });
  }
  function drawThemes() {
    var list = document.getElementById('host-themes'), appearance = window.dinahAppearance;
    if (!list || !appearance) return;
    list.replaceChildren();
    var state = appearance.snapshot().state;
    Object.entries(appearance.builtins).forEach(function (entry) { command(list, { label: entry[1].name + ' (built in)', checked: state.active === entry[0], run: function () { return appearance.select(entry[0]); } }); });
    if (state.themes.length) command(list, '-');
    state.themes.forEach(function (theme) { command(list, { label: theme.document.name, checked: state.active === theme.id, run: function () { return appearance.select(theme.id); } }); });
    command(list, '-');
    command(list, { label: 'Edit Current Theme…', run: function () { document.getElementById('appearance-edit').click(); } });
    command(list, { label: 'Appearance Settings…', run: function () { return bench.open('settings').then(function () { document.getElementById('settings-tab-appearance').click(); }); } });
  }
  /* A registry menu's rows go into its host list: all of it, or after the
     list's placeholder (li[data-commands]) where some rows are written in
     the page. */
  function drawRegistry(menu) {
    var list = document.querySelector('[data-command-menu="' + menu + '"]');
    if (!list) return;
    list.querySelectorAll(':scope > li[data-generated]').forEach(function (li) { li.remove(); });
    var marker = list.querySelector(':scope > li[data-commands]'), at = marker ? marker.nextSibling : null;
    window.dinahCommands.rows(menu).forEach(function (row) {
      var li = command(list, row);
      if (marker) list.insertBefore(li, at);
    });
  }
  function drawWindows() {
    var api = window.pudlWindows; if (!api) return;
    var state = api.state(); windows.replaceChildren();
    /* The front window's own commands are PUDL's; its Close is the
       registry's, which has the key. */
    if (state.top && !state.min[state.top]) api.menuCommands(state.top).filter(function (item) { return !['page', 'copy-link', 'close'].includes(item.id); }).forEach(function (item) { command(windows, item); });
    var rows = window.dinahCommands.rows('window');
    if (windows.children.length && rows.length && rows[0] !== '-') command(windows, '-');
    rows.forEach(function (row) { command(windows, row); });
  }
  /* PUDL's open panel presses the rows it drew from, so a redraw while a
     panel is open would leave its rows pressing buttons that are gone. The
     redraw waits until the panel closes; PUDL reads the rows afresh for a
     key, so keys are not held up. */
  var waiting = false;
  function panelOpen() { var panel = document.getElementById('menubar-panel'); return !!panel && panel.matches(':popover-open'); }
  function drawAll(event) {
    // Native mode changes use PUDL's documented refresh of an open menu.
    if (panelOpen() && !event?.detail?.refreshOpenMenus) {
      if (!waiting) {
        waiting = true;
        document.getElementById('menubar-panel').addEventListener('toggle', function closed(event) {
          if (event.newState !== 'closed') return;
          this.removeEventListener('toggle', closed);
          waiting = false;
          drawAll();
        });
      }
      return;
    }
    ['dinah', 'documents', 'view', 'help'].forEach(drawRegistry); drawWindows(); drawThemes();
    // The host registry supplies markup rows rather than PUDL commands.
    // Its explicit open-menu updates still use the documented notification.
    if (event?.detail?.refreshOpenMenus && window.pudlMenubar) window.pudlMenubar.refresh();
  }

  window.dinahOpenWindows = function () {
    var entries = window.pudlWindows.state().open.map(function (key) {
      var win = document.querySelector('[data-win="' + CSS.escape(key) + '"]');
      if (!win) return null;
      var title = win.querySelector('.win-title').textContent.trim(), item = bench.item(key);
      var detail = (window.pudlWindows.detailOf && window.pudlWindows.detailOf(key)) || (item && item.workspace) || '';
      return { key: key, kind: win.dataset.winKind || '', title: title, detail: detail, open: function () { if (document.querySelector('[data-win="' + CSS.escape(key) + '"]')) window.pudlWindows.raise(key); } };
    }).filter(Boolean);
    var counts = new Map(); entries.forEach(function (item) { counts.set(item.title, (counts.get(item.title) || 0) + 1); });
    return entries.map(function (item) {
      if (counts.get(item.title) > 1) item.title += ' (' + (item.detail || item.key) + ')';
      return item;
    });
  };

  /* ---- The Window menu's commands ---------------------------------------- */

  var C = window.dinahCommands;
  function api() { return window.pudlWindows; }
  function front() { var s = api() && api().state(); return s && s.top && !s.min[s.top] ? s.top : null; }
  function closeItem() { var key = front(); return key ? api().menuCommands(key).find(function (item) { return item.id === 'close'; }) : null; }
  function openCount() { return api() ? api().state().open.length : 0; }
  /* The open Editor windows in the order the taskbar shows them, and the
     next one after the front one, either way round. */
  function cycleDocuments(step) {
    var editors = window.dinahOpenWindows().filter(function (w) { return w.kind === 'editor'; });
    if (!editors.length) return;
    var at = editors.findIndex(function (w) { return w.key === front(); });
    var next = at < 0 ? (step > 0 ? 0 : editors.length - 1) : (at + step + editors.length) % editors.length;
    editors[next].open();
  }
  function hasEditors() { return window.dinahOpenWindows && window.dinahOpenWindows().some(function (w) { return w.kind === 'editor'; }); }
  /* The next window of any kind after the front one, in the Window menu's
     order, either way round. */
  function cycleWindows(step) {
    var all = window.dinahOpenWindows ? window.dinahOpenWindows() : [];
    if (all.length < 2) return;
    var at = all.findIndex(function (w) { return w.key === front(); });
    all[at < 0 ? 0 : (at + step + all.length) % all.length].open();
  }

  C.register({ id: 'window-close', glyph: 'close', menu: 'window', mnemonic: 'C', label: 'Close', shortcut: 'Ctrl+F4', visible: function () { return !!closeItem(); }, danger: true, run: function () { var item = closeItem(); if (item) item.run(); } });
  C.register({ id: 'window-next-document', menu: 'window', mnemonic: 'N', label: 'Next Document', shortcut: 'Ctrl+F6', enabled: hasEditors, run: function () { cycleDocuments(1); } });
  C.register({ id: 'window-previous-document', menu: 'window', mnemonic: 'P', label: 'Previous Document', shortcut: 'Ctrl+Shift+F6', enabled: hasEditors, run: function () { cycleDocuments(-1); } });
  C.register({ id: 'window-next', glyph: 'forward', menu: 'window', mnemonic: 'X', label: 'Next Window', shortcut: 'Ctrl+PageDown', enabled: function () { return openCount() > 1; }, run: function () { cycleWindows(1); } });
  C.register({ id: 'window-previous', glyph: 'back', menu: 'window', mnemonic: 'V', label: 'Previous Window', shortcut: 'Ctrl+PageUp', enabled: function () { return openCount() > 1; }, run: function () { cycleWindows(-1); } });
  C.register({ id: 'window-sep-1', menu: 'window', separator: true });
  C.register({ id: 'window-minimize-all', glyph: 'minimize-all', menu: 'window', mnemonic: 'I', label: 'Minimize All', enabled: function () { return !!openCount(); }, run: function () { api().minimizeAll(); } });
  C.register({ id: 'window-restore-all', glyph: 'restore-all', menu: 'window', mnemonic: 'R', label: 'Restore All', enabled: function () { return !!openCount(); }, run: function () { api().restoreAll(); } });
  C.register({ id: 'window-close-all', glyph: 'close-all', menu: 'window', mnemonic: 'A', label: 'Close All Windows', danger: true, enabled: function () { return !!openCount(); }, run: function () { api().closeAll(); } });
  C.register({ id: 'window-sep-2', menu: 'window', separator: true });
  [1, 2, 3, 4, 5, 6, 7, 8, 9].forEach(function (n) {
    function item() { return window.dinahOpenWindows ? window.dinahOpenWindows()[n - 1] : null; }
    C.register({ id: 'window-' + n, menu: 'window', shortcut: 'Alt+' + n, search: false,
      help: n === 1 ? { label: 'Window 1 to 9', key: 'Alt+1 to Alt+9' } : false,
      label: function () { var w = item(); return n + '. ' + (w ? w.title : ''); },
      visible: function () { return !!item(); },
      checked: function () { var w = item(); return !!w && w.key === front(); },
      run: function () { var w = item(); if (w) w.open(); } });
  });
  C.register({ id: 'window-all', glyph: 'window', menu: 'window', mnemonic: 'L', label: 'All Windows…', run: function () { window.dinahCommands.run('all-windows'); } });
  /* The keys PUDL's windows take on a focused title bar and dock tab
     (pudl-windows.js), listed with the Window menu's. */
  C.addHelp('Window', [
    { where: 'Title Bar', label: 'The Window\'s Menu', key: 'Shift+F10 or the Menu key' },
    { where: 'Title Bar', label: 'Maximize or Restore', key: 'Enter' },
    { where: 'Title Bar', label: 'Move the Window', key: 'Arrow keys' },
    { where: 'Title Bar', label: 'Resize a Docked Window', key: 'Shift+Arrow keys' },
    { where: 'Dock Tab', label: 'Move the Tab Along the Dock', key: 'Alt+Shift+Arrow keys' }], 'window-keys');
  /* PUDL's window switcher, which the layer turns on with data-win-switcher
     and names in its own words there. */
  C.addHelp('Window', [
    { label: 'Switch to the Last Window', key: 'Ctrl+Tab' },
    { label: 'Switch Back Through the Windows', key: 'Ctrl+Shift+Tab' },
    { where: 'Switching', label: 'Pick in the List of Windows', key: 'Tab, Shift+Tab or the arrows, with Ctrl held' },
    { where: 'Switching', label: 'Let Go Without Switching', key: 'Esc' }
  ], 'window-switcher');

  document.addEventListener('dinah:launchers-change', function () { drawTools(); if (window.pudlMenubar && !panelOpen()) window.pudlMenubar.refresh(); });
  document.addEventListener('pudl:windows-change', function () { C.changed(); });
  document.addEventListener('dinah:commands-change', drawAll);
  document.addEventListener('dinah:appearance-library', drawAll);
  document.addEventListener('DOMContentLoaded', function () { drawTools(); drawAll(); });
})();
