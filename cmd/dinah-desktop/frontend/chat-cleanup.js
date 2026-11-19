/* Clean Up Chats: the window where chats no longer needed are deleted, and
   deleted ones restored or deleted for good. The host contract is
   docs/design/chat-cleanup.md: the host says which chats there are, which
   may be deleted and why not, and what a final deletion will do, and this
   page shows that and asks.

   The window has two tabs, Chats and Recycle Bin. Each is a bar of controls,
   a table and a line that says what is shown or what just happened. A chat
   that leaves a table, because it was deleted, restored or deleted for good,
   is taken out of it at once, and one that could not be changed stays with
   the reason under its title. The approved mockup is
   docs/mockups/clean-up-chats.html (Paul, 17 November 2026). */
(function () {
  'use strict';
  var node = window.dinahHelpers.node;
  var DAY = 86400000;
  var numbers = new Intl.NumberFormat();
  function app() { return window.dinahHelpers.app(); }
  function glyph(name) {
    var g = node('span', 'glyph'); g.style.setProperty('--glyph', 'var(--glyph-' + name + ')'); g.setAttribute('aria-hidden', 'true'); return g;
  }
  function button(cls, text, run) {
    var b = node('button', cls, text); b.type = 'button';
    b.addEventListener('click', function () { Promise.resolve().then(run).catch(window.dinahHost.fail); });
    return b;
  }
  function dayOf(t) { var d = new Date(t); d.setHours(0, 0, 0, 0); return d.getTime(); }
  function daysBetween(from, to) { return Math.round((dayOf(to) - dayOf(from)) / DAY); }
  function chats(n) { return n === 1 ? '1 chat' : n + ' chats'; }
  /* A date and time in the person's regional form, with how long ago, or
     how far off, under it. A time the host does not know says Unknown. */
  function whenCell(value, ahead) {
    var cell = node('td', 'cleanup-when'), t = value ? Date.parse(value) : NaN;
    if (isNaN(t)) { cell.textContent = 'Unknown'; return cell; }
    var days = ahead ? Math.max(0, daysBetween(Date.now(), t)) : daysBetween(t, Date.now());
    var words = ahead ? (days === 0 ? 'Today' : days === 1 ? 'In 1 day' : 'In ' + days + ' days') : (days <= 0 ? 'Today' : days === 1 ? 'Yesterday' : days + ' days ago');
    cell.append(document.createTextNode(window.dinahRegional.format(t, 'date') + (ahead ? '' : ', ' + window.dinahRegional.format(t, 'clock'))), node('small', '', words));
    return cell;
  }
  function programsOf(data) {
    return Array.from(new Set([data.providerName || data.provider].concat((data.programCopies || []).map(function (copy) { return copy.providerName || copy.provider; })).filter(Boolean))).join(', ');
  }
  /* Why a chat cannot be changed, or what the person should know of it,
     under its title, with a glyph as well as the colour. */
  function setReason(entry, text, tone) {
    entry.reason.replaceChildren();
    entry.reason.hidden = !text;
    if (!text) return;
    entry.reason.dataset.tone = tone || 'warning';
    entry.reason.append(glyph(tone === 'info' ? 'info' : 'warning'), node('span', '', text));
  }

  function init(root, opts) {
    root.classList.add('chat-cleanup');
    var listing = null, busy = false, disposed = false, selected = new Set(), active = new Map(), removed = new Map(), authorityRead = null, authorityAgain = false;
    var sort = { key: 'last', dir: 'ascending' }, unlisted = new Set();

    /* ---- The window's parts ------------------------------------------- */
    var tabs = node('div', 'tabs'), tablist = node('div', 'tablist'); tablist.setAttribute('role', 'tablist'); tablist.setAttribute('aria-label', 'Clean Up Chats');
    function tab(id, panel, words, glyphName) {
      var b = node('button'); b.type = 'button'; b.setAttribute('role', 'tab'); b.id = id; b.setAttribute('aria-controls', panel);
      if (glyphName) b.append(glyph(glyphName), document.createTextNode(' '));
      var count = node('span', 'badge cleanup-tab-count', '0');
      b.append(document.createTextNode(words + ' '), count); tablist.append(b);
      return { tab: b, count: count };
    }
    var chatsTab = tab('chat-cleanup-tab-chats', 'chat-cleanup-chats', 'Chats'), binTab = tab('chat-cleanup-tab-bin', 'chat-cleanup-bin', 'Recycle Bin', 'bin');
    chatsTab.tab.setAttribute('aria-selected', 'true'); binTab.tab.setAttribute('aria-selected', 'false');
    function panel(id, by) { var p = node('section'); p.setAttribute('role', 'tabpanel'); p.id = id; p.setAttribute('aria-labelledby', by); var view = node('div', 'cleanup-view'); p.append(view); return { panel: p, view: view }; }
    var chatsPanel = panel('chat-cleanup-chats', 'chat-cleanup-tab-chats'), binPanel = panel('chat-cleanup-bin', 'chat-cleanup-tab-bin');
    binPanel.panel.hidden = true;
    tabs.append(tablist, chatsPanel.panel, binPanel.panel);

    // Chats: which chats are shown.
    var bar = node('div', 'cleanup-bar'); bar.setAttribute('role', 'search'); bar.setAttribute('aria-label', 'Which chats are shown');
    var find = node('input', 'form-input browser-search'); find.type = 'search'; find.placeholder = 'Find a Chat by Title'; find.setAttribute('aria-label', 'Find a chat by title');
    bar.append(find);
    function select(label, choices) {
      var wrap = node('span', 'form-select-wrap'), input = node('select', 'form-select');
      input.setAttribute('aria-label', label);
      choices.forEach(function (choice) { var o = node('option', '', choice[1]); o.value = choice[0]; input.append(o); });
      wrap.append(input); bar.append(wrap); input.addEventListener('change', changed); return input;
    }
    var workspace = select('Workspace', [['', 'All Workspaces']]);
    var provider = select('Agent Program', [['', 'All Agent Programs']]);
    var age = select('Last Activity', [['', 'Any Last Activity'], ['7', 'Older Than 7 Days'], ['30', 'Older Than 30 Days'], ['90', 'Older Than 90 Days'], ['180', 'Older Than 180 Days'], ['365', 'Older Than 1 Year']]);
    var closed = node('button', 'switch'); closed.type = 'button'; closed.setAttribute('role', 'switch'); closed.setAttribute('aria-checked', 'false');
    var track = node('span', 'switch-track'); track.append(node('span', 'switch-thumb')); closed.append(track, document.createTextNode(' Closed Only'));
    closed.addEventListener('click', function () { closed.setAttribute('aria-checked', String(!closedOnly())); changed(); });
    bar.append(closed);
    function closedOnly() { return closed.getAttribute('aria-checked') === 'true'; }
    find.addEventListener('input', changed);

    // Chats: what is selected, and what to do with it.
    var actions = node('div', 'cleanup-actions');
    var count = node('span', 'cleanup-selected', 'None selected'); count.setAttribute('role', 'status');
    var toolbar = node('div', 'toolbar'); toolbar.setAttribute('role', 'toolbar'); toolbar.setAttribute('aria-label', 'Selected chats');
    var group = node('div', 'toolbar-group'); group.setAttribute('role', 'group'); group.setAttribute('aria-label', 'Selection');
    var eraseWords = node('span', '', 'Delete Chats…');
    /* The button says how many chats it will delete, and keeps the width of
       the longest thing it can say: its words share one place with a copy
       of those that is not drawn. */
    var eraseSlot = node('span', 'cleanup-delete-words'), eraseReserve = node('span', 'cleanup-delete-reserve', 'Delete 9,999 Chats…'); eraseReserve.setAttribute('aria-hidden', 'true');
    eraseSlot.append(eraseWords, eraseReserve);
    var erase = button('btn btn-sm btn-primary cleanup-delete', undefined, confirmRemoval); erase.append(glyph('bin'), eraseSlot);
    var clear = button('btn btn-sm', 'Clear Selection', function () { selected.clear(); controls(); });
    group.append(erase, clear); toolbar.append(group);
    var refresh = button('icon-btn', undefined, load); refresh.setAttribute('aria-label', 'Refresh'); refresh.dataset.tooltip = 'Read the Chats Again'; refresh.append(glyph('reload'));
    actions.append(count, toolbar, refresh);

    // Chats: the table.
    function table(label) {
      var wrap = node('div', 'data-table-wrap cleanup-table'), t = node('table', 'data-table'), head = node('thead'), row = node('tr'), body = node('tbody');
      t.setAttribute('aria-label', label); head.append(row); t.append(head, body); wrap.append(t);
      return { wrap: wrap, head: row, body: body };
    }
    var chatsTable = table('Chats');
    var allCell = node('th', 'data-table-check'), all = node('input'); allCell.scope = 'col'; all.type = 'checkbox'; all.setAttribute('aria-label', 'Select every chat shown'); allCell.append(all);
    chatsTable.head.append(allCell);
    var sortHeads = {};
    [['title', 'Chat'], ['workspace', 'Workspace'], ['programs', 'Agent Programs'], ['last', 'Last Activity'], ['messages', 'Messages', 'num'], ['state', 'State']].forEach(function (column) {
      var th = node('th', column[2] || ''), press = node('button', '', column[1]); th.scope = 'col'; press.type = 'button'; th.setAttribute('aria-sort', 'none');
      press.addEventListener('click', function () { sort = { key: column[0], dir: sort.key === column[0] && sort.dir === 'ascending' ? 'descending' : 'ascending' }; changed(); });
      th.append(press); chatsTable.head.append(th); sortHeads[column[0]] = th;
    });
    var chatsEmpty = node('tr', 'data-table-empty'), chatsEmptyCell = node('td'); chatsEmptyCell.colSpan = 7; chatsEmpty.append(chatsEmptyCell);
    all.addEventListener('change', function () {
      active.forEach(function (entry) { if (entry.row.hidden || !selectable(entry)) return; if (all.checked) selected.add(entry.data.sessionId); else selected.delete(entry.data.sessionId); });
      controls();
    });
    function foot() { var f = node('div', 'cleanup-foot'), words = node('span'); words.setAttribute('role', 'status'); f.append(words); return { el: f, words: words, said: '' }; }
    var chatsFoot = foot(), binFoot = foot();
    chatsPanel.view.append(bar, actions, chatsTable.wrap, chatsFoot.el);

    // Recycle Bin.
    var binActions = node('div', 'cleanup-actions');
    var note = node('p', 'cleanup-note', 'A deleted chat stays here for 30 days and can be restored until then. It uses nothing meanwhile: it is not loaded and receives no messages.');
    var empty = button('btn btn-sm btn-danger', 'Empty Recycle Bin…', function () {
      confirm('Empty the Recycle Bin?', listing.finalDeleteConfirmation, binTitles(), 'Delete All for Good', true, function () { return run(function () { return app().EmptyChatRecycleBin(); }, true); });
    });
    binActions.append(note, empty);
    var binTable = table('Recycle Bin');
    ['Chat', 'Workspace', 'Agent Programs', 'Messages', 'Deleted', 'Deleted for Good'].forEach(function (words) { var th = node('th', words === 'Messages' ? 'num' : '', words); th.scope = 'col'; binTable.head.append(th); });
    var lastHead = node('th'); lastHead.scope = 'col'; lastHead.append(node('span', 'visually-hidden', 'Actions')); binTable.head.append(lastHead);
    var binEmpty = node('tr', 'data-table-empty'), binEmptyCell = node('td', '', 'The Recycle Bin is empty.'); binEmptyCell.colSpan = 7; binEmpty.append(binEmptyCell);
    binPanel.view.append(binActions, binTable.wrap, binFoot.el);
    root.append(tabs);
    if (window.pudlTabs) window.pudlTabs.enhance(tabs);

    /* ---- What is shown ------------------------------------------------- */
    function address() {
      var p = new URLSearchParams();
      if (workspace.value) p.set('workspace', workspace.value);
      if (provider.value) p.set('agent', provider.value);
      if (age.value) p.set('age', age.value);
      if (closedOnly()) p.set('closed', '1');
      if (find.value.trim()) p.set('find', find.value.trim());
      if (sort.key !== 'last' || sort.dir !== 'ascending') { p.set('sort', sort.key); p.set('dir', sort.dir); }
      return p.toString();
    }
    function read(state) {
      var p = new URLSearchParams(state || '');
      [[workspace, p.get('workspace')], [provider, p.get('agent')]].forEach(function (pair) {
        if (pair[1] && !Array.from(pair[0].options).some(function (o) { return o.value === pair[1]; })) { var o = node('option', '', pair[1]); o.value = pair[1]; pair[0].append(o); }
        pair[0].value = pair[1] || '';
      });
      age.value = p.get('age') || '';
      closed.setAttribute('aria-checked', String(p.get('closed') === '1'));
      find.value = p.get('find') || '';
      sort = sortHeads[p.get('sort')] ? { key: p.get('sort'), dir: p.get('dir') === 'descending' ? 'descending' : 'ascending' } : { key: 'last', dir: 'ascending' };
    }
    function shown(row) {
      var normalized = function (value) { return (value || '').toLowerCase().replace(/[\\/]+$/, ''); };
      var words = find.value.trim().toLowerCase();
      return (!words || (row.title || '').toLowerCase().indexOf(words) >= 0) &&
        (!workspace.value || !row.workspace || normalized(row.workspace) === normalized(workspace.value)) &&
        (!provider.value || row.provider === provider.value || (row.programCopies || []).some(function (copy) { return copy.provider === provider.value; })) &&
        (!closedOnly() || !row.open) &&
        (!age.value || !!row.lastActivity && Date.parse(row.lastActivity) < Date.now() - Number(age.value) * DAY);
    }
    function sortValue(entry) {
      var data = entry.data;
      if (sort.key === 'title') return (data.title || '').toLowerCase();
      if (sort.key === 'workspace') return (data.workspaceName || data.workspace || '').toLowerCase();
      if (sort.key === 'programs') return programsOf(data).toLowerCase();
      if (sort.key === 'messages') return data.messageCountKnown ? data.messageCount : -1;
      if (sort.key === 'state') return entry.stateWords || '';
      var t = data.lastActivity ? Date.parse(data.lastActivity) : NaN;
      return isNaN(t) ? -Infinity : t; // a chat with no known activity counts as the oldest
    }
    function changed() { opts.changed(address()); selected.clear(); applyView(); }
    function selectable(entry) { return !!entry.data.eligible && !entry.absent; }
    /* Puts the rows in order, shows the ones that match, and says how many. */
    function applyView() {
      var sign = sort.dir === 'ascending' ? 1 : -1, list = Array.from(active.values()), shownCount = 0, total = 0;
      list.sort(function (a, b) {
        var x = sortValue(a), y = sortValue(b);
        return (typeof x === 'number' ? x - y : x < y ? -1 : x > y ? 1 : 0) * sign || (a.data.title || '').localeCompare(b.data.title || '') || a.data.sessionId.localeCompare(b.data.sessionId);
      });
      chatsEmpty.remove();
      list.forEach(function (entry) {
        var show = !entry.absent && shown(entry.data);
        if (entry.row.hidden !== !show) entry.row.hidden = !show;
        if (!entry.absent) total++;
        if (show) shownCount++; else selected.delete(entry.data.sessionId);
        chatsTable.body.append(entry.row);
      });
      if (!shownCount) { chatsEmptyCell.textContent = total ? 'No chat matches. Change what is shown above.' : 'There are no chats.'; chatsTable.body.append(chatsEmpty); }
      Object.keys(sortHeads).forEach(function (key) { sortHeads[key].setAttribute('aria-sort', key === sort.key ? sort.dir : 'none'); });
      chatsTab.count.textContent = String(total);
      chatsFoot.resting = (shownCount === total ? chats(total) + '.' : shownCount + ' of ' + chats(total) + ' shown.') + ' A deleted chat goes to the Recycle Bin and can be restored for 30 days.';
      if (!chatsFoot.said) chatsFoot.words.textContent = chatsFoot.words.title = chatsFoot.resting;
      controls();
    }
    function applyBin() {
      binEmpty.remove();
      var list = Array.from(removed.values()).sort(function (a, b) { return String(a.data.removedAt || '').localeCompare(String(b.data.removedAt || '')) || a.data.sessionId.localeCompare(b.data.sessionId); });
      list.forEach(function (entry) { binTable.body.append(entry.row); });
      if (!list.length) binTable.body.append(binEmpty);
      binTab.count.textContent = String(list.length);
      binFoot.resting = list.length === 1 ? '1 deleted chat.' : list.length + ' deleted chats.';
      if (!binFoot.said) binFoot.words.textContent = binFoot.words.title = binFoot.resting;
      controls();
    }
    function binTitles() { return Array.from(removed.values()).map(function (entry) { return entry.data.title || 'Untitled Chat'; }); }
    /* A line that says what just happened stays until the next thing does. */
    function say(where, words, action) {
      where.said = words; where.words.textContent = where.words.title = words || where.resting || '';
      if (words) where.words.dataset.tone = 'result'; else delete where.words.dataset.tone;
      where.el.querySelectorAll('.btn').forEach(function (b) { b.remove(); });
      if (words && action) where.el.append(action);
    }
    function controls() {
      var boxes = [];
      active.forEach(function (entry) {
        var can = selectable(entry);
        if (!can) selected.delete(entry.data.sessionId);
        entry.check.checked = selected.has(entry.data.sessionId);
        entry.check.disabled = busy || !can;
        if (can && !entry.row.hidden) boxes.push(entry.check);
      });
      removed.forEach(function (entry) {
        entry.restore.disabled = busy || !!authorityRead || !entry.data.restorable;
        entry.erase.disabled = busy || !!authorityRead;
      });
      var n = selected.size;
      count.textContent = n ? n + ' selected' : 'None selected';
      eraseWords.textContent = n === 1 ? 'Delete 1 Chat…' : n ? 'Delete ' + numbers.format(n) + ' Chats…' : 'Delete Chats…';
      erase.disabled = busy || !n; clear.disabled = busy || !n;
      all.disabled = busy || !boxes.length;
      all.checked = !!boxes.length && boxes.every(function (b) { return b.checked; });
      all.indeterminate = !all.checked && boxes.some(function (b) { return b.checked; });
      [refresh, find, workspace, provider, age, closed].forEach(function (el) { el.disabled = busy; });
      empty.disabled = busy || !!authorityRead || !listing || !removed.size;
    }

    /* ---- The rows ------------------------------------------------------ */
    function nameCell(entry, data) {
      var cell = node('td', 'cleanup-chat');
      entry.title = node('span', 'cleanup-title', data.title || 'Untitled Chat');
      entry.reason = node('span', 'cleanup-reason'); entry.reason.hidden = true; entry.reason.setAttribute('role', 'status');
      cell.append(entry.title, entry.reason);
      return cell;
    }
    function fillActive(entry) {
      var data = entry.data, cells = entry.cells;
      entry.title.textContent = data.title || 'Untitled Chat';
      entry.check.setAttribute('aria-label', 'Select ' + (data.title || 'Untitled Chat'));
      cells.workspace.textContent = data.workspaceName || data.workspace || 'No Workspace';
      cells.programs.textContent = programsOf(data);
      cells.when.replaceWith(cells.when = whenCell(data.lastActivity));
      cells.messages.textContent = data.messageCountKnown ? numbers.format(data.messageCount) : 'Unknown';
      setReason(entry, data.eligible ? '' : data.refusal || 'This chat cannot be deleted now.');
    }
    function addActive(data) {
      if (active.has(data.sessionId)) return active.get(data.sessionId);
      var entry = { data: data, absent: false }, row = node('tr'), checkCell = node('td', 'data-table-check'), check = node('input'); check.type = 'checkbox';
      row.dataset.chatId = data.sessionId; entry.row = row; entry.check = check; checkCell.append(check);
      check.addEventListener('change', function () { if (check.checked) selected.add(data.sessionId); else selected.delete(data.sessionId); controls(); });
      entry.cells = { workspace: node('td'), programs: node('td'), when: node('td', 'cleanup-when'), messages: node('td', 'num'), state: node('td') };
      row.append(checkCell, nameCell(entry, data), entry.cells.workspace, entry.cells.programs, entry.cells.when, entry.cells.messages, entry.cells.state);
      fillActive(entry);
      active.set(data.sessionId, entry);
      return entry;
    }
    function binNote(data) { return [data.runtimeNote, data.restoreRefusal].filter(Boolean).join(' '); }
    function addBin(data) {
      if (removed.has(data.sessionId)) return;
      var entry = { data: data }, row = node('tr'); row.dataset.chatId = data.sessionId; entry.row = row;
      var actionsCell = node('td'), wrap = node('div', 'cleanup-row-actions'), name = data.title || 'Untitled Chat';
      entry.restore = button('btn btn-sm', 'Restore', function () { return run(function () { return app().RestoreRemovedChat(data.sessionId); }, true); });
      entry.restore.setAttribute('aria-label', 'Restore ' + name);
      entry.erase = button('btn btn-sm', 'Delete Now…', function () {
        confirm('Delete This Chat for Good?', entry.data.finalDeleteConfirmation, [name], 'Delete for Good', true, function () { return run(function () { return app().DeleteRemovedChat(data.sessionId); }, true); });
      });
      entry.erase.setAttribute('aria-label', 'Delete ' + name + ' now');
      wrap.append(entry.restore, entry.erase); actionsCell.append(wrap);
      row.append(nameCell(entry, data), node('td', '', data.workspaceName || data.workspace || 'No Workspace'), node('td', '', programsOf(data)),
        node('td', 'num', data.messageCountKnown ? numbers.format(data.messageCount) : 'Unknown'), whenCell(data.removedAt), whenCell(data.expiresAt, true), actionsCell);
      setReason(entry, binNote(data), data.restoreRefusal ? 'warning' : 'info');
      removed.set(data.sessionId, entry);
    }
    function choices(input, rows, key, label) {
      var value = input.value, values = new Map(); rows.forEach(function (r) { if (r[key]) values.set(r[key], r[label] || r[key]); });
      values.forEach(function (name, id) { if (!Array.from(input.options).some(function (o) { return o.value === id; })) { var o = node('option', '', name); o.value = id; input.append(o); } }); input.value = value;
    }

    /* ---- Keeping in step with the chats -------------------------------- */
    function inventoryMap() { return new Map(window.dinahChats.inventory().map(function (chat) { return [chat.sessionId, chat]; })); }
    function syncInventory() {
      if (disposed || !listing) return;
      var inventory = inventoryMap();
      active.forEach(function (entry, id) {
        var chat = inventory.get(id); entry.absent = !chat;
        if (!chat) { selected.delete(id); return; }
        if (entry.data.title !== chat.title) { entry.data.title = chat.title; entry.title.textContent = chat.title; entry.check.setAttribute('aria-label', 'Select ' + chat.title); }
        var badge = window.dinahChatState.badge(chat.state); badge.classList.add('cleanup-state');
        entry.stateWords = badge.textContent.trim();
        entry.cells.state.replaceChildren(badge);
      });
      applyView();
      unlisted.forEach(function (id) { if (!inventory.has(id)) unlisted.delete(id); });
      if (Array.from(inventory.keys()).some(function (id) { return !active.has(id) && !unlisted.has(id); })) refreshAuthority();
    }
    function refreshAuthority(invalidated) {
      if (disposed || !listing) return;
      if (authorityRead) { if (invalidated) authorityAgain = true; return; }
      authorityRead = Promise.resolve(app().ListChatCleanup()).then(function (value) {
        if (disposed) return;
        var inventory = inventoryMap();
        value.rows.forEach(function (data) {
          if (!inventory.has(data.sessionId)) return;
          data = Object.assign({}, data, { title: inventory.get(data.sessionId).title });
          var entry = active.get(data.sessionId);
          if (!entry) addActive(data); else { entry.data = data; entry.absent = false; fillActive(entry); }
        });
        var binIds = new Set(value.removed.map(function (data) { return data.sessionId; }));
        removed.forEach(function (entry, id) { if (!binIds.has(id)) { entry.row.remove(); removed.delete(id); } });
        value.removed.forEach(addBin);
        /* A chat the page still lists and the host no longer does, such as
           one deleted a moment ago, is not asked about again: asking is what
           this reading just did, and each further reading would ask once more. */
        inventory.forEach(function (chat, id) { if (!active.has(id)) unlisted.add(id); });
        syncInventory(); applyBin();
      }).catch(function (error) { say(chatsFoot, window.dinahHelpers.message(error, 'sentence')); }).finally(function () {
        authorityRead = null;
        if (authorityAgain) { authorityAgain = false; refreshAuthority(); }
        if (!disposed) controls();
      });
      controls();
    }
    function load() {
      busy = true; controls(); say(chatsFoot, 'Reading the chats…');
      return Promise.resolve().then(function () { return window.dinahChats.refreshInventory(); }).then(function () { return app().ListChatCleanup(); }).then(function (value) {
        if (disposed) return;
        if (!value || !Array.isArray(value.rows) || !Array.isArray(value.removed) || value.retentionDays !== 30) throw new Error('The chat list could not be read.');
        var inventory = inventoryMap();
        value.rows = value.rows.filter(function (row) { return inventory.has(row.sessionId); }).map(function (row) { return Object.assign({}, row, { title: inventory.get(row.sessionId).title }); });
        var authoritative = new Set(value.rows.map(function (row) { return row.sessionId; }));
        if (Array.from(inventory.keys()).some(function (id) { return !authoritative.has(id); })) throw new Error('Some chats have no authoritative cleanup record. Refresh before deleting them.');
        listing = value; active.clear(); removed.clear(); unlisted.clear(); chatsTable.body.replaceChildren(); binTable.body.replaceChildren(); selected.clear();
        choices(workspace, value.rows, 'workspace', 'workspaceName');
        var programs = value.rows.flatMap(function (r) { return [r].concat(r.programCopies || []); }); choices(provider, programs, 'provider', 'providerName');
        value.rows.forEach(addActive); value.removed.forEach(addBin);
        say(chatsFoot, ''); say(binFoot, '');
        syncInventory(); applyBin();
      }).catch(function (error) { say(chatsFoot, window.dinahHelpers.message(error, 'sentence')); }).finally(function () { busy = false; if (!disposed) controls(); });
    }

    /* ---- Doing it ------------------------------------------------------ */
    /* Takes one result. A chat that was changed leaves its table, and the
       tally says what to report; one that was not keeps its row, with the
       host's reason under its title. */
    function apply(result, isBin, tally) {
      var entries = isBin ? removed : active, entry = entries.get(result.sessionId);
      if (!entry) return;
      if (!result.ok) {
        if (result.row) entry.data = result.row;
        entry.row.dataset.outcome = result.status || 'failed';
        setReason(entry, result.error || 'The chat could not be changed.');
        tally.failed++;
        return;
      }
      entry.row.remove(); entries.delete(result.sessionId); selected.delete(result.sessionId);
      tally.names.push(entry.data.title || 'Untitled Chat');
      // The page's own list drops a deleted chat a moment later; until then the host is not asked about it again.
      if (result.status === 'removed') unlisted.add(result.sessionId);
      if (result.status === 'removed') { tally.removed++; if (result.row) addBin(Object.assign({}, result.row, { title: result.row.title || entry.data.title })); }
      if (result.status === 'restored') {
        tally.restored++;
        if (result.row) {
          window.dinahChats.restore(result.row);
          var stale = active.get(result.row.sessionId);
          if (stale) { stale.row.remove(); active.delete(result.row.sessionId); }
          addActive(result.row);
        }
      }
      if (result.status === 'deleted') {
        tally.deleted++;
        (entry.data.sessionIds || [result.sessionId]).forEach(function (id) { window.dinahComposer.forget('chat:' + id); });
        if (result.providerCopyRemains && result.providerDeletionReason && tally.copies.indexOf(result.providerDeletionReason) < 0) tally.copies.push(result.providerDeletionReason);
      }
    }
    function report(tally, isBin) {
      var one = tally.names.length === 1 ? '"' + tally.names[0] + '"' : '';
      var parts = [];
      if (tally.removed) parts.push((one ? one + ' was' : chats(tally.removed) + (tally.removed === 1 ? ' was' : ' were')) + ' moved to the Recycle Bin.');
      if (tally.restored) parts.push((one ? one + ' was' : chats(tally.restored) + (tally.restored === 1 ? ' was' : ' were')) + ' restored as ' + (tally.restored === 1 ? 'a closed chat.' : 'closed chats.'));
      if (tally.deleted) parts.push((one ? one + ' was' : chats(tally.deleted) + (tally.deleted === 1 ? ' was' : ' were')) + ' deleted for good.');
      if (tally.copies.length) parts.push(tally.copies.join(' '));
      if (tally.failed) parts.push((tally.failed === 1 ? '1 chat' : tally.failed + ' chats') + ' could not be changed; the reason is under ' + (tally.failed === 1 ? 'its title.' : 'each title.'));
      var action = null;
      if (!isBin && tally.removed) action = button('btn btn-sm', 'Show Recycle Bin', function () { binTab.tab.click(); });
      say(isBin ? binFoot : chatsFoot, parts.join(' ') || 'Nothing was changed.', action);
    }
    function run(call, isBin) {
      busy = true; controls(); say(isBin ? binFoot : chatsFoot, 'Updating the chats…');
      var tally = { removed: 0, restored: 0, deleted: 0, failed: 0, names: [], copies: [] };
      return Promise.resolve().then(call).then(function (results) {
        if (disposed) return;
        (Array.isArray(results) ? results : [results]).forEach(function (result) { apply(result, isBin, tally); });
        report(tally, isBin);
        syncInventory(); applyBin();
      }).catch(function (error) { say(isBin ? binFoot : chatsFoot, window.dinahHelpers.message(error, 'sentence') + ' Refresh before trying again.'); })
        .finally(function () { busy = false; if (!disposed) controls(); });
    }
    /* The dialog says what will happen in the host's words, or in the page's
       for a deletion that can be undone, and lists the chats. Cancel comes
       first and has the focus. A deletion for good is marked as dangerous;
       one that goes to the Recycle Bin is not, since it can be restored. */
    function confirm(title, text, names, accept, danger, action) {
      var where = danger ? binFoot : chatsFoot;
      if (!text) { say(where, 'The confirmation could not be read. Refresh before trying again.'); return; }
      var dialog = node('dialog', 'dialog cleanup-confirm'), heading = node('h2', 'dialog-title', title), body = node('p', 'dialog-body', text), list = node('ul'), buttons = node('div', 'dialog-actions');
      dialog.setAttribute('aria-label', title); var opener = document.activeElement;
      names.forEach(function (name) { list.append(node('li', '', name)); });
      var yes = button('btn ' + (danger ? 'btn-danger' : 'btn-primary'), accept, function () { dialog.close(); return action(); });
      var no = button('btn', 'Cancel', function () { dialog.close(); });
      buttons.append(no, yes); dialog.append(heading, body, list, buttons);
      dialog.addEventListener('close', function () { dialog.remove(); if (opener && opener.isConnected) opener.focus({ preventScroll: true }); });
      document.body.append(dialog); dialog.showModal(); no.focus();
    }
    function confirmRemoval() {
      var ids = Array.from(selected), names = ids.map(function (id) { return active.get(id).data.title || 'Untitled Chat'; });
      confirm(ids.length === 1 ? 'Delete 1 Chat?' : 'Delete ' + ids.length + ' Chats?',
        'These chats go to the Recycle Bin now and can be restored for 30 days. They receive no further messages. A chat whose agent program cannot unload it may stay loaded until that program stops or the window restarts. The agent programs\' own copies are left alone until a chat is deleted for good.',
        names, 'Delete', false, function () { return run(function () { return app().RemoveChats(ids); }); });
    }

    document.addEventListener('dinah:chats', syncInventory); document.addEventListener('dinah:execution-state', syncInventory);
    var stops = ['chat:removed', 'chat:restored'].map(function (event) { return window.dinahHost.on(event, function () { refreshAuthority(true); }); });
    read(opts.state); load();
    return {
      state: address,
      setState: function (state) { read(state); selected.clear(); applyView(); },
      destroy: function () { disposed = true; stops.forEach(function (stop) { stop(); }); document.removeEventListener('dinah:chats', syncInventory); document.removeEventListener('dinah:execution-state', syncInventory); }
    };
  }
  function open() { return window.dinahWorkbench.open('chat-cleanup'); }
  window.dinahWorkbench.register('chat-cleanup', { applet: { id: 'dinah.chat-cleanup', title: 'Clean Up Chats', glyph: 'bin', workspace: false }, launcher: { run: open }, singleton: true, title: 'Clean Up Chats', glyph: 'bin', mode: 'maximized', init: init });
  window.dinahChatCleanup = { open: open };
  window.dinahCommands.register({ id: 'clean-up-chats', menu: 'tools', mnemonic: 'U', label: 'Clean Up Chats', glyph: 'bin', run: open });
}());
