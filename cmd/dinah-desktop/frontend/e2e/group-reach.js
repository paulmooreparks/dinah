// Who a group message will wake, shown before it is sent
// (docs/plans/group-attention.md). The engine decides, from the mentions
// the box has bound; the page shows its answer. The placeholder says it
// for an empty box: everyone when no member is engaged, the engaged members
// when some are. A line under the box says it as the person types, the
// mentions they bind included, and follows an expiry and a change of the
// engaged set without a key pressed. The line is text in a row of its own
// height, so nothing moves, and its live region speaks only when the reach
// has changed and typing has paused.
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1280, height: 900 },
    app: () => {
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } };
      const review = { id: 'window-r1', chat: 'chat-review', title: 'Review', workspace: 'docs', address: 'Review (docs)', life: 'open', window: true, exec: { state: 'idle', why: 'No work in progress' } };
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const byId = { 'window-d1': docs, 'window-r1': review };
      window.__engaged = []; window.__expires = ''; window.__asked = [];
      return {
        State: async () => ({ workspace: 'C:/Work/docs', workspaces: ['C:/Work/docs'], recent: ['C:/Work/docs'] }),
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: [{ name: 'release-review', title: 'Release Review', lastAt: '', unread: 0, members: [docs, review, person], counts: { top: 'idle', idle: 2, working: 0, waiting: 0, stuck: 0, closed: 0, unknown: 0 } }], workspaces: [], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusGroupMessages: async () => window.__messages || [], BusMarkGroupRead: async () => {},
        BusGroupAttentionSettings: async () => window.__attention || { limit: 3, minutes: 30 },
        SetBusGroupAttentionSettings: async (limit, minutes) => { window.__attention = { limit, minutes }; window.__attentionSet = (window.__attentionSet || []).concat([[limit, minutes]]); return window.__attention; },
        // The send stores who it reached; __sendReach stands in for an
        // engagement that ran out between the line and the send.
        BusSendToGroup: async (key, text, mentions) => {
          const m = { id: 500 + (window.__messages || []).length, from: 'Paul', fromPerson: true, kind: 'text', text, mentions: [], attachments: [], createdAt: new Date().toISOString(), room: key, roomTitle: 'Release Review', reach: window.__sendReach };
          window.__messages = (window.__messages || []).concat([m]);
          return m;
        },
        // The engine's answer, as the plan rules it: explicit mentions reach
        // those, else the engaged members, else everyone.
        // In the engine's shape: reach lists the members woken, everyone is a
        // flag, version counts the group's changes.
        BusGroupReach: async (group, ids) => {
          window.__asked.push(ids.slice());
          const all = [docs, review].map(m => ({ id: m.id, title: m.title }));
          const engaged = window.__engaged.map(id => ({ id, title: byId[id].title, engagedAt: '', expires: window.__expires }));
          const base = { group, version: window.__version || 1, engaged, settings: { limit: 3, minutes: 30 } };
          if (ids.indexOf('everyone') >= 0) return Object.assign(base, { reach: all, everyone: true, reason: 'explicit', expires: '' });
          if (ids.length) return Object.assign(base, { reach: ids.map(id => ({ id, title: byId[id].title })), everyone: false, reason: 'explicit', expires: '' });
          if (engaged.length) return Object.assign(base, { reach: engaged.map(m => ({ id: m.id, title: m.title })), everyone: false, reason: 'engaged', expires: window.__expires });
          // A team channel with no one engaged goes to its lead.
          if (window.__team) return Object.assign(base, { reach: [{ id: 'window-r1', title: 'Review' }], everyone: false, reason: 'lead', expires: '' });
          return Object.assign(base, { reach: all, everyone: true, reason: 'everyone', expires: '' });
        }
      };
    },
  });
  const { page: p, ok } = t;
  try {
    await p.locator('[data-win="workspace"]').waitFor({ state: 'attached' });
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    const room = p.locator('.win[data-win-kind="bus-room"]');
    const box = room.locator('.group-composer textarea'), line = room.locator('.group-reach');
    await box.waitFor();
    const lineIs = words => p.waitForFunction(w => { const l = document.querySelector('.win[data-win-kind="bus-room"] .group-reach'); return l && l.textContent === w; }, words);
    await lineIs('Reaches everyone in the group');
    ok('with no member engaged, the placeholder offers everyone', await box.getAttribute('placeholder') === 'Type @ to choose who answers, or send to everyone', await box.getAttribute('placeholder'));
    ok('and the line says everyone', await line.textContent() === 'Reaches everyone in the group');
    const before = await room.locator('.group-composer').evaluate(el => { const r = el.getBoundingClientRect(); return [Math.round(r.top), Math.round(r.height)]; });
    // An engaged member, with an expiry; a change of the set reaches the page as an event.
    await p.evaluate(() => { window.__engaged = ['window-d1']; window.__expires = new Date(Date.now() + 2500).toISOString(); window.__version = 2; __emit('bus:changed', { counter: 2 }); });
    await lineIs('Reaches Docs Pass');
    ok('a change of the engaged set redraws the line without a key pressed', true);
    ok('with a member engaged, the placeholder names who replies go to', await box.getAttribute('placeholder') === 'Reaches Docs Pass. Type @ to choose others', await box.getAttribute('placeholder'));
    // Typing a mention reaches that member instead. The announcement for the
    // engaged member is let settle first, so only what typing causes is counted.
    await p.waitForTimeout(900);
    const spoken = await p.evaluate(() => { const r = document.querySelector('.win[data-win-kind="bus-room"] .group-composer [aria-live]'); window.__said = []; new MutationObserver(() => __said.push(r.textContent)).observe(r, { childList: true, characterData: true, subtree: true }); return !!r; });
    ok('the reach has a polite live region of its own', spoken);
    await box.pressSequentially('@Review please look at this', { delay: 20 });
    await lineIs('Reaches Review');
    ok('a typed mention reaches that member, as the engine says', JSON.stringify(await p.evaluate(() => __asked[__asked.length - 1])) === JSON.stringify(['window-r1']));
    await p.waitForTimeout(900);
    const said = await p.evaluate(() => __said.slice());
    ok('the live region speaks once for the new reach, not on every key', said.length === 1 && said[0] === 'Reaches Review', JSON.stringify(said));
    // Clearing the box goes back to the engaged member, until the expiry.
    await box.fill('');
    await lineIs('Reaches Docs Pass');
    await p.evaluate(() => { window.__engaged = []; });
    await lineIs('Reaches everyone in the group');
    ok('when the nearest expiry passes, the line follows without a key pressed', true);
    const after = await room.locator('.group-composer').evaluate(el => { const r = el.getBoundingClientRect(); return [Math.round(r.top), Math.round(r.height)]; });
    ok('the composer does not move or change size as the line changes', JSON.stringify(before) === JSON.stringify(after), JSON.stringify({ before, after }));
    // A team channel with no one engaged goes to its lead, and says so.
    await p.evaluate(() => { window.__team = true; __emit('bus:changed', { counter: 4 }); });
    await lineIs('Reaches the lead, Review');
    ok('in a team channel with no one engaged, the placeholder names the lead', await box.getAttribute('placeholder') === 'Goes to the lead, Review. Type @ to choose who answers', await box.getAttribute('placeholder'));
    await p.evaluate(() => { window.__team = false; });
    // The line says Docs Pass; the engine's send finds the engagement gone
    // and reaches everyone, so a line under the sent message says so.
    await p.evaluate(() => { window.__engaged = ['window-d1']; window.__expires = ''; __emit('bus:changed', { counter: 3 }); });
    await lineIs('Reaches Docs Pass');
    await p.evaluate(() => { window.__sendReach = { reach: [{ id: 'window-d1', title: 'Docs Pass' }, { id: 'window-r1', title: 'Review' }], everyone: true, reason: 'everyone' }; });
    await box.fill('Any news?');
    await box.press('Control+Enter');
    const note = room.locator('[data-bus-message="500"] .group-reach-note');
    await note.waitFor();
    ok('when the send reached others than the line said, the sent message says who it reached', /^Sent to everyone in the group, not as shown/.test(await note.textContent()), await note.textContent());
    // A send that reached what was shown adds nothing.
    await p.evaluate(() => { window.__sendReach = { reach: [{ id: 'window-d1', title: 'Docs Pass' }], everyone: false, reason: 'engaged' }; });
    await box.fill('And now?');
    await box.press('Control+Enter');
    await room.locator('[data-bus-message="501"]').waitFor();
    await p.waitForTimeout(200);
    ok('a send that reached whom the line said adds no note', await room.locator('[data-bus-message="501"] .group-reach-note').count() === 0);
    await t.shot('group-reach');
    // Settings keeps how many members a message follows and for how long,
    // and says a change applies at once.
    await p.evaluate(() => dinahWorkbench.open('settings'));
    const settings = p.locator('#settings');
    await settings.getByRole('tab', { name: 'Chat', exact: true }).click();
    const limit = settings.locator('#set-attention-limit'), minutes = settings.locator('#set-attention-minutes');
    await p.waitForFunction(() => document.getElementById('set-attention-limit').value === '3');
    ok('Settings shows the engine\'s limit and minutes', await limit.inputValue() === '3' && await minutes.inputValue() === '30');
    await minutes.fill('45'); await minutes.press('Tab');
    await p.waitForFunction(() => (window.__attentionSet || []).length === 1);
    ok('changing the minutes sets both through the engine', JSON.stringify(await p.evaluate(() => __attentionSet[0])) === JSON.stringify([3, 45]));
    ok('the help says a change applies at once to conversations under way', /applies at once to every group, including conversations already under way/.test(await settings.locator('#set-group-attention').locator('..').textContent()));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
