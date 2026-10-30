/* Documents: the host menu that holds every document command, the current
   document they act on, and Recent. Each Editor window gives this module a controller for its draft; the
   rows are declared once in the command registry (commands.js), which draws
   them and binds their keys. */
(function () {
  'use strict';

  const api = window.dinahHelpers.app;
  function host() { return window.dinahHost; }
  function editor() { return window.dinahEditor; }
  var C = window.dinahCommands;

  /* ---- The current document --------------------------------------------

     Save and Save As… act on the Editor in front, and when a chat or the
     Workspace tree is in front, on the Editor most recently in front, so Ctrl+S
     after clicking into the tree still saves the document the person was
     working on. Close Document and Discard Draft… act on the front window
     only, so a key never closes a window the person is not looking at. */
  var controllers = new Map(), recency = [];
  function frontKey() {
    var s = window.pudlWindows && window.pudlWindows.state();
    return s && s.top && !s.min[s.top] ? s.top : null;
  }
  function frontKind() {
    var key = frontKey(), win = key && document.querySelector('[data-win="' + CSS.escape(key) + '"]');
    return win ? win.dataset.winKind || '' : '';
  }
  function front() { var key = frontKey(); return key ? controllers.get(key) || null : null; }
  /* The windows a person clicks into while working on a document, which
     leave it the current one. Any other window in front, such as a Terminal
     or Files, has no current document, so a key pressed there never acts on
     a document behind it. */
  var BESIDE = ['chat', 'chat-session', 'workspace'];
  function current() {
    var f = front();
    if (f) return f;
    if (frontKey() && BESIDE.indexOf(frontKind()) < 0) return null;
    var key = recency.find(function (k) { return controllers.has(k); });
    return key ? controllers.get(key) : null;
  }
  function noteFront() {
    var key = frontKey();
    if (key && controllers.has(key)) recency = [key].concat(recency.filter(function (k) { return k !== key; }));
    C.changed();
  }
  document.addEventListener('pudl:windows-change', noteFront);
  // The Editor in front gives its Edit and View commands to command search.
  C.addSearch(function () { var f = front(); return f && f.searchRows ? f.searchRows() : []; });

  window.dinahDocuments = {
    attach: function (key, controller) { controllers.set(key, controller); noteFront(); },
    detach: function (key) { controllers.delete(key); recency = recency.filter(function (k) { return k !== key; }); C.changed(); },
    current: current,
    front: front,
    /* The documents open in Editor windows, the most recently in front
       first, as { draftId, title, workspaceId }, for the chat's @. */
    open: function () {
      var keys = recency.filter(function (k) { return controllers.has(k); });
      controllers.forEach(function (c, k) { if (keys.indexOf(k) < 0) keys.push(k); });
      return keys.map(function (k) {
        var item = window.dinahWorkbench.item(k), win = document.querySelector('[data-win="' + CSS.escape(k) + '"] .win-title');
        var title = (item && item.title) || (win ? win.textContent.replace(/ \(changed\)$/, '').trim() : '');
        return item && item.path ? { draftId: item.path, title: title, workspaceId: item.workspace } : null;
      }).filter(Boolean);
    },
    /* Recent shows each workspace's name as it is now, so a rename reads
       it again. */
    reloadRecent: function () { return load(); }
  };

  /* A key a Terminal's own line editing uses is left to the Terminal while
     one is in front, as a rule of this application's, not a reliance on
     what the terminal does with the key. */
  function suspended(kinds) { return kinds.indexOf(frontKind()) >= 0; }
  function cur(fn, fallback) { var c = current(); return c ? fn(c) : fallback; }

  /* ---- Rows -------------------------------------------------------------- */

  /* Why a document command cannot run now, for command search. */
  var NO_DOCUMENT = 'No document is open.';
  /* The current document's own reason, which knows what is in the way. */
  function whyNot(command, fallback) {
    return function () { return cur(function (c) { return (c.whyNot && c.whyNot(command)) || fallback; }, NO_DOCUMENT); };
  }

  C.register({ id: 'new-document', glyph: 'plus', menu: 'documents', mnemonic: 'N', label: 'New Document', reason: 'Open a workspace first.', enabled: function () { return !!(host() && host().workspaceId()); }, run: function () { return editor().newDocument(); } });
  C.register({ id: 'open-document', glyph: 'open', menu: 'documents', mnemonic: 'O', label: 'Open…', shortcut: 'Ctrl+O', reason: 'A Terminal is in front, and the key is its own.', enabled: function () { return !suspended(['terminal']); }, run: function () { return editor().openDocument(); } });
  C.register({ id: 'documents-sep-file', menu: 'document', separator: true });
  C.register({ id: 'save-document', glyph: 'download', menu: 'document', mnemonic: 'S', shortcut: 'Ctrl+S', help: { label: 'Save or Save As…', key: 'Ctrl+S' },
    label: function () { return cur(function (c) { return c.saveLabel(); }, 'Save'); },
    enabled: function () { return !suspended(['terminal']) && cur(function (c) { return c.canSave(); }, false); },
    reason: function () { return suspended(['terminal']) ? 'A Terminal is in front, and the key is its own.' : whyNot('save', 'There are no changes to save.')(); },
    run: function () { return cur(function (c) { return c.save(); }); } });
  /* PUDL keeps one of two rows with one label, so this row stands aside
     while the row above reads Save As…. */
  C.register({ id: 'save-document-as', glyph: 'copy', menu: 'document', mnemonic: 'V', label: 'Save As…', shortcut: 'Ctrl+Shift+S',
    visible: function () { return cur(function (c) { return c.saveLabel() !== 'Save As…'; }, true); },
    enabled: function () { return cur(function (c) { return c.canSaveAs(); }, false); }, reason: whyNot('save-as', 'The Editor is busy with the document.'),
    run: function () { return cur(function (c) { return c.saveAs(); }); } });
  /* F2 is the Editor window's own key, as it is a chat window's for Rename
     Chat…; a row here with the key would take it from every window, since
     PUDL stops at the first row that matches a chord, even a disabled one. */
  C.register({ id: 'rename-document', menu: 'document', mnemonic: 'M', label: 'Rename…', help: { label: 'Rename the Document…', key: 'F2' },
    enabled: function () { return cur(function (c) { return c.canRename(); }, false); },
    reason: whyNot('rename', 'The Editor is busy with the document.'),
    run: function () { return cur(function (c) { return c.rename(); }); } });
  C.register({ id: 'documents-sep-close', menu: 'document', separator: true });
  /* No key of its own: PUDL stops at the first row that matches a chord,
     even a disabled one, and Window › Close has Ctrl+F4 for every window. */
  C.register({ id: 'close-document', glyph: 'close', menu: 'document', mnemonic: 'C', label: 'Close Document',
    enabled: function () { return !!front(); }, reason: 'No Editor window is in front.',
    run: function () { var f = front(); if (f) return f.close(); } });
  C.register({ id: 'discard-draft', menu: 'document', mnemonic: 'D', label: 'Discard Draft…', danger: true,
    enabled: function () { return !!front(); }, reason: 'No Editor window is in front.',
    run: function () { var f = front(); if (f) return f.discard(); } });
  C.register({ id: 'documents-sep-recent', menu: 'documents', separator: true });
  C.register({ id: 'recent-documents', glyph: 'list', menu: 'documents', mnemonic: 'R', label: 'Recent', items: function () { return recentRows(); } });
  C.register({ id: 'in-progress', glyph: 'pencil', menu: 'documents', mnemonic: 'I', label: 'In Progress', run: function () { return window.dinahWorkbench.open('in-progress'); } });

  /* ---- Work in progress ----------------------------------------------------
     The words and the Discard question the In Progress pane uses for a
     document. A draft is a new document never
     saved anywhere; a changed document is one in a source with changes or
     suggestions not yet saved. The list the app gives has only
     these, never a working copy with nothing unsaved in it. */
  function isDraft(d) { return d.origin.kind === 'new'; }
  function draftState(d) {
    var n = (d.suggestions || []).length;
    var parts = [];
    if (isDraft(d)) parts.push('Not saved anywhere yet');
    else if (d.changed) parts.push('Changes not saved');
    if (n) parts.push(n === 1 ? '1 suggestion to decide' : n + ' suggestions to decide');
    var at = when(d.updated);
    return parts.join('; ') + (at ? ' · ' + at : '');
  }
  function draftPlace(d) {
    if (d.origin.kind === 'new') {
      var rec = window.dinahWorkspaceBrowser && window.dinahWorkspaceBrowser.records().find(function (r) { return r.id === d.origin.workspaceId; });
      return rec ? rec.name : '';
    }
    var ref = { workspaceId: d.origin.workspaceId, sourceId: d.origin.sourceId, path: d.origin.path };
    var source = window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.describe(ref) : '';
    // A file is told apart by its path, as the Editor's description line does.
    return d.origin.kind === 'local-folder' ? (source ? source + ' · ' : '') + d.origin.path : source || d.origin.path;
  }
  /* Discard throws a draft, or a changed document's changes, away after
     asking; the file is left as it is. It resolves true when the
     work was discarded. */
  function discardWork(d) {
    var title = '"' + (d.title || 'Untitled') + '"';
    var question = isDraft(d)
      ? ['Discard This Draft?', title + ' is thrown away. It was never saved anywhere, so it cannot be had back.', 'Discard Draft']
      : ['Discard These Changes?', 'The changes to ' + title + ' that are not saved are thrown away. The document itself is not changed.', 'Discard Changes'];
    return host().ask(question[0], question[1], question[2], true).then(function (yes) {
      if (!yes) return false;
      return Promise.resolve(api().DiscardDraft(d.id, d.revision)).then(function () { return true; });
    }).catch(function (err) { host().toast(title + ' could not be discarded: ' + String(err && err.message || err).replace(/^Error: /, ''), 'danger'); return false; });
  }
  window.dinahDocuments.work = { isDraft: isDraft, state: draftState, place: draftPlace, discard: discardWork, when: when };

  /* ---- Recent ------------------------------------------------------------

     The list is kept by Go in recent-documents.json, which the command line
     writes too, so it is read at start, when it changes here, and when the
     window comes back into view. */
  var MENU_ROWS = 10, recent = [], loading = null, again = false, shownError = '';
  /* A change told while a read is under way may have been written after
     that read began, so it is read once more when the read ends. */
  function load() {
    if (!api() || !api().RecentDocuments) return Promise.resolve();
    if (loading) { again = true; return loading; }
    loading = Promise.resolve().then(function () { return api().RecentDocuments(); }).then(function (list) {
      recent = Array.isArray(list) ? list : [];
      shownError = '';
      C.changed();
    }).catch(function (err) {
      /* The menu keeps the list it has, and the reason is shown once. */
      var text = 'Recent documents could not be read: ' + String(err && err.message || err).replace(/^Error: /, '');
      if (text !== shownError && host()) host().toast(text, 'danger');
      shownError = text;
    }).finally(function () {
      loading = null;
      if (again) { again = false; load(); }
    });
    return loading;
  }
  function sameRef(a, b) { return !!a && !!b && a.workspaceId === b.workspaceId && a.sourceId === b.sourceId && a.path === b.path; }
  function isCurrent(entry) {
    var c = current(), ref = c && c.ref();
    return !!entry.ref && sameRef(entry.ref, ref);
  }
  function openEntry(entry) {
    var opened = entry.ref ? editor().openResource(entry.ref)
      : Promise.resolve().then(function () { return api().OpenComputerFile(entry.workspaceId || '', entry.file); }).then(function (v) { return editor().open(v); });
    return Promise.resolve(opened).catch(function (err) { host().toast('"' + entry.title + '" could not be opened: ' + String(err && err.message || err).replace(/^Error: /, ''), 'danger'); });
  }
  /* The first nine rows are numbered, so that the digit is the row's
     letter, as the Window menu's open windows are. Two documents of one
     name are told apart by their workspace on the row's second line, as the
     window titles tell them apart. */
  function recentRows() {
    if (!recent.length) return [{ label: 'None yet', disabled: true }];
    var shown = recent.slice(0, MENU_ROWS), counts = new Map();
    shown.forEach(function (e) { counts.set(e.title, (counts.get(e.title) || 0) + 1); });
    return shown.map(function (e, i) {
      return { label: (i < 9 ? (i + 1) + '. ' : '') + e.title, detail: counts.get(e.title) > 1 ? e.workspaceName || '' : '',
        checked: isCurrent(e), run: function () { return openEntry(e); } };
    }).concat(['-', { label: 'Show All…', mnemonic: 'S', run: showAll }]);
  }

  function when(iso) {
    var d = new Date(iso);
    if (isNaN(d)) return '';
    var today = new Date(), day = 24 * 60 * 60 * 1000;
    var start = new Date(today.getFullYear(), today.getMonth(), today.getDate()).getTime();
    var time = dinahRegional.format(d, 'clock');
    if (d.getTime() >= start) return 'Today, ' + time;
    if (d.getTime() >= start - day) return 'Yesterday, ' + time;
    return dinahRegional.format(d, 'date');
  }

  /* Show All… lists every kept entry, newest first, in a PUDL grid: the
     arrow keys move, Enter or a double-click opens, Escape or Cancel closes. */
  function showAll() {
    var dialog = document.createElement('dialog');
    dialog.className = 'dialog recent-documents';
    dialog.setAttribute('aria-labelledby', 'recent-documents-title');
    dialog.innerHTML = '<h2 class="dialog-title" id="recent-documents-title">Recent Documents</h2>' +
      '<p class="dialog-body">The documents you opened most recently, newest first.</p>' +
      '<div class="data-table-wrap recent-documents-list"><table class="data-table" role="grid" aria-labelledby="recent-documents-title">' +
        '<thead><tr><th scope="col">Document</th><th scope="col">Workspace</th><th scope="col">Opened</th></tr></thead><tbody></tbody></table></div>' +
      '<div class="dialog-actions"><button type="button" class="btn" data-no>Cancel</button><button type="button" class="btn btn-primary" data-yes>Open</button></div>';
    var body = dialog.querySelector('tbody'), table = dialog.querySelector('table');
    recent.forEach(function (e, i) {
      var tr = document.createElement('tr');
      tr.setAttribute('aria-selected', String(i === 0));
      tr.dataset.index = String(i);
      var name = document.createElement('td'), mark = document.createElement('span');
      mark.className = 'glyph'; mark.style.setProperty('--glyph', 'var(--glyph-document)'); mark.setAttribute('aria-hidden', 'true');
      name.append(mark, document.createTextNode(' ' + e.title));
      var ws = document.createElement('td'); ws.className = 'recent-documents-detail'; ws.textContent = e.workspaceName || '';
      var at = document.createElement('td'); at.className = 'recent-documents-detail'; at.textContent = when(e.opened);
      tr.append(name, ws, at);
      body.appendChild(tr);
    });
    document.body.appendChild(dialog);
    window.pudlGrid.enhance(table);
    function selected() { var row = body.querySelector('tr[aria-selected="true"]'); return row ? recent[+row.dataset.index] : null; }
    function choose() { var e = selected(); dialog.close(); if (e) openEntry(e); }
    dialog.addEventListener('close', function () { dialog.remove(); });
    dialog.querySelector('[data-no]').addEventListener('click', function () { dialog.close(); });
    dialog.querySelector('[data-yes]').addEventListener('click', choose);
    table.addEventListener('pudl:row-open', choose);
    dialog.showModal();
    var first = body.querySelector('tr[aria-selected="true"]');
    if (first) first.focus();
  }

  /* Find and Open ranks the recent documents first, newest first, so Ctrl+P
     then Enter opens the last document. */
  function findItems() {
    return recent.map(function (e, i) {
      return { title: e.title, glyph: 'document', rank: i, detail: 'Recent Document' + (e.workspaceName ? ' · ' + e.workspaceName : ''), open: function () { return openEntry(e); } };
    });
  }

  document.addEventListener('DOMContentLoaded', function () {
    if (!host() || !api()) return;
    host().findSource(findItems);
    host().onResync(load);
    host().on('documents:recent', function (change) {
      if (change && change.error) host().toast(change.error, 'danger');
      load();
    });
    document.addEventListener('visibilitychange', function () { if (document.visibilityState === 'visible') load(); });
    window.addEventListener('focus', load);
    load();
  });
})();
