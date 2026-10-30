// The usage meter says what its size is. A size the person chose is the context length; a size that
// is the model's own limit is not what the server runs with, so the words say the real size may be
// lower, and a request past the size says it is over the context length instead of showing a
// figure above 100 percent.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => ({
      AgentProviders: async () => [{ id: 'ollama', installed: true, available: true }],
      OpenSessionAt: async (cwd, sessionId) => ({ sessionId, cwd, provider: 'ollama', open: true, chatState: { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } } })
    }),
    ready: 'chat',
  });
  const { page: p, ok } = t;
  try {
    const key = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'ollama:meter' }));
    const win = p.locator('[data-win="' + key + '"]');
    await p.waitForFunction(k => /Idle/.test(document.querySelector('[data-win="' + k + '"] .chat-working-chip').title), key);
    const usage = (used, size, sizeKind, windowKnown) => p.evaluate(([used, size, sizeKind, windowKnown]) => __emit('agent:update', { sessionId: 'ollama:meter', update: { sessionUpdate: 'usage_update', used, size, sizeKind, windowKnown } }), [used, size, sizeKind, windowKnown]);
    const button = win.locator('[data-view-id="usage-button"]');
    const title = async () => button.getAttribute('title');

    await usage(32768, 65536, 'chosen', true);
    await p.waitForFunction(k => /50%/.test(document.querySelector('[data-win="' + k + '"] [data-view-id="usage-button"]').title), key);
    ok('a size the person chose is called the context length', /context length of 65,536 tokens/.test(await title()) && !/server/.test(await title()), await title());

    await usage(100000, 262144, 'model', false);
    await p.waitForFunction(k => /model's limit/.test(document.querySelector('[data-win="' + k + '"] [data-view-id="usage-button"]').title), key);
    ok('a size that is the model\'s limit says so, and that the server\'s size may be lower', /model's limit of 262,144 tokens/.test(await title()) && /may be lower/.test(await title()), await title());

    await usage(40000, 200000);
    await p.waitForFunction(k => /200,000/.test(document.querySelector('[data-win="' + k + '"] [data-view-id="usage-button"]').title), key);
    ok('a provider that sends no windowKnown has a known window', !/may be lower/.test(await title()) && /200,000/.test(await title()), await title());
    await usage(119298, 65536, 'chosen', true);
    await p.waitForFunction(k => /^Over$/.test(document.querySelector('[data-win="' + k + '"] [data-view-id="usage-words"]').textContent), key);
    ok('a request past the size says it is over, not a figure above 100 percent', !/\d{3}%/.test(await button.innerText()) && /over the context length/.test(await title()), await title() + ' | ' + await button.innerText());
    ok('and the meter is full, marked as high', await win.locator('[data-view-id="usage-fill"]').evaluate(el => el.style.width === '100%') && await button.evaluate(el => el.classList.contains('usage-high')));
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
