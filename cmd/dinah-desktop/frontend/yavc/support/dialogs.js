/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* The site's questions as PUDL dialogs, one implementation for every
   application (review of 2026-11-03: there were six). PUDL's own dialog
   script handles declarative dialogs opened by a command; these are the
   ones a script asks and waits on.

     yavcDialog.open({ title, body, buttons, className, check, onOpen })  -> { value, body }
         body is safe HTML (escape anything from data with yavcUtil.esc);
         buttons: [{ id, label, primary, danger, submit }], the primary
         one is the form's submit, so Enter presses it; a danger button
         submits unless submit is false. Escape gives value null.
         check(value, body) runs on submit and keeps the dialog open when
         it returns a problem; onOpen({ dialog, body, close(value) }) runs
         once the dialog shows, to wire what the body holds.
     yavcDialog.askText(title, label, value, opts)       -> string or null
         opts: { text, okLabel, help, placeholder, signal, validate(v) -> error or null,
                 choices: [[value,label]], multiline, browse: true|'folder', accept, maxBytes }
     yavcDialog.askChoice(title, text, buttons)          -> an id or null
         buttons: [[id, label, primary, danger], ...]
     yavcDialog.confirm(title, text, opts)               -> true or false
         opts: { okLabel, danger }
     yavcDialog.askSave(name, opts)                      -> 'save', 'discard' or null
         opts: { many }: the question for several documents at once

   Each dialog is made for its question and removed after it, labels its
   title and its field, puts focus on the field or on the safe button, and
   gives focus back to where it was when it closes. Cancel comes first and
   the action last, as PUDL's dialogs order them. */
