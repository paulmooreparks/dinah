/* The Open and Save As dialog: the Files browser over Dinah's file
   system in a dialog, with a field for a path (Open) or a name (Save As), and
   a button for the computer's own files. It follows pcFileBrowser.pick from
   Paul Parks's site (vendor/parks-files/NOTICE.txt), adapted to the
   asynchronous file system.

     dinahFilePicker.pick({
       mode       'open' or 'save'
       title, okLabel
       start      the folder to start in (default: the current workspace)
       name       Save As: the file name to suggest
       saveKind(dir)  Save As: a promise of { kind: 'file' } for a folder
                  that takes a new document, or { why } for one that does
                  not
       any        Open: any file may be chosen, not only a document the
                  Editor can open
       commit(choice)  does what was chosen, as a promise; false keeps the
                  dialog open, and an error is shown in it
     })

   The choices commit receives:
     { node }              Open: an editable file
     { reference }         Open: a path on the computer, typed into the field
     { computer: true }    Open: From Your Computer…
                           (Open's choices other than node also have dir,
                           the folder shown)
     { dir, name }         Save As: a new file named name in dir
     { computer: name }    Save As: To Your Computer…, suggesting name

   pick resolves to true once a choice is committed, or false when the dialog
   is cancelled. */
(function () {
  'use strict';
  var count = 0, SIZE_KEY = 'dinah-picker-size';
  /* The size of Open and Save As and the width of their folders, kept in
     the UI state the windows' placements are kept in. */
  function remembered() {
    var value = window.dinahUIState ? window.dinahUIState.get(SIZE_KEY, {}) : {};
    return value && typeof value === 'object' ? value : {};
  }
  function remember(change) {
    if (!window.dinahUIState) return;
    var next = Object.assign({}, remembered(), change);
    Object.keys(next).forEach(function (k) { if (typeof next[k] !== 'number') delete next[k]; });
    window.dinahUIState.set(SIZE_KEY, next);
  }
  function editable(node) { return !!node && !node.directory && (node.actions || []).includes('edit'); }
  function folderOf(node) { return node && node.directory; }

  function pick(o) {
    o = o || {};
    var FB = window.pcFileBrowser, F = dinahResources.createTree(), save = o.mode === 'save', id = 'dinah-pick-' + (++count);
    // Open takes a document the Editor can open, or with any, any file.
    var accept = o.any ? function (n) { return !!n && !n.directory; } : editable;
    var dlg = document.createElement('dialog');
    dlg.className = 'dialog fb-picker dinah-picker dinah-files';
    dlg.setAttribute('aria-labelledby', id + '-title');
    dlg.innerHTML =
      '<form method="dialog" class="fb-pick-form">' +
        '<h2 class="dialog-title" id="' + id + '-title"></h2>' +
        '<div class="dialog-body fb-pick-body">' +
          '<div class="fb-pick-bar">' +
            '<button type="button" class="icon-btn" data-pick="up" aria-label="Up One Folder" title="Up One Folder"><svg viewBox="0 0 16 16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M8 13V3.5"/><path d="M4 7.5l4-4 4 4"/></svg></button>' +
            '<button type="button" class="icon-btn" data-pick="home" aria-label="Your Home Folder" title="Your Home Folder"><span class="glyph" style="--glyph:var(--glyph-home)" aria-hidden="true"></span></button>' +
            '<button type="button" class="icon-btn" data-pick="workspace" aria-label="The Workspace" title="The Workspace"><span class="glyph" style="--glyph:var(--glyph-folder)" aria-hidden="true"></span></button>' +
            FB.markup.pathBar() +
            '<button type="button" class="btn btn-sm" data-pick="folder">New Folder…</button>' +
          '</div>' +
          '<div class="fb-pick-browse">' + FB.markup.body({}) + '</div>' +
          '<div class="form-group fb-pick-field">' +
            '<label class="form-label" for="' + id + '-input" data-pick="label"></label>' +
            '<input class="form-input" id="' + id + '-input" data-pick="input" autocomplete="off" spellcheck="false">' +
            '<p class="form-help fm-readonly-line" data-pick="readonly" hidden></p>' +
          '</div>' +
          '<p class="form-error" data-pick="error" role="alert" hidden></p>' +
        '</div>' +
        '<div class="dialog-actions fb-pick-actions">' +
          '<button type="button" class="btn" data-pick="computer">' + (save ? 'To Your Computer…' : 'From Your Computer…') + '</button>' +
          '<span class="fb-pick-spacer"></span>' +
          '<button type="button" class="btn" data-pick="cancel">Cancel</button>' +
          '<button type="submit" class="btn btn-primary" data-pick="ok"></button>' +
        '</div>' +
      '</form>';
    var q = function (name) { return dlg.querySelector('[data-pick="' + name + '"]'); };
    var input = q('input'), error = q('error'), ok = q('ok'), computer = q('computer'), label = q('label'), readonly = q('readonly');
    dlg.querySelector('.dialog-title').textContent = o.title || (save ? 'Save As' : 'Open');
    var b = null, done = null, working = false, kind = null, why = '', fileName = o.name || '', kindRequest = 0, home = '/workspaces';
    /* Open shows the chosen entry's path in the field. */
    var chosen = null;
    function shown(node) { return node.path; }

    function say(message) { error.textContent = message || ''; error.hidden = !message; }
    function sentence(err) {
      var text = String(err && err.message || err).replace(/^Error:\s*/, '').trim();
      text = text.charAt(0).toUpperCase() + text.slice(1);
      return /[.!?]$/.test(text) ? text : text + '.';
    }
    function busy(on) {
      working = on;
      [ok, computer, q('up'), q('home'), q('workspace'), q('folder')].forEach(function (button) { button.disabled = on; });
      if (!on) { drawKind(); drawFolder(); if (b) q('up').disabled = !b.canUp(); }
    }
    /* New Folder… makes a folder in the folder shown, when it is a folder on
       the computer, and goes into it. */
    function drawFolder() {
      var button = q('folder'), can = !!b && dinahResources.takesFolder(b.cwd());
      button.disabled = working || !can;
      button.title = can ? 'Make a new folder here' : 'A folder can be made only in a folder on the computer';
    }
    function finish(value) {
      if (!done) return;
      var resolve = done; done = null;
      if (dlg.open) dlg.close();
      if (b) b.destroy();
      F.destroy();
      dlg.remove();
      resolve(value);
    }
    /* Hands a choice to the host, and closes the dialog when the host has
       done it; a failure is shown here, so the person can choose again. */
    function commit(choice) {
      if (working) return;
      say(''); busy(true);
      Promise.resolve().then(function () { return o.commit(choice); }).then(function (closed) {
        busy(false);
        if (closed !== false) finish(true);
      }, function (err) {
        busy(false);
        say(sentence(err));
      });
    }

    /* Save As asks the host what the folder shown takes: a file by name, or
       nothing, and changes the field and the buttons. A folder that takes
       nothing is read-only: the name field is disabled,
       a line under it says so with the read-only mark, as the Finder's
       status bar does, and To Your Computer… is the primary button. That
       is a state of the folder, not an error of the person's, so it is
       never shown in red. */
    function drawKind() {
      if (!save) {
        label.textContent = 'Path';
        ok.textContent = o.okLabel || 'Open';
        return;
      }
      label.textContent = 'File Name';
      ok.textContent = o.okLabel || 'Save';
      ok.disabled = working || !kind;
      input.disabled = !working && !kind && !!why;
      computer.classList.toggle('btn-primary', !kind);
      readonly.hidden = working || !!kind || !why;
      if (!readonly.hidden) { readonly.replaceChildren(dinahResources.readOnlyMark(), document.createTextNode(why)); }
      if (!working && kind) say('');
    }
    function folderState() {
      if (!save || !b) return;
      var dir = b.cwd(), request = ++kindRequest;
      if (kind === 'file') fileName = input.value;
      kind = null; why = ''; drawKind();
      Promise.resolve().then(function () { return o.saveKind(dir); }).then(function (answer) {
        if (request !== kindRequest || !done) return;
        kind = answer && answer.kind || null; why = answer && answer.why || 'Read-only. Open a folder to save the document in.';
        input.value = fileName;
        drawKind();
      }, function (err) {
        if (request !== kindRequest || !done) return;
        why = sentence(err); drawKind();
      });
    }

    /* A typed path inside Dinah's file system starts with / or ~, or
       is relative to the folder shown; a path on the computer goes to the
       host to find. */
    function external(value) { return /^[A-Za-z]:[\\/]/.test(value) || value.indexOf('\\\\') === 0; }
    function absolute(value) {
      if (value === '~' || value.indexOf('~/') === 0) return F.home().path + value.slice(1);
      if (value[0] === '/') return value;
      return b.cwd().path.replace(/\/$/, '') + '/' + value;
    }
    function normal(path) {
      var out = [];
      path.split('/').forEach(function (part) { if (part === '..') out.pop(); else if (part && part !== '.') out.push(part); });
      return '/' + out.join('/');
    }
    function submit() {
      if (working || !b) return;
      var value = input.value.trim();
      if (!save) {
        var selected = b.selected();
        if (chosen && value === shown(chosen)) return commit({ node: chosen });
        if (!value) {
          if (folderOf(selected)) { b.go(selected); return; }
          say('Choose a file.'); return;
        }
        if (external(value)) return commit({ reference: value, dir: b.cwd() });
        busy(true);
        F.lookup(normal(absolute(value))).then(function (node) {
          busy(false);
          if (folderOf(node) && !accept(F.childNamed(node, '@content'))) { input.value = ''; return b.go(node); }
          if (folderOf(node)) node = F.childNamed(node, '@content');
          if (!accept(node)) { say((node.title || node.name) + (o.any ? ' cannot be chosen.' : ' is not a document the Editor can open.')); return; }
          commit({ node: node });
        }, function () { busy(false); say('There is nothing at ' + value + '.'); });
        return;
      }
      if (!kind) return;
      if (!value) { say('Give the file a name.'); input.focus(); return; }
      if (/[\\/]/.test(value)) { say('Choose the folder above, and give the file\'s name alone.'); return; }
      if (/^\.|^\s|\s$|[:*?"<>|]/.test(value)) { say(value + ' is not a name a file can have here.'); return; }
      var existing = F.childNamed(b.cwd(), value);
      if (existing) { say(value + ' already exists here. Choose another name.'); return; }
      commit({ dir: b.cwd(), name: value });
    }

    return new Promise(function (resolve) {
      done = resolve;
      document.body.appendChild(dlg);
      drawKind();
      busy(true);
      if (save) input.value = fileName;
      else input.placeholder = 'Choose a file, or type a path such as ~/notes.md';
      dlg.addEventListener('submit', function (e) { e.preventDefault(); submit(); });
      dlg.addEventListener('close', function () { finish(false); });
      dlg.addEventListener('cancel', function (e) { if (working) e.preventDefault(); });
      input.addEventListener('input', function () { if (!save || kind) say(''); });
      dlg.addEventListener('click', function (e) {
        var t = e.target.closest('[data-pick]');
        if (!t || !b || t.disabled) return;
        switch (t.getAttribute('data-pick')) {
          case 'cancel': if (!working) finish(false); break;
          case 'up': b.up(); break;
          case 'home': b.go(F.home()); break;
          case 'workspace': b.goPath(home); break;
          case 'folder':
            dinahResources.newFolder(F, b.cwd()).then(function (node) { if (node && done) { b.refresh(); b.go(node); } }).catch(function (err) { say(sentence(err)); });
            break;
          case 'computer':
            if (!save) { commit({ computer: true, dir: b.cwd() }); break; }
            var name = kind === 'file' ? input.value.trim() : fileName;
            commit({ computer: name || o.name || 'document.md' });
            break;
        }
      });
      /* The dialog is resized from its corner, since a deep tree needs room;
         the size it was left at is where the next one opens. */
      var size = remembered(), opened = null;
      if (size.width) dlg.style.width = size.width + 'px';
      if (size.height) dlg.style.height = size.height + 'px';
      /* ResizeObserver reports the size the dialog opens at first; a later
         size is one the person dragged it to, and is kept. */
      var sized = new ResizeObserver(function () {
        var now = [Math.round(dlg.offsetWidth), Math.round(dlg.offsetHeight)];
        if (!opened) { opened = now; return; }
        if (now[0] && now[1] && (now[0] !== opened[0] || now[1] !== opened[1])) remember({ width: now[0], height: now[1] });
      });
      dlg.addEventListener('close', function () { sized.disconnect(); });
      dlg.showModal();
      sized.observe(dlg);
      Promise.resolve(o.start || dinahResources.currentPath()).then(function (start) {
        home = /^\/workspaces\/[^/]+/.exec(start || '') ? /^\/workspaces\/[^/]+/.exec(start)[0] : '/workspaces';
        return F.lookup(start || '/workspaces').catch(function () { return F.lookup(home); }).catch(function () { return F.lookup('/workspaces'); });
      }).then(function (node) {
        return F.load(node).then(function () { return node; });
      }).then(function (node) {
        if (!done) return;
        b = FB.create(dlg, F, {
          start: node.path, rowLinks: false,
          formatDate: function (d) { return dinahRegional.format(d, 'date'); },
          /* The folders' width is kept for the next Open or Save As, as the
             dialog's own size is. */
          treeWidth: remembered().tree,
          onResize: function (width) { remember({ tree: typeof width === 'number' ? Math.round(width) : undefined }); },
          filter: save ? null : accept,
          say: function (message) { if (message) say(message); },
          /* The browser draws before it is handed back, so the first redraw
             is marked for the starting folder. */
          onRender: function () { dinahResources.decorate(dlg, F, b ? b.cwd() : node); },
          onOpen: function (n) {
            if (n.directory) { b.go(n); b.focusList(); return; }
            if (!save) { if (accept(n)) { chosen = n; input.value = shown(n); commit({ node: n }); } return; }
            if (kind === 'file' && !n.directory) { input.value = n.name; say(n.name + ' already exists here. Choose another name.'); }
          },
          onSelect: function (n) {
            if (!n || n.directory) return;
            if (!save) { chosen = n; input.value = shown(n); say(''); return; }
            if (kind === 'file') input.value = n.name;
          },
          onNavigate: function () { if (!save) { chosen = null; input.value = ''; } folderState(); drawFolder(); q('up').disabled = working || !b.canUp(); }
        });
        busy(false);
        folderState();
        q('up').disabled = !b.canUp();
        if (save) { input.focus(); var dot = input.value.lastIndexOf('.'); input.setSelectionRange(0, dot > 0 ? dot : input.value.length); }
        else b.focusList();
        /* A suggested name may arrive after the dialog opens (a judgement
           agent's): it fills the field only while the person has not typed a
           name of their own, and changes nothing else. */
        if (save && o.suggest) {
          Promise.resolve(o.suggest).then(function (name) {
            if (!name || !dlg.isConnected) return;
            if (!input.value.trim() || input.value.trim() === fileName.trim()) {
              fileName = name;
              input.value = name;
            }
          }).catch(function () {});
        }
      }).catch(function (err) { busy(false); say(sentence(err)); });
    });
  }

  window.dinahFilePicker = { pick: pick };
})();
