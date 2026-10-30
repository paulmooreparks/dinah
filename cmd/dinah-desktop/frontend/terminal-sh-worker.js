/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0
   The worker a POSIX shell script runs in (terminal-sh.js): Go's runtime
   for WebAssembly, and the shell built from tools/shwasm, which sets
   dinahShRun. Its requests to the Terminal go out as "call" messages and come
   back as "reply" messages; its output goes out as it is written. */
'use strict';
importScripts('vendor/gosh/wasm_exec.js');

var started = null, calls = {}, next = 0;
function start(url) {
  if (started) return started;
  var go = new Go();
  started = fetch(url).then(function (r) {
    if (!r.ok) throw new Error('the shell could not be loaded');
    return r.arrayBuffer();
  }).then(function (bytes) {
    return WebAssembly.instantiate(bytes, go.importObject);
  }).then(function (result) {
    // The shell's main sets dinahShRun and then waits, so run returns at once.
    go.run(result.instance);
    if (typeof self.dinahShRun !== 'function') throw new Error('the shell did not start');
  });
  return started;
}
function host(op, args) {
  return new Promise(function (resolve, reject) {
    var id = ++next;
    calls[id] = { resolve: resolve, reject: reject };
    postMessage({ type: 'call', id: id, op: op, args: Array.from(args || []) });
  });
}
function write(stream, text) { postMessage({ type: stream === 'err' ? 'err' : 'out', text: String(text) }); }

onmessage = function (e) {
  var m = e.data;
  if (m.type === 'reply') {
    var call = calls[m.id];
    delete calls[m.id];
    if (!call) return;
    if (m.ok) call.resolve(m.value); else call.reject(new Error(m.value));
    return;
  }
  if (m.type !== 'run') return;
  start(m.wasm).then(function () {
    return self.dinahShRun({ source: m.source, name: m.name, args: m.args || [], env: m.env || {}, cwd: m.cwd, stdin: m.stdin == null ? null : String(m.stdin) }, host, write);
  }).then(function (result) {
    postMessage({ type: 'done', status: result.status, env: result.env, cwd: result.cwd });
  }, function (err) {
    write('err', m.name + ': ' + (err && err.message || err) + '\n');
    postMessage({ type: 'done', status: 1 });
  });
};
