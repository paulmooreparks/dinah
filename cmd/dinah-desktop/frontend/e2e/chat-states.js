// Every state surface consumes the engine's served fields and live snapshot.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const workspace = 'C:/Fixture';
    const states = ['working', 'waiting', 'stuck', 'idle', 'closed', 'retired', 'elsewhere'];
    const why = { working: 'Turn in progress', idle: 'No work in progress', waiting: 'Context full', stuck: 'The provider cannot run this chat' };
    const chats = states.map(state => ({ sessionId: state, title: state[0].toUpperCase() + state.slice(1) + ' Chat', provider: 'codex', cwd: workspace,
      open: state !== 'closed' && state !== 'retired', working: state === 'working',
      chatState: { life: state === 'closed' || state === 'retired' ? state : 'open', window: state !== 'idle' && state !== 'elsewhere', ...(state === 'elsewhere' ? { elsewhere: 'Running in another Dinah window' } : !['closed', 'retired'].includes(state) ? { exec: { state, why: why[state] || '' } } : {}) }
    }));
    const totals = { top: 'stuck', stuck: 1, waiting: 1, working: 1, idle: 1, closed: 2, unknown: 1 };
    const members = chats.map(chat => ({ id: 'bus-' + chat.sessionId, bus: 'bus-' + chat.sessionId, chat: chat.sessionId, title: chat.title, workspace: 'Fixture', provider: 'codex', members: [], ...chat.chatState }));
    const group = { name: 'states', title: 'State Review', members, counts: totals };
    const team = { lead: { chat: 'working', bus: 'bus-working', title: 'Working Chat' }, group: 'states', members: members.slice(1), counts: totals, stateGen: 'team-states', stateVer: 1 };
    window.__stateSnapshot = { chats: chats.map(chat => ({ sessionId: chat.sessionId, bus: 'bus-' + chat.sessionId, ...chat.chatState })), groups: [{ name: 'states', counts: totals }], stateGen: 'states', stateVer: 1 };
    window.__changeExecution = () => {
      const entry = __stateSnapshot.chats.find(chat => chat.sessionId === 'waiting');
      entry.exec = { state: 'working', why: '' };
      __stateSnapshot.groups[0].counts = { ...totals, waiting: 0, working: 2 };
      __stateSnapshot.stateVer++;
      __emit('agent:chatstate', structuredClone(__stateSnapshot));
    };
    return {
      State: async () => ({ workspace, workspaces: [workspace], recent: [workspace], defaultProvider: 'codex' }),
      Sessions: async () => structuredClone(chats), SessionsAt: async () => structuredClone(chats),
      OpenSessionAt: async (cwd, id) => structuredClone(chats.find(chat => chat.sessionId === id)),
      ChatStates: async () => structuredClone(__stateSnapshot),
      ChatLead: async () => null, Team: async id => id === 'working' ? structuredClone(team) : null,
      BusOverview: async () => ({ inbox: [], unread: 0, rooms: [group], workspaces: [], links: [] }),
      BusGroupMessages: async () => [{ id: 1, from: 'Working Chat', room: 'states', text: 'The shared transcript uses the same spacing and controls as a chat.', createdAt: '2026-10-30T08:00:00Z' }],
      BusMarkGroupRead: async () => {},
      Notifications: async () => [{ id: 'historical-failure', kind: 'turn-failure', severity: 'error', title: 'An earlier turn failed', body: 'The chat has since started another turn', sessionId: 'working', sessionTitle: 'Working Chat', workspace, provider: 'codex', created: '2026-10-30T07:00:00Z' }]
    };
  } });
  const { page, ok } = t;
  try {
    const groupKey = await page.evaluate(() => dinahWorkbench.open({ kind: 'bus-room', path: 'states' }));
    const group = page.locator('[data-win="' + groupKey + '"]');
    await group.locator('[data-group-member="bus-working"] .chat-state-working').waitFor();
    for (const state of ['working', 'waiting', 'stuck', 'idle']) {
      const badge = group.locator('[data-group-member="bus-' + state + '"] .chat-state-' + state);
      ok('group member displays the ' + state + ' word and shared glyph', await badge.isVisible() && (await badge.innerText()).toLowerCase().includes(state) && await badge.evaluate(el => getComputedStyle(el, '::before').content !== 'none'));
    }
    for (const life of ['closed', 'retired']) {
      const badge = group.locator('[data-group-member="bus-' + life + '"] .chat-lifecycle');
      ok(life + ' is lifecycle text without an execution glyph', await badge.isVisible() && await badge.evaluate(el => getComputedStyle(el, '::before').content === 'none'));
    }
    ok('waiting and stuck carry their reasons', /Context full/.test(await group.innerText()) && /provider cannot run/.test(await group.innerText()));
    ok('the group aggregate uses engine counts and priority', /1 stuck, 1 waiting, 1 working, 1 idle, 1 running elsewhere, 2 closed/.test(await group.locator('.group-aggregate').innerText()));
    const elsewhere = group.locator('[data-group-member="bus-elsewhere"] .chat-lifecycle');
    ok('activity in another window is words without a claimed execution state', /Running in another Dinah window/.test(await elsewhere.innerText()) && await elsewhere.evaluate(el => getComputedStyle(el, '::before').content === 'none'));
    async function usableTranscript(name) {
      await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] .bus-log').clientHeight >= 128, groupKey);
      const geometry = await group.evaluate(win => {
        const body = win.querySelector('.bus-conversation'), shell = win.querySelector('.group-member-shell'), list = win.querySelector('.group-members'), log = win.querySelector('.bus-log');
        return { body: body.clientHeight, members: shell.getBoundingClientRect().height, transcript: log.clientHeight, overflow: list.scrollHeight > list.clientHeight };
      });
      ok(name + ' reserves the conversation with seven expanded members', geometry.transcript >= 128 && geometry.members <= geometry.body * .4 + 2, JSON.stringify(geometry));
      return geometry;
    }
    const standard = await usableTranscript('the standard window');
    ok('a long member list scrolls inside its bounded area', standard.overflow);
    for (const theme of ['light', 'dark']) {
      await page.evaluate(theme => pudlSetTheme(theme), theme);
      await t.shot('group-states-' + theme);
    }
    await group.getByRole('button', { name: /^Members/ }).click();
    ok('folding members preserves aggregate words', await group.locator('.group-members').isHidden() && await group.locator('.group-aggregate').isVisible());
    await page.setViewportSize({ width: 700, height: 900 });
    await group.locator('[data-win-action="maximize"]').click();
    await group.getByRole('button', { name: /^Members/ }).click();
    await usableTranscript('the narrow window');
    const tops = await group.locator('[data-group-member]').evaluateAll(chips => chips.map(chip => Math.round(chip.getBoundingClientRect().top)));
    ok('short member chips share a row when there is room', new Set(tops).size < tops.length, JSON.stringify(tops));
    await t.shot('group-members-narrow-expanded');
    await group.getByRole('button', { name: /^Members/ }).click();
    await group.locator('.group-composer textarea').fill('A multiline draft for the group.\nThe attachment stays beside the message.\nThis draft has not been sent.');
    await group.locator('.group-composer input[type="file"]').setInputFiles({ name: 'review-notes.txt', mimeType: 'text/plain', buffer: Buffer.from('Review notes for the group.') });
    await group.locator('.attachment-name', { hasText: 'review-notes.txt' }).waitFor();
    await t.shot('group-states-narrow');
    await page.evaluate(() => {
      const root = document.documentElement, style = getComputedStyle(root);
      window.__reviewTextTokens = ['2xs', 'xs', 'sm', 'md', 'base', 'lg', 'xl', '2xl', '3xl'].map(size => ['--text-' + size, style.getPropertyValue('--text-' + size)]);
      __reviewTextTokens.forEach(([name, value]) => root.style.setProperty(name, parseFloat(value) * 2 + 'px'));
    });
    await page.setViewportSize({ width: 420, height: 900 });
    await group.locator('.group-toolbar [data-toolbar-stage]:not([data-toolbar-stage="1"])').waitFor();
    ok('the folded aggregate remains fully visible with large text', await group.evaluate(win => win.querySelector('.group-aggregate').getBoundingClientRect().bottom <= win.querySelector('.group-member-shell').getBoundingClientRect().bottom));
    await page.waitForFunction(key => {
      const win = document.querySelector('[data-win="' + key + '"]'), log = win.querySelector('.bus-log'), text = win.querySelector('.msg-text p');
      const viewport = log.getBoundingClientRect(), message = text.getBoundingClientRect();
      return log.clientHeight >= 128 && message.height > 0 && message.top >= viewport.top && message.bottom <= viewport.bottom;
    }, groupKey);
    ok('large text leaves a complete message paragraph visible in the transcript', true);
    await t.shot('group-states-large-text');
    for (const width of [320, 280, 240]) {
      await page.setViewportSize({ width, height: 900 });
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      if (Number(await group.locator('.group-toolbar [data-toolbar-stage]').getAttribute('data-toolbar-stage')) >= 3) break;
    }
    const overflow = group.locator('.group-toolbar :is(.toolbar-fold, .toolbar-more) > .menu-btn:visible').first();
    await overflow.click();
    const menu = page.locator('.menu-panel:popover-open');
    await menu.getByText('Add Chats…', { exact: true }).waitFor();
    await menu.getByText('Turn Budget…', { exact: true }).waitFor();
    await t.shot('group-toolbar-overflow-open');
    await page.keyboard.press('Escape');
    ok('the command row uses PUDL overflow without a wrapping command row', await group.locator('.group-toolbar[data-toolbar-fold]').count() === 1);
    await page.evaluate(() => __reviewTextTokens.forEach(([name]) => document.documentElement.style.removeProperty(name)));
    await page.setViewportSize({ width: 1440, height: 1000 });
    const chatKey = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'working' }));
    const chat = page.locator('[data-win="' + chatKey + '"]');
    await chat.locator('.win-execution .win-state[data-state="working"]').waitFor({ state: 'attached' });
    const statusControl = chat.locator('.win-status-toggle');
    if (await statusControl.isVisible()) {
      const panelId = await statusControl.getAttribute('popovertarget');
      await statusControl.click();
      await page.locator('#' + panelId).getByRole('img', { name: 'Working: Fixture · Turn in progress', exact: true }).waitFor();
      ok('folded status keeps the full workspace and execution reason', true);
      await page.keyboard.press('Escape');
    }
    await page.evaluate(() => __emit('agent:update', { sessionId: 'working', update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text: 'The shared transcript uses the same spacing and controls as a chat.' } } }));
    await chat.locator('.msg-text', { hasText: 'The shared transcript uses' }).waitFor();
    await group.locator('.group-composer textarea').fill('');
    await page.evaluate(key => pudlWindows.raise(key), groupKey);
    await page.evaluate(() => pudlSetTheme('light'));
    await t.shot('comparison-group');
    await page.evaluate(key => pudlWindows.raise(key), chatKey);
    await chat.locator('[data-win-action="maximize"]').click();
    await t.shot('comparison-chat');
    ok('chat and group comparison windows use the same size', JSON.stringify(await chat.boundingBox()) === JSON.stringify(await group.boundingBox()));
    ok('the title bar presents the execution state once through PUDL', await chat.locator('.win-execution .win-state[data-state="working"]').count() === 1 && await chat.locator('.chat-working-chip .chat-state-working').count() === 0);
    await chat.locator('.chat-team > button.disclosure').click();
    await chat.locator('[data-team-member="bus-waiting"] .chat-state-waiting').waitFor();
    await t.shot('chat-team-states');
    await page.setViewportSize({ width: 1920, height: 1100 });
    await page.evaluate(({ chatKey, groupKey }) => { pudlWindows.snap(chatKey, 'left'); pudlWindows.snap(groupKey, 'right'); }, { chatKey, groupKey });
    for (const expanded of [false, true]) {
      const teamToggle = chat.locator('.chat-team > button.disclosure'), memberToggle = group.getByRole('button', { name: /^Members/ });
      if ((await teamToggle.getAttribute('aria-expanded') === 'true') !== expanded) await teamToggle.click();
      if ((await memberToggle.getAttribute('aria-expanded') === 'true') !== expanded) await memberToggle.click();
      for (const theme of ['light', 'dark']) {
        await page.evaluate(theme => pudlSetTheme(theme), theme);
        await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
        await t.shot('chat-group-side-by-side-' + theme + '-' + (expanded ? 'expanded' : 'folded'));
      }
    }
    ok('the chat body has no duplicate name, status line or title actions', await chat.locator('.win-body h1, .win-body .page-state, .session-actions').count() === 0);
    const paired = { chat: await chat.boundingBox(), group: await group.boundingBox() };
    ok('side-by-side chat and group have equal width and height', Math.abs(paired.chat.width - paired.group.width) < 2 && Math.abs(paired.chat.height - paired.group.height) < 2, JSON.stringify(paired));
    ok('the state chip keeps workspace and reason in its tooltip', /Fixture/.test(await chat.locator('.win-execution .win-state').getAttribute('title')));
    await page.setViewportSize({ width: 1440, height: 1000 });
    await chat.locator('[data-win-action="maximize"]').click();
    for (const state of ['working', 'waiting', 'stuck', 'idle']) {
      const key = state === 'working' ? chatKey : await page.evaluate(state => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: state }), state);
      const current = page.locator('[data-win="' + key + '"]');
      await current.locator('.win-execution .win-state[data-state="' + state + '"]').waitFor({ state: 'attached' });
      for (const theme of ['light', 'dark']) {
        await page.evaluate(theme => pudlSetTheme(theme), theme);
        await t.shot('chat-state-' + state + '-' + theme);
      }
      if (state !== 'working') await page.evaluate(key => pudlWindows.close(key), key);
    }
    await page.evaluate(key => pudlWindows.raise(key), chatKey);
    const prompt = chat.locator('[data-view-id="prompt"]');
    await prompt.fill('@Waiting'); await prompt.press('End');
    await chat.locator('.mention-row .chat-state-waiting').waitFor();
    await t.shot('chat-mention-states');
    await prompt.fill(''); await prompt.press('Escape');
    await page.evaluate(() => dinahWorkbench.open('needs-you'));
    await page.locator('#questions [data-notice-id="historical-failure"] .notice-chat-state.chat-state-working').waitFor();
    ok('a historical failure keeps its event tone beside the current Working state', await page.locator('#questions [data-notice-id="historical-failure"]').getAttribute('data-type') === 'error');
    await t.shot('notification-current-chat-state');
    await page.evaluate(() => dinahWorkbench.open({ kind: 'group-picker', path: JSON.stringify({ group: 'states' }) }));
    await page.locator('.group-picker .chat-state-waiting').waitFor();
    await t.shot('add-chats-states');
    await page.evaluate(() => __changeExecution());
    await group.locator('[data-group-member="bus-waiting"] .chat-state-working').waitFor({ state: 'attached' });
    ok('one numbered event updates a group member and its aggregate', /2 working/.test(await group.locator('.group-aggregate').innerText()));
    await page.evaluate(() => __emit('agent:chatstate', { stateGen: 'states', stateVer: 1, chats: [], groups: [] }));
    ok('an older event cannot replace the current execution state', await group.locator('[data-group-member="bus-waiting"] .chat-state-working').count() === 1);
    await page.evaluate(() => __resync());
    await page.waitForFunction(() => __bridgeCalls.filter(call => call[0] === 'ChatStates').length >= 2);
    await group.locator('[data-group-member="bus-waiting"] .chat-state-working').waitFor({ state: 'attached' });
    ok('resync rereads the same authoritative state', (await t.calls('ChatStates')).length >= 2);
  } catch (error) { await t.shot('chat-states-failure'); console.error(await page.locator('.group-window').evaluate(win => { const log = win.querySelector('.bus-log'), text = win.querySelector('.msg-text p'); return { height: log.clientHeight, log: log.getBoundingClientRect().toJSON(), text: text && text.getBoundingClientRect().toJSON(), html: win.querySelector('.msg-text').innerHTML }; })); throw error; } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
