// Sessions, workspaces, the model drop-up and Settings against a stub of the
// Go side.
const { launch } = require('./lib/page');
const { choose, shows, rowsOf } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => {
      const emit = window.__emit;
      const models = [['adaptive', 'Adaptive'], ['swe-1-6', 'SWE-1.6'], ['claude-opus-5-5-medium', 'Claude Opus 5.5 Medium'], ['claude-opus-5-5-low', 'Claude Opus 5.5 Low'], ['gpt-6-sol-high', 'GPT-6 Sol High']];
      const mkOpts = (model) => [
        { id: 'mode', name: 'Session Mode', category: 'mode', type: 'select', currentValue: 'accept-edits', options: [{ value: 'accept-edits', name: 'Code' }, { value: 'ask', name: 'Ask' }, { value: 'plan', name: 'Plan' }] },
        { id: 'model', name: 'Model', category: 'model', type: 'select', currentValue: model, options: models.map(m => ({ value: m[0], name: m[1] })) }];
      let pinned = ['claude-opus-5-5-medium', 'gpt-6-sol-high'];
      let defaults = {}; let hidden = [];
      let ws = 'C:\\Work\\Parser Workspace';
      const recent = ['C:\\Work\\Parser Workspace', 'C:\\Work\\Website'];
      const opts = {};
      let n = 0;
      const listed = { 'C:\\Work\\Parser Workspace': [{ sessionId: 'old-1', provider: 'devin', cwd: 'C:\\Work\\Parser Workspace', title: 'Draft Note 14 design doc', updatedAt: '2026-10-06T09:12:00Z' }], 'C:\\Work\\Website': [{ sessionId: 'web-1', provider: 'devin', cwd: 'C:\\Work\\Website', title: 'Roadmap page review', updatedAt: '2026-10-05T09:12:00Z' }] };
      const state = () => ({ defaultProvider: 'devin', workspace: ws, recent: [ws].concat(recent.filter(r => r !== ws)), approvals: 'dinah', devin: 'devin.exe', catalog: mkOpts('swe-1-6'), defaults, pinned, showThinking: false });
      const rec = (name) => (...a) => { window.__calls.push([name, ...a]); return Promise.resolve(); };
      return {
        State: async () => state(),
        AgentProviders: async () => ['devin', 'codex', 'claude', 'ollama'].map(id => window.__provider(id)),
        Sessions: async () => (listed[ws] || []).filter(x => hidden.indexOf(x.sessionId) < 0),
        NewSessionAt: async () => { const id = 'new-' + (++n); const model = defaults.model || 'swe-1-6'; opts[id] = mkOpts(model); setTimeout(() => emit('agent:options', { sessionId: id, options: opts[id] }), 0); return { sessionId: id, provider: 'devin', cwd: ws, title: '', open: true, working: false, queued: [] }; },
        OpenSessionAt: async (workspace, id) => { opts[id] = mkOpts('swe-1-6');
          emit('agent:update', { sessionId: id, update: { sessionUpdate: 'user_message_chunk', content: { type: 'text', text: 'Draft the Note 14 design doc.' } } });
          emit('agent:update', { sessionId: id, update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'Here is the draft.' } } });
          emit('agent:options', { sessionId: id, options: opts[id] });
          return { sessionId: id, provider: 'devin', cwd: ws, title: 'Draft Note 14 design doc', open: true, working: false, queued: [] }; },
        Send: async (id, text) => { window.__calls.push(['Send', id, text]); emit('agent:session', { sessionId: id, provider: 'devin', cwd: ws, title: text.slice(0, 30), open: true, working: true, queued: [] }); },
        Queue: rec('Queue'), Steer: rec('Steer'), Unqueue: rec('Unqueue'), Answer: rec('Answer'), Cancel: rec('Cancel'), Stop: rec('Stop'),
        SetWorkspace: async (p) => { ws = p; return state(); }, ForgetWorkspace: async () => state(), ChooseWorkspace: async () => state(), SetApprovals: rec('SetApprovals'),
        SessionOptions: async (id) => opts[id],
        SetSessionOption: async (id, o, v) => { window.__calls.push(['SetSessionOption', id, o, v]); opts[id] = opts[id].map(x => x.id === o ? Object.assign({}, x, { currentValue: v }) : x); return opts[id]; },
        SetDefault: async (o, v) => { if (v === null) delete defaults[o]; else defaults[o] = v; window.__calls.push(['SetDefault', o, v]); return defaults; },
        SetPinned: async (v, p) => { pinned = pinned.filter(x => x !== v); if (p) pinned.push(v); return pinned; },
        ProviderModelPreferences: async provider => ({provider,catalog:mkOpts('swe-1-6'),defaults,pinned}),
        SetDefaultForProvider: async (provider,o,v) => {if(provider!=='devin')throw new Error('Wrong provider');return window.go.main.App.SetDefault(o,v);},
        SetPinnedForProvider: async (provider,v,on) => {if(provider!=='devin')throw new Error('Wrong provider');return window.go.main.App.SetPinned(v,on);},
        DeleteSession: async (id) => { throw "Session '" + id + "' is already open in another process. Close the other instance before opening it here."; }, HideSession: async (id) => { hidden.push(id); }, UnhideSessions: async () => { hidden = []; }, RenameSession: async (id, t) => { window.__calls.push(['RenameSession', id, t]); return t ? { [id]: t } : {}; }, SetShowThinking: rec('SetShowThinking'), SaveHistory: async () => '', ClearHistory: rec('ClearHistory'), SaveRun: async () => ''
      };
    },
    init: () => { window.__calls = []; },
  });
  const p = require('./lib/windows')(t.page);
  const { ok } = t;
  ok('sidebar shows the workspace name', (await p.$eval('#ws-select', s => s.selectedOptions[0].textContent)).trim() === 'Parser Workspace');
  // Launch waits for UI state, but the session fetch and scheduled list render
  // can still be pending. This assertion needs the expected row to arrive.
  await p.waitForFunction(() => document.querySelector('#sessions a[data-session="old-1"]')?.textContent.includes('Draft Note 14 design doc'));
  ok('the earlier session is listed', (await p.textContent('#sessions')).includes('Draft Note 14 design doc'));
  await t.shot('s-start');
  // The workspace chooser.
  ok('the workspace chooser lists recents', (await p.$$('#ws-select option')).length === 2);
  await p.selectOption('#ws-select', 'C:\\Work\\Website');
  await p.waitForTimeout(300);
  ok('switching workspace shows its sessions', (await p.textContent('#sessions')).includes('Roadmap page review') && !(await p.textContent('#sessions')).includes('Note 14'));
  await p.selectOption('#ws-select', 'C:\\Work\\Parser Workspace');
  await p.waitForTimeout(300);
  // Typing with no session starts one.
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Summarise the plan overview.');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  await p.waitForTimeout(300);
  const send = await p.evaluate(() => window.__calls.filter(c => c[0] === 'Send'));
  ok('sending with no session starts one and sends to it', send.length === 1 && send[0][1] === 'new-1', JSON.stringify(send));
  const chat = () => p.locator('.win[data-chat-current]');
  ok('the mode reads Work', await shows(chat(), 'mode', 'Work'));
  ok('the model button shows the session model', await shows(chat(), 'model', 'SWE-1.6'));
  // A second session.
  await p.evaluate(() => window.__emit('agent:session', { sessionId: 'new-1', cwd: 'C:\\Work\\Parser Workspace', title: 'Summarise the plan overview', open: true, working: false, queued: [] }));
  await p.click('#chat-browser-view [data-cmd="new-session"].btn');
  await p.waitForTimeout(300);
  ok('New Session opens a draft without creating another agent session', (await p.$$('#sessions .md-row')).length === 2 && (await p.getAttribute('.win[data-chat-current]', 'data-win-kind')) === 'chat');
  ok('the new session has an empty log', !(await p.isHidden('.win[data-chat-current] [data-view-id=log-empty]')));
  // A question from the session not in front.
  await p.evaluate(() => {
    window.__emit('agent:update', { sessionId: 'new-1', update: { sessionUpdate: 'tool_call', toolCallId: 't1', title: 'Ran command', rawInput: { command: 'git push origin main' } } });
    window.__emit('agent:permission', { sessionId: 'new-1', key: 'q1', toolCall: { toolCallId: 't1' }, options: [{ optionId: 'allow_once', name: 'Allow', kind: 'allow_once' }, { optionId: 'reject_once', name: 'Reject', kind: 'reject_once' }] });
    window.__emit('agent:chatstate', { stateGen: 'ui3', stateVer: 1, chats: [{ sessionId: 'new-1', life: 'open', window: true, exec: { state: 'waiting', why: 'Needs your answer or approval' } }], groups: [] });
  });
  await p.waitForTimeout(200);
  ok('a background question is counted', (await p.textContent('#status-needs .badge')) === '1');
  ok('it is not shown over this session', await p.isHidden('#needs-you'));
  ok('the session list marks it', (await p.getAttribute('#sessions .session-attention', 'aria-label')) === '1 question waits for you' && /Waiting/.test(await p.textContent('#sessions .session-badge.chat-state-waiting')) && (await p.textContent('#sessions .browser-group-title')) === 'Needs You');
  await p.click('#status-needs');
  await p.click('#notifications-status-menu [data-cmd="needs-you"]');
  await p.waitForTimeout(200);
  ok('Notifications opens its own window', await p.isVisible('[data-win="needs-you"]'));
  ok('and focuses Allow', await p.evaluate(() => document.activeElement && document.activeElement.textContent === 'Allow'));
  await t.shot('s-question');
  // Reopen an earlier session.
  await p.click('#sessions a:has-text("Draft Note 14 design doc")');
  await p.waitForTimeout(300);
  ok('reopening replays the history', (await p.textContent('.win[data-chat-current] [data-view-id=logs]')).includes('Here is the draft.') && (await p.textContent('.win[data-chat-current] [data-view-id=logs]')).includes('Draft the Note 14 design doc.'));
  // The model drop-up.
  const favs = (await rowsOf(p, chat(), 'model')).filter(t => t !== 'No Favorites Yet' && t !== 'All Models…');
  ok('the drop-up offers the session model and the favorites', favs.length === 3 && favs[0].includes('(This Session)'), JSON.stringify(favs));
  await t.shot('s-modelmenu');
  await p.locator('.menu-panel:popover-open .menu-action', { hasText: 'GPT-6 Sol High' }).first().click();
  await p.waitForTimeout(200);
  ok('choosing a favorite sets this session only', JSON.stringify(await p.evaluate(() => window.__calls.filter(c => c[0] === 'SetSessionOption').pop())) === JSON.stringify(['SetSessionOption', 'old-1', 'model', 'gpt-6-sol-high']));
  ok('the button follows', await shows(chat(), 'model', 'GPT-6 Sol High'));
  await choose(p, chat(), 'model', 'All Models…');
  await p.waitForTimeout(300);
  ok('All Models opens its own dialog', await p.evaluate(() => document.querySelector('.win[data-chat-current] [data-view-id=models]').open && !document.getElementById('settings').open));
  await t.shot('s-models');
  await p.fill('.win[data-chat-current] [data-view-id=models-filter]', 'low');
  await p.click('.win[data-chat-current] [data-view-id=models-list] a:has-text("Claude Opus 5.5 Low")');
  await p.waitForTimeout(200);
  ok('choosing in the dialog sets the model and closes it', !(await p.evaluate(() => document.querySelector('.win[data-chat-current] [data-view-id=models]').open)) && await shows(chat(), 'model', 'Claude Opus 5.5 Low'));
  // Settings: defaults and favorites.
  await p.evaluate(() => document.querySelector('[data-cmd="settings"]').click());
  await p.locator('#settings').getByRole('tab', {name:'Chat',exact:true}).click();
  await p.waitForTimeout(300);
  await t.shot('s-settings');
  await p.click('#defaults [data-open="model-prefs"]');
  await p.waitForTimeout(200);
  await p.click('#prefs-list a:has-text("Claude Opus 5.5 Medium")');
  await p.waitForTimeout(100);
  ok('the default is saved', JSON.stringify(await p.evaluate(() => window.__calls.filter(c => c[0] === 'SetDefault').pop())) === JSON.stringify(['SetDefault', 'model', 'claude-opus-5-5-medium']));
  await p.click('#prefs-list .model-row:has-text("Adaptive") .model-pin');
  await p.waitForTimeout(200);
  ok('pinning a model makes it a favorite', (await p.getAttribute('#prefs-list .model-row:has-text("Adaptive") .model-pin', 'aria-pressed')) === 'true');
  await p.keyboard.press('Escape');
  await p.waitForTimeout(100);
  await p.keyboard.press('Escape');
  await p.click('[data-cmd="new-session"].btn');
  await p.waitForTimeout(300);
  ok('a new session starts with the default model', await shows(chat(), 'model', 'Claude Opus 5.5 Medium'));
  // Renaming.
  await p.dblclick('#sessions a:has-text("Draft Note 14 design doc")');
  await p.waitForTimeout(100);
  ok('double-click turns the row into a field', await p.isVisible('#sessions input.session-rename'));
  await p.fill('#sessions input.session-rename', 'Note 14 Error Recovery');
  await p.keyboard.press('Enter');
  await p.waitForTimeout(200);
  ok('Enter saves the name', JSON.stringify(await p.evaluate(() => window.__calls.filter(c => c[0] === 'RenameSession').pop())) === JSON.stringify(['RenameSession', 'old-1', 'Note 14 Error Recovery']));
  ok('the list shows the new name', (await p.textContent('#sessions')).includes('Note 14 Error Recovery'));
  await p.click('#sessions a:has-text("Note 14 Error Recovery")');
  await p.waitForTimeout(200);
  ok('the window title shows it', (await p.textContent('.win[data-chat-current] .win-title')) === 'Note 14 Error Recovery');
  await p.click('.win[data-chat-current] [data-view-id=activity]');
  await p.keyboard.press('F2');
  await p.waitForTimeout(100);
  ok('F2 opens the rename form', await p.isVisible('.chat-rename-dialog input.session-rename'));
  await p.keyboard.press('Escape');
  await p.waitForTimeout(100);
  ok('Esc leaves the name as it was', (await p.textContent('.win[data-chat-current] .win-title')) === 'Note 14 Error Recovery' && !(await p.isVisible('.chat-rename-dialog input.session-rename')));
  await p.evaluate(() => dinahCommands.run('rename-session'));
  await p.waitForTimeout(100);
  await p.fill('.chat-rename-dialog input.session-rename', 'Note 14 Design Doc');
  await p.keyboard.press('Enter');
  await p.waitForTimeout(200);
  ok('the rename action updates the window title', (await p.textContent('.win[data-chat-current] .win-title')) === 'Note 14 Design Doc' && (await p.textContent('#sessions')).includes('Note 14 Design Doc'));
  await t.shot('s-head', { clip: { x: 288, y: 50, width: 992, height: 90 } });
  // Removing: Devin refuses, so the session is hidden instead.
  await p.evaluate(() => dinahCommands.run('remove-session'));
  await p.waitForTimeout(100);
  ok('remove asks first', await p.evaluate(() => document.getElementById('remove').open) && (await p.textContent('#remove-body')).includes('session history'));
  await p.click('#remove-go');
  await p.waitForTimeout(200);
  ok('a refusal offers to hide it instead', await p.isVisible('#remove-hide') && (await p.textContent('#remove-body')).includes('already open in another process'));
  await t.shot('s-remove');
  await p.click('#remove-hide');
  await p.waitForTimeout(300);
  ok('hiding takes it off the list and out of the front', !(await p.textContent('#sessions')).includes('Note 14 Design Doc') && (await p.textContent('.win[data-chat-current] .win-title')) === 'New Chat');
  await p.evaluate(() => dinahWorkbench.open('chat-browser'));
  await p.locator('.menubar-title').filter({ hasText: /^Actions$/ }).click();
  ok('Show Hidden Chats Again is available in the Chats browser', await p.locator('#menubar-panel').getByText('Show Hidden Chats Again', { exact: true }).isEnabled());
  await t.close();
})();
