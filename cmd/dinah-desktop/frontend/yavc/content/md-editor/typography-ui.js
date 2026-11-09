/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Whole-document typography, using PUDL's editable combos and number fields. */
(function () {
    'use strict';
    var serial = 0;
    var FONTS = ['Arial', 'Aptos', 'Calibri', 'Cambria', 'Candara', 'Consolas', 'Courier New', 'Garamond', 'Georgia', 'Helvetica', 'Inter', 'Liberation Sans', 'Liberation Serif', 'Palatino Linotype', 'Roboto', 'Segoe UI', 'Tahoma', 'Times New Roman', 'Trebuchet MS', 'Verdana', 'monospace', 'sans-serif', 'serif', 'system-ui'];
    var KEYS = ['font', 'heading-font', 'size', 'spacing', 'paragraph-spacing'];
    async function open(opts) {
        if (opts.signal.aborted || !opts.current()) { return; }
        var K = window.yavcMdEditor, P = await K.pages(), D = await K.styles(), S = window.yavcDocumentSettings, esc = window.yavcUtil.esc;
        if (opts.signal.aborted || !opts.current()) { return; }
        var id = 'typography-' + (++serial), api, controller, alive = true, ready = false, raf = 0, revision = 0;
        var props = opts.properties, explicit = {}, dirty = {}, pending = {}, defaults = {}, draft = {};
        KEYS.forEach(function (key) { explicit[key] = props[key] != null && props[key] !== ''; draft[key] = explicit[key] ? props[key] : null; });
        function resetButton(key, label) { return '<button type="button" class="btn btn-sm" data-type-default="' + key + '" aria-label="Use Stylesheet Default for ' + label + '">Default</button>'; }
        function group(key, label, control) { return '<div class="form-group" data-type-group="' + key + '"><div class="md-type-label"><label class="form-label" for="' + id + '-' + key + '">' + label + '</label>' + resetButton(key, label) + '</div>' + control + '<small class="form-help" data-type-state="' + key + '"></small></div>'; }
        function font(key, label) {
            return group(key, label, '<span class="combo"><input class="form-input" autocomplete="off" id="' + id + '-' + key + '" data-type-field="' + key + '" list="' + id + '-fonts" maxlength="80"></span>');
        }
        function number(key, label, min, max, step, unit) {
            return group(key, label, '<div class="md-type-measure"><span class="number-field"><input class="form-input" type="number" id="' + id + '-' + key + '" data-type-field="' + key + '" aria-describedby="' + id + '-' + key + '-unit" min="' + min + '" max="' + max + '" step="' + step + '"></span><span aria-hidden="true">' + unit + '</span><span class="visually-hidden" id="' + id + '-' + key + '-unit">Points</span></div>');
        }
        var body = '<p class="form-help">Set the typography for the whole document.</p><div class="md-type-layout"><div class="md-type-controls">' +
            '<fieldset class="form-fieldset"><legend>Fonts</legend>' + font('font', 'Body Font') + font('heading-font', 'Heading Font') + number('size', 'Font Size', 6, 72, .5, 'pt') + '</fieldset>' +
            '<fieldset class="form-fieldset"><legend>Spacing</legend><div class="form-group"><label class="form-label" for="' + id + '-spacing">Line Spacing</label><span class="form-select-wrap"><select class="form-select" id="' + id + '-spacing" data-type-field="spacing">' +
            '<option value="">Stylesheet Default</option><option value="1">Single</option><option value="1.15">1.15 Lines</option><option value="1.5">1.5 Lines</option><option value="2">Double</option><option value="custom">Multiple</option></select></span></div>' +
            '<div class="form-group" data-type-multiple hidden><label class="form-label" for="' + id + '-multiple">Multiple</label><span class="number-field"><input class="form-input" type="number" id="' + id + '-multiple" data-type-field="multiple" min="1" max="3" step=".05"></span></div>' +
            number('paragraph-spacing', 'Space After Paragraph', 0, 72, .5, 'pt') + '</fieldset></div>' +
            '<section class="md-type-sample" aria-label="Typography Preview"><div class="md-type-label"><span>Preview</span><output data-type-summary aria-live="polite"></output></div><div class="md-type-preview-boundary"><div class="doc-page md-type-preview" data-type-preview>' +
            '<h2>A clear document</h2><p>The quick brown fox jumps over the lazy dog. Numbers 0123456789.</p><p>Good typography makes the next paragraph easy to find and read.</p></div></div>' +
            '<p class="form-help">Document styles can override these settings. Font availability depends on this computer; PDF uses Roboto.</p></section></div>' +
            '<datalist id="' + id + '-fonts">' + FONTS.map(function (name) { return '<option value="' + esc(name) + '"></option>'; }).join('') + '</datalist>' +
            '<div class="md-type-defaults"><button type="button" class="btn btn-sm" data-type-reset>Use Stylesheet Defaults</button></div><div data-type-error-ref></div>';
        function field(key) { return api.body.querySelector('[data-type-field="' + key + '"]'); }
        function visible(key) { var node = field(key); return node && node.closest('.form-group').querySelector('label').control || node; }
        function showState(key) {
            var text = api.body.querySelector('[data-type-state="' + key + '"]'), button = api.body.querySelector('[data-type-default="' + key + '"]');
            if (text) { text.textContent = explicit[key] ? 'Document Setting' : 'Stylesheet Default'; }
            if (button) { button.disabled = !explicit[key] && !pending[key]; }
        }
        function setField(key, value) {
            var node = field(key); node.value = value == null ? '' : String(value);
            if (node.type === 'number' && window.pudlFields) { window.pudlFields.refresh(node); }
            if (node.closest('.combo') && window.pudlCombo) { window.pudlCombo.refresh(node); }
        }
        function paint(key) {
            var value = explicit[key] ? draft[key] : defaults[key];
            if (key === 'spacing') { var preset = explicit[key] ? String(value) : ''; field(key).value = ['', '1', '1.15', '1.5', '2'].indexOf(preset) >= 0 ? preset : 'custom'; setField('multiple', value || defaults.spacing); api.body.querySelector('[data-type-multiple]').hidden = field(key).value !== 'custom'; }
            else {
                if (/^(size|paragraph-spacing)$/.test(key)) { try { value = Math.round(P.length(value) * 1000000) / 1000000; } catch (_) { value = defaults[key]; } }
                setField(key, value);
            }
            showState(key);
        }
        function failure(err) {
            var key = KEYS.indexOf(err.field) >= 0 ? err.field : 'font'; if (key === 'spacing' && field(key).value === 'custom') { key = 'multiple'; }
            var notice = window.yavcUtil.notice(api.body.querySelector('[data-type-error-ref]'), err.message, true), node = visible(key); notice.id = id + '-error';
            Array.from(new Set([node, field(key)])).forEach(function (target) {
                if (!target.dataset.typeError) { target.dataset.typeDescription = target.getAttribute('aria-describedby') || ''; }
                target.setAttribute('aria-invalid', 'true'); target.setAttribute('aria-describedby', notice.id); target.dataset.typeError = 'true';
            }); node.focus();
        }
        function clearIssue(key) {
            Array.from(new Set([visible(key), field(key)])).forEach(function (node) {
                if (!node || !node.dataset.typeError) { return; }
                node.removeAttribute('aria-invalid');
                if (node.dataset.typeDescription) { node.setAttribute('aria-describedby', node.dataset.typeDescription); } else { node.removeAttribute('aria-describedby'); }
                delete node.dataset.typeError; delete node.dataset.typeDescription;
            });
        }
        function read() {
            var values = {};
            KEYS.forEach(function (key) { values[key] = explicit[key] ? draft[key] : null; });
            ['size', 'paragraph-spacing', 'multiple'].forEach(function (key) {
                var node = visible(key); if (key === 'multiple' && field('spacing').value !== 'custom') { return; }
                if (node.getAttribute('aria-invalid') === 'true') { throw Object.assign(new Error('Enter a valid number.'), { field: key === 'multiple' ? 'spacing' : key }); }
            }); S.typography(values); return values;
        }
        async function draw() {
            if (!alive || !ready) { return; }
            var mine = ++revision;
            try { var values = read(); await controller.update({ identity: id, base: P.paper + ';' + S.typeCSS(Object.assign({},props,values)), inline: opts.styles || [], stylesheet: opts.stylesheet || '' });
                if (!alive || mine !== revision) { return; }
                var preview = api.body.querySelector('[data-type-preview]'), css = getComputedStyle(preview);
                if (!explicit['heading-font']) { setField('heading-font', getComputedStyle(preview.querySelector('h2')).fontFamily.split(',')[0].trim().replace(/^["']|["']$/g, '')); }
                if (!explicit['paragraph-spacing'] && !pending['paragraph-spacing']) { setField('paragraph-spacing', Math.round(parseFloat(getComputedStyle(preview.querySelector('p')).marginBottom) * .75 * 1000000) / 1000000); }
                api.body.querySelector('[data-type-summary]').textContent = (parseFloat(css.fontSize) * .75).toFixed(1).replace(/\.0$/, '') + ' pt';
            } catch (_) { /* Invalid drafts remain in the controls until corrected or Apply reports them. */ }
        }
        function schedule() { cancelAnimationFrame(raf); raf = requestAnimationFrame(draw); }
        function changed(key) {
            dirty[key] = true; pending[key] = false;
            if (key === 'spacing') {
                var value = field(key).value; explicit[key] = value !== ''; draft[key] = value === 'custom' ? field('multiple').value || String(defaults.spacing) : value || null;
                api.body.querySelector('[data-type-multiple]').hidden = value !== 'custom';
            } else {
                var value = field(key).value.trim(); explicit[key] = value !== ''; draft[key] = value === '' ? null : /^(size|paragraph-spacing)$/.test(key) ? value + 'pt' : value;
            }
            showState(key); schedule();
        }
        function reset(key) {
            explicit[key] = false; dirty[key] = true; pending[key] = false; draft[key] = null;
            clearIssue(key);
            if (key === 'spacing') { clearIssue('multiple'); }
            paint(key); schedule();
        }
        async function initialize() {
            var preview = api.body.querySelector('[data-type-preview]'), boundary = api.body.querySelector('.md-type-preview-boundary');
            controller = D.attach({ surfaces: [preview], boundaries: [boundary], base: P.paper+';'+S.themeCSS(props), read: opts.read });
            await controller.update({ identity: id, inline: opts.styles || [], stylesheet: opts.stylesheet || '' });
            if (!alive) { return; }
            if (!opts.current()) { failure(new Error('The document changed. Close and reopen Typography.')); return; }
            var css = getComputedStyle(preview), heading = getComputedStyle(preview.querySelector('h2')), paragraph = getComputedStyle(preview.querySelector('p'));
            function family(value) { return value.split(',')[0].trim().replace(/^["']|["']$/g, ''); }
            defaults = { font: family(css.fontFamily), 'heading-font': family(heading.fontFamily), size: parseFloat(css.fontSize) * .75, spacing: parseFloat(css.lineHeight) / parseFloat(css.fontSize) || 1.5, 'paragraph-spacing': parseFloat(paragraph.marginBottom) * .75 || 0 };
            api.body.querySelectorAll('input,select,button').forEach(function (node) { node.disabled = false; });
            KEYS.forEach(paint); ready = true; api.body.removeAttribute('aria-busy');
            try { S.typography(props); } catch (err) { failure(err); }
            await draw();
        }
        function abort() { if (api) { api.close('cancel'); } }
        opts.signal.addEventListener('abort', abort);
        try { await window.yavcDialog.open({ title: 'Typography', body: body, className: 'md-typography-dialog', buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'apply', label: 'Apply', primary: true }],
            onOpen: function (value) {
                api = value;
                // Validation is associated with the visible PUDL fields, including inherited fractional values.
                api.dialog.querySelector('form').noValidate = true;
                if (window.pudlCombo) { window.pudlCombo.enhance(api.body); }
                api.body.setAttribute('aria-busy', 'true'); api.body.querySelectorAll('input,select,button').forEach(function (node) { node.disabled = true; });
                api.body.addEventListener('input', function (event) {
                    var node = event.target;
                    if (node.getAttribute('role') === 'spinbutton') { var original = node.closest('.form-group').querySelector('[data-type-field]'); if (original) { clearIssue(original.dataset.typeField); pending[original.dataset.typeField] = true; showState(original.dataset.typeField); } }
                    if (node.dataset.typeField) { clearIssue(node.dataset.typeField); }
                    if (node.matches('[data-type-field="font"],[data-type-field="heading-font"]')) { changed(node.dataset.typeField); }
                });
                api.body.addEventListener('change', function (event) { if (!ready) { return; } var key = event.target.dataset.typeField; if (key === 'multiple') { changed('spacing'); } else if (key) { changed(key); } });
                api.body.querySelectorAll('[data-type-default]').forEach(function (button) { button.addEventListener('click', function () { if (ready) { reset(button.dataset.typeDefault); } }); });
                api.body.querySelector('[data-type-reset]').addEventListener('click', function () { if (ready) { KEYS.forEach(reset); } });
                initialize().catch(function (err) { if (alive) { failure(err); } });
            },
            check: function () {
                var active = document.activeElement; if (active && api.body.contains(active) && active.closest('.number-field')) { active.blur(); }
                try { if (!opts.current() || opts.signal.aborted) { throw new Error('The document changed. Close and reopen Typography.'); } if (!ready) { throw new Error('Document styles are still loading.'); }
                    var values = read(); KEYS.forEach(function (key) { if (!dirty[key]) { values[key] = props[key] || null; } }); opts.save(values, {}); return false;
                } catch (err) { failure(err); return true; }
            } });
        } finally { alive = false; revision++; cancelAnimationFrame(raf); if (controller) { controller.destroy(); } opts.signal.removeEventListener('abort', abort); }
    }
    window.yavcTypography = Object.freeze({ open: open });
})();
