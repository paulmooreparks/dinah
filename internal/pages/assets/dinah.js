/* Dinah's pages script. It does five things, and no act depends on any of
   them: with script off a page is correct when drawn and stale until
   reloaded, and every form still works.

   1. Polling. Every three seconds while the page is visible it asks the
      changes route, with the cursor the page was drawn at, whether anything
      happened, and redraws when something did.
   2. Redrawing. It fetches the page's own URL and replaces each region that
      carries data-live with the same region of the new document, leaving
      alone any region holding the focus or a control the reader has changed.
      The windows themselves, their placement and their chrome are never
      replaced, so the windows script's state is not disturbed. A set of
      tabs keeps the tab the reader chose, and after a redraw it announces
      pudl:regions-swap, on which PUDL's windows script marks the new rows
      and its tabs script readies the new tabs.
   3. Keys. The / key, pressed outside a text control, focuses the command
      line.
   4. Chrome. It inserts the theme toggle, which only script could work, and
      a copy button beside each command line, inside windows the windows
      script fetches as well. Every string it writes is read from an
      attribute the server filled from the catalog.
   5. Layout. PUDL's windows script shows the list pane of a narrow layout
      whenever no window is open, which is right for the board and wrong for
      a page that is itself a detail, such as a card or the card list, so
      the pane the server drew, in data-page-pane, is put back. The width
      the reader drags the column list to is remembered in this browser. */
