/* Diff: the Editor's diff view. A window compares two texts of one file,
   the "before" and the "after", inline or side by side, drawn with
   CodeMirror's merge view (vendor/codemirror-6.43, @codemirror/merge) and
   themed from PUDL's tokens as the Editor's text is. One viewer serves the
   Git applet, reviews and agents' suggestions: whoever opens a diff hands
   it the actions to show for the whole file, for a hunk and for ticked
   lines, and each press comes back to the opener with the hunk and the
   lines it applies to. Paul decided on 25 October 2026 that diffs open
   here rather than in the Git window (dinah-git, docs/design/overhaul.md).

   window.dinahDiff.open(spec) opens a window, or brings the window with
   the same key to the front with the new texts, and resolves with the
   window's key. spec is:

     key       the comparison's identity, such as a repository path, a
               file path and a side; a second open with the same key
               reuses the window
     title     the window's title, by default the file's name
     detail    the title's detail, such as "internal/store · Modified,
               unstaged"
     path      the file's path, shown under the toolbar
     file      a workspace resource reference ({workspaceId, sourceId,
               path}) for Edit the File; without one, the path is looked
               for in the workspace's main folder
     language  a file name or language name for syntax colouring; the
               path when absent
     before    the text before the change
     after     the text after it
     mode      'inline' or 'split'; otherwise the mode last chosen
     readOnly  true; the texts are not edited here
     actions   [{ id, label, glyph, scope: 'file'|'hunk'|'line',
               primary, danger, confirm: { title, body, yes } }]; a line
               action's label is its verb, drawn as "Stage 3 lines"
     onAction(id, where)
               where is nothing for a file action, { hunk, before: { line,
               count }, after: { line, count } } for a hunk action, and the
               same with lines: [{ side: 'before'|'after', line }] for a
               line action, called once for each hunk that has ticked lines

   A hunk here is a chunk of CodeMirror's diff, numbered from 0 in the
   order of the file; before and after give each hunk's lines as a unified
   diff header does (@@ -line,count +line,count @@), so the opener need not
   agree with CodeMirror about where hunks begin. */
