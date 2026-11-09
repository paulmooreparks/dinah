/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* JSON Canvas for Diagrams (docs/canvas-design.md): reading a .canvas file
   against JSON Canvas 1.0 and the site's extension (docs/canvas-extension.md),
   writing it in one stable form, routing its connectors, and drawing it as
   SVG of the site's own, made from numbers and escaped text.

     yavcCanvas.read(text)        -> { doc, problems: [{ message, at }] }
     yavcCanvas.write(doc)        -> the file's text, in the stable form
     yavcCanvas.render(doc, opts) -> { svg, width, height, boxes, edges, origin }
     yavcCanvas.route(edge, nodes)-> { points, curve, from, to }

   It needs no DOM, so the same code draws for the screen, for a picture,
   and for the tests (apps/diagram-kit/test/jcanvas.test.mjs). */
(function (root) {
    'use strict';

    var MAX_TEXT = 2 * 1024 * 1024, MAX_ITEMS = 5000;
    var TYPES = ['text', 'file', 'link', 'group'];
    var SIDES = ['top', 'right', 'bottom', 'left'];
    var ENDS = ['none', 'arrow'];
    var BACKGROUND_STYLES = ['cover', 'ratio', 'repeat'];
    var SHAPES = ['rectangle', 'rounded', 'oval', 'diamond', 'parallelogram', 'document', 'cylinder', 'queue', 'person', 'cloud', 'hexagon', 'package', 'text'];
    var ROUTES = ['elbow', 'straight', 'curve'];
    var HEADS = ['none', 'arrow', 'triangle', 'diamond', 'circle', 'box', 'cross', 'cf-one', 'cf-one-required', 'cf-many', 'cf-many-required'];
    var LINES = ['solid', 'dashed', 'dotted'];
    var EXT_VERSION = 1;
    /* The order the specification lists each object's properties in, which
       the writer keeps, so a change shows in a diff as the lines it changed. */
    var NODE_KEYS = ['id', 'type', 'x', 'y', 'width', 'height', 'color', 'text', 'file', 'subpath', 'url', 'label', 'background', 'backgroundStyle'];
    var EDGE_KEYS = ['id', 'fromNode', 'fromSide', 'fromEnd', 'toNode', 'toSide', 'toEnd', 'color', 'label'];

    function esc(s) { // check-ignore(one-shared-implementation): the module needs no DOM and runs under Node, where yavcUtil.esc is not loaded
        return String(s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;').replace(/'/g, '&#39;');
    }
    function isObj(v) { return v !== null && typeof v === 'object' && !Array.isArray(v); }
    function isAnchor(n) { return !!(n && n.yavc && n.yavc.anchor === true); }
    function routes(edges, nodes) {
        var by = Object.create(null); nodes.forEach(function (n) { by[n.id] = n; });
        return edges.map(function (e) { return { id: e.id, route: route(e, by) }; });
    }
    function isColour(v) { return typeof v === 'string' && (/^#[0-9a-fA-F]{6}$/.test(v) || /^#[0-9a-fA-F]{3}$/.test(v) || /^[1-6]$/.test(v)); }

    /* === Reading ======================================================== */

    /* A JSON parse error's place, as an offset in the text, where the
       browser says one. */
    function jsonPlace(err, text) {
        var m = /position (\d+)/i.exec(err && err.message || '');
        if (m) { return +m[1]; }
        m = /line (\d+) column (\d+)/i.exec(err && err.message || '');
        if (m) {
            var lines = text.split('\n'), at = 0;
            for (var i = 0; i < +m[1] - 1 && i < lines.length; i++) { at += lines[i].length + 1; }
            return at + (+m[2]) - 1;
        }
        return null;
    }

    function read(text) {
        text = String(text == null ? '' : text);
        var problems = [];
        function fail(message, at) { problems.push({ message: message, at: at == null ? null : at }); return { doc: null, problems: problems }; }
        if (text.length > MAX_TEXT) { return fail('The canvas is over 2 MB, which is more than Diagrams will read.'); }
        if (!text.trim()) { return { doc: { nodes: [], edges: [] }, problems: problems }; }
        var doc;
        try { doc = JSON.parse(text); } catch (err) { return fail('The canvas is not valid JSON: ' + err.message.replace(/^JSON\.parse: /, ''), jsonPlace(err, text)); }
        if (!isObj(doc)) { return fail('A canvas is a JSON object, with nodes and edges.'); }
        var nodes = doc.nodes == null ? [] : doc.nodes, edges = doc.edges == null ? [] : doc.edges;
        if (!Array.isArray(nodes)) { return fail('nodes must be an array.'); }
        if (!Array.isArray(edges)) { return fail('edges must be an array.'); }
        if (nodes.length + edges.length > MAX_ITEMS) { return fail('The canvas has ' + (nodes.length + edges.length) + ' nodes and edges, which is more than the ' + MAX_ITEMS + ' Diagrams will read.'); }
        var ids = Object.create(null);
        for (var i = 0; i < nodes.length; i++) {
            var p = nodeProblem(nodes[i], i);
            if (p) { return fail(p, placeOfId(text, nodes[i])); }
            if (ids[nodes[i].id]) { return fail('Two nodes have the id "' + nodes[i].id + '".', placeOfId(text, nodes[i])); }
            ids[nodes[i].id] = true;
        }
        var edgeIds = Object.create(null);
        for (var j = 0; j < edges.length; j++) {
            var q = edgeProblem(edges[j], j, ids);
            if (q) { return fail(q, placeOfId(text, edges[j])); }
            if (edgeIds[edges[j].id]) { return fail('Two edges have the id "' + edges[j].id + '".', placeOfId(text, edges[j])); }
            edgeIds[edges[j].id] = true;
        }
        if (doc.yavc != null) {
            if (!isObj(doc.yavc)) { return fail('The file\'s yavc property must be an object.'); }
            if (doc.yavc.version != null && !(Number.isInteger(doc.yavc.version) && doc.yavc.version >= 1)) { return fail('The extension\'s version must be a whole number from 1.'); }
            if (doc.yavc.background != null && !isColour(doc.yavc.background)) { return fail('The canvas background must be a hex color or preset from 1 to 6.'); }
            if (doc.yavc.grid != null && typeof doc.yavc.grid !== 'boolean') { return fail('The canvas grid setting must be a boolean.'); }
            if (doc.yavc.snap != null && typeof doc.yavc.snap !== 'boolean') { return fail('The canvas snap setting must be a boolean.'); }
            if (doc.yavc.version > EXT_VERSION) { problems.push({ message: 'This canvas was written by a newer Diagrams (extension version ' + doc.yavc.version + '); what this one does not know is drawn plainly.', at: null, warning: true }); }
        }
        doc.nodes = nodes; doc.edges = edges;
        return { doc: doc, problems: problems };
    }
    /* Where an object's id is written, to mark a problem there. */
    function placeOfId(text, obj) {
        if (!obj || typeof obj.id !== 'string') { return null; }
        var at = text.indexOf('"' + obj.id.replace(/["\\]/g, '\\$&') + '"');
        return at >= 0 ? at : null;
    }
    function nodeProblem(n, i) {
        var where = 'Node ' + (i + 1);
        if (!isObj(n)) { return where + ' is not an object.'; }
        if (typeof n.id !== 'string' || !n.id) { return where + ' has no id.'; }
        where = 'Node "' + n.id + '"';
        if (TYPES.indexOf(n.type) < 0) { return where + ' has the type "' + n.type + '", which is not one of ' + TYPES.join(', ') + '.'; }
        var dims = ['x', 'y', 'width', 'height'];
        for (var k = 0; k < dims.length; k++) {
            if (typeof n[dims[k]] !== 'number' || !isFinite(n[dims[k]])) { return where + ' needs a number for ' + dims[k] + '.'; }
        }
        if (n.width < 0 || n.height < 0) { return where + ' has a negative size.'; }
        if (n.color != null && !isColour(n.color)) { return where + '\'s color must be a hex colour such as #ff0000, or a preset from 1 to 6.'; }
        if (n.type === 'text' && typeof n.text !== 'string') { return where + ' is a text node without text.'; }
        if (n.type === 'file') {
            if (typeof n.file !== 'string') { return where + ' is a file node without a file.'; }
            if (n.subpath != null && !(typeof n.subpath === 'string' && n.subpath.charAt(0) === '#')) { return where + '\'s subpath must start with #.'; }
        }
        if (n.type === 'link' && typeof n.url !== 'string') { return where + ' is a link node without a url.'; }
        if (n.type === 'group') {
            if (n.label != null && typeof n.label !== 'string') { return where + '\'s label must be text.'; }
            if (n.background != null && typeof n.background !== 'string') { return where + '\'s background must be a path.'; }
            if (n.backgroundStyle != null && BACKGROUND_STYLES.indexOf(n.backgroundStyle) < 0) { return where + '\'s backgroundStyle must be one of ' + BACKGROUND_STYLES.join(', ') + '.'; }
        }
        if (n.yavc != null) {
            if (!isObj(n.yavc)) { return where + '\'s yavc property must be an object.'; }
            if (n.yavc.fillColor != null && !isColour(n.yavc.fillColor)) { return where + '\'s fillColor must be a hex color or preset from 1 to 6.'; }
            if (n.yavc.strokeColor != null && !isColour(n.yavc.strokeColor)) { return where + '\'s strokeColor must be a hex color or preset from 1 to 6.'; }
            if (n.yavc.anchor != null && typeof n.yavc.anchor !== 'boolean') { return where + '\'s anchor must be a boolean.'; }
            if (isAnchor(n) && (n.type !== 'text' || n.text !== '' || n.width !== 0 || n.height !== 0 || n.yavc.shape !== 'text' || !Number.isSafeInteger(n.x) || !Number.isSafeInteger(n.y))) { return where + ' is a point anchor and must be empty text with shape text, zero size and integer coordinates.'; }
            if (n.yavc.shape != null && SHAPES.indexOf(n.yavc.shape) < 0) { return where + '\'s shape "' + n.yavc.shape + '" is not one of ' + SHAPES.join(', ') + '.'; }
            if (n.yavc.textAlign != null && (n.type !== 'text' || ['left', 'center', 'right'].indexOf(n.yavc.textAlign) < 0)) { return where + '\'s textAlign must be left, center or right on a text node.'; }
            if (n.yavc.textVerticalAlign != null && (n.type !== 'text' || ['top', 'middle', 'bottom'].indexOf(n.yavc.textVerticalAlign) < 0)) { return where + '\'s textVerticalAlign must be top, middle or bottom on a text node.'; }
            var lp = lineProblem(n.yavc);
            if (lp) { return where + lp; }
        }
        return null;
    }
    function lineProblem(y) {
        if (y.line != null && LINES.indexOf(y.line) < 0) { return '\'s line must be one of ' + LINES.join(', ') + '.'; }
        if (y.lineWidth != null && !(Number.isInteger(y.lineWidth) && y.lineWidth >= 1 && y.lineWidth <= 8)) { return '\'s lineWidth must be a whole number from 1 to 8.'; }
        return null;
    }
    function edgeProblem(e, i, ids) {
        var where = 'Edge ' + (i + 1);
        if (!isObj(e)) { return where + ' is not an object.'; }
        if (typeof e.id !== 'string' || !e.id) { return where + ' has no id.'; }
        where = 'Edge "' + e.id + '"';
        if (typeof e.fromNode !== 'string' || !ids[e.fromNode]) { return where + ' starts at "' + e.fromNode + '", which is not a node in the canvas.'; }
        if (typeof e.toNode !== 'string' || !ids[e.toNode]) { return where + ' ends at "' + e.toNode + '", which is not a node in the canvas.'; }
        if (e.fromSide != null && SIDES.indexOf(e.fromSide) < 0) { return where + '\'s fromSide must be one of ' + SIDES.join(', ') + '.'; }
        if (e.toSide != null && SIDES.indexOf(e.toSide) < 0) { return where + '\'s toSide must be one of ' + SIDES.join(', ') + '.'; }
        if (e.fromEnd != null && ENDS.indexOf(e.fromEnd) < 0) { return where + '\'s fromEnd must be none or arrow.'; }
        if (e.toEnd != null && ENDS.indexOf(e.toEnd) < 0) { return where + '\'s toEnd must be none or arrow.'; }
        if (e.color != null && !isColour(e.color)) { return where + '\'s color must be a hex colour such as #ff0000, or a preset from 1 to 6.'; }
        if (e.label != null && typeof e.label !== 'string') { return where + '\'s label must be text.'; }
        if (e.yavc != null) {
            if (!isObj(e.yavc)) { return where + '\'s yavc property must be an object.'; }
            if (e.yavc.route != null && ROUTES.indexOf(e.yavc.route) < 0) { return where + '\'s route must be one of ' + ROUTES.join(', ') + '.'; }
            if (e.yavc.fromHead != null && HEADS.indexOf(e.yavc.fromHead) < 0) { return where + '\'s fromHead must be one of ' + HEADS.join(', ') + '.'; }
            if (e.yavc.toHead != null && HEADS.indexOf(e.yavc.toHead) < 0) { return where + '\'s toHead must be one of ' + HEADS.join(', ') + '.'; }
            for (var end of ['from', 'to']) {
                var cached = e.yavc[end + 'Auto'];
                if (cached != null && (!isObj(cached) || typeof cached.node !== 'string' || !cached.node || SIDES.indexOf(cached.side) < 0)) { return where + '\'s ' + end + 'Auto must contain a node ID and a valid side.'; }
            }
            var lp = lineProblem(e.yavc);
            if (lp) { return where + lp; }
        }
        return null;
    }

    /* === Writing ======================================================== */

    /* The writer keeps the specification's ends in agreement with the
       extension's heads, so a file stripped of the extension still points
       its arrows the same way (docs/canvas-extension.md). */
    function agree(e) {
        if (!isObj(e.yavc)) { return e; }
        if (e.yavc.fromHead != null) { e.fromEnd = e.yavc.fromHead === 'none' ? 'none' : 'arrow'; }
        if (e.yavc.toHead != null) { e.toEnd = e.yavc.toHead === 'none' ? 'none' : 'arrow'; }
        return e;
    }
    function ordered(obj, keys) {
        var out = Object.create(null);
        keys.forEach(function (k) { if (obj[k] !== undefined) { out[k] = obj[k]; } });
        Object.keys(obj).forEach(function (k) { if (keys.indexOf(k) < 0 && k !== 'yavc' && obj[k] !== undefined) { out[k] = obj[k]; } });
        if (obj.yavc !== undefined) { out.yavc = obj.yavc; }
        return out;
    }
    /* The stable form: nodes, then edges, then anything else the file had;
       each object's properties in the specification's order, then the
       others as they were, then the extension; two spaces of indent. */
    function automatic(e, end) {
        var side = e[end + 'Side'], cached = isObj(e.yavc) && e.yavc[end + 'Auto'];
        return side == null || !!(isObj(cached) && cached.node === e[end + 'Node'] && cached.side === side);
    }
    function projected(e, byId) {
        var copy = Object.assign(Object.create(null), e), y = isObj(e.yavc) ? Object.assign(Object.create(null), e.yavc) : null;
        var a = byId[e.fromNode], b = byId[e.toNode], sides;
        if (a && b && [a, b].every(function (n) { return ['x','y','width','height'].every(function (key) { return Number.isFinite(n[key]); }); })) { sides = sidesFor(e, a, b); }
        ['from','to'].forEach(function (end, i) {
            if (automatic(e, end) && sides) {
                y = y || Object.create(null); copy[end + 'Side'] = sides[i];
                y[end + 'Auto'] = Object.assign(Object.create(null), isObj(y[end + 'Auto']) ? y[end + 'Auto'] : {}, {node:e[end + 'Node'],side:sides[i]});
            } else if (y) { delete y[end + 'Auto']; }
        });
        if (y) { copy.yavc = y; }
        return agree(copy);
    }
    function write(doc) {
        var top = Object.create(null), byId = Object.create(null);
        (doc.nodes || []).forEach(function (n) { if (isObj(n)) { byId[n.id] = n; } });
        top.nodes = (doc.nodes || []).map(function (n) { return ordered(n, NODE_KEYS); });
        top.edges = (doc.edges || []).map(function (e) { return ordered(projected(e, byId), EDGE_KEYS); });
        Object.keys(doc).forEach(function (k) { if (k !== 'nodes' && k !== 'edges' && k !== 'yavc') { top[k] = doc[k]; } });
        if (doc.yavc !== undefined) { top.yavc = doc.yavc; }
        return JSON.stringify(top, null, 2) + '\n';
    }

    /* === Routing ======================================================== */

    var NORMAL = { top: [0, -1], right: [1, 0], bottom: [0, 1], left: [-1, 0] };
    function sidePoint(n, side) {
        switch (side) {
            case 'top': return [n.x + n.width / 2, n.y];
            case 'bottom': return [n.x + n.width / 2, n.y + n.height];
            case 'left': return [n.x, n.y + n.height / 2];
            default: return [n.x + n.width, n.y + n.height / 2];
        }
    }
    /* The sides a connector uses: the file's, or the nearest pair. */
    function sidesFor(e, a, b) {
        var ax = a.x + a.width / 2, ay = a.y + a.height / 2, bx = b.x + b.width / 2, by = b.y + b.height / 2;
        var dx = bx - ax, dy = by - ay, horizontal = Math.abs(dx) >= Math.abs(dy);
        var fixedFrom = automatic(e, 'from') ? null : e.fromSide, fixedTo = automatic(e, 'to') ? null : e.toSide;
        var from = fixedFrom || (horizontal ? (dx >= 0 ? 'right' : 'left') : (dy >= 0 ? 'bottom' : 'top'));
        var to = fixedTo || (horizontal ? (dx >= 0 ? 'left' : 'right') : (dy >= 0 ? 'top' : 'bottom'));
        var froms = fixedFrom ? [from] : [from].concat(SIDES.filter(function (s) { return s !== from; }));
        var tos = fixedTo ? [to] : [to].concat(SIDES.filter(function (s) { return s !== to; }));
        var best = null, distance = Infinity;
        froms.forEach(function (f) { tos.forEach(function (t) {
            var p = sidePoint(a, f), q = sidePoint(b, t), d = Math.hypot(p[0] - q[0], p[1] - q[1]);
            if (d < distance) { distance = d; best = [f, t]; }
        }); });
        return best;
    }
    var STUB = 20;
    /* A connector's route. Elbow runs out from each side, then horizontally
       and vertically between them; straight joins the two side points;
       curve is one cubic Bezier segment leaving and arriving along the
       sides' normals. Nothing of it is stored (docs/canvas-design.md,
       decision 7). */
    function route(e, byId) {
        var a = byId[e.fromNode], b = byId[e.toNode];
        var sides = sidesFor(e, a, b), p0 = sidePoint(a, sides[0]), p3 = sidePoint(b, sides[1]);
        var n0 = NORMAL[sides[0]], n1 = NORMAL[sides[1]];
        var kind = (isObj(e.yavc) && e.yavc.route) || 'elbow';
        if (kind === 'straight') { return { points: [p0, p3], curve: false, from: sides[0], to: sides[1] }; }
        if (kind === 'curve') {
            var d = Math.max(40, Math.hypot(p3[0] - p0[0], p3[1] - p0[1]) / 3);
            return { points: [p0, [p0[0] + n0[0] * d, p0[1] + n0[1] * d], [p3[0] + n1[0] * d, p3[1] + n1[1] * d], p3], curve: true, from: sides[0], to: sides[1] };
        }
        var p1 = [p0[0] + n0[0] * STUB, p0[1] + n0[1] * STUB], p2 = [p3[0] + n1[0] * STUB, p3[1] + n1[1] * STUB];
        var h0 = n0[0] !== 0, h1 = n1[0] !== 0, mid;
        if (h0 && h1) { var mx = (p1[0] + p2[0]) / 2; mid = [[mx, p1[1]], [mx, p2[1]]]; }
        else if (!h0 && !h1) { var my = (p1[1] + p2[1]) / 2; mid = [[p1[0], my], [p2[0], my]]; }
        else if (h0) { mid = [[p2[0], p1[1]]]; }
        else { mid = [[p1[0], p2[1]]]; }
        var pts = [p0, p1].concat(mid, [p2, p3]);
        /* Points that repeat or lie on a straight run add nothing. */
        var clean = [pts[0]];
        for (var i = 1; i < pts.length; i++) {
            var q = pts[i], last = clean[clean.length - 1];
            if (Math.abs(q[0] - last[0]) < 0.01 && Math.abs(q[1] - last[1]) < 0.01) { continue; }
            if (clean.length >= 2) {
                var prev = clean[clean.length - 2];
                if ((Math.abs(prev[0] - last[0]) < 0.01 && Math.abs(last[0] - q[0]) < 0.01) || (Math.abs(prev[1] - last[1]) < 0.01 && Math.abs(last[1] - q[1]) < 0.01)) { clean[clean.length - 1] = q; continue; }
            }
            clean.push(q);
        }
        return { points: clean, curve: false, from: sides[0], to: sides[1] };
    }
    function pathD(r) {
        var p = r.points, f = function (q) { return q[0].toFixed(1) + ' ' + q[1].toFixed(1); };
        if (r.curve) { return 'M ' + f(p[0]) + ' C ' + f(p[1]) + ' ' + f(p[2]) + ' ' + f(p[3]); }
        return 'M ' + f(p[0]) + p.slice(1).map(function (q) { return ' L ' + f(q); }).join('');
    }
    /* The route's middle, for its label, and the direction it arrives in at
       each end, for its heads. */
    function midpoint(r) {
        var p = r.points;
        if (r.curve) {
            var t = 0.5, mt = 1 - t;
            return [mt * mt * mt * p[0][0] + 3 * mt * mt * t * p[1][0] + 3 * mt * t * t * p[2][0] + t * t * t * p[3][0],
                mt * mt * mt * p[0][1] + 3 * mt * mt * t * p[1][1] + 3 * mt * t * t * p[2][1] + t * t * t * p[3][1]];
        }
        var lens = [], total = 0;
        for (var i = 1; i < p.length; i++) { var l = Math.hypot(p[i][0] - p[i - 1][0], p[i][1] - p[i - 1][1]); lens.push(l); total += l; }
        var half = total / 2;
        for (var j = 0; j < lens.length; j++) {
            if (half <= lens[j] || j === lens.length - 1) { var k = lens[j] ? half / lens[j] : 0; return [p[j][0] + (p[j + 1][0] - p[j][0]) * k, p[j][1] + (p[j + 1][1] - p[j][1]) * k]; }
            half -= lens[j];
        }
        return p[0];
    }
    function endAngle(r, atStart) {
        var p = r.points, a = atStart ? p[1] : p[p.length - 2], b = atStart ? p[0] : p[p.length - 1];
        return Math.atan2(b[1] - a[1], b[0] - a[0]);
    }

    /* === Drawing ======================================================== */

    /* A shape's outline, drawn to fill its box (decision 10). */
    function shapePath(s, x, y, w, h) {
        var r = function (n) { return n.toFixed(1); };
        var X = x + w, Y = y + h, cx = x + w / 2, cy = y + h / 2;
        switch (s) {
            case 'rounded': var rr = Math.min(h / 2, 16); return { tag: 'rect', a: { x: x, y: y, width: w, height: h, rx: rr } };
            case 'oval': return { tag: 'ellipse', a: { cx: cx, cy: cy, rx: w / 2, ry: h / 2 } };
            case 'diamond': return { d: 'M ' + r(cx) + ' ' + r(y) + ' L ' + r(X) + ' ' + r(cy) + ' L ' + r(cx) + ' ' + r(Y) + ' L ' + r(x) + ' ' + r(cy) + ' Z' };
            case 'parallelogram': var k = Math.min(w * 0.15, h * 0.6); return { d: 'M ' + r(x + k) + ' ' + r(y) + ' L ' + r(X) + ' ' + r(y) + ' L ' + r(X - k) + ' ' + r(Y) + ' L ' + r(x) + ' ' + r(Y) + ' Z' };
            case 'hexagon': var e = Math.min(w * 0.18, h * 0.5); return { d: 'M ' + r(x + e) + ' ' + r(y) + ' L ' + r(X - e) + ' ' + r(y) + ' L ' + r(X) + ' ' + r(cy) + ' L ' + r(X - e) + ' ' + r(Y) + ' L ' + r(x + e) + ' ' + r(Y) + ' L ' + r(x) + ' ' + r(cy) + ' Z' };
            case 'document': var wv = Math.min(h * 0.12, 12); return { d: 'M ' + r(x) + ' ' + r(y) + ' L ' + r(X) + ' ' + r(y) + ' L ' + r(X) + ' ' + r(Y - wv) + ' C ' + r(x + w * 0.75) + ' ' + r(Y - wv * 3) + ' ' + r(x + w * 0.5) + ' ' + r(Y + wv) + ' ' + r(x + w * 0.25) + ' ' + r(Y - wv) + ' S ' + r(x) + ' ' + r(Y - wv) + ' ' + r(x) + ' ' + r(Y - wv) + ' Z' };
            case 'cylinder': var ry = Math.min(h * 0.12, 14); return { d: 'M ' + r(x) + ' ' + r(y + ry) + ' A ' + r(w / 2) + ' ' + r(ry) + ' 0 0 1 ' + r(X) + ' ' + r(y + ry) + ' L ' + r(X) + ' ' + r(Y - ry) + ' A ' + r(w / 2) + ' ' + r(ry) + ' 0 0 1 ' + r(x) + ' ' + r(Y - ry) + ' Z M ' + r(x) + ' ' + r(y + ry) + ' A ' + r(w / 2) + ' ' + r(ry) + ' 0 0 0 ' + r(X) + ' ' + r(y + ry) };
            case 'queue': var rx = Math.min(w * 0.08, 14); return { d: 'M ' + r(x + rx) + ' ' + r(y) + ' L ' + r(X - rx) + ' ' + r(y) + ' A ' + r(rx) + ' ' + r(h / 2) + ' 0 0 1 ' + r(X - rx) + ' ' + r(Y) + ' L ' + r(x + rx) + ' ' + r(Y) + ' A ' + r(rx) + ' ' + r(h / 2) + ' 0 0 1 ' + r(x + rx) + ' ' + r(y) + ' Z M ' + r(X - rx) + ' ' + r(y) + ' A ' + r(rx) + ' ' + r(h / 2) + ' 0 0 0 ' + r(X - rx) + ' ' + r(Y) };
            case 'person': var hr = Math.min(w, h) * 0.2; return { d: 'M ' + r(cx + hr) + ' ' + r(y + hr) + ' A ' + r(hr) + ' ' + r(hr) + ' 0 1 1 ' + r(cx - hr) + ' ' + r(y + hr) + ' A ' + r(hr) + ' ' + r(hr) + ' 0 1 1 ' + r(cx + hr) + ' ' + r(y + hr) + ' Z M ' + r(x) + ' ' + r(Y) + ' C ' + r(x) + ' ' + r(y + hr * 2.4) + ' ' + r(X) + ' ' + r(y + hr * 2.4) + ' ' + r(X) + ' ' + r(Y) + ' Z' };
            case 'cloud': return { d: 'M ' + r(x + w * 0.25) + ' ' + r(Y) + ' A ' + r(w * 0.22) + ' ' + r(h * 0.28) + ' 0 0 1 ' + r(x + w * 0.12) + ' ' + r(y + h * 0.45) + ' A ' + r(w * 0.22) + ' ' + r(h * 0.3) + ' 0 0 1 ' + r(x + w * 0.42) + ' ' + r(y + h * 0.15) + ' A ' + r(w * 0.25) + ' ' + r(h * 0.32) + ' 0 0 1 ' + r(x + w * 0.85) + ' ' + r(y + h * 0.35) + ' A ' + r(w * 0.2) + ' ' + r(h * 0.32) + ' 0 0 1 ' + r(x + w * 0.78) + ' ' + r(Y) + ' Z' };
            case 'package': var tw = Math.min(w * 0.4, 80), th = Math.min(h * 0.18, 14); return { d: 'M ' + r(x) + ' ' + r(y + th) + ' L ' + r(x) + ' ' + r(y) + ' L ' + r(x + tw) + ' ' + r(y) + ' L ' + r(x + tw) + ' ' + r(y + th) + ' L ' + r(X) + ' ' + r(y + th) + ' L ' + r(X) + ' ' + r(Y) + ' L ' + r(x) + ' ' + r(Y) + ' Z' };
            case 'text': return null;
            default: return { tag: 'rect', a: { x: x, y: y, width: w, height: h, rx: 4 } };
        }
    }
    function attrs(a) {
        return Object.keys(a).map(function (k) { var v = a[k]; return ' ' + k + '="' + esc(typeof v === 'number' ? +v.toFixed(2) : v) + '"'; }).join('');
    }
    function element(spec, extra) {
        if (!spec) { return ''; }
        if (spec.tag) { return '<' + spec.tag + attrs(Object.assign({}, spec.a, extra)) + '/>'; }
        return '<path' + attrs(Object.assign({ d: spec.d }, extra)) + '/>';
    }
    /* The colour of a preset or a hex, from the page's palette (decision 13:
       nothing reaches the drawing that is not a checked colour). */
    function colourOf(c, palette) {
        if (!c) { return null; }
        if (/^[1-6]$/.test(c)) { return palette.presets[+c - 1]; }
        return isColour(c) ? c : null;
    }
    function dashOf(y, w) {
        var line = isObj(y) ? y.line : null;
        return line === 'dashed' ? (w * 3) + ' ' + (w * 2) : line === 'dotted' ? w + ' ' + (w * 1.5) : null;
    }
    /* A connector's head, drawn at its point and pointing along its angle. */
    function headAt(kind, at, angle, colour, w, bg) {
        if (!kind || kind === 'none') { return ''; }
        var s = 6 + w * 2, c = Math.cos(angle), sn = Math.sin(angle);
        function pt(dx, dy) { return (at[0] + dx * c - dy * sn).toFixed(1) + ' ' + (at[1] + dx * sn + dy * c).toFixed(1); }
        var stroke = ' stroke="' + esc(colour) + '" stroke-width="' + w + '" stroke-linejoin="round"';
        switch (kind) {
            case 'arrow': return '<path d="M ' + pt(0, 0) + ' L ' + pt(-s * 1.6, -s * 0.7) + ' L ' + pt(-s * 1.6, s * 0.7) + ' Z" fill="' + esc(colour) + '"' + stroke + '/>';
            case 'triangle': return '<path d="M ' + pt(0, 0) + ' L ' + pt(-s * 1.6, -s * 0.7) + ' L ' + pt(-s * 1.6, s * 0.7) + ' Z" fill="' + esc(bg) + '"' + stroke + '/>';
            case 'diamond': return '<path d="M ' + pt(0, 0) + ' L ' + pt(-s, -s * 0.6) + ' L ' + pt(-s * 2, 0) + ' L ' + pt(-s, s * 0.6) + ' Z" fill="' + esc(bg) + '"' + stroke + '/>';
            case 'circle': var cc = pt(-s * 0.7, 0).split(' '); return '<circle cx="' + cc[0] + '" cy="' + cc[1] + '" r="' + (s * 0.7).toFixed(1) + '" fill="' + esc(bg) + '"' + stroke + '/>';
            case 'box': return '<path d="M ' + pt(0, -s * 0.6) + ' L ' + pt(-s * 1.2, -s * 0.6) + ' L ' + pt(-s * 1.2, s * 0.6) + ' L ' + pt(0, s * 0.6) + ' Z" fill="' + esc(bg) + '"' + stroke + '/>';
            case 'cross': return '<path d="M ' + pt(-s * 1.4, -s * 0.7) + ' L ' + pt(-s * 0.2, s * 0.7) + ' M ' + pt(-s * 1.4, s * 0.7) + ' L ' + pt(-s * 0.2, -s * 0.7) + '" fill="none"' + stroke + '/>';
            case 'cf-one': return '<path d="M ' + pt(-s, -s * 0.8) + ' L ' + pt(-s, s * 0.8) + '" fill="none"' + stroke + '/>';
            case 'cf-one-required': return '<path d="M ' + pt(-s * 0.8, -s * 0.8) + ' L ' + pt(-s * 0.8, s * 0.8) + ' M ' + pt(-s * 1.4, -s * 0.8) + ' L ' + pt(-s * 1.4, s * 0.8) + '" fill="none"' + stroke + '/>';
            case 'cf-many': return '<path d="M ' + pt(0, -s * 0.8) + ' L ' + pt(-s * 1.4, 0) + ' L ' + pt(0, s * 0.8) + '" fill="none"' + stroke + '/>';
            case 'cf-many-required': return '<path d="M ' + pt(0, -s * 0.8) + ' L ' + pt(-s * 1.4, 0) + ' L ' + pt(0, s * 0.8) + ' M ' + pt(-s * 1.7, -s * 0.8) + ' L ' + pt(-s * 1.7, s * 0.8) + '" fill="none"' + stroke + '/>';
        }
        return '';
    }
    /* Text as SVG text: paragraphs of plain words wrapped to the box, with
       Markdown's marks taken away. The screen draws a text node's Markdown
       properly over this (decision 6); a picture keeps this. */
    function plainLines(md) {
        return String(md).replace(/\r/g, '').split(/\n{2,}/).map(function (para) {
            return para.replace(/^#{1,6}\s+/gm, '').replace(/^\s*[-*+]\s+/gm, '• ').replace(/\*\*|__|`/g, '').replace(/(^|[^*])\*([^*]+)\*/g, '$1$2').replace(/\[([^\]]*)\]\([^)]*\)/g, '$1').replace(/\n/g, ' ').trim();
        }).filter(function (p) { return p; });
    }
    function wrap(text, width, size) {
        var per = Math.max(4, Math.floor(width / (size * 0.56))), out = [];
        text.split(/\s+/).forEach(function (word) {
            var last = out.length ? out[out.length - 1] : null;
            if (last !== null && (last + ' ' + word).length <= per) { out[out.length - 1] = last + ' ' + word; }
            else { while (word.length > per) { out.push(word.slice(0, per)); word = word.slice(per); } out.push(word); }
        });
        return out;
    }
    function textBlock(md, x, y, w, h, colour, size, align, vertical) {
        var lines = [];
        plainLines(md).forEach(function (p, i) { if (i) { lines.push(''); } lines = lines.concat(wrap(p, w - 16, size)); });
        var lh = size * 1.3, maxLines = Math.max(1, Math.floor((h - 8) / lh));
        if (lines.length > maxLines) { lines = lines.slice(0, maxLines); lines[maxLines - 1] += '…'; }
        var top = vertical === 'middle' ? y + h / 2 - (lines.length * lh) / 2 + size : vertical === 'bottom' ? y + h - 8 - lines.length * lh + size : y + 8 + size;
        var tx = align === 'center' ? x + w / 2 : align === 'right' ? x + w - 8 : x + 8, anchor = align === 'center' ? 'middle' : align === 'right' ? 'end' : 'start';
        return '<text x="' + tx.toFixed(1) + '" y="' + top.toFixed(1) + '" font-size="' + size + '" fill="' + esc(colour) + '" text-anchor="' + anchor + '">' +
            lines.map(function (l, i) { return '<tspan x="' + tx.toFixed(1) + '"' + (i ? ' dy="' + lh.toFixed(1) + '"' : '') + '>' + esc(l || ' ') + '</tspan>'; }).join('') + '</text>';
    }
    function bounds(doc, byId) {
        var minX = Infinity, minY = Infinity, maxX = -Infinity, maxY = -Infinity;
        function take(x, y) { minX = Math.min(minX, x); minY = Math.min(minY, y); maxX = Math.max(maxX, x); maxY = Math.max(maxY, y); }
        doc.nodes.forEach(function (n) { take(n.x, n.y); take(n.x + n.width, n.y + n.height); });
        doc.edges.forEach(function (e) { route(e, byId).points.forEach(function (p) { take(p[0], p[1]); }); });
        if (minX === Infinity) { return { x: 0, y: 0, w: 0, h: 0 }; }
        return { x: minX, y: minY, w: maxX - minX, h: maxY - minY };
    }

    /* The drawing. opts.palette: { text, muted, line, surface, bg, accent,
       presets: [six #rrggbb] }, all checked colours from the page;
       opts.pad, the margin around the contents; opts.title, the
       picture's text alternative. Nodes use the file's drawing order,
       and the edges are drawn on top. */
    var FONT = 14;
    function render(doc, opts) {
        opts = opts || {};
        var pal = Object.assign({}, opts.palette), pad = opts.pad == null ? 24 : opts.pad;
        if (doc.yavc && doc.yavc.background) { pal.bg = colourOf(doc.yavc.background, pal); pal.muted = inkFor(pal.bg, pal.muted); }
        var byId = Object.create(null);
        doc.nodes.forEach(function (n) { byId[n.id] = n; });
        var b = bounds(doc, byId), ox = b.x - pad, oy = b.y - pad, W = Math.ceil(b.w + pad * 2), H = Math.ceil(b.h + pad * 2);
        var out = [], boxes = [];
        function nodeColours(n) {
            var c = colourOf(n.color, pal), y = n.yavc || {}, fill = colourOf(y.fillColor, pal);
            var textOnly = n.type === 'text' && y.shape === 'text';
            return { stroke: colourOf(y.strokeColor, pal) || c || (n.type === 'group' && !fill ? inkFor(pal.bg, pal.line) : pal.line), fill: fill || (c ? mixHex(c, pal.surface, 0.14) : pal.surface), text: fill || textOnly ? inkFor(fill || pal.bg, pal.text) : pal.text };
        }
        doc.nodes.forEach(function (n) {
            if (isAnchor(n)) { return; }
            var x = n.x - ox, y = n.y - oy, cols = nodeColours(n), y2 = isObj(n.yavc) ? n.yavc : {};
            var lw = y2.lineWidth || (n.type === 'group' ? 1 : 2), dash = dashOf(y2, lw);
            var line = { fill: cols.fill, stroke: cols.stroke, 'stroke-width': lw };
            if (dash) { line['stroke-dasharray'] = dash; }
            boxes.push({ id: n.id, type: n.type, x: x, y: y, w: n.width, h: n.height, colors: { fill: n.type === 'text' && y2.shape === 'text' && !y2.fillColor ? pal.bg : cols.fill, stroke: cols.stroke } });
            out.push('<g data-node="' + esc(n.id) + '">');
            if (n.type === 'group') {
                var gl = Object.assign({}, line, { fill: colourOf(y2.fillColor, pal) || (colourOf(n.color, pal) ? mixHex(colourOf(n.color, pal), pal.bg, 0.08) : 'none'), 'stroke-dasharray': dash || '6 4' });
                boxes[boxes.length - 1].colors.fill = gl.fill === 'none' ? pal.bg : gl.fill;
                out.push(element({ tag: 'rect', a: { x: x, y: y, width: n.width, height: n.height, rx: 8 } }, gl));
                if (n.label) { out.push('<text x="' + (x + 8).toFixed(1) + '" y="' + (y - 6).toFixed(1) + '" font-size="' + FONT + '" font-weight="600" fill="' + esc(pal.muted) + '">' + esc(n.label) + '</text>'); }
            } else if (n.type === 'text') {
                var shape = y2.shape || 'rectangle';
                if (shape === 'text' && y2.fillColor) { out.push(element({ tag: 'rect', a: { x: x, y: y, width: n.width, height: n.height } }, { fill: cols.fill, stroke: 'none' })); }
                out.push(element(shapePath(shape, x, y, n.width, n.height), line));
                var inset = shape === 'person' ? { x: x, y: y + n.height * 0.55, w: n.width, h: n.height * 0.45 } : shape === 'diamond' || shape === 'hexagon' || shape === 'oval' || shape === 'cloud' ? { x: x + n.width * 0.15, y: y + n.height * 0.15, w: n.width * 0.7, h: n.height * 0.7 } : { x: x, y: y, w: n.width, h: n.height };
                /* Where the text goes, which the screen fills with the
                   Markdown drawn as HTML (decision 6). */
                var centred = shape !== 'rectangle' && shape !== 'rounded' && shape !== 'text' && shape !== 'document' && shape !== 'package';
                inset.align = y2.textAlign || (centred ? 'center' : 'left');
                inset.vertical = y2.textVerticalAlign || (centred ? 'middle' : 'top');
                inset.color = cols.text;
                boxes[boxes.length - 1].text = inset;
                out.push('<g data-text="' + esc(n.id) + '">' + textBlock(n.text, inset.x, inset.y, inset.w, inset.h, cols.text, FONT, inset.align, inset.vertical) + '</g>');
            } else if (n.type === 'file') {
                out.push(element({ tag: 'rect', a: { x: x, y: y, width: n.width, height: n.height, rx: 4 } }, line));
                var name = String(n.file).replace(/^.*\//, '') + (n.subpath || '');
                out.push(textBlock(name, x, y, n.width, n.height, cols.text, FONT, 'left', 'top'));
            } else if (n.type === 'link') {
                out.push(element({ tag: 'rect', a: { x: x, y: y, width: n.width, height: n.height, rx: 4 } }, line));
                out.push(textBlock(n.url, x, y, n.width, n.height, cols.text, FONT, 'left', 'top'));
            }
            out.push('</g>');
        });
        var paths = [];
        doc.edges.forEach(function (e) {
            var r = route(e, byId);
            r.points = r.points.map(function (p) { return [p[0] - ox, p[1] - oy]; });
            var y3 = isObj(e.yavc) ? e.yavc : {}, colour = colourOf(e.color, pal) || inkFor(pal.bg, pal.line), lw = y3.lineWidth || 2, dash = dashOf(y3, lw);
            var fromHead = y3.fromHead || (e.fromEnd === 'arrow' ? 'arrow' : 'none');
            var toHead = y3.toHead || (e.toEnd === 'none' ? 'none' : 'arrow');
            var d = pathD(r);
            paths.push({ id: e.id, d: d, fromPoint: r.points[0].slice(), toPoint: r.points[r.points.length - 1].slice() });
            out.push('<g data-edge="' + esc(e.id) + '"><path d="' + d + '" fill="none" stroke="' + esc(colour) + '" stroke-width="' + lw + '"' + (dash ? ' stroke-dasharray="' + dash + '"' : '') + '/>' +
                headAt(fromHead, r.points[0], endAngle(r, true), colour, lw, pal.bg) + headAt(toHead, r.points[r.points.length - 1], endAngle(r, false), colour, lw, pal.bg));
            if (e.label) {
                var m = midpoint(r), lw2 = Math.min(240, e.label.length * FONT * 0.56 + 12);
                out.push('<rect x="' + (m[0] - lw2 / 2).toFixed(1) + '" y="' + (m[1] - FONT * 0.8).toFixed(1) + '" width="' + lw2.toFixed(1) + '" height="' + (FONT * 1.6).toFixed(1) + '" rx="3" fill="' + esc(pal.bg) + '"/>' +
                    '<text x="' + m[0].toFixed(1) + '" y="' + (m[1] + FONT * 0.35).toFixed(1) + '" font-size="' + (FONT - 1) + '" text-anchor="middle" fill="' + esc(pal.muted) + '">' + esc(e.label) + '</text>');
            }
            out.push('</g>');
        });
        var svg = '<svg xmlns="http://www.w3.org/2000/svg" width="' + W + '" height="' + H + '" viewBox="0 0 ' + W + ' ' + H + '" font-family="' + esc(opts.font || 'Inter, system-ui, sans-serif') + '">' +
            (opts.title ? '<title>' + esc(opts.title) + '</title>' : '') + '<rect data-canvas-background="" width="' + W + '" height="' + H + '" fill="' + esc(pal.bg) + '"/>' + out.join('') + '</svg>';
        return { svg: svg, width: W, height: H, boxes: boxes, edges: paths, origin: [ox, oy], background: pal.bg };
    }
    function inkFor(fill, preferred) {
        function luminance(hex) {
            if (hex.length === 4) { hex = '#' + hex[1] + hex[1] + hex[2] + hex[2] + hex[3] + hex[3]; }
            var c = [1, 3, 5].map(function (i) { var v = parseInt(hex.slice(i, i + 2), 16) / 255; return v <= 0.04045 ? v / 12.92 : Math.pow((v + 0.055) / 1.055, 2.4); });
            return c[0] * 0.2126 + c[1] * 0.7152 + c[2] * 0.0722;
        }
        var bg = luminance(fill), fg = luminance(preferred), ratio = (Math.max(bg, fg) + 0.05) / (Math.min(bg, fg) + 0.05);
        return ratio >= 4.5 ? preferred : (bg + 0.05) / 0.05 >= 1.05 / (bg + 0.05) ? '#000000' : '#ffffff';
    }
    function mixHex(a, b, t) {
        function rgb(h) { if (h.length === 4) { h = '#' + h[1] + h[1] + h[2] + h[2] + h[3] + h[3]; } return [1, 3, 5].map(function (i) { return parseInt(h.substr(i, 2), 16); }); }
        var x = rgb(a), y = rgb(b);
        return '#' + [0, 1, 2].map(function (i) { return ('0' + Math.round(x[i] * t + y[i] * (1 - t)).toString(16)).slice(-2); }).join('');
    }

    /* The six preset colours, which the specification names red, orange,
       yellow, green, cyan and purple and leaves to each application: one set
       for a light background and one for a dark, each readable on it. */
    var PRESETS = Object.freeze({
        light: ['#c62828', '#c25a00', '#9a7b00', '#2e7d32', '#00838f', '#7b3fc4'],
        dark: ['#ff6b6b', '#ff9f43', '#f1c40f', '#66d17a', '#3cc8d8', '#b18cff']
    });

    var api = Object.freeze({ read: read, write: write, render: render, isAnchor: isAnchor, automatic: automatic, path: pathD, routes: routes, route: function (e, nodes) { return routes([e], nodes)[0].route; },
        PRESETS: PRESETS, SHAPES: SHAPES.slice(), HEADS: HEADS.slice(), ROUTES: ROUTES.slice(), LINES: LINES.slice(), EXT_VERSION: EXT_VERSION, LIMITS: Object.freeze({ text: MAX_TEXT, items: MAX_ITEMS }) });
    if (typeof module === 'object' && module.exports) { module.exports = api; } else { root.yavcCanvas = api; }
})(typeof window !== 'undefined' ? window : this);
