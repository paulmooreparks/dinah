// Completion choices remain outside the bounded composer's scrolling body.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1200, height: 900 }, app: () => {
    const cwd = 'C:/Fixture';
    const chatState = { life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } };
    const chats = ['Writer', 'Reviewer'].map(title => ({ sessionId: title.toLowerCase(), title, cwd, provider: 'codex', open: true, chatState }));
    const members = chats.map(chat => ({ id: 'bus-' + chat.sessionId, chat: chat.sessionId, title: chat.title, ...chatState }));
    const room = { name: 'review', title: 'Review', members, counts: { top: 'idle', idle: 2 } };
    window.__teamRouting = team => { room.lead = team ? 'bus-reviewer' : ''; __emit('bus:changed', {}); };
    return {
      State: async () => ({ workspace: cwd, workspaces: [cwd], recent: [cwd] }),
      Sessions: async () => chats, SessionsAt: async () => chats,
      OpenSessionAt: async (workspace, id) => chats.find(chat => chat.sessionId === id),
      SkillsForProviderAt: async () => [{ name: 'review-draft', description: 'Review the current draft', source: 'workspace' }],
      BusOverview: async () => ({ inbox: [], unread: 0, rooms: [room], workspaces: [], links: [] }),
      BusGroupMessages: async () => [], BusMarkGroupRead: async () => {}
    };
  } });
  const { page, ok } = t;
  async function candidateVisible(row, name) {
    await row.waitFor();
    const geometry = await row.evaluate(el => {
      const r = el.getBoundingClientRect(), pop = el.closest('.slash-pop'), p = pop.getBoundingClientRect();
      const x = r.left + r.width / 2, y = r.top + r.height / 2, hit = document.elementFromPoint(x, y);
      return { height: r.height, visible: r.top >= p.top && r.bottom <= p.bottom && !!hit && el.contains(hit), outsideScroll: !el.closest('.composer-scroll') };
    });
    ok(name + ' displays the actual candidate outside the scrolling boundary', geometry.height > 0 && geometry.visible && geometry.outsideScroll, JSON.stringify(geometry));
  }
  try {
    const chatKey = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'writer' }));
    const chat = page.locator('[data-win="' + chatKey + '"]'), prompt = chat.locator('[data-view-id=prompt]');
    await prompt.fill('@Reviewer'); await prompt.press('End');
    await candidateVisible(chat.locator('.mention-row').filter({ hasText: 'Reviewer' }).first(), 'chat mention completion');
    await t.shot('chat-mention-visible');
    await prompt.press('ArrowDown'); await prompt.press('Enter');
    ok('the keyboard selects the chat mention', (await prompt.inputValue()).includes('@Reviewer '));
    await prompt.fill('/review'); await prompt.press('End');
    await candidateVisible(chat.locator('.skill-row').filter({ hasText: '/review-draft' }).first(), 'skill completion');
    await t.shot('chat-skill-visible');
    await prompt.press('ArrowDown'); await prompt.press('Enter');
    ok('the keyboard selects the skill', (await prompt.inputValue()).startsWith('/review-draft'));
    const groupKey = await page.evaluate(() => dinahWorkbench.open({ kind: 'bus-room', path: 'review' }));
    const group = page.locator('[data-win="' + groupKey + '"]'), box = group.locator('textarea');
    await box.fill('@Reviewer'); await box.press('End');
    await candidateVisible(group.locator('.mention-row').filter({ hasText: 'Reviewer' }).first(), 'group mention completion');
    ok('the group candidate retains its shared state colour and background', await group.locator('.mention-row .chat-state-idle').first().evaluate(el => el.style.getPropertyValue('--chat-state-colour') === 'var(--text-muted)' && getComputedStyle(el).backgroundColor !== 'rgba(0, 0, 0, 0)'));
    await t.shot('group-mention-visible');
    await box.press('ArrowDown'); await box.press('ArrowUp'); await box.press('Enter');
    ok('the keyboard selects the group mention', (await box.inputValue()).includes('@Reviewer '));
    await box.fill('A report for the group');
    await group.locator('.group-routing-summary', { hasText: 'No replies requested.' }).waitFor();
    await group.getByText('Who receives this?', { exact: true }).click();
    await t.shot('group-routing-disclosure');
    await page.evaluate(() => __teamRouting(true));
    await group.locator('.group-routing-summary', { hasText: 'Reply requested from Reviewer.' }).waitFor();
    ok('a team group names the lead who receives an unmentioned message', /asks the team lead/.test(await group.locator('.group-routing-details').innerText()));
    await t.shot('team-routing-disclosure');
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
