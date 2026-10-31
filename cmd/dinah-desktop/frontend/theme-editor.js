(function () {
  'use strict';
  const api = () => window.dinahAppearance;
  // The name of the theme used until the person chooses another.
  const defaultName = () => api().builtins[api().defaultTheme()].name;
  const app = window.dinahHelpers.app;
  const clone = value => JSON.parse(JSON.stringify(value));
  let currentEditor;
  async function openEditor() { await dinahWorkbench.open('theme-editor'); if (currentEditor) await currentEditor.ready; return currentEditor; }
  const fontRoles = ['interface', 'heading', 'reading', 'monospace'];
  function element(tag, text, className) { return window.dinahHelpers.node(tag, className, text || undefined); }
  function button(text, run, primary) { const el = element('button', text, 'btn btn-sm' + (primary ? ' btn-primary' : '')); el.type = 'button'; el.addEventListener('click', run); return el; }
  function numberField(input) { const wrap = element('span', '', 'number-field'); wrap.append(input); return wrap; }
  function enhance(root) { window.pudlCombo?.enhance(root); window.pudlFields?.enhance(root); }
  function field(label, input) { const host = element('label', '', 'theme-field'); host.append(element('span', label), input); return host; }
  function fontField(value, role, changed) {
    const host = element('div', '', 'theme-font-field'), families = element('input'), fallback = element('select'); families.className = 'form-input'; families.type = 'text'; families.maxLength = 1032;
    families.value = value.families.join(', '); families.setAttribute('aria-label', role + ' font families'); families.placeholder = 'Font families, separated by commas';
    families.setAttribute('list', ['monospace', 'editor', 'terminal'].includes(role) ? 'theme-code-faces' : 'theme-text-faces');
    const choices = ['monospace', 'editor', 'terminal'].includes(role) ? ['monospace'] : role === 'interface' ? ['system-ui', 'sans-serif'] : ['sans-serif', 'serif', 'system-ui'];
    if (['heading', 'reading'].includes(role)) choices.unshift('inherit');
    choices.forEach(name => { const option = element('option', name === 'inherit' ? 'Use interface font' : name); option.value = name; fallback.append(option); }); fallback.value = value.fallback;
    fallback.className = 'form-select';
    fallback.setAttribute('aria-label', role + ' font fallback');
    function update() { if (families.value.trim() && fallback.value === 'inherit') fallback.value = 'sans-serif'; changed({ families: families.value.split(',').map(s => s.trim()).filter(Boolean), fallback: fallback.value }); }
    families.addEventListener('change', update); fallback.addEventListener('change', () => { if (fallback.value === 'inherit') families.value = ''; update(); });
    const combo = element('span', '', 'combo'), selectWrap = element('span', '', 'form-select-wrap'); combo.append(families); selectWrap.append(fallback); host.append(combo, selectWrap); return host;
  }
  function list(select, selected, includeNew) {
    select.replaceChildren();
    if (includeNew) { const option = element('option', 'New theme draft'); option.value = 'new'; select.append(option); }
    const snapshot = api().snapshot();
    [...Object.entries(api().builtins).map(([id, document]) => ({ id, document, builtin: true })), ...snapshot.state.themes].forEach(record => {
      const option = element('option', record.document.name + (record.builtin ? ' (built in)' : '')); option.value = record.id; select.append(option);
    }); select.value = selected;
  }
  function exportDocument(doc) {
    api().validate(doc);
    if (app()?.ExportThemeFile) return app().ExportThemeFile(doc);
    const url = URL.createObjectURL(new Blob([JSON.stringify(doc, null, 2)], { type: 'application/json' }));
    const link = element('a'); link.href = url; link.download = 'theme.dinah-theme.json'; link.click(); setTimeout(() => URL.revokeObjectURL(url), 1000); return Promise.resolve();
  }
  async function importDocument(file) {
    if (file.size > 256 * 1024) throw Error('Theme exceeds 256 KiB.');
    // A theme exported before category colours gets them, as appearance.js
    // describes, and the contrast checks show any rule they break.
    return api().validate(api().fillCategories(JSON.parse(await file.text())));
  }
  const labels = { bg: 'Workspace', surface: 'Content surface', 'surface-alt': 'Alternate surface', text: 'Text', 'text-muted': 'Secondary text', border: 'Control boundary', accent: 'Accent', 'accent-hover': 'Accent hover', 'on-accent': 'Text on accent', warn: 'Warning', 'on-warn': 'Text on warning', danger: 'Error / destructive', 'danger-hover': 'Destructive hover', positive: 'Success', light: 'Highlight color', shade: 'Shadow color', lit: 'Highlight strength', depth: 'Shadow strength', 'tb-bg': 'Topbar background', 'tb-fg': 'Topbar text', 'dinah-skill': 'Skill text', 'dinah-title': 'Chat title' };
  for (let n = 1; n <= 8; n++) labels['category-' + n] = 'Category ' + n;
  const sectionNames = { surfaces: 'Surfaces and text', actions: 'Actions and status', topbar: 'Topbar', syntax: 'Syntax colors', categories: 'Category colors', lighting: 'Lighting', fonts: 'Fonts' };
  function group(key) { return key.startsWith('syntax-') ? 'syntax' : key.startsWith('category-') ? 'categories' : ['light', 'shade', 'lit', 'depth'].includes(key) ? 'lighting' : key.startsWith('tb-') ? 'topbar' : ['accent', 'accent-hover', 'on-accent', 'warn', 'on-warn', 'danger', 'danger-hover', 'positive', 'dinah-skill', 'dinah-title'].includes(key) ? 'actions' : 'surfaces'; }
  function previewMarkup(mode) {
    const css = new URL('pudl/pudl.css', location.href).href, brand = new URL('brand.css', location.href).href;
    // The sample shows the product's name (product.js) as markup.
    const name = String(window.dinahProduct?.name || '').replace(/[&<>"]/g, c => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;' })[c]);
    return '<!doctype html><html lang="en" data-theme="' + mode + '"><meta charset="utf-8"><link rel="stylesheet" href="' + css + '"><link rel="stylesheet" href="' + brand + '"><style>body{margin:0;padding:14px;background:var(--bg)}.sample{padding:14px;background:var(--surface);border:1px solid var(--border);border-radius:8px}h2{font-size:18px;margin:0 0 12px}.sample p{font-family:var(--reading-font,var(--font))}.row{display:flex;gap:7px;flex-wrap:wrap;margin:10px 0}.picked{background:var(--surface-alt);border-left:3px solid var(--accent);padding:7px}textarea{width:100%;resize:vertical}code{font-family:var(--mono);color:var(--syntax-keyword)}.muted{color:var(--text-muted);font-size:12px}.status{color:var(--positive)}.warning{color:var(--warn)}.error{color:var(--danger)}.swatches span{width:18px;height:18px;border-radius:4px}' + [1, 2, 3, 4, 5, 6, 7, 8].map(n => '.swatches .c' + n + '{background:var(--category-' + n + ')}').join('') + '</style><body><header class="topbar" style="display:flex;gap:12px;margin-bottom:10px"><strong>' + name + '</strong><span>Documents</span><button class="btn btn-sm">Tools</button></header><div class="sample"><h2>' + name + ' Development</h2><div class="picked">Selected conversation</div><p>The theme preview keeps your other chats unchanged.</p><p class="muted">Secondary text and the default reading font.</p><code>const theme = "custom";</code><pre style="background:var(--input-bg);font-family:var(--mono);padding:6px">$ echo ready&#10;ready</pre><div class="row"><span class="status">✓ Done</span><span class="warning">△ Waiting</span><span class="error">× Failed</span></div><div class="row swatches" title="Category colors 1 to 8">' + [1, 2, 3, 4, 5, 6, 7, 8].map(n => '<span class="c' + n + '"></span>').join('') + '</div><textarea class="form-textarea" aria-label="Preview composer" placeholder="Message"></textarea><div class="row"><button class="btn btn-primary btn-sm">Send</button><button class="btn btn-sm">Open menu</button><button class="btn btn-danger btn-sm">Remove</button><button class="btn btn-sm" disabled>Disabled</button></div><input class="form-input" aria-label="Preview field" placeholder="Focusable input"></div></body></html>';
  }
  window.dinahWorkbench.register('theme-editor', { applet: { id: 'dinah.themes', title: 'Theme Editor', glyph: 'gear', workspace: false }, singleton: true, title: 'Theme Editor', glyph: 'gear', launcher: { run: () => dinahWorkbench.open('theme-editor') }, init(root, opts) {
    root.classList.add('theme-editor'); let selected = 'new', section = 'surfaces', mode = 'light', draft, baseline = '', disposed = false, busy = false, pendingState = opts.state || '', allowClose = false;
    const win = root.closest('.win'), top = element('div', '', 'theme-editor-toolbar'), select = element('select'), name = element('input'); name.className = 'form-input'; name.maxLength = 100;
    select.className = 'form-select'; select.setAttribute('aria-label', 'Theme to edit'); name.setAttribute('aria-label', 'Theme name');
    const status = element('p', '', 'form-help'); status.setAttribute('role', 'status');
    const controls = element('div', '', 'theme-editor-toolbar'), sections = element('select'), variants = element('select');
    Object.entries(sectionNames).forEach(([value, text]) => { const option = element('option', text); option.value = value; sections.append(option); }); sections.className = 'form-select'; sections.setAttribute('aria-label', 'Theme section');
    ['light', 'dark'].forEach(value => { const option = element('option', value === 'light' ? 'Edit light variant' : 'Edit dark variant'); option.value = value; variants.append(option); }); variants.className = 'form-select'; variants.setAttribute('aria-label', 'Theme variant');
    const fields = element('div', '', 'theme-editor-fields'), checks = element('ul', '', 'theme-contrast'), previews = element('div', '', 'theme-previews'), frames = ['light', 'dark'].map(value => { const article = element('section'); article.append(element('h3', value === 'light' ? 'Light preview' : 'Dark preview')); const frame = element('iframe'); frame.title = value + ' theme preview'; frame.srcdoc = previewMarkup(value); frame.addEventListener('load', () => schedulePreview()); article.append(frame); previews.append(article); return frame; });
    const picker = element('input'); picker.type = 'file'; picker.accept = '.json,application/json'; picker.hidden = true;
    function say(error) { status.textContent = String(error?.message || error || ''); }
    function address() { return new URLSearchParams({ theme: selected, section, variant: mode }).toString(); }
    function changed() { dinahUIState.set('dinah-theme-draft:' + selected, { document: draft, baseline }); opts.changed(address()); schedulePreview(); }
    function dirty() { return draft && JSON.stringify(draft) !== baseline; }
    async function run(action) { if (busy) return; busy = true; try { await action(); } catch (error) { say(error); } finally { busy = false; } }
    function choose(id, recovered = true) {
      if (id !== 'new' && !api().document(id)) { id = api().defaultTheme(); say('The requested theme is unavailable. ' + defaultName() + ' is shown.'); }
      selected = id; const original = id === 'new' ? api().document(api().snapshot().state.active) : api().document(id);
      let saved = recovered && dinahUIState.get('dinah-theme-draft:' + selected);
      if (saved) { try { api().validate(api().fillCategories(saved.document)); } catch (_) { saved = null; say('The saved draft is invalid. The saved theme is shown.'); } }
      draft = saved?.document || original || api().document(api().defaultTheme()); baseline = saved?.baseline || JSON.stringify(draft);
      name.value = draft.name; list(select, selected, true); sections.value = section; variants.value = mode; renderFields(); changed();
    }
    function renderFields() {
      fields.replaceChildren(); variants.hidden = section === 'fonts';
      if (section === 'fonts') {
        fontRoles.forEach(role => fields.append(field(role.charAt(0).toUpperCase() + role.slice(1) + ' font', fontField(draft.typography[role], role, value => { draft.typography[role] = value; changed(); }))));
        enhance(fields);
        fields.append(element('p', 'Families use local fonts with a generic fallback. Missing faces fall back; fonts are not downloaded or bundled.', 'form-help'));
        return;
      }
      api().keys.filter(key => group(key) === section).forEach(key => {
        const wrapper = element('div', '', 'theme-color-field'), title = element('label', labels[key] || key.replace('syntax-', 'Syntax ')), input = element('input'); title.title = '--' + key; input.id = 'theme-' + mode + '-' + key; title.htmlFor = input.id;
        input.className = 'form-input'; input.setAttribute('aria-label', key + ' value');
        if (['lit', 'depth'].includes(key)) { input.type = 'number'; input.min = 0; input.max = 100; input.step = .01; input.value = parseFloat(draft.palettes[mode][key]); input.addEventListener('change', () => { draft.palettes[mode][key] = input.value + '%'; changed(); }); wrapper.append(title, input); }
        else {
          input.type = 'text'; input.maxLength = 7; input.value = draft.palettes[mode][key];
          const color = element('input'); color.type = 'color'; color.value = input.value; color.setAttribute('aria-label', (labels[key] || key) + ' picker');
          color.addEventListener('input', () => { input.value = color.value; draft.palettes[mode][key] = color.value; changed(); });
          input.addEventListener('change', () => { draft.palettes[mode][key] = input.value; if (/^#[0-9a-f]{6}$/i.test(input.value)) color.value = input.value; changed(); }); wrapper.append(title, color, input);
        }
        fields.append(wrapper);
      });
      if (section === 'categories') fields.append(element('p', 'Category colors tell things apart, such as tags and the series of a chart, and say nothing about state. Each needs 3:1 contrast on the workspace, the content surface and input fields, stays apart from the accent and the warning, error and success colors, and differs from the next one for every reader.', 'form-help'));
    }
    function schedulePreview() { if (!disposed && draft) dinahRender.enqueue(root, root, drawPreview, null, 'theme preview'); }
    function drawPreview() {
      try {
        api().validate(draft); checks.replaceChildren();
        const failures = api().contrast(draft); (failures.length ? failures : ['Contrast checks pass for the tested pairs. Review focus, status distinctions and surface elevation in the previews.']).forEach(text => checks.append(element('li', text)));
        frames.forEach((frame, index) => {
          const doc = frame.contentDocument; if (!doc?.body) return;
          Object.entries(draft.palettes[index ? 'dark' : 'light']).forEach(([key, value]) => doc.documentElement.style.setProperty('--' + key, value));
          [['--font', 'interface'], ['--font-display', 'heading'], ['--reading-font', 'reading'], ['--mono', 'monospace']].forEach(([key, role]) => doc.documentElement.style.setProperty(key, api().font(draft.typography[role], role)));
        });
      } catch (error) { checks.replaceChildren(element('li', String(error.message || error))); }
    }
    async function save(activate, asNew) {
      const id = await api().save(draft, asNew ? null : selected, activate);
      dinahUIState.set('dinah-theme-draft:' + selected, undefined); choose(id, false);
      say(activate ? 'Theme applied and saved.' : 'Theme saved. The active theme is unchanged.');
    }
    async function discardQuestion() { return !dirty() || await dinahHost.ask('Discard Theme Changes?', 'The unsaved changes in this theme draft will be discarded.', 'Discard Changes'); }
    select.addEventListener('change', () => { const id = select.value; run(async () => { if (await discardQuestion()) choose(id); else select.value = selected; }); });
    name.addEventListener('input', () => { if (draft) { draft.name = name.value; changed(); } });
    sections.addEventListener('change', () => { section = sections.value; renderFields(); opts.changed(address()); }); variants.addEventListener('change', () => { mode = variants.value; renderFields(); opts.changed(address()); });
    function newDraft() { selected = 'new'; draft = api().document(api().snapshot().state.active); draft.name += ' copy'; baseline = JSON.stringify(draft); name.value = draft.name; list(select, selected, true); renderFields(); changed(); }
    function imported(doc) { api().validate(doc); selected = 'new'; draft = clone(doc); baseline = ''; name.value = draft.name; list(select, selected, true); renderFields(); changed(); say('Imported into a draft. Review and save or apply it.'); }
    picker.addEventListener('change', () => run(async () => { if (picker.files[0] && await discardQuestion()) imported(await importDocument(picker.files[0])); picker.value = ''; }));
    const actions = {
      'New from Current': () => run(async () => { if (await discardQuestion()) newDraft(); }),
      Duplicate: () => run(async () => { selected = 'new'; draft = clone(draft); draft.name += ' copy'; baseline = ''; name.value = draft.name; list(select, selected, true); changed(); }),
      Save: () => run(() => save(false, false)), 'Save As': () => run(() => save(false, true)), Apply: () => run(() => save(true, false)),
      Revert: () => run(async () => { if (await discardQuestion()) { dinahUIState.set('dinah-theme-draft:' + selected, undefined); choose(selected, false); } }),
      Import: () => run(async () => { if (app()?.ImportThemeFile) { if (!await discardQuestion()) return; const doc = await app().ImportThemeFile(); if (doc) imported(doc); } else picker.click(); }),
      Export: () => run(() => exportDocument(draft)),
      Delete: () => run(async () => { if (api().builtins[selected] || selected === 'new') throw Error('Built-in themes and unsaved drafts cannot be deleted.'); if (await dinahHost.ask('Delete Theme?', 'The saved theme will be removed. If active, ' + defaultName() + ' will become active first.', 'Delete')) { await api().remove(selected); dinahUIState.set('dinah-theme-draft:' + selected, undefined); choose(api().defaultTheme(), false); say('Theme deleted.'); } })
    };
    top.append(field('Theme', select), field('Name', name)); Object.entries(actions).forEach(([text, run]) => controls.append(button(text, run, text === 'Apply'))); controls.append(field('Section', sections), field('Variant', variants));
    root.append(top, controls, status, fields, checks, previews, picker);
    function closing(event) { if (event.target !== win || allowClose || !dirty()) return; event.preventDefault(); run(async () => { if (await discardQuestion()) { dinahUIState.set('dinah-theme-draft:' + selected, undefined); allowClose = true; pudlWindows.close(opts.key || win.dataset.win); } }); }
    win.addEventListener('pudl:window-closing', closing);
    function setState(value) { pendingState = value || ''; if (!draft) return; const params = new URLSearchParams(value); mode = params.get('variant') === 'dark' ? 'dark' : 'light'; variants.value = mode; section = Object.hasOwn(sectionNames, params.get('section') || '') ? params.get('section') : 'surfaces'; const id = params.get('theme') || selected; if (id !== selected) run(async () => { if (await discardQuestion()) choose(id); }); else { sections.value = section; renderFields(); } }
    const ready = Promise.all([api().ready, dinahUIState.ready]).then(() => { if (disposed) return; const params = new URLSearchParams(pendingState); mode = params.get('variant') === 'dark' ? 'dark' : 'light'; section = Object.hasOwn(sectionNames, params.get('section') || '') ? params.get('section') : 'surfaces'; choose(params.get('theme') || api().snapshot().state.active); if (api().snapshot().error) say(api().snapshot().error); });
    currentEditor = { ready, setState, importTheme: actions.Import };
    return { ready, state: address, setState, commands: () => Object.entries(actions).map(([label, run]) => ({ label, disabled: busy, run })), destroy() { disposed = true; currentEditor = null; dinahRender.cancel(root); win.removeEventListener('pudl:window-closing', closing); } };
  } });
  function settings() {
    const select = document.getElementById('appearance-theme'), status = document.getElementById('appearance-status'), fonts = document.getElementById('appearance-fonts'), overrides = document.getElementById('appearance-overrides');
    let fontDraft, overrideDraft, busy = false;
    for (const [id, names] of [['theme-text-faces', ['Inter', 'Segoe UI', 'Arial', 'Verdana', 'Georgia']], ['theme-code-faces', ['Cascadia Code', 'Consolas', 'Courier New', 'Menlo', 'DejaVu Sans Mono']]]) { const list = element('datalist'); list.id = id; names.forEach(name => { const option = element('option'); option.value = name; list.append(option); }); document.body.append(list); }
    async function run(action) { if (busy) return; busy = true; try { await action(); status.textContent = ''; } catch (error) { status.textContent = String(error.message || error); } finally { busy = false; } }
    /* Your Themes: every theme the person made, each with Delete, so one can
       go without being put in use first (Paul, 27 October 2026). Deleting
       the theme in use puts Mineral in its place. Built-in themes are not
       listed, since they cannot be deleted. */
    function drawYours(snapshot) {
      const host = document.getElementById('appearance-yours'), themes = snapshot.state.themes;
      document.getElementById('appearance-yours-empty').hidden = themes.length > 0;
      host.hidden = !themes.length;
      host.replaceChildren(...themes.map(record => {
        const row = element('li', '', 'theme-list-row'), name = element('span', record.document.name, 'theme-list-name');
        row.append(name);
        if (record.id === snapshot.state.active) row.append(element('span', 'In use', 'badge accent'));
        const remove = button('Delete…', () => run(async () => {
          const inUse = api().snapshot().state.active === record.id;
          const body = '"' + record.document.name + '" will be removed.' + (inUse ? ' It is in use, so ' + defaultName() + ' will be used in its place.' : '') + ' This cannot be undone; export the theme first to keep a copy.';
          if (await dinahHost.ask('Delete Theme?', body, 'Delete Theme', true)) { await api().remove(record.id); dinahUIState.set('dinah-theme-draft:' + record.id, undefined); }
        }));
        remove.classList.add('btn-sm', 'btn-danger');
        remove.setAttribute('aria-label', 'Delete the theme ' + record.document.name);
        row.append(remove);
        return row;
      }));
    }
    function refresh() {
      const snapshot = api().snapshot(); list(select, snapshot.state.active); status.textContent = snapshot.error || snapshot.notice;
      drawYours(snapshot);
      fontDraft = api().document(snapshot.state.active); overrideDraft = clone(snapshot.state.overrides); fonts.replaceChildren(); overrides.replaceChildren();
      fontRoles.forEach(role => fonts.append(field(role.charAt(0).toUpperCase() + role.slice(1), fontField(fontDraft.typography[role], role, value => { fontDraft.typography[role] = value; }))));
      ['chat', 'editor', 'preview', 'terminal'].forEach(kind => {
        const row = element('div', '', 'theme-override-row'), value = overrideDraft[kind] || {}, inherit = element('input'); inherit.type = 'checkbox'; inherit.checked = !value.font;
        const descriptor = value.font || clone(fontDraft.typography[['editor', 'terminal'].includes(kind) ? 'monospace' : 'reading']); if (descriptor.fallback === 'inherit') Object.assign(descriptor, clone(fontDraft.typography.interface));
        const font = fontField(descriptor, kind, next => { Object.assign(descriptor, clone(next)); overrideDraft[kind] ||= {}; overrideDraft[kind].font = next; inherit.checked = false; });
        inherit.addEventListener('change', () => { overrideDraft[kind] ||= {}; if (inherit.checked) delete overrideDraft[kind].font; else overrideDraft[kind].font = clone(descriptor); });
        const size = element('input'); size.type = 'number'; size.className = 'form-input'; size.min = 6; size.max = kind === 'terminal' ? 22 : 32; size.placeholder = 'Global default'; size.value = value.size || ''; size.setAttribute('aria-label', kind + ' text size');
        size.addEventListener('change', () => { overrideDraft[kind] ||= {}; if (size.value === '') delete overrideDraft[kind].size; else overrideDraft[kind].size = Number(size.value); const next = clone(overrideDraft); run(() => api().overrides(next)); });
        const inheritLabel = element('label', '', 'check'); inheritLabel.append(inherit, document.createTextNode('Use global font'));
        row.append(element('h4', ({ chat: 'Chat', editor: 'Editor source', preview: 'Document content', terminal: 'Terminal' })[kind]), inheritLabel, font, field('Content size (px)', numberField(size)), button('Reset ' + kind, () => run(async () => { const next = clone(overrideDraft); delete next[kind]; await api().overrides(next); })));
        overrides.append(row);
      });
      enhance(fonts); enhance(overrides);
    }
    select.addEventListener('change', () => run(async () => { const id = select.value; try { await api().select(id); } catch (e) { select.value = api().snapshot().state.active; throw e; } }));
    document.getElementById('appearance-edit').addEventListener('click', () => run(async () => { const editor = await openEditor(); editor?.setState(new URLSearchParams({ theme: api().snapshot().state.active }).toString()); }));
    document.getElementById('appearance-default').addEventListener('click', () => run(() => api().select(api().defaultTheme())));
    document.getElementById('appearance-reload').addEventListener('click', () => run(api().reload));
    document.getElementById('appearance-export').addEventListener('click', () => run(() => exportDocument(api().document(api().snapshot().state.active))));
    document.getElementById('appearance-import').addEventListener('click', () => run(async () => { const editor = await openEditor(); await editor?.importTheme(); }));
    document.getElementById('appearance-save-fonts').addEventListener('click', () => run(() => api().save(fontDraft, api().snapshot().state.active, true)));
    document.getElementById('appearance-save-overrides').addEventListener('click', () => run(() => api().overrides(overrideDraft)));
    document.getElementById('appearance-reset-overrides').addEventListener('click', () => run(() => api().overrides({})));
    document.addEventListener('dinah:appearance-library', refresh); api().ready.then(refresh);
    document.querySelectorAll('[data-default-theme]').forEach(button => button.addEventListener('click', () => run(() => api().select(api().defaultTheme()))));
  }
  settings();
})();
