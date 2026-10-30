/* The parts every browser pane is built from:
   a toolbar of search, scope and commands; groups with PUDL's small raised
   tree toggle, a count badge and, for a group that stands for a place, a
   line saying where it is stored; two-line master-detail rows with a status
   badge; and PUDL's empty state. Workspace, Chats, Skills and the
   applets' workspace sections all use them, so they look alike. */
(function () {
  'use strict';
  var serial = 0;
  const node = window.dinahHelpers.node;
  /* A command is { label, title, run, cmd, id, disabled }. One given as cmd
     is left to the application's own data-cmd dispatch; one given as run is
     called directly. */
  function button(c, cls) {
    var b = node('button', cls, c.label); b.type = 'button';
    if (c.title) b.title = c.title;
    if (c.id) b.id = c.id;
    if (c.cmd) b.dataset.cmd = c.cmd;
    if (c.disabled) b.disabled = true;
    if (c.run) b.addEventListener('click', function (e) { c.run(e); });
    return b;
  }

  /* A raised Actions button and its menu of secondary commands. */
  function actionsMenu(commands, label) {
    var id = 'browser-actions-' + (++serial);
    var wrap = node('span', 'browser-actions');
    var open = node('button', 'btn btn-sm menu-btn', label || 'Actions');
    open.type = 'button'; open.id = id + '-button'; open.setAttribute('popovertarget', id);
    var menu = node('nav', 'menu-panel'); menu.id = id; menu.setAttribute('popover', ''); menu.dataset.menuAnchor = open.id;
    menu.setAttribute('aria-label', label || 'Actions');
    commands.forEach(function (c) {
      var item = button(c, 'menu-action');
      item.addEventListener('click', function () { if (menu.matches(':popover-open')) menu.hidePopover(); });
      menu.appendChild(item);
    });
    wrap.append(open, menu);
    return wrap;
  }

  /* toolbar({ search: {label, id, value, onInput}, scope: {label, id,
     options: [[value, label]], value, onChange}, primary: command,
     actions: [commands] }) */
  function toolbar(o) {
    var bar = node('div', 'browser-toolbar'), find = null, scope = null;
    if (o.search) {
      find = node('input', 'form-input browser-search'); find.type = 'search';
      find.placeholder = o.search.label; find.setAttribute('aria-label', o.search.label);
      if (o.search.id) find.id = o.search.id;
      find.value = o.search.value || '';
      if (o.search.onInput) find.addEventListener('input', function () { o.search.onInput(find.value); });
      bar.appendChild(find);
    }
    var row = node('div', 'browser-toolbar-row');
    if (o.scope) {
      var wrap = node('span', 'form-select-wrap browser-scope');
      scope = node('select', 'form-select'); scope.setAttribute('aria-label', o.scope.label);
      if (o.scope.id) scope.id = o.scope.id;
      o.scope.options.forEach(function (x) { scope.add(new Option(x[1], x[0])); });
      if (o.scope.value) scope.value = o.scope.value;
      if (o.scope.onChange) scope.addEventListener('change', function () { o.scope.onChange(scope.value); });
      wrap.appendChild(scope); row.appendChild(wrap);
    }
    var commands = node('span', 'browser-commands');
    if (o.primary) commands.appendChild(button(o.primary, 'btn btn-sm'));
    if (o.actions && o.actions.length) commands.appendChild(actionsMenu(o.actions));
    row.appendChild(commands);
    bar.appendChild(row);
    return { el: bar, search: find, scope: scope };
  }

  /* enhance(section, { open, onToggle(open) }) wires a group's toggle, for
     a group written in the page as well as one made by group(). */
  function enhance(section, o) {
    var toggle = section.querySelector(':scope > .browser-group-head > .browser-group-toggle');
    var title = section.querySelector(':scope > .browser-group-head > .browser-group-title');
    var body = section.querySelector(':scope > .browser-group-body');
    function set(open) { toggle.setAttribute('aria-expanded', String(open)); body.hidden = !open; section.classList.toggle('closed', !open); }
    set(!o || o.open !== false);
    if (section.dataset.wired) return set;
    section.dataset.wired = 'true';
    toggle.addEventListener('click', function () { var open = body.hidden; set(open); if (o && o.onToggle) o.onToggle(open); });
    if (title) title.addEventListener('click', function () { toggle.click(); });
    return set;
  }

  /* group({ id, title, count, location, action: command, open,
     onToggle(open), label }) returns { el, body, head, setOpen }. The
     toggle is PUDL's tree toggle, a small raised button, as in the Sources
     tree. */
  function group(o) {
    var el = node('section', 'browser-group');
    if (o.id) el.dataset.section = o.id;
    var head = node('div', 'browser-group-head');
    var toggle = node('button', 'tree-toggle browser-group-toggle'); toggle.type = 'button';
    var titleId = 'browser-group-' + (++serial);
    var title = node('h3', 'browser-group-title', o.title); title.id = titleId;
    toggle.setAttribute('aria-labelledby', titleId);
    head.append(toggle, title);
    if (o.count != null) head.appendChild(count(o.count));
    if (o.action) head.appendChild(button(o.action, 'btn btn-sm browser-group-action'));
    el.appendChild(head);
    if (o.location) { var where = node('p', 'browser-group-where', o.location); where.title = o.location; el.appendChild(where); }
    var body = node('div', 'browser-group-body'); body.setAttribute('role', 'list'); body.setAttribute('aria-labelledby', titleId);
    el.appendChild(body);
    var set = enhance(el, o);
    return { el: el, body: body, head: head, setOpen: set };
  }
  function count(n) {
    var c = node('span', 'badge browser-group-count', String(n));
    c.setAttribute('aria-label', n + (n === 1 ? ' item' : ' items'));
    return c;
  }
  /* setCount(section, n) updates a group's count badge. */
  function setCount(section, n) {
    var head = section.querySelector(':scope > .browser-group-head'), old = head.querySelector('.browser-group-count');
    var next = count(n);
    if (old) old.replaceWith(next); else head.querySelector('.browser-group-title').after(next);
  }

  /* subheading(text): a plain label inside a group, not a group of its own. */
  function subheading(text) { var h = node('div', 'md-section-label browser-subheading', text); h.setAttribute('role', 'presentation'); return h; }

  /* row({ title, meta: [lines], badge: {text, kind, label}, current,
     tooltip, clamp, onOpen, data: {key: value} }) */
  function row(o) {
    var r = node('div', 'md-row browser-row'); r.setAttribute('role', 'listitem');
    var a = node('a', 'md-item'); a.href = '#';
    var line = node('span', 'browser-row-line');
    line.appendChild(node('span', 'browser-row-title session-title', o.title));
    if (o.badge && o.badge.text != null && o.badge.text !== '') line.appendChild(badge(o.badge));
    a.appendChild(line);
    [].concat(o.meta || []).filter(Boolean).forEach(function (m) {
      var meta = node('span', 'md-meta' + (o.clamp ? ' browser-row-clamp' : ''), m); a.appendChild(meta);
    });
    if (o.tooltip) a.title = o.tooltip;
    if (o.current) a.setAttribute('aria-current', 'true');
    if (o.data) Object.keys(o.data).forEach(function (k) { a.dataset[k] = o.data[k]; });
    if (o.onOpen) a.addEventListener('click', function (e) { e.preventDefault(); o.onOpen(e); });
    r.appendChild(a);
    return r;
  }
  /* badge({ text, kind, label, cls, chip }): a status badge, whose kind
     (warn, danger, positive, accent) gives it PUDL's glyph and colour; or,
     with chip, a chip, PUDL's plain label for an attribute rather than a
     state. */
  function badge(o) {
    var b = node('span', (o.chip ? 'chip' : 'badge') + ' browser-row-badge' + (o.kind ? ' ' + o.kind : '') + (o.cls ? ' ' + o.cls : ''), String(o.text));
    if (o.glyph) b.style.setProperty('--badge-glyph', 'var(--glyph-' + o.glyph + ')');
    Object.entries(o.variables || {}).forEach(function (entry) { b.style.setProperty(entry[0], entry[1]); });
    if (o.label) { b.setAttribute('aria-label', o.label); b.title = o.label; }
    return b;
  }

  /* empty({ title, body, action: command, id }) */
  function empty(o) {
    var e = node('div', 'empty-state browser-empty');
    if (o.id) e.id = o.id;
    e.appendChild(node('p', 'empty-state-title', o.title));
    if (o.body) e.appendChild(node('p', 'empty-state-body', o.body));
    if (o.action) {
      var actions = node('div', 'empty-state-actions');
      actions.appendChild(button(o.action, 'btn btn-sm'));
      e.appendChild(actions);
    }
    return e;
  }

  window.dinahBrowserList = { toolbar: toolbar, group: group, enhance: enhance, setCount: setCount, subheading: subheading, row: row, badge: badge, empty: empty, actionsMenu: actionsMenu };
})();
