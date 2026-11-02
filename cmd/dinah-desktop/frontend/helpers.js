/* Shared page helpers. Callers choose the existing presentation of errors
   and durations so extracting these helpers does not change visible text. */
(function () {
  'use strict';

  function app() { return window.go?.main?.App; }
  function message(error, style) {
    let text = String(error && error.message || error);
    if (style === 'raw') return text;
    text = text.replace(/^Error: /, '');
    if (style === 'capital' || style === 'sentence') {
      text = text.trim();
      text = text.charAt(0).toUpperCase() + text.slice(1);
    }
    return style === 'sentence' && !/[.!?]$/.test(text) ? text + '.' : text;
  }
  function node(tag, className, text) {
    const element = document.createElement(tag);
    if (className) element.className = className;
    if (text != null) element.textContent = text;
    return element;
  }
  function num(n) { return Number(n || 0).toLocaleString('en-US'); }
  function money(c) { return c ? (Math.round(c.amount * 100) / 100).toFixed(2) + ' ' + (c.currency || '') : ''; }
  function duration(sec, minimum = 1) {
    if (sec < 60) return Math.max(minimum, sec) + ' s';
    const h = Math.floor(sec / 3600), m = Math.floor(sec % 3600 / 60), x = sec % 60;
    return h ? h + ' h ' + m + ' min' : m + ' min' + (x ? ' ' + x + ' s' : '');
  }
  function contentPct(u) { return u && u.size ? Math.round(u.used / u.size * 100) : 0; }
  function fmtDate(iso) {
    const d = iso ? new Date(iso) : null;
    const width = dinahRegional.format(new Date(2000, 11, 31, 23, 59), 'stamp').length;
    if (!d || isNaN(d)) return ' '.repeat(width);
    return dinahRegional.format(d, 'stamp').padEnd(width);
  }
  function fmtSize(n) { return n < 1024 ? n + ' B' : n < 1048576 ? (n / 1024).toFixed(1) + ' KB' : (n / 1048576).toFixed(1) + ' MB'; }

  // The host exists before any applet loads. Its listeners survive when
  // app.js adds host services and routes agent events through the scheduler.
  const handlers = new Map();
  function dispatch(name, data) {
    (handlers.get(name) || []).slice().forEach(handler => handler(data));
  }
  let route = dispatch;
  function on(name, handler) {
    if (!handlers.has(name)) {
      handlers.set(name, []);
      if (window.runtime) window.runtime.EventsOn(name, data => route(name, data));
    }
    handlers.get(name).push(handler);
    return function () {
      handlers.set(name, handlers.get(name).filter(fn => fn !== handler));
    };
  }
  /* The bridge sends dinah:resync when this page's stream reconnected and the
     engine could not replay what it missed (another engine process, or a gap
     older than its buffer). Each module that owns a view registers what it does
     about that with onResync, usually a reread of what its events carry.
     Events queued before the resync are applied first, so a reread is not
     overwritten by an older event. A resync that arrives while the handlers
     are running makes them run once more when they finish, never twice at
     once. One handler failing does not stop the others. */
  const resyncHandlers = [];
  let drainer = null, recovering = false, recoverAgain = false, recovery = Promise.resolve();
  /* settled() resolves when no recovery is running, including the one more run
     that resyncs during it asked for. A handler added while a recovery is
     running joins that recovery if it has not read the handlers yet, so anything
     that adds a handler to watch one resync waits for this first. */
  function settled() { return recovery; }
  function recover() {
    if (recovering) { recoverAgain = true; return recovery; }
    recovering = true;
    const report = err => { (window.dinahHost.fail || console.error)(err); };
    return recovery = Promise.resolve()
      .then(() => (drainer ? drainer() : null))
      .then(() => Promise.all(resyncHandlers.slice().map(handler => Promise.resolve().then(handler).catch(report))))
      .catch(report)
      .then(() => {
        recovering = false;
        if (recoverAgain) { recoverAgain = false; return recover(); }
      });
  }
  function onResync(handler) {
    if (!resyncHandlers.length) on('dinah:resync', () => { recover(); });
    resyncHandlers.push(handler);
    return function () {
      const at = resyncHandlers.indexOf(handler);
      if (at >= 0) resyncHandlers.splice(at, 1);
    };
  }
  const events = { dispatch, route: function (fn) { route = fn; }, drainer: function (fn) { drainer = fn; } };
  /* A toolbar button's content as PUDL folds it: its glyph, then its words,
     which a folded toolbar hides and keeps as the button's name and tooltip
     (pudl-toolbar.js). Returns the words, which a changing label rewrites. */
  function toolbarLabel(button, glyph, words) {
    const mark = node('span', 'glyph'), text = node('span', 'toolbar-words', words);
    mark.setAttribute('aria-hidden', 'true'); mark.style.setProperty('--glyph', 'var(--glyph-' + glyph + ')');
    button.replaceChildren(mark, text);
    return text;
  }
  /* A list kept in step with its items (FE-18). arrange puts the given
     elements in a container in order and takes out every other child. It
     moves as few elements as it can: the longest run of elements already in
     the right order stays where it is and the rest are put around it. The
     row holding focus is always in that run, so a focused row never moves,
     blurs or regains focus because another row moved past it; nothing moves
     or loses focus as live state changes. When the element with focus is
     taken out, lost, if given, is called with the place it had, so the list
     can move focus to a neighbour. */
  function focusable(element) {
    return element.matches('button, a[href], input, select, textarea, [tabindex]') ? element : element.querySelector('button, a[href], input, select, textarea, [tabindex]');
  }
  // The longest run of the given elements whose places increase, found in
  // n log n steps (patience sorting), added to stay.
  function longestRun(list, place, stay) {
    const tails = [], from = new Array(list.length);
    for (let i = 0; i < list.length; i++) {
      const p = place.get(list[i]);
      let lo = 0, hi = tails.length;
      while (lo < hi) { const mid = (lo + hi) >> 1; if (place.get(list[tails[mid]]) < p) lo = mid + 1; else hi = mid; }
      from[i] = lo > 0 ? tails[lo - 1] : -1;
      tails[lo] = i;
    }
    for (let i = tails.length ? tails[tails.length - 1] : -1; i >= 0; i = from[i]) stay.add(list[i]);
  }
  // The elements, among those already in the container, that stay in place.
  // A redraw that changes no order, the usual one, keeps them all at once.
  // Otherwise the focused row stays, with the longest run before it that
  // sits above it and the longest run after it that sits below it.
  function staying(elements, place, focused) {
    const at = elements.filter(function (el) { return place.has(el); });
    let ordered = true;
    for (let i = 1; i < at.length && ordered; i++) ordered = place.get(at[i - 1]) < place.get(at[i]);
    if (ordered) return new Set(at);
    const stay = new Set(), k = focused ? at.indexOf(focused) : -1;
    if (k < 0) { longestRun(at, place, stay); return stay; }
    const mark = place.get(focused);
    longestRun(at.slice(0, k).filter(function (el) { return place.get(el) < mark; }), place, stay);
    stay.add(focused);
    longestRun(at.slice(k + 1).filter(function (el) { return place.get(el) > mark; }), place, stay);
    return stay;
  }
  function arrange(container, elements, lost) {
    const active = document.activeElement, focus = container.contains(active) ? active : null;
    const keep = new Set(elements), before = Array.from(container.children);
    let gone = -1;
    before.forEach(function (child, i) {
      if (keep.has(child)) return;
      if (focus && child.contains(focus)) gone = i;
      child.remove();
    });
    const place = new Map();
    Array.from(container.children).forEach(function (child, i) { place.set(child, i); });
    const focusedRow = focus && elements.find(function (el) { return el.contains(focus); });
    const stay = staying(elements, place, focusedRow);
    // From the end, each element that does not stay goes before the one
    // after it, which is already in its final place.
    let next = null;
    for (let i = elements.length - 1; i >= 0; i--) {
      const el = elements[i];
      if (!stay.has(el) && (el.parentElement !== container || el.nextSibling !== next)) container.insertBefore(el, next);
      next = el;
    }
    if (focus && focus.isConnected) { if (document.activeElement !== focus) focus.focus({ preventScroll: true }); }
    else if (gone >= 0 && lost) lost(gone);
  }
  /* A caller's mistake is thrown in development and in the browser suites,
     so it is found, and only logged in a production build, so the person's
     list still draws. The suites set dinahStrict; a development build says
     what it is through the Wails runtime's Environment, as debug or dev. */
  let developing = false;
  if (window.runtime && window.runtime.Environment) Promise.resolve(window.runtime.Environment()).then(function (env) { if (env && env.buildType && env.buildType !== 'production') developing = true; }).catch(function () {});
  function mistake(text) {
    if (developing || window.dinahStrict) throw new Error(text);
    console.error(text);
  }
  /* reconcile draws items as keyed rows in a container through arrange.
     options.rows is a Map from key to a row's view, which the caller keeps
     between draws; options.key(item) is an item's key; options.create(item,
     key) makes a view whose node is the row's element; options.update(view,
     item), if given, brings a view up to date, and may give it a new node,
     which takes the old one's place and its focus, on options.focus(node)
     when given and otherwise on its first control. A key no longer listed
     loses its view unless options.keep is set, for views a caller shares
     between lists. A key given to two items is a caller's mistake (mistake).
     Returns the views in order. */
  function reconcile(container, items, options) {
    const rows = options.rows, views = [], keys = new Set();
    let refocus = null;
    items.forEach(function (item) {
      const key = options.key(item);
      // Two items with one key would place one row twice and leave the list
      // a row short; the row is drawn once and the caller's mistake shown.
      if (keys.has(key)) { mistake('dinahHelpers.reconcile: the key ' + JSON.stringify(key) + ' is given to more than one item.'); return; }
      let view = rows.get(key);
      if (!view) { view = options.create(item, key); rows.set(key, view); }
      const prior = view.node;
      if (options.update) options.update(view, item);
      if (view.node !== prior && prior.parentElement === container) {
        if (prior.contains(document.activeElement)) refocus = view.node;
        prior.replaceWith(view.node);
      }
      keys.add(key); views.push(view);
    });
    arrange(container, views.map(function (view) { return view.node; }), options.lost);
    if (refocus && refocus.isConnected) { const target = options.focus ? options.focus(refocus) : focusable(refocus); if (target) target.focus({ preventScroll: true }); }
    if (!options.keep) rows.forEach(function (view, key) { if (!keys.has(key)) rows.delete(key); });
    return views;
  }
  /* Whether a key belongs to an input method composing text (FE-45), which
     a box leaves alone: Enter confirms the text, the arrows choose a
     candidate, Escape cancels and Backspace edits it. isComposing says so
     during composition; the UI Events specification gives keyCode 229 to a
     key the input method is processing, which is how WebKit, the macOS
     build's engine, reports the Enter that confirms, after compositionend. */
  function composing(e) { return !!e.isComposing || e.keyCode === 229; }
  window.dinahHost = { on, onResync, settled };
  window.dinahHelpers = { app, message, node, num, money, duration, contentPct, fmtDate, fmtSize, events, toolbarLabel, arrange, reconcile, composing };
}());
