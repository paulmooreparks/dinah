/* Host menus use PUDL's source lists and public window commands. The
   Chat, Documents, View, Window and Help rows come from the command
   registry (commands.js). Launcher destinations are drawn by
   title-launcher.js. */
(function () {
  'use strict';
  var bench = window.dinahWorkbench;
  var windows = document.getElementById('host-windows');
  function command(list, definition) {
    var li = document.createElement('li');
    li.dataset.generated = '';
    if (definition === '-') li.textContent = '-';
    else if (definition.items) {
      li.appendChild(document.createTextNode(definition.label));
      if (definition.mnemonic) li.dataset.mnemonic = definition.mnemonic;
      if (definition.glyph) li.dataset.menuGlyph = definition.glyph;
      var nested = document.createElement('ul'); li.appendChild(nested);
      // A flyout of which one row is on has radio marks (docs/design/menus.md).
      if (definition.radio) nested.setAttribute('data-menu-radio', '');
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
  function drawThemes() {
    var list = document.getElementById('host-themes'), appearance = window.dinahAppearance;
    if (!list || !appearance) return;
    list.replaceChildren();
    // One theme is on, so the themes are a set with one mark, not a row of
    // switches (docs/design/menus.md). The person's own themes stand under
    // a heading of their own, which says what "(built in)" after every
    // name used to.
    list.setAttribute('data-menu-radio', '');
    var state = appearance.snapshot().state;
    command(list, { heading: 'Built In' });
    Object.entries(appearance.builtins).forEach(function (entry) { command(list, { label: entry[1].name, checked: state.active === entry[0], run: function () { return appearance.select(entry[0]); } }); });
    if (state.themes.length) { command(list, '-'); command(list, { heading: 'Yours' }); }
    state.themes.forEach(function (theme) { command(list, { label: theme.document.name, checked: state.active === theme.id, run: function () { return appearance.select(theme.id); } }); });
    command(list, '-');
    // It opens the Theme Editor and asks for nothing more, so it has no ellipsis.
    command(list, { label: 'Edit Current Theme', run: function () { document.getElementById('appearance-edit').click(); } });
    command(list, { label: 'Appearance Settings…', run: function () { return bench.open('settings').then(function () { document.getElementById('settings-tab-appearance').click(); }); } });
  }
  function drawSkins() {
    var list = document.getElementById('host-skins'), appearance = window.dinahAppearance;
    if (!list || !appearance) return;
    var snapshot = appearance.snapshot(), desired = snapshot.state.skin || 'standard';
    list.replaceChildren();
    snapshot.skins.forEach(function (skin) {
      command(list, { label: skin.label, checked: desired === skin.id, run: function () { return appearance.selectSkin(skin.id); } });
    });
    if (!snapshot.skins.some(function (skin) { return skin.id === desired; })) {
      command(list, { label: desired + ' (unavailable)', checked: true, disabled: true, reason: snapshot.skinNotice });
    }
  }
  /* A registry menu's rows go into its host list: all of it, or after the
     list's placeholder (li[data-commands]) where some rows are written in
     the page. A list may have a placeholder for each group of rows
     (data-commands="panes"), and the one with no name takes the rows that
     have no group. */
  function drawRegistry(menu) {
    var list = document.querySelector('[data-command-menu="' + CSS.escape(String(menu)) + '"]');
    if (!list) return;
    list.querySelectorAll(':scope > li[data-generated]').forEach(function (li) { li.remove(); });
    var markers = Array.from(list.querySelectorAll(':scope > li[data-commands]'));
    if (!markers.length) { window.dinahCommands.rows(menu).forEach(function (row) { command(list, row); }); return; }
    markers.forEach(function (marker) {
      var at = marker.nextSibling;
      window.dinahCommands.rows(menu, marker.getAttribute('data-commands') || '').forEach(function (row) { list.insertBefore(command(list, row), at); });
    });
  }
  function drawWindows() {
    var api = window.pudlWindows; if (!api) return;
    var state = api.state(); windows.replaceChildren();
    /* The commands for the window in front are one block, and this menu is
       the one place in the bar that has them (docs/design/menus.md). They
       are PUDL's, except Close, which is the registry's and has the key. */
    if (state.top && !state.min[state.top]) api.menuCommands(state.top).filter(function (item) { return !['page', 'copy-link', 'close'].includes(item.id); }).forEach(function (item) { command(windows, item); });
    window.dinahCommands.rows('window', 'front').forEach(function (row) { command(windows, row); });
    var rows = window.dinahCommands.rows('window', '');
    if (windows.children.length && rows.length && rows[0] !== '-') command(windows, '-');
    rows.forEach(function (row) { command(windows, row); });
  }
  /* PUDL's open panel presses the rows it drew from, so a redraw while a
     panel is open would leave its rows pressing buttons that are gone. The
     redraw waits until the panel closes; PUDL reads the rows afresh for a
     key, so keys are not held up. */
  var waiting = false;
  // Whether a menu of the bar is open, and its closing, are PUDL's to say
  // (pudlMenubar.isOpen and pudl:menubar-close, 0.96.9).
  function panelOpen() { return !!(window.pudlMenubar && window.pudlMenubar.isOpen && window.pudlMenubar.isOpen()); }
  function drawAll(event) {
    // Native mode changes use PUDL's documented refresh of an open menu.
    if (panelOpen() && !event?.detail?.refreshOpenMenus) {
      if (!waiting) {
        waiting = true;
        document.addEventListener('pudl:menubar-close', function closed() {
          document.removeEventListener('pudl:menubar-close', closed);
          waiting = false;
          drawAll();
        });
      }
      return;
    }
    ['dinah', 'chat', 'workspace', 'documents', 'view', 'help'].forEach(drawRegistry); drawWindows(); drawThemes(); drawSkins();
    // The host registry supplies markup rows rather than PUDL commands.
    // Its explicit open-menu updates still use the documented notification.
    if (event?.detail?.refreshOpenMenus && window.pudlMenubar) window.pudlMenubar.refresh();
  }

  /* The list is asked for many times in one drawing of the menus (each
     Window row's label, visibility and check), and each asks the page for
     every window, so it is built once per task and dropped when the task
     ends. */
  var openWindowsMemo = null;
  window.dinahOpenWindows = function () {
    if (openWindowsMemo) return openWindowsMemo;
    Promise.resolve().then(function () { openWindowsMemo = null; });
    return (openWindowsMemo = listOpenWindows());
  };
  function listOpenWindows() {
    var entries = window.dinahHelpers.windowState().open.map(function (key) {
      var win = window.dinahHelpers.windowOf(key);
      if (!win) return null;
      var title = win.querySelector('.win-title').textContent.trim(), item = bench.item(key);
      var detail = (window.pudlWindows.detailOf && window.pudlWindows.detailOf(key)) || (item && item.workspace) || '';
      return { key: key, kind: win.dataset.winKind || '', title: title, detail: detail, open: function () { if (window.dinahHelpers.windowOf(key)) window.pudlWindows.raise(key); } };
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
  /* The open document windows (the Editor's, Docs', Diagrams' and
     Slides') in the order the taskbar shows them, and the next one after
     the front one, either way round. */
  function documentWindow(w) { return w.kind === 'editor' || /^content-/.test(w.kind || ''); }
  function cycleDocuments(step) {
    var editors = window.dinahOpenWindows().filter(documentWindow);
    if (!editors.length) return;
    var at = editors.findIndex(function (w) { return w.key === front(); });
    var next = at < 0 ? (step > 0 ? 0 : editors.length - 1) : (at + step + editors.length) % editors.length;
    editors[next].open();
  }
  function hasEditors() { return window.dinahOpenWindows && window.dinahOpenWindows().some(documentWindow); }
  /* The next window of any kind after the front one, in the Window menu's
     order, either way round. */
  function cycleWindows(step) {
    var all = window.dinahOpenWindows ? window.dinahOpenWindows() : [];
    if (all.length < 2) return;
    var at = all.findIndex(function (w) { return w.key === front(); });
    all[at < 0 ? 0 : (at + step + all.length) % all.length].open();
  }

  // Close is the last of the rows for the window in front (drawWindows).
  C.register({ id: 'window-close', glyph: 'close', menu: 'window', group: 'front', mnemonic: 'C', label: 'Close', shortcut: 'Ctrl+F4', visible: function () { return !!closeItem(); }, danger: true, run: function () { var item = closeItem(); if (item) item.run(); } });
  C.register({ id: 'window-next', glyph: 'forward', menu: 'window', mnemonic: 'X', label: 'Next Window', shortcut: 'Ctrl+PageDown', enabled: function () { return openCount() > 1; }, reason: 'No other window is open.', run: function () { cycleWindows(1); } });
  C.register({ id: 'window-previous', glyph: 'back', menu: 'window', mnemonic: 'V', label: 'Previous Window', shortcut: 'Ctrl+PageUp', enabled: function () { return openCount() > 1; }, reason: 'No other window is open.', run: function () { cycleWindows(-1); } });
  C.register({ id: 'window-next-document', menu: 'window', mnemonic: 'N', label: 'Next Document', shortcut: 'Ctrl+F6', enabled: hasEditors, reason: 'No document is open.', run: function () { cycleDocuments(1); } });
  C.register({ id: 'window-previous-document', menu: 'window', mnemonic: 'U', label: 'Previous Document', shortcut: 'Ctrl+Shift+F6', enabled: hasEditors, reason: 'No document is open.', run: function () { cycleDocuments(-1); } });
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
  /* View, Refresh reads again what the window in front lists: the chats,
     the workspace, the skills, a folder. A window says how with its reload
     (workbench.js, reloadOf), and the row is greyed where it has none. */
  function reloadFront() { var key = front(); return key && window.dinahWorkbench.reloadOf ? window.dinahWorkbench.reloadOf(key) : null; }
  C.register({ id: 'refresh', glyph: 'reload', menu: 'view', mnemonic: 'E', label: 'Refresh', shortcut: 'F5', enabled: function () { return !!reloadFront(); },
    reason: 'Nothing in front is a list that can be read again.', run: function () { var reload = reloadFront(); if (reload) return reload(); } });
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

  document.addEventListener('dinah:launchers-change', function () { if (window.pudlMenubar && !panelOpen()) window.pudlMenubar.refresh(); });
  document.addEventListener('pudl:windows-change', function () { C.changed(); });
  document.addEventListener('dinah:commands-change', drawAll);
  // A theme or skin saved while a menu or the ribbon shows its list ticks the
  // new choice in that open list at once, through PUDL's documented refresh,
  // so a list opened before the save ended never keeps the old tick.
  document.addEventListener('dinah:appearance-library', function () { drawAll({ detail: { refreshOpenMenus: true } }); });
  document.addEventListener('DOMContentLoaded', function () { drawAll(); });
})();

