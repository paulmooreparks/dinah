/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
   A POSIX shell for the Terminal: an
   interpreter module, which the Terminal runs a script with when its name
   ends in .sh or its first line is a #! line for sh or bash, and which
   gives the Terminal its sh command. Leaving this file out of the page
   leaves the Terminal's own sh, which runs a file of its command lines.

   The shell is mvdan.cc/sh's interpreter, built for WebAssembly from
   tools/shwasm and run in a worker of its own (terminal-sh-worker.js), so a
   script that never ends cannot hold up the page, and Ctrl+C ends the
   worker. A command that is not the shell's own, and every file the script
   opens, is a request the Terminal's host answers. */
(function () {
  'use strict';
  var base = new URL('.', document.currentScript.src);
  var WASM = new URL('vendor/gosh/sh.wasm', base).href;
  const message = window.dinahHelpers.message;
  function envObject(pairs) {
    var out = {};
    (pairs || []).forEach(function (pair) { if (/^[A-Za-z_][A-Za-z0-9_]*$/.test(pair[0])) out[pair[0]] = String(pair[1]); });
    return out;
  }

  var sh = {
    id: 'sh',
    name: 'POSIX shell',
    command: 'sh',
    inline: '-c',
    inlineName: true,
    summary: 'run a shell script, or a line of shell',
    help: 'sh <file> [arguments...]\nsh -c <commands> [arguments...]\n\nRuns a POSIX shell script, as sh and bash do: variables, quoting, if, for, while, case, functions, pipelines, redirection, $( ) and $(( )), with the common Bash additions. cd, export, read, test and the other shell builtins are its own; any other command is a Terminal command, a tool or a script, found in the folders of PATH, which is ~/bin and then /bin unless you change it. Files are your files and workspaces, as at the prompt. A script also runs by its name from a folder in PATH, as a file ending in .sh or whose first line is #!/bin/sh or #!/bin/bash. A script runs on its own, so cd and export in it change nothing at the prompt; "source <file>" runs one in the Terminal itself. Ctrl+C stops it. The shell is mvdan.cc/sh (BSD-3-Clause).',
    // A .profile is a shell script too, as on Unix.
    matches: function (path, text) { return /\.sh$/i.test(path) || /(^|\/)\.profile$/.test(path) || /^#![^\n]*\b(sh|bash)[ \t]*(\r?\n|$)/.test(String(text)); },
    /* Resolves to the script's status, the variables it exported and the
       folder it ended in, which "source" takes into the Terminal. */
    run: function (source, ctx) {
      return new Promise(function (finish) {
        var worker, ended = false, poll;
        function end(result) { if (ended) return; ended = true; clearInterval(poll); if (worker) worker.terminate(); finish(result); }
        try { worker = new Worker(new URL('terminal-sh-worker.js', base)); } catch (err) { ctx.error(ctx.name + ': the shell could not start: ' + message(err)); finish(1); return; }
        poll = setInterval(function () { if (ctx.interrupted()) end(130); }, 100);
        worker.onerror = function (e) { ctx.error(ctx.name + ': the shell could not start' + (e && e.message ? ': ' + e.message : '')); end(1); };
        worker.onmessage = async function (e) {
          var m = e.data;
          if (ended) return;
          if (m.type === 'out') { ctx.write(m.text); return; }
          if (m.type === 'err') { ctx.error(m.text); return; }
          if (m.type === 'done') { end({ status: m.status, env: m.env || null, cwd: m.cwd || null }); return; }
          if (m.type !== 'call') return;
          var reply;
          try { reply = { type: 'reply', id: m.id, ok: true, value: await ctx.request(m.op, m.args || []) }; }
          catch (err) { reply = { type: 'reply', id: m.id, ok: false, value: message(err) }; }
          if (!ended) worker.postMessage(reply);
        };
        worker.postMessage({ type: 'run', source: source, name: ctx.name, args: ctx.args, stdin: ctx.stdin, cwd: ctx.cwd || ctx.dir, env: envObject(ctx.env), wasm: WASM });
      });
    }
  };
  if (window.dinahTerminal && window.dinahTerminal.registerInterpreter) window.dinahTerminal.registerInterpreter(sh);
  else (window.dinahTerminalInterpreters = window.dinahTerminalInterpreters || []).push(sh);
})();
