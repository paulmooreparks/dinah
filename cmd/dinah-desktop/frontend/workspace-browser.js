/* Workspace renders the shared virtual filesystem in its source sidebar. */
(function () {
  'use strict';
  const node = window.dinahHelpers.node;
  function button(text, action, className) {
    var element = node('button', className || 'btn btn-sm', text);
    element.type = 'button'; element.addEventListener('click', action); return element;
  }
  function address(ref) { return JSON.stringify([ref.sourceId, ref.path || '']); }
  var registry = [];
  window.dinahWorkspaceBrowser = {
    setRecords: function (records) {
      registry = records || [];
      document.dispatchEvent(new CustomEvent('dinah:workspace-resource-labels'));
    },
    records: function () { return registry.slice(); },
    describe: function (ref) {
      var owner = registry.find(function (record) { return record.id === ref.workspaceId; });
      var source = owner && owner.sources.find(function (source) { return source.id === ref.sourceId; });
      var location = source && source.path;
      return (owner && owner.name || ref.workspaceId) + ' · ' + (source && source.name || ref.sourceId) + (location ? ' · ' + location : '');
    },
    mount: function (options) {
      var $ = function (id) { return document.getElementById(id); };
      var api = options.api, ui = window.dinahUIState;
      var record = null, generation = 0, disposed = false;
      var model = window.dinahResources.createTree(), roots = [], selected = '', expanded = new Set();
      var sourceSignature = '', busy = false;
      var listeners = [];
      function listen(element, event, handler) { element.addEventListener(event, handler); listeners.push(function () { element.removeEventListener(event, handler); }); }
      var tree = $('workspace-source-tree');
      function stateKey() { return 'dinah-section:workspace-sources:' + record.id; }
      function persist() {
        if (!record || !ui || disposed) return;
        ui.set(stateKey(), JSON.stringify({ selected: selected, expanded: Array.from(expanded) }));
      }
      function permitted(item, action) { return (item.actions || []).includes(action); }
      function sourceFor(entry) { return record && record.sources.find(function (source) { return entry.ref && source.id === entry.ref.sourceId && entry.path === '/workspaces/' + record.mountAlias + '/' + source.mountAlias; }); }
      function keyFor(entry) { var source = sourceFor(entry); return address(source ? { sourceId: source.id, path: '' } : entry.ref); }
      function current(epoch) { return !disposed && epoch === generation; }
      function errorText(error) { return window.dinahHelpers.message(error, 'raw'); }
      /* A Markdown document opens in the Editor, in Preview, because a
         double-click or Open is reading; Edit opens it in Split. Everything
         else, diagrams among it, opens in the resource viewer, which draws
         it. */
      function isDocument(item) { return /\.(md|markdown)$/i.test(item.path || item.title || ''); }
      function open(item) {
        if (permitted(item, 'edit') && isDocument(item) && window.dinahEditor) return window.dinahEditor.openResource(item.ref, 'read').catch(options.fail);
        if (permitted(item, 'preview') || permitted(item, 'open-external') || permitted(item, 'reveal')) return window.dinahResources.open(item, function () { return !disposed; }).catch(options.fail);
        if (permitted(item, 'list')) { expanded.add(keyFor(item)); persist(); drawTree(); }
      }
      function select(item) {
        selected = keyFor(item);
        tree.querySelectorAll('[data-resource]').forEach(function (row) { row.setAttribute('aria-current', String(row.dataset.resource === selected)); });
        persist();
      }
      // The pin of a tree entry: its reference, its title as the tree shows it, and its kind.
      function pinOf(entry) {
        var root = sourceFor(entry);
        var ref = { workspaceId: entry.ref.workspaceId, sourceId: entry.ref.sourceId, path: root ? '' : entry.ref.path || '' };
        return { kind: 'resource', title: root ? root.name : entry.title, ref: ref, itemKind: entry.kind || '', directory: !!entry.directory };
      }
      // A pinned item of a source opens as it does from the tree.
      if (window.dinahPins) window.dinahPins.registerOpener('resource', function (pin) {
        return window.dinahResources.verify({ ref: pin.ref }).then(function (current) { return open(Object.assign({}, current, { ref: current.ref || pin.ref })); });
      });
      // An entry's children are listed again after files went into it.
      function reload(entry) {
        expanded.add(keyFor(entry)); persist();
        loadChildren(entry, false, true);
        model.refresh().then(function () { if (!disposed) drawTree(); }).catch(function () {});
      }
      /* A local folder that is not on this computer, and the folders its
         import recorded inside it, are shown dimmed until it is
         (docs/design/workspace-export.md). Create the Folders… makes it on
         the person's word, and Choose the Folder… points the source at a
         folder elsewhere. Either changes what the source lists, so the tree
         is read again from the workspace down, keeping what was open. */
      function rebuild(updated) {
        if (disposed || !updated || !updated.id) return;
        options.changed(updated);
        record = null; setWorkspace(updated);
      }
      function createFolders(entry) {
        var owner = record, folder = owner && owner.sources.find(function (s) { return s.id === entry.ref.sourceId; });
        if (!folder) return;
        var count = (owner.plannedFolders && owner.plannedFolders[folder.id] || []).length;
        var product = window.dinahProduct ? window.dinahProduct.name : '';
        options.ask('Create the Folders?', product + ' creates ' + folder.path + (count ? ' and the ' + count + (count === 1 ? ' folder' : ' folders') + ' recorded inside it' : '') +
          '. Folders that already exist are left as they are. If OneDrive or another service is still bringing this folder, leave it for that service instead.', 'Create the Folders', false).then(function (yes) {
          if (!yes || disposed) return;
          return Promise.resolve(api().CreateWorkspaceFolders(owner.id, folder.id)).then(rebuild);
        }).catch(options.fail);
      }
      function removeSource(source) {
        var id = record.id;
        options.ask('Remove This Source?', 'Remove "' + (source.name || source.path) + '" from this workspace? Its files will remain unchanged.', 'Remove Source', true).then(function (confirmed) {
          if (!confirmed || disposed) return;
          return api().DetachWorkspaceSource(id, source.id).then(function (updated) { if (!disposed) options.changed(updated); });
        }).catch(options.fail);
      }
      // The rows of an entry's context menu.
      function sourceMenu(entry) {
        var rows = [], item = entry, source = sourceFor(entry);
        function add(label, run) { rows.push({ label: label, run: run }); }
        // A folder still to come offers only what can be done without it, and
        // a source of a kind this build does not use can only be removed.
        if (item.missing || item.unavailable) {
          if (permitted(item, 'create-folders')) add('Create the Folders…', function () { createFolders(entry); });
          if (source && permitted(item, 'choose-folder')) add('Choose the Folder…', function () {
            Promise.resolve(api().RelocateWorkspaceSource(record.id, source.id)).then(rebuild).catch(options.fail);
          });
          // A folder that has arrived since is shown when the tree is read again.
          if (source && item.missing) add('Refresh Source', function () { rebuild(record); });
          if (source) rows.push(rows.length ? '-' : null, { label: 'Remove from Workspace…', danger: true, run: function () { removeSource(source); } });
          return rows.filter(Boolean);
        }
        if (permitted(item, 'preview')) add('Open', function () { open(item); });
        if (permitted(item, 'edit') && window.dinahEditor) add('Edit', function () { window.dinahEditor.openResource(item.ref).catch(options.fail); });
        if (permitted(item, 'open-external') || permitted(item, 'reveal')) add(permitted(item, 'reveal') ? 'Show in Folder' : 'Open Externally', function () { Promise.resolve().then(function () { return api().OpenWorkspaceResource(item.ref); }).catch(options.fail); });
        // A folder on the computer takes files from anywhere on it, and any
        // folder opens in Files.
        if (window.dinahResources.takesFolder(item)) add('Upload Files…', function () { window.dinahResources.chooseAndUpload(item).then(function (r) { if (r && !disposed) reload(entry); }).catch(options.fail); });
        if (permitted(item, 'list') && item.path) add('Open in Files', function () { window.dinahResources.openFiles(item.path, item.ref).catch(options.fail); });
        if (item.writable && item.ref) add('Copy Path', function () { window.dinahResources.copyPath(item).catch(options.fail); });
        if (!source && window.dinahResources.renames(item)) add('Rename…', function () { window.dinahResources.renameFile(item).catch(options.fail); });
        var pin = window.dinahPins && item.ref && window.dinahPins.command(pinOf(entry));
        if (pin) rows.push(pin);
        if (permitted(item, 'list')) add(source ? 'Refresh Source' : 'Refresh Children', function () { reload(entry); });
        if (!source) return rows;
        rows.push('-', { label: 'Remove from Workspace…', danger: true, run: function () { removeSource(source); } });
        return rows;
      }
      function renderEntry(entry, parent) {
        var item = entry, key = keyFor(entry), wrap = node('li', 'workspace-resource-node'), rootSource = sourceFor(entry);
        var titleText = rootSource ? rootSource.name : item.title;
        var title = node('a', 'workspace-resource-name');
        if (permitted(item, 'list')) title.setAttribute('aria-expanded', String(expanded.has(key)));
        var glyph = node('span', 'glyph'); glyph.style.setProperty('--glyph', 'var(--glyph-' + window.dinahResources.glyphName(item) + ')'); glyph.setAttribute('aria-hidden', 'true');
        title.append(glyph, node('span', 'fm-ti-label', titleText));
        // A folder that is not on this computer is dimmed, and its source
        // says so; so is a source of a kind this build does not use.
        if (item.missing || item.unavailable) {
          wrap.classList.add('workspace-resource-missing');
          if (rootSource) {
            var mark = node('span', 'badge warn', item.missing ? 'Not Created Yet' : 'Not Available Here');
            if (item.unavailable) mark.title = (window.dinahProduct ? window.dinahProduct.name : '') + ' does not use this kind of source (' + rootSource.kind + '). It is kept for a version that does.';
            title.appendChild(mark);
          }
        }
        title.href = '#'; title.addEventListener('click', function (event) { event.preventDefault(); select(item); });
        // A folder on the computer takes files dropped onto its row.
        if (window.dinahResources.takesFolder(item) && window.dinahResources.uploadTarget) window.dinahResources.uploadTarget(title, function () { return item; });
        title.dataset.resource = key; title.setAttribute('aria-current', String(selected === key));
        title.title = item.ref.path || rootSource && rootSource.path || item.title;
        title.addEventListener('dblclick', function () { open(item); });
        title.addEventListener('keydown', function (event) { if (event.key === 'Enter') { event.preventDefault(); select(item); open(item); } });
        wrap.appendChild(title); parent.appendChild(wrap);
        // Every entry has its context menu, since any of them can be pinned.
        var more = (rootSource || item.ref) && node('button');
        if (more) {
          more.type = 'button'; more.setAttribute('data-row-actions', '');
          wrap.addEventListener('pudl:contextmenu', function (event) {
            if (event.detail.count()) return;
            event.detail.add(sourceMenu(entry)); event.detail.mark(wrap);
          });
        }
        if (expanded.has(key) && permitted(item, 'list')) {
          var children = node('ul', 'workspace-resource-children'); wrap.appendChild(children);
          (entry.children || []).forEach(function (child) { renderEntry(child, children); });
          function message(content) { var row = node('li', 'workspace-resource-message'); row.appendChild(content); children.appendChild(row); }
          if (entry.loading) message(node('p', 'form-help', 'Loading…'));
          else if (entry.error) {
            message(node('p', 'form-help', 'Unavailable: ' + entry.error));
            message(button('Retry', function () { loadChildren(entry, entry.loaded && entry.nextStart >= 0, true); }));
          } else if (!entry.loaded) loadChildren(entry, false);
          else if (entry.nextStart >= 0) message(button('Load More', function () { loadChildren(entry, true); }));
          else if (!entry.children.length) message(node('p', 'form-help', 'No items.'));
        }
        if (more) wrap.appendChild(more);
      }
      function drawTree() {
        if (!record || disposed) return;
        var focused = tree.contains(document.activeElement) && document.activeElement.closest('[data-resource]');
        var focusedKey = focused && focused.dataset.resource;
        tree.replaceChildren();
        var list = node('ul', 'tree workspace-resource-tree');
        list.setAttribute('aria-label', 'Workspace Sources'); tree.appendChild(list);
        roots.forEach(function (entry) { renderEntry(entry, list); });
        list.addEventListener('pudl:tree-toggle', function (event) {
          var key = event.target.dataset.resource;
          if (event.detail.open) expanded.add(key); else expanded.delete(key);
          persist(); drawTree();
        });
        window.pudlTree.enhance(list);
        list.querySelectorAll('a[aria-expanded]').forEach(function (link) {
          var toggle = link.querySelector('.tree-toggle');
          toggle.classList.add('workspace-resource-expand'); toggle.removeAttribute('aria-hidden'); toggle.setAttribute('role', 'button');
          toggle.setAttribute('aria-label', (link.getAttribute('aria-expanded') === 'true' ? 'Collapse ' : 'Expand ') + link.querySelector('.fm-ti-label').textContent);
        });
        if (focusedKey) {
          var restored = Array.from(list.querySelectorAll('[data-resource]')).find(function (link) { return link.dataset.resource === focusedKey; });
          if (restored) { list.querySelectorAll('[tabindex="0"]').forEach(function (link) { link.tabIndex = -1; }); restored.tabIndex = 0; restored.focus(); }
        }
        if (!record.sources.length) tree.appendChild(node('p', 'form-help', 'Add a folder to this workspace.'));
      }
      function loadChildren(entry, more, force) {
        if (entry.loading) return;
        var epoch = generation;
        entry.error = '';
        model.load(entry, more, force).catch(function (error) { if (current(epoch)) entry.error = errorText(error); }).finally(function () {
          if (current(epoch)) drawTree();
        });
        // The initial render already contains the loading row on its next redraw.
        queueMicrotask(function () { if (current(epoch)) drawTree(); });
      }
      function setWorkspace(next) {
        var available = !!(next && next.mountAlias && api().ListWorkspaceMounts);
        $('workspace-sources').hidden = !available;
        if (!available) { record = null; ++generation; tree.replaceChildren(); return; }
        var signature = JSON.stringify(next.sources || []), changed = !record || record.id !== next.id;
        if (!changed && signature === sourceSignature) { record = next; return; }
        ++generation; record = next; sourceSignature = signature;
        if (changed) {
          roots = []; model.destroy(); model = window.dinahResources.createTree(); bindModel();
          var saved = {};
          try { saved = JSON.parse(ui && ui.get(stateKey(), '{}') || '{}'); } catch (_) { /* Invalid saved view state starts collapsed. */ }
          expanded = new Set(saved.expanded || []); selected = saved.selected || '';
        }
        var epoch = generation;
        tree.replaceChildren(node('p', 'form-help', 'Loading…'));
        model.lookup('/workspaces/' + record.mountAlias).then(function (workspace) { return model.load(workspace, false, true).then(function () { return workspace; }); }).then(function (workspace) {
          if (!current(epoch)) return;
          roots = workspace.children || []; drawTree();
        }).catch(function (error) { if (current(epoch)) tree.replaceChildren(node('p', 'form-help', 'Unavailable: ' + errorText(error))); });
        $('workspace-source-choose').disabled = !api().ChooseWorkspaceSource;
      }
      /* The model reports a change for each folder it loads, many at a
         start, so the tree is drawn for them once a frame. A person's own
         action draws it at once. */
      var changeKey = {};
      function bindModel() { model.onChange(function () {
        if (!record || disposed) return;
        window.dinahRender.enqueue(changeKey, tree, function () {
          if (!record || disposed) return;
          var workspace = model.resolve(null, '/workspaces/' + record.mountAlias);
          if (workspace) roots = workspace.children || [];
          drawTree();
        });
      }); }
      bindModel();
      /* Add Source… asks for a folder on the computer and adds it to the
         workspace; Cancel in the folder dialog adds nothing. */
      function addSource() {
        if (!record || disposed || busy) return;
        busy = true;
        var id = record.id;
        Promise.resolve().then(function () { return api().ChooseWorkspaceSource(id); }).then(function (updated) {
          if (!disposed && updated) options.changed(updated);
        }).catch(options.fail).finally(function () { busy = false; });
      }
      listen($('workspace-source-choose'), 'click', addSource);
      return { setWorkspace: setWorkspace, addSource: addSource, destroy: function () { disposed = true; window.dinahRender.cancel(changeKey); ++generation; model.destroy(); listeners.forEach(function (remove) { remove(); }); } };
    }
  };
})();
