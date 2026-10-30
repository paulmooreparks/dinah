// State events carry stateGen and stateVer from their owner's counter; the page applies a
// snapshot only if its version is higher than the one it holds for the same kind and owner,
// a different generation always wins, and an unversioned snapshot is applied as before.
// Rereads go through the same rule as events.
const { launch } = require('./lib/page');

(async () => {
  const t = await launch({
    viewport: { width: 1400, height: 900 },
    app: () => {
      window.__reads = { states: [], accounts: [], sessions: [] };
      return {
        ProviderStates: async () => window.__reads.states,
        ProviderAccounts: async () => window.__reads.accounts,
        Sessions: async () => window.__reads.sessions, SessionsAt: async () => window.__reads.sessions,
        OpenSessionAt: async (ws, id) => ({ sessionId: id, title: 'opened', provider: 'codex', cwd: ws, open: true }),
      };
    },
    ready: () => window.dinahWorkbench && window.dinahEditor,
  });
  const { page, ok } = t;
  const title = win => page.locator('[data-win="' + win + '"] .win-title').first().textContent();
  try {
    // The rule itself.
    const rule = await page.evaluate(() => {
      const o = window.dinahStateOrder, v = (gen, ver) => ({ stateGen: gen, stateVer: ver });
      const out = {};
      out.first = o.accept('x', 'a', v('g', 5));
      out.older = o.accept('x', 'a', v('g', 4));
      out.same = o.accept('x', 'a', v('g', 5));
      out.newer = o.accept('x', 'a', v('g', 6));
      out.otherGen = o.accept('x', 'a', v('h', 1));
      out.afterGen = o.accept('x', 'a', v('h', 2)) && !o.accept('x', 'a', v('h', 1));
      out.otherOwner = o.accept('x', 'b', v('g', 1));
      out.otherName = o.accept('y', 'a', v('g', 1));
      out.unversioned = o.accept('x', 'a', { title: 'saved' });
      out.again = o.accept('x', 'a', v('h', 2), true) && !o.accept('x', 'a', v('h', 1), true);
      o.forget('a');
      out.kept = o.held('x', 'b') && o.held('x', 'b').ver === 1;
      return out;
    });
    ok('a first snapshot is applied, an older or repeated one is not, a newer one is', rule.first && !rule.older && !rule.same && rule.newer, JSON.stringify(rule));
    ok('a different generation wins whatever its version, and the old one is then older', rule.otherGen && rule.afterGen);
    ok('kinds and owners are kept apart, and an unversioned snapshot is applied', rule.otherOwner && rule.otherName && rule.unversioned);
    ok('a read may be applied again by another window but never when older', rule.again);
    ok('forgetting an owner leaves the others', rule.kept);

    // Events reach the handlers in the order of their versions, not of their arrival.
    await page.evaluate(() => {
      window.__seen = [];
      for (const name of ['agent:state', 'agent:account', 'agent:queue', 'agent:options', 'settings:changed', 'update:state']) {
        dinahHost.on(name, d => window.__seen.push(name + ':' + (d.tag || '')));
      }
    });
    const emit = (name, data) => page.evaluate(([n, d]) => __emit(n, d), [name, data]);
    const seen = async () => { await page.waitForFunction(() => window.__seen.length > 0); await page.waitForTimeout(400); return (await page.evaluate(() => window.__seen.splice(0))).join(','); };
    const stamp = (gen, ver, tag, extra) => Object.assign({ stateGen: gen, stateVer: ver, tag }, extra);
    await emit('agent:state', stamp('p1', 2, 'new', { provider: 'codex' }));
    await emit('agent:state', stamp('p1', 1, 'old', { provider: 'codex' }));
    await page.waitForTimeout(200);
    const first = await seen(); ok('an older provider state that arrives last is ignored', first === 'agent:state:new', first);
    await emit('agent:state', stamp('p1', 1, 'claude', { provider: 'claude' }));
    await page.waitForTimeout(150);
    const second = await seen(); ok('another provider has its own count', second === 'agent:state:claude', second);
    await emit('agent:account', stamp('w', 5, 'new', { provider: 'codex' }));
    await emit('agent:account', stamp('w', 3, 'old', { provider: 'codex' }));
    await emit('agent:queue', stamp('c1', 8, 'new', { sessionId: 's1' }));
    await emit('agent:queue', stamp('c1', 7, 'old', { sessionId: 's1' }));
    await emit('agent:options', stamp('c1', 9, 'new', { sessionId: 's1' }));
    await emit('agent:options', stamp('c1', 4, 'old', { sessionId: 's1' }));
    await emit('settings:changed', stamp('w', 6, 'new', {}));
    await emit('settings:changed', stamp('w', 2, 'old', {}));
    await emit('update:state', stamp('u', 3, 'new', {}));
    await emit('update:state', stamp('u', 1, 'old', {}));
    await page.waitForTimeout(300);
    const rest = await seen(); ok('the same holds for accounts, queues, options, settings and the update state', rest.split(',').sort().join(',') === 'agent:account:new,agent:options:new,agent:queue:new,settings:changed:new,update:state:new', rest);
    await emit('agent:queue', stamp('c2', 1, 'reopened', { sessionId: 's1' }));
    await emit('agent:state', { provider: 'codex', tag: 'plain' });
    await page.waitForTimeout(200);
    ok('a new generation is applied at once, and an event with no version as before', (await seen()) === 'agent:queue:reopened,agent:state:plain');

    // A chat's information: the newer title stays.
    const win = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'codex:order' }));
    await page.locator('[data-win="' + win + '"] [data-view-id="prompt"]').waitFor();
    const info = (ver, name, gen) => emit('agent:session', { sessionId: 'codex:order', title: name, provider: 'codex', cwd: 'C:/Fixture', open: true, stateGen: gen || 'c1', stateVer: ver });
    await info(10, 'Newer title'); await info(9, 'Older title');
    await page.waitForTimeout(300);
    ok('an older session snapshot that arrives last is ignored', (await title(win)).includes('Newer title'), await title(win));
    await info(1, 'Reopened title', 'c2');
    await page.waitForTimeout(300);
    ok('a session snapshot of a new generation is applied', (await title(win)).includes('Reopened title'), await title(win));

    // A reread older than an event already applied is ignored; a newer one is applied.
    await page.evaluate(() => { window.__reads.states = [{ provider: 'codex', state: 'ready', detail: 'old read', stateGen: 'p1', stateVer: 1 }]; });
    await page.evaluate(() => __resync());
    await page.waitForTimeout(300);
    ok('a reread older than the event already applied does not replace it', await page.evaluate(() => dinahStateOrder.held('agent:state', 'codex').ver === 2));
    await page.evaluate(() => { window.__reads.states = [{ provider: 'codex', state: 'ready', detail: 'new read', stateGen: 'p1', stateVer: 7 }]; });
    await page.evaluate(() => __resync());
    await page.waitForTimeout(300);
    ok('a newer reread is applied and becomes the one held', await page.evaluate(() => dinahStateOrder.held('agent:state', 'codex').ver === 7));
  } finally {
    await t.close();
  }
})().catch(e => { console.error(e); process.exitCode = 1; });
