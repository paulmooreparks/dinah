/* Windows for the yavc.app content applets (content-host.js). Each window
   shows one Dinah draft in one applet, the way Dinah's own Editor does, and
   never holds tabs (docs/design/content-applets.md, ruling 6). Its address
   is the draft, so a draft already open raises its window.

   A change to a document that has a file is saved to it a moment after the
   person stops typing, and a new document's text is kept in its draft, so
   closing a window asks nothing (ruling 3). Save As is Dinah's own, through
   its file picker and the draft service.

   While the new Editor is being adopted it is chosen by the setting
   "content-editor" in the UI state; Dinah's own Editor stays the default. */
(function () {
  'use strict';

  var helpers = window.dinahHelpers;
  function api() { return helpers.app(); }
  function host() { return window.dinahHost; }
  function message(err) { return String(err && err.message || err).replace(/^Error: /, ''); }
  function productName() { return window.dinahProduct ? window.dinahProduct.name : ''; }
  var SAVE_DELAY = 800;

  function kindOf(app) { return 'content-' + app; }
  function itemFor(app, v) { return { kind: kindOf(app), workspace: v.origin.workspaceId || '', path: v.id, title: v.title }; }

  function workspaceName(id) {
    var records = window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.records() : [];
    var owner = records.find(function (r) { return r.id === id; });
    return owner ? owner.name : '';
  }
  function itemOfWindow(key) {
    var root = key && document.querySelector('[data-win="' + CSS.escape(key) + '"] [data-item]');
    try { return root ? JSON.parse(root.dataset.item) : null; } catch (e) { return null; }
  }

  /* ---- One window ------------------------------------------------------ */

  function init(app, root, opts, item) {
    var key = opts.key || root.closest('[data-win]').dataset.win;
    var mount = null, resource = '', disposed = false, saveTimer = 0, saving = null, lastError = '';
    var draft = { id: item.path, origin: { workspaceId: item.workspace }, title: item.title };
    root.classList.add('content-window');

    function call(operation, input) { return window.dinahContentHost.accept(key, { operation: operation, input: input || {} }); }
    function current() { return call('documents').then(function (r) { var docs = r.output.documents || []; return docs[0] || null; }); }

    function retitle(doc) {
      if (disposed || !doc) return;
      window.dinahWorkbench.retitle(key, doc.name + (doc.modified ? ' (changed)' : ''), workspaceName(draft.origin.workspaceId));
    }
    function failed(err) {
      var text = message(err);
      if (text === lastError || !host()) return;
      lastError = text;
      host().toast(text, 'danger');
    }

    /* A document with a file is written back after each pause in typing;
       a new one's text goes to its draft, which keeps it until Save As. */
    function save() {
      if (disposed) return Promise.resolve();
      if (saving) return saving.then(save);
      saving = current().then(function (doc) {
        retitle(doc);
        if (!doc || !doc.modified || doc.readOnly) return;
        return call('save').then(function () { lastError = ''; return current(); }).then(retitle);
      }).catch(refused).finally(function () { saving = null; });
      return saving;
    }
    function changed() {
      clearTimeout(saveTimer);
      saveTimer = setTimeout(save, SAVE_DELAY);
    }
    function flush() { clearTimeout(saveTimer); return save(); }

    function mountApplet() {
      mount = document.createElement('div');
      mount.className = 'content-applet';
      mount.setAttribute('data-applet', app);
      mount.setAttribute('data-applet-flush', '');
      resource = window.dinahContentHost.resourceFor(draft);
      mount.dataset.contentResource = resource;
      root.appendChild(mount);
      window.pudlApplets.boot(mount);
    }
    function unmountApplet() {
      if (!mount) return;
      window.pudlApplets.destroy(mount);
      mount.remove();
      mount = null;
    }

    root.addEventListener('dinah:content-changed', changed);

    /* ---- Changes made elsewhere ------------------------------------------
       An agent may change the draft, and another program the file, while
       the window is open, as with Dinah's own Editor (editor-draft.js and
       editor-saving.js). A window with nothing unsaved shows the change; one
       with typing not yet saved asks which text to keep; a file changed on
       disk offers to bring that change into the draft. */
    var notices = document.createElement('div');
    notices.className = 'content-notices';
    notices.innerHTML =
      '<div class="notice warn" data-content-outside hidden><div class="notice-content"><p class="notice-body" data-content-outside-body></p>' +
        '<div class="notice-actions"><button type="button" class="btn btn-sm" data-content-theirs>Take Theirs</button>' +
        '<button type="button" class="btn btn-sm btn-primary" data-content-mine>Keep Mine</button></div></div></div>' +
      '<div class="notice warn" data-content-moved hidden><div class="notice-content"><p class="notice-body" data-content-moved-body></p>' +
        '<div class="notice-actions"><button type="button" class="btn btn-sm btn-primary" data-content-bring>Bring In Their Changes</button></div></div></div>';
    root.appendChild(notices);
    function $(name) { return notices.querySelector('[data-content-' + name + ']'); }

    function reload() {
      if (disposed) return Promise.resolve();
      unmountApplet();
      mountApplet();
      return current().then(retitle);
    }
    /* A change made to the draft elsewhere is saved to the file as typing
       is, since a document's changes are always saved. */
    function saveDraft() {
      return Promise.resolve(api().GetDraft(draft.id)).then(function (v) {
        draft = v;
        if (!v.changed || v.writeBack !== 'save') return;
        return Promise.resolve(api().SaveDraft(v.id, v.revision)).then(function (saved) { draft = saved; });
      }).catch(refused);
    }
    function showOutside() {
      $('outside-body').textContent = 'This document was changed elsewhere in ' + productName() + ', by an agent, while you had changes not yet saved. Keep Mine replaces that change with your text; Take Theirs replaces your latest changes with it.';
      $('outside').hidden = false;
    }
    $('theirs').addEventListener('click', function () { $('outside').hidden = true; reload().catch(failed); });
    $('mine').addEventListener('click', function () {
      $('outside').hidden = true;
      window.dinahContentHost.keepMine(resource);
      flush();
    });
    function draftChanged(e) {
      if (!e || e.id !== draft.id || disposed || !mount) return;
      if (e.kind === 'discarded') {
        if (host()) host().toast('The draft of "' + (draft.title || 'the document') + '" was discarded elsewhere, so its window has closed.', 'warn');
        allowClose = true;
        window.pudlWindows.close(key);
        return;
      }
      if (saving) { saving.then(function () { draftChanged(e); }); return; }
      Promise.resolve(api().GetDraft(draft.id)).then(function (latest) {
        if (disposed) return;
        draft = latest;
        if (latest.revision === window.dinahContentHost.knownRevision(resource)) return checkMoved(false);
        /* The event may come from this window's own save, before the save's
           reply, so the text decides, not the order. */
        return call('document').then(function (r) {
          var doc = r.output;
          if (doc && doc.text === latest.text) return checkMoved(false);
          if (doc && doc.modified) showOutside();
          else return reload().then(saveDraft).then(function () { if (host()) host().toast('"' + doc.name + '" was changed elsewhere, and shows that change.', 'info'); });
        });
      }).catch(function () { /* The next check tries again. */ });
    }
    var unsubscribe = host() ? host().on('draft:changed', draftChanged) : function () {};

    function checkMoved(afterRefusal) {
      if (!draft.origin || draft.origin.kind === 'new' || draft.writeBack !== 'save' || disposed) return Promise.resolve();
      return Promise.resolve(api().CompareDraft(draft.id)).then(function (c) {
        if (disposed) return;
        if (!c.moved) { $('moved').hidden = true; return; }
        var n = (c.conflicts || []).length;
        /* With nothing of the person's in the way, the window simply takes
           the file as it now is, as an agent that edits files on disk
           expects. */
        if (!afterRefusal && !draft.changed) {
          return current().then(function (doc) {
            if (doc && doc.modified) return showMoved(n, false);
            return Promise.resolve(api().MergeIntoDraft(draft.id, draft.revision)).then(function (v) {
              draft = v;
              return reload();
            });
          });
        }
        return showMoved(n, afterRefusal);
      }).catch(function () { /* The next check tries again. */ });
    }
    function showMoved(n, afterRefusal) {
      $('moved-body').textContent = '"' + (draft.title || 'This document') + '" was changed on disk since you opened it' + (afterRefusal ? ', so your changes were not saved. ' : '. ') +
        (n ? 'You both changed ' + (n === 1 ? 'one part' : n + ' parts') + ' of it. Change your text to settle ' + (n === 1 ? 'it' : 'them') + ', and their other changes can then be brought in.' :
          'Your changes and theirs do not touch, so theirs can be brought into your draft.');
      $('bring').hidden = n > 0;
      $('moved').hidden = false;
    }
    $('bring').addEventListener('click', function () {
      flush().then(function () { return api().GetDraft(draft.id); }).then(function (v) {
        return api().MergeIntoDraft(draft.id, v.revision);
      }).then(function (v) {
        draft = v;
        $('moved').hidden = true;
        return reload();
      }).then(function () { return flush(); }).catch(failed);
    });
    /* A refused save is a change made elsewhere, or a file changed on disk,
       or a plain failure, which is shown. */
    function refused(err) {
      return Promise.resolve(api().GetDraft(draft.id)).then(function (latest) {
        draft = latest;
        if (latest.revision !== window.dinahContentHost.knownRevision(resource)) return showOutside();
        return Promise.resolve(api().CompareDraft(draft.id)).then(function (c) { if (c.moved) return checkMoved(true); failed(err); });
      }).catch(function () { failed(err); });
    }

    /* Another program changes drafts without Dinah's events, so the window
       compares its draft's stamp every few seconds while it can be seen. */
    var lastStamp = '', checking = false, ticks = 0;
    var watchTimer = setInterval(function () {
      if (disposed || !mount || checking || document.hidden || saving) return;
      var st = window.pudlWindows.state();
      if (st.min && st.min[key]) return;
      if (st.top !== key && (++ticks % 4) !== 0) return;
      checking = true;
      Promise.resolve(api().DraftStamp(draft.id)).then(function (stamp) {
        var was = lastStamp;
        lastStamp = stamp;
        if (was && stamp !== was) draftChanged({ id: draft.id, kind: 'changed' });
        else return checkMoved(false);
      }).catch(function () {}).then(function () { checking = false; });
    }, 4000);

    /* Closing never asks; it only waits for the last change to be saved.
       A save that fails keeps the window open, with the failure shown. */
    var win = root.closest('[data-win]'), allowClose = false, closing = false;
    function closingWindow(e) {
      if (e.target !== win || allowClose || !mount) return;
      e.preventDefault();
      if (closing) return;
      closing = true;
      clearTimeout(saveTimer);
      var ok = true;
      (saving || Promise.resolve()).then(function () {
        return current().then(function (doc) {
          if (!doc || !doc.modified || doc.readOnly) return;
          return call('save').catch(function (err) { ok = false; failed(err); });
        });
      }).catch(function (err) { ok = false; failed(err); }).then(function () {
        closing = false;
        if (ok && !disposed) { allowClose = true; window.pudlWindows.close(key); }
      });
    }
    if (win) win.addEventListener('pudl:window-closing', closingWindow);

    var ready = window.dinahContentHost.install().then(function () {
      return api().GetDraft(draft.id);
    }).then(function (v) {
      if (disposed) return;
      draft = v;
      mountApplet();
      return current().then(retitle);
    });

    /* Save As puts the draft somewhere new; the applet then reads it again,
       under its new name. */
    function saveAs() {
      return flush().then(function () { return Promise.resolve(api().GetDraft(draft.id)); }).then(function (v) {
        draft = v;
        var start = window.dinahResources ? window.dinahResources.currentPath() : '/';
        return window.dinahFilePicker.pick({ mode: 'save', title: 'Save As', start: start, name: v.origin.path ? v.origin.path.split('/').pop() : (v.title || 'Untitled') + '.md',
          saveKind: window.dinahResources ? window.dinahResources.documentHomes() : undefined,
          commit: function (choice) {
            if (!choice.dir) return false;
            var ref = choice.dir.ref;
            var target = { workspaceId: ref.workspaceId, sourceId: ref.sourceId, path: (ref.path && ref.path !== '.' ? ref.path + '/' : '') + choice.name };
            return Promise.resolve(api().SaveDraftAs(draft.id, draft.revision, target)).then(function (saved) {
              draft = saved;
              unmountApplet();
              /* The applet's recovery snapshot is of the document under its
                 old name, so it starts again from the draft. */
              window.dinahContentHost.clearSnapshot(key);
              window.dinahContentHost.forget(resource);
              mountApplet();
              return current().then(retitle).then(function () { return true; });
            });
          } });
      }).catch(failed);
    }

    var controller = {
      key: key,
      ref: function () { var o = draft.origin || {}; return o.path ? { workspaceId: o.workspaceId, sourceId: o.sourceId, path: o.path } : null; },
      saveLabel: function () { return draft.origin && draft.origin.kind === 'new' ? 'Save As…' : 'Save'; },
      canSave: function () { return true; },
      whyNot: function () { return ''; },
      canRename: function () { return false; },
      rename: function () {},
      save: function () { return draft.origin && draft.origin.kind === 'new' ? saveAs() : flush(); },
      canSaveAs: function () { return true; },
      saveAs: saveAs,
      close: function () { window.pudlWindows.close(key); },
      discard: function () {}
    };
    if (!opts.embedded && window.dinahDocuments) window.dinahDocuments.attach(key, controller);

    return {
      ready: ready,
      flush: flush,
      menus: function () {
        var src = mount && window.pudlApplets.menuSourceIn(mount);
        return src && src.menus ? src.menus() || { titles: [] } : { titles: [] };
      },
      destroy: function () {
        disposed = true;
        clearTimeout(saveTimer);
        if (window.dinahDocuments) window.dinahDocuments.detach(key);
        root.removeEventListener('dinah:content-changed', changed);
        unsubscribe();
        clearInterval(watchTimer);
        if (win) win.removeEventListener('pudl:window-closing', closingWindow);
        /* Only a window that closes is destroyed; a page going away with
           the application keeps its windows for the next start. */
        var was = resource;
        unmountApplet();
        if (was) window.dinahContentHost.forget(was);
      }
    };
  }

  var TITLES = { editor: 'Editor', docs: 'Docs', diagrams: 'Diagrams', slides: 'Slides' };
  var GLYPHS = { editor: 'document', docs: 'document', diagrams: 'diagram', slides: 'slides' };
  Object.keys(TITLES).forEach(function (app) {
    window.dinahUIState.followWindowMode(kindOf(app), 'editor');
    window.dinahWorkbench.register(kindOf(app), { pinByItem: true, title: TITLES[app], glyph: GLYPHS[app], start: 'idle',
      init: function (root, opts, item) { return init(app, root, opts, item); } });
  });

  /* ---- Opening ------------------------------------------------------------ */

  /* The application a document opens in. Docs, Diagrams and Slides take
     their own documents as they are adopted; until then the Editor opens
     everything. */
  function applicationFor() { return 'editor'; }

  /* A document is open in one window at a time, so opening it in another
     application moves it there: the window it was in saves its last change
     and closes first. */
  function closed(key) {
    return new Promise(function (resolve) {
      function done(e) { if (e.detail && e.detail.key === key) { document.removeEventListener('pudl:window-close', done); resolve(); } }
      document.addEventListener('pudl:window-close', done);
      window.pudlWindows.close(key);
      setTimeout(function () { document.removeEventListener('pudl:window-close', done); resolve(); }, 10000);
    });
  }
  function open(v, app) {
    var item = itemFor(app || applicationFor(v), v);
    var elsewhere = window.pudlWindows.state().open.filter(function (key) {
      var other = itemOfWindow(key);
      return other && other.path === v.id && other.kind !== item.kind && /^content-/.test(other.kind);
    });
    return Promise.all(elsewhere.map(closed)).then(function () {
      if (window.pudlWindows.state().open.some(function (key) { return elsewhere.indexOf(key) >= 0; })) {
        throw new Error('"' + (v.title || 'The document') + '" could not be saved in the window it was open in, so it stays there.');
      }
      return window.dinahWorkbench.open(item);
    });
  }
  function openResource(ref) { return Promise.resolve(api().OpenDraft(ref)).then(function (v) { return open(v); }); }
  function newDocument(workspaceId, text, app) {
    return Promise.resolve(api().NewEmptyDraft(workspaceId || (host() && host().workspaceId()) || '')).then(function (v) {
      if (!text) return v;
      return api().UpdateDraft(v.id, v.revision, text);
    }).then(function (v) { return open(v, app); });
  }

  /* An applet's request for a document of its own (content-host.js's
     documentWindows.open). New and imported text become a new draft in the
     asking window's workspace, already in its new window; anything else is
     handed to the window it opens. */
  function route(application, request, fromKey) {
    var from = itemOfWindow(fromKey), workspace = from ? from.workspace : '';
    var input = request.input || {};
    if (request.operation === 'new' || request.operation === 'import-source') {
      return newDocument(workspace, input.text || '', application).then(function (key) { return { key: key, done: true }; });
    }
    if (request.operation === 'open-file' && input.path) {
      return Promise.resolve(api().GetDraft(String(input.path))).then(function (v) { return open(v, application); })
        .then(function (key) { return { key: key, done: false }; });
    }
    return Promise.reject(new Error(productName() + ' cannot open a window for ' + request.operation + '.'));
  }

  /* ---- Agents --------------------------------------------------------------- */

  /* The content windows, most recently in front first, so an agent's action
     goes to the document the person last worked on in that application. */
  var recent = [];
  function noteFront(key) {
    if (!key || !/^content-/.test((itemOfWindow(key) || {}).kind || '')) return;
    recent = [key].concat(recent.filter(function (k) { return k !== key; }));
  }
  ['pointerdown', 'focusin'].forEach(function (type) {
    document.addEventListener(type, function (e) {
      var win = e.target && e.target.closest && e.target.closest('[data-win]');
      if (win) noteFront(win.dataset.win);
    }, true);
  });
  document.addEventListener('pudl:window-close', function (e) {
    var key = e.detail && e.detail.key;
    recent = recent.filter(function (k) { return k !== key; });
  });

  function recordOf(args) {
    var records = window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.records() : [];
    function norm(s) { return String(s || '').replace(/\\/g, '/').replace(/\/+$/, '').toLowerCase(); }
    var rec = records.find(function (r) { return args.workspaceId && r.id === args.workspaceId; }) ||
      records.find(function (r) { return norm(r.executionPath) === norm(args.workspace); });
    if (!rec) throw new Error('The workspace ' + args.workspace + ' is not open in ' + productName() + '.');
    return rec;
  }
  /* A document path an agent gives, as the path in the workspace's main
     folder it stands for. */
  function workspaceRef(rec, given) {
    var p = String(given || '').replace(/\\/g, '/');
    var root = String(rec.executionPath || '').replace(/\\/g, '/').replace(/\/+$/, '');
    if (root && p.toLowerCase().indexOf(root.toLowerCase() + '/') === 0) p = p.slice(root.length + 1);
    p = p.replace(/^\.\//, '').replace(/^\/+/, '');
    if (!p || p.split('/').some(function (part) { return part === '..'; })) throw new Error('Give a path inside the workspace, such as notes/plan.md.');
    var source = (rec.sources || []).find(function (s) { return s.id === rec.primarySourceId; }) || (rec.sources || []).find(function (s) { return s.kind === 'local-folder'; });
    if (!source) throw new Error('The workspace has no folder to open documents from.');
    return { workspaceId: rec.id, sourceId: source.id, path: p };
  }
  function frontWindow(app, rec) {
    var kind = kindOf(app);
    var open = window.pudlWindows.state().open.filter(function (key) {
      var item = itemOfWindow(key);
      return item && item.kind === kind && item.workspace === rec.id;
    });
    var key = recent.find(function (k) { return open.indexOf(k) >= 0; }) || open[open.length - 1];
    if (!key) throw new Error('No ' + TITLES[app] + ' document is open in this workspace; open one with yavc.' + app + '/open-file@1, or start one with yavc.' + app + '/new@1.');
    return key;
  }
  function firstDocument(key) {
    return window.dinahContentHost.accept(key, { operation: 'documents', input: {} }).then(function (r) { return (r.output.documents || [])[0]; });
  }
  function encodeData(data) {
    return Promise.resolve(data instanceof Blob ? data.arrayBuffer() : data).then(function (d) {
      var bytes = typeof d === 'string' ? new TextEncoder().encode(d) : d instanceof ArrayBuffer ? new Uint8Array(d) : new Uint8Array(d.buffer, d.byteOffset, d.byteLength);
      var text = '';
      for (var i = 0; i < bytes.length; i += 0x8000) text += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000));
      return btoa(text);
    });
  }
  function answer(result) {
    var artifacts = (result && result.artifacts) || [];
    return Promise.all(artifacts.map(function (a) {
      return encodeData(a.data).then(function (data) { return { key: a.key, name: a.name, mime: a.mime, data: data }; });
    })).then(function (list) { return { output: (result && result.output) || {}, artifacts: list }; });
  }

  /* An agent's call of a content action (applet_content.go). Opening and
     starting a document make a window of their own; closing closes its
     window; a call that works on text alone runs in the page's broker; every
     other call goes to the window in front. */
  function agentCall(args) {
    var app = args.app, name = String(args.action).split('/')[1], op = name.replace(/@\d+$/, '');
    var input = args.input || {}, rec = recordOf(args);
    return window.dinahContentHost.install().then(function () {
      if (op === 'open-file' || (args.handles && input.path)) {
        return Promise.resolve(api().OpenDraft(workspaceRef(rec, input.path))).then(function (v) { return open(v, app); }).then(function (key) {
          noteFront(key);
          return window.dinahWorkbench.ready(key).then(function () {
            return input.view ? window.dinahContentHost.accept(key, { operation: 'view', input: { mode: input.view } }) : null;
          }).then(function () { return firstDocument(key); });
        }).then(function (doc) { return args.handles ? { output: { opened: true, view: 'window' } } : { output: doc }; });
      }
      if (op === 'new' || op === 'new-template') {
        return newDocument(rec.id, op === 'new' ? input.text || '' : '', app).then(function (key) {
          noteFront(key);
          return window.dinahWorkbench.ready(key).then(function () {
            return op === 'new-template' ? window.dinahContentHost.accept(key, { operation: 'new-template', input: input }).then(function () { return firstDocument(key); }) : firstDocument(key);
          });
        }).then(function (doc) { return { output: doc }; });
      }
      if (!args.live) {
        return window.pudlApplets.invoke(args.action, input, { owner: { kind: 'agent' }, grant: 'all' }).then(function (record) {
          if (record.status !== 'done') throw new Error(record.error || args.action + ' ended ' + record.status + '.');
          return { output: record.output };
        });
      }
      var key = frontWindow(app, rec);
      if (op === 'close') {
        return window.dinahWorkbench.ready(key).then(function () {
          window.pudlWindows.close(key);
          return { output: { closed: true, open: 0 } };
        });
      }
      return window.dinahWorkbench.ready(key).then(function () {
        return window.dinahContentHost.accept(key, { operation: op, input: input });
      }).then(answer);
    });
  }
  window.dinahWorkbench.answerAsks('content', agentCall);

  function enabled() { return !!(window.dinahUIState && window.dinahUIState.get('content-editor', false)); }

  window.dinahContentWindows = { open: open, openResource: openResource, newDocument: newDocument, route: route, enabled: enabled, kindOf: kindOf };

  /* While the new Editor is chosen, the ways into the Editor open it. */
  var own = window.dinahEditor;
  if (own) {
    window.dinahEditor = Object.assign({}, own, {
      open: function (v, intent) { return enabled() ? open(v) : own.open(v, intent); },
      openResource: function (ref, intent) { return enabled() ? openResource(ref) : own.openResource(ref, intent); },
      newDocument: function () { return enabled() ? newDocument().catch(function (err) { host().toast(message(err), 'danger'); }) : own.newDocument(); }
    });
  }
})();
