const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 950 },
    app: () => {
      const image = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="2200" height="1400"><rect width="2200" height="1400" fill="lightblue"/><text y="90" x="90" font-size="60">Embedded Image</text></svg>');
      return {
        PreviewWorkspaceResource: async ref => ({ resource: { ref, title: 'Pictures', revision: '1', actions: ['preview'] }, title: 'Pictures', problems: [], blocks: [{ line: 1, html: '<div data-preview-diagram="large"><pre>a -> b</pre></div><img alt="Architecture Image" src="' + image + '"><a href="https://example.test"><img alt="Linked" src="' + image + '"></a><img class="icon" alt="Icon" src="' + image + '">', diagrams: [{ id: 'large', language: 'd2', source: 'a -> b', line: 1 }] }] }),
        RenderD2: async () => '<svg xmlns="http://www.w3.org/2000/svg" width="2400" height="1800" viewBox="0 0 2400 1800"><defs><marker id="arrow"><path d="M0 0L5 5"/></marker></defs><rect width="2400" height="1800" fill="lightyellow"/><path d="M50 50L1500 900" stroke="blue" marker-end="url(#arrow)"/><text x="40" y="90" font-size="50">Vector Diagram</text></svg>'
      };
    },
    goto: false,
  });
  const { page, ok } = t;
  try {
    // The viewer is tried at a fractional scale, where a fit that is off by
    // a pixel shows as a scrollbar. The fixture's window has no scale
    // option, so the scale is set through the browser's own emulation, and
    // set again after the viewport changes, since setting the viewport
    // puts the scale back to one.
    const cdp = await t.context.newCDPSession(page);
    const scale = (width, height) => cdp.send('Emulation.setDeviceMetricsOverride', { width, height, deviceScaleFactor: 1.25, mobile: false });
    await scale(1400, 950);
    await page.goto(PAGE);
    await t.waitReady();
    const key = await page.evaluate(() => dinahWorkbench.open(dinahWorkbench.resourceItem({ workspaceId: 'w1', sourceId: 'local', path: 'pictures.md' }, 'Pictures')));
    const doc = page.locator('[data-win="' + key + '"]'), viewer = page.locator('#media-viewer'), stage = viewer.locator('[data-media-stage]');
    const expand = doc.getByRole('button', { name: 'Expand D2 Diagram', exact: true });
    await expand.waitFor();
    const sourceSVG = await doc.locator('.msg-diagram svg').evaluate(el => el.outerHTML);
    const workbenchURL = page.url();
    async function stableFit() {
      return stage.evaluate(async el => {
        const sizes = new Set();
        for (let frame = 0; frame < 30; frame++) {
          await new Promise(requestAnimationFrame);
          const media = el.querySelector('.lightbox-img').getBoundingClientRect(), box = el.getBoundingClientRect();
          sizes.add([el.clientWidth, el.clientHeight, el.scrollWidth, el.scrollHeight].join(','));
          if (el.scrollWidth > el.clientWidth || el.scrollHeight > el.clientHeight || media.width > box.width + 1 || media.height > box.height + 1) return false;
        }
        return sizes.size === 1;
      });
    }
    async function closeTo(trigger) {
      await page.keyboard.press('Escape');
      await page.waitForFunction(() => !document.getElementById('media-viewer').open);
      if (trigger) await page.waitForFunction(el => document.activeElement === el, await trigger.elementHandle());
    }
    await t.shot('media-viewer-inline-glyph');
    await expand.focus(); await page.keyboard.press('Enter'); await viewer.waitFor();
    ok('Parks Computing lightbox fills the client area', await viewer.evaluate(el => {
      const r = el.getBoundingClientRect(); return el.classList.contains('lightbox') && r.width >= innerWidth - 2 && r.height >= innerHeight - 2;
    }));
    ok('Fit remains stable without scrollbar judder', await stableFit());
    ok('vector snapshot has isolated fragment references', await stage.locator('svg').evaluate(svg => {
      const ids = [...document.querySelectorAll('.msg-diagram svg [id],#media-viewer .lightbox-stage svg [id]')].map(el => el.id);
      const marker = svg.querySelector('[marker-end]').getAttribute('marker-end').match(/#([^)'"\s]+)/)[1];
      return new Set(ids).size === ids.length && !!svg.querySelector('[id="' + marker + '"]');
    }));
    await t.shot('media-viewer-diagram');
    await stage.locator('.lightbox-img').click();
    ok('clicking a fitted diagram zooms to natural size', await viewer.evaluate(el => el.classList.contains('lightbox-zoomed')) && await stage.locator('svg').evaluate(el => el.getBoundingClientRect().width === 2400));
    await t.shot('media-viewer-natural-size');
    await stage.evaluate(el => el.scrollTo(0, 0));
    const bounds = await stage.boundingBox();
    await page.mouse.move(bounds.x + 250, bounds.y + 230); await page.mouse.down();
    await page.mouse.move(bounds.x + 100, bounds.y + 100, { steps: 5 }); await page.mouse.up();
    ok('drag pans the natural-size diagram', await stage.evaluate(el => el.scrollLeft >= 140 && el.scrollTop >= 120));
    await page.mouse.click(bounds.x + 200, bounds.y + 200);
    ok('clicking again restores Fit and clears pan', !await viewer.evaluate(el => el.classList.contains('lightbox-zoomed')) && await stage.evaluate(el => el.scrollLeft === 0 && el.scrollTop === 0) && await stableFit());
    ok('inline SVG remains unchanged', await doc.locator('.msg-diagram svg').evaluate(el => el.outerHTML) === sourceSVG);
    ok('lightbox leaves the workbench URL unchanged', page.url() === workbenchURL);
    await closeTo(expand);
    ok('Escape restores focus', await expand.evaluate(el => el === document.activeElement));
    await expand.click(); await page.mouse.click(2, 2);
    await page.waitForFunction(() => !document.getElementById('media-viewer').open);
    ok('clicking empty overlay closes the lightbox', !await viewer.evaluate(el => el.open));
    await doc.locator('[data-doc-page]').evaluate(el => { const icon = document.createElement('img'); icon.id = 'delayed-icon'; el.appendChild(icon); });
    await doc.locator('#delayed-icon[data-media-expand]').waitFor({ state: 'attached' });
    await doc.locator('#delayed-icon').evaluate(el => { el.src = 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16"><rect width="16" height="16"/></svg>'); });
    await page.waitForFunction(() => document.getElementById('delayed-icon').naturalWidth === 16 && !document.getElementById('delayed-icon').hasAttribute('data-media-expand'));
    ok('delayed small icons lose the lightbox affordance', !await doc.locator('#delayed-icon').getAttribute('role'));
    await page.evaluate(() => { const broken = new Image(); broken.src = 'data:image/png;base64,broken'; dinahMediaViewer.open(broken, document.querySelector('.msg-diagram .media-expand'), 'Broken Image'); });
    await viewer.getByText('The image could not be loaded.', { exact: true }).waitFor();
    ok('image failures leave Close available', await viewer.getByRole('button', { name: 'Close', exact: true }).isEnabled());
    await closeTo();
    const image = doc.locator('img[alt="Architecture Image"]');
    await page.waitForFunction(() => document.querySelector('img[alt="Architecture Image"]').complete);
    const imageExpand = doc.locator('.media-image-frame .media-expand').first();
    await imageExpand.click(); await stage.locator('img').waitFor();
    await stage.locator('img').click();
    ok('embedded images zoom to natural resolution', await stage.locator('img').evaluate(el => el.naturalWidth === 2200 && el.getBoundingClientRect().width === 2200));
    await closeTo(imageExpand);
    ok('linked images and icons retain their own actions', await doc.locator('img[alt="Linked"],img[alt="Icon"]').evaluateAll(images => images.every(image => !image.hasAttribute('data-media-expand') && !image.hasAttribute('role'))));
    await page.evaluate(() => {
      const source = document.querySelector('img[alt="Architecture Image"]');
      const second = new Image(); second.src = source.src; second.alt = 'Second Image'; source.closest('.media-image-frame').after(second);
    });
    await doc.locator('img[alt="Second Image"][data-media-expand]').waitFor();
    await image.press('Enter'); await stage.locator('img').waitFor();
    await page.keyboard.press('End');
    ok('gallery navigation reaches the next image', await stage.locator('img').getAttribute('alt') === 'Second Image' && (await viewer.locator('.lightbox-counter').textContent()).includes('of'));
    await page.keyboard.press('Home');
    ok('gallery Home returns to its first item', !(await viewer.locator('.lightbox-prev').isVisible()));
    await page.evaluate(() => pudlSetTheme('dark')); await page.setViewportSize({ width: 520, height: 650 }); await scale(520, 650);
    ok('resized Fit remains stable', await stableFit());
    await t.shot('media-viewer-dark-narrow');
    await page.evaluate(key => pudlWindows.close(key), key);
    ok('closing the source leaves a usable snapshot', await doc.count() === 0 && await stage.locator('.lightbox-img').count() === 1);
    await closeTo(); await page.waitForFunction(() => document.activeElement === document.getElementById('main'));
    ok('closed source returns focus to the workbench', await page.locator('#main').evaluate(el => el === document.activeElement));
  } finally { await t.close(); }
})();
