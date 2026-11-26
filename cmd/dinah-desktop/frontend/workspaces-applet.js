/* Workspaces owns the workspace list, selection controls, import and export.
   The shell supplies the selected path and chat updates. */
(function () {
  'use strict';

  function create(adapter) {
    const { app, ask, toast, fail, productName, drawSessions, drawGoTo, setContext, show } = adapter;
    const $ = id => document.getElementById(id);
    let workspaceId = '', workspaceRecords = [], recentWorkspaces = [], openWorkspaces = [];
    let workspaceBrowser = null;
    var workspaceChoice = null, switchingWorkspace = false, lastChoiceState = null;

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

    /* Workbench combines the applet identity with the workspace's stable ID.
       This resolver supplies current labels without changing that identity. */
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
      return { key: record.id, label: path ? workspaceLabel(path) : record.name || record.id, glyph: 'folder' };
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
      var chosenPath = workspaceChoice ? workspaceChoice.path : adapter.workspace();
      chooser.replaceChildren();
      if (!chosenPath) {
        var prompt = new Option('Choose a Workspace', '', true, true);
        prompt.disabled = true;
        chooser.add(prompt);
      }
      recentWorkspaces.forEach(function (p) {
        var record = workspaceRecord(p);
        // Labelled as the status area labels it, which tells two of one name apart.
        var option = new Option(workspaceLabel(p), p, false, workspaceIdentityPath(p) === workspaceIdentityPath(chosenPath));
        if (record) option.dataset.workspaceId = record.id;
        chooser.add(option);
      });
      if (chosenPath && !Array.from(chooser.options).some(function (option) { return workspaceIdentityPath(option.value) === workspaceIdentityPath(chosenPath); })) chooser.add(new Option(workspaceLabel(chosenPath), chosenPath, true, true));
      chooser.title = chosenPath ? workspaceLabel(chosenPath) : 'Choose a workspace';
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
      // A change on disk, or where a source is kept, changes what is in a
      // folder, not the workspaces.
      if (!event || event.kind === 'labels' || event.kind === 'disk-changed' || event.kind === 'source-changed') return;
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
      drawSessions();
    }

    function sameChoice(a, b) {
      return !!a && !!b && (a.id && b.id ? a.id === b.id : workspaceIdentityPath(a.path) === workspaceIdentityPath(b.path));
    }
    function selectNextWorkspace() {
      if (switchingWorkspace || !workspaceChoice) return;
      var requested = workspaceChoice;
      switchingWorkspace = true;
      Promise.resolve().then(function () {
        var missing = requested.id && missingWorkspaces.indexOf(requested.id) >= 0 && workspaceRecords.find(function (r) { return r.id === requested.id; });
        return missing ? offerMissingWorkspace(missing) : requested.id && app().SelectWorkspace ? app().SelectWorkspace(requested.id) : app().SetWorkspace(requested.path);
      }).then(function (state) {
        if (state) lastChoiceState = state;
        if (!sameChoice(requested, workspaceChoice)) return;
        workspaceChoice = null;
        if (lastChoiceState) switchTo(lastChoiceState);
      }).catch(function (error) {
        if (sameChoice(requested, workspaceChoice)) workspaceChoice = null;
        if (!workspaceChoice && lastChoiceState) switchTo(lastChoiceState);
        fail(error);
      }).finally(function () {
        switchingWorkspace = false; ++catalogRefresh;
        if (workspaceChoice) selectNextWorkspace();
        else { lastChoiceState = null; drawWorkspaceLabels(); }
      });
    }
    /* Keep the latest choice while the current host mutation completes.
       A failed final choice returns to the last workspace actually opened. */
    function chooseWorkspace(p, id) {
      var requested = { path: p, id: id || '' };
      if (!switchingWorkspace && !workspaceChoice && sameChoice(requested, { path: adapter.workspace(), id: workspaceId })) return;
      workspaceChoice = requested; ++catalogRefresh;
      drawWorkspaceLabels();
      selectNextWorkspace();
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
        return Promise.resolve(app().RemoveWorkspace(record.id)).then(function (st) { switchTo(st); });
      }).catch(function (err) { drawWorkspaceLabels(); fail(err); });
    }
    /* Export Workspace… and Import Workspace… (docs/design/workspace-export.md):
       the workspace's structure goes to a file, and the file makes the same
       workspace on another computer, or adds what is missing to the one on
       its main folder. A local folder that is not on this computer is
       imported all the same and waits for the folder, unless the person asks
       for it to be created now. A source of a kind this build does not know
       is kept as it is, and the window says so. */
    /* Export Workspace… first shows the export window: a table of the
       workspace's chats, every one ticked, with their program, model, state
       and size; a table of their groups and teams, one row each, saying how
       many of the members here go and who is elsewhere; the running total
       beside the buttons; and a warning that the file contains their
       conversations. Export… then asks where to save the file, with the
       chats that are still ticked. Ticking a chat changes words in cells
       and never the height of a row, so nothing else in the window moves. */
    var exporting = { plan: null, ticked: {}, busy: false, rows: [] };
    function exportWorkspace() {
      var record = workspaceRecords.find(function (r) { return r.id === workspaceId; });
      if (!record || !app().PlanWorkspaceExport || exporting.busy) return;
      Promise.resolve(app().PlanWorkspaceExport(record.id)).then(function (plan) {
        if (!plan) return;
        var ticked = {};
        (plan.chats || []).forEach(function (c) { ticked[c.chatId] = true; });
        exporting = { plan: plan, ticked: ticked, busy: false, rows: [] };
        drawWorkspaceExport();
        $('workspace-export').showModal();
        $('workspace-export-body').scrollTop = 0;
        $('workspace-export-go').focus();
      }).catch(fail);
    }
    function megabytes(bytes) {
      var mb = bytes / 1048576;
      return (bytes > 0 && mb < 0.1 ? 0.1 : mb).toFixed(1) + ' MB';
    }
    function listWords(items) {
      return items.length < 2 ? items.join('') : items.slice(0, -1).join(', ') + ' and ' + items[items.length - 1];
    }
    /* transferCell adds a cell to a row of a transfer window's table, with
       the label the narrow layout shows before its value. */
    function transferCell(tr, text, className, label) {
      var td = tr.appendChild(window.dinahHelpers.node('td', className, text));
      td.dataset.label = label;
      return td;
    }
    function transferBadge(mark) {
      return window.dinahHelpers.node('span', 'badge' + (mark[1] ? ' ' + mark[1] : ''), mark[0]);
    }
    function drawWorkspaceExport() {
      var plan = exporting.plan, chats = plan.chats || [];
      const el = window.dinahHelpers.node;
      $('workspace-export-summary').textContent = 'This saves the workspace "' + plan.name + '" to a file that Import Workspace… reads on another computer.';
      var sources = (plan.sources || []).map(function (s) {
        return (s.kind === 'local-folder' ? (s.main ? 'the main folder ' : 'the folder ') : 'the source ') + s.name;
      });
      var counted = sources.length + (sources.length === 1 ? ' source' : ' sources') + (plan.pins ? ' and ' + plan.pins + (plan.pins === 1 ? ' pin' : ' pins') : '');
      $('workspace-export-sources').textContent = counted + (sources.length === 1 && !plan.pins ? ' goes' : ' go') + ' with it' + (sources.length ? ': ' + listWords(sources) + '.' : '.');
      var body = $('workspace-export-chats').tBodies[0];
      body.replaceChildren();
      chats.forEach(function (c) {
        var tr = body.appendChild(el('tr'));
        tr.dataset.chatId = c.chatId;
        var check = transferCell(tr, null, 'data-table-check', 'Export');
        var box = check.appendChild(el('input'));
        box.type = 'checkbox'; box.checked = !!exporting.ticked[c.chatId];
        box.setAttribute('aria-label', c.title);
        transferCell(tr, c.title, 'wrap', 'Chat');
        transferCell(tr, c.providerName + (c.model ? ' · ' + c.model : ''), 'muted', 'Program and Model');
        transferCell(tr, (c.open ? 'Open' : 'Closed') + (c.hidden ? ', Hidden' : ''), 'nowrap', 'State');
        transferCell(tr, megabytes(c.size), 'num nowrap', 'History');
        box.addEventListener('change', function () {
          exporting.ticked[c.chatId] = box.checked;
          drawWorkspaceExportTotals();
        });
      });
      $('workspace-export-chats-wrap').hidden = !chats.length;
      $('workspace-export-none').hidden = chats.length > 0;
      var rows = exporting.rows = workspaceExportRows(), groups = $('workspace-export-groups').tBodies[0];
      groups.replaceChildren();
      rows.forEach(function (row) {
        var tr = groups.appendChild(el('tr'));
        transferCell(tr, row.title, 'wrap', 'Group or Team');
        row.cell = transferCell(tr, '', 'nowrap', 'Goes With It');
        transferCell(tr, row.elsewhere.length ? listWords(row.elsewhere) : 'None', 'muted wrap', 'Elsewhere');
      });
      $('workspace-export-groups-title').hidden = $('workspace-export-groups-wrap').hidden = !rows.length;
      // The rules for what goes are stated once below the table, and never
      // change as chats are ticked.
      var notes = $('workspace-export-group-notes');
      notes.replaceChildren();
      if ((plan.groups || []).length) notes.appendChild(el('li', null, 'A group goes, with its conversation, when any of its members here goes.'));
      if ((plan.teams || []).length) notes.appendChild(el('li', null, 'A team goes when its lead or any of its members goes. When its lead does not go, the team\'s budget stays behind, and its members rejoin the team on a computer that has the lead.'));
      notes.hidden = !notes.children.length;
      drawWorkspaceExportTotals();
    }
    /* workspaceExportRows lists each group and team with its members in this
       workspace, the lead of a team whose lead is here, and the members in
       other workspaces, so that its row can say how many go as chats are
       ticked. */
    function workspaceExportRows() {
      var plan = exporting.plan, rows = [];
      var here = function (m) { return m.here; };
      (plan.groups || []).forEach(function (g) {
        rows.push({ title: g.title + ', a group', here: g.members.filter(here), lead: null,
          elsewhere: g.members.filter(function (m) { return !m.here; }).map(function (m) { return m.title + ' in ' + m.workspace; }) });
      });
      (plan.teams || []).forEach(function (t) {
        var all = [t.lead].concat(t.members);
        rows.push({ title: t.lead.title + '\'s team', here: all.filter(here), lead: t.lead.here ? t.lead : null,
          elsewhere: all.filter(function (m) { return !m.here; }).map(function (m) { return m.title + (m === t.lead ? ' (its lead)' : '') + ' in ' + m.workspace; }) });
      });
      return rows;
    }
    function exportGoesWords(row, ticked) {
      var going = row.here.filter(function (m) { return ticked[m.chatId]; }).length;
      if (!going) return 'Stays behind';
      var words = going + ' of ' + row.here.length + (row.here.length === 1 ? ' member' : ' members');
      return row.lead && !ticked[row.lead.chatId] ? words + ', not its lead' : words;
    }
    function drawWorkspaceExportTotals() {
      var plan = exporting.plan, ticked = exporting.ticked, chats = plan.chats || [];
      var n = 0, bytes = 0;
      chats.forEach(function (c) { if (ticked[c.chatId]) { n++; bytes += c.size; } });
      // The All box is ticked when every chat is, and shows the mixed state
      // when only some are. The narrow layout has its own copy above the
      // cards, since it hides the table's head.
      ['workspace-export-all', 'workspace-export-all-cards'].forEach(function (id) {
        var all = $(id);
        all.checked = chats.length > 0 && n === chats.length;
        all.indeterminate = n > 0 && n < chats.length;
        all.disabled = !chats.length;
      });
      var total = $('workspace-export-total');
      total.textContent = chats.length ? n + ' of ' + chats.length + (chats.length === 1 ? ' chat, ' : ' chats, ') + megabytes(bytes) + ' of history' : '';
      total.title = total.textContent;
      exporting.rows.forEach(function (row) { row.cell.textContent = exportGoesWords(row, ticked); });
    }
    function tickAllChats() {
      var plan = exporting.plan, on = this.checked;
      if (!plan) return;
      (plan.chats || []).forEach(function (c) { exporting.ticked[c.chatId] = on; });
      $('workspace-export-chats').querySelectorAll('tbody input[type=checkbox]').forEach(function (box) { box.checked = on; });
      drawWorkspaceExportTotals();
    }
    $('workspace-export-all').addEventListener('change', tickAllChats);
    $('workspace-export-all-cards').addEventListener('change', tickAllChats);
    $('workspace-export-cancel').addEventListener('click', function () { $('workspace-export').close(); });
    $('workspace-export').addEventListener('cancel', function (event) { if (exporting.busy) event.preventDefault(); });
    $('workspace-export-go').addEventListener('click', function () {
      var plan = exporting.plan, go = this, cancel = $('workspace-export-cancel');
      if (!plan || exporting.busy) return;
      var chats = (plan.chats || []).filter(function (c) { return exporting.ticked[c.chatId]; }).map(function (c) { return c.chatId; });
      exporting.busy = true; go.disabled = true; cancel.disabled = true;
      Promise.resolve(app().ExportWorkspace(plan.workspaceId, { chats: chats })).then(function (file) {
        // Cancelling the save window leaves the export window as it was.
        if (!file) return;
        $('workspace-export').close();
        toast('Exported "' + plan.name + '" to ' + file + '. Import Workspace… in ' + productName() + ' on another computer reads it.', 'positive');
      }).catch(fail).finally(function () { exporting.busy = false; go.disabled = false; cancel.disabled = false; });
    });
    var importing = { plan: null, choices: {}, busy: false };
    function importWorkspace() {
      if (!app().ChooseWorkspaceImport || importing.busy) return;
      Promise.resolve(app().ChooseWorkspaceImport()).then(function (plan) {
        if (!plan || !plan.file) return;
        importing = { plan: plan, choices: {}, separate: {}, busy: false, createNote: false };
        drawWorkspaceImport();
        $('workspace-import').showModal();
        $('workspace-import-body').scrollTop = 0;
        $('workspace-import-go').focus();
      }).catch(fail);
    }
    function importChoices(choices) {
      var out = Object.keys(choices).map(function (key) { return { key: key, path: choices[key].path || '', create: !!choices[key].create }; });
      var separate = importing.separate || {};
      Object.keys(separate).forEach(function (key) { if (separate[key]) out.push({ key: '', chat: key, separate: true }); });
      return out;
    }
    /* The import window's Chats table (docs/design/workspace-export.md,
       "The Windows"): each chat of the file on one row, with what importing
       it does as a badge. A chat changed on both computers is left as it is
       unless the person ticks Import as a Separate Chat on its row. What
       each badge means is said once below the table, so that ticking
       changes no row's height. */
    var chatMarks = {
      'new': ['New', 'accent'],
      'up-to-date': ['Already Up to Date', ''],
      'updated': ['Updated From the Other Computer', 'positive'],
      'changed-on-both': ['Changed on Both Computers', 'warn'],
      'close-to-update': ['Close It to Update It', 'warn']
    };
    var chatMarkHelp = {
      'up-to-date': 'Nothing changed on the other computer.',
      'updated': 'It was continued on the other computer. Its newer history replaces this one\'s.',
      'changed-on-both': 'Both computers have messages the other lacks. This computer\'s chat is left as it is. Import as a Separate Chat adds the other computer\'s copy beside it, with "(Imported)" after its title and its own place in its groups.',
      'close-to-update': 'It is open and working here. Close it and import again to take the other computer\'s history.'
    };
    function drawImportChats(plan) {
      const el = window.dinahHelpers.node;
      var chats = plan.chats || [], body = $('workspace-import-chats').tBodies[0];
      $('workspace-import-chats-title').hidden = $('workspace-import-chats-wrap').hidden = !chats.length;
      body.replaceChildren();
      var separate = importing.separate;
      chats.forEach(function (c) {
        var tr = body.appendChild(el('tr'));
        tr.dataset.chatKey = c.key;
        transferCell(tr, c.title, 'wrap', 'Chat');
        var program = transferCell(tr, null, 'muted', 'Program and Model');
        program.appendChild(el('span', null, c.providerName + (c.model ? ' · ' + c.model : '')));
        if (c.providerMissing) program.appendChild(transferBadge(['Not on This Computer', 'warn']));
        transferCell(tr, null, 'nowrap', 'What Happens').appendChild(transferBadge(chatMarks[c.mark] || [c.mark, '']));
        var choice = transferCell(tr, null, 'nowrap', 'Choice');
        if (c.mark !== 'changed-on-both') return;
        var label = choice.appendChild(el('label', 'check'));
        var box = label.appendChild(el('input'));
        box.type = 'checkbox'; box.checked = !!separate[c.key];
        label.appendChild(document.createTextNode(' Import as a Separate Chat'));
        box.addEventListener('change', function () { separate[c.key] = box.checked; drawImportTotal(); });
      });
      var notes = $('workspace-import-chat-notes');
      notes.replaceChildren();
      Object.keys(chatMarkHelp).forEach(function (mark) {
        if (chats.some(function (c) { return c.mark === mark; })) notes.appendChild(el('li', null, chatMarks[mark][0] + ': ' + chatMarkHelp[mark]));
      });
      var missing = {};
      chats.forEach(function (c) { if (c.providerMissing) missing[c.providerName] = (missing[c.providerName] || 0) + 1; });
      Object.keys(missing).forEach(function (name) {
        notes.appendChild(el('li', null, 'Not on This Computer: ' + name + ' is not on this computer. ' + (missing[name] === 1 ? 'The chat opens on its saved history and asks which program continues it.'
          : 'The chats open on their saved history and ask which program continues them.')));
      });
      notes.hidden = !notes.children.length;
    }
    /* The Groups and Teams table: each group of the file, new or joining
       the group of its name here, with its members and its conversation,
       then each team. A member whose chat is neither in the file nor here
       waits for its own workspace's import. The trusted links the other
       computer had are listed last, as not imported. */
    function waitingWords(waiting) {
      return waiting.map(function (m) { return m.title + ' waits for the workspace ' + m.workspace + '; importing ' + m.workspace + ' adds it.'; }).join(' ');
    }
    function drawImportGroups(plan) {
      const el = window.dinahHelpers.node;
      var groups = plan.groups || [], teams = plan.teams || [], body = $('workspace-import-groups').tBodies[0];
      $('workspace-import-groups-title').hidden = $('workspace-import-groups-wrap').hidden = !groups.length && !teams.length;
      body.replaceChildren();
      var count = function (n, one, many) { return n + ' ' + (n === 1 ? one : many); };
      var row = function (title, mark, said) {
        var tr = body.appendChild(el('tr'));
        transferCell(tr, title, 'wrap', 'Group or Team');
        transferCell(tr, null, 'nowrap', 'What Happens').appendChild(transferBadge(mark));
        transferCell(tr, said.join(' '), 'muted wrap', 'Members and Conversation');
      };
      groups.forEach(function (g) {
        var waiting = g.waiting || [], said = [];
        if (g.existing && !waiting.length) said.push('All its members are here already.');
        else {
          if (g.inFile) said.push(count(g.inFile, 'member is', 'members are') + ' in this file.');
          if (g.here) said.push(count(g.here, 'member is', 'members are') + ' here already.');
        }
        if (waiting.length) said.push(waitingWords(waiting));
        if (g.messages) said.push(!g.existing ? 'Its conversation comes too: ' + count(g.messages, 'message', 'messages') + '.'
          : g.new ? count(g.new, 'newer message is', 'newer messages are') + ' added to its conversation.' : 'Its conversation is here already.');
        if (g.keptLead) said.push('It keeps its lead here, ' + g.keptLead + ', although the file gives it another.');
        row(g.title + ', a group', g.existing ? ['Joins the Existing Group', ''] : ['New Group', 'accent'], said);
      });
      teams.forEach(function (t) {
        var said = [listWords(t.members) + (t.members.length === 1 ? ' reports' : ' report') + ' to ' + t.lead + '.'];
        if ((t.waiting || []).length) said.push(waitingWords(t.waiting));
        row(t.lead + '\'s team', t.existing ? ['Joins the Existing Team', ''] : ['New Team', 'accent'], said);
      });
      var links = plan.links || [], line = $('workspace-import-links');
      line.hidden = !links.length;
      line.textContent = links.length ? 'Not imported: the trusted ' + (links.length === 1 ? 'link' : 'links') + ' between ' + links.map(function (l) { return l.a + ' and ' + l.b; }).join(', and between ') +
        '. A file cannot grant agents that trust; add ' + (links.length === 1 ? 'it' : 'them') + ' again in Settings if you want ' + (links.length === 1 ? 'it' : 'them') + '.' : '';
    }
    function importStatus(s, create) {
      if (s.kind === 'local-folder') return s.found ? ['Found', 'positive'] : create ? ['Created on Import', 'accent'] : ['Not on This Computer Yet', 'warn'];
      if (!s.checked) return ['Kept, Not Checked', ''];
      return s.problem ? [s.problem, 'warn'] : ['Available', 'positive'];
    }
    function drawWorkspaceImport() {
      var plan = importing.plan, choices = importing.choices;
      const el = window.dinahHelpers.node;
      var withChats = (plan.chats || []).length > 0;
      $('workspace-import-summary').textContent = plan.existingId
        ? '"' + plan.existingName + '" already works in this main folder, so the import adds ' + (withChats ? 'what it does not have and updates chats worked on elsewhere.' : 'the sources and pins it does not have.')
        : 'This makes the workspace "' + plan.name + '" with these sources' + (withChats ? ' and chats.' : '.');
      drawImportChats(plan);
      drawImportGroups(plan);
      /* Each source is a row: its name, where it is, what was found, and
         the choices the plan's actions allow. Ticking Create It Now changes
         only the words of the Found badge, in a column wide enough for every
         wording, so no row or column moves. */
      var body = $('workspace-import-sources').tBodies[0], waitingMain = false;
      body.replaceChildren();
      plan.sources.forEach(function (s) {
        var tr = body.appendChild(el('tr'));
        tr.dataset.key = s.key;
        var name = transferCell(tr, null, 'wrap', 'Source');
        name.appendChild(el('span', null, s.name || s.reference || s.path || s.kind));
        if (s.main) name.appendChild(transferBadge(['Main Folder', '']));
        var actions = s.actions || [], create = !!(choices[s.key] && choices[s.key].create) && actions.indexOf('create-folder') >= 0;
        if (actions.indexOf('create-folder') >= 0) importing.createNote = true;
        if (s.main && s.kind === 'local-folder' && !s.found) waitingMain = true;
        var where = s.kind === 'local-folder' ? s.path : s.site ? s.site + (s.reference ? ' · ' + s.reference : '') : '';
        var whereCell = transferCell(tr, null, 'muted wrap', 'Where');
        if (where) whereCell.appendChild(el('span', null, where));
        if (s.kind !== 'local-folder' && !s.checked) whereCell.appendChild(el('p', 'form-help', productName() + ' does not use this kind of source (' + s.kind + '). It is kept as it is, for a version that does.'));
        transferCell(tr, null, 'wrap', 'Found').appendChild(transferBadge(importStatus(s, create)));
        var choicesCell = transferCell(tr, null, '', 'Folder');
        if (!actions.length) return;
        var controls = choicesCell.appendChild(el('div', 'transfer-choices'));
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
        var label = controls.appendChild(el('label', 'check'));
        var box = label.appendChild(el('input'));
        box.type = 'checkbox'; box.checked = create;
        label.appendChild(document.createTextNode(' Create It Now' + (s.folders ? ', With ' + s.folders + (s.folders === 1 ? ' Folder' : ' Folders') + ' Inside' : '')));
        box.addEventListener('change', function () {
          choices[s.key] = Object.assign({}, choices[s.key], { create: box.checked });
          drawWorkspaceImport();
          // The row is drawn again, so the new tick box takes the focus.
          var again = Array.prototype.find.call(body.rows, function (row) { return row.dataset.key === s.key; });
          var next = again && again.querySelector('input[type=checkbox]');
          if (next) next.focus();
        });
      });
      var pins = $('workspace-import-pins');
      pins.hidden = !plan.pins;
      pins.textContent = plan.pins + (plan.pins === 1 ? ' pin comes' : ' pins come') + ' with it.' +
        (plan.uncheckedPins ? ' ' + plan.uncheckedPins + (plan.uncheckedPins === 1 ? ' of them is' : ' of them are') + ' of a kind ' + productName() + ' does not use, and ' + (plan.uncheckedPins === 1 ? 'is' : 'are') + ' kept as ' + (plan.uncheckedPins === 1 ? 'it is.' : 'they are.') : '');
      // What Create It Now does is said once, below the table, and stays
      // for as long as the window is open, so that choosing a folder never
      // moves what comes after it.
      var createNote = $('workspace-import-create');
      createNote.hidden = !importing.createNote;
      createNote.textContent = 'Create It Now makes the folder and the folders recorded inside it. If OneDrive or another service is still bringing the folder, leave it for that service instead. A folder that is not created now is shown, dimmed, until it appears here, and Create the Folders… in its menu makes it later.';
      $('workspace-import-waiting').hidden = !waitingMain;
      $('workspace-import-go').textContent = plan.existingId ? 'Add to the Workspace' : 'Import';
      drawImportTotal();
    }
    /* drawImportTotal writes the short summary at the start of the button
       row: the chats added, updated and added separately and the group
       messages, or, for a file without them, its sources and pins. */
    function drawImportTotal() {
      var plan = importing.plan, chats = plan.chats || [], separate = importing.separate || {};
      var count = function (n, one, many) { return n + ' ' + (n === 1 ? one : many); };
      var marked = function (mark) { return chats.filter(function (c) { return c.mark === mark; }).length; };
      var apart = chats.filter(function (c) { return c.mark === 'changed-on-both' && separate[c.key]; }).length;
      var messages = (plan.groups || []).reduce(function (n, g) { return n + ((g.existing ? g.new : g.messages) || 0); }, 0);
      var parts = [];
      if (marked('new')) parts.push(count(marked('new'), 'new chat', 'new chats'));
      if (marked('updated')) parts.push(marked('updated') + ' updated');
      if (apart) parts.push(count(apart, 'separate chat', 'separate chats'));
      if (messages) parts.push(count(messages, 'group message', 'group messages'));
      if (!parts.length) parts.push(count(plan.sources.length, 'source', 'sources') + (plan.pins ? ' and ' + count(plan.pins, 'pin', 'pins') : ''));
      var total = $('workspace-import-total');
      total.textContent = parts.join(', ');
      total.title = total.textContent;
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
        var count = function (n, one, many) { return n + ' ' + (n === 1 ? one : many); };
        var chatWords = [];
        if (result.chats) chatWords.push(count(result.chats, 'chat', 'chats'));
        if (result.groups && !result.existing) chatWords.push(count(result.groups, 'group', 'groups'));
        var added = result.existing ? 'Added ' + count(result.sources, 'source', 'sources') + ' and ' + count(result.pins, 'pin', 'pins') + ' to "' + plan.existingName + '".'
          : 'Imported the workspace "' + plan.name + '"' + (chatWords.length ? ' with ' + chatWords.join(' and ') : '') + '.';
        if (result.existing && result.chats) added += ' Added ' + count(result.chats, 'chat', 'chats') + '.';
        if (result.updated) added += ' Updated ' + count(result.updated, 'chat', 'chats') + ' worked on elsewhere.';
        if (result.separate) added += ' Added ' + count(result.separate, 'separate chat', 'separate chats') + '.';
        if (result.existing && result.messages) added += ' Added ' + count(result.messages, 'group message', 'group messages') + '.';
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
        return Promise.resolve(app().CreateWorkspaceFolders(record.id, main.id)).then(function () { return app().SelectWorkspace(record.id); });
      });
    }
    var renamingWorkspace = '', workspaceRenameBusy = false;
    var creatingWorkspace = false;
    function commandEnabled(id) {
      const methods = { 'rename-workspace': 'RenameWorkspace', 'remove-workspace': 'RemoveWorkspace', 'export-workspace': 'PlanWorkspaceExport' };
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
