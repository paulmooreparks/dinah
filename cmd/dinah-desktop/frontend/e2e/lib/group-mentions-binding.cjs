'use strict';
// The receiving binding for Quality's frozen group mentions checks
// (docs/review/group-mentions-independent-freeze-2026-11-16.md on
// quality/group-mentions). It starts the tagged real App host
// (group_mentions_receiving_test.go), opens the shipped page in headless
// Edge, and forwards the page's group and chat bridge calls (BusOverview,
// BusGroupReach, BusSendToGroup, BusGroupMessages, BusMarkGroupRead, Sessions,
// SavedChats, AppletCatalog) to that host with their original arguments, returning its
// actual results. It drives the page with pointer and keyboard input and reports
// what it observed; recipients come from the host's records, never the preview.
//
// Seeded on the page side, and recorded in each case's evidence (setup): the
// window's other bridge calls are the suites' ordinary stub, and one open
// document is given to the page's document list, since the host has no Editor.
//
//   QUALITY_GROUP_MENTIONS_BINARY=<group-mentions-host.exe>
//   QUALITY_GROUP_MENTIONS_BINDING=<this file>
//   pwsh scripts/with-test-lock.ps1 node --test --test-concurrency=1 internal/qualitygroupmentions/acceptance.test.cjs
const fs = require('fs'), os = require('os'), path = require('path'), crypto = require('crypto');
const { spawn, execFileSync } = require('child_process');
const { launch } = require('./page');

const FORWARDED = ['BusOverview', 'BusGroupReach', 'BusSendToGroup', 'BusGroupMessages', 'BusMarkGroupRead', 'Sessions', 'SavedChats', 'AppletCatalog'];
const SECTION_KIND = { 'Members, asked to answer': 'member', 'Windows': 'window', 'Applets': 'applet', 'Open Documents': 'document', 'Other Chats, referred to and not asked': 'chat' };

function sourceCommit() {
  try { return execFileSync('git', ['rev-parse', 'HEAD'], { cwd: __dirname, windowsHide: true }).toString().trim(); } catch (e) { return ''; }
}
async function startHost(artifact) {
  const binary = process.env.QUALITY_GROUP_MENTIONS_BINARY;
  if (!binary) throw new Error('QUALITY_GROUP_MENTIONS_BINARY must name the tagged Go test binary');
  const ready = path.join(artifact, 'ready.txt'), home = path.join(artifact, 'home');
  fs.mkdirSync(home, { recursive: true });
  const out = fs.openSync(path.join(artifact, 'host.log'), 'w');
  const child = spawn(binary, ['-test.run=^TestGroupMentionsAcceptanceServer$', '-test.timeout=300s', '-test.v'], {
    windowsHide: true, stdio: ['ignore', out, out],
    env: Object.assign({}, process.env, { QUALITY_GROUP_MENTIONS_READY: ready, QUALITY_GROUP_MENTIONS_ARTIFACT: artifact, DINAH_HOME: home, HOME: home, USERPROFILE: home })
  });
  fs.closeSync(out);
  const exited = new Promise((resolve, reject) => { child.on('error', reject); child.on('exit', (code, signal) => resolve({ code, signal })); });
  let url;
  try {
    url = await new Promise((resolve, reject) => {
      const timer = setTimeout(() => finish(new Error('The host did not publish its address; evidence ' + artifact)), 60000);
      const poll = setInterval(read, 100);
      function finish(err, value) { clearTimeout(timer); clearInterval(poll); err ? reject(err) : resolve(value); }
      function read() { try { const v = fs.readFileSync(ready, 'utf8'); if (v) finish(null, v); } catch (e) {} }
      child.once('exit', code => finish(new Error('The host exited ' + code + '; evidence ' + artifact)));
    });
  } catch (error) { child.kill(); await exited; throw error; }
  async function call(method, args) {
    const r = await fetch(url, { method: 'POST', body: JSON.stringify({ method, args: args || [] }), signal: AbortSignal.timeout(30000) });
    if (!r.ok) throw new Error(method + ': ' + r.status + ' ' + await r.text());
    return r.json();
  }
  /* Teardown fails when the host does not close cleanly: a refused close, a
     nonzero exit, or the watchdog. A host the watchdog kills is awaited until
     it has exited, so nothing outlives the case. */
  async function stop() {
    let closeError = null;
    try { await call('close'); } catch (e) { closeError = e; }
    let timer, watchdog = false;
    const result = await Promise.race([exited, new Promise(resolve => { timer = setTimeout(() => { watchdog = true; resolve(null); }, 15000); })]);
    clearTimeout(timer);
    if (watchdog) {
      child.kill();
      const killed = await exited;
      throw new Error('the host did not exit within 15 seconds and was killed (' + JSON.stringify(killed) + '); evidence ' + artifact);
    }
    if (result.code !== 0) throw new Error('the host exited ' + JSON.stringify(result) + '; evidence ' + artifact);
    if (closeError) throw new Error('the host refused close: ' + closeError.message + '; evidence ' + artifact);
    return result;
  }
  return { call, stop, binaryHash: crypto.createHash('sha256').update(fs.readFileSync(binary)).digest('hex') };
}

