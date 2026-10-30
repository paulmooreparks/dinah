/* The command registry.
   Each host command is declared once: its menu, label, shortcut in PUDL's
   Mod spelling, when it is enabled and what it runs. host-menus.js draws the
   Documents, View, Window and Help rows from it, so PUDL's menu bar handles
   their keys and shows them; Help › Keyboard Shortcuts and command search in
   Find and Open list the same entries.

   An entry is
     { id, menu, label, shortcut, enabled, visible, checked, danger, items,
       run, help, reason, inline }
   where label, enabled, visible, checked, items and reason may be
   functions read each time the menus are drawn. reason says why a disabled
   entry cannot run now, which command search shows beside it. items makes the entry a flyout of rows
   ({ label, run, disabled, checked } or '-'). An entry with expand instead
   stands for as many rows as its function returns, in that place, such as
   the actions a document's type declares. A separator is
   { id, menu, separator: true }. help, when given, is { label, key } for the
   Keyboard Shortcuts list in place of the entry's own, or false to leave the
   entry out of it. An inline entry has existing data-cmd placements rather
   than generated menu rows. data-command-label makes a placement read its
   label from the declaration; contextual labels may remain in markup.

   PUDL has no hook for a menu that is about to open, and reads a row's
   state when it draws the bar, so whatever changes a command's state calls
   dinahCommands.changed(), and the rows are drawn again. A command does not
   run while a modal dialog is open, which PUDL's dispatcher does not
   check. */
