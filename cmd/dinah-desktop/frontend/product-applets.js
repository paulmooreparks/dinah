/* The applets a product built from this source adds to the page
   (docs/design/product-applets.md). The product directory may hold
   product-applets.json, a list of the applets' page files:

     [{ "id": "timesheet", "script": "timesheet-applet.js", "style": "timesheet-applet.css" }]

   Each stylesheet is added to the page and each script is loaded, in the
   order listed, one after another, and a script registers its applet with
   dinahWorkbench.register as the page's own applets do. Dinah's own build
   has no such file, and the page carries on without one. The promise
   dinahProductApplets.ready settles once every listed file has loaded, or
   failed, and the dinah:product-applets event follows. */
(function () {
  'use strict';
  var MANIFEST = 'product-applets.json';
  function load(entry) {
    var done = Promise.resolve();
    if (entry.style) {
      done = new Promise(function (resolve) {
        var link = document.createElement('link');
        link.rel = 'stylesheet'; link.href = entry.style;
        link.onload = function () { resolve(); };
        link.onerror = function () { console.error('The product applet stylesheet could not be loaded: ' + entry.style); resolve(); };
        document.head.appendChild(link);
      });
    }
    if (entry.script) {
      done = done.then(function () {
        return new Promise(function (resolve) {
          var script = document.createElement('script');
          script.src = entry.script;
          script.onload = function () { resolve(); };
          script.onerror = function () { console.error('The product applet script could not be loaded: ' + entry.script); resolve(); };
          document.head.appendChild(script);
        });
      });
    }
    return done;
  }
  var entries = [];
  var ready = fetch(MANIFEST, { credentials: 'same-origin', cache: 'no-store' }).then(function (r) {
    if (!r.ok) return [];
    return r.json();
  }, function () { return []; }).then(function (list) {
    if (!Array.isArray(list)) return;
    entries = list.filter(function (entry) { return entry && typeof entry === 'object' && (entry.script || entry.style); });
    return entries.reduce(function (chain, entry) { return chain.then(function () { return load(entry); }); }, Promise.resolve());
  }).catch(function (err) { console.error('The product applets could not be loaded.', err); }).then(function () {
    document.dispatchEvent(new CustomEvent('dinah:product-applets', { detail: { applets: entries.slice() } }));
  });
  window.dinahProductApplets = { ready: ready, list: function () { return entries.slice(); } };
})();
