'use strict';
const fs = require('node:fs');
const path = require('node:path');
const assert = require('node:assert/strict');

const settle = page => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
const card = (host, id) => host.chart.locator('[data-org-chat=' + JSON.stringify(id) + ']');
async function chartFront(host) {
  await host.page.evaluate(() => dinahWorkbench.open('org-chart'));
  await settle(host.page);
}

async function structure(host) {
  const { chart, page, manifest } = host;
  assert.ok(manifest.expectedInitialInventoryIds && manifest.expectedTeams && manifest.expectedGroups,
    'The fixture declares its expected population independently of query results.');
  const refs = Object.values(manifest.refs);
  const normalize = value => value.toLowerCase().replace(/[\\/]+$/, '');
  const states = [];
  async function capture(name, workspace = '', folded = '') {
    const wanted = manifest.expectedInitialInventoryIds.filter(id => !workspace || normalize(refs.find(ref => ref.id === id).cwd) === normalize(workspace));
    const expected = new Set(wanted);
    if (name === 'empty-filter') expected.clear();
    if (folded) {
      const team = manifest.expectedTeams.find(team => team.id === folded);
      const descendants = new Set();
      function descend(team) {
        for (const id of team.memberIds) {
          if (descendants.has(id)) continue;
          descendants.add(id);
          const child = manifest.expectedTeams.find(team => team.leadId === id);
          if (child) descend(child);
        }
      }
      descend(team);
      for (const id of descendants) expected.delete(id);
    }
    const expectedGroups = manifest.expectedGroups.filter(group => name !== 'empty-filter' && group.memberIds.some(id => wanted.includes(id)))
      .map(group => ({ id: group.id, memberIds: group.memberIds }));
    const observed = await chart.evaluate(root => {
      const boxes = [...root.querySelectorAll('[data-org-chat]')];
      return {
        boxIds: boxes.map(box => box.dataset.orgChat),
        containers: [...root.querySelectorAll('.oc-team,.oc-ws,.oc-group')].map(container => ({
          contentIds: [...container.querySelectorAll('[data-org-chat],[data-chat-link]')].map(row => row.dataset.orgChat || row.dataset.chatLink)
        })),
        groupRows: [...root.querySelectorAll('[data-org-group]')].map(row => ({ id: row.dataset.orgGroup, memberIds: [...row.querySelectorAll('[data-chat-link]')].map(link => link.dataset.chatLink) })),
        boxGroupLabels: Object.fromEntries(boxes.map(box => [box.dataset.orgChat, [...box.querySelectorAll('[data-org-group-label]')].map(link => link.dataset.orgGroupLabel)])),
        dottedGroupLines: [...root.querySelectorAll('svg path,svg line')].filter(line => getComputedStyle(line).strokeDasharray !== 'none').length,
        emptyText: root.querySelector('.org-chart-empty')?.textContent || '',
        readingWidth: root.querySelector('.org-chart-scroll').clientWidth,
        contentWidth: root.querySelector('.org-chart-scroll').scrollWidth
      };
    });
    states.push({ name, ...observed, expectedVisibleIds: [...expected], expectedGroups });
    fs.writeFileSync(path.join(host.out, name + '-structure.json'), JSON.stringify(states.at(-1), null, 2));
    await page.screenshot({ path: path.join(host.out, name + '.png') });
    assert.ok(observed.contentWidth <= observed.readingWidth + 1,
      'The chart fits its horizontal reading surface: ' + observed.contentWidth + ' content pixels in ' + observed.readingWidth + ' reading pixels.');
  }
  await capture('all');
  await capture('nested');
  const first = manifest.expectedTeams.find(team => team.memberIds.length);
  await chart.locator('[data-org-fold=' + JSON.stringify(first.id) + ']').click();
  await settle(page);
  await capture('folded', '', first.id);
  await chart.locator('[data-org-fold=' + JSON.stringify(first.id) + ']').click();
  const workspace = refs[0].cwd;
  await chart.getByRole('combobox', { name: 'Workspace', exact: true }).selectOption(workspace);
  await settle(page);
  await capture('workspace', workspace);
  await chart.getByRole('searchbox').fill('Invented no matching chat 8821');
  await settle(page);
  await capture('empty-filter', workspace);
  assert.ok(states.at(-1).emptyText, 'The empty filter has a visible explanation.');
  return { states };
}

