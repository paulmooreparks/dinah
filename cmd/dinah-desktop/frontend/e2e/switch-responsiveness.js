// Facts and preparation are held independently to verify the approval boundary.
const { launch, OUT } = require('./lib/page');
const { choose } = require('./lib/commands');
const fs = require('fs'), path = require('path');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    window.__facts = []; window.__preparations = []; window.__sent = []; window.__timings = [];
    const info = id => ({ sessionId: id, title: id, cwd: 'C:/Fixture', provider: 'codex', open: true, queued: [], working: false });
    window.__review = (revision = 'r1') => ({ revision, warnings: ['Approval mode starts at the target provider default.'], facts: {
      kind: 'provider', from: { providerName: 'Codex' }, to: { providerName: 'Claude' },
      notCarried: [{ name: 'Approval mode', value: 'Current mode' }], running: { turn: false }
    } });
    function preview(id, provider) {
      const dlg = document.querySelector('dialog.switch-confirm[open]');
      const call = { id, provider, at: performance.now(), dialogOpen: !!dlg };
      __facts.push(call);
      return new Promise((resolve, reject) => { call.resolve = resolve; call.reject = reject; });
    }
    function prepare(id, provider, revision) {
      const root = document.querySelector('[data-win-kind="chat-session"] .switch-progress:not([hidden])');
      const call = { id, provider, revision, at: performance.now(), progressVisible: !!root && root.getBoundingClientRect().height > 0 };
      __preparations.push(call);
      return new Promise((resolve, reject) => { call.resolve = resolve; call.reject = reject; });
    }
    document.addEventListener('click', event => {
      if (event.target.closest('.menu-action') && /Claude/.test(event.target.textContent)) __timings.push({ kind: 'choice', at: performance.now() });
      if (event.target.closest('dialog.switch-confirm .btn-primary')) __timings.push({ kind: 'approval', at: performance.now() });
    }, true);
    const painted = new WeakMap();
    function measure(at) {
      const dlg = document.querySelector('dialog.switch-confirm[open]');
      if (dlg && dlg.getBoundingClientRect().height) {
        let state = painted.get(dlg);
        if (!state) { state = {}; painted.set(dlg, state); __timings.push({ kind: 'dialog visible', at }); }
        if (!state.enabled && !dlg.querySelector('.btn-primary').disabled) { state.enabled = true; __timings.push({ kind: 'approval enabled', at }); }
      }
      requestAnimationFrame(measure);
    }
    requestAnimationFrame(measure);
    return {
      State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'], workspaceRecords: [
        { id: 'fixture', name: 'Fixture', executionPath: 'C:/Fixture', sources: [{ id: 'local', kind: 'local-folder', path: 'C:/Fixture' }] },
        { id: 'destination', name: 'Destination', executionPath: 'C:/Destination', sources: [{ id: 'local', kind: 'local-folder', path: 'C:/Destination' }] }
      ] }),
      OpenSessionAt: async (ws, id) => info(id),
      AgentProviders: async () => ['codex', 'claude'].map(id => __provider(id)),
      PreviewSessionContinuation: preview, PreviewSessionRecovery: preview,
      ContinueSessionWithProvider: prepare, RecoverSession: prepare,
      PreviewSessionWorkspaceMove: preview, ContinueSessionInWorkspace: prepare,
      Send: async (id, text) => { __sent.push({ id, text }); },
      DismissSwitchFailure: async () => {}
    };
  } });
  const { page: p, ok } = t, evidence = [];
  try {
    const key = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'source' }));
    const win = p.locator('[data-win="' + key + '"]');
    await win.locator('[data-view-id=prompt]').fill('Keep this draft');
    const dialog = p.locator('dialog.switch-confirm[open]');
    const frames = () => p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    async function select() { await choose(p, win, 'provider', 'Claude'); await dialog.waitFor(); }
    await select();
    ok('the dialog exists before the facts bridge starts', await p.evaluate(() => __facts.length === 1 && __facts[0].dialogOpen));
    ok('approval waits for facts while Cancel remains usable', await dialog.getByRole('button', { name: 'Continue', exact: true }).isDisabled() && await dialog.getByRole('button', { name: 'Cancel' }).isEnabled());
    await frames();
    evidence.push(await p.evaluate(() => ({ phase: 'unresolved facts', at: performance.now(), facts: __facts.length, preparations: __preparations.length, dialog: !!document.querySelector('dialog.switch-confirm[open]') })));
    await dialog.getByRole('button', { name: 'Cancel' }).click();
    await select();
    await p.evaluate(() => __facts[0].resolve(__review('obsolete')));
    await frames();
    ok('an older reply cannot enable a newer dialog', await dialog.getByRole('button', { name: 'Continue', exact: true }).isDisabled());
    await p.keyboard.press('Escape');
    await p.evaluate(() => __facts[1].resolve(__review('also-obsolete')));
    await frames();
    ok('Cancel and Escape leave no dialog or continuation', await dialog.count() === 0 && await p.evaluate(() => __preparations.length === 0));
    ok('cancellation keeps the draft and controls usable', await win.locator('[data-view-id=prompt]').inputValue() === 'Keep this draft' && await win.locator('[data-view-id=send]').isEnabled());

    await select();
    await p.evaluate(() => __facts[2].reject(new Error('Facts unavailable')));
    await dialog.locator('.switch-facts-error').waitFor();
    ok('failed facts keep approval disabled and offer cancellation', await dialog.getByRole('button', { name: 'Continue', exact: true }).isDisabled());
    await dialog.getByRole('button', { name: 'Cancel' }).click();

    await select();
    await p.evaluate(() => __facts[3].resolve(__review('approved-r1')));
    await p.waitForFunction(() => !document.querySelector('dialog.switch-confirm .btn-primary').disabled);
    ok('missing handoff means preparation pending, with required warnings visible', /prepared after you continue/.test(await dialog.textContent()) && /Approval mode starts/.test(await dialog.textContent()) && !/no earlier conversation/.test(await dialog.textContent()));
    // Record every frame through approval and the unresolved preparation.
    await p.evaluate(k => {
      const root = document.querySelector('[data-win="' + k + '"]');
      window.__paintTrace = []; window.__sampling = true;
      function sample(at) {
        if (!__sampling) return;
        const progress = root.querySelector('.switch-progress');
        if (!progress) { __sampling = false; return; }
        __paintTrace.push({ at, progress: !progress.hidden && progress.getBoundingClientRect().height > 0, preparationCalls: __preparations.length });
        requestAnimationFrame(sample);
      }
      requestAnimationFrame(sample);
    }, key);
    await dialog.getByRole('button', { name: 'Continue', exact: true }).dblclick();
    await p.waitForFunction(() => __preparations.length === 1);
    await frames();
    ok('approval paints progress before invoking preparation', await p.evaluate(() => __paintTrace.some(f => f.progress && f.preparationCalls === 0) && __preparations[0].progressVisible));
    ok('duplicate approval starts one preparation with the approved revision', await p.evaluate(() => __preparations.length === 1 && __preparations[0].revision === 'approved-r1'));

    const otherKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'other' }));
    const other = p.locator('[data-win="' + otherKey + '"]');
    await other.locator('[data-view-id=prompt]').fill('Another chat stays usable');
    await other.locator('[data-view-id=send]').click();
    await p.waitForFunction(() => __sent.some(s => s.id === 'other'));
    ok('another chat can send during unresolved preparation', true);
    await p.evaluate(() => __preparations[0].reject(new Error('The chat changed; review the provider switch again')));
    await win.locator('.switch-failed').getByText('The chat changed', { exact: false }).waitFor();
    await p.evaluate(k => pudlWindows.raise(k), key);
    ok('stale revision restores the composer without retrying', await win.locator('.composer').isVisible() && await p.evaluate(() => __preparations.length === 1));
    await win.locator('.switch-dismiss').click();
    await select();
    ok('retry requires a fresh confirmation and fresh warnings', await dialog.getByRole('button', { name: 'Continue', exact: true }).isDisabled() && await p.evaluate(() => __facts.length === 5 && __preparations.length === 1));
    await p.evaluate(() => __facts[4].resolve(__review('approved-r2')));
    await dialog.getByRole('button', { name: 'Continue', exact: true }).click();
    await p.waitForFunction(() => __preparations.length === 2);
    await p.evaluate(() => __preparations[1].reject(new Error('Preparation failed')));
    await win.locator('.switch-failed').getByText('Preparation failed', { exact: false }).waitFor();
    ok('preparation failure restores controls and retains its explanation', await win.locator('[data-view-id=send]').isEnabled() && await win.locator('.switch-failure').isVisible());
    await win.locator('.switch-dismiss').click();
    await select();
    await p.evaluate(k => pudlWindows.close(k), key);
    await p.evaluate(() => __facts[5].resolve(__review('disposed')));
    await frames();
    ok('closing a window discards its pending facts', await dialog.count() === 0 && await p.evaluate(() => __preparations.length === 2));
    await p.evaluate(k => pudlWindows.raise(k), otherKey);
    await p.getByRole('menuitem', { name: 'Actions', exact: true }).click();
    await p.locator('.menu-panel:popover-open .menu-action', { hasText: 'Continue in Another Workspace' }).click();
    const pick = p.locator('dialog.dinah-new-chat-workspace[open]');
    await pick.waitFor();
    await pick.locator('select').selectOption('0');
    await pick.getByRole('button', { name: 'Continue', exact: true }).click();
    await dialog.waitFor();
    ok('a workspace move also opens before unresolved facts', await dialog.getByRole('button', { name: 'Continue', exact: true }).isDisabled() && await p.evaluate(() => __facts[6].dialogOpen));
    await p.keyboard.press('Escape');
    await p.evaluate(() => __facts[6].resolve(__review('cancelled-workspace')));
    await frames();
    ok('cancelling a workspace move never starts preparation', await dialog.count() === 0 && await p.evaluate(() => __preparations.length === 2));
    const lateKey = await p.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'late-result' }));
    const late = p.locator('[data-win="' + lateKey + '"]');
    await choose(p, late, 'provider', 'Claude');
    await dialog.waitFor();
    await p.evaluate(() => __facts[7].resolve(__review('late-result-r1')));
    await dialog.getByRole('button', { name: 'Continue', exact: true }).click();
    await p.waitForFunction(() => __preparations.length === 3);
    await p.evaluate(k => pudlWindows.close(k), lateKey);
    await p.evaluate(() => __preparations[2].resolve({ session: { sessionId: 'late-next', provider: 'claude', cwd: 'C:/Fixture', open: true }, history: [] }));
    await frames();
    ok('a late continuation result cannot rebind a closed window', await p.evaluate(() => dinahChats.ref('late-next') === null));
    evidence.push(await p.evaluate(() => { __sampling = false; return { phase: 'approval frames', frames: __paintTrace, timings: __timings, facts: __facts.map(({ id, at, dialogOpen }) => ({ id, at, dialogOpen })), preparations: __preparations.map(({ id, revision, at, progressVisible }) => ({ id, revision, at, progressVisible })) }; }));
    fs.mkdirSync(OUT, { recursive: true });
    fs.writeFileSync(path.join(OUT, 'switch-responsiveness.json'), JSON.stringify(evidence, null, 2));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
