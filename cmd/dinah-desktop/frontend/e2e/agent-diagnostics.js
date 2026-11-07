const { launch, OUT } = require('./lib/page');
const fs = require('fs'), path = require('path');
(async () => {
  const t = await launch({
    init: () => {
      window.__diagnosticObservers = [];
      window.PerformanceObserver = class {
        static supportedEntryTypes = ['event', 'longtask', 'long-animation-frame'];
        constructor(callback) { this.callback = callback; this.connected = false; __diagnosticObservers.push(this); }
        observe(options) { this.type = options.type; this.connected = true; }
        disconnect() { this.connected = false; }
      };
      window.__answers = {};
    },
    app: () => ({ AnswerPage: async (id, result, error) => { __answers[id] = { result: result ? JSON.parse(result) : null, error }; } }),
  });
  const p = t.page, ok = t.ok;
  const ask = (args) => p.evaluate(async args => {
    const id = 'diagnostic-' + Math.random();
    __emit('page:ask', {id, op:'diagnostics', args: {expiresAt: Date.now() + 3000, ...args}});
    for (let i = 0; i < 100 && !__answers[id]; i++) await new Promise(resolve => setTimeout(resolve, 10));
    return __answers[id];
  }, args);
  try {
    let reply = await ask({operation:'snapshot'});
    ok('snapshot works through the page bridge before recording', reply.result.capture.state === 'idle' && reply.result.counts.elements > 0);
    reply = await ask({operation:'start',captureId:'capture-one',seconds:10});
    ok('capture starts without changing responsiveness preferences', reply.result.capture.state === 'running' && await p.locator('#set-responsiveness').getAttribute('aria-checked') === 'false');
    ok('overlapping capture is refused', !!(await ask({operation:'start',captureId:'capture-two',seconds:10})).error);
    ok('stale Stop cannot stop the active capture', !!(await ask({operation:'stop',captureId:'old'})).error);
    await p.evaluate(() => {
      const scripts = Array.from({length:12}, (_, i) => ({sourceURL:i ? 'https://private.example/secret?token=SECRET' : new URL('transcript.js?secret=SECRET', location.href).href, sourceCharPosition:32, duration:12, forcedStyleAndLayoutDuration:7, target:document.body}));
      const event = {startTime:performance.now(),duration:90,blockingDuration:40,scripts};
      __diagnosticObservers.find(o=>o.connected && o.type==='long-animation-frame').callback({getEntries:()=>[event]});
      event.scripts[0].duration = 9999;
    });
    reply = await ask({operation:'snapshot'});
    const frame = reply.result.events.find(e=>e.kind==='long_frame');
    ok('attribution keeps shipped source positions and forced layout only', frame.scripts[0].source === 'transcript.js' && frame.scripts[0].durationMs === 12 && frame.scripts[0].position === 32 && frame.scripts[0].forcedLayoutMs === 7);
    ok('unknown sources, query strings and DOM objects are excluded', !JSON.stringify(reply).includes('SECRET') && !JSON.stringify(reply).includes('private.example') && frame.scripts[1].source === 'unknown');
    ok('script overflow is reported', frame.scripts.length === 8 && reply.result.coverage.scriptOmissions === 4);
    for (let batch=0;batch<3;batch++) {
      await p.evaluate(() => { for(let i=0;i<150;i++) dinahAgentDiagnostics.record('private label SECRET',i,0); });
      await p.waitForTimeout(1100);
    }
    reply = await ask({operation:'snapshot'});
    ok('rate and retention bounds are reported', reply.result.events.length <= 200 && reply.result.coverage.rateOmissions > 0 && reply.result.coverage.overwritten > 0 && !JSON.stringify(reply).includes('SECRET'));
    fs.writeFileSync(path.join(OUT,'agent-diagnostics-page.json'), JSON.stringify(reply.result,null,2));
    reply = await ask({operation:'stop',captureId:'capture-one'});
    const retained = reply.result.events.length;
    ok('stop disconnects every capture observer', reply.result.capture.state === 'stopped' && await p.evaluate(()=>__diagnosticObservers.every(o=>!o.connected)));
    await p.evaluate(()=>dinahAgentDiagnostics.record('chat output',100,0));
    ok('stopped evidence remains unchanged', (await ask({operation:'snapshot'})).result.events.length === retained);
    ok('expired control cannot run later', !!(await ask({operation:'start',captureId:'expired',seconds:5,expiresAt:1})).error);
    reply = await ask({operation:'start',captureId:'capture-two',seconds:1});
    ok('new capture clears old evidence', reply.result.coverage.overwritten === 0 && reply.result.capture.id === 'capture-two');
    await p.waitForTimeout(1200);
    ok('capture expires without another request', await p.locator('#agent-diagnostics-status').textContent() === 'Agent diagnostic capture: complete.');
    ok('expired capture disconnects observers', await p.evaluate(()=>__diagnosticObservers.every(o=>!o.connected)));
    await ask({operation:'start',captureId:'capture-three',seconds:10});
    await p.locator('#agent-diagnostics-stop').evaluate(button=>button.click());
    ok('person can stop the capture locally', (await ask({operation:'snapshot'})).result.capture.state === 'stopped');
  } finally { await t.close(); }
  const real = await launch();
  try {
    await real.page.evaluate(() => {
      dinahAgentDiagnostics.request({operation:'start',captureId:'real-browser',seconds:5,expiresAt:Date.now()+3000});
    });
    // Start the stall in a later frame so its timing interval begins inside the capture.
    await real.page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await real.page.evaluate(() => {
      dinahRender.enqueue({}, document.body, () => {
        const until = performance.now() + 90;
        while (performance.now() < until) { /* Known synthetic main-thread stall. */ }
        document.body.style.paddingTop = '1px';
        return false;
      }, null, 'chat output');
    });
    await real.page.waitForFunction(() => dinahAgentDiagnostics.request({operation:'snapshot',expiresAt:Date.now()+3000}).events.some(e=>e.kind==='long_task'));
    await real.page.waitForFunction(() => { const s = dinahAgentDiagnostics.request({operation:'snapshot',expiresAt:Date.now()+3000}); return s.features.longFrame !== 'measured' || s.events.some(e=>e.kind==='long_frame'); });
    const measured = await real.page.evaluate(() => dinahAgentDiagnostics.request({operation:'stop',captureId:'real-browser',expiresAt:Date.now()+3000}));
    real.ok('real browser observers detect a known main-thread stall', measured.events.some(e=>e.kind==='long_task' && e.durationMs >= 50));
    real.ok('real render work is attributed to the scheduler category', measured.events.some(e=>e.kind==='render' && e.label==='chat' && e.durationMs >= 80));
    fs.writeFileSync(path.join(OUT,'agent-diagnostics-real.json'), JSON.stringify(measured,null,2));
  } finally { await real.close(); }
})();
