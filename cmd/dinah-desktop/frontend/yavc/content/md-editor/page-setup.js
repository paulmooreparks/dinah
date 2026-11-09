/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Shared page setup. Draft dimensions stay in points while display units change. */
(function () {
    'use strict';
    var count = 0, units = { in: 72, mm: 72 / 25.4, cm: 72 / 2.54, pt: 1 };
    var presets = [
        { id: 'normal', label: 'Normal (1 in)', values: [72, 72, 72, 72] }, // check-ignore(title-case-labels): in is the lowercase inch unit, not a title word.
        { id: 'narrow', label: 'Narrow (0.5 in)', values: [36, 36, 36, 36] }, // check-ignore(title-case-labels): in is the lowercase inch unit, not a title word.
        { id: 'moderate', label: 'Moderate (1 in Top/Bottom, 0.75 in Sides)', values: [72, 54, 72, 54] },
        { id: 'wide', label: 'Wide (1 in Top/Bottom, 2 in Sides)', values: [72, 144, 72, 144] },
        { id: 'none', label: 'No Margins', values: [0, 0, 0, 0] }
    ];
    function same(values) { return values.every(function (n) { return Math.abs(n - values[0]) < 1e-7; }); }
    function fmt(value) { return String(+value.toFixed(9)); }
    async function open(opts) {
        if (!opts.current() || opts.signal.aborted) { return; }
        var P = window.yavcDocumentPages, esc = window.yavcUtil.esc, id = 'page-setup-' + (++count), api = null;
        var source = opts.properties, stock = P.lookup(source.page), initialProblem = '', initial;
        try { initial = P.settings(source); }
        catch (err) { initialProblem = err.message; try { initial = P.settings({ page: source.page, orientation: source.orientation, margins: '0' }); } catch (_) { initial = P.settings({}); } }
        var draft = { width: initial.width, height: initial.height, margins: [initial.margins[1], initial.margins[2], initial.margins[3], initial.margins[0]] };
        var authoredUnit = /(in|mm|cm|pt)\b/.exec(source.page || source.margins || '');
        var unit = stock ? stock.unit : authoredUnit ? authoredUnit[1] : 'mm', linked = same(draft.margins), paperId = stock ? stock.id : 'custom', rendered = {};
        function field(key, label) { return '<div class="form-group"><label class="form-label" for="' + id + '-' + key + '">' + label + '</label><div class="md-page-measure"><span class="number-field"><input class="form-input" type="number" step="any" min="0" id="' + id + '-' + key + '" data-measure="' + key + '"></span><span data-unit-label aria-hidden="true"></span></div></div>'; }
        function choice(key, label, options) { return '<div class="form-group"><label class="form-label" for="' + id + '-' + key + '">' + label + '</label><span class="form-select-wrap"><select class="form-select" id="' + id + '-' + key + '" data-choice="' + key + '">' + options + '</select></span></div>'; }
        function option(value, label) { return '<option value="' + esc(value) + '">' + esc(label) + '</option>'; }
        function control(key) { return api.body.querySelector('[data-measure="' + key + '"]'); }
        function visible(node) { return node.closest('.form-group').querySelector('label').control || node; }
        function select(key) { return api.body.querySelector('[data-choice="' + key + '"]'); }
        function commitActive() { var node = document.activeElement; if (node && api.body.contains(node) && node.closest('.number-field')) { node.blur(); } }
        function number(key) {
            var node = control(key), n = node.value.trim() === '' ? NaN : Number(node.value);
            if ((visible(node).getAttribute('aria-invalid') === 'true' && node.dataset.pageError !== 'layout') || !Number.isFinite(n) || n < 0 || ((key === 'width' || key === 'height') && n === 0)) { throw Object.assign(new Error('Enter a valid ' + (key === 'all' ? 'margin' : key) + ' in ' + unit + '.'), { field: key, numeric: true }); }
            if (rendered[key] && node.value === rendered[key].text) { return rendered[key].points; }
            return n * units[unit];
        }
        function margins() { return linked ? Array(4).fill(number('all')) : ['top','right','bottom','left'].map(number); }
        function capture() {
            draft.width = number('width'); draft.height = number('height');
            draft.margins = margins();
        }
        function error(err) {
            var notice = window.yavcUtil.notice(api.body, err.message, true), key = err.field || 'width';
            if (key === 'margins') { key = linked ? 'all' : 'top'; }
            var node = control(key) || select(key) || select('paper');
            var target = node.matches('[data-measure]') ? visible(node) : node;
            if (notice) { notice.id = id + '-error'; node.setAttribute('aria-describedby', notice.id); target.setAttribute('aria-describedby', notice.id); }
            node.dataset.pageError = err.numeric ? 'number' : 'layout'; node.setAttribute('aria-invalid', 'true'); target.setAttribute('aria-invalid', 'true'); target.focus();
        }
        function presetId() { var match = presets.find(function (p) { return p.values.every(function (n, i) { return Math.abs(n - draft.margins[i]) < 1e-7; }); }); return match ? match.id : 'custom'; }
        function render() {
            ['width','height'].forEach(function (key) { control(key).value = fmt(draft[key] / units[unit]); });
            ['top','right','bottom','left'].forEach(function (key, i) { control(key).value = fmt(draft.margins[i] / units[unit]); control(key).disabled = linked; });
            control('all').value = fmt(draft.margins[0] / units[unit]);
            control('all').closest('.form-group').hidden = !linked;
            api.body.querySelector('[data-link-margins]').checked = linked;
            select('paper').value = paperId; select('units').value = unit; select('preset').value = presetId();
            select('orientation').value = draft.width > draft.height ? 'landscape' : 'portrait';
            api.body.querySelectorAll('[data-unit-label]').forEach(function (node) { node.textContent = unit; });
            api.body.querySelectorAll('[data-measure]').forEach(function (node) {
                var key = node.dataset.measure;
                rendered[key] = { text: node.value, points: key === 'width' || key === 'height' ? draft[key] : draft.margins[key === 'all' ? 0 : ['top','right','bottom','left'].indexOf(key)] };
                node.removeAttribute('aria-invalid'); node.removeAttribute('aria-describedby'); delete node.dataset.pageError;
                node.setAttribute('aria-label', node.closest('.form-group').querySelector('label').textContent + ' (' + unit + ')');
                if (window.pudlFields) { window.pudlFields.refresh(node); }
            });
        }
        function papers(filter) {
            var groups = new Map(), favorites = ['letter','a4','legal','tabloid','a3','a5'];
            window.yavcPaperSizes.filter(function (item) { return !item.variant && (!filter || (item.label + ' ' + item.group + ' ' + item.pwg).toLowerCase().includes(filter.toLowerCase())); }).forEach(function (item) {
                var group = favorites.indexOf(item.id) >= 0 ? 'Common' : item.group;
                if (!groups.has(group)) { groups.set(group, []); } groups.get(group).push(item);
            });
            var html = option('custom', 'Custom Size');
            Array.from(groups.keys()).sort(function (a, b) { return a === 'Common' ? -1 : b === 'Common' ? 1 : a.localeCompare(b); }).forEach(function (group) {
                html += '<optgroup label="' + esc(group) + '">' + groups.get(group).map(function (item) { return option(item.id, item.label + ' (' + item.width + ' × ' + item.height + ' ' + item.unit + ')'); }).join('') + '</optgroup>';
            });
            if (paperId !== 'custom' && !Array.from(groups.values()).some(function (items) { return items.some(function (item) { return item.id === paperId; }); })) {
                var current = P.lookup(paperId); if (current) { html += option(current.id, current.label + ' (Current)'); }
            }
            select('paper').innerHTML = html; select('paper').value = paperId;
        }
        function abort() { if (api) { api.close('cancel'); } }
        try {
            return await window.yavcDialog.open({ title: 'Page Setup', className: 'md-page-setup', body:
                '<div class="md-page-setup-units">' + choice('units','Units',option('in','Inches') + option('mm','Millimeters') + option('cm','Centimeters') + option('pt','Points')) + '</div>' +
                '<div class="md-page-setup-grid"><fieldset class="form-fieldset md-page-settings"><legend>Paper</legend>' +
                '<div class="form-group"><label class="form-label" for="' + id + '-filter">Find a Paper Size</label><input class="form-input" type="search" id="' + id + '-filter" data-paper-filter placeholder="For example, A4, Letter or JIS"></div>' +
                choice('paper','Paper Size','') + '<div class="md-page-pair">' + field('width','Width') + field('height','Height') + '</div>' +
                choice('orientation','Orientation',option('portrait','Portrait') + option('landscape','Landscape')) +
                '<p class="form-help">Choose a standard size, or enter a custom width and height. Changing a dimension selects Custom Size.</p></fieldset>' +
                '<fieldset class="form-fieldset md-page-settings"><legend>Margins</legend>' + choice('preset','Preset',presets.map(function (p) { return option(p.id,p.label); }).join('') + option('custom','Custom Margins')) +
                '<label class="check"><input type="checkbox" data-link-margins>Use the Same Margin on All Sides</label>' + field('all','All Sides') +
                '<div class="md-page-pair">' + field('top','Top') + field('bottom','Bottom') + field('left','Left') + field('right','Right') + '</div></fieldset></div>',
                buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'apply', label: 'Apply', primary: true }],
                onOpen: function (opened) {
                    api = opened; api.dialog.querySelector('form').noValidate = true;
                    opts.signal.addEventListener('abort', abort, { once: true }); papers(''); render();
                    api.body.querySelector('[data-paper-filter]').addEventListener('input', function (event) { papers(event.target.value); });
                    select('paper').addEventListener('change', function () {
                        commitActive();
                        var next = select('paper').value;
                        try { draft.margins = margins(); } catch (err) { select('paper').value = paperId; error(err); return; }
                        paperId = next;
                        if (paperId === 'custom') { visible(control('width')).focus(); return; }
                        var item = P.lookup(paperId), dims = [P.length(item.width + item.unit), P.length(item.height + item.unit)].sort(function (a,b) { return a-b; });
                        if (select('orientation').value === 'landscape') { dims.reverse(); } draft.width = dims[0]; draft.height = dims[1]; render();
                    });
                    select('orientation').addEventListener('change', function () {
                        commitActive();
                        try { capture(); var w = draft.width, h = draft.height, landscape = select('orientation').value === 'landscape'; draft.width = landscape ? Math.max(w,h) : Math.min(w,h); draft.height = landscape ? Math.min(w,h) : Math.max(w,h); render(); } catch (err) { error(err); }
                    });
                    select('units').addEventListener('change', function () { commitActive(); var next = select('units').value; try { capture(); unit = next; render(); } catch (err) { select('units').value = unit; error(err); } });
                    select('preset').addEventListener('change', function () {
                        commitActive();
                        try { draft.width = number('width'); draft.height = number('height'); } catch (err) { select('preset').value = presetId(); error(err); return; }
                        var preset = presets.find(function (p) { return p.id === select('preset').value; });
                        if (preset) { draft.margins = preset.values.slice(); linked = same(draft.margins); render(); }
                    });
                    api.body.querySelector('[data-link-margins]').addEventListener('change', function (event) { try { capture(); linked = event.target.checked; if (linked) { draft.margins.fill(draft.margins[0]); } render(); } catch (err) { event.target.checked = linked; error(err); } });
                    api.body.querySelectorAll('[data-measure]').forEach(function (node) { node.addEventListener('input', function () {
                        var key = node.dataset.measure;
                        if (key === 'width' || key === 'height') {
                            paperId = 'custom'; select('paper').value = paperId;
                            try { draft[key] = number(key); select('orientation').value = number('width') > number('height') ? 'landscape' : 'portrait'; } catch (_) { }
                        } else {
                            select('preset').value = 'custom';
                            if (key === 'all') { ['top','right','bottom','left'].forEach(function (side) { control(side).value = node.value; }); }
                            try { if (key === 'all') { draft.margins.fill(number(key)); } else { draft.margins[['top','right','bottom','left'].indexOf(key)] = number(key); } } catch (_) { }
                        }
                    }); });
                    api.body.addEventListener('input', function (event) {
                        var group = event.target.closest('.form-group'), node = group && group.querySelector('[data-measure]');
                        if (node && node.dataset.pageError) { node.removeAttribute('aria-invalid'); node.removeAttribute('aria-describedby'); visible(node).removeAttribute('aria-invalid'); visible(node).removeAttribute('aria-describedby'); delete node.dataset.pageError; }
                    }, true);
                    if (initialProblem) { window.yavcUtil.notice(api.body, initialProblem + ' Correct the settings before applying.', true); }
                },
                check: function () {
                    commitActive();
                    try {
                        if (!opts.current() || opts.signal.aborted) { throw new Error('The document changed. Close and reopen Page Setup.'); }
                        capture();
                        var orientation = select('orientation').value, dims = [draft.width,draft.height].sort(function (a,b) { return a-b; });
                        // Dimensions are physical; orientation follows their order after a manual edit.
                        if (paperId === 'custom') { orientation = draft.width > draft.height ? 'landscape' : 'portrait'; }
                        var values = { page: paperId === 'custom' ? fmt(dims[0] / units[unit]) + unit + ' x ' + fmt(dims[1] / units[unit]) + unit : paperId,
                            orientation: orientation, margins: (same(draft.margins) ? [draft.margins[0]] : draft.margins).map(function (n) { return fmt(n / units[unit]) + unit; }).join(' ') };
                        P.settings(values); opts.save(values); return false;
                    } catch (err) { error(err); return true; }
                }
            });
        } finally { opts.signal.removeEventListener('abort', abort); }
    }
    window.yavcPageSetup = Object.freeze({ open: open });
})();
