/* Geometry for the read-only chart. Execution state never enters this calculation. */
(function () {
  'use strict';
  // Use clear horizontal and vertical corridors between the reserved boxes.
  // Diagrams still draws every segment with its documented straight route.
  function routes(plan) {
    var obstacles = plan.positions.concat(plan.controls).map(function (p) { return { left: Math.floor(p.x - 6), right: Math.ceil(p.x + p.width + 6), top: Math.floor(p.y - 6), bottom: Math.ceil(p.y + p.height + 6) }; });
    function port(p, side) { return side === 'bottom' ? [Math.round(p.x + p.width / 2), Math.ceil(p.y + p.height + 6)] : side === 'top' ? [Math.round(p.x + p.width / 2), Math.floor(p.y - 6)] : [Math.ceil(p.x + p.width + 6), Math.round(p.y + p.height / 2)]; }
    var links = plan.edges.map(function (e) { return Object.assign({}, e, { fromSide: e.dotted ? 'right' : 'bottom', toSide: e.dotted ? 'right' : 'top', start: port(e.from, e.dotted ? 'right' : 'bottom'), end: port(e.to, e.dotted ? 'right' : 'top') }); });
    var xs = new Set([0, plan.width]), ys = new Set([0, plan.height]);
    obstacles.forEach(function (o) { xs.add(o.left); xs.add(o.right); ys.add(o.top); ys.add(o.bottom); });
    links.forEach(function (e) { xs.add(e.start[0]); xs.add(e.end[0]); ys.add(e.start[1]); ys.add(e.end[1]); });
    xs = Array.from(xs).sort(function (a, b) { return a - b; }); ys = Array.from(ys).sort(function (a, b) { return a - b; });
    var columns = xs.length, count = columns * ys.length, clear = new Map();
    function point(index) { return [xs[index % columns], ys[Math.floor(index / columns)]]; }
    function free(a, b) {
      var key = Math.min(a, b) + ':' + Math.max(a, b); if (clear.has(key)) return clear.get(key);
      var first = point(a), last = point(b), horizontal = first[1] === last[1];
      var allowed = !obstacles.some(function (o) { return horizontal ? first[1] > o.top && first[1] < o.bottom && Math.max(first[0], last[0]) > o.left && Math.min(first[0], last[0]) < o.right : first[0] > o.left && first[0] < o.right && Math.max(first[1], last[1]) > o.top && Math.min(first[1], last[1]) < o.bottom; });
      clear.set(key, allowed); return allowed;
    }
    return links.map(function (edge) {
      var start = ys.indexOf(edge.start[1]) * columns + xs.indexOf(edge.start[0]), end = ys.indexOf(edge.end[1]) * columns + xs.indexOf(edge.end[0]);
      var previous = new Int32Array(count); previous.fill(-2); previous[start] = -1;
      var queue = new Int32Array(count), first = 0, last = 1; queue[0] = start;
      while (first < last && previous[end] === -2) {
        var current = queue[first++], x = current % columns, y = Math.floor(current / columns), neighbors = [];
        if (x > 0) neighbors.push(current - 1); if (x + 1 < columns) neighbors.push(current + 1);
        if (y > 0) neighbors.push(current - columns); if (y + 1 < ys.length) neighbors.push(current + columns);
        neighbors.forEach(function (next) { if (previous[next] === -2 && free(current, next)) { previous[next] = current; queue[last++] = next; } });
      }
      if (previous[end] === -2) throw new Error('A chart connection could not be drawn without crossing a box.');
      var points = [], cursor = end;
      while (cursor !== -1) { points.push(point(cursor)); cursor = previous[cursor]; } points.reverse();
      edge.points = points.filter(function (p, index) { if (!index || index === points.length - 1) return true; var a = points[index - 1], b = points[index + 1]; return !(a[0] === p[0] && p[0] === b[0] || a[1] === p[1] && p[1] === b[1]); });
      return edge;
    });
  }
  function layout(view, options) {
    var width = Math.max(160, Math.floor(options.width)), folded = new Set(options.folded || []);
    var chats = new Map(view.chats.map(function (c) { return [c.chatId, c]; }));
    var wanted = new Set(view.chats.filter(function (c) {
      return (!options.workspace || c.workspace === options.workspace) && (!options.find || (c.title + ' ' + c.workspaceName + ' ' + c.providerName).toLocaleLowerCase().includes(options.find.toLocaleLowerCase()));
    }).map(function (c) { return c.chatId; }));
    var teams = view.teams.filter(function (t) { return t.resolution === 'resolved'; });
    var byLead = new Map(), parent = new Map(), members = new Map(), claimed = new Set(), positions = [], frames = [], controls = [], edges = [], anchors = new Map();
    teams.forEach(function (t) { byLead.set(t.leadId, t); });
    teams.forEach(function (t) { members.set(t.leadId, t.memberIds.filter(function (id) { return chats.has(id) && id !== t.leadId; })); t.memberIds.forEach(function (id) { parent.set(id, t.leadId); }); });
    function descendants(id, seen) {
      if (seen.has(id)) return [];
      seen.add(id); return [id].concat((members.get(id) || []).flatMap(function (child) { return descendants(child, seen); }));
    }
    function visible(id) { return descendants(id, new Set()).some(function (key) { return wanted.has(key); }); }
    function card(id, x, y, w) {
      if (claimed.has(id) || !chats.has(id)) return;
      claimed.add(id); var p = { id: id, kind: 'chat', x: x, y: y, width: w, height: 100 }; positions.push(p); anchors.set(id, p);
    }
    function tree(id, x, y, w, seen) {
      if (seen.has(id) || claimed.has(id)) return y;
      seen = new Set(seen); seen.add(id);
      var team = byLead.get(id), start = y, children = (members.get(id) || []).filter(visible), cardW = Math.min(210, w);
      if (team) {
        var hidden = folded.has(team.id);
        controls.push({ id: team.id, leadId: id, x: x, y: y, width: w, height: 36, folded: hidden, count: descendants(id, new Set()).length });
        y += 48;
        if (hidden) {
          var proxy = { id: 'fold:' + team.id, kind: 'fold', x: x, y: start, width: w, height: 36 };
          descendants(id, new Set()).forEach(function (key) { if (!claimed.has(key)) { claimed.add(key); anchors.set(key, proxy); } });
          return y;
        }
      }
      card(id, x + (w - cardW) / 2, y, cardW); y += 132;
      var columns = w >= 280 && children.length > 1 ? 2 : 1, childW = (w - (columns - 1) * 24) / columns;
      for (var i = 0; i < children.length; i += columns) {
        var bottom = y;
        children.slice(i, i + columns).forEach(function (child, index) {
          bottom = Math.max(bottom, tree(child, x + index * (childW + 24), y, childW, seen));
          var a = anchors.get(id), b = anchors.get(child);
          if (a && b && a !== b) edges.push({ id: 'team:' + id + ':' + child, from: a, to: b, dotted: false });
        });
        y = bottom + 16;
      }
      return Math.max(start + 148, y);
    }
    var roots = teams.filter(function (t) { return !parent.has(t.leadId) && visible(t.leadId); }).sort(function (a, b) { return a.id.localeCompare(b.id); });
    var columns = width >= 650 ? 2 : 1, gutter = 24, sectionW = (width - gutter * (columns - 1)) / columns, y = 20;
    for (var r = 0; r < roots.length; r += columns) {
      var bottom = y;
      roots.slice(r, r + columns).forEach(function (t, index) {
        var x = index * (sectionW + gutter), end = tree(t.leadId, x + 12, y + 20, sectionW - 24, new Set());
        frames.push({ id: t.id, title: t.title, x: x, y: y, width: sectionW, height: end - y + 8 }); bottom = Math.max(bottom, end + 28);
      }); y = bottom + 28;
    }
    var grouped = new Set();
    view.groups.filter(function (g) { return g.resolution === 'resolved'; }).forEach(function (g) { g.memberIds.concat(g.ownerId || []).forEach(function (id) { grouped.add(id); }); });
    function remaining(title, predicate) {
      var rows = view.chats.filter(function (c) { return wanted.has(c.chatId) && !claimed.has(c.chatId) && predicate(c); }).sort(function (a, b) { return a.chatId.localeCompare(b.chatId); });
      if (!rows.length) return;
      var start = y, cols = width >= 850 ? 4 : width >= 350 ? 2 : 1, w = (width - 32 - (cols - 1) * 16) / cols;
      rows.forEach(function (c, index) { card(c.chatId, 16 + index % cols * (w + 16), start + 30 + Math.floor(index / cols) * 120, w); });
      y += 50 + Math.ceil(rows.length / cols) * 120;
      frames.push({ id: title, title: title, x: 0, y: start, width: width, height: y - start - 16 }); y += 24;
    }
    remaining('Group members', function (c) { return c.resolution === 'resolved' && grouped.has(c.chatId); });
    remaining('Free agents', function (c) { return c.resolution === 'resolved' && !parent.has(c.chatId) && !byLead.has(c.chatId); });
    remaining('Needs attention', function () { return true; });
    var groups = view.groups.filter(function (g) { return g.resolution === 'repair' || g.memberIds.concat(g.ownerId || []).some(function (id) { return wanted.has(id); }); });
    var repairs = view.teams.filter(function (t) { return t.resolution === 'repair'; }).map(function (t) { return Object.assign({ kind: 'team-repair' }, t); });
    groups.concat(repairs).sort(function (a, b) { return a.id.localeCompare(b.id); }).forEach(function (g, index) {
      var w = Math.min(320, width - 32), box = { id: 'group:' + g.id, sourceId: g.id, kind: g.kind || 'group', x: (width - w) / 2, y: y, width: w, height: 100 };
      positions.push(box); y += 124;
      if (g.resolution !== 'resolved') return;
      var linked = new Set();
      g.memberIds.concat(g.ownerId || []).forEach(function (id) {
        var a = anchors.get(id), chat = chats.get(id);
        if (!a || !chat || chat.resolution !== 'resolved' || linked.has(a.id)) return;
        linked.add(a.id); edges.push({ id: 'group:' + index + ':' + id, from: a, to: box, dotted: true });
      });
    });
    var plan = { width: width, height: Math.max(160, y), positions: positions, frames: frames, controls: controls, edges: edges };
    plan.routes = routes(plan); return plan;
  }
  function signature(view, options) {
    return JSON.stringify([Math.floor(options.width), options.workspace, options.find, options.folded.slice().sort(),
      view.chats.map(function (c) { return [c.chatId, c.workspace, c.resolution]; }).sort(),
      view.teams.map(function (t) { return [t.id, t.leadId, t.memberIds.slice().sort(), t.resolution]; }).sort(),
      view.groups.map(function (g) { return [g.id, g.ownerId, g.memberIds.slice().sort(), g.resolution]; }).sort()]);
  }
  window.dinahOrgChartLayout = { layout: layout, signature: signature };
}());
