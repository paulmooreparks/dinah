'use strict';
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { open } = require('./lib/org-chart-real-host.cjs');
let serial = 0;

exports.open = async function (spec) {
  const configs = JSON.parse(fs.readFileSync(process.env.MENTIONS_RECEIVING_CONFIG, 'utf8'));
  if (spec.scenario !== 'open-in-chat') {
    assert.ok(configs.hostBinding, 'Supply the immutable host-scenario binding.');
    return require(configs.hostBinding).open(spec);
  }
  const config = configs[spec.product];
  assert.ok(config && config.sourceCommit, 'Supply the actual product build identity.');
  const root = process.env.MENTIONS_RECEIVING_OUT;
  assert.ok(root, 'Supply a private evidence folder.');
  fs.mkdirSync(root, { recursive: true });
  const out = path.join(root, String(++serial).padStart(3, '0') + '-' + spec.product + '-' + spec.theme + '-' + spec.width);
  const host = await open(config, out, spec), p = host.page;
  assert.equal(host.manifest.product, config.productName, 'The executable must match its declared product profile.');
  const call = host.call;
  const state = () => call('OrgChartHostFixtureMentionState');
  const seed = input => call('OrgChartHostFixtureMentionSeed', input);
  return { close: async function () {
    if (!p.isClosed()) {
      await p.screenshot({ path: path.join(out, 'final-page.png') });
      fs.writeFileSync(path.join(out, 'final-page.json'), JSON.stringify(await p.evaluate(() => ({ text: document.body.innerText, notices: dinahHost.notices.list().map(n => ({ id: n.id, data: n.data })), ack: window.__mentionAck, navigation: window.__mentionNavigation })), null, 2));
    }
    await host.close();
  }, run: async function () {
    const groups = host.manifest.groups, chats = host.manifest.chats;
    const group = groups['Invented overlapping group'], otherGroup = groups['Invented second group'];
    const pendingReply = await seed({ kind: 'group', group, text: 'Invented review to acknowledge by reply.' });
    const existing = await seed({ kind: 'group', group, text: 'Invented review to open in an existing chat.' });
    const target = await seed({ kind: 'group', group, text: 'Invented review to open in a new chat.' });
    const other = await seed({ kind: 'group', group: otherGroup, chatId: chats['free-b'], text: 'Invented independent review.' });
    const request = await seed({ kind: 'request', group, text: 'Should this invented request remain open?' });
    const before = await state();
    fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({ pendingReply, existing, target, other, request, before }, null, 2));
    const prelude = spec.beforeNavigation ? await spec.beforeNavigation({ host, p, seed, state, group, target, request }) : null;
    await p.reload();
    await p.waitForFunction(() => window.dinahUIState && window.dinahWorkbench && window.dinahHost?.notices);
    await p.evaluate(() => dinahUIState.ready);
    await p.evaluate(theme => document.documentElement.dataset.theme = theme, spec.theme);
    await p.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    await p.waitForFunction(id => dinahHost.notices.list(n => n.data.message === id).length > 0, target.message.ID);
    await p.evaluate(() => {
      const app = go.main.App, forward = app.OrgChartHostFixtureMentionCall;
      window.__mentionNavigation = []; window.__mentionAck = []; window.__mentionHold = false; window.__mentionReleases = [];
      document.addEventListener('dinah:bus-message-opened', event => __mentionNavigation.push({ ...event.detail, time: performance.now() }));
      for (const name of ['Notifications', 'DecisionEntries', 'Decisions', 'DecisionByID', 'AcknowledgeMention', 'BusThread', 'BusGroupMessages', 'BusGroupMessagesFrom', 'BusMarkInboxRead', 'BusMarkGroupRead', 'BusSend', 'BusSendToGroup', 'BusReplyInThread', 'PendingTaskPermissions']) {
        app[name] = async function (...args) {
          if (name === 'AcknowledgeMention') {
            const row = document.querySelector('.win.active .bus-target-message[data-bus-message="' + args[0] + '"]');
            const r = row && row.getBoundingClientRect(), log = row && row.closest('.bus-log'), b = log && log.getBoundingClientRect();
            __mentionAck.push({ id: args[0], time: performance.now(), marked: !!row, visible: !!(r && b && r.width && r.bottom > b.top && r.top < b.bottom && r.bottom > 0 && r.top < innerHeight) });
          }
          const value = await forward(name, args);
          if (['BusGroupMessages', 'BusGroupMessagesFrom'].includes(name) && __mentionHold) await new Promise(resolve => __mentionReleases.push(resolve));
          return value;
        };
      }
      return Promise.resolve();
    });
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await p.evaluate(() => { window.__mentionHold = true; });
    await p.waitForFunction(id => dinahHost.notices.list(n => n.data.message === id).length > 0, target.message.ID);
    const cardId = await p.evaluate(id => dinahHost.notices.list(n => n.data.message === id)[0].id, target.message.ID);
    const action = () => p.locator('#questions [data-notice-id="' + cardId + '"]').getByRole('button', { name: 'Open in Chat', exact: true });
    const buttonName = await action().textContent();
    await action().focus();
    const keyboard = await action().evaluate(el => el.tabIndex >= 0 && document.activeElement === el && !el.disabled);
    await p.keyboard.press('Enter');
    await p.waitForFunction(() => window.__mentionReleases.length > 0);
    const notEarly = await p.evaluate(() => __mentionAck.length === 0);
    await p.evaluate(() => { __mentionHold = false; __mentionReleases.splice(0).forEach(resolve => resolve()); });
    await p.waitForFunction(id => __mentionAck.some(a => a.id === id), target.message.ID);
    await host.idle();
    const reached = await state();
    const first = await p.evaluate(() => ({ ack: __mentionAck.slice(), navigation: __mentionNavigation.slice(), windows: [...document.querySelectorAll('.group-window')].map(el => el.closest('[data-win]').dataset.win) }));
    await p.screenshot({ path: path.join(out, 'opened-target.png') });
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await action().click();
    await p.waitForFunction(id => __mentionAck.some(a => a.id === id), existing.message.ID);
    await host.idle();
    const afterExisting = await state();
    const windows = await p.locator('.group-window').count();
    const active = p.locator('.win.active .group-window');
    const composer = active.locator('textarea').first();
    await composer.fill('I have read this invented review.');
    await composer.press('Control+Enter');
    await p.waitForFunction(() => __orgReceivingCalls.some(c => c.method === 'OrgChartHostFixtureMentionCall' && c.args?.[0] === 'BusSendToGroup' && c.end));
    await host.idle();
    // Reading this port runs the actual mention watcher over the actual reply.
    const afterReply = await state();
    await p.evaluate(() => dinahWorkbench.open('decisions'));
    await host.idle();
    const rows = await p.locator('.decisions-row').evaluateAll(nodes => nodes.map(el => el.dataset.requestId));
    const ack = await p.evaluate(() => __mentionAck.slice());
    const present = (snapshot, id) => snapshot.notifications.some(n => n.message === id && !n.dismissed);
    const result = {
      sourceCommit: config.sourceCommit, hostBinarySha256: crypto.createHash('sha256').update(fs.readFileSync(config.host)).digest('hex'), evidencePath: out,
      realApp: !!before.sourceMessages.find(m => m.ID === target.message.ID), realProductPage: !!await p.locator('.decisions').count(), product: spec.product, theme: await p.evaluate(() => document.documentElement.dataset.theme), viewportWidth: p.viewportSize().width, buttonName,
      buttonKeyboardAccessible: keyboard, closedChatOpens: first.windows.length === 1, existingChatRaisedWithoutDuplicate: windows === 1 && ack.some(a => a.id === existing.message.ID && a.visible),
      exactMessageScrolledIntoView: first.ack[0].id === target.message.ID && first.ack[0].visible, targetMessageMarked: first.ack[0].marked,
      acknowledgedAfterArrival: notEarly && first.navigation.some(n => n.message === target.message.ID && n.time <= first.ack[0].time) && !present(reached, target.message.ID),
      replyClearsMention: present(afterExisting, pendingReply.message.ID) && !present(afterReply, pendingReply.message.ID),
      noMentionRowInDecisions: rows.length === 1 && rows[0] === request.request.id && afterReply.decisions.items.every(item => item.kind === 'request'),
      remainingDecisionCountsCorrect: afterReply.entries.count === 1 && afterReply.entries.items[0].id === request.request.id,
      otherChatUnaffected: present(afterReply, other.message.ID), horizontalOverflow: await p.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1), errors: host.errors.slice()
    };
    fs.writeFileSync(path.join(out, 'observations.json'), JSON.stringify({ result, reached, afterExisting, afterReply, first, ack, rows }, null, 2));
    return { ...result, raw: { before, prelude, reached, afterExisting, afterReply, first, ack, rows, pendingReply, existing, target, other, request } };
  } };
};

