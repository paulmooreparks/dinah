/* The command registry.
   Each host command is declared once: its menu, label, shortcut in PUDL's
   Mod spelling, when it is enabled and what it runs. host-menus.js draws the
   Documents, View, Window and Help rows from it, so PUDL's menu bar handles
   their keys and shows them; Help › Keyboard Shortcuts and command search in
   Find and Open list the same entries.

   An entry is
     { id, menu, label, menuLabel, shortcut, enabled, visible, checked,
       danger, items, radio, run, help, reason, inline, group, after }
   where label, enabled, visible, checked, items and reason may be
   functions read each time the menus are drawn. menuLabel is the entry's
   words in its own menu, where the menu's title already says what it is
   about: Chat has "Rename…", which command search lists as "Rename Chat…".
   reason says why a disabled
   entry cannot run now, which the menu and command search show beside it. items makes the entry a flyout of rows
   ({ label, run, disabled, checked } or '-'), and radio says that one of
   them is on. A flyout that cannot be used is drawn as a plain greyed row
   with its reason. An entry with expand instead
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
  // The commands of the chat in front, by ID, kept for one drawing of the
  // menus (installHost): undefined when not read yet, null with no chat.
  var frontChatCommands;
  // "document" is the Editor's own Document menu, drawn in its window's bar.
  var MENUS = { workspace: 'Workspace', tools: 'Tools', documents: 'Documents', document: 'Document', view: 'View', window: 'Window', help: 'Help' };
  // "chat" is the bar's first menu (docs/design/menus.md). It is kept apart
  // from MENUS because Keyboard Shortcuts takes a group with a menu's name
  // for the bar's own keys, and the keys that work only inside a chat's
  // window are a group called Chat.
  var FILED = { chat: 'Chat' };

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
    // A shortcut may be a function, for a key a command holds only at times
    // (Documents > Open..., which gives Ctrl+O to a content window's own Open).
    if (typeof entry.shortcut === 'string' && entry.shortcut) {
      var key = spelling(entry.shortcut);
      entries.forEach(function (other) {
        if (typeof other.shortcut === 'string' && other.shortcut && spelling(other.shortcut) === key) throw new Error(entry.id + ' takes ' + entry.shortcut + ', which ' + other.id + ' has');
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
  /* The controls that run a command (data-cmd) are found once and the list
     is kept until one is added to the page or taken from it. Looking for
     them read every element of every open window, and the look was made
     several times for each change of a chat's state; with 32 chats open that
     was a few milliseconds each time (17 November 2026,
     docs/design/scaling.md). What was added or removed since the last look
     is read from the page's own record of its changes, at once when the
     controls are about to be drawn, so a control drawn in the same task is
     never missed. */
  let controls = null;
  function holdsControl(node) { return node.nodeType === 1 && (node.hasAttribute('data-cmd') || !!node.querySelector('[data-cmd]')); }
  function controlsChanged(records) {
    if (!controls) return;
    for (const record of records) {
      if (record.type === 'attributes' || Array.prototype.some.call(record.addedNodes, holdsControl) || Array.prototype.some.call(record.removedNodes, holdsControl)) { controls = null; return; }
    }
  }
  const controlChanges = new MutationObserver(controlsChanged);
  controlChanges.observe(document.documentElement, { subtree: true, childList: true, attributes: true, attributeFilter: ['data-cmd'] });
  function commandControls() {
    controlsChanged(controlChanges.takeRecords());
    if (!controls) controls = Array.from(document.querySelectorAll('[data-cmd]'));
    return controls;
  }
  // The controls that run one command, for a caller that words them itself.
  function controlsFor(id) { return commandControls().filter(control => control.dataset.cmd === id); }
  function renderControls() {
    commandControls().forEach(control => {
      const entry = resolve(control.dataset.cmd);
      if (!entry) return;
      // Each is written only when it changes: a write of the same value still makes the browser restyle around it.
      const disabled = !available(entry);
      if ('disabled' in control && control.disabled !== disabled) control.disabled = disabled;
      if (!disabled) { if (control.hasAttribute('aria-disabled')) control.removeAttribute('aria-disabled'); }
      else if (control.getAttribute('aria-disabled') !== 'true') control.setAttribute('aria-disabled', 'true');
      const checked = value(entry.checked);
      if (typeof checked === 'boolean' && control.getAttribute('aria-checked') !== String(checked)) control.setAttribute('aria-checked', String(checked));
      if (control.hasAttribute('data-command-label') && control.textContent !== value(entry.label)) control.textContent = value(entry.label);
      // A greyed row of a menu says why, where its command gives a reason.
      if (entry.reason !== undefined) {
        const reason = disabled ? value(entry.reason) || '' : '';
        if ((control.getAttribute('data-menu-reason') || '') !== reason) { if (reason) control.setAttribute('data-menu-reason', reason); else control.removeAttribute('data-menu-reason'); }
      }
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
  /* A menu's entries in the order they are drawn: the order they were
     registered in, except that an entry with after stands straight after
     the entry of that ID, so a command registered in another file can take
     its place in a group. */
  function ordered(menu) {
    var own = entries.filter(function (entry) { return entry.menu === menu; });
    var late = own.filter(function (entry) { return entry.after; });
    var out = own.filter(function (entry) { return !entry.after; });
    late.forEach(function (entry) {
      var at = -1;
      out.forEach(function (other, i) { if (other.id === entry.after || other.after === entry.after) at = i; });
      if (at < 0) out.push(entry); else out.splice(at + 1, 0, entry);
    });
    return out;
  }
  /* With group given, only the entries of that group ('' for those with
     none), for a list that has a place for each group (host-menus.js). */
  function rows(menu, group) {
    var out = [];
    ordered(menu).forEach(function (entry) {
      if (entry.inline || !visible(entry)) return;
      if (group !== undefined && (entry.group || '') !== group) return;
      if (entry.separator) { if (out.length && out[out.length - 1] !== '-') out.push('-'); return; }
      if (entry.expand) { out = out.concat(expanded(entry)); return; }
      var on = enabled(entry);
      var row = { label: value(entry.menuLabel) || value(entry.label), disabled: !on, danger: !!entry.danger, shortcut: value(entry.shortcut) || '',
        reason: on ? '' : value(entry.reason) || '', mnemonic: entry.mnemonic || '', run: function () { run(entry.id); } };
      var checked = value(entry.checked);
      if (typeof checked === 'boolean') row.checked = checked;
      // The glyph a row shows in a menu and as a tool on the ribbon.
      if (entry.glyph) row.glyph = entry.glyph;
      if (entry.glyphOnly) row.glyphOnly = true;
      // The markup a menu is drawn from cannot grey a flyout, so one that
      // cannot be used is a plain greyed row, which shows its reason.
      if (entry.items && on) { row.items = value(entry.items) || []; if (entry.radio) row.radio = true; }
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
      var menu = MENUS[entry.menu] || FILED[entry.menu] || entry.menu;
      if (entry.expand) {
        expanded(entry).forEach(function (row) { out.push({ label: row.label, menu: menu, shortcut: '', enabled: !row.disabled, run: row.run }); });
        return;
      }
      var on = enabled(entry);
      var shortcut = value(entry.shortcut);
      out.push({ id: entry.id, label: value(entry.label), menu: menu, shortcut: shortcut ? shown(shortcut) : '', enabled: on,
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
    frontChatCommands = undefined;
    renderControls();
    document.dispatchEvent(new CustomEvent('dinah:commands-change',{detail:{refreshOpenMenus:!!options?.refreshOpenMenus}}));
  }
  function changed() {
    frontChatCommands = undefined;
    renderControls();
    if (timer) return;
    timer = setTimeout(flush, 30);
  }
  /* A key can follow a change within the redraw's delay, as Ctrl+S right
     after typing does, so a waiting redraw is made at once, before the menu
     bar reads the rows for the key. */
  // Labels written from the product's name (Exit, About) are drawn again
  // when the page learns the name (product.js).
  document.addEventListener('dinah:product', function () { changed(); });
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
      Object.keys(FILED).concat(Object.keys(MENUS)).forEach(function (menu) {
        entries.forEach(function (entry) {
          if (entry.menu !== menu || entry.separator || entry.help === false || !(entry.shortcut || entry.help)) return;
          var help = entry.help || {};
          row(help.label || value(entry.label), MENUS[menu] || FILED[menu], help.key || shown(value(entry.shortcut) || ''));
        });
        if (!MENUS[menu]) return;
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

    /* The Chat menu (docs/design/menus.md): first what is done with chats as
       a whole, then the commands of the chat in front, which are that
       chat's own (chat-applet.js, menuCommands) and are found here by their
       IDs. A row that needs a chat is greyed with the reason while none is
       in front, and so is a row the chat in front does not offer now. */
    const NO_CHAT = 'No chat is in front.', NOT_STARTED = 'The chat starts with its first message.';
    function ofChat(id) {
      if (frontChatCommands === undefined) {
        const chat = chats.front();
        frontChatCommands = chat ? new Map(chat.menuCommands().map(command => [command.id, command])) : null;
        Promise.resolve().then(() => { frontChatCommands = undefined; });
      }
      return frontChatCommands ? frontChatCommands.get(id) || null : null;
    }
    function usable(command) { return !!command && !command.hidden && !command.disabled; }
    // The rows of a chat's flyout. A row may be the ID of another of the
    // chat's commands, as the last rows of Subagents are.
    function rowsOf(command) {
      const items = usable(command) ? (typeof command.items === 'function' ? command.items() : command.items) || [] : [];
      return items.map(item => typeof item === 'string' && item !== '-' ? ofChat(item) : item)
        .filter(item => item && !item.hidden)
        .map(item => item === '-' || item.heading ? item : { label: item.label, detail: item.detail, disabled: !!item.disabled, reason: item.reason || '', checked: item.checked, danger: !!item.danger, run: item.run });
    }
    // own.from is the ID of the chat's command, and own.without says why
    // the chat in front does not offer it now.
    function chatRow(id, own) {
      const command = () => ofChat(own.from || id);
      register(Object.assign({ id, menu: 'chat',
        enabled: () => own.items ? own.items().length > 0 : usable(command()),
        reason: () => { const c = command(); return frontChatCommands === null ? NO_CHAT : c && !c.hidden && c.disabled && c.reason || own.without || ''; },
        run: () => { const c = command(); if (usable(c) && c.run) return c.run(); } }, own));
    }
    function chatList(id, own) {
      chatRow(id, Object.assign({ radio: true, items: () => rowsOf(ofChat(own.from || id)) }, own));
    }
    // A chat tells PUDL when its commands change, for the controls in its
    // own bar, and the Chat menu's rows are drawn again with them. An open
    // menu is brought up to date in place, through PUDL's refresh
    // (host-menus.js), so Stop is there to press as soon as a turn starts.
    document.addEventListener('pudl:commands-change', () => {
      if (!chats.front()) return;
      if (window.pudlMenubar && window.pudlMenubar.isOpen && window.pudlMenubar.isOpen()) flush({ refreshOpenMenus: true });
      else changed();
    });
    const TEAM = ['open-team', 'open-own-team', 'change-team-lead', 'leave-team'];
    register({ id: 'new-chat', menu: 'chat', mnemonic: 'N', glyph: 'comments', label: 'New Chat', shortcut: 'Mod+N', run: () => { startNewChat().catch(fail); } });
    register({ id: 'chat-sep-new', menu: 'chat', separator: true });
    chatRow('rename-session', { from: 'rename-chat', mnemonic: 'R', label: 'Rename Chat…', menuLabel: 'Rename…', shortcut: 'F2', without: NOT_STARTED });
    chatRow('continue-workspace', { mnemonic: 'C', label: 'Continue in Another Workspace…' });
    chatRow('pin-chat', { mnemonic: 'K', label: () => { const c = ofChat('pin-chat'); return c ? c.label : 'Pin to Workspace'; } });
    register({ id: 'chat-sep-agent', menu: 'chat', separator: true });
    chatList('chat-provider', { from: 'provider', mnemonic: 'I', label: 'Provider' });
    chatList('chat-mode', { from: 'mode', mnemonic: 'M', label: 'Mode', without: 'This agent offers no modes.' });
    chatList('chat-model', { from: 'model', mnemonic: 'O', label: 'Model', without: 'This agent offers no choice of model.' });
    chatList('chat-effort', { from: 'effort', mnemonic: 'F', label: 'Effort', without: 'This agent offers no levels of effort.' });
    register({ id: 'chat-sep-groups', menu: 'chat', separator: true });
    chatRow('start-group', { mnemonic: 'S', label: 'Start a Group Chat…' });
    chatList('add-to-group', { radio: false, mnemonic: 'A', label: 'Add to a Group Chat' });
    chatRow('chat-team', { mnemonic: 'T', label: 'Team', without: 'This chat has no team commands.',
      items: () => TEAM.map(ofChat).filter(c => c && !c.hidden).map(c => ({ label: c.label, disabled: !!c.disabled, reason: c.reason || '', run: c.run })) });
    register({ id: 'chat-sep-writing', menu: 'chat', separator: true });
    register({ id: 'history', menu: 'chat', mnemonic: 'H', label: 'Prompt History…', run: () => { chats.current()?.openHistory(); } });
    register({ id: 'templates', menu: 'chat', mnemonic: 'P', label: 'My Templates…', run: () => { chats.current()?.openTemplates(null); } });
    chatRow('save-run', { from: 'export-chat', mnemonic: 'E', label: 'Export Chat…', menuLabel: 'Export…', without: NOT_STARTED });
    chatRow('show-chat-log', { mnemonic: 'L', label: 'Show the Chat Log', without: NOT_STARTED });
    chatRow('failure-summaries', { mnemonic: 'W', label: 'Show Failed Command Summaries', without: NOT_STARTED,
      checked: () => { const c = ofChat('failure-summaries'); return !!(c && c.checked); } });
    chatList('chat-subagents', { from: 'subagents', radio: false, mnemonic: 'U', label: 'Subagents', without: 'This chat has started no subagents.' });
    register({ id: 'chat-sep-history', menu: 'chat', separator: true });
    chatRow('forget-context-choice', { mnemonic: 'X', label: 'Ask Me Again About a Full Context', without: 'This chat has not remembered an answer about a full context.' });
    chatRow('load-full-history', { mnemonic: 'Y', label: 'Load Full History', without: 'Nothing more of the history of this chat can be loaded.' });
    chatRow('drop-queue', { mnemonic: 'Q', label: 'Drop the Queued Messages', without: 'No messages are queued.' });
    // The Broadcast rows follow this separator (decisions-applet.js).
    register({ id: 'chat-sep-broadcast', menu: 'chat', separator: true });
    register({ id: 'chat-sep-stop', menu: 'chat', separator: true });
    chatRow('cancel', { from: 'stop-turn', danger: true, label: () => { const c = ofChat('stop-turn'); return c && !c.hidden ? c.label : 'Stop'; }, without: 'The chat is not working.' });
    register({ id: 'restart', menu: 'chat', label: 'Restart Agent', menuLabel: 'Restart the Agent', run: () => { restartAgent(); } });
    register({ id: 'chat-sep-delete', menu: 'chat', separator: true });
    chatRow('remove-session', { from: 'remove-chat', mnemonic: 'D', danger: true, label: 'Delete Chat…', menuLabel: 'Delete…', without: NOT_STARTED });
    register({ id: 'show-chat', menu: 'chat', inline: true, label: 'Show Chat', run: () => { showChat(); } });
    register({ id: 'find-tool', menu: 'tools', inline: true, label: 'Find a Tool…', run: () => { openGoTo('tools'); } });
    register({ id: 'show-workspace', menu: 'view', inline: true, label: 'Workspace', run: () => { window.dinahWorkbench.open('workspace').catch(fail); } });
    register({ id: 'rename-workspace', menu: 'workspace', inline: true, label: 'Rename Workspace…', enabled: () => workspaceCommands('rename-workspace'), run: () => { openWorkspaceRename(); } });
    register({ id: 'remove-workspace', menu: 'workspace', inline: true, label: 'Remove Workspace…', enabled: () => workspaceCommands('remove-workspace'), danger: true, run: () => { removeWorkspace(); } });
    register({ id: 'export-workspace', menu: 'workspace', inline: true, label: 'Export Workspace…', enabled: () => workspaceCommands('export-workspace'), run: () => { exportWorkspace(); } });
    register({ id: 'import-workspace', menu: 'workspace', inline: true, label: 'Import Workspace…', enabled: () => workspaceCommands('import-workspace'), run: () => { importWorkspace(); } });
    register({ id: 'install-packages-file', menu: 'tools', inline: true, label: 'Install Packages…', run: () => { installPackages(); } });
    register({ id: 'new-workspace', menu: 'workspace', inline: true, label: 'New Workspace…', enabled: () => workspaceCommands('new-workspace'), run: () => { createWorkspace(false); } });
    register({ id: 'new-empty-workspace', menu: 'workspace', inline: true, label: 'New Empty Workspace', enabled: () => workspaceCommands('new-empty-workspace'), run: () => { createWorkspace(true); } });
    register({ id: 'chat-browser', menu: 'view', inline: true, label: 'Chats', run: () => { window.dinahWorkbench.open('chat-browser').catch(fail); } });
    register({ id: 'skills-browser', menu: 'view', inline: true, label: 'Skills', run: () => { window.dinahWorkbench.open('skills-browser').catch(fail); } });
    register({ id: 'files', menu: 'tools', inline: true, label: 'Files', run: () => { dinahResources.currentPath().then(dinahResources.openFiles).catch(fail); } });
    // New Terminal runs the shell chosen in Settings › Terminal; the other
    // runs the product's own shell, over its files and applets, whichever
    // shell is chosen. Neither is drawn in a menu: View is for showing
    // things, and no menu is for starting programs, so they are started from
    // the launcher's Terminals and found by Commands… (Paul, 19 November
    // 2026).
    register({ id: 'terminal', menu: 'tools', inline: true, glyph: 'terminal', label: 'New Terminal', run: () => { dinahResources.currentPath().then(dinahResources.openTerminal).catch(fail); } });
    register({ id: 'own-shell', menu: 'tools', inline: true, glyph: 'script', label: () => 'New ' + window.dinahProduct.name + ' Shell', run: () => { dinahResources.currentPath().then(path => dinahResources.openTerminal(path, null, 'dinah')).catch(fail); } });
    register({ id: 'clear-notifications', menu: 'view', inline: true, label: 'Clear All…', enabled: () => notices.list(entry => entry.clearable).length > 0, danger: true, run: () => { clearNotifications(); } });
    register({ id: 'refresh-workspace', menu: 'workspace', inline: true, label: 'Refresh Workspace', enabled: () => workspaceCommands('refresh-workspace'), run: () => { app().State().then(function (st) { drawWorkspaces(st); refreshSessions(); }).catch(fail); } });
    register({ id: 'settings', menu: 'dinah', inline: true, label: 'Settings…', run: () => { openSettings(); } });
    register({ id: 'refresh-sessions', menu: 'chat', inline: true, label: 'Refresh Chats', run: () => { chatBrowser.refreshNativeSessions(); } });
    register({ id: 'all-models', menu: 'chat', inline: true, label: 'All Models…', enabled: () => chatCommands('all-models'), run: () => { chats.current()?.openModels(); } });
    register({ id: 'sign-in', menu: 'chat', inline: true, label: 'Sign In…', run: () => { signIn(); } });
    register({ id: 'usage-page', menu: 'chat', inline: true, label: 'Usage…', run: () => { var usageItem = window.dinahWorkbench.item(window.pudlWindows.state().top);
        var usageProvider = usageItem && (usageItem.kind === 'chat' || usageItem.kind === 'chat-session') ? frontProvider() : chats.defaultProvider();
        Promise.resolve().then(function () { return app().OpenProviderUsage ? app().OpenProviderUsage(usageProvider) : Promise.reject('No usage page is available for this provider.'); }).catch(fail); } });
    register({ id: 'thoughts', menu: 'view', inline: true, label: "Show the Agent's Thinking", enabled: () => !thinking().busy, checked: () => thinking().checked, run: () => { saveThinking(!thinking().checked); } });
    register({ id: 'about', menu: 'help', inline: true, label: () => 'About ' + productName(), run: () => { document.getElementById('about').showModal(); } });
    register({ id: 'quit', menu: 'dinah', inline: true, label: () => 'Exit ' + productName(), run: () => { Promise.all([flushNotifications(), window.dinahUIState ? window.dinahUIState.flush() : null]).then(function () { if (window.runtime) window.runtime.Quit(); }).catch(fail); } });
    alias('go-to', 'find-and-open');
    register({ id: 'all-windows', menu: 'window', inline: true, search: false, label: 'All Windows…', run: () => openGoTo('windows') });
    register({ id: 'needs-you', menu: 'view', inline: true, search: false, label: 'Show All', run: showNotifications });
    alias('next-notification', 'go-to-question');
    alias('workspace', 'new-workspace');
    alias('new-session', 'new-chat');
    changed();
  }

  window.dinahCommands = { installHost, describe, alias, register: register, run: run, rows: rows, searchable: searchable, changed: changed, flush: flush, controlsFor: controlsFor, shown: shown, menus: MENUS, addHelp: addHelp, addSearch: addSearch };

  register({ id: 'keyboard-shortcuts', menu: 'help', mnemonic: 'K', label: 'Keyboard Shortcuts', shortcut: 'F1', run: keyboardHelp });
})();
