/* The host of a slide show's page (show.html), which runs in a second copy
   of the window program on another display (docs/design/detached-slide-
   show.md). It gives Slides, mounted with show=1, the other end of the
   show's port (yavc.app docs/presentation-contract.md, version 2) and the
   services a slide is drawn from, and nothing that writes.

   The two pages are in different browser processes, so the program carries
   their messages: this page hears the editing window's as events of its
   bridge and posts its own through Post. It listens first and attaches
   second, and Attach returns what was posted before, so no message is lost
   or delivered twice, and all arrive in the order sent. */
(function () {
  'use strict';
  function app() { return window.go.main.App; }
  var BASE = new URL('yavc/', document.baseURI).href;
  var mount = document.getElementById('show');

  /* ---- The port ----------------------------------------------------------
     Slides sets onmessage when it is ready for messages; those that arrive
     sooner wait for it. */
  var receiver = null, waiting = [], closed = false, sending = Promise.resolve();
  function deliver(message) {
    if (closed) return;
    if (typeof receiver !== 'function') { waiting.push(message); return; }
    try { receiver(message); } catch (err) { console.error(err); }
  }
  var port = {
    onclose: null,
    postMessage: function (message) {
      if (closed) return;
      var text = JSON.stringify(message);
      sending = sending.then(function () { return app().Post(text); }).catch(function () { /* The show has ended; its window is about to close. */ });
    },
    close: function () { closed = true; waiting = []; }
  };
  Object.defineProperty(port, 'onmessage', {
    enumerable: true,
    get: function () { return receiver; },
    set: function (fn) {
      receiver = fn;
      if (typeof fn !== 'function') return;
      var held = waiting; waiting = [];
      held.forEach(deliver);
    }
  });

  function parsed(text) { try { return JSON.parse(text); } catch (_) { return undefined; } }
  var attached = null, early = [];
  window.runtime.EventsOn('message', function (text) {
    var message = parsed(text);
    if (message === undefined) return;
    if (attached) deliver(message); else early.push(message);
  });

  /* ---- What a slide is drawn from ------------------------------------------
     The same four reads the Slides canvas has in the window, for the one
     deck this show presents (content-host.js). */
  function directory() { return attached && attached.folder || null; }
  function resolvePicture(request, context) {
    var href = String(request && request.href || '');
    if (!attached || !attached.draft || !href || (context && context.current && !context.current())) return Promise.resolve(null);
    var name = href;
    try { name = decodeURIComponent(href); } catch (_) { name = href; }
    return Promise.resolve({ url: '/draft-file?' + new URLSearchParams({ draft: attached.draft, name: name }) });
  }
  function readText(path, options) {
    var signal = options && options.signal, max = options && options.maxBytes > 0 ? Math.floor(options.maxBytes) : 0;
    function stopped() { return new DOMException('The read was stopped.', 'AbortError'); }
    if (signal && signal.aborted) return Promise.reject(stopped());
    return Promise.resolve().then(function () { return app().ReadWorkspaceMountText(String(path), max); }).then(function (r) {
      if (signal && signal.aborted) throw stopped();
      return r && r.found ? r.text : null;
    }, function (err) { throw new Error(String(err && err.message || err)); });
  }
  /* A change to the deck arrives as a new state from the editing window. A
     file a slide refers to is read when the slide is drawn. */
  function onChange() { return function () {}; }

  document.addEventListener('pudl:applet-state', function (e) {
    if (e.target !== mount || !e.detail) return;
    e.detail.state = 'show=1&channel=' + encodeURIComponent(attached ? attached.id : '');
  });

  function start() {
    var services = window.dinahShortcodeServices;
    window.pudlHost = window.pudlHostKit.host({
      pages: BASE + 'content/', pudl: new URL('pudl/', document.baseURI).href,
      contentWorkspace: { version: 1, directory: directory, resources: { resolve: resolvePicture }, shortcodes: services.shortcodes,
        shortcodeHost: { version: 1, render: services.shortcodeHost.render } },
      fs: { readText: readText, onChange: onChange },
      presentation: { version: 2, showPort: function () { return port; } }
    });
    var url = BASE + 'content/slides/applet.json';
    return fetch(url, { credentials: 'same-origin' }).then(function (r) {
      if (!r.ok) throw new Error(url + ': HTTP ' + r.status);
      return r.json();
    }).then(function (m) {
      var cfg = window.pudlHostKit.defineFrom(m, url);
      delete cfg.handles; delete cfg.instances;
      window.pudlApplets.define(m.name, cfg);
      window.pudlApplets.boot(mount);
    });
  }

  window.dinahShow = { port: port, ready: app().Attach().then(function (a) {
    attached = a || {};
    if (window.dinahProduct) window.dinahProduct.apply(attached.product);
    if (attached.mode === 'light' || attached.mode === 'dark') document.documentElement.dataset.theme = attached.mode;
    if (window.dinahAppearance && window.dinahAppearance.reload) window.dinahAppearance.reload().catch(function () { /* The page keeps the build's default theme. */ });
    (attached.messages || []).map(parsed).concat(early).forEach(function (message) { if (message !== undefined) deliver(message); });
    early = [];
    return start();
  }) };
  window.dinahShow.ready.catch(function (err) { console.error(err); });
})();
