/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Versioned host extension; see docs/content-resource-contract.md. */
(function (global) {
    'use strict';
    var MAX_TEXT = 10 * 1024 * 1024;
    function token(value, name) {
        if (typeof value !== 'string' || !value.length || value.length > 4096) throw new TypeError('Invalid resource ' + name + '.');
        return value;
    }
    function text(value) {
        if (typeof value !== 'string' || value.length > MAX_TEXT) throw new TypeError('Resource text exceeds the supported limit.');
        return value;
    }
    function format(value){
        var encoding=value&&value.encoding===undefined?'utf-8':value&&value.encoding,bom=value&&value.bom===undefined?false:value&&value.bom;
        if(!['utf-8','utf-16le','utf-16be'].includes(encoding)||typeof bom!=='boolean'||encoding!=='utf-8'&&!bom)throw new TypeError('Unsupported text encoding or signature policy.');
        return Object.freeze({encoding:encoding,bom:bom});
    }
    function snapshot(value, resource, submitted, fallbackName) {
        if (!value || typeof value !== 'object' || value.resource !== resource) throw new TypeError('The host returned a different resource.');
        var result = { resource: token(resource, 'identity'), revision: token(value.revision, 'revision'), text: submitted === undefined ? text(value.text) : text(submitted), writable: value.writable === true };
        // A host whose new document has no saved location yet says so, and Save then asks for one (docs/content-creation-contract.md).
        if (value.needsLocation === true) result.needsLocation = true;
        var name = value.name === undefined ? fallbackName : value.name;
        if (name != null) { if (typeof name !== 'string' || !name || name.length > 255 || /[\x00-\x1f\x7f]/.test(name)) throw new TypeError('Invalid resource display name.'); result.name = name; }
        return Object.freeze(result);
    }
    function create(host) {
        if (!host || ![1,2].includes(host.version) || typeof host.read !== 'function') throw new TypeError('Resource host version 1 or 2 with read is required.');
        // Only snapshots issued by this client can authorize its writes.
        var issued = new WeakSet();
        function wanted(options){var value=format(options||{});if(host.version===1&&(value.encoding!=='utf-8'||value.bom))throw new Error('This host cannot preserve the selected encoding. Save an explicit UTF-8 copy on a supported host.');return value;}
        function request(value,encoding){return host.version===2?Object.assign(value,encoding):value;}
        function remember(response,resource,submitted,name,expected) {
            var value=snapshot(response,resource,submitted,name);
            if(host.version===2){
                if(response.encoding===undefined||response.bom===undefined)throw new Error('The host did not acknowledge the resource encoding.');
                var actual=format(response);
                if(expected&&(actual.encoding!==expected.encoding||actual.bom!==expected.bom))throw new Error('The host changed the submitted encoding.');
                value=Object.freeze(Object.assign({},value,actual));
            }
            issued.add(value);return value;
        }
        return Object.freeze({
            read: async function (resource) {
                token(resource, 'identity');
                return remember(await host.read({ resource: resource }), resource);
            },
            canWrite: function (value) { return !!value && issued.has(value) && value.writable && typeof host.write === 'function'; },
            canSaveAs: function () { return typeof host.saveAs === 'function'; },
            /* Whether Save should ask where to save: the host said this resource has no location yet, and it offers Save As. */
            needsLocation: function (value) { return !!value && issued.has(value) && value.needsLocation === true && typeof host.saveAs === 'function'; },
            canCreate: function () { return typeof host.create === 'function'; },
            canFormat:function(options){try{wanted(options);return true;}catch(err){return false;}},
            create: async function (resource, content,options) {
                if (typeof host.create !== 'function') throw new Error('This host cannot create a resource at an explicit destination.');
                token(resource, 'identity'); var submitted = text(content),encoding=wanted(options);
                return remember(await host.create(request({ resource: resource, text: submitted },encoding)),resource,submitted,undefined,encoding);
            },
            saveAs: async function (options, content, context) {
                if (typeof host.saveAs !== 'function') throw new Error('This resource host does not offer Save As.');
                var submitted = text(content), name = options && options.name;
                if (typeof name !== 'string' || !name || name.length > 255 || /[\x00-\x1f\x7f]/.test(name)) throw new TypeError('Invalid suggested file name.');
                var source = options.sourceResource == null ? null : token(options.sourceResource, 'identity');
                var encoding=wanted(options),result = await host.saveAs(request({ name: name, sourceResource: source, text: submitted },encoding), context);
                if (result == null) return null;
                var accepted = remember(result, token(result.resource, 'identity'), submitted,undefined,encoding);
                if (!accepted.name) throw new Error('Save As did not acknowledge a display name.');
                return accepted;
            },
            baseline: function (value) {
                if (!value || !issued.has(value)) throw new Error('The baseline was not issued by this resource client.');
                return Object.freeze(Object.assign({ resource: value.resource, revision: value.revision },host.version===2?format(value):{}));
            },
            write: async function (value, content) {
                if (!value || !issued.has(value) || !value.writable || typeof host.write !== 'function') throw new Error('This resource cannot be saved by this host.');
                var submitted = text(content), expected = value.revision,encoding=wanted(value);
                var result = await host.write(request({ resource: value.resource, expectedRevision: expected, text: submitted },encoding));
                var accepted = remember(result, value.resource, submitted, value.name,encoding);
                if (accepted.revision === expected && submitted !== value.text) throw new Error('The host did not advance the resource revision.');
                return accepted;
            }
        });
    }
    var api = Object.freeze({ create: create, format:format,maxText: MAX_TEXT });
    if (typeof module === 'object' && module.exports) module.exports = api;
    else global.yavcContentResources = api;
})(typeof window === 'undefined' ? null : window);
