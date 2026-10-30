/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
   Licensed under the Apache License, Version 2.0; see
   https://www.apache.org/licenses/LICENSE-2.0 */

/* Text tools for the terminal: head, tail, wc, sort and uniq. Each is a
   JavaScript command registered through window.pcTerminal.register, the
   way any command beyond the built-in ones is added
   (Architecture/terminal-design.md). The terminal loads this file because
   js/applets.js lists it in window.pcTerminalCommands. A command reaches
   only the io object it is given. */
(function () {
    'use strict';
    var T = window.pcTerminal;
    if (!T) { return; }

    /* The text a command works on: its files, one after another, or what
       was piped into it. Resolves to null after reporting a problem. */
    async function input(files, io, name) {
        if (!files.length) {
            if (io.stdin != null) { return io.stdin; }
            io.err(name + ': which file? (or pipe text into it)');
            return null;
        }
        var parts = [];
        for (var i = 0; i < files.length; i++) {
            try { parts.push(await io.read(files[i])); }
            catch (err) { io.err(name + ': ' + err.message); return null; }
        }
        return parts.join('');
    }

    function linesOf(text) {
        if (!text) { return []; }
        return text.replace(/\r/g, '').replace(/\n$/, '').split('\n');
    }

    /* Reads -n N, -N or -nN, and returns the count and the other words. */
    function countOption(args, fallback, name, io) {
        var n = fallback, rest = [];
        for (var i = 0; i < args.length; i++) {
            var a = args[i], m;
            if (a === '-n') { n = args[++i]; }
            else if ((m = /^-n?(\d+)$/.exec(a))) { n = m[1]; }
            else { rest.push(a); continue; }
            if (!/^\d+$/.test(String(n))) { io.err(name + ': -n needs a number'); return null; }
        }
        return { n: +n, rest: rest };
    }

    T.register({
        name: 'head',
        summary: 'show the first lines of text',
        help: 'head [-n count] [file...]\n\nPrints the first lines, ten unless you say otherwise, of its files or of what is piped into it.\n\nExample: ls -l /site/articles | head -n 5',
        complete: 'file',
        run: async function (args, io) {
            var o = countOption(args, 10, 'head', io);
            if (!o) { return 2; }
            var text = await input(o.rest, io, 'head');
            if (text == null) { return 1; }
            linesOf(text).slice(0, o.n).forEach(function (l) { io.out(l + '\n'); });
        }
    });

    T.register({
        name: 'tail',
        summary: 'show the last lines of text',
        help: 'tail [-n count] [file...]\n\nPrints the last lines, ten unless you say otherwise, of its files or of what is piped into it.',
        complete: 'file',
        run: async function (args, io) {
            var o = countOption(args, 10, 'tail', io);
            if (!o) { return 2; }
            var text = await input(o.rest, io, 'tail');
            if (text == null) { return 1; }
            var lines = linesOf(text);
            lines.slice(Math.max(0, lines.length - o.n)).forEach(function (l) { io.out(l + '\n'); });
        }
    });

    T.register({
        name: 'wc',
        summary: 'count lines, words and characters',
        help: 'wc [-l] [-w] [-c] [file...]\n\nCounts the lines, words and characters of its files or of what is piped into it.\n\n  -l   lines only\n  -w   words only\n  -c   characters only\n\nExample: ls /site/articles | wc -l',
        complete: 'file',
        run: async function (args, io) {
            var flags = '', files = [];
            args.forEach(function (a) { if (/^-[lwc]+$/.test(a)) { flags += a.slice(1); } else { files.push(a); } });
            if (!flags) { flags = 'lwc'; }
            var sets = files.length ? files.map(function (f) { return [f]; }) : [[]];
            var totals = [0, 0, 0];
            for (var i = 0; i < sets.length; i++) {
                var text = await input(sets[i], io, 'wc');
                if (text == null) { return 1; }
                var counts = [linesOf(text).length, (text.match(/\S+/g) || []).length, text.length];
                counts.forEach(function (c, k) { totals[k] += c; });
                io.out(format(counts) + (sets[i][0] ? ' ' + sets[i][0] : '') + '\n');
            }
            if (sets.length > 1) { io.out(format(totals) + ' total\n'); }
            function format(c) {
                return ['l', 'w', 'c'].filter(function (f) { return flags.indexOf(f) >= 0; })
                    .map(function (f) { return String(c['lwc'.indexOf(f)]).padStart(7); }).join(' ');
            }
        }
    });

    T.register({
        name: 'sort',
        summary: 'sort lines of text',
        help: 'sort [-r] [-n] [-u] [file...]\n\nSorts the lines of its files or of what is piped into it.\n\n  -r   reverse the order\n  -n   compare numbers at the start of each line as numbers\n  -u   keep one of each line',
        complete: 'file',
        run: async function (args, io) {
            var flags = '', files = [];
            args.forEach(function (a) { if (/^-[rnu]+$/.test(a)) { flags += a.slice(1); } else { files.push(a); } });
            var text = await input(files, io, 'sort');
            if (text == null) { return 1; }
            var lines = linesOf(text);
            var numeric = flags.indexOf('n') >= 0;
            lines.sort(function (a, b) {
                if (numeric) {
                    var d = (parseFloat(a) || 0) - (parseFloat(b) || 0);
                    if (d) { return d; }
                }
                return a < b ? -1 : a > b ? 1 : 0;
            });
            if (flags.indexOf('r') >= 0) { lines.reverse(); }
            if (flags.indexOf('u') >= 0) { lines = lines.filter(function (l, i) { return i === 0 || l !== lines[i - 1]; }); }
            lines.forEach(function (l) { io.out(l + '\n'); });
        }
    });

    T.register({
        name: 'uniq',
        summary: 'drop repeated lines',
        help: 'uniq [-c] [file...]\n\nDrops a line that repeats the one before it, so sort it first to drop every repeat.\n\n  -c   count how many times each line came\n\nExample: cat ~/notes | sort | uniq -c',
        complete: 'file',
        run: async function (args, io) {
            var count = false, files = [];
            args.forEach(function (a) { if (a === '-c') { count = true; } else { files.push(a); } });
            var text = await input(files, io, 'uniq');
            if (text == null) { return 1; }
            var lines = linesOf(text), prev = null, n = 0;
            function flush() { if (prev != null) { io.out((count ? String(n).padStart(7) + ' ' : '') + prev + '\n'); } }
            lines.forEach(function (l) {
                if (l === prev) { n++; return; }
                flush();
                prev = l; n = 1;
            });
            flush();
        }
    });
})();
