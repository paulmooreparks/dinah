/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* The small things every application of the site needs, written once
   (review of 2026-11-03: the HTML escaper had eight copies, the script
   loader ten, the download six). Loaded first, before the applications.

     yavcUtil.esc(s)                  text made safe for HTML and attributes
     yavcUtil.versioned(path)         a site script or stylesheet at this build's version
     yavcUtil.loadScript(src)         a promise; each address is loaded once
     yavcUtil.loadGlobal(src, name)   a promise of window[name], loading src if need be
     yavcUtil.siteFs()                a promise of the site file system, loaded and ready
     yavcUtil.download(name, blob)    hands the person a file through the browser
     yavcUtil.shortcut(key)           a CodeMirror key (Mod-b) as PUDL spells a shortcut (Mod+B)
     yavcUtil.notice(ref, text, kind) PUDL's persistent notice before ref
     yavcUtil.toast(text, kind)      a transient operation result, with a dialog fallback
     yavcUtil.documentCommands(service, application)   a host's commands for a document application, checked */
(function () {
    'use strict';
    if (window.yavcUtil) { return; }

    function esc(s) { return String(s == null ? '' : s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'); }

    /* This build's version, written into this script's own address by the
       page (internal/web), for the site's other shared scripts. */
    var V = (function () { var m = /[?&]v=([^&]+)/.exec((document.currentScript && document.currentScript.src) || ''); return m ? decodeURIComponent(m[1]) : (window.yavcStaticVersion || '0'); })();
    function versioned(path) { return path + (path.indexOf('?') < 0 ? '?' : '&') + 'v=' + encodeURIComponent(V); }

    var loading = {};
    function loadScript(src) {
        if (!loading[src]) {
            loading[src] = new Promise(function (resolve, reject) {
                var s = document.createElement('script');
                s.src = src;
                s.onload = function () { resolve(); };
                s.onerror = function () { delete loading[src]; reject(new Error(src + ' did not load')); };
                document.head.appendChild(s);
            });
        }
        return loading[src];
    }
    function loadGlobal(src, name) {
        if (window[name]) { return Promise.resolve(window[name]); }
        return loadScript(src).then(function () {
            if (!window[name]) { throw new Error(src + ' loaded without ' + name); }
            return window[name];
        });
    }

    var fsReady = null;
    function siteFs() {
        if (!fsReady) {
            fsReady = loadGlobal(versioned('/static/sitefs.js'), 'pcSiteFs')
                .then(function (F) { return F.load().then(function () { return F; }); })
                .catch(function (err) { fsReady = null; throw err; });
        }
        return fsReady;
    }

    function download(name, blob) {
        var url = URL.createObjectURL(blob), a = document.createElement('a');
        a.href = url; a.download = name;
        document.body.appendChild(a); a.click(); a.remove();
        setTimeout(function () { URL.revokeObjectURL(url); }, 10000);
    }

    /* A key as CodeMirror and the key-binding files write it (Mod-Shift-z,
       Ctrl-`, Alt--) in PUDL's spelling for a command's shortcut
       (Mod+Shift+Z), which PUDL shows per platform, as Ctrl or as the
       command key. */
    var KEY_PARTS = { mod: 'Mod', ctrl: 'Ctrl', c: 'Ctrl', control: 'Ctrl', alt: 'Alt', a: 'Alt', option: 'Alt', shift: 'Shift', s: 'Shift', meta: 'Meta', m: 'Meta', cmd: 'Meta' };
    function shortcut(key) {
        if (!key) { return undefined; }
        return String(key).split(/-(?!$)/).map(function (part, i, all) {
            if (i === all.length - 1) { return part.length === 1 ? part.toUpperCase() : part; }
            return KEY_PARTS[part.toLowerCase()] || part;
        }).join('+');
    }

    /* A message the person should see in full: PUDL's notice, dismissible,
       placed before ref (one per place, reused). An error (true or
       'danger') is a danger notice and is announced at once; 'problem' is
       a danger notice announced politely, for a fault in what the person
       is typing, which a preview redraws as they type and must not
       interrupt (PUDL review of the D2 drawings, 2026-11-03); 'warn' is a
       warning, announced politely; 'info' is a plain notice. A status line is
       for passing messages only (PUDL README, notices and toasts; review of
       2026-11-03: errors were shown three ways). Resolves to the notice;
       an empty text hides it. */
    function notice(ref, text, kind) {
        var host = ref && ref.parentNode;
        if (!host) { return null; }
        var n = ref.previousElementSibling && ref.previousElementSibling.classList.contains('yavc-notice') ? ref.previousElementSibling : null;
        if (!n) {
            n = document.createElement('div');
            n.className = 'notice yavc-notice';
            n.hidden = true;
            n.innerHTML = '<div class="notice-content"><p class="notice-body"></p></div><button class="notice-close" type="button" aria-label="Dismiss"></button>';
            n.querySelector('.notice-close').addEventListener('click', function () { n.hidden = true; n.dispatchEvent(new CustomEvent('yavc:notice-closed', { bubbles: true })); });
            host.insertBefore(n, ref);
        }
        if (!text) { n.hidden = true; return n; }
        var urgent = kind === true || kind === 'danger';
        n.classList.toggle('danger', urgent || kind === 'problem');
        n.classList.toggle('warn', kind === 'warn');
        n.setAttribute('role', urgent ? 'alert' : 'status');
        n.querySelector('.notice-body').textContent = text;
        n.hidden = false;
        return n;
    }

    /* Failed commands are transient. Persistent diagnostics and validation
       continue to use notice beside their owning content or field. */
    function toast(text, kind) {
        var severity = kind === true ? 'danger' : kind;
        function show() { return window.pudlToast(String(text), {kind: severity}); }
        if (window.pudlToast) { return Promise.resolve(show()); }
        var base = new URL((window.pudlHost || {}).pudl || '/pudl/', document.baseURI);
        return loadScript(versioned(new URL('pudl-toast.js', base).href)).then(show).catch(function () {
            return window.yavcDialog.open({title:'Operation Could Not Be Completed', body:'<p>'+esc(text)+'</p>', buttons:[{id:'ok',label:'OK',primary:true}]});
        });
    }

    /* Native modal dismissal can restore focus to its opener and thereby
       raise the old application. Preserve an explicit destination selected
       while the modal was open, without overriding subsequent focus moves. */
    function modalFocus(dialog) {
        var opener=document.activeElement, windows=window.pudlWindows;
        function top(){return windows?windows.state().top:null;}
        var origin=top();
        return {
            close:function(){
                var destination=top();
                if(dialog.open)dialog.close();
                if(windows&&destination&&destination!==origin&&(top()===origin||top()===destination)&&windows.state().open.includes(destination))windows.raise(destination);
            },
            restore:function(){
                if(document.querySelector('dialog[open]')||!opener||!opener.isConnected||!opener.focus)return;
                var active=document.activeElement;
                if(active&&active!==document.body&&active!==opener&&!dialog.contains(active))return;
                if(windows&&top()!==origin)return;
                opener.focus({preventScroll:true});
            }
        };
    }

    /* Compare the documented PUDL command presentation, excluding callback
       identity. Read dynamic fields with the command as their receiver. */
    function commandState(table) {
        var fields=['id','label','glyph','shortcut','disabled','reason','checked','choice','value','danger','badge','hidden','current','action','bind','glyphOnly','choiceLabel','toolbarChoice','toolbarPriority'];
        function value(command,key){var v=command[key];return typeof v==='function'?v.call(command):v;}
        function row(command){
            if(!command||typeof command!=='object')return command;
            var state={};fields.forEach(function(key){var v=value(command,key);if(v!==undefined)state[key]=v;});
            state.runnable=typeof command.run==='function';
            var items=value(command,'items');if(Array.isArray(items))state.items=items.map(row);
            return state;
        }
        return JSON.stringify(table.map(row));
    }
    function commandChanges(table,report,alive) {
        var previous=null,pending=false;
        return function(){
            if(pending||!alive())return;
            pending=true;
            Promise.resolve().then(function(){
                pending=false;if(!alive())return;
                var next=commandState(table());if(next===previous)return;
                previous=next;report();
            });
        };
    }

    /* The commands a host adds to a document application
       (docs/document-commands-contract.md), read once and checked, since
       they are drawn in the application's own menus. It gives { declared,
       state(d, context) }: the declarations that keep the rules, and a
       command's state for a document with an exception counted as not
       present. A host with no such service gives no declarations. */
    function documentCommands(service, application) {
        var declared = [];
        if (!service || service.version !== 1 || typeof service.list !== 'function' || typeof service.state !== 'function' || typeof service.run !== 'function') { return { declared: declared, state: function () { return { present: false, disabled: false }; } }; }
        try {
            (service.list(application) || []).slice(0, 8).forEach(function (d) {
                if (!d || typeof d.id !== 'string' || !/^[a-z][a-z0-9-]{0,39}$/.test(d.id) || typeof d.label !== 'string' || !d.label.trim() || d.label.length > 64 || /[\x00-\x1f\x7f]/.test(d.label)) { return; }
                if (declared.some(function (x) { return x.id === d.id; })) { return; }
                declared.push({ id: d.id, label: d.label, glyph: typeof d.glyph === 'string' && /^[a-z0-9-]{1,40}$/.test(d.glyph) ? d.glyph : undefined,
                    shortcut: typeof d.shortcut === 'string' && /^(?:(?:Mod|Ctrl|Alt|Shift)\+){0,3}(?:[A-Z0-9]|F[1-9]|F1[0-2])$/.test(d.shortcut) ? d.shortcut : undefined,
                    menu: d.menu === 'review' ? 'review' : 'file', context: d.context === true,
                    /* One command may stand in place of Save, the first that asks. */
                    replaces: d.replaces === 'save' && !declared.some(function (x) { return x.replaces; }) ? 'save' : null,
                    after: typeof d.after === 'string' && /^[a-z][a-z0-9-]{0,39}$/.test(d.after) ? d.after : null });
            });
        } catch (err) { declared = []; }
        return { declared: declared, state: function (d, context) {
            var s = null;
            try { s = service.state(d.id, context); } catch (err) { s = null; }
            return { present: !!(s && s.present === true), disabled: !!(s && s.disabled), reason: s && typeof s.reason === 'string' ? s.reason.slice(0, 200) : undefined };
        } };
    }

    /* The font families a font field suggests. A person may type any other family, or a list of them. */
    var fontChoices = Object.freeze(['Arial', 'Aptos', 'Calibri', 'Cambria', 'Candara', 'Consolas', 'Courier New', 'Garamond', 'Georgia', 'Helvetica', 'Inter', 'Liberation Sans', 'Liberation Serif', 'Palatino Linotype', 'Roboto', 'Segoe UI', 'Tahoma', 'Times New Roman', 'Trebuchet MS', 'Verdana', 'monospace', 'sans-serif', 'serif', 'system-ui']);
    window.yavcUtil = Object.freeze({ fontChoices: fontChoices, documentCommands: documentCommands, esc: esc, versioned: versioned, loadScript: loadScript, loadGlobal: loadGlobal, siteFs: siteFs, download: download, shortcut: shortcut, notice: notice, toast: toast, modalFocus: modalFocus, commandState: commandState, commandChanges: commandChanges, version: V });
})();
