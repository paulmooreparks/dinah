/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Validated document typography and running page furniture. */
(function () {
    'use strict';
    var count = 0, decorated = new WeakMap();
    var THEMES = Object.freeze([
        Object.freeze({id:'plain',title:'Plain',description:'The standard document typography.',css:''}),
        Object.freeze({id:'manuscript',title:'Manuscript',description:'Serif body and headings, 12 point text and double spacing.',css:'--doc-font:Georgia,serif;--doc-font-heading:Georgia,serif;--doc-size:12pt;--doc-leading:2;--doc-paragraph-spacing:0pt;'}),
        Object.freeze({id:'report',title:'Report',description:'Serif body text with sans-serif headings and generous paragraph spacing.',css:'--doc-font:Georgia,serif;--doc-font-heading:Arial,sans-serif;--doc-size:11pt;--doc-leading:1.45;--doc-paragraph-spacing:8pt;'}),
        Object.freeze({id:'letter',title:'Letter',description:'Compact serif text with space between paragraphs.',css:'--doc-font:Georgia,serif;--doc-font-heading:Georgia,serif;--doc-size:11pt;--doc-leading:1.3;--doc-paragraph-spacing:8pt;'})
    ]);
    function themeCSS(front) {
        var theme=THEMES.find(function(item){return item.id===(front.theme||'plain');});
        if(!theme)throw Object.assign(new Error('Choose Plain, Manuscript, Report or Letter as the document theme.'),{field:'theme'});
        return theme.css;
    }
    function properties(front) {
        if (front.problem) { throw new Error(front.problem); }
        var out = {};
        (front.entries || []).forEach(function (entry) {
            if (!/^(theme|page|orientation|margins|font|heading-font|size|spacing|paragraph-spacing|header|footer|header-first|title|author|date)$/.test(entry.key)) { return; }
            if (entry.kind === 'complex' && !(/^(header|footer)$/.test(entry.key) && entry.mapping)) { throw new Error('Page settings need plain values or header/footer alignment mappings.'); }
            out[entry.key] = entry.mapping || entry.value;
        }); return out;
    }
    function typography(front) {
        var out = {}, P = window.yavcDocumentPages;
        ['font', 'heading-font'].forEach(function (key) {
            if (front[key]) { if (!/^[a-zA-Z0-9][a-zA-Z0-9 -]{0,79}$/.test(front[key])) { throw Object.assign(new Error('Font names use letters, numbers, spaces and hyphens.'), { field: key }); } out[key] = front[key]; }
        });
        [['size', 6, 72], ['spacing', 1, 3], ['paragraph-spacing', 0, 72]].forEach(function (item) {
            if (front[item[0]] != null && front[item[0]] !== '') {
                var value;
                try { value = item[0] === 'spacing' ? Number(front[item[0]]) : P.length(front[item[0]]); }
                catch (err) { err.field = item[0]; throw err; }
                if (!Number.isFinite(value) || value < item[1] || value > item[2]) { throw Object.assign(new Error(item[0] + ' is outside its supported range.'), { field: item[0] }); }
                out[item[0]] = value;
            }
        }); return out;
    }
    function typeCSS(front) {
        var t = typography(front), css = [];
        if (t.font) { css.push('--doc-font:"' + t.font + '"'); }
        if (t['heading-font']) { css.push('--doc-font-heading:"' + t['heading-font'] + '"'); }
        if (t.size != null) { css.push('--doc-size:' + t.size + 'pt'); }
        if (t.spacing != null) { css.push('--doc-leading:' + t.spacing); }
        if (t['paragraph-spacing'] != null) { css.push('--doc-paragraph-spacing:' + t['paragraph-spacing'] + 'pt'); }
        return themeCSS(front) + css.join(';');
    }
    function furniture(props, engine) {
        var f = { first: props['header-first'] !== 'none', title: String(props.title || ''), author: String(props.author || ''), date: String(props.date || '') };
        if (props['header-first'] && !/^(none|show)$/.test(props['header-first'])) { throw new Error('First-page header is show or none.'); }
        ['header', 'footer'].forEach(function (key) {
            var value = props[key] || '', parts = typeof value === 'string' ? { center: value } : value;
            if (Object.keys(parts).some(function (align) { return !/^(left|center|right)$/.test(align); })) { throw new Error('Header/footer positions are left, center and right.'); }
            f[key] = ['left', 'center', 'right'].map(function (align) {
                try {
                var text = String(parts[align] || '');
                if (!/^(left|center|right)$/.test(align) || text.length > 256 || /[\r\n]/.test(text)) { throw new Error('Headers and footers use one line of at most 256 characters per position.'); }
                var rendered = engine.preview(text);
                var blocks = rendered.blocks || [];
                if ((rendered.problems || []).length || blocks.length > 1) { throw new Error('Use a single line of inline Markdown in each header/footer position.'); }
                var fragment = window.yavcHTML.fragment(blocks.map(function (b) { return b.html; }).join(''));
                Array.from(fragment.querySelectorAll('p,.dinah-src,.dinah-block')).reverse().forEach(function (node) { node.replaceWith.apply(node, Array.from(node.childNodes)); });
                if (fragment.querySelector('*:not(a,strong,b,em,i,u,s,del,code,span)') || fragment.querySelector('[style],[class],[data-applet]')) { throw new Error('Headers and footers support text, emphasis, code and links.'); }
                var box = document.createElement('div'); box.append(fragment);
                return { align: align, html: box.innerHTML };
                } catch (err) { err.field = key + '-' + align; throw err; }
            });
        }); return f;
    }
    function row(page, kind, number, total, doc) {
        doc = doc || document;
        var f = page.furniture, node = doc.createElement('div'); node.className = 'doc-' + kind; node.contentEditable = 'false';
        if (!f || kind === 'header' && number === 1 && !f.first) { return node; }
        (f[kind] || []).forEach(function (part) {
            var cell = doc.createElement('div'); cell.className = 'doc-furniture-cell'; cell.style.textAlign = part.align;
            cell.append(doc.importNode(window.yavcHTML.fragment(part.html), true));
            var walker = doc.createTreeWalker(cell, 4), nodes = []; while (walker.nextNode()) { nodes.push(walker.currentNode); }
            nodes.forEach(function (textNode) {
                var chunks = textNode.textContent.split(/(\{(?:page|pages|title|author|date)\})/g);
                if (chunks.length === 1) { return; }
                var fragment = doc.createDocumentFragment();
                chunks.forEach(function (text) {
                    var key = /^\{(page|pages|title|author|date)\}$/.exec(text);
                    if (!key) { fragment.append(doc.createTextNode(text)); return; }
                    var span = doc.createElement('span');
                    if (/^pages?$/.test(key[1])) { span.dataset.docField = key[1]; }
                    span.textContent = key[1] === 'page' ? number : key[1] === 'pages' ? total : f[key[1]]; fragment.append(span);
                }); textNode.replaceWith(fragment);
            }); node.append(cell);
        }); return node;
    }
    function check(page) {
        var f = page.furniture;
        if (!f) { return; }
        ['header', 'footer'].forEach(function (kind) {
            if (f[kind].some(function (part) { return part.html; }) && page.margins[kind === 'header' ? 1 : 3] < 30) { throw new Error('Headers and footers need at least 30pt in their page margin.'); }
        });
    }
    function checkRows(layer) {
        if (Array.from(layer.querySelectorAll('.doc-furniture-cell')).some(function (cell) { return cell.scrollWidth > cell.clientWidth + 1; })) { throw new Error('A header or footer is too wide. Shorten its text or change the page margins.'); }
    }
    function decorate(host, page, count) {
        var signature=JSON.stringify([page,count]),previous=decorated.get(host),current=host.querySelector(':scope > .doc-furniture');
        if(previous&&previous.signature===signature&&previous.layer===current){if(current)checkRows(current);return;}
        host.querySelectorAll(':scope > .doc-furniture').forEach(function (node) { node.remove(); });
        if (!page.furniture) { decorated.set(host,{signature:signature,layer:null});return; }
        check(page);
        var layer = document.createElement('div'); layer.className = 'doc-furniture'; layer.contentEditable = 'false';
        layer.setAttribute('aria-label', 'Page Headers and Footers');
        for (var i = 1; i <= count; i++) {
            if(i<count){var gutter=document.createElement('div');gutter.className='doc-sheet-gutter';gutter.setAttribute('aria-hidden','true');gutter.style.top=(i*page.height/.75+(i-1)*24)+'px';layer.append(gutter);}
            ['header', 'footer'].forEach(function (kind) {
                var node = row(page, kind, i, count);
                node.style.top = ((i - 1) * (page.height / .75 + 24) + (kind === 'header' ? page.margins[1] / 1.5 - 8 : (page.height - page.margins[3] / 2) / .75 - 8)) + 'px'; layer.append(node);
            });
        } host.append(layer);
        try { checkRows(layer); } catch (err) { layer.hidden = true; throw err; }
        decorated.set(host,{signature:signature,layer:layer});
    }
    async function open(kind, opts) {
        if (kind === 'typography') { var K = window.yavcMdEditor; await K.loadScript(K.here + 'typography-ui.js' + K.stamp); return window.yavcTypography.open(opts); }
        if (kind === 'theme') { var kit=window.yavcMdEditor;await kit.loadScript(kit.here+'document-gallery.js'+kit.stamp);return window.yavcDocumentGallery.theme(opts); }
        if (kind !== 'furniture') { throw new Error('Unknown document settings dialog.'); }
        var esc = window.yavcUtil.esc, api, props = opts.properties;
        var fields = ['header-left', 'header-center', 'header-right', 'footer-left', 'footer-center', 'footer-right'];
        var id = 'doc-options-' + (++count);
        var body = '<p>Use {page}, {pages}, {title}, {author} and {date}. Text, emphasis and links are supported.</p>';
        fields.forEach(function (key, index) {
            if (index % 3 === 0) { body += '<fieldset class="form-fieldset md-furniture-fields"><legend>' + (index ? 'Footer' : 'Header') + '</legend><div class="md-document-options">'; }
            var value, label;
            var parts = key.split('-'), existing = props[parts[0]] || ''; value = typeof existing === 'string' ? parts[1] === 'center' ? existing : '' : existing[parts[1]] || ''; label = parts.map(function (s) { return s[0].toUpperCase() + s.slice(1); }).join(' ');
            body += '<div class="form-group"><label class="form-label" for="' + id + '-' + key + '">' + label + '</label><input class="form-input" id="' + id + '-' + key + '" data-doc-option="' + key + '" value="' + esc(value) + '" maxlength="256"></div>';
            if (index % 3 === 2) { body += '</div></fieldset>'; }
        });
        body += '<label class="check"><input type="checkbox" data-first-header' + (props['header-first'] === 'none' ? '' : ' checked') + '> Show Header on First Page</label>';
        body += '<div data-doc-options-error-ref></div>';
        function abort() { if (api) { api.close('cancel'); } }
        if (opts.signal.aborted || !opts.current()) { return; }
        opts.signal.addEventListener('abort', abort);
        try { await window.yavcDialog.open({ title: 'Header and Footer', className: 'md-document-dialog', body: body,
            buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'apply', label: 'Apply', primary: true }],
            onOpen: function (value) { api = value; }, check: function () {
                try {
                    api.body.querySelectorAll('[data-doc-option]').forEach(function (node) { node.removeAttribute('aria-invalid'); node.removeAttribute('aria-describedby'); });
                    if (!opts.current()) { throw new Error('The document changed. Close and reopen this dialog.'); }
                    var set = {}, maps = {};
                    fields.forEach(function (key) { var value = api.body.querySelector('[data-doc-option="' + key + '"]').value.trim(), parts = key.split('-'); (maps[parts[0]] || (maps[parts[0]] = {}))[parts[1]] = value; });
                    set['header-first'] = api.body.querySelector('[data-first-header]').checked ? 'show' : 'none'; var values = Object.assign({}, props, set, maps), page = window.yavcDocumentPages.settings(values); page.furniture = furniture(values, opts.engine); check(page);
                    opts.save(set, maps); return false;
                } catch (err) { var notice = window.yavcUtil.notice(api.body.querySelector('[data-doc-options-error-ref]'), err.message, true), field = fields.indexOf(err.field) >= 0 ? err.field : fields[0], node = api.body.querySelector('[data-doc-option="' + field + '"]'); notice.id = id + '-error'; node.setAttribute('aria-invalid', 'true'); node.setAttribute('aria-describedby', notice.id); node.focus(); return true; }
            } }); } finally { opts.signal.removeEventListener('abort', abort); }
    }
    window.yavcDocumentSettings = { themes:THEMES, themeCSS:themeCSS, properties: properties, typography: typography, typeCSS: typeCSS, furniture: furniture, row: row, decorate: decorate, check: check, checkRows: checkRows, open: open };
})();
