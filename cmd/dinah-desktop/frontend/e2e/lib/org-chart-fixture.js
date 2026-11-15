const { launch } = require('./page');
function model() {
  const chats = ['mara', 'ivo', 'nina', 'lena', 'oren', 'tess', 'sasha', 'bo', 'cal', 'uma', 'repair'].map((id, index) => ({
    chatId: id, head: id === 'repair' ? '' : id, title: id[0].toUpperCase() + id.slice(1), workspace: index < 6 ? 'C:/Harbor' : 'C:/Orchard', workspaceName: index < 6 ? 'Harbor' : 'Orchard',
    provider: 'codex', providerName: 'Codex', state: { life: 'open', exec: { state: index % 3 === 0 ? 'working' : index % 3 === 1 ? 'idle' : 'waiting', why: index % 3 === 2 ? 'A question needs your answer.' : '' } }, resolution: id === 'repair' ? 'repair' : 'resolved', reason: id === 'repair' ? 'The chat owner could not be identified.' : ''
  }));
  return { generation: 'fixture:1', generatedAt: '2026-11-15T06:00:00Z', chats,
    teams: [
      { id: 'platform', title: 'Platform', leadId: 'mara', memberIds: ['ivo', 'nina'], resolution: 'resolved', reason: '' },
      { id: 'delivery', title: 'Delivery', leadId: 'lena', memberIds: ['oren', 'tess'], resolution: 'resolved', reason: '' },
      { id: 'research', title: 'Research', leadId: 'sasha', memberIds: ['bo'], resolution: 'resolved', reason: '' },
      { id: 'repair-team', title: 'Unresolved team', leadId: 'repair', memberIds: [], resolution: 'repair', reason: 'The team lead could not be identified.' }
    ], groups: [{ id: 'review', title: 'Release Review', ownerId: 'lena', memberIds: ['lena', 'nina', 'tess'], resolution: 'resolved', reason: '' }, { id: 'research-group', title: 'Research Exchange', ownerId: '', memberIds: ['cal', 'bo'], resolution: 'resolved', reason: '' }] };
}
async function start() {
  const t = await launch({ viewport: { width: 1440, height: 1000 }, args: model(), ready: 'workbench', app: value => {
    window.__orgModel = value; window.__orgReads = 0;
    return { OrgChart: async () => { window.__orgReads++; if (window.__orgGate) await window.__orgGate; if (window.__orgError) throw new Error(window.__orgError); return JSON.parse(JSON.stringify(window.__orgModel)); }, OrgChartActions: async chatId => ({ chatId, title: 'Chat', actions: [], unavailable: ['assign_team','make_lead','create_team','create_group','start_chat','delete_chat'].map(kind => ({ kind, reason: 'Unavailable in this fixture.' })) }) };
  } });
  t.key = await t.page.evaluate(() => dinahWorkbench.open('org-chart', 'fold=research'));
  t.chart = t.page.locator('[data-win="' + t.key + '"] .org-chart');
  await t.page.waitForFunction(() => document.querySelector('[data-org-chat="nina"]') || /could not|not available|invalid|must|unknown/i.test(document.querySelector('.org-chart [role="status"]')?.textContent || ''));
  if (!await t.chart.locator('[data-org-chat="nina"]').count()) { const error = await t.chart.getByRole('status').innerText(); await t.close(); throw new Error(error); }
  await t.page.evaluate(() => document.fonts.ready);
  return t;
}
module.exports = { model, start };
