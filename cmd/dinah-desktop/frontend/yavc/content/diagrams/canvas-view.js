/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* The JSON Canvas hit layer and camera. Camera coordinates are file pixels;
   exports keep using the renderer's content bounds. A drag only previews,
   and pointerup submits one edit against the text it started on. */
(function () {
    'use strict';
    var NS = 'http://www.w3.org/2000/svg';
    function attach(place, stage, host) {
        var M = window.yavcCanvasModel, J = window.yavcCanvas, doc, out, picture, selection = null, selections = [];
        var camera = null, scale = 1, autoFit = true, drag = null, space = false, destroyed = false, previewId;
        var layer = document.createElementNS(NS, 'svg'), grips = document.createElement('div');
        layer.setAttribute('class', 'dg-cv-layer'); layer.setAttribute('aria-hidden', 'true');
        grips.className = 'dg-cv-grips';
        place.classList.add('dg-cv-editor');
        function svg(name, attrs) {
            var e = document.createElementNS(NS, name);
            Object.keys(attrs).forEach(function (k) { e.setAttribute(k, attrs[k]); });
            return e;
        }
        function selectedNode() { return selection && selection.kind === 'node' && doc.nodes.find(function (n) { return n.id === selection.id; }); }
        function point(e) {
            var p = new DOMPoint(e.clientX, e.clientY).matrixTransform(layer.getScreenCTM().inverse());
            return { x: p.x + out.origin[0], y: p.y + out.origin[1] };
        }
        function view() {
            if (!out || !camera || !picture) { return; }
            var r = place.getBoundingClientRect();
            camera.w = r.width / scale; camera.h = r.height / scale;
            var vb = [camera.x - out.origin[0], camera.y - out.origin[1], camera.w, camera.h].join(' ');
            picture.setAttribute('viewBox', vb); layer.setAttribute('viewBox', vb);
            picture.setAttribute('preserveAspectRatio', 'none'); layer.setAttribute('preserveAspectRatio', 'none');
            mark();
            if (host.viewChanged) { host.viewChanged(); }
            var step = 8 * scale * Math.max(1, Math.ceil(6 / (8 * scale)));
            place.classList.toggle('dg-cv-grid', !!(doc.yavc && doc.yavc.grid));
            place.style.backgroundColor = out.background;
            place.style.backgroundSize = step + 'px ' + step + 'px';
            place.style.backgroundPosition = ((-camera.x * scale - step / 2) % step) + 'px ' + ((-camera.y * scale - step / 2) % step) + 'px';
        }
        function fit() {
            if (!out) { return; }
            var r = place.getBoundingClientRect();
            if (r.width < 1 || r.height < 1) { return; }
            scale = Math.min(1, r.width / out.width, r.height / out.height);
            autoFit = true;
            camera = { x: out.origin[0] - (r.width / scale - out.width) / 2, y: out.origin[1] - (r.height / scale - out.height) / 2 };
            view();
        }
        function zoom(factor, at) {
            if (!camera) { return; }
            autoFit = false;
            var r = place.getBoundingClientRect();
            at = at || { x: r.width / 2, y: r.height / 2 };
            var x = camera.x + at.x / scale, y = camera.y + at.y / scale;
            scale = Math.min(4, Math.max(0.05, scale * factor));
            camera.x = x - at.x / scale; camera.y = y - at.y / scale; view();
        }
        function pan(dx, dy) { if (camera) { autoFit = false; camera.x += dx / scale; camera.y += dy / scale; view(); } }
        function select(s, how) {
            var seen = new Set();
            selections = (Array.isArray(s) ? s : s ? [s] : []).filter(function (x) {
                var key = JSON.stringify([x.kind, x.id]);
                if (seen.has(key)) { return false; } seen.add(key); return true;
            }).map(function (x) { return { id: x.id, kind: x.kind }; });
            selection = selections[selections.length - 1] || null;
            mark(); host.selected(selection, how || 'key');
        }
        function reveal() {
            if (!selection || !camera || camera.w <= 0 || camera.h <= 0) { return; }
            var n = selectedNode(), edge = !n && out.edges.find(function (e) { return e.id === selection.id; });
            var b = n ? { x: n.x, y: n.y, w: n.width, h: n.height } : edge ? {
                x: Math.min(edge.fromPoint[0], edge.toPoint[0]) + out.origin[0], y: Math.min(edge.fromPoint[1], edge.toPoint[1]) + out.origin[1],
                w: Math.abs(edge.fromPoint[0] - edge.toPoint[0]), h: Math.abs(edge.fromPoint[1] - edge.toPoint[1])
            } : null;
            if (!b) { return; }
            var pad = 16 / scale, moved = false;
            [['x', 'w'], ['y', 'h']].forEach(function (a) {
                var p = a[0], size = a[1], next = camera[p];
                if (b[size] > camera[size] - pad * 2) { next = b[p] + b[size] / 2 - camera[size] / 2; }
                else if (b[p] < camera[p] + pad) { next = b[p] - pad; }
                else if (b[p] + b[size] > camera[p] + camera[size] - pad) { next = b[p] + b[size] - camera[size] + pad; }
                if (next !== camera[p]) { moved = true; camera[p] = next; }
            });
            if (moved) { autoFit = false; view(); }
        }
        function mark() {
            layer.querySelectorAll('.dg-cv-ring').forEach(function (e) { e.remove(); });
            grips.replaceChildren();
            if (!selection || !out) { return; }
            if (selections.length > 1) {
                selections.forEach(function (s) {
                    var box = s.kind === 'node' && out.boxes.find(function (b) { return b.id === s.id; });
                    var edge = s.kind === 'edge' && out.edges.find(function (e) { return e.id === s.id; });
                    if (box) { layer.appendChild(svg('rect', { x: box.x, y: box.y, width: box.w, height: box.h, 'class': 'dg-cv-ring' })); }
                    else if (edge) { layer.appendChild(svg('path', { d: edge.d, 'class': 'dg-cv-ring dg-cv-line-ring' })); }
                });
                return;
            }
            var n = selectedNode(), b = n && out.boxes.find(function (x) { return x.id === n.id; });
            if (!b) {
                var edge = out.edges.find(function (e) { return e.id === selection.id; });
                if (edge) {
                    layer.appendChild(svg('path', { d: edge.d, 'class': 'dg-cv-ring dg-cv-line-ring' }));
                    if (!locked()) {
                        grip('from', 'Move the Start Endpoint', edge.fromPoint[0] + out.origin[0], edge.fromPoint[1] + out.origin[1], 'reconnect');
                        grip('to', 'Move the End Endpoint', edge.toPoint[0] + out.origin[0], edge.toPoint[1] + out.origin[1], 'reconnect');
                    }
                }
                return;
            }
            layer.appendChild(svg('rect', { x: b.x, y: b.y, width: b.w, height: b.h, 'class': 'dg-cv-ring' }));
            if (locked()) return;
            [['nw', 0, 0], ['n', 0.5, 0], ['ne', 1, 0], ['e', 1, 0.5], ['se', 1, 1], ['s', 0.5, 1], ['sw', 0, 1], ['w', 0, 0.5]].forEach(function (h) {
                grip(h[0], 'Resize ' + ({ n: 'Top', s: 'Bottom', e: 'Right', w: 'Left', nw: 'Top Left', ne: 'Top Right', sw: 'Bottom Left', se: 'Bottom Right' })[h[0]], n.x + n.width * h[1], n.y + n.height * h[2], 'resize');
            });
            [['top', 0.5, 0, 0, -22], ['right', 1, 0.5, 22, 0], ['bottom', 0.5, 1, 0, 22], ['left', 0, 0.5, -22, 0]].forEach(function (h) {
                grip(h[0], 'Connect from the ' + h[0].charAt(0).toUpperCase() + h[0].slice(1), n.x + n.width * h[1], n.y + n.height * h[2], 'connect', h[3], h[4]);
            });
        }
        function grip(value, label, x, y, kind, dx, dy) {
            var b = document.createElement('button');
            b.type = 'button'; b.className = 'btn dg-cv-grip dg-cv-' + kind;
            b.tabIndex = -1; b.setAttribute('aria-label', label); b.dataset.grip = value; b.dataset.kind = kind;
            b.style.left = ((x - camera.x) * scale + (dx || 0)) + 'px'; b.style.top = ((y - camera.y) * scale + (dy || 0)) + 'px';
            if (kind === 'resize') { b.style.cursor = value + '-resize'; }
            grips.appendChild(b);
        }
        function show(d, o, pic) {
            cancel(); doc = d; out = o; picture = pic;
            var background = picture.querySelector('[data-canvas-background]');
            if (background) { background.setAttribute('fill', 'none'); }
            var used = new Set(doc.nodes.concat(doc.edges).map(function (n) { return n.id; })), i = 0;
            do { previewId = '@pointer-' + (++i); } while (used.has(previewId));
            place.append(layer, grips);
            layer.replaceChildren();
            /* Edges are drawn above nodes by the renderer, so their hits
               follow the node hits too. The selected node's grips sit above. */
            out.boxes.forEach(function (b) {
                var hit = svg('rect', { x: b.x, y: b.y, width: Math.max(b.w, 1), height: Math.max(b.h, 1), 'class': 'dg-cv-hit' });
                hit.dataset.id = b.id; hit.dataset.kind = 'node'; layer.appendChild(hit);
            });
            out.edges.forEach(function (e) {
                var hit = svg('path', { d: e.d, 'class': 'dg-cv-edge-hit' });
                hit.dataset.id = e.id; hit.dataset.kind = 'edge'; layer.appendChild(hit);
            });
            var kept = selections.filter(function (s) { return (s.kind === 'node' ? doc.nodes : doc.edges).some(function (x) { return x.id === s.id; }); });
            if (kept.length !== selections.length) { select(kept, 'redraw'); }
            if (!camera) { fit(); } else { view(); }
        }
        function preview(box, guides) {
            layer.querySelectorAll('.dg-cv-preview, .dg-cv-guide').forEach(function (e) { e.remove(); });
            var n = drag.node, moving = drag.kind === 'move' ? M.movingIds(doc, selections) : [];
            var changed = new Set([n.id].concat(moving));
            [Object.assign({}, n, box)].concat(doc.nodes.filter(function (x) { return x.id !== n.id && changed.has(x.id) && !J.isAnchor(x); }).map(function (x) { return Object.assign({}, x, { x: x.x + box.x - n.x, y: x.y + box.y - n.y }); })).forEach(function (x) {
                layer.appendChild(svg('rect', { x: x.x - out.origin[0], y: x.y - out.origin[1], width: x.width, height: x.height, 'class': 'dg-cv-preview' }));
            });
            var nodes = doc.nodes.map(function (x) {
                if (x.id === n.id) { return Object.assign({}, x, box); }
                return changed.has(x.id) ? Object.assign({}, x, { x: x.x + box.x - n.x, y: x.y + box.y - n.y }) : x;
            });
            J.routes(doc.edges.filter(function (e) { return changed.has(e.fromNode) || changed.has(e.toNode); }), nodes).forEach(function (r) { previewRoute(r.id, r.route); });
            guides.forEach(function (g) {
                var at = g.at - out.origin[g.axis === 'x' ? 0 : 1];
                layer.appendChild(svg('line', { x1: g.axis === 'x' ? at : camera.x - out.origin[0], x2: g.axis === 'x' ? at : camera.x + camera.w - out.origin[0], y1: g.axis === 'y' ? at : camera.y - out.origin[1], y2: g.axis === 'y' ? at : camera.y + camera.h - out.origin[1], 'class': 'dg-cv-guide' }));
            });
        }
        function previewRoute(id, r) {
            var g = picture.querySelector('g[data-edge="' + CSS.escape(id) + '"]');
            if (g) { g.classList.add('dg-cv-route-muted'); }
            r.points = r.points.map(function (p) { return [p[0] - out.origin[0], p[1] - out.origin[1]]; });
            layer.appendChild(svg('path', { d: J.path(r), 'class': 'dg-cv-preview', 'data-preview-edge': id }));
        }
        function connectionRequest(d, p) {
            var a = M.attachment(doc, p, 12 / scale, d.kind === 'connect' ? d.node.id : null);
            return d.kind === 'connect'
                ? { op: 'connect', from: d.node.id, fromSide: d.handle, to: a.node, toSide: a.side, x: a.x, y: a.y }
                : { op: 'reconnect', kind: 'edge', key: d.edge.id, end: d.handle, node: a.node, side: a.side, x: a.x, y: a.y };
        }
        function previewConnection(d, p) {
            layer.querySelectorAll('.dg-cv-preview').forEach(function (x) { x.remove(); });
            var req = connectionRequest(d, p), end = d.kind === 'connect' ? 'to' : d.handle;
            var edge = d.kind === 'connect' ? { id: previewId, fromNode: d.node.id, fromSide: d.handle } : Object.assign(Object.create(null), d.edge, {yavc:Object.assign(Object.create(null),d.edge.yavc)});
            var target = d.kind === 'connect' ? req.to : req.node, side = d.kind === 'connect' ? req.toSide : req.side;
            edge[end + 'Node'] = target || previewId;
            if (side) { edge[end + 'Side'] = side; } else { delete edge[end + 'Side']; }
            if (edge.yavc) { delete edge.yavc[end + 'Auto']; }
            var nodes = target ? doc.nodes : doc.nodes.concat([{ id: previewId, x: req.x, y: req.y, width: 0, height: 0 }]);
            previewRoute(edge.id, J.route(edge, nodes));
            if (target) {
                var b = out.boxes.find(function (x) { return x.id === target; });
                if (b) { layer.appendChild(svg('rect', { x: b.x, y: b.y, width: b.w, height: b.h, 'class': 'dg-cv-preview' })); }
            }
        }
        function locked() { return !!(host.readOnly && host.readOnly()); }
        function down(e) {
            if (drag || !out || !camera || !host.enabled() || (e.button !== 0 && e.button !== 1)) { return; }
            var grip = e.target.closest('[data-grip]'), hit = e.target.closest('[data-id]'), p = point(e);
            stage.focus({ preventScroll: true });
            if (space || e.button === 1 || host.panTool()) { drag = { kind: 'pan', px: e.clientX, py: e.clientY, camera: Object.assign({}, camera) }; }
            else if (!locked() && grip && grip.dataset.kind === 'reconnect' && selection && selection.kind === 'edge') {
                drag = { kind: 'reconnect', handle: grip.dataset.grip, edge: doc.edges.find(function (x) { return x.id === selection.id; }), point: p };
            }
            else if (!locked() && grip && selectedNode()) { drag = { kind: grip.dataset.kind, handle: grip.dataset.grip, node: Object.assign({}, selectedNode()), point: p }; }
            else if (!locked() && host.tool()) { host.create(Math.round(p.x), Math.round(p.y)); e.preventDefault(); return; }
            else if (hit) {
                var s = { kind: hit.dataset.kind, id: hit.dataset.id }, contains = selections.some(function (x) { return x.kind === s.kind && x.id === s.id; });
                if (e.shiftKey || e.ctrlKey || e.metaKey) {
                    select(contains ? selections.filter(function (x) { return x.kind !== s.kind || x.id !== s.id; }) : selections.concat([s]), 'pointer');
                    e.preventDefault(); return;
                }
                select(contains ? selections.filter(function (x) { return x.kind !== s.kind || x.id !== s.id; }).concat([s]) : s, 'pointer');
                if (!locked() && selection.kind === 'node') { drag = { kind: 'move', node: Object.assign({}, selectedNode()), point: p }; }
            } else { drag = { kind: 'marquee', point: p, base: e.shiftKey || e.ctrlKey || e.metaKey ? selections.slice() : [] }; }
            if (drag) {
                autoFit = false;
                e.preventDefault(); drag.pointer = e.pointerId; drag.text = host.text(); drag.px = e.clientX; drag.py = e.clientY;
                place.setPointerCapture(e.pointerId); place.classList.add('dg-cv-dragging');
            }
        }
        function move(e) {
            if (!drag || drag.pointer !== e.pointerId) { return; }
            if (drag.kind === 'pan') {
                camera.x = drag.camera.x - (e.clientX - drag.px) / scale; camera.y = drag.camera.y - (e.clientY - drag.py) / scale; view(); return;
            }
            if (host.text() !== drag.text || !host.enabled()) { cancel(); return; }
            var p = point(e), dx = Math.round(p.x - drag.point.x), dy = Math.round(p.y - drag.point.y);
            drag.moved = drag.moved || Math.hypot(e.clientX - drag.px, e.clientY - drag.py) >= 3;
            if (drag.kind === 'marquee') {
                layer.querySelectorAll('.dg-cv-preview').forEach(function (x) { x.remove(); });
                drag.box = { x: Math.min(p.x, drag.point.x), y: Math.min(p.y, drag.point.y), width: Math.abs(p.x - drag.point.x), height: Math.abs(p.y - drag.point.y) };
                layer.appendChild(svg('rect', { x: drag.box.x - out.origin[0], y: drag.box.y - out.origin[1], width: drag.box.width, height: drag.box.height, 'class': 'dg-cv-preview' }));
                return;
            }
            if (drag.kind === 'connect' || drag.kind === 'reconnect') {
                previewConnection(drag, p);
                return;
            }
            var box = drag.kind === 'move' ? Object.assign({}, drag.node, { x: drag.node.x + dx, y: drag.node.y + dy }) : M.resize(drag.node, drag.handle, dx, dy, e.shiftKey);
            var snapped;
            if (drag.kind === 'resize') {
                /* Snap the edge under the pointer, then apply the aspect
                   ratio and its opposite anchor again. */
                var west = /w/.test(drag.handle), north = /n/.test(drag.handle);
                var tip = { x: west ? box.x : box.x + box.width, y: north ? box.y : box.y + box.height, width: 0, height: 0 };
                snapped = M.snap(tip, doc.nodes, [drag.node.id], 5 / scale, e.altKey || !!(doc.yavc && doc.yavc.snap === false));
                box = M.resize(drag.node, drag.handle, /[ew]/.test(drag.handle) ? snapped.box.x - (west ? drag.node.x : drag.node.x + drag.node.width) : 0, /[ns]/.test(drag.handle) ? snapped.box.y - (north ? drag.node.y : drag.node.y + drag.node.height) : 0, e.shiftKey);
            } else { snapped = M.snap(box, doc.nodes, M.movingIds(doc, selections), 5 / scale, e.altKey || !!(doc.yavc && doc.yavc.snap === false)); box = snapped.box; }
            drag.box = box; preview(box, e.shiftKey && drag.kind === 'resize' ? [] : snapped.guides);
        }
        function up(e) {
            if (!drag || drag.pointer !== e.pointerId) { return; }
            move(e);
            if (!drag) { return; }
            var d = drag, p = point(e);
            cancel();
            if (d.kind === 'marquee' && host.text() === d.text && host.enabled()) {
                var b = d.box, picked = !d.moved || !b ? [] : doc.nodes.filter(function (n) {
                    return !J.isAnchor(n) && n.x >= b.x && n.y >= b.y && n.x + n.width <= b.x + b.width && n.y + n.height <= b.y + b.height;
                }).map(function (n) { return { kind: 'node', id: n.id }; });
                select(d.base.concat(picked), 'pointer'); return;
            }
            if (host.text() !== d.text || !d.moved || !host.enabled()) { return; }
            if (locked()) return;
            if (d.kind === 'connect' || d.kind === 'reconnect') {
                host.edit(connectionRequest(d, p), d.text);
            } else if (d.box) {
                host.edit(d.kind === 'move' ? { op: 'move', items: selections.slice(), dx: d.box.x - d.node.x, dy: d.box.y - d.node.y } : { op: 'resize', key: d.node.id, x: d.box.x, y: d.box.y, width: d.box.width, height: d.box.height }, d.text);
            }
        }
        function cancel() {
            var d = drag; drag = null;
            if (d && place.hasPointerCapture(d.pointer)) { place.releasePointerCapture(d.pointer); }
            place.classList.remove('dg-cv-dragging');
            layer.querySelectorAll('.dg-cv-preview, .dg-cv-guide').forEach(function (e) { e.remove(); });
            if (picture) { picture.querySelectorAll('.dg-cv-route-muted').forEach(function (e) { e.classList.remove('dg-cv-route-muted'); }); }
        }
        function wheel(e) {
            if (!camera || !host.enabled()) { return; }
            e.preventDefault();
            if (drag) { cancel(); }
            if (e.ctrlKey || e.metaKey) { var r = place.getBoundingClientRect(); zoom(Math.exp(-e.deltaY * 0.002), { x: e.clientX - r.left, y: e.clientY - r.top }); }
            else { var unit = e.deltaMode === 1 ? 16 : e.deltaMode === 2 ? place.clientHeight : 1; pan(e.deltaX * unit, e.deltaY * unit); }
        }
        function cycle(delta) {
            if (!doc) { return; }
            var items = doc.nodes.filter(function (n) { return !J.isAnchor(n); }).map(function (n) { return { kind: 'node', id: n.id }; }).concat(doc.edges.map(function (e) { return { kind: 'edge', id: e.id }; }));
            var i = items.findIndex(function (x) { return selection && x.kind === selection.kind && x.id === selection.id; }), next = i + delta;
            if (next < 0 || next >= items.length) { select(null); return false; }
            select(items[next]);
            var n = selectedNode();
            if (n) { camera.x = n.x + n.width / 2 - camera.w / 2; camera.y = n.y + n.height / 2 - camera.h / 2; view(); }
            return true;
        }
        function onKey(e) {
            if (e.key === 'Escape') { cancel(); select(null); return true; }
            if (e.key === 'Tab') { return cycle(e.shiftKey ? -1 : 1); }
            if (e.key === ' ') { space = true; e.preventDefault(); return true; }
            return false;
        }
        function keyup(e) { if (e.key === ' ') { space = false; } }
        function blur() { space = false; cancel(); }
        function activate(e) {
            var under = document.elementFromPoint(e.clientX, e.clientY), hit = under && place.contains(under) && under.closest('[data-id]');
            if (hit && hit.dataset.kind === 'node' && host.enabled() && !locked() && host.activate) { host.activate(hit.dataset.id); }
        }
        place.addEventListener('pointerdown', down); place.addEventListener('pointermove', move); place.addEventListener('pointerup', up);
        place.addEventListener('dblclick', activate);
        place.addEventListener('pointercancel', cancel); place.addEventListener('lostpointercapture', cancel); place.addEventListener('wheel', wheel, { passive: false });
        window.addEventListener('keyup', keyup); window.addEventListener('blur', blur);
        var ro = new ResizeObserver(function () { if (!destroyed) { cancel(); if (camera && !autoFit) { view(); } else { fit(); } } }); ro.observe(stage);
        return { show: show, refreshAccess: mark, selected: function () { return selection; }, selections: function () { return selections.map(function (s) { return Object.assign({}, s); }); }, select: select, reveal: reveal, cycle: cycle, onKey: onKey, fit: fit, zoom: zoom, actual: function () { zoom(1 / scale); }, pan: pan, cancel: cancel,
            bounds: function (key) {
                var n = doc && doc.nodes.find(function (x) { return x.id === key; }), r = place.getBoundingClientRect();
                return n && camera ? { x: r.left + (n.x - camera.x) * scale, y: r.top + (n.y - camera.y) * scale, width: n.width * scale, height: n.height * scale } : null;
            },
            centre: function () { return camera ? { x: Math.round(camera.x + camera.w / 2), y: Math.round(camera.y + camera.h / 2) } : { x: 0, y: 0 }; },
            destroy: function () {
                destroyed = true; cancel(); ro.disconnect(); window.removeEventListener('keyup', keyup); window.removeEventListener('blur', blur);
                place.removeEventListener('pointerdown', down); place.removeEventListener('pointermove', move); place.removeEventListener('pointerup', up);
                place.removeEventListener('dblclick', activate);
                place.removeEventListener('pointercancel', cancel); place.removeEventListener('lostpointercapture', cancel); place.removeEventListener('wheel', wheel);
                layer.remove(); grips.remove(); place.classList.remove('dg-cv-editor', 'dg-cv-grid');
                place.style.removeProperty('background-color'); place.style.removeProperty('background-size'); place.style.removeProperty('background-position');
            } };
    }
    window.yavcCanvasView = Object.freeze({ attach: attach });
})();
