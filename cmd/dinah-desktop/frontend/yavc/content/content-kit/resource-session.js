/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Capability selection and recovery validation shared by content applets. */
(function () {
    'use strict';
    async function open(host, load) {
        var authority = host.contentResources, prepared = host.contentResourceAdapter;
        if (!authority && prepared && host.snapshot && host.snapshot.version === 2 && typeof prepared.available === 'function' && await prepared.available()) authority = prepared;
        if (!authority) return null;
        if (!host.snapshot || host.snapshot.version !== 2) throw new Error('Resource editing requires snapshot service version 2.');
        await load('resources.js'); await load('recovery.js');
        var client = window.yavcContentResources.create(authority), recovery = window.yavcContentRecovery;
        return Object.freeze({ client: client, recovery: recovery,
            validate: function (value) {
                if (value == null) return;
                if (!Array.isArray(value.tabs) || (value.waiting && !Array.isArray(value.waiting))) throw new Error('The recovery snapshot is invalid. Its saved copy has been retained.');
                var tabs = (value.waiting || []).concat(value.tabs).map(function (tab) {
                    if (!tab || typeof tab !== 'object') throw new Error('The recovery tab is invalid. Its saved copy has been retained.');
                    return Object.assign({}, tab, { text: typeof tab.draft === 'string' ? tab.draft : tab.text });
                });
                recovery.validate({ version: value.version, tabs: tabs });
            }
        });
    }
    window.yavcResourceSession = Object.freeze({ open: open });
})();
