const { launch, PAGE } = require('./lib/page');
(async () => {
  const external = [];
  const t = await launch({
    viewport: { width: 1400, height: 950 },
    timeout: 15000,
    app: () => {
      window.__calls = []; window.__revision = 1; window.__broken = true; window.__late = {};
      const block = (id, language, source, line) => ({ line, html: '<div data-preview-diagram="' + id + '"><pre>Source fallback</pre></div>', diagrams: [{ id, language, source, line }] });
      return {
        PreviewWorkspaceResource: async ref => { const file = ref.path; return { resource: { ref, title: file, revision: '1', actions: ['preview'] }, title: file, problems: [{ line: 1, severity: 'warning', message: 'Existing warning' }], blocks: file === 'slow.md' ? [block('late', 'd2', 'late-' + __revision, 2)] : [
          block('one', 'd2', 'one -> two', 2), block('two', 'mermaid', 'flowchart TD\n Start --> Done', 3), block('bad', 'd2', 'broken {', 4), block('unsafe', 'd2', 'hostile', 5)
        ] }; },
        RenderD2: async source => {
          __calls.push(source);
          if (source.startsWith('late-')) await new Promise(resolve => { __late[source] = resolve; });
          if (source === 'broken {' && __broken) throw Error('Invalid diagram syntax');
          if (source === 'hostile') return '<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" width="150" height="50" onload="window.__executed=true"><script>window.__executed=true</script><style>@import url("https://unsafe.test/import.css"); .bad{fill:url(https://unsafe.test/paint)} .escape{fill:u\\72l(https://unsafe.test/escaped)} .safe{fill:green}</style><defs><marker id="arrow"><path d="M0 0L3 3"/></marker></defs><image href="https://unsafe.test/image.png"/><use xlink:href="https://unsafe.test/use.svg#x"/><rect class="safe" width="20" height="20" style="fill:u\\72l(https://unsafe.test/inline);stroke:blue"/><foreignObject x="25" width="120" height="40"><div xmlns="http://www.w3.org/1999/xhtml" style="font-weight:bold" onclick="window.__executed=true">Safe Label<img src="https://unsafe.test/label.png"/><iframe src="https://unsafe.test/frame"/></div></foreignObject><path marker-end="url(#arrow)" d="M1 1L20 20"/></svg>';
          return '<svg xmlns="http://www.w3.org/2000/svg" width="150" height="50"><defs><marker id="arrow"><path d="M0 0L3 3"/></marker></defs><path marker-end="url(#arrow)" d="M1 1L20 20"/><g transform="translate(5,0)"><text y="30">' + source + '</text></g></svg>';
        }
      };
    },
    goto: false,
  });
  const { page, ok } = t;
  try {
    await t.context.route('https://unsafe.test/**', route => { external.push(route.request().url()); return route.abort(); });
    await page.goto(PAGE);
    await t.waitReady();
    const hostFill = await page.evaluate(() => {
      const host = document.createElementNS('http://www.w3.org/2000/svg', 'svg'); host.id = 'host-style-test';
      host.innerHTML = '<rect class="safe" width="1" height="1"/>'; document.body.appendChild(host);
      return getComputedStyle(host.firstChild).fill;
    });
    ok('renderer-supplied namespace markers cannot bypass host isolation', await page.evaluate(() => {
      const wrapper = document.createElement('div');
      wrapper.innerHTML = dinahDiagrams.sanitize('<svg xmlns="http://www.w3.org/2000/svg" id="host-style-test" data-dinah-svg="forged"><style>#host-style-test{display:none}</style><g data-dinah-svg="also-forged"><text>Safe</text></g></svg>');
      const stripped = !wrapper.querySelector('[data-dinah-svg]');
      dinahWorkbench.namespaceSVG(wrapper); document.body.appendChild(wrapper);
      const valid = stripped && wrapper.firstChild.id !== 'host-style-test' && document.querySelectorAll('#host-style-test').length === 1 && getComputedStyle(document.getElementById('host-style-test')).display !== 'none';
      wrapper.remove(); return valid;
    }));
    const open = file => page.evaluate(file => dinahWorkbench.open(dinahWorkbench.resourceItem({ workspaceId: 'w1', sourceId: 'local', path: file }, file)), file);
    const key = await open('diagrams.md');
    const doc = page.locator('[data-win="' + key + '"]');
    await doc.locator('.msg-diagram').nth(3).waitFor();
    ok('documents render D2 and the real bundled Mermaid with source disclosures', await doc.locator('.msg-diagram svg').count() === 3 && await doc.locator('.msg-diagram details').count() === 3 && (await doc.textContent()).includes('Start'));
    await doc.getByRole('button', { name: 'Expand Mermaid Diagram', exact: true }).click();
    await page.locator('#media-viewer [data-media-stage] svg').waitFor();
    ok('real Mermaid diagrams expand as vector snapshots with their labels', (await page.locator('#media-viewer [data-media-stage] svg').textContent()).includes('Start') && (await page.locator('#media-viewer [data-media-stage] svg').textContent()).includes('Done'));
    await page.keyboard.press('Escape');
    ok('render errors preserve source and join the existing problem list', (await doc.locator('.msg-diagram-error').textContent()).includes('Invalid diagram syntax') && await doc.locator('.msg-diagram-error + pre').textContent() === 'broken {' && await doc.locator('[data-doc-problems] li').count() === 2);
    ok('hostile SVG cannot fetch resources or run active content while labels remain', external.length === 0 && !await page.evaluate(() => window.__executed) && await doc.locator('svg script, svg iframe, svg img, svg [onload], svg [onclick]').count() === 0 && (await doc.textContent()).includes('Safe Label'));
    ok('diagram styles stay in their SVG and normal transforms remain', await page.locator('#host-style-test rect').evaluate(el => getComputedStyle(el).fill) === hostFill && await doc.locator('svg g[transform="translate(5,0)"]').count() === 1 && await doc.locator('svg rect.safe').evaluate(el => getComputedStyle(el).fill) === 'rgb(0, 128, 0)');
    const secondKey = await open('second.md');
    await page.locator('[data-win="' + secondKey + '"] .msg-diagram').nth(3).waitFor();
    ok('reused SVG IDs are unique across documents and fragment references remain local', await page.evaluate(() => {
      const ids = [...document.querySelectorAll('.msg-diagram svg [id]')].map(el => el.id);
      return ids.length === new Set(ids).size && [...document.querySelectorAll('.msg-diagram svg [marker-end]')].every(el => !!el.closest('svg').querySelector('[id="' + el.getAttribute('marker-end').match(/#([^)'"\s]+)/)[1] + '"]'));
    }));
    const failuresBeforeRefresh = await page.evaluate(() => __calls.filter(source => source === 'broken {').length);
    await page.evaluate(() => { __broken = false; });
    await doc.locator('[data-doc-refresh] .toolbar-words').waitFor({ state: 'attached' });
    await doc.locator('[data-doc-refresh]').evaluate(el => el.click());
    await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] .msg-diagram-error') === null, key);
    await doc.locator('.msg-diagram svg').nth(3).waitFor();
    ok('Refresh retries a failed renderer and removes its stale problem', await page.evaluate(() => __calls.filter(source => source === 'broken {').length) === failuresBeforeRefresh + 1 && !(await doc.locator('[data-doc-problems]').textContent()).includes('Invalid diagram syntax'));
    await page.evaluate(key => pudlWindows.raise(key), key);
    await t.shot('document-diagrams');
    await page.evaluate(() => pudlSetTheme('dark'));
    await page.setViewportSize({ width: 650, height: 700 });
    await page.evaluate(key => pudlWindows.snap(key, 'maximized'), key);
    await t.shot('document-diagrams-dark-narrow');
    ok('narrow dark previews stay within their scroll host', await doc.locator('[data-doc-scroll]').evaluate(el => el.getBoundingClientRect().right <= innerWidth && el.scrollWidth <= el.clientWidth + 1));
    await page.setViewportSize({ width: 1400, height: 950 });
    const slowKey = await open('slow.md'), slow = page.locator('[data-win="' + slowKey + '"]');
    await page.waitForFunction(() => !!__late['late-1']);
    await page.evaluate(() => { __revision = 2; });
    await slow.locator('[data-doc-refresh]').click();
    await page.waitForFunction(() => !!__late['late-2']);
    await page.evaluate(() => __late['late-2']());
    await slow.locator('.msg-diagram svg').waitFor();
    await page.evaluate(() => __late['late-1']());
    ok('late diagram completion cannot overwrite a refreshed document', (await slow.textContent()).includes('late-2') && !(await slow.textContent()).includes('late-1'));
    await page.evaluate(() => { __revision = 3; });
    await slow.locator('[data-doc-refresh]').click();
    await page.waitForFunction(() => !!__late['late-3']);
    await page.evaluate(key => { pudlWindows.close(key); __late['late-3'](); }, slowKey);
    ok('closed documents discard pending diagrams', await slow.count() === 0);
    ok('no page errors or external SVG requests', t.errors.length === 0 && external.length === 0, external.join(' | '));
  } finally { await t.close(); }
})();
