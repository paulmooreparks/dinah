// The skills menu, the "/" pop-up, and Queue as the default during a turn.
const { launch } = require('./lib/page');
const { openMenu } = require('./lib/commands');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    app: () => {
      const skills = [
        { name: 'spec-draft', description: 'Draft a design spec from the meeting notes.', source: 'workspace' },
        { name: 'spec-post', description: 'Publish a spec to its folder.', source: 'workspace' },
        { name: 'edit-request', description: 'Turn a request to change a spec or design note into a written edit request.', source: 'workspace' },
        { name: 'edit-run', description: 'Run the filed edit requests.', source: 'workspace' },
        { name: 'notes-draft', description: 'Write meeting notes from a transcript.', source: 'workspace' },
        { name: 'btw', description: 'Ask a quick side question without disrupting the main conversation', source: 'devin' },
        { name: 'compact', description: 'Force conversation compaction', source: 'devin' },
        { name: 'claude:andoneer', description: 'Universal conventions for working with Andoneer', source: 'imported' }];
      const ws = 'C:\\x\\parser';
      return {
        State: async () => ({ workspace: ws, recent: [ws], approvals: 'dinah', devin: 'devin.exe', catalog: [], defaults: {}, pinned: [], titles: {} }),
        AgentProviders: async () => [window.__provider('devin')],
        // The fixture defines SkillsForProviderAt, which the page prefers to
        // SkillsAt when it exists, so both give this suite's skills.
        SkillsAt: async () => skills, SkillsForProviderAt: async () => skills,
        NewSessionAt: async () => ({ sessionId: 's1', cwd: ws, title: '', open: true, working: false, queued: [] }),
        Send: async (id, t) => { window.__calls.push(['Send', t]); window.__emit('agent:session', { sessionId: 's1', cwd: ws, title: 'x', open: true, working: true, queued: [] }); },
        CanSteer: async () => !!window.__canSteer,
        Queue: async (id, t) => { window.__calls.push(['Queue', t]); }, Steer: async (id, t) => { window.__calls.push(['Steer', t]); },
        Cancel: async () => { window.__calls.push(['Cancel']); }, SetSessionOption: async () => []
      };
    },
    init: () => { window.__calls = []; },
  });
  const p = require('./lib/windows')(t.page);
  const { ok } = t;
  // The Skills button.
  const chat = p.locator('.win[data-chat-current]');
  const skillsMenu = await openMenu(p, chat, 'skills');
  // In a narrow window the Message group is folded, and its menu lists Add
  // and Skills under their names before the skills' own groups.
  const headings = await skillsMenu.locator('.md-section-label').allTextContents();
  const groups = headings.slice(Math.max(0, headings.indexOf('Workspace Skills')));
  ok('the skills menu identifies workspace skills', JSON.stringify(groups) === JSON.stringify(['Workspace Skills', 'Devin', 'From Other Tools']), JSON.stringify(groups));
  await t.shot('skills-menu');
  await skillsMenu.locator('.menu-action', { hasText: '/edit-request' }).first().click();
  ok('choosing a skill puts it in the box', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === '/edit-request ');
  // The "/" pop-up.
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', '');
  await p.type('.win[data-chat-current] [data-view-id=prompt]', '/spe');
  await p.waitForTimeout(150);
  ok('typing / opens the pop-up, filtered', await p.isVisible('.win[data-chat-current] [data-view-id=slash-pop]') && (await p.$$('.win[data-chat-current] [data-view-id=slash-pop] .skill-row')).length === 2);
  await t.shot('skills-pop');
  await p.keyboard.press('ArrowDown');
  await p.keyboard.press('Enter');
  ok('Down then Enter inserts the second match', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === '/spec-post ' && await p.isHidden('.win[data-chat-current] [data-view-id=slash-pop]'));
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', '');
  await p.type('.win[data-chat-current] [data-view-id=prompt]', 'Check the input and');
  await p.type('.win[data-chat-current] [data-view-id=prompt]', '/output');
  await p.waitForTimeout(100);
  ok('a slash inside a word stays plain', await p.isHidden('.win[data-chat-current] [data-view-id=slash-pop]'));
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', '');
  await p.type('.win[data-chat-current] [data-view-id=prompt]', 'Use \\/edit as text');
  await p.waitForTimeout(100);
  ok('an escaped slash stays plain', await p.isHidden('.win[data-chat-current] [data-view-id=slash-pop]'));
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Please run ');
  await p.press('.win[data-chat-current] [data-view-id=prompt]', 'End');
  await p.type('.win[data-chat-current] [data-view-id=prompt]', '/ed');
  await p.waitForTimeout(100);
  await p.keyboard.press('Tab');
  ok('a slash after a space works mid-sentence, Tab inserts', (await p.inputValue('.win[data-chat-current] [data-view-id=prompt]')) === 'Please run /edit-request ');
  await p.type('.win[data-chat-current] [data-view-id=prompt]', 'x');
  await p.press('.win[data-chat-current] [data-view-id=prompt]', 'Escape');
  // Queue is the default during a turn.
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Draft the release notes for 2.4.');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  await p.waitForTimeout(300);
  ok('during a turn the main button reads Queue', (await p.textContent('.win[data-chat-current] [data-view-id=send]')).trim() === 'Queue' && await p.isVisible('.win[data-chat-current] [data-view-id=steer-send]'));
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Also check the changelog.');
  await p.keyboard.press('Control+Enter');
  await p.waitForTimeout(100);
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Stop and use the release template.');
  await p.click('.win[data-chat-current] [data-view-id=steer-send]');
  await p.waitForTimeout(100);
  const calls = await p.evaluate(() => window.__calls.map(c => c[0] + ':' + (c[1] || '')));
  ok('Ctrl+Enter queues and Steer steers', JSON.stringify(calls) === JSON.stringify(['Send:Draft the release notes for 2.4.', 'Queue:Also check the changelog.', 'Steer:Stop and use the release template.']), JSON.stringify(calls));
  // A steer the agent's hooks hand over shows as pending on its row, then
  // as delivered.
  const steerLabel = () => p.$$eval('.win[data-chat-current] [data-view-id=logs] .msg-label', els => els.map(e => e.textContent).filter(t => t.startsWith('You, steering')).pop());
  const waitSteer = label => p.waitForFunction(label => [...document.querySelectorAll('.win[data-chat-current] [data-view-id=logs] .msg-label')].map(e => e.textContent).filter(t => t.startsWith('You, steering')).pop() === label, label);
  await p.evaluate(() => window.__emit('agent:steer', { sessionId: 's1', steer: 7, state: 'pending' }));
  await waitSteer("You, steering (waiting for the agent's next step)");
  ok('a pending steer says it waits for the agent\'s next step', (await steerLabel()) === 'You, steering (waiting for the agent\'s next step)', await steerLabel());
  await p.evaluate(() => window.__emit('agent:steer', { sessionId: 's1', steer: 7, state: 'delivered' }));
  await waitSteer('You, steering (delivered)');
  ok('a delivered steer says so', (await steerLabel()) === 'You, steering (delivered)', await steerLabel());
  // A native receipt acknowledges runtime acceptance, and a queued input
  // stays distinguishable from a message actually sent to a later turn.
  for (const [id, state, label] of [[8,'accepted','You, steering (accepted by the agent)'],[9,'queued','You, steering (queued for the next turn)'],[10,'failed','You, steering (delivery unconfirmed)']]) {
    await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Additional guidance '+id);
    await p.click('.win[data-chat-current] [data-view-id=steer-send]');
    await p.evaluate(({id,state})=>{__emit('agent:steer',{sessionId:'s1',steer:id,state:'pending'});__emit('agent:steer',{sessionId:'s1',steer:id,state});},{id,state});
  await waitSteer(label);
    ok('steering reports '+state, (await steerLabel())===label, await steerLabel());
    if (state==='queued') {
      await p.evaluate(id=>__emit('agent:steer',{sessionId:'s1',steer:id,state:'sent'}),id);
  await waitSteer('You, steering (sent after the turn ended)');
      ok('queued steering remains tracked until sent', (await steerLabel())==='You, steering (sent after the turn ended)', await steerLabel());
    }
  }

  // Ordinary keyboard sends follow the runtime capability; explicit buttons
  // retain their action regardless of that capability.
  async function lastCallIs(action) {
    await p.waitForFunction(action=>__calls.at(-1)[0]===action, action);
    return p.evaluate(action=>__calls.at(-1)[0]===action,action);
  }
  await p.evaluate(()=>{window.__canSteer=true;});
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Steer with Ctrl Enter');
  await p.keyboard.press('Control+Enter');
  ok('ordinary Ctrl Enter steers a capable model',await lastCallIs('Steer'));
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Steer with Enter');
  await p.keyboard.press('Enter');
  await p.waitForFunction(()=>__calls.at(-1)[1]==='Steer with Enter');
  ok('ordinary Enter steers a capable model',await lastCallIs('Steer'));
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Explicit queue');
  await p.click('.win[data-chat-current] [data-view-id=send]');
  ok('explicit Queue overrides steering capability',await lastCallIs('Queue'));
  await p.evaluate(()=>{window.__canSteer=false;});
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Queue unsupported model');
  await p.keyboard.press('Control+Enter');
  await p.waitForFunction(()=>__calls.at(-1)[1]==='Queue unsupported model');
  ok('ordinary send queues an unsupported model',await lastCallIs('Queue'));
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Explicit steer');
  await p.click('.win[data-chat-current] [data-view-id=steer-send]');
  ok('explicit Steer keeps its chosen action',await lastCallIs('Steer'));
  await p.evaluate(()=>{go.main.App.CanSteer=async()=>{throw new Error('capability unavailable');};});
  await p.fill('.win[data-chat-current] [data-view-id=prompt]', 'Queue unavailable capability');
  await p.keyboard.press('Control+Enter');
  ok('a failed capability lookup queues the message',await lastCallIs('Queue'));
  await t.shot('skills-working', { clip: { x: 288, y: 640, width: 992, height: 180 } });
  await t.close();
})();
