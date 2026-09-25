/* Dinah's pages script. It does four things, and no act depends on any of
   them: with script off a page is correct when drawn and stale until
   reloaded, and every form still works.

   1. Polling. Every three seconds while the page is visible it asks the
      changes route, with the cursor the page was drawn at, whether anything
      happened, and redraws when something did.
   2. Redrawing. It fetches the page's own URL and replaces each region that
      carries data-live with the same region of the new document, leaving
      alone any region holding the focus or a control the reader has changed.
      The windows themselves, their placement and their chrome are never
      replaced, so the windows script's state is not disturbed.
   3. Keys. The / key, pressed outside a text control, focuses the command
      line.
   4. Chrome. It inserts the theme toggle, which only script could work, and
      a copy button beside each command line, inside windows the windows
      script fetches as well. Every string it writes is read from an
      attribute the server filled from the catalog. */
(function () {
  'use strict';

  var POLL_MS = 3000;
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
        for (var j = 0; j < c.options.length; j++) {
          if (c.options[j].selected !== c.options[j].defaultSelected) return true;
        }
      } else if (c.type === 'checkbox' || c.type === 'radio') {
        if (c.checked !== c.defaultChecked) return true;
      } else if (c.value !== c.defaultValue) {
        return true;
      }
    }
    return false;
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
        Object.keys(now).forEach(function (key) {
          var replacement = next[key];
          if (!replacement || busyRegion(now[key])) return;
          now[key].replaceWith(document.importNode(replacement, true));
        });
        var freshLayout = fresh.querySelector('.md-layout');
        if (freshLayout) cursor = freshLayout.getAttribute('data-changes-cursor') || cursor;
        addCopyButtons(document);
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
