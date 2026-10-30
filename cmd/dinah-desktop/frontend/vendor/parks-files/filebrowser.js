/* The file browser: the folder tree, the path bar and the list of a
   folder's contents, over js/sitefs.js (Architecture/files-editor-design.md).
   Files, the Editor's Open and Save as dialog, and the Editor's Explorer
   pane all use this one module, on the public site and the edit origin
   alike, so browsing works the same everywhere. It shows and moves about;
   what opening an entry means is its host's business.

     var b = pcFileBrowser.create(element, F, options);

   element holds the parts to fill, written with pcFileBrowser.markup:
   pathBar() and body({ tree, list, drop, hint }). Any part may be left out;
   the Explorer pane is a tree alone. Options, all optional:

     start        the folder to open at
     linkFor(n)   a folder's own address, for its links (default: #)
     addressOf(n) any other entry's address, for its row's link
     treeFiles    the tree shows files as well as folders
     toggleFolders  a click on a folder in the tree opens or closes it
                  instead of going there (the Explorer pane)
     navigate     false disables user navigation and sends folders to onOpen
     navigationRoot  limits navigation to this absolute folder and descendants
     parentEntry  shows a Parent Folder row when navigation upward is allowed
     draggable    entries in ~ can be dragged onto folders there to move them
     onUpload(files)  files dropped from the computer onto the list, in ~
     onOpen(node) an entry is opened (includes folders when navigate is false)
     onSelect(node)  the selection in the list changed (node may be null)
     onNavigate(dir) the current folder changed
     onRender()   after every redraw
     onKey(e, node)  a key on a row the browser doesn't handle; true if taken
     say(msg, isError)  a message for the host to show
     filter(node) true for the files to show; folders always show
     showHidden   show entries whose names start with a dot, such as
                  ~/.config, which are hidden otherwise, as in a shell
     expand       paths of folders to open in the tree at the start
     treeWidth    the tree's width in px, for a host that keeps it
     onResize(width)  the tree was resized; width is null when reset
     formatDate(d)  the text of an entry's date in the list (default: the
                  browser's own short date)

   The browser follows every change to the filesystem by itself.

   pcFileBrowser.pick(F, options) is the Open and Save as dialog built on
   it, which the Editor and the barcode tool share; its comment says what
   it takes and gives back. */
/* Dinah adaptation: asynchronous lookup/loading, paged branches and
   optional home root. See NOTICE.txt beside this file for source provenance. */
