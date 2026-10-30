// Devin's model prices in the model list, under Usage and in Settings, from
// a stub of the Go side: a matched model shows its price as a quiet second
// line, an unmatched one shows nothing, and with no prices known the list is
// drawn exactly as it was before prices existed.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => {
      const models = [['adaptive', 'Adaptive'], ['swe-2-high', 'SWE-2 High'], ['claude-opus-5-5-medium', 'Claude Opus 5.5 Medium'], ['gpt-6-sol-high', 'GPT-6 Sol High']];
      const mkOpts = model => [
        { id: 'mode', name: 'Session Mode', category: 'mode', type: 'select', currentValue: 'accept-edits', options: [{ value: 'accept-edits', name: 'Code' }, { value: 'ask', name: 'Ask' }] },
        { id: 'model', name: 'Model', category: 'model', type: 'select', currentValue: model, options: models.map(m => ({ value: m[0], name: m[1] })) }];
      const ws = 'C:\\Work\\Prices';
      let opts = mkOpts('claude-opus-5-5-medium');
      return {
        State: async () => ({ defaultProvider: 'devin', workspace: ws, recent: [ws], approvals: 'dinah', devin: 'devin.exe', catalog: mkOpts('adaptive'), defaults: {}, pinned: ['swe-2-high'], showThinking: false }),
        Sessions: async () => [{ sessionId: 's1', provider: 'devin', cwd: ws, title: 'A Devin chat', updatedAt: '2026-10-28T07:00:00Z' }],
        OpenSessionAt: async (workspace, id) => {
          setTimeout(() => window.__emit('agent:options', { sessionId: id, options: opts }), 0);
          return { sessionId: id, provider: 'devin', cwd: ws, title: 'A Devin chat', open: true, working: false, queued: [] };
        },
        SetSessionOption: async (id, o, v) => { opts = opts.map(x => x.id === o ? Object.assign({}, x, { currentValue: v }) : x); return opts; },
        ModelPrices: async provider => { window.__calls.push(['ModelPrices', provider]); return window.__prices; },
        AgentProviders: async () => [window.__provider('devin', { path: 'C:/devin.exe', detail: '' })],
        Send: async () => {}, Queue: async () => {}, Steer: async () => {}, Unqueue: async () => {}, Answer: async () => {}, Cancel: async () => {}, Stop: async () => {},
        SetWorkspace: async () => ({}), SetApprovals: async () => {}, SetDefault: async () => ({}), SetPinned: async () => [], SetShowThinking: async () => {},
        SaveHistory: async () => '', ClearHistory: async () => {}, SaveRun: async () => ''
      };
    },
    init: () => { window.__calls = []; window.__prices = {}; },
  });
  const { page: p, ok } = t;
  require('./lib/windows')(p);
  try {
    await p.waitForTimeout(500);
    await p.click('#sessions a:has-text("A Devin chat")');
    await p.locator('.win[data-chat-current] [data-command="model"]:not([hidden])').first().waitFor({ state: 'attached', timeout: 10000 }).catch(() => {});
    await p.waitForTimeout(300);

    const list = 'dialog.models-dialog[open] .model-list';
    const rows = () => p.$$eval(list + ' .model-row .md-item', els => els.map(a => ({ text: a.textContent, meta: (a.querySelector('.md-meta') || {}).textContent || null, color: a.querySelector('.md-meta') ? getComputedStyle(a.querySelector('.md-meta')).color : null })));
    const openModels = async () => {
      await p.locator('.menubar-title').filter({ hasText: /^Actions$/ }).click();
      await p.locator('#menubar-panel').getByText('Choose the Model…', { exact: true }).click();
      await p.waitForTimeout(200);
    };
    const closeModels = () => p.evaluate(() => document.querySelector('dialog.models-dialog[open]').close());

    // No prices known: every row is the model's name alone.
    await openModels();
    ok('opening the list asks for Devin\'s prices', (await p.evaluate(() => window.__calls)).some(c => c[0] === 'ModelPrices' && c[1] === 'devin'));
    const before = await p.innerHTML(list);
    const plain = await rows();
    ok('with no prices each row is the name alone', plain.length === 4 && plain.every(r => r.meta === null) && plain.map(r => r.text).join('|') === 'SWE-2 High|Adaptive|Claude Opus 5.5 Medium|GPT-6 Sol High', JSON.stringify(plain));
    await t.shot('model-prices-none');

    // Prices arrive while the list is open: it is drawn again with them.
    await p.evaluate(() => window.__emit('models:prices', { provider: 'devin', prices: {
      'swe-2-high': { costSummary: '$0.5 / MTok In · $2.5 / MTok Out', costTier: 'High cost', contextTokens: 262000 },
      'claude-opus-5-5-medium': { costSummary: '$4 / MTok In · $20 / MTok Out', contextTokens: 1000000 },
      'gpt-6-sol-high': { costTier: 'Med cost' },
      'not-offered': { costSummary: '$9 / MTok In' } } }));
    await p.waitForTimeout(200);
    const priced = await rows();
    const by = name => priced.find(r => r.text.startsWith(name)) || {};
    ok('a matched model shows its price, tier and context as a second line', by('SWE-2 High').meta === '$0.5 / MTok In · $2.5 / MTok Out · High cost · 262K context', JSON.stringify(by('SWE-2 High')));
    ok('a missing tier is left out', by('Claude Opus 5.5 Medium').meta === '$4 / MTok In · $20 / MTok Out · 1M context', JSON.stringify(by('Claude Opus 5.5 Medium')));
    ok('a model with no price shows nothing', by('Adaptive').meta === null && by('Adaptive').text === 'Adaptive');
    ok('a price without a summary shows nothing', by('GPT-6 Sol High').meta === null && by('GPT-6 Sol High').text === 'GPT-6 Sol High');
    const muted = await p.evaluate(() => { const s = document.createElement('span'); s.style.color = 'var(--text-muted)'; document.body.appendChild(s); const c = getComputedStyle(s).color; s.remove(); return c; });
    ok('the price is in the quiet metadata style', await p.$eval(list + ' .model-price', el => el.classList.contains('md-meta')) && by('SWE-2 High').color === muted, muted + ' ' + by('SWE-2 High').color);
    await t.shot('model-prices-shown');

    // The prices go away, as after a failed read: the list is as it was.
    await p.evaluate(() => window.__emit('models:prices', { provider: 'devin', prices: {} }));
    await p.waitForTimeout(200);
    ok('with the prices gone the list is exactly as before', (await p.innerHTML(list)) === before);
    // Prices for another provider are not Devin's.
    await p.evaluate(() => window.__emit('models:prices', { provider: 'codex', prices: { 'swe-2-high': { costSummary: '$1' } } }));
    await p.waitForTimeout(150);
    ok('prices sent for another provider are ignored', (await p.innerHTML(list)) === before);
    await closeModels();

    // Under Usage: the chat's model and its price, and nothing without one.
    await p.click('.win[data-chat-current] [data-view-id=usage-button]');
    await p.waitForTimeout(200);
    ok('Usage shows no model price when none is known', await p.$eval('.win[data-chat-current] [data-view-id=model-price]', el => el.hidden) && await p.$eval('.win[data-chat-current] [data-view-id=model-price-title]', el => el.hidden));
    await p.evaluate(() => window.__emit('models:prices', { provider: 'devin', prices: { 'claude-opus-5-5-medium': { costSummary: '$4 / MTok In · $20 / MTok Out', costTier: 'High cost', contextTokens: 1000000 } } }));
    await p.waitForTimeout(200);
    const usage = await p.textContent('.win[data-chat-current] [data-view-id=model-price]');
    ok('Usage shows the chat model\'s price when it arrives', !(await p.$eval('.win[data-chat-current] [data-view-id=model-price]', el => el.hidden)) && usage.includes('Claude Opus 5.5 Medium') && usage.includes('$4 / MTok In · $20 / MTok Out') && usage.includes('High cost · 1M context'), usage);
    await t.shot('model-prices-usage');
    await p.keyboard.press('Escape');

    // The model menu's rows show the price under the name.
    // A folded toolbar group's menu shows rows' words only (PUDL), so the
    // rows are read from the command itself, as the model's own menu draws
    // them.
    const detail = await p.evaluate(() => {
      const ctl = document.querySelector('.win[data-chat-current] [data-command="model"]');
      const cmd = window.pudlCommands.find('model', ctl);
      return (window.pudlCommands.value(cmd, 'items') || []).filter(i => i && i.id).map(i => ({ words: i.label, detail: i.detail === undefined ? null : i.detail }));
    });
    const row = detail.find(r => r.words.startsWith('Claude Opus 5.5 Medium')) || {}, other = detail.find(r => r.words.startsWith('SWE-2 High')) || {};
    ok('the model menu shows the chosen model\'s price, and nothing for a model without one', row.detail === '$4 / MTok In · $20 / MTok Out' && other.words && other.detail === null, JSON.stringify(detail));

    // Settings › Agents › Devin says where prices come from.
    await p.evaluate(() => window.dinahWorkbench.open('settings'));
    await p.locator('#settings').getByRole('tab', { name: 'Agents', exact: true }).click();
    await p.locator('#settings [data-provider="devin"]').waitFor({ timeout: 5000 }).catch(() => {});
    const note = await p.locator('#settings [data-provider="devin"] .devin-prices-note').textContent().catch(() => '');
    ok('Settings › Agents › Devin says where the prices come from', note === 'Model prices in the model list come from the Devin command line, and may be missing if Devin changes what it prints.', note);
  } finally { await t.close(); }
})();
