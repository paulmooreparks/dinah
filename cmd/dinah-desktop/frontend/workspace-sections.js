/* The order of the Workspace pane's sections (docs/plans/overnight-2026-10-31.md).
   Pinned stays first and is not moved. Every other section, the pane's own
   Chats, In Progress and Sources and any an applet adds, can be moved up and
   down: from the heading's context menu (PUDL's pudl:contextmenu, on a
   right-click, Shift+F10 or the Menu key), from the pane's Actions menu for the
   heading that last had focus, and with Alt+Up and Alt+Down on a heading. The
   order is one list of section IDs for every workspace, in the UI state the
   pane keeps its open and closed sections in; a section not in it yet, such
   as one an applet adds later, goes at the end. Moving a section moves that
   section only: focus stays on its heading and the pane does not scroll. */
(function () {
  'use strict';
  var KEY = 'dinah-workspace-section-order';
  var current = null;
  function pane() { return document.getElementById('sidebar'); }
  function anchor() { return document.getElementById('workspace-applets'); }
  function state() { return window.dinahUIState; }
  function saved() {
    var raw = state() ? state().get(KEY, '') : '';
    try { var list = raw ? JSON.parse(raw) : []; return Array.isArray(list) ? list.filter(function (id) { return typeof id === 'string'; }) : []; } catch (err) { return []; }
  }
  function save(list) { if (state()) state().set(KEY, JSON.stringify(list)); }
  /* The movable sections, in the pane's order: its own and an applet's, but
     never Pinned, which is in the frozen part at the top. */
  function movable() {
    var p = pane();
    if (!p) return [];
    return Array.from(p.querySelectorAll(':scope > .browser-group[data-section], #workspace-applets > .browser-group[data-section]'));
  }
  function nameOf(section) { var t = section.querySelector(':scope > .browser-group-head .browser-group-title'); return t ? t.textContent.trim() : 'Section'; }
  function shown(section) { return !section.hidden; }
  /* Keeps focus and every scroll position across a move of elements. */
  function keeping(fn) {
    var active = document.activeElement, scrolls = [], el = pane();
    while (el) { scrolls.push([el, el.scrollTop, el.scrollLeft]); el = el.parentElement; }
    fn();
    if (active && active.isConnected && document.activeElement !== active) active.focus({ preventScroll: true });
    scrolls.forEach(function (s) { s[0].scrollTop = s[1]; s[0].scrollLeft = s[2]; });
  }
  /* Puts the sections in the saved order, the ones it does not name after
     them in the order they are in, all just before the applets' place. */
  function apply() {
    prepare();
    var sections = movable(), order = saved(), at = anchor();
    if (!sections.length || !at) return;
    var ranked = sections.slice().sort(function (a, b) {
      var i = order.indexOf(a.dataset.section), j = order.indexOf(b.dataset.section);
      if (i < 0 && j < 0) return sections.indexOf(a) - sections.indexOf(b);
      if (i < 0) return 1;
      if (j < 0) return -1;
      return i - j;
    });
    var inPlace = ranked.every(function (s, k) { return s.parentElement === at.parentElement && (k === ranked.length - 1 ? s.nextElementSibling === at : s.nextElementSibling === ranked[k + 1]); });
    if (inPlace) return;
    keeping(function () { ranked.forEach(function (s) { at.parentElement.insertBefore(s, at); }); });
  }
  /* Writes the order the sections are in now, keeping the places of sections
     the saved order has that this pane does not show, such as another
     workspace's applet's. */
  function record(ids) {
    var now = ids || movable().map(function (s) { return s.dataset.section; }), order = saved(), queue = now.slice();
    var next = order.map(function (id) { return now.indexOf(id) >= 0 ? queue.shift() : id; });
    queue.forEach(function (id) { next.push(id); });
    save(next);
  }
  /* The section shown before or after this one, if any. */
  function neighbour(section, delta) {
    var list = movable().filter(shown), i = list.indexOf(section);
    return i < 0 ? null : list[i + delta] || null;
  }
  function canMove(section, delta) { return !!(section && section.isConnected && neighbour(section, delta)); }
  function move(section, delta) {
    var other = neighbour(section, delta);
    if (!other) return false;
    keeping(function () { other.parentElement.insertBefore(section, delta < 0 ? other : other.nextElementSibling); });
    record();
    if (window.dinahCommands) window.dinahCommands.changed();
    return true;
  }
  function sectionOf(el) {
    var head = el && el.closest && el.closest('.browser-group-head');
    var section = head && head.parentElement;
    return section && movable().indexOf(section) >= 0 ? section : null;
  }
  function rows(section) {
    var name = nameOf(section);
    return [
      { label: 'Move ' + name + ' Up', disabled: !canMove(section, -1), run: function () { move(section, -1); } },
      { label: 'Move ' + name + ' Down', disabled: !canMove(section, 1), run: function () { move(section, 1); } }
    ];
  }
  /* Reorder Sections (Paul's decision of 2 November, the second path): the
     pane is PUDL's reorderable list (pudl-reorder.js, 0.97.0). Its frozen
     top, with the toolbar and Pinned, and the applets' place at its end are
     fixed, so they keep their places; every other section is an item, whose
     grip stands where its expand toggle does. While the mode is on, every
     section's body is hidden (app.css, data-reorder-active), so each comes
     back as it was when the mode ends; Done and Escape end it. A move
     saves the order before PUDL makes it, so the pane's own ordering
     (apply) agrees with the move and never undoes it. */
  function prepare() {
    var p = pane();
    if (!p) return;
    if (!p.hasAttribute('data-reorder')) { p.setAttribute('data-reorder', 'off'); p.setAttribute('data-reorder-done', 'ws-reorder-done'); }
    p.querySelectorAll(':scope > .ws-frozen, :scope > #workspace-applets').forEach(function (el) { el.setAttribute('data-reorder-fixed', ''); });
    movable().forEach(function (section) {
      section.setAttribute('data-reorder-id', section.dataset.section);
      section.setAttribute('data-reorder-label', nameOf(section));
      var head = section.querySelector(':scope > .browser-group-head');
      if (head && !head.querySelector(':scope > [data-reorder-grip]')) {
        var slot = document.createElement('span'); slot.className = 'section-grip'; slot.setAttribute('data-reorder-grip', '');
        head.insertBefore(slot, head.firstChild);
      }
    });
  }
  function reordering() { var p = pane(); return !!p && p.getAttribute('data-reorder') === 'on'; }
  function startReorder() {
    var p = pane();
    if (!p || !window.pudlReorder || reordering()) return;
    prepare();
    window.pudlReorder.on(p);
  }
  function start() {
    var p = pane();
    if (!p) return;
    prepare();
    p.addEventListener('pudl:reorder', function (e) {
      var ids = movable().map(function (s) { return s.dataset.section; });
      record((e.detail.order || []).filter(function (id) { return ids.indexOf(id) >= 0; }));
      if (window.dinahCommands) window.dinahCommands.changed();
    });
    var lastGrip = null;
    p.addEventListener('focusin', function (e) { if (e.target.matches && e.target.matches('.reorder-grip')) lastGrip = e.target.closest('.browser-group[data-section]'); });
    p.addEventListener('pudl:reorder-mode', function (e) {
      var bar = p.querySelector('.ws-reorder-bar'), on = !!(e.detail && e.detail.on);
      if (bar) bar.hidden = !on;
      if (on) {
        var first = p.querySelector('.reorder-grip');
        if (first) first.focus({ preventScroll: true });
      } else {
        // Focus goes back to the heading whose grip had it, or to Actions.
        var toggle = lastGrip && lastGrip.isConnected && lastGrip.querySelector(':scope > .browser-group-head .browser-group-toggle');
        (toggle || document.getElementById('ws-actions-button')).focus({ preventScroll: true });
        lastGrip = null;
      }
      if (window.dinahCommands) window.dinahCommands.changed();
    });
    // The heading's menu: its context menu, which PUDL opens on a right-click,
    // Shift+F10 or the Menu key.
    p.addEventListener('pudl:contextmenu', function (e) {
      var section = sectionOf(e.target);
      if (!section || e.detail.count()) return;
      e.detail.add(rows(section));
      e.detail.mark(section.querySelector(':scope > .browser-group-head'));
    });
    // Alt+Up and Alt+Down on a heading move its section.
    p.addEventListener('keydown', function (e) {
      if (!e.altKey || e.ctrlKey || e.metaKey || e.shiftKey || (e.key !== 'ArrowUp' && e.key !== 'ArrowDown')) return;
      var section = sectionOf(e.target);
      if (!section) return;
      e.preventDefault();
      move(section, e.key === 'ArrowUp' ? -1 : 1);
    });
    // The heading that last had focus is the one Actions moves.
    p.addEventListener('focusin', function (e) {
      if (e.target.closest('#ws-actions, #ws-actions-button')) return;
      var section = sectionOf(e.target);
      if (section !== current) { current = section; if (window.dinahCommands) window.dinahCommands.changed(); }
    });
    if (window.dinahCommands) {
      window.dinahCommands.register({ id: 'workspace-section-up', menu: 'view', inline: true,
        label: function () { return current ? 'Move ' + nameOf(current) + ' Up' : 'Move Section Up'; },
        enabled: function () { return canMove(current, -1); }, run: function () { if (current) move(current, -1); } });
      window.dinahCommands.register({ id: 'workspace-section-down', menu: 'view', inline: true,
        label: function () { return current ? 'Move ' + nameOf(current) + ' Down' : 'Move Section Down'; },
        enabled: function () { return canMove(current, 1); }, run: function () { if (current) move(current, 1); } });
      window.dinahCommands.register({ id: 'workspace-sections-reorder', menu: 'view', inline: true, label: 'Reorder Sections',
        enabled: function () { return !!window.pudlReorder && !reordering() && movable().filter(shown).length > 1; }, run: startReorder });
    }
    // Actions draws its rows for the heading that last had focus as it opens.
    var actions = document.getElementById('ws-actions');
    if (actions && window.dinahCommands) actions.addEventListener('beforetoggle', function (e) { if (e.newState === 'open') window.dinahCommands.changed(); });
    apply();
    // A section added or brought back, such as an applet's, takes its place.
    new MutationObserver(function () { apply(); }).observe(p, { childList: true });
    var applets = anchor();
    if (applets) new MutationObserver(function () { apply(); }).observe(applets, { childList: true });
    document.addEventListener('dinah:ui-state-restored', apply);
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', start); else start();
  window.dinahWorkspaceSections = { apply: apply, move: move, order: saved };
})();