async function open({ scenario, theme, width }) {
  const artifact = fs.mkdtempSync(path.join(os.tmpdir(), 'group-mentions-receiving-' + scenario + '-' + theme + '-' + width + '-'));
  const host = await startHost(artifact);
  const fixture = (await host.call('fixture')).result;
  const sends = [], previews = [], setup = ['the window’s other bridge calls are the browser suites’ stub', 'one draft opened in the host’s Editor, release-notes.md, is given to the page’s document list'];
  const t = await launch({ goto: false, viewport: { width, height: 900 }, args: { forwarded: FORWARDED }, app: (opts) => {
    const ws = 'C:/Fixture';
    const own = {
      State: async () => ({ workspace: ws, workspaces: [ws], recent: [ws], defaultProvider: 'codex' }),
      ReadUIState: async () => ({ version: 1, values: { 'dinah-workbench-layout': '?open=workspace' } }), SaveUIState: async () => {},
      AgentProviders: async () => [window.__provider('codex', { default: true })],
      OpenSessionAt: async (w, id) => ({ sessionId: id, cwd: w, open: true })
    };
    /* The group and chat calls go to the real App host, with their original
       arguments, and return its actual results. */
    opts.forwarded.forEach(name => { own[name] = async (...args) => { const r = await window.__host(name, args); if (r && r.error) throw new Error(r.error); return r ? r.result : undefined; }; });
    return own;
  } });
  const p = t.page, errors = [];
  p.on('pageerror', e => errors.push(String(e && e.message || e)));
  await p.exposeFunction('__host', async (method, args) => {
    const r = await host.call(method, args);
    if (method === 'BusSendToGroup') sends.push({ args, response: r });
    if (method === 'BusGroupReach') previews.push({ args, response: r });
    return r;
  });
  await p.goto(require('./page').PAGE);
  await t.waitReady();
  await p.evaluate(th => pudlSetTheme(th), theme);
  await p.waitForFunction(th => document.documentElement.dataset.theme === th, theme, { timeout: 5000 }).catch(() => { throw new Error('the page did not take the ' + theme + ' theme'); });
  await p.evaluate(d => { window.dinahDocuments.open = () => [{ title: d.title, draftId: d.draftId, application: 'Docs', where: 'docs' }]; }, fixture.document);

  const frame = () => p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
  const chats = fixture.chats, byRole = role => chats.filter(c => c.role === role);
  const busOf = id => (chats.find(c => c.chatId === id) || {}).busId;
  const chatOfBus = id => (chats.find(c => c.busId === id) || {}).chatId;
  async function openRoom(key) {
    await p.evaluate(k => dinahBus.openGroup(k), key);
    const room = p.locator('.win[data-win-kind="bus-room"]').last();
    await room.locator('textarea').waitFor();
    return room;
  }
  const pop = room => room.locator('.group-mention-pop');
  async function listRows(room) {
    return pop(room).evaluate((el, kinds) => {
      const out = []; let section = '';
      let index = -1;
      el.querySelectorAll('.md-section-label, [data-mention]').forEach(n => {
        if (n.classList.contains('md-section-label')) { section = n.textContent; return; }
        index++;
        const key = n.dataset.mention, kind = key === 'everyone' ? 'everyone' : key === 'add' ? 'add' : kinds[section] || 'unknown';
        out.push({ index, section, key, kind, label: (n.querySelector('.mention-name') || {}).textContent || '', meta: (n.querySelector('.md-meta') || {}).textContent || '' });
      });
      return out;
    }, SECTION_KIND);
  }
  async function typeAt(room, text) { const box = room.locator('textarea'); await box.click(); await box.press('End'); await box.type(text); await frame(); }
  async function pickKey(room, row) { await pop(room).locator('[data-mention]').nth(row.index).locator('a').click(); await frame(); }
  async function waitPop(room) {
    await pop(room).waitFor({ state: 'visible' });
    await p.waitForFunction(() => [...document.querySelectorAll('.group-mention-pop .md-section-label')].some(h => /Other Chats|Applets|Windows/.test(h.textContent)), null, { timeout: 8000 }).catch(() => {});
    await frame();
  }
  async function send(room) {
    const before = sends.length;
    await room.locator('[data-view-id="send"], button[type="submit"]').first().click();
    for (let i = 0; i < 100 && sends.length <= before; i++) await new Promise(r => setTimeout(r, 100));
    await frame();
    const s = sends[sends.length - 1];
    if (!s || sends.length <= before) throw new Error('the page did not send to the host; evidence ' + artifact);
    /* A refused send or a failed observation fails the case, never an
       observation with entries missing. */
    if (s.response.error) throw new Error('the host: ' + s.response.error + '; evidence ' + artifact);
    return s;
  }
  async function reach(room) { return room.locator('.group-reach').textContent(); }
  async function popupInside(room) {
    if (!(await pop(room).isVisible())) return true;
    return pop(room).evaluate(el => { const r = el.getBoundingClientRect(); return r.left >= -0.5 && r.right <= innerWidth + 0.5 && r.top >= -0.5; });
  }
  async function overflow() { return p.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1); }
  function route(s, expected, preview) {
    const r = s.response || {};
    return { actualRecipients: (r.committedRecipients || []).slice(), expectedRecipients: expected.slice(), previewRecipients: preview.slice(), dispatchAttempts: r.dispatchAttempts || [], dispatchCounts: r.dispatchCounts || {}, dispatchCalls: r.dispatchCalls || [], storedDeliveries: r.storedDeliveries || [], deliveryRows: r.deliveryRows || [],
      referenceWakeAttempts: (r.dispatchAttempts || []).filter(id => !expected.includes(id)), membershipChanged: false, rawRequestPath: path.join(artifact, 'requests.jsonl'), rawReceiptPath: path.join(artifact, 'requests.jsonl') };
  }
  /* What the page last asked the host for this group's reach: the preview,
     recorded apart from what was committed. */
  function lastPreview(key) { const r = previews.filter(x => x.args[0] === key).pop(); return r && r.response && r.response.result ? (r.response.result.reach || []).map(m => chatOfBus(m.id)).filter(Boolean) : []; }
  const overviewSig = async () => JSON.stringify(((await host.call('BusOverview')).result.rooms || []).map(r => [r.name, (r.members || []).map(m => m.id).sort()]).sort((a, b) => a[0].localeCompare(b[0])));

  async function run() {
    const o = { scenario, theme, viewportWidth: width, evidencePath: artifact, sourceCommit: sourceCommit(), hostBinarySha256: host.binaryHash, realProductPage: true, setup };
    const membersBefore = await overviewSig();
    const member = byRole('member')[0], member2 = byRole('member')[1], outside = byRole('outside')[0], twin = byRole('outside-same-title')[0], lead = byRole('lead')[0], crew = byRole('team-member')[0];
    let room = await openRoom(fixture.group), inside = true;
    if (scenario === 'catalog') {
      await typeAt(room, '@'); await waitPop(room);
      const rows = await listRows(room); inside = await popupInside(room);
      o.sections = [...new Set(rows.map(r => r.section))];
      o.rows = rows;
      const firstRef = rows.findIndex(r => r.kind !== 'member' && r.kind !== 'everyone' && r.kind !== 'add');
      o.addChatLocation = rows.findIndex(r => r.kind === 'add') === firstRef - 1 && rows[firstRef - 1].section === 'Members, asked to answer' ? 'members-end' : 'elsewhere';
      o.referenceRowsExplainNoAsk = rows.filter(r => ['window', 'applet', 'document', 'chat'].includes(r.kind)).every(r => /asks no one/.test(r.meta));
      o.footerExplainsRouting = /Members are asked to answer; everything else is a reference and asks no one/.test(await pop(room).textContent());
      await p.keyboard.press('Escape');
      await room.locator('textarea').fill('');
      await room.locator('.group-add').click();
      await p.locator('.menu-panel:popover-open .menu-action', { hasText: 'Mention an Open Document or Chat' }).click();
      await waitPop(room);
      const viaAdd = await listRows(room);
      o.addOpensSameCatalog = JSON.stringify([...new Set(viaAdd.map(r => r.section))]) === JSON.stringify(o.sections) && viaAdd.length === rows.length;
      await p.keyboard.press('Escape');
    }
    if (scenario === 'keyboard') {
      const box = room.locator('textarea');
      await typeAt(room, 'See @rel'); await waitPop(room);
      const found = await listRows(room);
      await room.locator('textarea').fill(''); await typeAt(room, '@Sh'); await waitPop(room);
      const found2 = await listRows(room);
      o.searchAcrossAllKinds = found.some(r => r.kind === 'document') && found2.some(r => r.kind === 'chat');
      const sel0 = await pop(room).locator('[aria-selected="true"]').getAttribute('data-mention');
      await p.keyboard.press('ArrowDown');
      const sel1 = await pop(room).locator('[aria-selected="true"]').getAttribute('data-mention');
      await p.keyboard.press('ArrowUp');
      const sel2 = await pop(room).locator('[aria-selected="true"]').getAttribute('data-mention');
      o.arrowSelectionObserved = sel0 !== sel1 && sel2 === sel0;
      /* Enter on a reference row, never Add a Chat…, which opens the picker. */
      for (let i = 0; i < 8 && ['add', 'everyone'].includes(await pop(room).locator('[aria-selected="true"]').getAttribute('data-mention')); i++) await p.keyboard.press('ArrowDown');
      const chosen = await pop(room).locator('[aria-selected="true"]').getAttribute('data-mention');
      await p.keyboard.press('Enter'); await frame();
      const value = await box.inputValue();
      o.enterInsertsSelectedIdentity = (chosen.startsWith('@') ? value.includes(chosen.split(':')[0]) : value.includes('@')) && await pop(room).isHidden();
      await typeAt(room, ' and @'); await waitPop(room); inside = await popupInside(room);
      const draft = await box.inputValue();
      await p.keyboard.press('Escape'); await frame();
      o.escapeClosesWithoutDraftLoss = await pop(room).isHidden() && (await box.inputValue()) === draft;
      o.focusReturnsToComposer = await p.evaluate(() => document.activeElement && document.activeElement.tagName === 'TEXTAREA');
    }
    if (scenario === 'member-reference-routing' || scenario === 'everyone' || scenario === 'team-default') {
      o.actualHostRoutingObserved = true; o.routes = [];
      const key = scenario === 'team-default' ? fixture.team : fixture.group;
      if (scenario === 'team-default') room = await openRoom(fixture.team);
      const members = scenario === 'team-default' ? [lead.chatId, crew.chatId] : [member.chatId, member2.chatId];
      async function compose(parts) {
        await room.locator('textarea').fill('');
        for (const part of parts) {
          if (part.text) await typeAt(room, part.text);
          if (part.at) { await typeAt(room, '@' + part.at); await waitPop(room); const rows = await listRows(room); const r = rows.find(x => x.kind === part.kind && (!part.label || x.label === part.label)); if (!r) throw new Error('no ' + part.kind + ' row for ' + part.at); await pickKey(room, r); }
        }
        await frame(); await new Promise(r => setTimeout(r, 600));
      }
      if (scenario === 'member-reference-routing') {
        await compose([{ text: 'Baseline message.' }]);
        let s = await send(room);
        const baseline = (s.response.committedRecipients || []).slice();
        o.routes.push(route(s, baseline, lastPreview(key)));
        const refs = [{ at: 'Files', kind: 'window' }, { at: 'Git', kind: 'applet' }, { at: 'release', kind: 'document' }, { at: 'Sheet', kind: 'chat' }];
        const kinds = [];
        for (const ref of refs) {
          await compose([{ text: 'Context: ' }, ref]);
          s = await send(room);
          o.routes.push(route(s, baseline, lastPreview(key)));
          if ((s.args[3] || []).length) kinds.push(ref.kind);
        }
        o.allFourReferenceKindsExercised = kinds.length === 4;
        o.referencesLeaveBaselineRecipientsUnchanged = o.routes.slice(1).every(r => JSON.stringify([...r.actualRecipients].sort()) === JSON.stringify([...baseline].sort()));
        await compose([{ at: member.title, kind: 'member', label: member.title }, { text: 'please look at ' }, { at: 'Sheet', kind: 'chat' }]);
        s = await send(room);
        o.selectedMemberIds = [member.chatId];
        o.explicitMemberRecipients = (s.response.committedRecipients || []).slice();
        o.routes.push(route(s, [member.chatId], lastPreview(key)));
      }
      if (scenario === 'everyone') {
        await compose([{ at: 'everyone', kind: 'everyone' }, { text: 'all of you.' }]);
        let s = await send(room);
        o.allEligibleMemberIds = members;
        o.everyoneRecipients = (s.response.committedRecipients || []).slice();
        o.routes.push(route(s, members, lastPreview(key)));
        await compose([{ at: 'everyone', kind: 'everyone' }, { text: 'with ' }, { at: 'Files', kind: 'window' }]);
        s = await send(room);
        o.routes.push(route(s, members, lastPreview(key)));
      }
      if (scenario === 'team-default') {
        o.teamLeadId = lead.chatId;
        await compose([{ text: 'Status, please.' }]);
        let s = await send(room);
        o.baselineRecipients = (s.response.committedRecipients || []).slice();
        o.routes.push(route(s, [lead.chatId], lastPreview(key)));
        await compose([{ text: 'Context only: ' }, { at: 'Sheet', kind: 'chat' }]);
        s = await send(room);
        o.referenceOnlyRecipients = (s.response.committedRecipients || []).slice();
        o.routes.push(route(s, [lead.chatId], lastPreview(key)));
        await compose([{ at: crew.title, kind: 'member', label: crew.title }, { text: 'over to you.' }]);
        s = await send(room);
        o.selectedMemberIds = [crew.chatId];
        o.memberOverrideRecipients = (s.response.committedRecipients || []).slice();
        o.routes.push(route(s, [crew.chatId], lastPreview(key)));
      }
    }
    if (scenario === 'deduplication') {
      await typeAt(room, '@Docs'); await waitPop(room);
      const rows = await listRows(room); inside = await popupInside(room);
      const memberRows = rows.filter(r => r.kind === 'member' && r.label === member.title);
      o.memberListedOnce = memberRows.length === 1;
      /* The outside chat with the same title stays; the member's own chat is not
         offered again among Other Chats, by its identity or as a second row. */
      const sameTitleChats = rows.filter(r => r.kind === 'chat' && r.label === member.title);
      o.memberAbsentFromOtherChats = sameTitleChats.length === 1 && !sameTitleChats[0].key.endsWith(':' + encodeURIComponent(member.chatId));
      const twinRow = rows.find(r => r.kind === 'chat' && r.label === twin.title);
      await p.keyboard.press('Escape');
      await room.locator('textarea').fill('');
      await typeAt(room, '@' + member.title); await waitPop(room); await pickKey(room, memberRows[0]);
      await typeAt(room, 'and again @' + member.title); await p.keyboard.press('Escape');
      await typeAt(room, ' and the other one '); await typeAt(room, '@Docs'); await waitPop(room);
      const rows2 = await listRows(room);
      const twin2 = rows2.find(r => r.kind === 'chat' && r.label === twin.title);
      if (twin2) await pickKey(room, twin2); else await p.keyboard.press('Escape');
      await new Promise(r => setTimeout(r, 600));
      const s = await send(room);
      o.actualHostRoutingObserved = true;
      // The host's raw count of prompts to the member for this send, repeats included.
      o.repeatedMemberDispatchCount = (s.response.dispatchCounts || {})[member.chatId] || 0;
      const sentRefs = (s.args[3] || []).filter(f => f.kind === 'chat');
      o.sameTitleDifferentIdentityPreserved = !!twinRow && sentRefs.some(f => f.sessionId === twin.chatId) && !(s.response.committedRecipients || []).includes(twin.chatId) && (s.response.committedRecipients || []).includes(member.chatId);
    }
    if (scenario === 'sent-lines' || scenario === 'reference-identity') {
      async function compose(parts, r) {
        r = r || room;
        await r.locator('textarea').fill('');
        for (const part of parts) {
          if (part.text) await typeAt(r, part.text);
          if (part.at) { await typeAt(r, '@' + part.at); await waitPop(r); const rows = await listRows(r); const row = rows.find(x => x.kind === part.kind && (!part.label || x.label === part.label)); if (!row) throw new Error('no ' + part.kind + ' row for ' + part.at); await pickKey(r, row); }
        }
        await new Promise(res => setTimeout(res, 600));
      }
      async function card(r, s) {
        const id = s.response.result.id;
        const el = r.locator('[data-bus-message="' + id + '"]');
        await el.locator('.msg-text').waitFor();
        return el.evaluate((node, info) => {
          const m = node.querySelector('.mention'), refs = [...node.querySelectorAll('.mention-ref')];
          const mc = m && getComputedStyle(m), rc = refs[0] && getComputedStyle(refs[0]);
          const asked = node.querySelector('[data-asked]'), referred = node.querySelector('[data-referred]');
          const clip = el => !el || el.scrollWidth > el.clientWidth + 1 || el.getBoundingClientRect().right > innerWidth + 0.5;
          return { askedLine: asked ? asked.textContent : '', referredLine: referred ? referred.textContent : '',
            memberAndReferenceWeightDiffer: !!(mc && rc && mc.fontWeight !== rc.fontWeight), memberAndReferenceColorDiffer: !!(mc && rc && mc.color !== rc.color),
            referenceHasKindGlyph: refs.length > 0 && refs.every(x => x.querySelector('.glyph')), referenceHasDottedUnderline: refs.length > 0 && refs.every(x => /dotted/.test(getComputedStyle(x).textDecorationStyle || getComputedStyle(x).textDecoration)),
            linesVisibleWithoutClipping: !!asked && !!referred && !clip(asked) && !clip(referred), renderedTokens: refs.map(x => x.textContent), referredText: referred ? referred.textContent : '' };
        }, null);
      }
      if (scenario === 'sent-lines') {
        o.messages = [];
        const plans = [
          { r: room, key: fixture.group, parts: [{ at: member.title, kind: 'member', label: member.title }, { text: 'please check ' }, { at: 'release', kind: 'document' }, { text: 'and ' }, { at: 'Files', kind: 'window' }] },
          { r: room, key: fixture.group, parts: [{ at: member2.title, kind: 'member', label: member2.title }, { text: 'ask ' }, { at: 'Sheet', kind: 'chat' }] },
          { team: true, parts: [{ at: crew.title, kind: 'member', label: crew.title }, { text: 'context for you: ' }, { at: 'Sheet', kind: 'chat' }] }
        ];
        for (const plan of plans) {
          const r = plan.team ? await openRoom(fixture.team) : room;
          await compose(plan.parts, r);
          const s = await send(r);
          const c = await card(r, s);
          const inserted = (s.args[3] || []).map(f => f.token).filter(Boolean);
          const chatRef = (s.args[3] || []).find(f => f.kind === 'chat');
          o.messages.push(Object.assign(c, { askedIds: ((s.response.result.reach || {}).reach || []).map(m => chatOfBus(m.id)), actualRecipientIds: s.response.committedRecipients || [],
            referenceIds: c.renderedTokens, insertedReferenceIds: inserted, referenceChatWorkspaceShown: !chatRef || new RegExp('\\(' + (chats.find(x => x.chatId === chatRef.sessionId) || {}).workspace + '\\)').test(c.referredText) }));
          if (plan.team) room = r;
        }
        o.afterSendDraftEmpty = (await room.locator('textarea').inputValue()) === '';
        await new Promise(r => setTimeout(r, 800));
        o.afterSendDefaultReachRestored = /^Reaches /.test(await reach(room)) && !/references? asks? no one/.test(await reach(room));
        await typeAt(room, '@'); await waitPop(room); inside = await popupInside(room); await p.keyboard.press('Escape');
      }
      if (scenario === 'reference-identity') {
        await compose([{ text: 'See ' }, { at: 'Files', kind: 'window' }, { at: 'Git', kind: 'applet' }, { at: 'release', kind: 'document' }, { at: 'Sheet', kind: 'chat' }]);
        const s = await send(room);
        const files = s.args[3] || [];
        o.referenceKinds = [...new Set(files.map(f => f.kind === 'draft' ? 'document' : f.kind))];
        o.documentResourceLinkPreserved = files.some(f => f.kind === 'draft' && f.draftId === fixture.document.draftId);
        o.appletVerifiedIdentityPreserved = files.some(f => f.kind === 'applet' && !!f.provider);
        o.chatWorkspaceAndIdentityPreserved = files.some(f => f.kind === 'chat' && f.sessionId === outside.chatId) && (s.response.result.references || []).some(r => r.kind === 'chat' && r.workspace === outside.workspace);
        o.outsideChatAddOfferShown = await room.locator('text=/Add .*Sheet Helper.* to the group/').count() > 0;
        o.referenceWakeAttempts = (s.response.dispatchAttempts || []).filter(id => id === outside.chatId);
        await host.call('restart');
        const reloaded = (await host.call('BusGroupMessages', [fixture.group])).result || [];
        const again = reloaded.find(m => m.id === s.response.result.id);
        o.referencesPersistAfterReload = !!again && JSON.stringify(again.references) === JSON.stringify(s.response.result.references) && again.shownText === s.response.result.shownText;
        o.membershipChanged = (await overviewSig()) !== membersBefore;
        await typeAt(room, '@'); await waitPop(room); inside = await popupInside(room); await p.keyboard.press('Escape');
      }
    }
    if (scenario !== 'reference-identity' && scenario !== 'catalog' && scenario !== 'keyboard' && scenario !== 'deduplication' && scenario !== 'sent-lines') {
      await typeAt(room, '@'); await waitPop(room); inside = await popupInside(room); await p.keyboard.press('Escape');
    }
    o.popupInsideViewport = inside;
    o.horizontalOverflow = await overflow();
    o.membershipBefore = membersBefore; o.membershipAfter = await overviewSig();
    o.unrelatedStateUnchanged = o.membershipAfter === membersBefore;
    o.theme = await p.evaluate(() => document.documentElement.dataset.theme);
    o.pageErrors = errors.slice();
    await p.screenshot({ path: path.join(artifact, 'final.png') });
    fs.writeFileSync(path.join(artifact, 'observations.json'), JSON.stringify(Object.assign({}, o, { sends, previews }), null, 2));
    return o;
  }
  return { run, async close() {
    let pageError = null;
    try { await t.close(); } catch (e) { pageError = e; }
    await host.stop();
    if (pageError) throw pageError;
  } };
}
module.exports = { open };
