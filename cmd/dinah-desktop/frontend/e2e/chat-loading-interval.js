// Opening and saved display proceed independently, with a fresh journal
// checkpoint before live catch-up. The provider and each metadata read are held.
const { launch, OUT } = require('./lib/page');
const fs = require('fs'), path = require('path');
(async () => {
 const evidence = [];
 for (const mode of ['matching', 'zero', 'missing', 'mismatch', 'changed', 'unavailable', 'cached-title', 'cross-workspace', 'open-failure', 'history-failure', 'render-generation', 'fallback-generation']) {
  const t = await launch({ viewport: { width: 1440, height: 950 }, args: { mode },
   state: { titles: mode === 'cached-title' ? {} : { 'codex:loading': 'Known Loading Member' } },
   app: ({ mode }) => {
    const id = 'codex:loading', ws = mode === 'cross-workspace' ? 'C:/Elsewhere' : 'C:/Fixture';
    const lead = { chat: 'lead', title: 'Known Lead', group: 'loading-team' };
    const parent = Array.from({ length: 1000 }, (_, n) => ({ role: n % 2 ? 'you' : 'agent', text: 'Parent entry ' + n, provider: 'claude' }));
    const saved = parent.concat([{ role: 'agent', text: 'PRE-OPEN-SAVED-CONTENT' }]);
    const info = (gen = window.__activeGen || 'new') => ({ sessionId: id, provider: 'codex', cwd: ws, title: 'Known Loading Member', open: true, working: false, streamGen: gen, streamHead: 4, stateGen: 'loading', stateVer: ++window.__version });
    const entry = (n, text, role = 'agent_message_chunk') => ({ n, name: 'agent:update', data: { update: { sessionUpdate: role, content: { type: 'text', text } } } });
    window.__activeGen = 'new'; window.__freshReads = 0; window.__afterScheduled = new Set(); window.__providerReturned = false; window.__version = 0; window.__trace = []; window.__historyRequests = []; window.__cancels = [];
    window.__mark = phase => __trace.push({ phase, at: performance.now() });
    window.__journal = [entry(1, 'CURRENT-QUESTION', 'user_message_chunk'), entry(2, 'CURRENT-ANSWER'), entry(3, ' [BETWEEN-READS]')];
    window.__savedGate = new Promise(resolve => { window.__releaseSaved = resolve; });
    window.__providerGate = new Promise(resolve => { window.__releaseProvider = resolve; });
    window.__membershipGate = new Promise(resolve => { window.__releaseMembership = resolve; });
    window.__emitJournal = item => __emit(item.name, { sessionId: id, gen: __activeGen, n: item.n, ...item.data });
    window.__working = () => __emit('agent:session', { ...info(), working: true });
    window.__switchGeneration = phase => {
     __mark(phase); window.__activeGen = 'next';
     window.__journal = [entry(1, 'CURRENT-QUESTION', 'user_message_chunk'), entry(2, 'CURRENT-ANSWER [NEW-GENERATION]'), entry(3, ' [BETWEEN-READS]')];
     // Deliver this state at the controlled boundary, without relying on a timer.
     dinahHelpers.events.dispatch('agent:session', info());
    };
    return {
     AgentProviders: async () => [__provider('codex'), __provider('claude')],
     Sessions: async () => [], SessionsAt: async () => [],
     ResolveSessionContinuation: async value => value,
     SavedConversation: async () => {
      __mark(__providerReturned ? 'fresh-snapshot' : 'display-snapshot-request');
      if (!window.__providerReturned) { await __savedGate; return mode === 'unavailable' ? { available: false, messages: [] } : { available: true, complete: false, warning: 'Saved history is incomplete.', messages: saved }; }
      if (mode === 'history-failure' && !window.__retry) throw new Error('Fixture current-history read failed');
      if (mode === 'unavailable') return { available: false, messages: [] };
      if (mode === 'changed') __emit('agent:session', info('new'));
      window.__freshReads++;
      if (mode === 'render-generation' && __freshReads === 1) {
       const enqueue = dinahRender.enqueue;
       let wrapped = false;
       dinahRender.enqueue = function(key, root, step, after, label) {
        if (label === 'saved history' && !wrapped) {
         wrapped = true;
         return enqueue(key, root, deadline => {
          const more = step(deadline);
          if (more && __activeGen === 'new') __switchGeneration('generation-change-during-saved-render');
          return more;
         }, after, label);
        }
        return enqueue(key, root, step, after, label);
       };
      }
      const snapshotGen = __activeGen;
      if (!__afterScheduled.has(snapshotGen)) {
       __afterScheduled.add(snapshotGen);
       requestAnimationFrame(() => { if (__activeGen !== snapshotGen) return; const item = entry(__journal.length + 1, ' [AFTER-SNAPSHOT]'); __journal.push(item); __emitJournal(item); });
      }
      return { available: true, complete: false, warning: 'Saved history is incomplete.',
       ...(mode === 'missing' ? {} : { streamGen: mode === 'mismatch' || mode === 'fallback-generation' && __freshReads === 1 ? 'old' : __activeGen }),
       ...(mode === 'zero' ? {} : { streamThrough: 2 }),
       messages: mode === 'zero' ? parent : parent.concat([{ role: 'you', text: 'CURRENT-QUESTION' }, { role: 'agent', text: 'CURRENT-ANSWER' + (mode === 'render-generation' && __freshReads === 1 ? ' [OLD-GENERATION-COPY]' : __activeGen === 'next' ? ' [NEW-GENERATION]' : '') }]) };
     },
     SessionContinuation: async () => { __mark('parent-query'); return { parentSessionId: 'claude:parent', history: parent }; },
     OpenSessionAt: async () => { __mark('provider-open-request'); await __providerGate; window.__providerReturned = true; __mark('provider-open-return'); if (mode === 'open-failure' && !window.__retry) throw new Error('Fixture provider opening failed'); return info(mode === 'changed' ? 'old' : 'new'); },
     ChatEvents: async (value, gen, after) => {
      __mark('journal-' + gen + '-after-' + after);
      if (mode === 'fallback-generation' && gen === 'new' && after === 0 && __activeGen === 'new') {
       const obsolete = [entry(1, 'CURRENT-QUESTION', 'user_message_chunk'), entry(2, 'CURRENT-ANSWER [OLD-GENERATION-JOURNAL]'), entry(3, ' [BETWEEN-READS]'), entry(4, ' [AFTER-SNAPSHOT]')];
       __switchGeneration('generation-change-during-zero-catch-up');
       return {gen, head:4, events:obsolete};
      }
      return { gen, head: __journal.length, events: __journal.filter(item => item.n > after) };
     },
     ChatLead: async () => { await __membershipGate; if (window.__membershipFailure) throw new Error('Fixture membership failure'); return lead; },
     Team: async () => ({ lead: { chat: id }, members: [], counts: { top: 'idle', idle: 1 } }),
     Cancel: async value => { __cancels.push(value); __emit('agent:session', info()); },
     ShowSavedChatHistory: async value => { __historyRequests.push(value); if (mode === 'missing') throw new Error('Fixture history export failed'); },
    };
   }
  });
  const p = t.page, ok = t.ok, workspace = mode === 'cross-workspace' ? 'C:/Elsewhere' : 'C:/Fixture';
  try {
   await p.locator('#status-starting').waitFor({ state: 'hidden' });
   await p.evaluate(workspace => { __mark('background-connect'); __emit('chat:connect', { sessionId: 'codex:loading', cwd: workspace }); }, workspace);
   await p.waitForFunction(() => __trace.some(item => item.phase === 'provider-open-request'));
   ok(mode + ': provider opening starts while the saved response is unresolved', await p.evaluate(() => !__trace.some(item => item.phase === 'provider-open-return')));
   const key = await p.evaluate(({mode,workspace}) => {
    const item = { kind: 'chat-session', workspace, path: 'codex:loading' };
    if (mode === 'cached-title') dinahUIState.set('dinah-window-titles', { [dinahWorkbench.key(item)]: { title: 'Stored Known Title' } });
    return dinahWorkbench.open(item);
   }, {mode,workspace});
   await p.evaluate(key => {
    window.__intervalFrames = []; window.__sampleInterval = true;
    function sample() {
     if (!__sampleInterval) return;
     const win = document.querySelector('[data-win="' + key + '"]');
     if (win) {
      const body = win.querySelector('.dinah-applet'), root = win.querySelector('.chat-instance');
      const empty = win.querySelector('[data-view-id=log-empty]'), prompt = win.querySelector('[data-view-id=prompt]');
      const visible = el => !!el && !el.hidden && el.getClientRects().length > 0 && getComputedStyle(el).visibility !== 'hidden';
      const logs = [...win.querySelectorAll('[data-view-id=logs] > *')].filter(visible);
      __intervalFrames.push({ at: performance.now(), title: win.querySelector('.win-title').textContent, content: body.textContent.trim().length > 0,
       empty: visible(empty), ordinaryComposer: visible(prompt), member: !!root?.hasAttribute('data-team-member'), savedContent: logs.some(el=>el.children.length>0), staleCopy: logs.some(el=>/OLD-GENERATION/.test(el.textContent)) });
     }
     requestAnimationFrame(sample);
    }
    requestAnimationFrame(sample);
   }, key);
   let win = p.locator('[data-win="' + key + '"]');
   await win.getByText('Checking team membership…', { exact: true }).waitFor();
   ok(mode + ': pending membership withholds the ordinary composer', !await win.locator('[data-view-id=prompt]').isVisible());
   ok(mode + ': loading never presents an empty conversation', !await win.locator('[data-view-id=log-empty]').isVisible());
   ok(mode + ': the known title survives the unresolved interval', await win.locator('.win-title').textContent() === (mode === 'cached-title' ? 'Stored Known Title' : 'Known Loading Member'));
   if (mode === 'cached-title') {
    await p.evaluate(() => __emit('agent:session', {sessionId:'codex:loading',provider:'codex',cwd:'C:/Fixture',open:false,stateGen:'loading',stateVer:++__version}));
    ok('a sparse loading snapshot does not replace the cached title', await win.locator('.win-title').textContent() === 'Stored Known Title');
    await p.evaluate(() => __emit('agent:session', {sessionId:'codex:loading',provider:'codex',cwd:'C:/Fixture',title:'',open:false,stateGen:'loading',stateVer:++__version}));
    ok('an empty loading title does not replace the cached title', await win.locator('.win-title').textContent() === 'Stored Known Title');
   }
   await p.evaluate(() => { __releaseMembership(); __releaseSaved(); });
   const visible = mode === 'unavailable' ? 'Parent entry 999' : 'PRE-OPEN-SAVED-CONTENT';
   await win.getByText(visible, { exact: true }).waitFor();
   await win.getByRole('link', { name: 'Open Team Chat', exact: true }).waitFor();
   if (mode !== 'unavailable') ok(mode + ': incomplete-history warning is visible during opening', await win.locator('[data-view-id=history-warning]').isVisible());
   ok(mode + ': local saved rendering is not reported as provider parts', !/parts so far/.test(await win.innerText()));
   await p.evaluate(key => pudlWindows.close(key), key);
   await p.evaluate(workspace => dinahWorkbench.open({ kind: 'chat-session', workspace, path: 'codex:loading' }), workspace);
   win = p.locator('[data-win="' + key + '"]');
   await win.getByText(visible, { exact: true }).waitFor();
   ok(mode + ': reopening retains one saved copy and one provider request', await win.getByText(visible, { exact: true }).count() === 1 && await p.evaluate(() => __trace.filter(item => item.phase === 'provider-open-request').length === 1));
   await p.evaluate(() => { __journal.forEach(__emitJournal); __releaseProvider(); });
   if (mode === 'open-failure' || mode === 'history-failure') {
    await win.locator('[data-view-id=session-note]').waitFor();
    ok(mode + ': a failed opening or history read retains the available saved copy', await win.getByText('PRE-OPEN-SAVED-CONTENT', {exact:true}).isVisible() && !await win.locator('[data-view-id=session-save-history]').isDisabled());
    if (mode === 'history-failure') {
     ok('history failure is not labelled as provider startup failure', /opened, but its current history/.test(await win.locator('[data-view-id=session-note-title]').textContent()));
     await p.evaluate(() => { const item={n:4,name:'agent:update',data:{update:{sessionUpdate:'agent_message_chunk',content:{type:'text',text:' [DURING-FAILURE]'}}}}; __journal.push(item); __emitJournal(item); });
     await p.evaluate(() => { for (const item of [{n:5,name:'agent:sent',data:{kind:'asked',text:'SENT-DURING-FAILURE'}},{n:6,name:'agent:turn',data:{stopReason:'end_turn'}}]) { __journal.push(item); __emitJournal(item); } });
     ok('unverified history does not merge journal events into the saved display', !/DURING-FAILURE|finished its turn/.test(await win.locator('[data-view-id=logs]').innerText()));
    }
    await p.evaluate(() => { window.__retry=true; });
    await win.getByRole('button',{name:'Try Again',exact:true}).click();
   }
   await win.getByText(/CURRENT-ANSWER.*BETWEEN-READS/).first().waitFor();
   await p.waitForFunction(() => !document.querySelector('.win.active .chat-loading-chip') || document.querySelector('.win.active .chat-loading-chip').hidden);
   if (!['unavailable'].includes(mode)) await win.getByText(/AFTER-SNAPSHOT/).first().waitFor();
   const transcript = await win.locator('[data-view-id=logs]').innerText();
   if (mode === 'render-generation' || mode === 'fallback-generation') {
    ok(mode + ': a generation change rereads the current snapshot', await p.evaluate(() => __freshReads >= 2));
    ok(mode + ': the obsolete copy and checkpoint are replaced', !/OLD-GENERATION/.test(transcript) && transcript.split('[NEW-GENERATION]').length === 2 && await p.evaluate(() => __trace.some(item => item.phase === 'journal-next-after-2')), transcript.slice(-400));
   }
   if(mode === 'history-failure') ok('retry recovers journal events received during the failed history read', transcript.split('[DURING-FAILURE]').length === 2 && transcript.split('SENT-DURING-FAILURE').length === 2 && transcript.split('finished its turn').length === 2);
   ok(mode + ': snapshot and in-flight journal entries appear once', transcript.split('CURRENT-ANSWER').length === 2 && transcript.split('[BETWEEN-READS]').length === 2 && (mode === 'unavailable' || transcript.split('[AFTER-SNAPSHOT]').length === 2), transcript.slice(-400));
   const button = win.getByRole('button', { name: 'Show Saved Chat History', exact: true });
   await button.scrollIntoViewIfNeeded(); await button.click();
   await p.waitForFunction(() => __historyRequests.length === 1);
   ok(mode + ': omitted history requests the complete saved snapshot', await p.evaluate(() => __historyRequests[0] === 'codex:loading'));
   if (mode === 'missing') ok('export failure is visible', await p.locator('.toast-text').filter({hasText:'Fixture history export failed'}).isVisible());
   await p.evaluate(() => { __membershipFailure = true; __resync(); });
   await win.getByText(/Team membership could not be checked/).waitFor();
   ok(mode + ': failed metadata refresh retains known membership', await win.getByRole('link', { name: 'Open Team Chat', exact: true }).isVisible() && !await win.locator('[data-view-id=prompt]').isVisible());
   await p.evaluate(() => __working());
   const stop = win.getByRole('button', { name: 'Stop', exact: true }); await stop.waitFor(); await stop.click();
   await p.waitForFunction(() => __cancels.length === 1);
   ok(mode + ': Stop retains the provider-turn cancellation contract', await p.evaluate(() => __cancels[0] === 'codex:loading'));
   const frames = await p.evaluate(() => { __sampleInterval = false; return __intervalFrames; });
   let memberSeen = false;
   const invalid = frames.filter(frame => { memberSeen ||= frame.member; return frame.staleCopy || !frame.content || frame.empty || frame.ordinaryComposer || memberSeen && !frame.member || !['Stored Known Title','Known Loading Member'].includes(frame.title); });
   ok(mode + ': every sampled loading frame preserves content state, title and membership', frames.length > 0 && invalid.length === 0, JSON.stringify(invalid.slice(0,3)));
   evidence.push({ mode, frames, trace: await p.evaluate(() => __trace), finalText: transcript.slice(-400) });
   await t.shot('loading-' + mode);
  } finally {
   const observed = await p.evaluate(() => ({ trace: window.__trace, body: document.querySelector('.win.active')?.innerText.slice(-1600), logs: [...document.querySelectorAll('.win.active [data-view-id=logs] > *')].map(el=>({hidden:el.hidden,rows:el.children.length,text:el.textContent.slice(-100)})) }));
   fs.writeFileSync(path.join(OUT, 'loading-observed-' + mode + '.json'), JSON.stringify(observed, null, 2));
   await t.close();
  }
 }
 fs.writeFileSync(path.join(OUT, 'chat-loading-interval.json'), JSON.stringify(evidence, null, 2));
})().catch(error => { console.error(error); process.exitCode = 1; });
