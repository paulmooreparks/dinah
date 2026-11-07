const fs = require('fs');
const path = require('path');
const { launch } = require('./lib/page');
(async () => {
  const t = await launch({ viewport: { width: 1440, height: 900 } });
  try {
    const results = await t.page.evaluate(async () => {
      const settle = () => new Promise(r => requestAnimationFrame(() => requestAnimationFrame(r)));
      const results = [];
      const ids = ['workspace-source-tree', 'workspace-sources', 'workspace-source-choose'];
      const originals = ids.map(id => document.getElementById(id));
      originals.forEach((el, i) => { if (el) el.id = ids[i] + '-original'; });
      const container = document.createElement('div');
      container.innerHTML = '<div id="workspace-sources"><button id="workspace-source-choose"></button><div id="workspace-source-tree"></div></div>';
      document.body.append(container);
      const createTree = dinahResources.createTree;
      try {
        for (const count of [10, 150, 500]) {
          let changed;
          const workspace = { children: Array.from({ length: count }, (_, i) => ({ title: 'File ' + i, path: '/workspaces/a/s/file' + i, ref: { workspaceId: 'audit', sourceId: 's', path: 'file' + i }, actions: ['preview'], directory: false })) };
          dinahResources.createTree = () => ({ lookup: async () => workspace, load: async () => {}, resolve: () => workspace, onChange: fn => { changed = fn; }, destroy() {} });
          const view = dinahWorkspaceBrowser.mount({ api: () => ({ ListWorkspaceMounts() {} }), fail: e => { throw e; } });
          view.setWorkspace({ id: 'audit', mountAlias: 'a', sources: [{ id: 's', mountAlias: 's', name: 'Source' }] });
          await settle();
          const root = document.getElementById(ids[0]);
          const first = root.querySelector('[data-resource]'); first.focus();
          const samples = [];
          for (let i = 0; i < 5; i++) {
            let mutations = 0, drawMs = 0;
            const observer = new MutationObserver(rows => { mutations += rows.length; });
            observer.observe(root, { subtree: true, childList: true, attributes: true, characterData: true });
            const enqueue = dinahRender.enqueue;
            dinahRender.enqueue = (key, el, draw, ...args) => enqueue(key, el, () => { const start = performance.now(); draw(); drawMs += performance.now() - start; }, ...args);
            changed(); await settle(); dinahRender.enqueue = enqueue;
            mutations += observer.takeRecords().length; observer.disconnect();
            samples.push({ drawMs, mutations });
          }
          const retained = root.querySelector('[data-resource]') === first && document.activeElement === first;
          workspace.children[0].title = 'Renamed'; changed(); await settle();
          const renamed = root.querySelector('.fm-ti-label').textContent === 'Renamed';
          results.push({ count, samples, retained, renamed });
          view.destroy();
        }
      } finally {
        dinahResources.createTree = createTree; container.remove();
        originals.forEach((el, i) => { if (el) el.id = ids[i]; });
      }
      return results;
    });
    fs.writeFileSync(path.join(process.env.E2E_OUT || process.argv[3], 'workspace-render-audit.json'), JSON.stringify(results, null, 2));
    t.ok('workspace audit covers all sizes and updates labels', results.length === 3 && results.every(r => r.renamed));
    if (!process.env.AUDIT_BASELINE) t.ok('unchanged workspace trees preserve focus and perform no writes', results.every(r => r.retained && r.samples.every(s => s.mutations === 0)));
  } finally { await t.close(); }
})().catch(e => { console.error(e); process.exitCode = 1; });
