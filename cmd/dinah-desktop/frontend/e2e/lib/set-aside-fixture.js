const { launch, OUT } = require('./page');
const fs = require('fs');
const path = require('path');
function copy(n, extra = {}) {
  return Object.assign({ id: n.toString(16).padStart(64, '0'), program: 'window', name: (extra.program || 'window') + '.exe.old-20261112' + String(n).padStart(9, '0'), sizeBytes: n * 1024,
    date: '2026-11-12T08:00:00Z', runningPids: [], fallback: false, canRemove: true, reasonCode: '', reason: '' }, extra);
}
async function start(options = {}) {
  const t = await launch({ args: { items: [copy(1), copy(2, { program: 'command', fallback: true, canRemove: false, reasonCode: 'fallback', reason: 'This is the newest fallback.' }), copy(3, { program: 'tools', runningPids: [321], canRemove: false, reasonCode: 'in_use', reason: 'A running process uses this copy.' })], ...options }, app: data => {
    const clone = value => JSON.parse(JSON.stringify(value));
    window.__asideItems = clone(data.items); window.__asideWarnings = data.warnings || [];
    window.__asideCalls = []; window.__asideHold = {}; window.__asideFail = {};
    const api = { ProductServices: async () => data.updates ? ['updates'] : [] };
    if (data.unsupported) return api;
    api.ListSetAsidePrograms = async () => {
      __asideCalls.push(['list']);
      const value = { items: clone(__asideItems), totalBytes: __asideItems.reduce((sum, item) => sum + item.sizeBytes, 0), warnings: clone(__asideWarnings) };
      if (__asideHold.list) await new Promise(resolve => { window.__asideReleaseList = resolve; });
      if (__asideFail.list) throw new Error('Synthetic inventory failure');
      return window.__asideMalformedList ? { items: [{}] } : value;
    };
    api.RemoveSetAsidePrograms = async input => {
      __asideCalls.push(['remove', clone(input)]);
      if (__asideHold.remove) await new Promise(resolve => { window.__asideReleaseRemove = resolve; });
      if (__asideFail.remove) throw new Error('Synthetic removal failure');
      const result = { items: [], removedBytes: 0 };
      input.ids.forEach(id => {
        const item = __asideItems.find(item => item.id === id);
        if (!item || !item.canRemove) result.items.push({ id, state: 'refused', reasonCode: item?.reasonCode || 'changed_or_missing', reason: item?.reason || 'The selected identity changed or disappeared.' });
        else { result.items.push({ id, state: 'removed', reasonCode: '', reason: '' }); result.removedBytes += item.sizeBytes; __asideItems = __asideItems.filter(item => item.id !== id); }
      });
      if (window.__asideLostResponse) throw new Error('Synthetic lost removal response');
      return window.__asideMalformedRemoval ? { items: [], removedBytes: 0 } : result;
    };
    return api;
  }, init: data => { if (data.holdList && window.__asideHold) __asideHold.list = true; } });
  try {
    await t.page.evaluate(section => dinahWorkbench.open('settings', section), options.unsupported ? '' : 'set-aside');
    if (!options.unsupported && !options.holdList) await ready(t);
    return t;
  } catch (error) { await t.close(); throw error; }
}
async function ready(t) { await t.page.waitForFunction(() => document.querySelector('#set-aside-refresh').getAttribute('aria-disabled') === 'false'); }
const check = (t, n) => t.page.locator('[data-copy-id="' + n.toString(16).padStart(64, '0') + '"] input');
const removals = t => t.page.evaluate(() => __asideCalls.filter(call => call[0] === 'remove'));
async function finish(t, name) {
  if (await t.page.evaluate(() => !!window.__asideCalls)) fs.writeFileSync(path.join(OUT, name + '-calls.json'), JSON.stringify(await t.page.evaluate(() => __asideCalls), null, 2));
  t.ok('no native or external action', !(await t.page.evaluate(() => __runtime)).length);
  await t.close();
}
module.exports = { start, ready, copy, check, removals, finish };