async function actions(host) {
  const { page, chart } = host;
  await page.mouse.move(0, 0);
  await page.evaluate(() => document.activeElement?.blur());
  const buttons = [];
  const controls = chart.locator('.org-chart-actions-button');
  for (let index = 0; index < await controls.count(); index++) {
    const control = controls.nth(index);
    await control.scrollIntoViewIfNeeded();
    buttons.push(await control.evaluate(button => {
    const owner = button.closest('[data-org-chat],[data-org-group]');
    const rect = button.getBoundingClientRect(), style = getComputedStyle(button);
    return { ownerId: owner.dataset.orgChat || owner.dataset.orgGroup, width: rect.width, height: rect.height,
      opacity: Number(style.opacity), visibility: style.visibility, display: style.display, tabIndex: button.tabIndex,
      accessibleName: button.getAttribute('aria-label') || button.textContent };
    }));
  }
  const expectedOwnerIds = [...host.manifest.expectedInitialInventoryIds, ...host.manifest.expectedGroups.map(group => group.id)];
  const tabVisitedIds = [];
  // Actual keyboard traversal also exercises the narrow viewport's scroll path.
  await chart.locator('.org-chart-scroll').focus();
  for (let i = 0; i < expectedOwnerIds.length * 12 + 40; i++) {
    await page.keyboard.press('Tab');
    const owner = await page.evaluate(() => {
      const active = document.activeElement;
      if (!active?.matches('.org-chart-actions-button')) return null;
      const row = active.closest('[data-org-chat],[data-org-group]');
      return row.dataset.orgChat || row.dataset.orgGroup;
    });
    if (owner && !tabVisitedIds.includes(owner)) tabVisitedIds.push(owner);
    if (tabVisitedIds.length === expectedOwnerIds.length) break;
  }
  const target = chart.locator('[data-org-chat]').first();
  const items = () => page.locator('.context-menu:popover-open .menu-action').allTextContents();
  await target.locator('.org-chart-actions-button').click();
  await page.waitForFunction(() => [...document.querySelectorAll('.context-menu:popover-open .menu-action')].some(row => row.textContent.includes('Open Chat')));
  const pointerMenuItems = await items();
  await page.keyboard.press('Escape');
  await target.focus();
  await page.keyboard.press('Shift+F10');
  await page.waitForFunction(() => [...document.querySelectorAll('.context-menu:popover-open .menu-action')].some(row => row.textContent.includes('Open Chat')));
  const keyboardMenuItems = await items();
  await page.keyboard.press('Escape');
  return { buttons, expectedOwnerIds, tabVisitedIds, pointerMenuItems, keyboardMenuItems };
}

async function liveUpdate(host) {
  const { chart, call, manifest, page } = host;
  const id = manifest.open;
  const measure = () => chart.locator('[data-org-chat]').evaluateAll(rows => Object.fromEntries(rows.map(row => {
    const rect = row.getBoundingClientRect();
    return [row.dataset.orgChat, { x: rect.x, y: rect.y, width: rect.width, height: rect.height }];
  })));
  const metadata = () => card(host, id).evaluate(row => ({ title: row.querySelector('.oc-chat-title').textContent,
    state: row.querySelector('.org-chart-state').textContent, updated: row.querySelector('time')?.getAttribute('datetime') || row.querySelector('time')?.textContent || '' }));
  const before = await metadata(), beforeRects = await measure();
  const control = await call('OrgChartHostFixtureControl', { kind: 'provider-live-update', chatId: id, title: 'An invented updated title long enough to wrap without moving another chat' });
  await page.waitForFunction(({ id, before }) => {
    const row = document.querySelector('[data-org-chat="' + CSS.escape(id) + '"]');
    return row && row.querySelector('.oc-chat-title').textContent !== before.title && row.querySelector('.org-chart-state').textContent !== before.state &&
      (row.querySelector('time')?.getAttribute('datetime') || row.querySelector('time')?.textContent || '') !== before.updated;
  }, { id, before });
  await settle(page);
  const after = await metadata(), afterRects = await measure();
  await page.screenshot({ path: path.join(host.out, 'live-update.png') });
  return { beforeTitle: before.title, afterTitle: after.title, beforeState: before.state, afterState: after.state,
    beforeUpdated: before.updated, afterUpdated: after.updated, beforeRects, afterRects, control };
}

