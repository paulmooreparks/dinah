const fs = require('fs');
const path = require('path');
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 900 }, app: () => ({ Team: async () => window.__auditTeam,
    BusOverview: async () => ({ inbox: [], unread: 0, rooms: window.__auditRooms || [], workspaces: [], links: [] }) }) });
  try {
    const results = await t.page.evaluate(async () => {
      const results = [], settle = () => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
      async function measure(root, action) {
        let mutations = 0;
        const observer = new MutationObserver(rows => { mutations += rows.length; });
        observer.observe(root, { subtree: true, childList: true, attributes: true, characterData: true });
        const start = performance.now(); action(); const synchronousMs = performance.now() - start;
        await settle(); mutations += observer.takeRecords().length; observer.disconnect();
        return { synchronousMs, mutations };
      }
      for (const count of [10, 150, 500]) {
        window.__auditRooms = Array.from({ length: count }, (_, i) => ({ name: 'audit-room-' + i, title: 'Group ' + i, members: [], unread: 0 }));
        await dinahBus.load(); await dinahWorkbench.open('bus'); await settle();
        const busRoot = document.querySelector('.bus-browser'), groupSamples = [];
        const enqueue = dinahRender.enqueue; let drawMs = 0;
        dinahRender.enqueue = function (key, element, draw, after, label) {
          return enqueue(key, element, function (...args) { const start = performance.now(); const result = draw(...args); if (element === busRoot) drawMs += performance.now() - start; return result; }, after, label);
        };
        for (let i = 0; i < 5; i++) {
          drawMs = 0;
          const sample = await measure(busRoot, () => document.dispatchEvent(new Event('dinah:execution-state')));
          groupSamples.push({ drawMs, mutations: sample.mutations });
        }
        dinahRender.enqueue = enqueue;
        results.push({ kind: 'group-chats', count, samples: groupSamples });
        window.__auditTeam = { lead: { chat: 'audit-lead' }, group: 'audit-team', members: Array.from({ length: count }, (_, i) => ({ bus: 'audit-' + i, chat: 'audit-' + i, title: 'Member ' + i, life: 'open', window: false, exec: { state: 'idle', why: 'No work in progress' }, members: [], provider: 'codex', model: 'fixture', workspace: 'Fixture' })) };
        const panel = dinahTeams.reportPanel(() => 'audit-lead', () => {});
        document.body.appendChild(panel); panel.showPopover(); await settle();
        const samples = [];
        for (let i = 0; i < 5; i++) samples.push(await measure(panel, () => __emit('team:state', structuredClone(__auditTeam))));
        results.push({ kind: 'team-report', count, samples });
        panel.hidePopover(); panel.remove();
        const container = document.createElement('div'); document.body.appendChild(container);
        const store = Notices.create();
        for (let i = 0; i < count; i++) store.add({ id: 'notice-' + i, title: 'Notice ' + i, type: 'info', body: 'Audit fixture' });
        const view = store.mountPane(container), noticeSamples = [];
        await settle();
        for (let i = 0; i < 5; i++) noticeSamples.push(await measure(container, () => view.refresh()));
        results.push({ kind: 'notifications', count, samples: noticeSamples });
        view.destroy(); container.remove();
      }
      return results;
    });
    console.log('AUDIT ' + JSON.stringify(results));
    fs.writeFileSync(path.join(process.env.E2E_OUT || process.argv[3], 'list-render-audit.json'), JSON.stringify(results, null, 2));
    t.ok('group, team and notification audit completed at each catalog size', results.length === 9);
    t.ok('unchanged lists perform no DOM writes', results.every(r => r.samples.every(s => s.mutations === 0)));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
