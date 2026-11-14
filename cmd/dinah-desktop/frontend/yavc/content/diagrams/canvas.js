/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* The visual editor's canvas (docs/diagrams-design.md, milestone D3): a
   layer over a D2 drawing that knows where each shape and connection is,
   from the engine's compile (shapes and connections, in the drawing's own
   pixels; decision 4), so a click or a key can select one. The drawing
   itself stays an image (decision 10); this layer is the site's own SVG,
   made from numbers and keys, and draws only the selection, as PUDL's
   focus ring.

     yavcDiagramCanvas.attach(place, host) -> canvas
       host.selected(key, how)   a shape or connection was selected, or null;
                                 how is 'pointer' or 'key'
       host.activate(key)        Enter or a double click on one
       host.pick(key)            a click while the host is choosing a shape,
                                 such as the other end of a connection;
                                 return true to take it
       host.tool()               the kind of shape the palette places, or null
                                 when the palette is on Select
       host.place(container)     a click while a kind is chosen: place one
                                 inside that shape, or at the top for null
       host.drop(d)              a drag ended: d.kind is 'connect' (from the
                                 selected shape's handle; d.to a shape, or
                                 null for empty space) or 'move' (d.from the
                                 shape dragged, d.to the container it was
                                 dropped on, or null for the top)
     canvas.show(out)            the compile's answer, after a drawing
     canvas.select(key)          select one, or null for none
     canvas.selected()           the key selected, or null
     canvas.keys()               the keys in drawing order
     canvas.destroy() */
