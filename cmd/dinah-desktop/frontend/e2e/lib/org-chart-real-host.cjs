'use strict';
// The caller holds the shared test lock and supplies invented fixture programs.
const fs = require('node:fs');
const path = require('node:path');
const crypto = require('node:crypto');
const assert = require('node:assert/strict');
const { spawn } = require('node:child_process');
const { chromium } = require('playwright-core');

async function ready(read, child) {
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    if (child.exitCode !== null) throw Error('The isolated receiving host exited before readiness.');
    const value = read();
    if (value) return value;
    await new Promise(resolve => setTimeout(resolve, 40));
  }
  throw Error('The isolated receiving host did not publish its entry.');
}

exports.open = async function open(config, out, view) {
  fs.mkdirSync(out, { recursive: false });
  for (const dir of ['home', 'profile', 'appdata', 'local', 'tmp', 'peers']) fs.mkdirSync(path.join(out, dir));
  const home = path.join(out, 'home');
  const env = {
    SystemRoot: process.env.SystemRoot, WINDIR: process.env.WINDIR, COMSPEC: process.env.COMSPEC,
    PATH: path.join(out, 'peers') + ';' + path.join(process.env.SystemRoot, 'System32'),
    USERPROFILE: path.join(out, 'profile'), HOME: path.join(out, 'profile'),
    APPDATA: path.join(out, 'appdata'), LOCALAPPDATA: path.join(out, 'local'),
    TEMP: path.join(out, 'tmp'), TMP: path.join(out, 'tmp'), DINAH_HOME: home,
    CODEX_HOME: path.join(out, 'profile', '.codex'), CLAUDE_CONFIG_DIR: path.join(out, 'profile', '.claude'),
    DINAH_STARTUP_AUDIT_ENTRY: path.join(out, 'entry.txt'),
    DINAH_STARTUP_PEER_DIR: path.join(out, 'peers'), DINAH_STARTUP_PEER_LOG: path.join(out, 'peer.jsonl')
  };
  const settingsPath = path.join(home, 'settings.json');
  const workspace = path.join(home, 'invented-workspace');
  fs.mkdirSync(workspace);
  const settings = { workspace, sessionTitles: {}, sessionProviders: {}, nativeSessions: {}, busSessions: {} };
  settings.providerExecutables = {};
  settings.hostNotificationsOff = true;
  for (const id of ['codex', 'devin', 'claude']) {
    const exe = path.join(out, 'peers', id + '.exe');
    fs.copyFileSync(config.peer, exe);
    settings.providerExecutables[id] = exe;
  }
  assert.ok(config.commandProgram, 'Supply the command name from the actual product profile.');
  fs.copyFileSync(config.peer, path.join(out, 'peers', config.commandProgram + '.exe'));
  fs.writeFileSync(settingsPath, JSON.stringify(settings));
  const log = fs.openSync(path.join(out, 'host.log'), 'w');
  const child = spawn(config.host, ['-test.run=^TestStartupPageCostHost$', '-test.timeout=170s', '-test.v'],
    { env, windowsHide: true, stdio: ['pipe', log, log] });
  const exited = new Promise((resolve, reject) => { child.once('exit', resolve); child.once('error', reject); });
  let browser, page, closed = false;
  const errors = [];
  async function close() {
    if (closed) return;
    closed = true;
    try {
      try {
        if (page && !page.isClosed()) {
          fs.writeFileSync(path.join(out, 'bridge-raw.json'), JSON.stringify(await page.evaluate(() => window.__orgReceivingCalls || []), null, 2));
        }
      } finally {
        if (browser) await browser.close();
      }
    }
    finally {
      if (child.exitCode === null) child.stdin.end('done\n');
      const exit = await exited;
      fs.closeSync(log);
      fs.writeFileSync(path.join(out, 'host-receipt.json'), JSON.stringify({ exit, errors }, null, 2));
      assert.equal(exit, 0, 'The isolated receiving host exits cleanly.');
    }
  }
  try {
    const url = await ready(() => fs.existsSync(env.DINAH_STARTUP_AUDIT_ENTRY) && fs.readFileSync(env.DINAH_STARTUP_AUDIT_ENTRY, 'utf8').trim(), child);
    browser = await chromium.launch({ headless: true, channel: 'msedge' });
    page = await browser.newPage({ viewport: { width: view.width || 1440, height: 1000 }, colorScheme: view.theme || 'light' });
    page.on('pageerror', error => errors.push(error.message));
    await page.route('**/*', route => ['127.0.0.1', 'localhost', '[::1]'].includes(new URL(route.request().url()).hostname) ? route.continue() : route.abort());
    await page.addInitScript(() => {
      window.__orgReceivingPending = 0;
      window.__orgReceivingCalls = [];
      const original = window.fetch;
      window.fetch = function(input, options) {
        const url = new URL(typeof input === 'string' ? input : input.url, location.href);
        if (url.pathname !== '/bridge/call') return original.apply(this, arguments);
        const record = { ...JSON.parse(options.body), start: performance.now() };
        __orgReceivingCalls.push(record);
        __orgReceivingPending++;
        return original.apply(this, arguments).then(async response => {
          try { record.response = await response.clone().json(); }
          catch (error) { record.captureError = String(error); }
          return response;
        }).finally(() => { record.end = performance.now(); __orgReceivingPending--; });
      };
    });
    await page.goto(url);
    await page.waitForFunction(() => window.dinahUIState && window.dinahWorkbench && window.go?.main?.App?.OrgChartHostFixtureManifest);
    await page.evaluate(() => dinahUIState.ready);
    await page.waitForFunction(() => document.body && !document.body.hasAttribute('aria-busy') &&
      window.dinahCommands?.rows('view').some(row => row.label === 'Org Chart'));
    await page.evaluate(theme => document.documentElement.dataset.theme = theme, view.theme || 'light');
    await page.evaluate(() => document.fonts.ready);
    const idle = async () => {
      await page.waitForFunction(() => __orgReceivingPending === 0);
      await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
      await page.waitForFunction(() => __orgReceivingPending === 0);
    };
    const call = async (name, ...args) => {
      if (name === 'OrgChartHostFixtureSnapshot') await idle();
      return page.evaluate(({ name, args }) => go.main.App[name](...args), { name, args });
    };
    const manifest = await call('OrgChartHostFixtureManifest');
    const key = await page.evaluate(() => dinahWorkbench.open('org-chart'));
    await page.evaluate(key => pudlWindows.snap(key, 'maximized'), key);
    const chart = page.locator('[data-win="' + key + '"] .org-chart');
    await chart.locator('[data-org-chat]').first().waitFor();
    await page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
    fs.writeFileSync(path.join(out, 'provenance.json'), JSON.stringify({ manifest,
      hostSha256: crypto.createHash('sha256').update(fs.readFileSync(config.host)).digest('hex') }, null, 2));
    return { page, chart, call, manifest, errors, close, out, key, idle };
  } catch (error) { await close(); throw error; }
};
