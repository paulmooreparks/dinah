const { launch } = require('./lib/page');
const fs = require('fs');
const path = require('path');
const { OUT } = require('./lib/page');
const { execFileSync } = require('child_process');
const crypto = require('crypto');
const frontend = path.resolve(__dirname, '..');
const bindings = {
  hostCommit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: frontend, encoding: 'utf8' }).trim(),
  pudlVersion: fs.readFileSync(path.join(frontend, 'pudl/VERSION'), 'utf8').trim(),
  anchorCall: 'pudlMenubar.launcherButton({visible:true})',
  source: 'Real index.html and unmodified PUDL in the shared browser harness; App bridge is stubbed.',
  hashes: Object.fromEntries(['title-launcher.js', 'pudl/pudl-menubar.js', 'pudl/pudl-launcher.js', 'pudl/pudl-start.js'].map(file =>
    [file, crypto.createHash('sha256').update(fs.readFileSync(path.join(frontend, file))).digest('hex')]))
};
(async () => {
  const records = [];
  for (const theme of ['light', 'dark']) for (const ribbon of [false, true]) {
    const t = await launch({ viewport: { width: 1440, height: 1000 } });
    const p = t.page, name = theme + (ribbon ? '-ribbon' : '-menu');
    const captures = [];
    let cdp;
    try {
      await p.waitForFunction(() => window.pudlLauncher && pudlLauncher.state(document.getElementById('dinah-launcher')));
      await p.evaluate(({ theme, ribbon }) => { pudlSetTheme(theme); pudlMenubar.ribbon(ribbon); }, { theme, ribbon });
      await p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      let button;
      if (ribbon) button = p.getByRole('button', { name: 'Launcher', exact: true });
      else {
        await p.evaluate(() => { const b = pudlMenubar.launcherButton({ visible: true }); b.dataset.placementTestButton = ''; });
        button = p.locator('[data-placement-test-button]');
      }
      await button.waitFor({ state: 'visible' });
      const invoker = await button.boundingBox();
      const diagnostics = await p.evaluate(() => {
        const b = pudlMenubar.launcherButton({ visible: true }), r = b.getBoundingClientRect();
        return { selected: { text: b.textContent, label: b.getAttribute('aria-label'), tag: b.tagName,
          left: r.left, top: r.top, bottom: r.bottom }, version: document.querySelector('script[src*="pudl-launcher"]')?.src };
      });
      t.ok('the first opening starts closed ' + name, await p.evaluate(() => !document.getElementById('dinah-launcher').matches(':popover-open')));
      await p.evaluate(() => {
        const root = document.getElementById('dinah-launcher');
        window.__placementTrace = [];
        window.__placementDone = false;
        window.__placementEvents = [];
        for (const type of ['beforetoggle', 'toggle']) root.addEventListener(type, e => {
          if (e.target === root) __placementEvents.push({ type, newState: e.newState, at: performance.now(), top: root.style.top, left: root.style.left });
        });
        let first = null;
        const started = performance.now();
        function frame(at) {
          if (root.matches(':popover-open') && root.getClientRects().length && getComputedStyle(root).visibility !== 'hidden') {
            if (first === null) first = at;
            const r = root.getBoundingClientRect(), style = getComputedStyle(root);
            __placementTrace.push({ at, left: r.left, top: r.top, width: r.width, height: r.height,
              inlineTop: root.style.top, inlineLeft: root.style.left, transform: style.transform, transition: style.transitionProperty, animations: root.getAnimations().map(a => ({ type: a.constructor.name, property: a.transitionProperty || a.animationName || '' })) });
          }
          if (first !== null && at - first >= 1200 || at - started >= 4000) { __placementDone = true; return; }
          requestAnimationFrame(frame);
        }
        requestAnimationFrame(frame);
      });
      const timeOrigin = await p.evaluate(() => performance.timeOrigin);
      const captureDir = path.join(OUT, 'launcher-placement-frames', name);
      fs.mkdirSync(captureDir, { recursive: true });
      cdp = await t.context.newCDPSession(p);
      cdp.on('Page.screencastFrame', event => {
        const file = String(captures.length).padStart(4, '0') + '.png';
        fs.writeFileSync(path.join(captureDir, file), Buffer.from(event.data, 'base64'));
        captures.push({ file, metadata: event.metadata });
        cdp.send('Page.screencastFrameAck', { sessionId: event.sessionId }).catch(() => {});
      });
      await cdp.send('Page.startScreencast', { format: 'png', everyNthFrame: 1, maxWidth: 1440, maxHeight: 1000 });
      await p.mouse.click(invoker.x + invoker.width / 2, invoker.y + invoker.height / 2);
      await p.waitForFunction(() => __placementDone);
      const trace = await p.evaluate(() => __placementTrace), last = trace.at(-1);
      const events = await p.evaluate(() => __placementEvents);
      await cdp.send('Page.stopScreencast');
      const record = { name, invoker, diagnostics, trace, events, timeOrigin, captures,
        observationSources: { trace: 'requestAnimationFrame and getBoundingClientRect', events: 'native beforetoggle/toggle',
          captures: 'CDP Page.screencastFrame compositor PNGs; capture delivery may omit intermediate frames',
          invoker: 'visible control Playwright boundingBox', diagnostics: 'host anchor API return' } };
      records.push(record);
      t.ok('the launcher visibly opens ' + name, trace.length >= 10);
      if (last) {
        t.ok('the panel opens below its visible Launcher control ' + name, last.top >= invoker.y + invoker.height - 1,
          JSON.stringify({ invoker, final: last, diagnostics }));
        if (!ribbon) t.ok('menu-bar placement keeps its existing six-pixel gap ' + name,
          Math.abs(last.top - invoker.y - invoker.height - 6) <= 1 && Math.abs(last.left - Math.max(8, invoker.x)) <= 1,
          JSON.stringify({ invoker, final: last }));
        t.ok('first opening never moves after its first visible frame ' + name,
          trace.every(r => Math.abs(r.left - last.left) <= 1 && Math.abs(r.top - last.top) <= 1),
          JSON.stringify({ first: trace[0], final: last, minTop: Math.min(...trace.map(r => r.top)), maxTop: Math.max(...trace.map(r => r.top)), minLeft: Math.min(...trace.map(r => r.left)), maxLeft: Math.max(...trace.map(r => r.left)) }));
      }
      await t.shot('launcher-placement-' + name);
      record.covered = await p.evaluate(({ x, y }) => {
        const hit = document.elementFromPoint(x, y);
        return { panelCoversControl: document.getElementById('dinah-launcher').contains(hit), tag: hit?.tagName, text: hit?.textContent.slice(0, 100) };
      }, { x: invoker.x + invoker.width / 2, y: invoker.y + invoker.height / 2 });
      await p.mouse.click(invoker.x + invoker.width / 2, invoker.y + invoker.height / 2);
      await p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      record.closedOnSecondClick = await p.evaluate(() => !document.getElementById('dinah-launcher').matches(':popover-open'));
      t.ok('a second pointer click on the same control closes the panel ' + name, record.closedOnSecondClick, JSON.stringify(record.covered));
    } finally {
      fs.mkdirSync(OUT, { recursive: true });
      fs.writeFileSync(path.join(OUT, 'launcher-placement.json'), JSON.stringify({ bindings, records }, null, 2));
      await t.close();
    }
  }
})().catch(err => { console.error(err); process.exitCode = 1; });
