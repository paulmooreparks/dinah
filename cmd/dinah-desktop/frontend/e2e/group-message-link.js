// A real team-traffic link chooses an older message and releases following.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, app: () => {
    const state = { life: 'open', window: true, exec: { state: 'working', why: 'Turn in progress' } };
    const messages = Array.from({ length: 48 }, (_, i) => ({ id: 101 + i, room: 'team-room', from: 'Lead', text: 'Report ' + (101 + i) + '.\n\nThe report stays readable while newer messages arrive.', createdAt: '2026-10-30T08:00:00Z' }));
    window.__appendLinkedReport = () => { const id = messages.at(-1).id + 1; messages.push({ ...messages[0], id, text: 'Later report ' + id }); __emit('bus:changed', {}); return id; };
    return {
      State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'] }),
      Sessions: async () => [{ sessionId: 'lead', title: 'Lead', cwd: 'C:/Fixture', provider: 'codex', open: true, chatState: state }],
      OpenSessionAt: async () => ({ sessionId: 'lead', title: 'Lead', cwd: 'C:/Fixture', provider: 'codex', open: true, chatState: state }),
      Team: async () => ({ lead: { bus: 'bus-lead', chat: 'lead', title: 'Lead' }, group: 'team-room', members: [], counts: { top: 'working', working: 1 }, stateGen: 'team', stateVer: 1 }),
      ChatLead: async () => null,
      BusOverview: async () => ({ rooms: [{ name: 'team-room', title: 'Team Reports', lead: 'bus-lead', members: [], counts: { top: 'working', working: 1 } }], workspaces: [], inbox: [], links: [] }),
      BusGroupMessages: async () => messages.slice(), BusMarkGroupRead: async () => {}
    };
  } });
  const { page, ok } = t;
  let log;
  try {
    const roomKey = await page.evaluate(() => dinahBus.openGroup('team-room'));
    const room = page.locator('[data-win="' + roomKey + '"]'); log = room.locator('.bus-log');
    await room.locator('[data-bus-message="148"]').waitFor({ state: 'attached' });
    await page.waitForFunction(key => { const log = document.querySelector('[data-win="' + key + '"] .bus-log'); return log.dataset.following === 'true' && log.scrollHeight - log.scrollTop - log.clientHeight < 24; }, roomKey);
    const chatKey = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Fixture', path: 'lead' }));
    const chat = page.locator('[data-win="' + chatKey + '"]');
    await chat.locator('[data-view-id=prompt]').waitFor();
    await page.evaluate(() => __emit('agent:teamsent', { sessionId: 'lead', message: { id: 116, room: 'team-room', roomTitle: 'Team Reports', from: 'Lead' }, text: 'An older report.' }));
    await chat.locator('.team-traffic-fold a').first().click();
    await room.locator('[data-bus-message="116"].bus-target-message').waitFor();
    async function centred() {
      await page.waitForFunction(key => {
        const win = document.querySelector('[data-win="' + key + '"]'), log = win.querySelector('.bus-log'), row = win.querySelector('[data-bus-message="116"]');
        const a = log.getBoundingClientRect(), b = row.getBoundingClientRect();
        return log.dataset.following === 'false' && b.top >= a.top && b.bottom <= a.bottom && Math.abs((b.top + b.bottom) / 2 - (a.top + a.bottom) / 2) < 3;
      }, roomKey);
    }
    await centred();
    const next = await page.evaluate(() => __appendLinkedReport());
    await room.locator('[data-bus-message="' + next + '"]').waitFor({ state: 'attached' });
    await centred();
    const kept = await log.evaluate(el => el.scrollTop);
    await page.waitForFunction(({ key, kept }) => { const value = dinahUIState.get('dinah-scroll:' + key + ':bus-log'); return value && value.end === false && Math.abs(value.top - kept) < 2; }, { key: roomKey, kept });
    ok('the team fold opens an older message and keeps it centred through a later arrival', true);
    await t.shot('group-linked-message');
    await log.focus(); await log.press('Control+Home');
    await page.waitForFunction(key => document.querySelector('[data-win="' + key + '"] .bus-log').scrollTop === 0, roomKey);
    const afterNavigation = await page.evaluate(() => __appendLinkedReport());
    await room.locator('[data-bus-message="' + afterNavigation + '"]').waitFor({ state: 'attached' });
    ok('deliberate navigation releases the linked-message centre', await log.evaluate(el => el.scrollTop === 0));
    await room.getByRole('button', { name: 'Jump to the latest' }).click();
    const latest = await page.evaluate(() => __appendLinkedReport());
    await room.locator('[data-bus-message="' + latest + '"]').waitFor({ state: 'attached' });
    await page.waitForFunction(key => { const log = document.querySelector('[data-win="' + key + '"] .bus-log'); return log.dataset.following === 'true' && log.scrollHeight - log.scrollTop - log.clientHeight < 24; }, roomKey);
    ok('jumping to latest resumes following for subsequent messages', true);
    await t.shot('group-linked-message-latest');
  } catch (error) { await t.shot('group-linked-message-failure'); console.error(await log.evaluate(el => ({ log: el.getBoundingClientRect().toJSON(), row: el.querySelector('[data-bus-message="116"]').getBoundingClientRect().toJSON(), following: el.dataset.following }))); throw error; } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
