/* Editor: Dinah's document editor. A window edits one draft, the
   working copy the draft service keeps of a workspace resource or of a new
   document, in Page, Text, Preview or Split view (Page view is
   page-view.js). What the person types is written
   to the draft as they type, so closing the window, or the application,
   loses nothing; Save writes the draft back to its resource, and Save As
   writes it somewhere new. Agents change the same drafts, and their changes
   arrive here as "draft:changed" events.

   The text surface is CodeMirror 6, loaded and themed by
   editor-codemirror.js; the key table is editor-keys.js. */
(function () {
  'use strict';

  // What the Editor's files share (editor-common.js), its key table
  // (editor-keys.js) and its text surface (editor-codemirror.js).
  var common = window.dinahEditorCommon;
  var api = common.api, productName = common.productName, host = common.host, message = common.message;
  var VIEWS = common.VIEWS, workspaceName = common.workspaceName, originRef = common.originRef, isMarkdown = common.isMarkdown;
  var EDITOR_KEYS = window.dinahEditorKeys.table, editorKeyRows = window.dinahEditorKeys.rows;
  var loadCodeMirror = window.dinahEditorCodeMirror.load, makeTheme = window.dinahEditorCodeMirror.theme;

  /* The view a Markdown document opens in when it has none of its own yet,
     a choice in Settings (document_views.go); Visual unless chosen. */
  var documentViews = { 'text/markdown': 'page' };
  window.dinahDocumentViews = function (views) { if (views) documentViews = views; return documentViews; };
  function openingView() { return VIEWS[documentViews['text/markdown']] ? documentViews['text/markdown'] : 'page'; }

  /* An Editor window's parts share one object, ed: the window's state that
     more than one part reads or writes, and the functions one part calls in
     another. Each is read and written through ed when the code runs, never
     copied, so a change one part makes is the one the others see. A name
     is installed once; a second state field or function with the same name
     is an error. */
  function installEditorPart(ed, members) {
    Object.keys(members).forEach(function (name) {
      if (name in ed) throw new Error('The Editor window has two members called ' + name + '.');
      ed[name] = members[name];
    });
  }
  /* Adds a part (editor-<name>.js) to the window at this point of init:
     builds it from ed, installs what it returns, and keeps its teardown for
     the window's dispose. */
  function usePart(ed, name) {
    var made = window.dinahEditorParts[name](ed), teardown = made.teardown;
    delete made.teardown;
    installEditorPart(ed, made);
    if (teardown) ed.off.push(teardown);
  }

  function init(root, opts, item) {
    /* What the parts of this window share (installEditorPart, above): its
       state and the functions one part calls in another. */
    var ed = Object.create(null);
    installEditorPart(ed, {
      root: root,
      opts: opts,
      win: undefined,
      key: undefined,
      id: undefined,
      CM: undefined,
      view: undefined,
      draft: undefined,
      disposed: undefined,
      allowClose: undefined,
      busy: undefined,
      discarding: undefined,
      pending: undefined,
      writing: undefined,
      writeTimer: undefined,
      outside: undefined,
      applying: undefined,
      problemsOpen: undefined,
      mode: undefined,
      wrapLines: undefined,
      wrapComp: undefined,
      views: undefined,
      pageView: undefined,
      fromPage: undefined,
      off: undefined,
      $: undefined,
      ready: undefined,
      say: say,
      isNew: isNew,
      unsaved: unsaved,
      drawHead: drawHead,
      scheduleContext: scheduleContext,
      closingWindow: closingWindow,
      appearanceChanged: appearanceChanged,
      cmd: cmd,
      commandsChanged: commandsChanged,
      activePage: activePage
    });
    ed.win = ed.root.closest('[data-win]'); ed.key = ed.opts.key || ed.win.dataset.win;
    ed.id = item.path; ed.CM = null; ed.view = null; ed.draft = null;
    ed.disposed = false; ed.allowClose = false; var closing = false; ed.busy = false; ed.discarding = false;
    ed.pending = false; ed.writing = null; ed.writeTimer = 0;
    ed.outside = null; ed.applying = false; ed.problemsOpen = false;
    ed.mode = VIEWS[ed.opts.state] ? ed.opts.state : requestedViews[item.path] || null; ed.wrapLines = true; ed.wrapComp = null;
    /* A host that embeds the editor may leave views out: the Skill window
       offers no Visual view, since a skill never becomes a page. */
    ed.views = Array.isArray(ed.opts.views) ? ed.opts.views.filter(function (v) { return VIEWS[v]; }) : Object.keys(VIEWS);
    if (!ed.views.length) ed.views = Object.keys(VIEWS);
    ed.pageView = null; ed.fromPage = false;
    ed.off = [];

    ed.root.classList.add('editor-applet');
    /* The window's two toolbars are PUDL's folding toolbars of the Editor's
       commands (commandTable, below): each control names its command, and
       PUDL draws its words, glyph, key, state and menu from it, so a
       button, its menu row, its key and what an agent sees are one
       command. A group keeps to one row and folds, first to its glyphs,
       then into a menu, as the window narrows.
       The Document group stays at its row's end. */
    function tbGroup(name, glyph, priority, inner, end) {
      return '<div class="toolbar-group" data-toolbar-group="' + name + '"' + (glyph ? ' data-toolbar-glyph="' + glyph + '"' : '') +
        ' data-toolbar-priority="' + priority + '"' + (end ? ' data-toolbar-end' : '') + '>' + inner + '</div>';
    }
    function tbControl(id, extra) { return '<button type="button" data-command="' + id + '"' + (extra || '') + '></button>'; }
    ed.root.innerHTML =
      '<div class="toolbar editor-head" role="toolbar" aria-label="Document" data-toolbar-fold data-ed-head>' +
        // The document's commands stay at the end and fold last.
        tbGroup('View', null, 3, '<div class="seg sm" data-command-choice="view" aria-label="View"></div>' +
          '<button type="button" class="btn btn-sm doc-problems-btn" data-ed-problems-btn aria-expanded="false" hidden></button>') +
        tbGroup('Document', 'document', 4,
          tbControl('save-as', ' data-ed-save-as') + tbControl('save', ' class="btn btn-sm btn-primary" data-ed-save'), true) +
      '</div>' +
      // The Formatting toolbar gives way to the ribbon, whose Format and
      // Insert tabs hold the same commands (PUDL 0.80.0).
      '<div class="toolbar editor-toolbar" role="toolbar" aria-label="Formatting" data-toolbar-fold data-toolbar-ribbon="hide" data-ed-toolbar>' +
        // Text folds last, as the most used.
        tbGroup('Style', 'paragraph', 4, tbControl('paragraph-style', ' class="btn btn-sm editor-toolbar-style"')) +
        tbGroup('Text', 'bold', 5, tbControl('bold') + tbControl('italic') + tbControl('underline') + tbControl('strike') + tbControl('code')) +
        tbGroup('Lists', 'list', 3, tbControl('bullets') + tbControl('numbers') + tbControl('insert-checklist') + tbControl('quote')) +
        tbGroup('Insert', 'panel', 2, tbControl('link') + tbControl('insert-image') + tbControl('insert-file') + tbControl('insert-code')) +
        tbGroup('Table', 'table', 1, tbControl('table-menu')) +
      '</div>' +
      '<div class="editor-find" data-ed-find role="search" aria-label="Find and Replace" hidden>' +
        '<div class="editor-find-row">' +
          '<input class="form-input" data-ed-find-input placeholder="Find" aria-label="Find" autocomplete="off" spellcheck="false">' +
          '<span class="editor-find-count" data-ed-find-count role="status"></span>' +
          '<button type="button" class="icon-btn" data-ed-find-previous aria-label="Previous Match" title="Previous Match (Shift+F3)"><span class="glyph" style="--glyph: var(--glyph-arrow-up)" aria-hidden="true"></span></button>' +
          '<button type="button" class="icon-btn" data-ed-find-next aria-label="Next Match" title="Next Match (F3)"><span class="glyph" style="--glyph: var(--glyph-arrow-down)" aria-hidden="true"></span></button>' +
          '<button type="button" class="btn btn-sm" data-ed-find-option="matchCase" aria-pressed="false" title="Match Case">Aa</button>' +
          '<button type="button" class="btn btn-sm" data-ed-find-option="wholeWord" aria-pressed="false" title="Whole Word">Word</button>' +
          '<button type="button" class="btn btn-sm" data-ed-find-option="regexp" aria-pressed="false" title="Regular Expression">.*</button>' +
          '<button type="button" class="btn btn-sm" data-ed-find-show-replace aria-pressed="false" title="Replace (Ctrl+H)">Replace</button>' +
          '<button type="button" class="icon-btn" data-ed-find-close aria-label="Close Find" title="Close (Esc)"><span class="glyph" style="--glyph: var(--glyph-close)" aria-hidden="true"></span></button>' +
        '</div>' +
        '<div class="editor-find-row" data-ed-find-replace-row hidden>' +
          '<input class="form-input" data-ed-replace-input placeholder="Replace with" aria-label="Replace With" autocomplete="off" spellcheck="false">' +
          '<button type="button" class="btn btn-sm" data-ed-replace-one title="Replace this match and go to the next (Enter)">Replace</button>' +
          '<button type="button" class="btn btn-sm" data-ed-replace-all>Replace All</button>' +
        '</div>' +
      '</div>' +
      '<p class="doc-meta" data-ed-meta></p>' +
      '<div class="editor-body">' +
      '<div class="editor-proposals" data-ed-proposals hidden></div>' +
      '<div class="notice warn editor-notice" data-ed-notice hidden><div class="notice-content"><p class="notice-body" data-ed-notice-body></p>' +
        '<div class="notice-actions"><button type="button" class="btn btn-sm" data-ed-theirs>Take Theirs</button>' +
        '<button type="button" class="btn btn-sm btn-primary" data-ed-mine>Keep Mine</button></div></div></div>' +
      '<div class="notice warn editor-notice" data-ed-moved hidden><div class="notice-content"><p class="notice-body" data-ed-moved-body></p>' +
        '<div class="notice-actions"><button type="button" class="btn btn-sm" data-ed-compare>Compare…</button>' +
        '<button type="button" class="btn btn-sm btn-primary" data-ed-bring>Bring In Their Changes</button></div></div></div>' +
      '<ol class="doc-problems" data-ed-problems hidden></ol>' +
      '<div class="editor-panes" data-ed-panes>' +
        '<div class="split editor-split" data-ed-split>' +
          '<div class="split-pane editor-text" data-ed-text></div>' +
          '<div class="split-handle" data-ed-split-handle aria-label="Resize the text and the page" data-split-min="200"></div>' +
          '<div class="split-pane doc-scroll editor-preview" data-ed-scroll tabindex="-1"><article class="doc-page" data-ed-page aria-label="Preview of the draft"></article></div>' +
        '</div>' +
      '</div>' +
      '</div>' +
      '<p class="editor-status" data-ed-status role="status"></p>';
    ed.$ = function (name) { return ed.root.querySelector('[data-ed-' + name + ']'); };
    ed.$('problems').id = ed.key + '-problems';
    // The preview scrolls by keyboard, and a key typed in it switches to Split.
    ed.$('scroll').tabIndex = 0;
    ed.$('problems-btn').setAttribute('aria-controls', ed.$('problems').id);

    /* The status line under the text. An error said there is also a toast
       and a card under Notifications, which reopens the document, since the
       line is written over as the person moves the caret. */
    function say(text, warn, title) {
      ed.$('status').textContent = text || '';
      ed.$('status').classList.toggle('warn', !!warn);
      if (warn && text && host().documentFailed) host().documentFailed(ed.id, title || '"' + (ed.draft && ed.draft.title || 'Untitled') + '":', text);
    }
    function has(action) { return !!ed.draft && (ed.draft.actions || []).indexOf(action) >= 0; }
    function isNew() { return !!ed.draft && ed.draft.origin.kind === 'new'; }
    function unsaved() { return !!ed.draft && (ed.draft.changed || ed.pending); }

    /* ---- Head ---------------------------------------------------------- */

    function describe() {
      if (!ed.draft) return '';
      var ref = originRef(ed.draft), parts = [];
      if (ref) parts.push(window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.describe(ref) : ref.sourceId);
      else parts.push('New document, not saved anywhere yet');
      if (ref && ed.draft.origin.kind === 'local-folder') parts.push(ed.draft.origin.path);
      parts.push(ed.outside ? 'changed elsewhere' : ed.pending ? 'keeping your changes…' : unsaved() ? 'changes not saved' : 'no changes');
      return parts.join(' · ');
    }
    function drawHead() {
      if (!ed.draft || ed.disposed) return;
      var title = (ed.draft.title || 'Untitled') + (unsaved() ? ' (changed)' : '');
      // The workspace the document belongs to is the title's detail,
      // which PUDL draws after
      // the title and shortens first, so the taskbar, the dock tabs and the
      // Ctrl+Tab list say it too.
      if (!ed.opts.embedded) window.pudlWindows.retitle(ed.key, title, workspaceName(ed.draft.origin && ed.draft.origin.workspaceId) || '');
      // Save, Save As… and the rest are the command table's, drawn again
      // by commandsChanged below.
      ed.$('meta').textContent = describe();
      ed.drawProposals();
      ed.syncMarks();
      ed.pageMarks();
      commandsChanged();
      scheduleContext();
    }
    document.addEventListener('dinah:workspace-resource-labels', drawHead);

    usePart(ed, 'views');

    usePart(ed, 'draft');

    usePart(ed, 'saving');

    usePart(ed, 'preview');

    usePart(ed, 'suggestions');

    /* ---- The chat's context ----------------------------------------------
       While this window is in front, a chat works with its document: the
       draft's ID and the selected passage, widened to paragraphs, so the person can ask a chat to change it and the
       change arrives here as suggestions (documents-first.md, "One Surface
       for Documents"). */
    var contextTimer = 0, contextSent = '';
    /* raised says the window has just come to the front, which gives the
       chat its document again even after the person cleared the chip. */
    function context(raised) {
      if (ed.disposed || !ed.draft || ed.opts.embedded) return;
      var state = window.pudlWindows && window.pudlWindows.state();
      if (!state || state.top !== ed.key) return;
      var detail = { kind: '', path: ed.draft.origin.path || '', title: ed.draft.title, draftId: ed.id, selection: ed.selectedPassage() };
      var sent = JSON.stringify(detail);
      if (sent === contextSent) return;
      contextSent = sent;
      ed.root.dispatchEvent(new CustomEvent('dinah:item-context', { bubbles: true, detail: Object.assign({ raised: !!raised }, detail) }));
    }
    function scheduleContext() { clearTimeout(contextTimer); contextTimer = setTimeout(function () { context(false); }, 250); }
    function windowsChanged(e) { if (e.detail && e.detail.top === ed.key) { contextSent = ''; context(true); } }
    document.addEventListener('pudl:windows-change', windowsChanged);
    ed.off.push(function () { clearTimeout(contextTimer); document.removeEventListener('pudl:windows-change', windowsChanged); });

    usePart(ed, 'text');

    usePart(ed, 'find');

    usePart(ed, 'format');

    usePart(ed, 'clipboard');

    /* ---- Window -------------------------------------------------------- */

    /* The draft keeps everything typed, so closing never asks; it only
       waits for the last change to be written. */
    function closingWindow(e) {
      if (e.target !== ed.win || ed.allowClose) return;
      if (!ed.pending && !ed.writing) return;
      e.preventDefault();
      if (closing) return;
      closing = true;
      ed.flush().finally(function () {
        closing = false;
        if (!ed.outside && !ed.disposed) { ed.allowClose = true; window.pudlWindows.close(ed.key); }
      });
    }
    function appearanceChanged() { if (!ed.disposed && ed.view) dinahRender.enqueue(appearanceChanged, ed.root, function () { if (!ed.disposed && ed.view) ed.view.requestMeasure(); }, null, 'editor appearance'); }
    document.addEventListener('dinah:appearance-change', appearanceChanged);
    ed.win.addEventListener('pudl:window-closing', closingWindow);

    ed.ready = Promise.all([loadCodeMirror(), Promise.resolve().then(function () { return api().GetDraft(ed.id); })]).then(function (both) {
      if (ed.disposed) return;
      ed.CM = both[0];
      ed.draft = both[1];
      ed.makeView(ed.draft.text);
      // Without the Visual view, Split is the text beside the rendered
      // preview, and no page is built.
      if (window.dinahPageView && ed.views.indexOf('page') >= 0) ed.pageView = ed.attachPage();
      ed.setMode(isMarkdown(ed.draft) ? ed.mode || openingView() : ed.reading() ? 'text' : ed.mode || 'text', true);
      drawHead();
      ed.position();
      if (!ed.opts.embedded) openEditors.set(ed.key, { origin: function () { return ed.draft && ed.draft.origin; }, flush: ed.flush });
      // A draft with changes says at once when its file moved on.
      if (ed.draft.changed) ed.checkMoved(false);
    }).catch(function (err) {
      ed.$('meta').textContent = 'The draft could not be opened: ' + message(err);
      ed.openFailed = 'The draft could not be opened.';
      commandsChanged();
    });

    /* A window that is opened, or opened again, comes to the front with the
       cursor where the document is edited, so typing starts at once: the
       page in Visual, the text in Text and Split, and in Preview the page,
       where typing a letter goes on in the text. It waits for the document
       to load and for the dialog that opened it, such as Open…, to close and
       hand focus back. */
    function focusEditing() {
      if (ed.disposed) return;
      if (!ed.opts.embedded) window.pudlWindows.raise(ed.key);
      if (ed.mode === 'page' && ed.pageView && !ed.pageOff) ed.pageView.focus();
      else if (ed.mode === 'preview') ed.$('scroll').focus();
      else if (ed.view) ed.view.focus();
    }
    ed.root.addEventListener('dinah:editor-focus', function () {
      ed.ready.then(function () { return ed.pageEnabled; }).then(function () { setTimeout(focusEditing, 0); });
    });
    /* A link in a chat to a line, such as plan.md:42, opens the document
       and goes to the line once the text is there, which shows the text
       beside the page when the view was one for reading. */
    ed.root.addEventListener('dinah:editor-line', function (e) {
      var line = e.detail && +e.detail.line;
      if (line > 0) ed.ready.then(function () { return ed.pageEnabled; }).then(function () { setTimeout(function () { if (!ed.disposed) ed.goToLine(line); }, 0); });
    });

    function cmd(label, run, disabled, extra) { return Object.assign({ label: label, run: run, disabled: !!disabled }, extra || {}); }
    /* Something a command's state depends on has changed: the menus and
       the toolbars follow. */
    function commandsChanged() {
      if (window.dinahCommands) window.dinahCommands.changed();
      if (window.pudlCommands) window.pudlCommands.changed();
    }
    // The page the formatting acts on: in Visual, while it is built.
    function activePage() { return ed.mode === 'page' && ed.pageView && ed.pageView.active() ? ed.pageView : null; }

    usePart(ed, 'commands');

    /* What the Documents menu asks of this window when its draft is the
       current document (documents.js). */
    ed.documentController = {
      key: ed.key,
      searchRows: ed.searchRows,
      ref: function () { return originRef(ed.draft); },
      saveLabel: function () { return !ed.draft ? 'Save' : ed.isNew() ? 'Save As…' : 'Save'; },
      canSave: function () { return !!ed.draft && !ed.busy && !ed.outside && (ed.isNew() || (!!ed.draft.writeBack && ed.unsaved())); },
      /* Why a command cannot run now, for command search: the first thing
         in the way, in the order the command checks them. */
      whyNot: function (command) {
        if (!ed.draft) return 'The document is still opening.';
        if (ed.busy) return 'The Editor is busy with the document.';
        if (ed.outside) return 'The document was changed elsewhere in ' + productName() + '; settle that change first.';
        if (command === 'save' && !ed.isNew() && !ed.draft.writeBack) return 'The document has nowhere to be saved back to; use Save As….';
        if (command === 'save' && !ed.isNew() && !ed.unsaved()) return 'There are no changes to save.';
        if (command === 'rename' && !ed.canRename()) return 'Only a file in a folder can be renamed from ' + productName() + '.';
        return '';
      },
      canRename: ed.canRename,
      rename: ed.rename,
      save: ed.save,
      canSaveAs: function () { return !!ed.draft && !ed.busy && !ed.outside; },
      saveAs: ed.saveAs,
      close: function () { window.pudlWindows.close(ed.key); },
      discard: ed.discard
    };
    if (!ed.opts.embedded && window.dinahDocuments) {
      window.dinahDocuments.attach(ed.key, ed.documentController);
      ed.off.push(function () { window.dinahDocuments.detach(ed.key); });
    }
    // The Editor's menus after Document, in the order of the menu bar.
    // Each menu's id, title, letter and glyph; a title folds to its glyph
    // before the menu bar collapses (PUDL 0.79.0).
    var OWN_MENUS = [['edit', 'Edit', 'E', 'pencil'], ['insert', 'Insert', 'S', 'plus'], ['format', 'Format', 'F', 'paragraph'], ['review', 'Review', 'R', 'tick']];
    function viewItems() {
      return Object.keys(VIEWS).map(function (v) { return ed.cmd(VIEWS[v], function () { ed.setMode(v); }, false, { checked: ed.mode === v, radio: 'editor-view' }); })
        .concat(ed.menuRows('view'));
    }

    return {
      ready: ed.ready,
      state: function () { return ed.mode; },
      setState: function (s) { if (VIEWS[s]) ed.setMode(s, true); },
      commands: function () {
        return OWN_MENUS.reduce(function (all, m) { return all.concat(ed.menuRows(m[0])); }, []).concat(viewItems()).filter(function (c) { return c !== '-'; });
      },
      /* The first title is the applet's own, as PUDL's menu bar expects;
         View belongs to the host, so the views are added to it. What acts
         on this document as a whole (save, close, discard) is the window's
         own Document menu, so it is plain which
         document a command acts on, and its keys act on this window only;
         the host's Documents keeps New Document, Open and Recent. */
      menus: function () {
        // The window menu's rows, as every window's own menu opens with.
        var rows = window.dinahWorkbench.windowMenuRows(ed.key);
        // A document of the open workspace can be pinned to it, first.
        var o = ed.draft && ed.draft.origin, pin = o && o.kind !== 'new' && window.dinahPins && window.dinahHost && o.workspaceId === window.dinahHost.workspaceId() &&
          window.dinahPins.command({ kind: 'resource', title: ed.draft.title || 'Untitled', ref: { workspaceId: o.workspaceId, sourceId: o.sourceId, path: o.path }, itemKind: 'file' });
        if (pin) rows = [pin, '-'].concat(rows);
        return {
          titles: [
            { label: 'Editor', mnemonic: 'I', glyph: 'document', items: rows },
            { id: 'document', label: 'Document', mnemonic: 'O', glyph: 'file', items: window.dinahCommands.rows('document') }
          ].concat(OWN_MENUS.map(function (m) { return { id: m[0], label: m[1], mnemonic: m[2], glyph: m[3], items: ed.menuRows(m[0]) }; })),
          into: { view: viewItems() }
        };
      },
      destroy: function () {
        ed.disposed = true;
        document.removeEventListener('dinah:appearance-change', ed.appearanceChanged); dinahRender.cancel(ed.appearanceChanged);
        openEditors.delete(ed.key);
        clearTimeout(ed.writeTimer);
        if (ed.pending && ed.draft && ed.view) api().UpdateDraft(ed.id, ed.draft.revision, ed.view.state.doc.toString()).catch(function () {});
        ed.off.forEach(function (fn) { fn(); });
        ed.win.removeEventListener('pudl:window-closing', ed.closingWindow);
        document.removeEventListener('dinah:workspace-resource-labels', ed.drawHead);
        if (ed.pageView) ed.pageView.destroy();
        if (ed.view) ed.view.destroy();
      }
    };
  }

  /* ---- Opening ----------------------------------------------------------- */

  /* A document opened to read (a chat's chip, Find and Open, a double-click
     in Sources) opens in Preview; one opened to work on (Open…, Recent, New
     Document…, Edit) opens in Split, and an open window in Preview switches
     to Split. The intent is kept here for the window init reads, since the
     window's address stays the draft alone. */
  var requestedViews = {};
  function itemFor(v) { return { kind: 'editor', workspace: v.origin.workspaceId || '', path: v.id, title: v.title }; }
  function open(v, intent) {
    var item = itemFor(v), key = window.dinahWorkbench.key(item);
    var already = window.pudlWindows.state().open.indexOf(key) >= 0;
    if (!already && intent === 'read') requestedViews[v.id] = 'preview';
    return window.dinahWorkbench.open(item).then(function (opened) {
      delete requestedViews[v.id];
      var win = document.querySelector('[data-win="' + CSS.escape(opened) + '"] .editor-applet');
      if (already && intent !== 'read' && win) win.dispatchEvent(new CustomEvent('dinah:editor-work'));
      if (win) win.dispatchEvent(new CustomEvent('dinah:editor-focus'));
      return opened;
    });
  }
  /* Opening a document refreshes a draft that holds nothing of the
     person's, so the typing a window open on it has not yet written is
     written first; the draft then counts as changed and is left alone. */
  var openEditors = new Map();
  function openResource(ref, intent) {
    var waits = [];
    openEditors.forEach(function (e) {
      var o = e.origin();
      if (o && o.workspaceId === ref.workspaceId && o.sourceId === ref.sourceId && (o.path || '') === (ref.path || '')) waits.push(e.flush().catch(function () {}));
    });
    return Promise.all(waits).then(function () { return api().OpenDraft(ref); }).then(function (v) { return open(v, intent); });
  }

  /* Open shows Dinah's file system, starting in the current
     workspace. The field also takes a path on the computer, and From Your
     Computer opens the computer's own file dialog. A file outside every
     workspace's folders opens as a new document, which Save As puts where
     the person chooses. */
  function openDocument() {
    var current = host().workspaceId();
    function workspaceOf(choice) { return choice.dir && choice.dir.ref && choice.dir.ref.workspaceId || current; }
    return dinahFilePicker.pick({ mode: 'open', title: 'Open',
      commit: function (choice) {
        if (choice.node) return openResource(choice.node.ref);
        return Promise.resolve(choice.reference ? api().OpenComputerFile(workspaceOf(choice), choice.reference) : api().OpenComputerDocument(workspaceOf(choice))).then(function (v) {
          if (!v || !v.id) return false;
          if (v.origin.kind === 'new') host().toast('"' + v.title + '" is not in a workspace\'s folder, so it opened as a new document. Save As puts it where you choose.', 'warn');
          return open(v);
        });
      } }).catch(function (err) { host().toast(message(err), 'danger'); });
  }

  /* Drafts with work in them (changes not yet saved, or new documents) are
     offered in Find and Open. The list is kept current from draft events,
     since Find and Open draws as the person types. */
  var inProgress = [];
  function refreshDrafts() {
    Promise.resolve().then(function () { return api().ListDrafts(); }).then(function (list) {
      inProgress = (list || []).filter(function (d) { return d.changed || d.origin.kind === 'new'; });
    }).catch(function () {});
  }
  document.addEventListener('DOMContentLoaded', function () {
    if (!host() || !api()) return;
    refreshDrafts();
    host().on('draft:changed', refreshDrafts);
    host().findSource(function () {
      var records = window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.records() : [];
      var openKeys = window.pudlWindows ? window.pudlWindows.state().open : [];
      /* A draft whose window is open is already listed as that window. */
      return inProgress.filter(function (d) { return openKeys.indexOf(window.dinahWorkbench.key(itemFor(d))) < 0; }).map(function (d) {
        var owner = records.find(function (r) { return r.id === d.origin.workspaceId; });
        return { title: d.title || 'Untitled', glyph: 'document',
          detail: 'Draft · ' + (d.origin.kind === 'new' ? 'new, not saved' : 'not saved') + (owner ? ' · ' + owner.name : ''),
          open: function () { return open(d); } };
      });
    });
  });

  /* New Document and Open… are Documents commands (documents.js); New
     Document and Tools › Editor start an empty, untitled document in the
     current workspace, for notes or a script begun from nothing, which Save
     As… puts somewhere. */
  function newEmpty() {
    return Promise.resolve().then(function () { return api().NewEmptyDraft(host().workspaceId() || ''); }).then(function (v) { return open(v); })
      .catch(function (err) { host().toast(message(err), 'danger'); });
  }
  window.dinahEditor = { open: open, openResource: openResource, newDocument: newEmpty, openDocument: openDocument };
  // An Editor restored behind other windows starts when it is seen, or in
  // idle time after the windows in sight.
  window.dinahWorkbench.register('editor', { applet: { id: 'dinah.documents', title: 'Documents', glyph: 'document', workspace: true }, title: 'Editor', glyph: 'document', start: 'idle', launcher: { mnemonic: 'E', description: 'Start an empty document, for notes or a script.', run: newEmpty }, init: init });
})();
