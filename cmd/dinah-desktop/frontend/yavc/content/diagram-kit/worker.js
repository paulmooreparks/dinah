/* Copyright 2026 Paul Parks. SPDX-License-Identifier: Apache-2.0 */
/* The D2 engine's worker (docs/diagrams-design.md, decision 3): yavc-d2.wasm
   (cmd/yavc-d2), run apart from the page so that a large layout never
   freezes it. The engine serves this script with a policy of its own
   (internal/web, workerPolicy), which lets D2 run its layout engines'
   JavaScript through eval and lets the worker reach no host at all. So
   everything it needs arrives with the first message: the engine, already
   compiled by the page, and the address of the Go runtime's loader, one of
   this origin's scripts, which the policy lets it load. D2's ELK layout is
   not in this build (docs/diagrams-design.md, open question 7).

     in:  { type: 'init', engine: WebAssembly.Module, scripts: { go } }
          { type: 'call', id, name, arg }
     out: { type: 'ready' } or { type: 'failed', error }
          { type: 'result', id, out }, out being the engine's JSON text */
'use strict';
var api = null;

function failed(err) { self.postMessage({ type: 'failed', error: String((err && err.message) || err) }); }

self.onmessage = function (e) {
    var m = e.data || {};
    if (m.type === 'init') {
        try {
            importScripts(m.scripts.go);
            var go = new self.Go();
            WebAssembly.instantiate(m.engine, go.importObject).then(function (instance) {
                go.run(instance);
                api = self.yavcD2 || null;
                if (api) { self.postMessage({ type: 'ready' }); } else { failed('the diagram engine did not start'); }
            }, failed);
        } catch (err) { failed(err); }
        return;
    }
    if (m.type === 'call') {
        var out;
        try { out = api && typeof api[m.name] === 'function' ? api[m.name](m.arg) : JSON.stringify({ error: 'the diagram engine has no function called ' + m.name }); }
        catch (err) { out = JSON.stringify({ error: 'the diagram engine failed: ' + ((err && err.message) || err) }); }
        self.postMessage({ type: 'result', id: m.id, out: out });
    }
};
