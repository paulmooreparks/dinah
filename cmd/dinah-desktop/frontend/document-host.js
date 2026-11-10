/* The host of a stand-alone document application's page (document.html,
   docs/design/standalone-documents.md). The page is one applet of the
   yavc.app release in a window of its own, and this file gives the applet
   what its contracts ask of a host, each backed by the document service
   (internal/docservice) through the page's bridge:

   - contentResources (yavc.app content-resource-contract.md, version 1,
     and content-creation-contract.md). A resource is a file's full path.
     A read is the file as it is now. A write is refused when the file is
     no longer what the applet read, and Save As chooses the place in the
     system's own dialog.
   - snapshot (content-recovery-contract.md, version 2), kept by the
     service for each file, with the unsaved text in it. A document that
     has a file is remembered when its window closes; one that has none is
     kept only for a window that ends by force.
   - documentWindows (document-window-contract.md, version 1): the mount is
     single, and a request for another document gets another window, which
     is another process.
   - config, contentWorkspace.directory and resources.resolve, and
     fs.open, fs.readText and fs.onChange.

   There is no draft store here, as there is in the main window
   (content-host.js): the file is what programs share, and what the person
   has not saved is the applet's own, in its snapshot. */
(function () {
  'use strict';
  function app() { return window.go.main.App; }
  var BASE = new URL('yavc/', document.baseURI).href;
  var mount = document.getElementById('document');
  var attached = null;

  function message(err) { return String(err && err.message || err).replace(/^Error: /, ''); }
  /* The bridge rejects with the error's text, and the applets show an
     Error's message. */
  function failing(err) { throw err instanceof Error ? err : new Error(message(err)); }
  function say(text, tone) {
    if (window.pudlToast) window.pudlToast(text, { tone: tone || 'danger' }); else console.error(text);
  }

  /* ---- The document the applet shows --------------------------------------
     The applet says which file it shows through the API it binds
     (documentWindows.bind), and that can change under the window: a blank
     window opens a file in place, and Save As gives a document another
     place. The service is told when it changes, and whether the person has
     changed anything, which decide the window's title, whether opening
     that file again brings this window forward, and whether closing asks
     first. */
  var bound = null, boundWaiting = [], touched = false, told = null, telling = 0;
  function resourceNow() {
    var r = bound ? bound.resource() : (attached && attached.resource);
    return r ? String(r) : '';
  }
  function baseName(path) {
    var text = String(path || ''), at = Math.max(text.lastIndexOf('/'), text.lastIndexOf(String.fromCharCode(92)));
    return text.slice(at + 1);
  }
  /* Whether the person has changed the document is the Editor's to say,
     through its own action that describes the document. Until the Editor
     is there to ask, a change it reported counts. */
  function changedNow() {
    if (!bound) return Promise.resolve(touched);
    return Promise.resolve().then(function () { return bound.accept({ operation: 'document', input: {} }); }).then(function (r) {
      return !!(r && r.output && r.output.modified);
    }, function () { return touched; });
  }
  function tell() {
    clearTimeout(telling); telling = 0;
    return changedNow().then(function (changed) {
      var resource = resourceNow();
      if (attached) document.title = (baseName(resource) || 'Untitled') + ' - ' + attached.applicationName;
      var state = JSON.stringify([resource, changed]);
      if (state === told) return;
      told = state;
      return app().State(resource, changed).catch(function () { told = null; });
    });
  }
  function tellSoon() { if (!telling) telling = setTimeout(tell, 1000); }
  function whenBound() {
    if (bound) return Promise.resolve(bound);
    return new Promise(function (resolve) { boundWaiting.push(resolve); });
  }

  /* ---- Resources -------------------------------------------------------- */

  function read(request) {
    var resource = String(request.resource || '');
    return app().Read(resource).then(function (r) {
      return { resource: resource, name: r.name, text: r.text, revision: r.revision, writable: !!r.writable };
    }, failing);
  }
  /* Writes are queued for each file, so a save that finishes late is not
     overtaken by a later one. */
  var queues = new Map();
  function write(request) {
    var resource = String(request.resource || '');
    var prior = queues.get(resource) || Promise.resolve();
    var next = prior.catch(function () {}).then(function () {
      return app().Write(resource, String(request.expectedRevision || ''), String(request.text));
    }).then(function (r) {
      touched = false; tellSoon();
      return { resource: resource, name: r.name, revision: r.revision, writable: !!r.writable };
    }, failing);
    queues.set(resource, next);
    next.finally(function () { if (queues.get(resource) === next) queues.delete(resource); }).catch(function () {});
    return next;
  }
  /* Save As. The system's dialog chooses the place and asks before a file
     that is there is chosen. The applet's own check of the name runs before
     anything is written, and a name it refuses is said and the dialog shown
     again. The service writes only if a file that was there is still the
     one the person agreed to replace. */
  function saveAs(request, context) {
    function choose() {
      return app().PickSave(String(request.name || '')).then(function (pick) {
        if (!pick) return null;
        if (context && context.signal && context.signal.aborted) return null;
        var problem = context && context.checkName ? context.checkName(pick.name) : null;
        if (problem) { say(typeof problem === 'string' ? problem : message(problem)); return choose(); }
        return app().CommitSave(pick.ticket, String(request.text)).then(function (r) {
          touched = false;
          /* The applet takes the new place when this answer reaches it. */
          setTimeout(tell, 0);
          return { resource: r.resource, revision: r.revision, name: r.name, writable: !!r.writable };
        });
      });
    }
    return choose().catch(failing);
  }

  /* ---- Files ---------------------------------------------------------------
     Open shows the system's dialog and returns the choice. The applet then
     decides where it goes: into this window when it is blank and untouched,
     and otherwise to another window through documentWindows.open. */
  function open(options) {
    function choose() {
      return app().PickOpen().then(function (picked) {
        if (!picked) return undefined;
        if (options && options.signal && options.signal.aborted) return undefined;
        var problem = options && options.check ? options.check(picked.name) : null;
        if (problem) { say(typeof problem === 'string' ? problem : message(problem)); return choose(); }
        return { path: picked.path, name: picked.name };
      });
    }
    return choose().catch(failing);
  }
  /* A file a document refers to, such as a saved diagram beside it. The
     service reads it only from the folder of one of this window's files or
     below, and anything else reads as a file that is not there. */
  function readText(path, options) {
    var signal = options && options.signal, max = options && options.maxBytes > 0 ? Math.floor(options.maxBytes) : 0;
    function stopped() { return new DOMException('The read was stopped.', 'AbortError'); }
    if (signal && signal.aborted) return Promise.reject(stopped());
    return app().ReadText(String(path), max).then(function (r) {
      if (signal && signal.aborted) throw stopped();
      return r && r.found ? r.text : null;
    }, failing);
  }
  /* A file may have changed whenever the person comes back to the window,
     which is when the applets read again what a document refers to. */
  var changeListeners = new Set();
  function onChange(listener) {
    if (typeof listener !== 'function') return function () {};
    changeListeners.add(listener);
    return function () { changeListeners.delete(listener); };
  }
  window.addEventListener('focus', function () {
    changeListeners.forEach(function (fn) { try { fn(); } catch (err) { /* a listener's failure is its own */ } });
  });

  /* The folder of each file the applet has read, which it asks for without
     waiting (contentWorkspace.directory). */
  var folders = new Map();
  function directory(resource) { return folders.get(String(resource || '')) || null; }
  function learnFolder(resource) {
    if (!resource || folders.has(resource)) return Promise.resolve();
    return app().Directory(resource).then(function (folder) { if (folder) folders.set(resource, folder); }, function () {});
  }
  function readWithFolder(request) {
    var resource = String(request.resource || '');
    return learnFolder(resource).then(function () { return read(request); });
  }
  /* A picture a document shows by a name of its own is served from beside
     the document's file. */
  function resolvePicture(request, context) {
    var resource = String(request && request.resource || ''), href = String(request && request.href || '');
    if (!resource || resource.indexOf('untitled:') === 0 || !href || (context && context.current && !context.current())) return Promise.resolve(null);
    var name = href;
    try { name = decodeURIComponent(href); } catch (_) { name = href; }
    return Promise.resolve({ url: new URL('file?' + new URLSearchParams({ doc: resource, name: name }), document.baseURI).href });
  }

  /* ---- Recovery -------------------------------------------------------------
     The applet's snapshot is sent to the service a second after a change,
     and at once when the applet flushes or the window is closing. What the
     service answers says whether the snapshot is one the person can get
     back by opening the document again, which is so for a file. flush
     answers from that: true for a document with a file whose last send did
     not fail, and false for one with no file, so the applet keeps its own
     warning for a document nobody could reopen. */
  var sending = Promise.resolve(), sendFailed = false, handles = new Set();
  function send(value) {
    var resource = resourceNow(), text;
    try { text = JSON.stringify(value); } catch (err) { sendFailed = true; return; }
    sending = sending.then(function () { return app().PutSnapshot(resource, text); }).then(function () { sendFailed = false; }, function () { sendFailed = true; });
  }
  var keptOnce = false;
  var snapshot = {
    version: 2, remembersClosed: true,
    join: function (root, take, options) {
      /* The snapshot kept for this window's document is given once, to
         the mount the page starts with. */
      var kept = null;
      if (!keptOnce && attached && attached.kept) { try { kept = JSON.parse(attached.kept); } catch (_) { kept = null; } }
      if (options && options.validate) options.validate(kept);
      keptOnce = true;
      var left = false, timer = 0;
      function put() { clearTimeout(timer); timer = 0; if (!left) send(take()); }
      var handle = {
        kept: kept, focused: false,
        changed: function () { touched = true; if (!timer) timer = setTimeout(put, 1000); tellSoon(); },
        flush: function () { if (left) return false; put(); return !!resourceNow() && !sendFailed; },
        /* A mount that leaves is done with its snapshot, and the record of
           a file stays for the next window that opens the file. */
        leave: function () { if (left) return; clearTimeout(timer); left = true; handles.delete(handle); },
        put: put
      };
      handles.add(handle);
      return handle;
    }
  };
  /* The window is closing: everything is sent, and the service is told
     when it has arrived. */
  function flushAll() {
    handles.forEach(function (h) { h.put(); });
    return tell().then(function () { return sending; }, function () { return sending; });
  }

  /* ---- Settings ---------------------------------------------------------- */

  var configListeners = new Map();
  function parsed(values) {
    var out = {};
    Object.keys(values || {}).forEach(function (k) { out[k] = values[k]; });
    return out;
  }
  var config = {
    load: function (name, defaults) { return app().ConfigLoad(String(name)).then(function (v) { return Object.assign({}, defaults || {}, parsed(v)); }, failing); },
    stored: function (name) { return app().ConfigLoad(String(name)).then(parsed, failing); },
    get: function (name, key) { return app().ConfigLoad(String(name)).then(function (v) { return Object.prototype.hasOwnProperty.call(v || {}, key) ? v[key] : undefined; }, failing); },
    set: function (name, next) {
      return app().ConfigSet(String(name), JSON.stringify(next || {})).then(function (all) {
        var merged = parsed(all);
        (configListeners.get(name) || []).concat(configListeners.get('*') || []).forEach(function (fn) { try { fn(merged); } catch (e) { /* a listener's failure is its own */ } });
      }, failing);
    },
    onChange: function (name, fn) {
      if (typeof name === 'function') { fn = name; name = '*'; }
      var list = configListeners.get(name) || []; list.push(fn); configListeners.set(name, list);
      return function () { configListeners.set(name, (configListeners.get(name) || []).filter(function (f) { return f !== fn; })); };
    }
  };

  /* ---- Document windows ----------------------------------------------------- */

  var documentWindows = {
    version: 1,
    mode: function () { return 'single'; },
    bind: function (root, given) {
      bound = given;
      boundWaiting.splice(0).forEach(function (resolve) { resolve(given); });
      tellSoon();
      return function () { if (bound === given) bound = null; };
    },
    /* Another window is another process, which the service has started.
       The answer is what that window's applet returned once it had taken
       the request and kept it for recovery. */
    open: function (root, application, request) {
      return app().OpenWindow(String(application), JSON.stringify(request)).then(function (text) { return JSON.parse(text); }, failing);
    },
    split: function () { return Promise.resolve({ moved: [], error: 'Each document already has its own window.' }); }
  };

  /* A window another window asked for takes that window's request when its
     applet is ready, keeps the result for recovery, and answers. */
  function takeRequest() {
    if (!attached.request) return;
    var request;
    try { request = JSON.parse(attached.request); } catch (_) { app().Accepted('', 'The request for this window could not be read.'); return; }
    var wait = new Promise(function (_, reject) { setTimeout(function () { reject(new Error('The window of the document did not start.')); }, 60000); });
    Promise.race([whenBound(), wait]).then(function (target) {
      return Promise.resolve(target.accept(request)).then(function (result) {
        target.flush();
        return flushAll().then(function () {
          if (sendFailed) throw new Error('The new window could not keep its document for recovery.');
          return app().Accepted(JSON.stringify(result === undefined ? null : result), '');
        });
      });
    }).catch(function (err) { app().Accepted('', message(err) || 'The new window could not take the document.'); });
  }

  /* ---- An agent's action ------------------------------------------------------
     Another program of this product sends an action for this document
     through the service, naming the document. The person is asked here, in
     the window that owns it, and may allow the one action, allow that
     sender until the window closes, or refuse. */
  var allowed = new Set();
  function permit(from, operation) {
    if (allowed.has(from)) return Promise.resolve(true);
    var dialog = document.getElementById('document-permission');
    document.getElementById('document-permission-who').textContent = (from || 'Another program') + ' asks to act on this document.';
    document.getElementById('document-permission-what').textContent = 'The action is "' + operation + '".';
    return new Promise(function (resolve) {
      dialog.returnValue = 'refuse';
      dialog.addEventListener('close', function () {
        if (dialog.returnValue === 'window') allowed.add(from);
        resolve(dialog.returnValue === 'window' || dialog.returnValue === 'once');
      }, { once: true });
      dialog.showModal();
    });
  }
  function invoked(e) {
    var request;
    try { request = JSON.parse(e.request); } catch (_) { app().Answer(e.call, '', 'The action could not be read.'); return; }
    var operation = String(request && request.operation || '');
    permit(String(e.from || ''), operation).then(function (yes) {
      if (!yes) throw new Error('The person refused the action in the document\'s window.');
      return whenBound();
    }).then(function (target) { return target.accept(request); }).then(function (result) {
      tellSoon();
      return app().Answer(e.call, JSON.stringify(result === undefined ? null : result), '');
    }).catch(function (err) { app().Answer(e.call, '', message(err) || 'The action failed.'); });
  }

  /* ---- Start ------------------------------------------------------------------ */

  document.addEventListener('pudl:applet-state', function (e) {
    if (e.target !== mount || !e.detail) return;
    /* A window started for a file shows it; a blank one says nothing. */
    if (attached && attached.resource) e.detail.state = 'file=' + encodeURIComponent(attached.resource);
  });

  function fileServices() {
    return { files: { read: function (resource) { return readWithFolder({ resource: resource }).then(function (r) { return r.text; }); } } };
  }

  /* The application's own menu: another window, and closing this one,
     which the window's process does so that it can ask first. */
  function ownMenu() {
    ['document-application', 'document-application-plain'].forEach(function (id) {
      var el = document.getElementById(id);
      if (el) el.textContent = attached.applicationName;
    });
    var bar = document.querySelector('[data-menubar]');
    if (bar) bar.setAttribute('aria-label', attached.applicationName);
    document.getElementById('document-new-window').addEventListener('click', function () {
      app().NewWindow().catch(function (err) { say(message(err)); });
    });
    document.getElementById('document-close-window').addEventListener('click', function () {
      app().CloseWindow().catch(function (err) { say(message(err)); });
    });
  }

  /* Light, dark or the system's, for the stand-alone applications'
     windows, kept with their settings. The themes and skins themselves are
     the ones chosen in the main window, which this page only reads. */
  var WINDOWS = 'document-windows';
  function colourMode() {
    function ticks() {
      var pref = window.pudlThemePreference ? window.pudlThemePreference() : null;
      document.querySelectorAll('[data-theme-choice]').forEach(function (b) { b.setAttribute('aria-checked', String(b.getAttribute('data-theme-choice') === pref)); });
    }
    document.addEventListener('click', function (event) {
      var control = event.target.closest && event.target.closest('[data-theme-choice]');
      if (!control || !window.pudlSetTheme) return;
      window.pudlSetTheme(control.dataset.themeChoice);
      ticks();
      config.set(WINDOWS, { mode: control.dataset.themeChoice }).catch(function () { /* The choice holds for this window. */ });
    });
    document.addEventListener('pudl:theme-change', ticks);
    return config.get(WINDOWS, 'mode').then(function (mode) {
      if (['light', 'dark', 'system'].indexOf(mode) >= 0 && window.pudlSetTheme) window.pudlSetTheme(mode);
      ticks();
    }, ticks);
  }

  function start() {
    colourMode();
    mount.setAttribute('data-applet', attached.applet);
    document.title = (attached.name || 'Untitled') + ' - ' + attached.applicationName;
    ownMenu();
    window.pudlHost = window.pudlHostKit.host({
      pages: BASE + 'content/', pudl: new URL('pudl/', document.baseURI).href,
      contentWorkspace: { version: 1, directory: directory, resources: { resolve: resolvePicture } },
      contentResources: { version: 1, read: readWithFolder, write: write, saveAs: saveAs },
      fs: { open: open, readText: readText, onChange: onChange },
      snapshot: snapshot, config: config, documentWindows: documentWindows
    });
    window.pudlApplets.capability('files.read', fileServices);
    window.pudlApplets.capability('files.write', fileServices);
    var url = BASE + 'content/' + attached.applet + '/applet.json';
    return fetch(url, { credentials: 'same-origin' }).then(function (r) {
      if (!r.ok) throw new Error(url + ': HTTP ' + r.status);
      return r.json();
    }).then(function (m) {
      var cfg = window.pudlHostKit.defineFrom(m, url);
      delete cfg.handles; delete cfg.instances;
      window.pudlApplets.define(m.name, cfg);
      /* The folder of the starting file is known before the applet asks. */
      return learnFolder(attached.resource || '');
    }).then(function () {
      window.pudlApplets.boot(mount);
      takeRequest();
    });
  }

  window.runtime.EventsOn('invoke', invoked);
  window.runtime.EventsOn('closing', function () {
    flushAll().then(function () { return app().Flushed(); }, function () { return app().Flushed(); });
  });

  window.dinahDocument = {
    /* For a test: what the page was given, and when it has started. */
    attached: function () { return attached; },
    resource: resourceNow,
    flush: flushAll,
    ready: app().Attach().then(function (a) {
      attached = a || {};
      if (window.dinahProduct && attached.product) window.dinahProduct.apply(attached.product);
      if (window.dinahAppearance && window.dinahAppearance.reload && !attached.headless) window.dinahAppearance.reload().catch(function () { /* The page keeps the build's default theme. */ });
      return start();
    })
  };
  window.dinahDocument.ready.catch(function (err) {
    console.error(err);
    var p = document.createElement('p');
    p.className = 'document-failure';
    p.textContent = 'The document could not be shown. ' + message(err);
    mount.replaceWith(p);
  });
})();
