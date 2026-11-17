// The Chats list and the Org Chart show the model a chat uses after the
// program's name, as the host reports it (Info.Model), and change it in place
// when the model changes, with nothing growing or moving. An Org Chart card
// says its chat's workspace, after a dot on the name row, only when the card is
// drawn in another workspace's box (Paul, 17 November 2026).
const { launch } = require('./lib/page');
(async () => {
  const state = { life: 'open', exec: { state: 'idle', why: 'No work in progress' } };
  const chat = (id, title, workspace, model) => ({ chatId: id, head: id, title, workspace, workspaceName: workspace.split('/').pop(), provider: 'codex', providerName: 'Codex', model, state, resolution: 'resolved', reason: '' });
  const model = { generation: 'model:1', generatedAt: '2026-11-17T06:00:00Z',
    chats: [chat('top', 'Harbor Lead', 'C:/Harbor', 'Sol'), chat('a', 'Harbor Author', 'C:/Harbor', 'Luna'), chat('x', 'Docks Helper', 'C:/Docks', 'Sol'), chat('f', 'Docks Free Agent', 'C:/Docks', '')],
    teams: [{ id: 'top-team', title: 'Harbor Team', leadId: 'top', memberIds: ['a', 'x'], resolution: 'resolved', reason: '' }], groups: [] };
  const t = await launch({ viewport: { width: 1600, height: 1000 }, args: model, ready: 'workbench', app: value => {
    window.__orgModel = value;
    return { SavedChats: async () => ({ generation: __orgModel.generation, chats: __orgModel.chats.map(row => ({ ...row, sessionId: row.head, cwd: row.workspace, chatState: row.state, updatedAt: '2026-11-17T06:00:00Z' })) }),
      OrgChart: async () => JSON.parse(JSON.stringify(window.__orgModel)),
      OrgChartActions: async chatId => ({ chatId, title: 'Chat', actions: [], unavailable: [] }) };
  } });
  const { page: p, ok } = t;
  try {
    // The Chats list, showing every workspace.
    await p.evaluate(() => dinahWorkbench.open('chat-browser'));
    await p.evaluate(() => { const scope = document.getElementById('chat-scope'); scope.value = 'all'; scope.dispatchEvent(new Event('change', { bubbles: true })); });
    const row = id => p.locator('#sessions [data-session="' + id + '"], #sessions [data-session-id="' + id + '"]').first();
    const metaOf = async title => p.evaluate(text => { const a = [...document.querySelectorAll('#sessions .md-item')].find(el => el.querySelector('.session-title')?.textContent === text); return a ? { meta: a.querySelector('.md-meta').textContent, tip: a.querySelector('.md-meta').title, label: a.title, height: Math.round(a.getBoundingClientRect().height) } : null; }, title);
    await p.waitForFunction(() => [...document.querySelectorAll('#sessions .session-title')].some(el => el.textContent === 'Harbor Lead'));
    const lead = await metaOf('Harbor Lead'), free = await metaOf('Docks Free Agent');
    ok('a chat in the Chats list has its model after the program', /^Codex · Sol · /.test(lead.meta) && lead.tip === lead.meta && lead.label.includes('Codex · Sol'), JSON.stringify(lead));
    ok('a chat with no model has the program alone', /^Codex · /.test(free.meta) && !/Sol|Luna/.test(free.meta), JSON.stringify(free));

    // The Org Chart.
    const key = await p.evaluate(() => dinahWorkbench.open('org-chart'));
    const chart = p.locator('[data-win="' + key + '"] .org-chart');
    await chart.locator('[data-org-chat="f"]').waitFor();
    await p.evaluate(() => document.fonts.ready);
    const card = id => chart.locator('[data-org-chat="' + id + '"]').evaluate(box => {
      const where = box.querySelector('.oc-chat-where'), name = box.querySelector('.oc-chat-name');
      return { where: where.hidden ? '' : where.textContent, whereTip: where.title, meta: box.querySelector('.org-chart-meta').textContent, label: box.getAttribute('aria-label'),
        nameHeight: Math.round(name.getBoundingClientRect().height), height: Math.round(box.getBoundingClientRect().height), titleRight: Math.round(box.querySelector('.oc-chat-title').getBoundingClientRect().right), boxRight: Math.round(box.getBoundingClientRect().right) };
    });
    const [top, a, x, f] = await Promise.all(['top', 'a', 'x', 'f'].map(card));
    ok('a card has the program and the model on the line under its role', top.meta === 'Codex, Sol' && a.meta === 'Codex, Luna' && x.meta === 'Codex, Sol', JSON.stringify({ top, a, x }));
    ok('a card with no model has the program alone', f.meta === 'Codex', JSON.stringify(f));
    ok('a card in its own workspace\'s box says no workspace', top.where === '' && a.where === '' && f.where === '', JSON.stringify({ top, a, f }));
    ok('a member from another workspace says its workspace after a dot on the name row', x.where === ' · Docks' && /Docks/.test(x.whereTip) && /Docks Helper, in Docks\./.test(x.label), JSON.stringify(x));
    ok('the name row is one line high with or without the workspace, and so are the cards', x.nameHeight === a.nameHeight && x.height === a.height, JSON.stringify({ a, x }));
    // A very long title and workspace are both cut, and the row never runs past the card.
    await p.evaluate(() => { __orgModel.chats[2].title = 'Docks Helper with a very long title that will not fit'; __orgModel.chats[2].workspace = 'C:/A workspace with a long name'; __orgModel.chats[2].workspaceName = 'A workspace with a long name'; __orgModel.generation = 'model:2'; });
    await p.evaluate(() => window.dinahChats.refreshInventory());
    await p.waitForFunction(() => /long title/.test(document.querySelector('[data-org-chat="x"] .oc-chat-title')?.textContent || ''));
    const long = await card('x');
    ok('a long title and workspace are cut within the card, and the card keeps its height', long.where.startsWith(' · A workspace') && long.nameHeight === a.nameHeight && long.height === a.height && long.titleRight <= long.boxRight, JSON.stringify(long));
    await t.shot('chat-model-org-chart', { clip: await chart.boundingBox() });

    // A change of model reaches both places in place.
    const before = { list: await metaOf('Harbor Lead'), card: await card('top') };
    await p.evaluate(() => {
      const c = __orgModel.chats[0];
      window.__emit('agent:session', { sessionId: 'top', provider: 'codex', cwd: c.workspace, title: c.title, open: true, working: false, queued: [], chatState: c.state, model: 'Luna', stateGen: 'model-test', stateVer: 1 });
    });
    await p.waitForFunction(() => document.querySelector('[data-org-chat="top"] .org-chart-meta')?.textContent === 'Codex, Luna');
    const after = { list: await metaOf('Harbor Lead'), card: await card('top') };
    ok('a change of model is shown in the Chats list and on the card', /^Codex · Luna · /.test(after.list.meta) && after.card.meta === 'Codex, Luna', JSON.stringify(after));
    ok('nothing grows when the model changes', after.list.height === before.list.height && after.card.height === before.card.height, JSON.stringify({ before, after }));
    // Information that has no model now takes the model away.
    await p.evaluate(() => {
      const c = __orgModel.chats[0];
      window.__emit('agent:session', { sessionId: 'top', provider: 'codex', cwd: c.workspace, title: c.title, open: true, working: false, queued: [], chatState: c.state, stateGen: 'model-test', stateVer: 2 });
    });
    await p.waitForFunction(() => document.querySelector('[data-org-chat="top"] .org-chart-meta')?.textContent === 'Codex');
    const gone = await metaOf('Harbor Lead');
    ok('a chat whose information has no model any more shows none', /^Codex · /.test(gone.meta) && !/Luna|Sol/.test(gone.meta), JSON.stringify(gone));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
