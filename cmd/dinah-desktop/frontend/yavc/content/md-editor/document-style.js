/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Shared document CSS compiler. Hosts assign a scope and own installation. */
(function () {
    'use strict';
    var LIMIT = 256 * 1024, sequence = 0, surfaces = new WeakMap();
    var TOKENS = /^--(doc-(font|font-heading|size|leading|paragraph-spacing|ink|muted|accent|rule|page|code|warning|danger|positive)|deck-(bg|ink|heading|muted|accent|rule|code))$/;
    // This first compiler admits typography and layout without resource loads.
    // Containment, positioning and animation remain under host control.
    var PAINT = /^(color|background-color|font-(style|weight|size|family|variant)|text-(align|decoration(-[a-z]+)?|transform|indent)|letter-spacing|word-spacing|line-height|vertical-align|white-space|(max-|min-)?(width|height)|margin(-[a-z]+)?|padding(-[a-z]+)?|border(-(top|right|bottom|left))?(-(color|style|width|radius))?|border-radius|list-style-type|display|opacity|columns?(-[a-z]+)*|gap|aspect-ratio|object-fit|float|clear|break-(before|after|inside))$/;
    function compile(key, css) {
        var problems = [], classes = new Set();
        function problem(message) { if (problems.indexOf(message) < 0) { problems.push(message); } }
        css = String(css == null ? '' : css);
        if (!/^[a-z][a-z0-9-]{0,63}$/.test(key)) { throw new Error('Invalid document style scope'); }
        // Reject escapes until resource parsing has a token-aware resolver.
        // A textual url() check alone cannot recognize escaped CSS functions.
        if (new TextEncoder().encode(css).length > LIMIT || /\\/.test(css)) {
            return { css: '', classes: [], problems: ['Stylesheets must be at most 256 KiB and may not contain CSS escapes.'] };
        }
        function clean(rule) {
            if (rule.type !== CSSRule.STYLE_RULE) {
                problem('At-rules are not supported in document styles yet.'); return '';
            }
            var declarations = [];
            for (var i = 0; i < rule.style.length; i++) {
                var name = rule.style[i], value = rule.style.getPropertyValue(name);
                if ((!PAINT.test(name) && !TOKENS.test(name)) || /url\s*\(|image-set\s*\(|expression\s*\(|attr\s*\(/i.test(value)) {
                    problem('Unsupported or resource-loading declarations were omitted.'); continue;
                }
                declarations.push(name + ':' + value + (rule.style.getPropertyPriority(name) ? '!important' : '') + ';');
            }
            var nested = Array.prototype.map.call(rule.cssRules || [], clean).join('');
            if (!declarations.length && !nested) { return ''; }
            var match, names = /\.([a-zA-Z_][a-zA-Z0-9_-]*)/g;
            while ((match = names.exec(rule.selectorText))) { classes.add(match[1]); }
            return rule.selectorText + '{' + declarations.join('') + nested + '}';
        }
        var result = '';
        try {
            var sheet = new CSSStyleSheet();
            sheet.replaceSync(':scope {\n' + css + '\n}');
            var first = sheet.cssRules[0];
            if (first && first.type === CSSRule.STYLE_RULE && first.selectorText === ':scope') { result = clean(first); }
            if (sheet.cssRules.length > 1) { problem('Rules outside the document scope were omitted.'); }
        } catch (err) { problem('The browser could not parse this stylesheet.'); }
        return { css: result ? '@scope ([data-md-style~="' + key + '"]) to ([data-host-suggestion]) {' + result + '}' : '', classes: Array.from(classes).sort(), problems: problems };
    }
    function tokens(el) { return (el.getAttribute('data-md-style') || '').split(/\s+/).filter(Boolean); }
    function bind(el, key) { var keys = tokens(el); if (keys.indexOf(key) < 0) { keys.push(key); } el.setAttribute('data-md-style', keys.join(' ')); }
    function unbind(el, key) { var keys = tokens(el).filter(function (k) { return k !== key; }); if (keys.length) { el.setAttribute('data-md-style', keys.join(' ')); } else { el.removeAttribute('data-md-style'); } }
    function acquire(el, key) {
        var held = surfaces.get(el);
        if (!held) {
            var effective = getComputedStyle(el).contain;
            var applied = /\b(strict|content|paint)\b/.test(effective) ? effective : (effective === 'none' ? 'paint' : effective + ' paint');
            held = { keys: new Set(), value: el.style.getPropertyValue('contain'), priority: el.style.getPropertyPriority('contain'), applied: applied };
            surfaces.set(el, held);
        }
        held.keys.add(key);
        el.style.setProperty('contain', held.applied, 'important');
    }
    function release(el, key) {
        var held = surfaces.get(el);
        if (!held) { return; }
        held.keys.delete(key);
        if (held.keys.size) { return; }
        if (el.style.getPropertyValue('contain') === held.applied && el.style.getPropertyPriority('contain') === 'important') {
            if (held.value) { el.style.setProperty('contain', held.value, held.priority); } else { el.style.removeProperty('contain'); }
        }
        surfaces.delete(el);
    }
    // The host resolves relative paths against its virtual file system first.
    // No URL, network path or parent traversal reaches the read capability.
    function localSheet(path) {
        return /^(~\/|\/(?!\/))[^\\:\u0000-\u001f?#]+\.css$/i.test(path) && !path.split('/').some(function (part) { return part === '..' || part === '.'; });
    }
    function attach(opts) {
        opts = opts || {};
        var key = 'document-' + (++sequence), bound = new Set(), contained = new Set(), disposed = false, revision = 0, base = String(opts.base || '');
        var identity = null, path = null, shared = null, readToken = null, cached = '', result = { css: '', classes: [], problems: [] };
        var style = document.createElement('style'); style.setAttribute('data-md-style-owner', key); document.head.appendChild(style);
        function refresh() {
            if (disposed) { return; }
            var wanted = new Set(Array.from(typeof opts.surfaces === 'function' ? opts.surfaces() : (opts.surfaces || [])));
            var boundaries = new Set(Array.from(typeof opts.boundaries === 'function' ? opts.boundaries() : (opts.boundaries || [])));
            wanted.forEach(function (el) { if (!el || el.nodeType !== 1 || el.ownerDocument !== document) { throw new Error('Document style surfaces must belong to this page'); } });
            boundaries.forEach(function (el) {
                if (!el || el.nodeType !== 1 || el.ownerDocument !== document || Array.from(wanted).some(function (surface) { return surface.contains(el); })) { throw new Error('Style boundaries must be outside every styled surface'); }
                if (!/^(none|block|flow-root|flex|grid|inline-block|inline-flex|inline-grid)$/.test(getComputedStyle(el).display)) { throw new Error('Style boundaries need a host-owned principal box'); }
            });
            wanted.forEach(function (el) { if (!Array.from(boundaries).some(function (boundary) { return boundary.contains(el); })) { throw new Error('Each styled surface needs a containing host boundary'); } });
            contained.forEach(function (el) { if (!boundaries.has(el)) { release(el, key); } });
            bound.forEach(function (el) { if (!wanted.has(el)) { unbind(el, key); } });
            boundaries.forEach(function (el) { acquire(el, key); });
            wanted.forEach(function (el) { bind(el, key); });
            bound = wanted; contained = boundaries;
        }
        function install(layers, problems) {
            if (disposed) { return; }
            if (base) { layers = [base].concat(layers); }
            var size = layers.reduce(function (n, css) { return n + new TextEncoder().encode(css).length; }, 0);
            if (size > LIMIT || layers.length > 128) { layers = []; problems.push('Combined document styles exceed the supported size.'); }
            var parts = layers.map(function (css) { return compile(key, css); });
            result = { css: parts.map(function (p) { return p.css; }).join('\n'),
                classes: Array.from(new Set([].concat.apply([], parts.map(function (p) { return p.classes; })))).sort(),
                problems: Array.from(new Set(problems.concat([].concat.apply([], parts.map(function (p) { return p.problems; }))))) };
            // Bind containment before CSS is ever installed.
            refresh(); style.textContent = result.css;
            if (opts.onProblems) { opts.onProblems(result.problems.slice()); }
        }
        async function update(input) {
            if (disposed) { return { applied: false }; }
            input = input || {};
            // Hosts can update validated base tokens below authored stylesheet layers.
            if (Object.prototype.hasOwnProperty.call(input, 'base')) { base = String(input.base || ''); }
            var current = ++revision, nextPath = String(input.stylesheet || ''), layers = (input.inline || []).map(String);
            if (identity !== input.identity || path !== nextPath || input.reload) {
                identity = input.identity; path = nextPath; shared = null; readToken = null; cached = '';
            }
            // Clear the previous document immediately while another file loads.
            install((cached ? [cached] : []).concat(layers), []);
            if (disposed || current !== revision) { return { applied: false }; }
            if (!nextPath) { return { applied: true }; }
            if (!localSheet(nextPath)) {
                install(layers, ['Shared stylesheets must be resolved local .css paths.']); return { applied: true };
            }
            if (!opts.read) { install(layers, ['This host cannot read shared stylesheets.']); return { applied: true }; }
            if (!shared) {
                var token = {}; readToken = token;
                shared = Promise.resolve().then(function () { return !disposed && readToken === token ? opts.read(nextPath) : null; }).then(function (text) {
                    return typeof text === 'string' ? { text: text } : { problem: 'The shared stylesheet was not found.' };
                }, function () { return { problem: 'The shared stylesheet could not be read.' }; });
            }
            var loaded = await shared;
            if (disposed || current !== revision) { return { applied: false }; }
            cached = loaded.text || '';
            install((loaded.text != null ? [loaded.text] : []).concat(layers), loaded.problem ? [loaded.problem] : []);
            return { applied: true };
        }
        return Object.freeze({ update: update, refresh: refresh,
            state: function () { return { key: key, css: result.css, classes: result.classes.slice(), problems: result.problems.slice() }; },
            destroy: function () {
                if (disposed) { return; }
                disposed = true; revision++; style.remove(); bound.forEach(function (el) { unbind(el, key); }); contained.forEach(function (el) { release(el, key); }); bound.clear(); contained.clear(); shared = null;
            }
        });
    }
    window.yavcDocumentStyle = Object.freeze({ compile: compile, attach: attach });
})();
