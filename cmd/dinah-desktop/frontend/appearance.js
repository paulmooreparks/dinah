/* Dinah owns theme documents and persistence; PUDL owns mode and tokens. */
(function () {
  'use strict';
  const copy = value => JSON.parse(JSON.stringify(value));
  const builtins = copy(window.dinahThemeBuiltins), base = builtins.mineral;
  const roles = ['interface', 'heading', 'reading', 'monospace'];
  const kinds = ['chat', 'editor', 'preview', 'terminal'];
  const keys = Object.keys(base.palettes.light), cacheKey = 'dinah-appearance-cache-v1';
  // The theme used until the person chooses another is the product's
  // default (product.js), or the first built-in when there is no such
  // built-in theme.
  const defaultTheme = () => {
    const id = window.dinahProduct && window.dinahProduct.defaultTheme;
    return id && Object.prototype.hasOwnProperty.call(builtins, id) ? id : Object.keys(builtins)[0];
  };
  let state = { version: 1, active: defaultTheme(), themes: [], overrides: {} }, revision = '', error = '', notice = '', serial = 0;
  const forcedColours = matchMedia('(forced-colors: active)');
  let saving = Promise.resolve(), resolveReady, appliedMode;
  const ready = new Promise(resolve => { resolveReady = resolve; });
  function bytes(value) { return new TextEncoder().encode(value).length; }
  function face(value, role) {
    if (!value || Object.keys(value).some(k => !['families', 'fallback'].includes(k)) || !Array.isArray(value.families) || value.families.length > 8) throw Error('Invalid font definition.');
    if (['heading', 'reading'].includes(role) && value.fallback === 'inherit' && !value.families.length) return;
    if (!['sans-serif', 'system-ui', 'serif', 'monospace'].includes(value.fallback)) throw Error('Invalid font fallback.');
    if (role === 'interface' && !['sans-serif', 'system-ui'].includes(value.fallback)) throw Error('Interface fonts need a sans-serif fallback.');
    if (['monospace', 'editor', 'terminal'].includes(role) && value.fallback !== 'monospace') throw Error('Code and terminal fonts need a monospace fallback.');
    value.families.forEach(name => { if (typeof name !== 'string' || !name.trim() || bytes(name) > 128 || /[\x00-\x1f\x7f-\x9f]|url\(/i.test(name)) throw Error('Invalid font family name.'); });
  }
  function validate(doc) {
    if (!doc || doc.format !== 'dinah-theme' || doc.version !== 1 || Object.keys(doc).some(k => !['format', 'version', 'name', 'typography', 'palettes', 'extensions'].includes(k))) throw Error('Unsupported theme format or version.');
    if (typeof doc.name !== 'string' || !doc.name.trim() || bytes(doc.name) > 100) throw Error('A theme name must contain 1 to 100 bytes.');
    if (!doc.typography || Object.keys(doc.typography).length !== 4) throw Error('All four font roles are required.');
    roles.forEach(role => face(doc.typography[role], role));
    if (!doc.palettes || Object.keys(doc.palettes).length !== 2) throw Error('Light and dark palettes are required.');
    ['light', 'dark'].forEach(mode => {
      const p = doc.palettes[mode];
      if (!p || Object.keys(p).length !== keys.length) throw Error('Incomplete ' + mode + ' palette.');
      keys.forEach(key => { const value = p[key]; if (typeof value !== 'string' || (['lit', 'depth'].includes(key) ? !/^(?:100|[0-9]{1,2})(?:\.[0-9]{1,2})?%$/.test(value) || parseFloat(value) > 100 : !/^#[0-9a-f]{6}$/i.test(value))) throw Error('Invalid ' + mode + ' ' + key + '.'); });
    });
    if (doc.extensions !== undefined && (!doc.extensions || typeof doc.extensions !== 'object' || Array.isArray(doc.extensions))) throw Error('Theme extensions must be an object.');
    if (bytes(JSON.stringify(doc)) > 256 * 1024) throw Error('Theme exceeds 256 KiB.');
    return doc;
  }
  const rgb = value => [1, 3, 5].map(i => parseInt(value.slice(i, i + 2), 16) / 255);
  const luminance = value => value.reduce((sum, n, i) => sum + (n <= .04045 ? n / 12.92 : ((n + .055) / 1.055) ** 2.4) * [.2126, .7152, .0722][i], 0);
  function ratio(a, b) { const x = luminance(a), y = luminance(b); return (Math.max(x, y) + .05) / (Math.min(x, y) + .05); }
  function contrast(doc) {
    validate(doc); const failures = [];
    ['light', 'dark'].forEach(mode => {
      const p = doc.palettes[mode], lit = parseFloat(p.lit) / 100;
      const input = rgb(p.light).map((v, i) => v * lit + rgb(p.surface)[i] * (1 - lit));
      const backgrounds = [rgb(p.bg), rgb(p.surface), rgb(p['surface-alt']), input];
      keys.filter(key => ['text', 'text-muted', 'accent', 'warn', 'danger', 'positive', 'dinah-skill', 'dinah-title'].includes(key) || key.startsWith('syntax-')).forEach(key => { if (backgrounds.some(bg => ratio(rgb(p[key]), bg) < 4.5)) failures.push(mode + ' ' + key + ' needs 4.5:1 contrast'); });
      if (ratio(rgb(p['on-accent']), rgb(p.accent)) < 4.5) failures.push(mode + ' primary action text needs 4.5:1');
      if (ratio(rgb(p['on-warn']), rgb(p.warn)) < 4.5) failures.push(mode + ' warning badge text needs 4.5:1');
      if (ratio(rgb(p.border), input) < 3) failures.push(mode + ' input boundary needs 3:1');
      const inputBorder = rgb(p.text).map((v, i) => v * .55 + rgb(p.surface)[i] * .45);
      if (ratio(inputBorder, input) < 3) failures.push(mode + ' derived input boundary needs 3:1');
      if (ratio(rgb(p['on-accent']), rgb(p['accent-hover'])) < 4.5) failures.push(mode + ' primary hover text needs 4.5:1');
      const danger = rgb(p['danger-hover']);
      if (ratio(rgb(p['on-accent']), danger) < 4.5 || ratio(rgb(p['on-accent']), danger.map(v => v * .78 + .22)) < 4.5) failures.push(mode + ' destructive hover text needs 4.5:1');
      if (ratio(rgb(p['tb-fg']), rgb(p['tb-bg'])) < 4.5) failures.push(mode + ' topbar text needs 4.5:1');
      failures.push(...categoryFailures(mode, p));
    });
    return failures;
  }
  /* Category colours (PUDL 0.91.0, specification 0.48.0, chapter 3) tell
     things apart without saying anything about their state. Every Dinah
     theme sets all eight in both forms, and they keep the rules checked
     here, which theme_categories.go checks in the same way. Distance is in
     OKLab multiplied by 100, and colour vision is simulated with the
     matrices of Machado, Oliveira and Fernandes (2009) at full severity. */
  const categoryKeys = [1, 2, 3, 4, 5, 6, 7, 8].map(n => 'category-' + n);
  const linear = n => n <= .04045 ? n / 12.92 : ((n + .055) / 1.055) ** 2.4;
  function oklab([r, g, b]) {
    const l = Math.cbrt(.4122214708 * r + .5363325363 * g + .0514459929 * b), m = Math.cbrt(.2119034982 * r + .6806995451 * g + .1073969566 * b), s = Math.cbrt(.0883024619 * r + .2817188376 * g + .6299787005 * b);
    return [.2104542553 * l + .7936177850 * m - .0040720468 * s, 1.9779984951 * l - 2.4285922050 * m + .4505937099 * s, .0259040371 * l + .7827717662 * m - .8086757660 * s];
  }
  const distanceLinear = (a, b) => { const x = oklab(a), y = oklab(b); return 100 * Math.hypot(x[0] - y[0], x[1] - y[1], x[2] - y[2]); };
  const distance = (a, b) => distanceLinear(a.map(linear), b.map(linear));
  const hue = c => { const [, a, b] = oklab(c.map(linear)); return (Math.atan2(b, a) * 180 / Math.PI + 360) % 360; };
  const hueGap = (a, b) => { const d = Math.abs(hue(a) - hue(b)) % 360; return Math.min(d, 360 - d); };
  const vision = [[[.152286, 1.052583, -.204868], [.114503, .786281, .099216], [-.003882, -.048116, 1.051998]], [[.367322, .860646, -.227968], [.280085, .672501, .047413], [-.011820, .042940, .968881]]];
  const simulatedDistance = (a, b) => Math.min(...vision.map(m => { const see = c => { const l = c.map(linear); return m.map(row => Math.min(1, Math.max(0, row[0] * l[0] + row[1] * l[1] + row[2] * l[2]))); }; return distanceLinear(see(a), see(b)); }));
  // bg, surface and input-bg, which PUDL makes by mixing light into surface by lit.
  function categoryBases(p) { const lit = parseFloat(p.lit) / 100, surface = rgb(p.surface); return [rgb(p.bg), surface, rgb(p.light).map((v, i) => v * lit + surface[i] * (1 - lit))]; }
  function slotFailures(mode, p, slot, c) {
    const failures = [], key = categoryKeys[slot];
    if (categoryBases(p).some(base => ratio(c, base) < 3)) failures.push(mode + ' ' + key + ' needs 3:1 contrast');
    ['warn', 'danger', 'positive'].forEach(status => { if (distance(c, rgb(p[status])) < 12 || hueGap(c, rgb(p[status])) < 20) failures.push(mode + ' ' + key + ' is too close to ' + status); });
    if (distance(c, rgb(p.accent)) < 8) failures.push(mode + ' ' + key + ' is too close to accent');
    return failures;
  }
  const neighbourFailure = (mode, slot, a, b) => distance(a, b) < 15 || simulatedDistance(a, b) < 8 ? mode + ' ' + categoryKeys[slot] + ' and ' + categoryKeys[slot + 1] + ' are too alike' : '';
  function categoryFailures(mode, p) {
    const colours = categoryKeys.map(key => rgb(p[key])), failures = colours.flatMap((c, slot) => slotFailures(mode, p, slot, c));
    for (let slot = 0; slot < 7; slot++) { const failure = neighbourFailure(mode, slot, colours[slot], colours[slot + 1]); if (failure) failures.push(failure); }
    return failures;
  }
  /* A theme saved or exported before category colours gets them as
     fillThemeCategories in theme_categories.go gives them: a palette with
     none of the eight takes those of the built-in palette of the same form
     whose bg, surface and input-bg are nearest its own, and a slot that then
     breaks a rule takes the nearest built-in category colour that keeps them.
     A palette with some of the eight is left for validation to refuse. The
     contrast check reports any rule the filled colours still break. */
  function fillCategories(doc) {
    const ids = Object.keys(builtins).sort(), hex = /^#[0-9a-f]{6}$/i;
    ['light', 'dark'].forEach(mode => {
      const p = doc?.palettes?.[mode];
      if (!p || categoryKeys.some(key => key in p) || !['bg', 'surface', 'light'].every(key => hex.test(p[key])) || !/^[0-9.]+%$/.test(p.lit)) return;
      const own = categoryBases(p), pool = [];
      let nearest = ids[0], best = Infinity;
      ids.forEach(id => {
        const other = builtins[id].palettes[mode], theirs = categoryBases(other), d = own.reduce((sum, c, i) => sum + distance(c, theirs[i]), 0);
        if (d < best) { nearest = id; best = d; }
        categoryKeys.forEach(key => { const value = other[key].toLowerCase(); if (!pool.includes(value)) pool.push(value); });
      });
      const chosen = categoryKeys.map(key => builtins[nearest].palettes[mode][key].toLowerCase());
      const fits = (slot, value) => !slotFailures(mode, p, slot, rgb(value)).length && (slot === 0 || !neighbourFailure(mode, slot - 1, rgb(chosen[slot - 1]), rgb(value)));
      chosen.forEach((value, slot) => {
        if (fits(slot, value)) return;
        const original = rgb(value), found = pool.map((candidate, index) => ({ candidate, index, d: distance(rgb(candidate), original) })).sort((a, b) => a.d - b.d || a.index - b.index).find(({ candidate }) => fits(slot, candidate));
        if (found) chosen[slot] = found.candidate;
      });
      categoryKeys.forEach((key, slot) => { p[key] = chosen[slot]; });
    });
    return doc;
  }
  /* PUDL's diff tints (0.93.0, specification 0.50.0, the Diffs chapter):
     every syntax colour reaches 4.5:1 on diff-add-bg and diff-del-bg laid
     over input-bg, and each sign, in positive or danger, reaches 3:1 on its
     line's fill. A theme with other syntax colours than PUDL's sets its own
     amount, so the window uses, for each theme and mode, the strongest
     amount up to PUDL's 10 percent, in half-percent steps, at which both
     rules hold. At no tint both hold, since the contrast check asks 4.5:1 of
     the syntax and status colours on input-bg. */
  function diffTints(p) {
    const lit = parseFloat(p.lit) / 100, input = rgb(p.light).map((v, i) => v * lit + rgb(p.surface)[i] * (1 - lit));
    const syntax = keys.filter(key => key.startsWith('syntax-')).map(key => rgb(p[key]));
    const amount = colour => {
      const c = rgb(colour);
      for (let percent = 10; percent > 0; percent -= .5) {
        const tint = c.map((v, i) => v * percent / 100 + input[i] * (1 - percent / 100));
        if (ratio(c, tint) >= 3 && syntax.every(s => ratio(s, tint) >= 4.5)) return percent;
      }
      return 0;
    };
    return { add: amount(p.positive), del: amount(p.danger) };
  }
  function font(value, role) { face(value, role); return value.fallback === 'inherit' ? 'var(--font)' : value.families.map(name => JSON.stringify(name)).concat(value.fallback).join(', '); }
  function documentFor(id, source = state) { return Object.prototype.hasOwnProperty.call(builtins, id) ? builtins[id] : source.themes.find(t => t.id === id)?.document; }
  function snapshot() { return copy({ state, revision, error, notice }); }
  function content(kind) {
    const doc = documentFor(state.active), override = state.overrides[kind] || {}, role = ['editor', 'terminal'].includes(kind) ? 'monospace' : 'reading';
    return { font: font(override.font || doc.typography[role], override.font ? kind : role), size: override.size || (kind === 'chat' ? 15 : 14) };
  }
  function validateOverrides(values) {
    if (!values || typeof values !== 'object' || Array.isArray(values) || Object.keys(values).some(k => !kinds.includes(k))) throw Error('Invalid applet overrides.');
    Object.entries(values).forEach(([kind, value]) => {
      if (!value || Object.keys(value).some(k => !['font', 'size'].includes(k))) throw Error('Invalid applet font override.');
      if (value.font) face(value.font, kind);
      if (value.size !== undefined && (!Number.isInteger(value.size) || value.size < 6 || value.size > (kind === 'terminal' ? 22 : 32))) throw Error('Content text size is out of range.');
    });
  }
  function apply(notify = true) {
    const doc = documentFor(state.active) || base, mode = document.documentElement.dataset.theme === 'dark' ? 'dark' : 'light', style = document.documentElement.style;
    if (notify) document.dispatchEvent(new Event('dinah:appearance-before'));
    appliedMode = mode;
    // In forced colours PUDL makes every category colour the reader's text
    // colour from its stylesheet, which a value set here would override.
    const forced = forcedColours.matches;
    Object.entries(doc.palettes[mode]).forEach(([key, value]) => { if (forced && key.startsWith('category-')) style.removeProperty('--' + key); else style.setProperty('--' + key, value); });
    const tints = diffTints(doc.palettes[mode]);
    style.setProperty('--diff-add-bg', 'color-mix(in srgb, var(--diff-add) ' + tints.add + '%, transparent)');
    style.setProperty('--diff-del-bg', 'color-mix(in srgb, var(--diff-del) ' + tints.del + '%, transparent)');
    style.setProperty('--font', font(doc.typography.interface, 'interface'));
    style.setProperty('--font-display', font(doc.typography.heading, 'heading'));
    style.setProperty('--mono', font(doc.typography.monospace, 'monospace'));
    style.setProperty('--font-mono', 'var(--mono)');
    style.setProperty('--reading-font', font(doc.typography.reading, 'reading'));
    kinds.forEach(kind => { const value = content(kind); style.setProperty('--' + kind + '-font', value.font); style.setProperty('--' + kind + '-size', value.size + 'px'); });
    const current = ++serial;
    if (notify) {
      // A theme sets its fonts as inline styles on the page, which change the
      // width of every toolbar's controls, and PUDL does not watch inline
      // styles (0.93.1), so each folding toolbar is folded again here, once
      // for the theme and again when its fonts have loaded.
      const refold = () => { if (window.pudlToolbar) document.querySelectorAll('.toolbar[data-toolbar-fold]').forEach(bar => window.pudlToolbar.fit(bar)); };
      const announce = () => { if (current === serial) { refold(); document.dispatchEvent(new CustomEvent('dinah:appearance-change', { detail: { revision: current } })); } };
      if (window.dinahRender) dinahRender.enqueue(apply, null, announce, null, 'appearance'); else requestAnimationFrame(announce);
      // Font completion only requests another measure; it never installs an older theme.
      const loads = roles.map(role => document.fonts.load('14px ' + (doc.typography[role].fallback === 'inherit' ? font(doc.typography.interface, 'interface') : font(doc.typography[role], role))));
      Promise.allSettled(loads).then(() => { if (current === serial && window.dinahRender) dinahRender.enqueue(apply, null, announce, null, 'appearance font measure'); });
    }
  }
  function cache() { try { localStorage.setItem(cacheKey, JSON.stringify({ document: documentFor(state.active), overrides: state.overrides })); } catch (_) { /* Native preferences remain authoritative. */ } }
  try {
    const saved = JSON.parse(localStorage.getItem(cacheKey) || 'null');
    if (saved) { validate(fillCategories(saved.document)); validateOverrides(saved.overrides); if (contrast(saved.document).length) throw Error('Invalid cached palette.'); state = { version: 1, active: 'startup-cache', themes: [{ id: 'startup-cache', document: saved.document }], overrides: saved.overrides }; apply(false); }
  } catch (_) { state = { version: 1, active: defaultTheme(), themes: [], overrides: {} }; }
  document.addEventListener('pudl:theme-change', () => apply());
  forcedColours.addEventListener('change', () => apply());
  new MutationObserver(() => { if (document.documentElement.dataset.theme !== appliedMode) apply(); }).observe(document.documentElement, { attributes: true, attributeFilter: ['data-theme'] });
  async function initialize() {
    const app = window.dinahHelpers.app(); notice = '';
    try {
      if (app?.ReadAppearance) {
        const result = await app.ReadAppearance();
        if (result.state.version !== 1 || !Array.isArray(result.state.themes) || !documentFor(result.state.active, result.state)) throw Error('Unsupported appearance library.');
        result.state.themes.forEach(record => validate(record.document)); validateOverrides(result.state.overrides);
        if (contrast(documentFor(result.state.active, result.state)).length) throw Error('Saved active theme has invalid contrast.');
        // The notice says what reading changed, such as an old theme in use
        // whose filled category colours did not pass (appearance.go).
        state = result.state; revision = result.revision; notice = result.notice || '';
      } else state = { version: 1, active: defaultTheme(), themes: [], overrides: {} };
      apply(); cache();
    } catch (e) { error = String(e.message || e); state = { version: 1, active: defaultTheme(), themes: [], overrides: {} }; apply(); }
    resolveReady(); document.dispatchEvent(new Event('dinah:appearance-library'));
  }
  if (document.readyState === 'loading') document.addEventListener('DOMContentLoaded', initialize, { once: true }); else initialize();
  function transact(change) {
    const job = saving.then(async () => {
      await ready;
      if (error) throw Error(error + ' Reload appearance preferences before saving.');
      const app = window.dinahHelpers.app(); if (!app?.SaveAppearance) throw Error('Appearance persistence is unavailable.');
      const next = copy(state), result = change(next); validateOverrides(next.overrides);
      next.themes.forEach(record => validate(record.document));
      const active = documentFor(next.active, next); if (!active) throw Error('Theme does not exist.');
      const failures = contrast(active); if (failures.length) throw Error(failures.join('; '));
      const saved = await app.SaveAppearance(next, revision);
      state = saved.state; revision = saved.revision; apply(); cache(); document.dispatchEvent(new Event('dinah:appearance-library')); return result;
    });
    saving = job.catch(() => {}); return job;
  }
  function newID() { return 'theme-' + [...crypto.getRandomValues(new Uint8Array(12))].map(n => n.toString(16).padStart(2, '0')).join(''); }
  function save(doc, id, activate) {
    validate(doc);
    return transact(next => {
      const existing = next.themes.find(t => t.id === id), same = existing && JSON.stringify(existing.document) === JSON.stringify(doc);
      if (activate && Object.prototype.hasOwnProperty.call(builtins, id) && JSON.stringify(builtins[id]) === JSON.stringify(doc)) { next.active = id; return id; }
      // Save keeps an active snapshot intact; Apply updates a personal theme in place.
      if (!existing || (next.active === id && !same && !activate)) id = newID();
      const found = next.themes.find(t => t.id === id);
      if (found) found.document = copy(doc); else {
        if (next.themes.length >= 32) throw Error('The theme library is full (32 personal themes).');
        const names = Object.values(builtins).concat(next.themes.map(t => t.document)).map(t => t.name);
        const added = copy(doc); let suffix = 2; while (names.includes(added.name)) { let base = doc.name; const ending = ' (' + suffix++ + ')'; while (new TextEncoder().encode(base + ending).length > 100) base = [...base].slice(0, -1).join(''); added.name = base + ending; }
        next.themes.push({ id, document: added });
      }
      if (activate) next.active = id;
      return id;
    });
  }
  window.dinahAppearance = { ready, builtins: copy(builtins), validate, contrast, fillCategories, categoryKeys, diffTints, font, snapshot, content, keys, defaultTheme,
    document: id => documentFor(id) ? copy(documentFor(id)) : null, save,
    select: id => transact(next => { next.active = id; }),
    overrides: values => { validateOverrides(values); return transact(next => { next.overrides = copy(values); }); },
    remove: id => transact(next => { if (Object.prototype.hasOwnProperty.call(builtins, id)) throw Error('Built-in themes cannot be deleted.'); if (next.active === id) next.active = defaultTheme(); next.themes = next.themes.filter(t => t.id !== id); }),
    reload: () => { const job = saving.then(async () => { error = ''; await initialize(); if (error) throw Error(error); }); saving = job.catch(() => {}); return job; }
  };
})();