(function () {
    'use strict';
    if (window.yavcDiagramCanvas) { return; }
    var NS = 'http://www.w3.org/2000/svg';

    function attach(place, host) {
        var layer = document.createElementNS(NS, 'svg'), out = null, sel = null, order = [];
        layer.setAttribute('class', 'dg-layer');
        layer.setAttribute('aria-hidden', 'true');
        place.classList.add('dg-editable');

        /* The layer covers the picture exactly, wherever the picture is
           drawn and at whatever size, since the drawing scales to fit. */
        function fitLayer() {
            var img = place.querySelector(':scope > img.sc-diagram-image');
            if (!img || !out) { layer.style.display = 'none'; return; }
            var pr = place.getBoundingClientRect(), ir = img.getBoundingClientRect();
            layer.style.display = '';
            layer.style.left = (ir.left - pr.left + place.scrollLeft) + 'px';
            layer.style.top = (ir.top - pr.top + place.scrollTop) + 'px';
            layer.style.width = ir.width + 'px';
            layer.style.height = ir.height + 'px';
            if (img.nextSibling !== layer) { img.after(layer); }
        }
        var ro = 'ResizeObserver' in window ? new ResizeObserver(fitLayer) : null;
        if (ro) { ro.observe(place); }
        /* The D2 client draws the picture again by itself when the site's
           mode or colours change (d2.js redrawAll), replacing what the place
           holds; the layout is the same, so the layer goes straight back. */
        var mo = new MutationObserver(function () { if (out && !layer.isConnected) { fitLayer(); } });
        mo.observe(place, { childList: true });

        function el(name, attrs) {
            var e = document.createElementNS(NS, name);
            Object.keys(attrs).forEach(function (k) { e.setAttribute(k, attrs[k]); });
            return e;
        }
        function show(o) {
            out = o;
            layer.replaceChildren();
            order = [];
            if (!out || !out.width) { fitLayer(); return; }
            layer.setAttribute('viewBox', '0 0 ' + out.width + ' ' + out.height);
            layer.setAttribute('preserveAspectRatio', 'none');
            /* The ground takes a click on empty space, which places a shape
               while the palette has one chosen. */
            layer.appendChild(el('rect', { x: 0, y: 0, width: out.width, height: out.height, 'class': 'dg-ground' }));
            /* Containers come first in D2's list, so a shape inside one is
               drawn later and takes a click before its container does. */
            (out.shapes || []).forEach(function (s) {
                var r = el('rect', { x: s.x, y: s.y, width: s.w, height: s.h, rx: 4, 'class': 'dg-hit dg-hit-shape' });
                r.__key = s.id;
                layer.appendChild(r);
                order.push(s.id);
            });
            (out.connections || []).forEach(function (c) {
                var d = pathOf(c);
                var g = el('g', { 'class': 'dg-hit dg-hit-line' });
                g.appendChild(el('path', { d: d, 'class': 'dg-hit-wide' }));
                g.appendChild(el('path', { d: d, 'class': 'dg-hit-thin' }));
                g.__key = c.id;
                layer.appendChild(g);
                order.push(c.id);
            });
            if (sel && order.indexOf(sel) < 0) { sel = null; host.selected(null, 'redraw'); }
            mark();
            fitLayer();
        }
        /* A connection's line as D2 draws it: straight from point to point,
           or, for a curved route, cubic Bezier segments whose control points
           come in threes after the start, as D2's SVG renderer reads them.
           That reading is not documented by D2; Paul accepted it as a pinned
           exception, held to D2's drawing by TestCurvedRouteMatchesDrawing
           in internal/d2api (2026-11-04). */
        function pathOf(c) {
            var p = c.points, xy = function (q) { return q[0] + ' ' + q[1]; };
            if (!p || !p.length) { return ''; }
            var d = 'M ' + xy(p[0]);
            if (c.curve && p.length >= 4 && (p.length - 1) % 3 === 0) {
                for (var i = 1; i + 2 < p.length; i += 3) { d += ' C ' + xy(p[i]) + ' ' + xy(p[i + 1]) + ' ' + xy(p[i + 2]); }
            } else {
                for (var j = 1; j < p.length; j++) { d += ' L ' + xy(p[j]); }
            }
            return d;
        }
        function mark() {
            Array.prototype.forEach.call(layer.children, function (e) { e.classList.toggle('is-selected', !!e.__key && e.__key === sel); });
            /* A selected shape has a handle on its right edge, from which a
               connection is dragged out. */
            layer.querySelectorAll('.dg-handle').forEach(function (h) { h.remove(); });
            var box = sel && (out.shapes || []).find(function (s) { return s.id === sel; });
            if (box && !(host.readOnly && host.readOnly())) {
                /* Six screen pixels whatever the drawing's scale, with a
                   wider ring that takes the pointer. */
                var lr = layer.getBoundingClientRect(), k = lr.width ? out.width / lr.width : 1;
                /* One on each side, so a connection can be dragged out
                   whichever way it is going; D2's layout still decides where
                   the line meets the shape. */
                [[box.x + box.w, box.y + box.h / 2], [box.x, box.y + box.h / 2], [box.x + box.w / 2, box.y], [box.x + box.w / 2, box.y + box.h]].forEach(function (pt) {
                    var grip = el('circle', { cx: pt[0], cy: pt[1], r: 12 * k, 'class': 'dg-handle dg-handle-grip' });
                    var h = el('circle', { cx: pt[0], cy: pt[1], r: 6 * k, 'class': 'dg-handle' });
                    grip.__at = h.__at = pt;
                    layer.appendChild(grip);
                    layer.appendChild(h);
                });
            }
        }
        function shapeBox(key) { return (out && out.shapes || []).find(function (s) { return s.id === key; }) || null; }
        function select(key, how) {
            key = key && order.indexOf(key) >= 0 ? key : null;
            if (key === sel) { return; }
            sel = key;
            mark();
            if (how) { host.selected(sel, how); }
        }
        function hitOf(target) {
            for (var e = target; e && e !== layer; e = e.parentNode) { if (e.__key) { return e.__key; } }
            return null;
        }
        /* The pointer. A press and release without moving is a click: it
           chooses a connection's far end, places a shape of the palette's
           kind, or selects. A press on the handle that moves drags out a
           connection; a press on the selected shape that moves drags it to
           a container, or to empty space for the top. */
        function toDrawing(e) {
            var r = layer.getBoundingClientRect();
            return { x: (e.clientX - r.left) * out.width / r.width, y: (e.clientY - r.top) * out.height / r.height };
        }
        function shapeAt(e, except) {
            var all = document.elementsFromPoint(e.clientX, e.clientY);
            for (var i = 0; i < all.length; i++) {
                var k = all[i].classList && all[i].classList.contains('dg-hit-shape') ? all[i].__key : null;
                if (k && !(except && (k === except || k.indexOf(except + '.') === 0))) { return k; }
            }
            return null;
        }
        var press = null, ghost = null, aim = null;
        function clearDrag() {
            if (ghost) { ghost.remove(); ghost = null; }
            if (aim) { aim.classList.remove('is-target'); aim = null; }
            place.classList.remove('dg-dragging', 'dg-dragging-connect', 'dg-dragging-move');
        }
        function target(key) {
            var r = key ? Array.prototype.find.call(layer.children, function (e) { return e.__key === key && e.classList.contains('dg-hit-shape'); }) : null;
            if (r === aim) { return; }
            if (aim) { aim.classList.remove('is-target'); }
            aim = r || null;
            if (aim) { aim.classList.add('is-target'); }
        }
        /* The pointer is followed on the place and not on the layer. The D2 client draws a diagram again by replacing what the place holds,
           which takes the layer out of the page until it is put back, and a pointer captured by a layer that leaves the page is released with
           no pointerup, so a click made while a drawing was replaced selected nothing. The place stays. */
        place.addEventListener('pointerdown', function (e) {
            if (e.button !== 0 || !out || !layer.contains(e.target)) { return; }
            var handle = e.target.classList && e.target.classList.contains('dg-handle');
            var key = handle ? sel : hitOf(e.target);
            press = { x: e.clientX, y: e.clientY, key: key, at: handle ? e.target.__at : null, kind: handle ? 'connect' : (key && key === sel && shapeBox(key) ? 'move' : null), moved: false };
            if (host.readOnly && host.readOnly()) press.kind = null;
            place.setPointerCapture(e.pointerId);
            e.preventDefault();
        });
        place.addEventListener('pointermove', function (e) {
            if (!press) { return; }
            if (!press.moved && Math.hypot(e.clientX - press.x, e.clientY - press.y) < 5) { return; }
            press.moved = true;
            if (!press.kind) { return; }
            place.classList.add('dg-dragging', press.kind === 'connect' ? 'dg-dragging-connect' : 'dg-dragging-move');
            var at = toDrawing(e), box = shapeBox(press.key);
            if (!box) { return; }
            if (press.kind === 'connect') {
                if (!ghost) { ghost = el('line', { 'class': 'dg-ghost' }); layer.appendChild(ghost); }
                var from = press.at || [box.x + box.w, box.y + box.h / 2];
                ghost.setAttribute('x1', from[0]); ghost.setAttribute('y1', from[1]);
                ghost.setAttribute('x2', at.x); ghost.setAttribute('y2', at.y);
                target(shapeAt(e, null));
            } else {
                if (!ghost) { ghost = el('rect', { width: box.w, height: box.h, rx: 4, 'class': 'dg-ghost' }); layer.appendChild(ghost); }
                ghost.setAttribute('x', at.x - box.w / 2); ghost.setAttribute('y', at.y - box.h / 2);
                target(shapeAt(e, press.key));
            }
        });
        function release(e, cancelled) {
            var p = press;
            press = null;
            if (!p) { return; }
            var dropped = aim ? aim.__key : null;
            clearDrag();
            if (cancelled) { return; }
            if (p.moved) {
                if (host.readOnly && host.readOnly()) return;
                if (p.kind === 'connect') { host.drop({ kind: 'connect', from: p.key, to: shapeAt(e, null) }); }
                else if (p.kind === 'move') { host.drop({ kind: 'move', from: p.key, to: dropped }); }
                return;
            }
            var key = p.key;
            if (key && host.pick && host.pick(key)) { return; }
            if (host.tool && host.tool() && (!key || shapeBox(key))) { host.place(key || null); return; }
            select(key, 'pointer');
        }
        place.addEventListener('pointerup', function (e) { release(e, false); });
        place.addEventListener('pointercancel', function (e) { release(e, true); });
        layer.addEventListener('dblclick', function (e) {
            var key = hitOf(e.target);
            if (key) { select(key, 'pointer'); if (!(host.readOnly && host.readOnly())) host.activate(key); }
        });
        /* Keys, on the drawing's pane: the arrows and Home and End move the
           selection through the shapes and connections in drawing order,
           Enter activates it, and Escape clears it. Delete and the rest are
           the host's commands. */
        function onKey(e) {
            if (!out || e.altKey || e.ctrlKey || e.metaKey) { return false; }
            var i = order.indexOf(sel), n = order.length;
            if (!n) { return false; }
            var to = null;
            if (e.key === 'ArrowRight' || e.key === 'ArrowDown') { to = order[(i + 1) % n]; }
            else if (e.key === 'ArrowLeft' || e.key === 'ArrowUp') { to = order[i < 0 ? n - 1 : (i - 1 + n) % n]; }
            else if (e.key === 'Home') { to = order[0]; }
            else if (e.key === 'End') { to = order[n - 1]; }
            else if (e.key === 'Enter' && sel) { host.activate(sel); return true; }
            else if (e.key === 'Escape' && press) { press = null; clearDrag(); return true; }
            else if (e.key === 'Escape' && sel) { select(null, 'key'); return true; }
            if (to == null) { return false; }
            select(to, 'key');
            return true;
        }
        layer.style.display = 'none';
        place.appendChild(layer);
        return {
            show: show, refreshAccess: mark, fit: fitLayer, onKey: onKey,
            select: function (key) { select(key, null); },
            selected: function () { return sel; },
            reveal: function () { var hit = Array.from(layer.children).find(function (e) { return e.__key === sel; }); if (hit) hit.scrollIntoView({block:'nearest',inline:'nearest'}); },
            keys: function () { return order.slice(); },
            destroy: function () { if (ro) { ro.disconnect(); } mo.disconnect(); layer.remove(); place.classList.remove('dg-editable'); }
        };
    }

    window.yavcDiagramCanvas = Object.freeze({ attach: attach });
})();
