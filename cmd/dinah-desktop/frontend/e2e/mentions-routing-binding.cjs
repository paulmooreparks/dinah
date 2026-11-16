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
  return { close: host.close, run: async function () {
    const groups = host.manifest.groups, chats = host.manifest.chats;
    const group = groups['Invented overlapping group'], otherGroup = groups['Invented second group'];
    const pendingReply = await seed({ kind: 'group', group, text: 'Invented review to acknowledge by reply.' });
    const existing = await seed({ kind: 'group', group, text: 'Invented review to open in an existing chat.' });
    const target = await seed({ kind: 'group', group, text: 'Invented review to open in a new chat.' });
    const other = await seed({ kind: 'group', group: otherGroup, chatId: chats['free-b'], text: 'Invented independent review.' });
    const request = await seed({ kind: 'request', group, text: 'Should this invented request remain open?' });
    const before = await state();
    fs.writeFileSync(path.join(out, 'before.json'), JSON.stringify({ pendingReply, existing, target, other, request, before }, null, 2));
    await p.reload();
    await p.waitForFunction(() => window.dinahUIState && window.dinahWorkbench && window.dinahHost?.notices);
    await p.evaluate(() => dinahUIState.ready);
    await p.waitForFunction(id => dinahHost.notices.list(n => n.data.message === id).length > 0, target.message.id);
    await p.evaluate(() => {
      const app = go.main.App, forward = app.OrgChartHostFixtureMentionCall;
      window.__mentionNavigation = []; window.__mentionAck = []; window.__mentionHold = false;
      document.addEventListener('dinah:bus-message-opened', event => __mentionNavigation.push({ ...event.detail, time: performance.now() }));
      for (const name of ['Notifications', 'DecisionEntries', 'Decisions', 'DecisionByID', 'AcknowledgeMention', 'BusThread', 'BusGroupMessages', 'BusGroupMessagesFrom', 'BusMarkInboxRead', 'BusMarkGroupRead', 'BusSend', 'BusSendToGroup', 'BusReplyInThread', 'PendingTaskPermissions']) {
        app[name] = async function (...args) {
          if (name === 'AcknowledgeMention') {
            const row = document.querySelector('.win.active .bus-target-message[data-bus-message="' + args[0] + '"]');
            const r = row && row.getBoundingClientRect(), log = row && row.closest('.bus-log'), b = log && log.getBoundingClientRect();
            __mentionAck.push({ id: args[0], time: performance.now(), marked: !!row, visible: !!(r && b && r.width && r.bottom > b.top && r.top < b.bottom && r.bottom > 0 && r.top < innerHeight) });
          }
          const value = await forward(name, args);
          if (name === 'BusGroupMessagesFrom' && __mentionHold) await new Promise(resolve => window.__releaseMention = resolve);
          return value;
        };
      }
      return Promise.resolve();
    });
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await p.evaluate(() => { window.__mentionHold = true; });
    await p.waitForFunction(id => dinahHost.notices.list(n => n.data.message === id).length > 0, target.message.id);
    const cardId = await p.evaluate(id => dinahHost.notices.list(n => n.data.message === id)[0].id, target.message.id);
    const action = () => p.locator('#questions [data-notice-id="' + cardId + '"]').getByRole('button', { name: 'Open in Chat', exact: true });
    const buttonName = await action().textContent();
    await action().focus();
    const keyboard = await action().evaluate(el => el.tabIndex >= 0 && document.activeElement === el && !el.disabled);
    await p.keyboard.press('Enter');
    await p.waitForFunction(() => !!window.__releaseMention);
    const notEarly = await p.evaluate(() => __mentionAck.length === 0);
    await p.evaluate(() => { __mentionHold = false; __releaseMention(); });
    await p.waitForFunction(id => __mentionAck.some(a => a.id === id), target.message.id);
    await host.idle();
    const reached = await state();
    const first = await p.evaluate(() => ({ ack: __mentionAck.slice(), navigation: __mentionNavigation.slice(), windows: [...document.querySelectorAll('.group-window')].map(el => el.closest('[data-win]').dataset.win) }));
    await p.screenshot({ path: path.join(out, 'opened-target.png') });
    await p.evaluate(() => dinahWorkbench.open('needs-you'));
    await action().click();
    await p.waitForFunction(id => __mentionAck.some(a => a.id === id), existing.message.id);
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
      realApp: !!before.sourceMessages.find(m => m.id === target.message.id), realProductPage: !!await p.locator('.decisions').count(), product: spec.product, theme: await p.evaluate(() => document.documentElement.dataset.theme), viewportWidth: p.viewportSize().width, buttonName,
      buttonKeyboardAccessible: keyboard, closedChatOpens: first.windows.length === 1, existingChatRaisedWithoutDuplicate: windows === 1 && ack.some(a => a.id === existing.message.id && a.visible),
      exactMessageScrolledIntoView: first.ack[0].id === target.message.id && first.ack[0].visible, targetMessageMarked: first.ack[0].marked,
      acknowledgedAfterArrival: notEarly && first.navigation.some(n => n.message === target.message.id && n.time <= first.ack[0].time) && !present(reached, target.message.id),
      replyClearsMention: present(afterExisting, pendingReply.message.id) && !present(afterReply, pendingReply.message.id),
      noMentionRowInDecisions: rows.length === 1 && rows[0] === request.request.id && afterReply.decisions.items.every(item => item.kind === 'request'),
      remainingDecisionCountsCorrect: afterReply.entries.count === 1 && afterReply.entries.items[0].id === request.request.id,
      otherChatUnaffected: present(afterReply, other.message.id), horizontalOverflow: await p.evaluate(() => document.documentElement.scrollWidth > innerWidth + 1), errors: host.errors.slice()
    };
    fs.writeFileSync(path.join(out, 'observations.json'), JSON.stringify({ result, reached, afterExisting, afterReply, first, ack, rows }, null, 2));
    return result;
  } };
};
