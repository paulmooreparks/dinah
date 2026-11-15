/* Geometry for the read-only chart. Execution state never enters this calculation. */
(function () {
  'use strict';
  // Use clear horizontal and vertical corridors between the reserved boxes.
  // Diagrams still draws every segment with its documented straight route.
  function routes(plan) {
    var obstacles = plan.positions.concat(plan.controls).map(function (p) { return { left: Math.floor(p.x - 6), right: Math.ceil(p.x + p.width + 6), top: Math.floor(p.y - 6), bottom: Math.ceil(p.y + p.height + 6) }; });
    function port(p, side) { return side === 'bottom' ? [Math.round(p.x + p.width / 2), Math.ceil(p.y + p.height + 6)] : side === 'top' ? [Math.round(p.x + p.width / 2), Math.floor(p.y - 6)] : [side === 'left' ? Math.floor(p.x - 6) : Math.ceil(p.x + p.width + 6), Math.round(p.y + p.height / 2)]; }
    var groupIds = Array.from(new Set(plan.edges.filter(function (e) { return e.dotted; }).map(function (e) { return e.to.id; }))).sort(), occupied = [];
    var links = plan.edges.map(function (e) {
      var side = e.dotted ? groupIds.indexOf(e.to.id) % 2 ? 'left' : 'right' : 'bottom', start = port(e.from, side);
      if (e.dotted) {
        var memberships = plan.edges.filter(function (other) { return other.dotted && other.from === e.from; }).map(function (other) { return other.to.id; }).sort();
        start[1] = Math.round(e.from.y + e.from.height * (memberships.indexOf(e.to.id) + 1) / (memberships.length + 1));
        var frame = plan.frames.find(function (f) { return e.from.x >= f.x && e.from.y >= f.y && e.from.x + e.from.width <= f.x + f.width && e.from.y + e.from.height <= f.y + f.height; });
        if (frame && plan.positions.concat(plan.controls).some(function (p) { return p !== e.from && start[1] > p.y - 6 && start[1] < p.y + p.height + 6 && (side === 'right' ? p.x >= e.from.x + e.from.width && p.x < frame.x + frame.width : p.x + p.width <= e.from.x && p.x + p.width > frame.x); })) { side = side === 'left' ? 'right' : 'left'; start[0] = port(e.from, side)[0]; }
      }
      return Object.assign({}, e, { fromSide: side, toSide: e.dotted ? 'right' : 'top', start: start, end: port(e.to, e.dotted ? 'right' : 'top') });
    });
    var xs = new Set([0, plan.width]), ys = new Set([0, plan.height]);
    obstacles.forEach(function (o) { xs.add(o.left); xs.add(o.right); ys.add(o.top); ys.add(o.bottom); });
    plan.frames.forEach(function (f) { groupIds.forEach(function (_, i) { xs.add(Math.round(f.x - 16 - i * 16)); xs.add(Math.round(f.x + f.width + 16 + i * 16)); ys.add(Math.round(f.y - 16 - i * 16)); ys.add(Math.round(f.y + f.height + 16 + i * 16)); }); [f.x - 8, f.x + 8, f.x + f.width - 8, f.x + f.width + 8].forEach(function (x) { xs.add(Math.round(x)); }); [f.y - 8, f.y + 8, f.y + f.height - 8, f.y + f.height + 8].forEach(function (y) { ys.add(Math.round(y)); }); });
    links.forEach(function (e) { xs.add(e.start[0]); xs.add(e.end[0]); ys.add(e.start[1]); ys.add(e.end[1]); });
    xs = Array.from(xs).filter(function (x) { return x >= 0 && x <= plan.width; }).sort(function (a, b) { return a - b; }); ys = Array.from(ys).filter(function (y) { return y >= 0 && y <= plan.height; }).sort(function (a, b) { return a - b; });
    var columns = xs.length, count = columns * ys.length, clear = new Map();
    function point(index) { return [xs[index % columns], ys[Math.floor(index / columns)]]; }
    function free(a, b, edge) {
      var ownFrame = edge.dotted && plan.frames.find(function (f) { return edge.from.x >= f.x && edge.from.y >= f.y && edge.from.x + edge.from.width <= f.x + f.width && edge.from.y + edge.from.height <= f.y + f.height; });
      var key = JSON.stringify([edge.id, Math.min(a, b), Math.max(a, b)]); if (clear.has(key)) return clear.get(key);
      var first = point(a), last = point(b), horizontal = first[1] === last[1];
      var allowed = !obstacles.some(function (o) { return horizontal ? first[1] > o.top && first[1] < o.bottom && Math.max(first[0], last[0]) > o.left && Math.min(first[0], last[0]) < o.right : first[0] > o.left && first[0] < o.right && Math.max(first[1], last[1]) > o.top && Math.min(first[1], last[1]) < o.bottom; });
      if (allowed && edge.dotted) allowed = !plan.frames.some(function (f) {
        if (!(f === ownFrame && horizontal && first[1] === edge.start[1] && (edge.fromSide === 'right' ? Math.min(first[0], last[0]) >= edge.start[0] : Math.max(first[0], last[0]) <= edge.start[0])) && (horizontal ? first[1] > f.y - 6 && first[1] < f.y + f.height + 6 && Math.max(first[0], last[0]) > f.x - 6 && Math.min(first[0], last[0]) < f.x + f.width + 6 : first[0] > f.x - 6 && first[0] < f.x + f.width + 6 && Math.max(first[1], last[1]) > f.y - 6 && Math.min(first[1], last[1]) < f.y + f.height + 6)) return true;
        // Perpendicular crossings leave a container at one point. Parallel
        // segments stay clear of its border, so they cannot look like it.
        return horizontal ? (Math.abs(first[1] - f.y) < 16 || Math.abs(first[1] - f.y - f.height) < 16) && Math.max(first[0], last[0]) > f.x - 8 && Math.min(first[0], last[0]) < f.x + f.width + 8 : (Math.abs(first[0] - f.x) < 16 || Math.abs(first[0] - f.x - f.width) < 16) && Math.max(first[1], last[1]) > f.y - 8 && Math.min(first[1], last[1]) < f.y + f.height + 8;
      });
      if (allowed && edge.dotted) allowed = !occupied.some(function (r) {
        if (r.group === edge.to.id || horizontal !== r.horizontal) return false;
        return horizontal ? Math.abs(first[1] - r.a[1]) < 16 && Math.min(Math.max(first[0],last[0]),Math.max(r.a[0],r.b[0])) > Math.max(Math.min(first[0],last[0]),Math.min(r.a[0],r.b[0])) : Math.abs(first[0] - r.a[0]) < 16 && Math.min(Math.max(first[1],last[1]),Math.max(r.a[1],r.b[1])) > Math.max(Math.min(first[1],last[1]),Math.min(r.a[1],r.b[1]));
      });
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
        neighbors.forEach(function (next) { if (previous[next] === -2 && free(current, next, edge)) { previous[next] = current; queue[last++] = next; } });
      }
      if (previous[end] === -2) throw new Error('A chart connection could not be drawn without crossing a box.');
      var points = [], cursor = end;
      while (cursor !== -1) { points.push(point(cursor)); cursor = previous[cursor]; } points.reverse();
      edge.points = points.filter(function (p, index) { if (!index || index === points.length - 1) return true; var a = points[index - 1], b = points[index + 1]; return !(a[0] === p[0] && p[0] === b[0] || a[1] === p[1] && p[1] === b[1]); });
      if (edge.dotted) for (var i = 1; i < edge.points.length; i++) occupied.push({ group: edge.to.id, a: edge.points[i - 1], b: edge.points[i], horizontal: edge.points[i - 1][1] === edge.points[i][1] });
      clear.clear(); return edge;
    });
  }
  function layout(view, options) {
    var width = Math.max(160, Math.floor(options.width)), folded = new Set(options.folded || []);
    var chats = new Map(view.chats.map(function (c) { return [c.chatId, c]; }));
    var wanted = new Set(view.chats.filter(function (c) {
      return (!options.workspace || !c.workspace || c.workspace.toLowerCase().replace(/[\\/]+$/, '') === options.workspace.toLowerCase().replace(/[\\/]+$/, '')) && (!options.find || c.title.toLowerCase().includes(options.find.toLowerCase()));
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
      if (claimed.has(id) || !chats.has(id) || !wanted.has(id)) return;
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
      card(id, x + (w - cardW) / 2, y, cardW);
      if (!children.length) return y + 112;
      y += 132;
      var columns = w >= 288 && children.length > 1 ? 2 : 1, childW = (w - (columns - 1) * 24) / columns;
      for (var i = 0; i < children.length; i += columns) {
        var bottom = y;
        children.slice(i, i + columns).forEach(function (child, index) {
          bottom = Math.max(bottom, tree(child, x + index * (childW + 24), y, childW, seen));
          var a = anchors.get(id), b = anchors.get(child);
          if (a && b && a !== b) edges.push({ id: 'team:' + id + ':' + child, from: a, to: b, dotted: false });
        });
        y = bottom + 16;
      }
      return y;
    }
    var roots = teams.filter(function (t) { return !parent.has(t.leadId) && visible(t.leadId); }).sort(function (a, b) { return a.id.localeCompare(b.id); });
    var groups = view.groups.filter(function (g) { return g.resolution === 'repair' && !options.workspace && !options.find || g.memberIds.concat(g.ownerId || []).some(function (id) { return wanted.has(id); }); });
    // Both sides provide lanes. Reserve readable cards before adding more gutter.
    var columns = width >= 650 ? 2 : 1, margin = Math.min(16 + Math.ceil(groups.length / 2) * 16, Math.max(16, Math.floor((width - 144 * columns) / (2 * columns))));
    var gutter = margin * 2, sectionW = (width - margin * 2 - gutter * (columns - 1)) / columns, y = margin;
    var groupBoxes = new Map();
    function placeGroups(ready) {
      var next = groups.filter(function (g) { return !groupBoxes.has(g.id) && ready(g); }).sort(function (a, b) { return a.id.localeCompare(b.id); });
      for (var index = 0; index < next.length; index += columns) {
        next.slice(index, index + columns).forEach(function (g, slot) {
          var w = Math.min(320, sectionW - 16), box = { id: 'group:' + g.id, sourceId: g.id, kind: 'group', x: margin + slot * (sectionW + gutter) + (sectionW - w) / 2, y: y, width: w, height: 100 };
          positions.push(box); groupBoxes.set(g.id, box);
        }); y += 124;
      }
    }
    for (var r = 0; r < roots.length; r += columns) {
      var bottom = y;
      roots.slice(r, r + columns).forEach(function (t, index) {
        var x = margin + index * (sectionW + gutter), end = tree(t.leadId, x + 12, y + 20, sectionW - 24, new Set());
        frames.push({ id: t.id, title: t.title, x: x, y: y, width: sectionW, height: end - y + 8 }); bottom = Math.max(bottom, end + 28);
      }); y = bottom + 12;
      placeGroups(function (g) { return g.memberIds.concat(g.ownerId || []).some(function (id) { return claimed.has(id); }); });
    }
    var grouped = new Set();
    view.groups.filter(function (g) { return g.resolution === 'resolved'; }).forEach(function (g) { g.memberIds.concat(g.ownerId || []).forEach(function (id) { grouped.add(id); }); });
    function remaining(title, predicate) {
      var rows = view.chats.filter(function (c) { return wanted.has(c.chatId) && !claimed.has(c.chatId) && predicate(c); }).sort(function (a, b) { return a.chatId.localeCompare(b.chatId); });
      if (!rows.length) return;
      var start = y, cols = width >= 850 ? 4 : width >= 350 ? 2 : 1, w = (width - margin * 2 - 24 - (cols - 1) * 16) / cols;
      rows.forEach(function (c, index) { card(c.chatId, margin + 12 + index % cols * (w + 16), start + 30 + Math.floor(index / cols) * 120, w); });
      y += 50 + Math.ceil(rows.length / cols) * 120;
      frames.push({ id: title, title: title, x: margin, y: start, width: width - margin * 2, height: y - start - 16 }); y += 24;
    }
    remaining('Group members', function (c) { return c.resolution === 'resolved' && grouped.has(c.chatId); });
    placeGroups(function () { return true; });
    remaining('Free agents', function (c) { return c.resolution === 'resolved' && !parent.has(c.chatId) && !byLead.has(c.chatId); });
    remaining('Needs attention', function () { return true; });
    view.teams.filter(function (t) { return t.resolution === 'repair' && (!options.workspace && !options.find || t.memberIds.concat(t.leadId || []).some(function (id) { return wanted.has(id); })); }).forEach(function (t) {
      var w = Math.min(320, width - 32); positions.push({ id: 'repair:' + t.id, sourceId: t.id, kind: 'team-repair', x: (width - w) / 2, y: y, width: w, height: 100 }); y += 124;
    });
    groups.forEach(function (g, index) {
      var box = groupBoxes.get(g.id);
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
