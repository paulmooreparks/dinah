/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Original revision baselines for host-owned recovery; no storage access. */
(function (global) {
    'use strict';
    var MAX_TEXT = 10 * 1024 * 1024, MAX_TOTAL = 32 * 1024 * 1024;
    var formats=typeof module==='object'&&module.exports?require('./resources.js'):global.yavcContentResources;
    function token(value) {
        if (typeof value !== 'string' || !value || value.length > 4096) throw new Error('Invalid recovery resource token.');
        return value;
    }
    function baseline(value, resource) {
        if (value == null) return null;
        if (![1,2].includes(value.version) || token(value.resource) !== resource) throw new Error('The recovery baseline belongs to another resource or version.');
        if(value.version===2&&(value.encoding===undefined||value.bom===undefined))throw new Error('The recovery encoding baseline is incomplete.');
        return Object.freeze(Object.assign({ version: value.version, resource: resource, revision: token(value.revision) },value.version===2?formats.format(value):{}));
    }
    function validate(value) {
        if (value == null) return;
        if (!value || typeof value !== 'object' || (value.version !== undefined && value.version !== 2) || !Array.isArray(value.tabs) || value.tabs.length > 512) throw new Error('This recovery snapshot has an unsupported format. It has been retained.');
        if (value.active !== undefined && (!Number.isInteger(value.active) || value.active < -1 || value.active >= Math.max(1, value.tabs.length))) throw new Error('The recovery selection is invalid.');
        var total = 0;
        value.tabs.forEach(function (tab) {
            if (!tab || typeof tab !== 'object') throw new Error('The recovery tab is invalid.');
            if (tab.path != null) token(tab.path);
            if (tab.name != null && (typeof tab.name !== 'string' || !tab.name || tab.name.length > 255 || /[\x00-\x1f\x7f]/.test(tab.name))) throw new Error('The recovered display name is invalid.');
            if (tab.text != null && (typeof tab.text !== 'string' || tab.text.length > MAX_TEXT)) throw new Error('The recovered draft exceeds its text limit.');
            if(tab.format!=null){if(tab.format.encoding===undefined||tab.format.bom===undefined)throw new Error('The recovered source encoding is incomplete.');formats.format(tab.format);}
            if(tab.newlines!==undefined&&(typeof tab.newlines!=='string'||tab.newlines.length>MAX_TEXT||!/^[LRC]*$/.test(tab.newlines)||tab.text!=null&&tab.newlines.length!==(tab.text.match(/\r\n|\r|\n/g)||[]).length))throw new Error('The recovered source line endings are invalid. The saved copy has been retained.');
            if(tab.sourceHistory!=null&&(!tab.sourceHistory||tab.sourceHistory.version!==1||!Array.isArray(tab.sourceHistory.done)||!Array.isArray(tab.sourceHistory.undone)||tab.sourceHistory.done.length+tab.sourceHistory.undone.length>200))throw new Error('The recovered source history is invalid. The saved copy has been retained.');
            total += tab.text ? tab.text.length : 0;
            if (total > MAX_TOTAL) throw new Error('The recovery snapshot exceeds its text limit.');
            baseline(tab.baseline, tab.path);
        });
    }
    function capture(client, snapshot, retained) {
        if (snapshot) {
            var issued = client.baseline(snapshot);
            return baseline(Object.assign({version:issued.encoding===undefined?1:2},issued),issued.resource);
        }
        return retained ? baseline(retained, retained.resource) : null;
    }
    async function restore(client, resource, retained, draft) {
        token(resource);
        var original = baseline(retained, resource), modified = draft != null;
        if (modified && (typeof draft !== 'string' || draft.length > MAX_TEXT)) throw new Error('The recovered draft exceeds its text limit.');
        // An old draft without a revision cannot acquire today's baseline.
        if (modified && !original) return { snapshot: null, baseline: null, text: draft, modified: true, conflict: 'unknown' };
        var current;
        try { current = await client.read(resource); }
        catch (err) {
            if (!modified) throw err;
            return { snapshot: null, baseline: original, text: draft, modified: true, conflict: 'unavailable' };
        }
        if (modified && current.revision !== original.revision) return { snapshot: null, baseline: original, text: draft, modified: true, conflict: 'changed' };
        if(modified&&(original.version===2&&(current.encoding!==original.encoding||current.bom!==original.bom)||original.version===1&&current.encoding!==undefined&&(current.encoding!=='utf-8'||current.bom)))return {snapshot:null,baseline:original,text:draft,modified:true,conflict:'encoding'};
        return { snapshot: current, baseline: original, text: modified ? draft : current.text, modified: modified, conflict: null };
    }
    var api = Object.freeze({ version: 1, validate: validate, baseline: baseline, capture: capture, restore: restore });
    if (typeof module === 'object' && module.exports) module.exports = api;
    else global.yavcContentRecovery = api;
})(typeof window === 'undefined' ? null : window);
