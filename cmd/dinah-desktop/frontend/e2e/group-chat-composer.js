// A group chat's window has a chat's box and draws its messages as a chat
// does (docs/plans/group-chats.md, The Group Window Matches a Chat). The
// box grows with its text up to the chat's limit, sends with the key chosen
// in Settings › Chat, brings back the person's earlier messages in the group
// with Up and Down, mentions included, and keeps what was typed while the
// window is closed. A message is Markdown made safe, with its links and
// diagrams, its time in the regional format, Copy and Quote, and its tools
// again at the foot of a long one. The list stays at its end while the
// reader is there and keeps its place otherwise.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      const msg = (id, from, text, extra) => Object.assign({ id, createdAt: '2026-10-22T08:00:00Z', kind: 'text', from, fromPerson: from === 'Paul', fromSession: from === 'Paul' ? '' : 'window-' + id, fromWorkspace: from === 'Paul' ? '' : 'docs', to: 'Release Review', room: 'release-review', roomTitle: 'Release Review', mentions: [], replyTo: 0, text, attachments: [], unread: false }, extra);
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } };
      const fillers = Array.from({ length: 24 }, (_, i) => msg(100 + i, 'Docs Pass', 'Filler message ' + i + '.'));
      const long = Array.from({ length: 30 }, (_, i) => 'Line ' + i + ' of a long report.').join('\n');
      const messages = fillers.concat([
        msg(4, 'Docs Pass', '**Bold** and `code`\n\n- one\n- two\n\nSee https://example.com/page and [the notes](https://example.com/notes).\n\n```d2\na -> b\n```'),
        msg(5, 'Docs Pass', '<img src=x onerror="window.__pwned=1"><script>window.__pwned=2</script> [click](javascript:window.__pwned=3) <a href="javascript:window.__pwned=4">x</a>'),
        msg(6, 'Paul', 'Earlier from me @Docs Pass', { mentions: [{ id: 'window-d1', title: 'Docs Pass' }] }),
        msg(7, 'Paul', 'Latest from me'),
        msg(8, 'Docs Pass', long)]);
      window.__messages = messages;
      let sendKey = 'enter', next = 200;
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'], promptSendKey: sendKey }),
        SetPromptSendKey: async v => { sendKey = v; },
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [docs, person], counts: { top: 'idle', idle: 1, working: 0, waiting: 0, stuck: 0, closed: 0, unknown: 0 } }], workspaces: [{ name: 'docs', label: 'docs', path: 'C:/Work/docs', chats: [{ id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass' }] }], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusGroupMessages: async () => messages.slice(),
        BusMarkGroupRead: async () => {},
        BusSendToGroup: async (key, text, mentions) => {
          __calls.push(['group-send', key, text, mentions]);
          const m = msg(next++, 'Paul', text, { mentions: mentions.filter(id => id === 'window-d1').map(id => ({ id, title: 'Docs Pass' })) });
          messages.push(m);
          return m;
        }
      };
    },
    // What was copied and what the browser was asked to open are kept in
    // lists of their own, as the checks read them.
    init: () => {
      window.__copied = []; window.__opened = []; window.__calls = [];
      window.runtime.ClipboardSetText = async t => { __copied.push(t); };
      window.runtime.BrowserOpenURL = u => { __opened.push(u); };
    },
  });
  const { page: p, ok } = t;
  try {
    await p.locator('[data-win="workspace"]').waitFor({ state: 'attached' });
    // Diagrams are drawn by the chat's diagram engine; the calls are recorded.
    await p.evaluate(() => { window.__diagrams = []; const draw = dinahDiagrams.draw; dinahDiagrams.draw = (pre, lang, src, alive) => { __diagrams.push([lang, src]); return draw(pre, lang, src, alive); }; });
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    const room = p.locator('.win[data-win-kind="bus-room"]');
    await room.locator('[data-bus-message="8"]').waitFor();
    await room.locator('.bus-log').evaluate(el => {
      const trace = el.__tailTrace = [];
      const record = kind => trace.push({ kind, top: el.scrollTop, height: el.clientHeight, extent: el.scrollHeight, following: el.dataset.following, restore: dinahUIState.restorePending(el) });
      record('before arrival');
      el.addEventListener('scroll', () => record('scroll'));
      new MutationObserver(() => record('following')).observe(el, { attributes: true, attributeFilter: ['data-following'] });
      new ResizeObserver(() => record('resize')).observe(el);
    });
    const box = room.locator('textarea');
    const sends = () => p.evaluate(() => __calls.filter(c => c[0] === 'group-send'));

    // The messages: Markdown, made safe, with links, diagrams and times.
    const rendered = room.locator('[data-bus-message="4"] .msg-text');
    ok('a group message is drawn from its Markdown, not shown as its marks', await rendered.locator('strong').innerText() === 'Bold' && await rendered.locator('code').first().innerText() === 'code' &&
      await rendered.locator('li').count() === 2 && !(await rendered.innerText()).includes('**'), await rendered.innerHTML());
    ok('its addresses and Markdown links are link chips', await rendered.locator('a.smart-chip[data-chip-url="https://example.com/page"]').count() === 1 && await rendered.locator('a.smart-chip[data-chip-url="https://example.com/notes"]', { hasText: 'the notes' }).count() === 1);
    await rendered.locator('a.smart-chip[data-chip-url="https://example.com/notes"]').click();
    ok('a link chip opens in the browser', JSON.stringify(await p.evaluate(() => __opened)) === JSON.stringify(['https://example.com/notes']));
    ok('a fenced d2 block is given to the diagram engine', await p.evaluate(() => __diagrams.some(d => d[0] === 'd2' && d[1].includes('a -> b'))));
    const stamp = room.locator('[data-bus-message="4"] time.bus-message-when');
    ok('a message\'s time is written in the regional format and kept current', await stamp.getAttribute('data-regional') === 'dayTime' && !!(await stamp.getAttribute('datetime')) && await stamp.innerText() === await p.evaluate(() => dinahRegional.format('2026-10-22T08:00:00Z', 'dayTime')));
    const hostile = room.locator('[data-bus-message="5"]');
    await p.waitForTimeout(200);
    ok('a hostile message is neutralised: no element, handler or script address survives, and nothing ran',
      await hostile.locator('img, script, [onerror]').count() === 0 && await hostile.locator('a[href^="javascript" i]').count() === 0 && await p.evaluate(() => window.__pwned === undefined) &&
      (await hostile.locator('.msg-text').innerText()).includes('<img src=x'), await hostile.locator('.msg-text').innerHTML());
    ok('a mention in a message is still marked', await room.locator('[data-bus-message="6"] .mention').innerText() === '@Docs Pass');
    ok('a long message repeats its tools at its foot, and a short one does not', await room.locator('[data-bus-message="8"] .msg-foot').isVisible() && await room.locator('[data-bus-message="4"] .msg-foot').isHidden());
    ok('the person\'s message offers Edit and Copy, another\'s Quote and Copy',
      await room.locator('[data-bus-message="7"] .msg-head [data-act="edit"]').count() === 1 && await room.locator('[data-bus-message="4"] .msg-head [data-act="quote"]').count() === 1 &&
      await room.locator('[data-bus-message="4"] .msg-head .msg-copy').count() === 1);

    // Copy and Quote.
    await room.locator('[data-bus-message="4"]').hover();
    await room.locator('[data-bus-message="4"] .msg-head .msg-copy').click();
    ok('Copy copies the message\'s Markdown as written', await p.evaluate(() => __copied[0]) === await p.evaluate(() => __messages.find(m => m.id === 4).text));
    await room.locator('[data-bus-message="8"] .msg-foot .msg-copy').click();
    ok('Copy at the foot of a long message copies it whole', await p.evaluate(() => __copied[1]) === await p.evaluate(() => __messages.find(m => m.id === 8).text));
    await room.locator('[data-bus-message="4"] .msg-head [data-act="quote"]').click();
    ok('Quote puts the message in the group\'s box as a quotation', (await box.inputValue()).startsWith('> **Bold** and `code`\n> \n> - one') && await box.evaluate(el => document.activeElement === el));
    await box.fill('');

    // The box grows with its text, to the chat's limit.
    const height = () => box.evaluate(el => el.getBoundingClientRect().height);
    const empty = await height();
    await box.fill(Array.from({ length: 6 }, (_, i) => 'line ' + i).join('\n'));
    const six = await height();
    await box.fill(Array.from({ length: 80 }, (_, i) => 'line ' + i).join('\n'));
    const many = await height();
    // The chat's prompt is a .composer-input too; a probe with its classes, inside a chat,
    // gives the limit a chat's box has.
    const limit = await p.evaluate(() => { const w = document.createElement('div'); w.className = 'chat-instance'; const t = document.createElement('textarea'); t.className = 'form-textarea composer-input'; w.appendChild(t); document.body.appendChild(w); const h = parseFloat(getComputedStyle(t).maxHeight); w.remove(); return h; });
    ok('the box grows with its text', six > empty + 40, empty + ' then ' + six);
    ok('the box stops growing at the chat\'s limit and scrolls', Math.abs(many - limit) <= 2 && many > six && await box.evaluate(el => el.scrollHeight > el.clientHeight), many + ' against ' + limit);
    await box.fill('');

    // Up and Down bring back the person's own messages in this group.
    await box.focus();
    await box.press('ArrowUp');
    ok('Up brings back the person\'s latest message in the group', await box.inputValue() === 'Latest from me');
    await box.press('ArrowUp');
    ok('Up again brings back the one before', await box.inputValue() === 'Earlier from me @Docs Pass');
    await box.press('ArrowDown');
    await box.press('ArrowDown');
    ok('Down goes forward again, back to what was being typed', await box.inputValue() === '');
    await box.fill('first line\nsecond line');
    await box.evaluate(el => el.setSelectionRange(el.value.length, el.value.length));
    await box.press('ArrowUp');
    ok('Up on the last of several lines moves the caret instead', await box.inputValue() === 'first line\nsecond line');
    await box.fill('');
    await box.press('ArrowUp'); await box.press('ArrowUp');
    await box.press('Control+Enter');
    await p.waitForFunction(() => __calls.length === 1);
    ok('a message brought back keeps the chats it mentioned', JSON.stringify((await sends())[0]) === JSON.stringify(['group-send', 'release-review', 'Earlier from me @Docs Pass', ['window-d1']]), JSON.stringify(await sends()));

    // The send key from Settings › Chat.
    await box.fill('Sent with Enter');
    await box.press('Enter');
    await p.waitForFunction(() => __calls.length === 2);
    ok('Enter sends when the setting says Enter sends', (await sends())[1][2] === 'Sent with Enter' && await box.inputValue() === '');
    await box.fill('Line one');
    await box.press('Shift+Enter');
    ok('Shift+Enter is a new line then', await box.inputValue() === 'Line one\n');
    await box.fill('');
    await p.evaluate(() => { const s = document.getElementById('set-send-key'); s.value = 'ctrl-enter'; s.dispatchEvent(new Event('change')); });
    await p.waitForFunction(() => dinahComposer.sendKey() === 'ctrl-enter');
    await box.fill('Two lines');
    await box.press('Enter');
    ok('Enter is a new line when the setting says Ctrl+Enter sends', await box.inputValue() === 'Two lines\n' && (await sends()).length === 2);
    await box.press('Control+Enter');
    await p.waitForFunction(() => __calls.length === 3);
    ok('Ctrl+Enter sends then', (await sends())[2][2] === 'Two lines');
    const sendButton = room.getByRole('button', { name: 'Send', exact: true });
    await box.fill('For the tooltip');
    await sendButton.hover();
    ok('Send\'s tooltip says the send key in force', await sendButton.getAttribute('data-tooltip') === 'Send (Ctrl+Enter)');
    await box.fill('');

    // The list stays at its end while the reader is there, and keeps its
    // place when the reader has scrolled back.
    const log = room.locator('.bus-log');
    const atEnd = () => log.evaluate(el => el.scrollHeight - el.scrollTop - el.clientHeight < 24);
    await p.waitForFunction(() => { const el = document.querySelector('.win[data-win-kind="bus-room"] .bus-log'); return el.scrollHeight - el.scrollTop - el.clientHeight < 24; });
    ok('after sending, the list is at its end', await atEnd());

    await p.evaluate(() => { __messages.push(Object.assign({}, __messages[0], { id: 300, text: 'New while at the end.' })); __emit('bus:changed', { counter: 1 }); });
    await room.locator('[data-bus-message="300"]').waitFor();
    await p.waitForTimeout(150);
    ok('a message arriving while the reader is at the end keeps the end in view', await atEnd(), JSON.stringify(await log.evaluate(el => ({ top: el.scrollTop, height: el.clientHeight, extent: el.scrollHeight, following: el.dataset.following, divider: el.querySelector('.transcript-unread-divider')?.textContent, trace: el.__tailTrace.slice(-40) }))));
    await room.locator('[data-bus-message="104"] .msg-text p').evaluate(el => { el.__kept = true; });
    await log.focus(); await log.press('Control+Home');
    await p.waitForTimeout(150);
    await p.evaluate(() => { __messages.push(Object.assign({}, __messages[0], { id: 301, text: 'New while reading back.' })); __emit('bus:changed', { counter: 2 }); });
    await room.locator('[data-bus-message="301"]').waitFor();
    await p.waitForTimeout(150);
    ok('a message arriving while the reader has scrolled back keeps the reader\'s place', await log.evaluate(el => el.scrollTop) === 0);
    ok('messages already drawn are kept, not drawn again', await room.locator('[data-bus-message="104"] .msg-text p').evaluate(el => el.__kept === true));

    // The draft, with its mention, is kept while the window is closed.
    await box.fill('Unfinished, ');
    await box.press('End');
    await box.pressSequentially('@Do');
    await room.locator('.group-mention-pop').waitFor();
    await box.press('Enter');
    ok('the mention is chosen as before', await box.inputValue() === 'Unfinished, @Docs Pass ');
    const key = await room.getAttribute('data-win');
    await p.evaluate(k => pudlWindows.close(k), key);
    await room.waitFor({ state: 'detached' });
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    await room.locator('[data-bus-message="8"]').waitFor();
    ok('reopening the group brings back what was typed', await box.inputValue() === 'Unfinished, @Docs Pass ');
    await box.focus();
    await box.press('Control+Enter');
    await p.waitForFunction(() => __calls.length === 4);
    ok('the kept draft still mentions the chat it mentioned', JSON.stringify((await sends())[3]) === JSON.stringify(['group-send', 'release-review', 'Unfinished, @Docs Pass', ['window-d1']]), JSON.stringify((await sends())[3]));
    await p.evaluate(k => pudlWindows.close(k), await room.getAttribute('data-win'));
    await room.waitFor({ state: 'detached' });
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    await room.locator('[data-bus-message="8"]').waitFor();
    ok('a sent message leaves no draft behind', await box.inputValue() === '');

    // Edit puts one of the person's messages back, with its mentions.
    await room.locator('[data-bus-message="6"] .msg-head [data-act="edit"]').click();
    ok('Edit puts the person\'s message back in the box', await box.inputValue() === 'Earlier from me @Docs Pass');
    await box.press('Backspace');
    ok('a mention Edit put back is still one unit', await box.inputValue() === 'Earlier from me ', JSON.stringify(await box.inputValue()));
    await t.shot('group-chat-composer');
  } finally { await t.close(); }
})();
