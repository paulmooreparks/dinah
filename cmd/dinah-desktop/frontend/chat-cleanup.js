/* The host contract is docs/design/chat-cleanup.md. Row permissions come from the host. */
(function () {
  'use strict';
  var node = window.dinahHelpers.node;
  function app() { return window.dinahHelpers.app(); }
  function button(text, run) {
    var b = node('button', 'btn btn-sm', text); b.type = 'button';
    b.addEventListener('click', function () { Promise.resolve().then(run).catch(window.dinahHost.fail); }); return b;
  }
  function date(value) { return value ? new Date(value).toLocaleString() : 'Unknown'; }
  function init(root, opts) {
    root.classList.add('chat-cleanup');
    var listing = null, busy = false, disposed = false, selected = new Set(), active = new Map(), removed = new Map(), authorityRead = null;
    var toolbar = node('div', 'cleanup-toolbar'), filters = node('div', 'cleanup-filters');
    function select(label, choices) {
      var wrap = node('label', 'form-label', label), input = node('select', 'form-select');
      input.setAttribute('aria-label', label);
      choices.forEach(function (choice) { var o = node('option', '', choice[1]); o.value = choice[0]; input.append(o); });
      wrap.append(input); filters.append(wrap); input.addEventListener('change', changed); return input;
    }
    var workspace = select('Workspace', [['', 'All Workspaces']]);
    var provider = select('Agent Program', [['', 'All Programs']]);
    var age = select('Last Activity', [['', 'Any Time'], ['7', 'Older Than 7 Days'], ['30', 'Older Than 30 Days'], ['90', 'Older Than 90 Days'], ['180', 'Older Than 180 Days'], ['365', 'Older Than 1 Year']]);
    var closedLabel = node('label', 'cleanup-closed'), closed = node('input'); closed.type = 'checkbox'; closedLabel.append(closed, document.createTextNode(' Closed Only')); filters.append(closedLabel); closed.addEventListener('change', changed);
    var all = button('Select All Shown', function () { active.forEach(function (entry) { if (!entry.el.hidden && entry.data.eligible && !entry.done) selected.add(entry.data.sessionId); }); controls(); });
    var clear = button('Clear Selection', function () { selected.clear(); controls(); });
    var count = node('span', 'cleanup-count', '0 selected'); count.setAttribute('role', 'status');
    var erase = button('Delete 0 Chats…', confirmRemoval); erase.classList.add('cleanup-delete', 'btn-primary');
    var refresh = button('Refresh', load); toolbar.append(all, clear, count, erase, refresh);
    var status = node('p', 'cleanup-status', 'Loading chats.'); status.setAttribute('role', 'status');
    var lists = node('div', 'cleanup-lists');
    function section(title) {
      var section = node('section', 'cleanup-section'), heading = node('h2', '', title), rows = node('ul', 'cleanup-rows'); rows.setAttribute('aria-label', title);
      section.append(heading, rows); lists.append(section); return { section: section, rows: rows, heading: heading };
    }
    var chats = section('Chats'), bin = section('Recycle Bin');
    var empty = button('Empty Recycle Bin…', function () { confirm('Empty Recycle Bin?', listing.finalDeleteConfirmation, function () { return run(function () { return app().EmptyChatRecycleBin(); }, true); }); });
    bin.heading.after(empty); root.append(filters, toolbar, status, lists);
    function address() { var p = new URLSearchParams(); if (workspace.value) p.set('workspace', workspace.value); if (provider.value) p.set('agent', provider.value); if (age.value) p.set('age', age.value); if (closed.checked) p.set('closed', '1'); return p.toString(); }
    function read(state) {
      var p = new URLSearchParams(state || '');
      [[workspace, p.get('workspace')], [provider, p.get('agent')]].forEach(function (pair) {
        if (pair[1] && !Array.from(pair[0].options).some(function (o) { return o.value === pair[1]; })) { var o = node('option', '', pair[1]); o.value = pair[1]; pair[0].append(o); }
        pair[0].value = pair[1] || '';
      }); age.value = p.get('age') || ''; closed.checked = p.get('closed') === '1';
    }
    function shown(row) {
      var normalized = function (value) { return (value || '').toLowerCase().replace(/[\\/]+$/, ''); };
      return (!workspace.value || !row.workspace || normalized(row.workspace) === normalized(workspace.value)) && (!provider.value || row.provider === provider.value || (row.programCopies || []).some(function (copy) { return copy.provider === provider.value; })) && (!closed.checked || !row.open) &&
        (!age.value || !!row.lastActivity && Date.parse(row.lastActivity) < Date.now() - Number(age.value) * 86400000);
    }
    function changed() { opts.changed(address()); selected.clear(); applyFilters(); }
    function applyFilters() { active.forEach(function (entry) { entry.el.hidden = entry.absent || !shown(entry.data); }); controls(); }
    function controls() {
      active.forEach(function (entry) { entry.check.checked = selected.has(entry.data.sessionId); entry.check.disabled = busy || !entry.data.eligible || entry.done; });
      removed.forEach(function (entry) { entry.restore.disabled = busy || entry.done || !entry.data.restorable; entry.erase.disabled = busy || entry.done; });
      count.textContent = selected.size + ' selected'; erase.textContent = 'Delete ' + selected.size + (selected.size === 1 ? ' Chat…' : ' Chats…'); erase.disabled = busy || !selected.size;
      [all, clear, refresh, workspace, provider, age, closed].forEach(function (el) { el.disabled = busy; });
      empty.disabled = busy || !listing || !Array.from(removed.values()).some(function (entry) { return !entry.done; });
    }
    function row(data, isBin) {
      var el = node('li', 'cleanup-row' + (isBin ? ' cleanup-bin-row' : '')), main = node('div', 'cleanup-row-main'), check = node('input'); check.type = 'checkbox'; check.setAttribute('aria-label', 'Select ' + data.title);
      el.dataset.chatId = data.sessionId;
      var title = node('strong', 'cleanup-title', data.title || 'Untitled Chat'), meta = node('div', 'cleanup-meta');
      var programs = Array.from(new Set([data.providerName || data.provider].concat((data.programCopies || []).map(function (copy) { return copy.providerName || copy.provider; })))).join(', ');
      [data.workspaceName || data.workspace || 'No Workspace', programs, 'Last activity: ' + date(data.lastActivity), 'Messages: ' + (data.messageCountKnown ? data.messageCount : 'Unknown')].forEach(function (text) { meta.append(node('span', '', text)); });
      var reason = node('p', 'cleanup-reason'), actions = node('div', 'cleanup-row-actions'); reason.setAttribute('role', 'status');
      var entry = { data: data, el: el, check: check, reason: reason, title: title, meta: meta, done: false };
      if (data.refusal || data.runtimeNote || data.restoreRefusal) reason.dataset.status = 'warning';
      if (isBin) {
        entry.restore = button('Restore', function () { return run(function () { return app().RestoreRemovedChat(data.sessionId); }, true); });
        entry.erase = button('Delete Now…', function () { confirm('Delete This Chat Permanently?', entry.data.finalDeleteConfirmation, function () { return run(function () { return app().DeleteRemovedChat(data.sessionId); }, true); }); });
        actions.append(entry.restore, entry.erase);
        meta.append(node('span', '', 'Deleted: ' + date(data.removedAt)), node('span', '', 'Expires: ' + date(data.expiresAt)));
        reason.textContent = [data.runtimeNote, data.restoreRefusal, data.finalDeleteConfirmation].filter(Boolean).join(' ');
      } else {
        check.addEventListener('change', function () { if (check.checked) selected.add(data.sessionId); else selected.delete(data.sessionId); controls(); });
        main.append(check); reason.textContent = data.refusal || (data.open ? 'Open' : 'Closed');
      }
      main.append(title); el.append(main, meta, reason, actions); return entry;
    }
    function add(data, isBin) { var entries = isBin ? removed : active, target = isBin ? bin.rows : chats.rows; if (entries.has(data.sessionId)) return; var entry = row(data, isBin); entries.set(data.sessionId, entry); target.append(entry.el); }
    function choices(input, rows, key, label) {
      var value = input.value, values = new Map(); rows.forEach(function (r) { if (r[key]) values.set(r[key], r[label] || r[key]); });
      values.forEach(function (name, id) { if (!Array.from(input.options).some(function (o) { return o.value === id; })) { var o = node('option', '', name); o.value = id; input.append(o); } }); input.value = value;
    }
    function syncInventory() {
      if (disposed || !listing) return;
      var inventory = new Map(window.dinahChats.inventory().map(function (chat) { return [chat.sessionId, chat]; }));
      active.forEach(function (entry, id) {
        var chat = inventory.get(id); entry.absent = !chat && !entry.done;
        if (!chat) { selected.delete(id); return; }
        entry.data.title = chat.title; entry.title.textContent = chat.title;
        entry.check.setAttribute('aria-label', 'Select ' + chat.title);
        if (entry.state) entry.state.remove();
        entry.state = window.dinahChatState.badge(chat.state); entry.state.classList.add('cleanup-state'); entry.meta.append(entry.state);
      });
      applyFilters();
      if (Array.from(inventory.keys()).some(function (id) { return !active.has(id); })) refreshAuthority();
    }
    function refreshAuthority() {
      if (disposed || !listing || authorityRead) return;
      authorityRead = Promise.resolve(app().ListChatCleanup()).then(function (value) {
        if (disposed) return;
        var inventory = new Map(window.dinahChats.inventory().map(function (chat) { return [chat.sessionId, chat]; }));
        value.rows.forEach(function (data) {
          if (!inventory.has(data.sessionId)) return;
          data = Object.assign({}, data, { title: inventory.get(data.sessionId).title });
          var entry = active.get(data.sessionId);
          if (!entry) add(data, false);
          else if (entry.absent || entry.done) { entry.data = data; entry.absent = false; entry.done = false; delete entry.el.dataset.outcome; entry.reason.textContent = data.refusal || (data.open ? 'Open' : 'Closed'); }
        });
        value.removed.forEach(function (data) { add(data, true); }); syncInventory();
      }).catch(function (error) { status.textContent = window.dinahHelpers.message(error, 'sentence'); }).finally(function () { authorityRead = null; });
    }
    function load() {
      busy = true; controls(); status.textContent = 'Loading chats.';
      return Promise.resolve().then(function () { return window.dinahChats.refreshInventory(); }).then(function () { return app().ListChatCleanup(); }).then(function (value) {
        if (disposed) return;
        if (!value || !Array.isArray(value.rows) || !Array.isArray(value.removed) || value.retentionDays !== 30) throw new Error('The chat list could not be read.');
        var inventory = new Map(window.dinahChats.inventory().map(function (chat) { return [chat.sessionId, chat]; }));
        value.rows = value.rows.filter(function (row) { return inventory.has(row.sessionId); }).map(function (row) { return Object.assign({}, row, { title: inventory.get(row.sessionId).title }); });
        var authoritative = new Set(value.rows.map(function (row) { return row.sessionId; }));
        if (Array.from(inventory.keys()).some(function (id) { return !authoritative.has(id); })) throw new Error('Some chats have no authoritative cleanup record. Refresh before deleting them.');
        listing = value; active.clear(); removed.clear(); chats.rows.replaceChildren(); bin.rows.replaceChildren(); selected.clear();
        choices(workspace, value.rows, 'workspace', 'workspaceName');
        var programs = value.rows.flatMap(function (r) { return [r].concat(r.programCopies || []); }); choices(provider, programs, 'provider', 'providerName');
        value.rows.forEach(function (r) { add(r, false); }); value.removed.forEach(function (r) { add(r, true); }); syncInventory(); status.textContent = value.rows.length + ' chats. Deleted chats stay in the Recycle Bin for 30 days.';
      }).catch(function (error) { status.textContent = window.dinahHelpers.message(error, 'sentence'); }).finally(function () { busy = false; if (!disposed) controls(); });
    }
    function apply(result, isBin) {
      var entry = (isBin ? removed : active).get(result.sessionId);
      if (!entry) return;
      entry.el.dataset.outcome = result.status;
      if (result.ok) {
        if (result.status === 'restored' && result.row) { window.dinahChats.restore(result.row); var current = active.get(result.row.sessionId); if (current) { current.data = result.row; current.done = false; current.absent = false; delete current.el.dataset.outcome; current.reason.textContent = result.row.refusal || 'Closed'; } else add(result.row, false); }
        if (result.status === 'deleted') (entry.data.sessionIds || [result.sessionId]).forEach(function (id) { window.dinahComposer.forget('chat:' + id); });
        entry.done = true; selected.delete(result.sessionId);
        entry.reason.dataset.status = result.providerCopyRemains ? 'warning' : 'positive';
        entry.reason.textContent = result.status === 'removed' ? 'Moved to the Recycle Bin.' : result.status === 'restored' ? 'Restored as a closed chat. Refresh to see it in Chats.' : 'Permanently deleted.';
        if (result.providerCopyRemains) entry.reason.textContent += ' ' + result.providerDeletionReason;
        if (result.status === 'removed' && result.row) add(result.row, true);
      } else { if (result.row) entry.data = result.row; entry.reason.dataset.status = 'warning'; entry.reason.textContent = result.error || 'The chat could not be changed.'; }
    }
    function run(call, isBin) {
      busy = true; controls(); status.textContent = 'Updating the chats.';
      return Promise.resolve().then(call).then(function (results) { if (disposed) return; (Array.isArray(results) ? results : [results]).forEach(function (result) { apply(result, isBin); }); status.textContent = 'The results are shown beside each chat.'; })
        .catch(function (error) { status.textContent = window.dinahHelpers.message(error, 'sentence') + ' Refresh before trying again.'; })
        .finally(function () { busy = false; if (!disposed) controls(); });
    }
    function confirm(title, text, action) {
      if (!text) { status.textContent = 'The confirmation could not be read. Refresh before trying again.'; return; }
      var dialog = node('dialog', 'dialog cleanup-confirm'), heading = node('h2', 'dialog-title', title), body = node('p', '', text), actions = node('div', 'dialog-actions');
      dialog.setAttribute('aria-label', title); var opener = document.activeElement;
      var accept = button('Delete', function () { dialog.close(); return action(); }); accept.classList.add('btn-danger');
      actions.append(button('Cancel', function () { dialog.close(); }), accept); dialog.append(heading, body, actions);
      dialog.addEventListener('close', function () { dialog.remove(); if (opener && opener.isConnected) opener.focus({ preventScroll: true }); }); document.body.append(dialog); dialog.showModal();
    }
    function confirmRemoval() {
      var ids = Array.from(selected); confirm('Delete ' + ids.length + (ids.length === 1 ? ' Chat?' : ' Chats?'), 'The selected chats will move to the Recycle Bin now and can be restored for 30 days. They will receive no further messages. A chat whose agent program cannot unload it may stay loaded until that program stops or the window restarts. The agent programs\' own histories are untouched at this step.', function () { return run(function () { return app().RemoveChats(ids); }); });
    }
    document.addEventListener('dinah:chats', syncInventory); document.addEventListener('dinah:execution-state', syncInventory);
    var stops = ['chat:removed', 'chat:restored'].map(function (event) { return window.dinahHost.on(event, refreshAuthority); });
    read(opts.state); load();
    return { state: address, setState: function (state) { read(state); selected.clear(); applyFilters(); }, destroy: function () { disposed = true; stops.forEach(function (stop) { stop(); }); document.removeEventListener('dinah:chats', syncInventory); document.removeEventListener('dinah:execution-state', syncInventory); } };
  }
  function open() { return window.dinahWorkbench.open('chat-cleanup'); }
  window.dinahWorkbench.register('chat-cleanup', { singleton: true, title: 'Clean Up Chats', glyph: 'comments', mode: 'maximized', init: init });
  window.dinahChatCleanup = { open: open };
  window.dinahCommands.register({ id: 'clean-up-chats', menu: 'view', label: 'Clean Up Chats', run: open });
}());
