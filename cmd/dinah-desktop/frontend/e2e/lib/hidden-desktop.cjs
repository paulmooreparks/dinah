// The real desktop program, hidden, on a fresh isolated home, with its page driven from headless Edge.
//
// The hidden checks of Decisions (decisions-hidden.cjs, decisions-dismiss-hidden.cjs) share this file. It starts
// the program with the environment variable that keeps its window hidden and makes it write its page address to a
// file, opens that address in a headless browser, and gives the check the page, the bridge and the stand-in
// agent's record. Nothing appears on the screen: every process is started hidden, and the home's settings turn the
// operating system's notifications off before the program first starts.
//
// The stand-in agent is cmd/dinah-desktop/testdata/decisions-agent. It appends one JSON line to agent.jsonl in the
// output folder for every prompt it is sent and for every tool it calls on the real bus helper.
'use strict';
const fs = require('node:fs'), path = require('node:path');
const { spawn, spawnSync } = require('node:child_process');
const { chromium } = require('playwright-core');
const { acquire } = require('./browser-lock');

const EDGE = 'C:/Program Files (x86)/Microsoft/Edge/Application/msedge.exe';
// Other test runs may be using the machine, so a wait is given far longer than the program needs when it is alone.
const WAIT = 180000;

async function hiddenDesktop({ desktop, agent, out }) {
  if (!desktop || !agent || !out || fs.existsSync(out)) throw Error('Supply the desktop program, the stand-in agent and a fresh output folder.');
  fs.mkdirSync(out, { recursive: true });
  for (const name of ['home', 'profile', 'appdata', 'local', 'tmp']) fs.mkdirSync(path.join(out, name));
  // A copy of the program that is not the installed one registers its helper only for a workspace in its temporary folder.
  const workspace = path.join(out, 'tmp', 'workspace');
  fs.mkdirSync(workspace);
  const home = path.join(out, 'home'), exe = path.join(out, 'dinah-desktop.exe'), entry = path.join(out, 'entry.txt');
  fs.copyFileSync(desktop, exe);
  // A Devin chat is started only when the command program is beside the window program or on PATH (devin_steering.go).
  // The program is given no PATH of the person's, so the command program built from the same source is put beside it.
  const command = path.join(path.dirname(path.resolve(desktop)), 'dinah.exe');
  if (!fs.existsSync(command)) throw Error('Build ./cmd/dinah as dinah.exe beside the desktop program: ' + command);
  fs.copyFileSync(command, path.join(out, 'dinah.exe'));
  // The operating system's notifications are off from the first start, so a request shows nothing on the screen.
  fs.writeFileSync(path.join(home, 'settings.json'), JSON.stringify({ hostNotificationsOff: true }));
  // The program is given none of the person's own folders. Everything it reads or writes is under the output folder.
  const env = {
    SystemRoot: process.env.SystemRoot, WINDIR: process.env.WINDIR, COMSPEC: process.env.COMSPEC, PATH: path.join(process.env.SystemRoot, 'System32'),
    USERPROFILE: path.join(out, 'profile'), HOME: path.join(out, 'profile'), APPDATA: path.join(out, 'appdata'), LOCALAPPDATA: path.join(out, 'local'),
    TEMP: path.join(out, 'tmp'), TMP: path.join(out, 'tmp'), CODEX_HOME: path.join(out, 'profile', '.codex'), CLAUDE_CONFIG_DIR: path.join(out, 'profile', '.claude'),
    OLLAMA_HOST: 'http://127.0.0.1:1', DINAH_HOME: home, DINAH_SHELL_ENTRY_FILE: entry, DECISIONS_FIXTURE: out
  };
  const t = { out, home, exe, workspace, agent: path.resolve(agent), page: null, browser: null, pid: 0, errors: [], evidence: [], checks: [], failures: [] };
  let child = null, exited = null, releaseLock = null;

  const read = name => { try { return fs.readFileSync(path.join(out, name), 'utf8'); } catch { return ''; } };
  // Everything the stand-in agent has recorded so far.
  t.records = () => read('agent.jsonl').trim().split('\n').filter(Boolean).map(line => JSON.parse(line));
  // The prompts one chat's agent has been sent, in order.
  t.prompts = session => t.records().filter(x => x.session === session && typeof x.prompt === 'string').map(x => x.prompt);
  // Waits for a condition. It never waits for a fixed time: it asks again until the condition holds or the limit passes.
  t.until = async (fn, label, ms = WAIT) => {
    const end = Date.now() + ms;
    for (;;) {
      const value = await fn();
      if (value) return value;
      if (Date.now() > end) throw Error('Timed out waiting for: ' + label);
      await new Promise(resolve => setTimeout(resolve, 100));
    }
  };
  // The page reaches the program by posting each call to the bridge (internal/bridge). The calls of the methods in
  // watch are recorded here as they pass, each with the program's own answer, and nothing in them is changed. by
  // says whether the page made the call or this check did, through t.call. A check that needs a lost answer sets
  // lose to a method's name: the next call of it the page makes reaches the program, and its answer is dropped.
  t.calls = []; t.watch = new Set(); t.lose = ''; t.unanswered = [];
  const direct = [];
  const sorted = value => Array.isArray(value) ? value.map(sorted) : value && typeof value === 'object' ? Object.fromEntries(Object.keys(value).sort().map(key => [key, sorted(value[key])])) : value;
  const callKey = (method, args) => JSON.stringify([method, sorted(args)]);
  async function passCall(route, sent) {
    const at = direct.indexOf(callKey(sent.method, sent.args));
    const entry = { method: sent.method, args: sent.args, by: at < 0 ? 'page' : 'check' };
    if (at >= 0) direct.splice(at, 1);
    t.calls.push(entry);
    const response = await route.fetch();
    entry.answer = await response.json();
    if (entry.by === 'page' && t.lose === sent.method) { t.lose = ''; entry.lost = true; return route.abort('connectionfailed'); }
    return route.fulfill({ response });
  }
  // The calls of one method the page itself has made, from a place in the record on.
  t.pageCalls = (method, from = 0) => t.calls.slice(from).filter(c => c.by === 'page' && c.method === method);
  // Calls a method of the real program through the page's bridge, as the page itself does.
  t.call = (name, ...args) => {
    if (t.watch.has(name)) direct.push(callKey(name, args));
    return t.page.evaluate(({ name, args }) => window.go.main.App[name](...args), { name, args });
  };
  t.check = (name, pass, detail) => {
    pass = !!pass;
    console.log((pass ? 'PASS ' : 'FAIL ') + name + (pass || detail === undefined ? '' : ' :: ' + (typeof detail === 'string' ? detail : JSON.stringify(detail))));
    t.checks.push({ name, pass, detail: pass || detail === undefined ? undefined : detail });
    if (!pass) t.failures.push(name);
    return pass;
  };
  t.keep = value => { t.evidence.push(value); return value; };

  // Ends the program this file started, and everything that program started, by its process ID.
  function endTree() {
    if (!child || child.exitCode !== null || child.signalCode !== null) return;
    spawnSync('taskkill.exe', ['/PID', String(child.pid), '/T', '/F'], { windowsHide: true, stdio: 'ignore' });
  }
  process.on('exit', endTree);

  t.start = async () => {
    if (!releaseLock) releaseLock = await acquire();
    if (!t.browser) t.browser = await chromium.launch({ executablePath: EDGE, headless: true });
    fs.rmSync(entry, { force: true });
    const log = fs.openSync(path.join(out, 'desktop.log'), 'a');
    child = spawn(exe, [], { env, cwd: out, windowsHide: true, stdio: ['ignore', log, log] });
    fs.closeSync(log);
    const started = child;
    exited = new Promise(resolve => started.once('exit', resolve));
    t.pid = child.pid;
    const url = await t.until(() => { if (started.exitCode !== null) throw Error('The program ended before it wrote its page address.'); return read('entry.txt').trim(); }, 'the page address of the hidden program');
    t.page = await t.browser.newPage({ viewport: { width: 1440, height: 1000 } });
    t.page.setDefaultTimeout(WAIT);
    t.page.on('pageerror', error => t.errors.push(String(error)));
    t.page.on('dialog', dialog => { t.errors.push('Browser dialog: ' + dialog.message()); dialog.dismiss().catch(() => {}); });
    // The page reaches nothing outside this computer.
    await t.page.route('**/*', route => {
      const request = route.request(), url = new URL(request.url());
      if (!['127.0.0.1', 'localhost', '[::1]'].includes(url.hostname)) return route.abort();
      let sent = null;
      if (t.watch.size && request.method() === 'POST' && url.pathname.endsWith('/call')) { try { sent = request.postDataJSON(); } catch { sent = null; } }
      if (!sent || !t.watch.has(sent.method)) return route.continue();
      // A page that is closed or reloaded while a call is on its way has nobody left to answer. Such a call is kept for the receipt.
      return passCall(route, sent).catch(error => { t.unanswered.push(sent.method + ': ' + String(error)); });
    });
    await t.page.goto(url);
    await t.ready();
    return child.pid;
  };
  // The page is ready when its saved state has been read, its layout restored and its start screen gone, as in lib/page.js.
  t.ready = async () => {
    await t.page.waitForFunction(() => window.dinahUIState && window.dinahWorkbench && window.go && window.go.main && window.go.main.App);
    await t.page.evaluate(() => window.dinahUIState.ready);
    await t.page.waitForFunction(() => document.documentElement.dataset.uiStateReady === 'true' && (!document.getElementById('start-screen') || document.getElementById('start-screen').hidden));
  };
  t.stop = async () => {
    if (t.page) { await t.page.close().catch(() => {}); t.page = null; }
    endTree();
    if (exited) await exited;
    child = null; exited = null;
  };
  // Writes what the run kept, ends every process, and gives the shared test lock back.
  t.finish = async error => {
    const failure = error ? String(error && error.stack || error) : '';
    if (error && t.page) fs.writeFileSync(path.join(out, 'failure-page.txt'), await t.page.locator('body').innerText().catch(() => ''));
    if (t.errors.length) t.failures.push('The page reported errors.');
    fs.writeFileSync(path.join(out, 'receipt.json'), JSON.stringify({ passed: !error && !t.failures.length, failure, failures: t.failures, pageErrors: t.errors, checks: t.checks, evidence: t.evidence, calls: t.calls, unanswered: t.unanswered }, null, 2));
    await t.stop().catch(() => {});
    if (t.browser) await t.browser.close().catch(() => {});
    if (releaseLock) await releaseLock().catch(() => {});
    if (error || t.failures.length) process.exitCode = 1;
  };
  return t;
}

module.exports = { hiddenDesktop, WAIT };
