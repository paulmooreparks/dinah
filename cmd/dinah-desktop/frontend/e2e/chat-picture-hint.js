// The line beside an attached picture appears only for a chat whose model cannot read pictures,
// as the engine says (ChatImageInput: no), and names no provider or model. A model that can, or
// that is not known to, gets no line.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    state: { workspace: 'C:/Test', recent: ['C:/Test'] },
    app: () => ({
      OpenSessionAt: async (ws, id) => ({ sessionId: id, provider: 'ollama', title: id, cwd: ws, open: true }),
      ChatImageInput: async () => window.__img || 'unknown'
    }),
    ready: () => window.dinahWorkbench && window.dinahEditor,
  });
  const { page, ok } = t;
  try {
    const paste = el => el.evaluate(el => {
      const bytes = Uint8Array.from(atob('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mP8/x8AAwMCAO+jRZkAAAAASUVORK5CYII='), c => c.charCodeAt(0));
      const data = new DataTransfer(); data.items.add(new File([bytes], 'picture.png', { type: 'image/png' }));
      const event = new Event('paste', { bubbles: true, cancelable: true });
      Object.defineProperty(event, 'clipboardData', { value: data });
      el.dispatchEvent(event);
    });
    const info = answer => page.evaluate(answer => { window.__img = answer; __emit('agent:session', { sessionId: 'ollama:pics', cwd: 'C:/Test', provider: 'ollama', open: true }); }, answer);
    await info('unknown');
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'ollama:pics' }));
    const win = page.locator('[data-win="' + key + '"]');
    await win.locator('[data-view-id="prompt"]').waitFor();
    await paste(win.locator('[data-view-id="prompt"]'));
    await win.locator('.attachment-name').waitFor();
    const notes = () => win.locator('.attachment-note').allTextContents();
    await page.waitForTimeout(200);
    ok('with nothing known about the model there is no line', (await notes()).length === 0, (await notes()).join('|'));

    await info('yes');
    await page.waitForTimeout(300);
    ok('a model that can read pictures gets no line', (await notes()).length === 0, (await notes()).join('|'));

    await info('no');
    await win.locator('.attachment-note').waitFor();
    const said = (await notes()).join('|');
    ok('a model that cannot read pictures gets a warning saying so', /can't read pictures/.test(said) && /remove the picture/.test(said), said);
    ok('the warning names no provider or model', !/Claude|Devin|Ollama|Codex/.test(said), said);
    ok('and it is marked as a warning', await win.locator('.attachment-note.attachment-warn').count() === 1);

    await info('yes');
    await page.waitForFunction(k => !document.querySelector('[data-win="' + k + '"] .attachment-note'), key);
    ok('the warning goes when the model can read pictures', true);
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
