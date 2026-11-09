/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
(function () {
    'use strict';
    var source = document.currentScript.src;
    var app = 'docs';
    window.pudlApplets.register(app, { init: function (root, opts) {
        var api = null, disposed = false, booted = false, state = opts.state, revision = 0;
        var ready = window.yavcUtil.loadScript(window.yavcUtil.versioned(new URL('../text-workspace/workspace.js', source).href)).then(function () {
            if (disposed) return;
            var started = revision;
            api = window.yavcTextWorkspace.init(root, Object.assign({}, opts, {application: app, state:state}));
            return Promise.resolve(api.ready).then(async function () {
                while(!disposed && started!==revision){started=revision;await api.setState(state);}
                if(disposed)return;booted=true;if(opts.commandsChanged)opts.commandsChanged();
            });
        });
        return { ready:ready, state:function () {return api ? api.state() : state;}, setState:function (value) {if(disposed)return;state=value;revision++;if(booted)return api.setState(value);},
            menus:function () {return api ? api.menus() : {titles:[],into:{}};}, commandTable:function () {return api ? api.commandTable() : [];}, toolbars:function () {return api ? api.toolbars() : [];},
            destroy:function () {disposed=true;if(api)api.destroy();} };
    }});
})();
