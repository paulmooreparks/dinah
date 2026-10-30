/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
   Ported from parkscomputing.com's js/terminal.js and js/terminal-text.js.
   The original sources and their license are kept in vendor/parks-terminal.

   Dinah's Terminal: the
   site's shell, with its parser, wildcards, pipelines, redirection, scripts,
   commands, pager and nano-style editor, over Dinah's virtual file
   system. The site's file system is a tree held in memory; this one is
   paged and partly remote, so commands reach it through an asynchronous
   file system object (makeFs) that keeps a short-lived cache of folder
   listings for wildcards and completion. Local folders and Home can be
   changed. */
(function () {
  'use strict';
  // The product's name as people see it (product.js).
  function productName() { return window.dinahProduct ? window.dinahProduct.name : ''; }
  // A command's summary and help say {product} where the product's name
  // goes, which is put in as they are shown.
  function named(text) { return String(text).split('{product}').join(productName()); }
  var base = new URL('.', document.currentScript.src), loading;
  var MAX_INPUT = 4096, MAX_OUTPUT = 65536, MAX_HISTORY = 200, MAX_PIPE = 8 << 20, LIST_TTL = 10000, SCRIPT_DEPTH = 8;

  /* ANSI styling. Output that goes into a pipe or a file is stripped of it. */
  var C = { reset: '\x1b[0m', bold: '\x1b[1m', dim: '\x1b[2m', inverse: '\x1b[7m', red: '\x1b[31m', green: '\x1b[32m', yellow: '\x1b[33m', blue: '\x1b[34m', cyan: '\x1b[36m' };
  function strip(s) { return String(s).replace(/\x1b\[[0-9;?]*[A-Za-z]/g, ''); }
  function paint(color, s) { return C[color] + s + C.reset; }
  /* Text that comes from a document or a person, which must not reach the
     screen as control sequences: escape becomes a visible symbol, and other
     control and bidirectional characters are dropped. */
  function safe(value) {
    return String(value == null ? '' : value).replace(/\r\n?/g, '\n').replace(/\x1b/g, '\u241b')
      .replace(/[\x00-\x08\x0b-\x1f\x7f-\x9f\u202a-\u202e\u2066-\u2069]/g, '');
  }

  /* === Words ===============================================================
     A line is a list of pipelines joined by ;, && and ||; a pipeline is
     stages joined by |; a stage is words, with < file, > file or >> file.
     Single quotes keep everything; double quotes keep everything but $
     expansions and backslash escapes; outside quotes, a backslash escapes
     one character. $NAME, ${NAME}, $? and the script parameters $0 to $9,
     $@ and $# are expanded from lookup(name). A word with an unquoted *, ?
     or [ is a pattern, { glob, word }: glob is the word with its quoted,
     escaped and expanded characters escaped, so only the unquoted ones are
     wild, and word is what it says, for when nothing matches. */
  function parse(line, lookup) {
    lookup = lookup || function () { return ''; };
    var list = [], stages = [[]], word = '', pat = '', globbed = false, inWord = false, quote = null, op = null, from = 0;
    function lit(ch) { word += ch; pat += /[*?[\]\\]/.test(ch) ? '\\' + ch : ch; }
    function flush() { if (inWord) { stages[stages.length - 1].push(globbed ? { glob: pat, word: word } : word); word = ''; pat = ''; globbed = false; inWord = false; } }
    /* Each pipeline keeps its own text, so that its $ expansions can be made
       again when it runs, after the commands before it have run. */
    function endPipeline(next, at, width) {
      flush();
      var used = stages.filter(function (s) { return s.length; });
      if (!used.length) {
        if (!next && op && op !== ';') return 'a command is missing after ' + op;
        if (next) return 'a command is missing before ' + next;
        return null;
      }
      if (used.length !== stages.length) return 'a command is missing around |';
      list.push({ op: op, stages: stages, source: line.slice(from, at) }); stages = [[]]; op = next; from = at + (width || 0); return null;
    }
    function variable(i) {
      var rest = line.slice(i + 1), m;
      if ((m = /^\{([A-Za-z_][A-Za-z0-9_]*|[0-9@#?])(?::-([^}]*))?\}/.exec(rest))) { var v = lookup(m[1]); return { value: v === '' && m[2] != null ? m[2] : v, length: m[0].length, name: m[1] }; }
      if ((m = /^([A-Za-z_][A-Za-z0-9_]*|[0-9@#?])/.exec(rest))) return { value: lookup(m[1]), length: m[1].length, name: m[1] };
      return null;
    }
    for (var i = 0; i < line.length; i++) {
      var ch = line[i];
      if (quote === "'") { if (ch === quote) quote = null; else lit(ch); continue; }
      if (quote === '"') {
        if (ch === quote) quote = null;
        else if (ch === '\\' && i + 1 < line.length && /["\\$`]/.test(line[i + 1])) lit(line[++i]);
        else if (ch === '$' && variable(i)) { var dv = variable(i); String(dv.value).split('').forEach(lit); i += dv.length; }
        else lit(ch);
        continue;
      }
      if (ch === '"' || ch === "'") { quote = ch; inWord = true; continue; }
      if (ch === '\\' && i + 1 < line.length) { lit(line[++i]); inWord = true; continue; }
      if (ch === '$' && variable(i)) {
        var v = variable(i); i += v.length;
        // An unquoted $@ alone is each argument as its own word.
        if (v.name === '@' && !inWord && (i + 1 >= line.length || /[\s;|&<>]/.test(line[i + 1]))) { (lookup('@list') || []).forEach(function (arg) { stages[stages.length - 1].push(arg); }); continue; }
        String(v.value).split('').forEach(lit); inWord = true; continue;
      }
      if (ch === '*' || ch === '?' || ch === '[') { word += ch; pat += ch; globbed = true; inWord = true; continue; }
      if (ch === '#' && !inWord) { line = line.slice(0, i); break; }
      if (ch === '|' && line[i + 1] === '|') { var e1 = endPipeline('||', i, 2); if (e1) return { error: e1 }; i++; continue; }
      if (ch === '&' && line[i + 1] === '&') { var e2 = endPipeline('&&', i, 2); if (e2) return { error: e2 }; i++; continue; }
      if (ch === '&') return { error: 'running a command in the background (&) is not supported' };
      if (ch === ';') { var e3 = endPipeline(';', i, 1); if (e3) return { error: e3 }; continue; }
      if (ch === '|') { flush(); stages.push([]); continue; }
      if (ch === '>' || ch === '<') {
        flush();
        var token = ch === '>' && line[i + 1] === '>' ? '>>' : ch;
        if (token === '>>') i++;
        stages[stages.length - 1].push({ redirect: token }); continue;
      }
      if (/\s/.test(ch)) { flush(); continue; }
      word += ch; pat += ch; inWord = true;
    }
    if (quote) return { error: 'unterminated quote' };
    var end = endPipeline(null, line.length, 0); if (end) return { error: end };
    return { list: list };
  }

  /* One name's pattern (a part between slashes, with \ escaping a
     character) as a regular expression, or null when nothing in it is wild. */
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
          if (neg) body = body.slice(1);
          re += '[' + (neg ? '^' : '') + body.replace(/\\(.)/g, '$1').replace(/[\\\]^]/g, '\\$&') + ']';
          wild = true; i = end; continue;
        }
      }
      re += ch.replace(/[.*+?^${}()|[\]\\/]/g, '\\$&');
    }
    return wild ? new RegExp('^' + re + '$') : null;
  }
  function unescapeGlob(s) { return s.replace(/\\(.)/g, '$1'); }
  function globToRegex(glob, insensitive) { return new RegExp('^' + glob.replace(/[.+^${}()|[\]\\]/g, '\\$&').replace(/\*/g, '.*').replace(/\?/g, '.') + '$', insensitive ? 'i' : ''); }

  /* Joins a path to the current folder and settles . and .. lexically, as
     a shell does; ~ is the person's home. */
  function resolve(cwd, value, home) {
    value = String(value);
    home = home || '/home/dinah';
    if (value === '~' || value.startsWith('~/')) value = home + value.slice(1);
    if (/[\\\x00-\x1f\x7f]/.test(value) || value.length > 8192) throw new Error(value.slice(0, 80) + ': not a workspace path (use forward slashes)');
    var parts = value.startsWith('/') ? [] : cwd.split('/').filter(Boolean);
    value.split('/').forEach(function (part) { if (part === '..') parts.pop(); else if (part && part !== '.') parts.push(part); });
    return '/' + parts.join('/');
  }
  function nameOf(path) { return path === '/' ? '/' : path.slice(path.lastIndexOf('/') + 1); }
  function parentOf(path) { var i = path.lastIndexOf('/'); return i <= 0 ? '/' : path.slice(0, i); }

  /* Wraps prose at word boundaries to a width, keeping indented lines
     (tables and examples) as they are. */
  function wrap(text, width) {
    return String(text).split('\n').map(function (line) {
      if (/^\s/.test(line) || strip(line).length <= width) return line;
      var out = [], cur = '';
      line.split(' ').forEach(function (w) { if (cur && strip(cur + ' ' + w).length > width) { out.push(cur); cur = w; } else cur = cur ? cur + ' ' + w : w; });
      if (cur) out.push(cur);
      return out.join('\n');
    }).join('\n');
  }
  function columns(names, width) {
    var plain = names.map(strip), w = plain.reduce(function (m, s) { return Math.max(m, s.length); }, 0) + 2;
    var per = Math.max(1, Math.floor(width / w)), rows = Math.ceil(names.length / per), out = [];
    for (var r = 0; r < rows; r++) {
      var line = '';
      for (var c = 0; c < per; c++) { var i = c * rows + r; if (i < names.length) line += names[i] + ' '.repeat(Math.max(0, w - plain[i].length)); }
      out.push(line.replace(/\s+$/, ''));
    }
    return out.join('\n');
  }
  /* A listing's dates are in the computer's regional format (regional.js),
     with fixed-width fields, and a missing date is as wide as a date, so
     that the columns after it line up. */
  const { fmtDate, fmtSize } = window.dinahHelpers;
  function linesOf(text) { return !text ? [] : String(text).replace(/\r/g, '').replace(/\n$/, '').split('\n'); }
  const message = window.dinahHelpers.message;

  /* The comment block at the top of a script, for man: # lines in a shell
     script, -- lines in a Lua one. */
  function scriptHelp(src) {
    var out = [], lines = String(src).replace(/\r/g, '').split('\n');
    for (var i = 0; i < lines.length; i++) {
      var t = lines[i].trim();
      if (t.indexOf('#!') === 0) continue;
      var m = /^(#|--)\s?(.*)$/.exec(t);
      if (!m || /^--\[/.test(t)) { if (out.length || t) break; continue; }
      out.push(m[2]);
    }
    return out.join('\n');
  }
  /* === Interpreters ========================================================
     The languages a script may be written in besides the Terminal's own.
     Each is a module in a file of its
     own that registers itself here, so one can be added, replaced or left
     out without changing the Terminal:

       dinahTerminal.registerInterpreter({
         id, name,             'lua', 'Lua 5.4'
         command,              a command that runs it, such as 'lua'
         summary, help,        that command's line in help, and its man page
         matches(path, text),  whether a script is in this language: by its
                               name or by its first line
         run(source, ctx)      runs a script and resolves to its status
       })

     ctx is all a script can reach: { name, dir, args, stdin, write(text),
     error(text), interrupted(), request(op, args) }, where request is the
     Terminal's host (hostRequest below), the same for every language. */
  var INTERPRETERS = [];
  function registerInterpreter(def) {
    INTERPRETERS = INTERPRETERS.filter(function (d) { return d.id !== def.id; }).concat([def]);
    if (def.command) command(def.command, { summary: def.summary, help: def.help, complete: 'file', run: function (args, io) { return interpreterCommand(def, args, io); } });
  }
  function interpreterFor(path, text) {
    for (var i = 0; i < INTERPRETERS.length; i++) if (INTERPRETERS[i].matches(path, text)) return INTERPRETERS[i];
    return null;
  }
  /* An interpreter's command: <command> <file> [arguments...], or
     <command> -e <code> [arguments...], where the interpreter may give
     another flag for code, such as sh's -c. */
  async function interpreterCommand(def, args, io) {
    var me = def.command, inline = def.inline || '-e';
    if (!args.length) { io.err(me + ': which script? Or "' + me + ' ' + inline + ' <code>"'); return 1; }
    if (args[0] === inline) {
      if (args.length < 2) { io.err(me + ': ' + inline + ' needs some code'); return 1; }
      // As with sh -c, the word after the code may be the script's name.
      if (def.inlineName) return io.runWith(def, args[1], args.length > 2 ? args[2] : me, io.fs.cwd(), args.slice(3).map(io.tilde));
      return io.runWith(def, args[1], '(command line)', io.fs.cwd(), args.slice(2).map(io.tilde));
    }
    var e = await io.fs.stat(args[0]);
    if (!e || e.directory) { io.err(me + ': ' + args[0] + ': no such file'); return 127; }
    var text;
    try { text = await io.read(args[0]); } catch (err) { io.err(me + ': ' + message(err)); return 126; }
    return io.runWith(def, text, nameOf(e.path), parentOf(e.path), args.slice(1).map(io.tilde));
  }

  /* === Commands ============================================================
     A command is { name, summary, help, complete, run(args, io) }, and io is
     all it can reach. run resolves to its status: 0, or nothing, for
     success. */
  var COMMANDS = {};
  function command(name, def) { def.name = name; COMMANDS[name] = def; }
  function alias(name, target) { var t = COMMANDS[target]; command(name, { summary: 'the same as ' + target, help: name + ' is another name for ' + target + '.\n\n' + t.help, complete: t.complete, run: t.run }); }
  function flags(args, allowed) {
    var set = {}, rest = [], done = false;
    args.forEach(function (a) {
      if (!done && a === '--') { done = true; return; }
      if (!done && new RegExp('^-[' + allowed + ']+$').test(a)) { a.slice(1).split('').forEach(function (f) { set[f] = true; }); return; }
      rest.push(a);
    });
    return { set: set, rest: rest };
  }
  /* The text a command works on: its files, one after another, or what was
     piped into it. Resolves to null after reporting a problem. */
  async function input(files, io, name) {
    if (!files.length) {
      if (io.stdin != null) return io.stdin;
      io.err(name + ': which file? (or pipe text into it)'); return null;
    }
    var parts = [];
    for (var i = 0; i < files.length; i++) {
      try { parts.push(await io.read(files[i])); } catch (err) { io.err(name + ': ' + message(err)); return null; }
    }
    return parts.join('');
  }
  function countOption(args, fallback, name, io) {
    var n = fallback, rest = [];
    for (var i = 0; i < args.length; i++) {
      var a = args[i], m;
      if (a === '-n') n = args[++i];
      else if ((m = /^-n?(\d+)$/.exec(a))) n = m[1];
      else { rest.push(a); continue; }
      if (!/^\d+$/.test(String(n))) { io.err(name + ': -n needs a number'); return null; }
    }
    return { n: +n, rest: rest };
  }

  command('help', {
    summary: 'list the commands',
    help: 'help\n\nLists every command with a one-line summary, and the standard scripts in /bin. "man <command>" says more about one.',
    run: async function (args, io) {
      io.out(paint('bold', productName() + ' Terminal.') + ' Your workspaces are in /workspaces, and ~ is your own folder.\n');
      Object.keys(COMMANDS).sort().forEach(function (k) { io.out('  ' + paint('green', k.padEnd(11)) + named(COMMANDS[k].summary) + '\n'); });
      var tools = io.tools();
      if (tools.length) { io.out('\n' + paint('bold', 'Tools and applets') + '\n'); tools.forEach(function (t) { io.out('  ' + paint('green', t.name.padEnd(11)) + (t.description || '') + '\n'); }); }
      io.out('\n' + wrap('Local folders and ~ can be changed, and "edit -g" opens a file in the Editor. Tab completes, the arrow keys recall earlier commands, | pipes one command into another, > and >> save output in a file, and ;, && and || join commands. A script runs by its path, or by its name from ~/bin or /bin, where ' + productName() + ' keeps its standard scripts; yours come first. Scripts are Terminal commands, or Lua 5.4 ("man lua").', io.cols - 1) + '\n');
      if (io.listBin) {
        var standard = await io.listBin();
        if (standard.length) io.out('\n' + paint('bold', 'Standard scripts in /bin') + '\n' + wrap(standard.join('  '), io.cols - 1) + '\n');
      }
    }
  });
  command('man', {
    summary: 'explain a command',
    help: 'man <command>\n\nShows the help for a command, or the comments at the top of a script.',
    complete: 'command',
    run: async function (args, io) {
      if (!args.length) { io.err('man: which command? Try "help".'); return 1; }
      var c = COMMANDS[args[0]];
      if (c) { io.out(paint('bold', c.name) + ' - ' + named(c.summary) + '\n\n' + wrap(named(c.help), io.cols - 1) + '\n'); return; }
      var tool = io.tools().find(function (t) { return t.name === args[0]; });
      if (tool) { io.out(paint('bold', tool.name) + ' - ' + (tool.description || 'a tool') + '\n\n' + (tool.usage || tool.name) + '\n'); return; }
      var script = await io.findScript(args[0]);
      if (!script) { io.err('man: no entry for ' + args[0]); return 1; }
      var text = await io.read(script.path);
      io.text(paint('bold', script.name) + ' - a script in ' + io.display(parentOf(script.path)) + '\n\n' + wrap(scriptHelp(text) || 'It has no comments to explain it. "cat ' + io.display(script.path) + '" shows what it does.', io.cols - 1) + '\n');
    }
  });

  function styledName(e, label) {
    var name = label || nameOf(e.path);
    if (e.directory) return paint('blue', C.bold + name) + '/';
    if (/\.(sh|lua)$/.test(name) || /^\/bin\/[^/]+$/.test(e.path || '')) return paint('green', name) + '*';
    return name;
  }
  function kindLabel(e) {
    if (e.kind === 'workspace' || e.kind === 'catalog') return 'dir ';
    return e.directory ? 'dir ' : 'file';
  }
  command('ls', {
    summary: 'list a folder',
    help: 'ls [-l] [-a] [-d] [path...]\n\nLists a folder, or the current one. Folders end in /. An entry whose title differs from its name shows the title beside it. Names starting with a dot are hidden unless you ask for them.\n\n  -l   one entry per line, with its date, kind, and size or title\n  -a   show the hidden entries too\n  -d   list folders themselves, not what is in them',
    complete: 'path',
    run: async function (args, io) {
      var o = flags(args, 'lad'), paths = o.rest.length ? o.rest : ['.'], status = 0, files = [], dirs = [];
      function byName(x, y) { return (x.label || nameOf(x.e.path)).localeCompare(y.label || nameOf(y.e.path), undefined, { sensitivity: 'base', numeric: true }); }
      for (var i = 0; i < paths.length; i++) {
        var e = await io.fs.stat(paths[i]);
        if (!e) { io.err('ls: ' + paths[i] + ': no such file or folder'); status = 1; continue; }
        (e.directory && !o.set.d ? dirs : files).push({ e: e, label: paths[i] });
      }
      files.sort(byName); dirs.sort(byName);
      function emit(entries) {
        var titled = entries.some(function (x) { return x.e.title && x.e.title !== nameOf(x.e.path) && !x.e.writable; });
        if (o.set.l) {
          entries.forEach(function (x) {
            var e = x.e, extra = e.writable ? (e.directory ? '' : fmtSize(e.size || 0)) : (e.title && e.title !== nameOf(e.path) ? safe(e.title) : '');
            io.out(paint('dim', fmtDate(e.modified)) + '  ' + kindLabel(e).padEnd(7) + ' ' + styledName(e, x.label) + (extra ? '  ' + paint('dim', extra) : '') + '\n');
          });
        } else if (entries.length && !io.interactive) {
          entries.forEach(function (x) { io.out((x.label || nameOf(x.e.path)) + '\n'); });
        } else if (titled) {
          entries.forEach(function (x) { var e = x.e, t = e.title && e.title !== nameOf(e.path) ? '  ' + paint('dim', safe(e.title)) : ''; io.out(styledName(e, x.label) + t + '\n'); });
        } else if (entries.length) {
          io.out(columns(entries.map(function (x) { return styledName(x.e, x.label); }), io.cols) + '\n');
        }
      }
      emit(files);
      for (var d = 0; d < dirs.length; d++) {
        if (files.length + dirs.length > 1) io.out((files.length || d ? '\n' : '') + dirs[d].label + ':\n');
        var kids;
        try { kids = await io.fs.list(dirs[d].e.path); } catch (err) { io.err('ls: ' + dirs[d].label + ': ' + message(err)); status = 1; continue; }
        emit(kids.filter(function (c) { return o.set.a || nameOf(c.path).charAt(0) !== '.'; }).map(function (c) { return { e: c }; }).sort(byName));
      }
      return status;
    }
  });
  command('cd', {
    summary: 'change folder',
    help: 'cd [path]\n\nMoves into a folder. With no path, or ~, it goes to your home folder; / is the top, and /workspaces lists your workspaces. cd - goes back to the folder you were in before.',
    complete: 'dir',
    run: async function (args, io) {
      var target = args[0] || '~';
      if (target === '-') { target = io.env('OLDPWD'); if (!target) { io.err('cd: there is no previous folder'); return 1; } }
      var e = await io.fs.stat(target);
      if (!e) { io.err('cd: ' + target + ': no such folder'); return 1; }
      if (!e.directory) { io.err('cd: ' + target + ': not a folder'); return 1; }
      io.fs.chdir(e);
    }
  });
  command('pwd', { summary: 'show the current folder', help: 'pwd\n\nPrints the path of the current folder.', run: function (args, io) { io.text(io.fs.cwd() + '\n'); } });
  command('tree', {
    summary: 'show a folder and everything under it',
    help: 'tree [-L depth] [path]\n\nDraws a folder and what is in it as a tree, five levels deep unless -L says otherwise, and at most 2000 entries.',
    complete: 'dir',
    run: async function (args, io) {
      var depth = 5, path = '.';
      for (var i = 0; i < args.length; i++) { if (args[i] === '-L') depth = Math.max(1, +args[++i] || 1); else path = args[i]; }
      var root = await io.fs.stat(path);
      if (!root) { io.err('tree: ' + path + ': no such folder'); return 1; }
      io.out(paint('blue', C.bold + path) + '\n');
      var counts = { d: 0, f: 0, shown: 0 };
      async function walk(e, prefix, level) {
        var kids;
        try { kids = (await io.fs.list(e.path)).filter(function (c) { return nameOf(c.path)[0] !== '.'; }); } catch (err) { io.out(prefix + paint('red', message(err)) + '\n'); return; }
        for (var k = 0; k < kids.length && !io.interrupted(); k++) {
          if (++counts.shown > 2000) { if (counts.shown === 2001) io.out(prefix + '… (stopped at 2000 entries)\n'); return; }
          var last = k === kids.length - 1, c = kids[k];
          io.out(prefix + (last ? '└── ' : '├── ') + styledName(c) + '\n');
          if (c.directory) { counts.d++; if (level < depth) await walk(c, prefix + (last ? '    ' : '│   '), level + 1); } else counts.f++;
        }
      }
      await walk(root, '', 1);
      io.out('\n' + counts.d + ' folders, ' + counts.f + ' files\n');
    }
  });
  command('cat', {
    summary: 'print a file',
    help: 'cat <file...>\n\nPrints files, one after another, or what is piped into it.',
    complete: 'file',
    run: async function (args, io) {
      if (!args.length) { if (io.stdin != null) { io.text(io.stdin); return; } io.err('cat: which file?'); return 1; }
      var status = 0;
      for (var i = 0; i < args.length && !io.interrupted(); i++) {
        try { var text = await io.read(args[i]); io.text(text); if (text && !text.endsWith('\n') && io.interactive) io.out('\n'); } catch (err) { io.err('cat: ' + args[i] + ': ' + message(err)); status = 1; }
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
      if (args.length) { try { text = await io.read(args[0]); } catch (err) { io.err('less: ' + args[0] + ': ' + message(err)); return 1; } }
      else if (io.stdin != null) text = io.stdin;
      else { io.err('less: which file?'); return 1; }
      if (!io.interactive) { io.text(text); return; }
      return io.pager(text, args[0] || '(standard input)');
    }
  });
  alias('more', 'less');
  command('grep', {
    summary: 'search text for a pattern',
    help: 'grep [-i] [-l] [-n] [-v] [-r] <pattern> [path...]\n\nPrints the lines matching a pattern (a regular expression). With no path it searches what is piped into it. With -r it searches every file under a folder, or under the current one.\n\n  -i   ignore case\n  -l   print only the names of files that match\n  -n   number the lines\n  -v   print the lines that do not match\n  -r   search folders',
    complete: 'path',
    run: async function (args, io) {
      var o = { set: {}, rest: [] };
      args.forEach(function (a) { if (/^-[ilnvr]+$/.test(a) && !o.rest.length) a.slice(1).split('').forEach(function (f) { o.set[f] = true; }); else o.rest.push(a); });
      if (!o.rest.length) { io.err('grep: which pattern?'); return 2; }
      var re;
      try { re = new RegExp(o.rest[0], o.set.i ? 'i' : ''); } catch (err) { io.err('grep: bad pattern: ' + err.message); return 2; }
      var paths = o.rest.slice(1), found = false;
      function scan(text, label) {
        var hits = 0;
        linesOf(text).forEach(function (line, n) {
          if (re.test(line) === !!o.set.v) return;
          hits++; found = true;
          if (o.set.l) return;
          var shown = o.set.v ? safe(line) : safe(line).replace(new RegExp(re.source, re.flags + 'g'), function (m) { return m ? paint('red', C.bold + m) : m; });
          io.styled((label ? paint('cyan', label) + ':' : '') + (o.set.n ? paint('green', String(n + 1)) + ':' : '') + shown + '\n', (label ? label + ':' : '') + (o.set.n ? (n + 1) + ':' : '') + line + '\n');
        });
        if (hits && o.set.l && label) io.text(label + '\n');
      }
      if (!paths.length && io.stdin != null) { scan(io.stdin, ''); return found ? 0 : 1; }
      if (!paths.length) { if (!o.set.r) { io.err('grep: which file? (or -r to search this folder)'); return 2; } paths = ['.']; }
      var files = [];
      for (var i = 0; i < paths.length; i++) {
        var e = await io.fs.stat(paths[i]);
        if (!e) { io.err('grep: ' + paths[i] + ': no such file or folder'); continue; }
        if (e.directory) {
          if (!o.set.r) { io.err('grep: ' + paths[i] + ': is a folder'); continue; }
          await io.fs.walk(e, function (c) { if (!c.directory) files.push(c); });
        } else files.push(e);
      }
      var many = files.length > 1 || o.set.r;
      for (var f = 0; f < files.length && !io.interrupted(); f++) {
        try { scan(await io.read(files[f].path), many ? io.display(files[f].path) : ''); } catch (err) { if (!many) { io.err('grep: ' + paths[0] + ': ' + message(err)); return 2; } }
      }
      return found ? 0 : 1;
    }
  });
  command('find', {
    summary: 'find files by name',
    help: 'find [path] [-name <pattern>] [-iname <pattern>] [-type f|d] [-maxdepth n]\n\nLists every path under a folder, or only those whose name matches a pattern such as "*.md". -name is exact about case and -iname is not.',
    complete: 'path',
    run: async function (args, io) {
      var start = '.', re = null, type = '', depth = 20;
      for (var i = 0; i < args.length; i++) {
        if (args[i] === '-name' || args[i] === '-iname') { var p = args[++i]; if (p == null) { io.err('find: ' + args[i - 1] + ' needs a pattern'); return 2; } re = globToRegex(p, args[i - 1] === '-iname'); }
        else if (args[i] === '-type') { type = args[++i]; if (type !== 'f' && type !== 'd') { io.err('find: -type is f or d'); return 2; } }
        else if (args[i] === '-maxdepth') depth = Math.max(0, +args[++i] || 0);
        else start = args[i];
      }
      var root = await io.fs.stat(start);
      if (!root) { io.err('find: ' + start + ': no such file or folder'); return 1; }
      var prefix = start.replace(/\/$/, '');
      function show(e) {
        if (re && !re.test(nameOf(e.path))) return;
        if (type === 'f' && e.directory || type === 'd' && !e.directory) return;
        var rel = e.path === root.path ? prefix : prefix + e.path.slice(root.path.length);
        io.text(rel + '\n');
      }
      show(root);
      if (root.directory) await io.fs.walk(root, show, depth);
    }
  });
  command('echo', { summary: 'print its arguments', help: 'echo [-n] [text...]\n\nPrints its arguments, which is mostly useful with a pipe or a redirection: echo hello > ~/greeting. -n leaves off the line break.', run: function (args, io) { var n = args[0] === '-n'; io.text((n ? args.slice(1) : args).join(' ') + (n ? '' : '\n')); } });
  command('clear', { summary: 'clear the screen', help: 'clear\n\nClears the screen. Ctrl+L does the same.', run: function (args, io) { io.clear(); } });
  command('history', {
    summary: 'list earlier commands',
    help: 'history [-c]\n\nLists the commands typed in this terminal. -c clears the list.',
    run: function (args, io) { if (args[0] === '-c') { io.history.clear(); return; } io.history.list().forEach(function (h, i) { io.text(String(i + 1).padStart(4) + '  ' + h + '\n'); }); }
  });
  command('date', { summary: 'show the date and time', help: 'date\n\nShows the local date and time.', run: function (args, io) { io.out(dinahRegional.format(new Date(), 'full') + '\n'); } });
  command('which', {
    summary: 'say what runs for a name',
    help: 'which <name...>\n\nSays whether a name is a built-in command, a tool or a script, and where the script is.',
    complete: 'command',
    run: async function (args, io) {
      var status = 0;
      for (var i = 0; i < args.length; i++) {
        var n = args[i];
        if (COMMANDS[n]) { io.out(n + ': built-in command\n'); continue; }
        if (io.tools().some(function (t) { return t.name === n; })) { io.out(n + ': ' + productName() + ' tool\n'); continue; }
        var s = await io.findScript(n);
        if (s) io.text(io.display(s.path) + '\n'); else { io.err(n + ': not found'); status = 1; }
      }
      return status;
    }
  });
  command('stat', {
    summary: 'describe a file or folder',
    help: 'stat <path...>\n\nShows what {product} knows about a path: its kind, title, size, time, revision and whether it can be changed here.',
    complete: 'path',
    run: async function (args, io) {
      if (!args.length) { io.err('stat: which path?'); return 1; }
      var status = 0;
      for (var i = 0; i < args.length; i++) {
        var e = await io.fs.stat(args[i]);
        if (!e) { io.err('stat: ' + args[i] + ': no such file or folder'); status = 1; continue; }
        io.text('  Path: ' + e.path + '\n  Title: ' + (e.title || nameOf(e.path)) + '\n  Kind: ' + (e.kind || (e.directory ? 'folder' : 'file')) + (e.directory ? ' (folder)' : '') + '\n' +
          (e.size != null && !e.directory ? '  Size: ' + fmtSize(e.size) + '\n' : '') + (e.modified ? '  Changed: ' + fmtDate(e.modified) + '\n' : '') +
          (e.revision ? '  Revision: ' + e.revision + '\n' : '') + '  Writable: ' + (e.writable ? 'yes' : 'no') + '\n' + (e.url ? '  Address: ' + e.url + '\n' : ''));
      }
      return status;
    }
  });
  command('env', { summary: 'list the variables', help: 'env\n\nLists the variables: HOME, PWD, USER and any you have exported.', run: function (args, io) { io.env().forEach(function (pair) { io.text(pair[0] + '=' + pair[1] + '\n'); }); } });
  command('export', {
    summary: 'set a variable',
    help: 'export NAME=value...\n\nSets variables, which $NAME then expands to in later commands. They are kept with this terminal. export with no arguments lists them.',
    run: function (args, io) {
      if (!args.length) return COMMANDS.env.run([], io);
      for (var i = 0; i < args.length; i++) {
        var m = /^([A-Za-z_][A-Za-z0-9_]*)=([\s\S]*)$/.exec(args[i]);
        if (!m) { io.err('export: ' + args[i] + ': use NAME=value'); return 1; }
        if (!io.setEnv(m[1], m[2])) { io.err('export: ' + m[1] + ' cannot be changed'); return 1; }
      }
    }
  });
  command('unset', { summary: 'remove a variable', help: 'unset NAME...\n\nRemoves variables set with export.', run: function (args, io) { args.forEach(function (n) { io.setEnv(n, null); }); } });
  command('true', { summary: 'succeed', help: 'true\n\nDoes nothing, successfully.', run: function () { return 0; } });
  command('false', { summary: 'fail', help: 'false\n\nDoes nothing, and fails.', run: function () { return 1; } });
  command('exit', { summary: 'close this terminal', help: 'exit\n\nCloses this terminal window. In a script, it stops the script.', run: function (args, io) { return io.exit(); } });

  /* --- Changing files --------------------------------------------------- */
  command('edit', {
    summary: 'edit a file in the terminal',
    help: 'edit [-g] <file>\n\nOpens a file in a full-screen editor, creating it if it does not exist yet. Files in local folders and ~ can be changed.\n\n  Ctrl+S   save (Ctrl+O works too)\n  Ctrl+X   exit, asking to save any changes\n  Ctrl+K   cut the current line\n  Ctrl+U   paste the lines cut last\n  Ctrl+W   find text\n  Ctrl+G   show the keys\n\nThe arrow keys, Home, End, Page Up and Page Down move around.\n\nWith -g the file opens in {product}\'s Editor instead.',
    complete: 'file',
    run: function (args, io) {
      var graphical = args[0] === '-g';
      if (graphical) args = args.slice(1);
      if (!args.length) { io.err('edit: which file? For a new one, give it a name: edit ~/notes.md'); return 1; }
      return graphical ? io.openEditor(args[0]) : io.edit(args[0]);
    }
  });
  alias('nano', 'edit');
  command('open', {
    summary: 'open a file or folder in {product}',
    help: 'open <path>\n\nOpens a file in {product}\'s viewer, or a folder in Files, or a file in the program Windows uses for it.',
    complete: 'path',
    run: async function (args, io) {
      if (!args.length) { io.err('open: which path?'); return 1; }
      var e = await io.fs.stat(args[0]);
      if (!e) { io.err('open: ' + args[0] + ': no such file or folder'); return 1; }
      await io.open(e);
    }
  });
  command('touch', {
    summary: 'create an empty file',
    help: 'touch <file...>\n\nCreates each file that does not exist yet, empty. A file that exists is left as it is.',
    complete: 'file',
    run: async function (args, io) {
      if (!args.length) { io.err('touch: which file?'); return 1; }
      var status = 0;
      for (var i = 0; i < args.length; i++) {
        if (await io.fs.stat(args[i])) continue;
        try { await io.fs.write(args[i], '', 'create'); } catch (err) { io.err('touch: ' + message(err)); status = 1; }
      }
      return status;
    }
  });
  command('mkdir', {
    summary: 'make a folder',
    help: 'mkdir [-p] <folder...>\n\nMakes folders.\n\n  -p   make any missing folders above it too, and do not complain if it exists',
    complete: 'dir',
    run: async function (args, io) {
      var o = flags(args, 'p');
      if (!o.rest.length) { io.err('mkdir: which folder?'); return 1; }
      var status = 0;
      for (var i = 0; i < o.rest.length; i++) { try { await io.fs.mkdir(o.rest[i], !!o.set.p); } catch (err) { io.err('mkdir: ' + message(err)); status = 1; } }
      return status;
    }
  });
  command('rm', {
    summary: 'remove files',
    help: 'rm [-r] [-f] <path...>\n\nRemoves files, permanently: they do not go to the Recycle Bin.\n\n  -r   remove a folder and everything in it\n  -f   say nothing about paths that do not exist\n\nA folder that holds hidden or system files is not removed.',
    complete: 'path',
    run: async function (args, io) {
      var o = flags(args, 'rRf');
      if (!o.rest.length) { if (o.set.f) return 0; io.err('rm: which file?'); return 1; }
      var status = 0;
      for (var i = 0; i < o.rest.length; i++) {
        var p = o.rest[i], e = await io.fs.stat(p);
        if (!e) { if (!o.set.f) { io.err('rm: ' + p + ': no such file or folder'); status = 1; } continue; }
        if (e.directory && !(o.set.r || o.set.R)) { io.err('rm: ' + p + ': is a folder (use -r)'); status = 1; continue; }
        try { var n = await io.fs.remove(p, !!(o.set.r || o.set.R)); if (e.directory) io.text('removed ' + p + ' (' + n + (n === 1 ? ' entry' : ' entries') + ')\n'); } catch (err) { io.err('rm: ' + message(err)); status = 1; }
      }
      return status;
    }
  });
  command('rmdir', {
    summary: 'remove an empty folder',
    help: 'rmdir <folder...>\n\nRemoves empty folders.',
    complete: 'dir',
    run: async function (args, io) {
      if (!args.length) { io.err('rmdir: which folder?'); return 1; }
      var status = 0;
      for (var i = 0; i < args.length; i++) {
        var e = await io.fs.stat(args[i]);
        if (!e) { io.err('rmdir: ' + args[i] + ': no such folder'); status = 1; continue; }
        if (!e.directory) { io.err('rmdir: ' + args[i] + ': not a folder'); status = 1; continue; }
        try { await io.fs.remove(args[i], false); } catch (err) { io.err('rmdir: ' + message(err)); status = 1; }
      }
      return status;
    }
  });
  /* cp and mv put sources into a folder named last, or give one source a
     new name, as a shell's do. */
  async function transfer(name, args, io, recursive) {
    if (args.length < 2) { io.err(name + ': give a source and a destination'); return 1; }
    var sources = args.slice(0, -1), dest = args[args.length - 1], into = await io.fs.stat(dest), status = 0;
    if (sources.length > 1 && !(into && into.directory)) { io.err(name + ': ' + dest + ': not a folder'); return 1; }
    for (var i = 0; i < sources.length; i++) {
      var target = into && into.directory ? dest.replace(/\/$/, '') + '/' + nameOf(io.fs.path(sources[i])) : dest;
      try {
        if (name === 'cp') await io.fs.copy(sources[i], target, recursive);
        else await io.fs.move(sources[i], target);
      } catch (err) { io.err(name + ': ' + message(err)); status = 1; }
    }
    return status;
  }
  command('cp', {
    summary: 'copy files',
    help: 'cp [-r] <source...> <destination>\n\nCopies files into a folder, or one file to a new name. An existing file at the destination is replaced. Copies go between local folders and ~.\n\n  -r   copy folders and everything in them',
    complete: 'path',
    run: function (args, io) { var o = flags(args, 'rR'); return transfer('cp', o.rest, io, !!(o.set.r || o.set.R)); }
  });
  command('mv', {
    summary: 'move or rename files',
    help: 'mv <source...> <destination>\n\nMoves files and folders into a folder, or renames one. An existing file at the destination is replaced; an existing folder never is.',
    complete: 'path',
    run: function (args, io) { return transfer('mv', args, io, true); }
  });

  /* --- Text tools ------------------------------------------------------- */
  command('head', {
    summary: 'show the first lines of text',
    help: 'head [-n count] [file...]\n\nPrints the first lines, ten unless you say otherwise, of its files or of what is piped into it.',
    complete: 'file',
    run: async function (args, io) {
      var o = countOption(args, 10, 'head', io); if (!o) return 2;
      var text = await input(o.rest, io, 'head'); if (text == null) return 1;
      linesOf(text).slice(0, o.n).forEach(function (l) { io.text(l + '\n'); });
    }
  });
  command('tail', {
    summary: 'show the last lines of text',
    help: 'tail [-n count] [file...]\n\nPrints the last lines, ten unless you say otherwise, of its files or of what is piped into it.',
    complete: 'file',
    run: async function (args, io) {
      var o = countOption(args, 10, 'tail', io); if (!o) return 2;
      var text = await input(o.rest, io, 'tail'); if (text == null) return 1;
      var lines = linesOf(text);
      lines.slice(Math.max(0, lines.length - o.n)).forEach(function (l) { io.text(l + '\n'); });
    }
  });
  command('wc', {
    summary: 'count lines, words and characters',
    help: 'wc [-l] [-w] [-c] [file...]\n\nCounts the lines, words and characters of its files or of what is piped into it.\n\n  -l   lines only\n  -w   words only\n  -c   characters only',
    complete: 'file',
    run: async function (args, io) {
      var o = flags(args, 'lwc'), want = Object.keys(o.set).length ? o.set : { l: true, w: true, c: true };
      var sets = o.rest.length ? o.rest.map(function (f) { return [f]; }) : [[]], totals = [0, 0, 0];
      function format(c) { return ['l', 'w', 'c'].filter(function (f) { return want[f]; }).map(function (f) { return String(c['lwc'.indexOf(f)]).padStart(7); }).join(' '); }
      for (var i = 0; i < sets.length; i++) {
        var text = await input(sets[i], io, 'wc'); if (text == null) return 1;
        var counts = [linesOf(text).length, (text.match(/\S+/g) || []).length, text.length];
        counts.forEach(function (c, k) { totals[k] += c; });
        io.text(format(counts) + (sets[i][0] ? ' ' + sets[i][0] : '') + '\n');
      }
      if (sets.length > 1) io.text(format(totals) + ' total\n');
    }
  });
  command('sort', {
    summary: 'sort lines of text',
    help: 'sort [-r] [-n] [-u] [-f] [file...]\n\nSorts the lines of its files or of what is piped into it.\n\n  -r   reverse the order\n  -n   compare numbers at the start of each line as numbers\n  -u   keep one of each line\n  -f   ignore case',
    complete: 'file',
    run: async function (args, io) {
      var o = flags(args, 'rnuf'), text = await input(o.rest, io, 'sort'); if (text == null) return 1;
      var lines = linesOf(text);
      lines.sort(function (a, b) {
        if (o.set.n) { var d = (parseFloat(a) || 0) - (parseFloat(b) || 0); if (d) return d; }
        var x = o.set.f ? a.toLowerCase() : a, y = o.set.f ? b.toLowerCase() : b;
        return x < y ? -1 : x > y ? 1 : 0;
      });
      if (o.set.r) lines.reverse();
      if (o.set.u) lines = lines.filter(function (l, i) { return i === 0 || l !== lines[i - 1]; });
      lines.forEach(function (l) { io.text(l + '\n'); });
    }
  });
  command('uniq', {
    summary: 'drop repeated lines',
    help: 'uniq [-c] [file...]\n\nDrops a line that repeats the one before it, so sort it first to drop every repeat.\n\n  -c   count how many times each line came',
    complete: 'file',
    run: async function (args, io) {
      var o = flags(args, 'c'), text = await input(o.rest, io, 'uniq'); if (text == null) return 1;
      var prev = null, n = 0;
      function flush() { if (prev != null) io.text((o.set.c ? String(n).padStart(7) + ' ' : '') + prev + '\n'); }
      linesOf(text).forEach(function (l) { if (l === prev) { n++; return; } flush(); prev = l; n = 1; });
      flush();
    }
  });
  command('tee', {
    summary: 'copy text to a file as it passes',
    help: 'tee [-a] <file>\n\nWrites what is piped into it to a file, and passes it on.\n\n  -a   add to the end of the file instead of replacing it',
    complete: 'file',
    run: async function (args, io) {
      var o = flags(args, 'a');
      if (o.rest.length !== 1) { io.err('tee: which file?'); return 1; }
      var text = io.stdin || '';
      try { await io.fs.save(o.rest[0], text, !!o.set.a); } catch (err) { io.err('tee: ' + message(err)); return 1; }
      io.text(text);
    }
  });
  command('basename', { summary: 'the last part of a path', help: 'basename <path>\n\nPrints the last part of a path.', run: function (args, io) { if (!args.length) { io.err('basename: which path?'); return 1; } io.text(nameOf(args[0].replace(/\/+$/, '') || '/') + '\n'); } });
  command('dirname', { summary: 'a path without its last part', help: 'dirname <path>\n\nPrints a path without its last part.', run: function (args, io) { if (!args.length) { io.err('dirname: which path?'); return 1; } var p = args[0].replace(/\/+$/, ''); io.text((p.indexOf('/') < 0 ? '.' : parentOf(p)) + '\n'); } });
  command('source', {
    summary: 'run a script here, keeping what it sets',
    help: 'source <file> [arguments...]\n. <file> [arguments...]\n\nRuns a script in this terminal rather than on its own, so the variables it exports and the folder it changes to stay after it ends. ~/.profile is sourced when a terminal starts.',
    complete: 'file',
    run: function (args, io) {
      if (!args.length) { io.err('source: which file?'); return 1; }
      return io.source(args[0], args.slice(1));
    }
  });
  alias('.', 'source');
  command('log', {
    summary: 'add an entry to {product}\'s log',
    help: 'log [-e | -w] [-s source] [message...]\n\nAdds the message, or each line piped in, to {product}\'s log as information, or with -e as an error or -w as a warning. Its source is "terminal", or with -s a word such as script:backup. Settings › Diagnostics says how much the log keeps and where it is.',
    run: async function (args, io) {
      var level = 'info', source = 'terminal', words = [];
      for (var i = 0; i < args.length; i++) {
        if (args[i] === '-e') level = 'error';
        else if (args[i] === '-w') level = 'warning';
        else if (args[i] === '-s' && i + 1 < args.length) source = args[++i];
        else words.push(args[i]);
      }
      var lines = words.length ? [words.join(' ')] : String(io.stdin || '').split('\n').filter(function (l) { return l.trim(); });
      if (!lines.length) { io.err('log: what should it say?'); return 1; }
      var api = window.dinahHelpers.app();
      if (!api || !api.Log) { io.err('log: the log is unavailable'); return 1; }
      try { for (var j = 0; j < lines.length; j++) await api.Log(source, level, lines[j]); } catch (err) { io.err('log: ' + message(err)); return 1; }
    }
  });
  command('sh', {
    summary: 'run a script',
    help: 'sh <file> [arguments...]\n\nRuns a file of commands, one line at a time, stopping at the first that fails. In it, $1 to $9 are its arguments, $@ all of them, $# how many, and $0 its name. Lines starting with # are comments. A script also runs by its path, such as ./build.sh, or by its name when it is in ~/bin or /bin. A script in Lua runs in Lua; "man lua" says how.',
    complete: 'file',
    run: async function (args, io) {
      if (!args.length) { io.err('sh: which script?'); return 1; }
      var e = await io.fs.stat(args[0]);
      if (!e || e.directory) { io.err('sh: ' + args[0] + ': no such file'); return 127; }
      return io.runScript(e, args.slice(1));
    }
  });

  /* === xterm =============================================================== */
  function assets() {
    if (loading) return loading;
    var css = document.createElement('link'); css.rel = 'stylesheet'; css.href = new URL('vendor/xterm-6.0.0/xterm.css', base); document.head.appendChild(css);
    function script(file) { return new Promise(function (yes, no) { var tag = document.createElement('script'); tag.src = new URL('vendor/xterm-6.0.0/' + file, base); tag.onload = yes; tag.onerror = function () { loading = null; no(new Error('A bundled terminal component could not load.')); }; document.head.appendChild(tag); }); }
    loading = script('xterm.js').then(function () { return script('addon-fit.js'); });
    return loading;
  }
  function theme(host) {
    var probe = document.createElement('span'); probe.hidden = true; host.appendChild(probe);
    function color(token, fallback) { probe.style.color = ''; probe.style.color = 'var(' + token + ', ' + fallback + ')'; return getComputedStyle(probe).color || fallback; }
    var t = { background: color('--input-bg', '#ffffff'), foreground: color('--text', '#111111'), cursor: color('--accent', '#2f6fb3'), cursorAccent: color('--input-bg', '#ffffff'), black: color('--text', '#111111'), red: color('--danger', '#b91c1c'), green: color('--positive', '#1f7a3a'), yellow: color('--warn', '#a16207'), blue: color('--accent', '#2f6fb3'), magenta: '#8b5cf6', cyan: '#0e7490', white: color('--text-muted', '#6b7280'), brightBlack: color('--text-muted', '#6b7280') };
    // Parks Computing's visible selection, in the light and dark themes.
    var rgb = /rgba?\(([^)]+)\)/.exec(t.cursor);
    t.selectionBackground = rgb ? 'rgba(' + rgb[1].split(',').slice(0, 3).join(',') + ', 0.3)' : t.cursor;
    if (document.documentElement.getAttribute('data-theme') === 'dark') { t.cyan = '#22d3ee'; t.magenta = '#c4b5fd'; }
    probe.remove(); return t;
  }

  /* === The file system =====================================================
     The terminal's view of the virtual file system: paths it resolves
     lexically from the current folder, Go bindings for the rest, and a
     cache of folder listings that a change through the terminal, a change
     reported by Dinah, or LIST_TTL's passing clears. expected(path)
     gives the retained identity a path's operations must keep, so a folder
     that was renamed or detached cannot be confused with whatever has its
     old name now. */
  function makeFs(api, expected) {
    var listings = new Map();
    async function stat(path) {
      var e = expected(path);
      try { return await (e ? api.StatWorkspaceMountExpected(path, e) : api.StatWorkspaceMount(path)); } catch (err) {
        if (/no longer attached|identity has changed|no longer registered/.test(message(err))) throw err;
        return null;
      }
    }
    async function list(path) {
      var hit = listings.get(path);
      if (hit && Date.now() - hit.at < LIST_TTL) return hit.entries;
      var e = expected(path), out = [];
      for (var next = 0, pages = 0; next >= 0; pages++) {
        if (pages >= 100) throw new Error('the folder has more than 10000 entries; name what you want in it');
        var page = e ? await api.ListWorkspaceMountsExpected(path, next, 100, e) : await api.ListWorkspaceMounts(path, next, 100);
        out = out.concat(page.entries || []);
        if (page.nextStart >= 0 && page.nextStart <= next) throw new Error('the folder listing went wrong');
        next = typeof page.nextStart === 'number' ? page.nextStart : -1;
      }
      listings.set(path, { at: Date.now(), entries: out });
      return out;
    }
    function forget(path) { listings.delete(path); listings.delete(parentOf(path)); }
    function none() { return { workspaceId: '', sourceId: '', path: '' }; }
    return {
      stat: stat, list: list, forget: forget, clear: function () { listings.clear(); },
      read: function (path) { var e = expected(path); return e ? api.ReadWorkspaceMountExpected(path, e) : api.ReadWorkspaceMount(path); },
      write: function (path, text, mode, revision) { forget(path); return api.WriteWorkspaceMount(path, text, mode, revision || '', expected(path) || none()); },
      mkdir: function (path, parents) { forget(path); return api.MakeWorkspaceDirectory(path, parents, expected(path) || none()); },
      remove: function (path, recursive) { forget(path); listings.forEach(function (_, key) { if (key.startsWith(path + '/')) listings.delete(key); }); return api.RemoveWorkspaceMount(path, recursive, expected(path) || none()); },
      copy: function (from, to, recursive) { forget(to); return api.CopyWorkspaceMount(from, expected(from) || none(), to, expected(to) || none(), recursive); },
      move: function (from, to) { forget(from); forget(to); listings.forEach(function (_, key) { if (key.startsWith(from + '/')) listings.delete(key); }); return api.MoveWorkspaceMount(from, expected(from) || none(), to, expected(to) || none()); }
    };
  }

  /* === The terminal ======================================================== */
  function init(root, opts, item) {
    opts = opts || {}; item = item || {};
    var home = '/home/dinah', user = 'dinah', cwd, cwdRef = null, homeRef = null, key = 'dinah-terminal:' + item.instanceId;
    var term, fit, observer, destroyed = false, busy = false, generation = 0, run = { cancelled: false }, keyWaiter = null, typeahead = [], altScreen = false;
    var line = '', cursor = 0, histPos = 0, draft = '', history = [], output = '', fontSize = 14, explicitFontSize = false, visualTail = Promise.resolve(), unsubscribe, completionRequest = 0, lastTab = 0;
    var env = {}, lastStatus = 0, oldPwd = '';
    root.classList.add('pc-terminal', 'pc-terminal-fill');
    var screen = document.createElement('div'); screen.className = 'pc-terminal-screen'; root.appendChild(screen);
    const app = window.dinahHelpers.app;
    async function exitSensitiveCommand(runCommand) {
      var host=app(), lease=typeof host.BeginExitWork==='function'?await host.BeginExitWork('Built-in terminal command'):null;
      try { return await runCommand(); }
      finally { if(lease) await host.EndExitWork(lease); }
    }
    function save() { if (window.dinahUIState && item.instanceId) dinahUIState.set(key, { cwd: cwd, ref: cwdRef, homeRef: homeRef, history: history.slice(-MAX_HISTORY), output: output.slice(-MAX_OUTPUT), fontSize: explicitFontSize ? fontSize : undefined, env: env }); }
    function announce() { root.dataset.terminalCwd = cwd; if (opts.changed) opts.changed('cwd=' + encodeURIComponent(cwd)); save(); }
    /* write is for the terminal's own text, which may carry its own colours;
       anything from a document or a person goes through safe first. */
    function write(text) {
      text = String(text);
      if (!altScreen) output = (output + text.replace(/\r\n?/g, '\n')).slice(-MAX_OUTPUT);
      term.write(text.replace(/\r?\n/g, '\r\n'));
      if (!altScreen) save();
    }
    function display(path) { return path === home ? '~' : path.startsWith(home + '/') ? '~' + path.slice(home.length) : path; }
    function prompt() { return paint('green', safe(user) + '@dinah') + ':' + paint('blue', C.bold + safe(display(cwd)).replace(/\n|\t/g, ' ')) + '$ '; }
    function redraw() {
      var value = line, position = cursor;
      visualTail = visualTail.then(function () { return new Promise(function (done) {
        if (destroyed || altScreen) { done(); return; }
        term.write('', function () {
          if (destroyed) { done(); return; }
          // The buffer records wrapping and cell widths, including wide
          // characters, so the prompt is redrawn from its first row.
          var buffer = term.buffer.active, row = buffer.baseY + buffer.cursorY, start = row;
          while (start > buffer.baseY && buffer.getLine(start).isWrapped) start--;
          term.write('\r' + (row > start ? '\x1b[' + (row - start) + 'A' : '') + '\x1b[J' + prompt() + value.slice(0, position), function () {
            if (destroyed) { done(); return; }
            var target = { row: buffer.baseY + buffer.cursorY, col: buffer.cursorX };
            term.write(value.slice(position), function () {
              if (destroyed) { done(); return; }
              var up = buffer.baseY + buffer.cursorY - target.row;
              term.write((up > 0 ? '\x1b[' + up + 'A' : '') + '\r' + (target.col ? '\x1b[' + target.col + 'C' : ''), done);
            });
          });
        });
      }); });
      return visualTail;
    }
    function newPrompt() { line = ''; cursor = 0; histPos = history.length; draft = ''; term.write(prompt()); root.dataset.terminalBusy = 'false'; }
    function insert(text) { text = safe(text).replace(/[\n\t]/g, ' '); text = text.slice(0, MAX_INPUT - line.length); line = line.slice(0, cursor) + text + line.slice(cursor); cursor += text.length; redraw(); }
    function current(id) { return !destroyed && generation === id; }
    function sameOwner(a, b) { return a && b && a.workspaceId === b.workspaceId && a.sourceId === b.sourceId; }
    /* The identity a path's operations keep: the current folder's
       workspace, and its source when the path is in the same source, for a
       path given relative to the current folder; an absolute path is a new
       choice and keeps none. */
    var relativeTargets = new Set();
    function expectedFor(path) {
      if (!cwdRef || !relativeTargets.has(path)) return null;
      var o = cwd.split('/'), t = path.split('/');
      if (o[1] === 'workspaces' && t[1] === 'workspaces' && o[2] && t[2] === o[2] && cwdRef.workspaceId) return { workspaceId: cwdRef.workspaceId, sourceId: o[3] && t[3] === o[3] ? cwdRef.sourceId : '', path: '' };
      if (o[1] === 'home' && t[1] === 'home' && o[2] && t[2] === o[2] && cwdRef.sourceId) return { workspaceId: '', sourceId: cwdRef.sourceId, path: '' };
      return null;
    }
    var fs = makeFs(window.dinahHelpers.app(), expectedFor);
    function abs(p) {
      var value = String(p), path = resolve(cwd, value, home);
      if (!value.startsWith('/') && value !== '~' && !value.startsWith('~/')) relativeTargets.add(path);
      return path;
    }
    async function verifyCwd(id) {
      var checkedCwd = cwd;
      var resource = await (window.dinahResources && dinahResources.verify ? dinahResources.verify({ path: cwd, ref: cwdRef }) : app().StatWorkspaceMount(cwd));
      if (!current(id) || cwd !== checkedCwd) return false;
      if (cwdRef && cwdRef.sourceId && !sameOwner(cwdRef, resource.ref)) throw new Error('this folder belongs to a source that is no longer attached; use cd /workspaces to choose a folder');
      if (!resource.directory) throw new Error('the current folder is unavailable; use cd /workspaces to choose a folder');
      if (resource.ref) { cwdRef = resource.ref; if (cwd === home && !homeRef) homeRef = cwdRef; save(); }
      if (resource.path !== cwd) { cwd = resource.path; fs.clear(); announce(); }
      return true;
    }
    function chdir(e) {
      if (e.path !== cwd) oldPwd = cwd;
      cwd = e.path;
      cwdRef = e.ref || null;
      if (cwd === home && e.ref) homeRef = e.ref;
      announce();
    }

    /* --- Variables -------------------------------------------------------- */
    var FIXED = ['HOME', 'PWD', 'USER', 'OLDPWD'];
    function envValue(name) {
      if (name === 'HOME') return home;
      if (name === 'PWD') return cwd;
      if (name === 'USER') return user;
      if (name === 'OLDPWD') return oldPwd;
      if (name === '?') return String(lastStatus);
      if (name === 'PATH' && !Object.prototype.hasOwnProperty.call(env, 'PATH')) return defaultPath();
      return Object.prototype.hasOwnProperty.call(env, name) ? env[name] : '';
    }
    function envList() {
      var names = Object.keys(env);
      if (names.indexOf('PATH') < 0) names.push('PATH');
      return FIXED.map(function (n) { return [n, envValue(n)]; }).concat(names.sort().map(function (n) { return [n, envValue(n)]; }));
    }
    function setEnv(name, value) {
      if (FIXED.indexOf(name) >= 0) return false;
      if (value == null) delete env[name]; else env[name] = String(value).slice(0, 4096);
      save(); return true;
    }

    /* --- Tools ------------------------------------------------------------ */
    function tools() { return window.dinahResources && dinahResources.commands ? dinahResources.commands() : []; }

    /* --- Scripts ---------------------------------------------------------- */
    /* A name is looked up in the folders of PATH, in order, as on Unix; each
       as it is, then with .sh, then with .lua. PATH is ~/bin, then /bin,
       Dinah's own, unless it is set, so a person's script of the same
       name takes the standard one's place. A folder in PATH that is not
       absolute is taken from the current folder. */
    function defaultPath() { return home + '/bin:/bin'; }
    function binDirs(io) {
      var value = io ? io.env('PATH') : envValue('PATH');
      return String(value).split(':').filter(function (d) { return d; });
    }
    async function findScript(word, io) {
      var stat = io ? io.fs.stat : function (p) { return fs.stat(abs(p)); };
      if (word.indexOf('/') >= 0) { var p = await stat(word); return p && !p.directory ? { name: nameOf(p.path), path: p.path, entry: p } : null; }
      var dirs = binDirs(io);
      for (var d = 0; d < dirs.length; d++) {
        var candidates = [dirs[d] + '/' + word, dirs[d] + '/' + word + '.sh', dirs[d] + '/' + word + '.lua'];
        for (var i = 0; i < candidates.length; i++) { var e = await stat(candidates[i]); if (e && !e.directory) return { name: word, path: e.path, entry: e }; }
      }
      return null;
    }
    async function runScript(entry, args, io, depth) {
      if (depth >= SCRIPT_DEPTH) { io.err(nameOf(entry.path) + ': scripts call each other too deeply (the limit is ' + SCRIPT_DEPTH + ')'); return 1; }
      var text;
      try { text = (await fs.read(entry.path)).text; } catch (err) { io.err(nameOf(entry.path) + ': ' + message(err)); return 126; }
      var interpreter = interpreterFor(entry.path, String(text));
      if (interpreter) return runWith(interpreter, String(text), nameOf(entry.path), parentOf(entry.path), args, io, depth);
      var lines = String(text).replace(/\r/g, '').split('\n'), first = true, status = 0;
      var params = { '0': nameOf(entry.path), '#': String(args.length), '@': args.join(' '), '@list': args };
      args.forEach(function (a, i) { if (i < 9) params[String(i + 1)] = a; });
      for (var i = 0; i < lines.length && !io.interrupted(); i++) {
        var t = lines[i].trim();
        if (!t || t[0] === '#') continue;
        status = await executeLine(t, { out: io.write, stdin: first ? io.stdin : null, depth: depth + 1, interactive: io.interactive, params: params, cwd: io.scope.cwd, env: io.scope.env }, io.token);
        first = false;
        if (status === EXIT) { status = 0; break; }
        if (status) break;
      }
      return status;
    }
    var EXIT = -1;

    /* Runs a script with an interpreter module, giving it the Terminal's
       output, input and host. */
    async function runWith(def, source, name, dir, args, io, depth, sourced) {
      try {
        var result = await def.run(source, {
          name: name, dir: dir, args: args, stdin: io.stdin,
          // The folder and variables the script starts with.
          cwd: io.fs.cwd(), env: io.env(),
          write: function (s) { io.write(String(s)); },
          error: function (s) { io.err(String(s)); },
          interrupted: function () { return io.interrupted(); },
          request: function (op, a) { return hostRequest(op, a || [], dir, io, depth); }
        });
        // An interpreter may give back the variables and folder a script
        // ended with, which a sourced script leaves in the Terminal.
        var status = result && typeof result === 'object' ? result.status : result;
        if (sourced && result && typeof result === 'object' && !io.interrupted()) await keepSourced(result, io);
        return io.interrupted() ? 130 : (Number(status) || 0);
      } catch (err) { io.err(name + ': ' + message(err)); return 1; }
    }
    /* What a sourced script exported, changed or unset becomes the
       Terminal's own, and the folder it ended in becomes the current one. */
    async function keepSourced(result, io) {
      if (result.env) {
        Object.keys(result.env).forEach(function (n) { if (FIXED.indexOf(n) < 0 && envValue(n) !== result.env[n]) setEnv(n, result.env[n]); });
        Object.keys(env).forEach(function (n) { if (!Object.prototype.hasOwnProperty.call(result.env, n)) setEnv(n, null); });
      }
      if (result.cwd && result.cwd !== cwd) {
        var e = await fs.stat(result.cwd);
        if (e && e.directory) io.fs.chdir(e);
      }
    }
    /* source <file>: runs a script in the Terminal itself rather than on its
       own, so what it sets stays set. */
    async function sourceScript(path, args, io, depth) {
      var e = await io.fs.stat(path);
      if (!e || e.directory) { io.err('source: ' + path + ': no such file'); return 1; }
      var text;
      try { text = (await fs.read(e.path)).text; } catch (err) { io.err('source: ' + message(err)); return 126; }
      var interpreter = interpreterFor(e.path, String(text));
      if (interpreter) return runWith(interpreter, String(text), nameOf(e.path), parentOf(e.path), args, io, depth, true);
      return runScript(e, args, io, depth);
    }
    function entryInfo(e) { return { name: nameOf(e.path), path: e.path, directory: !!e.directory, size: e.size || 0, modified: e.modified || '', writable: !!e.writable }; }
    async function findModule(name, dir) {
      var rel = name.replace(/\./g, '/');
      var places = [dir, home + '/lib/lua', '/lib/lua'];
      for (var i = 0; i < places.length; i++) {
        var tries = [places[i] + '/' + rel + '.lua', places[i] + '/' + rel + '/init.lua'];
        for (var j = 0; j < tries.length; j++) {
          var e = await fs.stat(tries[j]);
          if (e && !e.directory) return e.path + '\n' + (await fs.read(e.path)).text;
        }
      }
      return '';
    }
    /* The host every interpreter's scripts use: files, folders, commands,
       variables, modules and applets. Each answer is text, true or a
       number; lists and entries are JSON. */
    async function hostRequest(op, a, dir, io, depth) {
      switch (op) {
        case 'read': return await io.read(a[0]);
        case 'save': await io.fs.save(a[0], String(a[1]), a[2] === 'append'); return true;
        case 'list': return JSON.stringify((await io.fs.list(a[0])).map(entryInfo));
        case 'stat': { var s = await io.fs.stat(a[0]); return s ? JSON.stringify(entryInfo(s)) : ''; }
        case 'mkdir': await io.fs.mkdir(a[0], !!a[1]); return true;
        case 'remove': await io.fs.remove(a[0], false); return true;
        case 'move': await io.fs.move(a[0], a[1]); return true;
        case 'cwd': return io.fs.cwd();
        case 'path': return io.fs.path(a[0]);
        /* Opens a draft in progress, by its ID, in the Editor, as Find and
           Open does; the drafts script offers it. */
        case 'open-draft': {
          var draftId = String(a[0] || '');
          if (!/^[0-9a-f]{32}$/.test(draftId)) throw new Error('that is not a draft\'s ID');
          if (!window.dinahEditor || !window.go || !window.dinahHelpers.app().GetDraft) throw new Error('the Editor is unavailable');
          await window.dinahEditor.open(await window.dinahHelpers.app().GetDraft(draftId));
          return true;
        }
        case 'getenv': return io.env(String(a[0]));
        case 'env': return JSON.stringify(io.env());
        /* Runs one command from its words, as a shell script's interpreter
           does for a command that is not its own: a[0] is the words as JSON,
           a[1] the text it reads or null, a[2] true when its output goes where
           the script's goes (to the screen, as it is written) and false when
           it is wanted back, a[3] the script's exported variables as JSON and
           a[4] the script's folder. Its answer is JSON: the status, and the
           output when it was wanted back. */
        case 'execv': {
          if (depth + 1 >= SCRIPT_DEPTH) throw new Error('scripts call each other too deeply (the limit is ' + SCRIPT_DEPTH + ')');
          var argv = JSON.parse(String(a[0])), direct = !!a[2], captured = [];
          if (!Array.isArray(argv) || !argv.length) throw new Error('there is no command to run');
          var sub = { out: direct ? io.write : function (styled, plain) { captured.push(plain == null ? strip(styled) : plain); }, stdin: a[1] == null ? null : String(a[1]), depth: depth + 1, interactive: direct && io.interactive, params: {}, cwd: a[4] ? String(a[4]) : io.scope.cwd, env: a[3] ? JSON.parse(String(a[3])) : io.scope.env };
          var code = await runCommand(argv.map(String), makeIo(sub.stdin, null, sub, io.token), sub);
          return JSON.stringify({ status: code === EXIT ? 0 : code, out: captured.join('') });
        }
        case 'module': return findModule(String(a[0]), dir);
        case 'exec': {
          if (depth + 1 >= SCRIPT_DEPTH) throw new Error('scripts call each other too deeply (the limit is ' + SCRIPT_DEPTH + ')');
          var parts = [];
          var status = await executeLine(String(a[0]), { out: function (styled, plain) { parts.push(plain == null ? strip(styled) : plain); }, stdin: null, depth: depth + 1, interactive: false, params: {} }, io.token);
          return String(status === EXIT ? 0 : status) + '\n' + parts.join('');
        }
        case 'run': {
          if (depth + 1 >= SCRIPT_DEPTH) throw new Error('scripts call each other too deeply (the limit is ' + SCRIPT_DEPTH + ')');
          var st = await executeLine(String(a[0]), { out: io.write, stdin: null, depth: depth + 1, interactive: io.interactive, params: {} }, io.token);
          return st === EXIT ? 0 : st;
        }
        case 'applets': return JSON.stringify(await dinahAppletSDK.catalog());
        case 'log': { var logApi = window.dinahHelpers.app(); if (!logApi || !logApi.Log) throw new Error('the log is unavailable'); await logApi.Log(String(a[0]), String(a[1]), String(a[2])); return ''; }
        case 'invoke': {
          var id = generation;
          var result = await dinahResources.invoke('applets', ['run', String(a[0]), String(a[1])], function () { return current(id) && !io.interrupted(); }, { path: cwd, ref: cwdRef });
          return typeof result === 'string' ? result : JSON.stringify(result);
        }
      }
      throw new Error('this script asked for something the Terminal does not offer (' + op + ')');
    }

    /* --- Output ----------------------------------------------------------- */
    /* token is the line's own: Ctrl+C cancels it, and every step of that line,
       however late it finishes, sees that and changes nothing more. */
    function makeIo(stdin, sink, ctx, token) {
      var interactive = !sink && (ctx ? ctx.interactive : true), depth = ctx ? ctx.depth : 0;
      /* A command a script runs sees the script's own folder and variables,
         which a shell script changes with cd and export, without the
         Terminal's own changing. */
      var here = ctx && ctx.cwd || null, vars = ctx && ctx.env || null;
      function at(p) { return here ? resolve(here, String(p), home) : abs(p); }
      function varValue(name) { return vars && Object.prototype.hasOwnProperty.call(vars, name) ? vars[name] : envValue(name); }
      function varList() {
        if (!vars) return envList();
        var seen = {}, out = [];
        Object.keys(vars).sort().forEach(function (n) { seen[n] = true; out.push([n, vars[n]]); });
        envList().forEach(function (pair) { if (!seen[pair[0]]) out.push(pair); });
        return out;
      }
      function push(text) { if (sink.size + text.length > MAX_PIPE) { if (!sink.full) { sink.full = true; writeErr('the output is more than ' + fmtSize(MAX_PIPE) + '; the rest was left out'); } return; } sink.size += text.length; sink.parts.push(text); }
      function writeErr(s) { write(paint('red', safe(String(s).replace(/\n$/, ''))) + '\n'); }
      /* out is the command's own text, with colours; text is a document's or
         a person's, made safe for the screen; styled gives both forms. */
      function emit(styledText, plain) {
        if (token.cancelled) return;
        if (sink) push(plain);
        else if (ctx) ctx.out(styledText, plain);
        else write(styledText);
      }
      var io = {
        cols: term.cols, rows: term.rows, interactive: interactive, stdin: stdin, token: token,
        interrupted: function () { return token.cancelled; },
        out: function (s) { emit(String(s), strip(s)); },
        text: function (s) { emit(safe(s), String(s)); },
        styled: function (s, plain) { emit(s, plain); },
        write: function (s, plain) { emit(s, plain == null ? strip(s) : plain); },
        err: writeErr,
        display: display,
        tools: tools,
        env: function (name) { return name ? varValue(name) : varList(); },
        setEnv: setEnv,
        scope: { cwd: here, env: vars },
        /* A script is given ~ and ~/... as the home folder's path, as a shell
           gives them; the Terminal's own commands take ~ as it is. */
        tilde: function (a) { a = String(a); return a === '~' ? home : a.startsWith('~/') ? home + a.slice(1) : a; },
        read: async function (p) {
          var path = at(p), e = await fs.stat(path);
          if (!e) throw new Error('no such file or folder');
          if (e.directory && !(e.actions || []).includes('read')) throw new Error('is a folder');
          return (await fs.read(path)).text;
        },
        findScript: function (word) { return findScript(word, io); },
        runScript: function (entry, args) { return runScript(entry, args, io, depth); },
        runWith: function (def, source, name, dir, args) { return runWith(def, source, name, dir, args, io, depth); },
        source: function (p, args) { return sourceScript(p, args || [], io, depth); },
        listBin: async function () {
          try { return (await fs.list('/bin')).filter(function (e) { return !e.directory; }).map(function (e) { return nameOf(e.path).replace(/\.(sh|lua)$/, ''); }).sort(); } catch (err) { return []; }
        },
        fs: {
          path: at,
          cwd: function () { return here || cwd; },
          stat: function (p) { return fs.stat(at(p)); },
          list: function (p) { return fs.list(at(p)); },
          chdir: function (e) { if (!token.cancelled) chdir(e); },
          walk: async function (e, visit, maxDepth) {
            var count = 0;
            async function step(dir, level) {
              var kids = (await fs.list(dir.path)).filter(function (c) { return nameOf(c.path)[0] !== '.'; });
              for (var i = 0; i < kids.length && !token.cancelled; i++) {
                if (++count > 20000) throw new Error('more than 20000 entries; start lower down');
                visit(kids[i]);
                if (kids[i].directory && level < (maxDepth == null ? 20 : maxDepth)) await step(kids[i], level + 1);
              }
            }
            await step(e, 1);
          },
          write: function (p, text, mode, revision) { return fs.write(at(p), text, mode, revision); },
          save: async function (p, text, append) {
            var path = at(p);
            if (!append) return fs.write(path, text, 'replace');
            var e = await fs.stat(path);
            if (!e) return fs.write(path, text, 'create');
            var current = await fs.read(path);
            return fs.write(path, current.text + text, 'expect', current.revision);
          },
          mkdir: function (p, parents) { return fs.mkdir(at(p), parents); },
          remove: function (p, recursive) { return fs.remove(at(p), recursive); },
          copy: function (a, b, recursive) { return fs.copy(at(a), at(b), recursive); },
          move: function (a, b) { return fs.move(at(a), at(b)); }
        },
        open: function (e) { return dinahResources.open(e, function () { return !token.cancelled && !destroyed; }); },
        openEditor: function (p) { return token.cancelled ? 130 : openEditor(p); },
        edit: function (p) { return token.cancelled ? 130 : editor(p, token); },
        pager: function (text, label) { return token.cancelled ? 130 : pager(text, label); },
        clear: function () { output = ''; term.clear(); term.write('\x1b[2J\x1b[H'); save(); },
        history: { list: function () { return history.slice(); }, clear: function () { history = []; histPos = 0; save(); } },
        exit: function () {
          if (ctx) return EXIT;
          var owner = root.closest('[data-win]');
          if (!owner || !window.pudlWindows) { writeErr('exit: the terminal window is unavailable'); return 1; }
          save(); setTimeout(function () { pudlWindows.close(owner.getAttribute('data-win')); }); return 0;
        }
      };
      return io;
    }

    /* --- Running a line --------------------------------------------------- */
    async function expandGlobs(words) {
      var out = [];
      for (var w = 0; w < words.length; w++) {
        var word = words[w];
        if (!word || typeof word !== 'object' || word.glob == null) { out.push(word); continue; }
        var segs = word.glob.split('/'), absolute = segs[0] === '', found = [];
        if (segs[0] === '~' || segs[0] === '\\~') { segs = home.split('/').concat(segs.slice(1)); absolute = true; }
        async function walk(prefix, i) {
          if (i === segs.length) { found.push(prefix); return; }
          var seg = segs[i];
          if (seg === '' && i < segs.length - 1) { await walk(prefix, i + 1); return; }
          var re = segmentRegex(seg);
          var join = function (name) { return prefix === '' ? name : prefix === '/' ? '/' + name : prefix + '/' + name; };
          if (!re) { await walk(join(unescapeGlob(seg)), i + 1); return; }
          var dir;
          try { dir = await fs.list(abs(prefix === '' ? '.' : prefix)); } catch (err) { return; }
          var dotOk = seg[0] === '.', names = dir.map(function (c) { return nameOf(c.path); }).filter(function (n) { return (dotOk || n[0] !== '.') && re.test(n); }).sort();
          for (var k = 0; k < names.length; k++) await walk(join(names[k]), i + 1);
        }
        await walk(absolute ? '/' : '', absolute ? 1 : 0);
        var existing = [];
        for (var f = 0; f < found.length; f++) if (await fs.stat(abs(found[f]))) existing.push(found[f]);
        if (existing.length) out.push.apply(out, existing); else out.push(word.word);
      }
      return out;
    }
    function fail(m) { write(paint('red', safe(m)) + '\n'); return 2; }
    async function runPipeline(stages, ctx, token) {
      var stdin = ctx ? ctx.stdin : null, redirectOut = null, redirectIn = null;
      for (var s = 0; s < stages.length; s++) {
        var words = [];
        for (var j = 0; j < stages[s].length; j++) {
          var w = stages[s][j];
          if (w && typeof w === 'object' && w.redirect) {
            var target = stages[s][j + 1];
            if (target && typeof target === 'object' && target.glob != null) target = target.word;
            if (typeof target !== 'string') return fail('a redirection needs a file name after it');
            j++;
            if (w.redirect === '<') { if (s !== 0) return fail('< can only give input to the first command'); redirectIn = target; }
            else { if (s !== stages.length - 1) return fail('a redirection (> or >>) can only end the line'); redirectOut = { mode: w.redirect, path: target }; }
            continue;
          }
          words.push(w);
        }
        stages[s] = words;
        if (!words.length) return fail('nothing to run');
      }
      if (redirectIn != null) {
        try { stdin = (await fs.read(abs(redirectIn))).text; } catch (err) { return fail(redirectIn + ': ' + message(err)); }
      }
      var status = 0;
      for (var i = 0; i < stages.length && !token.cancelled; i++) {
        // Patterns are matched as each stage starts, from where it runs.
        var argv = await expandGlobs(stages[i]);
        var last = i === stages.length - 1, sink = last && !redirectOut ? null : { parts: [], size: 0 };
        var io = makeIo(stdin, sink, ctx, token);
        status = await runCommand(argv, io, ctx);
        if (status === EXIT) return EXIT;
        if (sink) stdin = sink.parts.join('');
      }
      if (redirectOut && !token.cancelled) {
        try { await makeIo(null, { parts: [], size: 0 }, ctx, token).fs.save(redirectOut.path, stdin || '', redirectOut.mode === '>>'); }
        catch (err) { write(paint('red', safe(redirectOut.path + ': ' + message(err))) + '\n'); status = 1; }
      }
      return token.cancelled ? 130 : status;
    }
    async function runCommand(argv, io, ctx) {
      var name = argv[0], args = argv.slice(1);
      if (COMMANDS[name]) return (await COMMANDS[name].run(args, io)) || 0;
      if (tools().some(function (t) { return t.name === name; })) {
        var id = generation;
        var result = await dinahResources.invoke(name, args, function () { return current(id) && !io.token.cancelled; }, { path: cwd, ref: cwdRef });
        if (result != null && result !== '') io.text((typeof result === 'string' ? result : JSON.stringify(result, null, 2)) + '\n');
        return 0;
      }
      var script = await findScript(name, io);
      if (script) return runScript(script.entry, args.map(io.tilde), io, ctx ? ctx.depth : 0);
      io.err(name + ': command not found. Type "help" for the list.');
      return 127;
    }
    /* Runs one line: pipelines joined by ;, && and ||. Resolves to the last
       pipeline's status, or EXIT when a script's exit stops it. */
    async function executeLine(text, ctx, token) {
      var params = ctx && ctx.params;
      function lookup(name) {
        if (params && Object.prototype.hasOwnProperty.call(params, name)) return params[name];
        if (name === '@list') return [];
        if (params && /^[0-9#@]$/.test(name)) return '';
        return envValue(name);
      }
      var parsed = parse(text, lookup);
      if (parsed.error) return fail(parsed.error);
      var status = 0;
      for (var i = 0; i < parsed.list.length && !token.cancelled; i++) {
        var p = parsed.list[i];
        if (p.op === '&&' && status !== 0) continue;
        if (p.op === '||' && status === 0) continue;
        // Expanded now, after the commands before it have run.
        var now = parse(p.source, lookup);
        if (now.error || now.list.length !== 1) return fail(now.error || 'the line could not be read');
        status = await runPipeline(now.list[0].stages, ctx, token);
        if (status === EXIT) return EXIT;
        // A line stopped with Ctrl+C may finish after the prompt has come back
        // and other lines have run; its status is no longer $?.
        if (!token.cancelled) lastStatus = status;
      }
      return token.cancelled ? 130 : status;
    }

    /* --- Opening in Dinah ------------------------------------------- */
    async function openEditor(p) {
      var path = abs(p), e = await fs.stat(path);
      if (e && e.directory && !(e.actions || []).includes('edit')) { write(paint('red', safe(p + ': is a folder')) + '\n'); return 1; }
      if (!e) {
        try { await fs.write(path, '', 'create'); e = await fs.stat(path); } catch (err) { write(paint('red', safe('edit: ' + message(err))) + '\n'); return 1; }
      }
      if (!e || !e.ref) { write(paint('red', safe(p + ': cannot be opened in the Editor')) + '\n'); return 1; }
      if (!window.dinahEditor) { write(paint('red', 'edit: the Editor is unavailable') + '\n'); return 1; }
      await window.dinahEditor.openResource(e.ref);
      return 0;
    }

    /* --- The pager --------------------------------------------------------- */
    function enterAlt() { altScreen = true; term.write('\x1b[?1049h'); }
    function leaveAlt() { term.write('\x1b[?1049l'); altScreen = false; }
    function pager(text, label) {
      return new Promise(function (finish) {
        var lines = [];
        safe(String(text).replace(/\n$/, '')).split('\n').forEach(function (l) {
          if (l.length <= term.cols) { lines.push(l); return; }
          for (var i = 0; i < l.length; i += term.cols) lines.push(l.slice(i, i + term.cols));
        });
        var top = 0;
        function page() { return Math.max(1, term.rows - 1); }
        function draw() {
          var out = '\x1b[H\x1b[2J';
          for (var i = 0; i < page(); i++) out += (lines[top + i] != null ? lines[top + i] : paint('dim', '~')) + '\r\n';
          var end = Math.min(lines.length, top + page());
          out += C.inverse + ' ' + safe(label) + '  ' + (lines.length ? Math.round(end * 100 / lines.length) : 100) + '%  (space: next, b: back, q: quit) ' + C.reset;
          term.write(out);
        }
        function key(k) {
          var max = Math.max(0, lines.length - page());
          if (k === 'q' || k === 'Q' || k === '\x1b' || k === '\x03') { leaveAlt(); finish(0); return; }
          if (k === ' ' || k === 'f' || k === '\x1b[6~') top = Math.min(max, top + page());
          else if (k === 'b' || k === '\x1b[5~') top = Math.max(0, top - page());
          else if (k === '\x1b[B' || k === 'j' || k === '\r') top = Math.min(max, top + 1);
          else if (k === '\x1b[A' || k === 'k') top = Math.max(0, top - 1);
          else if (k === 'g' || k === '\x1b[H') top = 0;
          else if (k === 'G' || k === '\x1b[F') top = max;
          draw(); keyWaiter = key;
        }
        enterAlt(); draw(); keyWaiter = key;
      });
    }

    /* --- The editor ----------------------------------------------------------
       A small nano-style editor on the alternate screen: a title bar, the
       text, a message line and a line of keys. It changes files that can be
       written here, and saves against the revision it read, so a change made
       meanwhile elsewhere is not overwritten. */
    async function editor(p) {
      var path = abs(p), e = await fs.stat(path), text = '', revision = '';
      if (e) {
        if (e.directory && !(e.actions || []).includes('read')) { write(paint('red', safe('edit: ' + p + ': is a folder')) + '\n'); return 1; }
        if (!e.writable) { write(paint('red', safe('edit: ' + p + ' is read-only here; "edit -g ' + p + '" opens it in the Editor')) + '\n'); return 1; }
        try { var read = await fs.read(path); text = read.text; revision = read.revision; } catch (err) { write(paint('red', safe('edit: ' + p + ': ' + message(err))) + '\n'); return 1; }
      } else {
        var dir = await fs.stat(parentOf(path));
        if (!dir || !dir.directory) { write(paint('red', safe('edit: ' + display(parentOf(path)) + ': no such folder')) + '\n'); return 1; }
        if (!dir.writable) { write(paint('red', safe('edit: files cannot be created in ' + display(parentOf(path)) + ', which is read-only here')) + '\n'); return 1; }
      }
      var label = display(path);
      // Like nano, the buffer always ends in an empty line, so there is
      // somewhere to move to and paste after the last line of text.
      var lines = String(text).replace(/\r\n?/g, '\n').replace(/\t/g, '    ').split('\n');
      if (lines[lines.length - 1] !== '') lines.push('');
      function textLines() { return lines.length - 1; }
      var row = 0, col = 0, goal = 0, top = 0, left = 0, modified = false, exists = !!e;
      var note = e ? '' : 'New file', noteIsError = false, cut = [], lastWasCut = false, confirming = false, saving = false, searching = null, lastSearch = '';
      var KEYS = [['^S', 'Save'], ['^X', 'Exit'], ['^K', 'Cut'], ['^U', 'Paste'], ['^W', 'Find'], ['^G', 'Keys']];
      return new Promise(function (done) {
        var resizeSub = term.onResize(function () { draw(); });
        function say(m, isError) { note = m; noteIsError = !!isError; }
        function clampCol() { col = Math.min(col, lines[row].length); }
        function draw() {
          var rows = term.rows, cols = term.cols, textRows = Math.max(1, rows - 3);
          if (row < top) top = row;
          if (row >= top + textRows) top = row - textRows + 1;
          if (col < left) left = col;
          if (col >= left + cols) left = col - cols + 1;
          var title = ' edit  ' + label + (modified ? '  (modified)' : '');
          var out = '\x1b[?25l\x1b[H' + C.inverse + (safe(title) + ' '.repeat(cols)).slice(0, cols) + C.reset + '\r\n';
          for (var i = 0; i < textRows; i++) { var l = lines[top + i]; out += '\x1b[2K' + (l != null ? safe(l.slice(left, left + cols)) : paint('dim', '~')) + '\r\n'; }
          var shown = searching != null ? 'Find: ' + searching : note;
          out += '\x1b[2K' + (shown ? (noteIsError && searching == null ? paint('red', safe(shown).slice(0, cols)) : safe(shown).slice(0, cols)) : '') + '\r\n';
          var bar = '', used = 0;
          KEYS.forEach(function (k) { var piece = k[0] + ' ' + k[1] + '  '; if (used + piece.length > cols) return; bar += C.inverse + k[0] + C.reset + ' ' + k[1] + '  '; used += piece.length; });
          out += '\x1b[2K' + bar;
          out += searching != null ? '\x1b[' + (textRows + 2) + ';' + Math.min(cols, 7 + searching.length) + 'H\x1b[?25h' : '\x1b[' + (row - top + 2) + ';' + (col - left + 1) + 'H\x1b[?25h';
          term.write(out);
        }
        function insertText(s) {
          var parts = s.replace(/\r\n?/g, '\n').replace(/\t/g, '    ').replace(/[\x00-\x09\x0b-\x1f\x7f]/g, '').split('\n');
          var head = lines[row].slice(0, col), tail = lines[row].slice(col);
          if (parts.length === 1) { lines[row] = head + parts[0] + tail; col += parts[0].length; }
          else {
            var add = [head + parts[0]].concat(parts.slice(1, -1), [parts[parts.length - 1] + tail]);
            lines.splice.apply(lines, [row, 1].concat(add));
            row += parts.length - 1; col = parts[parts.length - 1].length;
          }
          goal = col; modified = true;
        }
        function find(needle, from) {
          if (!needle) return false;
          for (var n = 0; n < lines.length; n++) {
            var r = (row + n) % lines.length, start = n === 0 ? from : 0, at = lines[r].toLowerCase().indexOf(needle.toLowerCase(), start);
            if (at >= 0) { row = r; col = at; goal = col; return true; }
          }
          return false;
        }
        async function saveFile() {
          if (saving) return false;
          saving = true; say('Saving…'); draw();
          var content = lines.join('\n');
          if (content && content[content.length - 1] !== '\n') content += '\n';
          try {
            var item = await fs.write(path, content, exists ? 'expect' : 'create', revision);
            var read = await fs.read(path); revision = read.revision; exists = true; modified = false;
            label = display(item.path || path);
            say('Saved ' + label + ' (' + textLines() + (textLines() === 1 ? ' line)' : ' lines)'));
            saving = false; return true;
          } catch (err) { saving = false; say('Not saved: ' + message(err), true); return false; }
        }
        function exit() { resizeSub.dispose(); leaveAlt(); done(0); }
        async function key(k) {
          if (confirming) {
            if (k === 'y' || k === 'Y') { confirming = false; if (await saveFile()) { exit(); return; } }
            else if (k === 'n' || k === 'N') { exit(); return; }
            else if (k === '\x1b' || k === '\x03') { confirming = false; say('Still editing.'); }
            draw(); keyWaiter = key; return;
          }
          if (searching != null) {
            if (k === '\r') { lastSearch = searching || lastSearch; searching = null; if (!find(lastSearch, col + 1)) say('"' + lastSearch + '" was not found.'); else say(''); }
            else if (k === '\x1b' || k === '\x03' || k === '\x07') { searching = null; say('Cancelled.'); }
            else if (k === '\x7f' || k === '\b') searching = searching.slice(0, -1);
            else if (k >= ' ' && k.indexOf('\x1b') < 0) searching += k.replace(/[\x00-\x1f]/g, '');
            draw(); keyWaiter = key; return;
          }
          var cutting = false;
          if (k.length > 1 && k[0] !== '\x1b') insertText(k);
          else {
            switch (k) {
              case '\x13': case '\x0f': await saveFile(); break;
              case '\x18':
                if (!modified) { exit(); return; }
                confirming = true; say('Save changes to ' + label + '? y to save, n to discard, Esc to keep editing'); break;
              case '\x17': searching = ''; break;
              case '\x0b':
                cutting = true; if (!lastWasCut) cut = [];
                cut.push(lines[row]);
                if (lines.length > 1) { lines.splice(row, 1); if (row >= lines.length) row = lines.length - 1; } else lines[0] = '';
                col = 0; goal = 0; modified = true;
                say('Cut ' + cut.length + (cut.length === 1 ? ' line' : ' lines')); break;
              case '\x15':
                if (!cut.length) { say('Nothing has been cut yet.'); break; }
                lines.splice.apply(lines, [row, 0].concat(cut)); row += cut.length; col = 0; goal = 0; modified = true; break;
              case '\x07': say('Ctrl+S save, Ctrl+X exit, Ctrl+K cut line, Ctrl+U paste, Ctrl+W find, arrows/Home/End/PgUp/PgDn move'); break;
              case '\x03': say('Line ' + (row + 1) + ' of ' + textLines() + ', column ' + (col + 1)); break;
              case '\r': insertText('\n'); break;
              case '\t': insertText('    '.slice(col % 4)); break;
              case '\x7f': case '\b':
                if (col > 0) { lines[row] = lines[row].slice(0, col - 1) + lines[row].slice(col); col--; modified = true; }
                else if (row > 0) { col = lines[row - 1].length; lines[row - 1] += lines[row]; lines.splice(row, 1); row--; modified = true; }
                goal = col; break;
              case '\x1b[3~':
                if (col < lines[row].length) { lines[row] = lines[row].slice(0, col) + lines[row].slice(col + 1); modified = true; }
                else if (row < lines.length - 1) { lines[row] += lines[row + 1]; lines.splice(row + 1, 1); modified = true; }
                break;
              case '\x1b[D': if (col > 0) col--; else if (row > 0) { row--; col = lines[row].length; } goal = col; break;
              case '\x1b[C': if (col < lines[row].length) col++; else if (row < lines.length - 1) { row++; col = 0; } goal = col; break;
              case '\x1b[A': if (row > 0) { row--; col = goal; clampCol(); } break;
              case '\x1b[B': if (row < lines.length - 1) { row++; col = goal; clampCol(); } break;
              case '\x1b[H': case '\x1bOH': case '\x01': col = 0; goal = 0; break;
              case '\x1b[F': case '\x1bOF': case '\x05': col = lines[row].length; goal = col; break;
              case '\x1b[5~': row = Math.max(0, row - Math.max(1, term.rows - 4)); col = goal; clampCol(); break;
              case '\x1b[6~': row = Math.min(lines.length - 1, row + Math.max(1, term.rows - 4)); col = goal; clampCol(); break;
              default: if (k >= ' ' && k.indexOf('\x1b') < 0) insertText(k);
            }
          }
          lastWasCut = cutting;
          if (lines[lines.length - 1] !== '') lines.push('');
          if (!confirming && ['\x13', '\x0f', '\x07', '\x03', '\x0b', '\x15', '\x17'].indexOf(k) < 0 && note && !noteIsError && note !== 'New file') say('');
          draw(); keyWaiter = key;
        }
        enterAlt(); draw(); keyWaiter = key;
      });
    }

    /* --- The prompt ---------------------------------------------------------- */
    async function submit() {
      if (busy || !term) return;
      var input = line; busy = true; var token = run = { cancelled: false }; root.dataset.terminalBusy = 'true'; var id = ++generation; cursor = line.length; await redraw();
      if (!current(id)) return;
      term.write('\r\n'); output = (output + strip(prompt()) + input + '\n').slice(-MAX_OUTPUT); save();
      if (input.trim() && history[history.length - 1] !== input) history.push(input);
      history = history.slice(-MAX_HISTORY);
      if (input.trim()) {
        relativeTargets.clear();
        try {
          var status = await exitSensitiveCommand(function () { return executeLine(input, null, token); });
          if (status !== EXIT && !token.cancelled) lastStatus = status;
        } catch (error) { if (current(id) && !token.cancelled) { write(paint('red', safe('error: ' + message(error))) + '\n'); lastStatus = 1; } }
      }
      if (!current(id) || destroyed) return;
      if (altScreen) leaveAlt();
      keyWaiter = null; busy = false; newPrompt(); save();
      var pending = typeahead; typeahead = [];
      pending.forEach(onData);
    }
    /* Completion, Parks Computing's behaviour over asynchronous listings:
       the first word from the commands, tools and scripts in ~/bin, later
       words from the command's kind of argument. */
    async function complete() {
      var request = ++completionRequest, original = line, position = cursor, id = generation;
      function valid() { return current(id) && request === completionRequest && line === original && cursor === position && !busy; }
      var before = line.slice(0, cursor), start = 0, quote = null;
      for (var i = 0; i < before.length; i++) {
        var ch = before[i];
        if (ch === '\\' && i + 1 < before.length) { i++; continue; }
        if (quote) { if (ch === quote) quote = null; }
        else if (ch === '"' || ch === "'") quote = ch;
        else if (/[\s|;&<>]/.test(ch)) start = i + 1;
      }
      var head = before.slice(0, start), raw = before.slice(start);
      var segment = head.split(/\|\||&&|[|;]/).pop(), parsed = parse(segment), token = parse(raw + (quote || ''));
      if (parsed.error || token.error) return;
      var lastList = parsed.list[parsed.list.length - 1], stage = lastList ? lastList.stages[lastList.stages.length - 1] : [];
      var words = stage.map(function (w) { return w && typeof w === 'object' ? (w.glob != null ? w.word : '>') : w; });
      var partial = token.list.length ? token.list[0].stages[0][0] || '' : '';
      if (typeof partial !== 'string') partial = partial.word;
      var list = [], redirecting = words[words.length - 1] === '>';
      if (!words.length && !redirecting) {
        var names = Object.keys(COMMANDS).concat(tools().map(function (t) { return t.name; }));
        var dirs = binDirs();
        for (var d = 0; d < dirs.length; d++) {
          try { (await fs.list(dirs[d])).forEach(function (e) { if (!e.directory) names.push(nameOf(e.path).replace(/\.(sh|lua)$/, '')); }); } catch (err) { /* There is no such folder. */ }
        }
        if (!valid()) return;
        list = Array.from(new Set(names)).filter(function (n) { return n.startsWith(partial); }).sort().map(function (n) { return { word: n, done: true }; });
      } else {
        var c = COMMANDS[words[0]], mode = redirecting ? 'file' : c ? c.complete : 'path';
        if (mode === 'command') list = Object.keys(COMMANDS).filter(function (k) { return k.startsWith(partial); }).map(function (k) { return { word: k, done: true }; });
        if (mode === 'path' || mode === 'file' || mode === 'dir') {
          if (partial === '~') list = [{ word: '~/', done: false }];
          else {
            if (!await verifyCwd(id) || !valid()) return;
            var slash = partial.lastIndexOf('/'), dirPart = slash < 0 ? '' : partial.slice(0, slash + 1), baseName = partial.slice(slash + 1);
            var entries;
            try { entries = await fs.list(abs(dirPart || '.')); } catch (err) { return; }
            if (!valid()) return;
            entries.forEach(function (e) {
              var name = nameOf(e.path);
              if (name.startsWith(baseName) && (mode !== 'dir' || e.directory) && (baseName[0] === '.' || name[0] !== '.')) list.push({ word: dirPart + name + (e.directory ? '/' : ''), done: !e.directory });
            });
          }
        }
      }
      if (!valid() || !list.length) return;
      function replace(word, done) {
        var value = /[\s"'\\*?[\]$;|&<>]/.test(word) ? '"' + word.replace(/[\\"$]/g, '\\$&') + '"' : word;
        value += done ? ' ' : '';
        line = head + value + original.slice(position); cursor = head.length + value.length; redraw();
      }
      if (list.length === 1) { replace(list[0].word, list[0].done); return; }
      var common = list.reduce(function (value, entry) { var n = 0; while (n < value.length && value[n] === entry.word[n]) n++; return value.slice(0, n); }, list[0].word);
      if (common.length > partial.length) { replace(common, false); return; }
      var now = Date.now();
      if (now - lastTab < 800) { write('\n' + columns(list.map(function (entry) { return safe(entry.word.replace(/.*\/(?=.)/, '')); }), term.cols) + '\n'); redraw(); }
      lastTab = now;
    }
    function onData(data) {
      if (keyWaiter) { var k = keyWaiter; keyWaiter = null; k(data); return; }
      if (data !== '\t') completionRequest++;
      if (busy) {
        if (data === '\x03') {
          // A line stopped with Ctrl+C ends with status 130, as in a Unix shell.
          run.cancelled = true; typeahead = []; lastStatus = 130;
          var cancelled = ++generation;
          visualTail = visualTail.then(function () { if (current(cancelled)) { if (altScreen) leaveAlt(); keyWaiter = null; term.write('^C\r\n'); busy = false; newPrompt(); } });
        } else typeahead.push(data);
        return;
      }
      if (data === '\x03') { term.write('^C\r\n'); newPrompt(); return; }
      // Pasted text arrives in one piece; only its first line is taken, and
      // it is not run.
      if (data.length > 1 && data[0] !== '\x1b') { insert(data.replace(/\r\n?/g, '\n').split('\n')[0]); return; }
      if (data === '\r') { submit(); return; }
      if (data === '\x7f' || data === '\b') { if (cursor) { line = line.slice(0, cursor - 1) + line.slice(cursor); cursor--; } }
      else if (data === '\x1b[3~') line = line.slice(0, cursor) + line.slice(cursor + 1);
      else if (data === '\x1b[D') cursor = Math.max(0, cursor - 1);
      else if (data === '\x1b[C') cursor = Math.min(line.length, cursor + 1);
      else if (data === '\x01' || data === '\x1b[H' || data === '\x1bOH') cursor = 0;
      else if (data === '\x05' || data === '\x1b[F' || data === '\x1bOF') cursor = line.length;
      else if (data === '\x15') { line = line.slice(cursor); cursor = 0; }
      else if (data === '\x0b') line = line.slice(0, cursor);
      else if (data === '\x17') { var before = line.slice(0, cursor).replace(/\S+\s*$/, ''); line = before + line.slice(cursor); cursor = before.length; }
      else if (data === '\x0c') { term.clear(); term.write('\x1b[2J\x1b[H'); }
      else if (data === '\x1b[A' || data === '\x1b[B') { if (histPos === history.length) draft = line; histPos = Math.max(0, Math.min(history.length, histPos + (data === '\x1b[A' ? -1 : 1))); line = histPos === history.length ? draft : history[histPos]; cursor = line.length; }
      else if (data === '\t') { var completionLine = line, completionCursor = cursor, completionGeneration = generation; complete().catch(function (error) { if (current(completionGeneration) && !busy && line === completionLine && cursor === completionCursor) { write('\n' + paint('red', safe('error: ' + message(error))) + '\n'); redraw(); } }); return; }
      else if (!data.startsWith('\x1b')) { insert(data); return; }
      redraw();
    }
    function resize() { if (fit && !destroyed && screen.clientWidth && screen.clientHeight) { fit.fit(); if (!busy && root.dataset.terminalReady === 'true') redraw(); } }
    function applyTheme() { if (!term || destroyed) return; term.options.theme = theme(root); if (window.dinahAppearance) { var appearance = dinahAppearance.content('terminal'); term.options.fontFamily = appearance.font; if (!explicitFontSize) { fontSize = appearance.size; term.options.fontSize = fontSize; } } if (window.dinahRender) window.dinahRender.enqueue(applyTheme, root, resize, null, 'terminal appearance'); else resize(); }
    async function setState(state) {
      opts.state = state || '';
      if (!term) return;
      var value = new URLSearchParams(opts.state).get('cwd'); if (!value || value === cwd) return;
      var id = ++generation; busy = true; root.dataset.terminalBusy = 'true';
      try {
        var resource = await app().StatWorkspaceMount(resolve('/', value, home));
        if (!current(id)) return;
        if (!resource.directory) throw new Error('not a folder');
        chdir(resource);
      } catch (error) { if (current(id)) write('\n' + paint('red', safe('error: ' + message(error))) + '\n'); }
      if (current(id)) { line = ''; cursor = 0; histPos = history.length; draft = ''; await redraw(); if (current(id)) { busy = false; root.dataset.terminalBusy = 'false'; } }
    }
    /* The terminal is opened once its screen has a size. xterm measures its
       characters against the screen, caches a width only when it is more
       than nothing, and measures again for every character drawn until it
       is; a terminal opened in a minimised window measured thousands of
       times, each a layout of the whole page. */
    function sized() {
      return new Promise(function (resolve) {
        if (screen.clientWidth && screen.clientHeight) { resolve(); return; }
        var watcher = new ResizeObserver(function () {
          if (destroyed || (screen.clientWidth && screen.clientHeight)) { watcher.disconnect(); resolve(); }
        });
        watcher.observe(screen);
      });
    }
    var ready = Promise.all([assets(), window.dinahUIState ? dinahUIState.ready : Promise.resolve(), window.dinahAppearance ? dinahAppearance.ready : Promise.resolve()]);
    ready.then(sized).then(async function () {
      if (destroyed) return;
      // The home folder's path is the person's: /home/<name>.
      try { var homeItem = await app().StatWorkspaceMount('~'); if (homeItem && homeItem.path) { home = homeItem.path; user = nameOf(home); homeRef = homeItem.ref || null; } } catch (error) { /* Home is unavailable; ~ keeps its default. */ }
      if (destroyed) return;
      var saved = window.dinahUIState && item.instanceId ? dinahUIState.get(key, {}) : {};
      try { cwd = resolve('/', new URLSearchParams(opts.state || '').get('cwd') || saved.cwd || item.path || home, home); } catch (error) { cwd = home; }
      if (saved.homeRef && saved.homeRef.sourceId && !homeRef) homeRef = saved.homeRef;
      cwdRef = saved.cwd === cwd && saved.ref && typeof saved.ref === 'object' ? saved.ref : item.initialRef || (cwd === home ? homeRef : null);
      history = Array.isArray(saved.history) ? saved.history.filter(function (value) { return typeof value === 'string'; }).slice(-MAX_HISTORY).map(function (value) { return safe(value).replace(/[\n\t]/g, ' ').slice(0, MAX_INPUT); }) : [];
      env = saved.env && typeof saved.env === 'object' ? Object.keys(saved.env).filter(function (n) { return /^[A-Za-z_][A-Za-z0-9_]*$/.test(n) && FIXED.indexOf(n) < 0 && typeof saved.env[n] === 'string'; }).reduce(function (o, n) { o[n] = saved.env[n]; return o; }, {}) : {};
      output = String(saved.output || '').slice(-MAX_OUTPUT); explicitFontSize = Number.isInteger(saved.fontSize) && saved.fontSize >= 6 && saved.fontSize <= 22; fontSize = explicitFontSize ? saved.fontSize : (window.dinahAppearance ? dinahAppearance.content('terminal').size : 14);
      term = new window.Terminal({ cursorBlink: true, fontFamily: getComputedStyle(root).getPropertyValue('--mono').trim() || 'Consolas, "Courier New", monospace', fontSize: fontSize, theme: theme(root), scrollback: 2000, minimumContrastRatio: 4.5, allowProposedApi: false, screenReaderMode: true });
      fit = new window.FitAddon.FitAddon(); term.loadAddon(fit); term.open(screen); resize(); term.onData(onData);
      // A key the page has already taken, as the menu bar takes Alt alone,
      // F10 and Alt with a title's letter in the capture phase, is not the
      // shell's too: xterm's custom key handler skips it.
      term.attachCustomKeyEventHandler(function (e) { return !e.defaultPrevented; });
      term.textarea.setAttribute('aria-label', 'Terminal Command');
      // The saved transcript is the terminal's own text and what it showed,
      // which was made safe when it was first written.
      if (output) term.write(output.replace(/\n/g, '\r\n')); else write(paint('bold', productName() + ' Terminal.') + ' Type ' + paint('green', 'help') + ' for the commands; your own folder is ' + paint('blue', '~') + '.\n\n');
      // Capture the owner before a source can be removed and its alias reused.
      var initialGeneration = generation;
      busy = true;
      try { await verifyCwd(initialGeneration); } catch (error) { if (current(initialGeneration)) write(paint('red', safe('error: ' + message(error))) + '\n'); }
      if (!current(initialGeneration)) return;
      /* ~/.profile, when there is one, is sourced as the terminal starts, so
         what it exports and the folder it goes to are this terminal's own.
         Ctrl+C stops it. */
      var profileToken = run = { cancelled: false };
      try {
        var profile = await fs.stat(home + '/.profile');
        if (profile && !profile.directory && current(initialGeneration)) await exitSensitiveCommand(function () { return sourceScript(profile.path, [], makeIo(null, null, null, profileToken), 0); });
      } catch (error) { if (current(initialGeneration)) write(paint('red', safe('.profile: ' + message(error))) + '\n'); }
      if (!current(initialGeneration)) return;
      busy = false; newPrompt(); announce(); root.dataset.terminalReady = 'true';
      observer = new ResizeObserver(resize); observer.observe(screen); document.addEventListener('pudl:theme-change', applyTheme); document.addEventListener('dinah:appearance-change', applyTheme); applyTheme();
      if (window.dinahResources && dinahResources.subscribe) unsubscribe = dinahResources.subscribe(function (event) {
        fs.clear();
        var id = generation;
        // The home folder can be renamed; the Terminal follows it.
        var renamed = event && event.kind === 'home-renamed' ? app().StatWorkspaceMount('~').then(function (item) { if (item && item.path) { home = item.path; user = nameOf(home); homeRef = item.ref || homeRef; } }).catch(function () {}) : Promise.resolve();
        renamed.then(function () {
          if (busy || !current(id)) return;
          verifyCwd(id).then(function (valid) { if (valid && !busy) redraw(); }).catch(function () {});
        });
      });
      if (root.closest('.win.active')) term.focus();
    }).catch(function (error) { if (!destroyed) { screen.textContent = 'The terminal could not start: ' + message(error); screen.classList.add('pc-terminal-failed'); } });
    return {
      ready: ready,
      state: function () { return 'cwd=' + encodeURIComponent(cwd || home); },
      setState: setState,
      commands: function () { return [{ label: 'Reset Text Size', run: function () { explicitFontSize = false; if (!window.dinahAppearance && term) { fontSize = 14; term.options.fontSize = fontSize; } applyTheme(); save(); } }, { label: 'Clear Terminal', run: function () { if (!busy && term) { output = ''; term.clear(); term.write('\x1b[2J\x1b[H'); redraw(); save(); } } }, { label: 'Larger Text', run: function () { if (term) { explicitFontSize = true; fontSize = Math.min(22, fontSize + 1); term.options.fontSize = fontSize; resize(); save(); } } }, { label: 'Smaller Text', run: function () { if (term) { explicitFontSize = true; fontSize = Math.max(6, fontSize - 1); term.options.fontSize = fontSize; resize(); save(); } } }]; },
      resize: resize,
      destroy: function () { destroyed = true; ++generation; run.cancelled = true; if (unsubscribe) unsubscribe(); if (observer) observer.disconnect(); document.removeEventListener('pudl:theme-change', applyTheme); document.removeEventListener('dinah:appearance-change', applyTheme); if (window.dinahRender) window.dinahRender.cancel(applyTheme); if (term) term.dispose(); }
    };
  }
  window.dinahTerminal = { init: init, parse: parse, resolve: resolve, safe: safe, commands: function () { return Object.keys(COMMANDS).sort(); },
    // xterm and its theme, which a Terminal running a real shell shares
    // (terminal-shell.js).
    assets: assets, theme: theme,
    registerInterpreter: registerInterpreter, interpreters: function () { return INTERPRETERS.map(function (d) { return { id: d.id, name: d.name, command: d.command }; }); } };
  // Interpreters loaded before the Terminal wait in this list.
  (window.dinahTerminalInterpreters || []).forEach(registerInterpreter);
  /* The Terminal's keys, for Help › Keyboard Shortcuts: the command line's
     editing, the file editor's (edit) and the pager's (more). */
  if (window.dinahCommands && window.dinahCommands.addHelp) {
    window.dinahCommands.addHelp('Terminal', [
      { label: 'Start or End of the Line', key: 'Ctrl+A / Ctrl+E, or Home / End' },
      { label: 'Delete to the Start or End', key: 'Ctrl+U / Ctrl+K' },
      { label: 'Delete the Word Before the Cursor', key: 'Ctrl+W' },
      { label: 'Clear the Screen', key: 'Ctrl+L' },
      { label: 'Stop the Command or Drop the Line', key: 'Ctrl+C' },
      { label: 'An Earlier or Later Command', key: 'Up / Down' },
      { label: 'Complete the Name', key: 'Tab' },
      { where: 'Editing a File', label: 'Save', key: 'Ctrl+S or Ctrl+O' },
      { where: 'Editing a File', label: 'Leave the Editor', key: 'Ctrl+X' },
      { where: 'Editing a File', label: 'Cut the Line, Then Paste It', key: 'Ctrl+K, then Ctrl+U' },
      { where: 'Editing a File', label: 'Find', key: 'Ctrl+W' },
      { where: 'Editing a File', label: 'Where the Cursor Is', key: 'Ctrl+C' },
      { where: 'Editing a File', label: 'The Editor\'s Keys', key: 'Ctrl+G' },
      { where: 'Reading Long Output', label: 'Next or Previous Page', key: 'Space or PgDn / b or PgUp' },
      { where: 'Reading Long Output', label: 'Next or Previous Line', key: 'Down, j or Enter / Up or k' },
      { where: 'Reading Long Output', label: 'Top or Bottom', key: 'g or Home / G or End' },
      { where: 'Reading Long Output', label: 'Stop Reading', key: 'q or Esc' }]);
  }
})();
