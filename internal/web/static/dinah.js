// Dinah's page script, loaded by every page and by the desktop.

// A form inside a desktop window works in that window, as a form in a tab
// works in its tab. It is sent as it would be without script, to its own
// action, with the X-PUDL-Window header naming the window, and the server
// answers with its result as a window. A result about the same resource
// takes the window's place; a result about another resource, such as the
// chat a Start Chat made, opens in the window's place, as following a link
// in a tab does. Anything else, or a failure to reach the server, falls
// back to sending the form as a page. Taken from PUDL Desktop's desktop.js.
(function () {
  if (!document.body.hasAttribute('data-desktop')) return;

  // The desktop opens as it was left. PUDL keeps the arrangement in the
  // address alone, so the last address is remembered in this browser and an
  // arrival at the plain address returns to it. Storage may be refused, in
  // which case the desktop simply opens with its default windows.
  var KEY = 'dinah.desktop';
  try {
    var saved = localStorage.getItem(KEY);
    if (!location.search && saved) { location.replace('/' + saved); return; }
  } catch (e) {}
  var layer = document.querySelector('[data-win-layer]');
  if (layer) layer.addEventListener('pudl:windows-change', function () {
    try { localStorage.setItem(KEY, location.search); } catch (e) {}
  });

  // The applets in the old content are ended and those in the new content
  // started, since a window taking new content does not do either itself.
  function swapInto(win, fresh) {
    window.pudlApplets.destroy(win);
    window.pudlWindows.update(win.getAttribute('data-win'), fresh.outerHTML);
    var title = fresh.querySelector('.win-title');
    if (title) window.pudlWindows.retitle(win.getAttribute('data-win'), title.textContent.trim());
    var now = document.querySelector('.win[data-win="' + CSS.escape(win.getAttribute('data-win')) + '"]');
    if (now) window.pudlApplets.boot(now);
  }

  document.addEventListener('submit', function (e) {
    var form = e.target;
    var win = form.closest && form.closest('.win[data-win]');
    if (!win || e.defaultPrevented || !window.fetch || !window.pudlWindows) return;
    var key = win.getAttribute('data-win');
    var method = (form.getAttribute('method') || 'get').toUpperCase();
    var fields = new URLSearchParams(new FormData(form, e.submitter || undefined));
    var url = form.action;
    var init = { method: method, credentials: 'same-origin', headers: { 'X-PUDL-Window': key, Accept: 'text/html' } };
    if (method === 'POST') init.body = fields;
    else url += (url.indexOf('?') < 0 ? '?' : '&') + fields.toString();
    e.preventDefault();
    win.setAttribute('aria-busy', 'true');
    fetch(url, init).then(function (r) {
      return r.text().then(function (text) { return { r: r, text: text }; });
    }).then(function (got) {
      win.removeAttribute('aria-busy');
      var fresh = new DOMParser().parseFromString(got.text, 'text/html').querySelector('section.win[data-win]');
      if (!fresh) { location.assign(got.r.url); return; }
      var next = fresh.getAttribute('data-win');
      if (next === key) { swapInto(win, fresh); return; }
      var open = document.querySelector('.win[data-win="' + CSS.escape(next) + '"]');
      if (open) swapInto(open, fresh);
      window.pudlWindows.replace(key, next);
    }).catch(function () {
      win.removeAttribute('aria-busy');
      form.submit();
    });
  });
})();

// The server writes each time as a time element with its datetime and the
// computer's regional format as a language tag in data-locale (web.when).
// Each is written again in that format with Intl.DateTimeFormat: the time
// of day for a time today, and the day, month and time otherwise. Windows
// that arrive later are written as they are added to the page.
(function () {
  var forms = {};
  function formatter(tag, today) {
    var key = (today ? 'today ' : 'day ') + tag;
    if (!forms[key]) {
      var opts = today ? { hour: '2-digit', minute: '2-digit' } : { day: 'numeric', month: 'short', hour: '2-digit', minute: '2-digit' };
      try { forms[key] = new Intl.DateTimeFormat(tag || undefined, opts); } catch (e) { forms[key] = new Intl.DateTimeFormat(undefined, opts); }
    }
    return forms[key];
  }
  function write(scope) {
    if (!scope.querySelectorAll) return;
    var now = new Date();
    var list = Array.prototype.slice.call(scope.querySelectorAll('time[data-locale][datetime]'));
    if (scope.matches && scope.matches('time[data-locale][datetime]')) list.push(scope);
    list.forEach(function (t) {
      var d = new Date(t.getAttribute('datetime'));
      if (isNaN(d)) return;
      t.textContent = formatter(t.getAttribute('data-locale'), d.toDateString() === now.toDateString()).format(d);
    });
  }
  write(document);
  new MutationObserver(function (changes) {
    changes.forEach(function (c) { c.addedNodes.forEach(write); });
  }).observe(document.body, { childList: true, subtree: true });
})();

// When the bus changes, the bus's views are drawn afresh. On a page that is
// a reload; on the desktop it is each window that shows the bus, taken again
// from its own address. A view with something typed in it is left alone, and
// a page says that the bus has moved on instead.
(function () {
  if (!window.EventSource) return;
  var desktop = document.body.hasAttribute('data-desktop');
  var drawn = document.body.getAttribute('data-counter');
  var events = new EventSource('/events');

  function typing(scope) {
    return Array.prototype.some.call(scope.querySelectorAll('textarea, input:not([type=hidden])'),
      function (f) { return f.value.trim() !== ''; });
  }

  events.onmessage = function (e) {
    if (e.data === drawn) return;
    drawn = e.data;
    if (desktop) {
      document.querySelectorAll('.win[data-win]').forEach(function (win) {
        var key = win.getAttribute('data-win');
        if (/^chats?(-|$)/.test(key) || typing(win)) return;
        window.pudlWindows.reload(key);
      });
      return;
    }
    if (typing(document)) {
      var n = document.getElementById('reload-notice');
      if (n) n.hidden = false;
      return;
    }
    events.close();
    location.reload();
  };
})();
