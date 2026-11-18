/* The Chats browser draws the session store and owns chat names, removal
   and list grouping. Chat windows are supplied through explicit actions. */
(function () {
  'use strict';

  function create(adapter) {
    const { app, fail, toast, productName, sessionProvider, providerName, notices, sessions, S, show, openSession, workspaceLabel, needsCount, earlierSubagents, browserSection } = adapter;
    const $ = id => document.getElementById(id);

    /* A session's name: the one the person gave it, else Devin's title. */
    var titles = {};
    function title(s) { return (s && (titles[s.id] || s.info.title)) || (s && s.info.open ? 'New Chat' : 'Untitled Chat'); }
    window.dinahHost.on('agent:titles', function (value) { titles = value || {}; drawSessions(); });

    /* Renaming: the session's row in the list becomes a field. Enter keeps
       the name, Esc leaves it as it was, and an empty name goes back to
       Devin's title. */
    // Renaming from Actions opens a form; sidebar rows retain inline editing.
    function startRename(id, where) {
      id = id || adapter.active();
      if (!id) return;
      if (where !== 'list' && where !== 'workspace') {
        var dialog = document.createElement('dialog'); dialog.className = 'dialog chat-rename-dialog';
        dialog.setAttribute('aria-label', 'Rename Chat');
        dialog.innerHTML = '<h2 class="dialog-title">Rename Chat</h2><form class="vstack gap-3"><label class="form-label">Name of the chat<input class="form-input session-rename" name="title" autocomplete="off"></label><div class="dialog-actions"><button type="button" class="btn" data-rename-cancel>Cancel</button><button type="submit" class="btn btn-primary">Save</button></div></form>';
        var field = dialog.querySelector('input'); field.value = title(sessions[id]);
        var opener = document.activeElement;
        dialog.addEventListener('close', function () { dialog.remove(); if (opener && opener.isConnected) opener.focus({ preventScroll: true }); });
        dialog.querySelector('[data-rename-cancel]').addEventListener('click', function () { dialog.close(); });
        dialog.querySelector('form').addEventListener('submit', function (event) {
          event.preventDefault();
          app().RenameSession(id, field.value).then(function (t) {
            titles = t || {}; drawSessions();
            if (sessions[id] && sessions[id].view) sessions[id].view.refresh();
            dialog.close();
          }).catch(fail);
        });
        document.body.appendChild(dialog); dialog.showModal(); field.focus(); field.select(); return;
      }
      var target = document.querySelector((where === 'workspace' ? '#workspace-chats' : '#sessions') + ' a[data-session="' + CSS.escape(id) + '"]');
      if (!target) return;
      var parent = target.parentNode;
      var input = document.createElement('input');
      input.type = 'text';
      input.className = 'form-input session-rename';
      input.value = title(sessions[id]);
      input.setAttribute('aria-label', 'Name of the session');
      parent.replaceChild(input, target);
      input.focus();
      input.select();
      var done = false;
      function restore() { if (input.parentNode) input.parentNode.replaceChild(target, input); }
      function finish(save) {
        if (done) return;
        done = true;
        restore();
        if (!save) { drawSessions(); return; }
        app().RenameSession(id, input.value).then(function (t) {
          titles = t || {};
          drawSessions();
          if (sessions[id] && sessions[id].view) sessions[id].view.refresh();
        }).catch(fail);
      }
      input.addEventListener('keydown', function (e) {
        if (e.key === 'Enter') { e.preventDefault(); finish(true); }
        else if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); finish(false); }
      });
      input.addEventListener('blur', function () { finish(true); });
    }

    /* Single and batch deletion use the same retained local removal. */
    var removing = null;
    function openRemove(id, local) {
      id = id || adapter.active();
      if (!id) return;
      removing = id;
      var s = sessions[id], name = s ? title(s) : local && local.title || 'This chat';
      $('remove-title').textContent = 'Delete Chat?';
      $('remove-body').textContent = '"' + name + '" will move to the Recycle Bin in ' + productName() + '. You can restore it for 30 days. A chat whose agent program cannot unload it may stay loaded until that program stops or the window restarts. Its agent program\'s history is untouched at this step. Open Clean Up Chats to review the Recycle Bin.';
      $('remove-go').textContent = 'Delete';
      $('remove-go').hidden = false;
      $('remove').showModal();
    }
    /* A removed or hidden chat's rows leave every list at once. A list holds
       still while the person is in it (listBusy), and the person usually is
       here, on the row whose menu removed or hid the chat. That row is the
       one change the person asked for, so it goes without waiting, and the
       rest of the list stays held as before. Focus moves to the next chat's
       row in the same list, or the one before it; with neither, it leaves
       the list, which is then free to redraw. */
    function dropRows(id) {
      document.querySelectorAll('.md-row[data-session-row="' + CSS.escape(id) + '"]').forEach(function (row) {
        if (row.contains(document.activeElement)) {
          var all = Array.from((row.closest('.session-list') || row.parentElement).querySelectorAll('.md-row[data-session-row]')), at = all.indexOf(row);
          var others = all.filter(function (r) { return r.dataset.sessionRow !== id; });
          var next = others.find(function (r) { return all.indexOf(r) > at; }) || others.reverse().find(function (r) { return all.indexOf(r) < at; });
          var target = next && next.querySelector('a.md-item, [tabindex="0"], button');
          if (target) target.focus({ preventScroll: true }); else document.activeElement.blur();
        }
        row.remove();
      });
    }
    function removed(id) {
      ++sessionListGeneration;
      dropRows(id);
      var s = sessions[id];
      adapter.disposeChat(s);
      if (s) s.log.remove();
      delete sessions[id]; window.dinahStateOrder.forget(id);
      // Its cards go with it, those about it and the recovery cards for it as their original chat.
      notices.list(function (e) { return (e.target && e.target.chat === id) || (e.data && e.data.lead === id); }).forEach(function (e) { notices.remove(e.id); });
      listed = listed.filter(function (x) { return x.sessionId !== id; });
      // The removed chat's window closed with it (disposeChat). Nothing opens
      // in its place: showing no chat in a window is New Chat, which the
      // person did not ask for.
      if (adapter.active() === id) show(null); else drawSessions();
    }
    $('remove-go').addEventListener('click', function () {
      var id = removing;
      $('remove-go').disabled = true;
      Promise.resolve().then(function () { return app().RemoveChat(id); }).then(function () {
        $('remove').close();
        removed(id);
      }).catch(function (error) {
        $('remove-title').textContent = 'The Chat Was Not Deleted';
        $('remove-body').textContent = window.dinahHelpers.message(error, 'sentence');
      }).finally(function () { $('remove-go').disabled = false; });
    });
    window.dinahHost.on('chat:removed', function (event) { if (event && event.sessionId) removed(event.sessionId); });
    window.dinahHost.on('chat:restored', function (event) {
      if (!event || !event.sessionId) return;
      Promise.resolve(app().ListChatCleanup()).then(function (view) {
        var chat = (view.rows || []).find(function (row) { return row.sessionId === event.sessionId || (row.sessionIds || []).includes(event.sessionId); });
        if (chat) window.dinahChats.restore(chat);
      }).catch(fail);
    });

    var hiddenCount = 0;
    var listed = [];
    var sessionListGeneration = 0;
    var inventoryRefresh = null, inventoryRefreshKey = '', inventoryRefreshGeneration = 0, inventoryErrorToast = null;
    function clearInventoryError() {
      if (inventoryErrorToast && inventoryErrorToast.isConnected) { var close = inventoryErrorToast.querySelector('.toast-close'); if (close) close.click(); }
      inventoryErrorToast = null;
    }
    function refreshSessions() {
      var key = JSON.stringify([adapter.workspace(), adapter.openPaths()]);
      if (inventoryRefresh && inventoryRefreshKey === key && inventoryRefreshGeneration === sessionListGeneration) return inventoryRefresh;
      inventoryRefreshKey = key;
      var request = refreshSessionInventory();
      inventoryRefreshGeneration = sessionListGeneration;
      inventoryRefresh = request;
      request.finally(function () { if (inventoryRefresh === request) inventoryRefresh = null; });
      return request;
    }
    function refreshNativeSessions() {
      return app().DeclaredProviders().then(function (providers) {
        return Promise.allSettled(providers.map(function (provider) { return app().RefreshProviderSessions(adapter.workspace(), provider.id); }));
      }).then(function (results) {
        return refreshSessions().then(function () { results.forEach(function (result) { if (result.status === 'rejected') fail(result.reason); }); });
      }).catch(fail);
    }
    function refreshSessionInventory() {
      var generation = ++sessionListGeneration;
      return app().SavedChats().then(function (inventory) {
        if (generation !== sessionListGeneration) return;
        listed = (inventory.chats || []).map(function (row) {
          return row.resolution === 'repair' && !row.sessionId ? Object.assign({}, row, { sessionId: row.chatId }) : row;
        });
        listed.forEach(function (x) { if (window.dinahStateOrder.stale('agent:session', x.sessionId, x)) return; var s = S(x.sessionId); s.info = Object.assign({}, s.info, x); if (!x.model) delete s.info.model; if (Object.prototype.hasOwnProperty.call(x, 'title')) s.titleKnown = true; if (s.view) s.view.refresh(); });
        clearInventoryError();
        drawSessions();
      }).catch(function (error) { if (generation !== sessionListGeneration) return; clearInventoryError(); inventoryErrorToast = toast(String(error), 'danger'); });
    }
    $('chat-filter').addEventListener('input', drawSessions);
    $('chat-scope').addEventListener('change', drawSessions);
    // The lists say when each chat was last used in the regional format,
    // which may arrive after they were first drawn (regional.js).
    document.addEventListener('dinah:locale', drawSessions);
    document.addEventListener('dinah:team-state', drawSessions);

    function when(iso) {
      if (!iso) return '';
      var d = new Date(iso), now = new Date();
      if (isNaN(d)) return '';
      if (d.toDateString() === now.toDateString()) return 'Today ' + dinahRegional.format(d, 'clock');
      return dinahRegional.format(d, 'dayTime');
    }

    /* Every chat the window knows, open or closed, as the Group Chats pane
       and the picker of a group chat list them (bus-applets.js). They are
       the chat list's own: the chats of the open workspaces and those open
       in this run. A chat with no title of its own is untitled, and the
       picker asks for one before it can join a group. */
    function knownChats() {
      var ids = listed.map(function (x) { return x.sessionId; });
      Object.keys(sessions).forEach(function (k) { if (ids.indexOf(k) < 0 && sessions[k].info.open) ids.push(k); });
      return Array.from(new Set(ids)).filter(function (id) { return sessions[id] && sessions[id].info.resolution !== 'repair' && !sessions[id].info.continuedBy; }).map(chatRef);
    }
    function chatRef(id) {
      var s = sessions[id], state = window.dinahChatState.read(s.info), named = (titles[id] || s.info.title || '').trim(), cwd = s.info.cwd || '';
      var where = cwd ? workspaceLabel(cwd) : '';
      return { id: id, title: named || 'Untitled Chat in ' + where, untitled: !named, cwd: cwd, workspace: where,
        provider: providerName(sessionProvider(s)), exec: state.exec, life: state.life, window: state.window, elsewhere: state.elsewhere, updatedAt: s.info.updatedAt || '' };
    }
    var chatsChanged = 0;
    function announceChats() {
      if (chatsChanged) return;
      chatsChanged = requestAnimationFrame(function () { chatsChanged = 0; document.dispatchEvent(new CustomEvent('dinah:chats')); });
    }
    window.dinahChats = {
      known: knownChats,
      refreshInventory: refreshSessions,
      inventory: function () {
        return visibleChatIds('all', '').map(function (id) {
          var session = sessions[id];
          return Object.assign({}, session.info, { sessionId: id, title: title(session), state: window.dinahChatState.read(session.info), workspaceName: session.info.cwd ? workspaceLabel(session.info.cwd) : '', providerName: providerName(sessionProvider(session)), model: session.info.model || '' });
        });
      },
      created: function (info) {
        ++sessionListGeneration;
        var session = S(info.sessionId); session.info = Object.assign({}, session.info, info); session.titleKnown = true;
        var at = listed.findIndex(function (entry) { return entry.sessionId === info.sessionId; });
        if (at < 0) listed.push(info); else listed[at] = info;
        drawSessions(); return openSession(info.sessionId);
      },
      restore: function (row) {
        ++sessionListGeneration;
        var info = { sessionId: row.sessionId, title: row.title, cwd: row.workspace, provider: row.provider, updatedAt: row.lastActivity, open: false, working: false, chatState: row.chatState || { life: 'closed' } };
        var session = S(row.sessionId); session.info = Object.assign({}, session.info, info); session.working = false; session.titleKnown = true;
        var at = listed.findIndex(function (entry) { return entry.sessionId === row.sessionId; });
        if (at < 0) listed.push(info); else listed[at] = info;
        drawSessions();
      },
      ref: function (id) { return sessions[id] ? chatRef(id) : null; },
      open: function (id) { return openSession(id); },
      // Giving a chat its title, as renaming it in the list does.
      rename: function (id, name) {
        return Promise.resolve(app().RenameSession(id, name)).then(function (t) {
          titles = t || titles;
          drawSessions();
          if (sessions[id] && sessions[id].view) sessions[id].view.refresh();
          return chatRef(id);
        });
      }
    };

    function drawSessions() {
      announceChats();
      var chats = $('sessions'), workspaceChats = $('workspace-chats');
      window.dinahRender.enqueue(chats, chats, function () {
        drawSessionList(chats, $('chat-scope').value, $('chat-filter').value.trim().toLowerCase(), $('sessions-empty'), '');
      });
      window.dinahRender.enqueue(workspaceChats, workspaceChats, function () {
        drawSessionList(workspaceChats, 'current', '', $('workspace-chats-empty'), 'workspace-');
      });
    }
    var listViews = new WeakMap();
    function listBusy(host) {
      var view = listViews.get(host);
      return !!(view && (view.pressed.size || (host.matches(':hover') && performance.now() < view.pointerUntil))) || host.contains(document.activeElement) || !!host.querySelector('[data-menu-for]');
    }
    function listView(host) {
      var view = listViews.get(host);
      if (view) return view;
      view = { pending: false, signature: '', pressed: new Set(), pointerUntil: 0, pointerTimer: 0 }; listViews.set(host, view);
      function flush() { if (view.pending && !listBusy(host)) drawSessions(); }
      function clearPointer() {
        clearTimeout(view.pointerTimer); view.pointerTimer = 0; view.pointerUntil = 0;
      }
      function expirePointer() {
        view.pointerTimer = 0;
        var remaining = view.pointerUntil - performance.now();
        if (remaining > 0) { view.pointerTimer = setTimeout(expirePointer, Math.ceil(remaining)); return; }
        view.pointerUntil = 0; flush();
      }
      function renewPointer() {
        clearPointer();
        if (!view.pressed.size) {
          view.pointerUntil = performance.now() + 2000;
          view.pointerTimer = setTimeout(expirePointer, 2000);
        }
      }
      function trackPointer(event) {
        if (event.buttons) view.pressed.add(event.pointerId); else view.pressed.delete(event.pointerId);
        renewPointer();
      }
      host.addEventListener('pointerenter', trackPointer);
      host.addEventListener('pointermove', trackPointer);
      host.addEventListener('pointerdown', function (event) { view.pressed.add(event.pointerId); clearPointer(); });
      host.addEventListener('pointerleave', function () { clearPointer(); flush(); });
      function releasedPointer(event) {
        if (!view.pressed.delete(event.pointerId)) return;
        if (host.matches(':hover')) renewPointer(); else clearPointer();
        flush();
      }
      document.addEventListener('pointerup', releasedPointer, true);
      document.addEventListener('pointercancel', releasedPointer, true);
      host.addEventListener('focusout', function () { queueMicrotask(flush); });
      // PUDL marks an open context-menu anchor with data-menu-for.
      new MutationObserver(flush).observe(host, { subtree: true, attributes: true, attributeFilter: ['data-menu-for'] });
      if (host.matches(':hover')) renewPointer();
      return view;
    }
    var arrange = window.dinahHelpers.arrange;
    function groupCount(section, n) {
      var count = section.querySelector(':scope > .browser-group-head > .browser-group-count');
      if (!count) {
        window.dinahBrowserList.setCount(section, n);
        count = section.querySelector(':scope > .browser-group-head > .browser-group-count');
      }
      var words = n > 99 ? '99+' : String(n), label = n + (n === 1 ? ' item' : ' items');
      if (count.textContent !== words) count.textContent = words;
      if (count.getAttribute('aria-label') !== label) count.setAttribute('aria-label', label);
      if (count.title !== label) count.title = label;
    }
    function visibleChatIds(scope, query) {
      var ids = listed.map(function (x) { return x.sessionId; });
      // Sessions opened in this run that Devin's list does not have yet.
      Object.keys(sessions).forEach(function (k) {
        var s = sessions[k];
        if (ids.indexOf(k) < 0 && s.info.open && adapter.openPaths().some(function (ws) { return normalizedWorkspace(ws) === normalizedWorkspace(s.info.cwd); })) ids.unshift(k);
      });
      ids = Array.from(new Set(ids));
      ids = ids.filter(function (id) { var s = sessions[id]; return !s.info.continuedBy && (scope === 'all' || sameWs(s.info.cwd)) && title(s).toLowerCase().includes(query); });
      return ids;
    }
    function drawSessionList(host, scope, query, empty, prefix) {
      if (host.querySelector('input.session-rename')) return;
      var view = listView(host), signature = [scope, query, adapter.workspace()].join('|');
      var hold = view.signature === signature && host.childElementCount > 0 && listBusy(host);
      view.signature = signature;
      var rows = new Map();
      host.querySelectorAll('.md-row[data-session-row]').forEach(function (row) { rows.set(row.dataset.sessionRow, row); });
      var ids = visibleChatIds(scope, query);
      // Sort the combined workspace list by activity, retaining source order for ties.
      var activity = new Map(ids.map(function (id) {
        var time = Date.parse(sessions[id].info.updatedAt);
        return [id, Number.isFinite(time) ? time : -Infinity];
      }));
      ids.sort(function (a, b) {
        var left = activity.get(a), right = activity.get(b);
        return left === right ? 0 : left > right ? -1 : 1;
      });
      var groups = {};
      ids.forEach(function (id) {
        var s = sessions[id];
        var row = rows.get(id);
        var state = window.dinahChatState.read(s.info), execution = state.exec && state.exec.state;
        var groupName = execution === 'waiting' || execution === 'stuck' ? 'Needs You' : execution === 'working' || execution === 'background' ? 'Working' : 'Chats';
        var earlier = earlierSubagents(s).length;
        // Cache displayed values, including locale-dependent text and live
        // chat state. Unchanged rows retain their badges, focus and DOM nodes.
        var rowSignature = JSON.stringify([title(s), providerName(sessionProvider(s)), s.info.model || '', when(s.info.updatedAt),
          scope === 'all' ? workspaceLabel(s.info.cwd) : '', state, window.dinahChatState.describe(s.info),
          needsCount(id), earlier, id === adapter.active(), s.info.resolution, s.info.reason]);
        if (!groups[groupName]) groups[groupName] = [];
        if (row && row._chatRowSignature === rowSignature) {
          groups[groupName].push(row);
          if (earlier && row._subagents) groups[groupName].push(row._subagents);
          return;
        }
        if (!row) {
          row = document.createElement('div');
          row.className = 'md-row browser-row';
          row.dataset.sessionRow = id;
          row.setAttribute('role', 'listitem');
          row.innerHTML = '<a class="md-item" href="#"><span class="browser-row-line"><span class="browser-row-title session-title"></span><span class="session-attention-slot"></span></span><span class="session-reason"></span><span class="md-meta session-row-meta"></span></a>';
          // A row's context menu pins the chat to the open workspace, when
          // the chat is that workspace's, and removes the chat, so that a
          // chat can be removed without opening it first.
          if (window.dinahPins) window.dinahPins.rowActions(row, function () {
            return sessions[id] && sessions[id].info.resolution !== 'repair' && sameWs(sessions[id].info.cwd) ? chatPin(id) : null;
          }, function () {
            if (!sessions[id] || sessions[id].info.resolution === 'repair') return [];
            return [{ label: 'Delete Chat…', danger: true, run: function () { openRemove(id); } }];
          });
        }
        var a = row.firstChild;
        a.dataset.session = id;
        if (s.info.resolution === 'repair') {
          delete a.dataset.session; a.removeAttribute('href'); a.setAttribute('aria-disabled', 'true');
        } else { a.href = '#'; a.removeAttribute('aria-disabled'); }
        a.querySelector('.session-title').textContent = title(s);
        // The second line says which agent and model, when and, across workspaces, where;
        // the badge says what the chat is doing. The model is the name the host reports
        // (Info.Model), absent when the agent offers no choice or gives it no name.
        var meta = s.info.resolution === 'repair' && !s.info.provider ? [] : [providerName(sessionProvider(s))];
        if (meta.length && s.info.model) meta.push(s.info.model);
        if (s.info.updatedAt) meta.push(when(s.info.updatedAt));
        if (scope === 'all' && s.info.cwd) meta.push(workspaceLabel(s.info.cwd));
        if (window.dinahChatState.read(s.info).life === 'open' && window.dinahChatState.read(s.info).window === false) meta.push('No window');
        a.querySelector('.md-meta').textContent = meta.join(' · ');
        a.querySelector('.md-meta').title = meta.join(' · ');
        var reason = window.dinahChatState.read(s.info).exec;
        var why = s.info.resolution === 'repair' ? s.info.reason || 'This chat needs repair before it can be opened.' : reason && reason.why || '';
        a.querySelector('.session-reason').textContent = why;
        a.querySelector('.session-reason').title = why;
        a.querySelectorAll('.session-badge').forEach(function (badge) { badge.remove(); });
        var line = a.querySelector('.browser-row-line');
        line.classList.add('chat-state-line');
        line.appendChild(window.dinahChatState.badge(s.info, { labelled: true, className: 'session-badge' }));
        var attention = line.querySelector('.session-attention-slot'), n = needsCount(id);
        attention.replaceChildren();
        // The slot keeps the width of the widest count, so a count arriving
        // or going never moves the title.
        var reservedCount = window.dinahBrowserList.badge({ text: '99+', kind: 'warn', cls: 'session-attention-reserve' });
        reservedCount.setAttribute('aria-hidden', 'true'); attention.appendChild(reservedCount);
        if (n) attention.appendChild(window.dinahBrowserList.badge({ text: n > 99 ? '99+' : n, kind: 'warn', cls: 'session-attention', label: n === 1 ? '1 question waits for you' : n + ' questions wait for you' }));
        attention.setAttribute('aria-hidden', String(!n));
        // A row shows its own chat's state only; a team's state is shown
        // where the team is (Paul, 1 November 2026).
        line.querySelectorAll('.team-state-slot').forEach(function (slot) { slot.remove(); });
        a.title = [title(s), window.dinahChatState.describe(s.info).label, meta.join(' · ')].filter(Boolean).join('. ');
        a.setAttribute('aria-label', a.title);
        if (id === adapter.active()) a.setAttribute('aria-current', 'true'); else a.removeAttribute('aria-current');
        groups[groupName].push(row);
        // A chat with subagents from earlier runs has a Subagents part under
        // its entry, which opens them in the Earlier Subagents window. It is
        // absent until the chat's agent has sent them, which it does when the
        // chat is opened (subagents.go).
        if (earlier && window.dinahSubagents) {
          var part = row._subagents || window.dinahBrowserList.row({ title: 'Subagents', meta: [earlier === 1 ? '1 from an earlier run' : earlier + ' from earlier runs'],
            tooltip: 'Open the subagents of "' + title(s) + '" from earlier runs', onOpen: function () { window.dinahSubagents.openArchive(id, s.info.cwd || adapter.workspace()).catch(fail); },
            data: { earlierSubagents: id } });
          part.classList.add('browser-row-part', 'chat-subagents-part');
          row._subagents = part;
          part.querySelector('.md-meta').textContent = earlier === 1 ? '1 from an earlier run' : earlier + ' from earlier runs';
          groups[groupName].push(part);
        }
        row._chatRowSignature = rowSignature;
      });
      var sections = [], flat = [], active = document.activeElement;
      /* The scroll positions around the list are kept across a rearrangement
         and put back after it. Reading them makes the browser lay the page
         out, so they are read only when rows are about to move: most draws
         change a row's words or nothing, and each used to lay the whole page
         out, twice for every change of a chat's state, once for each list
         (17 November 2026, docs/design/scaling.md). */
      var scrolls = null;
      function place(container, elements) {
        var same = container.childElementCount === elements.length, child = container.firstElementChild;
        for (var i = 0; same && i < elements.length; i++, child = child.nextElementSibling) same = child === elements[i];
        if (same) return;
        if (!scrolls) { scrolls = []; for (var ancestor = host; ancestor; ancestor = ancestor.parentElement) scrolls.push([ancestor, ancestor.scrollTop, ancestor.scrollLeft]); }
        arrange(container, elements);
      }
      ['Needs You', 'Working', 'Chats'].forEach(function (name) {
        var wanted = groups[name] || [];
        var section = Array.from(host.children).find(function (el) { return el.dataset.section === prefix + 'chats-' + name; });
        if (section) groupCount(section, wanted.filter(function (row) { return !row.classList.contains('browser-row-part'); }).length);
        if (hold) return;
        if (prefix === 'workspace-') { flat.push.apply(flat, wanted); return; }
        if (!wanted.length) return;
        if (!section) {
          browserSection(host, prefix + 'chats-' + name, name, []);
          section = host.lastElementChild;
        }
        if (!section.classList.contains('session-browser-group')) section.classList.add('session-browser-group');
        groupCount(section, wanted.filter(function (row) { return !row.classList.contains('browser-row-part'); }).length);
        place(section.querySelector('.browser-group-body'), wanted); sections.push(section);
      });
      view.pending = hold;
      if (!hold) {
        place(host, prefix === 'workspace-' ? flat : sections);
        if (host.contains(active) && document.activeElement !== active) active.focus({ preventScroll: true });
        if (scrolls) scrolls.forEach(function (saved) { saved[0].scrollTop = saved[1]; saved[0].scrollLeft = saved[2]; });
      }
      if (prefix === 'workspace-') {
        var workspaceGroup = host.closest('.browser-group');
        if (!workspaceGroup.classList.contains('session-browser-group')) workspaceGroup.classList.add('session-browser-group');
        groupCount(workspaceGroup, ids.length);
      }
      empty.hidden = ids.length > 0;
      empty.querySelector('.empty-state-title').textContent = !adapter.workspace() ? 'No Workspace Chosen' : query ? 'No Matching Chats' : 'No Chats Yet';
      empty.querySelector('.empty-state-body').textContent = !adapter.workspace() ? 'Choose a workspace first.' : query ? 'No chat title contains those words.' : 'A chat with an agent in this workspace appears here.';
    }

    // A chat's pin (pins.js); the Pinned group opens it here.
    function chatPin(id) { return { kind: 'chat', title: title(sessions[id]), session: id }; }
    if (window.dinahPins) window.dinahPins.registerOpener('chat', function (pin) { return openSession(pin.session); });

    function sameWs(cwd) { return !cwd || !adapter.workspace() || cwd.toLowerCase().replace(/[\\/]+$/, '') === adapter.workspace().toLowerCase().replace(/[\\/]+$/, ''); }
    function normalizedWorkspace(path) { return (path || '').replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase(); }

    [$('sessions'), $('workspace-chats')].forEach(function (host) {
    host.addEventListener('click', function (e) {
      var a = e.target.closest('a[data-session]');
      if (!a) return;
      e.preventDefault();
      // A chat showing its saved copy comes forward as it is; choosing it is
      // not a need for its agent, and its first click is also the start of a
      // double-click to rename it.
      var s = sessions[a.dataset.session];
      if (s && s.saved && !s.info.open && !s.loading) { adapter.showSavedChat(s); return; }
      openSession(a.dataset.session);
    });
    host.addEventListener('dblclick', function (e) {
      var a = e.target.closest('a[data-session]');
      if (!a) return;
      e.preventDefault();
      startRename(a.dataset.session, host.id === 'sessions' ? 'list' : 'workspace');
    });
    });

    return { title, hasKnownTitle: id => !!titles[id] || !!(sessions[id] && sessions[id].info.title), startRename, openRemove, refreshSessions, refreshNativeSessions, when, drawSessions, chatPin, sameWs, normalizedWorkspace,
      setTitles: value => { titles = value || {}; Object.keys(sessions).forEach(id => { if (sessions[id].view) sessions[id].view.refresh(); }); if (window.dinahWorkbench && window.dinahWorkbench.refreshTitles) window.dinahWorkbench.refreshTitles(); },
      setHiddenCount: value => { hiddenCount = value || 0; },
      hiddenCount: () => hiddenCount, listed: () => listed
    };
  }

  window.dinahChatBrowser = { create };
})();