(function () {
    'use strict';
    if (window.yavcDialog) { return; }

    var count = 0;
    var esc = window.yavcUtil.esc;
    // Authored and generated dialogs share the same keyboard boundary.
    document.addEventListener('keydown', function (event) {
        if (event.key !== 'Tab' || event.defaultPrevented || event.altKey || event.ctrlKey || event.metaKey) return;
        var dialog = event.target.closest && event.target.closest('dialog.dialog[open]');
        if (!dialog) return;
        var stops = Array.from(dialog.querySelectorAll('button,input,select,textarea,a[href],[tabindex],[contenteditable="true"]')).filter(function (node) {
            return node.tabIndex >= 0 && !node.matches(':disabled') && !node.closest('[inert]') && node.getClientRects().length && getComputedStyle(node).visibility !== 'hidden';
        });
        if (!stops.length) {event.preventDefault();dialog.focus();return;}
        if (event.shiftKey && document.activeElement === stops[0]) {event.preventDefault();stops[stops.length-1].focus();}
        else if (!event.shiftKey && document.activeElement === stops[stops.length-1]) {event.preventDefault();stops[0].focus();}
    });

    function open(o) {
        var n = ++count, dlg = document.createElement('dialog'), focus = window.yavcUtil.modalFocus(dlg);
        dlg.className = 'dialog yavc-dialog' + (o.className ? ' ' + o.className : '');
        dlg.setAttribute('aria-labelledby', 'yavc-dlg-' + n + '-title');
        var buttons = (o.buttons || []).map(function (b) {
            var cls = 'btn' + (b.danger ? ' btn-danger' : b.primary ? ' btn-primary' : '');
            var glyph = b.danger ? '<span class="glyph" style="--glyph: var(--glyph-yavc-bin)" aria-hidden="true"></span>' : '';
            return b.primary || b.submit === true || (b.danger && b.submit !== false)
                ? '<button class="' + cls + '" type="submit" value="' + esc(b.id) + '">' + glyph + esc(b.label) + '</button>'
                : '<button class="' + cls + '" type="button" data-choice="' + esc(b.id) + '">' + glyph + esc(b.label) + '</button>';
        }).join('');
        /* A destructive question is PUDL's danger confirmation: the warning
           glyph in the danger colour beside its title. */
        var warn = o.warn ? '<span class="glyph yavc-dlg-warn" style="--glyph: var(--glyph-warning)" aria-hidden="true"></span>' : '';
        dlg.innerHTML = '<form method="dialog"><h3 class="dialog-title" id="yavc-dlg-' + n + '-title">' + warn + esc(o.title) + '</h3>' +
            '<div class="dialog-body">' + (o.body || '') + '</div><div class="dialog-actions">' + buttons + '</div></form>';
        document.body.appendChild(dlg);
        if (window.pudlFields && window.pudlFields.enhance) { window.pudlFields.enhance(dlg); }
        if (window.pudlDate && window.pudlDate.enhance) { window.pudlDate.enhance(dlg); }
        var body = dlg.querySelector('.dialog-body'), form = dlg.querySelector('form');
        return new Promise(function (resolve) {
            var done = false;
            function finish(value) {
                if (done) { return; }
                done = true;
                if (o.signal) { o.signal.removeEventListener('abort', abort); }
                focus.close();
                resolve({ value: value, body: body });
                setTimeout(function () { focus.restore(); dlg.remove(); }, 0);
            }
            function abort() { finish(null); }
            if (o.signal) { if (o.signal.aborted) { abort(); return; } o.signal.addEventListener('abort', abort, { once: true }); }
            dlg.addEventListener('click', function (e) { var b = e.target.closest('button[data-choice]'); if (b && dlg.contains(b)) { finish(b.getAttribute('data-choice')); } });
            form.addEventListener('submit', function (e) {
                e.preventDefault();
                var v = e.submitter ? e.submitter.value : (form.querySelector('[type="submit"]') || {}).value;
                if (o.check) { var problem = o.check(v, body); if (problem) { return; } }
                finish(v || null);
            });
            /* A dialog closed with a value (dlg.close('swatch:red')) answers it;
               Escape closes with none. */
            dlg.addEventListener('close', function () { finish(dlg.returnValue || null); });
            dlg.returnValue = '';
            dlg.showModal();
            if (o.onOpen) { o.onOpen({ dialog: dlg, body: body, close: finish }); }
            var field = body.querySelector('input:not([type="hidden"]):not([readonly]), select, textarea');
            if (field) { field.focus(); if (field.select && field.type !== 'number') { field.select(); } }
            else {
                var safe = dlg.querySelector('[data-choice="cancel"], [data-choice="no"]') || form.querySelector('[type="submit"]');
                if (safe) { safe.focus(); }
            }
        });
    }

    function pathField(o) {
        return '<div class="form-group"><label class="form-label" for="' + esc(o.id) + '">' + esc(o.label) + '</label><div class="yavc-path-field"><input class="form-input" id="' + esc(o.id) + '" data-file-path data-accept="' + esc(o.accept || '') + '"' +
            (o.maxBytes!=null?' data-max-bytes="'+esc(o.maxBytes)+'"':'') + (o.help?' aria-describedby="'+esc(o.id)+'-help"':'') + ' value="' + esc(o.value || '') + '" autocomplete="off"><button class="btn" type="button" data-browse-for="' + esc(o.id) + '"><span class="glyph" style="--glyph:var(--glyph-folder)" aria-hidden="true"></span>Browse…</button></div>' + (o.help ? '<p class="form-help" id="'+esc(o.id)+'-help">' + esc(o.help) + '</p>' : '') + '</div>';
    }
    /* A chosen file as a document writes it (docs/file-fields-contract.md).
       A path from the document's own folder goes with the document when it
       is saved somewhere else or published, and a full path does not. So a
       file that shares a folder with the document, at any depth, is written
       from the document's folder, with .. where it has to go up. A file
       that shares only the root with it, or a document with no folder yet,
       keeps the full path. */
    function referenceFrom(path, base) {
        path = String(path || ''); base = String(base || '');
        var rootOf = function (p) { return p.indexOf('~/') === 0 ? '~/' : p.charAt(0) === '/' && p.charAt(1) !== '/' ? '/' : ''; }, root = rootOf(path);
        if (!root || rootOf(base + '/') !== root) { return path; }
        var parts = function (p) { return p.slice(root.length).split('/').filter(function (part) { return part; }); }, to = parts(path), from = parts(base), same = 0;
        if (to.concat(from).some(function (part) { return part === '.' || part === '..'; })) { return path; }
        while (same < to.length - 1 && same < from.length && to[same] === from[same]) { same++; }
        if (!same) { return path; }
        return new Array(from.length - same + 1).join('../') + to.slice(same).join('/');
    }
    function wirePaths(api, opts) {
        opts = opts || {};
        var hostFields=window.pudlHost && window.pudlHost.fileFields;
        var controller = new AbortController(), capability = opts.choose || (hostFields && hostFields.version===1 && typeof hostFields.choose==='function' ? function (request) {return hostFields.choose(request);} : null);
        function abort() { controller.abort(); }
        if (opts.signal) { if (opts.signal.aborted) abort(); else opts.signal.addEventListener('abort', abort, {once:true}); }
        api.dialog.addEventListener('close', function () { abort(); if (opts.signal) opts.signal.removeEventListener('abort',abort); }, {once:true});
        api.body.querySelectorAll('[data-browse-for]').forEach(function (button) {
            if (!capability) { button.disabled = true; button.title = 'This host does not offer file browsing. Enter the path directly.'; return; }
            button.addEventListener('click', async function () {
                var input = api.body.querySelector('#' + CSS.escape(button.getAttribute('data-browse-for')));
                function current() { return api.dialog.open && !controller.signal.aborted && (!opts.current || opts.current()); }
                if (!input || !current()) return;
                button.disabled = true;
                try {
                    var limit=input.getAttribute('data-max-bytes');
                    var path = await capability({signal:controller.signal,current:current,kind:opts.browse === 'folder' ? 'folder' : 'file',accept:input.getAttribute('data-accept'),maxBytes:limit?Number(limit):undefined,start:opts.start || '~',host:opts.host});
                    if (path != null && current()) { input.value = opts.base ? referenceFrom(path, typeof opts.base === 'function' ? opts.base() : opts.base) : path; input.dispatchEvent(new Event('input',{bubbles:true})); input.focus(); }
                } catch (err) { if (current()) window.yavcUtil.notice(input.closest('.form-group'),err.message || String(err),true); }
                finally { if (button.isConnected) button.disabled = false; }
            });
        });
    }
    function askText(title, label, value, opts) {
        opts = opts || {};
        var id = 'yavc-dlg-field-' + (count + 1);
        var control = opts.choices ? '<span class="form-select-wrap"><select class="form-select" id="' + id + '"' + (opts.choices.length > 8 ? ' data-combo' : '') + '>' + opts.choices.map(function (choice) { return '<option value="' + esc(choice[0]) + '"' + (String(choice[0]) === String(value) ? ' selected' : '') + '>' + esc(choice[1]) + '</option>'; }).join('') + '</select></span>' : opts.multiline ? '<textarea class="form-textarea" id="'+id+'" rows="4">'+esc(value || '')+'</textarea>' :
            '<input class="form-input" id="' + id + '" value="' + esc(value || '') + '" autocomplete="off" spellcheck="false"' + (opts.placeholder ? ' placeholder="' + esc(opts.placeholder) + '"' : '') + '>';
        var body = (opts.text ? '<p>' + esc(opts.text) + '</p>' : '') +
            (opts.browse ? pathField({id:id,label:label,value:value,accept:opts.accept,help:opts.help,maxBytes:opts.maxBytes}) + '<p class="form-error" id="' + id + '-error" data-role="error" role="alert" hidden></p>' : '<div class="form-group"><label class="form-label" for="' + id + '">' + esc(label) + '</label>' +
            control +
            (opts.help ? '<p class="form-help" id="' + id + '-help">' + esc(opts.help) + '</p>' : '') +
            '<p class="form-error" id="' + id + '-error" data-role="error" role="alert" hidden></p></div>');
        return open({
            title: title, body: body, className: opts.className, signal: opts.signal,
            buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'ok', label: opts.okLabel || 'OK', primary: true }],
            onOpen: function (api) {
                var input = api.body.querySelector('#'+id);
                input.setAttribute('aria-describedby', [opts.help ? id + '-help' : '',id + '-error'].filter(Boolean).join(' '));
                input.addEventListener('input',function () { input.removeAttribute('aria-invalid'); api.body.querySelector('[data-role="error"]').hidden=true; });
                if (opts.browse) wirePaths(api,opts);
            },
            check: function (v, b) {
                if (v !== 'ok' || !opts.validate) { return null; }
                var input = b.querySelector('#'+id), err = b.querySelector('[data-role="error"]'), problem = opts.validate(input.value.trim());
                if (problem) { err.textContent = problem; err.hidden = false; input.setAttribute('aria-invalid', 'true'); input.focus(); }
                return problem;
            }
        }).then(function (r) { return r.value === 'ok' ? r.body.querySelector('#'+id).value.trim() : null; });
    }

    function askChoice(title, text, buttons) {
        return open({
            title: title, body: text ? '<p>' + esc(text) + '</p>' : '',
            buttons: buttons.map(function (b) { return { id: b[0], label: b[1], primary: !!b[2], danger: !!b[3] }; })
        }).then(function (r) { return r.value || null; });
    }

    function confirm(title, text, opts) {
        opts = opts || {};
        return open({
            title: title, body: text ? '<p>' + esc(text) + '</p>' : '', warn: !!opts.danger,signal:opts.signal,
            buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'ok', label: opts.okLabel || 'OK', primary: !opts.danger, danger: !!opts.danger }]
        }).then(function (r) { return r.value === 'ok'; });
    }

    /* The one "save your changes?" question: Don't Save is drawn as a
       destructive button, since it throws the changes away. */
    function askSave(name, opts) {
        opts = opts || {};
        return open({
            title: opts.many ? 'Save changes before closing?' : 'Save changes to ' + name + '?',
            body: '<p>' + esc(opts.many ? 'Some documents have changes that are not saved. They will be lost if you don\'t save them.' : 'Your changes will be lost if you don\'t save them.') + '</p>',
            buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'discard', label: 'Don\'t Save', danger: true, submit: false }, { id: 'save', label: opts.many ? 'Save All' : 'Save', primary: true }]
        }).then(function (r) { return r.value === 'save' || r.value === 'discard' ? r.value : null; });
    }

    window.yavcDialog = Object.freeze({ open: open, askText: askText, askChoice: askChoice, confirm: confirm, askSave: askSave, pathField:pathField, wirePaths:wirePaths,referenceFrom:referenceFrom });
})();
