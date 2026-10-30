// Attachments of a resource's preview: their URLs, the lightbox, a missing
// image among the document's problems, direct image and diagram resources,
// and a closed window ignoring a late image.
const { launch } = require('./lib/page');
(async () => {
  const requests = [];
  const t = await launch({
    // Wide enough for both default docks and documents between them.
    viewport: { width: 1920, height: 900 },
    state: { workspaceId: 'owner', workspaceRecords: [{ id: 'owner', name: 'Owner', executionPath: 'C:/Fixture', sources: [
      { id: 'local', kind: 'local-folder', name: 'Local Documents', path: 'D:/Attached' },
      { id: 'shared', kind: 'local-folder', name: 'Shared Documents', path: 'S:/Shared' }
    ] }] },
    app: () => {
      const blocks = () => [{ line: 55, html: '<h2>Attachments</h2><img data-attachment="diagram one.png" alt="Attached Diagram">' + (window.__missing ? '<img data-attachment="missing.png">' : '') }];
      return {
        PreviewWorkspaceResource: async ref => ({ resource: { ref, title: 'Resource', revision: '1', actions: ['preview'] },
          format: ['photo.png', 'drawing.svg'].includes(ref.path) ? 'image' : ref.path.endsWith('.d2') ? 'd2' : ref.path.endsWith('.mmd') ? 'mermaid' : '',
          blocks: ['photo.png', 'drawing.svg'].includes(ref.path) ? [{ line: 1, html: '<img data-attachment="' + ref.path + '" alt="Photo">' }] : /\.(d2|mmd)$/.test(ref.path) ? [{ line: 1, html: '<div data-preview-diagram="direct"><pre>Source</pre></div>', diagrams: [{ id: 'direct', language: ref.path.endsWith('.d2') ? 'd2' : 'mermaid', source: ref.path.endsWith('.d2') ? 'Read -> Review' : 'flowchart TD\n Read --> Review', line: 1 }] }] : blocks(),
          problems: [{ line: 1, severity: 'warning', message: 'Existing warning' }] }),
        RenderD2: async () => '<svg xmlns="http://www.w3.org/2000/svg" width="300" height="100"><text y="50">Direct D2</text></svg>'
      };
    },
    goto: false,
  });
  const { page, ok, errors } = t;
  try {
    await t.context.route(/\/(resource-attachment|doc-file)\?/, route => {
      const url = new URL(route.request().url()); requests.push(url);
      if (url.searchParams.get('name') === 'drawing.svg') return route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1909 394"><svg width="1909" height="394" viewBox="0 0 1909 394"><rect width="1909" height="394" fill="white"/><text x="40" y="80" fill="black">SVG Flow</text></svg></svg>' });
      return url.searchParams.get('name') === 'missing.png' ? route.fulfill({ status: 404, body: 'Unavailable' }) :
        route.fulfill({ contentType: 'image/svg+xml', body: '<svg xmlns="http://www.w3.org/2000/svg" width="240" height="120"><rect width="240" height="120" fill="teal"/></svg>' });
    });
    await page.goto('http://dinah.test/index.html');
    await t.waitReady();
    async function open(ref) {
      const key = await page.evaluate(ref => dinahWorkbench.open(dinahWorkbench.resourceItem(ref, 'Resource')), ref);
      const win = page.locator('[data-win="' + key + '"]');
      if (!/\.(d2|mmd)$/.test(ref.path)) {
        await win.locator('img[data-attachment]').first().waitFor({ state: 'attached' });
        await win.locator('img[data-attachment]').first().evaluate(image => image.decode());
      } else await win.locator('.msg-diagram svg').waitFor();
      return { key, win };
    }
    const local = await open({ workspaceId: 'owner', sourceId: 'local', path: 'nested/readme.md' });
    ok('nested local attachment URLs retain registered identity and encoded filename', requests.some(url => url.pathname === '/resource-attachment' && url.searchParams.get('workspaceId') === 'owner' && url.searchParams.get('sourceId') === 'local' && url.searchParams.get('path') === 'nested/readme.md' && url.searchParams.get('name') === 'diagram one.png'));
    await local.win.getByRole('button', { name: 'Expand Image: Attached Diagram', exact: true }).last().click();
    await page.locator('#media-viewer [data-media-stage] img').waitFor();
    ok('document attachments use the shared lightbox', await page.locator('#media-viewer [data-media-stage] img').evaluate(image => image.naturalWidth === 240));
    await page.keyboard.press('Escape');
    const shared = await open({ workspaceId: 'owner', sourceId: 'shared', path: 'guide.md' });
    ok('a second source owns its own attachment requests and metadata', requests.some(url => url.searchParams.get('sourceId') === 'shared' && url.searchParams.get('path') === 'guide.md') && (await shared.win.locator('[data-doc-meta]').textContent()).includes('Shared Documents'));
    await page.evaluate(() => { window.__missing = true; });
    // The lightbox gives focus back to the first window as it closes, which
    // can leave that window over this one's Refresh; the check is about the
    // missing image, so the button is pressed directly, once its command has
    // drawn it.
    await shared.win.locator('[data-doc-refresh] .toolbar-words').waitFor({ state: 'attached' });
    await shared.win.locator('[data-doc-refresh]').evaluate(button => button.click());
    await shared.win.locator('.dinah-broken-image').waitFor();
    // The Problems control is drawn from its command in the next frame.
    const nextFrame = () => page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
    await nextFrame();
    ok('missing image joins existing document problems at its source line', await shared.win.locator('[data-doc-problems-btn]').textContent() === '2 Problems' && (await shared.win.locator('[data-doc-problems]').textContent()).includes('Line 55errorImage could not be loaded: missing.png'));
    await shared.win.locator('img[data-attachment]').evaluate(image => { window.__oldImage = image; window.__missing = false; });
    await shared.win.locator('[data-doc-refresh]').evaluate(button => button.click());
    await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] [data-doc-problems-btn]').textContent === '1 Problem', shared.key);
    await page.evaluate(() => __oldImage.dispatchEvent(new Event('error')));
    await nextFrame();
    ok('refresh clears image errors and ignores callbacks from the old preview', await shared.win.locator('.dinah-broken-image').count() === 0 && await shared.win.locator('[data-doc-problems-btn]').textContent() === '1 Problem');
    const photo = await open({ workspaceId: 'owner', sourceId: 'local', path: 'photo.png' });
    ok('direct image resources use their own file reference', await photo.win.locator('img').evaluate(image => new URL(image.src).searchParams.get('path') === 'photo.png' && new URL(image.src).searchParams.get('name') === 'photo.png' && image.naturalWidth === 240));
    ok('direct image viewer removes document print margins', await photo.win.locator('.doc-page').evaluate(page => page.classList.contains('media-preview') && parseFloat(getComputedStyle(page).paddingBottom) < 20));
    for (const filename of ['drawing.d2', 'drawing.mmd']) {
      const diagram = await open({ workspaceId: 'owner', sourceId: 'local', path: filename });
      ok('direct ' + filename + ' renders a diagram with source and expansion controls', await diagram.win.locator('.doc-page.media-preview .msg-diagram svg').count() === 1 && await diagram.win.locator('.msg-diagram details').count() === 1 && await diagram.win.locator('.msg-diagram .media-expand').count() === 1);
      if (filename.endsWith('.mmd')) await t.shot('resource-diagram-direct');
    }
    ok('each opened resource keeps its own normal window without opening the lightbox', await page.locator('.win[data-win-kind="workspace-resource"]').count() === 5 && !await page.locator('#media-viewer').evaluate(viewer => viewer.open));
    await page.evaluate(() => dinahWorkbench.open(dinahWorkbench.resourceItem({ workspaceId: 'owner', sourceId: 'local', path: 'photo.png' }, 'Photo')));
    const windowAddress = page.url();
    await photo.win.getByRole('button', { name: 'Expand Image: Photo', exact: true }).last().click();
    await page.locator('#media-viewer [data-media-stage] img').waitFor();
    ok('direct image expands only on request and leaves its window intact', await photo.win.count() === 1 && page.url() === windowAddress);
    await page.keyboard.press('Escape');
    const vector = await open({ workspaceId: 'owner', sourceId: 'local', path: 'drawing.svg' });
    const vectorSize = await vector.win.locator('img').evaluate(image => ({ width: image.getBoundingClientRect().width, height: image.getBoundingClientRect().height, naturalWidth: image.naturalWidth, naturalHeight: image.naturalHeight }));
    ok('viewBox-only SVG has a visible rendered area', vectorSize.width > 100 && vectorSize.height > 20);
    await page.evaluate(key => pudlWindows.raise(key), vector.key);
    await vector.win.locator('img').scrollIntoViewIfNeeded();
    await t.shot('resource-svg-direct');
    await local.win.locator('img[data-attachment]').first().evaluate(image => { window.__closedImage = image; });
    await page.evaluate(key => pudlWindows.close(key), local.key);
    await page.evaluate(() => __closedImage.dispatchEvent(new Event('error')));
    ok('a closed window ignores attachment completion', await local.win.count() === 0 && errors.length === 0);
  } finally { await t.close(); }
})();
