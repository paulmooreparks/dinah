const { launch, OUT } = require('./page');
const fs = require('fs');
const path = require('path');

function fixture() {
  const installed = [
    { name: 'Desktop', file: 'desktop.exe', path: 'C:/Fixture/bin/desktop.exe', version: '2.0.0' },
    { name: 'Command line', file: 'command.exe', path: 'C:/Fixture/bin/command.exe', version: '2.0.0' },
    { name: 'Agent tools', file: 'tools.exe', path: 'C:/Fixture/bin/tools.exe', version: '2.0.0+source.abcdef' }
  ];
  return { release: { current: '2.0.0', latest: '2.0.0', available: false },
    status: { installed, running: installed.map((p, i) => ({ pid: 100 + i, path: p.path, version: p.version, current: true, startedBy: i === 2 ? 'external-agent.exe' : 'desktop.exe' })), behind: 0 },
    installed: [{ name: 'Writing skills', version: '1.0.0', target: 'me' }, { name: 'Project skills', version: '1.0.0', target: 'workspace', workspace: 'C:/Fixture' }],
    packages: { installed: [], updates: [] }, sources: 1,
    preview: { token: 'preview-1', programs: ['C:/Fixture/bin/tools.exe'], lines: ['The owned earlier entry would use C:/Fixture/bin/tools.exe.'], stale: 1, changed: 0, failed: false, findings: [
      { agent: 'Codex', server: 'fixture-tools', file: 'C:/Fixture/client-a', state: 'stale', runs: 'C:/Fixture/earlier/tools.exe' },
      { agent: 'Devin', server: 'fixture-tools', file: 'C:/Fixture/client-b', state: 'current', runs: 'C:/Fixture/bin/tools.exe' },
      { agent: 'Claude', server: 'fixture-tools', file: 'C:/Fixture/client-c', project: 'C:/Project', state: 'kept', runs: 'C:/Custom/tools.exe' }
    ] }
  };
}
async function start(changes = {}, openUpdates = true) {
  const args = Object.assign(fixture(), changes);
  args.packages.installed = args.installed;
  const t = await launch({ args, app: data => {
    const clone = x => JSON.parse(JSON.stringify(x));
    window.__u = clone(data); window.__updateCalls = []; window.__hold = {}; window.__pending = {}; window.__fail = {};
    async function finish(name, value) {
      const snapshot = clone(value);
      if (window.__hold[name]) await new Promise(resolve => { (window.__pending[name] ||= []).push(resolve); });
      if (window.__fail[name]) throw new Error(window.__fail[name]);
      return snapshot;
    }
    return {
      ProductServices: data.missingDiscovery ? undefined : async () => {
        window.__serviceChecks = (window.__serviceChecks || 0) + 1;
        if (window.__u.discoveryError) throw new Error('Synthetic discovery failure');
        return window.__u.services || (window.__u.noService ? [] : ['updates']);
      },
      State: async () => ({ workspace: 'C:/Fixture', recent: ['C:/Fixture'], catalog: [], defaults: {}, pinned: [], packageSources: window.__u.sources }),
      UpdateState: async () => clone(window.__u.restart || {ready:false,pending:false,working:0,writing:0}),
      CallService: async (service, name, input) => {
        const values = JSON.parse(input); window.__updateCalls.push([name, ...values]);
        if (service !== 'updates') throw new Error('Unexpected fixture service');
        if ((window.__u.unsupported || []).includes(name)) throw new Error('This service has no method ' + name);
        let result;
        if (name === 'Status') result = await finish(name, window.__u.status);
        else if (name === 'Check' || name === 'State') result = await finish(name, window.__u.release);
        else if (name === 'TakeResult') result = {};
        else if (name === 'Download') {
          if (!window.__u.release.downloading) { window.__u.release.downloading = true; window.__u.downloads = (window.__u.downloads || 0) + 1; }
          result = await finish(name, window.__u.release);
        } else if (name === 'PreviewAgentSettings') result = await finish(name, window.__u.preview);
        else if (name === 'RepairAgentSettings') {
          if (values[0] !== window.__u.preview.token) throw new Error('Preview changed');
          if (!Array.isArray(values[1]) || values[1].length) throw new Error('General repair must pass an explicit empty add list');
          const done = window.__u.repaired || { ...window.__u.preview, findings: window.__u.preview.findings.map(f => ({ ...f, changed: f.state === 'stale', backup: f.state === 'stale' ? 'C:/Fixture/backup' : '' })) };
          result = await finish(name, done);
        } else throw new Error('Unsupported fixture method ' + name);
        return JSON.stringify(result);
      },
      InstalledPackages: async () => finish('InstalledPackages', window.__u.installed),
      FindPackageUpdates: async () => { window.__updateCalls.push(['FindPackageUpdates']); return finish('FindPackageUpdates', window.__u.packages); },
      PlanPersonalPackages: async (workspace, bundle) => {
        window.__updateCalls.push(['PlanPersonalPackages', workspace, bundle]);
        return finish('PlanPersonalPackages', { name: 'Writing skills', from: '1.0.0', to: '1.1.0', token: 'plan-1', home: 'C:/Fixture/skills', added: 1, updated: 1, removed: 0, changed: [], warnings: [], conflicts: [], ...window.__u.plan });
      },
      InstallPersonalPackages: async (...values) => {
        window.__updateCalls.push(['InstallPersonalPackages', ...values]);
        const done = await finish('InstallPersonalPackages', { name: 'Writing skills', to: '1.1.0' });
        window.__u.installed[0].version = '1.1.0'; window.__u.packages.installed = clone(window.__u.installed); window.__u.packages.updates = [];
        return done;
      },
      PlanPackages: async () => { throw new Error('Unexpected workspace mutation'); },
      Restart: async () => { window.__updateCalls.push(['Restart']); throw new Error('Unexpected automatic restart'); },
      RestartToUpdate: async () => { window.__updateCalls.push(['RestartToUpdate']); throw new Error('Unexpected automatic restart'); }
    };
  }, init: () => { if (window.__u.initialHold) Object.assign(window.__hold, window.__u.initialHold); } });
  await t.page.evaluate(() => dinahWorkbench.open('settings'));
  if (openUpdates) await t.page.locator('#settings-tab-updates').click();
  return t;
}
async function settle(t) { await t.page.waitForFunction(() => !document.querySelector('#updates-refresh').disabled && !document.querySelector('#updates-check').disabled); }
async function setRelease(t, change) {
  await t.page.evaluate(change => { __u.release = { current: '2.0.0', ...change }; }, change);
  await t.page.locator('#updates-check').click();
  await t.page.waitForFunction(() => !document.querySelector('#updates-check').disabled || document.querySelector('#updates-release-status').textContent.includes('Downloading'));
}
async function ledger(t) { return t.page.evaluate(() => __updateCalls); }
async function geometry(t) {
  return t.page.evaluate(() => {
    const panel = document.querySelector('#settings-updates');
    return [...panel.querySelectorAll(':scope > section, button[id], .updates-programs > .updates-row')].map(el => {
      const r = el.getBoundingClientRect(); return [el.id || el.className, r.x, r.y + panel.scrollTop, r.width, r.height];
    });
  });
}
async function finish(t, name) {
  const calls = await ledger(t);
  t.ok('no automatic restart, kill or reconnect', !calls.some(c => /^(Restart|RestartToUpdate|Kill|AgentRestart)$/.test(c[0])));
  t.ok('no native or external-runtime action', !(await t.page.evaluate(() => __runtime)).some(c => ['Quit', 'BrowserOpenURL'].includes(c[0])));
  fs.mkdirSync(OUT, { recursive: true }); fs.writeFileSync(path.join(OUT, name + '-calls.json'), JSON.stringify(calls, null, 2));
  await t.close();
}
module.exports = { start, settle, setRelease, ledger, geometry, finish };