(function () {
  'use strict';
  var entries = [], byId = new Map(), aliases = new Map(), timer = 0;
  // "document" is the Editor's own Document menu, drawn in its window's bar.
  var MENUS = { documents: 'Documents', document: 'Document', view: 'View', window: 'Window', help: 'Help' };

  function value(v) { return typeof v === 'function' ? v() : v; }
  function modalOpen() {
    try { return !!document.querySelector('dialog:modal'); }
    catch (err) { return !!document.querySelector('dialog[open]'); }
  }
  /* A shortcut in one spelling, so that two entries cannot take one key. */
  function spelling(s) {
    var parts = String(s).split('+').map(function (p) { return p.trim(); }).filter(Boolean);
    var key = parts.pop(), mods = {};
    parts.forEach(function (p) { var l = p.toLowerCase(); mods[l === 'ctrl' || l === 'cmd' ? 'mod' : l] = true; });
    return ['mod', 'alt', 'shift'].filter(function (m) { return mods[m]; }).concat(key.length === 1 ? key.toUpperCase() : key).join('+');
  }
  function shown(s) {
    return String(s).split('+').map(function (p) { return p === 'Mod' ? 'Ctrl' : p; }).join('+');
  }

  function register(entry) {
    if (!entry || !entry.id || !entry.menu) throw new Error('A command needs an identity and a menu.');
    if (byId.has(entry.id) || aliases.has(entry.id)) throw new Error('Duplicate command: ' + entry.id);
    if (entry.shortcut) {
      var key = spelling(entry.shortcut);
      entries.forEach(function (other) {
        if (other.shortcut && spelling(other.shortcut) === key) throw new Error(entry.id + ' takes ' + entry.shortcut + ', which ' + other.id + ' has');
      });
    }
    entries.push(entry);
    byId.set(entry.id, entry);
    changed();
    return entry;
  }

  function enabled(entry) { return entry.enabled === undefined || !!value(entry.enabled); }
  function visible(entry) { return entry.visible === undefined || !!value(entry.visible); }

  function resolve(id) { return byId.get(aliases.get(id) || id); }
  function alias(id, target) {
    if (byId.has(id) || aliases.has(id) || !byId.has(target)) throw new Error('Invalid command alias: ' + id);
    aliases.set(id, target);
  }
  function available(entry) { return !!entry && !entry.separator && visible(entry) && enabled(entry); }
  function run(id) {
    const entry = resolve(id);
    if (!available(entry) || modalOpen()) return false;
    Promise.resolve().then(() => {
      if (available(entry) && !modalOpen()) return entry.run();
    }).catch(error => document.dispatchEvent(new CustomEvent('dinah:menu-error', { detail: error })));
    return true;
  }
  // A retained row always invokes the current declaration and availability.
  function describe(id, label) {
    const entry = resolve(id);
    if (!entry) throw new Error('Unknown command: ' + id);
    return { label: label || value(entry.label), disabled: !available(entry),
      checked: value(entry.checked), run: () => run(id) };
  }
  function renderControls() {
    document.querySelectorAll('[data-cmd]').forEach(control => {
      const entry = resolve(control.dataset.cmd);
      if (!entry) return;
      const disabled = !available(entry);
      if ('disabled' in control) control.disabled = disabled;
      if (disabled) control.setAttribute('aria-disabled', 'true'); else control.removeAttribute('aria-disabled');
      const checked = value(entry.checked);
      if (typeof checked === 'boolean' && control.getAttribute('aria-checked') !== String(checked)) control.setAttribute('aria-checked', String(checked));
      if (control.hasAttribute('data-command-label') && control.textContent !== value(entry.label)) control.textContent = value(entry.label);
    });
  }
  document.addEventListener('click', event => {
    const control = event.target.closest && event.target.closest('[data-cmd]');
    if (!control) return;
    event.preventDefault();
    if (control.disabled || control.getAttribute('aria-disabled') === 'true') return;
    run(control.dataset.cmd);
  });

  /* A menu's rows as host-menus.js draws them: hidden entries left out, and
     no separator at either end or beside another. */
  function guarded(fn) {
    return function () {
      if (modalOpen()) return;
      return fn();
    };
  }
  function expanded(entry) {
    return (value(entry.expand) || []).map(function (row) {
      return { label: row.label, disabled: !!row.disabled, run: guarded(row.run) };
    });
  }
  function rows(menu) {
    var out = [];
    entries.forEach(function (entry) {
      if (entry.menu !== menu || entry.inline || !visible(entry)) return;
      if (entry.separator) { if (out.length && out[out.length - 1] !== '-') out.push('-'); return; }
      if (entry.expand) { out = out.concat(expanded(entry)); return; }
      var on = enabled(entry);
      var row = { label: value(entry.label), disabled: !on, danger: !!entry.danger, shortcut: entry.shortcut || '',
        reason: on ? '' : value(entry.reason) || '', mnemonic: entry.mnemonic || '', run: function () { run(entry.id); } };
      var checked = value(entry.checked);
      if (typeof checked === 'boolean') row.checked = checked;
      // The glyph a row shows in a menu and as a tool on the ribbon.
      if (entry.glyph) row.glyph = entry.glyph;
      if (entry.glyphOnly) row.glyphOnly = true;
      if (entry.items) row.items = value(entry.items) || [];
      out.push(row);
    });
    while (out[out.length - 1] === '-') out.pop();
    return out;
  }

  /* Every command a person can run by name, for command search. */
  function searchable() {
    var out = [];
    entries.forEach(function (entry) {
      if (entry.separator || entry.items || !visible(entry) || entry.search === false) return;
      var menu = MENUS[entry.menu] || entry.menu;
      if (entry.expand) {
        expanded(entry).forEach(function (row) { out.push({ label: row.label, menu: menu, shortcut: '', enabled: !row.disabled, run: row.run }); });
        return;
      }
      var on = enabled(entry);
      out.push({ id: entry.id, label: value(entry.label), menu: menu, shortcut: entry.shortcut ? shown(entry.shortcut) : '', enabled: on,
        reason: on ? '' : value(entry.reason) || '', run: function () { run(entry.id); } });
    });
    searchSources.forEach(function (source) { out = out.concat(source() || []); });
    return out;
  }
  /* A window kind whose commands are its own rather than the registry's
     (the Editor's Edit and View rows) gives the front window's commands to
     command search here, as rows in the shape searchable() returns. */
  var searchSources = [];
  function addSearch(source) { searchSources.push(source); }

  function flush(options) {
    clearTimeout(timer); timer = 0;
    renderControls();
    document.dispatchEvent(new CustomEvent('dinah:commands-change',{detail:{refreshOpenMenus:!!options?.refreshOpenMenus}}));
  }
  function changed() {
    renderControls();
    if (timer) return;
    timer = setTimeout(flush, 30);
  }
  /* A key can follow a change within the redraw's delay, as Ctrl+S right
     after typing does, so a waiting redraw is made at once, before the menu
     bar reads the rows for the key. */
  document.addEventListener('keydown', function (e) {
    if (timer && (e.ctrlKey || e.metaKey || e.altKey || /^F\d+$/.test(e.key))) flush();
  }, true);

  /* An applet declares its own keys for Help › Keyboard Shortcuts here: a
     group named for the applet, such as Editor or Terminal, each row with
     its label and key and, where the key works in one part of the applet
     only, that part (where). A group named for a menu, such as Window, adds
     to the keys that work everywhere. An applet declares its keys once for
     all its windows, so declaring them again replaces them; where two
     scripts declare keys for one group, each gives its own id. */
  var helpGroups = [];
  function addHelp(group, rows, id) {
    id = id || group;
    helpGroups = helpGroups.filter(function (g) { return g.id !== id; });
    helpGroups.push({ id: id, group: group, rows: rows });
  }

  /* How the menu bar is reached from the keyboard, which PUDL's desktop host
     gives. The letters are not listed, since
     the underline in the menus is their listing. A Mac keeps F10 for itself,
     and Option with a letter types a character there. */
  var MENU_KEYS = /Mac|iPhone|iPad|iPod/.test(navigator.platform || '')
    ? 'Option, pressed and released alone, moves to the menu bar; Ctrl+Option with a menu\'s underlined letter opens that menu from anywhere, and a letter in an open menu runs its command.'
    : 'Alt, pressed and released alone, or F10 moves to the menu bar; Alt with a menu\'s underlined letter opens that menu from anywhere, and a letter in an open menu runs its command.';

  /* Help › Keyboard Shortcuts: the keys that work everywhere, which are the
     menus' commands with the keys declared for a menu's group, such as
     Window's Ctrl+Tab; and, chosen under Keys For, one applet's own. */
  function keyboardHelp() {
    var dialog = document.createElement('dialog');
    dialog.className = 'dialog keyboard-help';
    dialog.setAttribute('aria-labelledby', 'keyboard-help-title');
    dialog.innerHTML = '<h2 class="dialog-title" id="keyboard-help-title">Keyboard Shortcuts</h2>' +
      '<p class="dialog-body">' + MENU_KEYS + '</p>' +
      '<p class="dialog-body">The keys below work everywhere. Choose a part of the app under Keys For to see the keys that work in it, which work while its window is in front. A key does nothing while a dialog is open.</p>' +
      '<div class="form-group keyboard-help-for"><label class="form-label" for="keyboard-help-for">Keys For</label><span class="form-select-wrap"><select class="form-select" id="keyboard-help-for"></select></span></div>' +
      '<div class="data-table-wrap keyboard-help-table"><table class="data-table" aria-labelledby="keyboard-help-title"><thead><tr><th scope="col">Command</th><th scope="col">Where</th><th scope="col">Key</th></tr></thead><tbody></tbody></table></div>' +
      '<div class="dialog-actions"><button type="button" class="btn btn-primary" data-close>Close</button></div>';
    var body = dialog.querySelector('tbody'), choice = dialog.querySelector('select');
    function row(label, where, key) {
      var tr = document.createElement('tr');
      [label, where, key].forEach(function (text, i) {
        var td = document.createElement('td');
        if (i === 2) { var kbd = document.createElement('kbd'); kbd.textContent = text; td.appendChild(kbd); } else td.textContent = text;
        tr.appendChild(td);
      });
      body.appendChild(tr);
    }
    var menuNames = Object.keys(MENUS).map(function (m) { return MENUS[m]; });
    var applets = [];
    helpGroups.forEach(function (g) {
      if (menuNames.indexOf(g.group) >= 0) return;
      var a = applets.find(function (x) { return x.group === g.group; });
      if (a) a.rows = a.rows.concat(g.rows); else applets.push({ group: g.group, rows: g.rows });
    });
    applets.sort(function (a, b) { return a.group.localeCompare(b.group); });
    choice.appendChild(new Option('Everywhere', ''));
    applets.forEach(function (g) { choice.appendChild(new Option(g.group, g.group)); });
    function draw() {
      body.replaceChildren();
      var applet = applets.find(function (g) { return g.group === choice.value; });
      if (applet) {
        applet.rows.forEach(function (r) { row(r.label, r.where || applet.group, r.key); });
        return;
      }
      Object.keys(MENUS).forEach(function (menu) {
        entries.forEach(function (entry) {
          if (entry.menu !== menu || entry.separator || entry.help === false || !(entry.shortcut || entry.help)) return;
          var help = entry.help || {};
          row(help.label || value(entry.label), MENUS[menu], help.key || shown(entry.shortcut));
        });
        helpGroups.forEach(function (g) {
          if (g.group === MENUS[menu]) g.rows.forEach(function (r) { row(r.label, r.where || g.group, r.key); });
        });
      });
    }
    choice.addEventListener('change', draw);
    draw();
    document.body.appendChild(dialog);
    function close() { dialog.close(); }
    dialog.addEventListener('close', function () { dialog.remove(); });
    dialog.querySelector('[data-close]').addEventListener('click', close);
    dialog.showModal();
    dialog.querySelector('[data-close]').focus();
  }

  function installHost(adapter) {
    const { app, chats, chatBrowser, notices, productName, toast, fail,
      showChat, showNotifications, openGoTo, openWorkspaceRename, removeWorkspace,
      exportWorkspace, importWorkspace, installPackages, startNewChat, createWorkspace,
      clearNotifications, drawWorkspaces, refreshSessions, openSettings, startRename,
      openRemove, signIn, frontProvider, restartAgent, saveThinking, flushNotifications,
      workspaceCommands, chatCommands, thinking } = adapter;
    register({ id: 'show-chat', menu: 'view', inline: true, label: 'Show Chat', run: () => { showChat(); } });
    register({ id: 'find-tool', menu: 'view', inline: true, label: 'Find a Tool…', run: () => { openGoTo('tools'); } });
    register({ id: 'show-workspace', menu: 'view', inline: true, label: 'Workspace', run: () => { window.dinahWorkbench.open('workspace').catch(fail); } });
    register({ id: 'rename-workspace', menu: 'dinah', inline: true, label: 'Rename Workspace…', enabled: () => workspaceCommands('rename-workspace'), run: () => { openWorkspaceRename(); } });
    register({ id: 'remove-workspace', menu: 'view', inline: true, label: 'Remove Workspace…', enabled: () => workspaceCommands('remove-workspace'), danger: true, run: () => { removeWorkspace(); } });
    register({ id: 'export-workspace', menu: 'dinah', inline: true, label: 'Export Workspace…', enabled: () => workspaceCommands('export-workspace'), run: () => { exportWorkspace(); } });
    register({ id: 'import-workspace', menu: 'dinah', inline: true, label: 'Import Workspace…', enabled: () => workspaceCommands('import-workspace'), run: () => { importWorkspace(); } });
    register({ id: 'install-packages-file', menu: 'dinah', inline: true, label: 'Install Packages…', run: () => { installPackages(); } });
    register({ id: 'new-chat', menu: 'dinah', inline: true, label: 'New Chat', run: () => { startNewChat().catch(fail); } });
    register({ id: 'new-workspace', menu: 'dinah', inline: true, label: 'New Workspace…', enabled: () => workspaceCommands('new-workspace'), run: () => { createWorkspace(false); } });
    register({ id: 'new-empty-workspace', menu: 'dinah', inline: true, label: 'New Empty Workspace', enabled: () => workspaceCommands('new-empty-workspace'), run: () => { createWorkspace(true); } });
    register({ id: 'chat-browser', menu: 'view', inline: true, label: 'Chats', run: () => { window.dinahWorkbench.open('chat-browser').catch(fail); } });
    register({ id: 'skills-browser', menu: 'view', inline: true, label: 'Skills', run: () => { window.dinahWorkbench.open('skills-browser').catch(fail); } });
    register({ id: 'files', menu: 'view', inline: true, label: 'Files', run: () => { dinahResources.currentPath().then(dinahResources.openFiles).catch(fail); } });
    register({ id: 'terminal', menu: 'view', inline: true, label: 'Terminal', run: () => { dinahResources.currentPath().then(dinahResources.openTerminal).catch(fail); } });
    register({ id: 'clear-notifications', menu: 'view', inline: true, label: 'Clear All…', enabled: () => notices.list(entry => entry.clearable).length > 0, danger: true, run: () => { clearNotifications(); } });
    register({ id: 'refresh-workspace', menu: 'view', inline: true, label: 'Refresh Workspace', enabled: () => workspaceCommands('refresh-workspace'), run: () => { app().State().then(function (st) { drawWorkspaces(st); refreshSessions(); }).catch(fail); } });
    register({ id: 'settings', menu: 'dinah', inline: true, label: 'Settings…', run: () => { openSettings(); } });
    register({ id: 'rename-session', menu: 'view', inline: true, label: 'Rename Chat…', enabled: () => chatCommands('rename-session'), run: () => { startRename(chats.active(), 'head'); } });
    register({ id: 'remove-session', menu: 'view', inline: true, label: 'Remove Chat…', enabled: () => chatCommands('remove-session'), danger: true, run: () => { openRemove(chats.active()); } });
    register({ id: 'unhide-sessions', menu: 'view', inline: true, label: 'Show Hidden Chats Again', enabled: () => chatCommands('unhide-sessions'), run: () => { app().UnhideSessions().then(function () { chatBrowser.setHiddenCount(0); chats.current()?.menuStates(); refreshSessions(); toast('Hidden sessions are back in the list.'); }).catch(fail); } });
    register({ id: 'refresh-sessions', menu: 'view', inline: true, label: 'Refresh Chats', run: () => { refreshSessions(); } });
    register({ id: 'all-models', menu: 'view', inline: true, label: 'All Models…', enabled: () => chatCommands('all-models'), run: () => { chats.current()?.openModels(); } });
    register({ id: 'history', menu: 'view', inline: true, label: 'Prompt History…', run: () => { chats.current()?.openHistory(); } });
    register({ id: 'templates', menu: 'view', inline: true, label: 'My Templates…', run: () => { chats.current()?.openTemplates(null); } });
    register({ id: 'sign-in', menu: 'view', inline: true, label: 'Sign In…', run: () => { signIn(); } });
    register({ id: 'usage-page', menu: 'view', inline: true, label: 'Usage…', run: () => { var usageItem = window.dinahWorkbench.item(window.pudlWindows.state().top);
        var usageProvider = usageItem && (usageItem.kind === 'chat' || usageItem.kind === 'chat-session') ? frontProvider() : chats.defaultProvider();
        Promise.resolve().then(function () { return app().OpenProviderUsage ? app().OpenProviderUsage(usageProvider) : Promise.reject('No usage page is available for this provider.'); }).catch(fail); } });
    register({ id: 'save-run', menu: 'view', inline: true, label: 'Export Chat…', enabled: () => chatCommands('save-run'), run: () => { chats.current()?.saveRun(); } });
    register({ id: 'cancel', menu: 'view', inline: true, label: 'Stop', enabled: () => chatCommands('cancel'), run: () => { chats.current()?.stopTurn(); } });
    register({ id: 'restart', menu: 'view', inline: true, label: 'Restart Agent', run: () => { restartAgent(); } });
    register({ id: 'thoughts', menu: 'view', inline: true, label: "Show the Agent's Thinking", enabled: () => !thinking().busy, checked: () => thinking().checked, run: () => { saveThinking(!thinking().checked); } });
    register({ id: 'about', menu: 'help', inline: true, label: () => 'About ' + productName(), run: () => { document.getElementById('about').showModal(); } });
    register({ id: 'quit', menu: 'dinah', inline: true, label: () => 'Quit ' + productName(), run: () => { Promise.all([flushNotifications(), window.dinahUIState ? window.dinahUIState.flush() : null]).then(function () { if (window.runtime) window.runtime.Quit(); }).catch(fail); } });
    alias('go-to', 'find-and-open');
    register({ id: 'all-windows', menu: 'window', inline: true, search: false, label: 'All Windows…', run: () => openGoTo('windows') });
    register({ id: 'needs-you', menu: 'view', inline: true, search: false, label: 'Show All', run: showNotifications });
    alias('next-notification', 'go-to-question');
    alias('workspace', 'new-workspace');
    alias('new-session', 'new-chat');
    changed();
  }

  window.dinahCommands = { installHost, describe, alias, register: register, run: run, rows: rows, searchable: searchable, changed: changed, flush: flush, shown: shown, menus: MENUS, addHelp: addHelp, addSearch: addSearch };

  register({ id: 'keyboard-shortcuts', menu: 'help', mnemonic: 'K', label: 'Keyboard Shortcuts', shortcut: 'F1', run: keyboardHelp });
})();
