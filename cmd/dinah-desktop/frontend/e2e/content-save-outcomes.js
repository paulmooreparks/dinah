const { launch } = require('./lib/page');
(async () => {
  for (const keepNotice of [false, true]) {
    const t = await launch({ args: { keepNotice }, app: () => {
      // Capture the real window's public flush method without changing its source.
      let workbench;
      Object.defineProperty(window, 'dinahWorkbench', { configurable: true, get: () => workbench, set: value => {
        workbench = value;
        const register = value.register;
        value.register = function (kind, config) {
          if (kind === 'editor') {
            const init = config.init;
            config.init = function (...args) { const instance = init.apply(this, args); window.__windowInstance = instance; return instance; };
          }
          return register.apply(this, arguments);
        };
      }});
      const d = { id: 'a'.repeat(32), title: 'Release notes.md', text: '# Notes\n', revision: '1',
        origin: { kind: 'wiki-page', workspaceId: 'w', sourceId: 'site', path: 'notes.md' },
        writeBack: 'publish', changed: true, actions: ['update', 'save-as', 'discard'], suggestions: [], spans: {} };
      window.__refusal = 'The attachment file could not be found.';
      window.__updates = 0; window.__publishes = 0;
      window.__allowUpdates = false; window.__fault = ''; window.__fixture = d;
      return {
        GetDraft: () => { if (__fault === 'get-async') return Promise.reject(Error('The draft service is unavailable.')); if (__fault === 'get') throw Error('The draft service is unavailable.'); return Promise.resolve({ ...d }); },
        UpdateDraft: async (id, rev, text) => { window.__updates++; if (window.__holdUpdate) { __holdUpdate = false; await new Promise(resolve => { window.__releaseUpdate = resolve; }); throw Error(__refusal); } if (!__allowUpdates) throw Error(__refusal); d.text = text; d.revision = String(+d.revision + 1); return { ...d }; },
        SaveDraft: () => { window.__publishes++; if (__fault === 'complete') { d.changed = false; return Promise.resolve({ ...d }); } if (__fault === 'save-async') return Promise.reject(Error('The publish service is unavailable.')); if (__fault === 'save') throw Error('The publish service is unavailable.'); if (d.writeBack === 'save' && __allowUpdates) return Promise.resolve({ ...d }); return Promise.reject(Error(__refusal)); },
        SaveDraftAs: async (id, rev, ref) => { window.__copiedText = d.text; d.origin = { kind: 'local-folder', ...ref }; d.title = ref.path; d.writeBack = 'save'; return { ...d }; },
        CompareDraft: async () => ({ moved: false }),
        ShortcodeCatalog: async () => [], DraftFiles: async () => [], DocumentComments: async () => []
      };
    }});
    const p = t.page;
    try {
      await t.deadline('opening the document', p.evaluate(async () => {
        window.__notices = [];
        const toast = dinahHost.toast.bind(dinahHost);
        dinahHost.toast = (text, kind, ...rest) => { __notices.push({ text, kind }); return toast(text, kind, ...rest); };
        window.__key = await dinahContentWindows.open(await go.main.App.GetDraft(), 'editor');
        await dinahWorkbench.ready(__key);
      }));
      await p.locator('.content-applet .cm-content').click();
      await p.keyboard.press('Control+End');
      await p.keyboard.type(' Changed.');
      // The page's File Save stands in for Publish. The window flush is the
      // save of the typed draft that precedes that explicit Publish command.
      const save = await t.deadline('saving the typed draft', p.evaluate(async () => {
        try { await __windowInstance.flush(); return { ok: true }; }
        catch (e) { return { error: String(e.message || e) }; }
      }));
      await p.waitForFunction(() => __notices.some(n => n.text.includes(__refusal)));
      t.ok('Save shows the refusal, notice kept=' + keepNotice, await p.locator('.toast').filter({ hasText: 'The attachment file could not be found.' }).count() > 0);
      if (!keepNotice) await p.locator('.toast').filter({ hasText: 'The attachment file could not be found.' }).evaluateAll(nodes => nodes.forEach(n => n.remove()));
      const before = await p.evaluate(() => __notices.length);
      const publish = await t.deadline('publishing after the refusal', p.evaluate(async () => {
        dinahHost.ask = async () => true;
        try { await dinahDocuments.get(__key).publish(); return { ok: true }; }
        catch (e) { return { error: String(e.message || e) }; }
      }));
      t.ok('Publish shows a distinct refusal after Save, notice kept=' + keepNotice, await p.evaluate(n => __notices.slice(n).some(x => x.text.startsWith('The page was not published: ') && x.text.includes(__refusal)), before));
      t.ok('Publish refusal is visible, notice kept=' + keepNotice, await p.locator('.toast').filter({ hasText: 'The page was not published: ' }).count() > 0);
      t.ok('no success notice is shown, notice kept=' + keepNotice, await p.evaluate(() => !__notices.some(x => x.kind === 'success')));
      t.ok('the title remains Not published, notice kept=' + keepNotice, await p.locator('[data-win]').filter({ has: p.locator('.content-applet') }).evaluate(el => el.textContent.includes('Not published')));
      t.ok('Publish remains enabled, notice kept=' + keepNotice, await p.evaluate(() => dinahDocuments.get(__key).canPublish()));
      await p.locator('nav[data-menubar] .menubar-title').filter({ hasText: /^File$/ }).first().click();
      const publishRow = p.locator('.menubar-panel .menu-action').filter({ hasText: /^Publish/ });
      await publishRow.waitFor({ state: 'visible' });
      t.ok('the rendered Publish command remains enabled, notice kept=' + keepNotice,
        !await publishRow.isDisabled() && await publishRow.getAttribute('aria-disabled') !== 'true');
      await p.keyboard.press('Escape');
      t.ok('a refused flush rejects to its caller, notice kept=' + keepNotice, !!save.error);
      t.ok('a refused Publish rejects to its caller, notice kept=' + keepNotice, !!publish.error);
      t.ok('a refused flush prevents publishing stale draft text, notice kept=' + keepNotice, await p.evaluate(() => __publishes === 0));
      const counts = await p.evaluate(() => ({ notices: __notices.length, updates: __updates }));
      for (let i = 0; i < 2; i++) {
        await p.evaluate(() => document.querySelector('[data-content-key]').dispatchEvent(new CustomEvent('dinah:content-changed')));
        await p.waitForTimeout(1000);
      }
      t.ok('unchanged refused text is not automatically retried, notice kept=' + keepNotice, await p.evaluate(c => __updates === c.updates && __notices.length === c.notices, counts));

      const errorsBeforeTyping = await p.evaluate(() => __notices.filter(x => x.text === __refusal).length);
      for (let i = 0; i < 2; i++) {
        const before = await p.evaluate(() => __updates);
        await p.locator('.content-applet .cm-content').click();
        await p.keyboard.type(' Pause ' + i + '.');
        await p.waitForFunction(n => __updates > n, before);
      }
      t.ok('typing pauses do not repeat the host refusal notice, notice kept=' + keepNotice,
        await p.evaluate(n => __notices.filter(x => x.text === __refusal).length === n, errorsBeforeTyping));
      for (const action of ['flush', 'flush']) {
        const before = await p.evaluate(() => __notices.length);
        const error = await t.deadline('repeating ' + action, p.evaluate(async action => {
          try { await (action === 'flush' ? __windowInstance.flush() : dinahDocuments.get(__key).saveAs()); return ''; }
          catch (e) { return String(e.message || e); }
        }, action));
        t.ok('explicit ' + action + ' repeats its refusal, notice kept=' + keepNotice,
          error.includes('attachment file') && await p.evaluate(n => __notices.slice(n).some(x => x.kind === 'danger' && x.text.includes(__refusal)), before));
      }
      await p.evaluate(() => {
        document.querySelectorAll('.toast').forEach(n => n.remove());
        window.__picks = 0;
        dinahFilePicker.pick = async options => { __picks++; await options.commit({ name: 'Copy.md', dir: { ref: { workspaceId: 'w', sourceId: 'files', path: '.' } } }); };
      });
      const copy = await t.deadline('Save As after the refusal', p.evaluate(() => dinahDocuments.get(__key).saveAs()));
      t.ok('Save As reaches its destination and reports a refused write, notice kept=' + keepNotice,
        copy === false && await p.evaluate(() => __picks === 1) && await p.locator('.toast').filter({ hasText: 'The attachment file could not be found.' }).count() > 0);
      await p.evaluate(() => { __fixture.writeBack = 'save'; });
      try { await p.evaluate(() => __windowInstance.flush()); } catch (_) { /* The next command must try the same refused text again. */ }
      for (let i = 0; i < 2; i++) {
        const before = await p.evaluate(() => ({ notices: __notices.length, updates: __updates }));
        const error = await p.evaluate(async () => {
          try { await dinahDocuments.get(__key).save(); return ''; }
          catch (e) { return String(e.message || e); }
        });
        t.ok('explicit Save retries and shows the refusal again ' + i + ', notice kept=' + keepNotice,
          error.includes('attachment file') && await p.evaluate(c => __updates === c.updates + 1 && __notices.slice(c.notices).some(x => x.kind === 'danger'), before));
      }
      await p.evaluate(() => { __allowUpdates = true; window.__holdUpdate = true; __picks = 0; });
      await p.locator('.content-applet .cm-content').click();
      await p.keyboard.type(' Current copy.');
      await p.evaluate(() => { window.__pendingSave = __windowInstance.flush().catch(e => String(e.message || e)); });
      await p.waitForFunction(() => typeof window.__releaseUpdate === 'function');
      await p.evaluate(() => { window.__pendingCopy = dinahDocuments.get(__key).saveAs(); });
      await p.waitForTimeout(80);
      t.ok('Save As waits for the in-flight save, notice kept=' + keepNotice, await p.evaluate(() => __picks === 0));
      await p.evaluate(() => __releaseUpdate());
      const copied = await t.deadline('copying after the failed in-flight save', p.evaluate(() => __pendingCopy));
      t.ok('Save As recovers the latest text after the old write fails, notice kept=' + keepNotice,
        copied === true && await p.evaluate(() => __picks === 1 && __copiedText.includes('Current copy.')));
      await t.deadline('retrying the repaired service', p.evaluate(() => dinahDocuments.get(__key).save()));
      t.ok('explicit Save succeeds after repair without editing, notice kept=' + keepNotice, await p.evaluate(() => __notices.some(x => x.text.startsWith('Saved ') && x.kind === 'info')));
      const beforeUnchanged = await p.evaluate(() => __notices.length);
      await p.evaluate(() => dinahDocuments.get(__key).save());
      t.ok('unchanged explicit Save still confirms its outcome, notice kept=' + keepNotice, await p.evaluate(n => __notices.slice(n).some(x => x.text.startsWith('Saved ')), beforeUnchanged));
      await p.evaluate(() => { __fixture.writeBack = 'publish'; });
      await p.locator('.content-applet .cm-content').click();
      await p.keyboard.type(' Another change.');
      await t.deadline('saving the corrected draft', p.evaluate(() => __windowInstance.flush()));
      for (const fault of ['get', 'get-async', 'ask', 'ask-async', 'save', 'save-async', 'refusal']) {
        const before = await p.evaluate(() => __notices.length);
        const error = await t.deadline('publishing with ' + fault + ' unavailable', p.evaluate(async fault => {
          __fault = fault;
          dinahHost.ask = fault === 'ask' ? () => { throw Error('The confirmation service is unavailable.'); } : fault === 'ask-async' ? () => Promise.reject(Error('The confirmation service is unavailable.')) : async () => true;
          try { await dinahDocuments.get(__key).publish(); return ''; }
          catch (e) { return String(e.message || e); }
          finally { __fault = ''; }
        }, fault));
        t.ok(fault + ' failure is visible and rejects, notice kept=' + keepNotice,
          error.includes(fault === 'refusal' ? 'attachment file' : 'unavailable') && await p.evaluate(n => __notices.slice(n).some(x => x.kind === 'danger' && x.text.includes('not published:')), before));
      }
      await p.evaluate(async () => { __fault = 'complete'; dinahHost.ask = async () => true; await dinahDocuments.get(__key).publish(); __fault = ''; });
      t.ok('successful Publish clears its pending state, notice kept=' + keepNotice, await p.evaluate(() => !dinahDocuments.get(__key).canPublish() && __notices.some(x => x.kind === 'success')));
      const beforeNoChanges = await p.evaluate(() => __notices.length);
      await p.evaluate(() => dinahDocuments.get(__key).publish());
      t.ok('unchanged Publish reports no changes, notice kept=' + keepNotice, await p.evaluate(n => __notices.slice(n).some(x => x.kind === 'info' && x.text.includes('no changes to publish')), beforeNoChanges));
      await t.shot('content-save-publish-refusal-' + (keepNotice ? 'kept' : 'dismissed'));
    } finally { await t.close(); }
  }
})().catch(err => { console.error(err); process.exitCode = 1; });
