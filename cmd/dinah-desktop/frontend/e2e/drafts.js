// Unsent drafts outlast a restart (Paul, 28 October 2026). A chat's and a
// group's draft are saved in the UI state as they change, and a restart,
// here a new browser context reading the saved state as the app's next run
// would, shows the draft in the box again; Up brings back the last message
// sent and Down returns to the draft. While an earlier message is in the
// box, the draft Up set aside is what is saved. A sent message drops its
// draft. A group's draft keeps its mention as one unit, with the list of
// chats it mentions.
const { launch, PAGE } = require('./lib/page');
(async () => {
  const errors = [];
  let nativeState = { version: 1, values: {} };
  const saved = key => nativeState.values['dinah-draft:' + key];
  // Waits for the saved state, which the UI state writes shortly after a
  // change without being asked to.
  async function until(test, what) {
    for (let i = 0; i < 100; i++) { if (test()) return true; await new Promise(r => setTimeout(r, 50)); }
    throw new Error('Timed out waiting for ' + what + ': ' + JSON.stringify(nativeState.values));
  }
  // Each run of the app is a browser of its own, reading the state the last
  // one saved through nativeRead and nativeSave, which Node exposes before
  // the page is opened.
  async function start() {
    const t = await launch({
      viewport: { width: 1440, height: 900 },
      state: { workspace: 'C:/Test', recent: ['C:/Test'] },
      app: () => {
        window.__calls = [];
        const info = { sessionId: 'codex:first', provider: 'codex', title: 'First', cwd: 'C:/Test' };
        const msg = (id, from, text, extra) => Object.assign({ id, createdAt: '2026-10-22T08:00:00Z', kind: 'text', from, fromPerson: from === 'Paul', fromSession: from === 'Paul' ? '' : 'window-' + id, fromWorkspace: from === 'Paul' ? '' : 'docs', to: 'Release Review', room: 'release-review', roomTitle: 'Release Review', mentions: [], replyTo: 0, text, attachments: [], unread: false }, extra);
        const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
        const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', state: 'open' };
        const messages = [msg(1, 'Docs Pass', 'Hello.'), msg(2, 'Paul', 'Latest from me')];
        let next = 100;
        return {
          ReadUIState: () => nativeRead(), SaveUIState: state => nativeSave(state),
          Sessions: async () => [info],
          ChatHistory: async id => id === info.sessionId ? ['sent before'] : [],
          OpenSessionAt: async (ws, id) => ({ ...info, sessionId: id, open: true }),
          Send: async (id, text) => { __calls.push(['send', id, text]); },
          BusOverview: async () => ({ inbox: [], unread: 0, rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [docs, person] }], workspaces: [{ name: 'docs', label: 'docs', path: 'C:/Work/docs', chats: [{ id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass' }] }], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
          BusGroupMessages: async () => messages.slice(),
          BusMarkGroupRead: async () => {},
          BusSendToGroup: async (key, text, mentions) => { __calls.push(['group-send', key, text, mentions]); const m = msg(next++, 'Paul', text); messages.push(m); return m; }
        };
      },
      goto: false,
    });
    await t.context.exposeFunction('nativeRead', async () => JSON.parse(JSON.stringify(nativeState)));
    await t.context.exposeFunction('nativeSave', async state => { nativeState = state; });
    await t.page.goto(PAGE);
    await t.waitReady();
    return t;
  }
  // The run's browser closes as the app would, its errors kept for the end.
  async function stop(t) { errors.push(...t.errors); await t.close(); }
  // The chat's window, opened if the restored layout did not bring it back.
  async function chatBox(page) {
    const key = await page.evaluate(() => dinahWorkbench.open({ kind: 'chat-session', workspace: 'C:/Test', path: 'codex:first' }));
    const box = page.locator('[data-win="' + key + '"] [data-view-id="prompt"]');
    await box.waitFor();
    return box;
  }
  async function groupBox(page) {
    await page.evaluate(() => dinahBus.openGroup('release-review'));
    const room = page.locator('.win[data-win-kind="bus-room"]');
    await room.locator('[data-bus-message="2"]').waitFor();
    return room.locator('textarea');
  }
  let t;
  try {
    // A chat.
    t = await start();
    let page = t.page, ok = t.ok;
    let box = await chatBox(page);
    await box.focus();
    await box.fill('half a thought');
    await until(() => saved('chat:codex:first') && saved('chat:codex:first').text === 'half a thought', 'the chat\'s draft');
    ok('a chat\'s draft is saved as it is typed, in the UI state', true);
    await stop(t);

    t = await start(); page = t.page; ok = t.ok;
    box = await chatBox(page);
    await page.waitForFunction(() => document.querySelector('.win[data-win-kind="chat-session"] [data-view-id="prompt"]').value === 'half a thought');
    ok('after a restart the chat\'s box shows its draft', await box.inputValue() === 'half a thought');
    await box.focus();
    await box.evaluate(el => el.setSelectionRange(0, 0));
    await box.press('ArrowUp');
    ok('Up brings back the last message sent', await box.inputValue() === 'sent before');
    // The UI state writes 150 ms after a change; this waits past that.
    await page.waitForTimeout(400);
    await page.evaluate(() => dinahUIState.flush());
    ok('while an earlier message is in the box, the draft set aside stays saved', !!saved('chat:codex:first') && saved('chat:codex:first').text === 'half a thought', JSON.stringify(saved('chat:codex:first')));
    await box.press('ArrowDown');
    ok('Down returns to the draft', await box.inputValue() === 'half a thought');
    await box.press('Control+Enter');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'send'));
    await until(() => !saved('chat:codex:first'), 'the chat\'s draft to go');
    ok('a sent message drops the chat\'s saved draft', !saved('chat:codex:first'));
    await stop(t);

    t = await start(); page = t.page; ok = t.ok;
    box = await chatBox(page);
    await page.waitForTimeout(300);
    ok('after the next restart the chat\'s box is empty', await box.inputValue() === '');

    // A group.
    box = await groupBox(page);
    await box.fill('Unfinished, ');
    await box.press('End');
    await box.pressSequentially('@Do');
    await page.locator('.group-mention-pop').waitFor();
    await box.press('Enter');
    ok('the mention is chosen', await box.inputValue() === 'Unfinished, @Docs Pass ');
    await until(() => saved('group:release-review') && saved('group:release-review').text === 'Unfinished, @Docs Pass ', 'the group\'s draft');
    ok('a group\'s draft is saved with the chats it mentions', JSON.stringify(saved('group:release-review').mentioned) === JSON.stringify([{ id: 'window-d1', title: 'Docs Pass' }]), JSON.stringify(saved('group:release-review')));
    await stop(t);

    t = await start(); page = t.page; ok = t.ok;
    box = await groupBox(page);
    await page.waitForFunction(() => document.querySelector('.win[data-win-kind="bus-room"] textarea').value !== '');
    ok('after a restart the group\'s box shows its draft', await box.inputValue() === 'Unfinished, @Docs Pass ');
    await box.focus();
    await box.evaluate(el => el.setSelectionRange(0, 0));
    await box.press('ArrowUp');
    ok('Up brings back the person\'s last message in the group', await box.inputValue() === 'Latest from me');
    await box.press('ArrowDown');
    ok('Down returns to the group\'s draft', await box.inputValue() === 'Unfinished, @Docs Pass ');
    await box.press('Control+Enter');
    await page.waitForFunction(() => __calls.some(c => c[0] === 'group-send'));
    const sent = await page.evaluate(() => __calls.find(c => c[0] === 'group-send'));
    ok('the restored draft is sent with the chats it mentions', JSON.stringify(sent) === JSON.stringify(['group-send', 'release-review', 'Unfinished, @Docs Pass', ['window-d1']]), JSON.stringify(sent));
    await until(() => !saved('group:release-review'), 'the group\'s draft to go');
    ok('a sent message drops the group\'s saved draft', !saved('group:release-review'));
    await box.pressSequentially('Ask @Do');
    await page.locator('.group-mention-pop').waitFor();
    await box.press('Enter');
    await until(() => saved('group:release-review') && saved('group:release-review').text === 'Ask @Docs Pass ', 'the second group draft');
    await stop(t);

    t = await start(); page = t.page; ok = t.ok;
    box = await groupBox(page);
    await page.waitForFunction(() => document.querySelector('.win[data-win-kind="bus-room"] textarea').value !== '');
    await box.focus();
    await box.evaluate(el => el.setSelectionRange(el.value.length - 1, el.value.length - 1));
    await box.press('Backspace');
    ok('a restored mention is still one unit that Backspace takes out whole', await box.inputValue() === 'Ask  ', JSON.stringify(await box.inputValue()));

    // A deleted group's draft goes with it.
    await page.evaluate(() => { window.go.main.App.BusDeleteGroup = async key => { __calls.push(['delete', key]); }; });
    await page.locator('[data-menubar] .menubar-title').filter({ hasText: /^Actions$/ }).click();
    await page.locator('#menubar-panel').getByText('Delete Group…').click();
    await page.locator('#ask-yes').click();
    await page.waitForFunction(() => __calls.some(c => c[0] === 'delete'));
    await until(() => !saved('group:release-review'), 'the deleted group\'s draft to go');
    ok('deleting a group drops its saved draft', true);
    ok('no page errors', errors.length === 0 && t.errors.length === 0, errors.concat(t.errors).join(' | '));
    await stop(t);
    t = null;
  } catch (e) {
    console.error('Error: ' + (e && e.stack || e)); process.exitCode = 1;
    if (t) await t.close();
  }
})();
