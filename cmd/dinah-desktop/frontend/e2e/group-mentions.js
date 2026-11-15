// A group or team chat's @ list offers its members first and then what a
// one-on-one chat offers (docs/design/group-mentions.md). A member mentioned
// is asked to answer; a reference asks no one and never changes who a
// message reaches; a member chat is listed once; Add a Chat… stays at the end
// of the members; a team with no member mentioned goes to its lead; and a sent
// message shows the member asked and the references apart, with two lines
// under it. Against a stub of the Go side that keeps the host's contract:
// routing by the explicit mentions, and shownText and references stored with
// the message (engine/group-reference-metadata).
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({
    viewport: { width: 1440, height: 900 },
    app: () => {
      const ws = 'C:/Work/docs';
      const exec = { state: 'idle', why: 'No work in progress' };
      const docs = { id: 'window-d1', chat: 'chat-docs', title: 'Docs Pass', workspace: 'docs', address: 'Docs Pass (docs)', life: 'open', window: true, exec };
      const eng = { id: 'window-e1', chat: 'chat-eng', title: 'Engine Work', workspace: 'docs', address: 'Engine Work (docs)', life: 'open', window: true, exec };
      const lead = { id: 'window-l1', chat: 'chat-lead', title: 'Team Lead', workspace: 'docs', address: 'Team Lead (docs)', life: 'open', window: true, exec, lead: true };
      const person = { id: 'paul', chat: '', title: 'Paul', workspace: '', address: '', state: 'person' };
      const room = (name, title, members, extra) => Object.assign({ name, title, lastAt: '', unread: 0, members, counts: { top: 'idle', idle: 2, working: 0, waiting: 0, stuck: 0, closed: 0, unknown: 0 } }, extra);
      window.__rooms = { 'release-review': room('release-review', 'Release Review', [docs, eng, person]), 'desktop-team': room('desktop-team', 'Desktop Team', [lead, docs, person], { team: true, lead: 'window-l1' }) };
      window.__messages = { 'release-review': [], 'desktop-team': [] };
      const legacy = { id: 7, createdAt: '2026-11-16T08:00:00Z', kind: 'text', from: 'Paul', fromPerson: true, fromSession: '', fromWorkspace: '', to: 'Release Review', room: 'release-review', roomTitle: 'Release Review', mentions: [{ id: 'window-d1', title: 'Docs Pass' }], replyTo: 0, text: 'An older message to @Docs Pass.', attachments: [], unread: false, reach: { reason: 'explicit', reach: [{ id: 'window-d1', title: 'Docs Pass' }] } };
      window.__messages['release-review'].push(legacy);
      let next = 300;
      const chat = (id, title, cwd) => ({ sessionId: id, cwd, provider: 'codex', title, open: true, working: false, queued: [] });
      return {
        State: async () => ({ workspace: ws, workspaces: [ws], recent: [ws], defaultProvider: 'codex' }),
        ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
        AgentProviders: async () => [window.__provider('codex', { default: true })],
        Sessions: async () => [chat('chat-docs', 'Docs Pass', ws), chat('chat-eng', 'Engine Work', ws), chat('chat-lead', 'Team Lead', ws), chat('chat-sheet', 'Sheet Helper', 'C:/Work/harbor')],
        OpenSessionAt: async (w, id) => chat(id, id, w),
        AppletCatalog: async () => [{ id: 'example.git', title: 'Git Tools', description: 'The repository', actions: [{ ref: 'status', title: 'Status', description: 'Show status' }] }],
        BusOverview: async () => ({ inbox: [], unread: 0, rooms: Object.values(__rooms), workspaces: [{ name: 'docs', label: 'docs', path: ws, chats: [] }], links: [], defaultHops: 10, defaultMax: 3, noHopLimit: -1 }),
        BusGroupMessages: async key => (__messages[key] || []).slice(),
        BusMarkGroupRead: async () => {},
        BusGroupReach: async (key, ids) => {
          const r = __rooms[key], members = r.members.filter(m => m.state !== 'person');
          if (ids && ids.length) return { reason: 'explicit', everyone: ids.includes('everyone'), reach: ids.includes('everyone') ? members : members.filter(m => ids.includes(m.id)) };
          if (r.team) return { reason: 'lead', reach: members.filter(m => m.id === r.lead) };
          return { reason: 'engaged', reach: [docs] };
        },
        BusSendToGroup: async (key, text, mentions, files) => {
          __calls.push(['group-send', key, text, mentions, files]);
          const r = __rooms[key], members = r.members.filter(m => m.state !== 'person');
          // The host's contract: who is reached comes from the explicit mentions alone.
          const reach = mentions.length ? { reason: 'explicit', reach: members.filter(m => mentions.includes(m.id)) } : r.team ? { reason: 'lead', reach: members.filter(m => m.id === r.lead) } : { reason: 'engaged', reach: [docs] };
          const blocks = (files || []).map(f => '\n\nThe user selected ' + f.name + ' for the message.').join('');
          const references = (files || []).filter(f => f.kind !== 'file').map(f => ({ kind: f.kind, name: f.name, token: f.token || '', workspace: f.kind === 'chat' ? (f.sessionId === 'chat-sheet' ? 'harbor' : 'docs') : '' }));
          const m = { id: next++, createdAt: '2026-11-16T09:00:00Z', kind: 'text', from: 'Paul', fromPerson: true, fromSession: '', fromWorkspace: '', to: r.title, room: key, roomTitle: r.title, mentions: members.filter(x => mentions.includes(x.id)).map(x => ({ id: x.id, title: x.title })), replyTo: 0, text: text + blocks, shownText: text, references, attachments: [], unread: false, reach };
          __messages[key].push(m);
          return m;
        }
      };
    },
    init: () => { window.__calls = []; },
  });
  const { page: p, ok } = t;
  const frame = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
  try {
    await p.locator('[data-win="workspace"]').waitFor({ state: 'attached' });
    await p.evaluate(() => { window.dinahDocuments.open = () => [{ title: 'release-notes.md', draftId: 'draft-1', application: 'Docs', where: 'docs' }]; });
    await p.evaluate(() => dinahBus.openGroup('release-review'));
    const roomWin = p.locator('.win[data-win-kind="bus-room"]').last();
    await roomWin.locator('[data-bus-message="7"]').waitFor();
    const box = roomWin.locator('textarea');
    const pop = roomWin.locator('.group-mention-pop');
    const heads = () => pop.evaluate(el => [...el.querySelectorAll('.md-section-label')].map(h => h.textContent));
    const keys = () => pop.evaluate(el => [...el.querySelectorAll('[data-mention]')].map(r => r.dataset.mention));
    const reach = () => roomWin.locator('.group-reach').textContent();
    const pickRow = async key => { await pop.locator('[data-mention="' + key + '"] a').click(); await frame(); };

    // The list: members first, then a one-on-one chat's groups.
    await box.click(); await box.type('@');
    await p.waitForFunction(() => [...document.querySelectorAll('.group-mention-pop .md-section-label')].some(h => /Other Chats/.test(h.textContent)), null, { timeout: 5000 }).catch(() => {});
    const h1 = await heads();
    ok('the @ list offers the members first, then windows, applets, documents and other chats', h1[0] === 'Members, Asked to Answer' && h1.includes('Windows') && h1.includes('Applets') && h1.includes('Open Documents') && h1.includes('Other Chats, Referred To and Not Asked'), JSON.stringify(h1));
    const k1 = await keys();
    ok('a member chat is listed once, as a member, and a chat outside the group is a reference', k1.filter(k => /Docs_Pass|window-d1/.test(k)).length === 1 && k1.some(k => /Sheet_Helper/.test(k)), JSON.stringify(k1));
    ok('Add a Chat… stays at the end of the members, before the references', k1.indexOf('add') === k1.indexOf('everyone') + 1 && k1.indexOf('add') < k1.findIndex(k => /Sheet_Helper/.test(k)), JSON.stringify(k1));
    await p.keyboard.press('Escape');

    // A member, another chat and a document: only the member changes who is reached.
    await box.fill(''); await box.type('@Docs');
    await pickRow('window-d1');
    await box.type('please check '); await box.type('@release');
    await pickRow((await keys()).find(k => /release-notes/.test(k)));
    await box.type('against '); await box.type('@Sheet');
    await pickRow((await keys()).find(k => /Sheet_Helper/.test(k)));
    await box.type('numbers.');
    await p.waitForFunction(() => /Reaches Docs Pass/.test(document.querySelector('.win[data-win-kind="bus-room"]:last-of-type .group-reach, .group-reach').textContent), null, { timeout: 3000 }).catch(() => {});
    const r1 = await reach();
    ok('the line under the box says the member is reached and the references ask no one', r1.includes('Reaches Docs Pass') && r1.includes('2 references ask no one'), r1);
    await roomWin.locator('button[type="submit"], .composer-send, [data-view-id="send"]').first().click();
    await p.waitForFunction(() => __calls.some(c => c[0] === 'group-send'));
    const sent = await p.evaluate(() => __calls.filter(c => c[0] === 'group-send').pop());
    ok('only the member is sent as a mention; the references go as references with their tokens', JSON.stringify(sent[3]) === JSON.stringify(['window-d1']) && sent[4].some(f => f.kind === 'chat' && f.sessionId === 'chat-sheet' && f.token === '@Sheet_Helper') && sent[4].some(f => f.kind === 'draft' && f.token), JSON.stringify(sent[3]) + ' ' + JSON.stringify(sent[4]));

    // The sent message: the mention, the references and the two lines.
    const card = roomWin.locator('.bus-message').last();
    await card.locator('.msg-reach').waitFor();
    const look = await card.evaluate(el => {
      const m = el.querySelector('.mention'), refs = [...el.querySelectorAll('.mention-ref')];
      const mc = m && getComputedStyle(m), rc = refs[0] && getComputedStyle(refs[0]);
      return { mention: m && m.textContent, refs: refs.map(r => r.textContent), differ: !!(mc && rc && mc.color !== rc.color && mc.fontWeight !== rc.fontWeight), glyph: refs.every(r => r.querySelector('.glyph')), blocks: /The user selected/.test(el.textContent),
        asked: (el.querySelector('[data-asked]') || {}).textContent, referred: (el.querySelector('[data-referred]') || {}).textContent };
    });
    ok('the member is a mention and each reference is marked apart, with its glyph', look.mention === '@Docs Pass' && look.refs.length === 2 && look.differ && look.glyph, JSON.stringify(look));
    ok('the reference text sent to the agents is not shown to the person', !look.blocks);
    ok('the lines under the message say who was asked and what was only referred to', look.asked.includes('Asked to answer: Docs Pass.') && look.referred.includes('Referred to, not asked: release-notes.md and Sheet Helper (harbor).'), JSON.stringify([look.asked, look.referred]));
    const old = await roomWin.locator('[data-bus-message="7"]').evaluate(el => ({ asked: (el.querySelector('[data-asked]') || {}).textContent, referred: !!el.querySelector('[data-referred]'), text: el.querySelector('.msg-text').textContent }));
    ok('a message stored before references were kept shows its text and who was asked, with no references line', old.text.includes('An older message to') && old.asked.includes('Docs Pass') && !old.referred, JSON.stringify(old));

    // A reference alone leaves the group's own rule in charge.
    await box.fill(''); await box.type('Context: '); await box.type('@Sheet');
    await pickRow((await keys()).find(k => /Sheet_Helper/.test(k)));
    await p.waitForFunction(() => /1 reference asks no one/.test(document.querySelector('.group-reach').textContent), null, { timeout: 3000 }).catch(() => {});
    ok('with only a reference, the reach is the group’s own rule', (await reach()).startsWith('Reaches Docs Pass') && (await reach()).includes('1 reference asks no one'), await reach());
    await box.fill('');

    // A team: the same list, and no mention goes to the lead.
    await p.evaluate(() => dinahBus.openGroup('desktop-team'));
    const team = p.locator('.win[data-win-kind="bus-room"]').last();
    await team.locator('textarea').click(); await team.locator('textarea').type('Status, please.');
    await p.waitForFunction(() => [...document.querySelectorAll('.group-reach')].some(l => /Reaches the lead, Team Lead/.test(l.textContent)), null, { timeout: 3000 }).catch(() => {});
    ok('a team message with no member mentioned goes to the lead', /Reaches the lead, Team Lead/.test(await team.locator('.group-reach').textContent()));
    await team.locator('textarea').fill(''); await team.locator('textarea').type('@');
    const th = await team.locator('.group-mention-pop').evaluate(el => [...el.querySelectorAll('.md-section-label')].map(h => h.textContent));
    ok('a team chat has the same @ list', th[0] === 'Members, Asked to Answer' && th.includes('Applets'), JSON.stringify(th));
    await p.keyboard.press('Escape');

    // The phone.
    await p.setViewportSize({ width: 390, height: 844 }); await frame();
    await team.locator('textarea').fill(''); await team.locator('textarea').type('@');
    const fits = await team.locator('.group-mention-pop').evaluate(el => { const r = el.getBoundingClientRect(); return r.left >= 0 && r.right <= innerWidth + 1; }).catch(() => false);
    ok('on a phone the list fits the window', fits);
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