(function () {
  'use strict';

  var POLL_MS = 3000;
  var WIDTH_KEY = 'dinah-md-sidebar-w';
  var cursor = '';
  var timer = 0;
  var busy = false;

  function layout() { return document.querySelector('.md-layout'); }

  /* A region holding the focus, or a control whose value differs from the
     one it was drawn with, is the reader's work in progress. */
  function busyRegion(el) {
    if (el.contains(document.activeElement) && document.activeElement !== document.body) return true;
    var controls = el.querySelectorAll('input, textarea, select');
    for (var i = 0; i < controls.length; i++) {
      var c = controls[i];
      if (c.type === 'hidden') continue;
      if (c.tagName === 'SELECT') {
        /* A select whose options carry no selected attribute starts on its
           first option, which is its drawn value although that option's
           defaultSelected is false. */
        var drawn = [];
        for (var j = 0; j < c.options.length; j++) drawn.push(c.options[j].defaultSelected);
        if (!c.multiple && c.size <= 1 && drawn.indexOf(true) < 0 && drawn.length) drawn[0] = true;
        for (var k = 0; k < c.options.length; k++) {
          if (c.options[k].selected !== drawn[k]) return true;
        }
      } else if (c.type === 'checkbox' || c.type === 'radio') {
        if (c.checked !== c.defaultChecked) return true;
      } else if (c.value !== c.defaultValue) {
        return true;
      }
    }
    return false;
  }

  /* A set of tabs in a region about to be replaced keeps the reader's
     choice: each fresh tab list is marked with the tab chosen in the old. */
  function keepTabs(old, fresh) {
    old.querySelectorAll('.tabs-ready > [role="tablist"] > [role="tab"][aria-selected="true"]').forEach(function (chosen) {
      var id = chosen.getAttribute('aria-controls');
      var twin = id && fresh.querySelector('[role="tab"][aria-controls="' + CSS.escape(id) + '"]');
      if (!twin) return;
      Array.prototype.forEach.call(twin.parentElement.children, function (tab) {
        if (tab.getAttribute('role') === 'tab') tab.setAttribute('aria-selected', tab === twin ? 'true' : 'false');
      });
    });
  }

  /* The live regions of a document outside the window layer, keyed by their
     data-live value and position, and each window's body keyed by its key. */
  function regions(doc) {
    var found = {};
    var counts = {};
    doc.querySelectorAll('[data-live]').forEach(function (el) {
      var name = el.getAttribute('data-live');
      var win = el.closest('.win[data-win]');
      var key;
      if (win) {
        key = 'win:' + win.getAttribute('data-win');
      } else {
        counts[name] = (counts[name] || 0) + 1;
        key = name + ':' + counts[name];
      }
      found[key] = el;
    });
    return found;
  }

  function redraw() {
    if (busy) return Promise.resolve();
    busy = true;
    return fetch(location.href, { headers: { Accept: 'text/html' }, cache: 'no-store', credentials: 'same-origin' })
      .then(function (r) {
        if (!r.ok) throw new Error('redraw answered ' + r.status);
        return r.text();
      })
      .then(function (html) {
        var fresh = new DOMParser().parseFromString(html, 'text/html');
        var now = regions(document);
        var next = regions(fresh);
        var swapped = [];
        Object.keys(now).forEach(function (key) {
          var replacement = next[key];
          if (!replacement || busyRegion(now[key])) return;
          var incoming = document.importNode(replacement, true);
          keepTabs(now[key], incoming);
          now[key].replaceWith(incoming);
          swapped.push(key);
        });
        var freshLayout = fresh.querySelector('.md-layout');
        if (freshLayout) cursor = freshLayout.getAttribute('data-changes-cursor') || cursor;
        addCopyButtons(document);
        if (swapped.length) {
          document.dispatchEvent(new CustomEvent('pudl:regions-swap', { detail: { url: location.href, regions: swapped } }));
        }
      })
      .catch(function () { /* the next change redraws it */ })
      .then(function () { busy = false; });
  }

  function poll() {
    clearTimeout(timer);
    if (document.visibilityState !== 'visible' || !cursor) return;
    fetch('/changes?since=' + encodeURIComponent(cursor), {
      headers: { Accept: 'application/json' }, cache: 'no-store', credentials: 'same-origin'
    })
      .then(function (r) {
        if (r.status === 400) { location.reload(); return null; }
        return r.ok ? r.json() : null;
      })
      .then(function (answer) {
        if (!answer) return null;
        var changed = answer.changed;
        if (answer.cursor) cursor = answer.cursor;
        return changed ? redraw() : null;
      })
      .catch(function () { /* a network failure waits for the next tick */ })
      .then(function () { timer = setTimeout(poll, POLL_MS); });
  }

  function onKey(e) {
    if (e.key !== '/' || e.ctrlKey || e.metaKey || e.altKey) return;
    var t = e.target;
    if (t && (t.isContentEditable || /^(INPUT|TEXTAREA|SELECT)$/.test(t.tagName))) return;
    var line = document.getElementById('cmdline');
    if (!line) return;
    e.preventDefault();
    line.focus();
  }

  function insertThemeToggle() {
    var chrome = document.querySelector('.topbar-chrome');
    if (!chrome || chrome.querySelector('[data-theme-toggle]') || typeof window.pudlToggleTheme !== 'function') return;
    var button = document.createElement('button');
    button.className = 'theme-toggle';
    button.type = 'button';
    button.setAttribute('data-theme-toggle', '');
    button.setAttribute('aria-label', chrome.getAttribute('data-theme-label') || '');
    button.textContent = '◐';
    button.addEventListener('click', function () { window.pudlToggleTheme(); });
    chrome.appendChild(button);
  }

  /* The pane a narrow layout shows: the server's detail pane stays a
     detail pane whatever the windows script decided. */
  function keepPane() {
    var l = layout();
    if (l && l.getAttribute('data-page-pane') === 'detail') l.setAttribute('data-md-pane', 'detail');
  }

  /* The column list's width, as the reader last dragged it. Storage that is
     blocked or full only means the width is not remembered. */
  function restoreWidth() {
    var l = layout();
    if (!l) return;
    var width = null;
    try { width = localStorage.getItem(WIDTH_KEY); } catch (e) { /* not remembered */ }
    if (width && /^\d+(\.\d+)?$/.test(width)) l.style.setProperty('--md-sidebar-w', width + 'px');
    l.addEventListener('pudl:md-resize', function (e) {
      try {
        if (e.detail.reset) localStorage.removeItem(WIDTH_KEY);
        else localStorage.setItem(WIDTH_KEY, String(Math.round(e.detail.width)));
      } catch (err) { /* not remembered */ }
    });
  }

  function copyLabel() {
    var holder = document.querySelector('[data-copy-label]');
    return holder ? holder.getAttribute('data-copy-label') : '';
  }

  function addCopyButtons(root) {
    if (!navigator.clipboard) return;
    var label = copyLabel();
    root.querySelectorAll('code.cmdlog-cmd').forEach(function (code) {
      if (code.nextElementSibling && code.nextElementSibling.hasAttribute('data-copy')) return;
      var button = document.createElement('button');
      button.className = 'btn btn-sm';
      button.type = 'button';
      button.setAttribute('data-copy', '');
      button.textContent = label;
      button.addEventListener('click', function () { navigator.clipboard.writeText(code.textContent); });
      code.insertAdjacentElement('afterend', button);
    });
  }

  function init() {
    var l = layout();
    if (l) cursor = l.getAttribute('data-changes-cursor') || '';
    insertThemeToggle();
    restoreWidth();
    keepPane();
    document.addEventListener('pudl:windows-change', keepPane);
    addCopyButtons(document);
    document.addEventListener('keydown', onKey);
    document.addEventListener('pudl:window-open', function (e) { addCopyButtons(e.target); });
    document.addEventListener('visibilitychange', function () {
      if (document.visibilityState === 'visible') poll();
    });
    timer = setTimeout(poll, POLL_MS);
  }

  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', init);
  else init();
})();
