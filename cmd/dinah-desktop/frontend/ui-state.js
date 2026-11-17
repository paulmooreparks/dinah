/* Native settings store UI preferences across browser profiles and upgrades.
   Named view state is stored here, with the unsent drafts of chats, groups
   and threads (composer.js, keeper), which Paul decided on 28 October 2026
   should outlast a restart. Other form values and secrets are excluded. The
   store is ui-state.json in Dinah's home folder (ui_state.go), never in a
   workspace. */
(function () {
  'use strict';
  /* Each value is kept as its JSON text, and the keys changed since the last
     save are remembered. The whole store used to be copied and sent on every
     save, and compared twice more afterwards; a long-used installation's
     store is some 600 KB in 600 values, so bringing one window forward
     serialised megabytes on the page's thread (measured 17 November 2026,
     docs/design/scaling.md). A save now sends the changed values alone
     (SaveUIStateChanges, ui_state.go), and reading a value parses that one
     value. */
  var texts = {}, dirty = new Set(), loaded = false, writable = false, restoring = true;
  var saving = null, timer = null, mode = 'floating', kindModes = {}, modeOf = {}, initializationError = '';
  var originalSearch = location.search, scrolls = new Map();
  const backend = window.dinahHelpers.app;
  function has(key) { return Object.prototype.hasOwnProperty.call(texts, key); }
  function take(values) { texts = {}; Object.keys(values).forEach(function (key) { texts[key] = JSON.stringify(values[key]); }); }
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
  function get(key, fallback) { return has(key) ? JSON.parse(texts[key]) : fallback; }
  function set(key, value) {
    if (!loaded || restoring) return;
    var text = value === undefined ? undefined : JSON.stringify(value);
    if (text === (has(key) ? texts[key] : undefined)) return;
    if (text === undefined) delete texts[key]; else texts[key] = text;
    if (!backend() || !backend().ReadUIState) {
      try {
        if (value === undefined) localStorage.removeItem(key);
        else localStorage.setItem(key, typeof value === 'string' ? value : JSON.stringify(value));
      } catch (error) { /* Browser-only previews can disable storage. */ }
    }
    dirty.add(key);
    clearTimeout(timer); timer = setTimeout(function () { flush().catch(report); }, 150);
  }
  function flush() {
    clearTimeout(timer);
    if (initializationError) return Promise.reject(new Error(initializationError));
    if (!loaded || !writable || !dirty.size) return saving || Promise.resolve();
    if (saving) return saving.then(flush);
    var keys = Array.from(dirty), changes = { version: 1, set: {}, remove: [] };
    dirty.clear();
    keys.forEach(function (key) { if (has(key)) changes.set[key] = JSON.parse(texts[key]); else changes.remove.push(key); });
    saving = Promise.resolve().then(function () { return backend().SaveUIStateChanges(changes); }).catch(function (error) {
      // What was not saved is still to be saved, whatever has changed since.
      keys.forEach(function (key) { dirty.add(key); }); throw error;
    }).finally(function () { saving = null; });
    return saving.then(function () { if (dirty.size) return flush(); });
  }
  var finishReady, started = false;
  var ready = new Promise(function (resolve) { finishReady = resolve; });
  function initialize() {
    if (started) return ready;
    if (!backend()) return Promise.reject(new Error('The application bridge is not ready.'));
    started = true;
    Promise.resolve().then(function () {
    var app = backend();
    if (!app || !app.ReadUIState) { take(legacy()); return; }
    return Promise.resolve(app.ReadUIState()).then(function (state) {
      if (!state || state.version !== 1 || !state.values || Array.isArray(state.values)) throw new Error('The saved UI state has an unsupported format.');
      take(state.values); writable = !!app.SaveUIStateChanges;
      // An empty store takes over what an older build kept in the browser, and saves all of it.
      if (!Object.keys(texts).length) { take(legacy()); Object.keys(texts).forEach(function (key) { dirty.add(key); }); }
    });
    }).catch(function (error) { initializationError = String(error); report(error); }).then(function () {
    loaded = true;
    document.dispatchEvent(new CustomEvent('dinah:ui-state-ready'));
    finishReady();
    });
    return ready;
  }
  if (backend()) initialize();
  /* A product built from this source registers applets of its own from
     files loaded after the page's (product-applets.js). The saved layout
     may hold their windows, and a window whose applet is not registered
     yet is left out of the restored layout and then out of the saved one,
     so such a window was gone after every restart (Paul, 11 November
     2026). The layout is therefore restored once those files have loaded.
     A build with none waits only to learn that there are none. Until then
     the page is still restoring, so nothing is saved over the layout. */
  function productAppletsLoaded() {
    var parsed = document.readyState === 'loading'
      ? new Promise(function (resolve) { document.addEventListener('DOMContentLoaded', resolve, { once: true }); })
      : Promise.resolve();
    return parsed.then(function () { return window.dinahProductApplets ? window.dinahProductApplets.ready : null; }).catch(function () {});
  }
  var layoutRestore = null, layoutDrawn = null;
  function restoreLayout() {
    if (!layoutRestore) layoutRestore = productAppletsLoaded().then(restoreLayoutNow);
    return layoutRestore;
  }
  /* The start screen stays in front of the page until the saved layout is
     in place (app.css, .start-screen). Before this, the page was shown as
     soon as it was parsed, with the default windows in the default docks,
     and about a second later everything moved to where the saved layout
     has it (Paul, 17 November 2026).

     The layout is in place once PUDL has announced the arrangement that
     restoreLayoutNow put in the address (pudl:windows-change, which PUDL's
     CONTRACT.md says follows every change to the windows) and the frames
     that follow it have been drawn: the taskbar folds its tray one frame
     after its buttons arrive. An arrangement that changes nothing is
     announced by nothing, so the wait is bounded; the page is then already
     as the address has it.

     The bottom bar goes on changing for a few frames after that, and does
     with no saved layout too: the taskbar's pinned buttons and its arrange
     button are added, and status items are shown as their counts arrive.
     So the layout counts as drawn once nothing has been added to the bar,
     taken from it, shown or hidden for a few frames running, which is
     watched for and not guessed at, and that wait is bounded as well. */
  var SETTLE_LIMIT = 2000, FRAME_LIMIT = 500, QUIET_FRAMES = 4, QUIET_LIMIT = 1200;
  function barQuiet() {
    var bar = document.querySelector('.win-bar');
    if (!bar) return frames(2);
    return new Promise(function (resolve) {
      var left = QUIET_FRAMES, done = false;
      var observer = new MutationObserver(function () { left = QUIET_FRAMES; });
      observer.observe(bar, { childList: true, subtree: true, attributes: true, attributeFilter: ['hidden'] });
      var timer = setTimeout(finish, QUIET_LIMIT); // a window out of sight draws no frames
      function finish() { if (done) return; done = true; clearTimeout(timer); observer.disconnect(); resolve(); }
      (function next() {
        if (done) return;
        if (left-- <= 0) { finish(); return; }
        requestAnimationFrame(next);
      })();
    });
  }
  function frames(count) {
    return new Promise(function (resolve) {
      var timer = setTimeout(resolve, FRAME_LIMIT); // a window out of sight draws no frames
      (function next() {
        if (count-- <= 0) { clearTimeout(timer); resolve(); return; }
        requestAnimationFrame(next);
      })();
    });
  }
  function awaitArrangement() {
    return new Promise(function (resolve) {
      var timer = setTimeout(done, SETTLE_LIMIT);
      function done() { clearTimeout(timer); document.removeEventListener('pudl:windows-change', done); resolve(); }
      document.addEventListener('pudl:windows-change', done);
    }).then(barQuiet);
  }
  /* Resolves when the layout the page starts with has been drawn. A page
     whose layout was never restored, because its state could not be read,
     is shown as it is. */
  function layoutSettled() {
    return Promise.resolve(layoutRestore).then(function () { return layoutDrawn; }).catch(function () {});
  }
  function restoreLayoutNow() {
    var saved = get('dinah-workbench-layout');
    // The other listeners of the event below set the bar up; the quiet is looked for after them.
    layoutDrawn = frames(1).then(barQuiet);
    if (!originalSearch && typeof saved === 'string') {
      try {
        var query = new URLSearchParams(saved), opened = (query.get('open') || '').split(',');
        var kept = opened.filter(function (key) { var item = window.dinahWorkbench && window.dinahWorkbench.item(key); return key !== 'chat' && (!item || item.kind !== 'chat'); });
        if (kept.length !== opened.length) {
          query.set('open', kept.filter(Boolean).join(',') || 'workspace');
          if (opened.includes(query.get('top')) && !kept.includes(query.get('top'))) query.delete('top');
          if (query.has('z.order')) query.set('z.order', query.get('z.order').split(',').filter(function (key) { return kept.includes(key); }).join(','));
          saved = '?' + query.toString();
        }
        history.replaceState(history.state, '', location.pathname + saved + location.hash);
        layoutDrawn = awaitArrangement(); // listening before PUDL is told of the address
        window.dispatchEvent(new PopStateEvent('popstate'));
      } catch (error) { report(error); }
    }
    restoring = false;
    document.documentElement.dataset.uiStateReady = 'true';
    document.dispatchEvent(new CustomEvent('dinah:ui-state-restored', { detail: { explicit: !!originalSearch, restored: !originalSearch && typeof saved === 'string' } }));
    refreshScroll(document);
    if (dirty.size) flush().catch(report);
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
    Object.keys(texts).forEach(function (key) {
      if (key.startsWith(oldPrefix)) { set(newPrefix + key.slice(oldPrefix.length), JSON.parse(texts[key])); set(key, undefined); }
    });
    scrolls.forEach(function (entry) { if (entry.key.startsWith(oldPrefix)) entry.key = newPrefix + entry.key.slice(oldPrefix.length); });
  }
  window.dinahUIState = {
    ready: ready, initialize: initialize, get: get, set: set, flush: flush, restoreLayout: restoreLayout, layoutSettled: layoutSettled,
    initializationError: function () { return initializationError; },
    setNewWindowMode: function (value) { mode = value === 'maximized' ? value : 'floating'; },
    newWindowMode: function () { return mode; },
    /* A kind of window with a choice of its own opens that way; "same", or
       no choice, follows Open New Windows. */
    setWindowModes: function (modes) { kindModes = modes && typeof modes === 'object' ? Object.assign({}, modes) : {}; },
    windowMode: function (kind) { var own = kindModes[modeOf[kind] || kind]; return own === 'floating' || own === 'maximized' ? own : mode; },
    /* A kind that opens as another kind's choice says (the content
       applets' windows follow the Editor's). */
    followWindowMode: function (kind, other) { modeOf[kind] = other; },
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
