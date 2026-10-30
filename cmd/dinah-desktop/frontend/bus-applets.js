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
  function when(stamp) {
    var d = new Date(stamp);
    return isNaN(d) ? stamp : dinahRegional.format(d, 'dayTime');
  }
  function firstLine(text) { var line = String(text || '').split('\n').find(function (l) { return l.trim(); }) || ''; return line.length > 140 ? line.slice(0, 139) + '…' : line; }
  function glyphButton(cls, glyph, label) {
    var b = node('button', 'icon-btn ' + cls); b.type = 'button';
    b.setAttribute('aria-label', label); b.title = label;
    b.innerHTML = '<span class="glyph" aria-hidden="true"></span>';
    b.firstChild.style.setProperty('--glyph', 'var(--glyph-' + glyph + ')');
    return b;
  }

  var overview = { inbox: [], unread: 0, rooms: [], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }, loading = null, listeners = new Set();
  function load() {
    if (!app() || !app().BusOverview) return Promise.resolve(overview);
    if (loading) return loading;
    loading = Promise.resolve(app().BusOverview()).then(function (o) {
      overview = o || overview;
      listeners.forEach(function (fn) { fn(); });
      // A chat's cards show its title as it is now (app.js, deliveredSender).
      document.dispatchEvent(new CustomEvent('dinah:bus-chats'));
      if (window.pudlCommands) window.pudlCommands.changed();
      return overview;
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

  /* A chat's state is a glyph and a word: working is the accent badge with
     the ring, open the positive badge with the circle, open in the
     background the positive badge with PUDL's eye-off glyph, since the chat
     is open but has no window, and closed the plain badge with Dinah's
     small circle (dinah.css, bus-applets.css). */
  var STATES = {
    working: { text: 'Working', kind: 'accent', cls: 'running', label: 'The agent is working' },
    open: { text: 'Open', kind: 'positive', label: 'Open in a window' },
    background: { text: 'Open in Background', kind: 'positive', cls: 'background', label: 'Open without a window, for a message that woke it; it closes itself after 30 minutes idle' },
    closed: { text: 'Closed', kind: '', label: 'Closed; a message that wakes it opens it in the background, and other messages wait for it until it opens' }
  };
  /* A group member's state: a chat in the window as the window has it,
     which changes before the bus does, and any other as the bus has it. */
  function memberState(m) {
    var ref = m.chat && window.dinahChats && window.dinahChats.ref(m.chat);
    return ref ? ref.state : m.state;
  }
  function stateBadge(state, cls) {
    var s = STATES[state] || STATES.closed;
    return { text: s.text, kind: s.kind, cls: [s.cls, cls].filter(Boolean).join(' '), label: s.label };
  }
  function stateGlyph(state) {
    var s = STATES[state] || STATES.closed, g = node('span', 'state-glyph ' + (STATES[state] ? state : 'closed'));
    g.setAttribute('role', 'img'); g.setAttribute('aria-label', s.text); g.title = s.label;
    return g;
  }
  /* The members of a group as a sentence: its chats' titles, and you. */
  function memberNames(r) {
    var t = (r.members || []).filter(function (m) { return m.state !== 'person'; }).map(function (m) { return m.title; }).concat(['you']);
    return t.length === 1 ? t[0] : t.slice(0, -1).join(', ') + ' and ' + t[t.length - 1];
  }

  function openRoom(r) { return window.dinahWorkbench.open({ kind: 'bus-room', path: r.name, title: r.title || r.name }); }
  function openThread(m) { return window.dinahWorkbench.open({ kind: 'bus-thread', path: String(m.replyTo || m.id), title: 'Thread With ' + (m.fromPerson ? m.to : m.from) }); }
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
     dropped on the window (box.dropOn) or chosen with Attach…, shown as the
     chat's chips with Remove (composer.js, attachments), and sent with the
     message as o.send's third argument. A message may be files alone. The
     files are not part of the draft, the history or Edit, as a chat's
     pasted files are not kept across a restart: their contents are held
     only in memory. o.limit() is the limit on the files in one message
     (composer.js, attachments); at the limit Attach… is disabled, with the
     reason as its tip, and box.attachReason() is that reason for the
     window's menu. */
  function composer(o) {
    var form = node('form', 'bus-composer');
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
      form.appendChild(notice);
      host().toast(reason, 'danger', { title: 'Not attached:' });
    }
    if (o.files) {
      var list = node('ul', 'attachments group-attachments'); list.setAttribute('aria-label', 'Attached to the next message'); list.hidden = true;
      var input = node('input'); input.type = 'file'; input.multiple = true; input.hidden = true;
      chips = window.dinahComposer.attachments({ host: list, get: function () { return files; }, set: function (l) { files = l; },
        changed: function () { changed(); }, toast: attachmentFailed,
        limit: o.limit, alive: function () { return form.isConnected; } });
      input.addEventListener('change', function () { chips.add(input.files); input.value = ''; text.focus(); });
      var attach = node('button', 'btn group-attach', 'Attach…'); attach.type = 'button'; attach.title = 'Attach files from this computer';
      attach.addEventListener('click', function () { box.attach(); });
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
      // The limit changed, such as when a member joined or a provider's
      // limit was set: the chips' line and Attach… follow it.
      box.limitChanged = function () { chips.draw(); changed(); };
    }
    form.appendChild(parts.el);
    box.read = function () { return Object.assign({ text: text.value }, box.extra.read()); };
    box.write = function (state) { state = state || {}; text.value = state.text || ''; box.extra.write(state); changed(); };
    // What a message of the person's puts back in the box: its text, and
    // whatever the window adds from the message.
    box.entry = function (m) { return { text: String(m.text || '') }; };
    box.history = window.dinahComposer.history(box.read, box.write);
    var kept = null;
    function changed() {
      window.dinahComposer.fit(text);
      go.disabled = closed || sending || (!text.value.trim() && !files.length);
      var attach = chips && form.querySelector('.group-attach');
      if (attach) {
        var reason = chips.fullReason();
        attach.disabled = closed || !!reason;
        attach.title = reason || 'Attach files from this computer';
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
      var state = box.read(), sent = chips ? chips.files() : [];
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
  window.dinahWorkbench.register('bus', { singleton: true, title: 'Group Chats', glyph: 'inbox', mode: 'dock-right', init: function (root) {
    root.classList.add('bus-browser');
    var disposed = false;
    var bar = window.dinahBrowserList.toolbar({
      primary: { label: 'New Group Chat…', title: 'Start a group chat with chats you choose', run: function () { openPicker({}).catch(host().fail); } },
      actions: [{ label: 'Trust a Link…', run: function () { showLinkForm(null); } }, { label: 'Mark All Read', run: markRead }, { label: 'Refresh', run: refresh }]
    });
    // New Message… stands beside New Group Chat…, before the Actions menu.
    var newMessage = node('button', 'btn btn-sm', 'New Message…'); newMessage.type = 'button';
    newMessage.title = 'Write to one chat, a workspace or a group chat';
    newMessage.addEventListener('click', function () { showForm(); });
    var commandsEl = bar.el.querySelector('.browser-commands');
    commandsEl.insertBefore(newMessage, commandsEl.querySelector('.browser-actions'));
    var forms = node('div', 'bus-forms'), groups = node('div', 'bus-results');
    root.append(bar.el, forms, groups);

    function refresh() { load().catch(host().fail); draw(); }
    function markRead() { return Promise.resolve(app().MarkBusInboxRead()).then(load).catch(host().fail); }
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
    function render() {
      groups.replaceChildren();
      var sections = window.dinahBrowserSections, list = window.dinahBrowserList;
      function section(id, title, count) {
        var g = list.group({ id: id, title: title, count: count, open: sections ? sections.open(id) : true, onToggle: function (open) { if (sections) sections.save(id, open); } });
        groups.appendChild(g.el);
        return g;
      }
      /* The inbox: messages sent to Paul himself, and a group's messages
         that mention him. A group's other messages are in the group. */
      var inbox = section('bus-inbox', 'Inbox', overview.unread || overview.inbox.length);
      if (!overview.inbox.length) inbox.body.appendChild(list.empty({ title: 'Nothing for You Yet', body: 'Messages agents send to you, and group messages that mention you, arrive here.' }));
      overview.inbox.forEach(function (m) {
        inbox.body.appendChild(list.row({
          title: m.from + (m.fromWorkspace && m.fromWorkspace !== m.from ? ' (' + m.fromWorkspace + ')' : '') + (m.room ? ' in ' + (m.roomTitle || m.room) : ''), meta: [firstLine(m.text) || (m.kind === 'release' ? 'Release ' + m.version : ''), when(m.createdAt)],
          badge: m.unread ? { text: 'New', kind: 'accent', label: 'Not read yet' } : null, tooltip: m.text, data: { busMessage: String(m.id) },
          onOpen: function () { openThread(m).catch(host().fail); }
        }));
      });
      var rooms = section('bus-groups', 'Group Chats', overview.rooms.length);
      if (!overview.rooms.length) rooms.body.appendChild(list.empty({ title: 'No Group Chats', body: 'A group chat holds you and the chats you choose in one conversation.', action: { label: 'New Group Chat…', run: function () { openPicker({}).catch(host().fail); } } }));
      overview.rooms.forEach(function (r) {
        rooms.body.appendChild(list.row({ title: r.title, meta: [memberNames(r), r.lastAt ? 'Last message ' + when(r.lastAt) : 'Nothing said yet'].concat(r.unread && r.latest ? [r.latest.from + ': ' + r.latest.line] : []), data: { busRoom: r.name },
          badge: r.unread ? { text: r.unread + ' New', kind: 'accent', label: r.unread === 1 ? '1 message not read yet' : r.unread + ' messages not read yet' } : null,
          onOpen: function () { openRoom(r).catch(host().fail); } }));
      });
      /* Every chat the window knows, open or closed, by workspace, each with
         its state. Opening one opens its window, as the Chats pane does. */
      var chats = knownChats();
      var byWorkspace = {};
      chats.forEach(function (c) { (byWorkspace[c.workspace] = byWorkspace[c.workspace] || []).push(c); });
      var all = section('bus-chats', 'Chats', chats.length);
      if (!chats.length) all.body.appendChild(list.empty({ title: 'No Chats Yet', body: 'The chats of the open workspaces are listed here, closed ones too.' }));
      Object.keys(byWorkspace).sort(function (a, b) { return a.localeCompare(b, undefined, { sensitivity: 'base' }); }).forEach(function (w) {
        all.body.appendChild(list.subheading(w));
        byWorkspace[w].sort(function (a, b) { return (b.updatedAt || '').localeCompare(a.updatedAt || ''); }).forEach(function (c) {
          all.body.appendChild(list.row({ title: c.title, meta: [c.provider + (c.updatedAt ? ', active ' + when(c.updatedAt) : '')], badge: stateBadge(c.state, 'chat-row-state'), data: { busChat: c.id },
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
    }
    listeners.add(draw);
    var stopChats = onChats(draw);
    draw();
    load().catch(host().fail);
    return { state: function () { return ''; }, setState: function () {},
      commands: function () { return [{ label: 'New Group Chat…', run: function () { return openPicker({}); } }, { label: 'New Message…', run: function () { showForm(); } }, { label: 'Trust a Link…', run: function () { showLinkForm(null); } }, { label: 'Mark All Read', run: markRead }]; },
      destroy: function () { disposed = true; window.dinahRender.cancel(root); listeners.delete(draw); stopChats(); } };
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
  function conversation(root, emptyBody, fetch, box, seenKey) {
    root.classList.add('bus-conversation');
    var log = node('div', 'bus-log'), list = node('div', 'bus-log-list vstack gap-2');
    log.dataset.uiScroll = 'bus-log';
    log.appendChild(list);
    /* Where the person stopped reading: the highest message they had seen is kept (bus-seen, by
       seenKey), and the messages from others after it are marked, with a line before the first and
       a button that goes to it. They are marked when the window opens and each time it comes to the
       front again, so a group left open behind another window shows what arrived meanwhile. A message
       is seen when the list is at its end in the window in front; one that arrives while the person
       is looking is not marked. The list still opens at its end. */
    var jump = node('button', 'btn btn-sm bus-jump'); jump.type = 'button'; jump.hidden = true;
    root.append(jump, log, box.el);
    var disposed = false, revision = 0, drawn = [], following = true, placed = -1, key = {};
    var seenMark = seenKey ? storedSeen() : 0, away = true, newLine = null, firstNew = null, marked = false;
    function storedSeen() { var all = window.dinahUIState.get('bus-seen'); return all && Number(all[seenKey]) || 0; }
    function inFront() {
      var win = root.closest('.win');
      return !!win && !win.hidden && document.visibilityState === 'visible' && !!log.clientHeight && window.pudlWindows.state().top === win.dataset.win;
    }
    function keepSeen() {
      if (!seenKey || !drawn.length || !inFront() || !atEnd()) return;
      var last = 0; drawn.forEach(function (d) { if (d.message && d.message.id > last) last = d.message.id; });
      if (last <= seenMark) return;
      seenMark = last;
      var all = Object.assign({}, window.dinahUIState.get('bus-seen') || {});
      all[seenKey] = last; window.dinahUIState.set('bus-seen', all);
    }
    function clearMarks() {
      list.querySelectorAll('.bus-unseen').forEach(function (el) { el.classList.remove('bus-unseen'); });
      if (newLine) { newLine.remove(); newLine = null; }
      firstNew = null;
      jump.hidden = true;
    }
    function remark() {
      clearMarks();
      // The marks are for what is drawn; a window in front before its messages are drawn is marked after.
      if (drawn.length) marked = true;
      if (!seenKey || !seenMark) return;
      var fresh = drawn.filter(function (d) { return d.message && d.message.id > seenMark && !d.message.fromPerson && d.message.kind !== 'member'; });
      if (!fresh.length) return;
      fresh.forEach(function (d) { d.div.classList.add('bus-unseen'); });
      newLine = node('p', 'system-line bus-new-line', fresh.length === 1 ? 'New since you last looked' : fresh.length + ' new since you last looked');
      fresh[0].div.before(newLine);
      firstNew = fresh[0].div;
      jump.textContent = 'Jump to Where You Stopped';
      jump.hidden = false;
      toEnd(); // the line adds height above the end, which the list was following
      lineSeen(); lineSeenSoon();
    }
    // The window coming to the front marks what arrived while it was away, before what is at its end counts as seen.
    function check() {
      if (disposed || !seenKey) return;
      if (!inFront()) { away = true; return; }
      if (away) { away = false; marked = false; }
      if (!marked) remark();
      keepSeen();
    }
    // The button is for marked messages that begin above the visible part of the list. It goes when the
    // first of them is in view (a window that has just come to the front only shows that once it is laid
    // out, so it is asked again after the layout) and when the reader reaches the end.
    function lineSeen() {
      if (!firstNew || jump.hidden || !log.clientHeight) return;
      var a = firstNew.getBoundingClientRect(), b = log.getBoundingClientRect();
      if (a.top >= b.top - 2) jump.hidden = true;
    }
    function lineSeenSoon() { requestAnimationFrame(function () { requestAnimationFrame(function () { if (!disposed) lineSeen(); }); }); }
    jump.addEventListener('click', function () { if (!newLine) return; following = false; newLine.scrollIntoView({ block: 'start' }); jump.hidden = true; });
    document.addEventListener('pudl:windows-change', check);
    document.addEventListener('visibilitychange', check);
    function act(what, view) {
      if (what === 'quote') { window.dinahChatText.quoteInto(box.text, view.text); box.changed(); return; }
      if (what === 'edit' && view.message) { box.write(box.entry(view.message)); box.text.focus(); box.text.setSelectionRange(box.text.value.length, box.text.value.length); }
    }
    window.dinahChatText.openLinks(list, function (a) {
      var el = a.closest('[data-bus-message]'), view = el && drawn.find(function (d) { return d.div === el; });
      return view && view.message ? senderFolder(view.message) : '';
    });
    function atEnd() { return log.scrollHeight - log.scrollTop - log.clientHeight < 24; }
    function toEnd() { if (following && log.clientHeight) { log.scrollTop = log.scrollHeight; placed = log.scrollTop; } }
    log.addEventListener('scroll', function () {
      if (following && log.scrollTop === placed) return;
      following = atEnd();
      // The reader reaching the end themselves has found what is there.
      if (following && !jump.hidden && newLine) jump.hidden = true;
      lineSeen(); check();
    }, { passive: true });
    // Content that grows without a new message, such as a diagram drawn,
    // keeps the end in view too.
    var grown = new ResizeObserver(function () { if (!disposed) window.dinahRender.enqueue(key, root, function () {}, function () { toEnd(); lineSeen(); check(); }); });
    grown.observe(list);
    // A window laid out after its messages were drawn is looked at again when the list gets its size.
    grown.observe(log);
    // A message whose file was removed is drawn again, to show it removed.
    function signature(m) { return [m.id, m.from, m.text, JSON.stringify(m.relay || null), (m.mentions || []).map(function (x) { return x.title; }).join('|'), (m.attachments || []).map(function (a) { return a.index + (a.removed ? '-' : ''); }).join('|')].join('\u0000'); }
    function draw(end) {
      if (disposed) return Promise.resolve();
      if (end === true) following = true;
      var requested = ++revision;
      return Promise.resolve(fetch()).then(function (ms) {
        if (disposed || requested !== revision) return;
        ms = ms || [];
        box.history.set(ms.filter(function (m) { return m.fromPerson && m.kind !== 'member'; }).map(box.entry));
        var same = drawn.length <= ms.length && drawn.every(function (d, i) { return d.sig === signature(ms[i]); });
        if (same && drawn.length === ms.length && (ms.length || list.firstChild)) return;
        // New messages go after those drawn. A conversation drawn again is
        // built aside and put in place whole, so the list never stands
        // empty and the reader keeps their place.
        var adding = same && drawn.length > 0, index = adding ? drawn.length : 0;
        var into = adding ? list : document.createDocumentFragment(), views = adding ? drawn : [];
        window.dinahRender.enqueue(draw, root, function (deadline) {
          return window.dinahChatText.keepPlace(log, function () {
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
            // A conversation drawn again lost the marks with its elements.
            if (newLine && !newLine.isConnected) remark();
            return false;
          });
        }, function () { toEnd(); check(); });
      }).catch(function (err) { if (!disposed) host().toast(message(err), 'danger'); });
    }
    function redraw() { return draw(); }
    listeners.add(redraw);
    draw();
    return { draw: draw, focus: function () { box.text.focus(); },
      destroy: function () { disposed = true; grown.disconnect(); window.dinahRender.cancel(draw); window.dinahRender.cancel(key); listeners.delete(redraw); document.removeEventListener('pudl:windows-change', check); document.removeEventListener('visibilitychange', check); } };
  }

  /* The composer of a group chat: the chat's box with the group's @ list.
     @ opens a list of the group's members, with everyone and Add a Chat…
     after them; choosing one puts it in the box whole, as one unit that
     Backspace takes out whole, and the message is sent with the list of the
     chats it mentions (decision 2). The mentions chosen are part of what
     the box holds, so a draft, a message brought back with Up and one put
     back with Edit keep them. A group has no provider, model, Steer or Stop
     of its own, and no skills, which run in a chat's agent. */
  function mentionComposer(key, members, addChat, send, limit) {
    var mentioned = [];
    var box = composer({ label: 'Message to the Group', placeholder: 'Write to the group. Type @ and a chat\'s title to ask particular chats, or @everyone to ask them all.', draftKey: 'group:' + key, files: true, limit: limit, send: function (text, state, files) {
      var ids = mentioned.filter(function (x) { return text.indexOf('@' + x.title) >= 0; }).map(function (x) { return x.id; });
      return send(text, ids.filter(function (id, i) { return ids.indexOf(id) === i; }), files);
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
    box.sent = function () { close(); setTimeout(function () { refreshNote(); }, 0); };
    box.el.classList.add('group-composer');
    var pop = box.pop;
    pop.classList.add('group-mention-pop'); pop.setAttribute('aria-label', 'Members');
    var text = box.text, choices = [], chosen = 0, start = -1;
    text.setAttribute('aria-autocomplete', 'list'); text.setAttribute('aria-controls', pop.id);
    function close() { pop.hidden = true; start = -1; text.removeAttribute('aria-expanded'); }
    /* The @ the caret follows and what has been typed after it. Titles have
       spaces, so the search runs to the caret; an @ that begins a mention
       already chosen is finished, and the words typed after it are the
       message, not a search. */
    function query() {
      var before = text.value.slice(0, text.selectionStart), m = /(^|\s)@([^@\n]{0,40})$/.exec(before);
      if (!m) return null;
      var at = before.length - m[2].length - 1;
      var done = mentioned.some(function (x) { return text.value.slice(at + 1, at + 1 + x.title.length) === x.title; });
      return done ? null : { at: at, q: m[2].toLowerCase() };
    }
    function drawPop() {
      var q = query();
      if (!q) { close(); return; }
      start = q.at;
      choices = members().filter(function (m) { return m.state !== 'person' && m.title.toLowerCase().indexOf(q.q) >= 0; })
        .map(function (m) { var state = memberState(m); return { id: m.id, title: m.title, meta: [m.workspace, state === 'closed' ? 'closed, opens in the background for the mention' : ''].filter(Boolean).join(', '), state: state }; });
      if ('everyone'.indexOf(q.q) === 0) choices.push({ id: 'everyone', title: 'everyone', meta: 'Every member chat, opening closed ones in the background' });
      // Typing that matches no member is not a mention, so the list closes
      // rather than offering Add a Chat… alone under every word.
      if (!choices.length && q.q) { close(); return; }
      choices.push({ add: true, title: 'Add a Chat…', meta: 'Put another chat in the group' });
      chosen = Math.min(chosen, choices.length - 1);
      pop.replaceChildren(node('div', 'md-section-label', 'Members'));
      choices.forEach(function (c, i) {
        var row = node('div', 'md-row skill-row mention-row'); row.setAttribute('role', 'option');
        row.setAttribute('aria-selected', String(i === chosen)); row.dataset.mention = c.add ? 'add' : c.id;
        var a = node('a', 'md-item'); a.href = '#';
        a.appendChild(node('span', 'mention-name', c.title));
        if (c.meta) a.appendChild(node('span', 'md-meta', c.meta));
        if (c.state) a.appendChild(window.dinahBrowserList.badge(stateBadge(c.state)));
        a.addEventListener('mousedown', function (e) { e.preventDefault(); pick(i); });
        row.appendChild(a);
        pop.appendChild(row);
      });
      pop.appendChild(node('div', 'slash-hint', 'Enter or Tab adds · Up and Down choose · Esc closes'));
      pop.hidden = false;
      text.setAttribute('aria-expanded', 'true');
    }
    function pick(i) {
      var c = choices[i];
      if (!c) return;
      if (c.add) { text.value = text.value.slice(0, start) + text.value.slice(text.selectionStart); close(); box.changed(); addChat(); return; }
      var word = '@' + c.title + ' ', end = text.selectionStart;
      text.value = text.value.slice(0, start) + word + text.value.slice(end);
      var caret = start + word.length;
      text.setSelectionRange(caret, caret);
      if (c.id !== 'everyone') mentioned.push({ id: c.id, title: c.title });
      else mentioned.push({ id: 'everyone', title: 'everyone' });
      close();
      box.changed();
      text.focus();
    }
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
    text.addEventListener('blur', function () { setTimeout(close, 150); });
    // Registered for the capturing phase, so that while the list is open
    // its keys choose in it and never send or bring back a message.
    text.addEventListener('keydown', function (e) {
      if (!pop.hidden) {
        var handled = true;
        if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { chosen = (chosen + (e.key === 'ArrowDown' ? 1 : choices.length - 1)) % choices.length; drawPop(); }
        else if ((e.key === 'Enter' && !e.ctrlKey && !e.metaKey) || e.key === 'Tab') pick(chosen);
        else if (e.key === 'Escape') close();
        else handled = false;
        if (handled) { e.preventDefault(); e.stopImmediatePropagation(); return; }
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
    /* A message that mentions nobody reaches the members as a notice they may read, and asks none of them to answer.
       The box says so while that is what the message would do, and offers @everyone as one press. */
    var note = node('p', 'form-help group-notice-note'); note.hidden = true;
    var everyone = node('button', 'btn btn-sm', 'Mention Everyone'); everyone.type = 'button';
    note.appendChild(node('span', null, 'Nobody is asked to answer this. Each member only gets a notice that you wrote. '));
    note.appendChild(everyone);
    box.el.appendChild(note);
    function refreshNote() {
      var value = text.value, mentions = /(^|\s)@everyone\b/.test(value) || mentioned.some(function (x) { return value.indexOf('@' + x.title) >= 0; });
      note.hidden = !value.trim() || mentions;
    }
    everyone.addEventListener('click', function () {
      text.value = '@everyone ' + text.value;
      if (!mentioned.some(function (x) { return x.id === 'everyone'; })) mentioned.push({ id: 'everyone', title: 'everyone' });
      text.focus(); text.setSelectionRange(text.value.length, text.value.length);
      box.changed();
    });
    var changed = box.changed;
    box.changed = function () { var r = changed.apply(box, arguments); refreshNote(); return r; };
    text.addEventListener('input', refreshNote);
    box.mention = function (m) {
      if (!m) return;
      var word = '@' + m.title;
      if (text.value.indexOf(word) < 0) text.value += (text.value && !/\s$/.test(text.value) ? ' ' : '') + word + ' ';
      if (!mentioned.some(function (x) { return x.id === m.id; })) mentioned.push({ id: m.id, title: m.title });
      box.changed();
    };
    box.restore();
    refreshNote();
    return box;
  }

  /* ---- A group chat's window ---- */
  window.dinahWorkbench.register('bus-room', { title: 'Group Chat', glyph: 'comments', init: function (root, opts, item) {
    var key = item.path, room = group(key), disposed = false;
    var targetMessage = 0, pendingMention = null, targetFrame = 0, targetPending = false;
    function stateString() { return JSON.stringify({ message: targetMessage }); }
    function showTarget() {
      if (disposed || !targetMessage || !targetPending) return;
      var row = root.querySelector('[data-bus-message="' + targetMessage + '"]');
      if (!row) return;
      root.querySelectorAll('.bus-target-message').forEach(function (n) { n.classList.remove('bus-target-message'); });
      targetPending = false; row.classList.add('bus-target-message'); row.scrollIntoView({ block: 'center' });
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
      try { var state = value ? JSON.parse(value) : {}; targetMessage = Number(state.message) || 0; pendingMention = state.mention || null; targetPending = !!targetMessage; }
      catch (error) { targetMessage = 0; pendingMention = null; }
      applyMention(); targetSoon();
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
    var box = mentionComposer(key, members, addChats, function (text, mentions, files) { return Promise.resolve(app().BusSendToGroup(key, text, mentions, files || [])).then(function () { c.draw(true); }); }, fileLimit);
    var c = conversation(root, 'Everything said in the group appears here. The chats a message mentions are asked to answer it, and the other members are told that it was said.',
      function () { return Promise.resolve(app().BusGroupMessages(key)).then(function (ms) { markRead(); return ms; }); }, box, 'group:' + key);
    // Files dropped anywhere on the group's window go with the next message.
    box.dropOn(root);
    /* The members, over the conversation: a chip for each chat with its
       state and a menu, then the person, then Add Chats…. */
    var bar = node('div', 'group-members'); bar.setAttribute('role', 'list'); bar.setAttribute('aria-label', 'Members');
    root.prepend(bar);
    var menu = node('nav', 'menu-panel group-member-menu'); menu.setAttribute('popover', ''); menu.setAttribute('role', 'menu'); menu.setAttribute('aria-label', 'Member');
    menu.id = 'group-member-menu-' + Math.random().toString(36).slice(2);
    root.appendChild(menu);
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
      var open = node('button', 'menu-action', 'Open Chat'); open.type = 'button'; open.setAttribute('role', 'menuitem');
      if (!m.chat) { open.setAttribute('aria-disabled', 'true'); open.title = 'This chat runs outside the window.'; }
      open.addEventListener('click', function () { menu.hidePopover(); if (m.chat) Promise.resolve(window.dinahChats.open(m.chat)).catch(host().fail); });
      var remove = node('button', 'menu-action danger', 'Remove From Group…'); remove.type = 'button'; remove.setAttribute('role', 'menuitem');
      remove.addEventListener('click', function () {
        menu.hidePopover();
        host().ask('Remove ' + m.title + ' From the Group?', m.title + ' stops receiving the group\'s messages. What was said stays in the group.', 'Remove', true).then(function (yes) {
          if (!yes) return;
          return Promise.resolve(app().BusRemoveFromGroup(key, m.id)).then(load);
        }).catch(host().fail);
      });
      menu.append(open, remove);
      menu.dataset.menuAnchor = button.id;
      if (menu.matches(':popover-open')) menu.hidePopover();
      menu.showPopover();
      var r = button.getBoundingClientRect();
      menu.style.position = 'fixed'; menu.style.margin = '0';
      menu.style.left = r.left + 'px'; menu.style.top = (r.bottom + 4) + 'px';
    }
    function drawMembers() {
      if (disposed) return;
      room = group(key);
      // A group deleted elsewhere, such as on the bus's page, takes no more
      // messages; its conversation stays readable.
      var gone = !room && loaded;
      box.close(gone);
      box.limitChanged();
      if (gone) { bar.replaceChildren(node('span', 'md-meta', 'This group chat has been deleted.')); return; }
      var win = root.closest('[data-win]');
      if (room && win && window.pudlWindows) {
        var t = win.querySelector('.win-title');
        if (t && t.textContent.trim() !== room.title) {
          window.pudlWindows.retitle(win.dataset.win, room.title);
          /* The menu bar's own copy of the window's title does not follow a
             retitle on its own today; until PUDL draws that link itself, we
             tell the bar. */
          if (window.pudlMenubar && window.pudlMenubar.refresh) window.pudlMenubar.refresh();
        }
      }
      window.dinahRender.enqueue(drawMembers, root, function () {
        bar.replaceChildren(node('span', 'md-meta', 'Members'));
        members().forEach(function (m, i) {
          if (m.state === 'person') {
            var you = node('span', 'chip member-chip person', m.title + ' (You)'); you.setAttribute('role', 'listitem');
            you.dataset.groupMember = 'person';
            bar.appendChild(you);
            return;
          }
          var chip = node('span', 'chip member-chip'); chip.setAttribute('role', 'listitem'); chip.dataset.groupMember = m.id;
          chip.append(stateGlyph(memberState(m)), node('span', 'member-title', m.title));
          var more = glyphButton('member-menu-btn', 'caret', m.title + ' Menu');
          more.id = menu.id + '-' + i;
          more.setAttribute('aria-haspopup', 'menu');
          more.addEventListener('click', function () { showMenu(more, m); });
          chip.appendChild(more);
          bar.appendChild(chip);
        });
        var add = node('button', 'btn btn-sm group-add-chats', 'Add Chats…'); add.type = 'button';
        add.addEventListener('click', addChats);
        bar.appendChild(add);
        var budgetButton = node('button', 'btn btn-sm group-turn-budget', 'Turn Budget…'); budgetButton.type = 'button';
        budgetButton.addEventListener('click', openBudget);
        bar.appendChild(budgetButton);
        // The turn budget is shown only when the group has one.
        if (room && typeof room.budget === 'number' && room.budget >= 0) {
          var remaining = room.budget - (room.used || 0), spent = remaining <= 0;
          var left = node('span', 'chip group-budget' + (spent ? ' warn' : ''),
            spent ? 'Turn budget spent (' + room.budget + '). Your next message starts it again.'
              : remaining + (remaining === 1 ? ' turn' : ' turns') + ' left before the group waits for you');
          left.setAttribute('role', 'listitem'); left.dataset.groupBudget = spent ? 'spent' : 'open';
          bar.appendChild(left);
        }
      });
    }
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
          var win = root.closest('[data-win]');
          if (win && window.pudlWindows) window.pudlWindows.close(win.dataset.win);
          load();
        });
      }).catch(host().fail);
    }
    return { state: stateString, setState: setState, destroy: function () { disposed = true; targetObserver.disconnect(); if (targetFrame) cancelAnimationFrame(targetFrame); window.dinahRender.cancel(drawMembers); listeners.delete(refresh); stopChats(); document.removeEventListener('dinah:file-limits-changed', limitsChanged); c.destroy(); menu.remove(); },
      focus: function () { box.text.focus(); },
      // The person is always a member, so a group has no Leave (decision 8).
      // Attach… is disabled at the limit on the files in one message.
      commands: function () { var full = box.attachReason(); return [{ label: 'Make This Group a Team…', hidden: !!(room && room.lead) || !app().MakeGroupATeam, run: function () { return window.dinahTeams.convertGroup(room).then(load); } }, { label: 'Add Chats…', run: addChats }, { label: 'Turn Budget…', run: openBudget }, { label: 'Rename Group…', run: renameGroup }, { label: 'Attach…', disabled: !!full, reason: full, run: function () { return box.attach(); } },{ label: 'Refresh', run: function () { c.draw(); load(); } }, { label: 'Delete Group…', danger: true, run: deleteGroup }]; } };
  } });

  /* ---- The picker of a group chat ----
     Every chat the window knows, by workspace, the workspace of the chat it
     was opened from first, each with a check box, its provider, its state
     and when it was last active. The filter narrows the list. A chat with
     no title is given one in place before it can be ticked, since two rows
     reading "Untitled Chat in studio" cannot be told apart (decision 7). A
     new group is named, or takes its chats' titles as its name. The ticked
     chats and the name are the window's state, so its address keeps them. */
  window.dinahWorkbench.register('group-picker', { title: 'Start a Group Chat', glyph: 'comments', init: function (root, opts, item) {
    var spec = {};
    try { spec = JSON.parse(item.path || '{}') || {}; } catch (err) { spec = {}; }
    var adding = spec.group || '', from = spec.from || '', chosen = {}, name = '', filter = '', titling = '', disposed = false, active = 0;
    if (from) chosen[from] = true;
    root.classList.add('group-picker');
    root.innerHTML = '';
    var find = node('input', 'form-input picker-filter'); find.type = 'search'; find.placeholder = 'Filter by Title, Workspace or Provider';
    find.setAttribute('aria-label', 'Filter the Chats'); find.autocomplete = 'off';
    var heading = node('h2', 'visually-hidden', adding ? 'Add Chats' : 'Start a Group Chat'); heading.id = 'picker-title-' + Math.random().toString(36).slice(2);
    var list = node('div', 'listbox picker-list'); list.setAttribute('role', 'listbox'); list.setAttribute('aria-multiselectable', 'true');
    list.setAttribute('aria-labelledby', heading.id); list.tabIndex = 0;
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
          if (row) { active = rowsShown().indexOf(row); row.tabIndex = -1; row.focus(); row.scrollIntoView({ block: 'nearest' }); }
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
    function rowsShown() { return Array.prototype.slice.call(list.querySelectorAll('.listbox-row:not([hidden])')); }
    function draw() {
      if (disposed) return;
      var taken = inGroup(), q = filter.trim().toLowerCase(), titles = [];
      var focused = list.contains(document.activeElement) && document.activeElement.matches('.picker-row') ? document.activeElement.dataset.chat : '';
      var groups = chats(), known = {};
      list.replaceChildren();
      groups.forEach(function (w, gi) {
        var g = node('div'); g.setAttribute('role', 'group');
        var label = node('div', 'listbox-group', w.workspace); label.id = heading.id + '-ws-' + gi; label.setAttribute('role', 'presentation');
        g.setAttribute('aria-labelledby', label.id);
        g.appendChild(label);
        var any = false;
        w.chats.forEach(function (c) {
          known[c.id] = c;
          var already = taken.indexOf(c.id) >= 0, on = already || !!chosen[c.id];
          if (on && !already && !c.untitled) titles.push(c);
          var find = (c.title + ' ' + c.workspace + ' ' + c.provider).toLowerCase();
          var row = node('div', 'listbox-row picker-row' + (on ? ' is-chosen' : '')); row.setAttribute('role', 'option');
          row.dataset.chat = c.id; row.setAttribute('aria-selected', String(on));
          if (already) row.setAttribute('aria-disabled', 'true');
          row.hidden = !!q && find.indexOf(q) < 0;
          any = any || !row.hidden;
          var box = node('input'); box.type = 'checkbox'; box.tabIndex = -1; box.setAttribute('aria-hidden', 'true'); box.checked = on; box.disabled = already;
          var text = node('span', 'listbox-text');
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
            text.appendChild(give);
            setTimeout(function () { if (give.isConnected) give.focus(); }, 0);
          } else {
            text.appendChild(document.createTextNode(c.title));
            if (c.id === from) text.appendChild(node('span', 'chip', 'This Chat'));
            if (already) text.appendChild(node('span', 'chip', 'In the Group'));
          }
          row.append(box, text, node('span', 'col col-provider', c.provider), node('span', 'col state'), node('span', 'col col-active', c.updatedAt ? when(c.updatedAt) : ''));
          row.querySelector('.state').appendChild(window.dinahBrowserList.badge(stateBadge(c.state)));
          if (c.untitled && titling !== c.id) row.appendChild(node('p', 'row-note', 'Give this chat a title before it joins a group. Choose it to title it here.'));
          else if (c.state === 'closed' && on && !already) row.appendChild(node('p', 'row-note', 'This chat is closed. A message that mentions it opens it in the background; the group\'s other messages wait for it until it opens.'));
          row.addEventListener('click', function () { toggle(c); });
          g.appendChild(row);
        });
        g.hidden = !any;
        list.appendChild(g);
      });
      if (!groups.length) list.appendChild(window.dinahBrowserList.empty({ title: 'No Chats Yet', body: 'Open a workspace and start a chat first.' }));
      // The opening chat is ticked; an untitled one asks for its title.
      if (from && known[from] && known[from].untitled && chosen[from] && !titling) { titling = from; chosen[from] = false; draw(); return; }
      titles.sort(function (a, b) { return (b.id === from) - (a.id === from); });
      var names = titles.map(function (c) { return c.title; }), n = names.length;
      nameInput.placeholder = n === 0 ? 'Choose at least one chat' : n === 1 ? names[0] : names.slice(0, -1).join(', ') + ' and ' + names[n - 1];
      count.textContent = adding ? (n === 0 ? 'Choose the chats to add.' : 'Adds ' + (n === 1 ? 'one chat' : n + ' chats') + ' to the group.')
        : n === 0 ? 'Choose at least one chat. You are always in the group.' : 'The group will have ' + (n === 1 ? 'one chat' : n + ' chats') + ' and you.';
      if (selectionNote) count.textContent = selectionNote;
      go.disabled = n === 0;
      var rows = rowsShown();
      rows.forEach(function (r, i) { r.classList.toggle('is-active', i === active); });
      if (focused) { var kept = rows.find(function (r) { return r.dataset.chat === focused; }); if (kept) { active = rows.indexOf(kept); rows.forEach(function (r, i) { r.classList.toggle('is-active', i === active); }); kept.tabIndex = -1; kept.focus({ preventScroll: true }); } }
    }
    function toggle(c) {
      if (inGroup().indexOf(c.id) >= 0) return;
      if (c.untitled) { titling = c.id; draw(); return; }
      chosen[c.id] = !chosen[c.id];
      changed();
      draw();
    }
    list.addEventListener('keydown', function (e) {
      var rows = rowsShown();
      if (!rows.length) return;
      if (e.key === 'ArrowDown' || e.key === 'ArrowUp') { e.preventDefault(); active = Math.max(0, Math.min(rows.length - 1, active + (e.key === 'ArrowDown' ? 1 : -1))); rows[active].tabIndex = -1; rows[active].focus(); draw(); }
      else if (e.key === ' ' || e.key === 'Enter') { e.preventDefault(); var c = pickerChats().find(function (x) { return x.id === rows[Math.min(active, rows.length - 1)].dataset.chat; }); if (c) toggle(c); }
    });
    find.addEventListener('input', function () { filter = find.value; selectionNote = ''; active = 0; changed(); draw(); });
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
    load().catch(function () {});
    return { state: stateString, setState: setState, focus: function () { if (creation) { creation.focus(); return; } var selected = selectionNote && lastCreated && list.querySelector('[data-chat="' + CSS.escape(lastCreated) + '"][aria-selected="true"]'); if (selected) { selected.tabIndex = -1; selected.focus(); } else find.focus(); },
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

  window.dinahWorkbench.register('bus-thread', { title: 'Thread', glyph: 'inbox', init: function (root, opts, item) {
    var id = Number(item.path);
    var box = composer({ label: 'Reply', draftKey: 'thread:' + id, send: function (text) { return Promise.resolve(app().BusReplyInThread(id, text)).then(function () { c.draw(true); }); } });
    box.restore();
    var c = conversation(root, 'The message and its replies appear here.', function () { return app().BusThread(id); }, box);
    return { destroy: c.destroy, commands: function () { return [{ label: 'Refresh', run: c.draw }]; } };
  } });

  window.dinahCommands.register({ id: 'bus', glyph: 'inbox', menu: 'view', mnemonic: 'R', label: 'Group Chats', run: function () { return window.dinahWorkbench.open('bus'); } });

  /* The pane opens once by itself, as a tab behind the one in front of the
     right-hand dock, so a layout saved before it existed finds it; a person
     who closes it has chosen to, and it is not opened again. */
  var OFFERED = 'dinah-bus-offered', restored = false, offered = false;
  document.addEventListener('dinah:ui-state-restored', function () { restored = true; offer(); });
  window.addEventListener('load', offer);
  function offer() {
    var ui = window.dinahUIState, windows = window.pudlWindows;
    if (offered || !restored || !ui || !windows || document.readyState !== 'complete') return;
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
  window.dinahBus = { load: load, overview: function () { return overview; }, chat: chat, startGroup: startGroup, addItems: addItems, addChatToGroup: addChatToGroup, openGroup: function (key, target) { var r = group(key) || { name: key, title: key }; return window.dinahWorkbench.open({ kind: 'bus-room', path: r.name, title: r.title }, target ? JSON.stringify(target) : undefined); } };
})();
