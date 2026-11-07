const { launch } = require('./lib/page');
(async () => {
  const t = await launch();
  try {
    const cdp = await t.context.newCDPSession(t.page);
    await t.page.evaluate(() => {
      const root = document.createElement('div');
      root.innerHTML = '<div style="height:100px;overflow:auto"><div></div></div>';
      document.body.append(root);
      const log = root.firstChild, content = log.firstChild;
      window.__retention = { root, log, content, entries: [], refs: [] };
      const fixture = window.__retention;
      fixture.nav = dinahTranscript.mount({ root, log, content, ready: () => true, entries: () => fixture.entries });
      fixture.replace = () => {
        fixture.entries = [];
        content.replaceChildren();
        for (let i = 0; i < 200; i++) {
          const row = document.createElement('p'); row.textContent = 'Retained transcript row ' + i;
          content.append(row); fixture.entries.push({ identity: row, element: row }); fixture.refs.push(new WeakRef(row));
        }
        fixture.nav.update();
      };
    });
    const samples = [];
    for (let batch = 0; batch < 5; batch++) {
      await t.page.evaluate(() => __retention.replace());
      await t.page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      await cdp.send('HeapProfiler.collectGarbage');
      samples.push(await t.page.evaluate(() => ({ live: __retention.refs.filter(ref => ref.deref()).length, displayed: __retention.entries.length })));
    }
    require('fs').writeFileSync(require('path').join(process.argv[3], 'transcript-retention.json'), JSON.stringify(samples, null, 2));
    t.ok('discarded transcript rows are collectible while the transcript stays open', samples[4].live <= 200);
    await t.page.evaluate(() => {
      __retention.nav.destroy(); __retention.content.replaceChildren(); __retention.entries = [];
    });
    await t.page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await cdp.send('HeapProfiler.collectGarbage');
    t.ok('destroy releases unread entries even if the navigation handle is kept', await t.page.evaluate(() => __retention.refs.every(ref => !ref.deref())));
    await t.page.evaluate(() => { __retention.root.remove(); delete window.__retention; });
  } finally { await t.close(); }
})();
