/* Dinah's notification persistence, questions and views use the generic
   Notices model. Session state and chat actions come from the shell. */
(function () {
  'use strict';

  function create(adapter) {
    const { app, on, element: $, session: S, getSession, sessionProvider,
      workspace, title, openSession, openSavedRecovery, openRemove, row, now, drawSessions, toast, fail } = adapter;

    /* ask puts a yes-or-no question to the person in the page's own dialog,
       never the browser's, and resolves true when they confirm. Escape or
       Cancel resolves false. */
    var askDone = null;
    /* ask resolves true for its yes button and false for Cancel. Extra
       choices, [{label, value}], stand between the two, and resolve their
       value, for a question with more than two answers. */
    function ask(title, body, yes, danger, extra) {
      if (askDone) askDone(false);
      $('ask-title').textContent = title;
      $('ask-body').textContent = body || '';
      $('ask-body').hidden = !body;
      $('ask-yes').textContent = yes || 'OK';
      $('ask-yes').className = 'btn ' + (danger ? 'btn-danger' : 'btn-primary');
      $('ask').querySelectorAll('.ask-extra').forEach(function (b) { b.remove(); });
      (extra || []).forEach(function (choice) {
        var b = document.createElement('button');
        b.type = 'button'; b.className = 'btn ask-extra'; b.textContent = choice.label;
        b.addEventListener('click', function () { if (askDone) askDone(choice.value); });
        $('ask-yes').before(b);
      });
      return new Promise(function (resolve) {
        askDone = function (v) { askDone = null; if ($('ask').open) $('ask').close(); resolve(v); };
        $('ask').showModal();
        $('ask-no').focus();
      });
    }
    $('ask-yes').addEventListener('click', function () { if (askDone) askDone(true); });
    $('ask-no').addEventListener('click', function () { if (askDone) askDone(false); });
    $('ask').addEventListener('close', function () { if (askDone) askDone(false); });

    /* ---- Notifications ----------------------------------------------------
       One model holds every notification (notices.js, and
       docs/design/notifications.md). The pane and the taskbar's list draw
       from it and every count comes from it. Saving and dismissing, opening
       what an entry is about and the severity styles are Dinah's, given to
       the model here. */
    var notices = window.Notices.create({
      context: liveChatState,
      style: function (el, tone) { window.dinahNotificationStyle(el, tone); },
      open: openNotice,
      openTarget: function (entry) { return entry.target && entry.target.chat ? openSession(entry.target.chat) : null; },
      showAll: function () { return window.dinahWorkbench.open('needs-you'); },
      dismiss: function (entry) { return entry.data.notificationId ? dismissStoredNotification(entry.data.notificationId) : null; },
      clear: function (entries) { return flushNotifications().then(function () { return Promise.all(entries.filter(function (entry) { return !entry.dismiss && entry.data.notificationId; }).map(function (entry) { return dismissStoredNotification(entry.data.notificationId); })); }); },
      error: function (err) { fail(err); },
      formatTime: function (d) { return dinahRegional.format(d, 'dayTime'); }
    });
    function liveChatState(entry) {
      var id = entry.target && entry.target.chat, session = id && getSession(id);
      if (!session || !window.dinahChatState.read(session.info).life) return null;
      var badge = window.dinahChatState.badge(session.info, { className: 'notice-chat-state' });
      var named = 'Current chat state: ' + (badge.getAttribute('aria-label') || badge.textContent);
      badge.title = named; badge.setAttribute('aria-label', named);
      return badge;
    }
    // Historical notification tone stays separate from the chat's current state.
    function refreshLiveStates(event) {
      document.querySelectorAll('[data-notice-id]').forEach(function (element) {
        var entry = notices.get(element.dataset.noticeId);
        if (!entry || !entry.target || event.detail.id && entry.target.chat !== event.detail.id) return;
        var earlier = element.querySelector('.notice-chat-state'), badge = liveChatState(entry);
        if (earlier) { if (badge) earlier.replaceWith(badge); else earlier.remove(); }
        else if (badge) (element.querySelector('.notice-content') || element).appendChild(badge);
      });
    }
    document.addEventListener('dinah:session-info', refreshLiveStates);
    document.addEventListener('dinah:execution-state', refreshLiveStates);
    /* After a resync the page has read again what it can, but a notice is a
       moment that cannot be read again, so the person is told once that some
       activity may not have been shown. A second resync updates this entry. */
    window.dinahHost.onResync(function () {
      notices.add({ id: 'lost-activity', type: 'warning', title: 'Some activity may not have been shown.',
        description: 'The connection to ' + window.dinahProduct.name + ' was interrupted, and some of what happened meanwhile could not be replayed. Chats, questions and settings were read again; text that a chat was still receiving may be missing until the chat is reopened.',
        dismissible: true, className: 'lost-activity' });
    });
    // A card drawn before the regional format arrived is written again in it.
    document.addEventListener('dinah:locale', function () {
      document.querySelectorAll('time.notices-time[datetime]').forEach(function (t) { t.textContent = dinahRegional.format(t.dateTime, 'dayTime'); });
    });
    /* ---- Questions and the other notifications ----------------------------
       A permission request is a question entry in the notifications, with
       the options the agent offers; the area above a chat's composer shows
       the chat's own. The shared model owns both notification views. */
    /* The questions that wait on the person in a chat. A report of what went
       wrong in a finished turn is kept under Notifications until it is
       dismissed, but nothing in the chat waits on it, so it does not put the
       chat under Needs You. */
    function needsCount(id) {
      return notices.list(function (e) { return e.data.attentionChat === id || e.type === 'question' && e.target && e.target.chat === id; }).length;
    }

    /* types are the kinds ticked in the Show menu; none ticked shows every
       kind, as a filter with nothing chosen does on parkscomputing.com. */
    var needsFilter = { types: [], text: '' };
    function needsShown(entry, card) {
      var kind = entry.type === 'success' || entry.type === 'info' ? 'other' : entry.type, types = needsFilter.types, text = needsFilter.text.trim().toLowerCase();
      return (!types.length || types.indexOf(kind) >= 0) && (!text || card.textContent.toLowerCase().includes(text));
    }
    /* The pane draws every entry as a card, the filter hiding some; the
       taskbar item's popover lists them all. */
    var noticePane = notices.mountPane($('questions'), { stable: true, filter: needsShown, drawn: function (info) {
      $('needs-empty').style.visibility = info.shown > 0 ? 'hidden' : '';
      $('needs-empty').textContent = info.total ? 'No notifications match the filter.' : 'No notifications right now.';
    } });
    notices.mountList($('notifications-status-list'), { popover: $('notifications-status-menu'), empty: 'No notifications.' });
    // Down on the item opens its list, as on a menu button.
    $('status-needs').addEventListener('keydown', function (event) {
      if (event.key !== 'ArrowDown' || event.altKey || event.ctrlKey || event.metaKey) return;
      event.preventDefault();
      if (!$('notifications-status-menu').matches(':popover-open')) $('notifications-status-menu').showPopover();
    });
    /* The counts: the item's badge and the Notifications window's mark are
       the entries that need the person, and a chat's mark its questions. */
    var noticeCountsQueued = false;
    var noticesKnown = false;
    function drawNoticeCounts() {
      noticeCountsQueued = false;
      var n = notices.count().needsPerson, clearable = notices.list(function (e) { return e.clearable; }).length > 0;
      $('needs-clear').disabled = !clearable;
      window.dinahCommands.changed();
      var win = document.querySelector('[data-win="needs-you"]');
      if (win) win.dataset.winAttention = String(n);
      var st = $('status-needs'), b = st.querySelector('.badge');
      var errors = notices.list(function (entry) { return entry.needsPerson && entry.type === 'error'; }).length;
      st.dataset.statusState = errors ? 'stuck' : n ? 'waiting' : 'idle';
      // An error waiting is stuck, which the warning badge does not say, so
      // the item gives PUDL's folded Status item that tone (0.97.2).
      if (errors) st.setAttribute('data-status-tone', 'stuck'); else st.removeAttribute('data-status-tone');
      st.dataset.statusText = n ? 'Needs you' : 'Nothing waiting';
      st.dataset.statusGlyph = errors ? 'stop' : n ? 'comments' : 'ring';
      st.dataset.statusColour = errors ? 'danger' : n ? 'warn' : 'text-muted';
      st.dataset.statusCount = n;
      /* Once the stored notifications are read, none waiting shows as a
         neutral 0; before then the badge stays empty, never a false 0. A
         count above 0 keeps its warning badge. */
      if (noticesKnown) st.dataset.statusCountKnown = ''; else delete st.dataset.statusCountKnown;
      b.hidden = n === 0 && !noticesKnown;
      // A count not read yet is pending, so PUDL's folded Status total waits
      // for it (0.97.1).
      b.toggleAttribute('data-status-pending', !noticesKnown);
      b.classList.toggle('warn', n > 0);
      b.textContent = n ? (n > 99 ? '99+' : String(n)) : noticesKnown ? '0' : '';
      if (n) st.removeAttribute('data-empty'); else st.setAttribute('data-empty', '');
      var words = n ? n + ' unread' : 'none';
      st.setAttribute('aria-label', 'Notifications: ' + words);
      st.title = 'Notifications: ' + words;
      document.querySelectorAll('.win[data-win-kind="chat-session"]').forEach(function (chat) {
        var item = window.dinahWorkbench.item(chat.dataset.win);
        chat.dataset.winAttention = String(item ? needsCount(item.path) : 0);
      });
      drawSessions();
    }
    notices.subscribe(function () {
      if (noticeCountsQueued) return;
      noticeCountsQueued = true;
      Promise.resolve().then(drawNoticeCounts);
    });
    noticeCountsQueued = true;
    Promise.resolve().then(drawNoticeCounts);
    /* Opening a chat's permission question from the list shows it in its
       chat, beside the composer; anything else opens in the pane. */
    function openNotice(entry) {
      var t = entry.target;
      if (entry.type !== 'question' || !t || !t.chat || !t.permission) return false;
      return Promise.resolve(openSession(t.chat)).then(function () {
        return new Promise(function (resolve) {
          requestAnimationFrame(function () {
            var card = document.querySelector('.win[data-chat-current] [data-view-id="chat-approvals"] [data-permission-key="' + CSS.escape(t.permission) + '"]');
            if (!card) { resolve(false); return; }
            card.scrollIntoView({ block: 'nearest' });
            card.classList.remove('flash'); void card.offsetWidth; card.classList.add('flash');
            var b = card.querySelector('.btn-primary:not(:disabled)') || card.querySelector('button:not(:disabled)');
            if (b) b.focus({ preventScroll: true });
            resolve(true);
          });
        });
      });
    }

    /* The filter: which kind of entry shows, and text an entry must contain.
       Both are kept for the next session. */
    var NEEDS_FILTER_KEY = 'dinah-notifications-filter', NEEDS_KINDS = ['question', 'error', 'warning', 'other'];
    var NEEDS_NAMES = { question: 'Questions', error: 'Errors', warning: 'Warnings', other: 'Other' };
    /* The Show menu's boxes and its button say what is ticked: All when
       nothing is, else the kinds by name. */
    function setNeedsTypes(types) {
      needsFilter.types = NEEDS_KINDS.filter(function (k) { return types.indexOf(k) >= 0; });
      document.querySelectorAll('#needs-type-menu input[name="needs-type"]').forEach(function (box) { box.checked = needsFilter.types.indexOf(box.value) >= 0; });
      var words = needsFilter.types.length ? needsFilter.types.map(function (k) { return NEEDS_NAMES[k]; }).join(', ') : 'All';
      $('needs-type-words').textContent = words;
      $('needs-type').setAttribute('aria-label', 'Show: ' + words);
    }
    function keepNeedsFilter() { if (window.dinahUIState) window.dinahUIState.set(NEEDS_FILTER_KEY, { types: needsFilter.types, text: needsFilter.text }); }
    $('needs-type-menu').addEventListener('change', function () {
      setNeedsTypes(Array.from(document.querySelectorAll('#needs-type-menu input[name="needs-type"]:checked')).map(function (box) { return box.value; }));
      keepNeedsFilter(); noticePane.refresh();
    });
    $('needs-filter').addEventListener('input', function () { needsFilter.text = $('needs-filter').value; keepNeedsFilter(); noticePane.refresh(); });
    if (window.dinahUIState) window.dinahUIState.ready.then(function () {
      var kept = window.dinahUIState.get(NEEDS_FILTER_KEY, null);
      if (!kept || typeof kept !== 'object') return;
      // A filter kept by an earlier version holds one kind, or "all".
      if (Array.isArray(kept.types)) setNeedsTypes(kept.types.filter(function (k) { return typeof k === 'string'; }));
      else if (NEEDS_KINDS.includes(kept.type)) setNeedsTypes([kept.type]);
      if (typeof kept.text === 'string') { needsFilter.text = kept.text; $('needs-filter').value = kept.text; }
      noticePane.refresh();
    });
    /* Clear All dismisses every report, after asking; a question stays until
       it is answered. The model dismisses an applet's result in Go, where its
       state is kept, and a result that could not be dismissed stays, and the
       failure is said. */
    function clearNotifications() {
      return ask('Clear All Notifications?', 'Every report in Notifications is dismissed. Questions stay until they are answered.', 'Clear All', true).then(function (yes) {
        if (!yes) return;
        return notices.clear().then(function (failed) {
          if (failed.length) fail(new Error((failed.length === 1 ? 'One notification' : failed.length + ' notifications') + ' could not be dismissed: ' + String(failed[0] && failed[0].message || failed[0])));
        });
      }).catch(fail);
    }
    $('needs-clear').addEventListener('click', clearNotifications);

    var notificationWrites = new Map(), notificationSerial = 0;
    function writeNotification(id, run) {
      var entry = { run: run, pending: true, promise: null };
      notificationWrites.set(id, entry);
      entry.promise = Promise.resolve().then(run).then(function (result) {
        if (notificationWrites.get(id) === entry) notificationWrites.delete(id);
        return result;
      }).catch(function (error) { entry.pending = false; throw error; });
      return entry.promise;
    }
    function flushNotifications() {
      return Promise.all(Array.from(notificationWrites, function (pair) {
        return pair[1].pending ? pair[1].promise : writeNotification(pair[0], pair[1].run);
      }));
    }
    /* Dismisses a saved notification in Go, after any write of it still
       pending. The model takes its entry away when this resolves. */
    function dismissStoredNotification(id) {
      var write = notificationWrites.get(id);
      var pending = write ? (write.pending ? write.promise : writeNotification(id, write.run)) : null;
      return Promise.resolve(pending).then(function () {
        return app().DismissNotification ? writeNotification(id, function () { return app().DismissNotification(id); }) : null;
      });
    }
    /* A report is what a toast said, with its time, and for a document
       Open the Document, which reopens its draft. */
    function reportNotice(item) {
      var actions = item.draftId ? [{ label: 'Open the Document', run: function () {
        return Promise.resolve().then(function () { return app().GetDraft(item.draftId); }).then(function (v) { return window.dinahEditor.open(v); })
          .catch(function () { toast('The document\'s draft no longer exists, so it cannot be opened from here.', 'warn'); });
      } }] : [];
      return { id: item.id, type: item.severity, title: item.title, body: item.body || '', description: item.title + (item.body ? ' ' + item.body : ''),
        time: item.created || null, actions: actions, dismissible: true, announced: true, className: 'notification-report',
        data: { notificationId: item.id, reportKey: item.severity + '|' + item.title + '|' + (item.body || '') } };
    }
    /* A turn that ended in an error, about its chat. It needs the person, but
       nothing in the chat waits on it. */
    function turnFailureNotice(item) {
      return { id: item.id, type: 'error', title: item.title, body: item.body, details: item.details || [], description: item.title + ' ' + item.body,
        time: item.created || null, source: title(S(item.sessionId)), target: { chat: item.sessionId }, dismissible: true, announced: true,
        className: 'question heads-up', data: { session: item.sessionId, notificationId: item.id } };
    }

    /* Messages waiting for a chat that could not be opened. The card is about
       the original chat, known by its title, provider and start, since
       several can share a title. Recover continues its saved history in a
       new chat the person chooses; Remove removes the chat from this app and
       cancels its waiting messages, with no new chat; Dismiss and Clear All
       hide the card, which stays hidden until the incident changes, and
       leave the chat and its messages as they are, recoverable from it. */
    function deliveryFailureNotice(item) {
      var entry = turnFailureNotice(item);
      entry.dismissible = true;
      entry.clearable = true;
      if (item.originTitle) entry.source = item.originTitle;
      entry.data.lead = item.lead || '';
      var origin = { title: item.originTitle, provider: item.originProvider, started: item.originStarted };
      entry.actions = [{ label: 'Review Chat', primary: true, run: function () { return openSession(item.sessionId); } }];
        entry.actions.push({ label: 'Recover Original Chat', run: function () { return openSavedRecovery(item.lead, item.workspace, origin); } });
      if (item.lead && app().RemoveChat) entry.actions.push({ label: 'Remove Chat…', run: function () { return openRemove(item.lead, origin); } });
        entry.actions.push({ label: 'Refresh Recovery Status', run: function () { return app().ReloadWaitingChatState().then(function () { return restoreNotifications(); }); } });
      if (item.retryAllowed) entry.actions.push({ label: 'Retry Waiting Delivery', run: function () {
          return app().RetryWaitingSession(item.lead).then(function () { return restoreNotifications(); });
        } });
      return entry;
    }

    function deliveryStoreFailureNotice(item) {
      var entry = reportNotice(item);
      entry.details = item.details || [];
      entry.dismissible = false; entry.clearable = false;
      entry.actions = [{ label: 'Reload Recovery State', primary: true, run: function () {
        return app().ReloadWaitingChatState().then(function () { return restoreNotifications(); });
      } }];
      return entry;
    }
    var attentionNavigation = 0;
    var attentionItems = new Map(), attentionGroups = new Map(), attentionDismissals = new Map();
    function attentionSource(item) { return item.sessionId ? 'chat:' + item.sessionId : item.sourceId ? 'bus:' + item.sourceId : 'message:' + item.id; }
    function forgetAttention(id) {
      var item = attentionItems.get(id);
      if (!item) return;
      attentionItems.delete(id); drawAttention(attentionSource(item));
    }
    function dismissAttention(ids) {
      return Promise.all(ids.map(function (id) {
        if (attentionDismissals.has(id)) return attentionDismissals.get(id);
        var pending = Promise.resolve().then(function () { return dismissStoredNotification(id); }).then(function () { forgetAttention(id); })
          .finally(function () { attentionDismissals.delete(id); });
        attentionDismissals.set(id, pending); return pending;
      }));
    }
    function drawAttention(key) {
      var items = Array.from(attentionItems.values()).filter(function (item) { return attentionSource(item) === key; });
      var group = attentionGroups.get(key);
      if (!items.length) { if (group) notices.remove(group.id); attentionGroups.delete(key); return; }
      if (!group) { group = { id: items[0].id }; attentionGroups.set(key, group); }
      items.sort(function (a, b) { return new Date(a.created) - new Date(b.created) || (a.message || 0) - (b.message || 0); });
      var latest = items[items.length - 1], ids = items.map(function (item) { return item.id; });
      var firstLine = String(latest.body || '').trim().split(/\r?\n/)[0];
      var title = latest.title + (items.length > 1 ? ' (' + items.length + ' requests)' : '');
      function open() {
        attentionNavigation++;
        var opening = latest.group ? window.dinahBus.openGroup(latest.group, { message: latest.message || 0 }) : latest.message ?
          window.dinahWorkbench.open({ kind: 'bus-thread', path: String(latest.message), title: 'Message From ' + latest.sessionTitle }) : latest.sessionId ? openSession(latest.sessionId) : Promise.reject(new Error('The requesting chat is no longer available.'));
        return Promise.resolve(opening).then(function (shown) { if (shown === false || shown === null) throw new Error('The request could not be opened.'); return dismissAttention(ids); }).finally(function () { attentionNavigation--; });
      }
      var actions = [{ label: latest.group ? 'Open Group' : 'Open Chat', primary: true, run: open }];
      if (latest.kind === 'relay' && latest.lead) actions.push({ label: 'Open Lead', run: function () { return openSession(latest.lead); } });
      notices.add({ id: group.id, type: latest.kind === 'attention' ? 'question' : latest.kind === 'relay' ? 'warning' : 'info', needsPerson: true,
        title: title, body: firstLine, description: title + ' ' + firstLine, source: latest.sessionTitle || latest.groupTitle || '', time: latest.created,
        details: [items.length + (items.length === 1 ? ' request' : ' requests') + (latest.groupTitle ? '; latest in ' + latest.groupTitle : '')],
        actions: actions, dismissible: true, dismiss: function () { return dismissAttention(ids); }, announced: true,
        className: 'question heads-up attention-kept', data: { notificationId: group.id, attentionChat: latest.sessionId || '', group: latest.group || '', message: latest.message, requestCount: items.length } });
    }
    function keepAttention(item) {
      if (!item || item.dismissed) return;
      attentionItems.set(item.id, item);
      if (item.permissionKey) notices.remove('question-' + item.permissionKey);
      drawAttention(attentionSource(item));
    }
    // Restoring an applet or refreshing a covered group is not acknowledgement.
    // Only a new foreground target or direct interaction acknowledges its snapshot.
    var lastAttentionWindow = '';
    function acknowledgeWindow(key) {
      if (attentionNavigation || !key || window.dinahUIState && window.dinahUIState.isRestoring()) return;
      var item = window.dinahWorkbench.item(key);
      if (!item) return;
      var ids = Array.from(attentionItems.values()).filter(function (notice) {
        return item.kind === 'chat-session' && notice.sessionId === item.path || item.kind === 'bus-room' && notice.group === item.path || item.kind === 'bus-thread' && notice.message === Number(item.path);
      }).map(function (notice) { return notice.id; });
      if (ids.length) dismissAttention(ids).catch(fail);
    }
    document.addEventListener('pudl:windows-change', function (event) {
      var key = event.detail && event.detail.top;
      if (key === lastAttentionWindow) return;
      lastAttentionWindow = key; acknowledgeWindow(key);
    });
    document.addEventListener('focusin', function (event) {
      var win = event.target.closest('[data-win]');
      if (win && win.classList.contains('active')) acknowledgeWindow(win.dataset.win);
    });
    var SEVERITY = { danger: 'error', warn: 'warning', positive: 'success', info: 'info' };
    /* Keeps what a toast said as a report. The same report again, while an
       earlier copy is still listed, is not added twice. */
    function noteEntry(message, kind, options) {
      if (!notificationWrites || !notices) return;
      var text = String(message).replace(/^Error: /, '').trim();
      if (!text) return;
      var severity = SEVERITY[kind] || 'info', title = options.title || text, body = options.title ? text : '';
      var key = severity + '|' + title + '|' + body;
      if (notices.list(function (e) { return e.data.reportKey === key; }).length) return;
      var item = { id: 'report_' + (window.crypto && window.crypto.randomUUID ? window.crypto.randomUUID() : Date.now() + '_' + (++notificationSerial)),
        kind: options.draftId ? 'document' : 'app', severity: severity, title: title.slice(0, 4000), body: body.slice(0, 60000), details: [],
        draftId: options.draftId || '' };
      notices.add(reportNotice(item));
      if (app() && app().SaveNotification) writeNotification(item.id, function () { return app().SaveNotification(item); }).catch(function () { /* The entry stays for this session. */ });
    }
    function restoreNotifications() {
      if (!app().Notifications) return Promise.resolve();
      var requestedAttention = new Map(attentionItems);
      return app().Notifications().then(function (items) {
        var storedAttention = new Set((items || []).filter(function (item) { return !item.dismissed && /^(mention|relay|attention)$/.test(item.kind); }).map(function (item) { return item.id; }));
        Array.from(attentionItems.keys()).forEach(function (id) { if (requestedAttention.get(id) === attentionItems.get(id) && !storedAttention.has(id)) forgetAttention(id); });
        var waiting = new Set((items || []).filter(function (item) { return item.kind === 'delivery-failure' || item.kind === 'delivery-store-error'; }).map(function (item) { return item.id; }));
        notices.list(function (entry) { return /^(waiting_|transfer_)/.test(String(entry.id)) && !waiting.has(entry.id); }).forEach(function (entry) { notices.remove(entry.id); });
        (items || []).forEach(function (item) {
          if (item.kind === 'delivery-store-error') {
            if (notices.get(item.id)) notices.remove(item.id);
            notices.add(deliveryStoreFailureNotice(item)); return;
          }
          if (item.kind === 'delivery-failure') {
            if (notices.get(item.id)) notices.remove(item.id);
            var waitingSession = S(item.sessionId);
            waitingSession.info.cwd = waitingSession.info.cwd || item.workspace;
            waitingSession.info.provider = waitingSession.info.provider || item.provider;
            waitingSession.info.title = waitingSession.info.title || item.sessionTitle;
            notices.add(deliveryFailureNotice(item)); return;
          }
          if (item.dismissed) return;
          if (item.kind === 'mention' || item.kind === 'relay' || item.kind === 'attention') { keepAttention(item); return; }
          if (notices.get(item.id)) return;
          if (item.kind === 'document' || item.kind === 'app') { notices.add(reportNotice(item)); return; }
          if (item.kind !== 'turn-failure') return;
          var s = S(item.sessionId);
          s.info.cwd = s.info.cwd || item.workspace;
          s.info.provider = s.info.provider || item.provider;
          s.info.title = s.info.title || item.sessionTitle;
          notices.add(turnFailureNotice(item));
        });
        // The stored notifications are read, so a count of none is known.
        if (!noticesKnown) { noticesKnown = true; drawNoticeCounts(); }
      });
    }

    /* sub is the subagent that asks, when one does (subagents.go). */
    function describe(s, tc, sub) {
      var r = tc && s.rows[tc.toolCallId];
      var who = sub ? 'The subagent "' + (sub.name || 'Subagent') + '"' : 'The agent';
      var command = (tc && tc.rawInput && tc.rawInput.command) || (r && r.command);
      if (command) return who + ' wants to run: ' + command;
      if (tc && tc.title) return who + ' wants to: ' + tc.title;
      if (r && r.name) return who + ' wants to: ' + r.name + (r.text ? ': ' + r.text.split('\n')[0] : '');
      return who + ' asks to go on with its next step.';
    }

    var permissionRequests = new Map();
    function waitingApproval(id) {
      return Array.from(permissionRequests.values()).some(function (entry) { return entry.request.sessionId === id; });
    }
    function refreshApprovals(id) {
      var s = getSession(id);
      if (s && s.view) { s.view.drawApprovals(); s.view.stateLine(); }
    }
    function answerPermission(entry, option) {
      var q = entry.request;
      if (entry.answering || permissionRequests.get(q.key) !== entry) return;
      entry.answering = true;
      document.querySelectorAll('[data-permission-key="' + CSS.escape(q.key) + '"] .permission-option').forEach(function (button) { button.disabled = true; });
      var s = S(q.sessionId), tr = q.toolCall && s.rows[q.toolCall.toolCallId];
      if (tr && /^reject/.test(option.kind || '')) tr.rejected = true;
      Promise.resolve().then(function () { return app().Answer(q.key, option.optionId); }).then(function () {
        var r = row(s, 'answer-row');
        r.insertAdjacentHTML('beforeend', '<span class="activity-muted"></span>');
        r.lastChild.textContent = 'You answered "' + option.name + '" to: ' + entry.description;
      }).catch(function (error) {
        if (permissionRequests.get(q.key) !== entry) return;
        entry.answering = false;
        if (tr && /^reject/.test(option.kind || '')) tr.rejected = false;
        document.querySelectorAll('[data-permission-key="' + CSS.escape(q.key) + '"] .permission-option').forEach(function (button) { button.disabled = false; });
        fail(error);
      });
    }
    /* The question beside a chat's composer, made as the model makes its
       cards, with the question's glyph and colour. */
    function permissionCard(entry, inline) {
      var card = document.createElement('div');
      card.className = 'notice question';
      notices.decorate(card, 'question');
      card.dataset.permissionKey = entry.request.key;
      card.dataset.session = entry.request.sessionId;
      card.setAttribute('role', 'status');
      fillPermission(card, entry, inline);
      return card;
    }
    /* A permission question's text and the options the agent offers, in the
       chat's copy and in the card under Notifications. */
    function fillPermission(card, entry, inline) {
      var q = entry.request;
      card.innerHTML = '<div class="notice-content"><p class="notice-title"></p><p class="notice-body"></p><div class="notice-actions hstack gap-2"></div></div>';
      card.querySelector('.notice-title').textContent = inline ? 'Waiting for Your Approval' : entry.description;
      card.querySelector('.notice-body').textContent = inline ? entry.description : 'The agent waits for your answer before it goes on. Asked at ' + entry.askedAt + '.';
      var actions = card.querySelector('.notice-actions');
      var primaryDone = false;
      (q.options || []).forEach(function (o) {
        var b = document.createElement('button');
        b.type = 'button';
        var primary = !primaryDone && o.kind === 'allow_once';
        if (primary) primaryDone = true;
        b.className = 'btn btn-sm permission-option' + (primary ? ' btn-primary' : '');
        b.textContent = o.name;
        b.disabled = !!entry.answering;
        b.addEventListener('click', function () { answerPermission(entry, o); });
        actions.appendChild(b);
      });
    }
    /* Every key the page has been asked about, kept after the question is
       answered, so a question that a reread returns after its answer was seen
       is not shown again. Keys are never reused, so the set only grows by one
       per question; the oldest go once it is long. */
    var seenPermissions = new Set();
    function permissionAsked(q) {
      var s = S(q.sessionId);
      s.current = null;
      if (seenPermissions.has(q.key)) return;
      seenPermissions.add(q.key);
      if (seenPermissions.size > 1000) seenPermissions.delete(seenPermissions.values().next().value);
      var entry = { request: q, description: describe(s, q.toolCall, q.subagent), askedAt: now(), answering: false };
      permissionRequests.set(q.key, entry);
      if (!Array.from(attentionItems.values()).some(function (item) { return item.permissionKey === q.key && item.sessionId === q.sessionId; })) notices.add({ id: 'question-' + q.key, type: 'question', description: entry.description, source: title(s),
        target: { chat: q.sessionId, permission: q.key }, announced: true, className: 'question',
        data: { permissionKey: q.key, session: q.sessionId },
        render: function (card) { card.id = 'question-' + q.key; fillPermission(card, entry, false); } });
      refreshApprovals(q.sessionId);
      // The entry's toast goes with it, always, and announces it.
      toast('"' + title(s) + '" has a question for you.', 'warn', { noted: true });
    }
    on('agent:permission', permissionAsked);

    // Tool failures have an optional summary in the run. An unsuccessful
    // turn still raises an error, since it can leave the conversation stuck.
    function headsUp(s, failures, error) {
      window.dinahFailures.record(s, failures, title(s));
      if (!error) return;
      var notification = { id: 'notice_' + (window.crypto && window.crypto.randomUUID ? window.crypto.randomUUID() : Date.now() + '_' + (++notificationSerial)),
        kind: 'turn-failure', severity: 'error', title: 'The turn ended with an error.', body: error + ' Ended at ' + now() + '.', details: [],
        sessionId: s.id, sessionTitle: title(s), workspace: s.info.cwd || workspace(), provider: sessionProvider(s) };
      notices.add(turnFailureNotice(notification));
      if (app().SaveNotification) writeNotification(notification.id, function () { return app().SaveNotification(notification); }).catch(fail);
      toast('"' + title(s) + '": ' + notification.title, 'danger', { noted: true });
    }

    function permissionAnswered(a) {
      var entry = permissionRequests.get(a.key);
      permissionRequests.delete(a.key);
      notices.remove('question-' + a.key);
      if (entry) refreshApprovals(entry.request.sessionId);
    }
    on('agent:answered', permissionAnswered);
    // The engine makes a card when a chat mentions the person and ends it when the message is read.
    on('notifications:added', function (item) {
      if (item && (item.kind === 'mention' || item.kind === 'relay' || item.kind === 'attention')) keepAttention(item);
    });
    on('notifications:dismissed', function (done) { if (done && done.id) { if (attentionItems.has(done.id)) forgetAttention(done.id); else notices.remove(done.id); } });
    on('notification:delivery', function () { restoreNotifications().catch(fail); });
    /* After a resync the engine's pending questions are read again: one the
       page has not seen is shown, and one the page shows that is no longer
       pending was answered or ended while the page was not told. */
    window.dinahHost.onResync(function () {
      if (!app().PendingPermissions) return null;
      return Promise.resolve(app().PendingPermissions()).then(function (list) {
        list = list || [];
        var pending = new Set(list.map(function (q) { return q.key; }));
        Array.from(permissionRequests.keys()).forEach(function (key) { if (!pending.has(key)) permissionAnswered({ key: key }); });
        list.forEach(permissionAsked);
      });
    });

    var lastNotification = null;
    function showNotifications() {
      return window.dinahWorkbench.open('needs-you').then(function () {
        requestAnimationFrame(function () {
          var button = document.querySelector('#questions .btn-primary, #questions button');
          if (button) button.focus();
        });
      }).catch(fail);
    }
    /* Steps through the entries that need the person, the questions and the
       errors, that the filter shows. */
    function nextNotification() {
      var ids = notices.list(function (e) { return e.needsPerson && noticePane.shown(e.id); }).map(function (e) { return e.id; });
      if (!ids.length) return window.dinahWorkbench.open('needs-you').catch(fail);
      var id = ids[(ids.indexOf(lastNotification) + 1) % ids.length];
      lastNotification = id;
      return window.dinahWorkbench.open('needs-you').then(function () { noticePane.reveal(id); }).catch(fail);
    }

    function providerStopped(provider) {
        permissionRequests.forEach(function (entry, key) {
          if (sessionProvider(getSession(entry.request.sessionId)) !== provider) return;
          permissionRequests.delete(key);
          refreshApprovals(entry.request.sessionId);
        });
        // The questions die with the process; what went wrong in a turn stays
        // until it is dismissed.
        notices.list(function (e) { return e.type === 'question' && e.target && e.target.chat && sessionProvider(getSession(e.target.chat)) === provider; })
          .forEach(function (e) { notices.remove(e.id); });
    }
    function permissions(id) {
      return Array.from(permissionRequests.values()).filter(function (entry) { return entry.request.sessionId === id; });
    }

    return { notices, ask, noteEntry, needsCount, drawNoticeCounts,
      flushNotifications, restoreNotifications, waitingApproval, permissionCard,
      headsUp, showNotifications, nextNotification, clearNotifications,
      permissions, providerStopped };
  }

  window.dinahNotifications = { create };
}());
