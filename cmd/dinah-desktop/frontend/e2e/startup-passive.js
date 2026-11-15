const { launch } = require('./lib/page');
const fs = require('fs'), path = require('path');
const { OUT } = require('./lib/page');
(async () => {
  const t = await launch({ app: () => {
    window.__effects = [];
    window.__judge = { enabled: false, provider: 'ollama', retentionDays: 90, warnStreak: 3 };
    const rows = [{ sessionId: 'saved', chatId: 'logical', title: 'Saved Chat', cwd: 'C:/Fixture', provider: 'devin', open: false },
      { sessionId: 'repair', chatId: 'repair', title: 'Needs Repair', cwd: 'C:/Fixture', resolution: 'repair', reason: 'Two owners claim this chat.' }];
    window.__savedReads = []; window.__holdSaved = false;
    const record = (name, result) => { __effects.push(name); return Promise.resolve(result); };
    return {
      SavedChats: async () => __holdSaved ? new Promise(resolve => __savedReads.push(resolve)) : ({ generation: 'one', chats: rows }),
      DeclaredProviders: async () => ['codex','devin','claude','ollama'].map(id => ({ id, name: id, status: 'unknown', facts: __provider(id).facts })),
      Sessions: () => record('native-history', []), SessionsAt: () => record('native-history', []),
      RefreshProviderSessions: (workspace, id) => record('native-history:' + id, { generation: 'intermediate', chats: [{ sessionId: 'not-authoritative', title: 'Ignore intermediate response' }] }),
      AgentProviders: () => record('scan', ['codex','devin','claude','ollama'].map(id => __provider(id))),
      SkillCatalog: async () => ({ skills: [] }), ChatSkillCatalog: () => record('skills', { skills: [] }),
      ModelPrices: id => record('prices:' + id, {}),
      OllamaMemory: () => record('memory', { models: [] }),
      JudgementState: async () => ({ state: 'off' }), JudgementSettings: async () => __judge,
      JudgementSummary: async () => ({}), JudgementRecords: async () => ({ records: [] }),
      JudgementProviders: () => record('judge-providers', [{ id: 'ollama', name: 'Ollama', available: true }]),
      JudgementModels: () => record('judge-models', []), JudgementModelPrices: () => record('judge-prices', {}),
      JudgementKinds: async () => [], JudgementCalibration: async () => ({}),
      SetJudgementSettings: async value => (__judge = value)
    };
  } });
  const p = t.page, ok = t.ok;
  const settle = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
  try {
    await p.waitForFunction(() => !document.body.hasAttribute('aria-busy') && document.querySelector('#workspace-chats [data-session-row="repair"]'));
    await settle();
    ok('startup reads saved chats and declarations without contacting a provider', await p.evaluate(() => __effects.length === 0));
    ok('startup does not read a hidden calibration report', !(await t.calls()).some(call => call[0] === 'JudgementCalibration'));
    ok('fresh startup opens no blank New Chat window', await p.evaluate(() => !pudlWindows.state().open.some(key => dinahWorkbench.item(key)?.kind === 'chat')));
    ok('a repair row explains the problem and offers no open link', await p.locator('#workspace-chats [data-session-row="repair"]').evaluate(el => el.textContent.includes('Two owners') && !el.querySelector('a[href],a[data-session]')));
    ok('repair rows are absent from chat recipient choices', await p.evaluate(() => !dinahChats.known().some(row => row.id === 'repair')));
    const projected = await p.evaluate(() => dinahChatInventory.chart({ chats: [{ chatId: 'logical', head: 'saved' }], teams: [{ id: 'team', leadId: 'logical', memberIds: ['saved', 'missing'] }], groups: [] }, dinahChats.inventory()));
    ok('the chart keeps canonical identity and uses the current head for opening', projected.chats.filter(row => row.chatId === 'logical').length === 1 && projected.chats.find(row => row.chatId === 'logical').head === 'saved' && projected.teams[0].memberIds[0] === 'logical');
    ok('inventory repair rows and relationship-only references remain explained and cannot open', ['repair','missing'].every(id => projected.chats.some(row => row.chatId === id && row.resolution === 'repair' && row.reason && !row.head)));
    await p.evaluate(() => {
      __holdSaved = true;
      dinahChats.refreshInventory(); dinahChats.refreshInventory();
    });
    await p.waitForFunction(() => __savedReads.length === 1);
    ok('concurrent unchanged refreshes share one request', await p.evaluate(() => __savedReads.length === 1));
    await t.emit('chat:removed', { sessionId: 'removed-during-refresh' });
    await p.evaluate(() => { dinahChats.refreshInventory(); });
    await p.waitForFunction(() => __savedReads.length === 2);
    await p.evaluate(() => __savedReads[1]({ generation: 'a', chats: [{ sessionId: 'saved', cwd: 'C:/Fixture', provider: 'devin', title: 'Newer Snapshot' }] }));
    await p.locator('#workspace-chats').getByText('Newer Snapshot', { exact: true }).waitFor();
    await p.evaluate(() => { __savedReads[0]({ generation: 'z', chats: [{ sessionId: 'saved', cwd: 'C:/Fixture', provider: 'devin', title: 'Older Snapshot' }] }); __holdSaved = false; });
    await settle();
    ok('a late old response cannot replace a newer snapshot even with a lexically later generation', (await p.locator('#workspace-chats').textContent()).includes('Newer Snapshot') && !(await p.locator('#workspace-chats').textContent()).includes('Older Snapshot'));
    await t.emit('providers:facts', [{ id: 'devin', facts: { prices: true } }]);
    await t.emit('agent:state', { provider: 'devin', state: 'ready' });
    await settle();
    ok('unused ready providers and facts updates fetch no prices', await p.evaluate(() => __effects.length === 0));
    await p.evaluate(() => dinahWorkbench.open('settings'));
    await settle();
    ok('showing Appearance does not scan agents or read memory', await p.evaluate(() => __effects.length === 0));
    await p.locator('[data-settings-section="judgements"]').click();
    await settle();
    ok('disabled judging requests no provider checks, model inventory or prices when shown', await p.evaluate(() => !__effects.includes('judge-providers') && !__effects.includes('judge-models') && !__effects.includes('judge-prices')));
    await p.locator('#judgements-enabled').click();
    await p.waitForFunction(() => __effects.includes('judge-models') && __effects.includes('judge-prices'));
    ok('enabling judging in the shown section loads its inventory', true);
    await p.locator('[data-settings-section="agents"]').click();
    await p.waitForFunction(() => __effects.includes('scan') && __effects.includes('memory'));
    ok('showing Agents scans and reads the mounted Ollama memory section', true);
    await p.locator('[data-settings-section="appearance"]').click();
    const before = await p.evaluate(() => __effects.length);
    await p.evaluate(() => { __state.ollamaContextCap = 65536; __emit('settings:changed', {}); });
    await settle();
    ok('a hidden section stays passive after a settings update', await p.evaluate(n => __effects.length === n, before));
    await p.evaluate(() => dinahWorkbench.open('chat'));
    const prompt = p.locator('.chat-instance textarea[data-view-id="prompt"]');
    await prompt.focus();
    await p.waitForFunction(() => __effects.includes('skills'));
    ok('focusing the blank composer loads its skills', true);
    await t.emit('agent:session', { sessionId: 'saved', cwd: 'C:/Fixture', provider: 'devin', open: true, title: 'Saved Chat' });
    await p.waitForFunction(() => __effects.includes('prices:devin'));
    ok('an opened chat requests its provider prices', true);
    ok('the page never asks for native history implicitly', await p.evaluate(() => !__effects.includes('native-history')));
    const frames = [];
    for (const width of [1440,390]) {
      await p.setViewportSize({ width, height: 1000 }); await settle();
      const sample = await p.evaluate(async () => {
        const measure = () => Array.from(document.querySelectorAll('.chat-instance .composer-bar,.chat-instance textarea[data-view-id="prompt"]')).flatMap(el => { const r = el.getBoundingClientRect(); return [r.x,r.y,r.width,r.height]; });
        const before = measure(), samples = [];
        __emit('providers:facts', [{ id: 'codex', facts: {} }]);
        for (let frame=0;frame<12;frame++) { await new Promise(requestAnimationFrame); samples.push(measure()); }
        return { before, samples };
      });
      frames.push({ width, ...sample });
      ok('passive provider updates keep the composer fixed at ' + width, sample.samples.every(value => JSON.stringify(value) === JSON.stringify(sample.before)));
      await t.shot('startup-passive-' + width);
    }
    fs.writeFileSync(path.join(OUT,'startup-passive-frames.json'),JSON.stringify(frames,null,2));
    await p.evaluate(() => { dinahCommands.run('refresh-sessions'); });
    await p.waitForFunction(() => ['codex','devin','claude','ollama'].every(id => __effects.includes('native-history:' + id)));
    await p.locator('#workspace-chats').getByText('Saved Chat', { exact: true }).waitFor({ state: 'attached' });
    ok('explicit Refresh Chats asks each declared provider and uses the final saved inventory', await p.evaluate(() => !dinahChats.inventory().some(row => row.sessionId === 'not-authoritative') && __bridgeCalls.filter(call => call[0] === 'RefreshProviderSessions').every(call => call[1] === 'C:/Fixture')));
  } catch (error) {
    fs.writeFileSync(path.join(OUT,'startup-passive-failure.json'), JSON.stringify(await p.evaluate(() => ({ busy: document.body.hasAttribute('aria-busy'), calls: __bridgeCalls, rows: document.querySelector('#workspace-chats').innerHTML, toasts: Array.from(document.querySelectorAll('.toast-text')).map(el => el.textContent) })), null, 2));
    await t.shot('startup-passive-failure');
    throw error;
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
