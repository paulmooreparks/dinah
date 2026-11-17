// A lead whose members have teams of their own has those teams side by side
// in the Org Chart, each read down by itself, with a line from the lead that
// runs above them and drops to each. A team of chats with no teams of their
// own stays a stack. Stacked, the second team under a lead began far below
// the lead and the chart was one tall column (Paul, 17 November 2026).
const { launch } = require('./lib/page');
(async () => {
  const chat = (id, title, lead) => ({ chatId: id, head: id, title, workspace: 'C:/Harbor', workspaceName: 'Harbor', provider: 'codex', providerName: 'Codex', state: { life: 'open', exec: { state: 'idle', why: '' } }, resolution: 'resolved', reason: '' });
  const model = { generation: 'branches:1', generatedAt: '2026-11-17T06:00:00Z',
    chats: [chat('top', 'Customer Lead'), chat('a', 'Checkout Lead'), chat('a1', 'Checkout Review'), chat('a2', 'Checkout Author'), chat('a3', 'Checkout Research'), chat('b', 'Keyboard Lead'), chat('b1', 'Keyboard Verifier'), chat('b2', 'Keyboard Author'), chat('s', 'Docs Lead'), chat('s1', 'Docs Author'), chat('s2', 'Docs Editor')],
    teams: [
      { id: 'top-team', title: 'Customer Team', leadId: 'top', memberIds: ['a', 'b'], resolution: 'resolved', reason: '' },
      { id: 'team-a', title: 'Checkout', leadId: 'a', memberIds: ['a1', 'a2', 'a3'], resolution: 'resolved', reason: '' },
      { id: 'team-b', title: 'Keyboard', leadId: 'b', memberIds: ['b1', 'b2'], resolution: 'resolved', reason: '' },
      { id: 'team-s', title: 'Docs', leadId: 's', memberIds: ['s1', 's2'], resolution: 'resolved', reason: '' },
    ], groups: [] };
  const t = await launch({ viewport: { width: 1600, height: 1000 }, args: model, ready: 'workbench', app: value => {
    window.__orgModel = value;
    return { SavedChats: async () => ({ generation: __orgModel.generation, chats: __orgModel.chats.map(row => ({ ...row, sessionId: row.head, cwd: row.workspace, chatState: row.state })) }), OrgChart: async () => JSON.parse(JSON.stringify(window.__orgModel)),
      OrgChartActions: async chatId => ({ chatId, title: 'Chat', actions: [], unavailable: [] }) };
  } });
  const { page: p, ok } = t;
  try {
    const key = await p.evaluate(() => dinahWorkbench.open('org-chart'));
    const chart = p.locator('[data-win="' + key + '"] .org-chart');
    await chart.locator('[data-org-chat="b2"]').waitFor();
    await p.evaluate(() => document.fonts.ready);
    const box = id => chart.locator('[data-org-chat="' + id + '"]').evaluate(el => { const r = el.getBoundingClientRect(); return { left: Math.round(r.left), right: Math.round(r.right), top: Math.round(r.top), bottom: Math.round(r.bottom) }; });
    const team = id => chart.locator('[data-org-team="' + id + '"]').evaluate(el => { const r = el.getBoundingClientRect(); return { left: Math.round(r.left), right: Math.round(r.right), top: Math.round(r.top), bottom: Math.round(r.bottom) }; });
    const [top, a, b, a1, a3, b1, s, s1, s2] = await Promise.all(['top', 'a', 'b', 'a1', 'a3', 'b1', 's', 's1', 's2'].map(box));
    const [teamA, teamB] = await Promise.all(['team-a', 'team-b'].map(team));
    ok('the two teams under one lead start at the same height, below the lead', teamA.top === teamB.top && teamA.top > top.bottom, JSON.stringify({ top, teamA, teamB }));
    ok('the second team is beside the first, and they do not overlap', teamB.left >= teamA.right, JSON.stringify({ teamA, teamB }));
    ok('each team is read down by itself: its lead, then its members', a.top < a1.top && a1.top < a3.top && a1.left === a3.left && b.top < b1.top && b1.left > a1.right, JSON.stringify({ a, a1, a3, b, b1 }));
    ok('the second team begins beside the first one\'s lead, not below its last member', b.top === a.top && b.top < a3.top, JSON.stringify({ a, b, a3 }));
    ok('a team of chats with no teams of their own stays a stack', s1.top > s.bottom && s2.top > s1.bottom && s1.left === s2.left, JSON.stringify({ s, s1, s2 }));
    // The lines: one comes down from the lead and along to the first team, each team has its drop, and the first reaches along to the second.
    const lines = await chart.locator('[data-org-team="top-team"] > .oc-members').evaluate(list => {
      const style = (el, pseudo) => { const s = getComputedStyle(el, pseudo); return { content: s.content, left: s.borderLeftWidth, top: s.borderTopWidth, bottom: s.borderBottomWidth, width: Math.round(parseFloat(s.width)), height: Math.round(parseFloat(s.height)) }; };
      const items = [...list.children];
      return { row: list.classList.contains('oc-members-row'), fromLead: style(list, '::before'), drops: items.map(li => style(li, '::before')), across: style(items[0], '::after'), lastAcross: style(items[items.length - 1], '::after').content, gap: Math.round(items[1].getBoundingClientRect().left - items[0].getBoundingClientRect().right), firstWidth: Math.round(items[0].getBoundingClientRect().width) };
    });
    ok('a line comes down from the lead and turns along to the first team', lines.row && lines.fromLead.left === '2px' && lines.fromLead.bottom === '2px' && lines.fromLead.height > 0, JSON.stringify(lines.fromLead));
    ok('each team has a line down to it', lines.drops.length === 2 && lines.drops.every(d => d.left === '2px' && d.height > 0 && d.bottom === '0px'), JSON.stringify(lines.drops));
    ok('the line runs along from the first team to the second and stops there', lines.across.top === '2px' && lines.across.width === lines.firstWidth + lines.gap && lines.lastAcross === 'none', JSON.stringify(lines));
    await t.shot('org-chart-branches', { clip: await chart.boundingBox() });
    // Folding the lead's team takes its row away, and unfolding brings it back as it was.
    await chart.locator('[data-org-fold="top-team"]').click();
    ok('folding the lead\'s team hides the teams under it', await chart.locator('[data-org-team="team-a"]').count() === 0 || !(await chart.locator('[data-org-team="team-a"]').isVisible()));
    await chart.locator('[data-org-fold="top-team"]').click();
    await chart.locator('[data-org-chat="b2"]').waitFor();
    const again = await Promise.all(['team-a', 'team-b'].map(team));
    ok('unfolding puts them side by side again', again[0].top === again[1].top && again[1].left >= again[0].right);
    // A chart narrower than the row keeps the row and scrolls sideways; the row never wraps onto a second line.
    await chart.evaluate(el => { el.style.maxWidth = '360px'; });
    await p.evaluate(() => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r))));
    const narrow = await Promise.all(['team-a', 'team-b'].map(team));
    ok('in a chart narrower than the row the teams stay side by side and the chart scrolls sideways', narrow[0].top === narrow[1].top && narrow[1].left >= narrow[0].right && await chart.locator('.org-chart-scroll').evaluate(el => el.scrollWidth > el.clientWidth));
  } finally { await t.close(); }
})().catch(error => { console.error(error); process.exitCode = 1; });
