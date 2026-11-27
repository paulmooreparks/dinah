// Compare actual masked paint with an otherwise identical neutral glyph.
const { launch, OUT, FRONTEND } = require('./lib/page');
const png = require('./lib/png'), fs = require('fs'), path = require('path');
const skins = JSON.parse(fs.readFileSync(path.join(FRONTEND, 'pudl/pudl-skins.json'))).skins.filter(s => s.admitted);
(async () => {
  const t = await launch({ viewport: { width: 1600, height: 1000 } }), p = t.page, observations = [];
  try {
    await p.evaluate(() => {
      const panel = document.createElement('section'); panel.id = 'identity-paint-audit';
      // Compare ink on a flat substrate. Gradient raster differences are
      // outside this check; the zero-difference predicate stays exact.
      const style = document.createElement('style'); style.textContent = '#identity-paint-audit,#identity-paint-audit *{transition:none!important;animation:none!important}#identity-paint-audit button{background-image:none!important}'; document.head.append(style);
      Object.assign(panel.style, { position: 'fixed', left: '32px', top: '100px', zIndex: '2147483647', padding: '20px', background: 'var(--surface)', display: 'grid', gridTemplateColumns: '180px', gap: '8px' });
      for (const kind of ['conversation', 'document', 'files', 'tools']) for (const state of ['ordinary', 'current', 'disabled']) {
        const source = document.querySelector('#dinah-launcher .glyph[data-identity-kind="' + kind + '"]');
        if (!source) throw Error('Missing authored launcher glyph for ' + kind);
        {
          const button = document.createElement('button'); button.type = 'button'; button.className = 'btn'; button.tabIndex = -1;
          button.dataset.auditKind = kind; button.dataset.auditState = state;
          Object.assign(button.style, { width: '180px', height: '38px', justifyContent: 'flex-start', transition: 'none', animation: 'none' });
          if (state === 'current') button.setAttribute('aria-current', 'page');
          if (state === 'disabled') button.disabled = true;
          const glyph = source.cloneNode(true);
          button.append(glyph, document.createTextNode(kind)); panel.append(button);
        }
      }
      document.body.append(panel);
      // Translucent skins must not sample a blinking terminal or live applet
      // behind the audit. Keep layout, but suppress those unrelated paints.
      [...document.body.children].filter(el => el !== panel).forEach(el => { el.style.visibility = 'hidden'; });
    });
    for (const skin of skins) for (const mode of ['light', 'dark']) {
      await p.evaluate(({ id, mode }) => { pudlSetTheme(mode); pudlSkins.select(id); }, { id: skin.id, mode });
      await p.waitForFunction(id => pudlSkins.current() === id, skin.id);
      await p.evaluate(() => document.fonts.ready);
      await p.evaluate(() => Promise.all(document.getAnimations().filter(a => Number.isFinite(a.effect.getTiming().iterations)).map(a => a.finished.catch(() => {}))));
      await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
      await p.locator('#identity-paint-audit button').evaluateAll(buttons => buttons.forEach(button => { button.querySelector('.glyph').dataset.identityKind = button.dataset.auditKind; }));
      await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
      const measure = panel => {
        const base = panel.getBoundingClientRect();
        return [...panel.querySelectorAll('button')].map(button => {
          const el = button.querySelector('.glyph'), r = el.getBoundingClientRect(), style = getComputedStyle(el);
          return { kind: button.dataset.auditKind, state: button.dataset.auditState, rect: [r.x - base.x, r.y - base.y, r.width, r.height], color: style.color, paint: style.backgroundColor, mask: style.maskImage };
        });
      };
      const bounds = await p.locator('#identity-paint-audit').evaluate(measure);
      const shot = await p.locator('#identity-paint-audit').screenshot({ animations: 'disabled' });
      fs.writeFileSync(path.join(OUT, 'identity-ink-' + skin.id + '-' + mode + '.png'), shot);
      await p.locator('#identity-paint-audit .glyph').evaluateAll(els => els.forEach(el => el.removeAttribute('data-identity-kind')));
      await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
      const neutralBounds = await p.locator('#identity-paint-audit').evaluate(measure);
      const neutralShot = await p.locator('#identity-paint-audit').screenshot({ animations: 'disabled' });
      fs.writeFileSync(path.join(OUT, 'identity-ink-' + skin.id + '-' + mode + '-neutral.png'), neutralShot);
      const pixels = png(shot), neutralPixels = png(neutralShot);
      for (let i = 0; i < bounds.length; i++) {
        const a = bounds[i], b = neutralBounds[i]; let differences = 0;
        for (let y = 1; y < Math.floor(a.rect[3]) - 1; y++) for (let x = 1; x < Math.floor(a.rect[2]) - 1; x++) {
          if (String(pixels.pixel(Math.round(a.rect[0]) + x, Math.round(a.rect[1]) + y)) !== String(neutralPixels.pixel(Math.round(b.rect[0]) + x, Math.round(b.rect[1]) + y))) differences++;
        }
        const entry = { skin: skin.id, mode, ...a, neutral: b, differences }; observations.push(entry);
        t.ok('paint ' + skin.id + ' ' + mode + ' ' + a.kind + ' ' + a.state, a.state === 'ordinary' ? differences > 0 : differences === 0, JSON.stringify(entry));
        t.ok('classification preserves glyph geometry ' + skin.id + ' ' + mode + ' ' + a.kind + ' ' + a.state, a.rect[2] > 0 && a.rect[3] > 0 && JSON.stringify(a.rect) === JSON.stringify(b.rect));
      }
    }
  } finally {
    fs.writeFileSync(path.join(OUT, 'identity-ink-observations.json'), JSON.stringify(observations, null, 2));
    await t.close();
  }
})().catch(error => { console.error(error); process.exitCode = 1; });
