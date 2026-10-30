/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
   Licensed under the Apache License, Version 2.0; see
   https://www.apache.org/licenses/LICENSE-2.0 */

/* The terminal applet: the public site in terminal mode
   (Architecture/terminal-design.md). Its filesystem (js/sitefs.js) holds
   the site's structure as sitenav describes it, in /site, served read-only
   by /api/site/tree; the built-in commands, listed in /bin; and the
   reader's own home directory, /home/guest or ~, kept in this browser's
   storage, which on the public site is all it can write,
   with a small full-screen editor for its files. Commands are plain
   objects with an async run(args, io), and io is the only thing a command
   can touch. xterm.js 6.0.0 (MIT) draws the screen, loaded beside this
   script from js/vendor. */
(function () {
    'use strict';

    var SELF_SRC = document.currentScript && document.currentScript.src;
    function beside(file) { return SELF_SRC ? SELF_SRC.replace(/terminal\.js/, file) : '/js/' + file; }
    function loadScript(src) {
        return new Promise(function (resolve, reject) {
            var s = document.createElement('script');
            s.src = src;
            s.onload = resolve;
            s.onerror = function () { reject(new Error('could not load ' + src)); };
            document.head.appendChild(s);
        });
    }
    var xtermReady = null;
    function loadXterm() {
        if (window.Terminal && window.FitAddon && window.SerializeAddon) { return Promise.resolve(); }
        if (!xtermReady) {
            if (!document.querySelector('link[data-xterm-css]')) {
                var css = document.createElement('link');
                css.rel = 'stylesheet';
                css.href = beside('vendor/xterm-6.0.0/xterm.css');
                css.setAttribute('data-xterm-css', '');
                document.head.appendChild(css);
            }
            xtermReady = loadScript(beside('vendor/xterm-6.0.0/xterm.js'))
                .then(function () { return loadScript(beside('vendor/xterm-6.0.0/addon-fit.js')); })
                /* xterm's own add-on for writing out what the screen shows,
                   from the same release (0.14.0), which a reload brings
                   back (js/snapshot.js). */
                .then(function () { return loadScript(beside('vendor/xterm-6.0.0/addon-serialize.js')); })
                .catch(function (err) { xtermReady = null; throw err; });
        }
        return xtermReady;
    }
    var HISTORY_KEY = 'pc-terminal-history';
    var HISTORY_MAX = 200;

    /* The command history, shared by every terminal on the page. On the
       edit origin it is the file ~/.history in the admin's home on the
       server, so it follows the admin, as a shell's does; on the public
       site it stays in this browser's storage, where it has always been.
       Another tab's lines reach this one when it next adds a line (the
       browser's copy) or when the page loads (the file). */
    var History = (function () {
        var lines = [], file = false, ready = null, timer = 0;
        function fromStorage() {
            try { var h = JSON.parse(localStorage.getItem(HISTORY_KEY) || '[]'); return Array.isArray(h) ? h : []; } catch (err) { return []; }
        }
        function save() {
            lines = lines.slice(-HISTORY_MAX);
            if (!file) { try { localStorage.setItem(HISTORY_KEY, JSON.stringify(lines)); } catch (err) { } return; }
            clearTimeout(timer);
            timer = setTimeout(function () { F.write(F.home(), '~/.history', lines.join('\n') + (lines.length ? '\n' : ''), false); }, 800);
        }
        return {
            load: function () {
                if (!ready) {
                    file = !!F.mounted;
                    ready = !file ? Promise.resolve(lines = fromStorage()) : (function () {
                        var n = F.resolve(F.home(), '~/.history');
                        return (n ? F.read(n) : Promise.resolve('')).then(function (text) {
                            lines = String(text || '').split('\n').filter(Boolean).slice(-HISTORY_MAX);
                        }, function () { lines = []; });
                    })();
                }
                return ready;
            },
            list: function () { return lines; },
            add: function (text) {
                if (!file) { lines = fromStorage(); }
                if (lines[lines.length - 1] !== text) { lines.push(text); save(); }
            },
            clear: function () { lines = []; save(); }
        };
    })();

    /* ANSI styling. Output that goes into a pipe is stripped of it. */
    var C = {
        reset: '\x1b[0m', bold: '\x1b[1m', dim: '\x1b[2m', inverse: '\x1b[7m',
        red: '\x1b[31m', green: '\x1b[32m', yellow: '\x1b[33m', blue: '\x1b[34m', cyan: '\x1b[36m'
    };
    function strip(s) { return String(s).replace(/\x1b\[[0-9;?]*[A-Za-z]/g, ''); }
    function paint(color, s) { return C[color] + s + C.reset; }

    /* Conway boards named by the article's own pattern links. */
    var LIFE_PRESETS = {
        glider: 'boardSize=10&cellSize=20&speed=16.666666666666668&init=3,1;1,2;3,2;2,3;3,3;8,6;6,7;8,7;7,8;8,8;',
        gun: 'boardSize=200&cellSize=5&wrap=false&init=1,5;2,5;2,6;1,6;11,5;11,6;11,7;12,8;13,9;14,9;12,4;13,3;14,3;16,4;17,5;17,6;17,7;16,8;15,6;18,6;21,5;22,5;22,4;21,4;21,3;22,3;23,2;23,6;25,2;25,1;25,6;25,7;35,3;35,4;36,4;36,3',
        face: 'boardSize=49&cellSize=5&speed=16.666666666666668&init=28,20;28,12;28,13;29,13;28,14;24,16;25,16;26,16;30,16;31,16;32,16;28,18;28,19;',
        long: 'boardSize=176&cellSize=4&init=45,51;47,51;47,50;49,49;49,48;49,47;51,48;51,47;51,46;52,47;'
    };
    var BARCODE_SYMS = ['ean13', 'ean8', 'upca', 'upce', 'gs1-128', 'itf14', 'itf', 'code128', 'code39', 'codabar', 'qr'];

    /* === The filesystem ===================================================
       The site tree and the home directory live in js/sitefs.js, shared
       with the file manager and the editor. These wrappers keep the
       terminal's own names for its operations. */

    var F = null;
    var siteFsReady = null;
    function loadSiteFs() {
        if (window.pcSiteFs) { F = window.pcSiteFs; return Promise.resolve(F); }
        if (!siteFsReady) {
            siteFsReady = loadScript(window.pcSiteFsSrc || beside('sitefs.js'))
                .then(function () { F = window.pcSiteFs; return F; })
                .catch(function (err) { siteFsReady = null; throw err; });
        }
        return siteFsReady;
    }

    /* The settings files (js/config.js), from the address js/applets.js names. */
    var configReady = null;
    function loadConfig() {
        if (window.pcConfig) { return Promise.resolve(window.pcConfig); }
        if (!configReady) {
            configReady = loadScript(window.pcConfigSrc || beside('config.js'))
                .then(function () { return window.pcConfig; })
                .catch(function (err) { configReady = null; throw err; });
        }
        return configReady;
    }
    var SETTINGS = { fontSize: 14 };
    var FONT_SIZES = [11, 12, 13, 14, 15, 16, 18, 20, 22];
    var instanceCount = 0;

    function pathOf(n) { return F.pathOf(n); }
    function displayPath(n) { return F.displayPath(n); }
    function resolve(root, cwd, path) { return F.resolve(cwd, path); }
    function childNamed(d, name) { return F.childNamed(d, name); }
    function splitPath(path) { return F.splitPath(path); }
    function realOf(n) { return F.realOf(n); }
    function validName(n) { return F.validName(n); }
    function attached(n) { return F.attached(n); }
    function readHome(n) { return F.readHome(n); }
    function homeSave() { return F.save(); }
    function homeSync() { return F.sync(); }
    function homeDir() { return F.home(); }
    function writeFile(root, cwd, path, content, append) { return F.write(cwd, path, content, append); }
    function prefetchTexts(slugs) { return F.prefetch(slugs); }
    function textOf(r) { return F.read(r); }
    function userBin() { return F.userBin(); }
    function isScript(n) { return F.isScript(n); }
    function scriptSource(n) { return F.scriptSource(n); }
    function scriptSummary(name, src) { return F.scriptSummary(name, src); }

    /* === Files the applets understand ===================================== */

    /* A Conway pattern in the plain-text .cells format: O (or *, X) is a
       live cell, anything else dead, and lines starting with ! are notes.
       The pattern is centred on a board with room around it. */
    function parseCells(text) {
        var cells = [], y = 0, w = 0;
        String(text).replace(/\r/g, '').split('\n').forEach(function (line) {
            if (/^\s*!/.test(line)) { return; }
            y++;
            for (var x = 0; x < line.length; x++) { if (/[Oo*X]/.test(line[x])) { cells.push([x + 1, y]); } }
            w = Math.max(w, line.replace(/\s+$/, '').length);
        });
        while (y > 0 && !cells.some(function (c) { return c[1] === y; })) { y--; }
        if (!cells.length) { return { error: 'no live cells in it (mark them with O)' }; }
        var size = Math.max(20, Math.max(w, y) + 24);
        if (size > 400) { return { error: 'the pattern is too large (at most 376 cells across)' }; }
        var ox = Math.floor((size - w) / 2), oy = Math.floor((size - y) / 2);
        var cellSize = Math.max(2, Math.min(20, Math.floor(400 / size)));
        return {
            count: cells.length,
            state: 'boardSize=' + size + '&cellSize=' + cellSize + '&init=' + cells.map(function (c) { return (c[0] + ox) + ',' + (c[1] + oy); }).join(';')
        };
    }

    /* A Sudoku puzzle: 81 cells read left to right, top to bottom, where a
       digit is a clue and a dot, 0 or _ is empty. Lines starting with #
       are notes, and anything else (spaces, bars, dashes) is ignored. */
    function parseSudoku(text) {
        var cells = [];
        String(text).replace(/\r/g, '').split('\n').forEach(function (line) {
            if (/^\s*#/.test(line)) { return; }
            line.replace(/[1-9._0]/g, function (ch) { cells.push(/[1-9]/.test(ch) ? ch : ''); return ch; });
        });
        if (cells.length !== 81) { return { error: 'a puzzle has 81 cells, and this has ' + cells.length }; }
        var rows = [];
        for (var r = 0; r < 9; r++) {
            rows.push(cells.slice(r * 9, r * 9 + 9).map(function (v) { return v ? v + 'C' : '0P'; }).join('.'));
        }
        var board = rows.join('-');
        return { clues: cells.filter(Boolean).length, state: 'medium|' + board, query: 'difficulty=medium&board=' + board };
    }

    /* === Words ============================================================ */

    /* Splits a line into pipeline stages of words, honouring single and
       double quotes and backslash escapes. An unquoted > or >> becomes a
       redirection token. A word with an unquoted *, ? or [ is a pattern,
       { glob, word }: glob is the word with its quoted and escaped
       characters escaped, so only the unquoted ones are wild, and word is
       what it says, for when nothing matches (expandGlobs). */
    function parse(line) {
        var stages = [[]], word = '', pat = '', globbed = false, inWord = false, quote = null;
        function lit(ch) { word += ch; pat += /[*?[\]\\]/.test(ch) ? '\\' + ch : ch; }
        function flush() {
            if (inWord) { stages[stages.length - 1].push(globbed ? { glob: pat, word: word } : word); word = ''; pat = ''; globbed = false; inWord = false; }
        }
        for (var i = 0; i < line.length; i++) {
            var ch = line[i];
            if (quote) {
                if (ch === quote) { quote = null; } else if (ch === '\\' && quote === '"' && i + 1 < line.length) { lit(line[++i]); } else { lit(ch); }
                continue;
            }
            if (ch === '"' || ch === "'") { quote = ch; inWord = true; continue; }
            if (ch === '\\' && i + 1 < line.length) { lit(line[++i]); inWord = true; continue; }
            if (ch === '*' || ch === '?' || ch === '[') { word += ch; pat += ch; globbed = true; inWord = true; continue; }
            if (ch === '|') { flush(); stages.push([]); continue; }
            if (ch === '>') {
                flush();
                if (line[i + 1] === '>') { i++; stages[stages.length - 1].push({ redirect: '>>' }); }
                else { stages[stages.length - 1].push({ redirect: '>' }); }
                continue;
            }
            if (/\s/.test(ch)) { flush(); continue; }
            word += ch; pat += ch; inWord = true;
        }
        if (quote) { return { error: 'unterminated quote' }; }
        flush();
        return { stages: stages };
    }

    function globToRegex(glob) {
        return new RegExp('^' + glob.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.') + '$', 'i');
    }

    /* === Patterns ==========================================================
       Pathname expansion, as a shell does it before a command runs: * is
       any run of characters, ? any one, and [abc], [a-z] or [!abc] one from
       (or not from) a set, within one name, never across a /. A name
       starting with . matches only a pattern that starts with one. The
       matches come sorted; a pattern that matches nothing stays as typed,
       so the command can say it found no such file. */

    /* One name's pattern (a part between slashes, with \ escaping a
       character) as a regular expression, or null when nothing in it is
       wild. */
    function segmentRegex(seg) {
        var re = '', wild = false;
        for (var i = 0; i < seg.length; i++) {
            var ch = seg[i];
            if (ch === '\\' && i + 1 < seg.length) { re += seg[++i].replace(/[.*+?^${}()|[\]\\/]/g, '\\$&'); continue; }
            if (ch === '*') { re += '[^/]*'; wild = true; continue; }
            if (ch === '?') { re += '[^/]'; wild = true; continue; }
            if (ch === '[') {
                var end = seg.indexOf(']', i + 2);
                if (end > i) {
                    var body = seg.slice(i + 1, end), neg = body[0] === '!' || body[0] === '^';
                    if (neg) { body = body.slice(1); }
                    re += '[' + (neg ? '^' : '') + body.replace(/\\(.)/g, '$1').replace(/[\\\]^]/g, '\\$&') + ']';
                    wild = true; i = end; continue;
                }
            }
            re += ch.replace(/[.*+?^${}()|[\]\\/]/g, '\\$&');
        }
        return wild ? new RegExp('^' + re + '$') : null;
    }

    function unescapeGlob(s) { return s.replace(/\\(.)/g, '$1'); }

    /* The words of one stage with each pattern replaced by what it
       matches, found from cwd. */
    function expandGlobs(words, cwd) {
        var out = [];
        words.forEach(function (w) {
            if (!w || typeof w !== 'object' || w.glob == null) { out.push(w); return; }
            var segs = w.glob.split('/'), absolute = segs[0] === '';
            var found = [];
            (function walk(prefix, i) {
                if (i === segs.length) { found.push(prefix); return; }
                var seg = segs[i];
                if (seg === '' && i < segs.length - 1) { walk(prefix, i + 1); return; }
                var re = segmentRegex(seg);
                var join = function (name) { return prefix === '' ? name : prefix === '/' ? '/' + name : prefix + '/' + name; };
                if (!re) {
                    var name = unescapeGlob(seg), next = join(name);
                    if (i === segs.length - 1 || resolve(null, cwd, next)) { walk(next, i + 1); }
                    return;
                }
                var dir = resolve(null, cwd, prefix === '' ? '.' : prefix);
                if (!dir || !dir.children) { return; }
                var dotOk = seg[0] === '.';
                dir.children.map(function (c) { return c.name; })
                    .filter(function (n) { return (dotOk || n[0] !== '.') && re.test(n); })
                    .sort()
                    .forEach(function (n) { walk(join(n), i + 1); });
            })(absolute ? '/' : '', absolute ? 1 : 0);
            // A match must exist, whatever its last part was.
            found = found.filter(function (p) { return resolve(null, cwd, p); });
            if (found.length) { out.push.apply(out, found); } else { out.push(w.word); }
        });
        return out;
    }

    /* Wraps prose at word boundaries to a width, keeping indented lines
       (tables and examples) as they are. */
    function wrap(text, width) {
        return String(text).split('\n').map(function (line) {
            if (/^\s/.test(line) || strip(line).length <= width) { return line; }
            var out = [], cur = '';
            line.split(' ').forEach(function (w) {
                if (cur && strip(cur + ' ' + w).length > width) { out.push(cur); cur = w; }
                else { cur = cur ? cur + ' ' + w : w; }
            });
            if (cur) { out.push(cur); }
            return out.join('\n');
        }).join('\n');
    }

    function fmtDate(d) {
        if (!d || isNaN(d)) { return '          '; }
        return d.getFullYear() + '-' + String(d.getMonth() + 1).padStart(2, '0') + '-' + String(d.getDate()).padStart(2, '0');
    }

    function fmtSize(n) { return n < 1024 ? n + ' B' : (n / 1024).toFixed(1) + ' KB'; }

    /* === Scripts ===========================================================
       A script is a text file of terminal commands: the site's in
       /site/bin (content/bin on the server), and a reader's own in ~/bin.
       Both directories are on the PATH after /site/applets, the reader's
       first, and the built-in commands, which /bin lists, come before
       them all. Recognising one is
       sitefs.js's job (userBin, isScript, scriptSource, scriptSummary);
       running one is the terminal's. */

    /* The comment block at the top of a script, for man. */
    function scriptHelp(src) {
        var out = [];
        var lines = String(src).replace(/\r/g, '').split('\n');
        for (var i = 0; i < lines.length; i++) {
            var t = lines[i].trim();
            if (t.indexOf('#!') === 0) { continue; }
            if (t[0] !== '#') { if (out.length || t) { break; } continue; }
            out.push(t.replace(/^#\s?/, ''));
        }
        return out.join('\n');
    }

    /* Substitutes $1 to $9, ${1:-default}, $@ (every argument), $# (how
       many) and $0 (the script's name). Each value is escaped for the
       parser, so quotes in it stay literal; unquoted, it splits on spaces
       as in a shell. */
    function expand(line, name, args) {
        function q(s) { return String(s).replace(/(["\\'])/g, '\\$1'); }
        return line.replace(/\$\{([0-9])(?::-([^}]*))?\}|\$([0-9@#])/g, function (m, n1, def, n2) {
            var k = n1 != null ? n1 : n2;
            if (k === '@') { return args.map(q).join(' '); }
            if (k === '#') { return String(args.length); }
            if (k === '0') { return q(name); }
            var v = args[+k - 1];
            if (v == null || v === '') { v = def != null ? def : ''; }
            return q(v);
        });
    }

    /* A node's name, or the path it was named by, coloured and marked by
       its kind. */
    function styledName(n, label) {
        var k = realOf(n).kind, name = label || n.name;
        if (n.children) { return paint('blue', C.bold + name) + '/'; }
        if (k === 'app' || k === 'command' || isScript(n)) { return paint('green', name) + '*'; }
        if (k === 'link') { return paint('cyan', name) + '@'; }
        return name;
    }

    function columns(names, width) {
        var plain = names.map(strip);
        var w = plain.reduce(function (m, s) { return Math.max(m, s.length); }, 0) + 2;
        var per = Math.max(1, Math.floor(width / w));
        var rows = Math.ceil(names.length / per), out = [];
        for (var r = 0; r < rows; r++) {
            var line = '';
            for (var c = 0; c < per; c++) {
                var i = c * rows + r;
                if (i >= names.length) { continue; }
                line += names[i] + ' '.repeat(Math.max(0, w - plain[i].length));
            }
            out.push(line.replace(/\s+$/, ''));
        }
        return out.join('\n');
    }

    /* === Commands ========================================================= */

    var COMMANDS = {};
    function command(name, def) { def.name = name; COMMANDS[name] = def; }

    function needNode(io, path, what) {
        var n = io.fs.resolve(path);
        if (!n) { io.err((what || path) + ': no such file or directory'); }
        return n;
    }

    command('help', {
        summary: 'list the commands',
        help: 'help\n\nLists every command with a one-line summary. "man <command>" says more about one.',
        run: function (args, io) {
            io.out(paint('bold', 'This is parkscomputing.com in terminal mode.') + ' The site\'s pages are in /site, and ~ is your own.\n');
            Object.keys(COMMANDS).sort().forEach(function (k) {
                io.out('  ' + paint('green', k.padEnd(11)) + COMMANDS[k].summary + '\n');
            });
            [['Site scripts, in /site/bin', io.fs.resolve('/site/bin')], ['Your scripts, in ~/bin', userBin()]].forEach(function (sec) {
                var kids = sec[1] && sec[1].children ? sec[1].children.filter(isScript) : [];
                if (!kids.length) { return; }
                io.out('\n' + paint('bold', sec[0]) + '\n');
                kids.forEach(function (n) { io.out('  ' + paint('green', n.name.padEnd(11)) + scriptSummary(n.name, scriptSource(n)) + '\n'); });
            });
            io.out('\n' + wrap('Anything ls marks with * runs by its name, or by its path: the applets, and scripts, which are files of commands. Your home directory, ~, holds files you can edit, and a script you put in ~/bin runs like the site\'s; everything else is read-only. Tab completes, the arrow keys recall earlier commands, "|" pipes one command into another, and "> file" saves a command\'s output in ~. "guide" reads the full guide.', io.cols - 1) + '\n');
        }
    });

    command('man', {
        summary: 'explain a command',
        help: 'man <command>\n\nShows the help for a command.',
        complete: 'command',
        run: function (args, io) {
            if (!args.length) { io.err('man: which command? Try "help".'); return 1; }
            var c = COMMANDS[args[0]];
            if (!c) {
                var s = io.findScript(args[0]);
                if (!s) { io.err('man: no entry for ' + args[0]); return 1; }
                var h = scriptHelp(scriptSource(s));
                io.out(paint('bold', s.name) + ' - a script in ' + displayPath(realOf(s).parent) + '\n\n' + wrap(h || 'It has no comments to explain it. "cat ' + displayPath(realOf(s)) + '" shows what it does.', io.cols - 1) + '\n');
                return;
            }
            io.out(paint('bold', c.name) + ' - ' + c.summary + '\n\n' + wrap(c.help, io.cols - 1) + '\n');
        }
    });

    command('ls', {
        summary: 'list a directory',
        help: 'ls [-l] [-a] [-d] [path...]\n\nLists a directory, or the current one, in alphabetical order. Directories end in /, applets and commands in *, and links to other sites in @. Names starting with a dot, such as ~/.config, are hidden unless you ask for them. A directory named on the line is listed by its contents, so ls b* lists what is in bin; -d lists the directory itself.\n\n  -l   one entry per line, with its date, kind, and title or size\n  -a   show the hidden entries too\n  -d   list directories themselves, not what is in them',
        complete: 'path',
        run: function (args, io) {
            var long = false, all = false, self = false, paths = [];
            args.forEach(function (a) {
                if (/^-[a-z]+$/.test(a)) { if (a.indexOf('l') >= 0) { long = true; } if (a.indexOf('a') >= 0) { all = true; } if (a.indexOf('d') >= 0) { self = true; } }
                else { paths.push(a); }
            });
            /* Alphabetical, ignoring case, as a shell's ls is in most locales. */
            var byName = function (x, y) { return (x.label || x.node.name).localeCompare(y.label || y.node.name, undefined, { sensitivity: 'base', numeric: true }); };
            if (!paths.length) { paths.push('.'); }
            var status = 0;
            /* As a shell's ls does: files named on the line are listed
               together, as they were named, and then each directory, under
               its own heading when there is more than one thing to list. */
            var files = [], dirs = [];
            paths.forEach(function (p) {
                var n = needNode(io, p, 'ls: ' + p);
                if (!n) { status = 1; return; }
                (n.children && !self ? dirs : files).push({ node: n, label: p });
            });
            files.sort(byName);
            dirs.sort(byName);
            function emit(entries) {
                if (long) {
                    entries.forEach(function (e) {
                        var c = e.node, r = realOf(c);
                        var kind = c.children ? 'dir ' : isScript(c) ? 'script' : r.home ? 'file' : r.kind === 'app' ? 'app ' : r.kind === 'link' ? 'link' : r.kind === 'command' ? 'cmd ' : 'page';
                        var extra = r.home ? (c.children ? '' : fmtSize(F.size(r))) : (r.title && r.title !== c.name ? r.title : '');
                        io.out(paint('dim', fmtDate(r.home ? r.mtime : r.date)) + '  ' + kind + '  ' + styledName(c, e.label) + (extra ? '  ' + paint('dim', extra) : '') + '\n');
                    });
                } else if (entries.length && !io.interactive) {
                    /* Into a pipe, one name a line, as ls does in a shell. */
                    entries.forEach(function (e) { io.out((e.label || e.node.name) + '\n'); });
                } else if (entries.length) {
                    io.out(columns(entries.map(function (e) { return styledName(e.node, e.label); }), io.cols) + '\n');
                }
            }
            emit(files);
            dirs.forEach(function (d, i) {
                if (files.length + dirs.length > 1) { io.out((files.length || i ? '\n' : '') + d.label + ':\n'); }
                emit(d.node.children.filter(function (c) { return all || c.name.charAt(0) !== '.'; }).map(function (c) { return { node: c }; }).sort(byName));
            });
            return status;
        }
    });

    command('cd', {
        summary: 'change directory',
        help: 'cd [path]\n\nMoves into a directory. With no path, or ~, it goes to your home directory; / is the top of the site.',
        complete: 'dir',
        run: function (args, io) {
            var n = io.fs.resolve(args[0] || '~');
            if (!n) { io.err('cd: ' + args[0] + ': no such directory'); return 1; }
            if (!n.children) { io.err('cd: ' + args[0] + ': not a directory'); return 1; }
            io.fs.chdir(n);
        }
    });

    command('pwd', {
        summary: 'show the current directory',
        help: 'pwd\n\nPrints the path of the current directory.',
        run: function (args, io) { io.out(pathOf(io.fs.cwd()) + '\n'); }
    });

    command('tree', {
        summary: 'show a directory and everything under it',
        help: 'tree [path]\n\nDraws the directory and its contents as a tree.',
        complete: 'dir',
        run: function (args, io) {
            var n = needNode(io, args[0] || '.', 'tree: ' + (args[0] || '.'));
            if (!n) { return 1; }
            io.out(paint('blue', C.bold + (args[0] || '.')) + '\n');
            var counts = { d: 0, f: 0 };
            (function walk(node, prefix) {
                var kids = node.children || [];
                kids.forEach(function (c, i) {
                    var last = i === kids.length - 1;
                    io.out(prefix + (last ? '└── ' : '├── ') + styledName(c) + '\n');
                    if (c.children) { counts.d++; walk(c, prefix + (last ? '    ' : '│   ')); } else { counts.f++; }
                });
            })(n, '');
            io.out('\n' + counts.d + ' directories, ' + counts.f + ' files\n');
        }
    });

    function readable(io, path, cmd) {
        var n = needNode(io, path, cmd + ': ' + path);
        if (!n) { return null; }
        if (n.children) { io.err(cmd + ': ' + path + ': is a directory'); return null; }
        return realOf(n);
    }

    command('cat', {
        summary: 'print a file',
        help: 'cat <file...>\n\nPrints a file. A page prints as text, with each link\'s address after it in angle brackets; a link to another site prints its address.',
        complete: 'file',
        run: async function (args, io) {
            if (!args.length) { if (io.stdin != null) { io.out(io.stdin); return; } io.err('cat: which file?'); return 1; }
            var status = 0;
            for (var i = 0; i < args.length && !io.interrupted(); i++) {
                var r = readable(io, args[i], 'cat');
                if (!r) { status = 1; continue; }
                try { io.out(await textOf(r)); } catch (err) { io.err('cat: ' + args[i] + ': ' + err.message); status = 1; }
            }
            return status;
        }
    });

    command('less', {
        summary: 'read a file one screen at a time',
        help: 'less <file>\n\nShows a file a screen at a time. Space or Page Down moves forward, b or Page Up back, the arrow keys a line, g and G to the top and bottom, and q quits.',
        complete: 'file',
        run: async function (args, io) {
            var text;
            if (args.length) {
                var r = readable(io, args[0], 'less');
                if (!r) { return 1; }
                try { text = await textOf(r); } catch (err) { io.err('less: ' + err.message); return 1; }
            } else if (io.stdin != null) {
                text = io.stdin;
            } else { io.err('less: which file?'); return 1; }
            if (!io.interactive) { io.out(text); return; }
            return io.pager(text, args[0] || '(stdin)');
        }
    });

    command('open', {
        summary: 'open a page, an applet or a link',
        help: 'open <path>\n\nOpens a page the way this view opens things: as a window in the window view, or as a page in the classic view. A link to another site opens in a new tab.',
        complete: 'file',
        run: function (args, io) {
            if (!args.length) { io.err('open: which page?'); return 1; }
            var n = needNode(io, args[0], 'open: ' + args[0]);
            if (!n) { return 1; }
            if (n.children) { io.err('open: ' + args[0] + ': is a directory (try "cd")'); return 1; }
            if (realOf(n).home) { return io.openEditor(args[0]); }
            io.open(realOf(n));
        }
    });

    command('tags', {
        summary: 'list the tags and how many articles carry each',
        help: 'tags\n\nLists every tag. Each is a directory under /site/tags, so "ls /site/tags/travel" lists the travel articles.',
        run: function (args, io) {
            var t = io.fs.resolve('/site/tags');
            (t ? t.children : []).forEach(function (d) {
                io.out(String(d.children.length).padStart(3) + '  ' + paint('blue', d.name) + (d.title !== d.name ? paint('dim', '  (' + d.title + ')') : '') + '\n');
            });
        }
    });

    command('grep', {
        summary: 'search text for a pattern',
        help: 'grep [-i] [-l] [-r] <pattern> [path...]\n\nPrints the lines matching a pattern (a regular expression). With no path it searches what is piped into it. With -r it searches every file under a directory, or under the current one.\n\n  -i   ignore case\n  -l   print only the names of files that match\n  -r   search directories',
        complete: 'path',
        run: async function (args, io) {
            var flags = '', rest = [];
            args.forEach(function (a) { if (/^-[ilr]+$/.test(a) && !rest.length) { flags += a.slice(1); } else { rest.push(a); } });
            if (!rest.length) { io.err('grep: which pattern?'); return 1; }
            var re;
            try { re = new RegExp(rest[0], flags.indexOf('i') >= 0 ? 'i' : ''); } catch (err) { io.err('grep: bad pattern: ' + err.message); return 2; }
            var paths = rest.slice(1), found = false;
            function scan(text, label) {
                var hits = 0;
                text.split('\n').forEach(function (line) {
                    if (!re.test(line)) { return; }
                    hits++; found = true;
                    if (flags.indexOf('l') < 0) {
                        var shown = line.replace(new RegExp(re.source, re.flags + 'g'), function (m) { return paint('red', C.bold + m); });
                        io.out((label ? paint('cyan', label) + ':' : '') + shown + '\n');
                    }
                });
                if (hits && flags.indexOf('l') >= 0 && label) { io.out(label + '\n'); }
            }
            if (!paths.length && io.stdin != null) { scan(io.stdin, ''); return found ? 0 : 1; }
            if (!paths.length) {
                if (flags.indexOf('r') < 0) { io.err('grep: which file? (or -r to search this directory)'); return 2; }
                paths = ['.'];
            }
            var files = [];
            paths.forEach(function (p) {
                var n = needNode(io, p, 'grep: ' + p);
                if (!n) { return; }
                if (n.children) {
                    if (flags.indexOf('r') < 0) { io.err('grep: ' + p + ': is a directory'); return; }
                    (function walk(d) { d.children.forEach(function (c) { if (c.children) { if (!c.virtual) { walk(c); } } else { files.push(c); } }); })(n);
                } else { files.push(n); }
            });
            var seen = {};
            if (files.length > 1) {
                var pages = files.map(realOf).filter(function (r) { return r.kind !== 'link' && !r.home; }).map(function (r) { return r.name; });
                try { await io.prefetch(pages); } catch (err) { io.err('grep: ' + err.message); return 2; }
            }
            for (var i = 0; i < files.length && !io.interrupted(); i++) {
                var r = realOf(files[i]), key = pathOf(r);
                if (seen[key]) { continue; }
                seen[key] = true;
                try { scan(await textOf(r), files.length > 1 || flags.indexOf('r') >= 0 ? displayPath(r) : ''); } catch (err) { }
            }
            return found ? 0 : 1;
        }
    });

    command('find', {
        summary: 'find files by name',
        help: 'find [path] [-name <pattern>]\n\nLists every path under a directory, or only those whose name matches a pattern such as "*barcode*".',
        complete: 'path',
        run: function (args, io) {
            var start = '.', pattern = null;
            for (var i = 0; i < args.length; i++) {
                if (args[i] === '-name' || args[i] === '-iname') { pattern = args[++i]; } else { start = args[i]; }
            }
            var n = needNode(io, start, 'find: ' + start);
            if (!n) { return 1; }
            var re = pattern ? globToRegex(pattern) : null;
            (function walk(node) {
                if (!re || re.test(node.name)) { io.out(pathOf(node) + (node.children && node.parent ? '/' : '') + '\n'); }
                (node.children || []).forEach(function (c) { if (!(c.virtual && node !== n && !node.virtual)) { walk(c); } });
            })(n);
        }
    });

    command('echo', {
        summary: 'print its arguments',
        help: 'echo [text...]\n\nPrints its arguments, which is mostly useful in front of a pipe or a redirection: echo hello > ~/greeting',
        run: function (args, io) { io.out(args.join(' ') + '\n'); }
    });

    /* The guide is an ordinary page, terminal-guide, wherever the site's
       navigation lists it. */
    function findNamed(node, name) {
        var kids = node.children || [];
        for (var i = 0; i < kids.length; i++) {
            if (kids[i].name === name && !kids[i].children) { return kids[i]; }
            if (kids[i].children && !kids[i].virtual && !kids[i].home) { var hit = findNamed(kids[i], name); if (hit) { return hit; } }
        }
        return null;
    }

    command('guide', {
        summary: 'read the full guide to this terminal',
        help: 'guide [-w]\n\nShows the terminal guide a screen at a time, with the same keys as less. With -w it opens as a page instead: a window in the window view, or the page itself in the classic view.',
        run: async function (args, io) {
            var n = findNamed(io.fs.resolve('/'), 'terminal-guide');
            if (!n) { io.err('guide: the guide is not on this site yet'); return 1; }
            if (args[0] === '-w') { io.open(realOf(n)); return; }
            var text;
            try { text = await io.read(pathOf(n)); } catch (err) { io.err('guide: ' + err.message); return 1; }
            if (!io.interactive) { io.out(text); return; }
            return io.pager(text, 'Terminal Guide');
        }
    });

    command('clear', {
        summary: 'clear the screen',
        help: 'clear\n\nClears the screen. Ctrl+L does the same.',
        run: function (args, io) { io.clear(); }
    });

    command('history', {
        summary: 'list earlier commands',
        help: 'history [-c]\n\nLists the commands typed in this browser. -c clears the list.',
        run: function (args, io) {
            if (args[0] === '-c') { io.history.clear(); return; }
            io.history.list().forEach(function (h, i) { io.out(String(i + 1).padStart(4) + '  ' + h + '\n'); });
        }
    });

    /* --- Your own files ------------------------------------------------- */

    command('edit', {
        summary: 'edit a file in your home directory',
        help: 'edit [-g] <file>\n\nOpens a file in a full-screen editor, creating it if it does not exist yet. Only files in your home directory (~) can be changed.\n\n  Ctrl+S   save (Ctrl+O works too)\n  Ctrl+X   exit, asking to save any changes\n  Ctrl+K   cut the current line\n  Ctrl+U   paste the lines cut last\n  Ctrl+G   show the keys\n\nThe arrow keys, Home, End, Page Up and Page Down move around.\n\nWith -g the file opens in the graphical Editor instead, which has tabs, syntax colouring, and search and replace.',
        complete: 'file',
        run: function (args, io) {
            var graphical = args[0] === '-g';
            if (graphical) { args = args.slice(1); }
            if (!args.length) { io.err('edit: which file? For a new one, give it a name: edit ~/notes'); return 1; }
            if (graphical) { return io.openEditor(args[0]); }
            return io.edit(args[0]);
        }
    });

    command('touch', {
        summary: 'create an empty file',
        help: 'touch <file...>\n\nCreates each file that does not exist yet, empty, and updates the time on each that does. Only in your home directory.',
        complete: 'file',
        run: async function (args, io) {
            if (!args.length) { io.err('touch: which file?'); return 1; }
            var status = 0;
            for (var i = 0; i < args.length; i++) {
                var n = io.fs.resolve(args[i]);
                if (n && realOf(n).home && !n.children) {
                    /* A server file keeps its time: rewriting it would only
                       file an identical copy in its history. */
                    if (!n.special && !realOf(n).server) { n.mtime = new Date(); var e = io.fs.save(); if (e) { io.err('touch: ' + e); status = 1; } }
                    continue;
                }
                var r = await io.fs.write(args[i], '', false);
                if (r.error) { io.err('touch: ' + r.error); status = 1; }
            }
            return status;
        }
    });

    command('mkdir', {
        summary: 'make a directory',
        help: 'mkdir [-p] <directory...>\n\nMakes directories in your home directory.\n\n  -p   make any missing parent directories too, and don\'t complain if it exists',
        complete: 'dir',
        run: async function (args, io) {
            var parents = false, paths = [];
            args.forEach(function (a) { if (a === '-p') { parents = true; } else { paths.push(a); } });
            if (!paths.length) { io.err('mkdir: which directory?'); return 1; }
            var status = 0;
            for (var i = 0; i < paths.length; i++) {
                var e = await io.fs.mkdir(paths[i], parents);
                if (e) { io.err('mkdir: ' + e); status = 1; }
            }
            return status;
        }
    });

    command('rm', {
        summary: 'remove a file',
        help: 'rm [-r] <path...>\n\nRemoves files from your home directory.\n\n  -r   remove a directory and everything in it',
        complete: 'path',
        run: async function (args, io) {
            var recursive = false, paths = [];
            args.forEach(function (a) { if (/^-[rRf]+$/.test(a)) { if (/[rR]/.test(a)) { recursive = true; } } else { paths.push(a); } });
            if (!paths.length) { io.err('rm: which file?'); return 1; }
            var status = 0;
            for (var i = 0; i < paths.length; i++) {
                var n = needNode(io, paths[i], 'rm: ' + paths[i]);
                if (!n) { status = 1; continue; }
                var e = await io.fs.remove(n, recursive, paths[i]);
                if (e) { io.err('rm: ' + e); status = 1; }
            }
            return status;
        }
    });

    command('rmdir', {
        summary: 'remove an empty directory',
        help: 'rmdir <directory...>\n\nRemoves empty directories from your home directory.',
        complete: 'dir',
        run: async function (args, io) {
            if (!args.length) { io.err('rmdir: which directory?'); return 1; }
            var status = 0;
            for (var i = 0; i < args.length; i++) {
                var p = args[i], n = needNode(io, p, 'rmdir: ' + p);
                if (!n) { status = 1; continue; }
                if (!n.children) { io.err('rmdir: ' + p + ': not a directory'); status = 1; continue; }
                if (n.children.length) { io.err('rmdir: ' + p + ': not empty'); status = 1; continue; }
                var e = await io.fs.remove(n, true, p);
                if (e) { io.err('rmdir: ' + e); status = 1; }
            }
            return status;
        }
    });

    command('cp', {
        summary: 'copy a file into your home directory',
        help: 'cp <file> <destination>\n\nCopies a file into your home directory. The file may be one of yours or a page of the site, which is copied as text. The destination may be a directory, such as ~/, or a new name.',
        complete: 'path',
        run: async function (args, io) {
            if (args.length !== 2) { io.err('cp: give a file and a destination, such as: cp /site/articles/coincidences ~/'); return 1; }
            var r = readable(io, args[0], 'cp');
            if (!r) { return 1; }
            var res = await F.copy(io.fs.cwd(), r, args[1]);
            if (res.error) { io.err('cp: ' + res.error); return 1; }
        }
    });

    command('mv', {
        summary: 'move or rename a file',
        help: 'mv <path> <destination>\n\nMoves or renames a file or directory within your home directory. The destination may be a directory, such as ~/patterns/, or a new name.',
        complete: 'path',
        run: async function (args, io) {
            if (args.length !== 2) { io.err('mv: give a path and a destination'); return 1; }
            var n = needNode(io, args[0], 'mv: ' + args[0]);
            if (!n) { return 1; }
            var e = await io.fs.move(n, args[1], args[0]);
            if (e) { io.err('mv: ' + e); return 1; }
        }
    });

    command('download', {
        summary: 'save a file to your computer',
        help: 'download <file>\n\nSaves a file to your computer through the browser\'s usual download. A page of the site downloads as text.',
        complete: 'file',
        run: async function (args, io) {
            if (!args.length) { io.err('download: which file?'); return 1; }
            var r = readable(io, args[0], 'download');
            if (!r) { return 1; }
            var text;
            try { text = await textOf(r); } catch (err) { io.err('download: ' + err.message); return 1; }
            var name = r.home ? r.name : r.name + '.txt';
            io.download(name, text);
            io.out('Downloaded ' + name + ' (' + fmtSize(text.length) + ').\n');
        }
    });

    command('upload', {
        summary: 'bring text files in from your computer',
        help: 'upload [directory]\n\nAsks for text files on your computer and copies them into a directory in your home, or into ~ itself. A file with the same name is replaced. Each file may be up to 256 KB.',
        complete: 'dir',
        run: async function (args, io) {
            var d = io.fs.resolve(args[0] || (io.fs.cwd().home ? '.' : '~'));
            if (!d || !d.children) { io.err('upload: ' + (args[0] || '.') + ': no such directory'); return 1; }
            if (!d.home) { io.err('upload: files can only go into your home directory (~)' + (F.mounted ? ' or under /wwwroot' : '')); return 1; }
            io.out('Choose files in the dialog (Ctrl+C cancels).\n');
            var files = await io.upload();
            if (!files || !files.length) { io.out('Nothing uploaded.\n'); return; }
            var status = 0;
            for (var i = 0; i < files.length; i++) {
                var f = files[i];
                var target = displayPath(d).replace(/\/$/, '') + '/' + f.name;
                var res = await F.upload(d, f);
                if (res.error) { io.err('upload: ' + res.error); status = 1; } else { io.out('Uploaded ' + target + ' (' + fmtSize(f.size) + ').\n'); }
            }
            return status;
        }
    });

    /* --- The applets ---------------------------------------------------- */

    /* Each applet runs by the name it has in /site/applets, where ls marks it
       with *. A few also take arguments or have a shorter alias. */
    function alias(name, target) {
        var t = COMMANDS[target];
        command(name, {
            summary: 'the same as ' + target, help: name + ' is another name for ' + target + '.\n\n' + t.help,
            complete: t.complete, completePaths: t.completePaths, run: t.run
        });
    }

    async function fileArg(io, cmd, path) {
        var r = readable(io, path, cmd);
        if (!r) { return null; }
        try { return await textOf(r); } catch (err) { io.err(cmd + ': ' + path + ': ' + err.message); return null; }
    }

    command('sudoku', {
        summary: 'play Sudoku',
        help: 'sudoku [file]\n\nOpens the Sudoku game, with the puzzle in a file if you name one. A puzzle file holds 81 cells, left to right and top to bottom: a digit is a clue, and a dot is an empty cell. Try: sudoku ~/puzzle.sudoku',
        complete: 'file',
        run: async function (args, io) {
            if (!args.length) { io.open({ name: 'sudoku', kind: 'app', title: 'Sudoku' }); return; }
            var text = await fileArg(io, 'sudoku', args[0]);
            if (text == null) { return 1; }
            var p = parseSudoku(text);
            if (p.error) { io.err('sudoku: ' + args[0] + ': ' + p.error); return 1; }
            io.out('Loaded a puzzle with ' + p.clues + ' clues.\n');
            io.open({ name: 'sudoku', kind: 'app', title: 'Sudoku' }, p.state, p.query);
        }
    });

    command('flashcards', {
        summary: 'drill barcode symbologies',
        help: 'flashcards\n\nOpens the barcode flash cards.',
        run: function (args, io) { io.open({ name: 'flashcards', kind: 'app', title: 'Barcode Flash Cards' }); }
    });

    command('conway', {
        summary: 'run Conway\'s Game of Life',
        help: 'conway [pattern | file]\n\nOpens Conway\'s Game of Life, with one of the article\'s patterns or a pattern of your own. The patterns are:\n\n  glider   a pair of gliders that follow each other\n  gun      a Gosper glider gun\n  face     cells that settle into a funny face\n  long     a large board with a long-running pattern\n\nA file of your own is in the plain-text .cells format: O is a live cell, a dot is a dead one, and lines starting with ! are notes. Try: conway ~/glider.cells',
        complete: function () { return Object.keys(LIFE_PRESETS); },
        completePaths: true,
        run: async function (args, io) {
            var state = null;
            if (args[0] && LIFE_PRESETS[args[0]]) { state = LIFE_PRESETS[args[0]]; }
            else if (args[0]) {
                if (!io.fs.resolve(args[0])) { io.err('conway: ' + args[0] + ' is neither a pattern (' + Object.keys(LIFE_PRESETS).join(', ') + ') nor a file'); return 1; }
                var text = await fileArg(io, 'conway', args[0]);
                if (text == null) { return 1; }
                var p = parseCells(text);
                if (p.error) { io.err('conway: ' + args[0] + ': ' + p.error); return 1; }
                io.out('Loaded ' + p.count + ' live cells.\n');
                state = p.state;
            }
            io.open({ name: 'conway', kind: 'app', title: 'Conway\'s Game of Life' }, state);
        }
    });
    alias('life', 'conway');

    command('barcodes', {
        summary: 'make a barcode in the barcode tool',
        help: 'barcodes [symbology] [data]\n\nOpens the barcode tool, set to a symbology and data if you give them. Its layouts are in ~/barcode-layouts.json, which you can edit.\n\nSymbologies: ' + BARCODE_SYMS.join(', ') + '\n\nExample: barcodes ean13 480036140036',
        complete: function (words) { return words.length <= 2 ? BARCODE_SYMS : []; },
        run: function (args, io) {
            var state = null;
            if (args.length) {
                var sym = args[0].toLowerCase();
                if (BARCODE_SYMS.indexOf(sym) < 0) { io.err('barcodes: unknown symbology ' + args[0] + '. Try: ' + BARCODE_SYMS.join(', ')); return 1; }
                var q = new URLSearchParams();
                q.set('s', sym);
                if (args.length > 1) { q.set('d', args.slice(1).join(' ')); }
                state = q.toString();
            }
            io.open({ name: 'barcodes', kind: 'app', title: 'Barcode Tool' }, state);
        }
    });
    alias('barcode', 'barcodes');

    /* Another terminal, and Files, are asked for through PUDL's requests
       (pudlApplets.request), not by name. */
    function folderArg(io, args, label) {
        var dir = args.length ? io.fs.resolve(args[0]) : io.fs.cwd();
        if (!dir || !dir.children) { io.err(label + ': ' + args[0] + ': no such directory'); return null; }
        return pathOf(dir);
    }

    command('terminal', {
        summary: 'open another terminal',
        help: 'terminal [dir]\n\nOpens another terminal in this directory, or in the one you name. In the window view it is a new window, up to four terminals; with four open, the newest one moves to the directory instead. On the terminal\'s own page it opens in a new tab.',
        complete: 'dir',
        run: function (args, io) {
            var path = folderArg(io, args, 'terminal');
            if (!path) { return 1; }
            if (!io.request('shell', { path: path })) { io.err('terminal: nothing on this site opens another terminal'); return 1; }
        }
    });

    command('files', {
        summary: 'show a directory in Files',
        help: 'files [dir]\n\nShows this directory, or the one you name, in Files, the graphical file manager.',
        complete: 'dir',
        run: function (args, io) {
            var path = folderArg(io, args, 'files');
            if (!path) { return 1; }
            if (!io.request('browse', { path: path })) { io.err('files: nothing on this site browses folders'); return 1; }
        }
    });
    alias('nano', 'edit');

    /* The PATH: after the commands, a word may name an applet or a script,
       by its path, in the current directory, or in /site/applets, ~/bin or
       /site/bin, in that order, so a script of the reader's own comes
       before the site's of the same name. A path into /bin, such as
       /bin/ls, names a built-in command. */
    function runnable(n) { return n && !n.children && (realOf(n).kind === 'app' || realOf(n).kind === 'command' || isScript(n)); }

    function findProgram(fs, cwd, word) {
        if (word.indexOf('/') >= 0) { var p = resolve(fs, cwd, word); return runnable(p) ? p : null; }
        var places = [cwd, resolve(fs, fs, '/site/applets'), userBin(), resolve(fs, fs, '/site/bin')];
        for (var i = 0; i < places.length; i++) {
            var n = places[i] ? childNamed(places[i], word) : null;
            if (runnable(n)) { return n; }
        }
        return null;
    }

    function programAt(fs, cwd, word) {
        var n = findProgram(fs, cwd, word);
        if (!n) { return null; }
        var r = realOf(n);
        if (isScript(n)) { return { name: word, run: function (args, io) { return io.runScript(r, args); } }; }
        return COMMANDS[r.name] || { name: word, run: function (args, io) { io.open(r); } };
    }

    /* JavaScript commands beyond the built-in ones register here, from the
       files the site lists in window.pcTerminalCommands (js/applets.js).
       A command is { name, summary, help, complete, run(args, io) }, the
       same shape as the built-ins, and io is all it can reach. */
    window.pcTerminal = {
        register: function (def) {
            if (!def || !/^[a-z][a-z0-9_-]{0,39}$/.test(def.name || '') || typeof def.run !== 'function') { return false; }
            command(def.name, Object.assign({ summary: '', help: def.name }, def));
            return true;
        },
        paint: paint,
        strip: strip,
        wrap: wrap,
        columns: columns
    };

    var extrasReady = null;
    function loadExtras() {
        if (!extrasReady) {
            var list = Array.isArray(window.pcTerminalCommands) ? window.pcTerminalCommands : [];
            var ver = SELF_SRC && /\?/.test(SELF_SRC) ? SELF_SRC.slice(SELF_SRC.indexOf('?')) : '';
            extrasReady = Promise.all(list.map(function (src) {
                return loadScript(src + ver).catch(function () { });
            }));
        }
        return extrasReady;
    }

    /* === The applet ======================================================= */

    function themeColors(host) {
        var probe = document.createElement('span');
        probe.style.display = 'none';
        host.appendChild(probe);
        function tok(name, fallback) {
            probe.style.color = '';
            probe.style.color = 'var(' + name + ', ' + fallback + ')';
            return getComputedStyle(probe).color || fallback;
        }
        function alpha(rgb, a) {
            var m = /rgba?\(([^)]+)\)/.exec(rgb);
            if (!m) { return rgb; }
            var p = m[1].split(',').map(function (s) { return s.trim(); });
            return 'rgba(' + p[0] + ', ' + p[1] + ', ' + p[2] + ', ' + a + ')';
        }
        var t = {
            background: tok('--input-bg', '#ffffff'),
            foreground: tok('--text', '#111111'),
            cursor: tok('--accent', '#2f6fb3'),
            cursorAccent: tok('--input-bg', '#ffffff'),
            black: tok('--text', '#111111'),
            red: tok('--danger', '#b91c1c'),
            green: tok('--positive', '#1f7a3a'),
            yellow: tok('--warn', '#a16207'),
            blue: tok('--accent', '#2f6fb3'),
            magenta: '#8b5cf6',
            cyan: '#0e7490',
            white: tok('--text-muted', '#6b7280'),
            brightBlack: tok('--text-muted', '#6b7280')
        };
        t.selectionBackground = alpha(t.cursor, 0.3);
        if (document.documentElement.getAttribute('data-theme') === 'dark') { t.cyan = '#22d3ee'; t.magenta = '#c4b5fd'; }
        probe.remove();
        return t;
    }

    function init(root, opts) {
        opts = opts || {};
        root.classList.add('pc-terminal');
        if (opts.fit === 'fill') { root.classList.add('pc-terminal-fill'); }
        /* The terminal's own settings, kept in ~/.config/terminal.json, in a
           dialog its commands open: from the window menu in a window, and
           from PUDL's Commands menu on its own page. */
        var setId = 'pc-term-settings-' + (++instanceCount);
        root.innerHTML = '<div class="pc-terminal-screen" data-role="screen"></div>' +
            '<dialog class="dialog pc-terminal-settings" data-role="settings" aria-labelledby="' + setId + '-title">' +
              '<form method="dialog">' +
                '<h3 class="dialog-title" id="' + setId + '-title">Terminal settings</h3>' +
                '<div class="dialog-body">' +
                  '<div class="form-group"><label class="form-label" for="' + setId + '-font">Text size</label>' +
                    '<span class="form-select-wrap"><select class="form-select" id="' + setId + '-font" data-role="font-size">' +
                      FONT_SIZES.map(function (n) { return '<option value="' + n + '">' + n + ' px</option>'; }).join('') +
                    '</select></span></div>' +
                  '<p class="form-help">These apply to every terminal, and are kept in ~/.config/terminal.json.</p>' +
                '</div>' +
                '<div class="dialog-actions"><button type="submit" class="btn btn-primary">Done</button></div>' +
              '</form>' +
            '</dialog>';
        var screen = root.querySelector('[data-role="screen"]');
        var fontSel = root.querySelector('[data-role="font-size"]');
        var setDialog = root.querySelector('[data-role="settings"]');
        setDialog.addEventListener('close', function () { if (term) { term.focus(); } });

        /* The terminal's commands, for PUDL's window menu, and on its own
           page for the Commands menu PUDL puts above it (0.32.0). */
        function commands() {
            var list = [
                { label: 'Settings…', run: function () { setDialog.showModal(); fontSel.focus(); } },
                { label: 'Clear the screen', run: function () { if (term) { term.clear(); term.focus(); } } }
            ];
            if (window.pudlApplets && window.pudlApplets.can && window.pudlApplets.can('shell')) {
                list.push({ label: 'New terminal window here', run: function () { window.pudlApplets.request('shell', { path: cwd ? pathOf(cwd) : '/' }, root); } });
            }
            return list;
        }
        var settings = Object.assign({}, SETTINGS), unConfig = null;
        function fontSize() { var n = +settings.fontSize; return FONT_SIZES.indexOf(n) >= 0 ? n : SETTINGS.fontSize; }
        function applySettings(s) {
            settings = Object.assign({}, SETTINGS, s);
            fontSel.value = String(fontSize());
            if (term && term.options.fontSize !== fontSize()) {
                term.options.fontSize = fontSize();
                try { fit.fit(); } catch (err) { }
            }
        }
        fontSel.addEventListener('change', function () {
            loadConfig().then(function (c) { return c.set('terminal', { fontSize: +fontSel.value }); });
        });

        var destroyed = false;
        var term = null, fit = null, fs = null, cwd = null, ro = null;
        /* On its own page the address wins over the state kept for it, as
           the site's continuity expects, so a link to a folder opens there. */
        var initialCwd = (opts.ownsUrl ? cwdFrom(location.search) : null) || cwdFrom(opts.state) || '/site';
        /* A command to put at the prompt, never to run: from the page's own
           address, or handed over by Files' "Run in the terminal". */
        var prefill = (opts.ownsUrl ? (new URLSearchParams(location.search).get('run') || '') : '') ||
            (opts.state && new URLSearchParams(String(opts.state)).get('run')) || '';

        function cwdFrom(s) {
            if (!s) { return null; }
            var v = new URLSearchParams(String(s).replace(/^\?/, '')).get('cwd');
            return v && v[0] === '/' ? v : null;
        }
        function stateString() { return 'cwd=' + encodeURIComponent(cwd ? pathOf(cwd) : '/').replace(/%2F/g, '/'); }
        function announce() {
            var s = stateString();
            if (opts.ownsUrl) {
                var q = new URLSearchParams(location.search);
                q.set('cwd', cwd ? pathOf(cwd) : '/');
                q.delete('run');
                history.replaceState(history.state, '', location.pathname + '?' + q.toString().replace(/%2F/g, '/') + location.hash);
            }
            if (opts.changed) { opts.changed(s); }
        }


        /* === Line editing ============================================= */

        var line = '', cursor = 0, histPos = -1, draft = '', busy = false, keyWaiter = null, lastTab = 0;

        function write(s) { if (term) { term.write(String(s).replace(/\r?\n/g, '\r\n')); } }
        function prompt() {
            /* With the server mounted the prompt is the admin's, at the
               host they are on (edit.parkscomputing, beta.parkscomputing),
               and ends in #, as a root shell's does. */
            return F.mounted
                ? paint('red', F.user() + '@' + location.hostname.replace(/\.com$/, '')) + ':' + paint('blue', C.bold + displayPath(cwd)) + '# '
                : paint('green', 'guest@parkscomputing') + ':' + paint('blue', C.bold + displayPath(cwd)) + '$ ';
        }
        function redraw() {
            term.write('\r\x1b[K' + prompt() + line);
            var back = line.length - cursor;
            if (back > 0) { term.write('\x1b[' + back + 'D'); }
        }
        function newPrompt() { line = ''; cursor = 0; histPos = -1; term.write(prompt()); }

        function pathCompletions(partial, dirsOnly) {
            var slash = partial.lastIndexOf('/');
            var dirPart = slash >= 0 ? partial.slice(0, slash + 1) : '';
            var base = slash >= 0 ? partial.slice(slash + 1) : partial;
            if (partial === '~') { return [{ word: '~/', done: false }]; }
            var dir = resolve(fs, cwd, dirPart || '.');
            if (!dir || !dir.children) { return []; }
            return dir.children
                .filter(function (n) { return n.name.indexOf(base) === 0 && (!dirsOnly || n.children); })
                .map(function (n) { return { word: dirPart + n.name + (n.children ? '/' : ''), done: !n.children }; });
        }

        function completions(words, partial) {
            var lastTok = words[words.length - 1];
            if (lastTok && typeof lastTok === 'object') { return pathCompletions(partial, false); }
            words = words.filter(function (w) { return typeof w === 'string'; });
            if (!words.length) {
                var names = Object.keys(COMMANDS);
                [resolve(fs, fs, '/site/bin'), userBin()].forEach(function (d) {
                    (d && d.children ? d.children : []).forEach(function (n) { if (isScript(n) && names.indexOf(n.name) < 0) { names.push(n.name); } });
                });
                return names.filter(function (k) { return k.indexOf(partial) === 0; }).sort().map(function (k) { return { word: k, done: true }; });
            }
            var c = COMMANDS[words[0]];
            var mode = c && c.complete;
            var list = [];
            if (typeof mode === 'function') {
                list = mode(words.concat([partial])).filter(function (k) { return k.indexOf(partial) === 0; }).map(function (k) { return { word: k, done: true }; });
            } else if (mode === 'command') {
                list = Object.keys(COMMANDS).filter(function (k) { return k.indexOf(partial) === 0; }).map(function (k) { return { word: k, done: true }; });
            }
            if (mode === 'path' || mode === 'file' || mode === 'dir' || (c && c.completePaths)) {
                list = list.concat(pathCompletions(partial, mode === 'dir'));
            }
            return list;
        }

        function complete() {
            var before = line.slice(0, cursor);
            var m = /(\S*)$/.exec(before);
            var partial = m[1];
            var head = before.slice(0, before.length - partial.length);
            var parsed = parse(head);
            var words = (parsed.stages ? parsed.stages[parsed.stages.length - 1] : [])
                .map(function (w) { return w && typeof w === 'object' && w.glob != null ? w.word : w; });
            var list = completions(words, partial);
            if (!list.length) { return; }
            if (list.length === 1) {
                var w = list[0].word + (list[0].done ? ' ' : '');
                line = head + w + line.slice(cursor);
                cursor = head.length + w.length;
                redraw();
                return;
            }
            var common = list.reduce(function (acc, x) {
                var i = 0; while (i < acc.length && i < x.word.length && acc[i] === x.word[i]) { i++; }
                return acc.slice(0, i);
            }, list[0].word);
            if (common.length > partial.length) {
                line = head + common + line.slice(cursor);
                cursor = head.length + common.length;
                redraw();
                return;
            }
            var now = Date.now();
            if (now - lastTab < 800) {
                write('\n' + columns(list.map(function (x) { return x.word.replace(/.*\/(?=.)/, ''); }), term.cols) + '\n');
                redraw();
            }
            lastTab = now;
        }

        /* While a command runs, Ctrl+C interrupts it and anything else is
           typed ahead, to be replayed at the next prompt. */
        var typeahead = [], interrupted = false;
        /* A command that has taken the terminal over, such as ssh, gets
           every key, Ctrl+C included, until it gives it back. */
        var raw = null;
        function takeOver(h) {
            raw = h;
            var sub = term.onResize(function (size) { if (h.resize) { h.resize(size.cols, size.rows); } });
            return {
                cols: term.cols, rows: term.rows,
                write: function (d) { if (!destroyed) { term.write(d); } },
                release: function () { if (raw === h) { raw = null; } sub.dispose(); }
            };
        }
        function onData(data) {
            if (raw) { raw.data(data); return; }
            if (keyWaiter) { var k = keyWaiter; keyWaiter = null; k(data); return; }
            if (busy) {
                if (data === '\x03') { interrupted = true; typeahead = []; write('^C'); if (uploadCancel) { uploadCancel(); } }
                else { typeahead.push(data); }
                return;
            }
            /* Pasted text arrives in one piece; only its first line runs. */
            if (data.length > 1 && data[0] !== '\x1b') {
                var text = data.replace(/\r\n?/g, '\n');
                var nl = text.indexOf('\n');
                insert(nl >= 0 ? text.slice(0, nl) : text);
                if (nl >= 0) { submit(); }
                return;
            }
            switch (data) {
                case '\r': submit(); return;
                case '\x7f': case '\b':
                    if (cursor > 0) { line = line.slice(0, cursor - 1) + line.slice(cursor); cursor--; redraw(); }
                    return;
                case '\x1b[3~':
                    if (cursor < line.length) { line = line.slice(0, cursor) + line.slice(cursor + 1); redraw(); }
                    return;
                case '\x1b[D': if (cursor > 0) { cursor--; term.write(data); } return;
                case '\x1b[C': if (cursor < line.length) { cursor++; term.write(data); } return;
                case '\x1b[H': case '\x01': case '\x1bOH': cursor = 0; redraw(); return;
                case '\x1b[F': case '\x05': case '\x1bOF': cursor = line.length; redraw(); return;
                case '\x1b[A': historyStep(-1); return;
                case '\x1b[B': historyStep(1); return;
                case '\t': complete(); return;
                case '\x03': write('^C\n'); newPrompt(); return;
                case '\x0c': term.clear(); redraw(); return;
                case '\x15': line = line.slice(cursor); cursor = 0; redraw(); return;
                case '\x0b': line = line.slice(0, cursor); redraw(); return;
            }
            if (data >= ' ' && data.indexOf('\x1b') < 0) { insert(data); }
        }
        function insert(s) {
            s = s.replace(/[\x00-\x1f]/g, '');
            line = line.slice(0, cursor) + s + line.slice(cursor);
            cursor += s.length;
            redraw();
        }
        function historyStep(d) {
            var hist = History.list();
            if (!hist.length) { return; }
            if (histPos === -1) { if (d > 0) { return; } draft = line; histPos = hist.length; }
            histPos += d;
            if (histPos < 0) { histPos = 0; }
            if (histPos >= hist.length) { histPos = -1; line = draft; } else { line = hist[histPos]; }
            cursor = line.length;
            redraw();
        }

        async function submit() {
            var text = line.trim();
            write('\n');
            if (text) {
                History.add(text);
                busy = true;
                interrupted = false;
                try { await execute(text); } catch (err) { if (!interrupted) { write(paint('red', 'error: ' + err.message) + '\n'); } }
                busy = false;
                if (destroyed) { return; }
                if (interrupted) { write('\n'); }
            }
            newPrompt();
            if (waitingRequest) { var req = waitingRequest; waitingRequest = null; applyRequest(req); }
            var pending = typeahead;
            typeahead = [];
            pending.forEach(onData);
        }

        /* === Running a line ============================================ */

        /* The file operations, relative to the current directory. */
        function mkdir(path, parents) { return F.mkdir(cwd, path, parents); }
        function remove(n, recursive, label) { return F.remove(n, recursive, label); }
        function move(n, dest, label) { return F.move(cwd, n, dest, label); }

        function download(name, text) {
            var url = URL.createObjectURL(new Blob([text], { type: 'text/plain;charset=utf-8' }));
            var a = document.createElement('a');
            a.href = url;
            a.download = name;
            document.body.appendChild(a);
            a.click();
            a.remove();
            setTimeout(function () { URL.revokeObjectURL(url); }, 10000);
        }

        /* Asks for files with a hidden file input. It must start while the
           keypress that ran the command is still being handled, which it is:
           nothing awaits before a command's run() is called. */
        var uploadCancel = null;
        function upload() {
            return new Promise(function (done) {
                var input = document.createElement('input');
                input.type = 'file';
                input.multiple = true;
                input.accept = '.txt,.md,.json,.cells,.sudoku,.csv,.xfer,text/*';
                input.style.display = 'none';
                function finish(files) { uploadCancel = null; input.remove(); done(files); }
                input.addEventListener('change', function () { finish(Array.prototype.slice.call(input.files || [])); });
                input.addEventListener('cancel', function () { finish([]); });
                uploadCancel = function () { finish([]); };
                document.body.appendChild(input);
                input.click();
            });
        }

        /* ctx is set when a script runs a line: its output goes where the
           script's own output goes, and it knows how deep the scripts are. */
        function makeIo(stdin, sink, ctx) {
            var interactive = !sink && (ctx ? ctx.interactive : true);
            var depth = ctx ? ctx.depth : 0;
            return {
                cols: term.cols, rows: term.rows, interactive: interactive,
                stdin: stdin,
                /* A long command checks this between steps and stops. */
                interrupted: function () { return interrupted; },
                /* For a command run at the prompt, the whole terminal:
                   takeOver({ data(keys), resize(cols, rows), close() })
                   returns { cols, rows, write(text or bytes), release() }. */
                takeOver: interactive && !ctx ? takeOver : null,
                out: function (s) {
                    if (interrupted) { return; }
                    if (sink) { sink.push(strip(s)); } else if (ctx) { ctx.out(s); } else { write(s); }
                },
                err: function (s) { write(paint('red', String(s).replace(/\n$/, '')) + '\n'); },
                /* Reads any file as text, for commands that take files. */
                read: function (p) {
                    var n = resolve(fs, cwd, p);
                    if (!n) { return Promise.reject(new Error(p + ': no such file or directory')); }
                    if (n.children) { return Promise.reject(new Error(p + ': is a directory')); }
                    return textOf(realOf(n));
                },
                findScript: function (name) { var n = findProgram(fs, cwd, name); return n && isScript(n) ? n : null; },
                runScript: function (node, args) { return runScript(node, args, this, depth); },
                fs: {
                    resolve: function (p) { return resolve(fs, cwd, p); },
                    cwd: function () { return cwd; },
                    chdir: function (n) { cwd = n; announce(); },
                    write: function (p, content, append) { return writeFile(fs, cwd, p, content, append); },
                    save: homeSave,
                    mkdir: mkdir,
                    remove: remove,
                    move: move
                },
                prefetch: prefetchTexts,
                open: openEntry,
                request: function (verb, req) { return window.pudlApplets.request(verb, req, root); },
                edit: editor,
                openEditor: openEditor,
                download: download,
                upload: upload,
                clear: function () { term.clear(); },
                history: {
                    list: function () { return History.list().slice(); },
                    clear: function () { History.clear(); }
                },
                pager: pager
            };
        }

        /* Runs one line. Resolves to its status: 0 for success, and
           anything else for a failure, which stops a script. */
        async function execute(text, ctx) {
            if (!ctx) { homeSync(); }
            var fail = function (m) { write(paint('red', m) + '\n'); return 2; };
            var parsed = parse(text);
            if (parsed.error) { return fail(parsed.error); }
            var stages = parsed.stages.filter(function (s) { return s.length; });
            if (!stages.length) { return 0; }
            /* A redirection may end only the last stage, and names a file. */
            var redirect = null;
            for (var s = 0; s < stages.length; s++) {
                var at = -1;
                stages[s].forEach(function (w, j) { if (w && typeof w === 'object' && w.redirect && at < 0) { at = j; } });
                if (at < 0) { continue; }
                if (s !== stages.length - 1) { return fail('a redirection (> or >>) can only end the line'); }
                var target = stages[s][at + 1];
                // A file name with a wild character in it is taken as written.
                if (target && typeof target === 'object' && target.glob != null) { target = target.word; }
                if (typeof target !== 'string' || stages[s].length !== at + 2) { return fail('a redirection needs one file name after it'); }
                redirect = { mode: stages[s][at].redirect, path: target };
                stages[s] = stages[s].slice(0, at);
                if (!stages[s].length) { return fail('nothing to redirect'); }
            }
            var cwdPath = pathOf(cwd);
            var stdin = ctx ? ctx.stdin : null;
            var status = 0;
            for (var i = 0; i < stages.length && !interrupted; i++) {
                // Patterns are matched as each stage starts, from where it runs.
                var words = expandGlobs(stages[i], cwd), c = COMMANDS[words[0]] || programAt(fs, cwd, words[0]);
                if (!c) { write(paint('red', words[0] + ': command not found. Type "help" for the list.') + '\n'); return 127; }
                var last = i === stages.length - 1;
                var sink = last && !redirect ? null : [];
                status = (await c.run(words.slice(1), makeIo(stdin, sink, ctx))) || 0;
                if (sink) { stdin = sink.join(''); }
            }
            if (redirect && !interrupted) {
                var res = await writeFile(fs, cwd, redirect.path, stdin || '', redirect.mode === '>>');
                if (res.error) { write(paint('red', res.error) + '\n'); status = 1; }
            }
            /* A command may have removed or moved the current directory. */
            if (!attached(cwd)) { cwd = resolve(fs, fs, cwdPath) || homeDir(); announce(); }
            return interrupted ? 130 : status;
        }

        /* Runs a script's lines in order, stopping at the first that fails.
           Its output goes wherever the script's does, so a script can sit
           in a pipe, and what is piped into it reaches its first line. */
        var SCRIPT_DEPTH = 8;
        async function runScript(node, args, io, depth) {
            if (depth >= SCRIPT_DEPTH) { io.err(node.name + ': scripts call each other too deeply (the limit is ' + SCRIPT_DEPTH + ')'); return 1; }
            var lines = scriptSource(node).replace(/\r/g, '').split('\n');
            var first = true, status = 0;
            for (var i = 0; i < lines.length && !interrupted; i++) {
                var t = lines[i].trim();
                if (!t || t[0] === '#') { continue; }
                status = await execute(expand(t, node.name, args), {
                    out: io.out, stdin: first ? io.stdin : null, depth: depth + 1, interactive: io.interactive
                });
                first = false;
                if (status) { break; }
            }
            return status;
        }

        /* Opens a file in whatever applet opens files, the graphical Editor
           on this site. A file that does not exist yet opens as a new one. */
        function openEditor(path) {
            var n = resolve(fs, cwd, path);
            if (n && n.children) { write(paint('red', path + ': is a directory') + '\n'); return 1; }
            var target = n ? displayPath(realOf(n)) : (path[0] === '/' || path[0] === '~' ? path : displayPath(cwd).replace(/\/$/, '') + '/' + path);
            var kind = n ? F.kindOf(n) : 'file';
            if (!window.pudlApplets.request('open', { path: target, kind: kind }, root)) {
                write(paint('red', path + ': nothing on this site opens it') + '\n');
                return 1;
            }
            return 0;
        }

        /* Opens a page, an applet or a link the way the current view opens
           things. An applet may be handed a starting state, and a page of
           its own takes a query instead. */
        function openEntry(n, state, pageQuery) {
            if (n.kind === 'link') { window.open(n.url, '_blank', 'noopener'); return; }
            /* On a separate edit origin the pages and the other applets live
               on the public site, so they open there, in a tab of their own,
               even from a window of the admin desktop. That origin's layout
               gives the public site's in pc-public-origin; where the server
               tools are on the public site itself, as they are now, things
               open here, as they do for anyone. */
            var q = pageQuery || state;
            var publicOrigin = document.querySelector('meta[name="pc-public-origin"]');
            if (F.mounted && publicOrigin && publicOrigin.content && publicOrigin.content !== location.origin) {
                var url = F.publicUrl(n);
                if (url) { window.open(url + (q ? '?' + q : ''), '_blank', 'noopener'); }
                return;
            }
            var inWindows = opts.host === 'window' && window.pudlWindows;
            if (inWindows) {
                if (state != null) {
                    window.pcAppletHandoff = window.pcAppletHandoff || {};
                    window.pcAppletHandoff[n.name] = state;
                    if ((window.pudlWindows.state().open || []).indexOf(n.name) >= 0) { window.pudlWindows.close(n.name); }
                }
                window.pudlWindows.open(n.name);
                return;
            }
            location.assign('/page/' + encodeURIComponent(n.name) + (q ? '?' + q : ''));
        }

        /* A full-screen pager on the alternate screen. */
        function pager(text, label) {
            return new Promise(function (resolveDone) {
                var lines = [];
                String(text).replace(/\n$/, '').split('\n').forEach(function (l) {
                    var plain = strip(l);
                    if (plain.length <= term.cols) { lines.push(l); return; }
                    for (var i = 0; i < plain.length; i += term.cols) { lines.push(plain.slice(i, i + term.cols)); }
                });
                var top = 0;
                function page() { return Math.max(1, term.rows - 1); }
                function draw() {
                    var out = '\x1b[H\x1b[2J';
                    for (var i = 0; i < page(); i++) { out += (lines[top + i] != null ? lines[top + i] : paint('dim', '~')) + '\r\n'; }
                    var end = Math.min(lines.length, top + page());
                    out += '\x1b[7m ' + label + '  ' + (lines.length ? Math.round(end * 100 / lines.length) : 100) + '%  (space: next, b: back, q: quit) \x1b[0m';
                    term.write(out);
                }
                function key(k) {
                    var max = Math.max(0, lines.length - page());
                    if (k === 'q' || k === 'Q' || k === '\x1b' || k === '\x03') { term.write('\x1b[?1049l'); resolveDone(0); return; }
                    if (k === ' ' || k === 'f' || k === '\x1b[6~') { top = Math.min(max, top + page()); }
                    else if (k === 'b' || k === '\x1b[5~') { top = Math.max(0, top - page()); }
                    else if (k === '\x1b[B' || k === 'j' || k === '\r') { top = Math.min(max, top + 1); }
                    else if (k === '\x1b[A' || k === 'k') { top = Math.max(0, top - 1); }
                    else if (k === 'g' || k === '\x1b[H') { top = 0; }
                    else if (k === 'G' || k === '\x1b[F') { top = max; }
                    draw();
                    keyWaiter = key;
                }
                term.write('\x1b[?1049h');
                draw();
                keyWaiter = key;
            });
        }

        /* === The editor ================================================
           A small nano-style editor on the alternate screen: a title bar,
           the text, a message line and a line of keys. It edits files in
           the home directory only. Tabs become spaces. */
        async function editor(path) {
            var node = resolve(fs, cwd, path);
            if (node) {
                if (node.children) { write(paint('red', 'edit: ' + path + ': is a directory') + '\n'); return 1; }
                node = realOf(node);
                if (!node.home) { write(paint('red', 'edit: ' + path + ' is part of the site, which is read-only. Copy it first: cp ' + path + ' ~/') + '\n'); return 1; }
            } else {
                var sp = splitPath(path), dir = resolve(fs, cwd, sp.dir);
                if (!dir || !dir.children) { write(paint('red', 'edit: ' + sp.dir + ': no such directory') + '\n'); return 1; }
                if (!dir.home) { write(paint('red', 'edit: files can only be created in your home directory (~)') + '\n'); return 1; }
                if (!validName(sp.base)) { write(paint('red', 'edit: ' + sp.base + ' is not a valid file name') + '\n'); return 1; }
            }
            var label = node ? displayPath(node) : path;
            var text = '';
            if (node) {
                try { text = await F.read(node); } catch (err) { write(paint('red', 'edit: ' + err.message) + '\n'); return 1; }
            }
            /* Like nano, the buffer always ends in an empty line, so there is
               somewhere to move to and paste after the last line of text. */
            var lines = text.replace(/\r\n?/g, '\n').replace(/\t/g, '    ').split('\n');
            if (lines[lines.length - 1] !== '') { lines.push(''); }
            function textLines() { return lines.length - 1; }
            var row = 0, col = 0, goal = 0, top = 0, left = 0, modified = false;
            var message = node ? '' : 'New file', messageIsError = false, cut = [], lastWasCut = false, confirming = false, saving = false;
            var KEYS = [['^S', 'Save'], ['^X', 'Exit'], ['^K', 'Cut line'], ['^U', 'Paste'], ['^G', 'Keys']];

            return new Promise(function (done) {
                var resizeSub = term.onResize(function () { draw(); });

                function say(m, isError) { message = m; messageIsError = !!isError; }
                function clampCol() { col = Math.min(col, lines[row].length); }

                function draw() {
                    var rows = term.rows, cols = term.cols, textRows = Math.max(1, rows - 3);
                    if (row < top) { top = row; }
                    if (row >= top + textRows) { top = row - textRows + 1; }
                    if (col < left) { left = col; }
                    if (col >= left + cols) { left = col - cols + 1; }
                    var title = ' edit  ' + label + (modified ? '  (modified)' : '');
                    var out = '\x1b[?25l\x1b[H' + C.inverse + (title + ' '.repeat(cols)).slice(0, cols) + C.reset + '\r\n';
                    for (var i = 0; i < textRows; i++) {
                        var l = lines[top + i];
                        out += '\x1b[2K' + (l != null ? l.slice(left, left + cols) : paint('dim', '~')) + '\r\n';
                    }
                    out += '\x1b[2K' + (message ? (messageIsError ? paint('red', message.slice(0, cols)) : message.slice(0, cols)) : '') + '\r\n';
                    var bar = '', used = 0;
                    KEYS.forEach(function (k) {
                        var piece = k[0] + ' ' + k[1] + '  ';
                        if (used + piece.length > cols) { return; }
                        bar += C.inverse + k[0] + C.reset + ' ' + k[1] + '  ';
                        used += piece.length;
                    });
                    out += '\x1b[2K' + bar;
                    out += '\x1b[' + (row - top + 2) + ';' + (col - left + 1) + 'H\x1b[?25h';
                    term.write(out);
                }

                function insertText(s) {
                    var parts = s.replace(/\r\n?/g, '\n').replace(/\t/g, '    ').replace(/[\x00-\x09\x0b-\x1f]/g, '').split('\n');
                    var head = lines[row].slice(0, col), tail = lines[row].slice(col);
                    if (parts.length === 1) {
                        lines[row] = head + parts[0] + tail;
                        col += parts[0].length;
                    } else {
                        var add = [head + parts[0]].concat(parts.slice(1, -1), [parts[parts.length - 1] + tail]);
                        lines.splice.apply(lines, [row, 1].concat(add));
                        row += parts.length - 1;
                        col = parts[parts.length - 1].length;
                    }
                    goal = col;
                    modified = true;
                }

                async function save() {
                    if (saving) { return false; }
                    saving = true;
                    say('Saving…');
                    draw();
                    var content = lines.join('\n');
                    if (content && content[content.length - 1] !== '\n') { content += '\n'; }
                    var res = await writeFile(fs, cwd, node ? displayPath(node) : path, content, false);
                    saving = false;
                    if (res.error) { say('Not saved: ' + res.error, true); return false; }
                    node = res.node;
                    label = displayPath(node);
                    modified = false;
                    say(node.special ? 'Saved the layouts.'
                        : 'Saved ' + label + ' (' + textLines() + (textLines() === 1 ? ' line)' : ' lines)'));
                    return true;
                }

                function exit() {
                    resizeSub.dispose();
                    term.write('\x1b[?1049l');
                    done(0);
                }

                async function key(k) {
                    if (confirming) {
                        if (k === 'y' || k === 'Y') { confirming = false; if (await save()) { exit(); return; } }
                        else if (k === 'n' || k === 'N') { exit(); return; }
                        else if (k === '\x1b' || k === '\x03') { confirming = false; say('Still editing.'); }
                        draw();
                        keyWaiter = key;
                        return;
                    }
                    var cutting = false;
                    if (k.length > 1 && k[0] !== '\x1b') { insertText(k); }
                    else {
                        switch (k) {
                            case '\x13': case '\x0f': await save(); break;
                            case '\x18':
                                if (!modified) { exit(); return; }
                                confirming = true;
                                say('Save changes to ' + label + '? y to save, n to discard, Esc to keep editing');
                                break;
                            case '\x0b':
                                cutting = true;
                                if (!lastWasCut) { cut = []; }
                                cut.push(lines[row]);
                                if (lines.length > 1) { lines.splice(row, 1); if (row >= lines.length) { row = lines.length - 1; } } else { lines[0] = ''; }
                                col = 0; goal = 0; modified = true;
                                say('Cut ' + cut.length + (cut.length === 1 ? ' line' : ' lines'));
                                break;
                            case '\x15':
                                if (!cut.length) { say('Nothing has been cut yet.'); break; }
                                lines.splice.apply(lines, [row, 0].concat(cut));
                                row += cut.length; col = 0; goal = 0; modified = true;
                                break;
                            case '\x07':
                                say('Ctrl+S save, Ctrl+X exit, Ctrl+K cut line, Ctrl+U paste, arrows/Home/End/PgUp/PgDn move');
                                break;
                            case '\x03':
                                say('Line ' + (row + 1) + ' of ' + textLines() + ', column ' + (col + 1));
                                break;
                            case '\r': insertText('\n'); break;
                            case '\t': insertText('    '.slice(col % 4)); break;
                            case '\x7f': case '\b':
                                if (col > 0) { lines[row] = lines[row].slice(0, col - 1) + lines[row].slice(col); col--; modified = true; }
                                else if (row > 0) { col = lines[row - 1].length; lines[row - 1] += lines[row]; lines.splice(row, 1); row--; modified = true; }
                                goal = col;
                                break;
                            case '\x1b[3~':
                                if (col < lines[row].length) { lines[row] = lines[row].slice(0, col) + lines[row].slice(col + 1); modified = true; }
                                else if (row < lines.length - 1) { lines[row] += lines[row + 1]; lines.splice(row + 1, 1); modified = true; }
                                break;
                            case '\x1b[D': if (col > 0) { col--; } else if (row > 0) { row--; col = lines[row].length; } goal = col; break;
                            case '\x1b[C': if (col < lines[row].length) { col++; } else if (row < lines.length - 1) { row++; col = 0; } goal = col; break;
                            case '\x1b[A': if (row > 0) { row--; col = goal; clampCol(); } break;
                            case '\x1b[B': if (row < lines.length - 1) { row++; col = goal; clampCol(); } break;
                            case '\x1b[H': case '\x1bOH': case '\x01': col = 0; goal = 0; break;
                            case '\x1b[F': case '\x1bOF': case '\x05': col = lines[row].length; goal = col; break;
                            case '\x1b[5~': row = Math.max(0, row - Math.max(1, term.rows - 4)); col = goal; clampCol(); break;
                            case '\x1b[6~': row = Math.min(lines.length - 1, row + Math.max(1, term.rows - 4)); col = goal; clampCol(); break;
                            default:
                                if (k >= ' ' && k.indexOf('\x1b') < 0) { insertText(k); }
                        }
                    }
                    lastWasCut = cutting;
                    if (lines[lines.length - 1] !== '') { lines.push(''); }
                    if (!confirming && k !== '\x13' && k !== '\x0f' && k !== '\x07' && k !== '\x03' && k !== '\x0b' && k !== '\x15' && message && !messageIsError && message !== 'New file') { say(''); }
                    draw();
                    keyWaiter = key;
                }

                term.write('\x1b[?1049h');
                draw();
                keyWaiter = key;
            });
        }

        /* === Start ===================================================== */

        function applyTheme() { if (term) { term.options.theme = themeColors(root); } }
        document.addEventListener('pudl:theme-change', applyTheme);

        /* A state handed to this terminal, by a shell request that found
           no room for a new terminal or by a preset: it moves to the
           directory and puts any command at the prompt. While a command
           runs, which it does when this terminal made the request itself,
           it waits for the prompt. */
        var waitingRequest = null;
        function takeState(s) {
            if (!s || !fs || !term) { return; }
            if (busy) { waitingRequest = s; return; }
            applyRequest(s);
        }
        function applyRequest(s) {
            var p = cwdFrom(s), n = p ? resolve(fs, fs, F.upgradePath(p)) : null;
            if (n && n.children) { cwd = n; announce(); }
            var run = new URLSearchParams(String(s)).get('run');
            if (run) { line = ''; cursor = 0; }
            redraw();
            if (run) { insert(run); }
            term.focus();
        }

        /* === Snapshots ================================================

           The terminal's snapshot (js/snapshot.js) is what its screen
           shows, scrollback and colours included, written out by xterm's
           serialize add-on, and the command line being typed. A reload
           draws the screen again and puts the line back to carry on with.
           A program running at the time, such as ssh, ended with the page,
           which the screen then says. The folder is the state's, in the
           address. */
        var serializer = null;
        var snap = window.pcSnapshot ? window.pcSnapshot.join(root, takeSnapshot) : null;
        var SCREEN_MAX = 600000;
        function takeSnapshot() {
            if (!term || !serializer) { return null; }
            var text;
            try {
                /* A full-screen program's own screen goes with it. */
                text = serializer.serialize({ scrollback: 2000, excludeAltBuffer: true, excludeModes: true });
                if (text.length > SCREEN_MAX) { text = serializer.serialize({ scrollback: 300, excludeAltBuffer: true, excludeModes: true }); }
            } catch (err) { return null; }
            return { screen: text, line: busy ? '' : line, cursor: busy ? 0 : cursor, busy: busy };
        }
        function restoreScreen(kept) {
            term.write(kept.screen);
            if (kept.busy) {
                write('\n\nThe page was reloaded, which ended the program that was running here.\n');
                term.write(prompt());
                return;
            }
            line = typeof kept.line === 'string' ? kept.line : '';
            cursor = Math.max(0, Math.min(+kept.cursor || 0, line.length));
            redraw();
        }

        var boot = Promise.all([loadXterm(), loadSiteFs().then(function (fsApi) { return fsApi.load(); }), loadExtras()]).then(async function (results) {
            if (destroyed) { return; }
            fs = results[1];
            await History.load();
            /* Settings that can't be read leave the defaults in place. */
            var config = await loadConfig().catch(function () { return null; });
            if (config) {
                applySettings(await config.load('terminal', SETTINGS).catch(function () { return SETTINGS; }));
                unConfig = config.onChange('terminal', applySettings);
            } else { applySettings(SETTINGS); }
            if (destroyed) { return; }
            /* /bin lists the commands, which live here rather than in the
               filesystem. */
            F.defineCommands(Object.keys(COMMANDS).map(function (k) { return { name: k, summary: COMMANDS[k].summary }; }));
            cwd = resolve(fs, fs, F.upgradePath(initialCwd));
            if (!cwd || !cwd.children) { cwd = fs; }
            term = new window.Terminal({
                cursorBlink: true,
                fontFamily: getComputedStyle(root).getPropertyValue('--mono').trim() || 'ui-monospace, "Cascadia Mono", Consolas, monospace',
                fontSize: fontSize(),
                scrollback: 2000,
                theme: themeColors(root),
                /* xterm.js lightens or darkens any text that would fall
                   below this contrast with its background, so colours a
                   remote program picks, such as ls's blue on green for a
                   directory anyone may write to, stay readable in the
                   site's palette. 4.5 is WCAG AA for body text. */
                minimumContrastRatio: 4.5,
                allowProposedApi: false
            });
            fit = new window.FitAddon.FitAddon();
            term.loadAddon(fit);
            serializer = new window.SerializeAddon.SerializeAddon();
            term.loadAddon(serializer);
            term.open(screen);
            try { fit.fit(); } catch (err) { }
            term.onData(onData);
            if (window.ResizeObserver) {
                ro = new ResizeObserver(function () { try { fit.fit(); } catch (err) { } });
                ro.observe(screen);
            }
            var kept = snap && snap.kept;
            if (kept && typeof kept.screen === 'string') { restoreScreen(kept); }
            else {
                write(paint('bold', 'parkscomputing.com') + ' in terminal mode. Type ' + paint('green', 'help') + ' for the commands, or ' + paint('green', 'guide') + ' for the full guide; your own files are in ' + paint('blue', '~') + '.\n\n');
                term.write(prompt());
            }
            if (prefill) { insert(prefill); }
            if (snap) { term.onWriteParsed(snap.changed); }
            announce();
            if ((snap && snap.focused) || opts.ownsUrl || root.closest('.win.active')) { term.focus(); }
        }).catch(function (err) {
            if (destroyed) { return; }
            screen.textContent = 'The terminal could not start: ' + err.message + '.';
            screen.classList.add('pc-terminal-failed');
        });

        return {
            state: stateString,
            setState: takeState,
            commands: commands,
            menus: function () {
                var items = commands(), settingsCommand = items.shift();
                settingsCommand.label = 'Settings for all terminals…';
                return { titles: [
                    { label: 'Terminal', items: window.pcAppletIdentity(root, [settingsCommand]) },
                    { label: 'Session', items: items }
                ] };
            },
            destroy: function () {
                destroyed = true;
                if (snap) { snap.leave(); }
                if (raw && raw.close) { raw.close(); }
                if (unConfig) { unConfig(); }
                document.removeEventListener('pudl:theme-change', applyTheme);
                if (ro) { ro.disconnect(); }
                if (term) { term.dispose(); }
            },
            ready: boot
        };
    }

    window.pudlApplets.register('terminal', { init: init });
})();
