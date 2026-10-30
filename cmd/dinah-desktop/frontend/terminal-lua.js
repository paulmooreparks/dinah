/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
   Lua 5.4 for the Terminal: an
   interpreter module, which the Terminal runs a script with when its name
   ends in .lua or its first line is a #! line that ends in lua, and which
   gives the Terminal its lua command. Leaving this file out of the page
   leaves Lua out and changes nothing else.

   A script runs in a worker of its own (terminal-lua-worker.js), so that a
   script that never ends cannot hold up the page, and Ctrl+C ends the
   worker. Everything the script reaches outside itself is a request the
   worker sends and the Terminal's host answers. */
(function () {
  'use strict';
  var base = new URL('.', document.currentScript.src);
  var WASM = new URL('vendor/wasmoon-1.16.0/glue.wasm', base).href;
  const message = window.dinahHelpers.message;

  var lua = {
    id: 'lua',
    name: 'Lua 5.4',
    command: 'lua',
    summary: 'run a Lua script, or a line of Lua',
    help: 'lua <file> [arguments...]\nlua -e <code> [arguments...]\n\nRuns Lua 5.4. print, io.write, io.read and io.lines use the Terminal\'s output and input, so a Lua script takes part in a pipeline like any command; io.open, os.remove and os.rename use your files; io.popen and os.execute run a Terminal command; os.getenv reads its variables, and arg holds the script\'s name and arguments. require finds a module in the script\'s folder, in ~/lib/lua and in /lib/lua; require "fs" lists and inspects folders, and require "applets" calls the applets\' actions, with tables for their input and output. A script also runs by its name from ~/bin or /bin, as a file ending in .lua or whose first line is #!/bin/lua. Ctrl+C stops it. A file function cannot be used inside a coroutine the script makes itself.',
    matches: function (path, text) { return /\.lua$/i.test(path) || /^#![^\n]*\blua[ \t]*(\r?\n|$)/.test(String(text)); },
    run: function (source, ctx) {
      return new Promise(function (finish) {
        var worker, ended = false, poll;
        function end(status) { if (ended) return; ended = true; clearInterval(poll); if (worker) worker.terminate(); finish(status); }
        try { worker = new Worker(new URL('terminal-lua-worker.js', base)); } catch (err) { ctx.error(ctx.name + ': Lua could not start: ' + message(err)); finish(1); return; }
        poll = setInterval(function () { if (ctx.interrupted()) end(130); }, 100);
        worker.onerror = function (e) { ctx.error(ctx.name + ': Lua could not start' + (e && e.message ? ': ' + e.message : '')); end(1); };
        worker.onmessage = async function (e) {
          var m = e.data;
          if (ended) return;
          if (m.type === 'out') { ctx.write(m.text); return; }
          if (m.type === 'err') { ctx.error(m.text); return; }
          if (m.type === 'done') { end(m.status); return; }
          if (m.type !== 'call') return;
          var reply;
          try { reply = { type: 'reply', id: m.id, ok: true, value: await ctx.request(m.op, m.args || []) }; }
          catch (err) { reply = { type: 'reply', id: m.id, ok: false, value: message(err) }; }
          if (!ended) worker.postMessage(reply);
        };
        worker.postMessage({ type: 'run', source: source, name: ctx.name, args: ctx.args, stdin: ctx.stdin, wasm: WASM });
      });
    }
  };
  if (window.dinahTerminal && window.dinahTerminal.registerInterpreter) window.dinahTerminal.registerInterpreter(lua);
  else (window.dinahTerminalInterpreters = window.dinahTerminalInterpreters || []).push(lua);
})();