(function () {
    'use strict';
    if (window.pcFileBrowser) { return; }

    /* Its stylesheet comes with it, from the address js/applets.js names. */
    if (!document.querySelector('link[data-fb-css]')) {
        var css = document.createElement('link');
        css.rel = 'stylesheet';
        css.href = window.pcFileBrowserCss || '/css/filebrowser.css';
        css.setAttribute('data-fb-css', '');
        document.head.appendChild(css);
    }

    function esc(s) { return String(s == null ? '' : s).replace(/&/g, '&amp;').replace(/</g, '&lt;').replace(/>/g, '&gt;').replace(/"/g, '&quot;'); }
    function fmtDate(d) { return d && !isNaN(d) ? d.toLocaleDateString(undefined, { day: 'numeric', month: 'short', year: 'numeric' }) : ''; }
    function fmtSize(n) { return n == null ? '' : n < 1024 ? n + ' B' : n < 1024 * 1024 ? (n / 1024).toFixed(1) + ' KB' : (n / 1024 / 1024).toFixed(1) + ' MB'; }

    /* PUDL's glyph for each kind of entry. The shape says the kind, which
       the list also names in words in its own column. */
    var GLYPHS = { dir: 'folder', page: 'document', app: 'app', script: 'script', link: 'link', file: 'file', home: 'home', download: 'download', command: 'script' };
    var KIND_NAMES = { dir: 'Folder', page: 'Page', app: 'Applet', script: 'Script', link: 'Link', file: 'File', command: 'Command' };
    function glyph(kind) {
        return '<span class="glyph" style="--glyph: var(--glyph-' + (GLYPHS[kind] || 'file') + ')" aria-hidden="true"></span>';
    }

    var markup = {
        pathBar: function () { return '<nav class="path mono fm-path" data-fb="crumbs" aria-label="Location"></nav>'; },
        body: function (o) {
            o = o || {};
            var tree = o.tree !== false, list = o.list !== false;
            /* With both, PUDL's splitter divides them (pudl-split.js). */
            var both = tree && list;
            return '<div class="fm-body' + (both ? ' split' : '') + (list ? '' : ' fm-body-tree') + (tree ? '' : ' fm-body-list') + '">' +
                (tree ? '<div class="fm-tree-pane' + (both ? ' split-pane' : '') + '" data-fb="tree-pane"><ul class="tree fm-tree" data-fb="tree" aria-label="' + esc(o.treeLabel || 'Folders') + '"></ul></div>' : '') +
                (both ? '<div class="split-handle fm-split" data-fb="split" aria-label="Resize the folders" data-split-min="140" data-split-max="60%"></div>' : '') +
                (list
                    ? '<div class="fm-main' + (both ? ' split-pane' : '') + (o.drop ? ' drop-zone' : '') + '" data-fb="main">' +
                        '<table class="data-table fm-list" data-fb="list" role="grid" aria-label="Contents">' +
                          '<thead><tr><th scope="col" class="fm-c-name">Name</th><th scope="col" class="fm-c-kind">Kind</th><th scope="col" class="fm-c-date">Date</th><th scope="col" class="fm-c-size num">Size</th></tr></thead>' +
                          '<tbody data-fb="rows"></tbody>' +
                        '</table>' +
                        '<p class="fm-empty" data-fb="empty" hidden></p>' +
                        (o.drop ? '<p class="drop-hint">' + esc(o.hint || 'Drop files here to upload them') + '</p>' : '') +
                      '</div>'
                    : '') +
                '</div>';
        }
    };

    function create(root, F, o) {
        o = o || {};
        var q = function (sel) { return root.querySelector(sel); };
        var el = { crumbs: q('[data-fb="crumbs"]'), tree: q('[data-fb="tree"]'), list: q('[data-fb="list"]'), rows: q('[data-fb="rows"]'), empty: q('[data-fb="empty"]'), main: q('[data-fb="main"]') };

        /* The divider between the tree and the list is PUDL's; the host
           keeps the width, which is set on the split as --split-a. */
        var splitHandle = q('[data-fb="split"]'), splitter = null;
        if (splitHandle) {
            splitter = splitOf(splitHandle, o.onResize);
            splitter.set(o.treeWidth);
        }
        var cwd = null, selected = null, expanded = {}, currentPath = null, destroyed = false, navigation = 0;
        var changed = function () { };
        var say = o.say || function () { };
        var linkFor = o.linkFor || function () { return '#'; };
        var navigationRoot = o.navigationRoot && o.navigationRoot.replace(/\/$/, '') || '';
        function allowed(path) {
            if (!navigationRoot) return true;
            if (path.split('/').some(function (part) { return part === '..' || part === '.'; })) return false;
            return path === navigationRoot || path.indexOf(navigationRoot + '/') === 0;
        }
        function canUp() { return !!(cwd && cwd.parent && allowed(F.pathOf(cwd.parent))); }
        function entry(name) { return name === '..' && o.parentEntry && canUp() ? cwd.parent : F.childNamed(cwd, name); }

        function inHome(node) { return !!F.realOf(node).home; }

        /* === Moving about ================================================ */

        async function go(node, keepStatus) {
            if (!node || !node.children || !allowed(F.pathOf(node))) { return; }
            var request = ++navigation;
            if (o.onLoading) { o.onLoading(true); }
            try { if (F.load) { await F.load(node); } }
            catch (error) { if (!destroyed && request === navigation) { say(String(error), true); if (o.onLoading) { o.onLoading(false); } } return; }
            if (destroyed || request !== navigation || !allowed(F.pathOf(node))) { return; }
            cwd = node;
            selected = null;
            for (var x = node; x; x = x.parent) { expanded[F.pathOf(x)] = true; }
            if (!keepStatus) { say(''); }
            render();
            if (o.onNavigate) { o.onNavigate(cwd); }
            if (o.onLoading) { o.onLoading(false); }
            changed();
        }

        async function goPath(path) {
            if (!allowed(path)) return false;
            var request = ++navigation, node;
            try { node = F.lookup ? await F.lookup(path) : F.resolve(null, path); }
            catch (error) { if (!destroyed && request === navigation) { say(String(error), true); } return false; }
            if (destroyed || request !== navigation) { return false; }
            if (node && node.children && allowed(F.pathOf(node))) { await go(node); return true; }
            say(path + ': no such folder', true);
            return false;
        }

        /* After a change, the current folder and the selection are found
           again by path, since a reload replaces the nodes it covers. */
        function refresh() {
            if (!cwd) { return; }
            var path = F.pathOf(cwd), sel = selected ? selected.name : null, previous = selected;
            var node = F.resolve(null, path);
            while (!node && path !== '/' && path !== navigationRoot) { path = path.slice(0, path.lastIndexOf('/')) || '/'; node = F.resolve(null, path); }
            if (navigationRoot && (!node || !allowed(F.pathOf(node)))) { say('The starting folder is unavailable.', true); return; }
            cwd = node || F.root();
            selected = sel ? F.childNamed(cwd, sel) : null;
            if (previous && selected && F.sameNode && !F.sameNode(previous, selected)) { selected = null; }
            render();
        }

        /* === Drawing ===================================================== */

        function render() {
            if (el.crumbs) { renderCrumbs(); }
            if (el.tree) { renderTree(); }
            if (el.rows) { renderList(); }
            if (o.onRender) { o.onRender(); }
        }

        /* PUDL's path bar: the folders above this one, each an address. */
        function renderCrumbs() {
            var chain = [];
            for (var x = cwd; x; x = x.parent) { chain.unshift(x); if (F.pathOf(x) === navigationRoot) break; }
            var h = chain.indexOf(F.home()), start = h >= 0 ? h : 0, html = '';
            for (var i = start; i < chain.length; i++) {
                var c = chain[i], label = i === start && h >= 0 ? '~' : (c.parent ? c.name : '/');
                html += '<li>' + (i === chain.length - 1 ? '<span aria-current="location">' + esc(label) + '</span>'
                    : '<a href="' + esc(linkFor(c)) + '" data-path="' + esc(F.pathOf(c)) + '">' + esc(label) + '</a>') + '</li>';
            }
            el.crumbs.innerHTML = '<ol>' + html + '</ol>';
        }

        /* PUDL's tree (pudl-tree.js): nested lists of links. A closed
           folder's branch is drawn the first time it opens. The tree marks
           the current folder, or with treeFiles the current file, which the
           host names with markCurrent. */
        function visible(c) { return o.showHidden || c.name.charAt(0) !== '.'; }
        function kidsOf(node) {
            var kids = (node.children || []).filter(visible);
            var dirs = kids.filter(function (c) { return c.children; });
            return o.treeFiles ? dirs.concat(kids.filter(shownFile)) : dirs;
        }
        function treeNode(node, seen) {
            var path = F.pathOf(node), kids = kidsOf(node);
            if (!node.children) {
                var kind = F.kindOf(node);
                return '<li><a href="' + esc((o.addressOf && o.addressOf(node)) || '#') + '" data-path="' + esc(path) + '" data-file' +
                    (path === currentPath ? ' aria-current="location"' : '') + '>' + glyph(kind) + '<span class="fm-ti-label">' + esc(node.name) + '</span></a></li>';
            }
            var open = !!expanded[path];
            /* ~ shows twice, as its own root and under /home; only its
               first appearance is marked. */
            var current = !seen[path] && (currentPath != null ? path === currentPath : node === cwd);
            seen[path] = true;
            var label = !node.parent ? '/' : node === F.home() ? 'Home (~)' : (node.title || node.name);
            var expandable = kids.length || node.loaded === false || node.nextStart >= 0;
            return '<li><a href="' + esc(linkFor(node)) + '" data-path="' + esc(path) + '"' +
                (expandable ? ' aria-expanded="' + open + '"' : '') + (current ? ' aria-current="location"' : '') + '>' +
                glyph(node === F.home() ? 'home' : 'dir') + '<span class="fm-ti-label">' + esc(label) + '</span></a>' +
                (expandable && open ? '<ul>' + kids.map(function (d) { return treeNode(d, seen); }).join('') +
                    (node.nextStart >= 0 ? '<li><button type="button" class="btn btn-sm" data-more-path="' + esc(path) + '">Load More</button></li>' : '') + '</ul>' : '') + '</li>';
        }
        function branch(kids, seen) { return '<ul>' + kids.map(function (d) { return treeNode(d, seen); }).join('') + '</ul>'; }

        function renderTree() {
            var focusPath = document.activeElement && el.tree.contains(document.activeElement) ? document.activeElement.getAttribute('data-path') : null;
            var seen = {};
            /* The home directory is a root of its own, first, since it is
               where the changes happen; the site follows. */
            var boundary = navigationRoot && F.resolve(null, navigationRoot);
            el.tree.innerHTML = navigationRoot ? (boundary ? treeNode(boundary, seen) : '') : (F.home() ? treeNode(F.home(), seen) : '') + treeNode(F.root(), seen);
            if (window.pudlTree) { window.pudlTree.enhance(el.tree); }
            if (focusPath) {
                var f = el.tree.querySelector('a[data-path="' + CSS.escape(focusPath) + '"]');
                if (f) { el.tree.querySelectorAll('a[tabindex="0"]').forEach(function (a) { a.tabIndex = -1; }); f.tabIndex = 0; f.focus(); }
            }
        }

        /* The tree drawn on its own, after a branch opens or the current
           entry changes; the host is told as after any other redraw, since
           what it adds to the tree, such as Dinah's web glyph, is
           lost with the old markup. */
        function drawTree() {
            renderTree();
            if (o.onRender) { o.onRender(); }
        }

        async function onToggle(e) {
            var a = e.target, path = a.getAttribute('data-path');
            expanded[path] = !!(e.detail && e.detail.open);
            changed();
            var drawn = a.nextElementSibling && a.nextElementSibling.tagName === 'UL';
            if (!expanded[path]) { return; }
            var node = F.resolve(null, path);
            if (F.load && node) {
                try { await F.load(node); } catch (error) { if (!destroyed) { say(String(error), true); } return; }
                if (!destroyed) { drawTree(); }
                return;
            }
            if (drawn) { return; }
            a.insertAdjacentHTML('afterend', branch(node ? kidsOf(node) : [], {}));
            window.pudlTree.enhance(el.tree);
            if (o.onRender) { o.onRender(); }
        }

        function entries() {
            var kids = (F.listEntries ? F.listEntries(cwd) : cwd.children || []).filter(visible);
            /* Folders first; within each, the order the site gives, which
               for articles is newest first. */
            return kids.filter(function (c) { return c.children; }).concat(kids.filter(shownFile));
        }
        /* A host may show only some files, such as a dialog that opens
           layouts; folders always show, to move about in. */
        function shownFile(c) { return !c.children && (!o.filter || o.filter(c)); }

        /* PUDL's grid (pudl-grid.js): the rows are choices, selection
           follows focus, and Enter or a double-click opens a row. A row
           whose entry has an address of its own links to it; the rest open
           through pudl:row-open. */
        function renderList() {
            var list = entries(), html = '';
            if (o.parentEntry && canUp()) list.unshift(Object.assign({}, cwd.parent, {name:'..',title:'Parent Folder',special:true}));
            list.forEach(function (c) {
                var r = F.realOf(c), kind = F.kindOf(c);
                var date = r.home ? r.mtime : r.date;
                var sel = selected === c;
                var href = o.rowLinks === false ? null : kind === 'dir' ? linkFor(c) : (o.addressOf ? o.addressOf(c) : null);
                var name = esc(r.title || c.name) + (kind === 'dir' ? '/' : '');
                html += '<tr class="fm-row" data-name="' + esc(c.name) + '" aria-selected="' + sel + '" tabindex="' + (sel || (!selected && c === list[0]) ? '0' : '-1') + '"' +
                    (o.draggable && inHome(c) && !r.special ? ' draggable="true"' : '') + '>' +
                    '<td class="fm-c-name"><span class="fm-cell">' + glyph(kind) +
                    (href ? '<a class="fm-name" href="' + esc(href) + '">' + name + '</a>' : '<span class="fm-name">' + name + '</span>') +
                    (r.title && r.title !== c.name && !r.home && !r.contentFacet ? '<span class="fm-title">' + esc(c.name) + '</span>' : '') + '</span></td>' +
                    '<td class="fm-c-kind">' + KIND_NAMES[kind] + '</td>' +
                    '<td class="fm-c-date">' + esc(date && !isNaN(date) && o.formatDate ? o.formatDate(date) : fmtDate(date)) + '</td>' +
                    '<td class="fm-c-size num">' + esc(fmtSize(F.size(c))) + '</td></tr>';
            });
            el.rows.innerHTML = html;
            if (window.pudlGrid) { window.pudlGrid.enhance(el.list); }
            el.empty.hidden = list.length > 0;
            el.empty.textContent = o.emptyText ? o.emptyText(cwd) : 'This folder is empty.';
        }

        /* === Selection =================================================== */

        function rowFor(name) { return el.rows ? el.rows.querySelector('tr[data-name="' + CSS.escape(name) + '"]') : null; }
        function select(name, focus) {
            selected = name ? entry(name) : null;
            if (el.rows) {
                el.rows.querySelectorAll('tr').forEach(function (tr) {
                    var on = tr.getAttribute('data-name') === name;
                    tr.setAttribute('aria-selected', String(on));
                    tr.tabIndex = on ? 0 : -1;
                });
            }
            if (o.onSelect) { o.onSelect(selected); }
            if (focus !== false && name) { var r = rowFor(name); if (r) { r.focus(); } }
            changed();
        }
        function focusList() {
            if (!el.rows) { return; }
            var r = el.rows.querySelector('tr[tabindex="0"]') || el.rows.querySelector('tr');
            if (r) { r.focus(); }
        }

        /* Names the entry the host is showing, such as the Editor's open
           file, for the tree to mark and open its way to. */
        function markCurrent(path) {
            currentPath = path || null;
            if (currentPath) {
                var node = F.resolve(null, currentPath);
                for (var x = node && node.parent; x; x = x.parent) { expanded[F.pathOf(x)] = true; }
            }
            if (el.tree) { drawTree(); }
        }

        function open(node) {
            if (!node) { return; }
            if (F.kindOf(node) === 'dir' && o.navigate !== false) { go(node); focusList(); return; }
            if (o.onOpen) { o.onOpen(node); }
        }

        /* === Clicks and keys ============================================= */

        function onClick(e) {
            var more = e.target.closest('[data-more-path]');
            if (more && F.load) {
                e.preventDefault(); more.disabled = true;
                Promise.resolve(F.load(F.resolve(null, more.dataset.morePath), true)).then(function () { if (!destroyed) { render(); } }).catch(function (error) { if (!destroyed) { more.disabled = false; say(String(error), true); } });
                return;
            }
            /* Links in the tree, the path bar and the rows are addresses of
               their own, which a modified click opens as the browser would;
               a plain one is handled here. The tree's toggles are
               pudl-tree.js's. */
            var link = e.target.closest('a[href]');
            if (!link || !root.contains(link) || e.target.closest('.tree-toggle')) { return; }
            if (e.ctrlKey || e.metaKey || e.shiftKey || e.button !== 0) { return; }
            if (link.hasAttribute('data-path')) {
                e.preventDefault();
                var path = link.getAttribute('data-path');
                if (link.hasAttribute('data-file')) { open(F.resolve(null, path)); return; }
                /* In the Explorer pane a folder opens and closes where it is. */
                if (o.toggleFolders && el.tree && el.tree.contains(link)) {
                    var t = link.querySelector('.tree-toggle');
                    if (t) { t.click(); }
                    return;
                }
                /* Enter on a folder in the tree moves on to its contents;
                   asked before going there, which redraws the tree. */
                var fromTree = e.detail === 0 && el.tree && el.tree.contains(link);
                if (o.navigate === false) { if (o.onOpen) { o.onOpen(F.resolve(null, path)); } return; }
                goPath(path);
                if (fromTree) { focusList(); }
                return;
            }
            var tr = link.closest('tr.fm-row');
            if (tr) {
                e.preventDefault();
                select(tr.getAttribute('data-name'), false);
                open(selected);
            }
        }

        function onRowSelect(e) { selected = entry(e.target.getAttribute('data-name')); if (o.onSelect) { o.onSelect(selected); } changed(); }
        function onRowOpen(e) { var c = entry(e.target.getAttribute('data-name')); if (c) { selected = c; open(c); } }
        function onRowKey(e) {
            if (e.target.tagName !== 'TR') { return; }
            if (e.key === 'Backspace') { if (o.navigate !== false) { up(); } e.preventDefault(); return; }
            if (o.onKey && o.onKey(e, selected)) { e.preventDefault(); }
        }

        function up() { if (canUp()) { var from = cwd; go(cwd.parent).then(function () { if (!destroyed && cwd === from.parent) { select(from.name); } }); } }

        /* === Dragging ==================================================== */

        var dragName = null;
        function clearDropMarks() {
            root.querySelectorAll('[data-drop-target]').forEach(function (x) { x.removeAttribute('data-drop-target'); });
            if (el.main) { el.main.removeAttribute('data-drop-over'); }
        }
        /* A folder in ~ (a row or a tree node) takes a dragged entry; the
           list itself, in ~, takes files dropped from the computer. */
        function folderTarget(e) {
            var t = e.target.closest('tr.fm-row, .fm-tree a[data-path]');
            if (!t) { return null; }
            var node = t.hasAttribute('data-path') ? F.resolve(null, t.getAttribute('data-path')) : F.childNamed(cwd, t.getAttribute('data-name'));
            return node && node.children && inHome(node) ? { el: t, node: node } : null;
        }
        function isFiles(e) { return e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types || [], 'Files') >= 0; }
        function onDragStart(e) {
            var tr = e.target.closest('tr.fm-row');
            if (!tr || !o.draggable) { return; }
            dragName = tr.getAttribute('data-name');
            e.dataTransfer.setData('application/x-pc-files', F.pathOf(F.childNamed(cwd, dragName)));
            e.dataTransfer.effectAllowed = 'move';
        }
        function onDragEnd() { dragName = null; clearDropMarks(); }
        function onDragOver(e) {
            if (dragName) {
                var t = folderTarget(e);
                clearDropMarks();
                if (t && F.pathOf(t.node) !== F.pathOf(F.childNamed(cwd, dragName) || cwd)) { t.el.setAttribute('data-drop-target', ''); e.preventDefault(); e.dataTransfer.dropEffect = 'move'; }
                return;
            }
            if (o.onUpload && isFiles(e) && inHome(cwd) && el.main && el.main.contains(e.target)) { el.main.setAttribute('data-drop-over', ''); e.preventDefault(); e.dataTransfer.dropEffect = 'copy'; }
        }
        function onDragLeave(e) { if (!root.contains(e.relatedTarget)) { clearDropMarks(); } }
        async function onDrop(e) {
            if (dragName) {
                var t = folderTarget(e), moving = F.childNamed(cwd, dragName);
                clearDropMarks();
                if (!t || !moving) { return; }
                e.preventDefault();
                var err = await F.move(cwd, moving, F.displayPath(t.node));
                dragName = null;
                if (err) { say(err, true); } else { refresh(); say('Moved ' + moving.name + ' to ' + F.displayPath(t.node) + '.'); }
                return;
            }
            if (o.onUpload && isFiles(e) && inHome(cwd)) {
                e.preventDefault();
                clearDropMarks();
                o.onUpload(Array.prototype.slice.call(e.dataTransfer.files || []));
            }
        }

        root.addEventListener('click', onClick);
        root.addEventListener('dragover', onDragOver);
        root.addEventListener('dragleave', onDragLeave);
        root.addEventListener('drop', onDrop);
        if (el.tree) { el.tree.addEventListener('pudl:tree-toggle', onToggle); }
        if (el.rows) {
            el.rows.addEventListener('pudl:row-select', onRowSelect);
            el.rows.addEventListener('pudl:row-open', onRowOpen);
            el.rows.addEventListener('keydown', onRowKey);
            el.rows.addEventListener('dragstart', onDragStart);
            el.rows.addEventListener('dragend', onDragEnd);
        }
        var unsubscribe = F.onChange(function () { if (!destroyed) { refresh(); } });
        function onScroll() { changed(); }
        ['tree', 'list', 'main'].forEach(function (k) { if (el[k]) { el[k].addEventListener('scroll', onScroll, { passive: true }); } });

        /* === Start ======================================================= */

        var start = o.start ? F.resolve(null, o.start) : null;
        cwd = start && start.children ? start : F.root();
        if (navigationRoot && !allowed(F.pathOf(cwd))) throw new Error('The starting folder is outside the navigation root.');
        for (var x = cwd; x; x = x.parent) { expanded[F.pathOf(x)] = true; }
        if (F.home()) { expanded[F.pathOf(F.home())] = true; }
        (o.expand || []).forEach(function (p) { var n = F.resolve(null, p); if (n && n.children) { expanded[F.pathOf(n)] = true; } });
        render();

        return {
            cwd: function () { return cwd; },
            selected: function () { return selected; },
            go: go,
            goPath: goPath,
            up: up,
            canUp: canUp,
            refresh: refresh,
            render: render,
            select: select,
            focusList: focusList,
            markCurrent: markCurrent,
            /* Shows or hides the entries whose names start with a dot. */
            showHidden: function (on) { o.showHidden = !!on; render(); },
            /* Sets the tree's width, as when another window changed it. */
            treeWidth: function (w) { if (splitter) { splitter.set(w); } },
            /* What a reload loses, for the host's snapshot (js/snapshot.js):
               the folder, for a host whose address doesn't keep it, the
               selection, the open folders of the tree, and how far the tree
               and list are scrolled. restore() takes back all but the
               folder, which the host starts in, as far as it still fits: a
               folder or file gone since is let go. */
            snapshot: function () {
                return {
                    cwd: cwd ? F.pathOf(cwd) : null,
                    selected: selected ? selected.name : null,
                    expanded: Object.keys(expanded).filter(function (p) { return expanded[p]; }),
                    scroll: ['tree', 'list', 'main'].map(function (k) { return el[k] ? el[k].scrollTop : 0; })
                };
            },
            restore: async function (s) {
                if (!s || typeof s !== 'object') { return; }
                if (F.lookup && F.load) {
                    for (var path of (s.expanded || []).slice(0, 100)) {
                        if (destroyed) { return; }
                        if (!allowed(path)) continue;
                        try { var node = await F.lookup(path); if (node.children) { await F.load(node); } } catch (error) { /* Missing branches remain closed. */ }
                    }
                }
                if (destroyed) { return; }
                (s.expanded || []).forEach(function (p) { var n = F.resolve(null, p); if (n && n.children) { expanded[F.pathOf(n)] = true; } });
                render();
                if (s.selected && F.childNamed(cwd, s.selected)) { select(s.selected, false); }
                var scroll = s.scroll || [];
                ['tree', 'list', 'main'].forEach(function (k, i) { if (el[k] && scroll[i]) { el[k].scrollTop = scroll[i]; } });
            },
            /* Says when something a snapshot holds changes. */
            onChange: function (fn) { changed = fn; },
            destroy: function () {
                destroyed = true;
                if (unsubscribe) { unsubscribe(); }
                root.removeEventListener('click', onClick);
                root.removeEventListener('dragover', onDragOver);
                root.removeEventListener('dragleave', onDragLeave);
                root.removeEventListener('drop', onDrop);
            }
        };
    }

    /* === Open and Save as ================================================

       The browser in a dialog, with a field under it for the path (Open)
       or the name (Save as), and beside the buttons a way to use the
       reader's computer instead. It only chooses: the host opens or writes.

         pcFileBrowser.pick(F, {
           mode       'open' or 'save'
           title, okLabel
           host       the element to put the dialog in (default: the body)
           start      the folder to start in (default ~)
           name       Save as: the name to suggest
           current    Save as: the file being saved, which needs no
                      question before it is replaced
           accept     extensions, such as ['.json']: Open lists and takes
                      only those files, and the computer's chooser too
           check(node)  Open: why this file won't do, or null
           canSaveIn(dir)  Save as: why nothing can be saved in this
                      folder, or null; while it answers, Save is off and
                      Download instead is the main button
           computer   true for "From your computer…" (Open) or "Download
                      instead" (Save as)
         })

       resolves to one of
         { path, node }        Open: the file chosen
         { path, dir, name }   Save as: where to write (the dialog has
                               already asked about replacing a file)
         { file }              Open: a File from the reader's computer
         { download: name }    Save as: download it under this name
         null                  cancelled */
    var pickCount = 0;
    function pick(F, o) {
        o = o || {};
        var save = o.mode === 'save', id = 'fb-pick-' + (++pickCount);
        var accept = (o.accept || []).map(function (x) { return x.toLowerCase(); });
        var fits = function (name) { return !accept.length || accept.some(function (x) { return name.toLowerCase().slice(-x.length) === x; }); };
        var dlg = document.createElement('dialog');
        dlg.className = 'dialog fb-picker';
        dlg.setAttribute('aria-labelledby', id + '-title');
        dlg.innerHTML =
            '<form method="dialog" class="fb-pick-form">' +
              '<h3 class="dialog-title" id="' + id + '-title">' + esc(o.title || (save ? 'Save as' : 'Open')) + '</h3>' +
              '<div class="dialog-body fb-pick-body">' +
                '<div class="fb-pick-bar">' +
                  '<button type="button" class="icon-btn" data-pick="up" aria-label="Up one folder" title="Up one folder"><svg viewBox="0 0 16 16" aria-hidden="true" fill="none" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M8 13V3.5"/><path d="M4 7.5l4-4 4 4"/></svg></button>' +
                  '<button type="button" class="icon-btn" data-pick="home" aria-label="Your home directory" title="Your home directory">' + glyph('home') + '</button>' +
                  markup.pathBar() +
                '</div>' +
                '<div class="fb-pick-browse">' + markup.body({}) + '</div>' +
                '<div class="form-group fb-pick-field">' +
                  '<label class="form-label" for="' + id + '-input">' + (save ? 'File name' : 'Path') + '</label>' +
                  '<input class="form-input mono" id="' + id + '-input" data-pick="input" autocomplete="off" spellcheck="false"' +
                    (save ? '' : ' placeholder="Choose a file, or type a path such as ~/README"') + ' />' +
                '</div>' +
                '<p class="form-error" data-pick="error" role="alert" hidden></p>' +
              '</div>' +
              '<div class="dialog-actions fb-pick-actions">' +
                (o.computer ? (save
                    ? '<button type="button" class="btn" data-pick="download">' + glyph('download') + 'Download instead</button>'
                    : '<button type="button" class="btn" data-pick="computer">From your computer…</button>' +
                      '<input type="file" data-pick="chooser" hidden' + (accept.length ? ' accept="' + esc(accept.join(',')) + '"' : '') + ' />') : '') +
                '<span class="fb-pick-spacer"></span>' +
                '<button type="button" class="btn" data-pick="cancel">Cancel</button>' +
                '<button type="submit" class="btn btn-primary" data-pick="ok">' + esc(o.okLabel || (save ? 'Save' : 'Open')) + '</button>' +
              '</div>' +
            '</form>';
        (o.host || document.body).appendChild(dlg);
        var q = function (s) { return dlg.querySelector('[data-pick="' + s + '"]'); };
        var input = q('input'), error = q('error'), ok = q('ok'), dl = q('download');
        var okLabel = ok.textContent, replacing = null, done = null, b = null;

        function say(msg) { error.textContent = msg || ''; error.hidden = !msg; }
        function realName(v) { return v.indexOf('/') >= 0 ? v.slice(v.lastIndexOf('/') + 1) : v; }
        function resetReplace() { replacing = null; ok.textContent = okLabel; }

        /* Save as says at once when the folder shown can't take the file,
           and makes downloading the way forward. */
        function folderState() {
            if (!save || !b) { return; }
            var why = o.canSaveIn ? o.canSaveIn(F.realOf(b.cwd())) : null;
            ok.disabled = !!why;
            if (dl) { dl.classList.toggle('btn-primary', !!why); }
            say(why);
        }

        function finish(value) {
            if (!done) { return; }
            var d = done;
            done = null;
            if (dlg.open) { dlg.close(); }
            if (b) { b.destroy(); }
            dlg.remove();
            d(value);
        }

        function submit() {
            var v = input.value.trim(), dir = b.cwd();
            if (!save) {
                var sel = b.selected();
                var node = v ? F.resolve(dir, v) : (sel && !sel.children ? sel : null);
                if (!node) { say(v ? 'There\'s nothing at ' + v + '.' : 'Choose a file.'); return; }
                if (node.children) { b.go(node); input.value = ''; return; }
                var r = F.realOf(node);
                var why = r.kind === 'link' || r.kind === 'app' ? node.name + ' isn\'t a file that can be opened here.'
                    : !fits(r.name) ? node.name + ' isn\'t a ' + accept.join(' or ') + ' file.'
                    : o.check ? o.check(r) : null;
                if (why) { say(why); return; }
                finish({ path: F.displayPath(r), node: r });
                return;
            }
            if (!v) { say('Give it a name.'); return; }
            var target = dir, base = v;
            if (v.indexOf('/') >= 0) {
                var cut = v.lastIndexOf('/');
                target = F.resolve(dir, v.slice(0, cut) || '/');
                base = v.slice(cut + 1);
                if (!target || !target.children) { say('There\'s no folder at ' + v.slice(0, cut) + '.'); return; }
            }
            target = F.realOf(target);
            if (!target.home) { say('Files can only be saved in your home directory (~)' + (F.mounted ? ', /wwwroot or /etc' : '') + '.'); return; }
            var cannot = o.canSaveIn ? o.canSaveIn(target) : null;
            if (cannot) { say(cannot); return; }
            if (!F.validName(base)) { say(base + ' isn\'t a valid file name.'); return; }
            var existing = F.childNamed(target, base);
            if (existing && existing.children) { say(base + ' is a folder.'); return; }
            var path = F.displayPath(target).replace(/\/$/, '') + '/' + base;
            /* Replacing a file takes a second press, on a button that says
               so; any change to the name asks again. */
            if (existing && path !== o.current && replacing !== path) {
                replacing = path;
                ok.textContent = 'Replace';
                say(base + ' already exists here. Replace it?');
                return;
            }
            finish({ path: path, dir: target, name: base });
        }

        return new Promise(function (resolve) {
            done = resolve;
            b = create(dlg, F, {
                start: o.start || F.displayPath(F.home()),
                filter: accept.length ? function (n) { return fits(n.name); } : null,
                onOpen: function (node) {
                    input.value = save ? node.name : F.displayPath(F.realOf(node));
                    resetReplace();
                    submit();
                },
                onSelect: function (node) {
                    if (!node || node.children) { return; }
                    input.value = save ? node.name : F.displayPath(F.realOf(node));
                    resetReplace();
                    say('');
                },
                onNavigate: function () { resetReplace(); folderState(); },
                say: function (m) { if (m) { say(m); } }
            });
            if (save) { input.value = o.name || ''; }
            dlg.addEventListener('submit', function (e) { e.preventDefault(); submit(); });
            dlg.addEventListener('close', function () { finish(null); });
            input.addEventListener('input', function () { resetReplace(); if (!ok.disabled) { say(''); } });
            dlg.addEventListener('click', function (e) {
                var t = e.target.closest('[data-pick]');
                if (!t) { return; }
                switch (t.getAttribute('data-pick')) {
                    case 'cancel': finish(null); break;
                    case 'up': b.up(); break;
                    case 'home': b.go(F.home()); break;
                    case 'computer': q('chooser').value = ''; q('chooser').click(); break;
                    case 'download': {
                        var name = realName(input.value.trim());
                        if (!name) { say('Give it a name.'); input.focus(); break; }
                        finish({ download: name });
                        break;
                    }
                }
            });
            if (o.computer && !save) {
                q('chooser').addEventListener('change', function () {
                    var f = this.files && this.files[0];
                    if (f) { finish({ file: f }); }
                });
                /* A file dropped from the computer anywhere on the dialog
                   is the same as choosing it. */
                dlg.addEventListener('dragover', function (e) {
                    if (e.dataTransfer && Array.prototype.indexOf.call(e.dataTransfer.types || [], 'Files') >= 0) { e.preventDefault(); e.dataTransfer.dropEffect = 'copy'; }
                });
                dlg.addEventListener('drop', function (e) {
                    var f = e.dataTransfer && e.dataTransfer.files && e.dataTransfer.files[0];
                    if (!f) { return; }
                    e.preventDefault();
                    if (!fits(f.name)) { say(f.name + ' isn\'t a ' + accept.join(' or ') + ' file.'); return; }
                    finish({ file: f });
                });
            }
            dlg.showModal();
            folderState();
            if (save) { input.focus(); var dot = input.value.lastIndexOf('.'); input.setSelectionRange(0, dot > 0 ? dot : input.value.length); }
            else { b.focusList(); }
        });
    }

    /* === Splitters =========================================================

       The file browser and the Editor divide their panes with PUDL's
       splitter (pudl-split.js, 0.31.0), which keeps no state. This is the
       little a host needs on top: a stored width set on the split, and a
       callback when the reader changes it, with null after a reset.

         pcSplit.of(handle, onChange) -> { set(width or null) } */
    function splitOf(handle, onChange) {
        var split = handle.parentElement;
        if (onChange) {
            handle.addEventListener('pudl:split', function (e) { onChange(e.detail ? e.detail.size : null); });
        }
        function refresh() { if (window.pudlSplit) { window.pudlSplit.refresh(); } }
        /* The markup is made by script after PUDL's first look, so it is
           asked to look again once the handle is in the document. */
        setTimeout(refresh, 0);
        return {
            set: function (w) {
                if (typeof w === 'number') { split.style.setProperty('--split-a', Math.round(w) + 'px'); }
                else { split.style.removeProperty('--split-a'); }
                refresh();
            }
        };
    }
    window.pcSplit = { of: splitOf };

    window.pcFileBrowser = {
        create: create,
        pick: pick,
        markup: markup,
        glyph: glyph,
        kindName: function (kind) { return KIND_NAMES[kind]; },
        esc: esc,
        fmtDate: fmtDate,
        fmtSize: fmtSize
    };
})();
