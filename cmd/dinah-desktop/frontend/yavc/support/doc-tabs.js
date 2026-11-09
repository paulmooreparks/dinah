/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Document tabs for the site's applications, the Editor and Slides: a
   PUDL .tablist.doc-tabs (pudl-tabs.js gives it the keyboard and fires
   pudl:tab-close) drawn from the host's list of open documents. The row
   shows only while more than one document is open (Paul, 2026-11-03).

     var row = yavcDocTabs.attach(list, {
       application: 'Docs',
       tabs: function () { return [{ name, path, modified, note }]; },
       active: function () { return index; },
       activate: function (i) { … },
       close: function (i) { return promise; }   asks about unsaved changes itself
     });
     row.render();   after any change to the documents

   Load with defer, after pudl-tabs.js and before the applets. */
(function () {
    'use strict';
    if (window.yavcDocTabs) { return; }

    var esc = window.yavcUtil.esc;

    function attach(list, host) {
        var win = list.closest('.win[data-win]'), rendered = null;
        function renderTitle(tab) {
            if (!list.isConnected || !win || !win.contains(list) || !host.application || !window.pudlWindows) { return; }
            var name = tab && tab.name || '', dirty = !!(tab && tab.modified);
            var title = (dirty ? '\u25cf ' : '') + (name ? name + ' \u00b7 ' : '') + host.application;
            var heading = win.querySelector('.win-title');
            if (!heading) { return; }
            if (heading.textContent !== title) { window.pudlWindows.retitle(win.getAttribute('data-win'), title); }
            heading.setAttribute('aria-label', (name ? name + ' \u00b7 ' : '') + host.application + (dirty ? ', Unsaved changes' : ''));
            heading.title = (tab && (tab.path || tab.name) || host.application) + (dirty ? ', Unsaved changes' : '');
        }
        function render() {
            var tabs = host.tabs() || [], active = host.active();
            renderTitle(tabs[active]);
            list.hidden = tabs.length <= 1;
            var had = list.contains(document.activeElement) ? +document.activeElement.getAttribute('data-tab') : -1;
            var markup = tabs.map(function (t, i) {
                var on = i === active;
                return '<button type="button" role="tab" data-tab="' + i + '" aria-selected="' + on + '" tabindex="' + (on ? '0' : '-1') + '" title="' + esc(t.path || t.name) + '">' +
                    esc(t.name) +
                    (t.note ? '<span class="doc-tab-note">' + esc(t.note) + '</span>' : '') +
                    (t.modified ? '<span class="doc-tab-dirty">unsaved</span>' : '') +
                    '<span class="doc-tab-close" aria-hidden="true" title="Close"></span></button>';
            }).join('');
            if (markup === rendered) { return; }
            rendered = markup;
            list.innerHTML = markup;
            if (had >= 0) {
                var b = list.querySelector('[data-tab="' + Math.min(had, tabs.length - 1) + '"]');
                if (b) { b.focus(); }
            }
        }
        list.addEventListener('click', function (e) {
            if (e.target.closest('.doc-tab-close')) { return; }
            var b = e.target.closest('[data-tab]');
            if (b) { host.activate(+b.getAttribute('data-tab')); }
        });
        /* A right-click on a tab: close it, or every other one. */
        list.addEventListener('pudl:contextmenu', function (e) {
            var b = e.detail.target.closest && e.detail.target.closest('[data-tab]');
            if (!b) { return; }
            var i = +b.getAttribute('data-tab'), n = (host.tabs() || []).length;
            e.detail.mark(b);
            e.detail.add([
                { label: 'Close', glyph: 'close', run: function () { host.close(i); } },
                { label: 'Close the Others', glyph: 'close-all', disabled: n < 2, run: function () {
                    /* From the last down, so the numbers of those still to close hold. */
                    (async function () { for (var k = n - 1; k >= 0; k--) { if (k !== i) { await host.close(k); } } })();
                } }
            ]);
        });
        list.addEventListener('pudl:tab-close', function (e) {
            var fromList = list.contains(document.activeElement);
            Promise.resolve(host.close(+e.target.getAttribute('data-tab'))).then(function () {
                var b = list.querySelector('[aria-selected="true"]');
                if (fromList && b) { b.focus(); }
            });
        });
        return { render: render };
    }

    /* Unsaved documents, guarded: leaving the page asks the browser's own
       question, and closing the application's window asks the site's (one
       question for every application, static/dialogs.js askSave), unless
       the changes are kept elsewhere and come back.

         var off = yavcDocTabs.guard(root, {
           changed: function () { return [names of unsaved documents]; },
           keptOnReload: function () { return true when a reload keeps them; },
           keptOnClose: function () { return true when closing the window keeps them; },
           saveAll: function () { return a promise of true when every one saved; }
         });
         off();   when the application goes */
    function guard(root, o) {
        var win = root.closest('.win[data-win]'), agreed = false;
        function unsaved() { return (o.changed() || []).length > 0; }
        function onBeforeUnload(e) {
            if (unsaved() && !(o.keptOnReload && o.keptOnReload())) { e.preventDefault(); e.returnValue = ''; }
        }
        function onClosing(e) {
            if (e.target !== win || agreed) { return; }
            var names = o.changed() || [];
            if (!names.length || (o.keptOnClose && o.keptOnClose())) { return; }
            e.preventDefault();
            var key = win.getAttribute('data-win');
            window.yavcDialog.askSave(names[0], { many: names.length > 1 }).then(function (choice) {
                if (!choice) { return null; }
                return choice === 'save' ? Promise.resolve(o.saveAll()) : true;
            }).then(function (go) {
                if (!go) { return; }
                agreed = true;
                window.pudlWindows.close(key);
            });
        }
        window.addEventListener('beforeunload', onBeforeUnload);
        if (win) { win.addEventListener('pudl:window-closing', onClosing); }
        return function () {
            window.removeEventListener('beforeunload', onBeforeUnload);
            if (win) { win.removeEventListener('pudl:window-closing', onClosing); }
        };
    }

    window.yavcDocTabs = Object.freeze({ attach: attach, guard: guard });
})();
