// The Workspace pane's toolbar: the chooser shows the current workspace as
// the menu bar does, and the Actions menu opens against its button with the
// workspace commands as menu rows.
const { launch } = require('./lib/page');
(async () => {
  const ws = 'C:\\Work\\parser';
  const t = await launch({
    viewport: { width: 1280, height: 820 },
    state: { workspace: ws, recent: [ws, 'C:\\Work\\Website'], approvals: 'dinah', devin: 'd', titles: {} },
    app: () => ({ CreateWorkspace: async () => ({}), RenameWorkspace: async () => ({}) }),
  });
  const p = require('./lib/windows')(t.page);
  try {
    await p.waitForTimeout(400);
    const chosen = await p.$eval('#ws-select', s => ({ text: s.selectedOptions[0].textContent, sunken: s.classList.contains('form-select') }));
    t.ok('the chooser is a select showing the workspace the menu bar shows', chosen.sunken && chosen.text === await p.textContent('#status-workspace-name'), JSON.stringify(chosen));
    await p.click('#ws-actions-button');
    await p.waitForTimeout(200);
    const r = await p.evaluate(() => {
      const m = document.getElementById('ws-actions'), x = document.getElementById('ws-actions-button').getBoundingClientRect(), box = m.getBoundingClientRect();
      return { dx: Math.round(Math.abs(box.right - x.right) < Math.abs(box.left - x.left) ? Math.abs(box.right - x.right) : Math.abs(box.left - x.left)), below: Math.round(box.top - x.bottom),
        rows: Array.from(m.children, c => c.className + ':' + c.textContent) };
    });
    t.ok('the Actions menu opens under its button', r.dx < 40 && r.below >= 0 && r.below < 20, JSON.stringify(r));
    t.ok('the workspace commands are menu rows', r.rows.join('|') === 'menu-action:New Empty Workspace|menu-action:Rename Workspace…|menu-action:Export Workspace…|menu-action:Import Workspace…|menu-action:Refresh Workspace|menu-action:Remove Workspace…');
    const pane = await p.evaluate(() => Array.from(document.querySelectorAll('.ws-toolbar .btn'), x => x.textContent));
    t.ok('the toolbar has New Workspace… and Actions as raised buttons', pane.join('|') === 'New Workspace…|Actions', pane.join('|'));
    await t.shot('wsmenu-side').catch(() => {});
  } finally { await t.close(); }
})();
