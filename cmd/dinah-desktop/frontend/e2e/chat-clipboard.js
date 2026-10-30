const { launch, PAGE } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspace: 'C:/Test', recent: ['C:/Test'] },
    app: () => ({
      OpenSessionAt: async (ws, id) => ({ sessionId: id, provider: 'codex', title: id, cwd: ws, open: true }),
      ResolveDocumentLink: async (cwd, path) => { __links.push({cwd, path}); return { ref: path, found: true, opens: 'system', path: cwd + '/' + path }; },
      OpenPath: async () => 'opened',
      Send: async (id, text, doc, attachments) => { __sent.push({ id, text, attachments }); }
    }),
    init: () => { window.__sent = []; window.__links = []; },
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
    ok('the prompt allows its native editing context menu', await a.locator('[data-view-id="prompt"]').evaluate(el => {
      const event = new MouseEvent('contextmenu', { bubbles: true, cancelable: true, button: 2 });
      el.dispatchEvent(event); return !event.defaultPrevented;
    }));
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
    ok('sent images retain thumbnails', await a.locator('.sent-images img').count() === 4);
    await a.locator('.sent-images img').first().click();
    await page.locator('#media-viewer[open]').waitFor();
    ok('sent thumbnail opens the image viewer', true);
    await page.keyboard.press('Escape');
    await page.evaluate(() => __emit('agent:update', {sessionId:'codex:first', update:{sessionUpdate:'agent_message_chunk', content:{type:'text',text:'Read [the design note](docs/proposals/note.md) and [a spaced path](<docs/my note.md>). [unsafe](javascript:alert)'}}}));
    await a.locator('[data-chip-local]').first().waitFor();
    ok('relative and spaced local Markdown links render safely', await a.locator('[data-chip-local]').count() === 2 && await a.locator('a[href^="javascript:"]').count() === 0);
    await a.locator('[data-chip-local]').first().click();
    ok('local link uses the owning chat execution folder', await page.evaluate(() => __links.length === 1 && __links[0].cwd.toLowerCase() === 'c:/test' && __links[0].path === 'docs/proposals/note.md'));
  } finally { await t.close(); }
})();
