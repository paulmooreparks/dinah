/* Read-only projection of OrgChart(), specified in docs/design/org-chart-query.md. */
(function () {
  'use strict';
  var node = window.dinahHelpers.node, app = window.dinahHelpers.app;
  function init(root, opts) {
    var disposed = false, snapshot = null, pending = null, again = false, geometry = '', drawing = null, elements = new Map(), folded = new Set();
    var workspace = '', find = '', readingState = false;
    root.classList.add('org-chart');
    var toolbar = node('div', 'org-chart-toolbar'); toolbar.setAttribute('role', 'toolbar'); toolbar.setAttribute('aria-label', 'Org Chart');
    var scope = node('select', 'form-select'); scope.setAttribute('aria-label', 'Workspace');
    var search = node('input', 'form-input'); search.type = 'search'; search.placeholder = 'Find a Chat'; search.setAttribute('aria-label', 'Find a Chat');
    var refresh = node('button', 'btn btn-sm', 'Refresh'); refresh.type = 'button';
    toolbar.append(scope, search, refresh);
    var status = node('p', 'org-chart-status text-muted', 'Loading the chart.'); status.setAttribute('role', 'status'); status.setAttribute('aria-live', 'polite');
    var legend = node('p', 'org-chart-legend text-muted', 'Solid lines show teams. Dotted lines show groups.');
    var scroll = node('div', 'org-chart-scroll'); scroll.tabIndex = 0; scroll.setAttribute('aria-label', 'Org Chart');
    var canvas = node('div', 'org-chart-canvas'); scroll.append(canvas); root.append(toolbar, status, legend, scroll);
    function address() { var p = new URLSearchParams(); if (workspace) p.set('workspace', workspace); if (find) p.set('q', find); Array.from(folded).sort().forEach(function (id) { p.append('fold', id); }); return p.toString(); }
    function readState(value) { var p = new URLSearchParams(value || ''); workspace = p.get('workspace') || ''; find = p.get('q') || ''; folded = new Set(p.getAll('fold')); search.value = find; scope.value = workspace; }
    function changed() { if (!readingState) opts.changed(address()); render(); }
    function options() { return { width: Math.max(160, scroll.clientWidth - 32), workspace: workspace, find: find.trim(), folded: Array.from(folded) }; }
    function place(el, p) { el.style.left = p.x + 'px'; el.style.top = p.y + 'px'; el.style.width = p.width + 'px'; el.style.height = p.height + 'px'; }
    function palette() { var css = getComputedStyle(root); function colour(key) { return css.getPropertyValue(key).trim(); } return { bg: colour('--bg'), surface: colour('--surface'), text: colour('--text'), muted: colour('--text-muted'), line: colour('--text-muted'), accent: colour('--accent'), presets: [] }; }
    function drawLines() {
      if (!drawing) return;
      var colours = palette(), nodes = [], edges = [], added = new Set();
      function box(p, group) {
        if (added.has(p.id)) return; added.add(p.id);
        nodes.push({ id: p.id, type: group ? 'group' : 'text', x: p.x, y: p.y, width: p.width, height: p.height, text: '', label: '', yavc: { radius: 5, fillColor: group ? undefined : colours.surface, strokeColor: colours.line, lineWidth: 1 } });
      }
      drawing.frames.forEach(function (p) { box(Object.assign({}, p, { id: 'frame:' + p.id }), true); });
      drawing.positions.forEach(function (p) { box(p); });
      drawing.controls.forEach(function (p) { box(Object.assign({}, p, { id: 'fold:' + p.id })); });
      drawing.routes.forEach(function (e) {
        box(e.from); box(e.to);
        function line(id, a, side, b, toSide) { edges.push({ id: id, fromNode: a, fromSide: side, fromEnd: 'none', toNode: b, toSide: toSide, toEnd: 'none', yavc: { line: e.dotted ? 'dotted' : 'solid', lineWidth: 2, route: 'straight' } }); }
        var anchors = e.points.map(function (p, index) { var id = 'route:' + e.id + ':' + index; nodes.push({ id: id, type: 'text', x: p[0], y: p[1], width: 0, height: 0, text: '', yavc: { shape: 'text', anchor: true } }); return id; });
        line(e.id + ':start', e.from.id, e.fromSide, anchors[0], 'left');
        for (var index = 0; index + 1 < anchors.length; index++) line(e.id + ':' + index, anchors[index], 'right', anchors[index + 1], 'left');
        line(e.id + ':end', anchors[anchors.length - 1], 'right', e.to.id, e.toSide);
      });
      nodes.push({ id: 'bounds', type: 'text', x: 0, y: 0, width: drawing.width, height: drawing.height, text: '', yavc: { shape: 'text' } });
      var checked = window.yavcCanvas.read(JSON.stringify({ nodes: nodes, edges: edges }));
      if (checked.problems.length) throw new Error('The chart could not be drawn.');
      var rendered = window.yavcCanvas.render(checked.doc, { palette: colours, pad: 0, measure: false, font: getComputedStyle(root).fontFamily, title: 'Teams, groups and free agents' });
      var svg = new DOMParser().parseFromString(rendered.svg, 'image/svg+xml').documentElement;
      svg.classList.add('org-chart-lines'); svg.setAttribute('aria-hidden', 'true');
      var old = canvas.querySelector('.org-chart-lines'); if (old) old.replaceWith(svg); else canvas.prepend(svg);
    }
    function menuRows() { return ['Assign to a Team…', 'Make Team Lead', 'Create a Team…', 'Create a Group…', 'Start Chat', 'Delete Chat…'].map(function (label) { return { label: label, disabled: true, detail: 'Coming next' }; }); }
    function makeChat(p) {
      var box = node('article', 'org-chart-chat'); box.dataset.orgChat = p.id; box.tabIndex = 0; place(box, p);
      var title = node('strong', 'org-chart-title'), meta = node('span', 'org-chart-meta text-muted'), state = node('span', 'org-chart-state');
      box.append(title, meta, state); canvas.append(box); elements.set(p.id, { box: box, title: title, meta: meta, state: state });
      box.addEventListener('pudl:contextmenu', function (event) {
        var chat = snapshot.chats.find(function (c) { return c.chatId === p.id; });
        if (!chat || chat.resolution !== 'resolved') { event.preventDefault(); event.stopPropagation(); return; }
        event.detail.add(menuRows()); event.detail.mark(box); event.stopPropagation();
      });
    }
    function updateText() {
      snapshot.chats.forEach(function (chat) {
        var el = elements.get(chat.chatId); if (!el) return;
        el.title.textContent = chat.title || 'Untitled Chat'; el.title.title = el.title.textContent;
        el.meta.textContent = (chat.workspaceName || chat.workspace) + ' / ' + (chat.providerName || chat.provider || 'Unknown program'); el.meta.title = el.meta.textContent;
        var repair = chat.resolution !== 'resolved';
        var associations = snapshot.teams.filter(function (t) { return t.resolution === 'resolved' && (t.leadId === chat.chatId || t.memberIds.includes(chat.chatId)); }).map(function (t) { return (t.leadId === chat.chatId ? 'Leads ' : 'Member of ') + t.title; }).concat(snapshot.groups.filter(function (g) { return g.resolution === 'resolved' && (g.ownerId === chat.chatId || g.memberIds.includes(chat.chatId)); }).map(function (g) { return (g.ownerId === chat.chatId ? 'Leads group ' : 'Group member of ') + g.title; }));
        el.box.setAttribute('aria-label', el.title.textContent + '. ' + el.meta.textContent + '. ' + associations.join('. ') + (repair ? '. Needs attention. ' + chat.reason : ''));
        el.state.replaceChildren(repair ? node('span', 'org-chart-repair', '⚠ Needs attention') : window.dinahChatState.badge(chat.state, { labelled: true }));
        el.state.title = repair ? chat.reason : '';
        var button = el.box.querySelector('[data-row-actions]');
        if (repair && button) button.remove();
        if (!repair && !button) { button = node('button'); button.type = 'button'; button.setAttribute('data-row-actions', ''); button.setAttribute('aria-label', 'Actions for ' + el.title.textContent); el.box.append(button); }
        if (button) button.setAttribute('aria-label', 'Actions for ' + el.title.textContent);
      });
      canvas.querySelectorAll('[data-org-fold]').forEach(function (button) { var team = snapshot.teams.find(function (t) { return t.id === button.dataset.orgFold; }); if (team) { button.textContent = (button.getAttribute('aria-expanded') === 'false' ? '▸ ' : '▾ ') + team.title + (button.getAttribute('aria-expanded') === 'false' ? ' (' + button.dataset.count + ' chats)' : ''); button.title = team.title; } });
      canvas.querySelectorAll('[data-org-group]').forEach(function (box) {
        var group = snapshot.groups.find(function (g) { return g.id === box.dataset.orgGroup; }) || snapshot.teams.find(function (t) { return t.id === box.dataset.orgGroup; }); if (!group) return;
        var owner = snapshot.chats.find(function (c) { return c.chatId === group.ownerId; });
        box.children[0].textContent = group.title; box.children[0].title = group.title;
        box.children[1].textContent = group.resolution === 'repair' ? '⚠ Needs attention' : owner ? 'Led by ' + owner.title : 'Group';
        box.children[2].textContent = group.resolution === 'repair' ? group.reason : group.memberIds.length + (group.memberIds.length === 1 ? ' member' : ' members');
        box.title = group.reason || group.title + (owner ? '. Led by ' + owner.title : '');
      });
    }
    function render() {
      if (disposed || !snapshot || !scroll.clientWidth) return;
      var settings = options(), next = window.dinahOrgChartLayout.signature(snapshot, settings);
      if (next !== geometry) {
        geometry = next; drawing = window.dinahOrgChartLayout.layout(snapshot, settings); elements.clear(); canvas.replaceChildren();
        canvas.style.width = drawing.width + 'px'; canvas.style.height = drawing.height + 'px';
        drawLines();
        drawing.positions.forEach(function (p) {
          if (p.kind === 'chat') { makeChat(p); return; }
          var box = node('section', 'org-chart-group'); box.dataset.orgGroup = p.sourceId; place(box, p); box.append(node('strong', 'org-chart-title'), node('span', 'org-chart-meta text-muted'), node('span', 'org-chart-meta text-muted')); canvas.append(box);
        });
        drawing.controls.forEach(function (p) {
          var button = node('button', 'btn btn-sm org-chart-fold'); button.type = 'button'; button.dataset.orgFold = p.id; button.dataset.count = p.count; button.setAttribute('aria-expanded', String(!p.folded)); place(button, p);
          button.addEventListener('click', function () { if (folded.has(p.id)) folded.delete(p.id); else folded.add(p.id); changed(); var control = canvas.querySelector('[data-org-fold="' + CSS.escape(p.id) + '"]'); if (control) control.focus({ preventScroll: true }); }); canvas.append(button);
        });
        drawing.frames.filter(function (p) { return !snapshot.teams.some(function (t) { return t.id === p.id; }); }).forEach(function (p) { var label = node('h2', 'org-chart-section', p.title); label.style.left = p.x + 12 + 'px'; label.style.top = p.y + 5 + 'px'; canvas.append(label); });
      }
      updateText();
      canvas.dataset.generation = snapshot.generation;
      var visible = elements.size, summary = visible ? visible + (visible === 1 ? ' chat is shown.' : ' chats are shown.') : drawing.controls.length ? 'Teams are folded.' : 'No chats match these filters.';
      if (status.textContent !== summary) status.textContent = summary;
    }
    function workspaceOptions() {
      var values = new Map(snapshot.chats.map(function (c) { return [c.workspace, c.workspaceName || c.workspace]; }));
      if (workspace && !values.has(workspace)) values.set(workspace, workspace);
      var choices = [['', 'All Workspaces']].concat(Array.from(values).sort(function (a, b) { return a[1].localeCompare(b[1]); }));
      if (JSON.stringify(choices) !== scope.dataset.choices) { scope.replaceChildren(); choices.forEach(function (pair) { var option = node('option', '', pair[1]); option.value = pair[0]; scope.append(option); }); scope.dataset.choices = JSON.stringify(choices); } scope.value = workspace;
    }
    function load() {
      if (disposed) return Promise.resolve();
      if (pending) { again = true; return pending; }
      pending = Promise.resolve().then(function () { if (!app() || !app().OrgChart) throw new Error('The chart is not available in this build.'); return app().OrgChart(); }).then(function (view) {
        if (disposed) return;
        if (!view || typeof view.generation !== 'string' || !Array.isArray(view.chats) || !Array.isArray(view.teams) || !Array.isArray(view.groups)) throw new Error('The chart could not be read.');
        snapshot = view; workspaceOptions(); render();
      }).catch(function (error) { if (!disposed) { status.textContent = window.dinahHelpers.message(error, 'sentence'); status.title = status.textContent; } }).finally(function () { pending = null; if (again && !disposed) { again = false; return load(); } });
      return pending;
    }
    scope.addEventListener('change', function () { workspace = scope.value; changed(); }); search.addEventListener('input', function () { find = search.value; changed(); }); refresh.addEventListener('click', load);
    var stops = ['agent:chatstate', 'team:state', 'bus:changed', 'chat:removed', 'chat:restored', 'workspace:changed'].map(function (event) { return window.dinahHost.on(event, load); });
    stops.push(window.dinahHost.onResync(load));
    var resize = new ResizeObserver(render); resize.observe(scroll);
    var theme = new MutationObserver(function () { if (!disposed) drawLines(); }); theme.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'data-skin', 'style'] });
    readState(opts.state); load();
    return { state: address, setState: function (value) { readingState = true; readState(value); render(); readingState = false; }, commands: function () { return [{ label: 'Refresh Chart', run: load }]; }, destroy: function () { disposed = true; stops.forEach(function (stop) { stop(); }); resize.disconnect(); theme.disconnect(); } };
  }
  window.dinahWorkbench.register('org-chart', { applet: { id: 'dinah.org-chart', title: 'Org Chart', glyph: 'subagents', workspace: false }, singleton: true, title: 'Org Chart', glyph: 'subagents', mode: 'maximized', init: init });
  window.dinahCommands.register({ id: 'org-chart', menu: 'window', label: 'Org Chart', run: function () { return window.dinahWorkbench.open('org-chart'); } });
}());
