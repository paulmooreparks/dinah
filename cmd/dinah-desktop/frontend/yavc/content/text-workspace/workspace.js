/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Shared source/document workspace. The receiving host owns resources,
   recovery, preferences and optional file navigation. Editor and Docs
   register thin shells with fixed capability sets. */
(function () {
    'use strict';

    var SELF_SRC = document.currentScript.src;
    function beside(file) { return new URL(file, SELF_SRC).href; }
    /* The site's shared scripts load through static/yavc-util.js, each once. */
    var loadScript = window.yavcUtil.loadScript;
    /* The editing kit (apps/md-editor): CodeMirror, the page view, Mermaid,
       the formats, Insert and the dialogs, shared with Slides. */
    var kitReady = null;
    function loadKit() {
        if (window.yavcMdEditor) { return Promise.resolve(window.yavcMdEditor); }
        if (!kitReady) {
            kitReady = loadScript(window.yavcUtil.versioned(new URL('../md-editor/kit.js', SELF_SRC).href)).then(function () { return window.yavcMdEditor; }).catch(function (err) { kitReady = null; throw err; });
        }
        return kitReady;
    }
    function loadCodeMirror() { return loadKit().then(function (k) { return k.libs(); }); }


    /* Files is opt-in. Only an explicit saved choice opens the pane. */
    /* The Editor's settings are its own toolbar's Wrap lines and Explorer
       controls, which remember themselves in ~/.config/editor.json
       (js/config.js), so they follow the reader wherever ~ does. */

    /* === Key bindings ===================================================
       The Editor's commands have names, and a key is bound to a command by
       name. These are the bindings it starts with; ~/.config/editor.json's
       "keys" adds to them and overrides them, a key set to null does
       nothing at all (which turns off CodeMirror's own binding for it), and
       a change to the file applies at once. Keys are written as CodeMirror
       writes them: Mod- is Ctrl (Cmd on a Mac), then Alt-, Shift-, and the
       key, such as "Mod-Shift-;" or "Alt-d". */
    var DEFAULT_KEYS = {
        'Mod-Alt-o': 'toggle-outline', 'Alt-Shift-ArrowUp': 'section-up', 'Alt-Shift-ArrowDown': 'section-down',
        'Mod-s': 'save', 'Mod-o': 'open',
        'Mod-;': 'insert-timestamp', 'Mod-Shift-;': 'stamp-modified'
    };

    /* Now, in local time with its offset, as ISO 8601 and RFC 3339 write
       it (2026-10-06T16:12:00+08:00), or the local date alone, or in UTC
       with no offset, which is how an HTML page's meta dates are kept and
       read (Pages/page.cshtml.cs reads them in the "s" form). */
    function two(n) { return String(Math.floor(Math.abs(n))).padStart(2, '0'); }
    function now(form) {
        var d = new Date();
        if (form === 'utc') { return d.toISOString().slice(0, 19); }
        var date = d.getFullYear() + '-' + two(d.getMonth() + 1) + '-' + two(d.getDate());
        if (form === 'date') { return date; }
        var off = -d.getTimezoneOffset();
        return date + 'T' + two(d.getHours()) + ':' + two(d.getMinutes()) + ':' + two(d.getSeconds()) +
            (off >= 0 ? '+' : '-') + two(off / 60) + ':' + two(off % 60);
    }

    /* CodeMirror refuses a key with a modifier it doesn't know, and would
       refuse it at the first keypress, so a key is checked as it is read:
       its parts are those CodeMirror accepts (@codemirror/view's keymap). */
    function validKey(key) {
        var parts = String(key).split(/-(?!$)/), last = parts.pop();
        return !!last && parts.every(function (m) { return /^(cmd|meta|m|a|alt|c|ctrl|control|s|shift|mod)$/i.test(m); });
    }

    /* A key as a command shows it, in PUDL's spelling (static/yavc-util.js). */
    var keyLabel = window.yavcUtil.shortcut;

    function explorerWanted(s) {
        return !!(s && s.explorer === true);
    }

    var esc = window.yavcUtil.esc;

    /* Colours from PUDL's tokens, in the editing kit's code theme
       (apps/md-editor/kit.js codeTheme), shared with Diagrams. */
    function makeTheme(CM) { return window.yavcMdEditor.codeTheme(CM); }

    var uid = 0;

    function init(root, opts) {
        opts = opts || {};
        var app = opts.application, isDocs = app === 'docs';
        if (app !== 'editor' && !isDocs) throw new Error('The workspace requires a fixed application identity.');
        var hostServices = (window.pudlHost || {}).contentWorkspace;
        if (!hostServices || hostServices.version !== 1) throw new Error('Workspace host version 1 is required.');
        function loadSiteFs() { return hostServices.browser ? hostServices.browser().then(function (x) { return x.F; }) : Promise.resolve(null); }
        function loadBrowser() { return hostServices.browser ? hostServices.browser().then(function (x) { return x.FB; }) : Promise.resolve(null); }
        function loadConfig() { return Promise.resolve((window.pudlHost || {}).config || null); }
    var SETTINGS = { wrap: true, explorer: null, explorerWidth: null, outline: null, outlineWidth: null, keys: null, markdownView: 'visual', documentView: 'document', documentZoom: '100', navigation: 'files' };
    var MARKDOWN_VIEWS = (isDocs ? [['document', 'Document']] : []).concat([['visual', 'Visual'], ['text', 'Text'], ['split', 'Split'], ['preview', 'Preview']]);
    function supportsView(tab, mode) { return !!tab && (mode === 'text' || (tab.lang === 'markdown' && MARKDOWN_VIEWS.some(function (entry) { return entry[0] === mode; }))); }
    function resolvedView(tab, preferred) {
        if (supportsView(tab, preferred)) { return preferred; }
        return (isDocs ? ['document', 'visual', 'text'] : ['visual', 'text']).find(function (mode) { return supportsView(tab, mode); }) || 'text';
    }

        var n = ++uid;
        root.classList.add('pc-editor');
        if (opts.fit === 'fill') { root.classList.add('pc-editor-fill'); }
        /* The Editor's keys (Ctrl+F, Ctrl+S and the rest) work while focus
           is inside it. A press on a part that takes no focus of its own,
           such as the toolbar between its buttons, gives focus to the
           nearest ancestor that can take it, so the Editor's box takes it
           (out of the Tab order) rather than the page, whose keys are the
           browser's. */
        if (!root.hasAttribute('tabindex')) { root.tabIndex = -1; }
        root.innerHTML =
            /* The toolbar is PUDL's, drawn here from the Editor's command
               table and toolbars() (PUDL 0.68.0), folding its groups as the
               window narrows. */
            '<div class="ed-toolbar-slot" data-toolbar-slot></div>' +
            /* PUDL's splitter divides the Explorer from the work. */
            '<div class="ed-panes split" data-role="panes">' +
              /* The Explorer: the shared file browser's tree, files and all
                 (js/filebrowser.js), filled once the filesystem loads. */
              '<aside class="ed-explorer split-pane" data-role="explorer" aria-label="Files" hidden></aside>' +
              '<div class="split-handle ed-split" data-role="explorer-split" aria-label="Resize Files" data-split-min="160" data-split-max="50%" hidden></div>' +
              '<div class="ed-document-space split-pane">' +
              '<div class="tablist doc-tabs ed-tabs" role="tablist" aria-label="Open Files" data-role="tabs"></div>' +
              '<div class="ed-document-body split" data-role="document-panes">' +
              '<aside class="ed-outline-pane split-pane" data-role="outline-pane" aria-label="Document Outline" hidden></aside>' +
              '<div class="split-handle ed-split" data-role="outline-split" aria-label="Resize Outline" data-split-min="140" data-split-max="40%" hidden></div>' +
              '<div class="ed-work split-pane">' +
                '<div class="code-surface ed-surface" data-role="surface"></div>' +
                /* The page: the document as its page (page-view.js), edited
                   in Visual and Split and read in Preview. The page's find
                   bar sits over it (openFind). */
                '<div class="ed-page" data-role="page-box" hidden>' +
                  '<div class="ed-find-place" data-role="find-place" hidden></div>' +
                  '<div class="notice warn" data-role="style-problems" hidden role="status"><div class="notice-content"><p class="notice-body"></p></div></div>' +
                  '<div class="notice warn" data-role="page-problems" hidden role="status"><div class="notice-content"><p class="notice-body"></p></div></div>' +
                  '<div class="ed-scroll" data-role="scroll" tabindex="0"><div class="doc-page" data-role="page" data-ed-page></div></div>' +
                '</div>' +
              '</div>' +
            '</div>' +
            '<div class="ed-status" data-role="status-bar">' +
              '<span class="ed-readonly" data-role="readonly" hidden><span class="glyph" style="--glyph: var(--glyph-pencil-off)" aria-hidden="true"></span>Read-only</span>' +
              '<span data-role="where"></span><span data-role="lang"></span><span data-role="count"></span><span data-role="pages"></span><span data-role="pos"></span>' +
              '<span class="ed-message" data-role="message" role="status" aria-live="polite"></span>' +
            '</div></div></div>';

        var q = function (sel) { return root.querySelector(sel); };
        var el = {
            tabs: q('[data-role="tabs"]'), surface: q('[data-role="surface"]'), where: q('[data-role="where"]'),
            panes: q('[data-role="panes"]'), pageBox: q('[data-role="page-box"]'), scroll: q('[data-role="scroll"]'), page: q('[data-role="page"]'), findPlace: q('[data-role="find-place"]'),
            lang: q('[data-role="lang"]'), count: q('[data-role="count"]'), pages: q('[data-role="pages"]'), pos: q('[data-role="pos"]'), message: q('[data-role="message"]'),
            readonly: q('[data-role="readonly"]'),
            explorer: q('[data-role="explorer"]'), explorerSplit: q('[data-role="explorer-split"]'),outline:q('[data-role="outline-pane"]'),outlineSplit:q('[data-role="outline-split"]')
        };
        var FB = null, explorer = null;
        /* What the toolbar's commands show (the command table below): lines
           wrapped, the Markdown views offered, the formatting offered, a
           site page's source to open, and the file's address on the site. */
        var wrapOn = true, viewsOffered = false, formatOffered = false, sourceOffered = false, siteUrl = null;
        var reviewObservers=new Set(),reviewController=null,hostSuggestions=null;
        var reportCommands=window.yavcUtil.commandChanges(commandTable,function(){if(opts.commandsChanged)opts.commandsChanged();},function(){return !destroyed;});
        function commandsChanged() {
            if(destroyed)return;
            if(hostSuggestions)hostSuggestions.refresh();
            if(tabRow)tabRow.render();
            reviewObservers.forEach(function(listener){listener();});
            reportCommands();
        }

        var CM = null, sourceHistory = null, F = null, view = null, destroyed = false, startupFailed = false, unsubscribe = null, unwatchPictures = null, styleLifetime = new AbortController();
        var tabs = [], active = -1, untitled = 0;
        var H = window.pudlHost || {}, resourceSession = null, resources = null, recovery = null;
        var documentWindows = H.documentWindows, documentReady = false, receivingDocument = false, unbindDocuments = null;
        /* Commands the host adds to the File menu, to publish a document
           somewhere of the host's say (docs/document-commands-contract.md).
           The host says for each document whether a command is there and
           whether it can run, and the host runs it. What the host declares
           is checked here, since it is drawn in this applet's menu. */
        var hostCommands = (function () {
            var service = H.documentCommands, declared = [];
            if (!service || service.version !== 1 || typeof service.list !== 'function' || typeof service.state !== 'function' || typeof service.run !== 'function') return { declared: declared, rows: function () { return []; }, standIn: function () { return null; }, watch: function () { return null; } };
            try {
                (service.list(app) || []).slice(0, 8).forEach(function (d) {
                    if (!d || typeof d.id !== 'string' || !/^[a-z][a-z0-9-]{0,39}$/.test(d.id) || typeof d.label !== 'string' || !d.label.trim() || d.label.length > 64 || /[\x00-\x1f\x7f]/.test(d.label)) return;
                    if (declared.some(function (x) { return x.id === d.id; })) return;
                    declared.push({ id: d.id, label: d.label, glyph: typeof d.glyph === 'string' && /^[a-z0-9-]{1,40}$/.test(d.glyph) ? d.glyph : undefined,
                        shortcut: typeof d.shortcut === 'string' && /^(?:(?:Mod|Ctrl|Alt|Shift)\+){0,3}(?:[A-Z0-9]|F[1-9]|F1[0-2])$/.test(d.shortcut) ? d.shortcut : undefined,
                        menu: d.menu === 'review' ? 'review' : 'file', context: d.context === true,
                        /* One command may stand in place of Save, the first that asks. */
                        replaces: d.replaces === 'save' && !declared.some(function (x) { return x.replaces; }) ? 'save' : null,
                        after: typeof d.after === 'string' && /^[a-z][a-z0-9-]{0,39}$/.test(d.after) ? d.after : null });
                });
            } catch (err) { declared = []; }
            /* Whether the view in front has a selection, which is cheap
               to say each time the menus are drawn. Where it is in the
               text is worked out only when a command runs. */
            function onPage() { return pageActive() && (viewMode !== 'split' || editingPane !== 'text'); }
            function selected() {
                if (!view || readsAsPreview() && editingPane !== 'text') return false;
                return onPage() ? !!(visual.api.hasSelection && visual.api.hasSelection()) : !view.state.selection.main.empty;
            }
            /* The selection as a range of the document's text. One made on
               the page is the range it maps to, and null when it cannot be
               mapped exactly, as review proposals refuse it too. */
            async function selection() {
                if (!selected()) return null;
                function range(from, to) { return to > from ? { text: view.state.doc.sliceString(from, to), start: from, end: to } : null; }
                if (!onPage()) { var main = view.state.selection.main; return range(main.from, main.to); }
                var mapped = await visual.api.sourceSelection();
                return mapped ? range(mapped.from, mapped.to) : null;
            }
            function context() { var t = current(); return { application: app, resource: t && t.path || null, name: t ? t.name : '', modified: !!(t && dirtyTab(t)), selected: selected() }; }
            function state(d) {
                var s = null;
                try { s = service.state(d.id, context()); } catch (err) { s = null; }
                return { present: !!(s && s.present === true), disabled: !!(s && s.disabled), reason: s && typeof s.reason === 'string' ? s.reason.slice(0, 200) : undefined };
            }
            /* What is typed on the page is in the text before the host reads it. */
            async function run(d) {
                try {
                    if (pageActive()) await visual.api.sync();
                    if (destroyed || !state(d).present || state(d).disabled) return;
                    var chosen = await selection();
                    if (destroyed) return;
                    await service.run(d.id, Object.assign(context(), { selection: chosen, root: root, signal: styleLifetime.signal }));
                } catch (err) { if (!destroyed && !(err && err.name === 'AbortError')) say(String(err && err.message || err), true); }
                commandsChanged();
            }
            return {
                declared: declared,
                rows: function (ready) {
                    return declared.map(function (d) {
                        return { id: 'host-' + d.id, label: d.label, glyph: d.glyph, shortcut: d.shortcut,
                            hidden: function () { return !ready || !state(d).present; },
                            disabled: function () { return !ready || state(d).disabled; },
                            reason: function () { return state(d).reason; },
                            run: function () { return run(d); } };
                    });
                },
                /* The command that stands in place of Save for the document
                   in front, with its state, or null where Save is the
                   applet's own. */
                standIn: function () {
                    var d = declared.filter(function (x) { return x.replaces; })[0], s = d ? state(d) : null;
                    /* The button can fold to its glyph alone, so a glyph PUDL does not have gives way to Save's own. */
                    if (d && d.glyphKnown === undefined) d.glyphKnown = !!(d.glyph && getComputedStyle(root).getPropertyValue('--glyph-' + d.glyph).trim());
                    return s && s.present ? { label: d.label, glyph: d.glyphKnown ? d.glyph : undefined, disabled: s.disabled, reason: s.reason, run: function () { return run(d); } } : null;
                },
                watch: function () { return typeof service.onChange === 'function' && declared.length ? service.onChange(function () { if (!destroyed) commandsChanged(); }) : null; }
            };
        })();
        var unwatchHostCommands = hostCommands.watch();
        async function routeDocument(operation, input,guard) {
            if(guard&&!guard())throw new DOMException('Import cancelled.','AbortError');
            if (!documentWindows || documentWindows.mode(root) !== 'single' || !documentReady || receivingDocument || restoring) return null;
            if (operation === 'open-file') {
                var checked = resources ? await resources.read(input.path) : null;
                requireNative(checked ? checked.name : input.path);
                if (input.view && !supportsView({lang:languageFor(checked ? checked.name : input.path)},input.view)) throw new Error('This view is unavailable for this file.');
                if (!resources && F && !F.resolve(F.home(), input.path)) throw new Error('The file could not be found.');
            }
            if (pageActive()) await visual.api.sync();
            if(guard&&!guard())throw new DOMException('Import cancelled.','AbortError');
            var t = current();
            if (!t || (!t.path && !dirtyTab(t) && !view.state.doc.length)) return null;
            return {result: await documentWindows.open(root, app, {operation: operation, input: input})};
        }
        function resourceAsset(file) { return window.yavcUtil.versioned(new URL('../content-kit/' + file, SELF_SRC).href); }
        function refreshAccess(t, preserveRich) {
            var effects = [roComp.reconfigure([CM.EditorState.readOnly.of(!!t.readOnly), CM.EditorView.editable.of(!t.readOnly), CM.EditorView.contentAttributes.of({ 'aria-readonly': String(!!t.readOnly), tabindex: '0' })]), lspComp.reconfigure(languageAssistance(t))];
            if (current() === t) { view.dispatch({ effects: effects }); if (!preserveRich && viewMode !== 'text') return setView(viewMode); }
            else t.state = t.state.update({ effects: effects }).state;
        }
        var wrapComp = null, longLineField = null, wrapQueued = false, langComp = null, lintComp = null, roComp = null, keyComp = null, lspComp = null, lsp = null;
        var startFile = fileFrom(opts.state) || (opts.ownsUrl ? fileFrom(location.search) : null);
        var startLine = Number(new URLSearchParams(opts.state || (opts.ownsUrl ? location.search : '')).get('line')) || 1;

        function fileFrom(s) {
            if (!s) { return null; }
            var v = new URLSearchParams(String(s).replace(/^\?/, '')).get('file');
            return v || null;
        }

        /* === State ====================================================== */

        function current() { return tabs[active] || null; }
        function sourceFormat(t){return window.yavcContentResources.format(t&&t.resource&&t.resource.encoding!==undefined?t.resource:t&&t.format||{});}
        function normalizedSource(text) { return text.replace(/\r\n?/g, '\n'); }
        function submittedText(t) {
            return sourceHistory.raw(current() === t ? view.state : t.state);
        }
        function sourceModified(t,state){return t.saved===null||sourceHistory.raw(state||(current()===t?view.state:t.state))!==t.savedRaw;}
        function dirtyTab(t) { return !!(t && (t.modified || (t === current() && pageActive() && visual.api.pending()))); }
        function stateString() { var t = current(); return t && t.path ? 'file=' + encodeURIComponent(t.path).replace(/%2F/g, '/').replace(/%7E/g, '~') + '&view='+viewMode+(t.navigationLine > 1 ? '&line=' + t.navigationLine : '') : ''; }
        function announce(push) {
            if (opts.ownsUrl) {
                var qs = new URLSearchParams(location.search), t = current();
                if (t && t.path) { qs.set('file', t.path); } else { qs.delete('file'); }
                if (t && t.navigationLine > 1) { qs.set('line', String(t.navigationLine)); } else { qs.delete('line'); }
                var s = qs.toString().replace(/%2F/g, '/').replace(/%7E/g, '~');
                var address = location.pathname + (s ? '?' + s : '') + location.hash;
                if (address !== location.pathname + location.search + location.hash) { history[push ? 'pushState' : 'replaceState'](history.state, '', address); }
            }
            if (opts.changed) { opts.changed(stateString()); }
        }

        /* Command failures use toasts; ongoing document warnings remain
           beside the document. Routine confirmations use the status line. */
        function say(msg, isError) {
            if (isError && msg) {
                if (isError === true || isError === 'danger') { window.yavcUtil.toast(msg, 'danger'); }
                else { window.yavcUtil.notice(el.tabs, msg, isError); }
                el.message.textContent = ''; return;
            }
            el.message.textContent = msg || '';
        }

        /* === Languages ==================================================== */

        function languageFor(path) {
            var p = String(path || '');
            if (/\.(md|markdown)$/i.test(p)) { return 'markdown'; }
            if (/\.(json|canvas)$/i.test(p)) { return 'json'; }
            if (/\.css$/i.test(p)) { return 'css'; }
            if (/\.html?$/i.test(p)) { return 'html'; }
            if (/^~\/bin\/|^\/bin\/|\.sh$/i.test(p)) { return 'script'; }
            return 'text';
        }
        var LANG_NAMES = { markdown: 'Markdown', json: 'JSON', css: 'CSS', html: 'HTML', script: 'Script', text: 'Plain Text' };
        /* === Shortcodes in the text =========================================
           A {{shortcode}} is marked as one, its name and its arguments in
           their own colours; {{ offers the shortcodes' names and, inside a
           call, its parameters; and the pointer over one says what it is
           and takes. The bundled engine supplies the built-in catalog;
           optional host metadata can add names to it. */
        var SHORTCODE = /\{\{\/?[a-z][a-z0-9-]*(?::(?:[^}]|\}(?!\}))*)?\}\}/g;
        var shortcodeHighlight = null, shortcodeHover = null;
        /* Built when first asked for, since CodeMirror loads only when the
           first text is shown. */
        function shortcodeHighlighting() {
            if (shortcodeHighlight) { return shortcodeHighlight; }
            var marks = new CM.MatchDecorator({
                regexp: SHORTCODE,
                decorate: function (add, from, to, m) {
                    var head = /^\{\{\/?[a-z][a-z0-9-]*/.exec(m[0])[0].length;
                    add(from, from + head, CM.Decoration.mark({ class: 'cm-shortcode cm-shortcode-name' }));
                    if (to - 2 > from + head) { add(from + head, to - 2, CM.Decoration.mark({ class: 'cm-shortcode cm-shortcode-args' })); }
                    add(to - 2, to, CM.Decoration.mark({ class: 'cm-shortcode cm-shortcode-name' }));
                }
            });
            shortcodeHighlight = CM.ViewPlugin.define(function (v) {
                return { deco: marks.createDeco(v), update: function (u) { this.deco = marks.updateDeco(u, this.deco); } };
            }, { decorations: function (p) { return p.deco; } });
            return shortcodeHighlight;
        }
        var macroCache=null;
        function currentMacroEngine() {
            if(!macroContext||!view)return null;
            var text=view.state.doc.toString(),shared=macroOwner===current()?sharedMacros:'';
            if(!macroCache||macroCache.context!==macroContext||macroCache.text!==text||macroCache.shared!==shared)
                macroCache={context:macroContext,text:text,shared:shared,engine:macroContext.withMacros(text,shared)};
            return macroCache.engine;
        }
        function customShortcodes() { var engine=window.yavcMacroTools&&currentMacroEngine();return engine?window.yavcMacroTools.metadata(engine):[]; }
        function shortcodeByName(name) { return customShortcodes().concat(shortcodes || []).find(function (s) { return s.name === name; }) || null; }
        function shortcodeComplete(ctx) {
            var name = ctx.matchBefore(/\{\{\/?[a-z0-9-]*$/);
            if (name) {
                var closing = name.text.indexOf('{{/') === 0;
                return loadShortcodes().then(function (list) {
                    list = customShortcodes().concat(list);
                    return {
                        from: name.from + (closing ? 3 : 2),
                        validFor: /^[a-z0-9-]*$/,
                        options: list.filter(function (s) { return !closing || s.kind === 'container' || s.kind === 'span'; }).map(function (s) {
                            /* The braces close themselves as {{ is typed (closeBrackets), so only the name goes in, with a colon where arguments follow. */
                            return { label: s.name, type: 'keyword', detail: s.title, info: s.description + (s.example ? '\n\n' + s.example : ''),
                                apply: !closing && (s.argumentRequired || s.params.some(function(p){return p.required;})) ? s.name + ':' : s.name };
                        })
                    };
                });
            }
            var inside = ctx.matchBefore(/\{\{([a-z][a-z0-9-]*)(?::[^}]*)?[\s:]([a-z0-9-]*)$/);
            if (!inside) { return null; }
            var m = /\{\{([a-z][a-z0-9-]*)/.exec(inside.text), word = /[a-z0-9-]*$/.exec(inside.text)[0];
            return loadShortcodes().then(function () {
                var s = shortcodeByName(m[1]);
                if (!s || !s.params.length) { return null; }
                return {
                    from: ctx.pos - word.length,
                    validFor: /^[a-z0-9-]*$/,
                    options: s.params.map(function (p) { return { label: p.name, type: 'property', detail: p.required ? 'needed' : '', info: p.description, apply: p.name + '=' }; })
                };
            });
        }
        function shortcodeHovering() {
            if (shortcodeHover) { return shortcodeHover; }
            shortcodeHover = CM.hoverTooltip(function (v, pos) {
            var line = v.state.doc.lineAt(pos), text = line.text, m;
            SHORTCODE.lastIndex = 0;
            while ((m = SHORTCODE.exec(text))) {
                var from = line.from + m.index, to = from + m[0].length;
                if (pos < from || pos > to) { continue; }
                var name = /^\{\{\/?([a-z][a-z0-9-]*)/.exec(m[0])[1], s = shortcodeByName(name);
                if (!s) { loadShortcodes(); return null; }
                return { pos: from, end: to, above: true, create: function () {
                    var dom = document.createElement('div');
                    dom.className = 'ed-sc-tip';
                    dom.innerHTML = '<p class="ed-sc-tip-title"><strong>' + esc(s.title) + '</strong> <code>{{' + esc(s.name) + '}}</code></p><p>' + esc(s.description) + '</p>' +
                        (s.params.length ? '<dl>' + s.params.map(function (p) { return '<dt><code>' + esc(p.name) + '</code>' + (p.required ? ' (needed)' : '') + '</dt><dd>' + esc(p.description) + '</dd>'; }).join('') + '</dl>' : '');
                    return { dom: dom };
                } };
            }
            return null;
            }, { hoverTime: 350 });
            return shortcodeHover;
        }
        function shortcodeExtensions() {
            return [shortcodeHighlighting(), CM.autocompletion({ override: [shortcodeComplete], activateOnTyping: true, icons: false }), shortcodeHovering()];
        }

        function langExtension(lang) {
            if (lang === 'markdown') { return [CM.markdown(), shortcodeExtensions()]; }
            if (lang === 'json') { return CM.json(); }
            if (lang === 'css') { return CM.css(); }
            if (lang === 'html') { return CM.html(); }
            if (lang === 'script') { return CM.StreamLanguage.define(CM.shell); }
            return [];
        }

        /* JSON is checked as it is typed. */
        function lintExtension(tab) {
            if (tab.lang !== 'json' || tab.readOnly) { return []; }
            return [lsp ? [] : CM.linter(CM.jsonParseLinter()), CM.lintGutter()];
        }

        function stateFor(tab, text) {
            return CM.EditorState.create({ doc: text, extensions: extensionsFor(tab,text) });
        }
        function languageAssistance(tab) {
            return lsp && ['json','css','html'].indexOf(tab.lang) >= 0 ? lsp.extension(/\.canvas$/i.test(tab.path || tab.name) ? 'canvas' : tab.lang, !!tab.readOnly) : [];
        }
        var WRAP_LINE_LIMIT=65536;
        function rawLongLine(text){var count=0;for(var i=0;i<text.length;i++){if(text[i]==='\r'||text[i]==='\n')count=0;else if(++count>WRAP_LINE_LIMIT)return true;}return false;}
        function docLongLine(doc){var lines=doc.iterLines();while(!lines.next().done)if(lines.value.length>WRAP_LINE_LIMIT)return true;return false;}
        function longLinesActive(){return !!(view&&longLineField&&view.state.field(longLineField,false));}
        function refreshWrap(){
            if(wrapQueued)return;wrapQueued=true;Promise.resolve().then(function(){wrapQueued=false;if(destroyed||!view)return;view.dispatch({effects:wrapComp.reconfigure(wrapOn&&!longLinesActive()?CM.EditorView.lineWrapping:[])});commandsChanged();});
        }
        function extensionsFor(tab,raw) {
            return [
                    CM.lineNumbers(), CM.highlightActiveLineGutter(), CM.highlightSpecialChars(),
                    sourceHistory.extension(raw), longLineField, CM.drawSelection(), CM.dropCursor(), CM.indentOnInput(),
                    hostSuggestions?hostSuggestions.extension:[],
                    CM.bracketMatching(), CM.closeBrackets(), CM.rectangularSelection(), CM.crosshairCursor(),
                    CM.highlightActiveLine(), CM.highlightSelectionMatches(),
                    /* The Editor's own bindings come first, so they win over
                       CodeMirror's for the same key. */
                    keyComp.of(keyBindings()),
                    /* CodeMirror's history keys include Ctrl+U, undo selection,
                       which would answer before the menu's Underline does. */
                    CM.keymap.of([].concat(
                        [{key:'Mod-z',run:function(v){return sourceHistory.run('undo',v);}},{key:'Mod-y',run:function(v){return sourceHistory.run('redo',v);}},{key:'Mod-Shift-z',run:function(v){return sourceHistory.run('redo',v);}}],CM.closeBracketsKeymap, CM.defaultKeymap, CM.searchKeymap, CM.lintKeymap, [CM.indentWithTab])),
                    window.yavcMdEditor.codeSearch(CM, keepQuery),
                    makeTheme(CM),
                    wrapComp.of(wrapOn&&!rawLongLine(raw) ? CM.EditorView.lineWrapping : []),
                    langComp.of(langExtension(tab.lang)),
                    lintComp.of(lintExtension(tab)),
                    lspComp.of(languageAssistance(tab)),
                    roComp.of([CM.EditorState.readOnly.of(!!tab.readOnly), CM.EditorView.editable.of(!tab.readOnly), CM.EditorView.contentAttributes.of({ 'aria-readonly': String(!!tab.readOnly), tabindex: '0' })]),
                    CM.EditorState.transactionFilter.of(function (tr) { return tr.docChanged && tab.readOnly ? [] : tr; }),
                    CM.EditorView.contentAttributes.of({ 'aria-label': 'Text of ' + tab.name }),
                    CM.EditorView.updateListener.of(function (u) {
                        if(u.docChanged&&u.startState.field(longLineField)!==u.state.field(longLineField))refreshWrap();
                        if(hostSuggestions&&tabs[active]===tab)hostSuggestions.update(u);
                        if (u.docChanged && tabs[active] === tab) { markModified(tab); scheduleStyles(); scheduleOutline(); }
                        /* In Split, the page follows the text. */
                        if (u.docChanged && tabs[active] === tab && !visual.fromVisual) { textChangedForPage(); }
                        /* The paragraph style shows the line the cursor is on. */
                        if (u.selectionSet || u.docChanged) { showPos(); showCount(); commandsChanged(); }
                        if (tabs[active] === tab && (u.selectionSet || u.docChanged ||
                            CM.searchPanelOpen(u.state) !== CM.searchPanelOpen(u.startState))) { keep(); }
                    })
            ];
        }

        /* === Tabs ======================================================== */

        function markModified(tab) {
            var mod = !!tab.savePending || !!tab.saveFailed || sourceModified(tab,view.state);
            if (mod !== tab.modified) { tab.modified = mod; renderTabs(); }
        }

        /* PUDL's document tabs (pudl-tabs.js): the arrow keys move along
           them, and the close button or Delete asks, through
           pudl:tab-close, for the tab to close. */
        /* The tab row is the site's document tabs (static/doc-tabs.js),
           shown only while more than one document is open. */
        /* Right-clicks (PUDL's pudl:contextmenu): the text and the page get
           Edit and Format, a file in the pane gets Open. */
        root.addEventListener('pudl:contextmenu', function (e) {
            var t = e.detail.target;
            if (!t.closest) { return; }
            var node = t.closest('.ed-explorer a[data-path]');
            if (node) {
                var target = F && F.resolve(null, node.getAttribute('data-path'));
                if (!target) { return; }
                e.detail.mark(node);
                if (target.children) { e.detail.add([{ label: target.getAttribute ? 'Open' : 'Open', glyph: 'folder', run: function () { if (explorer) { explorer.go(target); } } }]); }
                else { e.detail.add([{ label: 'Open', glyph: 'document', run: function () { openPath(F.displayPath(F.realOf(target))); } }]); }
                return;
            }
            if (t.closest('.ed-surface, .ed-page, .cm-editor, .ProseMirror, [data-role="surface"], [data-role="page"]')) {
                var rows = ['undo', 'redo', '-', 'find', '-'];
                /* A diagram drawn from a file opens in the application that edits it. */
                var drawn = t.closest('[data-diagram-src]');
                if (drawn && root.contains(drawn) && canOpenDiagram()) { var drawnFile = drawn.getAttribute('data-diagram-src'); e.detail.mark(drawn); rows = [{ label: 'Edit Diagram', glyph: 'new-window', run: function () { openDiagram(drawnFile); } }, '-'].concat(rows); }
                if (formatOffered) { rows = rows.concat(window.yavcMdEditor ? window.yavcMdEditor.MENU.format : []); }
                /* The host's commands that asked to be offered here; one
                   that is not there for this document leaves no row. */
                var offered = hostCommands.declared.filter(function (d) { return d.context; }).map(function (d) { return 'host-' + d.id; });
                if (offered.length) rows = rows.concat(['-'], offered);
                e.detail.add(window.pudlCommands ? window.pudlCommands.menuRows(rows, root) : rows);
            }
        });
        var tabRow = window.yavcDocTabs ? window.yavcDocTabs.attach(el.tabs, {
            application: isDocs ? 'Docs' : 'Editor',
            tabs: function () { return tabs.map(function (t) { return { name: t.name, path: t.path, modified: dirtyTab(t), note: t.readOnly ? '(read-only)' : '' }; }); },
            active: function () { return active; },
            activate: async function (i) { await activate(i); view.focus(); },
            close: closeTab
        }) : null;
        function renderTabs() {
            if (tabRow) { tabRow.render(); }
            var t = current();
            el.where.textContent = t ? (t.path || 'Not saved yet') : '';
            el.where.title = el.where.textContent;
            el.lang.textContent = t ? LANG_NAMES[t.lang] : '';
            showCount();
            showFormat(viewMode);
            el.readonly.hidden = !(t && t.readOnly);
            var node = t && t.path && F ? F.resolve(F.home(), t.path) : null;
            sourceOffered = !!(F && F.mounted && node && t.readOnly && F.sourceOf(node));
            /* Any Markdown file has a page: Visual, Split and Preview. */
            viewsOffered = !!(t && t.lang === 'markdown');
            siteUrl = node && F.mounted ? F.publicUrl(node) : null;
            commandsChanged();
            keep();
        }

        /* An article's source under /wwwroot/content can be previewed (A11). */
        function previewable(node) {
            var r = F.realOf(node);
            return r.server === 'wwwroot' && /^content\/[A-Za-z0-9_-]+\.(md|html)$/.test(r.rel);
        }

        /* The words in a Markdown file, or in the text selected in it, with
           the time they take to read in the tooltip. It waits for a pause in
           typing, since counting reads the whole document. Every view
           counts the text, which the page keeps up to date as it is edited. */
        var countTimer = 0;
        function showCount() {
            clearTimeout(countTimer);
            countTimer = setTimeout(function () {
                var t = current(), K = window.yavcMdEditor;
                if (destroyed || !view || !K || !t || t.lang !== 'markdown') { el.count.textContent = ''; el.count.removeAttribute('title'); return; }
                var sel = view.state.selection.main, nf = new Intl.NumberFormat();
                var total = K.wordCount(view.state.doc.toString()), picked = sel.empty ? 0 : K.wordCount(view.state.sliceDoc(sel.from, sel.to));
                el.count.textContent = (picked ? nf.format(picked) + ' of ' : '') + nf.format(total) + (total === 1 ? ' word' : ' words');
                if (total) { el.count.title = 'About ' + K.readingMinutes(total) + ' min to read'; } else { el.count.removeAttribute('title'); }
            }, 250);
        }
        function showPos() {
            if (!view) { return; }
            var head = view.state.selection.main.head, line = view.state.doc.lineAt(head);
            el.pos.textContent = 'Line ' + line.number + ', column ' + (head - line.from + 1);
            markOutline();
        }

        async function leaveDocument() {
            var owner = current(), mode = viewMode;
            if (owner) owner.view = mode;
            if (pageActive()) await visual.api.sync();
            if (destroyed || current() !== owner) return;
            if (visual.api) await visual.api.enable(false);
            if (destroyed || current() !== owner) return;
            ++visual.gen;
            viewMode = 'text';
        }

        var activationQueue = Promise.resolve();
        function activate(i) {
            var target = tabs[i];
            var next = activationQueue.catch(function () {}).then(function () { return activateNow(target ? tabs.indexOf(target) : i); });
            activationQueue = next; return next;
        }
        async function activateNow(i) {
            var prev = current(), switching = i !== active;
            /* Each tab keeps its own view. The page belongs to the file it
               was made for, so the one coming to the front makes its own. */
            if (switching && viewMode !== 'text') { await leaveDocument(); if (destroyed) return; }
            if (prev && view) { prev.state = view.state; prev.scroll = scrollAnchor(); }
            active = i;
            var t = current();
            if (!t) { view.setState(CM.EditorState.create({ doc: '' })); refreshStyles(); renderTabs(); announce(); return; }
            view.setState(t.state);
            if(t.lang==='markdown')loadShortcodes().then(commandsChanged);
            refreshStyles();
            scheduleOutline();
            /* A tab kept from before the key bindings last changed takes
               the bindings as they are now. */
            view.dispatch({ effects: [keyComp.reconfigure(keyBindings()), lspComp.reconfigure(languageAssistance(t))] });
            /* Each tab keeps its place, and a search panel kept open from
               before a reload opens again. */
            scrollTo(t.scroll);
            if (t.openSearch) { t.openSearch = false; CM.openSearchPanel(view); }
            renderTabs();
            showPos();
            announce();
            /* Each tab keeps its view: the one it had, or for a Markdown
               file the one the settings open Markdown in. */
            var want = resolvedView(t, t.view || markdownDefault());
            if (want !== 'text' && t.lang === 'markdown' && want !== viewMode) { await setView(want); }
            markExplorer();
            say('', false);
            tabWarning(t);
        }
        /* What is wrong with the document in front stays beside it as a
           warning until it is put right: a recovered draft that can no
           longer be saved over its file, or a file changed somewhere else.
           The warning goes when the document it was about is saved under
           another name, closed, or no longer in front. */
        var tabWarned = false;
        function staleReason(t) {
            var keep = ' Use Save As to keep your text under another name.';
            if (t.conflict === 'unavailable') return t.name + ' could not be read, so Save is unavailable until it can be.' + keep;
            if (t.conflict === 'unknown') return 'This draft of ' + t.name + ' was kept without a record of the file it came from, so Save is unavailable.' + keep;
            return t.name + ' was changed elsewhere after you edited it, so Save is unavailable.' + keep;
        }
        function tabWarning(t) {
            var text = t && resources && t.conflict ? staleReason(t) : t && t.external ? 'This file was changed somewhere else since you opened it.' : '';
            if (text) { say(text, 'warn'); tabWarned = true; }
            else if (tabWarned) { window.yavcUtil.notice(el.tabs, ''); tabWarned = false; }
            return text;
        }

        async function addTab(t, text,guard) {
            await leaveDocument(); if (destroyed) return;
            if(guard&&!guard())return;
            if (documentWindows && documentWindows.mode(root) === 'single' && !restoring && tabs.length === 1 && !current().path && !dirtyTab(current()) && !view.state.doc.length) { tabs = []; active = -1; }
            t.savedRaw = text;
            t.format=sourceFormat(t);
            t.modified = false;
            t.state = stateFor(t, text);
            t.saved = t.state.doc.toString();
            tabs.push(t);
            await activate(tabs.length - 1);
            /* Focus would raise the Editor's window over one the reader has
               since brought forward, so it waits for the window to be in
               front. */
            if (inFront()) { view.focus(); }
        }
        function inFront() { return opts.ownsUrl || !root.closest('.win') || !!root.closest('.win.active'); }

        /* Opens a file by path: switches to it if it is open already. */
        /* With strict, a file that can't be opened throws, for the Open
           dialog to show; otherwise the status line says why. */
        function requireNative(name) { if (isDocs && (!/\.(md|markdown)$/i.test(name) || /\.slides\.md$/i.test(name))) throw new Error('Docs opens Markdown documents. Open this source in Editor or its native application.'); if(!isDocs && /\.(docx|pptx|pdf)$/i.test(name))throw new Error('This container requires an importer or viewer. It cannot be saved as source text.'); }
        async function openPath(path, strict, currentRequest, requestedView) {
            if (!tabs.some(function (t) { return t.path === path; })) {
                var routed = await routeDocument('open-file', {path: path, view: requestedView});
                if (routed) return true;
            }
            function fail(msg) { if (strict) { throw new Error(msg); } say(msg, true); return false; }
            function validView(name) {if(requestedView && !supportsView({lang:languageFor(name)},requestedView))throw new Error('This view is unavailable for this file in '+(isDocs?'Docs':'Editor')+'.');}
            if (resources) {
                for (var ri = 0; ri < tabs.length; ri++) if (tabs[ri].path === path) {validView(tabs[ri].name);await activate(ri); return true; }
                try {
                    var snapshot = await resources.read(path);
                    if (destroyed || currentRequest && !currentRequest()) return false;
                    var name = snapshot.name || String(path).split('/').pop(); requireNative(name);validView(name);
                    await addTab({ path: path, name: name, lang: languageFor(name), readOnly: !resources.canWrite(snapshot), resource: snapshot }, snapshot.text);
                    return true;
                } catch (err) { return fail(err.message); }
            }
            requireNative(path);
            validView(path);
            /* An address or kept tab from before the site moved under /site
               still finds its file. */
            path = F.upgradePath(path);
            var node = F.resolve(F.home(), path);
            // A restored named draft is already open even though Save has not created its file.
            var display = node && !node.children ? F.displayPath(F.realOf(node)) : path;
            for (var i = 0; i < tabs.length; i++) { if (tabs[i].path === display) { if (i !== active) { await activate(i); } view.focus(); return true; } }
            if (!node) { return fail(path + ': no such file'); }
            if (node.children) { return fail(path + ' is a folder'); }
            var r = F.realOf(node);
            if (!F.isText(r)) { return fail(display + ' isn\'t a text file, so the editor can\'t open it.'); }
            var text;
            try { text = await F.read(r); } catch (err) { return fail(err.message); }
            if(destroyed || currentRequest && !currentRequest())return false;
            var readOnly = !r.home || !!F.mounted;
            /* A server file remembers its time, which tells a change made
               elsewhere from this tab's own save. */
            await addTab({ path: display, name: r.name, lang: languageFor(display), readOnly: readOnly, mtime: r.server ? +r.mtime : null }, text);
            say(readOnly
                ? (F.mounted && F.sourceOf(r) ? 'This is the page as the site shows it, read-only. "Edit the source" opens the file it\'s made from.'
                    : 'This is part of the site, so it opens read-only. "Save a copy to ~" keeps an editable copy.')
                : '');
            return true;
        }

        /* A file that does not exist yet, in a folder of ~: a tab that
           creates it when saved. */
        async function newNamed(path) {
            requireNative(path);
            if (resources) { await addTab({path:path,name:String(path).split('/').pop(),lang:languageFor(path),readOnly:false,fresh:true}, ''); return; }
            var sp = F.splitPath(path), dir = F.resolve(F.home(), sp.dir);
            if (!dir || !dir.children || !dir.home || !F.validName(sp.base)) {
                await newTab();
                say(path + ': no such file, and one can\'t be made there', true);
                return;
            }
            var display = F.displayPath(dir).replace(/\/$/, '') + '/' + sp.base;
            await addTab({ path: display, name: sp.base, lang: languageFor(display), readOnly: false, fresh: true }, '');
            say(display + ' is a new file. Saving creates it.');
        }

        /* At docs.yavc.app the Editor starts for documents (docs-design,
           decision 17): the server marks the mount, and a new file is then
           a .docs.md document, which opens on its page. */
        var forDocuments = isDocs;
        async function newTab() {
            untitled++;
            if (forDocuments) {
                await addTab({ path: null, name: 'untitled-' + untitled + '.docs.md', lang: 'markdown', readOnly: false }, '');
                return;
            }
            await addTab({ path: null, name: 'untitled-' + untitled, lang: 'text', readOnly: false }, '');
        }

        async function closeTab(i) {
            var t = tabs[i];
            if (!t) { return; }
            if (dirtyTab(t)) {
                if (active !== i) { await activate(i); }
                var choice = await window.yavcDialog.askSave(t.name);
                if (choice == null) { return; }
                if (choice === 'save' && (!(await save()) || dirtyTab(t))) { return; }
            }
            /* The editor's text belongs to the tab in front. Closing another
               tab leaves it there and only renumbers. Closing the tab in
               front drops its text with it: active is cleared first, or
               activate() would keep the closed file's text in whichever tab
               slid into its place, and a save would write it there. */
            var wasActive = active;
            if (i === active) await leaveDocument();
            tabs.splice(i, 1);
            if (!tabs.length) { active = -1; await newTab(); return; }
            if (i !== wasActive) {
                active = i < wasActive ? wasActive - 1 : wasActive;
                renderTabs();
                return;
            }
            active = -1;
            await activate(Math.min(i, tabs.length - 1));
        }

        /* === Saving ====================================================== */

        /* Save as a person asks for it, from the button, the menu or the key. Where a
           command of the host stands in place of Save for the document, that
           command runs and the applet's own save does not. */
        function saveCommand() {
            var standIn = hostCommands.standIn();
            if (!standIn) return save();
            if (standIn.disabled) { say(standIn.reason || standIn.label + ' is not available now.', true); return false; }
            return standIn.run();
        }

        async function save() {
            var t = current();
            if (!t || t.readOnly) { return false; }
            if (!t.path || resources && resources.needsLocation(t.resource)) { return saveAs(); }
            return writeTo(t, t.path);
        }

        /* With strict, a write that fails throws, for the Save as dialog to
           show; otherwise the status line says why. */
        async function writeTo(t, path, strict) {
            if (current() === t && pageActive()) {
                await visual.api.sync();
                if (destroyed || current() !== t) throw new Error('The document changed before saving.');
            }
            var text = submittedText(t),format=sourceFormat(t);
            var ticket = t.saveTicket = (t.saveTicket || 0) + 1;
            t.savePending = (t.savePending || 0) + 1;
            var pending = (t.saveQueue || Promise.resolve()).then(function () {
                if (destroyed || tabs.indexOf(t) < 0) { throw new Error('The document was closed before its queued save could run.'); }
                return writeSubmitted(t, path, text, ticket,format);
            });
            t.saveQueue = pending.catch(function () { });
            try { return await pending; }
            catch (err) {
                if (!destroyed && tabs.indexOf(t) >= 0) { t.saveFailed = true; t.modified = true; if (current() === t) { say(err.message, true); } }
                if (strict) { throw err; }
                return false;
            } finally {
                t.savePending--;
                if (!destroyed && tabs.indexOf(t) >= 0) {
                    t.modified = !!t.savePending || !!t.saveFailed || sourceModified(t);
                    renderTabs(); keep();
                }
            }
        }
        async function writeSubmitted(t, path, text, ticket,format) {
            requireNative(t.path===path ? t.name : path);
            if (current() === t) { say('Saving…'); }
            if (resources) {
                var accepted;
                if (t.path === path && t.resource) {var currentFormat=sourceFormat(t);if(currentFormat.encoding!==format.encoding||currentFormat.bom!==format.bom)throw new Error('The source encoding changed before this queued save. Save again.');accepted = await resources.write(t.resource, text);}
                else if (t.path === path && !t.fresh) throw new Error('The recovered draft has no matching revision. Use Save As to preserve a copy.');
                else accepted = await resources.create(path, text,format);
                return acceptResource(t, accepted, text, ticket);
            }
            throw new Error('This host has no revision-safe writer. Save a copy on a supported host or download the source.');
        }

        async function saveAs() {
            var t = current();
            if (!t) { return false; }
            if (resources) return copyResource(t);
            /* A file of yours saves beside itself; anything else starts in ~. */
            var node = t.path ? F.resolve(F.home(), t.path) : null, r = node ? F.realOf(node) : null;
            var startDir = r && r.home && !t.readOnly ? F.displayPath(r.parent) : '~';
            var name = t.readOnly ? t.name + (/\.[a-z0-9]+$/i.test(t.name) ? '' : '.txt') : t.name;
            /* The dialog stays open while the file is written, and says
               there why it couldn't be. */
            var res = await FB.pick(F, {
                mode: 'save', title: t.readOnly ? 'Save a Copy' : 'Save As', host: root, start: (F && F.resolve(F.home(), '~/Documents')) ? '~/Documents' : '~',
                start: startDir, name: name, current: t.readOnly ? null : t.path, computer: true,
                commit: function (c) { return c.download ? true : writeTo(t, c.path, true); }
            });
            if (!res) { return false; }
            if (res.download) {
                await loadScript(resourceAsset('text-codec.js'));
                var format=sourceFormat(t),bytes=window.yavcTextCodec.encode((format.bom?'\ufeff':'')+submittedText(t),format.encoding);
                download(res.download, new Blob([bytes], { type: 'text/plain;charset='+format.encoding }));
                say('Downloaded ' + res.download + '. The tab still holds your text.');
                return false;
            }
            return true;
        }

        async function acceptResource(t, accepted, text, ticket) {
            if (destroyed || tabs.indexOf(t) < 0) return false;
            requireNative(accepted.name || t.name);
            var syncError = null;
            if (current() === t && pageActive()) {
                try { if (!resources.canWrite(accepted)) await visual.api.enable(false); else await visual.api.sync(); }
                catch (err) { syncError = err; if (!resources.canWrite(accepted)) visual.api.freeze(); }
                if (destroyed || !tabs.includes(t)) return false;
            }
            t.resource = accepted; t.baseline = null; t.conflict = null; t.path = accepted.resource;
            if (current() === t) tabWarning(t);
            t.format=sourceFormat(t);
            t.name = accepted.name || t.name; t.lang = languageFor(t.name); t.savedRaw = text; t.saved = normalizedSource(text);
            if(current()===t)window.yavcMdEditor.refreshReferences(root,picturesHost());
            t.readOnly = !resources.canWrite(accepted); t.saveFailed = !!syncError; t.fresh = false; t.external = false;
            t.modified = !!syncError || sourceModified(t) || ticket !== t.saveTicket;
            var effects = [langComp.reconfigure(langExtension(t.lang)), lintComp.reconfigure(lintExtension(t))];
            if (current() === t) view.dispatch({ effects: effects }); else t.state = t.state.update({ effects: effects }).state;
            await refreshAccess(t, !!syncError); if (destroyed || !tabs.includes(t)) return false;
            renderTabs(); announce(); keep();
            if (current() === t) say(syncError ? 'The submitted source was saved, but the newer rich draft could not synchronize. Copy it before leaving. ' + syncError.message : 'Saved ' + t.name + (t.modified ? '; newer edits are still unsaved.' : '.'), !!syncError);
            return true;
        }
        async function copyResource(t,requestedFormat) {
            if (current() === t && pageActive()) {
                await visual.api.sync();
                if (destroyed || current() !== t) throw new Error('The document changed before Save As.');
            }
            var text = submittedText(t),format=window.yavcContentResources.format(requestedFormat||sourceFormat(t)), ticket = t.saveTicket = (t.saveTicket || 0) + 1;
            t.savePending = (t.savePending || 0) + 1;
            var pending = (t.saveQueue || Promise.resolve()).then(async function () {
                if (destroyed || tabs.indexOf(t) < 0) throw new Error('The document closed before Save As.');
                var accepted = await resources.saveAs(Object.assign({ name: t.name, sourceResource: t.path },format), text, { host: root, signal: styleLifetime.signal, checkName:function (name) {try{requireNative(name);return null;}catch(err){return err.message;}} });
                return accepted ? acceptResource(t, accepted, text, ticket) : false;
            });
            t.saveQueue = pending.catch(function () {});
            try { return await pending; } catch (err) { if (!destroyed) say(err.message, true); return false; }
            finally { t.savePending--; if (!destroyed && tabs.includes(t)) { t.modified = !!t.savePending || !!t.saveFailed || sourceModified(t); renderTabs(); keep(); } }
        }
        async function encodedCopy(){
            var t=current();if(!t||!resources||!resources.canSaveAs())return;
            if(pageActive())await visual.api.sync();
            if(destroyed||current()!==t)return;
            var text=submittedText(t),formats=[['utf8','UTF-8',{encoding:'utf-8',bom:false}],['utf8bom','UTF-8 with Signature',{encoding:'utf-8',bom:true}],['utf16le','UTF-16 Little Endian',{encoding:'utf-16le',bom:true}],['utf16be','UTF-16 Big Endian',{encoding:'utf-16be',bom:true}]].filter(function(item){return resources.canFormat(item[2]);}),format=sourceFormat(t),selected=formats.find(function(item){return item[2].encoding===format.encoding&&item[2].bom===format.bom;})||formats[0];
            if(!selected)return;
            function unchanged(){return !destroyed&&current()===t&&submittedText(t)===text;}
            var value=await window.yavcDialog.askText('Save Encoded Copy','Encoding',selected[0],{choices:formats.map(function(item){return [item[0],item[1]];}),okLabel:'Choose Destination…',help:'Choose the encoding for the copy. The original file stays unchanged.',signal:styleLifetime.signal,validate:function(value){return !unchanged()?'The document changed. Cancel and try again.':formats.some(function(item){return item[0]===value;})?null:'Choose an encoding.';}});
            if(value!=null&&unchanged())return copyResource(t,formats.find(function(item){return item[0]===value;})[2]);
        }

        var download = window.yavcUtil.download;
        async function exportDocument(format,options) {
            var tab = current(); if (!tab || tab.lang !== 'markdown') { throw new Error('Choose a Markdown document to export.'); }
            if (pageActive()) { await visual.api.sync(); }
            if (destroyed || current() !== tab) { throw new Error('The document changed during export.'); }
            var text = view.state.doc.toString(), directory = styleDirectory(), pictures = picturesHost(); pictures.dir = function () { return directory; };
            function unchanged() { return !destroyed && current() === tab && view.state.doc.toString() === text; }
            var enginePromise = documentEngine(text), K = await loadKit(), m = await enginePromise, E = await K.exports();
            var R=await reviewModule(),output=await R.project({text:text,engine:m,root:root,signal:styleLifetime.signal,current:unchanged,interactive:!options||options.interactive!==false,policy:options&&options.reviewPolicy});
            if(output===null)throw new DOMException('Export cancelled.','AbortError');
            if(!unchanged())throw new Error('The document changed during export.');
            m=m.withMacros(output,sharedMacros);
            await K.styles();
            var front = m.front(output), path = '';
            (front.entries || []).forEach(function (entry) { if (entry.key === 'stylesheet') { path = entry.value; } });
            var result = await E.build(format, { text: output, engine: m, stylesheet: window.yavcStyleTools.resolve(path, directory),
                read: function (path) { if (!unchanged()) { throw new Error('The document changed during export.'); } return window.pudlHost.fs.readText(path); }, pictures: pictures, current: unchanged });
            result.name = (tab.path ? tab.path.split('/').pop().replace(/(?:\.docs)?\.md$/i, '') : 'document') + '.' + format;
            return result;
        }
        async function downloadDocument(format) {
            say('Writing the ' + format.toUpperCase() + ' file…');
            try { var result = await exportDocument(format); download(result.name, result.blob); say('Downloaded ' + result.name + '. ' + result.warnings.join(' ')); }
            catch (err) { if(err.name!=='AbortError')say('The document could not be exported: ' + err.message, true);else say('Export cancelled.'); }
        }

        /* === Dialogs ====================================================== */

        /* One dialog for questions and paths: resolves to the chosen
           button's name, or with an input, to the text entered. */
        async function openDialog() {
            if (!FB || !F) {
                if (!H.fs || !H.fs.open) throw new Error('This host does not offer file selection.');
                var chosen = await H.fs.open({host:root,title:'Open',signal:styleLifetime.signal,check:function (name) {
                    try {requireNative(name);return null;} catch(err) {return err.message;}
                }});
                if (!chosen || destroyed) return;
                if (chosen.path) return openPath(chosen.path,true);
                requireNative(chosen.name);
                await loadScript(resourceAsset('text-codec.js'));
                if (destroyed) return;
                var codec=window.yavcTextCodec, decoded=codec.decode(codec.encode(chosen.text,'utf-8'));
                await importedDocument(chosen.name,decoded.bom?decoded.text.slice(1):decoded.text,{encoding:decoded.encoding,bom:decoded.bom});
                return;
            }
            var t = current(), node = t && t.path && F ? F.resolve(F.home(), t.path) : null;
            /* It opens beside the file in front, or in ~. */
            var res = await FB.pick(F, {
                mode: 'open', title: 'Open', host: root, computer: true, signal:styleLifetime.signal,
                start: node ? F.displayPath(F.realOf(node).parent) : '~',
                check: function (r) { return F.isText(r)||resources&&!r.children&&r.home&&!r.server ? null : r.name + ' isn\'t a text file, so the editor can\'t open it.'; },
                /* The dialog stays open while the file is read, and says
                   there why it couldn't be. */
                commit: function (c) { return c.file ? openFromComputer(c.file, true) : openPath(c.path, true); }
            });
        }

        /* A file from the reader's computer opens as a new tab that belongs
           nowhere yet, so saving it asks where. */
        var COMPUTER_MAX = 5 * 1024 * 1024;
        async function openFromComputer(file, strict) {
            requireNative(file.name);
            function fail(msg) { if (strict) { throw new Error(msg); } say(msg, true); }
            if (file.size > COMPUTER_MAX) { return fail(file.name + ' is larger than 5 MB, too large to edit here.'); }
            await loadScript(resourceAsset('text-codec.js'));
            var decoded = window.yavcTextCodec.decode(new Uint8Array(await file.arrayBuffer()));
            if(destroyed)return false;
            var text = decoded.bom?decoded.text.slice(1):decoded.text;
            if (text.indexOf('\0') >= 0) { return fail(file.name + ' isn\'t a text file, so the editor can\'t open it.'); }
            await importedDocument(file.name,text,{encoding:decoded.encoding,bom:decoded.bom});
        }
        async function importWordBuffer(buffer,name) {
            var ticket=current(),stillCurrent=function(){return !destroyed&&current()===ticket;};
            await mdEngine();var K=await loadKit();await loadScript(K.here+'import-word.js'+K.stamp);
            var got=await window.yavcWordImport.docx(buffer,{kit:K,signal:styleLifetime.signal,current:stillCurrent});
            if(!stillCurrent())return false;
            await loadScript(K.here+'office-input.js'+K.stamp);
            var accepted=await window.yavcOfficeInput.report({title:'Word Import Report',root:root,signal:styleLifetime.signal,current:stillCurrent,
                summary:'The new native copy contains '+got.pictures+' image(s) and '+got.notes+' note(s). The original Word file stays unchanged.',messages:got.report});
            if(!accepted)return false;
            return importedDocument(String(name||'Imported Word').replace(/\.docx$/i,'')+'.docs.md',got.text,undefined,stillCurrent);
        }
        async function importWord(){
            var owner=current(),K=await loadKit();await loadScript(K.here+'office-input.js'+K.stamp);
            var selected=await window.yavcOfficeInput.pick({title:'Import Word',extension:'.docx',root:root,signal:styleLifetime.signal,current:function(){return !destroyed&&current()===owner;},browser:FB,fs:F,bytes:H.contentBytes});
            if(!selected||destroyed||current()!==owner)return false;
            return importWordBuffer(selected.buffer,selected.name);
        }
        async function importedDocument(name,text,format,guard) {
            requireNative(name);
            format=window.yavcContentResources.format(format||{});
            var routed = await routeDocument('import-source', {name: name, text: text,format:format},guard); if (routed) return routed.result;
            if(guard&&!guard())return false;
            var t={path:null,name:name,lang:languageFor(name),readOnly:false,format:format};await addTab(t,text,guard);
            if(!tabs.includes(t))return false;t.saved=null;t.modified=true;renderTabs();
            say('Opened '+name+'. Save As keeps a native copy through this host.');
        }

        /* === The Explorer pane ============================================ */

        function setupExplorer() {
            var back='<button type="button" class="btn btn-sm ed-navigation-back" data-close-navigation><span class="glyph" style="--glyph:var(--glyph-back)" aria-hidden="true"></span>Back to Document</button>';
            el.outline.innerHTML=back+'<h3 class="ed-pane-title">Outline</h3><div class="ed-outline-tools" role="group" aria-label="Section Order"><button type="button" class="btn btn-sm" data-section-move="up"><span class="glyph" style="--glyph:var(--glyph-arrow-up)" aria-hidden="true"></span>Move Up</button><button type="button" class="btn btn-sm" data-section-move="down"><span class="glyph" style="--glyph:var(--glyph-arrow-down)" aria-hidden="true"></span>Move Down</button></div><div class="ed-outline-panel" data-outline-tree></div>';
            if(F && FB){
            el.explorer.innerHTML=back+'<h3 class="ed-pane-title">Files</h3><div class="ed-files-panel">'+FB.markup.body({list:false,treeLabel:'Files'})+'</div>';

            el.explorer.querySelector('[data-close-navigation]').addEventListener('click',function () {compactNavigation('document');});
            explorer = FB.create(el.explorer.querySelector('.ed-files-panel'), F, {
                treeFiles: true,
                toggleFolders: true,
                onOpen: function (node) {
                    var r = F.realOf(node);
                    if (r.kind === 'link') { window.open(r.url, '_blank', 'noopener'); return; }
                    if (r.kind === 'app' || !F.isText(r)) {
                        var url = F.publicUrl(r);
                        if (url) { window.open(url, '_blank', 'noopener'); } else { say(node.name + ' isn\'t a file the editor can open.', true); }
                        return;
                    }
                    openPath(F.displayPath(r)).then(function (opened) {if(opened && root.clientWidth<=700)compactNavigation('document');});
                },
                say: function (m, isError) { if (m) { say(m, isError); } }
            });
            showExplorer(explorerWanted(settings));
            markExplorer();
            explorer.onChange(keep);
            }
            showOutline(typeof settings.outline==='boolean'?settings.outline:settings.navigation==='outline');
            el.outline.addEventListener('click', function (event) {
                if (event.target.closest('[data-close-navigation]')) { compactNavigation('document'); return; }
                var move = event.target.closest('[data-section-move]'); if (move) { moveOutline(move.dataset.sectionMove); return; }
                var link = event.target.closest('[data-outline-index]'); if (!link || event.button !== 0 || event.ctrlKey || event.metaKey || event.shiftKey || event.altKey) { return; }
                event.preventDefault(); navigateOutline(Number(link.dataset.outlineIndex));
            });
            el.outline.addEventListener('focusin', function (event) { var link = event.target.closest('[data-outline-index]'); if (link) { outlineTarget = Number(link.dataset.outlineIndex); outlineControls(); } });
            el.outline.addEventListener('keydown', function (event) { if (event.altKey && event.shiftKey && !event.ctrlKey && !event.metaKey && /^(ArrowUp|ArrowDown)$/.test(event.key)) { event.preventDefault(); event.stopPropagation(); moveOutline(event.key === 'ArrowUp' ? 'up' : 'down'); } }, true);
            el.outline.addEventListener('dragstart', function (event) {
                var link = event.target.closest('[data-outline-index]'); if (!link || !current() || current().readOnly) { event.preventDefault(); return; }
                outlineDrag = { index: Number(link.dataset.outlineIndex), text: outlineText, tab: current() }; event.dataTransfer.setData('text/plain', link.textContent); event.dataTransfer.effectAllowed = 'move';
            });
            el.outline.addEventListener('dragover', function (event) { var link = event.target.closest('[data-outline-index]'); if (link && outlineDrag) { var from = outlineEntries[outlineDrag.index], to = outlineEntries[Number(link.dataset.outlineIndex)]; if (from && to && from.level === to.level && from.parent === to.parent) { event.preventDefault(); event.dataTransfer.dropEffect = 'move'; } } });
            el.outline.addEventListener('drop', function (event) {
                var link = event.target.closest('[data-outline-index]'), drag = outlineDrag; outlineDrag = null;
                if (!link || !drag || drag.tab !== current() || drag.text !== view.state.doc.toString()) { return; }
                event.preventDefault(); var target = Number(link.dataset.outlineIndex); moveOutline(null, drag.index, target, event.clientY > link.getBoundingClientRect().top + link.getBoundingClientRect().height / 2);
            });
            el.outline.addEventListener('dragend', function () { outlineDrag = null; });
        }

        var outlineVisible = false, outlineEntries = [], outlineText = '', outlineOwner = null, outlineRenderedOwner = null, outlineRenderedShape = '', outlinePendingFocus = null, outlineTarget = -1, outlineDrag = null, outlineRevision = 0, outlineTimer = 0;
        function suspendOutline(place) {
            var link = document.activeElement && document.activeElement.closest('[data-outline-index]');
            if (link && place.contains(link) && outlineOwner === current()) {
                var entry = outlineEntries[Number(link.dataset.outlineIndex)];
                if (entry) { outlinePendingFocus = { owner: outlineOwner, key: outlineKey(outlineEntries, entry) }; }
            }
            place.inert = true; place.setAttribute('aria-busy', 'true');
        }
        function restoreOutlineFocus(place) {
            var pending = outlinePendingFocus; outlinePendingFocus = null;
            if (!pending || pending.owner !== current() || !outlineVisible || el.outline.hidden || document.activeElement !== document.body && !place.contains(document.activeElement)) { return; }
            var matches = outlineEntries.map(function (entry, i) { return outlineKey(outlineEntries, entry) === pending.key ? i : -1; }).filter(function (i) { return i >= 0; });
            if (matches.length === 1) { var link = place.querySelector('[data-outline-index="' + matches[0] + '"]'); if (link) { link.focus(); } }
        }
        function scheduleOutline() {
            clearTimeout(outlineTimer);
            var place = el.outline.querySelector('[data-outline-tree]');
            if (place && view && (current() !== outlineOwner || view.state.doc.toString() !== outlineText)) { suspendOutline(place); }
            outlineTimer = setTimeout(refreshOutline, 150);
        }
        function outlineControls() {
            var entry = outlineEntries[outlineTarget], editable = current() && !current().readOnly;
            el.outline.querySelectorAll('[data-section-move]').forEach(function (button) { button.disabled = !editable || !entry || (button.dataset.sectionMove === 'up' ? entry.previous : entry.next) < 0; });
        }
        function currentHeadingIndex() {
            if (!view || current() !== outlineOwner) { return -1; }
            if (pageActive() && visual.api.outlineIndex) { return visual.api.outlineIndex(); }
            var index = -1, position = view.state.selection.main.head; outlineEntries.forEach(function (entry, i) { if (entry.from <= position) { index = i; } }); return index;
        }
        function markOutline() {
            if (!view || !outlineEntries.length || current() !== outlineOwner) { outlineControls(); return; }
            var index = currentHeadingIndex();
            if (!el.outline.contains(document.activeElement)) { outlineTarget = index; }
            el.outline.querySelectorAll('[data-outline-index]').forEach(function (link) { if (Number(link.dataset.outlineIndex) === index) { link.setAttribute('aria-current', 'location'); } else { link.removeAttribute('aria-current'); } });
            outlineControls();
        }
        function headingHref(entry) {
            var url = new URL(opts.pageUrl || location.pathname, location.href), tab = current();
            if (opts.ownsUrl) { url = new URL(location.href); }
            if (tab && tab.path) { url.searchParams.set('file', tab.path); } else { url.searchParams.delete('file'); }
            url.searchParams.set('line', String(entry.line)); return url.href;
        }
        function refreshOutline() { return loadOutline().catch(function (err) { if (!destroyed) { say('The outline could not be updated: ' + err.message, true); } }); }
        async function loadOutline() {
            var revision = ++outlineRevision, tab = current();
            if (!view || !el.outline.querySelector('[data-outline-tree]')) { return; }
            var pendingPlace = el.outline.querySelector('[data-outline-tree]');
            if (tab !== outlineOwner || view.state.doc.toString() !== outlineText) { suspendOutline(pendingPlace); }
            var text = view.state.doc.toString(), m = tab && tab.lang === 'markdown' ? await mdEngine() : null;
            if (destroyed || revision !== outlineRevision || current() !== tab || view.state.doc.toString() !== text) { return; }
            var place = el.outline.querySelector('[data-outline-tree]'), focus = document.activeElement && document.activeElement.closest('[data-outline-index]'), oldFocus = focus && el.outline.contains(focus) ? Number(focus.dataset.outlineIndex) : null;
            var expanded = {}; el.outline.querySelectorAll('[data-outline-index][aria-expanded]').forEach(function (link) { expanded[link.dataset.outlineIndex] = link.getAttribute('aria-expanded'); });
            var entries = tab && tab.lang === 'markdown' ? m.outline(text) : [];
            var sameShape = tab === outlineOwner && JSON.stringify(entries.map(function (entry) { return [entry.level, entry.title]; })) === JSON.stringify(outlineEntries.map(function (entry) { return [entry.level, entry.title]; }));
            outlineEntries = entries; outlineText = text; outlineOwner = tab; outlineDrag = null;
            place.inert = false; place.setAttribute('aria-busy', 'false');
            if (!outlineVisible) { outlinePendingFocus = null; markOutline(); return; }
            var shape = JSON.stringify(entries.map(function (entry) { return [entry.level, entry.title]; }));
            if (outlineRenderedOwner === tab && outlineRenderedShape === shape && place.querySelector('.tree')) {
                place.querySelectorAll('[data-outline-index]').forEach(function (link, i) { link.href = headingHref(entries[i]); link.draggable = !tab.readOnly; });
                markOutline(); restoreOutlineFocus(place); return;
            }
            place.replaceChildren();
            outlineRenderedOwner = tab; outlineRenderedShape = shape;
            if (!entries.length) { var empty = document.createElement('p'); empty.className = 'ed-outline-empty'; empty.textContent = tab && tab.lang === 'markdown' ? 'No document headings yet.' : 'Outline is available for Markdown files.'; place.append(empty); outlinePendingFocus = null; outlineTarget = -1; outlineControls(); return; }
            var tree = document.createElement('ul'); tree.className = 'tree'; tree.setAttribute('aria-label', 'Document Outline'); tree.setAttribute('data-tree-toggles', 'buttons');
            var nodes = [];
            entries.forEach(function (entry, i) {
                var item = document.createElement('li'), link = document.createElement('a'); link.href = headingHref(entry); link.textContent = entry.title; link.dataset.outlineIndex = String(i); link.draggable = !tab.readOnly; link.setAttribute('data-tree-label', entry.title); item.append(link); nodes.push(item);
                if (entry.parent < 0) { tree.append(item); } else { var parent = nodes[entry.parent], list = parent.querySelector(':scope > ul'); if (!list) { list = document.createElement('ul'); parent.append(list); parent.querySelector(':scope > a').setAttribute('aria-expanded', sameShape && expanded[String(entry.parent)] || 'true'); } list.append(item); }
            });
            place.append(tree); markOutline(); if (window.pudlTree) { window.pudlTree.enhance(tree); }
            if (outlinePendingFocus) { restoreOutlineFocus(place); }
            else if (oldFocus != null) { var next = tree.querySelector('[data-outline-index="' + Math.min(oldFocus, entries.length-1) + '"]'); if (next) { next.focus(); } }
        }
        async function navigateOutline(index) {
            try { await navigateOutlineHeading(index); } catch (err) { if (!destroyed) { say(err.message, true); } }
        }
        async function navigateOutlineHeading(index) {
            var tab = current(), entry = outlineEntries[index]; if (!entry || tab !== outlineOwner) { return; }
            if (pageActive()) { await visual.api.sync(); }
            if (destroyed || current() !== tab) { return; }
            if (view.state.doc.toString() !== outlineText) {
                var identity = outlineKey(outlineEntries, entry);
                await refreshOutline();
                if (destroyed || current() !== tab || view.state.doc.toString() !== outlineText) { return; }
                var matches = outlineEntries.map(function (heading, i) { return outlineKey(outlineEntries, heading) === identity ? i : -1; }).filter(function (i) { return i >= 0; });
                if (matches.length !== 1) { say('The document headings changed. Choose the heading again.'); return; }
                index = matches[0]; entry = outlineEntries[index];
            }
            if (pageActive()) { await visual.api.reload(); }
            if (destroyed || current() !== tab || view.state.doc.toString() !== outlineText) { return; }
            view.dispatch({ selection: { anchor: entry.from }, effects: CM.EditorView.scrollIntoView(entry.from, { y: 'center' }) });
            if (pageActive() && visual.api.goToHeading) { visual.api.goToHeading(index); }
            else if (readsAsPreview()) { var block = el.page.querySelector('[data-line="' + entry.line + '"]'); if (block) { block.scrollIntoView({ block: 'center' }); } }
            else { view.focus(); }
            tab.navigationLine = entry.line; outlineTarget = index; markOutline(); announce(true);
            if (root.clientWidth<=700)compactNavigation('document');
        }
        function outlineKey(list, heading) {
            var parts = []; for (var node = heading; node; node = node.parent >= 0 ? list[node.parent] : null) { parts.unshift([node.level, node.title]); } return JSON.stringify(parts);
        }
        async function moveOutline(direction, index, target, after) {
            var tab = current(); if (!tab || tab.readOnly || !view) { return; }
            index = index == null ? el.outline.querySelector('.ed-outline-panel').contains(document.activeElement) ? outlineTarget : currentHeadingIndex() : index;
            var entry = outlineEntries[index], originalEntries = outlineEntries, destination = target == null ? null : outlineEntries[target];
            if (!entry || tab !== outlineOwner) { return; }
            try {
                if (pageActive()) { await visual.api.sync(); }
                var text = view.state.doc.toString(), m = await mdEngine();
                if (destroyed || current() !== tab || view.state.doc.toString() !== text) { scheduleOutline(); if (!destroyed) { say('The document changed. Choose the section again.', true); } return; }
                if (text !== outlineText) {
                    var entries = m.outline(text);
                    function rebind(heading) { var identity = outlineKey(originalEntries, heading), matches = entries.filter(function (candidate) { return outlineKey(entries, candidate) === identity; }); if (matches.length !== 1) { throw new Error('The document headings changed. Choose the section again.'); } return matches[0]; }
                    entry = rebind(entry); if (destination) { destination = rebind(destination); }
                }
                var request = { text: text, from: entry.from };
                if (direction) { request.direction = direction; } else { if (!destination) { return; } request[after ? 'after' : 'before'] = destination.from; }
                var moved = m.moveSection(request);
                if (moved.text === text) { return; }
                view.dispatch({ changes: { from: 0, to: text.length, insert: moved.text }, selection: { anchor: moved.from }, userEvent: 'input.move-section' });
                if (pageActive()) { await visual.api.reload(); }
                await refreshOutline();
                if (destroyed || current() !== tab || view.state.doc.toString() !== moved.text) { return; }
                var next = outlineEntries.findIndex(function (heading) { return heading.from === moved.from; });
                if (next >= 0 && pageActive()) { visual.api.goToHeading(next); }
                outlineTarget = next; outlineControls();
                if (next >= 0) { tab.navigationLine = outlineEntries[next].line; announce(); }
                if (next >= 0) { var link = el.outline.querySelector('[data-outline-index="' + next + '"]'); if (link) { link.focus(); } }
                say('The section moved. Undo restores its previous position.');
            } catch (err) { if (!destroyed) { say(err.message, true); } }
        }
        function toggleOutline() {
            if (!view) { return; }
            var on=!outlineVisible || root.clientWidth<=700 && !root.classList.contains('ed-compact-outline');showOutline(on);saveSetting({outline:on});if(on)compactNavigation('outline');
        }
        function hideNavigation() { showExplorer(false);showOutline(false);compactNavigation('document');saveSetting({explorer:false,outline:false}); }

        /* The settings, as last read or changed, and a change arriving from
           elsewhere: another Editor, another tab, or the file edited. */
        var settings = Object.assign({}, SETTINGS), unConfig = null, pendingSettings = {}, storedSettings = {};
        function applySettings(s, stored) {
            storedSettings=Object.assign({},stored || s);
            settings = Object.assign({}, SETTINGS, s);
            Object.keys(pendingSettings).forEach(function (key) { settings[key] = pendingSettings[key].value;storedSettings[key]=pendingSettings[key].value; });
            documentZoom = /^(width|page)$/.test(String(settings.documentZoom)) || Number(settings.documentZoom) >= 25 && Number(settings.documentZoom) <= 400 ? String(settings.documentZoom) : '100'; applyDocumentZoom();
            if (wrapOn !== !!settings.wrap) { setWrap(!!settings.wrap, false); }
            var showFiles = explorerWanted(settings);
            if (explorer && el.explorer.hidden === showFiles) { showExplorer(showFiles); }
            if (explorerSplit) { explorerSplit.set(typeof settings.explorerWidth === 'number' ? settings.explorerWidth : null); }
            if(outlineSplit)outlineSplit.set(typeof settings.outlineWidth==='number'?settings.outlineWidth:null);
            if(el.outline.firstChild)showOutline(typeof settings.outline==='boolean'?settings.outline:settings.navigation==='outline');
            schedulePaneLayout();
            if (view && keyComp) {
                view.dispatch({ effects: keyComp.reconfigure(keyBindings()) });
                if (keyProblems) { say(keyProblems, 'warn'); }
            }
        }

        /* The Explorer's width, dragged at its divider and kept with the
           other settings. */
        var explorerSplit = null,outlineSplit=null,paneLayoutObserver=null,paneLayoutFrame=0,paneFocus=false;
        function setupExplorerSplit() {
            if (!hostServices.split) { return; }
            explorerSplit = hostServices.split.of(el.explorerSplit, function (w) { saveSetting({ explorerWidth: w }); });
            explorerSplit.set(typeof settings.explorerWidth === 'number' ? settings.explorerWidth : null);
            outlineSplit=hostServices.split.of(el.outlineSplit,function (w) {saveSetting({outlineWidth:w});});outlineSplit.set(typeof settings.outlineWidth==='number'?settings.outlineWidth:null);
            paneLayoutObserver=new ResizeObserver(schedulePaneLayout);paneLayoutObserver.observe(root);schedulePaneLayout();
        }
        function saveSetting(changes) {
            Object.assign(settings, changes);Object.assign(storedSettings,changes);
            var ticket = {};
            Object.keys(changes).forEach(function (key) { pendingSettings[key] = { value: changes[key], ticket: ticket }; });
            return loadConfig().then(function (c) { return c.set(app, changes); }).catch(function () { }).finally(function () {
                Object.keys(changes).forEach(function (key) { if (pendingSettings[key] && pendingSettings[key].ticket === ticket) { delete pendingSettings[key]; } });
            });
        }

        function showExplorer(on) {
            el.explorer.hidden = !on;
            el.explorerSplit.hidden = !on;
            if (!on && root.classList.contains('ed-compact-files')) { compactNavigation('document'); }
            if (on && window.pudlSplit) { window.pudlSplit.refresh(); }
            commandsChanged();
            schedulePaneLayout();
        }
        function toggleExplorer() {
            if (!explorer) { return; }
            var on = el.explorer.hidden || root.clientWidth<=700 && !root.classList.contains('ed-compact-files');
            showExplorer(on);
            if(on)compactNavigation('files');else compactNavigation('document');
            saveSetting({ explorer: on });
        }
        function compactNavigation(pane) {root.classList.toggle('ed-compact-files',pane==='files');root.classList.toggle('ed-compact-outline',pane==='outline');paneFocus=pane==='document' && root.clientWidth<=700;schedulePaneLayout();}
        function showOutline(on) {outlineVisible=!!on;el.outline.hidden=!on;el.outlineSplit.hidden=!on;if(!on && root.classList.contains('ed-compact-outline'))compactNavigation('document');if(on)scheduleOutline();schedulePaneLayout();commandsChanged();}
        function schedulePaneLayout() {
            if(paneLayoutFrame || destroyed)return;
            paneLayoutFrame=requestAnimationFrame(function () {
                paneLayoutFrame=0;if(destroyed)return;
                var compact=root.clientWidth<=700,body=el.outline.parentElement;
                if(compact){el.panes.setAttribute('data-split-pane',root.classList.contains('ed-compact-files')?'first':'second');body.setAttribute('data-split-pane',root.classList.contains('ed-compact-outline')?'first':'second');}
                else {el.panes.removeAttribute('data-split-pane');body.removeAttribute('data-split-pane');if(explorerSplit)explorerSplit.set(typeof settings.explorerWidth==='number'?settings.explorerWidth:null);if(outlineSplit)outlineSplit.set(typeof settings.outlineWidth==='number'?settings.outlineWidth:null);}
                if(window.pudlSplit)window.pudlSplit.refresh();
                if(paneFocus){paneFocus=false;if(formatsThePage() && visual.api)visual.api.focus();else if(view)view.focus();}
            });
        }
        /* Wrap lines, as the toolbar, the View menu or a key switches it,
           kept with the settings when the reader changes it. */
        /* A saved preference overrides capability defaults. Document files
           and the Docs host share the document-view preference. */
        function viewKey() { var t = current(); return isDocs && t && /\.docs\.md$/i.test(t.path || t.name || '') ? 'documentView' : 'markdownView'; }
        function initialMode(tab, proposed) {var key=isDocs && /\.docs\.md$/i.test(tab.name)?'documentView':'markdownView';return supportsView(tab,storedSettings[key]) ? null : proposed;}
        function markdownDefault() {
            return MARKDOWN_VIEWS.some(function (v) { return v[0] === settings[viewKey()]; }) ? settings[viewKey()] : 'visual';
        }
        function setMarkdownDefault(v) {
            var change = {};
            change[viewKey()] = v;
            saveSetting(change);
            commandsChanged();
        }
        function setWrap(on, keepIt) {
            if(keepIt&&longLinesActive()){say('Line wrapping is unavailable while a line exceeds 65,536 characters.',true);return;}
            wrapOn = !!on;
            if (view) { view.dispatch({ effects: wrapComp.reconfigure(wrapOn&&!longLinesActive() ? CM.EditorView.lineWrapping : []) }); }
            if (keepIt) { saveSetting({ wrap: wrapOn }); }
            commandsChanged();
        }

        /* The Explorer marks the file in front and opens its way to it. */
        function markExplorer() {
            if (!explorer) { return; }
            var t = current(), node = t && t.path && F ? F.resolve(F.home(), t.path) : null;
            explorer.markCurrent(node ? F.pathOf(F.realOf(node)) : null);
        }

        /* === Views: Text, Visual and Split ================================= */

        /* A Markdown article's page is the public site's own page for the
           unsaved text, a draft the server keeps for an hour (A11). Visual
           and Split make that page the editing surface (js/editor-visual.js,
           Architecture/wysiwyg-editor-design.md), Visual alone and Split
           beside the text. Save publishes. Preview is not a view: it opens
           the page as it is on the site (openOnSite). */
        var viewMode = 'text';
        var visual = { api: null, front: null, fromVisual: false, timer: 0, loading: null, gen: 0 };
        var pageSettings = null, printFrame = null, previewPages = null, documentZoom = '100';
        function applyDocumentZoom() {
            var scale = 1;
            if (pagedView(viewMode) && pageSettings) {
                var css = getComputedStyle(el.scroll), width = el.scroll.clientWidth - parseFloat(css.paddingLeft) - parseFloat(css.paddingRight), height = el.scroll.clientHeight - parseFloat(css.paddingTop) - parseFloat(css.paddingBottom);
                scale = documentZoom === 'width' ? width / (pageSettings.width / .75) : documentZoom === 'page' ? Math.min(width / (pageSettings.width / .75), height / (pageSettings.height / .75)) : Number(documentZoom) / 100;
                scale = Math.max(.1, Math.min(4, scale || 1));
            }
            el.page.style.zoom = String(scale);
        }
        var zoomObserver = new ResizeObserver(applyDocumentZoom); zoomObserver.observe(el.scroll);
        function setDocumentZoom(value) {
            if (!/^(width|page)$/.test(String(value)) && (!Number.isFinite(Number(value)) || Number(value) < 25 || Number(value) > 400)) { throw new Error('Zoom must be 25 to 400 percent, Fit Width or Fit Page.'); }
            documentZoom = String(value); saveSetting({ documentZoom: documentZoom }); applyDocumentZoom(); commandsChanged();
        }
        function readsAsPreview() { var t = current(); return viewMode === 'preview' || !!(t && t.readOnly && viewMode !== 'text'); }
        function pagedView(mode) { return mode === 'document'; }
        function pageLayout(layout) {
            el.pages.textContent = layout.count + (layout.count === 1 ? ' page' : ' pages');
            pageNotice(layout.problem || (layout.oversized ? 'An indivisible block is taller than the page. Shorten it or enlarge the page before printing.' : ''));
        }
        function pageNotice(message) {
            var notice = root.querySelector('[data-role="page-problems"]');
            notice.hidden = !message; notice.querySelector('.notice-body').textContent = message;
        }
        function clearPages() {
            el.page.querySelectorAll(':scope > .doc-furniture').forEach(function (node) { node.remove(); }); el.page.style.zoom = '1';
            el.pages.textContent = ''; pageSettings = null; pageNotice('');
            el.page.classList.remove('doc-paginated'); el.page.style.minHeight = '';
            el.page.removeAttribute('data-page-count');
            ['width', 'height', 'left', 'top', 'right', 'bottom', 'gap'].forEach(function (key) { el.page.style.removeProperty('--sheet-' + key); });
        }
        async function configurePages() {
            var t = current(), text = view.state.doc.toString(), K = await loadKit(), P = await K.pages(), m = await mdEngine();
            if (destroyed || current() !== t || !pagedView(viewMode) || text !== view.state.doc.toString()) { return; }
            try { var S = window.yavcDocumentSettings, props = S.properties(m.front(text)); pageSettings = P.settings(props); pageSettings.furniture = S.furniture(props, m); P.surface(el.page, pageSettings); applyDocumentZoom(); pageNotice(''); }
            catch (err) { clearPages(); pageNotice(err.message); }
        }
        async function editPageSetup() {
            var t = current(); if (!t || t.readOnly || !view) { return; }
            if (pageActive()) { await visual.api.sync(); }
            var text = view.state.doc.toString(), m = await mdEngine(), K = await loadKit(), P = await K.pages(), UI = await K.pageSetup();
            function unchanged() { return !destroyed && current() === t && view.state.doc.toString() === text; }
            if (!unchanged()) { return; }
            try { await UI.open({ properties: P.properties(m.front(text)), current: unchanged, signal: styleLifetime.signal, save: function (values) {
                var next = m.frontset({ text: text, set: values, order: ['page', 'orientation', 'margins'] }).text;
                takeVisualText(next); textChangedForPage(); refreshStyles();
            } }); } catch (err) { say(err.message, true); }
        }
        async function editDocumentOptions(kind) {
            var t = current(); if (!t || t.readOnly || t.lang !== 'markdown') { return; }
            if (pageActive()) { await visual.api.sync(); }
            var text = view.state.doc.toString(), K = await loadKit(), P = await K.pages(), m = await mdEngine(), S = window.yavcDocumentSettings;
            function unchanged() { return !destroyed && current() === t && view.state.doc.toString() === text; }
            if (!unchanged()) { return; }
            try { await S.open(kind, { properties: S.properties(m.front(text)), styles: m.styles(text).map(function (block) { return block.css; }), stylesheet: currentSharedStyle, read: H.fs && H.fs.readText, engine: m, current: unchanged, signal: styleLifetime.signal, save: function (set, mappings) {
                var order = Object.keys(set).concat(Object.keys(mappings));
                takeVisualText(m.frontset({ text: text, set: set, mappings: mappings, order: order }).text,true); textChangedForPage(); refreshStyles();
            } }); } catch (err) { say(err.message, true); }
        }
        async function documentGallery() {
            var K=await loadKit();await K.pages();await K.loadScript(K.here+'document-gallery.js'+K.stamp);return window.yavcDocumentGallery;
        }
        async function newFromTemplate() {
            if(pageActive())await visual.api.sync();var tab=current(),text=view?view.state.doc.toString():'';
            function unchanged(){return !destroyed&&current()===tab&&(!view||view.state.doc.toString()===text);}
            var G=await documentGallery(),m=await mdEngine();if(!unchanged())return;
            var source=await G.template({engine:m,read:H.fs&&H.fs.readText,host:picturesHost(),signal:styleLifetime.signal,current:unchanged});
            if(source!==null&&source!==undefined&&unchanged())return liveActions['new@1']({text:source});
        }
        async function printDocument() {
            var t = current(); if (!t || t.lang !== 'markdown') { return; }
            try {
                if (pageActive()) { await visual.api.sync(); }
                var text = view.state.doc.toString(), K = await loadKit(), E = await K.exports(), P = await K.pages(), m = await documentEngine(text);
                function unchanged() { return !destroyed && current() === t && view.state.doc.toString() === text; }
                var R=await reviewModule(),output=await R.project({text:text,engine:m,root:root,signal:styleLifetime.signal,current:unchanged});if(output===null)return;
                m=m.withMacros(output,sharedMacros);
                if (printFrame) { printFrame.remove(); printFrame = null; }
                await E.prepare({ text: output, engine: m, stylesheet: currentSharedStyle, pictures: picturesHost(), read: window.pudlHost.fs.readText, current: unchanged,
                    renderPages: async function (surface, page) {
                        if (!unchanged()) { throw new Error('The document changed during print preparation.'); }
                        var d = P.surface(surface, page), flow = document.createElement('div'); flow.className = 'doc-page-flow';
                        Array.from(surface.querySelectorAll('.dinah-src[data-start],.doc-block,.dinah-block')).reverse().forEach(function (node) { node.replaceWith.apply(node, Array.from(node.childNodes)); });
                        flow.append.apply(flow, Array.from(surface.childNodes)); surface.append(flow);
                        await document.fonts.ready;
                        var blocks = P.blocks(flow), layout = P.plan(blocks, page);
                        if(layout.problem)throw new Error(layout.problem);
                        if (layout.oversized) { throw new Error('A block is taller than the page. Shorten it or enlarge the page before printing.'); }
                        var frame = document.createElement('iframe'); frame.className = 'ed-print-frame'; frame.title = 'Document Print'; frame.inert = true; frame.tabIndex = -1; frame.setAttribute('aria-hidden', 'true'); document.body.append(frame); printFrame = frame;
                        var doc = frame.contentDocument;
                        // A static standards-mode shell keeps table typography
                        // consistent with the document used for measurement.
                        doc.open();doc.write('<!doctype html><html><head></head><body></body></html>');doc.close();
                        frame.contentWindow.addEventListener('afterprint', function () { frame.remove(); if (printFrame === frame) { printFrame = null; } }, { once: true });
                        doc.title = t.name || 'Document';
                        document.querySelectorAll('head link[rel="stylesheet"], head style').forEach(function (node) { doc.head.append(node.cloneNode(true)); });
                        var css = doc.createElement('style'); css.textContent = '@page{size:' + page.width + 'pt ' + page.height + 'pt;margin:0}html,body{margin:0;padding:0;background:white} .doc-print-sheet{box-sizing:border-box;break-after:page;break-inside:avoid;margin:0!important;min-height:0!important;height:' + d.height + 'px!important;background-image:none!important;print-color-adjust:exact;-webkit-print-color-adjust:exact} .doc-print-sheet:last-child{break-after:auto} .doc-print-sheet .sc-page-break{display:none}'; doc.head.append(css);
                        var at = 0, starts = [0].concat(layout.gaps.map(function (gap) { return gap.index; })).concat(blocks.length);while(starts.length<layout.count+1)starts.push(blocks.length);
                        for (var i = 0; i < starts.length - 1; i++) {
                            var sheet = surface.cloneNode(false); sheet.classList.add('doc-print-sheet'); var body = doc.createElement('div'); body.className = 'doc-page-flow'; sheet.append(body);
                            body.append(window.yavcPageLines.fragment(flow, blocks, starts[i], starts[i + 1]));
                            (layout.footnotes||[]).filter(function(note){return note.page===i+1;}).forEach(function(note){var copy=P.noteFragment(note,blocks);copy.classList.add('doc-print-footnote');copy.style.position='absolute';copy.style.marginTop='0';copy.style.top=(d.top+note.top-i*(d.height+24))+'px';copy.style.left=d.left+'px';copy.style.width=(d.width-d.left-d.right)+'px';sheet.append(copy);});
                            var layer = doc.createElement('div'); layer.className = 'doc-furniture';
                            var header = window.yavcDocumentSettings.row(page, 'header', i + 1, starts.length - 1, doc), footer = window.yavcDocumentSettings.row(page, 'footer', i + 1, starts.length - 1, doc);
                            header.style.top = (page.margins[1] / 1.5 - 8) + 'px'; footer.style.top = ((page.height - page.margins[3] / 2) / .75 - 8) + 'px'; layer.append(header, footer); sheet.append(layer);
                            doc.body.append(sheet);
                        }
                        await Promise.all(Array.from(doc.querySelectorAll('link[rel="stylesheet"]')).map(function (link) { return link.sheet ? Promise.resolve() : new Promise(function (resolve, reject) { link.onload = resolve; link.onerror = function () { reject(new Error('A print stylesheet could not load.')); }; }); }));
                        await doc.fonts.ready; await Promise.all(Array.from(doc.images).map(function (img) { return img.decode(); }));
                        if (!unchanged()) { frame.remove(); throw new Error('The document changed during print preparation.'); }
                        frame.contentWindow.print(); say('Print uses the document page size and margins. Turn off browser headers and footers and select the matching paper size.');
                    }
                });
            } catch (err) { if (printFrame) { printFrame.remove(); printFrame = null; } say('The document could not be printed: ' + err.message, true); }
        }

        /* A call to one of the server's admin APIs, signing in again when the
           session has ended. The token is read at each try, since signing in
           replaces it. */
        function articleNode() {
            var t = current(), node = t && t.path && F ? F.resolve(F.home(), t.path) : null;
            return node && F.mounted && previewable(node) ? node : null;
        }
        function canEditVisually() { var node = articleNode(); return !!(node && /\.md$/i.test(F.realOf(node).rel)); }
        function slugOf(node) { return F.realOf(node).rel.replace(/^content\//, '').replace(/\.(md|html)$/i, ''); }

        /* === Views: Visual, Split and Preview =============================
           The page is Dinah Desktop's page view (page-view.js): the whole
           document as one ProseMirror editor in Visual, beside the text in
           Split, where each follows the other; Preview draws the page for
           reading. The Markdown goes to and from the page through the
           Markdown engine, md/yavc-md.wasm, which runs here in the browser,
           so the text never leaves it. */
        /* The page view's files sit beside this one and take its version,
           so a change to them ships with the Editor's own. */
        var HERE = new URL('./', SELF_SRC).href;
        var STAMP = SELF_SRC && SELF_SRC.indexOf('?') >= 0 ? SELF_SRC.slice(SELF_SRC.indexOf('?')) : '';
        /* The Markdown engine is shared with Slides: apps/md-engine/loader.js
           loads it once for the page. */
        function mdEngine() {
            return loadScript(window.yavcUtil.versioned(new URL('../md-engine/loader.js', SELF_SRC).href)).then(function () { return window.yavcMdEngine(); });
        }
        /* The shortcodes the host application draws
           (docs/content-workspace-contract.md, shortcodes and shortcodeHost).
           The host is asked for them once for the window. The document's
           engine reads them and writes a placeholder where one is, and the
           kit asks the host for each drawing (apps/md-editor/kit.js). A
           document drawn before the host has said waits a little for it,
           and is drawn again if the host says later. */
        var hostEntries = null, hostAsked = null, hostSettled = false, hostLate = false, hostedBase = null, hostedEngine = null, hostFills = new Map();
        function hostDraws() { var s = hostServices.shortcodeHost; return !!(s && s.version === 1 && typeof s.render === 'function'); }
        function hostShortcodes() {
            if (!hostAsked) {
                hostAsked = Promise.resolve().then(function () {
                    if (destroyed || typeof hostServices.shortcodes !== 'function') { return null; }
                    return hostServices.shortcodes({ signal: styleLifetime.signal, current: function () { return !destroyed; } });
                }).then(function (body) {
                    var list = body && Array.isArray(body.shortcodes) ? shortcodeCatalog([], body.shortcodes) : null;
                    hostSettled = true;
                    if (!destroyed && list && list.length && hostDraws()) { hostEntries = list; if (hostLate) { hostsArrived(); } }
                    return list;
                }, function () { hostSettled = true; return null; });
            }
            return hostAsked;
        }
        function hostsReady() {
            if (hostSettled || hostLate || typeof hostServices.shortcodes !== 'function' || !hostDraws()) { return Promise.resolve(); }
            return new Promise(function (resolve) {
                var timer = setTimeout(function () { hostLate = true; resolve(); }, 3000);
                hostShortcodes().then(function () { clearTimeout(timer); resolve(); });
            });
        }
        /* The engine with the host's shortcodes, where the host has any. */
        function hosted(m) {
            if (!hostEntries || typeof m.withHosts !== 'function') { return m; }
            if (hostedBase !== m) { hostedBase = m; hostedEngine = m.withHosts(hostEntries); }
            return hostedEngine;
        }
        function hostsArrived() {
            if (destroyed || !view) { return; }
            macroCache = null;
            if (pageActive()) { visual.api.textChanged(true); }
            if (readsAsPreview()) { schedulePreview(); }
            if (!pageActive() && !readsAsPreview() && macroContext) { documentEngine().catch(function () {}); }
        }
        /* What the engine made, once the drawings of the host's shortcodes
           in it are kept, so the page draws each in place as it builds. */
        function withHostDrawings(value) {
            return hostEntries && window.yavcMdEditor && window.yavcMdEditor.prefillHosts ? window.yavcMdEditor.prefillHosts(value, picturesHost()) : value;
        }
        var macroContext = null, macroOwner = null, sharedMacros = '', macroRevision = 0;
        async function documentEngine(definitions) {
            var t = current(), source = definitions == null ? view ? view.state.doc.toString() : '' : definitions, host = picturesHost(), directory = host.dir(), revision = ++macroRevision;
            host.dir = function () { return directory; };
            await hostsReady();
            var m = hosted(await mdEngine()), K = await loadKit(), M = await K.macros(), prepared = await M.prepare(m, source, host);
            if (!destroyed && revision === macroRevision && current() === t && view.state.doc.toString() === source) { macroContext = prepared.engine; macroOwner = t; sharedMacros = prepared.shared; }
            return revision === macroRevision ? prepared.engine : m.withMacros(source, macroOwner === t ? sharedMacros : '');
        }
        async function editDefinitions() {
            var t = current(); if (!t || t.readOnly) { return; }
            if (pageActive()) { await visual.api.sync(); }
            var text = view.state.doc.toString(), m = await mdEngine(), K = await loadKit(), M = await K.macros();
            function unchanged() { return !destroyed && current() === t && view.state.doc.toString() === text; }
            if (!unchanged()) { return; }
            try { await M.manage({ engine: m, text: text, host: picturesHost(), current: unchanged, signal: styleLifetime.signal,
                save: function (next) { takeVisualText(next); textChangedForPage(); refreshStyles(); } }); }
            catch (err) { say(err.message, true); }
        }
        async function insertFootnote() {
            var t=current();if(!t||t.readOnly||t.lang!=='markdown')return;
            var m=await mdEngine();
            if(destroyed||current()!==t||t.readOnly)return;
            if(pageActive()&&!await visual.api.prepareEdit())return;
            var rich=pageActive()?visual.api.footnoteSelection():null,text=view.state.doc.toString(),selection=view.state.selection;
            function unchanged(){return !destroyed&&current()===t&&!t.readOnly&&view.state.doc.toString()===text&&(rich?rich.current():view.state.selection.eq(selection));}
            if(!unchanged())return;
            if(pageActive()&&!rich){say('Place the caret in a paragraph before inserting a footnote.',true);return;}
            var tree=m.doc(text),used=new Set();function collect(node){if(node.attrs&&node.attrs.label)used.add(node.attrs.label);(node.content||[]).forEach(collect);}collect(tree);
            if(used.size>=256){say('A document supports at most 256 footnotes.',true);return;}
            var value=await window.yavcDialog.askText('Insert Footnote','Footnote Text','',{multiline:true,okLabel:'Insert',signal:styleLifetime.signal,help:'The reference is inserted after the selection. You can format the note on the page.',validate:function(value){if(!unchanged())return 'The document or selection changed. Cancel and try again.';return !value.trim()?'Enter the footnote text.':value.length>16384?'Footnote text is limited to 16,384 characters.':null;}});
            if(value==null||!unchanged())return;
            if(rich){rich.apply(value);return;}
            var index=1;while(used.has('note-'+index))index++;var label='note-'+index;
            var content=value.split(/\n\s*\n/).map(function(part){var children=[];part.split('\n').forEach(function(line,i){if(i)children.push({type:'soft_break'});if(line)children.push({type:'text',text:line});});return {type:'paragraph',content:children};});
            var definition=m.markdown({type:'doc',attrs:{tail:''},content:[{type:'footnote',attrs:{label:label,tail:''},content:content}]}),at=selection.main.to;
            var next=text.slice(0,at)+'[^'+label+']'+text.slice(at);next+=(next.endsWith('\n\n')?'':next.endsWith('\n')?'\n':'\n\n')+definition+'\n';
            takeVisualText(next,true);textChangedForPage();
        }
        async function pageParts() {
            await mdEngine();
            await loadKit().then(function (k) { return k.pageView(); });
        }
        async function reviewModule(){var K=await loadKit();if(!window.pudlListbox)await K.loadScript(new URL('pudl-listbox.js',new URL(H.pudl,document.baseURI)).href);await K.loadScript(K.here+'review-tools.js'+K.stamp);return window.yavcReviewTools;}
        async function captureReview(selection){
            var owner=current();if(!owner||owner.lang!=='markdown')throw new Error('Choose a Markdown document to review.');
            if(pageActive()){if(owner.readOnly)await visual.api.sync();else await visual.api.prepareEdit();}
            if(destroyed||current()!==owner)return null;
            var text=view.state.doc.toString(),selected=view.state.selection,m=await mdEngine(),range=null;
            function unchanged(){return !destroyed&&root.isConnected&&current()===owner&&view.state.doc.toString()===text&&!(pageActive()&&visual.api.pending());}
            if(!unchanged())return null;
            if(selection){
                if(readsAsPreview()&&editingPane!=='text')throw new Error('Choose Text or an editable document view before proposing a change.');
                if(pageActive()&&(viewMode!=='split'||editingPane!=='text')){range=await visual.api.sourceSelection();if(!range)throw new Error('This selection cannot be mapped safely. Use Text to propose the change.');}
                else range={from:selected.main.from,to:selected.main.to,current:function(){return view.state.selection.eq(selected);}};
                if(!unchanged()||!range.current())return null;
            }
            var author='';m.front(text).entries.forEach(function(entry){if(entry.key==='author')author=entry.value;});
            return {text:text,engine:m,writable:!owner.readOnly,current:unchanged,from:range?range.from:0,to:range?range.to:0,selectionCurrent:range?range.current:null,author:author,
                apply:async function(result){
                    if(!unchanged()||owner.readOnly)throw new Error('The document changed or is no longer editable.');
                    var changes=CM.ChangeSet.of(result.edits,text.length);if(changes.apply(view.state.doc).toString()!==result.text)throw new Error('The review result did not match its source edits.');
                    var anchor=changes.mapPos(result.edits.length?result.edits[0].from:view.state.selection.main.head,-1);
                    view.dispatch({changes:changes,selection:{anchor:anchor},annotations:[CM.Transaction.userEvent.of('input.review'),CM.isolateHistory.of('full')],scrollIntoView:true});
                }};
        }
        async function reviewTools(){
            if(reviewController)return reviewController;var R=await reviewModule();
            if(!reviewController)reviewController=R.create({root:root,signal:styleLifetime.signal,capture:captureReview,say:say,watch:function(listener){reviewObservers.add(listener);return function(){reviewObservers.delete(listener);};},navigate:async function(mark){await setView('text');view.dispatch({selection:{anchor:mark.from,head:mark.to},scrollIntoView:true});view.focus();}});
            return reviewController;
        }
        function reviewProposable(){var t=current();return !!(CM&&view&&t&&t.lang==='markdown'&&!t.readOnly&&(!readsAsPreview()||editingPane==='text'));}
        function wireReview(target){
            target.querySelectorAll('[data-critic-raw]').forEach(function(mark){if(mark.querySelector(':scope > .pm-critic-open'))return;var button=document.createElement('button');button.type='button';button.className='btn btn-sm pm-critic-open';button.setAttribute('aria-label','Review annotation');button.title='Review annotation';button.innerHTML='<span class="glyph" style="--glyph:var(--glyph-comments)" aria-hidden="true"></span>';button.addEventListener('click',function(event){event.preventDefault();event.stopPropagation();reviewTools().then(function(controller){return controller.browse(mark.getAttribute('data-critic-raw'));}).catch(function(err){say(err.message,true);});});mark.appendChild(button);});
        }
        function pageActive() { return !!(visual.api && visual.api.active()); }
        function pageHost() {
            return {
                signal: styleLifetime.signal,
                /* The / menu on the page offers the Insert menu's shortcodes. */
                inserts: function () {
                    var extra = [];
                    if (shortcodes && (F && F.mounted || hostEntries)) {
                        shortcodes.filter(function (s) { return !s.builtin && !s.managed && s.name !== 'column' && !SHORTCODES.some(function (i) { return i.name === s.name; }); }).forEach(function (s) {
                            var item = { id: 'insert-' + s.name, label: s.title, words: s.name + ' ' + s.title.toLowerCase(), markdown: s.example };
                            /* The host's own shortcode is asked for with the host's form. */
                            if (hostEntries) { item.run = function () { return insertShortcode(s.name); }; }
                            extra.push(item);
                        });
                    }
                    return window.yavcMdEditor ? window.yavcMdEditor.inserts(extra, picturesHost) : [];
                },
                /* A picture pressed on the page opens the picture dialog. */
                editBlock: function (src) { return window.yavcMdEditor.editBlock(src, picturesHost()); },
                /* Shortcodes the host application draws: a drawing put into
                   its placeholder, a span's drawing for the page to put the
                   words in, a shortcode's title, and a host's inline
                   shortcode that has just been typed. */
                fill: function (target, cachedOnly) { window.yavcMdEditor.fillHosts(target, picturesHost(), cachedOnly); },
                hostSpan: function (name, open) { return window.yavcMdEditor.hostSpan(name, open, picturesHost()); },
                editHostSpan: function (name, open) { return window.yavcMdEditor.editHostSpan(name, open, picturesHost()); },
                hostTitle: function (name) {
                    var s = (hostEntries || []).find(function (x) { return x.name === name; });
                    return s && typeof s.title === 'string' && s.title.trim() && s.title.length <= 256 ? s.title : '';
                },
                /* The host's inline shortcode that records a cell's colour in
                   a pipe table, the first the catalogue marks so, or null. */
                cellColourShortcode: function () {
                    var s = (hostEntries || []).find(function (x) { return x.cellColor === true && x.kind === 'inline'; });
                    return s ? { name: s.name, example: typeof s.example === 'string' ? s.example : '' } : null;
                },
                /* The attribute a host keeps equal to a cell's colour in the
                   tables of one of its literal containers, or ''. */
                colourAttribute: function (name) {
                    var s = (hostEntries || []).find(function (x) { return x.name === name; });
                    return s && s.literal && typeof s.colorAttribute === 'string' ? s.colorAttribute : '';
                },
                /* The beginning of the class names that host uses for a
                   cell's colour, which go when the colour changes, or ''. */
                colourClassPrefix: function (name) {
                    var s = (hostEntries || []).find(function (x) { return x.name === name; });
                    return s && s.literal && typeof s.colorClassPrefix === 'string' ? s.colorClassPrefix : '';
                },
                hostInline: function (src) {
                    if (!hostEntries) { return Promise.resolve(null); }
                    return Promise.resolve(currentMacroEngine() || documentEngine()).then(function (m) {
                        var out = m.inline(src);
                        if (!out || !window.yavcMdEditor.hostInlineOnly(out.html)) { return null; }
                        return withHostDrawings(out);
                    }).catch(function () { return null; });
                },
                review:function(raw){reviewTools().then(function(controller){return controller.browse(raw);}).catch(function(err){say(err.message,true);});},
                hostSuggestionWidget:hostSuggestions?function(item){return hostSuggestions.widget(item,true);}:null,
                root: root, page: el.page, scroll: el.scroll,
                pagination: function () { return pagedView(viewMode) ? pageSettings : null; },
                paginated: pageLayout,
                paginationProblem: pageNotice,
                customName: function (name) { return customShortcodes().some(function (item) { return item.name === name; }); },
                text: function () { return view.state.doc.toString(); },
                /* The text's caret, for the page to open at the block it
                   is in (decision 113). */
                caret: function () { return view ? view.state.selection.main.head : null; },
                setText: takeVisualText,
                docOf: function (text) { return documentEngine(text).then(function (m) { return withHostDrawings(m.doc(text)); }); },
                markdownOf: function (doc) { return mdEngine().then(function (m) { return m.markdown(doc); }); },
                sourceOf: function (src) { return documentEngine().then(function (m) { return withHostDrawings(m.source(src)); }); },
                wrapOf: function (line) { return documentEngine().then(function (m) { return withHostDrawings(m.container(line)); }); },
                inlineOf: function (src) { return documentEngine().then(function (m) { return withHostDrawings(m.inline(src)); }); },
                blocksOf: function (text) { return documentEngine().then(function (m) { return withHostDrawings(m.blocks(text)); }); },
                draw: function (target, html, diagrams) { drawBlocks(target, [{ line: 0, html: html, diagrams: diagrams }]); },
                keys: pageKeys,
                say: say,
                changed: function () { commandsChanged(); markOutline(); if (visualFind && !el.findPlace.hidden) { visualFind.refresh(); } },
                failed: function (text) { setView(viewMode === 'document' ? 'visual' : 'text').then(function () { say(text, true); }); }
            };
        }
        /* The Editor's keys on the page, in ProseMirror's spelling, which is
           CodeMirror's: the formatting, the styles, Save and Find. */
        function pageKeys() {
            var keys = [];
            [['Mod-z', 'undo'], ['Mod-y', 'redo'], ['Mod-Shift-z', 'redo']].forEach(function (item) {
                keys.push({ key: item[0], run: function () { editing().history(item[1]); return true; } });
            });
            FORMATS.forEach(function (f) { if (f[3]) { keys.push({ key: f[3], run: function () { applyFormat(f[0]); return true; } }); } });
            STYLES.forEach(function (st) { keys.push({ key: st[2], run: function () { applyFormat(st[0]); return true; } }); });
            keys.push({ key: 'Mod-f', run: function () { openFind(); return true; } });
            var bindings=effectiveKeys();
            Object.keys(bindings).forEach(function (key) {
                var name=bindings[key];if(!validKey(key) || name!==null&&!COMMANDS[name])return;
                keys.push({key:key,run:function(){if(name!==null){try{Promise.resolve(COMMANDS[name]()).catch(function(err){say(err.message,true);});}catch(err){say(err.message,true);}}return true;}});
            });
            return keys;
        }
        function textChangedForPage() {
            if (pageActive()) { visual.api.textChanged(); }
            if (readsAsPreview()) { schedulePreview(); }
        }

        /* Shows the document as asked. Visual and Split need a Markdown
           file that can be changed; Preview any Markdown file. */
        async function setView(mode) {
            var t = current();
            mode = resolvedView(t, mode);
            viewMode = mode;
            if (t) t.view = mode;
            commandsChanged();
            root.classList.toggle('ed-previewing', mode !== 'text');
            root.classList.toggle('ed-visual-only', mode === 'visual' || mode === 'preview' || mode === 'document');
            root.classList.toggle('ed-document', pagedView(mode));
            if (previewPages) { previewPages.destroy(); previewPages = null; }
            if (!pagedView(mode)) { clearPages(); }
            root.classList.toggle('ed-reading', readsAsPreview());
            showFormat(mode);
            el.pageBox.hidden = mode === 'text';
            var gen = ++visual.gen;
            refreshStyles();
            clearTimeout(visual.timer);
            /* The reader's place: leaving the page, its caret's place in
               the text becomes the text's caret, so Text, Split and
               Preview open there (decision 113). The page writes itself
               to the text first. */
            var place = null;
            if (visual.api && visual.api.active() && mode !== 'visual' && mode !== 'document') {
                place = await visual.api.placeInText().catch(function () { return null; });
                if (gen !== visual.gen || destroyed) { return; }
            }
            if (mode === 'text') {
                closePageFind(false);
                if (visual.api) { await visual.api.enable(false); }
                if (gen !== visual.gen || destroyed) { return; }
                el.page.replaceChildren();
                say('');
                showPlace(place, t);
                announce();
                keep();
                return;
            }
            try {
                await mdEngine();
                if (pagedView(mode)) { await configurePages(); }
                if (gen !== visual.gen) { return; }
                if (readsAsPreview()) {
                    if (visual.api) { await visual.api.enable(false); }
                    if (gen !== visual.gen) { return; }
                    await drawPreview();
                    if (gen !== visual.gen) { return; }
                    showPlace(place, t);
                } else {
                    if (!visual.api) {
                        await pageParts();
                        if (gen !== visual.gen) { return; }
                        visual.api = window.yavcPageView.attach(pageHost());
                        if (window.yavcMdEditor) { window.yavcMdEditor._page = visual.api; }
                    }
                    var wasOn = visual.api.active();
                    await visual.api.enable(true);
                    if (gen !== visual.gen) { return; }
                    /* Split keeps the page; the text beside it opens at the page's place. */
                    showPlace(place, t);
                    if (!el.findPlace.hidden && lastFind) { pageFind.set(lastFind); visualFind.refresh(); }
                    say('');
                    visual.api.repaginate();
                    /* A page just turned on opened at the text's caret; now that it is shown, bring that into view. */
                    if (!wasOn) { visual.api.scrollToSelection(); }
                    if ((mode === 'visual' || mode === 'document') && inFront()) { visual.api.focus(); }
                }
            } catch (err) {
                if (gen !== visual.gen || destroyed) { return; }
                await setView(mode === 'document' ? 'visual' : 'text');
                say('The page could not be made. Using ' + viewMode + ' view: ' + (err && err.message || err), 'warn');
                return;
            }
            announce(); keep();
        }
        function detachVisual() {
            clearTimeout(visual.timer);
            if (visual.api) { try { visual.api.enable(false); } catch (err) { } }
        }

        /* === Preview ===================================================== */
        var documentStyles = null, styledIdentity = null, styledMode = '', styledTypography = '', currentSharedStyle = '', styleRevision = 0, styleTimer = 0;
        function styleDirectory() { var t = current(); return t && t.path && F ? F.splitPath(t.path).dir : hostServices.directory && t ? hostServices.directory(t.path) : null; }
        function scheduleStyles() { clearTimeout(styleTimer); styleTimer = setTimeout(refreshStyles, 150); }
        async function refreshStyles(reload) {
            var revision = ++styleRevision, t = current(), text = t && view ? view.state.doc.toString() : '';
            if (styledIdentity !== t || styledMode !== viewMode) { if (documentStyles) { documentStyles.destroy(); documentStyles = null; } styledIdentity = t; styledMode = viewMode; currentSharedStyle = ''; }
            if (!t || t.lang !== 'markdown') { if (documentStyles) { documentStyles.destroy(); documentStyles = null; } return; }
            try {
                var K = await loadKit(), m = await mdEngine(), D = await K.styles(), P = await K.pages();
                if (destroyed || revision !== styleRevision || t !== current()) { return; }
                var typeCSS = window.yavcDocumentSettings.typeCSS(window.yavcDocumentSettings.properties(m.front(text)));
                if (documentStyles && styledTypography !== typeCSS) { documentStyles.destroy(); documentStyles = null; }
                styledTypography = typeCSS;
                if (!documentStyles) { documentStyles = D.attach({ surfaces: function () { return [el.page]; }, boundaries: function () { return [el.scroll]; },
                    base: P.paper + ';' + typeCSS,
                    read: function (path) { return window.pudlHost.fs.readText(path); }, onProblems: function (problems) {
                        var notice = root.querySelector('[data-role="style-problems"]'); notice.hidden = !problems.length; notice.querySelector('.notice-body').textContent = problems.join(' ');
                    } }); }
                var front = m.front(text), path = ''; (front.entries || []).forEach(function (entry) { if (entry.key === 'stylesheet') { path = entry.value; } });
                currentSharedStyle = window.yavcStyleTools.resolve(path, styleDirectory());
                await documentStyles.update({ identity: t, inline: m.styles(text).map(function (block) { return block.css; }), stylesheet: currentSharedStyle, reload: reload === true });
                var oldShared = sharedMacros; await documentEngine();
                if (!destroyed && revision === styleRevision && oldShared !== sharedMacros) {
                    if (pageActive()) { visual.api.textChanged(true); } else if (readsAsPreview()) { schedulePreview(); }
                }
                if (!destroyed && revision === styleRevision) { if (pagedView(viewMode)) { await configurePages(); if (visual.api) { visual.api.repaginate(); } if (previewPages) { previewPages.refresh(); } } commandsChanged(); }
            } catch (err) { if (!destroyed && revision === styleRevision) { say('Styles could not be applied: ' + err.message, true); } }
        }
        async function editStyles(shared) {
            var t = current(); if (!t || t.readOnly) { return; }
            if (pageActive() && visual.api.sync) { await visual.api.sync(); }
            var text = view.state.doc.toString(), m = await mdEngine(), K = await loadKit(); await K.styles();
            function unchanged() { return !destroyed && current() === t && view.state.doc.toString() === text; }
            if (!unchanged()) { return; }
            try {
                if (shared) {
                    var path = ''; (m.front(text).entries || []).forEach(function (entry) { if (entry.key === 'stylesheet') { path = entry.value; } });
                    await window.yavcStyleTools.shared({ path: path, dir: styleDirectory(), current: unchanged, signal: styleLifetime.signal, read: window.pudlHost.fs.readText, write: window.pudlHost.fs.writeText });
                } else {
                    await window.yavcStyleTools.document({ text: text, engine: m, current: unchanged, signal: styleLifetime.signal, save: function (next) {
                        takeVisualText(next); textChangedForPage(); refreshStyles();
                    } });
                }
                refreshStyles(true);
            } catch (err) { say(err.message, true); }
        }
        function schedulePreview() {
            clearTimeout(visual.timer);
            visual.timer = setTimeout(function () { drawPreview().catch(function (err) { say('The preview could not be drawn: ' + (err && err.message || err), true); }); }, 250);
        }
        async function drawPreview() {
            var gen = visual.gen, t = current(), source = view.state.doc.toString(), m = await documentEngine(source);
            if (gen !== visual.gen || !view || !readsAsPreview() || current() !== t || view.state.doc.toString() !== source) { return; }
            var p = m.preview(source);
            if (hostEntries) {
                /* The host's drawings are kept before the preview is drawn,
                   so each is in place when the preview first shows. */
                await withHostDrawings(p);
                if (gen !== visual.gen || !view || !readsAsPreview() || current() !== t || view.state.doc.toString() !== source) { return; }
            }
            var top = el.scroll.scrollTop;
            if (pagedView(viewMode)) {
                var flow = el.page.querySelector(':scope > .doc-page-flow');
                if (!flow) { el.page.replaceChildren(); flow = document.createElement('div'); flow.className = 'doc-page-flow'; el.page.append(flow); }
                drawBlocks(flow, p.blocks);
                if (!previewPages) { previewPages = window.yavcDocumentPages.preview({ host: el.page, flow: flow, settings: function () { return pageSettings; }, changed: pageLayout, problem: pageNotice }); }
                previewPages.refresh();
            } else { drawBlocks(el.page, p.blocks); }
            el.scroll.scrollTop = top;
            var problems = p.problems || [];
            if (problems.length) {
                say((problems.length === 1 ? 'One problem' : problems.length + ' problems') + (problems[0].line ? ', line ' + problems[0].line : '') + ': ' + problems[0].message, false);
            } else { say(''); }
        }
        /* Draws preview blocks into an element with the kit's renderer
           (apps/md-editor/kit.js drawBlocks): made safe, diagrams, pictures,
           listings and applets drawn, and blocks that did not change kept. */
        function drawBlocks(target, blocks) { window.yavcMdEditor.drawBlocks(target, blocks, { pictures: picturesHost(), applets:false }); wireReview(target); }
        var INSERTS = window.yavcMdEditor ? window.yavcMdEditor.INSERTS : [], SHORTCODES = window.yavcMdEditor ? window.yavcMdEditor.SHORTCODES : [];
        /* Where a format or an insert goes: the page in Visual, the text
           in Text and Preview, and in Split whichever has the keyboard. */
        function editCtx() {
            return {
                identity: current, signal: styleLifetime.signal,
                prepareFormats: async function () { clearTimeout(styleTimer); await refreshStyles(); },
                styleScope: function () { return el.page.getAttribute('data-md-style') || ''; },
                page: function () { return visual.api; }, text: function () { return view; },
                onPage: function () { return formatsThePage(); },
                readOnly: function () { var t = current(); return !t || t.lang !== 'markdown' || !!t.readOnly; },
                blockStyle: blockStyle, classes: function () { return documentStyles ? documentStyles.state().classes : []; }, say: say, pictures: picturesHost
            };
        }
        function insertBlock(item) { window.yavcMdEditor.insertItem(item, editCtx()); }
        /* File, Properties: the front matter read by the engine, changed in
           the kit's dialog, and written back by the engine, which changes
           only the lines of the properties that changed and reads the
           result back before it is used. The text takes it as one change,
           which Undo takes back. */
        async function editProperties() {
            var t = current();
            if (!t || t.readOnly || !view) { return; }
            try {
                if (pageActive() && visual.api.sync) { await visual.api.sync(); }
                if (destroyed || current() !== t) return;
                var m = await mdEngine(), text = view.state.doc.toString();
                if (destroyed || current() !== t) return;
                var front = m.front(text);
                function unchanged() { return !destroyed && current() === t && !t.readOnly && view.state.doc.toString() === text; }
                var r = await window.yavcMdEditor.askProperties(front, { title: 'Properties of ' + (t.name || 'the Document'), current: unchanged, signal: styleLifetime.signal });
                if (!r || !r.order.length) { return; }
                if (!unchanged()) { say('The document changed while the dialog was open, so the properties were not written. Try again.', true); return; }
                var next = m.frontset({ text: text, set: r.set, order: r.order }).text;
                var a = 0;
                while (a < text.length && a < next.length && text[text.length - 1 - a] === next[next.length - 1 - a]) { a++; }
                view.dispatch({ changes: { from: 0, to: text.length - a, insert: next.slice(0, next.length - a) }, userEvent: 'input' });
                say('The properties were changed.');
            } catch (err) { say(String(err && err.message || err), true); }
        }
        /* Where the document's pictures are, for the kit's picture dialog
           and its loader: beside the document when it is saved under ~. */
        /* === A diagram a document refers to, opened for editing ==========
           A document shows a saved diagram by its file ({{diagram:path}},
           decision 87). Edit Diagram opens that file in the application
           that edits it, through PUDL's request for the diagram verb, and the document
           draws it again when the file is saved. */
        function canOpenDiagram() { return typeof hostServices.openDiagram === 'function' || !!(window.pudlApplets && window.pudlApplets.can && window.pudlApplets.request && window.pudlApplets.can('diagram')); }
        /* report(text) is told why a diagram could not be opened; without one
           the reason is said as an error in the workspace. */
        function openDiagram(reference, report) {
            report = report || function (text) { say(text, true); };
            reference = String(reference || '');
            if (!/\.(d2|mmd|mermaid|canvas)$/i.test(reference) || /[:\\?#\u0000-\u001f\u007f]/.test(reference) || reference.startsWith('//') || reference.startsWith('~') && !reference.startsWith('~/')) { report('The diagram is not a local D2, Mermaid or Canvas file, so it cannot be opened.'); return false; }
            var directory = styleDirectory();
            if (!/^(~\/|\/)/.test(reference) && !directory) { report('Save this document first, so the diagram beside it can be found.'); return false; }
            var path = window.yavcStyleTools ? window.yavcStyleTools.resolve(reference, directory) : reference;
            if (!/^(~\/|\/(?!\/))/.test(path)) { report('The diagram\'s place cannot be worked out here.'); return false; }
            if (!canOpenDiagram()) { report('Nothing here opens a diagram file for editing.'); return false; }
            /* A host that opens files itself is asked first (contentWorkspace.openDiagram,
               docs/content-workspace-contract.md); it says whether it took the request. */
            if (typeof hostServices.openDiagram === 'function') {
                var took = false;
                try { took = hostServices.openDiagram(path, { host: root, document: current() && current().path || null }) === true; } catch (err) { report(String(err && err.message || err)); return false; }
                if (took) { return true; }
                if (!(window.pudlApplets && window.pudlApplets.can && window.pudlApplets.request && window.pudlApplets.can('diagram'))) { report('This application could not open the diagram.'); return false; }
            }
            window.pudlApplets.request('diagram', { path: path }, root);
            return true;
        }
        /* The diagram the cursor is on: the line it is written on in the
           text, or the diagram selected on the page. '' when there is none. */
        function diagramAtCursor() {
            if (!view || !current() || current().lang !== 'markdown') { return ''; }
            if (pageActive() && (viewMode !== 'split' || editingPane !== 'text')) {
                var page = visual.api.element && visual.api.element(), chosen = page && page.querySelector('.ProseMirror-selectednode');
                var figure = chosen && (chosen.matches('[data-diagram-src]') ? chosen : chosen.querySelector('[data-diagram-src]'));
                return figure ? figure.getAttribute('data-diagram-src') || '' : '';
            }
            var line = view.state.doc.lineAt(view.state.selection.main.head).text, call = /\{\{diagram:\s*(?:src=)?(?:"([^"}]*)"|'([^'}]*)'|([^\s}]+))/.exec(line);
            return call ? call[1] || call[2] || call[3] || '' : '';
        }
        function picturesHost() {
            var owner=current(),resource=owner&&owner.path;
            return {
                resources:hostServices.resources,resource:resource,owner:owner,
                current:function(){return !destroyed&&!!owner&&current()===owner&&owner.path===resource;},
                site: function () { return Promise.resolve({ F: F, FB: FB }); },
                root: root, say: say,
                macroContext: currentMacroEngine,
                openDiagram: canOpenDiagram() ? openDiagram : null,
                shortcodeHost: hostServices.shortcodeHost, hostFills: hostFills, hostCatalog: hostEntries,
                read: function (path, options) { if(!H.fs || !H.fs.readText)throw new Error('This application has not given documents a way to read other files (fs.readText).');return H.fs.readText(path, Object.assign({}, options, { document: resource || null })); },
                watch: H.fs && H.fs.onChange ? function(fn){return H.fs.onChange(fn);} : null,
                signal: styleLifetime.signal, identity: current, text: function () { return view ? view.state.doc.toString() : ''; },
                dir: styleDirectory
            };
        }

        /* === Find and replace ===========================================
           The find bar is the editing kit's (apps/md-editor/kit.js findBar
           and codeSearch), shared with Diagrams: in the text it is
           CodeMirror's search panel, and on the page it drives the visual
           editor's own search. The query carries over when the view
           changes. */
        var lastFind = null, visualFind = null;
        function keepQuery(q) { lastFind = q; }
        function findBar(driver) { return window.yavcMdEditor.findBar(driver, keepQuery); }

        /* The page's search: the page view marks the matches of a matcher
           built from the find bar's query, and the bar moves among them. */
        var pageMatches = { total: 0, at: 0 }, pageQuery = null;
        function matcherOf(q) {
            if (!q || !q.search) { return null; }
            var src = q.regexp ? q.search : q.search.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');
            if (q.wholeWord) { src = '(?<![\\p{L}\\p{N}_])(?:' + src + ')(?![\\p{L}\\p{N}_])'; }
            var re;
            try { re = new RegExp(src, 'gu' + (q.caseSensitive ? '' : 'i')); } catch (err) { return null; }
            return function (text) {
                var out = [], m;
                re.lastIndex = 0;
                while ((m = re.exec(text))) { if (!m[0]) { re.lastIndex++; continue; } out.push({ from: m.index, to: m.index + m[0].length }); }
                return out;
            };
        }
        function replacementFor(q, matched) {
            var rep = q && q.replace != null ? q.replace : '';
            if (!q || !q.regexp) { return rep; }
            try { return matched.replace(new RegExp(q.search, q.caseSensitive ? 'u' : 'iu'), rep); } catch (err) { return rep; }
        }
        function pageGo(i) { visual.api.goToMatch(i); pageMatches.at = i + 1; }
        var pageFind = {
            set: function (q) {
                pageQuery = q;
                if (!pageActive()) { pageMatches = { total: 0, at: 0 }; return; }
                var matcher = matcherOf(q), r = visual.api.find(matcher);
                pageMatches = { total: r.count, at: 0, error: !!(q && q.search && !matcher) };
                if (r.count) { pageGo(r.after >= 0 ? r.after : 0); }
            },
            next: function () { if (pageMatches.total && pageActive()) { pageGo(pageMatches.at % pageMatches.total); } },
            prev: function () { if (pageMatches.total && pageActive()) { pageGo((pageMatches.at - 2 + pageMatches.total) % pageMatches.total); } },
            replace: function () {
                if (!pageMatches.total || !pageActive()) { return; }
                var i = Math.max(0, pageMatches.at - 1);
                visual.api.replaceMatch(i, replacementFor(pageQuery, visual.api.matchText(i)));
                var r = visual.api.find(matcherOf(pageQuery));
                pageMatches = { total: r.count, at: 0 };
                if (r.count) { pageGo(Math.min(i, r.count - 1)); }
            },
            replaceAll: function () {
                if (!pageMatches.total || !pageActive()) { return; }
                visual.api.replaceAllMatches(function (text) { return replacementFor(pageQuery, text); });
                visual.api.find(matcherOf(pageQuery));
                pageMatches = { total: 0, at: 0 };
            },
            close: function () { closePageFind(true); },
            readOnly: function () { var t = current(); return !t || !!t.readOnly; },
            count: function () { return pageMatches; }
        };
        function openPageFind() {
            if (!visualFind) { visualFind = findBar(pageFind); el.findPlace.appendChild(visualFind.dom); }
            el.findPlace.hidden = false;
            var q = lastFind || (view ? CM.getSearchQuery(view.state) : null);
            if (q && q.search) { visualFind.show(q); pageFind.set(q); }
            visualFind.refresh();
            visualFind.focus();
        }
        function closePageFind(back) {
            el.findPlace.hidden = true;
            if (pageActive()) { visual.api.find(null); }
            pageMatches = { total: 0, at: 0 };
            if (back && pageActive()) { visual.api.focus(); }
        }
        /* Find in whichever the reader is working in: the page in Visual, or
           in Split while the text does not have the keyboard; else the text. */
        function openFind() {
            if (formatsThePage() && pageActive()) { openPageFind(); return; }
            if (!view) { return; }
            if (lastFind && !CM.getSearchQuery(view.state).search) {
                view.dispatch({ effects: CM.setSearchQuery.of(new CM.SearchQuery(lastFind)) });
            }
            CM.openSearchPanel(view);
        }

        /* The page's edit, as text: the least change that turns the text into
           the new text, so the cursor and the undo history in Split keep
           their sense. */
        function takeVisualText(text, isolate) {
            var old = view.state.doc.toString();
            if (old === text) { return; }
            var a = 0, b = 0;
            while (a < old.length && a < text.length && old[a] === text[a]) { a++; }
            while (b < old.length - a && b < text.length - a && old[old.length - 1 - b] === text[text.length - 1 - b]) { b++; }
            visual.fromVisual = true;
            try { view.dispatch({ changes: { from: a, to: old.length - b, insert: text.slice(a, text.length - b) }, userEvent: 'input.visual',annotations:isolate?CM.isolateHistory.of('full'):[] }); }
            finally { visual.fromVisual = false; }
        }

        /* The formatting is offered for a Markdown file in any view, and
           in Visual and Split whatever the file. */
        function showFormat(mode) {
            var t = current();
            var on = !!(mode === 'visual' || mode === 'split' || (t && t.lang === 'markdown' && !t.readOnly));
            if (on !== formatOffered) { formatOffered = on; commandsChanged(); if (window.pudlMenubar) { window.pudlMenubar.refresh(); } }
        }
        /* The formatting goes to the page in Visual, to the text in Text and
           Preview, and in Split to whichever has the keyboard, the text when
           it does and the page otherwise. A toolbar button keeps the
           keyboard where it was, so Split can still tell. */
        var editingCommands = null, editingPane = 'visual';
        root.addEventListener('focusin', function (e) {
            if (e.target.closest('.cm-editor')) editingPane = 'text';
            else if (e.target.closest('.ProseMirror')) editingPane = 'visual';
            else if (e.target.closest('.ed-page')) editingPane = 'preview';
            commandsChanged();
        });
        el.pageBox.addEventListener('pointerdown',function () {if(readsAsPreview())editingPane='preview';});
        document.addEventListener('selectionchange', function () { if (readsAsPreview()) commandsChanged(); }, { signal: styleLifetime.signal });
        function formatsThePage() { return pageActive() && (viewMode === 'visual' || viewMode === 'document' || (viewMode === 'split' && editingPane !== 'text')); }
        function editing() {
            if (!editingCommands && sourceHistory && window.yavcMdEditor) editingCommands = window.yavcMdEditor.editing({
                source: function () { return view; }, sourceHistory:sourceHistory,page: function () { return visual.api; }, rich: formatsThePage,
                preview: function () { return readsAsPreview() && (viewMode!=='split' || editingPane!=='text') ? el.page : null; }, unified: true,
                identity: current, alive: function () { return !destroyed && !!view && !!current(); },
                historyReadOnly: function () { var t = current(); return !t || !!t.readOnly; },
                readOnly: function () { var t = current(); return !t || !!t.readOnly || readsAsPreview() && (viewMode!=='split' || editingPane!=='text'); }, say: say, changed: commandsChanged
            });
            return editingCommands;
        }
        function applyFormat(name) { window.yavcMdEditor.apply(name, editCtx()); }

        /* The engine supplies built-in completion metadata. Optional host
           additions do not replace the authored insertion forms. */
        var shortcodes = null, shortcodesLoading = null;
        function shortcodeCatalog(builtin, extra) {
            var found=new Map(),units=0;
            [builtin,extra].forEach(function(list,origin){if(!Array.isArray(list))return;list.slice(0,256).forEach(function(s){
                if(found.size>=256||!s||typeof s.name!=='string'||!/^[a-z][a-z0-9-]{0,63}$/.test(s.name)||found.has(s.name)||!['inline','block','container','span'].includes(s.kind)||typeof s.title!=='string'||!s.title.trim()||s.title.length>256||typeof s.description!=='string'||s.description.length>4096)return;
                var params=Array.isArray(s.params)?s.params.slice(0,32).filter(function(p){return p&&typeof p.name==='string'&&/^[a-z][a-z0-9-]{0,63}$/.test(p.name)&&typeof p.description==='string'&&p.description.length<=2048;}).map(function(p){return {name:p.name,description:p.description,required:p.required===true};}):[];
                if(typeof s.example==='string'&&s.example.length>4096)return;
                var example=typeof s.example==='string'?s.example:'',size=s.name.length+s.title.length+s.description.length+example.length+params.reduce(function(n,p){return n+p.name.length+p.description.length;},0);if(units+size>128*1024)return;units+=size;
                found.set(s.name,{name:s.name,title:s.title,description:s.description,kind:s.kind,params:params,example:example,argumentRequired:s.argumentRequired===true,builtin:origin===0,managed:origin===1&&s.managed===true,literal:origin===1&&s.kind==='container'&&s.literal===true,colorAttribute:origin===1&&s.kind==='container'&&s.literal===true&&typeof s.colorAttribute==='string'&&/^[a-z][a-z0-9:_-]{0,63}$/.test(s.colorAttribute)?s.colorAttribute:'',colorClassPrefix:origin===1&&s.kind==='container'&&s.literal===true&&typeof s.colorClassPrefix==='string'&&/^[A-Za-z][A-Za-z0-9_-]{0,63}$/.test(s.colorClassPrefix)?s.colorClassPrefix:'',cellColor:origin===1&&s.kind==='inline'&&s.cellColor===true,prefix:origin===1&&s.kind==='inline'&&typeof s.prefix==='string'&&/^[@#~!$%&+=?^]$/.test(s.prefix)?s.prefix:''});
            });});return Array.from(found.values());
        }
        function loadShortcodes() {
            if (shortcodes) { return Promise.resolve(shortcodes); }
            if (!shortcodesLoading) {
                shortcodesLoading = mdEngine().then(function(m){
                    if(destroyed)return [];
                    var builtin=m.shortcodes().shortcodes;shortcodes=shortcodeCatalog(builtin,[]);
                    if(hostServices.shortcodes)hostShortcodes().then(function(list){if(!destroyed){shortcodes=shortcodeCatalog(builtin,list);commandsChanged();}}).catch(function(){});
                    return shortcodes;
                }).catch(function(){shortcodesLoading=null;return [];});
            }
            return shortcodesLoading;
        }
        /* The Insert menu's items, one command a shortcode, in order of
           their titles; until the table has loaded, one saying so. */
        var insertCache=null;
        function insertItems() {
            var custom=customShortcodes(),names=JSON.stringify(custom.map(function(s){return s.name;})),mounted=!!(F&&F.mounted);
            if(insertCache&&insertCache.inserts===INSERTS&&insertCache.authored===SHORTCODES&&insertCache.catalog===shortcodes&&insertCache.names===names&&insertCache.mounted===mounted)return insertCache.items;
            var items = INSERTS.map(function (i) { return { id: i.id, label: i.label, run: function () { insertBlock(i); } }; });
            items.push('-');
            SHORTCODES.forEach(function (i) { items.push({ id: i.id, label: i.label, run: function () { insertBlock(i); } }); });
            if (custom.length) { items.push('-'); custom.forEach(function (s) { items.push({ id: 'insert-custom-' + s.name, label: s.name, run: function () { insertCustom(s.name); } }); }); }
            if (shortcodes && (F && F.mounted || hostEntries)) {
                var extra=shortcodes.filter(function (s) { return !s.builtin && !s.managed && s.name !== 'column'; }).sort(function (a, b) {
                    return a.title.localeCompare(b.title, undefined, { sensitivity: 'base' });
                });if(extra.length)items.push('-');extra.forEach(function (s) {
                    items.push({ id: 'insert-' + s.name, label: s.title, run: function () { insertShortcode(s.name); } });
                });
            }
            insertCache={inserts:INSERTS,authored:SHORTCODES,catalog:shortcodes,names:names,mounted:mounted,items:items};
            return items;
        }
        async function insertCustom(name) {
            var t = current(), source = view.state.doc.toString(), m = await documentEngine();
            if (destroyed || styleLifetime.signal.aborted || current() !== t || view.state.doc.toString() !== source) { return; }
            var src = await window.yavcMacroTools.ask(m, name, null, picturesHost()), metadata = window.yavcMacroTools.metadata(m).find(function (item) { return item.name === name; });
            if (!src || current() !== t || view.state.doc.toString() !== source) { return; }
            if (formatsThePage()) { visual.api.insertShortcode(metadata.kind, src); }
            else { insertText(metadata.kind === 'inline' ? src : '\n' + src + '\n'); }
        }
        async function insertShortcode(name) {
            var s = (shortcodes || []).find(function (x) { return x.name === name; });
            if (!s) { return; }
            var owner=current(),onPage=formatsThePage(),api=visual.api;
            if(onPage && !await api.prepareEdit()) return;
            if(destroyed||current()!==owner||onPage!==formatsThePage())return;
            var doc=view.state.doc,selection=view.state.selection,stable=onPage?api.insertionTarget():function(){return view.state.doc===doc&&view.state.selection.eq(selection);};
            /* A figure is asked for as a form; the rest go in as their
               examples, into the page or into the text. */
            var asked = s.name === 'figure' ? window.yavcMdEditor.askFigure(null, picturesHost())
                : !s.builtin && hostEntries && window.yavcMdEditor && window.yavcMdEditor.askHostInsert ? window.yavcMdEditor.askHostInsert(s.example, picturesHost())
                : Promise.resolve(s.example);
            var src=await asked;
            if (!src || destroyed || current()!==owner || onPage!==formatsThePage() || !stable()) { return; }
            if (onPage) { return api.insertShortcode(s.name === 'figure' ? 'block' : s.kind, src); }
            return insertText(s.kind === 'inline' || s.kind === 'span' ? src : '\n' + src + '\n');
        }

        /* === Changes made elsewhere ======================================= */

        /* A file saved by the terminal, Files or another tab: an open tab
           with no unsaved changes takes the new text; one with changes is
           told, and keeps the reader's version. */
        function onFsChange() {
            // Resource tabs retain their captured revision until an acknowledged save or explicit reopen.
            if (resources) return;
            refreshStyles(true);
            tabs.forEach(function (t, i) {
                if (!t.path || t.readOnly) { return; }
                var node = F.resolve(F.home(), t.path);
                if (!node && t.fresh) { return; }
                if (!node) { t.external = true; if (i === active) { say(t.path + ' was deleted or moved somewhere else. Save to keep this text.', true); } return; }
                var r = F.realOf(node);
                /* A server file changed elsewhere shows a new time; only then
                   is its text fetched. */
                if (r.server) {
                    if (t.mtime != null && +r.mtime === t.mtime) { return; }
                    F.read(r).then(function (text) { t.mtime = +r.mtime; takeOutside(t, tabs.indexOf(t), text); renderTabs(); }).catch(function () { });
                    return;
                }
                var revision=r.revision;
                F.read(r).then(function(text){var current=F.resolve(F.home(),t.path);if(tabs.indexOf(t)<0||!current||current.revision!==revision)return;takeOutside(t,tabs.indexOf(t),text);renderTabs();}).catch(function(){});
            });
            renderTabs();
        }

        /* A change made somewhere else, as the text between what the two
           versions share at their start and at their end, so the cursor
           and the place in the file stay where they were. It isn't the
           reader's edit, so undo doesn't take it back. */
        function outsideChange(state, text) {
            var raw=text;text=normalizedSource(text);
            var old = state.doc.toString(), start = 0, end = 0;
            var max = Math.min(old.length, text.length);
            while (start < max && old.charCodeAt(start) === text.charCodeAt(start)) { start++; }
            while (end < max - start && old.charCodeAt(old.length - 1 - end) === text.charCodeAt(text.length - 1 - end)) { end++; }
            return {
                changes: { from: start, to: old.length - end, insert: text.slice(start, text.length - end) },
                effects:sourceHistory.adopt(raw),
                annotations: CM.Transaction.addToHistory.of(false)
            };
        }

        function takeOutside(t, i, text) {
            if (i < 0 || text === t.savedRaw) { return; }
            if (t.submittedWrite && t.path === t.submittedWrite.path && text === t.submittedWrite.text) { return; }
            var edited = sourceModified(t);
            if (!edited) {
                t.savedRaw = text; t.saved = normalizedSource(text);
                if (i === active) { view.dispatch(outsideChange(view.state, text)); t.modified = false; }
                else { t.state = t.state.update(outsideChange(t.state, text)).state; }
            } else {
                t.external = true;
                if (i === active) { say(t.path + ' was changed somewhere else. Saving will replace that version with yours.', true); }
            }
        }

        /* A file handed to a running editor, by an open request
           (pudlApplets.request) or a preset, opens in a tab, or as a new
           file by that name. */
        var stateRequest = 0;
        function takeFile(s) {
            var f = fileFrom(s);
            if (!f || !CM) { return; }
            var ticket=++stateRequest,params=new URLSearchParams(s || ''),existed=tabs.some(function (t) {return t.path===f;});function currentRequest(){return !destroyed && ticket===stateRequest;}
            return openPath(f,false,currentRequest,params.get('view')).then(async function (opened) {if(!currentRequest())return;if (!opened) {if(F && !F.resolve(F.home(), f))await newNamed(f);else return;} var requested=params.get('view') || (!existed && initialMode(current(),params.get('initial-view')));if(requested){if(!supportsView(current(),requested))throw new Error('This view is unavailable for this file. Choose a supported view from the toolbar.');await setView(requested);}if(currentRequest())await restoreLine(Number(new URLSearchParams(s || '').get('line')) || 1); }).catch(function (err) {if(currentRequest())say(err.message,true);});
        }
        async function restoreLine(number) {
            var tab = current(); if (!view || !tab) { return; }
            var text = view.state.doc.toString(), m = await mdEngine();
            if (destroyed || current() !== tab || view.state.doc.toString() !== text) { return; }
            var line = view.state.doc.line(Math.max(1, Math.min(view.state.doc.lines, Math.floor(number)))), entries = tab.lang === 'markdown' ? m.outline(text) : [];
            view.dispatch({ selection: { anchor: line.from }, effects: CM.EditorView.scrollIntoView(line.from, { y: 'center' }) });
            var index = entries.findIndex(function (heading) { return heading.line === line.number; });
            if (index >= 0 && pageActive()) { visual.api.goToHeading(index); }
            else if (readsAsPreview()) { var block = el.page.querySelector('[data-line="' + line.number + '"]'); if (block) { block.scrollIntoView({ block: 'center' }); } }
            tab.navigationLine = line.number; markOutline(); announce();
        }
        function locationChanged() { if (opts.ownsUrl) { takeFile(location.search); } }
        if (opts.ownsUrl) { window.addEventListener('popstate', locationChanged); }

        /* === Keeping the work through a reload ============================ */

        /* The Editor's snapshot (js/snapshot.js): each open tab with its
           unsaved text, cursor and selection, how far it is scrolled, its
           undo history and its search, and which tab is in front and whether
           which view each is in. A reload brings all of it back. A draft
           for a file that can't be reached when the Editor starts, as a
           server file can't while signed out, waits in the snapshot until it
           can.

           The same record is the Editor's session, kept in this browser's
           storage under pc-editor-session, so closing the Editor's window
           and opening it again, in this tab or another, or after the browser
           restarts, brings back the documents it had open, unsaved text
           and all. A reload prefers the tab's own snapshot, which is newer
           when two tabs each have an Editor. The first instance keeps the
           session; a numbered one keeps only its snapshot. */
        var snap = null;
        var waiting = [], restoring = false;
        var SESSION_KEY = 'pc-' + app + '-session';
        /* PUDL keys a numbered instance name-2 to name-9 (0.27.0). */
        var ownsSession = !/-[2-9]$/.test(String(opts.instance || ''));
        var sessionTimer = 0;
        function keep() {
            if (restoring) { return; }
            if (snap) { snap.changed(); }
            if (ownsSession && !resources) { clearTimeout(sessionTimer); sessionTimer = setTimeout(keepSession, 500); }
        }
        /* Writes the session now; false when the browser would not keep it. */
        function keepSession() {
            clearTimeout(sessionTimer);
            if (resources || !hostServices.legacySession || !ownsSession || restoring || !view || !CM) { return true; }
            var s = takeSnapshot();
            try {
                if (s && (s.tabs.length || s.waiting.length)) { hostServices.legacySession.write(app, s); }
                else { hostServices.legacySession.write(app, null); }
                return true;
            } catch (err) { return false; }
        }
        function readSession() {
            if (!ownsSession) { return null; }
            try { var s = hostServices.legacySession ? hostServices.legacySession.read(app) : null; return s && Array.isArray(s.tabs) ? s : null; }
            catch (err) { return null; }
        }

        /* Whether a file is the one a snapshot was taken against: its
           length and an FNV-1a hash of its text. */
        function fingerprint(text) {
            var h = 0x811c9dc5;
            for (var i = 0; i < text.length; i++) { h ^= text.charCodeAt(i); h = Math.imul(h, 0x01000193); }
            return text.length + ':' + (h >>> 0).toString(16);
        }

        /* Where a tab is scrolled to, as the line at the top of the view and
           how far into it, which survives a window of another size. */
        var pendingScroll = null;
        /* Puts the text's caret at a place the page gave (decision 113),
           with its line in view, and, where the preview is drawn, the
           preview's block that holds that line in view too. */
        function showPlace(place, tab) {
            if (!place || typeof place.from !== 'number' || !view || !CM || current() !== tab) { return; }
            var at = Math.max(0, Math.min(place.from, view.state.doc.length)), to = Math.max(at, Math.min(place.to == null ? at : place.to, view.state.doc.length));
            view.dispatch({ selection: { anchor: at, head: to }, effects: CM.EditorView.scrollIntoView(at, { y: 'center' }) });
            if (readsAsPreview()) {
                var line = view.state.doc.lineAt(at).number, block = null;
                el.page.querySelectorAll('[data-line]').forEach(function (b) { if (+b.getAttribute('data-line') <= line) { block = b; } });
                if (block) { block.scrollIntoView({ block: 'center' }); }
            }
        }
        function scrollAnchor() {
            if (!view) { return null; }
            /* A place asked for and not yet reached is still the place. */
            if (pendingScroll != null) { return pendingScroll; }
            var top = view.scrollDOM.scrollTop, block = view.lineBlockAtHeight(top);
            return block ? { at: block.from, off: Math.round(top - block.top) } : null;
        }
        function scrollTo(place) {
            pendingScroll = null;
            if (!place || typeof place !== 'object' || !view) { return; }
            var at = Math.min(place.at || 0, view.state.doc.length), off = place.off || 0, tab = current();
            var asked = pendingScroll = { at: at, off: off };
            requestAnimationFrame(function () {
                /* Unless another place was asked for meanwhile, or another
                   tab came to the front. */
                if (pendingScroll !== asked) { return; }
                pendingScroll = null;
                if (current() !== tab) { return; }
                /* The line is brought into view, which measures it, and the
                   offset into it then set in the same terms it was read in. */
                view.dispatch({ effects: CM.EditorView.scrollIntoView(at, { y: 'start', yMargin: 0 }) });
                requestAnimationFrame(function () {
                    if (current() === tab && view.state.doc.length >= at) { view.scrollDOM.scrollTop = view.lineBlockAt(at).top + off; }
                });
            });
        }

        function searchOf(state) {
            var q = CM.getSearchQuery(state), open = CM.searchPanelOpen(state);
            if (!open && !q.search) { return null; }
            return { open: open, search: q.search, replace: q.replace, caseSensitive: q.caseSensitive, regexp: q.regexp, wholeWord: q.wholeWord };
        }

        var SNAPSHOT_MAX = 1500000;
        function takeSnapshot() {
            /* Until what was kept is all back, it is still the whole of it. */
            if (restoring) { return snap && snap.kept; }
            if (!view || !CM) { return null; }
            var front = current();
            if (front) { front.state = view.state; front.scroll = scrollAnchor(); }
            var list = [], frontAt = -1;
            tabs.forEach(function (t) {
                var st = t.state, text = st.doc.toString();
                if (!t.path && !t.modified) { return; }
                if (t === front) { frontAt = list.length; }
                var json = st.toJSON({ sourceHistory: sourceHistory.field });
                list.push({
                    path: t.path, name: t.name, lang: t.lang, mtime: t.mtime, fresh: !!t.fresh,format:sourceFormat(t),
                    baseline: resources ? recovery.capture(resources, t.resource, t.baseline) : undefined,
                    draft: t.modified ? text : null, sum: t.modified ? null : fingerprint(text),rawSum:t.modified?null:fingerprint(sourceHistory.raw(st)),
                    selection: json.selection, sourceHistory:json.sourceHistory,newlines:sourceHistory.endings(st),scroll: t.scroll == null ? null : t.scroll,
                    search: searchOf(st), view: t === front ? viewMode : (t.view || 'text')
                });
            });
            var out = { tabs: list, front: frontAt, waiting: waiting, view: viewMode, explorer: explorer ? explorer.snapshot() : null };
            if (resources) out.version = 2;
            /* Undo history is the first thing let go when the whole would be
               too large for the browser to keep. */
            if (JSON.stringify(out).length > SNAPSHOT_MAX) { list.forEach(function (k) { k.sourceHistory = null; }); }
            return out;
        }

        /* A tab's kept state on the text it now has. Its cursor and undo
           history only fit the text they were taken from: the draft, or for
           a tab without changes, the file as it was then. A file changed
           since keeps its cursor near where it was, without its history. */
        async function applyKept(t, k) {
            var text = typeof k.draft === 'string' ? k.draft : t.saved;
            var unchangedRevision=!resources||!k.baseline||!!t.resource&&k.baseline.revision===t.resource.revision;
            var exact = typeof k.draft === 'string' || unchangedRevision&&(k.rawSum!=null?k.rawSum===fingerprint(t.savedRaw):k.sum!=null&&k.sum===fingerprint(t.saved));
            var state = null;
            var raw=exact&&typeof k.newlines==='string'?window.yavcSourceHistory.restore(text,k.newlines):typeof k.draft!=='string'&&typeof t.savedRaw==='string'&&normalizedSource(t.savedRaw)===text?t.savedRaw:text;
            if (exact && k.selection) {
                try {
                    var json = { doc: text, selection: k.selection };
                    if (k.sourceHistory) { json.sourceHistory = k.sourceHistory; }
                    state = CM.EditorState.fromJSON(json, { extensions: extensionsFor(t,raw) }, k.sourceHistory ? { sourceHistory: sourceHistory.field } : undefined);
                } catch (err) { state = null; }
            }
            if (!state) {
                state = stateFor(t, raw);
                var head = k.selection && k.selection.ranges && k.selection.ranges[k.selection.main || 0];
                if (head) {
                    var at = Math.min(head.head, text.length);
                    state = state.update({ selection: { anchor: at } }).state;
                }
            }
            if (k.search && k.search.search) {
                state = state.update({ effects: CM.setSearchQuery.of(new CM.SearchQuery(k.search)) }).state;
            }
            t.state = state;
            t.scroll = k.scroll;
            t.view = supportsView(t, k.view) ? k.view : resolvedView(t, markdownDefault());
            t.openSearch = !!(k.search && k.search.open);
            t.modified = sourceModified(t,state);
            /* The tab in front takes its new state through activate(), which
               would otherwise keep the old one. */
            var i = tabs.indexOf(t);
            if (i === active) { active = -1; await activate(i); }
        }

        /* The snapshot kept before the reload, or the Editor's own record
           from before snapshots, which it kept the same way. */
        function readKept() {
            if (snap && snap.kept && Array.isArray(snap.kept.tabs)) { return snap.kept; }
            var session = readSession();
            if (session) { if (resourceSession) resourceSession.validate(session); return session; }
            try {
                var old = hostServices.legacyWork ? hostServices.legacyWork(app, !resources) : null;
                if (resourceSession && old) resourceSession.validate(old);
                return old && Array.isArray(old.tabs) ? old : null;
            } catch (err) { return null; }
        }

        /* Reopens the kept tabs as they were; resolves to what to tell the
           reader, or null when there was nothing kept. */
        async function restoreKept() {
            var kept = readKept();
            if (!kept) { return null; }
            var back = [], all = (kept.waiting || []).concat(kept.tabs), frontTab = null;
            restoring = true;
            try { frontTab = await reopen(all, back, kept.tabs[kept.front] || null); } finally { restoring = false; }
            if (!isDocs && all.some(function (k) {return k && k.view === 'document';})) back.push('legacy page layout (open saved documents in Docs for Document view)');
            if (frontTab) { var fi = tabs.indexOf(frontTab); if (fi >= 0) { await activate(fi); } }
            var msg = back.length ? 'Your unsaved changes to ' + back.join(', ') + ' are back.' : '';
            if(all.some(function(k){return k&&k.history&&!k.sourceHistory;}))msg+=(msg?' ':'')+'The source and selection were restored. The previous editing history cannot be migrated.';
            if (waiting.length) {
                msg += (msg ? ' ' : '') + 'Unsaved changes to ' + waiting.map(function (k) { return k.path; }).join(', ') +
                    ' are kept until the file can be reached again; you may need to sign in, then reload.';
            }
            if (explorer && kept.explorer) { explorer.restore(kept.explorer); }
            keep();
            // A snapshot from before the views kept only whether the preview showed.
            return { text: msg, error: waiting.length > 0, view: kept.view || 'text' };
        }
        async function reopen(all, back, frontKept) {
            var frontTab = null;
            for (var i = 0; i < all.length; i++) {
                var k = all[i], t = null;
                if (!k || typeof k !== 'object') { continue; }
                var originalKept=k;
                if(typeof k.draft==='string'&&!k.format){
                    var signed=k.draft[0]==='\ufeff';k=Object.assign({},k,{format:{encoding:'utf-8',bom:signed}});
                    if(signed){k.draft=k.draft.slice(1);k.sourceHistory=null;k.history=null;if(k.selection&&Array.isArray(k.selection.ranges)&&k.selection.ranges.every(function(range){return range&&Number.isInteger(range.anchor)&&Number.isInteger(range.head);})){k.selection=Object.assign({},k.selection,{ranges:k.selection.ranges.map(function(range){return Object.assign({},range,{anchor:Math.max(0,range.anchor-1),head:Math.max(0,range.head-1)});})});}else{k.selection=null;}}
                }
                if (resources && k.path) {
                    var restored;
                    try { restored = await recovery.restore(resources, k.path, k.baseline, k.draft); }
                    catch (err) { waiting.push(k); continue; }
                    if(restored.modified&&restored.snapshot&&k.format){var loadedFormat=sourceFormat({resource:restored.snapshot});if(loadedFormat.encoding!==k.format.encoding||loadedFormat.bom!==k.format.bom){restored.snapshot=null;restored.conflict='encoding';}}
                    // A legacy draft identical to the current buffer contains no unsaved edit.
                    if (restored.conflict === 'unknown') {
                        try {
                            var sameFile = await resources.read(k.path);
                            var recoveredRaw=typeof k.newlines==='string'?window.yavcSourceHistory.restore(normalizedSource(restored.text),k.newlines):restored.text;
                            var recoveredFormat=window.yavcContentResources.format(k.format||{}),savedFormat=sourceFormat({resource:sameFile});
                            if (recoveredRaw === sameFile.text&&recoveredFormat.encoding===savedFormat.encoding&&recoveredFormat.bom===savedFormat.bom) {
                                restored = { snapshot: sameFile, baseline: null, text: sameFile.text, modified: false, conflict: null };
                            }
                        } catch (err) { /* Keep the original draft when its file is unavailable. */ }
                    }
                    if (destroyed) return null;
                    var name = (restored.snapshot && restored.snapshot.name) || k.name || String(k.path).split('/').pop();
                    await addTab({ path: k.path, name: name, lang: languageFor(name), resource: restored.snapshot, baseline: restored.baseline, conflict: restored.conflict,format:k.format,
                        readOnly: !!restored.snapshot && !resources.canWrite(restored.snapshot) }, restored.snapshot ? restored.snapshot.text : '');
                    t = current(); await applyKept(t, Object.assign({}, k, { draft: restored.modified ? restored.text : null }));
                    t.modified = restored.modified; t.conflict = restored.conflict;
                    if (t.modified) back.push(t.name);
                    if (originalKept === frontKept) frontTab = t;
                    continue;
                }
                if (!k.path) {
                    if (typeof k.draft !== 'string') { continue; }
                    var n = /^untitled-(\d+)(\.docs\.md)?$/.exec(k.name || '');
                    if (n) { untitled = Math.max(untitled, +n[1]); }
                    await addTab({ path: null, name: k.name || 'untitled', lang: k.lang || 'text', readOnly: false,format:k.format }, '');
                    t = current();
                } else {
                    var node = F.resolve(F.home(), F.upgradePath(k.path));
                    if (!node && k.fresh) {
                        await newNamed(k.path);
                        t = current() && current().path === k.path ? current() : null;
                    } else if (node && await openPath(k.path)) {
                        t = current();
                        /* The file was saved somewhere else since: the
                           reader's text stays, and saving would replace
                           that version. */
                        if (typeof k.draft === 'string' && k.mtime != null && t.mtime != null && k.mtime !== t.mtime) { t.external = true; }
                    } else if (typeof k.draft === 'string') {
                        waiting.push(k);
                    }
                }
                if (!t) { continue; }
                await applyKept(t, k);
                if (t.modified) { back.push(t.name); }
                if (originalKept === frontKept) { frontTab = t; }
            }
            renderTabs();
            return frontTab;
        }

        /* Unsaved text kept in the session comes back with the Editor, so
           leaving the page or closing the window asks only when the session
           could not keep it (static/doc-tabs.js guard). Save All saves every
           changed file, and the window stays if a save fails. */
        var unguard = window.yavcDocTabs.guard(root, {
            changed: function () { return tabs.filter(dirtyTab).map(function (t) { return t.name; }); },
            keptOnReload: function () { if (pageActive() && visual.api.pending()) return false; return resources ? !!snap && snap.flush() : ownsSession && keepSession(); },
            keptOnClose: function () {
                if (pageActive() && visual.api.pending()) return false;
                return resources ? !!(snap && ownsSession && H.snapshot && H.snapshot.remembersClosed && snap.flush()) : ownsSession && keepSession();
            },
            saveAll: async function () {
                for (var i = 0; i < tabs.length; i++) {
                    if (!dirtyTab(tabs[i])) { continue; }
                    await activate(i);
                    if (!(await save()) || dirtyTab(tabs[i])) { return false; }
                }
                return true;
            }
        });

        /* === Controls ===================================================== */

        /* A tab closed from the keyboard leaves the focus on the tab that
           takes its place, so Delete can close one after another. */
        /* Preview opens the page as it is on the site. In the window view it
           opens a page of this site as a window rather than a browser tab,
           and one already open comes to the front with its content fetched
           again, so it shows what was just saved. Anywhere else, or for an
           address that is not a page of this site, it opens a new tab. Text
           not yet saved is not on the site, which the status line says. */
        function previewNote() {
            if (current() && current().modified) { say('Preview shows the page as it is on the site; save to see your changes there.'); }
        }
        /* Preview: the file's page on the site, in a window in the window
           view and in a new tab otherwise. */
        function preview() {
            if (!siteUrl) { return; }
            var href = new URL(siteUrl, location.href).href;
            previewNote();
            if (!inWindowView(href)) { window.open(href, '_blank', 'noopener'); }
        }
        function openSource() {
            var cur = current(), node = cur && F.resolve(F.home(), cur.path), src = node && F.sourceOf(node);
            if (src) { openPath(F.displayPath(src)); }
        }
        async function openInOther() {
            var tab=current(),destination=isDocs?'editor':'docs';if(!tab || !H.fileResolution || !H.fileResolution.open)return;
            if(pageActive())await visual.api.sync();if(destroyed || current()!==tab)return;
            var changed=dirtyTab(tab),choice='saved';
            if(changed || !tab.path || tab.fresh){
                choice=await window.yavcDialog.askChoice('Open in '+(isDocs?'Editor':'Docs'),'Save your changes before opening this document in the other application.',tab.path && !tab.fresh ? [['cancel','Cancel'],['saved','Open Saved Version'],['save','Save and Open',true]] : [['cancel','Cancel'],['save','Save and Open',true]]);
                if(!choice || choice==='cancel' || destroyed || current()!==tab)return;
                if(choice==='save' && (!(await save()) || dirtyTab(tab)))return;
            }
            if(destroyed || current()!==tab || !tab.path || tab.fresh || !inFront())return;
            try{await H.fileResolution.open({resource:tab.path,applet:destination},{host:root});}catch(err){if(!destroyed)say(err.message,true);}
        }
        function inWindowView(href) {return !!(hostServices.openPage && hostServices.openPage(href));}

        /* === Commands and their keys ===================================== */

        /* Every command a key can be bound to, by the name ~/.config/editor.json
           uses for it. */
        var COMMANDS = {
            'new': function () { return liveActions['new@1']({}); },
            'open': openDialog,
            'save': saveCommand,
            'save-as': saveAs,
            'close-document': function () { if (current()) { closeTab(active); } },
            'undo': function () { return editing().history('undo'); },
            'redo': function () { return editing().history('redo'); },
            'find': openFind,
            'go-to-line': goToLine,
            'toggle-files': toggleExplorer,
            'toggle-outline': toggleOutline,
            'hide-navigation': hideNavigation,
            'section-up': function () { return moveOutline('up'); },
            'section-down': function () { return moveOutline('down'); },
            'toggle-wrap': function () { setWrap(!wrapOn, true); },
            'preview': preview,
            'insert-timestamp': function () { insertText(now()); },
            'insert-date': function () { insertText(now('date')); },
            'stamp-modified': stampModified,
            'key-bindings': editKeys
        };

        function goToLine() {
            var binding = CM.searchKeymap.find(function (key) { return key.key === 'Mod-Alt-g'; });
            if (binding) { binding.run(view); }
        }

        /* The bindings in force: the Editor's own, with the settings file's
           over them. A key set to null runs nothing and goes no further. */
        var keyProblems = '';
        function effectiveKeys() {
            var own = settings.keys && typeof settings.keys === 'object' && !Array.isArray(settings.keys) ? settings.keys : {};
            return Object.assign({}, DEFAULT_KEYS, own);
        }
        function keyBindings() {
            var map = effectiveKeys(), list = [], problems = [];
            Object.keys(map).forEach(function (key) {
                var name = map[key];
                if (!validKey(key)) { problems.push('"' + key + '" isn\'t a key CodeMirror knows'); return; }
                if (name === null) { list.push({ key: key, preventDefault: true, run: function () { return true; } }); return; }
                var run = COMMANDS[name];
                if (!run) { problems.push('there is no command called "' + name + '"'); return; }
                list.push({ key: key, preventDefault: true, run: function () { run(); return true; } });
            });
            keyProblems = problems.length ? 'In the key bindings, ' + problems.join(', and ') + '.' : '';
            return CM.keymap.of(list);
        }

        /* The key for a command, as a menu shows it. */
        function shortcutFor(name) {
            var map = effectiveKeys();
            var key = Object.keys(map).find(function (k) { return map[k] === name && validKey(k); });
            return key ? keyLabel(key) : undefined;
        }

        function insertText(text) {
            var t = current();
            if (!t || t.readOnly) { say('This file is read-only, so nothing can be added to it.', true); return; }
            if (formatsThePage()) { return visual.api.insertMarkdown(text); }
            view.dispatch(view.state.replaceSelection(text), { scrollIntoView: true, userEvent: 'input' });
            view.focus();
        }

        /* Sets the page's last-modified date to now: the lastModified line
           of a Markdown page's front matter, in local time with its offset,
           or an HTML page's last-modified meta tag, in UTC as those are
           kept. Either is added if the page has none. */
        async function stampModified() {
            var t = current();
            if (!t || t.readOnly) { say('This file is read-only, so its date can\'t be changed.', true); return; }
            if(formatsThePage()&&pageActive()){await visual.api.sync();if(destroyed||current()!==t)return;}
            var doc = view.state.doc.toString(), eol = /\r\n/.test(doc) ? '\r\n' : '\n', change, stamp;
            if (t.lang === 'markdown') {
                var fm = /^---[ \t]*\r?\n([\s\S]*?\r?\n)?---[ \t]*(?:\r?\n|$)/.exec(doc);
                if (!fm) { say('This page has no front matter (the lines between --- at its top), so there is no last-modified date to set.', true); return; }
                stamp = now();
                var body = fm[1] || '', from = fm[0].indexOf('\n') + 1;
                var line = /^lastmodified[ \t]*:.*$/im.exec(body);
                if (line) {
                    change = { from: from + line.index, to: from + line.index + line[0].length, insert: line[0].replace(/:.*$/, ': ' + stamp) };
                } else {
                    var date = /^date[ \t]*:.*(?:\r?\n|$)/im.exec(body);
                    change = { from: from + (date ? date.index + date[0].length : body.length), insert: 'lastModified: ' + stamp + eol };
                }
            } else if (/\.html?$/i.test(t.path || t.name)) {
                stamp = now('utc');
                var tag = '<meta http-equiv="last-modified" content="' + stamp + '" />';
                var meta = /<meta\s+http-equiv\s*=\s*["']last-modified["'][^>]*>/i.exec(doc);
                if (meta) {
                    change = { from: meta.index, to: meta.index + meta[0].length, insert: tag };
                } else {
                    var head = /<\/head\s*>/i.exec(doc);
                    if (!head) { say('This page has no <head>, so there is nowhere to put its last-modified date.', true); return; }
                    var indent = /[ \t]*$/.exec(doc.slice(0, head.index))[0];
                    change = { from: head.index, insert: tag + eol + indent };
                }
                stamp += ' UTC';
            } else {
                say('Update last-modified works in a Markdown or HTML page.', true);
                return;
            }
            view.dispatch({ changes: change, userEvent: 'input' });
            view.focus();
            say('Last modified ' + stamp + '.');
        }

        /* Preferences belong to their application and use the host config
           service, without opening a settings file as a document. */
        async function editKeys() {
            await loadScript(window.yavcUtil.versioned(beside('key-bindings.js')));
            if(destroyed)return;
            var baseline=JSON.stringify(settings.keys || {}), titles=new Map(commandTable().map(function (c) {return [c.id,c.label];}));
            return window.yavcKeyBindings.open({title:isDocs?'Docs':'Editor',defaults:DEFAULT_KEYS,keys:settings.keys,keyLabel:keyLabel,validKey:validKey,signal:styleLifetime.signal,
                commands:Object.keys(COMMANDS).sort().map(function (id) {return {id:id,label:typeof titles.get(id)==='string'?titles.get(id):id};}),
                current:function(){return !destroyed&&JSON.stringify(settings.keys || {})===baseline;},
                save:function(keys){return loadConfig().then(function (c) {return c.set(app,{keys:keys});});}});
        }

        /* === The command table and the toolbar (PUDL 0.68.0) ===============
           Every command the toolbar, the menus, the keys and an agent reach,
           once, by the names ~/.config/editor.json binds keys to. A command's
           shortcut is the key bound to it, or failing one, CodeMirror's own;
           CodeMirror answers its keys in the text first, so PUDL answers a
           key only where CodeMirror did not. */
        var FORMATS = window.yavcMdEditor ? window.yavcMdEditor.FORMATS : [];
        var STYLES = window.yavcMdEditor ? window.yavcMdEditor.STYLES : [];
        function blockStyle() {
            if (formatsThePage()) { return visual.api && visual.api.block ? visual.api.block() : null; }
            if (!view) { return null; }
            var m = /^\s{0,3}(#{1,6})(\s|$)/.exec(view.state.doc.lineAt(view.state.selection.main.head).text);
            return m ? 'h' + m[1].length : 'paragraph';
        }
        function commandTable() {
            var ready = !!(CM && view), tab = current(), editable = !!(tab && !tab.readOnly);
            function key(name, builtIn) { return function () { return (ready && shortcutFor(name)) || (builtIn ? keyLabel(builtIn) : undefined); }; }
            function cmd(id, label, glyph, run, more) {
                return Object.assign({ id: id, label: label, glyph: glyph || undefined, shortcut: key(id, more && more.builtIn),
                    disabled: function () { return !ready; }, run: run }, more || {});
            }
            var table = [
                cmd('suggestion-next','Next Host Suggestion','arrow-down',function(){return hostSuggestions.navigate(1);},{hidden:function(){return !hostSuggestions;},disabled:function(){return !hostSuggestions||!hostSuggestions.available();}}),
                cmd('suggestion-previous','Previous Host Suggestion','arrow-up',function(){return hostSuggestions.navigate(-1);},{hidden:function(){return !hostSuggestions;},disabled:function(){return !hostSuggestions||!hostSuggestions.available();}}),
                cmd('review-changes','Review Changes…','comments',async function(){return (await reviewTools()).browse();},{hidden:function(){return !tab||tab.lang!=='markdown';}}),
                cmd('review-addition','Suggest Insertion…','plus',async function(){return (await reviewTools()).propose('addition');},{hidden:function(){return !tab||tab.lang!=='markdown';},disabled:function(){return !reviewProposable();}}),
                cmd('review-deletion','Suggest Deletion','minus',async function(){return (await reviewTools()).propose('deletion');},{hidden:function(){return !tab||tab.lang!=='markdown';},disabled:function(){return !reviewProposable();}}),
                cmd('review-replacement','Suggest Replacement…','pencil',async function(){return (await reviewTools()).propose('replacement');},{hidden:function(){return !tab||tab.lang!=='markdown';},disabled:function(){return !reviewProposable();}}),
                cmd('review-comment','Add Comment…','comments',async function(){return (await reviewTools()).propose('comment');},{hidden:function(){return !tab||tab.lang!=='markdown';},disabled:function(){return !reviewProposable();}}),
                cmd('review-highlight','Highlight for Review','quote',async function(){return (await reviewTools()).propose('highlight');},{hidden:function(){return !tab||tab.lang!=='markdown';},disabled:function(){return !reviewProposable();}}),
                cmd('review-accept-all','Accept All Changes…','tick',async function(){return (await reviewTools()).all('accept');},{hidden:function(){return !tab||tab.lang!=='markdown';},disabled:function(){return !ready||!tab||tab.readOnly;}}),
                cmd('review-reject-all','Reject All Changes…','close',async function(){return (await reviewTools()).all('reject');},{hidden:function(){return !tab||tab.lang!=='markdown';},disabled:function(){return !ready||!tab||tab.readOnly;}}),
                cmd('toggle-files', 'Files', 'folder', toggleExplorer, { hidden:function () {return !explorer;}, checked: function () { return !el.explorer.hidden; } }),
                cmd('toggle-outline', 'Outline', 'list', toggleOutline, { checked: function () { return outlineVisible; }, shortcut: key('toggle-outline', 'Mod-Alt-o') }),
                cmd('hide-navigation', 'Hide Side Panes', 'panel', hideNavigation, { hidden: function () { return el.explorer.hidden && !outlineVisible; } }),
                cmd('section-up', 'Move Section Up', 'arrow-up', function () { return moveOutline('up'); }, { shortcut: key('section-up', 'Alt-Shift-ArrowUp'), disabled: function () { var entry = outlineEntries[outlineTarget]; return !editable || !entry || entry.previous < 0; } }),
                cmd('section-down', 'Move Section Down', 'arrow-down', function () { return moveOutline('down'); }, { shortcut: key('section-down', 'Alt-Shift-ArrowDown'), disabled: function () { var entry = outlineEntries[outlineTarget]; return !editable || !entry || entry.next < 0; } }),
                cmd('new', 'New', 'file', function () { return liveActions['new@1']({}); }),
                cmd('new-template','New from Template…','document',newFromTemplate,{hidden:function(){return !isDocs;},disabled:function(){return !ready;}}),
                cmd('new-markdown','New Markdown','document',async function () {untitled++;await importedDocument('untitled-'+untitled+'.md','');},{hidden:function () {return !isDocs;}}),
                cmd('open', 'Open…', 'folder', async function (input) {
                    if(!input || typeof input.resource!=='string')return openDialog();
                    if(input.view && !MARKDOWN_VIEWS.some(function (v) {return v[0]===input.view;}))throw new Error('This view is unavailable here.');
                    if (!tabs.some(function (t) {return t.path===input.resource;})) { var routed=await routeDocument('open-file',{path:input.resource,view:input.view || input.initialView});if(routed)return true; }
                    var ticket=++stateRequest;function currentRequest(){return !destroyed && ticket===stateRequest;}
                    var existed=tabs.some(function (t) {return t.path===input.resource;});
                    var opened=await openPath(input.resource,true,currentRequest,input.view);
                    if(!opened || !currentRequest())return false;
                    var mode=input.view || (!existed && initialMode(current(),input.initialView));
                    if(mode && supportsView(current(),mode)){current().view=mode;await setView(mode);}return true;
                }),
                cmd('save', 'Save', 'yavc-save', function () { return saveCommand(); }, { label: function () { var s = ready && hostCommands.standIn(); return s ? s.label : 'Save'; }, glyph: function () { var s = ready && hostCommands.standIn(); return s && s.glyph || 'yavc-save'; },
                    disabled: function () { var s = ready && hostCommands.standIn(); if (s) return s.disabled; var t = current(); return !ready || !t || t.readOnly || !resources || !t.path && !resources.canSaveAs() || t.fresh && !resources.canCreate() || !!(t.path && !t.resource && !t.fresh); },
                    reason: function () { var s = ready && hostCommands.standIn(); return s ? s.reason : 'This host cannot save this draft safely, or its recovered revision no longer matches. Use a supported Save As or download the source.'; } }),
                cmd('save-as', 'Save As…', 'yavc-save-as', function () { return saveAs(); }, { label: function () { var t = current(); return t && t.readOnly ? 'Save a Copy…' : 'Save As…'; }, disabled: function () { return !ready || !current() || !!(resources && !resources.canSaveAs()); } }),
                cmd('save-encoded-copy','Save Encoded Copy…','yavc-save-as',encodedCopy,{disabled:function(){return !ready||!current()||!resources||!resources.canSaveAs();}}),
                cmd('import-docx','Import Word…','upload',importWord,{hidden:function(){return !isDocs;},disabled:function(){return !ready;}}),
                cmd('close-document', 'Close Document', null, function () { COMMANDS['close-document'](); }, { disabled: function () { return !ready || !current(); } }),
                /* glyphOnly: a glyph every reader knows shows alone on the
                   menu bar's ribbon (PUDL 0.79.0), its words its tooltip. */
                cmd('export-docx', 'Download as Word', 'download', function () { return downloadDocument('docx'); }, { hidden:function () {return !isDocs;}, disabled: function () { return !ready || !tab || tab.lang !== 'markdown'; } }),
                cmd('export-pdf', 'Download as PDF', 'download', function () { return downloadDocument('pdf'); }, { hidden:function () {return !isDocs;}, disabled: function () { return !ready || !tab || tab.lang !== 'markdown'; } }),
                cmd('undo', 'Undo', 'yavc-undo', function () { return editing().history('undo'); }, { builtIn: 'Mod-z', glyphOnly: true, disabled: function () { return !editing() || !editing().canHistory('undo'); } }),
                cmd('redo', 'Redo', 'yavc-redo', function () { return editing().history('redo'); }, { builtIn: 'Mod-y', glyphOnly: true, disabled: function () { return !editing() || !editing().canHistory('redo'); } }),
                cmd('find', 'Find and Replace…', 'search', openFind, { builtIn: 'Mod-f' }),
                cmd('json-format', 'Format Source', 'code', function () { if (lsp) { lsp.format(view); } }, { shortcut: 'Shift+Alt+F', hidden: function () { return isDocs || !tab || ['json','css','html'].indexOf(tab.lang) < 0; }, disabled: function () { return !editable || !lsp; } }),
                cmd('language-restart', 'Restart Language Assistance', 'reload', function () { if (lsp) { lsp.restart(); view.dispatch({ effects: lspComp.reconfigure(languageAssistance(current())) }); } }, { hidden: function () { return isDocs || !tab || ['json','css','html'].indexOf(tab.lang) < 0; }, disabled: function () { return !ready || !lsp; } }),
                cmd('go-to-line', 'Go to Line…', null, goToLine, { builtIn: 'Mod-Alt-g' }),
                cmd('toggle-wrap', 'Wrap Lines', 'yavc-wrap', function () { setWrap(!wrapOn, true); }, { checked: function () { return wrapOn&&!longLinesActive(); },disabled:function(){return !ready||longLinesActive();},reason:'Line wrapping is unavailable while a line exceeds 65,536 characters.' }),
                cmd('insert-timestamp', 'Insert Date and Time', null, COMMANDS['insert-timestamp'], { disabled: function () { return !ready || !editable; } }),
                cmd('insert-date', 'Insert Date', null, COMMANDS['insert-date'], { disabled: function () { return !ready || !editable; } }),
                cmd('stamp-modified', 'Update Last-Modified', null, stampModified, { disabled: function () { return !ready || !editable; } }),
                cmd('key-bindings', 'Key Bindings…', null, editKeys),
                cmd('insert', 'Other', 'more', null, { items: insertItems, hidden: function () { return !formatOffered; }, shortcut: undefined }),
                cmd('source', 'Edit the Source', 'code', openSource, { hidden: function () { return !sourceOffered; } }),
                cmd('open-other',isDocs?'Open in Editor':'Open in Docs',isDocs?'code':'document',openInOther,{hidden:function () {return !H.fileResolution || !H.fileResolution.open || !tab || !/\.(md|markdown)$/i.test(tab.name) || /\.slides\.md$/i.test(tab.name);}}),
                cmd('preview', 'Preview', 'new-window', preview, { hidden: function () { return !siteUrl; } }),
                /* A deck (a file named .slides.md) presents in Slides, where
                   the site has it. */
cmd('edit-diagram', 'Edit Diagram', 'new-window', function () { return openDiagram(diagramAtCursor()); }, { hidden: function () { var t = current(); return !t || t.lang !== 'markdown' || !canOpenDiagram(); }, disabled: function () { return !ready || !diagramAtCursor(); }, reason: 'Put the cursor on a diagram from a file first.' }),
                cmd('present', 'Present the Deck', 'play', function () {
                    var t = current();
                    if (t && t.path && window.pudlApplets.request) { window.pudlApplets.request('present', { path: t.path, present: '1' }, root); }
                }, { hidden: function () { var t = current(); return !(t && t.path && /\.slides\.md$/i.test(t.path) && window.pudlApplets.can && window.pudlApplets.can('present')); } })
            ];
            FORMATS.forEach(function (f) {
                table.push(cmd(f[0], f[1], f[2], function () { applyFormat(f[0]); }, {
                    shortcut: f[3] ? keyLabel(f[3]) : undefined, hidden: function () { return !formatOffered; }, glyphOnly: true,
                    checked: f[0] === 'quote' ? function () { return window.yavcMdEditor.quoteState(editCtx()); } : undefined
                }));
            });
            /* The style menu has no glyph, so the style's name stays on the
               row when the other formatting folds to glyphs. Each level is
               hidden where there is nothing to format, so its key does
               nothing there either. */
            var unformatted = function () { return !formatOffered; }, noEdit = function () { return !ready || !editable; };
            table.push({ id: 'style', label: 'Paragraph Style', hidden: unformatted, disabled: noEdit,
                current: function () { var s = blockStyle(), f = STYLES.find(function (x) { return x[0] === s; }); return f ? f[1] : null; },
                items: STYLES.map(function (s) {
                    return { id: s[0], label: s[1], shortcut: keyLabel(s[2]), choice: 'ed-style', value: s[0],
                        checked: function () { return blockStyle() === s[0]; }, hidden: unformatted, disabled: noEdit,
                        run: function () { applyFormat(s[0]); } };
                }) });
            if (window.yavcMdEditor) {
                table.push(window.yavcMdEditor.tableCommand(editCtx(), { hidden: unformatted, disabled: noEdit }));
                table.push(window.yavcMdEditor.boxCommand(editCtx(), { hidden: unformatted, disabled: noEdit }));
            }
            /* The document's front matter as a form (DESIGN.md, decision 17). */
            table.push(cmd('properties', 'Properties…', 'tag', editProperties, {
                hidden: function () { var t = current(); return !isDocs || !(t && t.lang === 'markdown'); },
                disabled: function () { var t = current(); return !ready || !t || !!t.readOnly; } }));
            var noPageLayout=function () {return !isDocs || unformatted() || viewMode!=='document';};
            table.push(cmd('document-layout','Switch to Document View','document',function () {current().view='document';return setView('document');},{hidden:function () {return !isDocs || unformatted() || viewMode==='document';}}));
            table.push(cmd('page-setup', 'Page Setup…', 'file', editPageSetup, { hidden: noPageLayout, disabled: noEdit }));
            table.push(cmd('header-footer', 'Header and Footer…', 'file', function () { return editDocumentOptions('furniture'); }, { hidden: noPageLayout, disabled: noEdit }));
            table.push(cmd('typography', 'Typography…', 'paragraph', function () { return editDocumentOptions('typography'); }, { hidden: unformatted, disabled: noEdit }));
            table.push(cmd('document-theme','Document Theme…','theme',function(){return editDocumentOptions('theme');},{hidden:unformatted,disabled:noEdit}));
            table.push(cmd('document-zoom', 'Document Zoom', 'search', null, { hidden: function () { return viewMode !== 'document'; }, items: [
                cmd('document-fit-width', 'Fit Width', null, function () { setDocumentZoom('width'); }),
                cmd('document-fit-page', 'Fit Page', null, function () { setDocumentZoom('page'); }),
                cmd('document-zoom-custom', 'Zoom…', null, async function () { var value = await window.yavcMdEditor.askText('Document Zoom', 'Percent', /^\d/.test(documentZoom) ? documentZoom : '100', { validate: function (text) { return !Number.isFinite(Number(text)) || Number(text) < 25 || Number(text) > 400 ? 'Enter 25 to 400.' : null; } }); if (value != null) { setDocumentZoom(value); } }),
                cmd('document-zoom-reset', '100%', null, function () { setDocumentZoom('100'); })
            ] }));
            table.push(cmd('print-document', 'Print…', 'yavc-print', printDocument, { hidden: unformatted, disabled: function () { return !ready || !tab || tab.lang !== 'markdown'; } }));
            table.push(cmd('stylesheet', 'Document Stylesheet…', 'code', function () { return editStyles(false); }, { shortcut: 'Mod+Shift+F', hidden: unformatted, disabled: noEdit }));
            table.push(cmd('custom-shortcodes', 'Custom Shortcodes…', 'code', editDefinitions, { hidden: unformatted, disabled: noEdit }));
            table.push(cmd('footnote','Footnote…','comments',insertFootnote,{hidden:unformatted,disabled:noEdit}));
            table.push(cmd('shared-stylesheet', 'Edit Shared Stylesheet…', 'file', function () { return editStyles(true); }, { hidden: unformatted,
                disabled: function () { return noEdit() || !/^~\/.+\.css$/i.test(currentSharedStyle) || !(window.pudlHost && window.pudlHost.fs && window.pudlHost.fs.writeText); },
                reason: 'Choose a home-folder CSS file in Document Stylesheet first. This host also needs a writable file service.' }));
            table.push({ id: 'md-default', label: 'Open Markdown As', hidden: function () { return !viewsOffered; },
                current: function () { var f = MARKDOWN_VIEWS.find(function (x) { return x[0] === markdownDefault(); }); return f ? f[1] : null; },
                items: MARKDOWN_VIEWS.map(function (v) {
                    return { id: 'md-default-' + v[0], label: v[1], choice: 'ed-md-default', value: v[0],
                        checked: function () { return markdownDefault() === v[0]; }, run: function () { setMarkdownDefault(v[0]); } };
                }) });
            MARKDOWN_VIEWS.forEach(function (v) {
                table.push({ id: 'view-' + v[0], label: v[1], choice: 'ed-view', choiceLabel: 'View', value: v[0],
                    checked: function () { return viewMode === v[0]; },
                    hidden: function () { return !tab; },
                    disabled: function () { return !ready || !supportsView(tab, v[0]); },
                    /* A view the reader chooses is the tab's from then on. */
                    run: function () { var t = current(); if (t) { t.view = v[0]; } setView(v[0]); } });
            });
            return table.concat(hostCommands.rows(ready), editing() ? editing().commands() : [], commonToolbar ? commonToolbar.commands() : [], [{ id: 'view-mode', label: 'View', glyph: 'panel', toolbarChoice: 'ed-view', toolbarPriority: 50, items: MARKDOWN_VIEWS.map(function (v) { return 'view-'+v[0]; }) }]);
        }

        /* The toolbar: what is used most stays named longest, and the view
           switch and Preview keep to the end of the row. */
        function toolbars() {
            if (commonToolbar) { return []; }
            /* ribbon: 'hide' (PUDL 0.80.1): every command here is in a
               menu (menus() below), so the ribbon has them while it shows. */
            return [{ label: isDocs ? 'Docs' : 'Editor', ribbon: 'hide', groups: [
                { label: 'File', glyph: 'file', priority: 3, items: ['toggle-files', 'new', 'open', 'save', 'save-as'] },
                { label: 'Edit', glyph: 'yavc-undo', priority: 2, items: ['undo', 'redo', 'find', 'toggle-wrap'] },
                { label: 'Format', glyph: 'bold', priority: 1, items: ['style', 'bold', 'italic', 'underline', 'strike', 'code', 'link', 'color', 'bullets', 'numbers', 'insert', 'table', 'box'] },
                { label: 'View', glyph: 'new-window', priority: 4, end: true, items: ['source', { choice: 'ed-view' }, 'preview'] }
            ] }];
        }

        var LAYOUT = ['toggle-files', 'toggle-outline', 'hide-navigation', 'toggle-wrap', '-'].concat(MARKDOWN_VIEWS.map(function (v) { return 'view-' + v[0]; }), ['document-zoom', 'md-default', '-', 'source', 'preview']);
        function menus() {
            var titles = [
                { label: isDocs ? 'Docs' : 'Editor', glyph: isDocs ? 'document' : 'code', items: H.identity(root, ['key-bindings']) },
                { id: 'file', label: 'File', glyph: 'file', items: ['new', 'new-template', 'new-markdown', 'open', 'open-other', 'import-docx', 'save', 'save-as', 'save-encoded-copy', '-', 'export-docx', 'export-pdf', 'print-document', '-', 'properties', 'present', '-', 'close-document'] },
                { id: 'edit', label: 'Edit', glyph: 'pencil', items: ['undo', 'redo', '-', 'cut', 'copy', 'paste', '-', 'select-all', '-', 'find', 'go-to-line', 'section-up', 'section-down', '-', 'json-format', 'language-restart'] },
                { id: 'insert', label: 'Insert', glyph: 'plus', items: ['link', 'insert', 'table', 'box', 'footnote', '-', 'insert-timestamp', 'insert-date', 'stamp-modified'] }
            ];
            /* Format, where there is formatting to do. */
            if (formatOffered) { titles.push({ id: 'format', label: 'Format', glyph: 'paragraph', items: ['style', '-', 'bold', 'italic', 'underline', 'strike', 'code', '-', 'highlight', 'superscript', 'subscript', '-', 'color', 'class', '-', 'bullets', 'numbers', 'quote', 'table', '-', 'edit-diagram', '-', 'document-layout','page-setup', 'document-theme', 'typography', 'header-footer', '-', 'stylesheet', 'shared-stylesheet', 'custom-shortcodes'] }); }
            if(current()&&(current().lang==='markdown'||hostSuggestions))titles.push({id:'review',label:'Review',glyph:'comments',items:['suggestion-previous','suggestion-next','-','review-changes','-','review-addition','review-deletion','review-replacement','review-comment','review-highlight','-','review-accept-all','review-reject-all']});
            /* The host's commands sit in File after the command each names,
               or after Save As. One for Review goes after the command it
               names, or at the end of that menu, which is there for it
               even when the document has no review marks to offer. */
            hostCommands.declared.forEach(function (d) {
                /* A command that stands in place of Save is the Save row where it is present, and has no row of its own. */
                if (d.replaces) return;
                var title = titles[1];
                if (d.menu === 'review') {
                    title = titles.filter(function (t) { return t.id === 'review'; })[0];
                    if (!title) { title = { id: 'review', label: 'Review', glyph: 'comments', items: [] }; titles.push(title); }
                }
                var items = title.items, at = d.after ? items.indexOf(d.after) : -1;
                if (at < 0) at = d.menu === 'review' ? items.length - 1 : items.indexOf('save-as');
                if (d.menu === 'review' && at === items.length - 1 && items.length && items[at].indexOf('host-') !== 0) { items.push('-'); at++; }
                while (at + 1 < items.length && items[at + 1].indexOf('host-') === 0) at++;
                items.splice(at + 1, 0, 'host-' + d.id);
            });
            return { titles: titles, into: { view: LAYOUT.concat(commonToolbar ? ['-', 'common-toolbar-show', 'common-toolbar-customize'] : []) } };
        }

        /* === Start ======================================================== */

        var boot = Promise.all([loadCodeMirror(), loadSiteFs().then(function (api) { F = api; return F ? F.load() : null; }), loadBrowser()]).then(async function (res) {
            if (destroyed) { return; }
            CM = res[0];
            if(hostServices.suggestions){
                if(typeof hostServices.suggestions.subscribe!=='function'||typeof hostServices.suggestions.decide!=='function')throw new Error('Host suggestions require subscribe and decide.');
                var K=await loadKit();await loadScript(K.here+'host-suggestions.js'+K.stamp);
                if(destroyed)return;
                hostSuggestions=window.yavcHostSuggestions.create(CM,{provider:hostServices.suggestions,root:root,signal:styleLifetime.signal,view:function(){return view;},owner:current,
                    readOnly:function(){return !current()||!!current().readOnly;},text:function(){return current()&&view?submittedText(current()):'';},rich:function(){return pageActive()?visual.api:null;},
                    prepare:async function(){if(pageActive())await visual.api.sync();},say:say,changed:commandsChanged,
                    focus:function(){if(pageActive())visual.api.focus();else if(view)view.focus();},
                    edit:async function(value,unchanged){var answer=await window.yavcDialog.open({title:'Edit Suggested Replacement',body:'<div class="form-group"><label class="form-label" for="host-suggestion-edit-'+n+'">Replacement</label><textarea class="form-input" id="host-suggestion-edit-'+n+'" rows="6" maxlength="65536">'+esc(value)+'</textarea></div>',buttons:[{id:'cancel',label:'Cancel'},{id:'apply',label:'Apply',primary:true}],signal:styleLifetime.signal});return answer.value==='apply'&&unchanged()?answer.body.querySelector('textarea').value:null;},
                    navigate:async function(item){if(pageActive()&&visual.api.goToSuggestion(item.id))return;await setView('text');view.dispatch({selection:{anchor:item.from,head:item.to},scrollIntoView:true});view.focus();}});
            }
            await loadScript(resourceAsset('source-history.js'));sourceHistory=window.yavcSourceHistory.create(CM);await loadScript(resourceAsset('resources.js'));
            await loadScript(resourceAsset('resource-session.js'));
            resourceSession = await window.yavcResourceSession.open(H, function (file) { return loadScript(resourceAsset(file)); });
            if (destroyed) return;
            if (resourceSession) { resources = resourceSession.client; recovery = resourceSession.recovery; }
            if (!resources && !F) throw new Error('This host has no revision-safe resource service.');
            var snapshotHost = H.snapshot;
            restoring = true;
            if (snapshotHost) snap = snapshotHost.join(root, takeSnapshot, { remember: !!resources && ownsSession, validate: resourceSession ? resourceSession.validate : function (value) { if (value && value.version === 2) throw new Error('This recovery snapshot requires its resource host. Its saved copy has been retained.'); } });
            if (resources && (!snap || typeof snap.flush !== 'function')) { if (snap) snap.leave(); snap = null; throw new Error('Resource editing requires recovery write acknowledgement.'); }
            lspComp = new CM.Compartment();
            try {
                var languageServices = await window.yavcMdEditor.languageServices();
                if (destroyed) { return; }
                lsp = languageServices.create(CM, function (message, detail) { say(message, detail && detail.persistent ? 'warn' : true); });
            } catch (err) { if (!destroyed) { say('Language assistance could not load. Text editing remains available.', 'warn'); } }
            if (destroyed) { return; }
            FB = res[2];
            /* Settings that can't be read leave the defaults in place. */
            var config = await loadConfig().catch(function () { return null; });
            if (config) {
                var ownSettings = await config.load(app, {}).catch(function () {return {};});
                if(destroyed)return;if(config.stored)ownSettings=await config.stored(app);
                if(destroyed)return;
                if(isDocs && !ownSettings.workspaceMigration && config.adopt){
                    var legacy=config.stored ? await config.stored('editor').catch(function () {return {};}) : {};var adopted={workspaceMigration:1};
                    if(destroyed)return;
                    ['wrap','explorer','explorerWidth','outline','outlineWidth','markdownView','documentView','documentZoom'].forEach(function (key) {if(Object.prototype.hasOwnProperty.call(legacy,key) && !Object.prototype.hasOwnProperty.call(ownSettings,key))adopted[key]=legacy[key];});
                    ownSettings=await config.adopt(app,adopted,'workspaceMigration');
                }
                if(destroyed)return;
                storedSettings=Object.assign({},ownSettings);
                settings = Object.assign({}, SETTINGS, ownSettings);
                documentZoom = /^(width|page)$/.test(String(settings.documentZoom)) || Number(settings.documentZoom) >= 25 && Number(settings.documentZoom) <= 400 ? String(settings.documentZoom) : '100';
                unConfig = config.onChange(app, applySettings);
            }
            if (destroyed) { return; }
            wrapOn = !!settings.wrap;
            /* The shortcodes for the Insert menu, which draws again once they are here. */
            if(isDocs)loadShortcodes().then(commandsChanged);
            loadKit().then(function () { FORMATS = window.yavcMdEditor.FORMATS; STYLES = window.yavcMdEditor.STYLES; INSERTS = window.yavcMdEditor.INSERTS; SHORTCODES = window.yavcMdEditor.SHORTCODES; commandsChanged(); });
            setupExplorer();
            setupExplorerSplit();
            wrapComp = new CM.Compartment(); langComp = new CM.Compartment(); lintComp = new CM.Compartment(); roComp = new CM.Compartment();
            longLineField=CM.StateField.define({create:function(state){return docLongLine(state.doc);},update:function(value,tr){
                if(!tr.docChanged)return value;var found=false;tr.changes.iterChangedRanges(function(a,b,from,to){for(var at=from;at<=to;){var line=tr.newDoc.lineAt(at);if(line.length>WRAP_LINE_LIMIT){found=true;break;}if(line.to>=to)break;at=line.to+1;}});return found||value&&docLongLine(tr.newDoc);
            }});
            keyComp = new CM.Compartment();
            view = new CM.EditorView({ parent: el.surface });
            view.scrollDOM.addEventListener('scroll', keep, { passive: true });
            unsubscribe = F ? F.onChange(onFsChange) : null;
            /* A host that reports changes to files is also saying that the
               document's pictures may have changed or moved, a web root
               newly chosen say, so the pictures in view are asked for again
               (content-workspace-contract.md). */
            if (H.fs && H.fs.onChange) {
                var picturesAgain = null;
                unwatchPictures = H.fs.onChange(function () {
                    clearTimeout(picturesAgain);
                    picturesAgain = setTimeout(function () { if (!destroyed && current() && window.yavcMdEditor) window.yavcMdEditor.loadImages(root, picturesHost(), true); }, 150);
                });
            }
            var restored = await restoreKept();
            if(destroyed)return;
            /* The file the address or window asks for comes to the front,
               which after a reload is the one that was in front. */
            var startExisted=tabs.some(function (tab) {return tab.path===startFile;});
            var opened = startFile ? await openPath(startFile) : false;
            if(destroyed)return;
            if (!opened) {
                if (startFile && F && !F.resolve(F.home(), startFile)) { await newNamed(startFile); } else if (!tabs.length) { await newTab(); }
            }
            restoring = false;
            /* The view showing before the reload comes back, which asks the
               server for the page, so it is not waited for. */
            if (restored && current() && supportsView(current(), current().view)) { await setView(current().view); }
            if (startFile && startLine > 1) { await setView(viewMode); await restoreLine(startLine); }
            var initialParams=new URLSearchParams(opts.state || (opts.ownsUrl ? location.search : ''));
            var requestedView = initialParams.get('view') || (!startExisted && current() && initialMode(current(),initialParams.get('initial-view')));
            if(requestedView){if(supportsView(current(),requestedView))await setView(requestedView);else say('This view is unavailable for this file. Choose a supported view from the toolbar.',true);}
            /* What came back is said once the view has settled, since
               showing a view clears the status line (standalone host
               review, 2026-11-11). A draft that can no longer be saved
               over its file has its own warning (tabWarning). */
            function sayRestored() { if (restored && restored.text) { say(restored.text, restored.error); } }
            sayRestored();
            /* The keyboard goes back where it was: into the text, unless the
               search panel had it, which opening it did. */
            if (snap && snap.focused) { if (!view.dom.contains(document.activeElement)) { view.focus(); } }
            else if (!(opts.ownsUrl || root.closest('.win.active'))) { view.contentDOM.blur(); }
            documentReady = true;
            if (documentWindows && documentWindows.mode(root) === 'single' && tabs.length > 1) {
                var snapshot = takeSnapshot(), front = current(), split = await documentWindows.split(root, app, snapshot);
                split.moved.slice().reverse().forEach(function (i) {
                    var saved = snapshot.tabs[i], at = tabs.findIndex(function (t) { return t.path === saved.path && t.name === saved.name; });
                    if (at >= 0) tabs.splice(at, 1);
                });
                active = -1; await activate(tabs.indexOf(front)); renderTabs(); if (snap) snap.flush();
                sayRestored();
                if (split.error) { var frontWarning = tabWarning(current()); say('Some recovered documents remain together. ' + split.error + (frontWarning ? ' ' + frontWarning : ''), 'warn'); tabWarned = true; }
            }
        }).catch(function (err) {
            if (destroyed) { return; }
            startupFailed=true;
            root.innerHTML = '<p class="ed-failed">The editor could not start: ' + esc(err.message) + '.</p>';
        });


        /* === The live actions (actions.json; docs/agents-design.md) ===== */

        function docNo(i) {
            if (i == null) { if (active < 0 || !tabs[active]) { throw new Error('no document is open'); } return active; }
            i = Math.floor(Number(i));
            if (!(i >= 1 && i <= tabs.length)) { throw new Error('there is no document ' + i + '; ' + tabs.length + ' are open'); }
            return i - 1;
        }
        function docText(i) { return i === active && view ? view.state.doc.toString() : tabs[i].state.doc.toString(); }
        function docView(i, withText) {
            var t = tabs[i];
            var v = { index: i + 1, name: t.name, path: t.path || null, lang: t.lang, modified: dirtyTab(t), readOnly: !!t.readOnly };
            if (withText) { v.text = docText(i); }
            return v;
        }
        async function setDocText(i, text) {
            if (i !== active) { await activate(i); }
            if (tabs[i].readOnly) { throw new Error('the document is read-only'); }
            view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: String(text) }, userEvent: 'input' });
            markModified(tabs[i]);
        }
        async function updateDocumentOptions(input, running) {
            docNo(); var tab = current(); if (tab.readOnly || tab.lang !== 'markdown') { throw new Error('Choose an editable Markdown document.'); }
            if (pageActive()) { await visual.api.sync(); }
            var text = view.state.doc.toString(), K = await loadKit(), P = await K.pages(), m = await mdEngine(), S = window.yavcDocumentSettings;
            if (destroyed || current() !== tab || view.state.doc.toString() !== text) { throw new Error('The document changed. Try again.'); }
            var set = {}, mappings = {}, allowed = running ? ['header', 'footer', 'header-first'] : ['font', 'heading-font', 'size', 'spacing', 'paragraph-spacing'];
            Object.keys(input).forEach(function (key) {
                if (allowed.indexOf(key) < 0) { throw new Error('Unsupported document setting.'); }
                if (running && /^(header|footer)$/.test(key) && input[key] && typeof input[key] === 'object' && !Array.isArray(input[key])) { mappings[key] = input[key]; }
                else { if (input[key] !== null && typeof input[key] !== 'string') { throw new Error('Document settings must be text.'); } set[key] = input[key]; }
            });
            var props = Object.assign({}, S.properties(m.front(text)), set, mappings);
            S.typography(props); var page = P.settings(props); page.furniture = S.furniture(props, m); S.check(page);
            takeVisualText(m.frontset({ text: text, set: set, mappings: mappings, order: Object.keys(set).concat(Object.keys(mappings)) }).text); textChangedForPage(); refreshStyles();
            return { output: { changed: true } };
        }
        var liveActions = {
            'theme@1':async function(input){docNo();var tab=current();if(tab.readOnly||tab.lang!=='markdown')throw new Error('Choose an editable Markdown document.');if(pageActive())await visual.api.sync();var text=view.state.doc.toString(),K=await loadKit();await K.pages();var m=await mdEngine();if(destroyed||tab!==current()||text!==view.state.doc.toString())throw new Error('The document changed. Try again.');window.yavcDocumentSettings.themeCSS({theme:input.value});takeVisualText(m.frontset({text:text,set:{theme:input.value==='plain'?null:input.value},order:['theme']}).text,true);textChangedForPage();refreshStyles();return {output:{theme:input.value}};},
            'new-template@1':async function(input){var G=await documentGallery();if(destroyed)throw new Error('This workspace is no longer available.');return liveActions['new@1']({text:G.stock(input.template)});},
            'header-footer@1': async function (input) { return updateDocumentOptions(input, true); },
            'typography@1': async function (input) { return updateDocumentOptions(input, false); },
            'document-zoom@1': async function (input) { docNo(); if (viewMode !== 'document') { throw new Error('Choose Document view before setting document zoom.'); } setDocumentZoom(input.value); return { output: { value: documentZoom } }; },
            'page-setup@1': async function (input) {
                docNo(); var tab = current(); if (tab.readOnly || tab.lang !== 'markdown') { throw new Error('Choose an editable Markdown document.'); }
                if (pageActive()) { await visual.api.sync(); }
                var text = view.state.doc.toString(), K = await loadKit(), P = await K.pages(), m = await mdEngine();
                if (destroyed || current() !== tab || view.state.doc.toString() !== text) { throw new Error('The document changed during page setup.'); }
                var props = P.properties(m.front(text)), values = Object.assign({}, props, input), page = P.settings(values);
                takeVisualText(m.frontset({ text: text, set: input, order: ['page', 'orientation', 'margins'] }).text); textChangedForPage(); refreshStyles();
                return { output: { width: page.width, height: page.height, margins: page.margins } };
            },
            'export@1': async function (input) {
                var result = await exportDocument(input.format,{interactive:false,reviewPolicy:input.reviewPolicy});
                return { output: { format: input.format, blocks: result.blocks, warnings: result.warnings }, artifacts: [{ key: input.format, name: result.name, mime: result.blob.type, data: new Uint8Array(await result.blob.arrayBuffer()) }] };
            },
            'review@1':async function(){var snapshot=await captureReview(false);if(!snapshot)throw new Error('The document changed.');return {output:snapshot.engine.review(snapshot.text)};},
            'review-resolve@1':async function(input){var snapshot=await captureReview(false);if(!snapshot||!snapshot.writable)throw new Error('Choose an editable Markdown document.');var result=snapshot.engine.resolveReview({text:snapshot.text,revision:input.revision,id:input.id,decision:input.decision});await snapshot.apply(result);return {output:{revision:result.resultRevision,modified:true}};},
            'review-propose@1':async function(input){var snapshot=await captureReview(false);if(!snapshot||!snapshot.writable)throw new Error('Choose an editable Markdown document.');var result=snapshot.engine.proposeReview({text:snapshot.text,revision:input.revision,from:input.from,to:input.to,kind:input.kind,body:input.body||''});await snapshot.apply(result);return {output:{revision:result.resultRevision,modified:true}};},
            'outline@1': async function () {
                docNo(); var tab = current(); if (tab.lang !== 'markdown') { throw new Error('Outline is available for Markdown files.'); }
                if (pageActive()) { await visual.api.sync(); }
                var text = view.state.doc.toString(), m = await mdEngine();
                if (current() !== tab || view.state.doc.toString() !== text) { throw new Error('The document changed. Read its outline again.'); }
                return { output: m.outlineState(text) };
            },
            'move-section@1': async function (input) {
                docNo(); var tab = current(); if (tab.readOnly || tab.lang !== 'markdown') { throw new Error('Choose an editable Markdown document.'); }
                if (pageActive()) { await visual.api.sync(); }
                var text = view.state.doc.toString(), m = await mdEngine();
                if (destroyed || current() !== tab || view.state.doc.toString() !== text) { throw new Error('The document changed. Read its outline again.'); }
                var moved = m.moveSection({ text: text, from: input.from, direction: input.direction, before: input.before, after: input.after, expected: input.revision });
                if (moved.text === text) { return { output: { moved: false, from: moved.from } }; }
                view.dispatch({ changes: { from: 0, to: text.length, insert: moved.text }, selection: { anchor: moved.from }, userEvent: 'input.move-section' });
                if (pageActive()) { await visual.api.reload(); }
                if (!destroyed && current() === tab && view.state.doc.toString() === moved.text) { await refreshOutline(); }
                return { output: { moved: true, from: moved.from } };
            },
            'table@1': async function (input) {
                docNo(); if (current().readOnly || !pageActive() || !visual.api.inTable()) { throw new Error('Select a table cell in an editable Visual or Split view.'); }
                if (!visual.api.run(input.op)) { throw new Error('The table command is unavailable for this selection.'); }
                await visual.api.sync(); return { output: { changed: true } };
            },
            'format@1': async function (input) {
                docNo(); if (current().readOnly) { throw new Error('The document is read-only.'); }
                if (current().lang !== 'markdown') { throw new Error('Choose a Markdown document before applying document formatting.'); }
                if (formatsThePage()) { if (!visual.api.run(input.op)) { throw new Error('This format is unavailable for the selection.'); } await visual.api.sync(); }
                else { window.yavcMdEditor.textFormat(view, input.op, editCtx()); }
                return { output: { formatted: true } };
            },
            'define@1': async function () { docNo(); if (pageActive()) { await visual.api.sync(); } return { output: (await documentEngine()).macros() }; },
            'documents@1': async function () { return { output: { active: active + 1, documents: tabs.map(function (t, i) { return docView(i, false); }) } }; },
            'document@1': async function (input) { var i = docNo(input.index),t=tabs[i]; if(i===active&&pageActive())await visual.api.sync(); if(tabs[i]!==t)throw new Error('The document closed while its text was being read.'); return { output: docView(i, true) }; },
            'open-file@1': async function (input) { if(!tabs.some(function (t) {return t.path===input.path;})){var routed=await routeDocument('open-file',input);if(routed)return routed.result;} await openPath(String(input.path), true); if(input.view && supportsView(current(),input.view))await setView(input.view); return { output: docView(active, false) }; },
            'new@1': async function (input) {
                /* A new document never takes the name of a file that exists,
                   or the person's next Save would overwrite it unasked
                   (security review, 2026-11-03). */
                if (input.path && F && F.resolve(F.home(), String(input.path))) { throw new Error('there is already a file at ' + input.path + '; open it, or give the new document another name'); }
                if (input.path) requireNative(input.path);
                var routed = await routeDocument('new', input); if (routed) return routed.result;
                if (input.path) { await newNamed(String(input.path)); } else { await newTab(); }
                if (input.text != null) { await setDocText(active, input.text); }
                return { output: docView(active, false) };
            },
            'set-text@1': async function (input) { var i = docNo(input.index); await setDocText(i, input.text); return { output: docView(i, false) }; },
            'insert@1': async function (input) {
                docNo();var tab=current();if(tab.readOnly)throw new Error('the document is read-only');
                if(await insertText(String(input.text))===false || current()!==tab)throw new Error('The document or selection changed before insertion.');
                if(pageActive())await visual.api.sync();
                if(current()!==tab)throw new Error('The document changed before insertion was acknowledged.');
                return {output:docView(active,false)};
            },
            'replace@1': async function (input) {
                var i = docNo(input.index), text = docText(i), n = 0;
                var flags = (input.all !== false ? 'g' : '') + (input.ignoreCase ? 'i' : '');
                var re = new RegExp(input.regex ? input.find : String(input.find).replace(/[.*+?^${}()|[\]\\]/g, '\\$&'), flags);
                var out = text.replace(re, function () { n++; return input.regex ? String(input.replace).replace(/\$(\d)/g, function (m, d) { return arguments.length > 3 ? (arguments[d] || '') : m; }) : input.replace; });
                if (n) { await setDocText(i, out); }
                return { output: { replaced: n } };
            },
            'save@1': async function (input) {
                var i = docNo(input.index);
                if (i !== active) { await activate(i); }
                var t = tabs[i];
                if (t.readOnly) { throw new Error('The document is read-only. Use Save As to create an editable copy.'); }
                var path = input.path ? String(input.path) : t.path;
                if (!path) { throw new Error('the document has no file yet; give a path such as ~/notes/a.md'); }
                await writeTo(t, path, true);
                var savedIndex = tabs.indexOf(t);
                if (savedIndex < 0) { throw new Error('The document closed while its save was running.'); }
                return { output: docView(savedIndex, false) };
            },
            'close@1': async function (input) { var i = docNo(input.index); var before = tabs.length; await closeTab(i); return { output: { closed: tabs.length < before, open: tabs.length } }; },
            'view@1': async function (input) { docNo(); if(!supportsView(current(),input.mode))throw new Error('This view is unavailable in '+(isDocs?'Docs':'Editor')+'.'); await setView(input.mode); return { output: { mode: viewMode } }; },
            'go-to@1': async function (input) {
                docNo();
                var line = Math.min(view.state.doc.lines, Math.max(1, Math.floor(Number(input.line))));
                var l = view.state.doc.line(line), col = Math.min(l.length, Math.max(1, Math.floor(Number(input.column || 1))) - 1);
                view.dispatch({ selection: { anchor: l.from + col }, scrollIntoView: true });
                view.focus();
                return { output: { line: line } };
            }
        };
        if (!isDocs) ['new-template@1','page-setup@1','header-footer@1','export@1','document-zoom@1'].forEach(function (id) {if(liveActions[id])liveActions[id]=async function () {throw new Error('This document action moved to yavc.docs/'+id+'. Open the saved document in Docs.');};});
        Object.keys(liveActions).forEach(function (id) {var handler=liveActions[id];liveActions[id]=async function (input) {await boot;if(destroyed || startupFailed || !view)throw new Error('This workspace is no longer available.');return handler(input);};});
        if (documentWindows) unbindDocuments = documentWindows.bind(root, {
            application: app, resource: function () {return current() && current().path;},
            accept: async function (request) { receivingDocument = true; try {
                if (request.operation === 'import-source') { await importedDocument(request.input.name, request.input.text,request.input.format); return {output: docView(active, false)}; }
                return await liveActions[request.operation + '@1'](request.input);
            } finally { receivingDocument = false; } }, flush: function () { return !!snap && snap.flush(); }
        });
        var detachLive = window.pudlApplets && window.pudlApplets.attach ? window.pudlApplets.attach('yavc.' + app, liveActions) : null;
        var commonToolbar = window.pudlHost && window.pudlHost.commandToolbar ? window.pudlHost.commandToolbar.attach(root, { app: app, label: isDocs ? 'Docs' : 'Editor', table: commandTable,
            defaults: ['save', '-', 'undo', 'redo', '-', 'style', 'bold', 'italic', 'link', 'insert', '-', 'find', '-', 'view-mode'] }) : null;

        return {
            state: function () { return stateString() || null; },
            setState: takeFile,
            menus: menus,
            commandTable: commandTable,
            toolbars: toolbars,
            destroy: function () {
                if (unbindDocuments) unbindDocuments();
                zoomObserver.disconnect();
                if(paneLayoutObserver)paneLayoutObserver.disconnect();cancelAnimationFrame(paneLayoutFrame);
                if (printFrame) { printFrame.remove(); }
                if (previewPages) { previewPages.destroy(); }
                var finalSync = Promise.resolve().then(function () { if (visual.api) { return visual.api.sync(); } });
                outlineRevision++; clearTimeout(outlineTimer);
                window.removeEventListener('popstate', locationChanged);
                if (commonToolbar) { commonToolbar.destroy(); }
                /* The session as the Editor closes, for when it opens again. */
                if (detachLive) { detachLive(); }
                keepSession();
                if (snap) snap.flush();
                destroyed = true;
                styleLifetime.abort();
                styleRevision++; clearTimeout(styleTimer); if (documentStyles) { documentStyles.destroy(); }
                if (unsubscribe) { unsubscribe(); }
                if (typeof unwatchPictures === 'function') { unwatchPictures(); }
                if (typeof unwatchHostCommands === 'function') { unwatchHostCommands(); }
                if (unConfig) { unConfig(); }
                unguard();
                if (explorer) { explorer.destroy(); }
                finalSync.catch(function (err) { say(err.message, true); }).finally(function () { keepSession(); if (snap) { snap.flush(); snap.leave(); } if (visual.api) { visual.api.destroy(); } if (view) { view.destroy(); } });
                if (lsp) { lsp.dispose(); }
            },
            ready: boot
        };
    }

    window.yavcTextWorkspace = Object.freeze({version: 1, init: init});
})();
