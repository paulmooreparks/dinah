// Native UI state across fresh browser contexts: window geometry, docks and
// scroll restored after delayed document and chat loads, the new-window
// preference, an explicit address over the saved layout, migration of named
// preferences, serialized writes and persistence failures. One Node-side
// store stands in for the native state, and each start is a fresh browser
// reading it.
const { launch, PAGE } = require('./lib/page');
(async () => {
  let nativeState = { version: 1, values: {} }, writes = 0;
  // Errors and failures of the browsers already closed, carried into the last
  // one's close.
  const earlier = { errors: [], failures: [] };
  let t;
  const ok = (name, passed, detail) => t.ok(name, passed, detail);
  async function start(options = {}) {
    t = await launch({
      viewport: { width: 1440, height: 1000 },
      state: { documentViews: { 'text/markdown': 'split' }, newWindowMode: options.mode || 'floating' },
      app: () => {
        const pendingDocuments = [], pendingChats = [];
        // A request that arrives after its release goes straight through, so a
        // slow page cannot leave one waiting for a release that has passed.
        let documentsReady = false, chatsReady = false;
        window.__documentsReady = () => { documentsReady = true; pendingDocuments.splice(0).forEach(resolve => resolve()); };
        window.__chatsReady = () => { chatsReady = true; pendingChats.splice(0).forEach(resolve => resolve()); };
        const info = { sessionId: 'reading-chat', provider: 'codex', cwd: 'C:/Fixture', title: 'Reading Chat' };
        return {
          // The fixture's default BusOverview has an older shape (groups, not
          // rooms), which the Group Chats pane cannot draw; this is the shape
          // the page reads.
          BusOverview: async () => ({ inbox: [], unread: 0, rooms: [], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
          ReadUIState: () => nativeRead(), SaveUIState: state => nativeSave(state),
          AgentProviders: async () => [{ id: 'codex', name: 'Codex', available: true, installed: true }],
          Sessions: async () => [info],
          OpenSessionAt: async () => {
            if (window.__options.delay && !chatsReady) await new Promise(resolve => pendingChats.push(resolve));
            for (let i = 0; i < 45; i++) __emit('agent:update', { sessionId: info.sessionId, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: '\n\nParagraph ' + i + '. '.repeat(30) } } });
            return { ...info, open: true };
          },
          PreviewWorkspaceResource: async ref => {
            if (window.__options.delay && !documentsReady) await new Promise(resolve => pendingDocuments.push(resolve));
            return { resource: { ref, title: ref.path, revision: '1', actions: ['preview'] }, title: ref.path, problems: [], blocks: Array.from({ length: 80 }, (_, i) => ({ line: i + 1, html: '<p>Passage ' + i + '</p>' + (i === 0 ? '<pre style="width:1800px">Wide document content</pre>' : '') })) };
          },
        };
      },
      goto: false,
      timeout: 12000,
    });
    t.errors.push(...earlier.errors); t.failures.push(...earlier.failures);
    earlier.errors = []; earlier.failures = [];
    await t.context.exposeFunction('nativeRead', async () => { if (options.readError) throw new Error('Unreadable native UI state'); return nativeState; });
    await t.context.exposeFunction('nativeSave', async state => { nativeState = state; writes++; });
    await t.context.addInitScript(options => {
      Object.entries(options.legacy || {}).forEach(([key, value]) => localStorage.setItem(key, value));
      window.__options = options;
    }, options);
    await t.page.goto(PAGE + (options.query || ''));
    await t.waitReady();
    return { context: t.context, page: t.page };
  }
  // Closes this browser, keeping what it found for the last one's close.
  async function stop() {
    // browser.close has no deadline of its own, so a browser that cannot exit
    // must fail here in forty-five seconds rather than hang the suite forever.
    await t.deadline('the browser to close', t.browser.close(), 45000).catch(error => {
      // A wedged browser outlives its suite otherwise (its leader never dies).
      t.ok('the browser closes', false, String(error));
      try { const proc = t.browser.process(); if (proc) proc.kill(); } catch (killError) {}
    });
    earlier.errors.push(...t.errors); earlier.failures.push(...t.failures);
  }
  try {
    let { page } = await start();
    const first = await page.evaluate(() => dinahWorkbench.resourceItem({ workspaceId: 'w1', sourceId: 'local', path: 'first.md' }, 'first.md'));
    const firstKey = await page.evaluate(item => dinahWorkbench.open(item), first);
    await page.locator('[data-win="' + firstKey + '"] .doc-block').first().waitFor();
    ok('new windows float by default', await page.evaluate(key => pudlWindows.state().place[key].mode === 'floating', firstKey));
    await page.evaluate(key => {
      const url = new URL(location.href); url.searchParams.set('p.' + key, 'floating:0.12,0.09,0.65,0.72');
      history.replaceState(null, '', url); dispatchEvent(new PopStateEvent('popstate'));
    }, firstKey);
    await page.evaluate(() => dinahWorkbench.open('chat-browser'));
    await page.evaluate(() => { pudlWindows.dock('chat-browser', 'right'); pudlWindows.minimize('workspace'); });
    await page.evaluate(key => pudlWindows.raise(key), firstKey);
    const scroll = page.locator('[data-win="' + firstKey + '"] [data-doc-scroll]');
    await scroll.evaluate(el => { el.scrollTop = 620; el.scrollLeft = 85; });
    await page.waitForFunction(key => { const value = dinahUIState.get('dinah-scroll:' + key + ':document'); return value && value.top === 620 && value.left === 85; }, firstKey);
    const expected = await page.evaluate(key => ({ place: pudlWindows.state().place[key], docks: pudlWindows.state().docks }), firstKey);
    await t.deadline('geometry, docks and document scroll to be saved', page.evaluate(() => dinahUIState.flush()), 30000);
    ok('native state stores geometry, docking and document scroll', writes > 0 && nativeState.values['dinah-workbench-layout'].includes('dock-right') && nativeState.values['dinah-scroll:' + firstKey + ':document'].top === 620);
    await stop();

    ({ page } = await start({ delay: true, mode: 'maximized' }));
    // The docks are compared edge by edge, since the order of the edges in
    // the state means nothing.
    ok('a fresh browser context restores native placement and dock state', await page.evaluate(({ key, expected }) => JSON.stringify(pudlWindows.state().place[key]) === JSON.stringify(expected.place) &&
      ['left', 'right', 'top', 'bottom'].every(edge => JSON.stringify(pudlWindows.state().docks[edge]) === JSON.stringify(expected.docks[edge])), { key: firstKey, expected }));
    ok('initial empty content does not replace the saved scroll', nativeState.values['dinah-scroll:' + firstKey + ':document'].top === 620);
    await page.evaluate(() => __documentsReady());
    await page.waitForFunction(key => { const el = document.querySelector('[data-win="' + key + '"] [data-doc-scroll]'); return el && el.scrollTop === 620 && el.scrollLeft === 85; }, firstKey);
    ok('document scroll waits for asynchronous content in both directions', true);
    ok('a page wider than its window is white to its far edge', await page.locator('[data-win="' + firstKey + '"] [data-doc-scroll]').evaluate(el => el.querySelector('.doc-page').getBoundingClientRect().width >= el.scrollWidth - 1));
    await t.shot('ui-state-restored');
    await page.evaluate(key => pudlWindows.close(key), firstKey);
    await page.evaluate(item => dinahWorkbench.open(item), first);
    await page.evaluate(() => __documentsReady());
    ok('remembered placement takes precedence over the changed default', await page.evaluate(key => pudlWindows.state().place[key].mode === 'floating', firstKey));
    const secondKey = await page.evaluate(() => dinahWorkbench.open(dinahWorkbench.resourceItem({ workspaceId: 'w1', sourceId: 'local', path: 'second.md' }, 'second.md')));
    ok('the maximized preference applies to a new window', await page.evaluate(key => pudlWindows.state().place[key].mode === 'maximized', secondKey));
    // Two windows snapped side by side, with the line between them moved
    // (PUDL 0.67.0's splitters), keep that line after a restart.
    await page.evaluate(({ a, b }) => { pudlWindows.snap(a, 'left'); pudlWindows.snap(b, 'right'); }, { a: firstKey, b: secondKey });
    await page.locator('.win-splitter[aria-orientation="vertical"]').waitFor();
    await page.evaluate(() => pudlWindows.grid('x', 3, 0.4));
    await t.deadline('the moved splitter line to be saved', page.evaluate(() => dinahUIState.flush()), 30000);
    ok('a moved splitter line is saved with the layout', /grid=[^&]*x3:0\.4/.test(decodeURIComponent(nativeState.values['dinah-workbench-layout'])), nativeState.values['dinah-workbench-layout']);
    await stop();
    ({ page } = await start({ delay: true }));
    await page.evaluate(() => __documentsReady());
    ok('a moved splitter line is restored, and the two windows meet on it', await page.evaluate(({ a, b }) => {
      const left = document.querySelector('[data-win="' + a + '"]').getBoundingClientRect(), right = document.querySelector('[data-win="' + b + '"]').getBoundingClientRect();
      const st = pudlWindows.state();
      return st.place[a].mode === 'left' && st.place[b].mode === 'right' && Math.abs(left.right - right.left) < 2 && left.width < right.width;
    }, { a: firstKey, b: secondKey }));
    await t.shot('ui-state-splitter');
    await stop();

    // The explicit address has priority over the native layout. The bus pane
    // offered itself in the first start and saved its flag at once
    // (offer-once.js), so it cannot come back over the address here.
    ({ page } = await start({ query: '?open=workspace&top=workspace' }));
    // Under heavy load the restore can land a moment after ui-state-ready, so
    // wait for it rather than reading the list once; the detail shows the list
    // as it stood if the wait runs out.
    await page.waitForFunction(() => pudlWindows.state().open.length === 1 && pudlWindows.state().open[0] === 'workspace', null, { timeout: 30000 }).catch(() => {});
    ok('an explicit incoming address overrides the saved open windows', await page.evaluate(() => pudlWindows.state().open.length === 1 && pudlWindows.state().open[0] === 'workspace'), await page.evaluate(() => JSON.stringify(pudlWindows.state().open)));
    await page.evaluate(() => dinahWorkbench.open('chat-browser'));
    await page.locator('#sessions [data-session="reading-chat"]').click();
    const chat = page.locator('.win[data-win-kind="chat-session"]');
    await chat.locator('[data-view-id="activity"]').getByText(/Paragraph 44/).waitFor();
    const chatKey = await chat.getAttribute('data-win');
    await chat.locator('[data-view-id="activity"]').focus(); await page.keyboard.press('Control+Home');
    await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]').dataset.following === 'false', chatKey);
    await chat.locator('[data-view-id="activity"]').evaluate(el => { el.scrollTop = 200; });
    await page.waitForFunction(key => { const value = dinahUIState.get('dinah-scroll:' + key + ':activity'); return value && value.top === 200; }, chatKey);
    await page.evaluate(() => dinahUIState.flush());
    await stop();
    ({ page } = await start({ delay: true }));
    await page.waitForFunction(() => typeof __chatsReady === 'function');
    await page.evaluate(() => __chatsReady());
    await page.waitForFunction(key => { const el = document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]'); return el && el.textContent.includes('Paragraph 44') && el.scrollTop === 200; }, chatKey);
    ok('delayed chat replay preserves the reading position instead of following the end', true);
    ok('restored scrollback keeps exact layout instead of estimated message heights', await page.locator('[data-win="' + chatKey + '"] .activity-row').first().evaluate(el => getComputedStyle(el).contentVisibility === 'visible'));
    await page.evaluate(() => __emit('agent:update', { sessionId: 'reading-chat', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: '\n\nA later message.' } } }));
    ok('later chat content leaves a reader above the end in place', await page.locator('[data-win="' + chatKey + '"] [data-view-id="activity"]').evaluate(el => el.scrollTop === 200));
    await page.evaluate(() => {
      const app = go.main.App; window.__realSave = app.SaveUIState; window.__saves = [];
      app.SaveUIState = state => new Promise((resolve, reject) => __saves.push({ state, resolve, reject }));
      dinahUIState.set('test-serialized', 1); window.__firstFlush = dinahUIState.flush();
    });
    await page.waitForFunction(() => __saves.length === 1);
    await page.evaluate(() => { dinahUIState.set('test-serialized', 2); window.__secondFlush = dinahUIState.flush(); __saves[0].resolve(); });
    await page.waitForFunction(() => __saves.length === 2);
    ok('native writes serialize and retain the newest state', await page.evaluate(() => __saves[0].state.values['test-serialized'] === 1 && __saves[1].state.values['test-serialized'] === 2));
    await t.deadline('the two serialized native writes to finish', page.evaluate(async () => { __saves[1].resolve(); await Promise.all([__firstFlush, __secondFlush]); }), 30000);
    await page.evaluate(() => { dinahUIState.set('test-serialized', 3); window.__failure = dinahUIState.flush().then(() => 'accepted', () => 'rejected'); });
    await page.waitForFunction(() => __saves.length === 3);
    await page.evaluate(() => __saves[2].reject(new Error('Disk unavailable')));
    ok('an explicit flush rejects failure so native close can remain canceled', await t.deadline('the rejected flush to settle', page.evaluate(() => __failure), 30000) === 'rejected');
    await t.deadline('the restored native save to flush', page.evaluate(async () => { go.main.App.SaveUIState = __realSave; await dinahUIState.flush(); }), 30000);
    await stop();

    ({ page } = await start({ delay: true }));
    await page.locator('[data-win="' + chatKey + '"] [data-view-id="activity"]').evaluate(el => el.dispatchEvent(new WheelEvent('wheel', { deltaY: 80 })));
    await page.evaluate(() => __chatsReady());
    await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] [data-view-id="activity"]').textContent.includes('Paragraph 44'), chatKey);
    ok('user scroll input cancels a pending replay restore', await page.locator('[data-win="' + chatKey + '"] [data-view-id="activity"]').evaluate(el => !dinahUIState.restorePending(el)));
    await stop();
    const savedBeforeError = JSON.stringify(nativeState), writesBeforeError = writes;
    ({ page } = await start({ readError: true }));
    const readFailureRejected = await t.deadline('the flush after a failed native read to settle', page.evaluate(async () => {
      dinahUIState.set('test-unsafe-reset', true);
      try { await dinahUIState.flush(); return false; } catch (error) { return String(error).includes('Unreadable native UI state'); }
    }), 30000);
    ok('explicit saves reject after native state could not be read', readFailureRejected);
    ok('a failed native read never replaces saved state', writes === writesBeforeError && JSON.stringify(nativeState) === savedBeforeError);
    await stop();
    nativeState = { version: 1, values: {} };
    ({ page } = await start({ legacy: {
      'dinah-workbench-layout': '?open=workspace&top=workspace',
      'dinah-section:example': 'closed',
      'dinah-scroll:example:document': '{"top":45,"left":12}',
      'unrelated-private-field': 'must not migrate'
    } }));
    await t.deadline('the migrated preferences to be saved', page.evaluate(() => dinahUIState.flush()), 30000);
    ok('migration includes named view preferences without capturing unrelated fields',
      nativeState.values['dinah-section:example'] === 'closed' &&
      nativeState.values['dinah-scroll:example:document'].left === 12 && !('unrelated-private-field' in nativeState.values));
    await stop();
    nativeState = { version: 1, values: {} };
    ({ page } = await start({ mode: 'maximized' }));
    await page.evaluate(() => dinahWorkbench.open('chat'));
    ok('an initial draft respects the selected new-window preference', await page.evaluate(() => pudlWindows.state().place.chat.mode === 'maximized'));
  } finally {
    // The last browser's close hangs as easily as any earlier one's.
    if (t) await t.deadline('the last browser to close', t.close(), 45000).catch(error => {
      console.log('FAIL the last browser to close :: ' + error);
      try { const proc = t.browser.process(); if (proc) proc.kill(); } catch (killError) {}
      process.exitCode = 1;
    });
  }
})();
