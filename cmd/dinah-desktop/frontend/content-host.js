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
     or for a new document, its title with the extension of the application
     showing it (a diagram is D2, a deck is Slides' Markdown, anything else
     is Markdown), unless the title has one of its own. */
  var EXTENSIONS = { diagrams: '.d2', slides: '.slides.md' };
  function nameOf(v, resource) {
    var own = baseName(v.origin && v.origin.path);
    // A document of a folder is named by its file; a document of another
    // kind of source (a page on a site) by its title, with the Markdown
    // extension the applet reads the format from.
    // Such a title can end as a file name does ("Release 2.0") and is
    // still the title of a Markdown document.
    var page = false;
    if (own && v.origin && v.origin.kind !== 'local-folder' && v.title && !/\.[A-Za-z0-9]{1,8}$/.test(own)) { own = ''; page = true; }
    if (own) return own;
    var title = v.title || 'Untitled';
    if (page && !/\.(md|markdown)$/i.test(title)) return title + '.md';
    if (/\.[A-Za-z0-9]{1,8}$/.test(title)) return title;
    var mount = resource && Array.prototype.find.call(document.querySelectorAll('.content-applet'), function (m) { return m.dataset.contentResource === resource; });
    return title + (EXTENSIONS[mount && mount.getAttribute('data-applet')] || '.md');
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
    // A document of a folder is named by its path there. A document of
    // another kind of source (a page on a site) is named by its title, as
    // nameOf does, since its path can be made of identifiers.
    var where = o.path && o.kind !== 'local-folder' && v.title && !/\.[A-Za-z0-9]{1,8}$/.test(baseName(o.path)) ? v.title : o.path;
    var base = where ? (workspaceName(o.workspaceId) ? workspaceName(o.workspaceId) + '/' : '') + where : (v.title || 'Untitled') + ' (not saved)';
    var name = base, n = 1;
    /* A name a window shows now stays that window's; one left by a window
       that is gone, such as after a crash, is taken over. */
    function shown(resource) {
      return Array.prototype.some.call(document.querySelectorAll('.content-applet'), function (m) { return m.dataset.contentResource === resource; });
    }
    while (all[name] && all[name] !== v.id && shown(name)) name = base + ' (' + (++n) + ')';
    Object.keys(all).forEach(function (k) { if (all[k] === v.id && k !== name) delete all[k]; });
    if (all[name] !== v.id) { all[name] = v.id; window.dinahUIState.set(NAMES, all); }
    return name;
  }
  /* A closed window's draft no longer needs its name. */
  function forget(resource) {
    var all = names();
    if (Object.prototype.hasOwnProperty.call(all, resource)) { delete all[resource]; window.dinahUIState.set(NAMES, all); }
    folders.delete(resource);
    closeShowsOf(resource);
  }

  /* The folder of each document that is a file, as a virtual path, which an
     applet asks for to find a file the document refers to by a path of its
     own, such as a saved diagram beside a deck. The applet asks without
     waiting (contentWorkspace.directory), so the folder is found when the
     document is read and when it is saved under a new name. A document that
     has no file yet, and a page of a source that is not a folder, have
     none. */
  var folders = new Map();
  function findFolder(resource, v) {
    var o = v && v.origin || {};
    if (!o.path || o.kind !== 'local-folder' || !api().ResolveWorkspaceResourceMount) { folders.delete(resource); return Promise.resolve(); }
    return Promise.resolve(api().ResolveWorkspaceResourceMount({ workspaceId: o.workspaceId, sourceId: o.sourceId, path: o.path })).then(function (item) {
      var path = String(item && item.path || ''), at = path.lastIndexOf('/');
      if (at > 0) folders.set(resource, path.slice(0, at)); else folders.delete(resource);
    }).catch(function () { folders.delete(resource); });
  }
  function directory(resource) { return folders.get(String(resource || '')) || null; }

  /* The draft revision each resource's applet last read or wrote, which
     content-windows.js compares with the draft to see a change made
     elsewhere; and the resources whose next write is to replace such a
     change (Keep Mine). */
  var known = new Map(), replacing = new Set();
  function knownRevision(resource) { return known.get(resource) || ''; }
  function keepMine(resource) { replacing.add(resource); }

  /* A read or a write result for a new document, which has no file yet,
     says so (needsLocation, yavc.app content-creation-contract.md), and the
     applets' Save then asks the host's saveAs for a place instead of
     writing the draft. The draft still keeps the text while it has none. */
  function located(result, v) {
    if (v && v.origin && v.origin.kind === 'new') result.needsLocation = true;
    return result;
  }

  function read(request) {
    var resource = String(request.resource || '');
    return draftOf(resource).then(function (id) { return api().GetDraft(id); }).then(function (v) {
      known.set(resource, v.revision);
      return findFolder(resource, v).then(function () {
        return located({ resource: resource, name: nameOf(v, resource), text: v.text, revision: v.revision, writable: true }, v);
      });
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
        return located({ resource: resource, name: nameOf(v, resource), revision: v.revision, writable: true }, v);
      });
    });
    var id = resource;
    queues.set(id, next);
    next.finally(function () { if (queues.get(id) === next) queues.delete(id); }).catch(function () {});
    return next;
  }

  /* ---- Recovery and settings --------------------------------------------- */

  function store() { return window.dinahUIState; }
  function snapshotKey(root) { return 'content-snapshot:' + (windowKey(root) || root.id || 'page'); }
  /* The applets keep each tab's unsaved text in their recovery snapshot,
     for a host without drafts. Dinah's draft is the document's record,
     written at every pause in typing, so the text is left out of the
     snapshot both ways: the applet comes back to the draft's text, never to
     a recovered copy whose revision no longer matches the draft, which it
     then refused to save until Save As (10 November 2026). The snapshot
     keeps the rest: the tabs, the view and the place. */
  function withoutText(v) {
    if (!v || typeof v !== 'object' || !Array.isArray(v.tabs)) return v;
    // The Editor keeps a tab's unsaved text as draft, Slides as text.
    return Object.assign({}, v, { tabs: v.tabs.map(function (tab) {
      if (!tab || typeof tab !== 'object') return tab;
      var out = Object.assign({}, tab);
      if ('text' in out) out.text = null;
      if ('draft' in out) out.draft = null;
      return out;
    }) });
  }
  var snapshot = {
    version: 2,
    join: function (root, take, options) {
      var key = snapshotKey(root), kept = withoutText(store().get(key, null)), left = false, timer = 0;
      if (options && options.validate) options.validate(kept);
      function put() { clearTimeout(timer); timer = 0; if (!left) store().set(key, withoutText(take())); }
      /* The snapshot holds the whole document, and the store sends itself
         whole to Go when it changes, so it is taken once a second while
         typing goes on; the draft keeps the text in any case, and flush
         and leave take it at once. */
      function putSoon() { if (!timer) timer = setTimeout(put, 1000); }
      return {
        kept: kept, focused: false,
        changed: function () { putSoon(); root.dispatchEvent(new CustomEvent('dinah:content-changed', { bubbles: true })); },
        flush: function () { if (left) return false; put(); return true; },
        /* An applet leaves only when its window closes or Dinah mounts it
           afresh (a page going away with the application destroys nothing),
           and either way it is done with its snapshot; its draft keeps the
           document. */
        leave: function () { if (left) return; clearTimeout(timer); left = true; store().set(key, null); }
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
  /* The key of the content window an applet is mounted in: its window's,
     or an embedded Editor's own (content-windows.js). */
  function windowKey(root) {
    var holder = root.closest('[data-content-key]');
    if (holder) return holder.dataset.contentKey;
    var win = root.closest('[data-win]');
    return win ? win.dataset.win : null;
  }
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
          var taken = routed.done ? window.dinahContentWindows.listed(routed.key).then(function (doc) { return { output: doc }; }) : target.accept(request);
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
  function accept(key, request) {
    var mounted = document.querySelector('[data-content-key="' + CSS.escape(key) + '"]');
    if (!bound.has(key) && !mounted && window.pudlWindows.state().open.indexOf(key) < 0) return Promise.reject(new Error('The window of that document has closed.'));
    var settle = bound.has(key) ? boundApi(key) : Promise.race([boundApi(key), new Promise(function (resolve, reject) {
      setTimeout(function () { reject(new Error('The window of the document did not start.')); }, 30000);
    })]);
    return settle.then(function (target) { return target.accept(request); });
  }

  /* ---- Installation ------------------------------------------------------- */

  /* ---- Save As and Open -------------------------------------------------
     Both are Dinah's own, through its file picker and the draft service
     (yavc.app content-creation-contract.md). Save As moves the window's
     draft to the file chosen, as Dinah's Editor does, so the window keeps
     its document; Open opens the chosen document in a window of its own,
     never as a second document in the asking window. */

  function windowItem(el) {
    var win = el && el.closest && el.closest('[data-win]');
    var root = win && win.querySelector('[data-item]');
    try { return root ? JSON.parse(root.dataset.item) : null; } catch (e) { return null; }
  }
  function message(err) { return String(err && err.message || err).replace(/^Error: /, ''); }

  /* Save As starts in the folder of the document's file, as in Dinah's
     Editor, or for a new document where the person is. */
  function startFolder(v) {
    var here = window.dinahResources ? window.dinahResources.currentPath() : '/';
    var o = v.origin || {};
    if (!o.path || !api().ResolveWorkspaceResourceMount) return Promise.resolve(here);
    return Promise.resolve(api().ResolveWorkspaceResourceMount({ workspaceId: o.workspaceId, sourceId: o.sourceId, path: o.path })).then(function (item) {
      return item.path.slice(0, item.path.lastIndexOf('/')) || '/';
    }).catch(function () { return here; });
  }

  function saveAs(request, context) {
    var id = names()[String(request.sourceResource || '')];
    var item = windowItem(context && context.host);
    if (!id && item) id = item.path;
    if (!id) return Promise.reject(new Error('The document to save is not one ' + (window.dinahProduct ? window.dinahProduct.name : '') + ' knows.'));
    var draftView = null;
    return Promise.resolve(api().GetDraft(id)).then(function (v) {
      draftView = v;
      return startFolder(v);
    }).then(function (start) {
      var v = draftView, done = null;
      return window.dinahFilePicker.pick({ mode: 'save', title: 'Save As', start: start, name: request.name || nameOf(v, String(request.sourceResource || '')),
        saveKind: window.dinahResources ? window.dinahResources.documentHomes() : undefined,
        commit: function (choice) {
          var name = choice.computer || choice.name;
          var problem = context && context.checkName ? context.checkName(name) : null;
          if (problem) throw new Error(typeof problem === 'string' ? problem : message(problem));
          return Promise.resolve(api().GetDraft(id)).then(function (latest) {
            return api().UpdateDraft(id, latest.revision, String(request.text));
          }).then(function (updated) {
            /* A copy on the computer is only a copy: the window stays with
               its document. */
            if (choice.computer) {
              var ws = updated.origin.workspaceId || (window.dinahHost && window.dinahHost.workspaceId());
              return Promise.resolve(api().ChooseComputerSavePath(ws, choice.computer)).then(function (path) {
                if (!path) return false;
                return Promise.resolve(api().SaveDraftToComputer(id, path, false)).then(function () {
                  if (window.dinahHost) window.dinahHost.toast('Saved a copy.', 'info');
                  return true;
                });
              });
            }
            var ref = choice.dir.ref;
            var target = { workspaceId: ref.workspaceId, sourceId: ref.sourceId, path: (ref.path && ref.path !== '.' ? ref.path + '/' : '') + choice.name };
            var titled = Promise.resolve(updated);
            /* A source that makes its own documents (a page on a site)
               takes the new document under the chosen folder, titled with
               the name given, without the file extension. */
            if (choice.dir.creates && !choice.dir.writable) {
              target.path = ref.path && ref.path !== '.' ? ref.path : '';
              var pageTitle = String(choice.name || '').replace(/\.(md|markdown)$/i, '').trim() || updated.title || 'Untitled';
              titled = Promise.resolve(api().RetitleDraft(id, updated.revision, pageTitle));
            }
            return titled.then(function (ready) { return api().SaveDraftAs(id, ready.revision, target); }).then(function (saved) {
              var resource = resourceFor(saved);
              known.set(resource, saved.revision);
              return findFolder(resource, saved).then(function () {
                document.dispatchEvent(new CustomEvent('dinah:content-renamed', { detail: { draft: saved.id, resource: resource, view: saved } }));
                done = { resource: resource, revision: saved.revision, name: nameOf(saved), writable: true };
                return true;
              });
            });
          });
        } }).then(function () { return done; });
    });
  }

  /* A document can refer to another file by its virtual path: a saved
     diagram a slide shows, a stylesheet several documents share, a template
     an export uses. readText gives the applet that file's text, or null
     when there is no such file. The path is under /home or /workspaces
     (~/ is the person's home), and a relative path was already joined to
     the document's folder by the applet. The applet does not say which
     document asks, so the person's own files are what a path can be, in
     any of their workspaces. Dinah reads a text file of up to 1 MiB.
     Neither function uses this, since the applets pass them on alone. */
  function readText(path, options) {
    var signal = options && options.signal, max = options && options.maxBytes > 0 ? Math.floor(options.maxBytes) : 0;
    function stopped() { return new DOMException('The read was stopped.', 'AbortError'); }
    if (signal && signal.aborted) return Promise.reject(stopped());
    return Promise.resolve().then(function () { return api().ReadWorkspaceMountText(String(path), max); }).then(function (r) {
      if (signal && signal.aborted) throw stopped();
      return r && r.found ? r.text : null;
    }, function (err) { throw new Error(message(err)); });
  }
  /* onChange tells the applet that a file may have changed, so that it
     reads again what its document refers to. Dinah learns of a change it
     made itself and of one on disk, in a folder some window shows or in
     the folder of a file readText read lately (the newest 32 such
     folders, in local sources). The applets also read again when the
     window is returned to. */
  var changeListeners = new Set(), watchingFiles = false;
  function onChange(listener) {
    if (typeof listener !== 'function') return function () {};
    changeListeners.add(listener);
    if (!watchingFiles && window.dinahHost) {
      watchingFiles = true;
      window.dinahHost.on('workspace:changed', function (e) {
        if (!e || !/^(content|disk|source)-changed$/.test(e.kind)) return;
        changeListeners.forEach(function (fn) { try { fn(); } catch (err) { /* one listener's failure is its own */ } });
      });
    }
    return function () { changeListeners.delete(listener); };
  }

  /* The file service the applets use for Open (fs.open) and for the files
     a document refers to (fs.readText, fs.onChange). Open opens the chosen
     document in a window of its own and tells the applet that nothing was
     chosen for it. */
  var fs = {
    open: function (options) {
      var mount = options && options.host && options.host.closest && options.host.closest('[data-applet]');
      var app = mount ? mount.getAttribute('data-applet') : 'editor';
      return window.dinahContentWindows.openDocument(app).then(function () { return null; });
    },
    readText: readText, onChange: onChange
  };

  function fileServices() {
    return { files: { read: function (resource) { return read({ resource: resource }).then(function (r) { return r.text; }); } } };
  }

  /* ---- Shortcodes the build adds ------------------------------------------
     The two services that give a build's shortcodes to the applets,
     contentWorkspace.shortcodes and contentWorkspace.shortcodeHost, are in
     shortcode-services.js, which a slide show's page loads too. */
  var shortcodes = window.dinahShortcodeServices.shortcodes, shortcodeHost = window.dinahShortcodeServices.shortcodeHost;

  var installed = null;
  function install() {
    if (installed) return installed;
    window.pudlHost = window.pudlHostKit.host({
      pages: BASE + 'content/', pudl: new URL('pudl/', document.baseURI).href,
      contentWorkspace: { version: 1, directory: directory, resources: { resolve: resolvePicture }, suggestions: { subscribe: subscribeSuggestions, decide: decideSuggestion }, shortcodes: shortcodes, shortcodeHost: shortcodeHost },
      contentResources: { version: 1, read: read, write: write, saveAs: saveAs }, fs: fs,
      snapshot: snapshot, config: config, documentWindows: documentWindows, presentation: presentation,
      commandToolbar: { attach: function (root, options) { return window.dinahContentToolbar.attach(root, options); } }
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
     its state; a presenter's mount says so too, with the channel the show
     speaks on. */
  document.addEventListener('pudl:applet-state', function (e) {
    var id = e.target && e.target.dataset && e.target.dataset.contentResource;
    if (!id || !e.detail) return;
    e.detail.state = 'file=' + encodeURIComponent(id);
    var channel = e.target.dataset.contentPresenter;
    if (channel) e.detail.state += '&presenter=1&presenter-channel=' + encodeURIComponent(channel);
  });

  /* ---- Pictures and suggestions (yavc.app's document host overlays) ------
     The applets ask their host for two things Dinah owns (v0.2.53,
     docs/design/content-applets.md). A picture a document refers to by
     name, as "shot.png" beside it, is resolved to a URL: Dinah serves a
     draft's own files, and the ones beside the document it was opened
     from, at /draft-file, so the Markdown keeps the name and the window
     shows the picture. The suggestions Dinah keeps beside a draft (an
     agent's proposed changes, applet_documents.go) are published to the
     applet showing it, which marks them in the text with Accept, Reject and
     Edit; a decision comes back here and Dinah applies it to the draft, as
     the review panel does, and the applet shows the new text through its
     ordinary read. */
  function resolvePicture(request, context) {
    var id = names()[String(request && request.resource || '')];
    var href = String(request && request.href || '');
    if (!id || !href || (context && context.current && !context.current())) return Promise.resolve(null);
    var name = href;
    try { name = decodeURIComponent(href); } catch (_) { name = href; }
    return Promise.resolve({ url: '/draft-file?' + new URLSearchParams({ draft: id, name: name }) });
  }
  /* The suggestions of a draft, as the applet takes them: each placed one
     with its place in the text given. An unplaced one stays in the panel. */
  function suggestionSnapshot(v) {
    var spans = v.spans || {};
    var items = [];
    (v.suggestions || []).forEach(function (s) {
      var at = spans[s.id];
      if (!at) return;
      items.push({ id: s.id, start: at[0], end: at[1], before: s.before || '', after: s.after || '', author: s.author || '', reason: s.reason || '',
        group: s.group || '', stale: !!s.stale, edited: !!s.edited, prefix: s.prefix || '', suffix: s.suffix || '' });
    });
    return { revision: v.revision, text: v.text, items: items };
  }
  var suggestionSubscribers = new Map(), suggestionTimers = new Map();
  function publishSuggestions(id) {
    var set = suggestionSubscribers.get(id);
    if (!set || !set.size) return;
    Promise.resolve(api().GetDraft(id)).then(function (v) {
      var snapshot = suggestionSnapshot(v);
      set.forEach(function (sub) { try { sub(snapshot); } catch (_) { /* the applet's own */ } });
    }).catch(function () { /* the next change publishes again */ });
  }
  function publishSuggestionsSoon(id) {
    clearTimeout(suggestionTimers.get(id));
    suggestionTimers.set(id, setTimeout(function () { suggestionTimers.delete(id); publishSuggestions(id); }, 50));
  }
  function subscribeSuggestions(resource, callback, context) {
    var id = names()[String(resource || '')];
    if (!id || typeof callback !== 'function') return function () {};
    var set = suggestionSubscribers.get(id);
    if (!set) { set = new Set(); suggestionSubscribers.set(id, set); }
    set.add(callback);
    watchDrafts();
    var off = function () { var s = suggestionSubscribers.get(id); if (s) { s.delete(callback); if (!s.size) suggestionSubscribers.delete(id); } };
    if (context && context.signal) context.signal.addEventListener('abort', off, { once: true });
    publishSuggestions(id);
    return off;
  }
  var watchingDrafts = false;
  function watchDrafts() {
    if (watchingDrafts || !window.dinahHost) return;
    watchingDrafts = true;
    window.dinahHost.on('draft:changed', function (e) { if (e && e.id && suggestionSubscribers.has(e.id)) publishSuggestionsSoon(e.id); });
  }
  function decideSuggestion(request, context) {
    if (!window.dinahContentWindows || !window.dinahContentWindows.decideSuggestion) return Promise.reject(new Error('Suggestions cannot be decided here.'));
    return window.dinahContentWindows.decideSuggestion(request || {}, context || {});
  }

  /* ---- Presenting ---------------------------------------------------------
     Slides presents full screen in its own window, and opens a presenter
     view (the current slide, the next, the notes and a clock) as a second
     page of the same origin, through window.open, which the show and the
     presenter then speak over a BroadcastChannel (its presentation
     contract). Paul ruled (9 November 2026) that the show is
     full screen and the presenter view a separate window when asked for.
     Dinah has one page, so the presenter URL it gives is its own, and its
     window.open answers that URL with a Dinah window of the presenter kind
     (content-windows.js) rather than a browser popup; both applets are on
     one page, where the channel works as well. Any other address goes to
     the browser's own window.open. */
  /* From yavc.app v0.2.62 the capability is version 2, and offers openShow
     (below, A show on another display) beside presenterURL. */
  var PRESENTER_PATH = '/presenter';
  var presentation = { version: 2, openShow: function (request) { return openShow(request); }, presenterURL: function (args) {
    return PRESENTER_PATH + '?' + new URLSearchParams({ application: String(args.application || ''), resource: String(args.resource || ''), channel: String(args.channel || '') });
  } };
  var browserOpen = window.open;
  window.open = function (url) {
    var u = null;
    try { u = new URL(String(url), location.href); } catch (_) { u = null; }
    if (u && u.origin === location.origin && u.pathname === PRESENTER_PATH && window.dinahContentWindows && window.dinahContentWindows.openPresenter) {
      window.dinahContentWindows.openPresenter(u.searchParams.get('resource'), u.searchParams.get('channel'))
        .catch(function (err) { if (window.dinahHost) window.dinahHost.toast(String(err && err.message || err), 'danger'); });
      // What the opener needs of a popup: that it exists.
      return { closed: false, focus: function () {}, close: function () {} };
    }
    return browserOpen.apply(window, arguments);
  };

  /* ---- A show on another display -----------------------------------------
     From version 2 of the presentation contract (yavc.app docs/
     presentation-contract.md) Slides asks its host to run the show somewhere
     else, and drives it through a port. Dinah runs it in a second copy of
     the window program, full screen on a display the window is not on
     (docs/design/detached-slide-show.md; Paul, 10 November 2026). With one
     display the answer is null, at once, and Slides presents in its own
     window as before. The show's page is in another browser process, so
     the program carries the messages: PostShow one way, and the show:message
     event the other. A message posted before the show's page reads is kept
     for it by the program. Closing the port, or the deck's window, ends the
     show; a show's window the person closes is the port's onclose. */
  var shows = new Map(), listeningShows = false;
  function listenShows() {
    if (listeningShows || !window.dinahHost) return;
    listeningShows = true;
    window.dinahHost.on('show:message', function (e) {
      var show = e && shows.get(e.id), message;
      if (!show || typeof show.port.onmessage !== 'function') return;
      try { message = JSON.parse(e.message); } catch (_) { return; }
      show.port.onmessage(message);
    });
    window.dinahHost.on('show:closed', function (e) {
      var show = e && shows.get(e.id);
      if (!show) return;
      shows.delete(e.id);
      if (!show.closed && typeof show.port.onclose === 'function') show.port.onclose();
    });
  }
  function openShow(request) {
    var resource = String(request && request.resource || ''), id = names()[resource];
    if (!id || !api().OpenShow) return Promise.resolve(null);
    listenShows();
    return Promise.resolve(api().OpenShow({ application: String(request.application || 'slides'), resource: resource, draft: id, folder: folders.get(resource) || '',
      mode: document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light' })).then(function (opened) {
      if (!opened || !opened.id) return null;
      var show = { resource: resource, closed: false, sending: Promise.resolve(), port: null };
      show.port = { onmessage: null, onclose: null,
        postMessage: function (message) {
          if (show.closed) return;
          var text = JSON.stringify(message);
          show.sending = show.sending.then(function () { return api().PostShow(opened.id, text); }).catch(function () { /* The show has ended, which onclose says. */ });
        },
        close: function () {
          if (show.closed) return;
          show.closed = true;
          /* What was posted is passed on before the window goes. */
          show.sending.then(function () { return api().CloseShow(opened.id); }).catch(function () {});
        } };
      shows.set(opened.id, show);
      return show.port;
    }, function (err) {
      if (window.dinahHost && window.dinahHost.toast) window.dinahHost.toast(message(err), 'danger');
      return null;
    });
  }
  /* A deck's window that closes takes its show with it. */
  function closeShowsOf(resource) { shows.forEach(function (show) { if (show.resource === resource) show.port.close(); }); }

  function clearSnapshot(key) { store().set('content-snapshot:' + key, null); }

  window.dinahContentHost = { install: install, accept: accept, read: read, apps: APPS, resourceFor: resourceFor, forget: forget,
    clearSnapshot: clearSnapshot, openShow: openShow, knownRevision: knownRevision, keepMine: keepMine, draftOf: function (resource) { return names()[resource] || ''; } };
})();
