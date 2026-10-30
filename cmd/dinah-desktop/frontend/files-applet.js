/* Adapted from Paul Parks's Files applet. See vendor/parks-files/NOTICE.txt. */
(function () {
  'use strict';
  var serial = 0;
  function init(root, opts, item) {
    var id = ++serial, FB = window.pcFileBrowser, F = dinahResources.createTree();
    var browser, destroyed = false, previewRequest = 0, timer, loading = false, stateReady = false, failedPath = '';
    // Embedders can omit the toolbar, or keep a fixed directory with displayOnly.
    var displayOnly = opts.displayOnly === true, listOnly = opts.listOnly === true || displayOnly;
    var showToolbar = opts.toolbar !== false && !displayOnly;
    var rooted = opts.rooted === true, navigationRoot = '';
    var stateKey = 'dinah-files:' + (rooted ? 'rooted:' : '') + (displayOnly ? 'display:' : listOnly ? 'list:' : '') + dinahWorkbench.key(item), saved = window.dinahUIState ? dinahUIState.get(stateKey, {}) : {};
    var previewVisible = true, previewApplet = null;
    var initialPath = item.path, initialRef;
    if (initialPath && initialPath[0] === '{') { try { var address = JSON.parse(initialPath); initialPath = address.path; initialRef = address.ref; } catch (_) { initialPath = '/workspaces'; } }
    root.classList.add('pc-files', 'pc-files-fill', 'dinah-files');
    /* The toolbar is PUDL's folding toolbar of the window's commands
       (commandTable, below): Go before the path, and the folder's commands
       and Actions after it, which fold as the window narrows; the path keeps
       its place. */
    root.innerHTML = (showToolbar ? '<div class="fm-toolbar toolbar" role="toolbar" aria-label="Files" data-toolbar-fold>' +
      '<div class="toolbar-group" aria-label="Go" data-toolbar-glyph="home" data-toolbar-priority="4"><button type="button" data-command="up"></button><button type="button" data-command="home"></button></div>' +
      FB.markup.pathBar() +
      '<div class="toolbar-group" data-toolbar-group="Folder" data-toolbar-glyph="folder" data-toolbar-priority="2"><button type="button" data-command="new-folder"></button><button type="button" data-command="upload"></button><button type="button" data-command="refresh"></button></div>' +
      '<div class="toolbar-group" aria-label="Actions" data-toolbar-priority="3" data-toolbar-end><button type="button" data-command="files-actions"></button></div></div>' : '') +
      FB.markup.body({tree: !listOnly}) + '<div class="fm-page"><button type="button" class="btn btn-sm" data-action="more" hidden>Load More</button></div>' +
      '<section class="fm-detail" aria-label="Selection Details" aria-live="polite"></section>' +
      '<section class="fm-preview" data-ui-scroll="files-preview" aria-label="Resource Preview" hidden></section>' +
      '<p class="fm-status" role="status"></p>';
    var q = function (selector) { return root.querySelector(selector); };
    if (!listOnly) q('[data-fb="tree-pane"]').dataset.uiScroll = 'files-tree';
    q('[data-fb="main"]').dataset.uiScroll = 'files-list';
    function say(message, error) { if (destroyed) return; q('.fm-status').textContent = message || ''; q('.fm-status').classList.toggle('fm-status-error', !!error); }
    function current() { return browser && (browser.selected() || browser.cwd()); }
    function state() { return 'path=' + encodeURIComponent(failedPath || (browser ? browser.cwd().path : initialPath || '/workspaces')); }
    function persist() {
      if (!browser || !stateReady || destroyed || failedPath) return;
      var snapshot = browser.snapshot(); snapshot.treeWidth = saved.treeWidth;
      snapshot.previewVisible = previewVisible;
      snapshot.cwdRef = browser.cwd().ref || null; snapshot.selectedRef = browser.selected() && browser.selected().ref || null;
      if (window.dinahUIState) dinahUIState.set(stateKey, snapshot);
    }
    function refresh() {
      if (!browser) return;
      loading = true; redraw(); say('Refreshing…');
      return F.refresh().then(function () { return F.load(browser.cwd(), false, true); }).then(function () { if (!destroyed) { browser.refresh(); select(browser.selected()); say(''); } }).catch(function (error) { say(String(error), true); }).finally(function () { loading = false; redraw(); });
    }
    // New Folder… makes a folder in the folder shown and selects it.
    function newFolder() {
      if (!browser) return;
      return dinahResources.newFolder(F, browser.cwd()).then(function (node) {
        if (!node || destroyed) return;
        browser.refresh(); browser.select(node.name, true); say('Made the folder ' + node.name + '.');
      }).catch(function (error) { say(String(error), true); });
    }
    // Upload Files… copies files from anywhere on the computer into the
    // folder shown; files dropped onto the listing go there too.
    function shownAgain(dir) {
      if (destroyed || !browser || !dinahResources.sameRef(dir.ref, browser.cwd().ref)) return;
      F.load(browser.cwd(), false, true).then(function () { if (!destroyed) { browser.refresh(); redraw(); } }).catch(function (error) { say(String(error), true); });
    }
    function uploadFiles() {
      if (!browser) return;
      var dir = browser.cwd();
      return dinahResources.chooseAndUpload(dir).then(function (r) { if (r) shownAgain(dir); }).catch(function (error) { say(String(error), true); });
    }
    function more() {
      if (!browser || loading) return;
      loading = true; redraw();
      return F.load(browser.cwd(), true).then(function () { if (!destroyed) { browser.refresh(); persist(); } }).catch(function (error) { say(String(error), true); }).finally(function () { loading = false; redraw(); });
    }
    /* The window's commands, declared once for PUDL's commands
       (pudl-commands.js): the toolbar, the Actions menu and the window's
       menu name them by id, and an agent lists and runs them. What applies
       to the selection is shown; the rest is hidden. */
    var ACTIONS = ['retry', 'open', 'edit', 'browse-children', 'open-external', 'new-folder', 'upload', 'copy-path', 'rename', 'refresh', 'preview-pane', 'new-terminal'];
    function fail(error) { say(String(error), true); }
    function commandTable() {
      var node = current(), cwd = browser && browser.cwd(), takes = !!cwd && !displayOnly && dinahResources.takesFolder(cwd);
      var external = !node ? '' : dinahResources.has(node, 'open-external') ? 'Open in Default Application' : dinahResources.has(node, 'reveal') ? 'Show in Folder' : '';
      var opens = !!node && (node.ref && dinahResources.has(node, 'preview') || node.directory && dinahResources.has(node, 'list'));
      var notHere = 'This folder does not take new files.';
      return [
        { id: 'up', label: 'Up One Folder', glyph: 'arrow-up', disabled: !browser || !browser.canUp() || loading, run: function () { browser.up(); } },
        { id: 'home', label: rooted ? 'Starting Folder' : 'Home', glyph: 'home', disabled: !browser, run: function () { browser.goPath(navigationRoot || F.home().path); } },
        { id: 'retry', label: 'Retry', hidden: !failedPath || !node, run: function () {
          F.lookup(failedPath).then(function (n) { checkIdentity(n, saved.cwdRef || initialRef); return dinahResources.verify(n); }).then(function () { return browser.goPath(failedPath); }).catch(fail);
        } },
        { id: 'open', label: node && node.directory && !dinahResources.has(node, 'preview') ? 'Open Folder' : node && node.directory ? 'Open Content' : 'Open', hidden: !opens, run: function () { open(node); } },
        { id: 'edit', label: 'Edit', hidden: !(node && node.ref && dinahResources.has(node, 'edit') && window.dinahEditor), run: function () { window.dinahEditor.openResource(node.ref).catch(fail); } },
        { id: 'browse-children', label: 'Browse Children', hidden: !(node && !displayOnly && node.directory && dinahResources.has(node, 'list') && dinahResources.has(node, 'preview')), run: function () { browser.go(node); } },
        { id: 'open-external', label: external || 'Open Outside', hidden: !external, run: function () {
          var api = window.dinahHelpers.app();
          (node.ref ? api.OpenWorkspaceResource(node.ref) : api.OpenWorkspaceMount(node.path)).catch(fail);
        } },
        { id: 'new-folder', label: 'New Folder…', hidden: displayOnly, disabled: loading || !takes, reason: !takes ? notHere : null, run: newFolder },
        { id: 'upload', label: 'Upload Files…', glyph: 'upload', hidden: displayOnly, disabled: loading || !takes, reason: !takes ? notHere : null, run: uploadFiles },
        { id: 'copy-path', label: 'Copy Path', hidden: !(node && node.ref && node.writable), run: function () { dinahResources.copyPath(node).catch(fail); } },
        { id: 'rename', label: 'Rename…', shortcut: 'F2', hidden: displayOnly || !dinahResources.renames(node), run: function () { dinahResources.renameFile(node).catch(fail); } },
        { id: 'refresh', label: 'Refresh', glyph: 'reload', disabled: loading || !browser, run: refresh },
        { id: 'preview-pane', label: 'Show the Preview Pane', checked: previewVisible, hidden: listOnly, run: function () {
          previewVisible = !previewVisible;
          if (window.dinahUIState) dinahUIState.set('dinah-files-preferences', { previewVisible: previewVisible });
          select(browser.selected()); persist();
        } },
        { id: 'new-terminal', label: 'New Terminal Here', hidden: !node, run: function () { var folder = node.directory ? node : browser.cwd(); dinahResources.openTerminal(folder.path, folder.ref).catch(fail); } },
        { id: 'files-actions', label: 'Actions', items: ACTIONS, disabled: !node }
      ];
    }
    var unregisterCommands = window.pudlCommands ? window.pudlCommands.register(root, commandTable) : function () {};
    function redraw() {
      if (!browser || destroyed) return;
      dinahResources.decorate(root, F, browser.cwd());
      q('[data-action="more"]').hidden = browser.cwd().nextStart < 0;
      q('[data-action="more"]').disabled = loading;
      if (window.pudlCommands) window.pudlCommands.changed();
    }
    function detail(node) {
      var host = q('.fm-detail'); host.replaceChildren();
      var title = document.createElement('strong'); title.textContent = node.title || node.name || '/'; host.appendChild(title);
      var path = document.createElement('code'); path.textContent = node.path; host.appendChild(path);
      var revision = document.createElement('span'); revision.className = 'fm-d-meta'; revision.textContent = (node.kind || 'Folder') + (node.revision ? ' · Revision ' + node.revision : ''); host.appendChild(revision);
    }
    function select(node) {
      clearTimeout(timer); var generation = ++previewRequest;
      var preview = q('.fm-preview');
      if (previewApplet) previewApplet.destroy();
      previewApplet = null; preview.replaceChildren(); preview.hidden = true;
      if (!browser || destroyed) return;
      node = node || browser.cwd(); detail(node); redraw();
      if (!previewVisible || !node.ref || !dinahResources.has(node, 'preview')) return;
      preview.hidden = false; preview.textContent = 'Loading Preview…';
      timer = setTimeout(function () {
        if (destroyed || generation !== previewRequest) return;
        var host = document.createElement('div'); preview.replaceChildren(host);
        previewApplet = dinahWorkbench.embed(dinahWorkbench.resourceItem(node.ref, node.title), host, {});
      }, 180);
    }
    function open(node) {
      if (node.directory && !dinahResources.has(node, 'preview')) return displayOnly ? dinahResources.openFiles(node.path, node.ref) : browser.go(node);
      return dinahResources.open(node, function () { return !destroyed; }).catch(function (error) { say(String(error), true); });
    }
    function pathFrom(value) { return new URLSearchParams(value || '').get('path'); }
    function checkIdentity(node, ref) { if (ref && !dinahResources.compatibleRef(ref, node.ref)) throw new Error('This location now refers to a different workspace source. Open it again from the workspace catalog.'); }
    function setState(value) { var path = pathFrom(value); if (!displayOnly && browser && path && path !== browser.cwd().path) return browser.goPath(path); }
    var ready = Promise.resolve(window.dinahUIState && dinahUIState.ready).then(async function () {
      if (window.dinahUIState) {
        saved = dinahUIState.get(stateKey, {});
        var preferences = dinahUIState.get('dinah-files-preferences', {});
        previewVisible = typeof preferences.previewVisible === 'boolean' ? preferences.previewVisible : saved.previewVisible !== false;
      }
      if (listOnly) previewVisible = false;
      if (displayOnly) saved = {};
      var start = (displayOnly ? initialPath : pathFrom(opts.state) || saved.cwd || initialPath) || '/workspaces';
      var node;
      try {
        if (rooted) {
          navigationRoot = initialPath || '/workspaces';
          if (initialRef) navigationRoot = (await dinahResources.verify({path:navigationRoot,ref:initialRef})).path;
          function inside(path) { return !path.split('/').some(function(part){return part==='.'||part==='..';}) && (path===navigationRoot || path.startsWith(navigationRoot.replace(/\/$/,'')+'/')); }
          if (!inside(start)) { start=navigationRoot; saved={}; }
        }
        var retainedRef = saved.cwd === start ? saved.cwdRef : start === initialPath ? initialRef : null;
        if (retainedRef && window.dinahHelpers.app().ResolveWorkspaceResourceMount) {
          var mounted = await dinahResources.verify({ path: start, ref: retainedRef });
          start = mounted.path;
        }
        node = await F.lookup(start); checkIdentity(node, retainedRef); await F.load(node);
        if (retainedRef && saved.cwdRef && dinahResources.compatibleRef(retainedRef, saved.cwdRef)) {
          var oldPath = saved.cwd;
          saved = Object.assign({}, saved, { cwd: start, expanded: (saved.expanded || []).map(function (path) { return path === oldPath || path.startsWith(oldPath + '/') ? start + path.slice(oldPath.length) : path; }) });
        }
        if (saved.cwd === start && saved.selected && saved.selectedRef) checkIdentity(F.childNamed(node, saved.selected) || await F.lookup(start.replace(/\/$/, '') + '/' + saved.selected), saved.selectedRef);
      }
      catch (error) { failedPath = start; say('Could not open ' + start + '. ' + String(error), true); node = F.root(); }
      if (destroyed || (displayOnly || rooted) && failedPath) return;
      browser = FB.create(root, F, { start: node.path, treeWidth: saved.treeWidth, rowLinks: false, navigate: !displayOnly, navigationRoot: navigationRoot, parentEntry: rooted && !showToolbar,
        formatDate: function (d) { return dinahRegional.format(d, 'date'); },
        say: say, onSelect: select, onOpen: open,
        onNavigate: function () { failedPath = ''; select(null); if (opts.changed) opts.changed(state()); },
        onRender: redraw, onLoading: function (value) { loading = value; redraw(); },
        onResize: function (width) { saved.treeWidth = width; persist(); }
      });
      await browser.restore(failedPath ? {} : saved);
      if (destroyed) return;
      stateReady = true; browser.onChange(persist); select(browser.selected()); redraw();
      if (opts.changed) opts.changed(state());
      if (window.dinahUIState) dinahUIState.refreshScroll(root);
    });
    root.addEventListener('click', function (event) {
      var button = event.target.closest('[data-action]'); if (!button || !browser) return;
      if (button.dataset.action === 'more') more();
    });
    // Files dropped onto the listing go into the folder shown, when it takes
    // files; the listing is then loaded again.
    if (!displayOnly && dinahResources.uploadTarget) dinahResources.uploadTarget(q('[data-fb="main"]'), function () { var dir = browser && browser.cwd(); return dinahResources.takesFolder(dir) ? dir : null; });
    // A file renamed in the folder shown is listed again, under its new
    // name, and stays selected.
    function renamedHere(event) {
      var path = event.ref.path, dir = Object.assign({}, event.ref, { path: path.slice(0, Math.max(0, path.lastIndexOf('/'))) });
      if (destroyed || !browser || !dinahResources.sameRef(dir, browser.cwd().ref)) return;
      var was = browser.selected(), name = path.slice(path.lastIndexOf('/') + 1);
      F.load(browser.cwd(), false, true).then(function () {
        if (destroyed) return;
        browser.refresh();
        if (was && dinahResources.sameRef(was.ref, event.from)) browser.select(name, true);
        redraw();
      }).catch(function (error) { say(String(error), true); });
    }
    var unsubscribe = dinahResources.subscribe(function (event) {
      if (event && event.kind === 'uploaded') shownAgain({ ref: event.ref });
      if (event && event.kind === 'renamed') renamedHere(event);
    });
    return { ready: ready, state: state, setState: setState, commands: function () { return ACTIONS; }, destroy: function () { persist(); destroyed = true; unsubscribe(); unregisterCommands(); previewRequest++; clearTimeout(timer); if (previewApplet) previewApplet.destroy(); if (browser) browser.destroy(); F.destroy(); } };
  }
  window.dinahFiles = { init: init };
})();
