/* The host the yavc.app content applets run in (YAVC.md, and
   docs/design/content-applets.md). Each applet is a package of the release
   in yavc/, mounted inside a Dinah window of its own (content-windows.js),
   one document to a window. This file gives the packages what their
   contracts ask a host for, all of it backed by Dinah's own services:

   - contentResources (yavc.app content-resource-contract.md, version 1):
     a resource is a Dinah draft's ID. Reading returns the draft's text and
     revision; a write is the draft service's conditional update, then, for
     a document that has a file, the draft service's save, so a change to an
     existing document is always written to it.
   - snapshot (content-recovery-contract.md, version 2): each window's
     recovery snapshot is a key of Dinah's one UI-state store. flush() puts
     it there at once and returns true; the store's own write, which the
     window's close waits for, makes it durable. The text itself is in the
     draft in any case.
   - documentWindows (document-window-contract.md, version 1): every mount
     is single, and a request to open another document opens a Dinah window
     of its own.
   - config: each applet's settings, also in the UI-state store.

   The project capabilities files.read and files.write are registered here,
   before any package is defined, as the workspace contract requires. Agents
   reach the applets through Dinah's Go broker, never through this page's
   broker directly. */
(function () {
  'use strict';

  var helpers = window.dinahHelpers;
  function api() { return helpers.app(); }
  var BASE = new URL('yavc/', document.baseURI).href;
  var APPS = { editor: 'Editor', docs: 'Docs', diagrams: 'Diagrams', slides: 'Slides' };

  /* ---- Resources -------------------------------------------------------- */

  function baseName(path) { var parts = String(path || '').split(/[\\/]/); return parts[parts.length - 1] || ''; }
  /* The name an applet checks a document's format by: the file's own name,
     or for a new document, its title, which is Markdown unless the title
     says otherwise, as in Dinah's own Editor. */
  function nameOf(v) {
    var own = baseName(v.origin && v.origin.path);
    if (own) return own;
    var title = v.title || 'Untitled';
    return /\.[A-Za-z0-9]{1,8}$/.test(title) ? title : title + '.md';
  }

  /* A resource is named for the person, since the applets show it: the
     workspace's name and the file's path in it, or a new document's title.
     The UI state keeps which draft each name stands for, so a window
     restored after a restart finds its draft again. */
  var NAMES = 'content-resources';
  function names() { return window.dinahUIState.get(NAMES, {}); }
  function draftOf(resource) {
    var id = names()[resource];
    if (!id) return Promise.reject(new Error((window.dinahProduct ? window.dinahProduct.name : '') + ' does not know the document ' + resource + '.'));
    return Promise.resolve(id);
  }
  function workspaceName(id) {
    var records = window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.records() : [];
    var owner = records.find(function (r) { return r.id === id; });
    return owner ? owner.name : '';
  }
  /* The resource that stands for a draft, made once and then kept. */
  function resourceFor(v) {
    var all = names(), o = v.origin || {};
    var base = o.path ? (workspaceName(o.workspaceId) ? workspaceName(o.workspaceId) + '/' : '') + o.path : (v.title || 'Untitled') + ' (not saved)';
    var name = base, n = 1;
    while (all[name] && all[name] !== v.id) name = base + ' (' + (++n) + ')';
    Object.keys(all).forEach(function (k) { if (all[k] === v.id && k !== name) delete all[k]; });
    if (all[name] !== v.id) { all[name] = v.id; window.dinahUIState.set(NAMES, all); }
    return name;
  }
  /* A closed window's draft no longer needs its name. */
  function forget(resource) {
    var all = names();
    if (Object.prototype.hasOwnProperty.call(all, resource)) { delete all[resource]; window.dinahUIState.set(NAMES, all); }
  }

  /* The draft revision each resource's applet last read or wrote, which
     content-windows.js compares with the draft to see a change made
     elsewhere; and the resources whose next write is to replace such a
     change (Keep Mine). */
  var known = new Map(), replacing = new Set();
  function knownRevision(resource) { return known.get(resource) || ''; }
  function keepMine(resource) { replacing.add(resource); }

  function read(request) {
    var resource = String(request.resource || '');
    return draftOf(resource).then(function (id) { return api().GetDraft(id); }).then(function (v) {
      known.set(resource, v.revision);
      return { resource: resource, name: nameOf(v), text: v.text, revision: v.revision, writable: true };
    });
  }

  /* Writes are queued per draft, so a save that finishes late cannot be
     overtaken by a later one reaching the draft service first. */
  var queues = new Map();
  function write(request) {
    var resource = String(request.resource || '');
    var prior = queues.get(resource) || Promise.resolve();
    var next = prior.catch(function () {}).then(function () { return draftOf(resource); }).then(function (id) {
      var expected = String(request.expectedRevision || '');
      var replace = replacing.has(resource) ? Promise.resolve(api().GetDraft(id)).then(function (v) { replacing.delete(resource); return v.revision; }) : Promise.resolve(expected);
      return replace.then(function (rev) { return api().UpdateDraft(id, rev, String(request.text)); }).then(function (v) {
        known.set(resource, v.revision);
        if (v.writeBack !== 'save' || !v.changed) return v;
        return api().SaveDraft(id, v.revision);
      }).then(function (v) {
        document.dispatchEvent(new CustomEvent('dinah:content-written', { detail: { resource: resource, draft: v } }));
        return { resource: resource, name: nameOf(v), revision: v.revision, writable: true };
      });
    });
    var id = resource;
    queues.set(id, next);
    next.finally(function () { if (queues.get(id) === next) queues.delete(id); }).catch(function () {});
    return next;
  }

  /* ---- Recovery and settings --------------------------------------------- */

  function store() { return window.dinahUIState; }
  function snapshotKey(root) {
    var win = root.closest('[data-win]');
    return 'content-snapshot:' + (win ? win.dataset.win : root.id || 'page');
  }
  var snapshot = {
    version: 2,
    join: function (root, take, options) {
      var key = snapshotKey(root), kept = store().get(key, null), left = false;
      if (options && options.validate) options.validate(kept);
      function put() { if (!left) store().set(key, take()); }
      return {
        kept: kept, focused: false,
        changed: function () { put(); root.dispatchEvent(new CustomEvent('dinah:content-changed', { bubbles: true })); },
        flush: function () { if (left) return false; put(); return true; },
        /* An applet leaves only when its window closes or Dinah mounts it
           afresh (a page going away with the application destroys nothing),
           and either way it is done with its snapshot; its draft keeps the
           document. */
        leave: function () { if (left) return; left = true; store().set(key, null); }
      };
    }
  };

  var configListeners = new Map();
  function configKey(app) { return 'content-config:' + app; }
  var config = {
    load: function (app, defaults) { return Promise.resolve(Object.assign({}, defaults || {}, store().get(configKey(app), {}))); },
    stored: function (app) { return Promise.resolve(Object.assign({}, store().get(configKey(app), {}))); },
    get: function (app, name) { var all = store().get(configKey(app), {}); return Promise.resolve(Object.prototype.hasOwnProperty.call(all, name) ? all[name] : undefined); },
    set: function (app, next) {
      var merged = Object.assign({}, store().get(configKey(app), {}), next || {});
      store().set(configKey(app), merged);
      (configListeners.get(app) || []).forEach(function (fn) { try { fn(merged); } catch (e) { /* a listener's failure is its own */ } });
      return store().flush();
    },
    onChange: function (app, fn) {
      if (typeof app === 'function') { fn = app; app = '*'; }
      var list = configListeners.get(app) || []; list.push(fn); configListeners.set(app, list);
      return function () { configListeners.set(app, (configListeners.get(app) || []).filter(function (f) { return f !== fn; })); };
    }
  };

  /* ---- Document windows ---------------------------------------------------- */

  /* What each mount bound (document-window-contract.md): the window's key
     to its api, and to whoever waits for it. */
  var bound = new Map(), waiting = new Map();
  function windowKey(root) { var win = root.closest('[data-win]'); return win ? win.dataset.win : null; }
  function boundApi(key) {
    if (bound.has(key)) return Promise.resolve(bound.get(key));
    return new Promise(function (resolve) { var list = waiting.get(key) || []; list.push(resolve); waiting.set(key, list); });
  }
  var documentWindows = {
    version: 1,
    mode: function () { return 'single'; },
    bind: function (root, given) {
      var key = windowKey(root);
      if (!key) return function () {};
      bound.set(key, given);
      (waiting.get(key) || []).forEach(function (resolve) { resolve(given); });
      waiting.delete(key);
      return function () { if (bound.get(key) === given) bound.delete(key); };
    },
    open: function (root, application, request) {
      var from = windowKey(root);
      return window.dinahContentWindows.route(application, request, from).then(function (routed) {
        return boundApi(routed.key).then(function (target) {
          /* A new document is already the new window's own; anything else
             is the request the window takes. */
          var taken = routed.done ? target.accept({ operation: 'documents', input: {} }).then(function (r) {
            return { output: (r.output.documents || [])[0] || null };
          }) : target.accept(request);
          return Promise.resolve(taken).then(function (result) {
            if (!target.flush()) throw new Error('The new window could not keep its document for recovery.');
            return result;
          });
        });
      });
    },
    split: function () { return Promise.resolve({ moved: [], error: 'Each document already has its own window.' }); }
  };

  /* The api a window's applet bound, for an agent's action routed to that
     window by Dinah (content-windows.js). */
  function accept(key, request) { return boundApi(key).then(function (target) { return target.accept(request); }); }

  /* ---- Installation ------------------------------------------------------- */

  function fileServices() {
    return { files: { read: function (resource) { return read({ resource: resource }).then(function (r) { return r.text; }); } } };
  }

  var installed = null;
  function install() {
    if (installed) return installed;
    window.pudlHost = window.pudlHostKit.host({
      pages: BASE + 'content/', pudl: new URL('pudl/', document.baseURI).href,
      contentWorkspace: { version: 1, directory: function () { return null; } },
      contentResources: { version: 1, read: read, write: write },
      snapshot: snapshot, config: config, documentWindows: documentWindows
    });
    window.pudlApplets.capability('files.read', fileServices);
    window.pudlApplets.capability('files.write', fileServices);
    installed = Promise.all(Object.keys(APPS).map(function (app) {
      var url = BASE + 'content/' + app + '/applet.json';
      return fetch(url, { credentials: 'same-origin' }).then(function (r) {
        if (!r.ok) throw new Error(url + ': HTTP ' + r.status);
        return r.json();
      }).then(function (m) {
        /* Dinah opens the windows, one per document, so the package's
           handles verbs and its count of numbered windows are left out:
           PUDL must never open a window of its own for one of these. */
        var cfg = window.pudlHostKit.defineFrom(m, url);
        delete cfg.handles; delete cfg.instances;
        window.pudlApplets.define(m.name, cfg);
        return m;
      });
    }));
    return installed;
  }

  /* A mount says which draft it shows (content-windows.js), and gets it as
     its state. */
  document.addEventListener('pudl:applet-state', function (e) {
    var id = e.target && e.target.dataset && e.target.dataset.contentResource;
    if (id && e.detail) e.detail.state = 'file=' + encodeURIComponent(id);
  });

  function clearSnapshot(key) { store().set('content-snapshot:' + key, null); }

  window.dinahContentHost = { install: install, accept: accept, read: read, apps: APPS, resourceFor: resourceFor, forget: forget,
    clearSnapshot: clearSnapshot, knownRevision: knownRevision, keepMine: keepMine, draftOf: function (resource) { return names()[resource] || ''; } };
})();
