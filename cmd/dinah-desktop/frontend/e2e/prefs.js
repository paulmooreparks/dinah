// The default and favorite models, in their own dialog over Settings.
const { launch } = require('./lib/page');
const windows = require('./lib/windows');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => {
      window.__calls = [];
      const models = [['adaptive', 'Adaptive'], ['swe-1-6', 'SWE-1.6'], ['claude-opus-5-5-medium', 'Claude Opus 5.5 Medium'], ['gpt-6-sol-high', 'GPT-6 Sol High']];
      const catalog = [
        { id: 'mode', name: 'Session Mode', category: 'mode', type: 'select', currentValue: 'accept-edits', options: [{ value: 'accept-edits', name: 'Code' }, { value: 'ask', name: 'Ask' }] },
        { id: 'model', name: 'Model', category: 'model', type: 'select', currentValue: 'swe-1-6', options: models.map(m => ({ value: m[0], name: m[1] })) }];
      let defaults = { model: 'claude-opus-5-5-medium' }, pinned = ['claude-opus-5-5-medium', 'gpt-6-sol-high'];
      const SetDefault = async (id, v) => { window.__calls.push(['SetDefault', id, v]); defaults = Object.assign({}, defaults); if (v === null) delete defaults[id]; else defaults[id] = v; return defaults; };
      const SetPinned = async (v, on) => { window.__calls.push(['SetPinned', v, on]); pinned = on ? pinned.concat([v]) : pinned.filter(x => x !== v); return pinned; };
      return {
        AgentProviders: async () => [{ id: 'devin', name: 'Devin', installed: true, available: true, status: 'detected' }, { id: 'codex', name: 'Codex', installed: true, available: true, status: 'detected' }, { id: 'claude', name: 'Claude', installed: true, available: true, status: 'detected' }, { id: 'ollama', name: 'Ollama', installed: true, available: true, status: 'detected' }],
        State: async () => ({ defaultProvider: 'devin', workspace: 'C:\\W', recent: ['C:\\W'], approvals: 'dinah', devin: 'd', catalog, defaults, pinned, titles: {}, templates: [] }),
        SetDefault, SetPinned,
        ProviderModelPreferences: async provider => ({provider,catalog,defaults,pinned}),
        SetDefaultForProvider: async (provider,id,value) => {if(provider!=='devin')throw new Error('Wrong provider');return SetDefault(id,value);},
        SetPinnedForProvider: async (provider,value,on) => {if(provider!=='devin')throw new Error('Wrong provider');return SetPinned(value,on);},
      };
    },
  });
  const p = windows(t.page);
  const { ok } = t;
  try {
    await p.waitForTimeout(400);
    await p.evaluate(() => document.querySelector('[data-cmd="settings"]').click());
    await p.locator('#settings').getByRole('tab', {name:'Chat',exact:true}).click();
    await p.waitForTimeout(200);
    ok('Settings has no model list in it', (await p.$('#settings .model-list')) === null);
    ok('Settings shows the default model on one line', (await p.textContent('#defaults')).includes('Claude Opus 5.5 Medium'));
    ok('Settings lists the favorites on one line', (await p.textContent('#favs-summary')) === 'Claude Opus 5.5 Medium, GPT-6 Sol High', await p.textContent('#favs-summary'));
    await t.shot('prefs-settings');
    await p.click('#defaults [data-open="model-prefs"]');
    await p.waitForTimeout(200);
    ok('Choose opens the dialog over Settings', await p.evaluate(() => document.getElementById('model-prefs').open && window.pudlWindows.state().open.includes('settings')));
    ok('the dialog marks the default', (await p.textContent('#prefs-list [aria-current]')) === 'Claude Opus 5.5 Medium');
    await t.shot('prefs-dialog');
    await p.click('#prefs-list a:has-text("GPT-6 Sol High")');
    await p.waitForTimeout(200);
    ok('pressing a model makes it the default', JSON.stringify((await p.evaluate(() => window.__calls)).pop()) === '["SetDefault","model","gpt-6-sol-high"]' && (await p.textContent('#prefs-default')).includes('GPT-6 Sol High'));
    await p.click('#prefs-list .model-row:has-text("SWE-1.6") .model-pin');
    await p.waitForTimeout(200);
    ok('the pin adds a favorite', JSON.stringify((await p.evaluate(() => window.__calls)).pop()) === '["SetPinned","swe-1-6",true]');
    await p.fill('#prefs-filter', 'adap');
    await p.waitForTimeout(100);
    ok('the filter narrows the list', (await p.$$('#prefs-list .model-row')).length === 1);
    await p.click('#prefs-own-default');
    await p.waitForTimeout(200);
    ok('Devin\'s own default clears it', JSON.stringify((await p.evaluate(() => window.__calls)).pop()) === '["SetDefault","model",null]' && await p.isDisabled('#prefs-own-default'));
    await p.click('#model-prefs .dialog-actions .btn');
    await p.waitForTimeout(200);
    ok('closing goes back to Settings', await p.evaluate(() => !document.getElementById('model-prefs').open && window.pudlWindows.state().open.includes('settings')));
    ok('Settings shows the changes', (await p.textContent('#defaults')).includes('Devin\'s own default') && (await p.textContent('#favs-summary')).includes('SWE-1.6'));
    await p.click('#favs-summary + [data-open="model-prefs"]');
    await p.waitForTimeout(150);
    ok('the favorites button opens the same dialog', await p.evaluate(() => document.getElementById('model-prefs').open));
  } finally { await t.close(); }
})();
