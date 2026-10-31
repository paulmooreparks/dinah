(function () {
  'use strict';
  /* ---- In Progress ---------------------------------------------------------
     Everything the person started and has not finished: drafts, new documents never saved anywhere; changed
     documents, whose changes or suggestions are not saved yet. The In
     Progress pane lists them for every open workspace, a group to a
     workspace; the Workspace pane's In Progress group lists the current
     workspace's. Both draw from one list, loaded again shortly after a draft
     changes. */
  const app = window.dinahHelpers.app;
  function host() { return window.dinahHost; }
  function work() { return window.dinahDocuments.work; }
  const message = window.dinahHelpers.message;

  var items = [], loading = null, timer = 0, listeners = new Set();
  function records() { return window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.records() : []; }
  /* A workspace is told apart by its record's ID. */
  function placeOfDraft(d) {
    var rec = records().find(function (r) { return r.id === d.origin.workspaceId; });
    return rec ? { key: rec.id, title: rec.executionPath ? host().workspaceLabel(rec.executionPath) : rec.name } : { key: d.origin.workspaceId, title: 'Other Workspace' };
  }

  /* Where a changed document is, within its workspace's group: a file's
     path, or the name of the source it is in. */
  function whereIn(d) {
    if (d.origin.kind === 'local-folder') return d.origin.path;
    var rec = records().find(function (r) { return r.id === d.origin.workspaceId; });
    var source = rec && (rec.sources || []).find(function (s) { return s.id === d.origin.sourceId; });
    return source ? source.name : work().place(d);
  }
  function itemOfDraft(d) {
    var n = (d.suggestions || []).length;
    var item = {
      kind: work().isDraft(d) ? 'draft' : 'changed', place: placeOfDraft(d), key: 'draft:' + d.id,
      title: d.title || 'Untitled', meta: work().isDraft(d) ? [work().state(d)] : [whereIn(d), work().state(d)],
      badge: n ? { text: n, kind: 'warn', label: n === 1 ? '1 suggestion to decide' : n + ' suggestions to decide' } : null,
      words: (d.title || '') + ' ' + work().place(d),
      open: function () { return window.dinahEditor.open(d); }
    };
    item.menu = function () {
      return [{ label: 'Open', run: function () { openItem(item); } },
        { label: work().isDraft(d) ? 'Discard Draft…' : 'Discard Changes…', run: function () { work().discard(d).then(function (gone) { if (gone) reload(); }); } }];
    };
    return item;
  }
  function openItem(item) {
    Promise.resolve().then(function () { return item.open(); }).catch(function (err) { host().toast('"' + item.title + '" could not be opened: ' + message(err), 'danger'); });
  }

  function load() {
    if (!app() || !app().ListDrafts) return Promise.resolve(items);
    if (loading) return loading;
    loading = Promise.resolve(app().ListDrafts()).then(function (drafts) {
      items = (drafts || []).map(itemOfDraft);
      listeners.forEach(function (fn) { fn(); });
      return items;
    }).finally(function () { loading = null; });
    return loading;
  }
  function reload() { clearTimeout(timer); timer = setTimeout(function () { load().catch(function () {}); }, 150); }

  var KINDS = [['draft', 'Drafts'], ['changed', 'Changed Documents']];
  /* The rows of one workspace's items, in a group for each kind there is,
     which opens and closes like any other and is remembered by id. */
  function fill(body, list, id) {
    var sections = window.dinahBrowserSections;
    KINDS.forEach(function (k) {
      var of = list.filter(function (item) { return item.kind === k[0]; });
      if (!of.length) return;
      var gid = id + '-' + k[0];
      var g = window.dinahBrowserList.group({ id: gid, title: k[1], count: of.length, open: sections ? sections.open(gid) : true, onToggle: function (open) { if (sections) sections.save(gid, open); } });
      g.el.classList.add('in-progress-kind');
      of.forEach(function (item) {
        var row = window.dinahBrowserList.row({ title: item.title, meta: item.meta, badge: item.badge, tooltip: item.title, data: { inProgress: item.key }, onOpen: function () { openItem(item); } });
        window.dinahPins.rowActions(row, function () { return null; }, function () { return item.menu(); });
        g.body.appendChild(row);
      });
      body.appendChild(g.el);
    });
  }

  /* The Workspace pane's group: the current workspace's items. */
  function drawWorkspaceGroup() {
    var list = document.getElementById('workspace-in-progress'), empty = document.getElementById('workspace-in-progress-empty');
    if (!list) return;
    var id = host().workspaceId(), mine = items.filter(function (item) { return item.place.key === id; });
    list.replaceChildren();
    fill(list, mine, 'workspace-in-progress');
    empty.hidden = mine.length > 0;
    window.dinahBrowserList.setCount(list.closest('.browser-group'), mine.length);
  }
  listeners.add(drawWorkspaceGroup);

  window.dinahWorkbench.register('in-progress', { applet: { id: 'dinah.in-progress', title: 'In Progress', glyph: 'pencil', workspace: false }, singleton: true, title: 'In Progress', glyph: 'pencil', mode: 'dock-left', init: function (root, opts) {
    root.classList.add('in-progress-browser');
    var disposed = false, scope = 'all', query = '';
    var bar = window.dinahBrowserList.toolbar({
      search: { label: 'Find Work in Progress', onInput: function (value) { query = value; draw(); opts.changed(state()); } },
      scope: { label: 'Scope', options: [['all', 'All Workspaces'], ['current', 'This Workspace']], onChange: function (value) { scope = value; draw(); opts.changed(state()); } },
      actions: [{ label: 'Refresh', run: function () { load().catch(host().fail); } }]
    });
    var groups = document.createElement('div'); groups.className = 'in-progress-results';
    root.append(bar.el, groups);
    function state() { return JSON.stringify({ scope: scope, query: query }); }
    function draw() {
      if (!disposed) window.dinahRender.enqueue(root, root, render);
    }
    function render() {
      if (disposed) return;
      groups.replaceChildren();
      var current = host().workspaceId(), words = query.trim().toLowerCase();
      var shown = items.filter(function (item) { return (scope === 'all' || item.place.key === current) && (!words || item.words.toLowerCase().indexOf(words) >= 0); });
      var order = [], byKey = {};
      shown.forEach(function (item) { if (!byKey[item.place.key]) { byKey[item.place.key] = { place: item.place, list: [] }; order.push(item.place.key); } byKey[item.place.key].list.push(item); });
      order.sort(function (x, y) { return (y === current) - (x === current) || byKey[x].place.title.localeCompare(byKey[y].place.title); });
      order.forEach(function (key) {
        var sections = window.dinahBrowserSections, id = 'in-progress-' + key;
        var g = window.dinahBrowserList.group({ id: id, title: byKey[key].place.title, count: byKey[key].list.length, open: sections ? sections.open(id) : true, onToggle: function (open) { if (sections) sections.save(id, open); } });
        fill(g.body, byKey[key].list, id);
        groups.appendChild(g.el);
      });
      if (!shown.length) groups.appendChild(window.dinahBrowserList.empty({
        title: words ? 'Nothing Matches' : 'Nothing In Progress',
        body: words ? 'No draft or changed document here has those words in it.' : 'A new document not yet saved and a document with changes not yet saved appear here.'
      }));
    }
    function setState(value) {
      try { var saved = JSON.parse(value || '{}'); scope = saved.scope === 'current' ? 'current' : 'all'; query = saved.query || ''; } catch (_) {}
      bar.search.value = query; bar.scope.value = scope; draw();
    }
    listeners.add(draw);
    setState(opts.state);
    load().catch(host().fail);
    return { state: state, setState: setState, commands: function () { return [{ label: 'Refresh', run: function () { load().catch(host().fail); } }]; },
      destroy: function () { disposed = true; window.dinahRender.cancel(root); listeners.delete(draw); } };
  } });

  /* The pane is one of the default windows, which a first run opens. A
     layout saved before the pane existed does not have it, so it is opened
     once, as a tab behind the one in front, and never again: a person who
     closes it has chosen to. */
  /* It waits for both the restored layout and the windows, which start when
     the page has loaded; either can come first. */
  var OFFERED = 'dinah-in-progress-offered', offered = false;
  document.addEventListener('dinah:ui-state-restored', offer);
  window.addEventListener('load', offer);
  function offer() {
    var ui = window.dinahUIState, windows = window.pudlWindows;
    if (offered || !ui || ui.isRestoring() || !windows || document.readyState !== 'complete') return;
    offered = true;
    if (ui.get(OFFERED)) return;
    ui.set(OFFERED, true);
    /* "This was offered" is a fact that must survive the fastest close, not
       window layout that may lose its last few hundred milliseconds; flushed
       at once, as the bus pane's offer is (bus-applets.js). A failed write
       meets a page on its way out, and is not reported. */
    ui.flush().catch(function () {});
    var before = windows.state();
    if (before.open.indexOf('in-progress') >= 0) return;
    var front = before.docks && before.docks.left && before.docks.left.selected, top = before.top;
    window.dinahWorkbench.open('in-progress').then(function () {
      if (front) windows.raise(front);
      if (top && top !== front) windows.raise(top);
    }).catch(function () {});
  }

  document.addEventListener('DOMContentLoaded', function () {
    host().on('draft:changed', reload);
    host().onResync(reload);
    document.addEventListener('dinah:workspace-resource-labels', function () { listeners.forEach(function (fn) { fn(); }); reload(); });
    reload();
  });
  window.dinahInProgress = { load: load, items: function () { return items.slice(); } };
})();
