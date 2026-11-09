/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* A bounded static JSON Canvas image for document and slide surfaces. */
(function () {
    'use strict';
    var here = new URL('.', document.currentScript.src), loading = null, owners = new WeakMap(), count = 0;
    var palette = Object.freeze({text:'#202124',muted:'#5f6368',line:'#5f6368',surface:'#ffffff',bg:'#ffffff',accent:'#1f5fbf',presets:['#c62828','#c25a00','#9a7b00','#2e7d32','#00838f','#7b3fc4']});
    function library() {
        if (window.yavcCanvas) return Promise.resolve(window.yavcCanvas);
        if (!loading) loading = window.yavcUtil.loadScript(window.yavcUtil.versioned(new URL('jcanvas.js', here).href)).then(function () { return window.yavcCanvas; }).catch(function (error) { loading = null; throw error; });
        return loading;
    }
    async function draw(place, source) {
        var token = {}; owners.set(place, token);
        var J = await library();
        if (!place.isConnected || owners.get(place) !== token) return null;
        var parsed = J.read(source), problem = parsed.problems.find(function (p) { return !p.warning; });
        if (!parsed.doc || problem) throw new Error(problem ? problem.message : 'The canvas could not be read.');
        var out = J.render(parsed.doc, {palette:palette,pad:24,title:'Embedded canvas diagram'});
        if (!Number.isFinite(out.width) || !Number.isFinite(out.height) || out.width > 32768 || out.height > 32768 || out.svg.length > 16 * 1024 * 1024) throw new Error('This canvas is too large to embed. Open it in Diagrams.');
        var image = document.createElement('img');
        image.className = 'sc-diagram-image'; image.alt = 'Canvas diagram with ' + parsed.doc.nodes.length + ' nodes and ' + parsed.doc.edges.length + ' connections.';
        image.width = out.width; image.height = out.height; image.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(out.svg);
        image.style.maxWidth = '100%'; image.style.height = 'auto';
        place.replaceChildren(image); place.classList.add('ed-diagram');
        var disclosure = document.createElement('button'), list = document.createElement('ul');
        list.id = 'canvas-contents-' + (++count); list.hidden = true; list.setAttribute('data-canvas-outline', '');
        disclosure.type = 'button'; disclosure.className = 'btn btn-sm disclosure'; disclosure.textContent = 'Diagram Contents'; disclosure.setAttribute('data-canvas-contents', ''); disclosure.setAttribute('aria-expanded', 'false'); disclosure.setAttribute('aria-controls', list.id);
        disclosure.addEventListener('click', function () { list.hidden = !list.hidden; disclosure.setAttribute('aria-expanded', String(!list.hidden)); });
        parsed.doc.nodes.filter(function (n) { return !J.isAnchor(n); }).forEach(function (n) { var row = document.createElement('li'); row.textContent = n.text || n.label || n.file || n.url || n.id; list.append(row); });
        parsed.doc.edges.forEach(function (e) { var row = document.createElement('li'); row.textContent = e.fromNode + ' → ' + e.toNode + (e.label ? ': ' + e.label : ''); list.append(row); });
        place.append(disclosure, list);
        return out;
    }
    window.yavcCanvasEmbed = Object.freeze({draw:draw});
})();
