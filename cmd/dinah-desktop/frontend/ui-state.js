/* Native settings store UI preferences across browser profiles and upgrades.
   Named view state is stored here, with the unsent drafts of chats, groups
   and threads (composer.js, keeper), which Paul decided on 28 October 2026
   should outlast a restart. Other form values and secrets are excluded. The
   store is ui-state.json in Dinah's home folder (ui_state.go), never in a
   workspace. */
(function () {
  'use strict';
  var values = {}, loaded = false, writable = false, restoring = true;
  var pending = false, saving = null, timer = null, mode = 'floating', kindModes = {}, initializationError = '';
  var originalSearch = location.search, scrolls = new Map();
  const backend = window.dinahHelpers.app;
  function clone(value) { return JSON.parse(JSON.stringify(value)); }
  function report(error) { document.dispatchEvent(new CustomEvent('dinah:ui-state-error', { detail: String(error) })); }
  function legacy() {
    var data = {};
    try {
      ['dinah-workbench-layout', 'pudl-theme'].forEach(function (key) {
        var value = localStorage.getItem(key); if (value != null) data[key] = value;
      });
      for (var i = 0; i < localStorage.length; i++) {
        var key = localStorage.key(i);
        if (/^dinah-section:/.test(key)) data[key] = localStorage.getItem(key);
        if (key === 'dinah-window-placements' || /^dinah-scroll:/.test(key)) {
          try { data[key] = JSON.parse(localStorage.getItem(key)); } catch (error) { /* Ignore an invalid view preference. */ }
        }
      }
    } catch (error) { /* Browser storage is optional. */ }
    return data;
  }
  function get(key, fallback) { return Object.prototype.hasOwnProperty.call(values, key) ? clone(values[key]) : fallback; }
  function set(key, value) {
    if (!loaded || restoring) return;
    if (JSON.stringify(values[key]) === JSON.stringify(value)) return;
    if (value === undefined) delete values[key]; else values[key] = clone(value);
    if (!backend() || !backend().ReadUIState) {
      try {
        if (value === undefined) localStorage.removeItem(key);
        else localStorage.setItem(key, typeof value === 'string' ? value : JSON.stringify(value));
      } catch (error) { /* Browser-only previews can disable storage. */ }
    }
    pending = true;
    clearTimeout(timer); timer = setTimeout(function () { flush().catch(report); }, 150);
  }
  function flush() {
    clearTimeout(timer);
    if (initializationError) return Promise.reject(new Error(initializationError));
    if (!loaded || !writable || !pending) return saving || Promise.resolve();
    if (saving) return saving.then(flush);
    pending = false;
    var snapshot = { version: 1, values: clone(values) };
    saving = Promise.resolve().then(function () { return backend().SaveUIState(snapshot); }).catch(function (error) {
      pending = true; throw error;
    }).finally(function () { saving = null; });
    return saving.then(function () { if (pending && JSON.stringify(values) !== JSON.stringify(snapshot.values)) return flush(); });
  }
  var finishReady, started = false;
  var ready = new Promise(function (resolve) { finishReady = resolve; });
  function initialize() {
    if (started) return ready;
    if (!backend()) return Promise.reject(new Error('The application bridge is not ready.'));
    started = true;
    Promise.resolve().then(function () {
    var app = backend();
    if (!app || !app.ReadUIState) { values = legacy(); return; }
    return Promise.resolve(app.ReadUIState()).then(function (state) {
      if (!state || state.version !== 1 || !state.values || Array.isArray(state.values)) throw new Error('The saved UI state has an unsupported format.');
      values = state.values; writable = !!app.SaveUIState;
      if (!Object.keys(values).length) { values = legacy(); pending = !!Object.keys(values).length; }
    });
    }).catch(function (error) { initializationError = String(error); report(error); }).then(function () {
    loaded = true;
    document.dispatchEvent(new CustomEvent('dinah:ui-state-ready'));
    finishReady();
    });
    return ready;
  }
  if (backend()) initialize();
  function restoreLayout() {
    var saved = get('dinah-workbench-layout');
    if (!originalSearch && typeof saved === 'string') {
      try {
        history.replaceState(history.state, '', location.pathname + saved + location.hash);
        window.dispatchEvent(new PopStateEvent('popstate'));
      } catch (error) { report(error); }
    }
    restoring = false;
    document.documentElement.dataset.uiStateReady = 'true';
    document.dispatchEvent(new CustomEvent('dinah:ui-state-restored', { detail: { explicit: !!originalSearch, restored: !originalSearch && typeof saved === 'string' } }));
    refreshScroll(document);
    if (pending) flush().catch(report);
  }
  /* An entry reads its saved position once the saved state is loaded and
     the layout has been restored. Until then it may neither restore nor
     save: a chat restored with the layout can scroll itself to its end
     before that, and saving then wrote "at the end" over the reader's kept
     place, which a busy machine made likely (Paul, 26 October 2026). */
  function readSaved(entry) {
    if (entry.initialized) return true;
    if (!loaded || restoring) return false;
    entry.saved = get(entry.key);
    entry.pending = !!entry.saved && Number.isFinite(entry.saved.top) && Number.isFinite(entry.saved.left);
    entry.initialized = true;
    return true;
  }
  function saveScroll(entry) {
    if (!readSaved(entry) || entry.pending || !entry.el.isConnected || !entry.el.clientHeight) return;
    set(entry.key, { top: entry.el.scrollTop, left: entry.el.scrollLeft,
      end: !!entry.followEnd && entry.el.scrollHeight - entry.el.clientHeight - entry.el.scrollTop < 20 });
  }
  /* Restoring a scroll position reads and writes the element's layout.
     Content arrives in many pieces, each of which asks for a restore, so
     the restores are gathered and done once a frame; doing one for every
     piece laid the page out for every piece. */
  var restoreQueue = new Set(), restoreFrame = 0;
  function restoreScroll(entry) {
    if (!readSaved(entry) || !entry.pending) return;
    restoreQueue.add(entry);
    if (!restoreFrame) restoreFrame = requestAnimationFrame(restoreQueued);
  }
  function restoreQueued() {
    restoreFrame = 0;
    var batch = Array.from(restoreQueue);
    restoreQueue.clear();
    batch.forEach(restoreNow);
  }
  function restoreNow(entry) {
    if (!entry.pending || !entry.el.isConnected || !entry.el.clientHeight) return;
    var el = entry.el, value = entry.saved;
    el.scrollTop = value.end && entry.followEnd ? el.scrollHeight : value.top;
    el.scrollLeft = value.left;
    // A list that follows its end is told where the restore put the view, since that move is not the reader's.
    el.dispatchEvent(new CustomEvent('dinah:scroll-restored'));
    var reached = (value.end || Math.abs(el.scrollTop - value.top) < 2) && Math.abs(el.scrollLeft - value.left) < 2;
    if (reached && !entry.defer) entry.pending = false;
  }
  function watchScroll(el, key, options) {
    if (scrolls.has(el)) return;
    options = options || {};
    var entry = { el: el, key: key, pending: false, initialized: false, defer: !!options.defer, followEnd: !!options.followEnd };
    scrolls.set(el, entry);
    function input(event) {
      if (event.type === 'keydown' && !['ArrowUp', 'ArrowDown', 'PageUp', 'PageDown', 'Home', 'End', ' '].includes(event.key)) return;
      entry.pending = false; entry.defer = false; entry.initialized = true;
    }
    ['wheel', 'touchstart', 'pointerdown', 'keydown'].forEach(function (name) { el.addEventListener(name, input, { passive: true }); });
    function scrolled() { saveScroll(entry); }
    el.addEventListener('scroll', scrolled, { passive: true });
    var mutation = new MutationObserver(function () { restoreScroll(entry); });
    mutation.observe(el, { childList: true, subtree: true, characterData: true });
    var resize = new ResizeObserver(function () { restoreScroll(entry); }); resize.observe(el);
    entry.destroy = function () {
      saveScroll(entry); mutation.disconnect(); resize.disconnect(); scrolls.delete(el);
      ['wheel', 'touchstart', 'pointerdown', 'keydown'].forEach(function (name) { el.removeEventListener(name, input); });
      el.removeEventListener('scroll', scrolled);
    };
    restoreScroll(entry);
  }
  function refreshScroll(root) {
    scrolls.forEach(function (entry) { if (root === entry.el || root.contains(entry.el)) restoreScroll(entry); });
  }
  /* The element's content is complete: its position is restored now,
     before the frame, so the person never sees it jump. */
  function scrollReady(el) {
    var entry = scrolls.get(el);
    if (!entry) return;
    entry.defer = false;
    restoreScroll(entry);
    restoreQueue.delete(entry);
    restoreNow(entry);
  }
  function releaseScroll(el) {
    var entry = scrolls.get(el); if (entry) { entry.pending = false; entry.defer = false; }
  }
  function migratePrefix(oldPrefix, newPrefix) {
    Object.keys(values).forEach(function (key) {
      if (key.startsWith(oldPrefix)) { set(newPrefix + key.slice(oldPrefix.length), values[key]); set(key, undefined); }
    });
    scrolls.forEach(function (entry) { if (entry.key.startsWith(oldPrefix)) entry.key = newPrefix + entry.key.slice(oldPrefix.length); });
  }
  window.dinahUIState = {
    ready: ready, initialize: initialize, get: get, set: set, flush: flush, restoreLayout: restoreLayout,
    initializationError: function () { return initializationError; },
    setNewWindowMode: function (value) { mode = value === 'maximized' ? value : 'floating'; },
    newWindowMode: function () { return mode; },
    /* A kind of window with a choice of its own opens that way; "same", or
       no choice, follows Open New Windows. */
    setWindowModes: function (modes) { kindModes = modes && typeof modes === 'object' ? Object.assign({}, modes) : {}; },
    windowMode: function (kind) { var own = kindModes[kind]; return own === 'floating' || own === 'maximized' ? own : mode; },
    isRestoring: function () { return restoring; },
    watchScroll: watchScroll, refreshScroll: refreshScroll, scrollReady: scrollReady, releaseScroll: releaseScroll,
    restorePending: function (el) { var entry = scrolls.get(el); if (entry) restoreScroll(entry); return !!entry && entry.pending; },
    /* Restores what is waiting now, for a caller that is about to read
       the element's position. */
    restoreNow: function (el) { var entry = scrolls.get(el); if (entry) { restoreQueue.delete(entry); restoreNow(entry); } },
    detachScroll: function (root) { scrolls.forEach(function (entry) { if (root === entry.el || root.contains(entry.el)) entry.destroy(); }); },
    migratePrefix: migratePrefix
  };
  document.addEventListener('visibilitychange', function () { if (document.visibilityState === 'hidden') flush().catch(report); });
  window.addEventListener('pagehide', function () { flush().catch(report); });
})();