async function links(host) {
  const { page, chart, manifest } = host;
  const team = manifest.expectedTeams[0];
  const group = manifest.expectedGroups[0];
  const labelChatId = group.memberIds[0];
  const rows = [
    { kind: 'team', locator: chart.locator('[data-org-team=' + JSON.stringify(team.id) + '] > .oc-team-head > .oc-team-name'), expected: team.id.replace(/^team:/, ''), itemKind: 'bus-room' },
    { kind: 'group-list', locator: chart.locator('[data-org-group=' + JSON.stringify(group.id) + '] .oc-group-name'), expected: group.id, itemKind: 'bus-room' },
    { kind: 'group-label', locator: card(host, labelChatId).locator('[data-org-group-label=' + JSON.stringify(group.id) + ']'), expected: group.id, itemKind: 'bus-room' },
    { kind: 'chat', locator: card(host, manifest.closed).locator('a.oc-chat-title'), expected: manifest.closed, itemKind: 'chat-session' }
  ];
  const result = [];
  for (const row of rows) {
    await chartFront(host);
    assert.equal(await row.locator.evaluate(link => link.tagName), 'A');
    await row.locator.focus();
    await page.keyboard.press('Enter');
    await page.waitForFunction(({ expected, itemKind }) => {
      const key = pudlWindows.state().top, item = key && dinahWorkbench.item(key);
      return item?.kind === itemKind && item.path === expected;
    }, { expected: row.expected, itemKind: row.itemKind });
    const inspect = () => page.evaluate(({ expected, itemKind }) => {
      const state = pudlWindows.state();
      const matches = state.open.filter(key => { const item = dinahWorkbench.item(key); return item?.kind === itemKind && item.path === expected; });
      const top = state.top && dinahWorkbench.item(state.top);
      return { count: matches.length, opened: top?.path, topKind: top?.kind };
    }, { expected: row.expected, itemKind: row.itemKind });
    const first = await inspect();
    await chartFront(host);
    await row.locator.click();
    await page.waitForFunction(({ expected, itemKind }) => { const key = pudlWindows.state().top; const item = key && dinahWorkbench.item(key); return item?.path === expected && item.kind === itemKind; }, { expected: row.expected, itemKind: row.itemKind });
    const second = await inspect();
    result.push({ kind: row.kind, expectedChatId: row.expected, openedChatId: first.opened,
      openWindowCount: first.count, afterSecondOpenWindowCount: second.count, secondOpenedChatId: second.opened });
  }
  return { links: result };
}