(function () {
  'use strict';

  function host() { return window.dinahHost; }
  const api = window.dinahHelpers.app;
  function message(err) { return window.dinahHelpers.message(err, 'sentence'); }

  /* The bundle is the Editor's; it is loaded once, by whichever asks
     first. */
  var cmReady = null;
  function loadCodeMirror() {
    if (window.CM && window.CM.MergeView) return Promise.resolve(window.CM);
    if (!cmReady) {
      cmReady = new Promise(function (resolve, reject) {
        var script = document.createElement('script');
        script.src = 'vendor/codemirror-6.43/codemirror.js';
        script.onload = function () { resolve(window.CM); };
        script.onerror = function () { cmReady = null; reject(new Error('The diff view could not be loaded.')); };
        document.head.appendChild(script);
      });
    }
    return cmReady;
  }

  /* The language for a file name or a language name, for syntax
     colouring; plain text when it is not one the bundle has. */
  function languageFor(CM, hint) {
    var h = String(hint || '').toLowerCase(), name = h.slice(h.lastIndexOf('/') + 1);
    var ext = name.indexOf('.') >= 0 ? name.slice(name.lastIndexOf('.') + 1) : name;
    function stream(mode) { return mode ? CM.StreamLanguage.define(mode) : []; }
    switch (ext) {
      case 'md': case 'markdown': return CM.markdown();
      case 'json': return CM.json();
      case 'js': case 'mjs': case 'cjs': case 'jsx': case 'javascript': return CM.javascript({ jsx: ext === 'jsx' });
      case 'ts': case 'tsx': case 'typescript': return CM.javascript({ typescript: true, jsx: ext === 'tsx' });
      case 'html': case 'htm': return CM.html();
      case 'css': return CM.css();
      case 'sh': case 'bash': case 'zsh': case 'shell': return stream(CM.shell);
      case 'go': return stream(CM.go);
      case 'py': case 'python': return stream(CM.python);
      case 'yml': case 'yaml': return stream(CM.yaml);
      case 'lua': return stream(CM.lua);
      case 'ps1': case 'psm1': case 'psd1': case 'powershell': return stream(CM.powerShell);
      case 'toml': return stream(CM.toml);
      case 'rs': case 'rust': return stream(CM.rust);
      case 'xml': case 'svg': case 'csproj': case 'xaml': case 'plist': return stream(CM.xml);
      case 'sql': return stream(CM.standardSQL);
      case 'c': case 'h': return stream(CM.c);
      case 'cpp': case 'cc': case 'cxx': case 'hpp': case 'hh': return stream(CM.cpp);
      case 'cs': case 'csharp': return stream(CM.csharp);
      case 'java': return stream(CM.java);
      case 'properties': case 'ini': case 'cfg': case 'env': return stream(CM.properties);
      case 'dockerfile': return stream(CM.dockerFile);
      default: return [];
    }
  }

  /* The theme is the Editor's (editor-applet.js), so the two look alike. */
  function makeTheme(CM) {
    if (window.dinahEditorTheme) return window.dinahEditorTheme(CM);
    return CM.EditorView.theme({ '&': { color: 'var(--text)', backgroundColor: 'var(--input-bg)', fontSize: 'var(--editor-size, 14px)' }, '.cm-scroller': { fontFamily: 'var(--editor-font, var(--mono))', lineHeight: '1.5' } });
  }

  var MODE_KEY = 'dinah-diff-mode';
  var MODES = { inline: 'Inline', split: 'Side by Side' };
  // The same diff settings as the merge view's own, so the hunks counted
  // here are the ones it draws.
  var DIFF = { scanLimit: 500 };
  var COLLAPSE = { margin: 3, minSize: 4 };

  /* The comparisons that have been opened, by their key, and the windows
     showing them, by the window's key. A window restored with the layout
     after a restart has no texts; it says so until it is opened again. */
  var specs = new Map(), windows = new Map();

  function normalize(spec) {
    var path = String(spec.path || '');
    var out = Object.assign({}, spec, {
      key: String(spec.key),
      title: String(spec.title || path.slice(path.lastIndexOf('/') + 1) || 'Diff'),
      detail: String(spec.detail || ''),
      path: path,
      language: spec.language || path,
      before: String(spec.before == null ? '' : spec.before).replace(/\r\n/g, '\n'),
      after: String(spec.after == null ? '' : spec.after).replace(/\r\n/g, '\n'),
      mode: MODES[spec.mode] ? spec.mode : '',
      actions: Array.isArray(spec.actions) ? spec.actions.filter(function (a) { return a && a.id && a.label && /^(file|hunk|line)$/.test(a.scope); }) : [],
      onAction: typeof spec.onAction === 'function' ? spec.onAction : function () {}
    });
    return out;
  }

  /* ---- The hunks ----------------------------------------------------------
     A chunk's lines on each side, as a unified diff header gives them: the
     first line and the count, with a side that has no lines placed after
     the line before the change, as git places it. */
  function linesOf(doc, from, to) {
    if (to <= from) return { line: Math.max(0, doc.lineAt(from).number - 1), count: 0 };
    var first = doc.lineAt(from).number, last = doc.lineAt(to - 1).number;
    return { line: first, count: last - first + 1 };
  }
  function hunksOf(chunks, docA, docB) {
    return chunks.map(function (c, i) {
      return { index: i, fromA: c.fromA, toA: c.toA, fromB: c.fromB, toB: c.toB, before: linesOf(docA, c.fromA, c.toA), after: linesOf(docB, c.fromB, c.toB) };
    });
  }
  function header(h) {
    function part(sign, r) { return sign + r.line + (r.count === 1 ? '' : ',' + r.count); }
    return '@@ ' + part('-', h.before) + ' ' + part('+', h.after) + ' @@';
  }

  function init(root, opts, item) {
    var win = root.closest('[data-win]'), key = opts.key || win.dataset.win;
    var spec = specs.get(item.path) || null, CM = null;
    var mode = '', views = [], merge = null, hunks = [], docA = null, docB = null;
    var ticked = new Set(), disposed = false, fileRef = null, scrollTimer = 0, wanted = null;
    var off = [];
    root.classList.add('diff-applet');

    /* The window's head is PUDL's folding toolbar of the view's commands
       (commandTable, below): the layout, the file's actions as the opener
       gave them, and the file itself. */
    function tbGroup(name, glyph, priority, inner, end) {
      return '<div class="toolbar-group" data-toolbar-group="' + name + '" data-toolbar-glyph="' + glyph + '" data-toolbar-priority="' + priority + '"' + (end ? ' data-toolbar-end' : '') + '>' + inner + '</div>';
    }
    root.innerHTML =
      '<div class="toolbar diff-head" role="toolbar" aria-label="Diff" data-toolbar-fold data-df-head>' +
        tbGroup('View', 'eye', 3, '<div class="seg sm" data-command-choice="diff-mode" aria-label="Layout"></div>' +
          '<button type="button" class="btn btn-sm" data-command="previous-change"></button><button type="button" class="btn btn-sm" data-command="next-change"></button>') +
        tbGroup('Changes', 'pencil', 4, '<span data-df-file-actions></span>') +
        tbGroup('File', 'document', 2, '<button type="button" class="btn btn-sm" data-command="edit-file"></button>', true) +
      '</div>' +
      '<p class="doc-meta diff-meta" data-df-meta></p>' +
      '<div class="diff-body" data-df-body data-ui-scroll-ignore></div>' +
      '<div class="diff-foot" data-df-foot><span data-df-summary></span><span class="diff-foot-hint" data-df-hint></span><span class="push" data-df-line-actions></span></div>';
    var $ = function (name) { return root.querySelector('[data-df-' + name + ']'); };

    function actionsOf(scope) { return spec ? spec.actions.filter(function (a) { return a.scope === scope; }) : []; }
    /* One primary button in a context: the first the opener marked. */
    function primaryOf(list) { return list.find(function (a) { return a.primary; }) || null; }

    /* ---- Running an action -------------------------------------------- */

    function run(action, where) {
      if (!spec) return Promise.resolve();
      var confirm = action.confirm;
      var asked = confirm && host() && host().ask ? host().ask(String(confirm.title || action.label), String(confirm.body || ''), String(confirm.yes || action.label), !!action.danger) : Promise.resolve(true);
      return asked.then(function (go) {
        if (!go || disposed) return;
        return spec.onAction(action.id, where);
      }).catch(function (err) { if (host()) host().toast(message(err), 'danger'); });
    }
    function whereOf(h) { return { hunk: h.index, before: h.before, after: h.after }; }
    function runHunk(action, h) { return run(action, whereOf(h)); }
    /* A line action runs once for each hunk that has ticked lines, with
       the lines of that hunk, in the order of the file. */
    function runLines(action) {
      var byHunk = hunks.map(function (h) { return Object.assign(whereOf(h), { lines: [] }); });
      ticked.forEach(function (id) {
        var parts = id.split(':'), side = parts[0], line = +parts[1];
        var h = hunks.find(function (x) { var r = side === 'before' ? x.before : x.after; return r.count && line >= r.line && line < r.line + r.count; });
        if (h) byHunk[h.index].lines.push({ side: side, line: line });
      });
      var calls = byHunk.filter(function (w) { return w.lines.length; });
      calls.forEach(function (w) { w.lines.sort(function (a, b) { return a.side === b.side ? a.line - b.line : a.side === 'before' ? -1 : 1; }); });
      ticked.clear();
      redrawGutters();
      drawFoot();
      return calls.reduce(function (p, w) { return p.then(function () { return run(action, w); }); }, Promise.resolve());
    }

    /* ---- The text ------------------------------------------------------ */

    /* Ticks live outside the editor state; an effect tells the gutters to
       draw again when one changes. */
    var redraw = null, Rows = null, HunkHeader = null;
    function classes() {
      if (Rows) return;
      redraw = CM.StateEffect.define();
      /* A gutter marker of one row for a line, or of several for the
         deleted lines the inline view shows as one block. A row is a
         number, a sign or a tick box. */
      Rows = class extends CM.GutterMarker {
        constructor(rows, cls) { super(); this.rows = rows; this.cls = cls || ''; this.elementClass = cls || ''; }
        eq(other) { return this.cls === other.cls && this.rows.length === other.rows.length && this.rows.every(function (r, i) { return r.text === other.rows[i].text && r.tick === other.rows[i].tick && r.on === other.rows[i].on; }); }
        toDOM() {
          var box = document.createElement('div');
          box.className = 'diff-rows';
          this.rows.forEach(function (r) {
            var row = document.createElement('div');
            row.className = 'diff-row';
            if (r.tick) {
              var input = document.createElement('input');
              input.type = 'checkbox';
              input.className = 'diff-tick';
              input.checked = !!r.on;
              input.setAttribute('aria-label', 'Choose ' + (r.tick.split(':')[0] === 'before' ? 'removed' : 'added') + ' line ' + r.tick.split(':')[1]);
              input.addEventListener('change', function () { toggle(r.tick, input.checked); });
              row.appendChild(input);
            } else row.textContent = r.text;
            box.appendChild(row);
          });
          return box;
        }
      };
      /* The strip above a hunk: its header, as a unified diff writes it,
         and the hunk's actions. It sticks to the top while the hunk
         scrolls under it. */
      HunkHeader = class extends CM.WidgetType {
        constructor(h, withActions) { super(); this.h = h; this.withActions = withActions; }
        eq(other) { return other.h.index === this.h.index && other.withActions === this.withActions && other.h.fromB === this.h.fromB; }
        get estimatedHeight() { return 28; }
        ignoreEvent() { return true; }
        toDOM() {
          var h = this.h, strip = document.createElement('div');
          strip.className = 'diff-hunk';
          strip.dataset.hunk = String(h.index);
          var code = document.createElement('code');
          code.textContent = header(h);
          strip.appendChild(code);
          var words = document.createElement('span');
          words.textContent = 'Hunk ' + (h.index + 1) + ' of ' + hunks.length;
          strip.appendChild(words);
          if (this.withActions) {
            var acts = document.createElement('span');
            acts.className = 'diff-hunk-actions';
            actionsOf('hunk').forEach(function (a) {
              var b = document.createElement('button');
              b.type = 'button';
              b.className = 'btn btn-sm' + (a.danger ? ' btn-danger' : '');
              b.dataset.hunkAction = a.id;
              b.textContent = a.label;
              b.addEventListener('click', function () { runHunk(a, h); });
              acts.appendChild(b);
            });
            strip.appendChild(acts);
          }
          return strip;
        }
      };
    }
    function toggle(id, on) {
      if (on) ticked.add(id); else ticked.delete(id);
      redrawGutters();
      drawFoot();
    }
    function redrawGutters() {
      if (!redraw) return;
      views.forEach(function (v) { v.dispatch({ effects: redraw.of(null) }); });
    }
    function redrawn(u) { return u.transactions.some(function (tr) { return tr.effects.some(function (e) { return e.is(redraw); }); }); }

    /* The hunk a position of one side falls in, when it is a changed line. */
    function hunkAt(side, pos) {
      return hunks.find(function (h) { return side === 'before' ? pos >= h.fromA && pos < h.toA : pos >= h.fromB && pos < h.toB; }) || null;
    }
    /* The line's number on the other side of an unchanged line of the
       after text: its own number less the lines the hunks before it added. */
    function beforeNumberOf(pos) {
      var n = docB.lineAt(pos).number, delta = 0;
      hunks.forEach(function (h) { if (h.toB <= pos) delta += h.after.count - h.before.count; });
      return n - delta;
    }
    /* The deleted lines a deletion block of the inline view stands for:
       the block widget at a hunk's start whose before side has lines. */
    function deletedOf(block, widget) {
      if (widget instanceof HunkHeader || block.from !== block.to) return null;
      var h = hunks.find(function (x) { return x.fromB === block.from && x.before.count; });
      return h || null;
    }
    function rowsOfDeleted(h, make) {
      var out = [];
      for (var n = h.before.line; n < h.before.line + h.before.count; n++) out.push(make(n));
      return out;
    }
    function tickRow(side, n) { var id = side + ':' + n; return { tick: id, on: ticked.has(id) }; }

    /* The gutters of one view: the line numbers (both sides' in the inline
       view), the + and - column, and the tick boxes for the lines when
       there are line actions. side is the text the view shows. */
    function gutters(side, inline) {
      var out = [], ticks = actionsOf('line').length > 0;
      function gutter(cls, lineRow, deletedRow, spacer) {
        return CM.gutter({
          class: 'diff-gutter ' + cls,
          lineMarker: function (view, line) { var r = lineRow(line); return r ? new Rows([r.row], r.cls) : null; },
          widgetMarker: function (view, widget, block) {
            var h = inline ? deletedOf(block, widget) : null;
            return h ? new Rows(rowsOfDeleted(h, deletedRow), 'diff-del') : null;
          },
          lineMarkerChange: redrawn,
          initialSpacer: function () { return new Rows([{ text: spacer }]); }
        });
      }
      var widest = String(Math.max(docA.lines, docB.lines));
      if (inline) {
        out.push(gutter('diff-gutter-number', function (line) {
          var h = hunkAt('after', line.from);
          return { row: { text: h ? '' : String(beforeNumberOf(line.from)) }, cls: h ? 'diff-add' : '' };
        }, function (n) { return { text: String(n) }; }, widest));
      }
      out.push(gutter('diff-gutter-number', function (line) {
        var h = hunkAt(side, line.from);
        return { row: { text: String(docOf(side).lineAt(line.from).number) }, cls: h ? (side === 'before' ? 'diff-del' : 'diff-add') : '' };
      }, function () { return { text: '' }; }, widest));
      out.push(gutter('diff-gutter-mark', function (line) {
        var h = hunkAt(side, line.from);
        return h ? { row: { text: side === 'before' ? '-' : '+' }, cls: side === 'before' ? 'diff-del' : 'diff-add' } : { row: { text: '' } };
      }, function () { return { text: '-' }; }, '+'));
      if (ticks) {
        out.push(gutter('diff-gutter-tick', function (line) {
          var h = hunkAt(side, line.from);
          return h ? { row: tickRow(side, docOf(side).lineAt(line.from).number), cls: side === 'before' ? 'diff-del' : 'diff-add' } : null;
        }, function (n) { return tickRow('before', n); }, ''));
      }
      return out;
    }
    function docOf(side) { return side === 'before' ? docA : docB; }

    /* The hunk headers of one view, as block widgets above each hunk's
       first line; in the inline view they go above the deleted lines too
       (side -2 comes before the merge view's own block at -1). */
    function headers(side, withActions) {
      var field = CM.StateField.define({
        create: function () {
          var ranges = hunks.map(function (h) {
            var at = side === 'before' ? h.fromA : h.fromB;
            return CM.Decoration.widget({ widget: new HunkHeader(h, withActions), block: true, side: -2 }).range(at);
          });
          return CM.Decoration.set(ranges, true);
        },
        update: function (deco, tr) { return deco.map(tr.changes); },
        provide: function (f) { return CM.EditorView.decorations.from(f); }
      });
      return field;
    }

    function common(side, inline, withActions) {
      return [
        CM.EditorView.editable.of(false), CM.EditorState.readOnly.of(true),
        CM.highlightSpecialChars(),
        makeTheme(CM),
        languageFor(CM, spec.language),
        // A text that is not edited is still a tab stop, so the keys scroll
        // it and Next Change can put the cursor in it.
        CM.EditorView.contentAttributes.of({ 'aria-label': (side === 'before' ? 'Before: ' : 'After: ') + spec.title, tabindex: '0' }),
        gutters(side, inline),
        headers(side, withActions)
      ];
    }

    function clear() {
      if (merge) { merge.destroy(); merge = null; }
      views.forEach(function (v) { v.destroy(); });
      views = [];
      $('body').replaceChildren();
    }
    function emptyState(title, body) {
      var e = document.createElement('div');
      e.className = 'empty-state diff-empty';
      e.innerHTML = '<p class="empty-state-title"></p><p class="empty-state-body"></p>';
      e.querySelector('.empty-state-title').textContent = title;
      e.querySelector('.empty-state-body').textContent = body;
      return e;
    }
    function draw() {
      if (!CM || disposed) return;
      clear();
      var body = $('body');
      body.dataset.mode = mode;
      if (!spec) {
        hunks = [];
        body.appendChild(emptyState('Nothing to compare', 'This comparison was closed when ' + (window.dinahProduct ? window.dinahProduct.name : '') + ' stopped, and its texts were not kept. Open it again from where it came: the Git pane, a review or a chat.'));
        drawFoot();
        commandsChanged();
        return;
      }
      classes();
      docA = CM.Text.of(spec.before.split('\n'));
      docB = CM.Text.of(spec.after.split('\n'));
      hunks = hunksOf(CM.Chunk.build(docA, docB, DIFF), docA, docB);
      if (!hunks.length) body.appendChild(emptyState('No differences', 'The two texts are the same.'));
      else if (mode === 'split') {
        merge = new CM.MergeView({
          a: { doc: docA, extensions: common('before', false, false) },
          b: { doc: docB, extensions: common('after', false, true) },
          parent: body, gutter: false, highlightChanges: true, collapseUnchanged: COLLAPSE, diffConfig: DIFF
        });
        views = [merge.a, merge.b];
      } else {
        views = [new CM.EditorView({
          parent: body,
          state: CM.EditorState.create({ doc: docB, extensions: [
            CM.unifiedMergeView({ original: docA, mergeControls: false, gutter: false, highlightChanges: true, syntaxHighlightDeletions: true, collapseUnchanged: COLLAPSE, diffConfig: DIFF }),
            common('after', true, true)
          ] })
        })];
      }
      scroller().addEventListener('scroll', scrolled);
      if (wanted != null) { scroller().scrollTop = wanted; wanted = null; }
      drawFoot();
      drawMeta();
      commandsChanged();
    }
    /* What scrolls: the editor itself inline, the body with both editors
       side by side (the merge view keeps them the same height). */
    function scroller() { return mode === 'split' || !views.length ? $('body') : views[0].scrollDOM; }

    /* ---- Head, meta and foot ------------------------------------------- */

    function drawHead() {
      if (disposed) return;
      if (!opts.embedded && spec) window.pudlWindows.retitle(key, spec.title, spec.detail);
      // The file's actions are controls of the toolbar, filled from the
      // command table by PUDL.
      var holder = $('file-actions'), list = actionsOf('file'), primary = primaryOf(list);
      holder.replaceChildren();
      // A group with no command in it is left alone by PUDL, so it is hidden here.
      holder.closest('.toolbar-group').hidden = !list.length;
      list.forEach(function (a) {
        var b = document.createElement('button');
        b.type = 'button';
        b.className = 'btn btn-sm' + (a === primary ? ' btn-primary' : '');
        b.setAttribute('data-command', 'action-' + a.id);
        holder.appendChild(b);
      });
      drawMeta();
      commandsChanged();
    }
    function counts() {
      var added = 0, removed = 0;
      hunks.forEach(function (h) { added += h.after.count; removed += h.before.count; });
      return { added: added, removed: removed };
    }
    function summary() {
      if (!spec) return '';
      var c = counts(), n = hunks.length;
      return (n === 1 ? '1 hunk' : n + ' hunks') + ' · +' + c.added + ' −' + c.removed;
    }
    function drawMeta() {
      var parts = [];
      if (spec && spec.path) parts.push(spec.path);
      if (spec && spec.detail) parts.push(spec.detail);
      if (spec) parts.push(summary());
      $('meta').textContent = parts.join(' · ');
    }
    function drawFoot() {
      var list = actionsOf('line'), holder = $('line-actions'), primary = primaryOf(list), n = ticked.size;
      $('summary').textContent = summary();
      $('foot').hidden = !spec;
      holder.replaceChildren();
      $('hint').textContent = list.length ? (n ? '' : 'Tick lines to act on part of a hunk.') : '';
      list.forEach(function (a) {
        var b = document.createElement('button');
        b.type = 'button';
        b.className = 'btn btn-sm' + (a === primary ? ' btn-primary' : '') + (a.danger ? ' btn-danger' : '');
        b.dataset.lineAction = a.id;
        b.disabled = !n;
        b.textContent = a.label + ' ' + (n ? n + (n === 1 ? ' line' : ' lines') : 'lines');
        b.addEventListener('click', function () { runLines(a); });
        holder.appendChild(b);
      });
    }

    /* ---- The file ------------------------------------------------------- */

    /* The file the diff is of, as a workspace resource, for Edit the File:
       the reference the opener gave, or the path looked for in the
       workspace's main folder. */
    function resolveFile() {
      fileRef = null;
      if (!spec) { commandsChanged(); return; }
      if (spec.file && spec.file.workspaceId && spec.file.sourceId) { fileRef = spec.file; commandsChanged(); return; }
      if (!spec.path || !window.dinahWorkspaceBrowser || !api() || !api().ResolveWorkspaceResourceMount) { commandsChanged(); return; }
      var wsId = spec.workspaceId || (host() ? host().workspaceId() : '');
      var record = window.dinahWorkspaceBrowser.records().find(function (r) { return r.id === wsId; });
      if (!record || !record.primarySourceId) { commandsChanged(); return; }
      var ref = { workspaceId: record.id, sourceId: record.primarySourceId, path: spec.path.replace(/\\/g, '/').replace(/^\.?\//, '') }, asked = spec;
      Promise.resolve().then(function () { return api().ResolveWorkspaceResourceMount(ref); }).then(function (found) {
        if (disposed || spec !== asked || !found) return;
        fileRef = ref;
        commandsChanged();
      }).catch(function () { /* Not a file of the workspace: the button stays away. */ });
    }
    function editFile() {
      if (!fileRef || !window.dinahEditor) return;
      window.dinahEditor.openResource(fileRef).catch(function (err) { if (host()) host().toast(message(err), 'danger'); });
    }

    /* ---- Mode and state ------------------------------------------------- */

    function remembered() {
      try { var m = window.dinahUIState && window.dinahUIState.get(MODE_KEY); return MODES[m] ? m : ''; } catch (err) { return ''; }
    }
    function parseState(s) {
      var m = /^(inline|split)?(?:@(\d+))?$/.exec(String(s || ''));
      return m ? { mode: m[1] || '', scroll: m[2] ? +m[2] : null } : { mode: '', scroll: null };
    }
    function stateString() {
      var top = views.length ? Math.round(scroller().scrollTop) : (wanted || 0);
      return mode + (top ? '@' + top : '');
    }
    function changed() {
      if (typeof opts.changed === 'function') opts.changed(stateString());
      else root.dispatchEvent(new CustomEvent('pudl:applet-change', { bubbles: true, detail: { state: stateString() } }));
    }
    function scrolled() {
      clearTimeout(scrollTimer);
      scrollTimer = setTimeout(function () { if (!disposed) changed(); }, 300);
    }
    function setMode(next, quiet) {
      if (!MODES[next] || next === mode) return;
      var top = views.length ? scroller().scrollTop : wanted;
      mode = next;
      wanted = top;
      if (!quiet) {
        try { if (window.dinahUIState) window.dinahUIState.set(MODE_KEY, mode); } catch (err) { /* A view preference is optional. */ }
      }
      draw();
      if (!quiet) changed();
    }
    var initial = parseState(opts.state);
    mode = initial.mode || (spec && spec.mode) || remembered() || 'inline';
    wanted = initial.scroll;

    function go(direction) {
      var v = views[views.length - 1];
      if (!v) return;
      (direction > 0 ? CM.goToNextChunk : CM.goToPreviousChunk)(v);
      v.focus();
    }

    /* ---- The command table --------------------------------------------- */

    function commandsChanged() { if (window.pudlCommands) window.pudlCommands.changed(); }
    function commandTable() {
      var all = Object.keys(MODES).map(function (m) {
        return { id: 'diff-' + m, label: MODES[m], choice: 'diff-mode', choiceLabel: 'Layout', checked: function () { return mode === m; }, disabled: function () { return !spec; }, run: function () { setMode(m); } };
      });
      all.push(
        { id: 'previous-change', label: 'Previous Change', glyph: 'arrow-up', shortcut: 'Shift+F7', disabled: function () { return !hunks.length; }, reason: 'The texts have no differences.', run: function () { go(-1); } },
        { id: 'next-change', label: 'Next Change', glyph: 'arrow-down', shortcut: 'F7', disabled: function () { return !hunks.length; }, reason: 'The texts have no differences.', run: function () { go(1); } }
      );
      actionsOf('file').forEach(function (a) {
        all.push({ id: 'action-' + a.id, label: a.label, glyph: a.glyph || null, danger: !!a.danger, run: function () { return run(a); } });
      });
      all.push({ id: 'edit-file', label: 'Edit the File', glyph: 'pencil', hidden: function () { return !fileRef || !window.dinahEditor; }, run: editFile });
      return all;
    }
    var unregister = window.pudlCommands ? window.pudlCommands.register(root, commandTable) : function () {};
    off.push(unregister);

    /* ---- Showing a comparison ------------------------------------------ */

    /* A comparison opened again into this window: the new texts, the
       layout the person has, no ticks. */
    function show(next) {
      spec = next;
      ticked.clear();
      drawHead();
      resolveFile();
      draw();
    }
    function appearanceChanged() { if (!disposed) views.forEach(function (v) { v.requestMeasure(); }); }
    document.addEventListener('dinah:appearance-change', appearanceChanged);
    off.push(function () { document.removeEventListener('dinah:appearance-change', appearanceChanged); });

    var ready = loadCodeMirror().then(function (cm) {
      if (disposed) return;
      CM = cm;
      drawHead();
      resolveFile();
      draw();
    }).catch(function (err) {
      $('meta').textContent = message(err);
    });

    /* The comparison last asked of this window, which may still be on its
       way while the bundle loads. */
    var asked = spec;
    windows.set(key, {
      show: function (next) { asked = next; ready.then(function () { if (!disposed && spec !== next) show(next); }); },
      spec: function () { return asked; }
    });

    return {
      ready: ready,
      state: stateString,
      setState: function (s) {
        var p = parseState(s);
        if (p.mode && p.mode !== mode) { wanted = p.scroll; setMode(p.mode, true); }
        else if (p.scroll != null) { if (views.length) scroller().scrollTop = p.scroll; else wanted = p.scroll; }
      },
      commands: function () {
        return commandTable().filter(function (c) { return !c.choice && !(typeof c.hidden === 'function' && c.hidden()); })
          .map(function (c) { return { label: c.label, glyph: c.glyph, shortcut: c.shortcut, disabled: typeof c.disabled === 'function' ? c.disabled() : !!c.disabled, run: c.run }; })
          .concat(Object.keys(MODES).map(function (m) { return { label: MODES[m], checked: mode === m, radio: 'diff-mode', run: function () { setMode(m); } }; }));
      },
      destroy: function () {
        disposed = true;
        clearTimeout(scrollTimer);
        windows.delete(key);
        off.forEach(function (fn) { fn(); });
        clear();
      }
    };
  }

  /* ---- Opening ----------------------------------------------------------- */

  function open(raw) {
    if (!raw || typeof raw.key !== 'string' || !raw.key) return Promise.reject(new Error('A diff needs a key that says what it compares.'));
    var spec = normalize(raw);
    specs.set(spec.key, spec);
    var item = { kind: 'diff', workspace: spec.workspaceId || '', path: spec.key, title: spec.title };
    var key = window.dinahWorkbench.key(item), shown = windows.get(key);
    if (shown) shown.show(spec);
    return window.dinahWorkbench.open(item).then(function (opened) {
      var inst = windows.get(opened);
      if (inst && inst.spec() !== spec) inst.show(spec);
      return opened;
    });
  }

  window.dinahDiff = { open: open, languageFor: languageFor };
  // PUDL's compare glyph (0.92.0) is for setting two versions side by side.
  window.dinahWorkbench.register('diff', { applet: { id: 'dinah.documents', title: 'Documents', glyph: 'document', workspace: true }, title: 'Diff', glyph: 'compare', init: init });
})();
