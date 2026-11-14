/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Optional host capability for applets with PUDL command tables.
   attach(root, { app, label, defaults, table, config }) returns
   commands() to include in the existing table, and destroy().
   A command may set toolbarChoice to a PUDL choice-group name and
   toolbarPriority to its group's folding priority. Preferences still
   contain that command's ID, so the entire control can be removed. */
(function (global) {
    'use strict';
    var FILE = 'command-toolbars', SHOW = 'common-toolbar-show', EDIT = 'common-toolbar-customize', LIMIT = 60, serial = 0;
    function items(value, fallback) {
        if (!Array.isArray(value)) { value = fallback; }
        var seen = new Set(), out = [];
        value.slice(0, LIMIT).forEach(function (id) {
            if (typeof id !== 'string' || !id || id.length > 128) { return; }
            if (id === '-') { if (out.length && out[out.length - 1] !== '-') { out.push(id); } }
            else if (!seen.has(id) && id !== SHOW && id !== EDIT) { seen.add(id); out.push(id); }
        });
        if (out[out.length - 1] === '-') { out.pop(); }
        return out;
    }
    function normalize(value, defaults) {
        var v = value && typeof value === 'object' && !Array.isArray(value) && value.version === 1 ? value : {};
        return { version: 1, visible: typeof v.visible === 'boolean' ? v.visible : true,
            inRibbon: typeof v.inRibbon === 'boolean' ? v.inRibbon : true, items: items(v.items, defaults) };
    }
    function attach(root, options) {
        if (!/^[a-z][a-z0-9-]{0,63}$/.test(options.app)) { throw new Error('A toolbar needs a plain app key.'); }
        var slot = root.querySelector('[data-toolbar-slot]');
        if (!slot) { throw new Error('A command toolbar needs a toolbar slot.'); }
        var U = global.yavcUtil, config = options.config, prefs = normalize(null, options.defaults), signature = null, prefix = 'yavc-toolbar-' + (++serial) + '-';
        var destroyed = false, loading = true, saving = false, off = null, dialog = null, opening = null, focusFrame = 0;
        var bar = document.createElement('div');
        bar.className = 'toolbar yavc-command-toolbar'; bar.setAttribute('role', 'toolbar');
        bar.setAttribute('aria-label', options.label + ' Common Commands'); bar.setAttribute('data-toolbar-fold', '');
        slot.appendChild(bar);
        function changed() { if (global.pudlCommands) { global.pudlCommands.changed(); } }
        function field(command, name) { var v = command[name]; return typeof v === 'function' && name !== 'run' ? v.call(command) : v; }
        function catalog() {
            var out = new Map(), walked = new Set();
            function walk(commands, path, depth) {
                if (!Array.isArray(commands) || depth > 8) { return; }
                commands.forEach(function (c) {
                    if (!c || typeof c !== 'object' || walked.has(c)) { return; } walked.add(c);
                    var label, caption;
                    try { caption = String(field(c, 'label') || c.id || ''); label = String(field(c, 'toolbarLabel') || caption); } catch (err) { return; }
                    if (typeof c.id === 'string' && c.id !== SHOW && c.id !== EDIT && !out.has(c.id)) {
                        var choice, priority;
                        try { choice = field(c, 'toolbarChoice'); priority = field(c, 'toolbarPriority'); } catch (err) { choice = null; priority = 0; }
                        out.set(c.id, { id: c.id, label: path ? path + ' / ' + label : label, caption: caption,
                            choice: typeof choice === 'string' && /^[a-z][a-z0-9-]{0,63}$/.test(choice) ? choice : null,
                            priority: typeof priority === 'number' && Number.isFinite(priority) ? Math.max(-100, Math.min(100, priority)) : 0 });
                    }
                    try { walk(field(c, 'items'), path ? path + ' / ' + label : label, depth + 1); } catch (err) { /* A not-yet-ready submenu is retried on the next command change. */ }
                });
            }
            walk(options.table(), '', 0); return out;
        }
        function render() {
            if (destroyed) { return; }
            var known = catalog(), shown = prefs.items.filter(function (id) { return id === '-' || known.has(id); });
            var next = JSON.stringify([prefs.visible, prefs.inRibbon, shown.map(function (id) { return id === '-' ? id : [id, known.get(id).choice, known.get(id).priority]; })]);
            if (signature === next) { return; } signature = next;
            bar.hidden = !prefs.visible;
            if (prefs.inRibbon) { bar.removeAttribute('data-toolbar-ribbon'); } else { bar.setAttribute('data-toolbar-ribbon', 'hide'); }
            var ownedFocus = bar.contains(document.activeElement), focused = ownedFocus && document.activeElement.getAttribute('data-command');
            bar.replaceChildren();
            var group = null;
            shown.forEach(function (id) {
                if (id === '-') { group = null; return; }
                if (!group) {
                    group = document.createElement('div'); group.className = 'toolbar-group';
                    group.setAttribute('role', 'group'); group.setAttribute('aria-label', known.get(id).caption); group.setAttribute('data-toolbar-glyph', 'more');
                    group.setAttribute('data-toolbar-priority', known.get(id).priority);
                    bar.appendChild(group);
                }
                group.setAttribute('data-toolbar-priority', Math.max(+group.getAttribute('data-toolbar-priority') || 0, known.get(id).priority));
                if (known.get(id).choice) {
                    var segmented = document.createElement('div'); segmented.className = 'seg sm'; segmented.setAttribute('data-command-choice', known.get(id).choice); group.appendChild(segmented); return;
                }
                var button = document.createElement('button'); button.type = 'button'; button.setAttribute('data-command', id); group.appendChild(button);
            });
            var tools = document.createElement('div'); tools.className = 'toolbar-group'; tools.setAttribute('role', 'group');
            tools.setAttribute('aria-label', 'Toolbar Settings'); tools.setAttribute('data-toolbar-end', ''); tools.setAttribute('data-toolbar-priority', '100');
            tools.innerHTML = '<button type="button" data-command="' + EDIT + '" data-toolbar-glyph-only></button>'; bar.appendChild(tools);
            changed();
            if (global.pudlToolbar) { global.pudlToolbar.enhance(slot); }
            if (ownedFocus) {
                cancelAnimationFrame(focusFrame);
                focusFrame = requestAnimationFrame(function () {
                    focusFrame = 0;
                    if (destroyed || (!bar.contains(document.activeElement) && document.activeElement !== document.body)) { return; }
                    if (global.pudlToolbar) { global.pudlToolbar.fit(bar); }
                    var visible = Array.from(bar.querySelectorAll('button, input, select, a[href]')).filter(function (b) { return !b.disabled && b.getAttribute('aria-disabled') !== 'true' && b.getClientRects().length && getComputedStyle(b).visibility !== 'hidden'; });
                    var back = visible.find(function (b) { return focused && b.getAttribute('data-command') === focused; }) || visible[0];
                    (back || root).focus({ preventScroll: true });
                });
            }
        }
        function reason() { return !config || !config.load || !config.set ? 'This host does not save toolbar settings.' : loading ? 'Toolbar settings are loading.' : saving ? 'Toolbar settings are saving.' : null; }
        async function persist(next) {
            if (destroyed) { return false; }
            saving = true; changed();
            try {
                var update = {}; update[options.app] = next;
                var error = await config.set(FILE, update);
                if (error) { throw new Error(error); }
                if (destroyed) { return false; }
                prefs = normalize(next, options.defaults); render(); return true;
            } finally { saving = false; if (!destroyed) { changed(); } }
        }
        async function toggle() {
            var next = Object.assign({}, prefs, { visible: !prefs.visible });
            try { await persist(next); }
            catch (err) { if (!destroyed) { U.toast('Toolbar settings were not saved: ' + err.message, true); } }
        }
        function customize() {
            if (opening) { return opening; }
            var draft = { version: 1, items: prefs.items.slice(), visible: prefs.visible, inRibbon: prefs.inRibbon }, entries = catalog();
            var ui = null, busy = false;
            function lists(index) {
                var available = ui.body.querySelector('[data-available]'), selected = ui.body.querySelector('[data-selected]'), query = ui.body.querySelector('[data-search]').value.toLowerCase();
                var prior = available.value;
                available.innerHTML = Array.from(entries.values()).filter(function (c) { return draft.items.indexOf(c.id) < 0 && (c.label + ' ' + c.id).toLowerCase().indexOf(query) >= 0; })
                    .sort(function (a, b) { return a.label.localeCompare(b.label); }).map(function (c) { return '<option value="' + U.esc(c.id) + '">' + U.esc(c.label) + '</option>'; }).join('');
                if (Array.from(available.options).some(function (o) { return o.value === prior; })) { available.value = prior; }
                else { available.selectedIndex = available.options.length ? 0 : -1; }
                selected.innerHTML = draft.items.map(function (id, i) { return '<option value="' + i + '">' + U.esc(id === '-' ? 'Separator' : entries.has(id) ? entries.get(id).label : id + ' (unavailable)') + '</option>'; }).join('');
                selected.selectedIndex = draft.items.length ? Math.max(0, Math.min(index == null ? 0 : index, draft.items.length - 1)) : -1;
                buttons();
            }
            function buttons() {
                var available = ui.body.querySelector('[data-available]'), selected = ui.body.querySelector('[data-selected]');
                var i = selected.selectedIndex, count = draft.items.length, has = !!available.value;
                ui.body.querySelector('[data-available-caption]').textContent = available.options[available.selectedIndex] ? available.options[available.selectedIndex].text : 'No available commands';
                ui.body.querySelector('[data-selected-caption]').textContent = selected.options[i] ? selected.options[i].text : 'Toolbar is empty';
                ['add', 'separator', 'remove', 'up', 'down'].forEach(function (op) {
                    ui.body.querySelector('[data-edit="' + op + '"]').disabled = busy || (op === 'add' ? !has || count >= LIMIT : op === 'separator' ? count >= LIMIT : i < 0 || (op === 'up' ? i === 0 : op === 'down' ? i === count - 1 : false));
                });
            }
            function apply() {
                if (busy || destroyed) { return; } busy = true;
                var error = ui.body.querySelector('[data-error]'); error.hidden = true;
                draft.visible = ui.body.querySelector('[data-visible]').checked; draft.inRibbon = ui.body.querySelector('[data-ribbon]').checked;
                ui.dialog.querySelectorAll('button').forEach(function (b) { b.disabled = true; }); ui.body.querySelector('fieldset').disabled = true;
                persist(normalize(draft, options.defaults)).then(function (ok) { if (ok && !destroyed) { ui.close('apply'); } }).catch(function (err) {
                    if (!destroyed) { error.textContent = 'Toolbar settings were not saved: ' + err.message; error.hidden = false; }
                }).finally(function () {
                    busy = false;
                    if (!destroyed && ui.dialog.open) { ui.dialog.querySelectorAll('button').forEach(function (b) { b.disabled = false; }); ui.body.querySelector('fieldset').disabled = false; buttons(); }
                });
            }
            opening = global.yavcDialog.open({ title: 'Customize ' + options.label + ' Toolbar', className: 'yavc-toolbar-dialog',
                body: '<fieldset class="yavc-toolbar-fields"><div class="form-group"><label class="form-label" for="' + prefix + 'search">Search Available Commands</label><input class="form-input" id="' + prefix + 'search" type="search" data-search></div>' +
                    '<div class="yavc-toolbar-columns"><label class="form-label yavc-toolbar-available-label" for="' + prefix + 'available">Available Commands</label><label class="form-label yavc-toolbar-selected-label" for="' + prefix + 'selected">Toolbar Commands</label>' +
                    '<select class="form-input" id="' + prefix + 'available" size="10" data-available></select>' +
                    '<div class="yavc-toolbar-transfer" role="group" aria-label="Transfer Commands"><button type="button" class="btn" data-edit="add" aria-label="Add to Toolbar" title="Add the selected available command to the toolbar">&gt;</button>' +
                    '<button type="button" class="btn" data-edit="remove" aria-label="Remove from Toolbar" title="Remove the selected toolbar command">&lt;</button></div>' +
                    '<select class="form-input" id="' + prefix + 'selected" size="10" data-selected></select>' +
                    '<p class="form-help yavc-toolbar-caption" data-available-caption aria-hidden="true"></p><p class="form-help yavc-toolbar-caption" data-selected-caption aria-hidden="true"></p>' +
                    '<div class="hstack yavc-toolbar-order" role="group" aria-label="Toolbar Order"><button type="button" class="btn btn-sm" data-edit="up"><span class="glyph" data-glyph="arrow-up" aria-hidden="true"></span>Move Up</button>' +
                    '<button type="button" class="btn btn-sm" data-edit="down"><span class="glyph" data-glyph="arrow-down" aria-hidden="true"></span>Move Down</button><button type="button" class="btn btn-sm" data-edit="separator">Add Separator</button></div></div>' +
                    '<label class="check"><input type="checkbox" data-visible' + (draft.visible ? ' checked' : '') + '> Show the Toolbar</label><label class="check"><input type="checkbox" data-ribbon' + (draft.inRibbon ? ' checked' : '') + '> Keep Visible with the Ribbon</label><button type="button" class="btn btn-sm" data-edit="reset">Restore Defaults</button></fieldset><p class="form-error" data-error role="alert" hidden></p>',
                buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'apply', label: 'Apply', primary: true }],
                check: function (value) { if (value === 'apply') { apply(); return true; } },
                onOpen: function (context) {
                    ui = context; dialog = context; lists(0);
                    ui.dialog.addEventListener('cancel', function (e) { if (busy) { e.preventDefault(); } });
                    ui.body.addEventListener('input', function (e) { if (e.target.hasAttribute('data-search')) { lists(ui.body.querySelector('[data-selected]').selectedIndex); } });
                    ui.body.addEventListener('change', buttons);
                    ui.body.addEventListener('click', function (e) {
                        var b = e.target.closest('[data-edit]'); if (!b || b.disabled || busy) { return; }
                        var op = b.dataset.edit, i = ui.body.querySelector('[data-selected]').selectedIndex;
                        if (op === 'add' || op === 'separator') { draft.items.splice(i + 1, 0, op === 'add' ? ui.body.querySelector('[data-available]').value : '-'); i++; }
                        else if (op === 'remove') { draft.items.splice(i, 1); }
                        else if (op === 'up' || op === 'down') { var next = i + (op === 'up' ? -1 : 1), id = draft.items.splice(i, 1)[0]; draft.items.splice(next, 0, id); i = next; }
                        else if (op === 'reset') { draft = normalize(null, options.defaults); ui.body.querySelector('[data-visible]').checked = draft.visible; ui.body.querySelector('[data-ribbon]').checked = draft.inRibbon; i = 0; }
                        lists(i);
                    });
                }
            }).finally(function () {
                opening = null; dialog = null;
                setTimeout(function () { if (!destroyed && document.activeElement === document.body && !document.querySelector('dialog[open]')) { root.focus({ preventScroll: true }); } }, 0);
            });
            return opening;
        }
        function update(all) { if (!destroyed) { prefs = normalize(all && all[options.app], options.defaults); render(); changed(); } }
        document.addEventListener('pudl:commands-change', render);
        render();
        if (config && config.load) {
            config.load(FILE, {}).then(function (all) {
                if (destroyed) { return; } update(all);
                if (config.onChange) { off = config.onChange(FILE, update); }
            }).catch(function (err) { if (!destroyed) { U.notice(slot, 'Toolbar settings could not load: ' + err.message, true); } })
                .finally(function () { loading = false; if (!destroyed) { changed(); } });
        } else { loading = false; }
        return { commands: function () { return [
            { id: SHOW, label: 'Show the Toolbar', glyph: 'panel', checked: function () { return prefs.visible; }, disabled: function () { return !!reason(); }, reason: reason, run: toggle },
            { id: EDIT, label: 'Customize Toolbar…', glyph: 'pencil', disabled: function () { return !!reason(); }, reason: reason, run: customize }
        ]; }, destroy: function () { destroyed = true; cancelAnimationFrame(focusFrame); if (off) { off(); } document.removeEventListener('pudl:commands-change', render); if (dialog) { dialog.close('cancel'); } bar.remove(); } };
    }
    var api = { attach: attach, normalize: normalize };
    if (typeof module === 'object' && module.exports) { module.exports = api; } else { global.yavcCommandToolbar = api; }
})(typeof window === 'undefined' ? null : window);
