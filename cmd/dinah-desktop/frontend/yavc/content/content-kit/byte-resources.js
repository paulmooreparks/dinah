/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Read-only host extension; see docs/text-bytes-contract.md. */
(function (global) {
    'use strict';
    var MAX = 8 * 1024 * 1024;
    function token(value, label) { if (typeof value !== 'string' || !value || value.length > 4096) throw new TypeError('Invalid byte resource ' + label + '.'); return value; }
    function current(context) { if (context && ((context.signal && context.signal.aborted) || (context.current && !context.current()))) throw new Error('The byte read was cancelled.'); }
    function create(host) {
        if (!host || host.version !== 1 || typeof host.read !== 'function') throw new Error('This host does not support bounded source bytes.');
        return Object.freeze({read:async function (resource, maxBytes, context) {
            token(resource,'identity');
            if (!Number.isSafeInteger(maxBytes) || maxBytes < 1 || maxBytes > MAX) throw new TypeError('Invalid byte read limit.');
            current(context);
            var result = await host.read({resource:resource,maxBytes:maxBytes}, context);
            current(context);
            if (!result || result.resource !== resource) throw new Error('The host returned a different byte resource.');
            var revision = token(result.revision,'revision'), value = result.bytes;
            if (!(value instanceof ArrayBuffer || value instanceof Uint8Array && value.buffer instanceof ArrayBuffer)) throw new Error('The host did not return source bytes.');
            if (value.byteLength > maxBytes) throw new Error('The byte resource exceeds the read limit.');
            var name = result.name, mediaType = result.mediaType;
            if (name != null && (typeof name !== 'string' || !name || name.length > 255 || /[\x00-\x1f\x7f]/.test(name))) throw new Error('The host returned an invalid file name.');
            if (mediaType != null && (typeof mediaType !== 'string' || mediaType.length > 255 || /[\x00-\x1f\x7f]/.test(mediaType))) throw new Error('The host returned an invalid media type.');
            var copied = new Uint8Array(value instanceof ArrayBuffer ? new Uint8Array(value) : value);
            if (copied.byteLength > maxBytes) throw new Error('The byte resource exceeds the read limit.');
            return Object.freeze({resource:resource,revision:revision,bytes:copied,name:name,mediaType:mediaType});
        }});
    }
    var api=Object.freeze({create:create,maxBytes:MAX});
    if (typeof module === 'object' && module.exports) module.exports=api;
    else global.yavcByteResources=api;
})(typeof window === 'undefined' ? null : window);