// Quality owns the assertions over these raw navigation and persisted-state observations.
exports.openNavigation = async function (spec) {
  if (spec.scenario === 'open-exact') return exports.open({ ...spec, scenario: 'open-in-chat', product: spec.product || 'dinah', theme: spec.theme || 'light', width: spec.width || 1440, beforeNavigation: async function ({ host, p, seed, state, group, target, request }) {
    const beforeOpen = await state();
    const key = await p.evaluate(group => dinahBus.openGroup(group), group);
    await p.locator('.group-window [data-bus-message]').first().waitFor();
    await host.idle();
    const afterOpenWithoutTarget = await state();
    await p.evaluate(key => pudlWindows.close(key), key);
    const wrong = await seed({ kind: 'direct', text: 'Invented different acknowledgement target.' });
    await host.call('OrgChartHostFixtureMentionCall', 'AcknowledgeMention', [wrong.message.ID]);
    const afterWrongMessage = await state();
    const nonmention = await seed({ kind: 'nonmention', group });
    const refusals = [];
    for (const source of [nonmention, request]) {
      try { await host.call('OrgChartHostFixtureMentionCall', 'AcknowledgeMention', [source.message.ID]); refusals.push({ id: source.message.ID, error: '' }); }
      catch (error) { refusals.push({ id: source.message.ID, error: String(error) }); }
    }
    const afterRefusals = await state();
    return { beforeOpen, afterOpenWithoutTarget, wrong, afterWrongMessage, nonmention, refusals, afterRefusals, target };
  } });
  assert.equal(spec.scenario, 'unavailable');
  const configs = JSON.parse(fs.readFileSync(process.env.MENTIONS_RECEIVING_CONFIG, 'utf8'));
  const config = configs.dinah, root = process.env.MENTIONS_RECEIVING_OUT;
  fs.mkdirSync(root, { recursive: true });
  const out = path.join(root, String(++serial).padStart(3, '0') + '-unavailable');
  const host = await open(config, out, { width: 1440, theme: 'light' }), p = host.page;
  return { close: host.close, run: async function () {
    const seeded = await host.call('OrgChartHostFixtureMentionSeed', { kind: 'group' });
    const before = await host.call('OrgChartHostFixtureMentionState');
    const protectedBefore = await host.call('OrgChartHostFixtureSnapshot');
    await p.reload();
    await p.waitForFunction(id => window.dinahHost?.notices?.list(n => n.data.message === id).length, seeded.message.ID);
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    const cardId = await p.evaluate(id => dinahHost.notices.list(n => n.data.message === id)[0].id, seeded.message.ID);
    await host.call('OrgChartHostFixtureControl', { kind: 'cleanup-store-unavailable' });
    await p.locator('#questions [data-notice-id="' + cardId + '"]').getByRole('button', { name: 'Open in Chat', exact: true }).click();
    const failure = p.locator('.toast').filter({ hasText: /closed|database|unavailable/i }).first();
    await failure.waitFor();
    const failureText = await failure.innerText();
    const windows = await p.locator('.group-window').count();
    const cardPresent = await p.locator('#questions [data-notice-id="' + cardId + '"]').count();
    await host.call('OrgChartHostFixtureControl', { kind: 'restart' });
    const after = await host.call('OrgChartHostFixtureMentionState');
    const protectedAfter = await host.call('OrgChartHostFixtureSnapshot');
    const calls = await p.evaluate(() => __orgReceivingCalls);
    const result = { sourceCommit: config.sourceCommit, hostBinarySha256: crypto.createHash('sha256').update(fs.readFileSync(config.host)).digest('hex'), evidencePath: out, realApp: !!before.sourceMessages.find(m => m.ID === seeded.message.ID), errors: host.errors.slice(), raw: { seeded, before, after, protectedBefore, protectedAfter, failureText, windows, cardPresent, calls } };
    fs.writeFileSync(path.join(out, 'observations.json'), JSON.stringify(result, null, 2));
    return result;
  } };
};
