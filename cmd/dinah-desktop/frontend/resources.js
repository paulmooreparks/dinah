/* Shared human-facing resource adapter for Files and Terminal. */
(function () {
  'use strict';
  const app = window.dinahHelpers.app;
  function has(item, action) { return (item.actions || []).includes(action); }
  function sameRef(a, b) { return !!a && !!b && a.workspaceId === b.workspaceId && a.sourceId === b.sourceId && (a.path || '') === (b.path || ''); }
  function sameOwner(a, b) { return !!a && !!b && a.workspaceId === b.workspaceId && a.sourceId === b.sourceId; }
  // Expected methods validate the mounted source. Its root may resolve to a canonical page ID.
  function compatibleRef(a, b) { return sameOwner(a, b) && (sameRef(a, b) || !(a.path || '') || !(b.path || '')); }
  /* The folders every tree has opened are watched on disk (workspace_watch.go),
     so a change made outside Dinah, by an agent, another program
     or a sync client, arrives as workspace:changed of kind
     disk-changed, and each tree lists its opened folders again. Each tree
     gives a function that lists its opened folders; the set is sent after
     the trees settle, and only when it changed. */
  var trees = new Set(), watchTimer = 0, watchedKey = null;
  function watchFolders() {
    clearTimeout(watchTimer);
    watchTimer = setTimeout(function () {
      var refs = [], seen = {};
      trees.forEach(function (opened) {
        opened().forEach(function (ref) {
          var k = JSON.stringify([ref.workspaceId || '', ref.sourceId || '', ref.path || '']);
          if (!seen[k]) { seen[k] = true; refs.push(ref); }
        });
      });
      var key = Object.keys(seen).sort().join('|');
      if (key === watchedKey || !app().WatchFolders) return;
      watchedKey = key;
      Promise.resolve(app().WatchFolders(refs)).catch(function () { watchedKey = null; });
    }, 200);
  }
  var subscribers = new Set();
  function notify(event) { subscribers.forEach(function (fn) { fn(event); }); }
  function subscribe(fn) { subscribers.add(fn); return function () { subscribers.delete(fn); }; }
  document.addEventListener('dinah:workspace-resource-labels', function () { notify({kind:'labels'}); });
  window.dinahHost.on('workspace:changed', notify);
  // After a resync every tree lists its folders again and the catalog and terminals check theirs.
  window.dinahHost.onResync(function () { notify({ kind: 'resync' }); });
  function verify(item) {
    return (item.ref && app().ResolveWorkspaceResourceMount ? app().ResolveWorkspaceResourceMount(item.ref) : item.ref ? app().StatWorkspaceMountExpected(item.path, item.ref) : app().StatWorkspaceMount(item.path)).then(function (current) {
      if (item.ref && !compatibleRef(item.ref, current.ref)) throw new Error('This location now refers to a different workspace source. Open it again from the workspace catalog.');
      return current;
    });
  }
  function openFiles(path, ref) { return dinahWorkbench.open({ kind: 'files', workspace: ref ? ref.workspaceId : '', path: ref ? JSON.stringify({ path: path, ref: ref }) : path || '/workspaces' }); }
  /* A Terminal runs the shell given, or the one chosen in Settings, which is
     Dinah's own when the computer's shells cannot be listed. */
  function openTerminal(path, ref, shell) {
    var id = Array.from(crypto.getRandomValues(new Uint8Array(16)), function (value) { return value.toString(16).padStart(2, '0'); }).join('');
    var a = window.dinahHelpers.app();
    var chosen = shell ? Promise.resolve(shell) : a && a.TerminalShells ? Promise.resolve(a.TerminalShells()).then(function (s) { return s.default; }).catch(function () { return 'dinah'; }) : Promise.resolve('dinah');
    return chosen.then(function (name) { return dinahWorkbench.open({ kind: 'terminal', path: JSON.stringify([path || '/workspaces', id, ref || null, name || 'dinah']) }); });
  }
  function open(item, isCurrent) {
    return verify(item).then(function (current) {
      if (isCurrent && !isCurrent()) return;
      if (current.ref && has(current, 'preview')) return dinahWorkbench.open(dinahWorkbench.resourceItem(current.ref, current.title));
      if (current.directory && has(current, 'list')) return openFiles(current.path, current.ref);
      if (['open-external', 'reveal'].some(function (action) { return has(current, action); })) return current.ref ? app().OpenWorkspaceResource(current.ref) : app().OpenWorkspaceMount(current.path);
      throw new Error('This resource has no supported Open action.');
    });
  }
  function currentPath() {
    return app().State().then(function (state) {
      var record = (state.workspaceRecords || []).find(function (entry) { return entry.id === state.workspaceId; });
      return record && record.mountAlias ? '/workspaces/' + record.mountAlias : '/workspaces';
    });
  }
  /* The home folder's path, /home/<name>, from Dinah; until it
     answers, the name every installation starts with. */
  var homePath = '/home/dinah', homeLoading = null;
  function loadHome(again) {
    if (!homeLoading || again) {
      homeLoading = Promise.resolve().then(function () { return app() && app().HomeMountPath ? app().HomeMountPath() : homePath; })
        .then(function (path) { if (typeof path === 'string' && /^\/home\/[^/]+$/.test(path)) homePath = path; return homePath; }, function () { return homePath; });
    }
    return homeLoading;
  }
  /* A resource's glyph by what the item is: a folder or a document. Every
     surface that lists resources chooses through this, so they agree. */
  function glyphName(item) { return item.directory ? 'folder' : 'document'; }
  /* Whether a folder of the file system takes a new document: a folder on
     the computer that can be written. The rest is read-only: the top of the
     file system and the listing of a workspace's mounts. */
  function readOnly(node) { return !!node && !!node.directory && !node.writable; }
  /* The read-only mark: a struck-through pencil, as the Finder shows for a
     folder that cannot be written to, PUDL's pencil-off glyph. */
  function readOnlyMark() {
    var mark = document.createElement('span');
    mark.className = 'glyph fm-readonly-mark';
    mark.setAttribute('role', 'img');
    mark.setAttribute('aria-label', 'Read-only');
    mark.title = 'Read-only: nothing can be saved here';
    return mark;
  }
  /* New Folder, shared by Open, Save As and Files: a folder on the computer
     that can be written takes a new folder. The name is asked in a PUDL
     dialog over whatever is open, the folder is made, the listing is loaded
     again, and the promise gives the new folder's node, or null when the
     person cancels. */
  function takesFolder(node) { return !!node && !!node.directory && !!node.writable && !!node.ref; }

  /* Uploading copies files from anywhere on the computer into a folder of a
     workspace. When the folder already
     has files of the same names, one question says which and asks whether to
     replace them, keep both or skip them; Cancel uploads nothing. A toast
     says what was done, and the promise gives the report, or null when the
     person cancelled. */
  function host() { return window.dinahHost; }
  function names(list) { return list.length > 3 ? list.slice(0, 3).join(', ') + ' and ' + (list.length - 3) + ' more' : list.join(', '); }
  function upload(dir, paths) {
    if (!takesFolder(dir)) return Promise.reject(new Error('Files can be uploaded only to a folder on the computer for now.'));
    if (!paths || !paths.length) return Promise.resolve(null);
    return Promise.resolve(app().UploadConflicts(dir.ref, paths)).then(function (taken) { return existingChoice(dir, taken); }).then(function (existing) {
      if (!existing) return null;
      return Promise.resolve(app().UploadToWorkspaceFolder(dir.ref, paths, existing)).then(function (r) { return reported(dir, r); });
    });
  }
  /* Files dropped from the system's file manager reach the page as their
     contents and names, not their paths, so they are sent to the shell's
     POST /upload, which applies the same rules as an upload by path. */
  function uploadDropped(dir, files, folders) {
    if (!takesFolder(dir)) return Promise.reject(new Error('Files can be uploaded only to a folder on the computer for now.'));
    var refused = folders.map(function (n) { return n + ': a folder is not uploaded; upload the files in it'; });
    if (!files.length) return Promise.resolve(reported(dir, { refused: refused }));
    var list = files.map(function (f) { return f.name; });
    return Promise.resolve(app().UploadNameConflicts(dir.ref, list)).then(function (taken) { return existingChoice(dir, taken); }).then(function (existing) {
      if (!existing) return null;
      var form = new FormData();
      form.append('folder', JSON.stringify(dir.ref));
      form.append('existing', existing);
      files.forEach(function (f) { form.append('file', f, f.name); });
      return fetch('/upload', { method: 'POST', credentials: 'same-origin', body: form }).then(function (res) {
        if (!res.ok) return res.text().then(function (t) { throw new Error(t || 'The upload failed.'); });
        return res.json();
      }).then(function (r) { r.refused = refused.concat(r.refused || []); return reported(dir, r); });
    });
  }
  /* What happens to names the folder already has: one question says which
     and asks whether to replace them, keep both or skip them. "skip" when
     there are none, and null when the person cancelled. */
  function existingChoice(dir, taken) {
    if (!taken || !taken.length) return 'skip';
    var where = dir.title || dir.name || dir.path, one = taken.length === 1;
    return host().ask(one ? 'Replace ' + taken[0] + '?' : 'Replace ' + taken.length + ' Files?',
      where + ' already has ' + (one ? 'a file named ' + taken[0] : 'files named ' + names(taken)) + '. Replace ' + (one ? 'it' : 'them') + ', keep both (the new ' + (one ? 'one is' : 'ones are') + ' named with a number), or skip ' + (one ? 'it' : 'them') + ' and upload the rest.',
      'Replace', true, [{ label: 'Skip', value: 'skip' }, { label: 'Keep Both', value: 'keep-both' }])
      .then(function (answer) { return answer === true ? 'replace' : answer || null; });
  }
  /* A toast says what an upload did; the report is passed on. */
  function reported(dir, r) {
    var where = dir.title || dir.name || dir.path;
    var done = (r.added || []).length + (r.replaced || []).length + (r.kept || []).length;
    var parts = [];
    if (done) parts.push((done === 1 ? '1 file' : done + ' files') + ' uploaded to ' + where + '.');
    if ((r.skipped || []).length) parts.push('Skipped: ' + names(r.skipped) + '.');
    if ((r.refused || []).length) parts.push('Not uploaded: ' + r.refused.join('; ') + '.');
    host().toast(parts.join(' ') || 'Nothing was uploaded.', (r.refused || []).length ? 'warn' : 'info');
    return r;
  }
  function chooseAndUpload(dir) {
    return Promise.resolve(app().ChooseUploadFiles()).then(function (paths) { return paths && paths.length ? upload(dir, paths) : null; });
  }
  function copyPath(item) {
    return Promise.resolve(app().WorkspaceLocalPath(item.ref)).then(function (p) { host().copy(p); return p; });
  }
  /* Files dropped from the system's file manager land on a drop target: an
     element registered here, which says which folder a drop on it means
     with a resolver, so the Workspace tree and Files each decide. Only a
     drag that carries files is taken, and the target is outlined while one
     is over it (app.css, .upload-drop-active). */
  var dropTargets = new WeakMap(), dropOver = null;
  function uploadTarget(el, resolve) { el.dataset.uploadTarget = ''; dropTargets.set(el, resolve); }
  function targetOf(node) {
    var el = node;
    while (el && !(el.dataset && 'uploadTarget' in el.dataset && dropTargets.has(el))) el = el.parentElement;
    return el;
  }
  function carriesFiles(e) { return e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types || [], 'Files') >= 0; }
  function markDrop(el) {
    if (dropOver === el) return;
    if (dropOver) dropOver.classList.remove('upload-drop-active');
    dropOver = el;
    if (el) el.classList.add('upload-drop-active');
  }
  document.addEventListener('dragover', function (e) {
    if (!carriesFiles(e)) return;
    var el = targetOf(e.target);
    markDrop(el);
    if (!el) return;
    e.preventDefault();
    e.dataTransfer.dropEffect = 'copy';
  });
  document.addEventListener('dragleave', function (e) { if (!e.relatedTarget) markDrop(null); });
  document.addEventListener('drop', function (e) {
    if (!carriesFiles(e)) return;
    var el = targetOf(e.target);
    markDrop(null);
    if (!el) return;
    e.preventDefault();
    var dir = dropTargets.get(el)(e.target);
    if (!dir) return;
    // A dropped folder arrives as an entry that is a directory, which is
    // reported rather than sent.
    var files = [], folders = [], items = e.dataTransfer.items || [];
    for (var i = 0; i < items.length; i++) {
      if (items[i].kind !== 'file') continue;
      var entry = items[i].webkitGetAsEntry && items[i].webkitGetAsEntry(), f = items[i].getAsFile();
      if (entry && entry.isDirectory) folders.push(entry.name);
      else if (f) files.push(f);
    }
    uploadDropped(dir, files, folders).then(function (r) { if (r) notify({ kind: 'uploaded', ref: dir.ref }); }).catch(function (err) { host().toast(String(err && err.message || err), 'danger'); });
  });
  /* askName asks for a name in a PUDL dialog: o.title, o.label, o.value,
     o.help, o.yes, and o.selectTo, how much of the value starts selected.
     o.commit(name) does the work; while it runs the dialog waits, and an
     error it gives is said in the dialog, which stays open. The promise
     gives commit's result, or null when the person cancels or leaves the
     value as it was. */
  function askName(o) {
    return new Promise(function (resolve) {
      var dlg = document.createElement('dialog'), id = 'dinah-ask-name-' + Date.now();
      dlg.className = 'dialog dinah-ask-name';
      dlg.setAttribute('aria-labelledby', id + '-title');
      dlg.innerHTML = '<form method="dialog">' +
        '<h2 class="dialog-title" id="' + id + '-title"></h2>' +
        '<div class="dialog-body"><div class="form-group">' +
          '<label class="form-label" for="' + id + '-name"></label>' +
          '<input class="form-input" id="' + id + '-name" autocomplete="off" spellcheck="false" aria-describedby="' + id + '-help">' +
          '<p class="form-help" id="' + id + '-help"></p>' +
        '</div><p class="form-error" role="alert" hidden></p></div>' +
        '<div class="dialog-actions"><button type="button" class="btn" data-name="cancel">Cancel</button><button type="submit" class="btn btn-primary"></button></div>' +
      '</form>';
      var input = dlg.querySelector('input'), error = dlg.querySelector('.form-error'), yes = dlg.querySelector('[type="submit"]'), done = false;
      dlg.querySelector('.dialog-title').textContent = o.title;
      dlg.querySelector('.form-label').textContent = o.label;
      dlg.querySelector('.form-help').textContent = o.help || '';
      yes.textContent = o.yes || 'OK';
      input.value = o.value || '';
      function finish(result) { if (done) return; done = true; if (dlg.open) dlg.close(); dlg.remove(); resolve(result); }
      function say(text) { error.textContent = text || ''; error.hidden = !text; input.toggleAttribute('aria-invalid', !!text); }
      dlg.addEventListener('close', function () { finish(null); });
      dlg.querySelector('[data-name="cancel"]').addEventListener('click', function () { finish(null); });
      input.addEventListener('input', function () { say(''); });
      dlg.addEventListener('submit', function (event) {
        event.preventDefault();
        var name = input.value.trim();
        if (!name) { say('Type a ' + o.label.toLowerCase() + '.'); return; }
        if (name === (o.value || '')) { finish(null); return; }
        yes.disabled = true;
        Promise.resolve().then(function () { return o.commit(name); }).then(finish).catch(function (err) {
          var text = String(err && err.message || err).replace(/^Error:\s*/, '');
          say(text.charAt(0).toUpperCase() + text.slice(1) + (/[.!?]$/.test(text) ? '' : '.'));
          yes.disabled = false; input.focus();
        });
      });
      document.body.appendChild(dlg);
      dlg.showModal();
      input.focus();
      input.setSelectionRange(0, o.selectTo == null ? input.value.length : o.selectTo);
    });
  }
  /* Rename… for a file of a local folder, offered where the file's actions
     include rename: the file is renamed in its own folder, and its drafts,
     Recent entry and pin go with it (documents.Ops.Rename). The name is
     offered with its extension unselected, since the extension is usually
     kept. The promise gives the renamed file, or null when the person
     cancels. */
  function renames(item) { return !!item && has(item, 'rename') && !!item.ref && !!app().RenameDocumentFile; }
  function renameFile(item) {
    if (!item || !item.ref || !item.ref.path) return Promise.reject(new Error('Only a file in a folder on the computer can be renamed here.'));
    var name = item.ref.path.slice(item.ref.path.lastIndexOf('/') + 1), dot = name.lastIndexOf('.');
    return askName({ title: 'Rename the File', label: 'Name', value: name, yes: 'Rename', selectTo: dot > 0 ? dot : null,
      help: 'The file keeps its folder. A document open in the Editor follows it, with its changes.',
      commit: function (next) { return app().RenameDocumentFile(item.ref, next); } }).then(function (renamed) {
      if (!renamed) return null;
      host().toast('Renamed "' + name + '" to "' + renamed.name + '".', 'positive');
      notify({ kind: 'renamed', from: item.ref, ref: renamed.ref });
      return renamed;
    });
  }
  function newFolder(F, dir) {
    if (!takesFolder(dir)) return Promise.reject(new Error('A folder can be made only in a folder on the computer.'));
    return new Promise(function (resolve) {
      var dlg = document.createElement('dialog'), id = 'dinah-new-folder-' + Date.now();
      dlg.className = 'dialog dinah-new-folder';
      dlg.setAttribute('aria-labelledby', id + '-title');
      dlg.innerHTML = '<form method="dialog">' +
        '<h2 class="dialog-title" id="' + id + '-title">New Folder</h2>' +
        '<div class="dialog-body"><div class="form-group">' +
          '<label class="form-label" for="' + id + '-name">Folder Name</label>' +
          '<input class="form-input" id="' + id + '-name" autocomplete="off" spellcheck="false" value="New Folder">' +
          '<p class="form-help"></p>' +
        '</div><p class="form-error" role="alert" hidden></p></div>' +
        '<div class="dialog-actions"><button type="button" class="btn" data-folder="cancel">Cancel</button><button type="submit" class="btn btn-primary">Create</button></div>' +
      '</form>';
      var input = dlg.querySelector('input'), error = dlg.querySelector('.form-error'), create = dlg.querySelector('[type="submit"]'), done = false;
      dlg.querySelector('.form-help').textContent = 'The folder is made in ' + (dir.title || dir.name || dir.path) + '.';
      function finish(node) { if (done) return; done = true; if (dlg.open) dlg.close(); dlg.remove(); resolve(node); }
      function say(text) { error.textContent = text || ''; error.hidden = !text; }
      dlg.addEventListener('close', function () { finish(null); });
      dlg.querySelector('[data-folder="cancel"]').addEventListener('click', function () { finish(null); });
      input.addEventListener('input', function () { say(''); });
      dlg.addEventListener('submit', function (event) {
        event.preventDefault();
        var name = input.value.trim();
        if (!name) { say('Give the folder a name.'); return; }
        create.disabled = true;
        Promise.resolve().then(function () { return window.dinahHelpers.app().CreateWorkspaceFolder(dir.ref, name); })
          .then(function () { return F.load(dir, false, true); })
          // A long folder is listed a page at a time, so the new folder is
          // looked up by its path when the first page does not have it.
          .then(function () { return F.childNamed(dir, name) || F.lookup(dir.path.replace(/\/$/, '') + '/' + name); })
          .then(function (node) { finish(node || null); })
          .catch(function (err) {
            var text = String(err && err.message || err).replace(/^Error:\s*/, '');
            say(text.charAt(0).toUpperCase() + text.slice(1) + (/[.!?]$/.test(text) ? '' : '.'));
            create.disabled = false; input.focus();
          });
      });
      document.body.appendChild(dlg);
      dlg.showModal();
      input.focus(); input.select();
    });
  }
  /* Marks a file browser over a tree for what it shows, after each redraw:
     a read-only folder gets the read-only mark, a crumb shows its folder's
     title, and the Date and Size columns are hidden when no entry has them.
     Files and the Editor's Open and Save As dialog share it; root is the
     element with the dinah-files class. */
  function decorate(root, F, cwd) {
    function mark(element, node) {
      if (node) element.classList.toggle('fm-read-only', readOnly(node));
    }
    function marked(host, node) {
      if (!host) return;
      var old = host.querySelector(':scope > .fm-readonly-mark');
      if (readOnly(node) && !old) host.appendChild(readOnlyMark());
      else if (!readOnly(node) && old) old.remove();
    }
    root.querySelectorAll('[data-fb="tree"] a[data-path]').forEach(function (link) { var node = F.resolve(null, link.dataset.path); mark(link, node); marked(link, node); });
    root.querySelectorAll('.fm-row[data-name]').forEach(function (row) {
      var node = F.childNamed(cwd, row.dataset.name);
      mark(row, node);
      if (node) marked(row.querySelector('.fm-c-name .fm-cell'), node);
    });
    root.querySelectorAll('[data-fb="crumbs"] a[data-path], [data-fb="crumbs"] [aria-current="location"]').forEach(function (crumb) {
      var node = crumb.dataset.path ? F.resolve(null, crumb.dataset.path) : cwd;
      if (!node) return;
      // The folder shown carries the mark, as the Finder's status bar does.
      if (!crumb.dataset.path) marked(crumb, node);
      if (node.path === '/' || node.path === '/home') return;
      // The crumb's words are its first text; the mark after them stays.
      var words = crumb.firstChild && crumb.firstChild.nodeType === 3 ? crumb.firstChild : crumb.insertBefore(document.createTextNode(''), crumb.firstChild);
      words.textContent = node.title || node.name;
      crumb.title = node.path;
    });
    root.classList.toggle('fm-no-date', !(cwd.children || []).some(function (node) { return node.date; }));
    root.classList.toggle('fm-no-size', !(cwd.children || []).some(function (node) { return typeof node.size === 'number'; }));
  }
  function createTree() {
    var nodes = new Map(), identities = new Map(), disposed = false, listeners = new Set(), refreshing = false, pending = false;
    function identity(item) {
      if (!item.ref) return null;
      var root = item.ref.sourceId && item.path.startsWith('/workspaces/') && item.path.split('/').length === 4;
      return JSON.stringify([item.ref.workspaceId, item.ref.sourceId, root ? '' : item.ref.path || '']);
    }
    function put(item) {
      var key = identity(item), retained = key && identities.get(key), node = nodes.get(item.path);
      if (retained && retained.path !== item.path) {
        if (nodes.get(retained.path) === retained) nodes.delete(retained.path);
        node = retained; nodes.set(item.path, node);
      }
      if (node && (node.ref || item.ref) && !compatibleRef(node.ref, item.ref)) {
        nodes.forEach(function (_, path) { if (path.startsWith(item.path + '/')) nodes.delete(path); });
        node = null;
      }
      if (!node) { node = { name: item.path.slice(item.path.lastIndexOf('/') + 1), path: item.path, loaded: false, nextStart: -1 }; nodes.set(item.path, node); }
      Object.assign(node, item);
      // A folder that was missing and now exists is listed without the mark,
      // and so is a source that has become available.
      if (!item.missing) delete node.missing;
      if (!item.unavailable) delete node.unavailable;
      node.name = item.path.slice(item.path.lastIndexOf('/') + 1);
      if (key) identities.set(key, node);
      if (item.directory && !node.children) node.children = [];
      return node;
    }
    var root = put({ path: '/', title: '/', kind: 'directory', directory: true, actions: ['list'] });
    var homeParent = put({path:'/home',title:'home',kind:'catalog',directory:true,actions:['list']}); homeParent.parent = root;
    function homeNode(path) {
      var node = put({path:path,title:path.slice(path.lastIndexOf('/') + 1),kind:'directory',directory:true,writable:true,actions:['list'],ref:{workspaceId:'',sourceId:'dinah-home',path:''}});
      node.parent = homeParent; return node;
    }
    // The home folder is /home/<name>, and the name can change; the folder
    // keeps its identity.
    var home = homeNode(homePath);
    function placeHome(path) {
      if (disposed || !path || path === home.path) return;
      nodes.delete(home.path); home = homeNode(path);
      homeParent.children = (homeParent.children || []).filter(function (child) { return child.ref && child.ref.sourceId !== 'dinah-home'; }).concat([home]);
      listeners.forEach(function (fn) { fn(); });
    }
    loadHome().then(placeHome);
    function contentOf(node) {
      if (!node || !node.directory || !has(node, 'preview')) return null;
      var content = node.contentEntry || (node.contentEntry = {});
      Object.assign(content, {name:'@content',title:node.title,path:node.path,kind:node.kind,directory:false,ref:node.ref,revision:node.revision,parent:node,contentFacet:true,actions:(node.actions || []).filter(function (action) { return action !== 'list'; })});
      return content;
    }
    async function lookup(path) {
      var node = nodes.get(path);
      if (node) return node;
      var item = await app().StatWorkspaceMount(path);
      if (disposed) throw new Error('The resource browser was closed.');
      node = put(item);
      if (node.path !== '/') {
        node.parent = await lookup(node.path.slice(0, node.path.lastIndexOf('/')) || '/');
        if (!node.parent.children.includes(node)) node.parent.children.push(node);
      }
      return node;
    }
    function load(node, more, force) {
      if (!node || !node.directory) return Promise.resolve();
      if (node.loading) return node.loading;
      var start = more ? node.nextStart : 0;
      node.loading = verify(node).then(function (current) {
        put(current);
        if (node.loaded && !more && !force || more && node.nextStart < 0) return null;
        return node.ref ? app().ListWorkspaceMountsExpected(node.path, start, 100, node.ref) : app().ListWorkspaceMounts(node.path, start, 100);
      }).then(function (page) {
        if (!page) return;
        if (disposed) return;
        var entries = (page.entries || []).map(function (item) { var child = put(item); child.parent = node; return child; });
        node.children = more ? node.children.concat(entries.filter(function (child) { return !node.children.some(function (old) { return old.path === child.path; }); })) : entries;
        if (!node.loaded) watchFolders();
        node.loaded = true;
        node.error = '';
        node.loadedThrough = start;
        node.nextStart = typeof page.nextStart === 'number' ? page.nextStart : -1;
        if (node.nextStart >= 0 && node.nextStart <= start) throw new Error('The resource provider returned an invalid next page.');
      }).finally(function () { node.loading = null; });
      return node.loading;
    }
    async function refresh() {
      if (refreshing) { pending = true; return; }
      refreshing = true;
      try {
        do {
          pending = false;
          // Refresh ancestors as well as loaded listings so labels and breadcrumbs agree.
          for (var node of Array.from(nodes.values())) {
            if (disposed) return;
            try {
              put(await verify(node));
              if (node.path !== '/') node.parent = await lookup(node.path.slice(0, node.path.lastIndexOf('/')) || '/');
              node.error = '';
              if (node.loaded && node.directory) {
                var through = node.loadedThrough || 0;
                await load(node, false, true);
                while (node.nextStart >= 0 && node.nextStart <= through) await load(node, true);
              }
            } catch (error) { node.error = String(error); }
          }
          if (!disposed) listeners.forEach(function (fn) { fn(); });
        } while (pending && !disposed);
      } finally { refreshing = false; }
    }
    var unsubscribe = subscribe(function (event) {
      if (event && event.kind === 'home-renamed') loadHome(true).then(placeHome);
      refresh().catch(function () {});
    });
    function opened() {
      return Array.from(nodes.values()).filter(function (n) { return n.directory && n.loaded && n.ref; }).map(function (n) { return n.ref; });
    }
    trees.add(opened);
    return {
      root: function () { return root; }, home: function () { return home; }, refresh: refresh,
      lookup: lookup, load: load, resolve: function (_, path) { return nodes.get(path); },
      pathOf: function (node) { return node.path; }, displayPath: function (node) { return node.path; },
      listEntries: function (node) { var content = contentOf(node); return (content ? [content] : []).concat(node.children || []); },
      realOf: function (node) { return node; }, childNamed: function (node, name) { return name === '@content' ? contentOf(node) : (node.children || []).find(function (child) { return child.name === name; }); },
      sameNode: function (a, b) { return !!a && !!b && a.path === b.path && (!(a.ref || b.ref) || sameRef(a.ref, b.ref)); },
      kindOf: function (node) { return node.directory ? 'dir' : 'file'; },
      size: function (node) { return node.size; }, onChange: function (fn) { listeners.add(fn); return function () { listeners.delete(fn); }; },
      destroy: function () { disposed = true; unsubscribe(); listeners.clear(); trees.delete(opened); watchFolders(); }
    };
  }
  function commands() {
    return dinahWorkbench.launchers().map(function (tool) { return {name:tool.id,description:tool.description || tool.title,usage:tool.id}; }).concat([{name:'applets',description:'Discover applet actions and invoke them through the shared SDK.',usage:'applets [list | actions [id] | invoke <action> <json> | run <action> <json> | status <id> | cancel <id>]'}]);
  }
  function executionPathOf(records, id) {
    var record = id && records.find(function (entry) { return entry.id === id; });
    if (!record) return '';
    if (record.executionPath) return record.executionPath;
    var primary = (record.sources || []).find(function (source) { return source.kind === 'local-folder' && (!record.primarySourceId || source.id === record.primarySourceId); });
    return primary ? primary.path : '';
  }
  async function invoke(name, args, stillCurrent, context) {
    if (name !== 'applets') {
      var tool = dinahWorkbench.launchers().find(function (entry) { return entry.id === name; });
      if (!tool) throw new Error('Unknown command: ' + name);
      if (args.length) throw new Error('Usage: ' + name);
      if (!stillCurrent || stillCurrent()) await tool.run();
      return '';
    }
    var catalog = await dinahAppletSDK.catalog(), operation = args[0] || 'list';
    if (stillCurrent && !stillCurrent()) return '';
    if (operation === 'list') return (catalog || []).map(function (entry) { return entry.id + '  ' + entry.title; }).join('\n');
    if (operation === 'actions') return (catalog || []).filter(function (entry) { return !args[1] || entry.id === args[1]; }).flatMap(function (entry) { return (entry.actions || []).map(function (action) { return JSON.stringify(action); }); }).join('\n');
    var state = await app().State(), provider = (catalog || []).find(function (entry) { return (entry.actions || []).some(function (action) { return action.ref === args[1]; }); });
    var records = state.workspaceRecords || [], recordId = state.workspaceId || '';
    if (context) {
      if (context.ref) recordId = context.ref.workspaceId;
      else {
        var alias = (context.path || '').split('/')[2], owner = records.find(function (entry) { return entry.mountAlias === alias; });
        recordId = (context.path || '').startsWith('/workspaces/') && owner ? owner.id : '';
      }
    }
    /* An action runs for a workspace's folder on the computer, which is what
       the other windows give it, so a workspace found by its ID is given by
       its folder; one with no folder of its own is no workspace here. */
    var workspace = context || recordId ? executionPathOf(records, recordId) : state.workspace || '';
    var sdk = dinahAppletSDK.forApplet(provider ? provider.id : 'dinah.terminal', workspace);
    if (stillCurrent && !stillCurrent()) return '';
    if (operation === 'invoke' && args.length === 3 && provider) return JSON.stringify(await sdk.invokeJSON(args[1], args[2]));
    /* run waits for the action to end and gives its output, for a script
       that needs the answer; a failed action is an error. */
    if (operation === 'run' && args.length === 3 && provider) {
      var started = await sdk.invokeJSON(args[1], args[2]);
      var ended = started.status === 'done' ? started : await sdk.wait(started.id, { get aborted() { return !!stillCurrent && !stillCurrent(); } });
      return ended.outputJSON || '{}';
    }
    if (operation === 'status' && args.length === 2) return JSON.stringify(await sdk.status(args[1]));
    if (operation === 'cancel' && args.length === 2) { await sdk.cancel(args[1]); return 'Cancelled.'; }
    throw new Error('Usage: applets [list | actions [id] | invoke <action> <json> | run <action> <json> | status <id> | cancel <id>]');
  }
  /* What a folder takes as a document's home, for Save As: a file by name in
     a folder on the computer that belongs to a workspace or to the home
     folder. The reasons say where to go, since the person has done nothing
     wrong. */
  function documentHomes() {
    return function (dir) {
      if (!dir.ref) return { why: 'Read-only. Open a workspace, or your home folder, to save the document in one of its folders.' };
      if (dir.writable) return { kind: 'file' };
      // The workspace itself is the list of what is mounted in it, as a
      // workspace is in an editor, and takes no file of its own.
      if (dir.kind === 'workspace') return { why: 'Read-only. Open one of the workspace’s folders to save the document there.' };
      return { why: 'Read-only. A document is saved in a folder that can be written.' };
    };
  }

  window.dinahResources = { documentHomes: documentHomes, glyphName: glyphName, open: open, openFiles: openFiles, openTerminal: openTerminal, currentPath: currentPath, createTree: createTree, decorate: decorate, readOnly: readOnly, readOnlyMark: readOnlyMark, takesFolder: takesFolder, newFolder: newFolder, askName: askName, renames: renames, renameFile: renameFile, upload: upload, chooseAndUpload: chooseAndUpload, copyPath: copyPath, uploadTarget: uploadTarget, homePath: loadHome, has: has, verify: verify, sameRef: sameRef, compatibleRef: compatibleRef, subscribe: subscribe, commands: commands, invoke: invoke };
})();
