/* The Chats browser draws the session store and owns chat names, removal
   and list grouping. Chat windows are supplied through explicit actions. */
(function () {
  'use strict';

  function create(adapter) {
    const { app, fail, toast, productName, sessionProvider, providerName, notices, sessions, S, show, showChat, openSession, workspaceLabel, chatWorking, needsCount, waitingApproval, backgroundWork, earlierSubagents, browserSection } = adapter;
    const $ = id => document.getElementById(id);

    /* A session's name: the one the person gave it, else Devin's title. */
    var titles = {};
    function title(s) { return (s && (titles[s.id] || s.info.title)) || (s && s.info.open ? 'New Chat' : 'Untitled Chat'); }

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

    /* Removing a session asks Devin to delete it from its session history,
       which every program that lists Devin's sessions reads, Devin Desktop
       too. If Devin will not, or cannot, the session can be hidden from this
       app's list instead, and Devin keeps it. */
    var removing = null;
    function openRemove(id) {
      id = id || adapter.active();
      if (!id) return;
      removing = id;
      var s = sessions[id], who = providerName(sessionProvider(s));
      if (window.dinahProviderFact(sessionProvider(s), 'removal') === 'hides') {
        $('remove-title').textContent = 'Hide Chat?';
        $('remove-body').textContent = '"' + title(s) + '" will be hidden from ' + productName() + '. ' + who + ' keeps its saved conversation. Chats > Actions > Show Hidden Chats Again restores it to this list.';
        $('remove-go').hidden = true;
        $('remove-hide').hidden = false;
        $('remove').showModal();
        return;
      }
      $('remove-title').textContent = 'Remove Chat?';
      $('remove-body').textContent = '"' + title(s) + '" will disappear from ' + productName() + ' and from ' + who + '\'s session history. This cannot be undone.' + (s && s.working ? ' ' + who + ' is working in it now; removing it stops that work.' : '');
      $('remove-go').hidden = false;
      $('remove-hide').hidden = true;
      $('remove').showModal();
    }
    function removed(id) {
      var s = sessions[id];
      adapter.disposeChat(s);
      if (s) s.log.remove();
      delete sessions[id]; window.dinahStateOrder.forget(id);
      notices.list(function (e) { return e.target && e.target.chat === id; }).forEach(function (e) { notices.remove(e.id); });
      listed = listed.filter(function (x) { return x.sessionId !== id; });
      if (adapter.active() === id) { show(null); showChat(); } else drawSessions();
      refreshSessions();
    }
    $('remove-go').addEventListener('click', function () {
      var id = removing;
      $('remove-go').disabled = true;
      var provider = providerName(sessionProvider(sessions[id]));
      app().DeleteSession(id).then(function () {
        $('remove').close();
        // A removed chat's unsent draft goes with it; a hidden chat, which
        // can come back, keeps its draft.
        var s = sessions[id];
        if (s && s.view) s.view.forgetDraft(); else window.dinahComposer.forget('chat:' + id);
        removed(id);
        toast('The session is removed.', 'positive');
      }).catch(function (err) {
        var msg = String(err).replace(/^Error:\s*/, '');
        $('remove-title').textContent = provider + ' Did Not Remove It';
        $('remove-body').textContent = /does not support/.test(msg)
          ? provider + ' does not remove sessions. You can hide this one from ' + productName() + '\'s list instead; ' + provider + ' keeps it.'
          : provider + ' answered: ' + msg + ' You can hide the chat from ' + productName() + '\'s list instead; ' + provider + ' keeps it, and Chats > Actions > Show Hidden Chats Again brings it back.';
        $('remove-go').hidden = true;
        $('remove-hide').hidden = false;
      }).then(function () { $('remove-go').disabled = false; });
    });
    $('remove-hide').addEventListener('click', function () {
      var id = removing;
      app().HideSession(id).then(function () {
        hiddenCount++;
        $('remove').close();
        removed(id);
        toast('The chat is hidden. Chats > Actions > Show Hidden Chats Again brings it back.', 'positive');
      }).catch(fail);
    });
    var hiddenCount = 0;

    var listed = [];        // the workspace's sessions, from Sessions()
    var sessionListGeneration = 0;
    function refreshSessions() {
      if (!adapter.workspace()) { listed = []; drawSessions(); return Promise.resolve(); }
      var generation = ++sessionListGeneration, selected = adapter.workspace();
      return Promise.all(adapter.openPaths().map(function (ws) {
        return (ws === selected ? app().Sessions() : app().SessionsAt(ws)).then(function (l) { return l || []; });
      })).then(function (lists) {
        if (generation !== sessionListGeneration) return;
        listed = lists.flat();
        listed.forEach(function (x) { if (window.dinahStateOrder.stale('agent:session', x.sessionId, x)) return; var s = S(x.sessionId); s.info = Object.assign({}, s.info, x); if (Object.prototype.hasOwnProperty.call(x, 'title')) s.titleKnown = true; if (s.view) s.view.refresh(); });
        drawSessions();
      }).catch(fail);
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
      return Array.from(new Set(ids)).filter(function (id) { return sessions[id] && !sessions[id].info.continuedBy; }).map(chatRef);
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
    function arrange(parent, wanted) {
      wanted.forEach(function (el, i) { if (parent.children[i] !== el) parent.insertBefore(el, parent.children[i] || null); });
      Array.from(parent.children).forEach(function (el) { if (wanted.indexOf(el) < 0) el.remove(); });
    }
    function groupCount(section, n) {
      var count = section.querySelector(':scope > .browser-group-head > .browser-group-count');
      if (!count) {
        window.dinahBrowserList.setCount(section, n);
        count = section.querySelector(':scope > .browser-group-head > .browser-group-count');
      }
      count.textContent = n > 99 ? '99+' : String(n);
      count.setAttribute('aria-label', n + (n === 1 ? ' item' : ' items'));
      count.title = count.getAttribute('aria-label');
    }
    function drawSessionList(host, scope, query, empty, prefix) {
      if (host.querySelector('input.session-rename')) return;
      var view = listView(host), signature = [scope, query, adapter.workspace()].join('|');
      var hold = view.signature === signature && host.childElementCount > 0 && listBusy(host);
      view.signature = signature;
      var rows = new Map();
      host.querySelectorAll('.md-row[data-session-row]').forEach(function (row) { rows.set(row.dataset.sessionRow, row); });
      var ids = listed.map(function (x) { return x.sessionId; });
      // Sessions opened in this run that Devin's list does not have yet.
      Object.keys(sessions).forEach(function (k) {
        var s = sessions[k];
        if (ids.indexOf(k) < 0 && s.info.open && adapter.openPaths().some(function (ws) { return normalizedWorkspace(ws) === normalizedWorkspace(s.info.cwd); })) ids.unshift(k);
      });
      ids = Array.from(new Set(ids));
      ids = ids.filter(function (id) { var s = sessions[id]; return !s.info.continuedBy && (scope === 'all' || sameWs(s.info.cwd)) && title(s).toLowerCase().includes(query); });
      var groups = {};
      ids.forEach(function (id) {
        var s = sessions[id];
        var row = rows.get(id);
        if (!row) {
          row = document.createElement('div');
          row.className = 'md-row browser-row';
          row.dataset.sessionRow = id;
          row.setAttribute('role', 'listitem');
          row.innerHTML = '<a class="md-item" href="#"><span class="browser-row-line"><span class="browser-row-title session-title"></span><span class="session-attention-slot"></span></span><span class="session-reason"></span><span class="md-meta session-row-meta"></span></a>';
          // The Workspace pane's rows have a context menu to pin the chat.
          if (prefix === 'workspace-' && window.dinahPins) window.dinahPins.rowActions(row, function () { return chatPin(id); });
        }
        var a = row.firstChild;
        a.dataset.session = id;
        a.querySelector('.session-title').textContent = title(s);
        // The second line says which agent, when and, across workspaces, where;
        // the badge says what the chat is doing.
        var meta = [providerName(sessionProvider(s))];
        if (s.info.updatedAt) meta.push(when(s.info.updatedAt));
        if (scope === 'all') meta.push(workspaceLabel(s.info.cwd));
        if (window.dinahChatState.read(s.info).life === 'open' && window.dinahChatState.read(s.info).window === false) meta.push('No window');
        a.querySelector('.md-meta').textContent = meta.join(' · ');
        a.querySelector('.md-meta').title = meta.join(' · ');
        var reason = window.dinahChatState.read(s.info).exec;
        var why = reason && reason.why || '';
        a.querySelector('.session-reason').textContent = why;
        a.querySelector('.session-reason').title = why;
        a.querySelectorAll('.session-badge, .team-stuck-badge').forEach(function (badge) { badge.remove(); });
        var line = a.querySelector('.browser-row-line');
        line.classList.add('chat-state-line');
        line.appendChild(window.dinahChatState.badge(s.info, { compact: true, className: 'session-badge' }));
        var attention = line.querySelector('.session-attention-slot'), n = needsCount(id);
        attention.replaceChildren();
        var reservedCount = window.dinahBrowserList.badge({ text: '99+', kind: 'warn', cls: 'session-attention-reserve' });
        reservedCount.setAttribute('aria-hidden', 'true'); attention.appendChild(reservedCount);
        if (n) attention.appendChild(window.dinahBrowserList.badge({ text: n > 99 ? '99+' : n, kind: 'warn', cls: 'session-attention', label: n === 1 ? '1 question waits for you' : n + ' questions wait for you' }));
        attention.setAttribute('aria-hidden', String(!n));
        var aggregateBadge = document.createElement('span'); aggregateBadge.className = 'team-state-slot';
        if (window.dinahTeams && window.dinahTeams.aggregate) {
          var aggregate = s.info.team || window.dinahTeams.aggregate(id);
          if (aggregate) aggregateBadge.appendChild(window.dinahChatState.badge(aggregate, { compact: true, className: 'team-stuck-badge' }));
        }
        var oldAggregate = line.querySelector('.team-state-slot');
        if (oldAggregate) oldAggregate.replaceWith(aggregateBadge); else line.appendChild(aggregateBadge);
        line.insertBefore(line.querySelector('.session-badge'), aggregateBadge);
        a.title = [title(s), window.dinahChatState.describe(s.info).label, meta.join(' · ')].filter(Boolean).join('. ');
        a.setAttribute('aria-label', a.title);
        if (id === adapter.active()) a.setAttribute('aria-current', 'true'); else a.removeAttribute('aria-current');
        var state = window.dinahChatState.read(s.info), execution = state.exec && state.exec.state;
        var groupName = execution === 'waiting' || execution === 'stuck' ? 'Needs You' : execution === 'working' ? 'Working' : 'Chats';
        if (!groups[groupName]) groups[groupName] = [];
        groups[groupName].push(row);
        // A chat with subagents from earlier runs has a Subagents part under
        // its entry, which opens them in the Earlier Subagents window. It is
        // absent until the chat's agent has sent them, which it does when the
        // chat is opened (subagents.go).
        var earlier = earlierSubagents(s).length;
        if (earlier && window.dinahSubagents) {
          var part = row._subagents || window.dinahBrowserList.row({ title: 'Subagents', meta: [earlier === 1 ? '1 from an earlier run' : earlier + ' from earlier runs'],
            tooltip: 'Open the subagents of "' + title(s) + '" from earlier runs', onOpen: function () { window.dinahSubagents.openArchive(id, s.info.cwd || adapter.workspace()).catch(fail); },
            data: { earlierSubagents: id } });
          part.classList.add('browser-row-part', 'chat-subagents-part');
          row._subagents = part;
          part.querySelector('.md-meta').textContent = earlier === 1 ? '1 from an earlier run' : earlier + ' from earlier runs';
          groups[groupName].push(part);
        }
      });
      var sections = [], flat = [], active = document.activeElement;
      var scrolls = [], ancestor = host;
      while (ancestor) { scrolls.push([ancestor, ancestor.scrollTop, ancestor.scrollLeft]); ancestor = ancestor.parentElement; }
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
        section.classList.add('session-browser-group');
        groupCount(section, wanted.filter(function (row) { return !row.classList.contains('browser-row-part'); }).length);
        arrange(section.querySelector('.browser-group-body'), wanted); sections.push(section);
      });
      view.pending = hold;
      if (!hold) {
        arrange(host, prefix === 'workspace-' ? flat : sections);
        if (host.contains(active) && document.activeElement !== active) active.focus({ preventScroll: true });
        scrolls.forEach(function (saved) { saved[0].scrollTop = saved[1]; saved[0].scrollLeft = saved[2]; });
      }
      if (prefix === 'workspace-') {
        var workspaceGroup = host.closest('.browser-group');
        workspaceGroup.classList.add('session-browser-group');
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

    return { title, hasKnownTitle: id => !!titles[id] || !!(sessions[id] && sessions[id].info.title), startRename, openRemove, refreshSessions, when, drawSessions, chatPin, sameWs, normalizedWorkspace,
      setTitles: value => { titles = value || {}; Object.keys(sessions).forEach(id => { if (sessions[id].view) sessions[id].view.refresh(); }); if (window.dinahWorkbench && window.dinahWorkbench.refreshTitles) window.dinahWorkbench.refreshTitles(); },
      setHiddenCount: value => { hiddenCount = value || 0; },
      hiddenCount: () => hiddenCount, listed: () => listed
    };
  }

  window.dinahChatBrowser = { create };
})();
