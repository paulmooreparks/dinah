const { launch, PAGE } = require('./lib/page');
const { settled, settledCount } = require('./lib/settled');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspace: 'C:/Test', recent: ['C:/Test'] },
    app: () => ({
      OpenSessionAt: async (ws, id) => ({ sessionId: id, provider: 'codex', title: id, cwd: ws, open: true }),
      ResolveDocumentLink: async (cwd, path) => { __links.push({cwd, path}); return { ref: path, found: true, opens: 'system', path: cwd + '/' + path }; },
      OpenPath: async () => 'opened',
      Send: async (id, text, doc, attachments) => { __sent.push({ id, text, attachments }); },
      // The host's clipboard for the menu's Paste: a picture
      // (clipboard_image_windows.go) and the text (clipboard-transport.js).
      // 'refuse' makes the read fail. Go sends a picture's bytes as base64.
      ClipboardImage: async () => { if (__clip.image === 'refuse') throw new Error('The clipboard refused the picture.'); return __clip.image || { type: '', data: null }; },
      ClipboardRead: async () => { if (__clip.text === 'refuse') throw new Error('The clipboard refused the text.'); return { html: '', text: __clip.text }; }
    }),
    init: () => {
      window.__sent = []; window.__links = []; window.__clip = { image: null, text: '' }; window.__editErrors = [];
      document.addEventListener('pudl:edit-error', e => { __editErrors.push(e.detail.operation + ': ' + (e.detail.error && e.detail.error.message)); });
    },
    ready: () => window.dinahWorkbench && window.dinahEditor,
    goto: false,
  });
  const { page, ok } = t;
  try {
    // An external source file lets the regression be checked against a
    // previous revision without replacing files in the working tree.
    if (process.env.DINAH_CLIPBOARD_APP_SCRIPT) await t.context.route('http://dinah.test/app.js', r => r.fulfill({ path: process.env.DINAH_CLIPBOARD_APP_SCRIPT }));
    await page.goto(PAGE);
    await t.waitReady();
    // The provider knows the execution folder before a saved window opens
    // using a workspace identity or another spelling of the folder.
    await page.evaluate(() => __emit('agent:session', { sessionId: 'codex:first', cwd: 'c:/test', provider: 'codex', open: true }));
    const first = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:first' }));
    const second = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:second' }));
    await page.evaluate(key => pudlWindows.raise(key), first);
    const a = page.locator('[data-win="' + first + '"]'), b = page.locator('[data-win="' + second + '"]');
    const paste = (element, options) => element.evaluate((el, options) => {
      const bytes = Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII='), c => c.charCodeAt(0));
      const file = new File([bytes], options.name || 'screenshot.png', { type: 'image/png' });
      let data;
      if (options.itemsOnly) data = { files: [], items: [{ kind: 'file', getAsFile: () => file }], getData: () => options.text || '' };
      else { data = new DataTransfer(); if (!options.textOnly) data.items.add(file); if (options.text) data.setData('text/plain', options.text); }
      const event = new Event('paste', { bubbles: true, cancelable: true });
      Object.defineProperty(event, 'clipboardData', { value: data });
      el.dispatchEvent(event); return event.defaultPrevented;
    }, options);
    const attachments = win => win.locator('.attachment-name');
    await a.locator('[data-view-id="prompt"]').focus();
    for (const chord of [{ key: 'v', ctrlKey: true }, { key: 'Insert', shiftKey: true }]) {
      const cancelled = await a.locator('[data-view-id="prompt"]').evaluate((el, chord) => {
        const event = new KeyboardEvent('keydown', Object.assign({ bubbles: true, cancelable: true }, chord));
        el.dispatchEvent(event); return event.defaultPrevented;
      }, chord);
      ok(chord.key + ' paste shortcut is left to the browser', !cancelled);
    }
    // Since PUDL 0.109.0 the prompt's right-click menu is PUDL's edit menu,
    // and the web view's own menu is turned off (docs/design/chat-clipboard.md).
    await a.locator('[data-view-id="prompt"]').click({ button: 'right' });
    await page.locator('.context-menu').first().waitFor({ state: 'visible' });
    const promptRows = await page.evaluate(() => [...document.querySelectorAll('.context-menu .menu-action')].filter(r => r.offsetParent).map(r => (r.querySelector('.menubar-words') || r).textContent.trim()));
    ok('the prompt\'s right-click opens PUDL\'s edit menu', promptRows.join('|') === 'Cut|Copy|Paste|Select All', JSON.stringify(promptRows));
    await page.keyboard.press('Escape');
    await a.locator('[data-view-id="prompt"]').focus();
    await paste(a.locator('[data-view-id="prompt"]'), {});
    await attachments(a).filter({ hasText: 'screenshot.png' }).waitFor();
    ok('a pasted screenshot becomes an image attachment', await a.locator('.attachment img').count() > 0);
    await paste(a.locator('[data-view-id="prompt"]'), { itemsOnly: true, name: 'items-only.png' });
    await attachments(a).filter({ hasText: 'items-only.png' }).waitFor();
    ok('clipboard file items work even when the files list is empty', true);
    const mixedPrevented = await paste(a.locator('[data-view-id="prompt"]'), { name: 'mixed.png', text: 'Clipboard text' });
    await attachments(a).filter({ hasText: 'mixed.png' }).waitFor();
    ok('mixed image and text paste attaches the image without cancelling native text insertion', !mixedPrevented);
    await paste(a.locator('[data-view-id="logs"]'), { name: 'chat-window.png' });
    await attachments(a).filter({ hasText: 'chat-window.png' }).waitFor();
    ok('image paste works in the chat window outside the composer', true);
    await paste(b.locator('[data-view-id="prompt"]'), { name: 'wrong-window.png' });
    ok('a different chat does not receive the active chat paste', await attachments(b).count() === 0);
    ok('plain text paste keeps its native behavior', !(await paste(a.locator('[data-view-id="prompt"]'), { textOnly: true, text: 'ordinary text' })));
    await a.locator('[data-view-id="prompt"]').fill('Inspect the images.');
    await a.locator('[data-view-id="prompt"]').press('Control+Enter');
    await page.waitForFunction(() => __sent.length > 0);
    ok('sending keeps all image payloads on the intended chat', await page.evaluate(() => __sent[0].id === 'codex:first' && __sent[0].attachments.length === 4 && __sent[0].attachments.every(a => a.mimeType === 'image/png' && a.data.length > 0)));
    await a.locator('.sent-images img').first().waitFor();
    const thumbs = await settledCount(page, a.locator('.sent-images img'), 4);
    ok('sent images retain thumbnails', thumbs.ok, thumbs.detail);
    await a.locator('.sent-images img').first().click();
    await page.locator('#media-viewer[open]').waitFor();
    ok('sent thumbnail opens the image viewer', true);
    await page.keyboard.press('Escape');
    await page.evaluate(() => __emit('agent:update', {sessionId:'codex:first', update:{sessionUpdate:'agent_message_chunk', content:{type:'text',text:'Read [the design note](docs/proposals/note.md) and [a spaced path](<docs/my note.md>). [unsafe](javascript:alert)'}}}));
    await a.locator('[data-chip-local]').first().waitFor();
    const links = await settled(page, { local: [a.locator('[data-chip-local]'), 2], unsafe: [a.locator('a[href^="javascript:"]'), 0] });
    ok('relative and spaced local Markdown links render safely', links.ok, links.detail);
    await a.locator('[data-chip-local]').first().click();
    ok('local link uses the owning chat execution folder', await page.evaluate(() => __links.length === 1 && __links[0].cwd.toLowerCase() === 'c:/test' && __links[0].path === 'docs/proposals/note.md'));

    // The prompt claims PUDL's edit menu and gives the same four rows, whose
    // Paste attaches a copied picture as Ctrl+V does (docs/design/chat-clipboard.md).
    await page.evaluate(() => { window.__toasts = []; const t = window.pudlToast; window.pudlToast = (text, o) => { __toasts.push(String(text)); return t && t(text, o); }; });
    const prompt = a.locator('[data-view-id="prompt"]');
    // Shift+F10 opens the same menu without moving the caret off a selection.
    const openMenu = async keyboard => {
      if (keyboard) await page.keyboard.press('Shift+F10'); else await prompt.click({ button: 'right' });
      await page.locator('.context-menu').first().waitFor({ state: 'visible' });
      return page.evaluate(() => [...document.querySelectorAll('.context-menu .menu-action')].filter(r => r.offsetParent).map(r => ({
        label: (r.querySelector('.menubar-words') || r).textContent.trim(), disabled: r.getAttribute('aria-disabled') === 'true' })));
    };
    const choose = async label => {
      const at = await page.evaluate(l => [...document.querySelectorAll('.context-menu .menu-action')].findIndex(r => r.offsetParent && (r.querySelector('.menubar-words') || r).textContent.trim() === l), label);
      await page.locator('.context-menu .menu-action').nth(at).click();
    };
    const clipboard = c => page.evaluate(c => { __clip = c; __editErrors = []; __toasts = []; }, c);
    await prompt.fill('');
    const before = await attachments(a).count();
    await clipboard({ image: { type: 'image/png', data: 'iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII=' }, text: 'not this' });
    let rows = await openMenu();
    ok('the prompt\'s menu has the four stock rows, claimed by the chat box', rows.map(r => r.label).join('|') === 'Cut|Copy|Paste|Select All' &&
      await page.evaluate(() => __bridgeCalls.filter(c => c[0] === 'ClipboardImage').length === 0), JSON.stringify(rows));
    ok('Cut and Copy are disabled with no selection, Paste and Select All enabled', rows.map(r => r.disabled).join() === 'true,true,false,false', JSON.stringify(rows));
    await choose('Paste');
    await page.waitForFunction(n => document.querySelectorAll('[data-win] .attachment-name').length > n, before);
    const added = await attachments(a).allInnerTexts();
    ok('menu Paste with a picture on the clipboard attaches it as Ctrl+V does', added.length === before + 1 && /^pasted-.*\.png$/.test(added[added.length - 1]) &&
      await prompt.inputValue() === '' && await a.locator('.attachment img').count() > 0, JSON.stringify(added));

    await clipboard({ image: null, text: 'menu words' });
    await openMenu();
    await choose('Paste');
    await page.waitForFunction(() => [...document.querySelectorAll('[data-view-id="prompt"]')].some(p => p.value === 'menu words'));
    ok('menu Paste with no picture pastes the clipboard\'s text', await prompt.inputValue() === 'menu words' && await attachments(a).count() === before + 1);
    await prompt.press('Control+z');
    ok('Undo takes the pasted text away', await prompt.inputValue() === '', JSON.stringify(await prompt.inputValue()));

    await clipboard({ image: 'refuse', text: 'fallback words' });
    await openMenu();
    await choose('Paste');
    await page.waitForFunction(() => [...document.querySelectorAll('[data-view-id="prompt"]')].some(p => p.value === 'fallback words'));
    ok('a refused picture read falls back to the text and shows no error', await page.evaluate(() => __editErrors.length === 0 && __toasts.length === 0 && !document.querySelector('.toast')),
      JSON.stringify(await page.evaluate(() => [__editErrors, __toasts])));

    await prompt.fill('');
    await clipboard({ image: 'refuse', text: 'refuse' });
    await openMenu();
    await choose('Paste');
    await page.waitForFunction(() => __editErrors.length > 0);
    ok('when the text cannot be read either, the error is shown', await page.evaluate(() => __editErrors[0].startsWith('paste:') && __toasts.some(t => /refused the text/.test(t))) && await prompt.inputValue() === '',
      JSON.stringify(await page.evaluate(() => [__editErrors, __toasts])));

    await prompt.fill('keep cut words');
    await prompt.evaluate(el => { el.focus(); el.setSelectionRange(5, 8); });
    await clipboard({ image: null, text: '' });
    rows = await openMenu(true);
    ok('with a selection Cut and Copy are enabled', rows.map(r => r.disabled).join() === 'false,false,false,false', JSON.stringify(rows));
    await choose('Cut');
    await page.waitForFunction(() => [...document.querySelectorAll('[data-view-id="prompt"]')].some(p => p.value === 'keep  words'));
    ok('Cut writes the selection to the clipboard, then deletes it', await page.evaluate(() => __runtime.some(c => c[0] === 'ClipboardSetText' && c[1] === 'cut')));
    await prompt.press('Control+z');
    ok('Undo puts the cut text back', await prompt.inputValue() === 'keep cut words', JSON.stringify(await prompt.inputValue()));
  } finally { await t.close(); }
})();
