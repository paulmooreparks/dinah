// The width of the frame the media viewer puts round a content picture
// (media-viewer.js, enhance). A picture declared at 80% must show at 80% of
// its column, a picture declared in pixels at that width or the column's
// when narrower, and a picture with no width at most the column's width.
// The frame must hug the picture so the expand button stays on its corner,
// and a document page in a scroll area must stay within the scroll area.
// The content is invented: a 3000x600 PNG drawn on a canvas.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1400, height: 900 } });
  const { page, ok } = t;
  try {
    // Docs loads yavc's page stylesheet into the window (md-engine/loader.js);
    // load it here too, so the page below is styled as a document is.
    await page.evaluate(() => new Promise((resolve, reject) => {
      const link = document.createElement('link'); link.rel = 'stylesheet'; link.href = 'yavc/content/md-engine/page.css';
      link.onload = resolve; link.onerror = reject; document.head.appendChild(link);
    }));
    const pictures = await page.evaluate(() => {
      const draw = (w, h) => { const c = document.createElement('canvas'); c.width = w; c.height = h; const g = c.getContext('2d'); g.fillStyle = '#8ac'; g.fillRect(0, 0, w, h); g.fillStyle = '#246'; g.fillRect(0, 0, w / 10, h / 10); return c.toDataURL('image/png'); };
      return { wide: draw(3000, 600), photo: draw(1200, 800), strip: draw(2000, 200), tall: draw(200, 2000), small: draw(200, 100) };
    });
    await page.evaluate(async p => {
      const host = document.createElement('div'); host.id = 'frame-width-fixture';
      host.style.cssText = 'position:fixed;left:0;top:0;z-index:99999;background:#fff;color:#000';
      host.innerHTML =
        '<div id="column" class="doc-page" style="width:600px;min-width:0;min-height:0;padding:0;box-sizing:content-box">' +
        '<p><img id="attr-pct" alt="Attribute Percent" width="80%"></p>' +
        '<p><img id="style-pct" alt="Style Percent" style="width:80%"></p>' +
        '<p><img id="attr-px" alt="Attribute Pixels" width="400"></p>' +
        '<p><img id="attr-px-wide" alt="Attribute Pixels Wide" width="900"></p>' +
        '<p><img id="no-width" alt="No Width"></p>' +
        '<p id="loaded-slot"></p>' +
        '<p><img id="ratio-only" alt="Shape Only"></p>' +
        '<p><img id="small" alt="Small"></p>' +
        '<p><img id="height-only" alt="Height Only" height="60"></p>' +
        '<figure class="sc-figure"><img id="figure-max" alt="Figure Cap" style="max-width:80%"></figure>' +
        '</div>' +
        '<div id="scroll" style="width:684px;height:300px;overflow:auto">' +
        '<div id="page" class="doc-page"><p>A paragraph beside the picture.</p><p><img id="page-pct" alt="Page Picture" width="80%"></p>' +
        '<table><tr><th>Column</th><th>Value</th></tr><tr><td>One</td><td>Two</td></tr></table></div></div>' +
        '<div class="msg-attachments" style="width:600px"><div class="sent-images">' +
        '<img id="sent-photo" alt="Photo"><img id="sent-strip" alt="Strip"><img id="sent-tall" alt="Tall"></div></div>';
      document.body.appendChild(host);
      // Sources are set after the pictures are in the page, so the frame is
      // made before they load, as in a document that is still drawing.
      const set = (id, src) => { document.getElementById(id).src = src; };
      ['attr-pct', 'style-pct', 'attr-px', 'attr-px-wide', 'no-width', 'figure-max', 'page-pct'].forEach(id => set(id, p.wide));
      set('small', p.small); set('height-only', p.wide);
      set('sent-photo', p.photo); set('sent-strip', p.strip); set('sent-tall', p.tall);
      // An SVG with a viewBox and no width or height has a shape but no size.
      set('ratio-only', 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 1909 394"><rect width="1909" height="394" fill="#8ac"/></svg>'));
      // One picture already loaded before it reaches the page.
      const loaded = new Image(); loaded.id = 'loaded-pct'; loaded.alt = 'Loaded Percent'; loaded.setAttribute('width', '80%');
      loaded.src = p.wide; await loaded.decode();
      document.getElementById('loaded-slot').appendChild(loaded);
    }, pictures);
    await page.waitForFunction(() => [...document.querySelectorAll('#frame-width-fixture img')].every(img => img.complete && img.naturalWidth && img.parentElement.classList.contains('media-image-frame')));
    await page.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
    const m = await page.evaluate(() => {
      const out = {};
      document.querySelectorAll('#frame-width-fixture img').forEach(img => {
        const frame = img.parentElement, button = frame.querySelector('.media-expand');
        const i = img.getBoundingClientRect(), f = frame.getBoundingClientRect(), b = button.getBoundingClientRect();
        const column = frame.parentElement.getBoundingClientRect();
        out[img.id] = { img: i.width, imgH: i.height, frame: f.width, left: i.left - f.left, top: i.top - f.top, buttonRight: i.right - b.right, buttonTop: b.top - i.top, column: column.width };
      });
      const scroll = document.getElementById('scroll'), doc = document.getElementById('page');
      out.scroll = { client: scroll.clientWidth, scrollWidth: scroll.scrollWidth, page: doc.getBoundingClientRect().width, table: doc.querySelector('table').getBoundingClientRect().right - scroll.getBoundingClientRect().left };
      return out;
    });
    const near = (a, b) => Math.abs(a - b) <= 1;
    const show = id => JSON.stringify(m[id]);
    for (const id of ['attr-pct', 'style-pct', 'loaded-pct']) ok(id + ': picture at 80% of the 600px column', near(m[id].img, 480), show(id));
    ok('attr-px: picture at its 400px', near(m['attr-px'].img, 400), show('attr-px'));
    ok('attr-px-wide: 900px picture held to the 600px column', near(m['attr-px-wide'].img, 600), show('attr-px-wide'));
    ok('no-width: picture shrinks to the 600px column', near(m['no-width'].img, 600), show('no-width'));
    ok('figure-max: picture capped at 80% of the column', near(m['figure-max'].img, 480), show('figure-max'));
    const content = ['attr-pct', 'style-pct', 'loaded-pct', 'attr-px', 'attr-px-wide', 'no-width', 'figure-max'];
    ok('no frame or picture is wider than its column', content.every(id => m[id].frame <= 600.5 && m[id].img <= 600.5), JSON.stringify(content.map(id => [id, m[id].frame, m[id].img])));
    ok('ratio-only: an SVG with only a viewBox keeps a visible size within the column', m['ratio-only'].img > 100 && m['ratio-only'].imgH > 20 && m['ratio-only'].img <= 600.5, show('ratio-only'));
    ok('small: a 200px picture with no width keeps its 200px', near(m.small.img, 200), show('small'));
    // The document stylesheet sets height:auto on pictures, which outranks
    // a height attribute, so the check is the shape and the column.
    ok('height-only: a picture given only a height keeps its 5:1 shape within the column', Math.abs(m['height-only'].img / m['height-only'].imgH - 5) < 0.05 && m['height-only'].img <= 600.5, show('height-only'));
    const all = content.concat(['ratio-only', 'small', 'height-only','page-pct', 'sent-photo', 'sent-strip', 'sent-tall']);
    ok('every frame hugs its picture', all.every(id => near(m[id].frame, m[id].img) && near(m[id].left, 0) && near(m[id].top, 0)), JSON.stringify(all.map(id => [id, m[id].frame, m[id].img, m[id].left])));
    ok('every expand button sits on its picture\'s top right corner', all.every(id => m[id].buttonRight >= 0 && m[id].buttonRight < 20 && m[id].buttonTop >= 0 && m[id].buttonTop < 20), JSON.stringify(all.map(id => [id, m[id].buttonRight, m[id].buttonTop])));
    ok('the document page stays inside its scroll area', m.scroll.scrollWidth <= m.scroll.client && m.scroll.page <= m.scroll.client + 0.5 && m.scroll.table <= m.scroll.client, JSON.stringify(m.scroll));
    ok('the page picture is 80% of the page\'s text column', near(m['page-pct'].img, m['page-pct'].column * 0.8), show('page-pct'));
    ok('chat thumbnails stay within 240 by 160', ['sent-photo', 'sent-strip', 'sent-tall'].every(id => m[id].img <= 240.5 && m[id].imgH <= 160.5), JSON.stringify(['sent-photo', 'sent-strip', 'sent-tall'].map(id => [id, m[id].img, m[id].imgH])));
    await t.shot('media-frame-width');
  } finally { await t.close(); }
})();
