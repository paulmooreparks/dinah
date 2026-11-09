(function () {
  'use strict';
  /* ---- Group chats and the bus ----------------------------------------------
     The Group Chats pane holds Paul's inbox on the message bus, the group
     chats, every chat by workspace and the trusted links, with a form for a
     new message (docs/plans/group-chats.md). A group chat's members are
     chats and Paul. A group opens in a window of its own, with its members,
     its conversation and a box to write in, where @ and a member's title
     mentions that chat, which wakes it. The picker that starts a group, or
     adds chats to one, is a window too, so it has an address as every
     window does. A message's thread opens in a window with a box to reply
     in. Everything is drawn again when the bus changes (bus:changed,
     bus_views.go). A group's or a thread's messages are drawn as a chat
     draws its own, by the shared renderer (chat-text.js), and its box is
     the chat's box (composer.js), so both follow the chat's standard; what
     differs is set here, such as a group's @ list of its members. A chat is
     shown by its title and a workspace by its name in the window. */
  const app = window.dinahHelpers.app;
  function host() { return window.dinahHost; }
  const message = window.dinahHelpers.message;
  const node = window.dinahHelpers.node;
  var groupSerial = 0;
  function when(stamp) {
    var d = new Date(stamp);
    return isNaN(d) ? stamp : dinahRegional.format(d, 'dayTime');
  }
  document.addEventListener('dinah:execution-state', function () { listeners.forEach(function (refresh) { refresh(); }); });
  /* How many times the bus has changed since the page opened, for the
     conversations to know whether their messages may have changed. */
  var busChanges = 0;
  if (host()) host().on('bus:changed', function () { busChanges++; });
  function firstLine(text) { var line = String(text || '').split('\n').find(function (l) { return l.trim(); }) || ''; return line.length > 140 ? line.slice(0, 139) + '…' : line; }
  function glyphButton(cls, glyph, label) {
    var b = node('button', 'icon-btn ' + cls); b.type = 'button';
    b.setAttribute('aria-label', label); b.title = label;
    b.innerHTML = '<span class="glyph" aria-hidden="true"></span>';
    b.firstChild.style.setProperty('--glyph', 'var(--glyph-' + glyph + ')');
    return b;
  }

  var overview = { inbox: [], unread: 0, rooms: [], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }, loading = null, listeners = new Set();
  var overviewVersion = 0, overviewFailure = null;
  function load() {
    if (!app() || !app().BusOverview) return Promise.resolve(overview);
    if (loading) return loading;
    loading = Promise.resolve(app().BusOverview()).then(function (o) {
      overview = o || overview;
      overviewVersion++; overviewFailure = null;
      listeners.forEach(function (fn) { fn(); });
      // A chat's cards show its title as it is now (app.js, deliveredSender).
      document.dispatchEvent(new CustomEvent('dinah:bus-chats'));
      if (window.pudlCommands) window.pudlCommands.changed();
      return overview;
    }).catch(function (error) {
      overviewVersion++; overviewFailure = error;
      listeners.forEach(function (fn) { fn(); });
      throw error;
    }).finally(function () { loading = null; });
    return loading;
  }
  /* A chat open on the bus, by its session there, with its workspace's
     name in the window. */
  function chat(id) {
    var found = null;
    (overview.workspaces || []).forEach(function (w) {
      (w.chats || []).forEach(function (c) { if (c.id === id) found = { id: c.id, title: c.title, workspace: w.label || w.name }; });
    });
    (overview.rooms || []).forEach(function (r) {
      (r.members || []).forEach(function (m) { if (!found && m.id === id && m.state !== 'person') found = { id: m.id, title: m.title, workspace: m.workspace }; });
    });
    return found;
  }
  function group(key) { return (overview.rooms || []).find(function (r) { return r.name === key; }) || null; }
  function resolveGroup(key) {
    if (!app().BusResolveGroup) return Promise.reject(new Error('The group lookup service is unavailable.'));
    return Promise.resolve().then(function () { return app().BusResolveGroup(key); }).then(function (result) {
      if (!result || !['resolved', 'not-found', 'ambiguous'].includes(result.state) ||
          (result.state === 'resolved' && (!result.room || !result.room.name))) throw new Error('The group lookup returned an invalid result.');
      return result;
    });
  }
  function lookupWords(state) {
    return state === 'ambiguous' ? 'More than one group used this name. Open the group from Group Chats.' : 'This group chat is unavailable. Open Group Chats to find it.';
  }
  /* How a person writes a workspace as an address: its name in the window,
     unless another workspace has that name too. */
  function workspaceAddress(w) {
    var label = w.label || w.name;
    var same = overview.workspaces.filter(function (x) { return (x.label || x.name).toLowerCase() === label.toLowerCase(); }).length;
    return same > 1 ? w.name : label;
  }
  /* The addresses a message can go to, for the To field's suggestions: each
     chat open on the bus by its title and workspace, each workspace, and
     each group chat. */
  function addresses() {
    var out = [];
    overview.workspaces.forEach(function (w) {
      out.push(workspaceAddress(w));
      (w.chats || []).forEach(function (c) { out.push(c.title + ' (' + workspaceAddress(w) + ')'); });
    });
    overview.rooms.forEach(function (r) { out.push('#' + r.title); });
    return out;
  }
  /* The window's chats, open or closed (app.js, dinahChats). */
  function knownChats() { return window.dinahChats ? window.dinahChats.known() : []; }
  /* Calls fn when the window's chats change in a way this list shows. The
     chat list is drawn again for many reasons that change none of them,
     such as each step of a turn, so the chats are compared first. It
     returns the function that stops listening. */
  function onChats(fn) {
    var last = JSON.stringify(knownChats());
    function check() {
      var now = JSON.stringify(knownChats());
      if (now === last) return;
      last = now;
      fn();
    }
    document.addEventListener('dinah:chats', check);
    return function () { document.removeEventListener('dinah:chats', check); };
  }

  function memberState(member) {
    var ref = member.chat && window.dinahChats && window.dinahChats.ref(member.chat);
    return ref ? Object.assign({}, member, { exec: ref.exec, life: ref.life, window: ref.window, elsewhere: ref.elsewhere }) : member;
  }
  // A chat's or group's state as the shared state chip (chat-state.js).
  function stateBadge(value, cls) { return window.dinahChatState.badge(value, cls ? { className: cls } : undefined); }
  /* The members of a group as a sentence: its chats' titles, and you. */
  function memberNames(r) {
    var t = (r.members || []).filter(function (m) { return m.state !== 'person'; }).map(function (m) { return m.title; }).concat(['you']);
    return t.length === 1 ? t[0] : t.slice(0, -1).join(', ') + ' and ' + t[t.length - 1];
  }

  function openRoom(r) { return window.dinahWorkbench.open({ kind: 'bus-room', path: r.name, title: r.title || r.name }); }
  /* The picker of a group chat: for a new group, ticking the chat it was
     opened from, or for chats to add to a group. */
  function openPicker(o) {
    var r = o.group ? group(o.group) : null;
    var path = JSON.stringify(o.group ? { group: o.group, from: o.from || '' } : { from: o.from || '' });
    return window.dinahWorkbench.open({ kind: 'group-picker', path: path, title: r ? 'Add Chats to ' + r.title : 'Start a Group Chat' });
  }

  /* The mentions a message makes, as its text writes them: @ and the title
     of each chat in its list of mentions, and @everyone in a group, the
     longest first so that a title inside a longer one is not taken for it. */
  function mentionNames(m) {
    var names = (m.mentions || []).map(function (x) { return '@' + x.title; });
    if (m.room) names.push('@everyone');
    return names.filter(function (n, i) { return n.length > 1 && names.indexOf(n) === i; }).sort(function (a, b) { return b.length - a.length; });
  }
  /* Marks the mentions in a message's drawn text, outside code, links and
     math. The renderer gives the nodes here before they are placed
     (chat-text.js). */
  function markMentions(el, m) {
    var names = mentionNames(m);
    if (!names.length) return;
    var walk = document.createTreeWalker(el, NodeFilter.SHOW_TEXT, { acceptNode: function (n) { return n.parentElement.closest('pre, code, a, .msg-math') ? NodeFilter.FILTER_REJECT : NodeFilter.FILTER_ACCEPT; } });
    var texts = [];
    while (walk.nextNode()) texts.push(walk.currentNode);
    texts.forEach(function (t) {
      var text = t.nodeValue, at = 0, frag = null;
      while (at < text.length) {
        var next = -1, which = '';
        names.forEach(function (n) { var i = text.indexOf(n, at); if (i >= 0 && (next < 0 || i < next)) { next = i; which = n; } });
        if (next < 0) break;
        frag = frag || document.createDocumentFragment();
        if (next > at) frag.appendChild(document.createTextNode(text.slice(at, next)));
        frag.appendChild(node('span', 'mention', which));
        at = next + which.length;
      }
      if (!frag) return;
      if (at < text.length) frag.appendChild(document.createTextNode(text.slice(at)));
      t.parentNode.replaceChild(frag, t);
    });
  }
  /* The workspace folder a message's sender works in, from which a path
     the message gives as a link is read; the person has none. */
  function senderFolder(m) {
    var found = '';
    (overview.workspaces || []).forEach(function (w) {
      if (found) return;
      if ((w.chats || []).some(function (c) { return c.id === m.fromSession; }) || (m.fromWorkspace && (w.label === m.fromWorkspace || w.name === m.fromWorkspace))) found = w.path || '';
    });
    return found;
  }
  /* One message as a group or a thread shows it, or the line a group shows
     for a chat that joined or left it. A message is the chat's message card
     (chat-text.js): its head has the sender's title, the sender's
     workspace, whom it went to outside a group, and when it was sent, in
     the computer's regional format; its text is Markdown with its mentions
     marked; the person's own offers Edit and any other Quote, and both offer
     Copy. act(act, view) does Edit and Quote. */
  function messageView(m, act) {
    if (m.kind === 'member') {
      var line = node('p', 'system-line', m.line || m.text);
      line.dataset.busMessage = String(m.id);
      return { div: line };
    }
    var head = node('span', 'bus-message-head');
    head.appendChild(node('strong', null, m.from));
    if (m.fromWorkspace && m.fromWorkspace !== m.from) head.appendChild(node('span', 'md-meta', 'in ' + m.fromWorkspace));
    if (!m.room) { head.appendChild(node('span', 'md-meta', 'to')); head.appendChild(node('strong', null, m.to)); }
    var sent = node('time', 'md-meta bus-message-when');
    window.dinahRegional.stamp(sent, m.createdAt, 'dayTime');
    head.appendChild(sent);
    var view = window.dinahChatText.card({ you: !!m.fromPerson, text: String(m.text || ''), label: head, act: act });
    view.div.classList.add('bus-message');
    view.div.dataset.busMessage = String(m.id);
    view.message = m;
    if (m.relay) {
      var relay = node('p', 'bus-relay-mark');
      var glyph = node('span', 'glyph'); glyph.style.setProperty('--glyph', 'var(--glyph-info)'); glyph.setAttribute('aria-hidden', 'true');
      relay.append(glyph, document.createTextNode('For Paul through ' + m.relay.leadTitle));
      view.div.insertBefore(relay, view.el);
    }
    view.decorate = function (el) { markMentions(el, m); };
    // The paths in a message are read from its sender's folder.
    view.cwd = function () { return senderFolder(m); };
    if (m.kind === 'release') view.div.insertBefore(node('p', 'bus-message-kind', 'Release ' + m.version), view.el);
    if (m.kind === 'verdict') view.div.insertBefore(node('p', 'bus-message-kind', 'Verdict on ' + m.version + ': ' + m.verdict), view.el);
    // The attachments go under the text, outside it, so that drawing the
    // text again never takes them away.
    if (m.attachments && m.attachments.length) view.div.insertBefore(window.dinahChatText.attachments(m.attachments), view.foot);
    window.dinahChatText.draw(view);
    return view;
  }

  /* The box to write in: the chat's box (composer.js), which grows with
     its text, sends with the key chosen in Settings › Chat, and brings back
     the person's earlier messages with Up and Down. o.label is the box's
     name for a screen reader, o.placeholder is its hint, and o.send(text, state)
     returns a promise. What the box holds is its text and whatever a window
     adds to it through box.extra, such as a group's mentions; that is what
     a draft, an entry of the history and Edit put back. A box with
     o.draftKey keeps what was typed in it while its window is closed and
     across a restart (composer.js, keeper), with the draft Up set aside
     when an earlier message is in the box.

     A box with o.files takes files as a chat's box does: pasted into it,
     dropped on the window (box.dropOn) or chosen with Add, shown as the
     chat's chips with Remove (composer.js, attachments), and sent with the
     message as o.send's third argument. A message may be files alone. The
     files are not part of the draft, the history or Edit, as a chat's
     pasted files are not kept across a restart: their contents are held
     only in memory. o.limit() is the limit on the files in one message
     (composer.js, attachments); at the limit the computer-file menu item is disabled, with the
     reason shown, and box.attachReason() is that reason for the
     window's menu. */
  function composer(o) {
    var form = node('form', 'bus-composer composer');
    var parts = window.dinahComposer.box({ label: o.label, placeholder: o.placeholder });
    var text = parts.text, go = parts.button, sending = false, closed = false, files = [], chips = null;
    var box = { el: form, text: text, button: go, pop: parts.pop, submit: submit,
      extra: { read: function () { return {}; }, write: function () {} } };
    function attachmentFailed(reason) {
      var notice = node('div', 'notice danger group-attachment-error');
      notice.setAttribute('role', 'alert');
      var content = node('div', 'notice-content');
      content.appendChild(node('p', 'notice-title', 'Not Attached'));
      content.appendChild(node('p', 'notice-body', reason));
      var actions = node('div', 'notice-actions');
      var dismiss = node('button', 'btn btn-sm', 'Dismiss'); dismiss.type = 'button';
      dismiss.addEventListener('click', function () { notice.remove(); });
      actions.appendChild(dismiss); content.appendChild(actions); notice.appendChild(content);
      layout.content.appendChild(notice);
      host().toast(reason, 'danger', { title: 'Not attached:' });
    }
    if (o.files) {
      var list = node('ul', 'attachments group-attachments'); list.setAttribute('aria-label', 'Attached to the next message'); list.hidden = true;
      var input = node('input'); input.type = 'file'; input.multiple = true; input.hidden = true;
      chips = window.dinahComposer.attachments({ host: list, get: function () { return files; }, set: function (l) { files = l; },
        changed: function () { changed(); }, toast: attachmentFailed,
        limit: o.limit, alive: function () { return form.isConnected; } });
      input.addEventListener('change', function () { chips.add(input.files); input.value = ''; text.focus(); });
      var attach = node('button', 'btn group-add'); attach.type = 'button'; attach.setAttribute('data-command', 'group-add');
      var unregisterAdd = pudlCommands.register(form, function () { return [{ id: 'group-add', label: 'Add', glyph: 'clip', disabled: closed,
        items: function () { return dinahComposer.addItems({full:chips.full,fullReason:chips.fullReason,files:box.attach,browse:box.browse,mention:function (scope) { box.addMention(scope); }}); } }]; });
      parts.el.querySelector('.composer-actions').insertBefore(attach, go);
      form.append(list, input);
      // A picture or a file pasted into the box goes with the message. Text
      // pasted with it is still typed in, as in a chat's box.
      text.addEventListener('paste', function (e) {
        var found = chips.clipboardFiles(e.clipboardData);
        if (!found.length) return;
        if (!e.clipboardData.getData('text/plain')) e.preventDefault();
        chips.add(found);
      });
      box.attach = function () { return chips.pick(input).then(function () { text.focus(); }).catch(function (err) { attachmentFailed(message(err)); }); };
      box.dropOn = function (zone) { chips.dropOn(zone, function () { text.focus(); }); };
      box.files = function () { return files; };
      box.attachReason = function () { return chips.fullReason(); };
      box.addReference = function (at) {
        if (closed) return;
        if (!files.some(function (f) { return JSON.stringify(f) === JSON.stringify(at); })) files.push(at);
        chips.draw(); changed(); text.focus();
      };
      box.browse = function () {
        return dinahFilePicker.pick({ mode:'open',any:true,title:'Attach',okLabel:'Attach',commit:function (choice) {
          if (choice.computer) return box.attach().then(function () { return true; });
          if (!choice.node || !choice.node.ref) throw new Error('Choose a workspace file.');
          box.addReference({kind:'resource',name:choice.node.title || choice.node.name || choice.node.ref.path,ref:choice.node.ref}); return true;
        }}).catch(function (err) { attachmentFailed(message(err)); });
      };
      // The limit changed, such as when a member joined or a provider's
      // limit was set: the chips and Add menu follow it.
      box.limitChanged = function () { chips.draw(); changed(); };
    }
    form.appendChild(parts.el);
    var layout = window.dinahComposer.layout(form); box.content = layout.content; box.dispose = function () { if (unregisterAdd) unregisterAdd(); layout.destroy(); };
    box.read = function () { return Object.assign({ text: text.value }, box.extra.read()); };
    box.write = function (state) { state = state || {}; text.value = state.text || ''; box.extra.write(state); changed(); };
    // What a message of the person's puts back in the box: its text, and
    // whatever the window adds from the message.
    box.entry = function (m) { return { text: String(m.text || '') }; };
    box.history = window.dinahComposer.history(box.read, box.write);
    var kept = null, lastText = null, lastFiles = -1;
    /* Called for every change of the box and for every redraw of the
       window, so it touches only what changed: the box is sized again only
       for new text, and the commands are redrawn only when Add's state or
       the files it offers changed. */
    function changed() {
      if (text.value !== lastText) { lastText = text.value; window.dinahComposer.fit(text); }
      var goOff = closed || sending || (!text.value.trim() && !files.length);
      if (go.disabled !== goOff) go.disabled = goOff;
      var attach = chips && form.querySelector('.group-add');
      if (attach && (attach.disabled !== closed || files.length !== lastFiles)) {
        attach.disabled = closed; lastFiles = files.length;
        if (window.pudlCommands) pudlCommands.changed();
      }
      if (!kept) return;
      var draft = box.history.draft(box.read());
      kept.save(draft && draft.text ? draft : null);
    }
    box.changed = changed;
    // A box whose group has gone takes nothing more.
    box.close = function (gone) {
      closed = gone; text.disabled = gone;
      changed();
    };
    text.addEventListener('input', changed);
    function submit() {
      var words = text.value.trim();
      if ((!words && !files.length) || sending || closed) return;
      var state = box.read(), sent = chips ? chips.files().concat(files.filter(function (f) { return f.kind !== 'file' && (!f.token || referenceToken(f.token).test(text.value)); }).map(function (f) { var at = Object.assign({}, f); delete at.token; return at; })) : [];
      sending = true; changed();
      Promise.resolve(o.send(words, state, sent)).then(function () {
        if (words) box.history.push(Object.assign({}, state, { text: words }));
        box.write(null);
        if (chips) { files.length = 0; chips.draw(); }
        if (box.sent) box.sent();
      }).catch(function (err) { host().toast(message(err), 'danger', { title: 'Not sent:' }); })
        .finally(function () { sending = false; changed(); text.focus(); });
    }
    form.addEventListener('submit', function (e) { e.preventDefault(); submit(); });
    window.dinahComposer.bind(text, { history: box.history, send: submit });
    // What was typed before the window was closed, or before a restart,
    // comes back, unless something has been typed since.
    box.restore = function () {
      if (o.draftKey && !kept) kept = window.dinahComposer.keeper(o.draftKey, function (value) { if (!text.value) box.write(value); else changed(); });
      changed();
    };
    // A deleted group's draft goes with it.
    box.forget = function () { if (kept) kept.forget(); else if (o.draftKey) window.dinahComposer.forget(o.draftKey); };
    return box;
  }

  /* ---- The Group Chats pane ---- */
  window.dinahWorkbench.register('bus', { applet: { id: 'dinah.groups', title: 'Group Chats', glyph: 'account', workspace: false }, singleton: true, title: 'Group Chats', glyph: 'inbox', mode: 'dock-right', init: function (root) {
    root.classList.add('bus-browser');
    var disposed = false;
    var bar = window.dinahBrowserList.toolbar({
      primary: { label: 'New Group Chat…', title: 'Start a group chat with chats you choose', run: function () { openPicker({}).catch(host().fail); } },
      actions: [{ label: 'Trust a Link…', run: function () { showLinkForm(null); } }, { label: 'Refresh', run: refresh }]
    });
    // New Message… stands beside New Group Chat…, before the Actions menu.
    var newMessage = node('button', 'btn btn-sm', 'New Message…'); newMessage.type = 'button';
    newMessage.title = 'Write to one chat, a workspace or a group chat';
    newMessage.addEventListener('click', function () { showForm(); });
    var commandsEl = bar.el.querySelector('.browser-commands');
    commandsEl.insertBefore(newMessage, commandsEl.querySelector('.browser-actions'));
    var forms = node('div', 'bus-forms'), groups = node('div', 'bus-results');
    root.append(bar.el, forms, groups);

    function refresh() { load().catch(host().fail); draw(); readInbox(); }
    /* The inbox's list, read a page at a time for the view chosen
       (docs/design/inbox.md): items newest first, nextBefore for the page
       after, and newest, the highest inbox message when it was read, which
       bounds the bulk actions so a message arriving later is untouched. */
    var inboxState = { view: 'unread', items: [], nextBefore: 0, newest: 0, loading: false, error: '', generation: 0 };
    function readInbox(more) {
      if (!app().BusInboxPage) { inboxState.items = (overview.inbox || []).filter(function (m) { return inboxState.view === 'all' || (inboxState.view === 'unread' ? m.unread && !m.dismissed : m.dismissed); }); draw(); return Promise.resolve(); }
      var mine = ++inboxState.generation, view = inboxState.view;
      inboxState.loading = true; if (!more) inboxState.error = '';
      /* Load Older reads the next page. A refresh reads again every page
         already shown, down to the oldest row shown, so each row's state is
         current and the list does not shrink under the reader. */
      var depth = !more && inboxState.items.length ? inboxState.items[inboxState.items.length - 1].id : 0;
      var items = more ? inboxState.items.slice() : [], newest = 0;
      function next(before, pages) {
        return Promise.resolve(app().BusInboxPage(view, before)).then(function (page) {
          page = page || {};
          if (!newest) newest = page.newest || 0;
          (page.items || []).forEach(function (m) { if (!items.some(function (x) { return x.id === m.id; })) items.push(m); });
          var cursor = page.nextBefore || 0;
          if (!more && cursor && depth && cursor > depth && pages < 50) return next(cursor, pages + 1);
          return cursor;
        });
      }
      return next(more ? inboxState.nextBefore : 0, 1).then(function (cursor) {
        if (disposed || mine !== inboxState.generation) return;
        inboxState.items = items; inboxState.nextBefore = cursor; inboxState.newest = newest || inboxState.newest;
        inboxState.loading = false; draw();
      }).catch(function (error) {
        if (disposed || mine !== inboxState.generation) return;
        inboxState.loading = false; inboxState.error = message(error); draw();
      });
    }
    /* The inbox's two bulk actions, each confirmed first and applied to
       every message up to the newest the list was read with. The inbox's
       buttons and the window's commands run the same actions, and offer
       them in the same cases (bulkActions). */
    function bulk(label, heading, body, yes, run) {
      return { label: label, run: function () {
        var newest = inboxState.newest;
        if (!newest) return;
        return host().ask(heading, body, yes, false).then(function (ok) {
          if (!ok) return;
          return Promise.resolve(run(newest)).then(function () { return load(); }).then(function () { return readInbox(); });
        }).catch(host().fail);
      } };
    }
    var markAllRead = bulk('Mark All Read', 'Mark All Read?', 'Every message in the inbox up to now is marked read. Newer ones stay as they are. Marking a message read does not answer it or approve anything.', 'Mark Read', function (newest) { return app().BusMarkInboxReadThrough(newest); });
    var dismissAll = bulk('Dismiss All', 'Dismiss All?', 'Every message in the inbox up to now is hidden from it, and stays in its group or thread. Newer ones stay. Dismissing a message does not mark it read, answer it or approve anything.', 'Dismiss', function (newest) { return app().BusDismissInboxThrough(newest); });
    // The bulk actions that apply to the inbox as it is shown.
    function bulkActions() {
      var any = inboxState.items.length > 0;
      return [inboxState.view === 'unread' && any ? markAllRead : null, inboxState.view !== 'dismissed' && any ? dismissAll : null].filter(Boolean);
    }
    function inboxBulk(action) {
      var b = node('button', 'btn btn-sm', action.label); b.type = 'button';
      b.addEventListener('click', function () { action.run(); });
      return b;
    }
    /* Opens an inbox message where it was said and marks it read once it is
       shown there, the one message alone. */
    function openInboxItem(m) {
      var done = false, timer = 0;
      function shown(e) {
        if (done || !e.detail || e.detail.message !== m.id) return;
        done = true; clearTimeout(timer); document.removeEventListener('dinah:bus-message-shown', shown);
        if (m.unread && app().BusMarkInboxRead) Promise.resolve(app().BusMarkInboxRead(m.id)).then(function () { return load(); }).then(function () { readInbox(); }).catch(host().fail);
      }
      document.addEventListener('dinah:bus-message-shown', shown);
      timer = setTimeout(function () { done = true; document.removeEventListener('dinah:bus-message-shown', shown); }, 30000);
      var opening = m.room ? window.dinahBus.openGroup(m.room, { message: m.id, inbox: true })
        : window.dinahWorkbench.open({ kind: 'bus-thread', path: String(m.id), title: 'Message From ' + (m.fromPerson ? m.to : m.from) });
      return Promise.resolve(opening).catch(function (error) { done = true; clearTimeout(timer); document.removeEventListener('dinah:bus-message-shown', shown); host().fail(error); });
    }
    function field(label, input) { var l = node('label', 'form-field'); l.appendChild(node('span', 'form-label', label)); l.appendChild(input); return l; }
    /* The form for a new message, in the pane above the lists. */
    function showForm(to) {
      forms.replaceChildren();
      var f = node('form', 'card bus-form vstack gap-2');
      var list = node('datalist'); list.id = 'bus-addresses';
      addresses().forEach(function (a) { list.appendChild(new Option(a)); });
      var first = node('input', 'form-input'), second = node('textarea', 'form-input');
      first.setAttribute('list', list.id); first.placeholder = 'A chat, a workspace or a group chat';
      second.rows = 4;
      f.append(node('h3', 'bus-form-title', 'New Message'), field('To', first), field('Message', second), list);
      var actions = node('div', 'hstack gap-2 bus-form-actions');
      var ok = node('button', 'btn btn-primary btn-sm', 'Send'); ok.type = 'submit';
      var cancel = node('button', 'btn btn-sm', 'Cancel'); cancel.type = 'button';
      actions.append(ok, cancel);
      f.appendChild(actions);
      cancel.addEventListener('click', hideForm);
      f.addEventListener('submit', function (e) {
        e.preventDefault();
        ok.disabled = true;
        Promise.resolve(app().BusSend(first.value, second.value, 0)).then(function () { hideForm(); load(); })
          .catch(function (err) { host().toast(message(err), 'danger', { title: 'Not sent:' }); })
          .finally(function () { ok.disabled = false; });
      });
      forms.appendChild(f);
      if (to) { first.value = to; second.focus(); } else first.focus();
    }
    function hideForm() { forms.replaceChildren(); }

    /* The form for a trusted link: the two workspaces, and the link's limit
       on automatic handoffs or none. Turning the limit off asks first. An
       existing link's form can also remove it. */
    function limitText(n) { return n === overview.noHopLimit ? 'No limit on automatic handoffs' : 'Up to ' + n + ' automatic handoffs'; }
    function showLinkForm(link) {
      forms.replaceChildren();
      var f = node('form', 'card bus-form vstack gap-2');
      f.dataset.busLinkForm = '';
      function picker(value) {
        var wrap = node('span', 'form-select-wrap'), s = node('select', 'form-select');
        overview.workspaces.forEach(function (w) { s.add(new Option(w.label || w.name, w.name)); });
        if (value) s.value = value;
        wrap.appendChild(s);
        return { el: wrap, select: s };
      }
      var names = overview.workspaces.map(function (w) { return w.name; });
      var one = picker(link ? link.a : names[0]), two = picker(link ? link.b : names[1] || names[0]);
      var limit = node('input', 'form-input'); limit.type = 'number'; limit.min = '1';
      // PUDL's switch: a button whose aria-checked says whether it is on.
      var none = node('button', 'switch'); none.type = 'button'; none.setAttribute('role', 'switch');
      var track = node('span', 'switch-track'); track.appendChild(node('span', 'switch-thumb'));
      none.append(track, document.createTextNode('No Limit'));
      var off = link && link.hopLimit === overview.noHopLimit;
      limit.value = String(link && !off ? link.hopLimit : overview.defaultHops);
      function setNone(on) { none.setAttribute('aria-checked', String(on)); limit.disabled = on; }
      setNone(!!off);
      none.addEventListener('click', function () { setNone(none.getAttribute('aria-checked') !== 'true'); });
      f.append(node('h3', 'bus-form-title', link ? 'Trusted Link' : 'Trust a Link'),
        node('p', 'md-meta', 'The agents of these two workspaces may act on each other\'s requests without asking you. Without a trusted link the limit is ' + overview.defaultMax + '. A workspace may trust itself, so that its own chats can keep handing work to one another.'),
        field('Workspace', one.el), field('And Workspace', two.el), field('Automatic Handoffs', limit), none);
      if (link) { one.select.disabled = true; two.select.disabled = true; }
      var actions = node('div', 'hstack gap-2 bus-form-actions');
      var ok = node('button', 'btn btn-primary btn-sm', link ? 'Save' : 'Trust Link'); ok.type = 'submit';
      var cancel = node('button', 'btn btn-sm', 'Cancel'); cancel.type = 'button';
      actions.append(ok, cancel);
      if (link) {
        var remove = node('button', 'btn btn-danger btn-sm', 'Remove Link…'); remove.type = 'button';
        remove.addEventListener('click', function () {
          host().ask('Remove the Trusted Link?', 'The agents of ' + (link.aLabel || link.a) + ' and ' + (link.bLabel || link.b) + ' will check with you again before acting on each other\'s requests.', 'Remove Link', true).then(function (yes) {
            if (!yes) return;
            return Promise.resolve(app().BusUntrust(link.a, link.b)).then(function () { hideForm(); load(); });
          }).catch(host().fail);
        });
        actions.appendChild(remove);
      }
      f.appendChild(actions);
      cancel.addEventListener('click', hideForm);
      f.addEventListener('submit', function (e) {
        e.preventDefault();
        var unlimited = none.getAttribute('aria-checked') === 'true';
        var hops = unlimited ? overview.noHopLimit : parseInt(limit.value, 10);
        var asked = !unlimited || off ? Promise.resolve(true) :
          host().ask('Turn Off the Limit?', 'Two agents with no limit on handoffs can keep each other working, and spending, for as long as you leave them. Only do this for agents you will watch.', 'Turn Off the Limit', true);
        ok.disabled = true;
        asked.then(function (yes) {
          if (!yes) return;
          return Promise.resolve(app().BusTrust(one.select.value, two.select.value, hops)).then(function () { hideForm(); load(); });
        }).catch(function (err) { host().toast(message(err), 'danger', { title: 'Link not saved:' }); })
          .finally(function () { ok.disabled = false; });
      });
      forms.appendChild(f);
      (link ? limit : one.select).focus();
    }

    function draw() {
      if (disposed) return;
      window.dinahRender.enqueue(root, root, render);
    }
    var renderedSignature;
    function render() {
      var chats = knownChats();
      var signature = JSON.stringify([overview, inboxState, chats,
        overview.rooms.map(function (r) { return [window.dinahChatState.describe(r), r.lastAt ? when(r.lastAt) : '']; }),
        chats.map(function (c) { return [window.dinahChatState.describe(c), c.updatedAt ? when(c.updatedAt) : '']; }),
        inboxState.items.map(function (m) { return when(m.createdAt); })]);
      if (renderedSignature === signature) return;
      groups.replaceChildren();
      var sections = window.dinahBrowserSections, list = window.dinahBrowserList;
      function section(id, title, count) {
        var g = list.group({ id: id, title: title, count: count, open: sections ? sections.open(id) : true, onToggle: function (open) { if (sections) sections.save(id, open); } });
        groups.appendChild(g.el);
        return g;
      }
      /* The inbox: messages sent to the person, and a group's messages that
         mention them (docs/design/inbox.md). It shows what is unread unless
         the person chooses All or Dismissed, and its count is every unread
         message, however many. Each row leads with the message, then who
         sent it, where and when; opening it shows that very message, in its
         group or in a thread that starts with it, and marks that one message
         read once it is shown. Dismiss hides a row for good without marking
         it read, answered or approved; the bulk actions do the same for
         every message up to the newest the list was read with. */
      var inboxSection = section('bus-inbox', 'Inbox', overview.unread || 0);
      var tools = node('div', 'bus-inbox-tools');
      var views = node('div', 'seg sm bus-inbox-views'); views.setAttribute('role', 'group'); views.setAttribute('aria-label', 'Show');
      [['unread', 'Unread'], ['all', 'All'], ['dismissed', 'Dismissed']].forEach(function (v) {
        var b = node('button', '', v[1]); b.type = 'button'; b.setAttribute('aria-pressed', String(inboxState.view === v[0]));
        b.addEventListener('click', function () { if (inboxState.view !== v[0]) { inboxState.view = v[0]; inboxState.items = []; inboxState.nextBefore = 0; readInbox(); } });
        views.appendChild(b);
      });
      tools.appendChild(views);
      bulkActions().forEach(function (action) { tools.appendChild(inboxBulk(action)); });
      inboxSection.body.appendChild(tools);
      if (inboxState.error) {
        inboxSection.body.appendChild(list.empty({ title: 'The Inbox Could Not Be Read', body: inboxState.error, action: { label: 'Try Again', run: function () { readInbox(); } } }));
      } else if (!inboxState.items.length && !inboxState.loading) {
        var emptyWords = { unread: ['No Unread Messages', 'Messages agents send to you, and group messages that mention you, arrive here. Choose All to see the earlier ones.'],
          all: ['Nothing for You Yet', 'Messages agents send to you, and group messages that mention you, arrive here.'],
          dismissed: ['Nothing Dismissed', 'Messages you dismiss from the inbox are listed here. They stay in their groups and threads.'] }[inboxState.view];
        inboxSection.body.appendChild(list.empty({ title: emptyWords[0], body: emptyWords[1] }));
      }
      inboxState.items.forEach(function (m) {
        /* A row leads with the chat that sent it, by its title as the bus
           gives it now (BusMessage.from), then the message's first line, then
           where and when. A sender with no title left is said plainly; its
           id is only in the tooltip (docs/design/inbox.md). */
        var sender = m.fromPerson ? 'You' : (m.from || '').trim() || 'A chat that is no longer available';
        var context = [m.room ? 'In ' + (m.roomTitle || m.room) : 'To you', m.fromWorkspace && m.fromWorkspace !== m.from ? m.fromWorkspace : '', when(m.createdAt)].filter(Boolean).join(' · ');
        var row = list.row({
          title: sender, meta: [firstLine(m.text) || (m.kind === 'release' ? 'Release ' + m.version : ''), context],
          badge: m.dismissed ? { text: 'Dismissed', chip: true } : m.unread ? { text: 'New', kind: 'accent', label: 'Not read yet' } : null,
          tooltip: (m.from ? '' : 'Sender ' + (m.fromSession || 'unknown') + '\n') + m.text, data: { busMessage: String(m.id) },
          onOpen: function () { openInboxItem(m); }
        });
        row.classList.add('bus-inbox-row');
        if (!m.dismissed && app().BusDismissInbox) {
          var dismiss = node('button', 'icon-btn bus-inbox-dismiss'); dismiss.type = 'button';
          dismiss.setAttribute('aria-label', 'Dismiss the message from ' + sender); dismiss.title = 'Dismiss from the inbox';
          dismiss.innerHTML = '<span class="glyph" aria-hidden="true" style="--glyph: var(--glyph-close)"></span>';
          dismiss.addEventListener('click', function () {
            dismiss.disabled = true;
            Promise.resolve(app().BusDismissInbox(m.id)).then(function () {
              if (inboxState.view !== 'all') inboxState.items = inboxState.items.filter(function (x) { return x.id !== m.id; });
              return load();
            }).then(function () { readInbox(); }).catch(function (error) { dismiss.disabled = false; host().fail(error); });
          });
          row.appendChild(dismiss);
        }
        inboxSection.body.appendChild(row);
      });
      if (inboxState.nextBefore && !inboxState.error) {
        var older = node('button', 'btn btn-sm bus-inbox-older', inboxState.loading ? 'Loading…' : 'Load Older'); older.type = 'button'; older.disabled = inboxState.loading;
        older.addEventListener('click', function () { readInbox(true); });
        inboxSection.body.appendChild(older);
      }
      var rooms = section('bus-groups', 'Group Chats', overview.rooms.length);
      if (!overview.rooms.length) rooms.body.appendChild(list.empty({ title: 'No Group Chats', body: 'A group chat holds you and the chats you choose in one conversation.', action: { label: 'New Group Chat…', run: function () { openPicker({}).catch(host().fail); } } }));
      overview.rooms.forEach(function (r) {
        rooms.body.appendChild(list.row({ title: r.title, meta: [memberNames(r), r.unread ? r.unread + ' new messages' : '', r.lastAt ? 'Last message ' + when(r.lastAt) : 'Nothing said yet'].concat(r.unread && r.latest ? [r.latest.from + ': ' + r.latest.line] : []), data: { busRoom: r.name },
          badge: stateBadge(r),
          onOpen: function () { openRoom(r).catch(host().fail); } }));
      });
      /* Every chat the window knows, open or closed, by workspace, each with
         its state. Opening one opens its window, as the Chats pane does. */
      var byWorkspace = {};
      chats.forEach(function (c) { (byWorkspace[c.workspace] = byWorkspace[c.workspace] || []).push(c); });
      var all = section('bus-chats', 'Chats', chats.length);
      if (!chats.length) all.body.appendChild(list.empty({ title: 'No Chats Yet', body: 'The chats of the open workspaces are listed here, closed ones too.' }));
      Object.keys(byWorkspace).sort(function (a, b) { return a.localeCompare(b, undefined, { sensitivity: 'base' }); }).forEach(function (w) {
        all.body.appendChild(list.subheading(w));
        byWorkspace[w].sort(function (a, b) { return (b.updatedAt || '').localeCompare(a.updatedAt || ''); }).forEach(function (c) {
          all.body.appendChild(list.row({ title: c.title, meta: [c.provider + (c.updatedAt ? ', active ' + when(c.updatedAt) : ''), c.life === 'open' && c.window === false ? 'No window' : ''], badge: stateBadge(c, 'chat-row-state'), data: { busChat: c.id },
            onOpen: function () { Promise.resolve(window.dinahChats.open(c.id)).catch(host().fail); } }));
        });
      });
      var links = section('bus-links', 'Trusted Links', (overview.links || []).length);
      if (!(overview.links || []).length) links.body.appendChild(list.empty({ title: 'No Trusted Links', body: 'Without one, an agent checks with you before acting on another agent\'s request.', action: { label: 'Trust a Link…', run: function () { showLinkForm(null); } } }));
      (overview.links || []).forEach(function (l) {
        links.body.appendChild(list.row({ title: (l.aLabel || l.a) + ' and ' + (l.bLabel || l.b), meta: [limitText(l.hopLimit)],
          badge: l.hopLimit === overview.noHopLimit ? { text: 'No Limit', kind: 'warn', label: 'These agents can keep each other working without a limit' } : null,
          data: { busLink: l.a + '|' + l.b }, onOpen: function () { showLinkForm(l); } }));
      });
      renderedSignature = signature;
    }
    listeners.add(draw);
    // The bus changed: the inbox page is read again for the view chosen.
    function inboxChanged() { readInbox(); }
    listeners.add(inboxChanged); readInbox();
    var stopChats = onChats(draw);
    draw();
    load().catch(host().fail);
    return { state: function () { return ''; }, setState: function () {},
      commands: function () { return [{ label: 'New Group Chat…', run: function () { return openPicker({}); } }, { label: 'New Message…', run: function () { showForm(); } }, { label: 'Trust a Link…', run: function () { showLinkForm(null); } }].concat(bulkActions()); },
      destroy: function () { disposed = true; window.dinahRender.cancel(root); listeners.delete(draw); listeners.delete(inboxChanged); stopChats(); } };
  } });

  /* ---- A conversation: a group or a thread ----
     The messages are drawn as they arrive: a message already drawn is kept,
     with any selection made in it, and only the new ones are added after
     it, unless one drawn before has changed, such as a chat's title, when
     the whole conversation is drawn again. The list stays at its end while
     the reader is at it, as a chat's does, and keeps its place when the
     reader has scrolled back; whether the reader is at the end is decided
     by the reader's own scrolling, and the list's own move to the end is
     recognised by the position it left. Sending goes to the end. Quote puts
     a message in the box as a quotation, and Edit puts one of the person's
     back in the box, with the chats it mentioned. The person's messages are
     the box's history for Up and Down. */
  /* stamp, when given, says whether the messages may have changed since
     they were fetched (the count of bus changes); a redraw asked for by
     the overview's listeners or the chat list fetches the messages again
     only when it has, since every redraw of the chat list and every bus
     change loaded the overview, and every open group then fetched and
     redrew its whole list. An explicit draw always fetches. */
  function conversation(root, emptyBody, fetch, box, seenKey, stopped, stamp) {
    root.classList.add('bus-conversation');
    var lastStamp = null;
    var log = node('div', 'bus-log transcript'), list = node('div', 'bus-log-list transcript-messages');
    log.setAttribute('data-scroll-defer', '');
    log.dataset.uiScroll = 'bus-log';
    log.appendChild(list);
    root.append(log, box.el);
    var disposed = false, revision = 0, drawn = [], painting = true;
    var ledger = null, ledgerKey = seenKey && 'transcript-seen:' + seenKey;
    function readLedger() {
      if (ledger) return ledger;
      ledger = ledgerKey && window.dinahUIState.get(ledgerKey) || { through: seenKey && Number((window.dinahUIState.get('bus-seen') || {})[seenKey]) || 0, unseen: [] };
      return ledger;
    }
    var navigation = window.dinahTranscript.mount({ root: root, log: log, content: list, stopped: stopped,
      // Last Read keeps its place at the start of the toolbar and is
      // disabled while there is nowhere to go back to.
      stoppedChanged: stopped ? function (shown) { stopped.disabled = !shown; } : null,
      ready: function () { return !painting; }, canTail: function () { return !painting; },
      entries: function () { return drawn.map(function (d) { return { key: d.message && d.message.id, element: d.div, eligible: !!d.message && !d.message.fromPerson && d.message.kind !== 'member' }; }); },
      initialSeen: function (entry) { var saved = readLedger(); return Number(entry.key) <= saved.through && saved.unseen.indexOf(Number(entry.key)) < 0; },
      seenChanged: function (snapshot) {
        if (!ledgerKey) return;
        var through = readLedger().through;
        drawn.forEach(function (d) { if (d.message) through = Math.max(through, d.message.id); });
        ledger = { through: through, unseen: snapshot.unseen.map(function (e) { return Number(e.key); }) };
        window.dinahUIState.set(ledgerKey, ledger);
        if (snapshot.atEnd) {
          var old = Object.assign({}, window.dinahUIState.get('bus-seen') || {});
          if ((old[seenKey] || 0) < through) { old[seenKey] = through; window.dinahUIState.set('bus-seen', old); }
        }
      }
    });
    function act(what, view) {
      if (what === 'quote') { window.dinahChatText.quoteInto(box.text, view.text); box.changed(); return; }
      if (what === 'edit' && view.message) { box.write(box.entry(view.message)); box.text.focus(); box.text.setSelectionRange(box.text.value.length, box.text.value.length); }
    }
    window.dinahChatText.openLinks(list, function (a) {
      var el = a.closest('[data-bus-message]'), view = el && drawn.find(function (d) { return d.div === el; });
      return view && view.message ? senderFolder(view.message) : '';
    });
    // A message whose file was removed is drawn again, to show it removed.
    function signature(m) { return [m.id, m.from, m.text, JSON.stringify(m.relay || null), (m.mentions || []).map(function (x) { return x.title; }).join('|'), (m.attachments || []).map(function (a) { return a.index + (a.removed ? '-' : ''); }).join('|')].join('\u0000'); }
    function draw(end) {
      if (disposed || (box.canRead && !box.canRead())) return Promise.resolve();
      if (end === true) navigation.scroll(true);
      var requested = ++revision, stampNow = stamp ? stamp() : null;
      return Promise.resolve(fetch()).then(function (ms) {
        if (disposed || requested !== revision || (box.canRead && !box.canRead())) return;
        lastStamp = stampNow;
        ms = ms || [];
        box.history.set(ms.filter(function (m) { return m.fromPerson && m.kind !== 'member'; }).map(box.entry));
        var same = drawn.length <= ms.length && drawn.every(function (d, i) { return d.sig === signature(ms[i]); });
        if (same && drawn.length === ms.length && (ms.length || list.firstChild)) return;
        // New messages go after those drawn. A conversation drawn again is
        // built aside and put in place whole, so the list never stands
        // empty and the reader keeps their place.
        var adding = same && drawn.length > 0, index = adding ? drawn.length : 0;
        painting = true;
        var into = adding ? list : document.createDocumentFragment(), views = adding ? drawn : [];
        window.dinahRender.enqueue(draw, root, function (deadline) {
          return navigation.keep(function () {
          if (!ms.length) into.appendChild(window.dinahBrowserList.empty({ title: 'Nothing Said Yet', body: emptyBody }));
          while (index < ms.length) {
            var view = messageView(ms[index], act);
            view.sig = signature(ms[index++]);
            views.push(view);
            into.appendChild(view.div);
            if (index < ms.length && performance.now() >= deadline) return true;
          }
            if (into !== list) list.replaceChildren(into);
            drawn = views;
            return false;
          }, true);
        }, function () { painting = false; if (window.dinahUIState) window.dinahUIState.scrollReady(log); navigation.scroll(); navigation.update(); });
      }).catch(function (err) { if (!disposed) host().toast(message(err), 'danger'); });
    }
    function redraw() {
      if (stamp) { var now = stamp(); if (now !== null && now === lastStamp) return Promise.resolve(); }
      return draw();
    }
    listeners.add(redraw);
    draw();
    return { draw: draw, focus: function () { box.text.focus(); },
      navigate: navigation.navigate,
      destroy: function () { disposed = true; navigation.destroy(); if (box.dispose) box.dispose(); window.dinahRender.cancel(draw); listeners.delete(redraw); } };
  }

  /* The composer of a group chat: the chat's box with the group's @ list.
     @ opens a list of the group's members, with everyone and Add a Chat…
     after them; choosing one puts it in the box whole, as one unit that
     Backspace takes out whole, and the message is sent with the list of the
     chats it mentions (decision 2). The mentions chosen are part of what
     the box holds, so a draft, a message brought back with Up and one put
     back with Edit keep them. A group has no provider, model, Steer or Stop
     of its own, and no skills, which run in a chat's agent. */
  /* The mentions a group message's text writes out whole, typed or pasted
     without choosing them from the @ list: "@everyone", or "@" and a member
     chat's title, in any case, at the start of a word (no letter or digit
     glued before the @) and ending where a word ends. A longer title is matched before a shorter one that begins
     it, and each @ binds once, so "@Docs Pass" is Docs Pass and not a member
     titled Docs. Paul's typed "@everyone" was sent unbound on 2 November
     2026 and woke no member. */
  // A word's characters: letters, the marks that sit on them (a lowercased
  // İ ends in a combining dot), digits and the underscore.
  function referenceToken(token) {
    return new RegExp('(^|\\s)' + token.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '(?=$|[\\s.,!?;:)\\]}])', 'g');
  }
  var WORD = /[\p{L}\p{M}\p{N}_]/u;
  function typedMentions(text, members) {
    var named = members.filter(function (m) { return m.state !== 'person' && m.title; }).map(function (m) { return { id: m.id, title: m.title }; });
    named.push({ id: 'everyone', title: 'everyone' });
    named.sort(function (a, b) { return b.title.length - a.title.length; });
    var lower = text.toLowerCase(), taken = {}, found = [];
    named.forEach(function (m) {
      var word = '@' + m.title.toLowerCase();
      for (var at = lower.indexOf(word); at >= 0; at = lower.indexOf(word, at + 1)) {
        // Both edges are read in the lowercased text the match was found in,
        // since a letter can change length when lowercased (İ becomes two
        // characters). An @ starts a word unless a letter or digit is glued
        // to it, so "(@everyone" binds and "docs@everyone.example" does not.
        var end = at + word.length;
        if (taken[at] || (at > 0 && WORD.test(lower[at - 1])) || (end < lower.length && WORD.test(lower[end]))) continue;
        taken[at] = true;
        if (found.indexOf(m.id) < 0) found.push(m.id);
      }
    });
    return found;
  }

  function mentionComposer(key, members, addChat, send, limit) {
    var mentioned = [];
    // The mentions a text binds: those chosen from the list that it still
    // holds, and every one it writes out whole. Send and the reach line both
    // read them here, so what is shown and what is sent agree.
    function bound(text) {
      if (box) box.files().forEach(function (f) { if (f.token) text = text.replace(referenceToken(f.token), ' '); });
      var ids = mentioned.filter(function (x) { return text.indexOf('@' + x.title) >= 0; }).map(function (x) { return x.id; }).concat(typedMentions(text, members()));
      return ids.filter(function (id, i) { return ids.indexOf(id) === i; });
    }
    var box = composer({ label: 'Message to the Group', placeholder: 'Write to the group. Type @ and a chat\'s title to ask particular chats, or @everyone to ask them all.', draftKey: 'group:' + key, files: true, limit: limit, send: function (text, state, files) {
      return send(text, bound(text), files);
    } });
    box.extra = { read: function () { return { mentioned: mentioned.slice() }; }, write: function (state) { mentioned = (state.mentioned || []).slice(); close(); } };
    // A message of the person's puts back its text with the chats it
    // mentioned, and everyone when its text says so. The bus lists every member it
    // asked for the person's message, so only the chats the text writes with an @ count.
    box.entry = function (m) {
      var text = String(m.text || ''), list = (m.mentions || []).filter(function (x) { return text.indexOf('@' + x.title) >= 0; }).map(function (x) { return { id: x.id, title: x.title }; });
      if (/(^|\s)@everyone\b/.test(text)) list.push({ id: 'everyone', title: 'everyone' });
      return { text: text, mentioned: list };
    };
    box.sent = close;
    box.el.classList.add('group-composer');
    /* Who the message will wake (docs/plans/group-attention.md): the
       engine decides, from the mentions bound so far, and the page only
       shows its answer. The line under the box is text in a row of its own
       height, so nothing moves as it changes; the placeholder says the same
       when the box is empty. A hidden polite live region says the reach once
       it has changed and typing has paused, never on every key. Without the
       engine's call the line stays empty and the placeholder is the box's
       own. */
    var IDLE_HINT = box.text.placeholder;
    var reachLine = node('p', 'form-help group-reach'), reachSpoken = node('span', 'visually-hidden');
    reachSpoken.setAttribute('aria-live', 'polite'); reachSpoken.setAttribute('role', 'status');
    box.el.appendChild(reachLine); box.el.appendChild(reachSpoken);
    var reachAsked = 0, reachTimer = 0, expiryTimer = 0, sayTimer = 0, shown = null;
    function names(list) {
      var t = (list || []).map(function (m) { return m.title; });
      return t.length < 3 ? t.join(' and ') : t.slice(0, -1).join(', ') + ' and ' + t[t.length - 1];
    }
    // The engine's answer: reach lists the members it wakes, and everyone
    // says the message goes to every member, mentioned or by the rule.
    function reachWords(r) {
      if (!r) return '';
      // Everyone mentioned, or everyone because no one is in conversation:
      // the second says why, so the person can see the rule at work.
      if (r.everyone) return r.reason === 'everyone' ? 'Reaches everyone, since no one is in conversation' : 'Reaches everyone in the group';
      // A team channel with no one engaged goes to its lead.
      if (r.reason === 'lead') return 'Reaches the lead, ' + names(r.reach);
      return 'Reaches ' + names(r.reach);
    }
    function drawReach(r) {
      shown = r;
      var words = reachWords(r);
      if (reachLine.textContent !== words) { reachLine.textContent = words; reachLine.title = words; }
      box.text.placeholder = !r ? IDLE_HINT
        : r.everyone ? 'Type @ to choose who answers, or send to everyone'
        : r.reason === 'lead' ? 'Goes to the lead, ' + names(r.reach) + '. Type @ to choose who answers'
        : 'Reaches ' + names(r.reach) + '. Type @ to choose others';
      // The live region speaks once the reach has settled, after a pause.
      clearTimeout(sayTimer);
      sayTimer = setTimeout(function () { if (reachSpoken.textContent !== words) reachSpoken.textContent = words; }, 700);
      clearTimeout(expiryTimer);
      var at = r && r.expires ? Date.parse(r.expires) - Date.now() : NaN;
      if (isFinite(at)) expiryTimer = setTimeout(askReach, Math.max(0, at) + 50);
    }
    var reachEnded = false;
    function askReach() {
      if (reachEnded) return;
      if (!app().BusGroupReach) { drawReach(null); return; }
      // The box is built before its window puts it on the page.
      if (!box.el.isConnected) { reachSoon(); return; }
      var asked = ++reachAsked;
      Promise.resolve(app().BusGroupReach(key, bound(box.text.value))).then(function (r) {
        if (asked === reachAsked && box.el.isConnected) drawReach(r || null);
      }).catch(function () { if (asked === reachAsked) drawReach(null); });
    }
    function reachSoon() { clearTimeout(reachTimer); reachTimer = setTimeout(askReach, 250); }
    box.text.addEventListener('input', reachSoon);
    /* The box changes at every redraw of the window as well as at every
       keystroke; who a message reaches depends on its text and files, so
       Go is asked again only when those changed, and on a bus change. */
    var changedBefore = box.changed, reachFor = null;
    box.changed = function () {
      changedBefore.apply(box, arguments);
      var now = box.text.value + String.fromCharCode(0) + JSON.stringify(box.files ? box.files() : []);
      if (now !== reachFor) { reachFor = now; reachSoon(); }
    };
    var writeBefore = box.extra.write;
    box.extra.write = function (state) { writeBefore(state); reachSoon(); };
    // Any change on the bus may change who is engaged; the answer that
    // comes back last for the latest question is the one drawn.
    var offAttention = host().on('bus:changed', function () { askReach(); });
    box.reachShown = function () { return shown; };
    box.refreshReach = askReach;
    var disposeBefore = box.dispose;
    box.dispose = function () { reachEnded = true; clearTimeout(reachTimer); clearTimeout(expiryTimer); clearTimeout(sayTimer); if (offAttention) offAttention(); if (disposeBefore) disposeBefore(); };
    askReach();
    var pop = box.pop;
    pop.classList.add('group-mention-pop'); pop.setAttribute('aria-label', 'Members');
    var text = box.text, choices = [], chosen = 0, start = -1, scope = 'members', catalog = null;
    text.setAttribute('aria-autocomplete', 'list'); text.setAttribute('aria-controls', pop.id);
    function close() { pop.hidden = true; start = -1; scope = 'members'; text.removeAttribute('aria-expanded'); }
    /* The @ the caret follows and what has been typed after it. Titles have
       spaces, so the search runs to the caret; an @ that begins a mention
       already chosen is finished, and the words typed after it are the
       message, not a search. */
    function query() {
      var before = text.value.slice(0, text.selectionStart), m = /(^|\s)@([^@\n]{0,40})$/.exec(before);
      if (!m) return null;
      var at = before.length - m[2].length - 1;
      var done = mentioned.some(function (x) { return text.value.slice(at + 1, at + 1 + x.title.length) === x.title; });
      done = done || box.files().some(function (f) { return f.token && text.value.slice(at, at + f.token.length) === f.token; });
      return done ? null : { at: at, q: m[2].toLowerCase() };
    }
    function drawPop() {
      var q = query();
      if (!q) { close(); return; }
      start = q.at;
      choices = members().filter(function (m) { return m.state !== 'person' && m.title.toLowerCase().indexOf(q.q) >= 0; })
        .map(function (m) { var state = memberState(m); return { id: m.id, title: m.title, meta: [m.workspace, state.life === 'closed' ? 'closed, opens in the background for the mention' : ''].filter(Boolean).join(', '), state: state }; });
      if ('everyone'.indexOf(q.q) === 0) choices.push({ id: 'everyone', title: 'everyone', meta: 'Every member chat, opening closed ones in the background' });
      // Typing that matches no member is not a mention, so the list closes
      // rather than offering Add a Chat… alone under every word.
      if (scope !== 'members') {
        choices = catalog.items(q.q, scope).map(function (c) { return Object.assign({}, c, { title: c.label }); });
      } else if (!choices.length && q.q) { close(); return; }
      if (scope === 'members') choices.push({ add: true, title: 'Add a Chat…', meta: 'Put another chat in the group' });
      chosen = Math.min(chosen, choices.length - 1);
      var items = choices.map(function (c) { return { group: c.group || 'Members', label: c.title, meta: c.meta, badge: c.state && !c.att ? stateBadge(c.state) : null, key: c.add ? 'add' : c.ins || c.id }; });
      window.dinahMentionList.draw(pop, items, chosen, pick, { hint: catalog && scope !== 'members' && catalog.failed() ? 'Applet lookup failed. Reopen Add to retry.' : 'Enter or Tab adds · Up and Down choose · Esc closes' });
      text.setAttribute('aria-expanded', 'true');
    }
    function pick(i) {
      var c = choices[i];
      if (!c) return;
      if (c.add) { text.value = text.value.slice(0, start) + text.value.slice(text.selectionStart); close(); box.changed(); addChat(); return; }
      var word = (c.ins || '@' + c.title) + ' ', end = text.selectionStart;
      text.value = text.value.slice(0, start) + word + text.value.slice(end);
      var caret = start + word.length;
      text.setSelectionRange(caret, caret);
      if (c.att) box.addReference(Object.assign({}, c.att, { token: c.ins }));
      else if (c.id !== 'everyone') mentioned.push({ id: c.id, title: c.title });
      else mentioned.push({ id: 'everyone', title: 'everyone' });
      close();
      box.changed();
      text.focus();
    }
    box.addMention = function (kind) {
      if (reachEnded) return;
      catalog = catalog || window.dinahChatReferences.catalog();
      var before = text.value.slice(0, text.selectionStart), after = text.value.slice(text.selectionEnd);
      var insert = (before && !/\s$/.test(before) ? ' ' : '') + '@';
      text.value = before + insert + after;
      text.setSelectionRange(before.length + insert.length, before.length + insert.length);
      scope = kind; chosen = 0; box.changed(); text.focus(); drawPop();
      catalog.load().then(function () { if (!reachEnded && scope === kind) drawPop(); });
    };
    /* The mention the caret is in or at the end of, as a range. */
    function mentionAt(pos, forward) {
      var found = null;
      mentioned.forEach(function (x) {
        var word = '@' + x.title, i = text.value.indexOf(word);
        while (i >= 0 && !found) {
          var end = i + word.length;
          if (forward ? pos >= i && pos < end : pos > i && pos <= end) found = { start: i, end: end };
          i = text.value.indexOf(word, i + 1);
        }
      });
      return found;
    }
    text.addEventListener('input', function () { chosen = 0; drawPop(); });
    text.addEventListener('blur', function () { setTimeout(function () { if (document.activeElement !== text && !pop.contains(document.activeElement)) close(); }, 150); });
    // Registered for the capturing phase, so that while the list is open
    // its keys choose in it and never send or bring back a message.
    text.addEventListener('keydown', function (e) {
      // Backspace and Delete while composing edit the composition, not a mention.
      if (window.dinahHelpers.composing(e)) return;
      if (!pop.hidden) {
        var action = window.dinahMentionList.key(e);
        if (action === 'down' || action === 'up') { chosen = window.dinahMentionList.step(chosen, choices.length, action); drawPop(); }
        else if (action === 'pick') pick(chosen);
        else if (action === 'close') close();
        if (action) { e.preventDefault(); e.stopImmediatePropagation(); return; }
      }
      if ((e.key === 'Backspace' || e.key === 'Delete') && text.selectionStart === text.selectionEnd) {
        var r = mentionAt(text.selectionStart, e.key === 'Delete');
        if (r) {
          e.preventDefault();
          text.value = text.value.slice(0, r.start) + text.value.slice(r.end);
          text.setSelectionRange(r.start, r.start);
          box.changed();
        }
      }
    }, true);
    /* A mention added from outside the box: at its end, or with atCaret where
       the caret was last, replacing what was selected there as typing would,
       with a space either side only where none is, and the caret after it.
       Either way the chat is bound as a typed mention is (mentioned). */
    box.mention = function (m, atCaret) {
      if (!m) return;
      var word = '@' + m.title;
      if (atCaret) {
        var start = text.selectionStart, end = text.selectionEnd, before = text.value.slice(0, start), after = text.value.slice(end);
        var insert = (before && !/\s$/.test(before) ? ' ' : '') + word + (!after || !/^\s/.test(after) ? ' ' : '');
        text.value = before + insert + after;
        var caret = before.length + insert.length;
        text.setSelectionRange(caret, caret);
      } else if (text.value.indexOf(word) < 0) text.value += (text.value && !/\s$/.test(text.value) ? ' ' : '') + word + ' ';
      if (!mentioned.some(function (x) { return x.id === m.id; })) mentioned.push({ id: m.id, title: m.title });
      box.changed();
    };
    box.restore();
    return box;
  }

  /* ---- A group chat's window ---- */
  var groupWindows = new Map();
  function alternativeDraftKey(name) { return 'dinah-group-draft-alternatives:' + name; }
  var GROUP_IDENTITIES = 'dinah-group-canonical-names';
  function rememberGroup(windowKey, name) {
    var known = Object.assign({}, window.dinahUIState.get(GROUP_IDENTITIES, {}) || {});
    known[windowKey] = name; window.dinahUIState.set(GROUP_IDENTITIES, known);
  }
  function moveGroupDraft(address, name, occupied) {
    if (address === name || !window.dinahUIState) return;
    var source = 'dinah-draft:group:' + address, target = 'dinah-draft:group:' + name;
    var draft = window.dinahUIState.get(source), current = window.dinahUIState.get(target);
    if (!draft || !draft.text) return;
    if (!current && !occupied) window.dinahUIState.set(target, draft);
    else if (JSON.stringify(current) !== JSON.stringify(draft)) {
      var saved = (window.dinahUIState.get(alternativeDraftKey(name), []) || []).slice();
      if (!saved.some(function (entry) { return JSON.stringify(entry.draft) === JSON.stringify(draft); })) saved.push({ draft: draft });
      window.dinahUIState.set(alternativeDraftKey(name), saved);
    }
    window.dinahUIState.set(source, undefined);
  }
  function initGroupWindow(root, opts, item) {
    var inner = null, disposed = false, epoch = 0, state = opts.state;
    var win = root.closest('[data-win]'), windowKey = win.dataset.win;
    var pending = node('section', 'group-lookup'), words = node('p', 'form-help', 'Loading the group chat…');
    words.setAttribute('role', 'status');
    var retry = node('button', 'btn btn-sm', 'Retry Group Lookup'); retry.type = 'button'; retry.hidden = true;
    pending.append(words, retry); root.appendChild(pending);
    var controller = {
      state: function () { return inner ? inner.state() : state; },
      setState: function (value) { state = value; if (inner) inner.setState(value); },
      focus: function () { if (inner) inner.focus(); },
      commands: function () { return inner ? inner.commands() : [{ label: 'Retry Group Lookup', run: begin }]; },
      hasDraft: function () { return !!(inner && inner.hasDraft()); },
      refreshDrafts: function () { if (inner) inner.refreshDrafts(); },
      destroy: function () { disposed = true; epoch++; groupWindows.delete(windowKey); if (inner) inner.destroy(); }
    };
    groupWindows.set(windowKey, controller);
    function fail(error) {
      if (!pending.isConnected) root.replaceChildren(pending);
      words.textContent = 'The group lookup failed. ' + message(error); retry.hidden = false;
    }
    function begin() {
      var requested = ++epoch;
      words.textContent = 'Loading the group chat…'; retry.hidden = true;
      load().catch(function () {});
      return Promise.all([window.dinahUIState ? window.dinahUIState.ready : Promise.resolve(), resolveGroup(item.path)]).then(function (values) {
        var result = values[1];
        if (disposed || requested !== epoch) return;
        if (result.state !== 'resolved') { fail(new Error(lookupWords(result.state))); return; }
        var expected = (window.dinahUIState.get(GROUP_IDENTITIES, {}) || {})[windowKey];
        if (expected && result.room.name !== expected) throw new Error('The original group chat is unavailable. The lookup matched another group.');
        var room = result.room, canonical = { kind: 'bus-room', path: room.name, title: room.title };
        var canonicalKey = window.dinahWorkbench.key(canonical), other = groupWindows.get(canonicalKey);
        rememberGroup(canonicalKey, room.name);
        moveGroupDraft(item.path, room.name, !!(other && other !== controller && other.hasDraft()));
        if (canonicalKey !== windowKey && window.pudlWindows.state().open.includes(canonicalKey)) {
          if (other) other.refreshDrafts();
          if (state !== undefined) window.dinahWorkbench.setState(canonicalKey, state);
          var foreground = window.pudlWindows.state().top === windowKey;
          window.pudlWindows.close(windowKey);
          if (foreground) window.pudlWindows.raise(canonicalKey);
          return;
        }
        if (canonicalKey !== windowKey) {
          var oldKey = windowKey;
          windowKey = window.dinahWorkbench.rekey(windowKey, canonical);
          groupWindows.delete(oldKey); groupWindows.set(windowKey, controller);
        }
        root.replaceChildren();
        inner = initGroupRoom(root, Object.assign({}, opts, { key: windowKey, state: state }), canonical, room);
        if (window.pudlCommands) window.pudlCommands.changed();
      }).catch(function (error) { if (!disposed && requested === epoch) fail(error); });
    }
    retry.addEventListener('click', begin);
    begin();
    return controller;
  }
  function initGroupRoom(root, opts, item, initialRoom) {
    var key = item.path, room = initialRoom, disposed = false;
    var lookupState = 'resolved', lookupEpoch = 0, checkedOverview = overviewVersion;
    var lookupNote = node('p', 'form-help group-lookup-status'); lookupNote.setAttribute('role', 'status'); lookupNote.hidden = true;
    var lookupRetry = node('button', 'btn btn-sm', 'Retry Group Lookup'); lookupRetry.type = 'button'; lookupRetry.hidden = true;
    /* A window opened at a message (state.message) shows and highlights it,
       loading the group from a little before it when it is older than the
       newest messages, and keeps loading from there so updates do not take
       it away. Opened from the Inbox (state.inbox), the window does not mark
       the group's messages read as it reads them: the Inbox marks the one
       message it opened, once it is shown (docs/design/inbox.md). */
    var targetMessage = 0, pendingMention = null, targetFrame = 0, targetPending = false, fromInbox = false, loadFrom = 0;
    function stateString() { return JSON.stringify(Object.assign({ message: targetMessage }, fromInbox ? { inbox: true } : {}, pendingMention ? { mention: pendingMention } : {})); }
    function showTarget() {
      if (disposed || !targetMessage || !targetPending) return;
      var row = root.querySelector('[data-bus-message="' + CSS.escape(String(targetMessage)) + '"]');
      if (!row) return;
      root.querySelectorAll('.bus-target-message').forEach(function (n) { n.classList.remove('bus-target-message'); });
      targetPending = false; row.classList.add('bus-target-message'); c.navigate(row);
      document.dispatchEvent(new CustomEvent('dinah:bus-message-shown', { detail: { message: targetMessage, group: key } }));
    }
    function targetSoon() {
      if (targetFrame || !targetMessage || !targetPending) return;
      targetFrame = requestAnimationFrame(function () { targetFrame = 0; showTarget(); });
    }
    var targetObserver = new MutationObserver(targetSoon);
    targetObserver.observe(root, { childList: true, subtree: true });
    function applyMention() {
      if (!pendingMention || !room) return;
      var m = (room.members || []).find(function (x) { return x.chat === pendingMention.chat || x.id === pendingMention.id; });
      if (!m) return;
      box.mention(m); pendingMention = null;
    }
    function setState(value) {
      try { var state = value ? JSON.parse(value) : {}; targetMessage = Number(state.message) || 0; pendingMention = state.mention || null; targetPending = !!targetMessage; fromInbox = !!(state.inbox && targetMessage); }
      catch (error) { targetMessage = 0; pendingMention = null; fromInbox = false; }
      applyMention(); targetSoon();
      if (targetPending && c && !root.querySelector('[data-bus-message="' + CSS.escape(String(targetMessage)) + '"]')) c.draw();
      if (opts.changed) opts.changed(stateString());
    }
    root.classList.add('group-window');
    function title() { return room ? room.title : key; }
    function members() { return room ? room.members || [] : []; }
    function addChats() { return openPicker({ group: key }).catch(host().fail); }
    /* The limit on the files in the person's message is the smallest among
       the member chats that have one, worked out in Go with the member that
       set it (file_limits.go, groupFileLimit). */
    function fileLimit() { var l = room && room.fileLimit; return l && l.limit > 0 ? { max: l.limit, reason: l.reason } : null; }
    var box = mentionComposer(key, members, addChats, function (text, mentions, files) {
      var shown = box.reachShown();
      return Promise.resolve(app().BusSendToGroup(key, text, mentions, files || [])).then(function (sent) {
        return Promise.resolve(c.draw(true)).then(function () { noteReach(sent, shown); });
      });
    }, fileLimit);
    /* Who the message actually reached is the engine's, stored with the
       message (group-attention.md). When it differs from what the line said
       as it was sent, as when an engagement ran out in between, one line
       under the sent message says so. */
    function sameReach(a, b) {
      if (!a || !b) return true;
      var ids = function (r) { return (r.reach || []).map(function (m) { return m.id; }).sort().join('|'); };
      return !!a.everyone === !!b.everyone && (a.everyone || ids(a) === ids(b));
    }
    // The conversation draws its rows in a later frame, so the note waits a
    // few frames for its message's row.
    function noteReach(sent, shown, tries) {
      var actual = sent && sent.reach;
      if (!actual || !shown || sameReach(actual, shown)) return;
      var row = root.querySelector('[data-bus-message="' + CSS.escape(String(sent.id)) + '"]');
      if (!row) { if ((tries || 0) < 30 && root.isConnected) requestAnimationFrame(function () { noteReach(sent, shown, (tries || 0) + 1); }); return; }
      if (row.querySelector(':scope > .group-reach-note')) return;
      var words = actual.everyone ? 'Sent to everyone in the group' : 'Sent to ' + (actual.reach || []).map(function (m) { return m.title; }).join(', ');
      row.appendChild(node('p', 'form-help group-reach-note', words + ', not as shown: who was engaged changed before it was sent.'));
    }
    box.canRead = function () { return lookupState === 'resolved' && ((!overviewFailure && !!group(key)) || checkedOverview === overviewVersion); };
    var stopped = node('button', 'btn btn-sm group-jump-stopped'); stopped.type = 'button'; stopped.title = 'Go to where you stopped reading'; stopped.disabled = true;
    window.dinahHelpers.toolbarLabel(stopped, 'history', 'Last Read');
    var c = conversation(root, 'Everything said in the group appears here. The chats a message mentions are asked to answer it, and the other members are told that it was said.',
      function () {
        function newest() { return Promise.resolve(app().BusGroupMessages(key)); }
        function from(id) { return Promise.resolve(app().BusGroupMessagesFrom(key, id, 20)); }
        var read = loadFrom && app().BusGroupMessagesFrom ? from(loadFrom) : newest().then(function (ms) {
          var wanted = targetPending && targetMessage;
          if (!wanted || !app().BusGroupMessagesFrom || (ms || []).some(function (m) { return m.id === wanted; })) return ms;
          return from(wanted).then(function (older) { loadFrom = wanted; return older; });
        });
        return read.then(function (ms) { if (!fromInbox) markRead(); return ms; });
      }, box, 'group:' + key, stopped, function () { return String(busChanges); });
    var draftChoices = node('div', 'group-draft-choices'), draftLabel = node('label', 'form-field');
    var draftSelect = node('select', 'form-select'); draftSelect.setAttribute('aria-label', 'Other saved drafts');
    draftLabel.append(node('span', 'form-label', 'Other saved drafts'), draftSelect);
    var useDraft = node('button', 'btn btn-sm', 'Use Selected Draft'); useDraft.type = 'button';
    draftChoices.append(node('p', 'form-help', 'Another saved window contained a different draft. Choose which one to edit.'), draftLabel, useDraft);
    box.content.prepend(draftChoices);
    var draftsShown = '';
    function refreshDrafts() {
      var saved = window.dinahUIState.get(alternativeDraftKey(key), []) || [];
      var current = window.dinahUIState.get('dinah-draft:group:' + key);
      if (!box.text.value && !box.files().length && current && current.text) box.write(current);
      /* Asked at every keystroke and every redraw; the choices are rebuilt
         only when they would differ. */
      var shown = JSON.stringify([saved, lookupState, box.files().length]);
      if (shown === draftsShown) return;
      draftsShown = shown;
      draftChoices.hidden = !saved.length;
      draftSelect.replaceChildren();
      saved.forEach(function (entry, i) {
        var preview = String(entry.draft.text || '').split('\n')[0].slice(0, 70);
        var option = node('option', '', 'Saved draft ' + (i + 1) + ': ' + preview); option.value = String(i); draftSelect.appendChild(option);
      });
      useDraft.disabled = lookupState !== 'resolved' || !!box.files().length;
      useDraft.title = box.files().length ? 'Remove attached files before switching drafts.' : '';
    }
    useDraft.addEventListener('click', function () {
      var saved = (window.dinahUIState.get(alternativeDraftKey(key), []) || []).slice();
      var index = Number(draftSelect.value), selected = saved[index];
      if (!selected || lookupState !== 'resolved' || box.files().length) return;
      var current = box.history.draft(box.read());
      if (current && current.text && JSON.stringify(current) !== JSON.stringify(selected.draft)) saved[index] = { draft: current };
      else saved.splice(index, 1);
      window.dinahUIState.set(alternativeDraftKey(key), saved.length ? saved : undefined);
      box.history.push(null); box.write(selected.draft); refreshDrafts(); box.text.focus();
    });
    box.text.addEventListener('input', refreshDrafts);
    var draftAttachments = new MutationObserver(refreshDrafts);
    draftAttachments.observe(box.el.querySelector('.group-attachments'), { childList: true });
    refreshDrafts();
    // Files dropped anywhere on the group's window go with the next message.
    box.dropOn(root);
    /* The members are the Members dropdown in the toolbar, the one a lead's
       chat has too (teams.js): a row for each chat, its name and standard
       state chip, which opens the chat, with a … button for its menu; then
       the person; then the turn budget when the group has one. Rows are kept
       and updated in place, so a state change leaves an open dropdown, its
       focus and the composer's caret as they are. */
    var memberShell = node('section', 'group-member-shell');
    var bar = node('nav', 'menu-panel members-menu'); bar.setAttribute('popover', ''); bar.setAttribute('aria-label', 'Members');
    bar.id = 'group-members-' + (++groupSerial);
    var memberList = node('div', 'member-list'); memberList.setAttribute('role', 'list'); memberList.setAttribute('aria-label', 'Members');
    bar.appendChild(memberList);
    var memberToggle = node('button', 'btn btn-sm menu-btn group-members-toggle'); memberToggle.type = 'button';
    var memberWords = window.dinahHelpers.toolbarLabel(memberToggle, 'account', 'Members');
    memberToggle.id = bar.id + '-button'; memberToggle.setAttribute('popovertarget', bar.id);
    var memberMenu = node('div', 'menu'); memberMenu.append(memberToggle, bar);
    var commandBar = node('div', 'toolbar group-toolbar'); commandBar.setAttribute('role', 'toolbar'); commandBar.setAttribute('aria-label', 'Group commands');
    // A folding PUDL toolbar, as every chat's is: each control has its glyph.
    // Last Read stays longest, then Members, then the group's commands.
    commandBar.setAttribute('data-toolbar-fold', ''); commandBar.setAttribute('data-toolbar-captions', 'none');
    function toolbarGroup(name, glyph, priority) {
      var g = node('div', 'toolbar-group'); g.setAttribute('role', 'group'); g.setAttribute('aria-label', name);
      g.setAttribute('data-toolbar-group', name); g.setAttribute('data-toolbar-glyph', glyph); g.setAttribute('data-toolbar-priority', String(priority));
      return g;
    }
    var readingGroup = toolbarGroup('Reading', 'history', 30), membersGroup = toolbarGroup('Members', 'account', 20), commandGroup = toolbarGroup('Group', 'comments', 10);
    function command(label, glyph, cls, run) {
      var button = node('button', 'btn btn-sm ' + cls); button.type = 'button'; button.setAttribute('aria-label', label);
      var icon = node('span', 'glyph'); icon.style.setProperty('--glyph', 'var(--glyph-' + glyph + ')'); icon.setAttribute('aria-hidden', 'true');
      if (glyph) button.appendChild(icon); button.appendChild(node('span', 'toolbar-words', label)); if (run) button.addEventListener('click', run); commandGroup.appendChild(button);
      return button;
    }
    // Last Read comes first in every chat's toolbar.
    readingGroup.appendChild(stopped);
    membersGroup.appendChild(memberMenu);
    command('Add Chats…', 'plus', 'group-add-chats', addChats);
    command('Turn Budget…', 'list-numbered', 'group-turn-budget', openBudget);
    command('Mention Everyone', 'comments', 'group-mention-everyone', function () { box.mention({ id: 'everyone', title: 'everyone' }); box.text.focus(); });
    /* A team's group, the one a lead's team speaks in, has the team's
       report and its allowances; no other group shows them. The lead is
       known by its session on the bus (room.lead), which is the id of one
       of the group's members, whose chat is the lead's. */
    function teamLead() {
      var lead = room && room.lead && (room.members || []).filter(function (m) { return m.id === room.lead; })[0];
      return lead && lead.chat || '';
    }
    var teamReport = command('Team Report', 'board', 'group-team-report', null), teamAllowances = null;
    teamReport.hidden = true;
    if (window.dinahTeams && window.dinahTeams.reportPanel) {
      var reportPanel = window.dinahTeams.reportPanel(teamLead, function (chat) { return window.dinahChats.open(chat); });
      root.appendChild(reportPanel);
      teamReport.setAttribute('popovertarget', reportPanel.id);
      if (!teamReport.id) teamReport.id = reportPanel.id + '-button';
      /* The report stands against its button. When the toolbar has folded
         that button away, PUDL places it against the fold or More button
         instead (data-menu-anchor, 0.96.9). */
      reportPanel.dataset.menuAnchor = teamReport.id;
      teamAllowances = command('Team Allowances…', 'gear', 'group-team-allowances', function () {
        var lead = teamLead();
        if (lead) window.dinahTeams.editAllowances(lead).catch(host().fail);
      });
      teamAllowances.hidden = true;
    }
    commandBar.append(readingGroup, membersGroup, commandGroup); memberShell.appendChild(commandBar);
    var separator = node('hr', 'group-transcript-separator');
    root.prepend(lookupNote, lookupRetry, memberShell, separator);
    if (window.pudlToolbar) window.pudlToolbar.enhance(memberShell);
    /* A member's menu is inside the Members dropdown, so opening it leaves
       the dropdown open beneath it, and Escape closes only the menu. */
    var menu = node('nav', 'menu-panel group-member-menu'); menu.setAttribute('popover', ''); menu.setAttribute('role', 'menu'); menu.setAttribute('aria-label', 'Member');
    menu.id = 'group-member-menu-' + Math.random().toString(36).slice(2);
    bar.appendChild(menu);
    var memberViews = new Map(), memberSerial = 0, menuMember = null, menuState = null;
    menu.addEventListener('beforetoggle', function (event) {
      if (event.newState === 'closed' && menu.contains(document.activeElement)) {
        var view = memberViews.get(menuMember);
        (view && view.more && bar.matches(':popover-open') ? view.more : memberToggle).focus({ preventScroll: true });
      }
    });
    function memberDescription(m) {
      var value = memberState(m), spec = window.dinahChatState.describe(value), presence = window.dinahChatState.read(value);
      return spec.label + (presence.life === 'open' && presence.window === false && !presence.elsewhere ? '. No window' : '');
    }
    function markRead() { if (room && room.unread && app().BusMarkGroupRead) Promise.resolve(app().BusMarkGroupRead(key)).catch(function () {}); }
    /* Renaming a group changes its title alone: its members, messages,
       budget and unread marks stay, and the conversation keeps a line saying
       who did it and to what, the engine's part. The rules are a chat
       rename's: Enter sets it, Esc leaves it as it was, and an empty name
       keeps the one it has (bus_views.go, BusRenameGroup). A send that names
       the old title is refused with the new one, as the engine says. */
    function renameGroup() {
      if (!app().BusRenameGroup) return;
      /* The window's own menu may answer before the pane's first load has the
         group; the title then stands at the group's name. */
      window.dinahResources.askName({
        title: 'Rename the Group', label: 'Name', value: title(), yes: 'Rename',
        commit: function (name) {
          return Promise.resolve(app().BusRenameGroup(key, name)).then(load);
        }
      }).catch(host().fail);
    }
    function showMenu(button, m) {
      menu.replaceChildren();
      menuMember = m.id; menuState = node('p', 'menu-note member-menu-state', memberDescription(m)); menu.appendChild(menuState);
      menuState.title = menuState.textContent; menuState.setAttribute('aria-label', menuState.textContent);
      var open = node('button', 'menu-action', 'Open Chat'); open.type = 'button'; open.setAttribute('role', 'menuitem');
      if (!m.chat) { open.setAttribute('aria-disabled', 'true'); open.title = 'This chat runs outside the window.'; }
      open.addEventListener('click', function () { menu.hidePopover(); if (bar.matches(':popover-open')) bar.hidePopover(); if (m.chat) Promise.resolve(window.dinahChats.open(m.chat)).catch(host().fail); });
      // Address Message mentions the chat where the composer's caret was,
      // bound to it as a typed mention is, and goes back to the composer.
      var address = node('button', 'menu-action', 'Address Message'); address.type = 'button'; address.setAttribute('role', 'menuitem');
      address.title = 'Mention ' + m.title + ' where your message\'s caret is';
      address.addEventListener('click', function () {
        menu.hidePopover(); if (bar.matches(':popover-open')) bar.hidePopover();
        box.mention({ id: m.id, title: m.title }, true); box.text.focus();
      });
      var remove = node('button', 'menu-action danger', 'Remove From Group…'); remove.type = 'button'; remove.setAttribute('role', 'menuitem');
      remove.addEventListener('click', function () {
        menu.hidePopover(); if (bar.matches(':popover-open')) bar.hidePopover();
        host().ask('Remove ' + m.title + ' From the Group?', m.title + ' stops receiving the group\'s messages. What was said stays in the group.', 'Remove', true).then(function (yes) {
          if (!yes) return;
          return Promise.resolve(app().BusRemoveFromGroup(key, m.id)).then(load);
        }).catch(host().fail);
      });
      menu.append(open, address, remove);
      menu.dataset.menuAnchor = button.id;
      if (menu.matches(':popover-open')) menu.hidePopover();
      menu.showPopover();
    }
    function drawMembers() {
      if (disposed) return;
      var listed = !overviewFailure && group(key);
      if (listed) { room = listed; lookupState = 'resolved'; checkedOverview = overviewVersion; lookupEpoch++; lookupNote.hidden = true; lookupRetry.hidden = true; }
      else if (loaded && checkedOverview !== overviewVersion) checkAvailability();
      box.close(lookupState !== 'resolved');
      box.limitChanged();
      refreshDrafts();
      commandBar.hidden = lookupState !== 'resolved';
      var isTeam = !!teamLead();
      if (teamReport.hidden === isTeam) teamReport.hidden = !isTeam;
      if (teamAllowances && teamAllowances.hidden === isTeam) teamAllowances.hidden = !isTeam;
      var win = root.closest('[data-win]');
      if (room && win && window.pudlWindows) {
        var t = win.querySelector('.win-title');
        if (t && t.textContent.trim() !== room.title) {
          window.dinahWorkbench.retitle(win.dataset.win, room.title);
          /* The menu bar's own copy of the window's title does not follow a
             retitle on its own today; until PUDL draws that link itself, we
             tell the bar. */
          if (window.pudlMenubar && window.pudlMenubar.refresh) window.pudlMenubar.refresh();
        }
      }
      if (win) {
        var statuses = win.querySelector('.win-statuses');
        if (statuses) {
          var stateChip = statuses.querySelector('.group-state-chip');
          if (!stateChip) { stateChip = node('span', 'win-status group-state-chip'); statuses.appendChild(stateChip); }
          stateChip.replaceChildren(window.dinahChatState.badge(room || {}));
        }
      }
      window.dinahRender.enqueue(drawMembers, root, function () {
        /* The dropdown lists the group's chats. The person is a member too,
           and stays one for routing, but has no row: it is plain who they
           are, and a row of theirs would have nothing to do. */
        var chats = members().filter(function (m) { return m.state !== 'person'; });
        var count = 'Members (' + chats.length + ')';
        if (memberWords.textContent !== count) memberWords.textContent = count;
        // A member's menu closes with its row.
        if (menuMember && menu.matches(':popover-open') && !chats.some(function (m) { return m.id === menuMember; })) menu.hidePopover();
        window.dinahHelpers.reconcile(memberList, chats, { rows: memberViews, key: function (m) { return m.id; }, lost: function () { memberToggle.focus(); }, create: function (m) {
            var row = node('div', 'member-row'); row.setAttribute('role', 'listitem'); row.dataset.groupMember = m.id;
            var open = node('button', 'menu-action member-open'); open.type = 'button';
            var name = node('span', 'member-name'), slot = node('span', 'member-state-slot');
            // The chat glyph the taskbar draws for a chat window, before the name.
            var mark = node('span', 'glyph member-glyph'); mark.setAttribute('aria-hidden', 'true'); mark.style.setProperty('--glyph', 'var(--glyph-comments)');
            open.append(mark, name, slot);
            var more = glyphButton('member-menu-btn', 'more', m.title + ' Menu');
            more.id = menu.id + '-' + (++memberSerial); more.setAttribute('aria-haspopup', 'menu');
            var view = { node: row, row: row, open: open, name: name, slot: slot, more: more, member: m };
            /* The row acts for the member it was made for, by its id, read as
               it is when pressed; a state change only redraws its words. */
            view.open.addEventListener('click', function () {
              var current = view.member;
              if (bar.matches(':popover-open')) bar.hidePopover();
              if (current.chat) Promise.resolve(window.dinahChats.open(current.chat)).catch(host().fail);
            });
            view.more.addEventListener('click', function () { showMenu(view.more, view.member); });
            row.append(open, more);
            return view;
          }, update: function (view, m) {
          view.member = m;
          if (view.name.textContent !== m.title) view.name.textContent = m.title;
          view.more.setAttribute('aria-label', m.title + ' Menu');
          view.more.disabled = lookupState !== 'resolved';
          var description = memberDescription(m);
          view.open.setAttribute('aria-label', 'Open ' + m.title + ', ' + description);
          view.open.title = m.chat ? 'Open ' + m.title + '. ' + description : 'This chat runs outside the window. ' + description;
          view.open.setAttribute('aria-disabled', String(!m.chat));
          view.slot.replaceChildren(window.dinahChatState.badge(memberState(m), { labelled: true, className: 'member-state-chip' }));
          view.row.classList.toggle('member-ended', !window.dinahChatState.describe(memberState(m)).glyph);
          if (menuMember === m.id && menuState) { menuState.textContent = description; menuState.title = menuState.textContent; menuState.setAttribute('aria-label', menuState.textContent); }
        } });
        var oldBudget = memberShell.querySelector(':scope > .group-budget');
        // The turn budget is a line under the toolbar, only when the group has one.
        if (room && typeof room.budget === 'number' && room.budget >= 0) {
          var remaining = room.budget - (room.used || 0), spent = remaining <= 0;
          var words = spent ? 'Turn budget spent (' + room.budget + '). Your next message starts it again.'
            : remaining + (remaining === 1 ? ' turn' : ' turns') + ' left before the group waits for you';
          var left = oldBudget || node('p', 'form-help group-budget');
          if (left.textContent !== words) left.textContent = words;
          left.classList.toggle('warn', spent); left.dataset.groupBudget = spent ? 'spent' : 'open';
          if (!oldBudget) memberShell.appendChild(left);
        } else if (oldBudget) oldBudget.remove();
      });
    }
    function checkAvailability() {
      var requested = ++lookupEpoch;
      checkedOverview = overviewVersion; lookupState = 'loading';
      lookupNote.hidden = false; lookupNote.textContent = 'Checking whether the group chat is available…'; lookupRetry.hidden = true;
      box.close(true); commandBar.hidden = true;
      root.querySelectorAll('.member-menu-btn').forEach(function (button) { button.disabled = true; });
      if (menu.matches(':popover-open')) menu.hidePopover();
      return resolveGroup(key).then(function (result) {
        if (disposed || requested !== lookupEpoch) return;
        if (result.state === 'resolved' && result.room.name !== key) throw new Error('The group lookup returned a different group identity.');
        lookupState = result.state;
        if (result.state === 'resolved') {
          room = result.room; lookupNote.hidden = !overviewFailure;
          if (overviewFailure) lookupNote.textContent = 'The group list could not refresh. This group was checked separately.';
          lookupRetry.hidden = true; drawMembers(); c.draw();
        } else {
          lookupNote.textContent = lookupWords(result.state); lookupRetry.hidden = false;
          box.close(true); commandBar.hidden = true;
        }
      }).catch(function (error) {
        if (disposed || requested !== lookupEpoch) return;
        lookupState = 'error'; lookupNote.hidden = false; lookupNote.textContent = 'The group lookup failed. ' + message(error); lookupRetry.hidden = false;
        box.close(true); commandBar.hidden = true;
      });
    }
    lookupRetry.addEventListener('click', checkAvailability);
    var loaded = false;
    function refresh() { loaded = true; drawMembers(); applyMention(); }
    listeners.add(refresh);
    // A member chat's state in the window changes its chip.
    var stopChats = onChats(drawMembers);
    // A provider's limit set in Settings changes the group's limit, which
    // the group is read again for.
    function limitsChanged() { load().catch(function () {}); }
    document.addEventListener('dinah:file-limits-changed', limitsChanged);
    drawMembers();
    load().then(applyMention).catch(function () {});
    if (opts.state) setState(opts.state);
    /* The group's turn budget: how many turns the chats in it may start for
       themselves before their mentions wait for the person's next message.
       Unlimited, the default, is the first choice. Saving also gives the
       group its turns back. */
    function openBudget() {
      var current = room && typeof room.budget === 'number' ? room.budget : -1;
      var dialog = node('dialog', 'dialog group-budget-dialog'), form = node('form', 'vstack gap-3'); form.noValidate = true;
      var id = 'group-budget-title-' + Math.random().toString(36).slice(2);
      dialog.setAttribute('aria-labelledby', id);
      var head = node('h2', 'dialog-title', 'Turn Budget'); head.id = id;
      var help = node('p', 'form-help', 'A budget limits how many turns the chats in this group can start for themselves. Each time a chat is handed something another chat in the group asked of it, that is one turn. When the budget is spent, mentions between the chats wait until you post. Unlimited, the default, sets no limit.');
      // PUDL's switch: a button whose aria-checked says whether it is on.
      var none = node('button', 'switch'); none.type = 'button'; none.setAttribute('role', 'switch');
      var track = node('span', 'switch-track'); track.appendChild(node('span', 'switch-thumb'));
      none.append(track, document.createTextNode('Unlimited'));
      var count = node('input', 'form-input'); count.type = 'number'; count.min = '0'; count.value = String(current >= 0 ? current : 10);
      count.setAttribute('aria-label', 'Turns');
      var label = node('label', 'form-field'); label.append(node('span', 'form-label', 'Turns'), count);
      function setNone(on) { none.setAttribute('aria-checked', String(on)); count.disabled = on; }
      setNone(current < 0);
      none.addEventListener('click', function () { setNone(none.getAttribute('aria-checked') !== 'true'); });
      var error = node('p', 'form-error'); error.setAttribute('role', 'alert'); error.hidden = true;
      var actions = node('div', 'dialog-actions');
      var save = node('button', 'btn btn-primary', 'Save'); save.type = 'submit';
      var cancel = node('button', 'btn', 'Cancel'); cancel.type = 'button';
      actions.append(cancel, save);
      form.append(none, label, error, actions);
      dialog.append(head, help, form);
      cancel.addEventListener('click', function () { dialog.close(); });
      dialog.addEventListener('close', function () { dialog.remove(); });
      form.addEventListener('submit', function (e) {
        e.preventDefault();
        var unlimited = none.getAttribute('aria-checked') === 'true', n = unlimited ? -1 : parseInt(count.value, 10);
        if (!unlimited && !(n >= 0)) { error.textContent = 'Enter a number of turns, 0 or more.'; error.hidden = false; return; }
        save.disabled = true;
        Promise.resolve(app().BusGroupBudget(key, n)).then(function () { dialog.close(); return load(); }).catch(function (err) {
          save.disabled = false; error.textContent = String(err && err.message || err).replace(/^Error:\s*/, ''); error.hidden = false;
        });
      });
      document.body.appendChild(dialog);
      dialog.showModal();
      (current < 0 ? none : count).focus();
    }
    function deleteGroup() {
      return host().ask('Delete ' + title() + '?', 'The group goes, and nobody can write to it again. What was said in it stays in the inbox and threads.', 'Delete Group', true).then(function (yes) {
        if (!yes) return;
        return Promise.resolve(app().BusDeleteGroup(key)).then(function () {
          box.forget();
          window.dinahUIState.set(alternativeDraftKey(key), undefined);
          var win = root.closest('[data-win]');
          if (win && window.pudlWindows) window.pudlWindows.close(win.dataset.win);
          load();
        });
      }).catch(host().fail);
    }
    return { state: stateString, setState: setState, hasDraft: function () { return !!box.text.value || !!box.files().length; }, refreshDrafts: refreshDrafts, destroy: function () { disposed = true; lookupEpoch++; draftAttachments.disconnect(); targetObserver.disconnect(); if (targetFrame) cancelAnimationFrame(targetFrame); window.dinahRender.cancel(drawMembers); listeners.delete(refresh); stopChats(); document.removeEventListener('dinah:file-limits-changed', limitsChanged); c.destroy(); menu.remove(); },
      focus: function () { box.text.focus(); },
      // The person is always a member, so a group has no Leave (decision 8).
      // The Add menu keeps references available at the upload limit.
      commands: function () { if (lookupState !== 'resolved') return [{ label: 'Retry Group Lookup', run: checkAvailability }]; return [{ label: 'Make This Group a Team…', hidden: !!(room && room.lead) || !app().MakeGroupATeam, run: function () { return window.dinahTeams.convertGroup(room).then(load); } }, { label: 'Add Chats…', run: addChats }, { label: 'Turn Budget…', run: openBudget }, { label: 'Rename Group…', run: renameGroup }, { label: 'Add', glyph: 'clip', items: dinahComposer.addItems({ full: function () { return !!box.attachReason(); }, fullReason: box.attachReason, files: box.attach, browse: box.browse, mention: box.addMention }) },{ label: 'Refresh', run: function () { c.draw(); load(); } }, { label: 'Delete Group…', danger: true, run: deleteGroup }]; } };
  }

  window.dinahWorkbench.register('bus-room', { applet: { id: 'dinah.groups', title: 'Group Chats', glyph: 'account', workspace: false }, title: 'Group Chat', glyph: 'comments', statuses: true, init: initGroupWindow });

  /* ---- The picker of a group chat ----
     Every chat the window knows, by workspace, the workspace of the chat it
     was opened from first, each with a check box, its provider, its state
     and when it was last active. The filter narrows the list. A chat with
     no title is given one in place before it can be ticked, since two rows
     reading "Untitled Chat in studio" cannot be told apart (decision 7). A
     new group is named, or takes its chats' titles as its name. The ticked
     chats and the name are the window's state, so its address keeps them. */
  window.dinahWorkbench.register('group-picker', { pinnable: false, applet: { id: 'dinah.groups', title: 'Group Chats', glyph: 'account', workspace: false }, title: 'Start a Group Chat', glyph: 'comments', init: function (root, opts, item) {
    var spec = {};
    try { spec = JSON.parse(item.path || '{}') || {}; } catch (err) { spec = {}; }
    var adding = spec.group || '', from = spec.from || '', chosen = {}, name = '', filter = '', titling = '', disposed = false;
    if (from) chosen[from] = true;
    root.classList.add('group-picker');
    root.innerHTML = '';
    var find = node('input', 'form-input picker-filter'); find.type = 'search'; find.placeholder = 'Filter by Title, Workspace or Provider';
    find.setAttribute('aria-label', 'Filter the Chats'); find.autocomplete = 'off';
    var heading = node('h2', 'visually-hidden', adding ? 'Add Chats' : 'Start a Group Chat'); heading.id = 'picker-title-' + Math.random().toString(36).slice(2);
    /* The list is PUDL's rich list box (pudl-listbox.js, 0.97.0): PUDL gives
       it its roles, keys, type-ahead and check marks and does the choosing;
       Dinah writes a section for each workspace and keeps each row by its
       chat, filling it in place, so the row PUDL has active stays active
       as the list is drawn again. */
    var list = node('div', 'listbox picker-list'); list.setAttribute('data-listbox', 'multiple');
    list.setAttribute('aria-labelledby', heading.id);
    var rowsById = new Map(), sectionsByWs = new Map();
    var count = node('p', 'form-help picker-count'); count.setAttribute('role', 'status');
    var nameGroup = node('div', 'form-group picker-name-group');
    var nameLabel = node('label', 'form-label', 'Group Name'), nameInput = node('input', 'form-input picker-name');
    nameInput.id = heading.id + '-name'; nameLabel.htmlFor = nameInput.id; nameInput.autocomplete = 'off';
    nameGroup.append(nameLabel, nameInput, node('p', 'form-help', 'Leave it empty and the group takes its name from its chats\' titles.'));
    var actions = node('div', 'dialog-actions picker-actions');
    var cancel = node('button', 'btn', 'Cancel'); cancel.type = 'button';
    var go = node('button', 'btn btn-primary', adding ? 'Add' : 'Start'); go.type = 'button';
    actions.append(cancel, go);
    var listPane = node('section', 'picker-list-pane'), createPane = node('section', 'picker-create-pane'); createPane.hidden = true;
    var tools = node('div', 'hstack gap-2 picker-tools'), newChat = node('button', 'btn', 'New Chat\u2026'); newChat.type = 'button'; newChat.hidden = !adding;
    tools.append(find, newChat); listPane.append(heading, tools, list, count);
    if (!adding) listPane.appendChild(nameGroup);
    listPane.appendChild(actions); root.append(listPane, createPane);
    var creation = null, draft = null, created = {}, selectionNote = '', lastCreated = '';
    function pickerChats() { var known = knownChats(); Object.keys(created).forEach(function (id) { if (!known.some(function (c) { return c.id === id; })) known.push(created[id]); }); return known; }
    function showList() { if (creation) creation.destroy(); creation = null; draft = null; listPane.hidden = false; createPane.hidden = true; changed(); draw(); }
    function showNewChat(values) {
      listPane.hidden = true; createPane.hidden = false; draft = values || {};
      if (creation) creation.destroy();
      creation = window.dinahGroupNewChat.mount(createPane, { values: draft,
        changed: function (value) { draft = value; changed(); },
        cancel: function () { showList(); newChat.focus(); },
        created: function (ref) {
          created[ref.id] = ref; chosen[ref.id] = true; lastCreated = ref.id;
          var q = filter.trim().toLowerCase(), hidden = q && (ref.title + ' ' + ref.workspace + ' ' + ref.provider).toLowerCase().indexOf(q) < 0;
          if (hidden) { filter = ''; find.value = ''; }
          selectionNote = 'Chat created and selected. Press Add to add it to this group.' + (hidden ? ' The filter was cleared so the new chat is visible.' : '');
          showList();
          var row = list.querySelector('[data-chat="' + CSS.escape(ref.id) + '"]');
          // The keyboard goes to the new chat's row (pudlListbox.activate, 0.97.1).
          if (row && window.pudlListbox) { window.pudlListbox.activate(list, ref.id); list.focus(); }
        }
      });
      changed();
    }
    newChat.addEventListener('click', function () { showNewChat({}); });

    function changed() { root.dispatchEvent(new CustomEvent('pudl:applet-change', { bubbles: true, detail: { state: stateString() } })); }
    function stateString() {
      var ids = Object.keys(chosen).filter(function (k) { return chosen[k]; });
      return ids.length || name || filter || creation ? JSON.stringify({ chats: ids, name: name, filter: filter, step: creation ? 'new-chat' : 'list', draft: creation ? draft : undefined }) : null;
    }
    function setState(s) {
      if (!s) return;
      try { var v = JSON.parse(s); chosen = {}; (v.chats || []).forEach(function (id) { chosen[id] = true; }); name = v.name || ''; nameInput.value = name; filter = v.filter || ''; find.value = filter; if (adding && v.step === 'new-chat') showNewChat(v.draft || {}); } catch (err) { /* A state from elsewhere is ignored. */ }
      draw();
    }
    function inGroup() {
      var r = adding ? group(adding) : null;
      return r ? (r.members || []).map(function (m) { return m.chat; }).filter(Boolean) : [];
    }
    function chats() {
      var all = pickerChats(), home = (all.find(function (c) { return c.id === from; }) || {}).workspace;
      var byWs = {};
      all.forEach(function (c) { (byWs[c.workspace] = byWs[c.workspace] || []).push(c); });
      return Object.keys(byWs).sort(function (a, b) { return (b === home) - (a === home) || a.localeCompare(b, undefined, { sensitivity: 'base' }); })
        .map(function (w) { return { workspace: w, chats: byWs[w].sort(function (a, b) { return (b.updatedAt || '').localeCompare(a.updatedAt || ''); }) }; });
    }
    // Puts children in a parent in the order wanted, moving only those out
    // of place, and takes out the rest.
    function arrange(parent, wanted) {
      wanted.forEach(function (el, i) { if (parent.children[i] !== el) parent.insertBefore(el, parent.children[i] || null); });
      Array.prototype.slice.call(parent.children, wanted.length).forEach(function (el) { el.remove(); });
    }
    /* Fills a row in place: its choice, whether it can be chosen, whether
       the filter shows it, and its words. The check mark PUDL draws stays
       its first child. */
    function fill(row, c, on, already, q) {
      row.setAttribute('aria-selected', String(on));
      row.classList.toggle('is-chosen', on);
      row.toggleAttribute('data-disabled', already);
      row.hidden = !!q && (c.title + ' ' + c.workspace + ' ' + c.provider).toLowerCase().indexOf(q) < 0;
      var mark = row.querySelector(':scope > input[aria-hidden="true"]');
      if (!mark) { mark = node('input'); mark.type = 'checkbox'; mark.tabIndex = -1; mark.setAttribute('aria-hidden', 'true'); }
      var title = node('span', 'listbox-title');
      if (titling === c.id) {
        var give = node('input', 'form-input picker-title-input'); give.placeholder = 'Give This Chat a Title'; give.setAttribute('aria-label', 'Title for ' + c.title);
        give.addEventListener('click', function (e) { e.stopPropagation(); });
        give.addEventListener('keydown', function (e) {
          e.stopPropagation();
          if (e.key === 'Escape') { e.preventDefault(); titling = ''; draw(); list.focus(); }
          if (e.key !== 'Enter') return;
          e.preventDefault();
          var t = give.value.trim();
          if (!t) return;
          give.disabled = true;
          Promise.resolve(window.dinahChats.rename(c.id, t)).then(function () { titling = ''; chosen[c.id] = true; changed(); draw(); list.focus(); })
            .catch(function (err) { give.disabled = false; host().toast(message(err), 'danger', { title: 'Not titled:' }); });
        });
        title.appendChild(give);
        setTimeout(function () { if (give.isConnected) give.focus(); }, 0);
      } else {
        title.appendChild(document.createTextNode(c.title));
        if (c.id === from) title.appendChild(node('span', 'chip', 'This Chat'));
        if (already) title.appendChild(node('span', 'chip', 'In the Group'));
      }
      var provider = node('span', 'col col-provider', c.provider), state = node('span', 'col state');
      state.appendChild(stateBadge(c));
      if (c.life === 'open' && c.window === false && !c.elsewhere) provider.appendChild(node('span', 'md-meta', ' · No window'));
      var parts = [mark, title, provider, state, node('span', 'col col-active', c.updatedAt ? when(c.updatedAt) : '')];
      if (c.untitled && titling !== c.id) parts.push(node('p', 'row-note', 'Give this chat a title before it joins a group. Choose it to title it here.'));
      else if (c.life === 'closed' && on && !already) parts.push(node('p', 'row-note', 'This chat is closed. A message that mentions it opens it in the background; the group\'s other messages wait for it until it opens.'));
      row.replaceChildren.apply(row, parts);
    }
    function draw() {
      if (disposed) return;
      var taken = inGroup(), q = filter.trim().toLowerCase();
      var groups = chats(), known = {}, sections = [], seen = new Set();
      groups.forEach(function (w) {
        var section = sectionsByWs.get(w.workspace);
        if (!section) {
          section = node('div', 'listbox-section');
          section.appendChild(node('div', 'listbox-group', w.workspace));
          sectionsByWs.set(w.workspace, section);
        }
        var rows = [section.firstChild];
        w.chats.forEach(function (c) {
          known[c.id] = c; seen.add(c.id);
          var already = taken.indexOf(c.id) >= 0, on = already || !!chosen[c.id];
          var row = rowsById.get(c.id);
          if (!row) {
            row = node('div', 'listbox-row picker-row'); row.dataset.value = c.id; row.dataset.chat = c.id;
            rowsById.set(c.id, row);
          }
          fill(row, c, on, already, q);
          rows.push(row);
        });
        arrange(section, rows);
        sections.push(section);
      });
      rowsById.forEach(function (row, id) { if (!seen.has(id)) rowsById.delete(id); });
      sectionsByWs.forEach(function (s, w) { if (sections.indexOf(s) < 0) sectionsByWs.delete(w); });
      var empty = list.querySelector(':scope > .empty-state');
      if (!groups.length && !empty) empty = window.dinahBrowserList.empty({ title: 'No Chats Yet', body: 'Open a workspace and start a chat first.' });
      arrange(list, groups.length ? sections : [empty]);
      // The opening chat is ticked; an untitled one asks for its title.
      if (from && known[from] && known[from].untitled && chosen[from] && !titling) { titling = from; chosen[from] = false; draw(); return; }
      summarise(known);
    }
    // The count, the name's placeholder and Start or Add, from the choice.
    function summarise(known) {
      if (!known) { known = {}; pickerChats().forEach(function (c) { known[c.id] = c; }); }
      var taken = inGroup(), titles = Object.keys(chosen).filter(function (id) { return chosen[id] && known[id] && !known[id].untitled && taken.indexOf(id) < 0; }).map(function (id) { return known[id]; });
      titles.sort(function (a, b) { return (b.id === from) - (a.id === from); });
      var names = titles.map(function (c) { return c.title; }), n = names.length;
      nameInput.placeholder = n === 0 ? 'Choose at least one chat' : n === 1 ? names[0] : names.slice(0, -1).join(', ') + ' and ' + names[n - 1];
      count.textContent = adding ? (n === 0 ? 'Choose the chats to add.' : 'Adds ' + (n === 1 ? 'one chat' : n + ' chats') + ' to the group.')
        : n === 0 ? 'Choose at least one chat. You are always in the group.' : 'The group will have ' + (n === 1 ? 'one chat' : n + ' chats') + ' and you.';
      if (selectionNote) count.textContent = selectionNote;
      go.disabled = n === 0;
    }
    /* PUDL did the choosing; the window keeps what was chosen. A chat with
       no title is never kept chosen: choosing it opens its title field in
       its row instead, and it is chosen once it has a title. */
    list.addEventListener('pudl:listbox-change', function (e) {
      var taken = inGroup(), all = pickerChats();
      chosen = {};
      (e.detail.values || []).forEach(function (id) {
        if (taken.indexOf(id) >= 0) return;
        var c = all.find(function (x) { return x.id === id; });
        if (c && c.untitled) { if (!titling) titling = id; return; }
        chosen[id] = true;
      });
      selectionNote = '';
      // The rows are filled again in place, for a chosen chat's note, and
      // keep their places, so the row PUDL has active stays active.
      changed(); draw();
    });
    find.addEventListener('input', function () { filter = find.value; selectionNote = ''; changed(); draw(); });
    nameInput.addEventListener('input', function () { name = nameInput.value; changed(); });
    function closeWindow() { var win = root.closest('[data-win]'); if (win && window.pudlWindows) window.pudlWindows.close(win.dataset.win); }
    cancel.addEventListener('click', closeWindow);
    go.addEventListener('click', function () {
      var taken = inGroup();
      var refs = pickerChats().filter(function (c) { return chosen[c.id] && !c.untitled && taken.indexOf(c.id) < 0; })
        .map(function (c) { return { id: c.id, cwd: c.cwd, title: c.title }; });
      refs.sort(function (a, b) { return (b.id === from) - (a.id === from); });
      if (!refs.length) return;
      go.disabled = true;
      var done = adding ? Promise.resolve(app().BusAddToGroup(adding, refs)).then(function () { return group(adding) || { name: adding, title: adding }; })
        : Promise.resolve(app().BusStartGroup(nameInput.value.trim(), refs));
      done.then(function (r) {
        load();
        closeWindow();
        return openRoom(r);
      }).catch(function (err) { go.disabled = false; host().toast(message(err), 'danger', { title: adding ? 'Not added:' : 'No group made:' }); });
    });
    function redraw() { if (!titling) draw(); }
    listeners.add(redraw);
    var stopChats = onChats(redraw);
    draw();
    if (window.pudlListbox) window.pudlListbox.enhance(list);
    load().catch(function () {});
    return { state: stateString, setState: setState, focus: function () { if (creation) { creation.focus(); return; } var selected = selectionNote && lastCreated && list.querySelector('[data-chat="' + CSS.escape(lastCreated) + '"][aria-selected="true"]'); if (selected && window.pudlListbox) { window.pudlListbox.activate(list, lastCreated); list.focus(); } else find.focus(); },
      destroy: function () { disposed = true; if (creation) creation.destroy(); listeners.delete(redraw); stopChats(); } };
  } });

  /* A picker opens wide and tall enough for a chat's title, provider,
     state and last activity on one line, unless the person placed one
     before (workbench.js keeps that placement and gives it first). */
  document.addEventListener('pudl:window-place', function (e) {
    var item = window.dinahWorkbench.item(e.detail.key);
    if (e.detail.placement || !item || item.kind !== 'group-picker') return;
    e.detail.placement = { mode: 'floating', x: 0.1, y: 0.04, w: 0.8, h: 0.9 };
  }, true);

  window.dinahWorkbench.register('bus-thread', { applet: { id: 'dinah.groups', title: 'Group Chats', glyph: 'account', workspace: false }, title: 'Thread', glyph: 'inbox', init: function (root, opts, item) {
    var id = Number(item.path);
    var box = composer({ label: 'Reply', draftKey: 'thread:' + id, send: function (text) { return Promise.resolve(app().BusReplyInThread(id, text)).then(function () { c.draw(true); }); } });
    box.restore();
    var c = conversation(root, 'The message and its replies appear here.', function () { return app().BusThread(id); }, box);
    /* The thread starts with its message, which it highlights once drawn,
       saying so for the Inbox, which marks it read then (docs/design/inbox.md). */
    var shown = false;
    var watch = new MutationObserver(function () {
      var row = !shown && root.querySelector('[data-bus-message="' + CSS.escape(String(id)) + '"]');
      if (!row) return;
      shown = true; watch.disconnect();
      row.classList.add('bus-target-message'); c.navigate(row);
      document.dispatchEvent(new CustomEvent('dinah:bus-message-shown', { detail: { message: id, thread: true } }));
    });
    watch.observe(root, { childList: true, subtree: true });
    return { destroy: function () { watch.disconnect(); c.destroy(); }, commands: function () { return [{ label: 'Refresh', run: c.draw }]; } };
  } });

  window.dinahCommands.register({ id: 'bus', glyph: 'inbox', menu: 'view', mnemonic: 'R', label: 'Group Chats', run: function () { return window.dinahWorkbench.open('bus'); } });

  /* The pane opens once by itself, as a tab behind the one in front of the
     right-hand dock, so a layout saved before it existed finds it; a person
     who closes it has chosen to, and it is not opened again. */
  var OFFERED = 'dinah-bus-offered', offered = false;
  document.addEventListener('dinah:ui-state-restored', offer);
  window.addEventListener('load', offer);
  function offer() {
    var ui = window.dinahUIState, windows = window.pudlWindows;
    if (offered || !ui || ui.isRestoring() || !windows || document.readyState !== 'complete') return;
    offered = true;
    if (ui.get(OFFERED)) return;
    ui.set(OFFERED, true);
    /* "This was offered" is a fact that must survive the fastest close, not
       window layout that may lose its last few hundred milliseconds: through
       the debounced write a fast close lost it and the offer came back over
       the person's windows (the ui-state stress runs, 30 October 2026), so
       the flag is flushed at once. A failed write meets a page on its way
       out, and is not reported. */
    ui.flush().catch(function () {});
    var before = windows.state();
    if (before.open.indexOf('bus') >= 0) return;
    var front = before.docks && before.docks.right && before.docks.right.selected, top = before.top;
    window.dinahWorkbench.open('bus').then(function () {
      if (front) windows.raise(front);
      if (top && top !== front) windows.raise(top);
    }).catch(function () {});
  }

  /* What a chat window's Actions menu offers (app.js): Start a Group
     Chat… opens the picker with the chat ticked, and Add to a Group Chat…
     is a submenu of the groups the chat is not in, with New Group Chat… at
     its foot (decision 1). Choosing a group puts a titled chat in it at
     once; an untitled one goes to the picker, which asks for its title. */
  function startGroup(chatId) { return openPicker({ from: chatId }); }
  function addChatToGroup(key, chatId) {
    var ref = window.dinahChats && window.dinahChats.ref(chatId), r = group(key);
    if (!ref || ref.untitled) return openPicker({ group: key, from: chatId });
    return Promise.resolve(app().BusAddToGroup(key, [{ id: ref.id, cwd: ref.cwd, title: ref.title }])).then(function () {
      host().toast('Added ' + ref.title + ' to ' + (r ? r.title : key) + '.', 'positive');
      return load();
    }).catch(function (err) { host().toast(message(err), 'danger', { title: 'Not added:' }); });
  }
  function addItems(chatId) {
    var out = (overview.rooms || []).filter(function (r) { return !(r.members || []).some(function (m) { return m.chat === chatId; }); })
      .map(function (r) { return { id: 'add-to-group-' + r.name, label: r.title, run: function () { return addChatToGroup(r.name, chatId); } }; });
    if (!out.length) out.push({ id: 'add-to-group-none', label: 'No Other Group Chats', disabled: true, reason: 'This chat is in every group chat, or there are none yet.' });
    return out.concat(['-', { id: 'add-to-group-new', label: 'New Group Chat…', run: function () { return startGroup(chatId); } }]);
  }

  document.addEventListener('DOMContentLoaded', function () {
    host().on('bus:changed', function () { load().catch(function () {}); });
    host().onResync(function () { return load(); });
  });
  window.dinahBus = { load: load, overview: function () { return overview; }, chat: chat, startGroup: startGroup, addItems: addItems, addChatToGroup: addChatToGroup, openGroup: function (key, target) {
    return Promise.all([window.dinahUIState ? window.dinahUIState.ready : Promise.resolve(), resolveGroup(key)]).then(function (values) {
      var result = values[1];
      if (result.state !== 'resolved') throw new Error(lookupWords(result.state));
      var room = result.room, item = { kind: 'bus-room', path: room.name, title: room.title };
      var windowKey = window.dinahWorkbench.key(item), existing = groupWindows.get(windowKey);
      rememberGroup(windowKey, room.name);
      moveGroupDraft(key, room.name, !!(existing && existing.hasDraft()));
      return window.dinahWorkbench.open(item, target ? JSON.stringify(target) : undefined);
    }).then(function (windowKey) {
      var controller = groupWindows.get(windowKey); if (controller) controller.refreshDrafts();
      return windowKey;
    });
  } };
})();
