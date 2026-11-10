/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* The pictures a canvas's file nodes refer to (docs/canvas-extension.md,
   Pictures; DESIGN.md, decision 122), read through the host and turned into
   image data for yavcCanvas.render. Nothing is read that the host did not
   resolve, and nothing reaches the drawing that is not one of five kinds of
   picture, checked by its own bytes.

     yavcCanvasPictures.collect(doc, source, opts)
         -> Promise of { pictures: { path: data URL }, problems: [text] }

   source(path) resolves to { url }, a web address on this page's origin,
   to { bytes }, or to null for a picture that is not there. opts.signal
   ends the reading; opts.canvas is the canvas library when it is not
   window.yavcCanvas. */
(function () {
    'use strict';
    var MAX_PICTURES = 64, MAX_FILE = 1024 * 1024, MAX_TOTAL = 8 * 1024 * 1024, DEADLINE = 10000;
    var KINDS = { png: 'image/png', jpg: 'image/jpeg', jpeg: 'image/jpeg', gif: 'image/gif', webp: 'image/webp', svg: 'image/svg+xml' };
    function starts(bytes, at, values) { return values.every(function (v, i) { return bytes[at + i] === v; }); }
    /* The kind of picture the bytes are, when it is the kind the name says. */
    function kindOf(bytes, path) {
        var type = KINDS[(/\.([a-z]+)$/i.exec(path) || [])[1].toLowerCase()];
        if (type === 'image/png') { return starts(bytes, 0, [0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]) ? type : null; }
        if (type === 'image/jpeg') { return starts(bytes, 0, [0xff, 0xd8, 0xff]) ? type : null; }
        if (type === 'image/gif') { return starts(bytes, 0, [0x47, 0x49, 0x46, 0x38]) ? type : null; }
        if (type === 'image/webp') { return starts(bytes, 0, [0x52, 0x49, 0x46, 0x46]) && starts(bytes, 8, [0x57, 0x45, 0x42, 0x50]) ? type : null; }
        if (type !== 'image/svg+xml') { return null; }
        try {
            var text = new TextDecoder('utf-8', { fatal: true }).decode(bytes.subarray(0, lastWhole(bytes, 4096)));
            return /^﻿?\s*</.test(text) && /<svg[\s>]/i.test(text) ? type : null;
        } catch (err) { return null; }
    }
    /* The length of the first part of the bytes that ends on a whole UTF-8
       character, so that a cut through one is not taken for bad text. */
    function lastWhole(bytes, limit) {
        var end = Math.min(bytes.length, limit);
        if (end === bytes.length) { return end; }
        while (end > 0 && (bytes[end] & 0xc0) === 0x80) { end--; }
        return end;
    }
    function base64(bytes) {
        var text = '';
        for (var i = 0; i < bytes.length; i += 0x8000) { text += String.fromCharCode.apply(null, bytes.subarray(i, i + 0x8000)); }
        return btoa(text);
    }
    async function fetched(address, signal) {
        var url = new URL(address, location.href);
        if (!/^https?:$/.test(url.protocol) || url.origin !== location.origin || url.username || url.password) { throw new Error('the host gave an address outside this application'); }
        var response = await fetch(url.href, { credentials: 'same-origin', redirect: 'error', signal: signal });
        if (!response.ok) { throw new Error('the host returned status ' + response.status); }
        if (Number(response.headers.get('Content-Length')) > MAX_FILE) { throw new Error('it is larger than 1 MiB'); }
        if (!response.body || !response.body.getReader) {
            var whole = new Uint8Array(await response.arrayBuffer());
            if (whole.length > MAX_FILE) { throw new Error('it is larger than 1 MiB'); }
            return whole;
        }
        var reader = response.body.getReader(), parts = [], size = 0;
        for (;;) {
            var got = await reader.read();
            if (got.done) { break; }
            size += got.value.length;
            if (size > MAX_FILE) { await reader.cancel(); throw new Error('it is larger than 1 MiB'); }
            parts.push(got.value);
        }
        var bytes = new Uint8Array(size), at = 0;
        parts.forEach(function (part) { bytes.set(part, at); at += part.length; });
        return bytes;
    }
    async function one(path, source, signal) {
        if (path.length > 4096 || /[\x00-\x1f\x7f\\]/.test(path) || /^(?:[a-z][a-z0-9+.-]*:|\/\/)/i.test(path)) { throw new Error('its path is not a path to a file'); }
        var found = await source(path);
        if (signal.aborted) { throw new Error('the reading was ended'); }
        if (!found) { throw new Error('it was not found'); }
        var bytes = found.bytes ? (found.bytes instanceof Uint8Array ? found.bytes : new Uint8Array(found.bytes)) : await fetched(found.url, signal);
        if (bytes.length > MAX_FILE) { throw new Error('it is larger than 1 MiB'); }
        var type = kindOf(bytes, path);
        if (!type) { throw new Error('it is not the kind of picture its name says'); }
        return { href: 'data:' + type + ';base64,' + base64(bytes), size: bytes.length };
    }
    async function collect(doc, source, opts) {
        opts = opts || {};
        var J = opts.canvas || window.yavcCanvas, paths = [], pictures = {}, problems = [];
        doc.nodes.forEach(function (n) { if (J.isPicture(n) && paths.indexOf(n.file) < 0) { paths.push(n.file); } });
        if (!paths.length) { return { pictures: pictures, problems: problems }; }
        if (typeof source !== 'function') { return { pictures: pictures, problems: ['This application has not given canvases a way to read pictures, so each picture is shown by its name.'] }; }
        if (paths.length > MAX_PICTURES) { problems.push('A canvas draws at most ' + MAX_PICTURES + ' different pictures, and this one has ' + paths.length + '. The rest are shown by name.'); paths = paths.slice(0, MAX_PICTURES); }
        var life = new AbortController(), timer = setTimeout(function () { life.abort(); }, DEADLINE);
        function ended() { life.abort(); }
        if (opts.signal) { if (opts.signal.aborted) { life.abort(); } else { opts.signal.addEventListener('abort', ended, { once: true }); } }
        try {
            var read = await Promise.all(paths.map(function (path) {
                return one(path, source, life.signal).then(function (picture) { return { path: path, picture: picture }; }, function (err) { return { path: path, why: life.signal.aborted ? 'it was not read in time' : (err && err.message) || String(err) }; });
            }));
            var total = 0;
            read.forEach(function (r) {
                if (r.picture && total + r.picture.size > MAX_TOTAL) { r.why = 'the pictures of a canvas may be 8 MiB together'; r.picture = null; }
                if (r.picture) { total += r.picture.size; pictures[r.path] = r.picture.href; }
                else { problems.push('The picture ' + r.path + ' is shown by its name, because ' + r.why + '.'); }
            });
        } finally { clearTimeout(timer); if (opts.signal) { opts.signal.removeEventListener('abort', ended); } }
        return { pictures: pictures, problems: problems };
    }
    window.yavcCanvasPictures = Object.freeze({ collect: collect, LIMITS: Object.freeze({ pictures: MAX_PICTURES, file: MAX_FILE, total: MAX_TOTAL }) });
})();
