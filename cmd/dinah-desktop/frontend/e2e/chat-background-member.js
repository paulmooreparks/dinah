// Opening a background team member draws the saved copy before catching up,
// and consumes only journal entries newer than that copy.
const { launch } = require('./lib/page');
(async () => {
 for (const resumed of [true, false]) {
  const t = await launch({ viewport: { width: 1400, height: 950 }, args: { resumed }, app: ({ resumed }) => {
    const ws = 'C:/Fixture';
    const lead = { chat: 'lead', bus: 'bus-lead', title: 'Team Lead', group: 'team' };
    window.__reads = []; window.__opens = [];
    window.__catchup = new Promise(resolve => { window.__finishCatchup = resolve; });
    const info = id => ({ sessionId: id, provider: 'claude', cwd: ws, title: id === 'lead' ? 'Team Lead' : 'Background Member', open: true, working: id !== 'lead', resumed: id !== 'lead' && resumed, streamGen: id + '-gen', streamHead: id === 'lead' ? 0 : 5, chatState: { life: 'open', window: id === 'lead', exec: { state: id === 'lead' ? 'idle' : 'working', why: '' } } });
    return {
      Sessions: async () => [{ ...info('lead'), open: false }], SessionsAt: async () => [],
      ResolveSessionContinuation: async id => id,
      Team: async id => ({ lead: id === 'lead' ? lead : { chat: id }, group: id === 'lead' ? 'team' : '', stateGen: 'teams', stateVer: 1, members: id === 'lead' ? [{ chat: 'member', bus: 'bus-member', title: 'Background Member', provider: 'claude', workspace: 'Fixture', life: 'open', window: false, exec: { state: 'working', why: 'Turn in progress' }, members: [] }] : [], counts: { top: 'working', working: 1, idle: 0, waiting: 0, stuck: 0, unknown: 0, closed: 0 } }),
      SavedConversation: async id => id === 'lead' ? { available: false, messages: [] } : { available: true, complete: true, streamGen: 'member-gen', streamThrough: 4, messages: [{ role: 'you', text: 'The saved assignment.' }, { role: 'agent', text: 'SAVED-MEMBER-ANSWER' }] },
      ChatLead: async id => id === 'member' ? lead : null,
      SessionContinuation: async () => ({ history: [] }),
      OpenSessionAt: async (ws, id) => { __opens.push(id); return info(id); },
      ChatEvents: async (id, gen, after) => {
        __reads.push([id, gen, after]);
        if (id === 'lead') return { gen, head: 0, events: [] };
        await __catchup;
        const entry = (n, text) => ({ n, name: 'agent:update', data: { update: { sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } } } });
        return { gen, head: 5, events: [entry(4, 'SAVED-MEMBER-ANSWER'), entry(5, ' [current turn] ')].filter(e => e.n > after) };
      },
    };
  } });
  const { page, ok } = t;
  try {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'lead' }));
    const lead = page.locator('[data-win="' + key + '"]');
    await lead.locator('.chat-team > button.disclosure').click();
    await lead.locator('[data-team-member="bus-member"] > a').click();
    const member = page.locator('.win[data-chat-current]');
    await member.getByText('SAVED-MEMBER-ANSWER', { exact: true }).first().waitFor();
    ok('the saved copy is readable before the catch-up request returns', await member.locator('[data-view-id=activity]').getByText('The saved assignment.', { exact: true }).last().isVisible() && await member.locator('[data-view-id=replay]').isHidden());
    await page.waitForFunction(() => __reads.some(read => read[0] === 'member'));
    ok('opening the member starts at the saved journal checkpoint', await page.evaluate(() => __reads.filter(read => read[0] === 'member').every(read => read[1] === 'member-gen' && read[2] >= 4)), JSON.stringify(await page.evaluate(() => __reads)));
    await page.evaluate(() => __finishCatchup());
    await member.getByText('[current turn]', { exact: false }).first().waitFor();
    const text = await member.locator('[data-view-id=activity]').innerText();
    ok('the saved answer appears once followed by the current turn', text.split('SAVED-MEMBER-ANSWER').length === 2 && text.indexOf('SAVED-MEMBER-ANSWER') < text.indexOf('[current turn]'), text);
    ok('opening the member never asks for full history', await page.evaluate(() => !__bridgeCalls.some(call => call[0] === 'LoadFullHistory')));
    await page.evaluate(() => {
      const update = text => ({ sessionUpdate: 'agent_message_chunk', content: { type: 'text', text } });
      __emit('agent:update', { sessionId: 'member', gen: 'member-gen', n: 5, update: update(' [current turn] ') });
      __emit('agent:update', { sessionId: 'member', gen: 'member-gen', n: 6, update: update(' [next part] ') });
    });
    await member.getByText('[next part]', { exact: false }).first().waitFor();
    const later = await member.locator('[data-view-id=activity]').innerText();
    ok('a later part arrives once and an already drawn journal entry is ignored', later.split('[current turn]').length === 2 && later.split('[next part]').length === 2, later);
    await t.shot('background-member-' + (resumed ? 'resumed' : 'loaded'));

  } finally { await t.close(); }
 }
})().catch(error => { console.error(error); process.exitCode = 1; });