async function dialogs(host) {
  const { page, chart, call, manifest } = host;
  const panel = chart.locator('dialog.org-action');
  const ids = manifest.chats;
  const journeys = [];
  const definitions = [
    { action: 'assign', kind: 'assign_team', id: ids['member-a'] },
    { action: 'replace-lead', kind: 'replace_team_lead', id: ids['member-c'] },
    { action: 'create-team', kind: 'create_team', id: ids['free-a'] },
    { action: 'create-group', kind: 'create_group', id: ids['free-b'] },
    { action: 'start-chat', kind: 'start_chat', id: ids['free-b'] },
    { action: 'delete', kind: 'delete_chat', id: ids.closed }
  ];
  let snapshotIndex = 0;
  async function domain() {
    const raw = await call('OrgChartHostFixtureSnapshot');
    fs.writeFileSync(path.join(host.out, 'dialog-domain-' + (++snapshotIndex) + '.json'), JSON.stringify(raw, null, 2));
    const cleanup = structuredClone(raw.cleanup);
    // generatedAt records this query's observation time, not a persisted change.
    delete cleanup.generatedAt;
    return { ...require('./org-chart-host-scenarios.cjs').canonical(raw), cleanup };
  }
  const focus = () => page.evaluate(() => {
    const active = document.activeElement, row = active?.closest('[data-org-chat]');
    if (row) return (active.matches('.org-chart-actions-button') ? 'actions:' : active.matches('.oc-chat-title') ? 'title:' : 'box:') + row.dataset.orgChat;
    return active?.matches('.org-chart-scroll') ? 'chart-scroll' : active?.tagName || '';
  });
  for (const definition of definitions) {
    await chartFront(host);
    const offered = await call('OrgChartHostFixtureActions', definition.id);
    const action = offered.actions.find(action => action.kind === definition.kind);
    assert.ok(action, 'The actual host offers ' + definition.kind);
    const openerId = await card(host, definition.id).locator('.org-chart-actions-button').evaluate(button => 'actions:' + button.closest('[data-org-chat]').dataset.orgChat);
    async function open() {
      await card(host, definition.id).locator('.org-chart-actions-button').click();
      const menu = page.locator('.context-menu:popover-open .menu-action');
      await menu.filter({ hasText: action.label }).click();
      await panel.getByRole('heading', { name: action.label, exact: true }).waitFor();
    }
    async function dismiss(key) {
      const before = await domain();
      await open();
      await page.screenshot({ path: path.join(host.out, definition.action + '-' + key + '.png') });
      if (key === 'cancel') await panel.getByRole('button', { name: 'Cancel', exact: true }).click();
      else await page.keyboard.press('Escape');
      await page.waitForFunction(() => !document.querySelector('.org-action').open);
      return { before, after: await domain(), focusId: await focus() };
    }
    const cancel = await dismiss('cancel'), escape = await dismiss('escape');
    await open();
    let subjectTitle = offered.title;
    if (definition.kind === 'assign_team') {
      const target = action.chats.find(row => row.id === ids['lead-b']);
      assert.ok(target, 'The fixture has another eligible team.');
      await panel.getByLabel('Team Lead', { exact: true }).selectOption(target.id);
    } else if (definition.kind === 'create_team' || definition.kind === 'create_group') {
      subjectTitle = 'Invented receiving ' + definition.action;
      await panel.getByLabel('Name', { exact: true }).fill(subjectTitle);
      if (definition.kind === 'create_team') await panel.getByLabel('Lead', { exact: true }).selectOption(ids['free-a']);
      const memberId = definition.kind === 'create_team' ? ids['free-b'] : ids['closed'];
      const member = action.chats.find(row => row.id === memberId);
      assert.ok(member, 'The actual host offers the intended member.');
      await panel.locator('fieldset label').filter({ hasText: member.title }).locator('input').check();
    } else if (definition.kind === 'start_chat') {
      assert.ok(action.workspaces.length && action.providers.length);
      await panel.getByLabel('Workspace', { exact: true }).selectOption(action.workspaces[0].id);
      await panel.getByLabel('Agent Program', { exact: true }).selectOption(action.providers[0].id);
    }
    await panel.locator('button[type=submit]').click();
    await page.waitForFunction(() => !document.querySelector('.org-action').open);
    // A new chat's title comes from the real provider response and status line.
    if (definition.kind === 'start_chat') {
      const after = await call('OrgChartHostFixtureSnapshot');
      const created = after.view.chats.filter(row => !Object.values(ids).includes(row.chatId));
      assert.equal(created.length, 1, 'Start Chat creates one actual inventory row.');
      subjectTitle = created[0].title || 'Untitled chat';
    }
    const status = chart.locator('.org-chart-status');
    journeys.push({ action: definition.action, openerId, cancel, escape, subjectTitle,
      survivingFocusId: definition.kind === 'delete_chat' ? 'chart-scroll' : openerId,
      success: { focusId: await focus(), openDialogCount: await page.locator('dialog.org-action[open]').count(),
        statusRole: await status.getAttribute('role'), statusText: await status.textContent() } });
    await page.screenshot({ path: path.join(host.out, definition.action + '-success.png') });
  }
  return { journeys };
}

exports.run = async function run(host, scenario) {
  const handlers = { structure, actions, links, dialogs, 'live-update': liveUpdate };
  if (!handlers[scenario]) throw Error('The rendered receiving scenario is unbound: ' + scenario);
  const result = await handlers[scenario](host);
  fs.writeFileSync(path.join(host.out, 'rendered-observations.json'), JSON.stringify(result, null, 2));
  return result;
};
exports.settle = settle;
exports.chartFront = chartFront;
