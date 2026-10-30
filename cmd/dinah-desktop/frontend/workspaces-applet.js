/* Workspaces owns the workspace list, selection controls, import and export.
   The shell supplies the selected path and chat updates. */
(function () {
  'use strict';

  function create(adapter) {
    const { app, ask, toast, fail, productName, drawSessions, drawGoTo, setContext, show, refreshSessions } = adapter;
    const $ = id => document.getElementById(id);
    let workspaceId = '', workspaceRecords = [], recentWorkspaces = [], openWorkspaces = [];
    let workspaceBrowser = null;

    /* ---- Workspaces --------------------------------------------------------
       The folder button at the top of the sidebar opens the workspace menu:
       the recent workspaces and New Workspace. The menu bar's Recent
       Workspaces lists the same folders. */
    function base(p) { return p ? p.split(/[\\/]/).filter(Boolean).pop() : ''; }
    function workspaceIdentityPath(path) {
      var clean = path || '';
      var windowsPath = /^(?:[a-z]:(?:[\\/]|$)|\\\\|\/\/)/i.test(clean);
      if (windowsPath) clean = clean.replace(/\\/g, '/');
      clean = clean.replace(/\/+$/, '') || (clean ? '/' : '');
      return windowsPath ? clean.toLowerCase() : clean;
    }
    function workspaceRecord(path) {
      var normalized = workspaceIdentityPath(path);
      return workspaceRecords.find(function (record) {
        if (record.executionPath) return workspaceIdentityPath(record.executionPath) === normalized;
        var sources = record.sources || [], primary = sources.find(function (source) { return source.id === record.primarySourceId; }) || sources[0];
        return primary && primary.kind === 'local-folder' && workspaceIdentityPath(primary.path) === normalized;
      });
    }
    function workspaceName(path) {
      var record = workspaceRecord(path);
      return record && record.name || base(path);
    }
    function workspaceLabel(path) {
      var name = workspaceName(path);
      var duplicates = workspaceRecords.filter(function (record) { return record.name.toLowerCase() === name.toLowerCase(); }).length > 1;
      var record = workspaceRecord(path);
      var label = duplicates ? name + ' (' + (record && record.executionPath ? record.mountAlias || record.id : path) + ')' : name;
      return record && missingWorkspaces.indexOf(record.id) >= 0 ? label + ' (Not on This Computer Yet)' : label;
    }
    // The workspaces whose main folder is not on this computer, by ID.
    var missingWorkspaces = [];

    /* ---- Taskbar stacks ----------------------------------------------------
       The taskbar stacks each workspace's windows under its name, with the
       folder glyph the status area gives a workspace, the chosen workspace's
       as well as the others' (docs/plans/taskbar-stacks.md). The group
       key is the workspace's ID, so renaming it never splits its stack. PUDL
       takes keys of letters, digits, underscore and hyphen, which the IDs
       the engine makes already are; any other ID becomes "ws-" and a 64-bit hash
       of it, the same for the same ID on every load. */
    function groupKey(id) {
      if (/^[A-Za-z0-9_-]+$/.test(id)) return id;
      function fnv(seed) {
        var h = seed >>> 0;
        for (var i = 0; i < id.length; i++) { h ^= id.charCodeAt(i); h = Math.imul(h, 16777619) >>> 0; }
        return ('0000000' + h.toString(16)).slice(-8);
      }
      return 'ws-' + fnv(2166136261) + fnv(84696351);
    }
    function recordPath(record) {
      if (record.executionPath) return record.executionPath;
      var sources = record.sources || [], primary = sources.find(function (source) { return source.id === record.primarySourceId; }) || sources[0];
      return primary && primary.kind === 'local-folder' ? primary.path : '';
    }
    // A window's workspace: a record's ID, its folder, or { mount } for a
    // path in the workspace catalog (/workspaces/<alias>/...).
    function windowGroup(where) {
      var record = null;
      if (where && typeof where === 'object') {
        var mount = String(where.mount || ''), alias = mount.split('/')[2];
        if (mount.indexOf('/workspaces/') === 0 && alias) record = workspaceRecords.find(function (r) { return r.mountAlias === alias; });
      } else if (where) {
        record = workspaceRecords.find(function (r) { return r.id === where; }) || workspaceRecord(where);
      }
      if (!record || !record.id) return null;
      var path = recordPath(record);
      return { key: groupKey(record.id), label: path ? workspaceLabel(path) : record.name || record.id, glyph: 'folder' };
    }
    function drawGroups() {
      if (window.dinahWorkbench) window.dinahWorkbench.regroup();
    }
    if (window.dinahWorkbench) window.dinahWorkbench.setGrouping(windowGroup);
    function refreshWorkspaceNames() {
      var record = workspaceRecord(adapter.workspace());
      workspaceId = record ? record.id : '';
      drawWorkspaceLabels();
      drawSessions();
      adapter.refreshChatTitles();
      if ($('go-to').open) drawGoTo();
      if (window.dinahDocuments) window.dinahDocuments.reloadRecent();
      window.dinahCommands.changed();
    }
    /* Whether a browser group is open is remembered per workspace. */
    function sectionKey(id) { return 'dinah-section:' + JSON.stringify([adapter.workspace(), id]); }
    function sectionOpen(id, closedByDefault) {
      try {
        var saved = window.dinahUIState ? window.dinahUIState.get(sectionKey(id)) : localStorage.getItem(sectionKey(id));
        return saved ? saved !== 'closed' : !closedByDefault;
      } catch (err) { return !closedByDefault; }
    }
    function saveSection(id, open) {
      if (window.dinahUIState) window.dinahUIState.set(sectionKey(id), open ? 'open' : 'closed');
      else try { localStorage.setItem(sectionKey(id), open ? 'open' : 'closed'); } catch (err) { /* Browser previews can disable storage. */ }
    }
    window.dinahBrowserSections = { open: sectionOpen, save: saveSection };
    /* browserSection adds a group of rows to a browser pane: its name, a
       count and, for a group that stands for a place, where it is stored. */
    function browserSection(host, id, name, rows, location) {
      // A part of another row's entry is not counted as an item of its own.
      var count = rows.filter(function (row) { return !row.classList.contains('browser-row-part'); }).length;
      var g = window.dinahBrowserList.group({ id: id, title: name, count: count, location: location, open: sectionOpen(id), onToggle: function (open) { saveSection(id, open); } });
      rows.forEach(function (row) { g.body.appendChild(row); });
      host.appendChild(g.el);
    }
    /* The Workspace pane's own groups are written in the page. */
    function wireWorkspaceGroups() {
      document.querySelectorAll('#sidebar > .browser-group[data-section], #sidebar > .ws-frozen > .browser-group[data-section]').forEach(function (section) {
        var id = section.dataset.section;
        window.dinahBrowserList.enhance(section, { open: sectionOpen(id), onToggle: function (open) { saveSection(id, open); } });
      });
    }

    function drawWorkspaces(st) {
      adapter.setWorkspace(st.workspace || '');
      workspaceId = st.workspaceId || '';
      if (Array.isArray(st.workspaceRecords)) workspaceRecords = st.workspaceRecords;
      if (Array.isArray(st.missingWorkspaces)) missingWorkspaces = st.missingWorkspaces;
      adapter.setPackageSources(st.packageSources || 0);
      recentWorkspaces = (st.recent || []).slice();
      workspaceRecords.forEach(function (record) { if (record.executionPath && !recentWorkspaces.some(function (path) { return workspaceIdentityPath(path) === workspaceIdentityPath(record.executionPath); })) recentWorkspaces.push(record.executionPath); });
      if (window.dinahAppletSDK) window.dinahAppletSDK.setWorkspace(adapter.workspace());
      wireWorkspaceGroups();
      openWorkspaces = st.workspaces || (adapter.workspace() ? [adapter.workspace()] : []);
      drawWorkspaceLabels();
      // New Document is there to use only while a workspace is open.
      window.dinahCommands.changed();
      document.dispatchEvent(new Event('dinah:workspace-selected'));
    }
    function drawWorkspaceLabels() {
      if (window.dinahWorkspaceBrowser) window.dinahWorkspaceBrowser.setRecords(workspaceRecords);
      if (window.dinahWorkspaceBrowser && !workspaceBrowser) {
        workspaceBrowser = window.dinahWorkspaceBrowser.mount({ api: app, ask: ask, fail: fail, changed: function (record) {
          workspaceRecords = workspaceRecords.map(function (existing) { return existing.id === record.id ? record : existing; });
          refreshWorkspaceNames();
        } });
        window.addEventListener('pagehide', function () { workspaceBrowser.destroy(); });
      }
      if (workspaceBrowser) workspaceBrowser.setWorkspace(workspaceRecord(adapter.workspace()));
      drawGroups();
      $('status-workspace-name').textContent = adapter.workspace() ? workspaceLabel(adapter.workspace()) : 'Workspace';
      $('status-workspace').title = adapter.workspace() ? workspaceLabel(adapter.workspace()) : 'Show the Workspace';
      // The Workspace pane's chooser: one option a workspace, the current one
      // chosen, and a prompt while none is open.
      var chooser = $('ws-select');
      chooser.replaceChildren();
      if (!adapter.workspace()) {
        var prompt = new Option('Choose a Workspace', '', true, true);
        prompt.disabled = true;
        chooser.add(prompt);
      }
      recentWorkspaces.forEach(function (p) {
        var record = workspaceRecord(p);
        // Labelled as the status area labels it, which tells two of one name apart.
        var option = new Option(workspaceLabel(p), p, false, p === adapter.workspace());
        if (record) option.dataset.workspaceId = record.id;
        chooser.add(option);
      });
      chooser.title = adapter.workspace() ? workspaceLabel(adapter.workspace()) : 'Choose a workspace';
      var host = $('status-ws-recent');
      host.replaceChildren();
      recentWorkspaces.forEach(function (p) {
        var r = document.createElement('div');
        r.className = 'md-row';
        r.innerHTML = '<a class="md-item" href="#"><span></span><span class="md-meta"></span></a>';
        var a = r.firstChild;
        a.dataset.ws = p;
        var record = workspaceRecord(p);
        if (record) a.dataset.workspaceId = record.id;
        a.firstChild.textContent = workspaceName(p);
        a.lastChild.textContent = record && record.executionPath ? record.sources.length + (record.sources.length === 1 ? ' source' : ' sources') : p;
        if (p === adapter.workspace()) a.setAttribute('aria-current', 'true');
        host.appendChild(r);
      });
      if (!host.children.length) {
        var empty = document.createElement('p'); empty.className = 'menu-note'; empty.textContent = 'No recent workspaces.';
        $('status-ws-recent').appendChild(empty);
      }
      window.dinahCommands.changed();
      var menu = document.querySelector('[data-recent-menu]');
      if (menu) {
        menu.innerHTML = '';
        // Numbered so that the digit is the row's letter; a workspace that is
        // a folder has its path as the row's second line.
        recentWorkspaces.forEach(function (p, i) {
          var li = document.createElement('li');
          li.innerHTML = '<button type="button"></button>';
          var record = workspaceRecord(p);
          li.firstChild.textContent = (i < 9 ? (i + 1) + '. ' : '') + (record && record.executionPath ? workspaceLabel(p) : workspaceName(p));
          if (!(record && record.executionPath)) li.firstChild.dataset.menuDetail = p;
          li.firstChild.dataset.ws = p;
          if (record) li.firstChild.dataset.workspaceId = record.id;
          li.firstChild.setAttribute('aria-checked', String(p === adapter.workspace()));
          menu.appendChild(li);
        });
        if (!recentWorkspaces.length) menu.innerHTML = '<li><button type="button" disabled>None yet</button></li>';
        if (window.pudlMenubar && window.pudlMenubar.refresh) window.pudlMenubar.refresh();
      }
    }

    var catalogRefresh = 0;
    if (window.dinahResources) dinahResources.subscribe(function (event) {
      // A change on disk changes what is in a folder, not the workspaces.
      if (!event || event.kind === 'labels' || event.kind === 'disk-changed') return;
      var generation = ++catalogRefresh;
      Promise.resolve().then(function () { return app().State(); }).then(function (state) {
        if (generation === catalogRefresh) drawWorkspaces(state);
      }).catch(fail);
    });

    function switchTo(st) {
      drawWorkspaces(st);
      setContext(null);
      adapter.setDraftWorkspace(adapter.workspace());
      show(null);
      refreshSessions();
    }

    /* Opens a workspace, by its identity when it has one. When it cannot be
       opened, the chooser goes back to the workspace that is still open. */
    function chooseWorkspace(p, id) {
      if (p === adapter.workspace()) return;
      var missing = id && missingWorkspaces.indexOf(id) >= 0 && workspaceRecords.find(function (r) { return r.id === id; });
      if (missing) {
        offerMissingWorkspace(missing).then(function (opened) { if (!opened) drawWorkspaceLabels(); }).catch(function (err) { drawWorkspaceLabels(); fail(err); });
        return;
      }
      Promise.resolve().then(function () { return id && app().SelectWorkspace ? app().SelectWorkspace(id) : app().SetWorkspace(p); }).then(switchTo)
        .catch(function (err) { drawWorkspaceLabels(); fail(err); });
    }
    document.addEventListener('click', function (e) {
      var a = e.target.closest && e.target.closest('[data-ws]');
      if (!a) return;
      e.preventDefault();
      chooseWorkspace(a.dataset.ws, a.dataset.workspaceId);
    });
    $('ws-select').addEventListener('change', function () {
      var option = this.selectedOptions[0];
      if (option && option.value) chooseWorkspace(option.value, option.dataset.workspaceId);
    });

    /* Remove Workspace… forgets the workspace in front, after asking; its
       folders stay where they are. */
    function removeWorkspace() {
      var record = workspaceRecords.find(function (r) { return r.id === workspaceId; });
      if (!record || !app().RemoveWorkspace) return;
      ask('Remove "' + record.name + '"?',
        productName() + ' forgets this workspace: its sources, pins and place in Recent. Its folders and files are not touched, and New Workspace… on its main folder makes it a workspace again.',
        'Remove', true).then(function (yes) {
        if (!yes) return;
        return Promise.resolve(app().RemoveWorkspace(record.id)).then(function (st) {
          switchTo(st);
          toast('Removed the workspace "' + record.name + '".', 'positive');
        });
      }).catch(function (err) { drawWorkspaceLabels(); fail(err); });
    }
    /* Export Workspace… and Import Workspace… (docs/design/workspace-export.md):
       the workspace's structure goes to a file, and the file makes the same
       workspace on another computer, or adds what is missing to the one on
       its main folder. A local folder that is not on this computer is
       imported all the same and waits for the folder, unless the person asks
       for it to be created now. A source of a kind this build does not know
       is kept as it is, and the window says so. */
    function exportWorkspace() {
      var record = workspaceRecords.find(function (r) { return r.id === workspaceId; });
      if (!record || !app().ExportWorkspace) return;
      Promise.resolve(app().ExportWorkspace(record.id)).then(function (file) {
        if (file) toast('Exported "' + record.name + '" to ' + file + '. Import Workspace… in ' + productName() + ' on another computer reads it.', 'positive');
      }).catch(fail);
    }
    var importing = { plan: null, choices: {}, busy: false };
    function importWorkspace() {
      if (!app().ChooseWorkspaceImport || importing.busy) return;
      Promise.resolve(app().ChooseWorkspaceImport()).then(function (plan) {
        if (!plan || !plan.file) return;
        importing = { plan: plan, choices: {}, busy: false };
        drawWorkspaceImport();
        $('workspace-import').showModal();
        $('workspace-import-go').focus();
      }).catch(fail);
    }
    function importChoices(choices) {
      return Object.keys(choices).map(function (key) { return { key: key, path: choices[key].path || '', create: !!choices[key].create }; });
    }
    function importStatus(s, create) {
      if (s.kind === 'local-folder') return s.found ? ['Found', 'positive'] : create ? ['Created on Import', 'accent'] : ['Not on This Computer Yet', 'warn'];
      if (!s.checked) return ['Kept, Not Checked', ''];
      return s.problem ? [s.problem, 'warn'] : ['Available', 'positive'];
    }
    function drawWorkspaceImport() {
      var plan = importing.plan, choices = importing.choices;
      const el = window.dinahHelpers.node;
      $('workspace-import-summary').textContent = plan.existingId ? '"' + plan.existingName + '" already works in this main folder, so the import adds the sources and pins it does not have.'
        : 'This makes the workspace "' + plan.name + '" with these sources.';
      var list = $('workspace-import-sources'), waitingMain = false;
      list.replaceChildren();
      plan.sources.forEach(function (s) {
        var row = list.appendChild(el('li', 'import-source'));
        row.dataset.key = s.key;
        var head = row.appendChild(el('div', 'import-source-head'));
        var glyph = head.appendChild(el('span', 'glyph'));
        glyph.setAttribute('aria-hidden', 'true');
        glyph.style.setProperty('--glyph', 'var(--glyph-' + (s.kind === 'local-folder' ? 'folder' : 'link') + ')');
        head.appendChild(el('strong', 'import-source-name', s.name || s.reference || s.path || s.kind));
        if (s.main) head.appendChild(el('span', 'badge', 'Main Folder'));
        var actions = s.actions || [], create = !!(choices[s.key] && choices[s.key].create) && actions.indexOf('create-folder') >= 0;
        var status = importStatus(s, create);
        head.appendChild(el('span', 'badge' + (status[1] ? ' ' + status[1] : ''), status[0]));
        var where = s.kind === 'local-folder' ? s.path : s.site ? s.site + (s.reference ? ' · ' + s.reference : '') : '';
        if (where) row.appendChild(el('p', 'form-help import-source-where', where));
        if (s.kind !== 'local-folder' && !s.checked) row.appendChild(el('p', 'form-help', productName() + ' does not use this kind of source (' + s.kind + '). It is kept as it is, for a version that does.'));
        if (!actions.length) return;
        var controls = row.appendChild(el('div', 'hstack gap-3 align-center import-source-controls'));
        if (actions.indexOf('choose-folder') >= 0) {
          var choose = controls.appendChild(el('button', 'btn btn-sm', 'Choose…'));
          choose.type = 'button';
          choose.addEventListener('click', function () {
            if (importing.busy) return;
            Promise.resolve(app().ChooseImportFolder(s.path)).then(function (path) {
              if (!path) return;
              choices[s.key] = { path: path };
              return Promise.resolve(app().PlanWorkspaceImport(plan.file, importChoices(choices))).then(function (next) { importing.plan = next; drawWorkspaceImport(); });
            }).catch(fail);
          });
        }
        if (actions.indexOf('create-folder') < 0) return;
        if (s.main && !create) waitingMain = true;
        var label = controls.appendChild(el('label', 'check'));
        var box = label.appendChild(el('input'));
        box.type = 'checkbox'; box.checked = create;
        label.appendChild(document.createTextNode(' Create It Now' + (s.folders ? ', With ' + s.folders + (s.folders === 1 ? ' Folder' : ' Folders') + ' Inside' : '')));
        box.addEventListener('change', function () {
          choices[s.key] = Object.assign({}, choices[s.key], { create: box.checked });
          drawWorkspaceImport();
        });
        row.appendChild(el('p', 'form-help', create ? productName() + ' makes this folder and the folders recorded inside it. If OneDrive or another service is still bringing it, leave it for that service instead.'
          : 'It is shown, dimmed, until it appears here. Create the Folders… in its menu makes it later.'));
      });
      var pins = $('workspace-import-pins');
      pins.hidden = !plan.pins;
      pins.textContent = plan.pins + (plan.pins === 1 ? ' pin comes' : ' pins come') + ' with it.' +
        (plan.uncheckedPins ? ' ' + plan.uncheckedPins + (plan.uncheckedPins === 1 ? ' of them is' : ' of them are') + ' of a kind ' + productName() + ' does not use, and ' + (plan.uncheckedPins === 1 ? 'is' : 'are') + ' kept as ' + (plan.uncheckedPins === 1 ? 'it is.' : 'they are.') : '');
      $('workspace-import-waiting').hidden = !waitingMain;
      $('workspace-import-go').textContent = plan.existingId ? 'Add to the Workspace' : 'Import';
    }
    $('workspace-import-cancel').addEventListener('click', function () { $('workspace-import').close(); });
    $('workspace-import').addEventListener('cancel', function (event) { if (importing.busy) event.preventDefault(); });
    $('workspace-import-go').addEventListener('click', function () {
      var plan = importing.plan, go = this;
      if (!plan || importing.busy) return;
      importing.busy = true; go.disabled = true;
      Promise.resolve(app().ImportWorkspace(plan.file, importChoices(importing.choices))).then(function (result) {
        $('workspace-import').close();
        switchTo(result.state);
        var added = result.existing ? 'Added ' + result.sources + (result.sources === 1 ? ' source' : ' sources') + ' and ' + result.pins + (result.pins === 1 ? ' pin' : ' pins') + ' to "' + plan.existingName + '".' : 'Imported the workspace "' + plan.name + '".';
        if (result.waiting) added += ' ' + result.waiting + (result.waiting === 1 ? ' folder is' : ' folders are') + ' not on this computer yet.';
        toast(added, 'positive');
      }).catch(fail).finally(function () { importing.busy = false; go.disabled = false; });
    });
    /* A workspace whose main folder is not on this computer cannot be
       opened; choosing it offers to create the folder, or to wait for it. */
    function offerMissingWorkspace(record) {
      var main = record.sources.find(function (s) { return s.id === record.primarySourceId; });
      if (!main || !app().CreateWorkspaceFolders) return Promise.resolve(false);
      return ask('Create "' + record.name + '"\'s Folders?',
        'The main folder ' + main.path + ' is not on this computer yet. ' + productName() + ' can create it, with the folders recorded inside it, or the workspace can wait until it appears. If OneDrive or another service is still bringing the folder, wait for it.',
        'Create the Folders', false).then(function (yes) {
        if (!yes) return false;
        return Promise.resolve(app().CreateWorkspaceFolders(record.id, main.id)).then(function () { return app().SelectWorkspace(record.id); }).then(function (st) { switchTo(st); return true; });
      });
    }
    var renamingWorkspace = '', workspaceRenameBusy = false;
    var creatingWorkspace = false;
    function commandEnabled(id) {
      const methods = { 'rename-workspace': 'RenameWorkspace', 'remove-workspace': 'RemoveWorkspace', 'export-workspace': 'ExportWorkspace' };
      if (methods[id]) return !!workspaceId && !!app()?.[methods[id]];
      if (id === 'refresh-workspace') return !!adapter.workspace();
      if (id === 'import-workspace') return !!app()?.ChooseWorkspaceImport;
      if (id === 'new-workspace') return !creatingWorkspace && !!app()?.ChooseWorkspace;
      if (id === 'new-empty-workspace') return !creatingWorkspace && !!app()?.CreateWorkspace;
      return true;
    }
    // createWorkspace makes a workspace on a main folder the person chooses,
    // where its agents start and its skills are kept; with empty, one without
    // a main folder, whose folders are all added as sources.
    // Choosing a folder that is already a workspace's main folder selects it.
    function createWorkspace(empty) {
      var call = empty ? app().CreateWorkspace : app().ChooseWorkspace;
      if (creatingWorkspace || !call) return;
      creatingWorkspace = true;
      window.dinahCommands.changed();
      Promise.resolve().then(function () { return call(); }).then(switchTo).catch(fail).finally(function () {
        creatingWorkspace = false;
        window.dinahCommands.changed();
      });
    }
    function openWorkspaceRename() {
      var record = workspaceRecord(adapter.workspace());
      if (!record || !app().RenameWorkspace) return;
      renamingWorkspace = record.id;
      $('rename-workspace-name').value = record.name;
      $('rename-workspace-folder').textContent = record.executionPath ? record.sources.length + (record.sources.length === 1 ? ' attached source' : ' attached sources') : adapter.workspace();
      $('rename-workspace-error').hidden = true;
      $('rename-workspace').showModal();
      $('rename-workspace-name').focus(); $('rename-workspace-name').select();
    }
    $('rename-workspace-cancel').addEventListener('click', function () { $('rename-workspace').close(); });
    $('rename-workspace').addEventListener('cancel', function (event) { if (workspaceRenameBusy) event.preventDefault(); });
    $('rename-workspace-form').addEventListener('submit', function (event) {
      event.preventDefault();
      var name = $('rename-workspace-name').value.trim();
      if (!name || workspaceRenameBusy) return;
      workspaceRenameBusy = true;
      $('rename-workspace-error').hidden = true;
      ['name', 'save', 'cancel'].forEach(function (part) { $('rename-workspace-' + part).disabled = true; });
      Promise.resolve().then(function () { return app().RenameWorkspace(renamingWorkspace, name); }).then(function (record) {
        workspaceRecords = workspaceRecords.map(function (existing) { return existing.id === record.id ? record : existing; });
        refreshWorkspaceNames();
        $('rename-workspace').close();
      }).catch(function (error) {
        $('rename-workspace-error').textContent = String(error);
        $('rename-workspace-error').hidden = false;
      }).finally(function () {
        workspaceRenameBusy = false;
        ['name', 'save', 'cancel'].forEach(function (part) { $('rename-workspace-' + part).disabled = false; });
      });
    });
    function setRecords(records) { workspaceRecords = records; refreshWorkspaceNames(); }
    return { commandEnabled, workspaceLabel, refreshWorkspaceNames, browserSection, drawWorkspaces, removeWorkspace, exportWorkspace, importWorkspace, createWorkspace, openWorkspaceRename, setRecords, currentId: () => workspaceId, openPaths: () => openWorkspaces };
  }

  window.dinahWorkspaces = { create };
})();
