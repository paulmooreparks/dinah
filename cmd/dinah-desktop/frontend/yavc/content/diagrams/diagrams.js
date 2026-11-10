/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* Diagrams: D2 and Mermaid diagrams as text files (docs/diagrams-design.md).
   A diagram is one file, something.d2 or something.mmd (decision 1),
   edited as text in CodeMirror with the drawing beside it, as Slides puts
   a slide's canvas beside its Markdown (decision 15). D2 draws through D2
   itself in a worker (apps/diagram-kit/d2.js) and Mermaid through the
   vendored Mermaid (the editing kit's drawMermaid), so a diagram's text
   never leaves the browser. This is milestone D2, text first: the visual
   editor comes in D3, and Preview stands where Visual will.

   State: file=<path>&view=<text|preview|split>. */
(function () {
    'use strict';
    function sourceFormat(t){return window.yavcContentResources.format(t.resource&&t.resource.encoding!==undefined?t.resource:t.format||t.recoveryBaseline||{});}

    var esc = window.yavcUtil.esc, loadScript = window.yavcUtil.loadScript, versioned = window.yavcUtil.versioned;
    var packageBase = new URL('.', document.currentScript.src);
    function asset(path) { return new URL('../' + path, packageBase).href; }

    /* The shared parts, each loaded once: the editing kit (CodeMirror, its
       theme and Mermaid), the colouring for D2 and Mermaid, and the D2
       engine's client. */
    /* Every caller gets the same promise, so work queued on a part runs
       in the order it was asked for, whether or not the part had loaded:
       a fresh Promise.resolve for a later caller could otherwise run
       before an earlier caller's, and an older drawing replace a newer. */
    var loaded = {};
    function once(key, src, global) {
        if (!loaded[key]) {
            loaded[key] = window[global] ? Promise.resolve(window[global]) : loadScript(versioned(src)).then(function () {
                if (!window[global]) { throw new Error(src + ' did not load'); }
                return window[global];
            });
            loaded[key].catch(function () { loaded[key] = null; });
        }
        return loaded[key];
    }
    function kit() { return once('kit', asset('md-editor/kit.js'), 'yavcMdEditor'); }
    function modes() { return once('modes', asset('diagram-kit/modes.js'), 'yavcDiagramModes'); }
    function d2() { return once('d2', asset('diagram-kit/d2.js'), 'yavcD2Engine'); }
    function canvasLib() { return once('canvas', asset('diagrams/canvas.js'), 'yavcDiagramCanvas'); }
    /* JSON Canvas (docs/canvas-design.md), and the Markdown engine that
       draws a text node's Markdown on the screen (decision 6). */
    function jcanvas() { return once('jcanvas', asset('diagram-kit/jcanvas.js'), 'yavcCanvas'); }
    async function canvasEditor() {
        await jcanvas();
        await once('canvas-model', asset('diagrams/canvas-model.js'), 'yavcCanvasModel');
        return once('canvas-view', asset('diagrams/canvas-view.js'), 'yavcCanvasView');
    }
    function mdEngine() { return once('md', asset('md-engine/loader.js'), 'yavcMdEngine').then(function (f) { return f(); }); }

    /* The styles New offers (decision 7). A style is a starter text; the
       style itself is read from the text, never recorded in the file. */
    var D2_STYLES = [
        { id: 'flowchart', label: 'Flowchart', text: 'direction: right\n\nstart: Start {shape: oval}\ncheck: Is it ready? {shape: diamond}\nwork: Do the work\ndone: Done {shape: oval}\n\nstart -> check\ncheck -> work: no\nwork -> check\ncheck -> done: yes\n' },
        { id: 'architecture', label: 'Architecture', text: 'direction: right\n\nreader: Reader {shape: person}\nsite: The site {\n  web: Web server\n  app: Application\n  web -> app\n}\ndb: Database {shape: cylinder}\njobs: Jobs {shape: queue}\n\nreader -> site.web: HTTPS\nsite.app -> db: reads and writes\nsite.app -> jobs: sends work\n' },
        { id: 'sequence', label: 'Sequence', text: 'shape: sequence_diagram\n\nreader: Reader\npage: Page\nengine: Engine\n\nreader -> page: types the text\npage -> engine: compile\nengine -> page: the drawing\npage -> reader: shows it\n' },
        { id: 'entity', label: 'Entity Relationship', text: 'users: {\n  shape: sql_table\n  id: int {constraint: primary_key}\n  name: varchar\n  email: varchar {constraint: unique}\n}\n\nposts: {\n  shape: sql_table\n  id: int {constraint: primary_key}\n  author: int {constraint: foreign_key}\n  title: varchar\n}\n\nposts.author -> users.id\n' },
        { id: 'class', label: 'Class', text: 'Shape: {\n  shape: class\n  +name: string\n  +area(): float\n}\n\nCircle: {\n  shape: class\n  -radius: float\n  +area(): float\n}\n\nCircle -> Shape: extends\n' },
        { id: 'state', label: 'State Machine', text: 'direction: right\n\nidle: Idle {shape: oval}\nrunning: Running {shape: oval}\ndone: Done {shape: oval}\n\nidle -> running: start\nrunning -> idle: pause\nrunning -> done: finish\n' },
        { id: 'org', label: 'Org Chart', text: 'direction: down\n\nchief: Chief Executive {shape: person}\ntech: Technology {shape: person}\nmoney: Finance {shape: person}\nbuild: Developers {shape: person}\n\nchief -> tech\nchief -> money\ntech -> build\n' },
        { id: 'grid', label: 'Grid', text: 'grid-columns: 3\n\none: One\ntwo: Two\nthree: Three\nfour: Four\nfive: Five\nsix: Six\n' }
    ];
    var MERMAID_STYLES = [
        { id: 'mm-flowchart', label: 'Flowchart', text: 'flowchart LR\n    start([Start]) --> check{Is it ready?}\n    check -- no --> work[Do the work]\n    work --> check\n    check -- yes --> done([Done])\n' },
        { id: 'mm-sequence', label: 'Sequence', text: 'sequenceDiagram\n    participant R as Reader\n    participant P as Page\n    R->>P: types the text\n    P-->>R: shows the drawing\n' },
        { id: 'mm-class', label: 'Class', text: 'classDiagram\n    class Shape {\n        +String name\n        +area() float\n    }\n    class Circle {\n        -float radius\n        +area() float\n    }\n    Shape <|-- Circle\n' },
        { id: 'mm-state', label: 'State Machine', text: 'stateDiagram-v2\n    [*] --> Idle\n    Idle --> Running: start\n    Running --> Idle: pause\n    Running --> [*]: finish\n' },
        { id: 'mm-entity', label: 'Entity Relationship', text: 'erDiagram\n    USER ||--o{ POST : writes\n    USER {\n        int id\n        string name\n    }\n    POST {\n        int id\n        string title\n    }\n' },
        { id: 'mm-gantt', label: 'Gantt Chart', text: 'gantt\n    title A plan\n    dateFormat YYYY-MM-DD\n    section Design\n    Write it down :a1, 2026-11-02, 3d\n    Review it :after a1, 2d\n    section Build\n    Build it :2026-11-09, 5d\n' },
        { id: 'mm-pie', label: 'Pie Chart', text: 'pie title Where the time goes\n    "Writing" : 45\n    "Reading" : 30\n    "Meetings" : 25\n' },
        { id: 'mm-mindmap', label: 'Mind Map', text: 'mindmap\n  root((Diagrams))\n    D2\n      Flowcharts\n      Architecture\n    Mermaid\n      Charts\n      Timelines\n' },
        { id: 'mm-timeline', label: 'Timeline', text: 'timeline\n    title The site\n    2026-10 : The desktop\n    2026-11 : The Editor : Slides\n    2026-12 : Diagrams\n' },
        { id: 'mm-journey', label: 'User Journey', text: 'journey\n    title Drawing a diagram\n    section Start\n      Open Diagrams: 5: Reader\n      Choose a style: 4: Reader\n    section Finish\n      Save it: 5: Reader\n' },
        { id: 'mm-git', label: 'Git Graph', text: 'gitGraph\n    commit\n    branch feature\n    commit\n    checkout main\n    merge feature\n    commit\n' }
    ];
    var EXT = { d2: '.d2', mermaid: '.mmd', canvas: '.canvas' };
    var CANVAS_STYLES = [['blank', 'Blank'], ['basic', 'Basic'], ['flowchart', 'Flowchart'], ['sequence', 'Sequence']];
    /* The palette (decision 15: a toolbar of raised buttons): Select, and
       the kinds of shape flowcharts and architecture diagrams reach for. A
       kind chosen is placed by the next click on the drawing, inside the
       shape clicked or at the top on empty space, and by N. */
    var PALETTE = [['rectangle', 'Rectangle'], ['rounded', 'Rounded Rectangle'], ['oval', 'Oval'], ['diamond', 'Diamond'], ['parallelogram', 'Parallelogram'], ['document', 'Document'],
        ['cylinder', 'Cylinder'], ['queue', 'Queue'], ['person', 'Person'], ['cloud', 'Cloud'], ['hexagon', 'Hexagon'], ['package', 'Package'], ['text', 'Text Only']];
    /* D2's reserved words (d2graph's ReservedKeywords, D2 0.7.2), which
       cannot name a shape. */
    var RESERVED = ['label', 'shape', 'icon', 'constraint', 'tooltip', 'link', 'near', 'width', 'height', 'top', 'left', 'direction', 'class', 'classes', 'vars',
        'layers', 'scenarios', 'steps', 'style', 'grid-rows', 'grid-columns', 'grid-gap', 'vertical-gap', 'horizontal-gap', 'source-arrowhead', 'target-arrowhead',
        'filled', 'import', 'suspend', 'unsuspend', 'null'];
    /* The shapes the visual editor offers, D2's own names in the order a
       flowchart and an architecture diagram reach for them (milestone D3). */
    var SHAPES = [['rectangle', 'Rectangle'], ['square', 'Square'], ['oval', 'Oval'], ['circle', 'Circle'], ['diamond', 'Diamond'], ['hexagon', 'Hexagon'],
        ['parallelogram', 'Parallelogram'], ['step', 'Step'], ['cylinder', 'Cylinder'], ['queue', 'Queue'], ['stored_data', 'Stored Data'], ['package', 'Package'],
        ['page', 'Page'], ['document', 'Document'], ['person', 'Person'], ['cloud', 'Cloud'], ['callout', 'Callout'], ['text', 'Text Only']];
    var KIND_NAME = { d2: 'D2', mermaid: 'Mermaid', canvas: 'Canvas' };
    var VIEWS = [['preview', 'Preview'], ['text', 'Text'], ['split', 'Split']];
    /* Where the drawing sits beside the text, as Slides docks its canvas. */
    var DOCKS = [['left', 'Left'], ['right', 'Right'], ['top', 'Top'], ['bottom', 'Bottom']];
    var SETTINGS = { view: 'split', dock: 'left', fit: true, inspector: true, inspectorSplit: null, inspectorWidth: null, outline: false };
    /* The inspector's choices (milestone D3): line styles as D2's
       stroke-dash, and the arrowheads D2 draws. */
    var DASHES = [['', 'Solid', 'yavc-line-solid'], ['5', 'Dashed', 'yavc-line-dashed'], ['2', 'Dotted', 'yavc-line-dotted']];
    var HEADS = [['', 'D2\'s Own'], ['triangle', 'Triangle'], ['arrow', 'Arrow'], ['diamond', 'Diamond'], ['circle', 'Circle'], ['box', 'Box'],
        ['cf-one', 'One'], ['cf-one-required', 'Exactly One'], ['cf-many', 'Many'], ['cf-many-required', 'One or More'], ['cross', 'Cross']];
    var ARROWS = [['->', 'Forward'], ['<-', 'Backward'], ['<->', 'Both Ways'], ['--', 'No Arrows']];
    var DRAW_DELAY = 300;

    function kindOf(path, text) {
        var p = String(path || '').toLowerCase();
        if (/\.d2$/.test(p)) { return 'd2'; }
        if (/\.(mmd|mermaid)$/.test(p)) { return 'mermaid'; }
        if (/\.canvas$/.test(p)) { return 'canvas'; }
        /* A text with no suffix to go by: a canvas is a JSON object, and
           Mermaid begins with its type. */
        if (/^\s*\{/.test(text || '')) { return 'canvas'; }
        return /^\s*(%%[^\n]*\n\s*)*(graph|flowchart|sequenceDiagram|classDiagram|stateDiagram(-v2)?|erDiagram|gantt|pie|journey|gitGraph|mindmap|timeline|quadrantChart)\b/.test(text || '') ? 'mermaid' : 'd2';
    }
    function baseName(path) { return String(path || '').replace(/^.*\//, ''); }

    var count = 0;

    function init(root, opts) {
        opts = opts || {};
        var n = ++count, H = window.pudlHost || {};
        if (H.contentResources && H.snapshot && (H.snapshot.version !== 2 || typeof H.snapshot.join !== 'function')) throw new Error('This resource host needs snapshot version 2 before it can restore diagrams.');
        var resources = null, recovery = null, recoveryReady = false, recoveryKept = null;
        var documentWindows = H.documentWindows, documentReady = false, receivingDocument = false, unbindDocuments = null;
        async function routeDocument(operation, input) {
            if (!documentWindows || documentWindows.mode(root) !== 'single' || !documentReady || receivingDocument) return null;
            if (operation === 'open-file') {
                if (resources) await resources.read(input.path);
                else if (!H.fs || await H.fs.readText(input.path) == null) throw new Error('The file could not be found.');
            }
            finishInlineText(true);
            if (!current() || untouched(current())) return null;
            return {result: await documentWindows.open(root, 'diagrams', {operation: operation, input: input})};
        }
        var tabs = [], active = -1, untitled = 0, destroyed = false, startupFailed = false, lifetime = new AbortController();
        var settings = Object.assign({}, SETTINGS);
        var CM = null, cm = null, langs = {}, drawTimer = 0, drawSeq = 0, waiters = [], quiet = false, lsp = null, lspComp = null, inlineText = null, inlineNumber = 0;
        var start = new URLSearchParams(String(opts.state || '').replace(/^\?/, ''));
        if (opts.ownsUrl) {
            var own = new URLSearchParams(location.search);
            if (own.get('file')) { start = own; }
        }
        root.classList.add('dg', 'dg-' + (opts.fit === 'fill' ? 'fill' : 'flow'));
        root.tabIndex = -1;
        root.innerHTML =
            '<div class="dg-toolbar-slot" data-toolbar-slot></div>' +
            '<div class="tablist doc-tabs dg-tabs" role="tablist" aria-label="Open Diagrams" data-role="tabs" hidden></div>' +
            '<div class="dg-main" data-role="main">' +
              '<div class="dg-work split" data-role="work">' +
                '<div class="dg-stagebox split-pane" data-role="stagebox">' +
                  '<div class="dg-visual-row"><aside class="dg-outline" data-role="outline" aria-label="Diagram Outline" hidden></aside>' +
                  '<div class="dg-body split" data-role="body" data-split-pane="first">' +
                    '<div class="dg-stage split-pane" data-role="stage" tabindex="0" role="region" aria-label="The Drawing" aria-describedby="dg-keys-' + n + '"><div class="dg-canvas" data-role="canvas"></div></div>' +
                    '<div class="split-handle dg-inspector-split" aria-label="Resize the Inspector" data-split-min="176"></div>' +
                    '<aside class="dg-inspector split-pane" data-role="inspector" aria-label="Inspector" hidden></aside>' +
                  '</div>' +
                  '</div>' +
                  '<p class="dg-keys visually-hidden" id="dg-keys-' + n + '">In a D2 diagram, the arrow keys select a shape or a connection, Enter edits its label, F2 renames it, N adds a shape of the palette\'s kind, C connects the selected shape to another, Delete deletes it, and Escape goes back to the palette\'s Select. The Shape menu moves a shape into another.</p>' +
                '</div>' +
                '<div class="split-handle dg-split" data-role="split" aria-label="Resize the Text" data-split-min="200" data-split-max="70%"></div>' +
                '<div class="dg-text split-pane code-surface" data-role="text"></div>' +
              '</div>' +
            '</div>' +
            '<div class="dg-status" data-role="status"><span data-role="where"></span><span data-role="kind"></span><span data-role="state"></span><span class="dg-message" data-role="message" role="status" aria-live="polite"></span></div>';
        var q = function (sel) { return root.querySelector(sel); };
        var el = { outline: q('[data-role="outline"]'), body: q('[data-role="body"]'), inspector: q('[data-role="inspector"]'), tabsEl: q('[data-role="tabs"]'), work: q('[data-role="work"]'), stagebox: q('[data-role="stagebox"]'), stage: q('[data-role="stage"]'), canvas: q('[data-role="canvas"]'),
            split: q('[data-role="split"]'), text: q('[data-role="text"]'), where: q('[data-role="where"]'), kind: q('[data-role="kind"]'), state: q('[data-role="state"]'), message: q('[data-role="message"]') };

        function keepSetting(key, value) {
            if (!H.config || !H.config.set) { return; }
            var change = {}; change[key] = value;
            H.config.set('diagrams', change).catch(function () { });
        }
        function inspectorPresentation(show) {
            el.inspector.hidden = !show;
            var single = el.body.hasAttribute('data-split-pane');
            if (show) { el.body.removeAttribute('data-split-pane'); } else { el.body.setAttribute('data-split-pane', 'first'); }
            if (single === show && window.pudlSplit) { window.pudlSplit.refresh(); }
            if (single && show) { fitInspector(); }
        }
        el.body.addEventListener('pudl:split', function (e) {
            if (!e.target.classList.contains('dg-inspector-split')) { return; }
            if (el.inspector.hidden || el.body.clientWidth <= 0) { return; }
            settings.inspectorWidth = el.body.style.getPropertyValue('--split-a') ? el.inspector.getBoundingClientRect().width : null;
            keepSetting('inspectorWidth', settings.inspectorWidth);
        });
        function fitInspector() {
            if (destroyed || el.inspector.hidden || el.body.clientWidth <= 0) { return; }
            var handle = q('.dg-inspector-split').getBoundingClientRect().width, available = el.body.clientWidth - handle;
            var preferred = settings.inspectorWidth;
            if (!(Number.isFinite(preferred) && preferred > 0)) {
                preferred = parseFloat(getComputedStyle(document.documentElement).fontSize) * 22;
                if (Number.isFinite(settings.inspectorSplit) && settings.inspectorSplit > 0 && settings.inspectorSplit < 100) {
                    preferred = el.body.clientWidth * (1 - settings.inspectorSplit / 100) - handle;
                    settings.inspectorWidth = preferred; settings.inspectorSplit = null;
                }
            }
            var width = Math.max(0, Math.min(preferred, available - Math.min(176, available / 2)));
            el.body.style.setProperty('--split-a', 'calc(100% - ' + width + 'px - ' + handle + 'px)');
            positionInlineText();
        }
        var inspectorWhole = 0, inspectorLastWidth = 0;
        var inspectorSize = new ResizeObserver(function () {
            if (destroyed || el.inspector.hidden || el.body.clientWidth <= 0) { return; }
            if (el.body.clientWidth !== inspectorWhole) {
                inspectorWhole = el.body.clientWidth; fitInspector(); inspectorLastWidth = el.inspector.getBoundingClientRect().width;
            } else {
                var width = el.inspector.getBoundingClientRect().width;
                if (Math.abs(width - inspectorLastWidth) > 0.5) { settings.inspectorWidth = width; inspectorLastWidth = width; }
                positionInlineText();
            }
        }); inspectorSize.observe(el.body); inspectorSize.observe(el.inspector);
        /* Failed commands use toasts. Parse and recovery warnings remain
           beside their content; confirmations use the status line. */
        function say(msg, kind) {
            if (kind === true || kind === 'danger') { el.message.textContent = ''; window.yavcUtil.toast(msg, 'danger'); return; }
            if (kind) { el.message.textContent = ''; window.yavcUtil.notice(el.tabsEl, msg, kind); return; }
            el.message.textContent = msg || '';
        }
        var reportCommands=window.yavcUtil.commandChanges(commandTable,function(){if(opts.commandsChanged)opts.commandsChanged();},function(){return !destroyed;});
        function commandsChanged() { reportCommands(); tabRow(); }
        function current() { return tabs[active] || null; }
        function readOnly(t) { return !!(resources && t && t.path && t.resource && !resources.canWrite(t.resource)); }
        function mutationWhy(t) { return readOnly(t || current()) ? 'This diagram is read-only. Use Save As to make an editable copy.' : null; }
        function requireEditable(t) { var reason = mutationWhy(t); if (reason) throw new Error(reason); }
        function stateString() {
            var t = current(), p = new URLSearchParams();
            if (t && t.path) { p.set('file', t.path); }
            if (t && t.view && t.view !== settings.view) { p.set('view', t.view); }
            if (t && t.kind === 'd2' && t.canvas && t.canvas.selected()) { p.set('item', t.canvas.selected()); }
            return p.toString().replace(/%2F/g, '/').replace(/%7E/g, '~');
        }
        function announce() {
            var state = stateString();
            if (opts.ownsUrl) {
                var url = new URL(location.href), values = new URLSearchParams(state);
                ['file', 'view', 'item'].forEach(function (key) { if (values.has(key)) url.searchParams.set(key, values.get(key)); else url.searchParams.delete(key); });
                if (url.href !== location.href) history.replaceState(history.state, '', url);
            }
            if (opts.changed) { opts.changed(state); } keep();
        }

        /* === The text (CodeMirror) ======================================= */

        function language(kind) {
            if (!langs[kind]) { langs[kind] = kind === 'canvas' ? CM.json() : window.yavcDiagramModes[kind](CM); }
            return langs[kind];
        }
        /* Each tab keeps its own editor state, so its undo history and its
           cursor survive a switch to another tab and back. */
        function stateFor(t, text) {
            t.accessComp = new CM.Compartment();
            var visualChange = CM.StateField.define({ create: function () { return ''; }, update: function (v, tr) { return tr.docChanged ? tr.annotation(CM.Transaction.userEvent) || '' : v; } });
            var history = t.kind === 'canvas' ? CM.history({ joinToEvent: function (tr, adjacent) {
                var previous = tr.startState.field(visualChange), event = tr.annotation(CM.Transaction.userEvent) || '';
                if (tr.isUserEvent('input.type.canvas')) { return adjacent && previous === event; }
                return adjacent && !tr.isUserEvent('input.canvas') && !/^input\.(canvas|type\.canvas)/.test(previous);
            } }) : CM.history();
            var ext = [visualChange, CM.lineNumbers(), CM.highlightActiveLineGutter(), history, CM.drawSelection(), CM.highlightActiveLine(), CM.highlightSelectionMatches(), CM.bracketMatching(), CM.closeBrackets(),
                t.accessComp.of([]), CM.EditorState.transactionFilter.of(function (tr) { return tr.docChanged && readOnly(t) ? [] : tr; }),
                lspComp.of(lsp && t.kind === 'canvas' ? lsp.extension('canvas', readOnly(t)) : []),
                CM.keymap.of([].concat(CM.closeBracketsKeymap, CM.completionKeymap, CM.lintKeymap, CM.defaultKeymap, CM.searchKeymap, CM.historyKeymap, [CM.indentWithTab])),
                window.yavcMdEditor.codeSearch(CM),
                CM.indentUnit.of('  '),
                window.yavcMdEditor.codeTheme(CM), language(t.kind),
                CM.EditorView.contentAttributes.of({ 'aria-label': 'The Diagram\'s Text' }),
                CM.linter(lintSource, { delay: DRAW_DELAY }), CM.lintGutter(),
                CM.EditorView.updateListener.of(function (u) {
                    if (u.docChanged && !quiet) { textChanged(u.state.doc.toString()); }
                    else if (u.selectionSet && !quiet && u.view.hasFocus) { followCursor(u.state.selection.main.head); }
                })];
            if (t.kind === 'd2') { ext.push(CM.autocompletion({ override: [complete], activateOnTyping: true })); }
            // Keep completion state available when a tab has no completion provider.
            else if (t.kind === 'mermaid' || !lsp) { ext.push(CM.autocompletion({ override: [], activateOnTyping: false })); }
            return CM.EditorState.create({ doc: text, extensions: ext });
        }
        function showTab(t) {
            if (!cm) { cm = new CM.EditorView({ state: t.state, parent: el.text }); }
            else {
                quiet = true;
                try { cm.setState(t.state); } finally { quiet = false; }
            }
            refreshAccess(t);
        }
        function refreshAccess(t) {
            if (!cm || t !== current()) return;
            var locked = readOnly(t);
            root.setAttribute('data-read-only', String(locked));
            if (t.settled && !t.settled.problems.length) el.state.textContent = locked ? 'Read-only' : 'Drawn';
            cm.dispatch({ effects: [t.accessComp.reconfigure([CM.EditorState.readOnly.of(locked), CM.EditorView.editable.of(!locked), CM.EditorView.contentAttributes.of({ 'aria-readonly': String(locked), tabindex: '0' })]), lspComp.reconfigure(lsp && t.kind === 'canvas' ? lsp.extension('canvas', locked) : [])] });
            if (locked) { setTool(null); if (connecting) stopConnect(''); }
            if (t.cv && t.cv.refreshAccess) t.cv.refreshAccess();
            if (t.canvas && t.canvas.refreshAccess) t.canvas.refreshAccess();
        }
        /* The text a command replaces, as one change, so Undo takes it back. */
        function replaceText(text, userEvent) {
            var t = current();
            if (!t || !cm || cm.state.doc.toString() === text) { return; }
            requireEditable(t);
            cm.dispatch({ changes: { from: 0, to: cm.state.doc.length, insert: text }, userEvent: userEvent });
        }
        function textChanged(text) {
            var t = current();
            if (!t) { return; }
            if (inlineText && !inlineText.writing) { finishInlineText(false); }
            t.text = text;
            if (t.kind === 'd2') renderOutline();
            if (t.cv) { t.cv.cancel(); t.place.classList.add('sc-diagram-stale'); }
            if (!t.modified) { t.modified = true; commandsChanged(); }
            scheduleDraw();
            keep();
        }

        /* Completions from D2's own language service, run in the engine
           (docs/diagrams-design.md, "What this means for the Editor"). They
           come unasked after a dot or a colon, and with Ctrl+Space anywhere. */
        function complete(ctx) {
            var word = ctx.matchBefore(/[\w-]*/), before = ctx.state.doc.sliceString(Math.max(0, word.from - 2), word.from);
            if (!ctx.explicit && !/[.:]\s?$/.test(before)) { return null; }
            var line = ctx.state.doc.lineAt(ctx.pos);
            return d2().then(function (E) {
                /* D2's service answers for the place a word begins, not part
                   way through one, so it is asked there and CodeMirror
                   narrows the list by what has been typed. */
                return E.call('completions', { text: ctx.state.doc.toString(), line: line.number - 1, column: word.from - line.from }, 'complete-' + n);
            }).then(function (out) {
                if (!out || !out.items || !out.items.length) { return null; }
                return { from: word.from, validFor: /^[\w-]*$/, options: out.items.map(function (it) {
                    return { label: it.label, detail: it.detail || undefined, apply: it.insert || it.label, type: it.kind === 'shape' ? 'type' : it.kind === 'style' ? 'property' : 'keyword' };
                }) };
            }).catch(function () { return null; });
        }

        /* The mistakes in the text are marked where they are, as the
           drawing finds them: the linter waits for the drawing of the text
           it was asked about. */
        function lintSource(view) {
            var text = view.state.doc.toString(), t = current();
            /* A drawing can finish before the linter asks, as a canvas's
               does, so the answer for the text drawn last is kept. */
            if (t && t.settled && t.settled.text === text) { return diagnostics(view.state.doc, t.kind, t.settled.problems); }
            return new Promise(function (resolve) { waiters.push({ text: text, resolve: resolve, doc: view.state.doc, kind: (current() || {}).kind }); });
        }
        function settle(text, problems) {
            var left = [];
            waiters.forEach(function (w) {
                if (w.text === text) { w.resolve(diagnostics(w.doc, w.kind, problems)); }
                else if (destroyed) { w.resolve([]); }
                else { left.push(w); }
            });
            waiters = left.filter(function (w) { return w.text === (current() || {}).text; });
            left.forEach(function (w) { if (waiters.indexOf(w) < 0) { w.resolve([]); } });
        }
        function diagnostics(doc, kind, problems) {
            var out = [];
            (problems || []).forEach(function (p) {
                var line = p.line;
                if (kind === 'mermaid' && !line) { var m = /line (\d+)/i.exec(p.message); line = m ? +m[1] : 0; }
                if (!(line >= 1 && line <= doc.lines)) { return; }
                var l = doc.line(line), from = l.from, to = l.to;
                if (kind !== 'mermaid' && p.to > p.from && p.from >= l.from && p.to <= doc.length) { from = p.from; to = p.to; }
                else if (kind === 'd2' && p.column > 1) { from = Math.min(l.to, l.from + p.column - 1); }
                out.push({ from: from, to: Math.max(to, from), severity: 'error', message: p.message });
            });
            return out;
        }

        /* === The drawing ================================================== */

        function scheduleDraw() { clearTimeout(drawTimer); drawTimer = setTimeout(drawNow, DRAW_DELAY); }
        function altOf(t) { return 'The diagram ' + (t.path ? baseName(t.path) : t.name); }
        /* Draws the front tab's text. A mistake keeps the last good drawing,
           dimmed, with the site's notice above it saying what and where
           (decision 5). */
        function drawNow() {
            clearTimeout(drawTimer);
            var t = current();
            if (!t) { return Promise.resolve(); }
            var text = t.text, seq = ++drawSeq;
            el.state.textContent = 'Drawing…';
            var drawn = t.kind === 'd2'
                ? d2().then(function (E) { return E.draw(t.place, text, { keep: true, report: false, alt: altOf(t) }); })
                : t.kind === 'canvas' ? drawCanvas(t, text, function () { return seq === drawSeq && t === current() && t.text === text; })
                : kit().then(function (K) { return K.drawMermaid(t.place, text, { keep: true, report: false, alt: altOf(t) }); });
            return drawn.then(async function (out) {
                /* No answer means another drawing of this place replaced
                   this one; if this is still the newest, it draws again. */
                if (!out) { if (seq === drawSeq) { scheduleDraw(); } return; }
                var problems = out.problems || (out.problem ? [{ message: out.problem, line: 0 }] : []);
                if (seq === drawSeq && t === current()) {
                    t.problems = problems;
                    if (!problems.length) { t.svg = out.svg; t.size = out.width ? { w: out.width, h: out.height } : sizeOfSvg(out.svg); }
                    if (!problems.length && t.kind === 'd2') { await showCanvas(t, out, text); }
                    if (destroyed || seq !== drawSeq || t !== current() || t.text !== text) return;
                    showOutcome(t);
                }
                t.settled = { text: text, problems: problems };
                settle(text, problems);
            }, function (err) {
                var problems = [{ message: String((err && err.message) || err), line: 0 }];
                if (seq === drawSeq && t === current()) { t.problems = problems; showOutcome(t); }
                settle(text, problems);
            });
        }
        function showOutcome(t) {
            var p = t.problems || [];
            var stale = t.place.classList.contains('sc-diagram-stale');
            markStale(t, stale);
            if (p.length) {
                var first = p[0], where = first.line > 0 ? 'Line ' + first.line + ': ' : '';
                var more = p.length > 1 ? ' (and ' + (p.length - 1) + ' more)' : '';
                window.yavcUtil.notice(el.body, 'The diagram could not be drawn. ' + where + first.message + more + (stale ? ' The drawing shown is the last one that worked.' : ''), 'problem');
                el.state.textContent = p.length === 1 ? 'One mistake' : p.length + ' mistakes';
            } else {
                window.yavcUtil.notice(el.body, '');
                el.state.textContent = readOnly(t) ? 'Read-only' : 'Drawn';
            }
            commandsChanged();
            renderInspector();
        }
        /* The last good drawing, kept while the text has a mistake, is out
           of date, and its text alternative says so, so the dimming is not
           the only sign (PUDL: meaning never by colour alone). */
        function markStale(t, stale) {
            var pic = t.place.querySelector(':scope > img, :scope > svg, :scope > * > svg');
            if (!pic) { return; }
            var name = altOf(t) + (stale ? ' (out of date)' : '');
            if (pic.tagName.toLowerCase() === 'img') { pic.alt = name; } else { pic.setAttribute('aria-label', name); }
        }
        /* A Mermaid drawing's size, from its viewBox, for a picture of it. */
        function sizeOfSvg(svg) {
            var m = /viewBox="\s*[-\d.]+[\s,]+[-\d.]+[\s,]+([\d.]+)[\s,]+([\d.]+)/.exec(svg || '');
            return m ? { w: Math.ceil(+m[1]), h: Math.ceil(+m[2]) } : null;
        }
        /* === A canvas (docs/canvas-design.md, milestone C1) ===============
           The site reads and draws a canvas itself (decision 5): the file is
           checked against JSON Canvas and the extension before anything is
           drawn (decision 13), drawn as SVG made from numbers and escaped
           text, and each text node's Markdown is then drawn over its shape
           by the Markdown engine and the HTML cleaner (decision 6). The SVG
           without that, with the text as SVG text, is the drawing a picture
           or an export is made from. */
        function lineAt(text, at) { return at == null ? 0 : text.slice(0, at).split('\n').length; }
        /* What a problem at an offset marks: the quoted id it is at, or one
           character. */
        function spanAt(text, at) {
            if (at == null) { return [null, null]; }
            var end = text.charAt(at) === '"' ? text.indexOf('"', at + 1) + 1 : 0;
            return [at, end > at ? end : Math.min(text.length, at + 1)];
        }
        async function drawCanvas(t, text, isCurrent) {
            var J = await jcanvas(), V = await canvasEditor();
            var got = J.read(text), place = t.place;
            var faults = got.problems.filter(function (p) { return !p.warning; });
            if (!got.doc || faults.length) {
                if (place.firstElementChild) { place.classList.add('sc-diagram-stale'); }
                return { problems: faults.map(function (p) { var s = spanAt(text, p.at); return { message: p.message, line: lineAt(text, p.at) || 1, from: s[0], to: s[1] }; }) };
            }
            var palette = canvasPalette(place, J), out = J.render(got.doc, { palette: palette, pad: 24, font: getComputedStyle(root).fontFamily, title: altOf(t) });
            var warning = got.problems.filter(function (p) { return p.warning; })[0];
            if (warning) { say(warning.message, 'warn'); }
            if (isCurrent()) {
                var svg = new DOMParser().parseFromString(out.svg, 'image/svg+xml').documentElement;
                svg = document.importNode(svg, true);
                svg.setAttribute('role', 'img');
                svg.setAttribute('aria-label', altOf(t));
                svg.style.backgroundColor = 'transparent';
                place.classList.remove('sc-diagram-stale');
                place.replaceChildren(svg);
                t.cvDoc = got.doc; t.cvColors = out.boxes; t.cvText = text; t.cvRanges = canvasRanges(text);
                if (!t.cv) {
                    t.cv = V.attach(place, el.stage, {
                        selected: function (s, how) { canvasSelectionChanged(t, s, how); },
                        enabled: function () { return t === current() && t.text === t.cvText && !t.place.classList.contains('sc-diagram-stale'); },
                        readOnly: function () { return readOnly(t); },
                        text: function () { return t.text; },
                        tool: function () { return tool; }, panTool: function () { return canvasPan; },
                        create: function (x, y) { canvasCreate(x, y); },
                        activate: function (key) { if (t === current()) { startInlineText(key); } },
                        viewChanged: positionInlineText,
                        edit: function (req, expected) { canvasEdit(req, expected); }
                    });
                }
                t.cv.show(got.doc, out, svg);
                var pending = t.cvPending; t.cvPending = null;
                if (pending && pending.text === text) { t.cv.select(pending.selection, 'edit'); }
                else if (cm && cm.hasFocus) { followCanvasCursor(t, cm.state.selection.main.head); }
                richText(t, got.doc, out, svg, palette, isCurrent);
            }
            return { svg: out.svg, width: out.width, height: out.height, problems: [] };
        }
        /* === Editing JSON Canvas ========================================= */
        function finishInlineText(focus) {
            if (!inlineText) { return; }
            var entry = inlineText; inlineText = null; entry.el.remove();
            if (focus && !destroyed) { el.stage.focus({ preventScroll: true }); }
        }
        function positionInlineText() {
            var entry = inlineText;
            if (!entry || entry.owner !== current() || !entry.owner.cv) { return; }
            var box = entry.owner.cv.bounds(entry.key), stage = el.stage.getBoundingClientRect();
            if (!box) { finishInlineText(false); return; }
            entry.el.style.width = Math.min(Math.max(box.width, 180), el.stage.clientWidth) + 'px';
            entry.field.style.height = Math.min(240, Math.max(64, box.height)) + 'px';
            entry.el.style.left = (Math.max(0, Math.min(box.x - stage.left, el.stage.clientWidth - entry.el.offsetWidth)) + el.stage.scrollLeft) + 'px';
            entry.el.style.top = (Math.max(0, Math.min(box.y - stage.top, el.stage.clientHeight - entry.el.offsetHeight)) + el.stage.scrollTop) + 'px';
        }
        function startInlineText(key) {
            if (readOnly(current())) return;
            var t = current(), selected = canvasSelection();
            key = key || (selected && selected.id);
            if (canvasWhy('shape')) { return; }
            var node = t.cvDoc.nodes.find(function (n) { return n.id === key; });
            if (!node || node.type !== 'text' || window.yavcCanvas.isAnchor(node)) { return; }
            if (inlineText && inlineText.owner === t && inlineText.key === key) { inlineText.field.focus(); return; }
            finishInlineText(false);
            var editor = document.createElement('section'), field = document.createElement('textarea'), label = document.createElement('label'), controls = document.createElement('div'), help = document.createElement('span'), done = document.createElement('button');
            editor.className = 'dg-inline-editor form-group'; editor.setAttribute('role', 'group'); editor.setAttribute('aria-label', 'Edit Shape Text');
            field.id = 'dg-inline-' + n + '-' + (++inlineNumber); field.className = 'form-input'; field.value = node.text; field.spellcheck = true;
            label.className = 'form-label'; label.htmlFor = field.id; label.textContent = 'Text (Markdown)';
            help.id = field.id + '-help'; help.className = 'form-help'; help.textContent = 'Enter adds a line. Escape finishes. Undo reverts typing.'; field.setAttribute('aria-describedby', help.id);
            controls.className = 'hstack'; done.type = 'button'; done.className = 'btn btn-sm'; done.textContent = 'Done'; controls.append(help, done);
            editor.append(label, field, controls); el.stage.appendChild(editor);
            var entry = { owner: t, key: key, el: editor, field: field, value: node.text, session: inlineNumber, inputVersion: 0, writing: false }; inlineText = entry;
            done.addEventListener('click', function () { finishInlineText(true); });
            editor.addEventListener('focusout', function (e) { if (e.relatedTarget && !editor.contains(e.relatedTarget) && inlineText === entry) { finishInlineText(false); } });
            field.addEventListener('input', function () {
                if (inlineText !== entry || entry.owner !== current()) { return; }
                var value = field.value, before = entry.value, version = ++entry.inputVersion;
                entry.value = value; entry.writing = true;
                var pending = canvasEdit({ op: 'set', kind: 'node', key: key, property: 'text', value: value }, t.text, true, 'input.type.canvas.' + entry.session);
                entry.writing = false;
                pending.then(function () { if (inlineText === entry && entry.inputVersion === version) { markField(field, null); } }, function (err) {
                    if (inlineText === entry && entry.inputVersion === version && field.value === value) { entry.value = before; field.value = before; markField(field, err.message); }
                });
            });
            field.addEventListener('keydown', function (e) {
                if (e.isComposing) { return; }
                if (e.key === 'Escape') { e.preventDefault(); e.stopPropagation(); finishInlineText(true); }
                else if ((e.ctrlKey || e.metaKey) && /^[zy]$/i.test(e.key)) {
                    e.preventDefault(); e.stopPropagation(); entry.writing = true;
                    if (e.key.toLowerCase() === 'y' || e.shiftKey) { CM.redo(cm); } else { CM.undo(cm); }
                    entry.writing = false;
                    var got = window.yavcCanvas.read(t.text), updated = got.doc && got.doc.nodes.find(function (n) { return n.id === key && n.type === 'text'; });
                    if (updated) { entry.value = updated.text; field.value = updated.text; } else { finishInlineText(true); }
                }
            });
            positionInlineText(); field.focus(); field.select();
        }
        function canvasSelection() { var t = current(); return t && t.cv ? t.cv.selected() : null; }
        function canvasSelections() { var t = current(); return t && t.cv ? t.cv.selections() : []; }
        function canvasItems(t) {
            return t.cvDoc.nodes.filter(function (x) { return !window.yavcCanvas.isAnchor(x); }).map(function (x) { return { id: x.id, kind: 'node' }; }).concat(t.cvDoc.edges.map(function (x) { return { id: x.id, kind: 'edge' }; }));
        }
        function canvasSelectAll() { var t = current(); if (!canvasWhy(false)) { t.cv.select(canvasItems(t)); } }
        async function canvasSelectItems() {
            if (canvasWhy(false)) { return; }
            var t = current(), text = t.text, items = canvasItems(t), selected = canvasSelections(), filterId = 'dg-items-' + crypto.randomUUID();
            if(!window.pudlListbox)await loadScript((H.pudl || '/pudl/')+'pudl-listbox.js');
            if(destroyed || t!==current() || t.text!==text)return;
            return window.yavcDialog.open({ title: 'Select Canvas Items', className: 'dg-item-dialog', signal: lifetime.signal,
                body: '<div class="form-group"><label class="form-label" for="' + filterId + '">Find an Item</label><input type="search" class="form-input" id="' + filterId + '" data-item-filter></div><p class="form-help" data-selection-count aria-live="polite"></p><div class="listbox dg-item-list" data-listbox="multiple" data-listbox-rows="7" aria-label="Diagram Items">' + items.map(function (s, i) {
                var checked = selected.some(function (x) { return x.id === s.id && x.kind === s.kind; });
                var entry = s.kind === 'node' ? t.cvDoc.nodes.find(function (x) { return x.id === s.id; }) : t.cvDoc.edges.find(function (x) { return x.id === s.id; });
                var label = entry && (entry.text || entry.label), caption = label ? String(label).replace(/\s+/g, ' ').slice(0, 100) : s.id;
                return '<div class="listbox-row" data-item-row data-value="' + i + '" aria-selected="' + checked + '"><span class="listbox-title">' + esc(caption) + '</span><span class="listbox-detail">' + esc((s.kind === 'node' ? 'Node' : 'Connector') + ' · ' + s.id) + '</span></div>';
            }).join('') + '</div><p class="form-help" data-no-items hidden>No matching items.</p>', buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'apply', label: 'Select', primary: true }],
                onOpen: function (api) {
                    var list=api.body.querySelector('[data-listbox]');window.pudlListbox.enhance(api.body);
                    function refresh() {
                        var query = api.body.querySelector('[data-item-filter]').value.trim().toLowerCase(), visible = 0;
                        api.body.querySelectorAll('[data-item-row]').forEach(function (row) { row.hidden = !row.textContent.toLowerCase().includes(query); if (!row.hidden) visible++; });
                        api.body.querySelector('[data-no-items]').hidden = !!visible;
                        api.body.querySelector('[data-selection-count]').textContent = window.pudlListbox.value(list).length + ' selected of ' + items.length + ' items';
                        window.pudlListbox.refresh(list);
                    }
                    api.body.addEventListener('input', refresh);api.body.addEventListener('pudl:listbox-change',refresh);refresh();
                }, check: function (value, body) {
                    if (value === 'apply' && (destroyed || t !== current() || t.text !== text)) { window.yavcUtil.notice(body, 'The diagram changed. Close and reopen this selection.', true); return true; }
                } }).then(function (r) {
                if (r.value !== 'apply' || destroyed || t !== current()) { return; }
                if (t.text !== text) { say('The canvas changed while the selection dialog was open. Open it again to choose current items.', 'warn'); return; }
                t.cv.select(Array.from(r.body.querySelectorAll('[data-item-row][aria-selected="true"]')).map(function (e) { return items[Number(e.dataset.value)]; }));
            });
        }
        /* CodeMirror's JSON parser supplies offsets. Only objects directly
           in nodes/edges are indexed, so nested IDs and escaped keys cannot
           select the wrong item. JSON.parse has already validated the file. */
        function canvasRanges(text) {
            var object = language('canvas').language.parser.parse(text).topNode.getChild('Object'), ranges = [];
            function property(obj, key) {
                return obj.getChildren('Property').filter(function (p) { var name = p.getChild('PropertyName'); return name && JSON.parse(text.slice(name.from, name.to)) === key; }).pop();
            }
            ['nodes', 'edges'].forEach(function (key) {
                var prop = object && property(object, key), list = prop && prop.getChild('Array');
                if (!list) { return; }
                list.getChildren('Object').forEach(function (obj) {
                    var id = property(obj, 'id'), value = id && id.getChild('String');
                    if (value) { ranges.push({ id: JSON.parse(text.slice(value.from, value.to)), kind: key === 'nodes' ? 'node' : 'edge', from: obj.from, to: obj.to }); }
                });
            });
            return ranges;
        }
        function canvasSelectionChanged(t, s, how) {
            if (t !== current()) { return; }
            if (how === 'pointer') { el.stage.focus({ preventScroll: true }); }
            if (how !== 'cursor') {
                say(canvasSelections().length > 1 ? 'Selected ' + canvasSelections().length + ' items.' : s ? 'Selected ' + s.kind + ' "' + s.id + '".' : '');
                var range = s && (t.cvRanges || []).find(function (r) { return r.id === s.id && r.kind === s.kind; });
                if (range && cm && t.cvText === t.text) {
                    quiet = true;
                    try { cm.dispatch({ selection: { anchor: range.from, head: range.to }, effects: CM.EditorView.scrollIntoView(range.from, { y: 'center' }) }); } finally { quiet = false; }
                }
            }
            commandsChanged(); renderInspector(); renderOutline();
        }
        var outlineText = null, outlineOwner = null, outlineAnchor = null;
        function d2ItemURL(key) {
            var p = new URLSearchParams(stateString()), url = new URL(location.href), param = root.getAttribute('data-applet-param');
            p.set('item', key);
            if (current()) p.set('view', viewOf(current()));
            if (param) url.searchParams.set(param, p.toString());
            else ['file', 'view', 'item'].forEach(function (name) { if (p.has(name)) url.searchParams.set(name, p.get(name)); else url.searchParams.delete(name); });
            return url.href;
        }
        function renderD2Outline(t) {
            var ready = !!(t.canvas && t.outline && t.outlineText === t.text && !(t.problems || []).length);
            var key = t.canvas && t.canvas.selected(), shapes = t.outline && t.outline.shapes || [], connections = t.outline && t.outline.connections || [];
            if (outlineOwner !== t || outlineText !== t.outlineText) {
                var focused = el.outline.contains(document.activeElement) ? document.activeElement.dataset.d2Outline : null;
                if (outlineOwner === t) el.outline.querySelectorAll('[data-d2-outline][aria-expanded]').forEach(function (a) { (t.outlineOpen || (t.outlineOpen = {}))[a.dataset.d2Outline] = a.getAttribute('aria-expanded') === 'true'; });
                outlineOwner = t; outlineText = t.outlineText;
                var children = new Map(), known = new Set(shapes.map(function (s) { return s.id; })), byId = new Map(shapes.map(function (s) { return [s.id, s]; }));
                shapes.forEach(function (s) { var parent = known.has(s.parent) ? s.parent : ''; if (!children.has(parent)) children.set(parent, []); children.get(parent).push(s); });
                function ancestors(id) { var result = [], item = byId.get(id); while (item && item.parent && result.length < 64) { result.push(item.parent); item = byId.get(item.parent); } result.push(''); return result; }
                connections.forEach(function (c) {
                    var right = new Set(ancestors(c.to)), parent = ancestors(c.from).find(function (id) { return right.has(id); }) || '';
                    if (!children.has(parent)) children.set(parent, []);
                    children.get(parent).push(Object.assign({}, c, {label:c.label || c.from + ' → ' + c.to}));
                });
                function link(item, nested) {
                    var label = item.label || item.id;
                    return '<a data-d2-outline="' + esc(item.id) + '" href="' + esc(d2ItemURL(item.id)) + '" title="' + esc(item.id) + '"' + (nested ? ' aria-expanded="' + (!(t.outlineOpen && t.outlineOpen[item.id] === false)) + '"' : '') + '>' + optionGlyph(['', '', isConnection(item.id) ? 'yavc-connect' : 'square']) + '<span>' + esc(label) + '</span></a>';
                }
                function branch(parent, depth) { return (children.get(parent) || []).map(function (s) { var nested = depth < 64 && children.has(s.id); return '<li>' + link(s, nested) + (nested ? '<ul>' + branch(s.id, depth + 1) + '</ul>' : '') + '</li>'; }).join(''); }
                el.outline.innerHTML = '<h3 class="dg-inspector-title">Outline</h3><ul class="tree" aria-label="Shapes and Connections">' + branch('', 0) + '</ul>';
                if (window.pudlTree) window.pudlTree.enhance(el.outline.querySelector('.tree'));
                if (focused) { var back = Array.from(el.outline.querySelectorAll('[data-d2-outline]')).find(function (a) { return a.dataset.d2Outline === focused; }); if (back) back.focus({preventScroll:true}); }
            }
            el.outline.querySelectorAll('[data-d2-outline]').forEach(function (a) {
                a.href = d2ItemURL(a.dataset.d2Outline);
                a.setAttribute('aria-disabled', String(!ready));
                if (a.dataset.d2Outline === key) {
                    a.setAttribute('aria-current', 'true');
                    for (var group = a.parentElement.parentElement; group && group !== el.outline.querySelector('.tree') && group.tagName === 'UL'; group = group.parentElement.parentElement) {
                        var parent = group.previousElementSibling; if (parent && parent.hasAttribute('aria-expanded')) parent.setAttribute('aria-expanded', 'true');
                    }
                } else a.removeAttribute('aria-current');
            });
        }
        function renderOutline() {
            var t = current(), show = settings.outline && t && (t.kind === 'd2' || t.kind === 'canvas' && !!t.cv);
            el.outline.hidden = !show;
            if (!show) { outlineOwner = null; outlineText = null; el.outline.replaceChildren(); return; }
            if (t.kind === 'd2') { renderD2Outline(t); return; }
            var items = canvasItems(t), selected = canvasSelections();
            if (outlineOwner !== t || outlineText !== t.cvText) {
                var focused = el.outline.contains(document.activeElement) ? document.activeElement.dataset.outline : null;
                if (outlineOwner !== t) { outlineAnchor = null; }
                outlineOwner = t; outlineText = t.cvText;
                el.outline.innerHTML = '<h3 class="dg-inspector-title">Outline</h3><ul>' + items.map(function (s, i) {
                    var item = (s.kind === 'node' ? t.cvDoc.nodes : t.cvDoc.edges).find(function (x) { return x.id === s.id; });
                    var label = String(item.text || item.label || item.file || item.url || item.id).replace(/\s+/g, ' ').slice(0, 160);
                    return '<li><button type="button" class="btn btn-sm" data-outline="' + i + '" aria-pressed="false" title="' + esc(s.kind + ' ' + s.id) + '">' + optionGlyph(['', '', s.kind === 'node' ? 'square' : 'yavc-connect']) + '<span>' + esc(label) + '</span></button></li>';
                }).join('') + '</ul><p class="form-help">Ctrl or Command toggles items; Shift selects a range. Arrow keys move between rows.</p>';
                if (focused !== null) { var back = el.outline.querySelector('[data-outline="' + focused + '"]'); if (back) { back.focus({ preventScroll: true }); } }
            }
            el.outline.querySelectorAll('[data-outline]').forEach(function (button) {
                var item = items[Number(button.dataset.outline)]; button.disabled = !!canvasWhy(false);
                button.setAttribute('aria-pressed', String(selected.some(function (s) { return s.id === item.id && s.kind === item.kind; })));
            });
        }
        function toggleOutline() { settings.outline = !settings.outline; keepSetting('outline', settings.outline); renderOutline(); commandsChanged(); }
        el.outline.addEventListener('click', function (e) {
            var link = e.target.closest('[data-d2-outline]');
            if (link) {
                if (e.target.closest('.tree-toggle')) return;
                link.href = d2ItemURL(link.dataset.d2Outline);
                if (link.getAttribute('aria-disabled') !== 'true' && (e.button !== 0 || e.ctrlKey || e.metaKey || e.shiftKey || e.altKey)) return;
                e.preventDefault(); var owner = current();
                if (owner && owner.kind === 'd2' && owner.outlineText === owner.text && !(owner.problems || []).length) { selectKey(owner, link.dataset.d2Outline, 'outline'); if(owner.canvas)owner.canvas.reveal(); }
                return;
            }
            var button = e.target.closest('[data-outline]'); if (!button || canvasWhy(false)) { return; }
            var t = current(), items = canvasItems(t), i = Number(button.dataset.outline), item = items[i], chosen = [item];
            if (e.shiftKey && outlineAnchor !== null && outlineAnchor < items.length) { chosen = items.slice(Math.min(i, outlineAnchor), Math.max(i, outlineAnchor) + 1); }
            else if (e.ctrlKey || e.metaKey) {
                chosen = canvasSelections(); var at = chosen.findIndex(function (s) { return s.id === item.id && s.kind === item.kind; });
                if (at < 0) { chosen.push(item); } else { chosen.splice(at, 1); }
            }
            if (!e.shiftKey) { outlineAnchor = i; }
            t.cv.select(chosen, 'outline'); t.cv.reveal();
        });
        el.outline.addEventListener('keydown', function (e) {
            var buttons = Array.from(el.outline.querySelectorAll('[data-outline]')), i = buttons.indexOf(e.target), next;
            if (i < 0 || !['ArrowUp', 'ArrowDown', 'Home', 'End'].includes(e.key)) { return; }
            next = e.key === 'Home' ? 0 : e.key === 'End' ? buttons.length - 1 : Math.max(0, Math.min(buttons.length - 1, i + (e.key === 'ArrowUp' ? -1 : 1)));
            e.preventDefault(); buttons[next].focus();
        });
        /* F6 leaves item cycling and reaches each editing pane directly. */
        root.addEventListener('keydown', function (e) {
            if (e.defaultPrevented || e.isComposing || e.key !== 'F6' || e.altKey || e.ctrlKey || e.metaKey || !isCanvas() && !isD2() || e.target.closest('dialog, [role="dialog"]') || e.target.closest('[data-applet]') !== root) { return; }
            var panes = [el.stage, el.outline, el.inspector, el.text].filter(function (p) { return p.getClientRects().length && getComputedStyle(p).visibility !== 'hidden'; });
            var at = panes.findIndex(function (p) { return p === document.activeElement || p.contains(document.activeElement); });
            var next = panes[(at + (e.shiftKey ? panes.length - 1 : 1) + panes.length) % panes.length];
            if (!next) { return; } e.preventDefault(); e.stopPropagation();
            if (next === el.text) { cm.focus(); }
            else if (next === el.stage) { el.stage.focus(); }
            else { var first = next.querySelector('a[tabindex="0"], button:not(:disabled), input:not(:disabled), textarea:not(:disabled), select:not(:disabled)'); if (first) { first.focus(); } else { next.tabIndex = -1; next.focus(); } }
        }, true);
        function followCanvasCursor(t, pos) {
            if (!t.cv || t.cvText !== t.text) { return; }
            var range = (t.cvRanges || []).find(function (r) { return pos >= r.from && pos < r.to; });
            if (range && range.kind === 'node' && window.yavcCanvas.isAnchor(t.cvDoc.nodes.find(function (n) { return n.id === range.id; }))) { range = null; }
            var s = t.cv.selected();
            if ((!range && !s) || (range && s && range.id === s.id && range.kind === s.kind && canvasSelections().length === 1)) { return; }
            t.cv.select(range ? { id: range.id, kind: range.kind } : null, 'cursor');
            if (range) { t.cv.reveal(); }
        }
        function canvasWhy(needs) {
            var t = current(), s = canvasSelection();
            if (!t || t.kind !== 'canvas') { return 'Open a canvas first.'; }
            if (!t.cv || t.cvText !== t.text || (t.problems && t.problems.length)) { return 'Wait for a valid drawing before editing the canvas.'; }
            if (needs && !s) { return 'Select a node or connector first.'; }
            if (needs === 'nodes' && !canvasSelections().some(function (s) { return s.kind === 'node'; })) { return 'Select one or more nodes first.'; }
            if ((needs === 'align' || needs === 'distribute') && (canvasSelections().some(function (s) { return s.kind !== 'node'; }) || canvasSelections().length < (needs === 'align' ? 2 : 3))) { return 'Select at least ' + (needs === 'align' ? 'two' : 'three') + ' nodes.'; }
            if (needs === 'shape' && (canvasSelections().length !== 1 || !s || s.kind !== 'node')) { return 'Select one node first.'; }
            if (needs === 'text' && (canvasSelections().length !== 1 || !s || s.kind !== 'node' || !t.cvDoc.nodes.some(function (n) { return n.id === s.id && n.type === 'text' && !window.yavcCanvas.isAnchor(n); }))) { return 'Select one text shape first.'; }
            if (needs === 'edge' && (canvasSelections().length !== 1 || !s || s.kind !== 'edge')) { return 'Select one connector first.'; }
            return null;
        }
        async function canvasEdit(req, expected, strict, userEvent) {
            var t = current();
            try {
                requireEditable(t);
                if (!t || t.kind !== 'canvas' || (expected != null && expected !== t.text)) { throw new Error('The canvas changed before the edit finished. Try again on the current drawing.'); }
                var J = window.yavcCanvas, M = window.yavcCanvasModel, got = J.read(t.text);
                if (!got.doc) { throw new Error('Put the mistake in the canvas text right before editing the drawing.'); }
                var out = M.edit(got.doc, req);
                if (out.text === t.text) { return out; }
                var isEdge = out.doc.edges.some(function (x) { return x.id === out.key; }) && (req.op === 'connect' || req.op === 'reconnect' || req.kind === 'edge');
                t.cvPending = { text: out.text, selection: out.selection || (out.key ? { kind: isEdge ? 'edge' : 'node', id: out.key } : null) };
                replaceText(out.text, userEvent || 'input.canvas');
                await drawNow();
                if (t === current()) { say('Changed the canvas. Undo puts it back.'); }
                return out;
            } catch (err) {
                if (strict) { throw err; }
                say('The canvas could not be changed. ' + err.message, 'problem');
                return null;
            }
        }
        function canvasRequest(req) {
            var s = canvasSelection();
            if (canvasSelections().length > 1 && ['move', 'delete', 'align', 'distribute'].indexOf(req.op) >= 0) { return canvasEdit(Object.assign({ items: canvasSelections() }, req)); }
            return canvasEdit(Object.assign({ key: s ? s.id : '', kind: s ? s.kind : 'node' }, req));
        }
        function canvasCreate(x, y, type) {
            var t = current();
            if (canvasWhy(false)) { return; }
            var p = t.cv.centre(), shape = tool || 'rectangle';
            setTool(null);
            return canvasEdit({ op: 'create', type: type || 'text', shape: shape, x: x == null ? p.x : x, y: y == null ? p.y : y });
        }
        function canvasConnect() {
            if (canvasWhy('shape')) { return; }
            var t = current(), s = canvasSelection(), text = t.text;
            var targets = t.cvDoc.nodes.filter(function (x) { return x.id !== s.id && !window.yavcCanvas.isAnchor(x); }).map(function (x) { return x.id; });
            if (!targets.length) { say('Add another shape before connecting.',true); return; }
            window.yavcDialog.askText('Connect to a Node', 'Target Node', targets[0] || '', {
                signal:lifetime.signal, choices:targets.map(function (id) { var node=t.cvDoc.nodes.find(function (node) {return node.id===id;}); return [id,(node.text || node.label || id).replace(/\s+/g,' ').slice(0,100) + ' (' + id + ')']; }),
                okLabel: 'Connect', help: 'Change either anchor in the Inspector after connecting.',
                validate: function (v) { return t !== current() || t.text !== text ? 'The diagram changed. Close and reopen this form.' : targets.indexOf(v) < 0 ? 'Choose another node in this canvas.' : null; }
            }).then(function (v) { if (v != null && t === current()) { canvasEdit({ op: 'connect', from: s.id, to: v }, text); } });
        }
        function canvasResize() {
            if (canvasWhy('shape')) { return; }
            var t = current(), s = canvasSelection(), text = t.text, node = t.cvDoc.nodes.find(function (x) { return x.id === s.id; });
            var prefix='dg-size-'+crypto.randomUUID(), values;
            return window.yavcDialog.open({title:'Resize Shape',signal:lifetime.signal,
                body:numberField(prefix+'-width','width','Width (px)',String(node.width),1,Number.MAX_SAFE_INTEGER)+numberField(prefix+'-height','height','Height (px)',String(node.height),1,Number.MAX_SAFE_INTEGER),
                buttons:[{id:'cancel',label:'Cancel'},{id:'apply',label:'Resize',primary:true}],
                onOpen:function (api) {api.dialog.querySelector('form').noValidate=true;api.body.addEventListener('input',clearDialogNumberError,true);},
                check:function (choice,body) {
                    if(t!==current() || t.text!==text || readOnly(t)) { window.yavcUtil.notice(body,'The diagram changed. Close and reopen this form.',true);return true; }
                    values={};var invalid=[];
                    ['width','height'].forEach(function (key) {var input=body.querySelector('[data-set="'+key+'"]'),target=visibleField(input),value=Number(input.value);values[key]=value;var bad=target.getAttribute('aria-invalid')==='true'||!Number.isSafeInteger(value)||value<1;markField(input,bad?'Enter a positive whole number.':null);if(bad){input.dataset.dialogNumberError='true';invalid.push(target);}});
                    if(invalid.length)invalid[0].focus();return invalid.length>0;
                }
            }).then(function (answer) {if(answer.value==='apply' && t===current() && t.text===text)canvasEdit({op:'resize',key:s.id,width:values.width,height:values.height},text);});
        }
        function canvasReconnect(end) {
            if (canvasWhy('edge')) { return; }
            var t = current(), s = canvasSelection(), text = t.text, J = window.yavcCanvas;
            var edge = t.cvDoc.edges.find(function (e) { return e.id === s.id; }), node = t.cvDoc.nodes.find(function (x) { return x.id === edge[end + 'Node']; });
            var route = J.route(edge, t.cvDoc.nodes), point = route.points[end === 'from' ? 0 : route.points.length - 1], p = 'dg-end-' + n + '-';
            var options = [['', 'Free Point']].concat(t.cvDoc.nodes.filter(function (x) { return !J.isAnchor(x); }).map(function (x) { return [x.id, x.id]; }));
            var body = selectField(p + 'node', 'target', 'Attach To', J.isAnchor(node) ? '' : node.id, options) +
                selectField(p + 'side', 'side', 'Anchor', J.automatic(edge,end) ? '' : edge[end + 'Side'], [['', 'Automatic'], ['top', 'Top'], ['right', 'Right'], ['bottom', 'Bottom'], ['left', 'Left']]) +
                numberField(p + 'x', 'x', 'X', String(Math.round(point[0])), null, null) +
                numberField(p + 'y', 'y', 'Y', String(Math.round(point[1])), null, null) +
                '<p class="form-help">Automatic chooses the nearest side as the shapes move. Free Point places the endpoint at X and Y.</p>';
            function read(body) {
                var get = function (k) { return body.querySelector('[data-set="' + k + '"]').value; };
                return { op: 'reconnect', kind: 'edge', key: s.id, end: end, node: get('target') || null, side: get('target') ? get('side') || null : null, x: get('x') === '' ? NaN : Number(get('x')), y: get('y') === '' ? NaN : Number(get('y')) };
            }
            return window.yavcDialog.open({ title: end === 'from' ? 'Reconnect the Start' : 'Reconnect the End', body: body, signal:lifetime.signal,
                buttons: [{ id: 'cancel', label: 'Cancel' }, { id: 'apply', label: 'Reconnect', primary: true }],
                check: function (value, body) {
                    if(t!==current() || t.text!==text || readOnly(t)) {window.yavcUtil.notice(body,'The diagram changed. Close and reopen this form.',true);return true;}
                    var req = read(body);
                    var invalid = [];
                    ['x', 'y'].forEach(function (k) {
                        var field = body.querySelector('[data-set="' + k + '"]'), bad = !req.node && (visibleField(field).getAttribute('aria-invalid')==='true'||!Number.isSafeInteger(req[k]));
                        markField(field, bad ? 'Give a whole-number coordinate within the safe numeric range.' : null);
                        if(bad)field.dataset.dialogNumberError='true';
                        if (bad) { invalid.push(field); }
                    });
                    if (invalid.length) {
                        var label = invalid[0].closest('.form-group').querySelector('label');
                        (label && label.control ? label.control : invalid[0]).focus();
                    }
                    return invalid.length > 0;
                },
                onOpen: function (o) {
                    o.dialog.querySelector('form').noValidate = true;
                    o.body.addEventListener('input',clearDialogNumberError,true);
                    function changed() {
                        var free = !o.body.querySelector('[data-set="target"]').value;
                        ['x', 'y'].forEach(function (k) { o.body.querySelector('[data-set="' + k + '"]').disabled = !free; });
                        o.body.querySelector('[data-set="side"]').disabled = free;
                    }
                    o.body.querySelector('[data-set="target"]').addEventListener('change', changed); changed();
                    o.body.addEventListener('input', function (e) {
                        if (!e.target.matches('[data-set="x"], [data-set="y"]')) { return; }
                        if (e.target.value !== '' && Number.isSafeInteger(Number(e.target.value))) { markField(e.target, null); }
                    });
                }
            }).then(function (out) { if (out.value === 'apply' && t === current()) { return canvasEdit(read(out.body), text); } });
        }
        function canvasCopy() {
            if (canvasWhy('shape')) { return; }
            var t = current(), s = canvasSelection(), fragment = window.yavcCanvasModel.fragment(t.cvDoc, s.id, s.kind);
            return navigator.clipboard.writeText(window.yavcCanvas.write(fragment)).then(function () { say('Copied the node and its contained nodes.'); }, function (err) { say('The selection could not be copied. ' + err.message, 'problem'); });
        }
        async function canvasPaste() {
            if (canvasWhy(false)) { return; }
            var t = current(), text = t.text;
            try {
                var clip = await navigator.clipboard.readText();
                if (t !== current()) { return; }
                await canvasEdit({ op: 'paste', text: clip }, text);
            } catch (err) { say('The selection could not be pasted. ' + err.message, 'problem'); }
        }
        function renderCanvasInspector(t) {
            renderOutline();
            var selected = canvasSelections(), show = !!(settings.inspector && t.cv);
            inspectorPresentation(show);
            if (!show) { inspected = null; return; }
            var blocked = !!(canvasWhy(false) || mutationWhy(t)), state = 'canvas|' + JSON.stringify(selected) + '|' + blocked;
            if (inspected === state && el.inspector.contains(document.activeElement)) { return; }
            inspected = state;
            if (!selected.length) { renderDocumentInspector(t, blocked); return; }
            var entries = selected.map(function (s) { return { kind: s.kind, item: (s.kind === 'node' ? t.cvDoc.nodes : t.cvDoc.edges).find(function (x) { return x.id === s.id; }) }; });
            if (entries.some(function (e) { return !e.item; })) { return; }
            var properties = entries.map(function (e) { return window.yavcCanvasModel.properties(e.item, e.kind); }).reduce(function (a, b) { return a.filter(function (k) { return b.indexOf(k) >= 0; }); });
            var p = 'dg-cv-in-' + n + '-', had = el.inspector.contains(document.activeElement) ? document.activeElement.id : null;
            var html = '<h3 class="dg-inspector-title">' + (selected.length > 1 ? selected.length + ' Items' : selected[0].kind === 'node' ? 'Node' : 'Connector') + '</h3><p class="dg-inspector-id">' + esc(selected.map(function (s) { return s.id; }).join(', ')) + '</p>';
            var defaults = { shape: 'rectangle', line: 'solid', lineWidth: 2, route: 'elbow' };
            function value(e, k) {
                var ext = e.item.yavc || {};
                if (k === 'fromHead') { return ext.fromHead || e.item.fromEnd || 'none'; }
                if (k === 'toHead') { return ext.toHead || e.item.toEnd || 'arrow'; }
                if (e.kind === 'edge' && (k === 'fromSide' || k === 'toSide')) { return window.yavcCanvas.automatic(e.item,k.slice(0,-4)) ? '' : e.item[k]; }
                var v = ['shape', 'line', 'lineWidth', 'route', 'textAlign', 'textVerticalAlign', 'fillColor', 'strokeColor'].indexOf(k) >= 0 ? ext[k] : e.item[k];
                return v == null ? (k === 'lineWidth' && e.item.type === 'group' ? 1 : defaults[k] == null ? '' : defaults[k]) : v;
            }
            function add(k, label, type, options) {
                if (properties.indexOf(k) < 0) { return; }
                var values = entries.map(function (e) { return String(value(e, k)); }), mixed = values.some(function (v) { return v !== values[0]; }), v = mixed ? null : values[0], id = p + k;
                var control;
                if (type === 'segments') { control = segmentField(id, k, label, v, options); }
                else if (type === 'picker') { control = pickerField(id, k, label, v, options); }
                else if (type === 'select') { control = selectField(id, k, label, mixed ? '__mixed__' : v, mixed ? [['__mixed__', 'Mixed']].concat(options) : options); }
                else if (type === 'number') { control = numberField(id, k, label, v || '', k === 'x' || k === 'y' ? null : 1, k === 'lineWidth' ? 8 : null); }
                else if (type === 'text') { control = field(id, label, '<textarea class="form-input" id="' + id + '" data-set="' + k + '" data-was="' + esc(v || '') + '" rows="3">' + esc(v || '') + '</textarea>'); }
                else if (type === 'color') {
                    var pal = canvasPalette(t.place, window.yavcCanvas), resolved = /^[1-6]$/.test(v || '') ? pal.presets[Number(v) - 1] : /^#[a-f0-9]{3}$/i.test(v || '') ? v.replace(/([a-f0-9])/ig, '$1$1').replace('##', '#') : v;
                    var box = (t.cvColors || []).find(function (b) { return b.id === selected[0].id; });
                    var fallback = box && box.colors && (k === 'fillColor' ? box.colors.fill : k === 'strokeColor' ? box.colors.stroke : null);
                    control = colourField(id, k, label, v || '', resolved || fallback || (k === 'fillColor' ? pal.surface : pal.line)) + '<div class="hstack dg-color-presets" role="group" aria-label="' + esc(label) + ' Presets">' + pal.presets.map(function (c, i) {
                        var name = ['Red', 'Orange', 'Yellow', 'Green', 'Cyan', 'Purple'][i];
                        return '<button type="button" class="btn btn-sm" data-set="' + esc(k) + '" value="' + (i + 1) + '" data-was="' + esc(mixed ? '__mixed__' : v) + '" aria-label="' + name + '" title="' + name + '"><span class="dg-color-dot" style="background:' + esc(c) + '" aria-hidden="true"></span></button>';
                    }).join('') + '<button type="button" class="btn btn-sm" data-set="' + esc(k) + '" value="" title="' + (k === 'color' ? 'Theme Default' : 'Use the Coordinated Color') + '" data-was="' + esc(mixed ? '__mixed__' : v) + '">Default</button></div>';
                } else { control = textField(id, k, label, v); }
                if (mixed && type !== 'segments' && type !== 'picker') { control = control.replace(/data-was="[^"]*"/g, 'data-mixed="true" data-was=""').replace(/ placeholder="[^"]*"/g, '').replace(/<(input|textarea)\b/g, '<$1 placeholder="Mixed"'); control += '<p class="form-help">Mixed values</p>'; }
                html += control;
            }
            add('text', 'Text', 'text'); add('label', 'Label');
            add('shape', 'Shape', 'select', [['rectangle', 'Rectangle'], ['rounded', 'Rounded Rectangle']].concat(PALETTE.filter(function (x) { return x[0] !== 'rectangle' && x[0] !== 'rounded'; })));
            add('textAlign', 'Horizontal Alignment', 'segments', [['', 'Default', 'yavc-align-default'], ['left', 'Left', 'yavc-align-left'], ['center', 'Center', 'yavc-align-center'], ['right', 'Right', 'yavc-align-right']]);
            add('textVerticalAlign', 'Vertical Alignment', 'segments', [['', 'Default', 'yavc-align-default'], ['top', 'Top', 'yavc-align-top'], ['middle', 'Middle', 'yavc-align-middle'], ['bottom', 'Bottom', 'yavc-align-bottom']]);
            ['x', 'y', 'width', 'height'].forEach(function (k) { add(k, k === 'x' ? 'X' : k === 'y' ? 'Y' : k === 'width' ? 'Width' : 'Height', 'number'); });
            add('color', 'Color', 'color'); add('line', 'Line Style', 'segments', [['solid', 'Solid', 'yavc-line-solid'], ['dashed', 'Dashed', 'yavc-line-dashed'], ['dotted', 'Dotted', 'yavc-line-dotted']]); add('lineWidth', 'Line Width', 'number');
            add('fillColor', 'Fill Color', 'color'); add('strokeColor', 'Outline Color', 'color');
            add('route', 'Route', 'segments', [['elbow', 'Elbow', 'yavc-route-elbow'], ['straight', 'Straight', 'yavc-route-straight'], ['curve', 'Curve', 'yavc-route-curve']]);
            var heads = [['none', 'None'], ['arrow', 'Arrow'], ['triangle', 'Triangle'], ['diamond', 'Diamond'], ['circle', 'Circle'], ['box', 'Box'], ['cross', 'Cross'], ['cf-one', 'One'], ['cf-one-required', 'Exactly One'], ['cf-many', 'Many'], ['cf-many-required', 'One or More']];
            heads = heads.map(function (h) { return h.concat('yavc-head-' + h[0]); });
            add('fromHead', 'Start Head', 'picker', heads); add('toHead', 'End Head', 'picker', heads);
            var sides = [['', 'Automatic'], ['top', 'Top'], ['right', 'Right'], ['bottom', 'Bottom'], ['left', 'Left']];
            add('fromSide', 'Start Side', 'select', sides); add('toSide', 'End Side', 'select', sides);
            if (selected.length === 1 && selected[0].kind === 'edge') { html += '<fieldset class="form-fieldset"><legend>Endpoints</legend><div class="hstack"><button type="button" class="btn btn-sm" data-command="cv-reconnect-start"></button><button type="button" class="btn btn-sm" data-command="cv-reconnect-end"></button></div></fieldset>'; }
            el.inspector.innerHTML = '<fieldset class="dg-inspector-fields"' + (blocked ? ' disabled' : '') + '>' + html + '</fieldset>';
            if (blocked) { el.inspector.insertAdjacentHTML('afterbegin', '<p class="form-help">Put the canvas text right before changing these settings.</p>'); }
            if (window.pudlFields) { window.pudlFields.enhance(el.inspector); }
            if (had) { var back = document.getElementById(had); if (back && !back.disabled) { back.focus({ preventScroll: true }); } }
        }
        function renderDocumentInspector(t, blocked) {
            var id = 'dg-doc-' + n + '-', settings = t.cvDoc.yavc || {}, palette = canvasPalette(t.place, window.yavcCanvas);
            var had = el.inspector.contains(document.activeElement) ? document.activeElement.id : null;
            var value = settings.background || '', resolved = /^[1-6]$/.test(value) ? palette.presets[Number(value) - 1] : /^#[a-f0-9]{3}$/i.test(value) ? value.replace(/([a-f0-9])/ig, '$1$1') : value;
            var color = colourField(id + 'background', 'background', 'Background', value, resolved || palette.bg).replace(/ data-set=/g, ' data-document data-set=');
            var html = '<h3 class="dg-inspector-title">Diagram</h3><p class="form-help">Canvas settings are saved in this diagram.</p>' + color +
                '<button type="button" class="btn btn-sm" id="' + id + 'reset" data-document data-set="background" value="">Theme Default</button>';
            [['grid', 'Show Grid', !!settings.grid], ['snap', 'Snap to Grid and Shapes', settings.snap !== false]].forEach(function (setting) {
                html += '<div class="form-group"><label class="check"><input type="checkbox" id="' + id + setting[0] + '" data-document data-set="' + setting[0] + '"' + (setting[2] ? ' checked' : '') + '> ' + setting[1] + '</label></div>';
            });
            html += '<p class="form-help">Grid spacing is 8 pixels. Hold Alt while dragging to bypass snapping. The grid is not exported.</p>';
            el.inspector.innerHTML = '<fieldset class="dg-inspector-fields"' + (blocked ? ' disabled' : '') + '>' + html + '</fieldset>';
            if (had) { var back = document.getElementById(had); if (back && !back.disabled) { back.focus({ preventScroll: true }); } }
        }
        async function canvasInspectorChanged(f) {
            if (!canvasSelections().length && f.hasAttribute('data-document')) {
                if (canvasWhy(false)) { return; }
                var documentOwner = current(), documentValue = f.type === 'checkbox' ? f.checked : f.value || null;
                try {
                    var changed = await canvasEdit({ op: 'document', property: f.dataset.set, value: documentValue }, documentOwner.text, true);
                    if (current() === documentOwner && documentOwner.text === changed.text) { inspected = null; renderCanvasInspector(documentOwner); }
                }
                catch (err) { if (current() === documentOwner && f.isConnected) { markField(f, err.message); } }
                return;
            }
            if (canvasWhy(true)) { return; }
            var owner = current(), property = f.dataset.set, value = f.value, s = canvasSelection();
            var picker = f.dataset.picker, restoreFocus = picker && document.activeElement === f;
            if (f.tagName === 'SELECT' && value === '__mixed__') { return; }
            if (!f.hasAttribute('data-mixed') && value === f.dataset.was) { return; }
            var req = { op: 'set', key: s.id, kind: s.kind, property: property, value: property === 'text' ? value : value || null };
            if (['x', 'y', 'width', 'height'].indexOf(property) >= 0) { req = { op: 'resize', key: s.id }; req[property] = value.trim() ? Number(value) : NaN; }
            if (property === 'lineWidth') { req.value = value.trim() ? Number(value) : NaN; }
            if (canvasSelections().length > 1) { req.items = canvasSelections(); }
            try {
                var result = await canvasEdit(req, owner.text, true);
                if (destroyed || owner !== current() || owner.text !== result.text) { return; }
                f.dataset.was = value; markField(f, null); inspected = null; renderCanvasInspector(owner);
                if (restoreFocus && (document.activeElement === document.body || document.activeElement === f || document.activeElement.id === picker)) {
                    var trigger = document.getElementById(picker); if (trigger) { trigger.focus({ preventScroll: true }); }
                }
            } catch (err) { if (!destroyed && owner === current() && f.isConnected) { markField(f, err.message); } }
        }
        function canvasCommands() {
            function c(id, label, run, key, needs, glyph) {
                var changes = !['select-all','select-items','copy','next','previous','zoom-in','zoom-out','actual','pan'].includes(id) && !id.startsWith('pan-');
                function reason() { return canvasWhy(needs) || (changes ? mutationWhy() : null); }
                return cmd('cv-' + id, label, glyph, run, { hidden: function () { return !isCanvas(); }, builtIn: key, disabled: function () { return !!reason(); }, reason: reason });
            }
            var commands = [
                c('edit-text', 'Edit Text', startInlineText, 'Enter', 'text', 'pencil'),
                c('select-all', 'Select All Items', canvasSelectAll, 'Mod-a', false),
                c('select-items', 'Select Items…', canvasSelectItems, null, false),
                c('new', 'Add Shape', function () { canvasCreate(); }, 'n', false, 'plus'),
                c('group', 'Add Group', function () { canvasCreate(null, null, 'group'); }, 'g', false, 'folder'),
                c('connect', 'Connect to a Node…', canvasConnect, 'c', 'shape', 'yavc-connect'),
                c('reconnect-start', 'Reconnect Start…', function () { return canvasReconnect('from'); }, 'Shift-e', 'edge', 'yavc-connect'),
                c('reconnect-end', 'Reconnect End…', function () { return canvasReconnect('to'); }, 'e', 'edge', 'yavc-connect'),
                c('resize', 'Resize…', canvasResize, 'r', 'shape'),
                c('copy', 'Copy Node', canvasCopy, 'Mod-c', 'shape', 'copy'),
                c('paste', 'Paste Nodes', canvasPaste, 'Mod-v', false),
                c('duplicate', 'Duplicate', function () { canvasRequest({ op: 'duplicate' }); }, 'Mod-d', 'shape', 'copy'),
                c('forward', 'Bring Forward', function () { canvasRequest({ op: 'order', direction: 'forward' }); }, 'Mod-]', 'shape'),
                c('back', 'Send Back', function () { canvasRequest({ op: 'order', direction: 'back' }); }, 'Mod-[', 'shape'),
                c('delete', 'Delete', function () { canvasRequest({ op: 'delete' }); }, 'Delete', true, 'yavc-bin'),
                c('next', 'Select Next', function () { current().cv.cycle(1); el.stage.focus(); }, 'Tab', false),
                c('previous', 'Select Previous', function () { current().cv.cycle(-1); el.stage.focus(); }, 'Shift-Tab', false),
                c('zoom-in', 'Zoom In', function () { current().cv.zoom(1.25); }, '+', false, 'plus'),
                c('zoom-out', 'Zoom Out', function () { current().cv.zoom(0.8); }, '-', false, 'minus'),
                c('actual', 'Actual Size', function () { current().cv.actual(); }, '1', false),
                Object.assign(c('pan', 'Pan', function () { setCanvasPan(!canvasPan); }, 'h', false, 'move'), { checked: function () { return canvasPan; } })
            ];
            [['left', 'Left', -1, 0], ['right', 'Right', 1, 0], ['up', 'Up', 0, -1], ['down', 'Down', 0, 1]].forEach(function (d) {
                commands.push(c(d[0], 'Move ' + d[1], function () { canvasRequest({ op: 'move', dx: d[2], dy: d[3] }); }, 'Arrow' + d[1], 'nodes'));
                commands.push(c('pan-' + d[0], 'Pan ' + d[1], function () { current().cv.pan(d[2] * 80, d[3] * 80); }, 'Alt-Arrow' + d[1], false));
            });
            [['left', 'Left'], ['center', 'Horizontal Centers'], ['right', 'Right'], ['top', 'Top'], ['middle', 'Vertical Centers'], ['bottom', 'Bottom']].forEach(function (d) {
                commands.push(c('align-' + d[0], 'Align ' + d[1], function () { canvasRequest({ op: 'align', direction: d[0] }); }, null, 'align'));
            });
            ['horizontal', 'vertical'].forEach(function (direction) {
                commands.push(c('distribute-' + direction, 'Distribute ' + (direction === 'horizontal' ? 'Horizontally' : 'Vertically'), function () { canvasRequest({ op: 'distribute', direction: direction }); }, null, 'distribute'));
            });
            return commands;
        }
        function canvasKey(e) {
            var t = current(), mod = e.ctrlKey || e.metaKey, key = e.key.toLowerCase();
            if (readOnly(t) && ((mod && ['z','y','v','d','[',']'].includes(key)) || (!mod && !e.altKey && ['n','g','c','e','r','delete','enter'].includes(key)))) { e.preventDefault(); return; }
            if (mod && key === 'z') { e.preventDefault(); if (e.shiftKey) { CM.redo(cm); } else { CM.undo(cm); } return; }
            if (mod && key === 'y') { e.preventDefault(); CM.redo(cm); return; }
            if (e.key === 'Escape') { setTool(null); t.cv.cancel(); t.cv.select(null); e.preventDefault(); return; }
            if (e.key === 'Tab' || e.key === ' ') { if (t.cv.onKey(e)) { e.preventDefault(); } return; }
            if (canvasWhy(false)) { return; }
            if (e.key === 'Enter' && !canvasWhy('text')) { e.preventDefault(); startInlineText(); return; }
            if (mod && key === 'a') { e.preventDefault(); canvasSelectAll(); return; }
            if (/^Arrow/.test(e.key) && !mod) {
                e.preventDefault();
                var dx = e.key === 'ArrowLeft' ? -1 : e.key === 'ArrowRight' ? 1 : 0, dy = e.key === 'ArrowUp' ? -1 : e.key === 'ArrowDown' ? 1 : 0;
                if (e.altKey) { t.cv.pan(dx * 80, dy * 80); }
                else if (!readOnly(t) && !canvasWhy('nodes')) { canvasRequest({ op: 'move', dx: dx * (e.shiftKey ? 8 : 1), dy: dy * (e.shiftKey ? 8 : 1) }); }
                return;
            }
            if (mod && key === 'c') { e.preventDefault(); canvasCopy(); }
            else if (mod && key === 'v') { e.preventDefault(); canvasPaste(); }
            else if (mod && key === 'd' && !canvasWhy('shape')) { e.preventDefault(); canvasRequest({ op: 'duplicate' }); }
            else if (mod && (key === '[' || key === ']') && !canvasWhy('shape')) { e.preventDefault(); canvasRequest({ op: 'order', direction: key === '[' ? 'back' : 'forward' }); }
            else if (!mod && !e.altKey) {
                if (key === 'n') { e.preventDefault(); canvasCreate(); }
                else if (key === 'g') { e.preventDefault(); canvasCreate(null, null, 'group'); }
                else if (key === 'c') { e.preventDefault(); canvasConnect(); }
                else if (key === 'e') { e.preventDefault(); canvasReconnect(e.shiftKey ? 'from' : 'to'); }
                else if (key === 'r') { e.preventDefault(); canvasResize(); }
                else if (key === 'h') { e.preventDefault(); setCanvasPan(!canvasPan); }
                else if (key === 'delete' && !canvasWhy(true)) { e.preventDefault(); canvasRequest({ op: 'delete' }); }
                else if (key === '+' || key === '=') { e.preventDefault(); t.cv.zoom(1.25); }
                else if (key === '-') { e.preventDefault(); t.cv.zoom(0.8); }
                else if (key === '0') { e.preventDefault(); t.cv.fit(); }
                else if (key === '1') { e.preventDefault(); t.cv.actual(); }
            }
        }
        /* The colours of a canvas, read from the page's tokens where it is
           drawn, as a D2 diagram's are (docs/diagrams-design.md, decision
           12), with the format's six presets in a set readable on the
           background. Every value is a plain #rrggbb. */
        function canvasPalette(place, J) {
            var probe = document.createElement('span');
            probe.setAttribute('aria-hidden', 'true');
            probe.style.cssText = 'position:absolute;visibility:hidden;pointer-events:none';
            place.appendChild(probe);
            function read(css, prop) {
                probe.style.color = ''; probe.style.backgroundColor = '';
                probe.style[prop || 'color'] = css;
                var m = /rgba?\((\d+(?:\.\d+)?),\s*(\d+(?:\.\d+)?),\s*(\d+(?:\.\d+)?)/.exec(getComputedStyle(probe)[prop || 'color']);
                return m ? '#' + [m[1], m[2], m[3]].map(function (v) { return ('0' + Math.round(+v).toString(16)).slice(-2); }).join('') : null;
            }
            var pal = { text: read('var(--text)') || '#1a1a1a', muted: read('var(--text-muted)') || '#5c5c5c', line: read('var(--text-muted)') || '#5c5c5c',
                surface: read('var(--surface)', 'backgroundColor') || '#ffffff', bg: read('var(--bg)', 'backgroundColor') || '#ffffff', accent: read('var(--accent)') || '#0057b8' };
            probe.remove();
            /* The colour behind the drawing, from the nearest ancestor that
               paints a solid one, as d2.js finds it, so the canvas has no
               box of its own. */
            for (var a = place; a && a.nodeType === 1; a = a.parentElement) {
                /* The live canvas paints its document override here. Theme
                   defaults must come from the host behind that canvas. */
                if (a === place && a.classList.contains('dg-cv-editor')) { continue; }
                var m = /^rgba?\((\d+),\s*(\d+),\s*(\d+)(?:,\s*([\d.]+))?\)$/.exec(getComputedStyle(a).backgroundColor);
                if (m && (m[4] === undefined || +m[4] === 1)) { pal.bg = '#' + [m[1], m[2], m[3]].map(function (v) { return ('0' + (+v).toString(16)).slice(-2); }).join(''); break; }
                if (m && +m[4] > 0) { break; }
            }
            var rgb = [1, 3, 5].map(function (i) { return parseInt(pal.bg.substr(i, 2), 16) / 255; });
            var light = 0.2126 * rgb[0] + 0.7152 * rgb[1] + 0.0722 * rgb[2] > 0.45;
            pal.presets = (light ? J.PRESETS.light : J.PRESETS.dark).slice();
            return pal;
        }
        /* Each text node's Markdown, drawn as HTML in the place of its SVG
           text once the Markdown engine is here. Applets are not started
           inside a node, so their attributes are taken away. */
        function richText(t, doc, out, svg, palette, isCurrent) {
            var texts = doc.nodes.filter(function (nd) { return nd.type === 'text' && nd.text; });
            if (!texts.length) { return; }
            mdEngine().then(function (md) {
                if (!isCurrent() || !svg.isConnected) { return; }
                var byId = {};
                out.boxes.forEach(function (b) { byId[b.id] = b; });
                texts.forEach(function (nd) {
                    var box = byId[nd.id] && byId[nd.id].text, g = svg.querySelector('g[data-text="' + CSS.escape(nd.id) + '"]');
                    if (!box || !g) { return; }
                    var html = md.source(nd.text).html;
                    var frag = window.yavcHTML.fragment(html);
                    frag.querySelectorAll('[data-applet]').forEach(function (a) {
                        Array.prototype.slice.call(a.attributes).forEach(function (at) { if (/^data-applet/i.test(at.name)) { a.removeAttribute(at.name); } });
                    });
                    /* The drawing is one labelled picture, so a link in it is
                       its words only, as in the exported picture: a link
                       inside a picture cannot be reached properly, and
                       following one would leave the page. */
                    frag.querySelectorAll('a').forEach(function (a) {
                        var span = document.createElement('span');
                        while (a.firstChild) { span.appendChild(a.firstChild); }
                        a.replaceWith(span);
                    });
                    var fo = document.createElementNS('http://www.w3.org/2000/svg', 'foreignObject');
                    fo.setAttribute('x', box.x); fo.setAttribute('y', box.y); fo.setAttribute('width', box.w); fo.setAttribute('height', box.h);
                    var div = document.createElement('div');
                    div.className = 'dg-cv-text' + (nd.yavc && nd.yavc.textAlign ? '' : ' is-auto-align');
                    div.style.textAlign = box.align;
                    div.style.justifyContent = box.vertical === 'middle' ? 'center' : box.vertical === 'bottom' ? 'flex-end' : 'flex-start';
                    div.style.color = box.color || palette.text;
                    div.appendChild(frag);
                    fo.appendChild(div);
                    g.replaceChildren(fo);
                });
            }).catch(function (err) { say('The text in the canvas is shown plainly, because the Markdown engine did not load: ' + ((err && err.message) || err), 'warn'); });
        }
        function applyFit() {
            el.canvas.classList.toggle('dg-fit', !!settings.fit);
            var t = current();
            if (t && t.canvas) { t.canvas.fit(); }
            if (t && t.cv) { t.cv.fit(); }
        }

        /* === The visual editor (milestone D3) =============================
           The drawing of a D2 diagram is a canvas: a layer over it
           (canvas.js) knows where each shape and connection is, so a click
           or a key selects one, its text is selected beside it, and every
           change is one of D2's own edits to the text (decision 5), which
           Undo takes back like any typing. */
        async function showCanvas(t, out, text) {
            try {
                var parts = await Promise.all([canvasLib(), d2().then(function (E) { return E.call('outline', {text:text}, 'outline-' + n); })]);
                if (destroyed || t !== current() || t.text !== text) { return; }
                var C = parts[0], o = parts[1];
                t.outline = o; t.outlineText = text;
                if (!t.canvas) {
                    t.canvas = C.attach(t.place, {
                        selected: function (key, how) { canvasSelected(t, key, how); },
                        readOnly: function () { return readOnly(t); },
                        activate: function (key) { if (t === current()) { editLabel(key); } },
                        pick: function (key) { return t === current() && pickEnd(key); },
                        tool: function () { return tool; },
                        place: function (container) { if (t === current()) { placeShape(container); } },
                        drop: function (d) { if (t === current()) { dropped(d); } }
                    });
                }
                t.canvas.show(out);
                if (t.pending != null) { var k = t.pending; t.pending = null; selectKey(t, k, 'edit'); }
                commandsChanged(); renderInspector();
            } catch (err) { if (!destroyed && t === current() && t.text === text) say('The drawing cannot be edited here: ' + ((err && err.message) || err), true); }
        }
        function isConnection(key) { var t = current(); return !!(t && t.outline && (t.outline.connections || []).some(function (c) { return c.id === key; })); }
        function shapeOf(t, key) { return ((t.outline && t.outline.shapes) || []).find(function (s) { return s.id === key; }) || null; }
        function connectionOf(t, key) { return ((t.outline && t.outline.connections) || []).find(function (c) { return c.id === key; }) || null; }
        function describe(t, key) {
            if (isConnection(key)) {
                var c = connectionOf(t, key);
                return c ? 'the connection from ' + c.from + ' to ' + c.to + (c.label ? ', "' + c.label + '"' : '') : 'a connection';
            }
            var s = shapeOf(t, key);
            return s ? '"' + (s.label || s.id) + '" (' + s.id + ', ' + s.shape.replace(/_/g, ' ') + ')' : key;
        }
        function selection() { var t = current(); return t && t.canvas ? t.canvas.selected() : null; }
        function canEdit() { var t = current(); return !!(t && !readOnly(t) && t.kind === 'd2' && t.canvas && !(t.problems && t.problems.length)); }
        /* A selection on the canvas selects the shape's text beside it,
           without taking the keyboard from the drawing. */
        function canvasSelected(t, key, how) {
            if (t !== current()) { return; }
            if (how === 'pointer') { el.stage.focus({ preventScroll: true }); }
            if (key) {
                /* The cursor in the text selects quietly: the text pane says
                   where the cursor is, and a screen reader would otherwise
                   hear an announcement at every shape it passes. */
                if (how !== 'cursor') { say('Selected ' + describe(t, key) + '.'); }
                var item = isConnection(key) ? connectionOf(t, key) : shapeOf(t, key), at = item && item.places && item.places[0];
                if (at && cm && how !== 'cursor') {
                    quiet = true;
                    try { cm.dispatch({ selection: { anchor: at.from, head: at.to }, effects: CM.EditorView.scrollIntoView(at.from, { y: 'center' }) }); } finally { quiet = false; }
                }
            } else if (how !== 'redraw') { say(''); }
            commandsChanged();
            renderInspector();
            if (how !== 'redraw') announce();
        }
        /* === The inspector ================================================
           The selected shape's or connection's settings as a form of
           PUDL's sunken fields (decision 15), beside the drawing. Each field
           is one of D2's settings, changed as one edit when the field is
           left or Enter is pressed, which Undo takes back; an empty field
           takes the setting away. The form keeps the keyboard where it was
           when it is drawn again, and is not drawn again at all while a
           field has the keyboard, so a drawing that finishes never takes
           the person's typing away. A change D2 refuses is marked at its
           field. While the text has a mistake the fields are disabled, with
           the reason above them (decision 5). */
        var inspected = null;
        function field(id, label, control, help) {
            return '<div class="form-group"><label class="form-label" for="' + id + '">' + esc(label) + '</label>' + control +
                (help ? '<p class="form-help" id="' + id + '-help">' + esc(help) + '</p>' : '') + '</div>';
        }
        function described(id, help) { return help ? ' aria-describedby="' + id + '-help"' : ''; }
        function textField(id, key, label, value, help) {
            return field(id, label, '<input class="form-input" id="' + id + '" data-set="' + esc(key) + '" data-was="' + esc(value || '') + '" value="' + esc(value || '') + '" autocomplete="off" spellcheck="false"' + described(id, help) + '>', help);
        }
        function selectField(id, key, label, value, options) {
            return field(id, label, '<span class="form-select-wrap"><select class="form-select" id="' + id + '" data-set="' + esc(key) + '" data-was="' + esc(value || '') + '">' +
                options.map(function (o) { return '<option value="' + esc(o[0]) + '"' + (o[0] === (value || '') ? ' selected' : '') + '>' + esc(o[1]) + '</option>'; }).join('') + '</select></span>');
        }
        function segmentField(id, key, label, value, options) {
            return '<fieldset class="form-group dg-choice"><legend class="form-label" id="' + id + '-label">' + esc(label) + '</legend><div class="seg sm">' +
                options.map(function (o, i) { return '<button type="button" id="' + id + '-' + i + '" data-set="' + esc(key) + '" value="' + esc(o[0]) + '" data-was="' + esc(value == null ? '__mixed__' : value) + '" aria-label="' + esc(o[1]) + '" title="' + esc(o[1]) + '" aria-pressed="' + (value != null && o[0] === value) + '">' + optionGlyph(o) + '<span>' + esc(o[1]) + '</span></button>'; }).join('') + '</div>' + (value == null ? '<p class="form-help">Mixed values</p>' : '') + '</fieldset>';
        }
        function optionGlyph(option) { return option[2] ? '<span class="glyph" style="--glyph:var(--glyph-' + esc(option[2]) + ')" aria-hidden="true"></span>' : ''; }
        function pickerField(id, key, label, value, options) {
            var selected = options.find(function (o) { return o[0] === value; }), mixed = value == null;
            return '<div class="form-group"><span class="form-label" id="' + id + '-label">' + esc(label) + '</span><div class="menu dg-property-menu-wrap">' +
                '<button type="button" class="btn menu-btn dg-property-picker" id="' + id + '" popovertarget="' + id + '-menu" aria-haspopup="menu" aria-labelledby="' + id + '-label ' + id + '-value">' +
                (selected ? optionGlyph(selected) : '') + '<span id="' + id + '-value">' + esc(mixed ? 'Mixed' : selected ? selected[1] : value) + '</span></button>' +
                '<div class="menu-panel dg-property-menu" id="' + id + '-menu" popover role="menu" aria-labelledby="' + id + '-label">' + options.map(function (o) {
                    return '<button type="button" class="menu-action" role="menuitemradio" aria-checked="' + (value === o[0]) + '" data-set="' + esc(key) + '" data-picker="' + id + '" value="' + esc(o[0]) + '" ' +
                        (mixed ? 'data-mixed="true" ' : '') + 'data-was="' + esc(value == null ? '' : value) + '"><span class="dg-option-content">' + optionGlyph(o) + '<span>' + esc(o[1]) + '</span></span></button>';
                }).join('') + '</div></div>' + (mixed ? '<p class="form-help">Mixed values</p>' : '') + '</div>';
        }
        function numberField(id, key, label, value, min, max) {
            return field(id, label, '<span class="number-field"><input class="form-input" type="number" step="1"' + (min == null ? '' : ' min="' + min + '"') + (max == null ? '' : ' max="' + max + '"') + ' id="' + id + '" data-set="' + esc(key) + '" data-was="' + esc(value || '') + '" value="' + esc(value || '') + '"></span>');
        }
        function colourField(id, key, label, value, fallback) {
            if (/^#[a-f0-9]{3}$/i.test(fallback || '')) { fallback = fallback.replace(/([a-f0-9])/ig, '$1$1'); }
            return window.yavcMdEditor.colourRows([{ id: id, key: key, label: label, value: value || '', fallback: fallback || '#808080', placeholder: 'Theme default' }])
                .replace(/data-key="/, 'data-was="' + esc(value || '') + '" data-set="');
        }
        function checks(legend, items) {
            return '<fieldset class="form-fieldset"><legend>' + esc(legend) + '</legend><div class="form-options inline">' + items.map(function (c) {
                return '<label class="check"><input type="checkbox" data-set="' + esc(c[0]) + '" data-was="' + (c[2] ? 'true' : '') + '"' + (c[2] ? ' checked' : '') + '> ' + esc(c[1]) + '</label>';
            }).join('') + '</div></fieldset>';
        }
        function dashOf(st) { var d = st['stroke-dash']; return d && d !== '0' ? (+d <= 3 ? '2' : '5') : ''; }
        function renderInspector() {
            renderOutline();
            var t = current(), key = selection();
            if (t && t.kind === 'canvas') { renderCanvasInspector(t); return; }
            var show = !!(settings.inspector && t && t.kind === 'd2' && t.canvas);
            inspectorPresentation(show);
            if (!show) { inspected = null; return; }
            var blocked = !!(readOnly(t) || t.problems && t.problems.length), state = key + '|' + blocked;
            if (el.inspector.contains(document.activeElement) && inspected === state) { return; }
            inspected = state;
            /* Where the keyboard was, to put it back: the fields' ids last
               across a change of key, such as a new direction. */
            var had = el.inspector.contains(document.activeElement) ? document.activeElement.id : null;
            var p = 'dg-in-' + n + '-';
            if (!key) {
                el.inspector.innerHTML = '<p class="dg-inspector-empty">Select a shape or a connection to see its settings here.</p>';
                return;
            }
            var html;
            if (isConnection(key)) {
                var c = connectionOf(t, key) || {}, st = c.style || {}, src = c.source || {}, dst = c.target || {};
                html = '<h3 class="dg-inspector-title">Connection</h3><p class="dg-inspector-id">' + esc((c.from || '') + ' to ' + (c.to || '')) + '</p>' +
                    textField(p + 'label', key, 'Label', c.label) +
                    field(p + 'arrows', 'Direction', '<div class="dg-row"><span class="form-select-wrap"><select class="form-select" id="' + p + 'arrows" data-arrows data-was="' + esc(c.arrows || '->') + '">' +
                        ARROWS.map(function (a) { return '<option value="' + esc(a[0]) + '"' + (a[0] === (c.arrows || '->') ? ' selected' : '') + '>' + esc(a[1] + ' (' + a[0] + ')') + '</option>'; }).join('') +
                        '</select></span><button type="button" class="btn btn-sm" id="' + p + 'reverse" data-reverse>Reverse</button></div>') +
                    selectField(p + 'src', key + '.source-arrowhead.shape', 'Start Arrowhead', src.shape, HEADS) +
                    textField(p + 'srcl', key + '.source-arrowhead', 'Start Label', src.label) +
                    selectField(p + 'dst', key + '.target-arrowhead.shape', 'End Arrowhead', dst.shape, HEADS) +
                    textField(p + 'dstl', key + '.target-arrowhead', 'End Label', dst.label) +
                    colourField(p + 'stroke', key + '.style.stroke', 'Line Color', st.stroke) +
                    segmentField(p + 'dash', key + '.style.stroke-dash', 'Line Style', dashOf(st), DASHES) +
                    numberField(p + 'width', key + '.style.stroke-width', 'Line Width', st['stroke-width'], 1, 15) +
                    checks('Effects', [[key + '.style.animated', 'Animated', st.animated === 'true']]);
            } else {
                var s = shapeOf(t, key) || {}, ss = s.style || {};
                html = '<h3 class="dg-inspector-title">Shape</h3><p class="dg-inspector-id">' + esc(key) + '</p>' +
                    textField(p + 'label', key, 'Label', s.label) +
                    selectField(p + 'kind', key + '.shape', 'Kind', s.shape || 'rectangle', SHAPES) +
                    colourField(p + 'fill', key + '.style.fill', 'Fill', ss.fill) +
                    colourField(p + 'stroke', key + '.style.stroke', 'Line Color', ss.stroke) +
                    segmentField(p + 'dash', key + '.style.stroke-dash', 'Line Style', dashOf(ss), DASHES) +
                    numberField(p + 'width', key + '.style.stroke-width', 'Line Width', ss['stroke-width'], 1, 15) +
                    colourField(p + 'font', key + '.style.font-color', 'Text Color', ss['font-color']) +
                    numberField(p + 'size', key + '.style.font-size', 'Text Size', ss['font-size'], 8, 100) +
                    checks('Text', [[key + '.style.bold', 'Bold', ss.bold === 'true'], [key + '.style.italic', 'Italic', ss.italic === 'true'], [key + '.style.underline', 'Underline', ss.underline === 'true']]) +
                    checks('Effects', [[key + '.style.shadow', 'Shadow', ss.shadow === 'true'], [key + '.style.3d', '3D', ss['3d'] === 'true'], [key + '.style.multiple', 'Multiple', ss.multiple === 'true'], [key + '.style.double-border', 'Double Border', ss['double-border'] === 'true']]) +
                    textField(p + 'icon', key + '.icon', 'Icon', s.icon, 'Give the address of a picture. Diagrams draws only icons written into the diagram, as data: addresses.') +
                    textField(p + 'link', key + '.link', 'Link', s.link) +
                    textField(p + 'tip', key + '.tooltip', 'Tooltip', s.tooltip);
            }
            el.inspector.innerHTML = blocked
                ? '<p class="form-help dg-inspector-blocked" id="' + p + 'blocked">' + (readOnly(t) ? 'This diagram is read-only. Use Save As to make an editable copy.' : 'The text has a mistake in it, so these settings cannot be changed until it is put right.') + '</p><fieldset class="dg-inspector-fields" disabled aria-describedby="' + p + 'blocked">' + html + '</fieldset>'
                : html;
            window.yavcMdEditor.wireColours(el.inspector);
            if (window.pudlFields && window.pudlFields.enhance) { window.pudlFields.enhance(el.inspector); }
            if (had) { var back = document.getElementById(had); if (back && !back.disabled) { back.focus({ preventScroll: true }); } }
        }
        /* A field's name, for what the status line says it did. */
        function labelOf(f) {
            var l = f.id && el.inspector.querySelector('label[for="' + f.id + '"]');
            if (l) { return l.textContent.trim(); }
            var c = f.closest('label');
            return c ? c.textContent.trim() : 'setting';
        }
        /* A change D2 refused is marked at its field, with the reason
           beside it; the next change that works takes the mark away. */
        function visibleField(f) {var group=f.closest('.form-group'),label=group && group.querySelector('label');return label && label.control || f;}
        function clearDialogNumberError(event) {var group=event.target.closest('.form-group'),input=group && group.querySelector('input[data-dialog-number-error]');if(input){markField(input,null);visibleField(input).removeAttribute('aria-invalid');delete input.dataset.dialogNumberError;}}
        function markField(f, problem) {
            var errId = (f.id || 'dg-in-field') + '-error', old = document.getElementById(errId);
            if (old) { old.remove(); }
            var ids = (f.getAttribute('aria-describedby') || '').split(/\s+/).filter(function (x) { return x && x !== errId; });
            if (problem) {
                var p = document.createElement('p');
                p.className = 'form-error'; p.id = errId; p.textContent = problem;
                (f.closest('.form-group, .form-fieldset') || f.parentNode).appendChild(p);
                ids.push(errId);
                f.setAttribute('aria-invalid', 'true');
            } else { f.removeAttribute('aria-invalid'); }
            if (ids.length) { f.setAttribute('aria-describedby', ids.join(' ')); } else { f.removeAttribute('aria-describedby'); }
        }
        /* A field left changed is one edit. The label is the shape or the
           connection itself; an empty label shows a shape's name. A field
           whose value is the one already written does nothing, so Enter and
           the leaving after it make one edit, not two. */
        function inspectorChanged(f) {
            var t = current(), key = selection();
            if (t && t.kind === 'canvas') { canvasInspectorChanged(f); return; }
            if (!t || !key || !canEdit()) { return; }
            var now = f.type === 'checkbox' ? (f.checked ? 'true' : '') : f.value.trim();
            if (now === (f.getAttribute('data-was') || '')) { return; }
            var name = labelOf(f), finish = function (out, msg) {
                if (!out) { return; }
                if (out.problems && out.problems.length) { markField(f, 'D2 would not take that: ' + out.problems[0].message); return; }
                f.setAttribute('data-was', now);
                var segment = f.closest('.seg');
                if (segment) { segment.querySelectorAll('button').forEach(function (b) { b.dataset.was = now; b.setAttribute('aria-pressed', String(b.value === now)); }); }
                markField(f, null);
                /* A colour taken away shows the neutral swatch again. */
                var pick = f.id && el.inspector.querySelector('.md-colour-pick[data-for="' + f.id + '"]');
                if (pick && !now) { pick.value = '#808080'; }
                done(out, msg);
            };
            if (f.hasAttribute('data-arrows')) {
                var arrows = ARROWS.find(function (a) { return a[0] === now; });
                applyEdit({ op: 'arrows', key: key, value: now }, null, true).then(function (out) { finish(out, 'Made the direction ' + (arrows ? arrows[1].toLowerCase() : now) + '.'); });
                return;
            }
            var setKey = f.getAttribute('data-set');
            if (!setKey) { return; }
            var value = f.type === 'checkbox' ? (f.checked ? 'true' : null) : now;
            if (setKey === key) {
                if (value === '' && !isConnection(key)) { value = lastName(key); }
            } else if (value === '') { value = null; }
            var shown = f.tagName === 'SELECT' && f.selectedOptions[0] ? f.selectedOptions[0].textContent : value;
            var msg = f.type === 'checkbox' ? 'Turned ' + name + (value ? ' on.' : ' off.') : value == null ? 'Took the ' + name.toLowerCase() + ' away.' : 'Set the ' + name.toLowerCase() + ' to ' + shown + '.';
            applyEdit({ op: 'set', key: setKey, value: value }, key, true).then(function (out) { finish(out, msg); });
        }
        el.inspector.addEventListener('change', function (e) {
            if (e.target.classList.contains('md-colour-pick')) {
                var paired = el.inspector.querySelector('#' + CSS.escape(e.target.getAttribute('data-for')));
                if (paired) { paired.value = e.target.value; inspectorChanged(paired); }
                return;
            }
            if (e.target.matches('[data-set], [data-arrows]')) { inspectorChanged(e.target); }
        });
        /* Enter makes the change and leaves the keyboard in the field. */
        el.inspector.addEventListener('keydown', function (e) {
            if (e.key === 'Enter' && e.target.matches('input.form-input[data-set]')) { e.preventDefault(); inspectorChanged(e.target); }
        });
        el.inspector.addEventListener('click', function (e) {
            var setting = e.target.closest('button[data-set]');
            if (setting) { inspectorChanged(setting); return; }
            if (!e.target.closest('[data-reverse]')) { return; }
            var t = current(), key = selection(), c = t && connectionOf(t, key);
            if (!c || !canEdit()) { return; }
            applyEdit({ op: 'reconnect', key: key, from: c.to, to: c.from }).then(function (out) { done(out, 'Reversed it; it now goes from ' + c.to + ' to ' + c.from + '.'); });
        });
        function toggleInspector() {
            settings.inspector = !settings.inspector;
            keepSetting('inspector', settings.inspector);
            renderInspector();
            commandsChanged();
        }

        function selectKey(t, key, how) {
            if (!t.canvas) { return; }
            t.canvas.select(key);
            canvasSelected(t, t.canvas.selected(), how);
        }
        /* The cursor in the text selects the shape or connection written
           where it stands, the innermost when they nest. */
        function followCursor(pos) {
            var t = current();
            if (t && t.kind === 'canvas') { followCanvasCursor(t, pos); return; }
            if (!t || !t.canvas || !t.outline || t.outlineText !== t.text) { return; }
            var best = null, size = Infinity;
            [].concat(t.outline.shapes || [], t.outline.connections || []).forEach(function (it) {
                (it.places || []).forEach(function (p) {
                    if (pos >= p.from && pos <= p.to && p.to - p.from < size) { best = it.id; size = p.to - p.from; }
                });
            });
            if (best !== t.canvas.selected()) { t.canvas.select(best); canvasSelected(t, best, 'cursor'); }
        }

        /* One of D2's edits to the front diagram's text (internal/d2api
           edit). The first one in a file whose text is not as D2 writes it
           says so once, since D2 writes the whole text again. */
        async function applyEdit(req, keep, quiet) {
            var t = current();
            if (!t || t.kind !== 'd2') { return null; }
            if (readOnly(t)) { say(mutationWhy(t), 'info'); return null; }
            var original = t.text;
            if (t.problems && t.problems.length) { say('The text has a mistake in it, so the drawing cannot be changed. Put the mistake right first.', 'warn'); return null; }
            var out;
            try {
                var E = await d2();
                if (!t.tidyKnown) {
                    var f = await E.call('format', { text: t.text }).catch(function () { return null; });
                    t.tidyKnown = true;
                    t.untidy = !!(f && f.text && f.text !== t.text);
                }
                out = await E.call('edit', Object.assign({ text: t.text }, req));
            } catch (err) {
                say('The change could not be made, because the diagram engine failed: ' + ((err && err.message) || err), 'problem');
                return null;
            }
            if (!out) { return null; }
            if (out.problems && out.problems.length) { if (!quiet) { say('That change could not be made. ' + out.problems[0].message, 'warn'); } return out; }
            if (destroyed || t !== current() || t.text !== original || readOnly(t)) { return null; }
            /* A change that worked puts right whatever a refused one said. */
            say('', 'warn');
            t.pending = keep || out.key || null;
            replaceText(out.text);
            await drawNow();
            /* The first change to a file D2 did not write itself rewrites the
               whole text (decision 5), which the caller's message says. */
            if (t.untidy) { t.untidy = false; out.rewrote = true; }
            return out;
        }
        function done(out, msg) {
            if (!out || (out.problems && out.problems.length)) { return; }
            say(msg + (out.rewrote ? ' D2 also wrote the whole text in its own layout, as the first change on the drawing does; Undo puts it back as it was.' : ''));
        }
        function lastName(key) { var m = /([^.]+)$/.exec(key || ''); return m ? m[1] : key; }
        /* The palette's choice: null for Select, or a kind of shape. */
        var tool = null;
        var canvasPan = false;
        function setCanvasPan(value) { setTool(null); canvasPan = value; root.classList.toggle('dg-cv-panning', value); commandsChanged(); }
        function setTool(kind) {
            if (kind && kind !== 'select' && readOnly(current())) { say(mutationWhy(), 'info'); return; }
            canvasPan = false;
            root.classList.remove('dg-cv-panning');
            tool = kind && kind !== 'select' ? kind : null;
            root.classList.toggle('dg-placing', !!tool);
            say('');
            if (tool) {
                el.stage.focus({ preventScroll: true });
            }
            commandsChanged();
        }
        function placeShape(container) {
            var kind = tool || 'rectangle';
            setTool(null);
            return applyEdit({ op: 'create', key: (container ? container + '.' : '') + 'item', shape: kind }).then(function (out) {
                if (out && out.key) { done(out, 'Placed ' + out.key + (container ? ' inside ' + container : '') + '. Press Enter to give it a label.'); }
            });
        }
        /* A drag on the drawing: a connection dragged out from a shape's
           handle, to another shape or to empty space, where it makes a new
           shape of the palette's kind; or a shape dragged into a container,
           or out to the top, which D2 moves with everything that refers to
           it (decision 5). */
        function dropped(d) {
            if (!canEdit()) { return; }
            if (d.kind === 'connect') {
                if (d.to === d.from) { say('A connection needs another shape at its far end.'); return; }
                var kind = tool || 'rectangle';
                setTool(null);
                applyEdit({ op: 'create', key: d.from + ' -> ' + (d.to || freeName('')), shape: d.to ? '' : kind }).then(function (out) {
                    done(out, 'Connected ' + d.from + ' to ' + (d.to || 'a new shape') + '. Press Enter to label the connection.');
                });
                return;
            }
            if (d.kind === 'move') { moveTo(d.from, d.to); }
        }
        function moveTo(key, container) {
            var to = (container ? container + '.' : '') + lastName(key);
            if (!key || to === key) { return; }
            applyEdit({ op: 'move', key: key, to: to, descendants: true }).then(function (out) {
                done(out, 'Moved ' + key + (container ? ' into ' + container : ' out to the top') + '. Undo puts it back.');
            });
        }
        /* Move Into, the keyboard's way to what a drag onto a container
           does: the container is asked for by its full name, checked
           against the diagram's shapes in the dialog. */
        function moveInto() {
            var key = selection(), t = current();
            if (!key || isConnection(key) || !canEdit()) { return; }
            var original = t.text;
            var names = ((t.outline && t.outline.shapes) || []).map(function (s) { return s.id; }).filter(function (id) { return id !== key && id.indexOf(key + '.') !== 0; });
            if (!names.length) {say('There are no other shapes to move into.',true);return;}
            window.yavcDialog.askText('Move Into a Shape', 'Container', '', {
                okLabel: 'Move',signal:lifetime.signal,choices:names.map(function (id) {return [id,id];}),
                help: 'Move ' + key + ' into this shape. The destination becomes a container.',
                validate: function (v) { return t!==current() || t.text!==original ? 'The diagram changed. Close and reopen this form.' : names.indexOf(v) < 0 ? 'Choose a shape in this diagram.' : null; }
            }).then(function (v) { if (v && !destroyed && t === current() && t.text === original && !readOnly(t)) { moveTo(key, v); } });
        }
        /* The next free name for a new shape in a container ('' for the
           top): item, item 2, item 3, as D2 numbers them. A connection's
           key contains the name of its far end, so the name must be free,
           or the connection would go to the shape already called that. */
        function freeName(parent) {
            var t = current(), taken = {};
            ((t && t.outline && t.outline.shapes) || []).forEach(function (s) { if ((s.parent || '') === parent) { taken[lastName(s.id).toLowerCase()] = true; } });
            var name = 'item', i = 1;
            while (taken[name]) { name = 'item ' + (++i); }
            return (parent ? parent + '.' : '') + name;
        }
        function addShape(inside) {
            var sel = selection(), parent = inside && sel && !isConnection(sel) ? sel + '.' : '';
            el.stage.focus({ preventScroll: true });
            var kind = tool || 'rectangle';
            setTool(null);
            /* A new shape is called item, which D2 numbers when it is taken;
               shape would be D2's own keyword, which it renames. */
            return applyEdit({ op: 'create', key: parent + 'item', shape: kind }).then(function (out) {
                if (out && out.key) { done(out, 'Added ' + out.key + '. Press Enter to give it a label, or F2 to rename it.'); }
            });
        }
        function editLabel(key) {
            var t = current();
            key = key || selection();
            if (!t || !key || !canEdit()) { return; }
            var original = t.text;
            var item = isConnection(key) ? connectionOf(t, key) : shapeOf(t, key);
            var now = item ? (item.label || '') : '';
            var conn = isConnection(key);
            window.yavcDialog.askText(conn ? 'The Connection\'s Label' : 'The Shape\'s Label', 'Label', now, {
                okLabel: 'Apply',signal:lifetime.signal,multiline:true,
                help: conn ? 'The words shown on the connection. Leave it empty for none.' : 'The words shown on the shape. Leave it empty to show its name, ' + lastName(key) + '.',
                validate: function (v) { return destroyed || t!==current() || t.text!==original || readOnly(t) ? 'The diagram changed. Close and reopen this form.' : /[{}]/.test(v) ? 'A label cannot hold a brace, { or }.' : null; }
            }).then(function (v) {
                if (v == null || v === now || destroyed || t !== current() || t.text !== original || readOnly(t)) { return; }
                applyEdit({ op: 'set', key: key, value: v === '' ? (conn ? '' : lastName(key)) : v }).then(function (out) { done(out, 'Set the label.'); });
            });
        }
        function renameShape() {
            var key = selection();
            if (!key || isConnection(key) || !canEdit()) { return; }
            var t = current(), parent = key.indexOf('.') >= 0 ? key.replace(/\.[^.]*$/, '') : '';
            var original = t.text;
            var siblings = ((t.outline && t.outline.shapes) || []).filter(function (s) { return (s.parent || '') === parent && s.id !== key; }).map(function (s) { return lastName(s.id).toLowerCase(); });
            window.yavcDialog.askText('Rename the Shape', 'Name', lastName(key), {
                okLabel: 'Rename',signal:lifetime.signal,
                help: 'The name the text and every connection use for it. Its label, the words shown, stays as it is.',
                validate: function (v) {
                    if(destroyed || t!==current() || t.text!==original || readOnly(t))return 'The diagram changed. Close and reopen this form.';
                    if (!v) { return 'A shape needs a name.'; }
                    if (/[.:;{}()\[\]<>|`'"#&*$@\\]/.test(v) || /->|<-|--/.test(v)) { return 'A name cannot hold a dot, a colon, a brace, a bracket, an arrow, a quote or # & * $ @ |.'; }
                    if (RESERVED.indexOf(v.toLowerCase()) >= 0) { return v + ' is one of D2\'s own words, so it cannot name a shape.'; }
                    if (siblings.indexOf(v.toLowerCase()) >= 0) { return 'Another shape here is called ' + v + ' already.'; }
                    return null;
                }
            }).then(function (v) {
                if (v == null || v === lastName(key) || destroyed || t !== current() || t.text !== original || readOnly(t)) { return; }
                applyEdit({ op: 'rename', key: key, name: v }).then(function (out) { done(out, 'Renamed ' + key + ' to ' + v + '.'); });
            });
        }
        function setShape(type) {
            var key = selection();
            if (!key || isConnection(key)) { return; }
            applyEdit({ op: 'set', key: key + '.shape', value: type === 'rectangle' ? null : type }).then(function (out) { done(out, 'Made it a ' + type.replace(/_/g, ' ') + '.'); });
        }
        function deleteSelected() {
            var key = selection();
            if (!key || !canEdit()) { return; }
            var what = describe(current(), key);
            applyEdit({ op: 'delete', key: key }).then(function (out) {
                done(out, 'Deleted ' + what + '. Undo brings it back.');
            });
        }
        /* Connect: the selected shape is one end, and the next shape chosen
           on the drawing, by a click or by Enter, is the other. */
        var connecting = null;
        function startConnect() {
            var key = selection();
            if (!key || isConnection(key) || !canEdit()) { say('Select the shape to connect from first.'); return; }
            connecting = key;
            root.classList.add('dg-connecting');
            el.stage.focus({ preventScroll: true });
            say('Connecting from ' + describe(current(), key) + '.');
            commandsChanged();
        }
        function stopConnect(msg) {
            connecting = null;
            root.classList.remove('dg-connecting');
            if (msg != null) { say(msg); }
            commandsChanged();
        }
        function pickEnd(key) {
            if (!connecting) { return false; }
            if (isConnection(key)) { say('A connection needs a target shape.',true); return true; }
            var from = connecting;
            stopConnect(null);
            applyEdit({ op: 'create', key: from + ' -> ' + key }).then(function (out) {
                done(out, 'Connected ' + from + ' to ' + key + '. Press Enter to label the connection.');
            });
            return true;
        }
        /* Keys on the drawing's pane. The arrows, Home, End, Enter and
           Escape are the canvas's; the rest are the editor's commands, kept
           to this pane so that typing in the text never deletes a shape. */
        el.stage.addEventListener('keydown', function (e) {
            var t = current();
            if (t && t.cv && e.target === el.stage) { canvasKey(e); return; }
            if (!t || !t.canvas || e.target !== el.stage || e.defaultPrevented || e.isComposing || t.outlineText !== t.text || (t.problems || []).length) { return; }
            if (connecting && e.key === 'Escape') { e.preventDefault(); stopConnect('Stopped connecting.'); return; }
            if (tool && e.key === 'Escape') { e.preventDefault(); setTool(null); say('Back to Select.'); return; }
            if (connecting && e.key === 'Enter') {
                e.preventDefault();
                if (selection() && selection() !== connecting) { pickEnd(selection()); } else { say('Move to the shape to connect to, then press Enter; Escape stops.'); }
                return;
            }
            if (t.canvas.onKey(e)) { e.preventDefault(); return; }
            if (e.ctrlKey || e.metaKey || e.altKey) { return; }
            if (e.key === 'Delete') { e.preventDefault(); deleteSelected(); }
            else if (e.key === 'F2') { e.preventDefault(); renameShape(); }
            else if (e.key === 'c' || e.key === 'C') { e.preventDefault(); startConnect(); }
            else if (e.key === 'n' || e.key === 'N') { e.preventDefault(); addShape(e.shiftKey); }
        });

        /* === Views ======================================================== */

        function viewOf(t) { return t && VIEWS.some(function (v) { return v[0] === t.view; }) ? t.view : settings.view; }
        function applyDock() {
            var dock = DOCKS.some(function (d) { return d[0] === settings.dock; }) ? settings.dock : 'left';
            var view = viewOf(current()), first = dock === 'left' || dock === 'top';
            root.setAttribute('data-dock', dock);
            root.setAttribute('data-view', view);
            el.work.classList.toggle('stacked', dock === 'top' || dock === 'bottom');
            el.work.replaceChildren.apply(el.work, first ? [el.stagebox, el.split, el.text] : [el.text, el.split, el.stagebox]);
            /* PUDL's splitter shows one pane on data-split-pane, and keeps
               where the handle was for when both are back. */
            if (view === 'split') { el.work.removeAttribute('data-split-pane'); }
            else if (view === 'preview') { el.work.setAttribute('data-split-pane', first ? 'first' : 'second'); }
            else { el.work.setAttribute('data-split-pane', first ? 'second' : 'first'); }
            if (cm) { cm.requestMeasure(); }
        }
        function setView(view) {
            var t = current();
            if (!VIEWS.some(function (v) { return v[0] === view; })) { return; }
            if (t) { t.view = view; }
            settings.view = view;
            keepSetting('view', view);
            applyDock();
            if (view !== 'preview' && cm && (opts.ownsUrl || !root.closest('.win') || root.closest('.win.active'))) { cm.focus(); }
            commandsChanged();
            if (window.pudlMenubar) { window.pudlMenubar.refresh(); }
            announce();
        }
        function setDock(dock) { settings.dock = dock; applyDock(); commandsChanged(); keepSetting('dock', dock); }

        /* === Tabs ========================================================= */

        function tabTitle(t) { return t.resource && t.resource.name || t.name || (t.path ? baseName(t.path) : 'Untitled'); }
        var tabRowApi = window.yavcDocTabs ? window.yavcDocTabs.attach(el.tabsEl, {
            application: 'Diagrams',
            tabs: function () { return tabs.map(function (t) { return { name: tabTitle(t), path: t.path, modified: t.modified }; }); },
            active: function () { return active; },
            activate: function (i) { activateTab(i); },
            close: function (i) { return closeTab(i); }
        }) : null;
        function tabRow() { if (tabRowApi) { tabRowApi.render(); } }
        function makeTab(text, path, kind, view) {
            kind = kind || kindOf(path, text);
            /* Each tab draws into a place of its own, so a drawing that
               finishes after a switch lands in its own tab, never over the
               tab in front, and a tab comes back with its last drawing. */
            var place = document.createElement('div');
            place.className = 'dg-place';
            var t = { path: path || null, kind: kind, text: text, modified: false, view: view || null, problems: [], svg: null, size: null, place: place };
            if (!path) { untitled++; t.name = 'untitled-' + untitled + EXT[kind]; }
            t.state = stateFor(t, text);
            return t;
        }
        function stash() { var t = current(); if (t && cm) { t.state = cm.state; } }
        function activateTab(i) {
            if (!tabs[i]) { return; }
            finishInlineText(false);
            if (current() && current().cv) { current().cv.cancel(); }
            setTool(null);
            stash();
            active = i;
            var t = tabs[i];
            showTab(t);
            el.canvas.replaceChildren(t.place);
            el.where.textContent = t.path || 'Not saved yet'; el.where.title = el.where.textContent;
            el.kind.textContent = KIND_NAME[t.kind];
            root.setAttribute('data-kind', t.kind);
            q('.dg-keys').textContent = t.kind === 'canvas'
                ? 'Tab selects nodes and connectors in drawing order. Shift+Tab selects the previous item and leaves the drawing at the first item. Arrow keys move a node by one pixel, or eight with Shift. N adds a shape, G a group, C connects, R resizes, E reconnects a connector end, Shift+E reconnects its start, and Delete deletes. Ctrl or Command with C, V and D copies, pastes and duplicates. Alt with arrow keys pans. Plus and minus zoom, 0 fits, and 1 shows actual size. Space with a drag pans. Escape cancels a drag and returns to Select.'
                : 'Arrow keys select a shape or connection. Enter edits its label, F2 renames it, N adds a shape, C connects it, Delete deletes it, and Escape returns to Select.';
            inspected = undefined;
            renderInspector();
            applyDock();
            drawNow();
            commandsChanged();
            announce();
        }
        function untouched(t) { return t && !t.path && !t.modified; }
        /* A diagram opens in a tab of its own, unless the front tab is a new
           diagram nobody has touched, which it replaces. */
        function openInTab(text, path, kind, view) {
            stash();
            var t = makeTab(text, path, kind, view);
            /* The tab replaced is let go first, so that showing the new one
               does not keep the old one's text in the new one's place. */
            if (untouched(current()) && tabs.length) { var at = active, old = current(); if (old.canvas) { old.canvas.destroy(); } if (old.cv) { old.cv.destroy(); } tabs[at] = t; active = -1; activateTab(at); return t; }
            tabs.push(t);
            activateTab(tabs.length - 1);
            return t;
        }
        async function closeTab(i) {
            var t = tabs[i];
            if (!t) { return; }
            if (t.modified) {
                if (i !== active) { activateTab(i); }
                var choice = await window.yavcDialog.askSave(tabTitle(t));
                if (choice !== 'discard' && choice !== 'save') { return; }
                if (choice === 'save') { await save(); if (t.modified) { return; } }
            }
            stash();
            var front = active;
            if (t.canvas) { t.canvas.destroy(); }
            if (t.cv) { t.cv.destroy(); }
            tabs.splice(i, 1);
            if (i < front) { front--; } else if (i === front) { front = Math.min(i, tabs.length - 1); }
            if (!tabs.length) { tabs.push(makeTab(D2_STYLES[0].text, null, 'd2')); front = 0; }
            active = -1;
            activateTab(front);
        }

        /* === Files ======================================================== */

        async function openPath(p, view) {
            if (!tabs.some(function (t) { return t.path === p; })) { var routed = await routeDocument('open-file', {path: p, view: view}); if (routed) return true; }
            if (destroyed) return false;
            for (var i = 0; i < tabs.length; i++) {
                if (tabs[i].path === p) { activateTab(i); if (view) { setView(view); } return true; }
            }
            if (!resources && (!H.fs || !H.fs.readText)) { say('There is no file service here.', true); return false; }
            var text;
            var resource;
            try { if (resources) { resource = await resources.read(p); text = resource.text; } else { text = await H.fs.readText(p); } } catch (err) { if (!destroyed) say(p + ' could not be read: ' + ((err && err.message) || err), true); return false; }
            if (destroyed) return false;
            if (text == null) { say('There is no file at ' + p + '.', true); return false; }
            var opened = openInTab(text, p, null, view); opened.resource = resource || null; refreshAccess(opened); renderInspector(); commandsChanged();
            say('Opened ' + p + '.');
            return true;
        }
        async function open(input) {
            if(destroyed || startupFailed)return false;
            if(input && typeof input.resource==='string'){
                var existed=tabs.some(function (t) {return t.path===input.resource;});
                var preferences=H.config && H.config.stored ? await H.config.stored('diagrams') : {};
                if(destroyed)return false;
                var desired=input.view || (!existed && !VIEWS.some(function (v) {return v[0]===preferences.view;}) && input.initialView);
                var mode=desired==='visual'?'preview':desired;
                if(mode && !VIEWS.some(function (v) {return v[0]===mode;}))throw new Error('This view is unavailable in Diagrams.');
                return openPath(input.resource,mode);
            }
            if (!H.fs || !H.fs.open) { say('There is no file service here, so nothing can be opened.', true); return; }
            var res = await H.fs.open({ title: 'Open a Diagram', host: root, start: '~', accept: ['.d2', '.mmd', '.mermaid', '.canvas'] }).catch(function (err) { say('The file could not be opened: ' + ((err && err.message) || err), true); return null; });
            if (!res) { return; }
            if (res.path) { await openPath(res.path); return; }
            var routed = await routeDocument('new', {text: res.text, kind: kindOf(res.name, res.text)}); if (routed) return;
            var t = openInTab(res.text, null, kindOf(res.name, res.text));
            t.modified = true;
            say('Opened ' + res.name + '.');
            commandsChanged();
        }
        function queueSave(t, run) {
            var ticket = t.saveTicket = (t.saveTicket || 0) + 1;
            var pending = (t.saveQueue || Promise.resolve()).then(function () {
                if (destroyed || tabs.indexOf(t) < 0) { throw new Error('The diagram was closed before its queued save could run.'); }
                return run(ticket);
            });
            t.saveQueue = pending.catch(function () { });
            function settled(value) {
                if (!destroyed && tabs.indexOf(t) >= 0) {
                    if (ticket === t.saveTicket && !t.saveFailed && t.savedPath === t.path && t.savedText !== undefined) { t.modified = t.text !== t.savedText; }
                    commandsChanged(); keep();
                }
                return value;
            }
            return pending.then(settled, function (err) { t.saveFailed = true; t.modified = true; settled(null); throw err; });
        }
        function saved(t, path, text, ticket, rename) {
            if (destroyed || tabs.indexOf(t) < 0) { return false; }
            if (rename) { t.path = path; }
            t.savedPath = path; t.savedText = text; t.saveFailed = false;
            t.modified = t.path !== path || t.text !== text || ticket !== t.saveTicket;
            return true;
        }
        async function writeResource(t, path, submitted) {
            if (resources) {
                if (t.recoveryConflict) throw new Error('These recovered edits have no matching file revision. Download the source before reopening the file.');
                if (!t.resource || path !== t.resource.resource) throw new Error('This resource host does not offer Save As yet.');
                var accepted = await resources.write(t.resource, submitted);
                if (!destroyed && tabs.indexOf(t) >= 0) { t.resource = accepted; if (t === current()) { refreshAccess(t); renderInspector(); } }
                return null;
            }
            if (!H.fs || !H.fs.writeText) throw new Error('This host cannot save files.');
            return H.fs.writeText(path, submitted);
        }
        async function save() {
            var t = current();
            if (!t) { return; }
            if (!t.path || resources && resources.needsLocation(t.resource)) { return saveAs(); }
            var submitted = t.text;
            return queueSave(t, async function (ticket) {
                var path = t.path, err = await writeResource(t, path, submitted);
                if (err) { t.saveFailed = true; t.modified = true; say('Could not save: ' + err, true); return; }
                if (saved(t, path, submitted, ticket, false) && t === current()) { say('Saved ' + path + (t.modified ? '; newer edits are still unsaved.' : '.')); }
            }).catch(function (err) {
                if (!destroyed && tabs.indexOf(t) >= 0 && t === current()) say('Save was not acknowledged: ' + ((err && err.message) || err), true);
                throw err;
            });
        }
        async function saveAs() {
            var t = current();
            if (!t) return;
            if (resources) {
                if (!resources.canSaveAs()) return;
                var captured = t.text,format=sourceFormat(t);
                return queueSave(t, async function (ticket) {
                    var accepted = await resources.saveAs(Object.assign({ name: tabTitle(t), sourceResource: t.path },format), captured, { host: root, signal: lifetime.signal, checkName:function(name){
                        var suffix=/\.(d2|mmd|mermaid|canvas)$/i.exec(String(name||'')), kind=suffix && (suffix[1].toLowerCase()==='mmd'||suffix[1].toLowerCase()==='mermaid'?'mermaid':suffix[1].toLowerCase());
                        return kind===t.kind?null:'Keep the '+EXT[t.kind]+' extension. Use Convert to create a different diagram format.';
                    } });
                    if (accepted) savedAs(t, accepted.resource, captured, ticket, accepted);
                }).catch(function (err) { if (destroyed) return; if (tabs.indexOf(t) >= 0) say('Save As was not acknowledged: ' + err.message, true); throw err; });
            }
            throw new Error('This host does not offer conditional Save As. Download the source to keep a copy.');
        }
        function savedAs(t, path, submitted, ticket, accepted) {
            if (!saved(t, path, submitted, ticket, true)) return;
            t.name = accepted && accepted.name || null;
            if (accepted) { t.resource = accepted;t.format=sourceFormat(t); t.recoveryBaseline = null; t.recoveryConflict = null; }
            refreshAccess(t); renderInspector();
            var kind = kindOf(accepted && accepted.name || path, t.text);
            if (kind !== t.kind) {
                t.kind = kind;
                if (t === current()) { finishInlineText(false); stash(); }
                if (t.canvas) { t.canvas.destroy(); t.canvas = null; }
                if (t.cv) { t.cv.destroy(); t.cv = null; }
                t.svg = null; t.size = null; t.cvPending = null; t.state = stateFor(t, t.text);
                if (t === current()) { showTab(t); el.kind.textContent = KIND_NAME[kind]; root.setAttribute('data-kind', kind); drawNow(); }
            }
            if (t === current()) { window.yavcUtil.notice(el.tabsEl, ''); el.where.textContent = t.path; el.where.title = el.where.textContent; say('Saved ' + (accepted && accepted.name || path) + (t.modified ? '; newer edits are still unsaved.' : '.')); announce(); }
        }
        async function newDiagram(style, kind) {
            var routed = await routeDocument('new', {text: style.text, kind: kind}); if (routed) return;
            openInTab(style.text, null, kind, null);
            say('This is a new ' + style.label.toLowerCase() + '. Save it to keep it.');
        }
        async function newCanvas(style) {
            await canvasEditor();
            if (!destroyed) { await newDiagram({ label: 'Canvas', text: window.yavcCanvasModel.template(style).text }, 'canvas'); }
        }
        async function conversion(format) {
            var owner = current(), text = owner && owner.text;
            if (!owner || format === owner.kind || !['d2', 'canvas'].includes(owner.kind) || !['d2', 'canvas'].includes(format)) throw new Error('Convert between D2 and Canvas diagrams.');
            await jcanvas();
            var C = await once('conversion', asset('diagram-kit/conversion.js'), 'yavcDiagramConversion'), E = await d2(), result;
            if (owner.kind === 'd2') {
                var parts = await Promise.all([E.compile({text:text}), E.call('outline', {text:text})]);
                result = C.toCanvas(parts[0], parts[1]);
            } else {
                var got = window.yavcCanvas.read(text), fault = got.problems.find(function (p) { return !p.warning; });
                if (!got.doc || fault) throw new Error(fault ? fault.message : 'The canvas cannot be read.');
                var quoted = await E.call('quote', {values:C.labels(got.doc)});
                result = C.toD2(got.doc, quoted && quoted.values);
                var compiled = await E.compile({text:result.text});
                if (!compiled || (compiled.problems || []).length) throw new Error('D2 cannot represent this copy: ' + (compiled && compiled.problems && compiled.problems[0] ? compiled.problems[0].message : 'the result was replaced by another request.'));
            }
            if (destroyed || owner !== current() || owner.text !== text) throw new Error('The diagram changed during conversion. Try again.');
            return result;
        }
        async function convertDiagram(format) {
            var owner = current(), text = owner && owner.text;
            try {
                var result = await conversion(format), title = format === 'canvas' ? 'Convert to Canvas' : 'Convert to D2';
                var accepted = await window.yavcDialog.open({title:title,className:'dg-conversion-dialog',
                    body:'<p>The conversion creates an unsaved copy and keeps the original open.</p><ul>' + result.warnings.map(function (warning) { return '<li>' + esc(warning) + '</li>'; }).join('') + '</ul>',
                    signal:lifetime.signal,buttons:[{id:'cancel',label:'Cancel'},{id:'create',label:'Create Copy',primary:true}]});
                if (accepted.value !== 'create' || destroyed) return;
                if (owner !== current() || owner.text !== text) throw new Error('The diagram changed. Convert it again.');
                if (documentWindows && documentWindows.mode(root) === 'single') { await documentWindows.open(root, 'diagrams', {operation:'new',input:{kind:format,text:result.text}}); return; }
                stash();var t = makeTab(result.text, null, format, 'preview'); t.modified = true; tabs.push(t); activateTab(tabs.length - 1); await drawNow();
            } catch (error) { if (!destroyed) say(error.message, true); }
        }
        function editInEditor() {
            var t = current();
            if (!t || !t.path) { say('Save the diagram first, so the Editor can open it.'); return; }
            if (window.pudlApplets && window.pudlApplets.can && window.pudlApplets.can('open', 'file')) {
                window.pudlApplets.request('open', { path: t.path, kind: 'file' }, root);
            } else { say('The Editor is not on this page, so the diagram cannot be opened in it here.', true); }
        }
        /* Find, in the text, with the editing kit's find bar; from Preview
           the text comes back beside the drawing first. */
        function openFind() {
            if (!cm) { return; }
            if (viewOf(current()) === 'preview') { setView('split'); }
            CM.openSearchPanel(cm);
        }
        function goToLine() {
            if (!cm) { return; }
            if (viewOf(current()) === 'preview') { setView('split'); }
            var binding = CM.searchKeymap.find(function (key) { return key.key === 'Mod-Alt-g'; });
            if (binding) { binding.run(cm); }
        }

        /* === Export (decision 14) ========================================= */

        function stem(t) { return tabTitle(t).replace(/\.(d2|mmd|mermaid|canvas)$/i, '') || 'diagram'; }
        function drawable() { var t = current(); return !!(t && t.svg && !(t.problems && t.problems.length)); }
        function needDrawing() { say('The diagram has a mistake in it, so there is no drawing to export. Put the mistake right first.', 'warn'); }
        function downloadSVG() {
            var t = current();
            if (!drawable()) { needDrawing(); return; }
            window.yavcUtil.download(stem(t) + '.svg', new Blob([t.svg], { type: 'image/svg+xml' }));
            say('Downloaded ' + stem(t) + '.svg.');
        }
        /* A picture of the drawing at one, two or three times its size, drawn
           from the SVG through an image, which loads nothing from outside. */
        function pngOf(t, scale) {
            return new Promise(function (resolve, reject) {
                var size = t.size || { w: 800, h: 600 };
                var img = new Image();
                img.onload = function () {
                    var c = document.createElement('canvas');
                    c.width = Math.max(1, Math.round(size.w * scale));
                    c.height = Math.max(1, Math.round(size.h * scale));
                    var g = c.getContext('2d');
                    g.drawImage(img, 0, 0, c.width, c.height);
                    c.toBlob(function (b) { if (b) { resolve(b); } else { reject(new Error('the browser could not make the picture')); } }, 'image/png');
                };
                img.onerror = function () { reject(new Error('the drawing could not be read as a picture')); };
                var svg = t.svg;
                if (!/<svg[^>]*\swidth=/.test(svg)) { svg = svg.replace(/<svg\b/, '<svg width="' + size.w + '" height="' + size.h + '"'); }
                img.src = 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svg);
            });
        }
        function downloadPNG(scale) {
            var t = current();
            if (!drawable()) { needDrawing(); return; }
            var name = stem(t) + (scale > 1 ? '@' + scale + 'x' : '') + '.png';
            pngOf(t, scale).then(function (blob) { window.yavcUtil.download(name, blob); say('Downloaded ' + name + '.'); })
                .catch(function (err) { say('The picture could not be made: ' + err.message, true); });
        }
        function downloadSource() {
            var t = current();
            if (!t) { return; }
            window.yavcUtil.download(tabTitle(t), new Blob([t.text], { type: 'text/plain' }));
            say('Downloaded ' + tabTitle(t) + '.');
        }
        /* The diagram as a fenced block, which the Editor and Slides draw
           wherever it is pasted, and which carries its source with it
           (decision 9). The {{diagram}} shortcode, by reference, comes with
           the embedding work. */
        function copyEmbed() {
            var t = current();
            if (!t) { return; }
            var longest = (t.text.match(/`+/g) || []).reduce(function (length, run) { return Math.max(length, run.length); }, 2);
            var fence = '`'.repeat(longest + 1);
            var block = fence + (t.kind === 'canvas' ? 'canvas' : t.kind === 'd2' ? 'd2' : 'mermaid') + '\n' + t.text.replace(/\s+$/, '') + '\n' + fence + '\n';
            if (!navigator.clipboard || !navigator.clipboard.writeText) { say('This browser does not let the page write to the clipboard.', true); return; }
            navigator.clipboard.writeText(block).then(function () { say('Copied as a fenced block. Paste it into a document in the Editor or a slide.'); },
                function () { say('The clipboard could not be written to.', true); });
        }
        /* D2's own formatting of the text (decision 5), as one change. */
        /* A canvas's text written in the stable form (docs/canvas-design.md,
           decision 4), or null with the reason when it has a mistake. */
        function tidyCanvas(text) {
            return jcanvas().then(function (J) {
                var got = J.read(text), fault = got.problems.filter(function (p) { return !p.warning; })[0];
                return fault || !got.doc ? { problem: fault ? fault.message : 'it could not be read' } : { text: J.write(got.doc) };
            });
        }
        function formatText() {
            var t = current();
            if (!t || readOnly(t)) return;
            var original = t.text;
            if (t && t.kind === 'canvas') {
                tidyCanvas(t.text).then(function (out) {
                    if (destroyed || t !== current() || t.text !== original || readOnly(t)) return;
                    if (out.problem) { say('The text has a mistake in it, so it was not tidied. Put the mistake right first.', 'warn'); return; }
                    if (out.text === t.text) { say('The text was tidy already.'); return; }
                    replaceText(out.text);
                    say('Tidied the text. Undo puts it back as it was.');
                }).catch(function (err) { say('The text could not be tidied: ' + err.message, true); });
                return;
            }
            if (!t || t.kind !== 'd2') { return; }
            d2().then(function (E) { return E.call('format', { text: t.text }); }).then(function (out) {
                if (destroyed || t !== current() || t.text !== original || readOnly(t)) return;
                if (!out) { return; }
                if (out.problems && out.problems.length) { say('The text has a mistake in it, so it was not tidied. Put the mistake right first.', 'warn'); return; }
                if (out.text === t.text) { say('The text was tidy already.'); return; }
                replaceText(out.text);
                say('Tidied the text. Undo puts it back as it was.');
            }).catch(function (err) { say('The text could not be tidied: ' + err.message, true); });
        }

        /* === Snapshots and continuity ===================================== */

        var unguard = window.yavcDocTabs.guard(root, {
            changed: function () { return tabs.filter(function (t) { return t.modified; }).map(tabTitle); },
            keptOnReload: function () { return !!snap && (typeof snap.flush === 'function' ? snap.flush() : !resources); },
            keptOnClose: function () { return false; },
            saveAll: async function () {
                for (var i = 0; i < tabs.length; i++) {
                    if (!tabs[i].modified) { continue; }
                    activateTab(i);
                    await save();
                    if (tabs[i].modified) { return false; }
                }
                return true;
            }
        });
        function legacySnapshot() {
            return { tabs: tabs.map(function (t) { return { path: t.path, name: t.name || null, kind: t.kind, text: t.modified ? t.text : null, view: t.view || null }; }), active: active };
        }
        var snap = null;
        function resourceSnapshot() {
            if (!recoveryReady) return recoveryKept;
            var value = { version: 2, tabs: tabs.map(function (t) {
                return { path: t.path, name: t.resource && t.resource.name || t.name || null, kind: t.kind, text: t.modified || !t.path ? t.text : null, view: t.view || null,
                    format:sourceFormat(t),baseline: t.path ? recovery.capture(resources, t.resource, t.recoveryBaseline) : null };
            }), active: active };
            recovery.validate(value);
            return value;
        }
        function keep() { if (snap) { snap.changed(); } }

        /* === Commands ===================================================== */

        /* builtIn is a key CodeMirror binds itself, such as Ctrl+Z, which
           the menus show and PUDL leaves to CodeMirror (bind: false). */
        function cmd(id, label, glyph, run, more) {
            var c = Object.assign({ id: id, label: label, glyph: glyph || undefined, run: run }, more || {});
            if (/^(cv-|shape-)/.test(id) || id === 'connect') {
                c.toolbarLabel = function () { return (typeof c.label === 'function' ? c.label.call(c) : c.label) + (/^cv-/.test(id) ? ' (Canvas)' : ' (D2)'); };
            }
            if (c.builtIn) { c.shortcut = window.yavcUtil.shortcut(c.builtIn); c.bind = false; delete c.builtIn; }
            return c;
        }
        function isD2() { var t = current(); return !!(t && t.kind === 'd2'); }
        function isCanvas() { var t = current(); return !!(t && t.kind === 'canvas'); }
        /* Why a shape command cannot run now, or null when it can: needs is
           false for none, true for anything selected, 'shape' for a shape.
           PUDL shows it on the disabled control and tells an agent. */
        function whyNot(needs) {
            var t = current();
            if (readOnly(t)) return mutationWhy(t);
            if (t && t.kind === 'canvas') { return canvasWhy(needs); }
            if (!t || t.kind !== 'd2') { return 'Only D2 diagrams and canvases are edited on their drawings.'; }
            if (t.problems && t.problems.length) { return 'The text has a mistake in it; put it right first.'; }
            if (!t.canvas || t.outlineText !== t.text) { return 'The drawing is not ready yet.'; }
            var k = t.canvas.selected();
            if (needs === true && !k) { return 'Select a shape or a connection first.'; }
            if (needs === 'shape' && (!k || isConnection(k))) { return 'Select a shape first.'; }
            return null;
        }
        function commandTable() {
            var table = [
                cmd('new', 'New Diagram', 'file', null, { items: D2_STYLES.map(function (s) { return { id: 'new-' + s.id, label: s.label, run: function () { newDiagram(s, 'd2'); } }; }) }),
                cmd('new-mermaid', 'New Mermaid Diagram', 'file', null, { items: MERMAID_STYLES.map(function (s) { return { id: 'new-' + s.id, label: s.label, run: function () { newDiagram(s, 'mermaid'); } }; }) }),
                cmd('new-canvas', 'New Canvas', 'file', null, { items: CANVAS_STYLES.map(function (s) { return { id: 'new-canvas-' + s[0], label: s[1], run: function () { return newCanvas(s[0]); } }; }) }),
                cmd('convert-canvas', 'Convert to Canvas…', 'copy', function () { return convertDiagram('canvas'); }, {hidden:function(){return !isD2();}}),
                cmd('convert-d2', 'Convert to D2…', 'copy', function () { return convertDiagram('d2'); }, {hidden:function(){return !isCanvas();}}),
                cmd('open', 'Open…', 'folder', open, { shortcut: 'Mod+O' }),
                cmd('save', 'Save', 'yavc-save', save, { shortcut: 'Mod+S', disabled: function () { var t = current(); return !t || (resources ? !t.path ? !resources.canSaveAs() : !resources.canWrite(t.resource) : !t.path || !H.fs || !H.fs.writeText); }, reason: function () { var t = current(); return mutationWhy(t) || (!resources && t && !t.path ? 'This host cannot create a destination. Download the source.' : t && t.recoveryConflict ? 'Recovered edits have no matching file revision. Use Save As or download the source.' : undefined); } }),
                cmd('save-as', 'Save As…', 'yavc-save-as', saveAs, { disabled: function () { return !current() || !resources || !resources.canSaveAs(); }, reason: function () { return !resources || !resources.canSaveAs() ? 'This host does not offer conditional Save As. Download the source to keep a copy.' : undefined; } }),
                cmd('close-diagram', 'Close Diagram', null, function () { closeTab(active); }, { disabled: function () { return !current(); } }),
                cmd('undo', 'Undo', 'yavc-undo', function () { if (cm && !readOnly(current())) { CM.undo(cm); } }, { builtIn: 'Mod-z', glyphOnly: true, disabled: function () { return readOnly(current()); }, reason: function () { return mutationWhy(); } }),
                cmd('redo', 'Redo', 'yavc-redo', function () { if (cm && !readOnly(current())) { CM.redo(cm); } }, { builtIn: 'Mod-y', glyphOnly: true, disabled: function () { return readOnly(current()); }, reason: function () { return mutationWhy(); } }),
                cmd('find', 'Find and Replace…', 'search', openFind, { builtIn: 'Mod-f' }),
                cmd('language-restart', 'Restart Language Assistance', 'reload', function () { if (lsp && cm) { lsp.restart(); cm.dispatch({ effects: lspComp.reconfigure(lsp.extension('canvas', readOnly(current()))) }); } }, { hidden: function () { return !isCanvas(); }, disabled: function () { return !lsp || !cm; } }),
                cmd('go-to-line', 'Go to Line…', null, goToLine, { builtIn: 'Mod-Alt-g' }),
                cmd('edit-in-editor', 'Open in the Editor', 'pencil', editInEditor, { disabled: function () { var c = current(); return !(c && c.path); } }),
                cmd('export-svg', 'Download as SVG', 'download', downloadSVG, { disabled: function () { return !drawable(); } }),
                cmd('export-png', 'Download as PNG', 'download', null, { disabled: function () { return !drawable(); }, items: [1, 2, 3].map(function (k) { return { id: 'export-png-' + k, label: k === 1 ? 'Actual Size' : k + ' Times the Size', run: function () { downloadPNG(k); } }; }) }),
                cmd('export-source', 'Download the Text', 'download', downloadSource, { disabled: function () { return !current(); } }),
                cmd('copy-embed', 'Copy as Embed', 'copy', copyEmbed, { disabled: function () { return !current(); } }),
                cmd('format-text', 'Tidy the Text', 'code', formatText, { hidden: function () { return !isD2() && !isCanvas(); }, disabled: function () { return readOnly(current()); }, reason: function () { return mutationWhy(); } }),
                cmd('shape-add', 'Add Shape', 'plus', function () { addShape(false); }, { builtIn: 'n', hidden: function () { return !isD2(); }, disabled: function () { return !!whyNot(false); }, reason: function () { return whyNot(false); } }),
                cmd('shape-add-inside', 'Add Shape Inside', null, function () { addShape(true); }, { builtIn: 'Shift-n', hidden: function () { return !isD2(); }, disabled: function () { return !!whyNot('shape'); }, reason: function () { return whyNot('shape'); } }),
                cmd('connect', 'Connect To…', 'yavc-connect', function () { if (connecting) { stopConnect('Stopped connecting.'); } else { startConnect(); } }, { builtIn: 'c', hidden: function () { return !isD2(); }, checked: function () { return !!connecting; }, disabled: function () { return !connecting && !!whyNot('shape'); }, reason: function () { return connecting ? null : whyNot('shape'); } }),
                cmd('shape-label', 'Edit Label…', 'pencil', function () { editLabel(); }, { builtIn: 'Enter', hidden: function () { return !isD2(); }, disabled: function () { return !!whyNot(true); }, reason: function () { return whyNot(true); } }),
                cmd('shape-rename', 'Rename…', null, renameShape, { builtIn: 'F2', hidden: function () { return !isD2(); }, disabled: function () { return !!whyNot('shape'); }, reason: function () { return whyNot('shape'); } }),
                { id: 'shape-type', label: 'Shape', glyph: 'square', hidden: function () { return !isD2(); }, disabled: function () { return !!whyNot('shape'); }, reason: function () { return whyNot('shape'); },
                    current: function () { var t = current(), s = t && shapeOf(t, selection()); var f = s && SHAPES.find(function (x) { return x[0] === s.shape; }); return f ? f[1] : null; },
                    items: SHAPES.map(function (sh) { return { id: 'shape-type-' + sh[0], label: sh[1], choice: 'dg-shape', value: sh[0], checked: function () { var t = current(), s = t && shapeOf(t, selection()); return !!s && s.shape === sh[0]; }, run: function () { setShape(sh[0]); } }; }) },
                { id: 'tool-select', label: 'Select', glyph: 'yavc-select', hidden: function () { return !isD2() && !isCanvas(); },
                    checked: function () { return !tool && !canvasPan; }, run: function () { setTool(null); } }
            ].concat(PALETTE.map(function (k) {
                return { id: 'tool-' + k[0], label: k[1], glyph: 'yavc-shape-' + k[0], // check-ignore(glyph-defined): one glyph for each PALETTE kind, each defined on :root in diagrams.css as --glyph-yavc-shape-<kind>
                    hidden: function () { return (!isD2() && !isCanvas()) || (k[0] === 'rounded' && !isCanvas()); }, disabled: function () { return !!whyNot(false); }, reason: function () { return whyNot(false); },
                    checked: function () { return tool === k[0]; }, run: function () { setTool(tool === k[0] ? null : k[0]); } };
            })).concat([
                cmd('shape-move-into', 'Move Into…', null, moveInto, { hidden: function () { return !isD2(); }, disabled: function () { return !!whyNot('shape'); }, reason: function () { return whyNot('shape'); } }),
                cmd('shape-move-top', 'Move to the Top', null, function () { moveTo(selection(), null); }, { hidden: function () { return !isD2(); }, disabled: function () { var k = selection(); return !!whyNot('shape') || k.indexOf('.') < 0; }, reason: function () { var w = whyNot('shape'); return w || (selection().indexOf('.') < 0 ? 'It is at the top already.' : null); } }),
                cmd('shape-delete', 'Delete', 'yavc-bin', deleteSelected, { builtIn: 'Delete', danger: true, hidden: function () { return !isD2(); }, disabled: function () { return !!whyNot(true); }, reason: function () { return whyNot(true); } }),
                cmd('redraw', 'Draw Again', 'reload', function () { drawNow(); }),
                cmd('toggle-inspector', 'Show the Inspector', 'panel', toggleInspector, { hidden: function () { return !isD2() && !isCanvas(); }, checked: function () { return !!settings.inspector; } }),
                cmd('toggle-outline', 'Show the Outline', 'panel', toggleOutline, { hidden: function () { return !isCanvas() && !isD2(); }, checked: function () { return !!settings.outline; } }),
                cmd('fit', 'Fit the Drawing to the Pane', 'maximize', function () { if (isCanvas()) { var t = current(); if (t.cv) { t.cv.fit(); } return; } settings.fit = !settings.fit; applyFit(); keepSetting('fit', settings.fit); commandsChanged(); }, { checked: function () { return !isCanvas() && !!settings.fit; } }),
                { id: 'dock', label: 'Drawing', glyph: 'dock', current: function () { var d = DOCKS.find(function (x) { return x[0] === settings.dock; }); return d ? d[1] : null; },
                    items: DOCKS.map(function (d) { return { id: 'dock-' + d[0], label: d[1], choice: 'dg-dock', value: d[0], checked: function () { return settings.dock === d[0]; }, run: function () { setDock(d[0]); } }; }) }
            ]);
            VIEWS.forEach(function (v) {
                table.push({ id: 'view-' + v[0], label: v[0] === 'preview' ? function () { return isD2() || isCanvas() ? 'Visual' : 'Preview'; } : v[1], choice: 'dg-view', choiceLabel: 'View', value: v[0], checked: function () { return viewOf(current()) === v[0]; }, run: function () { setView(v[0]); } });
            });
            return table.concat(canvasCommands(), commonToolbar ? commonToolbar.commands() : [], [
                cmd('drawing-tool', 'Drawing Tool', 'square', null, { hidden: function () { return !isD2() && !isCanvas(); }, items: ['tool-select'].concat(PALETTE.map(function (k) { return 'tool-' + k[0]; })) }),
                cmd('view-mode', 'View', 'panel', null, { toolbarChoice: 'dg-view', toolbarPriority: 50, items: ['view-preview', 'view-text', 'view-split'] })
            ]);
        }
        function toolbars() {
            if (commonToolbar) { return []; }
            return [{ label: 'Diagrams', ribbon: 'hide', groups: [
                { label: 'File', glyph: 'file', priority: 3, items: ['new', 'open', 'save', 'export-svg', 'export-png'] },
                { label: 'Edit', glyph: 'pencil', priority: 4, items: ['undo', 'redo', 'find', 'format-text', 'copy-embed'] },
                { label: 'Shape', glyph: 'square', priority: 5, items: ['shape-add', 'connect', 'shape-label', 'shape-type', 'shape-delete', 'cv-new', 'cv-connect', 'cv-delete', 'cv-pan', 'cv-zoom-in', 'cv-zoom-out'] },
                { label: 'View', glyph: 'panel', priority: 1, end: true, items: [{ choice: 'dg-view' }, 'dock', 'fit', 'toggle-inspector', 'toggle-outline'] }
            ] }];
        }
        /* Diagrams' help is a page of the site, /help/diagrams, which the
           site's Help menu links to (DESIGN.md, decision 19). */
        function menus() {
            return { titles: [
                { label: 'Diagrams', glyph: 'document', items: H.identity ? H.identity(root, []) : [] },
                { id: 'file', label: 'File', glyph: 'file', items: ['new', 'new-mermaid', 'new-canvas', 'open', 'save', 'save-as', '-', 'convert-canvas', 'convert-d2', 'edit-in-editor', '-', 'export-svg', 'export-png', 'export-source', '-', 'close-diagram'] },
                { id: 'edit', label: 'Edit', glyph: 'pencil', items: ['undo', 'redo', '-', 'cv-copy', 'cv-paste', 'cv-duplicate', 'cv-delete', 'copy-embed', '-', 'cv-select-all', 'cv-select-items', '-', 'find', 'go-to-line', 'cv-next', 'cv-previous', '-', 'format-text', 'redraw', 'language-restart'] },
                { id: 'shape', label: 'Shape', glyph: 'square', items: (isD2() || isCanvas() ? [{ label: 'Drawing Tool', glyph: 'square', items: ['tool-select'].concat(PALETTE.map(function (k) { return 'tool-' + k[0]; })) }, '-'] : []).concat(['shape-add', 'shape-add-inside', 'connect', 'cv-new', 'cv-group', 'cv-connect', 'cv-reconnect-start', 'cv-reconnect-end', 'cv-resize', 'cv-edit-text', '-', { label: 'Align and Distribute', items: ['cv-align-left', 'cv-align-center', 'cv-align-right', 'cv-align-top', 'cv-align-middle', 'cv-align-bottom', '-', 'cv-distribute-horizontal', 'cv-distribute-vertical'] }, '-', 'shape-label', 'shape-rename', 'shape-type', '-', 'shape-move-into', 'shape-move-top', 'cv-left', 'cv-right', 'cv-up', 'cv-down', 'cv-forward', 'cv-back', '-', 'shape-delete']) }
            ], into: { view: ['view-preview', 'view-text', 'view-split', '-', 'dock', 'fit', 'toggle-inspector', 'toggle-outline'].concat(commonToolbar ? ['-', 'common-toolbar-show', 'common-toolbar-customize'] : [], ['-', 'cv-pan', 'cv-pan-left', 'cv-pan-right', 'cv-pan-up', 'cv-pan-down', 'cv-zoom-in', 'cv-zoom-out', 'cv-actual']) } };
        }
        root.addEventListener('pudl:contextmenu', function (e) {
            var t = e.detail.target;
            var own = function (ids) { return window.pudlCommands ? window.pudlCommands.menuRows(ids, root) : ids; };
            if (t.closest && t.closest('.dg-stagebox')) {
                var rows = isCanvas() ? ['cv-edit-text', 'cv-new', 'cv-group', 'cv-connect', 'cv-reconnect-start', 'cv-reconnect-end', 'cv-resize', '-', 'cv-copy', 'cv-paste', 'cv-duplicate', 'cv-forward', 'cv-back', 'cv-delete', '-'] : isD2() && selection() ? ['shape-label', 'shape-rename', 'shape-type', 'connect', 'shape-add-inside', '-', 'shape-delete', '-'] : isD2() ? ['shape-add', '-'] : [];
                e.detail.add(own(rows.concat(['copy-embed', 'export-svg', 'export-png', '-', 'fit', 'redraw'])));
            }
            else if (t.closest && t.closest('.dg-text')) { e.detail.add(own(['undo', 'redo', '-', 'find', '-', 'format-text', 'copy-embed', '-', 'view-text', 'view-preview', 'view-split'])); }
        });

        /* === Start ======================================================== */

        /* A canvas takes its colours from the page, so it is drawn again
           when the site's mode, colour theme or skin changes, as d2.js
           draws D2 again. */
        function modeChanged() { var t = current(); if (t && t.kind === 'canvas') { drawNow(); } }
        var modeWatch = new MutationObserver(modeChanged);
        modeWatch.observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme', 'data-palette', 'data-skin'] });
        var darkQuery = window.matchMedia ? window.matchMedia('(prefers-color-scheme: dark)') : null;
        if (darkQuery) { darkQuery.addEventListener('change', modeChanged); }

        var booted = (async function () {
            var got = await Promise.all([kit(), modes(), once('tree', (H.pudl || '/pudl/') + 'pudl-tree.js', 'pudlTree')]);
            var resourceHost = H.contentResources;
            if (!resourceHost && H.contentResourceAdapter && H.snapshot && H.snapshot.version === 2 && typeof H.contentResourceAdapter.available === 'function' && await H.contentResourceAdapter.available()) resourceHost = H.contentResourceAdapter;
            if (destroyed) return;
            if (resourceHost) {
                var resourceAPI = await once('resources', asset('content-kit/resources.js'), 'yavcContentResources');
                resources = resourceAPI.create(resourceHost);
                recovery = await once('recovery', asset('content-kit/recovery.js'), 'yavcContentRecovery');
                if (recovery.version !== 1 || typeof resources.baseline !== 'function' || typeof resources.canSaveAs !== 'function') throw new Error('The content recovery modules are incompatible. Reload with matching package assets.');
            }
            CM = await got[0].libs();
            lspComp = new CM.Compartment();
            try {
                var languageServices = await got[0].languageServices();
                if (destroyed) { return; }
                lsp = languageServices.create(CM, function (message, detail) { say(message, detail && detail.persistent ? 'warn' : true); });
            } catch (err) { if (!destroyed) { say('Language assistance could not load. Text editing remains available.', 'warn'); } }
            if (destroyed) { return; }
            if (H.config && H.config.load) { try { Object.assign(settings, (await H.config.load('diagrams', SETTINGS)) || {}); } catch (err) { } }
            if (destroyed) { return; }
            fitInspector();
            applyFit();
            if (resources && H.snapshot) {
                snap = H.snapshot.join(root, resourceSnapshot, { validate: recovery.validate });
                recoveryKept = snap.kept;
                if (typeof snap.flush !== 'function') { snap.leave(); snap = null; throw new Error('Snapshot version 2 must acknowledge recovery writes.'); }
            }
            else if (H.snapshot) {
                snap = H.snapshot.join(root, legacySnapshot, { validate: function (value) { if (value && value.version === 2) throw new Error('This resource recovery snapshot needs its original resource host. Its saved copy has been retained.'); } });
            }
            /* The tabs kept across a reload come back first, unsaved text and
               all; then the address's file comes to the front. */
            var kept = snap && snap.kept;
            if (kept && kept.tabs) {
                for (var i = 0; i < kept.tabs.length; i++) {
                    var k = kept.tabs[i], text = k.text;
                    var restored = null;
                    if (resources && k.path) {
                        try { restored = await recovery.restore(resources, k.path, k.baseline, text); text = restored.text; }
                        catch (err) { if (destroyed) return; say('A saved tab could not be reopened: ' + err.message, 'warn'); continue; }
                    } else if (text == null && k.path && H.fs && H.fs.readText) { text = await H.fs.readText(k.path).catch(function () { return null; }); }
                    if (destroyed) return;
                    if (text == null) { continue; }
                    var t = makeTab(text, k.path || null, Object.hasOwn(EXT, k.kind) ? k.kind : null, k.view);
                    t.format=k.format;
                    if (restored) { t.resource = restored.snapshot; t.recoveryBaseline = restored.baseline; t.recoveryConflict = restored.conflict; if (k.name) t.name = k.name; }
                    if (!k.path && k.name && /^untitled-\d+\.(d2|mmd|canvas)$/.test(k.name)) { t.name = k.name; untitled = Math.max(untitled, +/\d+/.exec(k.name)[0]); }
                    t.modified = k.text != null;
                    tabs.push(t);
                }
                if (tabs.length) { activateTab(Math.max(0, Math.min(kept.active || 0, tabs.length - 1))); }
            }
            var file = start.get('file'), view = start.get('view');
            if(!view && file && start.get('initial-view')){var preference=H.config && H.config.stored ? await H.config.stored('diagrams') : {};if(destroyed)return;if(!VIEWS.some(function (v) {return v[0]===preference.view;}))view=start.get('initial-view');}
            if(view==='visual')view='preview';
            if (file) { await openPath(file, view).catch(function (err) { say(err.message, true); return false; }); }
            if (destroyed) return;
            if (!tabs.length) { tabs.push(makeTab(D2_STYLES[0].text, null, 'd2')); activateTab(0); }
            if (view && !file) { setView(view); }
            if (start.has('item') && current() && current().kind === 'd2') { current().pending = start.get('item'); await drawNow(); }
            recoveryReady = true;
            documentReady = true;
            if (documentWindows && documentWindows.mode(root) === 'single' && tabs.length > 1) {
                var oldTabs = tabs.slice(), front = current(), split = await documentWindows.split(root, 'diagrams', resources ? resourceSnapshot() : legacySnapshot());
                split.moved.forEach(function (i) { var at = tabs.indexOf(oldTabs[i]); if (at >= 0) { var removed = tabs.splice(at, 1)[0]; if (removed.canvas) removed.canvas.destroy(); if (removed.cv) removed.cv.destroy(); } });
                active = -1; activateTab(tabs.indexOf(front)); if (snap) snap.flush();
                if (split.error) say('Some recovered documents remain together. ' + split.error, 'warn');
            }
            if (tabs.some(function (t) { return t.recoveryConflict; })) say('Recovered edits have no matching file revision. ' + (resources && resources.canSaveAs() ? 'Use Save As to keep a copy, or download the source.' : 'Download the source to keep a copy before reopening the file.') + ' The file has not been overwritten.', 'warn');
            announce();
        })().catch(function (err) { startupFailed=true;if (!destroyed) { root.innerHTML = '<div class="notice danger" role="alert"><div class="notice-content"><p class="notice-title">Diagrams could not start</p><p class="notice-body">' + esc(err.message) + '</p></div></div>'; } });

        /* === The live actions (actions.json, decision 13) ================= */

        function tabNo(i) {
            if (i == null) { if (active < 0) { throw new Error('no diagram is open'); } return active; }
            i = Math.floor(Number(i));
            if (!(i >= 1 && i <= tabs.length)) { throw new Error('there is no diagram ' + i + '; ' + tabs.length + ' are open'); }
            return i - 1;
        }
        function summary(t, i) { return { index: i + 1, name: tabTitle(t), path: t.path, kind: t.kind, modified: !!t.modified, view: viewOf(t) }; }
        async function front(i) { var k = tabNo(i); if (k !== active) { activateTab(k); } return current(); }
        async function drawnNow() { await drawNow(); var t = current(); return { drawn: !(t.problems && t.problems.length), problems: (t.problems || []).map(function (p) { return { message: p.message, line: p.line || 0 }; }) }; }
        var liveActions = {
            'diagrams@1': async function () { await booted; stash(); return { output: { active: active + 1, diagrams: tabs.map(summary) } }; },
            'diagram@1': async function (input) {
                await booted;
                var i = tabNo(input.index), t = tabs[i];
                if (i === active) { stash(); }
                return { output: Object.assign(summary(t, i), { text: t.text, problems: (t.problems || []).map(function (p) { return { message: p.message, line: p.line || 0 }; }) }) };
            },
            'new@1': async function (input) {
                await booted;
                var routed = await routeDocument('new', input); if (routed) return routed.result;
                var kind = input.kind === 'mermaid' || input.kind === 'canvas' ? input.kind : 'd2';
                var text;
                if (kind === 'canvas') { await canvasEditor(); text = window.yavcCanvasModel.template(input.style || 'basic').text; }
                else {
                    var list = kind === 'd2' ? D2_STYLES : MERMAID_STYLES;
                    var style = list.find(function (s) { return s.id === input.style || s.id === 'mm-' + input.style; }) || list[0]; text = style.text;
                }
                var t = openInTab(input.text != null ? String(input.text) : text, null, kind, null);
                if (input.text != null) { t.modified = true; commandsChanged(); }
                return { output: Object.assign(summary(t, active), await drawnNow()) };
            },
            'open-file@1': async function (input) {
                await booted;
                if(!tabs.some(function (t) {return t.path===input.path;})){var routed=await routeDocument('open-file',input);if(routed)return routed.result;}
                var ok = await openPath(String(input.path), input.view);
                if (!ok) { throw new Error('there is no file at ' + input.path); }
                return { output: summary(current(), active) };
            },
            'set-text@1': async function (input) {
                await booted;
                await front(input.index);
                requireEditable(current());
                replaceText(String(input.text));
                return { output: Object.assign(summary(current(), active), await drawnNow()) };
            },
            'save@1': async function (input) {
                await booted;
                var t = await front(input.index);
                if (!resources && (!t.path || input.path && input.path !== t.path)) throw new Error('This host cannot create a destination. Download the source to keep a copy.');
                var submitted = t.text,format=resources?sourceFormat(t):null;
                return queueSave(t, async function (ticket) {
                    var destination = input.path ? String(input.path) : t.path;
                    if (resources && destination !== t.path) {
                        if (!destination) throw new Error('Give an explicit destination for the new resource.');
                        var created = await resources.create(destination, submitted,format);
                        if (destroyed || tabs.indexOf(t) < 0) throw new Error('The resource was created after its diagram was closed.');
                        savedAs(t, created.resource, submitted, ticket, created);
                        return { output: summary(t, tabs.indexOf(t)) };
                    }
                    if (input.path) {
                        /* An agent's diagram never takes the name of a file that
                           exists, or saving would overwrite it unasked. */
                        var to = String(input.path);
                        if (to !== t.path && H.fs && H.fs.readText) {
                            var there = await H.fs.readText(to).catch(function () { return null; });
                            if (there != null) { throw new Error('there is already a file at ' + to + '; open it, or give the diagram another name'); }
                        }
                    }
                    if (!destination) { throw new Error('the diagram has no file yet; give a path such as ~/diagrams/plan.d2'); }
                    if (!resources && (!H.fs || !H.fs.writeText)) { throw new Error('there is no file service here'); }
                    if (destroyed || tabs.indexOf(t) < 0) { throw new Error('The diagram was closed before it could be saved.'); }
                    var err = await writeResource(t, destination, submitted);
                    if (err) { throw new Error('could not save: ' + err); }
                    if (destroyed || tabs.indexOf(t) < 0) { throw new Error('The file was saved after its diagram was closed.'); }
                    saved(t, destination, submitted, ticket, true);
                    if (t === current()) { el.where.textContent = t.path; el.where.title = el.where.textContent; say('Saved ' + t.path + (t.modified ? '; newer edits are still unsaved.' : '.')); announce(); }
                    commandsChanged();
                    keep();
                    return { output: summary(t, tabs.indexOf(t)) };
                });
            },
            'close@1': async function (input) {
                await booted;
                var i = tabNo(input.index), before = tabs.length;
                await closeTab(i);
                return { output: { closed: tabs.length < before || before === 1, open: tabs.length } };
            },
            'view@1': async function (input) {
                await booted;
                if (input.dock) { setDock(String(input.dock)); }
                if (input.mode) { setView(String(input.mode)); }
                return { output: { mode: viewOf(current()), dock: settings.dock } };
            },
            'outline@1': async function (input) {
                await booted;
                var t = await front(input.index);
                if (t.kind !== 'd2') { throw new Error('an outline is read from D2 diagrams; this one is ' + KIND_NAME[t.kind]); }
                var out = await (await d2()).call('outline', { text: t.text });
                return { output: { shapes: out.shapes || [], connections: out.connections || [], boards: out.boards || [], problems: out.problems || [] } };
            },
            'convert@1': async function (input) { await booted; await front(input.index); return {output:await conversion(input.format)}; },
            'format@1': async function (input) {
                await booted;
                var t = await front(input.index);
                requireEditable(t);
                var original = t.text;
                if (t.kind === 'mermaid') { throw new Error('only D2 and canvas text is tidied; this diagram is Mermaid'); }
                var out;
                if (t.kind === 'canvas') {
                    out = await tidyCanvas(t.text);
                    if (out.problem) { throw new Error('the text has a mistake: ' + out.problem); }
                } else {
                    out = await (await d2()).call('format', { text: t.text });
                    if (out.problems && out.problems.length) { throw new Error('the text has a mistake on line ' + (out.problems[0].line || 1) + ': ' + out.problems[0].message); }
                }
                if (destroyed || t !== current() || t.text !== original || readOnly(t)) throw new Error('The diagram changed before formatting finished.');
                var changed = out.text !== t.text;
                replaceText(out.text);
                return { output: { index: active + 1, changed: changed } };
            },
            'edit@1': async function (input) {
                await booted;
                var t = await front(input.index);
                requireEditable(t);
                if (t.kind !== 'd2') { throw new Error('only D2 diagrams are edited shape by shape; this one is ' + KIND_NAME[t.kind]); }
                if (!t.canvas) { await drawNow(); }
                var req = { op: String(input.op), key: String(input.key) };
                ['value', 'name', 'from', 'to'].forEach(function (k) { if (input[k] != null) { req[k] = String(input[k]); } });
                if (input.op === 'set' && input.value === null) { req.value = null; }
                if (input.descendants) { req.descendants = true; }
                var out = await applyEdit(req);
                if (!out) { throw new Error('the change could not be made; the diagram may have a mistake in it'); }
                if (out.problems && out.problems.length) { throw new Error(out.problems[0].message); }
                return { output: Object.assign({ key: out.key || '' }, await drawnNow()) };
            },
            'canvas-view@1': async function (input) {
                await booted;
                var t = current();
                if (!t || t.kind !== 'canvas') { throw new Error('Open a canvas before changing its drawing view.'); }
                await drawNow();
                if (destroyed || t !== current() || canvasWhy(false)) { throw new Error('The current canvas is not ready.'); }
                if (input.op === 'select') {
                    var key = input.key || null, kind = input.kind || 'node';
                    var items = input.items || (key ? [{ id: key, kind: kind }] : []);
                    items.forEach(function (s) {
                        if (['node', 'edge'].indexOf(s.kind) < 0 || !(s.kind === 'edge' ? t.cvDoc.edges : t.cvDoc.nodes.filter(function (n) { return !window.yavcCanvas.isAnchor(n); })).some(function (x) { return x.id === s.id; })) { throw new Error('There is no selectable ' + s.kind + ' with that ID.'); }
                    });
                    t.cv.select(items);
                } else if (input.op === 'pan') { t.cv.pan(input.dx || 0, input.dy || 0); }
                else if (input.op === 'zoom') { t.cv.zoom(input.factor || 1); }
                else if (input.op === 'fit') { t.cv.fit(); }
                else if (input.op === 'actual') { t.cv.actual(); }
                else { throw new Error('Unknown canvas view operation.'); }
                return { output: { applied: true } };
            },
            'canvas-edit@1': async function (input) {
                await booted;
                var t = await front(input.index);
                requireEditable(t);
                if (t.kind !== 'canvas') { throw new Error('Open a JSON Canvas file before using canvas-edit.'); }
                await canvasEditor();
                if (destroyed || t !== current()) { throw new Error('The open canvas changed before the edit could run.'); }
                var out = await canvasEdit(input, t.text, true);
                return { output: Object.assign({ key: out.key || '' }, await drawnNow()) };
            },
            'render@1': async function (input) {
                await booted;
                var t = await front(input.index);
                var got = await drawnNow();
                if (!got.drawn) { throw new Error('the diagram has a mistake: ' + (got.problems[0] ? got.problems[0].message : 'unknown')); }
                if (input.format === 'png') {
                    var scale = Math.max(1, Math.min(3, Math.floor(Number(input.scale) || 2)));
                    var blob = await pngOf(t, scale);
                    return { output: { name: stem(t) + '.png', width: t.size ? t.size.w * scale : 0, height: t.size ? t.size.h * scale : 0 }, artifacts: [{ key: 'png', name: stem(t) + '.png', mime: 'image/png', data: new Uint8Array(await blob.arrayBuffer()) }] };
                }
                return { output: { name: stem(t) + '.svg', width: t.size ? t.size.w : 0, height: t.size ? t.size.h : 0 }, artifacts: [{ key: 'svg', name: stem(t) + '.svg', mime: 'image/svg+xml', data: t.svg }] };
            }
        };
        Object.keys(liveActions).forEach(function (id) {var handler=liveActions[id];liveActions[id]=async function (input) {await booted;if(destroyed || startupFailed)throw new Error('This workspace is no longer available.');return handler(input);};});
        var detachLive = window.pudlApplets && window.pudlApplets.attach ? window.pudlApplets.attach('yavc.diagrams', liveActions) : null;
        if (documentWindows) unbindDocuments = documentWindows.bind(root, {
            application: 'diagrams', resource: function () {return current() && current().path;},
            accept: async function (request) { receivingDocument = true; try { return await liveActions[request.operation + '@1'](request.input); } finally { receivingDocument = false; } },
            flush: function () { return !!snap && snap.flush(); }
        });
        var commonToolbar = H.commandToolbar ? H.commandToolbar.attach(root, { app: 'diagrams', label: 'Diagrams', table: commandTable,
            defaults: ['save', '-', 'undo', 'redo', '-', 'drawing-tool', 'shape-add', 'connect', 'cv-new', 'cv-connect', '-', 'fit', 'cv-zoom-in', 'cv-zoom-out', 'view-mode'] }) : null;

        return {
            ready: booted,
            state: stateString,
            setState: function (s) {
                var p = new URLSearchParams(String(s || '').replace(/^\?/, ''));
                booted.then(async function () {
                    if (destroyed || startupFailed) return;
                    var file = p.get('file'), view = p.get('view');
                    var t = current();
                    if (file) { try { await open({resource:file,view:view,initialView:p.get('initial-view')}); } catch (err) { say(String((err && err.message) || err), true); return; } }
                    else if (view) { setView(view==='visual'?'preview':view); }
                    t = current();
                    if (t && t.kind === 'd2' && p.has('item')) {
                        if (t.canvas && t.outlineText === t.text) { selectKey(t, p.get('item'), 'state'); t.canvas.reveal(); }
                        else { t.pending = p.get('item'); }
                    }
                });
            },
            commandTable: commandTable, toolbars: toolbars, menus: menus,
            destroy: function () {
                if (unbindDocuments) unbindDocuments();
                if (commonToolbar) { commonToolbar.destroy(); }
                destroyed = true;
                lifetime.abort();
                finishInlineText(false); inspectorSize.disconnect();
                modeWatch.disconnect();
                if (darkQuery) { darkQuery.removeEventListener('change', modeChanged); }
                tabs.forEach(function (t) { if (t.canvas) { t.canvas.destroy(); } if (t.cv) { t.cv.destroy(); } });
                clearTimeout(drawTimer);
                settle(null, []);
                if (detachLive) { detachLive(); }
                if (cm) { cm.destroy(); }
                if (lsp) { lsp.dispose(); }
                if (snap) { snap.leave(); }
                unguard();
            }
        };
    }

    window.pudlApplets.register('diagrams', { init: init });
})();
