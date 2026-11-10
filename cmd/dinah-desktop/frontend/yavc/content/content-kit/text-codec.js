/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Pure bounded Unicode codec; see docs/text-bytes-contract.md. */
(function (global) {
    'use strict';
    var MAX_BYTES = 100 * 1024 * 1024, MAX_TEXT = 100 * 1024 * 1024;
    function limit(value, fallback, ceiling) {
        if (value === undefined) return fallback;
        if (!Number.isSafeInteger(value) || value < 1 || value > ceiling) throw new TypeError('Invalid text size limit.');
        return value;
    }
    function bytes(value) {
        if (value instanceof ArrayBuffer) return new Uint8Array(value);
        if (value instanceof Uint8Array && value.buffer instanceof ArrayBuffer) return value;
        throw new TypeError('Source bytes must be an ArrayBuffer or Uint8Array.');
    }
    function validText(text, maxText, sourcePolicy) {
        if (typeof text !== 'string' || text.length > maxText) throw new TypeError('The source exceeds the text limit.');
        for (var i = 0; i < text.length; i++) {
            var c = text.charCodeAt(i);
            if (sourcePolicy && ((c < 32 && c !== 9 && c !== 10 && c !== 12 && c !== 13) || c === 127)) throw new TypeError('The source contains binary control characters.');
            if (c >= 0xD800 && c <= 0xDBFF) {
                var next = text.charCodeAt(++i);
                if (!(next >= 0xDC00 && next <= 0xDFFF)) throw new TypeError('The source contains an isolated Unicode surrogate.');
            } else if (c >= 0xDC00 && c <= 0xDFFF) throw new TypeError('The source contains an isolated Unicode surrogate.');
        }
        return text;
    }
    function encode(text, encoding, options) {
        options = options || {};
        var maxText = limit(options.maxText, 4*1024*1024, MAX_TEXT), maxBytes = limit(options.maxBytes, 8*1024*1024, MAX_BYTES);
        validText(text, maxText, false);
        var result;
        if (encoding === 'utf-8') {
            // Count before allocation; TextEncoder only implements UTF-8.
            var length = 0;
            for (var i = 0; i < text.length; i++) {
                var c = text.charCodeAt(i);
                length += c < 0x80 ? 1 : c < 0x800 ? 2 : c >= 0xD800 && c <= 0xDBFF ? (i++, 4) : 3;
                if (length > maxBytes) throw new TypeError('The source exceeds the byte limit.');
            }
            result = new TextEncoder().encode(text);
        } else if (encoding === 'utf-16le' || encoding === 'utf-16be') {
            if (text.length * 2 > maxBytes) throw new TypeError('The source exceeds the byte limit.');
            result = new Uint8Array(text.length * 2);
            var view = new DataView(result.buffer);
            for (var j = 0; j < text.length; j++) view.setUint16(j * 2, text.charCodeAt(j), encoding === 'utf-16le');
        } else throw new TypeError('The source encoding is unsupported.');
        return result;
    }
    function decode(input, options) {
        options = options || {};
        var source = bytes(input), maxBytes = limit(options.maxBytes, 8*1024*1024, MAX_BYTES), maxText = limit(options.maxText, 4*1024*1024, MAX_TEXT);
        if (source.byteLength > maxBytes) throw new TypeError('The source exceeds the byte limit.');
        var encoding = 'utf-8', bom = false;
        // UTF-32 signatures overlap UTF-16LE and must never be guessed as UTF-16.
        if (source.length >= 4 && ((source[0] === 255 && source[1] === 254 && source[2] === 0 && source[3] === 0) || (source[0] === 0 && source[1] === 0 && source[2] === 254 && source[3] === 255))) throw new TypeError('UTF-32 source is unsupported.');
        if (source.length >= 2 && source[0] === 255 && source[1] === 254) { encoding = 'utf-16le'; bom = true; }
        else if (source.length >= 2 && source[0] === 254 && source[1] === 255) { encoding = 'utf-16be'; bom = true; }
        else if (source.length >= 3 && source[0] === 239 && source[1] === 187 && source[2] === 191) bom = true;
        var text;
        try { text = new TextDecoder(encoding, { fatal: true, ignoreBOM: true }).decode(source); }
        catch (err) { throw new TypeError('The source is not valid ' + encoding + '.'); }
        validText(text, maxText, true);
        var encoded = encode(text, encoding, {maxBytes:maxBytes,maxText:maxText});
        if (encoded.length !== source.length || encoded.some(function (value, index) { return value !== source[index]; })) throw new TypeError('The source cannot be decoded without loss.');
        return Object.freeze({text:text,encoding:encoding,bom:bom,byteLength:source.byteLength});
    }
    var api = Object.freeze({decode:decode,encode:encode,maxBytes:MAX_BYTES,maxText:MAX_TEXT});
    if (typeof module === 'object' && module.exports) module.exports = api;
    else global.yavcTextCodec = api;
})(typeof window === 'undefined' ? null : window);
