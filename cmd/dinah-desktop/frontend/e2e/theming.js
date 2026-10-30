// The appearance library under the shell's content security policy: the
// built-in themes and their contrast and category colours, choosing a theme
// without losing the composer's draft, font overrides, the theme editor,
// export and import, failed saves, Your Themes and deleting, and the narrow
// fit.
const { launch, PAGE, FRONTEND } = require('./lib/page');
const path = require('path');
const fs = require('fs');
// The policy the shell serves the page with.
const CSP = "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data:; media-src 'self' blob:; connect-src 'self'; worker-src 'self' blob:; frame-src 'self' blob:; object-src 'none'; base-uri 'none'; form-action 'self'; frame-ancestors 'self'";
(async () => {
  let state = { version: 1, active: 'mineral', themes: [], overrides: {} }, revision = 0, fail = false, ui = { version: 1, values: {} };
  const t = await launch({
    viewport: { width: 1440, height: 1000 },
    app: () => ({
      SavedConversation: async () => ({ available: false }),
      OpenSessionAt: async (cwd, sessionId) => ({ sessionId, cwd, provider: 'codex', title: 'Appearance work', open: true }),
      ReadAppearance: () => __themeAPI('read'), SaveAppearance: (value, expected) => __themeAPI('save', value, expected),
      ReadUIState: () => __themeAPI('read-ui'), SaveUIState: value => __themeAPI('save-ui', value),
    }),
    goto: false,
    timeout: 30000,
  });
  const { page, ok } = t;
  try {
    // The page is served under the shell's policy; this route, added last,
    // is asked first.
    await t.context.route('http://dinah.test/**', route => route.fulfill({ path: path.join(FRONTEND, new URL(route.request().url()).pathname), headers: { 'Content-Security-Policy': CSP } }));
    await page.exposeFunction('__themeAPI', (method, value, expected) => {
      if (method === 'fail') { fail = true; return; }
      if (method === 'read-ui') return ui;
      if (method === 'save-ui') { ui = value; return; }
      if (method === 'save') { if (fail) { fail = false; throw Error('Theme storage unavailable'); } if (String(revision) !== expected) throw Error('Revision conflict'); state = value; revision++; }
      return { state, revision: String(revision) };
    });
    await page.goto(PAGE);
    await page.waitForFunction(() => window.dinahAppearance && window.dinahWorkbench);
    await page.evaluate(() => dinahAppearance.ready);
    ok('View exposes built-in themes and direct editor access', await page.locator('#host-themes').evaluate(el => el.textContent.includes('Mineral (built in)') && el.textContent.includes('Linen (built in)') && el.textContent.includes('Edit Current Theme')));
    ok('Mineral is the persisted-library default', await page.evaluate(() => dinahAppearance.snapshot().state.active === 'mineral'));
    // Solarized, Nord, Gruvbox and Graphite are built in too (27 October
    // 2026), and One Dark Pro, Dracula, Catppuccin, Tokyo Night and GitHub
    // (28 October 2026), each passing the page's own contrast check in both
    // modes.
    const added = { solarized: 'Solarized', nord: 'Nord', gruvbox: 'Gruvbox', graphite: 'Graphite', 'one-dark-pro': 'One Dark Pro', dracula: 'Dracula', catppuccin: 'Catppuccin', 'tokyo-night': 'Tokyo Night', github: 'GitHub' };
    ok('the View menu lists the added themes', await page.locator('#host-themes').evaluate((el, names) => names.every(n => el.textContent.includes(n + ' (built in)')), Object.values(added)));
    ok('every built-in theme passes the contrast check', await page.evaluate(() => Object.values(dinahAppearance.builtins).every(doc => dinahAppearance.contrast(doc).length === 0)));
    // Diff tints (PUDL 0.93.0): the dark syntax colours of Solarized, Nord,
    // Gruvbox, One Dark Pro, Dracula and GitHub were lightened on 28 October
    // 2026 so that their tints show. Every palette now reaches 10 percent but
    // Tokyo Night's dark added lines, at 9, and Dracula's, at 8, where 10
    // would have made its pink and purple pastel.
    const tints = await page.evaluate(() => Object.fromEntries(Object.entries(dinahAppearance.builtins).flatMap(([id, doc]) => ['light', 'dark'].map(m => [id + ' ' + m, dinahAppearance.diffTints(doc.palettes[m])]))));
    const lower = { 'tokyo-night dark': { add: 9, del: 10 }, 'dracula dark': { add: 8, del: 10 } };
    const tintsExpected = Object.entries(tints).every(([key, t]) => t.add === (lower[key]?.add ?? 10) && t.del === (lower[key]?.del ?? 10));
    ok('every built-in palette has the diff tints expected of it', tintsExpected);
    if (!tintsExpected) console.log(JSON.stringify(tints));
    // Category colours (PUDL 0.91.0): all 22 built-in palettes set the
    // eight, the check refuses one that breaks a rule, an old theme without
    // them gets them, a copy of Nord getting Nord's, and in forced colours
    // they are the reader's text colour.
    ok('every built-in palette sets all eight category colours', await page.evaluate(() => Object.values(dinahAppearance.builtins).flatMap(doc => [doc.palettes.light, doc.palettes.dark]).filter(p => dinahAppearance.categoryKeys.every(k => /^#[0-9a-f]{6}$/i.test(p[k]))).length === 22));
    ok('the contrast check refuses a category colour that breaks a rule', await page.evaluate(() => {
      const doc = dinahAppearance.document('mineral'), failures = () => dinahAppearance.contrast(doc).join('; ');
      doc.palettes.light['category-1'] = doc.palettes.light.surface; const low = failures().includes('light category-1 needs 3:1');
      doc.palettes.light['category-1'] = doc.palettes.light.danger; const near = failures().includes('light category-1 is too close to danger');
      doc.palettes.light['category-1'] = doc.palettes.light['category-2']; const alike = failures().includes('light category-1 and category-2 are too alike');
      return low && near && alike;
    }));
    ok('a theme without category colours gets those of the nearest built-in palette', await page.evaluate(() => {
      const nord = dinahAppearance.document('nord'), old = dinahAppearance.document('nord');
      ['light', 'dark'].forEach(m => dinahAppearance.categoryKeys.forEach(k => delete old.palettes[m][k]));
      dinahAppearance.fillCategories(old);
      return ['light', 'dark'].every(m => dinahAppearance.categoryKeys.every(k => old.palettes[m][k] === nord.palettes[m][k]));
    }));
    ok('a theme with other surfaces gets category colours that pass', await page.evaluate(() => {
      const old = dinahAppearance.document('mineral');
      ['light', 'dark'].forEach(m => dinahAppearance.categoryKeys.forEach(k => delete old.palettes[m][k]));
      Object.assign(old.palettes.light, { bg: '#f7f2fa', surface: '#ffffff' }); Object.assign(old.palettes.dark, { bg: '#000000', surface: '#101418' });
      dinahAppearance.fillCategories(old); dinahAppearance.validate(old);
      return !dinahAppearance.contrast(old).some(f => f.includes('category'));
    }));
    ok('the window gives the theme\'s category colours', await page.evaluate(() => getComputedStyle(document.documentElement).getPropertyValue('--category-1').trim() === dinahAppearance.builtins.mineral.palettes.light['category-1']));
    await page.emulateMedia({ forcedColors: 'active' });
    await page.waitForFunction(() => getComputedStyle(document.documentElement).getPropertyValue('--category-1').trim().toLowerCase() === 'canvastext');
    ok('in forced colours every category colour is the reader\'s text colour', await page.evaluate(() => dinahAppearance.categoryKeys.every(k => getComputedStyle(document.documentElement).getPropertyValue('--' + k).trim().toLowerCase() === 'canvastext')));
    await page.emulateMedia({ forcedColors: 'none' });
    await page.waitForFunction(() => getComputedStyle(document.documentElement).getPropertyValue('--category-1').trim() === dinahAppearance.builtins.mineral.palettes.light['category-1']);
    await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:theme' }));
    const prompt = page.locator('.win[data-chat-current] [data-view-id=prompt]'); await prompt.fill('Unsent theme-test draft');
    await prompt.evaluate(el => { el.setSelectionRange(3, 9); window.__originalComposer = el; });
    await page.evaluate(() => dinahWorkbench.open('settings')); await page.evaluate(() => pudlWindows.snap('settings', 'maximized'));
    await page.locator('#appearance-theme').selectOption('linen');
    await page.waitForFunction(() => dinahAppearance.snapshot().state.active === 'linen');
    ok('choosing Linen applies its light palette', await page.evaluate(() => getComputedStyle(document.body).backgroundColor === 'rgb(234, 229, 218)'));
    // Each added theme can be chosen in Settings and applies its own light
    // background; Linen is chosen again for the checks that follow.
    for (const id of Object.keys(added)) {
      await page.locator('#appearance-theme').selectOption(id);
      await page.waitForFunction(id => dinahAppearance.snapshot().state.active === id, id);
      ok('choosing ' + added[id] + ' applies its light palette', await page.evaluate(id => {
        const hex = dinahAppearance.builtins[id].palettes.light.bg, rgb = [1, 3, 5].map(i => parseInt(hex.slice(i, i + 2), 16)).join(', ');
        return getComputedStyle(document.body).backgroundColor === 'rgb(' + rgb + ')';
      }, id));
    }
    await page.locator('#appearance-theme').selectOption('linen');
    await page.waitForFunction(() => dinahAppearance.snapshot().state.active === 'linen');
    ok('theme application preserves the composer node, draft and selection', await prompt.evaluate(el => el === __originalComposer && el.value === 'Unsent theme-test draft' && el.selectionStart === 3 && el.selectionEnd === 9));
    await page.evaluate(() => pudlSetTheme('dark'));
    ok('Linen also supplies a dark palette', await page.evaluate(() => getComputedStyle(document.body).backgroundColor === 'rgb(25, 26, 22)'));
    const check = page.locator('#appearance-overrides label.check').first();
    ok('global-font checkbox sits beside its label', await check.evaluate(el => { const input = el.querySelector('input'), box = input.getBoundingClientRect(), label = el.getBoundingClientRect(); return getComputedStyle(el).display === 'flex' && Math.abs(box.top + box.height / 2 - label.top - label.height / 2) < 3 && box.left - label.left < 4; }));
    await page.getByRole('spinbutton', { name: 'chat text size', exact: true }).filter({ visible: true }).fill('6');
    await page.getByRole('spinbutton', { name: 'chat text size', exact: true }).filter({ visible: true }).press('Tab');
    await page.waitForFunction(() => dinahAppearance.snapshot().state.overrides.chat?.size === 6);
    ok('font suggestions use PUDL combo controls', await page.locator('#appearance-overrides .combo .combo-open').count() === 4);
    ok('6px content size applies without Apply', await prompt.evaluate(el => getComputedStyle(el).fontSize === '6px'));
    await page.getByRole('combobox', { name: 'chat font families', exact: true }).fill('Georgia');
    await page.getByRole('combobox', { name: 'chat font families', exact: true }).press('Tab');
    await page.getByRole('combobox', { name: 'chat font fallback', exact: true }).selectOption('serif');
    await page.getByRole('spinbutton', { name: 'chat text size', exact: true }).filter({ visible: true }).fill('18');
    await page.getByRole('spinbutton', { name: 'chat text size', exact: true }).filter({ visible: true }).press('Tab');
    await page.waitForFunction(() => dinahAppearance.snapshot().state.overrides.chat?.size === 18);
    ok('applet content overrides reach existing composers', await prompt.evaluate(el => getComputedStyle(el).fontFamily.includes('Georgia') && getComputedStyle(el).fontSize === '18px'));
    await page.locator('#appearance-edit').click();
    const editor = page.locator('[data-win="theme-editor"]'); await editor.getByRole('textbox', { name: 'Theme name', exact: true }).waitFor();
    await page.evaluate(() => pudlWindows.snap('theme-editor', 'maximized'));
    await editor.getByRole('textbox', { name: 'Theme name', exact: true }).fill('Night notes');
    await editor.getByRole('textbox', { name: 'text value', exact: true }).fill('#f4efe2'); await editor.getByRole('textbox', { name: 'text value', exact: true }).press('Tab');
    await editor.getByRole('button', { name: 'Save', exact: true }).click();
    await page.waitForFunction(() => dinahAppearance.snapshot().state.themes.length === 1);
    ok('an unreadable draft saves without changing the active theme', await page.evaluate(() => dinahAppearance.snapshot().state.active === 'linen'));
    await editor.getByRole('button', { name: 'Apply', exact: true }).click();
    await editor.getByRole('status').filter({ hasText: 'needs 4.5:1' }).waitFor();
    ok('an unreadable draft cannot be applied', await page.evaluate(() => dinahAppearance.snapshot().state.active === 'linen'));
    await editor.getByRole('textbox', { name: 'text value', exact: true }).fill('#302d24'); await editor.getByRole('textbox', { name: 'text value', exact: true }).press('Tab');
    await editor.getByRole('button', { name: 'Apply', exact: true }).click();
    await page.waitForFunction(() => dinahAppearance.snapshot().state.active.startsWith('theme-'));
    let active = await page.evaluate(() => dinahAppearance.snapshot().state.active);
    ok('valid custom themes apply without clearing personal font overrides', await page.evaluate(() => dinahAppearance.snapshot().state.overrides.chat.size === 18));
    const downloadPromise = page.waitForEvent('download'); await editor.getByRole('button', { name: 'Export', exact: true }).click();
    const downloaded = await (await downloadPromise).path(), exported = JSON.parse(fs.readFileSync(downloaded, 'utf8'));
    ok('export contains both variants and fonts, excluding personal overrides', exported.format === 'dinah-theme' && exported.palettes.light && exported.palettes.dark && exported.typography.monospace && !('overrides' in exported));
    exported.name = 'Imported study'; exported.extensions = { 'example.app': { note: 'retain me' } };
    await editor.locator('input[type=file]').setInputFiles({ name: 'roundtrip.dinah-theme.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(exported)) });
    await editor.getByRole('textbox', { name: 'Theme name', exact: true }).waitFor();
    await page.waitForFunction(() => document.querySelector('[data-win=theme-editor] input[aria-label="Theme name"]').value === 'Imported study');
    await editor.getByRole('button', { name: 'Save', exact: true }).click();
    await page.waitForFunction(() => dinahAppearance.snapshot().state.themes.length === 2);
    ok('import preserves extension data and leaves the active theme unchanged', await page.evaluate(id => dinahAppearance.snapshot().state.active === id && dinahAppearance.snapshot().state.themes.some(t => t.document.extensions?.['example.app']?.note === 'retain me'), active));
    exported.version = 99;
    await editor.locator('input[type=file]').setInputFiles({ name: 'future.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(exported)) });
    await editor.getByRole('status').filter({ hasText: 'Unsupported theme' }).waitFor();
    ok('unsupported imports leave the selected theme intact', await page.evaluate(id => dinahAppearance.snapshot().state.active === id, active));
    // A theme exported before category colours imports with those of the
    // nearest built-in palette, here Linen's, and the editor shows the eight
    // as fields whose breaches the contrast list reports.
    const older = JSON.parse(JSON.stringify(exported)); older.version = 1; older.name = 'Older study'; delete older.extensions;
    ['light', 'dark'].forEach(m => { for (let n = 1; n <= 8; n++) delete older.palettes[m]['category-' + n]; });
    await editor.locator('input[type=file]').setInputFiles({ name: 'older.dinah-theme.json', mimeType: 'application/json', buffer: Buffer.from(JSON.stringify(older)) });
    await page.waitForFunction(() => document.querySelector('[data-win=theme-editor] input[aria-label="Theme name"]').value === 'Older study');
    await editor.getByRole('combobox', { name: 'Theme section', exact: true }).selectOption('categories');
    await editor.getByRole('combobox', { name: 'Theme variant', exact: true }).selectOption('light');
    const category = editor.getByRole('textbox', { name: 'category-1 value', exact: true });
    const linenFirst = await page.evaluate(() => dinahAppearance.builtins.linen.palettes.light['category-1']);
    ok('an older theme imports with category colours, shown as eight fields', await category.inputValue() === linenFirst && await editor.locator('.theme-color-field').count() === 8);
    await category.fill(older.palettes.light.surface); await category.press('Tab');
    await editor.locator('.theme-contrast li').filter({ hasText: 'light category-1 needs 3:1' }).waitFor();
    ok('the editor reports a category colour without 3:1 contrast', true);
    await category.fill(linenFirst); await category.press('Tab');
    await editor.getByRole('button', { name: 'Save', exact: true }).click();
    await page.waitForFunction(() => dinahAppearance.snapshot().state.themes.some(t => t.document.name === 'Older study' && t.document.palettes.dark['category-8']));
    ok('the older theme saves with its category colours', true);
    await t.shot('theming-editor');
    await page.evaluate(() => dinahWorkbench.open('settings'));
    await page.evaluate(() => pudlWindows.snap('settings', 'maximized'));
    await page.getByRole('combobox', { name: 'interface font families', exact: true }).fill('MissingThemeFontFixture');
    await page.getByRole('combobox', { name: 'interface font families', exact: true }).press('Tab');
    const personalCount = await page.evaluate(() => dinahAppearance.snapshot().state.themes.length);
    await page.locator('#appearance-save-fonts').click();
    await page.waitForFunction(() => dinahAppearance.document(dinahAppearance.snapshot().state.active).typography.interface.families.includes('MissingThemeFontFixture'));
    ok('font changes update the active personal theme without copies', await page.evaluate(({id, count}) => dinahAppearance.snapshot().state.active === id && dinahAppearance.snapshot().state.themes.length === count, {id: active, count: personalCount}));
    active = await page.evaluate(() => dinahAppearance.snapshot().state.active);
    ok('global fonts apply as a personal theme with a system fallback', await page.evaluate(() => getComputedStyle(document.body).fontFamily.includes('MissingThemeFontFixture') && getComputedStyle(document.body).fontFamily.includes('system-ui')));
    ok('repeated font changes retain identity and name', await page.evaluate(async () => {
      const before = dinahAppearance.snapshot().state, id = before.active, name = dinahAppearance.document(id).name;
      for (const family of ['Arial', 'MissingThemeFontFixture']) { const doc = dinahAppearance.document(id); doc.typography.interface.families = [family]; await dinahAppearance.save(doc, id, true); }
      const after = dinahAppearance.snapshot().state;
      return after.active === id && after.themes.length === before.themes.length && dinahAppearance.document(id).name === name;
    }));
    await page.reload(); await page.waitForFunction(() => window.dinahAppearance); await page.evaluate(() => dinahAppearance.ready);
    ok('active theme and personal overrides survive reload', await page.evaluate(id => dinahAppearance.snapshot().state.active === id && dinahAppearance.snapshot().state.overrides.chat.size === 18, active));
    await page.evaluate(() => dinahWorkbench.open('settings')); await page.evaluate(() => pudlWindows.snap('settings', 'maximized'));
    await page.evaluate(() => __themeAPI('fail'));
    await page.locator('#appearance-theme').selectOption('mineral');
    await page.locator('#appearance-status').filter({ hasText: 'storage unavailable' }).waitFor();
    ok('failed saves preserve the active theme and selector', await page.locator('#appearance-theme').inputValue() === active && await page.evaluate(id => dinahAppearance.snapshot().state.active === id, active));
    // Your Themes lists the person's themes, each with Delete, so one can go
    // without being put in use first; deleting the one in use restores
    // Mineral. Built-in themes are not listed.
    await page.evaluate(() => { const doc = dinahAppearance.document('mineral'); doc.name = 'Spare Theme'; return dinahAppearance.save(doc, undefined, false); });
    const rows = page.locator('#appearance-yours .theme-list-row');
    await rows.filter({ hasText: 'Spare Theme' }).waitFor();
    const personal = () => page.evaluate(() => dinahAppearance.snapshot().state.themes.length);
    ok('Your Themes lists each personal theme, the one in use marked, and no built-in theme', await rows.count() === await personal() &&
      await rows.filter({ hasText: 'In use' }).count() === 1 && !(await page.locator('#appearance-yours').textContent()).includes('Mineral'));
    await rows.filter({ hasText: 'Spare Theme' }).getByRole('button', { name: 'Delete the theme Spare Theme' }).click();
    await page.locator('#ask').waitFor();
    ok('deleting a theme asks first, naming it', (await page.locator('#ask-body').textContent()).includes('"Spare Theme" will be removed.'));
    await page.locator('#ask-yes').click();
    await page.waitForFunction(() => !dinahAppearance.snapshot().state.themes.some(t => t.document.name === 'Spare Theme'));
    ok('a theme not in use is deleted without changing the theme in use', await page.evaluate(id => dinahAppearance.snapshot().state.active === id, active) && await rows.count() === await personal() && !(await rows.allTextContents()).some(t => t.includes('Spare Theme')));
    await rows.filter({ hasText: 'In use' }).getByRole('button', { name: /^Delete the theme / }).click();
    await page.locator('#ask').waitFor();
    ok('deleting the theme in use says Mineral takes its place', (await page.locator('#ask-body').textContent()).includes('Mineral will be used in its place'));
    await page.locator('#ask-yes').click();
    await page.waitForFunction(() => dinahAppearance.snapshot().state.active === 'mineral');
    ok('deleting the active personal theme restores Mineral first', await page.evaluate(id => dinahAppearance.snapshot().state.active === 'mineral' && !dinahAppearance.snapshot().state.themes.some(t => t.id === id), active));
    while (await personal() > 0) {
      const before = await personal();
      await rows.first().getByRole('button', { name: /^Delete the theme / }).click();
      await page.locator('#ask-yes').click();
      await page.waitForFunction(n => dinahAppearance.snapshot().state.themes.length < n, before);
    }
    ok('with no personal themes left, the list says how to make one', await rows.count() === 0 && await page.locator('#appearance-yours-empty').isVisible());
    await page.setViewportSize({ width: 390, height: 844 });
    ok('appearance settings fit a narrow window', await page.locator('#settings-appearance').evaluate(el => el.scrollWidth <= el.clientWidth + 1));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
