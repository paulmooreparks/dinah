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

  /* The Editor's windows are of kind editor, as Dinah's Editor's always
     were, so a saved layout, a pin and the Editor's own window setting
     carry over; the others are content-docs, content-diagrams and
     content-slides. */
  function kindOf(app) { return app === 'editor' ? 'editor' : 'content-' + app; }
  function contentKind(kind) { return kind === 'editor' || /^content-/.test(kind || ''); }
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

  /* ---- What a window holds -------------------------------------------------
     Each application lists what it has open, and gives a document's text,
     under actions of its own names: the Editor and Docs their documents,
     Diagrams its diagrams, Slides its decks. These read them alike. */
  var OPS = { editor: { list: 'documents', text: 'document' }, docs: { list: 'documents', text: 'document' },
    diagrams: { list: 'diagrams', text: 'diagram' }, slides: { list: 'decks', text: 'text' } };
  function appOfWindow(key) {
    var item = itemOfWindow(key);
    if (!item || item.kind === 'editor') return 'editor';
    // A presenter window is Slides too (content-slides-presenter).
    return item.kind.replace(/^content-/, '').replace(/-presenter$/, '');
  }
  /* The window's document as its application lists it, for an agent. */
  function listed(key) {
    var ops = OPS[appOfWindow(key)];
    return window.dinahContentHost.accept(key, { operation: ops.list, input: {} }).then(function (r) { return ((r.output || {})[ops.list] || [])[0] || null; });
  }
  /* The same, in one shape for this file: its name, path, and whether it
     has changes not yet saved. */
  function shown(key) {
    return listed(key).then(function (d) {
      if (!d) return null;
      var path = d.path || null;
      return { name: d.name || d.title || (path ? String(path).split('/').pop() : 'Untitled'), path: path, modified: !!d.modified, readOnly: !!d.readOnly };
    });
  }
  function textOf(key) {
    return window.dinahContentHost.accept(key, { operation: OPS[appOfWindow(key)].text, input: {} }).then(function (r) { return (r.output || {}).text; });
  }

  /* ---- One window ------------------------------------------------------ */

  /* Each open window's remount, by its key, for an agent's save to a path. */
  var remounts = new Map();

  /* Each window's decision on a suggestion made in its text, by window key
     (decideFromText), for the host's suggestions.decide. */
  var decisions = new Map();
  function decideSuggestion(request, context) {
    var mount = context && context.host && context.host.closest ? context.host.closest('.content-applet') : null;
    if (!mount || !request || !request.resource) return Promise.reject(new Error('The suggestion belongs to no open window.'));
    var win = mount.closest('[data-win]'), decide = win && decisions.get(win.dataset.win);
    if (!decide || mount.dataset.contentResource !== String(request.resource)) return Promise.reject(new Error('The suggestion belongs to no open window.'));
    return decide(request);
  }

  /* The channel a presenter window was opened for, by its deck's draft;
     a presenter window with none was restored with the layout, not opened
     by a show, and closes itself (openPresenter, init). */
  var presenterChannels = new Map();

  function init(app, root, opts, item) {
    var key = opts.key || root.closest('[data-win]').dataset.win;
    var presenter = !!opts.presenter;
    if (presenter && !presenterChannels.has(item.path)) {
      setTimeout(function () { window.pudlWindows.close(key); }, 0);
      return { ready: Promise.resolve(), destroy: function () {} };
    }
    /* The host finds a window's applet by this key, which for an Editor
       embedded in another window (a skill's) is its own. */
    root.dataset.contentKey = key;
    var mount = null, resource = '', disposed = false, saveTimer = 0, saving = null, lastError = '';
    var draft = { id: item.path, origin: { workspaceId: item.workspace }, title: item.title };
    root.classList.add('content-window');

    function call(operation, input) { return window.dinahContentHost.accept(key, { operation: operation, input: input || {} }); }
    function current() { return shown(key); }

    /* The title is the document's name and its workspace, and nothing
       that comes and goes: a mark for unsaved typing changed the title at
       every pause in typing, which moved the title bar's words and redrew
       the taskbar and the docks for every window (Paul, 9 November 2026).
       Retitling redraws them, so the title is set only when it changes. */
    var titled = '';
    function retitle(doc) {
      if (disposed || !doc || opts.embedded) return;
      var title = (presenter ? 'Presenter · ' : '') + doc.name, detail = workspaceName(draft.origin.workspaceId);
      /* A page is published on the person's word, so the title says while
         the draft holds changes the page does not have yet. */
      if (draft.writeBack === 'publish' && draft.changed) detail = (detail ? detail + ' · ' : '') + 'Not published';
      if (titled === title + '\u0000' + detail) return;
      titled = title + '\u0000' + detail;
      window.dinahWorkbench.retitle(key, title, detail);
    }
    function failed(err) {
      var text = message(err);
      if (text === lastError || !host()) return;
      lastError = text;
      host().toast(text, 'danger');
    }

    /* A document with a file is written back after each pause in typing;
       a new one's text goes to its draft, which keeps it until Save As. */
    /* A save the applet refused (a recovered draft whose revision no longer
       matches, say) is not tried again for the same text: the applet keeps
       its recovery snapshot at each attempt, which this window took as a
       change and saved again, so the refusal and its toast came round every
       pause (seen on the real layout, 10 November 2026). The next change to
       the text tries again. */
    var refusedText = null;
    function save() {
      if (disposed) return Promise.resolve();
      if (saving) return saving.then(save);
      saving = current().then(function (doc) {
        retitle(doc);
        if (!doc || !doc.modified || doc.readOnly) return;
        return textOf(key).then(function (text) {
          if (refusedText !== null && text === refusedText) return;
          return call('save').then(function () { refusedText = null; lastError = ''; refreshProblemsSoon(); return refreshDraft(); }).then(current).then(retitle)
            .catch(function (err) { refusedText = text; throw err; });
        });
      }).catch(refused).finally(function () { saving = null; });
      return saving;
    }
    function changed() {
      clearTimeout(saveTimer);
      saveTimer = setTimeout(save, SAVE_DELAY);
      /* A window holds one document, so closing it (File › Close
         Document) closes the window. The applet then holds a blank
         document of its own, which no draft stands behind. */
      current().then(function (doc) {
        if ((!doc || !doc.path) && !disposed && mount) { allowClose = true; window.pudlWindows.close(key); }
      }).catch(function () {});
    }
    function flush() { clearTimeout(saveTimer); return save(); }
    /* The draft as the service now sees it, after the applet's text went
       into it; a document that publishes needs this to know whether there
       is anything to publish. */
    function refreshDraft() {
      if (draft.writeBack !== 'publish') return Promise.resolve(draft);
      return Promise.resolve(api().GetDraft(draft.id)).then(function (v) { draft = v; if (window.pudlCommands) window.pudlCommands.changed(); return v; });
    }
    /* Publish writes the draft to its page, after asking, since a page is
       read by others as soon as it is written. The applet's text goes into
       the draft first, and the draft service refuses the write when the page
       moved on since it was opened. */
    function unsentComments() { return !!(comments && comments.unsent()); }
    function publish() {
      return flush().then(function () { return api().GetDraft(draft.id); }).then(function (v) {
        draft = v;
        var sendsComments = unsentComments();
        if (!v.changed && !sendsComments) { if (host()) host().toast('"' + (v.title || 'The document') + '" has no changes to publish.', 'info'); return v; }
        var where = window.dinahWorkspaceBrowser && v.origin ? window.dinahWorkspaceBrowser.describe(v.origin) : '';
        var what = v.changed ? 'Your changes to "' + (v.title || 'the document') + '" are written to its page' + (where ? ' (' + where + ')' : '') + ', where everyone who reads the page sees them at once.' : 'The comments you wrote on "' + (v.title || 'the document') + '" are sent to its page' + (where ? ' (' + where + ')' : '') + ', where everyone who reads the page sees them.';
        if (v.changed && sendsComments) what += ' The comments you wrote go with them.';
        return host().ask('Publish This Document?', what, 'Publish', false).then(function (go) {
          if (!go || disposed) return v;
          return Promise.resolve(api().SaveDraft(v.id, v.revision)).then(function (saved) {
            draft = saved.view || saved;
            if (window.pudlCommands) window.pudlCommands.changed();
            if (host()) {
              host().toast('"' + (draft.title || 'The document') + '" was published.', 'success');
              (saved.notes || []).forEach(function (note) { host().toast(note, 'info'); });
            }
            if (comments) comments.reload();
            return current().then(retitle).then(function () { return draft; });
          });
        });
      }).catch(failed);
    }

    var body = null, commentsAside = null, comments = null;
    function mountApplet() {
      mount = document.createElement('div');
      mount.className = 'content-applet';
      mount.setAttribute('data-applet', app);
      mount.setAttribute('data-applet-flush', '');
      resource = window.dinahContentHost.resourceFor(draft);
      mount.dataset.contentResource = resource;
      if (presenter) mount.dataset.contentPresenter = presenterChannels.get(draft.id);
      /* The applet and the document's comment margin (document-comments.js)
         share a row; the margin is hidden until there is a comment. */
      if (!body) {
        body = document.createElement('div'); body.className = 'content-body';
        root.appendChild(body);
      }
      body.appendChild(mount);
      if (!commentsAside && !presenter && !opts.embedded && window.dinahDocumentComments) {
        commentsAside = document.createElement('aside'); commentsAside.className = 'content-comments'; commentsAside.setAttribute('aria-label', 'Comments');
        body.appendChild(commentsAside);
        comments = window.dinahDocumentComments.mount(commentsAside, { draftId: draft.id, api: api, host: host, applet: body, flush: flush, changed: function () { if (window.pudlCommands) window.pudlCommands.changed(); } });
      }
      /* The applet reports its state (file, view and caret line) at every
         change, and the workbench keeps every window's state in the address
         and the saved layout, so the caret moving a line wrote the layout
         to Go. Dinah's state for the window is the draft, which the mount
         is given (content-host.js), and the applet's own recovery snapshot
         keeps its view; its reports stay inside the window. */
      mount.addEventListener('pudl:applet-change', function (e) { e.stopPropagation(); });
      window.pudlApplets.boot(mount);
      quietSaves(mount);
    }
    /* The applet's status line says "Saving…" and "Saved name" at every
       write, and Dinah writes at every pause in typing, so the words came
       and went with the typing and moved the line. A document is always
       kept, so nothing needs saying; the line is blanked whenever it says
       one of those, and anything else it says (an error, a count) stays
       (Paul, 9 November 2026). */
    var quiet = null;
    function quietSaves(mount) {
      if (quiet) quiet.disconnect();
      quiet = new MutationObserver(function (records) {
        records.forEach(function (r) {
          var el = r.target.nodeType === 1 ? r.target : r.target.parentNode;
          var line = el && el.closest && el.closest('[data-role="message"]');
          if (line && /^(Saving…|Saved\s)/.test(line.textContent)) line.textContent = '';
        });
      });
      quiet.observe(mount, { subtree: true, childList: true, characterData: true });
    }
    function unmountApplet() {
      if (!mount) return;
      if (quiet) { quiet.disconnect(); quiet = null; }
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
        '<div class="notice-actions"><button type="button" class="btn btn-sm" data-content-compare>Compare…</button>' +
        '<button type="button" class="btn btn-sm btn-primary" data-content-bring>Bring In Their Changes</button></div></div></div>';
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
    /* ---- Files added to the document --------------------------------------
       Insert Image… and Attach File… (the Documents menu), or an image pasted
       or a file dropped into the window, keep the file with the draft, which
       writes it beside the document when it is saved (drafts.Service.Save),
       and put a reference to it at the cursor through the applet's own
       insert: an image as an image, any other file as a link, as the built-in
       Editor did. The Editor and Docs insert at the cursor; Slides adds to
       the end of the current slide, a picture as its figure shortcode;
       Diagrams has no such insert, so the commands stand aside for it. */
    var IMAGE_TYPES = /^image\/(png|jpeg|gif|webp|svg\+xml)$/;
    var BACKSLASH = String.fromCharCode(92);
    function canAttach() { return app === 'editor' || app === 'docs' || app === 'slides'; }
    // A shortcode argument with a space or a quote in it is quoted (the kit's quoteArg).
    function quoteArg(v) { return /[\s"]/.test(v) ? '"' + v.replace(/(["\\])/g, BACKSLASH + '$1') + '"' : v; }
    function fileMarkdown(name, image) {
      var target = /[\s()<>]/.test(name) ? '<' + name + '>' : name;
      if (!image) return '[' + name.replace(/([\\[\]])/g, BACKSLASH + '$1') + '](' + target + ')';
      if (app === 'slides') return '{{figure:' + quoteArg(name) + ' alt=""}}';
      return '![](' + target + ')';
    }
    function insertFiles(names, images) {
      if (!names || !names.length || disposed) return Promise.resolve();
      var markdown = names.map(function (n, i) { return fileMarkdown(n, Array.isArray(images) ? images[i] : images); }).join('\n\n');
      // Slides' insert takes markdown and adds to the current slide; the Editor's and Docs' take text at the cursor.
      return call('insert', app === 'slides' ? { markdown: markdown } : { text: markdown }).then(function () { changed(); });
    }
    function pickFiles(images) {
      if (!canAttach()) return Promise.resolve();
      return Promise.resolve(api().PickDraftFiles(draft.id, images)).then(function (names) { return insertFiles(names, images); }).catch(failed);
    }
    function readBase64(file) {
      return new Promise(function (resolve, reject) {
        var r = new FileReader();
        r.onload = function () { var s = String(r.result); resolve(s.slice(s.indexOf(',') + 1)); };
        r.onerror = function () { reject(r.error || new Error('The file could not be read.')); };
        r.readAsDataURL(file);
      });
    }
    function addFiles(list) {
      var files = Array.from(list || []);
      return Promise.all(files.map(function (f) {
        // A pasted image has no name of its own.
        var name = f.name && f.name !== 'image.png' ? f.name : 'pasted-' + new Date().toISOString().replace(/[:.]/g, '-') + '.' + ((f.type.split('/')[1] || 'png').replace('svg+xml', 'svg').replace('jpeg', 'jpg'));
        return readBase64(f).then(function (data) { return api().AddDraftFile(draft.id, name, data); });
      })).then(function (names) {
        return insertFiles(names, files.map(function (f) { return IMAGE_TYPES.test(f.type); }));
      }).catch(failed);
    }
    // The text (CodeMirror), the page (ProseMirror) and a slide's stage take a drop; the applets' own fields do not.
    function inDocument(target) { return !!(target && target.closest && target.closest('.cm-editor, .ProseMirror, .deck-stage')) && !target.closest('textarea, input'); }
    function filesOf(transfer) { return transfer ? Array.from(transfer.files || []) : []; }
    root.addEventListener('paste', function (e) {
      var files = filesOf(e.clipboardData).filter(function (f) { return IMAGE_TYPES.test(f.type); });
      if (!canAttach() || !files.length || !inDocument(e.target)) return;
      e.preventDefault(); e.stopPropagation();
      addFiles(files);
    }, true);
    root.addEventListener('drop', function (e) {
      var files = filesOf(e.dataTransfer);
      if (!canAttach() || !files.length || !inDocument(e.target)) return;
      e.preventDefault(); e.stopPropagation();
      addFiles(files);
    }, true);
    root.addEventListener('dragover', function (e) {
      if (canAttach() && filesOf(e.dataTransfer).length && inDocument(e.target)) e.preventDefault();
    }, true);

    /* ---- Suggestions -----------------------------------------------------
       An agent's suggestions (dinah.documents/suggest@1) are kept beside the
       draft's text, never in it, until the person decides. The panel above
       the document lists them, each with the text it would remove and the
       text it would put in, who suggested it and why, and Accept, Reject
       and Edit; Accept All and Reject All decide them all. A suggestion
       whose text has changed since can only be rejected. Accepting changes
       the draft, which the window then shows and saves (draftChanged). */
    var review = document.createElement('section');
    review.className = 'content-review card';
    review.setAttribute('aria-label', 'Suggestions');
    review.hidden = true;
    notices.appendChild(review);
    var deciding = false, editing = '';

    /* ---- Problems --------------------------------------------------------
       Dinah's own checks of a document (the draft preview's problems: a
       skill's package rules, a table row with more cells than its header)
       are listed under a button that says how many there are, as the
       built-in Editor listed them. The applet's own language checks stay
       in the applet. A problem with a line goes to it. */
    var problemsBox = document.createElement('section');
    problemsBox.className = 'content-problems';
    problemsBox.setAttribute('aria-label', 'Problems');
    problemsBox.hidden = true;
    var problemsButton = document.createElement('button');
    problemsButton.type = 'button';
    problemsButton.className = 'btn btn-sm';
    problemsButton.setAttribute('aria-expanded', 'false');
    var problemsList = document.createElement('ul');
    problemsList.className = 'content-problems-list';
    problemsList.hidden = true;
    problemsBox.append(problemsButton, problemsList);
    notices.appendChild(problemsBox);
    problemsButton.addEventListener('click', function () { showProblemList(problemsList.hidden); });
    function showProblemList(open) {
      problemsList.hidden = !open;
      problemsButton.setAttribute('aria-expanded', String(open));
    }
    /* The check renders the whole document in Go, so it runs once the
       document has been still for a while, and once per revision. */
    var problemsAsked = 0, problemsFor = '', problemsTimer = 0;
    function refreshProblemsSoon() { clearTimeout(problemsTimer); problemsTimer = setTimeout(refreshProblems, 3000); }
    function refreshProblems() {
      clearTimeout(problemsTimer); problemsTimer = 0;
      if (!api().PreviewDraft || disposed || problemsFor === draft.revision) return Promise.resolve();
      var asked = ++problemsAsked, revision = draft.revision;
      return Promise.resolve().then(function () { return api().PreviewDraft(draft.id); }).then(function (p) {
        if (disposed || asked !== problemsAsked) return;
        problemsFor = revision;
        var list = ((p && p.problems) || []).slice().sort(function (a, b) { return (a.line || 0) - (b.line || 0); });
        problemsBox.hidden = !list.length;
        problemsButton.textContent = list.length === 1 ? '1 Problem' : list.length + ' Problems';
        problemsButton.classList.toggle('btn-danger', list.some(function (x) { return x.severity === 'error'; }));
        problemsList.replaceChildren();
        list.forEach(function (x) {
          var item = el('li', 'content-problem');
          var go = el('button', 'content-problem-go');
          go.type = 'button';
          if (x.line) go.appendChild(el('span', 'content-problem-line', 'Line ' + x.line));
          go.appendChild(el('span', 'badge ' + (x.severity === 'error' ? 'danger' : x.severity === 'warning' ? 'warn' : 'info'), x.severity));
          go.appendChild(el('span', 'content-problem-message', x.message));
          go.addEventListener('click', function () { if (x.line) call('go-to', { line: x.line }).catch(function () {}); });
          item.appendChild(go);
          problemsList.appendChild(item);
        });
        if (!list.length) showProblemList(false);
      }).catch(function () { /* The next change asks again. */ });
    }
    /* Check (a skill's) saves what was typed, asks again and shows the list. */
    function check() {
      return flush().then(refreshProblems).then(function () { if (!problemsBox.hidden) showProblemList(true); });
    }

    function el(tag, cls, text) {
      var n = document.createElement(tag);
      if (cls) n.className = cls;
      if (text != null) n.textContent = text;
      return n;
    }
    function button(label, cls, run, disabled) {
      var b = el('button', 'btn btn-sm' + (cls ? ' ' + cls : ''), label);
      b.type = 'button';
      b.disabled = !!disabled || deciding;
      b.addEventListener('click', run);
      return b;
    }
    /* The part of a change that differs: the words two texts begin with
       alike and end with alike, and what is between them in each. A text
       that is wholly new or wholly gone has no such words. */
    function changedPart(was, now) {
      if (!was || !now) return { head: '', was: was, now: now, tail: '' };
      var a = was.split(/(\s+)/), b = now.split(/(\s+)/), start = 0, end = 0;
      while (start < a.length && start < b.length && a[start] === b[start]) start++;
      while (end < a.length - start && end < b.length - start && a[a.length - 1 - end] === b[b.length - 1 - end]) end++;
      return { head: a.slice(0, start).join(''), was: a.slice(start, a.length - end).join(''), now: b.slice(start, b.length - end).join(''), tail: a.slice(a.length - end).join('') };
    }
    function drawReview() {
      var list = (draft.suggestions || []);
      review.hidden = !list.length;
      review.replaceChildren();
      if (!list.length) return;
      var live = list.filter(function (s) { return !s.stale; });
      var head = el('div', 'content-review-head');
      head.appendChild(el('h3', 'content-review-title', list.length === 1 ? '1 Suggestion' : list.length + ' Suggestions'));
      var all = el('div', 'content-review-all');
      all.appendChild(button('Reject All', '', function () { decideAll(false); }));
      all.appendChild(button('Accept All', 'btn-primary', function () { decideAll(true); }, !live.length));
      head.appendChild(all);
      review.appendChild(head);
      var items = el('ol', 'content-review-list');
      list.forEach(function (s) {
        var item = el('li', 'content-review-item' + (s.stale ? ' is-stale' : ''));
        var who = el('p', 'content-review-who', (s.author || 'An agent') + (s.edited ? ', edited by you' : '') + (s.reason ? ': ' + s.reason : ''));
        item.appendChild(who);
        var change = el('p', 'content-review-change');
        /* Only the words beside the change on its own line are shown
           around it, and a paragraph's closing line breaks are left out. */
        var before = String(s.prefix || '').split('\n').pop().slice(-60), after = String(s.suffix || '').split('\n')[0].slice(0, 60);
        /* A suggestion replaces whole lines, and a line may be a long
           paragraph or a row of a table of which one word changed. The
           words the two texts begin and end with alike are shown as what
           is around the change, so that what is struck and what is put in
           are the words that differ. While it is edited the whole
           suggested text is in the field, so the whole old text is shown. */
        var was = String(s.before || '').replace(/\n+$/, ''), now = String(s.after || '').replace(/\n+$/, '');
        var part = editing === s.id ? { head: '', was: was, now: now, tail: '' } : changedPart(was, now);
        if (part.head) before = (before + part.head).slice(-60);
        if (part.tail) after = (part.tail + after).slice(0, 60);
        if (before) change.appendChild(el('span', 'content-review-context', '…' + before));
        if (part.was) change.appendChild(el('del', '', part.was));
        if (editing === s.id) {
          var field = el('textarea', 'form-input content-review-edit');
          field.value = s.after;
          field.setAttribute('aria-label', 'Suggested text');
          change.appendChild(field);
        } else if (part.now) change.appendChild(el('ins', '', part.now));
        if (after) change.appendChild(el('span', 'content-review-context', after + '…'));
        item.appendChild(change);
        if (s.stale) item.appendChild(el('p', 'content-review-stale', 'The text this would change has been changed since, so it can only be rejected.'));
        var actions = el('div', 'content-review-actions');
        if (editing === s.id) {
          actions.appendChild(button('Cancel', '', function () { editing = ''; drawReview(); }));
          actions.appendChild(button('Keep Edit', 'btn-primary', function () { revise(s, item.querySelector('textarea').value); }));
        } else {
          actions.appendChild(button('Edit', '', function () { editing = s.id; drawReview(); var f = review.querySelector('textarea'); if (f) f.focus(); }, s.stale));
          actions.appendChild(button('Reject', '', function () { decide(s, false); }));
          actions.appendChild(button('Accept', 'btn-primary', function () { decide(s, true); }, s.stale));
        }
        item.appendChild(actions);
        items.appendChild(item);
      });
      review.appendChild(items);
    }
    /* Typing not yet saved is saved first, so an accepted change is made to
       the person's latest text. */
    function settle(work) {
      if (deciding) return Promise.resolve();
      deciding = true;
      drawReview();
      return flush().then(work).then(function (v) {
        if (v) draft = v;
      }).catch(failed).finally(function () { deciding = false; editing = ''; drawReview(); });
    }
    function decide(s, accept) {
      return settle(function () {
        return Promise.resolve(api().GetDraft(draft.id)).then(function (v) {
          return accept ? api().AcceptSuggestion(v.id, v.revision, s.id, false) : api().RejectSuggestion(v.id, s.id, false);
        });
      });
    }
    /* Accept All accepts every suggestion still in place, proposal by
       proposal; Reject All rejects every one. */
    function decideAll(accept) {
      return settle(function () {
        var groups = [];
        (draft.suggestions || []).forEach(function (s) {
          if (accept && s.stale) return;
          if (!groups.some(function (g) { return g.group === s.group; })) groups.push(s);
        });
        return groups.reduce(function (chain, s) {
          return chain.then(function () { return api().GetDraft(draft.id); }).then(function (v) {
            if (!(v.suggestions || []).some(function (x) { return x.id === s.id; })) return v;
            return accept ? api().AcceptSuggestion(v.id, v.revision, s.id, true) : api().RejectSuggestion(v.id, s.id, true);
          });
        }, Promise.resolve());
      });
    }
    function revise(s, after) {
      return settle(function () { return api().ReviseSuggestion(draft.id, s.id, after); });
    }
    /* A decision made in the text (the applet's Accept, Reject or an edited
       replacement, content-host.js) is applied as the panel's are: typing
       not yet saved is written first, and the decision is refused when the
       text it was made against is no longer the draft's, or the suggestion
       is gone, rather than made to later typing. The applet then reads the
       new text and the new list through its ordinary paths. */
    function decideFromText(request) {
      if (deciding) return Promise.reject(new Error('Another decision is being applied.'));
      var accept = request.decision === 'accept';
      var fresh = null;
      return settle(function () {
        return Promise.resolve(api().GetDraft(draft.id)).then(function (v) {
          fresh = v;
          var s = (v.suggestions || []).filter(function (x) { return x.id === request.id; })[0];
          if (!s) throw new Error('That suggestion has been decided already.');
          if (typeof request.text === 'string' && request.text !== v.text) throw new Error('The document changed since; the suggestion is shown again where it is now.');
          if (typeof request.revision === 'string' && request.revision !== v.revision && typeof request.text !== 'string') throw new Error('The document changed since; the suggestion is shown again where it is now.');
          if (!accept) return api().RejectSuggestion(v.id, s.id, false);
          if (s.stale) throw new Error('The text this would change has been changed since, so it can only be rejected.');
          var revised = typeof request.after === 'string' && request.after !== s.after ? Promise.resolve(api().ReviseSuggestion(v.id, s.id, request.after)) : Promise.resolve(v);
          return revised.then(function (w) { return api().AcceptSuggestion(w.id, w.revision, s.id, false); });
        });
      }).then(function () { if (!fresh) throw new Error('The decision could not be applied.'); });
    }
    decisions.set(key, decideFromText);

    function draftChanged(e) {
      if (!e || e.id !== draft.id || disposed || !mount) return;
      if (e.kind === 'discarded') {
        if (host()) host().toast('The draft of "' + (draft.title || 'the document') + '" was discarded elsewhere, so its window has closed.', 'warn');
        allowClose = true;
        window.pudlWindows.close(key);
        return;
      }
      if (saving) { saving.then(function () { draftChanged(e); }); return; }
      /* A file renamed elsewhere, as from Files, keeps its draft; the window
         reads it again under the new name. */
      if (e.kind === 'renamed') {
        remount();
        return;
      }
      Promise.resolve(api().GetDraft(draft.id)).then(function (latest) {
        if (disposed) return;
        draft = latest;
        drawReview();
        refreshProblemsSoon();
        /* The window's own write; the watch below compares the file in
           its own time. */
        if (latest.revision === window.dinahContentHost.knownRevision(resource)) return;
        /* The event may come from this window's own save, before the save's
           reply, so the text decides, not the order. */
        return Promise.all([textOf(key), shown(key)]).then(function (both) {
          var doc = both[1] && { text: both[0], modified: both[1].modified, name: both[1].name };
          if (doc && doc.text === latest.text) return checkMoved(false);
          if (doc && doc.modified) return showOutside();
          /* A change the person made here, by deciding a suggestion, is
             shown without being announced. */
          var own = deciding;
          return reload().then(saveDraft).then(function () { if (!own && host()) host().toast('"' + doc.name + '" was changed elsewhere, and shows that change.', 'info'); });
        });
      }).catch(function () { /* The next check tries again. */ });
    }
    var unsubscribe = host() ? host().on('draft:changed', draftChanged) : function () {};

    var comparison = null;
    function checkMoved(afterRefusal) {
      if (!draft.origin || draft.origin.kind === 'new' || (draft.writeBack !== 'save' && draft.writeBack !== 'publish') || disposed) return Promise.resolve();
      return Promise.resolve(api().CompareDraft(draft.id)).then(function (c) {
        if (disposed) return;
        if (!c.moved) { $('moved').hidden = true; comparison = null; return; }
        comparison = c;
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
      $('moved-body').textContent = '"' + (draft.title || 'This document') + '" was changed ' + (draft.writeBack === 'publish' ? 'at its source' : 'on disk') + ' since you opened it' + (afterRefusal ? ', so your changes were not ' + (draft.writeBack === 'publish' ? 'published. ' : 'saved. ') : '. ') +
        (n ? 'You both changed ' + (n === 1 ? 'one part' : n + ' parts') + ' of it. Change your text to settle ' + (n === 1 ? 'it' : 'them') + ', and their other changes can then be brought in.' :
          'Your changes and theirs do not touch, so theirs can be brought into your draft.');
      $('bring').hidden = n > 0;
      $('moved').hidden = false;
    }
    function bringIn() {
      return flush().then(function () { return api().GetDraft(draft.id); }).then(function (v) {
        return api().MergeIntoDraft(draft.id, v.revision);
      }).then(function (v) {
        /* The merged text is the draft's now and is saved to the file at
           once, before the applet shows it: the applet counts nothing as
           modified after a reload, so a save through it would write nothing. */
        return v.changed && v.writeBack === 'save' ? api().SaveDraft(v.id, v.revision) : v;
      }).then(function (v) {
        draft = v; comparison = null;
        $('moved').hidden = true;
        var diff = window.pudlWindows.state().open.filter(function (k) { return k === diffKey(); })[0];
        if (diff) window.pudlWindows.close(diff);
        return reload();
      }).catch(failed);
    }
    $('bring').addEventListener('click', bringIn);
    /* Compare shows the draft's text and the file's, as it is now, side by
       side in Dinah's Diff window, with Bring In Their Changes there as well
       when the two touch no part in common; a modal dialog would hide the
       document it compares (docs/design/content-applets.md). */
    function diffKey() { return window.dinahWorkbench.key({ kind: 'diff', workspace: draft.origin && draft.origin.workspaceId || '', path: 'draft-moved:' + draft.id, title: draft.title }); }
    function compare() {
      var c = comparison;
      if (!c || !window.dinahDiff) return;
      var n = (c.conflicts || []).length, o = draft.origin || {};
      window.dinahDiff.open({
        key: 'draft-moved:' + draft.id, title: draft.title, workspaceId: o.workspaceId,
        detail: 'Yours, then the file as it is now' + (n ? ' · you both changed ' + (n === 1 ? 'one part' : n + ' parts') : ''),
        path: o.path, file: o.workspaceId && o.sourceId ? { workspaceId: o.workspaceId, sourceId: o.sourceId, path: o.path } : undefined,
        before: c.draft, after: c.current, readOnly: true,
        actions: n ? [] : [{ id: 'bring', label: 'Bring In Their Changes', glyph: 'download', scope: 'file', primary: true }],
        onAction: function (id) { if (id === 'bring') bringIn(); }
      }).catch(failed);
    }
    $('compare').addEventListener('click', compare);
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
      Promise.resolve().then(function () { return api().DraftStamp(draft.id); }).then(function (stamp) {
        var was = lastStamp;
        lastStamp = stamp;
        if (was && stamp !== was) draftChanged({ id: draft.id, kind: 'changed' });
        else return checkMoved(false);
      }).catch(function () {}).then(function () { checking = false; });
    }, 4000);

    /* Closing never asks; it only waits for the last change to be saved.
       A save that fails keeps the window open, with the failure shown. */
    var win = opts.embedded ? null : root.closest('[data-win]'), allowClose = false, closing = false;
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
      drawReview();
      refreshProblems();
      return current().then(retitle);
    }).then(function () {
      /* An embedding window may ask for a view, as a skill asks for Split. */
      if (opts.embedded && opts.state && !disposed) return call('view', { mode: opts.state }).catch(function () {});
    });

    /* Documents › Save As runs the applet's own Save As, which asks Dinah
       for the file (content-host.js), so the window keeps its undo history. */
    function saveAs() {
      return flush().then(function () {
        if (!mount) return;
        return window.pudlCommands.run('save-as', undefined, mount, { owner: { kind: 'person' } });
      }).catch(failed);
    }
    /* After Save As the draft has a new file, and its resource a new name. */
    function renamed(e) {
      var d = e.detail || {};
      if (d.draft !== draft.id || disposed) return;
      draft = d.view;
      resource = d.resource;
      if (mount) mount.dataset.contentResource = resource;
      current().then(retitle).catch(function () {});
    }
    document.addEventListener('dinah:content-renamed', renamed);
    /* A draft that moved to another file (an agent's save to a path, a
       rename here or elsewhere) is read again: the window mounts the applet
       afresh on it, under the new name. */
    function remount() {
      if (disposed) return Promise.resolve();
      /* Typing not yet saved is saved first, under the old name, which
         still leads to the same draft. */
      return flush().then(function () {
        if (disposed) return;
        return Promise.resolve(api().GetDraft(draft.id)).then(function (v) {
          draft = v;
          unmountApplet();
          window.dinahContentHost.clearSnapshot(key);
          if (resource) window.dinahContentHost.forget(resource);
          mountApplet();
          return current().then(retitle);
        });
      }).catch(failed);
    }
    remounts.set(key, remount);

    /* Rename gives the file a new name in its folder, through Dinah's own
       rename; the applet then reads the document under its new name. */
    function rename() {
      if (!draft.origin || draft.origin.kind !== 'local-folder') return;
      var o = draft.origin;
      return flush().then(function () {
        return window.dinahResources.renameFile({ ref: { workspaceId: o.workspaceId, sourceId: o.sourceId, path: o.path } });
      }).then(function (renamed) {
        var mine = renamed && (renamed.drafts || []).filter(function (v) { return v.id === draft.id; })[0];
        if (!mine || disposed) return;
        return remount();
      }).catch(failed);
    }
    /* Discard throws the draft away, after asking, and closes the window;
       the document itself is not changed. */
    function discard() {
      var isNew = draft.origin && draft.origin.kind === 'new';
      return host().ask('Discard This Draft?', 'Your changes to "' + (draft.title || 'the document') + '" that are not saved will be lost. ' +
        (isNew ? 'The document has not been saved anywhere.' : 'The document itself is not changed.'), 'Discard Draft', true).then(function (go) {
        if (!go || disposed) return;
        clearTimeout(saveTimer);
        return Promise.resolve(api().GetDraft(draft.id)).then(function (v) { return api().DiscardDraft(v.id, v.revision); }).then(function () {
          allowClose = true;
          window.pudlWindows.close(key);
        });
      }).catch(failed);
    }

    var controller = {
      key: key,
      ref: function () { var o = draft.origin || {}; return o.path ? { workspaceId: o.workspaceId, sourceId: o.sourceId, path: o.path } : null; },
      saveLabel: function () { return draft.origin && draft.origin.kind === 'new' ? 'Save As…' : draft.writeBack === 'publish' ? 'Publish…' : 'Save'; },
      canSave: function () { return draft.writeBack !== 'publish' || !!draft.changed || unsentComments(); },
      whyNot: function () { return draft.writeBack === 'publish' && !draft.changed && !unsentComments() ? 'There are no changes to publish.' : ''; },
      // A document that publishes offers Publish… in the Document menu.
      canPublish: function () { return draft.writeBack === 'publish' ? !!draft.changed || unsentComments() : null; },
      publish: publish,
      // The document's comments (document-comments.js).
      comments: function () { return comments; },
      canRename: function () { return !!draft.origin && draft.origin.kind === 'local-folder'; },
      rename: rename,
      save: function () { return draft.origin && draft.origin.kind === 'new' ? saveAs() : draft.writeBack === 'publish' ? publish() : flush(); },
      canSaveAs: function () { return true; },
      saveAs: saveAs,
      close: function () { window.pudlWindows.close(key); },
      discard: discard,
      // The applet's File menu has Save and Close Document of its own.
      ownsFile: true,
      canAttach: canAttach,
      insertImages: function () { return pickFiles(true); },
      attachFiles: function () { return pickFiles(false); }
    };
    if (!opts.embedded && window.dinahDocuments) window.dinahDocuments.attach(key, controller);

    return {
      ready: ready,
      check: check,
      flush: flush,
      /* The applet's own menus, and after them Dinah's Document menu, whose
         rows are the registry's (documents.js): what the applet's File menu
         has not, such as Rename, Insert Image, Attach File and Discard
         Draft. Alt+M opens it (O is Workspaces' letter). */
      menus: function () {
        var src = mount && window.pudlApplets.menuSourceIn(mount);
        var own = src && src.menus ? src.menus() || { titles: [] } : { titles: [] };
        var rows = window.dinahCommands ? window.dinahCommands.rows('document') : [];
        if (!rows.length) return own;
        /* What the applet adds to the window's own View menu (its into)
           is kept: it was dropped here, so the applets' View rows, Show
           the Toolbar and Customize Toolbar among them, were in no menu. */
        return Object.assign({}, own, { titles: (own.titles || []).concat([{ id: 'document', label: 'Document', mnemonic: 'M', glyph: 'document', items: rows }]) });
      },
      destroy: function () {
        if (comments) { comments.destroy(); comments = null; }
        disposed = true;
        clearTimeout(saveTimer);
        clearTimeout(problemsTimer);
        // The deck's presenter window goes with the deck's window.
        if (!presenter) {
          presenterChannels.delete(draft.id);
          window.pudlWindows.state().open.forEach(function (k) { var other = itemOfWindow(k); if (other && other.kind === 'content-slides-presenter' && other.path === draft.id) window.pudlWindows.close(k); });
        }
        if (window.dinahDocuments) window.dinahDocuments.detach(key);
        decisions.delete(key);
        root.removeEventListener('dinah:content-changed', changed);
        document.removeEventListener('dinah:content-renamed', renamed);
        remounts.delete(key);
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
  /* Each package's own picture (the icon its applet.json gives), which the
     taskbar, the docks, the switcher and the Window menu show in place of
     the glyph (PUDL's data-win-icon, and the destination's icon for a pin).
     It is a coloured picture and is drawn as one; the glyph stays as what
     is shown until it loads, or if it does not (Paul, 11 November 2026). */
  var ICONS = {};
  ['editor', 'docs', 'diagrams', 'slides'].forEach(function (app) { ICONS[app] = new URL('yavc/content/' + app + '/icon.svg', document.baseURI).href; });
  var MNEMONICS = { editor: 'E', docs: 'D', diagrams: 'I', slides: 'S' };
  var DESCRIPTIONS = { editor: 'Start an empty document, for notes or a script.', docs: 'Start an empty Markdown document, written as it will look.',
    diagrams: 'Start an empty diagram, drawn from text.', slides: 'Start an empty slide deck, written in Markdown.' };
  /* A presenter window of a deck (content-host.js, Presenting): Slides in
     its presenter role, for the deck of the window that is presenting. It
     is not pinned and not restored with the layout. */
  window.dinahWorkbench.register('content-slides-presenter', { pinnable: false, title: 'Presenter', glyph: GLYPHS.slides, icon: ICONS.slides, start: 'open',
    init: function (root, opts, item) { return init('slides', root, Object.assign({}, opts, { presenter: true }), item); } });
  function openPresenter(resource, channel) {
    var mount = Array.prototype.find.call(document.querySelectorAll('.content-applet'), function (m) { return m.dataset.contentResource === resource && !m.dataset.contentPresenter; });
    var win = mount && mount.closest('[data-win]'), item = win && itemOfWindow(win.dataset.win);
    if (!item) return Promise.reject(new Error('The deck to present is not open in a window.'));
    presenterChannels.set(item.path, channel || '');
    return window.dinahWorkbench.open({ kind: 'content-slides-presenter', workspace: item.workspace, path: item.path, title: 'Presenter' });
  }
  /* A show fills the page (Slides asks for the browser's full screen on
     its root); the native window goes full screen with it and comes back
     when the show ends, unless it was full screen already. */
  var showFullscreen = null;
  document.addEventListener('fullscreenchange', function () {
    var el = document.fullscreenElement, inShow = !!(el && el.closest && el.closest('.content-applet')), native = window.dinahNativeWindow;
    if (!native || !native.enter) return;
    if (inShow && !showFullscreen) showFullscreen = native.enter().catch(function () { return false; });
    else if (!inShow && showFullscreen) { var entered = showFullscreen; showFullscreen = null; entered.then(function (did) { if (did) return native.leave(); }).catch(function () {}); }
  });

  /* The stack each application's windows gather in on a full taskbar, one
     for each workspace. The Editor's and Docs's are both documents of text
     and share Documents, with the older document windows; diagrams and
     decks each have their own. Only the Editor's windows said which stack
     they were in before, so no other application's stacked (Paul, 12
     November 2026). */
  var GROUPS = {
    editor: { id: 'dinah.documents', title: 'Documents', glyph: 'document', workspace: true },
    docs: { id: 'dinah.documents', title: 'Documents', glyph: 'document', workspace: true },
    diagrams: { id: 'yavc.diagrams', title: 'Diagrams', glyph: 'diagram', workspace: true },
    slides: { id: 'yavc.slides', title: 'Slides', glyph: 'slides', workspace: true }
  };
  Object.keys(TITLES).forEach(function (app) {
    if (app !== 'editor') window.dinahUIState.followWindowMode(kindOf(app), 'editor');
    /* Each application is in the launcher's Documents category, and in Tools
       in the menu bar, and starts an empty, untitled document of its own kind
       in the current workspace, which Save As puts somewhere (Paul, 9 November
       2026). The Editor's is a text file. The taskbar's New Document is a
       Markdown document, so it is Docs's. */
    var launcher = { mnemonic: MNEMONICS[app], category: 'documents', description: DESCRIPTIONS[app], icon: ICONS[app],
      run: function () { return newDocument(undefined, '', app).catch(function (err) { host().toast(message(err), 'danger'); }); } };
    window.dinahWorkbench.register(kindOf(app), { pinByItem: true, title: TITLES[app], glyph: GLYPHS[app], icon: ICONS[app], start: 'idle', launcher: launcher,
      applet: GROUPS[app],
      init: function (root, opts, item) { return init(app, root, opts, item); } });
  });

  /* ---- Opening ------------------------------------------------------------ */

  /* The application a document opens in, and the view. Every Markdown
     document is Docs's and every other text file the Editor's (Paul, 12
     November 2026). Before that a Markdown document opened in either,
     by the view chosen and by how it was opened, and there was no rule a
     person could guess for which. Settings › Markdown Opens In now
     chooses the view of Docs only: Visual, Text, Split or Preview. D2,
     Mermaid and JSON Canvas files are Diagrams's and .slides.md decks are
     Slides's. A document opened to read (a chat's chip, a double-click in
     Sources) is shown in Docs's Preview. Edit as Text is the one way a
     Markdown document reaches the Editor from here, since it asks for
     the source by name; Docs's own Open in Editor is the other. */
  /* The view a Markdown document opens in, a choice in Settings
     (document_views.go); Visual unless chosen. */
  var documentViews = { 'text/markdown': 'page' };
  window.dinahDocumentViews = function (views) { if (views) documentViews = views; return documentViews; };

  /* The file name a document's kind is read from: its path, or for a new
     document its title when the title ends as a file name does (the
     Editor's "Untitled.txt", or a file from outside every workspace, which
     is titled with its own name). */
  function pathOf(v) {
    var o = v.origin || {};
    if (o.kind === 'new') return /\.[A-Za-z0-9]{1,8}$/.test(v.title || '') ? v.title : '';
    return o.path || '';
  }
  /* A new document is Markdown unless its title says otherwise, and so is a
     document of a source other than a folder whose path has no file name with an extension (a page on a
     site, which its source reads as Markdown), as the applets read it
     (nameOf in content-host.js). A file is Markdown by its extension. */
  function markdown(v) {
    var o = v.origin || {}, path = pathOf(v);
    if (o.kind === 'new' && !path) return true;
    if (o.kind && o.kind !== 'local-folder' && !/\.[A-Za-z0-9]{1,8}$/.test(path.split(/[\\/]/).pop())) return true;
    return /\.(md|markdown)$/i.test(path);
  }
  function placeFor(v, intent) {
    var path = pathOf(v);
    if (intent === 'text') return { app: 'editor', view: 'text' };
    if (/\.docs\.md$/i.test(path)) return { app: 'docs' };
    if (/\.(d2|mmd|mermaid|canvas)$/i.test(path)) return { app: 'diagrams' };
    if (/\.slides\.md$/i.test(path)) return { app: 'slides' };
    if (!markdown(v)) return { app: 'editor' };
    if (intent === 'read') return { app: 'docs', view: 'preview' };
    var chosen = window.dinahDocumentViews ? window.dinahDocumentViews()['text/markdown'] : 'page';
    if (chosen === 'text' || chosen === 'split' || chosen === 'preview') return { app: 'docs', view: chosen };
    return { app: 'docs' };
  }
  /* The application a document chosen in an application's own Open dialog
     opens in. It is the application that asked, except that the Editor
     does not take a Markdown document: one chosen there opens in Docs. */
  function appChosenIn(app, v) {
    return app === 'editor' && placeFor(v).app === 'docs' ? 'docs' : app;
  }

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
  function open(v, app, intent) {
    var place = app ? { app: app } : placeFor(v, intent);
    var item = itemFor(place.app, v);
    var already = window.pudlWindows.state().open.indexOf(window.dinahWorkbench.key(item)) >= 0;
    var elsewhere = window.pudlWindows.state().open.filter(function (key) {
      var other = itemOfWindow(key);
      return other && other.path === v.id && other.kind !== item.kind && contentKind(other.kind);
    });
    return Promise.all(elsewhere.map(closed)).then(function () {
      if (window.pudlWindows.state().open.some(function (key) { return elsewhere.indexOf(key) >= 0; })) {
        throw new Error('"' + (v.title || 'The document') + '" could not be saved in the window it was open in, so it stays there.');
      }
      return window.dinahWorkbench.open(item);
    }).then(function (key) {
      /* A window just opened shows the view chosen; one already open keeps
         the view the person left it in. */
      if (already || !place.view) return key;
      return window.dinahWorkbench.ready(key).then(function () {
        return window.dinahContentHost.accept(key, { operation: 'view', input: { mode: place.view } });
      }).then(function () { return key; }, function () { return key; });
    });
  }
  function openResource(ref, intent) { return Promise.resolve(api().OpenDraft(ref)).then(function (v) { return open(v, null, intent); }); }
  /* Open shows Dinah's file picker in the current workspace, as Dinah's
     Editor does; the field also takes a path on the computer, and a file
     outside every workspace's folders opens as a new document. */
  /* asked is what the application said of the dialog it wants: its title
     and the kinds of file it opens, as { title, accept } (the applets'
     fs.open). Without it the dialog is Open, for any document. */
  function openDocument(app, asked) {
    var currentWs = host() && host().workspaceId();
    function workspaceOf(choice) { return choice.dir && choice.dir.ref && choice.dir.ref.workspaceId || currentWs; }
    /* The dialog gives the keyboard back to the window it was asked from
       when it closes, which brings that window forward, over the document's
       own window opened a moment before. So the document's window is
       brought forward once the dialog has gone. */
    var opened = null;
    function shown(v) { return open(v, appChosenIn(app, v)).then(function (key) { opened = key; return key; }); }
    return window.dinahFilePicker.pick({ mode: 'open', title: asked && asked.title || 'Open', extensions: asked && asked.accept || [],
      commit: function (choice) {
        if (choice.node) return Promise.resolve(api().OpenDraft(choice.node.ref)).then(shown);
        return Promise.resolve(choice.reference ? api().OpenComputerFile(workspaceOf(choice), choice.reference) : api().OpenComputerDocument(workspaceOf(choice))).then(function (v) {
          if (!v || !v.id) return false;
          if (v.origin.kind === 'new') host().toast('"' + v.title + '" is not in the folder of any workspace, so it opened as a new document. Save As puts it where you choose.', 'warn');
          return shown(v);
        });
      } }).then(function (picked) {
        if (opened && window.pudlWindows.state().open.indexOf(opened) >= 0) window.pudlWindows.raise(opened);
        return picked;
      }).catch(function (err) { host().toast(message(err), 'danger'); });
  }

  /* The Editor's new document is a text file, "Untitled.txt": the Editor
     edits text and Markdown is Docs's, so a document started in the Editor
     is not Markdown, and Save As offers it as a text file (Paul, 12 November
     2026). Any other application's is "Untitled", in its own format. */
  function newDocument(workspaceId, text, app) {
    return Promise.resolve(api().NewEmptyDraft(workspaceId || (host() && host().workspaceId()) || '')).then(function (v) {
      return app === 'editor' ? api().RetitleDraft(v.id, v.revision, 'Untitled.txt') : v;
    }).then(function (v) {
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
    if (!key || !contentKind((itemOfWindow(key) || {}).kind)) return;
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
    if (!key) throw new Error('No ' + TITLES[app] + ' document is open in this workspace; open one with yavc.' + app + '/open-file@1. Where yavc.' + app + '/new@1 is among the actions offered, it starts a new one.');
    return key;
  }
  function firstDocument(key) { return listed(key); }
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
        /* An agent that asks the Editor for a Markdown file gets it in
           Docs, as a person choosing it in the Editor's Open dialog does. */
        return Promise.resolve(api().OpenDraft(workspaceRef(rec, input.path))).then(function (v) { return open(v, appChosenIn(app, v)); }).then(function (key) {
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
      /* Saving to a path is Save As of the window's document to a new
         file in the workspace; a file that is there is never replaced. */
      if (op === 'save' && input.path) {
        var saveKey = frontWindow(app, rec), target = workspaceRef(rec, input.path);
        return window.dinahWorkbench.ready(saveKey).then(function () {
          return textOf(saveKey);
        }).then(function (text) {
          var draftId = (itemOfWindow(saveKey) || {}).path;
          return Promise.resolve(api().GetDraft(draftId)).then(function (v) {
            return api().UpdateDraft(v.id, v.revision, text);
          }).then(function (v) { return api().SaveDraftAs(v.id, v.revision, target); });
        }).then(function (saved) {
          var again = remounts.get(saveKey);
          return (again ? again() : Promise.resolve()).then(function () { return firstDocument(saveKey); });
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

  /* Opening a document where a chat link points (chat-text.js): the
     window's applet goes to the line, where it has lines. */
  function goToLine(key, line) {
    if (!key || !line) return Promise.resolve();
    return window.dinahWorkbench.ready(key).then(function () {
      return window.dinahContentHost.accept(key, { operation: 'go-to', input: { line: line } });
    }).catch(function () { /* An application without lines opens the document at its start. */ });
  }

  window.dinahContentWindows = { listed: listed, open: open, openResource: openResource, openDocument: openDocument, newDocument: newDocument, route: route, kindOf: kindOf, openPresenter: openPresenter, decideSuggestion: decideSuggestion };

  /* The ways into the Editor (Documents, Files, Sources, chats, the
     Terminal's edit, In Progress) open documents here. */
  window.dinahEditor = {
    open: function (v, intent) { return open(v, null, intent); },
    openResource: function (ref, intent) { return openResource(ref, intent); },
    newDocument: function () { return newDocument().catch(function (err) { host().toast(message(err), 'danger'); }); },
    openDocument: function () { return openDocument(); },
    goToLine: goToLine
  };
})();
