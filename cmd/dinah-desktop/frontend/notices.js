/* Notices: a list of notifications and its two views, the compact list a
   status item opens and the full pane. Nothing here belongs to one
   application. Persistence, navigation and colours come from the adapter
   given to create(), so the same file serves any page built on PUDL. The
   design is in docs/design/notifications.md.

     var store = Notices.create(adapter);
     store.add({ id, type, description, ... });
     store.mountList(host, { popover });   // the compact list
     store.mountPane(host, { filter });    // the full cards */
(function () {
  'use strict';

  /* Each type has a glyph, the tone that colours it, a word for readers,
     and whether it needs the person unless the entry says otherwise. */
  var TYPES = {
    question: { glyph: 'comments', tone: 'warning', word: 'Question', needs: true },
    error: { glyph: 'stop', tone: 'error', word: 'Error', needs: true },
    warning: { glyph: 'warning', tone: 'warning', word: 'Warning', needs: false },
    success: { glyph: 'check', tone: 'success', word: 'Success', needs: false },
    info: { glyph: 'info', tone: 'info', word: 'Information', needs: false }
  };
  var COLOURS = { error: 'var(--danger)', warning: 'var(--warn)', success: 'var(--positive)', info: 'var(--accent)' };

  function el(tag, cls, text) {
    var n = document.createElement(tag);
    if (cls) n.className = cls;
    if (text !== undefined) n.textContent = text;
    return n;
  }

  /* One polite live region for every store on the page. */
  var announcer = null;
  function announce(text) {
    if (!announcer) {
      announcer = el('div', 'notices-announcer');
      announcer.setAttribute('role', 'status');
      announcer.setAttribute('aria-live', 'polite');
      document.body.appendChild(announcer);
    }
    announcer.textContent = '';
    setTimeout(function () { announcer.textContent = text; }, 50);
  }

  function create(adapter) {
    adapter = adapter || {};
    var records = new Map(), listeners = new Set(), announced = new Set(), seq = 0;
    var views = [];

    function normalize(input, old) {
      var type = Object.prototype.hasOwnProperty.call(TYPES, input.type) ? input.type : 'info';
      var t = TYPES[type];
      var description = String(input.description || input.title || input.body || '').replace(/\s+/g, ' ').trim();
      return {
        id: String(input.id), type: type, glyph: input.glyph || t.glyph, tone: t.tone,
        description: description, title: input.title || '', body: input.body || '',
        details: Array.isArray(input.details) ? input.details.slice() : [],
        time: input.time ? new Date(input.time) : old ? old.time : new Date(),
        source: input.source || '', target: input.target || null,
        needsPerson: input.needsPerson != null ? !!input.needsPerson : t.needs,
        actions: Array.isArray(input.actions) ? input.actions.slice() : [],
        dismissible: !!input.dismissible, dismiss: typeof input.dismiss === 'function' ? input.dismiss : null,
        clearable: input.clearable != null ? !!input.clearable : type !== 'question',
        render: typeof input.render === 'function' ? input.render : null,
        className: input.className || '', data: Object.assign({}, input.data || {}),
        announced: !!input.announced
      };
    }

    function emit(kind, id) {
      listeners.forEach(function (fn) {
        try { fn({ kind: kind, id: id }); } catch (e) { setTimeout(function () { throw e; }, 0); }
      });
    }

    function add(input) {
      if (!input || input.id == null) throw new Error('A notification needs an id.');
      var old = records.get(String(input.id));
      var entry = normalize(input, old && old.entry);
      records.set(entry.id, { entry: entry, seq: old ? old.seq : ++seq, version: old ? old.version + 1 : 1 });
      if (!old && entry.type === 'question' && !entry.announced && !announced.has(entry.id)) {
        announce(TYPES.question.word + ': ' + entry.description + (entry.source ? ', ' + entry.source : ''));
      }
      if (entry.type === 'question') announced.add(entry.id);
      emit(old ? 'update' : 'add', entry.id);
      return entry;
    }
    function update(id, patch) {
      var r = records.get(String(id));
      if (!r) return null;
      return add(Object.assign({}, r.entry, patch, { id: r.entry.id }));
    }
    function remove(id) {
      id = String(id);
      if (!records.delete(id)) return false;
      emit('remove', id);
      return true;
    }
    function get(id) { var r = records.get(String(id)); return r ? r.entry : null; }

    /* Questions first, then the rest, each newest first. */
    function ordered() {
      return Array.from(records.values()).sort(function (a, b) {
        var qa = a.entry.type === 'question' ? 0 : 1, qb = b.entry.type === 'question' ? 0 : 1;
        if (qa !== qb) return qa - qb;
        var ta = a.entry.time.getTime(), tb = b.entry.time.getTime();
        if (ta !== tb) return tb - ta;
        return b.seq - a.seq;
      });
    }
    function list(test) {
      return ordered().map(function (r) { return r.entry; }).filter(function (e) { return !test || test(e); });
    }
    function count() {
      var total = 0, needs = 0;
      records.forEach(function (r) { total++; if (r.entry.needsPerson) needs++; });
      return { total: total, needsPerson: needs };
    }
    function subscribe(fn) { listeners.add(fn); return function () { listeners.delete(fn); }; }

    /* Opening an entry shows it where the adapter can, and otherwise in the
       pane, scrolled to it. */
    function open(id) {
      var entry = get(id);
      if (!entry) return Promise.resolve(false);
      return Promise.resolve(adapter.open ? adapter.open(entry) : false).then(function (shown) {
        if (shown) return true;
        return Promise.resolve(adapter.showAll ? adapter.showAll() : null).then(function () { return reveal(entry.id); });
      });
    }
    function reveal(id) {
      return views.some(function (v) { return v.reveal && v.reveal(id); });
    }

    function dismiss(id) {
      var entry = get(id);
      if (!entry) return Promise.resolve();
      return Promise.resolve().then(function () {
        return entry.dismiss ? entry.dismiss(entry) : adapter.dismiss ? adapter.dismiss(entry) : null;
      }).then(function () { remove(entry.id); });
    }
    /* Clear All takes every clearable entry. The adapter records it in one
       call; an entry with a dismissal of its own makes it, and stays if
       that fails. Resolves with the failures. */
    function clear() {
      var targets = list(function (e) { return e.clearable; });
      return Promise.resolve(adapter.clear ? adapter.clear(targets) : null).then(function () {
        return Promise.allSettled(targets.map(function (e) {
          if (e.dismiss) return Promise.resolve().then(function () { return e.dismiss(e); }).then(function () { remove(e.id); });
          if (adapter.clear) { remove(e.id); return null; }
          return dismiss(e.id);
        }));
      }).then(function (settled) {
        return settled.filter(function (s) { return s.status === 'rejected'; }).map(function (s) { return s.reason; });
      });
    }

    function failed(err) { if (adapter.error) adapter.error(err); else setTimeout(function () { throw err; }, 0); }
    function colour(tone) { return (adapter.colour && adapter.colour(tone)) || COLOURS[tone] || COLOURS.info; }
    function word(entry) { return TYPES[entry.type].word; }
    function when(entry) {
      return adapter.formatTime ? adapter.formatTime(entry.time)
        : entry.time.toLocaleString(undefined, { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' });
    }

    /* Gives an element the glyph and the colour of a type: the rows and
       cards here, and a host's own copy of an entry, such as a question
       drawn beside a chat's composer. */
    function decorate(node, typeOrEntry, plain) {
      var entry = typeof typeOrEntry === 'string' ? null : typeOrEntry;
      var type = entry ? entry.type : Object.prototype.hasOwnProperty.call(TYPES, typeOrEntry) ? typeOrEntry : 'info';
      var tone = TYPES[type].tone;
      // A row of the compact list takes no host styles, which are a card's.
      if (adapter.style && !plain) adapter.style(node, tone);
      node.style.setProperty('--glyph', 'var(--glyph-' + ((entry && entry.glyph) || TYPES[type].glyph) + ')');
      node.style.setProperty('--notices-colour', colour(tone));
      node.dataset.noticeType = type;
    }

    function button(label, cls, run) {
      var b = el('button', 'btn btn-sm' + (cls ? ' ' + cls : ''), label);
      b.type = 'button';
      b.title = label;
      b.addEventListener('click', function () {
        if (b.disabled) return;
        b.disabled = true;
        Promise.resolve().then(run).then(function () { b.disabled = false; }, function (err) { b.disabled = false; failed(err); });
      });
      return b;
    }

    /* The full card of an entry: a PUDL notice with its glyph and colour.
       A producer's renderer fills it; otherwise its title, body, details,
       time and source. Then its actions, Open for its source, and Dismiss. */
    function card(entry) {
      var c = el('div', 'notice notices-card' + (entry.className ? ' ' + entry.className : ''));
      decorate(c, entry);
      c.dataset.noticeId = entry.id;
      c.dataset.type = entry.type;
      Object.keys(entry.data).forEach(function (k) { if (entry.data[k] != null) c.dataset[k] = String(entry.data[k]); });
      c.setAttribute('role', 'group');
      c.setAttribute('aria-label', word(entry) + ': ' + entry.description);
      c.tabIndex = -1;
      var actions;
      if (entry.render) {
        entry.render(c, entry, api);
        actions = c.querySelector('.notice-actions');
        if (!actions) { actions = el('div', 'notice-actions hstack gap-2'); (c.querySelector('.notice-content, .notice-body') || c).appendChild(actions); }
      } else {
        var content = el('div', 'notice-content');
        content.appendChild(el('p', 'notice-title', entry.title || entry.description));
        if (entry.body) content.appendChild(el('p', 'notice-body', entry.body));
        if (entry.details.length) {
          var ul = el('ul', 'notices-details');
          entry.details.forEach(function (d) { ul.appendChild(el('li', '', d)); });
          content.appendChild(ul);
        }
        // The time is a time element, so that a host can write it again
        // from its datetime when its format changes.
        var meta = el('p', 'notices-meta', 'At '), at = el('time', 'notices-time', when(entry));
        if (!isNaN(entry.time)) at.dateTime = entry.time.toISOString();
        meta.append(at, (entry.source ? ' · ' + entry.source : '') + '.');
        content.appendChild(meta);
        actions = el('div', 'notice-actions hstack gap-2');
        content.appendChild(actions);
        c.appendChild(content);
      }
      entry.actions.forEach(function (a) {
        actions.appendChild(button(a.label, a.primary ? 'btn-primary' : '', function () { return a.run(entry); }));
      });
      if (entry.target && entry.source && adapter.openTarget) {
        actions.appendChild(button('Open ' + entry.source, 'notices-open', function () { return adapter.openTarget(entry); }));
      }
      if (adapter.context) { var context = adapter.context(entry); if (context) (c.querySelector('.notice-content') || c).appendChild(context); }
      if (entry.dismissible) actions.appendChild(button('Dismiss', (entry.actions.some(function (a) { return a.primary; }) ? '' : 'btn-primary ') + 'notices-dismiss', function () { return dismiss(entry.id); }));
      return c;
    }

    /* A compact row: the glyph in its colour and the description, clamped
       to two lines; the whole text is in its name and its tooltip. */
    function row(entry) {
      var li = el('li', 'notices-item');
      var b = el('button', 'menu-action notices-row');
      b.type = 'button';
      decorate(b, entry, true);
      b.dataset.noticeId = entry.id;
      var g = el('span', 'glyph notices-glyph');
      g.setAttribute('aria-hidden', 'true');
      b.appendChild(g);
      b.appendChild(el('span', 'notices-text', entry.description));
      b.setAttribute('aria-label', word(entry) + ': ' + entry.description + (entry.source ? ', ' + entry.source : ''));
      if (adapter.context) { var context = adapter.context(entry); if (context) b.appendChild(context); }
      b.title = entry.description;
      b.addEventListener('click', function () { open(entry.id).catch(failed); });
      li.appendChild(b);
      return li;
    }

    /* Keeps one element per entry in a container, in the store's order,
       rebuilding one only when its entry changed. Both views use it. */
    function keyed(container, build) {
      var nodes = new Map();
      return function draw(entries) {
        var keep = new Set(entries.map(function (e) { return e.id; }));
        // Moving a focused element takes its focus, so the gone go first and
        // the rest move only when their order changed, and focus is put back.
        var focus = container.contains(document.activeElement) ? document.activeElement : null;
        nodes.forEach(function (v, id) { if (!keep.has(id)) { v.node.remove(); nodes.delete(id); } });
        var cursor = container.firstChild;
        entries.forEach(function (entry) {
          var r = records.get(entry.id), had = nodes.get(entry.id), node;
          if (had && had.version === r.version) node = had.node;
          else {
            node = build(entry);
            if (had) {
              var focused = had.node.contains(document.activeElement);
              if (had.node === cursor) cursor = node;
              had.node.replaceWith(node);
              if (focused) { var f = node.matches('button') ? node : node.querySelector('button'); if (f) f.focus({ preventScroll: true }); }
            }
            nodes.set(entry.id, { node: node, version: r.version });
          }
          if (node === cursor) cursor = cursor.nextSibling;
          else container.insertBefore(node, cursor);
        });
        if (focus && focus !== document.activeElement && focus.isConnected) focus.focus({ preventScroll: true });
        return nodes;
      };
    }

    /* A view draws once per moment however many changes come. */
    function mount(drawNow) {
      var pending = false, alive = true;
      function schedule() {
        if (pending || !alive) return;
        pending = true;
        Promise.resolve().then(function () { if (!pending || !alive) return; pending = false; drawNow(); });
      }
      var stop = subscribe(schedule);
      drawNow();
      return {
        flush: function () { if (pending) { pending = false; drawNow(); } },
        refresh: drawNow,
        destroy: function () { alive = false; stop(); }
      };
    }

    function mountList(host, options) {
      options = options || {};
      var ul = el('ul', 'notices-list');
      ul.setAttribute('role', 'list');
      var empty = el('p', 'menu-empty notices-empty', options.empty || 'No notifications.');
      host.replaceChildren(ul, empty);
      var draw = keyed(ul, row);
      var view = mount(function () {
        var entries = list();
        draw(entries);
        empty.hidden = entries.length > 0;
      });
      /* The popover the list is in takes focus to its first row as it
         opens, so the arrow keys move through the rows at once. */
      var popover = options.popover;
      function toggled(e) {
        if (e.newState !== 'open') return;
        view.flush();
        var first = ul.querySelector('.notices-row');
        if (first) first.focus({ preventScroll: true });
      }
      if (popover) popover.addEventListener('toggle', toggled);
      var destroy = view.destroy;
      view.destroy = function () { destroy(); if (popover) popover.removeEventListener('toggle', toggled); };
      views.push(view);
      return view;
    }

    function mountPane(host, options) {
      options = options || {};
      var nodes = null;
      var draw = keyed(host, card);
      var view = mount(function () {
        var entries = list(), shown = 0;
        nodes = draw(entries);
        entries.forEach(function (entry) {
          var node = nodes.get(entry.id).node;
          node.hidden = !!(options.filter && !options.filter(entry, node));
          if (!node.hidden) shown++;
        });
        if (options.drawn) options.drawn({ total: entries.length, shown: shown });
      });
      view.shown = function (id) { view.flush(); var n = nodes && nodes.get(String(id)); return !!(n && !n.node.hidden); };
      /* Scrolls to an entry's card, marks it a moment and focuses its first
         button, or the card itself. */
      view.reveal = function (id) {
        view.flush();
        var n = nodes && nodes.get(String(id));
        if (!n || !n.node.isConnected) return false;
        var c = n.node;
        c.hidden = false;
        c.scrollIntoView({ block: 'nearest' });
        c.classList.remove('notices-flash'); void c.offsetWidth; c.classList.add('notices-flash');
        var b = c.querySelector('.btn-primary:not(:disabled)') || c.querySelector('button:not(:disabled)') || c;
        requestAnimationFrame(function () { b.focus({ preventScroll: true }); });
        return true;
      };
      views.push(view);
      return view;
    }

    var api = {
      add: add, update: update, remove: remove, get: get, list: list, count: count, subscribe: subscribe,
      open: open, reveal: reveal, dismiss: dismiss, clear: clear,
      card: card, decorate: function (node, typeOrEntry) { decorate(node, typeOrEntry, false); }, mountList: mountList, mountPane: mountPane,
      types: function () { return Object.keys(TYPES); }
    };
    return api;
  }

  window.Notices = Object.freeze({ create: create });
})();
