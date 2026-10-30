/* Pinned items: chats and items
   of the workspace's sources, pinned to the top of the Workspace
   pane in the person's order. Go keeps them on the workspace's record
   (workspace_pins.go); this module draws the Pinned group, gives the other
   lists Pin to Workspace and Unpin from Workspace, and opens a pinned item
   through the opener its kind registered. */
(function () {
  'use strict';
  const api = window.dinahHelpers.app;
  function fail(err) { if (window.dinahHost) window.dinahHost.fail(err); }
  const node = window.dinahHelpers.node;
  var openers = {};

  // A pin's identity in its workspace, as workspaces.PinKey makes it.
  function key(p) {
    if (p.kind === 'chat') return 'chat:' + p.session;
    if (p.kind === 'resource' && p.ref) return 'resource:' + p.ref.sourceId + '/' + (p.ref.path || '');
    return '';
  }
  function record() {
    var id = window.dinahHost && window.dinahHost.workspaceId();
    var records = window.dinahWorkspaceBrowser ? window.dinahWorkspaceBrowser.records() : [];
    return records.find(function (r) { return r.id === id; }) || null;
  }
  function pins() { var r = record(); return (r && r.pins) || []; }
  function isPinned(p) { var k = key(p); return pins().some(function (x) { return key(x) === k; }); }
  // A changed record replaces the one the pane has, so every list redraws.
  function changed(updated) {
    var records = window.dinahWorkspaceBrowser.records().map(function (r) { return r.id === updated.id ? updated : r; });
    window.dinahWorkspaceBrowser.setRecords(records);
  }
  function toggle(p) {
    var r = record();
    if (!r || !api()) return Promise.resolve();
    return Promise.resolve().then(function () {
      return isPinned(p) ? api().UnpinWorkspaceItem(r.id, key(p)) : api().PinWorkspaceItem(r.id, p);
    }).then(changed).catch(fail);
  }
  function move(p, index) {
    var r = record();
    if (!r) return;
    Promise.resolve().then(function () { return api().MoveWorkspacePin(r.id, key(p), index); }).then(changed).catch(fail);
  }
  /* A command that pins the item or takes its pin off, for any menu; null
     when no workspace is open. */
  function command(p) {
    if (!p || !record()) return null;
    return { label: isPinned(p) ? 'Unpin from Workspace' : 'Pin to Workspace', run: function () { toggle(p); } };
  }

  /* A row's context menu, with Pin to Workspace or Unpin from Workspace
     for the item pinOf gives and any rows more gives, and PUDL's row
     actions button last in the row. The button, a right-click, Shift+F10
     and a long press all open the same menu. */
  function rowActions(row, pinOf, more) {
    var b = node('button'); b.type = 'button'; b.setAttribute('data-row-actions', '');
    row.appendChild(b);
    row.addEventListener('pudl:contextmenu', function (e) {
      if (e.detail.count()) return;
      e.detail.add([command(pinOf())].concat(more ? more() : []).filter(Boolean));
      e.detail.mark(row);
    });
  }

  function glyphOf(p) {
    if (p.kind === 'chat') return 'comments';
    return window.dinahResources ? window.dinahResources.glyphName({ kind: p.itemKind, directory: p.directory }) : 'document';
  }
  function whereOf(p) {
    if (p.kind === 'chat') return 'Chat';
    var r = record(), source = r && r.sources.find(function (s) { return p.ref && s.id === p.ref.sourceId; });
    return source ? source.name : '';
  }
  function open(p) {
    var opener = openers[p.kind];
    if (opener) Promise.resolve().then(function () { return opener(p); }).catch(fail);
  }

  /* The Pinned group: one row a pin, with its glyph, its title and where it
     is; Enter or a click opens it, its context menu unpins or moves it, and a
     row can be dragged to a new place. A pin of a kind this page does not
     know, which an imported workspace can bring from a version that has it,
     stays on the record but has no row. A move counts places among all the
     pins, so it keeps those in their places. */
  var dragging = null;
  function place(p) { var k = key(p); return pins().findIndex(function (x) { return key(x) === k; }); }
  function draw() {
    var host = document.getElementById('workspace-pins'), group = document.getElementById('workspace-pins-group');
    if (!host || !group) return;
    var list = pins().filter(function (p) { return key(p) !== ''; });
    host.replaceChildren();
    list.forEach(function (p, i) {
      var row = window.dinahBrowserList.row({ title: p.title, meta: [whereOf(p)], tooltip: p.title, onOpen: function () { open(p); }, data: { pinKey: key(p) } });
      var glyph = node('span', 'glyph pin-glyph'); glyph.setAttribute('aria-hidden', 'true');
      glyph.style.setProperty('--glyph', 'var(--glyph-' + glyphOf(p) + ')');
      row.querySelector('.browser-row-line').prepend(glyph);
      rowActions(row, function () { return p; }, function () {
        return [{ label: 'Move Up', disabled: i === 0, run: function () { move(p, place(list[i - 1])); } },
          { label: 'Move Down', disabled: i === list.length - 1, run: function () { move(p, place(list[i + 1])); } }];
      });
      row.draggable = true;
      row.addEventListener('dragstart', function (e) { dragging = p; e.dataTransfer.effectAllowed = 'move'; e.dataTransfer.setData('text/plain', p.title); row.classList.add('dragging'); });
      row.addEventListener('dragend', function () { dragging = null; row.classList.remove('dragging'); });
      row.addEventListener('dragover', function (e) { if (dragging) { e.preventDefault(); e.dataTransfer.dropEffect = 'move'; } });
      row.addEventListener('drop', function (e) {
        if (!dragging) return;
        e.preventDefault();
        var from = list.findIndex(function (x) { return key(x) === key(dragging); });
        if (from >= 0 && from !== i) move(dragging, place(p));
      });
      host.appendChild(row);
    });
    window.dinahBrowserList.setCount(group, list.length);
    document.getElementById('workspace-pins-empty').hidden = list.length > 0;
  }
  document.addEventListener('dinah:workspace-resource-labels', draw);

  window.dinahPins = {
    key: key, isPinned: isPinned, toggle: toggle, command: command, rowActions: rowActions, draw: draw,
    registerOpener: function (kind, fn) { openers[kind] = fn; }
  };
})();
