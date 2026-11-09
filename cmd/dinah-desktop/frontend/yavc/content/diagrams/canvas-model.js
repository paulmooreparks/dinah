/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* JSON Canvas edits and gesture geometry. The input is never changed. Every
   result passes the same reader as a file before it can replace the text. */
(function (root) {
    'use strict';
    var J = typeof module === 'object' && module.exports ? require('../diagram-kit/jcanvas.js') : root.yavcCanvas;
    function copy(v) { return JSON.parse(JSON.stringify(v)); }
    function node(doc, id) { return doc.nodes.find(function (n) { return n.id === id; }); }
    function integer(v) { if (!Number.isSafeInteger(v)) { throw new Error('Give a whole number within the safe numeric range.'); } return v; }
    function idMaker(doc) {
        var taken = new Set(doc.nodes.concat(doc.edges).map(function (n) { return n.id; })), next = Object.create(null);
        return function (prefix) {
            var i = next[prefix] || 1, id = prefix;
            while (taken.has(id)) { id = prefix + '-' + (++i); }
            next[prefix] = i; taken.add(id); return id;
        };
    }
    function inside(n, g) { return n.id !== g.id && n.x >= g.x && n.y >= g.y && n.x + n.width <= g.x + g.width && n.y + n.height <= g.y + g.height; }
    function members(doc, id) {
        var g = node(doc, id);
        return g && g.type === 'group' ? doc.nodes.filter(function (n) { return inside(n, g); }).map(function (n) { return n.id; }) : [];
    }
    function properties(item, kind) {
        var shared = ['color', 'line', 'lineWidth'];
        return shared.concat(kind === 'edge' ? ['label', 'route', 'fromHead', 'toHead', 'fromSide', 'toSide'] :
            ['x', 'y', 'width', 'height', 'fillColor', 'strokeColor'].concat(item.type === 'text' ? ['text', 'shape', 'textAlign', 'textVerticalAlign'] : item.type === 'group' ? ['label'] : []));
    }
    function setProperty(item, kind, property, value) {
        if (properties(item, kind).indexOf(property) < 0 || ['x', 'y', 'width', 'height'].indexOf(property) >= 0) { throw new Error('That property cannot be changed on this item.'); }
        var target = ['shape', 'route', 'fromHead', 'toHead', 'line', 'lineWidth', 'textAlign', 'textVerticalAlign', 'fillColor', 'strokeColor'].indexOf(property) >= 0 ? (item.yavc || (item.yavc = {})) : item;
        if (value === null || (value === '' && property !== 'text')) { delete target[property]; } else { target[property] = value; }
        if (kind === 'edge' && ['fromSide','toSide'].indexOf(property) >= 0 && item.yavc) { delete item.yavc[property.slice(0,-4) + 'Auto']; }
    }
    function movingIds(doc, selected) {
        var ids = new Set();
        selected.filter(function (s) { return s.kind === 'node'; }).forEach(function (s) { ids.add(s.id); members(doc, s.id).forEach(function (id) { ids.add(id); }); });
        return Array.from(ids);
    }
    function editSelection(doc, req) {
        if (!Array.isArray(req.items) || !req.items.length || req.items.length > 5000) { throw new Error('Choose one or more canvas items.'); }
        var selected = [], seen = new Set();
        req.items.forEach(function (s) {
            if (!s || ['node', 'edge'].indexOf(s.kind) < 0 || typeof s.id !== 'string') { throw new Error('Each selected item needs a kind and ID.'); }
            var item = (s.kind === 'node' ? doc.nodes : doc.edges).find(function (n) { return n.id === s.id; });
            if (!item || (s.kind === 'node' && J.isAnchor(item))) { throw new Error('That selected item is no longer available.'); }
            var key = JSON.stringify([s.kind, s.id]);
            if (!seen.has(key)) { seen.add(key); selected.push({ id: s.id, kind: s.kind }); }
        });
        if (req.op === 'align' || req.op === 'distribute') {
            arrange(doc, selected, req);
        } else if (req.op === 'move') {
            var ids = new Set(movingIds(doc, selected)), dx = integer(req.dx), dy = integer(req.dy);
            if (!ids.size) { throw new Error('Select a node to move.'); }
            doc.nodes.forEach(function (n) { if (ids.has(n.id)) { n.x = integer(n.x + dx); n.y = integer(n.y + dy); } });
        } else if (req.op === 'set' || req.op === 'resize') {
            selected.forEach(function (s) {
                var item = (s.kind === 'node' ? doc.nodes : doc.edges).find(function (n) { return n.id === s.id; });
                if (req.op === 'set') { setProperty(item, s.kind, req.property, req.value); }
                else {
                    if (s.kind !== 'node') { throw new Error('Only nodes have geometry.'); }
                    ['x', 'y', 'width', 'height'].forEach(function (k) { if (req[k] != null) { item[k] = integer(req[k]); } });
                    if (item.width < 1 || item.height < 1) { throw new Error('A resized node must be at least one pixel wide and high.'); }
                }
            });
        } else if (req.op === 'delete') {
            var nodes = new Set(selected.filter(function (s) { return s.kind === 'node'; }).map(function (s) { return s.id; }));
            var edges = new Set(selected.filter(function (s) { return s.kind === 'edge'; }).map(function (s) { return s.id; }));
            doc.nodes = doc.nodes.filter(function (n) { return !nodes.has(n.id); });
            doc.edges = doc.edges.filter(function (e) { return !edges.has(e.id) && !nodes.has(e.fromNode) && !nodes.has(e.toNode); });
            pruneAnchors(doc); selected = [];
        } else { throw new Error('This operation requires a single item.'); }
        var result = checked(doc); result.selection = selected; return result;
    }
    function arrange(doc, selected, req) {
        if (selected.some(function (s) { return s.kind !== 'node'; })) { throw new Error('Choose only nodes to align or distribute.'); }
        var chosen = selected.map(function (s) { return node(doc, s.id); });
        /* A selected group is one unit, including any selected descendants. */
        var units = chosen.filter(function (n) { return !chosen.some(function (g) { return g.type === 'group' && inside(n, g); }); });
        if (units.length < (req.op === 'align' ? 2 : 3)) { throw new Error('Choose ' + (req.op === 'align' ? 'two' : 'three') + ' independent nodes or groups.'); }
        var positions = new Map(), axis, size;
        if (req.op === 'align') {
            var modes = { left: ['x', 'width', 0], center: ['x', 'width', 0.5], right: ['x', 'width', 1], top: ['y', 'height', 0], middle: ['y', 'height', 0.5], bottom: ['y', 'height', 1] };
            var mode = modes[req.direction];
            if (!mode) { throw new Error('Choose a supported alignment.'); }
            axis = mode[0]; size = mode[1];
            var lo = Math.min.apply(null, units.map(function (n) { return n[axis]; }));
            var hi = Math.max.apply(null, units.map(function (n) { return n[axis] + n[size]; }));
            var at = lo + (hi - lo) * mode[2];
            units.forEach(function (n) { positions.set(n.id, Math.round(at - n[size] * mode[2])); });
        } else {
            if (['horizontal', 'vertical'].indexOf(req.direction) < 0) { throw new Error('Choose horizontal or vertical distribution.'); }
            axis = req.direction === 'horizontal' ? 'x' : 'y'; size = axis === 'x' ? 'width' : 'height';
            units.sort(function (a, b) { return a[axis] - b[axis] || doc.nodes.indexOf(a) - doc.nodes.indexOf(b); });
            var start = units[0][axis], last = units[units.length - 1];
            var gap = (last[axis] + last[size] - start - units.reduce(function (sum, n) { return sum + n[size]; }, 0)) / (units.length - 1);
            units.forEach(function (n) { positions.set(n.id, Math.round(start)); start += n[size] + gap; });
        }
        var moves = new Map();
        units.forEach(function (n) {
            var delta = positions.get(n.id) - n[axis];
            [n.id].concat(members(doc, n.id)).forEach(function (id) {
                if (moves.has(id) && moves.get(id) !== delta) { throw new Error('Overlapping groups cannot be arranged together.'); }
                moves.set(id, delta);
            });
        });
        doc.nodes.forEach(function (n) { if (moves.has(n.id)) { n[axis] = integer(n[axis] + moves.get(n.id)); } });
    }
    function pruneAnchors(doc) {
        var used = new Set();
        doc.edges.forEach(function (e) { used.add(e.fromNode); used.add(e.toNode); });
        doc.nodes = doc.nodes.filter(function (n) { return !J.isAnchor(n) || used.has(n.id); });
    }
    /* A pointer near a side-centre pins that side. The rest of the node
       means automatic attachment, including a drop near its centre. */
    function attachment(doc, point, tolerance, exclude) {
        var result = null;
        doc.nodes.slice().reverse().some(function (n) {
            if (n.id === exclude || J.isAnchor(n) || point.x < n.x - tolerance || point.x > n.x + n.width + tolerance || point.y < n.y - tolerance || point.y > n.y + n.height + tolerance) { return false; }
            var inside = point.x >= n.x && point.x <= n.x + n.width && point.y >= n.y && point.y <= n.y + n.height;
            var centre = point.x >= n.x + n.width / 4 && point.x <= n.x + n.width * 3 / 4 && point.y >= n.y + n.height / 4 && point.y <= n.y + n.height * 3 / 4;
            var spots = [['top', n.x + n.width / 2, n.y], ['right', n.x + n.width, n.y + n.height / 2], ['bottom', n.x + n.width / 2, n.y + n.height], ['left', n.x, n.y + n.height / 2]];
            spots.sort(function (a, b) { return Math.hypot(a[1] - point.x, a[2] - point.y) - Math.hypot(b[1] - point.x, b[2] - point.y); });
            var near = Math.hypot(spots[0][1] - point.x, spots[0][2] - point.y) <= tolerance;
            if (!inside && !near) { return false; }
            result = { node: n.id, side: near && !centre ? spots[0][0] : null }; return true;
        });
        return result || { node: null, x: Math.round(point.x), y: Math.round(point.y) };
    }
    function checked(doc) {
        var text = J.write(doc), got = J.read(text), problem = got.problems.find(function (p) { return !p.warning; });
        if (!got.doc || problem) { throw new Error(problem ? problem.message : 'The canvas is invalid.'); }
        return { doc: got.doc, text: text };
    }
    /* Snapping uses screen distance, converted by the view to file units.
       Shape guides win over the grid when they are within that distance. */
    function snap(box, nodes, ignore, tolerance, off, resize) {
        var b = Object.assign({}, box), guides = [];
        if (off) { return { box: b, guides: guides }; }
        [['x', 'width'], ['y', 'height']].forEach(function (axis) {
            var a = axis[0], size = axis[1], factors = resize ? [1] : [0, 0.5, 1], best = null;
            nodes.forEach(function (n) {
                if (J.isAnchor(n) || ignore.indexOf(n.id) >= 0) { return; }
                [0, 0.5, 1].forEach(function (f) {
                    var at = n[a] + n[size] * f;
                    factors.forEach(function (q) {
                        var d = at - (b[a] + b[size] * q);
                        if (Math.abs(d) <= tolerance && (!best || Math.abs(d) < Math.abs(best.d))) { best = { d: d, at: at }; }
                    });
                });
            });
            if (resize) { b[size] = Math.max(1, Math.round(b[size] + (best ? best.d : Math.round((b[a] + b[size]) / 8) * 8 - b[a] - b[size]))); }
            else { b[a] = Math.round(best ? b[a] + best.d : Math.round(b[a] / 8) * 8); }
            if (best) { guides.push({ axis: a, at: best.at }); }
        });
        return { box: b, guides: guides };
    }
    function resize(box, handle, dx, dy, ratio) {
        var b = Object.assign({}, box), west = handle.indexOf('w') >= 0, north = handle.indexOf('n') >= 0;
        if (/[ew]/.test(handle)) { b.width = Math.max(1, Math.round(box.width + (west ? -dx : dx))); }
        if (/[ns]/.test(handle)) { b.height = Math.max(1, Math.round(box.height + (north ? -dy : dy))); }
        if (ratio && box.width > 0 && box.height > 0) {
            var r = box.width / box.height;
            if (!/[ew]/.test(handle) || (/[ns]/.test(handle) && Math.abs(dy / box.height) > Math.abs(dx / box.width))) { b.width = Math.max(1, Math.round(b.height * r)); }
            else { b.height = Math.max(1, Math.round(b.width / r)); }
        }
        b.x = west ? box.x + box.width - b.width : box.x;
        b.y = north ? box.y + box.height - b.height : box.y;
        return b;
    }
    function edit(source, req) {
        if (req.op === 'document') {
            if (['background', 'grid', 'snap'].indexOf(req.property) < 0 || req.items !== undefined) { throw new Error('Choose a supported diagram setting.'); }
            var changed = copy(source), settings = changed.yavc || (changed.yavc = { version: 1 });
            if (req.value === null) { delete settings[req.property]; } else { settings[req.property] = req.value; }
            return checked(changed);
        }
        if (req.items !== undefined) { return editSelection(copy(source), req); }
        var doc = copy(source), key = req.key || '', n = node(doc, key), e = doc.edges.find(function (x) { return x.id === key; }), item = req.kind === 'edge' ? e : req.kind === 'node' ? n : n || e;
        var freeId = idMaker(doc);
        function needNode() { if (!n || req.kind === 'edge') { throw new Error('Select a node first.'); } }
        function needItem() { if (!item) { throw new Error('That node or connector is no longer here.'); } }
        function endpoint(edge, end, target, side, x, y) {
            if (side != null && ['top', 'right', 'bottom', 'left'].indexOf(side) < 0) { throw new Error('Choose a valid connector side or Automatic.'); }
            if (target != null && target !== '') {
                if (!node(doc, target)) { throw new Error('That endpoint node is no longer here.'); }
                edge[end + 'Node'] = target;
            } else {
                var point = { id: freeId('anchor'), type: 'text', x: integer(x), y: integer(y), width: 0, height: 0, text: '', yavc: { shape: 'text', anchor: true } };
                doc.nodes.push(point); edge[end + 'Node'] = point.id;
            }
            if (side) { edge[end + 'Side'] = side; } else { delete edge[end + 'Side']; }
            if (edge.yavc) { delete edge.yavc[end + 'Auto']; }
        }
        if (req.op === 'move') {
            needNode();
            var dx = integer(req.dx), dy = integer(req.dy), moving = [key].concat(members(doc, key));
            doc.nodes.forEach(function (x) { if (moving.indexOf(x.id) >= 0) { x.x = integer(Math.round(x.x + dx)); x.y = integer(Math.round(x.y + dy)); } });
        } else if (req.op === 'resize') {
            needNode();
            ['x', 'y', 'width', 'height'].forEach(function (k) { if (req[k] != null) { n[k] = integer(req[k]); } });
            if (n.width < 1 || n.height < 1) { throw new Error('A resized node must be at least one pixel wide and high.'); }
        } else if (req.op === 'create') {
            var type = req.type || 'text';
            n = { id: freeId(type === 'group' ? 'group' : 'node'), type: type, x: integer(req.x == null ? 0 : req.x), y: integer(req.y == null ? 0 : req.y), width: 180, height: 80 };
            if (type === 'group') { n.label = 'Group'; n.width = 400; n.height = 240; }
            else if (type === 'text') { n.text = req.text == null ? 'New shape' : req.text; n.yavc = { shape: req.shape || 'rectangle' }; }
            else { throw new Error('New nodes must be text nodes or groups.'); }
            if (type === 'group') { doc.nodes.unshift(n); } else { doc.nodes.push(n); }
            key = n.id;
        } else if (req.op === 'connect') {
            if (!node(doc, req.from)) { throw new Error('Choose a source node for the connector.'); }
            e = { id: freeId('edge'), fromNode: req.from, toEnd: 'arrow' };
            endpoint(e, 'from', req.from, req.fromSide);
            endpoint(e, 'to', req.to, req.toSide, req.x, req.y);
            doc.edges.push(e); key = e.id;
        } else if (req.op === 'reconnect') {
            if (!e || req.kind === 'node') { throw new Error('Select a connector first.'); }
            if (req.end !== 'from' && req.end !== 'to') { throw new Error('Choose the start or end of the connector.'); }
            endpoint(e, req.end, req.node, req.side, req.x, req.y);
            pruneAnchors(doc);
        } else if (req.op === 'set') {
            needItem();
            setProperty(item, item === n ? 'node' : 'edge', req.property, req.value);
        } else if (req.op === 'delete') {
            needItem();
            if (item === n) {
                doc.nodes = doc.nodes.filter(function (x) { return x.id !== key; });
                doc.edges = doc.edges.filter(function (x) { return x.fromNode !== key && x.toNode !== key; });
            } else { doc.edges = doc.edges.filter(function (x) { return x.id !== key; }); }
            pruneAnchors(doc);
            key = '';
        } else if (req.op === 'order') {
            needNode();
            if (req.direction !== undefined && ['forward', 'back'].indexOf(req.direction) < 0) { throw new Error('Choose forward or back for drawing order.'); }
            var at = doc.nodes.indexOf(n), to = Math.max(0, Math.min(doc.nodes.length - 1, at + (req.direction === 'back' ? -1 : 1)));
            doc.nodes.splice(at, 1); doc.nodes.splice(to, 0, n);
        } else if (req.op === 'duplicate' || req.op === 'paste') {
            if (req.op === 'duplicate') { needNode(); }
            var fragment = req.op === 'paste' ? J.read(req.text) : { doc: fragmentOf(doc, key, req.kind) };
            if (!fragment.doc || (fragment.problems || []).some(function (p) { return !p.warning; })) { throw new Error('The clipboard does not hold a valid canvas selection.'); }
            var part = copy(fragment.doc), map = Object.create(null), offset = req.offset == null ? 24 : integer(req.offset);
            if (doc.nodes.length + doc.edges.length + part.nodes.length + part.edges.length > J.LIMITS.items) { throw new Error('Pasting would exceed the ' + J.LIMITS.items + ' node and connector limit.'); }
            part.nodes.forEach(function (x) {
                var old = x.id; x.id = freeId('node'); map[old] = x.id;
                x.x = integer(x.x + offset); x.y = integer(x.y + offset); doc.nodes.push(x);
            });
            part.edges.forEach(function (x) {
                if (!map[x.fromNode] || !map[x.toNode]) { return; }
                x.id = freeId('edge');
                ['from','to'].forEach(function (end) { var auto = J.automatic(x, end); x[end + 'Node'] = map[x[end + 'Node']]; if (auto) { delete x[end + 'Side']; } if (x.yavc) { delete x.yavc[end + 'Auto']; } });
                doc.edges.push(x);
            });
            key = part.nodes.length ? part.nodes[0].id : '';
        } else { throw new Error('Unknown canvas operation.'); }
        var result = checked(doc); result.key = key; return result;
    }
    function fragmentOf(doc, key, kind) {
        var ids = kind === 'edge' ? [] : [key].concat(members(doc, key));
        doc.edges.forEach(function (e) {
            if (ids.indexOf(e.fromNode) >= 0 && J.isAnchor(node(doc, e.toNode))) { ids.push(e.toNode); }
            if (ids.indexOf(e.toNode) >= 0 && J.isAnchor(node(doc, e.fromNode))) { ids.push(e.fromNode); }
        });
        return copy({ nodes: doc.nodes.filter(function (n) { return ids.indexOf(n.id) >= 0; }), edges: doc.edges.filter(function (e) { return ids.indexOf(e.fromNode) >= 0 && ids.indexOf(e.toNode) >= 0; }) });
    }
    function template(name) {
        var doc = { nodes: [], edges: [], yavc: { version: 1 } };
        function text(id, x, y, value, shape) { doc.nodes.push({ id: id, type: 'text', x: x, y: y, width: 180, height: 80, text: value, yavc: { shape: shape || 'rectangle', textAlign: 'center', textVerticalAlign: 'middle' } }); }
        function edge(id, from, to, label) { doc.edges.push({ id: id, fromNode: from, toNode: to, label: label || '' }); }
        function point(id, x, y) { doc.nodes.push({ id: id, type: 'text', x: x, y: y, width: 0, height: 0, text: '', yavc: { shape: 'text', anchor: true } }); }
        if (name === 'blank') { return checked(doc); }
        if (name === 'basic' || name === 'canvas' || name == null) {
            text('start', 0, 0, '**Start** here', 'rounded'); text('next', 300, 0, 'Then this'); edge('start-next', 'start', 'next', 'leads to');
        } else if (name === 'flowchart') {
            text('start', 0, 0, 'Start', 'rounded'); text('process', 0, 160, 'Process'); text('decision', 0, 320, 'Continue?', 'diamond');
            text('yes', 300, 320, 'Next Step'); text('end', 0, 500, 'End', 'rounded');
            edge('start-process', 'start', 'process'); edge('process-decision', 'process', 'decision'); edge('decision-yes', 'decision', 'yes', 'Yes'); edge('decision-end', 'decision', 'end', 'No');
        } else if (name === 'sequence') {
            ['client', 'service'].forEach(function (id, i) {
                var x = i * 320;
                doc.nodes.push({ id: id + '-lane', type: 'group', x: x - 12, y: -24, width: 204, height: 520 });
                text(id, x, 0, i ? 'Service' : 'Client'); point(id + '-end', x + 90, 460);
                edge(id + '-lifeline', id, id + '-end'); Object.assign(doc.edges[doc.edges.length - 1], { fromSide: 'bottom', yavc: { route: 'straight', line: 'dashed', toHead: 'none' } });
            });
            point('request-from', 90, 180); point('request-to', 410, 180); edge('request', 'request-from', 'request-to', 'Request');
            point('response-from', 410, 320); point('response-to', 90, 320); edge('response', 'response-from', 'response-to', 'Response');
            doc.edges.slice(-2).forEach(function (e, i) { e.yavc = { route: 'straight', line: i ? 'dashed' : 'solid' }; });
        } else { throw new Error('Choose Blank, Basic, Flowchart or Sequence.'); }
        return checked(doc);
    }
    var api = Object.freeze({ edit: edit, template: template, snap: snap, resize: resize, members: members, movingIds: movingIds, properties: properties, fragment: fragmentOf, attachment: attachment });
    if (typeof module === 'object' && module.exports) { module.exports = api; } else { root.yavcCanvasModel = api; }
})(typeof window !== 'undefined' ? window : this);
