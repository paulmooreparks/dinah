'use strict';
// Product names, declared command names and executable locations belong in the
// private receiving config. Each product supplies host, peer and commandProgram.
// Run the independent suite with QUALITY_ORG_CHART_BINDING set to this file,
// ORG_CHART_RECEIVING_CONFIG set to a JSON map and ORG_CHART_RECEIVING_OUT to a
// fresh private directory. The caller owns the shared test lock.
const fs = require('node:fs');
const path = require('node:path');
const { open } = require('./lib/org-chart-real-host.cjs');
const rendered = require('./lib/org-chart-rendered.cjs');
let serial = 0;
exports.open = async function receiving(view) {
  const configs = JSON.parse(fs.readFileSync(process.env.ORG_CHART_RECEIVING_CONFIG));
  const config = configs[view.product];
  if (!config) throw Error('The requested product has no explicit receiving executable.');
  const root = process.env.ORG_CHART_RECEIVING_OUT;
  if (!root) throw Error('A private receiving output directory is required.');
  fs.mkdirSync(root, { recursive: true });
  const out = path.join(root, String(++serial).padStart(3, '0') + '-' + view.scenario + '-' + (view.width || 'host') + '-' + (view.theme || 'default'));
  const host = await open(config, out, view);
  if (host.manifest.product !== view.product) {
    await host.close();
    throw Error('The actual host product does not match the requested product.');
  }
  return {
    close: host.close,
    run: async () => {
      const isRendered = ['structure', 'links', 'actions', 'dialogs', 'live-update'].includes(view.scenario);
      let observations;
      if (isRendered) observations = await rendered.run(host, view.scenario);
      else {
        const { runHostScenario } = require('./lib/org-chart-host-scenarios.cjs');
        observations = await runHostScenario({ call: host.call, product: view.product, scenario: view.scenario, artifactDirectory: out, browserErrors: host.errors,
          visibleReason: async ({ surface, id, kind }) => {
            if (surface === 'cleanup') {
              await host.page.evaluate(() => dinahChatCleanup.open());
              const row = host.page.locator('.chat-cleanup [data-chat-id=' + JSON.stringify(id) + ']');
              await row.waitFor();
              return row.innerText();
            }
            await rendered.chartFront(host);
            const row = host.chart.locator('[data-org-chat=' + JSON.stringify(id) + ']');
            await row.locator('.org-chart-actions-button').click();
            const label = { assign_team: 'Assign to a Team…', delete_chat: 'Delete Chat…' }[kind];
            if (!label) throw Error('The unavailable action has no rendered reason selector.');
            const unavailable = host.page.locator('.context-menu:popover-open .menu-action').filter({ hasText: label });
            await unavailable.waitFor();
            const text = await unavailable.innerText();
            await host.page.keyboard.press('Escape');
            return text;
          } });
      }
      const snapshot = isRendered ? await host.call('OrgChartHostFixtureSnapshot') : null;
      const result = { ...observations, product: view.product, scenario: view.scenario, artifactDirectory: out,
        browserErrors: [...host.errors], remoteWrites: isRendered ? snapshot.remoteWrites : observations.remoteWrites };
      fs.writeFileSync(path.join(out, 'observations.json'), JSON.stringify(result, null, 2));
      return result;
    }
  };
};
